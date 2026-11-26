package storetest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// AdvisoryKey is the exact key of a PostgreSQL advisory lock as pg_locks reports it.
type AdvisoryKey struct{ ClassID, ObjectID, SubID uint32 }

// BigintKey returns the key of the one-argument bigint form of the advisory functions.
// It uses the high and low words with SubID 1.
func BigintKey(value int64) AdvisoryKey {
	return AdvisoryKey{ClassID: uint32(uint64(value) >> 32), ObjectID: uint32(uint64(value)), SubID: 1}
}

// IsBlocked reports whether the backend waiter waits for the exact advisory key
// while the backend blocker is among the backends that block it.
func IsBlocked(ctx context.Context, observer *pgx.Conn, waiter, blocker int32, key AdvisoryKey) (bool, error) {
	var blocked bool
	err := observer.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1 FROM pg_locks l
    WHERE l.locktype = 'advisory' AND NOT l.granted AND l.pid = $1
      AND l.classid::bigint = $3 AND l.objid::bigint = $4 AND l.objsubid = $5
) AND $2 = ANY (pg_blocking_pids($1))`,
		waiter, blocker, int64(key.ClassID), int64(key.ObjectID), int16(key.SubID)).Scan(&blocked)
	return blocked, err
}

// WaitForBlock polls every 10 milliseconds for at most 15 seconds until IsBlocked holds.
func WaitForBlock(t testing.TB, observer *pgx.Conn, waiter, blocker int32, key AdvisoryKey) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		blocked, err := IsBlocked(ctx, observer, waiter, blocker, key)
		cancel()
		if err != nil {
			t.Fatalf("the gate observation failed: %v", err)
		}
		if blocked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("backend %d did not block on backend %d for advisory key %+v within 15 seconds", waiter, blocker, key)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// HoldAdvisoryGate takes the session advisory lock on the observer connection
// and returns an idempotent release that is also registered as cleanup.
func HoldAdvisoryGate(t testing.TB, observer *pgx.Conn, key int64) func() {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := observer.Exec(ctx, "SELECT pg_advisory_lock($1::bigint)", key); err != nil {
		t.Fatalf("the gate could not be held: %v", err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = observer.Exec(ctx, "SELECT pg_advisory_unlock($1::bigint)", key)
		})
	}
	t.Cleanup(release)
	return release
}
