package link

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

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
		id, attempt, watermark := result.GetCommandId(), result.GetAttempt(), result.GetLastEventNumber()
		if !canonicalUUID.MatchString(id) || attempt == 0 || watermark > math.MaxInt64 {
			return ErrInvalidMessage
		}
		commands, err := readCommandsToSettle(ctx, tx, `
WHERE id = $1
  AND machine_id = $2
  AND sent_stream_id = $3
  AND attempt = $4
  AND dispatched
  AND outcome IS NULL
  AND staged_outcome IS NULL`, id, machineID, streamID, int64(attempt))
		if err != nil || len(commands) == 0 {
			return err
		}
		command := commands[0]
		outcome := commandOutcome(result.GetOutcome())
		if command.kind == "set_state" && outcome == OutcomeExpired {
			outcome = OutcomeFailed
		}
		var mark uint64
		if err := tx.QueryRow(ctx, `SELECT event_mark FROM link_machines WHERE machine_id = $1`, machineID).Scan(&mark); err != nil {
			return err
		}
		if watermark <= mark {
			return s.finishCommand(ctx, tx, machineID, command, outcome, &watermark, streamID)
		}
		if _, err := tx.Exec(ctx, `
UPDATE link_commands
SET staged_outcome = $2, staged_event_number = $3
WHERE id = $1`, id, outcome, int64(watermark)); err != nil {
			return err
		}
		return tx.RecordOnce(store.ChangeRef{Topic: outboxTopic, Key: machineID, Change: store.Saved})
	})
}

type commandToSettle struct {
	id              string
	kind            CommandKind
	payload         []byte
	attempt         int64
	expiresAt       time.Time
	stagedOutcome   SettlementOutcome
	stagedWatermark *uint64
}

// readCommandsToSettle closes the query before a settlement hook can use the transaction.
func readCommandsToSettle(ctx context.Context, tx *store.Tx, selection string, args ...any) ([]commandToSettle, error) {
	rows, err := tx.Query(ctx, `
SELECT id, action, payload, attempt, expires_at, coalesce(staged_outcome, ''), staged_event_number
FROM link_commands
`+selection, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (commandToSettle, error) {
		var command commandToSettle
		err := row.Scan(&command.id, &command.kind, &command.payload, &command.attempt, &command.expiresAt,
			&command.stagedOutcome, &command.stagedWatermark)
		return command, err
	})
}

// finishCommand writes the terminal row before dispatching its hook under the machine lock.
func (s *State) finishCommand(ctx context.Context, tx *store.Tx, machineID string, pending commandToSettle, outcome SettlementOutcome, watermark *uint64, resultStream string) error {
	query := `
UPDATE link_commands
SET outcome = $2, finished_at = clock_timestamp(), payload = NULL,
    staged_outcome = NULL, staged_event_number = NULL
WHERE id = $1 AND outcome IS NULL`
	args := []any{pending.id, outcome}
	if resultStream != "" {
		query += `
  AND sent_stream_id = $3 AND attempt = $4 AND dispatched`
		args = append(args, resultStream, pending.attempt)
	}
	tag, err := tx.Exec(ctx, query, args...)
	if err != nil || tag.RowsAffected() == 0 || s.commandSettled[pending.kind] == nil {
		return err
	}
	command := new(linkv1.Command)
	if err := (proto.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(pending.payload, command); err != nil || command.GetAction() == nil {
		return errStoredCommand
	}
	command.Id = pending.id
	command.Attempt = uint32(pending.attempt)
	command.ExpiresAt = timestamppb.New(pending.expiresAt)
	return s.dispatchSettlement(ctx, tx, pending.kind, Settlement{
		MachineID:       machineID,
		Command:         command,
		Outcome:         outcome,
		LastEventNumber: watermark,
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

func commandOutcome(outcome linkv1.CommandOutcome) SettlementOutcome {
	switch outcome {
	case linkv1.CommandOutcome_COMMAND_OUTCOME_DONE:
		return OutcomeDone
	case linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED:
		return OutcomeExpired
	case linkv1.CommandOutcome_COMMAND_OUTCOME_UNSUPPORTED:
		return OutcomeUnsupported
	case linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED:
		return OutcomeInterrupted
	default:
		return OutcomeFailed
	}
}
