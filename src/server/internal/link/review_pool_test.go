package link

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestCommandSendPoolServesOldestLiveWaiter(t *testing.T) {
	first, second := newCommandWireProbe(), newCommandWireProbe()
	dispatching := make(chan context.Context, 1)
	held, releaseDispatch := storetest.Signal(t)
	defer releaseDispatch()
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
			if ctx.Value(testCommandWireKey{}) == first && q.SQL == "UPDATE link_commands SET dispatched = true WHERE id = $1" {
				dispatching <- ctx
				select {
				case <-held:
				case <-ctx.Done():
				}
			}
			return ctx
		})
	}})
	canceled := newCommandWireProbe()
	server := commandListener(t, f, map[string]*commandWireProbe{"canceled": canceled, "first": first, "second": second})
	client := integrationCommandClient(t, server.URL)
	slots := &f.svc.commandSlots
	var holders [3]*commandSession
	for i := range holders {
		// Fixture sessions own permits without starting the deadline of a transport write.
		holders[i] = &commandSession{handle: &Handle{ctx: t.Context()}}
		if !slots.acquire(holders[i]) {
			t.Fatal("The fixture could not occupy all three command permits.")
		}
		defer slots.release(holders[i])
	}
	waitForQueue := func(count int) {
		t.Helper()
		storetest.Poll(t, "the expected number of registered command waiters", func() bool {
			slots.mu.Lock()
			defer slots.mu.Unlock()
			return slots.waiters.Len() == count
		})
	}
	openWaiter := func(label string, count int) (claimFixtureRow, *openClientStream, context.CancelFunc) {
		t.Helper()
		m, key := f.register(t, label)
		if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			return f.r.SetApproved(ctx, tx, m.ID, label)
		}); err != nil {
			t.Fatal("The fixture could not approve a waiter with its own name.")
		}
		row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
		stream, cancel := openCancellableCommandStream(t, f, client, m, key, label)
		waitForQueue(count)
		requireClaimStored(t, f.presenceFixture, row.id, "", 1)
		return row, stream, cancel
	}
	canceledRow, canceledStream, cancel := openWaiter("canceled", 1)
	firstRow, firstStream, _ := openWaiter("first", 2)
	secondRow, secondStream, _ := openWaiter("second", 3)
	cancel()
	if canceledStream.CloseRequest() != nil {
		t.Fatal("The fixture could not close the canceled waiter's request stream.")
	}
	storetest.Receive(t, "the canceled waiter's handler exiting", canceled.closed)
	waitForQueue(2)
	requireClaimStored(t, f.presenceFixture, canceledRow.id, "", 1)

	slots.release(holders[0])
	select {
	case <-first.flushing:
	case <-second.flushing:
		t.Fatal("The newer live waiter received the first released permit.")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("The oldest live waiter did not reach command flush after a permit was released.")
	}
	receiveIntegrationCommand(t, firstStream, firstRow, 1)
	// The first stream has delivered its frame and still owns its permit until dispatch commits.
	firstDispatch := storetest.Receive(t, "the first waiter's held dispatch", dispatching)
	requireClaimStored(t, f.presenceFixture, secondRow.id, "", 1)

	slots.release(holders[1])
	receiveIntegrationCommand(t, secondStream, secondRow, 1)
	if firstDispatch.Err() != nil {
		t.Fatal("The first waiter's dispatch ended before the second waiter received its command.")
	}
	releaseDispatch()
}
