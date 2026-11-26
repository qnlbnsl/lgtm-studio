package machines

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const terminalWatchWriteLimit = 200 * time.Millisecond

// shortWatchDeadline lowers the real HTTP/2 deadline when the server arms one.
type shortWatchDeadline struct {
	http.ResponseWriter
}

func (w shortWatchDeadline) Flush() {
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w shortWatchDeadline) SetWriteDeadline(at time.Time) error {
	if limit := time.Now().Add(terminalWatchWriteLimit); at.After(limit) {
		at = limit
	}
	return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(at)
}

type watchServiceEnd struct {
	at   time.Time
	code connect.Code
}

type observedWatchService struct {
	*Service
	ended chan watchServiceEnd
}

func (s observedWatchService) WatchClients(ctx context.Context, req *connect.Request[lgtmv1.WatchClientsRequest], stream *connect.ServerStream[lgtmv1.WatchClientsResponse]) error {
	err := s.Service.WatchClients(ctx, req, stream)
	s.ended <- watchServiceEnd{at: time.Now(), code: connect.CodeOf(err)}
	return err
}

type watchLagGate struct {
	Link
	initialID string
	loading   chan struct{}
	lagged    chan struct{}
}

func (g watchLagGate) Presence(ctx context.Context, _ Querier, ids []string) (map[string]Presence, error) {
	if len(ids) == 1 && ids[0] == g.initialID {
		return nil, nil
	}
	close(g.loading)
	<-ctx.Done()
	close(g.lagged)
	return nil, context.Cause(ctx)
}

func appendUnrelatedWatchChanges(t *testing.T, s *store.Store) {
	t.Helper()
	for start := 0; start < store.MaxWatchLag; start += store.MaxWriteChanges {
		err := s.Write(t.Context(), func(_ context.Context, tx *store.Tx) error {
			for i := start; i < min(start+store.MaxWriteChanges, store.MaxWatchLag); i++ {
				if err := tx.Record(store.ChangeRef{
					Topic: "terminal-watch-fixture", Key: strconv.Itoa(i), Change: store.Saved,
				}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal("Recording changes for the lagging follower failed.")
		}
	}
}

func TestWatchClientsTerminalWriteDeadline(t *testing.T) {
	for _, tc := range []struct {
		name string
		code connect.Code
	}{
		{"revoked session", connect.CodeUnauthenticated},
		{"lagging feed", connect.CodeAborted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClientsWatchFixture(t)
			initial := f.create(t, strings.Repeat("x", 60))[0]
			head, err := f.store.Snapshot(t.Context(), func(context.Context, *store.ReadTx) error { return nil })
			if err != nil {
				t.Fatal("Reading the snapshot position failed.")
			}
			expected := &lgtmv1.WatchClientsResponse{
				Position: &lgtmv1.FeedPosition{Value: uint64(head)},
				Change: &lgtmv1.WatchClientsResponse_All{
					All: &lgtmv1.ClientList{Clients: []*lgtmv1.Client{Project(initial)}},
				},
			}
			const envelopeHeader = 5
			window := envelopeHeader + proto.Size(expected) + 4
			service := NewService(f.store, f.sessions, f.registry, nil)
			gate := watchLagGate{initialID: initial.ID, loading: make(chan struct{}), lagged: make(chan struct{})}
			if tc.code == connect.CodeAborted {
				service = service.WithLink(gate)
			}
			origins, err := auth.NewOriginPolicy("http://127.0.0.1:8787", nil)
			if err != nil {
				t.Fatal("Creating the watch origin policy failed.")
			}
			handlerDone := make(chan watchServiceEnd, 1)
			outerDone := make(chan time.Time, 1)
			router := api.NewHandler(api.Options{Sessions: f.sessions, Origins: origins, Clients: observedWatchService{service, handlerDone}})
			client, baseURL := serveClientsWatchTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				router.ServeHTTP(shortWatchDeadline{w}, r)
				outerDone <- time.Now()
			}), window)

			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			url := baseURL + lgtmv1connect.ClientServiceWatchClientsProcedure
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(make([]byte, envelopeHeader)))
			if err != nil {
				t.Fatal("Building the watch request failed.")
			}
			req.Header.Set("Content-Type", "application/connect+proto")
			req.Header.Set("Connect-Protocol-Version", "1")
			req.Header.Set("Cookie", f.cookie)
			res, err := client.Do(req)
			if err != nil {
				t.Fatal("Opening the signed-in watch failed.")
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK || res.ProtoMajor != 2 {
				t.Fatal("The signed-in watch did not open over HTTP/2.")
			}
			if size := int(watchFrameSize(t, res.Body, "snapshot")); window-envelopeHeader-size != 4 {
				t.Fatalf("The first frame has %d payload bytes. It must fit and leave four bytes of the initial window.", size)
			}
			// The client leaves the snapshot payload unread, with too little credit for the closing envelope.
			if tc.code == connect.CodeUnauthenticated {
				if err := f.sessions.Revoke(ctx, f.verifier); err != nil {
					t.Fatal("Revoking the watch session failed.")
				}
				f.create(t, "change after revocation")
			} else {
				// Hold a record load so the watch cannot acknowledge the backlog. The snapshot send has finished.
				f.create(t, "change held until the follower lags")
				storetest.Receive(t, "the watch entering the record load", gate.loading)
				appendUnrelatedWatchChanges(t, f.store)
				storetest.Receive(t, "the follower canceling the record load", gate.lagged)
			}

			var ended watchServiceEnd
			select {
			case ended = <-handlerDone:
				if ended.code != tc.code {
					t.Fatalf("The watch handler ended with %s, want %s.", ended.code, tc.code)
				}
			case <-ctx.Done():
				t.Fatal("The watch handler did not return after its watch ended.")
			}
			// The deadline starts before the handler returns. Allow time for the timer and server goroutines to run.
			returnBy := ended.at.Add(terminalWatchWriteLimit + 500*time.Millisecond)
			select {
			case returned := <-outerDone:
				if returned.After(returnBy) {
					t.Fatal("The outer ServeHTTP returned after the terminal write deadline.")
				}
			case <-time.After(time.Until(returnBy)):
				t.Fatal("The outer ServeHTTP was still blocked on the terminal write after its deadline.")
			}
		})
	}
}

func serveClientsWatchTest(t *testing.T, handler http.Handler, window int) (*http.Client, string) {
	t.Helper()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server := httptest.NewUnstartedServer(handler)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{
		Protocols: protocols,
		HTTP2:     &http.HTTP2Config{MaxReceiveBufferPerStream: window},
	}}
	t.Cleanup(client.CloseIdleConnections)
	return client, server.URL
}
