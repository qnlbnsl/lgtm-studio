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

// WithWriteDeadline is a stub of the test writer.
func WithWriteDeadline(next http.Handler) http.Handler { return next }

// WithTimeout is a stub of the test writer.
func WithTimeout(next http.Handler, _ time.Duration) http.Handler { return next }

// Send is a stub of the test writer.
func Send(_ context.Context, _ func() error) error { return nil }
