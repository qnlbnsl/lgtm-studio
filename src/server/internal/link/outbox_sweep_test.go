package link

import (
	"bytes"
	"context"
	"crypto/ed25519"
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
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func seedSweepTerminal(t *testing.T, f *presenceFixture, machine, stream string, expiry, finished time.Time) string {
	t.Helper()
	row := seedClaimRow(t, f, machine, restartAction(), expiry)
	outboxFixtureExec(t, f.obs, `UPDATE link_commands SET sent_stream_id = $2, dispatched = true,
		outcome = 'done', finished_at = $3, payload = NULL WHERE id = $1`, row.id, stream, finished)
	return row.id
}

func requireTerminalCommand(t *testing.T, f *presenceFixture, id, outcome string, start, end time.Time) {
	t.Helper()
	row := readCommandResult(t, f, id)
	if row.outcome != outcome || row.finished == nil || row.finished.Before(start) || row.finished.After(end) || row.hasPayload {
		t.Errorf("The command did not finish as %s with a database finish time and cleared payload.", outcome)
	}
}

func queueDigest(t *testing.T, f *presenceFixture) string {
	t.Helper()
	return storetest.Scalar[string](t, f.obs,
		"SELECT md5(coalesce(jsonb_agg(to_jsonb(c) ORDER BY seq)::text, '')) FROM link_commands c")
}

func TestSweepCommands(t *testing.T) {
	t.Run("expiry and retention", testCommandExpiryAndRetention)
	t.Run("result wins after candidate lookup", testCommandResultBeforeSweepLock)
}

func testCommandExpiryAndRetention(t *testing.T) {
	t.Helper()
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "maintenance")
	f.approve(t, m.ID)
	a := f.admit(t, m.ID, 1)
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	due := seedClaimRow(t, f, m.ID, scanAction(), now.Add(-25*time.Hour))
	dueRestart := seedClaimRow(t, f, m.ID, restartAction(), now.Add(-25*time.Hour))
	notDue := seedClaimRow(t, f, m.ID, restartAction(), now.Add(-23*time.Hour))
	notice := seedClaimRow(t, f, m.ID, standingAction(), now.Add(-8*24*time.Hour))
	sixDays := now.Add(-6 * 24 * time.Hour)
	preserved := seedSweepTerminal(t, f, m.ID, a.ID, now.Add(-10*24*time.Hour), sixDays)
	pruned := seedSweepTerminal(t, f, m.ID, a.ID, now.Add(-10*24*time.Hour), now.Add(-8*24*time.Hour))
	head := f.head(t)
	start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	if f.state.SweepCommands(t.Context()) != nil {
		t.Fatal("The first command sweep returned an error.")
	}
	end := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	requireTerminalCommand(t, f, due.id, "timed_out", start, end)
	requireTerminalCommand(t, f, dueRestart.id, "timed_out", start, end)
	requirePendingResult(t, f, notDue.id)
	requirePendingResult(t, f, notice.id)
	requireTerminalCommand(t, f, preserved, "done", sixDays, sixDays)
	if storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM link_commands WHERE id = $1)", pruned) {
		t.Error("The sweep retained a terminal row past seven days.")
	}
	after := queueDigest(t, f)
	if f.state.SweepCommands(t.Context()) != nil || queueDigest(t, f) != after || f.head(t) != head {
		t.Error("A repeated sweep changed a disposition, finish time or feed reference.")
	}
}

type sweepCandidateKey struct{}

func testCommandResultBeforeSweepLock(t *testing.T) {
	t.Helper()
	atLock := make(chan struct{}, 1)
	var entered atomic.Bool
	hold, resume := storetest.Signal(t)
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
			if ctx.Value(sweepCandidateKey{}) == true && strings.Contains(strings.ToLower(q.SQL), "nowait") && entered.CompareAndSwap(false, true) {
				atLock <- struct{}{}
				select {
				case <-hold:
				case <-ctx.Done():
				}
			}
			return ctx
		})
	})
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "result before sweep")
	f.approve(t, m.ID)
	stream := f.admit(t, m.ID, 1)
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	row := seedClaimRow(t, f, m.ID, restartAction(), now.Add(-25*time.Hour))
	outboxFixtureExec(t, f.obs, "UPDATE link_commands SET sent_stream_id = $2, dispatched = true WHERE id = $1", row.id, stream.ID)
	ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
	defer cancel()
	sweep := storetest.Start(t, func() error {
		return f.state.SweepCommands(context.WithValue(ctx, sweepCandidateKey{}, true))
	})
	defer resume()
	claimGate(t, sweep, atLock, "The sweep did not reach its machine lock after finding due work.")
	if f.state.AcceptResult(ctx, m.ID, stream.ID, commandResult(row.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE)) != nil {
		t.Fatal("The matching dispatched result did not commit before the sweep acquired its machine lock.")
	}
	finished := requireFinishedResult(t, f, row.id, "done", now)
	resume()
	if sweep.Wait(t) != nil {
		t.Fatal("The sweep errored when its due candidate had already finished.")
	}
	requireTerminalCommand(t, f, row.id, "done", finished, finished)
}

type sweepBatchKey struct{}
type sweepLockTraceKey struct{}

func TestSweepReleasesMachineBetweenBatches(t *testing.T) {
	const pending = 145
	var denied atomic.Bool
	var lastWrite atomic.Uint32
	ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
	defer cancel()
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = streamReviewQueryTrace{
			handshakeQueryTrace: func(ctx context.Context, conn *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
				if ctx.Value(sweepBatchKey{}) != true {
					return ctx
				}
				sql := strings.ToLower(strings.TrimSpace(q.SQL))
				if strings.Contains(sql, "nowait") {
					return context.WithValue(ctx, sweepLockTraceKey{}, "lock")
				}
				if strings.Contains(sql, "update link_commands") || strings.Contains(sql, "delete from link_commands") {
					lastWrite.Store(conn.PgConn().PID())
				}
				if sql == "commit" && lastWrite.CompareAndSwap(conn.PgConn().PID(), 0) {
					return context.WithValue(ctx, sweepLockTraceKey{}, "commit")
				}
				return ctx
			},
			end: func(ctx context.Context, q pgx.TraceQueryEndData) {
				switch ctx.Value(sweepLockTraceKey{}) {
				case "lock":
					var pgErr *pgconn.PgError
					if errors.As(q.Err, &pgErr) && pgErr.Code == "55P03" {
						denied.Store(true)
					}
				case "commit":
					if q.Err == nil && denied.Load() {
						cancel()
					}
				}
			},
		}
	})
	a := f.machine(t, bytes.Repeat([]byte{1}, 32), "locked sweep")
	b := f.machine(t, bytes.Repeat([]byte{2}, 32), "other sweep")
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	locked := seedClaimRow(t, f, a.ID, restartAction(), now.Add(-26*time.Hour))
	outboxFixtureExec(t, f.obs, `INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
		SELECT 'batch-' || n, $1, 'restart', $2, $3::timestamptz - interval '2 minutes', $3
		FROM generate_series(1, $4::int) n ORDER BY n`, b.ID, []byte{}, now.Add(-25*time.Hour), pending)
	outboxFixtureExec(t, f.obs, `CREATE TABLE sweep_fixture_audit (write_id bigint NOT NULL);
		CREATE FUNCTION sweep_fixture_change() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			INSERT INTO sweep_fixture_audit VALUES (txid_current());
			RETURN NULL;
		END $$;
		CREATE TRIGGER sweep_fixture_changes AFTER UPDATE OR DELETE ON link_commands
			FOR EACH ROW EXECUTE FUNCTION sweep_fixture_change()`)

	released, unlock := storetest.Signal(t)
	defer unlock()
	held := make(chan struct{})
	holder := storetest.Start(t, func() error {
		return f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if _, err := f.r.Lock(ctx, tx, a.ID); err != nil {
				return err
			}
			close(held)
			select {
			case <-released:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	})
	claimGate(t, holder, held, "The fixture did not hold the first machine's registry lock.")
	err := f.state.SweepCommands(context.WithValue(ctx, sweepBatchKey{}, true))
	if !denied.Load() {
		t.Error("The command sweep did not observe PostgreSQL's nonwaiting machine-lock refusal.")
	}
	requirePendingResult(t, f, locked.id)
	count := storetest.Scalar[int](t, f.obs,
		"SELECT count(*) FROM link_commands WHERE machine_id = $1 AND outcome = 'timed_out'", b.ID)
	if count < 1 || count >= pending || ctx.Err() != context.Canceled || (err != nil && !errors.Is(err, context.Canceled)) {
		t.Error("The other machine did not leave a committed batch and work remaining before cancellation.")
	}
	unlock()
	if holder.Wait(t) != nil {
		t.Fatal("The held registry writer did not release cleanly.")
	}
	if f.state.SweepCommands(t.Context()) != nil ||
		storetest.Scalar[int](t, f.obs, "SELECT count(*) FROM link_commands WHERE outcome = 'timed_out'") != pending+1 {
		t.Error("A fresh sweep did not finish all work left by lock refusal and cancellation.")
	}
	max := storetest.Scalar[int](t, f.obs,
		"SELECT coalesce(max(n), 0) FROM (SELECT count(*) AS n FROM sweep_fixture_audit GROUP BY write_id) AS batches")
	if max < 1 || max > 64 {
		t.Error("A maintenance write changed more than 64 commands or made no observable changes.")
	}
}

type sweepWorkerKey struct{}

func TestCommandSweepStops(t *testing.T) {
	var gateNextPass atomic.Bool
	atQuery := make(chan struct{}, 1)
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
			sql := strings.ToLower(q.SQL)
			if ctx.Value(sweepWorkerKey{}) == true && strings.Contains(sql, "link_commands") && gateNextPass.CompareAndSwap(true, false) {
				atQuery <- struct{}{}
				<-ctx.Done()
			}
			return ctx
		})
	})
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "worker sweep")
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	first := seedClaimRow(t, f, m.ID, restartAction(), now.Add(-25*time.Hour))
	ticks := make(chan time.Time, 2)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx = context.WithValue(ctx, commandSweepTicksKey{}, (<-chan time.Time)(ticks))
	ctx = context.WithValue(ctx, sweepWorkerKey{}, true)
	var reports atomic.Int32
	worker := storetest.Start(t, func() struct{} {
		f.state.RunCommandSweep(ctx, func(error) { reports.Add(1) })
		return struct{}{}
	})
	defer cancel()
	storetest.Poll(t, "the worker's initial sweep", func() bool {
		if readCommandResult(t, f, first.id).outcome == "timed_out" {
			return true
		}
		select {
		case <-worker.Done():
			t.Fatal("The command worker returned before its initial sweep.")
		default:
		}
		return false
	})
	second := seedClaimRow(t, f, m.ID, restartAction(), now.Add(-25*time.Hour))
	ticks <- now.Add(time.Hour)
	storetest.Poll(t, "a sweep on the private hourly tick", func() bool {
		return readCommandResult(t, f, second.id).outcome == "timed_out"
	})
	gateNextPass.Store(true)
	ticks <- now.Add(2 * time.Hour)
	claimGate(t, worker, atQuery, "The command worker returned before the cancellation gate.")
	cancel()
	worker.Wait(t)
	if reports.Load() != 0 {
		t.Error("The worker reported a failure during its normal passes or cancellation.")
	}
}

type removalFixture struct {
	clients lgtmv1connect.ClientServiceClient
	cookie  string
	token   *linkv1.JoinToken
	machine string
}

func newCommandRemovalRoute(t *testing.T, f *presenceFixture) removalFixture {
	t.Helper()
	public := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	const origin = "http://127.0.0.1:8787"
	tokens := machines.NewTokens(f.s, f.r, machines.Issuer{PublicURL: origin, ServerKey: public})
	issued, err := tokens.Create(t.Context(), machines.TokenOptions{})
	if err != nil {
		t.Fatal("The removal fixture could not issue its token.")
	}
	token, err := machines.DecodeToken(issued.Text)
	if err != nil {
		t.Fatal("The removal fixture could not decode its token.")
	}
	joined, err := tokens.Redeem(t.Context(), machines.Redemption{TokenID: token.GetTokenId(),
		Secret: token.GetSecret(), PublicKey: bytes.Repeat([]byte{3}, 32), HostName: "remove commands"})
	if err != nil {
		t.Fatal("The removal fixture could not redeem its token.")
	}
	password, err := auth.NewPassword("command removal fixture password")
	if err != nil {
		t.Fatal("The removal fixture password could not be initialized.")
	}
	sessions := auth.NewSessions(f.s, password)
	cookie, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatal("The removal fixture session could not be issued.")
	}
	origins, err := auth.NewOriginPolicy(origin, nil)
	if err != nil {
		t.Fatal("The removal fixture origin is invalid.")
	}
	svc := machines.NewService(f.s, sessions, f.r, tokens).WithLink(f.state)
	server := httptest.NewServer(api.NewHandler(api.Options{Sessions: sessions, Origins: origins, Clients: svc}))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = storetest.ObservationTimeout
	return removalFixture{
		lgtmv1connect.NewClientServiceClient(client, server.URL),
		(&http.Cookie{Name: auth.CookieName, Value: cookie}).String(), token, joined.MachineID,
	}
}

func removalRequest[T any](f removalFixture, body *T) *connect.Request[T] {
	req := connect.NewRequest(body)
	req.Header().Set("Cookie", f.cookie)
	req.Header().Set(store.RequestKeyHeader, "fixture-removal")
	return req
}

func TestCommandRemovalIsAtomic(t *testing.T) {
	for _, mode := range []string{"approved removal", "waiting removal", "waiting revocation", "approved revocation"} {
		t.Run(mode, func(t *testing.T) {
			var arm atomic.Bool
			var writer atomic.Uint32
			atCommit := make(chan uint32, 1)
			held, release := storetest.Signal(t)
			defer release()
			f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
				cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
					sql := strings.ToLower(strings.TrimSpace(q.SQL))
					pid := conn.PgConn().PID()
					if arm.Load() && strings.HasPrefix(sql, "insert into receipts") {
						writer.Store(pid)
					}
					if sql == "commit" && writer.Load() == pid && arm.CompareAndSwap(true, false) {
						atCommit <- pid
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
			if strings.HasPrefix(mode, "approved") {
				f.approve(t, route.machine)
			}
			a := f.admit(t, route.machine, 1)
			outboxFixtureExec(t, f.obs, "UPDATE link_machines SET event_mark = 7 WHERE machine_id = $1", route.machine)
			now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
			finished := now.Add(-time.Hour)
			kept := seedSweepTerminal(t, f, route.machine, a.ID, now, finished)
			scan := seedClaimRow(t, f, route.machine, scanAction(), now.Add(time.Hour))
			notice := seedClaimRow(t, f, route.machine, standingAction(), now.Add(time.Hour))
			before, beforeQueue, head := outboxPartBRecords(t, f), queueDigest(t, f), f.head(t)

			call := func() error {
				if strings.HasSuffix(mode, "revocation") {
					_, err := route.clients.RevokeClientToken(t.Context(), removalRequest(route, &webv1.RevokeClientTokenRequest{TokenId: route.token.GetTokenId()}))
					return err
				}
				_, err := route.clients.RemoveClient(t.Context(), removalRequest(route, &webv1.RemoveClientRequest{Id: route.machine}))
				return err
			}
			arm.Store(true)
			failed := storetest.Start(t, call)
			defer release()
			pid := claimGate(t, failed, atCommit, "The removal route returned before reaching its real commit.")
			unchanged := func() bool { return outboxPartBRecords(t, f) == before && queueDigest(t, f) == beforeQueue }
			if !unchanged() {
				t.Error("Uncommitted removal exposed changes to the registry, queue, link, feed or receipts.")
			}
			if !storetest.Scalar[bool](t, f.obs, "SELECT pg_terminate_backend($1, 1000)", int32(pid)) {
				t.Fatal("PostgreSQL did not terminate the removal write.")
			}
			release()
			if connect.CodeOf(failed.Wait(t)) != connect.CodeUnavailable || !unchanged() {
				t.Error("Rejecting removal's commit did not preserve the registry, queue, link, feed and receipts.")
			}
			start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
			if call() != nil {
				t.Fatal("The removal route did not commit on retry.")
			}
			if mode == "approved revocation" {
				if queueDigest(t, f) != beforeQueue || f.state.Heartbeat(t.Context(), route.machine, a.ID, &linkv1.Heartbeat{}) != nil {
					t.Error("Revoking an approved machine's redeemed token changed its commands or fenced its stream.")
				}
				return
			}
			end := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
			requireTerminalCommand(t, f, scan.id, "removed", start, end)
			requireTerminalCommand(t, f, notice.id, "removed", start, end)
			requireTerminalCommand(t, f, kept, "done", finished, finished)
			if !storetest.Scalar[bool](t, f.obs, `SELECT
				NOT EXISTS (SELECT FROM machines WHERE id = $1)
				AND NOT EXISTS (SELECT FROM link_machines WHERE machine_id = $1)
				AND NOT EXISTS (SELECT FROM link_connections WHERE machine_id = $1)
				AND (SELECT count(*) FROM feed WHERE position > $2 AND topic = 'machines' AND key = $1 AND change = 'removed') = 1
				AND NOT EXISTS (SELECT FROM feed WHERE position > $2 AND topic = 'machine_outbox')
				AND (SELECT count(*) FROM receipts WHERE request_key = 'fixture-removal') = 1`, route.machine, head) {
				t.Error("The committed removal did not delete the registry and link rows with one owner receipt and no outbox wake.")
			}
			committed := queueDigest(t, f)
			requireResultError(t, f, route.machine, a.ID, commandResult(kept, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED), ErrUnknownMachine)
			if queueDigest(t, f) != committed {
				t.Error("A result after removal changed the committed command dispositions.")
			}
			outboxFixtureExec(t, f.obs, "UPDATE link_commands SET finished_at = clock_timestamp() - interval '8 days' WHERE machine_id = $1 AND outcome IS NOT NULL", route.machine)
			if f.state.SweepCommands(t.Context()) != nil || outboxCount(t, f) != 0 {
				t.Error("The sweep did not eventually prune terminal commands whose machine was removed.")
			}
		})
	}
}
