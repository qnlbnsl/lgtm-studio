package link

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

var revisionTwoRunID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestRevisionTwoRunAnnouncement(t *testing.T) {
	t.Run("retained events, retries and restart", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			payload := fixtureFactsPayload(t, "retained")
			f.seed(t, payload, payload, payload)
			acked := watchFixtureAcks(t)
			cancel, done := f.start(t)
			defer cancel()

			leg := f.next(t, 0)
			first := requireRevisionTwoAnnouncement(t, f, leg, 4)
			leg.send(t, welcomeFrame(runWelcome(approvedApproval, 1, 0)))
			if event := eventOnLeg(t, leg); event.GetNumber() != 1 {
				t.Fatal("the first stream did not replay event one")
			}
			leg.send(t, ackEvent(1))
			if receiveNow(t, acked, "the first acknowledgement did not remove the saved event") != 1 {
				t.Fatal("the first acknowledgement removed the wrong event")
			}
			for n := uint64(2); n <= 4; n++ {
				if event := eventOnLeg(t, leg); event.GetNumber() != n {
					t.Fatalf("the first stream did not send event %d in order", n)
				}
			}

			previous := first
			for _, refusal := range []error{
				peerError(connect.CodeFailedPrecondition, refusalKey, "event_gap"),
				peerError(connect.CodeInvalidArgument, "", ""),
			} {
				leg.refuse <- refusal
				leg = f.next(t, 15*time.Second)
				next := requireRevisionTwoAnnouncement(t, f, leg, 4)
				if next.GetRunId() != first.GetRunId() {
					t.Fatal("the reconnect changed the run identifier")
				}
				if bytes.Equal(next.GetMachineNonce(), previous.GetMachineNonce()) ||
					bytes.Equal(next.GetMachineNonce(), first.GetMachineNonce()) {
					t.Fatal("the reconnect reused a stream nonce")
				}
				previous = next
			}

			leg.refuse <- peerError(connect.CodeFailedPrecondition, refusalKey, "link_revision_mismatch")
			result := receiveNow(t, done, "the client did not return after the revision refusal")
			if result.outcome != RunRecoveryStop || result.err != nil {
				t.Error("the client did not return RunRecoveryStop without an error after the revision refusal")
			}
			if result.err != nil {
				f.checkOutput(t, result.err.Error())
			}
			time.Sleep(15 * time.Second)
			requireEmpty(t, f.peer.opens, "the client retried a signed revision refusal")
			refusals := 0
			for _, line := range f.diagnostics(t) {
				if line.Msg == "link_refused" && line.Code == "link_revision_mismatch" {
					refusals++
				}
			}
			if refusals != 1 {
				t.Fatal("the client did not report exactly one signed revision refusal")
			}

			store := mustOpen(t, f.opts.Folder, StoreOptions{})
			wantNext := store.Next()
			if err := store.Close(); err != nil {
				t.Fatal("could not close the store before restarting the client")
			}
			if wantNext <= 4 {
				t.Fatal("the first run did not append a fresh report")
			}
			stopRestarted, restartedDone := f.start(t)
			defer stopRestarted()
			restarted := requireRevisionTwoAnnouncement(t, f, f.next(t, 0), wantNext)
			if restarted.GetRunId() == first.GetRunId() {
				t.Fatal("the restarted process reused its predecessor's run identifier")
			}
			stopRestarted()
			f.stop(t, restartedDone, nil)
		})
	})

	t.Run("two runs at the same empty boundary", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			f.seed(t)
			var previous string
			for range 2 {
				cancel, done := f.start(t)
				defer cancel()
				leg := f.next(t, 0)
				hello := requireRevisionTwoAnnouncement(t, f, leg, 1)
				leg.send(t, welcomeFrame(runWelcome(waitingApproval, 1, 0)))
				requireRevisionTwoBareHeartbeat(t, leg, 0)
				if hello.GetRunId() == previous {
					t.Fatal("two empty runs announced the same identifier")
				}
				previous = hello.GetRunId()
				cancel()
				f.stop(t, done, nil)
			}
		})
	})
}

func requireRevisionTwoAnnouncement(t *testing.T, f *runPeerFixture, leg *wireLeg, wantFirst uint64) *linkv1.Hello {
	t.Helper()
	hello := leg.next(t).GetHello()
	if leg.hasToken || hello.GetMachineId() != f.opts.Server.MachineID {
		t.Fatal("the announced machine or the new stream's credentials were wrong")
	}
	if got := hello.GetFirstEventNumber(); got != wantFirst {
		t.Fatalf("Hello announced first event %d, want durable next %d", got, wantFirst)
	}
	if hello.GetLinkRevision() != 2 || !revisionTwoRunID.MatchString(hello.GetRunId()) {
		t.Fatal("Hello did not announce revision two and a canonical version 7 run identifier")
	}
	nonce := hello.GetMachineNonce()
	if len(nonce) != 32 || bytes.Equal(nonce, make([]byte, 32)) {
		t.Fatal("Hello did not carry a valid stream nonce")
	}
	f.peer.prove(t, leg, hello)
	return hello
}

func TestRevisionTwoWaitingPrefix(t *testing.T) {
	window := revisionTwoEventWindow(t)

	t.Run("full window slides by one", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			payload := fixtureFactsPayload(t, "retained")
			prefix := window + 1
			reports := make([][]byte, prefix)
			for i := range reports {
				reports[i] = payload
			}
			f.seed(t, reports...)
			checkHandoffs := watchRevisionTwoEventHandoffs(t)
			cancel, done := f.start(t)
			defer cancel()
			leg := f.next(t, 0)
			hello := openRevisionTwoWaitingFixture(t, f, leg, 0)
			requireRevisionTwoBareHeartbeat(t, leg, 0)
			for n := uint64(1); n <= window; n++ {
				requireRevisionTwoRetainedEvent(t, leg, n)
			}
			checkHandoffs(t, window)
			requireRevisionTwoBareHeartbeat(t, leg, reportInterval)
			checkHandoffs(t, window)

			leg.send(t, ackEvent(1))
			requireRevisionTwoRetainedEvent(t, leg, prefix)
			checkHandoffs(t, prefix)
			leg.send(t, ackEvent(prefix))
			checkHandoffs(t, prefix)

			if hello.GetFirstEventNumber() != prefix+1 {
				t.Fatal("the waiting prefix did not end at the announced run boundary")
			}
			requireRevisionTwoApprovalReports(t, f, leg, prefix+1)
			cancel()
			f.stop(t, done, nil)
		})
	})

	t.Run("empty history", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			f.seed(t)
			checkHandoffs := watchRevisionTwoEventHandoffs(t)
			cancel, done := f.start(t)
			defer cancel()
			leg := f.next(t, 0)
			hello := openRevisionTwoWaitingFixture(t, f, leg, 0)
			requireRevisionTwoBareHeartbeat(t, leg, 0)
			checkHandoffs(t, 0)
			requireRevisionTwoApprovalReports(t, f, leg, 1)
			if hello.GetFirstEventNumber() != 1 {
				t.Fatal("the empty waiting run did not announce its first event as one")
			}
			cancel()
			f.stop(t, done, nil)
		})
	})

	t.Run("same run resumes from the new Welcome mark", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			payload := fixtureFactsPayload(t, "retained")
			f.seed(t, payload, payload, payload)
			acked := watchFixtureAcks(t)
			cancel, done := f.start(t)
			defer cancel()
			first := f.next(t, 0)
			hello := openRevisionTwoWaitingFixture(t, f, first, 0)
			requireRevisionTwoBareHeartbeat(t, first, 0)
			requireRevisionTwoRetainedEvent(t, first, 1)
			first.send(t, ackEvent(1))
			if receiveNow(t, acked, "the client did not persist the first acknowledgement") != 1 {
				t.Fatal("the first acknowledgement discarded the wrong event")
			}
			requireRevisionTwoRetainedEvent(t, first, 2)

			// Event two committed at the peer and its acknowledgement was lost.
			first.refuse <- peerError(connect.CodeUnavailable, "", "")
			second := f.next(t, 15*time.Second)
			reconnect := openRevisionTwoWaitingFixture(t, f, second, 2)
			if reconnect.GetRunId() != hello.GetRunId() || reconnect.GetFirstEventNumber() != 4 {
				t.Fatal("the waiting reconnect did not keep its run and boundary")
			}
			requireRevisionTwoBareHeartbeat(t, second, 0)
			if receiveNow(t, acked, "the new Welcome did not retire the committed prefix") != 2 {
				t.Fatal("the new Welcome discarded the wrong event")
			}
			requireRevisionTwoRetainedEvent(t, second, 3)
			second.send(t, ackEvent(3))
			requireRevisionTwoApprovalReports(t, f, second, 4)
			cancel()
			f.stop(t, done, nil)
		})
	})
}

func revisionTwoEventWindow(t *testing.T) uint64 {
	t.Helper()
	path := filepath.Join("..", "..", "..", "contracts", "testdata", "link-v2-limits.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("could not read the link limits fixture")
	}
	var limits struct {
		UnackedEvents uint64 `json:"unacked_events"`
	}
	if err := json.Unmarshal(data, &limits); err != nil || limits.UnackedEvents == 0 {
		t.Fatal("the link limits fixture has no valid event window")
	}
	return limits.UnackedEvents
}

func watchRevisionTwoEventHandoffs(t *testing.T) func(*testing.T, uint64) {
	t.Helper()
	var handed uint64
	useSessionIO(t, func(_ context.Context, _ *session, wire runSessionIO) runSessionIO {
		send := wire.send
		wire.send = func(frame *linkv1.OpenRequest) error {
			if frame.GetEvent() != nil {
				handed++
			}
			return send(frame)
		}
		return wire
	})
	return func(t *testing.T, want uint64) {
		t.Helper()
		synctest.Wait()
		if handed != want {
			t.Fatalf("the waiting stream handed off %d events, want %d", handed, want)
		}
	}
}

func openRevisionTwoWaitingFixture(t *testing.T, f *runPeerFixture, leg *wireLeg, through uint64) *linkv1.Hello {
	t.Helper()
	hello := leg.next(t).GetHello()
	if hello == nil {
		t.Fatal("the waiting stream did not send a Hello")
	}
	f.peer.prove(t, leg, hello)
	leg.send(t, welcomeFrame(runWelcome(waitingApproval, 1, through)))
	return hello
}

func requireRevisionTwoBareHeartbeat(t *testing.T, leg *wireLeg, wait time.Duration) {
	t.Helper()
	missing := "the waiting stream did not send its heartbeat"
	pending := leg.read()
	var read wireRead
	if wait == 0 {
		read = receiveNow(t, pending, missing)
	} else {
		read = receiveWithin(t, pending, wait, missing)
	}
	if beat := read.frame.GetHeartbeat(); read.err != nil || beat == nil || proto.Size(beat) != 0 {
		t.Fatal("the waiting stream did not send a bare heartbeat")
	}
	leg.send(t, ackHeartbeat())
}

func requireRevisionTwoRetainedEvent(t *testing.T, leg *wireLeg, want uint64) {
	t.Helper()
	read := receiveNow(t, leg.read(), "the waiting stream did not replay the next retained event")
	event := read.frame.GetEvent()
	if read.err != nil || event.GetNumber() != want || event.GetFacts().GetClientVersion() != "retained" {
		t.Fatalf("the waiting stream replayed event %d, want retained event %d", event.GetNumber(), want)
	}
}

func requireRevisionTwoApprovalReports(t *testing.T, f *runPeerFixture, leg *wireLeg, first uint64) {
	t.Helper()
	command := stateCommand(1, time.Now().Add(time.Minute), 2, approvedApproval, "fixture machine")
	leg.send(t, commandFrame(command))
	var facts, inventory, result bool
	next := first
	for !facts || !inventory || !result {
		frame := leg.next(t)
		if reply := frame.GetCommandResult(); reply != nil && !result {
			wantCommandResult(t, reply, command, commandDone)
			result = true
			continue
		}
		event := frame.GetEvent()
		if event.GetNumber() != next {
			t.Fatalf("approval sent event %d, want fresh event %d", event.GetNumber(), next)
		}
		switch {
		case !facts && event.GetFacts().GetClientVersion() == f.opts.Version:
			facts = true
		case !inventory && event.GetHarnessesFound() != nil && hasFixtureInventory(event.GetHarnessesFound()):
			inventory = true
		default:
			t.Fatal("approval did not send one fresh facts report and one inventory report")
		}
		leg.send(t, ackEvent(next))
		next++
	}
}
