package link

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type oneLineHolderKey struct{}

// checkOneLine holds first immediately after the registry row lock. It observes either
// the second operation waiting on PostgreSQL or its retryable nonwaiting refusal.
func checkOneLine(t *testing.T, f *presenceFixture, first, second func(context.Context) error, nonwaiting bool) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	locked := make(chan struct{})
	release, unlock := storetest.Signal(t)
	f.state.afterLock = func(ctx context.Context, _ string) {
		if ctx.Value(oneLineHolderKey{}) == true {
			close(locked)
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
	}
	holder := storetest.Start(t, func() error { return first(context.WithValue(ctx, oneLineHolderKey{}, true)) })
	defer func() {
		unlock()
		holder.Wait(t)
	}()
	storetest.ReceiveFrom(t, holder, "the first operation's registry lock", locked)
	waiter := storetest.Start(t, func() error { return second(ctx) })
	if nonwaiting {
		select {
		case <-waiter.Done():
			var retry *store.Error
			if err := waiter.Result(); !errors.As(err, &retry) || !retry.Retryable() || retry.SQLState != "55P03" {
				t.Fatal("Disconnect did not return the retryable PostgreSQL lock error.")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Disconnect waited on the locked machine instead of returning immediately.")
		}
	} else {
		storetest.Poll(t, "the competing operation waiting for the first registry lock", func() bool {
			select {
			case <-waiter.Done():
				t.Fatal("The second operation returned before the first released the registry lock.")
			default:
			}
			return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
				SELECT FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'
				AND lower(query) LIKE '%for no key update%' AND cardinality(pg_blocking_pids(pid)) > 0)`)
		})
	}
	unlock()
	if holder.Wait(t) != nil {
		t.Fatal("The first operation failed after its registry lock was released.")
	}
	return waiter.Wait(t)
}

func TestWritesOfOneMachineFormOneLine(t *testing.T) {
	for _, tc := range []struct {
		name       string
		admitFirst bool
		disconnect bool
	}{
		{"heartbeat then admission", false, false},
		{"admission then heartbeat", true, false},
		{"disconnect then admission", false, true},
		{"admission then disconnect", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPresenceFixture(t, nil)
			m := f.machine(t, bytes.Repeat([]byte{1}, 32), "station")
			a := f.admit(t, m.ID, 1)
			b := fixtureStreamID(t)
			before := f.head(t)
			admission := func(ctx context.Context) error {
				s, err := f.state.BecomeCurrent(ctx, m.ID, b, presenceAddress)
				if err == nil && (s.ID != b || s.Generation != 2) {
					return errors.New("Admission did not return the replacement generation.")
				}
				return err
			}
			old := func(ctx context.Context) error {
				if tc.disconnect {
					deleted, err := f.state.Disconnect(ctx, m.ID, a.ID)
					if err == nil && !deleted {
						return errors.New("The current stream was not deleted.")
					}
					return err
				}
				return f.state.Heartbeat(ctx, m.ID, a.ID, &linkv1.Heartbeat{CpuPercent: proto.Uint32(7)})
			}
			if tc.admitFirst {
				err := checkOneLine(t, f, admission, old, tc.disconnect)
				if tc.disconnect {
					if changed, err := f.state.Disconnect(t.Context(), m.ID, a.ID); err != nil || changed {
						t.Error("Retrying the old disconnect affected the replacement.")
					}
				} else if !errors.Is(err, ErrNotCurrent) {
					t.Error("The displaced heartbeat did not return ErrNotCurrent.")
				}
				f.requireSaved(t, before, m.ID)
			} else {
				if err := checkOneLine(t, f, old, admission, false); err != nil {
					t.Fatal("Admission did not succeed after the preceding write committed.")
				}
				f.requireSaved(t, before, m.ID, m.ID)
			}
			if p := f.current(t, m.ID); p.StreamID != b || p.Generation != 2 || p.CPUPercent != nil {
				t.Error("The old stream's load or cleanup changed the replacement row.")
			}
		})
	}
}

func TestOldStreamCannotChangeRecord(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "station")
	a := f.admit(t, m.ID, 1)
	b := f.admit(t, m.ID, 2)
	original := f.current(t, m.ID)
	before := f.head(t)

	holder, err := f.obs.Begin(t.Context())
	if err != nil {
		t.Fatal("The fixture could not begin its row lock.")
	}
	defer func() { _ = holder.Rollback(t.Context()) }()
	storetest.Exec(t, holder, "SELECT id FROM machines WHERE id = $1 FOR NO KEY UPDATE", m.ID)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if changed, err := f.state.Disconnect(ctx, m.ID, a.ID); err != nil || changed {
		var retry *store.Error
		if changed || !errors.As(err, &retry) || !retry.Retryable() {
			t.Fatal("The stale disconnect neither ignored the old identifier nor returned a retryable lock error.")
		}
	}
	if holder.Rollback(t.Context()) != nil {
		t.Fatal("The fixture row lock did not release.")
	}

	if err := f.state.Heartbeat(t.Context(), m.ID, a.ID, &linkv1.Heartbeat{CpuPercent: proto.Uint32(101)}); !errors.Is(err, ErrNotCurrent) {
		t.Error("The late heartbeat's load was checked before the stream identifier.")
	}
	if err := f.state.Heartbeat(t.Context(), m.ID, a.ID, &linkv1.Heartbeat{CpuPercent: proto.Uint32(7)}); !errors.Is(err, ErrNotCurrent) {
		t.Error("A valid heartbeat from the old stream did not return ErrNotCurrent.")
	}
	if changed, err := f.state.Disconnect(t.Context(), m.ID, a.ID); err != nil || changed {
		t.Error("The late delete was not a stale no-op.")
	}
	if p := f.current(t, m.ID); p.StreamID != b.ID || p.CPUPercent != nil || !p.LastHeardAt.Equal(original.LastHeardAt) {
		t.Error("The old stream changed the replacement's connection or contact time.")
	}
	f.requireSaved(t, before)
}
