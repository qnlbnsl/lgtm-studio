package link

import (
	"context"
	"errors"
	"time"
)

const (
	connectBudget = 14 * time.Second
	stableStream  = 30 * time.Second
)

// streamEnd records when a live stream failed before its workers retired.
type streamEnd struct {
	error
	at time.Time
}

func (e streamEnd) Unwrap() error { return e.error }

// supervise waits for an attempt to retire before starting another one.
func supervise(ctx context.Context, attempt func(context.Context, func()) error) error {
	delays := [...]time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 15 * time.Second}
	step := 0
	for ctx.Err() == nil {
		started := time.Now()
		attemptCtx, cancel := context.WithCancel(ctx)
		deadline := time.AfterFunc(connectBudget, cancel)
		var welcome time.Time
		err := attempt(attemptCtx, func() {
			if deadline.Stop() {
				welcome = time.Now()
			}
		})
		deadline.Stop()
		cancel()

		if ctx.Err() != nil {
			break
		}
		var refusal terminalRefusal
		if errors.As(err, &refusal) && refusal.fromWire {
			return refusal
		}
		if !welcome.IsZero() {
			ended := time.Now()
			var closed streamEnd
			if errors.As(err, &closed) {
				ended = closed.at
			}
			if ended.Sub(welcome) >= stableStream {
				step = 0
			}
			continue
		}
		target := started.Add(delays[step])
		if step < len(delays)-1 {
			step++
		}
		if wait := time.Until(target); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
			}
		}
	}
	return ctx.Err()
}

// readElapsedClock samples the elapsed clock at proof send.
var readElapsedClock = elapsedClock
