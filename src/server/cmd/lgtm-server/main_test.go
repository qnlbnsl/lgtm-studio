package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/app"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault/vaulttest"
)

const (
	commandPassword  = "fake-command-owner-password"
	privateDetail    = "fake-command-private-detail"
	passwordInvalid  = "LGTM_WEB_PASSWORD is missing or invalid."
	reconcileFailed  = "Session reconciliation did not complete."
	purgeUnconfirmed = "Session revocation was not confirmed."
	purgeCommitted   = "Session revocation committed."
)

func lookupMap(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

func loopbackAddress(t *testing.T) (address, port string) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("Could not reserve a loopback command port.")
	}
	address = listener.Addr().String()
	port = strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	_ = listener.Close()
	return address, port
}

func removalSettings(t *testing.T, cfg *pgxpool.Config) map[string]string {
	t.Helper()
	c := cfg.ConnConfig
	file := filepath.Join(t.TempDir(), "postgres.password")
	if err := os.WriteFile(file, []byte(c.Password+"\n"), 0o400); err != nil {
		t.Fatal("Could not write the owner-only PostgreSQL fixture file.")
	}
	return map[string]string{
		"LGTM_WEB_PASSWORD":           "",
		"LGTM_ALLOW_PRIVATE_STACK":    "true",
		"LGTM_POSTGRES_HOST":          c.Host,
		"LGTM_POSTGRES_PORT":          strconv.Itoa(int(c.Port)),
		"LGTM_POSTGRES_USER":          c.User,
		"LGTM_POSTGRES_DATABASE":      c.Database,
		"LGTM_POSTGRES_PASSWORD_FILE": file,
	}
}

func commandSettings(t *testing.T, cfg *pgxpool.Config) map[string]string {
	t.Helper()
	env := removalSettings(t, cfg)
	env["LGTM_LISTEN_ADDR"], _ = loopbackAddress(t)
	env["LGTM_WEB_PASSWORD"] = commandPassword
	env["LGTM_OPENBAO_ADDR"], env["LGTM_OPENBAO_CREDENTIAL_FILE"] = vaulttest.Start(t, [32]byte{31})
	return env
}

func commandObserver(t *testing.T, cfg *pgxpool.Config) *pgx.Conn {
	t.Helper()
	conn, err := pgx.ConnectConfig(t.Context(), cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatal("Could not connect to the command fixture database.")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = conn.Close(ctx)
	})
	return conn
}

func commandStore(t *testing.T, cfg *pgxpool.Config) *store.Store {
	t.Helper()
	s, err := store.Open(t.Context(), cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("Could not open the command fixture store.")
	}
	if err := s.Migrate(t.Context(), auth.Migrations(), vault.Migrations()); err != nil {
		s.Close()
		t.Fatal("Could not migrate the command fixture store.")
	}
	return s
}

func seedCommandSession(t *testing.T, cfg *pgxpool.Config) {
	t.Helper()
	s := commandStore(t, cfg)
	defer s.Close()
	password, err := auth.NewPassword(commandPassword)
	if err != nil {
		t.Fatal("Could not create the command fixture password.")
	}
	if _, err := auth.NewSessions(s, password).Issue(t.Context()); err != nil {
		t.Fatal("Could not issue a command fixture session.")
	}
}

type commandResult struct {
	code           int
	stdout, stderr string
}

func callRun(ctx context.Context, args []string, lookup func(string) (string, bool)) commandResult {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := run(ctx, args, lookup, &stdout, &stderr)
	return commandResult{code, stdout.String(), stderr.String()}
}

func requireCommandFailure(t *testing.T, result commandResult, private []string, want ...string) {
	t.Helper()
	if result.code != 1 {
		t.Errorf("The startup command returned exit %d, want 1.", result.code)
	}
	if result.stdout != "" {
		t.Error("A refused startup wrote to standard output.")
	}
	for _, value := range private {
		if value != "" && strings.Contains(result.stderr, value) {
			t.Error("A startup diagnostic disclosed a private fixture value.")
		}
	}
	got := strings.Split(strings.TrimSuffix(result.stderr, "\n"), "\n")
	if !slices.Equal(got, want) {
		t.Errorf("The startup diagnostics did not match the expected lines in order: %q.", want)
	}
}

func requireCommandSessions(t *testing.T, obs *pgx.Conn, count int) {
	t.Helper()
	if got := storetest.Scalar[int](t, obs, "SELECT count(*) FROM web_sessions"); got != count {
		t.Errorf("The database holds %d sessions, want %d.", got, count)
	}
}

func TestStartupDiagnostics(t *testing.T) {
	t.Run("unusable password and malformed database settings", func(t *testing.T) {
		result := callRun(t.Context(), nil, lookupMap(map[string]string{
			"LGTM_POSTGRES_PORT": privateDetail, "LGTM_WEB_PASSWORD": strings.Repeat("x", 4097),
		}))
		requireCommandFailure(t, result, []string{privateDetail, strings.Repeat("x", 4097)},
			"The PostgreSQL settings are invalid.", reconcileFailed, purgeUnconfirmed, passwordInvalid)
	})

	t.Run("database connection refused", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		env := removalSettings(t, cfg)
		_, env["LGTM_POSTGRES_PORT"] = loopbackAddress(t)
		result := callRun(t.Context(), nil, lookupMap(env))
		requireCommandFailure(t, result, []string{cfg.ConnConfig.Password},
			"The database connection failed, check the PostgreSQL connection settings.",
			reconcileFailed, purgeUnconfirmed, passwordInvalid)
	})

	t.Run("admission canceled after waiting", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		obs := commandObserver(t, cfg)
		env := removalSettings(t, cfg)
		release := storetest.HoldAdvisoryGate(t, obs, 841941100003)
		defer release()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		task := storetest.Start(t, func() commandResult { return callRun(ctx, nil, lookupMap(env)) })

		storetest.Poll(t, "the startup command waiting for the instance lock", func() bool {
			select {
			case <-task.Done():
				t.Fatal("The command returned before the dedicated backend waited for admission.")
			default:
			}
			return storetest.Scalar[bool](t, obs, `
SELECT EXISTS (
    SELECT 1 FROM pg_locks l JOIN pg_stat_activity a USING (pid)
    WHERE a.application_name = 'lgtm-server-instance' AND NOT l.granted
      AND l.locktype = 'advisory' AND $1 = ANY(pg_blocking_pids(l.pid))
)`, int32(obs.PgConn().PID()))
		})
		cancel()
		requireCommandFailure(t, task.Wait(t), []string{cfg.ConnConfig.Password},
			"Server startup was canceled.", reconcileFailed, purgeUnconfirmed, passwordInvalid)
	})

	t.Run("only the admission deadline reports a busy instance", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		obs := commandObserver(t, cfg)
		release := storetest.HoldAdvisoryGate(t, obs, 841941100003)
		defer release()
		result := callRun(t.Context(), nil, lookupMap(removalSettings(t, cfg)))
		requireCommandFailure(t, result, []string{cfg.ConnConfig.Password},
			"Another server is already using this database.", reconcileFailed, purgeUnconfirmed, passwordInvalid)
	})

	t.Run("instance socket refused after the store opens", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		obs := commandObserver(t, cfg)
		db := pgx.Identifier{cfg.ConnConfig.Database}.Sanitize()
		// Only the observer and the Store's initial ping connection fit.
		storetest.Exec(t, obs, "ALTER DATABASE "+db+" CONNECTION LIMIT 2")
		result := callRun(t.Context(), nil, lookupMap(removalSettings(t, cfg)))
		requireCommandFailure(t, result, []string{cfg.ConnConfig.Password, cfg.ConnConfig.Database},
			"The server could not acquire its PostgreSQL instance lock.", reconcileFailed, purgeUnconfirmed, passwordInvalid)
	})

	t.Run("migration failure", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		s := commandStore(t, cfg)
		s.Close()
		obs := commandObserver(t, cfg)
		storetest.Exec(t, obs, "UPDATE schema_migrations SET version = 9999 WHERE part = 'auth'")
		result := callRun(t.Context(), nil, lookupMap(removalSettings(t, cfg)))
		requireCommandFailure(t, result, []string{cfg.ConnConfig.Password},
			"The server could not migrate its PostgreSQL database.", reconcileFailed, purgeUnconfirmed, passwordInvalid)
	})

	t.Run("expired startup context", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
		defer cancel()
		result := callRun(ctx, nil, lookupMap(removalSettings(t, cfg)))
		requireCommandFailure(t, result, []string{cfg.ConnConfig.Password},
			"The startup deadline expired.", reconcileFailed, purgeUnconfirmed, passwordInvalid)
	})

	for _, abort := range []bool{false, true} {
		name := "removal uses only the seven launcher settings"
		if abort {
			name = "aborted removal does not claim revocation"
		}
		t.Run(name, func(t *testing.T) {
			cfg := storetest.NewDatabase(t, "public")
			seedCommandSession(t, cfg)
			obs := commandObserver(t, cfg)
			requireCommandSessions(t, obs, 1)
			outcome := []string{purgeCommitted, passwordInvalid}
			remaining := 0
			if abort {
				storetest.Exec(t, obs, `
CREATE FUNCTION refuse_command_purge() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fake-command-private-detail'; END $$;
CREATE CONSTRAINT TRIGGER refuse_command_purge AFTER DELETE ON web_sessions
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION refuse_command_purge()`)
				outcome = []string{reconcileFailed, purgeUnconfirmed, passwordInvalid}
				remaining = 1
			}
			result := callRun(t.Context(), nil, lookupMap(removalSettings(t, cfg)))
			requireCommandFailure(t, result, []string{cfg.ConnConfig.Password, commandPassword, privateDetail}, outcome...)
			requireCommandSessions(t, obs, remaining)
		})
	}

	t.Run("OpenBao refused the server connection", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		env := commandSettings(t, cfg)
		closed, _ := loopbackAddress(t)
		env["LGTM_OPENBAO_ADDR"] = "http://" + closed
		result := callRun(t.Context(), nil, lookupMap(env))
		requireCommandFailure(t, result, []string{cfg.ConnConfig.Password, commandPassword, closed},
			"The server could not authenticate to OpenBao.")
	})

	t.Run("OpenBao signing key pin cannot commit", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		s := commandStore(t, cfg)
		s.Close()
		obs := commandObserver(t, cfg)
		storetest.Exec(t, obs, `
CREATE FUNCTION refuse_command_pin() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fake-command-private-detail'; END $$;
CREATE CONSTRAINT TRIGGER refuse_command_pin AFTER INSERT ON server_link_identity
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION refuse_command_pin()`)
		result := callRun(t.Context(), nil, lookupMap(commandSettings(t, cfg)))
		requireCommandFailure(t, result, []string{cfg.ConnConfig.Password, commandPassword, privateDetail},
			"The server could not establish its OpenBao signing key.")
		if count := storetest.Scalar[int](t, obs, "SELECT count(*) FROM server_link_identity"); count != 0 {
			t.Error("The failed first pin left an identity in the database.")
		}
	})

	t.Run("established identity cannot be replaced", func(t *testing.T) {
		cfg := storetest.NewDatabase(t, "public")
		env := commandSettings(t, cfg)
		s := commandStore(t, cfg)
		client, err := vault.Open(env["LGTM_OPENBAO_ADDR"], env["LGTM_OPENBAO_CREDENTIAL_FILE"], true)
		if err != nil {
			s.Close()
			t.Fatal("Could not open the original OpenBao fixture.")
		}
		pinned, err := vault.LoadLinkIdentity(t.Context(), s, client, true)
		s.Close()
		if err != nil {
			t.Fatal("Could not establish the original signing identity.")
		}
		env["LGTM_OPENBAO_ADDR"], env["LGTM_OPENBAO_CREDENTIAL_FILE"] = vaulttest.Start(t, [32]byte{42})
		result := callRun(t.Context(), nil, lookupMap(env))
		requireCommandFailure(t, result, []string{cfg.ConnConfig.Password, commandPassword, "fixture-batch-token", "fixture-secret"},
			"Restore the established OpenBao signing key before restarting the server.")
		obs := commandObserver(t, cfg)
		if storetest.Scalar[string](t, obs, "SELECT fingerprint FROM server_link_identity WHERE singleton") != pinned.Fingerprint {
			t.Error("The command changed the established fingerprint.")
		}
	})

	for _, retired := range []string{"LGTM_POSTGRES_URL", "LGTM_SERVER_LISTEN_ADDR"} {
		t.Run("retired "+retired, func(t *testing.T) {
			result := callRun(t.Context(), nil, lookupMap(map[string]string{
				retired: privateDetail, "LGTM_POSTGRES_PORT": "invalid",
			}))
			var rest []string
			warnings := 0
			for _, line := range strings.Split(strings.TrimSuffix(result.stderr, "\n"), "\n") {
				if strings.Contains(line, retired) && !strings.Contains(line, privateDetail) {
					warnings++
				} else {
					rest = append(rest, line)
				}
			}
			if warnings != 1 {
				t.Fatal("The command did not issue one redacted warning naming the retired variable.")
			}
			result.stderr = strings.Join(rest, "\n")
			requireCommandFailure(t, result, []string{privateDetail},
				"The PostgreSQL settings are invalid.", reconcileFailed, purgeUnconfirmed, passwordInvalid)
		})
	}

	for _, scenario := range []struct {
		name, setting, value, want string
	}{
		{"invalid listener", "LGTM_LISTEN_ADDR", privateDetail, "LGTM_LISTEN_ADDR"},
		{"invalid origin", "LGTM_ALLOWED_ORIGINS", privateDetail, "LGTM_ALLOWED_ORIGINS"},
		{"invalid vault address", "LGTM_OPENBAO_ADDR", privateDetail, "LGTM_OPENBAO_ADDR"},
		{"invalid vault credential", "LGTM_OPENBAO_CREDENTIAL_FILE", privateDetail, "LGTM_OPENBAO_CREDENTIAL_FILE"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cfg := storetest.NewDatabase(t, "public")
			seedCommandSession(t, cfg)
			env := commandSettings(t, cfg)
			env["LGTM_WEB_PASSWORD"] = "fake-command-replacement-password"
			env["LGTM_PUBLIC_URL"] = "http://" + env["LGTM_LISTEN_ADDR"]
			env[scenario.setting] = scenario.value
			result := callRun(t.Context(), nil, lookupMap(env))
			if result.code != 1 || result.stdout != "" || !strings.Contains(result.stderr, scenario.want) ||
				strings.Contains(result.stderr, privateDetail) || strings.Contains(result.stderr, env["LGTM_WEB_PASSWORD"]) {
				t.Error("The command did not refuse the invalid setting with a redacted diagnostic naming it.")
			}
			requireCommandSessions(t, commandObserver(t, cfg), 0)
		})
	}
}

func healthResult(t *testing.T, ctx context.Context, address string) commandResult {
	t.Helper()
	var readListen, readOther bool
	result := callRun(ctx, []string{"health"}, func(name string) (string, bool) {
		if name == "LGTM_LISTEN_ADDR" {
			readListen = true
			return address, true
		}
		readOther = true
		return "", false
	})
	if !readListen || readOther {
		t.Error("The health command must read only LGTM_LISTEN_ADDR.")
	}
	return result
}

func TestHealthCommand(t *testing.T) {
	t.Run("no proxy lookup", func(t *testing.T) {
		var requests atomic.Int32
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer proxy.Close()
		t.Setenv("HTTP_PROXY", proxy.URL)
		t.Setenv("http_proxy", proxy.URL)
		t.Setenv("NO_PROXY", "")
		t.Setenv("no_proxy", "")
		t.Setenv("REQUEST_METHOD", "")

		// A documentation address bypasses the loopback exemption of Go's proxy rules.
		if got := healthResult(t, t.Context(), "192.0.2.1:8080"); got.code != 1 || requests.Load() != 0 {
			t.Error("The health command contacted the environment proxy.")
		}
	})

	t.Run("wildcard maps to a ready local app", func(t *testing.T) {
		env := commandSettings(t, storetest.NewDatabase(t, "public"))
		cfg, err := config.Load(lookupMap(env))
		if err != nil {
			t.Fatal("Could not load the application health fixture settings.")
		}
		server, err := app.Start(t.Context(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err != nil || server == nil || server.Addr() == nil {
			t.Fatal("The app did not start for the command health check.")
		}
		defer server.Close()
		_, port, err := net.SplitHostPort(server.Addr().String())
		if err != nil {
			t.Fatal("The health fixture did not bind to a TCP port.")
		}
		for _, host := range []string{"", "0.0.0.0"} {
			if got := healthResult(t, t.Context(), net.JoinHostPort(host, port)); got.code != 0 {
				t.Error("The health command failed to reach the ready app through a wildcard address.")
			}
		}
	})

	t.Run("closed port", func(t *testing.T) {
		address, _ := loopbackAddress(t)
		if result := healthResult(t, t.Context(), address); result.code != 1 {
			t.Error("The health command did not fail when the port was closed.")
		}
	})

	for _, status := range []int{http.StatusOK, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var reached, redirected atomic.Bool
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirected" {
					redirected.Store(true)
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
					reached.Store(true)
				}
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(status)
			}))
			defer fixture.Close()
			if got := healthResult(t, t.Context(), fixture.Listener.Addr().String()); got.code != 1 {
				t.Error("The health command accepted a response other than 204.")
			}
			if !reached.Load() || redirected.Load() {
				t.Error("The health command did not make exactly the initial GET /healthz without following a redirect.")
			}
		})
	}

	t.Run("stalled reply has a two second bound", func(t *testing.T) {
		arrived := make(chan time.Time, 1)
		release := make(chan struct{})
		fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			arrived <- time.Now()
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		defer fixture.Close()
		defer close(release)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		task := storetest.Start(t, func() commandResult {
			return callRun(ctx, []string{"health"}, lookupMap(map[string]string{"LGTM_LISTEN_ADDR": fixture.Listener.Addr().String()}))
		})
		var start time.Time
		select {
		case start = <-arrived:
		case <-task.Done():
			t.Fatal("The command returned before it sent the stalled health request.")
		case <-time.After(storetest.ObservationTimeout):
			t.Fatal("The health request did not reach the stalled handler.")
		}
		timer := time.NewTimer(time.Until(start.Add(3 * time.Second)))
		defer timer.Stop()
		select {
		case <-task.Done():
			if got := task.Result(); got.code != 1 {
				t.Error("A stalled health reply must fail the health command.")
			}
		case <-timer.C:
			t.Fatal("The health command outlived its two-second request limit.")
		}
	})

	t.Run("invalid command syntax", func(t *testing.T) {
		for _, args := range [][]string{{"unknown-fixture-command"}, {"health", "unexpected-fixture-argument"}} {
			got := callRun(t.Context(), args, lookupMap(nil))
			if got.code != 2 {
				t.Error("Invalid command syntax did not return status 2.")
			}
		}
	})
}
