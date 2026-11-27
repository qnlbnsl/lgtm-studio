package link

import (
	"testing"
	"testing/synctest"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestReviewGUnacknowledgedEventWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const wantEvents = 32
		f := newRunPeerFixture(t, protocols[0])
		payload := fixtureFactsPayload(t, "saved")
		reports := make([][]byte, 80)
		for i := range reports {
			reports[i] = payload
		}
		f.seed(t, reports...)

		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		welcome := runWelcome(approvedApproval, 1, 0)
		f.welcome(t, leg, welcome)
		cmd := stateCommand(1, time.Now().Add(time.Minute), welcome.State.Revision, approvedApproval, welcome.State.Name)
		results := make(chan *linkv1.CommandResult, 1)
		var events []uint64
		beats := 0
		leg.work <- func() {
			for {
				frame, err := leg.stream.Receive()
				if err != nil {
					return
				}
				switch {
				case frame.GetEvent() != nil:
					events = append(events, frame.GetEvent().GetNumber())
				case frame.GetHeartbeat() != nil:
					beats++
					// Keep the peer silent until the next heartbeat, after replay fills the window.
					if beats == 2 {
						if err := leg.stream.Send(ackHeartbeat()); err != nil {
							return
						}
						if err := leg.stream.Send(commandFrame(cmd)); err != nil {
							return
						}
					}
				case frame.GetCommandResult() != nil:
					results <- frame.GetCommandResult()
				}
			}
		}
		synctest.Wait()
		if len(events) != wantEvents {
			t.Fatalf("The silent peer received %d events, want %d.", len(events), wantEvents)
		}
		for i, number := range events {
			if number != uint64(i+1) {
				t.Fatalf("The silent peer received event %d at position %d.", number, i+1)
			}
		}

		time.Sleep(reportInterval)
		result := receiveNow(t, results, "The peer did not receive a command result while the event window was full.")
		wantCommandResult(t, result, cmd, commandDone)
		// Pass the original silence deadline while keeping all events unacknowledged.
		time.Sleep(streamSilence - reportInterval + time.Nanosecond)
		requireEmpty(t, leg.closed, "The original stream closed after the heartbeat acknowledgement and command.")
		requireEmpty(t, f.peer.opens, "The client opened another stream while the event window was full.")
		if len(events) != wantEvents {
			t.Errorf("The peer received %d events without an event acknowledgement, want %d.", len(events), wantEvents)
		}

		cancel()
		f.stop(t, done, nil)
	})
}
