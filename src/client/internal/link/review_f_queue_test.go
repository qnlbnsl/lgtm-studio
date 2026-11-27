package link

import (
	"context"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestReviewFImmediateAcksSurviveOwnerStall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const savedEvents = 300
		f := newRunPeerFixture(t, protocols[0])
		payload := fixtureFactsPayload(t, "saved")
		reports := make([][]byte, savedEvents)
		for i := range reports {
			reports[i] = payload
		}
		f.seed(t, reports...)

		readingLoad, loadReady := make(chan struct{}), make(chan struct{})
		releaseLoad := sync.OnceFunc(func() { close(loadReady) })
		readLoad := runReportProbes.readLoad
		reads := 0
		runReportProbes.readLoad = func() (sysinfo.Load, error) {
			reads++
			if reads == 2 {
				close(readingLoad)
				<-loadReady
			}
			return readLoad()
		}
		receiveReady := make(chan struct{})
		releaseReceiver := sync.OnceFunc(func() { close(receiveReady) })
		useSessionIO(t, func(ctx context.Context, _ *session, wire runSessionIO) runSessionIO {
			receive := wire.receive
			wire.receive = func() (*linkv1.OpenResponse, error) {
				// Let the transport buffer replies until the owner starts its
				// slow load read. The peer sends each reply as it reads the event.
				select {
				case <-receiveReady:
					return receive()
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return wire
		})

		cancel, done := f.start(t)
		defer cancel()
		defer releaseLoad()
		defer releaseReceiver()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		var through uint64
		var outOfOrder bool
		leg.work <- func() {
			for {
				frame, err := leg.stream.Receive()
				if err != nil {
					return
				}
				if event := frame.GetEvent(); event != nil {
					number := event.GetNumber()
					if err := leg.stream.Send(ackEvent(number)); err != nil {
						return
					}
					outOfOrder = outOfOrder || number != through+1
					through = number
				} else if frame.GetHeartbeat() != nil {
					if err := leg.stream.Send(ackHeartbeat()); err != nil {
						return
					}
				}
			}
		}

		time.Sleep(reportInterval)
		receiveNow(t, readingLoad, "the owner did not enter the second heartbeat's load read")
		releaseReceiver()
		synctest.Wait()
		releaseLoad()
		synctest.Wait()

		if records := f.diagnostics(t); slices.ContainsFunc(records, func(record sessionDiagnostic) bool {
			return record.Msg == "link_disconnected" && record.Code == "receive_queue_full"
		}) {
			t.Fatal("the stream ended with receive_queue_full while the peer acknowledged each event immediately")
		}
		requireEmpty(t, leg.closed, "the peer's stream did not stay open through replay")
		if through < savedEvents || outOfOrder {
			t.Fatalf("the peer did not acknowledge all saved events in order (last event %d, want at least %d)", through, savedEvents)
		}

		cancel()
		f.stop(t, done, nil)
		store := mustOpen(t, f.opts.Folder, StoreOptions{})
		if count := store.Len(); count != 0 {
			t.Errorf("the client kept %d events after the peer acknowledged them", count)
		}
	})
}
