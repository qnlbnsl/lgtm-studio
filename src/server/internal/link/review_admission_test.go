package link

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type streamReviewQueryTrace struct {
	handshakeQueryTrace
	end func(context.Context, pgx.TraceQueryEndData)
}

func (tr streamReviewQueryTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	tr.end(ctx, data)
}

type replacementCommitKey struct{}

func TestRemovalAfterReplacementCommitBeforePublication(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			var armCommit atomic.Bool
			var admissionConn atomic.Pointer[pgx.Conn]
			committed := make(chan struct{}, 1)
			hold, publish := storetest.Signal(t)
			defer publish()
			f := newHandshakeFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
				cfg.ConnConfig.Tracer = streamReviewQueryTrace{
					handshakeQueryTrace: func(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
						sql := strings.ToLower(strings.TrimSpace(data.SQL))
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
							<-hold
						}
					},
				}
			}})
			m, key := f.register(t, "station awaiting replacement")
			client := f.wire(t, handshakeWireOptions{}, protocol.options...)
			old := startHandshake(t, client, nil)
			submitProof(t, f, old, m.ID, key, 1)
			requireWelcome(t, old)
			a := f.current(t, m.ID)
			awaitHeld(t, f.hub, m.ID, a.StreamID)

			replacement := startHandshake(t, client, nil)
			hello := helloFrame(m.ID, 1, 2)
			sendHandshake(t, replacement, hello)
			challenge := requireChallenge(t, replacement, f.serverKey, hello.GetHello())
			// Arm after Challenge, whose lookup may still be completing on another connection.
			armCommit.Store(true)
			sendHandshake(t, replacement, proofFrame(key, hello.GetHello(), challenge))
			storetest.Receive(t, "the replacement commit before hub publication", committed)

			var streamID string
			var generation int64
			err := f.obs.QueryRow(t.Context(),
				"SELECT stream_id, generation FROM link_connections WHERE machine_id = $1", m.ID,
			).Scan(&streamID, &generation)
			if err != nil || generation != a.Generation+1 || streamID == a.StreamID {
				t.Fatal("The observer did not see the replacement at the commit barrier.")
			}
			if _, err := f.state.CheckCurrent(t.Context(), m.ID, a.StreamID); !errors.Is(err, ErrNotCurrent) {
				t.Fatal("The durable identity check did not reject the displaced stream.")
			}
			holdCheck := storetest.Start(t, func() bool {
				return f.hub.Holds(m.ID, a.StreamID)
			})
			if holdCheck.Wait(t) {
				t.Error("The displaced stream still held a live action gate after the replacement committed.")
			}

			if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
				if err := f.r.Delete(ctx, tx, m.ID); err != nil {
					return err
				}
				return f.state.Forget(ctx, tx, m.ID)
			}); err != nil {
				t.Fatal("The fixture removal could not commit during the publication hold.")
			}
			afterRemoval := f.head(t)
			notifier := storetest.Start(t, func() struct{} {
				f.hub.EndRemoved(m.ID)
				return struct{}{}
			})
			notifier.Wait(t)
			publish()

			frame, err := replacement.Receive()
			if welcome := frame.GetWelcome(); welcome != nil {
				if welcome.GetState().GetApproval() == linkv1.Approval_APPROVAL_REMOVED {
					t.Error("The replacement Welcome carried the removed standing.")
				}
				sendHandshake(t, replacement, heartbeatFrame(nil, nil))
				frame, err = replacement.Receive()
			}
			checkOpenEnd(t, frame, err, connect.CodeFailedPrecondition, openUnknown, "unknown_machine")
			f.requireSaved(t, afterRemoval)
			if f.connectionCount(t, m.ID) != 0 {
				t.Error("The replacement restored a connection row after removal.")
			}

			requireOpenEnd(t, old, connect.CodeAborted, openReplaced, "")
		})
	}
}
