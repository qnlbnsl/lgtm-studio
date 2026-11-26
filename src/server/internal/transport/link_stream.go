package transport

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// These declarations are stubs of the test writer.
var ErrLinkFrameTooLarge = errors.New("The link frame is too large.")

func WithLinkStream(next http.Handler, _ int) http.Handler              { return next }
func SendLinkWelcome(context.Context, func() error, func() error) error { return nil }
func CloseLinkRead(context.Context) error                               { return nil }
func BoundLinkEnd(context.Context, time.Duration)                       {}
