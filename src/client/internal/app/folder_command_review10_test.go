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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestStalledFolderSettlementSignInReadDoesNotBlockLink(t *testing.T) {
	const (
		initial     = "review10-synthetic-initial"
		rotated     = "review10-synthetic-rotated"
		folderID    = "0195977c-0000-7000-8000-000000000b01"
		operationID = "0195977c-0000-7000-8000-000000000b02"
		restartID   = "0195977c-0000-7000-8000-000000000b03"
		allowance   = 2 * time.Second
		readBudget  = 5 * time.Second
	)
	opts, log := appOptions(t)
	credentials := defaultOutgoingCredentials(t, opts, initial)
	readSignIn := claudeSignInReader(opts.Getenv)
	releaseReads := make(chan struct{})
	workerEntered := make(chan time.Time, 1)
	var workerHeld, settlementHeld atomic.Bool
	filter := link.NewRefreshingOutgoingFilter(func() []string {
		switch {
		case isFolderWorkerSignInRead() && workerHeld.CompareAndSwap(false, true):
			workerEntered <- time.Now()
			<-releaseReads
		case workerHeld.Load() && isFolderSettlementSignInRead():
			settlementHeld.Store(true)
			<-releaseReads
		}
		return readSignIn()
	})
	useAppLink(t, func(ctx context.Context, options link.RunOptions) (link.RunOutcome, error) {
		options.Filter = filter
		options.Logger = newOutgoingLogger(opts.Stderr, filter)
		return link.Run(ctx, options)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 22*time.Second)
	defer cancel()
	ready := make(chan *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], 1)
	frames := make(chan *linkv1.OpenRequest, 16)
	var opened atomic.Bool
	var sendMu sync.Mutex
	streamEnded := false
	peer := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		if !opened.CompareAndSwap(false, true) {
			return errors.New("the client replaced the signed stream")
		}
		defer func() {
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
			select {
			case frames <- frame:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	seedApp(t, opts, peer, false)
	done := make(chan struct{})
	var status int
	go func() {
		defer close(done)
		status = Run(ctx, opts)
	}()
	defer func() {
		close(releaseReads)
		cancel()
		<-done
	}()

	var stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]
	select {
	case stream = <-ready:
	case <-ctx.Done():
		t.Fatal("the peer could not open the signed stream")
	}
	send := func(frame *linkv1.OpenResponse) bool {
		t.Helper()
		sendMu.Lock()
		defer sendMu.Unlock()
		if streamEnded {
			return false
		}
		if err := stream.Send(frame); err != nil {
			t.Errorf("the signed peer could not send a frame: %v", err)
			return false
		}
		return true
	}
	sendCommand := func(command *linkv1.Command) bool {
		return send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: command}})
	}
	inspectFrame := func(frame *linkv1.OpenRequest) {
		t.Helper()
		wire, err := proto.Marshal(frame)
		if err != nil {
			t.Fatal("the peer could not encode the captured frame")
		}
		requireOutgoingSafe(t, "signed stream", wire, initial, rotated)
		if frame.GetEvent().GetFolderSeen() != nil {
			t.Error("the peer received a folder report while its screening was unavailable")
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
				inspectFrame(frame)
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
	var initialBeat time.Time
	var watermark uint64
	var gotFacts, gotInventory bool
	if !waitFrame(time.Now().Add(allowance), func(frame *linkv1.OpenRequest) bool {
		if frame.GetHeartbeat() != nil {
			initialBeat = time.Now()
		}
		if event := frame.GetEvent(); event != nil {
			watermark = event.GetNumber()
			gotFacts = gotFacts || event.GetFacts() != nil
			gotInventory = gotInventory || event.GetHarnessesFound() != nil
			send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{EventAck: &linkv1.EventAck{Through: watermark}}})
		}
		return !initialBeat.IsZero() && gotFacts && gotInventory
	}) || watermark == 0 {
		t.Fatal("the signed peer did not receive the initial heartbeat and startup reports")
	}
	if !sendCommand(&linkv1.Command{
		Id: folderID, Attempt: 1, ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
		Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
			OperationId: operationID, Path: "~/" + rotated,
		}},
	}) {
		t.Fatal("the signed peer could not send the folder command")
	}
	var enteredAt time.Time
	select {
	case enteredAt = <-workerEntered:
	case <-time.After(allowance):
		t.Fatal("the folder worker did not enter its blocked refresh")
	}
	if err := replaceOutgoingCredentials(credentials, rotated, "review10-synthetic-refresh"); err != nil {
		t.Fatal("could not replace the disposable sign-in file")
	}

	checkResult := func(got *linkv1.CommandResult, want linkv1.CommandOutcome) {
		t.Helper()
		if got.GetAttempt() != 1 || got.GetOutcome() != want || got.GetLastEventNumber() != watermark {
			t.Errorf("the result has attempt %d, outcome %s and watermark %d, want attempt 1, %s and watermark %d",
				got.GetAttempt(), got.GetOutcome(), got.GetLastEventNumber(), want, watermark)
		}
	}
	nextBeat := false
	if !waitFrame(enteredAt.Add(readBudget+allowance), func(frame *linkv1.OpenRequest) bool {
		nextBeat = nextBeat || frame.GetHeartbeat() != nil
		if got := frame.GetCommandResult(); got.GetCommandId() == folderID {
			checkResult(got, linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED)
			return true
		}
		return false
	}) {
		t.Errorf("the folder command missed its settlement limit with refreshes held (append refresh entered: %t)", settlementHeld.Load())
	}
	if !nextBeat && !waitFrame(initialBeat.Add(12*time.Second), func(frame *linkv1.OpenRequest) bool {
		return frame.GetHeartbeat() != nil
	}) {
		t.Error("the peer missed the next heartbeat with refreshes held")
	}
	restartAt := time.Now()
	if !sendCommand(&linkv1.Command{
		Id: restartID, Attempt: 1, ExpiresAt: timestamppb.New(restartAt.Add(time.Minute)),
		Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}},
	}) || !waitFrame(restartAt.Add(allowance), func(frame *linkv1.OpenRequest) bool {
		if got := frame.GetCommandResult(); got.GetCommandId() == restartID {
			checkResult(got, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE)
			return true
		}
		return false
	}) {
		t.Error("the peer missed the restart result with refreshes held")
	}
	select {
	case <-done:
		if status != 75 {
			t.Errorf("the application exited with status %d, want restart status 75", status)
		}
		requireOutgoingSafe(t, "process output", log.Bytes(), initial, rotated)
		timer := time.NewTimer(allowance)
		defer timer.Stop()
		for {
			select {
			case frame, ok := <-frames:
				if !ok {
					return
				}
				inspectFrame(frame)
			case <-timer.C:
				t.Error("the signed stream did not close after restart")
				return
			}
		}
	case <-time.After(time.Until(restartAt.Add(3 * time.Second))):
		t.Error("the application did not exit for restart with refreshes held")
	}
}

func isFolderSettlementSignInRead() bool {
	var callers [24]uintptr
	frames := runtime.CallersFrames(callers[:runtime.Callers(2, callers[:])])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, "/internal/link.(*Store).appendFilteredReport") {
			return true
		}
		if !more {
			return false
		}
	}
}
