package projects

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/watch"
)

var errServerStopping = errors.New("The server is stopping.")

func listRepositories(ctx context.Context, q machines.Querier) ([]*webv1.Repository, error) {
	rows, err := q.Query(ctx, `
SELECT r.id, r.name, r.main_branch, r.remote, r.first_commit, r.remote_omitted,
       f.id, f.machine_id, f.resolved_path
FROM repositories r LEFT JOIN repository_folders f ON f.repository_id = r.id
ORDER BY r.id, f.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repositories []*webv1.Repository
	var current *webv1.Repository
	for rows.Next() {
		var repository webv1.Repository
		var folderID, machineID, resolvedPath *string
		err := rows.Scan(&repository.Id, &repository.Name, &repository.MainBranch, &repository.Remote,
			&repository.FirstCommit, &repository.RemoteOmitted, &folderID, &machineID, &resolvedPath)
		if err != nil {
			return nil, err
		}
		if current == nil || current.Id != repository.Id {
			current = &repository
			repositories = append(repositories, current)
		}
		if folderID != nil {
			current.Folders = append(current.Folders, &webv1.RepositoryFolder{
				Id: *folderID, ClientId: *machineID, ResolvedPath: *resolvedPath,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return repositories, nil
}

// WatchRepositories streams the repository snapshot and subsequent changes for a live session.
func (s *Service) WatchRepositories(
	ctx context.Context,
	_ *connect.Request[webv1.WatchRepositoriesRequest],
	stream *connect.ServerStream[webv1.WatchRepositoriesResponse],
) error {
	defer transport.BoundLinkEnd(ctx, transport.SendTimeout)

	verifier, ok := api.SessionVerifier(ctx)
	if !ok {
		return watchError(watch.ErrWatchUnauthorized, false)
	}
	if s.watchShutdown != nil {
		if s.watchShutdown.Err() != nil {
			return watchError(errServerStopping, false)
		}
		var cancel context.CancelCauseFunc
		ctx, cancel = context.WithCancelCause(ctx)
		defer cancel(nil)
		stop := context.AfterFunc(s.watchShutdown, func() { cancel(errServerStopping) })
		defer stop()
	}
	if s.follower == nil {
		return watchError(store.ErrFeedUnavailable, false)
	}

	source := watch.Source[*webv1.Repository]{
		Topic: "repositories",
		Authorize: func(ctx context.Context, tx *store.ReadTx) error {
			err := s.sessions.CheckInRead(ctx, tx, verifier)
			if errors.Is(err, auth.ErrNoSession) {
				return watch.ErrWatchUnauthorized
			}
			return err
		},
		Snapshot: func(ctx context.Context, tx *store.ReadTx) ([]*webv1.Repository, error) {
			return listRepositories(ctx, tx)
		},
		Load: func(ctx context.Context, tx *store.ReadTx, id string) (*webv1.Repository, bool, error) {
			return loadRepository(ctx, tx, id)
		},
	}
	sentSnapshot := false
	err := watch.Serve(ctx, s.store, s.follower, source, func(ctx context.Context, message watch.Message[*webv1.Repository]) error {
		err := transport.Send(ctx, func() error {
			return stream.Send(watchResponse(message))
		})
		if err == nil && message.Snapshot != nil {
			sentSnapshot = true
		}
		return err
	})
	return watchError(err, sentSnapshot)
}

func watchResponse(message watch.Message[*webv1.Repository]) *webv1.WatchRepositoriesResponse {
	response := &webv1.WatchRepositoriesResponse{
		Position: &webv1.FeedPosition{Value: uint64(message.Position)},
	}
	switch {
	case message.Snapshot != nil:
		response.Change = &webv1.WatchRepositoriesResponse_All{All: &webv1.RepositoryList{Repositories: message.Snapshot.Records}}
	case message.Saved != nil:
		response.Change = &webv1.WatchRepositoriesResponse_Saved{Saved: *message.Saved}
	default:
		response.Change = &webv1.WatchRepositoriesResponse_RemovedId{RemovedId: message.RemovedKey}
	}
	return response
}

func watchError(err error, sentSnapshot bool) error {
	var dbErr *store.Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, watch.ErrWatchUnauthorized):
		if !sentSnapshot {
			return projectError(connect.CodeUnauthenticated, "Sign in to continue.")
		}
		return connect.NewError(connect.CodeUnauthenticated, watch.ErrWatchUnauthorized)
	case errors.Is(err, watch.ErrAuthorizationUnavailable):
		return connect.NewError(connect.CodeUnavailable, watch.ErrAuthorizationUnavailable)
	case errors.Is(err, store.ErrWatchLagged):
		return connect.NewError(connect.CodeAborted, store.ErrWatchLagged)
	case errors.Is(err, store.ErrFeedUnavailable):
		return connect.NewError(connect.CodeUnavailable, store.ErrFeedUnavailable)
	case errors.Is(err, errServerStopping):
		return connect.NewError(connect.CodeUnavailable, errServerStopping)
	case errors.Is(err, context.Canceled):
		return projectError(connect.CodeCanceled, "The request was canceled.")
	case errors.Is(err, context.DeadlineExceeded):
		return projectError(connect.CodeDeadlineExceeded, "The request timed out.")
	case errors.As(err, &dbErr) && dbErr.Retryable():
		return projectError(connect.CodeAborted, "The database was busy, retry the request.")
	default:
		return projectError(connect.CodeUnavailable, "The server cannot reach its database, try again.")
	}
}
