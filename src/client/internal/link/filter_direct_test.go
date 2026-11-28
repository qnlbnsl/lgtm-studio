package link

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	bolt "go.etcd.io/bbolt"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestDirectScreeningWaitsBeforeHeartbeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		read, heartbeat := make(chan struct{}), make(chan struct{})
		release := make(chan struct{})
		markRead := sync.OnceFunc(func() { close(read) })
		releaseRead := sync.OnceFunc(func() { close(release) })
		f.opts.Filter = NewRefreshingOutgoingFilter(func() []string {
			markRead()
			<-release
			return nil
		})
		markHeartbeat := sync.OnceFunc(func() { close(heartbeat) })
		useSessionIO(t, func(_ context.Context, _ *session, wire runSessionIO) runSessionIO {
			send := wire.send
			wire.send = func(frame *linkv1.OpenRequest) error {
				if frame.GetHeartbeat() != nil {
					markHeartbeat()
				}
				return send(frame)
			}
			return wire
		})
		cancel, done := f.start(t)
		defer cancel()
		defer releaseRead()

		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(waitingApproval, 1, 0))
		select {
		case <-read:
		default:
			t.Error("the waiting client did not read the sign-in file before its first heartbeat")
		}
		select {
		case <-heartbeat:
			t.Error("a heartbeat reached the sender before the sign-in reader was released")
		default:
		}

		releaseRead()
		if leg.next(t).GetHeartbeat() == nil {
			t.Fatal("the waiting peer did not receive its heartbeat after release")
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func TestDirectScreeningOverlappingReportsDoNotWaitForHeartbeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		const secret = "fixture-sign-in-for-direct-screen"
		f.opts.Version = "before-" + secret + "-after"
		f.opts.Filter = NewRefreshingOutgoingFilter(func() []string { return []string{secret} })

		firstHandoff, release := make(chan struct{}), make(chan struct{})
		useSessionIO(t, func(ctx context.Context, _ *session, wire runSessionIO) runSessionIO {
			send := wire.send
			wire.send = func(frame *linkv1.OpenRequest) error {
				if frame.GetEvent().GetNumber() == 1 {
					close(firstHandoff)
					select {
					case <-release:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return send(frame)
			}
			return wire
		})
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)

		before := time.Now()
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		receiveNow(t, firstHandoff, "the first report did not reach the sender at the current tick")
		buffered := f.diagnostics(t)
		if !slices.ContainsFunc(buffered, func(record sessionDiagnostic) bool {
			return record.Msg == "link_event_buffered" && record.Sequence == 2
		}) {
			t.Fatal("the second report was not buffered while the first send was in progress")
		}
		close(release)

		first, second := eventOnLeg(t, leg), eventOnLeg(t, leg)
		if first.GetNumber() != 1 || first.GetFacts().GetClientVersion() != "before-[redacted]-after" ||
			second.GetNumber() != 2 || second.GetHarnessesFound() == nil {
			t.Error("the signed peer did not receive the masked facts followed by the inventory")
		}
		if !time.Now().Equal(before) {
			t.Error("the sender waited for a clock advance with two healthy reports ready")
		}
		leg.send(t, ackEvent(2))
		cancel()
		f.stop(t, done, nil)
	})
}

func TestDirectScreeningLeavesStoreAvailableDuringRead(t *testing.T) {
	for _, operation := range []string{"report admission", "retained replay"} {
		t.Run(operation, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			event := replayFacts(tNow)
			payload := storedReport(t, event)
			appendLinkFixture(t, store, payload)

			reading, release := make(chan struct{}), make(chan struct{})
			releaseRead := sync.OnceFunc(func() { close(release) })
			markReading := sync.OnceFunc(func() { close(reading) })
			defer releaseRead()
			filter := NewRefreshingOutgoingFilter(func() []string {
				markReading()
				<-release
				return nil
			})
			screened := make(chan error, 1)
			go func() {
				if operation == "report admission" {
					number, err := store.appendFilteredReport(t.Context(), filter, event)
					if err == nil && number != 3 {
						err = fmt.Errorf("screened report number = %d, want 3", number)
					}
					screened <- err
					return
				}
				_, err := newReplayWithFilter(store, 0, filter)
				screened <- err
			}()
			select {
			case <-reading:
			case err := <-screened:
				t.Fatalf("screening completed before reading the sign-in file: %v", err)
			}

			if store.mu.TryLock() {
				store.mu.Unlock()
				if number, err := store.Append(payload); err != nil || number != 2 {
					t.Errorf("append during the held read = (%d, %v), want (2, nil)", number, err)
				}
			} else {
				t.Error("screening held the store lock while reading the sign-in file")
			}
			releaseRead()
			if err := <-screened; err != nil {
				t.Errorf("screening after release failed: %v", err)
			}
		})
	}
}

func TestDirectScreeningRestartDrainsQueuedResults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		f.opts.Filter = NewRefreshingOutgoingFilter(func() []string { return nil })
		welcome := runWelcome(approvedApproval, 1, 0)
		expiry := time.Now().Add(time.Minute)
		commands := make([]*linkv1.Command, 0, 4)
		for i := range 3 {
			commands = append(commands, stateCommand(i+1, expiry, welcome.State.Revision, approvedApproval, "conflicting fixture state"))
		}
		restart := scanCommand(4, 1, expiry)
		restart.Action = &linkv1.Command_Restart{Restart: &linkv1.Restart{}}
		commands = append(commands, restart)

		savedRestart := make(chan struct{})
		markRestart := sync.OnceFunc(func() { close(savedRestart) })
		t.Cleanup(swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
			var finished bool
			err := db.Update(func(tx *bolt.Tx) error {
				if err := write(tx); err != nil {
					return err
				}
				record, _, ok := readReceipt(tx.Bucket(receiptsBucket).Get([]byte(restart.GetId())), 1)
				finished = ok && record.State == receiptFinished
				return nil
			})
			if err == nil && finished {
				markRestart()
			}
			return err
		}))

		firstHandoff, release := make(chan struct{}), make(chan struct{})
		useSessionIO(t, func(ctx context.Context, _ *session, wire runSessionIO) runSessionIO {
			send := wire.send
			wire.send = func(frame *linkv1.OpenRequest) error {
				if frame.GetCommandResult().GetCommandId() == commands[0].GetId() {
					close(firstHandoff)
					select {
					case <-release:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return send(frame)
			}
			return wire
		})
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, welcome)
		leg.send(t, commandFrame(commands[0]))
		receiveNow(t, firstHandoff, "the first command result did not reach the blocked sender")
		for _, command := range commands[1:] {
			leg.send(t, commandFrame(command))
		}
		receiveNow(t, savedRestart, "the Restart receipt did not finish with earlier results still queued")
		close(release)

		var results []*linkv1.CommandResult
		requestEnd := make(chan error, 1)
		leg.work <- func() {
			for {
				frame, err := leg.stream.Receive()
				if err != nil {
					requestEnd <- err
					return
				}
				if result := frame.GetCommandResult(); result != nil {
					results = append(results, result)
				}
			}
		}
		if err := receiveNow(t, requestEnd, "the peer did not receive request EOF after the queued results"); !errors.Is(err, io.EOF) {
			t.Fatalf("reading the request through EOF returned %v", err)
		}
		if len(results) != len(commands) {
			t.Fatalf("the peer received %d results before EOF, want %d", len(results), len(commands))
		}
		for i, result := range results {
			wantCommandResult(t, result, commands[i], commandDone)
		}
		leg.refuse <- nil
		run := receiveNow(t, done, "the client did not stop after handing off the Restart result")
		if run.outcome != RunRestart || run.err != nil {
			t.Errorf("the client did not return a successful Restart outcome: %v", run)
		}
	})
}
