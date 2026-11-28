package projects_test

import (
	"context"
	"crypto/sha256"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	retireWorkerName   = "projects retirement fixture"
	retireReplayName   = "projects replay fixture"
	retireDeleteGateID = int64(52819)
	retireFaultText    = "fixture private diagnostic"
)

type retireClock struct {
	config *pgxpool.Config
	now    time.Time
}

func newRetireClock(t *testing.T, f *operationFixture) retireClock {
	t.Helper()
	// Use the same database clock seam as the Store and session fixtures.
	storetest.Exec(t, f.observer, `
CREATE TABLE retire_fixture_clock (at timestamptz NOT NULL);
CREATE FUNCTION clock_timestamp() RETURNS timestamptz LANGUAGE sql STABLE AS
$$ SELECT at FROM retire_fixture_clock $$;`)
	now := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	storetest.Exec(t, f.observer, "INSERT INTO retire_fixture_clock VALUES ($1)", now)
	config := f.config.Copy()
	config.ConnConfig.RuntimeParams["search_path"] += ", pg_catalog"
	config.ConnConfig.RuntimeParams["application_name"] = retireWorkerName
	return retireClock{config: config, now: now}
}

func (c retireClock) set(t *testing.T, f *operationFixture, at time.Time) {
	t.Helper()
	storetest.Exec(t, f.observer, "UPDATE retire_fixture_clock SET at = $1", at)
}

type retireOperation struct {
	call          operationCall
	id, commandID string
}

func startRetireOperation(t *testing.T, f *operationFixture, key string) retireOperation {
	t.Helper()
	call := operationCall{key: key, machine: f.machine, path: "/workspace/checkout "}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	task := call.start(t, ctx, f.service(t, f.db, f.state))
	id, commandID := f.awaitOperation(t, call, task)
	cancel()
	requireOperationRefusal(t, task.Wait(t), connect.CodeCanceled)
	return retireOperation{call: call, id: id, commandID: commandID}
}

func (o retireOperation) stamp(t *testing.T, f *operationFixture, at time.Time) {
	t.Helper()
	storetest.Exec(t, f.observer, "UPDATE folder_operations SET retention_started_at = $2 WHERE id = $1", o.id, at)
}

func (o retireOperation) stampedAt(t *testing.T, f *operationFixture, at *time.Time) bool {
	t.Helper()
	return storetest.Scalar[bool](t, f.observer, `
SELECT EXISTS (SELECT FROM folder_operations WHERE id = $1 AND retention_started_at IS NOT DISTINCT FROM $2::timestamptz)`, o.id, at)
}

// rowCount independently observes the operation and its held receipt in a single snapshot.
func (o retireOperation) rowCount(t *testing.T, f *operationFixture) int {
	t.Helper()
	return storetest.Scalar[int](t, f.observer, `
SELECT (SELECT count(*) FROM folder_operations WHERE id = $1)
     + (SELECT count(*) FROM receipts WHERE request_key = $2 AND held AND reply = convert_to($1, 'UTF8'))`,
		o.id, o.call.key)
}

func (o retireOperation) requireRetired(t *testing.T, f *operationFixture) {
	t.Helper()
	if o.rowCount(t, f) != 0 || storetest.Scalar[bool](t, f.observer,
		"SELECT EXISTS (SELECT FROM receipts WHERE request_key = $1)", o.call.key) {
		t.Fatal("Retirement left the operation or its request key behind.")
	}
}

func (o retireOperation) requireConflicts(t *testing.T, f *operationFixture, s *projects.Service) {
	t.Helper()
	changed := o.call
	changed.path = "/workspace/checkout"
	requireOperationRefusal(t, changed.run(t.Context(), s, changed.key), connect.CodeAlreadyExists, operationConflict)
	changed = o.call
	changed.add = true
	requireOperationRefusal(t, changed.run(t.Context(), s, changed.key), connect.CodeAlreadyExists, operationConflict)
	request := connect.NewRequest(&webv1.RenameClientRequest{Id: f.machine, Name: "Renamed fixture"})
	request.Header().Set(store.RequestKeyHeader, o.call.key)
	response, err := machines.NewService(f.db, nil, f.registry, nil).RenameClient(t.Context(), request)
	if response != nil {
		t.Error("A Machines action reused a key held by a folder operation.")
	}
	requireOperationRefusal(t, operationResult{err: err}, connect.CodeAlreadyExists, operationConflict)
}

type retireWorker struct {
	task    *storetest.Task[struct{}]
	reports <-chan error
}

func startRetireWorker(t *testing.T, s *projects.Service) (retireWorker, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	reports := make(chan error, 4)
	task := storetest.Start(t, func() struct{} {
		s.RunRetirement(ctx, func(err error) {
			select {
			case reports <- err:
			case <-ctx.Done():
			}
		})
		return struct{}{}
	})
	t.Cleanup(cancel)
	return retireWorker{task: task, reports: reports}, cancel
}

func (w retireWorker) await(t *testing.T, condition string, observe func() bool) {
	t.Helper()
	w.awaitWithin(t, storetest.ObservationTimeout, condition, observe)
}

func (w retireWorker) awaitWithin(t *testing.T, limit time.Duration, condition string, observe func() bool) {
	t.Helper()
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-w.reports:
			t.Fatalf("The retirement worker reported a failure while the test was waiting for %s.", condition)
		case <-w.task.Done():
			t.Fatalf("The retirement worker exited while the test was waiting for %s.", condition)
		case <-deadline.C:
			t.Fatalf("The test reached its %v limit while waiting for %s.", limit, condition)
		case <-tick.C:
			if observe() {
				return
			}
		}
	}
}

func (w retireWorker) report(t *testing.T) error {
	t.Helper()
	select {
	case err := <-w.reports:
		if err == nil {
			t.Fatal("The retirement worker reported an empty diagnostic.")
		}
		return err
	case <-w.task.Done():
		t.Fatal("The retirement worker exited before reporting a diagnostic.")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("The retirement worker did not report the diagnostic.")
	}
	return nil
}

func (w retireWorker) stop(t *testing.T, cancel func()) {
	t.Helper()
	cancel()
	w.task.Wait(t)
}

func TestProjectReceiptRetention(t *testing.T) {
	t.Run("open and terminal keys survive their retention", testRetirementKeepsKeys)
	t.Run("equal timestamps across a cohort", testRetirementStampCohort)
}

func testRetirementKeepsKeys(t *testing.T) {
	f := newOperationFixture(t)
	f.approve(t)
	clock := newRetireClock(t, f)
	db := openOperationStore(t, clock.config)
	s := f.service(t, db, f.state)

	open := startRetireOperation(t, f, "old-open-fixture")
	recent := startRetireOperation(t, f, "old-just-settled-fixture")
	retained := startRetireOperation(t, f, "retained-failure-fixture")
	mature := startRetireOperation(t, f, "mature-control-fixture")
	if _, err := db.WriteOnce(t.Context(), store.Request{
		Caller: "owner", Key: "ordinary-fixture", Procedure: "fixture", ReceiptSafe: true,
		FingerprintVersion: 1, Fingerprints: map[int16][]byte{1: []byte("fixture")},
	}, func(context.Context, *store.Tx) ([]byte, error) {
		return []byte("ordinary reply"), nil
	}); err != nil {
		t.Fatal("The ordinary receipt could not be admitted.")
	}

	// Age the admission records independently of the operations' terminal observation times.
	old := clock.now.Add(-72 * time.Hour)
	storetest.Exec(t, f.observer, "UPDATE receipts SET written_at = $1, retention_started_at = $1", old)
	storetest.Exec(t, f.observer, `
UPDATE link_commands SET created_at = $1::timestamptz, expires_at = $1::timestamptz + interval '20 seconds'
WHERE action = 'look_at_folder'`, old)
	wantRecent := f.settleFixtureReply(t, recent.id, inspectOperationReply())
	stamp := clock.now.Add(-24*time.Hour + time.Microsecond)
	storetest.Exec(t, f.observer, `
UPDATE folder_operations SET state = 'failed', failure = 'machine_removed', retention_started_at = $2 WHERE id = $1`,
		retained.id, stamp)
	f.settleFixtureReply(t, mature.id, inspectOperationReply())
	mature.stamp(t, f, clock.now.Add(-25*time.Hour))
	f.disconnect(t)

	result, err := db.Prune(t.Context())
	if err != nil || result.ReceiptRows != 1 ||
		storetest.Scalar[bool](t, f.observer, "SELECT EXISTS (SELECT FROM receipts WHERE request_key = 'ordinary-fixture')") {
		t.Error("Ordinary pruning did not delete exactly the unrelated ordinary receipt.")
	}
	for _, op := range []retireOperation{open, recent, retained, mature} {
		if op.rowCount(t, f) != 2 {
			t.Fatal("Ordinary pruning removed a held request key or its operation.")
		}
	}

	worker, stop := startRetireWorker(t, s)
	defer stop()
	worker.await(t, "the first committed terminal observation and the mature control's removal", func() bool {
		return recent.stampedAt(t, f, &clock.now) && mature.rowCount(t, f) == 0
	})
	worker.stop(t, stop)
	mature.requireRetired(t, f)
	if !open.stampedAt(t, f, nil) || !retained.stampedAt(t, f, &stamp) {
		t.Fatal("The startup sweep stamped an open operation or changed an existing terminal stamp.")
	}
	for _, op := range []retireOperation{open, recent, retained} {
		op.requireConflicts(t, f, s)
	}
	requireOperationReply(t, recent.call.run(t.Context(), s, recent.call.key), wantRecent)
	f.requireTerminalReply(t, recent.id, recent.commandID, wantRecent)
	requireOperationRefusal(t, retained.call.run(t.Context(), s, retained.call.key), connect.CodeNotFound, "The machine was removed.")
	if retained.rowCount(t, f) != 2 {
		t.Fatal("The failed operation lost its receipt within its retained day.")
	}

	// A matching offline retry reads the old open operation and waits for its original result.
	read := make(chan struct{}, 1)
	config := clock.config.Copy()
	config.ConnConfig.Tracer = operationQueryTrace(func(_ context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) {
		if retireOperationRead(query, open.id) {
			select {
			case read <- struct{}{}:
			default:
			}
		}
	})
	replayDB := openOperationStore(t, config)
	replayCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	task := open.call.start(t, replayCtx, f.service(t, replayDB, f.state))
	storetest.Receive(t, "a replay read of the original open operation", read)
	wantOpen := f.settleFixtureReply(t, open.id, inspectOperationReply())
	requireOperationReply(t, task.Wait(t), wantOpen)
	f.requireTerminalReply(t, open.id, open.commandID, wantOpen)
	if !open.stampedAt(t, f, nil) {
		t.Error("Settlement itself stamped the open operation's retention time.")
	}

	next := clock.now.Add(24*time.Hour + time.Microsecond)
	clock.set(t, f, next)
	worker, stop = startRetireWorker(t, s)
	defer stop()
	worker.await(t, "the mature operation and receipt pairs to retire", func() bool {
		return open.stampedAt(t, f, &next) && recent.rowCount(t, f) == 0 && retained.rowCount(t, f) == 0
	})
	worker.stop(t, stop)
	recent.requireRetired(t, f)
	retained.requireRetired(t, f)
	requireOperationReply(t, open.call.run(t.Context(), s, open.call.key), wantOpen)
	f.requireTerminalReply(t, open.id, open.commandID, wantOpen)
	f.requireCounts(t, operationCounts{operations: 1, commands: 4, receipts: 1, wakes: 4})
}

func testRetirementStampCohort(t *testing.T) {
	f := newOperationFixture(t)
	clock := newRetireClock(t, f)
	db := openOperationStore(t, clock.config)
	s := f.service(t, db, f.state)
	// Exceed Store's usual maintenance page of 512 with ties on the entire cohort.
	const cohort = 520
	storetest.Exec(t, f.observer, `
INSERT INTO folder_operations (id, machine_id, request_key, kind, command_id, state, failure)
SELECT '00000000-0000-7000-8000-' || lpad(n::text, 12, '0'), $1, 'page-fixture-' || n,
       'inspect', '00000000-0000-7000-8001-' || lpad(n::text, 12, '0'), 'failed', 'machine_removed'
FROM generate_series(1, $2::int) AS n`, f.machine, cohort)
	principal := sha256.Sum256([]byte("owner"))
	storetest.Exec(t, f.observer, `
INSERT INTO receipts (principal_hash, request_key, fingerprint_version, request_hash, reply, held)
SELECT $1, request_key, 1, $1, convert_to(id, 'UTF8'), true FROM folder_operations`, principal[:])

	worker, stop := startRetireWorker(t, s)
	defer stop()
	worker.await(t, "every operation in the cohort to receive the same first observation", func() bool {
		return storetest.Scalar[int](t, f.observer,
			"SELECT count(*) FROM folder_operations WHERE retention_started_at = $1", clock.now) == cohort
	})
	worker.stop(t, stop)
	clock.set(t, f, clock.now.Add(25*time.Hour))
	worker, stop = startRetireWorker(t, s)
	defer stop()
	// A sweep can commit one transaction per candidate. Leave room for concurrent database tests.
	worker.awaitWithin(t, time.Minute, "every tied operation and held receipt to retire", func() bool {
		return storetest.Scalar[bool](t, f.observer, `
SELECT NOT EXISTS (SELECT FROM folder_operations) AND NOT EXISTS (SELECT FROM receipts)`)
	})
	worker.stop(t, stop)
}

func TestProjectReplayRetirementRace(t *testing.T) {
	t.Run("replay wins the receipt lock", testReplayWinsRetirement)
	t.Run("retirement wins the receipt lock", testRetirementWinsReplay)
	t.Run("two retirement workers delete once", testConcurrentRetirement)
	t.Run("shutdown cancels the active delete", testRetirementShutdown)
	t.Run("receipt delete fault rolls back both rows", func(t *testing.T) { testRetirementDeleteFault(t, "receipts") })
	t.Run("operation delete fault rolls back both rows", func(t *testing.T) { testRetirementDeleteFault(t, "folder_operations") })
	t.Run("lost reply for committed retirement", func(t *testing.T) { testRetirementLostCommit(t, false) })
	t.Run("lost reply for rolled-back retirement", func(t *testing.T) { testRetirementLostCommit(t, true) })
	t.Run("a missing held receipt is reported", testRetirementMissingReceipt)
}

func matureRetireFixture(t *testing.T) (*operationFixture, retireClock, retireOperation, []byte) {
	t.Helper()
	f := newOperationFixture(t)
	f.approve(t)
	clock := newRetireClock(t, f)
	op := startRetireOperation(t, f, "retirement-race-fixture")
	reply := f.settleFixtureReply(t, op.id, inspectOperationReply())
	op.stamp(t, f, clock.now.Add(-25*time.Hour))
	return f, clock, op, reply
}

func retireOperationRead(query pgx.TraceQueryStartData, id string) bool {
	sql := strings.ToLower(strings.TrimSpace(query.SQL))
	if !strings.Contains(sql, "folder_operations") ||
		(!strings.HasPrefix(sql, "select") && !strings.HasPrefix(sql, "with")) {
		return false
	}
	for _, arg := range query.Args {
		if value, ok := arg.(string); ok && value == id {
			return true
		}
	}
	return false
}

type retireCommitTrace struct {
	operationQueryTrace
	afterCommit func(context.Context)
}

type retireCommitTraceKey struct{}

func (trace retireCommitTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if trace.operationQueryTrace != nil {
		ctx = trace.operationQueryTrace.TraceQueryStart(ctx, conn, query)
	}
	if strings.EqualFold(strings.TrimSpace(query.SQL), "commit") {
		return context.WithValue(ctx, retireCommitTraceKey{}, true)
	}
	return ctx
}

func (trace retireCommitTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryEndData) {
	if committed, _ := ctx.Value(retireCommitTraceKey{}).(bool); committed && query.Err == nil {
		trace.afterCommit(ctx)
	}
}

func retireBlocked(t *testing.T, f *operationFixture, workerName string, blocker int32) bool {
	t.Helper()
	return storetest.Scalar[bool](t, f.observer, `
SELECT EXISTS (SELECT FROM pg_stat_activity
WHERE datname = current_database() AND application_name = $1
  AND wait_event = 'advisory' AND $2 = ANY (pg_blocking_pids(pid)))`, workerName, blocker)
}

func testReplayWinsRetirement(t *testing.T) {
	f, clock, op, want := matureRetireFixture(t)
	f.disconnect(t)
	replayConfig := f.config.Copy()
	replayConfig.ConnConfig.RuntimeParams["application_name"] = retireReplayName
	readEntered := make(chan int32, 1)
	readHold, releaseRead := storetest.Signal(t)
	defer releaseRead()
	commitEntered := make(chan struct{}, 1)
	commitHold, releaseCommit := storetest.Signal(t)
	defer releaseCommit()
	var readSeen, commitSeen atomic.Bool
	replayConfig.ConnConfig.Tracer = retireCommitTrace{
		operationQueryTrace: func(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) {
			if retireOperationRead(query, op.id) && readSeen.CompareAndSwap(false, true) {
				readEntered <- int32(conn.PgConn().PID())
				select {
				case <-readHold:
				case <-ctx.Done():
				}
			}
		},
		afterCommit: func(ctx context.Context) {
			if readSeen.Load() && commitSeen.CompareAndSwap(false, true) {
				commitEntered <- struct{}{}
				select {
				case <-commitHold:
				case <-ctx.Done():
				}
			}
		},
	}
	replayDB := openOperationStore(t, replayConfig)
	replayCtx, cancelReplay := context.WithCancel(t.Context())
	defer cancelReplay()
	replay := op.call.start(t, replayCtx, f.service(t, replayDB, f.state))
	blocker := storetest.Receive(t, "the first admitted operation read", readEntered)

	db := openOperationStore(t, clock.config)
	worker, stop := startRetireWorker(t, f.service(t, db, f.state))
	defer stop()
	worker.await(t, "retirement to wait on the admitted replay's receipt lock", func() bool {
		if op.rowCount(t, f) != 2 {
			t.Fatal("Retirement deleted the result before the admitted replay's first operation read.")
		}
		return retireBlocked(t, f, retireWorkerName, blocker)
	})
	releaseRead()
	storetest.Receive(t, "the replay commit after the terminal read", commitEntered)
	worker.await(t, "both rows to disappear before the replay returns", func() bool { return op.rowCount(t, f) == 0 })
	op.requireRetired(t, f)
	releaseCommit()
	requireOperationReply(t, replay.Wait(t), want)
	f.requireCounts(t, operationCounts{commands: 1, wakes: 1})
	worker.stop(t, stop)
}

func holdRetireDelete(t *testing.T, f *operationFixture) func() {
	t.Helper()
	storetest.Exec(t, f.observer, `
CREATE TABLE retire_fixture_deletes (id text NOT NULL);
CREATE FUNCTION retire_fixture_delete_gate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(52819::bigint);
    INSERT INTO retire_fixture_deletes VALUES (OLD.id);
    RETURN OLD;
END;
$$;
CREATE TRIGGER retire_fixture_delete_gate BEFORE DELETE ON folder_operations
    FOR EACH ROW EXECUTE FUNCTION retire_fixture_delete_gate();`)
	return storetest.HoldAdvisoryGate(t, f.observer, retireDeleteGateID)
}

func awaitRetireDelete(t *testing.T, f *operationFixture, worker retireWorker) int32 {
	t.Helper()
	var pid int32
	worker.await(t, "retirement to reach the operation delete under the receipt lock", func() bool {
		pid = storetest.Scalar[int32](t, f.observer, `
SELECT coalesce((SELECT pid FROM pg_stat_activity
    WHERE datname = current_database() AND application_name = $1
      AND wait_event = 'advisory' AND pg_backend_pid() = ANY (pg_blocking_pids(pid))
    LIMIT 1), 0)`, retireWorkerName)
		return pid != 0
	})
	return pid
}

func testRetirementWinsReplay(t *testing.T) {
	f, clock, op, _ := matureRetireFixture(t)
	f.disconnect(t)
	release := holdRetireDelete(t, f)
	defer release()
	db := openOperationStore(t, clock.config)
	worker, stop := startRetireWorker(t, f.service(t, db, f.state))
	defer stop()
	blocker := awaitRetireDelete(t, f, worker)
	replayConfig := f.config.Copy()
	replayConfig.ConnConfig.RuntimeParams["application_name"] = retireReplayName
	replayDB := openOperationStore(t, replayConfig)
	replayCtx, cancelReplay := context.WithCancel(t.Context())
	defer cancelReplay()
	offline := f.service(t, replayDB, f.state)
	replay := op.call.start(t, replayCtx, offline)
	worker.await(t, "the retry to wait on retirement's receipt lock", func() bool {
		select {
		case <-replay.Done():
			t.Fatal("The retry returned while retirement still held the receipt lock.")
		default:
		}
		return retireBlocked(t, f, retireReplayName, blocker)
	})
	if op.rowCount(t, f) != 2 {
		t.Fatal("One row disappeared before retirement committed.")
	}
	release()
	requireOperationRefusal(t, replay.Wait(t), connect.CodeUnavailable, operationUnavailable)
	op.requireRetired(t, f)
	worker.stop(t, stop)
	f.requireCounts(t, operationCounts{commands: 1, wakes: 1})

	reconnected := retireReconnect(t, f)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fresh := op.call.start(t, ctx, f.service(t, f.db, reconnected))
	id, commandID := f.awaitOperation(t, op.call, fresh)
	if id == op.id || commandID == op.commandID {
		t.Error("Fresh online admission reused the retired operation or command identifier.")
	}
	want := f.settleFixtureReply(t, id, inspectOperationReply())
	requireOperationReply(t, fresh.Wait(t), want)
	f.requireCounts(t, operationCounts{1, 2, 1, 2})
}

func retireReconnect(t *testing.T, f *operationFixture) *link.State {
	t.Helper()
	// The signed fixture has disconnected. Admit a new held stream for that same machine.
	hub := link.NewHub()
	state := link.NewState(f.db, f.registry, hub)
	ctx, cancel := context.WithCancelCause(t.Context())
	streamID := projectMigrationID(t)
	handle := hub.Track(f.machine, streamID, ctx, cancel)
	t.Cleanup(func() {
		cancel(nil)
		hub.Unregister(handle)
	})
	announcement := link.RunAnnouncement{ID: projectMigrationID(t), FirstEventNumber: 1}
	if err := hub.Admit(handle, func() (int64, bool, error) {
		stream, err := state.BecomeCurrent(ctx, f.machine, streamID, "127.0.0.1", announcement)
		return stream.Generation, false, err
	}); err != nil {
		t.Fatal("The replacement stream could not be admitted.")
	}
	if err := hub.BeginWelcome(handle); err != nil {
		t.Fatal("The replacement stream could not start its welcome.")
	}
	hub.FinishWelcome(handle)
	return state
}

func testConcurrentRetirement(t *testing.T) {
	f, clock, op, _ := matureRetireFixture(t)
	release := holdRetireDelete(t, f)
	defer release()
	db := openOperationStore(t, clock.config)
	first, stopFirst := startRetireWorker(t, f.service(t, db, f.state))
	defer stopFirst()
	blocker := awaitRetireDelete(t, f, first)

	config := clock.config.Copy()
	config.ConnConfig.RuntimeParams["application_name"] = "second retirement fixture"
	committed := make(chan struct{}, 1)
	var armed atomic.Bool
	config.ConnConfig.Tracer = retireCommitTrace{afterCommit: func(context.Context) {
		if armed.CompareAndSwap(true, false) {
			committed <- struct{}{}
		}
	}}
	secondDB := openOperationStore(t, config)
	second, stopSecond := startRetireWorker(t, f.service(t, secondDB, f.state))
	defer stopSecond()
	second.await(t, "the second retirement worker to wait on the same receipt key", func() bool {
		return retireBlocked(t, f, "second retirement fixture", blocker)
	})
	armed.Store(true)
	release()
	second.await(t, "the second retirement worker to commit its no-op", func() bool {
		select {
		case <-committed:
			return true
		default:
			return false
		}
	})
	second.stop(t, stopSecond)
	first.stop(t, stopFirst)
	op.requireRetired(t, f)
	if storetest.Scalar[int](t, f.observer, "SELECT count(*) FROM retire_fixture_deletes WHERE id = $1", op.id) != 1 {
		t.Fatal("The two retirement workers did not produce exactly one committed operation delete.")
	}
}

func testRetirementShutdown(t *testing.T) {
	f, clock, op, want := matureRetireFixture(t)
	release := holdRetireDelete(t, f)
	defer release()
	db := openOperationStore(t, clock.config)
	worker, stop := startRetireWorker(t, f.service(t, db, f.state))
	defer stop()
	awaitRetireDelete(t, f, worker)
	worker.stop(t, stop)
	// Keep the gate held until the worker has exited and its interrupted write is checked.
	if op.rowCount(t, f) != 2 {
		t.Fatal("Shutdown interrupted retirement after only one row was committed.")
	}
	f.requireTerminalReply(t, op.id, op.commandID, want)
}

func testRetirementDeleteFault(t *testing.T, table string) {
	f, clock, op, want := matureRetireFixture(t)
	storetest.Exec(t, f.observer, `
CREATE FUNCTION retire_fixture_refuse_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'fixture private diagnostic' USING ERRCODE = '23514';
END;
$$;
CREATE TRIGGER retire_fixture_refuse_delete BEFORE DELETE ON `+table+`
    FOR EACH ROW EXECUTE FUNCTION retire_fixture_refuse_delete();`)
	db := openOperationStore(t, clock.config)
	s := f.service(t, db, f.state)
	worker, stop := startRetireWorker(t, s)
	defer stop()
	requireRetireDiagnostic(t, worker.report(t), op)
	worker.stop(t, stop)
	if op.rowCount(t, f) != 2 {
		t.Fatal("The delete fault left the operation and held receipt in different committed states.")
	}
	f.requireTerminalReply(t, op.id, op.commandID, want)
	requireOperationReply(t, op.call.run(t.Context(), s, op.call.key), want)

	storetest.Exec(t, f.observer, "DROP TRIGGER retire_fixture_refuse_delete ON "+table)
	worker, stop = startRetireWorker(t, s)
	defer stop()
	worker.await(t, "both rows to retire after the fault clears", func() bool { return op.rowCount(t, f) == 0 })
	worker.stop(t, stop)
	op.requireRetired(t, f)
}

func testRetirementLostCommit(t *testing.T, rollback bool) {
	f, clock, op, want := matureRetireFixture(t)
	f.disconnect(t)
	release := holdRetireDelete(t, f)
	defer release()

	config := clock.config.Copy()
	var armed, dropped atomic.Bool
	dial := config.ConnConfig.DialFunc
	config.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &operationCommitWire{Conn: conn, armed: &armed, dropped: &dropped, abort: rollback}, nil
	}
	db := openOperationStore(t, config)
	worker, stop := startRetireWorker(t, f.service(t, db, f.state))
	defer stop()
	awaitRetireDelete(t, f, worker)
	// Only the retirement write is armed after the operation delete reaches the gate.
	armed.Store(true)
	release()
	requireRetireDiagnostic(t, worker.report(t), op)
	if !dropped.Load() {
		t.Fatal("The fixture did not lose the retirement commit reply.")
	}
	worker.stop(t, stop)
	db.Close()

	db = openOperationStore(t, f.config)
	s := f.service(t, db, link.NewState(db, f.registry, link.NewHub()))
	if rollback {
		if op.rowCount(t, f) != 2 {
			t.Fatal("Reopening after an unconfirmed rollback did not preserve both rows.")
		}
		f.requireTerminalReply(t, op.id, op.commandID, want)
		requireOperationReply(t, op.call.run(t.Context(), s, op.call.key), want)
		return
	}
	op.requireRetired(t, f)
	requireOperationRefusal(t, op.call.run(t.Context(), s, op.call.key), connect.CodeUnavailable, operationUnavailable)
	f.requireCounts(t, operationCounts{commands: 1, wakes: 1})
}

func requireRetireDiagnostic(t *testing.T, err error, op retireOperation) {
	t.Helper()
	message := err.Error()
	if strings.TrimSpace(message) == "" {
		t.Fatal("The retirement diagnostic was empty.")
	}
	for _, value := range []string{op.call.key, strings.TrimSpace(op.call.path), op.id, op.commandID, retireFaultText} {
		if strings.Contains(message, value) {
			t.Fatal("The retirement diagnostic exposed private fixture data.")
		}
	}
}

func testRetirementMissingReceipt(t *testing.T) {
	f, clock, first, _ := matureRetireFixture(t)
	db := openOperationStore(t, clock.config)
	s := f.service(t, db, f.state)
	missingReceipt := func(op retireOperation) string {
		storetest.Exec(t, f.observer, "DELETE FROM receipts WHERE request_key = $1", op.call.key)
		worker, stop := startRetireWorker(t, s)
		defer stop()
		err := worker.report(t)
		requireRetireDiagnostic(t, err, op)
		worker.stop(t, stop)
		if op.rowCount(t, f) != 1 {
			t.Fatal("Retirement deleted an operation whose held receipt was missing.")
		}
		storetest.Exec(t, f.observer, "DELETE FROM folder_operations WHERE id = $1", op.id)
		return err.Error()
	}
	want := missingReceipt(first)
	second := startRetireOperation(t, f, "other-missing-receipt-fixture")
	f.settleFixtureReply(t, second.id, inspectOperationReply())
	second.stamp(t, f, clock.now.Add(-25*time.Hour))
	if got := missingReceipt(second); got != want {
		t.Fatal("The missing-held-receipt invariant did not use a fixed diagnostic.")
	}
}
