package servertest_test

import (
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

var (
	_ func(testing.TB, servertest.Options) *servertest.Server = servertest.Start
	_ func(*servertest.Server, testing.TB)                    = (*servertest.Server).Restart
	_ func(*servertest.Server, testing.TB)                    = (*servertest.Server).Stop
)

func TestMachinesFixtureClientsAndLink(t *testing.T) {
	const password = "fake-machines-fixture-password"
	s := servertest.Start(t, servertest.Options{Password: password})
	session := lgtmv1connect.NewSessionServiceClient(s.HTTPClient, s.URL)
	requireGetServer(t, session, false)

	t.Run("Clients signs in through the fixture jar", func(t *testing.T) {
		clients := s.Clients(t)
		if clients == nil {
			t.Fatal("Clients did not return a ClientService client.")
		}
		requireGetServer(t, session, true)
		ctx, cancel := contextWithRequestBudget(t)
		defer cancel()
		watch, err := clients.WatchClients(ctx, connect.NewRequest(&webv1.WatchClientsRequest{}))
		if err != nil {
			t.Fatalf("The fixture's signed-in watch did not open: %v", err)
		}
		defer watch.Close()
		if !watch.Receive() || watch.Msg().GetAll() == nil {
			t.Fatalf("The fixture's signed-in watch did not receive a snapshot: %v", watch.Err())
		}
	})

	t.Run("Link leaves the website cookie in the jar", func(t *testing.T) {
		// The jar must contain a live session even if the Clients assertion failed.
		signInFixture(t, session, password)
		link := s.Link()
		if link == nil {
			t.Fatal("Link did not return a LinkService client.")
		}
		var sentCookie atomic.Bool
		ctx, cancel := contextWithRequestBudget(t)
		defer cancel()
		ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
			WroteHeaderField: func(key string, _ []string) {
				if strings.EqualFold(key, "Cookie") {
					sentCookie.Store(true)
				}
			},
		})
		_, err := link.Join(ctx, connect.NewRequest(&linkv1.JoinRequest{}))
		if sentCookie.Load() {
			t.Error("Link put the fixture's website cookie on its Join request.")
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("Link sent an empty Join and received %v, want invalid_argument.", err)
		}
		requireGetServer(t, session, true)
	})
}
