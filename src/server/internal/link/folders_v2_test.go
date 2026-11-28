package link

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	folderInnerBudget = 49152
	folderOuterBudget = 65536
	folderPathBudget  = 4096
	folderCredit      = 32
	folderPath        = "/workspace/repository"
)

func TestFolderReportPrecedesSettlement(t *testing.T) {
	t.Run("ReplayRollsBackThenCommitsInOrder", testFolderReplaySettlement)
	t.Run("WaitingThroughApproval", testFolderWaitingReplay)
	t.Run("SettledAndMismatchedReports", testFolderReportIdentity)
	for _, sample := range []struct {
		name                 string
		removal, reportFirst bool
	}{
		{"ReportBeforeTimeout", false, true},
		{"TimeoutBeforeReport", false, false},
		{"ReportBeforeRemoval", true, true},
		{"RemovalBeforeReport", true, false},
	} {
		t.Run(sample.name, func(t *testing.T) { testFolderTerminalRace(t, sample.removal, sample.reportFirst) })
	}
}

func testFolderReplaySettlement(t *testing.T) {
	var fault atomic.Bool
	var reports, reportsAtFault atomic.Int32
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventCommit: func(_ context.Context, _ string, number uint64) error {
			if number == 1 && fault.Load() {
				reportsAtFault.Store(reports.Load())
				return errors.New("The fixture interrupted the final replay write.")
			}
			return nil
		},
	}})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "folder replay")
	f.approve(t, m.ID)
	runA, runB, operation := fixtureStreamID(t), fixtureStreamID(t), fixtureStreamID(t)
	command := seedFolderCommand(t, f, m.ID, operation)
	storetest.Exec(t, f.obs, `CREATE TABLE fixture_folder_replay (
		seq bigint GENERATED ALWAYS AS IDENTITY, effect text, visible_mark bigint)`)
	registerFolderPart(t, f, PartHooks{
		ReportApplied: map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
			ReportFolderSeen: func(ctx context.Context, tx *store.Tx, machine string, event *linkv1.Event) error {
				report := event.GetFolderSeen()
				if machine != m.ID || event.GetNumber() != 1 || report.GetOperationId() != operation ||
					report.GetCommandId() != command.id || report.GetFacts().GetResolvedPath() != folderPath {
					return errors.New("The folder report hook received the wrong identity or facts.")
				}
				reports.Add(1)
				if _, err := tx.Exec(ctx, `INSERT INTO fixture_folder_replay (effect, visible_mark)
					SELECT 'report', event_mark FROM link_machines WHERE machine_id = $1`, machine); err != nil {
					return err
				}
				return tx.Record(store.ChangeRef{Topic: "fixture_folders", Key: operation, Change: store.Saved})
			},
		},
		CommandSettled: map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
			CommandLookAtFolder: func(ctx context.Context, tx *store.Tx, event Settlement) error {
				if event.MachineID != m.ID || event.Command.GetId() != command.id ||
					event.Command.GetLookAtFolder().GetOperationId() != operation || event.Outcome != OutcomeDone ||
					event.LastEventNumber == nil || *event.LastEventNumber != 1 {
					return errors.New("The folder settlement hook received the wrong command or result.")
				}
				_, err := tx.Exec(ctx, `INSERT INTO fixture_folder_replay (effect, visible_mark)
					SELECT CASE WHEN EXISTS (SELECT FROM fixture_folder_replay WHERE effect = 'report')
						AND ready_run_id = $2 THEN 'settlement' ELSE 'settlement out of order' END, event_mark
					FROM link_machines WHERE machine_id = $1`, m.ID, runA)
				return err
			},
		},
		RunChanged: func(ctx context.Context, tx *store.Tx, change RunChange) error {
			if change.RunID != runB {
				return nil
			}
			if change.MachineID != m.ID || change.PreviousRunID != runA || change.FirstEventNumber != 2 {
				return errors.New("The ready hook received the wrong run transition.")
			}
			_, err := tx.Exec(ctx, `INSERT INTO fixture_folder_replay (effect, visible_mark)
				SELECT CASE WHEN EXISTS (SELECT FROM fixture_folder_replay WHERE effect = 'settlement')
					AND ready_run_id = $2 THEN 'ready' ELSE 'ready out of order' END, event_mark
				FROM link_machines WHERE machine_id = $1`, m.ID, runB)
			return err
		},
	})

	wire := steadyWire(t, f, streamTestOptions{})
	original := submitAdmission(t, f, wire, m.ID, key, runA, 1)
	requireAdmissionWelcome(t, original, 0)
	requireFolderDelivery(t, original, command, 1)
	sendFolderResult(t, original, command, 1, 1)
	sendHandshake(t, original, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, original)
	if !hasSettlementStage(t, f.presenceFixture, command.id, "done", 1) {
		t.Fatal("The original stream did not stage the folder result.")
	}

	pending := submitAdmission(t, f, wire, m.ID, key, runB, 2)
	requireAdmissionWelcome(t, pending, 0)
	requireAdmissionRun(t, f, m.ID, runB, runA, 2)
	head := f.head(t)
	report := folderFactsEvent(1, operation, command.id)
	fault.Store(true)
	sendHandshake(t, pending, eventFrame(report))
	requireOpenEnd(t, pending, connect.CodeUnavailable, openDatabase, "")
	if reportsAtFault.Load() != 1 {
		t.Error("The pre-commit fault did not run after the folder callback.")
	}
	requireFolderMark(t, f, m.ID, 0)
	requireAdmissionRun(t, f, m.ID, runB, runA, 2)
	if !hasSettlementStage(t, f.presenceFixture, command.id, "done", 1) ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM fixture_folder_replay") != 0 ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM feed WHERE position > $1 AND topic = 'fixture_folders'", head) != 0 {
		t.Error("The failed replay changed the staged result, part records, or feed.")
	}

	fault.Store(false)
	retry := submitAdmission(t, f, wire, m.ID, key, runB, 2)
	requireAdmissionWelcome(t, retry, 0)
	sendHandshake(t, retry, eventFrame(report))
	requireEventAck(t, retry, 1)
	requireFolderMark(t, f, m.ID, 1)
	requireAdmissionRun(t, f, m.ID, runB, runB, 2)
	requireSettlementTerminal(t, f.presenceFixture, command.id, "done")
	got := storetest.Rows[string](t, f.obs, "SELECT effect || ':' || visible_mark FROM fixture_folder_replay ORDER BY seq")
	if !slices.Equal(got, []string{"report:0", "settlement:1", "ready:1"}) ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(DISTINCT xmin::text) FROM fixture_folder_replay") != 1 {
		t.Error("Report, mark, settlement and readiness were not committed in the required order and write.")
	}
	if storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM feed WHERE position > $1 AND topic = 'fixture_folders'", head) != 1 {
		t.Error("Retry did not commit exactly one part feed reference.")
	}
	atRetry := reports.Load()
	sendHandshake(t, retry, eventFrame(report))
	requireEventAck(t, retry, 1)
	if atRetry != 2 || reports.Load() != atRetry {
		t.Error("The replay repeat ran the folder callback again.")
	}
}

func testFolderWaitingReplay(t *testing.T) {
	held := make(chan uint64, 1)
	released, release := storetest.Signal(t)
	defer release()
	var calls atomic.Int32
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventAck: func(ctx context.Context, _ string, mark uint64) {
			if mark == 2 {
				held <- mark
				select {
				case <-released:
				case <-ctx.Done():
				}
			}
		},
	}})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "waiting folder report")
	operation, command := fixtureStreamID(t), fixtureStreamID(t)
	registerFolderCounter(t, f, &calls)
	peer := submitAdmission(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, fixtureStreamID(t), 3)
	requireAdmissionWelcome(t, peer, 0)
	sendHandshake(t, peer, eventFrame(factsEvent(1, "Retained facts while waiting")))
	requireEventAck(t, peer, 1)
	report := folderRefusalEvent(2, operation, command)
	sendHandshake(t, peer, eventFrame(report))
	storetest.Receive(t, "the consumed folder report reaching the acknowledgement hold", held)
	requireFolderMark(t, f, m.ID, 2)
	if calls.Load() != 0 {
		t.Error("A waiting machine ran the folder callback.")
	}
	f.approve(t, m.ID)
	standing := seedClaimRow(t, f.presenceFixture, m.ID, standingAction(), folderDatabaseNow(t, f).Add(time.Minute))
	release()
	requireEventAck(t, peer, 2)
	frame, err := peer.Receive()
	if err != nil {
		t.Fatal("The approved machine did not receive its standing notice.")
	}
	standing.requireFrame(t, frame.GetCommand(), 1)

	fresh := proto.CloneOf(report)
	fresh.Number = 3
	sendHandshake(t, peer, eventFrame(fresh))
	requireEventAck(t, peer, 3)
	if calls.Load() != 1 {
		t.Error("The equivalent fresh report did not reach the part exactly once after approval.")
	}
}

func testFolderReportIdentity(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	f.svc.streaming.options.Commands = f.state
	owner, ownerKey := f.register(t, "folder operation owner")
	other, otherKey := f.register(t, "different authenticated machine")
	f.approve(t, owner.ID)
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.r.SetApproved(ctx, tx, other.ID, "Other approved machine")
	}); err != nil {
		t.Fatal("The fixture could not approve its second machine.")
	}
	closed, open := fixtureStreamID(t), fixtureStreamID(t)
	closedCommand := seedFolderCommand(t, f, owner.ID, closed)
	wrongCommand, unknown := fixtureStreamID(t), fixtureStreamID(t)
	storetest.Exec(t, f.obs, `CREATE TABLE fixture_folder_operation (
		id text PRIMARY KEY, machine_id text, command_id text, outcome text, report_number bigint)`)
	storetest.Exec(t, f.obs, `INSERT INTO fixture_folder_operation (id, machine_id, command_id)
		VALUES ($1, $2, $3)`, closed, owner.ID, closedCommand.id)
	var calls atomic.Int32
	registerFolderPart(t, f, PartHooks{
		ReportApplied: map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
			ReportFolderSeen: func(ctx context.Context, tx *store.Tx, machine string, event *linkv1.Event) error {
				calls.Add(1)
				report := event.GetFolderSeen()
				_, err := tx.Exec(ctx, `UPDATE fixture_folder_operation SET report_number = $4
					WHERE id = $1 AND machine_id = $2 AND command_id = $3 AND outcome IS NULL
					  AND report_number IS NULL`, report.GetOperationId(), machine, report.GetCommandId(), int64(event.GetNumber()))
				return err
			},
		},
		CommandSettled: map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
			CommandLookAtFolder: func(ctx context.Context, tx *store.Tx, event Settlement) error {
				_, err := tx.Exec(ctx, `UPDATE fixture_folder_operation SET outcome = $3
					WHERE machine_id = $1 AND command_id = $2 AND outcome IS NULL`,
					event.MachineID, event.Command.GetId(), string(event.Outcome))
				return err
			},
		},
	})

	wire := steadyWire(t, f, streamTestOptions{})
	run := fixtureStreamID(t)
	original := submitAdmission(t, f, wire, owner.ID, ownerKey, run, 1)
	requireAdmissionWelcome(t, original, 0)
	requireFolderDelivery(t, original, closedCommand, 1)
	oldStream := f.current(t, owner.ID).StreamID
	sendFolderResult(t, original, closedCommand, 1, 0)
	sendHandshake(t, original, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, original)
	requireSettlementTerminal(t, f.presenceFixture, closedCommand.id, "done")
	openCommand := seedFolderCommand(t, f, owner.ID, open)
	storetest.Exec(t, f.obs, `INSERT INTO fixture_folder_operation (id, machine_id, command_id)
		VALUES ($1, $2, $3)`, open, owner.ID, openCommand.id)
	sendHandshake(t, original, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, original)
	requireFolderDelivery(t, original, openCommand, 1)

	foreign := submitAdmission(t, f, wire, other.ID, otherKey, fixtureStreamID(t), 1)
	requireAdmissionWelcome(t, foreign, 0)
	sendHandshake(t, foreign, eventFrame(folderFactsEvent(1, open, openCommand.id)))
	requireEventAck(t, foreign, 1)
	peer := submitAdmission(t, f, wire, owner.ID, ownerKey, run, 1)
	requireAdmissionWelcome(t, peer, 0)
	requireFolderDelivery(t, peer, openCommand, 2)
	requireOpenEnd(t, original, connect.CodeAborted, openReplaced, "")

	obsolete, err := f.state.ApplyEvent(t.Context(), owner.ID, oldStream, folderFactsEvent(1, open, openCommand.id))
	if !errors.Is(err, ErrNotCurrent) || obsolete != (EventResult{}) {
		t.Error("The obsolete stream did not fail before routing the report.")
	}
	for _, report := range []*linkv1.Event{
		folderFactsEvent(1, closed, closedCommand.id),
		folderFactsEvent(2, unknown, openCommand.id),
		folderRefusalEvent(3, open, wrongCommand),
	} {
		sendHandshake(t, peer, eventFrame(report))
		requireEventAck(t, peer, report.GetNumber())
	}
	if calls.Load() != 4 || storetest.Scalar[int64](t, f.obs, "SELECT count(report_number) FROM fixture_folder_operation") != 0 {
		t.Error("A stale or mismatched report changed a part record or bypassed the authenticated route.")
	}
	sendHandshake(t, peer, eventFrame(folderRefusalEvent(4, open, openCommand.id)))
	requireEventAck(t, peer, 4)
	if calls.Load() != 5 || !storetest.Scalar[bool](t, f.obs, `SELECT
		EXISTS (SELECT FROM fixture_folder_operation WHERE id = $1 AND outcome = 'done' AND report_number IS NULL)
		AND EXISTS (SELECT FROM fixture_folder_operation WHERE id = $2 AND outcome IS NULL AND report_number = 4)`, closed, open) {
		t.Error("The valid refusal did not reach the open operation or the late report changed the closed one.")
	}
}

func folderFactsEvent(number uint64, operation, command string) *linkv1.Event {
	return &linkv1.Event{Number: number, Report: &linkv1.Event_FolderSeen{FolderSeen: &linkv1.FolderSeen{
		OperationId: operation, CommandId: command,
		Result: &linkv1.FolderSeen_Facts{Facts: &linkv1.FolderFacts{
			ResolvedPath: folderPath, Name: "repository", MainBranch: "main", Clean: true,
		}},
	}}}
}

func folderRefusalEvent(number uint64, operation, command string) *linkv1.Event {
	event := folderFactsEvent(number, operation, command)
	event.GetFolderSeen().Result = &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER}
	return event
}

func registerFolderPart(t *testing.T, f *handshakeFixture, hooks PartHooks) {
	t.Helper()
	if err := f.state.RegisterPart(hooks); err != nil {
		t.Fatal("The folder part could not register its routes.")
	}
}

func registerFolderCounter(t *testing.T, f *handshakeFixture, calls *atomic.Int32) {
	t.Helper()
	registerFolderPart(t, f, PartHooks{ReportApplied: map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
		ReportFolderSeen: func(context.Context, *store.Tx, string, *linkv1.Event) error {
			calls.Add(1)
			return nil
		},
	}})
}

func requireFolderMark(t *testing.T, f *handshakeFixture, machine string, want uint64) {
	t.Helper()
	if got := eventMark(t, f.presenceFixture, machine); got != want {
		t.Fatalf("The durable event mark was %d, want %d.", got, want)
	}
}

func folderDatabaseNow(t *testing.T, f *handshakeFixture) time.Time {
	t.Helper()
	return storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
}

func folderAction(operation, path string) *linkv1.Command {
	return &linkv1.Command{Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
		OperationId: operation, Path: path,
	}}}
}

func seedFolderCommand(t *testing.T, f *handshakeFixture, machine, operation string) claimFixtureRow {
	t.Helper()
	row := claimFixtureRow{fixtureStreamID(t), folderDatabaseNow(t, f).Add(time.Minute).Truncate(time.Microsecond), folderAction(operation, folderPath)}
	payload := marshalFolderProto(t, row.action)
	storetest.Exec(t, f.obs, `INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5::timestamptz - interval '2 minutes', $5)`, row.id, machine, CommandLookAtFolder, payload, row.expires)
	return row
}

func requireFolderDelivery(t *testing.T, peer *openClientStream, row claimFixtureRow, attempt uint32) {
	t.Helper()
	frame, err := peer.Receive()
	if err != nil || frame.GetCommand().GetLookAtFolder() == nil || frame.GetCommand().GetId() != row.id {
		t.Fatal("The stream did not deliver the expected folder head.")
	}
	row.requireFrame(t, frame.GetCommand(), attempt)
}

func sendFolderResult(t *testing.T, peer *openClientStream, row claimFixtureRow, attempt uint32, through uint64) {
	t.Helper()
	result := settlementFrame(row.id, attempt, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, through)
	sendHandshake(t, peer, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: result}})
}

func marshalFolderProto(t *testing.T, message proto.Message) []byte {
	t.Helper()
	data, err := proto.Marshal(message)
	if err != nil {
		t.Fatal("The folder fixture could not encode its Protocol Buffers message.")
	}
	return data
}

func TestRevisionTwoEventBudgetsAndAcks(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			t.Run("WireLimits", func(t *testing.T) { testFolderWireLimits(t, protocol.options) })
			t.Run("TerminalFrames", func(t *testing.T) { testFolderTerminalFrames(t, protocol.options) })
			t.Run("FullCreditWindow", func(t *testing.T) { testFolderCreditWindow(t, protocol.options) })
		})
	}
	t.Run("ReportValidation", testFolderReportValidation)
	t.Run("DirectAndJSONBudgets", testFolderDecodedBudget)
}

func folderWire(t *testing.T, f *handshakeFixture, options ...connect.ClientOption) linkv1connect.LinkServiceClient {
	t.Helper()
	server := httptest.NewUnstartedServer(api.NewHandler(api.Options{Link: f.svc}))
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	t.Cleanup(server.Close)
	return linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), server.URL, options...)
}

type folderWireCodec map[uint64][]byte

func (folderWireCodec) Name() string { return "proto" }

func (c folderWireCodec) Marshal(message any) ([]byte, error) {
	if request, ok := message.(*linkv1.OpenRequest); ok && request.GetEvent() != nil {
		if wire, ok := c[request.GetEvent().GetNumber()]; ok {
			// The client's envelope writer can return the output buffer to its pool.
			return bytes.Clone(wire), nil
		}
	}
	return proto.Marshal(message.(proto.Message))
}

func (folderWireCodec) Unmarshal(data []byte, message any) error {
	return proto.Unmarshal(data, message.(proto.Message))
}

func sizedFolderEvent(t *testing.T, number uint64, operation, command string, size int) *linkv1.Event {
	t.Helper()
	event := folderFactsEvent(number, operation, command)
	facts := event.GetFolderSeen().GetFacts()
	facts.MainBranch = strings.Repeat("m", size)
	facts.MainBranch = facts.MainBranch[:2*size-proto.Size(event)]
	if proto.Size(event) != size || len(marshalFolderProto(t, event)) != size {
		t.Fatal("The generated event did not reach the requested encoded size.")
	}
	return event
}

func folderWireField(field protowire.Number, payload []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, field, protowire.BytesType), payload)
}

func testFolderWireLimits(t *testing.T, options []connect.ClientOption) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "folder wire size")
	f.approve(t, m.ID)
	var calls atomic.Int32
	registerFolderCounter(t, f, &calls)
	operation, command := fixtureStreamID(t), fixtureStreamID(t)
	innerEdge := sizedFolderEvent(t, 1, operation, command, folderInnerBudget)
	innerOver := sizedFolderEvent(t, 2, operation, command, folderInnerBudget+1)
	redundant := folderFactsEvent(3, operation, command)
	// Repeated values of the singular number field disappear when the event is decoded.
	repeatedNumber := strings.Repeat("\x08\x03", folderInnerBudget/2)
	redundantPayload := append([]byte(repeatedNumber), marshalFolderProto(t, redundant)...)
	twoOccurrences := folderFactsEvent(4, operation, command)
	firstOccurrence := []byte(strings.Repeat("\x08\x04", folderInnerBudget/2))
	reset := folderFactsEvent(5, operation, command)
	eventField := protowire.Number(eventFrame(reset).ProtoReflect().Descriptor().Fields().ByName("event").Number())
	heartbeatField := protowire.Number(eventFrame(reset).ProtoReflect().Descriptor().Fields().ByName("heartbeat").Number())
	outerEdge := sizedFolderEvent(t, 6, operation, command, folderInnerBudget+1)
	outerOver := sizedFolderEvent(t, 7, operation, command, folderInnerBudget+1)
	padOuter := func(event *linkv1.Event, size int) []byte {
		raw := marshalFolderProto(t, eventFrame(event))
		// Field 100 costs two tag bytes and this padding uses a two-byte length.
		raw = append(raw, folderWireField(100, make([]byte, size-len(raw)-4))...)
		if len(raw) != size {
			t.Fatal("The generated wrapper did not reach the requested encoded size.")
		}
		return raw
	}
	// A real intervening arm discards the earlier Event. Unknown wrapper fields do not.
	resetWire := append(folderWireField(eventField, redundantPayload), folderWireField(heartbeatField, nil)...)
	resetWire = append(resetWire, folderWireField(eventField, marshalFolderProto(t, reset))...)
	combinedWire := append(folderWireField(eventField, firstOccurrence), folderWireField(100, nil)...)
	combinedWire = append(combinedWire, folderWireField(eventField, marshalFolderProto(t, twoOccurrences))...)
	codec := folderWireCodec{
		3: folderWireField(eventField, redundantPayload),
		4: combinedWire,
		5: resetWire,
		6: padOuter(outerEdge, folderOuterBudget),
		7: padOuter(outerOver, folderOuterBudget+1),
	}
	for _, event := range []*linkv1.Event{redundant, twoOccurrences, reset, outerEdge, outerOver} {
		var decoded linkv1.OpenRequest
		if err := proto.Unmarshal(codec[event.GetNumber()], &decoded); err != nil || !proto.Equal(event, decoded.GetEvent()) {
			t.Fatal("The custom wire fixture did not decode to the intended numbered event.")
		}
	}
	if len(redundantPayload) <= folderInnerBudget || proto.Size(redundant) >= folderInnerBudget ||
		len(firstOccurrence)+proto.Size(twoOccurrences) <= folderInnerBudget {
		t.Fatal("The redundant wire fixtures did not cross the encoded-only budget.")
	}

	run := fixtureStreamID(t)
	wire := folderWire(t, f, append(slices.Clone(options), connect.WithCodec(codec))...)
	peer := submitAdmission(t, f, wire, m.ID, key, run, 1)
	requireAdmissionWelcome(t, peer, 0)
	for _, sample := range []struct {
		event *linkv1.Event
		apply int32
	}{
		{innerEdge, 1}, {innerOver, 0}, {redundant, 0}, {twoOccurrences, 0}, {reset, 1}, {outerEdge, 0},
	} {
		before := calls.Load()
		sendHandshake(t, peer, eventFrame(sample.event))
		requireEventAck(t, peer, sample.event.GetNumber())
		requireFolderMark(t, f, m.ID, sample.event.GetNumber())
		if got := calls.Load() - before; got != sample.apply {
			t.Errorf("Event %d called the folder part %d times, want %d.", sample.event.GetNumber(), got, sample.apply)
		}
	}
	beforeRepeat := calls.Load()
	for _, repeat := range []*linkv1.Event{innerEdge, redundant, outerEdge} {
		sendHandshake(t, peer, eventFrame(repeat))
		frame, err := peer.Receive()
		if err != nil || frame.GetEventAck().GetThrough() != 6 {
			t.Fatalf("Repeat %d did not acknowledge mark 6. The receive code was %v.", repeat.GetNumber(), connect.CodeOf(err))
		}
	}
	sendHandshake(t, peer, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, peer)
	if calls.Load() != beforeRepeat {
		t.Error("A repeated event reached the folder route or produced an extra acknowledgement.")
	}

	gzipWire := folderWire(t, f, append(slices.Clone(options), connect.WithCodec(codec), connect.WithSendGzip())...)
	compressed := submitAdmission(t, f, gzipWire, m.ID, key, run, 1)
	requireAdmissionWelcome(t, compressed, 6)
	sendHandshake(t, compressed, eventFrame(outerOver))
	requireOpenEnd(t, compressed, connect.CodeInvalidArgument, openInvalid, "")
	requireFolderMark(t, f, m.ID, 6)
	if calls.Load() != beforeRepeat {
		t.Error("The decoded outer frame above the limit reached the part.")
	}
}

func testFolderTerminalFrames(t *testing.T, options []connect.ClientOption) {
	var calls atomic.Int32
	diagnostics := &handshakeLog{}
	f := newServerStreamFixture(t, handshakeFixtureOptions{logger: slog.New(slog.NewJSONHandler(diagnostics, nil))})
	m, key := f.register(t, "folder terminal frames")
	f.approve(t, m.ID)
	registerFolderCounter(t, f, &calls)
	operation, command, run := fixtureStreamID(t), fixtureStreamID(t), fixtureStreamID(t)
	const credentialMarker = "fixture-credential-in-rejected-folder"
	// The malformed Event field ends with an incomplete embedded message.
	malformed := append(folderWireField(100, []byte(credentialMarker)), 0x22, 0x02, 0xff)
	wire := folderWire(t, f, append(slices.Clone(options), connect.WithCodec(folderWireCodec{1: malformed}))...)
	for _, sample := range []struct {
		name   string
		number uint64
		code   connect.Code
		text   string
		reason string
	}{
		{"Zero", 0, connect.CodeInvalidArgument, openInvalid, ""},
		{"OutsideSignedRange", math.MaxInt64 + 1, connect.CodeInvalidArgument, openInvalid, ""},
		{"Gap", 2, connect.CodeFailedPrecondition, ErrEventGap.Error(), "event_gap"},
		{"Undecodable", 1, connect.CodeInvalidArgument, openInvalid, ""},
	} {
		t.Run(sample.name, func(t *testing.T) {
			peer := submitAdmission(t, f, wire, m.ID, key, run, 1)
			requireAdmissionWelcome(t, peer, 0)
			event := folderFactsEvent(sample.number, operation, command)
			event.GetFolderSeen().GetFacts().MainBranch = credentialMarker
			sendHandshake(t, peer, eventFrame(event))
			requireOpenEnd(t, peer, sample.code, sample.text, sample.reason)
			requireFolderMark(t, f, m.ID, 0)
		})
	}
	if calls.Load() != 0 {
		t.Error("A terminal event frame reached the folder callback.")
	}
	diagnostics.mu.Lock()
	defer diagnostics.mu.Unlock()
	if strings.Contains(diagnostics.log.String(), credentialMarker) {
		t.Error("The log disclosed a synthetic credential marker from a rejected frame.")
	}
}

func testFolderCreditWindow(t *testing.T, options []connect.ClientOption) {
	var calls atomic.Int32
	held := make(chan struct{}, 1)
	released, release := storetest.Signal(t)
	defer release()
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventAck: func(ctx context.Context, _ string, mark uint64) {
			if mark == 1 {
				held <- struct{}{}
				select {
				case <-released:
				case <-ctx.Done():
				}
			}
		},
	}})
	m, key := f.register(t, "full event credit")
	f.approve(t, m.ID)
	registerFolderCounter(t, f, &calls)
	operation, command := fixtureStreamID(t), fixtureStreamID(t)
	peer := submitAdmission(t, f, folderWire(t, f, options...), m.ID, key, fixtureStreamID(t), 1)
	requireAdmissionWelcome(t, peer, 0)
	sendHandshake(t, peer, eventFrame(folderRefusalEvent(1, operation, command)))
	storetest.Receive(t, "the first acknowledgement reaching the credit hold", held)
	for n := uint64(2); n <= folderCredit; n++ {
		if err := peer.Send(eventFrame(folderRefusalEvent(n, operation, command))); err != nil {
			t.Fatal("The fixture could not hand over all 32 events before the first acknowledgement.")
		}
	}
	requireFolderMark(t, f, m.ID, 1)
	release()
	for n := uint64(1); n <= folderCredit; n++ {
		requireEventAck(t, peer, n)
	}
	sendHandshake(t, peer, eventFrame(folderRefusalEvent(folderCredit+1, operation, command)))
	requireEventAck(t, peer, folderCredit+1)
	sendHandshake(t, peer, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, peer)
	requireFolderMark(t, f, m.ID, folderCredit+1)
	if calls.Load() != folderCredit+1 {
		t.Error("The full credit window did not apply each event once.")
	}
}

func testFolderReportValidation(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "folder report validation")
	f.approve(t, m.ID)
	var calls atomic.Int32
	registerFolderCounter(t, f, &calls)
	operation, command := fixtureStreamID(t), fixtureStreamID(t)
	peer := submitAdmission(t, f, folderWire(t, f), m.ID, key, fixtureStreamID(t), 1)
	requireAdmissionWelcome(t, peer, 0)

	resolvedEdge := "/" + strings.Repeat("é", 4090) + "/repository"
	originPrefix := "https://example.test/"
	originEdge := originPrefix + strings.Repeat("a", 2048-len(originPrefix))
	tests := []struct {
		name  string
		valid bool
		edit  func(*linkv1.Event, *linkv1.FolderSeen, *linkv1.FolderFacts)
	}{
		{"PathAndRemoteAtLimits", true, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) {
			facts.ResolvedPath, facts.Remote = resolvedEdge, proto.String(originEdge)
		}},
		{"NoOriginConfigured", true, func(*linkv1.Event, *linkv1.FolderSeen, *linkv1.FolderFacts) {}},
		{"OriginWithheld", true, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) { facts.RemoteOmitted = true }},
		{"MissingReport", false, func(event *linkv1.Event, _ *linkv1.FolderSeen, _ *linkv1.FolderFacts) { event.Report = nil }},
		{"UnknownReport", false, func(event *linkv1.Event, _ *linkv1.FolderSeen, _ *linkv1.FolderFacts) {
			event.Report = nil
			event.ProtoReflect().SetUnknown(folderWireField(100, nil))
		}},
		{"MissingResult", false, func(_ *linkv1.Event, report *linkv1.FolderSeen, _ *linkv1.FolderFacts) { report.Result = nil }},
		{"ZeroRefusal", false, func(_ *linkv1.Event, report *linkv1.FolderSeen, _ *linkv1.FolderFacts) {
			report.Result = &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED}
		}},
		{"UnknownRefusal", false, func(_ *linkv1.Event, report *linkv1.FolderSeen, _ *linkv1.FolderFacts) {
			report.Result = &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal(99)}
		}},
		{"NoncanonicalOperation", false, func(_ *linkv1.Event, report *linkv1.FolderSeen, _ *linkv1.FolderFacts) {
			report.OperationId = "01890ABC-0000-7000-8000-000000000001"
		}},
		{"CommandFromAnotherVersion", false, func(_ *linkv1.Event, report *linkv1.FolderSeen, _ *linkv1.FolderFacts) {
			report.CommandId = "01890abc-0000-4000-8000-000000000001"
		}},
		{"RelativeResolvedPath", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) {
			facts.ResolvedPath = "workspace/repository"
		}},
		{"ResolvedPathOverLimit", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) {
			facts.ResolvedPath = "/a" + resolvedEdge[1:]
		}},
		{"WrongDerivedName", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) { facts.Name = "other" }},
		{"EmptyMainBranch", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) { facts.MainBranch = "" }},
		{"ZeroByteInText", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) { facts.MainBranch = "ma\x00in" }},
		{"PresentEmptyBranch", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) {
			facts.CheckedOutBranch = proto.String("")
		}},
		{"PresentEmptyFirstCommit", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) {
			facts.FirstCommit = proto.String("")
		}},
		{"PresentEmptyRemote", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) {
			facts.Remote = proto.String("")
		}},
		{"RemoteOverLimit", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) {
			facts.Remote = proto.String(originEdge + "a")
		}},
		{"ContradictoryOmission", false, func(_ *linkv1.Event, _ *linkv1.FolderSeen, facts *linkv1.FolderFacts) {
			facts.Remote, facts.RemoteOmitted = proto.String(originEdge), true
		}},
	}
	if len(resolvedEdge) != 8192 || len(originEdge) != 2048 {
		t.Fatal("The text boundary fixtures have the wrong byte lengths.")
	}
	for i, sample := range tests {
		t.Run(sample.name, func(t *testing.T) {
			number := uint64(i + 1)
			event := folderFactsEvent(number, operation, command)
			report := event.GetFolderSeen()
			sample.edit(event, report, report.GetFacts())
			before := calls.Load()
			sendHandshake(t, peer, eventFrame(event))
			requireEventAck(t, peer, number)
			requireFolderMark(t, f, m.ID, number)
			want := int32(0)
			if sample.valid {
				want = 1
			}
			if calls.Load()-before != want {
				t.Error("Report validation did not leave the expected number of part callbacks.")
			}
		})
	}
}

func testFolderDecodedBudget(t *testing.T) {
	for _, name := range []string{"Direct", "JSON"} {
		t.Run(name, func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			m, key := f.register(t, "decoded folder budget")
			f.approve(t, m.ID)
			var calls atomic.Int32
			registerFolderCounter(t, f, &calls)
			operation, command := fixtureStreamID(t), fixtureStreamID(t)
			events := []*linkv1.Event{
				sizedFolderEvent(t, 1, operation, command, folderInnerBudget),
				sizedFolderEvent(t, 2, operation, command, folderInnerBudget+1),
			}
			if name == "JSON" {
				peer := submitAdmission(t, f, folderWire(t, f, connect.WithProtoJSON()), m.ID, key, fixtureStreamID(t), 1)
				requireAdmissionWelcome(t, peer, 0)
				for _, event := range events {
					sendHandshake(t, peer, eventFrame(event))
					requireEventAck(t, peer, event.GetNumber())
				}
			} else {
				stream := f.admit(t, m.ID, 1)
				invalidText := folderFactsEvent(3, operation, command)
				invalidText.GetFolderSeen().GetFacts().MainBranch = "\xff"
				for _, event := range append(events, invalidText) {
					result, err := f.state.ApplyEvent(t.Context(), m.ID, stream.ID, event)
					if err != nil || result.Mark != event.GetNumber() || !result.Applied {
						t.Fatal("The direct event did not commit an advance of its mark.")
					}
				}
			}
			if calls.Load() != 1 {
				t.Error("Only the exact decoded size edge should reach the folder part.")
			}
		})
	}
}

func TestLookAtFolderCommandBudgets(t *testing.T) {
	t.Run("Enqueue", testEnqueueFolderBudget)
	t.Run("CompleteEnvelope", testFolderCommandEnvelope)
	for _, release := range []string{"Staged", "Settled", "Reconnect", "OfferCutoff"} {
		t.Run(release, func(t *testing.T) { testFolderQueue(t, release) })
	}
}

func enqueueFolderAction(t *testing.T, f *handshakeFixture, machine string, action *linkv1.Command) (string, error) {
	t.Helper()
	var id string
	err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.r.Lock(ctx, tx, machine); err != nil {
			return err
		}
		var err error
		id, err = f.state.EnqueueCommand(ctx, tx, machine, action, time.Minute)
		return err
	})
	return id, err
}

func testEnqueueFolderBudget(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, _ := f.register(t, "outbound folder budget")
	f.approve(t, m.ID)
	operation := fixtureStreamID(t)
	path := "/" + strings.Repeat("é", folderPathBudget/2-1) + "a"
	want := folderAction(operation, path)
	action := proto.CloneOf(want)
	action.Id, action.Attempt = fixtureStreamID(t), 23
	action.ExpiresAt = timestamppb.New(time.Unix(1, 0))
	unknown := folderWireField(100, make([]byte, folderInnerBudget))
	action.ProtoReflect().SetUnknown(unknown)
	action.GetLookAtFolder().ProtoReflect().SetUnknown(unknown)
	if proto.Size(action) <= folderInnerBudget {
		t.Fatal("The fixture did not exceed the command limit before cleaning.")
	}
	head := f.head(t)
	before := folderDatabaseNow(t, f)
	id, err := enqueueFolderAction(t, f, m.ID, action)
	if err != nil || !streamVersion7.MatchString(id) || id == action.GetId() {
		t.Fatal("Enqueue did not accept the 4,096-byte path and return a fresh version 7 command identifier.")
	}
	after := folderDatabaseNow(t, f)
	if !storetest.Scalar[bool](t, f.obs, `SELECT machine_id = $2 AND action = 'look_at_folder'
		AND created_at BETWEEN $3 AND $4 AND expires_at = created_at + interval '1 minute'
		FROM link_commands WHERE id = $1`, id, m.ID, before, after) {
		t.Error("The committed command did not have the exact route, machine and sampled lifetime.")
	}
	payload := storetest.Scalar[[]byte](t, f.obs, "SELECT payload FROM link_commands WHERE id = $1", id)
	var saved linkv1.Command
	if err := proto.Unmarshal(payload, &saved); err != nil || !proto.Equal(want, &saved) {
		t.Error("The stored action did not contain exactly the original operation and path.")
	}
	if !hasOneSettlementWake(t, f.presenceFixture, head, m.ID) {
		t.Error("Enqueue did not commit exactly one machine outbox wake.")
	}

	for _, home := range []string{"~", "~/workspace", "/"} {
		if id, err := enqueueFolderAction(t, f, m.ID, folderAction(operation, home)); err != nil || id == "" {
			t.Error("Enqueue rejected a valid absolute path or home form.")
		}
	}
	head = f.head(t)
	count := storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_commands WHERE machine_id = $1", m.ID)
	for _, sample := range []struct{ name, operation, path string }{
		{"PathOverLimit", operation, path + "a"},
		{"Empty", operation, ""},
		{"Relative", operation, "workspace/repository"},
		{"NamedHome", operation, "~someone/workspace"},
		{"WindowsDrive", operation, "C:\\workspace"},
		{"SpaceBeforePath", operation, " /workspace"},
		{"InvalidText", operation, "/work\xff"},
		{"ZeroByte", operation, "/work\x00space"},
		{"NoncanonicalIdentifier", "01890ABC-0000-7000-8000-000000000001", folderPath},
		{"WrongIdentifierVersion", "01890abc-0000-4000-8000-000000000001", folderPath},
		{"MissingIdentifier", "", folderPath},
	} {
		t.Run(sample.name, func(t *testing.T) {
			id, err := enqueueFolderAction(t, f, m.ID, folderAction(sample.operation, sample.path))
			if !errors.Is(err, ErrInvalidMessage) || id != "" {
				t.Error("Invalid folder input received a command identifier or the wrong error.")
			}
			if f.head(t) != head || storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_commands WHERE machine_id = $1", m.ID) != count {
				t.Error("Invalid folder input changed the queue or feed.")
			}
		})
	}
}

func testFolderCommandEnvelope(t *testing.T) {
	id, operation := fixtureStreamID(t), fixtureStreamID(t)
	expiry := time.Unix(1789366800, 123456000)
	base := folderAction(operation, "/"+strings.Repeat("a", folderInnerBudget))
	encoded := proto.CloneOf(base)
	encoded.Id, encoded.ExpiresAt, encoded.Attempt = id, timestamppb.New(expiry), math.MaxUint32
	path := base.GetLookAtFolder().GetPath()
	path = path[:len(path)-(proto.Size(encoded)-folderInnerBudget)]
	for _, sample := range []struct {
		name, path string
		size       int
		allowed    bool
	}{
		{"AtLimit", path, folderInnerBudget, true},
		{"AboveLimit", path + "a", folderInnerBudget + 1, false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			command := folderAction(operation, sample.path)
			full := proto.CloneOf(command)
			full.Id, full.ExpiresAt, full.Attempt = id, timestamppb.New(expiry), math.MaxUint32
			if len(marshalFolderProto(t, full)) != sample.size {
				t.Fatal("The complete command fixture has the wrong encoded size.")
			}
			full.Attempt = 1
			if proto.Size(full) > folderInnerBudget {
				t.Fatal("Using a narrow attempt would not separate these size cases.")
			}
			if commandWithinBudget(command, id, expiry) != sample.allowed {
				t.Errorf("The complete command budget did not yield the expected decision at %d bytes.", sample.size)
			}
		})
	}
}

func testFolderQueue(t *testing.T, release string) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "deferred folder "+release)
	f.approve(t, m.ID)
	first := seedFolderCommand(t, f, m.ID, fixtureStreamID(t))
	second := seedFolderCommand(t, f, m.ID, fixtureStreamID(t))
	scan := seedClaimRow(t, f.presenceFixture, m.ID, scanAction(), first.expires)
	wire := steadyWire(t, f, streamTestOptions{})
	run := fixtureStreamID(t)
	peer := submitAdmission(t, f, wire, m.ID, key, run, 1)
	requireAdmissionWelcome(t, peer, 0)
	requireFolderDelivery(t, peer, first, 1)
	frame, err := peer.Receive()
	if err != nil || frame.GetCommand().GetScanHarnesses() == nil {
		t.Fatal("A busy folder head blocked the scan or allowed the folder successor to overtake it.")
	}
	scan.requireFrame(t, frame.GetCommand(), 1)
	sendHandshake(t, peer, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, peer)
	requireClaimStored(t, f.presenceFixture, second.id, "", 1)

	switch release {
	case "Staged":
		sendFolderResult(t, peer, first, 1, 1)
	case "Settled":
		sendFolderResult(t, peer, first, 1, 0)
	case "Reconnect":
		old := peer
		peer = submitAdmission(t, f, wire, m.ID, key, run, 1)
		requireAdmissionWelcome(t, peer, 0)
		requireFolderDelivery(t, peer, first, 2)
		frame, err := peer.Receive()
		if err != nil || frame.GetCommand().GetScanHarnesses() == nil {
			t.Fatal("The reconnected stream did not independently recover the scan after the original folder head.")
		}
		scan.requireFrame(t, frame.GetCommand(), 2)
		requireOpenEnd(t, old, connect.CodeAborted, openReplaced, "")
		sendHandshake(t, peer, heartbeatFrame(nil, nil))
		requireHeartbeatAck(t, peer)
		requireClaimStored(t, f.presenceFixture, second.id, "", 1)
		sendFolderResult(t, peer, first, 2, 0)
	case "OfferCutoff":
		storetest.Exec(t, f.obs, `UPDATE link_commands
			SET expires_at = clock_timestamp() - interval '23 hours 1 minute',
				created_at = clock_timestamp() - interval '23 hours 2 minutes' WHERE id = $1`, first.id)
		sendHandshake(t, peer, heartbeatFrame(nil, nil))
		requireHeartbeatAck(t, peer)
	}
	requireFolderDelivery(t, peer, second, 1)
	switch release {
	case "Staged":
		if !hasSettlementStage(t, f.presenceFixture, first.id, "done", 1) {
			t.Error("The successor did not become available while the first result still waited for its report.")
		}
	case "Settled", "Reconnect":
		requireSettlementTerminal(t, f.presenceFixture, first.id, "done")
	case "OfferCutoff":
		stream := f.current(t, m.ID).StreamID
		requireDispatch(t, f.presenceFixture, first, stream, 1, true)
	}
}

func testFolderTerminalRace(t *testing.T, removal, reportFirst bool) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, _ := f.register(t, "terminal folder race")
	f.approve(t, m.ID)
	operation := fixtureStreamID(t)
	command := seedFolderCommand(t, f, m.ID, operation)
	stream := f.admit(t, m.ID, 1)
	if !removal {
		storetest.Exec(t, f.obs, `UPDATE link_commands
			SET expires_at = clock_timestamp() - interval '25 hours',
				created_at = clock_timestamp() - interval '26 hours' WHERE id = $1`, command.id)
	}
	storetest.Exec(t, f.obs, `CREATE TABLE fixture_folder_terminal (outcome text)`)
	storetest.Exec(t, f.obs, `INSERT INTO fixture_folder_terminal VALUES (NULL)`)
	held := make(chan struct{}, 1)
	released, release := storetest.Signal(t)
	defer release()
	hold := func(ctx context.Context) error {
		held <- struct{}{}
		select {
		case <-released:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	registerFolderPart(t, f, PartHooks{
		ReportApplied: map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
			ReportFolderSeen: func(ctx context.Context, tx *store.Tx, machine string, event *linkv1.Event) error {
				if machine != m.ID || event.GetFolderSeen().GetCommandId() != command.id {
					return errors.New("The report reached the part under the wrong machine or command.")
				}
				_, err := tx.Exec(ctx, "UPDATE fixture_folder_terminal SET outcome = 'report' WHERE outcome IS NULL")
				return err
			},
		},
		CommandSettled: map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
			CommandLookAtFolder: func(ctx context.Context, tx *store.Tx, event Settlement) error {
				if _, err := tx.Exec(ctx, "UPDATE fixture_folder_terminal SET outcome = $1 WHERE outcome IS NULL", event.Outcome); err != nil {
					return err
				}
				if !reportFirst {
					return hold(ctx)
				}
				return nil
			},
		},
	})
	report := folderFactsEvent(1, operation, command.id)
	apply := func() error {
		var seam func(context.Context, string, uint64) error
		if reportFirst {
			seam = func(ctx context.Context, _ string, _ uint64) error { return hold(ctx) }
		}
		result, err := f.state.applyEvent(t.Context(), m.ID, stream.ID, report, seam)
		if err == nil && result != (EventResult{Mark: 1, Applied: true}) {
			return errors.New("The report did not commit its event mark.")
		}
		return err
	}
	terminate := func() error {
		if !removal {
			return f.state.SweepCommands(t.Context())
		}
		return f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if err := f.r.Delete(ctx, tx, m.ID); err != nil {
				return err
			}
			return f.state.Forget(ctx, tx, m.ID)
		})
	}
	waitForRow := func() {
		storetest.Poll(t, "the competing write waiting on the machine row", func() bool {
			return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
				SELECT FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'
				  AND (lower(query) LIKE '%for no key update%' OR lower(query) LIKE 'delete from machines%')
				  AND cardinality(pg_blocking_pids(pid)) > 0)`)
		})
	}

	wantPart, wantLedger := "timed_out", "timed_out"
	if removal {
		wantPart, wantLedger = "removed", "removed"
	}
	if reportFirst {
		wantPart = "report"
		applying := storetest.Start(t, apply)
		storetest.Receive(t, "the uncommitted folder report", held)
		if removal {
			ending := storetest.Start(t, terminate)
			waitForRow()
			release()
			if ending.Wait(t) != nil {
				t.Fatal("Removal did not complete after the report.")
			}
		} else {
			if err := terminate(); err != nil {
				t.Fatal("The sweep failed while skipping the locked machine.")
			}
			if !storetest.Scalar[bool](t, f.obs, "SELECT outcome IS NULL FROM link_commands WHERE id = $1", command.id) {
				t.Error("The sweep bypassed the machine lock held by the report.")
			}
			release()
		}
		if applying.Wait(t) != nil {
			t.Fatal("The report did not complete before the terminal action.")
		}
		if !removal && terminate() != nil {
			t.Fatal("The sweep failed after the report committed.")
		}
	} else {
		ending := storetest.Start(t, terminate)
		storetest.Receive(t, "the uncommitted terminal settlement", held)
		applying := storetest.Start(t, apply)
		waitForRow()
		release()
		if ending.Wait(t) != nil {
			t.Fatal("The terminal action did not complete before the report.")
		}
		if err := applying.Wait(t); (removal && !errors.Is(err, ErrUnknownMachine)) || (!removal && err != nil) {
			t.Error("The report did not respect the terminal action that won the machine lock.")
		}
	}
	if !storetest.Scalar[bool](t, f.obs, "SELECT COALESCE(outcome = $1, false) FROM fixture_folder_terminal", wantPart) {
		t.Error("The folder part did not preserve the first terminal reply.")
	}
	requireSettlementTerminal(t, f.presenceFixture, command.id, wantLedger)
	if !removal {
		requireFolderMark(t, f, m.ID, 1)
	}
}
