package linktest

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

type pairedScript struct {
	server *servertest.Server
	opts   clienttest.Options
	id     string
	db     *pgx.Conn
}

func newPairedScript(t *testing.T, options servertest.Options) pairedScript {
	t.Helper()
	options.Password = fixturePassword
	server := servertest.Start(t, options)
	opts := newClient(t, new(diagnostics))
	id := join(t, server.Clients(t), opts, server.URL)
	return pairedScript{server: server, opts: opts, id: id, db: openObserver(t, server)}
}

func (f pairedScript) attempt(t *testing.T, run string, first uint64) *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse] {
	t.Helper()
	stream := openPairedStream(t, f.server.Link())
	submitScriptedHello(t, stream, f.server.SigningPublicKey, f.opts, &linkv1.Hello{
		MachineId: f.id, LinkRevision: 2, RunId: run, FirstEventNumber: first,
	})
	return stream
}

func (f pairedScript) requireRun(t *testing.T, admitted, ready string, through uint64) {
	t.Helper()
	if !scalar[bool](t, f.db, `
SELECT EXISTS (
  SELECT FROM link_machines m JOIN link_runs r ON r.machine_id=m.machine_id AND r.run_id=m.admitted_run_id
  WHERE m.machine_id=$1 AND m.admitted_run_id=$2 AND m.ready_run_id=$3 AND r.replay_through=$4
)`, f.id, admitted, ready, through) {
		t.Fatal("The durable admitted or ready run did not match the announced pair.")
	}
}

func requirePairedWelcome(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], mark uint64) {
	t.Helper()
	welcome := receivePairedFrame(t, stream).GetWelcome()
	if welcome.GetLinkRevision() != 2 || welcome.GetLastEventNumber() != mark {
		t.Fatalf("The paired peer did not receive revision 2 Welcome at mark %d.", mark)
	}
}

func requirePairedEnd(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], code connect.Code, reason string) {
	t.Helper()
	reply, err := stream.Receive()
	if reply != nil || connect.CodeOf(err) != code {
		t.Fatalf("The paired peer ended with code %s or an unexpected frame, want %s.", connect.CodeOf(err), code)
	}
	var want []string
	if reason != "" {
		want = []string{reason}
	}
	if !slices.Equal(stream.ResponseTrailer().Values("Lgtm-Link-Refusal"), want) {
		t.Fatalf("The paired peer did not observe the expected refusal reason %q.", reason)
	}
}

func sendPairedFacts(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], number, through uint64) {
	t.Helper()
	sendPairedFrame(t, stream, pairedEvent(pairedFacts(number, "paired scripted history")))
	if ack := receivePairedFrame(t, stream).GetEventAck(); ack == nil || ack.GetThrough() != through {
		t.Fatalf("The paired peer did not receive an acknowledgement through %d.", through)
	}
}

func TestSprintFourPendingRun(t *testing.T) {
	requirePostgres(t)
	t.Run("R8", func(t *testing.T) { t.Run("CommitOrder", testPairedProjectCommitOrder) })
	t.Run("R9", func(t *testing.T) {
		t.Run("RunGate", func(t *testing.T) {
			t.Run("RetainedPrefix", func(t *testing.T) { testPairedProjectRunGate(t, true) })
			t.Run("EmptyPrefix", func(t *testing.T) { testPairedProjectRunGate(t, false) })
			t.Run("RetainedPrefixWithFreePermit", testPairedProjectRunGateWithFreePermit)
		})
	})
	t.Run("RestartMidDrain", func(t *testing.T) {
		f := newPairedScript(t, servertest.Options{})
		execute(t, f.db, "INSERT INTO link_machines (machine_id,event_mark) VALUES ($1,5)", f.id)
		readyID, pendingID, replacementID := newPairedID(), newPairedID(), newPairedID()
		ready := f.attempt(t, readyID, 6)
		requirePairedWelcome(t, ready, 5)
		f.requireRun(t, readyID, readyID, 5)

		pending := f.attempt(t, pendingID, 10)
		requirePairedWelcome(t, pending, 5)
		f.requireRun(t, pendingID, readyID, 9)
		sendPairedFacts(t, pending, 6, 6)
		f.server.Restart(t)
		f.requireRun(t, pendingID, readyID, 9)
		generation := scalar[uint64](t, f.db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", f.id)
		requirePairedEnd(t, f.attempt(t, readyID, 6), connect.CodeInvalidArgument, "")
		requirePairedEnd(t, f.attempt(t, pendingID, 11), connect.CodeInvalidArgument, "")
		if !scalar[bool](t, f.db, `
SELECT event_mark=6 AND stream_generation=$2 AND recovery_set_generation IS NULL
FROM link_machines WHERE machine_id=$1`, f.id, generation) {
			t.Fatal("The refused run changed the mark, generation or recovery warning.")
		}

		replacement := f.attempt(t, replacementID, 10)
		requirePairedWelcome(t, replacement, 6)
		f.requireRun(t, replacementID, readyID, 9)
		sendPairedFacts(t, replacement, 6, 6)
		for _, number := range []uint64{7, 8, 9} {
			sendPairedFacts(t, replacement, number, number)
		}
		f.requireRun(t, replacementID, replacementID, 9)
		sendPairedFrame(t, replacement, pairedEvent(pairedFacts(11, "gap fixture")))
		requirePairedEnd(t, replacement, connect.CodeFailedPrecondition, "event_gap")
		if mark(t, f.db, f.id) != 9 {
			t.Fatal("The gap changed the committed mark.")
		}
	})

	t.Run("EqualReadyBoundary", func(t *testing.T) {
		f := newPairedScript(t, servertest.Options{})
		execute(t, f.db, "INSERT INTO link_machines (machine_id,event_mark) VALUES ($1,5)", f.id)
		firstID, secondID := newPairedID(), newPairedID()
		requirePairedWelcome(t, f.attempt(t, firstID, 6), 5)
		f.requireRun(t, firstID, firstID, 5)
		requirePairedWelcome(t, f.attempt(t, secondID, 6), 5)
		f.requireRun(t, secondID, secondID, 5)
		f.server.Restart(t)
		generation := scalar[uint64](t, f.db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", f.id)
		requirePairedEnd(t, f.attempt(t, firstID, 6), connect.CodeInvalidArgument, "")
		f.requireRun(t, secondID, secondID, 5)
		if scalar[uint64](t, f.db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", f.id) != generation {
			t.Fatal("The refused superseded run changed the ready generation.")
		}
		t.Run("ProjectsEffect", testPairedEqualProjectReadiness)
	})
	t.Run("ReplayWindow", testPairedReplayWindow)
	t.Run("ReplayWithOccupiedSendPermits", testPairedReplayWithOccupiedPermits)
}

type pairedSendHold struct {
	machineID string
	release   chan struct{}
}

func testPairedReplayWithOccupiedPermits(t *testing.T) {
	t.Helper()
	var target atomic.Pointer[string]
	holders := make(chan pairedSendHold, 3)
	freeAll := make(chan struct{})
	defer close(freeAll)
	atFinal, allowFinal := make(chan struct{}), make(chan struct{})
	releaseEvent := sync.OnceFunc(func() { close(allowFinal) })
	defer releaseEvent()
	server := servertest.Start(t, servertest.Options{Password: fixturePassword, LinkSeams: servertest.LinkSeams{
		BeforeCommandSend: func(ctx context.Context, id, _ string) error {
			if current := target.Load(); current != nil && *current == id {
				return nil
			}
			hold := pairedSendHold{machineID: id, release: make(chan struct{})}
			select {
			case holders <- hold:
			case <-ctx.Done():
				return ctx.Err()
			}
			select {
			case <-hold.release:
				return nil
			case <-freeAll:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		BeforeEventCommit: func(ctx context.Context, id string, number uint64) error {
			if current := target.Load(); current != nil && *current == id && number == 2 {
				close(atFinal)
				select {
				case <-allowFinal:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		},
	}})
	page := server.Clients(t)
	r := startRelay(t, server.Link())
	var firstHold pairedSendHold
	for i := range 3 {
		opts := newClient(t, new(diagnostics))
		id := join(t, page, opts, r.address)
		approve(t, page, id, fmt.Sprintf("Permit holder fixture %d", i+1))
		startClient(t, opts)
		receive(t, r.legs, "the holder's stream")
		hold := receive(t, holders, "a hold inside a real server send permit")
		if hold.machineID != id {
			t.Fatal("A different machine occupied the intended send permit.")
		}
		if i == 0 {
			firstHold = hold
		}
	}

	opts := newClient(t, new(diagnostics))
	id := join(t, page, opts, r.address)
	target.Store(&id)
	approve(t, page, id, "Replay permit fixture")
	db := openObserver(t, server)
	payload := retainedPairedEvent(t, pairedFacts(0, "permit replay fixture"))
	seedPairedEvents(t, opts, payload, payload)
	run := startClient(t, opts)
	leg := receive(t, r.legs, "the target replay stream")
	receive(t, atFinal, "the target's final retained event to reach its commit seam")
	waitForAck(t, leg, 1)
	unclaimed := func() bool {
		return scalar[bool](t, db, `
SELECT count(*)=1 AND bool_and(sent_stream_id IS NULL AND attempt=1 AND NOT dispatched AND outcome IS NULL)
FROM link_commands WHERE machine_id=$1 AND action='set_state'`, id)
	}
	if !unclaimed() || mark(t, db, id) != 1 {
		t.Fatal("The target claimed its command before the retained prefix committed.")
	}
	releaseEvent()
	waitForAck(t, leg, 2)
	if !unclaimed() || len(leg.observe().commands) != 0 {
		t.Fatal("The target claimed a command while all three send permits were occupied.")
	}
	close(firstHold.release)
	standing := waitForCommand(t, leg, stateAction, 0)
	waitForResult(t, leg, standing, commandDone)
	waitForStoredResult(t, db, standing, "done")
	sends := leg.observe().sends
	ack := slices.Index(sends, frameSend{kind: "ack", number: 2})
	command := slices.Index(sends, frameSend{kind: "command", commandID: standing.id, attempt: standing.attempt})
	if ack < 0 || command <= ack {
		t.Fatal("The target's command did not follow the final replay acknowledgement on its original stream.")
	}
	run.stop(t)
	requireAcknowledgedOutbox(t, opts, 2)
}

func testPairedReplayWindow(t *testing.T) {
	t.Helper()
	held, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	server := servertest.Start(t, servertest.Options{Password: fixturePassword, LinkSeams: servertest.LinkSeams{
		BeforeEventAck: func(ctx context.Context, _ string, number uint64) {
			if number == 1 {
				close(held)
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
		},
	}})
	page := server.Clients(t)
	r := startRelay(t, server.Link())
	opts := newClient(t, new(diagnostics))
	id := join(t, page, opts, r.address)
	db := openObserver(t, server)
	payload := retainedPairedEvent(t, pairedFacts(0, "credit window fixture"))
	events := make([][]byte, 34)
	for i := range events {
		events[i] = payload
	}
	seedPairedEvents(t, opts, events...)
	run := startClient(t, opts)
	first := receive(t, r.legs, "the initial replay stream")
	receive(t, held, "the first acknowledgement to be held after commit")
	waitFor(t, 15*time.Second, "another heartbeat after the replay window filled", func() bool {
		trace := first.observe()
		return len(trace.events) >= 32 && trace.heartbeats >= 2
	})
	full := first.observe()
	if !slices.Equal(full.events, sequenceRange(1, 32)) || len(full.acks) != 0 || mark(t, db, id) != 1 {
		t.Fatalf("The replay before an acknowledgement exceeded its 32-event credit or advanced its mark: %v.", full.events)
	}
	first.cut()
	receive(t, first.done, "the held stream to close")
	unblock()
	second := receive(t, r.legs, "the same process to reconnect")
	waitForAck(t, second, 34)
	resumed := second.observe()
	if full.hello.first != 35 || full.hello.runID == "" || resumed.hello != full.hello || resumed.mark != 1 ||
		!slices.Equal(resumed.events, sequenceRange(2, 34)) || !slices.Equal(resumed.acks, sequenceRange(2, 34)) {
		t.Fatal("The same process did not repeat its run pair and resume from the committed mark.")
	}
	approve(t, page, id, "Window fixture")
	standing := waitForCommand(t, second, stateAction, 0)
	waitForResult(t, second, standing, commandDone)
	run.stop(t)
	requireAcknowledgedOutbox(t, opts, 34)
}
