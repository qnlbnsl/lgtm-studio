package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type receiptResult struct {
	reply []byte
	err   error
}

func receiptRequest() Request {
	return Request{
		Key: "request-a", ReceiptSafe: true, Caller: "fixture-principal",
		Procedure: "SaveRecord", FingerprintVersion: 1,
		Fingerprints: map[int16][]byte{1: []byte("value")},
	}
}

func receiptLockWord(t *testing.T, obs *pgx.Conn, req Request) int32 {
	t.Helper()
	principal := sha256.Sum256([]byte(req.Caller))
	return storetest.Scalar[int32](t, obs, "SELECT hashtext(encode($1::bytea, 'hex') || ':' || $2)", principal[:], req.Key)
}

func receiptKey(word int32) storetest.AdvisoryKey {
	return storetest.AdvisoryKey{ClassID: uint32(receiptLockNamespace), ObjectID: uint32(word), SubID: 2}
}

func receiptWrite(key string, reply []byte) func(context.Context, *Tx) ([]byte, error) {
	return func(ctx context.Context, tx *Tx) ([]byte, error) {
		return reply, feedSave(ctx, tx, key)
	}
}

func receiptAssertReplay(t *testing.T, s *Store, req Request, want []byte, wantErr error) {
	t.Helper()
	called := false
	reply, err := s.WriteOnce(t.Context(), req, func(context.Context, *Tx) ([]byte, error) {
		called = true
		return []byte("unexpected callback reply"), nil
	})
	if called || !bytes.Equal(reply, want) || !errors.Is(err, wantErr) || (wantErr != nil && reply != nil) {
		t.Fatalf("replay invoked callback = %v, reply length = %d, error = %v, want length %d and error %v",
			called, len(reply), err, len(want), wantErr)
	}
}

func receiptStart(t *testing.T, s *Store, ctx context.Context, req Request, write func(context.Context, *Tx) ([]byte, error)) *storetest.Task[receiptResult] {
	t.Helper()
	return storetest.Start(t, func() receiptResult {
		reply, err := s.WriteOnce(ctx, req, write)
		return receiptResult{reply, err}
	})
}

func TestWriteOnceConcurrentRetry(t *testing.T) {
	s, cfg, obs := feedFixture(t, nil)
	second := storeTestOpen(t, cfg)
	goOn, release := storetest.Signal(t)
	defer release()
	req := receiptRequest()
	want := []byte("original reply")
	var callbacks atomic.Int32
	aPID := make(chan int32, 1)
	a := receiptStart(t, s, t.Context(), req, func(ctx context.Context, tx *Tx) ([]byte, error) {
		callbacks.Add(1)
		var pid int32
		if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
			return nil, err
		}
		if err := feedSave(ctx, tx, "a"); err != nil {
			return nil, err
		}
		aPID <- pid
		select {
		case <-goOn:
			return want, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	blocker := storetest.ReceiveFrom(t, a, "the first WriteOnce inserting state", aPID)

	b := receiptStart(t, second, t.Context(), req, func(context.Context, *Tx) ([]byte, error) {
		callbacks.Add(1)
		return []byte("duplicate reply"), nil
	})
	key := receiptKey(receiptLockWord(t, obs, req))
	var waiter int32
	storetest.Poll(t, "the retry waiting on the principal's key", func() bool {
		select {
		case <-b.Done():
			t.Fatalf("the retry finished before the original committed: callbacks = %d, error = %v", callbacks.Load(), b.Result().err)
		default:
		}
		for _, pid := range poolBlockedPIDs(t, obs, key) {
			blocked, err := storetest.IsBlocked(t.Context(), obs, pid, blocker, key)
			if err != nil {
				t.Fatal(err)
			}
			if blocked {
				waiter = pid
				return true
			}
		}
		return false
	})
	storetest.WaitForBlock(t, obs, waiter, blocker, key)
	release()
	for i, call := range []*storetest.Task[receiptResult]{a, b} {
		result := call.Wait(t)
		if result.err != nil || !bytes.Equal(result.reply, want) {
			t.Fatalf("call %d returned the wrong reply: %v", i, result.err)
		}
	}
	if got := callbacks.Load(); got != 1 {
		t.Fatalf("the callback ran %d times, want once", got)
	}
	s.Close()
	second.Close()
	receiptAssertReplay(t, storeTestOpen(t, cfg), req, want, nil)
	feedRequireCount(t, obs, "records", 1)
	feedRequireCount(t, obs, "receipts", 1)
	feedRequireOrder(t, obs, "a")
}

func TestWriteOnceMismatch(t *testing.T) {
	t.Run("saved request", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		req := receiptRequest()
		req.Procedure = "SaveRecordX"
		want := []byte("original reply")
		if reply, err := s.WriteOnce(t.Context(), req, receiptWrite("a", want)); err != nil || !bytes.Equal(reply, want) {
			t.Fatalf("seeding a receipt failed: %v", err)
		}
		differentBody := req
		differentBody.Fingerprints = map[int16][]byte{1: []byte("different")}
		receiptAssertReplay(t, s, differentBody, nil, ErrRequestKeyConflict)
		differentProcedure := req
		differentProcedure.Procedure = "RemoveRecord"
		receiptAssertReplay(t, s, differentProcedure, nil, ErrRequestKeyConflict)
		unframedCollision := req
		unframedCollision.Procedure = "SaveRecord"
		unframedCollision.Fingerprints = map[int16][]byte{1: []byte("Xvalue")}
		receiptAssertReplay(t, s, unframedCollision, nil, ErrRequestKeyConflict)

		for _, independent := range []struct {
			caller, key, record string
		}{
			{"other-principal", req.Key, "b"},
			{req.Caller, strings.ToUpper(req.Key), "c"},
			{req.Caller, strings.Repeat("é", 128), "d"},
		} {
			request := req
			request.Caller, request.Key = independent.caller, independent.key
			if reply, err := s.WriteOnce(t.Context(), request, receiptWrite(independent.record, want)); err != nil || !bytes.Equal(reply, want) {
				t.Fatalf("an independent key failed: %v", err)
			}
		}
		feedRequireCount(t, obs, "receipts", 4)
		feedRequireCount(t, obs, "records", 4)
		feedRequireOrder(t, obs, "a", "b", "c", "d")
	})

	t.Run("invalid request", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		req := receiptRequest()
		word := receiptLockWord(t, obs, req)
		storetest.Exec(t, obs, "SELECT pg_advisory_lock($1::int4, $2::int4)", receiptLockNamespace, word)
		defer storetest.Exec(t, obs, "SELECT pg_advisory_unlock($1::int4, $2::int4)", receiptLockNamespace, word)
		for _, tc := range []struct {
			name   string
			change func(*Request)
			want   error
		}{
			{"zero byte", func(r *Request) { r.Key = "key\x00" }, ErrInvalidRequestKey},
			{"invalid Unicode encoding", func(r *Request) { r.Key = "key\xff" }, ErrInvalidRequestKey},
			{"257-byte key", func(r *Request) { r.Key = strings.Repeat("é", 128) + "a" }, ErrInvalidRequestKey},
			{"missing caller", func(r *Request) { r.Caller = "" }, ErrInvalidRequest},
			{"missing procedure", func(r *Request) { r.Procedure = "" }, ErrInvalidRequest},
			{"zero version", func(r *Request) { r.FingerprintVersion = 0; r.Fingerprints[0] = []byte("value") }, ErrInvalidRequest},
			{"missing encoding", func(r *Request) { delete(r.Fingerprints, 1) }, ErrInvalidRequest},
			{"empty encoding", func(r *Request) { r.Fingerprints[1] = []byte{} }, ErrInvalidRequest},
			{"no safety declaration", func(r *Request) { r.ReceiptSafe = false }, ErrInvalidRequest},
		} {
			t.Run(tc.name, func(t *testing.T) {
				req := receiptRequest()
				tc.change(&req)
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				called := false
				reply, err := s.WriteOnce(ctx, req, func(context.Context, *Tx) ([]byte, error) {
					called = true
					return nil, nil
				})
				if called || reply != nil || !errors.Is(err, tc.want) {
					t.Fatalf("invalid request invoked callback = %v, reply length = %d, error = %v, want %v",
						called, len(reply), err, tc.want)
				}
			})
		}
		feedRequireCount(t, obs, "receipts", 0)
	})
}

func receiptDigest(req Request) [32]byte {
	version := req.FingerprintVersion
	framed := binary.BigEndian.AppendUint16(nil, uint16(version))
	for _, field := range [][]byte{[]byte(req.Caller), []byte(req.Procedure), req.Fingerprints[version]} {
		framed = binary.BigEndian.AppendUint64(framed, uint64(len(field)))
		framed = append(framed, field...)
	}
	return sha256.Sum256(framed)
}

func TestWriteOnceVersionedReplay(t *testing.T) {
	s, cfg, obs := feedFixture(t, nil)
	v1 := receiptRequest()
	want := []byte{0, 1, 2, 255}
	if reply, err := s.WriteOnce(t.Context(), v1, receiptWrite("a", want)); err != nil || !bytes.Equal(reply, want) {
		t.Fatalf("storing the version 1 reply failed: %v", err)
	}
	s.Close()
	s = storeTestOpen(t, cfg)

	v2 := v1
	v2.FingerprintVersion = 2
	v2.Fingerprints = map[int16][]byte{1: []byte("value"), 2: []byte("new encoding")}
	receiptAssertReplay(t, s, v2, want, nil)
	v2.Fingerprints[1] = []byte("changed")
	receiptAssertReplay(t, s, v2, nil, ErrRequestKeyConflict)
	delete(v2.Fingerprints, 1)
	receiptAssertReplay(t, s, v2, nil, ErrRequestFingerprintUnavailable)
	v2.Fingerprints[1] = []byte{}
	receiptAssertReplay(t, s, v2, nil, ErrRequestFingerprintUnavailable)

	v2.Key = "request-b"
	v2.Fingerprints[2] = v1.Fingerprints[1]
	if reply, err := s.WriteOnce(t.Context(), v2, receiptWrite("b", nil)); err != nil || len(reply) != 0 {
		t.Fatalf("storing a zero-byte reply failed: length = %d, error = %v", len(reply), err)
	}
	receiptAssertReplay(t, s, v2, nil, nil)
	for _, req := range []Request{v1, v2} {
		digest := receiptDigest(req)
		principal := sha256.Sum256([]byte(req.Caller))
		var got []byte
		var version int16
		if err := obs.QueryRow(t.Context(), `
SELECT request_hash, fingerprint_version FROM receipts WHERE principal_hash = $1 AND request_key = $2`,
			principal[:], req.Key).Scan(&got, &version); err != nil {
			t.Fatal(err)
		}
		if version != req.FingerprintVersion || !bytes.Equal(got, digest[:]) {
			t.Fatalf("the receipt did not persist the framed digest for version %d", req.FingerprintVersion)
		}
	}
	if size := storetest.Scalar[int](t, obs, "SELECT octet_length(reply) FROM receipts WHERE request_key = $1", v2.Key); size != 0 {
		t.Fatalf("the stored empty reply has %d bytes", size)
	}
	feedRequireOrder(t, obs, "a", "b")
}

func TestWriteOnceRollbackAndUnkeyed(t *testing.T) {
	t.Run("failed commit leaves the key reusable", func(t *testing.T) {
		s, _, obs := feedFixture(t, &feedTestTracer{after: feedDeferGate})
		feedInstallGate(t, obs, true)
		release := storetest.HoldAdvisoryGate(t, obs, feedTestGateID)
		defer release()
		req := receiptRequest()
		storetest.Exec(t, obs, `
CREATE FUNCTION receipt_test_before_feed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.key = 'a' AND NOT EXISTS (SELECT FROM receipts WHERE request_key = 'request-a') THEN
        RAISE EXCEPTION 'the receipt insert must precede the feed flush';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER receipt_test_before_feed BEFORE INSERT ON feed
FOR EACH ROW EXECUTE FUNCTION receipt_test_before_feed();`)
		pid := make(chan int32, 1)
		task := receiptStart(t, s, t.Context(), req, func(ctx context.Context, tx *Tx) ([]byte, error) {
			var writer int32
			if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&writer); err != nil {
				return nil, err
			}
			pid <- writer
			return receiptWrite("a", []byte("discard"))(ctx, tx)
		})
		feedWaitCommit(t, obs, storetest.ReceiveFrom(t, task, "the keyed writer backend", pid))
		if seq := feedSequenceValue(t, obs); seq != 1 {
			t.Fatalf("feed sequence before commit = %d, want 1", seq)
		}
		release()
		result := task.Wait(t)
		feedOutcome(t, result.err, "23514", OutcomeRolledBack, false)
		if result.reply != nil {
			t.Fatal("a failed commit returned a reply")
		}
		for _, table := range []string{"records", "receipts", "feed"} {
			feedRequireCount(t, obs, table, 0)
		}
		req.Fingerprints[1] = []byte("changed after failure")
		want := []byte("new reply")
		if reply, err := s.WriteOnce(t.Context(), req, receiptWrite("b", want)); err != nil || !bytes.Equal(reply, want) {
			t.Fatalf("retrying the rolled-back key failed: %v", err)
		}
		receiptAssertReplay(t, s, req, want, nil)
		feedRequireCount(t, obs, "receipts", 1)
		feedRequireOrder(t, obs, "b")
	})

	t.Run("unkeyed calls both execute", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		for _, key := range []string{"a", "b"} {
			want := []byte(key)
			reply, err := s.WriteOnce(t.Context(), Request{}, receiptWrite(key, want))
			if err != nil || !bytes.Equal(reply, want) {
				t.Fatalf("unkeyed call %s returned the wrong reply: %v", key, err)
			}
		}
		feedRequireCount(t, obs, "receipts", 0)
		feedRequireCount(t, obs, "records", 2)
		feedRequireOrder(t, obs, "a", "b")
	})

	t.Run("reply limit", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		req := receiptRequest()
		reply, err := s.WriteOnce(t.Context(), req, receiptWrite("a", make([]byte, 65537)))
		if reply != nil || !errors.Is(err, ErrReceiptReplyTooLarge) {
			t.Fatalf("an oversized reply returned %d bytes and error %v", len(reply), err)
		}
		for _, table := range []string{"records", "receipts", "feed"} {
			feedRequireCount(t, obs, table, 0)
		}
		if reply, err := s.WriteOnce(t.Context(), req, receiptWrite("b", make([]byte, 65536))); err != nil || len(reply) != 65536 {
			t.Fatalf("the maximum reply size returned %d bytes and error %v", len(reply), err)
		}
	})
}

func TestWriteOnceRetriesAfterDeadlock(t *testing.T) {
	s, _, obs := feedFixture(t, nil)
	writes, ready, release := storeTestDeadlockCallbacks(t, obs)
	defer release()
	keys := [2]string{"a", "b"}
	var requests [2]Request
	var tasks [2]*storetest.Task[receiptResult]
	for i, write := range writes {
		requests[i] = receiptRequest()
		requests[i].Key = keys[i]
		tasks[i] = receiptStart(t, s, t.Context(), requests[i], func(ctx context.Context, tx *Tx) ([]byte, error) {
			if err := write(ctx, tx); err != nil {
				return nil, err
			}
			return receiptWrite(keys[i], []byte(keys[i]))(ctx, tx)
		})
	}
	for i, locked := range ready {
		storetest.ReceiveFrom(t, tasks[i], "the first row lock of the keyed write", locked)
	}
	release()
	victim := -1
	for i, task := range tasks {
		result := task.Wait(t)
		if result.err == nil {
			if !bytes.Equal(result.reply, []byte(keys[i])) {
				t.Fatal("the surviving writer did not return its reply")
			}
			continue
		}
		feedOutcome(t, result.err, "40P01", OutcomeRolledBack, true)
		if victim != -1 || result.reply != nil {
			t.Fatal("both writers rolled back or the victim returned a reply")
		}
		victim = i
	}
	if victim == -1 {
		t.Fatal("neither keyed write was a deadlock victim")
	}
	feedRequireCount(t, obs, "receipts", 1)
	feedRequireOrder(t, obs, keys[1-victim])
	want := []byte(keys[victim])
	if reply, err := s.WriteOnce(t.Context(), requests[victim], receiptWrite(keys[victim], want)); err != nil || !bytes.Equal(reply, want) {
		t.Fatalf("retrying the deadlock victim failed: %v", err)
	}
	receiptAssertReplay(t, s, requests[victim], want, nil)
	feedRequireCount(t, obs, "receipts", 2)
	feedRequireCount(t, obs, "records", 4)
	feedRequireOrder(t, obs, keys[1-victim], keys[victim])
}

func TestWriteOnceReleasesKeyLock(t *testing.T) {
	for _, name := range []string{"cancel", "panic", "commit rejection"} {
		t.Run(name, func(t *testing.T) {
			var tracer *feedTestTracer
			if name == "commit rejection" {
				tracer = &feedTestTracer{after: feedDeferGate}
			}
			s, cfg, obs := feedFixture(t, tracer)
			req := receiptRequest()
			key := receiptKey(receiptLockWord(t, obs, req))
			cfg.ConnConfig.Tracer = nil
			second := storeTestOpen(t, cfg)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			pids := make(chan int32, 1)
			write := func(ctx context.Context, tx *Tx) ([]byte, error) {
				var pid int32
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
					return nil, err
				}
				pids <- pid
				if err := feedSave(ctx, tx, "a"); err != nil {
					return nil, err
				}
				switch name {
				case "cancel":
					cancel()
				case "panic":
					panic("receipt test panic")
				}
				return []byte("discard"), nil
			}
			var firstPID int32
			if name == "commit rejection" {
				feedInstallGate(t, obs, true)
				release := storetest.HoldAdvisoryGate(t, obs, feedTestGateID)
				defer release()
				task := receiptStart(t, s, ctx, req, write)
				firstPID = storetest.ReceiveFrom(t, task, "the committing receipt writer", pids)
				feedWaitCommit(t, obs, firstPID)
				release()
				feedOutcome(t, task.Wait(t).err, "23514", OutcomeRolledBack, false)
			} else {
				var raised any
				var err error
				func() {
					defer func() { raised = recover() }()
					_, err = s.WriteOnce(ctx, req, write)
				}()
				if name == "cancel" && (raised != nil || !errors.Is(err, context.Canceled)) {
					t.Fatalf("cancelling the callback returned %v and panic %v", err, raised)
				}
				if name == "panic" && raised != "receipt test panic" {
					t.Fatalf("panicking in the callback returned %v and panic %v", err, raised)
				}
				firstPID = storetest.Receive(t, "the interrupted receipt writer", pids)
			}
			storeTestRequireNoLock(t, obs, key)
			feedRequireCount(t, obs, "receipts", 0)

			short, end := context.WithTimeout(t.Context(), time.Second)
			defer end()
			var secondPID int32
			want := []byte("recovered reply")
			reply, err := second.WriteOnce(short, req, func(ctx context.Context, tx *Tx) ([]byte, error) {
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&secondPID); err != nil {
					return nil, err
				}
				return receiptWrite("b", want)(ctx, tx)
			})
			if err != nil || secondPID == 0 || firstPID == secondPID || !bytes.Equal(reply, want) {
				t.Fatalf("retry on another backend failed: first = %d, second = %d, error = %v", firstPID, secondPID, err)
			}
			receiptAssertReplay(t, second, req, want, nil)
			feedRequireOrder(t, obs, "b")
		})
	}
}

// The fixture discards the first server reply after the beforeCommit hook arms it.
type receiptCommitReplyConn struct {
	net.Conn
	armed   *atomic.Bool
	arrived chan<- struct{}
	drop    <-chan struct{}
}

func (conn *receiptCommitReplyConn) Read(buffer []byte) (int, error) {
	intercept := conn.armed.CompareAndSwap(true, false)
	count, err := conn.Conn.Read(buffer)
	if !intercept || count == 0 {
		return count, err
	}
	conn.arrived <- struct{}{}
	<-conn.drop
	return 0, io.ErrUnexpectedEOF
}

func TestWriteOnceAfterLostCommitReply(t *testing.T) {
	for _, keyed := range []bool{true, false} {
		name := "unkeyed"
		if keyed {
			name = "keyed"
		}
		t.Run(name, func(t *testing.T) {
			_, cfg, obs := feedFixture(t, nil)
			var armed atomic.Bool
			arrived := make(chan struct{}, 1)
			drop, release := storetest.Signal(t)
			defer release()
			clientCfg := cfg.Copy()
			clientCfg.ConnConfig.Tracer = &feedTestTracer{before: func(context.Context, pgx.Tx) error {
				armed.Store(true)
				return nil
			}}
			dial := clientCfg.ConnConfig.DialFunc
			clientCfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
				conn, err := dial(ctx, network, address)
				if err != nil {
					return nil, err
				}
				return &receiptCommitReplyConn{Conn: conn, armed: &armed, arrived: arrived, drop: drop}, nil
			}
			s := storeTestOpen(t, clientCfg)
			var req Request
			if keyed {
				req = receiptRequest()
			}
			var calls atomic.Int32
			task := receiptStart(t, s, t.Context(), req, func(ctx context.Context, tx *Tx) ([]byte, error) {
				calls.Add(1)
				return receiptWrite("a", nil)(ctx, tx)
			})
			storetest.ReceiveFrom(t, task, "the COMMIT reply reaching the fixture connection", arrived)
			feedRequireCount(t, obs, "records", 1)
			feedRequireOrder(t, obs, "a")
			if keyed {
				feedRequireCount(t, obs, "receipts", 1)
			} else {
				feedRequireCount(t, obs, "receipts", 0)
			}
			release()
			result := task.Wait(t)
			feedOutcome(t, result.err, "", OutcomeUnknown, false)
			if result.reply != nil || calls.Load() != 1 {
				t.Fatalf("the ambiguous write returned %d bytes and executed %d times", len(result.reply), calls.Load())
			}
			if keyed {
				s.Close()
				req.FingerprintVersion = 2
				req.Fingerprints[2] = []byte("new encoding")
				receiptAssertReplay(t, storeTestOpen(t, cfg), req, nil, nil)
				feedRequireCount(t, obs, "receipts", 1)
			}
			feedRequireCount(t, obs, "records", 1)
			feedRequireOrder(t, obs, "a")
		})
	}
}
