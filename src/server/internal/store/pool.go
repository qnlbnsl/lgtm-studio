package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Stub of the test writer: every body here does nothing. The builder of work item 1 replaces this file.

type Store struct{}

type ReadTx struct{}

type ConnectionPolicy struct {
	AllowPrivateStackPostgres bool
}

type Error struct {
	SQLState   string
	Constraint string
}

type Outcome string

const (
	OutcomeRolledBack Outcome = "rolled_back"
	OutcomeUnknown    Outcome = "unknown"
)

func (e *Error) Error() string { return "" }

func (e *Error) Retryable() bool { return false }

func (e *Error) Outcome() Outcome { return "" }

func NewDomainError(message string) error { return nil }

func Open(ctx context.Context, config *pgxpool.Config, policy ConnectionPolicy) (*Store, error) {
	return &Store{}, nil
}

func (s *Store) Close() {}

func (s *Store) Read(ctx context.Context, read func(context.Context, *ReadTx) error) error {
	return nil
}

func (s *Store) withWrite(ctx context.Context, write func(context.Context, pgx.Tx) error) error {
	return nil
}

var errStub = errors.New("stub of the test writer")

func (tx *ReadTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errStub
}

func (tx *ReadTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, errStub
}

func (tx *ReadTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return stubRow{}
}

type stubRow struct{}

func (stubRow) Scan(dest ...any) error { return errStub }
