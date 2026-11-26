package machines_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestApplyFactsAndHarnesses(t *testing.T) {
	e := newRegEnv(t)
	m := e.create(t, 1, "station", true)
	other := e.create(t, 2, "other", false)
	facts := &linkv1.Facts{Os: "Linux", Cores: 9, MemoryBytes: 12 << 30, ClientVersion: "1.2", Workspace: "~/work", AgentLimit: 3}
	inventory := &linkv1.HarnessesFound{Harnesses: []*linkv1.Harness{
		{Name: "codex", Version: proto.String("")},
		{Name: "claude"},
		{Name: "gemini", Version: proto.String("2.4")},
	}}
	wireFacts := proto.CloneOf(facts)
	wireInventory := proto.CloneOf(inventory)
	unknown := protowire.AppendString(protowire.AppendTag(nil, 100, protowire.BytesType), "discard this field")
	wireFacts.ProtoReflect().SetUnknown(unknown)
	wireInventory.ProtoReflect().SetUnknown(unknown)
	wireInventory.Harnesses[0].ProtoReflect().SetUnknown(unknown)

	seen := time.Date(2026, 10, 2, 3, 4, 5, 123456000, time.UTC)
	before := len(e.feed(t))
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		if err := e.reg.ApplyFacts(ctx, tx, m.ID, wireFacts); err != nil {
			return err
		}
		if err := e.reg.ApplyHarnesses(ctx, tx, m.ID, wireInventory); err != nil {
			return err
		}
		if err := e.reg.NoteSeen(ctx, tx, m.ID, seen, "10.2.3.4"); err != nil {
			return err
		}
		return e.reg.Touch(ctx, tx, m.ID)
	})
	if err != nil {
		t.Fatal("The report and last-seen write failed.")
	}

	saved := e.mustGet(t, m.ID)
	var savedFacts linkv1.Facts
	if proto.Unmarshal(saved.Facts, &savedFacts) != nil || !proto.Equal(&savedFacts, facts) {
		t.Error("Stored facts did not preserve exactly the declared fields.")
	}
	var savedInventory linkv1.HarnessesFound
	if !saved.HarnessesScanned || proto.Unmarshal(saved.Harnesses, &savedInventory) != nil || !proto.Equal(&savedInventory, inventory) {
		t.Error("Stored inventory did not preserve optional versions and discard nested and top-level unknown fields.")
	}
	if saved.LastSeenAt == nil || !saved.LastSeenAt.Equal(seen) || saved.LastAddress != "10.2.3.4" {
		t.Error("The machine did not retain the supplied last-seen time and address.")
	}
	if got := e.feedSince(t, before); !reflect.DeepEqual(got, []regFeedEntry{{Topic: "machines", Key: m.ID, Change: "saved"}}) {
		t.Error("The combined report write did not record exactly one machine reference.")
	}
	if untouched := e.mustGet(t, other.ID); untouched.Facts != nil || untouched.HarnessesScanned || untouched.LastSeenAt != nil {
		t.Error("A write for one machine changed another machine.")
	}

	if err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.ApplyHarnesses(ctx, tx, other.ID, &linkv1.HarnessesFound{})
	}); err != nil {
		t.Fatal("The empty inventory write failed.")
	}
	empty := e.mustGet(t, other.ID)
	if !empty.HarnessesScanned || len(machines.Project(empty).GetHarnesses()) != 0 {
		t.Error("An empty inventory did not mark the machine as scanned.")
	}

	wantWaiting := &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_WAITING, Revision: 1}
	waitingRecord := saved
	waitingRecord.Name = "An old name"
	if !proto.Equal(machines.Standing(waitingRecord), wantWaiting) {
		t.Error("Waiting standing did not use the stored revision and an empty name.")
	}
	if err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.SetApproved(ctx, tx, m.ID, "Station")
	}); err != nil {
		t.Fatal("Approving the fixture machine failed.")
	}
	if err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		_, err := e.reg.SetName(ctx, tx, m.ID, "Renamed station")
		return err
	}); err != nil {
		t.Fatal("Renaming the fixture machine failed.")
	}
	wantApproved := &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_APPROVED, Name: "Renamed station", Revision: 3}
	if !proto.Equal(machines.Standing(e.mustGet(t, m.ID)), wantApproved) {
		t.Error("Approved standing did not use the saved name and revision.")
	}
}

func TestLockHoldsOffRemoval(t *testing.T) {
	e := newRegEnv(t)
	m := e.create(t, 1, "station", false)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	locked := make(chan struct{})
	release, unlock := storetest.Signal(t)
	holder := storetest.Start(t, func() error {
		return e.s.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
			got, err := e.reg.Lock(ctx, tx, m.ID)
			if err != nil || got.ID != m.ID {
				return store.NewDomainError("Lock did not return the requested machine.")
			}
			close(locked)
			<-release
			return nil
		})
	})
	defer unlock()
	storetest.ReceiveFrom(t, holder, "the registry lock", locked)
	deleting := make(chan int32, 1)
	remove := storetest.Start(t, func() error {
		return e.s.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
			var pid int32
			if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
				return err
			}
			deleting <- pid
			return e.reg.Delete(ctx, tx, m.ID)
		})
	})
	pid := storetest.ReceiveFrom(t, remove, "the removal backend", deleting)
	storetest.Poll(t, "the removal waiting for the locked machine", func() bool {
		select {
		case <-remove.Done():
			t.Fatal("Removal returned before the locked machine was released.")
		default:
		}
		return storetest.Scalar[bool](t, e.obs, `SELECT EXISTS (
			SELECT FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock'
			AND cardinality(pg_blocking_pids(pid)) > 0)`, pid)
	})
	unlock()
	if holder.Wait(t) != nil || remove.Wait(t) != nil {
		t.Fatal("The lock or removal failed after release.")
	}
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		_, err := e.reg.Lock(ctx, tx, m.ID)
		return err
	})
	if !errors.Is(err, machines.ErrNoMachine) {
		t.Error("Lock did not return ErrNoMachine after removal.")
	}
}
