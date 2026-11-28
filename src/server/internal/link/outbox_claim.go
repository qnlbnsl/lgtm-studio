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
	lastSeq        int64
	approved       bool
	scanHead       string
	deferredScan   int64
	folderHead     string
	deferredFolder int64
}

type commandMetadata struct {
	id, action, streamID string
	seq, attempt         int64
	expiresAt            time.Time
	dispatched           bool
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
		ready, err := runReady(ctx, tx, machineID)
		if err != nil || !ready {
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
		if err := next.revisitScans(ctx, tx, machineID, streamID, cutoff); err != nil {
			return err
		}
		if err := next.revisitCommandHead(ctx, tx, machineID, streamID, cutoff, &next.folderHead, &next.deferredFolder); err != nil {
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
			if row.attempt < math.MaxUint32 || (row.streamID == streamID && row.dispatched) {
				switch row.action {
				case "scan_harnesses":
					if next.deferScan(row) {
						continue
					}
				case string(CommandLookAtFolder):
					if deferCommand(row, &next.folderHead, &next.deferredFolder) {
						continue
					}
				}
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
SELECT id, action, coalesce(sent_stream_id, ''), seq, attempt, expires_at, dispatched
FROM link_commands
WHERE machine_id = $1 AND outcome IS NULL AND staged_outcome IS NULL AND seq > $2
ORDER BY seq LIMIT $3`, machineID, after, claimPageSize)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (commandMetadata, error) {
		var entry commandMetadata
		err := row.Scan(&entry.id, &entry.action, &entry.streamID, &entry.seq, &entry.attempt, &entry.expiresAt, &entry.dispatched)
		return entry, err
	})
}

func (cursor *commandCursor) revisitScans(ctx context.Context, tx *store.Tx, machineID, streamID string, cutoff time.Time) error {
	return cursor.revisitCommandHead(ctx, tx, machineID, streamID, cutoff, &cursor.scanHead, &cursor.deferredScan)
}

func (cursor *commandCursor) revisitCommandHead(ctx context.Context, tx *store.Tx, machineID, streamID string, cutoff time.Time, head *string, deferred *int64) error {
	if *head == "" {
		return nil
	}
	var pending bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT FROM link_commands
    WHERE id = $1 AND machine_id = $2 AND outcome IS NULL AND staged_outcome IS NULL AND expires_at > $3
      AND (attempt < $4 OR (sent_stream_id = $5 AND dispatched))
)`, *head, machineID, cutoff, int64(math.MaxUint32), streamID).Scan(&pending)
	if err != nil || pending {
		return err
	}
	if *deferred != 0 {
		cursor.lastSeq = min(cursor.lastSeq, *deferred-1)
	}
	*head = ""
	*deferred = 0
	return nil
}

// deferScan reserves the scan slot before checking the stream assignment.
func (cursor *commandCursor) deferScan(row commandMetadata) bool {
	return deferCommand(row, &cursor.scanHead, &cursor.deferredScan)
}

func deferCommand(row commandMetadata, head *string, deferred *int64) bool {
	if *head == "" {
		*head = row.id
		return false
	}
	if *head == row.id {
		return false
	}
	if *deferred == 0 {
		*deferred = row.seq
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
