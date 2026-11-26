package store

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MaxWatchLag is the number of unacknowledged rows at which a subscriber is cut off.
const MaxWatchLag = 2048

const feedStaleAfter = 10 * time.Second

var (
	// ErrWatchLagged ends a subscription that fell too far behind.
	ErrWatchLagged = NewDomainError("The watch fell behind and must be reopened.")
	// ErrSubscriptionState rejects a call made in the wrong state.
	ErrSubscriptionState = NewDomainError("The subscription cannot do that now.")
)

// Follower keeps a bounded history of committed rows for all subscriptions.
type Follower struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	now    func() time.Time
	report func(error)

	mu             sync.Mutex
	subs           map[*Subscription]struct{}
	changed        chan struct{}
	initialHead    int64
	appliedThrough int64
	evictedThrough int64
	prunedThrough  int64
	ring           [MaxWatchLag]FeedRow
	first          int
	count          int
	lastAccepted   time.Time
	stale          bool
	recovered      chan struct{}
}

// Subscription tracks the acknowledged cursor and at most one delivered row.
type Subscription struct {
	follower *Follower
	ctx      context.Context
	cancel   context.CancelCauseFunc
	stop     func() bool

	started     bool
	waiting     bool
	cursor      int64
	outstanding int64
	lag         int
}

// StartFollower reads the initial head and starts following later feed entries.
func StartFollower(ctx context.Context, s *Store, report func(error)) (*Follower, error) {
	return startFollower(ctx, s, report, feedReaderOptions{})
}

// StartFollowerWithLifetime reads the head under ctx and follows the feed under lifetime.
func StartFollowerWithLifetime(ctx, lifetime context.Context, s *Store, report func(error)) (*Follower, error) {
	return startFollowerWithLifetime(ctx, lifetime, s, report, feedReaderOptions{})
}

func startFollower(ctx context.Context, s *Store, report func(error), options feedReaderOptions) (*Follower, error) {
	return startFollowerWithLifetime(ctx, ctx, s, report, options)
}

func startFollowerWithLifetime(ctx, lifetime context.Context, s *Store, report func(error), options feedReaderOptions) (*Follower, error) {
	var head int64
	if err := withFeedRead(ctx, s.pagePool, func(ctx context.Context, tx *ReadTx) error {
		var err error
		head, err = readHead(ctx, tx)
		return err
	}); err != nil {
		return nil, err
	}

	now := options.now
	if now == nil {
		now = time.Now
	}
	ctx, cancel := context.WithCancel(lifetime)
	f := &Follower{
		ctx:            ctx,
		cancel:         cancel,
		done:           make(chan struct{}),
		now:            now,
		report:         report,
		subs:           make(map[*Subscription]struct{}),
		changed:        make(chan struct{}),
		initialHead:    head,
		appliedThrough: head,
		lastAccepted:   now(),
		recovered:      make(chan struct{}, 1),
	}
	onReadError := options.onReadError
	options.onReadError = func(err error) {
		f.checkStaleness()
		if onReadError != nil {
			onReadError(err)
		}
	}

	go func() {
		defer close(f.done)
		if options.now == nil {
			checked := make(chan struct{})
			go func() {
				defer close(checked)
				f.watchStaleness()
			}()
			defer func() { <-checked }()
		}
		readFeed(ctx, s, head, options, f.acceptPage)
		f.mu.Lock()
		defer f.mu.Unlock()
		for sub := range f.subs {
			sub.endLocked(context.Cause(ctx))
		}
	}()
	return f, nil
}

// Close cancels the subscriptions and joins the reader and listener.
func (f *Follower) Close() {
	f.cancel()
	<-f.done
}

// Subscribe registers a stream before the stream reads its snapshot.
func (f *Follower) Subscribe(ctx context.Context) (*Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	if f.ctx.Err() != nil || f.stale {
		return nil, ErrFeedUnavailable
	}
	ctx, cancel := context.WithCancelCause(ctx)
	sub := &Subscription{follower: f, ctx: ctx, cancel: cancel}
	f.subs[sub] = struct{}{}
	sub.stop = context.AfterFunc(ctx, sub.Close)
	return sub, nil
}

// acceptPage validates the page before changing the cursor, freshness or any subscription.
func (f *Follower) acceptPage(page feedPage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	end, err := feedPageEnd(f.appliedThrough, page)
	if err != nil {
		return err
	}

	f.lastAccepted = f.now()
	if f.stale {
		f.stale = false
		select {
		case f.recovered <- struct{}{}:
		default:
		}
	}
	if page.PrunedThrough > f.prunedThrough {
		f.prunedThrough = page.PrunedThrough
		for sub := range f.subs {
			if sub.started && sub.cursor < f.prunedThrough {
				sub.endLocked(ErrWatchLagged)
			}
		}
	}
	for _, row := range page.Rows {
		f.appendRow(row)
	}
	f.appliedThrough = end
	if len(page.Rows) > 0 {
		close(f.changed)
		f.changed = make(chan struct{})
	}
	return nil
}

func (f *Follower) appendRow(row FeedRow) {
	if f.count == len(f.ring) {
		f.evictedThrough = f.ring[f.first].Position
		f.ring[f.first] = row
		f.first = (f.first + 1) % len(f.ring)
	} else {
		f.ring[(f.first+f.count)%len(f.ring)] = row
		f.count++
	}
	for sub := range f.subs {
		if sub.started && row.Position > sub.cursor {
			sub.lag++
			if sub.lag >= MaxWatchLag {
				sub.endLocked(ErrWatchLagged)
			}
		}
	}
}

func (f *Follower) indexAfter(position int64) int {
	return sort.Search(f.count, func(i int) bool {
		return f.ring[(f.first+i)%len(f.ring)].Position > position
	})
}

// checkStaleness cancels streams once per outage and returns the time left until expiry.
func (f *Follower) checkStaleness() time.Duration {
	f.mu.Lock()
	remaining := feedStaleAfter - f.now().Sub(f.lastAccepted)
	if remaining > 0 || f.stale || f.ctx.Err() != nil {
		f.mu.Unlock()
		return remaining
	}
	f.stale = true
	for sub := range f.subs {
		sub.endLocked(ErrFeedUnavailable)
	}
	f.mu.Unlock()
	if f.report != nil {
		f.report(ErrFeedUnavailable)
	}
	return 0
}

// watchStaleness enforces the production expiry even while a page is in progress.
func (f *Follower) watchStaleness() {
	for f.ctx.Err() == nil {
		if remaining := f.checkStaleness(); remaining > 0 {
			select {
			case <-f.ctx.Done():
				return
			case <-time.After(remaining):
			}
		} else {
			select {
			case <-f.ctx.Done():
				return
			case <-f.recovered:
			}
		}
	}
}

// Context ends when the stream is canceled and carries the reason it ended.
func (sub *Subscription) Context() context.Context { return sub.ctx }

// Start activates a subscription at the head read by its snapshot.
func (sub *Subscription) Start(after int64) error {
	f := sub.follower
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := context.Cause(sub.ctx); err != nil {
		return err
	}
	if sub.started {
		return ErrSubscriptionState
	}
	lag := f.count - f.indexAfter(after)
	if after < max(f.initialHead, f.evictedThrough, f.prunedThrough) || lag >= MaxWatchLag {
		sub.endLocked(ErrWatchLagged)
		return ErrWatchLagged
	}
	sub.started = true
	sub.cursor = after
	sub.lag = lag
	return nil
}

// Next waits for the next row and keeps it outstanding until Ack.
func (sub *Subscription) Next() (FeedRow, error) {
	f := sub.follower
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := context.Cause(sub.ctx); err != nil {
		return FeedRow{}, err
	}
	if !sub.started || sub.waiting || sub.outstanding != 0 {
		return FeedRow{}, ErrSubscriptionState
	}
	for {
		if err := context.Cause(sub.ctx); err != nil {
			return FeedRow{}, err
		}
		if i := f.indexAfter(sub.cursor); i < f.count {
			row := f.ring[(f.first+i)%len(f.ring)]
			sub.outstanding = row.Position
			return row, nil
		}
		changed := f.changed
		sub.waiting = true
		f.mu.Unlock()
		select {
		case <-changed:
		case <-sub.ctx.Done():
		}
		f.mu.Lock()
		sub.waiting = false
	}
}

// Ack advances the cursor past the outstanding row.
func (sub *Subscription) Ack(position int64) error {
	f := sub.follower
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := context.Cause(sub.ctx); err != nil {
		return err
	}
	if sub.outstanding == 0 || sub.outstanding != position {
		return ErrSubscriptionState
	}
	sub.cursor = position
	sub.outstanding = 0
	sub.lag--
	return nil
}

// Close cancels and unregisters the subscription.
func (sub *Subscription) Close() {
	f := sub.follower
	f.mu.Lock()
	defer f.mu.Unlock()
	sub.endLocked(context.Canceled)
}

func (sub *Subscription) endLocked(cause error) {
	delete(sub.follower.subs, sub)
	sub.stop()
	sub.cancel(cause)
}
