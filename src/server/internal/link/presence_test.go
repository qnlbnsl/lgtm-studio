package link

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const presenceAddress = "10.2.3.4"

var streamVersion7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type presenceFixture struct {
	s     *store.Store
	r     *machines.Registry
	obs   *pgx.Conn
	hub   *Hub
	state *State
}

func newPresenceFixture(t *testing.T, configure func(*pgxpool.Config)) *presenceFixture {
	t.Helper()
	conf := storetest.NewDatabase(t, "public")
	// Open the independent observer before installing any application fault.
	obs, err := pgx.ConnectConfig(t.Context(), conf.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The database observer could not connect.")
	}
	t.Cleanup(func() { _ = obs.Close(context.Background()) })
	if configure != nil {
		configure(conf)
	}
	s, err := store.Open(t.Context(), conf, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The test store did not open.")
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(t.Context(), machines.Migrations(), Migrations()); err != nil {
		t.Fatal("The machine and link migrations did not apply.")
	}
	r := machines.NewRegistry()
	hub := NewHub()
	return &presenceFixture{s: s, r: r, obs: obs, hub: hub, state: NewState(s, r, hub)}
}

func (f *presenceFixture) machine(t *testing.T, key []byte, name string) machines.Machine {
	t.Helper()
	var m machines.Machine
	err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		var err error
		m, err = f.r.CreateWaiting(ctx, tx, machines.NewMachine{PublicKey: key, HostName: name, TokenID: "fixture-" + name})
		return err
	})
	if err != nil {
		t.Fatal("The waiting fixture machine could not be created.")
	}
	return m
}

func (f *presenceFixture) approve(t *testing.T, id string) {
	t.Helper()
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.r.SetApproved(ctx, tx, id, "Approved machine")
	}); err != nil {
		t.Fatal("The fixture machine could not be approved.")
	}
}

func fixtureStreamID(t *testing.T) string {
	t.Helper()
	id, err := store.NewID()
	if err != nil {
		t.Fatal("The fixture stream identifier could not be allocated.")
	}
	return id
}

func (f *presenceFixture) admit(t *testing.T, machineID string, generation int64) Stream {
	t.Helper()
	id := fixtureStreamID(t)
	result, err := f.state.BecomeCurrent(t.Context(), machineID, id, presenceAddress)
	if err != nil || result.ID != id || result.Generation != generation || result.Standing.ID != machineID {
		t.Fatalf("BecomeCurrent did not return the requested stream and generation %d.", generation)
	}
	return result
}

func (f *presenceFixture) current(t *testing.T, id string) machines.Presence {
	t.Helper()
	var got map[string]machines.Presence
	err := f.s.Read(t.Context(), func(ctx context.Context, tx *store.ReadTx) error {
		var err error
		got, err = f.state.Presence(ctx, tx, []string{id})
		return err
	})
	p, ok := got[id]
	if err != nil || !ok || len(got) != 1 {
		t.Fatal("Presence did not return the requested machine's connection.")
	}
	return p
}

func (f *presenceFixture) head(t *testing.T) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, f.obs, "SELECT COALESCE(max(position), 0) FROM feed")
}

func (f *presenceFixture) requireSaved(t *testing.T, after int64, ids ...string) {
	t.Helper()
	want := make([]string, 0, len(ids))
	for _, id := range ids {
		want = append(want, "machines//"+id+"/saved")
	}
	got := storetest.Rows[string](t, f.obs, `SELECT topic || '/' || scope || '/' || key || '/' || change
		FROM feed WHERE position > $1 ORDER BY position`, after)
	if !slices.Equal(got, want) {
		t.Fatalf("The feed after the barrier contained %d references, want %d saved references for the expected machines in order.", len(got), len(want))
	}
}

func validConnectionToken(token string) bool {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	return err == nil && len(token) == 43 && len(decoded) == 32
}

func TestBecomeCurrent(t *testing.T) {
	f := newPresenceFixture(t, nil)
	if !storetest.Scalar[bool](t, f.obs, `SELECT to_regclass('link_machines') IS NOT NULL
		AND to_regclass('link_connections') IS NOT NULL
		AND EXISTS (SELECT FROM schema_migrations WHERE part = 'link' AND version = 3)`) {
		t.Error("Link migration version 3 or the Part B tables were missing before admission.")
	}
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "station")
	before := f.head(t)
	start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	a := f.admit(t, m.ID, 1)
	end := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	if a.Standing.Approval != machines.ApprovalWaiting || a.Standing.StateRevision != 1 ||
		a.ConnectionToken != "" || a.Mark != 0 || a.ServerTime.Before(start) || a.ServerTime.After(end) {
		t.Error("Waiting admission returned the wrong standing, mark, token policy or database time.")
	}
	p := f.current(t, m.ID)
	if p.StreamID != a.ID || p.Generation != 1 || p.Address != presenceAddress ||
		p.ConnectedAt.Before(start) || p.ConnectedAt.After(end) ||
		p.LastHeardAt.Before(start) || p.LastHeardAt.After(end) || p.Held {
		t.Error("The database presence did not reflect the first admission and process liveness.")
	}
	approval, err := f.state.CheckCurrent(t.Context(), m.ID, a.ID)
	if err != nil || approval != machines.ApprovalWaiting {
		t.Error("CheckCurrent did not identify the current waiting stream.")
	}
	f.requireSaved(t, before, m.ID)
	storetest.Exec(t, f.obs, "UPDATE link_machines SET event_mark = 42 WHERE machine_id = $1", m.ID)
	storetest.Exec(t, f.obs, `UPDATE link_connections SET facts_received = true, harnesses_received = true,
		cpu_percent = 9, memory_percent = 20 WHERE machine_id = $1`, m.ID)
	f.approve(t, m.ID)
	before = f.head(t)
	b := f.admit(t, m.ID, 2)
	p = f.current(t, m.ID)
	if b.Standing.Approval != machines.ApprovalApproved || b.Standing.Name != "Approved machine" ||
		b.Standing.StateRevision != 2 || b.Mark != 42 || !validConnectionToken(b.ConnectionToken) {
		t.Error("The replacement did not return the admitted standing, saved mark and approved token.")
	}
	if p.StreamID != b.ID || p.Generation != 2 || p.CPUPercent != nil || p.MemoryPercent != nil ||
		p.FactsReceived || p.HarnessesReceived {
		t.Error("The replacement kept load or readiness from the older stream.")
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	handle := f.hub.Track(m.ID, b.ID, ctx, cancel)
	defer f.hub.Unregister(handle)
	publishHubStream(t, f.hub, handle, 2)
	finishHubWelcome(t, f.hub, handle)
	if !f.current(t, m.ID).Held {
		t.Error("Presence did not report an exact live handle.")
	}
	cancel(errors.New("The test ended this handle."))
	if f.current(t, m.ID).Held {
		t.Error("A retained database row kept a canceled handle live.")
	}
	c := f.admit(t, m.ID, 3)
	if c.Mark != 42 || !validConnectionToken(c.ConnectionToken) || c.ConnectionToken == b.ConnectionToken {
		t.Error("The third admission changed the mark or reused the preceding approved token.")
	}
	f.requireSaved(t, before, m.ID, m.ID)
	if _, err := f.state.CheckCurrent(t.Context(), m.ID, a.ID); !errors.Is(err, ErrNotCurrent) {
		t.Error("CheckCurrent did not distinguish a displaced stream.")
	}

	unknown := fixtureStreamID(t)
	before = f.head(t)
	if _, err := f.state.BecomeCurrent(t.Context(), unknown, fixtureStreamID(t), ""); !errors.Is(err, ErrUnknownMachine) {
		t.Error("A missing machine did not return ErrUnknownMachine.")
	}
	if _, err := f.state.CheckCurrent(t.Context(), unknown, a.ID); !errors.Is(err, ErrUnknownMachine) {
		t.Error("CheckCurrent did not distinguish a missing machine.")
	}
	if storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_machines WHERE machine_id = $1", unknown) != 0 ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_connections WHERE machine_id = $1", unknown) != 0 {
		t.Error("An unknown machine acquired a mark or connection.")
	}
	f.requireSaved(t, before)
}

func TestHeartbeatAndDisconnect(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "station")
	a := f.admit(t, m.ID, 1)
	previous := time.Date(2026, 10, 2, 3, 4, 5, 0, time.UTC)
	storetest.Exec(t, f.obs, "UPDATE link_connections SET last_heard_at = $2 WHERE machine_id = $1", m.ID, previous)
	before := f.head(t)
	for _, tc := range []struct{ cpu, memory *uint32 }{
		{proto.Uint32(7), proto.Uint32(28)}, {}, {proto.Uint32(0), nil},
	} {
		start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
		err := f.state.Heartbeat(t.Context(), m.ID, a.ID, &linkv1.Heartbeat{CpuPercent: tc.cpu, MemoryPercent: tc.memory})
		end := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
		p := f.current(t, m.ID)
		if err != nil || !sameLoad(p.CPUPercent, tc.cpu) || !sameLoad(p.MemoryPercent, tc.memory) ||
			p.LastHeardAt.Before(start) || p.LastHeardAt.After(end) {
			t.Fatal("Heartbeat lost load presence or did not advance the database contact time.")
		}
		previous = p.LastHeardAt
	}
	f.requireSaved(t, before, m.ID, m.ID, m.ID)
	before = f.head(t)
	if err := f.state.Heartbeat(t.Context(), m.ID, a.ID, &linkv1.Heartbeat{MemoryPercent: proto.Uint32(101)}); !errors.Is(err, ErrInvalidMessage) {
		t.Error("Heartbeat accepted a load above 100.")
	}
	if p := f.current(t, m.ID); !p.LastHeardAt.Equal(previous) || !sameLoad(p.CPUPercent, proto.Uint32(0)) || p.MemoryPercent != nil {
		t.Error("A refused heartbeat changed the saved contact time or loads.")
	}
	f.requireSaved(t, before)
	if changed, err := f.state.Disconnect(t.Context(), m.ID, a.ID); err != nil || !changed {
		t.Fatal("Disconnect did not remove the current connection.")
	}
	if changed, err := f.state.Disconnect(t.Context(), m.ID, a.ID); err != nil || changed {
		t.Error("Disconnect did not treat an already absent connection as a no-op.")
	}
	f.requireSaved(t, before, m.ID)
	seen := storetest.Scalar[time.Time](t, f.obs, "SELECT last_seen_at FROM machines WHERE id = $1", m.ID)
	address := storetest.Scalar[string](t, f.obs, "SELECT last_address FROM machines WHERE id = $1", m.ID)
	if !seen.Equal(previous) || address != presenceAddress || f.connectionCount(t, m.ID) != 0 {
		t.Error("Disconnect did not save the last contact time and address and remove the connection.")
	}
}

func sameLoad(a, b *uint32) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (f *presenceFixture) connectionCount(t *testing.T, id string) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_connections WHERE machine_id = $1", id)
}

func TestClearConnections(t *testing.T) {
	f := newPresenceFixture(t, nil)
	lock, err := f.s.AcquireServerInstance(t.Context(), 5*time.Second)
	if err != nil {
		t.Fatal("The fixture could not take the instance lock.")
	}
	t.Cleanup(func() { _ = lock.Close(context.Background()) })
	var ids []string
	for i := range 3 {
		m := f.machine(t, bytes.Repeat([]byte{byte(i + 1)}, 32), "station")
		f.admit(t, m.ID, 1)
		f.admit(t, m.ID, 2)
		storetest.Exec(t, f.obs, "UPDATE link_machines SET event_mark = stream_generation + 40 WHERE machine_id = $1", m.ID)
		ids = append(ids, m.ID)
	}
	slices.Sort(ids)
	before := f.head(t)
	if err := f.state.ClearConnections(t.Context()); err != nil {
		t.Fatal("Clearing stale connections returned an error.")
	}
	f.requireSaved(t, before, ids...)
	for _, id := range ids {
		if f.connectionCount(t, id) != 0 ||
			storetest.Scalar[int64](t, f.obs, "SELECT stream_generation FROM link_machines WHERE machine_id = $1", id) != 2 ||
			storetest.Scalar[int64](t, f.obs, "SELECT event_mark FROM link_machines WHERE machine_id = $1", id) != 42 {
			t.Error("ClearConnections did not remove only the connection row.")
		}
	}
	if storetest.Scalar[int64](t, f.obs, "SELECT count(DISTINCT xmin::text) FROM feed WHERE position > $1", before) != 3 {
		t.Error("ClearConnections did not commit one machine per write.")
	}
	if lock.Check(t.Context()) != nil {
		t.Error("ClearConnections released the server instance lock.")
	}
}

func TestForget(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "station")
	f.admit(t, m.ID, 1)
	other := f.machine(t, bytes.Repeat([]byte{2}, 32), "other")
	f.admit(t, other.ID, 1)
	rollback := store.NewDomainError("The fixture requests a rollback.")
	for _, wantErr := range []error{rollback, nil} {
		before := f.head(t)
		removedInside := false
		err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if err := f.r.Delete(ctx, tx, m.ID); err != nil {
				return err
			}
			if err := f.state.Forget(ctx, tx, m.ID); err != nil {
				return err
			}
			err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT FROM link_machines WHERE machine_id = $1)
				AND NOT EXISTS(SELECT FROM link_connections WHERE machine_id = $1)`, m.ID).Scan(&removedInside)
			if err != nil {
				return err
			}
			return wantErr
		})
		if !errors.Is(err, wantErr) || !removedInside {
			t.Fatal("Forget did not clear both rows inside the caller's write.")
		}
		wantCount := int64(0)
		if wantErr != nil {
			wantCount = 1
			f.requireSaved(t, before)
		}
		if f.connectionCount(t, m.ID) != wantCount ||
			storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_machines WHERE machine_id = $1", m.ID) != wantCount ||
			storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM machines WHERE id = $1", m.ID) != wantCount ||
			f.connectionCount(t, other.ID) != 1 {
			t.Error("The commit or rollback did not preserve the expected three rows and the unrelated connection.")
		}
	}
}
