package linktest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

const reservedResultMarker = "linktest-reserved-result-canary-7c1982b1"

func TestLinkReservedCommandResult(t *testing.T) {
	requirePostgres(t)
	logs := &diagnostics{marker: reservedResultMarker}
	server := servertest.Start(t, servertest.Options{Password: fixturePassword, Log: logs})
	page := server.Clients(t)
	field := protowire.AppendString(protowire.AppendTag(nil, 4, protowire.BytesType), reservedResultMarker)
	var relayed atomic.Int32
	r := startRelayWithControls(t, server.Link(), relayControls{observeResult: func(result *linkv1.CommandResult) {
		if bytes.Equal(result.ProtoReflect().GetUnknown(), field) {
			relayed.Add(1)
		}
	}})
	opts := newClient(t, new(diagnostics))
	id := join(t, page, opts, r.address)
	const machineName = "Reserved result fixture"
	approve(t, page, id, machineName)
	db := openObserver(t, server)

	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport := &http.Transport{Protocols: protocols}
	defer transport.CloseIdleConnections()
	peer := linkv1connect.NewLinkServiceClient(&http.Client{Transport: transport}, r.address)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	stream := peer.Open(ctx)
	defer stream.CloseRequest()
	defer stream.CloseResponse()
	welcome := authenticateScriptedLink(t, stream, server.SigningPublicKey, opts, id, 1)
	if welcome.GetState().GetApproval() != linkv1.Approval_APPROVAL_APPROVED {
		t.Fatal("The scripted machine did not connect as approved.")
	}
	reply, err := stream.Receive()
	notice := reply.GetCommand()
	if err != nil || notice.GetSetState().GetState().GetName() != machineName || notice.GetAttempt() == 0 {
		t.Fatal("The relay did not deliver the stored approval notice to the scripted machine.")
	}
	delivery := commandDelivery{id: notice.GetId(), attempt: notice.GetAttempt()}
	// The command can reach the peer before the server commits its dispatch marker.
	waitFor(t, 5*time.Second, "the delivered approval notice to commit its dispatch marker", func() bool {
		return scalar[bool](t, db, "SELECT dispatched FROM link_commands WHERE id=$1", delivery.id)
	})
	requirePending(t, db, delivery)

	sendResult := func(attempt uint32) {
		t.Helper()
		result := &linkv1.CommandResult{CommandId: delivery.id, Attempt: attempt, Outcome: commandDone}
		result.ProtoReflect().SetUnknown(field)
		if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: result}}); err != nil {
			t.Fatal("The scripted machine could not send its command result.")
		}
	}
	sendResult(delivery.attempt)
	waitForStoredResult(t, db, delivery, "done")
	if relayed.Load() != 1 {
		t.Fatal("The committed result did not carry the reserved field through the relay.")
	}

	if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}}); err != nil {
		t.Fatal("The scripted machine could not send a heartbeat after the result.")
	}
	reply, err = stream.Receive()
	if err != nil || reply.GetHeartbeatAck() == nil {
		t.Fatal("The link did not acknowledge the heartbeat after the result.")
	}
	requireNoReservedResultMarker(t, reply)

	watch, err := page.WatchClients(ctx, connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err != nil {
		t.Fatal("The website watch could not open after the result committed.")
	}
	defer watch.Close()
	if !watch.Receive() {
		t.Fatal("The website watch did not return a snapshot after the result committed.")
	}
	rows := watch.Msg().GetAll().GetClients()
	if len(rows) != 1 || rows[0].GetId() != id || rows[0].GetName() != machineName {
		t.Fatal("The website watch did not show the fixture machine.")
	}
	requireNoReservedResultMarker(t, watch.Msg())

	finished := scalar[time.Time](t, db, "SELECT finished_at FROM link_commands WHERE id=$1", delivery.id)
	sendResult(0)
	_, err = stream.Receive()
	if connect.CodeOf(err) != connect.CodeInvalidArgument || relayed.Load() != 2 {
		t.Fatal("The relay did not carry the marked malformed result to the expected refusal.")
	}
	if strings.Contains(err.Error(), reservedResultMarker) ||
		strings.Contains(stream.ResponseTrailer().Get("Lgtm-Link-Refusal"), reservedResultMarker) {
		t.Fatal("The reserved command result field reached the link refusal.")
	}
	waitForStoredResult(t, db, delivery, "done")
	if current := scalar[time.Time](t, db, "SELECT finished_at FROM link_commands WHERE id=$1", delivery.id); !current.Equal(finished) {
		t.Fatal("The invalid result changed the stored finish time.")
	}
	requireNoStoredResultMarker(t, db)
	server.Stop(t)
	logs.mu.Lock()
	marked := logs.marked
	logs.mu.Unlock()
	if marked {
		t.Fatal("The reserved command result field reached the server log.")
	}
}

func requireNoReservedResultMarker(t *testing.T, reply proto.Message) {
	t.Helper()
	wire, err := proto.Marshal(reply)
	if err != nil || bytes.Contains(wire, []byte(reservedResultMarker)) {
		t.Fatal("The reply could not be verified as free of the reserved command result field.")
	}
}

func requireNoStoredResultMarker(t *testing.T, db *pgx.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	rows, err := db.Query(ctx, `
SELECT tablename FROM pg_catalog.pg_tables
WHERE schemaname = 'public' AND has_table_privilege(format('%I.%I', schemaname, tablename), 'SELECT')
ORDER BY tablename`)
	if err != nil {
		t.Fatal("The database observer could not list readable application tables.")
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(tables) == 0 {
		t.Fatal("The database observer did not find readable application tables.")
	}
	marker := []byte(reservedResultMarker)
	for _, table := range tables {
		qualified := pgx.Identifier{"public", table}.Sanitize()
		found := scalar[bool](t, db, `SELECT EXISTS (
SELECT 1 FROM `+qualified+` AS record
CROSS JOIN LATERAL (SELECT to_jsonb(record)::text AS body) AS contents
WHERE strpos(body, $1) > 0 OR strpos(body, $2) > 0 OR strpos(body, $3) > 0
)`, reservedResultMarker, hex.EncodeToString(marker), base64.StdEncoding.EncodeToString(marker))
		if found {
			t.Fatal("The reserved command result field reached a readable database record.")
		}
	}
}
