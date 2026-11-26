package link

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func startLinkRunner(t *testing.T, f *handshakeFixture, controls *runnerControls) *storetest.Task[bool] {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	follower, err := store.StartFollower(ctx, f.s, nil)
	if err != nil {
		cancel()
		t.Fatal("The runner's follower could not start.")
	}
	t.Cleanup(follower.Close)
	if controls != nil {
		ctx = context.WithValue(ctx, runnerControlsKey{}, controls)
	}
	run := storetest.Start(t, func() bool { f.svc.Run(ctx, follower); return true })
	t.Cleanup(cancel)
	return run
}

func writeRunnerRefs(t *testing.T, f *handshakeFixture, refs ...store.ChangeRef) {
	t.Helper()
	if err := f.s.Write(t.Context(), func(_ context.Context, tx *store.Tx) error {
		for _, ref := range refs {
			if err := tx.Record(ref); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal("The runner's fixture references could not be published.")
	}
}

func runnerHandle(t *testing.T, f *handshakeFixture, id, stream string, generation int64) (*Handle, context.Context, context.CancelCauseFunc) {
	t.Helper()
	ctx, cancel := context.WithCancelCause(t.Context())
	h := f.hub.Track(id, stream, ctx, cancel)
	t.Cleanup(func() { cancel(nil); f.hub.Unregister(h) })
	if f.hub.Admit(h, func() (int64, bool, error) { return generation, false, nil }) != nil || f.hub.BeginWelcome(h) != nil {
		t.Fatal("The runner fixture could not install a live handle.")
	}
	f.hub.FinishWelcome(h)
	if !f.hub.Holds(id, stream) {
		t.Fatal("The runner fixture's handle is not live.")
	}
	return h, ctx, cancel
}

func runnerReady(t *testing.T, f *handshakeFixture, onStart func(*store.Subscription)) *storetest.Task[bool] {
	t.Helper()
	started := make(chan *store.Subscription, 8)
	run := startLinkRunner(t, f, &runnerControls{started: func(sub *store.Subscription) {
		started <- sub
		if onStart != nil {
			onStart(sub)
		}
	}})
	storetest.ReceiveFrom(t, run, "the initial feed subscription start", started)
	return run
}

func runnerOutbox(id string) store.ChangeRef {
	return store.ChangeRef{Topic: "machine_outbox", Key: id, Change: store.Saved}
}

func runnerRemoval(id string) store.ChangeRef {
	return store.ChangeRef{Topic: "machines", Key: id, Change: store.Removed}
}

func drainRunnerWake(h *Handle) {
	select {
	case <-h.Wake():
	default:
	}
}

func TestFollowWakesAndEnds(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	runnerReady(t, f, nil)
	first, firstCtx, _ := runnerHandle(t, f, fixtureStreamID(t), fixtureStreamID(t), 1)
	second, secondCtx, _ := runnerHandle(t, f, fixtureStreamID(t), fixtureStreamID(t), 1)
	writeRunnerRefs(t, f,
		store.ChangeRef{Topic: "other", Key: first.machineID, Change: store.Removed},
		runnerOutbox(second.machineID))
	storetest.Receive(t, "the target machine waking for its outbox", second.Wake())
	select {
	case <-first.Wake():
		t.Fatal("An unrelated topic woke a machine.")
	case <-firstCtx.Done():
		t.Fatal("An unrelated topic ended a machine.")
	default:
	}
	writeRunnerRefs(t, f, runnerRemoval(second.machineID))
	storetest.Receive(t, "the removed machine ending", secondCtx.Done())
	if !errors.Is(context.Cause(secondCtx), ErrUnknownMachine) || f.hub.Holds(second.machineID, second.streamID) {
		t.Fatal("Removal did not end only the exact removed handle.")
	}
}

func TestFollowResubscribes(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	first, firstCtx, _ := runnerHandle(t, f, fixtureStreamID(t), fixtureStreamID(t), 1)
	second, secondCtx, _ := runnerHandle(t, f, fixtureStreamID(t), fixtureStreamID(t), 1)
	release, resume := storetest.Signal(t)
	defer resume()
	started := make(chan *store.Subscription, 8)
	recovering := make(chan struct{}, 1)
	recoverGate, recoverNow := storetest.Signal(t)
	defer recoverNow()
	var subscriptions, starts atomic.Int32
	run := startLinkRunner(t, f, &runnerControls{
		subscribed: func(sub *store.Subscription) {
			if subscriptions.Add(1) == 2 {
				recovering <- struct{}{}
				select {
				case <-recoverGate:
				case <-sub.Context().Done():
				}
			}
		},
		started: func(sub *store.Subscription) {
			started <- sub
			if starts.Add(1) == 1 {
				select {
				case <-release:
				case <-t.Context().Done():
				}
			}
		},
	})
	old := storetest.ReceiveFrom(t, run, "the subscription held before consuming its feed", started)
	drainRunnerWake(first)
	drainRunnerWake(second)
	for offset := 0; offset <= store.MaxWatchLag; offset += store.MaxWriteChanges {
		size := min(store.MaxWriteChanges, store.MaxWatchLag+1-offset)
		rows := make([]store.ChangeRef, size)
		for i := range rows {
			rows[i] = store.ChangeRef{Topic: "other", Key: fmt.Sprintf("lag-%d", offset+i), Change: store.Saved}
		}
		writeRunnerRefs(t, f, rows...)
	}
	storetest.Receive(t, "the original subscriber exceeding its feed ring", old.Context().Done())
	if !errors.Is(context.Cause(old.Context()), store.ErrWatchLagged) {
		t.Fatal("The held subscriber never reached the lag condition.")
	}
	resume()
	storetest.ReceiveFrom(t, run, "the runner beginning a new subscription", recovering)
	storetest.Receive(t, "the first handle waking on feed failure", first.Wake())
	storetest.Receive(t, "the second handle waking on feed failure", second.Wake())
	recoverNow()
	if recovered := storetest.ReceiveFrom(t, run, "a new subscription after lag", started); recovered == old {
		t.Fatal("The runner attempted to restart its closed subscription.")
	}
	storetest.Receive(t, "the first live handle waking on feed recovery", first.Wake())
	storetest.Receive(t, "the second live handle waking on feed recovery", second.Wake())
	writeRunnerRefs(t, f, runnerRemoval(first.machineID), runnerOutbox(second.machineID))
	storetest.Receive(t, "removal from the recovered feed", firstCtx.Done())
	storetest.Receive(t, "an outbox wake from the recovered feed", second.Wake())
	if !errors.Is(context.Cause(firstCtx), ErrUnknownMachine) || secondCtx.Err() != nil {
		t.Fatal("The recovered feed did not distinguish removal and wake.")
	}
}

func TestFollowRecoversAfterSnapshotError(t *testing.T) {
	ready := make(chan *store.Subscription, 8)
	subscribed := make(chan *store.Subscription, 8)
	query := make(chan uint32, 1)
	release, resume := storetest.Signal(t)
	defer resume()
	var armed atomic.Bool
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
			if strings.Contains(data.SQL, "FROM feed_retention WHERE singleton") &&
				strings.Contains(data.SQL, "SELECT max(position) FROM feed") && armed.CompareAndSwap(true, false) {
				query <- conn.PgConn().PID()
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
			return ctx
		})
	}})
	a, aCtx, _ := runnerHandle(t, f, fixtureStreamID(t), fixtureStreamID(t), 1)
	b, _, _ := runnerHandle(t, f, fixtureStreamID(t), fixtureStreamID(t), 1)
	var attempts atomic.Int32
	run := startLinkRunner(t, f, &runnerControls{
		subscribed: func(sub *store.Subscription) {
			if attempts.Add(1) == 1 {
				armed.Store(true)
			}
			subscribed <- sub
		},
		started: func(sub *store.Subscription) { ready <- sub },
	})
	first := storetest.ReceiveFrom(t, run, "the initial subscription before its head read", subscribed)
	pid := storetest.ReceiveFrom(t, run, "the first head query", query)
	if !storetest.Scalar[bool](t, f.obs, "SELECT pg_terminate_backend($1, 5000)", int32(pid)) {
		t.Fatal("The head query's backend could not be terminated.")
	}
	resume()
	storetest.Receive(t, "the failed subscription closing", first.Context().Done())
	recovered := storetest.ReceiveFrom(t, run, "feed start after the failed head read", ready)
	if recovered == first {
		t.Fatal("The failed head read did not get a fresh subscription.")
	}
	storetest.Receive(t, "the live handle waking during head recovery", b.Wake())
	writeRunnerRefs(t, f, runnerRemoval(a.machineID), runnerOutbox(b.machineID))
	storetest.Receive(t, "removal on the recovered subscription", aCtx.Done())
	storetest.Receive(t, "outbox wake on the recovered subscription", b.Wake())
	if !errors.Is(context.Cause(aCtx), ErrUnknownMachine) {
		t.Fatal("The recovered subscription did not deliver removal.")
	}
}

func runnerConnection(t *testing.T, f *handshakeFixture, id, stream string) bool {
	t.Helper()
	return storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM link_connections WHERE machine_id = $1 AND stream_id = $2)", id, stream)
}

func TestFailedDisconnectIsRetried(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprintf("replacement %t", replace), func(t *testing.T) {
			var oldStream atomic.Value
			retried := make(chan struct{}, 4)
			var deleteAttempts atomic.Int32
			f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
				cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
					if id, ok := oldStream.Load().(string); ok && len(data.Args) == 2 && data.Args[1] == id &&
						strings.Contains(data.SQL, "DELETE FROM link_connections") && deleteAttempts.Add(1) > 1 {
						retried <- struct{}{}
					}
					return ctx
				})
			}})
			m := f.machine(t, bytes.Repeat([]byte{1}, 32), "retry disconnect")
			s := f.admit(t, m.ID, 1)
			oldStream.Store(s.ID)
			_, _, cancel := runnerHandle(t, f, m.ID, s.ID, 1)
			eventSQL(t, f.presenceFixture, `
				CREATE SEQUENCE runner_attempt;
				CREATE FUNCTION runner_refuse_first_delete() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN
				  IF nextval('runner_attempt') = 1 THEN RAISE EXCEPTION 'fixture rejects first delete'; END IF;
				  RETURN OLD;
				END $$;
				CREATE TRIGGER runner_refuse_first_delete BEFORE DELETE ON link_connections
				FOR EACH ROW EXECUTE FUNCTION runner_refuse_first_delete()`)
			cancel(nil)
			f.svc.queueDisconnect(m.ID, s.ID)
			f.svc.queueDisconnect(m.ID, s.ID)
			if f.hub.Holds(m.ID, s.ID) || !runnerConnection(t, f, m.ID, s.ID) {
				t.Fatal("Queueing a closed handle did not leave only its durable row before Run.")
			}
			startLinkRunner(t, f, nil)
			storetest.Poll(t, "the failed first disconnect attempt", func() bool {
				return storetest.Scalar[bool](t, f.obs, "SELECT is_called FROM runner_attempt")
			})
			if !replace {
				storetest.Poll(t, "the queued disconnect retry deleting its row", func() bool { return !runnerConnection(t, f, m.ID, s.ID) })
				if got := storetest.Scalar[int64](t, f.obs, "SELECT last_value FROM runner_attempt"); got != 2 {
					t.Fatal("The first disconnect did not retry to successful deletion.")
				}
				return
			}
			b := f.admit(t, m.ID, 2)
			_, _, _ = runnerHandle(t, f, m.ID, b.ID, 2)
			storetest.Receive(t, "the retry selecting the old exact stream after replacement", retried)
			if !runnerConnection(t, f, m.ID, b.ID) || !f.hub.Holds(m.ID, b.ID) {
				t.Fatal("Disconnection of the old identifier removed the replacement.")
			}
		})
	}
}

func TestFreshDisconnectBypassesCleanupBacklog(t *testing.T) {
	var firstLocked atomic.Value
	var attempts atomic.Int32
	retryEntered := make(chan struct{}, 1)
	retryHeld, releaseRetry := storetest.Signal(t)
	defer releaseRetry()
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
			id, ok := firstLocked.Load().(string)
			if ok && len(data.Args) == 1 && data.Args[0] == id &&
				strings.Contains(strings.ToUpper(data.SQL), "FOR NO KEY UPDATE NOWAIT") && attempts.Add(1) == 2 {
				retryEntered <- struct{}{}
				<-retryHeld
			}
			return ctx
		})
	}})
	var ids, streams []string
	for i := 0; i < 6; i++ {
		m := f.machine(t, bytes.Repeat([]byte{byte(i + 1)}, 32), fmt.Sprintf("locked %d", i))
		ids = append(ids, m.ID)
		streams = append(streams, f.admit(t, m.ID, 1).ID)
	}
	free := f.machine(t, bytes.Repeat([]byte{9}, 32), "unlocked cleanup")
	fresh := f.admit(t, free.ID, 1)
	locked, unlock := storetest.Signal(t)
	defer unlock()
	holding := make(chan struct{})
	task := storetest.Start(t, func() error {
		return f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if _, err := tx.Exec(ctx, "SELECT id FROM machines WHERE id = ANY($1) FOR NO KEY UPDATE", ids); err != nil {
				return err
			}
			close(holding)
			select {
			case <-locked:
				return nil
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		})
	})
	storetest.ReceiveFrom(t, task, "the registry rows under fixture locks", holding)
	firstLocked.Store(ids[0])
	for i, id := range ids {
		f.svc.queueDisconnect(id, streams[i])
	}
	startLinkRunner(t, f, nil)
	storetest.Receive(t, "an older queued cleanup entering its retry lane", retryEntered)
	// Keep all older machine rows locked through the fresh deletion.
	before := f.head(t)
	f.svc.queueDisconnect(free.ID, fresh.ID)
	deadline := time.Now().Add(5 * time.Second)
	storetest.Poll(t, "fresh cleanup and its feed change within five seconds", func() bool {
		if time.Now().After(deadline) {
			t.Fatal("Already queued locked machines delayed the fresh disconnect past five seconds.")
		}
		return !runnerConnection(t, f, free.ID, fresh.ID) && f.head(t) > before
	})
	f.requireSaved(t, before, free.ID)
	for i, id := range ids {
		if !runnerConnection(t, f, id, streams[i]) {
			t.Fatal("A locked connection disappeared before its row lock was released.")
		}
	}
	releaseRetry()
	unlock()
	if task.Wait(t) != nil {
		t.Fatal("The fixture could not release its registry locks.")
	}
	for i, id := range ids {
		storetest.Poll(t, "a previously locked stream clearing on retry", func() bool { return !runnerConnection(t, f, id, streams[i]) })
	}
}

func TestLifecycleCancellationDoesNotWaitForCleanup(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	life, shutdown := context.WithCancel(t.Context())
	defer shutdown()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{21}, ed25519.SeedSize))
	f.svc = NewStreamingService(life, f.s, f.tokens, f.r, f.state, f.hub, key, Options{})
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "held cleanup")
	s := f.admit(t, m.ID, 1)
	// The observer owns the advisory lock. PostgreSQL shows independently when
	// the cleanup transaction is waiting on it.
	const gate int64 = 145290
	release := storetest.HoldAdvisoryGate(t, f.obs, gate)
	defer release()
	eventSQL(t, f.presenceFixture, `CREATE FUNCTION runner_hold_delete() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_advisory_xact_lock(145290); RETURN OLD; END $$;
		CREATE TRIGGER runner_hold_delete BEFORE DELETE ON link_connections
		FOR EACH ROW EXECUTE FUNCTION runner_hold_delete()`)
	streams := make([]context.Context, 4)
	cancels := make([]context.CancelCauseFunc, len(streams))
	for i := range streams {
		streams[i], cancels[i] = f.svc.streamContext(t.Context())
		defer cancels[i](nil)
	}
	watchdog := connect.NewError(connect.CodeDeadlineExceeded, errors.New(openTimeout))
	cancels[3](watchdog)
	f.svc.queueDisconnect(m.ID, s.ID)
	startLinkRunner(t, f, nil)
	storetest.Poll(t, "the real cleanup transaction blocked on its advisory gate", func() bool {
		return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND $1 = ANY(pg_blocking_pids(pid)))`, int32(f.obs.PgConn().PID()))
	})
	shutdown()
	for i, ctx := range streams {
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
			t.Fatalf("Phase %d did not receive shutdown while cleanup was held.", i)
		}
		got := context.Cause(ctx)
		if i == 3 {
			if got != watchdog {
				t.Fatal("Shutdown replaced the watchdog's earlier cause.")
			}
		} else if got == nil || !strings.Contains(got.Error(), "The server is stopping.") {
			t.Fatalf("Phase %d did not retain the fixed shutdown refusal.", i)
		}
	}
	release()
}

func TestPeerAddress(t *testing.T) {
	for _, tc := range []struct {
		address string
		private bool
		want    string
	}{
		{"10.2.3.4:4141", false, "10.2.3.4"},
		{"[::1]:4141", false, "::1"},
		{"10.2.3.4:4141", true, ""},
		{"malformed-address", false, ""},
	} {
		if got := peerAddress(tc.address, tc.private); got != tc.want {
			t.Errorf("The peer projection for private=%t did not match the expected address.", tc.private)
		}
	}
}

func TestOpenUsesTransportPeerPolicy(t *testing.T) {
	for _, private := range []bool{false, true} {
		t.Run(fmt.Sprintf("private stack %t", private), func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{privateStack: private})
			m, key := f.register(t, "transport peer")
			wrap := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r.RemoteAddr = "10.2.3.4:4141"
					next.ServeHTTP(w, r)
				})
			}
			client := f.wire(t, handshakeWireOptions{wrap: wrap})
			for i, header := range []string{"", "127.0.0.1", "malformed", "127.0.0.1, ::1"} {
				var headers map[string]string
				if header != "" {
					headers = map[string]string{"X-Forwarded-For": header, "Forwarded": "for=" + header}
				}
				stream := startHandshake(t, client, headers)
				submitProof(t, f, stream, m.ID, key, byte(i+1))
				requireWelcome(t, stream)
				want := "10.2.3.4"
				if private {
					want = ""
				}
				if f.current(t, m.ID).Address != want {
					t.Fatal("The admitted address did not follow the transport-peer policy.")
				}
				_ = stream.CloseRequest()
				_ = stream.CloseResponse()
			}
		})
	}
}
