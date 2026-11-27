package link

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func scanAction() *linkv1.Command {
	return &linkv1.Command{Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}}}
}

func restartAction() *linkv1.Command {
	return &linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}}
}

func standingAction() *linkv1.Command {
	return &linkv1.Command{Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{State: &linkv1.MachineState{
		Approval: linkv1.Approval_APPROVAL_APPROVED, Name: "Approved machine", Revision: 2,
	}}}}
}

type claimFixtureRow struct {
	id      string
	expires time.Time
	action  *linkv1.Command
}

func seedClaimRow(t *testing.T, f *presenceFixture, machine string, action *linkv1.Command, expiry time.Time) claimFixtureRow {
	t.Helper()
	kind, clean, err := commandAction(action)
	if err != nil {
		t.Fatal("The fixture action did not validate.")
	}
	payload, err := proto.Marshal(clean)
	if err != nil {
		t.Fatal("The fixture action did not encode.")
	}
	row := claimFixtureRow{fixtureStreamID(t), expiry.Truncate(time.Microsecond), clean}
	outboxFixtureExec(t, f.obs, `INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5::timestamptz - interval '2 minutes', $5)`, row.id, machine, kind, payload, row.expires)
	return row
}

func (row claimFixtureRow) requireFrame(t *testing.T, got *linkv1.Command, attempt uint32) {
	t.Helper()
	if got.GetId() != row.id || got.GetAttempt() != attempt || got.GetExpiresAt().CheckValid() != nil ||
		!got.GetExpiresAt().AsTime().Equal(row.expires) {
		t.Fatal("The claim did not return the stored identifier, expiry and expected attempt.")
	}
	clean := proto.CloneOf(got)
	clean.Id, clean.Attempt, clean.ExpiresAt = "", 0, nil
	if !proto.Equal(clean, row.action) {
		t.Error("The claim did not reconstruct exactly the stored action.")
	}
}

func requireClaimStored(t *testing.T, f *presenceFixture, id, stream string, attempt int64) {
	t.Helper()
	if !storetest.Scalar[bool](t, f.obs, `SELECT
		coalesce(sent_stream_id, '') = $2 AND attempt = $3 AND NOT dispatched
		AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL
		FROM link_commands WHERE id = $1`, id, stream, attempt) {
		t.Error("The committed command row did not have the expected claim and reset dispatch marker.")
	}
}

func claimAt(t *testing.T, f *presenceFixture, machine, stream string, cursor *commandCursor) *linkv1.Command {
	t.Helper()
	cmd, _, err := f.state.claimNext(t.Context(), machine, stream, cursor)
	if err != nil {
		t.Fatal("The current stream's claim failed.")
	}
	return cmd
}

func TestClaimRaisesAttemptOncePerStream(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "claim attempts")
	f.approve(t, m.ID)
	a := f.admit(t, m.ID, 1)
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	row := seedClaimRow(t, f, m.ID, restartAction(), now.Add(time.Minute))
	head := f.head(t)
	cursor := new(commandCursor)
	row.requireFrame(t, claimAt(t, f, m.ID, a.ID, cursor), 1)
	requireClaimStored(t, f, row.id, a.ID, 1)
	outboxFixtureExec(t, f.obs, "UPDATE link_commands SET dispatched = true WHERE id = $1", row.id)
	if claimAt(t, f, m.ID, a.ID, cursor) != nil || claimAt(t, f, m.ID, a.ID, new(commandCursor)) != nil {
		t.Error("The same stream received a second claim even when its cursor was reset.")
	}
	b := f.admit(t, m.ID, 2)
	headAfterAdmission := f.head(t)
	row.requireFrame(t, claimAt(t, f, m.ID, b.ID, new(commandCursor)), 2)
	requireClaimStored(t, f, row.id, b.ID, 2)
	outboxFixtureExec(t, f.obs, "UPDATE link_commands SET attempt = $2 WHERE id = $1", row.id, int64(math.MaxUint32))
	next := seedClaimRow(t, f, m.ID, restartAction(), now.Add(time.Minute))
	next.requireFrame(t, claimAt(t, f, m.ID, b.ID, new(commandCursor)), 1)
	requireClaimStored(t, f, row.id, b.ID, math.MaxUint32)
	c := f.admit(t, m.ID, 3)
	afterReplacement := f.head(t)
	next.requireFrame(t, claimAt(t, f, m.ID, c.ID, new(commandCursor)), 2)
	requireClaimStored(t, f, row.id, b.ID, math.MaxUint32)
	if headAfterAdmission != head+1 || afterReplacement != headAfterAdmission+1 || f.head(t) != afterReplacement {
		t.Error("Claiming an attempt wrote a feed reference.")
	}
}

type claimCommitKey struct{}

func TestClaimCommitsBeforeReturn(t *testing.T) {
	for _, mode := range []string{"release", "rollback", "lost reply"} {
		t.Run(mode, func(t *testing.T) {
			atCommit := make(chan uint32, 1)
			arrived := make(chan struct{}, 1)
			hold, resume := storetest.Signal(t)
			f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
				if mode == "lost reply" {
					dial := cfg.ConnConfig.DialFunc
					cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
						conn, err := dial(ctx, network, address)
						if err != nil {
							return nil, err
						}
						return &admissionReplyConn{Conn: conn, arrived: arrived, release: hold}, nil
					}
				}
				cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
					if ctx.Value(claimCommitKey{}) != true || !strings.EqualFold(strings.TrimSpace(d.SQL), "commit") {
						return ctx
					}
					if mode == "lost reply" {
						if pipe, ok := conn.PgConn().Conn().(*admissionReplyConn); ok {
							pipe.armed.Store(true)
						}
					}
					atCommit <- conn.PgConn().PID()
					if mode != "lost reply" {
						select {
						case <-hold:
						case <-ctx.Done():
						}
					}
					return ctx
				})
			})
			m := f.machine(t, bytes.Repeat([]byte{1}, 32), "commit claim")
			f.approve(t, m.ID)
			a := f.admit(t, m.ID, 1)
			now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
			row := seedClaimRow(t, f, m.ID, restartAction(), now.Add(time.Minute))
			type result struct {
				cmd  *linkv1.Command
				err  error
				more bool
			}
			task := storetest.Start(t, func() result {
				cmd, more, err := f.state.claimNext(context.WithValue(t.Context(), claimCommitKey{}, true), m.ID, a.ID, new(commandCursor))
				return result{cmd, err, more}
			})
			defer resume()
			pid := claimGate(t, task, atCommit, "The claim returned before reaching its real commit.")
			if mode == "lost reply" {
				claimGate(t, task, arrived, "The claim returned before the durable commit reply reached the wire gate.")
				requireClaimStored(t, f, row.id, a.ID, 1)
			} else {
				requireClaimStored(t, f, row.id, "", 1)
			}
			select {
			case <-task.Done():
				t.Fatal("The claim returned while its commit or commit reply was held.")
			default:
			}
			if mode == "rollback" && !storetest.Scalar[bool](t, f.obs, "SELECT pg_terminate_backend($1, 1000)", int32(pid)) {
				t.Fatal("PostgreSQL did not terminate the transaction before commit.")
			}
			resume()
			got := task.Wait(t)
			switch mode {
			case "release":
				if got.err != nil {
					t.Fatal("The claim errored after the commit was released.")
				}
				row.requireFrame(t, got.cmd, 1)
				requireClaimStored(t, f, row.id, a.ID, 1)
			case "rollback", "lost reply":
				if got.err == nil || got.cmd != nil || got.more {
					t.Error("An unsuccessful claim returned work or continuation instead of a stream-ending error.")
				}
				want := uint32(2)
				if mode == "rollback" {
					requireClaimStored(t, f, row.id, "", 1)
					want = 1
				}
				b := f.admit(t, m.ID, 2)
				row.requireFrame(t, claimAt(t, f, m.ID, b.ID, new(commandCursor)), want)
				requireClaimStored(t, f, row.id, b.ID, int64(want))
			}
		})
	}
}

func claimGate[V, R any](t *testing.T, task *storetest.Task[R], event <-chan V, failure string) V {
	t.Helper()
	select {
	case value := <-event:
		return value
	case <-task.Done():
		select {
		case value := <-event:
			return value
		default:
			t.Fatal(failure)
		}
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("The expected database gate was never reached.")
	}
	var zero V
	return zero
}

func TestClaimOrder(t *testing.T) {
	t.Run("sequence cutoff and approval transition", func(t *testing.T) {
		f := newPresenceFixture(t, nil)
		m := f.machine(t, bytes.Repeat([]byte{1}, 32), "waiting claim")
		a := f.admit(t, m.ID, 1)
		now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
		tooOld := seedClaimRow(t, f, m.ID, restartAction(), now.Add(-23*time.Hour-time.Minute))
		first := seedClaimRow(t, f, m.ID, restartAction(), now.Add(-23*time.Hour+time.Minute))
		second := seedClaimRow(t, f, m.ID, restartAction(), now.Add(time.Hour))
		outboxFixtureExec(t, f.obs, "UPDATE link_commands SET created_at = $2 WHERE id = $1", second.id, first.expires.Add(-time.Hour))
		notice := seedClaimRow(t, f, m.ID, standingAction(), now.Add(-10*24*time.Hour))
		cursor := new(commandCursor)
		head := f.head(t)
		notice.requireFrame(t, claimAt(t, f, m.ID, a.ID, cursor), 1)
		if claimAt(t, f, m.ID, a.ID, cursor) != nil || f.head(t) != head {
			t.Error("A waiting stream claimed a nonstate action or wrote a feed reference.")
		}
		f.approve(t, m.ID)
		head = f.head(t)
		first.requireFrame(t, claimAt(t, f, m.ID, a.ID, cursor), 1)
		second.requireFrame(t, claimAt(t, f, m.ID, a.ID, cursor), 1)
		if claimAt(t, f, m.ID, a.ID, cursor) != nil || f.head(t) != head {
			t.Error("The approved stream claimed expired or duplicate work, or wrote a feed reference.")
		}
		requireClaimStored(t, f, tooOld.id, "", 1)
		f.admit(t, m.ID, 2)
		cmd, more, err := f.state.claimNext(t.Context(), m.ID, a.ID, cursor)
		if !errors.Is(err, ErrNotCurrent) || cmd != nil || more {
			t.Error("The displaced stream did not receive the current-stream fence.")
		}
	})

	for _, settle := range []string{"result", "cutoff and heartbeat"} {
		t.Run(settle, func(t *testing.T) {
			f := newPresenceFixture(t, nil)
			m := f.machine(t, bytes.Repeat([]byte{1}, 32), "scan deferral")
			f.approve(t, m.ID)
			a := f.admit(t, m.ID, 1)
			now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
			first := seedClaimRow(t, f, m.ID, scanAction(), now.Add(time.Minute))
			cursor := new(commandCursor)
			first.requireFrame(t, claimAt(t, f, m.ID, a.ID, cursor), 1)
			second := seedClaimRow(t, f, m.ID, scanAction(), now.Add(time.Minute))
			restart := seedClaimRow(t, f, m.ID, restartAction(), now.Add(time.Minute))
			restart.requireFrame(t, claimAt(t, f, m.ID, a.ID, cursor), 1)
			if claimAt(t, f, m.ID, a.ID, cursor) != nil {
				t.Error("The stream claimed a distinct scan while its head remained pending.")
			}
			b := f.admit(t, m.ID, 2)
			cursor = new(commandCursor)
			first.requireFrame(t, claimAt(t, f, m.ID, b.ID, cursor), 2)
			restart.requireFrame(t, claimAt(t, f, m.ID, b.ID, cursor), 2)
			if claimAt(t, f, m.ID, b.ID, cursor) != nil {
				t.Error("The replacement stream lost the older scan's reservation.")
			}
			requireClaimStored(t, f, second.id, "", 1)
			if settle == "result" {
				outboxFixtureExec(t, f.obs, "UPDATE link_commands SET dispatched = true WHERE id = $1", first.id)
				if f.state.AcceptResult(t.Context(), m.ID, b.ID, commandResult(first.id, 2, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE)) != nil {
					t.Fatal("The remaining scan's dispatched result did not commit.")
				}
			} else {
				outboxFixtureExec(t, f.obs, "UPDATE link_commands SET expires_at = clock_timestamp() - interval '23 hours 1 minute' WHERE id = $1", first.id)
				if f.state.Heartbeat(t.Context(), m.ID, b.ID, &linkv1.Heartbeat{}) != nil {
					t.Fatal("The heartbeat after the scan cutoff did not commit.")
				}
			}
			head := f.head(t)
			second.requireFrame(t, claimAt(t, f, m.ID, b.ID, cursor), 1)
			if f.head(t) != head {
				t.Error("Rewinding the cursor to the deferred scan wrote a feed reference.")
			}
		})
	}
}

type claimPageTraceKey struct{}

type observedClaimPage struct {
	sql  string
	args []any
}

func TestClaimBoundsIneligiblePrefix(t *testing.T) {
	const pages = 3
	var queries []observedClaimPage
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
			sql := strings.ToLower(query.SQL)
			if ctx.Value(claimPageTraceKey{}) == true && strings.Contains(sql, "link_commands") &&
				strings.Contains(sql, "order by") && strings.Contains(sql, "seq") {
				queries = append(queries, observedClaimPage{query.SQL, append([]any(nil), query.Args...)})
			}
			return ctx
		})
	})
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "claim prefix")
	f.approve(t, m.ID)
	a := f.admit(t, m.ID, 1)
	outboxFixtureExec(t, f.obs, `INSERT INTO link_commands
		(id, machine_id, action, payload, created_at, expires_at, sent_stream_id)
		SELECT 'prefix-' || n, $1, 'restart', $4, now() - interval '2 days',
		       CASE WHEN n % 2 = 0 THEN now() - interval '25 hours' ELSE now() END,
		       CASE WHEN n % 2 = 0 THEN NULL ELSE $2 END
		FROM generate_series(1, $3::int) n ORDER BY n`, m.ID, a.ID, pages*32, []byte{})
	now := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	tail := seedClaimRow(t, f, m.ID, restartAction(), now.Add(time.Minute))
	outboxFixtureExec(t, f.obs, `INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
		SELECT 'background-' || n, 'other-' || n / 50, 'restart', $1, now(), now()
		FROM generate_series(1, 10000) n`, []byte{})
	outboxFixtureExec(t, f.obs, "ANALYZE link_commands")

	cursor := new(commandCursor)
	emptyTurn := func(ctx context.Context) error {
		cmd, more, err := f.state.claimNext(context.WithValue(ctx, claimPageTraceKey{}, true), m.ID, a.ID, cursor)
		if err != nil || cmd != nil || !more {
			return errors.New("The ineligible page did not yield an empty continuation.")
		}
		return nil
	}
	// Observe the heartbeat actually waiting on the first turn's registry lock.
	if err := checkOneLine(t, f, emptyTurn, func(ctx context.Context) error {
		return f.state.Heartbeat(ctx, m.ID, a.ID, &linkv1.Heartbeat{})
	}, false); err != nil {
		t.Fatal("The competing heartbeat did not commit between claim turns.")
	}
	for turn := 1; turn < pages; turn++ {
		if emptyTurn(t.Context()) != nil {
			t.Fatal("The ineligible page did not yield an empty continuation.")
		}
	}
	if len(queries) != pages {
		t.Fatal("Each ineligible turn must issue one ordered pending-page query.")
	}
	for _, query := range queries {
		requireBoundedClaimPlan(t, f, query)
	}
	tail.requireFrame(t, claimAt(t, f, m.ID, a.ID, cursor), 1)
}

type claimPlanNode struct {
	Kind  string          `json:"Node Type"`
	Index string          `json:"Index Name"`
	Rows  int             `json:"Actual Rows"`
	Loops int             `json:"Actual Loops"`
	Plans []claimPlanNode `json:"Plans"`
}

func requireBoundedClaimPlan(t *testing.T, f *presenceFixture, query observedClaimPage) {
	t.Helper()
	var plan []byte
	if err := f.obs.QueryRow(t.Context(), "EXPLAIN (ANALYZE, FORMAT JSON) "+query.sql, query.args...).Scan(&plan); err != nil {
		t.Fatal("The observed pending-page statement could not be explained.")
	}
	var roots []struct{ Plan claimPlanNode }
	if json.Unmarshal(plan, &roots) != nil || len(roots) != 1 {
		t.Fatal("The measured pending-page plan did not decode.")
	}
	var visits int
	var inspect func(claimPlanNode, bool)
	inspect = func(node claimPlanNode, bounded bool) {
		bounded = bounded || node.Kind == "Limit" && node.Loops == 1 && node.Rows == 32
		if node.Index == "link_commands_pending" && bounded && node.Loops == 1 && node.Rows == 32 {
			visits++
		}
		for _, child := range node.Plans {
			inspect(child, bounded)
		}
	}
	inspect(roots[0].Plan, false)
	if visits != 1 {
		t.Error("The measured PostgreSQL plan did not read one 32-row page through the pending index before filtering.")
	}
}
