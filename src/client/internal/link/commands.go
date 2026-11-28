package link

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

const (
	outcomeDone        = linkv1.CommandOutcome_COMMAND_OUTCOME_DONE
	outcomeFailed      = linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED
	outcomeExpired     = linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED
	outcomeUnsupported = linkv1.CommandOutcome_COMMAND_OUTCOME_UNSUPPORTED
	outcomeInterrupted = linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED
)

var (
	errScanBusy            = errors.New("link: another command scan is running")
	errFolderBusy          = errors.New("link: another folder command is running")
	errCommandWhileWaiting = errors.New("link: received work while waiting for approval")
)

// serverClock bounds server time from samples around the verified exchange.
type serverClock struct {
	serverTime   time.Time
	proofTime    time.Duration
	proofOK      bool
	receivedTime time.Duration
	receivedOK   bool
	elapsed      func() (time.Duration, error)
}

func (c serverClock) upperBound() time.Time {
	if !c.proofOK {
		return time.Time{}
	}
	return c.advance(c.proofTime)
}

func (c serverClock) lowerBound() time.Time {
	if !c.receivedOK {
		return time.Time{}
	}
	return c.advance(c.receivedTime)
}

func (c serverClock) advance(sample time.Duration) time.Time {
	now, err := c.elapsed()
	if err != nil || now < sample {
		return time.Time{}
	}
	return c.serverTime.Add(now - sample)
}

func (c serverClock) canStart(admission, expiry time.Time) bool {
	now := c.upperBound()
	return !now.IsZero() && !now.Before(admission) && now.Before(expiry)
}

type commandAttempt struct {
	id      string
	number  uint32
	session *session
}

type activeScan struct {
	attempt commandAttempt
	cancel  context.CancelFunc
}

type scanCompletion struct {
	scan    *activeScan
	outcome linkv1.CommandOutcome
	err     error
}

// commandHandler belongs to the link owner, which settles commands and sends replies.
type commandHandler struct {
	store   *Store
	scan    func(context.Context, bool) (bool, error)
	send    func(context.Context, *session, *linkv1.CommandResult) error
	elapsed func() (time.Duration, error)
	logger  *slog.Logger
	filter  *OutgoingFilter

	current   *session
	state     *linkv1.MachineState
	clock     serverClock
	active    *activeScan
	completed chan scanCompletion

	folderHome      string
	folder          *activeFolder
	folderReads     chan struct{}
	folderScreens   chan struct{}
	folderCompleted chan folderInspection
}

func newCommandHandler(store *Store, scan func(context.Context, bool) (bool, error), send func(context.Context, *session, *linkv1.CommandResult) error, elapsed func() (time.Duration, error), logger *slog.Logger) *commandHandler {
	if elapsed == nil {
		elapsed = elapsedClock
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &commandHandler{
		store: store, scan: scan, send: send, elapsed: elapsed, logger: logger,
		completed:   make(chan scanCompletion, 1),
		folderReads: make(chan struct{}, 2), folderScreens: make(chan struct{}, 1),
		folderCompleted: make(chan folderInspection, 1),
	}
}

func (h *commandHandler) setSession(s *session) {
	h.current = s
	h.clock = serverClock{}
	if s == nil {
		return
	}
	h.applyState(s.welcome.GetState())
	if sample := s.welcome.GetServerTime(); sample.IsValid() {
		h.clock = serverClock{
			serverTime:   sample.AsTime(),
			proofTime:    s.sentAt,
			proofOK:      s.sentAtOK && s.sentAt >= 0,
			receivedTime: s.receivedAt,
			receivedOK:   s.receivedAtOK && s.receivedAt >= 0,
			elapsed:      h.elapsed,
		}
	}
}

func (h *commandHandler) applyState(state *linkv1.MachineState) {
	switch {
	case state.GetRevision() > h.state.GetRevision():
		h.state = proto.CloneOf(state)
	case state.GetRevision() == h.state.GetRevision() && !proto.Equal(state, h.state):
		h.logger.Warn("The server sent conflicting machine states at the same revision.")
	}
}

func (h *commandHandler) standing() *linkv1.MachineState {
	state := proto.CloneOf(h.state)
	if state != nil && state.Approval != linkv1.Approval_APPROVAL_APPROVED {
		state.Approval = linkv1.Approval_APPROVAL_WAITING
		state.Name = ""
	}
	return state
}

// handle uses the process context so work can finish across stream changes.
func (h *commandHandler) handle(ctx context.Context, msg *linkv1.Command) (restart bool, err error) {
	id, expiry := msg.GetId(), msg.GetExpiresAt()
	if !validLinkCommandID(id) || msg.GetAttempt() == 0 || !expiry.IsValid() || proto.Size(msg) > eventWireLimit {
		return false, ErrInvalidCommand
	}
	if h.current == nil {
		return false, errStream
	}
	release, err := h.filter.lockSecrets(ctx, peerIdentifiers)
	if err != nil {
		return false, ErrInvalidCommand
	}
	unlock := sync.OnceFunc(release)
	defer unlock()
	if h.filter.containsSecret(id) {
		return false, ErrInvalidCommand
	}
	folderAction, isFolder := msg.GetAction().(*linkv1.Command_LookAtFolder)
	if isFolder {
		if folderAction == nil {
			return false, ErrInvalidCommand
		}
		operation := folderAction.LookAtFolder.GetOperationId()
		if !validLinkCommandID(operation) || h.filter.containsSecret(operation) {
			return false, ErrInvalidCommand
		}
	}
	attempt := commandAttempt{id: id, number: msg.GetAttempt(), session: h.current}
	cmd := Command{ID: id, ExpiresAt: expiry.AsTime()}
	if _, isSetState := msg.GetAction().(*linkv1.Command_SetState); isSetState {
		state := msg.GetSetState().GetState()
		if state.GetRevision() == 0 || !utf8.ValidString(state.GetName()) || strings.ContainsRune(state.GetName(), 0) {
			return false, ErrInvalidCommand
		}
		_, result, err := h.store.recordResult(cmd, []byte{byte(outcomeDone)}, true, h.clock.lowerBound())
		unlock()
		if err != nil {
			return false, err
		}
		if result.outcome == outcomeDone {
			h.applyState(state)
		}
		return false, h.reply(ctx, attempt, result)
	}
	if h.state.GetApproval() != linkv1.Approval_APPROVAL_APPROVED {
		return false, errCommandWhileWaiting
	}

	_, isScan := msg.GetAction().(*linkv1.Command_ScanHarnesses)
	_, isRestart := msg.GetAction().(*linkv1.Command_Restart)
	clock := h.clock
	admission := clock.upperBound()
	if admission.IsZero() {
		admission = cmd.ExpiresAt
	}

	lookupTime := admission
	scanBusy := isScan && h.active != nil
	folderBusy := isFolder && h.folder != nil
	if scanBusy || folderBusy {
		// At expiry, begin can find a receipt without reserving another worker.
		lookupTime = cmd.ExpiresAt
	}
	decision, saved, err := h.store.begin(cmd, lookupTime)
	if err != nil {
		return false, err
	}
	if decision == DecisionExpired {
		if folderBusy && admission.Before(cmd.ExpiresAt) {
			return false, errFolderBusy
		}
		if scanBusy && admission.Before(cmd.ExpiresAt) {
			return false, errScanBusy
		}
		decision, saved, err = h.store.recordResult(cmd, []byte{byte(outcomeExpired)}, false, clock.lowerBound())
		if err != nil {
			return false, err
		}
		if decision == 0 {
			return false, h.reply(ctx, attempt, saved)
		}
	}

	switch decision {
	case DecisionReplay, DecisionInterrupted:
		return false, h.reply(ctx, attempt, saved)
	case DecisionRunning:
		if scan := h.active; scan != nil && scan.attempt.id == id && attempt.number >= scan.attempt.number {
			scan.attempt = attempt
		}
		if folder := h.folder; folder != nil && folder.attempt.id == id && attempt.number >= folder.attempt.number {
			folder.attempt = attempt
		}
		return false, nil
	case DecisionRun:
		if isScan {
			h.active = &activeScan{attempt: attempt}
			h.startScan(ctx, h.active, clock, admission, cmd.ExpiresAt)
			return false, nil
		}
		if isFolder {
			unlock()
			h.startFolder(ctx, attempt, folderAction.LookAtFolder, clock, admission, cmd.ExpiresAt)
			return false, nil
		}
		if !clock.canStart(admission, cmd.ExpiresAt) {
			return false, h.finishCommand(ctx, attempt, outcomeExpired)
		}
		if isRestart {
			// A scan being joined can still need the filter at its report gate.
			unlock()
			return h.restart(ctx, attempt)
		}
		return false, h.finishCommand(ctx, attempt, outcomeUnsupported)
	default:
		return false, ErrStoreDamaged
	}
}

func validLinkCommandID(id string) bool {
	return validCommandID(id) && id[14] == '7' && strings.ContainsRune("89ab", rune(id[19]))
}

func (h *commandHandler) startScan(ctx context.Context, scan *activeScan, clock serverClock, admission, expiry time.Time) {
	ctx, scan.cancel = context.WithCancel(ctx)
	go func() {
		completion := scanCompletion{scan: scan, outcome: outcomeFailed}
		if ctx.Err() == nil {
			if clock.canStart(admission, expiry) {
				committed, err := h.scan(ctx, true)
				if err != nil && !errors.Is(err, context.Canceled) {
					completion.err = err
				} else if committed {
					completion.outcome = outcomeDone
				}
			} else {
				completion.outcome = outcomeExpired
			}
		}
		h.completed <- completion
	}()
}

func (h *commandHandler) completions() <-chan scanCompletion { return h.completed }

func (h *commandHandler) finishScan(ctx context.Context, completion scanCompletion) error {
	attempt, result, err := h.settleScan(completion)
	if err != nil {
		return err
	}
	return h.reply(ctx, attempt, result)
}

func (h *commandHandler) settleScan(completion scanCompletion) (commandAttempt, savedResult, error) {
	if h.active == nil || completion.scan != h.active {
		return commandAttempt{}, savedResult{}, nil
	}
	h.active.cancel()
	attempt := h.active.attempt
	h.active = nil
	if completion.err != nil {
		return commandAttempt{}, savedResult{}, completion.err
	}
	result, err := h.store.finish(attempt.id, []byte{byte(completion.outcome)}, h.clock.lowerBound())
	return attempt, result, err
}

// stopScan joins an active worker and leaves its result for the owner to settle.
func (h *commandHandler) stopScan() scanCompletion {
	if h.active == nil {
		return scanCompletion{}
	}
	h.active.cancel()
	return <-h.completed
}

func (h *commandHandler) restart(ctx context.Context, attempt commandAttempt) (bool, error) {
	h.stopFolder()
	completion := h.stopScan()
	scanAttempt, scanResult, err := h.settleScan(completion)
	if err != nil {
		return false, err
	}
	result, err := h.store.finish(attempt.id, []byte{byte(outcomeDone)}, h.clock.lowerBound())
	if err != nil {
		return false, err
	}

	budget, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = h.reply(budget, scanAttempt, scanResult)
	_ = h.reply(budget, attempt, result)
	return true, nil
}

func (h *commandHandler) finishCommand(ctx context.Context, attempt commandAttempt, outcome linkv1.CommandOutcome) error {
	result, err := h.store.finish(attempt.id, []byte{byte(outcome)}, h.clock.lowerBound())
	if err != nil {
		return err
	}
	return h.reply(ctx, attempt, result)
}

func (h *commandHandler) reply(ctx context.Context, attempt commandAttempt, result savedResult) error {
	if attempt.session == nil || attempt.session != h.current {
		return nil
	}
	if result.outcome == linkv1.CommandOutcome_COMMAND_OUTCOME_UNSPECIFIED {
		return ErrStoreDamaged
	}
	return h.send(ctx, attempt.session, &linkv1.CommandResult{
		CommandId: attempt.id, Attempt: attempt.number, Outcome: result.outcome,
		LastEventNumber: result.lastEventNumber,
	})
}
