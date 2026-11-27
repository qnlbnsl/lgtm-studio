package link

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// AcceptResult records the first result for a dispatched attempt on the current stream.
func (s *State) AcceptResult(ctx context.Context, machineID, streamID string, result *linkv1.CommandResult) error {
	return s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		machine, err := s.lockCommandStream(ctx, tx, machineID, streamID)
		if err != nil {
			return err
		}
		if machine.Approval != machines.ApprovalApproved {
			return nil
		}
		id, attempt := result.GetCommandId(), result.GetAttempt()
		if !canonicalUUID.MatchString(id) || attempt == 0 {
			return ErrInvalidMessage
		}
		_, err = tx.Exec(ctx, `
UPDATE link_commands
SET outcome = CASE WHEN action = 'set_state' AND $5::text = 'expired' THEN 'failed' ELSE $5 END,
    finished_at = clock_timestamp(),
    payload = NULL
WHERE id = $1
  AND machine_id = $2
  AND sent_stream_id = $3
  AND attempt = $4
  AND dispatched
  AND outcome IS NULL`, id, machineID, streamID, int64(attempt), commandOutcome(result.GetOutcome()))
		return err
	})
}

// lockCommandStream checks the stream while holding the machine lock for the write.
func (s *State) lockCommandStream(ctx context.Context, tx *store.Tx, machineID, streamID string) (machines.Machine, error) {
	machine, err := s.registry.Lock(ctx, tx, machineID)
	if err == machines.ErrNoMachine {
		return machines.Machine{}, ErrUnknownMachine
	}
	if err != nil {
		return machines.Machine{}, err
	}
	s.locked(ctx, machineID)
	if err := checkCurrentStream(ctx, tx, machineID, streamID); err != nil {
		return machines.Machine{}, err
	}
	return machine, nil
}

func commandOutcome(outcome linkv1.CommandOutcome) string {
	switch outcome {
	case linkv1.CommandOutcome_COMMAND_OUTCOME_DONE:
		return "done"
	case linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED:
		return "expired"
	case linkv1.CommandOutcome_COMMAND_OUTCOME_UNSUPPORTED:
		return "unsupported"
	case linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED:
		return "interrupted"
	default:
		return "failed"
	}
}
