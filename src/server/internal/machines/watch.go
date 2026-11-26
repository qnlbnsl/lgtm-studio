package machines

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/watch"
)

// WatchClients streams the machine snapshot and subsequent changes for a live session.
func (s *Service) WatchClients(
	ctx context.Context,
	_ *connect.Request[lgtmv1.WatchClientsRequest],
	stream *connect.ServerStream[lgtmv1.WatchClientsResponse],
) error {
	verifier, ok := api.SessionVerifier(ctx)
	if !ok {
		return watchError(watch.ErrWatchUnauthorized)
	}

	follower := s.follower
	if follower == nil {
		// A standalone service owns its follower for the lifetime of the watch.
		var err error
		follower, err = store.StartFollower(ctx, s.store, nil)
		if err != nil {
			return watchError(err)
		}
		defer follower.Close()
	}

	source := watch.Source[Machine]{
		Topic: "machines",
		Authorize: func(ctx context.Context, tx *store.ReadTx) error {
			err := s.sessions.CheckInRead(ctx, tx, verifier)
			if errors.Is(err, auth.ErrNoSession) {
				return watch.ErrWatchUnauthorized
			}
			return err
		},
		Snapshot: func(ctx context.Context, tx *store.ReadTx) ([]Machine, error) {
			return s.registry.List(ctx, tx)
		},
		Load: func(ctx context.Context, tx *store.ReadTx, id string) (Machine, bool, error) {
			machine, err := s.registry.Get(ctx, tx, id)
			if errors.Is(err, ErrNoMachine) {
				return Machine{}, false, nil
			}
			return machine, err == nil, err
		},
	}
	err := watch.Serve(ctx, s.store, follower, source, func(ctx context.Context, message watch.Message[Machine]) error {
		return transport.Send(ctx, func() error {
			return stream.Send(watchResponse(message))
		})
	})
	return watchError(err)
}

func watchResponse(message watch.Message[Machine]) *lgtmv1.WatchClientsResponse {
	response := &lgtmv1.WatchClientsResponse{
		Position: &lgtmv1.FeedPosition{Value: uint64(message.Position)},
	}
	switch {
	case message.Snapshot != nil:
		clients := make([]*lgtmv1.Client, 0, len(message.Snapshot.Records))
		for _, machine := range message.Snapshot.Records {
			clients = append(clients, Project(machine))
		}
		response.Change = &lgtmv1.WatchClientsResponse_All{All: &lgtmv1.ClientList{Clients: clients}}
	case message.Saved != nil:
		response.Change = &lgtmv1.WatchClientsResponse_Saved{Saved: Project(*message.Saved)}
	default:
		response.Change = &lgtmv1.WatchClientsResponse_RemovedId{RemovedId: message.RemovedKey}
	}
	return response
}

func watchError(err error) error {
	var storeErr *store.Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, watch.ErrWatchUnauthorized):
		return connect.NewError(connect.CodeUnauthenticated, watch.ErrWatchUnauthorized)
	case errors.Is(err, watch.ErrAuthorizationUnavailable):
		return connect.NewError(connect.CodeUnavailable, watch.ErrAuthorizationUnavailable)
	case errors.Is(err, store.ErrWatchLagged):
		return connect.NewError(connect.CodeAborted, store.ErrWatchLagged)
	case errors.Is(err, store.ErrFeedUnavailable):
		return connect.NewError(connect.CodeUnavailable, store.ErrFeedUnavailable)
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, errors.New("The request was canceled."))
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, errors.New("The request timed out."))
	case errors.As(err, &storeErr) && storeErr.Retryable():
		return connect.NewError(connect.CodeAborted, errors.New("The database was busy, retry the request."))
	default:
		return connect.NewError(connect.CodeUnavailable, errors.New("The server cannot reach its database, try again."))
	}
}
