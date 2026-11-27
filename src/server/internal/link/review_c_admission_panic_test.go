package link

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdmissionCommitPanicAllowsCommandsAfterReconnect(t *testing.T) {
	var armed, panicked atomic.Bool
	var admissionConn atomic.Pointer[pgx.Conn]
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
			sql := strings.ToLower(strings.TrimSpace(data.SQL))
			if armed.Load() && strings.HasPrefix(sql, "insert into link_machines") {
				admissionConn.Store(conn)
			}
			if sql == "commit" && admissionConn.CompareAndSwap(conn, nil) && armed.CompareAndSwap(true, false) {
				panicked.Store(true)
				panic("The fixture interrupted one admission commit.")
			}
			return ctx
		})
	}})
	machine, key := f.register(t, "Admission commit recovery")
	f.approve(t, machine.ID)
	server := commandListener(t, f, nil)
	client := integrationCommandClient(t, server.URL)
	// Keep a live handle without a command reader that could claim the queued restart.
	previous := startHandshake(t, f.wire(t, handshakeWireOptions{}), nil)
	submitProof(t, f, previous, machine.ID, key, 1)
	requireWelcome(t, previous)
	current := f.current(t, machine.ID)
	awaitHeld(t, f.hub, machine.ID, current.StreamID)

	broken := startHandshake(t, client, nil)
	hello := helloFrame(machine.ID, 1, 2)
	sendHandshake(t, broken, hello)
	challenge := requireChallenge(t, broken, f.serverKey, hello.GetHello())
	armed.Store(true)
	sendHandshake(t, broken, proofFrame(key, hello.GetHello(), challenge))
	frame, err := broken.Receive()
	if !panicked.Load() || err == nil || frame.GetWelcome() != nil {
		t.Fatal("The admission commit did not panic and end the replacement Open before Welcome.")
	}

	// A successful later admission can overwrite the panic's marker. Check the surviving stream first.
	if !f.hub.Holds(machine.ID, current.StreamID) {
		t.Error("The admission commit panic left the surviving stream's command gate closed.")
	}
	command := queueSendRestart(t, f.presenceFixture, machine.ID, 0)
	retry := openCommandStream(t, f, client, machine, key, "")
	reconnected := f.current(t, machine.ID)
	if reconnected.StreamID == current.StreamID || reconnected.Generation != current.Generation+1 {
		t.Fatal("The retry did not admit a new stream after the interrupted commit.")
	}
	receiveIntegrationCommand(t, retry, command, 1)
	requireOpenEnd(t, previous, connect.CodeAborted, openReplaced, "")
}
