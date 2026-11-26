package store

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// rfGateID is the advisory lock that the reader tests hold to stall a page.
const rfGateID int64 = 841941100902

// rfSeed commits count feed rows of one topic in one fixture transaction and returns the last position.
// The rows are inserted under the flush guard, so no notification is sent.
func rfSeed(t *testing.T, obs *pgx.Conn, count int, topic string) int64 {
	t.Helper()
	tx, err := obs.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	}()
	storetest.Exec(t, tx, "SET LOCAL lgtm.feed_flush = 'on'")
	storetest.Exec(t, tx, `
INSERT INTO feed (topic, scope, key, change)
SELECT $2, '', n::text, 'saved' FROM generate_series(1, $1::int) AS n ORDER BY n`, count, topic)
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("committing the seeded feed rows failed: %v", err)
	}
	return storetest.Scalar[int64](t, obs, "SELECT COALESCE(max(position), 0) FROM feed")
}

// rfPrune advances the floor to through and deletes the feed through it in one flagged transaction.
func rfPrune(t *testing.T, obs *pgx.Conn, through int64) {
	t.Helper()
	tx, err := obs.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	}()
	storetest.Exec(t, tx, "SET LOCAL lgtm.feed_prune = 'on'")
	storetest.Exec(t, tx, "UPDATE feed_retention SET pruned_through = $1 WHERE singleton", through)
	storetest.Exec(t, tx, "DELETE FROM feed WHERE position <= $1", through)
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("pruning the fixture feed failed: %v", err)
	}
}

// rfHarness runs one reader with channels for everything that its hooks report.
type rfHarness struct {
	t          *testing.T
	ticks      chan time.Time
	pages      chan feedPage
	readErrors chan error
	listening  chan int32
	task       *storetest.Task[struct{}]

	mu     sync.Mutex
	rows   []FeedRow
	floors []int64
}

// rfStart starts a reader. The tweak function adjusts the options before the start.
func rfStart(t *testing.T, s *Store, after int64, tweak func(*feedReaderOptions)) *rfHarness {
	t.Helper()
	h := &rfHarness{
		t:          t,
		ticks:      make(chan time.Time),
		pages:      make(chan feedPage, 4096),
		readErrors: make(chan error, 4096),
		listening:  make(chan int32, 16),
	}
	options := feedReaderOptions{
		ticks:       h.ticks,
		onListen:    func(pid int32) { h.listening <- pid },
		onReadError: func(err error) { h.readErrors <- err },
		onPage:      func(page feedPage) { h.pages <- page },
	}
	if tweak != nil {
		tweak(&options)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.task = storetest.Start(t, func() struct{} {
		runFeedReader(ctx, s, after, options, func(page feedPage) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.rows = append(h.rows, page.Rows...)
			h.floors = append(h.floors, page.PrunedThrough)
		})
		return struct{}{}
	})
	// Cleanups run last in, first out, so the reader stops before the task is joined.
	t.Cleanup(cancel)
	return h
}

// tick requests one poll.
func (h *rfHarness) tick() {
	h.t.Helper()
	select {
	case h.ticks <- time.Now():
	case <-time.After(storetest.ObservationTimeout):
		h.t.Fatalf("the reader did not accept a poll within %v", storetest.ObservationTimeout)
	}
}

func (h *rfHarness) applied() []FeedRow {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]FeedRow(nil), h.rows...)
}

// pump polls repeatedly until the condition holds, with an early exit.
func (h *rfHarness) pump(what string, done func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(storetest.ObservationTimeout)
	for !done() {
		if time.Now().After(deadline) {
			h.t.Fatalf("did not observe %s within %v", what, storetest.ObservationTimeout)
		}
		select {
		case h.ticks <- time.Now():
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// waitRows waits without polling until n rows have been applied.
func (h *rfHarness) waitRows(what string, n int) []FeedRow {
	h.t.Helper()
	storetest.Poll(h.t, what, func() bool { return len(h.applied()) >= n })
	return h.applied()
}

// requireQuiet fails if the reader applies another row within the quiet period.
func (h *rfHarness) requireQuiet(what string, rowsBefore int) {
	h.t.Helper()
	time.Sleep(storeTestQuiet)
	if got := len(h.applied()); got != rowsBefore {
		h.t.Fatalf("%s: %d rows were applied, want %d", what, got, rowsBefore)
	}
}

func rfFixture(t *testing.T) (*Store, *pgx.Conn) {
	t.Helper()
	s, _, obs := feedFixture(t, nil)
	return s, obs
}

func TestFeedReaderNotificationAndPolling(t *testing.T) {
	if feedPollInterval != 2*time.Second {
		t.Fatalf("the production poll interval is %v, want 2s", feedPollInterval)
	}
	s, obs := rfFixture(t)

	// A committed row arrives on the notification alone.
	h := rfStart(t, s, 0, nil)
	storetest.Receive(t, "the listener registration", h.listening)
	feedWrite(t, s, "a")
	rows := h.waitRows("a notified row without a poll", 1)
	if rows[0].Key != "a" || rows[0].Topic != "records" || rows[0].Change != Saved || rows[0].Position <= 0 {
		t.Fatalf("the notified row is %+v, want record a saved", rows[0])
	}
	first := rows[0].Position

	// With the listener disabled only a poll delivers.
	h2 := rfStart(t, s, first, func(o *feedReaderOptions) { o.disableListener = true })
	feedWrite(t, s, "b")
	h2.requireQuiet("a row arrived without a notification or a poll", 0)
	h2.tick()
	rows2 := h2.waitRows("a polled row without a listener", 1)
	if rows2[0].Key != "b" || rows2[0].Position <= first {
		t.Fatalf("the polled row is %+v, want record b above position %d", rows2[0], first)
	}
	select {
	case pid := <-h2.listening:
		t.Fatalf("a disabled listener registered backend %d", pid)
	default:
	}

	// A notification and a poll together deliver one row once.
	h3 := rfStart(t, s, rows2[0].Position, nil)
	storetest.Receive(t, "the second listener registration", h3.listening)
	feedWrite(t, s, "c")
	h3.tick()
	h3.waitRows("the row of the notification and the poll", 1)
	h3.tick()
	h3.requireQuiet("a second poll repeated an applied row", 1)
	if got := h3.applied(); got[0].Key != "c" {
		t.Fatalf("the delivered row is %+v, want record c", got[0])
	}
	if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM feed"); count != 3 {
		t.Fatalf("the feed holds %d rows, want 3", count)
	}
}

func TestFeedReaderCatchupAndRetry(t *testing.T) {
	s, cfg, obs := feedFixture(t, nil)
	const total = 1100
	last := rfSeed(t, obs, total, "records")

	var calls atomic.Int32
	killer := poolObserver(t, cfg)
	h := rfStart(t, s, 0, func(o *feedReaderOptions) {
		o.disableListener = true
		o.afterAcquire = func(ctx context.Context, tx *ReadTx, _ int64) error {
			if calls.Add(1) != 2 {
				return nil
			}
			var pid int32
			if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
				return err
			}
			// The second page loses its backend before its query runs.
			_, err := killer.Exec(context.Background(), "SELECT pg_terminate_backend($1)", pid)
			return err
		}
	})
	h.pump("all seeded rows applied after one lost backend", func() bool { return len(h.applied()) >= total })

	if got := len(h.readErrors); got < 1 {
		t.Fatal("the lost backend produced no read error")
	}
	rows := h.applied()
	if len(rows) != total {
		t.Fatalf("%d rows were applied, want exactly %d", len(rows), total)
	}
	for i, row := range rows {
		if i > 0 && row.Position <= rows[i-1].Position {
			t.Fatalf("row %d has position %d after %d: rows are repeated or out of order", i, row.Position, rows[i-1].Position)
		}
	}
	if rows[total-1].Position != last {
		t.Fatalf("the last applied position is %d, want %d", rows[total-1].Position, last)
	}
	// An empty accepted page proves that the reader reached the final cursor.
	for len(h.pages) > 0 {
		<-h.pages
	}
	deadline := time.Now().Add(storetest.ObservationTimeout)
	for {
		h.tick()
		select {
		case page := <-h.pages:
			if len(page.Rows) == 0 {
				h.requireQuiet("an empty page applied rows", total)
				return
			}
			t.Fatalf("a page after the last position holds %d rows", len(page.Rows))
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("did not observe an empty page after the last row")
		}
	}
}

func TestFeedReaderReadsAcrossPrunedPrefix(t *testing.T) {
	s, obs := rfFixture(t)
	last := rfSeed(t, obs, 1300, "records")
	rfPrune(t, obs, 600)

	for _, after := range []int64{0, 10, 599} {
		page, err := s.readFeedPage(t.Context(), after)
		if err != nil {
			t.Fatalf("reading after %d failed: %v", after, err)
		}
		if page.PrunedThrough != 600 {
			t.Fatalf("after %d the floor is %d, want 600", after, page.PrunedThrough)
		}
		if len(page.Rows) != 512 || page.Rows[0].Position != 601 || page.Rows[511].Position != 1112 {
			t.Fatalf("after %d the page holds %d rows from %d, want 512 rows from 601 to 1112",
				after, len(page.Rows), firstPosition(page))
		}
	}
	page, err := s.readFeedPage(t.Context(), 1100)
	if err != nil || len(page.Rows) != int(last-1100) || page.Rows[0].Position != 1101 {
		t.Fatalf("a page above position 1100 holds %d rows from %d (%v), want %d rows from 1101",
			len(page.Rows), firstPosition(page), err, last-1100)
	}
	// The cursor is exclusive: reading after the last position returns nothing.
	page, err = s.readFeedPage(t.Context(), last)
	if err != nil || len(page.Rows) != 0 || page.PrunedThrough != 600 {
		t.Fatalf("a page after the last position is %+v (%v), want no rows and floor 600", page, err)
	}

	// A reader whose cursor is below the floor starts above the floor and applies each row once.
	h := rfStart(t, s, 0, func(o *feedReaderOptions) { o.disableListener = true })
	h.pump("the pruned feed applied", func() bool { return len(h.applied()) >= int(last-600) })
	rows := h.applied()
	if len(rows) != int(last-600) || rows[0].Position != 601 || rows[len(rows)-1].Position != last {
		t.Fatalf("the reader applied %d rows from %d, want %d rows from 601 to %d", len(rows), rows[0].Position, last-600, last)
	}
}

func firstPosition(page feedPage) int64 {
	if len(page.Rows) == 0 {
		return 0
	}
	return page.Rows[0].Position
}

func TestFeedReaderBoundsStalledQuery(t *testing.T) {
	s, obs := rfFixture(t)
	var stall atomic.Bool
	h := rfStart(t, s, 0, func(o *feedReaderOptions) {
		o.disableListener = true
		o.afterAcquire = func(ctx context.Context, tx *ReadTx, _ int64) error {
			if !stall.Load() {
				return nil
			}
			// A real wait on a lock that the observer holds.
			_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::bigint)", rfGateID)
			return err
		}
	})

	rfSeed(t, obs, 1, "records")
	h.tick()
	applied := h.waitRows("the first row", 1)

	release := storetest.HoldAdvisoryGate(t, obs, rfGateID)
	stall.Store(true)
	rfSeed(t, obs, 1, "records")
	started := time.Now()
	h.tick()
	select {
	case err := <-h.readErrors:
		if err == nil {
			t.Fatal("the stalled page reported a nil error")
		}
	case <-time.After(storetest.ObservationTimeout):
		t.Fatalf("the stalled page did not time out within %v while the lock was held", storetest.ObservationTimeout)
	}
	if elapsed := time.Since(started); elapsed > 8*time.Second {
		t.Fatalf("the stalled page ended after %v, want within the two-second page budget plus rollback", elapsed)
	}
	if got := len(h.applied()); got != len(applied) {
		t.Fatalf("the stalled page applied rows: %d, want %d", got, len(applied))
	}

	// The next read resumes at the last applied position.
	release()
	stall.Store(false)
	h.pump("the row after the stall", func() bool { return len(h.applied()) >= 2 })
	rows := h.applied()
	h.requireQuiet("a row was repeated after the stall", 2)
	if rows[1].Position <= rows[0].Position || len(rows) != 2 {
		t.Fatalf("the rows after the stall are %+v, want two increasing positions", rows)
	}
}

func TestSnapshotRetainsPrunedHead(t *testing.T) {
	s, obs := rfFixture(t)

	loaded := 0
	head, err := s.Snapshot(t.Context(), func(context.Context, *ReadTx) error { loaded++; return nil })
	if err != nil || head != 0 || loaded != 1 {
		t.Fatalf("a new feed gave head %d, %d loads and %v, want head 0 and one load", head, loaded, err)
	}

	last := rfSeed(t, obs, 3, "records")
	head, err = s.Snapshot(t.Context(), func(context.Context, *ReadTx) error { return nil })
	if err != nil || head != last {
		t.Fatalf("a feed with rows gave head %d (%v), want %d", head, err, last)
	}

	rfPrune(t, obs, last)
	if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM feed"); count != 0 {
		t.Fatalf("%d feed rows remain after the prune", count)
	}
	head, err = s.Snapshot(t.Context(), func(context.Context, *ReadTx) error { return nil })
	if err != nil || head != last {
		t.Fatalf("a fully pruned feed gave head %d (%v), want the floor %d", head, err, last)
	}
}

func TestSnapshotReadsHeadBeforeLoad(t *testing.T) {
	s, obs := rfFixture(t)
	feedWrite(t, s, "a")
	headBefore := storetest.Scalar[int64](t, obs, "SELECT max(position) FROM feed")

	// A write that commits while the loader waits is outside the snapshot and its head.
	entered, enter := storetest.Signal(t)
	proceed, resume := storetest.Signal(t)
	task := storetest.Start(t, func() [2]any {
		var seen int
		head, err := s.Snapshot(t.Context(), func(ctx context.Context, tx *ReadTx) error {
			enter()
			<-proceed
			return tx.QueryRow(ctx, "SELECT count(*) FROM records").Scan(&seen)
		})
		return [2]any{head, []any{seen, err}}
	})
	storetest.Receive(t, "the loader to start", entered)
	feedWrite(t, s, "b")
	resume()
	result := task.Wait(t)
	detail := result[1].([]any)
	if detail[1] != nil || result[0].(int64) != headBefore || detail[0].(int) != 1 {
		t.Fatalf("the snapshot returned head %v, %v records and error %v, want head %d, 1 record and no error",
			result[0], detail[0], detail[1], headBefore)
	}
}

func TestFollowerProgressesUnderReadSaturation(t *testing.T) {
	const connections = 8
	// The pool has exactly eight connections whatever the number of processors.
	s, cfg, _ := feedFixtureSized(t, nil, connections)
	writer := storeTestOpen(t, cfg.Copy())

	// Hold every connection of the main pool with an ordinary read.
	hold, release := storetest.Signal(t)
	var started sync.WaitGroup
	started.Add(connections)
	for range connections {
		storetest.Start(t, func() error {
			return s.Read(context.Background(), func(ctx context.Context, tx *ReadTx) error {
				started.Done()
				<-hold
				return nil
			})
		})
	}
	waited := make(chan struct{})
	go func() { started.Wait(); close(waited) }()
	storetest.Receive(t, "eight held reads", waited)
	if acquired := s.pool.Stat().AcquiredConns(); acquired != connections {
		t.Fatalf("%d main pool connections are held, want %d", acquired, connections)
	}

	h := rfStart(t, s, 0, func(o *feedReaderOptions) { o.disableListener = true })
	feedWrite(t, writer, "saturated")
	h.pump("a page while the main pool is saturated", func() bool { return len(h.applied()) >= 1 })
	if got := h.applied()[0]; got.Key != "saturated" {
		t.Fatalf("the applied row is %+v, want record saturated", got)
	}
	release()
}

func TestSnapshotAdmissionLeavesReadCapacity(t *testing.T) {
	s, _ := rfFixture(t)

	entered, enter := storetest.Signal(t)
	hold, release := storetest.Signal(t)
	var loads atomic.Int32
	held := storetest.Start(t, func() error {
		_, err := s.Snapshot(context.Background(), func(ctx context.Context, tx *ReadTx) error {
			loads.Add(1)
			enter()
			<-hold
			return nil
		})
		return err
	})
	storetest.Receive(t, "the held snapshot loader", entered)
	if acquired := s.pool.Stat().AcquiredConns(); acquired != 1 {
		t.Fatalf("the held snapshot acquired %d connections, want 1", acquired)
	}

	const queued = 8
	type outcome struct {
		err     error
		elapsed time.Duration
	}
	results := make(chan outcome, queued)
	for range queued {
		go func() {
			start := time.Now()
			_, err := s.Snapshot(context.Background(), func(context.Context, *ReadTx) error {
				loads.Add(1)
				return nil
			})
			results <- outcome{err, time.Since(start)}
		}()
	}
	for range queued {
		got := storetest.Receive(t, "a queued snapshot to give up", results)
		if !errors.Is(got.err, ErrFeedUnavailable) {
			t.Fatalf("a queued snapshot returned %v, want ErrFeedUnavailable", got.err)
		}
		if got.elapsed > 8*time.Second {
			t.Fatalf("a queued snapshot waited %v, want about two seconds", got.elapsed)
		}
	}
	if n := loads.Load(); n != 1 {
		t.Fatalf("%d loaders ran while the slot was held, want 1", n)
	}
	if acquired := s.pool.Stat().AcquiredConns(); acquired != 1 {
		t.Fatalf("the queued snapshots left %d acquired connections, want 1", acquired)
	}

	// A session-style read and a write finish while the snapshot is held.
	poolReadOne(t, s, time.Second)
	feedWrite(t, s, "while-held")

	// A cancelled queued snapshot returns the caller's cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Snapshot(ctx, func(context.Context, *ReadTx) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled snapshot returned %v, want context.Canceled", err)
	}

	release()
	if err := held.Wait(t); err != nil {
		t.Fatalf("the held snapshot failed: %v", err)
	}
	if _, err := s.Snapshot(t.Context(), func(context.Context, *ReadTx) error { return nil }); err != nil {
		t.Fatalf("a snapshot after the release failed: %v", err)
	}

	// A failing loader and a panicking loader free the slot.
	loadErr := errors.New("loader failed")
	if _, err := s.Snapshot(t.Context(), func(context.Context, *ReadTx) error { return loadErr }); err == nil {
		t.Fatal("a failing loader returned no error")
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = s.Snapshot(t.Context(), func(context.Context, *ReadTx) error { panic("loader panic") })
	}()
	start := time.Now()
	if _, err := s.Snapshot(t.Context(), func(context.Context, *ReadTx) error { return nil }); err != nil || time.Since(start) > time.Second {
		t.Fatalf("a snapshot after a panicking loader returned %v after %v, want success within a second", err, time.Since(start))
	}
}
