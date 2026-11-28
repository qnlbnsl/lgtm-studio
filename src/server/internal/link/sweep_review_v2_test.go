package link

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestSweepContinuesAfterTimeoutCallbackFailure(t *testing.T) {
	f := newPresenceFixture(t, nil)
	failing := f.machine(t, bytes.Repeat([]byte{1}, 32), "failing timeout callback")
	healthy := f.machine(t, bytes.Repeat([]byte{2}, 32), "healthy timeout callback")
	f.approve(t, failing.ID)
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.r.SetApproved(ctx, tx, healthy.ID, "Healthy timeout machine")
	}); err != nil {
		t.Fatalf("The fixture could not approve the healthy machine. %v", err)
	}
	failingStream := f.admit(t, failing.ID, 1)
	healthyStream := f.admit(t, healthy.ID, 1)

	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	first := seedSettlementCommand(t, f, failing.ID, failingStream.ID, restartAction(), -49*time.Hour)
	second := seedSettlementCommand(t, f, failing.ID, failingStream.ID, restartAction(), -48*time.Hour)
	other := seedSettlementCommand(t, f, healthy.ID, healthyStream.ID, restartAction(), -47*time.Hour)
	old := seedSweepTerminal(t, f, healthy.ID, healthyStream.ID, now.Add(-10*24*time.Hour), now.Add(-8*24*time.Hour))

	newSettlementAudit(t, f)
	refusal := store.NewDomainError("The fixture refused the second timeout callback.")
	refuse := true
	registerRestartSettlement(t, f, func(ctx context.Context, tx *store.Tx, settlement Settlement) error {
		if err := recordRestartSettlement(ctx, tx, settlement); err != nil {
			return err
		}
		if refuse && settlement.Command.GetId() == second.id {
			return refusal
		}
		return nil
	})

	if err := f.state.SweepCommands(t.Context()); !errors.Is(err, refusal) {
		t.Errorf("The sweep returned %v, want the timeout callback error.", err)
	}
	for _, id := range []string{first.id, second.id} {
		if !hasSettlementStage(t, f, id, "", 0) || settlementAuditCount(t, f, id) != 0 {
			t.Error("The failed machine's batch committed a timeout or a settlement callback write.")
		}
	}
	requireSettlementTerminal(t, f, other.id, "timed_out")
	requireSettlementAudit(t, f, other.id, OutcomeTimedOut, nil, 0)
	if storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM link_commands WHERE id = $1)", old) {
		t.Error("The sweep retained the healthy machine's eight-day-old terminal command after the timeout callback failed.")
	}

	refuse = false
	if err := f.state.SweepCommands(t.Context()); err != nil {
		t.Fatalf("The sweep could not retry the failed batch. %v", err)
	}
	for _, id := range []string{first.id, second.id} {
		requireSettlementTerminal(t, f, id, "timed_out")
		requireSettlementAudit(t, f, id, OutcomeTimedOut, nil, 0)
		if settlementAuditCount(t, f, id) != 1 {
			t.Error("Retrying the failed batch did not commit exactly one callback per command.")
		}
	}
	if settlementAuditCount(t, f, other.id) != 1 {
		t.Error("The healthy command did not have exactly one committed callback after the retry.")
	}
}
