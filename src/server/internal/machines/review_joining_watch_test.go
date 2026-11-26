package machines_test

import (
	"context"
	"crypto/ed25519"
	"log/slog"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestWatchClientsShowsJoiningWithoutMachineTraffic(t *testing.T) {
	f := newStreamRecordsFixture(t)
	admitted := make(chan string, 1)
	releaseWelcome, resumeWelcome := storetest.Signal(t)
	seams := link.Seams{AfterAdmission: func(ctx context.Context, id string) {
		admitted <- id
		select {
		case <-releaseWelcome:
		case <-ctx.Done():
		}
	}}
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	service := link.NewStreamingService(t.Context(), f.s, f.tokens, f.reg, f.presence.State, f.hub, key,
		link.Options{Seams: seams, Logger: slog.New(slog.DiscardHandler)})
	_, route := linkv1connect.NewLinkServiceHandler(service)
	server := machinetest.NewServer(t, route, storetest.ObservationTimeout)
	defer resumeWelcome()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	httpClient := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	t.Cleanup(httpClient.CloseIdleConnections)
	client := linkv1connect.NewLinkServiceClient(httpClient, server.URL)

	token, err := f.tokens.Create(t.Context(), machines.TokenOptions{})
	if err != nil {
		t.Fatal("The fixture could not issue a join token.")
	}
	machine, err := machinetest.Join(t.Context(), client, token.Text, "joining watch machine")
	if err != nil {
		t.Fatal("The fixture machine could not join.")
	}
	if _, err := f.approve(t, machine.ID, ""); err != nil {
		t.Fatal("The fixture machine could not be approved.")
	}

	openWatch := func() *connect.ServerStreamForClient[webv1.WatchClientsResponse] {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
		t.Cleanup(cancel)
		w, err := f.clients.WatchClients(ctx, recordRequest(f.recordsFixture, &webv1.WatchClientsRequest{}, ""))
		if err != nil {
			t.Fatal("The signed-in Machines watch could not open.")
		}
		t.Cleanup(func() { _ = w.Close() })
		if !w.Receive() || w.Msg().GetAll() == nil {
			t.Fatal("The signed-in Machines watch did not receive its snapshot.")
		}
		return w
	}
	watch := openWatch()
	before := watch.Msg().GetAll().GetClients()
	if len(before) != 1 || before[0].GetId() != machine.ID {
		t.Fatal("The watch snapshot did not contain the approved fixture machine.")
	}
	requireProjectedState(t, before[0], webv1.ClientState_CLIENT_STATE_OFFLINE)
	snapshotPosition := watch.Msg().GetPosition().GetValue()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	handshake := storetest.Start(t, func() *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse] {
		stream, _, _ := machine.Connect(ctx, client)
		return stream
	})
	if storetest.Receive(t, "the real stream held after admission", admitted) != machine.ID {
		t.Fatal("The admission hook ran for a different machine.")
	}
	streamID := storetest.Scalar[string](t, f.obs, "SELECT stream_id FROM link_connections WHERE machine_id = $1", machine.ID)
	admissionPosition := uint64(f.head(t))
	if !watch.Receive() || admissionPosition <= snapshotPosition || watch.Msg().GetPosition().GetValue() != admissionPosition ||
		watch.Msg().GetSaved().GetId() != machine.ID {
		t.Fatal("The open watch did not consume the committed admission while Welcome was held.")
	}
	projected := watch.Msg().GetSaved()

	resumeWelcome()
	stream := handshake.Wait(t)
	if stream == nil {
		t.Fatal("The real stream did not receive a valid Welcome.")
	}
	storetest.Poll(t, "the admitted stream becoming live", func() bool {
		return f.hub.Holds(machine.ID, streamID)
	})
	fresh := openWatch()
	defer fresh.Close()
	after := fresh.Msg().GetAll().GetClients()
	if len(after) != 1 || after[0].GetId() != machine.ID {
		t.Fatal("The fresh Machines snapshot did not contain the connected machine.")
	}
	requireProjectedState(t, after[0], webv1.ClientState_CLIENT_STATE_JOINING)

	// An admission update can already show Joining under the frozen contract.
	if projected.GetState() != webv1.ClientState_CLIENT_STATE_JOINING {
		if !watch.Receive() {
			t.Fatalf("The open watch stayed %s after Welcome while a fresh snapshot showed Joining. Watch ended with %s.",
				projected.GetState(), connect.CodeOf(watch.Err()))
		}
		projected = watch.Msg().GetSaved()
	}
	requireProjectedState(t, projected, webv1.ClientState_CLIENT_STATE_JOINING)
	if !proto.Equal(projected, after[0]) {
		t.Fatal("The open watch's Joining record did not match the fresh snapshot.")
	}
}
