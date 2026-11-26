package clienttest

import (
	"context"
	"io"
)

type Options struct {
	StateDir string
	Version  string
	Getenv   func(string) string
	Stderr   io.Writer
}

// Join is a stub of the test writer.
func Join(ctx context.Context, opts Options, address, token string) error { return nil }

// Run is a stub of the test writer.
func Run(ctx context.Context, opts Options) int { return 0 }
