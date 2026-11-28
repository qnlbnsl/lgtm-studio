package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestOutgoingDirectScreeningWaitsAndResumes(t *testing.T) {
	const initial = "direct-initial-fixture"
	const replacement = "direct.replacement.fixture"
	ctx, cancel := context.WithCancel(appContext(t))
	defer cancel()
	opts, _ := appOptions(t)
	opts.Version = "client-" + replacement + "-end"
	credentials := defaultOutgoingCredentials(t, opts, initial)
	bin := filepath.Join(opts.Getenv("HOME"), "fixture-bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeOutgoingProbe(bin, "codex", "codex-cli 2.3.4-before."+replacement+".after"); err != nil {
		t.Fatal(err)
	}

	entered, release := make(chan struct{}), make(chan struct{})
	markRead := sync.OnceFunc(func() { close(entered) })
	releaseRead := sync.OnceFunc(func() { close(release) })
	defer releaseRead()
	var armed atomic.Bool
	readFile := claudeSignInReader(opts.Getenv)
	filter := link.NewRefreshingOutgoingFilter(func() []string {
		if armed.Load() {
			markRead()
			<-release
		}
		return readFile()
	})
	loggedWhileHeld := make(chan struct{}, 1)
	logs := &outgoingLogObserver{onWrite: func([]byte) {
		select {
		case <-entered:
		default:
			return
		}
		select {
		case <-release:
		default:
			select {
			case loggedWhileHeld <- struct{}{}:
			default:
			}
		}
	}}
	opts.Stderr = logs
	logger := newOutgoingLogger(logs, filter)
	useAppLink(t, func(ctx context.Context, options link.RunOptions) (link.RunOutcome, error) {
		options.Filter, options.Logger = filter, logger
		options.ScanOptions.SearchPath = []string{bin}
		return link.Run(ctx, options)
	})

	commandsSent := make(chan struct{})
	wireWhileHeld := make(chan struct{}, 1)
	peerDone := make(chan error, 1)
	var opened atomic.Bool
	peer := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) (err error) {
		if !opened.CompareAndSwap(false, true) {
			return errors.New("the signed stream unexpectedly reconnected")
		}
		defer func() { peerDone <- err }()
		if err := p.challenge(stream); err != nil {
			return err
		}
		proof, err := stream.Receive()
		if err != nil || proof.GetProof() == nil {
			return errors.New("the peer did not receive proof")
		}
		if err := replaceOutgoingCredentials(credentials, replacement, "direct-refresh-fixture"); err != nil {
			return err
		}
		armed.Store(true)
		if err := sendOutgoingWelcome(stream, true, 0); err != nil {
			return err
		}
		select {
		case <-entered:
		case <-ctx.Done():
			return ctx.Err()
		}
		commands := directStateCommands("filter fixture")
		for _, command := range commands {
			if err := sendDirectCommand(stream, command); err != nil {
				return err
			}
		}
		close(commandsSent)

		var numbers []uint64
		var results []string
		var heartbeats int
		var requestedRestart bool
		restart := directRestart()
		for {
			frame, err := stream.Receive()
			if errors.Is(err, io.EOF) && requestedRestart {
				if !slices.Equal(numbers, []uint64{1, 2}) || !slices.Equal(results, directCommandIDs(commands, restart)) {
					return fmt.Errorf("the peer received reports %v and %d results in an unexpected order", numbers, len(results))
				}
				if heartbeats != 1 {
					return fmt.Errorf("the peer received %d heartbeats before Restart, want one", heartbeats)
				}
				return nil
			}
			if err != nil {
				return err
			}
			select {
			case <-release:
			default:
				select {
				case wireWhileHeld <- struct{}{}:
				default:
				}
			}
			wire, err := proto.Marshal(frame)
			if err != nil {
				return err
			}
			requireOutgoingSafe(t, "frame captured during the hold or after release", wire, replacement)

			switch {
			case frame.GetHeartbeat() != nil:
				heartbeats++
				if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{
					HeartbeatAck: &linkv1.HeartbeatAck{},
				}}); err != nil {
					return err
				}
			case frame.GetEvent() != nil:
				event := frame.GetEvent()
				numbers = append(numbers, event.GetNumber())
				switch event.GetNumber() {
				case 1:
					if event.GetFacts().GetClientVersion() != "client-[redacted]-end" {
						t.Error("the released read did not mask the new sign-in value in the facts")
					}
				case 2:
					if inventoryVersion(event, "codex") != "2.3.4-before.[redacted].after" {
						t.Error("the released read did not mask the new sign-in value in the inventory")
					}
				}
				if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{
					EventAck: &linkv1.EventAck{Through: event.GetNumber()},
				}}); err != nil {
					return err
				}
			case frame.GetCommandResult() != nil:
				result := frame.GetCommandResult()
				if result.GetAttempt() != 1 || result.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_DONE {
					t.Error("a released result did not settle successfully")
				}
				results = append(results, result.GetCommandId())
			default:
				return errors.New("the peer received an unexpected frame after Welcome")
			}
			if !requestedRestart && heartbeats != 0 && len(numbers) >= 2 && len(results) >= len(commands) {
				requestedRestart = true
				if err := sendDirectCommand(stream, restart); err != nil {
					return err
				}
			}
		}
	})
	seedApp(t, opts, peer, false)
	status := make(chan int, 1)
	go func() { status <- Run(ctx, opts) }()
	select {
	case <-commandsSent:
	case err := <-peerDone:
		t.Fatalf("the peer ended before producing commands during the held read: %v", err)
	case <-ctx.Done():
		t.Fatal("the signed peer did not send commands while the read was held")
	}

	logStarted, logDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(logDone)
		logger.Info("direct fixture log", "value", directLogValue{started: logStarted, value: "before-" + replacement + "-after"})
	}()
	<-logStarted

	// The channels establish the hold and the pending work. The timer gives
	// the real signed connection an observation window while both stay held.
	observe := time.NewTimer(1250 * time.Millisecond)
	defer observe.Stop()
	select {
	case <-wireWhileHeld:
		t.Error("a report, result or heartbeat reached the signed peer while the read was held")
	case <-loggedWhileHeld:
		t.Error("a log line produced after the hold began reached the writer")
	case <-logDone:
		t.Error("the log emission stopped waiting before the read was released")
	case <-observe.C:
	case <-ctx.Done():
		t.Error("the client stopped while the fixture held its sign-in read")
	}
	releaseRead()

	select {
	case err := <-peerDone:
		if err != nil {
			t.Errorf("the peer did not receive the complete released sequence: %v", err)
		}
	case <-ctx.Done():
		t.Error("the peer did not receive both reports, the queued results and a heartbeat after release")
	}
	select {
	case got := <-status:
		if got != 75 {
			t.Errorf("the app exited with status %d after Restart, want 75", got)
		}
	case <-ctx.Done():
		t.Error("the app did not exit after the peer closed the restarted stream")
	}
	select {
	case <-logDone:
	case <-ctx.Done():
		t.Fatal("the log call did not finish after release")
	}

	logs.mu.Lock()
	defer logs.mu.Unlock()
	requireOutgoingSafe(t, "application output after the held read", logs.records.Bytes(), replacement)
	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(logs.records.Bytes()))
	for scanner.Scan() {
		var row struct {
			Message string `json:"msg"`
			Value   string `json:"value"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("could not decode the emitted log: %v", err)
		}
		if row.Message == "direct fixture log" {
			count++
			if row.Value != "before-[redacted]-after" {
				t.Error("the released read did not mask the new sign-in value in the log")
			}
		}
	}
	if err := scanner.Err(); err != nil || count != 1 {
		t.Errorf("the writer received %d copies of the released log, want one, scan error %v", count, err)
	}
}

type directLogValue struct {
	started chan struct{}
	value   string
}

func (v directLogValue) LogValue() slog.Value {
	close(v.started)
	return slog.StringValue(v.value)
}

func TestOutgoingDirectLogWaitsForReadWithoutDroppingRecord(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const value = "direct.log.fixture"
		entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		markRead := sync.OnceFunc(func() { close(entered) })
		releaseRead := sync.OnceFunc(func() { close(release) })
		defer releaseRead()
		filter := link.NewRefreshingOutgoingFilter(func() []string {
			markRead()
			<-release
			return []string{value}
		})
		var output bytes.Buffer
		logger := newOutgoingLogger(&output, filter)
		go func() {
			defer close(done)
			logger.Info("before-" + value + "-after")
		}()
		<-entered
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if output.Len() != 0 {
			t.Error("the writer received a line while the reader was held")
		}
		select {
		case <-done:
			t.Error("the log call abandoned its record while the read was held")
		default:
		}

		releaseRead()
		<-done
		rows := appLogs(t, &output)
		if len(rows) != 1 || rows[0].Msg != "before-[redacted]-after" {
			t.Error("the writer did not receive exactly one masked record after release")
		}
	})
}

func TestOutgoingDirectRestartExitsAfterSendingQueuedResults(t *testing.T) {
	ctx := appContext(t)
	opts, _ := appOptions(t)
	defaultOutgoingCredentials(t, opts, "direct-healthy-sign-in")
	peerDone := make(chan error, 1)
	var opened atomic.Bool
	peer := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) (err error) {
		if !opened.CompareAndSwap(false, true) {
			return errors.New("the signed stream unexpectedly reconnected")
		}
		defer func() { peerDone <- err }()
		if err := welcomeOutgoingPeer(p, stream, true); err != nil {
			return err
		}

		commands := directStateCommands("conflicting fixture state")
		restart := directRestart()
		for _, command := range append(commands, restart) {
			if err := sendDirectCommand(stream, command); err != nil {
				return err
			}
		}
		var results []string
		for {
			frame, err := stream.Receive()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if result := frame.GetCommandResult(); result != nil {
				if result.GetAttempt() != 1 || result.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_DONE {
					t.Error("the peer received a result without its expected attempt or outcome")
				}
				results = append(results, result.GetCommandId())
			}
		}
		if !slices.Equal(results, directCommandIDs(commands, restart)) {
			return fmt.Errorf("the peer received %d results before EOF, want all state results followed by Restart", len(results))
		}
		return nil
	})
	seedApp(t, opts, peer, false)
	if status := Run(ctx, opts); status != 75 {
		t.Errorf("the app exited with status %d, want 75", status)
	}
	select {
	case err := <-peerDone:
		if err != nil {
			t.Errorf("the peer did not observe a complete Restart drain: %v", err)
		}
	case <-ctx.Done():
		t.Error("the peer did not observe request EOF after Restart")
	}
}

func directStateCommands(name string) []*linkv1.Command {
	commands := make([]*linkv1.Command, 3)
	for i := range commands {
		commands[i] = &linkv1.Command{
			Id: fmt.Sprintf("0195977c-0000-7000-8000-%012x", i+1), Attempt: 1,
			ExpiresAt: timestamppb.New(outgoingServerTime.Add(time.Minute)),
			Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{State: &linkv1.MachineState{
				Approval: linkv1.Approval_APPROVAL_APPROVED, Revision: 1, Name: name,
			}}},
		}
	}
	return commands
}

func directRestart() *linkv1.Command {
	return &linkv1.Command{
		Id: "0195977c-0000-7000-8000-000000000004", Attempt: 1,
		ExpiresAt: timestamppb.New(outgoingServerTime.Add(time.Minute)),
		Action:    &linkv1.Command_Restart{Restart: &linkv1.Restart{}},
	}
}

func sendDirectCommand(stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], command *linkv1.Command) error {
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: command}})
}

func directCommandIDs(commands []*linkv1.Command, restart *linkv1.Command) []string {
	ids := make([]string, 0, len(commands)+1)
	for _, command := range commands {
		ids = append(ids, command.GetId())
	}
	return append(ids, restart.GetId())
}
