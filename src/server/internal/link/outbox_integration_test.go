package link

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestCommandSendAndSweep(t *testing.T) {
	t.Run("sweep wins before the send fence", testSweepBeforeCommandSend)
	t.Run("fenced flush defers only its machine", testSweepDuringCommandFlush)
	t.Run("unaged replay preserves the original expiry", testUnagedCommandReplay)
	t.Run("reported expiry wins before sweep", func(t *testing.T) { testReportedExpiryAndSweep(t, true) })
	t.Run("timeout wins before reported expiry", func(t *testing.T) { testReportedExpiryAndSweep(t, false) })
}

func integrationCommandClient(t *testing.T, address string) linkv1connect.LinkServiceClient {
	t.Helper()
	// The response writer probe recognizes uncompressed envelopes.
	return linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), address, connect.WithAcceptCompression("gzip", nil, nil))
}

func testSweepBeforeCommandSend(t *testing.T) {
	trace := newIntegrationSweepTrace()
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: trace.configure})
	m, key := f.register(t, "swept before send")
	f.approve(t, m.ID)
	row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	probe := newCommandWireProbe()
	claimed := make(chan *linkv1.Command, 1)
	held, release := storetest.Signal(t)
	defer release()
	probe.afterClaim = func(ctx context.Context, cmd *linkv1.Command) error {
		claimed <- cmd
		select {
		case <-held:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	server := commandListener(t, f, map[string]*commandWireProbe{"held claim": probe})
	stream := openCommandStream(t, f, integrationCommandClient(t, server.URL), m, key, "held claim")
	row.requireFrame(t, storetest.Receive(t, "the durable claim before the send fence", claimed), 1)
	requireDispatch(t, f.presenceFixture, row, f.current(t, m.ID).StreamID, 1, false)

	ageIntegrationCommand(t, f.presenceFixture, m.ID, row.id)
	start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	if err := trace.sweep(t.Context(), f.state); err != nil {
		t.Fatal("The sweep could not finish the held command.")
	}
	if got := storetest.Receive(t, "the sweep selecting the aged command", trace.dueRows); got != 1 {
		t.Fatal("The sweep did not select exactly the one aged claim.")
	}
	finished := requireFinishedResult(t, f.presenceFixture, row.id, "timed_out", start)
	release()

	sendHandshake(t, stream, heartbeatFrame(nil, nil))
	frame, err := stream.Receive()
	if err != nil || frame.GetHeartbeatAck() == nil {
		t.Fatal("The swept claim emitted a command frame or prevented a heartbeat on the live stream.")
	}
	select {
	case <-probe.flushing:
		t.Fatal("The swept claim still reached the transport flush.")
	default:
	}
	requireTerminalCommand(t, f.presenceFixture, row.id, "timed_out", finished, finished)
}

func ageIntegrationCommand(t *testing.T, f *presenceFixture, machine, command string) {
	t.Helper()
	err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.r.Lock(ctx, tx, machine); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE link_commands SET created_at = clock_timestamp() - interval '26 hours',
			expires_at = clock_timestamp() - interval '25 hours' WHERE id = $1 AND machine_id = $2 AND outcome IS NULL`,
			command, machine)
		if err == nil && tag.RowsAffected() != 1 {
			return errors.New("The command could not be aged while pending.")
		}
		return err
	})
	if err != nil {
		t.Fatal("The fixture could not age its pending command under the registry lock.")
	}
}

type integrationSweepKey struct{}
type integrationSweepQueryKey struct{}

type integrationSweepTrace struct {
	dueRows   chan int64
	lockCodes chan string
}

func newIntegrationSweepTrace() *integrationSweepTrace {
	return &integrationSweepTrace{dueRows: make(chan int64, 4), lockCodes: make(chan string, 8)}
}

func (trace *integrationSweepTrace) configure(cfg *pgxpool.Config) {
	cfg.ConnConfig.Tracer = trace
}

func (*integrationSweepTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if ctx.Value(integrationSweepKey{}) != true {
		return ctx
	}
	sql := strings.ToLower(query.SQL)
	switch {
	case strings.Contains(sql, "select expires_at, machine_id, seq") && strings.Contains(sql, "from link_commands"):
		return context.WithValue(ctx, integrationSweepQueryKey{}, "due")
	case strings.Contains(sql, "for no key update") && strings.Contains(sql, "from machines"):
		return context.WithValue(ctx, integrationSweepQueryKey{}, "registry lock")
	default:
		return ctx
	}
}

func (trace *integrationSweepTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryEndData) {
	switch ctx.Value(integrationSweepQueryKey{}) {
	case "due":
		trace.dueRows <- query.CommandTag.RowsAffected()
	case "registry lock":
		var err *pgconn.PgError
		code := ""
		if errors.As(query.Err, &err) {
			code = err.Code
		}
		trace.lockCodes <- code
	}
}

func (*integrationSweepTrace) sweep(ctx context.Context, state *State) error {
	return state.SweepCommands(context.WithValue(ctx, integrationSweepKey{}, true))
}

func testSweepDuringCommandFlush(t *testing.T) {
	trace := newIntegrationSweepTrace()
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: trace.configure})
	m, key := f.register(t, "held flush")
	control, _ := f.register(t, "control sweep")
	f.approve(t, m.ID)
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.r.SetApproved(ctx, tx, control.ID, "Control machine")
	}); err != nil {
		t.Fatal("The fixture could not approve an independent control machine.")
	}
	row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	due := seedClaimRow(t, f.presenceFixture, m.ID, restartAction(), now.Add(-26*time.Hour))
	controlDue := seedClaimRow(t, f.presenceFixture, control.ID, restartAction(), now.Add(-25*time.Hour))
	probe := newCommandWireProbe()
	server := commandListener(t, f, map[string]*commandWireProbe{"held flush": probe})
	peer := openCommandPeer(t, f, server, m, key, "held flush")
	flush := watchCommandFlush(t, probe)
	row.requireFrame(t, flush.frame, 1)

	// The raw peer has granted no command credit. The sweep must finish before the flush can.
	ctx, cancel := context.WithDeadline(t.Context(), flush.deadline)
	defer cancel()
	if trace.sweep(ctx, f.state) != nil {
		t.Fatal("The sweep did not finish while the command flush held the machine lock.")
	}
	if rows := storetest.Receive(t, "both due commands in the same sweep page", trace.dueRows); rows != 2 {
		t.Fatal("The sweep did not select the two due commands while the flush was held.")
	}
	select {
	case code := <-trace.lockCodes:
		if code != "55P03" {
			t.Fatal("PostgreSQL did not refuse the sweep's nonwaiting machine lock during the flush.")
		}
	default:
		t.Fatal("The sweep attempted no nonwaiting machine lock during the flush.")
	}
	select {
	case code := <-trace.lockCodes:
		if code != "" {
			t.Fatal("PostgreSQL did not grant the independent control machine's lock.")
		}
	default:
		t.Fatal("The sweep did not proceed to the control machine's lock.")
	}
	select {
	case <-flush.done:
		t.Fatal("The command flush returned before the sweep proved progress on the control machine.")
	default:
	}
	requirePendingResult(t, f.presenceFixture, due.id)
	requireFinishedResult(t, f.presenceFixture, controlDue.id, "timed_out", now)

	row.requireFrame(t, peer.readEnvelope(t).GetCommand(), 1)
	if storetest.Receive(t, "the credited command's real flush", flush.done) != nil {
		t.Fatal("Granting credit did not complete the held command flush.")
	}
	streamID := f.current(t, m.ID).StreamID
	waitIntegrationDispatch(t, f.presenceFixture, row, streamID, 1)
	if f.state.SweepCommands(t.Context()) != nil {
		t.Fatal("The sweep failed after the command released the machine lock.")
	}
	requireFinishedResult(t, f.presenceFixture, due.id, "timed_out", now)
	requireDispatch(t, f.presenceFixture, row, streamID, 1, true)
}

func waitIntegrationDispatch(t *testing.T, f *presenceFixture, row claimFixtureRow, streamID string, attempt int) {
	t.Helper()
	storetest.Poll(t, "the flushed command's committed dispatch marker", func() bool {
		return storetest.Scalar[bool](t, f.obs, `SELECT dispatched AND sent_stream_id = $2 AND attempt = $3
			FROM link_commands WHERE id = $1`, row.id, streamID, attempt)
	})
	requireDispatch(t, f, row, streamID, attempt, true)
}

func receiveIntegrationCommand(t *testing.T, stream *openClientStream, row claimFixtureRow, attempt uint32) {
	t.Helper()
	frame, err := stream.Receive()
	if err != nil || frame.GetCommand() == nil {
		t.Fatal("The live machine did not receive the queued command.")
	}
	row.requireFrame(t, frame.GetCommand(), attempt)
}

func testUnagedCommandReplay(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "unaged restart replay")
	f.approve(t, m.ID)
	row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	server := commandListener(t, f, nil)
	f.startRunner(t)
	client := integrationCommandClient(t, server.URL)
	open, cancel := openCancellableCommandStream(t, f, client, m, key, "")
	receiveIntegrationCommand(t, open, row, 1)
	firstID := f.current(t, m.ID).StreamID
	waitIntegrationDispatch(t, f.presenceFixture, row, firstID, 1)

	cancel()
	_ = open.CloseRequest()
	_ = open.CloseResponse()
	storetest.Poll(t, "the canceled stream's committed disconnect", func() bool {
		return f.connectionCount(t, m.ID) == 0
	})
	retry := openCommandStream(t, f, client, m, key, "")
	receiveIntegrationCommand(t, retry, row, 2)
	nextID := f.current(t, m.ID).StreamID
	if firstID == nextID || !storetest.Scalar[bool](t, f.obs, "SELECT expires_at = $2 FROM link_commands WHERE id = $1", row.id, row.expires) {
		t.Fatal("Replaying on a fresh stream did not preserve the stored expiry.")
	}
	waitIntegrationDispatch(t, f.presenceFixture, row, nextID, 2)
}

func testReportedExpiryAndSweep(t *testing.T, resultFirst bool) {
	t.Helper()
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "expiry report")
	f.approve(t, m.ID)
	row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	probe := newCommandWireProbe()
	server := commandListener(t, f, map[string]*commandWireProbe{"expiry": probe})
	open := openCommandStream(t, f, integrationCommandClient(t, server.URL), m, key, "expiry")
	receiveIntegrationCommand(t, open, row, 1)
	if storetest.Receive(t, "the expiry fixture's real flush", watchCommandFlush(t, probe).done) != nil {
		t.Fatal("The expiry fixture did not flush its attempt.")
	}
	waitIntegrationDispatch(t, f.presenceFixture, row, f.current(t, m.ID).StreamID, 1)
	ageIntegrationCommand(t, f.presenceFixture, m.ID, row.id)
	start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")

	report := func() {
		sendHandshake(t, open, integrationResultFrame(row.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED))
		sendHandshake(t, open, heartbeatFrame(nil, nil))
		requireHeartbeatAck(t, open)
	}
	sweep := func() {
		if f.state.SweepCommands(t.Context()) != nil {
			t.Fatal("The terminal ordering sweep could not finish.")
		}
	}
	if resultFirst {
		report()
		finished := requireFinishedResult(t, f.presenceFixture, row.id, "expired", start)
		sweep()
		requireTerminalCommand(t, f.presenceFixture, row.id, "expired", finished, finished)
	} else {
		sweep()
		finished := requireFinishedResult(t, f.presenceFixture, row.id, "timed_out", start)
		report()
		requireTerminalCommand(t, f.presenceFixture, row.id, "timed_out", finished, finished)
	}
}

func integrationResultFrame(id string, attempt uint32, outcome linkv1.CommandOutcome) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: commandResult(id, attempt, outcome)}}
}

type integrationCommitGate struct {
	writer  atomic.Uint32
	pid     chan int32
	wait    <-chan struct{}
	release func()
}

func newIntegrationCommitGate(t *testing.T) *integrationCommitGate {
	t.Helper()
	wait, release := storetest.Signal(t)
	return &integrationCommitGate{pid: make(chan int32, 1), wait: wait, release: release}
}

func TestCommandResultAndRemoval(t *testing.T) {
	for _, tc := range []struct {
		name        string
		resultFirst bool
	}{
		{"result commits first", true},
		{"removal commits first", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resultCommit, removalCommit := newIntegrationCommitGate(t), newIntegrationCommitGate(t)
			defer resultCommit.release()
			defer removalCommit.release()
			f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
				cfg.ConnConfig.Tracer = integrationRemovalTrace(resultCommit, removalCommit)
			}})
			if f.s.Migrate(t.Context(), machines.Migrations(), Migrations(), auth.Migrations()) != nil {
				t.Fatal("The removal ordering fixture could not migrate owner sessions.")
			}
			route := integrationRemovalRoutes(t, f)
			m, key := f.register(t, "result and removal")
			f.approve(t, m.ID)
			row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
			server := commandListener(t, f, nil)
			f.startRunner(t)
			client := integrationCommandClient(t, server.URL)
			open := openCommandStream(t, f, client, m, key, "")
			receiveIntegrationCommand(t, open, row, 1)
			streamID := f.current(t, m.ID).StreamID
			waitIntegrationDispatch(t, f.presenceFixture, row, streamID, 1)

			ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
			defer cancel()
			watchReq := removalRequest(route, &webv1.WatchClientsRequest{})
			watchReq.Header().Del(store.RequestKeyHeader)
			watch, err := route.clients.WatchClients(ctx, watchReq)
			if err != nil {
				t.Fatal("The signed-in removal watch could not start.")
			}
			defer watch.Close()
			if !watch.Receive() || !integrationWatchContains(watch.Msg(), m.ID) {
				t.Fatal("The removal watch did not initially contain the machine.")
			}
			remove := func() error {
				_, err := route.clients.RemoveClient(ctx, removalRequest(route, &webv1.RemoveClientRequest{Id: m.ID}))
				return err
			}
			result := func() error {
				return f.state.AcceptResult(ctx, m.ID, streamID, commandResult(row.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE))
			}
			start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
			var firstFinish time.Time
			want := "removed"
			if tc.resultFirst {
				want = "done"
				sendHandshake(t, open, integrationResultFrame(row.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE))
				blocker := storetest.Receive(t, "the wire result holding its commit", resultCommit.pid)
				removing := storetest.Start(t, remove)
				waitIntegrationRemovalOrder(t, f.presenceFixture, blocker)
				requirePendingResult(t, f.presenceFixture, row.id)
				resultCommit.release()
				storetest.Receive(t, "the owner removal holding its commit", removalCommit.pid)
				firstFinish = requireFinishedResult(t, f.presenceFixture, row.id, want, start)
				removalCommit.release()
				if removing.Wait(t) != nil {
					t.Fatal("Owner removal did not commit after the result won.")
				}
			} else {
				removing := storetest.Start(t, remove)
				blocker := storetest.Receive(t, "the owner removal holding its commit", removalCommit.pid)
				completing := storetest.Start(t, result)
				waitIntegrationRemovalOrder(t, f.presenceFixture, blocker)
				requirePendingResult(t, f.presenceFixture, row.id)
				removalCommit.release()
				if removing.Wait(t) != nil {
					t.Fatal("The owner removal did not commit first.")
				}
				if !errors.Is(completing.Wait(t), ErrUnknownMachine) {
					t.Fatal("The waiting result was not refused after the removal committed first.")
				}
				firstFinish = requireFinishedResult(t, f.presenceFixture, row.id, want, start)
			}

			for watch.Receive() && watch.Msg().GetRemovedId() != m.ID {
			}
			if watch.Msg().GetRemovedId() != m.ID {
				t.Fatal("The signed-in watch did not remove the machine after the owner committed.")
			}
			requireIntegrationRemovedStream(t, open)
			after := client.Open(ctx)
			defer after.CloseResponse()
			defer after.CloseRequest()
			submitProof(t, f, after, m.ID, key, 2)
			requireIntegrationRemovedStream(t, after)
			if !errors.Is(result(), ErrUnknownMachine) {
				t.Fatal("A late result could still reach the removed machine's command.")
			}
			restartReq := removalRequest(route, &webv1.ReconnectClientRequest{Id: m.ID})
			restartReq.Header().Del(store.RequestKeyHeader)
			_, err = route.clients.ReconnectClient(ctx, restartReq)
			if connect.CodeOf(err) != connect.CodeNotFound {
				t.Fatal("The owner could still queue a restart for the removed machine.")
			}
			requireTerminalCommand(t, f.presenceFixture, row.id, want, firstFinish, firstFinish)
			if storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_commands WHERE machine_id = $1", m.ID) != 1 {
				t.Fatal("The removed machine acquired another command.")
			}
		})
	}
}

func integrationRemovalRoutes(t *testing.T, f *handshakeFixture) removalFixture {
	t.Helper()
	password, err := auth.NewPassword("command result removal fixture password")
	if err != nil {
		t.Fatal("The removal ordering fixture could not initialize owner sessions.")
	}
	sessions := auth.NewSessions(f.s, password)
	token, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatal("The removal ordering fixture could not sign in.")
	}
	origin, err := auth.NewOriginPolicy("http://127.0.0.1:8787", nil)
	if err != nil {
		t.Fatal("The removal ordering fixture could not set its loopback origin.")
	}
	clients := machines.NewService(f.s, sessions, f.r, f.tokens).WithLink(f.state).WithCommands(f.state)
	server := httptest.NewServer(api.NewHandler(api.Options{Sessions: sessions, Origins: origin, Clients: clients}))
	t.Cleanup(server.Close)
	return removalFixture{
		clients: lgtmv1connect.NewClientServiceClient(server.Client(), server.URL),
		cookie:  (&http.Cookie{Name: auth.CookieName, Value: token}).String(),
	}
}

func integrationRemovalTrace(result, removal *integrationCommitGate) handshakeQueryTrace {
	return func(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
		sql := strings.ToLower(strings.TrimSpace(query.SQL))
		pid := conn.PgConn().PID()
		switch {
		case strings.HasPrefix(sql, "update link_commands") && strings.Contains(sql, "set outcome =") && strings.Contains(sql, "sent_stream_id = $3"):
			result.writer.Store(pid)
		case strings.HasPrefix(sql, "delete from machines where id ="):
			removal.writer.Store(pid)
		case sql == "commit":
			for _, gate := range []*integrationCommitGate{result, removal} {
				if gate.writer.CompareAndSwap(pid, 0) {
					gate.pid <- int32(pid)
					select {
					case <-gate.wait:
					case <-ctx.Done():
					}
				}
			}
		}
		return ctx
	}
}

func waitIntegrationRemovalOrder(t *testing.T, f *presenceFixture, blocker int32) {
	t.Helper()
	storetest.Poll(t, "the competing writer waiting for the first machine transaction", func() bool {
		return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
			SELECT FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'
			AND (lower(query) LIKE '%for no key update%' OR lower(query) LIKE 'delete from machines%')
			AND $1 = ANY (pg_blocking_pids(pid)))`, blocker)
	})
}

func integrationWatchContains(frame *webv1.WatchClientsResponse, id string) bool {
	for _, m := range frame.GetAll().GetClients() {
		if m.GetId() == id {
			return true
		}
	}
	return false
}

func requireIntegrationRemovedStream(t *testing.T, open *openClientStream) {
	t.Helper()
	frame, err := open.Receive()
	var refusal *connect.Error
	if frame != nil || !errors.As(err, &refusal) || refusal.Code() != connect.CodeFailedPrecondition ||
		refusal.Meta().Get("Lgtm-Link-Refusal") != "unknown_machine" {
		t.Fatal("The removed machine did not receive its terminal refusal without another frame.")
	}
}
