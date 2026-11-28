package projects_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type reportStream = connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]

type reportFixture struct {
	*operationFixture
	peer      *machinetest.Machine
	serverKey ed25519.PrivateKey
	server    *httptest.Server
	client    linkv1connect.LinkServiceClient
	hub       *link.Hub
	service   *projects.Service
	logs      operationLogs
}

type reportControls struct {
	link.Seams
	afterReport func(context.Context, *store.Tx, *linkv1.Event) error
	dispatch    bool
}

func newReportFixture(t *testing.T, controls reportControls) *reportFixture {
	t.Helper()
	base := newOperationFixture(t)
	base.disconnect(t)
	_, serverKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("The report fixture could not generate a server identity.")
	}
	f := &reportFixture{operationFixture: base, serverKey: serverKey}
	// The admission fixture keeps its signing identity local. Register a peer whose
	// identity can be used again when the server restarts or rejects a write.
	f.peer = f.registerPeer(t, "Report fixture")
	f.machine = f.peer.ID
	f.startServer(t, controls)
	f.stream = f.open(t, f.peer, 0)
	f.awaitPresence(t, true)
	return f
}

func (f *reportFixture) registerPeer(t *testing.T, name string) *machinetest.Machine {
	t.Helper()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("The report fixture could not generate a machine identity.")
	}
	var machine machines.Machine
	if err := f.db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		var err error
		machine, err = f.registry.CreateWaiting(ctx, tx, machines.NewMachine{PublicKey: public, HostName: name})
		if err != nil {
			return err
		}
		return f.registry.SetApproved(ctx, tx, machine.ID, name)
	}); err != nil {
		t.Fatal("The report fixture could not register and approve its machine.")
	}
	return &machinetest.Machine{
		ID: machine.ID, Key: private, RunID: projectMigrationID(t), FirstEventNumber: 1,
	}
}

func (f *reportFixture) startServer(t *testing.T, controls reportControls) {
	t.Helper()
	f.hub = link.NewHub()
	f.state = link.NewState(f.db, f.registry, f.hub)
	service := f.operationFixture.service(t, f.db, f.state)
	f.service = service
	if err := f.state.RegisterPart(link.PartHooks{
		ReportApplied: map[link.ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
			link.ReportFolderSeen: func(ctx context.Context, tx *store.Tx, machine string, event *linkv1.Event) error {
				if err := service.ApplyFolderSeen(ctx, tx, machine, event.GetFolderSeen()); err != nil {
					return err
				}
				if controls.afterReport != nil {
					return controls.afterReport(ctx, tx, event)
				}
				return nil
			},
		},
		CommandSettled: map[link.CommandKind]func(context.Context, *store.Tx, link.Settlement) error{
			link.CommandLookAtFolder: func(ctx context.Context, tx *store.Tx, event link.Settlement) error {
				return service.SettleCommand(ctx, tx, event.MachineID, event.Command.GetId(), string(event.Outcome))
			},
		},
	}); err != nil {
		t.Fatal("The report fixture could not register the Projects callbacks.")
	}
	options := link.Options{Logger: slog.New(slog.NewTextHandler(&f.logs, nil)), Seams: controls.Seams}
	if controls.dispatch {
		options.Commands = f.state
	}
	streams := link.NewStreamingService(t.Context(), f.db, nil, f.registry, f.state, f.hub, f.serverKey, options)
	_, handler := linkv1connect.NewLinkServiceHandler(streams)
	f.server = machinetest.NewServer(t, handler, 5*time.Second)
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	httpClient := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	t.Cleanup(httpClient.CloseIdleConnections)
	f.client = linkv1connect.NewLinkServiceClient(httpClient, f.server.URL, connect.WithGRPC())
}

func (f *reportFixture) open(t *testing.T, peer *machinetest.Machine, mark uint64) *reportStream {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	identity := *peer
	identity.ServerKey = f.serverKey.Public().(ed25519.PublicKey)
	stream, welcome, err := identity.Connect(ctx, f.client)
	if err != nil || welcome.GetLastEventNumber() != mark {
		t.Fatal("The signed report stream did not open at the durable mark.")
	}
	t.Cleanup(func() {
		cancel()
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	})
	requireReportHeartbeat(t, stream)
	return stream
}

func sendReportFrame(t *testing.T, stream *reportStream, frame *linkv1.OpenRequest) {
	t.Helper()
	if err := stream.Send(frame); err != nil {
		t.Fatal("The fixture could not send a frame on the signed report stream.")
	}
}

func requireReportHeartbeat(t *testing.T, stream *reportStream) {
	t.Helper()
	sendReportFrame(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}})
	if frame, err := stream.Receive(); err != nil || frame.GetHeartbeatAck() == nil {
		t.Fatal("The fixture expected a heartbeat acknowledgement without a command or another frame.")
	}
}

type reportOperation struct {
	call        operationCall
	id, command string
}

func (f *reportFixture) admit(t *testing.T, key string, add bool) reportOperation {
	t.Helper()
	call := operationCall{key: key, machine: f.machine, path: "~/typed folder", add: add}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	task := call.start(t, ctx, f.service)
	op := reportOperation{call: call}
	storetest.Poll(t, "the committed operation, command and receipt", func() bool {
		err := f.observer.QueryRow(t.Context(), `
SELECT o.id, o.command_id FROM folder_operations o
JOIN link_commands c ON c.id = o.command_id
JOIN receipts r ON r.request_key = o.request_key AND r.held
WHERE o.request_key = $1 AND o.machine_id = $2 AND o.state = 'open'`, key, f.machine).Scan(&op.id, &op.command)
		if errors.Is(err, pgx.ErrNoRows) {
			select {
			case <-task.Done():
				t.Fatal("The caller exited before committing its folder operation.")
			default:
				return false
			}
		}
		if err != nil {
			t.Fatal("The observer could not read the report fixture admission.")
		}
		return true
	})
	cancel()
	task.Wait(t)
	return op
}

func (f *reportFixture) deliver(t *testing.T, op reportOperation) *linkv1.Command {
	t.Helper()
	f.hub.Wake(f.machine)
	frame, err := f.stream.Receive()
	command := frame.GetCommand()
	if err != nil || command.GetId() != op.command || command.GetLookAtFolder().GetOperationId() != op.id || command.GetAttempt() == 0 {
		t.Fatal("Link did not deliver the admitted folder command.")
	}
	// A later inbound frame crosses the send transaction before its result arrives.
	requireReportHeartbeat(t, f.stream)
	return command
}

func sendReportResult(t *testing.T, stream *reportStream, id string, attempt uint32, outcome linkv1.CommandOutcome, mark uint64) {
	t.Helper()
	sendReportFrame(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: &linkv1.CommandResult{
		CommandId: id, Attempt: attempt, Outcome: outcome, LastEventNumber: mark,
	}}})
}

func sendFolderReport(t *testing.T, stream *reportStream, number uint64, seen *linkv1.FolderSeen) {
	t.Helper()
	sendReportFrame(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: &linkv1.Event{
		Number: number, Report: &linkv1.Event_FolderSeen{FolderSeen: seen},
	}}})
}

func (f *reportFixture) requireAck(t *testing.T, stream *reportStream, machine string, mark uint64) {
	t.Helper()
	frame, err := stream.Receive()
	if err != nil || frame.GetEventAck() == nil || frame.GetEventAck().GetThrough() != mark {
		t.Fatal("The signed stream did not acknowledge the expected event mark.")
	}
	f.requireMark(t, machine, mark)
}

func (f *reportFixture) requireMark(t *testing.T, machine string, mark uint64) {
	t.Helper()
	var got uint64
	if err := f.observer.QueryRow(t.Context(), "SELECT event_mark FROM link_machines WHERE machine_id = $1", machine).Scan(&got); err != nil || got != mark {
		t.Fatal("The independent observer did not see the expected durable event mark.")
	}
}

func requireReportWriteFailure(t *testing.T, stream *reportStream) {
	t.Helper()
	frame, err := stream.Receive()
	var refusal *connect.Error
	if frame != nil || !errors.As(err, &refusal) || refusal.Code() != connect.CodeUnavailable || refusal.Message() != operationDatabase {
		t.Fatal("The failed write did not end the stream with the fixed database refusal and no intervening frame.")
	}
}

type reportRow struct {
	state, failure, version string
	reply                   []byte
}

func (f *reportFixture) row(t *testing.T, op reportOperation) reportRow {
	t.Helper()
	var row reportRow
	if err := f.observer.QueryRow(t.Context(), `
SELECT state, COALESCE(failure, ''), xmin::text, reply
FROM folder_operations WHERE id = $1 AND machine_id = $2 AND command_id = $3`,
		op.id, op.call.machine, op.command).Scan(&row.state, &row.failure, &row.version, &row.reply); err != nil {
		t.Fatal("The observer could not read the admitted folder operation.")
	}
	return row
}

func (f *reportFixture) requireUnchanged(t *testing.T, op reportOperation, before reportRow) {
	t.Helper()
	after := f.row(t, op)
	if after.version != before.version || after.state != before.state || after.failure != before.failure || !bytes.Equal(after.reply, before.reply) {
		t.Error("An ineffective report or outcome rewrote the operation.")
	}
}

func (f *reportFixture) requireInspect(t *testing.T, op reportOperation, want *webv1.FolderFacts, equivalentOrigin ...string) reportRow {
	t.Helper()
	row := f.row(t, op)
	var reply webv1.InspectFolderResponse
	if row.state != "succeeded" || row.failure != "" || proto.Unmarshal(row.reply, &reply) != nil {
		t.Fatal("The inspected operation did not store a successful Protocol Buffers reply.")
	}
	for _, origin := range equivalentOrigin {
		if reply.GetFacts().GetRemote() == origin {
			want = proto.CloneOf(want)
			want.Remote = new(origin)
			break
		}
	}
	if !proto.Equal(&reply, &webv1.InspectFolderResponse{Facts: want}) {
		t.Fatal("The Inspect reply did not preserve the expected folder facts.")
	}
	requireOperationReply(t, op.call.run(t.Context(), f.service, op.call.key), row.reply)
	return row
}

func (f *reportFixture) requireFailure(t *testing.T, op reportOperation, key string, code connect.Code, sentence string) reportRow {
	t.Helper()
	row := f.row(t, op)
	if row.state != "failed" || row.failure != key || row.reply != nil {
		t.Fatalf("The folder operation did not store the fixed %s failure.", key)
	}
	requireOperationRefusal(t, op.call.run(t.Context(), f.service, op.call.key), code, sentence)
	return row
}

func (f *reportFixture) requireNoMembership(t *testing.T) {
	t.Helper()
	var clean bool
	if err := f.observer.QueryRow(t.Context(), `
SELECT NOT EXISTS (SELECT FROM repositories)
   AND NOT EXISTS (SELECT FROM repository_folders)
   AND NOT EXISTS (SELECT FROM feed WHERE topic = 'repositories')`).Scan(&clean); err != nil || !clean {
		t.Error("The report write left repository membership or a repository feed reference.")
	}
}

func (f *reportFixture) requireCommand(t *testing.T, op reportOperation, attempt uint32, outcome, staged string, watermark *uint64) {
	t.Helper()
	var matches bool
	if err := f.observer.QueryRow(t.Context(), `
SELECT attempt = $2 AND outcome IS NOT DISTINCT FROM NULLIF($3, '')
   AND staged_outcome IS NOT DISTINCT FROM NULLIF($4, '')
   AND staged_event_number IS NOT DISTINCT FROM $5::bigint
FROM link_commands WHERE id = $1`, op.command, attempt, outcome, staged, watermark).Scan(&matches); err != nil || !matches {
		t.Fatal("The independent observer did not see the expected Link attempt and settlement state.")
	}
}

func (op reportOperation) facts() *linkv1.FolderSeen {
	return &linkv1.FolderSeen{OperationId: op.id, CommandId: op.command, Result: &linkv1.FolderSeen_Facts{Facts: &linkv1.FolderFacts{
		ResolvedPath: "/workspace/checkout", Name: "checkout", MainBranch: "main",
		CheckedOutBranch: new("topic"), Remote: new("https://example.test/checkout.git"), FirstCommit: new("first-commit"),
	}}}
}

func expectedReportFacts() *webv1.FolderFacts {
	return &webv1.FolderFacts{
		ResolvedPath: "/workspace/checkout", Name: "checkout", MainBranch: "main",
		CheckedOutBranch: new("topic"), Remote: new("https://example.test/checkout.git"), FirstCommit: new("first-commit"),
	}
}

func TestProjectReportIdentityAndFacts(t *testing.T) {
	f := newReportFixture(t, reportControls{})
	op := f.admit(t, "identity and facts", false)
	crossed := f.admit(t, "second operation", false)
	before, crossedBefore := f.row(t, op), f.row(t, crossed)

	t.Run("WrongMachineReport", func(t *testing.T) {
		other := f.registerPeer(t, "Other report fixture")
		foreign := f.open(t, other, 0)
		sendFolderReport(t, foreign, 1, op.facts())
		f.requireAck(t, foreign, other.ID, 1)
		f.requireUnchanged(t, op, before)
	})

	const unsafeOriginMarker = "synthetic-origin-value"
	resolvedEdge := "/" + strings.Repeat("é", 4091) + "/checkout"
	originPrefix := "https://example.test/"
	remoteEdge := originPrefix + strings.Repeat("a", 2048-len(originPrefix))
	if len(resolvedEdge) != 8192 || len(remoteEdge) != 2048 {
		t.Fatal("The folder facts fixtures have the wrong byte lengths.")
	}
	type ineffectiveCase struct {
		name string
		edit func(*linkv1.FolderSeen)
	}
	ineffective := []ineffectiveCase{
		{"CrossedOperation", func(seen *linkv1.FolderSeen) { seen.OperationId = crossed.id }},
		{"CrossedCommand", func(seen *linkv1.FolderSeen) { seen.CommandId = crossed.command }},
		{"MissingResult", func(seen *linkv1.FolderSeen) { seen.Result = nil }},
		{"ZeroRefusal", func(seen *linkv1.FolderSeen) { seen.Result = &linkv1.FolderSeen_Refusal{} }},
		{"UnknownRefusal", func(seen *linkv1.FolderSeen) {
			seen.Result = &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal(99)}
		}},
		{"RelativePath", func(seen *linkv1.FolderSeen) { seen.GetFacts().ResolvedPath = "workspace/checkout" }},
		{"PathOverLimit", func(seen *linkv1.FolderSeen) { seen.GetFacts().ResolvedPath = "/a" + resolvedEdge[1:] }},
		{"WrongName", func(seen *linkv1.FolderSeen) { seen.GetFacts().Name = "workspace" }},
		{"EmptyMainBranch", func(seen *linkv1.FolderSeen) { seen.GetFacts().MainBranch = "" }},
		{"ZeroByte", func(seen *linkv1.FolderSeen) { seen.GetFacts().MainBranch = "ma\x00in" }},
		{"EmptyCheckedOutBranch", func(seen *linkv1.FolderSeen) { seen.GetFacts().CheckedOutBranch = new("") }},
		{"EmptyFirstCommit", func(seen *linkv1.FolderSeen) { seen.GetFacts().FirstCommit = new("") }},
		{"EmptyRemote", func(seen *linkv1.FolderSeen) { seen.GetFacts().Remote = new("") }},
		{"RemoteOverLimit", func(seen *linkv1.FolderSeen) { seen.GetFacts().Remote = new(remoteEdge + "a") }},
		{"ContradictoryOmission", func(seen *linkv1.FolderSeen) { seen.GetFacts().RemoteOmitted = true }},
		{"CredentialBearingOrigin", func(seen *linkv1.FolderSeen) {
			seen.GetFacts().Remote = new("https://fixture-user:" + unsafeOriginMarker + "@example.test/checkout.git")
		}},
		{"AccountRelativeURL", func(seen *linkv1.FolderSeen) {
			seen.GetFacts().Remote = new("ssh://example.test/~/checkout.git")
		}},
	}
	for _, origin := range []struct{ name, value string }{
		{"AccountRelativeShorthand", "example.test:checkout.git"},
		{"LocalOrigin", "/workspace/checkout.git"},
		{"UnsupportedOriginScheme", "ftp://example.test/checkout.git"},
		{"OriginQuery", "https://example.test/checkout.git?fixture=1"},
		{"OriginFragment", "https://example.test/checkout.git#fixture"},
		{"EscapedOriginPath", "https://example.test/checkout%2Fsample.git"},
		{"OriginDotSegment", "https://example.test/group/../checkout.git"},
	} {
		ineffective = append(ineffective, ineffectiveCase{origin.name, func(seen *linkv1.FolderSeen) {
			seen.GetFacts().Remote = new(origin.value)
		}})
	}
	var mark uint64
	for _, sample := range ineffective {
		t.Run(sample.name, func(t *testing.T) {
			mark++
			seen := op.facts()
			sample.edit(seen)
			sendFolderReport(t, f.stream, mark, seen)
			f.requireAck(t, f.stream, f.machine, mark)
			f.requireUnchanged(t, op, before)
			f.requireUnchanged(t, crossed, crossedBefore)

			// Link checks the wire shape before routing. Exercise the part's own
			// validation under the same machine lock as the production callback.
			if err := f.db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
				if _, err := f.registry.Lock(ctx, tx, f.machine); err != nil {
					return err
				}
				return f.service.ApplyFolderSeen(ctx, tx, f.machine, seen)
			}); err != nil {
				t.Fatal("Projects did not consume the ineffective folder report.")
			}
			f.requireUnchanged(t, op, before)
			f.requireUnchanged(t, crossed, crossedBefore)
		})
	}

	t.Run("DirtyNonMainAndByteLimits", func(t *testing.T) {
		mark++
		seen := op.facts()
		seen.GetFacts().ResolvedPath, seen.GetFacts().Remote = resolvedEdge, new(remoteEdge)
		sendFolderReport(t, f.stream, mark, seen)
		f.requireAck(t, f.stream, f.machine, mark)
		want := expectedReportFacts()
		want.ResolvedPath, want.Remote = resolvedEdge, new(remoteEdge)
		saved := f.requireInspect(t, op, want)

		// First retry the same sequence with different valid facts. Then number it
		// as a new event, which can advance the mark but cannot change the reply.
		late := op.facts()
		late.GetFacts().MainBranch = "replacement"
		sendFolderReport(t, f.stream, mark, late)
		f.requireAck(t, f.stream, f.machine, mark)
		f.requireUnchanged(t, op, saved)
		mark++
		sendFolderReport(t, f.stream, mark, late)
		f.requireAck(t, f.stream, f.machine, mark)
		f.requireUnchanged(t, op, saved)
		requireOperationReply(t, op.call.run(t.Context(), f.service, op.call.key), saved.reply)
	})

	t.Run("DetachedAndUnconfigured", func(t *testing.T) {
		mark++
		seen := crossed.facts()
		facts := seen.GetFacts()
		facts.CheckedOutBranch, facts.FirstCommit, facts.Remote, facts.Clean = nil, nil, nil, true
		sendFolderReport(t, f.stream, mark, seen)
		f.requireAck(t, f.stream, f.machine, mark)
		want := expectedReportFacts()
		want.CheckedOutBranch, want.FirstCommit, want.Remote, want.Clean = nil, nil, nil, true
		f.requireInspect(t, crossed, want)
	})

	t.Run("WithheldOrigin", func(t *testing.T) {
		withheld := f.admit(t, "withheld origin", false)
		mark++
		seen := withheld.facts()
		seen.GetFacts().Remote, seen.GetFacts().RemoteOmitted = nil, true
		sendFolderReport(t, f.stream, mark, seen)
		f.requireAck(t, f.stream, f.machine, mark)
		want := expectedReportFacts()
		want.Remote, want.RemoteOmitted = nil, true
		f.requireInspect(t, withheld, want)
	})

	t.Run("AbsoluteShorthand", func(t *testing.T) {
		shorthand := f.admit(t, "absolute shorthand", false)
		mark++
		const origin = "example.test:/git/Checkout.git"
		seen := shorthand.facts()
		seen.GetFacts().Remote = new(origin)
		sendFolderReport(t, f.stream, mark, seen)
		f.requireAck(t, f.stream, f.machine, mark)
		want := expectedReportFacts()
		want.Remote = new(origin)
		f.requireInspect(t, shorthand, want, "ssh://example.test/git/Checkout.git")
	})

	if f.logs.containsAny(unsafeOriginMarker) {
		t.Error("A synthetic credential from an ineffective origin reached the Link log.")
	}
	f.requireNoMembership(t)

	t.Run("AddAttachmentError", func(t *testing.T) {
		add := f.admit(t, "valid add", true)
		before := f.row(t, add)
		// Item 4 uses the temporary attachment error. Once item 5 replaces the
		// seam, this trigger preserves the same rollback observation.
		f.exec(t, `
CREATE FUNCTION fixture_refuse_repository_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'The fixture rejects repository creation.'; END
$$;
CREATE TRIGGER fixture_refuse_repository_insert BEFORE INSERT ON repositories
FOR EACH ROW EXECUTE FUNCTION fixture_refuse_repository_insert();`)
		sendFolderReport(t, f.stream, mark+1, add.facts())
		requireReportWriteFailure(t, f.stream)
		f.requireMark(t, f.machine, mark)
		f.requireUnchanged(t, add, before)
		f.requireNoMembership(t)
	})
}

func (f *reportFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.observer.Exec(t.Context(), sql, args...); err != nil {
		t.Fatal("The report fixture could not apply its database control.")
	}
}

func TestProjectFolderSettlement(t *testing.T) {
	t.Run("AllOutcomes", testProjectReportOutcomes)
	t.Run("ReportRefusals", testProjectReportRefusals)
	t.Run("WrongMachine", testProjectReportWrongMachine)
	t.Run("StagedAcrossRestart", testProjectStagedReport)
	t.Run("OutcomeBeforeLateReport", testProjectLateReport)
	t.Run("SettlementStorageError", testProjectSettlementStorageError)
}

func testProjectReportOutcomes(t *testing.T) {
	f := newReportFixture(t, reportControls{dispatch: true})
	const (
		notTaken = "The machine did not take the folder command."
		unknown  = "The outcome of the folder request is unknown."
	)
	wire := []struct {
		name, failure, sentence string
		outcome                 linkv1.CommandOutcome
	}{
		{"done", "no_answer", operationDeadline, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE},
		{"failed", "no_answer", operationDeadline, linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED},
		{"expired", "not_taken", notTaken, linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED},
		{"unsupported", "not_taken", notTaken, linkv1.CommandOutcome_COMMAND_OUTCOME_UNSUPPORTED},
		{"interrupted", "unknown", unknown, linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED},
	}
	for _, sample := range wire {
		t.Run(sample.name, func(t *testing.T) {
			op := f.admit(t, sample.name, false)
			command := f.deliver(t, op)
			sendReportResult(t, f.stream, op.command, command.GetAttempt(), sample.outcome, 0)
			requireReportHeartbeat(t, f.stream)
			f.requireCommand(t, op, command.GetAttempt(), sample.name, "", nil)
			f.requireFailure(t, op, sample.failure, connect.CodeDeadlineExceeded, sample.sentence)
		})
	}
	t.Run("timed_out", func(t *testing.T) {
		op := f.admit(t, "timed out", false)
		f.exec(t, `UPDATE link_commands SET created_at = clock_timestamp() - interval '2 days',
expires_at = clock_timestamp() - interval '1 day' WHERE id = $1`, op.command)
		if err := f.state.SweepCommands(t.Context()); err != nil {
			t.Fatal("Link could not sweep the aged folder command.")
		}
		f.requireCommand(t, op, 1, "timed_out", "", nil)
		f.requireFailure(t, op, "unknown", connect.CodeDeadlineExceeded, unknown)
	})
	t.Run("removed", func(t *testing.T) {
		op := f.admit(t, "removed", false)
		if err := f.db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if err := f.registry.Delete(ctx, tx, f.machine); err != nil {
				return err
			}
			return f.state.Forget(ctx, tx, f.machine)
		}); err != nil {
			t.Fatal("Link could not settle the folder command in the machine removal write.")
		}
		f.requireCommand(t, op, 1, "removed", "", nil)
		f.requireFailure(t, op, "machine_removed", connect.CodeNotFound, "The machine was removed.")
	})
	f.requireNoMembership(t)
}

func testProjectReportRefusals(t *testing.T) {
	f := newReportFixture(t, reportControls{})
	refusals := []struct {
		key, sentence string
		refusal       linkv1.FolderRefusal
	}{
		{"not_a_folder", "The path is not a folder.", linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER},
		{"not_a_git_repository", "The folder is not a Git repository.", linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY},
		{"not_readable", "The folder cannot be read.", linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE},
		{"path_not_usable", "The folder path cannot be used.", linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE},
	}
	for i, sample := range refusals {
		t.Run(sample.key, func(t *testing.T) {
			// The non-repository Add refusal must settle without reaching attachment.
			op := f.admit(t, sample.key, sample.refusal == linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY)
			mark := uint64(i + 1)
			sendFolderReport(t, f.stream, mark, &linkv1.FolderSeen{
				OperationId: op.id, CommandId: op.command, Result: &linkv1.FolderSeen_Refusal{Refusal: sample.refusal},
			})
			f.requireAck(t, f.stream, f.machine, mark)
			f.requireFailure(t, op, sample.key, connect.CodeFailedPrecondition, sample.sentence)
		})
	}
	f.requireNoMembership(t)
}

func testProjectReportWrongMachine(t *testing.T) {
	f := newReportFixture(t, reportControls{dispatch: true})
	op := f.admit(t, "wrong machine outcome", false)
	command := f.deliver(t, op)
	before := f.row(t, op)
	other := f.registerPeer(t, "Other report fixture")
	foreign := f.open(t, other, 0)
	sendReportResult(t, foreign, op.command, command.GetAttempt(), linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 0)
	requireReportHeartbeat(t, foreign)
	f.requireCommand(t, op, command.GetAttempt(), "", "", nil)

	// Link declines the forged result before routing. Check Projects' own
	// machine and command guards under the corresponding registry lock.
	for _, mismatch := range []struct{ machine, command string }{
		{other.ID, op.command},
		{f.machine, projectMigrationID(t)},
	} {
		if err := f.db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if _, err := f.registry.Lock(ctx, tx, mismatch.machine); err != nil {
				return err
			}
			return f.service.SettleCommand(ctx, tx, mismatch.machine, mismatch.command, "done")
		}); err != nil {
			t.Fatal("Projects did not consume the mismatched settlement callback.")
		}
		f.requireUnchanged(t, op, before)
	}

	sendReportResult(t, f.stream, op.command, command.GetAttempt(), linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 0)
	requireReportHeartbeat(t, f.stream)
	f.requireFailure(t, op, "no_answer", connect.CodeDeadlineExceeded, operationDeadline)
}

func testProjectStagedReport(t *testing.T) {
	f := newReportFixture(t, reportControls{dispatch: true})
	op := f.admit(t, "staged report", false)
	command := f.deliver(t, op)
	open := f.row(t, op)
	sendReportResult(t, f.stream, op.command, command.GetAttempt(), linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED, 1)
	requireReportHeartbeat(t, f.stream)
	f.requireCommand(t, op, command.GetAttempt(), "", "interrupted", new(uint64(1)))
	f.requireUnchanged(t, op, open)
	f.disconnect(t)

	f.server.Close()
	f.db.Close()
	f.db = openOperationStore(t, f.config)
	f.startServer(t, reportControls{dispatch: true})
	f.stream = f.open(t, f.peer, 0)
	f.requireCommand(t, op, command.GetAttempt(), "", "interrupted", new(uint64(1)))
	f.requireUnchanged(t, op, open)

	sendFolderReport(t, f.stream, 1, op.facts())
	f.requireAck(t, f.stream, f.machine, 1)
	f.requireCommand(t, op, command.GetAttempt(), "interrupted", "", nil)
	saved := f.requireInspect(t, op, expectedReportFacts())
	f.requireNoMembership(t)
	if counts := f.counts(t); counts.operations != 1 || counts.commands != 1 || counts.receipts != 1 {
		t.Error("Replay did not keep exactly one operation, command and receipt.")
	}

	f.disconnect(t)
	requireOperationReply(t, op.call.run(t.Context(), f.service, op.call.key), saved.reply)
}

func testProjectLateReport(t *testing.T) {
	f := newReportFixture(t, reportControls{dispatch: true})
	op := f.admit(t, "late report", false)
	command := f.deliver(t, op)
	sendReportResult(t, f.stream, op.command, command.GetAttempt(), linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 0)
	requireReportHeartbeat(t, f.stream)
	f.requireMark(t, f.machine, 0)
	failed := f.requireFailure(t, op, "no_answer", connect.CodeDeadlineExceeded, operationDeadline)

	sendFolderReport(t, f.stream, 1, op.facts())
	f.requireAck(t, f.stream, f.machine, 1)
	f.requireUnchanged(t, op, failed)
	f.requireCommand(t, op, command.GetAttempt(), "done", "", nil)
	f.requireFailure(t, op, "no_answer", connect.CodeDeadlineExceeded, operationDeadline)
	f.requireNoMembership(t)
}

func testProjectSettlementStorageError(t *testing.T) {
	f := newReportFixture(t, reportControls{dispatch: true})
	op := f.admit(t, "settlement write error", false)
	command := f.deliver(t, op)
	before := f.row(t, op)
	var streamID string
	if err := f.observer.QueryRow(t.Context(), "SELECT stream_id FROM link_connections WHERE machine_id = $1", f.machine).Scan(&streamID); err != nil {
		t.Fatal("The observer could not read the current signed stream identity.")
	}
	f.exec(t, `
CREATE FUNCTION fixture_refuse_operation_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'The fixture rejects operation settlement.'; END
$$;
CREATE TRIGGER fixture_refuse_operation_update BEFORE UPDATE ON folder_operations
FOR EACH ROW EXECUTE FUNCTION fixture_refuse_operation_update();`)
	done := linkv1.CommandOutcome_COMMAND_OUTCOME_DONE
	if err := f.state.AcceptResult(t.Context(), f.machine, streamID, &linkv1.CommandResult{
		CommandId: op.command, Attempt: command.GetAttempt(), Outcome: done,
	}); err == nil {
		t.Error("The Link result write did not return the Projects storage error.")
	}
	f.requireUnchanged(t, op, before)
	f.requireCommand(t, op, command.GetAttempt(), "", "", nil)
	f.requireMark(t, f.machine, 0)

	f.exec(t, "DROP TRIGGER fixture_refuse_operation_update ON folder_operations")
	sendReportResult(t, f.stream, op.command, command.GetAttempt(), done, 0)
	requireReportHeartbeat(t, f.stream)
	f.requireCommand(t, op, command.GetAttempt(), "done", "", nil)
	f.requireFailure(t, op, "no_answer", connect.CodeDeadlineExceeded, operationDeadline)
}

func TestProjectReportRollback(t *testing.T) {
	var fail atomic.Bool
	var callbacks, writesSeen, acks atomic.Int32
	f := newReportFixture(t, reportControls{
		dispatch: true,
		Seams: link.Seams{
			BeforeEventCommit: func(_ context.Context, _ string, number uint64) error {
				if number == 1 && fail.Load() {
					writesSeen.Store(callbacks.Load())
					return errors.New("The fixture interrupted the report commit.")
				}
				return nil
			},
			BeforeEventAck: func(context.Context, string, uint64) { acks.Add(1) },
		},
		afterReport: func(ctx context.Context, tx *store.Tx, event *linkv1.Event) error {
			var applied bool
			if err := tx.QueryRow(ctx, `
SELECT state = 'succeeded' AND reply IS NOT NULL AND failure IS NULL
FROM folder_operations WHERE id = $1`, event.GetFolderSeen().GetOperationId()).Scan(&applied); err != nil {
				return err
			}
			if applied {
				callbacks.Add(1)
			}
			return nil
		},
	})
	op := f.admit(t, "report rollback", false)
	before := f.row(t, op)
	command := f.deliver(t, op)
	sendReportResult(t, f.stream, op.command, command.GetAttempt(), linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, 1)
	requireReportHeartbeat(t, f.stream)
	f.requireCommand(t, op, command.GetAttempt(), "", "done", new(uint64(1)))

	fail.Store(true)
	sendFolderReport(t, f.stream, 1, op.facts())
	requireReportWriteFailure(t, f.stream)
	if writesSeen.Load() != 1 {
		t.Error("The pre-commit fault did not see the successful report write inside the transaction.")
	}
	if acks.Load() != 0 {
		t.Error("The failed report reached the committed acknowledgement hook.")
	}
	f.requireMark(t, f.machine, 0)
	f.requireUnchanged(t, op, before)
	f.requireCommand(t, op, command.GetAttempt(), "", "done", new(uint64(1)))
	f.requireNoMembership(t)

	fail.Store(false)
	f.stream = f.open(t, f.peer, 0)
	sendFolderReport(t, f.stream, 1, op.facts())
	f.requireAck(t, f.stream, f.machine, 1)
	saved := f.requireInspect(t, op, expectedReportFacts())
	f.requireCommand(t, op, command.GetAttempt(), "done", "", nil)
	if callbacks.Load() != 2 || acks.Load() != 1 {
		t.Error("The retry did not apply and acknowledge exactly one committed report.")
	}
	sendFolderReport(t, f.stream, 1, op.facts())
	f.requireAck(t, f.stream, f.machine, 1)
	f.requireUnchanged(t, op, saved)
	if callbacks.Load() != 2 || acks.Load() != 1 {
		t.Error("The duplicate report repeated a callback or a committed mark advance.")
	}
	f.requireNoMembership(t)
}
