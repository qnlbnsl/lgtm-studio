package store

import "context"

// This file holds stubs of the test writer for item 8. The builder replaces every body.

// MaxWatchLag is the number of unacknowledged rows at which a subscriber is cut off. STUB of the test writer.
const MaxWatchLag = 2048

var (
	// ErrWatchLagged ends a subscription that fell too far behind. STUB of the test writer.
	ErrWatchLagged = NewDomainError("The watch fell behind and must be reopened.")
	// ErrSubscriptionState rejects a call made in the wrong state. STUB of the test writer.
	ErrSubscriptionState = NewDomainError("The subscription cannot do that now.")
)

// Follower is a STUB of the test writer.
type Follower struct{}

// Subscription is a STUB of the test writer.
type Subscription struct{ ctx context.Context }

// StartFollower is a STUB of the test writer.
func StartFollower(ctx context.Context, s *Store, report func(error)) (*Follower, error) {
	return startFollower(ctx, s, report, feedReaderOptions{})
}

// startFollower is a STUB of the test writer.
func startFollower(ctx context.Context, s *Store, report func(error), options feedReaderOptions) (*Follower, error) {
	return &Follower{}, nil
}

// acceptPage is the private acceptance path of every page. STUB of the test writer.
func (f *Follower) acceptPage(page feedPage) error { return nil }

func (f *Follower) Close() {}

func (f *Follower) Subscribe(ctx context.Context) (*Subscription, error) {
	return &Subscription{ctx: context.Background()}, nil
}

func (sub *Subscription) Context() context.Context { return sub.ctx }
func (sub *Subscription) Start(after int64) error  { return nil }
func (sub *Subscription) Next() (FeedRow, error)   { return FeedRow{}, nil }
func (sub *Subscription) Ack(position int64) error { return nil }
func (sub *Subscription) Close()                   {}
