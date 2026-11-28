package projects_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	operationInvalidInput = "The machine, repository, or folder path is invalid."
	operationInvalidKey   = "The request key is invalid."
	operationConflict     = "That request key was already used for a different request."
	operationUnknown      = "The machine was not found."
	operationUnavailable  = "The machine is not connected."
	operationDeadline     = "The machine did not answer the folder request."
	operationDatabase     = "The server cannot reach its database, try again."
	operationUncertain    = "The database did not confirm whether the write committed."
)

var operationUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type operationFixture struct {
	config   *pgxpool.Config
	db       *store.Store
	observer *pgx.Conn
	registry *machines.Registry
	state    *link.State
	machine  string
	stream   *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]
}

func openOperationStore(t *testing.T, config *pgxpool.Config) *store.Store {
	t.Helper()
	db, err := store.Open(t.Context(), config, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The operation fixture could not open Store.")
	}
	t.Cleanup(db.Close)
	return db
}

func newOperationFixture(t *testing.T) *operationFixture {
	t.Helper()
	config := storetest.NewDatabase(t, "project_operations_test")
	db := openOperationStore(t, config)
	if err := db.Migrate(t.Context(), auth.Migrations(), machines.Migrations(), link.Migrations(), projects.Migrations()); err != nil {
		t.Fatal("The operation fixture could not apply the server migrations.")
	}
	observer, err := pgx.ConnectConfig(t.Context(), config.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The independent operation observer could not connect.")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observer.Close(ctx)
	})

	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("The operation fixture could not generate a machine identity.")
	}
	serverPublic, serverPrivate, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("The operation fixture could not generate a server identity.")
	}
	registry := machines.NewRegistry()
	var machine machines.Machine
	if err := db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		var err error
		machine, err = registry.CreateWaiting(ctx, tx, machines.NewMachine{PublicKey: public, HostName: "Operation fixture"})
		return err
	}); err != nil {
		t.Fatal("The operation fixture could not register its machine.")
	}

	hub := link.NewHub()
	state := link.NewState(db, registry, hub)
	// Keep a signed stream without running the command dispatcher or sweep.
	streams := link.NewStreamingService(t.Context(), db, nil, registry, state, hub, serverPrivate,
		link.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	_, handler := linkv1connect.NewLinkServiceHandler(streams)
	server := machinetest.NewServer(t, handler, 5*time.Second)
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	httpClient := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	t.Cleanup(httpClient.CloseIdleConnections)
	client := linkv1connect.NewLinkServiceClient(httpClient, server.URL, connect.WithGRPC())
	peer := &machinetest.Machine{
		ID: machine.ID, Key: private, ServerKey: serverPublic, RunID: projectMigrationID(t), FirstEventNumber: 1,
	}
	fixture := &operationFixture{config: config, db: db, observer: observer, registry: registry, state: state, machine: machine.ID}
	fixture.connect(t, peer, client)
	return fixture
}

func (f *operationFixture) connect(t *testing.T, peer *machinetest.Machine, client linkv1connect.LinkServiceClient) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	stream, _, err := peer.Connect(ctx, client)
	if err != nil {
		t.Fatalf("The signed fixture machine could not open its Link stream. The code was %v.", connect.CodeOf(err))
	}
	// The first heartbeat proves that the signed stream has finished its welcome.
	heartbeat := &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}}
	if err := stream.Send(heartbeat); err != nil {
		t.Fatal("The signed fixture machine could not send a heartbeat.")
	}
	reply, err := stream.Receive()
	if err != nil || reply.GetHeartbeatAck() == nil {
		t.Fatal("The signed fixture machine did not receive its heartbeat acknowledgement.")
	}
	t.Cleanup(func() {
		cancel()
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	})
	f.stream = stream
	f.awaitPresence(t, true)
}

func (f *operationFixture) disconnect(t *testing.T) {
	t.Helper()
	if err := f.stream.CloseRequest(); err != nil {
		t.Fatal("The signed fixture machine could not end its request stream.")
	}
	if _, err := f.stream.Receive(); !errors.Is(err, io.EOF) {
		t.Fatalf("The signed fixture stream did not finish normally. The receive code was %v.", connect.CodeOf(err))
	}
	f.awaitPresence(t, false)
}

func (f *operationFixture) awaitPresence(t *testing.T, want bool) {
	t.Helper()
	storetest.Poll(t, "the fixture Link presence", func() bool {
		var rows map[string]machines.Presence
		if err := f.db.Read(t.Context(), func(ctx context.Context, tx *store.ReadTx) error {
			var err error
			rows, err = f.state.Presence(ctx, tx, []string{f.machine})
			return err
		}); err != nil {
			t.Fatal("The fixture could not read Link presence.")
		}
		return rows[f.machine].Held == want
	})
}

func (f *operationFixture) approve(t *testing.T) {
	t.Helper()
	if err := f.db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.registry.SetApproved(ctx, tx, f.machine, "Operation fixture")
	}); err != nil {
		t.Fatal("The operation fixture could not approve its machine.")
	}
}

func (f *operationFixture) service(t *testing.T, db *store.Store, commands projects.CommandLink) *projects.Service {
	t.Helper()
	return projects.NewService(db, auth.NewSessions(db, auth.Password{}), f.registry, commands, nil, t.Context())
}

type operationCall struct {
	key, machine, path string
	target             *string
	add                bool
}

func (c operationCall) procedure() string {
	if c.add {
		return lgtmv1connect.ProjectServiceAddRepositoryProcedure
	}
	return lgtmv1connect.ProjectServiceInspectFolderProcedure
}

type operationResult struct {
	reply proto.Message
	err   error
}

func (c operationCall) run(ctx context.Context, s *projects.Service, keys ...string) operationResult {
	if c.add {
		req := connect.NewRequest(&webv1.AddRepositoryRequest{ClientId: c.machine, Path: c.path, RepositoryId: c.target})
		for _, key := range keys {
			req.Header().Add(store.RequestKeyHeader, key)
		}
		res, err := s.AddRepository(ctx, req)
		if res == nil || res.Msg == nil {
			return operationResult{err: err}
		}
		return operationResult{reply: res.Msg, err: err}
	}
	req := connect.NewRequest(&webv1.InspectFolderRequest{ClientId: c.machine, Path: c.path})
	for _, key := range keys {
		req.Header().Add(store.RequestKeyHeader, key)
	}
	res, err := s.InspectFolder(ctx, req)
	if res == nil || res.Msg == nil {
		return operationResult{err: err}
	}
	return operationResult{reply: res.Msg, err: err}
}

func (c operationCall) start(t *testing.T, ctx context.Context, s *projects.Service) *storetest.Task[operationResult] {
	t.Helper()
	return storetest.Start(t, func() operationResult { return c.run(ctx, s, c.key) })
}

func requireOperationRefusal(t *testing.T, got operationResult, code connect.Code, sentences ...string) {
	t.Helper()
	var refusal *connect.Error
	if got.reply != nil || !errors.As(got.err, &refusal) || refusal.Code() != code {
		t.Fatalf("Expected no reply and the %s refusal.", code)
	}
	for _, sentence := range sentences {
		if refusal.Message() == sentence {
			return
		}
	}
	if len(sentences) != 0 {
		t.Fatalf("The %s refusal did not use the fixed display sentence.", code)
	}
}

// readOperationStart observes a full committed acceptance through one independent lookup.
func (f *operationFixture) readOperationStart(t *testing.T, c operationCall) (string, string, bool) {
	t.Helper()
	var id, commandID, machine, kind, action string
	var target *string
	var payload, receipt, principal, digest []byte
	var held, open, pending bool
	var version int16
	err := f.observer.QueryRow(t.Context(), `
SELECT o.id, o.command_id, o.machine_id, o.kind, o.target_repository_id,
       o.state = 'open' AND o.reply IS NULL AND o.failure IS NULL AND o.retention_started_at IS NULL,
       c.action, c.payload, c.expires_at - c.created_at = interval '20 seconds'
       AND c.machine_id = o.machine_id AND c.outcome IS NULL AND c.attempt = 1
       AND c.sent_stream_id IS NULL AND NOT c.dispatched,
       r.reply, r.held, r.principal_hash, r.request_hash, r.fingerprint_version
FROM folder_operations o
JOIN link_commands c ON c.id = o.command_id
JOIN receipts r ON r.request_key = o.request_key
WHERE o.request_key = $1`, c.key).Scan(&id, &commandID, &machine, &kind, &target, &open,
		&action, &payload, &pending, &receipt, &held, &principal, &digest, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false
	}
	if err != nil {
		t.Fatal("The observer could not read the operation admission.")
	}
	wantKind := "inspect"
	fields := []string{c.machine, c.path}
	if c.add {
		wantKind = "add"
		fields = append(fields, "")
		if c.target != nil {
			fields[2] = *c.target
		}
	}
	if machine != c.machine || kind != wantKind || !open ||
		(target == nil) != (c.target == nil) || (target != nil && *target != *c.target) {
		t.Error("The operation did not preserve the requested machine, kind, target and open state.")
	}
	wantCommand := &linkv1.Command{Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
		OperationId: id, Path: c.path,
	}}}
	var command linkv1.Command
	if action != "look_at_folder" || !pending || proto.Unmarshal(payload, &command) != nil || !proto.Equal(&command, wantCommand) {
		t.Error("The command did not preserve the exact folder action and pending 20-second expiry.")
	}
	if !operationUUID.MatchString(id) || !operationUUID.MatchString(commandID) || !held || !bytes.Equal(receipt, []byte(id)) || version != 1 {
		t.Error("The held version-one receipt did not contain only the operation identifier.")
	}
	wantPrincipal := sha256.Sum256([]byte("owner"))
	wantDigest := operationRequestHash(c.procedure(), fields)
	if !bytes.Equal(principal, wantPrincipal[:]) || !bytes.Equal(digest, wantDigest[:]) {
		t.Error("The receipt did not use the caller-wide owner identity and version-one field framing.")
	}
	return id, commandID, true
}

func operationRequestHash(procedure string, fields []string) [sha256.Size]byte {
	var fingerprint []byte
	for _, field := range fields {
		fingerprint = binary.BigEndian.AppendUint64(fingerprint, uint64(len(field)))
		fingerprint = append(fingerprint, field...)
	}
	framed := binary.BigEndian.AppendUint16(nil, 1)
	for _, part := range [][]byte{[]byte("owner"), []byte(procedure), fingerprint} {
		framed = binary.BigEndian.AppendUint64(framed, uint64(len(part)))
		framed = append(framed, part...)
	}
	return sha256.Sum256(framed)
}

func (f *operationFixture) awaitOperation(t *testing.T, c operationCall, task *storetest.Task[operationResult]) (id, commandID string) {
	t.Helper()
	storetest.Poll(t, "the operation, command and receipt becoming visible together", func() bool {
		var found bool
		id, commandID, found = f.readOperationStart(t, c)
		if !found {
			select {
			case <-task.Done():
				t.Fatal("The call returned before its operation, command and held receipt became visible.")
			default:
			}
		}
		return found
	})
	return id, commandID
}

type operationCounts struct{ operations, commands, receipts, wakes int }

func (f *operationFixture) counts(t *testing.T) operationCounts {
	t.Helper()
	var counts operationCounts
	if err := f.observer.QueryRow(t.Context(), `
SELECT (SELECT count(*) FROM folder_operations),
       (SELECT count(*) FROM link_commands),
       (SELECT count(*) FROM receipts),
       (SELECT count(*) FROM feed WHERE topic = 'machine_outbox' AND scope = '' AND key = $1 AND change = 'saved')`,
		f.machine).Scan(&counts.operations, &counts.commands, &counts.receipts, &counts.wakes); err != nil {
		t.Fatal("The observer could not count operation acceptance effects.")
	}
	return counts
}

func (f *operationFixture) requireCounts(t *testing.T, want operationCounts) {
	t.Helper()
	if got := f.counts(t); got != want {
		t.Errorf("The operation, command, held receipt and outbox wake counts were %+v. Want %+v.", got, want)
	}
}

func (f *operationFixture) settleFixtureReply(t *testing.T, id string, reply proto.Message) []byte {
	t.Helper()
	// Preserve an unknown field so replay must return the complete stored message.
	reply.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 88, protowire.VarintType), 42))
	data, err := proto.Marshal(reply)
	if err != nil {
		t.Fatal("The fixture could not encode the terminal reply.")
	}
	tag, err := f.observer.Exec(t.Context(), `UPDATE folder_operations SET state = 'succeeded', reply = $2
WHERE id = $1 AND state = 'open'`, id, data)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal("The fixture could not settle exactly one open operation.")
	}
	return data
}

func inspectOperationReply() *webv1.InspectFolderResponse {
	return &webv1.InspectFolderResponse{Facts: &webv1.FolderFacts{
		ResolvedPath: "/workspace/checkout", Name: "checkout", MainBranch: "main",
		CheckedOutBranch: new("work"), Remote: new("https://example.com/checkout"),
		FirstCommit: new("first-commit"),
	}}
}

func requireOperationReply(t *testing.T, result operationResult, want []byte) {
	t.Helper()
	if result.err != nil || result.reply == nil {
		t.Fatal("The call did not return the stored operation reply.")
	}
	got, err := proto.Marshal(result.reply)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("The call did not return the complete original reply bytes.")
	}
}

func TestProjectOperationAtomicStart(t *testing.T) {
	t.Run("validation precedes admission", testOperationValidation)
	t.Run("expiry and exact path boundaries", testOperationPathBoundaries)
	t.Run("fault after enqueue", testOperationEnqueueRollback)
	t.Run("one key races", func(t *testing.T) { testOperationRace(t, true) })
	t.Run("distinct keys share the machine lock", func(t *testing.T) { testOperationRace(t, false) })
	t.Run("changed request conflicts", testOperationConflicts)
}

type operationQueryTrace func(context.Context, *pgx.Conn, pgx.TraceQueryStartData)

func (trace operationQueryTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	trace(ctx, conn, data)
	return ctx
}

func (operationQueryTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func testOperationValidation(t *testing.T) {
	f := newOperationFixture(t)
	f.approve(t)
	var queries atomic.Int64
	config := f.config.Copy()
	config.ConnConfig.Tracer = operationQueryTrace(func(context.Context, *pgx.Conn, pgx.TraceQueryStartData) {
		queries.Add(1)
	})
	s := f.service(t, openOperationStore(t, config), f.state)
	valid := operationCall{machine: f.machine, path: "/workspace/checkout"}
	const key = "validation-fixture"

	invalidInputs := []struct {
		name string
		call operationCall
	}{
		{"empty machine", operationCall{path: valid.path}},
		{"malformed machine", operationCall{machine: "broken-id", path: valid.path}},
		{"uppercase machine", operationCall{machine: "019AE900-0000-7aba-8000-000000000000", path: valid.path}},
		{"wrong machine version", operationCall{machine: "019ae900-0000-4aba-8000-000000000000", path: valid.path}},
		{"wrong machine variant", operationCall{machine: "019ae900-0000-7aba-7000-000000000000", path: valid.path}},
		{"present empty target", operationCall{machine: f.machine, path: valid.path, add: true, target: new("")}},
		{"malformed target", operationCall{machine: f.machine, path: valid.path, add: true, target: new("broken-id")}},
		{"uppercase target", operationCall{machine: f.machine, path: valid.path, add: true, target: new("019AE900-0000-7aba-8000-000000000000")}},
		{"wrong target version", operationCall{machine: f.machine, path: valid.path, add: true, target: new("019ae900-0000-4aba-8000-000000000000")}},
		{"wrong target variant", operationCall{machine: f.machine, path: valid.path, add: true, target: new("019ae900-0000-7aba-c000-000000000000")}},
	}
	for _, path := range []struct{ name, value string }{
		{"empty path", ""},
		{"relative path", "workspace/checkout"},
		{"named-user home path", "~other/checkout"},
		{"leading whitespace", " /workspace/checkout"},
		{"path with a zero byte", "/workspace/\x00checkout"},
		{"path with invalid text", "/workspace/\xffcheckout"},
		{"path over the byte limit", "/" + strings.Repeat("é", 2048)},
	} {
		call := valid
		call.path = path.value
		invalidInputs = append(invalidInputs, struct {
			name string
			call operationCall
		}{path.name, call})
	}
	for _, tc := range invalidInputs {
		t.Run(tc.name, func(t *testing.T) {
			before := queries.Load()
			requireOperationRefusal(t, tc.call.run(t.Context(), s, key), connect.CodeInvalidArgument, operationInvalidInput)
			if queries.Load() != before {
				t.Error("Invalid input reached the database before refusal.")
			}
		})
	}
	for _, tc := range []struct {
		name string
		keys []string
	}{
		{"missing key", nil},
		{"empty key", []string{""}},
		{"same header repeated", []string{key, key}},
		{"different headers repeated", []string{key, "other-fixture"}},
		{"empty header followed by another", []string{"", key}},
		{"key over the byte limit", []string{strings.Repeat("é", 128) + "a"}},
		{"key with a zero byte", []string{"invalid\x00key"}},
		{"key with invalid text", []string{"invalid\xffkey"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := queries.Load()
			requireOperationRefusal(t, valid.run(t.Context(), s, tc.keys...), connect.CodeInvalidArgument, operationInvalidKey)
			if queries.Load() != before {
				t.Error("An invalid request key reached the database before refusal.")
			}
		})
	}
	f.requireCounts(t, operationCounts{})

	missing := valid
	missing.machine = projectMigrationID(t)
	requireOperationRefusal(t, missing.run(t.Context(), s, key), connect.CodeNotFound, operationUnknown)
	f.requireCounts(t, operationCounts{})
}

func testOperationPathBoundaries(t *testing.T) {
	f := newOperationFixture(t)
	f.approve(t)
	s := f.service(t, f.db, f.state)
	for index, tc := range []struct{ name, key, path string }{
		{"exact byte limits", strings.Repeat("é", 128), "/" + strings.Repeat("é", 2046) + "/. "},
		{"home marker", "home-marker", "~"},
		{"home slash", "home-slash", "~/"},
		{"significant path bytes", "exact-path", "~/a//./b/../ checkout "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := operationCall{key: tc.key, machine: f.machine, path: tc.path}
			task := call.start(t, t.Context(), s)
			id, _ := f.awaitOperation(t, call, task)
			count := index + 1
			f.requireCounts(t, operationCounts{count, count, count, count})
			want := f.settleFixtureReply(t, id, inspectOperationReply())
			requireOperationReply(t, task.Wait(t), want)
		})
	}
}

func testOperationEnqueueRollback(t *testing.T) {
	f := newOperationFixture(t)
	f.approve(t)
	s := f.service(t, f.db, f.state)
	call := operationCall{key: "rollback-fixture", machine: f.machine, path: "/workspace/fixture"}
	storetest.Exec(t, f.observer, `
CREATE SEQUENCE operation_post_enqueue;
CREATE FUNCTION operation_refuse_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT FROM link_commands WHERE id = NEW.command_id AND action = 'look_at_folder') THEN
        PERFORM nextval('operation_post_enqueue');
    END IF;
    RAISE EXCEPTION 'Synthetic operation diagnostic.' USING ERRCODE = '23514';
END;
$$;
CREATE TRIGGER operation_refuse_insert BEFORE INSERT ON folder_operations
FOR EACH ROW EXECUTE FUNCTION operation_refuse_insert();`)
	request := connect.NewRequest(&webv1.InspectFolderRequest{ClientId: call.machine, Path: call.path})
	const cookie = "synthetic-operation-cookie"
	request.Header().Set("Cookie", auth.CookieName+"="+cookie)
	request.Header().Set(store.RequestKeyHeader, call.key)
	var logs operationLogs
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	reply, err := s.InspectFolder(t.Context(), request)
	slog.SetDefault(previous)
	if reply != nil {
		t.Error("The failed operation returned a response.")
	}
	requireOperationRefusal(t, operationResult{err: err}, connect.CodeUnavailable, operationDatabase)
	if logs.containsAny(call.path, call.key, cookie, "Synthetic operation diagnostic.") {
		t.Error("The failed operation logged protected request data or a database diagnostic.")
	}
	if !storetest.Scalar[bool](t, f.observer, "SELECT is_called FROM operation_post_enqueue") {
		t.Error("The insertion fault did not observe Link's queued command in the failing write.")
	}
	f.requireCounts(t, operationCounts{})

	storetest.Exec(t, f.observer, "DROP TRIGGER operation_refuse_insert ON folder_operations")
	task := call.start(t, t.Context(), s)
	id, _ := f.awaitOperation(t, call, task)
	want := f.settleFixtureReply(t, id, inspectOperationReply())
	requireOperationReply(t, task.Wait(t), want)
	f.requireCounts(t, operationCounts{1, 1, 1, 1})
}

type operationLogs struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (logs *operationLogs) Write(data []byte) (int, error) {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	return logs.buffer.Write(data)
}

func (logs *operationLogs) containsAny(values ...string) bool {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	for _, value := range values {
		if bytes.Contains(logs.buffer.Bytes(), []byte(value)) {
			return true
		}
	}
	return false
}

type operationEnqueueHold struct {
	*link.State
	firstPath string
	entered   chan<- int32
	release   <-chan struct{}
}

func (hold operationEnqueueHold) EnqueueCommand(ctx context.Context, tx *store.Tx, machine string, command *linkv1.Command, ttl time.Duration) (string, error) {
	id, err := hold.State.EnqueueCommand(ctx, tx, machine, command, ttl)
	if err != nil || command.GetLookAtFolder().GetPath() != hold.firstPath {
		return id, err
	}
	var pid int32
	if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		return "", err
	}
	select {
	case hold.entered <- pid:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	select {
	case <-hold.release:
		return id, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func testOperationRace(t *testing.T, sameKey bool) {
	f := newOperationFixture(t)
	f.approve(t)
	first := operationCall{key: "concurrent-first", machine: f.machine, path: "/workspace/first"}
	second := first
	blockedOn := "%pg_advisory_xact_lock%"
	if !sameKey {
		second.key, second.path = "concurrent-second", "/workspace/second"
		blockedOn = "%for no key update%"
	}
	release, unhold := storetest.Signal(t)
	defer unhold()
	entered := make(chan int32, 1)
	s := f.service(t, f.db, operationEnqueueHold{f.state, first.path, entered, release})
	a := first.start(t, t.Context(), s)
	var blocker int32
	select {
	case blocker = <-entered:
	case <-a.Done():
		t.Fatal("The first call returned before reaching the enqueue barrier.")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("The first call did not reach the enqueue barrier.")
	}
	f.requireCounts(t, operationCounts{})
	b := second.start(t, t.Context(), s)
	storetest.Poll(t, "the second call waiting on the first call's lock", func() bool {
		select {
		case <-b.Done():
			t.Fatal("The second call returned while the first call still held the lock.")
		default:
		}
		return storetest.Scalar[bool](t, f.observer, `
SELECT EXISTS (SELECT FROM pg_stat_activity
    WHERE datname = current_database() AND wait_event_type = 'Lock'
      AND lower(query) LIKE $2 AND $1 = ANY(pg_blocking_pids(pid)))`, blocker, blockedOn)
	})
	f.requireCounts(t, operationCounts{})
	unhold()

	operationA, commandA := f.awaitOperation(t, first, a)
	wantA := f.settleFixtureReply(t, operationA, inspectOperationReply())
	requireOperationReply(t, a.Wait(t), wantA)
	if sameKey {
		requireOperationReply(t, b.Wait(t), wantA)
		f.requireCounts(t, operationCounts{1, 1, 1, 1})
		return
	}
	operationB, commandB := f.awaitOperation(t, second, b)
	if operationA == operationB || commandA == commandB || !storetest.Scalar[bool](t, f.observer, `
SELECT a.seq < b.seq FROM link_commands a CROSS JOIN link_commands b
WHERE a.id = $1 AND b.id = $2`, commandA, commandB) {
		t.Error("The command queue did not preserve the order of the two machine writes.")
	}
	wantB := f.settleFixtureReply(t, operationB, inspectOperationReply())
	requireOperationReply(t, b.Wait(t), wantB)
	f.requireCounts(t, operationCounts{2, 2, 2, 2})
}

func testOperationConflicts(t *testing.T) {
	f := newOperationFixture(t)
	f.approve(t)
	s := f.service(t, f.db, f.state)
	original := operationCall{add: true, key: "conflict-fixture", machine: f.machine, path: "/workspace/checkout "}
	task := original.start(t, t.Context(), s)
	id, _ := f.awaitOperation(t, original, task)
	for _, tc := range []struct {
		name   string
		change func(*operationCall)
	}{
		{"changed exact path", func(call *operationCall) { call.path = "/workspace/checkout" }},
		{"changed procedure", func(call *operationCall) { call.add = false }},
		{"changed target", func(call *operationCall) { call.target = new(projectMigrationID(t)) }},
		{"changed machine", func(call *operationCall) { call.machine = projectMigrationID(t) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := original
			tc.change(&call)
			requireOperationRefusal(t, call.run(t.Context(), s, call.key), connect.CodeAlreadyExists, operationConflict)
		})
	}
	// Machines shares the caller-wide key space with Projects.
	request := connect.NewRequest(&webv1.RenameClientRequest{Id: f.machine, Name: "Renamed fixture"})
	request.Header().Set(store.RequestKeyHeader, original.key)
	machineService := machines.NewService(f.db, nil, f.registry, nil)
	response, err := machineService.RenameClient(t.Context(), request)
	if response != nil {
		t.Error("The Machines call returned a response for a key held by Projects.")
	}
	requireOperationRefusal(t, operationResult{err: err}, connect.CodeAlreadyExists, operationConflict)
	f.requireCounts(t, operationCounts{1, 1, 1, 1})

	want := f.settleFixtureReply(t, id, addOperationReply(t, f.machine, projectMigrationID(t)))
	requireOperationReply(t, task.Wait(t), want)
}

func addOperationReply(t *testing.T, machine, repository string) *webv1.AddRepositoryResponse {
	t.Helper()
	folder := &webv1.RepositoryFolder{Id: projectMigrationID(t), ClientId: machine, ResolvedPath: "/workspace/checkout"}
	return &webv1.AddRepositoryResponse{
		Repository: &webv1.Repository{Id: repository, Name: "checkout", MainBranch: "main", Folders: []*webv1.RepositoryFolder{folder}},
		Folder:     folder,
	}
}

func TestProjectOperationReplayAfterRestart(t *testing.T) {
	for _, tc := range []struct {
		name string
		add  bool
	}{{"inspect", false}, {"add to the selected repository", true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOperationFixture(t)
			f.approve(t)
			call := operationCall{key: "restart-fixture", machine: f.machine, path: "~/checkout ", add: tc.add}
			var reply proto.Message = inspectOperationReply()
			if tc.add {
				call.target = new(projectMigrationID(t))
				reply = addOperationReply(t, f.machine, *call.target)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			task := call.start(t, ctx, f.service(t, f.db, f.state))
			operationID, commandID := f.awaitOperation(t, call, task)
			f.disconnect(t)
			cancel()
			requireOperationRefusal(t, task.Wait(t), connect.CodeCanceled)

			f.db.Close()
			db := openOperationStore(t, f.config)
			f.registry = machines.NewRegistry()
			s := f.service(t, db, link.NewState(db, f.registry, link.NewHub()))
			want := f.settleFixtureReply(t, operationID, reply)
			requireOperationReply(t, call.run(t.Context(), s, call.key), want)
			f.requireTerminalReply(t, operationID, commandID, want)
			f.requireCounts(t, operationCounts{1, 1, 1, 1})

			changed := call
			changed.path = "~/checkout"
			requireOperationRefusal(t, changed.run(t.Context(), s, changed.key), connect.CodeAlreadyExists, operationConflict)
		})
	}
	t.Run("lost reply for committed acceptance", func(t *testing.T) { testOperationLostCommit(t, false) })
	t.Run("lost reply for rolled-back acceptance", func(t *testing.T) { testOperationLostCommit(t, true) })
}

func (f *operationFixture) requireTerminalReply(t *testing.T, id, commandID string, reply []byte) {
	t.Helper()
	if !storetest.Scalar[bool](t, f.observer, `
SELECT EXISTS (SELECT FROM folder_operations o
    JOIN receipts r ON r.request_key = o.request_key
    WHERE o.id = $1 AND o.command_id = $2 AND o.state = 'succeeded' AND o.reply = $3
      AND r.held AND r.reply = convert_to(o.id, 'UTF8'))`, id, commandID, reply) {
		t.Error("The durable operation, command and held receipt did not retain the original reply and identities.")
	}
}

// operationCommitWire drops the first reply to the acceptance COMMIT after PostgreSQL answers.
type operationCommitWire struct {
	net.Conn
	armed   *atomic.Bool
	dropped *atomic.Bool
	abort   bool
	pending atomic.Bool
}

func (wire *operationCommitWire) Write(frame []byte) (int, error) {
	if len(frame) == 12 && frame[0] == 'Q' && bytes.EqualFold(frame[5:], []byte("commit\x00")) &&
		wire.armed.CompareAndSwap(true, false) {
		if wire.abort {
			// ABORT and COMMIT have the same width in the simple-query frame.
			frame = bytes.Clone(frame)
			copy(frame[5:], "abort \x00")
		}
		wire.pending.Store(true)
	}
	return wire.Conn.Write(frame)
}

func (wire *operationCommitWire) Read(buffer []byte) (int, error) {
	n, err := wire.Conn.Read(buffer)
	if n > 0 && wire.pending.CompareAndSwap(true, false) {
		wire.dropped.Store(true)
		return 0, io.ErrUnexpectedEOF
	}
	return n, err
}

func testOperationLostCommit(t *testing.T, rollback bool) {
	f := newOperationFixture(t)
	f.approve(t)
	config := f.config.Copy()
	var armed, dropped atomic.Bool
	dial := config.ConnConfig.DialFunc
	config.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &operationCommitWire{Conn: conn, armed: &armed, dropped: &dropped, abort: rollback}, nil
	}
	db := openOperationStore(t, config)
	s := f.service(t, db, f.state)
	call := operationCall{key: "lost-commit-fixture", machine: f.machine, path: "/workspace/checkout "}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	armed.Store(true)
	result := call.run(ctx, s, call.key)
	if !dropped.Load() {
		t.Error("The fixture did not lose the acceptance commit reply.")
	}
	requireOperationRefusal(t, result, connect.CodeUnavailable, operationDatabase, operationUncertain)
	db.Close()

	db = openOperationStore(t, f.config)
	var operationID, commandID string
	var task *storetest.Task[operationResult]
	if rollback {
		f.requireCounts(t, operationCounts{})
		// Keep the live Link for the fresh admission on the reopened Store.
		s = f.service(t, db, f.state)
		task = call.start(t, t.Context(), s)
		operationID, commandID = f.awaitOperation(t, call, task)
	} else {
		f.requireCounts(t, operationCounts{1, 1, 1, 1})
		var found bool
		operationID, commandID, found = f.readOperationStart(t, call)
		if !found {
			t.Fatal("The accepted operation was missing after its commit reply was lost.")
		}
		f.disconnect(t)
		s = f.service(t, db, link.NewState(db, f.registry, link.NewHub()))
	}
	want := f.settleFixtureReply(t, operationID, inspectOperationReply())
	if task != nil {
		result = task.Wait(t)
	} else {
		result = call.run(t.Context(), s, call.key)
	}
	requireOperationReply(t, result, want)
	f.requireTerminalReply(t, operationID, commandID, want)
	f.requireCounts(t, operationCounts{1, 1, 1, 1})
}

func TestProjectOperationDeadlineAndDisconnect(t *testing.T) {
	t.Run("open wait releases the connection", testOperationIndependentWrite)
	t.Run("disconnect leaves the row open at the server deadline", testOperationWaitDeadline)
	t.Run("caller cancellation and deadline preserve replay", testOperationCallerEnd)
	t.Run("fresh requests need approval and held presence", testOperationUnavailable)
}

func testOperationIndependentWrite(t *testing.T) {
	f := newOperationFixture(t)
	f.approve(t)
	config := f.config.Copy()
	config.MaxConns = 8
	reads := make(chan time.Time, 2)
	config.ConnConfig.Tracer = operationQueryTrace(func(_ context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) {
		sql := strings.ToLower(strings.TrimSpace(query.SQL))
		if (strings.HasPrefix(sql, "select") || strings.HasPrefix(sql, "with")) && strings.Contains(sql, "folder_operations") {
			select {
			case reads <- time.Now():
			default:
			}
		}
	})
	db := openOperationStore(t, config)

	// Use seven of the eight pool connections so the wait and the writer must share one.
	hold, release := storetest.Signal(t)
	defer release()
	readers := make(chan struct{}, 7)
	var tasks []*storetest.Task[error]
	for range cap(readers) {
		tasks = append(tasks, storetest.Start(t, func() error {
			return db.Read(t.Context(), func(ctx context.Context, tx *store.ReadTx) error {
				readers <- struct{}{}
				select {
				case <-hold:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}))
	}
	for range cap(readers) {
		storetest.Receive(t, "an occupied Store connection", readers)
	}

	call := operationCall{key: "wait-write-fixture", machine: f.machine, path: "/workspace/checkout"}
	s := f.service(t, db, f.state)
	task := call.start(t, t.Context(), s)
	id, _ := f.awaitOperation(t, call, task)
	first := storetest.Receive(t, "the immediate operation read", reads)
	second := storetest.Receive(t, "the next polling read", reads)
	// Check the cadence with room for scheduling and query latency under concurrent runs.
	if gap := second.Sub(first); gap < 150*time.Millisecond || gap > 900*time.Millisecond {
		t.Errorf("The next operation read was %v later. Want a 250-millisecond polling interval.", gap)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := db.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.registry.Lock(ctx, tx, f.machine); err != nil {
			return err
		}
		return f.registry.Touch(ctx, tx, f.machine)
	}); err != nil {
		t.Fatal("An independent write could not acquire the connection and the machine lock while Projects waited.")
	}
	select {
	case <-task.Done():
		t.Fatal("The call returned before its open operation was settled.")
	default:
	}
	want := f.settleFixtureReply(t, id, inspectOperationReply())
	requireOperationReply(t, task.Wait(t), want)
	release()
	for _, reader := range tasks {
		if reader.Wait(t) != nil {
			t.Error("A held fixture reader failed to finish.")
		}
	}
}

func testOperationWaitDeadline(t *testing.T) {
	f := newOperationFixture(t)
	f.approve(t)
	s := f.service(t, f.db, f.state)
	call := operationCall{key: "server-deadline-fixture", machine: f.machine, path: "/workspace/checkout"}
	// Leave enough room for the service to answer before the caller's deadline.
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	started := time.Now()
	task := call.start(t, ctx, s)
	id, _ := f.awaitOperation(t, call, task)
	f.disconnect(t)
	select {
	case <-task.Done():
	case <-ctx.Done():
		t.Fatal("The caller's deadline elapsed before the Projects deadline.")
	}
	if elapsed := time.Since(started); elapsed < 24500*time.Millisecond || elapsed > 32*time.Second {
		t.Errorf("The call waited %v. Want the service to end its open wait after 25 seconds.", elapsed)
	}
	requireOperationRefusal(t, task.Result(), connect.CodeDeadlineExceeded, operationDeadline)
	if !storetest.Scalar[bool](t, f.observer, `
SELECT EXISTS (SELECT FROM folder_operations o JOIN link_commands c ON c.id = o.command_id
    WHERE o.id = $1 AND o.state = 'open' AND o.reply IS NULL AND o.failure IS NULL AND c.outcome IS NULL)`, id) {
		t.Fatal("The unanswered call changed its open operation or finished its Link command.")
	}
	f.requireCounts(t, operationCounts{1, 1, 1, 1})
	if _, err := f.observer.Exec(t.Context(), `
UPDATE folder_operations SET state = 'failed', failure = 'not_a_git_repository' WHERE id = $1`, id); err != nil {
		t.Fatal("The fixture could not save a terminal refusal.")
	}
	requireOperationRefusal(t, call.run(t.Context(), s, call.key), connect.CodeFailedPrecondition, "The folder is not a Git repository.")
	f.requireCounts(t, operationCounts{1, 1, 1, 1})
}

func testOperationCallerEnd(t *testing.T) {
	f := newOperationFixture(t)
	f.approve(t)
	s := f.service(t, f.db, f.state)
	for index, tc := range []struct {
		name string
		code connect.Code
	}{
		{"cancel after admission", connect.CodeCanceled},
		{"earlier caller deadline", connect.CodeDeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ctx context.Context
			var cancel context.CancelFunc
			if tc.code == connect.CodeCanceled {
				ctx, cancel = context.WithCancel(t.Context())
			} else {
				ctx, cancel = context.WithTimeout(t.Context(), 2*time.Second)
			}
			defer cancel()
			call := operationCall{key: projectMigrationID(t), machine: f.machine, path: "/workspace/checkout"}
			task := call.start(t, ctx, s)
			id, _ := f.awaitOperation(t, call, task)
			if tc.code == connect.CodeCanceled {
				cancel()
			}
			requireOperationRefusal(t, task.Wait(t), tc.code)
			want := f.settleFixtureReply(t, id, inspectOperationReply())
			requireOperationReply(t, call.run(t.Context(), s, call.key), want)
			count := index + 1
			f.requireCounts(t, operationCounts{count, count, count, count})
		})
	}
}

func testOperationUnavailable(t *testing.T) {
	f := newOperationFixture(t)
	s := f.service(t, f.db, f.state)
	call := operationCall{key: "available-after-approval", machine: f.machine, path: "/workspace/checkout"}
	requireOperationRefusal(t, call.run(t.Context(), s, call.key), connect.CodeUnavailable, operationUnavailable)
	f.requireCounts(t, operationCounts{})
	f.approve(t)
	task := call.start(t, t.Context(), s)
	id, _ := f.awaitOperation(t, call, task)
	want := f.settleFixtureReply(t, id, inspectOperationReply())
	requireOperationReply(t, task.Wait(t), want)

	f.disconnect(t)
	fresh := call
	fresh.key = "disconnected-fixture"
	requireOperationRefusal(t, fresh.run(t.Context(), s, fresh.key), connect.CodeUnavailable, operationUnavailable)
	f.requireCounts(t, operationCounts{1, 1, 1, 1})
}
