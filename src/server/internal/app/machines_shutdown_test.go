package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestShutdownDrainsMachineApproval(t *testing.T) {
	cfg, observer := appConfig(t)
	order := &machineShutdownOrder{
		listening:         make(chan struct{}, 1),
		followingAtCommit: make(chan bool, 1),
		followerClosed:    make(chan bool, 1),
		instanceClosed:    make(chan bool, 1),
	}
	cfg.Postgres.ConnConfig.Tracer = order
	cfg.Postgres.BeforeClose = order.beforePoolClose
	server, err := Start(t.Context(), cfg, nil)
	if err != nil {
		t.Fatalf("Starting the application failed: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	storetest.Receive(t, "the PostgreSQL feed listener", order.listening)

	httpClient := appClient(t)
	httpClient.Timeout = storetest.ObservationTimeout
	signIn(t, httpClient, cfg)
	clients := lgtmv1connect.NewClientServiceClient(httpClient, cfg.PublicURL)
	tokenReply, err := clients.CreateClientToken(t.Context(), connect.NewRequest(&webv1.CreateClientTokenRequest{}))
	if err != nil {
		t.Fatalf("Creating the fixture join token failed: %v", err)
	}
	token, err := machines.DecodeToken(tokenReply.Msg.GetToken())
	if err != nil {
		t.Fatal("The fixture join token did not decode.")
	}
	unsigned := &http.Client{Transport: httpClient.Transport, Timeout: httpClient.Timeout}
	link := linkv1connect.NewLinkServiceClient(unsigned, cfg.PublicURL)
	machineKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	join, err := link.Join(t.Context(), connect.NewRequest(&linkv1.JoinRequest{
		TokenId: token.GetTokenId(), TokenSecret: token.GetSecret(),
		MachinePublicKey: machineKey, HostName: "shutdown.example.test",
	}))
	if err != nil {
		t.Fatalf("Joining the fixture machine failed: %v", err)
	}
	id := join.Msg.GetMachineId()

	watch, err := clients.WatchClients(t.Context(), connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err != nil {
		t.Fatalf("Opening the Machines watch failed: %v", err)
	}
	if !watch.Receive() {
		_ = watch.Close()
		t.Fatalf("The Machines watch ended before its snapshot: %v", watch.Err())
	}
	list := watch.Msg().GetAll().GetClients()
	if len(list) != 1 || list[0].GetId() != id || list[0].GetState() != webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL {
		_ = watch.Close()
		t.Fatal("The Machines watch did not start with the waiting machine.")
	}
	watchEnd := storetest.Start(t, func() error {
		defer watch.Close()
		for watch.Receive() {
		}
		return watch.Err()
	})

	// The Store forces deferred domain checks before it appends feed rows.
	// Defer the fixture gate again during the feed insert so PostgreSQL waits inside COMMIT.
	const commitGate = 890071
	storetest.Exec(t, observer, fmt.Sprintf(`
CREATE FUNCTION hold_machine_approval() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(%d::bigint);
    RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER hold_machine_approval AFTER INSERT ON feed
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
WHEN (NEW.topic = 'machines')
EXECUTE FUNCTION hold_machine_approval();

CREATE FUNCTION defer_machine_approval_gate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    SET CONSTRAINTS hold_machine_approval DEFERRED;
    RETURN NEW;
END;
$$;
CREATE TRIGGER defer_machine_approval_gate BEFORE INSERT ON feed
FOR EACH ROW WHEN (NEW.topic = 'machines')
EXECUTE FUNCTION defer_machine_approval_gate();`, commitGate))
	release := storetest.HoldAdvisoryGate(t, observer, commitGate)
	defer release()

	type approvalResult struct {
		client *webv1.Client
		err    error
	}
	approval := storetest.Start(t, func() approvalResult {
		reply, err := clients.ApproveClient(t.Context(), connect.NewRequest(&webv1.ApproveClientRequest{Id: id, Name: "Drain fixture"}))
		if err != nil {
			return approvalResult{err: err}
		}
		return approvalResult{client: reply.Msg.GetClient()}
	})

	blocker := int32(observer.PgConn().PID())
	var writer int32
	storetest.Poll(t, "the approval waiting inside COMMIT", func() bool {
		select {
		case <-approval.Done():
			t.Fatalf("Approval returned before reaching the commit gate: %v", approval.Result().err)
		default:
		}
		writer = storetest.Scalar[int32](t, observer, `
SELECT COALESCE((
    SELECT a.pid FROM pg_stat_activity a JOIN pg_locks l USING (pid)
    WHERE a.datname = current_database() AND a.state = 'active' AND upper(a.query) = 'COMMIT'
      AND l.locktype = 'advisory' AND NOT l.granted AND $1 = ANY(pg_blocking_pids(a.pid))
), 0)`, blocker)
		return writer != 0
	})
	order.writer.Store(uint32(writer))

	stop := storetest.Start(t, server.Close)
	storetest.Poll(t, "the application to stop accepting connections", func() bool {
		conn, err := net.DialTimeout("tcp", server.Addr().String(), 100*time.Millisecond)
		if err != nil {
			return true
		}
		_ = conn.Close()
		return false
	})
	blocked, err := storetest.IsBlocked(t.Context(), observer, writer, blocker, storetest.BigintKey(commitGate))
	if err != nil || !blocked {
		t.Errorf("The approval was no longer held at commit when the listener stopped: %v", err)
	}
	if !storetest.Scalar[bool](t, observer, `
SELECT EXISTS (
    SELECT FROM pg_locks l JOIN pg_stat_activity a USING (pid)
    WHERE a.datname = current_database() AND a.application_name = 'lgtm-server-instance'
      AND l.locktype = 'advisory' AND l.granted
)`) {
		t.Error("The application released its instance lock while the approval was held.")
	}
	release()

	if got := approval.Wait(t); got.err != nil || got.client.GetId() != id ||
		got.client.GetName() != "Drain fixture" || got.client.GetState() != webv1.ClientState_CLIENT_STATE_OFFLINE {
		t.Errorf("Shutdown did not let the in-flight approval succeed: %v", got.err)
	}
	if got := storetest.Scalar[string](t, observer, "SELECT name FROM machines WHERE id = $1", id); got != "Drain fixture" {
		t.Errorf("The approval left the stored name %q, want Drain fixture.", got)
	}
	var wireErr *connect.Error
	if err := watchEnd.Wait(t); !connect.IsWireError(err) || !errors.As(err, &wireErr) || wireErr.Code() != connect.CodeUnavailable {
		t.Errorf("The Machines watch ended with %v, want a server-sent unavailable error.", err)
	} else if wireErr.Message() == "The server cannot reach its database, try again." {
		t.Error("The Machines watch reported a database outage during an ordinary stop.")
	}
	if err := stop.Wait(t); err != nil {
		t.Errorf("Closing the application failed: %v", err)
	}
	if !storetest.Receive(t, "the feed listener state when approval committed", order.followingAtCommit) {
		t.Error("The follower closed before PostgreSQL acknowledged the approval commit.")
	}
	if !storetest.Receive(t, "the feed listener state at instance unlock", order.followerClosed) {
		t.Error("The instance unlock started before the follower closed.")
	}
	if !storetest.Receive(t, "the instance connection state at Store close", order.instanceClosed) {
		t.Error("The Store started closing pooled connections before the instance lock was released.")
	}
}

// machineShutdownOrder observes completion through the driver's connection lifecycle.
type machineShutdownOrder struct {
	writer            atomic.Uint32
	follower          atomic.Pointer[pgconn.PgConn]
	instance          atomic.Pointer[pgconn.PgConn]
	listening         chan struct{}
	followingAtCommit chan bool
	followerClosed    chan bool
	instanceClosed    chan bool
}

func (o *machineShutdownOrder) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "SELECT pg_advisory_unlock(") &&
		conn.Config().RuntimeParams["application_name"] == "lgtm-server-instance" &&
		o.instance.CompareAndSwap(nil, conn.PgConn()) {
		o.followerClosed <- shutdownConnectionClosed(o.follower.Load())
	}
	return ctx
}

func (o *machineShutdownOrder) TraceQueryEnd(_ context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		return
	}
	switch data.CommandTag.String() {
	case "LISTEN":
		if o.follower.CompareAndSwap(nil, conn.PgConn()) {
			o.listening <- struct{}{}
		}
	case "COMMIT":
		if conn.PgConn().PID() == o.writer.Load() {
			select {
			case o.followingAtCommit <- !shutdownConnectionClosed(o.follower.Load()):
			default:
			}
		}
	}
}

func (o *machineShutdownOrder) beforePoolClose(conn *pgx.Conn) {
	// A canceled query can leave a closed connection for the pool to discard.
	if conn.IsClosed() {
		return
	}
	select {
	case o.instanceClosed <- shutdownConnectionClosed(o.instance.Load()):
	default:
	}
}

func shutdownConnectionClosed(conn *pgconn.PgConn) bool {
	if conn == nil {
		return false
	}
	select {
	case <-conn.CleanupDone():
		return true
	default:
		return false
	}
}
