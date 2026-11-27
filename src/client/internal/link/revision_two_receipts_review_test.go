package link

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

func TestRevisionTwoReceiptRejectsDuplicateWatermark(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	for want := uint64(1); want <= 2; want++ {
		if got := appendLinkFixture(t, store, storedReport(t, replayFacts(tNow))); got != want {
			t.Fatalf("seeded event number %d, want %d", got, want)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal("could not close the seeded store")
	}

	cmd := cmdAt(1, time.Hour)
	raw := fmt.Sprintf(`{"version":2,"expires_at":%q,"state":"finished","standing":false,"last_event_number":2,"last_event_number":0,"result":"AQ=="}`,
		cmd.ExpiresAt.Format(time.RFC3339))
	path := folder.Paths().Store
	putReceipts(t, path, map[string]string{cmd.ID: raw})
	before := readBytes(t, path)

	store = mustOpen(t, folder, StoreOptions{})
	if err := store.prepareRun(); !errors.Is(err, ErrStoreDamaged) {
		t.Error("receipt preparation accepted conflicting legal watermarks instead of returning ErrStoreDamaged")
	}
	if err := store.Close(); err != nil {
		t.Fatal("could not close the tested store")
	}
	if !bytes.Equal(readBytes(t, path), before) {
		t.Error("receipt preparation changed the store with conflicting watermarks")
	}
}

func TestRevisionTwoReceiptRunValidatesBeforeTransport(t *testing.T) {
	f := newRunPeerFixture(t, protocols[0])
	f.seed(t)
	path := f.file()
	putReceipts(t, path, map[string]string{uid(1): "{"})
	before := readBytes(t, path)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var initializations int
	newRunLinkClient = func(enroll.Server) (linkv1connect.LinkServiceClient, func(), error) {
		initializations++
		cancel()
		return nil, nil, errors.New("fixture transport unavailable")
	}
	if outcome, err := Run(ctx, f.opts); outcome != RunRecoveryStop || err != nil {
		t.Error("Run did not return RunRecoveryStop without an error for the malformed receipt")
	}
	if initializations != 0 {
		t.Errorf("Run initialized transport %d times before reporting a damaged receipt, want zero", initializations)
	}
	if !bytes.Equal(readBytes(t, path), before) {
		t.Error("Run changed the store with a malformed receipt")
	}
}

func TestRevisionTwoReceiptPruningPreservesDamage(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	requireReceiptPreparation(t, store)
	cmd := cmdAt(2, time.Hour)
	if decision, _, err := store.Begin(cmd, tNow); err != nil || decision != DecisionRun {
		t.Fatal("could not start the command before planting an unrelated damaged receipt")
	}
	started := receiptOnDisk(t, store, cmd.ID)
	if started == "" {
		t.Fatal("the command start was missing before the finish")
	}

	badID, bad := uid(1), "{"
	if err := store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(receiptsBucket).Put([]byte(badID), []byte(bad))
	}); err != nil {
		t.Fatal("could not plant the unrelated damaged receipt")
	}

	if _, err := store.finish(cmd.ID, []byte{byte(commandDone)}, tNow); !errors.Is(err, ErrStoreDamaged) {
		t.Error("finishing with a trusted pruning time did not report ErrStoreDamaged")
	}
	if err := store.Close(); err != nil {
		t.Fatal("could not close the tested store")
	}
	persisted := readReceipts(t, folder.Paths().Store)
	if persisted[badID] != bad {
		t.Error("pruning changed or removed the unrelated damaged receipt")
	}
	if persisted[cmd.ID] != started {
		t.Error("the failed finish changed or removed the started receipt")
	}
}

func TestRevisionTwoReceiptRejectsResultOnStarted(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	if err := store.Close(); err != nil {
		t.Fatal("could not close the seeded store")
	}
	cmd := cmdAt(1, time.Hour)
	raw := fmt.Sprintf(`{"version":2,"expires_at":%q,"state":"started","standing":false,"last_event_number":null,"result":"AQ=="}`,
		cmd.ExpiresAt.Format(time.RFC3339))
	path := folder.Paths().Store
	putReceipts(t, path, map[string]string{cmd.ID: raw})

	store = mustOpen(t, folder, StoreOptions{})
	if err := store.prepareRun(); !errors.Is(err, ErrStoreDamaged) {
		t.Error("receipt preparation accepted a result on a started receipt instead of returning ErrStoreDamaged")
	}
	if err := store.Close(); err != nil {
		t.Fatal("could not close the tested store")
	}
	if readReceipts(t, path)[cmd.ID] != raw {
		t.Error("receipt preparation changed or removed the damaged started receipt")
	}
}

func TestRevisionTwoReceiptFirstDeclineDecision(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	var commits int
	t.Cleanup(swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
		err := db.Update(fn)
		if err == nil {
			commits++
		}
		return err
	}))
	cmd := cmdAt(1, -time.Minute)
	decision, replay, err := store.Decline(cmd, []byte{byte(commandExpired)})
	if err != nil {
		t.Error("the first decline returned an error")
	}
	if decision != DecisionExpired {
		t.Errorf("the first decline returned decision %d, want DecisionExpired", decision)
	}
	if len(replay) != 0 {
		t.Errorf("the first decline returned %d replay bytes, want zero", len(replay))
	}
	if commits != 1 {
		t.Errorf("the first decline committed %d writes, want one", commits)
	}
	if err := store.Close(); err != nil {
		t.Fatal("could not close the tested store")
	}
	persisted := readReceipts(t, folder.Paths().Store)
	if len(persisted) != 1 {
		t.Errorf("the first decline left %d durable receipts, want one", len(persisted))
	}
	requireRevisionTwoReceipt(t, persisted[cmd.ID], "finished", false, cmd.ExpiresAt, 0, `"Aw=="`)
}
