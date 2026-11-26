package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// instanceApplicationName names the dedicated session so that an operator or a test can find its backend.
const instanceApplicationName = "lgtm-server-instance"

// instanceCleanupWait bounds how long a failed acquisition waits for the driver to cancel and close its socket.
const instanceCleanupWait = 2 * time.Second

var (
	// ErrServerInstanceBusy reports that another server held the instance lock for the whole admission wait.
	ErrServerInstanceBusy = NewDomainError("Another server is already using this database.")
	// errInstanceAcquire reports any other failure to open the dedicated session or to take the lock.
	errInstanceAcquire = NewDomainError("The server could not acquire its PostgreSQL instance lock.")
	// errInstanceLost reports a failed probe or a lock that is no longer held.
	errInstanceLost = NewDomainError("The server lost its PostgreSQL instance lock.")
	// errInstanceRelease reports that the unlock did not complete. The socket is closed regardless.
	errInstanceRelease = NewDomainError("The server could not release its PostgreSQL instance lock.")
)

// InstanceLock is the session advisory lock that makes one server the only user of a database.
// It owns one unpooled connection. Its methods are serialized, because a connection runs one query at a time.
type InstanceLock struct {
	mu   sync.Mutex
	conn *pgx.Conn
}

// AcquireServerInstance opens a dedicated session through the Store's validated connection settings
// and waits at most wait for the server instance lock.
// It returns the context's error when ctx ends, ErrServerInstanceBusy only when the wait expires
// during the lock query, and a fixed acquisition error for every other failure.
func (s *Store) AcquireServerInstance(ctx context.Context, wait time.Duration) (*InstanceLock, error) {
	admission, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	conn, err := s.connectInstance(admission)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errInstanceAcquire
	}

	// The Store's default statement timeout would end the wait early, so the admission deadline alone bounds it.
	_, err = conn.Exec(admission, "SET statement_timeout = 0")
	if err == nil {
		_, err = conn.Exec(admission, "SELECT pg_advisory_lock($1::bigint)", serverInstanceLockID)
		if err == nil {
			return &InstanceLock{conn: conn}, nil
		}
	}
	closeInstanceConn(conn, true)
	switch {
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case errors.Is(admission.Err(), context.DeadlineExceeded):
		return nil, ErrServerInstanceBusy
	default:
		return nil, errInstanceAcquire
	}
}

// connectInstance dials one session with the pool's own configuration, hook and dial policy.
// The application name is set before the hook runs, so the hook can recognize this session.
func (s *Store) connectInstance(ctx context.Context) (*pgx.Conn, error) {
	pool := s.pool.Config()
	conn := pool.ConnConfig.Copy()
	if conn.RuntimeParams == nil {
		conn.RuntimeParams = make(map[string]string)
	}
	conn.RuntimeParams["application_name"] = instanceApplicationName
	if pool.BeforeConnect != nil {
		if err := pool.BeforeConnect(ctx, conn); err != nil {
			return nil, err
		}
	}
	return pgx.ConnectConfig(ctx, conn)
}

// closeInstanceConn closes the socket at once, so PostgreSQL releases the lock without waiting for the driver.
// With waitCleanup it also waits briefly for the driver's cleanup. When a query was interrupted, that cleanup
// sends PostgreSQL a cancel request, so a backend that still waits for the lock gives up and exits.
func closeInstanceConn(conn *pgx.Conn, waitCleanup bool) {
	_ = conn.PgConn().Conn().Close()
	ctx, cancel := context.WithTimeout(context.Background(), instanceCleanupWait)
	defer cancel()
	_ = conn.Close(ctx)
	if !waitCleanup {
		return
	}
	select {
	case <-conn.PgConn().CleanupDone():
	case <-ctx.Done():
	}
}

// Check runs one probe on the dedicated session. A failed probe closes the session,
// which releases the lock on the server, and every later call reports the loss.
func (l *InstanceLock) Check(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return errInstanceLost
	}
	var one int
	if err := l.conn.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		closeInstanceConn(l.conn, false)
		l.conn = nil
		return errInstanceLost
	}
	return nil
}

// Close unlocks on the live session within ctx and then closes the session, even if the unlock failed.
// It is safe to call more than once.
func (l *InstanceLock) Close(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return nil
	}
	var released bool
	err := l.conn.QueryRow(ctx, "SELECT pg_advisory_unlock($1::bigint)", serverInstanceLockID).Scan(&released)
	closeInstanceConn(l.conn, false)
	l.conn = nil
	if err != nil || !released {
		return errInstanceRelease
	}
	return nil
}
