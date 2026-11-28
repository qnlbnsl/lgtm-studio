package projects_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

const watchRefreshReference = "refresh"

func TestProjectWatchAttachment(t *testing.T) {
	f := newReportFixture(t, reportControls{})
	origin := repositoryOrigin{remote: new("https://example.test/watch.git"), first: new("first-commit")}
	add := func(machineID string, stream *reportStream, target *string, path string) *webv1.AddRepositoryResponse {
		op, caller := startRepositoryAdd(t, f, machineID, target, path)
		sendFolderReport(t, stream, 1, origin.report(op, path))
		f.requireAck(t, stream, machineID, 1)
		_, reply := requireRepositoryAdd(t, f, op, caller.Wait(t))
		return reply
	}
	first := add(f.machine, f.stream, nil, "/workspace/first-checkout")
	peer := f.registerPeer(t, "Watch attachment machine")
	peerStream := f.open(t, peer, 0)
	sessions := auth.NewSessions(f.db, auth.Password{})
	base, _ := watchSessionContext(t, sessions)
	head := projectWatchHead(t, f.db)
	client, _ := serveProjectWatch(t, f.db, sessions, base)
	stream := openProjectWatch(t, client)
	requireProjectWatchSnapshot(t, nextProjectWatch(t, stream, "the snapshot before real attachment"), head, first.GetRepository())

	target := first.GetRepository().GetId()
	second := add(peer.ID, peerStream, &target, "/workspace/second-checkout")
	requireRepositoryFolder(t, second.GetFolder(), peer.ID, "/workspace/second-checkout")
	requireRepositoryProjection(t, f, second.GetRepository(), first.GetFolder(), second.GetFolder())
	requireRepositorySavesAfter(t, f, int64(head), target)
	position := repositoryObserve[int64](t, f, `
SELECT position FROM feed WHERE position > $1 AND topic = 'repositories' AND scope = '' AND key = $2 AND change = 'saved'`,
		int64(head), target)
	saved := nextProjectWatch(t, stream, "the complete saved record from real attachment")
	requireProjectWatchPosition(t, saved, uint64(position))
	requireProjectWatchRecord(t, saved.GetSaved(), second.GetRepository())
}

type watchRemovalSeed struct {
	shared, orphan, survivor *webv1.Repository
	removedMachine           string
}

func seedProjectWatchRemoval(t *testing.T, db *store.Store) watchRemovalSeed {
	t.Helper()
	removed := watchSeedFolder(t, "/workspace/shared")
	kept := watchSeedFolder(t, "/workspace/shared-elsewhere")
	orphanFolder := watchSeedFolder(t, "/workspace/orphan")
	orphanFolder.ClientId = removed.ClientId
	shared := watchSeedRepository(t, "shared", removed, kept)
	shared.Remote = new("https://example.test/shared.git")
	orphan := watchSeedRepository(t, "orphan", orphanFolder)
	survivor := proto.CloneOf(shared)
	survivor.Folders = []*webv1.RepositoryFolder{kept}
	writeWatchSeed(t, db, func(ctx context.Context, tx *store.Tx) error {
		if err := insertWatchRepository(ctx, tx, shared); err != nil {
			return err
		}
		return insertWatchRepository(ctx, tx, orphan)
	})
	return watchRemovalSeed{shared: shared, orphan: orphan, survivor: survivor, removedMachine: removed.ClientId}
}

func (s watchRemovalSeed) remove(ctx context.Context, tx *store.Tx, refresh bool) error {
	if _, err := tx.Exec(ctx, "DELETE FROM repository_folders WHERE machine_id = $1", s.removedMachine); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM repositories WHERE id = $1", s.orphan.Id); err != nil {
		return err
	}
	if refresh {
		return tx.Record(store.ChangeRef{Topic: watchRepositoryTopic, Key: watchRefreshReference, Change: store.Saved})
	}
	if err := tx.Record(store.ChangeRef{Topic: watchRepositoryTopic, Key: s.shared.Id, Change: store.Saved}); err != nil {
		return err
	}
	return tx.Record(store.ChangeRef{Topic: watchRepositoryTopic, Key: s.orphan.Id, Change: store.Removed})
}

func requireProjectWatchAborted(t *testing.T, stream *connect.ServerStreamForClient[webv1.WatchRepositoriesResponse], after string) {
	t.Helper()
	if stream.Receive() {
		t.Fatalf("The watch sent a data frame after %s. It must end with aborted.", after)
	}
	var rpcErr *connect.Error
	if !errors.As(stream.Err(), &rpcErr) || rpcErr.Code() != connect.CodeAborted ||
		rpcErr.Message() != "The watch fell behind and must be reopened." {
		t.Fatalf("The watch returned code %v after %s. It must return aborted with the contracted sentence.", connect.CodeOf(stream.Err()), after)
	}
}

func TestProjectWatchExplicitRefresh(t *testing.T) {
	db := newProjectWatchStore(t, nil)
	seed := seedProjectWatchRemoval(t, db)
	sessions := auth.NewSessions(db, auth.Password{})
	base, _ := watchSessionContext(t, sessions)
	head := projectWatchHead(t, db)
	client, _ := serveProjectWatch(t, db, sessions, base)
	stream := openProjectWatch(t, client)
	requireProjectWatchSnapshot(t, nextProjectWatch(t, stream, "the snapshot before explicit refresh"), head, seed.shared, seed.orphan)

	position := writeWatchSeed(t, db, func(ctx context.Context, tx *store.Tx) error {
		return seed.remove(ctx, tx, true)
	})
	requireProjectWatchAborted(t, stream, "the reserved refresh reference")
	reopened := openProjectWatch(t, client)
	requireProjectWatchSnapshot(t, nextProjectWatch(t, reopened, "the replacement snapshot after refresh"), position, seed.survivor)
}

func TestProjectWatchSeededRemoval(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		rollback bool
	}{{"commit", false}, {"rollback", true}} {
		t.Run(scenario.name, func(t *testing.T) {
			db := newProjectWatchStore(t, nil)
			seed := seedProjectWatchRemoval(t, db)
			sessions := auth.NewSessions(db, auth.Password{})
			base, _ := watchSessionContext(t, sessions)
			head := projectWatchHead(t, db)
			client, _ := serveProjectWatch(t, db, sessions, base)
			stream := openProjectWatch(t, client)
			requireProjectWatchSnapshot(t, nextProjectWatch(t, stream, "the snapshot before seeded removal"), head, seed.shared, seed.orphan)

			if scenario.rollback {
				rollbackWatchRemoval(t, db, seed, head, client, stream)
				return
			}
			position := writeWatchSeed(t, db, func(ctx context.Context, tx *store.Tx) error {
				return seed.remove(ctx, tx, false)
			})
			// The fixture publishes exactly two references in this isolated write.
			saved := nextProjectWatch(t, stream, "the survivor of committed removal")
			requireProjectWatchPosition(t, saved, position-1)
			requireProjectWatchRecord(t, saved.GetSaved(), seed.survivor)
			removed := nextProjectWatch(t, stream, "the orphan removed by the committed transaction")
			requireProjectWatchPosition(t, removed, position)
			if removed.GetRemovedId() != seed.orphan.Id {
				t.Fatal("The orphan's repository identifier was missing from the committed removal frame.")
			}
			reopened := openProjectWatch(t, client)
			requireProjectWatchSnapshot(t, nextProjectWatch(t, reopened, "the snapshot after committed removal"), position, seed.survivor)
		})
	}
}

func rollbackWatchRemoval(t *testing.T, db *store.Store, seed watchRemovalSeed, head uint64,
	client lgtmv1connect.ProjectServiceClient, stream *connect.ServerStreamForClient[webv1.WatchRepositoriesResponse],
) {
	t.Helper()
	injected := store.NewDomainError("The watch fixture rolled back the staged removal.")
	for _, refresh := range []bool{false, true} {
		err := db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if err := seed.remove(ctx, tx, refresh); err != nil {
				return err
			}
			return injected
		})
		if !errors.Is(err, injected) {
			t.Fatal("The fixture did not reach its rollback after staging the removal and its references.")
		}
	}
	reopened := openProjectWatch(t, client)
	requireProjectWatchSnapshot(t, nextProjectWatch(t, reopened, "the unchanged snapshot after rollback"), head, seed.shared, seed.orphan)
	// A later committed reference makes any leaked removal frame or terminal refresh observable.
	marker := watchSeedRepository(t, "after-rollback", watchSeedFolder(t, "/workspace/after-rollback"))
	position := writeWatchSeed(t, db, func(ctx context.Context, tx *store.Tx) error {
		return insertWatchRepository(ctx, tx, marker)
	})
	saved := nextProjectWatch(t, stream, "the first committed frame after both rollbacks")
	requireProjectWatchPosition(t, saved, position)
	requireProjectWatchRecord(t, saved.GetSaved(), marker)
}

type heldProjectWatchWriter struct {
	http.ResponseWriter
	ctx         context.Context
	armed       atomic.Bool
	blocked     atomic.Bool
	entered     chan struct{}
	cancelWrite func()
	canceled    <-chan struct{}
	released    <-chan struct{}
}

func (w *heldProjectWatchWriter) Flush() {
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *heldProjectWatchWriter) SetWriteDeadline(at time.Time) error {
	if !at.IsZero() && w.blocked.Load() {
		// The send armed its ordinary deadline before Write. The next nonzero deadline
		// interrupts this held Write. Keep the underlying writer available for the trailer.
		w.cancelWrite()
		return nil
	}
	return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(at)
}

func (w *heldProjectWatchWriter) Write(data []byte) (int, error) {
	if w.armed.Swap(false) {
		w.blocked.Store(true)
		defer w.blocked.Store(false)
		close(w.entered)
		select {
		case <-w.canceled:
			return 0, context.Canceled
		case <-w.ctx.Done():
			return 0, w.ctx.Err()
		case <-w.released:
		}
	}
	return w.ResponseWriter.Write(data)
}

func serveHeldProjectWatch(t *testing.T, db *store.Store, sessions *auth.Sessions, base context.Context) (
	lgtmv1connect.ProjectServiceClient, *heldProjectWatchWriter, func(),
) {
	t.Helper()
	follower, err := store.StartFollower(t.Context(), db, nil)
	if err != nil {
		t.Fatal("The lag fixture could not start the Store follower.")
	}
	t.Cleanup(follower.Close)
	service := projects.NewService(db, sessions, machines.NewRegistry(), nil, follower, t.Context())
	_, handler := lgtmv1connect.NewProjectServiceHandler(service)
	handler = transport.WithWriteDeadline(handler)
	released, release := storetest.Signal(t)
	canceled, cancelWrite := storetest.Signal(t)
	writer := &heldProjectWatchWriter{entered: make(chan struct{}), released: released, canceled: canceled, cancelWrite: cancelWrite}
	var used atomic.Bool
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if used.CompareAndSwap(false, true) {
			writer.ResponseWriter, writer.ctx = w, r.Context()
			w = writer
		}
		handler.ServeHTTP(w, r)
	}))
	server.Config.BaseContext = func(net.Listener) context.Context { return base }
	server.Start()
	t.Cleanup(server.Close)
	t.Cleanup(release)
	return lgtmv1connect.NewProjectServiceClient(server.Client(), server.URL), writer, release
}

func appendUnrelatedProjectWatchReferences(t *testing.T, db *store.Store, count int) {
	t.Helper()
	for start := 0; start < count; start += store.MaxWriteChanges {
		err := db.Write(t.Context(), func(_ context.Context, tx *store.Tx) error {
			for i := start; i < min(start+store.MaxWriteChanges, count); i++ {
				if err := tx.Record(store.ChangeRef{Topic: "watch-fixture", Key: strconv.Itoa(i), Change: store.Saved}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal("The lag fixture could not commit a bounded batch of references.")
		}
	}
}

func TestProjectWatchLagAndReopen(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		cutoff bool
	}{{"reference_2047_stays_open", false}, {"reference_2048_aborts_and_reopens", true}} {
		t.Run(scenario.name, func(t *testing.T) {
			db := newProjectWatchStore(t, nil)
			sessions := auth.NewSessions(db, auth.Password{})
			base, _ := watchSessionContext(t, sessions)
			head := projectWatchHead(t, db)
			client, writer, release := serveHeldProjectWatch(t, db, sessions, base)
			stream := openProjectWatch(t, client)
			requireProjectWatchSnapshot(t, nextProjectWatch(t, stream, "the slow subscriber's snapshot"), head)
			fast := openProjectWatch(t, client)
			requireProjectWatchSnapshot(t, nextProjectWatch(t, fast, "the fast subscriber's snapshot"), head)

			save := func(name string) (*webv1.Repository, uint64) {
				repo := watchSeedRepository(t, name, watchSeedFolder(t, "/workspace/"+name))
				position := writeWatchSeed(t, db, func(ctx context.Context, tx *store.Tx) error {
					return insertWatchRepository(ctx, tx, repo)
				})
				return repo, position
			}
			writer.armed.Store(true)
			held, heldPosition := save("first")
			storetest.Receive(t, "the slow subscriber entering its first saved send", writer.entered)
			first := nextProjectWatch(t, fast, "the first saved reference on the fast watch")
			requireProjectWatchPosition(t, first, heldPosition)
			requireProjectWatchRecord(t, first.GetSaved(), held)

			// The held reference counts. The last reference below the cutoff is a saved marker.
			appendUnrelatedProjectWatchReferences(t, db, 2045)
			near, nearPosition := save("reference-2047")
			if nearPosition-head != 2047 {
				t.Fatalf("The fast marker is %d positions past the snapshot. It must be reference 2,047.", nearPosition-head)
			}
			nearFrame := nextProjectWatch(t, fast, "reference 2,047 on the fast watch")
			requireProjectWatchPosition(t, nearFrame, nearPosition)
			requireProjectWatchRecord(t, nearFrame.GetSaved(), near)

			if !scenario.cutoff {
				// Drain the slow watch below the cutoff to prove it stayed open independently of callback scheduling.
				release()
				heldFrame := nextProjectWatch(t, stream, "the released saved record below the cutoff")
				requireProjectWatchPosition(t, heldFrame, heldPosition)
				requireProjectWatchRecord(t, heldFrame.GetSaved(), held)
				nearFrame := nextProjectWatch(t, stream, "reference 2,047 on the released watch")
				requireProjectWatchPosition(t, nearFrame, nearPosition)
				requireProjectWatchRecord(t, nearFrame.GetSaved(), near)
				return
			}
			select {
			case <-writer.canceled:
				t.Fatal("The slow subscriber's held send was interrupted at or before reference 2,047.")
			default:
			}
			if !writer.blocked.Load() {
				t.Fatal("The slow subscriber stopped holding its first saved send before reference 2,048.")
			}
			trip, tripPosition := save("reference-2048")
			if tripPosition-head != 2048 {
				t.Fatalf("The final marker is %d positions past the snapshot. It must be reference 2,048.", tripPosition-head)
			}
			finalFrame := nextProjectWatch(t, fast, "reference 2,048 on the fast watch")
			requireProjectWatchPosition(t, finalFrame, tripPosition)
			requireProjectWatchRecord(t, finalFrame.GetSaved(), trip)
			storetest.Receive(t, "reference 2,048 interrupting the held send", writer.canceled)
			requireProjectWatchAborted(t, stream, "natural follower lag")
			reopened := openProjectWatch(t, client)
			requireProjectWatchSnapshot(t, nextProjectWatch(t, reopened, "the replacement snapshot after natural lag"), tripPosition, held, near, trip)
		})
	}
}
