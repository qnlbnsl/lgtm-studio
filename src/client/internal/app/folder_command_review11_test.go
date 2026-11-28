package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

const (
	review11SignInValue = "review11-synthetic-sign-in"
	review11Rotated     = "review11-synthetic-rotated"
	review11Refresh     = "review11-synthetic-refresh"
	review11RestartID   = "0195977c-0000-7000-8000-000000000c03"
)

type review11SignInStall struct {
	path    string
	read    func() []string
	hold    func()
	release func()
	entered <-chan struct{}
}

func newReview11SignInStall(t *testing.T) review11SignInStall {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".credentials.json")
	if err := replaceOutgoingCredentials(path, review11SignInValue, review11Refresh); err != nil {
		t.Fatal(err)
	}
	read := claudeSignInReader(func(name string) string {
		if name == "CLAUDE_CONFIG_DIR" {
			return dir
		}
		return ""
	})
	held, released, entered := make(chan struct{}), make(chan struct{}), make(chan struct{})
	enter := sync.OnceFunc(func() { close(entered) })
	release := sync.OnceFunc(func() { close(released) })
	t.Cleanup(release)
	return review11SignInStall{
		path: path, entered: entered, release: release,
		hold: sync.OnceFunc(func() { close(held) }),
		read: func() []string {
			select {
			case <-held:
				enter()
				<-released
			default:
			}
			return read()
		},
	}
}

type review11Run struct {
	done    chan struct{}
	outcome link.RunOutcome
	status  int
}

func startReview11Run(t *testing.T, ctx context.Context, opts Options, stall review11SignInStall) *review11Run {
	t.Helper()
	run := &review11Run{done: make(chan struct{})}
	filter := link.NewRefreshingOutgoingFilter(stall.read)
	useAppLink(t, func(ctx context.Context, options link.RunOptions) (link.RunOutcome, error) {
		options.Filter = filter
		options.Logger = newOutgoingLogger(opts.Stderr, filter)
		outcome, err := link.Run(ctx, options)
		run.outcome = outcome
		return outcome, err
	})
	go func() {
		defer close(run.done)
		run.status = Run(ctx, opts)
	}()
	return run
}

type review11Stream = connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]

func receiveReview11Frame(stream *review11Stream) (*linkv1.OpenRequest, error) {
	frame, err := stream.Receive()
	if err == nil && frame.GetHeartbeat() != nil {
		err = stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}})
	}
	return frame, err
}

func sendReview11Command(stream *review11Stream, command *linkv1.Command) error {
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: command}})
}

func sendReview11Restart(stream *review11Stream) error {
	return sendReview11Command(stream, &linkv1.Command{
		Id: review11RestartID, Attempt: 1, ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
		Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}},
	})
}

func ackReview11Event(stream *review11Stream, through uint64) error {
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{EventAck: &linkv1.EventAck{Through: through}}})
}

func inspectReview11Frame(t *testing.T, frame *linkv1.OpenRequest) {
	t.Helper()
	wire, err := proto.Marshal(frame)
	if err != nil {
		t.Errorf("could not encode a captured frame: %v", err)
		return
	}
	requireOutgoingSafe(t, "signed stream", wire, review11SignInValue, review11Rotated, review11Refresh)
}

func TestPersistentSignInReadStallKeepsSignedControlPathReview11(t *testing.T) {
	const folderID = "0195977c-0000-7000-8000-000000000c01"
	const operationID = "0195977c-0000-7000-8000-000000000c02"
	opts, log := appOptions(t)
	if err := os.Mkdir(filepath.Join(opts.Getenv("HOME"), review11Rotated), 0o700); err != nil {
		t.Fatal(err)
	}
	stall := newReview11SignInStall(t)
	opts.OnFolderInspectionStart = func(_ context.Context, operation, command string) {
		if operation != operationID || command != folderID {
			t.Error("the folder worker received unexpected correlation identifiers")
		}
		if err := replaceOutgoingCredentials(stall.path, review11Rotated, review11Refresh); err != nil {
			t.Errorf("could not replace the disposable sign-in file: %v", err)
		}
		stall.hold()
	}

	type observation struct {
		folder, restart                  *linkv1.CommandResult
		watermark                        uint64
		initialBeat, nextBeat, restartAt time.Time
		folderDelay, restartDelay        time.Duration
	}
	observed := make(chan observation, 1)
	var opened atomic.Bool
	peer := newAppPeer(t, nil, func(p *appPeer, stream *review11Stream) error {
		if !opened.CompareAndSwap(false, true) {
			return connect.NewError(connect.CodeUnavailable, errors.New("the original signed stream ended"))
		}
		var seen observation
		defer func() { observed <- seen }()
		if err := p.welcome(stream, 0, true); err != nil {
			return fmt.Errorf("signing the initial stream: %w", err)
		}
		var facts, inventory bool
		for seen.initialBeat.IsZero() || !facts || !inventory {
			frame, err := receiveReview11Frame(stream)
			if err != nil {
				return err
			}
			inspectReview11Frame(t, frame)
			if frame.GetHeartbeat() != nil {
				seen.initialBeat = time.Now()
			}
			if event := frame.GetEvent(); event != nil {
				facts = facts || event.GetFacts() != nil
				inventory = inventory || event.GetHarnessesFound() != nil
				seen.watermark = event.GetNumber()
				if err := ackReview11Event(stream, seen.watermark); err != nil {
					return err
				}
			}
		}

		folderAt := time.Now()
		if err := sendReview11Command(stream, &linkv1.Command{
			Id: folderID, Attempt: 1, ExpiresAt: timestamppb.New(folderAt.Add(time.Minute)),
			Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
				OperationId: operationID, Path: "~/" + review11Rotated,
			}},
		}); err != nil {
			return err
		}
		for {
			frame, err := receiveReview11Frame(stream)
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			inspectReview11Frame(t, frame)
			if frame.GetEvent() != nil {
				t.Errorf("the signed peer received event %d while the folder report could not be screened",
					frame.GetEvent().GetNumber())
			}
			if frame.GetHeartbeat() != nil && seen.nextBeat.IsZero() {
				seen.nextBeat = time.Now()
			}
			result := frame.GetCommandResult()
			switch result.GetCommandId() {
			case folderID:
				seen.folder = result
				seen.folderDelay = time.Since(folderAt)
			case review11RestartID:
				seen.restart = result
				seen.restartDelay = time.Since(seen.restartAt)
			}
			if !seen.nextBeat.IsZero() && seen.restartAt.IsZero() &&
				(seen.folder != nil || time.Since(folderAt) >= 8*time.Second) {
				seen.restartAt = time.Now()
				if err := sendReview11Restart(stream); err != nil {
					return err
				}
			}
		}
	})
	seedApp(t, opts, peer, false)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	run := startReview11Run(t, ctx, opts, stall)
	defer func() {
		cancel()
		stall.release()
		<-run.done
	}()

	var seen observation
	select {
	case seen = <-observed:
	case <-ctx.Done():
		t.Fatal("the signed stream did not finish while sign-in reads were held")
	}
	select {
	case <-stall.entered:
	default:
		t.Fatal("no sign-in file read reached the persistent stall")
	}
	if seen.watermark == 0 || seen.initialBeat.IsZero() {
		t.Fatal("the signed peer did not establish the initial heartbeat and report watermark")
	}
	if result := seen.folder; result == nil || result.GetAttempt() != 1 ||
		result.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED ||
		result.GetLastEventNumber() != seen.watermark || seen.folderDelay > 8*time.Second {
		t.Errorf("the folder command did not return failed at watermark %d within its allowance: %v",
			seen.watermark, result)
	}
	if seen.nextBeat.IsZero() || seen.nextBeat.Sub(seen.initialBeat) > 13*time.Second {
		t.Error("the signed stream missed its next heartbeat while all sign-in reads were held")
	}
	if result := seen.restart; result == nil || result.GetAttempt() != 1 ||
		result.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_DONE ||
		result.GetLastEventNumber() != seen.watermark || seen.restartDelay > 3*time.Second {
		t.Errorf("the signed peer missed a timely done restart result at watermark %d: %v", seen.watermark, result)
	}
	select {
	case <-run.done:
		if run.status != 75 || run.outcome != link.RunRestart {
			t.Errorf("the application returned status %d and outcome %v, want restart and status 75", run.status, run.outcome)
		}
		requireOutgoingSafe(t, "application output", log.Bytes(), review11SignInValue, review11Rotated, review11Refresh)
	case <-time.After(4 * time.Second):
		t.Error("the application did not exit for restart while all sign-in reads were held")
	}
}

func TestReconnectRetriesUnavailableSignInScreeningReview11(t *testing.T) {
	opts, log := appOptions(t)
	stall := newReview11SignInStall(t)
	retained := []string{"review11-retained-first", "review11-retained-second"}
	var opens atomic.Int32
	var prefixDelivered atomic.Bool
	recovered := make(chan error, 1)
	peer := newAppPeer(t, nil, func(p *appPeer, stream *review11Stream) (err error) {
		leg := opens.Add(1)
		switch leg {
		case 2:
			stall.hold()
		case 3:
			stall.release()
		}
		if leg >= 3 {
			defer func() {
				select {
				case recovered <- err:
				default:
				}
			}()
		}
		// Waiting streams replay the retained prefix without publishing startup reports.
		if leg < 3 {
			if err := p.welcome(stream, 0, false); err != nil {
				return err
			}
		} else {
			if err := p.challenge(stream); err != nil {
				return err
			}
			frame, err := stream.Receive()
			if err != nil {
				return err
			}
			if frame.GetProof() == nil {
				return errors.New("missing proof")
			}
			welcome := &linkv1.Welcome{
				State: &linkv1.MachineState{
					Approval: linkv1.Approval_APPROVAL_APPROVED,
					Name:     "test machine",
					Revision: 2,
				},
				ServerTime:      timestamppb.Now(),
				LinkRevision:    2,
				ConnectionToken: "fixture-stream",
			}
			if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: welcome}}); err != nil {
				return err
			}
		}

		var through uint64
		restartSent := false
		for {
			frame, err := receiveReview11Frame(stream)
			if err != nil {
				if errors.Is(err, io.EOF) && leg >= 3 {
					return errors.New("the recovered stream ended before verifying the replay and restart")
				}
				return err
			}
			inspectReview11Frame(t, frame)
			if event := frame.GetEvent(); event != nil {
				if leg == 2 {
					t.Errorf("the stalled reconnect sent retained event %d before screening was available", event.GetNumber())
					return errors.New("the stalled reconnect sent an unscreened event")
				}
				through++
				if event.GetNumber() != through {
					return fmt.Errorf("on signed stream %d, got event %d, want %d", leg, event.GetNumber(), through)
				}
				if through <= uint64(len(retained)) {
					if event.GetFacts().GetClientVersion() != retained[through-1] {
						return fmt.Errorf("on signed stream %d, the retained report at %d changed", leg, through)
					}
					if leg == 1 && through == uint64(len(retained)) {
						// Drop the stream with both committed reports still unacknowledged.
						prefixDelivered.Store(true)
						return connect.NewError(connect.CodeUnavailable, errors.New("fixture disconnect"))
					}
				}
				if leg >= 3 {
					if err := ackReview11Event(stream, through); err != nil {
						return err
					}
					if !restartSent && through > uint64(len(retained)) {
						if err := sendReview11Restart(stream); err != nil {
							return err
						}
						restartSent = true
					}
				}
			}
			if result := frame.GetCommandResult(); result.GetCommandId() == review11RestartID {
				if !restartSent || result.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_DONE {
					return errors.New("the recovered stream did not accept restart after replaying the retained prefix")
				}
				return nil
			}
		}
	})
	seedApp(t, opts, peer, false)
	seedReview11RetainedFacts(t, opts.StateDir, retained)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	run := startReview11Run(t, ctx, opts, stall)
	defer func() {
		cancel()
		stall.release()
		<-run.done
	}()

	var replayErr error
	select {
	case replayErr = <-recovered:
	case <-run.done:
		select {
		case replayErr = <-recovered:
		default:
			replayErr = fmt.Errorf("the application exited with outcome %v and status %d after %d stream attempts",
				run.outcome, run.status, opens.Load())
		}
	case <-ctx.Done():
		replayErr = ctx.Err()
	}
	if !prefixDelivered.Load() {
		t.Error("the initial signed stream did not deliver both unacknowledged fixture events")
	}
	if replayErr != nil {
		t.Errorf("the signed peer did not retry the stalled reconnect and recover ordered replay: %v", replayErr)
	}
	select {
	case <-stall.entered:
	default:
		t.Error("the reconnect never tried to refresh the held sign-in file")
	}

	select {
	case <-run.done:
		if run.outcome == link.RunRecoveryStop {
			t.Error("unavailable sign-in screening sent the intact store to the recovery stop")
		} else if run.outcome != link.RunRestart || run.status != 75 {
			t.Errorf("the application returned status %d and outcome %v, want restart after replay", run.status, run.outcome)
		}
		for _, diagnostic := range appLogs(t, log) {
			message := strings.ToLower(diagnostic.Message)
			if strings.Contains(message, "restore") || (strings.Contains(message, "enroll") && strings.Contains(message, "again")) {
				t.Error("the application instructed the owner to restore the store or enroll again")
			}
		}
		requireOutgoingSafe(t, "application output", log.Bytes(), review11SignInValue, review11Refresh)
	case <-ctx.Done():
		t.Error("the application did not exit after the recovered stream requested restart")
	}
}

func seedReview11RetainedFacts(t *testing.T, stateDir string, versions []string) {
	t.Helper()
	folder := appFolder(t, stateDir)
	defer closeAppFolder(t, folder)
	store, err := link.OpenStore(folder, link.StoreOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	for i, version := range versions {
		report := &linkv1.Event{
			HappenedAt: timestamppb.New(outgoingServerTime),
			Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{
				Os: "fixture", Cores: 1, ClientVersion: version,
			}},
		}
		payload, err := (proto.MarshalOptions{}).MarshalAppend([]byte{1}, report)
		if err != nil {
			t.Fatal(err)
		}
		if number, err := store.Append(payload); err != nil || number != uint64(i+1) {
			t.Fatalf("the retained fixture appended number %d with error %v, want %d", number, err, i+1)
		}
	}
}
