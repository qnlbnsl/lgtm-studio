package machines_test

import (
	"context"
	"crypto/ed25519"
	"log/slog"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestWatchKeepsJoiningThroughReplacementRollback(t *testing.T) {
	f := newStreamRecordsFixture(t)
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	service := link.NewStreamingService(t.Context(), f.s, f.tokens, f.reg, f.presence.State, f.hub, key,
		link.Options{Logger: slog.New(slog.DiscardHandler)})
	_, route := linkv1connect.NewLinkServiceHandler(service)
	server := machinetest.NewServer(t, route, storetest.ObservationTimeout)
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	httpClient := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	t.Cleanup(httpClient.CloseIdleConnections)
	client := linkv1connect.NewLinkServiceClient(httpClient, server.URL)

	token, err := f.tokens.Create(t.Context(), machines.TokenOptions{})
	if err != nil {
		t.Fatal("The fixture could not issue a join token.")
	}
	machine, err := machinetest.Join(t.Context(), client, token.Text, "watch during replacement")
	if err != nil {
		t.Fatal("The fixture machine could not join.")
	}
	if _, err := f.approve(t, machine.ID, ""); err != nil {
		t.Fatal("The fixture machine could not be approved.")
	}
	ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
	defer cancel()
	original, _, err := machine.Connect(ctx, client)
	if err != nil {
		t.Fatal("The original stream did not receive a valid Welcome.")
	}
	t.Cleanup(func() {
		_ = original.CloseRequest()
		_ = original.CloseResponse()
	})
	streamID := storetest.Scalar[string](t, f.obs, `SELECT stream_id FROM link_connections WHERE machine_id = $1`, machine.ID)
	storetest.Poll(t, "the original stream finishing Welcome", func() bool {
		return f.hub.Holds(machine.ID, streamID)
	})
	requireProjectedState(t, f.snapshot(t), webv1.ClientState_CLIENT_STATE_JOINING)
	before := f.head(t)

	storetest.Exec(t, f.obs, `CREATE FUNCTION reject_replacement_for_watch() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'The fixture rejected stream replacement.'; END; $$`)
	storetest.Exec(t, f.obs, `CREATE TRIGGER reject_replacement_for_watch BEFORE UPDATE OF stream_id ON link_connections
		FOR EACH ROW EXECUTE FUNCTION reject_replacement_for_watch()`)
	block, err := f.obs.Begin(t.Context())
	if err != nil {
		t.Fatal("The observer could not start the machine row lock transaction.")
	}
	defer func() { _ = block.Rollback(t.Context()) }()
	storetest.Exec(t, block, "SELECT id FROM machines WHERE id = $1 FOR NO KEY UPDATE", machine.ID)

	replacement := storetest.Start(t, func() error {
		stream, _, err := machine.Connect(ctx, client)
		if stream != nil {
			_ = stream.CloseRequest()
			_ = stream.CloseResponse()
		}
		return err
	})
	storetest.Poll(t, "the replacement waiting on the held machine row", func() bool {
		return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
			SELECT 1 FROM pg_locks WHERE locktype = 'transactionid'
			AND NOT granted AND $1 = ANY(pg_blocking_pids(pid)))`, int32(f.obs.PgConn().PID()))
	})
	if f.hub.Holds(machine.ID, streamID) {
		t.Fatal("The live action gate admitted the original stream during replacement.")
	}
	watch := f.watch(t)
	defer watch.Close()
	clients := watch.Msg().GetAll().GetClients()
	if len(clients) != 1 || clients[0].GetId() != machine.ID {
		t.Fatal("The open watch did not select the original machine during replacement.")
	}
	projected := clients[0]

	if block.Rollback(t.Context()) != nil {
		t.Fatal("The observer could not release the machine row lock.")
	}
	if err := replacement.Wait(t); !connect.IsWireError(err) || connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatal("The server did not refuse the replacement whose write rolled back.")
	}
	if current := storetest.Scalar[string](t, f.obs, `SELECT stream_id FROM link_connections WHERE machine_id = $1`, machine.ID); current != streamID {
		t.Fatal("Rollback did not preserve the original committed stream.")
	}
	storetest.Poll(t, "rollback restoring the original stream's action gate", func() bool {
		return f.hub.Holds(machine.ID, streamID)
	})
	fresh := f.snapshot(t)
	requireProjectedState(t, fresh, webv1.ClientState_CLIENT_STATE_JOINING)
	if f.head(t) != before {
		t.Fatal("The feed advanced during the rolled-back replacement without machine traffic.")
	}
	if !proto.Equal(projected, fresh) {
		t.Errorf("The open watch showed %s through replacement rollback, want %s.", projected.GetState(), fresh.GetState())
	}
}
