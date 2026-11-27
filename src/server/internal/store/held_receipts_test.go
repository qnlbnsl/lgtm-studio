package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func heldReceiptMachineRequest() Request {
	req := receiptRequest()
	req.Caller = "owner"
	req.Procedure = lgtmv1connect.ClientServiceRenameClientProcedure
	// Machines frames the identifier and name as length-prefixed fields.
	var fingerprint []byte
	for _, field := range []string{"0198fa57-a805-7000-8000-000000000001", "fixture machine"} {
		fingerprint = binary.BigEndian.AppendUint64(fingerprint, uint64(len(field)))
		fingerprint = append(fingerprint, field...)
	}
	req.Fingerprints[1] = fingerprint
	return req
}

func heldReceiptProjectRequest() Request {
	req := heldReceiptMachineRequest()
	req.Procedure = lgtmv1connect.ProjectServiceInspectFolderProcedure
	return req
}

func TestHeldReceiptUpgrade(t *testing.T) {
	s, _, obs := poolOpen(t, "held_upgrade")
	ctx := t.Context()
	v1, err := storeMigrations.ReadFile("migrations/0001_store.sql")
	if err != nil {
		t.Fatal("the merged Store migration could not be read")
	}
	storetest.Exec(t, obs, string(v1))
	storetest.Exec(t, obs, `
CREATE TABLE schema_migrations (part text PRIMARY KEY, version integer NOT NULL CHECK (version > 0));
INSERT INTO schema_migrations VALUES ('store', 1);`)

	req := heldReceiptMachineRequest()
	reply := []byte{0x00, 0xf0, 0xff}
	principal, digest := sha256.Sum256([]byte(req.Caller)), receiptDigest(req)
	cutoff := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	storetest.Exec(t, obs, `
INSERT INTO receipts (principal_hash, request_key, fingerprint_version, request_hash, reply, written_at, retention_started_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		principal[:], req.Key, req.FingerprintVersion, digest[:], reply, cutoff.Add(-time.Hour), cutoff)
	before := storetest.Scalar[string](t, obs, "SELECT to_jsonb(r)::text FROM receipts r")
	checkOldColumns := func() {
		t.Helper()
		got := storetest.Scalar[string](t, obs, "SELECT (to_jsonb(r) - 'held')::text FROM receipts r")
		if got != before {
			t.Fatal("the Store migration changed an existing receipt column")
		}
	}

	// A later part rejects the startup transaction after the Store migration runs.
	interrupted := Migrations{Part: "fixture", Files: migrateFiles(map[string]string{
		"0001_interrupt.sql": "SELECT 1 / 0;",
	})}
	if err := s.Migrate(ctx, interrupted); err == nil {
		t.Fatal("the failed startup migration returned no error")
	}
	ledger, _ := migrateLedger(t, obs)
	migrateEqualLedger(t, ledger, map[string]int{"store": 1})
	if storetest.Scalar[bool](t, obs, "SELECT to_jsonb(r) ? 'held' FROM receipts r") {
		t.Fatal("the failed startup migration changed the receipt schema")
	}
	checkOldColumns()
	receiptAssertReplay(t, s, req, reply, nil)

	for range 2 {
		if err := s.Migrate(ctx); err != nil {
			t.Fatalf("the Store upgrade failed: %v", err)
		}
		ledger, _ := migrateLedger(t, obs)
		migrateEqualLedger(t, ledger, map[string]int{"store": 2})
		checkOldColumns()
		if storetest.Scalar[bool](t, obs, "SELECT held FROM receipts") {
			t.Fatal("the Store upgrade marked the existing ordinary receipt as held")
		}
		receiptAssertReplay(t, s, req, reply, nil)
	}
	if !storetest.Scalar[bool](t, obs, `
SELECT data_type = 'boolean' AND is_nullable = 'NO'
FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'receipts' AND column_name = 'held'`) {
		t.Fatal("the hold column is not a required boolean")
	}
	probe := pruneFixtureTx(t, obs)
	var defaultHold bool
	if err := probe.QueryRow(ctx, `
INSERT INTO receipts (principal_hash, request_key, fingerprint_version, request_hash, reply)
SELECT principal_hash, 'legacy-default-probe', fingerprint_version, request_hash, reply FROM receipts
RETURNING held`).Scan(&defaultHold); err != nil || defaultHold {
		t.Fatal("a receipt inserted through the old columns did not default to ordinary")
	}
	if err := probe.Rollback(ctx); err != nil {
		t.Fatal("the legacy insert probe could not be rolled back")
	}

	err = s.withMaintenance(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return applyMigrations(ctx, tx, map[string][]string{"store": {string(v1)}})
	})
	if !errors.Is(err, errLedger) {
		t.Fatalf("the version 1 manifest returned %v against Store version 2, want the ledger refusal", err)
	}
	checkOldColumns()

	got, err := s.pruneBefore(ctx, cutoff)
	pruneCheckResult(t, got, err, 0, 0)
	receiptAssertReplay(t, s, req, reply, nil)
	got, err = s.pruneBefore(ctx, cutoff.Add(time.Microsecond))
	pruneCheckResult(t, got, err, 0, 1)
	feedRequireCount(t, obs, "receipts", 0)
}

func heldReceiptNoReplay(context.Context, *Tx, []byte) error {
	return NewDomainError("An unexpected retained replay ran.")
}

func heldReceiptNoStart(context.Context, *Tx) ([]byte, error) {
	return nil, NewDomainError("An unexpected retained start ran.")
}

func TestHeldReceiptSharedKeys(t *testing.T) {
	for _, retainedWins := range []bool{false, true} {
		name := "ordinary wins"
		if retainedWins {
			name = "retained wins"
		}
		t.Run(name, func(t *testing.T) {
			s, cfg, obs := feedFixture(t, nil)
			second := storeTestOpen(t, cfg)
			ordinary := heldReceiptMachineRequest()
			retained := heldReceiptProjectRequest()
			// Each procedure has the same semantic bytes. The procedure alone makes the request conflict.
			reply := []byte("first operation")
			resume, release := storetest.Signal(t)
			defer release()
			entered := make(chan int32, 1)
			var winnerStarts, loserStarts atomic.Int32
			start := func(ctx context.Context, tx *Tx) ([]byte, error) {
				winnerStarts.Add(1)
				var pid int32
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
					return nil, err
				}
				if err := feedSave(ctx, tx, "first"); err != nil {
					return nil, err
				}
				entered <- pid
				select {
				case <-resume:
					return reply, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			call := func(s *Store, retainedMode bool, start func(context.Context, *Tx) ([]byte, error)) *storetest.Task[receiptResult] {
				return storetest.Start(t, func() receiptResult {
					if retainedMode {
						got, err := s.WriteOnceRetained(t.Context(), retained, start, heldReceiptNoReplay)
						return receiptResult{got, err}
					}
					got, err := s.WriteOnce(t.Context(), ordinary, start)
					return receiptResult{got, err}
				})
			}
			first := call(s, retainedWins, start)
			blocker := heldReceiptWaitForCallback(t, first, entered)
			loser := call(second, !retainedWins, func(ctx context.Context, tx *Tx) ([]byte, error) {
				loserStarts.Add(1)
				return receiptWrite("second", []byte("second operation"))(ctx, tx)
			})
			heldReceiptWaitForKey(t, obs, ordinary, loser, blocker)
			release()
			if got := first.Wait(t); got.err != nil || !bytes.Equal(got.reply, reply) {
				t.Fatalf("the winning call returned %d bytes and error %v", len(got.reply), got.err)
			}
			if got := loser.Wait(t); !errors.Is(got.err, ErrRequestKeyConflict) || got.reply != nil {
				t.Fatalf("the competing call returned %d bytes and error %v, want a conflict", len(got.reply), got.err)
			}
			if winnerStarts.Load() != 1 || loserStarts.Load() != 0 {
				t.Fatalf("the winning and competing callbacks ran %d and %d times, want one and zero", winnerStarts.Load(), loserStarts.Load())
			}
			feedRequireOrder(t, obs, "first")
			feedRequireCount(t, obs, "records", 1)
			feedRequireCount(t, obs, "receipts", 1)
			if storetest.Scalar[bool](t, obs, "SELECT held FROM receipts") != retainedWins {
				t.Fatal("the saved hold does not match the winning call")
			}

			if retainedWins {
				replays := 0
				got, err := second.WriteOnceRetained(t.Context(), retained, heldReceiptNoStart,
					func(ctx context.Context, tx *Tx, saved []byte) error {
						replays++
						if !bytes.Equal(saved, reply) {
							return NewDomainError("The replay did not receive the saved bytes.")
						}
						return feedSave(ctx, tx, "replay")
					})
				if err != nil || !bytes.Equal(got, reply) || replays != 1 {
					t.Fatalf("the retained retry invoked %d replays, returned %d bytes and error %v", replays, len(got), err)
				}
				feedRequireOrder(t, obs, "first", "replay")

				changed := heldReceiptProjectRequest()
				changed.Fingerprints[1] = []byte("a different path")
				got, err = second.WriteOnceRetained(t.Context(), changed, heldReceiptNoStart, heldReceiptNoReplay)
				if !errors.Is(err, ErrRequestKeyConflict) || got != nil {
					t.Fatalf("the changed request returned %d bytes and error %v, want a conflict", len(got), err)
				}
			}
		})
	}

	t.Run("retained requests require a key", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		req := heldReceiptProjectRequest()
		req.Key = ""
		called := false
		reply, err := s.WriteOnceRetained(t.Context(), req, func(ctx context.Context, tx *Tx) ([]byte, error) {
			called = true
			return receiptWrite("unkeyed", []byte("operation"))(ctx, tx)
		}, heldReceiptNoReplay)
		if err == nil || reply != nil || called {
			t.Fatalf("the unkeyed retained call returned %d bytes and error %v, callback ran = %t", len(reply), err, called)
		}
		feedRequireCount(t, obs, "records", 0)
		feedRequireCount(t, obs, "receipts", 0)
		feedRequireCount(t, obs, "feed", 0)
	})
}

func heldReceiptWaitForCallback[R any](t *testing.T, task *storetest.Task[R], entered <-chan int32) int32 {
	t.Helper()
	select {
	case pid := <-entered:
		return pid
	case <-task.Done():
		t.Fatal("the write finished before its expected callback")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("the expected callback did not run")
	}
	return 0
}

func heldReceiptWaitForKey[R any](t *testing.T, obs *pgx.Conn, req Request, waiter *storetest.Task[R], blocker int32) {
	t.Helper()
	key := receiptKey(receiptLockWord(t, obs, req))
	storetest.Poll(t, "the competing write waiting on the shared receipt key", func() bool {
		select {
		case <-waiter.Done():
			t.Fatal("the competing write finished while the first callback still held the key")
		default:
		}
		for _, pid := range poolBlockedPIDs(t, obs, key) {
			blocked, err := storetest.IsBlocked(t.Context(), obs, pid, blocker, key)
			if err != nil {
				t.Fatal("the shared receipt key could not be observed")
			}
			if blocked {
				return true
			}
		}
		return false
	})
}

func heldReceiptStartOperation(t *testing.T, s *Store, req Request, identifier string) {
	t.Helper()
	want := []byte(identifier)
	reply, err := s.WriteOnceRetained(t.Context(), req, receiptWrite(identifier, want), heldReceiptNoReplay)
	if err != nil || !bytes.Equal(reply, want) {
		t.Fatalf("held admission returned %d bytes and error %v, want the operation identifier", len(reply), err)
	}
}

func heldReceiptCheckPair(t *testing.T, obs *pgx.Conn, req Request, identifier string, present bool) {
	t.Helper()
	var receipt, operation bool
	err := obs.QueryRow(t.Context(), `
SELECT EXISTS (SELECT FROM receipts WHERE request_key = $1 AND held AND reply = $2),
       EXISTS (SELECT FROM records WHERE id = $3)`, req.Key, []byte(identifier), identifier).Scan(&receipt, &operation)
	if err != nil {
		t.Fatal("the held receipt and operation could not be read")
	}
	if receipt != present || operation != present {
		t.Fatalf("held receipt present = %t, operation present = %t, want both %t", receipt, operation, present)
	}
}

func heldReceiptRemove(ctx context.Context, tx *Tx, identifier string) (bool, error) {
	tag, err := tx.Exec(ctx, "DELETE FROM records WHERE id = $1", identifier)
	return tag.RowsAffected() == 1, err
}

func heldReceiptInstallDeleteFault(t *testing.T, obs *pgx.Conn) {
	t.Helper()
	storetest.Exec(t, obs, `
CREATE FUNCTION held_receipt_reject_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'fixture rejected receipt deletion' USING ERRCODE = '23514';
END;
$$;
CREATE CONSTRAINT TRIGGER held_receipt_reject_delete
AFTER DELETE ON receipts DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW WHEN (OLD.held) EXECUTE FUNCTION held_receipt_reject_delete();`)
}

func TestHeldReceiptPruneAndRetire(t *testing.T) {
	t.Run("pruning and guarded retirement", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		ctx := t.Context()
		req := heldReceiptProjectRequest()
		const identifier = "held operation"
		heldReceiptStartOperation(t, s, req, identifier)
		ordinary := heldReceiptMachineRequest()
		ordinary.Key = "ordinary-request"
		if _, err := s.WriteOnce(ctx, ordinary, receiptWrite("ordinary operation", []byte("ordinary operation"))); err != nil {
			t.Fatalf("ordinary admission failed: %v", err)
		}

		// Both rows look old, but only the ordinary row gets an observation stamp.
		now := storetest.Scalar[time.Time](t, obs, "SELECT clock_timestamp()")
		cutoff := now.Add(-24 * time.Hour)
		old := cutoff.Add(-time.Microsecond)
		storetest.Exec(t, obs, "UPDATE receipts SET written_at = $1", old)
		result, err := s.pruneBefore(ctx, cutoff)
		pruneCheckResult(t, result, err, 0, 0)
		if !storetest.Scalar[bool](t, obs, "SELECT retention_started_at IS NOT NULL FROM receipts WHERE request_key = $1", ordinary.Key) {
			t.Fatal("the ordinary receipt was not stamped")
		}
		if storetest.Scalar[bool](t, obs, "SELECT retention_started_at IS NOT NULL FROM receipts WHERE request_key = $1", req.Key) {
			t.Fatal("ordinary pruning stamped the held receipt")
		}

		var calls int
		decline := func(context.Context, *Tx) (bool, error) {
			calls++
			return false, nil
		}
		for _, tc := range []struct {
			request Request
			reply   []byte
			calls   int
		}{
			{req, []byte("wrong operation"), 0},
			{ordinary, []byte("ordinary operation"), 0},
			{req, []byte(identifier), 1},
		} {
			calls = 0
			retired, err := s.RetireReceipt(ctx, tc.request.Caller, tc.request.Key, tc.reply, decline)
			if err != nil || retired || calls != tc.calls {
				t.Errorf("guarded retirement returned %t and %v, calling removal %d times, want %d", retired, err, calls, tc.calls)
			}
			feedRequireCount(t, obs, "receipts", 2)
			feedRequireCount(t, obs, "records", 2)
			heldReceiptCheckPair(t, obs, req, identifier, true)
		}

		// Give both rows an expired stamp so a missing deletion filter is visible.
		storetest.Exec(t, obs, "UPDATE receipts SET retention_started_at = $1", old)
		result, err = s.pruneBefore(ctx, cutoff)
		pruneCheckResult(t, result, err, 0, 1)
		feedRequireCount(t, obs, "receipts", 1)
		heldReceiptCheckPair(t, obs, req, identifier, true)
	})

	t.Run("failed retirement rolls back both rows", func(t *testing.T) {
		s, _, obs := feedFixture(t, nil)
		ctx := t.Context()
		req := heldReceiptProjectRequest()
		const identifier = "held operation"
		heldReceiptStartOperation(t, s, req, identifier)
		heldReceiptInstallDeleteFault(t, obs)
		calls := 0
		remove := func(ctx context.Context, tx *Tx) (bool, error) {
			calls++
			return heldReceiptRemove(ctx, tx, identifier)
		}
		retired, err := s.RetireReceipt(ctx, req.Caller, req.Key, []byte(identifier), remove)
		if retired || calls != 1 {
			t.Errorf("failed retirement returned %t and ran removal %d times, want false and one call", retired, calls)
		}
		heldReceiptCheckPair(t, obs, req, identifier, true)
		feedOutcome(t, err, "23514", OutcomeRolledBack, false)

		storetest.Exec(t, obs, "DROP TRIGGER held_receipt_reject_delete ON receipts")
		retired, err = s.RetireReceipt(ctx, req.Caller, req.Key, []byte(identifier), remove)
		if err != nil || !retired || calls != 2 {
			t.Fatalf("retirement retry returned %t and %v, total removal calls = %d", retired, err, calls)
		}
		heldReceiptCheckPair(t, obs, req, identifier, false)
		feedRequireCount(t, obs, "receipts", 0)
		feedRequireCount(t, obs, "records", 0)
	})

	t.Run("replay wins the receipt lock", func(t *testing.T) { heldReceiptReplayRace(t, true) })
	t.Run("retirement wins the receipt lock", func(t *testing.T) { heldReceiptReplayRace(t, false) })
	for _, phase := range []string{"admission", "retirement"} {
		t.Run(phase+" committed with a lost reply", func(t *testing.T) {
			heldReceiptLostCommit(t, phase == "retirement", false)
		})
		t.Run(phase+" rolled back with a lost reply", func(t *testing.T) {
			heldReceiptLostCommit(t, phase == "retirement", true)
		})
	}
}

type heldReceiptRetirement struct {
	retired bool
	err     error
}

func heldReceiptReplayRace(t *testing.T, replayFirst bool) {
	s, cfg, obs := feedFixture(t, nil)
	second := storeTestOpen(t, cfg)
	ctx := t.Context()
	req := heldReceiptProjectRequest()
	const original, replacement = "original operation", "replacement operation"
	heldReceiptStartOperation(t, s, req, original)
	resume, release := storetest.Signal(t)
	defer release()
	entered := make(chan int32, 1)
	pause := func(ctx context.Context, tx *Tx) error {
		var pid int32
		if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
			return err
		}
		entered <- pid
		select {
		case <-resume:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	retire := func() heldReceiptRetirement {
		retired, err := s.RetireReceipt(ctx, req.Caller, req.Key, []byte(original), func(ctx context.Context, tx *Tx) (bool, error) {
			if !replayFirst {
				if err := pause(ctx, tx); err != nil {
					return false, err
				}
			}
			return heldReceiptRemove(ctx, tx, original)
		})
		return heldReceiptRetirement{retired, err}
	}
	replay := func() receiptResult {
		reply, err := second.WriteOnceRetained(ctx, req, receiptWrite(replacement, []byte(replacement)),
			func(ctx context.Context, tx *Tx, saved []byte) error {
				if !replayFirst || !bytes.Equal(saved, []byte(original)) {
					return NewDomainError("The retained replay read the wrong operation.")
				}
				if err := pause(ctx, tx); err != nil {
					return err
				}
				var present bool
				if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT FROM records WHERE id = $1)", string(saved)).Scan(&present); err != nil {
					return err
				}
				if !present {
					return NewDomainError("The operation retired before replay could read it.")
				}
				return nil
			})
		return receiptResult{reply, err}
	}
	var replayTask *storetest.Task[receiptResult]
	var retireTask *storetest.Task[heldReceiptRetirement]
	if replayFirst {
		replayTask = storetest.Start(t, replay)
		blocker := heldReceiptWaitForCallback(t, replayTask, entered)
		retireTask = storetest.Start(t, retire)
		heldReceiptWaitForKey(t, obs, req, retireTask, blocker)
	} else {
		retireTask = storetest.Start(t, retire)
		blocker := heldReceiptWaitForCallback(t, retireTask, entered)
		replayTask = storetest.Start(t, replay)
		heldReceiptWaitForKey(t, obs, req, replayTask, blocker)
	}
	heldReceiptCheckPair(t, obs, req, original, true)
	if unlocked := storetest.Scalar[int](t, obs, `
SELECT count(*) FROM (SELECT 1 FROM receipts WHERE request_key = $1 FOR UPDATE SKIP LOCKED) AS unlocked`, req.Key); unlocked != 0 {
		t.Fatal("the receipt row was unlocked during the winning callback")
	}
	release()
	if got := retireTask.Wait(t); got.err != nil || !got.retired {
		t.Fatalf("retirement returned %t and %v, want success", got.retired, got.err)
	}
	want := replacement
	if replayFirst {
		want = original
	}
	if got := replayTask.Wait(t); got.err != nil || !bytes.Equal(got.reply, []byte(want)) {
		t.Fatalf("the retry returned %d bytes and error %v, want the operation from the winning lock order", len(got.reply), got.err)
	}
	heldReceiptCheckPair(t, obs, req, original, false)
	if replayFirst {
		feedRequireCount(t, obs, "receipts", 0)
		feedRequireCount(t, obs, "records", 0)
	} else {
		heldReceiptCheckPair(t, obs, req, replacement, true)
		feedRequireCount(t, obs, "receipts", 1)
		feedRequireCount(t, obs, "records", 1)
	}
}

func heldReceiptDropCommit(t *testing.T, cfg *pgxpool.Config, drop <-chan struct{}, arrived chan<- struct{}) *Store {
	t.Helper()
	var armed atomic.Bool
	client := cfg.Copy()
	client.ConnConfig.Tracer = &feedTestTracer{before: func(context.Context, pgx.Tx) error {
		armed.Store(true)
		return nil
	}}
	dial := client.ConnConfig.DialFunc
	client.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &receiptCommitReplyConn{Conn: conn, armed: &armed, arrived: arrived, drop: drop}, nil
	}
	return storeTestOpen(t, client)
}

func heldReceiptLostCommit(t *testing.T, retirement, reject bool) {
	t.Helper()
	s, cfg, obs := feedFixture(t, nil)
	ctx := t.Context()
	req := heldReceiptProjectRequest()
	const original, replacement = "original operation", "replacement operation"
	if retirement {
		heldReceiptStartOperation(t, s, req, original)
		if reject {
			heldReceiptInstallDeleteFault(t, obs)
		}
	} else if reject {
		storetest.Exec(t, obs, `
CREATE FUNCTION held_receipt_reject_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'fixture rejected receipt insertion' USING ERRCODE = '23514';
END;
$$;
CREATE CONSTRAINT TRIGGER held_receipt_reject_insert
AFTER INSERT ON receipts DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW WHEN (NEW.held) EXECUTE FUNCTION held_receipt_reject_insert();`)
	}

	drop, release := storetest.Signal(t)
	defer release()
	arrived := make(chan struct{}, 1)
	client := heldReceiptDropCommit(t, cfg, drop, arrived)
	type result struct {
		reply   []byte
		retired bool
		err     error
	}
	remove := func(ctx context.Context, tx *Tx) (bool, error) { return heldReceiptRemove(ctx, tx, original) }
	task := storetest.Start(t, func() result {
		if retirement {
			retired, err := client.RetireReceipt(ctx, req.Caller, req.Key, []byte(original), remove)
			return result{retired: retired, err: err}
		}
		reply, err := client.WriteOnceRetained(ctx, req, func(ctx context.Context, tx *Tx) ([]byte, error) {
			// With no feed references, the deferred constraint runs inside COMMIT.
			_, err := tx.Exec(ctx, "INSERT INTO records VALUES ($1, $1)", original)
			return []byte(original), err
		}, heldReceiptNoReplay)
		return result{reply: reply, err: err}
	})
	select {
	case <-arrived:
	case <-task.Done():
		t.Fatal("the held write finished before its commit reply could be dropped")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("the held write did not reach the commit reply")
	}
	present := !reject
	if retirement {
		present = reject
	}
	heldReceiptCheckPair(t, obs, req, original, present)
	release()
	got := task.Wait(t)
	if got.reply != nil || got.retired {
		t.Fatal("the write reported success without a commit reply")
	}
	feedOutcome(t, got.err, "", OutcomeUnknown, false)
	client.Close()
	reopened := storeTestOpen(t, cfg)
	heldReceiptCheckPair(t, obs, req, original, present)
	if retirement {
		if reject {
			storetest.Exec(t, obs, "DROP TRIGGER held_receipt_reject_delete ON receipts")
		}
		retired, err := reopened.RetireReceipt(ctx, req.Caller, req.Key, []byte(original), remove)
		if err != nil || retired != reject {
			t.Fatalf("retirement retry returned %t and %v, want %t", retired, err, reject)
		}
		heldReceiptCheckPair(t, obs, req, original, false)
		feedRequireCount(t, obs, "receipts", 0)
		feedRequireCount(t, obs, "records", 0)
		return
	}
	if reject {
		storetest.Exec(t, obs, "DROP TRIGGER held_receipt_reject_insert ON receipts")
		heldReceiptStartOperation(t, reopened, req, replacement)
		heldReceiptCheckPair(t, obs, req, replacement, true)
	} else {
		replays := 0
		reply, err := reopened.WriteOnceRetained(ctx, req, heldReceiptNoStart, func(_ context.Context, _ *Tx, saved []byte) error {
			replays++
			if !bytes.Equal(saved, []byte(original)) {
				return NewDomainError("The reopened store replayed a different operation.")
			}
			return nil
		})
		if err != nil || replays != 1 || !bytes.Equal(reply, []byte(original)) {
			t.Fatalf("admission retry replayed %d times, returned %d bytes and error %v", replays, len(reply), err)
		}
	}
	feedRequireCount(t, obs, "receipts", 1)
	feedRequireCount(t, obs, "records", 1)
}
