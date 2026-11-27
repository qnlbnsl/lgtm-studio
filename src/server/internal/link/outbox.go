package link

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const (
	outboxTopic      = "machine_outbox"
	CommandGrace     = 24 * time.Hour
	CommandRetention = 7 * 24 * time.Hour
)

// Enqueue stores an action and its wake reference in the caller's machine write.
func (*State) Enqueue(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrInvalidMessage
	}
	kind, clean, err := commandAction(action)
	if err != nil {
		return err
	}
	payload, err := proto.Marshal(clean)
	if err != nil {
		return ErrInvalidMessage
	}
	id, err := store.NewID()
	if err != nil {
		return err
	}
	lifetime := pgtype.Interval{Microseconds: ttl.Microseconds(), Valid: true}
	_, err = tx.Exec(ctx, `
INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
SELECT $1, $2, $3, $4, created_at, created_at + $5::interval
FROM (SELECT clock_timestamp() AS created_at) AS sampled`, id, machineID, kind, payload, lifetime)
	if err != nil {
		return err
	}
	return tx.Record(store.ChangeRef{Topic: outboxTopic, Key: machineID, Change: store.Saved})
}

// commandAction rebuilds the payload from declared action fields.
func commandAction(command *linkv1.Command) (string, *linkv1.Command, error) {
	switch action := command.GetAction().(type) {
	case *linkv1.Command_ScanHarnesses:
		if action != nil && action.ScanHarnesses != nil {
			return "scan_harnesses", &linkv1.Command{
				Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}},
			}, nil
		}
	case *linkv1.Command_SetState:
		if action == nil || action.SetState == nil {
			break
		}
		state := action.SetState.GetState()
		if state.GetApproval() != linkv1.Approval_APPROVAL_APPROVED || state.GetRevision() == 0 {
			break
		}
		name, err := machines.NormalizeName(state.GetName())
		if err != nil {
			break
		}
		return "set_state", &linkv1.Command{
			Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{
				State: &linkv1.MachineState{Approval: state.GetApproval(), Name: name, Revision: state.GetRevision()},
			}},
		}, nil
	case *linkv1.Command_Restart:
		if action != nil && action.Restart != nil {
			return "restart", &linkv1.Command{
				Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}},
			}, nil
		}
	}
	return "", nil, ErrInvalidMessage
}
