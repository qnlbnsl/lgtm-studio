package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	feedTestGateID int64 = 841941100901
	storeTestQuiet       = 250 * time.Millisecond
)

type feedTestTracer struct {
	after   func(context.Context, pgx.Tx) error
	before  func(context.Context, pgx.Tx) error
	batches atomic.Int32
	commits atomic.Int32
}

var _ feedTestHooks = (*feedTestTracer)(nil)

func (f *feedTestTracer) afterConstraints(ctx context.Context, tx pgx.Tx) error {
	if f.after != nil {
		return f.after(ctx, tx)
	}
	return nil
}

func (f *feedTestTracer) beforeCommit(ctx context.Context, tx pgx.Tx) error {
	if f.before != nil {
		return f.before(ctx, tx)
	}
	return nil
}

func (f *feedTestTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.EqualFold(data.SQL, "COMMIT") {
		f.commits.Add(1)
	}
	return ctx
}

func (*feedTestTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (f *feedTestTracer) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	f.batches.Add(1)
	return ctx
}

func (*feedTestTracer) TraceBatchQuery(context.Context, *pgx.Conn, pgx.TraceBatchQueryData) {}
func (*feedTestTracer) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData)     {}

func storeTestOpen(t *testing.T, cfg *pgxpool.Config) *Store {
	t.Helper()
	s, err := Open(t.Context(), cfg, ConnectionPolicy{})
	if err != nil {
		t.Fatalf("opening the store failed: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func feedFixture(t *testing.T, tracer *feedTestTracer) (*Store, *pgxpool.Config, *pgx.Conn) {
	t.Helper()
	cfg := storetest.NewDatabase(t, "store_fixture")
	if tracer != nil {
		cfg.ConnConfig.Tracer = tracer
	}
	s := storeTestOpen(t, cfg)
	observer := poolObserver(t, cfg)
	if err := s.Migrate(t.Context()); err != nil {
		t.Fatalf("migrating the store failed: %v", err)
	}
	storetest.Exec(t, observer, "CREATE TABLE records (id text PRIMARY KEY, value text NOT NULL)")
	return s, cfg, observer
}

func feedRequireCount(t *testing.T, obs *pgx.Conn, table string, want int) {
	t.Helper()
	if got := storetest.Scalar[int](t, obs, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()); got != want {
		t.Fatalf("%s contains %d rows, want %d", table, got, want)
	}
}

func feedRecord(key string) ChangeRef {
	return ChangeRef{Topic: "records", Key: key, Change: Saved}
}

func feedSave(ctx context.Context, tx *Tx, key string) error {
	if _, err := tx.Exec(ctx, "INSERT INTO records VALUES ($1, $1)", key); err != nil {
		return err
	}
	return tx.Record(feedRecord(key))
}

func feedWrite(t *testing.T, s *Store, key string) {
	t.Helper()
	if err := s.Write(t.Context(), func(ctx context.Context, tx *Tx) error {
		return feedSave(ctx, tx, key)
	}); err != nil {
		t.Fatalf("writing record %q failed: %v", key, err)
	}
}

func feedStartWrite(t *testing.T, s *Store, ctx context.Context, key string) (*storetest.Task[error], int32) {
	t.Helper()
	pid := make(chan int32, 1)
	task := storetest.Start(t, func() error {
		return s.Write(ctx, func(ctx context.Context, tx *Tx) error {
			var writer int32
			if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&writer); err != nil {
				return err
			}
			pid <- writer
			return feedSave(ctx, tx, key)
		})
	})
	return task, storetest.ReceiveFrom(t, task, "writer "+key+" reporting its backend", pid)
}

func feedInstallGate(t *testing.T, obs *pgx.Conn, reject bool) {
	t.Helper()
	body := ""
	if reject {
		body = "RAISE EXCEPTION 'fixture rejected commit' USING ERRCODE = '23514', CONSTRAINT = 'feed_test_gate';"
	}
	storetest.Exec(t, obs, fmt.Sprintf(`
CREATE FUNCTION feed_test_gate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.key = 'a' THEN
        PERFORM pg_advisory_xact_lock(%d::bigint);
        %s
    END IF;
    RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER feed_test_gate AFTER INSERT ON feed
    DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW EXECUTE FUNCTION feed_test_gate();`, feedTestGateID, body))
}

func feedDeferGate(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "SET CONSTRAINTS feed_test_gate DEFERRED")
	return err
}

func feedWaitCommit(t *testing.T, obs *pgx.Conn, writer int32) {
	t.Helper()
	key := storetest.BigintKey(feedTestGateID)
	blocker := poolBackendPID(t, obs)
	// One query observes both facts, so a delayed second observation cannot miss the wait.
	storetest.Poll(t, "the writer waiting for the fixture gate inside COMMIT", func() bool {
		return storetest.Scalar[bool](t, obs, `
SELECT EXISTS (
    SELECT FROM pg_stat_activity a JOIN pg_locks l ON l.pid = a.pid
    WHERE a.pid = $1 AND a.state = 'active' AND upper(a.query) = 'COMMIT'
      AND l.locktype = 'advisory' AND NOT l.granted
      AND l.classid::bigint = $3 AND l.objid::bigint = $4 AND l.objsubid = $5
      AND $2 = ANY (pg_blocking_pids(a.pid))
)`, writer, blocker, int64(key.ClassID), int64(key.ObjectID), int16(key.SubID))
	})
}

// feedWaitWriterEnded waits until the writer's backend neither holds nor awaits an advisory lock
// and the pool counts no acquired connection, since the driver disposes of a cancelled connection asynchronously.
func feedWaitWriterEnded(t *testing.T, s *Store, obs *pgx.Conn, writer int32) {
	t.Helper()
	storetest.Poll(t, "the ended writer returning its pool connection and leaving every advisory lock", func() bool {
		return s.pool.Stat().AcquiredConns() == 0 && !storetest.Scalar[bool](t, obs,
			"SELECT EXISTS (SELECT FROM pg_locks WHERE pid = $1 AND locktype = 'advisory')", writer)
	})
}

func feedSequenceValue(t *testing.T, obs *pgx.Conn) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, obs, `
SELECT s.last_value
FROM pg_sequences s
JOIN pg_namespace n ON n.nspname = s.schemaname
JOIN pg_class c ON c.relnamespace = n.oid AND c.relname = s.sequencename
WHERE c.oid = pg_get_serial_sequence('feed', 'position')::regclass`)
}

func feedRequireOrder(t *testing.T, obs *pgx.Conn, want ...string) {
	t.Helper()
	var cursor int64
	for _, key := range want {
		var current int64
		var got string
		err := obs.QueryRow(t.Context(), `
SELECT position, key FROM feed WHERE position > $1 ORDER BY position LIMIT 1`, cursor).Scan(&current, &got)
		if err != nil || got != key || current <= cursor {
			t.Fatalf("after position %d got position %d and key %q, want key %q: %v", cursor, current, got, key, err)
		}
		cursor = current
	}
	feedRequireCount(t, obs, "feed", len(want))
}

func TestFeedPositionsFollowCommitOrder(t *testing.T) {
	s, _, obs := feedFixture(t, &feedTestTracer{after: feedDeferGate})
	feedInstallGate(t, obs, false)
	release := storetest.HoldAdvisoryGate(t, obs, feedTestGateID)
	defer release()
	a, aPID := feedStartWrite(t, s, t.Context(), "a")
	feedWaitCommit(t, obs, aPID)

	b, bPID := feedStartWrite(t, s, t.Context(), "b")
	if aPID == bPID {
		t.Fatal("the writers use the same backend")
	}
	storetest.Poll(t, "writer B waiting on the feed lock or committing", func() bool {
		visibleFirst := storetest.Scalar[bool](t, obs, `
SELECT EXISTS (SELECT FROM feed WHERE key = 'b') AND NOT EXISTS (SELECT FROM feed WHERE key = 'a')`)
		if visibleFirst {
			release()
			if aErr, bErr := a.Wait(t), b.Wait(t); aErr != nil || bErr != nil {
				t.Fatalf("joining the writers failed: A = %v, B = %v", aErr, bErr)
			}
			t.Fatal("writer B became visible before writer A")
		}
		select {
		case <-b.Done():
			if b.Result() != nil {
				t.Fatalf("writer B returned an error: %v", b.Result())
			}
		default:
		}
		blocked, err := storetest.IsBlocked(t.Context(), obs, bPID, aPID, storetest.BigintKey(feedLockID))
		if err != nil {
			t.Fatal(err)
		}
		return blocked
	})
	feedRequireCount(t, obs, "feed", 0)
	if seq := feedSequenceValue(t, obs); seq != 1 {
		t.Fatalf("feed sequence reached %d before writer A committed, want 1", seq)
	}
	release()
	if aErr, bErr := a.Wait(t), b.Wait(t); aErr != nil || bErr != nil {
		t.Fatalf("joining the writers failed: A = %v, B = %v", aErr, bErr)
	}
	feedRequireOrder(t, obs, "a", "b")
}

func TestFeedLockStartsAtFlush(t *testing.T) {
	s, _, obs := feedFixture(t, nil)
	pause, resume := storetest.Signal(t)
	defer resume()
	ready := make(chan struct{}, 1)
	a := storetest.Start(t, func() error {
		return s.Write(t.Context(), func(ctx context.Context, tx *Tx) error {
			if err := feedSave(ctx, tx, "a"); err != nil {
				return err
			}
			ready <- struct{}{}
			select {
			case <-pause:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	})
	storetest.ReceiveFrom(t, a, "writer A pausing after Record", ready)
	short, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := s.Write(short, func(ctx context.Context, tx *Tx) error {
		return feedSave(ctx, tx, "b")
	}); err != nil {
		t.Fatalf("writer B did not commit while A's callback was paused: %v", err)
	}
	feedRequireOrder(t, obs, "b")
	resume()
	if err := a.Wait(t); err != nil {
		t.Fatal(err)
	}
	feedRequireOrder(t, obs, "b", "a")
}

func feedOutcome(t *testing.T, err error, code string, outcome Outcome, retryable bool) *Error {
	t.Helper()
	got, ok := err.(*Error)
	if !ok || got.Outcome() != outcome || got.Retryable() != retryable || (code != "" && got.SQLState != code) {
		t.Fatalf("got error %T %v, want SQLSTATE %q, outcome %q and retryable = %v", err, err, code, outcome, retryable)
	}
	if errors.Unwrap(err) != nil {
		t.Fatal("the database error exposed a raw cause")
	}
	return got
}

func TestFeedRollbackAndNotification(t *testing.T) {
	tracer := &feedTestTracer{after: feedDeferGate}
	s, cfg, obs := feedFixture(t, tracer)
	listener := poolObserver(t, cfg)
	storetest.Exec(t, listener, "LISTEN lgtm_feed_changed")
	feedInstallGate(t, obs, true)
	release := storetest.HoldAdvisoryGate(t, obs, feedTestGateID)
	defer release()
	writer, pid := feedStartWrite(t, s, t.Context(), "a")
	feedWaitCommit(t, obs, pid)

	silent := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), storeTestQuiet)
		defer cancel()
		if notification, err := listener.WaitForNotification(ctx); notification != nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("wanted notification silence, got %v and %v", notification, err)
		}
	}
	silent()
	release()
	feedOutcome(t, writer.Wait(t), "23514", OutcomeRolledBack, false)
	feedRequireCount(t, obs, "records", 0)
	feedRequireCount(t, obs, "feed", 0)
	silent()

	before := tracer.batches.Load()
	var want []string
	err := s.Write(t.Context(), func(ctx context.Context, tx *Tx) error {
		for i := MaxWriteChanges; i > 0; i-- {
			key := fmt.Sprintf("record-%03d", i)
			want = append(want, key)
			if err := feedSave(ctx, tx, key); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if batches := tracer.batches.Load() - before; batches != 1 {
		t.Fatalf("the 256-reference write sent %d driver batches, want 1", batches)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	notification, err := listener.WaitForNotification(ctx)
	if err != nil || notification == nil || notification.Channel != "lgtm_feed_changed" || notification.Payload != "" {
		t.Fatalf("expected one empty committed feed notification, got %v and %v", notification, err)
	}
	got := storetest.Rows[string](t, obs, "SELECT key FROM feed ORDER BY position")
	if len(want) != 256 || !slices.Equal(got, want) || feedSequenceValue(t, obs) != 257 {
		t.Fatalf("the write did not publish all 256 references in Record order: committed %d of %d", len(got), len(want))
	}
	feedRequireCount(t, obs, "records", 256)
	silent()
}

func TestFeedRejectsInsertOutsideFlush(t *testing.T) {
	s, _, obs := feedFixture(t, nil)
	storetest.Exec(t, obs, `
BEGIN;
SET LOCAL lgtm.feed_flush = 'on';
INSERT INTO feed (topic, scope, key, change) VALUES ('records', '', 'seed', 'saved');
COMMIT;`)
	for _, tc := range []struct {
		name, sql, constraint string
	}{
		{"insert", "INSERT INTO feed (topic, scope, key, change) VALUES ('records', '', 'stray', 'saved')", "feed_requires_store_flush"},
		{"update", "UPDATE feed SET key = 'stray'", "feed_requires_store_prune"},
		{"delete", "DELETE FROM feed", "feed_requires_store_prune"},
		{"truncate", "TRUNCATE feed", "feed_refuses_truncate"},
		{"watermark", "SET LOCAL lgtm.feed_prune = 'on'; DELETE FROM feed", "feed_delete_requires_watermark"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Write(t.Context(), func(ctx context.Context, tx *Tx) error {
				_, err := tx.Exec(ctx, tc.sql)
				return err
			})
			got := feedOutcome(t, err, "23514", OutcomeRolledBack, false)
			if got.Constraint != tc.constraint {
				t.Fatalf("constraint = %q, want %q", got.Constraint, tc.constraint)
			}
			feedRequireOrder(t, obs, "seed")
		})
	}
	t.Run("record", func(t *testing.T) {
		feedWrite(t, s, "a")
		feedRequireOrder(t, obs, "seed", "a")
	})
}

func TestRecordValidationAndLimit(t *testing.T) {
	s, _, obs := feedFixture(t, nil)
	for _, tc := range []struct {
		name   string
		change ChangeRef
	}{
		{"empty topic", ChangeRef{Key: "a", Change: Saved}},
		{"empty key", ChangeRef{Topic: "records", Change: Saved}},
		{"zero byte in topic", ChangeRef{Topic: "records\x00", Key: "a", Change: Saved}},
		{"zero byte in scope", ChangeRef{Topic: "records", Scope: "\x00", Key: "a", Change: Saved}},
		{"zero byte in key", feedRecord("a\x00")},
		{"invalid Unicode encoding", feedRecord("a\xff")},
		{"invalid change", ChangeRef{Topic: "records", Key: "a", Change: "renamed"}},
		{"limit", feedRecord("overflow")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var recordErr error
			err := s.Write(t.Context(), func(ctx context.Context, tx *Tx) error {
				if _, err := tx.Exec(ctx, "INSERT INTO records VALUES ('a', 'a')"); err != nil {
					return err
				}
				if tc.name == "limit" {
					for i := range 256 {
						if err := tx.Record(feedRecord(fmt.Sprintf("key-%d", i))); err != nil {
							return err
						}
					}
				}
				recordErr = tx.Record(tc.change)
				return nil
			})
			if recordErr == nil || err == nil {
				t.Fatalf("Record error = %v, Write error = %v, want both to fail", recordErr, err)
			}
			if tc.name == "limit" && (!errors.Is(recordErr, ErrTooManyChanges) || !errors.Is(err, ErrTooManyChanges)) {
				t.Fatalf("the 257th reference returned %v and %v, want ErrTooManyChanges", recordErr, err)
			}
			feedRequireCount(t, obs, "records", 0)
			feedRequireCount(t, obs, "feed", 0)
		})
	}
}

func TestFeedLockDoesNotBlockUnrecordedWrites(t *testing.T) {
	for _, cancelWaiter := range []bool{true, false} {
		name := "lock timeout"
		if cancelWaiter {
			name = "caller cancellation"
		}
		t.Run(name, func(t *testing.T) {
			tracer := &feedTestTracer{}
			s, _, obs := feedFixture(t, tracer)
			release := storetest.HoldAdvisoryGate(t, obs, feedLockID)
			defer release()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			waiter, pid := feedStartWrite(t, s, ctx, "a")
			storetest.WaitForBlock(t, obs, pid, poolBackendPID(t, obs), storetest.BigintKey(feedLockID))

			short, end := context.WithTimeout(t.Context(), time.Second)
			defer end()
			before := tracer.batches.Load()
			err := s.Write(short, func(ctx context.Context, tx *Tx) error {
				_, err := tx.Exec(ctx, "INSERT INTO records VALUES ('unrecorded', 'value')")
				return err
			})
			if err != nil || tracer.batches.Load() != before {
				t.Fatalf("a write with no references while the feed lock was held returned %v or sent a batch", err)
			}
			feedRequireCount(t, obs, "records", 1)
			feedRequireCount(t, obs, "feed", 0)

			if cancelWaiter {
				cancel()
				if err := waiter.Wait(t); !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelling a writer before commit returned %v, want context.Canceled", err)
				}
			} else {
				feedOutcome(t, waiter.Wait(t), "55P03", OutcomeRolledBack, true)
			}
			poolWaitNoOpenTransaction(t, obs)
			feedWaitWriterEnded(t, s, obs, pid)
			feedRequireCount(t, obs, "records", 1)
			feedRequireCount(t, obs, "feed", 0)
		})
	}
}

func storeTestRequireNoLock(t *testing.T, obs *pgx.Conn, key storetest.AdvisoryKey) {
	t.Helper()
	query := `
SELECT count(*) FROM pg_locks
WHERE locktype = 'advisory' AND database = (SELECT oid FROM pg_database WHERE datname = current_database())
  AND classid::bigint = $1 AND objid::bigint = $2 AND objsubid = $3`
	if count := storetest.Scalar[int](t, obs, query, int64(key.ClassID), int64(key.ObjectID), int16(key.SubID)); count != 0 {
		t.Fatalf("%d advisory locks for %+v remain after the transaction ended", count, key)
	}
}

func TestFeedLockEndsWithTransaction(t *testing.T) {
	for _, name := range []string{"cancel", "panic", "commit rejection"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			tracer := &feedTestTracer{before: func(context.Context, pgx.Tx) error {
				switch name {
				case "cancel":
					cancel()
				case "panic":
					panic("feed test panic")
				}
				return nil
			}}
			if name == "commit rejection" {
				tracer.after = feedDeferGate
			}
			s, cfg, obs := feedFixture(t, tracer)
			cfg.ConnConfig.Tracer = nil
			second := storeTestOpen(t, cfg)

			var firstPID int32
			if name == "commit rejection" {
				feedInstallGate(t, obs, true)
				release := storetest.HoldAdvisoryGate(t, obs, feedTestGateID)
				defer release()
				task, pid := feedStartWrite(t, s, ctx, "a")
				firstPID = pid
				feedWaitCommit(t, obs, pid)
				release()
				feedOutcome(t, task.Wait(t), "23514", OutcomeRolledBack, false)
			} else {
				var panicValue any
				var err error
				func() {
					defer func() { panicValue = recover() }()
					err = s.Write(ctx, func(ctx context.Context, tx *Tx) error {
						if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&firstPID); err != nil {
							return err
						}
						return feedSave(ctx, tx, "a")
					})
				}()
				if name == "cancel" && (panicValue != nil || !errors.Is(err, context.Canceled)) {
					t.Fatalf("cancel after the batch returned %v and panic %v", err, panicValue)
				}
				if name == "panic" && panicValue != "feed test panic" {
					t.Fatalf("panic after the batch returned %v and panic %v", err, panicValue)
				}
			}
			storeTestRequireNoLock(t, obs, storetest.BigintKey(feedLockID))
			feedRequireCount(t, obs, "feed", 0)
			feedRequireCount(t, obs, "records", 0)

			short, end := context.WithTimeout(t.Context(), time.Second)
			defer end()
			var secondPID int32
			err := second.Write(short, func(ctx context.Context, tx *Tx) error {
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&secondPID); err != nil {
					return err
				}
				return feedSave(ctx, tx, "b")
			})
			if err != nil || firstPID == 0 || secondPID == 0 || firstPID == secondPID {
				t.Fatalf("writing from another backend failed: first = %d, second = %d, err = %v", firstPID, secondPID, err)
			}
			feedRequireOrder(t, obs, "b")
		})
	}
}

// deadlockWrite lets the caller run the same lock inversion through Write and WriteOnce.
type deadlockWrite func(context.Context, *Tx) error

func storeTestDeadlockCallbacks(t *testing.T, obs *pgx.Conn) ([2]deadlockWrite, [2]chan struct{}, func()) {
	t.Helper()
	storetest.Exec(t, obs, "INSERT INTO records VALUES ('left', 'original'), ('right', 'original')")
	goOn, release := storetest.Signal(t)
	ready := [2]chan struct{}{make(chan struct{}, 1), make(chan struct{}, 1)}
	var writes [2]deadlockWrite
	keys := [2]string{"left", "right"}
	for i := range writes {
		writes[i] = func(ctx context.Context, tx *Tx) error {
			for j, key := range [2]string{keys[i], keys[1-i]} {
				if _, err := tx.Exec(ctx, "SELECT id FROM records WHERE id = $1 FOR UPDATE", key); err != nil {
					return err
				}
				if j == 0 {
					ready[i] <- struct{}{}
					select {
					case <-goOn:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
			return nil
		}
	}
	return writes, ready, release
}

func TestFeedChecksDeferredDomainConstraintsBeforeLock(t *testing.T) {
	t.Run("deferred foreign key", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		storetest.Exec(t, obs, `
CREATE TABLE parent (id int PRIMARY KEY);
CREATE TABLE child (parent_id int REFERENCES parent DEFERRABLE INITIALLY DEFERRED);
INSERT INTO parent VALUES (1);`)
		goOn, release := storetest.Signal(t)
		defer release()
		bPID := make(chan int32, 1)
		b := storetest.Start(t, func() error {
			return s.Write(t.Context(), func(ctx context.Context, tx *Tx) error {
				var pid int32
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM parent WHERE id = 1 FOR UPDATE").Scan(&pid); err != nil {
					return err
				}
				bPID <- pid
				select {
				case <-goOn:
					return feedSave(ctx, tx, "b")
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		})
		blocker := storetest.ReceiveFrom(t, b, "writer B locking the parent", bPID)
		aPID := make(chan int32, 1)
		a := storetest.Start(t, func() error {
			return s.Write(t.Context(), func(ctx context.Context, tx *Tx) error {
				var pid int32
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, "INSERT INTO child VALUES (1)"); err != nil {
					return err
				}
				aPID <- pid
				return feedSave(ctx, tx, "a")
			})
		})
		waiter := storetest.ReceiveFrom(t, a, "writer A inserting the child", aPID)
		storetest.WaitOnRow(t, obs, waiter, blocker)
		storeTestRequireNoLock(t, obs, storetest.BigintKey(feedLockID))
		release()
		if aErr, bErr := a.Wait(t), b.Wait(t); aErr != nil || bErr != nil {
			t.Fatalf("joining the writers failed: A = %v, B = %v", aErr, bErr)
		}
		feedRequireOrder(t, obs, "b", "a")
	})

	t.Run("domain deadlock", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		writes, ready, release := storeTestDeadlockCallbacks(t, obs)
		defer release()
		var tasks [2]*storetest.Task[error]
		for i, write := range writes {
			tasks[i] = storetest.Start(t, func() error {
				return s.Write(t.Context(), func(ctx context.Context, tx *Tx) error {
					if err := write(ctx, tx); err != nil {
						return err
					}
					_, err := tx.Exec(ctx, "UPDATE records SET value = 'committed' WHERE id = 'left'")
					return err
				})
			})
		}
		for i, locked := range ready {
			storetest.ReceiveFrom(t, tasks[i], "the first row lock", locked)
		}
		release()
		victims := 0
		for _, task := range tasks {
			if err := task.Wait(t); err != nil {
				feedOutcome(t, err, "40P01", OutcomeRolledBack, true)
				victims++
			}
		}
		if victims != 1 {
			t.Fatalf("got %d deadlock victims, want 1", victims)
		}
		if value := storetest.Scalar[string](t, obs, "SELECT value FROM records WHERE id = 'left'"); value != "committed" {
			t.Fatalf("the surviving writer left the record value %q", value)
		}
		feedRequireCount(t, obs, "feed", 0)
	})
}

func TestFeedCommitErrorOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		outcome    Outcome
		retryable  bool
	}{
		{"lock timeout", "55P03", OutcomeRolledBack, true},
		{"trigger rejection", "23514", OutcomeRolledBack, false},
		{"aborted transaction", "", OutcomeRolledBack, false},
		{"backend loss", "", OutcomeUnknown, false},
		{"caller cancellation", "", OutcomeUnknown, false},
		{"caller deadline", "", OutcomeUnknown, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tracer := &feedTestTracer{after: feedDeferGate}
			switch tc.name {
			case "lock timeout":
				tracer.before = func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '3s'")
					return err
				}
			case "aborted transaction":
				tracer.before = func(ctx context.Context, tx pgx.Tx) error {
					// Division by zero leaves the transaction in the failed state.
					_, _ = tx.Exec(ctx, "SELECT 1 / 0")
					return nil
				}
			}
			s, _, obs := feedFixture(t, tracer)
			feedInstallGate(t, obs, tc.name == "trigger rejection")
			release := storetest.HoldAdvisoryGate(t, obs, feedTestGateID)
			defer release()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.name == "caller deadline" {
				var end context.CancelFunc
				ctx, end = context.WithTimeout(ctx, 3*time.Second)
				defer end()
			}
			commitsBefore := tracer.commits.Load()
			writer, pid := feedStartWrite(t, s, ctx, "a")
			if tc.name != "aborted transaction" {
				feedWaitCommit(t, obs, pid)
			}
			switch tc.name {
			case "trigger rejection":
				release()
			case "backend loss":
				if !storetest.Scalar[bool](t, obs, "SELECT pg_terminate_backend($1, 3000)", pid) {
					t.Fatal("could not terminate the committing backend")
				}
			case "caller cancellation":
				cancel()
			}
			got := feedOutcome(t, writer.Wait(t), tc.code, tc.outcome, tc.retryable)
			if commits := tracer.commits.Load() - commitsBefore; commits != 1 {
				t.Fatalf("the writer attempted COMMIT %d times, want 1", commits)
			}
			if tc.name == "aborted transaction" && got.SQLState != "" {
				t.Fatalf("the transaction rollback tag returned SQLSTATE %q", got.SQLState)
			}
			if tc.outcome == OutcomeUnknown && got.Error() != "The database did not confirm whether the write committed." {
				t.Fatalf("unexpected public ambiguity diagnostic %q", got.Error())
			}
			if tc.outcome == OutcomeRolledBack {
				feedRequireCount(t, obs, "records", 0)
				feedRequireCount(t, obs, "feed", 0)
			}
		})
	}
}

func TestFeedStatementTimeoutDuringCommit(t *testing.T) {
	tracer := &feedTestTracer{after: feedDeferGate, before: func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '3s'")
		return err
	}}
	s, _, obs := feedFixture(t, tracer)
	feedInstallGate(t, obs, false)
	release := storetest.HoldAdvisoryGate(t, obs, feedTestGateID)
	defer release()
	writer, pid := feedStartWrite(t, s, t.Context(), "a")
	feedWaitCommit(t, obs, pid)
	select {
	case <-writer.Done():
		t.Fatalf("the statement timeout ended COMMIT before the gate was released: %v", writer.Result())
	case <-time.After(3250 * time.Millisecond):
	}
	feedWaitCommit(t, obs, pid)
	release()
	if err := writer.Wait(t); err != nil {
		t.Fatalf("COMMIT failed after releasing the gate: %v", err)
	}
	feedRequireOrder(t, obs, "a")
}
