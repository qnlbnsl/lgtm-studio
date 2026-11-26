package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	retention      = 24 * time.Hour
	pruneInterval  = time.Hour
	pruneBatchRows = 512
)

// errFeedPrefixBlocked reports that a retained feed entry keeps an entry older than twice the retention.
var errFeedPrefixBlocked = NewDomainError("A retained feed entry prevents the removal of older feed entries.")

// PruneResult counts the committed deletions of one sweep.
type PruneResult struct {
	FeedRows    int64
	ReceiptRows int64
}

// Prune removes feed entries and receipts observed more than 24 hours before the database clock.
// It returns the counts of committed deletions. When a retained feed entry blocks the removal
// of an entry older than twice the retention, it also returns a diagnostic error with those counts.
func (s *Store) Prune(ctx context.Context) (PruneResult, error) {
	var cutoff time.Time
	err := s.withMaintenance(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT clock_timestamp() - interval '24 hours'").Scan(&cutoff)
	})
	if err != nil {
		return PruneResult{}, err
	}
	return s.pruneBefore(ctx, cutoff)
}

// RunPruner sweeps at once and then one hour after each completed sweep, until ctx ends.
// It passes each sweep error and diagnostic to report.
func (s *Store) RunPruner(ctx context.Context, report func(error)) {
	s.runPruner(ctx, time.After, report)
}

func (s *Store) runPruner(ctx context.Context, after func(time.Duration) <-chan time.Time, report func(error)) {
	for {
		if _, err := s.Prune(ctx); err != nil && ctx.Err() == nil && report != nil {
			report(err)
		}
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-after(pruneInterval):
		}
	}
}

// pruneBefore stamps newly visible rows, then deletes rows whose observation precedes cutoff.
// Every batch commits by itself. The feed phases finish before the receipt phases start.
func (s *Store) pruneBefore(ctx context.Context, cutoff time.Time) (PruneResult, error) {
	var result PruneResult
	if err := s.stampFeed(ctx); err != nil {
		return result, err
	}
	feedRows, blocked, err := s.deleteFeed(ctx, cutoff)
	result.FeedRows = feedRows
	if err != nil {
		return result, err
	}
	if err := s.stampReceipts(ctx); err != nil {
		return result, err
	}
	result.ReceiptRows, err = s.deleteReceipts(ctx, cutoff)
	if err != nil {
		return result, err
	}
	if blocked {
		return result, errFeedPrefixBlocked
	}
	return result, nil
}

// withMaintenance runs one pruning batch with the write limits but without domain-writer admission.
func (s *Store) withMaintenance(ctx context.Context, run func(context.Context, pgx.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return withTransaction(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite},
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '15s'; SET LOCAL lock_timeout = '10s'"); err != nil {
				return err
			}
			return run(ctx, tx)
		})
}

// lockFeedRetention serializes feed batches on the retention singleton and permits feed updates and deletes.
func lockFeedRetention(ctx context.Context, tx pgx.Tx) (int64, error) {
	var floor int64
	err := tx.QueryRow(ctx, "SELECT pruned_through FROM feed_retention WHERE singleton FOR UPDATE").Scan(&floor)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, "SELECT set_config('lgtm.feed_prune', 'on', true)")
	return floor, err
}

// stampFeed records when each visible feed row was first observed, in ascending positions.
func (s *Store) stampFeed(ctx context.Context) error {
	var after int64
	for {
		var count int
		err := s.withMaintenance(ctx, func(ctx context.Context, tx pgx.Tx) error {
			floor, err := lockFeedRetention(ctx, tx)
			if err != nil {
				return err
			}
			rows, err := tx.Query(ctx, `
SELECT position FROM feed
WHERE position > $1 AND retention_started_at IS NULL
ORDER BY position LIMIT $2
FOR UPDATE`, max(after, floor), pruneBatchRows)
			if err != nil {
				return err
			}
			positions, err := pgx.CollectRows(rows, pgx.RowTo[int64])
			if err != nil || len(positions) == 0 {
				return err
			}
			// The observation time is sampled after the row locks are held.
			if _, err := tx.Exec(ctx, `
UPDATE feed SET retention_started_at = clock_timestamp() WHERE position = ANY ($1)`, positions); err != nil {
				return err
			}
			count, after = len(positions), positions[len(positions)-1]
			return nil
		})
		if err != nil || count < pruneBatchRows {
			return err
		}
	}
}

// deleteFeed deletes the expired prefix of the feed and advances the watermark in the same transaction.
// It stops at the first retained row and reports whether that row keeps a much older row in place.
func (s *Store) deleteFeed(ctx context.Context, cutoff time.Time) (int64, bool, error) {
	var deleted int64
	for {
		var done, blocked bool
		var count int64
		err := s.withMaintenance(ctx, func(ctx context.Context, tx pgx.Tx) error {
			floor, err := lockFeedRetention(ctx, tx)
			if err != nil {
				return err
			}
			rows, err := tx.Query(ctx, `
SELECT position, coalesce(retention_started_at < $2, false) FROM feed
WHERE position > $1 ORDER BY position LIMIT $3`, floor, cutoff, pruneBatchRows)
			if err != nil {
				return err
			}
			var through int64
			var page, prefix int
			retained := false
			for rows.Next() {
				var position int64
				var expired bool
				if err := rows.Scan(&position, &expired); err != nil {
					rows.Close()
					return err
				}
				page++
				if retained {
					continue
				}
				if !expired {
					retained = true
					continue
				}
				prefix++
				through = position
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			done = retained || page < pruneBatchRows

			if retained {
				if err := tx.QueryRow(ctx, `
SELECT EXISTS (SELECT FROM feed WHERE position > $1 AND retention_started_at < $2)`,
					max(through, floor), cutoff.Add(-retention)).Scan(&blocked); err != nil {
					return err
				}
			}
			if prefix == 0 {
				return nil
			}
			if _, err := tx.Exec(ctx, `
UPDATE feed_retention SET pruned_through = greatest(pruned_through, $1) WHERE singleton`, through); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, "DELETE FROM feed WHERE position > $1 AND position <= $2", floor, through)
			if err != nil {
				return err
			}
			count = tag.RowsAffected()
			return nil
		})
		if err != nil {
			return deleted, false, err
		}
		deleted += count
		if done {
			return deleted, blocked, nil
		}
	}
}

// stampReceipts records when each visible receipt was first observed.
func (s *Store) stampReceipts(ctx context.Context) error {
	for {
		var count int
		err := s.withMaintenance(ctx, func(ctx context.Context, tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
SELECT principal_hash, request_key FROM receipts
WHERE retention_started_at IS NULL
ORDER BY retention_started_at NULLS FIRST, principal_hash, request_key
LIMIT $1
FOR UPDATE`, pruneBatchRows)
			if err != nil {
				return err
			}
			var principals [][]byte
			var keys []string
			for rows.Next() {
				var principal []byte
				var key string
				if err := rows.Scan(&principal, &key); err != nil {
					rows.Close()
					return err
				}
				principals, keys = append(principals, principal), append(keys, key)
			}
			rows.Close()
			if err := rows.Err(); err != nil || len(keys) == 0 {
				return err
			}
			// The observation time is sampled after the row locks are held.
			if _, err := tx.Exec(ctx, `
UPDATE receipts r SET retention_started_at = clock_timestamp()
FROM unnest($1::bytea[], $2::text[]) AS k (principal_hash, request_key)
WHERE r.principal_hash = k.principal_hash AND r.request_key = k.request_key`, principals, keys); err != nil {
				return err
			}
			count = len(keys)
			return nil
		})
		if err != nil || count < pruneBatchRows {
			return err
		}
	}
}

// deleteReceipts deletes receipts observed before cutoff in index order.
// A replay that holds a receipt's row lock first completes with the saved reply.
func (s *Store) deleteReceipts(ctx context.Context, cutoff time.Time) (int64, error) {
	var deleted int64
	for {
		var count int64
		err := s.withMaintenance(ctx, func(ctx context.Context, tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, `
WITH expired AS (
    SELECT principal_hash, request_key FROM receipts
    WHERE retention_started_at < $1
    ORDER BY retention_started_at NULLS FIRST, principal_hash, request_key
    LIMIT $2
    FOR UPDATE
)
DELETE FROM receipts r USING expired
WHERE r.principal_hash = expired.principal_hash AND r.request_key = expired.request_key`, cutoff, pruneBatchRows)
			count = tag.RowsAffected()
			return err
		})
		if err != nil {
			return deleted, err
		}
		deleted += count
		if count < pruneBatchRows {
			return deleted, nil
		}
	}
}
