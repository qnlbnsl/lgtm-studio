package watch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type wRec struct {
	ID    string
	Scope string
	Value string
}

const (
	authNormal int32 = iota
	authError
	authBlock
	authPanic
)

const (
	secretError = "secret-error-detail-7c1f"
	secretPanic = "secret-panic-value-9d3a"
)

// wFix is a disposable database, a store and a running follower.
type wFix struct {
	t        *testing.T
	s        *store.Store
	f        *store.Follower
	obs      *pgx.Conn
	authMode atomic.Int32

	mu          sync.Mutex
	beforeAuth  func()
	beforeLoad  func()
	afterFirst  func()
	probeReadOn string
	probeIso    string
}

func newFix(t *testing.T) *wFix {
	t.Helper()
	cfg := storetest.NewDatabase(t, "watch_fixture")
	s, err := store.Open(t.Context(), cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatalf("opening the store failed: %v", err)
	}
	t.Cleanup(s.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	obs, err := pgx.ConnectConfig(ctx, cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatalf("the observer could not connect: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = obs.Close(ctx)
	})
	if err := s.Migrate(t.Context()); err != nil {
		t.Fatalf("migrating the store failed: %v", err)
	}
	storetest.Exec(t, obs, "CREATE TABLE records (id text PRIMARY KEY, scope text NOT NULL, value text NOT NULL)")
	storetest.Exec(t, obs, "CREATE TABLE sessions (id text PRIMARY KEY, active boolean NOT NULL)")
	storetest.Exec(t, obs, "INSERT INTO sessions VALUES ('s1', true)")
	f, err := store.StartFollower(t.Context(), s, func(error) {})
	if err != nil {
		t.Fatalf("starting the follower failed: %v", err)
	}
	t.Cleanup(f.Close)
	return &wFix{t: t, s: s, f: f, obs: obs}
}

func (x *wFix) head() int64 {
	x.t.Helper()
	return storetest.Scalar[int64](x.t, x.obs, "SELECT COALESCE(max(position), 0) FROM feed")
}

func (x *wFix) write(fn func(ctx context.Context, tx *store.Tx) error) int64 {
	x.t.Helper()
	if err := x.s.Write(x.t.Context(), fn); err != nil {
		x.t.Fatalf("a fixture write failed: %v", err)
	}
	return x.head()
}

func ref(topic, scope, key string, change store.Change) store.ChangeRef {
	return store.ChangeRef{Topic: topic, Scope: scope, Key: key, Change: change}
}

// save upserts a record and returns the position of its change.
func (x *wFix) save(id, scope, value string) int64 {
	x.t.Helper()
	return x.write(func(ctx context.Context, tx *store.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO records VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET scope = excluded.scope, value = excluded.value`, id, scope, value); err != nil {
			return err
		}
		return tx.Record(ref("records", scope, id, store.Saved))
	})
}

func (x *wFix) remove(id, scope string) int64 {
	x.t.Helper()
	return x.write(func(ctx context.Context, tx *store.Tx) error {
		if _, err := tx.Exec(ctx, "DELETE FROM records WHERE id = $1", id); err != nil {
			return err
		}
		return tx.Record(ref("records", scope, id, store.Removed))
	})
}

// move changes the scope of a record and records a save in both scopes.
func (x *wFix) move(id, from, to string) int64 {
	x.t.Helper()
	return x.write(func(ctx context.Context, tx *store.Tx) error {
		if _, err := tx.Exec(ctx, "UPDATE records SET scope = $2 WHERE id = $1", id, to); err != nil {
			return err
		}
		if err := tx.Record(ref("records", from, id, store.Saved)); err != nil {
			return err
		}
		return tx.Record(ref("records", to, id, store.Saved))
	})
}

func (x *wFix) note(topic, scope, key string) int64 {
	x.t.Helper()
	return x.write(func(ctx context.Context, tx *store.Tx) error {
		return tx.Record(ref(topic, scope, key, store.Saved))
	})
}

// saveMany commits count records in one write.
func (x *wFix) saveMany(prefix string, count int) {
	x.t.Helper()
	x.write(func(ctx context.Context, tx *store.Tx) error {
		for i := 0; i < count; i++ {
			id := fmt.Sprintf("%s-%03d", prefix, i)
			if _, err := tx.Exec(ctx, "INSERT INTO records VALUES ($1, 'p1', $1)", id); err != nil {
				return err
			}
			if err := tx.Record(ref("records", "p1", id, store.Saved)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (x *wFix) setSession(active bool) {
	x.t.Helper()
	storetest.Exec(x.t, x.obs, "UPDATE sessions SET active = $1 WHERE id = 's1'", active)
}

func (x *wFix) authorize(ctx context.Context, tx *store.ReadTx) error {
	switch x.authMode.Load() {
	case authError:
		return errors.New(secretError)
	case authBlock:
		<-ctx.Done()
		return ctx.Err()
	case authPanic:
		panic(secretPanic)
	}
	x.mu.Lock()
	beforeAuth := x.beforeAuth
	x.mu.Unlock()
	if beforeAuth != nil {
		beforeAuth()
	}
	var active int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE id = 's1' AND active").Scan(&active); err != nil {
		return err
	}
	if active == 0 {
		return ErrWatchUnauthorized
	}
	return nil
}

func (x *wFix) source(scope string) Source[wRec] {
	return Source[wRec]{
		Topic:     "records",
		Scope:     scope,
		Authorize: x.authorize,
		Snapshot: func(ctx context.Context, tx *store.ReadTx) ([]wRec, error) {
			x.mu.Lock()
			before, after := x.beforeLoad, x.afterFirst
			x.mu.Unlock()
			if before != nil {
				before()
			}
			rows, err := tx.Query(ctx, "SELECT id, scope, value FROM records WHERE scope = $1 ORDER BY id", scope)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			var out []wRec
			for rows.Next() {
				var r wRec
				if err := rows.Scan(&r.ID, &r.Scope, &r.Value); err != nil {
					return nil, err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return nil, err
			}
			rows.Close()
			if after != nil {
				after()
			}
			var readOn, iso string
			if err := tx.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readOn); err != nil {
				return nil, err
			}
			if err := tx.QueryRow(ctx, "SHOW transaction_isolation").Scan(&iso); err != nil {
				return nil, err
			}
			x.mu.Lock()
			x.probeReadOn, x.probeIso = readOn, iso
			x.mu.Unlock()
			return out, nil
		},
		Load: func(ctx context.Context, tx *store.ReadTx, key string) (wRec, bool, error) {
			rows, err := tx.Query(ctx, "SELECT id, scope, value FROM records WHERE id = $1 AND scope = $2", key, scope)
			if err != nil {
				return wRec{}, false, err
			}
			defer rows.Close()
			if !rows.Next() {
				return wRec{}, false, rows.Err()
			}
			var r wRec
			if err := rows.Scan(&r.ID, &r.Scope, &r.Value); err != nil {
				return wRec{}, false, err
			}
			return r, true, nil
		},
	}
}

// run is one serving stream with its observable output.
type run struct {
	msgs   chan Message[wRec]
	causes chan error
	task   *storetest.Task[error]
	cancel context.CancelFunc
	sends  atomic.Int32
}

// blockSend decides what a send does before the message is delivered to the test.
type blockSend func(ctx context.Context, msg Message[wRec]) error

func (x *wFix) serve(source Source[wRec], ticks <-chan time.Time, block blockSend) *run {
	x.t.Helper()
	r := &run{msgs: make(chan Message[wRec], 4096), causes: make(chan error, 16)}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	x.t.Cleanup(cancel)
	send := func(ctx context.Context, msg Message[wRec]) error {
		r.sends.Add(1)
		if block != nil {
			if err := block(ctx, msg); err != nil {
				r.causes <- context.Cause(ctx)
				return err
			}
		}
		select {
		case r.msgs <- msg:
			return nil
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
	r.task = storetest.Start(x.t, func() error {
		return serve(ctx, x.s, x.f, source, send, ticks)
	})
	return r
}

func (r *run) next(t *testing.T, what string) Message[wRec] {
	t.Helper()
	return storetest.Receive(t, what, r.msgs)
}

// wait returns the error of Serve.
func (r *run) wait(t *testing.T, what string) error {
	t.Helper()
	select {
	case <-r.task.Done():
		return r.task.Result()
	case <-time.After(storetest.ObservationTimeout):
		t.Fatalf("Serve did not return for %s within %v", what, storetest.ObservationTimeout)
	}
	return nil
}

// requireVariant checks that exactly one variant of the message is present.
func requireVariant(t *testing.T, what string, msg Message[wRec], want string) {
	t.Helper()
	present := []string{}
	if msg.Snapshot != nil {
		present = append(present, "snapshot")
	}
	if msg.Saved != nil {
		present = append(present, "saved")
	}
	if msg.RemovedKey != "" {
		present = append(present, "removed")
	}
	if len(present) != 1 || present[0] != want {
		t.Fatalf("%s has variants %v, want exactly [%s]", what, present, want)
	}
}

func requireSaved(t *testing.T, msg Message[wRec], position int64, want wRec) {
	t.Helper()
	requireVariant(t, "a saved message", msg, "saved")
	if msg.Position != position || *msg.Saved != want {
		t.Fatalf("saved message is position %d record %+v, want position %d record %+v", msg.Position, *msg.Saved, position, want)
	}
}

func requireRemoved(t *testing.T, msg Message[wRec], position int64, key string) {
	t.Helper()
	requireVariant(t, "a removed message", msg, "removed")
	if msg.Position != position || msg.RemovedKey != key {
		t.Fatalf("removed message is position %d key %q, want position %d key %q", msg.Position, msg.RemovedKey, position, key)
	}
}

func requireSnapshot(t *testing.T, msg Message[wRec], position int64, want ...wRec) {
	t.Helper()
	requireVariant(t, "a snapshot message", msg, "snapshot")
	if msg.Position != position || len(msg.Snapshot.Records) != len(want) {
		t.Fatalf("snapshot is position %d with %d records, want position %d with %d records", msg.Position, len(msg.Snapshot.Records), position, len(want))
	}
	for i, got := range msg.Snapshot.Records {
		if got != want[i] {
			t.Fatalf("snapshot record %d is %+v, want %+v", i, got, want[i])
		}
	}
}

func TestWatchSnapshotUsesOneTransaction(t *testing.T) {
	t.Run("empty table", func(t *testing.T) {
		x := newFix(t)
		r := x.serve(x.source("p1"), nil, nil)
		requireSnapshot(t, r.next(t, "the empty snapshot"), 0)
	})

	for _, tc := range []struct {
		name string
		set  func(x *wFix, hook func())
	}{
		// The authorization query fixes the view of a repeatable-read transaction, so a write before it
		// separates a snapshot that shares one transaction from one that reads the head on its own.
		{"write before the first authorization query", func(x *wFix, hook func()) { x.beforeAuth = hook }},
		{"write before the first loader query", func(x *wFix, hook func()) { x.beforeLoad = hook }},
		{"write after the first loader query", func(x *wFix, hook func()) { x.afterFirst = hook }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newFix(t)
			old := x.save("old", "p1", "vo")
			var created atomic.Int64
			var once sync.Once
			tc.set(x, func() {
				once.Do(func() { created.Store(x.save("new", "p1", "vn")) })
			})
			r := x.serve(x.source("p1"), nil, nil)
			requireSnapshot(t, r.next(t, "the snapshot"), old, wRec{"old", "p1", "vo"})
			if created.Load() <= old {
				t.Fatalf("the competing write has position %d, want one above %d", created.Load(), old)
			}
			requireSaved(t, r.next(t, "the competing change"), created.Load(), wRec{"new", "p1", "vn"})
			x.mu.Lock()
			readOn, iso := x.probeReadOn, x.probeIso
			x.mu.Unlock()
			if readOn != "on" || iso != "repeatable read" {
				t.Fatalf("the snapshot transaction is read-only=%q isolation=%q, want read-only on and repeatable read", readOn, iso)
			}
		})
	}
}

func TestWatchFiltersAndLoadsCurrentRecords(t *testing.T) {
	x := newFix(t)
	entered, enter := storetest.Signal(t)
	release, letGo := storetest.Signal(t)
	hold := func(ctx context.Context, msg Message[wRec]) error {
		if msg.Snapshot == nil {
			return nil
		}
		enter()
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
	w1 := x.serve(x.source("p1"), nil, hold)
	w2 := x.serve(x.source("p2"), nil, nil)
	storetest.Receive(t, "the held snapshot send", entered)
	requireSnapshot(t, w2.next(t, "the p2 snapshot"), 0)

	// Writes complete while the sender is held.
	pA1 := x.save("a", "p1", "v1")
	pZ := x.save("z", "p2", "vz")
	x.note("other", "p1", "a")
	pA2 := x.save("a", "p1", "v2")
	pM := x.save("m", "p1", "vm")
	pB1 := x.save("b", "p1", "vb")
	pB2 := x.remove("b", "p1")
	letGo()

	requireSnapshot(t, w1.next(t, "the held snapshot"), 0)
	requireSaved(t, w1.next(t, "the first save of a"), pA1, wRec{"a", "p1", "v2"})
	requireSaved(t, w1.next(t, "the second save of a"), pA2, wRec{"a", "p1", "v2"})
	requireSaved(t, w1.next(t, "the marker"), pM, wRec{"m", "p1", "vm"})
	requireRemoved(t, w1.next(t, "a save of a missing record"), pB1, "b")
	requireRemoved(t, w1.next(t, "a removal"), pB2, "b")

	requireSaved(t, w2.next(t, "the p2 record"), pZ, wRec{"z", "p2", "vz"})

	pR := x.remove("a", "p1")
	requireRemoved(t, w1.next(t, "the removal of a"), pR, "a")

	// A move across scopes removes for the old watcher and saves for the new one.
	pS := x.save("mv", "p1", "vmv")
	pMove := x.move("mv", "p1", "p2")
	requireSaved(t, w1.next(t, "the save of mv"), pS, wRec{"mv", "p1", "vmv"})
	requireRemoved(t, w1.next(t, "the departure of mv"), pMove-1, "mv")
	requireSaved(t, w2.next(t, "the arrival of mv"), pMove, wRec{"mv", "p2", "vmv"})

	// Markers prove that nothing else was sent in between.
	e1 := x.save("end1", "p1", "e")
	e2 := x.save("end2", "p2", "e")
	requireSaved(t, w1.next(t, "the p1 end marker"), e1, wRec{"end1", "p1", "e"})
	requireSaved(t, w2.next(t, "the p2 end marker"), e2, wRec{"end2", "p2", "e"})
}

func TestWatchCanRestartAfterLag(t *testing.T) {
	x := newFix(t)
	var cause atomic.Value
	hold := func(ctx context.Context, msg Message[wRec]) error {
		<-ctx.Done()
		cause.Store(fmt.Sprint(context.Cause(ctx)))
		return context.Cause(ctx)
	}
	r := x.serve(x.source("p1"), nil, hold)
	// The held first send proves that the subscription is active before writes begin.
	storetest.Poll(t, "the held first send", func() bool { return r.sends.Load() >= 1 })
	for batch := 0; batch < 12; batch++ {
		select {
		case <-r.task.Done():
		default:
			x.saveMany(fmt.Sprintf("b%02d", batch), 256)
			continue
		}
		break
	}
	err := r.wait(t, "the lagging stream")
	if !errors.Is(err, store.ErrWatchLagged) {
		t.Fatalf("Serve returned %v, want ErrWatchLagged", err)
	}
	senderCause := storetest.Receive(t, "the sender's cause", r.causes)
	if !errors.Is(senderCause, store.ErrWatchLagged) {
		t.Fatalf("the sender saw the cause %v, want ErrWatchLagged", senderCause)
	}

	// Reopening gives a snapshot of the new records.
	total := storetest.Scalar[int](t, x.obs, "SELECT count(*) FROM records")
	reopened := x.serve(x.source("p1"), nil, nil)
	msg := reopened.next(t, "the snapshot after a restart")
	requireVariant(t, "the reopened snapshot", msg, "snapshot")
	if len(msg.Snapshot.Records) != total || msg.Position != x.head() {
		t.Fatalf("the reopened snapshot has %d records at position %d, want %d at %d", len(msg.Snapshot.Records), msg.Position, total, x.head())
	}

	// Caller cancellation ends the stream with the caller's error.
	blocked := x.serve(x.source("p1"), nil, hold)
	storetest.Poll(t, "the sender's entry", func() bool { return blocked.sends.Load() >= 1 })
	blocked.cancel()
	if err := blocked.wait(t, "a canceled caller"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve returned %v after cancellation, want context.Canceled", err)
	}
	if got := storetest.Receive(t, "the canceled sender's cause", blocked.causes); !errors.Is(got, context.Canceled) {
		t.Fatalf("the sender saw %v after cancellation, want context.Canceled", got)
	}
}

func TestWatchRejectsInvalidSource(t *testing.T) {
	x := newFix(t)
	good := x.source("p1")
	cases := map[string]func(s *Source[wRec]){
		"empty topic":    func(s *Source[wRec]) { s.Topic = "" },
		"nil authorize":  func(s *Source[wRec]) { s.Authorize = nil },
		"nil snapshot":   func(s *Source[wRec]) { s.Snapshot = nil },
		"nil load":       func(s *Source[wRec]) { s.Load = nil },
		"nil send":       nil,
		"empty and nils": func(s *Source[wRec]) { *s = Source[wRec]{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			source := good
			var sends atomic.Int32
			send := func(context.Context, Message[wRec]) error { sends.Add(1); return nil }
			if mutate != nil {
				mutate(&source)
			}
			var err error
			if name == "nil send" {
				err = Serve[wRec](t.Context(), x.s, x.f, source, nil)
			} else {
				err = Serve(t.Context(), x.s, x.f, source, send)
			}
			if !errors.Is(err, ErrInvalidSource) {
				t.Fatalf("Serve returned %v, want ErrInvalidSource", err)
			}
			if sends.Load() != 0 {
				t.Fatalf("the sender was called %d times, want none", sends.Load())
			}
		})
	}
}

func TestWatchRechecksAuthorization(t *testing.T) {
	t.Run("revoked before the snapshot", func(t *testing.T) {
		x := newFix(t)
		x.setSession(false)
		r := x.serve(x.source("p1"), nil, nil)
		if err := r.wait(t, "a revoked session"); !errors.Is(err, ErrWatchUnauthorized) {
			t.Fatalf("Serve returned %v, want ErrWatchUnauthorized", err)
		}
		if r.sends.Load() != 0 {
			t.Fatalf("%d messages were sent to a revoked session", r.sends.Load())
		}
	})

	t.Run("revoked before a matching change", func(t *testing.T) {
		x := newFix(t)
		r := x.serve(x.source("p1"), nil, nil)
		requireSnapshot(t, r.next(t, "the snapshot"), 0)
		x.setSession(false)
		x.save("late", "p1", "v")
		if err := r.wait(t, "a revoked session with a change"); !errors.Is(err, ErrWatchUnauthorized) {
			t.Fatalf("Serve returned %v, want ErrWatchUnauthorized", err)
		}
		if got := r.sends.Load(); got != 1 {
			t.Fatalf("%d messages were sent, want only the snapshot", got)
		}
	})

	t.Run("failing authorization before a matching change", func(t *testing.T) {
		x := newFix(t)
		r := x.serve(x.source("p1"), nil, nil)
		requireSnapshot(t, r.next(t, "the snapshot"), 0)
		x.authMode.Store(authPanic)
		x.save("late", "p1", "v")
		err := r.wait(t, "a panicking authorizer")
		requireUnavailable(t, err)
		if got := r.sends.Load(); got != 1 {
			t.Fatalf("%d messages were sent, want only the snapshot", got)
		}
	})

	for _, position := range []string{"idle", "blocked send"} {
		for _, cause := range []string{"revocation", "error", "timeout", "panic"} {
			t.Run(position+" "+cause, func(t *testing.T) {
				x := newFix(t)
				ticks := make(chan time.Time)
				var block blockSend
				if position == "blocked send" {
					block = func(ctx context.Context, msg Message[wRec]) error {
						<-ctx.Done()
						return context.Cause(ctx)
					}
				}
				r := x.serve(x.source("p1"), ticks, block)
				if position == "idle" {
					requireSnapshot(t, r.next(t, "the snapshot"), 0)
				} else {
					storetest.Poll(t, "the held snapshot send", func() bool { return r.sends.Load() == 1 })
				}
				switch cause {
				case "revocation":
					x.setSession(false)
				case "error":
					x.authMode.Store(authError)
				case "timeout":
					x.authMode.Store(authBlock)
				case "panic":
					x.authMode.Store(authPanic)
				}
				started := time.Now()
				select {
				case ticks <- time.Now():
				case <-time.After(storetest.ObservationTimeout):
					t.Fatalf("Serve did not accept a reauthorization tick within %v", storetest.ObservationTimeout)
				}
				err := r.wait(t, cause)
				if cause == "revocation" {
					if !errors.Is(err, ErrWatchUnauthorized) || errors.Is(err, ErrAuthorizationUnavailable) {
						t.Fatalf("Serve returned %v, want ErrWatchUnauthorized", err)
					}
				} else {
					requireUnavailable(t, err)
				}
				if cause == "timeout" {
					if elapsed := time.Since(started); elapsed > 10*time.Second {
						t.Fatalf("the authorization check took %v, want a cap of about two seconds", elapsed)
					}
				}
				if position == "blocked send" {
					got := storetest.Receive(t, "the sender's cancellation", r.causes)
					if got == nil {
						t.Fatal("the blocked sender was canceled without a cause")
					}
					requireNoLeak(t, got)
				}
				if got := r.sends.Load(); got != 1 {
					t.Fatalf("%d messages were sent, want only the snapshot", got)
				}
			})
		}
	}
}

func requireUnavailable(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrAuthorizationUnavailable) || errors.Is(err, ErrWatchUnauthorized) {
		t.Fatalf("Serve returned %v, want ErrAuthorizationUnavailable", err)
	}
	requireNoLeak(t, err)
}

func requireNoLeak(t *testing.T, err error) {
	t.Helper()
	for _, text := range []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
		if strings.Contains(text, secretError) || strings.Contains(text, secretPanic) {
			t.Fatalf("the error shows a raw cause: %q", text)
		}
	}
	for cause := errors.Unwrap(err); cause != nil; cause = errors.Unwrap(cause) {
		if strings.Contains(cause.Error(), secretError) || strings.Contains(cause.Error(), secretPanic) {
			t.Fatalf("the error wraps a raw cause: %q", cause.Error())
		}
	}
}
