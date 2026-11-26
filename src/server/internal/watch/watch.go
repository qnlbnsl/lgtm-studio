// Package watch serves a typed snapshot followed by live changes from the store's feed.
package watch

import (
	"context"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// This file holds stubs of the test writer for item 9. The builder replaces every body.

type Source[T any] struct {
	Topic     string
	Scope     string
	Authorize func(context.Context, *store.ReadTx) error
	Snapshot  func(context.Context, *store.ReadTx) ([]T, error)
	Load      func(context.Context, *store.ReadTx, string) (T, bool, error)
}

type Snapshot[T any] struct {
	Records []T
}

var (
	ErrInvalidSource            = store.NewDomainError("The watch source is invalid.")
	ErrWatchUnauthorized        = store.NewDomainError("Sign in again to reopen this watch.")
	ErrAuthorizationUnavailable = store.NewDomainError("The watch authorization check is unavailable, try again.")
)

type Message[T any] struct {
	Position   int64
	Snapshot   *Snapshot[T]
	Saved      *T
	RemovedKey string
}

// Serve is a STUB of the test writer.
func Serve[T any](
	ctx context.Context,
	s *store.Store,
	follower *store.Follower,
	source Source[T],
	send func(context.Context, Message[T]) error,
) error {
	return serve(ctx, s, follower, source, send, nil)
}

// serve is a STUB of the test writer. A nil ticks channel uses the production 30-second ticker.
func serve[T any](
	ctx context.Context,
	s *store.Store,
	follower *store.Follower,
	source Source[T],
	send func(context.Context, Message[T]) error,
	ticks <-chan time.Time,
) error {
	return nil
}
