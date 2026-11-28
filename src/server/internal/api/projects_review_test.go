package api_test

import (
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

func TestProjectRouterUnaryRequiresProtocolVersion(t *testing.T) {
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	probe := &projectRouterProbe{}
	handler := projectRouter(t, fixture, probe)
	const refusal = `missing required header: set Connect-Protocol-Version to "1"`

	for _, route := range projectRoutes {
		if route.watch {
			continue
		}
		t.Run(route.name, func(t *testing.T) {
			request := jsonProcedure(t, route.procedure, string(route.jsonBody(t, "/checkout")))
			request.Header.Set("Origin", projectAllowedOrigin)
			request.Header.Set(store.RequestKeyHeader, "router protocol version test")
			request.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
			request.Header.Del("Connect-Protocol-Version")

			before := probe.calls.Load()
			reply := serveRequest(handler, request)
			if probe.calls.Load() != before {
				t.Error("A unary request without the protocol version reached the Projects handler.")
			}
			requireConnectJSONError(t, reply, http.StatusBadRequest, connect.CodeInvalidArgument, refusal)
		})
	}
}

func TestProjectRouterWatchWriteDeadlineWithinSendBudget(t *testing.T) {
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	handler := projectRouter(t, fixture, &projectRouterProbe{})
	observed := make(chan []time.Duration, 1)
	server := startHTTP2(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if _, ok := req.Context().Deadline(); ok || req.Header.Get("Connect-Timeout-Ms") != "" {
			t.Error("The watch fixture request must have no earlier deadline.")
		}
		writer := &projectBudgetWriter{ResponseWriter: w}
		handler.ServeHTTP(writer, req)
		observed <- writer.budgets
	}))
	client := webrpc.NewProjectServiceClient(server.Client(), server.URL)
	request := connect.NewRequest(&webv1.WatchRepositoriesRequest{})
	request.Header().Set("Cookie", (&http.Cookie{Name: auth.CookieName, Value: token}).String())
	stream, err := client.WatchRepositories(t.Context(), request)
	if err != nil {
		t.Fatalf("Opening the Projects watch failed with %v.", err)
	}
	defer stream.Close()
	if !stream.Receive() || stream.Msg().GetAll() == nil || stream.Receive() || stream.Err() != nil {
		t.Fatalf("The Projects watch must send one snapshot and close successfully. The stream error was %v.", stream.Err())
	}

	budgets := storetest.Receive(t, "the Projects watch write deadlines", observed)
	if len(budgets) == 0 {
		t.Fatal("The real response writer did not accept a write deadline.")
	}
	for _, budget := range budgets {
		if budget <= 0 || budget > transport.SendTimeout {
			t.Errorf("The real response writer received a deadline %v away, want a future deadline within %v.", budget, transport.SendTimeout)
		}
	}
}

type projectBudgetWriter struct {
	http.ResponseWriter
	budgets []time.Duration
}

func (w *projectBudgetWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *projectBudgetWriter) Flush() { _ = http.NewResponseController(w.ResponseWriter).Flush() }

func (w *projectBudgetWriter) SetWriteDeadline(at time.Time) error {
	remaining := time.Until(at)
	if err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(at); err != nil {
		return err
	}
	if !at.IsZero() {
		w.budgets = append(w.budgets, remaining)
	}
	return nil
}
