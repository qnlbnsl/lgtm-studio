package main

import (
	"context"
	"io"
)

// joinTokenCommand is a stub from the test writer.
func joinTokenCommand(ctx context.Context, args []string, lookup func(string) (string, bool), stdout, stderr io.Writer) int {
	return 0
}
