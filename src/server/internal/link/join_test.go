package link_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const refusalKey = "Lgtm-Join-Refusal"

var uuidV7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// sentences are the fixed sentences of the five refusals.
var sentences = map[string]string{
	"token_unknown":   "The server does not know this join token.",
	"token_expired":   "This join token has expired. Make a new one.",
	"token_revoked":   "This join token was revoked. Make a new one.",
	"token_used":      "This join token was already used. Make a new one.",
	"machine_removed": "The machine that used this join token was removed. Make a new token.",
}

func keyOf(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

// env is a migrated store, the tokens and the link service behind a cleartext HTTP/2 server.
type env struct {
	s         *store.Store
	obs       *pgx.Conn
	tokens    *machines.Tokens
	reg       *machines.Registry
	serverKey ed25519.PublicKey
	connect   linkv1connect.LinkServiceClient
	grpc      linkv1connect.LinkServiceClient
}

func newEnv(t *testing.T) *env {
	t.Helper()
	config := storetest.NewDatabase(t, "link_join")
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
	t.Cleanup(func() { _ = obs.Close(context.Background()) })

	serverKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	reg := machines.NewRegistry()
	tokens := machines.NewTokens(s, reg, machines.Issuer{PublicURL: "http://127.0.0.1:8787", ServerKey: serverKey})
	mux := http.NewServeMux()
	mux.Handle(linkv1connect.NewLinkServiceHandler(link.NewService(s, tokens, serverKey)))

	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server := httptest.NewUnstartedServer(mux)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	clientProtocols := new(http.Protocols)
	clientProtocols.SetUnencryptedHTTP2(true)
	client := &http.Client{Transport: &http.Transport{Protocols: clientProtocols}}

	return &env{
		s: s, obs: obs, tokens: tokens, reg: reg, serverKey: serverKey,
		connect: linkv1connect.NewLinkServiceClient(client, server.URL),
		grpc:    linkv1connect.NewLinkServiceClient(client, server.URL, connect.WithGRPC()),
	}
}

// newToken makes a token and decodes it. No test prints its text.
func (e *env) newToken(t *testing.T) (machines.NewToken, *linkv1.JoinToken) {
	t.Helper()
	tok, err := e.tokens.Create(t.Context(), machines.TokenOptions{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	msg, err := machines.DecodeToken(tok.Text)
	if err != nil {
		t.Fatalf("the token text does not decode: %v", err)
	}
	return tok, msg
}

func request(msg *linkv1.JoinToken, key []byte, host string) *connect.Request[linkv1.JoinRequest] {
	return connect.NewRequest(&linkv1.JoinRequest{
		TokenId: msg.GetTokenId(), TokenSecret: msg.GetSecret(), MachinePublicKey: key, HostName: host,
	})
}

func (e *env) machineCount(t *testing.T) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, e.obs, "SELECT count(*) FROM machines")
}

func (e *env) tokenState(t *testing.T) string {
	t.Helper()
	return storetest.Scalar[string](t, e.obs, "SELECT COALESCE(string_agg(row_to_json(c)::text, ';' ORDER BY id), '') FROM client_join_tokens c")
}

func requireCode(t *testing.T, what string, err error, want connect.Code) *connect.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: the call succeeded, want %v", what, want)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("%s: the error %v is no Connect error", what, err)
	}
	if ce.Code() != want {
		t.Fatalf("%s: the code is %v (%s), want %v", what, ce.Code(), ce.Message(), want)
	}
	return ce
}

// leaks lists the places in text where a part of the token's secret or text shows.
func leaks(text string, tok machines.NewToken, msg *linkv1.JoinToken) []string {
	secret := msg.GetSecret()
	var found []string
	for name, needle := range map[string]string{
		"the token text":              tok.Text,
		"the secret in hex":           hex.EncodeToString(secret),
		"a half of the secret in hex": hex.EncodeToString(secret[:16]),
		"the secret in base64":        base64.StdEncoding.EncodeToString(secret),
		"the secret in base64url":     base64.RawURLEncoding.EncodeToString(secret),
		"the token body":              strings.TrimPrefix(tok.Text, "lgtm1_"),
	} {
		if strings.Contains(text, needle) {
			found = append(found, name)
		}
	}
	return found
}

func TestJoinCall(t *testing.T) {
	e := newEnv(t)
	tok, msg := e.newToken(t)
	key := keyOf(0x31)

	res, err := e.connect.Join(t.Context(), request(msg, key, "box"))
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if !uuidV7.MatchString(res.Msg.GetMachineId()) {
		t.Errorf("the machine id is %q, want a version 7 identifier", res.Msg.GetMachineId())
	}
	got := sha256.Sum256(res.Msg.GetServerPublicKey())
	if !bytes.Equal(got[:], msg.GetServerKeySha256()) || !bytes.Equal(res.Msg.GetServerPublicKey(), e.serverKey) {
		t.Errorf("the server key %x does not match the token's server_key_sha256 %x", res.Msg.GetServerPublicKey(), msg.GetServerKeySha256())
	}
	if n := e.machineCount(t); n != 1 {
		t.Errorf("%d machines exist, want 1", n)
	}
	if hostName := storetest.Scalar[string](t, e.obs, "SELECT host_name FROM machines WHERE id = $1", res.Msg.GetMachineId()); hostName != "box" {
		t.Errorf("the machine's host name is %q, want box", hostName)
	}

	// A lost reply: the same token and key, even with another host name, get the same answer.
	before := e.tokenState(t)
	again, err := e.connect.Join(t.Context(), request(msg, key, "other"))
	if err != nil {
		t.Fatalf("the replay failed: %v", err)
	}
	if again.Msg.GetMachineId() != res.Msg.GetMachineId() || !bytes.Equal(again.Msg.GetServerPublicKey(), res.Msg.GetServerPublicKey()) {
		t.Errorf("the replay answered %q, want %q", again.Msg.GetMachineId(), res.Msg.GetMachineId())
	}
	if n := e.machineCount(t); n != 1 || e.tokenState(t) != before {
		t.Errorf("the replay changed the database (%d machines)", n)
	}

	// Another key is refused, and the first key still gets the first answer.
	_, err = e.connect.Join(t.Context(), request(msg, keyOf(0x32), "box"))
	ce := requireCode(t, "another key", err, connect.CodePermissionDenied)
	if ce.Meta().Get(refusalKey) != "token_used" {
		t.Errorf("another key got the reason %q, want token_used", ce.Meta().Get(refusalKey))
	}
	third, err := e.connect.Join(t.Context(), request(msg, key, "box"))
	if err != nil || third.Msg.GetMachineId() != res.Msg.GetMachineId() {
		t.Errorf("the first key after a refusal answered %v with %v, want the same machine", third, err)
	}

	// A call that carries the header must not leak and must not be cached.
	if cc := res.Header().Get("Cache-Control"); strings.Contains(cc, "public") {
		t.Errorf("the reply carries Cache-Control %q", cc)
	}
	_ = tok
}

func TestJoinRefusalMetadata(t *testing.T) {
	type scenario struct {
		reason  string
		prepare func(t *testing.T, e *env, tok machines.NewToken, msg *linkv1.JoinToken) *connect.Request[linkv1.JoinRequest]
	}
	scenarios := []scenario{
		{"token_unknown", func(t *testing.T, e *env, tok machines.NewToken, msg *linkv1.JoinToken) *connect.Request[linkv1.JoinRequest] {
			other := &linkv1.JoinToken{TokenId: msg.GetTokenId(), Secret: keyOf(0x55)}
			return request(other, keyOf(0x31), "box")
		}},
		{"token_expired", func(t *testing.T, e *env, tok machines.NewToken, msg *linkv1.JoinToken) *connect.Request[linkv1.JoinRequest] {
			storetest.Exec(t, e.obs, `UPDATE client_join_tokens SET expires_at = clock_timestamp() - interval '1 second' WHERE id = $1`, tok.ID)
			return request(msg, keyOf(0x31), "box")
		}},
		{"token_revoked", func(t *testing.T, e *env, tok machines.NewToken, msg *linkv1.JoinToken) *connect.Request[linkv1.JoinRequest] {
			storetest.Exec(t, e.obs, `UPDATE client_join_tokens SET revoked_at = clock_timestamp() WHERE id = $1`, tok.ID)
			return request(msg, keyOf(0x31), "box")
		}},
		{"token_used", func(t *testing.T, e *env, tok machines.NewToken, msg *linkv1.JoinToken) *connect.Request[linkv1.JoinRequest] {
			if _, err := e.connect.Join(t.Context(), request(msg, keyOf(0x31), "box")); err != nil {
				t.Fatalf("the first use failed: %v", err)
			}
			return request(msg, keyOf(0x32), "box")
		}},
		{"machine_removed", func(t *testing.T, e *env, tok machines.NewToken, msg *linkv1.JoinToken) *connect.Request[linkv1.JoinRequest] {
			res, err := e.connect.Join(t.Context(), request(msg, keyOf(0x31), "box"))
			if err != nil {
				t.Fatalf("the first use failed: %v", err)
			}
			storetest.Exec(t, e.obs, `DELETE FROM machines WHERE id = $1`, res.Msg.GetMachineId())
			return request(msg, keyOf(0x31), "box")
		}},
	}
	for _, protocol := range []string{"connect", "grpc"} {
		for _, sc := range scenarios {
			t.Run(protocol+"/"+sc.reason, func(t *testing.T) {
				e := newEnv(t)
				client := e.connect
				if protocol == "grpc" {
					client = e.grpc
				}
				tok, msg := e.newToken(t)
				req := sc.prepare(t, e, tok, msg)
				machinesBefore, tokensBefore := e.machineCount(t), e.tokenState(t)

				_, err := client.Join(t.Context(), req)
				ce := requireCode(t, sc.reason, err, connect.CodePermissionDenied)
				if got := ce.Meta().Get(refusalKey); got != sc.reason {
					t.Errorf("the reason under %s is %q, want %q (over %s)", refusalKey, got, sc.reason, protocol)
				}
				if ce.Message() != sentences[sc.reason] {
					t.Errorf("the message is %q, want %q", ce.Message(), sentences[sc.reason])
				}
				// The reason is metadata and no part of the message.
				if strings.Contains(ce.Message(), sc.reason) {
					t.Errorf("the message %q holds the reason word", ce.Message())
				}
				var all strings.Builder
				all.WriteString(ce.Message())
				for key, values := range ce.Meta() {
					all.WriteString(key + "=" + strings.Join(values, ",") + "\n")
				}
				if found := leaks(all.String(), tok, msg); len(found) != 0 {
					t.Errorf("the refusal's message or metadata holds %v", found)
				}
				if n := e.machineCount(t); n != machinesBefore {
					t.Errorf("a refusal changed the number of machines from %d to %d", machinesBefore, n)
				}
				if sc.reason != "token_used" && sc.reason != "machine_removed" && e.tokenState(t) != tokensBefore {
					t.Error("a refusal changed the token row")
				}
			})
		}
	}

	t.Run("success carries no secret", func(t *testing.T) {
		e := newEnv(t)
		tok, msg := e.newToken(t)
		res, err := e.connect.Join(t.Context(), request(msg, keyOf(0x31), "box"))
		if err != nil {
			t.Fatalf("Join failed: %v", err)
		}
		var all strings.Builder
		for _, h := range []http.Header{res.Header(), res.Trailer()} {
			for key, values := range h {
				all.WriteString(key + "=" + strings.Join(values, ",") + "\n")
			}
		}
		all.WriteString(res.Msg.GetMachineId())
		if found := leaks(all.String(), tok, msg); len(found) != 0 {
			t.Errorf("the reply's headers or trailers hold %v", found)
		}
	})
}

func TestJoinValueRules(t *testing.T) {
	e := newEnv(t)
	tok, msg := e.newToken(t)
	machinesBefore, tokensBefore := e.machineCount(t), e.tokenState(t)

	upper := &linkv1.JoinToken{TokenId: strings.ToUpper(msg.GetTokenId()), Secret: msg.GetSecret()}
	short := &linkv1.JoinToken{TokenId: msg.GetTokenId(), Secret: msg.GetSecret()[:31]}
	long := &linkv1.JoinToken{TokenId: msg.GetTokenId(), Secret: append(bytes.Clone(msg.GetSecret()), 0)}
	notUUID := &linkv1.JoinToken{TokenId: "not-an-identifier", Secret: msg.GetSecret()}
	empty := &linkv1.JoinToken{}
	cases := []struct {
		name string
		req  *connect.Request[linkv1.JoinRequest]
	}{
		{"an identifier in upper case", request(upper, keyOf(0x31), "box")},
		{"an identifier that is no UUID", request(notUUID, keyOf(0x31), "box")},
		{"a secret of 31 bytes", request(short, keyOf(0x31), "box")},
		{"a secret of 33 bytes", request(long, keyOf(0x31), "box")},
		{"a key of 33 bytes", request(msg, append(keyOf(0x31), 1), "box")},
		{"a key of 31 bytes", request(msg, keyOf(0x31)[:31], "box")},
		{"an empty key", request(msg, nil, "box")},
		{"an empty request", request(empty, nil, "")},
		{"a host name of 256 bytes", request(msg, keyOf(0x31), strings.Repeat("h", 256))},
		{"a host name with a zero byte", request(msg, keyOf(0x31), "bo\x00x")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := e.connect.Join(t.Context(), c.req)
			ce := requireCode(t, c.name, err, connect.CodeInvalidArgument)
			if ce.Meta().Get(refusalKey) != "" {
				t.Errorf("%s: the error carries the reason %q, want none", c.name, ce.Meta().Get(refusalKey))
			}
			if ce.Message() != "The link message broke a rule of the contract." {
				t.Errorf("%s: the message is %q", c.name, ce.Message())
			}
			if n := e.machineCount(t); n != machinesBefore || e.tokenState(t) != tokensBefore {
				t.Errorf("%s: the call changed the database", c.name)
			}
		})
	}

	// A first use with a good host name works after all those failures: nothing was spent.
	first, err := e.connect.Join(t.Context(), request(msg, keyOf(0x31), "box"))
	if err != nil {
		t.Fatalf("the first good use failed: %v", err)
	}

	// Only a first use reads the host name. A replay with a broken one answers the first machine.
	for _, host := range []string{strings.Repeat("h", 256), "bo\x00x"} {
		again, err := e.connect.Join(t.Context(), request(msg, keyOf(0x31), host))
		if err != nil || again.Msg.GetMachineId() != first.Msg.GetMachineId() {
			t.Errorf("a replay with a broken host name answered %v with %v, want the first machine", again, err)
		}
	}

	// Every refusal comes before the host name: an expired token with a broken host name is expired.
	_, expiredMsg := e.newToken(t)
	storetest.Exec(t, e.obs, `UPDATE client_join_tokens SET expires_at = clock_timestamp() - interval '1 second' WHERE id = $1`, expiredMsg.GetTokenId())
	_, err = e.connect.Join(t.Context(), request(expiredMsg, keyOf(0x33), strings.Repeat("h", 256)))
	ce := requireCode(t, "an expired token with a broken host name", err, connect.CodePermissionDenied)
	if ce.Meta().Get(refusalKey) != "token_expired" {
		t.Errorf("the reason is %q, want token_expired", ce.Meta().Get(refusalKey))
	}
	_ = tok
}

func TestJoinDatabaseDown(t *testing.T) {
	e := newEnv(t)
	_, msg := e.newToken(t)
	e.s.Close()
	_, err := e.connect.Join(t.Context(), request(msg, keyOf(0x31), "box"))
	ce := requireCode(t, "a closed store", err, connect.CodeUnavailable)
	if ce.Message() != "The server cannot reach its database, try again." {
		t.Errorf("the message is %q", ce.Message())
	}
	if ce.Meta().Get(refusalKey) != "" {
		t.Errorf("a database failure carries the reason %q, want none", ce.Meta().Get(refusalKey))
	}
}

// logBuffer is a writer that tests may read while the server writes.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestJoinKeepsSecretsOutOfLogs(t *testing.T) {
	logs := new(logBuffer)
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	e := newEnv(t)
	tok, msg := e.newToken(t)
	if _, err := e.connect.Join(t.Context(), request(msg, keyOf(0x31), "box")); err != nil {
		t.Fatalf("the success failed: %v", err)
	}
	_, err := e.connect.Join(t.Context(), request(msg, keyOf(0x32), "box"))
	requireCode(t, "the refusal", err, connect.CodePermissionDenied)
	_, other := e.newToken(t)
	_, err = e.connect.Join(t.Context(), request(&linkv1.JoinToken{TokenId: other.GetTokenId(), Secret: keyOf(0x66)}, keyOf(0x33), "box"))
	requireCode(t, "the unknown token", err, connect.CodePermissionDenied)

	if found := leaks(logs.String(), tok, msg); len(found) != 0 {
		t.Errorf("the log holds %v", found)
	}
	if found := leaks(logs.String(), machines.NewToken{Text: "lgtm1_unused"}, &linkv1.JoinToken{Secret: keyOf(0x66)}); len(found) != 0 {
		t.Errorf("the log holds the secret of a refused request: %v", found)
	}
}

func TestOpenUnimplemented(t *testing.T) {
	e := newEnv(t)
	stream := e.connect.Open(t.Context())
	_ = stream.Send(&linkv1.OpenRequest{})
	_, err := stream.Receive()
	requireCode(t, "Open in part A", err, connect.CodeUnimplemented)
	_ = stream.CloseRequest()
	_ = stream.CloseResponse()
}
