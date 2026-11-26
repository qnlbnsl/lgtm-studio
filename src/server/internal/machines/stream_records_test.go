package machines_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func requireMachineLinkRows(t *testing.T, f *streamRecordsFixture, id string, want bool) {
	t.Helper()
	for _, table := range []string{"machines", "link_machines", "link_connections"} {
		column := "machine_id"
		if table == "machines" {
			column = "id"
		}
		exists := storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM "+table+" WHERE "+column+" = $1)", id)
		if exists != want {
			t.Fatalf("Existence in %s was %t, want %t.", table, exists, want)
		}
	}
}

func TestRemoveClearsLinkRowsAtomically(t *testing.T) {
	f := newStreamRecordsFixture(t)
	id := f.join(t, f.issue(t, false))
	a, _ := f.admit(t, id, 1)
	f.apply(t, id, a.ID, 1, true)
	var fail atomic.Bool
	fail.Store(true)
	var calls atomic.Int32
	f.presence.forget = func(ctx context.Context, tx *store.Tx, target string) error {
		calls.Add(1)
		var parent bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT FROM machines WHERE id = $1)", target).Scan(&parent); err != nil {
			return err
		}
		if parent {
			return errors.New("The parent machine remained visible in the removal write.")
		}
		if err := f.presence.State.Forget(ctx, tx, target); err != nil {
			return err
		}
		if fail.Load() {
			return errors.New("The fixture rejected Forget after its link writes.")
		}
		return nil
	}
	w := f.watch(t)
	before := f.head(t)
	remove := func() error {
		_, err := f.clients.RemoveClient(t.Context(), recordRequest(f.recordsFixture, &webv1.RemoveClientRequest{Id: id}, "remove-link-rows"))
		return err
	}
	if err := remove(); connect.CodeOf(err) != connect.CodeUnavailable || calls.Load() != 1 {
		t.Fatal("A failure in Forget did not refuse the same keyed machine removal.")
	}
	requireMachineLinkRows(t, f, id, true)
	if f.head(t) != before {
		t.Fatal("A failed removal published a feed row.")
	}
	fail.Store(false)
	if remove() != nil || calls.Load() != 2 {
		t.Fatal("The keyed removal could not be retried after rollback.")
	}
	requireMachineLinkRows(t, f, id, false)
	requireRecordRemoval(t, w, id)
	before = f.head(t)
	if remove() != nil || f.head(t) != before || calls.Load() != 2 {
		t.Fatal("Removal replay attempted another deletion.")
	}
}

func TestRevokeClearsOnlyWaitingLinkRows(t *testing.T) {
	for _, approved := range []bool{false, true} {
		name := "waiting"
		if approved {
			name = "approval wins"
		}
		t.Run(name, func(t *testing.T) {
			f := newStreamRecordsFixture(t)
			token := f.issue(t, false)
			id := f.join(t, token)
			a, _ := f.admit(t, id, 1)
			f.apply(t, id, a.ID, 1, true)
			if approved {
				if _, err := f.approve(t, id, "approve-before-revoke"); err != nil {
					t.Fatal("Approval did not commit before revocation.")
				}
			}
			var calls atomic.Int32
			f.presence.forget = func(ctx context.Context, tx *store.Tx, id string) error {
				calls.Add(1)
				return f.presence.State.Forget(ctx, tx, id)
			}
			w := f.watch(t)
			revoke := func() error {
				_, err := f.clients.RevokeClientToken(t.Context(), recordRequest(f.recordsFixture, &webv1.RevokeClientTokenRequest{TokenId: token.GetTokenId()}, "revoke-link-rows"))
				return err
			}
			if revoke() != nil {
				t.Fatal("The keyed token revocation failed.")
			}
			requireMachineLinkRows(t, f, id, approved)
			if approved {
				if calls.Load() != 0 || !f.hub.Holds(id, a.ID) {
					t.Fatal("Revoking an approved machine's token touched its live stream.")
				}
				if f.presence.Heartbeat(t.Context(), id, a.ID, &linkv1.Heartbeat{}) != nil {
					t.Fatal("Revocation fenced an approved machine's durable stream.")
				}
			} else {
				if calls.Load() != 1 {
					t.Fatal("Waiting token revocation did not forget its link rows exactly once.")
				}
				requireRecordRemoval(t, w, id)
			}
			before, count := f.head(t), calls.Load()
			if revoke() != nil || calls.Load() != count || f.head(t) != before {
				t.Fatal("Revocation replay attempted another deletion.")
			}
		})
	}
}
