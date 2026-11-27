package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/app"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

type appOpenStream = connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]

func appOpenContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func appOpen(t *testing.T, client linkv1connect.LinkServiceClient) *appOpenStream {
	t.Helper()
	s := client.Open(appOpenContext(t))
	t.Cleanup(func() { _ = s.CloseResponse(); _ = s.CloseRequest() })
	return s
}

func requireAppOpenStatus(t *testing.T, s *appOpenStream, code connect.Code, sentence string) {
	t.Helper()
	frame, err := s.Receive()
	var ce *connect.Error
	if frame != nil || !errors.As(err, &ce) || ce.Code() != code || ce.Message() != sentence {
		t.Fatalf("Open did not return the fixed %s refusal. Received frame=%t, code=%s.", code, frame != nil, connect.CodeOf(err))
	}
}

func sendAppOpen(t *testing.T, s *appOpenStream, frame *linkv1.OpenRequest) {
	t.Helper()
	if err := s.Send(frame); err != nil && !errors.Is(err, io.EOF) {
		t.Fatal("The Open fixture could not send a frame.")
	}
}

func joinAppLink(t *testing.T, server *servertest.Server, clients lgtmv1connect.ClientServiceClient, name string) *machinetest.Machine {
	t.Helper()
	reply, err := clients.CreateClientToken(appOpenContext(t), connect.NewRequest(&webv1.CreateClientTokenRequest{}))
	if err != nil {
		t.Fatal("The application did not issue its fixture join token.")
	}
	m, err := machinetest.Join(appOpenContext(t), server.Link(), reply.Msg.GetToken(), name)
	if err != nil || m == nil {
		t.Fatal("The scripted machine did not join the application.")
	}
	return m
}

func appProofText(label, id string, machineNonce, serverNonce []byte) []byte {
	text := append([]byte(label), 0, byte(len(id)))
	text = append(text, id...)
	text = append(text, machineNonce...)
	return append(text, serverNonce...)
}

func appChallenge(t *testing.T, s *appOpenStream, m *machinetest.Machine, nonceByte byte) *linkv1.OpenRequest {
	t.Helper()
	nonce := bytes.Repeat([]byte{nonceByte}, 32)
	sendAppOpen(t, s, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Hello{Hello: &linkv1.Hello{
		MachineId: m.ID, LinkRevision: 2, MachineNonce: nonce,
		RunId: m.RunID, FirstEventNumber: m.FirstEventNumber,
	}}})
	frame, err := s.Receive()
	ch := frame.GetChallenge()
	if err != nil || ch == nil || len(ch.ServerNonce) != 32 ||
		!ed25519.Verify(m.ServerKey, appProofText("lgtm-link-v1 server", m.ID, nonce, ch.ServerNonce), ch.ServerSignature) {
		t.Fatalf("The unsigned caller did not receive a verifiable Challenge. Open returned %s.", connect.CodeOf(err))
	}
	signature := ed25519.Sign(m.Key, appProofText("lgtm-link-v1 machine", m.ID, nonce, ch.ServerNonce))
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{Proof: &linkv1.Proof{MachineSignature: signature}}}
}

func TestUnsignedOpenProvesServer(t *testing.T) {
	server := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	m := joinAppLink(t, server, server.Clients(t), "unsigned open")
	unsigned := &http.Client{Transport: server.HTTPClient.Transport}
	for _, tc := range []struct {
		name string
		opts []connect.ClientOption
	}{
		{"Connect", nil}, {"gRPC", []connect.ClientOption{connect.WithGRPC()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := linkv1connect.NewLinkServiceClient(unsigned, server.URL, tc.opts...)
			appChallenge(t, appOpen(t, client), m, 17)
			stream := appOpen(t, client)
			sendAppOpen(t, stream, &linkv1.OpenRequest{})
			requireAppOpenStatus(t, stream, connect.CodeInvalidArgument, "The link message broke a rule of the contract.")
		})
	}
	website := lgtmv1connect.NewClientServiceClient(unsigned, server.URL)
	watch, err := website.WatchClients(appOpenContext(t), connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err == nil {
		defer watch.Close()
		if watch.Receive() {
			t.Fatal("Unsigned WatchClients received a machine frame.")
		}
		err = watch.Err()
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("The unsigned website watch was not refused independently of Open.")
	}
}

func fixtureLinkConfig(t *testing.T, s *servertest.Server) (config.Config, *pgx.Conn) {
	t.Helper()
	env := s.Environment()
	cfg, err := config.Load(func(key string) (string, bool) { value, ok := env[key]; return value, ok })
	if err != nil {
		t.Fatal("The application fixture's settings could not be read.")
	}
	obs, err := pgx.ConnectConfig(appOpenContext(t), cfg.Postgres.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The startup database observer could not connect.")
	}
	t.Cleanup(func() { _ = obs.Close(context.Background()) })
	return cfg, obs
}

func applicationLinkStream(t *testing.T, m *machinetest.Machine, client linkv1connect.LinkServiceClient) (*appOpenStream, *linkv1.Welcome) {
	t.Helper()
	s, w, err := m.Connect(appOpenContext(t), client)
	if err != nil || s == nil || w == nil {
		t.Fatal("The running application did not complete a scripted Open.")
	}
	t.Cleanup(func() { _ = s.CloseResponse(); _ = s.CloseRequest() })
	return s, w
}

func appLinkEvent(t *testing.T, s *appOpenStream, n uint64) {
	t.Helper()
	sendAppOpen(t, s, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: &linkv1.Event{Number: n, Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{Cores: 2, Os: "Startup report"}}}}})
	reply, err := s.Receive()
	if err != nil || reply.GetEventAck() == nil || reply.GetEventAck().Through != n {
		t.Fatal("The application did not acknowledge the committed fixture report.")
	}
}

func requireLinkTables(t *testing.T, obs *pgx.Conn) {
	t.Helper()
	if !storetest.Scalar[bool](t, obs, "SELECT to_regclass('link_connections') IS NOT NULL AND to_regclass('link_machines') IS NOT NULL") {
		t.Fatal("Startup did not register and apply the link migration.")
	}
}

func seedStale(t *testing.T, obs *pgx.Conn, id string, generation, mark int64) {
	t.Helper()
	requireLinkTables(t, obs)
	stream, err := store.NewID()
	if err != nil {
		t.Fatal("The stale stream identifier could not be allocated.")
	}
	storetest.Exec(t, obs, `INSERT INTO link_machines(machine_id, event_mark, stream_generation) VALUES ($1,$2,$3)
		ON CONFLICT (machine_id) DO UPDATE SET event_mark=EXCLUDED.event_mark, stream_generation=EXCLUDED.stream_generation`, id, mark, generation)
	storetest.Exec(t, obs, `INSERT INTO link_connections(machine_id, stream_id, generation, connected_at, last_heard_at, facts_received, harnesses_received)
		VALUES ($1,$2,$3,'2000-01-01','2000-01-02',true,true)
		ON CONFLICT (machine_id) DO UPDATE SET stream_id=EXCLUDED.stream_id, generation=EXCLUDED.generation,
			connected_at=EXCLUDED.connected_at, last_heard_at=EXCLUDED.last_heard_at, facts_received=true, harnesses_received=true`, id, stream, generation)
}

func requireFirstOffline(t *testing.T, clients lgtmv1connect.ClientServiceClient, ids ...string) {
	t.Helper()
	rows := fixtureMachinesSnapshot(t, fixtureMachinesWatch(t, clients))
	if len(rows) != len(ids) {
		t.Fatalf("The first watch had %d machine records, want %d.", len(rows), len(ids))
	}
	for _, row := range rows {
		if !slices.Contains(ids, row.Id) || row.State != webv1.ClientState_CLIENT_STATE_OFFLINE || row.ConnectedAt != nil || row.LastSeen == nil {
			t.Fatal("The first watch did not show the expected retained machines offline with saved last contact.")
		}
	}
}

func approveAppLink(t *testing.T, clients lgtmv1connect.ClientServiceClient, id, name string) {
	t.Helper()
	if _, err := clients.ApproveClient(appOpenContext(t), connect.NewRequest(&webv1.ApproveClientRequest{Id: id, Name: name})); err != nil {
		t.Fatal("The application could not approve a fixture machine.")
	}
}

func TestStreamStartClearsStalePresence(t *testing.T) {
	server := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	_, obs := fixtureLinkConfig(t, server)
	clients := server.Clients(t)
	m := joinAppLink(t, server, clients, "restart stream")
	approveAppLink(t, clients, m.ID, "Restart stream")
	s, _ := applicationLinkStream(t, m, server.Link())
	appLinkEvent(t, s, 1)
	requireLinkTables(t, obs)
	generation := storetest.Scalar[int64](t, obs, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", m.ID)
	savedKey := bytes.Clone(server.SigningPublicKey)
	_ = s.CloseRequest()
	server.Stop(t)
	seedStale(t, obs, m.ID, generation, 1)
	server.Restart(t)
	if !bytes.Equal(server.SigningPublicKey, savedKey) {
		t.Fatal("Restart changed the server signing public key.")
	}
	requireFirstOffline(t, server.Clients(t), m.ID)
	if storetest.Scalar[bool](t, obs, "SELECT EXISTS (SELECT FROM link_connections WHERE machine_id=$1)", m.ID) {
		t.Fatal("Startup retained the stale connection row.")
	}
	_, w := applicationLinkStream(t, m, server.Link())
	if w.LastEventNumber != 1 || storetest.Scalar[int64](t, obs, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", m.ID) != generation+1 {
		t.Fatal("Reconnect did not reuse the mark and advance the retained generation.")
	}
}

type appStreamLog struct {
	mu     sync.Mutex
	output bytes.Buffer
}

func (l *appStreamLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.output.Write(p)
}
func (l *appStreamLog) contains(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Contains(l.output.String(), s)
}

func TestPartialLinkCleanupRefusesReadiness(t *testing.T) {
	server := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	cfg, obs := fixtureLinkConfig(t, server)
	clients := server.Clients(t)
	a := joinAppLink(t, server, clients, "first stale machine")
	b := joinAppLink(t, server, clients, "second stale machine")
	approveAppLink(t, clients, a.ID, "First approved machine")
	approveAppLink(t, clients, b.ID, "Second approved machine")
	oldKey := bytes.Clone(server.SigningPublicKey)
	server.Stop(t)
	ids := []string{a.ID, b.ID}
	slices.Sort(ids)
	seedStale(t, obs, ids[0], 11, 31)
	seedStale(t, obs, ids[1], 12, 32)
	storetest.Exec(t, obs, `CREATE SEQUENCE startup_link_deletes;
		CREATE FUNCTION startup_link_second_delete() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
		  IF nextval('startup_link_deletes') = 2 THEN RAISE EXCEPTION 'fixture rejects second delete'; END IF;
		  RETURN OLD;
		END $$;
		CREATE TRIGGER startup_link_second_delete BEFORE DELETE ON link_connections
		FOR EACH ROW EXECUTE FUNCTION startup_link_second_delete()`)
	logs := new(appStreamLog)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	started, err := app.Start(ctx, cfg, slog.New(slog.NewJSONHandler(logs, nil)))
	if started != nil {
		_ = started.Close()
		t.Fatal("The listener started after the second cleanup failed.")
	}
	const diagnostic = "The server could not clear stale link connections."
	var refused *app.StartupError
	if !errors.As(err, &refused) || !slices.Contains(refused.Lines(), diagnostic) || !logs.contains(diagnostic) || logs.contains("link_signing_key_loaded") {
		t.Fatal("Partial cleanup did not report its exact startup failure with readiness withheld.")
	}
	dialer := net.Dialer{Timeout: time.Second}
	if conn, err := dialer.DialContext(ctx, "tcp", cfg.ListenAddr); err == nil {
		_ = conn.Close()
		t.Fatal("The partially cleaned server listened for requests.")
	}
	if storetest.Scalar[bool](t, obs, "SELECT EXISTS (SELECT FROM link_connections WHERE machine_id=$1)", ids[0]) ||
		!storetest.Scalar[bool](t, obs, "SELECT EXISTS (SELECT FROM link_connections WHERE machine_id=$1)", ids[1]) {
		t.Fatal("The first stale connection did not commit independently of the failed second connection.")
	}
	s, err := store.Open(ctx, cfg.Postgres, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The independent startup-lock probe could not open its store.")
	}
	defer s.Close()
	lock, err := s.AcquireServerInstance(ctx, time.Second)
	if err != nil {
		t.Fatal("Startup did not release its database instance lock after partial cleanup.")
	}
	defer lock.Close(context.Background())
	if err := lock.Close(ctx); err != nil {
		t.Fatal("The fixture could not release its instance-lock probe.")
	}
	storetest.Exec(t, obs, "DROP TRIGGER startup_link_second_delete ON link_connections")
	server.Restart(t)
	if !bytes.Equal(server.SigningPublicKey, oldKey) {
		t.Fatal("Partial cleanup or restart changed the server signing key.")
	}
	requireFirstOffline(t, server.Clients(t), ids...)
	for i, id := range ids {
		if storetest.Scalar[int64](t, obs, "SELECT event_mark FROM link_machines WHERE machine_id=$1", id) != 31+int64(i) ||
			storetest.Scalar[int64](t, obs, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", id) != 11+int64(i) {
			t.Fatal("Cleanup or retry changed a durable event mark or stream generation.")
		}
	}
	if storetest.Scalar[int64](t, obs, "SELECT count(*) FROM link_connections") != 0 {
		t.Fatal("The successful retry left a stale connection row.")
	}
}

func TestShutdownEndsLinkStreams(t *testing.T) {
	opts := servertest.Options{Password: machinesFixturePassword}
	seams := reflect.ValueOf(&opts).Elem().FieldByName("LinkSeams")
	if !seams.IsValid() {
		t.Fatal("The fixture did not expose its planned link seams for shutdown staging.")
	}
	proofHeld := make(chan string, 1)
	admitHeld := make(chan string, 1)
	release, finish := storetest.Signal(t)
	defer finish()
	var phases sync.Map
	hold := func(which string, entered chan<- string) func(context.Context, string) {
		return func(ctx context.Context, id string) {
			if phase, ok := phases.Load(id); ok && phase == which {
				entered <- id
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
		}
	}
	seams.FieldByName("AfterProof").Set(reflect.ValueOf(hold("proof", proofHeld)))
	seams.FieldByName("AfterAdmission").Set(reflect.ValueOf(hold("admission", admitHeld)))
	server := servertest.Start(t, opts)
	_, obs := fixtureLinkConfig(t, server)
	clients := server.Clients(t)
	live := joinAppLink(t, server, clients, "live on shutdown")
	pre := joinAppLink(t, server, clients, "proof on shutdown")
	admitted := joinAppLink(t, server, clients, "admission on shutdown")
	cleanup := joinAppLink(t, server, clients, "cleanup on shutdown")
	phases.Store(pre.ID, "proof")
	phases.Store(admitted.ID, "admission")
	liveStream, _ := applicationLinkStream(t, live, server.Link())
	deleting, _ := applicationLinkStream(t, cleanup, server.Link())
	p := appOpen(t, server.Link())
	sendAppOpen(t, p, appChallenge(t, p, pre, 4))
	if storetest.Receive(t, "the application holding the proven machine before admission", proofHeld) != pre.ID {
		t.Fatal("The wrong proof seam entered.")
	}
	a := appOpen(t, server.Link())
	sendAppOpen(t, a, appChallenge(t, a, admitted, 5))
	if storetest.Receive(t, "the application holding an admitted machine before Welcome", admitHeld) != admitted.ID {
		t.Fatal("The wrong admission seam entered.")
	}
	const gate int64 = 145291
	unlock := storetest.HoldAdvisoryGate(t, obs, gate)
	defer unlock()
	storetest.Exec(t, obs, `CREATE FUNCTION startup_hold_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_advisory_xact_lock(145291); RETURN OLD; END $$;
		CREATE TRIGGER startup_hold_cleanup BEFORE DELETE ON link_connections
		FOR EACH ROW EXECUTE FUNCTION startup_hold_cleanup()`)
	_ = deleting.CloseRequest()
	storetest.Poll(t, "the separate cleanup waiting in PostgreSQL", func() bool {
		return storetest.Scalar[bool](t, obs, `SELECT EXISTS(SELECT FROM pg_stat_activity WHERE datname=current_database()
			AND wait_event_type='Lock' AND $1=ANY(pg_blocking_pids(pid)))`, int32(obs.PgConn().PID()))
	})
	deadline := time.Now().Add(5 * time.Second)
	stopped := storetest.Start(t, func() bool { server.Stop(t); return true })
	for _, stream := range []*appOpenStream{p, a, liveStream} {
		requireAppOpenStatus(t, stream, connect.CodeUnavailable, "The server is stopping.")
	}
	unlock()
	finish()
	select {
	case <-stopped.Done():
	case <-time.After(max(0, time.Until(deadline))):
		t.Fatal("Stopping the server exceeded the application drain budget.")
	}
}

func TestOpenReportsDoNotReachRevokedWatch(t *testing.T) {
	server := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	clients := server.Clients(t)
	m := joinAppLink(t, server, clients, "report after sign-out")
	w := fixtureMachinesWatch(t, clients)
	if len(fixtureMachinesSnapshot(t, w)) != 1 {
		t.Fatal("The authorized watch did not see the joined fixture machine.")
	}
	stream, _ := applicationLinkStream(t, m, server.Link())
	for {
		if !w.Receive() {
			t.Fatalf("The authorized watch did not receive the connection save before revocation: %v", w.Err())
		}
		saved := w.Msg().GetSaved()
		if saved.GetId() == m.ID && saved.GetConnectedAt() != nil {
			break
		}
	}
	session := lgtmv1connect.NewSessionServiceClient(server.HTTPClient, server.URL)
	if _, err := session.SignOut(appOpenContext(t), connect.NewRequest(&webv1.SignOutRequest{})); err != nil {
		t.Fatal("The fixture could not revoke its website session.")
	}
	appLinkEvent(t, stream, 1)
	if w.Receive() || connect.CodeOf(w.Err()) != connect.CodeUnauthenticated {
		t.Fatal("A report reached the revoked website watch or its authorization failure was not readable.")
	}

	clients = server.Clients(t)
	req := connect.NewRequest(&webv1.WatchClientsRequest{})
	req.Header().Set("Origin", "https://disallowed.example.test")
	disallowed, err := clients.WatchClients(appOpenContext(t), req)
	if err == nil {
		defer disallowed.Close()
		if disallowed.Receive() {
			t.Fatal("A watch with a disallowed origin received a machine record.")
		}
		err = disallowed.Err()
	}
	if err == nil {
		t.Fatal("The disallowed-origin watch was accepted.")
	}
}

func TestLinkStartupPreservesPartAReceipt(t *testing.T) {
	fixture := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	cfg, _ := fixtureLinkConfig(t, fixture)
	fixture.Stop(t)
	cfg.Postgres = storetest.NewDatabase(t, "public")
	s, err := store.Open(t.Context(), cfg.Postgres, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The Part A fixture database could not open.")
	}
	defer s.Close()
	partA := []store.Migrations{auth.Migrations(), vault.Migrations(), machines.Migrations()}
	if s.Migrate(t.Context(), partA...) != nil {
		t.Fatal("The Part A fixture migrations could not be applied.")
	}
	registry := machines.NewRegistry()
	var id string
	if err := s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		pub, _, err := ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		m, err := registry.CreateWaiting(ctx, tx, machines.NewMachine{PublicKey: pub, HostName: "Part A machine", TokenID: "part-a-fixture"})
		id = m.ID
		return err
	}); err != nil {
		t.Fatal("The Part A fixture machine could not be registered.")
	}
	approve := func() *connect.Request[webv1.ApproveClientRequest] {
		req := connect.NewRequest(&webv1.ApproveClientRequest{Id: id, Name: "Part A machine"})
		req.Header().Set(store.RequestKeyHeader, "part-a-keyed-approval")
		return req
	}
	baseline := machines.NewService(s, nil, registry, nil)
	before, err := baseline.ApproveClient(t.Context(), approve())
	if err != nil {
		t.Fatal("The Part A service could not create a keyed receipt.")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	active, err := app.Start(ctx, cfg, nil)
	if err != nil {
		t.Fatal("The streaming application could not upgrade the Part A database.")
	}
	defer active.Close()
	clients := fixture.Clients(t)
	replayed, err := clients.ApproveClient(ctx, approve())
	if err != nil || !proto.Equal(before.Msg, replayed.Msg) {
		t.Fatal("Migration changed the existing keyed approval receipt or its replay.")
	}
	obs, err := pgx.ConnectConfig(ctx, cfg.Postgres.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The Part A ledger observer could not connect.")
	}
	defer obs.Close(context.Background())
	requireLinkTables(t, obs)
	if storetest.Scalar[int64](t, obs, "SELECT state_revision FROM machines WHERE id = $1", id) != 2 {
		t.Fatal("Replaying the keyed approval changed the migrated registry.")
	}
	if err := active.Close(); err != nil {
		t.Fatal("The upgraded application did not close.")
	}
	if err := s.Migrate(ctx, partA...); err == nil {
		t.Fatal("The Part A migration set silently accepted the new link ledger.")
	}
}
