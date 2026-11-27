package link

import (
	"bytes"
	"context"
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestClaimScanAfterExhaustedAttempt(t *testing.T) {
	now := time.Date(2026, time.November, 26, 12, 0, 0, 0, time.UTC)
	f, _ := newCommandDecisionClock(t, now)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "exhausted scan")
	f.approve(t, m.ID)
	original := f.admit(t, m.ID, 1)
	exhausted := seedClaimRow(t, f, m.ID, scanAction(), now.Add(time.Minute))
	outboxFixtureExec(t, f.obs, `UPDATE link_commands
		SET attempt = $2, sent_stream_id = $3, dispatched = true WHERE id = $1`,
		exhausted.id, int64(math.MaxUint32), original.ID)
	next := seedClaimRow(t, f, m.ID, scanAction(), now.Add(time.Minute))

	if claimAt(t, f, m.ID, original.ID, new(commandCursor)) != nil {
		t.Fatal("The current stream advanced a second scan while its exhausted attempt could still finish.")
	}
	requireClaimStored(t, f, next.id, "", 1)

	replacement := f.admit(t, m.ID, 2)
	got := claimAt(t, f, m.ID, replacement.ID, new(commandCursor))
	if !storetest.Scalar[bool](t, f.obs, `SELECT
		attempt = $2 AND sent_stream_id = $3 AND dispatched
		AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL
		FROM link_commands WHERE id = $1`, exhausted.id, int64(math.MaxUint32), original.ID) {
		t.Fatal("The exhausted scan did not stay pending on the displaced stream.")
	}
	if got.GetId() != next.id {
		t.Fatal("The replacement stream did not claim the next scan past the exhausted attempt.")
	}
	next.requireFrame(t, got, 1)
	requireClaimStored(t, f, next.id, replacement.ID, 1)
}

func TestCommandDecisionTimeBoundaries(t *testing.T) {
	claimTime := time.Date(2026, time.November, 26, 12, 0, 0, 0, time.UTC)
	sweepTime := claimTime.Add(time.Hour)
	f, setClock := newCommandDecisionClock(t, claimTime)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "command boundaries")
	f.approve(t, m.ID)
	stream := f.admit(t, m.ID, 1)
	atRetryCutoff := seedClaimRow(t, f, m.ID, restartAction(), claimTime.Add(-23*time.Hour))
	insideRetryWindow := seedClaimRow(t, f, m.ID, restartAction(), atRetryCutoff.expires.Add(time.Microsecond))
	finished := sweepTime.Add(-7 * 24 * time.Hour)
	atRetentionCutoff := seedSweepTerminal(t, f, m.ID, stream.ID, finished.Add(-time.Hour), finished)

	got := claimAt(t, f, m.ID, stream.ID, new(commandCursor))
	insideRetryWindow.requireFrame(t, got, 1)
	requireClaimStored(t, f, atRetryCutoff.id, "", 1)

	setClock(sweepTime)
	if f.state.SweepCommands(t.Context()) != nil {
		t.Fatal("The command sweep failed at the timeout and retention boundaries.")
	}
	if row := readCommandResult(t, f, atRetryCutoff.id); row.outcome != "" || row.finished != nil || !row.hasPayload {
		t.Fatal("The command did not stay pending at exactly 24 hours past expiry.")
	}
	requireTerminalCommand(t, f, atRetentionCutoff, "done", finished, finished)

	afterCutoff := sweepTime.Add(time.Microsecond)
	setClock(afterCutoff)
	if f.state.SweepCommands(t.Context()) != nil {
		t.Fatal("The command sweep failed one microsecond past the boundaries.")
	}
	requireTerminalCommand(t, f, atRetryCutoff.id, "timed_out", afterCutoff, afterCutoff)
	if storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM link_commands WHERE id = $1)", atRetentionCutoff) {
		t.Error("The sweep retained a terminal command one microsecond past seven days.")
	}
}

func newCommandDecisionClock(t *testing.T, at time.Time) (*presenceFixture, func(time.Time)) {
	t.Helper()
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		// The application resolves the fixture clock before the built-in function.
		cfg.ConnConfig.RuntimeParams["search_path"] = "public, pg_catalog"
	})
	outboxFixtureExec(t, f.obs, "CREATE TABLE command_fixture_clock (at timestamptz NOT NULL)")
	outboxFixtureExec(t, f.obs, "INSERT INTO command_fixture_clock (at) VALUES ($1)", at)
	outboxFixtureExec(t, f.obs, `CREATE FUNCTION public.clock_timestamp() RETURNS timestamptz LANGUAGE sql STABLE
		AS $$ SELECT at FROM public.command_fixture_clock $$`)
	return f, func(next time.Time) {
		t.Helper()
		outboxFixtureExec(t, f.obs, "UPDATE command_fixture_clock SET at = $1", next)
	}
}

func TestApprovedRemovalFinalizesPendingRestart(t *testing.T) {
	var armed atomic.Bool
	atCommit := make(chan uint32, 1)
	held, release := storetest.Signal(t)
	defer release()
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
			if strings.EqualFold(q.SQL, "commit") && armed.CompareAndSwap(true, false) {
				atCommit <- conn.PgConn().PID()
				select {
				case <-held:
				case <-ctx.Done():
				}
			}
			return ctx
		})
	})
	if f.s.Migrate(t.Context(), machines.Migrations(), Migrations(), auth.Migrations()) != nil {
		t.Fatal("The removal fixture could not migrate owner sessions.")
	}
	route := newCommandRemovalRoute(t, f)
	f.approve(t, route.machine)
	stream := f.admit(t, route.machine, 1)
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	finished := now.Add(-time.Hour)
	completed := seedSweepTerminal(t, f, route.machine, stream.ID, now, finished)
	pending := seedClaimRow(t, f, route.machine, restartAction(), now.Add(time.Hour))

	before, beforeQueue := outboxPartBRecords(t, f), queueDigest(t, f)
	unchanged := func() bool { return outboxPartBRecords(t, f) == before && queueDigest(t, f) == beforeQueue }
	remove := func() error {
		_, err := route.clients.RemoveClient(t.Context(), removalRequest(route, &webv1.RemoveClientRequest{Id: route.machine}))
		return err
	}
	armed.Store(true)
	failed := storetest.Start(t, remove)
	pid := claimGate(t, failed, atCommit, "The approved removal did not reach its real commit.")
	if !unchanged() {
		t.Error("The held removal exposed changes before commit.")
	}
	if !storetest.Scalar[bool](t, f.obs, "SELECT pg_terminate_backend($1, 1000)", int32(pid)) {
		t.Fatal("PostgreSQL did not terminate the approved removal write.")
	}
	release()
	if connect.CodeOf(failed.Wait(t)) != connect.CodeUnavailable || !unchanged() {
		t.Fatal("Rejecting the removal commit did not preserve the pending restart and the existing records.")
	}

	start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	if remove() != nil {
		t.Fatal("Approved removal did not commit on retry.")
	}
	end := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	requireTerminalCommand(t, f, pending.id, "removed", start, end)
	requireTerminalCommand(t, f, completed, "done", finished, finished)
	if storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM machines WHERE id = $1)", route.machine) {
		t.Error("The approved machine still existed after the removal retry committed.")
	}
}
