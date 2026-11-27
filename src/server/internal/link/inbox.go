package link

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

var (
	ErrEventNumberZero = store.NewDomainError("The event number is outside the allowed range.")
	ErrEventGap        = store.NewDomainError("The event leaves a gap after the server's mark.")
)

type EventResult struct {
	Mark uint64
	// Applied reports a committed advance, including an event consumed without a report effect.
	Applied bool
}

func (s *State) ApplyEvent(ctx context.Context, machineID, streamID string, event *linkv1.Event) (EventResult, error) {
	return s.applyEvent(ctx, machineID, streamID, event, nil)
}

func (s *State) applyEvent(ctx context.Context, machineID, streamID string, event *linkv1.Event, beforeCommit func(context.Context, string, uint64) error) (EventResult, error) {
	var result EventResult
	err := s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		machine, err := s.registry.Lock(ctx, tx, machineID)
		if err == machines.ErrNoMachine {
			return ErrUnknownMachine
		}
		if err != nil {
			return err
		}
		s.locked(ctx, machineID)

		var mark uint64
		err = tx.QueryRow(ctx, `
SELECT m.event_mark FROM link_machines m JOIN link_connections c USING (machine_id)
WHERE m.machine_id = $1 AND c.stream_id = $2`, machineID, streamID).Scan(&mark)
		if err == pgx.ErrNoRows {
			return ErrNotCurrent
		}
		if err != nil {
			return err
		}
		number := event.GetNumber()
		switch {
		case number == 0 || number > math.MaxInt64:
			return ErrEventNumberZero
		case number <= mark:
			result.Mark = mark
			return nil
		case number != mark+1:
			return ErrEventGap
		}

		if err := s.applyReport(ctx, tx, machine, event); err != nil {
			return err
		}
		if beforeCommit != nil {
			if err := beforeCommit(ctx, machineID, number); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE link_machines SET event_mark = $2 WHERE machine_id = $1`, machineID, int64(number)); err != nil {
			return err
		}
		result = EventResult{Mark: number, Applied: true}
		return s.registry.Touch(ctx, tx, machineID)
	})
	if err != nil {
		return EventResult{}, err
	}
	return result, nil
}

func (s *State) applyReport(ctx context.Context, tx *store.Tx, machine machines.Machine, event *linkv1.Event) error {
	approved := machine.Approval == machines.ApprovalApproved
	facts, harnesses := event.GetFacts(), event.GetHarnessesFound()
	var factsReceived, harnessesReceived bool
	switch {
	case validFacts(facts):
		if err := s.registry.ApplyFacts(ctx, tx, machine.ID, facts); err != nil {
			return err
		}
		factsReceived = approved
	case approved && validHarnesses(harnesses):
		if err := s.registry.ApplyHarnesses(ctx, tx, machine.ID, harnesses); err != nil {
			return err
		}
		harnessesReceived = true
	default:
		return nil
	}
	_, err := tx.Exec(ctx, `
UPDATE link_connections SET last_heard_at = clock_timestamp(),
    facts_received = facts_received OR $2, harnesses_received = harnesses_received OR $3
WHERE machine_id = $1`, machine.ID, factsReceived, harnessesReceived)
	return err
}

func validFacts(facts *linkv1.Facts) bool {
	return facts.GetCores() >= 1 && validReportText(facts.GetOs(), 64) &&
		validReportText(facts.GetClientVersion(), 64) && validReportText(facts.GetWorkspace(), 256)
}

func validHarnesses(report *linkv1.HarnessesFound) bool {
	if report == nil || len(report.Harnesses) > 16 {
		return false
	}
	for _, harness := range report.Harnesses {
		switch harness.GetName() {
		case "claude", "codex", "gemini", "cursor":
		default:
			return false
		}
		if !validReportText(harness.GetVersion(), 64) {
			return false
		}
	}
	return true
}

func validReportText(text string, limit int) bool {
	return len(text) <= limit && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
}
