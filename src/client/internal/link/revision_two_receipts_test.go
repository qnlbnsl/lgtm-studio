package link

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func setReceiptClock(f *commandFixture, serverTime time.Time, proof, welcome time.Duration, trusted bool) {
	f.stream.welcome.ServerTime = timestamppb.New(serverTime)
	f.stream.sentAt = proof
	f.stream.receivedAt, f.stream.receivedAtOK = welcome, trusted
	f.elapsed.Store(int64(welcome))
	f.handler.setSession(f.stream)
}

func requireReceiptPreparation(t *testing.T, store *Store) {
	t.Helper()
	if err := store.prepareRun(); err != nil {
		t.Fatal("receipt preparation failed")
	}
}

func requireResultWatermark(t *testing.T, result *linkv1.CommandResult, cmd *linkv1.Command, outcome linkv1.CommandOutcome, watermark uint64) {
	t.Helper()
	want := &linkv1.CommandResult{CommandId: cmd.GetId(), Attempt: cmd.GetAttempt(), Outcome: outcome, LastEventNumber: watermark}
	if !proto.Equal(result, want) {
		t.Errorf("attempt %d returned outcome %s and watermark %d, want %s and %d with the current identifier and attempt",
			cmd.GetAttempt(), result.GetOutcome(), result.GetLastEventNumber(), outcome, watermark)
	}
}

func requireReceiptReply(t *testing.T, f *commandFixture, cmd *linkv1.Command, outcome linkv1.CommandOutcome, watermark uint64) {
	t.Helper()
	select {
	case reply := <-f.replies:
		if reply.session != f.stream {
			t.Fatal("the receipt result went to an older stream")
		}
		requireResultWatermark(t, reply.result, cmd, outcome, watermark)
	default:
		t.Fatal("the command did not answer on the current attempt")
	}
}

func receiptOnDisk(t *testing.T, store *Store, id string) string {
	t.Helper()
	var raw string
	if err := store.db.View(func(tx *bolt.Tx) error {
		raw = string(tx.Bucket(receiptsBucket).Get([]byte(id)))
		return nil
	}); err != nil {
		t.Fatal("could not read the receipt")
	}
	return raw
}

func requireRevisionTwoReceipt(t *testing.T, raw, state string, standing bool, expiry time.Time, watermark uint64, result string) {
	t.Helper()
	var envelope struct {
		Version   int             `json:"version"`
		ExpiresAt time.Time       `json:"expires_at"`
		State     string          `json:"state"`
		Standing  *bool           `json:"standing"`
		Watermark *uint64         `json:"last_event_number"`
		Result    json.RawMessage `json:"result"`
	}
	if json.Unmarshal([]byte(raw), &envelope) != nil || envelope.Version != 2 || envelope.State != state ||
		envelope.Standing == nil || *envelope.Standing != standing || !envelope.ExpiresAt.Equal(expiry) ||
		envelope.Watermark == nil || *envelope.Watermark != watermark || string(envelope.Result) != result {
		t.Errorf("the saved receipt did not have the expected revision 2 %s envelope, standing %t, watermark %d", state, standing, watermark)
	}
}

func TestRevisionTwoReceiptWatermarks(t *testing.T) {
	for _, tc := range []struct {
		name      string
		outcome   linkv1.CommandOutcome
		watermark uint64
		result    string
	}{
		{"scan", commandDone, 3, `"AQ=="`},
		{"restart", commandDone, 2, `"AQ=="`},
		{"failed append", commandFailed, 2, `"Ag=="`},
		{"expired", commandExpired, 2, `"Aw=="`},
		{"unsupported", commandUnsupported, 2, `"BA=="`},
		{"expired standing", commandDone, 2, `"AQ=="`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				folder, _ := newState(t)
				opts := StoreOptions{MaxEvents: 1}
				store := mustCreate(t, folder, opts)
				payload := storedReport(t, replayFacts(tNow))
				appendLinkFixture(t, store, payload)
				if err := store.Ack(1); err != nil {
					t.Fatal("could not acknowledge the first fixture event")
				}
				appendLinkFixture(t, store, payload)
				if tc.name != "failed append" && tc.name != "restart" {
					if err := store.Ack(2); err != nil {
						t.Fatal("could not empty the fixture queue")
					}
				}

				future, past := tNow.Add(time.Hour), tNow.Add(-time.Hour)
				cmd := scanCommand(1, 1, future)
				var scans atomic.Int32
				f := newCommandFixture(t, store, func(context.Context, bool) (bool, error) {
					scans.Add(1)
					_, err := store.Append(payload)
					if errors.Is(err, ErrOutboxFull) {
						return false, nil
					}
					return err == nil, err
				}, nil, nil)
				var scansWanted int32
				switch tc.name {
				case "scan", "failed append":
					scansWanted = 1
				case "restart":
					cmd.Action = &linkv1.Command_Restart{Restart: &linkv1.Restart{}}
				case "expired":
					cmd.ExpiresAt = timestamppb.New(past)
				case "unsupported":
					cmd.Action = nil
					cmd.ProtoReflect().SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 100, protowire.BytesType), nil))
				case "expired standing":
					cmd = stateCommand(1, past, 8, waitingApproval, "")
					f.stream.welcome.State.Approval = waitingApproval
					f.stream.welcome.State.Revision++
				}
				setReceiptClock(f, tNow, 5*time.Second, 7*time.Second, true)
				restart, err := f.handler.handle(t.Context(), cmd)
				if err != nil || restart != (tc.name == "restart") {
					t.Fatalf("first command returned restart %t and error present %t", restart, err != nil)
				}
				if scansWanted == 1 {
					if err := completeScan(t, f); err != nil {
						t.Fatal("could not settle the first scan")
					}
				}
				requireReceiptReply(t, f, cmd, tc.outcome, tc.watermark)
				if store.Next() != tc.watermark+1 {
					t.Fatal("the command left the wrong durable event counter")
				}
				first := receiptOnDisk(t, store, cmd.GetId())
				requireRevisionTwoReceipt(t, first, "finished", tc.name == "expired standing", cmd.GetExpiresAt().AsTime(), tc.watermark, tc.result)

				if err := store.Ack(tc.watermark); err != nil || store.Len() != 0 {
					t.Fatal("could not clear the acknowledged queue")
				}
				appendLinkFixture(t, store, payload)
				for attempt := uint32(2); attempt <= 4; attempt++ {
					if attempt > 2 {
						store = reopen(t, folder, store, opts)
						requireReceiptPreparation(t, store)
						f = newCommandFixture(t, store, func(context.Context, bool) (bool, error) {
							scans.Add(1)
							return false, nil
						}, nil, nil)
					} else {
						f.connect()
					}
					setReceiptClock(f, tNow.Add(2*time.Hour), 5*time.Second, 7*time.Second, true)
					retry := attemptCommand(cmd, attempt)
					requireContinues(t, f, retry)
					requireReceiptReply(t, f, retry, tc.outcome, tc.watermark)
					if got := receiptOnDisk(t, store, cmd.GetId()); got != first {
						t.Error("replay changed the first receipt bytes")
					}
				}
				synctest.Wait()
				f.noReply(t)
				if scans.Load() != scansWanted || store.Next() != tc.watermark+2 {
					t.Error("a replay repeated work or consumed an event number")
				}
			})
		})
	}

	t.Run("standing survives ordinary pruning", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		payload := storedReport(t, replayFacts(tNow))
		appendLinkFixture(t, store, payload)
		f := newCommandFixture(t, store, nil, nil, nil)
		setReceiptClock(f, tNow, 5*time.Second, 7*time.Second, true)
		expired := tNow.Add(-time.Minute)
		standing := stateCommand(1, expired, 8, approvedApproval, "first")
		ordinary := scanCommand(2, 1, expired)
		requireContinues(t, f, standing)
		requireReceiptReply(t, f, standing, commandDone, 1)
		requireContinues(t, f, ordinary)
		requireReceiptReply(t, f, ordinary, commandExpired, 1)
		saved := receiptOnDisk(t, store, standing.GetId())
		if receiptOnDisk(t, store, ordinary.GetId()) == "" {
			t.Fatal("the ordinary expiry did not leave a receipt to prune")
		}

		store = reopen(t, folder, store, StoreOptions{})
		requireReceiptPreparation(t, store)
		appendLinkFixture(t, store, payload)
		f = newCommandFixture(t, store, nil, nil, nil)
		setReceiptClock(f, tNow.Add(25*time.Hour), 5*time.Second, 7*time.Second, true)
		newest := stateCommand(3, tNow.Add(time.Hour), 9, approvedApproval, "newest")
		requireContinues(t, f, newest)
		requireReceiptReply(t, f, newest, commandDone, 2)
		pruneTrigger := scanCommand(4, 1, tNow.Add(23*time.Hour))
		requireContinues(t, f, pruneTrigger)
		requireReceiptReply(t, f, pruneTrigger, commandExpired, 2)
		if receiptOnDisk(t, store, ordinary.GetId()) != "" {
			t.Error("verified elapsed time did not prune the old ordinary receipt")
		}
		if got := receiptOnDisk(t, store, standing.GetId()); got == "" || got != saved {
			t.Error("ordinary pruning removed or changed the older standing receipt")
		}
		retry := attemptCommand(standing, 2)
		requireContinues(t, f, retry)
		requireReceiptReply(t, f, retry, commandDone, 1)
		if !proto.Equal(f.handler.standing(), newest.GetSetState().GetState()) {
			t.Error("replaying the older standing receipt changed the newest applied state")
		}
	})
}

const (
	receiptKillState = "LGTM_TEST_REVISION_TWO_RECEIPT_KILL_STATE"
	receiptKillReady = "receipt start and later events are durable"
)

func runInterruptedReceiptChild(t *testing.T, dir string) {
	folder, err := state.Open(dir)
	if err != nil {
		t.Fatal("could not open the child receipt store")
	}
	defer folder.Close()
	store := mustOpen(t, folder, StoreOptions{})
	payload := storedReport(t, replayFacts(tNow))
	appendLinkFixture(t, store, payload)
	for _, n := range []int{71, 72} {
		cmd := receiptCommand(scanCommand(n, 1, tNow.Add(time.Hour)))
		if got, _, err := store.Begin(cmd, tNow); err != nil || got != DecisionRun {
			t.Fatal("the child did not save the start")
		}
	}
	appendLinkFixture(t, store, payload)
	appendLinkFixture(t, store, payload)
	if _, err := fmt.Fprintln(os.Stdout, receiptKillReady); err != nil {
		t.Fatal("the child could not signal the durable boundary")
	}
	var input [1]byte
	_, _ = os.Stdin.Read(input[:])
	t.Fatal("the child returned without being killed")
}

func killInterruptedReceiptChild(t *testing.T, dir string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), openCeiling)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRevisionTwoReceiptMetadata$")
	child.Env = append(os.Environ(), receiptKillState+"="+dir)
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal("could not create the child hold pipe")
	}
	defer input.Close()
	output, err := child.StdoutPipe()
	if err != nil || child.Start() != nil {
		t.Fatal("could not run the receipt child")
	}
	marker := bufio.NewScanner(output)
	reached := marker.Scan() && marker.Text() == receiptKillReady
	killErr := child.Process.Kill()
	var exit *exec.ExitError
	if err := child.Wait(); !reached || killErr != nil || !errors.As(err, &exit) {
		t.Fatal("the child did not stop at the durable receipt boundary")
	}
	if status, ok := exit.Sys().(syscall.WaitStatus); !ok || status.Signal() != syscall.SIGKILL {
		t.Fatal("the child did not terminate from the kill signal")
	}
}

func revisionTwoReceiptJSON(expiry time.Time, state, fields string) string {
	return fmt.Sprintf(`{"version":2,"expires_at":%q,"state":%q,"standing":false%s}`,
		expiry.UTC().Format(time.RFC3339), state, fields)
}

func TestRevisionTwoReceiptMetadata(t *testing.T) {
	if dir := os.Getenv(receiptKillState); dir != "" {
		runInterruptedReceiptChild(t, dir)
		return
	}

	t.Run("seal the dead process before later appends", func(t *testing.T) {
		folder, dir := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		path := folder.Paths().Store
		if err := errors.Join(store.Close(), folder.Close()); err != nil {
			t.Fatal("could not close the initial store before the child starts")
		}
		killInterruptedReceiptChild(t, dir)
		folder, err := state.Open(dir)
		if err != nil {
			t.Fatal("could not reopen the store after the kill")
		}
		t.Cleanup(func() { _ = folder.Close() })
		before := readBytes(t, path)
		store = mustOpen(t, folder, StoreOptions{})
		if !bytes.Equal(readBytes(t, path), before) {
			t.Fatal("OpenStore wrote bytes before receipt preparation")
		}
		if store.Next() != 4 {
			t.Fatal("the dead process did not leave its three durable events")
		}
		payload := storedReport(t, replayFacts(tNow))
		expiry := tNow.Add(time.Hour)
		first, second := scanCommand(71, 1, expiry), scanCommand(72, 1, expiry)
		var writes int
		restore := swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
			writes++
			return db.Update(write)
		})
		t.Cleanup(restore)
		requireReceiptPreparation(t, store)
		restore()
		if writes != 1 {
			t.Errorf("sealing two starts used %d commits, want one", writes)
		}
		for _, cmd := range []*linkv1.Command{first, second} {
			requireRevisionTwoReceipt(t, receiptOnDisk(t, store, cmd.GetId()), "interrupted", false, expiry, 3, "")
		}
		events, err := store.Read(1, 3)
		if err != nil || len(events) != 3 {
			t.Fatal("receipt sealing did not leave the old events readable")
		}
		for i, event := range events {
			if event.Seq != uint64(i+1) || !bytes.Equal(event.Payload, payload) {
				t.Fatal("receipt sealing rewrote a durable event")
			}
		}

		var scans atomic.Int32
		noRepeat := func(context.Context, bool) (bool, error) {
			scans.Add(1)
			return false, nil
		}
		f := newCommandFixture(t, store, noRepeat, nil, nil)
		for _, cmd := range []*linkv1.Command{first, second} {
			retry := attemptCommand(cmd, 2)
			requireContinues(t, f, retry)
			requireReceiptReply(t, f, retry, commandInterrupted, 3)
		}
		f.noReply(t)

		sealed := receiptOnDisk(t, store, first.GetId())
		appendLinkFixture(t, store, payload)
		store = reopen(t, folder, store, StoreOptions{})
		restore = swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
			writes++
			return db.Update(write)
		})
		writes = 0
		t.Cleanup(restore)
		requireReceiptPreparation(t, store)
		restore()
		if writes != 0 || receiptOnDisk(t, store, first.GetId()) != sealed {
			t.Error("preparing an already sealed receipt rewrote it after another append")
		}
		f = newCommandFixture(t, store, noRepeat, nil, nil)
		for _, cmd := range []*linkv1.Command{first, second} {
			retry := attemptCommand(cmd, 3)
			requireContinues(t, f, retry)
			requireReceiptReply(t, f, retry, commandInterrupted, 3)
		}
		f.noReply(t)
		if scans.Load() != 0 || store.Next() != 5 {
			t.Error("an interrupted command repeated work or consumed another event number")
		}
	})

	t.Run("torn seal rolls back both starts", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		payload := storedReport(t, replayFacts(tNow))
		appendLinkFixture(t, store, payload)
		if err := store.Close(); err != nil {
			t.Fatal("could not close the seed store")
		}
		path := folder.Paths().Store
		started := revisionTwoReceiptJSON(tNow.Add(time.Hour), "started", `,"last_event_number":null`)
		fixtures := map[string]string{uid(1): started, uid(2): started}
		putReceipts(t, path, fixtures)
		store = mustOpen(t, folder, StoreOptions{})
		var changed int
		torn := errors.New("fixture aborted the seal transaction")
		restore := swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
			return db.Update(func(tx *bolt.Tx) error {
				if err := write(tx); err != nil {
					return err
				}
				for id, original := range fixtures {
					if string(tx.Bucket(receiptsBucket).Get([]byte(id))) != original {
						changed++
					}
				}
				return torn
			})
		})
		err := store.prepareRun()
		restore()
		if err == nil || changed != 2 {
			t.Errorf("aborting the seal after %d of two pending receipt writes did not report the transaction failure", changed)
		}
		if err := store.Close(); err != nil {
			t.Fatal("could not close the store after the aborted seal")
		}
		if !maps.Equal(readReceipts(t, path), fixtures) {
			t.Fatal("the aborted seal left a partly written receipt")
		}
		store = mustOpen(t, folder, StoreOptions{})
		requireReceiptPreparation(t, store)
		for id := range fixtures {
			requireRevisionTwoReceipt(t, receiptOnDisk(t, store, id), "interrupted", false, tNow.Add(time.Hour), 1, "")
		}
	})

	t.Run("empty history accepts and seals present zero", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		if err := store.Close(); err != nil {
			t.Fatal("could not close the empty seed store")
		}
		expires := tNow.Add(time.Hour)
		finished, started := scanCommand(1, 1, expires), scanCommand(2, 1, expires)
		putReceipts(t, folder.Paths().Store, map[string]string{
			finished.GetId(): revisionTwoReceiptJSON(expires, "finished", `,"last_event_number":0,"result":"AQ=="`),
			started.GetId():  revisionTwoReceiptJSON(expires, "started", `,"last_event_number":null`),
		})
		store = mustOpen(t, folder, StoreOptions{})
		requireReceiptPreparation(t, store)
		requireRevisionTwoReceipt(t, receiptOnDisk(t, store, started.GetId()), "interrupted", false, expires, 0, "")
		f := newCommandFixture(t, store, nil, nil, nil)
		for _, tc := range []struct {
			cmd     *linkv1.Command
			outcome linkv1.CommandOutcome
		}{{finished, commandDone}, {started, commandInterrupted}} {
			retry := attemptCommand(tc.cmd, 2)
			requireContinues(t, f, retry)
			requireReceiptReply(t, f, retry, tc.outcome, 0)
		}
		if store.Next() != 1 || store.Len() != 0 {
			t.Error("handling a receipt in an empty history appended an event")
		}
	})

	t.Run("signed run seals before startup appends", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			payload := storedReport(t, replayFacts(tNow))
			f.seed(t, payload, payload)
			expiry := tNow.Add(time.Hour)
			started := scanCommand(1, 2, expiry)
			oldOrdinary := scanCommand(2, 1, tNow.Add(-25*time.Hour))
			putReceipts(t, f.file(), map[string]string{
				started.GetId():     revisionTwoReceiptJSON(expiry, "started", `,"last_event_number":null`),
				oldOrdinary.GetId(): receiptJSON(oldOrdinary.GetExpiresAt().AsTime(), "finished", []byte{byte(commandDone)}),
			})
			oldWallClock := wallClock
			wallClock = func() time.Time { return tNow.Add(-48 * time.Hour) }
			defer func() { wallClock = oldWallClock }()

			cancel, done := f.start(t)
			defer cancel()
			leg := f.next(t, 0)
			welcome := runWelcome(approvedApproval, 1, 0)
			welcome.ServerTime = timestamppb.New(tNow)
			f.welcome(t, leg, welcome)
			for n := uint64(1); n <= 4; n++ {
				if event := eventOnLeg(t, leg); event.GetNumber() != n {
					t.Fatal("the first signed stream did not hand off its retained and new reports in order")
				}
			}
			leg.send(t, commandFrame(started))
			requireResultWatermark(t, resultOnReceiptLeg(t, leg), started, commandInterrupted, 2)
			trigger := scanCommand(3, 1, tNow.Add(-time.Minute))
			leg.send(t, commandFrame(trigger))
			requireResultWatermark(t, resultOnReceiptLeg(t, leg), trigger, commandExpired, 4)
			cancel()
			f.stop(t, done, nil)

			receipts := readReceipts(t, f.file())
			requireRevisionTwoReceipt(t, receipts[started.GetId()], "interrupted", false, expiry, 2, "")
			if _, ok := receipts[oldOrdinary.GetId()]; ok {
				t.Error("a receipt write on a signed stream did not prune by verified Welcome time")
			}
		})
	})

	expiry := tNow.Add(time.Hour)
	finish := func(fields string) string { return revisionTwoReceiptJSON(expiry, "finished", fields) }
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"missing watermark", finish(`,"result":"AQ=="`)},
		{"null finished watermark", finish(`,"last_event_number":null,"result":"AQ=="`)},
		{"null interrupted watermark", revisionTwoReceiptJSON(expiry, "interrupted", `,"last_event_number":null`)},
		{"missing started watermark", revisionTwoReceiptJSON(expiry, "started", "")},
		{"integer on an unsealed start", revisionTwoReceiptJSON(expiry, "started", `,"last_event_number":0`)},
		{"above the counter", finish(`,"last_event_number":2,"result":"AQ=="`)},
		{"negative watermark", finish(`,"last_event_number":-1,"result":"AQ=="`)},
		{"fractional watermark", finish(`,"last_event_number":0.5,"result":"AQ=="`)},
		{"string watermark", finish(`,"last_event_number":"0","result":"AQ=="`)},
		{"unknown version", fmt.Sprintf(`{"version":3,"expires_at":%q,"state":"started"}`, expiry.Format(time.RFC3339))},
		{"missing standing field", fmt.Sprintf(`{"version":2,"expires_at":%q,"state":"finished","last_event_number":0,"result":"AQ=="}`, expiry.Format(time.RFC3339))},
		{"result on an interrupted receipt", revisionTwoReceiptJSON(expiry, "interrupted", `,"last_event_number":0,"result":"AQ=="`)},
		{"unknown outcome", finish(`,"last_event_number":0,"result":"/w=="`)},
		{"no finished result", finish(`,"last_event_number":0`)},
		{"malformed legacy receipt", fmt.Sprintf(`{"version":1,"expires_at":%q,"state":"finished","result":"!"}`, expiry.Format(time.RFC3339))},
		{"revision one with revision two fields", fmt.Sprintf(`{"version":1,"expires_at":%q,"state":"finished","result":"AQ==","last_event_number":0,"standing":false}`, expiry.Format(time.RFC3339))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			appendLinkFixture(t, store, storedReport(t, replayFacts(tNow)))
			if err := store.Close(); err != nil {
				t.Fatal("could not close the seed store before corrupting a receipt")
			}
			path := folder.Paths().Store
			cmd := scanCommand(2, 2, expiry)
			putReceipts(t, path, map[string]string{
				scanCommand(1, 1, expiry).GetId(): revisionTwoReceiptJSON(expiry, "started", `,"last_event_number":null`),
				cmd.GetId():                       tc.raw,
			})
			before := readBytes(t, path)
			store, err := OpenStore(folder, StoreOptions{})
			if err == nil {
				err = store.prepareRun()
				if err == nil {
					t.Error("receipt preparation accepted corrupt metadata instead of ErrStoreDamaged")
					f := newCommandFixture(t, store, nil, nil, nil)
					if restart, err := f.handler.handle(t.Context(), cmd); restart || !errors.Is(err, ErrStoreDamaged) {
						t.Error("the broken receipt reached command handling without ErrStoreDamaged")
					}
					select {
					case <-f.replies:
						t.Error("the corrupt receipt produced a result")
					default:
					}
				}
				if closeErr := store.Close(); closeErr != nil {
					t.Fatal("could not close the tested store")
				}
			}
			if !errors.Is(err, ErrStoreDamaged) {
				t.Error("the corrupt receipt did not yield ErrStoreDamaged")
			}
			if !bytes.Equal(readBytes(t, path), before) {
				t.Error("the corrupt receipt or adjacent valid history was rewritten")
			}
		})
	}
}

func resultOnReceiptLeg(t *testing.T, leg *wireLeg) *linkv1.CommandResult {
	t.Helper()
	for range 8 {
		frame := leg.next(t)
		if result := frame.GetCommandResult(); result != nil {
			return result
		}
		if frame.GetHeartbeat() == nil {
			t.Fatal("the command replay unexpectedly handed off more work")
		}
		leg.send(t, ackHeartbeat())
	}
	t.Fatal("the command replay did not send a result")
	return nil
}

func TestRevisionTwoLegacyStore(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		source := filepath.Join("testdata", "client-v1.db")
		original := readBytes(t, source)
		path := f.file()
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal("could not copy the revision 1 producer's database")
		}

		expiry := tNow.Add(time.Hour)
		finished, started := scanCommand(101, 1, expiry), scanCommand(102, 1, expiry)
		legacy := map[string]string{
			finished.GetId(): `{"version":1,"expires_at":"2099-01-01T01:00:00Z","state":"finished","result":"AQ=="}`,
			started.GetId():  `{"version":1,"expires_at":"2099-01-01T01:00:00Z","state":"started"}`,
		}
		if !maps.Equal(readReceipts(t, path), legacy) {
			t.Fatal("the revision 1 producer's two receipt records did not match their fixture")
		}
		store := mustOpen(t, f.opts.Folder, StoreOptions{})
		if store.Next() != 4 || !bytes.Equal(readBytes(t, path), original) {
			t.Fatal("opening the revision 1 store changed its bytes or counter")
		}
		facts, factsBytes := readReport(t, store, 1)
		if facts.GetFacts().GetClientVersion() != "revision-one-fixture" ||
			fmt.Sprintf("%x", sha256.Sum256(factsBytes)) != "bc24402f6332eecc8619bbab9367d3bdaa8c7a86a19b8d380671dd89838a3838" {
			t.Fatal("retained Facts did not match the revision 1 producer")
		}
		history, err := store.Read(1, 3)
		if err != nil || len(history) != 3 || store.Close() != nil {
			t.Fatal("could not capture the original event bytes")
		}

		var scans atomic.Int32
		scan := runReportProbes.scan
		runReportProbes.scan = func(ctx context.Context, opts harness.ScanOptions) []harness.Found {
			scans.Add(1)
			return scan(ctx, opts)
		}
		for run := uint32(0); run < 2; run++ {
			cancel, done := f.start(t)
			defer cancel()
			leg := f.next(t, 0)
			first := uint64(4 + 2*run)
			f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
			for n := uint64(1); n <= first+1; n++ {
				got := eventOnLeg(t, leg)
				if got.GetNumber() != n {
					t.Fatal("an upgraded run did not hand off the retained prefix in order")
				}
				if n == 1 {
					got.Number = 0
					if !proto.Equal(got, facts) {
						t.Error("the upgraded run changed the retained Facts")
					}
				}
			}
			for _, tc := range []struct {
				cmd     *linkv1.Command
				outcome linkv1.CommandOutcome
			}{{finished, commandDone}, {started, commandInterrupted}} {
				retry := attemptCommand(tc.cmd, 2+run)
				leg.send(t, commandFrame(retry))
				requireResultWatermark(t, resultOnReceiptLeg(t, leg), retry, tc.outcome, 0)
			}
			fresh := stateCommand(103+int(run), time.Now().Add(-time.Minute), 2, approvedApproval, "updated fixture")
			leg.send(t, commandFrame(fresh))
			requireResultWatermark(t, resultOnReceiptLeg(t, leg), fresh, commandDone, first+1)
			cancel()
			f.stop(t, done, nil)

			store := mustOpen(t, f.opts.Folder, StoreOptions{})
			for id, original := range legacy {
				if receiptOnDisk(t, store, id) != original {
					t.Error("the upgraded run rewrote a revision 1 receipt")
				}
			}
			requireRevisionTwoReceipt(t, receiptOnDisk(t, store, fresh.GetId()), "finished", true, fresh.GetExpiresAt().AsTime(), first+1, `"AQ=="`)
			kept, err := store.Read(1, 3)
			if err != nil || len(kept) != len(history) {
				t.Fatal("the revision 1 event prefix was lost")
			}
			for i, event := range kept {
				if event.Seq != history[i].Seq || !bytes.Equal(event.Payload, history[i].Payload) {
					t.Error("the upgraded run rewrote a revision 1 event")
				}
			}
			if store.Next() != first+2 || scans.Load() != int32(run+1) {
				t.Error("the upgraded run repeated a revision 1 scan")
			}
			if err := store.Close(); err != nil {
				t.Fatal("could not close the copied revision 1 store")
			}
		}
		if !bytes.Equal(readBytes(t, source), original) {
			t.Error("the original revision 1 fixture was changed")
		}
	})
}
