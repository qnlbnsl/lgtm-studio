package machines_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type actionClient interface {
	ScanClientHarnesses(context.Context, *connect.Request[webv1.ScanClientHarnessesRequest]) (*connect.Response[webv1.ScanClientHarnessesResponse], error)
	ReconnectClient(context.Context, *connect.Request[webv1.ReconnectClientRequest]) (*connect.Response[webv1.ReconnectClientResponse], error)
}

type actionRoute struct {
	name, refusal string
	call          func(context.Context, *recordsFixture, actionClient, string, string) error
	payload       *linkv1.Command
}

var actionRoutes = []actionRoute{
	{
		name: "scan_harnesses", refusal: "This machine is not online.",
		call: func(ctx context.Context, f *recordsFixture, client actionClient, id, key string) error {
			reply, err := client.ScanClientHarnesses(ctx, recordRequest(f, &webv1.ScanClientHarnessesRequest{Id: id}, key))
			if err == nil && (reply == nil || !proto.Equal(reply.Msg, &webv1.ScanClientHarnessesResponse{})) {
				return errors.New("Scan returned a nonempty response.")
			}
			return err
		},
		payload: &linkv1.Command{Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}}},
	},
	{
		name: "restart", refusal: "This machine is not connected.",
		call: func(ctx context.Context, f *recordsFixture, client actionClient, id, key string) error {
			reply, err := client.ReconnectClient(ctx, recordRequest(f, &webv1.ReconnectClientRequest{Id: id}, key))
			if err == nil && (reply == nil || !proto.Equal(reply.Msg, &webv1.ReconnectClientResponse{})) {
				return errors.New("Restart returned a nonempty response.")
			}
			return err
		},
		payload: &linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}},
	},
}

func approveActionMachine(t *testing.T, f *stateCommandsFixture, id string) {
	t.Helper()
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.reg.SetApproved(ctx, tx, id, "Action fixture "+id[len(id)-12:])
	}); err != nil {
		t.Fatal("The action fixture could not approve its machine.")
	}
}

func requireActionRefusal(t *testing.T, err error, code connect.Code, sentence string) {
	t.Helper()
	var refusal *connect.Error
	if !errors.As(err, &refusal) || refusal.Code() != code || (sentence != "" && refusal.Message() != sentence) {
		t.Errorf("The action did not return the expected %s refusal and message.", code)
	}
}

func requireQueuedAction(t *testing.T, f *stateCommandsFixture, route actionRoute, id, key string, head int64) {
	t.Helper()
	var payload []byte
	var initial bool
	err := f.obs.QueryRow(t.Context(), `SELECT payload, attempt = 1 AND sent_stream_id IS NULL
		AND NOT dispatched AND outcome IS NULL AND finished_at IS NULL
		AND expires_at - created_at = interval '2 minutes'
		FROM link_commands WHERE machine_id = $1 AND action = $2 ORDER BY seq DESC LIMIT 1`, id, route.name).Scan(&payload, &initial)
	if err != nil || !initial {
		t.Error("Acceptance did not save a pending, unassigned two-minute command.")
	}
	var saved linkv1.Command
	if proto.Unmarshal(payload, &saved) != nil || !proto.Equal(&saved, route.payload) {
		t.Error("The stored command did not contain exactly the requested action.")
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 1 AND count(*) FILTER
		(WHERE fingerprint_version = 1 AND octet_length(reply) = 0) = 1
		FROM receipts WHERE request_key = $1`, key) {
		t.Error("Acceptance did not save one version-one receipt for the empty reply.")
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 1 AND count(*) FILTER
		(WHERE topic = 'machine_outbox' AND scope = '' AND key = $2 AND change = 'saved') = 1
		FROM feed WHERE position > $1`, head, id) {
		t.Error("Acceptance did not emit exactly one outbox wake.")
	}
}

func TestScanAndReconnectQueueCommands(t *testing.T) {
	f := newStateCommandsFixture(t, nil)
	id := f.join(t, f.issue(t, false))
	check := func(t *testing.T, machine string, client actionClient, scan, restart bool) {
		t.Helper()
		for index, route := range actionRoutes {
			t.Run(route.name, func(t *testing.T) {
				before, count, head := f.digest(t), f.noticeCount(t, machine), f.head(t)
				key, err := store.NewID()
				if err != nil {
					t.Fatal("The action fixture could not allocate a request key.")
				}
				err = route.call(t.Context(), f.recordsFixture, client, machine, key)
				want := []bool{scan, restart}[index]
				switch {
				case machine == unknownRecordID:
					requireActionRefusal(t, err, connect.CodeNotFound, "No machine has that id.")
				case !want:
					requireActionRefusal(t, err, connect.CodeFailedPrecondition, route.refusal)
				default:
					if err != nil {
						t.Fatalf("The eligible action did not return its empty reply. The code was %v.", connect.CodeOf(err))
					}
					if f.noticeCount(t, machine) != count+1 {
						t.Error("Acceptance did not add exactly one command.")
					}
					requireQueuedAction(t, f, route, machine, key, head)
				}
				if !want && f.digest(t) != before {
					t.Error("The refused action left a command, receipt, feed entry or machine change.")
				}
			})
		}
	}
	t.Run("missing", func(t *testing.T) { check(t, unknownRecordID, f.clients, false, false) })
	a, cancelA := f.admit(t, id, 1)
	f.apply(t, id, a.ID, 1, true)
	f.apply(t, id, a.ID, 2, false)
	// Seed both flags so approval is the only remaining eligibility condition.
	storetest.Exec(t, f.obs, `UPDATE link_connections SET facts_received = true, harnesses_received = true
		WHERE machine_id = $1 AND stream_id = $2`, id, a.ID)
	t.Run("waiting with both reports", func(t *testing.T) { check(t, id, f.clients, false, false) })
	cancelA(nil)
	if changed, err := f.presence.Disconnect(t.Context(), id, a.ID); err != nil || !changed {
		t.Fatal("The fixture could not disconnect its waiting machine.")
	}
	approveActionMachine(t, f, id)
	t.Run("offline", func(t *testing.T) { check(t, id, f.clients, false, false) })

	b, cancelB := f.admit(t, id, 2)
	t.Run("joining", func(t *testing.T) { check(t, id, f.clients, false, true) })
	f.apply(t, id, b.ID, 3, true)
	t.Run("facts only", func(t *testing.T) { check(t, id, f.clients, false, true) })
	f.apply(t, id, b.ID, 4, false)
	t.Run("online", func(t *testing.T) { check(t, id, f.clients, true, true) })
	t.Run("no queue", func(t *testing.T) { check(t, id, f.old, false, false) })
	t.Run("no link", func(t *testing.T) {
		svc := machines.NewService(f.s, nil, f.reg, f.tokens).WithCommands(f.queue)
		check(t, id, svc, false, false)
	})

	t.Run("canceled with a stale row after failed deletion", func(t *testing.T) {
		storetest.Exec(t, f.obs, `CREATE FUNCTION fail_action_disconnect() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'Fixture refused the deletion'; END $$;
			CREATE TRIGGER fail_action_disconnect BEFORE DELETE ON link_connections
			FOR EACH ROW EXECUTE FUNCTION fail_action_disconnect()`)
		defer storetest.Exec(t, f.obs, "DROP TRIGGER fail_action_disconnect ON link_connections")
		cancelB(nil)
		if changed, err := f.presence.Disconnect(t.Context(), id, b.ID); err == nil || changed ||
			!storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM link_connections
				WHERE machine_id = $1 AND stream_id = $2 AND facts_received AND harnesses_received)`, id, b.ID) {
			t.Fatal("The fixture did not leave the old online row after failed cleanup.")
		}
		check(t, id, f.clients, false, false)
	})

	c, _ := f.admit(t, id, 3)
	f.apply(t, id, c.ID, 5, false)
	t.Run("harnesses only", func(t *testing.T) { check(t, id, f.clients, false, true) })
	t.Run("replacement with only the old handle", func(t *testing.T) {
		replacement, err := store.NewID()
		if err != nil {
			t.Fatal("The fixture could not allocate the replacement identifier.")
		}
		row, err := f.presence.State.BecomeCurrent(t.Context(), id, replacement, "10.2.3.4", projectionRun(id))
		if err != nil || row.Generation != 4 {
			t.Fatal("The fixture could not commit the replacement row.")
		}
		f.apply(t, id, replacement, 6, true)
		f.apply(t, id, replacement, 7, false)
		if !f.hub.Holds(id, c.ID) || f.hub.Holds(id, replacement) {
			t.Fatal("The fixture did not preserve only the old hold after replacement.")
		}
		check(t, id, f.clients, false, false)
	})
}

func TestActionRequestKeyReplays(t *testing.T) {
	for _, route := range actionRoutes {
		t.Run(route.name, func(t *testing.T) {
			gate := newStateCommitGate(t)
			f := newStateCommandsFixture(t, gate)
			id := f.join(t, f.issue(t, false))
			other := f.join(t, f.issue(t, false))
			approveActionMachine(t, f, id)
			approveActionMachine(t, f, other)
			row, cancel := f.admit(t, id, 1)
			f.apply(t, id, row.ID, 1, true)
			f.apply(t, id, row.ID, 2, false)
			const key = "action-repeated"
			call := func(id, key string) error {
				return route.call(t.Context(), f.recordsFixture, f.clients, id, key)
			}
			head := f.head(t)
			gate.armed.Store(true)
			first := storetest.Start(t, func() error { return call(id, key) })
			defer gate.release()
			pid := stateCommitPID(t, gate, first)
			second := storetest.Start(t, func() error { return call(id, key) })
			storetest.Poll(t, "the second action waiting on the first action's request lock", func() bool {
				select {
				case <-second.Done():
					t.Fatal("The concurrent action returned before the first receipt committed.")
				default:
				}
				return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
					SELECT FROM pg_locks WHERE locktype = 'advisory' AND NOT granted
					AND $1 = ANY(pg_blocking_pids(pid)))`, pid)
			})
			if f.noticeCount(t, id) != 0 || f.head(t) != head ||
				storetest.Scalar[int](t, f.obs, "SELECT count(*) FROM receipts") != 0 {
				t.Error("The command, wake or receipt became visible before the keyed commit.")
			}
			gate.release()
			if first.Wait(t) != nil || second.Wait(t) != nil || f.noticeCount(t, id) != 1 {
				t.Fatal("The concurrent requests did not return two empty replies and save one command.")
			}
			requireQueuedAction(t, f, route, id, key, head)

			cancel(nil)
			if changed, err := f.presence.Disconnect(t.Context(), id, row.ID); err != nil || !changed {
				t.Fatal("The fixture could not disconnect the machine before replay.")
			}
			before := f.digest(t)
			if call(id, key) != nil || f.digest(t) != before {
				t.Error("Replay after disconnection did not return the empty reply without another write.")
			}
			requireActionRefusal(t, call(other, key), connect.CodeAlreadyExists, "")
			if f.digest(t) != before {
				t.Error("A conflicting request body changed the command, receipt or feed.")
			}

			row, _ = f.admit(t, id, 2)
			f.apply(t, id, row.ID, 3, true)
			f.apply(t, id, row.ID, 4, false)
			const retryKey = "action-retry-after-lock"
			t.Run("retryable registry lock", func(t *testing.T) {
				before, head := f.digest(t), f.head(t)
				holder, err := f.obs.Begin(t.Context())
				if err != nil {
					t.Fatal("The fixture could not begin a competing transaction.")
				}
				defer holder.Rollback(context.Background())
				storetest.Exec(t, holder, "SELECT FROM machines WHERE id = $1 FOR NO KEY UPDATE", id)
				requireActionRefusal(t, call(id, retryKey), connect.CodeAborted, "")
				if holder.Rollback(t.Context()) != nil {
					t.Fatal("The fixture could not release its registry lock.")
				}
				if f.digest(t) != before {
					t.Error("The retryable failure saved a command, receipt or feed entry.")
				}
				if call(id, retryKey) != nil || f.noticeCount(t, id) != 2 {
					t.Fatal("The same request key did not succeed after the lock was released.")
				}
				requireQueuedAction(t, f, route, id, retryKey, head)
			})
		})
	}
}
