package projects_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth/authtest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

const watchRepositoryTopic = "repositories"

type watchSessionProbe struct {
	lgtmv1connect.UnimplementedSessionServiceHandler
	guarded context.Context
}

func (p *watchSessionProbe) SignOut(ctx context.Context, _ *connect.Request[webv1.SignOutRequest]) (*connect.Response[webv1.SignOutResponse], error) {
	p.guarded = context.WithoutCancel(ctx)
	return connect.NewResponse(&webv1.SignOutResponse{}), nil
}

func watchSessionContext(t *testing.T, sessions *auth.Sessions) (context.Context, auth.Verifier) {
	t.Helper()
	token, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatal("The watch fixture could not issue a session.")
	}
	verifier, ok := auth.TokenVerifier(token)
	if !ok {
		t.Fatal("The watch fixture could not construct a session verifier.")
	}
	origins, err := auth.NewOriginPolicy("http://127.0.0.1:8787", nil)
	if err != nil {
		t.Fatal("The watch fixture could not construct the origin policy.")
	}
	// The SignOut stand-in captures a context from the real guard without revoking its session.
	probe := new(watchSessionProbe)
	router := api.NewHandler(api.Options{Origins: origins, Sessions: sessions, Session: probe})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, lgtmv1connect.SessionServiceSignOutProcedure, bytes.NewReader(nil))
	req.Header.Set("Content-Type", "application/proto")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK || probe.guarded == nil {
		t.Fatalf("The session guard did not pass a context to the fixture. The response status was %d.", res.Code)
	}
	if _, ok := api.SessionVerifier(probe.guarded); !ok {
		t.Fatal("The guarded context did not have a session verifier.")
	}
	return probe.guarded, verifier
}

type watchDeadlineWriter struct {
	http.ResponseWriter
	wasBound *atomic.Bool
}

func (w watchDeadlineWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w watchDeadlineWriter) Flush() {
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w watchDeadlineWriter) SetWriteDeadline(at time.Time) error {
	err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(at)
	if err == nil && !at.IsZero() {
		w.wasBound.Store(true)
	}
	return err
}

func serveProjectWatch(t *testing.T, db *store.Store, sessions *auth.Sessions, base context.Context) (lgtmv1connect.ProjectServiceClient, *atomic.Bool) {
	t.Helper()
	follower, err := store.StartFollower(t.Context(), db, nil)
	if err != nil {
		t.Fatal("The watch fixture could not start the Store follower.")
	}
	t.Cleanup(follower.Close)
	service := projects.NewService(db, sessions, machines.NewRegistry(), nil, follower, t.Context())
	_, handler := lgtmv1connect.NewProjectServiceHandler(service)
	handler = transport.WithWriteDeadline(handler)
	wasBound := new(atomic.Bool)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(watchDeadlineWriter{ResponseWriter: w, wasBound: wasBound}, r)
	}))
	server.Config.BaseContext = func(net.Listener) context.Context { return base }
	server.Start()
	t.Cleanup(server.Close)
	return lgtmv1connect.NewProjectServiceClient(server.Client(), server.URL), wasBound
}

func openProjectWatch(t *testing.T, client lgtmv1connect.ProjectServiceClient) *connect.ServerStreamForClient[webv1.WatchRepositoriesResponse] {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
	t.Cleanup(cancel)
	stream, err := client.WatchRepositories(ctx, connect.NewRequest(&webv1.WatchRepositoriesRequest{}))
	if err != nil {
		t.Fatalf("The watch request failed before it produced a stream. The code was %v.", connect.CodeOf(err))
	}
	t.Cleanup(func() { _ = stream.Close() })
	return stream
}

func nextProjectWatch(t *testing.T, stream *connect.ServerStreamForClient[webv1.WatchRepositoriesResponse], frame string) *webv1.WatchRepositoriesResponse {
	t.Helper()
	if !stream.Receive() {
		t.Fatalf("The watch ended before %s. The code was %v.", frame, connect.CodeOf(stream.Err()))
	}
	return stream.Msg()
}

func requireProjectWatchEnd(t *testing.T, stream *connect.ServerStreamForClient[webv1.WatchRepositoriesResponse], reason string, sentence string) {
	t.Helper()
	if stream.Receive() {
		t.Fatalf("The watch sent a frame after %s.", reason)
	}
	var rpcErr *connect.Error
	if !errors.As(stream.Err(), &rpcErr) || rpcErr.Code() != connect.CodeUnauthenticated || (sentence != "" && rpcErr.Message() != sentence) {
		t.Fatalf("After %s the watch returned code %v. It must return unauthenticated with the contracted sentence when applicable.", reason, connect.CodeOf(stream.Err()))
	}
}

type watchSnapshotQuery struct{}

type watchSnapshotBarrier struct {
	hit     atomic.Bool
	enter   func()
	release <-chan struct{}
}

func (b *watchSnapshotBarrier) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if _, guarded := api.SessionVerifier(ctx); !guarded {
		return ctx
	}
	sql := strings.ToLower(strings.Join(strings.Fields(query.SQL), " "))
	readsHead := strings.Contains(sql, "max(position)") && strings.Contains(sql, "from feed_retention")
	readsRepositories := strings.Contains(sql, "from repositories")
	if readsHead || readsRepositories {
		return context.WithValue(ctx, watchSnapshotQuery{}, true)
	}
	return ctx
}

func (b *watchSnapshotBarrier) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryEndData) {
	if marked, _ := ctx.Value(watchSnapshotQuery{}).(bool); query.Err != nil || !marked || !b.hit.CompareAndSwap(false, true) {
		return
	}
	b.enter()
	select {
	case <-b.release:
	case <-ctx.Done():
	}
}

func watchSeedRepository(t *testing.T, name string, folders ...*webv1.RepositoryFolder) *webv1.Repository {
	t.Helper()
	return &webv1.Repository{Id: projectMigrationID(t), Name: name, MainBranch: "main", Folders: folders}
}

func watchSeedFolder(t *testing.T, path string) *webv1.RepositoryFolder {
	t.Helper()
	return &webv1.RepositoryFolder{Id: projectMigrationID(t), ClientId: projectMigrationID(t), ResolvedPath: path}
}

func insertWatchFolder(ctx context.Context, tx *store.Tx, repositoryID string, folder *webv1.RepositoryFolder) error {
	_, err := tx.Exec(ctx, `
INSERT INTO repository_folders (id, repository_id, machine_id, resolved_path)
VALUES ($1, $2, $3, $4)`, folder.Id, repositoryID, folder.ClientId, folder.ResolvedPath)
	return err
}

func insertWatchRepository(ctx context.Context, tx *store.Tx, repo *webv1.Repository) error {
	_, err := tx.Exec(ctx, `
INSERT INTO repositories (id, name, main_branch, remote, first_commit, remote_omitted)
VALUES ($1, $2, $3, $4, $5, $6)`, repo.Id, repo.Name, repo.MainBranch, repo.Remote, repo.FirstCommit, repo.RemoteOmitted)
	if err != nil {
		return err
	}
	for _, folder := range repo.Folders {
		if err := insertWatchFolder(ctx, tx, repo.Id, folder); err != nil {
			return err
		}
	}
	return tx.Record(store.ChangeRef{Topic: watchRepositoryTopic, Key: repo.Id, Change: store.Saved})
}

func writeWatchSeed(t *testing.T, db *store.Store, write func(context.Context, *store.Tx) error) uint64 {
	t.Helper()
	if err := db.Write(t.Context(), write); err != nil {
		t.Fatal("The watch fixture could not commit its rows and feed references.")
	}
	return projectWatchHead(t, db)
}

func projectWatchHead(t *testing.T, db *store.Store) uint64 {
	t.Helper()
	head, err := db.Snapshot(t.Context(), func(context.Context, *store.ReadTx) error { return nil })
	if err != nil {
		t.Fatal("The watch fixture could not read the committed feed head.")
	}
	return uint64(head)
}

func requireProjectWatchPosition(t *testing.T, response *webv1.WatchRepositoriesResponse, position uint64) {
	t.Helper()
	if response.GetPosition() == nil || response.GetPosition().GetValue() != position {
		t.Fatalf("The watch returned position %v. It must include committed position %d.", response.GetPosition(), position)
	}
}

func requireProjectWatchRecord(t *testing.T, actual, expected *webv1.Repository) {
	t.Helper()
	if actual == nil {
		t.Fatal("The watch omitted the complete repository record.")
	}
	// The contract specifies membership and the full record. It does not give a folder sort key.
	got, want := proto.CloneOf(actual), proto.CloneOf(expected)
	for _, repo := range []*webv1.Repository{got, want} {
		slices.SortFunc(repo.Folders, func(a, b *webv1.RepositoryFolder) int { return strings.Compare(a.GetId(), b.GetId()) })
	}
	if !proto.Equal(got, want) {
		t.Fatalf("The watch did not return the complete repository record. It returned %d folders, want %d with all stored fields.", len(actual.Folders), len(expected.Folders))
	}
}

func requireProjectWatchSnapshot(t *testing.T, response *webv1.WatchRepositoriesResponse, position uint64, repos ...*webv1.Repository) {
	t.Helper()
	requireProjectWatchPosition(t, response, position)
	list := response.GetAll()
	if list == nil || len(list.GetRepositories()) != len(repos) {
		t.Fatalf("The first frame must be all with %d repositories. It had %d repositories.", len(repos), len(list.GetRepositories()))
	}
	got := slices.Clone(list.GetRepositories())
	want := slices.Clone(repos)
	order := func(a, b *webv1.Repository) int { return strings.Compare(a.GetId(), b.GetId()) }
	slices.SortFunc(got, order)
	slices.SortFunc(want, order)
	for i := range got {
		requireProjectWatchRecord(t, got[i], want[i])
	}
}

func newProjectWatchStore(t *testing.T, trace pgx.QueryTracer) *store.Store {
	t.Helper()
	config := storetest.NewDatabase(t, "projects_watch")
	config.ConnConfig.Tracer = trace
	db := openOperationStore(t, config)
	if err := db.Migrate(t.Context(), auth.Migrations(), machines.Migrations(), projects.Migrations()); err != nil {
		t.Fatal("The watch fixture could not apply its migrations.")
	}
	return db
}

func TestProjectWatchSnapshotAndChanges(t *testing.T) {
	t.Run("empty snapshot", func(t *testing.T) {
		db := newProjectWatchStore(t, nil)
		sessions := auth.NewSessions(db, auth.Password{})
		base, _ := watchSessionContext(t, sessions)
		head := projectWatchHead(t, db)
		client, deadline := serveProjectWatch(t, db, sessions, base)
		stream := openProjectWatch(t, client)
		requireProjectWatchSnapshot(t, nextProjectWatch(t, stream, "the explicitly empty snapshot"), head)
		if !deadline.Load() {
			t.Fatal("The snapshot send did not set a write deadline on the server response.")
		}
	})

	t.Run("snapshot at its feed head and seeded changes", func(t *testing.T) {
		reachedHead, enter := storetest.Signal(t)
		releaseHead, release := storetest.Signal(t)
		barrier := &watchSnapshotBarrier{enter: enter, release: releaseHead}
		db := newProjectWatchStore(t, barrier)
		sessions := auth.NewSessions(db, auth.Password{})
		base, _ := watchSessionContext(t, sessions)
		initial := watchSeedRepository(t, "project", watchSeedFolder(t, "/workspace/one"), watchSeedFolder(t, "/workspace/two"))
		initial.Remote = proto.String("https://example.test/team/project")
		initial.FirstCommit = proto.String("1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b")
		withheld := watchSeedRepository(t, "withheld", watchSeedFolder(t, "/workspace/withheld"))
		withheld.RemoteOmitted = true
		oldHead := writeWatchSeed(t, db, func(ctx context.Context, tx *store.Tx) error {
			if err := insertWatchRepository(ctx, tx, initial); err != nil {
				return err
			}
			return insertWatchRepository(ctx, tx, withheld)
		})
		client, _ := serveProjectWatch(t, db, sessions, base)
		ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
		defer cancel()
		defer release()
		type firstFrame struct {
			stream   *connect.ServerStreamForClient[webv1.WatchRepositoriesResponse]
			received bool
			err      error
		}
		first := storetest.Start(t, func() firstFrame {
			stream, err := client.WatchRepositories(ctx, connect.NewRequest(&webv1.WatchRepositoriesRequest{}))
			if err != nil {
				return firstFrame{err: err}
			}
			received := stream.Receive()
			return firstFrame{stream: stream, received: received, err: stream.Err()}
		})
		select {
		case <-reachedHead:
		case <-first.Done():
			t.Fatal("The watch returned before the snapshot head and row reads could be separated by a commit.")
		case <-time.After(storetest.ObservationTimeout):
			t.Fatal("The watch did not reach the snapshot head or repository query.")
		}
		addedFolder := watchSeedFolder(t, "/workspace/three")
		err := db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if err := insertWatchFolder(ctx, tx, initial.Id, addedFolder); err != nil {
				return err
			}
			return tx.Record(store.ChangeRef{Topic: watchRepositoryTopic, Key: initial.Id, Change: store.Saved})
		})
		if err != nil {
			t.Fatal("The competing seed write did not commit during the snapshot.")
		}
		release()
		savedPosition := projectWatchHead(t, db)
		result := first.Wait(t)
		if !result.received {
			t.Fatalf("The watch ended before the snapshot from before the competing commit. The code was %v.", connect.CodeOf(result.err))
		}
		stream := result.stream
		t.Cleanup(func() { _ = stream.Close() })
		requireProjectWatchSnapshot(t, stream.Msg(), oldHead, initial, withheld)
		saved := nextProjectWatch(t, stream, "the complete saved record after the snapshot head")
		requireProjectWatchPosition(t, saved, savedPosition)
		full := proto.CloneOf(initial)
		full.Folders = append(full.Folders, addedFolder)
		requireProjectWatchRecord(t, saved.GetSaved(), full)

		// The only matching reference is last. Any misrouted frame precedes it and fails the next assertion.
		otherTopic, otherScope, absent := projectMigrationID(t), projectMigrationID(t), projectMigrationID(t)
		removedPosition := writeWatchSeed(t, db, func(_ context.Context, tx *store.Tx) error {
			for _, ref := range []store.ChangeRef{
				{Topic: "machines", Key: otherTopic, Change: store.Removed},
				{Topic: watchRepositoryTopic, Scope: "fixture-scope", Key: otherScope, Change: store.Removed},
				{Topic: watchRepositoryTopic, Key: absent, Change: store.Removed},
			} {
				if err := tx.Record(ref); err != nil {
					return err
				}
			}
			return nil
		})
		removed := nextProjectWatch(t, stream, "the seeded removed identifier")
		requireProjectWatchPosition(t, removed, removedPosition)
		if removed.GetRemovedId() != absent {
			t.Fatal("The watch did not return the repository identifier from the matching removal reference.")
		}
	})
}

func TestProjectWatchSessionExpiry(t *testing.T) {
	t.Run("missing verifier", func(t *testing.T) {
		db := newProjectWatchStore(t, nil)
		sessions := auth.NewSessions(db, auth.Password{})
		client, _ := serveProjectWatch(t, db, sessions, t.Context())
		requireProjectWatchEnd(t, openProjectWatch(t, client), "opening without a verified session", "")
	})

	for _, reason := range []string{"expiry", "revocation"} {
		t.Run(reason, func(t *testing.T) {
			start := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
			db, _, setClock := authtest.ClockStore(t, start, auth.Migrations(), machines.Migrations(), projects.Migrations())
			sessions := auth.NewSessions(db, auth.Password{})
			base, verifier := watchSessionContext(t, sessions)
			head := projectWatchHead(t, db)
			client, _ := serveProjectWatch(t, db, sessions, base)
			stream := openProjectWatch(t, client)
			requireProjectWatchSnapshot(t, nextProjectWatch(t, stream, "the snapshot while the session was live"), head)

			switch reason {
			case "expiry":
				setClock(start.Add((auth.SessionMaxAgeSeconds + 1) * time.Second))
			case "revocation":
				if err := sessions.Revoke(t.Context(), verifier); err != nil {
					t.Fatal("The watch fixture could not revoke the session.")
				}
			}
			if !errors.Is(sessions.Check(t.Context(), verifier), auth.ErrNoSession) {
				t.Fatal("The fixture session remained live after the invalidation.")
			}
			repo := watchSeedRepository(t, "after-session-end", watchSeedFolder(t, "/workspace/after-session-end"))
			writeWatchSeed(t, db, func(ctx context.Context, tx *store.Tx) error {
				return insertWatchRepository(ctx, tx, repo)
			})
			requireProjectWatchEnd(t, stream, reason, "Sign in again to reopen this watch.")
			requireProjectWatchEnd(t, openProjectWatch(t, client), "reopening with a stale verifier", "")
		})
	}
}
