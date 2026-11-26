package machines_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type projectionLink struct {
	*link.State
	load   func(context.Context, machines.Querier, []string) (map[string]machines.Presence, error)
	forget func(context.Context, *store.Tx, string) error
}

func (p *projectionLink) Presence(ctx context.Context, q machines.Querier, ids []string) (map[string]machines.Presence, error) {
	if p.load != nil {
		return p.load(ctx, q, ids)
	}
	return p.State.Presence(ctx, q, ids)
}

func (p *projectionLink) Forget(ctx context.Context, tx *store.Tx, id string) error {
	if p.forget != nil {
		return p.forget(ctx, tx, id)
	}
	return p.State.Forget(ctx, tx, id)
}

type streamRecordsFixture struct {
	*recordsFixture
	obs      *pgx.Conn
	hub      *link.Hub
	presence *projectionLink
}

func newStreamRecordsFixture(t *testing.T) *streamRecordsFixture {
	t.Helper()
	cfg := storetest.NewDatabase(t, "public")
	obs, err := pgx.ConnectConfig(t.Context(), cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The independent projection observer could not connect.")
	}
	t.Cleanup(func() { _ = obs.Close(context.Background()) })
	s, err := store.Open(t.Context(), cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The projection store could not open.")
	}
	t.Cleanup(s.Close)
	if s.Migrate(t.Context(), machines.Migrations(), auth.Migrations(), link.Migrations()) != nil {
		t.Fatal("The projection database could not migrate.")
	}
	reg := machines.NewRegistry()
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	tokens := machines.NewTokens(s, reg, machines.Issuer{PublicURL: recordsIssuerURL, ServerKey: key})
	password, err := auth.NewPassword("projection fixture password")
	if err != nil {
		t.Fatal("The fixture password could not be initialized.")
	}
	sessions := auth.NewSessions(s, password)
	cookie, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatal("The fixture session could not be issued.")
	}
	origins, err := auth.NewOriginPolicy(recordsIssuerURL, nil)
	if err != nil {
		t.Fatal("The fixture origin is invalid.")
	}
	hub := link.NewHub()
	presence := &projectionLink{State: link.NewState(s, reg, hub)}
	base := machines.NewService(s, sessions, reg, tokens)
	// This assertion keeps the external test package buildable before the
	// additive WithLink method exists on the frozen Service declaration.
	service, ok := any(base).(interface {
		WithLink(machines.Link) *machines.Service
	})
	if !ok {
		t.Fatal("The Machines service did not expose its planned link loader.")
	}
	router := api.NewHandler(api.Options{Origins: origins, Sessions: sessions, Clients: service.WithLink(presence), Link: link.NewService(s, tokens, key)})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := server.Client()
	return &streamRecordsFixture{
		recordsFixture: &recordsFixture{
			regEnv: &regEnv{s: s, reg: reg}, tokens: tokens, http: client, url: server.URL,
			clients: lgtmv1connect.NewClientServiceClient(client, server.URL),
			link:    linkv1connect.NewLinkServiceClient(client, server.URL),
			cookie:  &http.Cookie{Name: auth.CookieName, Value: cookie},
		}, obs: obs, hub: hub, presence: presence,
	}
}

func (f *streamRecordsFixture) issue(t *testing.T, hostsServer bool) *linkv1.JoinToken {
	t.Helper()
	issued, err := f.tokens.Create(t.Context(), machines.TokenOptions{HostsServer: hostsServer})
	if err != nil {
		t.Fatal("The projection fixture could not issue a token.")
	}
	decoded, err := machines.DecodeToken(issued.Text)
	if err != nil {
		t.Fatal("The projection fixture could not decode its token.")
	}
	return decoded
}

func (f *streamRecordsFixture) join(t *testing.T, token *linkv1.JoinToken) string {
	t.Helper()
	public, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("The projection fixture could not generate a machine key.")
	}
	reply, err := f.link.Join(t.Context(), connect.NewRequest(&linkv1.JoinRequest{
		TokenId: token.TokenId, TokenSecret: token.Secret, MachinePublicKey: public, HostName: "projection machine",
	}))
	if err != nil || reply.Msg.GetMachineId() == "" {
		t.Fatal("The projection fixture could not join its machine.")
	}
	return reply.Msg.GetMachineId()
}

func (f *streamRecordsFixture) installHandle(t *testing.T, machine, id string, generation int64) context.CancelCauseFunc {
	t.Helper()
	ctx, cancel := context.WithCancelCause(t.Context())
	h := f.hub.Track(machine, id, ctx, cancel)
	t.Cleanup(func() { cancel(nil); f.hub.Unregister(h) })
	if f.hub.Admit(h, func() (int64, bool, error) { return generation, false, nil }) != nil || f.hub.BeginWelcome(h) != nil {
		t.Fatal("The fixture could not publish its live handle.")
	}
	f.hub.FinishWelcome(h)
	return cancel
}

func (f *streamRecordsFixture) admit(t *testing.T, machine string, gen int64) (link.Stream, context.CancelCauseFunc) {
	t.Helper()
	id, err := store.NewID()
	if err != nil {
		t.Fatal("The presence fixture could not allocate a stream identifier.")
	}
	cancel := f.installHandle(t, machine, id, gen)
	result, err := f.presence.BecomeCurrent(t.Context(), machine, id, "10.2.3.4")
	if err != nil || result.Generation != gen || result.ID != id {
		t.Fatal("The fixture did not commit its stream admission.")
	}
	return result, cancel
}

func (f *streamRecordsFixture) approve(t *testing.T, id, key string) (*webv1.ApproveClientResponse, error) {
	t.Helper()
	r, err := f.clients.ApproveClient(t.Context(), recordRequest(f.recordsFixture, &webv1.ApproveClientRequest{Id: id, Name: "Fixture machine"}, key))
	if err != nil {
		return nil, err
	}
	return r.Msg, nil
}

func (f *streamRecordsFixture) head(t *testing.T) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, f.obs, "SELECT COALESCE(max(position), 0) FROM feed")
}

func (f *streamRecordsFixture) snapshot(t *testing.T) *webv1.Client {
	t.Helper()
	stream := f.watch(t)
	defer stream.Close()
	rows := stream.Msg().GetAll().GetClients()
	if len(rows) != 1 {
		t.Fatalf("The Machines snapshot had %d rows, want one.", len(rows))
	}
	return rows[0]
}

func requireProjectedState(t *testing.T, c *webv1.Client, state webv1.ClientState) {
	t.Helper()
	if c == nil || c.State != state {
		t.Fatalf("The Machines projection showed %s, want %s.", c.GetState(), state)
	}
	if state != webv1.ClientState_CLIENT_STATE_ONLINE && (c.CpuPercent != nil || c.MemoryPercent != nil) {
		t.Fatal("The Machines projection showed load outside Online.")
	}
}

func (f *streamRecordsFixture) apply(t *testing.T, id, stream string, n uint64, facts bool) {
	t.Helper()
	event := &linkv1.Event{Number: n}
	if facts {
		event.Report = &linkv1.Event_Facts{Facts: &linkv1.Facts{Os: "Fixture OS", Cores: 4}}
	} else {
		event.Report = &linkv1.Event_HarnessesFound{HarnessesFound: &linkv1.HarnessesFound{}}
	}
	result, err := f.presence.ApplyEvent(t.Context(), id, stream, event)
	if err != nil || result.Mark != n || !result.Applied {
		t.Fatal("The projection fixture could not apply its event.")
	}
}

func TestProjectStreamPresence(t *testing.T) {
	f := newStreamRecordsFixture(t)
	id := f.join(t, f.issue(t, false))
	row, cancel := f.admit(t, id, 1)
	if f.presence.Heartbeat(t.Context(), id, row.ID, &linkv1.Heartbeat{CpuPercent: proto.Uint32(7)}) != nil {
		t.Fatal("The fixture could not save waiting load.")
	}
	c := f.snapshot(t)
	requireProjectedState(t, c, webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL)
	if c.GetAddress() != "10.2.3.4" || c.GetConnectedAt() == nil || c.GetLastSeen() == nil {
		t.Fatal("Waiting did not project the current address and contact times.")
	}
	if _, err := f.approve(t, id, "project-approval"); err != nil {
		t.Fatal("The fixture could not approve the waiting machine.")
	}
	c = f.snapshot(t)
	requireProjectedState(t, c, webv1.ClientState_CLIENT_STATE_JOINING)
	f.apply(t, id, row.ID, 1, true)
	f.apply(t, id, row.ID, 2, false)
	if f.presence.Heartbeat(t.Context(), id, row.ID, &linkv1.Heartbeat{CpuPercent: proto.Uint32(0)}) != nil {
		t.Fatal("The fixture could not save present zero load.")
	}
	c = f.snapshot(t)
	requireProjectedState(t, c, webv1.ClientState_CLIENT_STATE_ONLINE)
	if c.CpuPercent == nil || *c.CpuPercent != 0 || c.MemoryPercent != nil || c.Os != "Fixture OS" || !c.HarnessesScanned {
		t.Fatal("The online projection lost present zero, absent memory or committed report fields.")
	}
	last := c.GetLastSeen().AsTime()
	cancel(nil)
	c = f.snapshot(t)
	requireProjectedState(t, c, webv1.ClientState_CLIENT_STATE_OFFLINE)
	if c.GetConnectedAt() != nil {
		t.Fatal("A canceled handle with a retained connection row still looked connected.")
	}
	if deleted, err := f.presence.Disconnect(t.Context(), id, row.ID); err != nil || !deleted {
		t.Fatal("The fixture could not clear the closed stream.")
	}
	c = f.snapshot(t)
	requireProjectedState(t, c, webv1.ClientState_CLIENT_STATE_OFFLINE)
	if c.GetConnectedAt() != nil || c.Address != "10.2.3.4" || !c.GetLastSeen().AsTime().Equal(last) {
		t.Fatal("Offline did not keep the last saved address and contact time.")
	}
}

func TestWatchClientsStreamPresence(t *testing.T) {
	t.Run("watch and snapshot agree", func(t *testing.T) {
		f := newStreamRecordsFixture(t)
		id := f.join(t, f.issue(t, false))
		if _, err := f.approve(t, id, "watch-approval"); err != nil {
			t.Fatal("The watch fixture could not approve its machine.")
		}
		w := f.watch(t)
		requireProjectedState(t, w.Msg().GetAll().GetClients()[0], webv1.ClientState_CLIENT_STATE_OFFLINE)
		check := func(want webv1.ClientState) {
			t.Helper()
			if !w.Receive() {
				t.Fatal("The watch ended before its next saved record.")
			}
			c := w.Msg().GetSaved()
			requireProjectedState(t, c, want)
			if c.Id != id || !proto.Equal(c, f.snapshot(t)) {
				t.Fatal("The live saved frame disagreed with a fresh snapshot.")
			}
		}
		a, _ := f.admit(t, id, 1)
		check(webv1.ClientState_CLIENT_STATE_JOINING)
		f.apply(t, id, a.ID, 1, true)
		check(webv1.ClientState_CLIENT_STATE_JOINING)
		f.apply(t, id, a.ID, 2, false)
		check(webv1.ClientState_CLIENT_STATE_ONLINE)
		b, _ := f.admit(t, id, 2)
		check(webv1.ClientState_CLIENT_STATE_JOINING)
		if deleted, err := f.presence.Disconnect(t.Context(), id, b.ID); err != nil || !deleted {
			t.Fatal("The watch fixture could not disconnect its replacement.")
		}
		check(webv1.ClientState_CLIENT_STATE_OFFLINE)
	})
	t.Run("presence uses the machine snapshot", func(t *testing.T) {
		f := newStreamRecordsFixture(t)
		id := f.join(t, f.issue(t, false))
		if _, err := f.approve(t, id, "snapshot-approval"); err != nil {
			t.Fatal("The snapshot fixture could not approve its machine.")
		}
		a, _ := f.admit(t, id, 1)
		f.apply(t, id, a.ID, 1, true)
		f.apply(t, id, a.ID, 2, false)
		atLoader := make(chan struct{}, 1)
		replacement, proceed := storetest.Signal(t)
		release, finish := storetest.Signal(t)
		defer proceed()
		defer finish()
		read := make(chan string, 1)
		var held atomic.Bool
		f.presence.load = func(ctx context.Context, q machines.Querier, ids []string) (map[string]machines.Presence, error) {
			if !held.CompareAndSwap(false, true) {
				return f.presence.State.Presence(ctx, q, ids)
			}
			atLoader <- struct{}{}
			select {
			case <-replacement:
			case <-ctx.Done():
				return nil, context.Cause(ctx)
			}
			rows, err := f.presence.State.Presence(ctx, q, ids)
			read <- rows[id].StreamID
			select {
			case <-release:
			case <-ctx.Done():
				return nil, context.Cause(ctx)
			}
			return rows, err
		}
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		watch := storetest.Start(t, func() *connect.ServerStreamForClient[webv1.WatchClientsResponse] {
			w, err := f.clients.WatchClients(ctx, recordRequest(f.recordsFixture, &webv1.WatchClientsRequest{}, ""))
			if err != nil {
				return nil
			}
			return w
		})
		storetest.Receive(t, "the watch holding its presence read after loading the machine", atLoader)
		bid, err := store.NewID()
		if err != nil {
			t.Fatal("The replacement stream identifier could not be allocated.")
		}
		b, err := f.presence.BecomeCurrent(t.Context(), id, bid, "10.2.3.4")
		if err != nil || b.Generation != 2 {
			t.Fatal("The staged replacement did not commit.")
		}
		proceed()
		if storetest.Receive(t, "the presence read in the original snapshot", read) != a.ID {
			t.Fatal("The presence loader used a newer database snapshot than the machine query.")
		}
		f.installHandle(t, id, b.ID, 2)
		finish()
		w := watch.Wait(t)
		if w == nil {
			t.Fatal("The staged watch did not open.")
		}
		defer w.Close()
		if !w.Receive() || len(w.Msg().GetAll().GetClients()) != 1 {
			t.Fatal("The staged watch did not receive its original snapshot.")
		}
		requireProjectedState(t, w.Msg().GetAll().GetClients()[0], webv1.ClientState_CLIENT_STATE_ONLINE)
		if !w.Receive() {
			t.Fatal("The staged watch did not observe the replacement.")
		}
		requireProjectedState(t, w.Msg().GetSaved(), webv1.ClientState_CLIENT_STATE_JOINING)
		if !proto.Equal(w.Msg().GetSaved(), f.snapshot(t)) {
			t.Fatal("The replacement save disagreed with a fresh snapshot.")
		}
	})
	t.Run("approval loads in the keyed write", func(t *testing.T) {
		f := newStreamRecordsFixture(t)
		id := f.join(t, f.issue(t, false))
		a, _ := f.admit(t, id, 1)
		f.apply(t, id, a.ID, 1, true)
		head := f.head(t)
		type observation struct {
			inWrite  bool
			revision int64
		}
		observed := make(chan observation, 3)
		atRead, continueRead := storetest.Signal(t)
		defer continueRead()
		var fail atomic.Bool
		fail.Store(true)
		f.presence.load = func(ctx context.Context, q machines.Querier, ids []string) (map[string]machines.Presence, error) {
			_, isWrite := q.(*store.Tx)
			var revision int64
			if err := q.QueryRow(ctx, "SELECT state_revision FROM machines WHERE id = $1", id).Scan(&revision); err != nil {
				return nil, err
			}
			observed <- observation{isWrite, revision}
			if fail.Load() {
				select {
				case <-atRead:
				case <-ctx.Done():
					return nil, context.Cause(ctx)
				}
				return nil, errors.New("The fixture rejected the approval presence read.")
			}
			return f.presence.State.Presence(ctx, q, ids)
		}
		const key = "stream-keyed-approval"
		request := recordRequest(f.recordsFixture, &webv1.ApproveClientRequest{Id: id, Name: "Fixture machine"}, key)
		call := storetest.Start(t, func() error { _, err := f.clients.ApproveClient(t.Context(), request); return err })
		o := storetest.ReceiveFrom(t, call, "the approval presence read", observed)
		if !o.inWrite || o.revision != 2 || storetest.Scalar[int64](t, f.obs, "SELECT state_revision FROM machines WHERE id = $1", id) != 1 {
			t.Fatal("The presence loader did not see the uncommitted new revision while the observer saw the original.")
		}
		continueRead()
		if connect.CodeOf(call.Wait(t)) != connect.CodeUnavailable || f.head(t) != head ||
			storetest.Scalar[int64](t, f.obs, "SELECT state_revision FROM machines WHERE id = $1", id) != 1 ||
			storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM receipts WHERE request_key = $1)", key) {
			t.Fatal("A failed in-write loader did not roll back approval, feed and receipt.")
		}
		fail.Store(false)
		first, err := f.approve(t, id, key)
		if err != nil {
			t.Fatal("The approval could not be retried after its rollback.")
		}
		o = storetest.Receive(t, "the successful approval reading its new revision", observed)
		if !o.inWrite || o.revision != 2 {
			t.Fatal("Successful approval bypassed its write transaction.")
		}
		requireProjectedState(t, first.GetClient(), webv1.ClientState_CLIENT_STATE_JOINING)
		if first.GetClient().Os != "Fixture OS" {
			t.Fatal("Approval omitted the waiting machine's applied facts.")
		}
		f.apply(t, id, a.ID, 2, false)
		f.apply(t, id, a.ID, 3, true)
		beforeReplay := f.head(t)
		replay, err := f.approve(t, id, key)
		firstBytes, firstErr := proto.MarshalOptions{Deterministic: true}.Marshal(first)
		replayBytes, replayErr := proto.MarshalOptions{Deterministic: true}.Marshal(replay)
		if err != nil || firstErr != nil || replayErr != nil || string(firstBytes) != string(replayBytes) || f.head(t) != beforeReplay {
			t.Fatal("Approval replay did not return the original reply bytes without another saved record.")
		}
		select {
		case <-observed:
			t.Fatal("Approval replay reloaded presence.")
		default:
		}
	})
}
