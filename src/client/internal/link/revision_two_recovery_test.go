package link

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

func TestRevisionTwoTerminalHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *runPeerFixture)
		mark    uint64
		welcome bool
		missing bool
	}{
		{
			name: "missing database", missing: true,
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t)
				if os.Remove(f.file()) != nil {
					t.Fatal("could not remove the fixture database")
				}
			},
		},
		{
			name: "corrupt receipt",
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t, fixtureFactsPayload(t, "saved"))
				putReceipts(t, f.file(), map[string]string{scanCommand(1, 1, tNow).GetId(): "damaged receipt"})
			},
		},
		{
			name: "suffix hole",
			prepare: func(t *testing.T, f *runPeerFixture) {
				payload := fixtureFactsPayload(t, "saved")
				f.seed(t, payload, payload, payload)
				editStore(t, f.file(), func(tx *bolt.Tx) error {
					return tx.Bucket(eventsBucket).Delete(be(2))
				})
			},
		},
		{
			name: "empty suffix below floor", welcome: true, mark: 1,
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t)
				setNext(t, f.file(), 3)
			},
		},
		{
			name: "welcome at next", welcome: true, mark: 2,
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t, fixtureFactsPayload(t, "saved"))
			},
		},
		{
			name: "welcome below retained floor", welcome: true, mark: 1,
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t)
				setNext(t, f.file(), 3)
				store := mustOpen(t, f.opts.Folder, StoreOptions{})
				appendLinkFixture(t, store, fixtureFactsPayload(t, "saved"))
				if store.Close() != nil {
					t.Fatal("could not close the fixture database")
				}
			},
		},
		{
			name: "corrupt event beyond replay window", welcome: true, mark: 1,
			prepare: func(t *testing.T, f *runPeerFixture) {
				payload := fixtureFactsPayload(t, "saved")
				reports := make([][]byte, 1+2*revisionTwoEventWindow(t))
				for i := range reports {
					reports[i] = payload
				}
				f.seed(t, append(reports, []byte{1, 0xff})...)
			},
		},
		{
			name: "oversized required event", welcome: true, mark: 1,
			prepare: func(t *testing.T, f *runPeerFixture) {
				payload := fixtureFactsPayload(t, "saved")
				f.seed(t, payload, payload)
				oversized := protowire.AppendTag(bytes.Clone(payload), 99, protowire.BytesType)
				oversized = protowire.AppendBytes(oversized, make([]byte, recoveryEventByteLimit(t)))
				editStore(t, f.file(), func(tx *bolt.Tx) error {
					return tx.Bucket(eventsBucket).Put(be(2), oversized)
				})
			},
		},
		{
			name: "next above limit",
			prepare: func(t *testing.T, f *runPeerFixture) {
				f.seed(t)
				started := scanCommand(2, 1, time.Now().Add(time.Minute))
				store := mustOpen(t, f.opts.Folder, StoreOptions{})
				if decision, _, err := store.Begin(receiptCommand(started), time.Now()); err != nil || decision != DecisionRun {
					t.Fatal("could not write the fixture started receipt")
				}
				if store.Close() != nil {
					t.Fatal("could not close the fixture database")
				}
				setNext(t, f.file(), uint64(math.MaxInt64)+2)
				before := readBytes(t, f.file())
				store, err := OpenStore(f.opts.Folder, StoreOptions{})
				if store != nil {
					_ = store.Close()
				}
				if store != nil || !errors.Is(err, ErrStoreDamaged) {
					t.Error("store inspection accepted next above the revision 2 limit")
				}
				if !bytes.Equal(before, readBytes(t, f.file())) {
					t.Error("store inspection changed the database before rejecting its counter")
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newRunPeerFixture(t, protocols[0])
				dials := countRecoveryDials(t)
				frames := observeRecoveryFrames(t)
				tc.prepare(t, f)
				var before []byte
				if !tc.missing {
					before = readBytes(t, f.file())
				}

				cancel, done := f.start(t)
				defer cancel()
				wantDials := 0
				if tc.welcome {
					wantDials = 1
					f.welcome(t, f.next(t, 0), runWelcome(waitingApproval, 1, tc.mark))
				}
				// Let a second attempt become observable before checking the outcome.
				time.Sleep(16 * time.Second)
				requireRecoveryStop(t, f, done, false)
				if got := dials.Load(); got != int32(wantDials) {
					t.Errorf("history recovery made %d dial attempts, want %d", got, wantDials)
				}
				select {
				case <-frames:
					t.Error("history recovery handed off an event or result")
				default:
				}
				if tc.missing {
					if _, err := os.Lstat(f.file()); !errors.Is(err, os.ErrNotExist) {
						t.Error("history recovery published a replacement database")
					}
				} else if !bytes.Equal(before, readBytes(t, f.file())) {
					t.Error("history recovery changed the retained database bytes")
				}
			})
		})
	}

	t.Run("hole after store inspection preserves confirmed rows", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		for range 4 {
			appendLinkFixture(t, store, storedReport(t, replayFacts(tNow)))
		}
		// Damage the disk view after startup inspection has cached the range.
		if store.db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket(eventsBucket).Delete(be(3))
		}) != nil {
			t.Fatal("could not create a hole in the open fixture database")
		}
		before := readBytes(t, folder.Paths().Store)
		cursor, err := newReplay(store, 1)
		if cursor != nil || err == nil {
			t.Error("initial replay validation allowed a hole in the required range")
		}
		if !bytes.Equal(before, readBytes(t, folder.Paths().Store)) {
			t.Error("initial replay validation deleted a confirmed row before finding the hole")
		}
	})

	t.Run("confirmed corrupt prefix is skipped", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			f.seed(t, []byte{1, 0xff}, fixtureFactsPayload(t, "required"))
			cancel, done := f.start(t)
			defer cancel()
			leg := f.next(t, 0)
			f.welcome(t, leg, runWelcome(waitingApproval, 1, 1))
			event := eventOnLeg(t, leg)
			if event.GetNumber() != 2 || event.GetFacts().GetClientVersion() != "required" {
				t.Error("the verified mark did not skip the confirmed corrupt payload")
			}
			cancel()
			f.stop(t, done, nil)
			store := mustOpen(t, f.opts.Folder, StoreOptions{})
			rows, err := store.Read(1, 2)
			if err != nil || len(rows) != 1 || rows[0].Seq != 2 {
				t.Error("the verified mark did not remove only the confirmed prefix")
			}
		})
	})

	t.Run("last legal event drains", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			f.seed(t)
			setNext(t, f.file(), math.MaxInt64)
			store := mustOpen(t, f.opts.Folder, StoreOptions{})
			payload := fixtureFactsPayload(t, "last legal event")
			if seq := appendLinkFixture(t, store, payload); seq != math.MaxInt64 {
				t.Fatal("the append did not use the final legal number")
			}
			if store.Close() != nil {
				t.Fatal("could not close the fixture database")
			}
			cancel, done := f.start(t)
			defer cancel()
			leg := f.next(t, 0)
			hello := leg.hello(t)
			if hello.GetFirstEventNumber() != 1<<63 {
				t.Error("the final next counter was not announced exactly")
			}
			f.peer.prove(t, leg, hello)
			leg.send(t, welcomeFrame(runWelcome(waitingApproval, 1, math.MaxInt64-1)))
			if event := eventOnLeg(t, leg); event.GetNumber() != math.MaxInt64 ||
				event.GetFacts().GetClientVersion() != "last legal event" {
				t.Error("the final legal event was not replayed intact")
			}
			leg.send(t, ackEvent(math.MaxInt64))
			synctest.Wait()
			cancel()
			f.stop(t, done, nil)
			store = mustOpen(t, f.opts.Folder, StoreOptions{})
			if store.Len() != 0 || store.Next() != 1<<63 {
				t.Error("the acknowledgement did not drain the last event without wrapping next")
			}
			before := readBytes(t, f.file())
			if seq, err := store.Append(payload); seq != 0 || !errors.Is(err, ErrSequenceExhausted) {
				t.Error("an append after the last legal event did not report sequence exhaustion")
			}
			if store.Next() != 1<<63 || !bytes.Equal(before, readBytes(t, f.file())) {
				t.Error("sequence exhaustion changed the database or wrapped the counter")
			}
		})
	})

	for _, refusal := range []bool{false, true} {
		name := "incompatible welcome"
		if refusal {
			name = "verified revision refusal"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newRunPeerFixture(t, protocols[0])
				f.seed(t, fixtureFactsPayload(t, "saved"))
				before := readBytes(t, f.file())
				dials := countRecoveryDials(t)
				frames := observeRecoveryFrames(t)
				cancel, done := f.start(t)
				defer cancel()
				leg := f.next(t, 0)
				f.peer.prove(t, leg, leg.hello(t))
				if refusal {
					leg.refuse <- peerError(connect.CodeFailedPrecondition, refusalKey, "link_revision_mismatch")
				} else {
					welcome := runWelcome(waitingApproval, 1, 1)
					welcome.LinkRevision = 1
					leg.send(t, welcomeFrame(welcome))
				}
				time.Sleep(16 * time.Second)
				requireRecoveryStop(t, f, done, true)
				if got := dials.Load(); got != 1 {
					t.Errorf("incompatible revision made %d dial attempts, want one", got)
				}
				requireEmpty(t, frames, "an incompatible revision handed off an event or result")
				if !bytes.Equal(before, readBytes(t, f.file())) {
					t.Error("an incompatible revision changed the saved bytes")
				}
			})
		})
	}

	t.Run("nonterminal handshake faults retry", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			f.seed(t)
			cancel, done := f.start(t)
			defer cancel()

			first := f.next(t, 0)
			hello := first.hello(t)
			peerScene := scene{keys: f.peer.keys}
			challenge := peerScene.challenge(hello, fixedServerNonce)
			challenge.GetChallenge().ServerSignature = nil
			first.send(t, challenge)

			second := f.next(t, time.Second)
			second.hello(t)
			second.refuse <- peerError(connect.CodeFailedPrecondition, refusalKey, "link_revision_mismatch")

			third := f.next(t, 2*time.Second)
			f.peer.prove(t, third, third.hello(t))
			malformed := runWelcome(waitingApproval, 1, 0)
			malformed.LinkRevision, malformed.State = 1, nil
			third.send(t, welcomeFrame(malformed))

			fourth := f.next(t, 4*time.Second)
			f.peer.prove(t, fourth, fourth.hello(t))
			fourth.refuse <- peerError(connect.CodeInvalidArgument, "", "")

			fifth := f.next(t, 8*time.Second)
			f.welcome(t, fifth, runWelcome(waitingApproval, 1, 0))
			requireRevisionTwoBareHeartbeat(t, fifth, time.Second)
			logs := f.diagnostics(t)
			var failures []string
			for _, line := range logs {
				if recoveryRemedy(line.Message, false) || recoveryRemedy(line.Message, true) {
					t.Error("a retryable handshake fault emitted a recovery instruction")
				}
				if line.Msg == "link_disconnected" {
					failures = append(failures, line.Code)
				}
			}
			if strings.Join(failures, ",") != "invalid_frame,failed_precondition,invalid_frame,invalid_argument" {
				t.Error("the handshake faults did not retain their separate retry diagnoses")
			}
			cancel()
			f.stop(t, done, nil)
		})
	})

	t.Run("late scan cannot settle after recovery", testRecoveryWriteFence)
}

func testRecoveryWriteFence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newRunPeerFixture(t, protocols[0])
		f.seed(t)
		dials := countRecoveryDials(t)
		frames := observeRecoveryFrames(t)
		started, release := make(chan struct{}), make(chan struct{})
		var scans int
		runReportProbes.scan = func(context.Context, harness.ScanOptions) []harness.Found {
			scans++
			if scans == 2 {
				close(started)
				<-release
			}
			return nil
		}

		cancel, done := f.start(t)
		defer cancel()
		released := false
		defer func() {
			if !released {
				close(release)
			}
		}()
		first := f.next(t, 0)
		f.welcome(t, first, runWelcome(approvedApproval, 1, 0))
		for number := uint64(1); number <= 2; number++ {
			if event := eventOnLeg(t, first); event.GetNumber() != number {
				t.Fatal("approved startup did not publish the expected two reports")
			}
			first.send(t, ackEvent(number))
		}
		scan := scanCommand(3, 1, time.Now().Add(time.Minute))
		first.send(t, commandFrame(scan))
		receiveNow(t, started, "the requested scan never reached its held probe")
		// Discard the observation from the two approved startup reports.
		receiveNow(t, frames, "startup did not hand off its reports")

		first.refuse <- peerError(connect.CodeUnavailable, "", "")
		second := f.next(t, 0)
		hello := second.hello(t)
		before := readBytes(t, f.file())
		f.peer.prove(t, second, hello)
		// The reconnect is authenticated but claims an impossible event mark.
		second.send(t, welcomeFrame(runWelcome(waitingApproval, 2, math.MaxInt64)))
		receiveNow(t, second.closed, "recovery did not close the invalid stream")
		close(release)
		released = true
		requireRecoveryStop(t, f, done, false)
		if got := dials.Load(); got != 2 {
			t.Errorf("the fenced scan made %d dial attempts, want two", got)
		}
		select {
		case <-frames:
			t.Error("the fenced scan handed off a late event or result")
		default:
		}
		if !bytes.Equal(before, readBytes(t, f.file())) {
			t.Error("the late scan changed an event row, next, or receipt after recovery")
		}
		saved := readReceipts(t, f.file())[scan.GetId()]
		var receipt struct {
			State string `json:"state"`
		}
		if json.Unmarshal([]byte(saved), &receipt) != nil || receipt.State != "started" {
			t.Error("the scan's started receipt did not survive recovery")
		}
	})
}

func countRecoveryDials(t *testing.T) *atomic.Int32 {
	t.Helper()
	open := newRunLinkClient
	count := new(atomic.Int32)
	newRunLinkClient = func(server enroll.Server) (linkv1connect.LinkServiceClient, func(), error) {
		count.Add(1)
		return open(server)
	}
	t.Cleanup(func() { newRunLinkClient = open })
	return count
}

func observeRecoveryFrames(t *testing.T) <-chan struct{} {
	t.Helper()
	frames := make(chan struct{}, 1)
	useSessionIO(t, func(_ context.Context, _ *session, wire runSessionIO) runSessionIO {
		send := wire.send
		wire.send = func(frame *linkv1.OpenRequest) error {
			if frame.GetEvent() != nil || frame.GetCommandResult() != nil {
				select {
				case frames <- struct{}{}:
				default:
				}
			}
			return send(frame)
		}
		return wire
	})
	return frames
}

func requireRecoveryStop(t *testing.T, f *runPeerFixture, done <-chan runResult, revision bool) {
	t.Helper()
	synctest.Wait()
	select {
	case result := <-done:
		if result.outcome != RunRecoveryStop || result.err != nil {
			t.Error("the client did not return RunRecoveryStop without a retry error")
		}
		if result.err != nil {
			f.checkOutput(t, result.err.Error())
		}
	default:
		t.Error("the client did not stop after the recovery fault")
	}
	count := 0
	for _, line := range f.diagnostics(t) {
		if recoveryRemedy(line.Message, revision) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the client emitted %d actionable recovery instructions, want one", count)
	}
}

func recoveryRemedy(message string, revision bool) bool {
	message = strings.ToLower(message)
	if revision {
		return strings.Contains(message, "compatible") && strings.Contains(message, "client") &&
			(strings.Contains(message, "install") || strings.Contains(message, "upgrade")) &&
			(strings.Contains(message, "saved") || strings.Contains(message, "existing")) && strings.Contains(message, "state")
	}
	return strings.Contains(message, "client.db") &&
		(strings.Contains(message, "restor") || strings.Contains(message, "recover")) &&
		(strings.Contains(message, "histor") || strings.Contains(message, "original") || strings.Contains(message, "backup")) &&
		strings.Contains(message, "remov") && strings.Contains(message, "enroll") &&
		(strings.Contains(message, "fresh") || strings.Contains(message, "new state") || strings.Contains(message, "new folder"))
}

func recoveryEventByteLimit(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "testdata", "link-v2-limits.json"))
	var limits struct {
		CommandEventBytes int `json:"command_event_bytes"`
	}
	if err != nil || json.Unmarshal(data, &limits) != nil || limits.CommandEventBytes <= 0 {
		t.Fatal("could not read the revision 2 event byte limit")
	}
	return limits.CommandEventBytes
}
