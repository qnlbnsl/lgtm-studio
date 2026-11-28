package linktest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

type projectRetryTrace struct {
	operation atomic.Pointer[string]
	read      chan struct{}
}

type projectRetryRead struct{}

func (tr *projectRetryTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if id := tr.operation.Load(); id != nil && strings.Contains(query.SQL, "SELECT state, reply, COALESCE(failure, '') FROM folder_operations WHERE id = $1") &&
		len(query.Args) == 1 && query.Args[0] == *id {
		return context.WithValue(ctx, projectRetryRead{}, id)
	}
	return ctx
}

func (tr *projectRetryTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, result pgx.TraceQueryEndData) {
	id, _ := ctx.Value(projectRetryRead{}).(*string)
	if id != nil && result.Err == nil && result.CommandTag.RowsAffected() == 1 && tr.operation.CompareAndSwap(id, nil) {
		close(tr.read)
	}
}

func testPairedProjectRestart(t *testing.T, retryWhileOpen bool) {
	root, _, _ := gitCheckout(t)
	starts := make(chan inspectedFolder, 4)
	reading, report := make(chan struct{}), make(chan struct{})
	releaseRead := sync.OnceFunc(func() { close(reading) })
	releaseReport := sync.OnceFunc(func() { close(report) })
	defer releaseRead()
	defer releaseReport()
	trace := &projectRetryTrace{read: make(chan struct{})}
	f := newProjectFixture(t, servertest.Options{ConfigurePostgres: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = trace
	}}, relayControls{holdReport: func(observed reportDelivery) <-chan struct{} {
		if observed.folder.number != 0 {
			return report
		}
		return nil
	}})
	f.opts.OnFolderInspectionStart = func(ctx context.Context, operation, command string) {
		starts <- inspectedFolder{operation, command}
		select {
		case <-reading:
		case <-ctx.Done():
		}
	}
	run, leg := f.startApproved(t, 0)
	callCtx, cancelCall := context.WithCancel(t.Context())
	defer cancelCall()
	call := make(chan addResult, 1)
	go func() { call <- addFolder(callCtx, f.projects, f.id, "restart", root) }()
	go func() {
		select {
		case <-leg.done:
			cancelCall()
		case <-callCtx.Done():
		}
	}()
	op := waitProjectOperation(t, f.db, "restart")
	first := waitForCommand(t, leg, folderAction, 0)
	requireProjectCommand(t, op, first)
	if receive(t, starts, "the original folder inspection to reach its barrier") != (inspectedFolder{op.id, op.command}) {
		t.Fatal("The held inspection did not match the accepted operation.")
	}
	f.server.Restart(t)
	// The original caller disconnects with the stopped server.
	if result := finishAdd(t, call); result.err == nil {
		t.Fatal("The original caller returned a reply before the report was applied.")
	}
	releaseRead()
	after := waitProjectOperation(t, f.db, "restart")
	requireOpenProject(t, after)
	if after.id != op.id || after.command != op.command {
		t.Fatal("Restart changed the accepted operation or its Link command.")
	}
	requireProjectRows(t, f.db, 0)
	var newLeg *relayLeg
	waitFor(t, 15*time.Second, "a welcomed reconnect after server restart", func() bool {
		if newLeg == nil {
			select {
			case newLeg = <-f.relay.legs:
			default:
				return false
			}
		}
		if newLeg.observe().welcomed {
			return true
		}
		select {
		case <-newLeg.done:
			newLeg = nil
		default:
		}
		return false
	})
	if newLeg.observe().hello != leg.observe().hello {
		t.Fatal("The client did not reconnect with the original run pair after server restart.")
	}
	held := waitForPairedFolderReport(t, newLeg, op)
	var retry <-chan addResult
	if retryWhileOpen {
		trace.operation.Store(&op.id)
		retry = beginAdd(t, f.projects, f.id, "restart", root)
		receive(t, trace.read, "the keyed retry to finish reading the still-open operation")
		requireOpenProject(t, readProjectOperation(t, f.db, "restart"))
		if mark(t, f.db, f.id) >= held.number {
			t.Fatal("The held report committed ahead of the retry's read.")
		}
	}
	releaseReport()
	waitForAck(t, newLeg, held.number)
	if !retryWhileOpen {
		if settled := readProjectOperation(t, f.db, "restart"); settled.state != "succeeded" || settled.id != op.id || settled.command != op.command {
			t.Fatal("The acknowledged report did not settle the original operation before its first retry.")
		}
		requireProjectSaveCount(t, f.db, 1)
		retry = beginAdd(t, f.projects, f.id, "restart", root)
	}
	reply := requireAddReply(t, finishAdd(t, retry), f.id, root)
	var reportNumber uint64
	for _, observed := range newLeg.observe().reports {
		if observed.folder.operationID == op.id && observed.folder.commandID == op.command {
			if reportNumber != 0 {
				t.Fatal("The reconnected client sent the original report more than once.")
			}
			reportNumber = observed.folder.number
		}
	}
	if reportNumber == 0 {
		t.Fatal("The restarted server did not receive the original inspection's report on the reconnected stream.")
	}
	replayed := requireAddReply(t, addFolder(t.Context(), f.projects, f.id, "restart", root), f.id, root)
	final := readProjectOperation(t, f.db, "restart")
	if !proto.Equal(reply, replayed) || final.id != op.id || final.command != op.command ||
		final.state != "succeeded" || len(starts) != 0 ||
		scalar[int](t, f.db, "SELECT count(*) FROM link_commands WHERE machine_id=$1 AND action='look_at_folder'", f.id) != 1 {
		t.Fatal("The accepted retry did not preserve one operation, command, response and inspection.")
	}
	requireProjectRows(t, f.db, 1)
	requireProjectSaveCount(t, f.db, 1)
	run.stop(t)
}

func testPairedProjectInterruption(t *testing.T) {
	root, _, _ := gitCheckout(t)
	reportAppended := make(chan uint64, 1)
	interrupt := make(chan struct{})
	stopAtAppend := sync.OnceFunc(func() { close(interrupt) })
	defer stopAtAppend()
	var starts atomic.Int32
	var drop atomic.Bool
	var interruptedCommand atomic.Pointer[string]
	serverLogs := new(diagnostics)
	f := newProjectFixture(t, servertest.Options{Log: serverLogs}, relayControls{dropResult: func(result noticeResult) bool {
		target := interruptedCommand.Load()
		return target != nil && result.id == *target &&
			result.outcome == linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED && drop.CompareAndSwap(true, false)
	}})
	f.opts.OnFolderInspectionStart = func(context.Context, string, string) { starts.Add(1) }
	f.opts.AfterFolderReport = func(ctx context.Context, number uint64) error {
		reportAppended <- number
		select {
		case <-interrupt:
			return errors.New("The paired client stopped after the durable append.")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	f.probe.arm(t)
	approve(t, f.page, f.id, "Interruption fixture")
	first := startClientExpecting(t, f.opts, 1)
	leg := receive(t, f.relay.legs, "the original stream with a held startup scan")
	state := waitForCommand(t, leg, stateAction, 0)
	waitForStoredResult(t, f.db, state, "done")
	f.probe.heldPID(t)
	call := beginAdd(t, f.projects, f.id, "after-append", root)
	op := waitProjectOperation(t, f.db, "after-append")
	command := waitForCommand(t, leg, folderAction, 0)
	requireProjectCommand(t, op, command)
	number := receive(t, reportAppended, "the folder report to append durably")
	f.probe.release(t, "1.0.0")
	logs := f.opts.Stderr.(*diagnostics)
	waitFor(t, 5*time.Second, "startup inventory to append after the held folder report", func() bool {
		return slices.Contains(logs.bufferedAfter(number), number+1)
	})
	stopAtAppend()
	first.expectExit(t, 1)
	receive(t, leg.done, "the interrupted client's stream to close")
	if slices.Contains(leg.observe().events, number) || slices.ContainsFunc(leg.observe().results, func(result noticeResult) bool {
		return result.id == op.command
	}) {
		t.Fatal("The interrupted process handed off the report or a result.")
	}
	requireOpenProject(t, readProjectOperation(t, f.db, "after-append"))
	watermark := interruptedProjectWatermark(t, f, op, number)
	interruptedCommand.Store(&op.command)

	f.opts.AfterFolderReport = nil
	var replyBytes []byte
	for _, version := range []string{"2.0.0", "3.0.0"} {
		drop.Store(version == "2.0.0")
		f.probe.setVersion(t, version)
		run := startClient(t, f.opts)
		recoveryLeg := receive(t, f.relay.legs, "the recovering client after its append interruption")
		delivery := waitForCommand(t, recoveryLeg, folderAction, 0)
		requireProjectCommand(t, op, delivery)
		if delivery.attempt != command.attempt+1 {
			t.Fatalf("The recovery attempt was %d, want %d.", delivery.attempt, command.attempt+1)
		}
		interrupted := waitForResult(t, recoveryLeg, delivery, linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED)
		if interrupted.lastEventNumber != watermark {
			t.Fatalf("Recovered result used watermark %d, want %d.", interrupted.lastEventNumber, watermark)
		}
		waitFor(t, 3*time.Second, "the replayed result to pass the drop decision", func() bool { return !drop.Load() })
		newMark := drainPairedInventory(t, &f.commandFixture, recoveryLeg, version)
		if newMark <= watermark {
			t.Fatal("The restarted client did not append fresh inventory beyond the original watermark.")
		}
		final := readProjectOperation(t, f.db, "after-append")
		if final.id != op.id || final.command != op.command || final.state != "succeeded" {
			t.Fatal("The replayed folder report did not win over the interrupted result.")
		}
		if version == "2.0.0" {
			replyBytes = final.reply
			requirePending(t, f.db, delivery)
		} else {
			waitForStoredResult(t, f.db, delivery, "interrupted")
			final = readProjectOperation(t, f.db, "after-append")
			if final.state != "succeeded" || !bytes.Equal(final.reply, replyBytes) {
				t.Fatal("Forwarding the second interrupted result changed the stored success.")
			}
		}
		run.stop(t)
		requireAcknowledgedOutbox(t, f.opts, newMark)
		command = delivery
	}
	reply := requireAddReply(t, finishAdd(t, call), f.id, root)
	if retry := requireAddReply(t, addFolder(t.Context(), f.projects, f.id, "after-append", root), f.id, root); !proto.Equal(retry, reply) {
		t.Fatal("The accepted Add and its retry returned different saved replies.")
	}
	serverLogs.mu.Lock()
	applied := 0
	for _, entry := range serverLogs.entries {
		if entry.Message == "link_event_applied" && entry.Sequence == number {
			applied++
		}
	}
	serverLogs.mu.Unlock()
	if starts.Load() != 1 || applied != 1 {
		t.Fatalf("The operation started %d inspections and applied its report %d times, want one of each.", starts.Load(), applied)
	}
	requireProjectRows(t, f.db, 1)
	requireProjectSaveCount(t, f.db, 1)
}

func interruptedProjectWatermark(t *testing.T, f *projectFixture, op projectOperation, number uint64) uint64 {
	t.Helper()
	var watermark uint64
	withClosedClientDB(t, f.opts, false, func(db *bolt.DB) error {
		return db.View(func(tx *bolt.Tx) error {
			saved, err := commandReceipt(tx, op.command)
			if err != nil || saved.Version != 2 || saved.State != "started" || saved.LastEventNumber != nil || len(saved.Result) != 0 {
				t.Error("The interrupted run did not leave an unsealed started receipt.")
				return errors.New("incorrect started receipt")
			}
			next := tx.Bucket([]byte("meta")).Get([]byte("next"))
			if len(next) != 8 || binary.BigEndian.Uint64(next) != number+2 {
				t.Error("The closed store did not place inventory immediately after the folder report.")
				return errors.New("incorrect next event number")
			}
			watermark = binary.BigEndian.Uint64(next) - 1
			events := tx.Bucket([]byte("events"))
			var folder, inventory linkv1.Event
			read := func(sequence uint64, event *linkv1.Event) bool {
				payload := events.Get(binary.BigEndian.AppendUint64(nil, sequence))
				return len(payload) > 1 && payload[0] == 1 && proto.Unmarshal(payload[1:], event) == nil
			}
			if !read(number, &folder) || folder.GetFolderSeen().GetCommandId() != op.command ||
				folder.GetFolderSeen().GetOperationId() != op.id || !read(watermark, &inventory) ||
				!slices.ContainsFunc(inventory.GetHarnessesFound().GetHarnesses(), func(item *linkv1.Harness) bool {
					return item.GetName() == "claude" && item.GetVersion() == "1.0.0"
				}) {
				t.Error("The durable folder report or subsequent inventory bytes were missing.")
				return errors.New("incorrect retained events")
			}
			return nil
		})
	})
	return watermark
}
