package linktest

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

type projectAcceptanceTrace struct {
	armed   atomic.Bool
	first   chan uint32
	release chan struct{}
}

type projectAcceptanceQuery struct{}

func (tr *projectAcceptanceTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.Contains(query.SQL, "INSERT INTO folder_operations") && tr.armed.CompareAndSwap(true, false) {
		return context.WithValue(ctx, projectAcceptanceQuery{}, true)
	}
	return ctx
}

func (tr *projectAcceptanceTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, result pgx.TraceQueryEndData) {
	if result.Err == nil && ctx.Value(projectAcceptanceQuery{}) == true {
		tr.first <- conn.PgConn().PID()
		select {
		case <-tr.release:
		case <-ctx.Done():
		}
	}
}

func testPairedProjectCommitOrder(t *testing.T) {
	root, subfolder, _ := gitCheckout(t)
	trace := &projectAcceptanceTrace{first: make(chan uint32, 1), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(trace.release) })
	defer release()
	f := newProjectFixture(t, servertest.Options{ConfigurePostgres: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = trace
	}}, relayControls{})
	_, leg := f.startApproved(t, 0)
	trace.armed.Store(true)

	first := beginAdd(t, f.projects, f.id, "first-write", root)
	pid := receive(t, trace.first, "the first accepted write to hold the machine row")
	second := beginAdd(t, f.projects, f.id, "second-write", subfolder)
	waitFor(t, 5*time.Second, "the second Projects write to block on the first write's machine row", func() bool {
		return scalar[bool](t, f.db, `
SELECT EXISTS (SELECT FROM pg_stat_activity
  WHERE $1::int=ANY(pg_blocking_pids(pid)) AND query LIKE '%FROM machines WHERE id%FOR NO KEY UPDATE')`, int32(pid))
	})
	if readProjectOperation(t, f.db, "first-write").id != "" || readProjectOperation(t, f.db, "second-write").id != "" {
		t.Fatal("The blocked acceptance exposed a partial operation before commit.")
	}
	release()
	firstOp := waitProjectOperation(t, f.db, "first-write")
	secondOp := waitProjectOperation(t, f.db, "second-write")
	firstReply := requireAddReply(t, finishAdd(t, first), f.id, root)
	secondReply := requireAddReply(t, finishAdd(t, second), f.id, root)
	requireFolderCommandOrder(t, f.db, leg, firstOp, secondOp)
	if !proto.Equal(firstReply, secondReply) {
		t.Fatal("The two ordered calls did not settle onto the same saved checkout.")
	}
	requireProjectSaveCount(t, f.db, 1)
}

type projectClaimTrace struct {
	machine atomic.Pointer[string]
	watch   atomic.Bool
	cursors chan int64
}

func (tr *projectClaimTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if machine := tr.machine.Load(); tr.watch.Load() && machine != nil &&
		strings.Contains(query.SQL, "staged_outcome IS NULL AND seq > $2") && query.Args[0] == *machine {
		select {
		case tr.cursors <- query.Args[1].(int64):
		default:
		}
	}
	return ctx
}

func (*projectClaimTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func testPairedProjectRunGate(t *testing.T, retained bool) {
	root, subfolder, _ := gitCheckout(t)
	var activeReplay atomic.Uint64
	holders := make(chan pairedSendHold, 3)
	releaseAll := make(chan struct{})
	defer close(releaseAll)
	replayReached, releaseReplay := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseReplay) })
	defer release()
	trace := &projectClaimTrace{cursors: make(chan int64, 32)}
	f := newProjectFixture(t, servertest.Options{
		ConfigurePostgres: func(cfg *pgxpool.Config) { cfg.ConnConfig.Tracer = trace },
		LinkSeams: servertest.LinkSeams{
			BeforeCommandSend: func(ctx context.Context, machineID, _ string) error {
				if target := trace.machine.Load(); target == nil || *target == machineID {
					return nil
				}
				hold := pairedSendHold{machineID: machineID, release: make(chan struct{})}
				select {
				case holders <- hold:
				case <-ctx.Done():
					return ctx.Err()
				}
				select {
				case <-hold.release:
				case <-releaseAll:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			},
			BeforeEventCommit: func(ctx context.Context, machineID string, number uint64) error {
				if target := trace.machine.Load(); target != nil && *target == machineID && activeReplay.CompareAndSwap(number, 0) {
					close(replayReached)
					select {
					case <-releaseReplay:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return nil
			},
		}}, relayControls{})
	trace.machine.Store(&f.id)
	incumbent, oldLeg := f.startApproved(t, 0)
	baseline := drainPairedInventory(t, &f.commandFixture, oldLeg, "1.0.0")

	var firstHold pairedSendHold
	for i := range 3 {
		opts := newClient(t, new(diagnostics))
		id := join(t, f.page, opts, f.relay.address)
		approve(t, f.page, id, fmt.Sprintf("Projects permit holder %d", i+1))
		startClient(t, opts)
		receive(t, f.relay.legs, "the permit holder's real client stream")
		hold := receive(t, holders, "a holder inside a server send permit")
		if hold.machineID != id {
			t.Fatal("The send permit belongs to a different machine.")
		}
		if i == 0 {
			firstHold = hold
		}
	}

	first := beginAdd(t, f.projects, f.id, "first-gated", root)
	firstOp := waitProjectOperation(t, f.db, "first-gated")
	second := beginAdd(t, f.projects, f.id, "second-gated", subfolder)
	secondOp := waitProjectOperation(t, f.db, "second-gated")
	requireGated := func() {
		t.Helper()
		if !scalar[bool](t, f.db, `
SELECT count(*)=2 AND bool_and(sent_stream_id IS NULL AND attempt=1 AND NOT dispatched AND outcome IS NULL)
FROM link_commands WHERE id IN ($1,$2)`, firstOp.command, secondOp.command) || len(trace.cursors) != 0 {
			t.Fatal("Projects work acquired a claim or consulted the send cursor before both gates opened.")
		}
	}
	requireGated()
	incumbent.stop(t)
	requireAcknowledgedOutbox(t, f.opts, baseline)
	if retained {
		appendPairedProjectPrefix(t, f.opts, baseline, "first retained project run", "second retained project run")
		activeReplay.Store(baseline + 2)
	}

	trace.watch.Store(true)
	replacement := startClient(t, f.opts)
	leg := receive(t, f.relay.legs, "the replacement stream while command permits are occupied")
	welcome := waitForWelcome(t, leg)
	firstNumber := baseline + 1
	if retained {
		firstNumber += 2
		receive(t, replayReached, "the final retained report to reach its commit seam")
		waitForAck(t, leg, baseline+1)
		if mark(t, f.db, f.id) != baseline+1 {
			t.Fatal("The first retained event did not commit while the second was held.")
		}
		requireGated()
		release()
		waitForAck(t, leg, baseline+2)
	}
	if welcome.hello.first != firstNumber || welcome.hello.runID == oldLeg.observe().hello.runID || welcome.mark != baseline {
		t.Fatal("The replacement's Hello and Welcome do not describe the selected old prefix.")
	}
	requireGated()
	close(firstHold.release)
	left := requireAddReply(t, finishAdd(t, first), f.id, root)
	right := requireAddReply(t, finishAdd(t, second), f.id, root)
	requireFolderCommandOrder(t, f.db, leg, firstOp, secondOp)
	if cursor := receive(t, trace.cursors, "the first real cursor read after the gates opened"); cursor != 0 {
		t.Fatalf("The replacement began with send cursor %d, want zero.", cursor)
	}
	if retained {
		sends := leg.observe().sends
		firstAck := slices.Index(sends, frameSend{kind: "ack", number: baseline + 1})
		finalAck := slices.Index(sends, frameSend{kind: "ack", number: baseline + 2})
		firstCommand := slices.IndexFunc(sends, func(send frameSend) bool { return send.kind == "command" && send.commandID == firstOp.command })
		if !(firstAck >= 0 && firstAck < finalAck && finalAck < firstCommand) {
			t.Fatal("Both retained acknowledgements must cross the replacement stream before either Projects command.")
		}
	}
	if !proto.Equal(left, right) {
		t.Fatal("The gated calls did not settle onto the same saved identities.")
	}
	requireProjectSaveCount(t, f.db, 1)
	replacement.stop(t)
	if retained {
		requireAcknowledgedOutbox(t, f.opts, baseline+2)
	}
}

func testPairedProjectRunGateWithFreePermit(t *testing.T) {
	root, _, _ := gitCheckout(t)
	var finalNumber atomic.Uint64
	startReplay, held, releaseReplay := make(chan struct{}), make(chan struct{}), make(chan struct{})
	allowStart := sync.OnceFunc(func() { close(startReplay) })
	allowFinal := sync.OnceFunc(func() { close(releaseReplay) })
	recordHold := sync.OnceFunc(func() { close(held) })
	defer allowStart()
	defer allowFinal()
	trace := &projectClaimTrace{cursors: make(chan int64, 32)}
	f := newProjectFixture(t, servertest.Options{ConfigurePostgres: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = trace
	}}, relayControls{
		releaseReports: func() <-chan struct{} {
			if finalNumber.Load() != 0 {
				return startReplay
			}
			return nil
		},
		holdEvent: func(number uint64) <-chan struct{} {
			if number == finalNumber.Load() {
				recordHold()
				return releaseReplay
			}
			return nil
		},
	})
	trace.machine.Store(&f.id)
	incumbent, oldLeg := f.startApproved(t, 0)
	baseline := drainPairedInventory(t, &f.commandFixture, oldLeg, "1.0.0")
	incumbent.stop(t)
	receive(t, oldLeg.done, "the incumbent stream to close")
	requireAcknowledgedOutbox(t, f.opts, baseline)
	appendPairedProjectPrefix(t, f.opts, baseline, "first retained fact", "second retained fact", "last retained fact")
	final := baseline + 3
	finalNumber.Store(final)
	trace.watch.Store(true)
	replacement := startClient(t, f.opts)
	leg := receive(t, f.relay.legs, "the replacement stream with free send permits")
	welcome := waitForWelcome(t, leg)
	if welcome.hello.runID == oldLeg.observe().hello.runID || welcome.hello.first != final+1 || welcome.mark != baseline {
		t.Fatal("The replacement did not announce the retained prefix on a new run.")
	}
	// The server handles heartbeats after it publishes the welcomed stream.
	waitFor(t, 15*time.Second, "a heartbeat acknowledgement on the replacement stream", func() bool {
		return leg.observe().heartbeatAcks > 0
	})
	call := beginAdd(t, f.projects, f.id, "gated-with-permit", root)
	op := waitProjectOperation(t, f.db, "gated-with-permit")
	allowStart()
	receive(t, held, "the relay to hold the last retained event")
	// The second acknowledgement proves the writer finished the turn after
	// the first event with the Projects command already queued.
	waitForAck(t, leg, final-1)
	before := leg.observe()
	if mark(t, f.db, f.id) != final-1 || slices.Contains(before.sends, frameSend{kind: "event", number: final}) {
		t.Fatal("The final retained event reached the server before release.")
	}
	unclaimed := scalar[bool](t, f.db, `
SELECT sent_stream_id IS NULL AND attempt=1 AND NOT dispatched AND outcome IS NULL
FROM link_commands WHERE id=$1`, op.command)
	frames := slices.ContainsFunc(before.sends, func(send frameSend) bool { return send.kind == "command" })
	if !unclaimed || len(trace.cursors) != 0 || frames {
		t.Fatalf("Before the last retained event, the command was unclaimed=%t, with %d cursor reads and command frames=%t.",
			unclaimed, len(trace.cursors), frames)
	}
	allowFinal()
	waitForAck(t, leg, final)
	command := waitForCommand(t, leg, folderAction, 0)
	requireProjectCommand(t, op, command)
	if cursor := receive(t, trace.cursors, "the first cursor read after replay"); cursor != 0 {
		t.Fatalf("The replacement began with send cursor %d, want zero.", cursor)
	}
	sends := leg.observe().sends
	ack := slices.Index(sends, frameSend{kind: "ack", number: final})
	offer := slices.Index(sends, frameSend{kind: "command", commandID: command.id, attempt: command.attempt})
	if ack < 0 || offer <= ack {
		t.Fatal("The Projects command did not follow the final replay acknowledgement.")
	}
	requireAddReply(t, finishAdd(t, call), f.id, root)
	requireProjectSaveCount(t, f.db, 1)
	replacement.stop(t)
}

func appendPairedProjectPrefix(t *testing.T, opts clienttest.Options, baseline uint64, facts ...string) {
	t.Helper()
	payloads := make([][]byte, len(facts))
	for i, name := range facts {
		payloads[i] = retainedPairedEvent(t, pairedFacts(0, name))
	}
	withClosedClientDB(t, opts, true, func(db *bolt.DB) error {
		return db.Update(func(tx *bolt.Tx) error {
			meta, events := tx.Bucket([]byte("meta")), tx.Bucket([]byte("events"))
			next := meta.Get([]byte("next"))
			if len(next) != 8 || binary.BigEndian.Uint64(next) != baseline+1 {
				return errors.New("the acknowledged incumbent history does not match the replay fixture")
			}
			for i, payload := range payloads {
				if err := events.Put(binary.BigEndian.AppendUint64(nil, baseline+1+uint64(i)), payload); err != nil {
					return err
				}
			}
			return meta.Put([]byte("next"), binary.BigEndian.AppendUint64(nil, baseline+1+uint64(len(payloads))))
		})
	})
}

func requireFolderCommandOrder(t *testing.T, db *pgx.Conn, leg *relayLeg, first, second projectOperation) {
	t.Helper()
	requireProjectCommand(t, first, waitForCommand(t, leg, folderAction, 0))
	requireProjectCommand(t, second, waitForCommand(t, leg, folderAction, 1))
	if !scalar[bool](t, db, `
SELECT a.seq<b.seq FROM link_commands a, link_commands b WHERE a.id=$1 AND b.id=$2`, first.command, second.command) {
		t.Fatal("The delivered identifiers did not follow the durable command sequence.")
	}
}

func testPairedEqualProjectReadiness(t *testing.T) {
	root, _, _ := gitCheckout(t)
	f := newProjectFixture(t, servertest.Options{}, relayControls{})
	run, leg := f.startApproved(t, 0)
	baseline := drainPairedInventory(t, &f.commandFixture, leg, "1.0.0")
	run.stop(t)
	script := pairedScript{server: f.server, opts: f.opts, id: f.id, db: f.db}
	oldID, currentID := newPairedID(), newPairedID()
	old := script.attempt(t, oldID, baseline+1)
	requirePairedWelcome(t, old, baseline)
	call := beginAdd(t, f.projects, f.id, "equal-ready", root)
	op := waitProjectOperation(t, f.db, "equal-ready")
	readPairedFolderCommand(t, old, op)
	current := script.attempt(t, currentID, baseline+1)
	requirePairedWelcome(t, current, baseline)
	readPairedFolderCommand(t, current, op)
	generation := scalar[uint64](t, f.db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", f.id)
	requirePairedEnd(t, script.attempt(t, oldID, baseline+1), connect.CodeInvalidArgument, "")
	script.requireRun(t, currentID, currentID, baseline)
	if scalar[uint64](t, f.db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", f.id) != generation ||
		scalar[int](t, f.db, "SELECT count(*) FROM folder_operations WHERE machine_id=$1", f.id) != 1 {
		t.Fatal("The refused ready pair changed the generation or duplicated Projects work.")
	}
	after := readProjectOperation(t, f.db, "equal-ready")
	requireOpenProject(t, after)
	if after.id != op.id || after.command != op.command {
		t.Fatal("The refused ready pair changed existing Projects work.")
	}
	requireProjectRows(t, f.db, 0)
	requireProjectSaveCount(t, f.db, 0)
	sendPairedFrame(t, current, pairedEvent(projectFolderFacts(baseline+1, op, root)))
	requireProjectAck(t, current, baseline+1)
	requireAddReply(t, finishAdd(t, call), f.id, root)
	requireProjectSaveCount(t, f.db, 1)
}
