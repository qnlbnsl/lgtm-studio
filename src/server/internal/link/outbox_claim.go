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

const (
	claimPageSize     = 32
	commandRetryGrace = 23 * time.Hour
)

type commandCursor struct {
	lastSeq      int64
	approved     bool
	scanHead     string
	deferredScan int64
}

type commandMetadata struct {
	id, action, streamID string
	seq, attempt         int64
	expiresAt            time.Time
}

var errStoredCommand = store.NewDomainError("The stored command payload is invalid.")

func (s *State) claimNext(ctx context.Context, machineID, streamID string, cursor *commandCursor) (*linkv1.Command, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	next := *cursor
	var command *linkv1.Command
	var more bool
	err := s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		machine, err := s.lockCommandStream(ctx, tx, machineID, streamID)
		if err != nil {
			return err
		}
		if machine.Approval == machines.ApprovalApproved && !next.approved {
			next = commandCursor{approved: true}
		}

		var now time.Time
		if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
			return err
		}
		cutoff := now.Add(-commandRetryGrace)
		if err := next.revisitScans(ctx, tx, machineID, cutoff); err != nil {
			return err
		}
		page, err := readClaimPage(ctx, tx, machineID, next.lastSeq)
		if err != nil {
			return err
		}
		more = len(page) == claimPageSize
		for _, row := range page {
			next.lastSeq = row.seq
			if row.action != "set_state" && (!next.approved || !row.expiresAt.After(cutoff)) {
				continue
			}
			if row.action == "scan_harnesses" && next.deferScan(row) {
				continue
			}
			if row.streamID == streamID || row.attempt == math.MaxUint32 {
				continue
			}
			command, err = assignCommand(ctx, tx, row, streamID)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	*cursor = next
	return command, more, nil
}

func readClaimPage(ctx context.Context, tx *store.Tx, machineID string, after int64) ([]commandMetadata, error) {
	rows, err := tx.Query(ctx, `
SELECT id, action, coalesce(sent_stream_id, ''), seq, attempt, expires_at
FROM link_commands
WHERE machine_id = $1 AND outcome IS NULL AND seq > $2
ORDER BY seq LIMIT $3`, machineID, after, claimPageSize)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (commandMetadata, error) {
		var entry commandMetadata
		err := row.Scan(&entry.id, &entry.action, &entry.streamID, &entry.seq, &entry.attempt, &entry.expiresAt)
		return entry, err
	})
}

func (cursor *commandCursor) revisitScans(ctx context.Context, tx *store.Tx, machineID string, cutoff time.Time) error {
	if cursor.scanHead == "" {
		return nil
	}
	var pending bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT FROM link_commands
    WHERE id = $1 AND machine_id = $2 AND outcome IS NULL AND expires_at > $3
)`, cursor.scanHead, machineID, cutoff).Scan(&pending)
	if err != nil || pending {
		return err
	}
	if cursor.deferredScan != 0 {
		cursor.lastSeq = cursor.deferredScan - 1
	}
	cursor.scanHead = ""
	cursor.deferredScan = 0
	return nil
}

// deferScan reserves the scan slot before checking the stream assignment.
func (cursor *commandCursor) deferScan(row commandMetadata) bool {
	if cursor.scanHead == "" {
		cursor.scanHead = row.id
		return false
	}
	if cursor.scanHead == row.id {
		return false
	}
	if cursor.deferredScan == 0 {
		cursor.deferredScan = row.seq
	}
	return true
}

func assignCommand(ctx context.Context, tx *store.Tx, row commandMetadata, streamID string) (*linkv1.Command, error) {
	attempt := row.attempt
	if row.streamID != "" {
		attempt++
	}
	var payload []byte
	err := tx.QueryRow(ctx, `
UPDATE link_commands
SET attempt = $2, sent_stream_id = $3, dispatched = false
WHERE id = $1
RETURNING payload`, row.id, attempt, streamID).Scan(&payload)
	if err != nil {
		return nil, err
	}
	command := new(linkv1.Command)
	if err := (proto.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(payload, command); err != nil || command.GetAction() == nil {
		return nil, errStoredCommand
	}
	command.Id = row.id
	command.ExpiresAt = timestamppb.New(row.expiresAt)
	command.Attempt = uint32(attempt)
	return command, nil
}
