package linktest

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

type joinReceipt struct {
	id        string
	keyDigest [sha256.Size]byte
}

type relayControls struct {
	beforeReply    func(context.Context, *linkv1.OpenResponse) bool
	releaseReports func() <-chan struct{}
	holdReport     func(reportDelivery) <-chan struct{}
	holdEvent      func(uint64) <-chan struct{}
	dropResult     func(noticeResult) bool
	observeResult  func(*linkv1.CommandResult)
}

type relay struct {
	controls relayControls
	upstream linkv1connect.LinkServiceClient
	address  string
	dropJoin atomic.Bool
	blockNew atomic.Bool
	joins    chan joinReceipt
	legs     chan *relayLeg
	mu       sync.Mutex
	closing  bool
	workers  sync.WaitGroup
}

func startRelay(t *testing.T, upstream linkv1connect.LinkServiceClient) *relay {
	t.Helper()
	return startRelayWithControls(t, upstream, relayControls{})
}

func startRelayWithControls(t *testing.T, upstream linkv1connect.LinkServiceClient, controls relayControls) *relay {
	t.Helper()
	r := &relay{controls: controls, upstream: upstream, joins: make(chan joinReceipt, 8), legs: make(chan *relayLeg, 16)}
	_, handler := linkv1connect.NewLinkServiceHandler(r)
	listener := httptest.NewUnstartedServer(handler)
	listener.Config.Protocols = new(http.Protocols)
	listener.Config.Protocols.SetUnencryptedHTTP2(true)
	listener.Start()
	t.Cleanup(func() {
		r.mu.Lock()
		r.closing = true
		r.mu.Unlock()
		listener.Close()
		r.workers.Wait()
	})
	r.address = listener.URL
	return r
}

func (r *relay) Join(ctx context.Context, call *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	reply, err := r.upstream.Join(ctx, connect.NewRequest(call.Msg))
	if err != nil {
		return nil, err
	}
	select {
	case r.joins <- joinReceipt{reply.Msg.GetMachineId(), sha256.Sum256(call.Msg.GetMachinePublicKey())}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if r.dropJoin.Swap(false) {
		panic(http.ErrAbortHandler)
	}
	return connect.NewResponse(reply.Msg), nil
}

type legObservation struct {
	hello           helloObservation
	challenges      int
	proofs          int
	welcomed        bool
	mark            uint64
	revision        uint64
	name            string
	approval        linkv1.Approval
	events          []uint64
	acks            []uint64
	heartbeats      int
	heartbeatAcks   int
	emptyHeartbeats int
	results         []noticeResult
	unknownMachine  bool
	commands        []commandDelivery
	reports         []reportDelivery
	sends           []frameSend
}

type helloObservation struct {
	machineID, runID string
	revision         uint32
	first            uint64
}

// frameSend records the start of a relay send. A successful send or a
// subsequent acknowledgement is separately observed by the caller.
type frameSend struct {
	kind      string
	number    uint64
	commandID string
	attempt   uint32
}

type commandDelivery struct {
	id          string
	attempt     uint32
	expiry      time.Time
	kind        string
	revision    uint64
	operationID string
}

type reportDelivery struct {
	facts     bool
	harnesses bool
	folder    folderReport
}

type folderReport struct {
	number                 uint64
	operationID, commandID string
}

type commandAttempt struct {
	id      string
	attempt uint32
}

type noticeResult struct {
	id              string
	attempt         uint32
	outcome         linkv1.CommandOutcome
	lastEventNumber uint64
}

type relayLeg struct {
	cut          context.CancelFunc
	done         chan struct{}
	requestsDone chan struct{}
	inject       chan *linkv1.Command

	mu       sync.Mutex
	trace    legObservation
	injected map[commandAttempt]struct{}
}

func (l *relayLeg) observe() legObservation {
	l.mu.Lock()
	defer l.mu.Unlock()
	trace := l.trace
	trace.events = append([]uint64(nil), trace.events...)
	trace.acks = append([]uint64(nil), trace.acks...)
	trace.results = append([]noticeResult(nil), trace.results...)
	trace.commands = append([]commandDelivery(nil), trace.commands...)
	trace.reports = append([]reportDelivery(nil), trace.reports...)
	trace.sends = append([]frameSend(nil), trace.sends...)
	return trace
}

func (r *relay) Open(ctx context.Context, downstream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	disconnected := func() error {
		return connect.NewError(connect.CodeUnavailable, errors.New("The test link was disconnected."))
	}
	if r.blockNew.Load() {
		return disconnected()
	}
	r.mu.Lock()
	if r.closing {
		r.mu.Unlock()
		return disconnected()
	}
	r.workers.Add(1)
	r.mu.Unlock()
	defer r.workers.Done()
	ctx, cancel := context.WithCancel(ctx)
	leg := &relayLeg{
		cut: cancel, done: make(chan struct{}), requestsDone: make(chan struct{}),
		inject: make(chan *linkv1.Command, 1), injected: make(map[commandAttempt]struct{}),
	}
	defer close(leg.done)
	select {
	case r.legs <- leg:
	case <-ctx.Done():
		cancel()
		return disconnected()
	}
	upstream := r.upstream.Open(ctx)
	// Return the downstream status before joining the upstream reader. The
	// client may still need that status to close its request side.
	defer cancel()
	requests := make(chan *linkv1.OpenRequest)
	r.workers.Go(func() {
		defer close(requests)
		for {
			frame, err := downstream.Receive()
			if err != nil {
				return
			}
			select {
			case requests <- frame:
			case <-ctx.Done():
				return
			}
		}
	})
	r.workers.Go(func() { leg.forwardRequests(ctx, r.controls, requests, upstream) })

	type reply struct {
		frame *linkv1.OpenResponse
		err   error
	}
	replies := make(chan reply)
	r.workers.Go(func() {
		defer upstream.CloseResponse()
		for ctx.Err() == nil {
			frame, err := upstream.Receive()
			select {
			case replies <- reply{frame, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	})

	for {
		select {
		case <-ctx.Done():
			return disconnected()
		case notice := <-leg.inject:
			leg.mu.Lock()
			leg.injected[commandAttempt{notice.GetId(), notice.GetAttempt()}] = struct{}{}
			leg.mu.Unlock()
			later := &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: notice}}
			leg.recordReplySend(later)
			if err := downstream.Send(later); err != nil {
				return err
			}
		case next := <-replies:
			if next.err != nil {
				if errors.Is(next.err, io.EOF) {
					return nil
				}
				const refusal = "Lgtm-Link-Refusal"
				reasons := upstream.ResponseTrailer().Values(refusal)
				for _, reason := range reasons {
					downstream.ResponseTrailer().Add(refusal, reason)
				}
				if connect.CodeOf(next.err) == connect.CodeFailedPrecondition &&
					len(reasons) == 1 && reasons[0] == "unknown_machine" {
					leg.mu.Lock()
					leg.trace.unknownMachine = true
					leg.mu.Unlock()
				}
				return next.err
			}
			if r.controls.beforeReply != nil && !r.controls.beforeReply(ctx, next.frame) {
				return disconnected()
			}
			leg.recordReplySend(next.frame)
			if err := downstream.Send(next.frame); err != nil {
				return err
			}
			leg.recordReply(next.frame)
		}
	}
}

func (l *relayLeg) forwardRequests(
	ctx context.Context, controls relayControls,
	requests <-chan *linkv1.OpenRequest,
	upstream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse],
) {
	defer close(l.requestsDone)
	defer upstream.CloseRequest()
	var release <-chan struct{}
	var held []*linkv1.OpenRequest
	for {
		select {
		case <-ctx.Done():
			return
		case <-release:
			for _, frame := range held {
				if !l.sendRequest(ctx, upstream, frame) {
					return
				}
			}
			release, held = nil, nil
		case frame, ok := <-requests:
			if !ok {
				return
			}
			report, result, injected := l.recordRequest(frame)
			if raw := frame.GetCommandResult(); raw != nil {
				if controls.observeResult != nil {
					controls.observeResult(raw)
				}
				if injected || (controls.dropResult != nil && controls.dropResult(result)) {
					continue
				}
			}
			if frame.GetEvent() != nil {
				if controls.releaseReports != nil {
					if all := controls.releaseReports(); all != nil {
						select {
						case <-all:
						case <-ctx.Done():
							return
						}
					}
				}
				if release == nil && report.folder.number != 0 && controls.holdReport != nil {
					release = controls.holdReport(report)
				}
				if release == nil && controls.holdEvent != nil {
					release = controls.holdEvent(frame.GetEvent().GetNumber())
				}
				// Keep the numbered suffix in order while results and heartbeats pass.
				if release != nil {
					held = append(held, frame)
					continue
				}
			}
			if !l.sendRequest(ctx, upstream, frame) {
				return
			}
		}
	}
}

func (l *relayLeg) recordRequest(frame *linkv1.OpenRequest) (reportDelivery, noticeResult, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var report reportDelivery
	var result noticeResult
	var injected bool
	switch {
	case frame.GetHello() != nil:
		hello := frame.GetHello()
		l.trace.hello = helloObservation{hello.GetMachineId(), hello.GetRunId(), hello.GetLinkRevision(), hello.GetFirstEventNumber()}
	case frame.GetProof() != nil:
		l.trace.proofs++
	case frame.GetEvent() != nil:
		event := frame.GetEvent()
		report = reportDelivery{facts: event.GetFacts() != nil, harnesses: event.GetHarnessesFound() != nil}
		if folder := event.GetFolderSeen(); folder != nil {
			report.folder = folderReport{event.GetNumber(), folder.GetOperationId(), folder.GetCommandId()}
		}
		l.trace.events = append(l.trace.events, event.GetNumber())
		l.trace.reports = append(l.trace.reports, report)
	case frame.GetHeartbeat() != nil:
		l.trace.heartbeats++
		beat := frame.GetHeartbeat()
		if beat.CpuPercent == nil && beat.MemoryPercent == nil {
			l.trace.emptyHeartbeats++
		}
	case frame.GetCommandResult() != nil:
		reply := frame.GetCommandResult()
		result = noticeResult{reply.GetCommandId(), reply.GetAttempt(), reply.GetOutcome(), reply.GetLastEventNumber()}
		l.trace.results = append(l.trace.results, result)
		_, injected = l.injected[commandAttempt{result.id, result.attempt}]
	}
	return report, result, injected
}

func (l *relayLeg) sendRequest(ctx context.Context, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], frame *linkv1.OpenRequest) bool {
	if ctx.Err() != nil {
		return false
	}
	switch {
	case frame.GetEvent() != nil:
		l.recordSend(frameSend{kind: "event", number: frame.GetEvent().GetNumber()})
	case frame.GetCommandResult() != nil:
		reply := frame.GetCommandResult()
		l.recordSend(frameSend{kind: "result", number: reply.GetLastEventNumber(), commandID: reply.GetCommandId(), attempt: reply.GetAttempt()})
	}
	return stream.Send(frame) == nil
}

func (l *relayLeg) recordReplySend(frame *linkv1.OpenResponse) {
	switch {
	case frame.GetEventAck() != nil:
		l.recordSend(frameSend{kind: "ack", number: frame.GetEventAck().GetThrough()})
	case frame.GetCommand() != nil:
		command := frame.GetCommand()
		l.recordSend(frameSend{kind: "command", commandID: command.GetId(), attempt: command.GetAttempt()})
	}
}

func (l *relayLeg) recordSend(send frameSend) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.trace.sends = append(l.trace.sends, send)
}

func (l *relayLeg) recordReply(frame *linkv1.OpenResponse) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch {
	case frame.GetChallenge() != nil:
		l.trace.challenges++
	case frame.GetWelcome() != nil:
		welcome := frame.GetWelcome()
		l.trace.welcomed = true
		l.trace.mark = welcome.GetLastEventNumber()
		l.trace.revision = welcome.GetState().GetRevision()
		l.trace.approval = welcome.GetState().GetApproval()
		l.trace.name = welcome.GetState().GetName()
	case frame.GetEventAck() != nil:
		l.trace.acks = append(l.trace.acks, frame.GetEventAck().GetThrough())
	case frame.GetHeartbeatAck() != nil:
		l.trace.heartbeatAcks++
	case frame.GetCommand() != nil:
		cmd := frame.GetCommand()
		delivery := commandDelivery{id: cmd.GetId(), attempt: cmd.GetAttempt(), expiry: cmd.GetExpiresAt().AsTime()}
		switch {
		case cmd.GetSetState() != nil:
			delivery.kind = "set_state"
			delivery.revision = cmd.GetSetState().GetState().GetRevision()
		case cmd.GetScanHarnesses() != nil:
			delivery.kind = "scan_harnesses"
		case cmd.GetRestart() != nil:
			delivery.kind = "restart"
		case cmd.GetLookAtFolder() != nil:
			delivery.kind = "look_at_folder"
			delivery.operationID = cmd.GetLookAtFolder().GetOperationId()
		}
		l.trace.commands = append(l.trace.commands, delivery)
	}
}
