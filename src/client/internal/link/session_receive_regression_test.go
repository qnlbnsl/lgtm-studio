package link

import (
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
)

func holdApprovalFacts() (<-chan struct{}, func()) {
	reading, release := make(chan struct{}), make(chan struct{})
	readFacts := runReportProbes.readFacts
	holdFirstRead := sync.OnceFunc(func() {
		close(reading)
		<-release
	})
	runReportProbes.readFacts = func(version string) (sysinfo.Facts, error) {
		holdFirstRead()
		return readFacts(version)
	}
	return reading, sync.OnceFunc(func() { close(release) })
}

func TestSessionReceivesHeartbeatsDuringApprovalSetup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		reading, releaseFacts := holdApprovalFacts()
		cancel, done := f.start(t)
		defer cancel()
		defer releaseFacts()

		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		receiveNow(t, reading, "the approval did not start reading machine facts")

		time.Sleep(time.Second)
		leg.send(t, ackHeartbeat())
		time.Sleep(28 * time.Second)
		leg.send(t, ackHeartbeat())
		lastFrameAt := time.Now()

		time.Sleep(2 * time.Second)
		requireEmpty(t, leg.closed, "the stream closed 2 seconds after the peer's second heartbeat acknowledgement")
		time.Sleep(time.Until(lastFrameAt.Add(30*time.Second - time.Nanosecond)))
		requireEmpty(t, leg.closed, "the stream closed before 30 seconds of silence after the second acknowledgement")
		time.Sleep(time.Nanosecond)
		receiveNow(t, leg.closed, "the stream stayed open after 30 seconds of silence after the second acknowledgement")

		cancel()
		releaseFacts()
		f.stop(t, done, nil)
	})
}

func TestRetryProtocolRejectionUsesFrameReceiptTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		reading, releaseFacts := holdApprovalFacts()
		cancel, done := f.start(t)
		defer cancel()
		defer releaseFacts()

		first := f.next(t, 0)
		first.hello(t)
		first.refuse <- connect.NewError(connect.CodeUnavailable, nil)

		live := f.next(t, time.Second)
		f.welcome(t, live, runWelcome(approvedApproval, 1, 0))
		receiveNow(t, reading, "the approval did not start reading machine facts")

		time.Sleep(29 * time.Second)
		// A second Welcome is a known frame that is invalid on a live stream.
		live.send(t, welcomeFrame(runWelcome(waitingApproval, 2, 0)))
		time.Sleep(2 * time.Second)
		releaseFacts()

		third := f.next(t, 0)
		thirdAt := time.Now()
		if !slices.ContainsFunc(f.diagnostics(t), func(record sessionDiagnostic) bool {
			return record.Msg == "link_disconnected" && record.Code == "invalid_frame"
		}) {
			t.Error("the repeated welcome did not produce an invalid-frame diagnostic")
		}
		third.hello(t)
		third.refuse <- connect.NewError(connect.CodeUnavailable, nil)

		fourth := f.next(t, 2*time.Second)
		if got := time.Since(thirdAt); got != 2*time.Second {
			t.Errorf("the next dial followed the failed attempt after %v, want 2s with the previous backoff retained", got)
		}
		fourth.hello(t)
		cancel()
		f.stop(t, done, nil)
	})
}
