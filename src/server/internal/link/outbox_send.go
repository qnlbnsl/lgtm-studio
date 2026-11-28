package link

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const (
	commandSlotLimit = 3
	commandSendLimit = 2 * time.Second
)

type afterCommandClaimKey struct{}
type commandSessionKey struct{}

type commandSession struct {
	cursor       commandCursor
	writeContext context.Context
	continuation chan struct{}
	handle       *Handle

	// The command slot mutex protects waiting and granted.
	waiting *list.Element
	granted bool
}

type commandSlots struct {
	mu      sync.Mutex
	used    int
	waiters list.List
}

func (slots *commandSlots) acquire(session *commandSession) bool {
	slots.mu.Lock()
	defer slots.mu.Unlock()
	if session.handle.ctx.Err() != nil {
		return false
	}
	if session.granted {
		return true
	}
	if slots.used < commandSlotLimit {
		slots.used++
		session.granted = true
		return true
	}
	if session.waiting == nil {
		session.waiting = slots.waiters.PushBack(session)
	}
	return false
}

func (slots *commandSlots) release(session *commandSession) {
	slots.mu.Lock()
	defer slots.mu.Unlock()
	if session.waiting != nil {
		slots.waiters.Remove(session.waiting)
		session.waiting = nil
	}
	if !session.granted {
		return
	}
	session.granted = false
	for front := slots.waiters.Front(); front != nil; front = slots.waiters.Front() {
		waiter := slots.waiters.Remove(front).(*commandSession)
		waiter.waiting = nil
		if waiter.handle.ctx.Err() != nil {
			continue
		}
		waiter.granted = true
		select {
		case waiter.handle.wake <- struct{}{}:
		default:
		}
		return
	}
	slots.used--
}

// SendNext claims one page and sends at most one command in the granted writer turn.
func (s *State) SendNext(ctx context.Context, machineID, streamID string, send func(*linkv1.Command) error) (bool, error) {
	// The writer attaches the session only to calls that hold a permit.
	session, _ := ctx.Value(commandSessionKey{}).(*commandSession)
	if session == nil {
		return false, nil
	}
	if err := context.Cause(ctx); err != nil {
		return false, err
	}
	command, more, err := s.claimNext(ctx, machineID, streamID, &session.cursor)
	if err != nil {
		return false, commandStoreError(ctx, err)
	}
	var sent bool
	if command != nil {
		if afterClaim, _ := ctx.Value(afterCommandClaimKey{}).(func(context.Context, *linkv1.Command) error); afterClaim != nil {
			if err := afterClaim(ctx, command); err != nil {
				return false, err
			}
		}
		if err := context.Cause(ctx); err != nil {
			return false, err
		}
		sent, err = s.sendClaimed(ctx, machineID, streamID, command, func(writeContext context.Context) error {
			session.writeContext = writeContext
			defer func() { session.writeContext = nil }()
			return send(command)
		})
		if err != nil {
			return false, err
		}
	}
	if more || (command != nil && !sent) {
		select {
		case session.continuation <- struct{}{}:
		default:
		}
	}
	return sent, nil
}

func (s *State) sendClaimed(ctx context.Context, machineID, streamID string, command *linkv1.Command, send func(context.Context) error) (bool, error) {
	budget, cancel := context.WithTimeout(ctx, commandSendLimit)
	defer cancel()

	var sent bool
	var sendErr error
	err := s.store.Write(budget, func(ctx context.Context, tx *store.Tx) error {
		machine, err := s.lockCommandStream(ctx, tx, machineID, streamID)
		if err != nil {
			return err
		}
		ready, err := runReady(ctx, tx, machineID)
		if err != nil || !ready {
			return err
		}
		var action string
		var expiresAt, databaseNow time.Time
		checkedAt := time.Now()
		err = tx.QueryRow(ctx, `
SELECT action, expires_at, clock_timestamp()
FROM link_commands
WHERE id = $1 AND machine_id = $2 AND sent_stream_id = $3 AND attempt = $4
  AND outcome IS NULL AND staged_outcome IS NULL AND NOT dispatched`,
			command.GetId(), machineID, streamID, int64(command.GetAttempt())).Scan(&action, &expiresAt, &databaseNow)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}

		writeContext := ctx
		if action != "set_state" {
			cutoff := checkedAt.Add(expiresAt.Add(commandRetryGrace).Sub(databaseNow))
			if machine.Approval != machines.ApprovalApproved || !time.Now().Before(cutoff) {
				return nil
			}
			var stop context.CancelFunc
			writeContext, stop = context.WithDeadline(ctx, cutoff)
			defer stop()
		}
		if sendErr = send(writeContext); sendErr != nil {
			return sendErr
		}
		_, err = tx.Exec(ctx, `UPDATE link_commands SET dispatched = true WHERE id = $1`, command.GetId())
		sent = err == nil
		return err
	})
	// The store sanitizes callback errors. Keep the transport error for Open's fixed status mapping.
	if sendErr != nil {
		return false, sendErr
	}
	return sent && err == nil, commandStoreError(ctx, err)
}

func commandStoreError(ctx context.Context, err error) error {
	if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
		return errDatabaseUnavailable
	}
	return err
}
