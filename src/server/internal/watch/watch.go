// Package watch serves a typed snapshot followed by live changes from the store's feed.
package watch

import (
	"context"
	"errors"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const (
	authorizationTimeout  = 2 * time.Second
	authorizationInterval = 30 * time.Second
)

// Source reads records and checks authorization for one topic and scope.
// Authorize must honor its context and may run concurrently with another read.
type Source[T any] struct {
	Topic     string
	Scope     string
	Authorize func(context.Context, *store.ReadTx) error
	Snapshot  func(context.Context, *store.ReadTx) ([]T, error)
	Load      func(context.Context, *store.ReadTx, string) (T, bool, error)
}

// Snapshot holds the complete list, including when the list is empty.
type Snapshot[T any] struct {
	Records []T
}

var (
	ErrInvalidSource            = store.NewDomainError("The watch source is invalid.")
	ErrWatchUnauthorized        = store.NewDomainError("Sign in again to reopen this watch.")
	ErrAuthorizationUnavailable = store.NewDomainError("The watch authorization check is unavailable, try again.")
)

// Message carries one snapshot, saved record or removed key at a feed position.
type Message[T any] struct {
	Position   int64
	Snapshot   *Snapshot[T]
	Saved      *T
	RemovedKey string
}

// Serve sends a snapshot and later matching changes until the stream ends.
// The send callback must honor cancellation of the context it receives.
func Serve[T any](
	ctx context.Context,
	s *store.Store,
	follower *store.Follower,
	source Source[T],
	send func(context.Context, Message[T]) error,
) error {
	return serve(ctx, s, follower, source, send, nil)
}

func serve[T any](
	ctx context.Context,
	s *store.Store,
	follower *store.Follower,
	source Source[T],
	send func(context.Context, Message[T]) error,
	ticks <-chan time.Time,
) (err error) {
	if source.Topic == "" || source.Authorize == nil || source.Snapshot == nil || source.Load == nil || send == nil {
		return ErrInvalidSource
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	sub, err := follower.Subscribe(ctx)
	if err != nil {
		return err
	}
	defer sub.Close()
	ctx = sub.Context()
	var authorizationDone chan struct{}
	defer func() {
		if cause := context.Cause(ctx); cause != nil {
			err = cause
		}
		cancel(err)
		if authorizationDone != nil {
			<-authorizationDone
		}
	}()

	var records []T
	head, err := s.Snapshot(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		checkCtx, finish := context.WithTimeout(ctx, authorizationTimeout)
		defer finish()
		if err := authorize(checkCtx, tx, source.Authorize); err != nil {
			return err
		}
		var err error
		records, err = source.Snapshot(ctx, tx)
		return err
	})
	if err != nil {
		return err
	}
	if err := sub.Start(head); err != nil {
		return err
	}

	authorizationDone = make(chan struct{})
	go func() {
		defer close(authorizationDone)
		reauthorize(ctx, s, source.Authorize, cancel, ticks)
	}()
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if err := send(ctx, Message[T]{Position: head, Snapshot: &Snapshot[T]{Records: records}}); err != nil {
		return err
	}

	for {
		row, err := sub.Next()
		if err != nil {
			return err
		}
		if row.Topic == source.Topic && row.Scope == source.Scope {
			message, err := readChange(ctx, s, source, row)
			if err != nil {
				return err
			}
			if err := context.Cause(ctx); err != nil {
				return err
			}
			if err := send(ctx, message); err != nil {
				return err
			}
		}
		if err := sub.Ack(row.Position); err != nil {
			return err
		}
	}
}

func readChange[T any](ctx context.Context, s *store.Store, source Source[T], row store.FeedRow) (Message[T], error) {
	// Bound acquisition and authorization. The record load uses the stream context.
	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()
	checkCtx, finishCheck := context.WithTimeout(readCtx, authorizationTimeout)
	defer finishCheck()
	stopDeadline := context.AfterFunc(checkCtx, cancelRead)
	defer stopDeadline()

	message := Message[T]{Position: row.Position, RemovedKey: row.Key}
	authorized := false
	err := s.Read(readCtx, func(_ context.Context, tx *store.ReadTx) error {
		if err := authorize(checkCtx, tx, source.Authorize); err != nil {
			return err
		}
		if !stopDeadline() || checkCtx.Err() != nil {
			return ErrAuthorizationUnavailable
		}
		finishCheck()
		authorized = true

		if row.Change == store.Saved {
			record, found, err := source.Load(readCtx, tx, row.Key)
			if err != nil {
				return err
			}
			if found {
				message.Saved = &record
				message.RemovedKey = ""
			}
		}
		return nil
	})
	if err != nil && !authorized {
		err = authorizationError(err)
	}
	return message, err
}

func reauthorize(
	ctx context.Context,
	s *store.Store,
	check func(context.Context, *store.ReadTx) error,
	cancel context.CancelCauseFunc,
	ticks <-chan time.Time,
) {
	if ticks == nil {
		ticker := time.NewTicker(authorizationInterval)
		defer ticker.Stop()
		ticks = ticker.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case _, open := <-ticks:
			if !open {
				return
			}
		}
		checkCtx, finish := context.WithTimeout(ctx, authorizationTimeout)
		err := s.Read(checkCtx, func(ctx context.Context, tx *store.ReadTx) error {
			return authorize(ctx, tx, check)
		})
		finish()
		if err != nil {
			cancel(authorizationError(err))
			return
		}
	}
}

func authorize(
	ctx context.Context, tx *store.ReadTx, check func(context.Context, *store.ReadTx) error,
) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrAuthorizationUnavailable
		}
	}()
	if ctx.Err() != nil {
		return ErrAuthorizationUnavailable
	}
	err = check(ctx, tx)
	if ctx.Err() != nil {
		return ErrAuthorizationUnavailable
	}
	if err != nil {
		return authorizationError(err)
	}
	return nil
}

func authorizationError(err error) error {
	if errors.Is(err, ErrWatchUnauthorized) {
		return ErrWatchUnauthorized
	}
	return ErrAuthorizationUnavailable
}
