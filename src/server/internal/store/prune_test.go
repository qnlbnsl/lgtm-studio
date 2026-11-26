package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func pruneFixture(t *testing.T) (*Store, *pgx.Conn, *pgx.Conn, time.Time) {
	t.Helper()
	s, cfg, obs := poolOpen(t, "retention")
	if err := s.Migrate(t.Context()); err != nil {
		t.Fatalf("migrating the retention fixture failed: %v", err)
	}
	storetest.Exec(t, obs, "CREATE TABLE records (id text PRIMARY KEY); INSERT INTO records VALUES ('preserve')")
	return s, obs, poolObserver(t, cfg), storetest.Scalar[time.Time](t, obs, "SELECT clock_timestamp()")
}

func pruneFixtureTx(t *testing.T, conn *pgx.Conn) pgx.Tx {
	t.Helper()
	tx, err := conn.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})
	return tx
}

func pruneSeedFeed(t *testing.T, tx pgx.Tx, count int, key string, written time.Time, observed *time.Time) {
	t.Helper()
	storetest.Exec(t, tx, "SET LOCAL lgtm.feed_flush = 'on'")
	storetest.Exec(t, tx, `
INSERT INTO feed (topic, scope, key, change, written_at, retention_started_at)
SELECT 'records', '', $2, 'saved', $3, $4 FROM generate_series(1, $1::int)`, count, key, written, observed)
}

func pruneSeedReceipts(t *testing.T, conn storetest.Executor, count int, key string, written time.Time, observed *time.Time) {
	t.Helper()
	storetest.Exec(t, conn, `
INSERT INTO receipts (principal_hash, request_key, fingerprint_version, request_hash, reply, written_at, retention_started_at)
SELECT decode(repeat('11', 32), 'hex'), $2 || '-' || n, 1, decode(repeat('22', 32), 'hex'),
       decode('00ff', 'hex'), $3, $4
FROM generate_series(1, $1::int) AS n`, count, key, written, observed)
}

func pruneCommit(t *testing.T, tx pgx.Tx) {
	t.Helper()
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("committing the retention fixture failed: %v", err)
	}
}

func pruneCheckResult(t *testing.T, got PruneResult, err error, feed, receipts int64) {
	t.Helper()
	want := PruneResult{FeedRows: feed, ReceiptRows: receipts}
	if err != nil || got != want {
		t.Fatalf("prune returned %+v and %v, want %+v", got, err, want)
	}
}

func pruneCheckFloor(t *testing.T, obs *pgx.Conn, want int64) {
	t.Helper()
	got := storetest.Scalar[int64](t, obs, "SELECT pruned_through FROM feed_retention WHERE singleton")
	if got != want {
		t.Fatalf("pruning watermark = %d, want %d", got, want)
	}
}

func TestPruneCutoffAndWatermark(t *testing.T) {
	t.Run("multiple pages", func(t *testing.T) {
		s, obs, raw, now := pruneFixture(t)
		cutoff := now.Add(-24 * time.Hour)
		old, fresh := cutoff.Add(-time.Microsecond), cutoff.Add(time.Microsecond)
		tx := pruneFixtureTx(t, raw)
		pruneSeedFeed(t, tx, 1300, "expired", old, &old)
		pruneSeedFeed(t, tx, 1, "cutoff", old, &cutoff)
		pruneSeedFeed(t, tx, 1, "fresh", old, &fresh)
		pruneSeedFeed(t, tx, 1100, "unobserved", old, nil)
		pruneCommit(t, tx)
		pruneSeedReceipts(t, raw, 1300, "expired", old, &old)
		pruneSeedReceipts(t, raw, 1, "cutoff", old, &cutoff)
		pruneSeedReceipts(t, raw, 1, "fresh", old, &fresh)
		pruneSeedReceipts(t, raw, 1100, "unobserved", old, nil)

		got, err := s.pruneBefore(t.Context(), cutoff)
		pruneCheckResult(t, got, err, 1300, 1300)
		pruneCheckFloor(t, obs, 1300)
		keys := storetest.Rows[string](t, obs, "SELECT key FROM feed WHERE key <> 'unobserved' ORDER BY position")
		if !slices.Equal(keys, []string{"cutoff", "fresh"}) {
			t.Fatalf("feed rows near the cutoff = %v, want cutoff and fresh", keys)
		}
		keys = storetest.Rows[string](t, obs, "SELECT request_key FROM receipts WHERE request_key NOT LIKE 'unobserved-%' ORDER BY request_key")
		if !slices.Equal(keys, []string{"cutoff-1", "fresh-1"}) {
			t.Fatalf("receipts near the cutoff = %v, want cutoff-1 and fresh-1", keys)
		}
		for _, query := range []string{
			"SELECT count(*) FROM feed WHERE key = 'unobserved' AND retention_started_at >= $1",
			"SELECT count(*) FROM receipts WHERE request_key LIKE 'unobserved-%' AND retention_started_at >= $1",
		} {
			if stamped := storetest.Scalar[int](t, obs, query, now); stamped != 1100 {
				t.Fatalf("stamped %d of 1100 previously unobserved rows", stamped)
			}
		}
		if record := storetest.Scalar[string](t, obs, "SELECT id FROM records"); record != "preserve" {
			t.Fatalf("pruning changed the part-owned record to %q", record)
		}
		got, err = s.pruneBefore(t.Context(), cutoff)
		pruneCheckResult(t, got, err, 0, 0)
		pruneCheckFloor(t, obs, 1300)
	})

	t.Run("empty feed keeps its sequence", func(t *testing.T) {
		s, obs, raw, now := pruneFixture(t)
		old := now.Add(-25 * time.Hour)
		tx := pruneFixtureTx(t, raw)
		pruneSeedFeed(t, tx, 3, "expired", old, &old)
		pruneCommit(t, tx)
		got, err := s.Prune(t.Context())
		pruneCheckResult(t, got, err, 3, 0)
		pruneCheckFloor(t, obs, 3)
		if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM feed"); count != 0 {
			t.Fatalf("the entire feed was eligible, but %d rows remain", count)
		}
		tx = pruneFixtureTx(t, raw)
		pruneSeedFeed(t, tx, 1, "next", now, nil)
		pruneCommit(t, tx)
		if next := storetest.Scalar[int64](t, obs, "SELECT position FROM feed"); next <= 3 {
			t.Fatalf("the next feed position = %d, want greater than the watermark 3", next)
		}
	})
}

func TestPrunePreservesFeedPrefix(t *testing.T) {
	s, obs, raw, now := pruneFixture(t)
	old, stale, future := now.Add(-25*time.Hour), now.Add(-73*time.Hour), now.Add(24*time.Hour)
	tx := pruneFixtureTx(t, raw)
	pruneSeedFeed(t, tx, 1, "expired", future, &old)
	pruneSeedFeed(t, tx, 1, "retained", now, &now)
	pruneSeedFeed(t, tx, 1, "blocked", stale, &stale)
	pruneCommit(t, tx)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reports := make(chan error, 2)
	task := storetest.Start(t, func() struct{} {
		s.RunPruner(ctx, func(err error) { reports <- err })
		return struct{}{}
	})
	select {
	case err := <-reports:
		if err == nil {
			t.Fatal("the pruner reported no diagnostic for the blocked stale prefix")
		}
	case <-task.Done():
		t.Fatal("the pruner exited without reporting the blocked stale prefix")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("the pruner did not report the blocked stale prefix")
	}
	want := []string{"retained", "blocked"}
	if keys := storetest.Rows[string](t, obs, "SELECT key FROM feed ORDER BY position"); !slices.Equal(keys, want) {
		t.Fatalf("the retained feed suffix = %v, want %v", keys, want)
	}
	pruneCheckFloor(t, obs, 1)
	cancel()
	task.Wait(t)
}

func TestPruneStartsAgeAfterCommit(t *testing.T) {
	s, obs, raw, now := pruneFixture(t)
	cutoff := now.Add(-24 * time.Hour)
	backdated := now.Add(-72 * time.Hour)
	tx := pruneFixtureTx(t, raw)
	pruneSeedFeed(t, tx, 1, "late", backdated, nil)
	pruneSeedReceipts(t, tx, 1, "late", backdated, nil)

	got, err := s.pruneBefore(t.Context(), cutoff)
	pruneCheckResult(t, got, err, 0, 0)
	for _, table := range []string{"feed", "receipts"} {
		if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()); count != 0 {
			t.Fatalf("the independent observer saw %d uncommitted rows in %s", count, table)
		}
		var observed *time.Time
		if err := tx.QueryRow(t.Context(), "SELECT retention_started_at FROM "+pgx.Identifier{table}.Sanitize()).Scan(&observed); err != nil || observed != nil {
			t.Fatalf("the uncommitted row in %s has observation %v and error %v", table, observed, err)
		}
	}

	beforeCommit := storetest.Scalar[time.Time](t, obs, "SELECT clock_timestamp()")
	pruneCommit(t, tx)
	got, err = s.pruneBefore(t.Context(), cutoff)
	pruneCheckResult(t, got, err, 0, 0)
	for _, table := range []string{"feed", "receipts"} {
		observed := storetest.Scalar[*time.Time](t, obs, "SELECT retention_started_at FROM "+pgx.Identifier{table}.Sanitize())
		if observed == nil || observed.Before(beforeCommit) || !observed.After(cutoff) {
			t.Fatalf("%s observation %v predates visibility at %v", table, observed, beforeCommit)
		}
	}
	got, err = s.pruneBefore(t.Context(), cutoff)
	pruneCheckResult(t, got, err, 0, 0)
	if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM receipts"); count != 1 {
		t.Fatalf("pruning removed the newly visible receipt: count = %d", count)
	}
}

func pruneWaitOnHolder(t *testing.T, obs *pgx.Conn, blocker int32, task *storetest.Task[pruneTestResult]) {
	t.Helper()
	storetest.Poll(t, "the sweep waiting at a row lock", func() bool {
		select {
		case <-task.Done():
			t.Fatalf("the sweep returned %+v before waiting for the replay's row lock", task.Result())
		default:
		}
		return storetest.Scalar[bool](t, obs, `
SELECT EXISTS (
    SELECT FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid
    WHERE a.datname = current_database() AND NOT l.granted AND l.locktype = 'transactionid'
      AND $1 = ANY (pg_blocking_pids(a.pid))
)`, blocker)
	})
}

type pruneTestResult struct {
	counts PruneResult
	err    error
}

func TestPruneWaitsForReplayOfSameKey(t *testing.T) {
	s, obs, raw, now := pruneFixture(t)
	cutoff := now.Add(-24 * time.Hour)
	old := cutoff.Add(-time.Hour)
	tx := pruneFixtureTx(t, raw)
	pruneSeedFeed(t, tx, 1, "expired", old, &old)
	pruneCommit(t, tx)
	pruneSeedReceipts(t, raw, 1, "expired", old, &old)
	pruneSeedReceipts(t, raw, 1, "cutoff", old, &cutoff)

	holder := pruneFixtureTx(t, raw)
	defer holder.Rollback(context.Background())
	var pid int32
	if err := holder.QueryRow(t.Context(), `
SELECT pg_backend_pid() FROM receipts WHERE request_key = 'expired-1' FOR UPDATE`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	task := storetest.Start(t, func() pruneTestResult {
		counts, err := s.pruneBefore(t.Context(), cutoff)
		return pruneTestResult{counts, err}
	})
	pruneWaitOnHolder(t, obs, pid, task)
	pruneCheckFloor(t, obs, 1)
	if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM feed"); count != 0 {
		t.Fatalf("feed cleanup has not committed before the receipt lock wait: %d rows remain", count)
	}
	var reply []byte
	if err := holder.QueryRow(t.Context(), "SELECT reply FROM receipts WHERE request_key = 'expired-1'").Scan(&reply); err != nil || !bytes.Equal(reply, []byte{0, 255}) {
		t.Fatalf("the replay holder did not read the original reply: %v", err)
	}
	pruneCommit(t, holder)
	result := task.Wait(t)
	pruneCheckResult(t, result.counts, result.err, 1, 1)
	if key := storetest.Scalar[string](t, obs, "SELECT request_key FROM receipts"); key != "cutoff-1" {
		t.Fatalf("receipt %q survived, want the receipt at the cutoff", key)
	}
	got, err := s.pruneBefore(t.Context(), cutoff.Add(time.Microsecond))
	pruneCheckResult(t, got, err, 0, 1)
	if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM receipts"); count != 0 {
		t.Fatalf("the advanced cutoff left %d receipts", count)
	}
}

type pruneTestTimer struct {
	due  time.Duration
	tick chan time.Time
}

type pruneTestClock struct {
	mu         sync.Mutex
	elapsed    time.Duration
	pending    []pruneTestTimer
	registered chan time.Duration
}

func (c *pruneTestClock) after(delay time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	tick := make(chan time.Time, 1)
	c.pending = append(c.pending, pruneTestTimer{due: c.elapsed + delay, tick: tick})
	c.registered <- delay
	return tick
}

func (c *pruneTestClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.elapsed += by
	remaining := c.pending[:0]
	for _, timer := range c.pending {
		if timer.due <= c.elapsed {
			timer.tick <- time.Unix(0, int64(c.elapsed))
		} else {
			remaining = append(remaining, timer)
		}
	}
	c.pending = remaining
}

func pruneAssertTimer(t *testing.T, clock *pruneTestClock) {
	t.Helper()
	if delay := storetest.Receive(t, "the timer after a completed sweep", clock.registered); delay != time.Hour {
		t.Fatalf("the next sweep is scheduled after %v, want one hour", delay)
	}
}

func pruneAssertNoTimer(t *testing.T, clock *pruneTestClock) {
	t.Helper()
	select {
	case delay := <-clock.registered:
		t.Fatalf("the pruner scheduled a %v timer before the sweep completed", delay)
	default:
	}
}

func TestRunPrunerLifecycle(t *testing.T) {
	for _, failFirst := range []bool{false, true} {
		name := "held sweep"
		if failFirst {
			name = "failed sweep recovers"
		}
		t.Run(name, func(t *testing.T) {
			s, obs, raw, now := pruneFixture(t)
			const gate int64 = 841941100906
			storetest.Exec(t, obs, `
CREATE FUNCTION prune_test_gate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(841941100906::bigint);
    IF NEW.key = 'fail' THEN
        RAISE EXCEPTION 'fixture-private-prune-failure' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER prune_test_gate BEFORE UPDATE ON feed
FOR EACH ROW EXECUTE FUNCTION prune_test_gate();`)
			seed := func(key string) {
				t.Helper()
				tx := pruneFixtureTx(t, raw)
				pruneSeedFeed(t, tx, 1, key, now, nil)
				pruneCommit(t, tx)
			}
			firstKey := "first"
			if failFirst {
				firstKey = "fail"
			}
			seed(firstKey)
			key := storetest.BigintKey(gate)
			release := storetest.HoldAdvisoryGate(t, obs, gate)
			defer release()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			clock := &pruneTestClock{registered: make(chan time.Duration, 16)}
			reports := make(chan error, 16)
			task := storetest.Start(t, func() struct{} {
				s.runPruner(ctx, clock.after, func(err error) { reports <- err })
				return struct{}{}
			})

			// A sweep must enter the database before the fixture clock advances.
			storetest.Poll(t, "the startup sweep entering PostgreSQL", func() bool {
				select {
				case <-task.Done():
					t.Fatal("the pruner returned before its startup sweep reached the gate")
				default:
				}
				pids := poolBlockedPIDs(t, obs, key)
				if len(pids) != 1 {
					return false
				}
				blocked, err := storetest.IsBlocked(t.Context(), obs, pids[0], poolBackendPID(t, obs), key)
				if err != nil {
					t.Fatal(err)
				}
				return blocked
			})
			clock.advance(3 * time.Hour)
			pruneAssertNoTimer(t, clock)
			if got := len(poolBlockedPIDs(t, obs, key)); got != 1 {
				t.Fatalf("found %d sweeps while the first was held, want 1", got)
			}
			release()
			if failFirst {
				err := storetest.Receive(t, "the failed sweep diagnostic", reports)
				var safe *Error
				var rawError *pgconn.PgError
				if !errors.As(err, &safe) || errors.As(err, &rawError) || strings.Contains(err.Error(), "fixture-private-prune-failure") {
					t.Fatalf("the sweep returned an unsanitized diagnostic: %v", err)
				}
				storetest.Exec(t, obs, "DROP TRIGGER prune_test_gate ON feed")
			}
			pruneAssertTimer(t, clock)

			seed("second")
			clock.advance(time.Hour - time.Minute)
			if observed := storetest.Scalar[*time.Time](t, obs, "SELECT retention_started_at FROM feed WHERE key = 'second'"); observed != nil {
				t.Fatal("a sweep started before one hour elapsed after completion")
			}
			pruneAssertNoTimer(t, clock)
			clock.advance(time.Minute)
			pruneAssertTimer(t, clock)
			if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM feed WHERE retention_started_at IS NOT NULL"); count != 2 {
				t.Fatalf("the next clock advance stamped %d rows, want 2", count)
			}
			seed("third")
			cancel()
			task.Wait(t)
			clock.advance(time.Hour)
			if observed := storetest.Scalar[*time.Time](t, obs, "SELECT retention_started_at FROM feed WHERE key = 'third'"); observed != nil {
				t.Fatal("the pruner ran another sweep after cancellation")
			}
			select {
			case err := <-reports:
				t.Fatalf("unexpected extra pruner diagnostic: %v", err)
			default:
			}
		})
	}
}

func TestPruneCountsOnlyCommittedFeedBatches(t *testing.T) {
	s, obs, raw, now := pruneFixture(t)
	cutoff := now.Add(-24 * time.Hour)
	old := cutoff.Add(-time.Hour)
	tx := pruneFixtureTx(t, raw)
	pruneSeedFeed(t, tx, pruneBatchRows+1, "expired", old, &old)
	pruneCommit(t, tx)
	// The second batch deletes only the last position, and its commit rejects that deletion.
	storetest.Exec(t, obs, fmt.Sprintf(`
CREATE FUNCTION prune_test_reject() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.position = %d THEN
        RAISE EXCEPTION 'fixture rejected commit' USING ERRCODE = '23514';
    END IF;
    RETURN OLD;
END;
$$;
CREATE CONSTRAINT TRIGGER prune_test_reject AFTER DELETE ON feed
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION prune_test_reject();`, pruneBatchRows+1))

	got, err := s.pruneBefore(t.Context(), cutoff)
	if err == nil {
		t.Fatal("the sweep succeeded although the commit of its second batch was rejected")
	}
	if got.FeedRows != pruneBatchRows || got.ReceiptRows != 0 {
		t.Fatalf("the sweep reported %+v, want only the %d rows of the committed batch", got, pruneBatchRows)
	}
	pruneCheckFloor(t, obs, pruneBatchRows)
	if remaining := storetest.Scalar[int](t, obs, "SELECT count(*) FROM feed"); remaining != 1 {
		t.Fatalf("%d feed rows remain, want the one of the rolled-back batch", remaining)
	}
}

const (
	pruneReplayGateID int64 = 841941100911
	pruneDeleteGateID int64 = 841941100912
)

// TestPruneAndReplayWaitForReceiptRowLock proves the receipt row lock in both orders through the public WriteOnce.
func TestPruneAndReplayWaitForReceiptRowLock(t *testing.T) {
	setup := func(t *testing.T, tracer *feedTestTracer) (*Store, *pgx.Conn, Request, time.Time) {
		s, _, obs := feedFixture(t, tracer)
		req := receiptRequest()
		original := []byte("original reply")
		if reply, err := s.WriteOnce(t.Context(), req, receiptWrite("a", original)); err != nil || !bytes.Equal(reply, original) {
			t.Fatalf("the first write failed: %v", err)
		}
		// A sweep with a past cutoff only records when the receipt became visible.
		past := storetest.Scalar[time.Time](t, obs, "SELECT clock_timestamp() - interval '24 hours'")
		if got, err := s.pruneBefore(t.Context(), past); err != nil || got.ReceiptRows != 0 {
			t.Fatalf("the observation sweep returned %+v and %v", got, err)
		}
		future := storetest.Scalar[time.Time](t, obs, "SELECT clock_timestamp() + interval '1 hour'")
		return s, obs, req, future
	}

	t.Run("replay locks first", func(t *testing.T) {
		var armed atomic.Bool
		tracer := &feedTestTracer{before: func(ctx context.Context, tx pgx.Tx) error {
			if armed.Load() {
				_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::bigint)", pruneReplayGateID)
				return err
			}
			return nil
		}}
		s, obs, req, future := setup(t, tracer)
		release := storetest.HoldAdvisoryGate(t, obs, pruneReplayGateID)
		defer release()
		armed.Store(true)
		original := []byte("original reply")
		replay := receiptStart(t, s, t.Context(), req, func(context.Context, *Tx) ([]byte, error) {
			return []byte("unexpected callback reply"), nil
		})
		var replayPID int32
		storetest.Poll(t, "the replay holding its receipt lock at the fixture gate", func() bool {
			pids := poolBlockedPIDs(t, obs, storetest.BigintKey(pruneReplayGateID))
			if len(pids) == 1 {
				replayPID = pids[0]
			}
			return len(pids) == 1
		})
		sweep := storetest.Start(t, func() pruneTestResult {
			counts, err := s.pruneBefore(t.Context(), future)
			return pruneTestResult{counts, err}
		})
		pruneWaitOnHolder(t, obs, replayPID, sweep)
		feedRequireCount(t, obs, "receipts", 1)
		release()
		if got := replay.Wait(t); got.err != nil || !bytes.Equal(got.reply, original) {
			t.Fatalf("the replay returned %q and %v, want the original reply", got.reply, got.err)
		}
		result := sweep.Wait(t)
		pruneCheckResult(t, result.counts, result.err, 1, 1)
		feedRequireCount(t, obs, "receipts", 0)
	})

	t.Run("deletion locks first", func(t *testing.T) {
		s, obs, req, future := setup(t, nil)
		storetest.Exec(t, obs, fmt.Sprintf(`
CREATE FUNCTION prune_test_delete_gate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(%d::bigint);
    RETURN OLD;
END;
$$;
CREATE TRIGGER prune_test_delete_gate BEFORE DELETE ON receipts
FOR EACH ROW EXECUTE FUNCTION prune_test_delete_gate();`, pruneDeleteGateID))
		release := storetest.HoldAdvisoryGate(t, obs, pruneDeleteGateID)
		defer release()
		sweep := storetest.Start(t, func() pruneTestResult {
			counts, err := s.pruneBefore(t.Context(), future)
			return pruneTestResult{counts, err}
		})
		var sweepPID int32
		storetest.Poll(t, "the deletion holding the receipt lock at the fixture gate", func() bool {
			pids := poolBlockedPIDs(t, obs, storetest.BigintKey(pruneDeleteGateID))
			if len(pids) == 1 {
				sweepPID = pids[0]
			}
			return len(pids) == 1
		})

		var calls atomic.Int32
		fresh := []byte("fresh reply")
		replay := receiptStart(t, s, t.Context(), req, func(ctx context.Context, tx *Tx) ([]byte, error) {
			calls.Add(1)
			return fresh, feedSave(ctx, tx, "b")
		})
		storetest.Poll(t, "the replay waiting for the receipt row lock", func() bool {
			select {
			case <-replay.Done():
				t.Fatalf("the replay returned %q and %v before the deletion committed", replay.Result().reply, replay.Result().err)
			default:
			}
			return storetest.Scalar[bool](t, obs, `
SELECT EXISTS (
    SELECT FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid
    WHERE a.datname = current_database() AND NOT l.granted AND l.locktype = 'transactionid'
      AND $1 = ANY (pg_blocking_pids(a.pid)) AND a.pid <> $1
)`, sweepPID)
		})
		feedRequireCount(t, obs, "receipts", 1)
		release()
		result := sweep.Wait(t)
		pruneCheckResult(t, result.counts, result.err, 1, 1)
		if got := replay.Wait(t); got.err != nil || !bytes.Equal(got.reply, fresh) {
			t.Fatalf("the retry returned %q and %v, want the fresh reply", got.reply, got.err)
		}
		if calls.Load() != 1 {
			t.Fatalf("the retry ran %d new mutations, want 1", calls.Load())
		}
		feedRequireCount(t, obs, "receipts", 1)
		if id := storetest.Scalar[string](t, obs, "SELECT id FROM records WHERE id = 'b'"); id != "b" {
			t.Fatalf("the new mutation left %q", id)
		}
	})
}
