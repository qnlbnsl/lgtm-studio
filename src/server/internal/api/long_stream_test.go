package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	linkrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

type watchDeadlineProbe struct {
	webrpc.UnimplementedClientServiceHandler
}

func (watchDeadlineProbe) WatchClients(ctx context.Context, _ *connect.Request[webv1.WatchClientsRequest], stream *connect.ServerStream[webv1.WatchClientsResponse]) error {
	return transport.Send(ctx, func() error {
		return stream.Send(&webv1.WatchClientsResponse{Change: &webv1.WatchClientsResponse_RemovedId{RemovedId: "watch probe"}})
	})
}

type linkDeadlineProbe struct {
	linkrpc.UnimplementedLinkServiceHandler
	joins atomic.Int32
}

func (p *linkDeadlineProbe) Join(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	p.joins.Add(1)
	return connect.NewResponse(&linkv1.JoinResponse{MachineId: "join probe"}), nil
}

func (*linkDeadlineProbe) Open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	if _, err := stream.Receive(); err != nil {
		return err
	}
	return transport.Send(ctx, func() error {
		return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}})
	})
}

func TestRouterWrapsLongStreams(t *testing.T) {
	fixture := newSessionDatabase(t)
	token, err := fixture.sessions.Issue(t.Context())
	if err != nil {
		t.Fatalf("Issuing the probe session failed: %v", err)
	}
	link := &linkDeadlineProbe{}
	server := httptest.NewUnstartedServer(api.NewHandler(api.Options{
		Sessions: fixture.sessions,
		Origins:  testOriginPolicy(t, loopbackOrigin),
		Clients:  watchDeadlineProbe{},
		Link:     link,
	}))
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	t.Cleanup(client.CloseIdleConnections)

	t.Run("WatchClients", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		web := webrpc.NewClientServiceClient(client, server.URL)
		req := connect.NewRequest(&webv1.WatchClientsRequest{})
		req.Header().Set("Cookie", (&http.Cookie{Name: auth.CookieName, Value: token}).String())
		stream, err := web.WatchClients(ctx, req)
		if err != nil {
			t.Fatalf("Opening the WatchClients probe failed: %v", err)
		}
		defer stream.Close()
		if !stream.Receive() || stream.Msg().GetRemovedId() != "watch probe" {
			t.Fatalf("The WatchClients deadline probe got %v with %v, want the probe's streamed response.", stream.Msg(), stream.Err())
		}
		if stream.Receive() || stream.Err() != nil {
			t.Fatalf("The WatchClients deadline probe did not close successfully: %v", stream.Err())
		}
	})

	t.Run("Open", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		stream := linkrpc.NewLinkServiceClient(client, server.URL, connect.WithGRPC()).Open(ctx)
		defer stream.CloseResponse()
		if err := stream.Send(&linkv1.OpenRequest{}); err != nil {
			t.Fatalf("Sending to the Open probe failed: %v", err)
		}
		if err := stream.CloseRequest(); err != nil {
			t.Fatalf("Closing the Open request failed: %v", err)
		}
		res, err := stream.Receive()
		if err != nil || res.GetHeartbeatAck() == nil {
			t.Fatalf("The Open deadline probe got %v with %v, want the probe's streamed response.", res, err)
		}
	})

	t.Run("Join limit", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		rpc := linkrpc.NewLinkServiceClient(client, server.URL)
		const limit = 64 << 10
		join := func(size int) (*connect.Response[linkv1.JoinResponse], error) {
			// The bytes field adds one tag byte and three length bytes at this size.
			msg := &linkv1.JoinRequest{TokenSecret: make([]byte, size-4)}
			if got := proto.Size(msg); got != size {
				t.Fatalf("The Join fixture is %d bytes, want %d.", got, size)
			}
			return rpc.Join(ctx, connect.NewRequest(msg))
		}
		if _, err := join(limit + 1); connect.CodeOf(err) != connect.CodeResourceExhausted {
			t.Errorf("Join one byte over the limit returned %v, want resource_exhausted.", err)
		}
		if got := link.joins.Load(); got != 0 {
			t.Errorf("The Join probe ran %d times for a message over the limit.", got)
		}
		if res, err := join(limit); err != nil || res.Msg.GetMachineId() != "join probe" {
			t.Fatalf("Join at the limit got %v with %v, want the probe's response.", res, err)
		}
		if got := link.joins.Load(); got != 1 {
			t.Errorf("The Join probe ran %d times across two calls, want one.", got)
		}
		if _, exposed := reflect.TypeFor[api.Options]().FieldByName("LinkOptions"); exposed {
			t.Error("The router exposes LinkOptions, want a fixed limit without a caller option.")
		}
	})
}
