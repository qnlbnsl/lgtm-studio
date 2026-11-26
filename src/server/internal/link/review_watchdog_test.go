package link

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestRemovedIdleStreamRefusesBeforeHeartbeatTimeout(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			machine, key := f.register(t, "removed idle machine")
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
			tick := storetest.Receive(t, "the watchdog starting after Welcome", clock.created)
			sendHandshake(t, stream, heartbeatFrame(nil, nil))
			requireHeartbeatAck(t, stream)
			head := f.head(t)

			err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
				if err := f.r.Delete(ctx, tx, machine.ID); err != nil {
					return err
				}
				return f.state.Forget(ctx, tx, machine.ID)
			})
			if err != nil {
				t.Fatal("The fixture could not commit the machine removal.")
			}
			if !storetest.Scalar[bool](t, f.obs, `
SELECT NOT EXISTS (SELECT FROM machines WHERE id = $1)
   AND NOT EXISTS (SELECT FROM link_machines WHERE machine_id = $1)
   AND NOT EXISTS (SELECT FROM link_connections WHERE machine_id = $1)
   AND EXISTS (SELECT FROM feed WHERE position > $2 AND topic = 'machines'
               AND key = $1 AND change = 'removed')`, machine.ID, head) {
				t.Fatal("The observer did not see the committed removal and its feed reference.")
			}

			clock.set(HeartbeatTimeout + time.Second)
			tick.fire <- clock.now()
			response, err := stream.Receive()
			checkOpenEnd(t, response, err, connect.CodeFailedPrecondition, openUnknown, "unknown_machine")
			if !connect.IsWireError(err) {
				t.Fatal("The removed stream did not receive a terminal refusal from the server.")
			}
		})
	}
}
