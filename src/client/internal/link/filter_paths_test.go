package link

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	bolt "go.etcd.io/bbolt"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestDirectScreeningReadsBeforeRetainedEventHandoff(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	const value = "retained-event-sign-in-fixture"
	var values []string
	filter := NewRefreshingOutgoingFilter(func() []string { return values })
	cursor, err := newReplayWithFilter(store, 0, filter)
	if err != nil {
		t.Fatal("the empty store could not start a replay")
	}
	event := replayFacts(tNow)
	event.GetFacts().ClientVersion = "before-" + value + "-after"
	appendLinkFixture(t, store, storedReport(t, event))
	values = []string{value}

	handedOff := false
	got, err := cursor.next(func(*linkv1.Event) { handedOff = true })
	if handedOff || got != nil {
		t.Error("the replay exposed a retained event containing the new sign-in value")
	}
	if !errors.Is(err, ErrStoreDamaged) {
		t.Error("the replay did not report a damaged store for the required event")
	}
}

func TestDirectScreeningWaitsBeforeCommandResult(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		welcome := runWelcome(waitingApproval, 1, 0)
		command := stateCommand(413, time.Now().Add(time.Minute), 2, waitingApproval, "")

		// The command may pass admission. The new value appears after its receipt commits.
		saved, reading, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
		markSaved := sync.OnceFunc(func() { close(saved) })
		markReading := sync.OnceFunc(func() { close(reading) })
		releaseRead := sync.OnceFunc(func() { close(release) })
		t.Cleanup(swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
			var finished bool
			err := db.Update(func(tx *bolt.Tx) error {
				if err := write(tx); err != nil {
					return err
				}
				record, _, ok := readReceipt(tx.Bucket(receiptsBucket).Get([]byte(command.GetId())), 1)
				finished = ok && record.State == receiptFinished
				return nil
			})
			if err == nil && finished {
				markSaved()
			}
			return err
		}))
		f.opts.Filter = NewRefreshingOutgoingFilter(func() []string {
			select {
			case <-saved:
				markReading()
				<-release
				return []string{command.GetId()}
			default:
				return nil
			}
		})

		sent := make(chan struct{})
		markSent := sync.OnceFunc(func() { close(sent) })
		useSessionIO(t, func(_ context.Context, _ *session, wire runSessionIO) runSessionIO {
			send := wire.send
			wire.send = func(frame *linkv1.OpenRequest) error {
				select {
				case <-saved:
					markSent()
				default:
				}
				return send(frame)
			}
			return wire
		})
		cancel, done := f.start(t)
		defer cancel()
		defer releaseRead()

		leg := f.next(t, 0)
		f.welcome(t, leg, welcome)
		if leg.next(t).GetHeartbeat() == nil {
			t.Fatal("the waiting session did not send its initial heartbeat")
		}
		leg.send(t, commandFrame(command))
		receiveNow(t, saved, "the command result was never recorded")
		requireEmpty(t, sent, "a frame reached the sender before the sign-in read returned")
		receiveNow(t, reading, "the queued result did not start a fresh sign-in read")

		releaseRead()
		requireEmpty(t, sent, "the session sent a frame after the read exposed the unsafe result identifier")
		if !slices.ContainsFunc(f.diagnostics(t), func(record sessionDiagnostic) bool {
			return record.Msg == "link_disconnected" && record.Code == "invalid_frame"
		}) {
			t.Error("the session did not refuse the result as an invalid frame")
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func TestDirectScreeningReadsAtCommandAdmission(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	f := newCommandFixture(t, store, nil, nil, nil)
	var values []string
	f.handler.filter = NewRefreshingOutgoingFilter(func() []string { return values })
	command := stateCommand(414, tNow.Add(time.Minute), 2, approvedApproval, "fixture state")
	if f.handler.filter.ContainsSecret(command.GetId()) {
		t.Fatal("the command identifier was already known before admission")
	}
	values = []string{command.GetId()}

	restart, err := f.handler.handle(t.Context(), command)
	if restart || !errors.Is(err, ErrInvalidCommand) {
		t.Error("admission did not reject the command with the newly unsafe identifier as invalid")
	}
	f.noReply(t)
}
