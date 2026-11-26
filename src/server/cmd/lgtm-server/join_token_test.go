package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

var (
	joinTokenLine   = regexp.MustCompile(`^lgtm1_[A-Za-z0-9_-]+\n$`)
	joinTokenUUIDv7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func TestJoinTokenPrintsOneTokenLine(t *testing.T) {
	server := startJoinTokenServer(t)
	requireJoinTokenLine(t, server.Environment(), nil)
}

func TestJoinTokenCarriesAddressAndServerKey(t *testing.T) {
	server := startJoinTokenServer(t)
	env := server.Environment()
	// Keep the public address distinct from the listen address.
	env["LGTM_LISTEN_ADDR"] = "127.0.0.1:1"
	token := issueJoinToken(t, env)

	if token.GetAddress() != server.URL {
		t.Error("The token address differs from the public address of the running server.")
	}
	if !joinTokenUUIDv7.MatchString(token.GetTokenId()) {
		t.Error("The token identifier is not a version 7 universally unique identifier.")
	}
	if size := len(token.GetSecret()); size != 32 {
		t.Errorf("The token secret has %d bytes, want 32.", size)
	}
	wantKeyHash := sha256.Sum256(server.SigningPublicKey)
	if !bytes.Equal(token.GetServerKeySha256(), wantKeyHash[:]) {
		t.Error("The token signing key fingerprint differs from the fingerprint of the running server.")
	}
	if size := len(token.GetCertificateKeySha256()); size != 0 {
		t.Errorf("The loopback token certificate fingerprint has %d bytes, want zero.", size)
	}
}

func TestJoinTokenJoinsOnce(t *testing.T) {
	server := startJoinTokenServer(t)
	token := issueJoinToken(t, server.Environment())

	joinMachineWithToken(t, server, token, "first-machine")
	_, err := server.Link().Join(t.Context(), joinTokenRequest(t, token, "second-machine"))
	var refusal *connect.Error
	if !errors.As(err, &refusal) || refusal.Code() != connect.CodePermissionDenied {
		t.Fatalf("The second join with a new key returned %v, want permission_denied.", connect.CodeOf(err))
	}
	if refusal.Meta().Get("Lgtm-Join-Refusal") != "token_used" {
		t.Error("The second join with a new key did not report token_used.")
	}
}

func TestJoinTokenHostsServerFlag(t *testing.T) {
	server := startJoinTokenServer(t)
	hostToken := issueJoinToken(t, server.Environment(), "--hosts-server")
	remoteToken := issueJoinToken(t, server.Environment())
	hostID := joinMachineWithToken(t, server, hostToken, "server-machine")
	remoteID := joinMachineWithToken(t, server, remoteToken, "remote-machine")
	if hostID == remoteID {
		t.Fatal("The two joins returned the same machine identifier.")
	}

	watch, err := server.Clients(t).WatchClients(t.Context(), connect.NewRequest(&lgtmv1.WatchClientsRequest{}))
	if err != nil {
		t.Fatal("The signed-in client could not open the machine watch.")
	}
	defer watch.Close()
	if !watch.Receive() || watch.Msg().GetAll() == nil {
		t.Fatal("The machine watch did not send a snapshot as its first message.")
	}
	clients := watch.Msg().GetAll().GetClients()
	if len(clients) != 2 {
		t.Fatalf("The first machine snapshot contains %d machines, want two.", len(clients))
	}
	wantHostsServer := map[string]bool{hostID: true, remoteID: false}
	for _, client := range clients {
		id := client.GetId()
		want, found := wantHostsServer[id]
		if !found {
			t.Fatal("The first machine snapshot contains an unexpected or repeated machine.")
		}
		if client.GetState() != lgtmv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL {
			t.Error("A newly joined machine is not waiting for approval.")
		}
		if client.GetHostsServer() != want {
			t.Errorf("The machine has hosts_server=%t, want %t.", client.GetHostsServer(), want)
		}
		delete(wantHostsServer, id)
	}
}

func TestJoinTokenRefusesUnknownArgument(t *testing.T) {
	server := startJoinTokenServer(t)
	env := server.Environment()
	const usage = "usage: lgtm-server join-token [--hosts-server]\n"

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"address override", []string{"--address", "x"}},
		{"flag with a value", []string{"--hosts-server=yes"}},
		{"repeated flag", []string{"--hosts-server", "--hosts-server"}},
		{"bare word", []string{"lgtm1_marker"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := joinTokenCommand(t.Context(), tc.args, joinTokenLookup(env), &stdout, &stderr); code != 2 {
				t.Errorf("The command exited with status %d, want 2 for invalid arguments.", code)
			}
			if stdout.Len() != 0 {
				t.Errorf("The command wrote %d bytes to standard output for invalid arguments.", stdout.Len())
			}
			if stderr.String() != usage {
				t.Errorf("Standard error contains %d bytes and does not match the fixed usage line.", stderr.Len())
			}
		})
	}
}

func TestJoinTokenFailsWithoutDatabase(t *testing.T) {
	server := startJoinTokenServer(t)
	env := server.Environment()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("The test could not reserve a loopback port.")
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal("The test could not close the loopback port.")
	}
	env["LGTM_POSTGRES_HOST"] = "127.0.0.1"
	env["LGTM_POSTGRES_PORT"] = strconv.Itoa(port)

	var stdout, stderr bytes.Buffer
	if code := joinTokenCommand(t.Context(), nil, joinTokenLookup(env), &stdout, &stderr); code != 1 {
		t.Errorf("The command exited with status %d, want 1 when PostgreSQL is unreachable.", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("The command wrote %d bytes to standard output with no database.", stdout.Len())
	}
	requireJoinTokenDiagnostic(t, stderr.String())
	for name, value := range env {
		if value != "" && strings.Contains(stderr.String(), value) {
			t.Errorf("The diagnostic includes the value of %s.", name)
		}
	}
}

func TestJoinTokenFailsOnWriteError(t *testing.T) {
	server := startJoinTokenServer(t)
	var stdout failingJoinTokenWriter
	var stderr bytes.Buffer
	if code := joinTokenCommand(t.Context(), nil, joinTokenLookup(server.Environment()), &stdout, &stderr); code != 1 {
		t.Errorf("The command exited with status %d, want 1 after a failed output write.", code)
	}
	if !stdout.wrote {
		t.Error("The command did not attempt to write the token to standard output.")
	}
	requireJoinTokenDiagnostic(t, stderr.String())
	if strings.Contains(stderr.String(), joinTokenWriteError) {
		t.Error("The diagnostic includes the underlying writer error.")
	}
}

func startJoinTokenServer(t *testing.T) *servertest.Server {
	t.Helper()
	return servertest.Start(t, servertest.Options{Password: "fake-join-token-test-password"})
}

func joinTokenLookup(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

func requireJoinTokenLine(t *testing.T, env map[string]string, args []string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := joinTokenCommand(t.Context(), args, joinTokenLookup(env), &stdout, &stderr)
	if !joinTokenLine.Match(stdout.Bytes()) {
		t.Fatalf("Standard output has %d bytes, want exactly one join token line.", stdout.Len())
	}
	if code != 0 {
		t.Errorf("The command exited with status %d, want 0.", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("Standard error has %d bytes, want zero.", stderr.Len())
	}
	return strings.TrimSuffix(stdout.String(), "\n")
}

func issueJoinToken(t *testing.T, env map[string]string, args ...string) *linkv1.JoinToken {
	t.Helper()
	token, err := machines.DecodeToken(requireJoinTokenLine(t, env, args))
	if err != nil {
		t.Fatal("The command did not print a decodable join token.")
	}
	return token
}

func joinTokenRequest(t *testing.T, token *linkv1.JoinToken, host string) *connect.Request[linkv1.JoinRequest] {
	t.Helper()
	key, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("The test could not generate a machine key.")
	}
	return connect.NewRequest(&linkv1.JoinRequest{
		TokenId:          token.GetTokenId(),
		TokenSecret:      token.GetSecret(),
		MachinePublicKey: key,
		HostName:         host,
	})
}

func joinMachineWithToken(t *testing.T, server *servertest.Server, token *linkv1.JoinToken, host string) string {
	t.Helper()
	reply, err := server.Link().Join(t.Context(), joinTokenRequest(t, token, host))
	if err != nil {
		t.Fatalf("The new machine could not join with the token. The join returned %v.", connect.CodeOf(err))
	}
	id := reply.Msg.GetMachineId()
	if id == "" {
		t.Fatal("The successful join did not return a machine identifier.")
	}
	return id
}

func requireJoinTokenDiagnostic(t *testing.T, output string) {
	t.Helper()
	line, ok := strings.CutSuffix(output, ".\n")
	if !ok || strings.TrimSpace(line) == "" || strings.ContainsAny(line, "\r\n") {
		t.Fatalf("Standard error has %d bytes, want one sentence ending with a full stop and a line feed.", len(output))
	}
	if strings.Contains(line, "lgtm1_") {
		t.Error("Standard error contains a join token marker.")
	}
}

const joinTokenWriteError = "join token test writer error marker"

type failingJoinTokenWriter struct {
	wrote bool
}

func (w *failingJoinTokenWriter) Write(p []byte) (int, error) {
	w.wrote = w.wrote || len(p) != 0
	return 0, errors.New(joinTokenWriteError)
}
