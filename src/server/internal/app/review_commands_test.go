package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"io"
	"io/fs"
	"net"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/app"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func TestServerPeriodicCommandSweep(t *testing.T) {
	fixture := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	cfg, obs := fixtureLinkConfig(t, fixture)
	machine := joinAppLink(t, fixture, fixture.Clients(t), "periodic sweep")
	fixture.Stop(t)

	sweep := &commandReviewTrace{pruned: make(chan struct{}, 1)}
	cfg.Postgres.ConnConfig.Tracer = sweep
	ticks := make(chan time.Time, 1)
	startCommandReviewApp(t, link.WithCommandSweepTicks(t.Context(), ticks), cfg)
	storetest.Receive(t, "the startup sweep finishing its prune query", sweep.pruned)

	id, err := store.NewID()
	payload, marshalErr := proto.Marshal(&linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}})
	if err != nil || marshalErr != nil {
		t.Fatal("The periodic sweep fixture could not prepare a restart.")
	}
	if _, err := obs.Exec(t.Context(), `INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
		VALUES ($1, $2, 'restart', $3, clock_timestamp() - interval '26 hours', clock_timestamp() - interval '25 hours')`,
		id, machine.ID, payload); err != nil || !appCommandStillQueued(t, obs, id) {
		t.Fatal("The due restart was not queued after the startup pass.")
	}

	ticks <- time.Time{}
	appPollCommand(t, "The application's periodic sweep did not finish the restart queued after startup.", func(ctx context.Context) bool {
		var finished bool
		err := obs.QueryRow(ctx, `SELECT EXISTS (SELECT FROM link_commands WHERE id = $1
			AND outcome = 'timed_out' AND finished_at IS NOT NULL AND payload IS NULL)`, id).Scan(&finished)
		return err == nil && finished
	})
}

func TestServerStopsWithCommandFlushAndSweepHeld(t *testing.T) {
	fixture := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	cfg, obs := fixtureLinkConfig(t, fixture)
	owner := fixture.Clients(t)
	machine := joinAppLink(t, fixture, owner, "held restart")
	approveAppLink(t, owner, machine.ID, "Machine with a held restart")
	stream, _ := applicationLinkStream(t, machine, fixture.Link())
	standing := appReceiveCommand(t, stream, "The setup stream did not receive the approval notice.")
	appSendCommandDone(t, stream, standing)
	appAwaitCommandDone(t, obs, machine.ID, standing, "The setup stream did not finish the approval notice.")
	_ = stream.CloseRequest()
	_ = stream.CloseResponse()
	fixture.Stop(t)

	_, locker := fixtureLinkConfig(t, fixture)
	blocker, err := locker.Begin(t.Context())
	if err != nil {
		t.Fatal("The sweep blocker could not begin its transaction.")
	}
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })

	sweepGate, releaseSweep := storetest.Signal(t)
	trace := &commandReviewTrace{due: make(chan int32, 1), dueGate: sweepGate, sendChecked: make(chan int32, 1)}
	cfg.Postgres.ConnConfig.Tracer = trace
	server := startCommandReviewApp(t, t.Context(), cfg)
	defer releaseSweep()
	sweeper := storetest.Receive(t, "the startup sweep arriving at its due query", trace.due)
	peer := openCommandReviewPeer(t, server.Addr().String(), machine)
	defer peer.conn.Close()

	if _, err := owner.ReconnectClient(appOpenContext(t), connect.NewRequest(&webv1.ReconnectClientRequest{Id: machine.ID})); err != nil {
		t.Fatal("The owner could not queue a restart on the live stream.")
	}
	sender := storetest.Receive(t, "the fenced command read before the held flush", trace.sendChecked)
	var id string
	var expiry time.Time
	err = obs.QueryRow(t.Context(), `SELECT id, expires_at FROM link_commands
		WHERE machine_id = $1 AND action = 'restart' AND attempt = 1 AND NOT dispatched
			AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL`, machine.ID).Scan(&id, &expiry)
	if err != nil {
		t.Fatal("The first claim was not durable while the flush lacked response credit.")
	}

	// The send transaction holds a shared table lock. Queue an exclusive lock ahead of the sweep.
	lockerPID := int32(locker.PgConn().PID())
	lockCtx, cancelLock := context.WithCancel(t.Context())
	defer cancelLock()
	lock := storetest.Start(t, func() error {
		_, err := blocker.Exec(lockCtx, "LOCK TABLE link_commands IN ACCESS EXCLUSIVE MODE")
		return err
	})
	appPollCommand(t, "The flush did not hold the send transaction across the queued table lock.", func(ctx context.Context) bool {
		var blocked bool
		err := obs.QueryRow(ctx, `SELECT $1 = ANY(pg_blocking_pids($2))`, sender, lockerPID).Scan(&blocked)
		return err == nil && blocked
	})
	releaseSweep()
	appPollCommand(t, "The command flush and sweep query were never held together.", func(ctx context.Context) bool {
		var held bool
		err := obs.QueryRow(ctx, `SELECT $1 = ANY(pg_blocking_pids($2)) AND $2 = ANY(pg_blocking_pids($3))
			AND EXISTS (SELECT FROM pg_stat_activity WHERE pid = $1 AND state = 'idle in transaction'
				AND query LIKE '%SELECT action, expires_at, clock_timestamp()%')
			AND EXISTS (SELECT FROM pg_stat_activity WHERE pid = $3 AND wait_event_type = 'Lock'
				AND query LIKE '%SELECT expires_at, machine_id, seq%')`, sender, lockerPID, sweeper).Scan(&held)
		return err == nil && held
	})

	instance := storetest.Scalar[int32](t, obs, `SELECT pid FROM pg_stat_activity
		WHERE datname = current_database() AND application_name = 'lgtm-server-instance'`)
	if !storetest.Scalar[bool](t, obs, "SELECT pg_terminate_backend($1)", instance) {
		t.Fatal("The fixture could not end the application's instance session.")
	}
	stopped := storetest.Start(t, server.Wait)
	shutdownErr := stopped.Wait(t)
	if shutdownErr == nil || shutdownErr.Error() != "The server lost its PostgreSQL instance lock." {
		t.Fatal("Application shutdown lost the original instance-lock failure.")
	}
	if lock.Wait(t) != nil {
		t.Fatal("Application shutdown did not free the flush's table lock.")
	}
	appPollCommand(t, "Shutdown left the instance session or either held database operation behind.", func(ctx context.Context) bool {
		var clear bool
		err := obs.QueryRow(ctx, `SELECT NOT EXISTS (SELECT FROM pg_stat_activity
			WHERE datname = current_database() AND (pid = $1
				OR (pid = $2 AND xact_start IS NOT NULL)
				OR (pid = $3 AND wait_event_type = 'Lock')))`, instance, sender, sweeper).Scan(&clear)
		return err == nil && clear
	})

	var pending bool
	err = blocker.QueryRow(t.Context(), `SELECT EXISTS (SELECT FROM link_commands
		WHERE id = $1 AND expires_at = $2 AND attempt = 1 AND NOT dispatched
			AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL)`, id, expiry).Scan(&pending)
	if err != nil || !pending {
		t.Fatal("Shutdown did not leave the original unflushed attempt pending.")
	}
	if blocker.Rollback(t.Context()) != nil {
		t.Fatal("The fixture could not release the sweep blocker.")
	}

	fixture.Restart(t)
	reconnected, _ := applicationLinkStream(t, machine, fixture.Link())
	retry := appReceiveCommand(t, reconnected, "The restarted application did not redeliver the held command.")
	if retry.GetRestart() == nil || retry.GetId() != id || retry.GetAttempt() != 2 || !retry.GetExpiresAt().AsTime().Equal(expiry) {
		t.Fatal("The restart did not deliver the same command and expiry at attempt two.")
	}
	appPollCommand(t, "The retry did not durably dispatch at attempt two.", func(ctx context.Context) bool {
		return appRestartPending(ctx, obs, machine.ID, retry)
	})
	appSendCommandDone(t, reconnected, retry)
	appAwaitCommandDone(t, obs, machine.ID, retry, "The matching second attempt did not commit its result.")
}

func TestServerRestartsAfterInterruptedCommandMigration(t *testing.T) {
	fixture := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	cfg, _ := fixtureLinkConfig(t, fixture)
	fixture.Stop(t)
	cfg.Postgres = storetest.NewDatabase(t, "public")
	obs := commandReviewObserver(t, cfg)
	locker := commandReviewObserver(t, cfg)
	id, approve, saved := seedCommandReviewUpgrade(t, cfg)
	recordDigest := storetest.Scalar[string](t, obs, `SELECT md5(to_jsonb(m)::text) FROM machines m
		WHERE m.id = $1 AND m.name = 'Machine after approval' AND m.state_revision = 3`, id)
	if !storetest.Scalar[bool](t, obs, `SELECT count(*) = 1 AND max(version) = 1
		FROM schema_migrations WHERE part = 'link'`) {
		t.Fatal("The fixture did not start from the version-one link schema.")
	}

	blocker, err := locker.Begin(t.Context())
	if err != nil {
		t.Fatal("The reconciliation blocker could not begin its transaction.")
	}
	defer blocker.Rollback(context.Background())
	if _, err := blocker.Exec(t.Context(), "LOCK TABLE web_sessions IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		t.Fatal("The fixture could not block session reconciliation.")
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	trace := &commandReviewTrace{pruned: make(chan struct{}, 1)}
	cfg.Postgres.ConnConfig.Tracer = trace
	type startupResult struct {
		server *app.Server
		err    error
	}
	startup := storetest.Start(t, func() startupResult {
		server, err := app.Start(ctx, cfg, nil)
		return startupResult{server, err}
	})
	storetest.Poll(t, "startup waiting on the session reconciliation lock", func() bool {
		select {
		case <-startup.Done():
			if result := startup.Result(); result.server != nil {
				_ = result.server.Close()
			}
			t.Fatal("Startup returned before it waited on session reconciliation.")
		default:
		}
		return storetest.Scalar[bool](t, obs, `SELECT EXISTS (SELECT FROM pg_locks l JOIN pg_stat_activity a USING (pid)
			WHERE a.datname = current_database() AND l.relation = 'web_sessions'::regclass
				AND NOT l.granted AND l.mode = 'ShareRowExclusiveLock'
				AND $1 = ANY(pg_blocking_pids(l.pid)))`, int32(locker.PgConn().PID()))
	})
	if !storetest.Scalar[bool](t, obs, `SELECT EXISTS (SELECT FROM schema_migrations WHERE part = 'link' AND version = 2)
		AND to_regclass('link_commands') IS NOT NULL`) {
		t.Fatal("Startup reached the interruption point before command migration two committed.")
	}
	requireCommandReviewListenerClosed(t, cfg.ListenAddr)
	cancel()
	interrupted := startup.Wait(t)
	if interrupted.server != nil {
		_ = interrupted.server.Close()
		t.Fatal("The interrupted migration startup returned a listening application.")
	}
	startupErr, ok := interrupted.err.(*app.StartupError)
	if !ok || !slices.Equal(startupErr.Lines(), []string{"Server startup was canceled.", "Session reconciliation did not complete."}) {
		t.Fatal("The interrupted startup did not report cancellation during session reconciliation.")
	}
	requireCommandReviewListenerClosed(t, cfg.ListenAddr)
	if trace.passes.Load() != 0 {
		t.Fatal("Interrupted startup launched a sweep before reconciliation completed.")
	}
	appPollCommand(t, "Interrupted startup did not release the instance lock and reconciliation query.", func(ctx context.Context) bool {
		var clear bool
		err := obs.QueryRow(ctx, `SELECT NOT EXISTS (
			SELECT FROM pg_stat_activity WHERE datname = current_database()
				AND (application_name = 'lgtm-server-instance' OR $1 = ANY(pg_blocking_pids(pid))))`,
			int32(locker.PgConn().PID())).Scan(&clear)
		return err == nil && clear
	})
	if blocker.Rollback(t.Context()) != nil {
		t.Fatal("The fixture could not release the reconciliation blocker.")
	}

	restarted := startCommandReviewApp(t, t.Context(), cfg)
	storetest.Receive(t, "the restarted application's startup sweep", trace.pruned)
	if !storetest.Scalar[bool](t, obs, `SELECT count(*) = 1 FROM pg_locks l JOIN pg_stat_activity a USING (pid)
		WHERE a.datname = current_database() AND a.application_name = 'lgtm-server-instance'
			AND l.locktype = 'advisory' AND l.granted`) {
		t.Fatal("The restarted application did not hold exactly one instance lock.")
	}
	replayed, err := fixture.Clients(t).ApproveClient(appOpenContext(t), approve())
	if err != nil || !proto.Equal(saved, replayed.Msg) {
		t.Fatal("The restarted application did not replay the original keyed approval reply.")
	}
	if !storetest.Scalar[bool](t, obs, `SELECT EXISTS (SELECT FROM machines m WHERE m.id = $1
			AND md5(to_jsonb(m)::text) = $2)
		AND (SELECT count(*) FROM receipts) = 1 AND NOT EXISTS (SELECT FROM link_commands)`, id, recordDigest) {
		t.Fatal("Restart or replay changed the prior record, receipt count, or command queue.")
	}
	_ = restarted.Close()
	if trace.passes.Load() != 1 {
		t.Fatal("The restarted application did not run exactly one startup sweep.")
	}
}

func seedCommandReviewUpgrade(t *testing.T, cfg config.Config) (string, func() *connect.Request[webv1.ApproveClientRequest], *webv1.ApproveClientResponse) {
	t.Helper()
	old, err := store.Open(t.Context(), cfg.Postgres, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The prior-version fixture store could not open.")
	}
	defer old.Close()
	linkV1 := link.Migrations()
	const migration = "0001_link.sql"
	schema, err := fs.ReadFile(linkV1.Files, migration)
	if err != nil {
		t.Fatal("The prior link migration could not be read.")
	}
	linkV1.Files = fstest.MapFS{migration: &fstest.MapFile{Data: schema}}
	if old.Migrate(t.Context(), auth.Migrations(), vault.Migrations(), machines.Migrations(), linkV1) != nil {
		t.Fatal("The prior-version server schema could not be applied.")
	}

	public, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("The prior-version fixture could not generate a machine identity.")
	}
	registry := machines.NewRegistry()
	var id string
	if old.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		machine, err := registry.CreateWaiting(ctx, tx, machines.NewMachine{
			PublicKey: public, HostName: "Upgrade machine", TokenID: "upgrade-fixture",
		})
		id = machine.ID
		return err
	}) != nil {
		t.Fatal("The prior-version fixture could not save the machine.")
	}
	approve := func() *connect.Request[webv1.ApproveClientRequest] {
		req := connect.NewRequest(&webv1.ApproveClientRequest{Id: id, Name: "Machine at approval"})
		req.Header().Set(store.RequestKeyHeader, "interrupted-upgrade-approval")
		return req
	}
	baseline := machines.NewService(old, nil, registry, nil)
	saved, err := baseline.ApproveClient(t.Context(), approve())
	if err != nil || saved.Msg.GetClient().GetName() != "Machine at approval" {
		t.Fatal("The prior-version fixture could not save the keyed approval reply.")
	}
	if _, err := baseline.RenameClient(t.Context(), connect.NewRequest(&webv1.RenameClientRequest{
		Id: id, Name: "Machine after approval",
	})); err != nil {
		t.Fatal("The prior-version fixture could not make the live record differ from the saved reply.")
	}
	return id, approve, saved.Msg
}

func startCommandReviewApp(t *testing.T, ctx context.Context, cfg config.Config) *app.Server {
	t.Helper()
	server, err := app.Start(ctx, cfg, nil)
	if err != nil {
		t.Fatal("The command review application could not start.")
	}
	t.Cleanup(func() { _ = server.Close() })
	return server
}

func commandReviewObserver(t *testing.T, cfg config.Config) *pgx.Conn {
	t.Helper()
	observer, err := pgx.ConnectConfig(t.Context(), cfg.Postgres.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The command review database observer could not connect.")
	}
	t.Cleanup(func() { _ = observer.Close(context.Background()) })
	return observer
}

func requireCommandReviewListenerClosed(t *testing.T, address string) {
	t.Helper()
	if conn, err := net.DialTimeout("tcp", address, time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("The application opened its listener before it finished the interrupted startup.")
	}
}

type commandReviewTrace struct {
	due         chan int32
	dueGate     <-chan struct{}
	pruned      chan struct{}
	sendChecked chan int32
	passes      atomic.Int32
}

type commandReviewSQLKey struct{}

func (trace *commandReviewTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "SELECT expires_at, machine_id, seq") {
		trace.passes.Add(1)
		if trace.due != nil {
			select {
			case trace.due <- int32(conn.PgConn().PID()):
			default:
			}
			select {
			case <-trace.dueGate:
			case <-ctx.Done():
			}
		}
	}
	return context.WithValue(ctx, commandReviewSQLKey{}, data.SQL)
}

func (trace *commandReviewTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		return
	}
	sql, _ := ctx.Value(commandReviewSQLKey{}).(string)
	switch {
	case trace.pruned != nil && strings.Contains(sql, "SELECT finished_at, machine_id, seq"):
		select {
		case trace.pruned <- struct{}{}:
		default:
		}
	case trace.sendChecked != nil && strings.Contains(sql, "SELECT action, expires_at, clock_timestamp()"):
		select {
		case trace.sendChecked <- int32(conn.PgConn().PID()):
		default:
		}
	}
}

// The peer grants response credit only for the handshake and one heartbeat.
type commandReviewPeer struct {
	conn   net.Conn
	framer *http2.Framer
}

func openCommandReviewPeer(t *testing.T, address string, machine *machinetest.Machine) *commandReviewPeer {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, storetest.ObservationTimeout)
	if err != nil {
		t.Fatal("The command peer could not connect to the application.")
	}
	t.Cleanup(func() { _ = conn.Close() })
	if conn.SetDeadline(time.Now().Add(storetest.ObservationTimeout)) != nil {
		t.Fatal("The command peer could not bound its input and output.")
	}
	p := &commandReviewPeer{conn: conn, framer: http2.NewFramer(conn, conn)}
	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil ||
		p.framer.WriteSettings(http2.Setting{ID: http2.SettingInitialWindowSize, Val: 0}) != nil {
		t.Fatal("The command peer could not advertise zero stream credit.")
	}
	var block bytes.Buffer
	encoder := hpack.NewEncoder(&block)
	for _, field := range []hpack.HeaderField{
		{Name: ":method", Value: "POST"},
		{Name: ":scheme", Value: "http"},
		{Name: ":authority", Value: address},
		{Name: ":path", Value: linkv1connect.LinkServiceOpenProcedure},
		{Name: "content-type", Value: "application/grpc"},
		{Name: "te", Value: "trailers"},
	} {
		if encoder.WriteField(field) != nil {
			t.Fatal("The command peer could not encode its request headers.")
		}
	}
	if p.framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 1, BlockFragment: block.Bytes(), EndHeaders: true}) != nil {
		t.Fatal("The command peer could not open a request stream.")
	}
	nonce := bytes.Repeat([]byte{18}, 32)
	p.send(t, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Hello{Hello: &linkv1.Hello{
		MachineId: machine.ID, LinkRevision: 1, MachineNonce: nonce,
	}}})
	challenge := p.receive(t).GetChallenge()
	if len(challenge.GetServerNonce()) != 32 || !ed25519.Verify(machine.ServerKey,
		appProofText("lgtm-link-v1 server", machine.ID, nonce, challenge.GetServerNonce()), challenge.GetServerSignature()) {
		t.Fatal("The command peer did not receive a verifiable challenge.")
	}
	proof := ed25519.Sign(machine.Key, appProofText("lgtm-link-v1 machine", machine.ID, nonce, challenge.GetServerNonce()))
	p.send(t, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{Proof: &linkv1.Proof{MachineSignature: proof}}})
	welcome := p.receive(t).GetWelcome()
	if welcome.GetState().GetApproval() != linkv1.Approval_APPROVAL_APPROVED || welcome.GetConnectionToken() == "" {
		t.Fatal("The command peer did not receive an approved Welcome.")
	}
	// The acknowledgement proves that the server published the stream before it receives the restart.
	p.send(t, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}})
	if p.receive(t).GetHeartbeatAck() == nil {
		t.Fatal("The command peer did not receive a heartbeat acknowledgement before withholding credit.")
	}
	return p
}

func (p *commandReviewPeer) send(t *testing.T, frame *linkv1.OpenRequest) {
	t.Helper()
	body, err := proto.Marshal(frame)
	if err != nil {
		t.Fatal("The command peer could not encode its request.")
	}
	envelope := make([]byte, 5, 5+len(body))
	binary.BigEndian.PutUint32(envelope[1:], uint32(len(body)))
	if p.framer.WriteData(1, false, append(envelope, body...)) != nil {
		t.Fatal("The command peer could not send its request.")
	}
}

func (p *commandReviewPeer) receive(t *testing.T) *linkv1.OpenResponse {
	t.Helper()
	prefix := p.readCredit(t, 5)
	size := binary.BigEndian.Uint32(prefix[1:])
	if prefix[0] != 0 || size == 0 || size > 64<<10 {
		t.Fatal("The command peer did not receive a bounded, uncompressed response envelope.")
	}
	frame := new(linkv1.OpenResponse)
	if proto.Unmarshal(p.readCredit(t, size), frame) != nil {
		t.Fatal("The command peer could not decode the response envelope.")
	}
	return frame
}

func (p *commandReviewPeer) readCredit(t *testing.T, size uint32) []byte {
	t.Helper()
	if p.framer.WriteWindowUpdate(1, size) != nil {
		t.Fatal("The command peer could not grant response credit.")
	}
	data := make([]byte, 0, size)
	for len(data) < int(size) {
		frame, err := p.framer.ReadFrame()
		if err != nil {
			t.Fatal("The command peer could not receive its credited response bytes.")
		}
		switch frame := frame.(type) {
		case *http2.SettingsFrame:
			if !frame.IsAck() && p.framer.WriteSettingsAck() != nil {
				t.Fatal("The command peer could not acknowledge server settings.")
			}
		case *http2.DataFrame:
			if frame.StreamID != 1 || frame.StreamEnded() || len(data)+len(frame.Data()) > int(size) {
				t.Fatal("The server ended the stream or exceeded its response credit.")
			}
			data = append(data, frame.Data()...)
		case *http2.RSTStreamFrame, *http2.GoAwayFrame:
			t.Fatal("The command peer's transport ended during the handshake.")
		}
	}
	return data
}
