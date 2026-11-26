package storetest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ObservationTimeout bounds a fixture's wait for an observable event.
const ObservationTimeout = 15 * time.Second

// Executor accepts fixture statements on a connection or a transaction.
type Executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// Exec runs a fixture statement and fails the test if it returns an error.
func Exec(t *testing.T, conn Executor, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("executing the fixture statement failed: %v", err)
	}
}

// Scalar reads a single value from the observer connection.
func Scalar[V any](t *testing.T, conn *pgx.Conn, sql string, args ...any) V {
	t.Helper()
	var value V
	if err := conn.QueryRow(t.Context(), sql, args...).Scan(&value); err != nil {
		t.Fatalf("reading the fixture value failed: %v", err)
	}
	return value
}

// Rows reads a single column from each row of a fixture query.
func Rows[V any](t *testing.T, obs *pgx.Conn, sql string, args ...any) []V {
	t.Helper()
	rows, err := obs.Query(t.Context(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	values, err := pgx.CollectRows(rows, pgx.RowTo[V])
	if err != nil {
		t.Fatal(err)
	}
	return values
}

// Receive waits for an event with the fixture deadline.
func Receive[V any](t *testing.T, what string, ch <-chan V) V {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(ObservationTimeout):
		t.Fatalf("did not observe %s within %v", what, ObservationTimeout)
		var zero V
		return zero
	}
}

// Task records a fixture goroutine's result when it finishes.
type Task[V any] struct {
	result V
	done   chan struct{}
}

// Start runs a fixture operation and joins it during cleanup.
func Start[V any](t *testing.T, run func() V) *Task[V] {
	t.Helper()
	task := &Task[V]{done: make(chan struct{})}
	go func() {
		defer close(task.done)
		task.result = run()
	}()
	t.Cleanup(func() {
		Receive(t, "fixture task cleanup", task.done)
	})
	return task
}

// Wait returns the result within the fixture deadline.
func (task *Task[V]) Wait(t *testing.T) V {
	t.Helper()
	Receive(t, "fixture task completion", task.done)
	return task.result
}

// ReceiveFrom waits for an event from a task and detects a premature exit.
func ReceiveFrom[V, R any](t *testing.T, task *Task[R], what string, event <-chan V) V {
	t.Helper()
	select {
	case value := <-event:
		return value
	case <-task.Done():
		select {
		case value := <-event:
			return value
		default:
			t.Fatalf("the fixture operation returned %v before %s", task.Result(), what)
		}
	case <-time.After(ObservationTimeout):
		t.Fatalf("did not observe %s within %v", what, ObservationTimeout)
	}
	var zero V
	return zero
}

// Signal returns a channel and an idempotent function that closes it.
func Signal(t *testing.T) (<-chan struct{}, func()) {
	t.Helper()
	ch := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(ch) }) }
	t.Cleanup(release)
	return ch, release
}

// Poll waits until an observer confirms a condition.
func Poll(t *testing.T, what string, observe func() bool) {
	t.Helper()
	deadline := time.Now().Add(ObservationTimeout)
	for !observe() {
		if time.Now().After(deadline) {
			t.Fatalf("did not observe %s within %v", what, ObservationTimeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// WaitOnRow observes a backend waiting for a specific row lock holder.
func WaitOnRow(t *testing.T, obs *pgx.Conn, waiter, blocker int32) {
	t.Helper()
	Poll(t, "a row lock blocked by the fixture transaction", func() bool {
		return Scalar[bool](t, obs, `
SELECT EXISTS (SELECT FROM pg_locks WHERE pid = $1 AND NOT granted AND locktype = 'transactionid')
       AND $2 = ANY (pg_blocking_pids($1))`, waiter, blocker)
	})
}

// Done closes when the task's result is available.
func (task *Task[V]) Done() <-chan struct{} { return task.done }

// Result returns the result of a completed task.
func (task *Task[V]) Result() V {
	<-task.done
	return task.result
}
