package store

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MaxWriteChanges is the largest number of feed references that one write may record.
const MaxWriteChanges = 256

// ErrTooManyChanges reports a write that recorded more than MaxWriteChanges references.
var ErrTooManyChanges = NewDomainError("The write contains too many changes.")

var errInvalidChange = NewDomainError("The change reference is invalid.")

// Change names what happened to a domain record.
type Change string

const (
	Saved   Change = "saved"
	Removed Change = "removed"
)

// ChangeRef points at a changed domain record without carrying its value.
// A blank scope denotes a server-wide record.
type ChangeRef struct {
	Topic  string
	Scope  string
	Key    string
	Change Change
}

// FeedRow is one committed feed entry.
type FeedRow struct {
	Position int64
	ChangeRef
	WrittenAt time.Time
}

// Tx exposes queries within the lifetime of a Store write callback.
// It buffers the recorded changes until the store flushes them before commit.
type Tx struct {
	tx      pgx.Tx
	changes []ChangeRef
	err     error
}

func (tx *Tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.tx.Exec(ctx, sql, args...)
	return tag, sanitizeError(ctx, err)
}

func (tx *Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.tx.Query(ctx, sql, args...)
	if rows != nil {
		rows = &readRows{Rows: rows, ctx: ctx}
	}
	return rows, sanitizeError(ctx, err)
}

func (tx *Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return readRow{Row: tx.tx.QueryRow(ctx, sql, args...), ctx: ctx}
}

// Record buffers a reference to a changed domain record.
// Its first error fails the whole write, even if the callback ignores it.
func (tx *Tx) Record(change ChangeRef) error {
	if tx.err != nil {
		return tx.err
	}
	switch {
	case len(tx.changes) >= MaxWriteChanges:
		tx.err = ErrTooManyChanges
	case change.Topic == "", change.Key == "",
		change.Change != Saved && change.Change != Removed,
		!validText(change.Topic), !validText(change.Scope), !validText(change.Key):
		tx.err = errInvalidChange
	default:
		tx.changes = append(tx.changes, change)
		return nil
	}
	return tx.err
}

// RecordOnce records a reference if this write has not already recorded it.
func (tx *Tx) RecordOnce(change ChangeRef) error {
	if tx.err != nil {
		return tx.err
	}
	if slices.Contains(tx.changes, change) {
		return nil
	}
	return tx.Record(change)
}

// validText reports whether PostgreSQL can store the value as text.
func validText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

// feedTestHooks lets a fixture tracer on the Store's connections interrupt a write.
// Production connections carry no tracer that implements it.
type feedTestHooks interface {
	afterConstraints(context.Context, pgx.Tx) error
	beforeCommit(context.Context, pgx.Tx) error
}

// Write calls write once inside a read-committed transaction.
// It then appends the recorded changes to the feed in the same transaction.
func (s *Store) Write(ctx context.Context, write func(context.Context, *Tx) error) error {
	return s.writeFeed(ctx, write)
}

// writeFeed runs write and flushes its recorded changes before commit.
// WriteOnce inserts its receipt inside write, so the receipt precedes the feed rows.
func (s *Store) writeFeed(ctx context.Context, write func(context.Context, *Tx) error) error {
	return s.withWrite(ctx, func(ctx context.Context, ptx pgx.Tx) error {
		tx := &Tx{tx: ptx}
		err := write(ctx, tx)
		if tx.err != nil {
			return tx.err
		}
		if err != nil {
			return err
		}
		hooks, _ := ptx.Conn().Config().Tracer.(feedTestHooks)
		if err := flushFeed(ctx, ptx, tx.changes, hooks); err != nil {
			return err
		}
		if hooks != nil {
			return hooks.beforeCommit(ctx, ptx)
		}
		return nil
	})
}

// flushFeed allocates feed positions under a transaction-level advisory lock.
// PostgreSQL releases the lock only when the transaction ends, so positions become visible in commit order.
func flushFeed(ctx context.Context, tx pgx.Tx, changes []ChangeRef, hooks feedTestHooks) error {
	if len(changes) == 0 {
		return nil
	}
	// Deferred domain checks run here, while this writer holds no feed lock.
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS ALL IMMEDIATE"); err != nil {
		return err
	}
	if hooks != nil {
		if err := hooks.afterConstraints(ctx, tx); err != nil {
			return err
		}
	}

	batch := &pgx.Batch{}
	batch.Queue("SELECT pg_advisory_xact_lock($1::bigint)", feedLockID) // lgtm:feed-lock
	batch.Queue("SELECT set_config('lgtm.feed_flush', 'on', true)")
	for _, change := range changes {
		batch.Queue("INSERT INTO feed (topic, scope, key, change) VALUES ($1, $2, $3, $4)",
			change.Topic, change.Scope, change.Key, string(change.Change))
	}
	batch.Queue("SELECT pg_notify('lgtm_feed_changed', '')")
	return tx.SendBatch(ctx, batch).Close()
}
