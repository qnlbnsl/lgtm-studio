package link

import (
	"context"
	"log/slog"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// serverClock is a stub of the test writer.
type serverClock struct{}

// commandHandler is a stub of the test writer.
type commandHandler struct{}

// scanCompletion is a stub of the test writer.
type scanCompletion struct{}

// newCommandHandler is a stub of the test writer.
func newCommandHandler(_ *Store, _ func(context.Context, bool) (bool, error), _ func(context.Context, *session, *linkv1.CommandResult) error, _ func() (time.Duration, error), _ *slog.Logger) *commandHandler {
	return &commandHandler{}
}

// setSession is a stub of the test writer.
func (*commandHandler) setSession(_ *session) {}

// standing is a stub of the test writer.
func (*commandHandler) standing() *linkv1.MachineState { return nil }

// handle is a stub of the test writer.
func (*commandHandler) handle(_ context.Context, _ *linkv1.Command) (restart bool, err error) {
	return false, nil
}

// completions is a stub of the test writer.
func (*commandHandler) completions() <-chan scanCompletion { return nil }

// finishScan is a stub of the test writer.
func (*commandHandler) finishScan(_ context.Context, _ scanCompletion) error { return nil }

// Decline is a stub of the test writer. Its implementation belongs in receipts.go.
func (*Store) Decline(_ Command, _ []byte) (Decision, []byte, error) { return 0, nil, nil }
