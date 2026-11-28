package linktest

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func TestSprintFourFolderSettlement(t *testing.T) {
	requirePostgres(t)
	t.Run("R3", func(t *testing.T) { t.Run("SimultaneousAliases", testPairedAliases) })
	t.Run("R7", func(t *testing.T) { t.Run("DisconnectBeforeDelivery", testPairedUndeliveredFolder) })
	t.Run("R10", func(t *testing.T) {
		t.Run("StagedAcrossRestart", func(t *testing.T) { testPairedStagedFolder(t, false) })
		t.Run("RemoveBeforeSettlement", func(t *testing.T) { testPairedStagedFolder(t, true) })
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
	first := receive(t, tripped, "the relay to cut the first folder command before delivery")
	requireProjectCommand(t, op, first)
	f.relay.blockNew.Store(true)
	receive(t, leg.done, "the cut leg to close")
	run.stop(t)
	if slices.ContainsFunc(leg.observe().commands, func(cmd commandDelivery) bool { return cmd.id == op.command }) {
		t.Fatal("The relay delivered the folder command before the cut.")
	}
	requireAddRefusal(t, finishAdd(t, call), connect.CodeDeadlineExceeded, "The machine did not answer the folder request.")
	if elapsed := time.Since(started); elapsed < 25*time.Second || elapsed >= 35*time.Second || !time.Now().After(first.expiry) {
		t.Fatalf("The server answered outside its 25-second wait, before expiry, or at the client deadline: %v.", elapsed)
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

func testPairedStagedFolder(t *testing.T, removeBefore bool) {
	root, _, _ := gitCheckout(t)
	held := make(chan struct{})
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
		if number := report.folder.number; number != 0 && reportNumber.CompareAndSwap(0, number) {
			return held
		}
		return nil
	}})
	f.opts.OnFolderInspectionStart = func(context.Context, string, string) { inspected.Add(1) }
	run, leg := f.startApproved(t, 0)
	call := beginAdd(t, f.projects, f.id, "staged-report", root)
	op := waitProjectOperation(t, f.db, "staged-report")
	command := waitForCommand(t, leg, folderAction, 0)
	requireProjectCommand(t, op, command)
	result := waitForResult(t, leg, command, commandDone)
	number := reportNumber.Load()
	if number == 0 || result.lastEventNumber != number {
		t.Fatal("The folder result did not carry the held report watermark.")
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
	run.stop(t)
	receive(t, leg.done, "the old staged leg to close")
	if removeBefore {
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
	baseline := mark(t, f.db, f.id)
	if baseline != number-1 {
		t.Fatalf("The held report changed the committed mark to %d.", baseline)
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
