package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const testPassword = "fixture owner password"

type sessionDatabase struct {
	password auth.Password
	sessions *auth.Sessions
	observer *pgx.Conn
}

func newSessionDatabase(t *testing.T) sessionDatabase {
	t.Helper()
	return sessionDatabaseWithPassword(t, testPassword)
}

func sessionDatabaseWithPassword(t *testing.T, value string) sessionDatabase {
	t.Helper()
	cfg := storetest.NewDatabase(t, "api_test")
	db, err := store.Open(t.Context(), cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(t.Context(), auth.Migrations()); err != nil {
		t.Fatal(err)
	}
	observer, err := pgx.ConnectConfig(t.Context(), cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = observer.Close(ctx)
	})
	password := newTestPassword(t, value)
	return sessionDatabase{password, auth.NewSessions(db, password), observer}
}

func newTestPassword(t *testing.T, value string) auth.Password {
	t.Helper()
	password, err := auth.NewPassword(value)
	if err != nil {
		t.Fatal(err)
	}
	return password
}

func issueTestSession(t *testing.T, sessions *auth.Sessions) string {
	t.Helper()
	token, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatalf("Issuing a fixture session failed: %v.", err)
	}
	verifierOf(t, token)
	return token
}

func verifierOf(t *testing.T, token string) auth.Verifier {
	t.Helper()
	v, ok := auth.TokenVerifier(token)
	if !ok {
		t.Fatal("The session credential is not a canonical token.")
	}
	return v
}

func startHTTP2(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.StartTLS()
	server.Client().Timeout = 10 * time.Second
	t.Cleanup(server.Close)
	return server
}

func requireRPCError(t *testing.T, err error, code connect.Code, message string) {
	t.Helper()
	var rpcErr *connect.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code() != code || rpcErr.Message() != message {
		t.Fatalf("The call returned %v, want %s with message %q.", err, code, message)
	}
}

func headerListContains(header http.Header, name, value string) bool {
	for _, field := range header.Values(name) {
		for _, entry := range strings.Split(field, ",") {
			if strings.EqualFold(strings.TrimSpace(entry), value) {
				return true
			}
		}
	}
	return false
}

func requireOriginResponse(t *testing.T, header http.Header, origin string) {
	t.Helper()
	if header.Get("Access-Control-Allow-Origin") != origin ||
		header.Get("Access-Control-Allow-Credentials") != "true" ||
		!headerListContains(header, "Vary", "Origin") {
		t.Errorf("The response lacks permission for the exact credentialed origin and Vary: Origin. Headers were %v.", header)
	}
}

func TestBrowserOriginGate(t *testing.T) {
	const browserOrigin = "http://localhost:18787"
	probe := &sessionProbe{}
	handler := api.NewHandler(api.Options{
		Origins: testOriginPolicy(t, loopbackOrigin),
		Session: probe,
	})

	allowedHeaders := []string{
		"content-type", "Connect-Protocol-Version", "connect-timeout-ms", "Connect-Accept-Encoding",
		"connect-content-encoding", "Grpc-Timeout", "X-GRPC-Web", "x-user-agent", "Lgtm-Request-Key",
	}
	for _, tc := range []struct {
		name, origin, procedure, method, headers string
		wantStatus                               int
	}{
		{"allowed", browserOrigin, webrpc.SessionServiceSignInProcedure, "POST", strings.Join(allowedHeaders, ", "), http.StatusNoContent},
		{"protected preflight without a cookie", browserOrigin, webrpc.ClientServiceWatchClientsProcedure, "POST", "content-type", http.StatusNoContent},
		{"wrong port", "http://localhost:8787", webrpc.SessionServiceSignInProcedure, "POST", "content-type", http.StatusForbidden},
		{"unlisted origin", "https://outside.example.test", webrpc.SessionServiceSignInProcedure, "POST", "content-type", http.StatusForbidden},
		{"unexpected header", browserOrigin, webrpc.SessionServiceSignInProcedure, "POST", "content-type, X-Fixture-Unsupported", http.StatusForbidden},
		{"unsupported method", browserOrigin, webrpc.SessionServiceSignInProcedure, "GET", "content-type", http.StatusForbidden},
		{"unknown procedure", browserOrigin, "/lgtm.v1.SessionService/Missing", "POST", "content-type", http.StatusForbidden},
	} {
		t.Run("preflight/"+tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, tc.procedure, nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", tc.method)
			req.Header.Set("Access-Control-Request-Headers", tc.headers)
			res := serveRequest(handler, req)
			if res.Code != tc.wantStatus || res.Body.Len() != 0 {
				t.Errorf("The preflight returned HTTP %d and %d body bytes, want HTTP %d and no body.", res.Code, res.Body.Len(), tc.wantStatus)
			}
			header := res.Header()
			if tc.wantStatus == http.StatusNoContent {
				requireOriginResponse(t, header, tc.origin)
				if header.Get("Access-Control-Allow-Methods") != "POST" {
					t.Error("The preflight must allow exactly POST.")
				}
				for _, requested := range strings.Split(tc.headers, ", ") {
					if !headerListContains(header, "Access-Control-Allow-Headers", requested) {
						t.Errorf("The preflight did not allow the requested header %q.", requested)
					}
				}
				if headerListContains(header, "Access-Control-Allow-Headers", "*") {
					t.Error("The preflight allowed wildcard headers.")
				}
				for _, field := range []string{"Access-Control-Request-Method", "Access-Control-Request-Headers"} {
					if !headerListContains(header, "Vary", field) {
						t.Errorf("The preflight response does not vary by %s.", field)
					}
				}
			} else {
				for _, name := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers"} {
					if header.Get(name) != "" {
						t.Errorf("A refused preflight returned %s.", name)
					}
				}
			}
			if header.Get("Set-Cookie") != "" {
				t.Error("A preflight issued a cookie.")
			}
		})
	}

	t.Run("denied actual calls", func(t *testing.T) {
		for _, tc := range []struct {
			name, contentType, version string
			origins                    []string
			fetchSite                  string
			protocol                   bool
		}{
			{"valid protocol call", "application/json", "1", []string{"https://outside.example.test"}, "", true},
			{"denial before decoding", "application/json", "1", []string{"http://localhost:8787"}, "", true},
			{"duplicate origins", "application/json", "1", []string{browserOrigin, browserOrigin}, "", true},
			{"missing origin on a cross-site fetch", "application/json", "1", nil, "cross-site", true},
			{"unknown media type", "text/plain", "1", []string{"https://outside.example.test"}, "", false},
			{"missing protocol version", "application/json", "", []string{"https://outside.example.test"}, "", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				body := `{"password":"unused"}`
				if tc.name == "denial before decoding" {
					body = "{"
				}
				req := jsonProcedure(t, webrpc.SessionServiceSignInProcedure, body)
				req.Header.Set("Content-Type", tc.contentType)
				for _, origin := range tc.origins {
					req.Header.Add("Origin", origin)
				}
				if tc.version == "" {
					req.Header.Del("Connect-Protocol-Version")
				}
				req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
				req.Header.Set("Forwarded", "host=localhost:18787;proto=http")
				req.Header.Set("X-Forwarded-Host", "localhost:18787")
				res := serveRequest(handler, req)
				if tc.protocol {
					requireConnectJSONError(t, res, http.StatusForbidden, connect.CodePermissionDenied, "This website origin is not allowed.")
				} else if res.Code != http.StatusForbidden || res.Body.Len() != 0 {
					t.Errorf("A nonprotocol browser request returned HTTP %d and %d body bytes, want an empty 403.", res.Code, res.Body.Len())
				}
				for _, name := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Set-Cookie"} {
					if res.Header().Get(name) != "" {
						t.Errorf("A refused request returned %s.", name)
					}
				}
				if res.Header().Get("Cache-Control") != "no-store" {
					t.Error("The refused SignIn response lacks Cache-Control: no-store.")
				}
			})
		}
	})
	if probe.signInCalls.Load() != 0 {
		t.Error("A preflight or refused call reached SignIn.")
	}

	t.Run("allowed actual calls", func(t *testing.T) {
		for _, procedure := range []string{webrpc.SessionServiceSignInProcedure, webrpc.SessionServiceSignOutProcedure} {
			req := jsonProcedure(t, procedure, "{}")
			req.Header.Set("Origin", browserOrigin)
			res := serveRequest(handler, req)
			if procedure == webrpc.SessionServiceSignOutProcedure {
				requireConnectJSONError(t, res, http.StatusUnauthorized, connect.CodeUnauthenticated, "Sign in to continue.")
			} else if res.Code != http.StatusOK {
				t.Errorf("The allowed browser SignIn returned HTTP %d, want 200.", res.Code)
			}
			requireOriginResponse(t, res.Header(), browserOrigin)
			for _, field := range []string{"Grpc-Status", "Grpc-Message", "Grpc-Status-Details-Bin"} {
				if !headerListContains(res.Header(), "Access-Control-Expose-Headers", field) {
					t.Errorf("The allowed response did not expose %s.", field)
				}
			}
		}
		if probe.signInCalls.Load() != 1 || probe.signOutCalls.Load() != 0 {
			t.Error("The allowed browser must reach SignIn once and leave protected SignOut untouched.")
		}
	})

	t.Run("public Connect unary requires version", func(t *testing.T) {
		req := jsonProcedure(t, webrpc.SessionServiceSignInProcedure, "{}")
		req.Header.Del("Connect-Protocol-Version")
		res := serveRequest(handler, req)
		if res.Code != http.StatusBadRequest || probe.signInCalls.Load() != 1 {
			t.Errorf("A public Connect unary call without the version returned HTTP %d or reached SignIn.", res.Code)
		}
	})
}

type observedVerifier struct {
	value auth.Verifier
	ok    bool
}

type clientProbe struct {
	webrpc.UnimplementedClientServiceHandler
	unaryCalls atomic.Int32
	watchCalls atomic.Int32
	seen       chan observedVerifier
}

func (p *clientProbe) record(ctx context.Context) {
	if p.seen != nil {
		value, ok := api.SessionVerifier(ctx)
		p.seen <- observedVerifier{value, ok}
	}
}

func (p *clientProbe) ApproveClient(ctx context.Context, _ *connect.Request[webv1.ApproveClientRequest]) (*connect.Response[webv1.ApproveClientResponse], error) {
	p.unaryCalls.Add(1)
	p.record(ctx)
	return connect.NewResponse(&webv1.ApproveClientResponse{}), nil
}

func (p *clientProbe) WatchClients(ctx context.Context, _ *connect.Request[webv1.WatchClientsRequest], stream *connect.ServerStream[webv1.WatchClientsResponse]) error {
	p.watchCalls.Add(1)
	p.record(ctx)
	return stream.Send(&webv1.WatchClientsResponse{})
}

func TestProtectedStreamAndProtocolCodes(t *testing.T) {
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	verifier := verifierOf(t, token)
	cookie := (&http.Cookie{Name: auth.CookieName, Value: token}).String()
	probe := &clientProbe{seen: make(chan observedVerifier, 32)}
	handler := api.NewHandler(api.Options{
		Sessions: fixture.sessions,
		Origins:  testOriginPolicy(t, loopbackOrigin),
		Clients:  probe,
	})
	server := startHTTP2(t, handler)
	for _, transport := range []struct {
		name    string
		options []connect.ClientOption
	}{
		{"Connect JSON", []connect.ClientOption{connect.WithProtoJSON()}},
		{"gRPC over HTTP2", []connect.ClientOption{connect.WithGRPC()}},
		{"gRPC-Web", []connect.ClientOption{connect.WithGRPCWeb()}},
	} {
		t.Run(transport.name, func(t *testing.T) {
			client := webrpc.NewClientServiceClient(server.Client(), server.URL, transport.options...)
			for _, tc := range []struct {
				name    string
				cookies []string
				valid   bool
			}{
				{"missing", nil, false},
				{"malformed", []string{auth.CookieName + "=malformed"}, false},
				{"padded", []string{cookie + "="}, false},
				{"duplicate", []string{cookie, cookie}, false},
				{"valid", []string{cookie}, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					beforeUnary, beforeWatch := probe.unaryCalls.Load(), probe.watchCalls.Load()
					unary := connect.NewRequest(&webv1.ApproveClientRequest{})
					unary.Header()["Cookie"] = tc.cookies
					_, unaryErr := client.ApproveClient(t.Context(), unary)

					watch := connect.NewRequest(&webv1.WatchClientsRequest{})
					watch.Header()["Cookie"] = tc.cookies
					stream, watchErr := client.WatchClients(t.Context(), watch)
					var received bool
					if watchErr == nil {
						defer stream.Close()
						received = stream.Receive()
						if received && stream.Receive() {
							t.Error("The stream probe sent more than one response.")
						}
						watchErr = stream.Err()
					}

					if !tc.valid {
						requireRPCError(t, unaryErr, connect.CodeUnauthenticated, "Sign in to continue.")
						requireRPCError(t, watchErr, connect.CodeUnauthenticated, "Sign in to continue.")
						if received || probe.unaryCalls.Load() != beforeUnary || probe.watchCalls.Load() != beforeWatch {
							t.Error("An unauthenticated request reached a protected handler.")
						}
						return
					}
					if unaryErr != nil || watchErr != nil || !received {
						t.Fatalf("The valid cookie did not reach both handlers. Unary error was %v, stream error was %v, and a stream message arrived=%t.", unaryErr, watchErr, received)
					}
					if probe.unaryCalls.Load() != beforeUnary+1 || probe.watchCalls.Load() != beforeWatch+1 {
						t.Fatal("Each protected handler must run once for the valid cookie.")
					}
					for range 2 {
						select {
						case got := <-probe.seen:
							if !got.ok || got.value != verifier {
								t.Error("The handler context must expose the session digest.")
							}
						default:
							t.Error("The protected handler did not report a context verifier.")
						}
					}
				})
			}
		})
	}

	t.Run("store errors are sanitized", func(t *testing.T) {
		breakSessionReads(t, fixture.observer)
		client := webrpc.NewClientServiceClient(server.Client(), server.URL, connect.WithProtoJSON())
		req := connect.NewRequest(&webv1.ApproveClientRequest{})
		req.Header().Set("Cookie", cookie)
		before := probe.unaryCalls.Load()
		_, err := client.ApproveClient(t.Context(), req)
		requireRPCError(t, err, connect.CodeUnavailable, "Web session storage is unavailable.")
		if strings.Contains(err.Error(), fixtureFailureDetail) || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), testPassword) {
			t.Error("A protected call disclosed a fixture secret.")
		}
		if probe.unaryCalls.Load() != before {
			t.Error("The protected handler ran after session storage failed.")
		}
	})
}

const fixtureFailureDetail = "fixture-private-database-diagnostic"

func breakSessionReads(t *testing.T, observer *pgx.Conn) {
	t.Helper()
	storetest.Exec(t, observer, "ALTER TABLE web_sessions RENAME TO fixture_sessions")
	storetest.Exec(t, observer, `
CREATE FUNCTION fixture_fail_session_read(value bytea) RETURNS bytea LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'fixture-private-database-diagnostic';
END;
$$`)
	storetest.Exec(t, observer, `
CREATE VIEW web_sessions AS
SELECT fixture_fail_session_read(verifier) AS verifier, created_at, password_salt, password_binding
FROM fixture_sessions`)
}
