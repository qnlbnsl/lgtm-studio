package link

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

const (
	HandshakeTimeout     = 10 * time.Second
	LinkRefusalKey       = "Lgtm-Link-Refusal"
	LinkRevision         = 1
	lookupTimeout        = 15 * time.Second
	openQueueSize        = 16
	terminalWriteTimeout = 2 * time.Second
)

var (
	errHandshakeDeadline = errors.New("The machine did not answer in time.")
	errLinkRevision      = errors.New("The server speaks revision 1 of the link.")
	errMachineSignature  = errors.New("The machine's signature did not verify.")
	errLinkStopping      = errors.New("The server is stopping.")
	errLinkFinished      = errors.New("The link stream finished.")
)

type receivedFrame struct {
	request     *linkv1.OpenRequest
	err         error
	at          time.Time
	interrupted bool
}

type handshakeReceive struct {
	result      chan receivedFrame
	closingRead atomic.Bool
}

func ServerProofText(machineID string, machineNonce, serverNonce []byte) []byte {
	return proofText("lgtm-link-v1 server", machineID, machineNonce, serverNonce)
}

func MachineProofText(machineID string, machineNonce, serverNonce []byte) []byte {
	return proofText("lgtm-link-v1 machine", machineID, machineNonce, serverNonce)
}

func proofText(label, machineID string, machineNonce, serverNonce []byte) []byte {
	text := make([]byte, 0, len(label)+2+len(machineID)+len(machineNonce)+len(serverNonce))
	text = append(text, label...)
	text = append(text, 0, byte(len(machineID)))
	text = append(text, machineID...)
	text = append(text, machineNonce...)
	return append(text, serverNonce...)
}

type openHandshake struct {
	stream          *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]
	controls        handshakeControls
	steady          streamControls
	watchdog        *streamWatchdog
	phases          chan *handshakeReceive
	frames          chan receivedFrame
	readerDone      chan struct{}
	machineID       string
	handle          *Handle
	cleanup         bool
	publishPresence bool
}

func (s *Service) open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], serve func(context.Context, *Handle, <-chan receivedFrame, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error) error {
	link := s.streaming
	controls := openControls(ctx)
	entered := controls.now()
	h := &openHandshake{
		stream: stream, controls: controls, steady: steadyControls(ctx), phases: make(chan *handshakeReceive, 1),
		frames: make(chan receivedFrame, openQueueSize), readerDone: make(chan struct{}),
	}
	ctx, cancel := s.streamContext(ctx)
	go h.read(ctx)

	err := s.handshake(ctx, cancel, h, entered)
	var watchdogDone chan struct{}
	if err == nil && context.Cause(ctx) == nil {
		h.watchdog = &streamWatchdog{controls: h.steady, deadline: h.steady.now().Add(HeartbeatTimeout)}
		watchdogDone = make(chan struct{})
		go func() {
			defer close(watchdogDone)
			h.watchdog.run(ctx, cancel, func() error { return link.silenceCause(ctx, h.handle) })
		}()
		close(h.phases)
		if h.publishPresence {
			err = link.state.noteWelcome(ctx, h.machineID, h.handle.streamID)
		}
		if err == nil {
			err = serve(ctx, h.handle, h.frames, stream)
		}
	}
	if err == nil {
		err = errLinkFinished
	}
	if h.handle != nil && errors.Is(err, ErrUnknownMachine) {
		// A registry miss can arrive before the admission that displaced this stream is published.
		leave, gateErr := link.hub.enter(ctx, h.handle)
		if gateErr != nil {
			err = gateErr
		} else {
			leave()
		}
	}
	cancel(err)
	cause := context.Cause(ctx)
	if watchdogDone != nil {
		<-watchdogDone
	}
	if h.handle != nil {
		link.hub.Unregister(h.handle)
	}
	if h.cleanup {
		s.queueDisconnect(h.machineID, h.handle.streamID)
		if h.controls.onQueueDisconnect != nil {
			h.controls.onQueueDisconnect(h.machineID, h.handle.streamID)
		}
	}
	_ = transport.CloseLinkRead(ctx)
	<-h.readerDone
	transport.BoundLinkEnd(ctx, terminalWriteTimeout)
	reply := openStatus(cause)
	if h.machineID != "" {
		code := "ok"
		if reply != nil {
			code = connect.CodeOf(reply).String()
		}
		link.options.Logger.Info("link_stream_closed", "machine_id", h.machineID, "code", code)
	}
	return reply
}

func (s *Service) handshake(ctx context.Context, cancel context.CancelCauseFunc, h *openHandshake, entered time.Time) error {
	link := s.streaming
	first, err := h.receivePhase(ctx, entered)
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if !canonicalUUID.MatchString(hello.GetMachineId()) || len(hello.GetMachineNonce()) != 32 {
		return ErrInvalidMessage
	}
	machineID := hello.GetMachineId()
	h.machineID = machineID
	var nonce [32]byte
	rand.Read(nonce[:])
	challenge := &linkv1.Challenge{
		ServerNonce:     nonce[:],
		ServerSignature: ed25519.Sign(link.key, ServerProofText(machineID, hello.GetMachineNonce(), nonce[:])),
	}
	if err := transport.Send(ctx, func() error {
		return h.stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: challenge}})
	}); err != nil {
		return err
	}

	// The proof clock runs while the registry lookup is in progress.
	proofStart := h.controls.now()
	var proof *linkv1.OpenRequest
	var proofErr error
	proofCtx, stopProof := context.WithCancel(ctx)
	proofDone := make(chan struct{})
	go func() {
		defer close(proofDone)
		proof, proofErr = h.receivePhase(proofCtx, proofStart)
	}()
	defer func() {
		stopProof()
		<-proofDone
	}()

	lookupCtx, stopLookup := context.WithTimeout(ctx, lookupTimeout)
	var machine machines.Machine
	err = s.store.Read(lookupCtx, func(ctx context.Context, tx *store.ReadTx) error {
		var loadErr error
		machine, loadErr = link.registry.Get(ctx, tx, machineID)
		return loadErr
	})
	stopLookup()
	switch {
	case errors.Is(err, machines.ErrNoMachine):
		return ErrUnknownMachine
	case err != nil:
		return errDatabaseUnavailable
	case hello.GetLinkRevision() != LinkRevision:
		return errLinkRevision
	}

	<-proofDone
	if proofErr != nil {
		return proofErr
	}
	signature := proof.GetProof().GetMachineSignature()
	if len(signature) != ed25519.SignatureSize {
		return ErrInvalidMessage
	}
	if !validMachinePublicKey(machine.PublicKey) ||
		!h.controls.verify(machine.PublicKey, MachineProofText(machineID, hello.GetMachineNonce(), nonce[:]), signature) {
		return errMachineSignature
	}
	streamID, err := store.NewID()
	if err != nil {
		return errDatabaseUnavailable
	}
	if hook := link.options.Seams.AfterProof; hook != nil {
		hook(ctx, machineID)
	}
	h.handle = link.hub.Track(machineID, streamID, ctx, cancel)
	var admitted Stream
	err = link.hub.Admit(h.handle, func() (int64, bool, error) {
		var admissionErr error
		admitted, admissionErr = link.state.BecomeCurrent(ctx, machineID, streamID, peerAddress(h.stream.Peer().Addr, link.options.PrivateStack))
		var dbErr *store.Error
		unknown := errors.As(admissionErr, &dbErr) && dbErr.Outcome() == store.OutcomeUnknown
		h.cleanup = admissionErr == nil || unknown
		return admitted.Generation, unknown, admissionErr
	})
	if err != nil {
		return err
	}
	if hook := link.options.Seams.AfterAdmission; hook != nil {
		hook(ctx, machineID)
	}
	welcome := &linkv1.Welcome{
		State: machines.Standing(admitted.Standing), LastEventNumber: admitted.Mark,
		ServerTime: timestamppb.New(admitted.ServerTime), ConnectionToken: admitted.ConnectionToken,
		LinkRevision: LinkRevision,
	}
	err = transport.SendLinkWelcome(ctx, func() error { return link.hub.BeginWelcome(h.handle) }, func() error {
		return h.stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: welcome}})
	})
	if err != nil {
		return err
	}
	h.publishPresence = link.hub.finishWelcome(h.handle)
	link.options.Logger.Info("link_stream_opened", "machine_id", machineID, "generation", admitted.Generation)
	return nil
}

func (h *openHandshake) read(ctx context.Context) {
	defer close(h.readerDone)
	defer close(h.frames)
	if h.steady.readerDone != nil {
		defer h.steady.readerDone()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case phase, ok := <-h.phases:
			if ok {
				request, err := h.stream.Receive()
				interrupted := phase.closingRead.Load()
				at := h.controls.now()
				if h.controls.afterReceive != nil {
					h.controls.afterReceive(request, err, at)
				}
				phase.result <- receivedFrame{request: request, err: err, at: at, interrupted: interrupted}
				continue
			}
		}
		break
	}
	for ctx.Err() == nil {
		request, err := h.stream.Receive()
		if hb := request.GetHeartbeat(); err == nil && hb != nil && hb.GetCpuPercent() <= 100 && hb.GetMemoryPercent() <= 100 {
			h.watchdog.refresh()
		}
		if h.steady.afterReceive != nil {
			h.steady.afterReceive(request, err)
		}
		frame := receivedFrame{request: request, err: err}
		select {
		case h.frames <- frame:
		case <-ctx.Done():
			return
		}
		if frame.err != nil {
			return
		}
	}
}

func (h *openHandshake) receivePhase(ctx context.Context, start time.Time) (*linkv1.OpenRequest, error) {
	cutoff := start.Add(HandshakeTimeout)
	timer, stop := h.controls.timer(cutoff.Sub(h.controls.now()))
	defer stop()
	phase := &handshakeReceive{result: make(chan receivedFrame, 1)}
	select {
	case h.phases <- phase:
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}

	var frame receivedFrame
	select {
	case frame = <-phase.result:
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case <-timer:
		select {
		case frame = <-phase.result:
		default:
			phase.closingRead.Store(true)
			_ = transport.CloseLinkRead(ctx)
			select {
			case frame = <-phase.result:
			case <-ctx.Done():
				return nil, context.Cause(ctx)
			}
		}
	}
	if frame.at.After(cutoff) {
		return nil, errHandshakeDeadline
	}
	if frame.err != nil {
		if malformedOpenFrame(frame.err) {
			return nil, ErrInvalidMessage
		}
		if frame.interrupted {
			return nil, errHandshakeDeadline
		}
		return nil, frame.err
	}
	return frame.request, nil
}

func malformedOpenFrame(err error) bool {
	return errors.Is(err, transport.ErrLinkFrameTooLarge) || errors.Is(err, io.ErrUnexpectedEOF) ||
		connect.CodeOf(err) == connect.CodeInvalidArgument || connect.CodeOf(err) == connect.CodeResourceExhausted
}

func openStatus(err error) error {
	switch {
	case err == nil, errors.Is(err, errLinkFinished):
		return nil
	case errors.Is(err, ErrUnknownMachine):
		return linkRefusal(ErrUnknownMachine, "unknown_machine")
	case errors.Is(err, errLinkRevision):
		return linkRefusal(errLinkRevision, "link_revision_mismatch")
	case errors.Is(err, errMachineSignature):
		return connect.NewError(connect.CodeUnauthenticated, errMachineSignature)
	case errors.Is(err, ErrEventGap):
		return linkRefusal(ErrEventGap, "event_gap")
	case errors.Is(err, ErrInvalidMessage), errors.Is(err, ErrEventNumberZero):
		return connect.NewError(connect.CodeInvalidArgument, errInvalidLinkMessage)
	case errors.Is(err, errHandshakeDeadline), errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, errHandshakeDeadline)
	case errors.Is(err, ErrSuperseded), errors.Is(err, ErrNotCurrent):
		return connect.NewError(connect.CodeAborted, ErrSuperseded)
	case errors.Is(err, errLinkStopping):
		return connect.NewError(connect.CodeUnavailable, errLinkStopping)
	case errors.Is(err, context.Canceled), errors.Is(err, io.EOF):
		return connect.NewError(connect.CodeCanceled, context.Canceled)
	default:
		var coded *connect.Error
		if errors.As(err, &coded) && coded.Code() == connect.CodeFailedPrecondition && coded.Meta().Get(LinkRefusalKey) == "unknown_machine" {
			return linkRefusal(ErrUnknownMachine, "unknown_machine")
		}
		switch connect.CodeOf(err) {
		case connect.CodeAborted:
			return connect.NewError(connect.CodeAborted, ErrSuperseded)
		case connect.CodeCanceled:
			return connect.NewError(connect.CodeCanceled, context.Canceled)
		case connect.CodeDeadlineExceeded:
			return connect.NewError(connect.CodeDeadlineExceeded, errHandshakeDeadline)
		case connect.CodeInvalidArgument:
			return connect.NewError(connect.CodeInvalidArgument, errInvalidLinkMessage)
		default:
			return connect.NewError(connect.CodeUnavailable, errDatabaseUnavailable)
		}
	}
}

func linkRefusal(err error, reason string) error {
	refusal := connect.NewError(connect.CodeFailedPrecondition, err)
	refusal.Meta().Set(LinkRefusalKey, reason)
	return refusal
}

type handshakeControlsKey struct{}

type handshakeControls struct {
	now               func() time.Time
	timer             func(time.Duration) (<-chan time.Time, func())
	afterReceive      func(*linkv1.OpenRequest, error, time.Time)
	verify            func(ed25519.PublicKey, []byte, []byte) bool
	onQueueDisconnect func(machineID, streamID string)
}

func openControls(ctx context.Context) handshakeControls {
	var c handshakeControls
	if supplied, _ := ctx.Value(handshakeControlsKey{}).(*handshakeControls); supplied != nil {
		c = *supplied
	}
	if c.now == nil {
		c.now = time.Now
	}
	if c.timer == nil {
		c.timer = func(wait time.Duration) (<-chan time.Time, func()) {
			timer := time.NewTimer(wait)
			return timer.C, func() { timer.Stop() }
		}
	}
	if c.verify == nil {
		c.verify = ed25519.Verify
	}
	return c
}
