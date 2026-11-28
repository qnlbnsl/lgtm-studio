package link

import (
	"context"
	"math"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const (
	outboxTopic      = "machine_outbox"
	CommandGrace     = 24 * time.Hour
	CommandRetention = 7 * 24 * time.Hour
)

// Enqueue implements the Machines command queue in the caller's machine write.
func (s *State) Enqueue(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) error {
	_, err := s.EnqueueCommand(ctx, tx, machineID, action, ttl)
	return err
}

// EnqueueCommand stores an action and its wake in the caller's write.
// The caller must lock the machine first and keep the returned identifier only on commit.
func (s *State) EnqueueCommand(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", ErrInvalidMessage
	}
	kind, clean, err := commandAction(action)
	if err != nil {
		return "", err
	}
	payload, err := proto.Marshal(clean)
	if err != nil {
		return "", ErrInvalidMessage
	}
	if _, err := s.registry.Get(ctx, tx, machineID); err != nil {
		if err == machines.ErrNoMachine {
			return "", ErrUnknownMachine
		}
		return "", err
	}
	id, err := store.NewID()
	if err != nil {
		return "", err
	}
	lifetime := pgtype.Interval{Microseconds: ttl.Microseconds(), Valid: true}
	var createdAt, expiresAt time.Time
	err = tx.QueryRow(ctx, `
SELECT created_at, created_at + $1::interval
FROM (SELECT clock_timestamp() AS created_at) AS sampled`, lifetime).Scan(&createdAt, &expiresAt)
	if err != nil {
		return "", err
	}
	if !commandWithinBudget(clean, id, expiresAt) {
		return "", ErrInvalidMessage
	}
	_, err = tx.Exec(ctx, `
INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)`, id, machineID, kind, payload, createdAt, expiresAt)
	if err != nil {
		return "", err
	}
	if err := tx.RecordOnce(store.ChangeRef{Topic: outboxTopic, Key: machineID, Change: store.Saved}); err != nil {
		return "", err
	}
	return id, nil
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
	case *linkv1.Command_LookAtFolder:
		if action == nil || action.LookAtFolder == nil {
			break
		}
		operationID := action.LookAtFolder.GetOperationId()
		typedPath := action.LookAtFolder.GetPath()
		if !runUUID.MatchString(operationID) || !validReportText(typedPath, 4096) ||
			!(path.IsAbs(typedPath) || typedPath == "~" || strings.HasPrefix(typedPath, "~/")) {
			break
		}
		return string(CommandLookAtFolder), &linkv1.Command{
			Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
				OperationId: operationID, Path: typedPath,
			}},
		}, nil
	}
	return "", nil, ErrInvalidMessage
}

func commandWithinBudget(command *linkv1.Command, id string, expiresAt time.Time) bool {
	return proto.Size(&linkv1.Command{
		Id:        id,
		Attempt:   math.MaxUint32,
		ExpiresAt: timestamppb.New(expiresAt),
		Action:    command.GetAction(),
	}) <= commandEventLimit
}
