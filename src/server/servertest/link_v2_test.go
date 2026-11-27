package servertest_test

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
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func TestServerLinkRevisionTwoWiring(t *testing.T) {
	t.Run("PoolRestart", testFixtureLinkPoolRestart)
	t.Run("EventRollbackAndAcknowledgements", testFixtureLinkEvents)
	t.Run("ClaimedSend", testFixtureLinkClaimedSend)
}

const wiringPassword = "fixture wiring password"

type fixturePoolTrace struct {
	connected atomic.Bool
	signedIn  atomic.Bool
}

func (trace *fixturePoolTrace) TraceConnectStart(ctx context.Context, _ pgx.TraceConnectStartData) context.Context {
	return ctx
}

func (trace *fixturePoolTrace) TraceConnectEnd(_ context.Context, data pgx.TraceConnectEndData) {
	if data.Err == nil && data.Conn != nil {
		trace.connected.Store(true)
	}
}

func (trace *fixturePoolTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "INSERT INTO web_sessions") {
		trace.signedIn.Store(true)
	}
	return ctx
}

func (*fixturePoolTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func testFixtureLinkPoolRestart(t *testing.T) {
	var configs []*pgxpool.Config
	var traces []*fixturePoolTrace
	const marker = "fixture_link_wiring"
	dirty := false
	server := servertest.Start(t, servertest.Options{
		Password: wiringPassword,
		ConfigurePostgres: func(cfg *pgxpool.Config) {
			dirty = dirty || cfg.ConnConfig.RuntimeParams["application_name"] == marker || cfg.ConnConfig.Tracer != nil
			if cfg.ConnConfig.RuntimeParams == nil {
				cfg.ConnConfig.RuntimeParams = make(map[string]string)
			}
			trace := new(fixturePoolTrace)
			cfg.ConnConfig.RuntimeParams["application_name"] = marker
			cfg.ConnConfig.Tracer = trace
			configs = append(configs, cfg)
			traces = append(traces, trace)
		},
	})
	observer := fixtureLinkObserver(t, server)
	for launch := 1; launch <= 2; launch++ {
		if launch == 2 {
			server.Restart(t)
		}
		server.Clients(t)
		if got := storetest.Scalar[int](t, observer, "SELECT version FROM schema_migrations WHERE part = 'link'"); got != 3 {
			t.Fatalf("Launch %d has Link migration %d, want 3.", launch, got)
		}
		if len(configs) != launch || dirty || (launch == 2 && configs[0] == configs[1]) {
			t.Fatalf("Launch %d did not configure one fresh PostgreSQL pool configuration.", launch)
		}
		trace := traces[launch-1]
		if !trace.connected.Load() || !trace.signedIn.Load() {
			t.Fatalf("Launch %d did not use its configured PostgreSQL pool for sign-in.", launch)
		}

		// The paired folder call uses the signed-in client with a longer budget.
		client := *server.HTTPClient
		client.Timeout = 35 * time.Second
		ctx, cancel := context.WithTimeout(t.Context(), client.Timeout)
		session := lgtmv1connect.NewSessionServiceClient(&client, server.URL)
		res, err := session.GetServer(ctx, connect.NewRequest(&webv1.GetServerRequest{}))
		cancel()
		if err != nil || !res.Msg.GetSignedIn() {
			t.Fatalf("The longer-budget client lost its authenticated session on launch %d.", launch)
		}
	}
}

type fixtureWiringStream = connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]

func joinFixtureWiringMachine(t *testing.T, server *servertest.Server) (*machinetest.Machine, lgtmv1connect.ClientServiceClient) {
	t.Helper()
	clients := server.Clients(t)
	token, err := clients.CreateClientToken(t.Context(), connect.NewRequest(&webv1.CreateClientTokenRequest{}))
	if err != nil {
		t.Fatal("The fixture could not issue a join token.")
	}
	machine, err := machinetest.Join(t.Context(), server.Link(), token.Msg.GetToken(), "Link wiring machine")
	if err != nil || machine == nil {
		t.Fatal("The scripted machine could not join.")
	}
	return machine, clients
}

func openFixtureWiringStream(t *testing.T, server *servertest.Server, machine *machinetest.Machine, mark uint64) (*fixtureWiringStream, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	stream, welcome, err := machine.Connect(ctx, server.Link())
	if err != nil || stream == nil || welcome.GetState() == nil {
		cancel()
		t.Fatal("The scripted machine did not receive Welcome.")
	}
	closeStream := func() {
		cancel()
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	}
	t.Cleanup(closeStream)
	if welcome.GetLastEventNumber() != mark {
		t.Fatalf("Welcome reported mark %d, want %d.", welcome.GetLastEventNumber(), mark)
	}
	return stream, closeStream
}

func approveFixtureWiringMachine(t *testing.T, clients lgtmv1connect.ClientServiceClient, machineID string) {
	t.Helper()
	_, err := clients.ApproveClient(t.Context(), connect.NewRequest(&webv1.ApproveClientRequest{Id: machineID, Name: "Link wiring machine"}))
	if err != nil {
		t.Fatal("The fixture could not approve the scripted machine.")
	}
}

type fixtureEventPosition struct {
	machineID string
	number    uint64
}

type fixtureWiringReply struct {
	frame *linkv1.OpenResponse
	err   error
}

func readFixtureWiringReply(t *testing.T, stream *fixtureWiringStream) *storetest.Task[fixtureWiringReply] {
	t.Helper()
	return storetest.Start(t, func() fixtureWiringReply {
		frame, err := stream.Receive()
		return fixtureWiringReply{frame: frame, err: err}
	})
}

func requireFixtureWiringHold[V any](t *testing.T, reply *storetest.Task[fixtureWiringReply], phase string, held <-chan V) V {
	t.Helper()
	select {
	case value := <-held:
		return value
	case <-reply.Done():
		t.Fatalf("The stream answered before the %s callback held it.", phase)
	case <-time.After(storetest.ObservationTimeout):
		t.Fatalf("The fixture did not reach the %s callback.", phase)
	}
	var zero V
	return zero
}

func sendFixtureWiringEvent(t *testing.T, stream *fixtureWiringStream, event *linkv1.Event) {
	t.Helper()
	if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}}); err != nil {
		t.Fatal("The scripted machine could not send its event.")
	}
}

func requireFixtureWiringAck(t *testing.T, reply fixtureWiringReply, mark uint64) {
	t.Helper()
	ack := reply.frame.GetEventAck()
	if reply.err != nil || ack == nil || ack.GetThrough() != mark {
		t.Fatalf("The stream did not acknowledge through mark %d.", mark)
	}
}

func testFixtureLinkEvents(t *testing.T) {
	logs := new(fixtureLinkLog)
	committing := make(chan fixtureEventPosition, 1)
	acknowledging := make(chan fixtureEventPosition, 1)
	faultRelease, releaseFault := storetest.Signal(t)
	ackRelease, releaseAck := storetest.Signal(t)
	consumedRelease, releaseConsumed := storetest.Signal(t)
	defer releaseFault()
	defer releaseAck()
	defer releaseConsumed()
	var fault atomic.Bool
	var commitCalls atomic.Int32
	fault.Store(true)
	server := servertest.Start(t, servertest.Options{
		Password: wiringPassword,
		Log:      logs,
		LinkSeams: servertest.LinkSeams{
			BeforeEventCommit: func(ctx context.Context, id string, number uint64) error {
				commitCalls.Add(1)
				if !fault.Load() {
					return nil
				}
				select {
				case committing <- fixtureEventPosition{id, number}:
				case <-ctx.Done():
					return context.Cause(ctx)
				}
				select {
				case <-faultRelease:
					return errors.New("The fixture interrupted the event write.")
				case <-ctx.Done():
					return context.Cause(ctx)
				}
			},
			BeforeEventAck: func(ctx context.Context, id string, number uint64) {
				if fault.Load() {
					return
				}
				select {
				case acknowledging <- fixtureEventPosition{id, number}:
				case <-ctx.Done():
					return
				}
				release := ackRelease
				if number == 2 {
					release = consumedRelease
				}
				select {
				case <-release:
				case <-ctx.Done():
				}
			},
		},
	})
	machine, clients := joinFixtureWiringMachine(t, server)
	observer := fixtureLinkObserver(t, server)

	// Sequence increments survive rollback and reveal which writes started before the fault.
	storetest.Exec(t, observer, `CREATE SEQUENCE fixture_facts_written;
		CREATE SEQUENCE fixture_mark_written;
		CREATE FUNCTION fixture_event_write_phase() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM nextval(TG_ARGV[0]::regclass);
			RETURN NEW;
		END $$;
		CREATE TRIGGER fixture_facts_written AFTER UPDATE OF facts ON machines
			FOR EACH ROW EXECUTE FUNCTION fixture_event_write_phase('fixture_facts_written');
		CREATE TRIGGER fixture_mark_written AFTER UPDATE OF event_mark ON link_machines
			FOR EACH ROW EXECUTE FUNCTION fixture_event_write_phase('fixture_mark_written')`)

	stream, closeStream := openFixtureWiringStream(t, server, machine, 0)
	facts := &linkv1.Facts{Os: "Fixture wiring report", Cores: 1}
	event := &linkv1.Event{Number: 1, Report: &linkv1.Event_Facts{Facts: facts}}
	sendFixtureWiringEvent(t, stream, event)
	failed := readFixtureWiringReply(t, stream)
	position := requireFixtureWiringHold(t, failed, "pre-commit event", committing)
	if position != (fixtureEventPosition{machine.ID, 1}) {
		t.Fatal("The pre-commit callback received the wrong machine or event.")
	}
	if !storetest.Scalar[bool](t, observer, "SELECT is_called FROM fixture_facts_written") ||
		storetest.Scalar[bool](t, observer, "SELECT is_called FROM fixture_mark_written") {
		t.Fatal("The pre-commit callback did not run after the Facts write and before the mark write.")
	}
	releaseFault()
	reply := failed.Wait(t)
	if reply.frame != nil || connect.CodeOf(reply.err) != connect.CodeUnavailable {
		t.Fatal("The faulted event did not end the stream with unavailable and no acknowledgement.")
	}
	const readMark = "SELECT event_mark FROM link_machines WHERE machine_id = $1"
	const readFacts = "SELECT facts FROM machines WHERE id = $1"
	if storetest.Scalar[int64](t, observer, readMark, machine.ID) != 0 ||
		len(storetest.Scalar[[]byte](t, observer, readFacts, machine.ID)) != 0 {
		t.Fatal("The faulted event left committed Facts or advanced the mark.")
	}

	closeStream()
	server.Restart(t)
	fault.Store(false)
	stream, closeStream = openFixtureWiringStream(t, server, machine, 0)
	sendFixtureWiringEvent(t, stream, event)
	applied := readFixtureWiringReply(t, stream)
	position = requireFixtureWiringHold(t, applied, "committed Facts acknowledgement", acknowledging)
	if commitCalls.Load() != 2 {
		t.Fatal("The pre-commit callback did not run for the retried event after restart.")
	}
	var saved linkv1.Facts
	raw := storetest.Scalar[[]byte](t, observer, readFacts, machine.ID)
	if position != (fixtureEventPosition{machine.ID, 1}) ||
		storetest.Scalar[int64](t, observer, readMark, machine.ID) != 1 ||
		proto.Unmarshal(raw, &saved) != nil || !proto.Equal(&saved, facts) {
		t.Fatal("The acknowledgement callback did not observe the committed Facts and mark.")
	}
	releaseAck()
	requireFixtureWiringAck(t, applied.Wait(t), 1)

	sendFixtureWiringEvent(t, stream, event)
	repeated := readFixtureWiringReply(t, stream)
	requireFixtureWiringAck(t, repeated.Wait(t), 1)
	if commitCalls.Load() != 2 {
		t.Fatal("A repeat invoked the pre-commit callback.")
	}
	select {
	case <-acknowledging:
		t.Fatal("A repeat invoked the committed-advance acknowledgement callback.")
	default:
	}

	// The machine still waits for approval when its valid inventory report is consumed.
	inventory := &linkv1.HarnessesFound{Harnesses: []*linkv1.Harness{{Name: "codex"}}}
	sendFixtureWiringEvent(t, stream, &linkv1.Event{
		Number: 2, Report: &linkv1.Event_HarnessesFound{HarnessesFound: inventory},
	})
	consumed := readFixtureWiringReply(t, stream)
	position = requireFixtureWiringHold(t, consumed, "consumed inventory acknowledgement", acknowledging)
	if position != (fixtureEventPosition{machine.ID, 2}) ||
		storetest.Scalar[int64](t, observer, readMark, machine.ID) != 2 ||
		storetest.Scalar[bool](t, observer, "SELECT harnesses_scanned FROM machines WHERE id = $1", machine.ID) {
		t.Fatal("The consumed event did not commit its mark without changing inventory.")
	}
	approveFixtureWiringMachine(t, clients, machine.ID)
	releaseConsumed()
	requireFixtureWiringAck(t, consumed.Wait(t), 2)
	standing := readFixtureWiringReply(t, stream).Wait(t)
	if standing.err != nil || standing.frame.GetCommand().GetSetState().GetState().GetApproval() != linkv1.Approval_APPROVAL_APPROVED {
		t.Fatal("The consumed event acknowledgement did not precede the standing command.")
	}

	closeStream()
	openFixtureWiringStream(t, server, machine, 2)
	var applications, advances int
	for _, entry := range logs.entries() {
		if entry.Message == "link_event_applied" && entry.Machine == machine.ID {
			if entry.Sequence == 1 {
				applications++
			}
			if entry.Sequence == 2 {
				advances++
			}
		}
	}
	if applications != 1 || advances != 1 {
		t.Fatal("The application log did not record one entry per committed advance.")
	}
	if storetest.Scalar[int64](t, observer, "SELECT last_value FROM fixture_facts_written") != 2 {
		t.Fatal("The Facts write ran again when the machine repeated its acknowledged event.")
	}
}

func testFixtureLinkClaimedSend(t *testing.T) {
	type claim struct {
		machineID string
		commandID string
	}
	claims := make(chan claim, 1)
	sendRelease, releaseSend := storetest.Signal(t)
	defer releaseSend()
	server := servertest.Start(t, servertest.Options{
		Password: wiringPassword,
		LinkSeams: servertest.LinkSeams{
			BeforeCommandSend: func(ctx context.Context, machineID, commandID string) error {
				select {
				case claims <- claim{machineID, commandID}:
				case <-ctx.Done():
					return context.Cause(ctx)
				}
				select {
				case <-sendRelease:
					return nil
				case <-ctx.Done():
					return context.Cause(ctx)
				}
			},
		},
	})
	machine, clients := joinFixtureWiringMachine(t, server)
	observer := fixtureLinkObserver(t, server)
	stream, _ := openFixtureWiringStream(t, server, machine, 0)
	delivered := readFixtureWiringReply(t, stream)
	approveFixtureWiringMachine(t, clients, machine.ID)
	held := requireFixtureWiringHold(t, delivered, "claimed command send", claims)

	var attempt int64
	var current, dispatched bool
	err := observer.QueryRow(t.Context(), `
		SELECT cmd.attempt, cmd.sent_stream_id = conn.stream_id, cmd.dispatched
		FROM link_commands cmd JOIN link_connections conn USING (machine_id)
		WHERE cmd.id = $1 AND cmd.machine_id = $2 AND cmd.action = 'set_state'
		FOR UPDATE OF cmd NOWAIT`, held.commandID, machine.ID).Scan(&attempt, &current, &dispatched)
	if held.machineID != machine.ID || err != nil || attempt != 1 || !current || dispatched {
		t.Fatal("The send callback did not hold a committed, unlocked claim before dispatch.")
	}
	select {
	case <-delivered.Done():
		t.Fatal("The stream received a frame while the send callback held the command.")
	default:
	}

	releaseSend()
	reply := delivered.Wait(t)
	command := reply.frame.GetCommand()
	if reply.err != nil || command.GetId() != held.commandID || int64(command.GetAttempt()) != attempt ||
		command.GetSetState().GetState().GetApproval() != linkv1.Approval_APPROVAL_APPROVED {
		t.Fatal("The stream did not receive the claimed standing command after release.")
	}
	storetest.Poll(t, "the committed dispatch marker", func() bool {
		return storetest.Scalar[bool](t, observer, `
			SELECT dispatched FROM link_commands WHERE id = $1 AND machine_id = $2 AND attempt = $3`,
			held.commandID, machine.ID, attempt)
	})
}
