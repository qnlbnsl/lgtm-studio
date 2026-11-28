package app

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestStalledFolderSignInReadDoesNotBlockLink(t *testing.T) {
	const (
		budget      = 5 * time.Second
		allowance   = 2 * time.Second
		folderID    = "0195977c-0000-7000-8000-000000000a01"
		operationID = "0195977c-0000-7000-8000-000000000a02"
		restartID   = "0195977c-0000-7000-8000-000000000a03"
	)
	opts, _ := appOptions(t)
	opts.Stderr = io.Discard
	defaultOutgoingCredentials(t, opts, outgoingSignIn)
	readSignIn := claudeSignInReader(opts.Getenv)
	readEntered, releaseRead := make(chan time.Time, 1), make(chan struct{})
	var stalled atomic.Bool
	filter := link.NewRefreshingOutgoingFilter(func() []string {
		// Hold only the folder worker's file read. Admission, settlement,
		// and handoff can all read the same disposable sign-in normally.
		if isFolderWorkerSignInRead() && stalled.CompareAndSwap(false, true) {
			readEntered <- time.Now()
			<-releaseRead
		}
		return readSignIn()
	})
	useAppLink(t, func(ctx context.Context, options link.RunOptions) (link.RunOutcome, error) {
		options.Filter = filter
		options.Logger = newOutgoingLogger(opts.Stderr, filter)
		return link.Run(ctx, options)
	})

	ready := make(chan *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], 1)
	frames := make(chan *linkv1.OpenRequest, 16)
	var opened atomic.Bool
	var sendMu sync.Mutex
	streamEnded := false
	peer := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		if !opened.CompareAndSwap(false, true) {
			return errors.New("the client replaced the test's signed stream")
		}
		defer func() {
			// The handler writes trailers on return. Finish any test send first.
			sendMu.Lock()
			streamEnded = true
			close(frames)
			sendMu.Unlock()
		}()
		if err := p.welcome(stream, 0, true); err != nil {
			return err
		}
		ready <- stream
		for {
			frame, err := stream.Receive()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			frames <- frame
		}
	})
	seedApp(t, opts, peer, false)
	ctx, cancel := context.WithTimeout(t.Context(), 22*time.Second)
	done := make(chan struct{})
	var status int
	go func() {
		defer close(done)
		status = Run(ctx, opts)
	}()
	defer func() {
		close(releaseRead)
		cancel()
		<-done
	}()

	var stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]
	select {
	case stream = <-ready:
	case <-ctx.Done():
		t.Fatal("the peer could not open the signed stream")
	}
	send := func(frame *linkv1.OpenResponse) {
		t.Helper()
		sendMu.Lock()
		defer sendMu.Unlock()
		if streamEnded {
			t.Fatal("the signed stream ended before the fixture could send a frame")
		}
		if err := stream.Send(frame); err != nil {
			t.Fatalf("the fixture could not send a frame on the signed stream: %v", err)
		}
	}
	waitFrame := func(deadline time.Time, match func(*linkv1.OpenRequest) bool) bool {
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		for {
			select {
			case frame, ok := <-frames:
				if !ok {
					return false
				}
				if frame.GetHeartbeat() != nil {
					send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}})
				}
				if match(frame) {
					return true
				}
			case <-timer.C:
				return false
			}
		}
	}
	isHeartbeat := func(frame *linkv1.OpenRequest) bool { return frame.GetHeartbeat() != nil }
	if !waitFrame(time.Now().Add(allowance), isHeartbeat) {
		t.Fatal("the approved stream did not send its initial heartbeat")
	}
	sentAt := time.Now()
	send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: &linkv1.Command{
		Id: folderID, Attempt: 1, ExpiresAt: timestamppb.New(sentAt.Add(time.Minute)),
		Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
			OperationId: operationID, Path: "~/review-worker-hold",
		}},
	}}})
	var enteredAt time.Time
	select {
	case enteredAt = <-readEntered:
	case <-time.After(allowance):
		t.Fatal("the folder worker did not enter the blocked sign-in read")
	}

	var result *linkv1.CommandResult
	var event *linkv1.Event
	if waitFrame(enteredAt.Add(budget+allowance), func(frame *linkv1.OpenRequest) bool {
		if got := frame.GetCommandResult(); got.GetCommandId() == folderID {
			result = got
		}
		if got := frame.GetEvent(); got.GetFolderSeen().GetCommandId() == folderID {
			event = got
		}
		return result != nil && event != nil
	}) {
		seen := event.GetFolderSeen()
		if time.Since(sentAt) < budget || seen.GetOperationId() != operationID ||
			seen.GetRefusal() != linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE {
			t.Error("the timed-out inspection did not return its correlated not-readable refusal")
		}
		if result.GetAttempt() != 1 || result.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_DONE ||
			event.GetNumber() == 0 || result.GetLastEventNumber() != event.GetNumber() {
			t.Error("the timed-out inspection did not settle with the refusal's event number")
		}
	} else {
		t.Errorf("the timed-out inspection did not settle on the stream within %s of the blocked sign-in read", budget+allowance)
	}

	if !waitFrame(sentAt.Add(12*time.Second), isHeartbeat) {
		t.Error("the stream did not send its next heartbeat while the sign-in read was still blocked")
	}
	restartAt := time.Now()
	send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: &linkv1.Command{
		Id: restartID, Attempt: 1, ExpiresAt: timestamppb.New(restartAt.Add(time.Minute)),
		Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}},
	}}})
	if !waitFrame(restartAt.Add(allowance), func(frame *linkv1.OpenRequest) bool {
		got := frame.GetCommandResult()
		if got.GetCommandId() != restartID {
			return false
		}
		if got.GetAttempt() != 1 || got.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_DONE {
			t.Error("the restart did not return a done result for its current attempt")
		}
		return true
	}) {
		t.Error("the restart did not return its result while the sign-in read was still blocked")
	}
	select {
	case <-done:
		if status != 75 {
			t.Errorf("the application exited with status %d, want restart status 75", status)
		}
	case <-time.After(time.Until(restartAt.Add(3 * time.Second))):
		t.Error("the application did not exit for restart while the sign-in read was still blocked")
	}
}

func isFolderWorkerSignInRead() bool {
	var callers [24]uintptr
	frames := runtime.CallersFrames(callers[:runtime.Callers(2, callers[:])])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, "/internal/link.(*OutgoingFilter).folderPathUnsafe") {
			return true
		}
		if !more {
			return false
		}
	}
}
