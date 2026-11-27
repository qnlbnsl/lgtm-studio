package link

import (
	"context"
	"errors"
	"io"
	"testing"
	"testing/synctest"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestReviewFRestartHalfClosesBeforeCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		serving := make(chan context.Context, 1)
		useSessionIO(t, func(ctx context.Context, _ *session, wire runSessionIO) runSessionIO {
			serving <- ctx
			return wire
		})
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		serveCtx := receiveNow(t, serving, "the client did not enter the live session")
		cmd := scanCommand(1, 1, time.Now().Add(time.Minute))
		cmd.Action = &linkv1.Command_Restart{Restart: &linkv1.Restart{}}

		type requestEnd struct {
			result               *linkv1.CommandResult
			cleanEOF, beforeStop bool
		}
		observed := make(chan requestEnd, 1)
		leg.work <- func() {
			var end requestEnd
			if err := leg.stream.Send(commandFrame(cmd)); err == nil {
				for {
					frame, err := leg.stream.Receive()
					if err != nil {
						end.cleanEOF = errors.Is(err, io.EOF)
						end.beforeStop = serveCtx.Err() == nil
						break
					}
					if result := frame.GetCommandResult(); result != nil {
						end.result = result
					}
				}
			}
			observed <- end
		}
		end := receiveNow(t, observed, "the peer did not observe the restart result and request EOF before cancellation")
		if end.result == nil || !end.cleanEOF || !end.beforeStop {
			t.Fatalf("the peer did not observe the restart result and request EOF before cancellation (result %t, EOF %t, still serving %t)",
				end.result != nil, end.cleanEOF, end.beforeStop)
		}
		wantCommandResult(t, end.result, cmd, commandDone)
		requireEmpty(t, leg.closed, "the response closed before the peer ended it")
		requireEmpty(t, done, "the client restarted while the peer held the response open")
		if serveCtx.Err() != nil {
			t.Fatal("the client cancelled the stream while the peer held the response open")
		}

		leg.refuse <- nil
		result := receiveNow(t, done, "the client did not restart after the peer ended its response")
		if result.outcome != RunRestart || result.err != nil {
			t.Fatal("the client did not return a successful restart outcome")
		}
	})
}
