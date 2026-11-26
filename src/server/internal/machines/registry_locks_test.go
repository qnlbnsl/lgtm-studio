package machines_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// These tests close the gaps post-a 1, post-a 2 and post-a 8 of the review of the built items 1 and 3.

// holdWrite runs fn in a write of its own. Once fn has run, the write stays open until release
// is called, so a second write that starts then meets the first one's locks.
func holdWrite(t *testing.T, s *store.Store, fn func(ctx context.Context, tx *store.Tx) error) (release func(), done *storetest.Task[error]) {
	t.Helper()
	ran := make(chan error, 1)
	gate, release := storetest.Signal(t)
	done = storetest.Start(t, func() error {
		return s.Write(context.Background(), func(ctx context.Context, tx *store.Tx) error {
			err := fn(ctx, tx)
			ran <- err
			if err != nil {
				return err
			}
			<-gate
			return nil
		})
	})
	if err := storetest.Receive(t, "the held write to run", ran); err != nil {
		t.Fatalf("the held write failed: %v", err)
	}
	return release, done
}

// waitForLockWait waits until a backend of the test's database waits on a lock.
func waitForLockWait(t *testing.T, obs *pgx.Conn) {
	t.Helper()
	storetest.Poll(t, "a write to wait on a lock", func() bool {
		var waiting int64
		err := obs.QueryRow(t.Context(), `
SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting)
		return err == nil && waiting >= 1
	})
}

type renameResult struct {
	changed bool
	err     error
}

func (e *regEnv) startRename(t *testing.T, id, name string) *storetest.Task[renameResult] {
	t.Helper()
	return storetest.Start(t, func() renameResult {
		var changed bool
		err := e.s.Write(context.Background(), func(ctx context.Context, tx *store.Tx) error {
			var err error
			changed, err = e.reg.SetName(ctx, tx, id, name)
			return err
		})
		return renameResult{changed, err}
	})
}

// TestApproveHoldsTheRowLock: two approvals of one waiting machine form one line, decided by the
// machine's row. The second reads the machine only after the first committed, so it finds it approved.
func TestApproveHoldsTheRowLock(t *testing.T) {
	e := newRegEnv(t)
	m := e.create(t, 0x41, "box", false)

	release, first := holdWrite(t, e.s, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.SetApproved(ctx, tx, m.ID, "One")
	})
	second := storetest.Start(t, func() error {
		return e.s.Write(context.Background(), func(ctx context.Context, tx *store.Tx) error {
			return e.reg.SetApproved(ctx, tx, m.ID, "Two")
		})
	})
	waitForLockWait(t, e.obs)
	release()
	if err := first.Wait(t); err != nil {
		t.Fatalf("the first approval failed: %v", err)
	}
	if err := second.Wait(t); !errors.Is(err, machines.ErrNotWaiting) {
		t.Errorf("the second approval returned %v, want ErrNotWaiting", err)
	}
	if got := e.mustGet(t, m.ID); got.Name != "One" || got.StateRevision != 2 {
		t.Errorf("the machine is %q at revision %d, want One at revision 2", got.Name, got.StateRevision)
	}
}

// TestRenameHoldsTheRowLock: SetName reads the current name under the row lock, so a rename that
// races an approval or another rename to the same name sees it and changes nothing.
func TestRenameHoldsTheRowLock(t *testing.T) {
	t.Run("against an approval", func(t *testing.T) {
		e := newRegEnv(t)
		m := e.create(t, 0x41, "box", false)
		release, first := holdWrite(t, e.s, func(ctx context.Context, tx *store.Tx) error {
			return e.reg.SetApproved(ctx, tx, m.ID, "One")
		})
		second := e.startRename(t, m.ID, "One")
		waitForLockWait(t, e.obs)
		release()
		if err := first.Wait(t); err != nil {
			t.Fatalf("the approval failed: %v", err)
		}
		if r := second.Wait(t); r.err != nil || r.changed {
			t.Errorf("the rename to the approved name returned %+v, want no change and no error", r)
		}
		if got := e.mustGet(t, m.ID); got.Name != "One" || got.StateRevision != 2 {
			t.Errorf("the machine is %q at revision %d, want One at revision 2", got.Name, got.StateRevision)
		}
	})

	t.Run("against a rename", func(t *testing.T) {
		e := newRegEnv(t)
		m := e.create(t, 0x41, "box", false)
		if err := e.write(t, func(ctx context.Context, tx *store.Tx) error { return e.reg.SetApproved(ctx, tx, m.ID, "One") }); err != nil {
			t.Fatalf("approving failed: %v", err)
		}
		release, first := holdWrite(t, e.s, func(ctx context.Context, tx *store.Tx) error {
			changed, err := e.reg.SetName(ctx, tx, m.ID, "Two")
			if err == nil && !changed {
				t.Error("the first rename reports no change")
			}
			return err
		})
		second := e.startRename(t, m.ID, "Two")
		waitForLockWait(t, e.obs)
		release()
		if err := first.Wait(t); err != nil {
			t.Fatalf("the first rename failed: %v", err)
		}
		if r := second.Wait(t); r.err != nil || r.changed {
			t.Errorf("the second rename to the same name returned %+v, want no change and no error", r)
		}
		if got := e.mustGet(t, m.ID); got.Name != "Two" || got.StateRevision != 3 {
			t.Errorf("the machine is %q at revision %d, want Two at revision 3", got.Name, got.StateRevision)
		}
	})
}

// TestRevokeHoldsTheTokenRowLock: a revocation that meets a use of the token waits for it and
// then reads who used the token. Without the lock it returns no machine, and the caller would
// leave a waiting machine of a revoked token alive.
func TestRevokeHoldsTheTokenRowLock(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	tok, _ := e.create(t, machines.TokenOptions{})
	const user = "machine-of-the-use"

	release, first := holdWrite(t, e.s, func(ctx context.Context, tx *store.Tx) error {
		// This is what a redemption does: lock the row, then bind the token.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM client_join_tokens WHERE id = $1 FOR UPDATE`, tok.ID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE client_join_tokens SET used_at = clock_timestamp(), used_key = $2, used_by = $3 WHERE id = $1`,
			tok.ID, keyOf(0x31), user)
		return err
	})
	second := storetest.Start(t, func() string {
		var usedBy string
		err := e.s.Write(context.Background(), func(ctx context.Context, tx *store.Tx) error {
			var err error
			usedBy, err = e.tokens.Revoke(ctx, tx, tok.ID)
			return err
		})
		if err != nil {
			return "error: " + err.Error()
		}
		return usedBy
	})
	waitForLockWait(t, e.obs)
	release()
	if err := first.Wait(t); err != nil {
		t.Fatalf("the use failed: %v", err)
	}
	if got := second.Wait(t); got != user {
		t.Errorf("Revoke returned %q after the use committed, want %q", got, user)
	}
	if row := e.row(t, tok.ID); row.revokedAt == nil {
		t.Error("the token is not revoked")
	}
}

// TestRegistryNormalizesNames: SetApproved and SetName normalize the name themselves, so a caller
// that skips the page's normalization still stores a trimmed name and never a bad one.
func TestRegistryNormalizesNames(t *testing.T) {
	e := newRegEnv(t)
	m := e.create(t, 0x41, "box", false)

	for _, bad := range []string{"", "   ", "Bo\tx", "Bo\xffx", string(make([]byte, 65))} {
		err := e.write(t, func(ctx context.Context, tx *store.Tx) error { return e.reg.SetApproved(ctx, tx, m.ID, bad) })
		if !errors.Is(err, machines.ErrInvalidName) {
			t.Errorf("SetApproved with the name %q returned %v, want ErrInvalidName", bad, err)
		}
	}
	if got := e.mustGet(t, m.ID); got.Approval != machines.ApprovalWaiting || got.Name != "" || got.StateRevision != 1 {
		t.Fatalf("a refused name changed the machine: %q, %q, revision %d", got.Approval, got.Name, got.StateRevision)
	}

	if err := e.write(t, func(ctx context.Context, tx *store.Tx) error { return e.reg.SetApproved(ctx, tx, m.ID, "  Box ") }); err != nil {
		t.Fatalf("approving as a name with spaces failed: %v", err)
	}
	if got := e.mustGet(t, m.ID); got.Name != "Box" {
		t.Errorf("the approved name is %q, want Box", got.Name)
	}

	var changed bool
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		changed, err = e.reg.SetName(ctx, tx, m.ID, " Two ")
		return err
	})
	if err != nil || !changed {
		t.Fatalf("renaming to a name with spaces returned %v, %v", changed, err)
	}
	if got := e.mustGet(t, m.ID); got.Name != "Two" || got.StateRevision != 3 {
		t.Errorf("the renamed name is %q at revision %d, want Two at revision 3", got.Name, got.StateRevision)
	}
	// The same name in other spacing is no change.
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		var err error
		changed, err = e.reg.SetName(ctx, tx, m.ID, "  Two")
		return err
	})
	if err != nil || changed {
		t.Errorf("renaming to the same name with spaces returned %v, %v, want no change", changed, err)
	}
	err = e.write(t, func(ctx context.Context, tx *store.Tx) error {
		_, err := e.reg.SetName(ctx, tx, m.ID, "Tw\to")
		return err
	})
	if !errors.Is(err, machines.ErrInvalidName) {
		t.Errorf("SetName with a tab returned %v, want ErrInvalidName", err)
	}
	if got := e.mustGet(t, m.ID); got.Name != "Two" {
		t.Errorf("the name is %q after a refused rename, want Two", got.Name)
	}
}
