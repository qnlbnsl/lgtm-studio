package link

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
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestStagedResultsSettleInReplayOrder(t *testing.T) {
	var refuseReady atomic.Bool
	before := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := before.register(t, "ordered settlement after restart")
	before.approve(t, m.ID)
	seedAdmissionMark(t, before, m.ID, 5)
	runA, runB := fixtureStreamID(t), fixtureStreamID(t)
	incumbent := submitAdmission(t, before, steadyWire(t, before, streamTestOptions{}), m.ID, key, runA, 6)
	requireAdmissionWelcome(t, incumbent, 5)
	awaitAdmissionPublication(t, before, m.ID)
	oldStream := before.current(t, m.ID).StreamID
	first := seedSettlementCommand(t, before.presenceFixture, m.ID, oldStream, restartAction(), time.Minute)
	second := seedSettlementCommand(t, before.presenceFixture, m.ID, oldStream, restartAction(), time.Minute)
	earliest := seedSettlementCommand(t, before.presenceFixture, m.ID, oldStream, restartAction(), time.Minute)
	// Seed the same durable stages as the early result test, so replay can be checked independently.
	seedJoinedStage(t, before.presenceFixture, first.id, "interrupted", 9)
	seedJoinedStage(t, before.presenceFixture, second.id, "done", 9)
	seedJoinedStage(t, before.presenceFixture, earliest.id, "unsupported", 7)
	finishAdmissionPeer(t, incumbent)

	f := restartAdmissionFixture(t, before)
	f.svc.streaming.options.Commands = f.state
	newSettlementAudit(t, f.presenceFixture)
	eventSQL(t, f.presenceFixture, `ALTER TABLE fixture_settlements ADD COLUMN ordinal bigint GENERATED ALWAYS AS IDENTITY`)
	eventSQL(t, f.presenceFixture, `CREATE TABLE fixture_settlement_ready (due bigint NOT NULL)`)
	registerRestartSettlement(t, f.presenceFixture, func(ctx context.Context, tx *store.Tx, settlement Settlement) error {
		var ready string
		if err := tx.QueryRow(ctx, "SELECT ready_run_id FROM link_machines WHERE machine_id = $1", m.ID).Scan(&ready); err != nil {
			return err
		}
		if ready != runA {
			return store.NewDomainError("The replacement was ready before its replay settled the due command.")
		}
		if settlement.Command.GetId() == earliest.id {
			var payload []byte
			if err := tx.QueryRow(ctx, "SELECT facts FROM machines WHERE id = $1", m.ID).Scan(&payload); err != nil {
				return err
			}
			facts := new(linkv1.Facts)
			if proto.Unmarshal(payload, facts) != nil || facts.GetOs() != "Report for the early watermark" {
				return store.NewDomainError("The settlement hook could not see the report at its watermark.")
			}
		}
		if err := recordRestartSettlement(ctx, tx, settlement); err != nil {
			return err
		}
		return tx.Record(store.ChangeRef{Topic: "fixture_settlements", Key: settlement.Command.GetId(), Change: store.Saved})
	})
	if err := f.state.RegisterPart(PartHooks{RunChanged: func(ctx context.Context, tx *store.Tx, change RunChange) error {
		if change.RunID != runB || change.PreviousRunID != runA || change.FirstEventNumber != 10 {
			return store.NewDomainError("The readiness hook saw the wrong run transition.")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO fixture_settlement_ready
			SELECT count(*) FROM fixture_settlements WHERE event_mark = 9`); err != nil {
			return err
		}
		if refuseReady.Load() {
			return store.NewDomainError("The fixture refused readiness after the replay write.")
		}
		return nil
	}}); err != nil {
		t.Fatal("The replay fixture could not register its readiness hook.")
	}

	client := steadyWire(t, f, streamTestOptions{})
	peer := submitAdmission(t, f, client, m.ID, key, runB, 10)
	requireAdmissionWelcome(t, peer, 5)
	requireAdmissionRun(t, f, m.ID, runB, runA, 10)
	for _, event := range []*linkv1.Event{
		factsEvent(6, "First replay report"),
		factsEvent(7, "Report for the early watermark"),
		factsEvent(8, "Report before the final watermark"),
	} {
		sendHandshake(t, peer, eventFrame(event))
		requireEventAck(t, peer, event.GetNumber())
	}
	seven, nine := int64(7), int64(9)
	requireSettlementTerminal(t, f.presenceFixture, earliest.id, "unsupported")
	requireSettlementAudit(t, f.presenceFixture, earliest.id, OutcomeUnsupported, &seven, 7)
	if !hasSettlementStage(t, f.presenceFixture, first.id, "interrupted", 9) ||
		!hasSettlementStage(t, f.presenceFixture, second.id, "done", 9) {
		t.Error("Replay settled a result before reaching its own watermark.")
	}

	// The next event advances the mark even though it has no report to apply.
	last := &linkv1.Event{Number: 9}
	refuseReady.Store(true)
	head := f.head(t)
	sendHandshake(t, peer, eventFrame(last))
	requireOpenEnd(t, peer, connect.CodeUnavailable, openDatabase, "")
	if eventMark(t, f.presenceFixture, m.ID) != 8 ||
		!hasSettlementStage(t, f.presenceFixture, first.id, "interrupted", 9) ||
		!hasSettlementStage(t, f.presenceFixture, second.id, "done", 9) ||
		settlementAuditCount(t, f.presenceFixture, first.id) != 0 ||
		settlementAuditCount(t, f.presenceFixture, second.id) != 0 ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM fixture_settlement_ready") != 0 {
		t.Error("The failed ready hook leaked the mark, settlements, or part state.")
	}
	requireAdmissionRun(t, f, m.ID, runB, runA, 10)
	f.requireSaved(t, head)

	refuseReady.Store(false)
	peer = submitAdmission(t, f, client, m.ID, key, runB, 10)
	requireAdmissionWelcome(t, peer, 8)
	sendHandshake(t, peer, eventFrame(last))
	requireEventAck(t, peer, 9)
	requireAdmissionRun(t, f, m.ID, runB, runB, 10)
	firstFinished := requireSettlementTerminal(t, f.presenceFixture, first.id, "interrupted")
	secondFinished := requireSettlementTerminal(t, f.presenceFixture, second.id, "done")
	requireSettlementAudit(t, f.presenceFixture, first.id, OutcomeInterrupted, &nine, 9)
	requireSettlementAudit(t, f.presenceFixture, second.id, OutcomeDone, &nine, 9)
	if !storetest.Scalar[bool](t, f.obs, "SELECT count(*) = 1 AND min(due) = 2 FROM fixture_settlement_ready") {
		t.Error("The ready-run callback did not see both final-watermark settlements in its write.")
	}
	requireOrder := func() {
		t.Helper()
		got := storetest.Rows[string](t, f.obs, "SELECT command_id FROM fixture_settlements ORDER BY ordinal")
		if !slices.Equal(got, []string{earliest.id, first.id, second.id}) {
			t.Error("Replay did not settle each command once in watermark and command sequence order.")
		}
		if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 3 FROM feed WHERE topic = 'fixture_settlements'`) {
			t.Error("Replay did not publish exactly the three settlement references.")
		}
	}
	requireOrder()
	head = f.head(t)
	sendHandshake(t, peer, eventFrame(last))
	requireEventAck(t, peer, 9)
	sendJoinedResult(t, peer, settlementFrame(first.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED, 9))
	sendHandshake(t, peer, eventFrame(last))
	requireEventAck(t, peer, 9)
	requireOrder()
	if !requireSettlementTerminal(t, f.presenceFixture, first.id, "interrupted").Equal(firstFinished) ||
		!requireSettlementTerminal(t, f.presenceFixture, second.id, "done").Equal(secondFinished) {
		t.Error("A repeated event or result changed the original terminal record.")
	}
	f.requireSaved(t, head)
}

func seedJoinedStage(t *testing.T, f *presenceFixture, id, outcome string, number int64) {
	t.Helper()
	eventSQL(t, f, "UPDATE link_commands SET staged_outcome = $2, staged_event_number = $3 WHERE id = $1", id, outcome, number)
}

func sendJoinedResult(t *testing.T, peer *openClientStream, result *linkv1.CommandResult) {
	t.Helper()
	sendHandshake(t, peer, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: result}})
}

func TestStagedResultsAndWriterTurns(t *testing.T) {
	t.Run("StagedScanReleasesDeferredScan", func(t *testing.T) { testJoinedDeferredScans(t, 9) })
	t.Run("ImmediateScanUsesNextTurnWithoutWake", func(t *testing.T) { testJoinedDeferredScans(t, 5) })
	t.Run("ReconnectSkipsStagedWork", testJoinedResend)
	t.Run("SendRecheckPreservesStagedAssignment", testJoinedSendRecheck)
}

func testJoinedDeferredScans(t *testing.T, watermark uint64) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "deferred scan writer")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 5)
	expiry := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp() + interval '2 minutes'")
	first := seedClaimRow(t, f.presenceFixture, m.ID, scanAction(), expiry)
	next := seedClaimRow(t, f.presenceFixture, m.ID, scanAction(), expiry)
	barrier := seedClaimRow(t, f.presenceFixture, m.ID, restartAction(), expiry)
	peer := submitAdmission(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, m.ID, 6)
	requireAdmissionWelcome(t, peer, 5)
	readJoinedCommand(t, peer, first, 1)
	// The later restart proves the cursor has already walked past and deferred the second scan.
	readJoinedCommand(t, peer, barrier, 1)
	requireClaimStored(t, f.presenceFixture, next.id, "", 1)

	head := f.head(t)
	sendJoinedResult(t, peer, settlementFrame(first.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, watermark))
	// No follower or heartbeat is needed to give the writer its next turn.
	readJoinedCommand(t, peer, next, 1)
	if watermark > 5 {
		if !hasSettlementStage(t, f.presenceFixture, first.id, "done", int64(watermark)) ||
			!hasOneSettlementWake(t, f.presenceFixture, head, m.ID) {
			t.Error("The writer delivered the deferred scan without preserving the first scan's durable stage and single wake.")
		}
	} else {
		requireSettlementTerminal(t, f.presenceFixture, first.id, "done")
		f.requireSaved(t, head)
	}
}

func testJoinedResend(t *testing.T) {
	before := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := before.register(t, "resend staged work")
	before.approve(t, m.ID)
	stream := before.admit(t, m.ID, 1)
	restart := seedSettlementCommand(t, before.presenceFixture, m.ID, stream.ID, restartAction(), time.Minute)
	stagedStanding := seedSettlementCommand(t, before.presenceFixture, m.ID, stream.ID, standingAction(), -48*time.Hour)
	seedJoinedStage(t, before.presenceFixture, restart.id, "done", 1)
	seedJoinedStage(t, before.presenceFixture, stagedStanding.id, "done", 1)
	ordinary := seedClaimRow(t, before.presenceFixture, m.ID, standingAction(), stagedStanding.expires)
	f := restartAdmissionFixture(t, before)
	f.svc.streaming.options.Commands = f.state
	peer := submitAdmission(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, m.ID, 1)
	requireAdmissionWelcome(t, peer, 0)
	readJoinedCommand(t, peer, ordinary, 1)
	sendHandshake(t, peer, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, peer)
	for _, row := range []claimFixtureRow{restart, stagedStanding} {
		requireDispatch(t, f.presenceFixture, row, stream.ID, 1, true)
		if !hasSettlementStage(t, f.presenceFixture, row.id, "done", 1) {
			t.Error("Reconnection offered or altered a command that already held a result.")
		}
	}
}

func readJoinedCommand(t *testing.T, peer *openClientStream, row claimFixtureRow, attempt uint32) {
	t.Helper()
	frame := receiveOpenFrame(peer)
	if frame.err != nil || frame.frame.GetCommand() == nil {
		t.Fatal("The next writer turn did not deliver the expected command.")
	}
	row.requireFrame(t, frame.frame.GetCommand(), attempt)
}

func testJoinedSendRecheck(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, make([]byte, 32), "send recheck with a staged result")
	f.approve(t, m.ID)
	stream := f.admit(t, m.ID, 1)
	row := seedSettlementCommand(t, f, m.ID, stream.ID, restartAction(), time.Minute)
	seedJoinedStage(t, f, row.id, "done", 1)
	command := proto.CloneOf(row.action)
	command.Id, command.Attempt = row.id, 1
	writes := 0
	sent, err := f.state.sendClaimed(t.Context(), m.ID, stream.ID, command, func(context.Context) error {
		writes++
		return nil
	})
	if err != nil || sent || writes != 0 || !hasSettlementStage(t, f, row.id, "done", 1) {
		t.Error("A repeated send for the staged assignment wrote transport bytes or changed its result.")
	}
	requireDispatch(t, f, row, stream.ID, 1, true)
}

func TestResultAndRunAdmissionUseOneMachineLock(t *testing.T) {
	t.Run("StageWins", func(t *testing.T) { testJoinedAdmissionRace(t, true) })
	t.Run("AdmissionWins", func(t *testing.T) { testJoinedAdmissionRace(t, false) })
}

func testJoinedAdmissionRace(t *testing.T, stageFirst bool) {
	var traceLocks, holdNext atomic.Bool
	lockRequests := make(chan int32, 4)
	locked := make(chan struct{}, 1)
	held, release := storetest.Signal(t)
	defer release()
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
			if traceLocks.Load() {
				sql := strings.ToLower(query.SQL)
				if strings.Contains(sql, "from machines") && strings.Contains(sql, "for no key update") {
					lockRequests <- int32(conn.PgConn().PID())
				}
			}
			return ctx
		})
	}})
	f.state.afterLock = func(ctx context.Context, _ string) {
		if holdNext.CompareAndSwap(true, false) {
			locked <- struct{}{}
			select {
			case <-held:
			case <-ctx.Done():
			}
		}
	}
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "result racing a signed admission")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 5)
	runA, runB := fixtureStreamID(t), fixtureStreamID(t)
	// The handshake fixture keeps its steady writer idle while the two writes contend.
	client := f.wire(t, handshakeWireOptions{})
	a := submitAdmission(t, f, client, m.ID, key, runA, 6)
	requireAdmissionWelcome(t, a, 5)
	awaitAdmissionPublication(t, f, m.ID)
	oldStream := f.current(t, m.ID).StreamID
	command := seedSettlementCommand(t, f.presenceFixture, m.ID, oldStream, restartAction(), time.Minute)
	eventSQL(t, f.presenceFixture, "UPDATE link_machines SET recovery_set_generation = stream_generation WHERE machine_id = $1", m.ID)
	warning := storetest.Scalar[int64](t, f.obs, "SELECT recovery_set_generation FROM link_machines WHERE machine_id = $1", m.ID)
	oldResult := settlementFrame(command.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 9)
	sendResult := func() error {
		_, err := f.svc.dispatch(t.Context(), m.ID, oldStream, receivedFrame{
			request: &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: oldResult}},
		}, func(*linkv1.OpenResponse) error {
			return errors.New("A result unexpectedly wrote a response.")
		})
		return err
	}

	traceLocks.Store(true)
	holdNext.Store(true)
	var b *openClientStream
	var result *storetest.Task[error]
	if stageFirst {
		result = storetest.Start(t, sendResult)
	} else {
		b = submitAdmission(t, f, client, m.ID, key, runB, 9)
	}
	storetest.Receive(t, "the winning write holding the machine row", locked)
	holder := storetest.Receive(t, "the winning registry lock request", lockRequests)
	if stageFirst {
		b = submitAdmission(t, f, client, m.ID, key, runB, 9)
	} else {
		result = storetest.Start(t, sendResult)
	}
	waiter := storetest.Receive(t, "the competing registry lock request", lockRequests)
	storetest.WaitOnRow(t, f.obs, waiter, holder)
	traceLocks.Store(false)
	release()

	if stageFirst {
		if result.Wait(t) != nil {
			t.Fatal("The incumbent could not stage its dispatched result.")
		}
		if !hasSettlementStage(t, f.presenceFixture, command.id, "done", 9) {
			t.Error("The result that won the lock did not survive as a stage for the admission fence.")
		}
		requireOpenEnd(t, b, connect.CodeInvalidArgument, openInvalid, "")
		requireAdmissionRun(t, f, m.ID, runA, runA, 6)
		requireDispatch(t, f.presenceFixture, command, oldStream, 1, true)
		sendHandshake(t, a, heartbeatFrame(nil, nil))
		requireHeartbeatAck(t, a)
	} else {
		requireAdmissionWelcome(t, b, 5)
		awaitAdmissionPublication(t, f, m.ID)
		if !errors.Is(result.Wait(t), ErrNotCurrent) {
			t.Error("The replacement that won the lock did not fence the old result frame.")
		}
		requireAdmissionRun(t, f, m.ID, runB, runA, 9)
		requireOpenEnd(t, a, connect.CodeAborted, openReplaced, "")
		if !storetest.Scalar[bool](t, f.obs, `SELECT outcome IS NULL AND staged_outcome IS NULL
			AND attempt = 1 AND dispatched AND sent_stream_id = $2 FROM link_commands WHERE id = $1`, command.id, oldStream) {
			t.Error("The old result changed the pending first attempt after replacement.")
		}
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT recovery_set_generation IS NOT DISTINCT FROM $2
			AND event_mark = 5 FROM link_machines WHERE machine_id = $1`, m.ID, warning) {
		t.Error("The admission race changed the recovery warning or event mark.")
	}
	if stageFirst {
		return
	}

	stream := f.current(t, m.ID).StreamID
	for number := uint64(6); number <= 8; number++ {
		requireEvent(t, f.presenceFixture, m.ID, stream, &linkv1.Event{Number: number}, number, true)
	}
	var cursor commandCursor
	retry := claimAt(t, f.presenceFixture, m.ID, stream, &cursor)
	command.requireFrame(t, retry, 2)
	sent, err := f.state.sendClaimed(t.Context(), m.ID, stream, retry, func(context.Context) error { return nil })
	if err != nil || !sent {
		t.Fatal("The current ready stream could not dispatch the eligible new attempt.")
	}
	requireResultError(t, f.presenceFixture, m.ID, stream, settlementFrame(command.id, 2, oldResult.GetOutcome(), 9), nil)
	if !hasSettlementStage(t, f.presenceFixture, command.id, "done", 9) {
		t.Error("The result was not staged when it finally answered the current dispatched attempt.")
	}
}

func TestLinkRemovalSettlesAndCallsParts(t *testing.T) {
	t.Run("FlaggedMachineRollsBackAndRetries", testJoinedRemoval)
	t.Run("NoCommands", testJoinedRemovalWithoutCommands)
	t.Run("WaitingTokenRevocation", func(t *testing.T) { testJoinedTokenRevocation(t, false) })
	t.Run("ApprovedTokenRevocation", func(t *testing.T) { testJoinedTokenRevocation(t, true) })
	t.Run("ResultWinsRemovalRace", func(t *testing.T) { testJoinedRemovalRace(t, true) })
	t.Run("RemovalWinsResultRace", func(t *testing.T) { testJoinedRemovalRace(t, false) })
}

func testJoinedRemovalRace(t *testing.T, resultFirst bool) {
	var enabled atomic.Bool
	resultPID, removalPID := make(chan int32, 1), make(chan int32, 1)
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
			if enabled.Load() {
				sql := strings.ToLower(query.SQL)
				switch {
				case strings.Contains(sql, "delete from machines where"):
					removalPID <- int32(conn.PgConn().PID())
				case strings.Contains(sql, "from machines") && strings.Contains(sql, "for no key update"):
					resultPID <- int32(conn.PgConn().PID())
				}
			}
			return ctx
		})
	})
	m := f.machine(t, make([]byte, 32), "removal racing an eligible result")
	f.approve(t, m.ID)
	stream := f.admit(t, m.ID, 1)
	command := seedSettlementCommand(t, f, m.ID, stream.ID, restartAction(), time.Minute)
	newSettlementAudit(t, f)
	registerRestartSettlement(t, f, recordRestartSettlement)
	held, release := storetest.Signal(t)
	defer release()
	enter := make(chan struct{}, 1)
	pause := func(ctx context.Context) {
		enter <- struct{}{}
		select {
		case <-held:
		case <-ctx.Done():
		}
	}
	f.state.afterLock = func(ctx context.Context, _ string) {
		if resultFirst {
			pause(ctx)
		}
	}
	finish := func() error {
		return f.state.AcceptResult(t.Context(), m.ID, stream.ID,
			settlementFrame(command.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 0))
	}
	remove := func() error {
		// Use the same registry deletion and Forget write as the Machines handler.
		return f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if err := f.r.Delete(ctx, tx, m.ID); err != nil {
				return err
			}
			if !resultFirst {
				pause(ctx)
			}
			return f.state.Forget(ctx, tx, m.ID)
		})
	}
	head := f.head(t)
	enabled.Store(true)
	var result, removal *storetest.Task[error]
	if resultFirst {
		result = storetest.Start(t, finish)
	} else {
		removal = storetest.Start(t, remove)
	}
	storetest.Receive(t, "the first write holding the removal race's machine row", enter)
	if resultFirst {
		removal = storetest.Start(t, remove)
	} else {
		result = storetest.Start(t, finish)
	}
	resultBackend := storetest.Receive(t, "the result lock request", resultPID)
	removalBackend := storetest.Receive(t, "the removal lock request", removalPID)
	if resultFirst {
		storetest.WaitOnRow(t, f.obs, removalBackend, resultBackend)
	} else {
		storetest.WaitOnRow(t, f.obs, resultBackend, removalBackend)
	}
	enabled.Store(false)
	release()
	if removal.Wait(t) != nil {
		t.Fatal("The serialized removal failed.")
	}
	if resultFirst {
		if result.Wait(t) != nil {
			t.Error("The result did not commit before the removal.")
		}
		requireSettlementTerminal(t, f, command.id, "done")
		zero := int64(0)
		requireSettlementAudit(t, f, command.id, OutcomeDone, &zero, 0)
	} else {
		if !errors.Is(result.Wait(t), ErrUnknownMachine) {
			t.Error("Removal did not fence the outstanding result.")
		}
		requireSettlementTerminal(t, f, command.id, "removed")
		requireSettlementAudit(t, f, command.id, OutcomeRemoved, nil, 0)
	}
	if settlementAuditCount(t, f, command.id) != 1 {
		t.Error("The locked result and removal did not produce exactly one settlement.")
	}
	refs := storetest.Rows[string](t, f.obs, "SELECT topic || '/' || change FROM feed WHERE position > $1 ORDER BY position", head)
	if !slices.Equal(refs, []string{"machines/removed"}) {
		t.Error("The race added a feed reference beyond the Machines removal.")
	}
}

func testJoinedRemoval(t *testing.T) {
	var fail atomic.Bool
	var acks atomic.Int32
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventAck: func(context.Context, string, uint64) { acks.Add(1) },
	}})
	m, key := f.register(t, "flagged removal with an early result")
	f.approve(t, m.ID)
	installJoinedRemovalHooks(t, f, &fail)
	peer := submitAdmission(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, m.ID, 1)
	requireAdmissionWelcome(t, peer, 0)
	awaitAdmissionPublication(t, f, m.ID)
	stream := f.current(t, m.ID).StreamID
	old := seedSettlementCommand(t, f.presenceFixture, m.ID, stream, restartAction(), time.Minute)
	next := seedClaimRow(t, f.presenceFixture, m.ID, restartAction(), old.expires)
	seedJoinedStage(t, f.presenceFixture, old.id, "interrupted", 9)
	eventSQL(t, f.presenceFixture, "UPDATE link_machines SET recovery_set_generation = stream_generation WHERE machine_id = $1", m.ID)
	original := readAdmissionRows(t, f, m.ID)

	owner := machines.NewService(f.s, nil, f.r, f.tokens).WithLink(f.state)
	remove := func() error {
		_, err := owner.RemoveClient(t.Context(), connect.NewRequest(&webv1.RemoveClientRequest{Id: m.ID}))
		return err
	}
	fail.Store(true)
	head := f.head(t)
	if connect.CodeOf(remove()) != connect.CodeUnavailable {
		t.Error("A failing lifecycle callback did not abort machine removal.")
	}
	requireAdmissionUnchanged(t, f, m.ID, head, original)
	requireJoinedRemovalState(t, f, m.ID, true, 0)
	if settlementAuditCount(t, f.presenceFixture, old.id)+settlementAuditCount(t, f.presenceFixture, next.id) != 0 {
		t.Error("The failed machine removal committed a settlement callback's writes.")
	}
	if !storetest.Scalar[bool](t, f.obs, "SELECT recovery_set_generation IS NOT NULL FROM link_machines WHERE machine_id = $1", m.ID) {
		t.Error("Rollback lost the recovery warning.")
	}

	fail.Store(false)
	if remove() != nil {
		t.Fatal("Retry could not remove the flagged machine and finish its staged and unfinished commands.")
	}
	requireJoinedRemovalState(t, f, m.ID, false, 1)
	for _, command := range []claimFixtureRow{old, next} {
		requireSettlementTerminal(t, f.presenceFixture, command.id, "removed")
		// Removal supersedes the stored client result, so it reports a server-origin outcome.
		requireSettlementAudit(t, f.presenceFixture, command.id, OutcomeRemoved, nil, 0)
	}
	if !storetest.Scalar[bool](t, f.obs, "SELECT count(*) = 2 FROM link_commands WHERE machine_id = $1", m.ID) {
		t.Error("Removal failed to retain the two terminal rows or accepted a callback's enqueue.")
	}
	requireJoinedRemovalFeed(t, f, head, m.ID, "fixture_settlements//"+old.id+"/saved", "fixture_settlements//"+next.id+"/saved")

	head = f.head(t)
	sendHandshake(t, peer, eventFrame(factsEvent(1, "Late frame after removal")))
	requireOpenEnd(t, peer, connect.CodeFailedPrecondition, openUnknown, "unknown_machine")
	if acks.Load() != 0 || settlementAuditCount(t, f.presenceFixture, old.id) != 1 ||
		settlementAuditCount(t, f.presenceFixture, next.id) != 1 {
		t.Error("The obsolete frame reached an acknowledgement or settlement callback.")
	}
	requireJoinedRemovalState(t, f, m.ID, false, 1)
	f.requireSaved(t, head)
}

func testJoinedRemovalWithoutCommands(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, _ := f.register(t, "removal with no commands")
	f.approve(t, m.ID)
	f.admit(t, m.ID, 1)
	installJoinedRemovalHooks(t, f, nil)
	if err := f.state.RegisterPart(PartHooks{MachineRemoved: func(ctx context.Context, tx *store.Tx, id string) error {
		var firstRan bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT FROM fixture_machine_removals WHERE machine_id = $1)", id).Scan(&firstRan); err != nil {
			return err
		}
		if !firstRan {
			return store.NewDomainError("The second part did not see the first part's removal observation.")
		}
		return tx.Record(store.ChangeRef{Topic: "fixture_secondary_removals", Key: id, Change: store.Saved})
	}}); err != nil {
		t.Fatal("The fixture could not register the second removal hook.")
	}
	owner := machines.NewService(f.s, nil, f.r, f.tokens).WithLink(f.state)
	head := f.head(t)
	if _, err := owner.RemoveClient(t.Context(), connect.NewRequest(&webv1.RemoveClientRequest{Id: m.ID})); err != nil {
		t.Fatal("The service could not remove the machine with an empty ledger.")
	}
	requireJoinedRemovalState(t, f, m.ID, false, 1)
	if outboxCount(t, f.presenceFixture) != 0 {
		t.Error("Removing a machine with no work added a command.")
	}
	requireJoinedRemovalFeed(t, f, head, m.ID, "fixture_secondary_removals//"+m.ID+"/saved")
}

func testJoinedTokenRevocation(t *testing.T, approved bool) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	issued, err := f.tokens.Create(t.Context(), machines.TokenOptions{})
	if err != nil {
		t.Fatal("The fixture could not issue a join token.")
	}
	token, err := machines.DecodeToken(issued.Text)
	if err != nil {
		t.Fatal("The fixture could not decode its join token.")
	}
	joined, err := f.tokens.Redeem(t.Context(), machines.Redemption{
		TokenID: token.GetTokenId(), Secret: token.GetSecret(),
		PublicKey: f.serverKey, HostName: "token revocation settlement",
	})
	if err != nil {
		t.Fatal("The fixture could not redeem its join token.")
	}
	id := joined.MachineID
	if approved {
		f.approve(t, id)
	}
	stream := f.admit(t, id, 1)
	installJoinedRemovalHooks(t, f, nil)
	pending := seedClaimRow(t, f.presenceFixture, id, standingAction(), stream.ServerTime.Add(time.Minute))
	owner := machines.NewService(f.s, nil, f.r, f.tokens).WithLink(f.state)
	head := f.head(t)
	if _, err := owner.RevokeClientToken(t.Context(), connect.NewRequest(&webv1.RevokeClientTokenRequest{TokenId: token.GetTokenId()})); err != nil {
		t.Fatal("The service could not revoke the machine's used token.")
	}
	if !storetest.Scalar[bool](t, f.obs, "SELECT revoked_at IS NOT NULL FROM client_join_tokens WHERE id = $1", token.GetTokenId()) {
		t.Error("The token was not revoked in the write.")
	}
	if approved {
		requireJoinedRemovalState(t, f, id, true, 0)
		requireClaimStored(t, f.presenceFixture, pending.id, "", 1)
		refs := joinedFeedRefs(t, f, head)
		if len(refs) != 1 || !strings.HasPrefix(refs[0], "client_join_tokens//") || !strings.HasSuffix(refs[0], "/saved") {
			t.Error("Approved token revocation did not limit the feed to the changed token.")
		}
	} else {
		requireJoinedRemovalState(t, f, id, false, 1)
		requireSettlementTerminal(t, f.presenceFixture, pending.id, "removed")
		requireJoinedRemovalFeed(t, f, head, id, "client_join_tokens//"+token.GetTokenId()+"/saved")
	}
}

func installJoinedRemovalHooks(t *testing.T, f *handshakeFixture, fail *atomic.Bool) {
	t.Helper()
	newSettlementAudit(t, f.presenceFixture)
	eventSQL(t, f.presenceFixture, `CREATE TABLE fixture_machine_removals (machine_id text PRIMARY KEY)`)
	missing := func(ctx context.Context, tx *store.Tx, machineID string) error {
		id, err := f.state.EnqueueCommand(ctx, tx, machineID, restartAction(), time.Minute)
		if id != "" || !errors.Is(err, ErrUnknownMachine) {
			return store.NewDomainError("A removal callback did not receive the missing-machine refusal and empty command identifier.")
		}
		return nil
	}
	if err := f.state.RegisterPart(PartHooks{
		CommandSettled: map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
			CommandKind("restart"): func(ctx context.Context, tx *store.Tx, settlement Settlement) error {
				if settlement.Outcome != OutcomeRemoved {
					return store.NewDomainError("Machine removal passed the wrong outcome to its settlement hook.")
				}
				if err := recordRestartSettlement(ctx, tx, settlement); err != nil {
					return err
				}
				if err := missing(ctx, tx, settlement.MachineID); err != nil {
					return err
				}
				return tx.Record(store.ChangeRef{Topic: "fixture_settlements", Key: settlement.Command.GetId(), Change: store.Saved})
			},
		},
		MachineRemoved: func(ctx context.Context, tx *store.Tx, id string) error {
			if err := missing(ctx, tx, id); err != nil {
				return err
			}
			var unfinished, linked bool
			if err := tx.QueryRow(ctx, `SELECT
				EXISTS (SELECT FROM link_commands WHERE machine_id = $1 AND outcome IS NULL),
				EXISTS (SELECT FROM link_machines JOIN link_connections USING (machine_id) WHERE machine_id = $1)`,
				id).Scan(&unfinished, &linked); err != nil {
				return err
			}
			if unfinished || !linked {
				return store.NewDomainError("The removal hook did not run between command settlement and Link cleanup.")
			}
			if _, err := tx.Exec(ctx, "INSERT INTO fixture_machine_removals VALUES ($1)", id); err != nil {
				return err
			}
			if err := tx.Record(store.ChangeRef{Topic: "fixture_removals", Key: id, Change: store.Saved}); err != nil {
				return err
			}
			if fail != nil && fail.Load() {
				return store.NewDomainError("The fixture refused machine removal after the callbacks wrote their records.")
			}
			return nil
		},
	}); err != nil {
		t.Fatal("The machine removal fixture could not register its callbacks.")
	}
}

func requireJoinedRemovalState(t *testing.T, f *handshakeFixture, id string, present bool, calls int64) {
	t.Helper()
	if !storetest.Scalar[bool](t, f.obs, `SELECT
		(EXISTS (SELECT FROM machines WHERE id = $1)) = $2
		AND (EXISTS (SELECT FROM link_machines WHERE machine_id = $1)) = $2
		AND (EXISTS (SELECT FROM link_connections WHERE machine_id = $1)) = $2
		AND (EXISTS (SELECT FROM link_runs WHERE machine_id = $1)) = $2
		AND (SELECT count(*) FROM fixture_machine_removals WHERE machine_id = $1) = $3`, id, present, calls) {
		t.Error("The machine, Link rows or committed removal-callback count did not match the removal outcome.")
	}
}

func joinedFeedRefs(t *testing.T, f *handshakeFixture, after int64) []string {
	t.Helper()
	return storetest.Rows[string](t, f.obs, `SELECT topic || '/' || scope || '/' || key || '/' || change
		FROM feed WHERE position > $1 ORDER BY position`, after)
}

func requireJoinedRemovalFeed(t *testing.T, f *handshakeFixture, after int64, id string, extra ...string) {
	t.Helper()
	want := append([]string{"machines//" + id + "/removed", "fixture_removals//" + id + "/saved"}, extra...)
	got := joinedFeedRefs(t, f, after)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Error("Removal did not publish exactly the Machines and part references.")
	}
}
