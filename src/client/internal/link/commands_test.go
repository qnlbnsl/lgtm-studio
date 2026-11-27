package link

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

const (
	commandDone        = linkv1.CommandOutcome_COMMAND_OUTCOME_DONE
	commandFailed      = linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED
	commandExpired     = linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED
	commandUnsupported = linkv1.CommandOutcome_COMMAND_OUTCOME_UNSUPPORTED
	commandInterrupted = linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED
)

type commandReply struct {
	session  *session
	result   *linkv1.CommandResult
	deadline bool
}

type commandFixture struct {
	store   *Store
	handler *commandHandler
	stream  *session
	replies chan commandReply
	elapsed atomic.Int64
	clockOK atomic.Bool
}

func newCommandFixture(t *testing.T, store *Store, scan func(context.Context, bool) (bool, error), send func(context.Context, *session, *linkv1.CommandResult) error, logger *slog.Logger) *commandFixture {
	t.Helper()
	f := &commandFixture{store: store, replies: make(chan commandReply, 16)}
	f.elapsed.Store(int64(5 * time.Second))
	f.clockOK.Store(true)
	if send == nil {
		send = func(ctx context.Context, stream *session, msg *linkv1.CommandResult) error {
			_, deadline := ctx.Deadline()
			select {
			case f.replies <- commandReply{stream, proto.CloneOf(msg), deadline}:
				return nil
			default:
				return errors.New("too many fixture replies")
			}
		}
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	f.handler = newCommandHandler(store, scan, send, func() (time.Duration, error) {
		if !f.clockOK.Load() {
			return 0, errors.New("fixture clock unavailable")
		}
		return time.Duration(f.elapsed.Load()), nil
	}, logger)
	f.connect()
	return f
}

func (f *commandFixture) connect() {
	w := goodWelcome()
	w.ServerTime = timestamppb.New(tNow)
	f.stream = &session{welcome: w, sentAt: 5 * time.Second, sentAtOK: true}
	f.handler.setSession(f.stream)
}

func scanCommand(n int, attempt uint32, expires time.Time) *linkv1.Command {
	return &linkv1.Command{Id: fmt.Sprintf("%08x-0000-7000-8000-%012x", n, n), Attempt: attempt, ExpiresAt: timestamppb.New(expires),
		Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}}}
}

func receiptCommand(c *linkv1.Command) Command {
	return Command{ID: c.GetId(), ExpiresAt: c.GetExpiresAt().AsTime()}
}

func attemptCommand(c *linkv1.Command, attempt uint32) *linkv1.Command {
	next := proto.CloneOf(c)
	next.Attempt = attempt
	return next
}

func requireContinues(t *testing.T, f *commandFixture, cmd *linkv1.Command) {
	t.Helper()
	if restart, err := f.handler.handle(t.Context(), cmd); err != nil || restart {
		t.Fatalf("command %s attempt %d returned restart %v, error %v", cmd.GetId(), cmd.GetAttempt(), restart, err)
	}
}

func requireCommandResult(t *testing.T, got *linkv1.CommandResult, cmd *linkv1.Command, outcome linkv1.CommandOutcome, watermark ...uint64) {
	t.Helper()
	want := &linkv1.CommandResult{CommandId: cmd.GetId(), Attempt: cmd.GetAttempt(), Outcome: outcome}
	switch len(watermark) {
	case 0:
	case 1:
		want.LastEventNumber = watermark[0]
	default:
		t.Fatal("a command result must have one expected watermark")
	}
	if !proto.Equal(got, want) {
		t.Fatalf("command result id = %q, attempt = %d, outcome = %s, watermark = %d; want id %q, attempt %d, outcome %s, watermark = %d",
			got.GetCommandId(), got.GetAttempt(), got.GetOutcome(), got.GetLastEventNumber(), want.GetCommandId(), want.GetAttempt(), want.GetOutcome(), want.GetLastEventNumber())
	}
	fields, err := proto.Marshal(got)
	wantFields, wantErr := proto.Marshal(want)
	if err != nil || wantErr != nil || !bytes.Equal(fields, wantFields) {
		t.Fatal("command result did not match the expected identifier, attempt, outcome and watermark on the wire")
	}
}

func (f *commandFixture) reply(t *testing.T, cmd *linkv1.Command, outcome linkv1.CommandOutcome) commandReply {
	t.Helper()
	select {
	case reply := <-f.replies:
		if reply.session != f.stream {
			t.Fatal("reply went to the wrong stream")
		}
		requireCommandResult(t, reply.result, cmd, outcome)
		return reply
	default:
		t.Fatalf("no reply for command %s attempt %d, want %s", cmd.GetId(), cmd.GetAttempt(), outcome)
		return commandReply{}
	}
}

func (f *commandFixture) noReply(t *testing.T) {
	t.Helper()
	select {
	case <-f.replies:
		t.Fatal("unexpected command reply")
	default:
	}
}

func requireReceipt(t *testing.T, store *Store, c Command, decision Decision, result []byte) {
	t.Helper()
	got, body, err := store.Begin(c, c.ExpiresAt)
	if err != nil || got != decision || !bytes.Equal(body, result) {
		t.Fatalf("receipt lookup: decision %v result %v error %v; want %v %v", got, body, err, decision, result)
	}
}

func completeScan(t *testing.T, f *commandFixture) error {
	t.Helper()
	synctest.Wait()
	select {
	case completion := <-f.handler.completions():
		return f.handler.finishScan(t.Context(), completion)
	default:
		t.Fatal("the scan did not deliver a completion")
		return nil
	}
}

func blockScan(t *testing.T) (func(context.Context, bool) (bool, error), <-chan struct{}, func()) {
	t.Helper()
	starts := make(chan struct{}, 8)
	released := make(chan struct{})
	release := sync.OnceFunc(func() { close(released) })
	t.Cleanup(release)
	return func(ctx context.Context, requested bool) (bool, error) {
		if !requested {
			t.Error("command scan was not requested")
		}
		starts <- struct{}{}
		select {
		case <-released:
			return true, nil
		case <-ctx.Done():
			return false, nil
		}
	}, starts, release
}

func expectScan(t *testing.T, starts <-chan struct{}) {
	t.Helper()
	synctest.Wait()
	select {
	case <-starts:
	default:
		t.Fatal("no scan started")
	}
}

func TestCommandLatestAttemptAndReplay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		scan, starts, release := blockScan(t)
		f := newCommandFixture(t, store, scan, nil, nil)
		first := scanCommand(1, 1, tNow.Add(time.Minute))
		requireContinues(t, f, first)
		expectScan(t, starts)
		latest := attemptCommand(first, 3)
		requireContinues(t, f, latest)
		requireContinues(t, f, attemptCommand(first, 2))
		f.noReply(t)
		release()
		if err := completeScan(t, f); err != nil {
			t.Fatal(err)
		}
		f.reply(t, latest, commandDone)
		requireReceipt(t, store, receiptCommand(first), DecisionReplay, []byte{byte(commandDone)})
		f.elapsed.Store(int64(2 * time.Minute))
		later := attemptCommand(first, 4)
		requireContinues(t, f, later)
		f.reply(t, later, commandDone)
		if len(starts) != 0 {
			t.Fatal("a later attempt ran the scan again")
		}
		f.noReply(t)
	})
}

func TestCommandReconnectWhileScanRuns(t *testing.T) {
	for _, completeFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("completion first %v", completeFirst), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				folder, _ := newState(t)
				store := mustCreate(t, folder, StoreOptions{})
				scan, starts, release := blockScan(t)
				f := newCommandFixture(t, store, scan, nil, nil)
				first := scanCommand(1, 1, tNow.Add(time.Minute))
				requireContinues(t, f, first)
				expectScan(t, starts)
				f.handler.setSession(nil)
				f.connect()
				second := attemptCommand(first, 2)
				if !completeFirst {
					requireContinues(t, f, second)
					f.noReply(t)
				}
				release()
				if err := completeScan(t, f); err != nil {
					t.Fatal(err)
				}
				if completeFirst {
					f.noReply(t)
					requireContinues(t, f, second)
				}
				f.reply(t, second, commandDone)
				requireReceipt(t, store, receiptCommand(first), DecisionReplay, []byte{byte(commandDone)})
				if len(starts) != 0 {
					t.Fatal("reconnect ran a second scan")
				}
				f.noReply(t)
			})
		})
	}
}

func TestCommandBusySlotWritesNoReceipt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		future, past := tNow.Add(time.Minute), tNow.Add(-time.Minute)
		replayed := scanCommand(2, 2, future)
		recovered := scanCommand(3, 2, future)
		savedExpiry := scanCommand(4, 2, past)
		for _, c := range []*linkv1.Command{replayed, recovered, savedExpiry} {
			if d, _, err := store.Begin(receiptCommand(c), past.Add(-time.Second)); d != DecisionRun || err != nil {
				t.Fatalf("seeding receipt: %v, %v", d, err)
			}
		}
		if err := store.Finish(replayed.GetId(), []byte{byte(commandDone)}); err != nil {
			t.Fatal(err)
		}
		if err := store.Finish(savedExpiry.GetId(), []byte{byte(commandExpired)}); err != nil {
			t.Fatal(err)
		}
		store = reopen(t, folder, store, StoreOptions{})
		requireReceiptPreparation(t, store)
		scan, starts, release := blockScan(t)
		f := newCommandFixture(t, store, scan, nil, nil)
		active := scanCommand(1, 1, future)
		requireContinues(t, f, active)
		expectScan(t, starts)

		var writes int
		restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
			writes++
			return db.Update(fn)
		})
		defer restore()
		for _, tc := range []struct {
			cmd     *linkv1.Command
			outcome linkv1.CommandOutcome
		}{
			{replayed, commandDone},
			{recovered, commandInterrupted},
			{savedExpiry, commandExpired},
		} {
			requireContinues(t, f, tc.cmd)
			f.reply(t, tc.cmd, tc.outcome)
		}
		if writes != 0 {
			t.Fatalf("replaying ordinary receipts while busy wrote %d receipts", writes)
		}

		standing := stateCommand(5, past, 4, approvedApproval, "updated")
		requireContinues(t, f, standing)
		f.reply(t, standing, commandDone)
		requireReceipt(t, store, receiptCommand(standing), DecisionReplay, []byte{byte(commandDone)})
		if writes != 1 {
			t.Errorf("the first expired standing notice used %d writes, want one", writes)
		}
		replayedStanding := attemptCommand(standing, 2)
		requireContinues(t, f, replayedStanding)
		f.reply(t, replayedStanding, commandDone)
		if writes != 1 {
			t.Error("replaying the expired standing notice wrote again")
		}

		newExpired := scanCommand(6, 1, past)
		requireContinues(t, f, newExpired)
		f.reply(t, newExpired, commandExpired)
		requireReceipt(t, store, receiptCommand(newExpired), DecisionReplay, []byte{byte(commandExpired)})
		if writes != 2 {
			t.Fatalf("new expiry while busy performed %d cumulative writes, want 2", writes)
		}
		busy := scanCommand(7, 1, future)
		if restart, err := f.handler.handle(t.Context(), busy); restart || err == nil {
			t.Fatalf("distinct runnable scan while busy did not request a stream retry: restart %v, error %v", restart, err)
		}
		f.noReply(t)
		requireReceipt(t, store, receiptCommand(busy), DecisionExpired, nil)
		if writes != 2 {
			t.Fatal("runnable scan while busy wrote a receipt")
		}
		f.handler.setSession(nil)
		release()
		if err := completeScan(t, f); err != nil {
			t.Fatal(err)
		}
		f.noReply(t)
		f.connect()
		second := attemptCommand(busy, 2)
		requireContinues(t, f, second)
		expectScan(t, starts)
		if err := completeScan(t, f); err != nil {
			t.Fatal(err)
		}
		f.reply(t, second, commandDone)
		if len(starts) != 0 {
			t.Fatal("a runnable command used more than one slot on retry")
		}
	})
}

func TestCommandExpiryUsesServerTime(t *testing.T) {
	for _, tc := range []struct {
		name      string
		proofTime bool
		readOK    bool
		elapsed   time.Duration
		expiry    time.Duration
		runs      bool
	}{
		{"machine wall clock is behind", true, true, 6 * time.Second, time.Second, false},
		{"machine wall clock is ahead", true, true, 6 * time.Second, 2 * time.Second, true},
		{"delay between proof and admission", true, true, 17 * time.Second, 11 * time.Second, false},
		{"elapsed clock includes sleep", true, true, 2 * time.Hour, time.Hour, false},
		{"failed proof clock", false, true, 5 * time.Second, time.Hour, false},
		{"failed admission clock", true, false, 5 * time.Second, time.Hour, false},
		{"backward clock", true, true, 4 * time.Second, time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				folder, _ := newState(t)
				store := mustCreate(t, folder, StoreOptions{})
				old := wallClock
				wallClock = func() time.Time {
					if tc.runs {
						return tNow.Add(time.Hour)
					}
					return tNow.Add(-time.Hour)
				}
				defer func() { wallClock = old }()
				var scans atomic.Int32
				f := newCommandFixture(t, store, func(context.Context, bool) (bool, error) {
					scans.Add(1)
					return true, nil
				}, nil, nil)
				f.stream.sentAtOK = tc.proofTime
				f.handler.setSession(f.stream)
				f.clockOK.Store(tc.readOK)
				f.elapsed.Store(int64(tc.elapsed))
				cmd := scanCommand(1, 1, tNow.Add(tc.expiry))
				requireContinues(t, f, cmd)
				want, count := commandExpired, int32(0)
				if tc.runs {
					if err := completeScan(t, f); err != nil {
						t.Fatal(err)
					}
					want, count = commandDone, 1
				}
				f.reply(t, cmd, want)
				requireReceipt(t, store, receiptCommand(cmd), DecisionReplay, []byte{byte(want)})
				f.clockOK.Store(false)
				retry := attemptCommand(cmd, 2)
				requireContinues(t, f, retry)
				f.reply(t, retry, want)
				synctest.Wait()
				if scans.Load() != count {
					t.Errorf("scan count %d, want %d", scans.Load(), count)
				}
			})
		})
	}

	for _, failSecondRead := range []bool{false, true} {
		t.Run(fmt.Sprintf("expiry during reservation with failed clock %v", failSecondRead), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				folder, _ := newState(t)
				store := mustCreate(t, folder, StoreOptions{})
				var scans atomic.Int32
				f := newCommandFixture(t, store, func(context.Context, bool) (bool, error) {
					scans.Add(1)
					return true, nil
				}, nil, nil)
				var writes int
				restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
					if err := db.Update(fn); err != nil {
						return err
					}
					if writes == 0 {
						f.elapsed.Store(int64(6 * time.Second))
						f.clockOK.Store(!failSecondRead)
					}
					writes++
					return nil
				})
				defer restore()
				cmd := scanCommand(1, 1, tNow.Add(time.Second))
				requireContinues(t, f, cmd)
				synctest.Wait()
				// A handler may report this from admission or through its scan completion.
				select {
				case completion := <-f.handler.completions():
					if err := f.handler.finishScan(t.Context(), completion); err != nil {
						t.Fatal(err)
					}
				default:
				}
				f.reply(t, cmd, commandExpired)
				if scans.Load() != 0 || writes != 2 {
					t.Errorf("expiry during reservation ran %d scans and performed %d writes; want 0 and 2", scans.Load(), writes)
				}
				requireReceipt(t, store, receiptCommand(cmd), DecisionReplay, []byte{byte(commandExpired)})
			})
		})
	}
	t.Run("expired restart leaves active scan running", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			scan, starts, release := blockScan(t)
			f := newCommandFixture(t, store, scan, nil, nil)
			active := scanCommand(1, 1, tNow.Add(time.Minute))
			requireContinues(t, f, active)
			expectScan(t, starts)
			restart := scanCommand(2, 1, tNow)
			restart.Action = &linkv1.Command_Restart{Restart: &linkv1.Restart{}}
			requireContinues(t, f, restart)
			f.reply(t, restart, commandExpired)
			requireReceipt(t, store, receiptCommand(restart), DecisionReplay, []byte{byte(commandExpired)})
			synctest.Wait()
			select {
			case <-f.handler.completions():
				t.Fatal("expired restart cancelled the active scan")
			default:
			}
			release()
			if err := completeScan(t, f); err != nil {
				t.Fatal(err)
			}
			f.reply(t, active, commandDone)
		})
	})
}

func TestCommandWaitsForStartCommit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("commit fails %v", fail), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				folder, _ := newState(t)
				store := mustCreate(t, folder, StoreOptions{})
				scan, starts, releaseScan := blockScan(t)
				f := newCommandFixture(t, store, scan, nil, nil)
				entered := make(chan struct{})
				release := make(chan struct{})
				releaseCommit := sync.OnceFunc(func() { close(release) })
				restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
					select {
					case <-entered:
					default:
						close(entered)
						<-release
						if fail {
							return errors.New("fixture start failed")
						}
					}
					return db.Update(fn)
				})
				defer func() {
					releaseCommit()
					synctest.Wait()
					restore()
				}()
				cmd := scanCommand(1, 1, tNow.Add(time.Minute))
				type disposition struct {
					restart bool
					err     error
				}
				finished := make(chan disposition, 1)
				go func() {
					restart, err := f.handler.handle(t.Context(), cmd)
					finished <- disposition{restart, err}
				}()
				synctest.Wait()
				select {
				case <-entered:
				default:
					t.Fatal("command did not attempt to commit its start")
				}
				f.noReply(t)
				if len(starts) != 0 {
					t.Error("scan ran before its start commit returned")
				}
				select {
				case <-finished:
					t.Fatal("admission returned while its start commit was held")
				default:
				}
				releaseCommit()
				got := <-finished
				if fail {
					if got.restart || !errors.Is(got.err, ErrStoreFailed) {
						t.Errorf("failed start: restart %v, error %v", got.restart, got.err)
					}
					synctest.Wait()
					f.noReply(t)
					if len(starts) != 0 {
						t.Error("scan ran after a failed commit")
					}
					restore()
					store = reopen(t, folder, store, StoreOptions{})
					requireReceipt(t, store, receiptCommand(cmd), DecisionExpired, nil)
					return
				}
				if got.restart || got.err != nil {
					t.Fatalf("successful start: restart %v, error %v", got.restart, got.err)
				}
				expectScan(t, starts)
				releaseScan()
				if err := completeScan(t, f); err != nil {
					t.Fatal(err)
				}
				f.reply(t, cmd, commandDone)
			})
		})
	}
}

func TestCommandRestartAndRecoveredReceipt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		active := scanCommand(1, 1, tNow.Add(time.Minute))
		restart := scanCommand(2, 1, tNow.Add(time.Minute))
		restart.Action = &linkv1.Command_Restart{Restart: &linkv1.Restart{}}
		recovered := scanCommand(3, 2, tNow.Add(time.Minute))
		if d, _, err := store.Begin(receiptCommand(recovered), tNow); d != DecisionRun || err != nil {
			t.Fatalf("seeding recovered start: %v, %v", d, err)
		}
		store = reopen(t, folder, store, StoreOptions{})
		requireReceiptPreparation(t, store)
		requireRevisionTwoReceipt(t, receiptOnDisk(t, store, recovered.GetId()), "interrupted", false, recovered.GetExpiresAt().AsTime(), 0, "")
		scan, starts, _ := blockScan(t)

		checkRestartSent := false
		f := newCommandFixture(t, store, scan, func(ctx context.Context, _ *session, msg *linkv1.CommandResult) error {
			switch msg.GetCommandId() {
			case active.GetId():
				requireCommandResult(t, msg, active, commandFailed)
				requireReceipt(t, store, receiptCommand(active), DecisionReplay, []byte{byte(commandFailed)})
			case restart.GetId():
				requireCommandResult(t, msg, restart, commandDone)
				requireReceipt(t, store, receiptCommand(active), DecisionReplay, []byte{byte(commandFailed)})
				requireReceipt(t, store, receiptCommand(restart), DecisionReplay, []byte{byte(commandDone)})
				checkRestartSent = true
				deadline, ok := ctx.Deadline()
				if remaining := time.Until(deadline); !ok || remaining <= 0 || remaining > 2*time.Second {
					t.Error("restart result send did not have a deadline within two seconds")
					return errors.New("fixture send error")
				}
				<-ctx.Done()
				return ctx.Err()
			default:
				t.Error("a different command answered during restart")
			}
			return nil
		}, nil)
		requireContinues(t, f, active)
		expectScan(t, starts)
		restore := swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
			if err := db.Update(write); err != nil {
				return err
			}
			return db.View(func(tx *bolt.Tx) error {
				bucket := tx.Bucket(receiptsBucket)
				r, _, valid := readReceipt(bucket.Get([]byte(restart.GetId())), store.opts.MaxResultBytes)
				if valid && r.State == receiptFinished {
					r, outcome, valid := readReceipt(bucket.Get([]byte(active.GetId())), store.opts.MaxResultBytes)
					if !valid || r.State != receiptFinished || !bytes.Equal(outcome, []byte{byte(commandFailed)}) {
						t.Error("restart result committed before the cancelled scan was settled")
					}
				}
				return nil
			})
		})
		defer restore()
		stop, err := f.handler.handle(t.Context(), restart)
		if err != nil || !stop || !checkRestartSent {
			t.Fatalf("restart after dropped result: restart %v, send attempted %v, error %v", stop, checkRestartSent, err)
		}
		store = reopen(t, folder, store, StoreOptions{})
		requireReceiptPreparation(t, store)
		f = newCommandFixture(t, store, scan, nil, nil)
		for _, tc := range []struct {
			cmd     *linkv1.Command
			outcome linkv1.CommandOutcome
		}{
			{attemptCommand(active, 2), commandFailed},
			{attemptCommand(restart, 2), commandDone},
			{recovered, commandInterrupted},
		} {
			requireContinues(t, f, tc.cmd)
			f.reply(t, tc.cmd, tc.outcome)
		}
		synctest.Wait()
		if len(starts) != 0 {
			t.Fatal("an accepted or recovered command ran again")
		}
	})
}

func stateCommand(n int, expires time.Time, revision uint64, approval linkv1.Approval, name string) *linkv1.Command {
	cmd := scanCommand(n, 1, expires)
	cmd.Action = &linkv1.Command_SetState{SetState: &linkv1.SetState{State: &linkv1.MachineState{
		Revision: revision, Approval: approval, Name: name,
	}}}
	return cmd
}

func TestCommandStateAndUnknownAction(t *testing.T) {
	t.Run("revision and unsupported action", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			f := newCommandFixture(t, store, nil, nil, logger)
			past := tNow.Add(-time.Hour)
			approved := linkv1.Approval_APPROVAL_APPROVED
			changed := stateCommand(1, past, 4, approved, "renamed")
			for _, cmd := range []*linkv1.Command{
				changed,
				stateCommand(2, past, 2, linkv1.Approval_APPROVAL_WAITING, ""),
				stateCommand(3, past, 4, approved, "renamed"),
			} {
				requireContinues(t, f, cmd)
				f.reply(t, cmd, commandDone)
				if !proto.Equal(f.handler.standing(), changed.GetSetState().GetState()) {
					t.Fatal("late update, older revision or identical revision left the wrong standing")
				}
				requireReceipt(t, store, receiptCommand(cmd), DecisionReplay, []byte{byte(commandDone)})
			}
			before := logs.String()
			conflict := stateCommand(4, past, 4, approved, "conflicting")
			requireContinues(t, f, conflict)
			f.reply(t, conflict, commandDone)
			if !proto.Equal(f.handler.standing(), changed.GetSetState().GetState()) || logs.String() == before {
				t.Error("conflicting revision did not keep standing and log the conflict")
			}
			requireReceipt(t, store, receiptCommand(conflict), DecisionReplay, []byte{byte(commandDone)})

			unknown := scanCommand(9, 1, tNow.Add(time.Minute))
			unknown.Action = nil
			unknown.ProtoReflect().SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 100, protowire.BytesType), nil))
			requireContinues(t, f, unknown)
			f.reply(t, unknown, commandUnsupported)
			requireReceipt(t, store, receiptCommand(unknown), DecisionReplay, []byte{byte(commandUnsupported)})
		})
	})
	t.Run("removal reply uses the normal send path", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			notice := stateCommand(1, tNow.Add(-time.Minute), 4, linkv1.Approval_APPROVAL_REMOVED, "")
			sending := make(chan commandReply, 1)
			release := make(chan struct{})
			releaseReply := sync.OnceFunc(func() { close(release) })
			defer releaseReply()
			f := newCommandFixture(t, store, nil, func(ctx context.Context, stream *session, msg *linkv1.CommandResult) error {
				_, deadline := ctx.Deadline()
				sending <- commandReply{stream, proto.CloneOf(msg), deadline}
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}, nil)
			type disposition struct {
				restart bool
				err     error
			}
			finished := make(chan disposition, 1)
			go func() {
				restart, err := f.handler.handle(t.Context(), notice)
				finished <- disposition{restart, err}
			}()
			synctest.Wait()
			select {
			case msg := <-sending:
				if msg.session != f.stream || msg.deadline {
					t.Error("removal used a retired stream or an exit deadline")
				}
				requireCommandResult(t, msg.result, notice, commandDone)
				requireReceipt(t, store, receiptCommand(notice), DecisionReplay, []byte{byte(commandDone)})
			default:
				t.Fatal("removal notice did not take the regular result path")
			}
			got := f.handler.standing()
			if got.GetApproval() != linkv1.Approval_APPROVAL_WAITING || got.GetRevision() != 4 {
				t.Error("higher removal revision did not move approved standing to waiting")
			}
			// The fake clock passes any restart send budget while the reply is held.
			time.Sleep(3 * time.Second)
			synctest.Wait()
			select {
			case d := <-finished:
				t.Fatalf("removal handling ended during a held reply: restart %v, error %v", d.restart, d.err)
			default:
			}
			releaseReply()
			if got := <-finished; got.err != nil || got.restart {
				t.Errorf("removal notice ended the stream: restart %v, error %v", got.restart, got.err)
			}
			requireReceipt(t, store, receiptCommand(notice), DecisionReplay, []byte{byte(commandDone)})
		})
	})
	t.Run("invalid commands", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		var scans atomic.Int32
		f := newCommandFixture(t, store, func(context.Context, bool) (bool, error) {
			scans.Add(1)
			return true, nil
		}, nil, nil)
		valid := scanCommand(1, 1, tNow.Add(time.Minute))
		badID, zeroAttempt, noExpiry, badExpiry := proto.CloneOf(valid), proto.CloneOf(valid), proto.CloneOf(valid), proto.CloneOf(valid)
		badID.Id = "not-a-canonical-id"
		zeroAttempt.Attempt = 0
		noExpiry.ExpiresAt = nil
		badExpiry.ExpiresAt.Nanos = -1
		for i, cmd := range []*linkv1.Command{badID, zeroAttempt, noExpiry, badExpiry} {
			if restart, err := f.handler.handle(t.Context(), cmd); restart || err == nil {
				t.Errorf("invalid command %d: restart %v, error %v; want stream error", i, restart, err)
			}
		}
		f.noReply(t)
		requireReceipt(t, store, receiptCommand(valid), DecisionExpired, nil)
		if scans.Load() != 0 {
			t.Fatal("invalid command ran a scan")
		}
	})
}

func TestCommandScanRequiresDurableReport(t *testing.T) {
	for _, scenario := range []string{"committed", "full", "cancelled before commit", "overtaken", "uncertain commit"} {
		t.Run(scenario, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				folder, _ := newState(t)
				store := mustCreate(t, folder, StoreOptions{MaxEvents: 1})
				log := new(reportLog)
				var scans atomic.Int32
				started := make(chan struct{})
				allow := make(chan struct{})
				release := sync.OnceFunc(func() { close(allow) })
				defer release()
				probes := fixtureReportProbes()
				inventory := probes.scan
				probes.scan = func(ctx context.Context, opts harness.ScanOptions) []harness.Found {
					if scans.Add(1) == 1 && (scenario == "overtaken" || scenario == "cancelled before commit") {
						close(started)
						<-allow
					}
					return inventory(ctx, opts)
				}
				appendReport := store.Append
				if scenario == "uncertain commit" {
					appendReport = func(body []byte) (uint64, error) {
						seq, err := store.Append(body)
						if err != nil {
							return seq, err
						}
						return 0, ErrStoreFailed
					}
				}
				r := newReporter(config.Settings{}, "v", harness.ScanOptions{}, appendReport, log.logger(), probes)
				if scenario == "full" {
					appendLinkFixture(t, store, storedReport(t, replayFacts(tNow)))
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				f := newCommandFixture(t, store, r.scanHarnesses, nil, nil)
				cmd := scanCommand(1, 1, tNow.Add(time.Minute))
				if stop, err := f.handler.handle(ctx, cmd); stop || err != nil {
					t.Fatalf("scan admission: restart %v, error %v", stop, err)
				}
				if scenario == "overtaken" || scenario == "cancelled before commit" {
					expectScan(t, started)
					if scenario == "overtaken" {
						if committed, err := r.scanHarnesses(t.Context(), false); !committed || err != nil {
							t.Fatalf("newer inventory: commit %v, error %v", committed, err)
						}
						if err := store.Ack(1); err != nil {
							t.Fatal(err)
						}
					} else {
						cancel()
					}
					release()
				}
				err := completeScan(t, f)
				if scenario == "uncertain commit" {
					if !errors.Is(err, ErrStoreFailed) {
						t.Errorf("uncertain scan commit: %v, want ErrStoreFailed", err)
					}
					f.noReply(t)
					store = reopen(t, folder, store, StoreOptions{})
					requireReceipt(t, store, receiptCommand(cmd), DecisionInterrupted, nil)
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				want := commandFailed
				wantWatermark := uint64(1)
				if scenario == "cancelled before commit" {
					wantWatermark = 0
				}
				if scenario == "committed" {
					want = commandDone
					if event, _ := readReport(t, store, 1); event.GetHarnessesFound() == nil {
						t.Error("done scan had no durable inventory")
					}
				}
				if scenario == "cancelled before commit" && store.Next() != 1 ||
					(scenario == "full" || scenario == "overtaken") && store.Next() != 2 {
					t.Error("refused scan acquired an outbox number")
				}
				reply := receiveNow(t, f.replies, "the scan did not send a result")
				if reply.session != f.stream {
					t.Fatal("the scan replied on the wrong stream")
				}
				requireCommandResult(t, reply.result, cmd, want, wantWatermark)
				requireReceipt(t, store, receiptCommand(cmd), DecisionReplay, []byte{byte(want)})
				if scenario == "full" {
					if err := store.Ack(1); err != nil {
						t.Fatal(err)
					}
					if err := r.flush(); err != nil {
						t.Fatal(err)
					}
					if store.Next() != 2 || store.Len() != 0 {
						t.Error("refused requested scan was retried after room opened")
					}
				}
			})
		})
	}
}

func TestCommandRemovalKeepsHeartbeatsUntilVerifiedRefusal(t *testing.T) {
	for _, protocol := range protocols {
		t.Run(protocol.name, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			notice := stateCommand(1, tNow, 4, linkv1.Approval_APPROVAL_REMOVED, "")
			var attempts atomic.Int32
			seen := make(chan *linkv1.OpenRequest, 2)
			_, client, server, credential := startScene(t, protocol, func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
				switch attempts.Add(1) {
				case 1:
					if err := honest(goodWelcome())(s, stream); err != nil {
						return err
					}
					if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: notice}}); err != nil {
						return err
					}
					var result, heartbeat bool
					for !result || !heartbeat {
						frame, err := stream.Receive()
						if err != nil {
							return err
						}
						select {
						case seen <- frame:
						default:
							return connect.NewError(connect.CodeInvalidArgument, errors.New("extra fixture request"))
						}
						switch {
						case frame.GetCommandResult() != nil:
							result = true
						case frame.GetHeartbeat() != nil:
							heartbeat = true
						default:
							return connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected fixture request"))
						}
					}
					return connect.NewError(connect.CodeUnavailable, errors.New("fixture disconnection"))
				case 2:
					if _, err := s.readHello(stream); err != nil {
						return err
					}
					return linkRefusal("unknown_machine")
				default:
					hello, err := s.readHello(stream)
					if err != nil {
						return err
					}
					if err := stream.Send(s.challenge(hello, fixedServerNonce)); err != nil {
						return err
					}
					// Let the client prove that it accepted the challenge before the refusal.
					proof, err := s.readProof(stream)
					if err != nil {
						return err
					}
					if !s.verifiesProof(hello, fixedServerNonce, proof) {
						return connect.NewError(connect.CodeUnauthenticated, errors.New("fixture proof did not verify"))
					}
					return linkRefusal("unknown_machine")
				}
			})

			openCtx, cancel := context.WithTimeout(t.Context(), openCeiling)
			defer cancel()
			session, err := openSession(openCtx, client, server, credential, fixtureRunIdentity)
			if err != nil {
				t.Fatal(err)
			}
			defer session.close()
			f := newCommandFixture(t, store, nil, sendCommandFrame, nil)
			f.handler.setSession(session)
			frame, err := session.receive()
			if err != nil || !proto.Equal(frame.GetCommand(), notice) {
				t.Fatalf("reading the removal notice: %v", err)
			}
			if restart, err := f.handler.handle(openCtx, frame.GetCommand()); err != nil || restart {
				t.Fatalf("notice alone: restart %v, error %v", restart, err)
			}

			// Move to the next heartbeat opportunity directly. Item 7b owns its timer.
			probes := fixtureReportProbes()
			probes.readLoad = func() (sysinfo.Load, error) {
				return sysinfo.Load{CPUPercent: 42, MemoryPercent: 70}, nil
			}
			r := newReporter(config.Settings{}, "v", harness.ScanOptions{}, store.Append,
				slog.New(slog.NewTextHandler(io.Discard, nil)), probes)
			standing := f.handler.standing()
			if standing.GetApproval() != linkv1.Approval_APPROVAL_WAITING {
				t.Fatal("removal did not leave the machine waiting for the next heartbeat")
			}
			heartbeat := r.heartbeat(standing.GetApproval() == linkv1.Approval_APPROVAL_APPROVED)
			if err := session.send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: heartbeat}}); err != nil {
				t.Fatal(err)
			}
			if _, err = session.receive(); connect.CodeOf(err) != connect.CodeUnavailable {
				t.Fatalf("expected the ordinary disconnect after the result and heartbeat: %v", err)
			}
			for i := range 2 {
				select {
				case message := <-seen:
					if i == 0 {
						requireCommandResult(t, message.GetCommandResult(), notice, commandDone)
					} else if hb := message.GetHeartbeat(); hb == nil || hb.CpuPercent != nil || hb.MemoryPercent != nil {
						t.Fatal("waiting machine did not send a bare heartbeat after the notice")
					}
				default:
					t.Fatal("loopback server did not see the removal result and heartbeat")
				}
			}
			if _, stop := isTerminal(err); stop {
				t.Fatal("ordinary disconnect after a removal notice was terminal")
			}
			if err := session.close(); err != nil {
				t.Fatal(err)
			}

			early, err := openSession(openCtx, client, server, credential, fixtureRunIdentity)
			if early != nil {
				_ = early.close()
				t.Fatal("pre-challenge refusal unexpectedly opened a session")
			}
			wantRetryable(t, err, "unknown machine before challenge")
			verified, err := openSession(openCtx, client, server, credential, fixtureRunIdentity)
			if verified != nil {
				_ = verified.close()
				t.Fatal("verified refusal unexpectedly opened a session")
			}
			wantTerminal(t, err, "unknown_machine", "unknown machine after verified challenge")
			if attempts.Load() != 3 {
				t.Fatalf("server saw %d stream attempts, want 3", attempts.Load())
			}
		})
	}
}

func sendCommandFrame(_ context.Context, stream *session, result *linkv1.CommandResult) error {
	return stream.send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: result}})
}

func TestDeclineDecisions(t *testing.T) {
	folder, _ := newState(t)
	opts := StoreOptions{MaxResultBytes: 2}
	store := mustCreate(t, folder, opts)
	oldClock := wallClock
	wallClock = func() time.Time { return tNow }
	defer func() { wallClock = oldClock }()
	recovered := cmdAt(1, time.Hour)
	prunable := cmdAt(2, -25*time.Hour)
	malformed := cmdAt(3, time.Hour)
	cutoff := cmdAt(4, -24*time.Hour)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	putReceipts(t, folder.Paths().Store, map[string]string{
		recovered.ID: receiptJSON(recovered.ExpiresAt, receiptStarted, nil),
		prunable.ID:  receiptJSON(prunable.ExpiresAt, receiptFinished, []byte{byte(commandDone)}),
		cutoff.ID:    receiptJSON(cutoff.ExpiresAt, receiptFinished, []byte{byte(commandDone)}),
	})
	store = mustOpen(t, folder, opts)
	requireReceiptPreparation(t, store)
	running := cmdAt(5, time.Hour)
	if d, _, err := store.Begin(running, tNow); d != DecisionRun || err != nil {
		t.Fatalf("seeding running receipt: %v, %v", d, err)
	}

	var writes int
	restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
		writes++
		return db.Update(fn)
	})
	defer restore()
	expired := cmdAt(6, -time.Minute)
	expiredResult := []byte{byte(commandExpired)}
	f := newCommandFixture(t, store, nil, nil, nil)
	setReceiptClock(f, tNow, 5*time.Second, 7*time.Second, true)
	msg := scanCommand(6, 1, expired.ExpiresAt)
	expired = receiptCommand(msg)
	requireContinues(t, f, msg)
	f.reply(t, msg, commandExpired)
	if writes != 1 {
		t.Fatalf("decline of new command used %d writes, want one", writes)
	}
	requireReceipt(t, store, expired, DecisionReplay, expiredResult)
	requireReceipt(t, store, prunable, DecisionExpired, nil)
	requireReceipt(t, store, cutoff, DecisionReplay, []byte{byte(commandDone)})
	for _, tc := range []struct {
		cmd  Command
		want Decision
		body []byte
	}{
		{expired, DecisionReplay, expiredResult},
		{running, DecisionRunning, nil},
		{recovered, DecisionInterrupted, nil},
	} {
		got, body, err := store.Decline(tc.cmd, []byte("x"))
		if err != nil || got != tc.want || !bytes.Equal(body, tc.body) {
			t.Errorf("decline of existing command: decision %v, body %v, error %v; want %v, %v", got, body, err, tc.want, tc.body)
		}
	}
	for _, tc := range []struct {
		cmd  Command
		body []byte
		err  error
	}{
		{Command{ID: "bad-id", ExpiresAt: tNow}, expiredResult, ErrInvalidCommand},
		{Command{ID: uid(7)}, expiredResult, ErrInvalidCommand},
		{cmdAt(8, -time.Minute), []byte("too large"), ErrResultTooLarge},
	} {
		if _, _, err := store.Decline(tc.cmd, tc.body); !errors.Is(err, tc.err) {
			t.Errorf("decline validation: %v, want %v", err, tc.err)
		}
	}
	if writes != 1 {
		t.Errorf("existing and invalid declines performed %d additional writes", writes-1)
	}
	if err := store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(receiptsBucket).Put([]byte(malformed.ID), []byte("{"))
	}); err != nil {
		t.Fatal("could not plant the malformed receipt")
	}
	if _, _, err := store.Decline(malformed, expiredResult); !errors.Is(err, ErrStoreDamaged) || writes != 1 {
		t.Errorf("decline of a malformed receipt returned %v with %d total writes, want ErrStoreDamaged with no new write", err, writes)
	}
	if receiptOnDisk(t, store, malformed.ID) != "{" {
		t.Error("decline rewrote the malformed receipt")
	}
	if err := store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(receiptsBucket).Delete([]byte(malformed.ID))
	}); err != nil {
		t.Fatal("could not clear the malformed fixture before reopening")
	}
	store = reopen(t, folder, store, opts)
	requireReceiptPreparation(t, store)
	requireReceipt(t, store, expired, DecisionReplay, expiredResult)
	requireReceipt(t, store, running, DecisionInterrupted, nil)
}

func TestDeclineSurvivesKillBeforeFinish(t *testing.T) {
	if mode := os.Getenv("LGTM_TEST_DECLINE_KILL_MODE"); mode != "" {
		holdReceiptCommitInChild(t, mode)
		return
	}

	for _, tc := range []struct {
		mode     string
		decision Decision
		payload  []byte
	}{
		{"before decline commit", DecisionExpired, nil},
		{"after decline commit", DecisionReplay, []byte{byte(commandExpired)}},
		{"after start commit", DecisionInterrupted, nil},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			folder, dir := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			if err := folder.Close(); err != nil {
				t.Fatal(err)
			}
			killReceiptChild(t, dir, tc.mode)
			folder, err := state.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = folder.Close() })
			store = mustOpen(t, folder, StoreOptions{})
			requireReceipt(t, store, receiptCommand(scanCommand(1, 1, tNow.Add(time.Second))), tc.decision, tc.payload)
		})
	}

	t.Run("failed commit fences writes", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		var attempts int
		restore := swapCommit(func(_ *bolt.DB, _ func(*bolt.Tx) error) error {
			attempts++
			return errors.New("fixture failed commit")
		})
		defer restore()
		expired := cmdAt(1, -time.Minute)
		_, _, err := store.Decline(expired, []byte{byte(commandExpired)})
		if !errors.Is(err, ErrStoreFailed) || attempts != 1 {
			t.Fatalf("failed decline: error %v, commit attempts %d", err, attempts)
		}
		if _, err := store.Append([]byte("fixture")); !errors.Is(err, ErrStoreFailed) {
			t.Error("decline failure left later writes unfenced")
		}
		if attempts != 1 {
			t.Error("write after failed decline reached commit")
		}
		restore()
		store = reopen(t, folder, store, StoreOptions{})
		requireReceipt(t, store, expired, DecisionExpired, nil)
	})
}

func killReceiptChild(t *testing.T, dir, mode string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), openCeiling)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDeclineSurvivesKillBeforeFinish$")
	cmd.Env = append(os.Environ(), "LGTM_TEST_DECLINE_KILL_MODE="+mode, "LGTM_TEST_DECLINE_KILL_STATE="+dir)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	fromChild := bufio.NewScanner(output)
	reached := fromChild.Scan() && fromChild.Text() == "fixture commit boundary"
	if !reached {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("child did not reach %s: %q", mode, fromChild.Text())
	}
	if err := cmd.Process.Kill(); err != nil {
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("child returned successfully instead of being killed at the commit boundary")
	}
}

func holdReceiptCommitInChild(t *testing.T, mode string) {
	folder, err := state.Open(os.Getenv("LGTM_TEST_DECLINE_KILL_STATE"))
	if err != nil {
		t.Fatal(err)
	}
	defer folder.Close()
	store := mustOpen(t, folder, StoreOptions{})
	wallClock = func() time.Time { return tNow }
	var fixture *commandFixture
	restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
		if mode != "before decline commit" {
			if err := db.Update(fn); err != nil {
				return err
			}
		}
		if mode == "after start commit" {
			fixture.elapsed.Store(int64(6 * time.Second))
		}
		if _, err := fmt.Fprintln(os.Stdout, "fixture commit boundary"); err != nil {
			return err
		}
		var input [1]byte
		_, err := os.Stdin.Read(input[:])
		return err
	})
	defer restore()
	cmd := scanCommand(1, 1, tNow.Add(time.Second))
	if mode == "after start commit" {
		fixture = newCommandFixture(t, store, func(context.Context, bool) (bool, error) {
			return false, errors.New("scan ran before start returned")
		}, nil, nil)
		requireContinues(t, fixture, cmd)
	} else {
		_, _, _ = store.Decline(receiptCommand(cmd), []byte{byte(commandExpired)})
	}
	t.Fatal("receipt operation returned without a kill")
}

func TestDeclineClockOffsets(t *testing.T) {
	for _, tc := range []struct {
		name        string
		machineTime time.Time
		serverTime  time.Time
	}{
		{"two hours fast at last retry", tNow.Add(25 * time.Hour), tNow.Add(23 * time.Hour)},
		{"thirty hours fast with an hour left", tNow.Add(29 * time.Hour), tNow.Add(-time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			oldClock := wallClock
			now := tNow
			wallClock = func() time.Time { return now }
			defer func() { wallClock = oldClock }()
			payload := storedReport(t, replayFacts(tNow))
			appendLinkFixture(t, store, payload)
			original := scanCommand(1, 1, tNow)
			original.Action = &linkv1.Command_Restart{Restart: &linkv1.Restart{}}
			f := newCommandFixture(t, store, nil, nil, nil)
			setReceiptClock(f, tNow.Add(-2*time.Hour), 5*time.Second, 7*time.Second, true)
			if restart, err := f.handler.handle(t.Context(), original); err != nil || !restart {
				t.Fatal("the original restart did not run")
			}
			requireReceiptReply(t, f, original, commandDone, 1)

			now = tc.machineTime
			store = reopen(t, folder, store, StoreOptions{})
			requireReceiptPreparation(t, store)
			appendLinkFixture(t, store, payload)
			f = newCommandFixture(t, store, nil, nil, nil)
			setReceiptClock(f, tc.serverTime, 5*time.Second, 7*time.Second, true)
			later := scanCommand(2, 1, tc.serverTime.Add(-time.Minute))
			requireContinues(t, f, later)
			requireReceiptReply(t, f, later, commandExpired, 2)
			if receiptOnDisk(t, store, original.GetId()) == "" {
				t.Error("an unrelated receipt write pruned the first result by the fast local clock")
			}
			retry := attemptCommand(original, 2)
			requireContinues(t, f, retry)
			requireReceiptReply(t, f, retry, commandDone, 1)
		})
	}
}
