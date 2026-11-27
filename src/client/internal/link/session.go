package link

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/transport"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

type RunOptions struct {
	Server      enroll.Server
	Credential  *enroll.Credential
	Folder      *state.Folder
	Settings    config.Settings
	ScanOptions harness.ScanOptions
	Version     string
	Logger      *slog.Logger
	// Filter shares known secrets with the process that owns this link.
	Filter *OutgoingFilter
}

type RunOutcome uint8

const (
	RunStopped RunOutcome = 1
	RunRestart RunOutcome = 2
	// RunRecoveryStop requires the owner to intervene before running again.
	RunRecoveryStop RunOutcome = 3
)

const (
	reportInterval    = 10 * time.Second
	streamSilence     = 30 * time.Second
	restartSendBudget = 2 * time.Second
	maxPendingReplies = 256
	maxPendingFrames  = 64
	// Leave room in the receive queue for heartbeats and commands.
	maxUnackedEvents = maxPendingFrames / 2
)

var (
	errInvalidFrame     = errors.New("link: invalid response frame")
	errSilentStream     = errors.New("link: the server stopped responding")
	errReceiveQueueFull = errors.New("link: too many pending response frames")
)

var newRunLinkClient = func(server enroll.Server) (linkv1connect.LinkServiceClient, func(), error) {
	httpClient, retire, err := transport.NewStreamHTTPClient(server.Address, server.CertificateKeySHA256)
	if err != nil {
		return nil, nil, err
	}
	client := linkv1connect.NewLinkServiceClient(httpClient, server.Address, connect.WithReadMaxBytes(64<<10))
	return client, retire, nil
}

var runReportProbes reportProbes

type runSessionIO struct {
	send    func(*linkv1.OpenRequest) error
	receive func() (*linkv1.OpenResponse, error)
}

var wrapRunSessionIO func(context.Context, *session, runSessionIO) runSessionIO

// Run opens the saved outbox and owns the link until cancellation, a final
// refusal, an accepted restart or a local store fault. The caller owns the folder.
func Run(ctx context.Context, opts RunOptions) (RunOutcome, error) {
	if ctx.Err() != nil {
		return RunStopped, nil
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if opts.Filter == nil {
		opts.Filter = NewOutgoingFilter()
	}
	store, err := OpenStore(opts.Folder, StoreOptions{})
	if err != nil {
		if ctx.Err() != nil {
			return RunStopped, nil
		}
		return stopForStore(logger, err)
	}
	if err := store.checkRunReceipts(); err != nil {
		return stopForStore(logger, errors.Join(err, store.Close()))
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r := clientRun{
		ctx: ctx, cancel: cancel, opts: opts, logger: logger, store: store,
		announcement: newRunIdentity(store.Next()),
	}
	r.reports = newReporterWithEventAppend(opts.Settings, opts.Version, opts.ScanOptions, r.appendEvent, logger, runReportProbes)
	r.commands = newCommandHandler(store, r.reports.scanHarnesses, r.queueResult, nil, logger)
	r.commands.filter = opts.Filter
	linkErr := supervise(ctx, r.attempt)
	revisionStop := isRevisionRefusal(linkErr)
	if revisionStop {
		r.cancel()
	}

	outageErr := r.stopOutage()
	completion := r.commands.stopScan()
	recoveryErr := errors.Join(r.fault, outageErr)
	var scanErr error
	if !revisionStop && recoveryStoreCode(recoveryErr) == "" {
		_, _, scanErr = r.commands.settleScan(completion)
	}
	closeErr := store.Close()
	if revisionStop {
		r.reportFailure(linkFailureCode(linkErr))
		return RunRecoveryStop, nil
	}
	if err := errors.Join(r.fault, outageErr, scanErr, closeErr); err != nil {
		return stopForStore(logger, err)
	}
	if r.restart {
		return RunRestart, nil
	}
	return RunStopped, nil
}

// clientRun owns process-wide state. During an outage the disconnected worker
// may settle the command scan. The next session joins it before using commands.
type clientRun struct {
	ctx      context.Context
	cancel   context.CancelFunc
	opts     RunOptions
	logger   *slog.Logger
	store    *Store
	reports  *reporter
	commands *commandHandler

	announcement runIdentity

	live        *liveSession
	endOutage   func() error
	lastReason  string
	sawApproval bool
	prepared    bool
	restart     bool
	fault       error
}

func (r *clientRun) attempt(ctx context.Context, welcomed func()) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	client, retire, err := newRunLinkClient(r.opts.Server)
	if err != nil {
		if r.ctx.Err() == nil {
			r.reportFailure("transport_setup")
		}
		return errStream
	}
	if retire != nil {
		retire = sync.OnceFunc(retire)
		defer retire()
	}
	s, err := openSessionWithRevisionFence(ctx, client, r.opts.Server, r.opts.Credential, r.announcement, r.store.fenceWrites)
	if err != nil {
		if !isRevisionRefusal(err) && r.ctx.Err() == nil {
			r.reportFailure(linkFailureCode(err))
		}
		return err
	}
	defer s.close()
	r.opts.Filter.AddSecrets(s.welcome.GetConnectionToken())
	welcomed()

	live := &liveSession{run: r}
	r.live = live
	restart, err := live.serve(ctx, s, retire)
	r.commands.setSession(nil)
	r.live = nil

	var local storeFault
	switch {
	case errors.As(err, &local):
		if r.fault == nil {
			r.stopLocal(local.err)
		}
	case isRevisionRefusal(err):
		// The supervisor returns the verified refusal to Run.
	case restart:
		r.restart = true
		r.cancel()
	case r.ctx.Err() == nil:
		r.reportFailure(linkFailureCode(err))
		var refusal terminalRefusal
		if r.sawApproval && !errors.As(err, &refusal) {
			if err := r.startOutage(); err != nil {
				r.stopLocal(err)
			}
		}
	}
	if err != nil {
		return streamEnd{error: err, at: live.ended}
	}
	return err
}

func (r *clientRun) stopLocal(err error) {
	if recoveryStoreCode(err) != "" {
		r.store.fenceWrites()
	}
	r.fault = err
	r.cancel()
}

func (r *clientRun) appendReport(ctx context.Context, payload []byte) (uint64, error) {
	event, err := decodeReportCandidate(payload)
	if err != nil {
		return 0, errInvalidReport
	}
	return r.appendEvent(ctx, event)
}

func (r *clientRun) appendEvent(ctx context.Context, event *linkv1.Event) (uint64, error) {
	number, err := r.store.appendFilteredReport(ctx, r.opts.Filter, event)
	if errors.Is(err, ErrSequenceExhausted) && r.store.Len() != 0 {
		// Keep pending reports queued until the last legal event drains.
		return 0, ErrOutboxFull
	}
	return number, err
}

func (r *clientRun) captureFacts() error {
	err := r.reports.captureFacts()
	if errors.Is(err, errReadFacts) {
		return nil
	}
	return err
}

func (r *clientRun) startOutage() error {
	if err := r.captureFacts(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(r.ctx)
	done := make(chan error, 1)
	ticks := time.NewTicker(reportInterval)
	go func() {
		defer ticks.Stop()
		err := r.watchOutage(ctx, ticks.C)
		if err != nil {
			if recoveryStoreCode(err) != "" {
				r.store.fenceWrites()
			}
			r.cancel()
		}
		done <- err
	}()
	r.endOutage = func() error {
		cancel()
		return <-done
	}
	return nil
}

func (r *clientRun) watchOutage(ctx context.Context, ticks <-chan time.Time) error {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case <-ticks:
			if err := r.captureFacts(); err != nil {
				return err
			}
		case completion := <-r.commands.completions():
			if err := r.commands.finishScan(ctx, completion); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *clientRun) stopOutage() error {
	if r.endOutage == nil {
		return nil
	}
	err := r.endOutage()
	r.endOutage = nil
	return err
}

func (r *clientRun) queueResult(ctx context.Context, _ *session, result *linkv1.CommandResult) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(r.live.results) >= maxPendingReplies {
		return errStream
	}
	r.live.results = append(r.live.results, result)
	return nil
}

func (r *clientRun) reportFailure(code string) {
	if r.lastReason == code {
		return
	}
	r.lastReason = code
	switch code {
	case "unknown_machine":
		r.logger.Warn("link_refused", "code", code, "message", "Enroll this machine again.")
	case "link_revision_mismatch", "invalid_welcome_revision":
		r.logger.Warn("link_refused", "code", code, "message", "Install a compatible client using the saved state.")
	case "server_key_mismatch", "certificate_key_mismatch":
		r.logger.Warn("link_refused", "code", code, "message", "Check the server key and enroll again if it changed.")
	case "aborted":
		r.logger.Warn("link_disconnected", "code", code, "message", "Check for another client process using this enrollment.")
	default:
		r.logger.Warn("link_disconnected", "code", code)
	}
}

func linkFailureCode(err error) string {
	var refusal terminalRefusal
	var status *connect.Error
	switch {
	case errors.As(err, &refusal):
		if refusal.fromWire {
			return refusal.Code
		}
		return "invalid_welcome_revision"
	case errors.Is(err, errServerKeyMismatch):
		return "server_key_mismatch"
	case errors.Is(err, transport.ErrCertificateKeyMismatch):
		return "certificate_key_mismatch"
	case errors.Is(err, errEventGap):
		return "event_gap"
	case errors.Is(err, errSilentStream):
		return "server_silent"
	case errors.Is(err, errReceiveQueueFull):
		return "receive_queue_full"
	case errors.Is(err, errInvalidChallenge), errors.Is(err, errInvalidWelcome), errors.Is(err, errInvalidFrame), errors.Is(err, ErrInvalidCommand):
		return "invalid_frame"
	case errors.Is(err, errInvalidCredentials):
		return "invalid_credentials"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, io.EOF):
		return "stream_closed"
	case errors.As(err, &status):
		return status.Code().String()
	default:
		return "stream_failed"
	}
}

type storeFault struct{ err error }

func (storeFault) Error() string { return "link: client.db operation failed" }

func recoveryStoreCode(err error) string {
	switch {
	case errors.Is(err, ErrAckAhead):
		return "impossible_ack"
	case errors.Is(err, ErrStoreDamaged), errors.Is(err, ErrStoreMissing):
		return "store_failed"
	default:
		return ""
	}
}

func stopForStore(logger *slog.Logger, err error) (RunOutcome, error) {
	if code := recoveryStoreCode(err); code != "" {
		logger.Error("link_stopped", "code", code, "message", "Restore the original client.db history or remove this machine and enroll again with fresh state.")
		return RunRecoveryStop, nil
	}
	if errors.Is(err, ErrSequenceExhausted) {
		logger.Error("link_stopped", "code", "sequence_exhausted", "message", "An owner must decide how to recover this machine's saved events before restarting.")
		return RunStopped, nil
	}

	logger.Error("link_stopped", "code", "store_failed", "message", "Could not use client.db.")
	// Keep the public store errors while dropping underlying paths and details.
	for _, known := range []error{ErrStoreFailed, ErrStoreUnsafe, ErrStoreClosed, state.ErrStoreOpen, state.ErrFolderClosed} {
		if errors.Is(err, known) {
			return RunStopped, fmt.Errorf("%s: %w", state.StoreFile, known)
		}
	}
	return RunStopped, fmt.Errorf("%s: %w", state.StoreFile, ErrStoreFailed)
}

type liveSession struct {
	run     *clientRun
	replay  *replay
	results []*linkv1.CommandResult
	ended   time.Time
}

type streamReply struct {
	frame      *linkv1.OpenResponse
	err        error
	ackCeiling uint64
	receivedAt time.Time
}

func (l *liveSession) approved() bool {
	return l.run.commands.state.GetApproval() == linkv1.Approval_APPROVAL_APPROVED
}

func (l *liveSession) serve(ctx context.Context, s *session, retire func()) (restart bool, err error) {
	ctx, cancel := context.WithCancel(ctx)
	wire := runSessionIO{send: s.send, receive: s.receive}
	if wrapRunSessionIO != nil {
		wire = wrapRunSessionIO(ctx, s, wire)
	}

	var workers sync.WaitGroup
	reportFailed := make(chan error, 1)
	var sendErr error
	var timingMu sync.Mutex
	deadline := s.welcomedAt.Add(streamSilence)
	handedThrough := s.welcome.GetLastEventNumber()
	ackedThrough := handedThrough
	var transportEnded time.Time
	var receiveStop error
	var silence *time.Timer
	timingMu.Lock()
	silence = time.AfterFunc(time.Until(deadline), func() {
		timingMu.Lock()
		defer timingMu.Unlock()
		if ctx.Err() != nil || !transportEnded.IsZero() {
			return
		}
		if remaining := time.Until(deadline); remaining > 0 {
			silence.Reset(remaining)
			return
		}
		transportEnded, receiveStop = deadline, errSilentStream
		cancel()
		s.cancel()
	})
	timingMu.Unlock()

	defer func() {
		var local storeFault
		recovering := errors.As(err, &local) && recoveryStoreCode(local.err) != ""
		if recovering {
			l.run.stopLocal(local.err)
		} else if isRevisionRefusal(err) {
			l.run.store.fenceWrites()
		}
		if l.ended.IsZero() {
			l.ended = time.Now()
		}
		timingMu.Lock()
		if !transportEnded.IsZero() && transportEnded.Before(l.ended) {
			l.ended = transportEnded
		}
		if receiveStop != nil && errors.Is(err, context.Canceled) {
			err = receiveStop
			if receiveStop == errSilentStream && sendErr != nil {
				err = sendErr
			}
		}
		cancel()
		silence.Stop()
		timingMu.Unlock()
		if restart && retire != nil {
			// Close the connection before joining workers because the peer
			// can leave a transport write blocked after request cancellation.
			s.cancel()
			retire()
		}
		_ = s.close()
		workers.Wait()
		s.welcome.ConnectionToken = ""
		select {
		case reportErr := <-reportFailed:
			if !recovering && !isRevisionRefusal(err) {
				err = storeFault{reportErr}
			}
		default:
		}
	}()

	send := make(chan *linkv1.OpenRequest, 1)
	sent := make(chan error, 1)
	replies := make(chan streamReply, maxPendingFrames)
	workers.Go(func() {
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			case frame := <-send:
				err := wire.send(frame)
				sent <- err
				if err != nil {
					return
				}
			}
		}
	})
	workers.Go(func() {
		for ctx.Err() == nil {
			frame, err := wire.receive()
			receivedAt := time.Now()
			// Account for the frame before handing it to the owner, who may
			// be blocked on a local read or write.
			timingMu.Lock()
			if ctx.Err() != nil || !receivedAt.Before(deadline) {
				timingMu.Unlock()
				return
			}
			if err != nil {
				transportEnded = receivedAt
				timingMu.Unlock()
				// Preserve a terminal reply behind the queued frames.
				select {
				case replies <- streamReply{err: err}:
				case <-ctx.Done():
				}
				return
			}
			deadline = receivedAt.Add(streamSilence)
			select {
			case replies <- streamReply{frame: frame, ackCeiling: handedThrough, receivedAt: receivedAt}:
			default:
				transportEnded, receiveStop = receivedAt, errReceiveQueueFull
				cancel()
				s.cancel()
			}
			timingMu.Unlock()
		}
	})

	if err := l.run.stopOutage(); err != nil {
		return false, storeFault{err}
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	cursor, err := newReplayWithFilter(l.run.store, s.welcome.GetLastEventNumber(), l.run.opts.Filter)
	if err != nil {
		return false, storeFault{err}
	}
	if !l.run.prepared {
		if err := l.run.store.prepareRun(); err != nil {
			return false, storeFault{err}
		}
		l.run.prepared = true
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	l.replay = cursor
	if l.run.lastReason != "" {
		l.run.logger.Info("link_recovered")
		l.run.lastReason = ""
	}
	l.run.commands.setSession(s)

	var cancelInventory context.CancelFunc
	freshReports := func() error {
		l.run.sawApproval = true
		if err := l.run.captureFacts(); err != nil {
			return storeFault{err}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		scanCtx, stop := context.WithCancel(ctx)
		cancelInventory = stop
		workers.Go(func() {
			defer stop()
			if _, err := l.run.reports.scanHarnesses(scanCtx, false); err != nil {
				select {
				case reportFailed <- err:
				default:
				}
			}
		})
		return nil
	}

	if l.approved() {
		if err := freshReports(); err != nil {
			return false, err
		}
	}
	if err := l.run.reports.flush(); err != nil {
		return false, storeFault{err}
	}

	started := time.Now()
	nextBeat := started
	heartbeats := time.NewTicker(reportInterval)
	defer heartbeats.Stop()
	ready := make(chan struct{})
	close(ready)
	var restartDeadline <-chan time.Time
	requestClosed := false

	busy, haveEvents := false, true
	canSendEvent := func() bool {
		retained := handedThrough+1 < l.run.announcement.firstEventNumber
		return (l.approved() || retained) && haveEvents && handedThrough-ackedThrough < maxUnackedEvents
	}
	handoffEvent := func() (bool, error) {
		unlock := l.run.opts.Filter.lockSecrets()
		defer unlock()
		event, err := l.replay.next()
		if err != nil || event == nil {
			return false, err
		}
		timingMu.Lock()
		send <- &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}}
		handedThrough = event.GetNumber()
		timingMu.Unlock()
		return true, nil
	}
	handle := func(reply streamReply) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if reply.err != nil {
			return reply.err
		}
		if sendErr != nil || restartDeadline != nil {
			return nil
		}
		// A queued acknowledgement can only cover handoffs made before it arrived.
		ack := reply.frame.GetEventAck()
		if ack != nil && ack.GetThrough() > reply.ackCeiling {
			l.ended = reply.receivedAt
			return storeFault{ErrAckAhead}
		}
		wasApproved := l.approved()
		restart, err := l.receive(reply.frame)
		if err != nil {
			l.ended = reply.receivedAt
			return err
		}
		if ack != nil {
			ackedThrough = max(ackedThrough, ack.GetThrough())
		}
		if restart {
			restartDeadline = time.After(restartSendBudget)
		}
		if !wasApproved && l.approved() {
			return freshReports()
		}
		if wasApproved && !l.approved() && cancelInventory != nil {
			cancelInventory()
			cancelInventory = nil
		}
		return nil
	}
	for ctx.Err() == nil {
		restarting := restartDeadline != nil
		if restarting && !busy && len(l.results) == 0 && !requestClosed {
			// Send hands bytes to the local request pipe. Close the request
			// side and let the peer finish before canceling, within the same
			// restart deadline.
			if err := s.closeRequest(); err != nil {
				return true, nil
			}
			requestClosed = true
		}
		var dispatch <-chan struct{}
		if !busy && sendErr == nil && (len(l.results) != 0 ||
			!restarting && (!time.Now().Before(nextBeat) || canSendEvent())) {
			dispatch = ready
		}
		select {
		case <-ctx.Done():
			return restarting, ctx.Err()
		case <-restartDeadline:
			return true, nil
		case <-heartbeats.C:
		case <-l.run.store.Changed():
			haveEvents = true
		case err := <-reportFailed:
			return false, storeFault{err}
		case completion := <-l.run.commands.completions():
			if err := l.run.commands.finishScan(l.run.ctx, completion); err != nil {
				return false, commandError(err)
			}
		case err := <-sent:
			busy = false
			if err != nil {
				if restarting {
					return true, nil
				}
				// A peer may have ended the RPC with a refusal. Close the
				// request and give the receiver the rest of its silence budget.
				sendErr = err
				_ = s.closeRequest()
			}
		case reply := <-replies:
			if err := handle(reply); err != nil {
				return restarting, err
			}
		case <-dispatch:
			// Give an already received command or refusal a turn before
			// handing another event to the sender.
			select {
			case reply := <-replies:
				if err := handle(reply); err != nil {
					return restarting, err
				}
			default:
			}
			now := time.Now()
			switch {
			case len(l.results) != 0:
				result := l.results[0]
				unlock := l.run.opts.Filter.lockSecrets()
				if l.run.opts.Filter.containsSecret(result.GetCommandId()) {
					unlock()
					return false, errInvalidFrame
				}
				send <- &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: result}}
				unlock()
				l.results[0] = nil
				l.results = l.results[1:]
			case restartDeadline != nil:
				continue
			case !now.Before(nextBeat):
				nextBeat = nextBeat.Add((now.Sub(nextBeat)/reportInterval + 1) * reportInterval)
				send <- &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: l.run.reports.heartbeat(l.approved())}}
			case canSendEvent():
				sent, err := handoffEvent()
				if err != nil {
					return false, storeFault{err}
				}
				if !sent {
					haveEvents = false
					continue
				}
			default:
				continue
			}
			busy = true
		}
	}
	return restartDeadline != nil, ctx.Err()
}

func (l *liveSession) receive(frame *linkv1.OpenResponse) (bool, error) {
	switch msg := frame.GetKind().(type) {
	case *linkv1.OpenResponse_HeartbeatAck:
		if msg.HeartbeatAck != nil {
			return false, nil
		}
	case *linkv1.OpenResponse_EventAck:
		if msg.EventAck != nil {
			if err := l.replay.ack(msg.EventAck.GetThrough()); err != nil {
				return false, storeFault{err}
			}
			if err := l.run.reports.flush(); err != nil {
				return false, storeFault{err}
			}
			return false, nil
		}
	case *linkv1.OpenResponse_Command:
		restart, err := l.run.commands.handle(l.run.ctx, msg.Command)
		return restart, commandError(err)
	case nil:
		if frame != nil && len(frame.ProtoReflect().GetUnknown()) != 0 {
			return false, nil
		}
	}
	return false, errInvalidFrame
}

func commandError(err error) error {
	switch {
	case err == nil, errors.Is(err, ErrInvalidCommand), errors.Is(err, errScanBusy),
		errors.Is(err, errCommandWhileWaiting), errors.Is(err, errStream), errors.Is(err, context.Canceled):
		return err
	default:
		return storeFault{err}
	}
}
