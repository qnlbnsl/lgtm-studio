package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSprintFourRelayArmsAfterMalformedControl(t *testing.T) {
	upstream := newRelayPeer(t, false)
	dir := t.TempDir()
	relay := startReviewRelay(t, upstream.address, dir)
	serverKey := checkJoin(t, relay.client, upstream, false)
	stream, _ := openLink(t, t.Context(), relay.client, upstream, serverKey, 2)
	streamID := waitObservation(t, dir, func(o observation) bool {
		return o.Action == "forwarded" && o.Frame == "welcome"
	}).StreamID
	requireStreamID(t, streamID)

	const generation = 1
	event := eventSelector(7)
	replaceControl(t, dir, json.RawMessage(`" "`), math.MaxUint64, event)
	// The relay reads the control before forwarding the heartbeat.
	heartbeatBarrier(t, stream)
	replaceControl(t, dir, streamID, generation, event)
	sendFrame(t, stream, eventFrame(7))
	decision := waitObservation(t, dir, func(o observation) bool {
		return bytes.Equal(o.StreamID, streamID) && o.frameSelector == event &&
			(o.Action == "held" || o.Action == "forwarded")
	})
	if decision.Action != "held" || decision.Generation != generation {
		t.Fatal("The valid control did not hold the live stream's event after a malformed control.")
	}
	waitRule(t, dir, streamID, generation, "armed", event)
	heartbeatBarrier(t, stream)
	upstream.requireTraffic(t, 2, "heartbeat", "heartbeat")
}

func TestSprintFourRelayPreservesEventGapTrailer(t *testing.T) {
	serverPublic, serverKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("Could not generate the fixture server's signing key.")
	}
	machinePublic, machineKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("Could not generate the fixture machine's signing key.")
	}

	const mark, gap = 5, 7
	upstream := httptest.NewUnstartedServer(connect.NewBidiStreamHandler(
		linkv1connect.LinkServiceOpenProcedure,
		func(_ context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
			return refuseRelayEventGap(stream, serverKey, machinePublic, mark)
		},
	))
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	upstream.Config.Protocols = protocols
	upstream.Start()
	t.Cleanup(upstream.Close)
	t.Cleanup(upstream.CloseClientConnections)
	relay := startReviewRelay(t, upstream.URL, t.TempDir())
	stream, _ := openLink(t, t.Context(), relay.client, &relayPeer{machineKey: machineKey}, serverPublic, 2)
	sendFrame(t, stream, eventFrame(gap))
	frame, err := stream.Receive()
	if frame != nil || connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatal("The gap did not end the authenticated stream with failed_precondition.")
	}
	const header = "Lgtm-Link-Refusal"
	if len(stream.ResponseHeader().Values(header)) != 0 {
		t.Fatal("The relay sent the event gap reason in a leading response header.")
	}
	if !slices.Equal(stream.ResponseTrailer().Values(header), []string{"event_gap"}) {
		t.Fatal("The relay did not deliver exactly one event_gap refusal in the downstream trailer.")
	}
}

func refuseRelayEventGap(stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], serverKey ed25519.PrivateKey, machineKey ed25519.PublicKey, mark uint64) error {
	first, err := stream.Receive()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello.GetMachineId() != fixtureMachine || hello.GetLinkRevision() != 2 || len(hello.GetMachineNonce()) != 32 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("The fixture received an invalid Hello."))
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("The fixture could not generate a nonce."))
	}
	challenge := &linkv1.Challenge{ServerNonce: nonce,
		ServerSignature: ed25519.Sign(serverKey, proofText("server", hello.GetMachineNonce(), nonce))}
	if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: challenge}}); err != nil {
		return err
	}
	second, err := stream.Receive()
	if err != nil {
		return err
	}
	if !ed25519.Verify(machineKey, proofText("machine", hello.GetMachineNonce(), nonce), second.GetProof().GetMachineSignature()) {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("The fixture could not verify Proof."))
	}
	welcome := &linkv1.Welcome{
		LinkRevision: 2, ConnectionToken: "fixture-only-connection-token", LastEventNumber: mark,
		ServerTime: timestamppb.New(time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)),
		State:      &linkv1.MachineState{Revision: 1, Approval: linkv1.Approval_APPROVAL_APPROVED},
	}
	if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: welcome}}); err != nil {
		return err
	}
	request, err := stream.Receive()
	if err != nil {
		return err
	}
	number := request.GetEvent().GetNumber()
	if number <= mark+1 || number > math.MaxInt64 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("The fixture did not receive an event above the next number."))
	}
	stream.ResponseTrailer().Set("Lgtm-Link-Refusal", "event_gap")
	return connect.NewError(connect.CodeFailedPrecondition, errors.New("The event leaves a gap after the server's mark."))
}
