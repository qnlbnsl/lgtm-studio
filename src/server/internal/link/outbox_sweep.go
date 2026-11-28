package link

import (
	"cmp"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const commandSweepPageSize = 64

type commandSweepTicksKey struct{}

// WithCommandSweepTicks lets application tests drive the periodic command sweep.
func WithCommandSweepTicks(ctx context.Context, ticks <-chan time.Time) context.Context {
	return context.WithValue(ctx, commandSweepTicksKey{}, ticks)
}

var (
	errCommandSweep     = errors.New("The command sweep could not finish.")
	errCommandSweepBusy = store.NewDomainError("The machine is busy.")
)

type commandSweepKey struct {
	dueAt     pgtype.Timestamptz
	machineID string
	seq       int64
}

const expiredCommandPage = `
SELECT expires_at, machine_id, seq
FROM link_commands
WHERE outcome IS NULL AND staged_outcome IS NULL AND action <> 'set_state' AND expires_at < $1
  AND (expires_at, machine_id, seq) > ($2, $3, $4)
ORDER BY expires_at, machine_id, seq LIMIT $5`

const finishedCommandPage = `
SELECT finished_at, machine_id, seq
FROM link_commands
WHERE outcome IS NOT NULL AND finished_at < $1
  AND (finished_at, machine_id, seq) > ($2, $3, $4)
ORDER BY finished_at, machine_id, seq LIMIT $5`

// SweepCommands expires due actions and prunes old terminal results.
func (s *State) SweepCommands(ctx context.Context) error {
	var now time.Time
	if err := s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		return tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now)
	}); err != nil {
		return err
	}

	busy := make(map[string]bool)
	timeoutErr := s.sweepCommandPages(ctx, expiredCommandPage, now.Add(-CommandGrace), false, busy)
	retentionErr := s.sweepCommandPages(ctx, finishedCommandPage, now.Add(-CommandRetention), true, busy)
	return cmp.Or(timeoutErr, retentionErr)
}

func (s *State) sweepCommandPages(ctx context.Context, query string, cutoff time.Time, prune bool, busy map[string]bool) error {
	after := commandSweepKey{dueAt: pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true}}
	failedTimeouts := make(map[string]bool)
	var firstTimeoutErr error
	for ctx.Err() == nil {
		var page []commandSweepKey
		err := s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
			rows, err := tx.Query(ctx, query, cutoff, after.dueAt, after.machineID, after.seq, commandSweepPageSize)
			if err != nil {
				return err
			}
			page, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (commandSweepKey, error) {
				var key commandSweepKey
				err := row.Scan(&key.dueAt, &key.machineID, &key.seq)
				return key, err
			})
			return err
		})
		if err != nil {
			return cmp.Or(firstTimeoutErr, err)
		}

		groups := make(map[string][]int64)
		var machinesInOrder []string
		for _, ref := range page {
			if busy[ref.machineID] || failedTimeouts[ref.machineID] {
				continue
			}
			if groups[ref.machineID] == nil {
				machinesInOrder = append(machinesInOrder, ref.machineID)
			}
			groups[ref.machineID] = append(groups[ref.machineID], ref.seq)
		}
		for _, machineID := range machinesInOrder {
			if err := ctx.Err(); err != nil {
				return cmp.Or(firstTimeoutErr, err)
			}
			err := s.sweepCommandBatch(ctx, machineID, groups[machineID], cutoff, prune)
			switch err {
			case nil:
			case errCommandSweepBusy:
				busy[machineID] = true
			default:
				if prune {
					return err
				}
				// Retry this machine's timeouts on the next sweep.
				failedTimeouts[machineID] = true
				firstTimeoutErr = cmp.Or(firstTimeoutErr, err)
			}
		}
		if len(page) < commandSweepPageSize {
			break
		}
		after = page[len(page)-1]
	}
	return cmp.Or(firstTimeoutErr, ctx.Err())
}

func (s *State) sweepCommandBatch(ctx context.Context, machineID string, sequences []int64, cutoff time.Time, prune bool) error {
	return s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		if _, err := s.registry.TryLock(ctx, tx, machineID); err != nil {
			var dbErr *store.Error
			switch {
			case errors.As(err, &dbErr) && dbErr.SQLState == "55P03":
				return errCommandSweepBusy
			case err == machines.ErrNoMachine:
				if !prune {
					return nil
				}
			default:
				return err
			}
		}
		if prune {
			_, err := tx.Exec(ctx, `
DELETE FROM link_commands
WHERE machine_id = $1 AND seq = ANY ($2) AND outcome IS NOT NULL AND finished_at < $3`, machineID, sequences, cutoff)
			return err
		}
		commands, err := readCommandsToSettle(ctx, tx, `
WHERE machine_id = $1 AND seq = ANY ($2)
  AND outcome IS NULL AND staged_outcome IS NULL AND action <> 'set_state' AND expires_at < $3
ORDER BY expires_at, seq`, machineID, sequences, cutoff)
		if err != nil {
			return err
		}
		for _, command := range commands {
			if err := s.finishCommand(ctx, tx, machineID, command, OutcomeTimedOut, nil, ""); err != nil {
				return err
			}
		}
		return nil
	})
}

// RunCommandSweep sweeps at startup and hourly until the context ends.
func (s *State) RunCommandSweep(ctx context.Context, report func(error)) {
	ticks, _ := ctx.Value(commandSweepTicksKey{}).(<-chan time.Time)
	if ticks == nil {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		ticks = ticker.C
	}
	for ctx.Err() == nil {
		if err := s.SweepCommands(ctx); err != nil && ctx.Err() == nil && report != nil {
			report(errCommandSweep)
		}
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
		}
	}
}
