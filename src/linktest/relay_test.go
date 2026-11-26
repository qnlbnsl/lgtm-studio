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

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

type joinReceipt struct {
	id        string
	keyDigest [sha256.Size]byte
}

type relay struct {
	upstream linkv1connect.LinkServiceClient
	address  string
	dropJoin atomic.Bool
	blockNew atomic.Bool
	joins    chan joinReceipt
	legs     chan *relayLeg
}

func startRelay(t *testing.T, upstream linkv1connect.LinkServiceClient) *relay {
	t.Helper()
	r := &relay{upstream: upstream, joins: make(chan joinReceipt, 8), legs: make(chan *relayLeg, 16)}
	_, handler := linkv1connect.NewLinkServiceHandler(r)
	listener := httptest.NewUnstartedServer(handler)
	listener.Config.Protocols = new(http.Protocols)
	listener.Config.Protocols.SetUnencryptedHTTP2(true)
	listener.Start()
	t.Cleanup(listener.Close)
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
	challenges      int
	proofs          int
	welcomed        bool
	mark            uint64
	revision        uint64
	approval        linkv1.Approval
	events          []uint64
	acks            []uint64
	heartbeats      int
	emptyHeartbeats int
	results         []noticeResult
	unknownMachine  bool
}

type noticeResult struct {
	id      string
	attempt uint32
	outcome linkv1.CommandOutcome
}

type relayLeg struct {
	cut    context.CancelFunc
	done   chan struct{}
	inject chan *linkv1.Command

	mu    sync.Mutex
	trace legObservation
}

func (l *relayLeg) observe() legObservation {
	l.mu.Lock()
	defer l.mu.Unlock()
	trace := l.trace
	trace.events = append([]uint64(nil), trace.events...)
	trace.acks = append([]uint64(nil), trace.acks...)
	trace.results = append([]noticeResult(nil), trace.results...)
	return trace
}

func (r *relay) Open(ctx context.Context, downstream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	disconnected := func() error {
		return connect.NewError(connect.CodeUnavailable, errors.New("The test link was disconnected."))
	}
	if r.blockNew.Load() {
		return disconnected()
	}
	ctx, cancel := context.WithCancel(ctx)
	leg := &relayLeg{cut: cancel, done: make(chan struct{}), inject: make(chan *linkv1.Command, 1)}
	defer close(leg.done)
	select {
	case r.legs <- leg:
	case <-ctx.Done():
		cancel()
		return disconnected()
	}
	upstream := r.upstream.Open(ctx)
	go leg.forwardRequests(downstream, upstream)

	type reply struct {
		frame *linkv1.OpenResponse
		err   error
	}
	replies := make(chan reply)
	readerDone := make(chan struct{})
	defer func() {
		cancel()
		<-readerDone
		_ = upstream.CloseResponse()
	}()
	go func() {
		defer close(readerDone)
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
	}()

	for {
		select {
		case <-ctx.Done():
			return disconnected()
		case notice := <-leg.inject:
			if err := downstream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: notice}}); err != nil {
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
			if err := downstream.Send(next.frame); err != nil {
				return err
			}
			leg.recordReply(next.frame)
		}
	}
}

func (l *relayLeg) forwardRequests(
	downstream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse],
	upstream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse],
) {
	defer upstream.CloseRequest()
	for {
		frame, err := downstream.Receive()
		if err != nil {
			return
		}
		l.mu.Lock()
		switch {
		case frame.GetProof() != nil:
			l.trace.proofs++
		case frame.GetEvent() != nil:
			l.trace.events = append(l.trace.events, frame.GetEvent().GetNumber())
		case frame.GetHeartbeat() != nil:
			l.trace.heartbeats++
			beat := frame.GetHeartbeat()
			if beat.CpuPercent == nil && beat.MemoryPercent == nil {
				l.trace.emptyHeartbeats++
			}
		case frame.GetCommandResult() != nil:
			reply := frame.GetCommandResult()
			l.trace.results = append(l.trace.results, noticeResult{reply.GetCommandId(), reply.GetAttempt(), reply.GetOutcome()})
			l.mu.Unlock()
			// Only the relay sends notices. The application has no stored command for them.
			continue
		}
		l.mu.Unlock()
		if upstream.Send(frame) != nil {
			return
		}
	}
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
	case frame.GetEventAck() != nil:
		l.trace.acks = append(l.trace.acks, frame.GetEventAck().GetThrough())
	}
}
