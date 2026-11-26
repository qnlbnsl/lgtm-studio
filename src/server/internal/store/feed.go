package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// This file contains test writer stubs for feed writes.

type Tx struct{}

func (tx *Tx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (tx *Tx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (tx *Tx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

const MaxWriteChanges = 256

var ErrTooManyChanges = NewDomainError("The write contains too many changes.")

type Change string

const (
	Saved   Change = "saved"
	Removed Change = "removed"
)

type ChangeRef struct {
	Topic  string
	Scope  string
	Key    string
	Change Change
}

type FeedRow struct {
	Position int64
	ChangeRef
	WrittenAt time.Time
}

// A fixture tracer can provide hooks on the Store's connections.
type feedTestHooks interface {
	afterConstraints(context.Context, pgx.Tx) error
	beforeCommit(context.Context, pgx.Tx) error
}

func (s *Store) Write(context.Context, func(context.Context, *Tx) error) error { return nil }

func (tx *Tx) Record(ChangeRef) error { return nil }
