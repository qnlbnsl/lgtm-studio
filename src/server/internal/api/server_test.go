package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	linkrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
)

const loopbackOrigin = "http://127.0.0.1:18787"

type sessionProbe struct {
	webrpc.UnimplementedSessionServiceHandler
	getServerCalls atomic.Int32
	signInCalls    atomic.Int32
	signOutCalls   atomic.Int32
}

func (p *sessionProbe) GetServer(_ context.Context, _ *connect.Request[webv1.GetServerRequest]) (*connect.Response[webv1.GetServerResponse], error) {
	p.getServerCalls.Add(1)
	return connect.NewResponse(&webv1.GetServerResponse{}), nil
}

func (p *sessionProbe) SignIn(_ context.Context, _ *connect.Request[webv1.SignInRequest]) (*connect.Response[webv1.SignInResponse], error) {
	p.signInCalls.Add(1)
	return connect.NewResponse(&webv1.SignInResponse{}), nil
}

func (p *sessionProbe) SignOut(_ context.Context, _ *connect.Request[webv1.SignOutRequest]) (*connect.Response[webv1.SignOutResponse], error) {
	p.signOutCalls.Add(1)
	return connect.NewResponse(&webv1.SignOutResponse{}), nil
}

func testOriginPolicy(t *testing.T, public string, allowed ...string) *auth.OriginPolicy {
	t.Helper()
	policy, err := auth.NewOriginPolicy(public, allowed)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func jsonProcedure(t *testing.T, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, loopbackOrigin+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	return req
}

func serveRequest(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func requireConnectJSONError(t *testing.T, res *httptest.ResponseRecorder, status int, code connect.Code, message string) {
	t.Helper()
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || res.Code != status ||
		body.Code != code.String() || body.Message != message {
		t.Fatalf("Response was HTTP %d with body %q, want HTTP %d with %s and %q.", res.Code, res.Body.String(), status, code, message)
	}
}

func TestWebRoutesRequireSession(t *testing.T) {
	probe := &sessionProbe{}
	clients := &clientProbe{}
	opts := api.Options{Origins: testOriginPolicy(t, loopbackOrigin), Session: probe, Clients: clients}
	handler := api.NewHandler(opts)

	t.Run("public procedures and readiness", func(t *testing.T) {
		for _, procedure := range []string{webrpc.SessionServiceGetServerProcedure, webrpc.SessionServiceSignInProcedure} {
			res := serveRequest(handler, jsonProcedure(t, procedure, "{}"))
			if res.Code != http.StatusOK || res.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("%s returned HTTP %d and Cache-Control %q, want 200 and no-store.", procedure, res.Code, res.Header().Get("Cache-Control"))
			}
		}
		if probe.getServerCalls.Load() != 1 || probe.signInCalls.Load() != 1 {
			t.Error("Each public Session handler must be called once.")
		}
		res := serveRequest(handler, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if res.Code != http.StatusNoContent || res.Body.Len() != 0 {
			t.Errorf("Readiness returned HTTP %d and %d body bytes, want 204 and no body.", res.Code, res.Body.Len())
		}
		res = serveRequest(handler, httptest.NewRequest(http.MethodGet, "/ordinary-page", nil))
		if res.Code != http.StatusNotFound {
			t.Errorf("An ordinary unknown path returned HTTP %d, want 404.", res.Code)
		}
	})

	protected := []struct {
		name      string
		procedure string
	}{
		{"Session", webrpc.SessionServiceSignOutProcedure},
		{"Client", webrpc.ClientServiceApproveClientProcedure},
		{"Map", webrpc.MapServiceOpenMapProcedure},
		{"Preview", webrpc.PreviewServiceStartPreviewProcedure},
		{"unknown web service", "/lgtm.v1.MissingService/Missing"},
		{"unknown Session method", "/lgtm.v1.SessionService/Missing"},
		{"unknown Link method", "/lgtm.link.v1.LinkService/Missing"},
		{"unknown Link service", "/lgtm.link.v1.MissingService/Join"},
	}
	for _, call := range protected {
		t.Run("anonymous/"+call.name, func(t *testing.T) {
			res := serveRequest(handler, jsonProcedure(t, call.procedure, "{}"))
			requireConnectJSONError(t, res, http.StatusUnauthorized, connect.CodeUnauthenticated, "Sign in to continue.")
			if call.name == "Session" && res.Header().Get("Cache-Control") != "no-store" {
				t.Error("An unauthenticated SignOut must carry Cache-Control: no-store.")
			}
		})
	}
	if probe.signOutCalls.Load() != 0 || clients.unaryCalls.Load() != 0 {
		t.Error("A protected handler received an unauthenticated call.")
	}

	t.Run("paths and methods", func(t *testing.T) {
		getServerBefore, signInBefore := probe.getServerCalls.Load(), probe.signInCalls.Load()
		for _, path := range []string{
			"/lgtm.v1.SessionService/%47etServer",
			"/lgtm.v1.SessionService%2FSignIn",
			"/lgtm.v1.SessionService//SignIn",
			"/lgtm.v1.SessionService/./SignIn",
			"/other/../lgtm.v1.SessionService/GetServer",
			"/lgtm.v1.SessionService/SignIn/",
			"/lgtm.link.v1.LinkService/%4Aoin",
		} {
			res := serveRequest(handler, jsonProcedure(t, path, "{}"))
			if res.Code != http.StatusBadRequest || res.Body.Len() != 0 || res.Header().Get("Location") != "" {
				t.Errorf("Alias %s returned HTTP %d and %d body bytes, want an empty 400 without redirect.", path, res.Code, res.Body.Len())
			}
		}
		for _, procedure := range []string{webrpc.SessionServiceSignInProcedure, webrpc.ClientServiceApproveClientProcedure, linkrpc.LinkServiceJoinProcedure} {
			req := httptest.NewRequest(http.MethodGet, procedure, nil)
			res := serveRequest(handler, req)
			if res.Code != http.StatusMethodNotAllowed || res.Header().Get("Allow") != "POST" || res.Body.Len() != 0 {
				t.Errorf("GET %s returned HTTP %d, Allow %q and %d body bytes, want an empty 405 with Allow: POST.", procedure, res.Code, res.Header().Get("Allow"), res.Body.Len())
			}
		}
		if probe.getServerCalls.Load() != getServerBefore || probe.signInCalls.Load() != signInBefore {
			t.Error("A path alias or an unsupported method reached a public handler.")
		}
	})

	t.Run("exact Link placeholders", func(t *testing.T) {
		req := jsonProcedure(t, linkrpc.LinkServiceJoinProcedure, "{}")
		req.Header.Set("Origin", "https://outside.example.test")
		res := serveRequest(handler, req)
		requireConnectJSONError(t, res, http.StatusNotImplemented, connect.CodeUnimplemented, "This call is not implemented.")

		server := startHTTP2(t, handler)
		client := linkrpc.NewLinkServiceClient(server.Client(), server.URL, connect.WithGRPC())
		stream := client.Open(t.Context())
		defer stream.CloseResponse()
		if err := stream.Send(nil); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("Sending to Link Open failed: %v.", err)
		}
		if err := stream.CloseRequest(); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("Closing the Link Open request failed: %v.", err)
		}
		_, err := stream.Receive()
		requireRPCError(t, err, connect.CodeUnimplemented, "This call is not implemented.")
	})

	t.Run("authenticated procedures", func(t *testing.T) {
		fixture := newSessionDatabase(t)
		opts.Sessions = fixture.sessions
		handler := api.NewHandler(opts)
		token := issueTestSession(t, fixture.sessions)
		for _, call := range protected {
			t.Run(call.name, func(t *testing.T) {
				req := jsonProcedure(t, call.procedure, "{}")
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
				res := serveRequest(handler, req)
				switch call.name {
				case "Session", "Client":
					if res.Code != http.StatusOK {
						t.Errorf("%s returned HTTP %d with a valid cookie, want 200.", call.name, res.Code)
					}
				default:
					requireConnectJSONError(t, res, http.StatusNotImplemented, connect.CodeUnimplemented, "This call is not implemented.")
				}
			})
		}
		if probe.signOutCalls.Load() != 1 || clients.unaryCalls.Load() != 1 {
			t.Error("Each registered protected handler must receive one authenticated call.")
		}
	})
}
