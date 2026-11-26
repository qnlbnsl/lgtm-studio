package link

import (
	"context"
	"time"
)

// supervise is a stub of the test writer.
func supervise(ctx context.Context, attempt func(context.Context, func()) error) error {
	return nil
}

// readElapsedClock is a stub of the test writer.
var readElapsedClock = func() (time.Duration, error) { return 0, nil }
