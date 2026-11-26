package link

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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
	errCommandWhileWaiting = errors.New("link: received work while waiting for approval")
)

// serverClock bounds server time using the elapsed time since proof send.
type serverClock struct {
	serverTime time.Time
	proofTime  time.Duration
	valid      bool
	elapsed    func() (time.Duration, error)
}

func (c serverClock) upperBound() time.Time {
	if !c.valid {
		return time.Time{}
	}
	now, err := c.elapsed()
	if err != nil || now < c.proofTime {
		return time.Time{}
	}
	return c.serverTime.Add(now - c.proofTime)
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

// commandHandler belongs to the owner of the stream and scan slot. The scan
// worker reports completion to the owner, which writes receipts and sends replies.
type commandHandler struct {
	store   *Store
	scan    func(context.Context, bool) (bool, error)
	send    func(context.Context, *session, *linkv1.CommandResult) error
	elapsed func() (time.Duration, error)
	logger  *slog.Logger

	current   *session
	state     *linkv1.MachineState
	clock     serverClock
	active    *activeScan
	completed chan scanCompletion
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
		completed: make(chan scanCompletion, 1),
	}
}

func (h *commandHandler) setSession(s *session) {
	h.current = s
	h.clock = serverClock{}
	if s == nil {
		return
	}
	h.applyState(s.welcome.GetState())
	if sample := s.welcome.GetServerTime(); s.sentAtOK && s.sentAt >= 0 && sample.IsValid() {
		h.clock = serverClock{
			serverTime: sample.AsTime(), proofTime: s.sentAt, valid: true, elapsed: h.elapsed,
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

// handle uses the process context so scans can finish across stream changes.
func (h *commandHandler) handle(ctx context.Context, msg *linkv1.Command) (restart bool, err error) {
	id, expiry := msg.GetId(), msg.GetExpiresAt()
	if !validCommandID(id) || id[14] != '7' || !strings.ContainsRune("89ab", rune(id[19])) ||
		msg.GetAttempt() == 0 || !expiry.IsValid() {
		return false, ErrInvalidCommand
	}
	if h.current == nil {
		return false, errStream
	}
	attempt := commandAttempt{id: id, number: msg.GetAttempt(), session: h.current}
	if _, isSetState := msg.GetAction().(*linkv1.Command_SetState); isSetState {
		state := msg.GetSetState().GetState()
		if state.GetRevision() == 0 || !utf8.ValidString(state.GetName()) || strings.ContainsRune(state.GetName(), 0) {
			return false, ErrInvalidCommand
		}
		h.applyState(state)
		return false, h.reply(ctx, attempt, outcomeDone)
	}
	if h.state.GetApproval() != linkv1.Approval_APPROVAL_APPROVED {
		return false, errCommandWhileWaiting
	}

	_, isScan := msg.GetAction().(*linkv1.Command_ScanHarnesses)
	_, isRestart := msg.GetAction().(*linkv1.Command_Restart)
	cmd := Command{ID: id, ExpiresAt: expiry.AsTime()}
	clock := h.clock
	admission := clock.upperBound()
	if admission.IsZero() {
		admission = cmd.ExpiresAt
	}

	lookupTime := admission
	var reserved *activeScan
	if isScan {
		if h.active == nil {
			reserved = &activeScan{attempt: attempt}
			h.active = reserved
			defer func() {
				if reserved.cancel == nil {
					h.active = nil
				}
			}()
		} else {
			// At expiry, Begin can find a receipt without starting another scan.
			lookupTime = cmd.ExpiresAt
		}
	}
	decision, saved, err := h.store.Begin(cmd, lookupTime)
	if err != nil {
		return false, err
	}
	if decision == DecisionExpired {
		if isScan && reserved == nil && admission.Before(cmd.ExpiresAt) {
			return false, errScanBusy
		}
		decision, saved, err = h.store.Decline(cmd, []byte{byte(outcomeExpired)})
		if err != nil {
			return false, err
		}
	}

	switch decision {
	case DecisionReplay:
		return false, h.reply(ctx, attempt, savedOutcome(saved))
	case DecisionInterrupted:
		return false, h.reply(ctx, attempt, outcomeInterrupted)
	case DecisionExpired:
		return false, h.reply(ctx, attempt, outcomeExpired)
	case DecisionRunning:
		if scan := h.active; scan != nil && scan != reserved &&
			scan.attempt.id == id && attempt.number >= scan.attempt.number {
			scan.attempt = attempt
		}
		return false, nil
	case DecisionRun:
		if reserved != nil {
			h.startScan(ctx, reserved, clock, admission, cmd.ExpiresAt)
			return false, nil
		}
		if !clock.canStart(admission, cmd.ExpiresAt) {
			return false, h.finishCommand(ctx, attempt, outcomeExpired)
		}
		if isRestart {
			return h.restart(ctx, attempt)
		}
		return false, h.finishCommand(ctx, attempt, outcomeUnsupported)
	default:
		return false, ErrStoreDamaged
	}
}

func savedOutcome(saved []byte) linkv1.CommandOutcome {
	if len(saved) == 1 && saved[0] >= byte(outcomeDone) && saved[0] <= byte(outcomeInterrupted) {
		return linkv1.CommandOutcome(saved[0])
	}
	return outcomeInterrupted
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
	attempt, err := h.settleScan(completion)
	if err != nil {
		return err
	}
	return h.reply(ctx, attempt, completion.outcome)
}

func (h *commandHandler) settleScan(completion scanCompletion) (commandAttempt, error) {
	if h.active == nil || completion.scan != h.active {
		return commandAttempt{}, nil
	}
	h.active.cancel()
	attempt := h.active.attempt
	h.active = nil
	if completion.err != nil {
		return commandAttempt{}, completion.err
	}
	return attempt, h.store.Finish(attempt.id, []byte{byte(completion.outcome)})
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
	completion := h.stopScan()
	scanAttempt, err := h.settleScan(completion)
	if err != nil {
		return false, err
	}
	if err := h.store.Finish(attempt.id, []byte{byte(outcomeDone)}); err != nil {
		return false, err
	}

	budget, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = h.reply(budget, scanAttempt, completion.outcome)
	_ = h.reply(budget, attempt, outcomeDone)
	return true, nil
}

func (h *commandHandler) finishCommand(ctx context.Context, attempt commandAttempt, outcome linkv1.CommandOutcome) error {
	if err := h.store.Finish(attempt.id, []byte{byte(outcome)}); err != nil {
		return err
	}
	return h.reply(ctx, attempt, outcome)
}

func (h *commandHandler) reply(ctx context.Context, attempt commandAttempt, outcome linkv1.CommandOutcome) error {
	if attempt.session == nil || attempt.session != h.current {
		return nil
	}
	return h.send(ctx, attempt.session, &linkv1.CommandResult{
		CommandId: attempt.id, Attempt: attempt.number, Outcome: outcome,
	})
}
