package store

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// flReadFailure is the error that the fixture hook returns to make a page attempt fail at once.
var flReadFailure = errors.New("fixture read failure")

const (
	flModeNormal = iota
	flModeStall
	flModeFail
)

// flHarness runs one follower with a poll channel and a fixture clock.
type flHarness struct {
	t         *testing.T
	f         *Follower
	obs       *pgx.Conn
	ticks     chan time.Time
	pages     chan feedPage
	reports   chan error
	readErrs  chan error
	mode      atomic.Int32
	offset    atomic.Int64
	lastApply int64
}

var flBase = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

// flStart opens a fixture database and a follower that is driven only by explicit polls.
func flStart(t *testing.T) *flHarness {
	t.Helper()
	s, _, obs := feedFixture(t, nil)
	h := &flHarness{
		t:        t,
		obs:      obs,
		ticks:    make(chan time.Time),
		pages:    make(chan feedPage, 4096),
		reports:  make(chan error, 16),
		readErrs: make(chan error, 4096),
	}
	options := feedReaderOptions{
		ticks:           h.ticks,
		now:             func() time.Time { return flBase.Add(time.Duration(h.offset.Load())) },
		disableListener: true,
		onReadError:     func(err error) { h.readErrs <- err },
		onPage:          func(page feedPage) { h.pages <- page },
		afterAcquire: func(ctx context.Context, tx *ReadTx, _ int64) error {
			switch h.mode.Load() {
			case flModeStall:
				_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::bigint)", rfGateID)
				return err
			case flModeFail:
				return flReadFailure
			}
			return nil
		},
	}
	f, err := startFollower(t.Context(), s, func(err error) { h.reports <- err }, options)
	if err != nil {
		t.Fatalf("starting the follower failed: %v", err)
	}
	h.f = f
	t.Cleanup(f.Close)
	return h
}

func (h *flHarness) advance(d time.Duration) { h.offset.Add(int64(d)) }

func (h *flHarness) sendTick() {
	h.t.Helper()
	select {
	case h.ticks <- time.Now():
	case <-time.After(storetest.ObservationTimeout):
		h.t.Fatalf("the reader did not accept a poll within %v", storetest.ObservationTimeout)
	}
}

// receivePage waits for one accepted page and tracks the highest applied position.
func (h *flHarness) receivePage() feedPage {
	h.t.Helper()
	page := storetest.Receive(h.t, "an accepted page", h.pages)
	if n := len(page.Rows); n > 0 && page.Rows[n-1].Position > h.lastApply {
		h.lastApply = page.Rows[n-1].Position
	}
	return page
}

// tickPage sends one poll and returns after the drain of that poll ended with a short page.
func (h *flHarness) tickPage() {
	h.t.Helper()
	h.sendTick()
	for {
		if page := h.receivePage(); len(page.Rows) < 512 {
			return
		}
	}
}

// sync polls until the follower applied the target position.
func (h *flHarness) sync(target int64) {
	h.t.Helper()
	for tries := 0; h.lastApply < target; tries++ {
		if tries > 20 {
			h.t.Fatalf("the follower did not apply position %d, last applied %d", target, h.lastApply)
		}
		h.tickPage()
	}
}

// seed commits count rows and applies them.
func (h *flHarness) seed(count int, topic string) int64 {
	h.t.Helper()
	last := rfSeed(h.t, h.obs, count, topic)
	h.sync(last)
	return last
}

func (h *flHarness) subscribe() *Subscription {
	h.t.Helper()
	sub, err := h.f.Subscribe(h.t.Context())
	if err != nil {
		h.t.Fatalf("subscribing failed: %v", err)
	}
	h.t.Cleanup(sub.Close)
	return sub
}

func (h *flHarness) started(after int64) *Subscription {
	h.t.Helper()
	sub := h.subscribe()
	if err := sub.Start(after); err != nil {
		h.t.Fatalf("starting after %d failed: %v", after, err)
	}
	return sub
}

func flNext(t *testing.T, sub *Subscription) (FeedRow, error) {
	t.Helper()
	type result struct {
		row FeedRow
		err error
	}
	ch := make(chan result, 1)
	go func() {
		row, err := sub.Next()
		ch <- result{row, err}
	}()
	select {
	case got := <-ch:
		return got.row, got.err
	case <-time.After(storetest.ObservationTimeout):
		t.Fatalf("Next did not return within %v", storetest.ObservationTimeout)
	}
	return FeedRow{}, nil
}

// flConsume receives and acknowledges count rows and returns the last position.
func flConsume(t *testing.T, sub *Subscription, count int) int64 {
	t.Helper()
	var last int64
	for i := 0; i < count; i++ {
		row, err := flNext(t, sub)
		if err != nil {
			t.Fatalf("Next failed after %d rows: %v", i, err)
		}
		if row.Position <= last {
			t.Fatalf("row %d has position %d after %d", i, row.Position, last)
		}
		if err := sub.Ack(row.Position); err != nil {
			t.Fatalf("acknowledging %d failed: %v", row.Position, err)
		}
		last = row.Position
	}
	return last
}

func flRequireOpen(t *testing.T, what string, sub *Subscription) {
	t.Helper()
	if err := sub.Context().Err(); err != nil {
		t.Fatalf("%s was canceled (%v), want it open", what, context.Cause(sub.Context()))
	}
}

func flRequireEnded(t *testing.T, what string, sub *Subscription, want error) {
	t.Helper()
	if sub.Context().Err() == nil {
		t.Fatalf("%s is still open, want it canceled with %v", what, want)
	}
	if cause := context.Cause(sub.Context()); !errors.Is(cause, want) {
		t.Fatalf("%s was canceled with %v, want %v", what, cause, want)
	}
	if _, err := flNext(t, sub); !errors.Is(err, want) {
		t.Fatalf("Next of %s returned %v, want %v", what, err, want)
	}
}

func TestFollowerLagAt2048(t *testing.T) {
	if MaxWatchLag != 2048 {
		t.Fatalf("MaxWatchLag is %d, want 2048", MaxWatchLag)
	}

	t.Run("no Next", func(t *testing.T) {
		h := flStart(t)
		slow := h.started(0)
		fast := h.started(0)
		h.seed(1000, "records")
		flConsume(t, fast, 1000)
		h.seed(1046, "records")
		flRequireOpen(t, "the slow subscriber at lag 2046", slow)
		h.seed(1, "records")
		flRequireOpen(t, "the slow subscriber at lag 2047", slow)
		h.seed(1, "records")
		flRequireEnded(t, "the slow subscriber at lag 2048", slow, ErrWatchLagged)
		flRequireOpen(t, "the fast subscriber", fast)
		if last := flConsume(t, fast, 1048); last != 2048 {
			t.Fatalf("the fast subscriber ended at position %d, want 2048", last)
		}
	})

	t.Run("unacknowledged first row", func(t *testing.T) {
		h := flStart(t)
		slow := h.started(0)
		h.seed(1, "records")
		row, err := flNext(t, slow)
		if err != nil || row.Position != 1 {
			t.Fatalf("the first row is %+v (%v), want position 1", row, err)
		}
		h.seed(2045, "records")
		flRequireOpen(t, "the subscriber at lag 2046 with a row in flight", slow)
		h.seed(1, "records")
		flRequireOpen(t, "the subscriber at lag 2047 with a row in flight", slow)
		h.seed(1, "records")
		if slow.Context().Err() == nil {
			t.Fatal("the subscriber with a row in flight is open at lag 2048, want it canceled")
		}
		flRequireEnded(t, "the subscriber with a row in flight", slow, ErrWatchLagged)
	})

	t.Run("filtered rows", func(t *testing.T) {
		h := flStart(t)
		sub := h.started(0)
		h.seed(500, "other")
		flConsume(t, sub, 500)
		h.seed(1500, "records")
		h.seed(546, "other")
		flRequireOpen(t, "the subscriber at lag 2046 across topics", sub)
		h.seed(1, "other")
		flRequireOpen(t, "the subscriber at lag 2047 across topics", sub)
		h.seed(1, "records")
		flRequireEnded(t, "the subscriber at lag 2048 across topics", sub, ErrWatchLagged)
	})

	t.Run("a full page crosses the limit", func(t *testing.T) {
		h := flStart(t)
		slow := h.started(0)
		fast := h.started(0)
		h.seed(1600, "records")
		flConsume(t, fast, 1600)
		h.seed(440, "records")
		flRequireOpen(t, "the slow subscriber at lag 2040", slow)
		last := rfSeed(t, h.obs, 512, "records")
		h.sync(last)
		flRequireEnded(t, "the slow subscriber after the crossing page", slow, ErrWatchLagged)
		flRequireOpen(t, "the fast subscriber", fast)
		if got := flConsume(t, fast, 952); got != last {
			t.Fatalf("the fast subscriber ended at position %d, want %d", got, last)
		}
	})
}

func TestFollowerStartCountsRowsAlreadyApplied(t *testing.T) {
	h := flStart(t)
	h.seed(2048, "records")
	late := h.subscribe()
	if err := late.Start(0); !errors.Is(err, ErrWatchLagged) {
		t.Fatalf("starting at lag 2048 returned %v, want ErrWatchLagged", err)
	}
	flRequireEnded(t, "the late subscriber", late, ErrWatchLagged)
	edge := h.subscribe()
	if err := edge.Start(1); err != nil {
		t.Fatalf("starting at lag 2047 failed: %v", err)
	}
}

func TestFollowerBulkWriteCutsOffEveryone(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		open  bool
	}{{"2047 rows in one commit", 2047, true}, {"2048 rows in one commit", 2048, false}} {
		t.Run(tc.name, func(t *testing.T) {
			h := flStart(t)
			subs := []*Subscription{h.started(0), h.started(0), h.started(0)}
			h.seed(tc.count, "records")
			for i, sub := range subs {
				if tc.open {
					flRequireOpen(t, "a subscriber", sub)
				} else {
					flRequireEnded(t, "a subscriber", sub, ErrWatchLagged)
				}
				_ = i
			}
		})
	}
}

func TestFollowerPrunedCursor(t *testing.T) {
	t.Run("evicted history", func(t *testing.T) {
		h := flStart(t)
		old := h.subscribe()
		x := h.started(0)
		h.seed(10, "records")
		flConsume(t, x, 10)
		for range 3 {
			last := h.seed(1000, "records")
			if got := flConsume(t, x, 1000); got != last {
				t.Fatalf("the active subscriber ended at %d, want %d", got, last)
			}
		}
		head := h.seed(1, "records")
		row, err := flNext(t, x)
		if err != nil || row.Position != head {
			t.Fatalf("the in-flight row is %+v (%v), want position %d", row, err, head)
		}
		marker := head - 2048

		if err := old.Start(5); !errors.Is(err, ErrWatchLagged) {
			t.Fatalf("starting below the evicted history returned %v, want ErrWatchLagged", err)
		}
		flRequireEnded(t, "the subscriber below the evicted history", old, ErrWatchLagged)
		below := h.subscribe()
		if err := below.Start(marker - 1); !errors.Is(err, ErrWatchLagged) {
			t.Fatalf("starting one below the marker returned %v, want ErrWatchLagged", err)
		}
		// The marker passes the history check, and the full ring still leaves 2,048 rows above it.
		equal := h.subscribe()
		if err := equal.Start(marker); !errors.Is(err, ErrWatchLagged) {
			t.Fatalf("starting at the marker of a full ring returned %v, want ErrWatchLagged", err)
		}
		flRequireEnded(t, "the subscriber at the marker of a full ring", equal, ErrWatchLagged)
		inside := h.subscribe()
		if err := inside.Start(marker + 1); err != nil {
			t.Fatalf("starting one above the marker failed: %v", err)
		}
		if got, err := flNext(t, inside); err != nil || got.Position != marker+2 {
			t.Fatalf("the first row above the marker is %+v (%v), want position %d", got, err, marker+2)
		}
		flRequireOpen(t, "the subscriber with a row in flight", x)
		if err := x.Ack(head); err != nil {
			t.Fatalf("acknowledging the in-flight row failed: %v", err)
		}
	})

	t.Run("pruned floor", func(t *testing.T) {
		h := flStart(t)
		h.seed(20, "records")
		x := h.started(0)
		if row, err := flNext(t, x); err != nil || row.Position != 1 {
			t.Fatalf("the in-flight row is %+v (%v), want position 1", row, err)
		}
		z := h.started(0)
		flConsume(t, z, 20)
		v := h.started(10)

		rfPrune(t, h.obs, 10)
		flRequireOpen(t, "the subscriber with a row in flight before a poll", x)
		h.tickPage()
		flRequireEnded(t, "the subscriber with a row in flight behind the floor", x, ErrWatchLagged)
		flRequireOpen(t, "the subscriber ahead of the floor", z)
		flRequireOpen(t, "the subscriber at the floor", v)
		// Equality with the floor is usable: it passes the history check, and fewer than 2,048 rows lie above it.
		equal := h.subscribe()
		if err := equal.Start(10); err != nil {
			t.Fatalf("starting at the floor failed: %v", err)
		}
		if row, err := flNext(t, equal); err != nil || row.Position != 11 {
			t.Fatalf("the first row at the floor is %+v (%v), want position 11", row, err)
		}
		stale := h.subscribe()
		if err := stale.Start(5); !errors.Is(err, ErrWatchLagged) {
			t.Fatalf("starting below the floor returned %v, want ErrWatchLagged", err)
		}

		fresh := h.started(20)
		last := rfSeed(t, h.obs, 1, "records")
		h.sync(last)
		if row, err := flNext(t, fresh); err != nil || row.Position != last {
			t.Fatalf("a new subscriber at the head received %+v (%v), want position %d", row, err, last)
		}
	})
}

func TestFollowerIgnoresSequenceGaps(t *testing.T) {
	h := flStart(t)
	slow := h.started(0)

	// A rolled-back transaction burns 5,000 identity values.
	tx, err := h.obs.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	storetest.Exec(t, tx, "SET LOCAL lgtm.feed_flush = 'on'")
	storetest.Exec(t, tx, "INSERT INTO feed (topic, scope, key, change) SELECT 'records', '', n::text, 'saved' FROM generate_series(1, 5000) AS n")
	if err := tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	last := h.seed(1, "records")
	if last < 5001 {
		t.Fatalf("the committed row has position %d, want a gap above 5000", last)
	}
	flRequireOpen(t, "a subscriber behind a sequence gap", slow)
	if row, err := flNext(t, slow); err != nil || row.Position != last {
		t.Fatalf("the row behind the gap is %+v (%v), want position %d", row, err, last)
	}
	late := h.subscribe()
	if err := late.Start(last - 1); err != nil {
		t.Fatalf("starting inside the gap failed: %v", err)
	}
	if row, err := flNext(t, late); err != nil || row.Position != last {
		t.Fatalf("the late subscriber received %+v (%v), want position %d", row, err, last)
	}

	// A subscriber ahead of the reader ignores rows through its cursor.
	base := h.seed(3, "records")
	ahead := rfSeed(t, h.obs, 3, "records")
	sub := h.subscribe()
	if err := sub.Start(ahead - 1); err != nil {
		t.Fatalf("starting ahead of the reader failed: %v", err)
	}
	h.sync(ahead)
	if row, err := flNext(t, sub); err != nil || row.Position != ahead {
		t.Fatalf("the ahead subscriber received %+v (%v), want position %d", row, err, ahead)
	}
	if ahead != base+3 {
		t.Fatalf("the fixture produced positions %d and %d, want a difference of 3", base, ahead)
	}
}

func TestFollowerSubscriptionLifecycle(t *testing.T) {
	h := flStart(t)
	h.seed(50, "records")

	sub := h.subscribe()
	if _, err := flNext(t, sub); !errors.Is(err, ErrSubscriptionState) {
		t.Fatalf("Next before Start returned %v, want ErrSubscriptionState", err)
	}
	if err := sub.Ack(41); !errors.Is(err, ErrSubscriptionState) {
		t.Fatalf("Ack before Start returned %v, want ErrSubscriptionState", err)
	}
	if err := sub.Start(40); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := sub.Start(10); !errors.Is(err, ErrSubscriptionState) {
		t.Fatalf("a second Start returned %v, want ErrSubscriptionState", err)
	}
	if err := sub.Ack(41); !errors.Is(err, ErrSubscriptionState) {
		t.Fatalf("Ack without a row in flight returned %v, want ErrSubscriptionState", err)
	}
	if row, err := flNext(t, sub); err != nil || row.Position != 41 {
		t.Fatalf("the first row is %+v (%v), want position 41", row, err)
	}
	if _, err := flNext(t, sub); !errors.Is(err, ErrSubscriptionState) {
		t.Fatalf("a second Next returned %v, want ErrSubscriptionState", err)
	}
	for _, wrong := range []int64{40, 42, 0} {
		if err := sub.Ack(wrong); !errors.Is(err, ErrSubscriptionState) {
			t.Fatalf("Ack(%d) returned %v, want ErrSubscriptionState", wrong, err)
		}
	}
	if err := sub.Start(5); !errors.Is(err, ErrSubscriptionState) {
		t.Fatalf("Start with a row in flight returned %v, want ErrSubscriptionState", err)
	}
	if err := sub.Ack(41); err != nil {
		t.Fatalf("Ack(41) failed: %v", err)
	}
	if err := sub.Ack(41); !errors.Is(err, ErrSubscriptionState) {
		t.Fatalf("a repeated Ack returned %v, want ErrSubscriptionState", err)
	}
	if row, err := flNext(t, sub); err != nil || row.Position != 42 {
		t.Fatalf("the next row is %+v (%v), want position 42", row, err)
	}

	// Close ends a blocked wait and is idempotent.
	waiting := h.started(50)
	result := make(chan error, 1)
	go func() {
		_, err := waiting.Next()
		result <- err
	}()
	waiting.Close()
	waiting.Close()
	if err := storetest.Receive(t, "the blocked Next to end after Close", result); err == nil {
		t.Fatal("a blocked Next returned no error after Close")
	}
	if _, err := flNext(t, waiting); err == nil {
		t.Fatal("Next after Close returned no error")
	}

	// A canceled context ends a blocked wait.
	ctx, cancel := context.WithCancel(t.Context())
	canceled, err := h.f.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := canceled.Start(50); err != nil {
		t.Fatal(err)
	}
	go func() {
		_, err := canceled.Next()
		result <- err
	}()
	cancel()
	if err := storetest.Receive(t, "the blocked Next to end after cancellation", result); err == nil {
		t.Fatal("a blocked Next returned no error after its context ended")
	}

	// Closing the follower ends every wait and stops the reader.
	blocked := h.started(50)
	go func() {
		_, err := blocked.Next()
		result <- err
	}()
	h.f.Close()
	h.f.Close()
	if err := storetest.Receive(t, "the blocked Next to end after the follower closed", result); err == nil {
		t.Fatal("a blocked Next returned no error after the follower closed")
	}
	select {
	case h.ticks <- time.Now():
		t.Fatal("the reader accepted a poll after the follower closed")
	case <-time.After(storeTestQuiet):
	}
	if _, err := h.f.Subscribe(t.Context()); err == nil {
		t.Fatal("Subscribe on a closed follower returned no error")
	}
}

func TestFollowerEndsStaleStreams(t *testing.T) {
	requireNoReport := func(t *testing.T, h *flHarness) {
		t.Helper()
		if len(h.reports) != 0 {
			t.Fatalf("%d outage reports were made, want none", len(h.reports))
		}
	}
	requireRefusal := func(t *testing.T, h *flHarness) {
		t.Helper()
		if sub, err := h.f.Subscribe(t.Context()); !errors.Is(err, ErrFeedUnavailable) {
			if sub != nil {
				sub.Close()
			}
			t.Fatalf("Subscribe during an outage returned %v, want ErrFeedUnavailable", err)
		}
	}

	t.Run("empty pages keep the clock", func(t *testing.T) {
		h := flStart(t)
		sub := h.started(0)
		for range 12 {
			h.advance(time.Second)
			h.tickPage()
		}
		requireNoReport(t, h)
		flRequireOpen(t, "the subscriber after twelve seconds of empty pages", sub)
		h.subscribe()
	})

	t.Run("stalled database", func(t *testing.T) {
		h := flStart(t)
		existing := h.started(0)
		release := storetest.HoldAdvisoryGate(t, h.obs, rfGateID)
		h.mode.Store(flModeStall)
		rfSeed(t, h.obs, 1, "records")

		h.advance(11 * time.Second)
		h.sendTick()
		storetest.Receive(t, "the read error of the stalled page", h.readErrs)
		report := storetest.Receive(t, "the outage report", h.reports)
		if !errors.Is(report, ErrFeedUnavailable) {
			t.Fatalf("the outage report is %v, want ErrFeedUnavailable", report)
		}
		flRequireEnded(t, "the existing stream", existing, ErrFeedUnavailable)
		requireRefusal(t, h)

		// Later failures during the same outage make no second report.
		h.mode.Store(flModeFail)
		for range 2 {
			h.advance(time.Second)
			h.sendTick()
			storetest.Receive(t, "a read error during the outage", h.readErrs)
		}
		requireNoReport(t, h)
		requireRefusal(t, h)

		// An accepted page clears the outage and keeps the cursor at the applied position.
		release()
		h.mode.Store(flModeNormal)
		h.sync(1)
		recovered := h.started(0)
		if row, err := flNext(t, recovered); err != nil || row.Position != 1 {
			t.Fatalf("after recovery the first row is %+v (%v), want position 1", row, err)
		}
		requireNoReport(t, h)
	})

	t.Run("rejected page", func(t *testing.T) {
		h := flStart(t)
		existing := h.started(0)
		h.seed(1, "records")
		h.mode.Store(flModeFail)

		h.advance(9 * time.Second)
		repeated := feedPage{Rows: []FeedRow{{Position: 1, ChangeRef: ChangeRef{Topic: "records", Key: "1", Change: Saved}}}}
		if err := h.f.acceptPage(repeated); err == nil {
			t.Fatal("a page that repeats an applied position was accepted")
		}
		requireNoReport(t, h)
		flRequireOpen(t, "the stream after a rejected page", existing)

		h.advance(2 * time.Second)
		h.sendTick()
		storetest.Receive(t, "a read error after eleven seconds", h.readErrs)
		report := storetest.Receive(t, "the outage report after a rejected page", h.reports)
		if !errors.Is(report, ErrFeedUnavailable) {
			t.Fatalf("the outage report is %v, want ErrFeedUnavailable", report)
		}
		flRequireEnded(t, "the existing stream", existing, ErrFeedUnavailable)
		requireRefusal(t, h)
	})
}

// TestFollowerStoredLagOfALateStart proves that a subscriber that starts behind keeps the rows it already owes.
func TestFollowerStoredLagOfALateStart(t *testing.T) {
	h := flStart(t)
	h.seed(2000, "records")
	late := h.started(0)
	h.seed(47, "records")
	flRequireOpen(t, "the subscriber at lag 2047", late)
	h.seed(1, "records")
	flRequireEnded(t, "the subscriber at lag 2048 after a late start", late, ErrWatchLagged)
}

// TestFollowerProductionStalenessTimer runs the follower with the production clock, which starts the
// staleness timer. No page is ever accepted, so only the timer can end the stream after ten seconds.
func TestFollowerProductionStalenessTimer(t *testing.T) {
	s, _, _ := feedFixture(t, nil)
	reports := make(chan error, 4)
	// Nothing sends a poll, the listener is off, and the follower has no injected clock.
	options := feedReaderOptions{ticks: make(chan time.Time), disableListener: true}
	f, err := startFollower(t.Context(), s, func(err error) { reports <- err }, options)
	if err != nil {
		t.Fatalf("starting the follower failed: %v", err)
	}
	t.Cleanup(f.Close)
	sub, err := f.Subscribe(t.Context())
	if err != nil {
		t.Fatalf("subscribing failed: %v", err)
	}
	t.Cleanup(sub.Close)
	if err := sub.Start(0); err != nil {
		t.Fatalf("starting failed: %v", err)
	}
	flRequireOpen(t, "the stream before the deadline", sub)

	storetest.Poll(t, "the stream ended by the staleness timer", func() bool { return sub.Context().Err() != nil })
	if cause := context.Cause(sub.Context()); !errors.Is(cause, ErrFeedUnavailable) {
		t.Fatalf("the stream ended with %v, want ErrFeedUnavailable", cause)
	}
	if report := storetest.Receive(t, "the outage report", reports); !errors.Is(report, ErrFeedUnavailable) {
		t.Fatalf("the outage report is %v, want ErrFeedUnavailable", report)
	}
	if _, err := f.Subscribe(t.Context()); !errors.Is(err, ErrFeedUnavailable) {
		t.Fatalf("Subscribe during the outage returned %v, want ErrFeedUnavailable", err)
	}
}
