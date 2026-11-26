package store

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	feedPageSize             = 512
	feedPollInterval         = 2 * time.Second
	feedPageTimeout          = 2 * time.Second
	feedListenTimeout        = 3 * time.Second
	snapshotAdmissionTimeout = 2 * time.Second
)

// ErrFeedUnavailable reports that the change feed cannot be read now.
var ErrFeedUnavailable = NewDomainError("The change feed is unavailable, try again.")

var errFeedPageOrder = NewDomainError("The change feed page is out of order.")

// activeSnapshots holds a completion channel for each Store with a snapshot in progress.
var activeSnapshots sync.Map // map[*Store]chan struct{}

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

// Snapshot reads the feed head and loads records in one read-only repeatable read.
// Only one snapshot at a time can use the main pool.
func (s *Store) Snapshot(ctx context.Context, load func(context.Context, *ReadTx) error) (int64, error) {
	release, err := s.admitSnapshot(ctx)
	if err != nil {
		return 0, err
	}
	defer release()

	var head int64
	err = s.Read(ctx, func(ctx context.Context, tx *ReadTx) error {
		var err error
		head, err = readHead(ctx, tx)
		if err != nil {
			return err
		}
		return load(ctx, tx)
	})
	if err != nil {
		return 0, err
	}
	return head, nil
}

func (s *Store) admitSnapshot(ctx context.Context) (func(), error) {
	admission, cancel := context.WithTimeout(ctx, snapshotAdmissionTimeout)
	defer cancel()
	done := make(chan struct{})
	for admission.Err() == nil {
		active, busy := activeSnapshots.LoadOrStore(s, done)
		if !busy {
			return func() {
				activeSnapshots.Delete(s)
				close(done)
			}, nil
		}
		select {
		case <-admission.Done():
		case <-active.(chan struct{}):
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrFeedUnavailable
}

func readHead(ctx context.Context, tx *ReadTx) (int64, error) {
	var head int64
	err := tx.QueryRow(ctx, `
SELECT greatest(pruned_through, (SELECT max(position) FROM feed))
FROM feed_retention WHERE singleton`).Scan(&head)
	return head, err
}

func withFeedRead(ctx context.Context, pool *pgxpool.Pool, read func(context.Context, *ReadTx) error) error {
	ctx, cancel := context.WithTimeout(ctx, feedPageTimeout)
	defer cancel()
	return withTransaction(ctx, pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly},
		func(ctx context.Context, tx pgx.Tx) error { return read(ctx, &ReadTx{tx: tx}) })
}

// readFeedPage returns the floor and the next page from the same database view.
func (s *Store) readFeedPage(ctx context.Context, after int64) (feedPage, error) {
	return s.readFeedPageWithHook(ctx, after, nil)
}

func (s *Store) readFeedPageWithHook(
	ctx context.Context, after int64, afterAcquire func(context.Context, *ReadTx, int64) error,
) (feedPage, error) {
	var page feedPage
	err := withFeedRead(ctx, s.pagePool, func(ctx context.Context, tx *ReadTx) error {
		if afterAcquire != nil {
			if err := afterAcquire(ctx, tx, after); err != nil {
				return err
			}
		}
		if err := tx.QueryRow(ctx, "SELECT pruned_through FROM feed_retention WHERE singleton").Scan(&page.PrunedThrough); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
SELECT position, topic, scope, key, change, written_at FROM feed
WHERE position > $1 ORDER BY position ASC LIMIT $2`, max(after, page.PrunedThrough), feedPageSize)
		if err != nil {
			return err
		}
		page.Rows, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (FeedRow, error) {
			var entry FeedRow
			err := row.Scan(&entry.Position, &entry.Topic, &entry.Scope, &entry.Key, &entry.Change, &entry.WrittenAt)
			return entry, err
		})
		return err
	})
	if err != nil {
		return feedPage{}, err
	}
	return page, nil
}

// feedPageEnd returns the cursor that can be used once the whole page is accepted.
func feedPageEnd(after int64, page feedPage) (int64, error) {
	end := max(after, page.PrunedThrough)
	for _, row := range page.Rows {
		if row.Position <= end {
			return after, errFeedPageOrder
		}
		end = row.Position
	}
	return end, nil
}

func runFeedReader(ctx context.Context, s *Store, after int64, options feedReaderOptions, apply func(feedPage)) {
	readFeed(ctx, s, after, options, func(page feedPage) error {
		apply(page)
		return nil
	})
}

// readFeed owns the reader cursor and only advances it after accepting a complete page.
func readFeed(ctx context.Context, s *Store, after int64, options feedReaderOptions, accept func(feedPage) error) {
	ticks := options.ticks
	if ticks == nil {
		ticker := time.NewTicker(feedPollInterval)
		defer ticker.Stop()
		ticks = ticker.C
	}
	wake := make(chan struct{}, 1)
	if !options.disableListener {
		ctx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			listenForFeed(ctx, s.pool.Config(), wake, options.onListen)
		}()
		defer func() {
			cancel()
			<-done
		}()
	}
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case _, open := <-ticks:
			if !open {
				ticks = nil
				continue
			}
		case <-wake:
		}

		for ctx.Err() == nil {
			page, err := s.readFeedPageWithHook(ctx, after, options.afterAcquire)
			var end int64
			if err == nil {
				end, err = feedPageEnd(after, page)
			}
			if err == nil && ctx.Err() == nil {
				err = accept(page)
			}
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				if options.onReadError != nil {
					options.onReadError(err)
				}
				break
			}
			after = end
			if options.onPage != nil {
				options.onPage(page)
			}
			if len(page.Rows) < feedPageSize {
				break
			}
		}
	}
}

func listenForFeed(ctx context.Context, config *pgxpool.Config, wake chan<- struct{}, onListen func(int32)) {
	signal := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	for ctx.Err() == nil {
		conn, err := registerFeedListener(ctx, config)
		if err == nil {
			signal()
			if onListen != nil {
				onListen(int32(conn.PgConn().PID()))
			}
			for ctx.Err() == nil {
				if _, err := conn.WaitForNotification(ctx); err != nil {
					break
				}
				signal()
			}
			closeFeedListener(conn)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(feedPollInterval):
		}
	}
}

func registerFeedListener(ctx context.Context, config *pgxpool.Config) (*pgx.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, feedListenTimeout)
	defer cancel()

	connConfig := config.ConnConfig.Copy()
	if config.BeforeConnect != nil {
		if err := config.BeforeConnect(ctx, connConfig); err != nil {
			return nil, err
		}
	}
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, err
	}
	if config.AfterConnect != nil {
		if err := config.AfterConnect(ctx, conn); err != nil {
			closeFeedListener(conn)
			return nil, err
		}
	}
	if _, err := conn.Exec(ctx, "LISTEN lgtm_feed_changed"); err != nil {
		closeFeedListener(conn)
		return nil, err
	}
	return conn, nil
}

func closeFeedListener(conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
	defer cancel()
	_ = conn.Close(ctx)
}
