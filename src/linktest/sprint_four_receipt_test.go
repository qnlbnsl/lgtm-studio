package linktest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
)

func TestSprintFourReceiptReplay(t *testing.T) {
	requirePostgres(t)
	t.Run("StandingResult", testPairedStandingReplay)
	t.Run("ExplicitZero", testPairedZeroReceipt)
	t.Run("DamagedMetadata", func(t *testing.T) {
		t.Run("MissingWatermark", func(t *testing.T) { testPairedDamagedReceipt(t, "") })
		t.Run("InvalidWatermark", func(t *testing.T) { testPairedDamagedReceipt(t, `,"last_event_number":-1`) })
	})
	t.Run("R11", func(t *testing.T) {
		t.Run("LegacyReceiptAndBuffer", testLegacyScanReplay)
	})
}

func testPairedStandingReplay(t *testing.T) {
	t.Helper()
	f := newCommandFixture(t, relayControls{})
	f.dropNext.Store(true)
	approve(t, f.page, f.id, "Standing receipt fixture")
	run := startClient(t, f.opts)
	leg := receive(t, f.relay.legs, "the original standing stream")
	first := waitForCommand(t, leg, stateAction, 0)
	original := waitForResult(t, leg, first, commandDone)
	if original.lastEventNumber == 0 {
		t.Fatal("The original standing receipt did not have a positive watermark.")
	}
	previousMark := drainPairedInventory(t, f, leg, "1.0.0")
	run.stop(t)
	requireAcknowledgedOutbox(t, f.opts, previousMark)
	requirePending(t, f.db, first)

	// Age only the server deadline. The client must replay the standing receipt
	// it actually wrote, including its original watermark.
	agedExpiry := scalar[time.Time](t, f.db, `
UPDATE link_commands SET expires_at=clock_timestamp()-interval '25 hours'
WHERE id=$1 RETURNING expires_at`, first.id)
	previous := first
	previousRun := leg.observe().hello.runID
	for _, version := range []string{"2.0.0", "3.0.0"} {
		drop := version == "2.0.0"
		f.dropNext.Store(drop)
		f.probe.setVersion(t, version)
		run := startClient(t, f.opts)
		leg := receive(t, f.relay.legs, "the next standing replay stream")
		retry := waitForCommand(t, leg, stateAction, 0)
		if retry.id != first.id || retry.attempt != previous.attempt+1 || !retry.expiry.Equal(agedExpiry) ||
			!retry.expiry.Before(time.Now().Add(-24*time.Hour)) {
			t.Fatal("The aged standing command did not retain its identifier and deadline on the next attempt.")
		}
		if got := waitForResult(t, leg, retry, commandDone); got.lastEventNumber != original.lastEventNumber {
			t.Fatalf("The new attempt used watermark %d, want original %d.", got.lastEventNumber, original.lastEventNumber)
		}
		pair := leg.observe().hello
		if pair.runID == "" || pair.runID == previousRun || pair.first != previousMark+1 ||
			waitForWelcome(t, leg).mark != previousMark {
			t.Fatal("The relaunched process did not start with an empty retained prefix and a new run pair.")
		}
		current := drainPairedInventory(t, f, leg, version)
		if current <= original.lastEventNumber || current <= previousMark {
			t.Fatal("The relaunched process did not append independent events beyond the original watermark.")
		}
		if drop {
			requirePending(t, f.db, retry)
		} else {
			waitForStoredResult(t, f.db, retry, "done")
		}
		run.stop(t)
		requireAcknowledgedOutbox(t, f.opts, current)
		previous, previousMark, previousRun = retry, current, pair.runID
	}
	f.probe.requireRuns(t, 3)
}

func drainPairedInventory(t *testing.T, f *commandFixture, leg *relayLeg, version string) uint64 {
	t.Helper()
	waitForInventory(t, f, version)
	current := mark(t, f.db, f.id)
	waitForAck(t, leg, current)
	waitForClientToProcessReplies(t, leg)
	return current
}

func writePairedReceipt(t *testing.T, opts clienttest.Options, id, raw string) {
	t.Helper()
	withClosedClientDB(t, opts, true, func(db *bolt.DB) error {
		return db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte("receipts")).Put([]byte(id), []byte(raw))
		})
	})
}

func testPairedZeroReceipt(t *testing.T) {
	t.Helper()
	f := newCommandFixture(t, relayControls{})
	id, expiry := newPairedID(), time.Now().UTC().Add(time.Hour)
	writePairedReceipt(t, f.opts, id, fmt.Sprintf(
		`{"version":2,"expires_at":%q,"state":"finished","standing":false,"last_event_number":0,"result":"AQ=="}`,
		expiry.Format(time.RFC3339Nano)))
	seedDispatchedScan(t, f, id, expiry)
	run, leg := f.startApproved(t, 0)
	retry := waitForCommand(t, leg, scanAction, 0)
	if retry.id != id || retry.attempt != 2 || waitForResult(t, leg, retry, commandDone).lastEventNumber != 0 {
		t.Fatal("The explicit zero receipt did not replay its original result at attempt 2.")
	}
	waitForStoredResult(t, f.db, retry, "done")
	run.stop(t)
	f.probe.requireRuns(t, 1)
}

func testPairedDamagedReceipt(t *testing.T, watermark string) {
	t.Helper()
	f := newCommandFixture(t, relayControls{})
	id := newPairedID()
	writePairedReceipt(t, f.opts, id, fmt.Sprintf(
		`{"version":2,"expires_at":%q,"state":"finished","standing":false,"result":"AQ=="%s}`,
		time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano), watermark))
	filename := filepath.Join(f.opts.StateDir, "client.db")
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	startClient(t, f.opts).expectCleanExit(t)
	requireSingleHistoryStop(t, f.opts.Stderr.(*diagnostics), "store_failed")
	after, err := os.ReadFile(filename)
	if len(f.relay.legs) != 0 || err != nil || !bytes.Equal(before, after) {
		t.Fatal("A damaged receipt did not stop the client before dial or preserve its bytes.")
	}
}
