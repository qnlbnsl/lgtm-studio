// Package clienttest runs the client in process against a test server.
package clienttest

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/app"
)

// Options configures a client with an explicit disposable state folder.
type Options struct {
	StateDir string
	Version  string
	Getenv   func(string) string
	Stderr   io.Writer
}

// Join enrolls a client using the supplied state folder.
func Join(ctx context.Context, opts Options, address, token string) error {
	if !filepath.IsAbs(opts.StateDir) {
		return errors.New("clienttest: provide an absolute path to a disposable state folder")
	}
	return app.Join(ctx, app.Options(opts), address, token)
}

// Run blocks until the client stops and releases its state folder.
func Run(ctx context.Context, opts Options) int {
	if !filepath.IsAbs(opts.StateDir) {
		return 1
	}
	return app.Run(ctx, app.Options(opts))
}
