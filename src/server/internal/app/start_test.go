package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault/vaulttest"
)

const (
	fixturePassword     = "fake-app-owner-password"
	replacementPassword = "fake-app-replacement-password"
	instanceApplication = "lgtm-server-instance"
	migrationKey        = 841941100002
	keyLoaded           = "link_signing_key_loaded"
	reconcileFailed     = "Session reconciliation did not complete."
	purgeCommitted      = "Session revocation committed."
	purgeUnconfirmed    = "Session revocation was not confirmed."
	passwordInvalid     = "LGTM_WEB_PASSWORD is missing or invalid."
	instanceBusy        = "Another server is already using this database."
)

var fixtureSeed = [32]byte{83, 19, 5}

type fixtureLog struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (l *fixtureLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.Write(p)
}

func (l *fixtureLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}

type startResult struct {
	server *Server
	err    error
}

func testBudgets() phaseBudgets {
	return phaseBudgets{
		admission: 3 * time.Second, migration: 15 * time.Second,
		prerequisites: 10 * time.Second, probe: 5 * time.Second,
	}
}

func testAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("Could not reserve a loopback fixture port.")
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func appConfig(t *testing.T) (config.Config, *pgx.Conn) {
	t.Helper()
	cfg := storetest.NewDatabase(t, "public")
	observer, err := pgx.ConnectConfig(t.Context(), cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatal("Could not connect the startup observer.")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = observer.Close(ctx)
	})
	address, file := vaulttest.Start(t, fixtureSeed)
	listen := testAddress(t)
	return config.Config{
		Postgres: cfg, ListenAddr: listen, PublicURL: "http://" + listen,
		Password: fixturePassword, OpenBaoAddr: address, OpenBaoCredentialFile: file,
	}, observer
}

func useAddress(cfg config.Config, address string) config.Config {
	cfg.ListenAddr, cfg.PublicURL = address, "http://"+address
	return cfg
}

func appClient(t *testing.T) *http.Client {
	t.Helper()
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	transport := &http.Transport{Protocols: &protocols}
	t.Cleanup(transport.CloseIdleConnections)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal("Could not create a fixture cookie jar.")
	}
	return &http.Client{Transport: transport, Jar: jar, Timeout: 3 * time.Second}
}

func signIn(t *testing.T, client *http.Client, cfg config.Config) {
	t.Helper()
	service := lgtmv1connect.NewSessionServiceClient(client, cfg.PublicURL)
	resp, err := service.SignIn(t.Context(), connect.NewRequest(&webv1.SignInRequest{Password: cfg.Password}))
	if err != nil || resp.Msg.GetResult() != webv1.SignInResult_SIGN_IN_RESULT_SIGNED_IN {
		t.Fatal("The generated client could not sign in with the fixture password.")
	}
	requireSignedIn(t, client, cfg, true)
}

func requireSignedIn(t *testing.T, client *http.Client, cfg config.Config, want bool) {
	t.Helper()
	service := lgtmv1connect.NewSessionServiceClient(client, cfg.PublicURL)
	resp, err := service.GetServer(t.Context(), connect.NewRequest(&webv1.GetServerRequest{}))
	if err != nil || resp.Msg.GetSignedIn() != want {
		t.Fatalf("GetServer did not report a signed-in state of %t.", want)
	}
	if !want {
		_, err := service.SignOut(t.Context(), connect.NewRequest(&webv1.SignOutRequest{}))
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Error("The old cookie could still call the protected sign-out procedure.")
		}
	}
}

func requireHealthy(t *testing.T, client *http.Client, cfg config.Config) {
	t.Helper()
	res, err := client.Get(cfg.PublicURL + "/healthz")
	if err != nil {
		t.Fatal("The app did not answer a health request.")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("Health returned HTTP %d, want 204.", res.StatusCode)
	}
}

func requireClosed(t *testing.T, address string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		_ = c.Close()
		t.Fatal("The app accepted a connection before it was ready or after it stopped.")
	}
}

func closeApp(t *testing.T, server *Server) {
	t.Helper()
	if err := server.Close(); err != nil {
		t.Error("Closing the running app failed.")
	}
}

func launchApp(t *testing.T, cfg config.Config, budgets phaseBudgets, output io.Writer) *storetest.Task[startResult] {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	log := slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{AddSource: false}))
	task := storetest.Start(t, func() startResult {
		server, err := start(ctx, cfg, log, budgets)
		return startResult{server, err}
	})
	t.Cleanup(func() {
		cancel()
		if result := task.Wait(t); result.server != nil {
			_ = result.server.Close()
		}
	})
	return task
}

func started(t *testing.T, task *storetest.Task[startResult]) *Server {
	t.Helper()
	res := task.Wait(t)
	if res.err != nil || res.server == nil || res.server.Addr() == nil {
		t.Fatal("Startup did not return a listening application.")
	}
	return res.server
}

func refused(t *testing.T, cfg config.Config, task *storetest.Task[startResult], output *fixtureLog) string {
	t.Helper()
	res := task.Wait(t)
	if res.err == nil || res.server != nil {
		t.Error("Startup did not refuse the failed prerequisite.")
	}
	requireClosed(t, cfg.ListenAddr)
	diagnostics := output.String()
	if res.err != nil {
		diagnostics += "\n" + res.err.Error()
	}
	if strings.Contains(diagnostics, keyLoaded) {
		t.Error("Startup logged a ready signing identity after refusing to serve.")
	}
	return diagnostics
}

func requireDiagnostics(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, sentence := range want {
		if !strings.Contains(got, sentence) {
			t.Errorf("Startup omitted the diagnostic %q.", sentence)
		}
	}
}

func requireRedacted(t *testing.T, diagnostic string, values ...string) {
	t.Helper()
	for _, value := range values {
		if value != "" && strings.Contains(diagnostic, value) {
			t.Error("Startup diagnostics included a private fixture value.")
		}
	}
}

func waitForStartupBlock(t *testing.T, obs *pgx.Conn, blocker int32, task *storetest.Task[startResult], kind string) {
	t.Helper()
	storetest.Poll(t, "startup waiting on the fixture lock", func() bool {
		select {
		case <-task.Done():
			t.Fatal("Startup returned before it waited at the prerequisite gate.")
		default:
		}
		return storetest.Scalar[bool](t, obs, `
SELECT EXISTS (
    SELECT 1 FROM pg_locks l JOIN pg_stat_activity a USING (pid)
    WHERE a.datname = current_database() AND l.locktype = $1 AND NOT l.granted
      AND $2 = ANY(pg_blocking_pids(l.pid))
      AND ($1 <> 'relation' OR (l.relation = to_regclass('web_sessions') AND l.mode = 'ShareRowExclusiveLock'))
)`, kind, blocker)
	})
}

func terminateInstance(t *testing.T, obs *pgx.Conn) {
	t.Helper()
	pid := storetest.Scalar[int32](t, obs, `
SELECT COALESCE(max(pid), 0) FROM pg_stat_activity
WHERE datname = current_database() AND application_name = $1`, instanceApplication)
	if pid == 0 || !storetest.Scalar[bool](t, obs, "SELECT pg_terminate_backend($1)", pid) {
		t.Fatal("Could not terminate the app's dedicated instance backend.")
	}
}

func requireWaitBy(t *testing.T, server *Server, deadline time.Time) {
	t.Helper()
	task := storetest.Start(t, server.Wait)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-task.Done():
	case <-timer.C:
		t.Fatal("The app did not finish shutting down within its probe bound.")
	}
}

func sessionCount(t *testing.T, obs *pgx.Conn, want int) {
	t.Helper()
	if got := storetest.Scalar[int](t, obs, "SELECT count(*) FROM web_sessions"); got != want {
		t.Errorf("The database holds %d session rows, want %d.", got, want)
	}
}

func TestStartRevokesBeforeListen(t *testing.T) {
	t.Run("changed password blocks readiness", func(t *testing.T) {
		cfg, obs := appConfig(t)
		server := started(t, launchApp(t, cfg, testBudgets(), io.Discard))
		client := appClient(t)
		signIn(t, client, cfg)
		closeApp(t, server)
		client.CloseIdleConnections()

		tx, err := obs.Begin(t.Context())
		if err != nil {
			t.Fatal("Could not open the reconciliation gate transaction.")
		}
		release := sync.OnceFunc(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = tx.Rollback(ctx)
		})
		defer release()
		storetest.Exec(t, tx, "LOCK TABLE web_sessions IN SHARE ROW EXCLUSIVE MODE")

		// Observe from a separate connection while the transaction owns the gate.
		gateObserver, err := pgx.ConnectConfig(t.Context(), cfg.Postgres.ConnConfig.Copy())
		if err != nil {
			t.Fatal("Could not connect the reconciliation gate observer.")
		}
		defer gateObserver.Close(context.Background())

		cfg.Password = replacementPassword
		pending := launchApp(t, cfg, testBudgets(), io.Discard)
		waitForStartupBlock(t, gateObserver, int32(obs.PgConn().PID()), pending, "relation")
		requireClosed(t, cfg.ListenAddr)
		release()

		started(t, pending)
		requireHealthy(t, client, cfg)
		sessionCount(t, obs, 0)
		requireSignedIn(t, client, cfg, false)
	})

	for _, scenario := range []struct {
		name, password string
		abort          bool
	}{
		{name: "empty password"},
		{name: "overlong password", password: strings.Repeat("x", auth.MaxPasswordBytes+1)},
		{name: "purge rolls back", abort: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cfg, obs := appConfig(t)
			server := started(t, launchApp(t, cfg, testBudgets(), io.Discard))
			client := appClient(t)
			signIn(t, client, cfg)
			closeApp(t, server)
			client.CloseIdleConnections()

			if scenario.abort {
				storetest.Exec(t, obs, `
CREATE FUNCTION refuse_startup_purge() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fake-private-purge-detail'; END $$;
CREATE TRIGGER refuse_startup_purge BEFORE DELETE ON web_sessions
FOR EACH ROW EXECUTE FUNCTION refuse_startup_purge()`)
			}
			var log fixtureLog
			bad := cfg
			bad.Password = scenario.password
			bad.OpenBaoAddr = "http://" + testAddress(t)
			diagnostic := refused(t, bad, launchApp(t, bad, testBudgets(), &log), &log)
			requireDiagnostics(t, diagnostic, passwordInvalid)
			requireRedacted(t, diagnostic, fixturePassword, scenario.password, "fake-private-purge-detail")
			if scenario.abort {
				requireDiagnostics(t, diagnostic, reconcileFailed, purgeUnconfirmed)
				if strings.Contains(diagnostic, purgeCommitted) {
					t.Error("Startup reported a committed purge after PostgreSQL rejected it.")
				}
				sessionCount(t, obs, 1)
				return
			}
			requireDiagnostics(t, diagnostic, purgeCommitted)
			if strings.Contains(diagnostic, purgeUnconfirmed) || strings.Contains(diagnostic, vault.ErrUnavailable.Error()) {
				t.Error("Startup continued to the vault or claimed that a committed purge was unconfirmed.")
			}
			sessionCount(t, obs, 0)
			started(t, launchApp(t, cfg, testBudgets(), io.Discard))
			requireSignedIn(t, client, cfg, false)
		})
	}
}

func TestStartWithUnavailableVault(t *testing.T) {
	cfg, obs := appConfig(t)
	s, err := store.Open(t.Context(), cfg.Postgres, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("Could not open the vault startup fixture database.")
	}
	err = s.Migrate(t.Context(), auth.Migrations(), vault.Migrations())
	s.Close()
	if err != nil {
		t.Fatal("Could not migrate the vault startup fixture database.")
	}
	target, err := url.Parse(cfg.OpenBaoAddr)
	if err != nil {
		t.Fatal("The OpenBao fixture did not supply a valid address.")
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorLog = slog.NewLogLogger(slog.NewTextHandler(io.Discard, nil), slog.LevelError)
	var allow atomic.Bool
	var refusedRequests atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allow.Load() {
			refusedRequests.Add(1)
			http.Error(w, "fake-private-vault-error", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(fixture.Close)
	cfg.OpenBaoAddr = fixture.URL

	var failure fixtureLog
	diagnostic := refused(t, cfg, launchApp(t, cfg, testBudgets(), &failure), &failure)
	requireDiagnostics(t, diagnostic, vault.ErrUnavailable.Error())
	requireRedacted(t, diagnostic, cfg.Postgres.ConnConfig.Password, fixturePassword, "fake-private-vault-error", "fixture-role",
		"fixture-secret", "fixture-batch-token", base64.StdEncoding.EncodeToString(fixtureSeed[:]))
	if refusedRequests.Load() == 0 {
		t.Error("Startup failed before reaching the unavailable OpenBao fixture.")
	}
	if n := storetest.Scalar[int](t, obs, "SELECT count(*) FROM server_link_identity"); n != 0 {
		t.Error("The failed startup pinned an identity.")
	}

	allow.Store(true)
	var success fixtureLog
	server := started(t, launchApp(t, cfg, testBudgets(), &success))
	requireHealthy(t, appClient(t), cfg)
	public := ed25519.NewKeyFromSeed(fixtureSeed[:]).Public().(ed25519.PublicKey)
	digest := sha256.Sum256(public)
	if !bytes.Equal(server.SigningPublicKey(), public) {
		t.Error("The app returned a different public key from the one in OpenBao.")
	}
	decoder := json.NewDecoder(strings.NewReader(success.String()))
	events := 0
	for {
		var entry struct {
			Message     string `json:"msg"`
			Source      string `json:"source"`
			Fingerprint string `json:"fingerprint"`
			Version     int    `json:"vault_version"`
		}
		err := decoder.Decode(&entry)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal("Startup did not write valid structured log entries with integer vault versions.")
		}
		if entry.Message != keyLoaded {
			continue
		}
		events++
		if entry.Source != "openbao" || entry.Fingerprint != hex.EncodeToString(digest[:]) || entry.Version != 1 {
			t.Error("The key event did not identify the exact OpenBao fingerprint and version.")
		}
	}
	if events != 1 {
		t.Errorf("Startup emitted %d signing-key readiness events, want one.", events)
	}
	requireRedacted(t, success.String(), cfg.Postgres.ConnConfig.Password, fixturePassword, "fixture-role", "fixture-secret",
		"fixture-batch-token", base64.StdEncoding.EncodeToString(fixtureSeed[:]))
}

func TestStartRefusesSecondInstance(t *testing.T) {
	t.Run("changed password cannot reconcile while another instance serves", func(t *testing.T) {
		cfg, obs := appConfig(t)
		first := started(t, launchApp(t, cfg, testBudgets(), io.Discard))
		client := appClient(t)
		signIn(t, client, cfg)

		second := useAddress(cfg, testAddress(t))
		second.Password = replacementPassword
		budgets := testBudgets()
		budgets.admission = 500 * time.Millisecond
		var log fixtureLog
		diagnostic := refused(t, second, launchApp(t, second, budgets, &log), &log)
		requireDiagnostics(t, diagnostic, instanceBusy, reconcileFailed)
		sessionCount(t, obs, 1)
		requireSignedIn(t, client, cfg, true)

		closeApp(t, first)
		started(t, launchApp(t, second, testBudgets(), io.Discard))
		// Cookie jars scope cookies by hostname, so the second loopback port gets the original cookie.
		requireSignedIn(t, client, second, false)
		sessionCount(t, obs, 0)
	})

	t.Run("dedicated backend termination stops readiness", func(t *testing.T) {
		cfg, obs := appConfig(t)
		budgets := testBudgets()
		budgets.probe = time.Second
		server := started(t, launchApp(t, cfg, budgets, io.Discard))
		requireHealthy(t, appClient(t), cfg)
		terminateInstance(t, obs)
		requireWaitBy(t, server, time.Now().Add(budgets.probe+time.Second))
		requireClosed(t, cfg.ListenAddr)
		started(t, launchApp(t, cfg, testBudgets(), io.Discard))
	})
}

func TestStartupMigrationBudget(t *testing.T) {
	for _, loseLock := range []bool{false, true} {
		name := "migration has its own deadline"
		if loseLock {
			name = "lock lost while migration waits"
		}
		t.Run(name, func(t *testing.T) {
			cfg, obs := appConfig(t)
			release := storetest.HoldAdvisoryGate(t, obs, migrationKey)
			defer release()
			budgets := testBudgets()
			budgets.prerequisites = 2 * time.Second
			budgets.probe = time.Second
			var log fixtureLog
			pending := launchApp(t, cfg, budgets, &log)
			waitForStartupBlock(t, obs, int32(obs.PgConn().PID()), pending, "advisory")
			requireClosed(t, cfg.ListenAddr)
			if loseLock {
				terminateInstance(t, obs)
				timer := time.NewTimer(budgets.probe + 2*time.Second)
				defer timer.Stop()
				select {
				case <-pending.Done():
				case <-timer.C:
					t.Fatal("The migration did not stop when the instance lock was lost.")
				}
				refused(t, cfg, pending, &log)
				return
			}
			timer := time.NewTimer(budgets.prerequisites)
			defer timer.Stop()
			select {
			case <-pending.Done():
				t.Fatal("The prerequisite deadline interrupted the independent migration budget.")
			case <-timer.C:
			}
			release()
			started(t, pending)
			requireHealthy(t, appClient(t), cfg)
		})
	}
}

// probeRelay withholds one PostgreSQL response on the actual instance connection.
type probeRelay struct {
	arm       atomic.Bool
	arrived   chan time.Time
	release   chan struct{}
	gone      chan time.Time
	forwarded chan struct{}
}

func installProbeRelay(t *testing.T, cfg config.Config) *probeRelay {
	t.Helper()
	relay := &probeRelay{
		arrived: make(chan time.Time, 1), release: make(chan struct{}),
		gone: make(chan time.Time, 1), forwarded: make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	cfg.Postgres.BeforeConnect = func(_ context.Context, c *pgx.ConnConfig) error {
		if c.RuntimeParams["application_name"] != instanceApplication {
			return nil
		}
		dial := c.DialFunc
		c.DialFunc = func(connectCtx context.Context, network, addr string) (net.Conn, error) {
			return relay.dial(ctx, connectCtx, dial, network, addr)
		}
		return nil
	}
	return relay
}

func (r *probeRelay) dial(ctx, connectCtx context.Context, dial func(context.Context, string, string) (net.Conn, error), network, addr string) (net.Conn, error) {
	upstream, err := dial(connectCtx, network, addr)
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		_ = upstream.Close()
		return nil, err
	}
	defer listener.Close()
	_ = listener.SetDeadline(time.Now().Add(time.Second))
	client, err := (&net.Dialer{}).DialContext(connectCtx, "tcp4", listener.Addr().String())
	if err != nil {
		_ = upstream.Close()
		return nil, err
	}
	peer, err := listener.AcceptTCP()
	if err != nil {
		_ = upstream.Close()
		_ = client.Close()
		return nil, err
	}

	done := make(chan struct{})
	var heldReply atomic.Bool
	stop := context.AfterFunc(ctx, func() {
		_ = peer.Close()
		_ = upstream.Close()
	})
	go func() {
		defer close(done)
		_, _ = io.Copy(upstream, peer)
		if heldReply.Load() {
			r.gone <- time.Now()
		}
		_ = upstream.Close()
	}()
	go func() {
		defer stop()
		defer peer.Close()
		defer upstream.Close()
		r.forward(peer, upstream, done, &heldReply)
	}()
	return client, nil
}

func (r *probeRelay) forward(peer, upstream net.Conn, done <-chan struct{}, heldReply *atomic.Bool) {
	buf := make([]byte, 4096)
	for {
		n, err := upstream.Read(buf)
		if n == 0 && err != nil {
			return
		}
		held := r.arm.CompareAndSwap(true, false)
		if held {
			heldReply.Store(true)
			r.arrived <- time.Now()
			watchdog := time.NewTimer(8 * time.Second)
			select {
			case <-done:
				watchdog.Stop()
				return
			case <-watchdog.C:
				return
			case <-r.release:
				watchdog.Stop()
			}
		}
		if _, err := peer.Write(buf[:n]); err != nil {
			return
		}
		if held {
			r.forwarded <- struct{}{}
		}
	}
}

func TestStartMonitorsLockProbeBudget(t *testing.T) {
	for _, short := range []bool{false, true} {
		name := "normal budget survives a delayed reply"
		if short {
			name = "short budget closes the socket and releases the lock"
		}
		t.Run(name, func(t *testing.T) {
			cfg, obs := appConfig(t)
			relay := installProbeRelay(t, cfg)
			budgets := testBudgets()
			if short {
				budgets.probe = 200 * time.Millisecond
			}
			server := started(t, launchApp(t, cfg, budgets, io.Discard))
			client := appClient(t)
			requireHealthy(t, client, cfg)
			readPID := func() int32 {
				return storetest.Scalar[int32](t, obs, `
SELECT COALESCE(max(pid), 0) FROM pg_stat_activity
WHERE datname = current_database() AND application_name = $1`, instanceApplication)
			}
			pid := readPID()
			if pid == 0 {
				t.Fatal("Readiness did not hold a dedicated PostgreSQL backend.")
			}

			relay.arm.Store(true)
			arrived := storetest.Receive(t, "the next instance probe reply at the relay", relay.arrived)
			if !short {
				timer := time.NewTimer(time.Until(arrived.Add(1500 * time.Millisecond)))
				defer timer.Stop()
				select {
				case <-relay.gone:
					t.Fatal("The dedicated socket closed during a delay inside the normal probe budget.")
				case <-timer.C:
				}
				if readPID() != pid {
					t.Error("The app dropped or replaced its instance backend while the reply was held.")
				}
				requireHealthy(t, client, cfg)
				close(relay.release)
				storetest.Receive(t, "the delayed reply being forwarded", relay.forwarded)
				requireHealthy(t, client, cfg)
				if readPID() != pid {
					t.Error("The app did not keep its instance backend after the delayed probe.")
				}
				return
			}

			deadline := arrived.Add(budgets.probe + time.Second)
			timer := time.NewTimer(time.Until(deadline))
			defer timer.Stop()
			select {
			case closedAt := <-relay.gone:
				if closedAt.After(deadline) {
					t.Error("The app closed the dedicated socket after the probe bound.")
				}
			case <-timer.C:
				t.Fatal("The app did not close the dedicated socket before the relay watchdog.")
			}
			requireWaitBy(t, server, deadline)
			requireClosed(t, cfg.ListenAddr)

			postgres := cfg.Postgres.Copy()
			postgres.BeforeConnect = nil
			s, err := store.Open(t.Context(), postgres, store.ConnectionPolicy{})
			if err != nil {
				t.Fatal("Could not open a new store after the failed probe.")
			}
			defer s.Close()
			lock, err := s.AcquireServerInstance(t.Context(), time.Second)
			if err != nil || lock == nil {
				t.Fatal("A new instance could not acquire the lock after the failed probe.")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := lock.Close(ctx); err != nil {
				t.Error("Closing the replacement lock failed.")
			}
		})
	}
}
