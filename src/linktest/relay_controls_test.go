package linktest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

type relayControlPeer struct {
	linkv1connect.UnimplementedLinkServiceHandler
	command *linkv1.Command
	arrived chan *linkv1.OpenRequest
}

func (p *relayControlPeer) Open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	for {
		req, err := stream.Receive()
		if err != nil {
			return err
		}
		select {
		case p.arrived <- req:
		case <-ctx.Done():
			return ctx.Err()
		}
		var reply *linkv1.OpenResponse
		switch {
		case req.GetHello() != nil && p.command != nil:
			reply = &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: p.command}}
		case req.GetEvent() != nil:
			reply = &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{EventAck: &linkv1.EventAck{Through: req.GetEvent().GetNumber()}}}
		case req.GetHeartbeat() != nil:
			reply = &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}}
		}
		if reply != nil {
			if err := stream.Send(reply); err != nil {
				return err
			}
		}
	}
}

func startRelayControlFixture(t *testing.T, controls relayControls, command *linkv1.Command) (*relay, *relayControlPeer, linkv1connect.LinkServiceClient) {
	t.Helper()
	peer := &relayControlPeer{arrived: make(chan *linkv1.OpenRequest, 16), command: command}
	_, handler := linkv1connect.NewLinkServiceHandler(peer)
	server := httptest.NewUnstartedServer(handler)
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	transport := &http.Transport{Protocols: protocols}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport}
	r := startRelayWithControls(t, linkv1connect.NewLinkServiceClient(client, server.URL), controls)
	return r, peer, linkv1connect.NewLinkServiceClient(client, r.address)
}

func openPairedStream(t *testing.T, client linkv1connect.LinkServiceClient) *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse] {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	stream := client.Open(ctx)
	t.Cleanup(func() {
		cancel()
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	})
	return stream
}

func sendPairedFrame(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], frame *linkv1.OpenRequest) {
	t.Helper()
	if err := stream.Send(frame); err != nil {
		t.Fatalf("The paired peer could not send its frame (%s).", connect.CodeOf(err))
	}
}

func receivePairedFrame(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]) *linkv1.OpenResponse {
	t.Helper()
	reply, err := stream.Receive()
	if err != nil {
		t.Fatalf("The paired peer could not receive its frame (%s).", connect.CodeOf(err))
	}
	return reply
}

func pairedHello(hello *linkv1.Hello) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Hello{Hello: hello}}
}

func pairedEvent(event *linkv1.Event) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}}
}

func pairedFacts(number uint64, version string) *linkv1.Event {
	return &linkv1.Event{Number: number, HappenedAt: timestamppb.Now(), Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{
		Os: "Fixture", Cores: 1, ClientVersion: version,
	}}}
}

func pairedFolder(number uint64, operationID, commandID string) *linkv1.Event {
	return &linkv1.Event{Number: number, HappenedAt: timestamppb.Now(), Report: &linkv1.Event_FolderSeen{FolderSeen: &linkv1.FolderSeen{
		OperationId: operationID, CommandId: commandID,
		Result: &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER},
	}}}
}

func pairedResult(id string, attempt uint32, number uint64) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: &linkv1.CommandResult{
		CommandId: id, Attempt: attempt, LastEventNumber: number, Outcome: commandDone,
	}}}
}

func TestRelaySprintFourControls(t *testing.T) {
	t.Run("FolderIdentity", func(t *testing.T) {
		const commandID, operationID = "folder-command", "folder-operation"
		command := &linkv1.Command{Id: commandID, Attempt: 2, Action: &linkv1.Command_LookAtFolder{
			LookAtFolder: &linkv1.LookAtFolder{OperationId: operationID},
		}}
		r, _, client := startRelayControlFixture(t, relayControls{}, command)
		stream := openPairedStream(t, client)
		sendPairedFrame(t, stream, pairedHello(&linkv1.Hello{}))
		leg := receive(t, r.legs, "the relay leg")
		if receivePairedFrame(t, stream).GetCommand().GetId() != commandID {
			t.Fatal("The relay did not deliver the folder command.")
		}
		sendPairedFrame(t, stream, pairedEvent(pairedFolder(5, operationID, commandID)))
		if receivePairedFrame(t, stream).GetEventAck().GetThrough() != 5 {
			t.Fatal("The relay did not deliver the folder acknowledgement.")
		}
		trace := leg.observe()
		if len(trace.commands) != 1 || trace.commands[0].kind != "look_at_folder" ||
			trace.commands[0].operationID != operationID || trace.commands[0].id != commandID || trace.commands[0].attempt != 2 ||
			!slices.Equal(trace.reports, []reportDelivery{{folder: folderReport{5, operationID, commandID}}}) {
			t.Fatalf("The relay did not record the folder command and report identity: %+v.", trace)
		}
	})

	t.Run("RunPair", func(t *testing.T) {
		r, peer, client := startRelayControlFixture(t, relayControls{}, nil)
		for _, pair := range []helloObservation{{"machine", "run-old", 2, 5}, {"machine", "run-new", 2, 8}} {
			stream := openPairedStream(t, client)
			sendPairedFrame(t, stream, pairedHello(&linkv1.Hello{
				MachineId: pair.machineID, RunId: pair.runID, LinkRevision: pair.revision, FirstEventNumber: pair.first,
			}))
			leg := receive(t, r.legs, "the relay leg")
			receive(t, peer.arrived, "Hello to cross the relay")
			if got := leg.observe().hello; got != pair {
				t.Fatalf("The relay recorded Hello as %+v, want %+v.", got, pair)
			}
		}
	})

	t.Run("SendOrder", func(t *testing.T) {
		r, _, client := startRelayControlFixture(t, relayControls{}, nil)
		stream := openPairedStream(t, client)
		sendPairedFrame(t, stream, pairedEvent(pairedFacts(11, "send order")))
		leg := receive(t, r.legs, "the relay leg")
		if receivePairedFrame(t, stream).GetEventAck().GetThrough() != 11 {
			t.Fatal("The event did not receive its acknowledgement.")
		}
		delivery := injectCommand(t, leg, &linkv1.Command{Id: "injected", Attempt: 2})
		if receivePairedFrame(t, stream).GetCommand().GetId() != delivery.id {
			t.Fatal("The injected command did not reach the peer.")
		}
		if want := []frameSend{{kind: "event", number: 11}, {kind: "ack", number: 11}, {kind: "command", commandID: "injected", attempt: 2}}; !slices.Equal(leg.observe().sends, want) {
			t.Fatalf("The relay send order was %v, want %v.", leg.observe().sends, want)
		}
	})

	t.Run("DroppedResultWatermark", func(t *testing.T) {
		input := make(chan uint64, 1)
		type observedDecision struct {
			result     noticeResult
			seenBefore bool
		}
		decision := make(chan observedDecision, 1)
		r, peer, client := startRelayControlFixture(t, relayControls{
			observeResult: func(result *linkv1.CommandResult) { input <- result.GetLastEventNumber() },
			dropResult: func(result noticeResult) bool {
				seen := false
				select {
				case value := <-input:
					seen = value == result.lastEventNumber
				default:
				}
				decision <- observedDecision{result, seen}
				return true
			},
		}, nil)
		stream := openPairedStream(t, client)
		sendPairedFrame(t, stream, pairedResult("dropped", 3, 27))
		leg := receive(t, r.legs, "the relay leg")
		want := noticeResult{"dropped", 3, commandDone, 27}
		if got := receive(t, decision, "the watermark before the drop decision"); got.result != want || !got.seenBefore ||
			!slices.Equal(leg.observe().results, []noticeResult{want}) {
			t.Fatal("The dropped result lost its watermark before the drop decision.")
		}
		sendPairedFrame(t, stream, pairedEvent(pairedFacts(28, "after the drop")))
		if first := receive(t, peer.arrived, "the first forwarded frame"); first.GetEvent().GetNumber() != 28 ||
			slices.ContainsFunc(leg.observe().sends, func(send frameSend) bool { return send.kind == "result" }) {
			t.Fatal("A dropped result appeared in the upstream requests or send trace.")
		}
	})

	t.Run("SelectiveHold", func(t *testing.T) {
		for _, mode := range []string{"ReleaseInOrder", "DiscardOnCut"} {
			t.Run(mode, func(t *testing.T) {
				release, held := make(chan struct{}), make(chan struct{})
				var armed atomic.Bool
				armed.Store(true)
				r, peer, client := startRelayControlFixture(t, relayControls{holdReport: func(report reportDelivery) <-chan struct{} {
					if report.folder == (folderReport{8, "selected-operation", "selected-command"}) && armed.Swap(false) {
						close(held)
						return release
					}
					return nil
				}}, nil)
				stream := openPairedStream(t, client)
				sendPairedFrame(t, stream, pairedEvent(pairedFolder(7, "other-operation", "other-command")))
				leg := receive(t, r.legs, "the relay leg")
				if frame := receive(t, peer.arrived, "the unrelated report"); frame.GetEvent().GetNumber() != 7 {
					t.Fatal("The selective hold delayed the unrelated report.")
				}
				receivePairedFrame(t, stream)
				sendPairedFrame(t, stream, pairedEvent(pairedFolder(8, "selected-operation", "selected-command")))
				receive(t, held, "the selected report to be held")
				sendPairedFrame(t, stream, pairedEvent(pairedFacts(9, "numbered suffix")))
				sendPairedFrame(t, stream, pairedResult("selected-command", 2, 8))
				if first := receive(t, peer.arrived, "the result ahead of the held report"); first.GetCommandResult().GetLastEventNumber() != 8 {
					t.Fatal("The held report or its numbered suffix crossed ahead of the result.")
				}
				sendPairedFrame(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}})
				if req := receive(t, peer.arrived, "the heartbeat ahead of the held report"); req.GetHeartbeat() == nil ||
					receivePairedFrame(t, stream).GetHeartbeatAck() == nil {
					t.Fatal("The selective hold delayed the heartbeat.")
				}
				if mode == "DiscardOnCut" {
					leg.cut()
					receive(t, leg.requestsDone, "the old request loop to stop")
					close(release)
					next := openPairedStream(t, client)
					sendPairedFrame(t, next, pairedEvent(pairedFolder(8, "selected-operation", "selected-command")))
					nextLeg := receive(t, r.legs, "the replacement leg")
					if receivePairedFrame(t, next).GetEventAck().GetThrough() != 8 ||
						!slices.Equal(nextLeg.observe().sends, []frameSend{{kind: "event", number: 8}, {kind: "ack", number: 8}}) ||
						slices.Contains(leg.observe().sends, frameSend{kind: "event", number: 8}) {
						t.Fatal("The retired hold reached the upstream server or blocked fresh replay.")
					}
					return
				}
				close(release)
				for _, number := range []uint64{8, 9} {
					if req := receive(t, peer.arrived, "the resumed numbered suffix"); req.GetEvent().GetNumber() != number ||
						receivePairedFrame(t, stream).GetEventAck().GetThrough() != number {
						t.Fatal("The released reports did not cross in event-number order.")
					}
				}
			})
		}
	})
}
