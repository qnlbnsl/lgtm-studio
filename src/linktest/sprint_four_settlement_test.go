package linktest

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func TestSprintFourFolderSettlement(t *testing.T) {
	requirePostgres(t)
	t.Run("R3", func(t *testing.T) { t.Run("SimultaneousAliases", testPairedAliases) })
	t.Run("R7", func(t *testing.T) { t.Run("DisconnectBeforeDelivery", testPairedUndeliveredFolder) })
	t.Run("R10", func(t *testing.T) {
		t.Run("StagedAcrossRestart", func(t *testing.T) { testPairedStagedFolder(t, stagedRestart) })
		t.Run("RemoveBeforeSettlement", func(t *testing.T) { testPairedStagedFolder(t, stagedRemoval) })
		t.Run("StagedOnSameRunReconnect", func(t *testing.T) { testPairedStagedFolder(t, stagedReconnect) })
	})
}

type inspectedFolder struct{ operationID, commandID string }

func testPairedAliases(t *testing.T) {
	root, subfolder, alias := gitCheckout(t)
	hold := make(chan struct{})
	starts := make(chan inspectedFolder, 4)
	f := newProjectFixture(t, servertest.Options{}, relayControls{holdReport: func(report reportDelivery) <-chan struct{} {
		if report.folder.number != 0 {
			return hold
		}
		return nil
	}})
	f.opts.OnFolderInspectionStart = func(_ context.Context, operation, command string) {
		starts <- inspectedFolder{operation, command}
	}
	_, leg := f.startApproved(t, 0)
	first := beginAdd(t, f.projects, f.id, "alias-link", alias)
	second := beginAdd(t, f.projects, f.id, "alias-subfolder", subfolder)
	firstOp := waitProjectOperation(t, f.db, "alias-link")
	secondOp := waitProjectOperation(t, f.db, "alias-subfolder")
	requireOpenProject(t, firstOp)
	requireOpenProject(t, secondOp)
	if firstOp.id == secondOp.id || firstOp.command == secondOp.command {
		t.Fatal("The overlapping Add calls did not accept independent operations.")
	}
	seen := []inspectedFolder{
		receive(t, starts, "the first real alias inspection"),
		receive(t, starts, "the second real alias inspection"),
	}
	for _, op := range []projectOperation{firstOp, secondOp} {
		if !slices.Contains(seen, inspectedFolder{op.id, op.command}) {
			t.Fatal("The accepted alias operation did not start a matching inspection.")
		}
	}
	close(hold)
	left := requireAddReply(t, finishAdd(t, first), f.id, root)
	right := requireAddReply(t, finishAdd(t, second), f.id, root)
	if !proto.Equal(left, right) || len(starts) != 0 {
		t.Fatal("The two aliases did not return the same saved repository and folder after one read per call.")
	}
	for _, accepted := range []projectOperation{firstOp, secondOp} {
		if !slices.ContainsFunc(leg.observe().reports, func(report reportDelivery) bool {
			return report.folder.operationID == accepted.id && report.folder.commandID == accepted.command
		}) {
			t.Fatal("An alias report did not match its accepted operation and command.")
		}
	}
	requireProjectRows(t, f.db, 1)
	requireProjectSaveCount(t, f.db, 1)
}

func testPairedUndeliveredFolder(t *testing.T) {
	root, _, _ := gitCheckout(t)
	var inspected atomic.Int32
	tripped := make(chan commandDelivery, 1)
	var cut atomic.Bool
	f := newProjectFixture(t, servertest.Options{}, relayControls{beforeReply: func(_ context.Context, frame *linkv1.OpenResponse) bool {
		cmd := frame.GetCommand()
		if cmd.GetLookAtFolder() != nil && cut.CompareAndSwap(false, true) {
			tripped <- commandDelivery{id: cmd.GetId(), operationID: cmd.GetLookAtFolder().GetOperationId(),
				attempt: cmd.GetAttempt(), expiry: cmd.GetExpiresAt().AsTime()}
			return false
		}
		return true
	}})
	f.opts.OnFolderInspectionStart = func(context.Context, string, string) { inspected.Add(1) }
	run, leg := f.startApproved(t, 0)
	started := time.Now()
	call := beginAdd(t, f.projects, f.id, "undelivered", root)
	op := waitProjectOperation(t, f.db, "undelivered")
	accepted := time.Now()
	first := receive(t, tripped, "the relay to cut the first folder command before delivery")
	requireProjectCommand(t, op, first)
	f.relay.blockNew.Store(true)
	receive(t, leg.done, "the cut leg to close")
	run.stop(t)
	if slices.ContainsFunc(leg.observe().commands, func(cmd commandDelivery) bool { return cmd.id == op.command }) {
		t.Fatal("The relay delivered the folder command before the cut.")
	}
	requireAddRefusal(t, finishAdd(t, call), connect.CodeDeadlineExceeded, "The machine did not answer the folder request.")
	answered := time.Now()
	if answered.Sub(started) < 25*time.Second || answered.Sub(accepted) >= 28*time.Second || !answered.After(first.expiry) {
		t.Fatalf("The server answered %v after the call began and %v after observed acceptance, want a 25-second wait and an expired command.",
			answered.Sub(started), answered.Sub(accepted))
	}
	requireOpenProject(t, readProjectOperation(t, f.db, "undelivered"))
	f.relay.blockNew.Store(false)
	run = startClient(t, f.opts)
	retryLeg := receive(t, f.relay.legs, "the reconnected client after folder expiry")
	retry := waitForCommand(t, retryLeg, folderAction, 0)
	requireProjectCommand(t, op, retry)
	if !retry.expiry.Equal(first.expiry) || retry.attempt < first.attempt {
		t.Fatal("Late delivery did not preserve the accepted command's deadline and attempt order.")
	}
	waitForResult(t, retryLeg, retry, commandExpired)
	waitForStoredResult(t, f.db, retry, "expired")
	requireAddRefusal(t, addFolder(t.Context(), f.projects, f.id, "undelivered", root),
		connect.CodeDeadlineExceeded, "The machine did not take the folder command.")
	if got := readProjectOperation(t, f.db, "undelivered"); got.state != "failed" || inspected.Load() != 0 {
		t.Fatal("The expired folder command did not fail the operation with zero real inspections.")
	}
	requireProjectRows(t, f.db, 0)
	run.stop(t)
}

type projectSweepTrace struct {
	queried  atomic.Bool
	attached atomic.Bool
}

type projectSettlementQuery int

const (
	sweepFinishedQuery projectSettlementQuery = iota + 1
	saveFolderQuery
)

func (tr *projectSweepTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	switch {
	case strings.Contains(query.SQL, "SELECT finished_at, machine_id, seq"):
		// The sweep reaches terminal pruning after processing expired commands.
		return context.WithValue(ctx, projectSettlementQuery(0), sweepFinishedQuery)
	case strings.Contains(query.SQL, "INSERT INTO repository_folders"):
		return context.WithValue(ctx, projectSettlementQuery(0), saveFolderQuery)
	}
	return ctx
}

func (tr *projectSweepTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, result pgx.TraceQueryEndData) {
	if result.Err != nil {
		return
	}
	switch ctx.Value(projectSettlementQuery(0)) {
	case sweepFinishedQuery:
		tr.queried.Store(true)
	case saveFolderQuery:
		tr.attached.Store(true)
	}
}

type stagedFolderScenario int

const (
	stagedRestart stagedFolderScenario = iota
	stagedRemoval
	stagedReconnect
)

func testPairedStagedFolder(t *testing.T, scenario stagedFolderScenario) {
	root, _, _ := gitCheckout(t)
	held := make(chan struct{})
	releaseReport := sync.OnceFunc(func() { close(held) })
	defer releaseReport()
	var reportNumber atomic.Uint64
	var failReplay atomic.Bool
	var inspected atomic.Int32
	var sweep *projectSweepTrace
	faulted := make(chan uint64, 1)
	f := newProjectFixture(t, servertest.Options{
		ConfigurePostgres: func(config *pgxpool.Config) {
			if sweep != nil {
				config.ConnConfig.Tracer = sweep
			}
		},
		LinkSeams: servertest.LinkSeams{
			BeforeEventCommit: func(_ context.Context, _ string, number uint64) error {
				if number == reportNumber.Load() && failReplay.CompareAndSwap(true, false) {
					faulted <- number
					return errors.New("The paired folder report write was interrupted.")
				}
				return nil
			},
		},
	}, relayControls{holdReport: func(report reportDelivery) <-chan struct{} {
		if number := report.folder.number; number != 0 {
			if reportNumber.CompareAndSwap(0, number) || (scenario == stagedReconnect && reportNumber.Load() == number) {
				return held
			}
		}
		return nil
	}})
	f.opts.OnFolderInspectionStart = func(context.Context, string, string) { inspected.Add(1) }
	run, leg := f.startApproved(t, 0)
	baseline := drainPairedInventory(t, &f.commandFixture, leg, "1.0.0")
	call := beginAdd(t, f.projects, f.id, "staged-report", root)
	op := waitProjectOperation(t, f.db, "staged-report")
	command := waitForCommand(t, leg, folderAction, 0)
	requireProjectCommand(t, op, command)
	result := waitForResult(t, leg, command, commandDone)
	report := waitForPairedFolderReport(t, leg, op)
	number := result.lastEventNumber
	if number != report.number || report.number != baseline+1 {
		t.Fatalf("The result watermark is %d and the held report is %d after baseline %d.", number, report.number, baseline)
	}
	requireStaged := func() {
		t.Helper()
		if !scalar[bool](t, f.db, `
SELECT outcome IS NULL AND staged_outcome='done' AND staged_event_number=$2 AND attempt=$3
FROM link_commands WHERE id=$1`, op.command, number, command.attempt) {
			t.Fatal("The report's result did not remain durably staged.")
		}
		requireOpenProject(t, readProjectOperation(t, f.db, "staged-report"))
		requireProjectRows(t, f.db, 0)
		requireProjectSaveCount(t, f.db, 0)
	}
	waitFor(t, 5*time.Second, "the held report result to stage", func() bool {
		return scalar[bool](t, f.db, "SELECT staged_event_number IS NOT NULL FROM link_commands WHERE id=$1", op.command)
	})
	requireStaged()
	if scenario == stagedReconnect {
		assigned := scalar[string](t, f.db, "SELECT sent_stream_id FROM link_commands WHERE id=$1", op.command)
		leg.cut()
		receive(t, leg.done, "the staged command's original stream to close")
		reconnect := receive(t, f.relay.legs, "the same client process to reconnect")
		welcome := waitForWelcome(t, reconnect)
		if welcome.hello != leg.observe().hello || welcome.mark != baseline {
			t.Fatal("The same-process reconnect did not preserve its run pair and resume before the held report.")
		}
		if replay := waitForPairedFolderReport(t, reconnect, op); replay != report {
			t.Fatal("The reconnect did not replay the original staged report.")
		}
		if !scalar[bool](t, f.db, "SELECT expires_at > clock_timestamp()-interval '23 hours' FROM link_commands WHERE id=$1", op.command) {
			t.Fatal("The staged command fell outside the retry window before the writer turn.")
		}
		// Renaming queues a standing command while the reconnect's fresh reports are held.
		if _, err := f.page.RenameClient(t.Context(), connect.NewRequest(&webv1.RenameClientRequest{
			Id: f.id, Name: "Staged reconnect fixture",
		})); err != nil {
			t.Fatalf("Could not queue the independent standing command: %v.", err)
		}
		if standing := waitForCommand(t, reconnect, stateAction, 0); standing.revision != welcome.revision+1 {
			t.Fatal("The reconnect did not deliver the independent rename command.")
		}
		observed := reconnect.observe()
		if slices.ContainsFunc(observed.sends, func(send frameSend) bool { return send.kind == "command" && send.commandID == op.command }) ||
			!scalar[bool](t, f.db, `
SELECT COALESCE(sent_stream_id=$2 AND attempt=$3 AND dispatched,false) FROM link_commands WHERE id=$1`, op.command, assigned, command.attempt) {
			t.Fatal("The reconnect assigned or offered the staged folder command before its report was released.")
		}
		if mark(t, f.db, f.id) != baseline || slices.Contains(observed.sends, frameSend{kind: "event", number: number}) {
			t.Fatal("The staged folder report reached the server before release.")
		}
		requireStaged()
		releaseReport()
		waitForAck(t, reconnect, number)
		requireAddReply(t, finishAdd(t, call), f.id, root)
		waitForStoredResult(t, f.db, command, "done")
		if inspected.Load() != 1 {
			t.Fatal("The same process inspected the staged folder more than once.")
		}
		requireProjectRows(t, f.db, 1)
		requireProjectSaveCount(t, f.db, 1)
		run.stop(t)
		return
	}
	run.stop(t)
	receive(t, leg.done, "the old staged leg to close")
	if scenario == stagedRemoval {
		remove(t, f.page, f.id)
		requireAddRefusal(t, finishAdd(t, call), connect.CodeNotFound, "The machine was removed.")
		if !scalar[bool](t, f.db, `
SELECT outcome='removed' AND staged_event_number IS NULL FROM link_commands WHERE id=$1`, op.command) {
			t.Fatal("Removal did not settle the still-staged folder command.")
		}
		requireProjectRows(t, f.db, 0)
		requireProjectSaveCount(t, f.db, 0)
		return
	}
	if actual := mark(t, f.db, f.id); actual != baseline {
		t.Fatalf("The held report changed the committed mark to %d.", actual)
	}
	execute(t, f.db, "UPDATE link_commands SET expires_at=clock_timestamp()-interval '25 hours' WHERE id=$1", op.command)
	sweep = new(projectSweepTrace)
	f.server.Restart(t)
	f.page = f.server.Clients(t)
	waitFor(t, 8*time.Second, "the restarted command sweeper to finish checking expired commands", sweep.queried.Load)
	requireStaged()

	failReplay.Store(true)
	faultRun := startClient(t, f.opts)
	faultLeg := receive(t, f.relay.legs, "the replay stream with a commit fault")
	if receive(t, faulted, "the report to reach the pre-commit fault") != number {
		t.Fatal("The report fault intercepted a different event.")
	}
	f.relay.blockNew.Store(true)
	waitFor(t, 8*time.Second, "the rolled-back report stream to close", func() bool {
		if slices.Contains(faultLeg.observe().acks, number) {
			t.Fatal("The interrupted report received an acknowledgement.")
		}
		select {
		case <-faultLeg.done:
			return true
		default:
			return false
		}
	})
	faultRun.stop(t)
	if !sweep.attached.Load() || slices.Contains(faultLeg.observe().acks, number) || mark(t, f.db, f.id) != baseline {
		t.Fatal("The report did not reach its attachment write before the fault, or its mark or acknowledgement escaped rollback.")
	}
	requireStaged()

	f.relay.blockNew.Store(false)
	retry := beginAdd(t, f.projects, f.id, "staged-report", root)
	run = startClient(t, f.opts)
	recoveredLeg := receive(t, f.relay.legs, "the recovered staged stream")
	reply := requireAddReply(t, finishAdd(t, retry), f.id, root)
	waitForAck(t, recoveredLeg, number)
	waitForStoredResult(t, f.db, command, "done")
	waitForClientToProcessReplies(t, recoveredLeg)
	if inspected.Load() != 1 || slices.ContainsFunc(recoveredLeg.observe().commands, func(cmd commandDelivery) bool { return cmd.id == op.command }) {
		t.Fatal("A staged command was resent or caused another inspection.")
	}
	requireProjectRows(t, f.db, 1)
	requireProjectSaveCount(t, f.db, 1)

	run.stop(t)
	script := pairedScript{server: f.server, opts: f.opts, id: f.id, db: f.db}
	committed := mark(t, f.db, f.id)
	peer := script.attempt(t, newPairedID(), committed+1)
	requirePairedWelcome(t, peer, committed)
	sendPairedFrame(t, peer, pairedEvent(projectFolderFacts(committed+1, op, root+"-late")))
	requireProjectAck(t, peer, committed+1)
	requireProjectRows(t, f.db, 1)
	requireProjectSaveCount(t, f.db, 1)
	remove(t, f.page, f.id)
	after := requireAddReply(t, addFolder(t.Context(), f.projects, f.id, "staged-report", root), f.id, root)
	if !proto.Equal(reply, after) {
		t.Fatal("Removal or the eligible late report changed the settled reply.")
	}
	requireProjectRows(t, f.db, 0)
	requireProjectSaveCount(t, f.db, 1)
	if !scalar[bool](t, f.db, "SELECT count(*)=1 FROM feed WHERE topic='repositories' AND change='removed' AND key=$1", reply.GetRepository().GetId()) {
		t.Fatal("Removal of the settled checkout did not publish one repository removal.")
	}
}
