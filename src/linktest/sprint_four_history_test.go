package linktest

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func TestSprintFourHistoryRecovery(t *testing.T) {
	requirePostgres(t)
	t.Run("RevisionOneRefusal", testRevisionOneRefusal)
	t.Run("WarningSurvivesRestart", func(t *testing.T) {
		t.Run("WithoutStagedResult", func(t *testing.T) { testPairedRecoveryWarning(t, false, false) })
		t.Run("WithStagedResult", func(t *testing.T) { testPairedRecoveryWarning(t, true, false) })
	})
	t.Run("RemoveInstead", func(t *testing.T) { testPairedRecoveryWarning(t, true, true) })
	t.Run("DiagnosticCommit", func(t *testing.T) {
		t.Run("BeforeDelivery", func(t *testing.T) { testPairedDiagnosticCommit(t, false) })
		t.Run("AfterCommit", func(t *testing.T) { testPairedDiagnosticCommit(t, true) })
	})
	t.Run("ClientStop", func(t *testing.T) {
		for _, scenario := range []string{"MissingHistory", "CorruptRequiredPayload", "IncompatibleWelcomeMark"} {
			t.Run(scenario, func(t *testing.T) { testPairedHistoryStop(t, scenario) })
		}
	})
}

func pairedMachineWatch(t *testing.T, page lgtmv1connect.ClientServiceClient) *connect.ServerStreamForClient[webv1.WatchClientsResponse] {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	watch, err := page.WatchClients(ctx, connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err != nil {
		t.Fatal("The paired machine watch did not open.")
	}
	t.Cleanup(func() { _ = watch.Close() })
	return watch
}

func requirePairedWarningSnapshot(t *testing.T, watch *connect.ServerStreamForClient[webv1.WatchClientsResponse], id string, want bool) {
	t.Helper()
	if !watch.Receive() {
		t.Fatal("The paired machine watch did not return its initial snapshot.")
	}
	for _, row := range watch.Msg().GetAll().GetClients() {
		if row.GetId() == id && row.GetEventHistoryNeedsRecovery() == want {
			return
		}
	}
	t.Fatalf("The machine snapshot did not show the fixture warning as %t.", want)
}

func requirePairedWarningChange(t *testing.T, watch *connect.ServerStreamForClient[webv1.WatchClientsResponse], id string, want bool) {
	t.Helper()
	for watch.Receive() {
		row := watch.Msg().GetSaved()
		if row.GetId() == id && row.GetEventHistoryNeedsRecovery() == want {
			return
		}
	}
	t.Fatalf("The live machine watch did not publish the fixture warning as %t.", want)
}

func testPairedRecoveryWarning(t *testing.T, stage, removeInstead bool) {
	t.Helper()
	atReplay, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	f := newPairedScript(t, servertest.Options{LinkSeams: servertest.LinkSeams{
		BeforeEventCommit: func(ctx context.Context, _ string, number uint64) error {
			if number == 12 {
				close(atReplay)
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
			return nil
		},
	}})
	page := f.server.Clients(t)
	if stage {
		approve(t, page, f.id, "Recovery warning fixture")
	}
	execute(t, f.db, "INSERT INTO link_machines (machine_id,event_mark) VALUES ($1,10)", f.id)
	incumbent := f.attempt(t, newPairedID(), 11)
	requirePairedWelcome(t, incumbent, 10)
	var commandID string
	if stage {
		command := receivePairedFrame(t, incumbent).GetCommand()
		if command.GetSetState() == nil {
			t.Fatal("The incumbent did not receive the approval notice.")
		}
		commandID = command.GetId()
		sendPairedFrame(t, incumbent, pairedResult(commandID, command.GetAttempt(), 12))
		waitFor(t, 5*time.Second, "the incumbent result to stage at watermark 12", func() bool {
			return scalar[bool](t, f.db, "SELECT COALESCE(staged_event_number=12 AND outcome IS NULL,false) FROM link_commands WHERE id=$1", commandID)
		})
	}
	watch := pairedMachineWatch(t, page)
	requirePairedWarningSnapshot(t, watch, f.id, false)
	generation := scalar[uint64](t, f.db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", f.id)
	requirePairedEnd(t, f.attempt(t, newPairedID(), 9), connect.CodeInvalidArgument, "")
	requirePairedWarningChange(t, watch, f.id, true)
	sendPairedFacts(t, incumbent, 11, 11)
	if !scalar[bool](t, f.db, `
SELECT event_mark=11 AND stream_generation=$2 AND recovery_set_generation=$2
FROM link_machines WHERE machine_id=$1`, f.id, generation) {
		t.Fatal("Incumbent traffic cleared the warning or lost its generation.")
	}
	var add <-chan addResult
	var open projectOperation
	var path string
	if removeInstead {
		path = t.TempDir()
		add = beginAdd(t, projectService(f.server), f.id, "remove-flagged-machine", path)
		open = waitProjectOperation(t, f.db, "remove-flagged-machine")
		requireOpenProject(t, open)
	}
	f.server.Restart(t)
	page = f.server.Clients(t)
	watch = pairedMachineWatch(t, page)
	requirePairedWarningSnapshot(t, watch, f.id, true)
	if stage && !scalar[bool](t, f.db, "SELECT staged_event_number=12 AND outcome IS NULL FROM link_commands WHERE id=$1", commandID) {
		t.Fatal("The staged result did not survive the refusal and restart.")
	}
	if removeInstead {
		project := projectService(f.server)
		execute(t, f.db, "DELETE FROM link_commands WHERE id=$1", open.command)
		requireOpenProject(t, readProjectOperation(t, f.db, "remove-flagged-machine"))
		if scalar[bool](t, f.db, "SELECT EXISTS (SELECT FROM link_commands WHERE id=$1)", open.command) {
			t.Fatal("The open Projects operation still has a Link command before removal.")
		}
		remove(t, page, f.id)
		requireAddRefusal(t, addFolder(t.Context(), project, f.id, "remove-flagged-machine", path),
			connect.CodeNotFound, "The machine was removed.")
		finishAdd(t, add)
		removed := readProjectOperation(t, f.db, "remove-flagged-machine")
		if removed.id != open.id || removed.command != open.command || removed.state != "failed" ||
			removed.failure != "machine_removed" || removed.reply != nil {
			t.Fatal("Removing the flagged machine did not fail its original Projects operation.")
		}
		for watch.Receive() {
			if watch.Msg().GetRemovedId() == f.id {
				if !scalar[bool](t, f.db, "SELECT outcome='removed' AND staged_event_number IS NULL FROM link_commands WHERE id=$1", commandID) {
					t.Fatal("Removal did not settle the staged command as removed.")
				}
				return
			}
		}
		t.Fatal("The watch did not remove the flagged machine.")
	}

	// A real recovered process has the original prefix and one event still to replay.
	payload := retainedPairedEvent(t, pairedFacts(0, "recovered history"))
	events := make([][]byte, 12)
	for i := range events {
		events[i] = payload
	}
	seedPairedEvents(t, f.opts, events...)
	run := startClient(t, f.opts)
	receive(t, atReplay, "the recovered client to replay event 12")
	if !scalar[bool](t, f.db, `
SELECT event_mark=11 AND stream_generation>$2 AND recovery_set_generation=$2
       AND admitted_run_id IS DISTINCT FROM ready_run_id
FROM link_machines WHERE machine_id=$1`, f.id, generation) {
		t.Fatal("Admission cleared the warning before the recovered run was ready.")
	}
	unblock()
	requirePairedWarningChange(t, watch, f.id, false)
	if stage && !scalar[bool](t, f.db, "SELECT outcome='done' AND staged_event_number IS NULL FROM link_commands WHERE id=$1", commandID) {
		t.Fatal("The recovered event did not settle the preserved staged result.")
	}
	run.stop(t)
}

func testPairedHistoryStop(t *testing.T, scenario string) {
	t.Helper()
	server := servertest.Start(t, servertest.Options{Password: fixturePassword})
	r := startRelayWithControls(t, server.Link(), relayControls{beforeReply: func(_ context.Context, frame *linkv1.OpenResponse) bool {
		if welcome := frame.GetWelcome(); welcome != nil && scenario == "IncompatibleWelcomeMark" {
			welcome.LastEventNumber = 2
		}
		return true
	}})
	logs := new(diagnostics)
	opts := newClient(t, logs)
	id := join(t, server.Clients(t), opts, r.address)
	db := openObserver(t, server)
	filename := filepath.Join(opts.StateDir, "client.db")
	wantCode := "store_failed"
	var before []byte
	var err error
	if scenario == "MissingHistory" {
		if err := os.Remove(filename); err != nil {
			t.Fatal(err)
		}
	} else {
		payload := retainedPairedEvent(t, pairedFacts(0, "paired retained history"))
		if scenario == "CorruptRequiredPayload" {
			payload = []byte{1, 0xff}
		} else {
			wantCode = "impossible_ack"
		}
		seedPairedEvents(t, opts, payload)
		before, err = os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
	}
	startClient(t, opts).expectCleanExit(t)
	requireSingleHistoryStop(t, logs, wantCode)
	if scenario == "MissingHistory" {
		if _, err := os.Stat(filename); !os.IsNotExist(err) || len(r.legs) != 0 ||
			scalar[bool](t, db, "SELECT EXISTS(SELECT FROM link_machines WHERE machine_id=$1)", id) {
			t.Fatal("The client recreated missing history or opened a stream.")
		}
		return
	}
	leg := receive(t, r.legs, "the admitted stream before the local stop")
	receive(t, leg.requestsDone, "the stopped client's last request")
	trace := leg.observe()
	after, err := os.ReadFile(filename)
	if !trace.welcomed || len(trace.events) != 0 || len(trace.results) != 0 || len(r.legs) != 0 ||
		err != nil || !bytes.Equal(after, before) || mark(t, db, id) != 0 {
		t.Fatal("The local history stop changed retained state, replayed traffic or retried the connection.")
	}
}

func requireSingleHistoryStop(t *testing.T, logs *diagnostics, code string) {
	t.Helper()
	logs.mu.Lock()
	defer logs.mu.Unlock()
	var count int
	for _, entry := range logs.entries {
		if entry.Message == "link_stopped" && entry.Code == code {
			if !strings.Contains(entry.Remedy, "Restore the original client.db history") {
				t.Fatal("The client did not tell the owner how to recover the stopped history.")
			}
			count++
		}
	}
	if count != 1 {
		t.Fatalf("The client emitted %d history remedies, want one.", count)
	}
}

// Only the database connection that writes a recovery diagnostic receives the
// fault. The two positions distinguish a missing commit from a lost reply.
type pairedCommitConn struct {
	net.Conn
	diagnostic bool
	fault      atomic.Uint32
	cut        chan struct{}
}

func (c *pairedCommitConn) Write(p []byte) (int, error) {
	if c.fault.CompareAndSwap(1, 0) {
		close(c.cut)
		_ = c.Conn.Close()
		return 0, io.ErrClosedPipe
	}
	return c.Conn.Write(p)
}

func (c *pairedCommitConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 && c.fault.CompareAndSwap(2, 0) {
		close(c.cut)
		_ = c.Conn.Close()
		return 0, io.ErrUnexpectedEOF
	}
	return n, err
}

type pairedCommitTracer uint32

func (mode pairedCommitTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if pipe, ok := conn.PgConn().Conn().(*pairedCommitConn); ok {
		if strings.Contains(query.SQL, "UPDATE link_machines SET recovery_set_generation = stream_generation") {
			pipe.diagnostic = true
		}
		if pipe.diagnostic && strings.EqualFold(strings.TrimSpace(query.SQL), "commit") {
			pipe.diagnostic = false
			pipe.fault.Store(uint32(mode))
		}
	}
	return ctx
}

func (pairedCommitTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func testPairedDiagnosticCommit(t *testing.T, committed bool) {
	t.Helper()
	mode := pairedCommitTracer(1)
	if committed {
		mode = 2
	}
	cut := make(chan struct{})
	f := newPairedScript(t, servertest.Options{ConfigurePostgres: func(cfg *pgxpool.Config) {
		dial := cfg.ConnConfig.DialFunc
		cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := dial(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &pairedCommitConn{Conn: conn, cut: cut}, nil
		}
		cfg.ConnConfig.Tracer = mode
	}})
	execute(t, f.db, "INSERT INTO link_machines (machine_id,event_mark) VALUES ($1,10)", f.id)
	incumbent := f.attempt(t, newPairedID(), 11)
	requirePairedWelcome(t, incumbent, 10)
	heartbeat := &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}}
	sendPairedFrame(t, incumbent, heartbeat)
	if receivePairedFrame(t, incumbent).GetHeartbeatAck() == nil {
		t.Fatal("The incumbent did not acknowledge its heartbeat before the fault.")
	}
	current := scalar[string](t, f.db, "SELECT stream_id FROM link_connections WHERE machine_id=$1", f.id)
	head := scalar[int64](t, f.db, "SELECT max(position) FROM feed")
	requirePairedEnd(t, f.attempt(t, newPairedID(), 9), connect.CodeUnavailable, "")
	receive(t, cut, "the diagnostic commit fault")
	if scalar[bool](t, f.db, "SELECT recovery_set_generation IS NOT NULL FROM link_machines WHERE machine_id=$1", f.id) != committed ||
		scalar[bool](t, f.db, "SELECT EXISTS(SELECT FROM feed WHERE topic='machines' AND key=$1 AND change='saved' AND position>$2)", f.id, head) != committed ||
		scalar[string](t, f.db, "SELECT stream_id FROM link_connections WHERE machine_id=$1", f.id) != current ||
		mark(t, f.db, f.id) != 10 {
		t.Fatal("The fault split the recovery flag and feed or displaced the incumbent.")
	}
	sendPairedFrame(t, incumbent, heartbeat)
	if receivePairedFrame(t, incumbent).GetHeartbeatAck() == nil {
		t.Fatal("The incumbent lost its heartbeat after the diagnostic commit fault.")
	}
	f.server.Restart(t)
	requirePairedWarningSnapshot(t, pairedMachineWatch(t, f.server.Clients(t)), f.id, committed)
}
