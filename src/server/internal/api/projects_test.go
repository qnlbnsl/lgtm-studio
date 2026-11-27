package api_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protowire"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

const (
	projectAllowedOrigin = "http://127.0.0.1:18788"
	projectDeniedOrigin  = "https://outside.example.test"
	projectReadLimit     = 32 << 10
	projectClientID      = "00000000-0000-7000-8000-000000000001"
	projectRepositoryID  = "00000000-0000-7000-8000-000000000002"
)

type projectRoute struct {
	name, procedure string
	watch           bool
}

var projectRoutes = []projectRoute{
	{"InspectFolder", webrpc.ProjectServiceInspectFolderProcedure, false},
	{"AddRepository", webrpc.ProjectServiceAddRepositoryProcedure, false},
	{"WatchRepositories", webrpc.ProjectServiceWatchRepositoriesProcedure, true},
}

func (r projectRoute) jsonBody(t *testing.T, path string) []byte {
	t.Helper()
	if r.watch {
		return []byte("{}")
	}
	fields := map[string]string{"clientId": projectClientID, "path": path}
	if r.procedure == webrpc.ProjectServiceAddRepositoryProcedure {
		fields["repositoryId"] = projectRepositoryID
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal("Encoding the Projects request failed.")
	}
	return encoded
}

// projectRouterProbe is a test writer's service stub for the generated Projects interface.
type projectRouterProbe struct {
	webrpc.UnimplementedProjectServiceHandler
	calls atomic.Int32
	path  atomic.Pointer[string]
}

func (p *projectRouterProbe) entered(ctx context.Context) error {
	p.calls.Add(1)
	if _, ok := api.SessionVerifier(ctx); !ok {
		return connect.NewError(connect.CodeInternal, errors.New("The Projects probe did not receive a verified session."))
	}
	return nil
}

func (p *projectRouterProbe) InspectFolder(ctx context.Context, req *connect.Request[webv1.InspectFolderRequest]) (*connect.Response[webv1.InspectFolderResponse], error) {
	path := req.Msg.GetPath()
	p.path.Store(&path)
	return connect.NewResponse(&webv1.InspectFolderResponse{}), p.entered(ctx)
}

func (p *projectRouterProbe) AddRepository(ctx context.Context, req *connect.Request[webv1.AddRepositoryRequest]) (*connect.Response[webv1.AddRepositoryResponse], error) {
	path := req.Msg.GetPath()
	p.path.Store(&path)
	return connect.NewResponse(&webv1.AddRepositoryResponse{}), p.entered(ctx)
}

func (p *projectRouterProbe) WatchRepositories(ctx context.Context, _ *connect.Request[webv1.WatchRepositoriesRequest], stream *connect.ServerStream[webv1.WatchRepositoriesResponse]) error {
	if err := p.entered(ctx); err != nil {
		return err
	}
	return transport.Send(ctx, func() error {
		return stream.Send(&webv1.WatchRepositoriesResponse{
			Change: &webv1.WatchRepositoriesResponse_All{All: &webv1.RepositoryList{}},
		})
	})
}

func projectRouter(t *testing.T, fixture sessionDatabase, projects webrpc.ProjectServiceHandler) http.Handler {
	t.Helper()
	options := api.Options{
		Origins:  testOriginPolicy(t, loopbackOrigin, projectAllowedOrigin),
		Sessions: fixture.sessions,
		Session:  api.NewSessionService(fixture.password, fixture.sessions),
	}
	if projects != nil {
		// The test writer's option stub leaves Projects unmounted until the planned field exists.
		if field := reflect.ValueOf(&options).Elem().FieldByName("Projects"); field.IsValid() {
			field.Set(reflect.ValueOf(&projects).Elem())
		}
	}
	return api.NewHandler(options)
}

func TestProjectRouterPreflight(t *testing.T) {
	fixture, setClock := clockSessionDatabase(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	expired := issueTestSession(t, fixture.sessions)
	setClock(time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC))
	token := issueTestSession(t, fixture.sessions)
	probe := &projectRouterProbe{}
	handler := projectRouter(t, fixture, probe)
	server := startHTTP2(t, handler)
	for _, route := range projectRoutes {
		t.Run(route.name+"/Preflight", func(t *testing.T) {
			for _, origin := range []string{projectAllowedOrigin, projectDeniedOrigin} {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, route.procedure, nil)
				req.Header.Set("Origin", origin)
				req.Header.Set("Access-Control-Request-Method", http.MethodPost)
				headers := "content-type, connect-protocol-version"
				if !route.watch {
					headers += ", " + strings.ToLower(store.RequestKeyHeader)
				}
				req.Header.Set("Access-Control-Request-Headers", headers)
				res := serveRequest(handler, req)
				if origin == projectDeniedOrigin {
					if res.Code != http.StatusForbidden || res.Header().Get("Access-Control-Allow-Origin") != "" ||
						res.Header().Get("Access-Control-Allow-Credentials") != "" {
						t.Error("The unlisted origin must receive HTTP 403 without credentialed access.")
					}
					continue
				}
				if res.Code != http.StatusNoContent || res.Body.Len() != 0 {
					t.Errorf("The listed origin received HTTP %d, want an empty HTTP 204.", res.Code)
				}
				if res.Header().Get("Access-Control-Allow-Origin") != origin ||
					res.Header().Get("Access-Control-Allow-Credentials") != "true" ||
					res.Header().Get("Access-Control-Allow-Methods") != http.MethodPost {
					t.Error("The preflight must allow a credentialed POST from the listed origin.")
				}
				for _, name := range strings.Split(headers, ", ") {
					if !headerListContains(res.Header(), "Access-Control-Allow-Headers", name) {
						t.Errorf("The preflight did not allow the %s header.", name)
					}
				}
			}
		})
		t.Run(route.name+"/Dispatch", func(t *testing.T) {
			before := probe.calls.Load()
			body := route.jsonBody(t, "/checkout")
			for _, credential := range []string{"", expired} {
				reply := sendProjectRequest(t, server, route, body, "json", false, projectAllowedOrigin, credential)
				reply.requireError(t, route, connect.CodeUnauthenticated, sessionRequired)
			}
			denied := sendProjectRequest(t, server, route, body, "json", false, projectDeniedOrigin, token)
			denied.requireError(t, route, connect.CodePermissionDenied, "This website origin is not allowed.")
			if probe.calls.Load() != before {
				t.Error("A call refused at entry reached the Projects handler.")
			}
			ok := sendProjectRequest(t, server, route, body, "json", false, projectAllowedOrigin, token)
			ok.requireSuccess(t, route)
			if probe.calls.Load() != before+1 {
				t.Error("The signed-in call must reach the Projects handler once.")
			}
		})
	}
}

func TestProjectRouterMalformedJSON(t *testing.T) {
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	probe := &projectRouterProbe{}
	handler := projectRouter(t, fixture, probe)
	server := startHTTP2(t, handler)
	const marker = "ProjectSubmittedPayload"
	malformed := []byte(`{"path":"ok","path":` + marker + `}`)
	for _, route := range projectRoutes {
		t.Run(route.name, func(t *testing.T) {
			anonymous := sendProjectRequest(t, server, route, malformed, "json", false, "", "")
			anonymous.requireError(t, route, connect.CodeUnauthenticated, sessionRequired)
			signedIn := sendProjectRequest(t, server, route, malformed, "json", false, "", token)
			if signedIn.contains(marker) {
				t.Error("The malformed Projects response exposed the submitted marker.")
			}
			signedIn.requireError(t, route, connect.CodeInvalidArgument, "The request body is invalid.")
			if probe.calls.Load() != 0 {
				t.Error("A malformed request reached the Projects handler.")
			}
		})
	}
	t.Run("BinaryWatch", func(t *testing.T) {
		var installed, cleared atomic.Int32
		observed := startHTTP2(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			handler.ServeHTTP(projectDeadlineWriter{w, &installed, &cleared}, req)
		}))
		// This valid unknown field has the same bytes as the existing malformed JSON mark.
		unknown := protowire.AppendBytes(protowire.AppendTag(nil, protowire.MaxValidNumber, protowire.BytesType), nil)
		web := webrpc.NewProjectServiceClient(observed.Client(), observed.URL)
		msg := &webv1.WatchRepositoriesRequest{}
		msg.ProtoReflect().SetUnknown(unknown)
		req := connect.NewRequest(msg)
		req.Header().Set("Cookie", (&http.Cookie{Name: auth.CookieName, Value: token}).String())
		stream, err := web.WatchRepositories(t.Context(), req)
		if err != nil {
			t.Fatalf("Opening the binary watch returned %s, want a stream.", connect.CodeOf(err))
		}
		defer stream.Close()
		if !stream.Receive() || stream.Msg().GetAll() == nil || stream.Receive() || stream.Err() != nil {
			t.Error("The binary watch with an unknown field must return one snapshot and close successfully.")
		}
		if installed.Load() == 0 || cleared.Load() == 0 {
			t.Error("The watch must set and clear a write deadline on the HTTP response writer.")
		}
	})
}

type projectDeadlineWriter struct {
	http.ResponseWriter
	installed, cleared *atomic.Int32
}

func (w projectDeadlineWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w projectDeadlineWriter) Flush() { _ = http.NewResponseController(w.ResponseWriter).Flush() }

func (w projectDeadlineWriter) SetWriteDeadline(at time.Time) error {
	if err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(at); err != nil {
		return err
	}
	if at.IsZero() {
		w.cleared.Add(1)
	} else {
		w.installed.Add(1)
	}
	return nil
}

func TestProjectRouterDiscovery(t *testing.T) {
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	for _, tc := range []struct {
		name     string
		projects webrpc.ProjectServiceHandler
		count    int
	}{
		{"Mounted", &projectRouterProbe{}, 1},
		{"Absent", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := startHTTP2(t, projectRouter(t, fixture, tc.projects))
			session := webrpc.NewSessionServiceClient(server.Client(), server.URL)
			for _, signedIn := range []bool{false, true} {
				req := connect.NewRequest(&webv1.GetServerRequest{})
				if signedIn {
					req.Header().Set("Cookie", (&http.Cookie{Name: auth.CookieName, Value: token}).String())
				}
				res, err := session.GetServer(t.Context(), req)
				if err != nil {
					t.Fatalf("GetServer with signed_in=%t returned %s.", signedIn, connect.CodeOf(err))
				}
				if res.Msg.GetContract() != 1 || res.Msg.GetSignedIn() != signedIn || res.Msg.GetSample() {
					t.Errorf("GetServer must preserve revision 1, signed_in=%t and sample=false.", signedIn)
				}
				count := 0
				for _, name := range res.Msg.GetServices() {
					if name == webrpc.ProjectServiceName {
						count++
					}
				}
				if count != tc.count {
					t.Errorf("GetServer with signed_in=%t advertised Projects %d times, want %d.", signedIn, count, tc.count)
				}
			}
			if tc.projects == nil {
				for _, route := range projectRoutes {
					t.Run(route.name, func(t *testing.T) {
						body := route.jsonBody(t, "/checkout")
						anonymous := sendProjectRequest(t, server, route, body, "json", false, "", "")
						anonymous.requireError(t, route, connect.CodeUnauthenticated, sessionRequired)
						signedIn := sendProjectRequest(t, server, route, body, "json", false, "", token)
						signedIn.requireError(t, route, connect.CodeUnimplemented, "This call is not implemented.")
					})
				}
			}
		})
	}
}

func TestProjectRouterReadLimit(t *testing.T) {
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	probe := &projectRouterProbe{}
	server := startHTTP2(t, projectRouter(t, fixture, probe))
	// The tag and length take four bytes at this size. Every request accepts the unknown field.
	messageOfSize := func(size int) []byte {
		padding := protowire.AppendTag(nil, 15, protowire.BytesType)
		msg := protowire.AppendBytes(padding, make([]byte, size-4))
		if len(msg) != size {
			t.Fatal("The Projects size fixture has the wrong encoded length.")
		}
		return msg
	}
	atLimit, tooLarge := messageOfSize(projectReadLimit), messageOfSize(projectReadLimit+1)
	for _, route := range projectRoutes {
		t.Run(route.name, func(t *testing.T) {
			for _, encoding := range []string{"Plain", "Gzip"} {
				t.Run(encoding, func(t *testing.T) {
					before := probe.calls.Load()
					compressed := encoding == "Gzip"
					denied := sendProjectRequest(t, server, route, tooLarge, "proto", compressed, projectDeniedOrigin, token)
					denied.requireError(t, route, connect.CodePermissionDenied, "This website origin is not allowed.")
					anonymous := sendProjectRequest(t, server, route, tooLarge, "proto", compressed, projectAllowedOrigin, "")
					anonymous.requireError(t, route, connect.CodeUnauthenticated, sessionRequired)
					signedIn := sendProjectRequest(t, server, route, tooLarge, "proto", compressed, projectAllowedOrigin, token)
					signedIn.requireError(t, route, connect.CodeResourceExhausted, "")
					if probe.calls.Load() != before {
						t.Error("A rejected oversized request reached the Projects handler.")
					}
					boundary := sendProjectRequest(t, server, route, atLimit, "proto", compressed, projectAllowedOrigin, token)
					boundary.requireSuccess(t, route)
					if probe.calls.Load() != before+1 {
						t.Error("A request at the size limit must reach the Projects handler once.")
					}
				})
			}
			if route.watch {
				return
			}
			t.Run("EscapedPath", func(t *testing.T) {
				before := probe.calls.Load()
				const pathBytes = 4096
				path := "/" + strings.Repeat("<", pathBytes-2) + " "
				body := route.jsonBody(t, path)
				if len(body) <= 24<<10 || len(body) >= projectReadLimit {
					t.Fatal("The heavily escaped path fixture must exceed 24 kibibytes and fit under the request cap.")
				}
				reply := sendProjectRequest(t, server, route, body, "json", false, projectAllowedOrigin, token)
				reply.requireSuccess(t, route)
				if got := probe.path.Load(); probe.calls.Load() != before+1 || got == nil || *got != path {
					t.Error("The Projects handler must receive the exact 4,096-byte typed path once.")
				}
			})
		})
	}
}

type projectReply struct {
	status          int
	body            []byte
	header, trailer http.Header
}

func sendProjectRequest(t *testing.T, server *httptest.Server, route projectRoute, body []byte, codec string, compressed bool, origin, token string) projectReply {
	t.Helper()
	if compressed {
		var buf bytes.Buffer
		zipper := gzip.NewWriter(&buf)
		if _, err := zipper.Write(body); err != nil {
			t.Fatal("Writing the compressed Projects fixture failed.")
		}
		if err := zipper.Close(); err != nil || buf.Len() >= projectReadLimit {
			t.Fatal("The compressed Projects fixture must fit under the wire limit.")
		}
		body = buf.Bytes()
	}
	contentType := "application/" + codec
	if route.watch {
		contentType = "application/connect+" + codec
		frame := make([]byte, 5, 5+len(body))
		if compressed {
			frame[0] = 1
		}
		binary.BigEndian.PutUint32(frame[1:], uint32(len(body)))
		body = append(frame, body...)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+route.procedure, bytes.NewReader(body))
	if err != nil {
		t.Fatal("Building the Projects request failed.")
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Connect-Protocol-Version", "1")
	if compressed && route.watch {
		req.Header.Set("Connect-Content-Encoding", "gzip")
	} else if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if !route.watch {
		req.Header.Set(store.RequestKeyHeader, "router test request")
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	}
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal("Sending the Projects request failed.")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal("Reading the Projects reply failed.")
	}
	return projectReply{res.StatusCode, raw, res.Header, res.Trailer}
}

func (r projectReply) contains(marker string) bool {
	if bytes.Contains(r.body, []byte(marker)) {
		return true
	}
	for _, headers := range []http.Header{r.header, r.trailer} {
		for _, values := range headers {
			for _, value := range values {
				if strings.Contains(value, marker) {
					return true
				}
			}
		}
	}
	return false
}

func (r projectReply) requireSuccess(t *testing.T, route projectRoute) {
	t.Helper()
	if r.status != http.StatusOK {
		t.Errorf("%s returned HTTP %d, want HTTP 200.", route.name, r.status)
		return
	}
	if route.watch && (len(r.body) < 5 || r.body[0]&^byte(1) != 0) {
		t.Error("The signed-in watch did not return a data frame.")
	}
}

func (r projectReply) requireError(t *testing.T, route projectRoute, code connect.Code, message string) {
	t.Helper()
	type publicError struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	var reply publicError
	if route.watch {
		if r.status != http.StatusOK {
			t.Errorf("%s returned HTTP %d, want an HTTP 200 Connect stream with %s.", route.name, r.status, code)
			return
		}
		var terminal struct {
			Error publicError `json:"error"`
		}
		flags := byte(0x02)
		if len(r.body) > 0 && r.body[0] == 0x03 {
			flags = 0x03
		}
		payload := malformedReplyFrame(t, r.body, flags)
		if flags == 0x03 {
			if r.header.Get("Connect-Content-Encoding") != "gzip" {
				t.Fatal("The compressed Projects watch reply did not declare gzip.")
			}
			reader, err := gzip.NewReader(bytes.NewReader(payload))
			if err != nil {
				t.Fatal("The Projects watch reply did not contain a gzip stream.")
			}
			defer reader.Close()
			payload, err = io.ReadAll(reader)
			if err != nil {
				t.Fatal("Reading the compressed Projects watch reply failed.")
			}
		}
		if err := json.Unmarshal(payload, &terminal); err != nil {
			t.Fatal("The Projects watch reply did not contain a Connect terminal error.")
		}
		reply = terminal.Error
	} else if err := json.Unmarshal(r.body, &reply); err != nil {
		t.Fatalf("The Projects unary reply was HTTP %d, want a Connect %s error.", r.status, code)
	}
	if reply.Code != code.String() || (message != "" && reply.Message != message) {
		t.Errorf("The %s reply must use Connect %s and the expected public message. HTTP status was %d.", route.name, code, r.status)
	}
}
