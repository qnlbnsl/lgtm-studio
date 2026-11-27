package link

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func reviewStateNotice(approval linkv1.Approval, name string, revision uint64) *linkv1.Command {
	return &linkv1.Command{Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{
		State: &linkv1.MachineState{Approval: approval, Name: name, Revision: revision},
	}}}
}

func TestEnqueuePreservesDeclaredNoticeAndRestart(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	machine, _ := f.register(t, "enqueue fixture")
	f.approve(t, machine.ID)
	const approved = linkv1.Approval_APPROVAL_APPROVED
	enqueue := func(command *linkv1.Command) error {
		return f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if _, err := f.r.Lock(ctx, tx, machine.ID); err != nil {
				return err
			}
			return f.state.Enqueue(ctx, tx, machine.ID, command, time.Minute)
		})
	}
	readCommand := func(t *testing.T, action string) *linkv1.Command {
		t.Helper()
		var payload []byte
		var command linkv1.Command
		if f.obs.QueryRow(t.Context(), `SELECT payload FROM link_commands WHERE machine_id = $1 AND action = $2`,
			machine.ID, action).Scan(&payload) != nil || payload == nil || proto.Unmarshal(payload, &command) != nil {
			t.Fatal("The observer could not decode the committed command.")
		}
		return &command
	}
	unknown := protowire.AppendVarint(protowire.AppendTag(nil, 40, protowire.VarintType), 1)

	t.Run("declared notice", func(t *testing.T) {
		notice := reviewStateNotice(approved, "  Bench station  ", 17)
		notice.GetSetState().GetState().ProtoReflect().SetUnknown(unknown)
		if enqueue(notice) != nil {
			t.Fatal("The valid state notice did not enqueue.")
		}
		committed := readCommand(t, "set_state")
		if committed.GetSetState().GetState().GetRevision() != 17 {
			t.Fatal("The committed state notice did not preserve the declared revision.")
		}
		if !proto.Equal(committed, reviewStateNotice(approved, "Bench station", 17)) {
			t.Error("The committed state notice did not contain exactly the declared fields and normalized name.")
		}
	})

	t.Run("declared restart", func(t *testing.T) {
		restart := &linkv1.Restart{}
		restart.ProtoReflect().SetUnknown(unknown)
		if enqueue(&linkv1.Command{Action: &linkv1.Command_Restart{Restart: restart}}) != nil {
			t.Fatal("The valid restart did not enqueue.")
		}
		want := &linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}}
		if !proto.Equal(readCommand(t, "restart"), want) {
			t.Error("The committed restart did not contain exactly the declared action.")
		}
	})

	for _, tc := range []struct {
		name   string
		notice *linkv1.Command
	}{
		{"missing notice", &linkv1.Command{Action: &linkv1.Command_SetState{}}},
		{"missing state", &linkv1.Command{Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{}}}},
		{"zero revision", reviewStateNotice(approved, "Station", 0)},
		{"waiting standing", reviewStateNotice(linkv1.Approval_APPROVAL_WAITING, "Station", 17)},
		{"removed standing", reviewStateNotice(linkv1.Approval_APPROVAL_REMOVED, "Station", 17)},
		{"empty normalized name", reviewStateNotice(approved, "  ", 17)},
		{"control in name", reviewStateNotice(approved, "Station\nname", 17)},
		{"overlong name", reviewStateNotice(approved, strings.Repeat("n", 65), 17)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count, head := outboxCount(t, f.presenceFixture), f.head(t)
			if !errors.Is(enqueue(tc.notice), ErrInvalidMessage) {
				t.Error("Enqueue did not reject an invalid state notice.")
			}
			if outboxCount(t, f.presenceFixture) != count || f.head(t) != head {
				t.Error("An invalid state notice committed a command or wake.")
			}
		})
	}
}

func reviewRemoveWatchdogMachine(t *testing.T, f *handshakeFixture, id string) {
	t.Helper()
	if f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if err := f.r.Delete(ctx, tx, id); err != nil {
			return err
		}
		return f.state.Forget(ctx, tx, id)
	}) != nil {
		t.Fatal("The fixture could not commit the machine removal.")
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT NOT EXISTS (SELECT 1 FROM machines WHERE id = $1)
		AND NOT EXISTS (SELECT 1 FROM link_connections WHERE machine_id = $1)`, id) {
		t.Fatal("The observer did not see the committed machine removal.")
	}
}

func TestWatchdogAbortsDisplacedStreamBeforeReplacementPublication(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			var armCommit atomic.Bool
			var admissionConn atomic.Pointer[pgx.Conn]
			committed := make(chan struct{}, 1)
			holdPublication, publish := storetest.Signal(t)
			defer publish()
			f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
				cfg.ConnConfig.Tracer = streamReviewQueryTrace{
					handshakeQueryTrace: func(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
						sql := strings.ToLower(strings.TrimSpace(query.SQL))
						if armCommit.Load() && strings.HasPrefix(sql, "insert into link_machines") {
							admissionConn.Store(conn)
						}
						if sql == "commit" && admissionConn.CompareAndSwap(conn, nil) {
							return context.WithValue(ctx, replacementCommitKey{}, true)
						}
						return ctx
					},
					end: func(ctx context.Context, data pgx.TraceQueryEndData) {
						if ctx.Value(replacementCommitKey{}) == true && data.Err == nil {
							committed <- struct{}{}
							<-holdPublication
						}
					},
				}
			}})
			machine, key := f.register(t, "idle before replacement")
			f.approve(t, machine.ID)
			clock := newSteadyClock()
			client := steadyWire(t, f, streamTestOptions{controls: clock.controls(), clientOptions: protocol.options})
			old, _ := openSteady(t, f, client, machine.ID, key, 1)
			tick := storetest.Receive(t, "the displaced stream's watchdog", clock.created)
			sendHandshake(t, old, heartbeatFrame(nil, nil))
			requireHeartbeatAck(t, old)
			original := f.current(t, machine.ID)

			replacement := startHandshake(t, client, nil)
			hello := helloFrame(machine.ID, LinkRevision, 2)
			sendHandshake(t, replacement, hello)
			challenge := requireChallenge(t, replacement, f.serverKey, hello.GetHello())
			armCommit.Store(true)
			sendHandshake(t, replacement, proofFrame(key, hello.GetHello(), challenge))
			storetest.Receive(t, "the replacement commit before hub publication", committed)
			current := f.current(t, machine.ID)
			if current.StreamID == original.StreamID || current.Generation != original.Generation+1 {
				t.Fatal("The observer did not see the committed replacement while publication was held.")
			}
			reviewRemoveWatchdogMachine(t, f, machine.ID)

			// The publication barrier stays held through the entire watchdog decision.
			clock.set(HeartbeatTimeout + time.Second)
			tick.fire <- clock.now()
			frame, err := old.Receive()
			checkOpenEnd(t, frame, err, connect.CodeAborted, openReplaced, "")
			if !connect.IsWireError(err) {
				t.Error("The displaced stream did not receive its terminal status from the server.")
			}
		})
	}
}

func TestWatchdogReportsDatabaseUnavailableWhenIdentityReadTimesOut(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			machine, key := f.register(t, "idle before identity timeout")
			f.approve(t, machine.ID)
			releaseFeed, resumeFeed := storetest.Signal(t)
			defer resumeFeed()
			runnerReady(t, f, func(sub *store.Subscription) {
				select {
				case <-releaseFeed:
				case <-sub.Context().Done():
				}
			})

			clock := newSteadyClock()
			client := steadyWire(t, f, streamTestOptions{controls: clock.controls(), clientOptions: protocol.options})
			stream, _ := openSteady(t, f, client, machine.ID, key, 1)
			tick := storetest.Receive(t, "the idle stream's watchdog", clock.created)
			sendHandshake(t, stream, heartbeatFrame(nil, nil))
			requireHeartbeatAck(t, stream)
			reviewRemoveWatchdogMachine(t, f, machine.ID)

			block, err := f.obs.Begin(t.Context())
			if err != nil {
				t.Fatal("The observer could not start the registry lock transaction.")
			}
			defer func() { _ = block.Rollback(t.Context()) }()
			storetest.Exec(t, block, "LOCK TABLE machines IN ACCESS EXCLUSIVE MODE")
			clock.set(HeartbeatTimeout + time.Second)
			tick.fire <- clock.now()
			storetest.Poll(t, "the watchdog identity read blocked by the registry table lock", func() bool {
				return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
					SELECT 1 FROM pg_locks WHERE relation = 'machines'::regclass
					AND mode = 'AccessShareLock' AND NOT granted AND $1 = ANY(pg_blocking_pids(pid)))`,
					int32(f.obs.PgConn().PID()))
			})

			frame, err := stream.Receive()
			checkOpenEnd(t, frame, err, connect.CodeUnavailable, openDatabase, "")
			if !connect.IsWireError(err) {
				t.Error("The idle stream did not receive its terminal status from the server.")
			}
		})
	}
}
