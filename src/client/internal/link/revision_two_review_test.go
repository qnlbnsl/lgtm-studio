package link

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
)

func TestRevisionTwoWaitingReconnectAtFullWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		window := revisionTwoEventWindow(t)
		last := 2*window + 1
		payload := fixtureFactsPayload(t, "retained")
		reports := make([][]byte, last)
		for i := range reports {
			reports[i] = payload
		}
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t, reports...)
		checkHandoffs := watchRevisionTwoEventHandoffs(t)
		cancel, done := f.start(t)
		defer cancel()

		first := f.next(t, 0)
		hello := openRevisionTwoWaitingFixture(t, f, first, 0)
		if hello.GetFirstEventNumber() != last+1 {
			t.Fatal("the run boundary did not follow the retained history")
		}
		requireRevisionTwoBareHeartbeat(t, first, 0)
		for n := uint64(1); n <= window; n++ {
			requireRevisionTwoRetainedEvent(t, first, n)
		}
		checkHandoffs(t, window)

		// The peer committed the full window, but its acknowledgement was lost.
		first.refuse <- peerError(connect.CodeUnavailable, "", "")
		second := f.next(t, 15*time.Second)
		reconnect := openRevisionTwoWaitingFixture(t, f, second, window)
		if reconnect.GetRunId() != hello.GetRunId() || reconnect.GetFirstEventNumber() != last+1 {
			t.Fatal("the reconnect changed the run or its boundary")
		}
		requireRevisionTwoBareHeartbeat(t, second, 0)
		for n := window + 1; n < last; n++ {
			requireRevisionTwoRetainedEvent(t, second, n)
		}
		checkHandoffs(t, 2*window)

		second.send(t, ackEvent(window+1))
		requireRevisionTwoRetainedEvent(t, second, last)
		checkHandoffs(t, last)
		cancel()
		f.stop(t, done, nil)
	})
}

func TestRevisionTwoWaitingHoldsEventAtRunBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		store := mustCreate(t, f.opts.Folder, StoreOptions{})
		if n, err := store.Append(fixtureFactsPayload(t, "retained")); err != nil || n != 1 {
			t.Fatal("could not append the retained fixture event")
		}
		boundary := store.Next()
		announcement := newRunIdentity(boundary)
		if n, err := store.Append(fixtureFactsPayload(t, "queued at boundary")); err != nil || n != boundary {
			t.Fatal("could not queue a fixture event at the run boundary")
		}

		// Start at the stream attempt so the sender sees the queue and the earlier announcement.
		ctx, cancel := context.WithCancel(t.Context())
		r := &clientRun{
			ctx: ctx, cancel: cancel, opts: f.opts, logger: f.opts.Logger,
			store: store, announcement: announcement,
		}
		r.reports = newReporterWithContextAppend(f.opts.Settings, f.opts.Version, f.opts.ScanOptions, store.appendContext, r.logger, runReportProbes)
		r.commands = newCommandHandler(store, r.reports.scanHarnesses, r.queueResult, nil, r.logger)
		checkHandoffs := watchRevisionTwoEventHandoffs(t)
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = r.attempt(ctx, func() {})
		}()
		defer func() {
			cancel()
			<-done
		}()

		leg := f.next(t, 0)
		hello := openRevisionTwoWaitingFixture(t, f, leg, 0)
		if hello.GetFirstEventNumber() != boundary {
			t.Fatal("the peer did not receive the run boundary for the queued event")
		}
		requireRevisionTwoBareHeartbeat(t, leg, 0)
		requireRevisionTwoRetainedEvent(t, leg, 1)
		leg.send(t, ackEvent(1))
		checkHandoffs(t, 1)

		leg.send(t, commandFrame(stateCommand(1, time.Now().Add(time.Minute), 2, approvedApproval, "fixture machine")))
		frame := leg.next(t)
		if frame.GetCommandResult() != nil {
			frame = leg.next(t)
		}
		event := frame.GetEvent()
		if event.GetNumber() != boundary || event.GetFacts().GetClientVersion() != "queued at boundary" {
			t.Fatalf("approval sent event %d, want the queued boundary event %d", event.GetNumber(), boundary)
		}
	})
}
