package linktest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func requirePostgres(t *testing.T) {
	t.Helper()
	if os.Getenv("LGTM_TEST_POSTGRES") == "" {
		t.Skip("Set LGTM_TEST_POSTGRES through deploy/test/with-postgres.sh to run this combined link test.")
	}
}

func waitFor(t *testing.T, within time.Duration, description string, condition func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), within)
	defer cancel()
	ticks := time.NewTicker(25 * time.Millisecond)
	defer ticks.Stop()
	for !condition() {
		select {
		case <-ticks.C:
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for %s.", description)
		}
	}
}

func receive[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case item := <-ch:
		return item
	case <-time.After(15 * time.Second):
		t.Fatalf("Timed out waiting for %s.", what)
		var zero T
		return zero
	}
}

type diagnostic struct {
	Message  string `json:"msg"`
	Code     string `json:"code"`
	Sequence uint64 `json:"sequence"`
}

type diagnostics struct {
	mu      sync.Mutex
	entries []diagnostic
	marker  string
	marked  bool
}

func (d *diagnostics) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.marker != "" && strings.Contains(string(p), d.marker) {
		d.marked = true
	}
	var entry diagnostic
	if json.Unmarshal(p, &entry) == nil {
		d.entries = append(d.entries, entry)
	}
	return len(p), nil
}

func (d *diagnostics) has(message, code string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, entry := range d.entries {
		if entry.Message == message && entry.Code == code {
			return true
		}
	}
	return false
}

func (d *diagnostics) bufferedAfter(mark uint64) []uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	var result []uint64
	for _, entry := range d.entries {
		if entry.Message == "link_event_buffered" && entry.Sequence > mark {
			result = append(result, entry.Sequence)
		}
	}
	return result
}

func (d *diagnostics) joinFailureCode() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, entry := range d.entries {
		switch entry.Code {
		case "state_open_failed", "store_failed", "identity_save_failed", "join_timeout", "join_failed",
			"frame_debug_enabled", "invalid_reply", "token_unknown", "token_refused", "origin_mismatch",
			"invalid_token", "server_key_mismatch":
			return entry.Code
		}
	}
	return "unrecognized"
}

func newClient(t *testing.T, logs *diagnostics) clienttest.Options {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("Could not resolve the disposable client home.")
	}
	return clienttest.Options{
		StateDir: filepath.Join(home, "state"),
		Version:  "0.0.0-linktest",
		Stderr:   logs,
		Getenv: func(name string) string {
			if name == "HOME" {
				return home
			}
			return ""
		},
	}
}

type runningClient struct {
	cancel context.CancelFunc
	done   chan struct{}
	code   int
}

func startClient(t *testing.T, opts clienttest.Options) *runningClient {
	t.Helper()
	return startClientExpecting(t, opts, 0)
}

func startClientExpecting(t *testing.T, opts clienttest.Options, code int) *runningClient {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	run := &runningClient{cancel: cancel, done: make(chan struct{})}
	go func() {
		run.code = clienttest.Run(ctx, opts)
		close(run.done)
	}()
	t.Cleanup(func() {
		run.cancel()
		run.expectExit(t, code)
	})
	return run
}

func (r *runningClient) stop(t *testing.T) {
	t.Helper()
	r.cancel()
	r.expectCleanExit(t)
}

func (r *runningClient) expectCleanExit(t *testing.T) {
	t.Helper()
	r.expectExit(t, 0)
}

func (r *runningClient) expectExit(t *testing.T, code int) {
	t.Helper()
	receive(t, r.done, "the client to exit and release its state")
	if r.code != code {
		t.Fatalf("The client exited with status %d, want %d.", r.code, code)
	}
}

func issueToken(t *testing.T, clients lgtmv1connect.ClientServiceClient, address string) string {
	t.Helper()
	result, err := clients.CreateClientToken(t.Context(), connect.NewRequest(&webv1.CreateClientTokenRequest{}))
	if err != nil {
		t.Fatal("Could not issue a fixture enrollment token.")
	}
	token := result.Msg.GetToken()
	if address == result.Msg.GetAddress() {
		return token
	}
	// The relay listens on a different loopback port. Keep the issued secret and key pin.
	const prefix = "lgtm1_"
	wire, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, prefix))
	var decoded linkv1.JoinToken
	if err != nil || !strings.HasPrefix(token, prefix) || proto.Unmarshal(wire, &decoded) != nil {
		t.Fatal("Could not read the fixture token for the loopback relay.")
	}
	decoded.Address = address
	wire, err = proto.Marshal(&decoded)
	if err != nil {
		t.Fatal("Could not encode the fixture token for the loopback relay.")
	}
	return prefix + base64.RawURLEncoding.EncodeToString(wire)
}

func join(t *testing.T, clients lgtmv1connect.ClientServiceClient, opts clienttest.Options, address string) string {
	t.Helper()
	token := issueToken(t, clients, address)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	if clienttest.Join(ctx, opts, address, token) != nil {
		t.Fatalf("The real client did not complete enrollment (%s).", opts.Stderr.(*diagnostics).joinFailureCode())
	}
	return readIdentity(t, opts).MachineID
}

type savedIdentity struct {
	MachineID       string `json:"machineId"`
	ClientPublicKey []byte `json:"clientPublicKey"`
	ServerPublicKey []byte `json:"serverPublicKey"`
}

func readIdentity(t *testing.T, opts clienttest.Options) savedIdentity {
	t.Helper()
	path := filepath.Join(opts.StateDir, "identity.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatal("The client did not save one private identity file.")
	}
	body, err := os.ReadFile(path)
	var saved savedIdentity
	if err != nil || json.Unmarshal(body, &saved) != nil || saved.MachineID == "" {
		t.Fatal("Could not read the enrolled machine identity.")
	}
	return saved
}

func approve(t *testing.T, clients lgtmv1connect.ClientServiceClient, id, name string) {
	t.Helper()
	if _, err := clients.ApproveClient(t.Context(), connect.NewRequest(&webv1.ApproveClientRequest{Id: id, Name: name})); err != nil {
		t.Fatal("The website service did not approve the machine.")
	}
}

func remove(t *testing.T, clients lgtmv1connect.ClientServiceClient, id string) {
	t.Helper()
	if _, err := clients.RemoveClient(t.Context(), connect.NewRequest(&webv1.RemoveClientRequest{Id: id})); err != nil {
		t.Fatal("The website service did not remove the machine.")
	}
}

func snapshot(ctx context.Context, clients lgtmv1connect.ClientServiceClient) ([]*webv1.Client, bool) {
	// The first authorized request after a restart may need to derive the session binding.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	watch, err := clients.WatchClients(ctx, connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err != nil {
		return nil, false
	}
	defer watch.Close()
	if !watch.Receive() || watch.Msg().GetAll() == nil {
		return nil, false
	}
	return watch.Msg().GetAll().GetClients(), true
}

func waitForClient(t *testing.T, clients lgtmv1connect.ClientServiceClient, id string, state webv1.ClientState) *webv1.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 29*time.Second)
	defer cancel()
	ticks := time.NewTicker(25 * time.Millisecond)
	defer ticks.Stop()
	for {
		rows, _ := snapshot(ctx, clients)
		for _, row := range rows {
			if row.GetId() == id && row.GetState() == state {
				return row
			}
		}
		select {
		case <-ticks.C:
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for the machine to reach %s in WatchClients.", state)
		}
	}
}

func openObserver(t *testing.T, server *servertest.Server) *pgx.Conn {
	t.Helper()
	env := server.Environment()
	password, err := os.ReadFile(env["LGTM_POSTGRES_PASSWORD_FILE"])
	if err != nil {
		t.Fatal("Could not read the disposable database credential.")
	}
	dsn := url.URL{
		Scheme:   "postgresql",
		Host:     net.JoinHostPort(env["LGTM_POSTGRES_HOST"], env["LGTM_POSTGRES_PORT"]),
		User:     url.UserPassword(env["LGTM_POSTGRES_USER"], strings.TrimSuffix(string(password), "\n")),
		Path:     "/" + env["LGTM_POSTGRES_DATABASE"],
		RawQuery: "sslmode=disable",
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	db, err := pgx.Connect(ctx, dsn.String())
	if err != nil {
		t.Fatal("Could not open the independent PostgreSQL observer.")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Close(ctx)
	})
	return db
}

func execute(t *testing.T, db *pgx.Conn, statement string, args ...any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := db.Exec(ctx, statement, args...); err != nil {
		t.Fatal("The disposable database could not execute a test statement.")
	}
}

func scalar[T any](t *testing.T, db *pgx.Conn, statement string, args ...any) T {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var value T
	if err := db.QueryRow(ctx, statement, args...).Scan(&value); err != nil {
		t.Fatal("The database observer could not read a test value.")
	}
	return value
}
