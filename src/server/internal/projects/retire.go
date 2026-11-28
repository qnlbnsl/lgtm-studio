package projects

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const (
	retirementBatchRows = 512
	retirementInterval  = time.Hour
)

var errMissingFolderReceipt = store.NewDomainError("A terminal folder operation is missing its held receipt.")

type retirementCandidate struct {
	id        string
	key       string
	startedAt time.Time
}

// RunRetirement sweeps at startup and one hour after each completed sweep until ctx ends.
func (s *Service) RunRetirement(ctx context.Context, report func(error)) {
	for ctx.Err() == nil {
		if err := s.sweepRetirement(ctx, report); err != nil && ctx.Err() == nil && report != nil {
			report(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retirementInterval):
		}
	}
}

func (s *Service) sweepRetirement(ctx context.Context, report func(error)) error {
	if err := s.stampTerminalOperations(ctx); err != nil {
		return err
	}
	var cutoff time.Time
	if err := s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		return tx.QueryRow(ctx, "SELECT clock_timestamp() - interval '24 hours'").Scan(&cutoff)
	}); err != nil {
		return err
	}

	var afterTime *time.Time
	var afterID string
	for {
		page, err := s.retirementCandidates(ctx, cutoff, afterTime, afterID)
		if err != nil {
			return err
		}
		for _, candidate := range page {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := s.retireOperation(ctx, candidate, cutoff); err != nil && ctx.Err() == nil && report != nil {
				report(err)
			}
		}
		if len(page) < retirementBatchRows {
			return nil
		}
		last := &page[len(page)-1]
		afterTime, afterID = &last.startedAt, last.id
	}
}

func (s *Service) stampTerminalOperations(ctx context.Context) error {
	var afterID string
	for {
		var count int
		err := s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
			// Lock each candidate before recording the first observation of its committed result.
			return tx.QueryRow(ctx, `
WITH candidates AS (
    SELECT id FROM folder_operations
    WHERE id > $1 AND state <> 'open' AND retention_started_at IS NULL
    ORDER BY id LIMIT $2
    FOR UPDATE
), stamped AS (
    UPDATE folder_operations o SET retention_started_at = clock_timestamp()
    FROM candidates c WHERE o.id = c.id
    RETURNING o.id
)
SELECT COALESCE(max(id), $1), count(*) FROM stamped`, afterID, retirementBatchRows).Scan(&afterID, &count)
		})
		if err != nil || count < retirementBatchRows {
			return err
		}
	}
}

func (s *Service) retirementCandidates(ctx context.Context, cutoff time.Time, afterTime *time.Time, afterID string) ([]retirementCandidate, error) {
	var page []retirementCandidate
	err := s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		rows, err := tx.Query(ctx, `
SELECT id, request_key, retention_started_at FROM folder_operations
WHERE state <> 'open' AND retention_started_at <= $1
  AND (retention_started_at, id) > (COALESCE($2::timestamptz, '-infinity'), $3)
ORDER BY retention_started_at, id LIMIT $4`, cutoff, afterTime, afterID, retirementBatchRows)
		if err != nil {
			return err
		}
		page, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (retirementCandidate, error) {
			var candidate retirementCandidate
			err := row.Scan(&candidate.id, &candidate.key, &candidate.startedAt)
			return candidate, err
		})
		return err
	})
	return page, err
}

func (s *Service) retireOperation(ctx context.Context, candidate retirementCandidate, cutoff time.Time) error {
	var matched bool
	_, err := s.store.RetireReceipt(ctx, "owner", candidate.key, []byte(candidate.id),
		func(ctx context.Context, tx *store.Tx) (bool, error) {
			matched = true
			tag, err := tx.Exec(ctx, `
DELETE FROM folder_operations
WHERE id = $1 AND request_key = $2 AND state <> 'open' AND retention_started_at <= $3`,
				candidate.id, candidate.key, cutoff)
			return tag.RowsAffected() == 1, err
		})
	if err != nil || matched {
		return err
	}

	// A competing worker may have removed the pair while this worker waited for the lock.
	return s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT FROM folder_operations WHERE id = $1)", candidate.id).
			Scan(&exists); err != nil {
			return err
		}
		if exists {
			return errMissingFolderReceipt
		}
		return nil
	})
}
