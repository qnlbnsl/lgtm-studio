package store

import (
	"context"
	"time"
)

// This file holds stubs of the test writer for item 7. The builder replaces every body.

// feedPollInterval is the production period of the reader's poll. STUB of the test writer.
const feedPollInterval = 0 * time.Second

// feedPageTimeout is the production budget of one page for acquisition and queries. STUB of the test writer.
const feedPageTimeout = 0 * time.Second

// ErrFeedUnavailable reports that the change feed cannot be read now. STUB of the test writer.
var ErrFeedUnavailable = NewDomainError("The change feed is unavailable, try again.")

type feedPage struct {
	PrunedThrough int64
	Rows          []FeedRow
}

type feedReaderOptions struct {
	ticks           <-chan time.Time
	now             func() time.Time
	disableListener bool
	onListen        func(int32)
	onReadError     func(error)
	afterAcquire    func(context.Context, *ReadTx, int64) error
	onPage          func(feedPage)
}

// Snapshot is a STUB of the test writer.
func (s *Store) Snapshot(ctx context.Context, load func(context.Context, *ReadTx) error) (int64, error) {
	return 0, nil
}

// readFeedPage is a STUB of the test writer.
func (s *Store) readFeedPage(ctx context.Context, after int64) (feedPage, error) {
	return feedPage{}, nil
}

// runFeedReader is a STUB of the test writer. It does nothing until the context ends.
func runFeedReader(ctx context.Context, s *Store, after int64, options feedReaderOptions, apply func(feedPage)) {
	<-ctx.Done()
}
