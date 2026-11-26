package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const instanceApplication = "lgtm-server-instance"

type instanceResult struct {
	lock *InstanceLock
	err  error
}

func instanceAcquire(t *testing.T, s *Store, ctx context.Context, wait time.Duration) *storetest.Task[instanceResult] {
	t.Helper()
	task := storetest.Start(t, func() instanceResult {
		lock, err := s.AcquireServerInstance(ctx, wait)
		return instanceResult{lock, err}
	})
	t.Cleanup(func() {
		if result := task.Wait(t); result.lock != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = result.lock.Close(ctx)
		}
	})
	return task
}

func instanceWaiter(t *testing.T, obs *pgx.Conn, blocker int32, task *storetest.Task[instanceResult]) int32 {
	t.Helper()
	key := storetest.BigintKey(serverInstanceLockID)
	var pid int32
	storetest.Poll(t, "the dedicated backend waiting for the instance lock", func() bool {
		select {
		case <-task.Done():
			t.Fatal("Acquisition returned before the dedicated backend waited for the lock.")
		default:
		}
		pid = storetest.Scalar[int32](t, obs, `
SELECT COALESCE(max(l.pid), 0)
FROM pg_locks l JOIN pg_stat_activity a USING (pid)
WHERE a.datname = current_database() AND a.application_name = $1
  AND l.locktype = 'advisory' AND NOT l.granted
  AND l.classid::bigint = $2 AND l.objid::bigint = $3 AND l.objsubid = $4
  AND $5 = ANY(pg_blocking_pids(l.pid))`,
			instanceApplication, int64(key.ClassID), int64(key.ObjectID), int16(key.SubID), blocker)
		return pid != 0
	})
	return pid
}

func requireInstance(t *testing.T, result instanceResult) *InstanceLock {
	t.Helper()
	if result.err != nil || result.lock == nil {
		t.Fatal("The dedicated connection did not acquire the instance lock.")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := result.lock.Check(ctx); err != nil {
		t.Fatal("A newly acquired instance lock failed its live probe.")
	}
	return result.lock
}

func requireInstanceBackendGone(t *testing.T, obs *pgx.Conn, pid int32) {
	t.Helper()
	storetest.Poll(t, "the retired instance backend to disconnect", func() bool {
		return !storetest.Scalar[bool](t, obs, "SELECT EXISTS(SELECT FROM pg_stat_activity WHERE pid = $1)", pid)
	})
}

func TestServerInstanceLock(t *testing.T) {
	t.Run("reserved identifiers", func(t *testing.T) {
		if feedLockID == migrationLockID || feedLockID == serverInstanceLockID || migrationLockID == serverInstanceLockID {
			t.Error("The feed, migration and server instance need distinct advisory keys.")
		}
	})

	t.Run("wait, release and reopen on another backend", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		// An inherited statement timeout must not end the admission query.
		cfg.ConnConfig.RuntimeParams["statement_timeout"] = "100ms"
		s := storeTestOpen(t, cfg)
		obs := poolObserver(t, cfg)
		release := storetest.HoldAdvisoryGate(t, obs, serverInstanceLockID)
		defer release()

		task := instanceAcquire(t, s, t.Context(), 8*time.Second)
		firstPID := instanceWaiter(t, obs, int32(obs.PgConn().PID()), task)
		timer := time.NewTimer(125 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-task.Done():
			t.Fatal("The inherited statement timeout interrupted the admission wait.")
		case <-timer.C:
		}
		release()
		first := requireInstance(t, task.Wait(t))

		next := instanceAcquire(t, s, t.Context(), 8*time.Second)
		secondPID := instanceWaiter(t, obs, firstPID, next)
		closeCtx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := first.Close(closeCtx); err != nil {
			t.Fatal("Closing the first instance lock failed.")
		}
		requireInstance(t, next.Wait(t))
		if secondPID == firstPID {
			t.Error("The second lock reused the first instance backend.")
		}
		requireInstanceBackendGone(t, obs, firstPID)
	})

	t.Run("admission deadline", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		s := storeTestOpen(t, cfg)
		obs := poolObserver(t, cfg)
		release := storetest.HoldAdvisoryGate(t, obs, serverInstanceLockID)
		defer release()

		task := instanceAcquire(t, s, t.Context(), time.Second)
		pid := instanceWaiter(t, obs, int32(obs.PgConn().PID()), task)
		if result := task.Wait(t); result.lock != nil || !errors.Is(result.err, ErrServerInstanceBusy) {
			t.Error("An expired admission query must return the instance-busy error.")
		}
		requireInstanceBackendGone(t, obs, pid)
	})

	t.Run("parent cancellation", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		s := storeTestOpen(t, cfg)
		obs := poolObserver(t, cfg)
		release := storetest.HoldAdvisoryGate(t, obs, serverInstanceLockID)
		defer release()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		task := instanceAcquire(t, s, ctx, 8*time.Second)
		pid := instanceWaiter(t, obs, int32(obs.PgConn().PID()), task)
		cancel()
		if result := task.Wait(t); result.lock != nil || !errors.Is(result.err, context.Canceled) {
			t.Error("Parent cancellation must stay distinct from instance contention.")
		}
		requireInstanceBackendGone(t, obs, pid)
	})

	t.Run("dedicated dial error", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		const privateDetail = "fake-private-dial-detail"
		failedDial := make(chan struct{}, 1)
		cfg.BeforeConnect = func(_ context.Context, conn *pgx.ConnConfig) error {
			if conn.RuntimeParams["application_name"] == instanceApplication {
				conn.DialFunc = func(context.Context, string, string) (net.Conn, error) {
					select {
					case failedDial <- struct{}{}:
					default:
					}
					return nil, errors.New(privateDetail)
				}
			}
			return nil
		}
		s := storeTestOpen(t, cfg)
		lock, err := s.AcquireServerInstance(t.Context(), time.Second)
		if lock != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = lock.Close(ctx)
		}
		if lock != nil || err == nil || errors.Is(err, ErrServerInstanceBusy) || errors.Is(err, context.DeadlineExceeded) {
			t.Error("A dedicated dial failure must return a distinct acquisition error.")
		}
		if err != nil && strings.Contains(fmt.Sprintf("%+v", err), privateDetail) {
			t.Error("The acquisition error disclosed a private dial detail.")
		}
		select {
		case <-failedDial:
		default:
			t.Error("Acquisition did not use the dedicated connection's dial hook.")
		}
	})
}
