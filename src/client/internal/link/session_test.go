package link

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

const (
	waitingApproval      = linkv1.Approval_APPROVAL_WAITING
	approvedApproval     = linkv1.Approval_APPROVAL_APPROVED
	removedApproval      = linkv1.Approval_APPROVAL_REMOVED
	refusalKey           = "Lgtm-Link-Refusal"
	storeRecoveryMessage = "An owner must decide how to recover this machine's saved events before restarting."
)

type runPeerFixture struct {
	peer *wirePeer
	opts RunOptions
	log  *reportLog
}

type runResult struct {
	outcome RunOutcome
	err     error
}

func newRunPeerFixture(t *testing.T, p protocol) *runPeerFixture {
	t.Helper()
	peer, client, server, credential := newWirePeer(t, p)
	folder, _ := newState(t)
	log := new(reportLog)
	oldClient, oldProbes := newRunLinkClient, runReportProbes
	newRunLinkClient = func(enroll.Server) (linkv1connect.LinkServiceClient, func(), error) {
		return client, func() {}, nil
	}
	runReportProbes = fixtureReportProbes()
	runReportProbes.now = time.Now
	runReportProbes.readLoad = func() (sysinfo.Load, error) {
		return sysinfo.Load{CPUPercent: 17, MemoryPercent: 31}, nil
	}
	t.Cleanup(func() {
		newRunLinkClient, runReportProbes = oldClient, oldProbes
	})
	return &runPeerFixture{
		peer: peer, log: log,
		opts: RunOptions{Server: server, Credential: credential, Folder: folder, Logger: log.logger(), Version: "session-fixture"},
	}
}

func (f *runPeerFixture) start(t *testing.T) (context.CancelFunc, <-chan runResult) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan runResult, 1)
	go func() {
		outcome, err := Run(ctx, f.opts)
		done <- runResult{outcome, err}
	}()
	return cancel, done
}

func (f *runPeerFixture) next(t *testing.T, limit time.Duration) *wireLeg {
	t.Helper()
	if limit == 0 {
		return receiveNow(t, f.peer.opens, "the client did not start the expected stream")
	}
	return receiveWithin(t, f.peer.opens, limit, "the client did not reconnect within the expected window")
}

func (f *runPeerFixture) stop(t *testing.T, done <-chan runResult, wantErr error) runResult {
	t.Helper()
	result := receiveNow(t, done, "the client did not return after it was stopped")
	if result.outcome != RunStopped || !errors.Is(result.err, wantErr) {
		t.Error("the client did not return RunStopped with the expected error")
	}
	if result.err != nil {
		f.checkOutput(t, result.err.Error())
	}
	return result
}

func (f *runPeerFixture) checkOutput(t *testing.T, text string) {
	t.Helper()
	for _, key := range []ed25519.PrivateKey{f.peer.keys.machineKey, f.peer.keys.serverKey} {
		checkNoFixtureSecrets(t, text,
			string(key), hex.EncodeToString(key), base64.StdEncoding.EncodeToString(key),
			hex.EncodeToString(key.Seed()), base64.StdEncoding.EncodeToString(key.Seed()))
	}
}

type sessionDiagnostic struct {
	Msg      string    `json:"msg"`
	Code     string    `json:"code"`
	Message  string    `json:"message"`
	Time     time.Time `json:"time"`
	Level    string    `json:"level"`
	Sequence uint64    `json:"sequence"`
}

func (f *runPeerFixture) diagnostics(t *testing.T) []sessionDiagnostic {
	t.Helper()
	f.log.mu.Lock()
	raw := f.log.buffer.String()
	f.log.mu.Unlock()
	f.checkOutput(t, raw)
	var logs []sessionDiagnostic
	decoder := json.NewDecoder(strings.NewReader(raw))
	for decoder.More() {
		var line sessionDiagnostic
		if err := decoder.Decode(&line); err != nil {
			t.Fatal("the diagnostic is not valid JSON")
		}
		logs = append(logs, line)
	}
	return logs
}

func (f *runPeerFixture) file() string {
	return f.opts.Folder.Paths().Store
}

func (f *runPeerFixture) seed(t *testing.T, reports ...[]byte) {
	t.Helper()
	store := mustCreate(t, f.opts.Folder, StoreOptions{})
	for i, payload := range reports {
		n, err := store.Append(payload)
		if err != nil || n != uint64(i+1) {
			t.Fatal("could not write the fixture event")
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal("could not close the seeded store")
	}
}

func runWelcome(approval linkv1.Approval, revision, mark uint64) *linkv1.Welcome {
	w := &linkv1.Welcome{
		State:           &linkv1.MachineState{Approval: approval, Revision: revision},
		LastEventNumber: mark, LinkRevision: linkRevision, ServerTime: timestamppb.Now(),
	}
	if approval == approvedApproval {
		w.State.Name = "fixture machine"
		w.ConnectionToken = fixtureToken
	}
	return w
}

func (f *runPeerFixture) welcome(t *testing.T, leg *wireLeg, w *linkv1.Welcome) {
	t.Helper()
	f.peer.prove(t, leg, leg.hello(t))
	leg.send(t, welcomeFrame(w))
}

func commandFrame(cmd *linkv1.Command) *linkv1.OpenResponse {
	return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: cmd}}
}

func ackEvent(n uint64) *linkv1.OpenResponse {
	return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{EventAck: &linkv1.EventAck{Through: n}}}
}

func ackHeartbeat() *linkv1.OpenResponse {
	return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}}
}

func wantCommandResult(t *testing.T, result *linkv1.CommandResult, cmd *linkv1.Command, outcome linkv1.CommandOutcome) {
	t.Helper()
	if result == nil || result.GetCommandId() != cmd.GetId() ||
		result.GetAttempt() != cmd.GetAttempt() || result.GetOutcome() != outcome {
		t.Error("the command result did not match the expected identifier, attempt, and outcome")
	}
}

func fixtureFactsPayload(t *testing.T, version string) []byte {
	t.Helper()
	raw, err := encodeReport(&linkv1.Event{HappenedAt: timestamppb.Now(), Report: &linkv1.Event_Facts{
		Facts: &linkv1.Facts{Cores: 2, ClientVersion: version},
	}})
	if err != nil {
		t.Fatal("could not encode the fixture facts")
	}
	return raw
}

func eventOnLeg(t *testing.T, leg *wireLeg) *linkv1.Event {
	t.Helper()
	for {
		msg := leg.next(t)
		if event := msg.GetEvent(); event != nil {
			return event
		}
		if msg.GetHeartbeat() == nil {
			t.Fatal("the client sent an unexpected frame before its next event")
		}
		leg.send(t, ackHeartbeat())
	}
}

func TestRetryStopsForVerifiedRemoval(t *testing.T) {
	for _, afterWelcome := range []bool{false, true} {
		name := "before welcome"
		if afterWelcome {
			name = "after welcome"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newRunPeerFixture(t, protocols[0])
				f.seed(t)
				cancel, done := f.start(t)
				defer cancel()

				for i := range 2 {
					leg := f.next(t, 15*time.Second)
					signer := scene{keys: f.peer.keys}
					challenge := signer.challenge(leg.hello(t), fixedServerNonce)
					challenge.GetChallenge().ServerSignature[0] ^= 1
					leg.send(t, challenge)
					read := receiveNow(t, leg.read(), "the client kept open a stream with a wrong server key")
					if read.err == nil || read.frame != nil {
						t.Errorf("wrong-key attempt %d disclosed a frame after the invalid challenge", i+1)
					}
				}

				leg := f.next(t, 15*time.Second)
				leg.hello(t)
				keyText := hex.EncodeToString(f.peer.keys.machineKey)
				leg.refuse <- peerError(connect.CodeFailedPrecondition, refusalKey, "unknown_machine", keyText)
				leg = f.next(t, 15*time.Second)
				for _, line := range f.diagnostics(t) {
					if line.Msg == "link_refused" && line.Code == "unknown_machine" {
						t.Error("the client reported removal before verifying a challenge")
					}
				}
				f.welcome(t, leg, runWelcome(approvedApproval, 3, 0))
				removalNoticeContinues(t, leg, 4)

				leg.refuse <- peerError(connect.CodeUnavailable, "", "", keyText)
				leg = f.next(t, time.Second)
				f.peer.prove(t, leg, leg.hello(t))
				if afterWelcome {
					leg.send(t, welcomeFrame(runWelcome(approvedApproval, 5, 0)))
					removalNoticeContinues(t, leg, 6)
				}
				leg.refuse <- peerError(connect.CodeFailedPrecondition, refusalKey, "unknown_machine", keyText)
				f.stop(t, done, nil)
				time.Sleep(15 * time.Second)
				requireEmpty(t, f.peer.opens, "the client dialled again after the verified removal")

				var wrongKey, removed int
				for _, line := range f.diagnostics(t) {
					if line.Msg != "link_refused" {
						continue
					}
					switch line.Code {
					case "server_key_mismatch":
						wrongKey++
					case "unknown_machine":
						removed++
					}
				}
				if wrongKey != 1 || removed != 1 {
					t.Errorf("the client logged %d server-key mismatches and %d verified removals, want one each", wrongKey, removed)
				}
			})
		})
	}
}

func removalNoticeContinues(t *testing.T, leg *wireLeg, revision uint64) {
	t.Helper()
	cmd := stateCommand(int(revision), time.Now().Add(time.Minute), revision, removedApproval, "")
	leg.send(t, commandFrame(cmd))
	for {
		frame := leg.next(t)
		if result := frame.GetCommandResult(); result != nil {
			wantCommandResult(t, result, cmd, commandDone)
			break
		}
		if frame.GetEvent() == nil && frame.GetHeartbeat() == nil {
			t.Fatal("the client did not answer the removal notice")
		}
	}
	time.Sleep(10 * time.Second)
	frame := leg.next(t)
	beat := frame.GetHeartbeat()
	if beat == nil || beat.CpuPercent != nil || beat.MemoryPercent != nil {
		t.Fatal("the client did not continue with a bare heartbeat after the notice")
	}
	leg.send(t, ackHeartbeat())
}

func TestRetryResumesAfterEventGap(t *testing.T) {
	for _, p := range protocols {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newRunPeerFixture(t, p)
				f.seed(t, fixtureFactsPayload(t, "one"), fixtureFactsPayload(t, "two"), fixtureFactsPayload(t, "three"))
				cancel, done := f.start(t)
				defer cancel()
				first := f.next(t, 0)
				f.welcome(t, first, runWelcome(approvedApproval, 3, 0))
				for _, n := range []uint64{1, 2} {
					if got := eventOnLeg(t, first).GetNumber(); got != n {
						t.Fatalf("the first stream sent event %d, want %d", got, n)
					}
				}
				first.refuse <- peerError(connect.CodeFailedPrecondition, refusalKey, "event_gap")
				second := f.next(t, time.Second)
				f.welcome(t, second, runWelcome(approvedApproval, 3, 1))
				for i, version := range []string{"two", "three"} {
					n := uint64(i + 2)
					event := eventOnLeg(t, second)
					if event.GetNumber() != n || event.GetFacts().GetClientVersion() != version {
						t.Errorf("the new stream did not replay saved event %d from the server's committed prefix", n)
					}
					second.send(t, ackEvent(n))
				}
				synctest.Wait()
				cancel()
				f.stop(t, done, nil)
				store := mustOpen(t, f.opts.Folder, StoreOptions{})
				events, err := store.Read(1, 1)
				if err != nil || len(events) > 0 && events[0].Seq < 4 {
					t.Error("the confirmed prefix remained in the outbox")
				}
				f.diagnostics(t)
			})
		})
	}
}

func TestRetryReportsOnApprovalAndOutage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t, fixtureFactsPayload(t, "standing backlog"))
		cancel, done := f.start(t)
		defer cancel()
		first := f.next(t, 0)
		f.welcome(t, first, runWelcome(waitingApproval, 1, 0))
		waitingBeat := receiveWithin(t, first.read(), 10*time.Second, "the waiting stream did not send a heartbeat")
		beat := waitingBeat.frame.GetHeartbeat()
		if waitingBeat.err != nil || beat == nil || beat.CpuPercent != nil || beat.MemoryPercent != nil {
			t.Fatal("the waiting stream did not send a bare heartbeat")
		}
		firstBeat := time.Now()
		first.send(t, ackHeartbeat())
		saved := eventOnLeg(t, first)
		if saved.GetNumber() != 1 || saved.GetFacts().GetClientVersion() != "standing backlog" {
			t.Fatal("the waiting stream did not send the saved event")
		}
		first.send(t, ackEvent(1))

		approval := stateCommand(5, time.Now().Add(time.Minute), 2, approvedApproval, "fixture machine")
		first.send(t, commandFrame(approval))
		seenFacts, seenInventory, seenResult := false, false, false
		for sequence := uint64(2); !seenFacts || !seenInventory || !seenResult; {
			frame := first.next(t)
			if result := frame.GetCommandResult(); result != nil {
				wantCommandResult(t, result, approval, commandDone)
				seenResult = true
				continue
			}
			event := frame.GetEvent()
			if event == nil || event.GetNumber() != sequence || sequence > 3 {
				t.Fatal("approval did not send both fresh reports in order")
			}
			switch {
			case event.GetFacts() != nil:
				seenFacts = event.GetFacts().GetClientVersion() == f.opts.Version
			case event.GetHarnessesFound() != nil:
				seenInventory = hasFixtureInventory(event.GetHarnessesFound())
			}
			first.send(t, ackEvent(sequence))
			sequence++
		}
		approvedBeat := first.read()
		time.Sleep(10*time.Second - time.Nanosecond)
		requireEmpty(t, approvedBeat, "the client sent another heartbeat before its 10-second cadence")
		time.Sleep(time.Nanosecond)
		read := receiveNow(t, approvedBeat, "the approved stream missed its heartbeat")
		beat = read.frame.GetHeartbeat()
		if read.err != nil || beat == nil || beat.GetCpuPercent() != 17 || beat.GetMemoryPercent() != 31 {
			t.Error("the approved heartbeat did not contain the measured load")
		}
		if time.Since(firstBeat) != 10*time.Second {
			t.Error("the heartbeat cadence did not stay at ten seconds across approval")
		}

		// Withhold every response after the event acknowledgements.
		time.Sleep(20*time.Second - time.Nanosecond)
		requireEmpty(t, first.closed, "the client closed the silent stream before 30 seconds")
		requireEmpty(t, f.peer.opens, "the silent stream overlapped a new attempt")
		time.Sleep(time.Nanosecond)
		receiveNow(t, first.closed, "the client did not close the stream after 30 seconds of silence")
		outage := time.Now()
		for range 4 {
			leg := f.next(t, 15*time.Second)
			leg.hello(t)
			leg.refuse <- peerError(connect.CodeUnavailable, "", "")
		}
		recovery := f.next(t, 15*time.Second)
		hello := recovery.hello(t)
		// Keep the current handshake open while the third outage capture comes due.
		time.Sleep(21*time.Second - time.Since(outage))
		var buffered []sessionDiagnostic
		for _, line := range f.diagnostics(t) {
			if line.Msg == "link_event_buffered" && !line.Time.Before(outage) {
				buffered = append(buffered, line)
			}
		}
		if len(buffered) != 3 {
			t.Fatalf("the client buffered %d events during the outage, want 3 facts", len(buffered))
		}
		f.peer.prove(t, recovery, hello)
		recovery.send(t, welcomeFrame(runWelcome(approvedApproval, 2, 3)))
		for i, line := range buffered {
			wantNumber := uint64(i + 4)
			wantTime := outage.Add(time.Duration(i) * 10 * time.Second)
			event := eventOnLeg(t, recovery)
			if line.Sequence != wantNumber || !line.Time.Equal(wantTime) || event.GetNumber() != wantNumber ||
				event.GetFacts() == nil || !event.GetHappenedAt().AsTime().Equal(wantTime) {
				t.Errorf("outage fact %d did not buffer at its scheduled instant and replay in order", i+1)
			}
			recovery.send(t, ackEvent(wantNumber))
		}
		freshFacts, freshInventory := false, false
		for n := uint64(7); n <= 8; n++ {
			event := eventOnLeg(t, recovery)
			if event.GetNumber() != n {
				t.Errorf("the recovered stream sent event %d, want %d", event.GetNumber(), n)
			}
			freshFacts = freshFacts || event.GetFacts().GetClientVersion() == f.opts.Version
			freshInventory = freshInventory || hasFixtureInventory(event.GetHarnessesFound())
			recovery.send(t, ackEvent(n))
		}
		if !freshFacts || !freshInventory {
			t.Error("the approved welcome did not queue fresh facts and inventory")
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func hasFixtureInventory(inventory *linkv1.HarnessesFound) bool {
	var names []string
	for _, row := range inventory.GetHarnesses() {
		names = append(names, row.GetName())
	}
	slices.Sort(names)
	return slices.Equal(names, []string{"claude", "codex", "cursor", "gemini"})
}

func TestRetryFailureExitsAndKeepsStore(t *testing.T) {
	for _, tc := range []struct {
		name      string
		prepare   func(*testing.T, *runPeerFixture)
		onStream  func(*testing.T, *runPeerFixture)
		wantErr   error
		finalCode string
		recovery  bool
	}{
		{
			name: "open",
			prepare: func(t *testing.T, f *runPeerFixture) {
				if err := os.WriteFile(f.file(), []byte("fixture is not a database"), 0o600); err != nil {
					t.Fatal("could not write the damaged fixture store")
				}
			},
			recovery: true,
		},
		{
			name: "read",
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t, []byte("fixture is not an encoded report"))
			},
			onStream: func(t *testing.T, f *runPeerFixture) {
				f.welcome(t, f.next(t, 0), runWelcome(approvedApproval, 2, 0))
			},
			recovery: true,
		},
		{
			name: "commit",
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t, fixtureFactsPayload(t, "saved"))
				t.Cleanup(swapCommit(func(*bolt.DB, func(*bolt.Tx) error) error {
					return errors.New("fixture commit failed")
				}))
			},
			onStream: func(t *testing.T, f *runPeerFixture) {
				// A welcome that confirms an event must delete it durably.
				f.welcome(t, f.next(t, 0), runWelcome(approvedApproval, 2, 1))
			},
			wantErr: ErrStoreFailed,
		},
		{
			name: "sequence exhaustion",
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t)
				setNext(t, f.file(), math.MaxInt64+1)
			},
			onStream: func(t *testing.T, f *runPeerFixture) {
				f.welcome(t, f.next(t, 0), runWelcome(approvedApproval, 2, math.MaxInt64))
			},
			finalCode: "sequence_exhausted",
		},
		{
			name: "impossible welcome mark",
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t, fixtureFactsPayload(t, "saved"))
			},
			onStream: func(t *testing.T, f *runPeerFixture) {
				f.welcome(t, f.next(t, 0), runWelcome(waitingApproval, 1, 2))
			},
			finalCode: "impossible_ack",
			recovery:  true,
		},
		{
			name: "impossible live mark for an event in the store",
			prepare: func(t *testing.T, f *runPeerFixture) {
				payload := fixtureFactsPayload(t, "saved")
				f.seed(t, payload, payload)
				useSessionIO(t, func(ctx context.Context, _ *session, wire runSessionIO) runSessionIO {
					// Park the sender so the second saved event stays above the handoff ceiling.
					wire.send = func(*linkv1.OpenRequest) error {
						<-ctx.Done()
						return ctx.Err()
					}
					return wire
				})
			},
			onStream: func(t *testing.T, f *runPeerFixture) {
				leg := f.next(t, 0)
				f.welcome(t, leg, runWelcome(waitingApproval, 1, 0))
				leg.send(t, ackEvent(2))
			},
			finalCode: "impossible_ack",
			recovery:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newRunPeerFixture(t, protocols[0])
				tc.prepare(t, f)
				before := readBytes(t, f.file())
				cancel, done := f.start(t)
				defer cancel()
				if tc.onStream != nil {
					tc.onStream(t, f)
				}
				if tc.recovery {
					requireRecoveryStop(t, f, done, false)
				} else {
					f.stop(t, done, tc.wantErr)
				}
				time.Sleep(15 * time.Second)
				requireEmpty(t, f.peer.opens, "the client dialled after a store fault")
				if !bytes.Equal(readBytes(t, f.file()), before) {
					t.Error("the fault changed the saved store")
				}
				var errorsLogged []sessionDiagnostic
				for _, line := range f.diagnostics(t) {
					if line.Level == "ERROR" {
						errorsLogged = append(errorsLogged, line)
					}
				}
				if len(errorsLogged) != 1 {
					t.Fatalf("the client logged %d errors for a store fault, want one", len(errorsLogged))
				}
				line := errorsLogged[0]
				if tc.finalCode != "" && (line.Msg != "link_stopped" || line.Code != tc.finalCode) {
					t.Error("the final store diagnostic is missing its required message, code, or recovery instruction")
				}
				if tc.finalCode == "sequence_exhausted" && line.Message != storeRecoveryMessage {
					t.Error("sequence exhaustion changed its diagnostic")
				}
			})
		})
	}
	t.Run("cancel without diagnostic", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			f.seed(t)
			cancel, done := f.start(t)
			defer cancel()
			leg := f.next(t, 0)
			f.welcome(t, leg, runWelcome(waitingApproval, 1, 0))
			synctest.Wait()
			before := len(f.diagnostics(t))
			cancel()
			f.stop(t, done, nil)
			if len(f.diagnostics(t)) != before {
				t.Error("cancellation added a diagnostic")
			}
			requireEmpty(t, f.peer.opens, "cancellation started another stream")
		})
	})
}

func useSessionIO(t *testing.T, hook func(context.Context, *session, runSessionIO) runSessionIO) {
	t.Helper()
	original := wrapRunSessionIO
	wrapRunSessionIO = hook
	t.Cleanup(func() { wrapRunSessionIO = original })
}

func TestSessionPrioritizesReadyFrames(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t, fixtureFactsPayload(t, "first"), fixtureFactsPayload(t, "second"))
		release := make(chan struct{})
		firstSent := make(chan struct{}, 1)
		useSessionIO(t, func(ctx context.Context, _ *session, stream runSessionIO) runSessionIO {
			send := stream.send
			stream.send = func(frame *linkv1.OpenRequest) error {
				err := send(frame)
				if err == nil && frame.GetEvent().GetNumber() == 1 {
					firstSent <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return err
			}
			return stream
		})
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		w := runWelcome(approvedApproval, 2, 0)
		f.welcome(t, leg, w)
		if eventOnLeg(t, leg).GetNumber() != 1 {
			t.Fatal("the sender did not hand off the first saved event")
		}
		receiveNow(t, firstSent, "the first event was not held before send returned")
		cmd := stateCommand(4, time.Now().Add(time.Minute), w.State.Revision, approvedApproval, w.State.Name)
		leg.send(t, commandFrame(cmd))
		time.Sleep(10 * time.Second)
		synctest.Wait()
		close(release)

		var result, heartbeat bool
		for range 2 {
			frame := leg.next(t)
			switch {
			case frame.GetCommandResult() != nil:
				wantCommandResult(t, frame.GetCommandResult(), cmd, commandDone)
				result = true
			case frame.GetHeartbeat() != nil:
				leg.send(t, ackHeartbeat())
				heartbeat = true
			default:
				t.Fatal("an event went out while a command result and heartbeat were ready")
			}
		}
		if !result || !heartbeat {
			t.Error("both ready control frames must precede the next event")
		}
		if eventOnLeg(t, leg).GetNumber() != 2 {
			t.Error("the second saved event did not follow the ready frames")
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func TestSessionAcceptsImmediateAck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t, fixtureFactsPayload(t, "one"), fixtureFactsPayload(t, "two"))
		releaseFirst := make(chan struct{})
		failSecond := make(chan struct{})
		releaseRetired := make(chan struct{})
		retired := make(chan struct{}, 1)
		held := make(chan struct{}, 1)
		useSessionIO(t, func(ctx context.Context, s *session, stream runSessionIO) runSessionIO {
			if s.welcome.GetLastEventNumber() != 0 {
				return stream
			}
			send, receive := stream.send, stream.receive
			stream.send = func(frame *linkv1.OpenRequest) error {
				if err := send(frame); err != nil {
					return err
				}
				switch frame.GetEvent().GetNumber() {
				case 1:
					select {
					case <-releaseFirst:
					case <-ctx.Done():
						return ctx.Err()
					}
				case 2:
					select {
					case <-failSecond:
						return errStream
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return nil
			}
			stream.receive = func() (*linkv1.OpenResponse, error) {
				frame, err := receive()
				if err == nil && frame.GetEventAck().GetThrough() == 2 {
					held <- struct{}{}
					select {
					case <-ctx.Done():
					case <-t.Context().Done():
						return nil, t.Context().Err()
					}
					retired <- struct{}{}
					select {
					case <-releaseRetired:
					case <-t.Context().Done():
					}
				}
				return frame, err
			}
			return stream
		})
		acked := watchFixtureAcks(t)
		cancel, done := f.start(t)
		defer cancel()
		first := f.next(t, 0)
		f.welcome(t, first, runWelcome(approvedApproval, 2, 0))
		if eventOnLeg(t, first).GetNumber() != 1 {
			t.Fatal("the first stream did not send event one")
		}
		first.send(t, ackEvent(1))
		if n := receiveNow(t, acked, "the immediate acknowledgement did not commit before send returned"); n != 1 {
			t.Errorf("the immediate acknowledgement committed event %d, want 1", n)
		}
		close(releaseFirst)
		if eventOnLeg(t, first).GetNumber() != 2 {
			t.Fatal("the first stream did not send event two")
		}
		first.send(t, ackEvent(2))
		receiveNow(t, held, "the receive worker did not hold the delayed acknowledgement")
		close(failSecond)
		receiveWithin(t, retired, 30*time.Second, "the send failure did not cancel the first stream")
		requireEmpty(t, f.peer.opens, "the client dialled while the retired receive worker was still running")
		close(releaseRetired)
		synctest.Wait()
		requireEmpty(t, acked, "the client applied an acknowledgement from a retired stream")

		second := f.next(t, time.Second)
		f.welcome(t, second, runWelcome(approvedApproval, 2, 1))
		event := eventOnLeg(t, second)
		if event.GetNumber() != 2 || event.GetFacts().GetClientVersion() != "two" {
			t.Fatal("the second stream did not replay the failed unacknowledged event")
		}
		second.send(t, ackEvent(2))
		if n := receiveNow(t, acked, "the current stream's acknowledgement did not commit"); n != 2 {
			t.Errorf("the current stream acknowledged event %d, want 2", n)
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func watchFixtureAcks(t *testing.T) <-chan uint64 {
	t.Helper()
	acked := make(chan uint64, 4)
	t.Cleanup(swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
		var removed []uint64
		err := db.Update(func(tx *bolt.Tx) error {
			events := tx.Bucket(eventsBucket)
			one, two := numberKey(1), numberKey(2)
			hadOne, hadTwo := events.Get(one[:]) != nil, events.Get(two[:]) != nil
			if err := write(tx); err != nil {
				return err
			}
			if hadOne && events.Get(one[:]) == nil {
				removed = append(removed, 1)
			}
			if hadTwo && events.Get(two[:]) == nil {
				removed = append(removed, 2)
			}
			return nil
		})
		if err == nil {
			for _, n := range removed {
				acked <- n
			}
		}
		return err
	}))
	return acked
}
