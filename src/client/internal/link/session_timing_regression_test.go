package link

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestSessionSilenceExpiresDuringApprovalSetup(t *testing.T) {
	for _, tc := range []struct {
		name          string
		waitForChange bool
	}{
		{name: "approved welcome"},
		{name: "approval on a waiting stream", waitForChange: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newRunPeerFixture(t, protocols[0])
				f.seed(t)
				reading := make(chan struct{})
				release := make(chan struct{})
				releaseFacts := sync.OnceFunc(func() { close(release) })
				readFacts := runReportProbes.readFacts
				holdFirstRead := sync.OnceFunc(func() {
					close(reading)
					<-release
				})
				runReportProbes.readFacts = func(version string) (sysinfo.Facts, error) {
					holdFirstRead()
					return readFacts(version)
				}

				cancel, done := f.start(t)
				defer cancel()
				defer releaseFacts()
				leg := f.next(t, 0)
				var lastFrameAt time.Time
				if tc.waitForChange {
					f.welcome(t, leg, runWelcome(waitingApproval, 1, 0))
					time.Sleep(10 * time.Second)
					lastFrameAt = time.Now()
					approval := stateCommand(5, lastFrameAt.Add(time.Minute), 2, approvedApproval, "fixture machine")
					leg.send(t, commandFrame(approval))
				} else {
					lastFrameAt = time.Now()
					f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
				}
				receiveNow(t, reading, "the approval did not start reading machine facts")

				// The peer stays silent while the local facts read is held.
				time.Sleep(time.Until(lastFrameAt.Add(30*time.Second - time.Nanosecond)))
				requireEmpty(t, leg.closed, "the client closed the stream before 30 seconds of silence")
				time.Sleep(time.Nanosecond)
				receiveNow(t, leg.closed, "the client did not close the stream after 30 seconds of silence while the facts read was pending")

				releaseFacts()
				next := f.next(t, time.Second)
				next.hello(t)
				cancel()
				f.stop(t, done, nil)
			})
		})
	}
}

func TestRetryTeardownDoesNotCountTowardStableStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		holding, retiring, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
		releaseSender := sync.OnceFunc(func() { close(release) })
		useSessionIO(t, func(ctx context.Context, _ *session, wire runSessionIO) runSessionIO {
			send := wire.send
			wire.send = func(frame *linkv1.OpenRequest) error {
				err := send(frame)
				if err == nil && frame.GetHeartbeat() != nil {
					close(holding)
					<-ctx.Done()
					close(retiring)
					<-release
					return ctx.Err()
				}
				return err
			}
			return wire
		})

		cancel, done := f.start(t)
		defer cancel()
		defer releaseSender()
		origin := time.Now()
		first := f.next(t, 0)
		first.hello(t)
		first.refuse <- connect.NewError(connect.CodeUnavailable, nil)

		live := f.next(t, time.Second)
		if got := time.Since(origin); got != time.Second {
			t.Fatalf("the second attempt started at %v, want 1s", got)
		}
		f.welcome(t, live, runWelcome(waitingApproval, 1, 0))
		welcomedAt := time.Now()
		receiveWithin(t, holding, 10*time.Second, "the client did not start a heartbeat send")
		time.Sleep(time.Until(welcomedAt.Add(29 * time.Second)))
		live.refuse <- connect.NewError(connect.CodeUnavailable, nil)
		receiveNow(t, retiring, "the stream break did not start retiring the sender")

		time.Sleep(2 * time.Second)
		requireEmpty(t, f.peer.opens, "the client retried before its old sender retired")
		releaseSender()
		third := f.next(t, 0)
		third.hello(t)
		third.refuse <- connect.NewError(connect.CodeUnavailable, nil)

		fourth := f.next(t, 2*time.Second)
		if got := time.Since(origin); got != 34*time.Second {
			t.Errorf("the fourth attempt started at %v, want 34s with the two-second disconnected backoff retained", got)
		}
		fourth.hello(t)
		cancel()
		f.stop(t, done, nil)
	})
}
