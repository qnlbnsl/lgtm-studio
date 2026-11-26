package machines_test

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// settle is how long a test waits before it checks that something did not happen.
const settle = 250 * time.Millisecond

var uuidV7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// regEnv is the fixture of the tests of the registry and the tokens: a migrated store,
// the registry, and a second connection on the same database for the test's own queries.
type regEnv struct {
	s   *store.Store
	obs *pgx.Conn
	reg *machines.Registry
}

func newRegEnv(t *testing.T) *regEnv {
	t.Helper()
	config := storetest.NewDatabase(t, "machines_a")
	ctx := t.Context()
	s, err := store.Open(ctx, config, store.ConnectionPolicy{})
	if err != nil {
		t.Fatalf("opening the fixture store failed: %v", err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx, machines.Migrations()); err != nil {
		t.Fatalf("applying the machine migrations failed: %v", err)
	}
	obs, err := pgx.ConnectConfig(ctx, config.ConnConfig.Copy())
	if err != nil {
		t.Fatalf("the fixture connection could not connect: %v", err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = obs.Close(cleanup)
	})
	return &regEnv{s: s, obs: obs, reg: machines.NewRegistry()}
}

func (e *regEnv) write(t *testing.T, fn func(ctx context.Context, tx *store.Tx) error) error {
	t.Helper()
	return e.s.Write(t.Context(), fn)
}

// create makes a waiting machine and fails the test if that does not work.
func (e *regEnv) create(t *testing.T, key byte, host string, hostsServer bool) machines.Machine {
	t.Helper()
	var m machines.Machine
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		m, err = e.reg.CreateWaiting(ctx, tx, machines.NewMachine{
			PublicKey: bytes.Repeat([]byte{key}, 32), HostName: host, TokenID: "token-" + host, HostsServer: hostsServer,
		})
		return err
	})
	if err != nil {
		t.Fatalf("CreateWaiting(%q) failed: %v", host, err)
	}
	if !uuidV7.MatchString(m.ID) {
		t.Fatalf("CreateWaiting(%q) returned the id %q, want a version 7 identifier", host, m.ID)
	}
	return m
}

func (e *regEnv) get(t *testing.T, id string) (machines.Machine, error) {
	t.Helper()
	var m machines.Machine
	err := e.s.Read(t.Context(), func(ctx context.Context, tx *store.ReadTx) error {
		var err error
		m, err = e.reg.Get(ctx, tx, id)
		return err
	})
	return m, err
}

func (e *regEnv) mustGet(t *testing.T, id string) machines.Machine {
	t.Helper()
	m, err := e.get(t, id)
	if err != nil {
		t.Fatalf("Get(%s) failed: %v", id, err)
	}
	return m
}

func (e *regEnv) list(t *testing.T) []machines.Machine {
	t.Helper()
	var all []machines.Machine
	err := e.s.Read(t.Context(), func(ctx context.Context, tx *store.ReadTx) error {
		var err error
		all, err = e.reg.List(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	return all
}

type regFeedEntry struct{ Topic, Scope, Key, Change string }

func (e *regEnv) feed(t *testing.T) []regFeedEntry {
	t.Helper()
	rows, err := e.obs.Query(t.Context(), "SELECT topic, scope, key, change FROM feed ORDER BY position")
	if err != nil {
		t.Fatalf("reading the feed failed: %v", err)
	}
	entries, err := pgx.CollectRows(rows, pgx.RowToStructByPos[regFeedEntry])
	if err != nil {
		t.Fatalf("reading the feed failed: %v", err)
	}
	return entries
}

func (e *regEnv) feedHead(t *testing.T) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, e.obs, "SELECT COALESCE(max(position), 0) FROM feed")
}

// feedSince returns the entries after the first n.
func (e *regEnv) feedSince(t *testing.T, n int) []regFeedEntry {
	t.Helper()
	all := e.feed(t)
	if len(all) < n {
		t.Fatalf("the feed lost entries: %d, want at least %d", len(all), n)
	}
	return all[n:]
}

func randomID(t *testing.T) string {
	t.Helper()
	id, err := store.NewID()
	if err != nil {
		t.Fatalf("NewID failed: %v", err)
	}
	return id
}

func TestCreateWaitingAndRead(t *testing.T) {
	e := newRegEnv(t)
	key := bytes.Repeat([]byte{0x11}, 32)
	before := len(e.feed(t))

	var m machines.Machine
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		m, err = e.reg.CreateWaiting(ctx, tx, machines.NewMachine{
			PublicKey: key, HostName: "box", TokenID: "token-one", HostsServer: true,
		})
		return err
	})
	if err != nil {
		t.Fatalf("CreateWaiting failed: %v", err)
	}
	if !uuidV7.MatchString(m.ID) {
		t.Fatalf("the id is %q, want a version 7 identifier", m.ID)
	}
	if !bytes.Equal(m.PublicKey, key) {
		t.Errorf("the key is %x, want %x", m.PublicKey, key)
	}
	if m.HostName != "box" || m.TokenID != "token-one" || !m.HostsServer {
		t.Errorf("the record is %+v, want host name box, token token-one and HostsServer true", m)
	}
	if m.Approval != "waiting" || m.StateRevision != 1 || m.Name != "" {
		t.Errorf("the record is %+v, want approval waiting, revision 1 and an empty name", m)
	}

	got := e.mustGet(t, m.ID)
	if got.ID != m.ID || !bytes.Equal(got.PublicKey, key) || got.HostName != "box" || got.TokenID != "token-one" ||
		!got.HostsServer || got.Approval != "waiting" || got.StateRevision != 1 || got.Name != "" {
		t.Errorf("Get returned %+v, want what CreateWaiting returned (%+v)", got, m)
	}
	if got.JoinedAt.IsZero() || got.ApprovedAt != nil || got.HarnessesScanned || got.LastSeenAt != nil || got.LastAddress != "" {
		t.Errorf("Get returned %+v, want a joined time and nothing that only the stream writes", got)
	}
	if all := e.list(t); len(all) != 1 || all[0].ID != m.ID {
		t.Errorf("List returned %+v, want the one machine", all)
	}

	added := e.feedSince(t, before)
	if want := (regFeedEntry{"machines", "", m.ID, "saved"}); len(added) != 1 || added[0] != want {
		t.Errorf("the feed gained %+v, want exactly %+v", added, want)
	}

	if _, err := e.get(t, randomID(t)); !errors.Is(err, machines.ErrNoMachine) {
		t.Errorf("Get of an unknown identifier returned %v, want ErrNoMachine", err)
	}
	// A second machine with the same key is a second record with another identifier.
	other := e.create(t, 0x11, "box2", false)
	if other.ID == m.ID {
		t.Errorf("two machines share the identifier %s", m.ID)
	}

	// A write that fails after CreateWaiting leaves no row and no feed row.
	head := e.feedHead(t)
	boom := errors.New("the test fails the write")
	var doomed string
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		created, err := e.reg.CreateWaiting(ctx, tx, machines.NewMachine{
			PublicKey: bytes.Repeat([]byte{0x22}, 32), HostName: "doomed", TokenID: "token-doomed",
		})
		if err != nil {
			return err
		}
		doomed = created.ID
		if !uuidV7.MatchString(doomed) {
			t.Errorf("CreateWaiting returned the id %q inside the doomed write", doomed)
		}
		return boom
	})
	if err == nil {
		t.Fatal("the doomed write returned no error")
	}
	if _, err := e.get(t, doomed); !errors.Is(err, machines.ErrNoMachine) {
		t.Errorf("Get of the rolled back machine returned %v, want ErrNoMachine", err)
	}
	if n := storetest.Scalar[int64](t, e.obs, "SELECT count(*) FROM machines"); n != 2 {
		t.Errorf("the table holds %d machines after a rolled back creation, want 2", n)
	}
	if got := e.feedHead(t); got != head {
		t.Errorf("the feed head moved from %d to %d on a rolled back write", head, got)
	}
}

func TestCreateWaitingListOrder(t *testing.T) {
	e := newRegEnv(t)
	a := e.create(t, 1, "a", false)
	b := e.create(t, 2, "b", false)
	c := e.create(t, 3, "c", false)
	d := e.create(t, 4, "d", false)

	// The order is the time of joining, then the identifier, and not the order of the identifiers.
	same := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	later := same.Add(time.Hour)
	earlier := same.Add(-time.Hour)
	for id, at := range map[string]time.Time{a.ID: later, b.ID: same, c.ID: same, d.ID: earlier} {
		storetest.Exec(t, e.obs, "UPDATE machines SET joined_at = $1 WHERE id = $2", at, id)
	}
	tie := []string{b.ID, c.ID}
	if tie[0] > tie[1] {
		tie[0], tie[1] = tie[1], tie[0]
	}
	want := []string{d.ID, tie[0], tie[1], a.ID}

	all := e.list(t)
	var got []string
	for _, m := range all {
		got = append(got, m.ID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("List returned the order %v, want %v", got, want)
	}
}

func TestApproveRenameAndNames(t *testing.T) {
	e := newRegEnv(t)
	m1 := e.create(t, 1, "box", false)
	m2 := e.create(t, 2, "other", false)

	head := e.feedHead(t)
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.SetApproved(ctx, tx, m1.ID, "Box")
	})
	if err != nil {
		t.Fatalf("SetApproved failed: %v", err)
	}
	got := e.mustGet(t, m1.ID)
	if got.Approval != "approved" || got.Name != "Box" || got.StateRevision != 2 || got.ApprovedAt == nil {
		t.Fatalf("after the approval the record is %+v, want approved, named Box, revision 2 and an approval time", got)
	}
	if moved := e.feedSince(t, int(head)); len(moved) != 1 || moved[0] != (regFeedEntry{"machines", "", m1.ID, "saved"}) {
		t.Errorf("the approval wrote the feed entries %+v, want one saved entry of the machine", moved)
	}

	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.SetApproved(ctx, tx, m1.ID, "Again")
	})
	if !errors.Is(err, machines.ErrNotWaiting) {
		t.Errorf("a second approval returned %v, want ErrNotWaiting", err)
	}
	if got := e.mustGet(t, m1.ID); got.Name != "Box" || got.StateRevision != 2 {
		t.Errorf("a refused approval changed the record to %+v", got)
	}

	// The same name in other letter case is taken, and the machine keeps waiting.
	head = e.feedHead(t)
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.SetApproved(ctx, tx, m2.ID, "bOX")
	})
	if !errors.Is(err, machines.ErrNameTaken) {
		t.Errorf("approving as a taken name returned %v, want ErrNameTaken", err)
	}
	if got := e.mustGet(t, m2.ID); got.Approval != "waiting" || got.Name != "" || got.StateRevision != 1 {
		t.Errorf("a refused approval changed the record to %+v, want it still waiting", got)
	}
	if got := e.feedHead(t); got != head {
		t.Errorf("a refused approval moved the feed head from %d to %d", head, got)
	}

	// A waiting host name that equals an approved name is no conflict.
	m3 := e.create(t, 3, "Box", false)
	if m3.Approval != "waiting" {
		t.Errorf("a machine with the host name of an approved machine is %q, want waiting", m3.Approval)
	}

	// Renames.
	head = e.feedHead(t)
	changed := true
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		changed, err = e.reg.SetName(ctx, tx, m1.ID, "Box")
		return err
	})
	if err != nil || changed {
		t.Errorf("SetName to the same name returned %v and %v, want false and no error", changed, err)
	}
	if got := e.mustGet(t, m1.ID); got.StateRevision != 2 {
		t.Errorf("a rename to the same name left the revision at %d, want 2", got.StateRevision)
	}
	if got := e.feedHead(t); got != head {
		t.Errorf("a rename to the same name moved the feed head from %d to %d", head, got)
	}

	changed = false
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		changed, err = e.reg.SetName(ctx, tx, m1.ID, "Crate")
		return err
	})
	if err != nil || !changed {
		t.Errorf("SetName to a new name returned %v and %v, want true and no error", changed, err)
	}
	if got := e.mustGet(t, m1.ID); got.Name != "Crate" || got.StateRevision != 3 || got.Approval != "approved" {
		t.Errorf("after the rename the record is %+v, want name Crate, revision 3, approved", got)
	}
	if moved := e.feedSince(t, int(head)); len(moved) != 1 || moved[0] != (regFeedEntry{"machines", "", m1.ID, "saved"}) {
		t.Errorf("the rename wrote the feed entries %+v, want one saved entry of the machine", moved)
	}

	// Only the letter case changes: another name, so a change, and no conflict with the row itself.
	changed = false
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		changed, err = e.reg.SetName(ctx, tx, m1.ID, "CRATE")
		return err
	})
	if err != nil || !changed {
		t.Errorf("SetName that changes only the letter case returned %v and %v, want true and no error", changed, err)
	}
	if got := e.mustGet(t, m1.ID); got.Name != "CRATE" || got.StateRevision != 4 {
		t.Errorf("after the change of case the record is %+v, want name CRATE and revision 4", got)
	}

	// A rename to the name of another approved machine is refused.
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.SetApproved(ctx, tx, m2.ID, "Other")
	})
	if err != nil {
		t.Fatalf("approving the second machine failed: %v", err)
	}
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		_, err := e.reg.SetName(ctx, tx, m1.ID, "OTHER")
		return err
	})
	if !errors.Is(err, machines.ErrNameTaken) {
		t.Errorf("renaming to a taken name returned %v, want ErrNameTaken", err)
	}
	if got := e.mustGet(t, m1.ID); got.Name != "CRATE" || got.StateRevision != 4 {
		t.Errorf("a refused rename changed the record to %+v", got)
	}

	t.Run("NormalizeName", func(t *testing.T) {
		accepted := map[string]string{
			" Box ":                              "Box",
			"Box":                                "Box",
			"a b":                                "a b",
			strings.Repeat("a", 64):              strings.Repeat("a", 64),
			strings.Repeat("é", 32):              strings.Repeat("é", 32),
			"  " + strings.Repeat("b", 64) + " ": strings.Repeat("b", 64),
		}
		for in, want := range accepted {
			got, err := machines.NormalizeName(in)
			if err != nil || got != want {
				t.Errorf("NormalizeName(%q) = %q, %v, want %q and no error", in, got, err, want)
			}
		}
		refused := map[string]string{
			"the empty string":           "",
			"only spaces":                "   ",
			"65 bytes":                   strings.Repeat("a", 65),
			"33 two byte characters":     strings.Repeat("é", 33),
			"a tab inside":               "a\tb",
			"a line feed inside":         "a\nb",
			"a zero byte inside":         "a\x00b",
			"a delete character":         "a\x7fb",
			"invalid UTF-8":              "a\xffb",
			"invalid UTF-8 after spaces": " \xc3 ",
		}
		for what, in := range refused {
			got, err := machines.NormalizeName(in)
			if !errors.Is(err, machines.ErrInvalidName) {
				t.Errorf("NormalizeName of %s returned %q and %v, want ErrInvalidName", what, got, err)
			}
		}
	})
}

func TestDeleteWaitingSparesApproved(t *testing.T) {
	e := newRegEnv(t)
	m := e.create(t, 1, "box", false)

	entered := make(chan struct{})
	release, releaseGate := storetest.Signal(t)
	approver := storetest.Start(t, func() error {
		return e.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if err := e.reg.SetApproved(ctx, tx, m.ID, "Box"); err != nil {
				return err
			}
			close(entered)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	})
	storetest.ReceiveFrom(t, approver, "the approval holding its transaction", entered)

	type deleted struct {
		ok  bool
		err error
	}
	deleter := storetest.Start(t, func() deleted {
		var out deleted
		out.err = e.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			var err error
			out.ok, err = e.reg.DeleteWaiting(ctx, tx, m.ID)
			return err
		})
		return out
	})
	// The deletion must wait for the approval's lock on the row.
	select {
	case <-deleter.Done():
		releaseGate()
		t.Fatalf("DeleteWaiting returned %+v while the approval of the machine was uncommitted, want it to wait for the row", deleter.Result())
	case <-time.After(settle):
	}
	storetest.Poll(t, "the deletion waiting for the approval's row lock", func() bool {
		return storetest.Scalar[bool](t, e.obs,
			"SELECT EXISTS (SELECT FROM pg_locks WHERE NOT granted AND locktype = 'transactionid')")
	})
	releaseGate()
	if err := approver.Wait(t); err != nil {
		t.Fatalf("the approval failed: %v", err)
	}
	got := deleter.Wait(t)
	if got.err != nil || got.ok {
		t.Errorf("DeleteWaiting after the approval committed returned %v and %v, want false and no error", got.ok, got.err)
	}
	if kept := e.mustGet(t, m.ID); kept.Approval != "approved" || kept.Name != "Box" {
		t.Errorf("the machine is %+v after the deletion, want it kept and approved", kept)
	}
	for _, entry := range e.feed(t) {
		if entry.Change == "removed" {
			t.Errorf("the feed holds %+v, want no removal of the approved machine", entry)
		}
	}

	// A machine that still waits is deleted and recorded.
	waiting := e.create(t, 2, "waiting", false)
	head := int(e.feedHead(t))
	var ok bool
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		ok, err = e.reg.DeleteWaiting(ctx, tx, waiting.ID)
		return err
	})
	if err != nil || !ok {
		t.Fatalf("DeleteWaiting of a waiting machine returned %v and %v, want true and no error", ok, err)
	}
	if _, err := e.get(t, waiting.ID); !errors.Is(err, machines.ErrNoMachine) {
		t.Errorf("Get of the deleted machine returned %v, want ErrNoMachine", err)
	}
	if moved := e.feedSince(t, head); len(moved) != 1 || moved[0] != (regFeedEntry{"machines", "", waiting.ID, "removed"}) {
		t.Errorf("the deletion wrote the feed entries %+v, want one removed entry", moved)
	}

	// An unknown machine is not deleted, and nothing is recorded.
	head = int(e.feedHead(t))
	ok = true
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		ok, err = e.reg.DeleteWaiting(ctx, tx, randomID(t))
		return err
	})
	if err != nil || ok {
		t.Errorf("DeleteWaiting of an unknown machine returned %v and %v, want false and no error", ok, err)
	}
	if got := int(e.feedHead(t)); got != head {
		t.Errorf("the feed head moved from %d to %d", head, got)
	}
}

func TestDeleteWaitingAndDeleteRecordRemovals(t *testing.T) {
	e := newRegEnv(t)
	m := e.create(t, 1, "box", true)
	if err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.SetApproved(ctx, tx, m.ID, "Box")
	}); err != nil {
		t.Fatalf("SetApproved failed: %v", err)
	}
	if e.mustGet(t, m.ID).Approval != "approved" {
		t.Fatal("the machine is not approved")
	}

	head := int(e.feedHead(t))
	if err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.Delete(ctx, tx, m.ID)
	}); err != nil {
		t.Fatalf("Delete of an approved machine failed: %v", err)
	}
	if _, err := e.get(t, m.ID); !errors.Is(err, machines.ErrNoMachine) {
		t.Errorf("Get of the deleted machine returned %v, want ErrNoMachine", err)
	}
	if moved := e.feedSince(t, head); len(moved) != 1 || moved[0] != (regFeedEntry{"machines", "", m.ID, "removed"}) {
		t.Errorf("Delete wrote the feed entries %+v, want one removed entry", moved)
	}

	head = int(e.feedHead(t))
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.Delete(ctx, tx, m.ID)
	})
	if !errors.Is(err, machines.ErrNoMachine) {
		t.Errorf("a second Delete returned %v, want ErrNoMachine", err)
	}
	if got := int(e.feedHead(t)); got != head {
		t.Errorf("a refused Delete moved the feed head from %d to %d", head, got)
	}
}
