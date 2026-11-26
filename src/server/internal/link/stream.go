package link

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// These declarations are stubs of the test writer.
const HeartbeatTimeout = 30 * time.Second

func (s *Service) Open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	if s.streaming == nil {
		return connect.NewError(connect.CodeUnimplemented, errors.New("The stream loop is unimplemented."))
	}
	return s.open(ctx, stream, s.serve)
}

func (*Service) serve(context.Context, *Handle, <-chan receivedFrame, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("The stream loop is unimplemented."))
}

// These optional controls are stubs of the test writer. They let tests observe
// a saturated reader and move the monotonic watchdog clock without a sleep.
// afterReceive runs after the watchdog refresh decision and before enqueue.
// readerDone runs before a terminal status can be written.
type streamControlsKey struct{}

type streamControls struct {
	now          func() time.Time
	timer        func(time.Duration) (<-chan time.Time, func())
	afterReceive func(*linkv1.OpenRequest, error)
	readerDone   func()
}
