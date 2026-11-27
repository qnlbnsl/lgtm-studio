package link

import (
	"bytes"
	"testing"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestLinkMigrationRejectsStageWithoutStreamAssignment(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "unassigned stage")
	f.approve(t, machine.ID)
	seedLinkMigrationConnection(t, f, machine.ID)
	expiry := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	command := seedClaimRow(t, f, machine.ID, restartAction(), expiry)

	requireLinkMigrationConstraint(t, f.obs, "23514", `UPDATE link_commands
		SET dispatched = true, sent_stream_id = '', staged_outcome = 'done', staged_event_number = 43
		WHERE id = $1`, command.id)
}

func TestLinkMigrationStagedClientOutcomes(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "staged outcomes")
	f.approve(t, machine.ID)
	streamID := seedLinkMigrationConnection(t, f, machine.ID)
	expiry := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name, outcome string
		action        *linkv1.Command
	}{
		{"done", "done", restartAction()},
		{"failed", "failed", restartAction()},
		{"expired", "expired", restartAction()},
		{"unsupported", "unsupported", restartAction()},
		{"interrupted", "interrupted", restartAction()},
		{"standing notice done", "done", standingAction()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := seedClaimRow(t, f, machine.ID, tc.action, expiry)
			tag, err := f.obs.Exec(t.Context(), `UPDATE link_commands
				SET dispatched = true, sent_stream_id = $2, staged_outcome = $3, staged_event_number = 43
				WHERE id = $1`, command.id, streamID, tc.outcome)
			if err != nil || tag.RowsAffected() != 1 {
				t.Fatal("The schema did not accept a valid staged client outcome for a dispatched command.")
			}
			if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM link_commands
				WHERE id = $1 AND staged_outcome = $2 AND staged_event_number = 43
				AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL)`, command.id, tc.outcome) {
				t.Error("The dispatched command did not retain its staged client outcome and watermark.")
			}
		})
	}

	t.Run("standing notice expired", func(t *testing.T) {
		command := seedClaimRow(t, f, machine.ID, standingAction(), expiry)
		outboxFixtureExec(t, f.obs, `UPDATE link_commands SET dispatched = true, sent_stream_id = $2
			WHERE id = $1`, command.id, streamID)
		requireLinkMigrationConstraint(t, f.obs, "23514", `UPDATE link_commands
			SET staged_outcome = 'expired', staged_event_number = 43 WHERE id = $1`, command.id)
	})
}

func TestLinkMigrationSweepSkipsAgedStage(t *testing.T) {
	f := newPresenceFixture(t, nil)
	machine := f.machine(t, bytes.Repeat([]byte{1}, 32), "aged stage")
	f.approve(t, machine.ID)
	streamID := seedLinkMigrationConnection(t, f, machine.ID)
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	expiry := now.Add(-48 * time.Hour)
	staged := seedClaimRow(t, f, machine.ID, restartAction(), expiry)
	ordinary := seedClaimRow(t, f, machine.ID, restartAction(), expiry)
	payload := storetest.Scalar[[]byte](t, f.obs, "SELECT payload FROM link_commands WHERE id = $1", staged.id)
	outboxFixtureExec(t, f.obs, `UPDATE link_commands SET dispatched = true, sent_stream_id = $2,
		staged_outcome = 'interrupted', staged_event_number = 43 WHERE id = $1`, staged.id, streamID)

	// Check the page itself so a later lock-time recheck cannot hide a page regression.
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 1
		AND bool_and(seq = (SELECT seq FROM link_commands WHERE id = $6))
		FROM (`+expiredCommandPage+`) AS candidates`,
		now.Add(-24*time.Hour), time.Time{}, "", int64(0), commandSweepPageSize, ordinary.id) {
		t.Error("The expiry page did not select only the aged ordinary command.")
	}
	if err := f.state.SweepCommands(t.Context()); err != nil {
		t.Fatal("The command sweep failed with an aged staged command beside an aged ordinary command.")
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM link_commands
		WHERE id = $1 AND outcome = 'timed_out' AND finished_at IS NOT NULL AND payload IS NULL)`, ordinary.id) {
		t.Error("The sweep did not time out the aged ordinary command.")
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM link_commands
		WHERE id = $1 AND dispatched AND sent_stream_id = $2 AND payload = $3
		AND staged_outcome = 'interrupted' AND staged_event_number = 43
		AND outcome IS NULL AND finished_at IS NULL)`, staged.id, streamID, payload) {
		t.Error("The sweep did not preserve the staged result, watermark and exact payload of the aged command.")
	}
}
