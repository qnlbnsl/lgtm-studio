package link

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestStagedResultSurvivesRestartAndSweep(t *testing.T) {
	server := newServerStreamFixture(t, handshakeFixtureOptions{})
	f := server.presenceFixture
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "restart settlement")
	f.approve(t, machine.ID)
	stream := f.admit(t, machine.ID, 1)
	outboxFixtureExec(t, f.obs, "UPDATE link_machines SET event_mark = 5 WHERE machine_id = $1", machine.ID)
	newSettlementAudit(t, f)
	registerRestartSettlement(t, f, recordRestartSettlement)

	var ids []string
	var payloads [][]byte
	staged := []struct {
		watermark int64
		outcome   linkv1.CommandOutcome
		want      string
	}{
		{9, linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED, "interrupted"},
		{9, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, "done"},
		{7, linkv1.CommandOutcome_COMMAND_OUTCOME_UNSUPPORTED, "unsupported"},
	}
	for _, tc := range staged {
		command := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), 2*time.Minute)
		ids = append(ids, command.id)
		payloads = append(payloads, storetest.Scalar[[]byte](t, f.obs, "SELECT payload FROM link_commands WHERE id = $1", command.id))
		head := f.head(t)
		result := settlementFrame(command.id, 1, tc.outcome, uint64(tc.watermark))
		requireResultError(t, f, machine.ID, stream.ID, result, nil)
		if !hasSettlementStage(t, f, command.id, tc.want, tc.watermark) {
			t.Errorf("The eligible result with watermark %d did not stage as %s.", tc.watermark, tc.want)
		}
		if !hasOneSettlementWake(t, f, head, machine.ID) {
			t.Error("A first stage did not commit exactly one outbox wake.")
		}
	}
	head := f.head(t)
	for _, mark := range []uint64{5, 13} {
		conflict := settlementFrame(ids[0], 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, mark)
		requireResultError(t, f, machine.ID, stream.ID, conflict, nil)
	}
	if !hasSettlementStage(t, f, ids[0], "interrupted", 9) {
		t.Error("A later result replaced the first staged outcome or watermark.")
	}
	f.requireSaved(t, head)

	reopened := restartAdmissionFixture(t, server)
	f = reopened.presenceFixture
	registerRestartSettlement(t, f, recordRestartSettlement)
	reconnected := f.admit(t, machine.ID, 2)
	if reconnected.Mark != 5 {
		t.Error("The reopened store did not return the original committed mark on reconnect.")
	}
	outboxFixtureExec(t, f.obs, `UPDATE link_commands
		SET expires_at = clock_timestamp() - interval '48 hours',
		    created_at = clock_timestamp() - interval '49 hours'
		WHERE id = ANY ($1)`, ids)
	ordinary := seedSettlementCommand(t, f, machine.ID, reconnected.ID, restartAction(), -48*time.Hour)
	head = f.head(t)
	if err := f.state.SweepCommands(t.Context()); err != nil {
		t.Error("The sweep failed after reconnect.")
	}
	requireSettlementTerminal(t, f, ordinary.id, "timed_out")
	requireSettlementAudit(t, f, ordinary.id, OutcomeTimedOut, nil, 5)
	for i, tc := range staged {
		if !hasSettlementStage(t, f, ids[i], tc.want, tc.watermark) ||
			!storetest.Scalar[bool](t, f.obs, `SELECT attempt = 1 AND dispatched AND sent_stream_id = $2 AND payload = $3
				FROM link_commands WHERE id = $1`, ids[i], stream.ID, payloads[i]) {
			t.Error("After restart and sweep, a command lost its original stage, payload or dispatch metadata.")
		}
		if settlementAuditCount(t, f, ids[i]) != 0 {
			t.Error("A staged result invoked its settlement callback before the watermark was covered.")
		}
	}
	f.requireSaved(t, head)
}

func TestResultFenceKeepsAttemptAndCommandRules(t *testing.T) {
	t.Run("OnlyEligibleAttemptsCanStage", testOnlyEligibleResultsCanStage)
	t.Run("CoveredWatermarksAndBounds", testCoveredResultWatermarks)
	t.Run("CoveredDuplicateKeepsFirstStage", testCoveredDuplicateKeepsFirstStage)
	t.Run("StandingNoticeAfterExpiry", testStagedStandingNotice)
	t.Run("ResultWinsAfterSweepSelection", testStageBeforeSweepLock)
	t.Run("SweepWinsBeforeResult", testSweepBeforeStagingResult)
	t.Run("SweepCallbacksRollBackTogether", testSweepSettlementRollback)
	t.Run("LiveSettlementEnqueuesFollowups", testSettlementEnqueuesFollowups)
}

func testOnlyEligibleResultsCanStage(t *testing.T) {
	f := newPresenceFixture(t, nil)
	target := f.machine(t, bytes.Repeat([]byte{1}, 32), "target for result fences")
	other := f.machine(t, bytes.Repeat([]byte{2}, 32), "other result sender")
	waiting := f.machine(t, bytes.Repeat([]byte{3}, 32), "waiting result sender")
	f.approve(t, target.ID)
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.r.SetApproved(ctx, tx, other.ID, "Approved sender")
	}); err != nil {
		t.Fatal("The other fixture machine could not be approved.")
	}
	obsolete := f.admit(t, target.ID, 1)
	current := f.admit(t, target.ID, 2)
	otherStream := f.admit(t, other.ID, 1)
	waitingStream := f.admit(t, waiting.ID, 1)
	matching := seedSettlementCommand(t, f, target.ID, current.ID, restartAction(), time.Minute)
	outboxFixtureExec(t, f.obs, "UPDATE link_commands SET attempt = 2 WHERE id = $1", matching.id)
	wrongMachine := seedSettlementCommand(t, f, target.ID, otherStream.ID, restartAction(), time.Minute)
	wrongStream := seedSettlementCommand(t, f, target.ID, obsolete.ID, restartAction(), time.Minute)
	unapproved := seedSettlementCommand(t, f, waiting.ID, waitingStream.ID, standingAction(), time.Minute)
	undelivered := seedClaimRow(t, f, target.ID, restartAction(), current.ServerTime.Add(time.Minute))
	outboxFixtureExec(t, f.obs, "UPDATE link_commands SET sent_stream_id = $2 WHERE id = $1", undelivered.id, current.ID)
	head := f.head(t)
	for _, tc := range []struct {
		name, machineID, streamID, commandID string
		attempt                              uint32
		want                                 error
	}{
		{"wrong machine alone", other.ID, otherStream.ID, wrongMachine.id, 1, nil},
		{"obsolete stream", target.ID, obsolete.ID, wrongStream.id, 1, ErrNotCurrent},
		{"old assignment on the current stream", target.ID, current.ID, wrongStream.id, 1, nil},
		{"claimed but undelivered", target.ID, current.ID, undelivered.id, 1, nil},
		{"waiting machine", waiting.ID, waitingStream.ID, unapproved.id, 1, nil},
		{"later attempt", target.ID, current.ID, matching.id, 3, nil},
		{"stale attempt", target.ID, current.ID, matching.id, 1, nil},
	} {
		result := settlementFrame(tc.commandID, tc.attempt, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 9)
		if err := f.state.AcceptResult(t.Context(), tc.machineID, tc.streamID, result); !errors.Is(err, tc.want) {
			t.Errorf("The result had an unexpected status for %s.", tc.name)
		}
		if !hasSettlementStage(t, f, tc.commandID, "", 0) {
			t.Errorf("The result changed the command for %s.", tc.name)
		}
	}
	f.requireSaved(t, head)
	result := settlementFrame(matching.id, 2, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 9)
	requireResultError(t, f, target.ID, current.ID, result, nil)
	if !hasSettlementStage(t, f, matching.id, "done", 9) {
		t.Error("A matching result did not stage after the mismatched inputs.")
	}
	requirePartOutboxWakes(t, f, head, target.ID, 1)
}

func testCoveredResultWatermarks(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "covered result")
	f.approve(t, machine.ID)
	stream := f.admit(t, machine.ID, 1)
	outboxFixtureExec(t, f.obs, "UPDATE link_machines SET event_mark = 5 WHERE machine_id = $1", machine.ID)
	newSettlementAudit(t, f)
	registerRestartSettlement(t, f, recordRestartSettlement)
	head := f.head(t)
	for _, tc := range []struct {
		name      string
		watermark uint64
		outcome   linkv1.CommandOutcome
		want      SettlementOutcome
	}{
		{"legacy or empty history", 0, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, OutcomeDone},
		{"equal to the mark", 5, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, OutcomeDone},
		{"interrupted from an older process", 3, linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED, OutcomeInterrupted},
		{"unspecified outcome", 5, linkv1.CommandOutcome_COMMAND_OUTCOME_UNSPECIFIED, OutcomeFailed},
		{"unknown outcome", 5, linkv1.CommandOutcome(97), OutcomeFailed},
	} {
		command := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), time.Minute)
		result := settlementFrame(command.id, 1, tc.outcome, tc.watermark)
		if err := f.state.AcceptResult(t.Context(), machine.ID, stream.ID, result); err != nil {
			t.Errorf("The covered result was refused for %s.", tc.name)
		}
		requireSettlementTerminal(t, f, command.id, string(tc.want))
		watermark := int64(tc.watermark)
		requireSettlementAudit(t, f, command.id, tc.want, &watermark, 5)
		requireResultError(t, f, machine.ID, stream.ID, result, nil)
		if settlementAuditCount(t, f, command.id) != 1 {
			t.Errorf("The covered result did not invoke its hook exactly once for %s.", tc.name)
		}
	}
	f.requireSaved(t, head)

	command := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), time.Minute)
	overflow := settlementFrame(command.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, uint64(math.MaxInt64)+1)
	requireResultError(t, f, machine.ID, stream.ID, overflow, ErrInvalidMessage)
	if !hasSettlementStage(t, f, command.id, "", 0) || settlementAuditCount(t, f, command.id) != 0 {
		t.Error("An out-of-range watermark changed the command or invoked its hook.")
	}
	f.requireSaved(t, head)
	maximum := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), time.Minute)
	result := settlementFrame(maximum.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, math.MaxInt64)
	requireResultError(t, f, machine.ID, stream.ID, result, nil)
	if !hasSettlementStage(t, f, maximum.id, "done", math.MaxInt64) {
		t.Error("The largest legal result watermark was not staged exactly.")
	}
	requirePartOutboxWakes(t, f, head, machine.ID, 1)
}

func testStagedStandingNotice(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "standing settlement")
	f.approve(t, machine.ID)
	stream := f.admit(t, machine.ID, 1)
	notice := seedSettlementCommand(t, f, machine.ID, stream.ID, standingAction(), -48*time.Hour)
	control := seedSettlementCommand(t, f, machine.ID, stream.ID, standingAction(), -48*time.Hour)

	head := f.head(t)
	result := settlementFrame(notice.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED, 9)
	requireResultError(t, f, machine.ID, stream.ID, result, nil)
	if !hasOneSettlementWake(t, f, head, machine.ID) {
		t.Error("The standing notice stage did not commit exactly one outbox wake.")
	}
	head = f.head(t)
	if err := f.state.SweepCommands(t.Context()); err != nil {
		t.Error("The sweep failed with aged standing notices.")
	}
	if !hasSettlementStage(t, f, notice.id, "failed", 9) || !hasSettlementStage(t, f, control.id, "", 0) {
		t.Error("The sweep did not preserve both the staged failed notice and the unanswered notice past normal expiry.")
	}
	f.requireSaved(t, head)
}

func testCoveredDuplicateKeepsFirstStage(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "duplicate staged result")
	f.approve(t, machine.ID)
	stream := f.admit(t, machine.ID, 1)
	outboxFixtureExec(t, f.obs, "UPDATE link_machines SET event_mark = 5 WHERE machine_id = $1", machine.ID)
	newSettlementAudit(t, f)
	registerRestartSettlement(t, f, recordRestartSettlement)
	command := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), time.Minute)
	outboxFixtureExec(t, f.obs, `UPDATE link_commands SET staged_outcome = 'interrupted', staged_event_number = 9
		WHERE id = $1`, command.id)
	head := f.head(t)
	covered := settlementFrame(command.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 5)
	requireResultError(t, f, machine.ID, stream.ID, covered, nil)
	if !hasSettlementStage(t, f, command.id, "interrupted", 9) || settlementAuditCount(t, f, command.id) != 0 {
		t.Error("A conflicting covered result replaced the first stage or settled the command early.")
	}
	f.requireSaved(t, head)
}

type settlementSweepContextKey struct{}

func testStageBeforeSweepLock(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stage func(*testing.T, *presenceFixture, string, string, string)
	}{
		{"ThroughResult", func(t *testing.T, f *presenceFixture, machineID, streamID, commandID string) {
			result := settlementFrame(commandID, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 9)
			requireResultError(t, f, machineID, streamID, result, nil)
		}},
		{"PreseededStageIsRechecked", func(t *testing.T, f *presenceFixture, _, _, commandID string) {
			// Seed after selection to isolate the lock-time predicate from result handling.
			outboxFixtureExec(t, f.obs, `UPDATE link_commands SET staged_outcome = 'done', staged_event_number = 9
				WHERE id = $1`, commandID)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			atLock := make(chan struct{})
			hold, resume := storetest.Signal(t)
			var entered sync.Once
			f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
				cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
					if ctx.Value(settlementSweepContextKey{}) == true && strings.Contains(strings.ToLower(query.SQL), "nowait") {
						entered.Do(func() {
							close(atLock)
							select {
							case <-hold:
							case <-ctx.Done():
							}
						})
					}
					return ctx
				})
			})
			machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "selected sweep candidate")
			f.approve(t, machine.ID)
			stream := f.admit(t, machine.ID, 1)
			command := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), -48*time.Hour)
			ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
			defer cancel()
			sweep := storetest.Start(t, func() error {
				return f.state.SweepCommands(context.WithValue(ctx, settlementSweepContextKey{}, true))
			})
			defer resume()
			claimGate(t, sweep, atLock, "The sweep did not reach its machine lock after selecting the due command.")
			tc.stage(t, f, machine.ID, stream.ID, command.id)
			if !hasSettlementStage(t, f, command.id, "done", 9) {
				t.Error("The command was not staged before the sweep took its machine lock.")
			}
			head := f.head(t)
			resume()
			if err := sweep.Wait(t); err != nil {
				t.Error("The sweep failed when its selected command became staged before the locked recheck.")
			}
			if !hasSettlementStage(t, f, command.id, "done", 9) {
				t.Error("The locked sweep recheck did not leave the staged command intact.")
			}
			f.requireSaved(t, head)
		})
	}
}

func testSweepBeforeStagingResult(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "sweep before result")
	f.approve(t, machine.ID)
	stream := f.admit(t, machine.ID, 1)
	newSettlementAudit(t, f)
	registerRestartSettlement(t, f, recordRestartSettlement)
	command := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), -48*time.Hour)
	head := f.head(t)
	if err := f.state.SweepCommands(t.Context()); err != nil {
		t.Fatal("The ordinary sweep did not finish.")
	}
	finished := requireSettlementTerminal(t, f, command.id, "timed_out")
	result := settlementFrame(command.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 9)
	requireResultError(t, f, machine.ID, stream.ID, result, nil)
	if after := requireSettlementTerminal(t, f, command.id, "timed_out"); !after.Equal(finished) {
		t.Error("The late result changed the first finish time.")
	}
	requireSettlementAudit(t, f, command.id, OutcomeTimedOut, nil, 0)
	if settlementAuditCount(t, f, command.id) != 1 {
		t.Error("The late result changed the number of timeout callbacks.")
	}
	f.requireSaved(t, head)
}

func testSweepSettlementRollback(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "sweep rollback")
	f.approve(t, machine.ID)
	stream := f.admit(t, machine.ID, 1)
	newSettlementAudit(t, f)
	first := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), -49*time.Hour)
	second := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), -48*time.Hour)
	const partTopic = "fixture_settlements"
	injected := store.NewDomainError("The fixture aborted its settlement write.")
	fail := true
	registerRestartSettlement(t, f, func(ctx context.Context, tx *store.Tx, event Settlement) error {
		if err := recordRestartSettlement(ctx, tx, event); err != nil {
			return err
		}
		id := event.Command.GetId()
		if err := tx.Record(store.ChangeRef{Topic: partTopic, Key: id, Change: store.Saved}); err != nil {
			return err
		}
		if id == second.id && fail {
			return injected
		}
		return nil
	})
	head := f.head(t)
	if err := f.state.SweepCommands(t.Context()); !errors.Is(err, injected) {
		t.Error("The second timeout callback did not abort the sweep write.")
	}
	for _, id := range []string{first.id, second.id} {
		if !hasSettlementStage(t, f, id, "", 0) || settlementAuditCount(t, f, id) != 0 {
			t.Error("A failed sweep committed a partial terminal row or callback record.")
		}
	}
	f.requireSaved(t, head)
	fail = false
	if err := f.state.SweepCommands(t.Context()); err != nil {
		t.Fatal("The sweep could not retry the ordinary timeouts.")
	}
	for _, id := range []string{first.id, second.id} {
		requireSettlementTerminal(t, f, id, "timed_out")
		requireSettlementAudit(t, f, id, OutcomeTimedOut, nil, 0)
		if settlementAuditCount(t, f, id) != 1 {
			t.Error("The sweep did not commit exactly one callback per timed-out command.")
		}
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 2 AND count(DISTINCT key) = 2
		AND COALESCE(bool_and(topic = $2 AND scope = '' AND key = ANY ($3) AND change = 'saved'), false)
		FROM feed WHERE position > $1`, head, partTopic, []string{first.id, second.id}) {
		t.Error("The successful sweep did not preserve exactly the two part references.")
	}
}

func testSettlementEnqueuesFollowups(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "result with followups")
	f.approve(t, machine.ID)
	stream := f.admit(t, machine.ID, 1)
	newSettlementAudit(t, f)
	command := seedSettlementCommand(t, f, machine.ID, stream.ID, restartAction(), time.Minute)
	outboxFixtureExec(t, f.obs, `CREATE TABLE fixture_settlement_followups (
		command_id text PRIMARY KEY REFERENCES link_commands (id))`)
	injected := store.NewDomainError("The fixture aborted after enqueueing its follow-up commands.")
	fail := true
	registerRestartSettlement(t, f, func(ctx context.Context, tx *store.Tx, event Settlement) error {
		if err := recordRestartSettlement(ctx, tx, event); err != nil {
			return err
		}
		for range 2 {
			id, err := f.state.EnqueueCommand(ctx, tx, event.MachineID, restartAction(), time.Minute)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "INSERT INTO fixture_settlement_followups (command_id) VALUES ($1)", id); err != nil {
				return err
			}
		}
		if fail {
			return injected
		}
		return nil
	})
	head := f.head(t)
	result := settlementFrame(command.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 0)
	requireResultError(t, f, machine.ID, stream.ID, result, injected)
	if !hasSettlementStage(t, f, command.id, "", 0) || settlementAuditCount(t, f, command.id) != 0 ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_commands WHERE machine_id = $1", machine.ID) != 1 {
		t.Error("The failed live settlement committed a terminal row, callback record or follow-up.")
	}
	f.requireSaved(t, head)
	fail = false
	requireResultError(t, f, machine.ID, stream.ID, result, nil)
	requireSettlementTerminal(t, f, command.id, "done")
	requirePartOutboxWakes(t, f, head, machine.ID, 1)
	head = f.head(t)
	requireResultError(t, f, machine.ID, stream.ID, result, nil)
	if storetest.Scalar[int64](t, f.obs, `SELECT count(*) FROM fixture_settlement_followups f
		JOIN link_commands c ON c.id = f.command_id
		WHERE c.machine_id = $1 AND c.action = 'restart' AND c.outcome IS NULL AND c.sent_stream_id IS NULL`, machine.ID) != 2 ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_commands WHERE machine_id = $1", machine.ID) != 3 ||
		settlementAuditCount(t, f, command.id) != 1 {
		t.Error("The live settlement did not commit exactly two identified follow-ups and one callback.")
	}
	f.requireSaved(t, head)
}

func settlementFrame(commandID string, attempt uint32, outcome linkv1.CommandOutcome, watermark uint64) *linkv1.CommandResult {
	result := commandResult(commandID, attempt, outcome)
	result.LastEventNumber = watermark
	return result
}

func seedSettlementCommand(t *testing.T, f *presenceFixture, machineID, streamID string, action *linkv1.Command, remaining time.Duration) claimFixtureRow {
	t.Helper()
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	command := seedClaimRow(t, f, machineID, action, now.Add(remaining))
	outboxFixtureExec(t, f.obs, "UPDATE link_commands SET sent_stream_id = $2, dispatched = true WHERE id = $1", command.id, streamID)
	return command
}

func hasSettlementStage(t *testing.T, f *presenceFixture, commandID, outcome string, watermark int64) bool {
	t.Helper()
	return storetest.Scalar[bool](t, f.obs, `SELECT COALESCE(staged_outcome, '') = $2 AND
		COALESCE(staged_event_number, 0) = $3 AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL
		FROM link_commands WHERE id = $1`, commandID, outcome, watermark)
}

func hasOneSettlementWake(t *testing.T, f *presenceFixture, after int64, machineID string) bool {
	t.Helper()
	return storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 1 AND
		COALESCE(bool_and(topic = 'machine_outbox' AND scope = '' AND key = $2 AND change = 'saved'), false)
		FROM feed WHERE position > $1`, after, machineID)
}

func newSettlementAudit(t *testing.T, f *presenceFixture) {
	t.Helper()
	outboxFixtureExec(t, f.obs, `CREATE TABLE fixture_settlements (
		command_id text NOT NULL,
		outcome text NOT NULL,
		watermark bigint,
		event_mark bigint NOT NULL)`)
}

func registerRestartSettlement(t *testing.T, f *presenceFixture, hook func(context.Context, *store.Tx, Settlement) error) {
	t.Helper()
	if err := f.state.RegisterPart(PartHooks{CommandSettled: map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
		CommandKind("restart"): hook,
	}}); err != nil {
		t.Fatal("The restart settlement hook could not be registered.")
	}
}

func recordRestartSettlement(ctx context.Context, tx *store.Tx, event Settlement) error {
	command := event.Command
	if command.GetRestart() == nil || command.GetExpiresAt() == nil || command.GetExpiresAt().CheckValid() != nil {
		return store.NewDomainError("The settlement hook did not receive the stored restart action and expiry.")
	}
	var watermark *int64
	if event.LastEventNumber != nil {
		value := int64(*event.LastEventNumber)
		watermark = &value
	}
	tag, err := tx.Exec(ctx, `INSERT INTO fixture_settlements (command_id, outcome, watermark, event_mark)
		SELECT c.id, c.outcome, $4, m.event_mark
		FROM link_commands c JOIN link_machines m ON c.machine_id = m.machine_id
		WHERE c.id = $1 AND m.machine_id = $2 AND c.outcome = $3
		AND c.attempt = $5 AND c.expires_at = $6
		AND c.finished_at IS NOT NULL AND c.payload IS NULL
		AND c.staged_outcome IS NULL AND c.staged_event_number IS NULL`,
		command.GetId(), event.MachineID, string(event.Outcome), watermark, int64(command.GetAttempt()), command.GetExpiresAt().AsTime())
	if err == nil && tag.RowsAffected() != 1 {
		return store.NewDomainError("The settlement hook did not see the matching terminal command inside its write.")
	}
	return err
}

func settlementAuditCount(t *testing.T, f *presenceFixture, commandID string) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM fixture_settlements WHERE command_id = $1", commandID)
}

func requireSettlementAudit(t *testing.T, f *presenceFixture, commandID string, outcome SettlementOutcome, watermark *int64, mark int64) {
	t.Helper()
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 1 FROM fixture_settlements
		WHERE command_id = $1 AND outcome = $2 AND watermark IS NOT DISTINCT FROM $3 AND event_mark = $4`,
		commandID, string(outcome), watermark, mark) {
		t.Errorf("The settlement callback did not commit one %s observation with the expected watermark and mark.", outcome)
	}
}

func requireSettlementTerminal(t *testing.T, f *presenceFixture, commandID, outcome string) time.Time {
	t.Helper()
	var finished *time.Time
	err := f.obs.QueryRow(t.Context(), `SELECT finished_at FROM link_commands WHERE id = $1 AND outcome = $2
		AND payload IS NULL AND staged_outcome IS NULL AND staged_event_number IS NULL`, commandID, outcome).Scan(&finished)
	if err != nil || finished == nil {
		t.Errorf("The command did not finish as %s with cleared action payload and stage.", outcome)
		return time.Time{}
	}
	return *finished
}
