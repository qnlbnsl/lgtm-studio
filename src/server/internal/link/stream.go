package link

import (
	"context"
	"errors"
	"sync"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

const HeartbeatTimeout = 30 * time.Second

const watchdogIdentityTimeout = 2 * time.Second

func (s *Service) Open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	if s.streaming == nil {
		return (linkv1connect.UnimplementedLinkServiceHandler{}).Open(ctx, stream)
	}
	return s.open(ctx, stream, s.serve)
}

func (s *Service) serve(ctx context.Context, handle *Handle, frames <-chan receivedFrame, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	machineID, streamID := handle.machineID, handle.streamID
	hooks := s.streaming.options.Commands
	send := func(response *linkv1.OpenResponse) error {
		return transport.Send(ctx, func() error { return stream.Send(response) })
	}
	var session *commandSession
	var continuation <-chan struct{}
	if _, persistent := hooks.(*State); persistent {
		session = &commandSession{handle: handle, continuation: make(chan struct{}, 1)}
		continuation = session.continuation
		defer s.commandSlots.release(session)
	}
	sendCommand := func(command *linkv1.Command) error {
		writeContext := ctx
		if session != nil {
			writeContext = session.writeContext
		}
		return transport.Send(writeContext, func() error {
			return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: command}})
		})
	}
	replayComplete := handle.admissionMark >= handle.replayThrough
	checkIdentity, tryCommand := true, true
	for ctx.Err() == nil {
		select {
		case <-continuation:
			tryCommand = true
		default:
		}
		select {
		case <-handle.Wake():
			checkIdentity = true
		default:
		}
		if checkIdentity {
			if _, err := s.streaming.state.CheckCurrent(ctx, machineID, streamID); err != nil {
				return err
			}
			checkIdentity, tryCommand = false, true
		}

		var frame receivedFrame
		ready, open := true, true
		select {
		case frame, open = <-frames:
		default:
			if replayComplete && tryCommand {
				ready = false
				break
			}
			select {
			case frame, open = <-frames:
			case <-handle.Wake():
				checkIdentity = true
				continue
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		}
		if !open {
			return nil
		}
		if ctx.Err() != nil {
			break
		}
		if ready {
			ackedMark, err := s.dispatch(ctx, machineID, streamID, frame, send)
			if err != nil {
				return err
			}
			replayComplete = replayComplete || ackedMark >= handle.replayThrough
		}

		tryCommand = false
		if replayComplete && hooks != nil && ctx.Err() == nil {
			turn := ctx
			if session != nil {
				if !s.commandSlots.acquire(session) {
					continue
				}
				turn = context.WithValue(ctx, commandSessionKey{}, session)
				if hook := s.streaming.options.Seams.BeforeCommandSend; hook != nil {
					turn = context.WithValue(turn, afterCommandClaimKey{}, func(ctx context.Context, command *linkv1.Command) error {
						return hook(ctx, machineID, command.GetId())
					})
				}
			}
			var err error
			tryCommand, err = hooks.SendNext(turn, machineID, streamID, sendCommand)
			if session != nil {
				s.commandSlots.release(session)
			}
			if err != nil {
				return err
			}
		}
	}
	return context.Cause(ctx)
}

// dispatch returns the acknowledged event mark after sending the acknowledgement.
func (s *Service) dispatch(ctx context.Context, machineID, streamID string, frame receivedFrame, send func(*linkv1.OpenResponse) error) (uint64, error) {
	link := s.streaming
	if frame.err != nil {
		if malformedOpenFrame(frame.err) {
			return 0, link.checkFrame(ctx, machineID, streamID, frame)
		}
		return 0, errLinkFinished
	}
	switch kind := frame.request.GetKind().(type) {
	case *linkv1.OpenRequest_Heartbeat:
		if err := link.state.Heartbeat(ctx, machineID, streamID, kind.Heartbeat); err != nil {
			return 0, err
		}
		return 0, send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}})
	case *linkv1.OpenRequest_Event:
		result, err := link.state.applyEvent(ctx, machineID, streamID, kind.Event, link.options.Seams.BeforeEventCommit)
		if err != nil {
			return 0, err
		}
		if result.Applied {
			link.options.Logger.Info("link_event_applied", "machine_id", machineID, "sequence", result.Mark)
			if hook := link.options.Seams.BeforeEventAck; hook != nil {
				hook(ctx, machineID, result.Mark)
			}
		}
		if err := context.Cause(ctx); err != nil {
			return 0, err
		}
		if _, err := link.state.CheckCurrent(ctx, machineID, streamID); err != nil {
			return 0, err
		}
		ack := &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{EventAck: &linkv1.EventAck{Through: result.Mark}}}
		if err := send(ack); err != nil {
			return 0, err
		}
		return result.Mark, nil
	default:
		return 0, link.checkFrame(ctx, machineID, streamID, frame)
	}
}

func (s *streamingService) checkFrame(ctx context.Context, machineID, streamID string, frame receivedFrame) error {
	approval, err := s.state.CheckCurrent(ctx, machineID, streamID)
	if err != nil {
		return err
	}
	if frame.err != nil {
		return ErrInvalidMessage
	}
	switch kind := frame.request.GetKind().(type) {
	case *linkv1.OpenRequest_Hello, *linkv1.OpenRequest_Proof:
		return ErrInvalidMessage
	case *linkv1.OpenRequest_CommandResult:
		if approval != machines.ApprovalApproved {
			return nil
		}
		result := kind.CommandResult
		if !canonicalUUID.MatchString(result.GetCommandId()) || result.GetAttempt() == 0 {
			return ErrInvalidMessage
		}
		if s.options.Commands != nil {
			return s.options.Commands.AcceptResult(ctx, machineID, streamID, result)
		}
	}
	return nil
}

// The controls observe the reader and let tests drive its monotonic watchdog.
type streamControlsKey struct{}

type streamControls struct {
	now          func() time.Time
	timer        func(time.Duration) (<-chan time.Time, func())
	afterReceive func(*linkv1.OpenRequest, error)
	readerDone   func()
}

func steadyControls(ctx context.Context) streamControls {
	var controls streamControls
	if supplied, _ := ctx.Value(streamControlsKey{}).(*streamControls); supplied != nil {
		controls = *supplied
	}
	if controls.now == nil {
		controls.now = time.Now
	}
	if controls.timer == nil {
		controls.timer = func(wait time.Duration) (<-chan time.Time, func()) {
			timer := time.NewTimer(wait)
			return timer.C, func() { timer.Stop() }
		}
	}
	return controls
}

type streamWatchdog struct {
	controls streamControls
	mu       sync.Mutex
	deadline time.Time
}

func (w *streamWatchdog) refresh() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if now := w.controls.now(); now.Before(w.deadline) {
		w.deadline = now.Add(HeartbeatTimeout)
	}
}

func (w *streamWatchdog) run(ctx context.Context, cancel context.CancelCauseFunc, silenceCause func() error) {
	for ctx.Err() == nil {
		w.mu.Lock()
		wait := w.deadline.Sub(w.controls.now())
		w.mu.Unlock()
		if wait <= 0 {
			cancel(silenceCause())
			return
		}
		tick, stop := w.controls.timer(wait)
		select {
		case <-ctx.Done():
		case <-tick:
		}
		stop()
	}
}

func (s *streamingService) silenceCause(ctx context.Context, handle *Handle) error {
	read, stop := context.WithTimeout(ctx, watchdogIdentityTimeout)
	defer stop()
	_, err := s.state.CheckCurrent(read, handle.machineID, handle.streamID)
	switch {
	case err == nil:
		return errHandshakeDeadline
	case errors.Is(err, ErrUnknownMachine):
		cause := s.hub.removalCause(read, handle)
		if errors.Is(cause, context.DeadlineExceeded) {
			return errDatabaseUnavailable
		}
		return cause
	case errors.Is(err, ErrNotCurrent):
		return ErrNotCurrent
	default:
		return errDatabaseUnavailable
	}
}
