package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	linkv1connect "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/app"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

// Existing callers can still start the application through its sign-in entry point.
var _ func(context.Context, config.Config, *slog.Logger) (*app.Server, error) = app.Start

const machinesFixturePassword = "fake-machines-owner-password"

func TestServerServesMachines(t *testing.T) {
	s := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	unsigned := &http.Client{Transport: s.HTTPClient.Transport, Timeout: 10 * time.Second}

	t.Run("watch needs a session", func(t *testing.T) {
		client := lgtmv1connect.NewClientServiceClient(unsigned, s.URL)
		stream, err := client.WatchClients(t.Context(), connect.NewRequest(&webv1.WatchClientsRequest{}))
		if err == nil {
			defer stream.Close()
			if stream.Receive() {
				t.Fatal("The unsigned watch received a message.")
			}
			err = stream.Err()
		}
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("The unsigned watch returned %v, want unauthenticated.", err)
		}
	})

	t.Run("oversized join", func(t *testing.T) {
		client := linkv1connect.NewLinkServiceClient(unsigned, s.URL)
		req := connect.NewRequest(&linkv1.JoinRequest{HostName: strings.Repeat("x", (64<<10)+1)})
		_, err := client.Join(t.Context(), req)
		if connect.CodeOf(err) != connect.CodeResourceExhausted {
			t.Errorf("The oversized Join returned %v, want resource_exhausted.", err)
		}
	})

	t.Run("open refuses an empty first frame", func(t *testing.T) {
		client := linkv1connect.NewLinkServiceClient(unsigned, s.URL, connect.WithGRPC())
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		stream := client.Open(ctx)
		defer stream.CloseResponse()
		if err := stream.Send(&linkv1.OpenRequest{}); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("Sending to Open failed: %v", err)
		}
		if err := stream.CloseRequest(); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("Closing the request side of Open failed: %v", err)
		}
		requireAppOpenStatus(t, stream, connect.CodeInvalidArgument, "The link message broke a rule of the contract.")
	})

	t.Run("the website issues a token and the machine joins without a cookie", func(t *testing.T) {
		clients := s.Clients(t)
		if clients == nil {
			t.Fatal("The fixture did not provide a signed-in ClientService client.")
		}
		watch := fixtureMachinesWatch(t, clients)
		if list := fixtureMachinesSnapshot(t, watch); len(list) != 0 {
			t.Fatal("The new server started with a nonempty list of machines.")
		}
		joined := joinFixtureMachine(t, s, clients)
		if !watch.Receive() {
			t.Fatalf("The live watch ended before the Join appeared: %v", watch.Err())
		}
		saved := watch.Msg().GetSaved()
		if saved.GetId() != joined || saved.GetState() != webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL {
			t.Error("The live watch did not show the newly joined machine as waiting.")
		}
	})
}

func TestRestartKeepsMachines(t *testing.T) {
	s := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	originalKey := bytes.Clone(s.SigningPublicKey)
	clients := s.Clients(t)
	if clients == nil {
		t.Fatal("The fixture did not provide a signed-in ClientService client.")
	}
	id := joinFixtureMachine(t, s, clients)
	reply, err := clients.ApproveClient(t.Context(), connect.NewRequest(&webv1.ApproveClientRequest{Id: id, Name: "Box"}))
	if err != nil || reply.Msg.GetClient().GetState() != webv1.ClientState_CLIENT_STATE_OFFLINE {
		t.Fatalf("Approving the machine before restart did not return an offline client: %v", err)
	}

	s.Restart(t)
	if len(originalKey) != ed25519.PublicKeySize || !bytes.Equal(originalKey, s.SigningPublicKey) {
		t.Error("Restart changed the server's signing public key.")
	}
	watch := fixtureMachinesWatch(t, clients)
	list := fixtureMachinesSnapshot(t, watch)
	if len(list) != 1 || list[0].GetId() != id || list[0].GetName() != "Box" ||
		list[0].GetState() != webv1.ClientState_CLIENT_STATE_OFFLINE ||
		list[0].GetTokenId() != "" {
		t.Error("Restart did not preserve the approved, offline machine in the website's snapshot.")
	}
}

func joinFixtureMachine(t *testing.T, s *servertest.Server, clients lgtmv1connect.ClientServiceClient) string {
	t.Helper()
	reply, err := clients.CreateClientToken(t.Context(), connect.NewRequest(&webv1.CreateClientTokenRequest{}))
	if err != nil {
		t.Fatalf("Creating a token through the running server failed: %v", err)
	}
	token, err := machines.DecodeToken(reply.Msg.GetToken())
	if err != nil {
		t.Fatal("The running server's token did not decode.")
	}
	if reply.Msg.GetAddress() != s.URL || token.GetAddress() != s.URL {
		t.Fatal("The token did not name the server's configured public URL.")
	}
	publicKeyHash := sha256.Sum256(s.SigningPublicKey)
	if !bytes.Equal(token.GetServerKeySha256(), publicKeyHash[:]) {
		t.Error("The token did not bind the application's signing public key.")
	}

	link := s.Link()
	if link == nil {
		t.Fatal("The fixture did not provide a LinkService client.")
	}
	var sentCookie atomic.Bool
	ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
		WroteHeaderField: func(key string, _ []string) {
			if strings.EqualFold(key, "Cookie") {
				sentCookie.Store(true)
			}
		},
	})
	machineKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	joined, err := link.Join(ctx, connect.NewRequest(&linkv1.JoinRequest{
		TokenId: token.GetTokenId(), TokenSecret: token.GetSecret(), MachinePublicKey: machineKey, HostName: "machine.example.test",
	}))
	if sentCookie.Load() {
		t.Error("Link sent a website session cookie on Join.")
	}
	if err != nil || joined.Msg.GetMachineId() == "" {
		t.Fatalf("Joining through the running server did not produce a machine identifier: %v", err)
	}
	if !bytes.Equal(joined.Msg.GetServerPublicKey(), s.SigningPublicKey) {
		t.Error("Join returned a signing public key different from the application's.")
	}
	return joined.Msg.GetMachineId()
}

func fixtureMachinesWatch(t *testing.T, clients lgtmv1connect.ClientServiceClient) *connect.ServerStreamForClient[webv1.WatchClientsResponse] {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	stream, err := clients.WatchClients(ctx, connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err != nil {
		t.Fatalf("Opening the signed-in watch failed: %v", err)
	}
	t.Cleanup(func() { _ = stream.Close() })
	return stream
}

func fixtureMachinesSnapshot(t *testing.T, stream *connect.ServerStreamForClient[webv1.WatchClientsResponse]) []*webv1.Client {
	t.Helper()
	if !stream.Receive() || stream.Msg().GetAll() == nil {
		t.Fatalf("The signed-in watch did not return its snapshot: %v", stream.Err())
	}
	return stream.Msg().GetAll().GetClients()
}
