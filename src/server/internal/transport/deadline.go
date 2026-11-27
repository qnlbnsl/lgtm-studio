// Package transport provides controls for streaming responses.
package transport

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const SendTimeout = 10 * time.Second

var ErrNoDeadline = errors.New("The response has no write deadline setter.")

type deadlineKey struct{}

type writeDeadline struct {
	set     func(time.Time) error
	timeout time.Duration
}

// WithWriteDeadline provides the default timeout for each call to Send.
func WithWriteDeadline(next http.Handler) http.Handler {
	return WithTimeout(next, SendTimeout)
}

// WithTimeout makes the response's write deadline available to Send.
func WithTimeout(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline := writeDeadline{set: http.NewResponseController(w).SetWriteDeadline, timeout: timeout}
		ctx := context.WithValue(r.Context(), deadlineKey{}, deadline)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Send bounds one response write by the timeout and the supplied context.
func Send(ctx context.Context, send func() error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, ok := ctx.Value(deadlineKey{}).(writeDeadline)
	if !ok {
		return ErrNoDeadline
	}
	writeBy := time.Now().Add(deadline.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(writeBy) {
		writeBy = contextDeadline
	}
	if err := deadline.set(writeBy); err != nil {
		return err
	}

	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		_ = deadline.set(time.Now())
	})
	defer func() {
		var clearErr error
		if stop() {
			clearErr = deadline.set(time.Time{})
		} else {
			// The cancellation callback must finish before the handler releases the writer.
			<-done
		}
		if err != nil {
			return
		}
		if err = ctx.Err(); err == nil {
			err = clearErr
		}
	}()
	var flushErr error
	if stream, ok := ctx.Value(linkStreamKey{}).(*linkStream); ok {
		stream.writer.sendFlushErr = &flushErr
		defer func() { stream.writer.sendFlushErr = nil }()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := send(); err != nil {
		return err
	}
	return flushErr
}
