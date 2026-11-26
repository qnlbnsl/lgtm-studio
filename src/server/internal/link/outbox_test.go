package link

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestCommandMigrationUpgradesPartB(t *testing.T) {
	ctx := t.Context()
	cfg := storetest.NewDatabase(t, "public")
	s, err := store.Open(ctx, cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The upgrade store did not open.")
	}
	t.Cleanup(s.Close)
	obs, err := pgx.ConnectConfig(ctx, cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The upgrade observer did not connect.")
	}
	t.Cleanup(func() { _ = obs.Close(context.Background()) })

	full := Migrations()
	const oldName = "0001_link.sql"
	oldSQL, err := fs.ReadFile(full.Files, oldName)
	if err != nil {
		t.Fatal("The Part B migration was not available.")
	}
	old := store.Migrations{Part: "link", Files: fstest.MapFS{oldName: {Data: oldSQL}}}
	if s.Migrate(ctx, machines.Migrations(), old) != nil {
		t.Fatal("The Part B manifest did not apply.")
	}
	if !storetest.Scalar[bool](t, obs, `SELECT version = 1 AND to_regclass('link_commands') IS NULL
		FROM schema_migrations WHERE part = 'link'`) {
		t.Fatal("The old manifest did not start at link revision 1 without a queue.")
	}

	r, hub := machines.NewRegistry(), NewHub()
	f := &presenceFixture{s: s, r: r, obs: obs, hub: hub, state: NewState(s, r, hub)}
	f.machine(t, bytes.Repeat([]byte{1}, 32), "waiting for upgrade")
	connected := f.machine(t, bytes.Repeat([]byte{2}, 32), "connected before upgrade")
	f.approve(t, connected.ID)
	stream := f.admit(t, connected.ID, 1)
	outboxFixtureExec(t, obs, "UPDATE link_machines SET event_mark = 42 WHERE machine_id = $1", connected.ID)

	request := store.Request{
		Key: "upgrade-request", Caller: "upgrade-owner", Procedure: "upgrade-fixture",
		FingerprintVersion: 1, Fingerprints: map[int16][]byte{1: []byte("upgrade fingerprint")},
		ReceiptSafe: true,
	}
	reply := []byte("saved upgrade reply")
	if saved, err := s.WriteOnce(ctx, request, func(context.Context, *store.Tx) ([]byte, error) {
		return reply, nil
	}); err != nil || !bytes.Equal(saved, reply) {
		t.Fatal("The keyed reply was not saved before the upgrade.")
	}
	if !storetest.Scalar[bool](t, obs, `SELECT
		(SELECT count(*) FROM machines WHERE approval = 'waiting') = 1
		AND EXISTS (SELECT FROM link_connections WHERE machine_id = $1 AND stream_id = $2)
		AND EXISTS (SELECT FROM link_machines WHERE machine_id = $1 AND event_mark = 42)
		AND (SELECT count(*) FROM receipts) = 1`, connected.ID, stream.ID) {
		t.Fatal("The Part B records to preserve were incomplete.")
	}
	before := outboxPartBRecords(t, f)
	for range 2 {
		if s.Migrate(ctx, machines.Migrations(), full) != nil {
			t.Fatal("The full link manifest did not apply to Part B.")
		}
		if outboxPartBRecords(t, f) != before {
			t.Error("The full manifest changed a Part B record.")
		}
	}
	if !storetest.Scalar[bool](t, obs, `SELECT version = 2 AND to_regclass('link_commands') IS NOT NULL
		FROM schema_migrations WHERE part = 'link'`) {
		t.Fatal("The full manifest did not record link revision 2 and create the queue.")
	}
	if storetest.Scalar[int64](t, obs, "SELECT count(*) FROM link_commands") != 0 {
		t.Error("The upgrade created a command for a Part B record.")
	}
	if s.Migrate(ctx, machines.Migrations(), old) == nil {
		t.Error("The revision 1 manifest was accepted after the queue upgrade.")
	}
	restarted, err := store.Open(ctx, cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The upgraded store could not reopen.")
	}
	t.Cleanup(restarted.Close)
	if restarted.Migrate(ctx, machines.Migrations(), full) != nil {
		t.Fatal("The full manifest could not start again after refusing the old manifest.")
	}
	replayed := false
	saved, err := restarted.WriteOnce(ctx, request, func(context.Context, *store.Tx) ([]byte, error) {
		replayed = true
		return nil, nil
	})
	if err != nil || replayed || !bytes.Equal(saved, reply) {
		t.Error("The upgraded store did not replay the original keyed reply.")
	}

	now := storetest.Scalar[time.Time](t, obs, "SELECT clock_timestamp()")
	for _, tc := range []struct {
		name     string
		attempt  int64
		payload  []byte
		outcome  *string
		finished *time.Time
	}{
		{name: "zero attempt", attempt: 0, payload: []byte{}},
		{name: "overflow attempt", attempt: 4294967296, payload: []byte{}},
		{name: "outcome without finish", attempt: 1, outcome: proto.String("done")},
		{name: "finish without outcome", attempt: 1, payload: []byte{}, finished: &now},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := obs.Exec(t.Context(), `INSERT INTO link_commands
				(id, machine_id, action, payload, created_at, expires_at, attempt, sent_stream_id, dispatched, finished_at, outcome)
				VALUES ($1, $2, 'scan_harnesses', $3, $4, $4::timestamptz + interval '2 minutes', $5, $6, true, $7, $8)`,
				fixtureStreamID(t), connected.ID, tc.payload, now, tc.attempt, stream.ID, tc.finished, tc.outcome)
			var constraint *pgconn.PgError
			if err == nil {
				t.Fatal("PostgreSQL did not reject the invalid command with a check constraint.")
			}
			if !errors.As(err, &constraint) || constraint.Code != "23514" {
				t.Fatal("The invalid command failed for a reason other than a check constraint.")
			}
		})
	}
}

func outboxPartBRecords(t *testing.T, f *presenceFixture) string {
	t.Helper()
	return storetest.Scalar[string](t, f.obs, `SELECT md5(jsonb_build_array(
		(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.id) FROM machines m),
		(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.machine_id) FROM link_machines m),
		(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.machine_id) FROM link_connections c),
		(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM client_join_tokens t),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.principal_hash, r.request_key) FROM receipts r),
		(SELECT jsonb_agg(to_jsonb(f) ORDER BY f.position) FROM feed f)
	)::text)`)
}

func outboxFixtureExec(t *testing.T, conn storetest.Executor, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Fatalf("A command fixture statement did not execute (SQLSTATE %s).", pgErr.Code)
		}
		t.Fatal("A command fixture statement did not execute.")
	}
}

type enqueueCommitKey struct{}

func TestEnqueueStoresBeforeSend(t *testing.T) {
	t.Run("committed scan", func(t *testing.T) {
		f := newPresenceFixture(t, nil)
		m := f.machine(t, bytes.Repeat([]byte{1}, 32), "scan")
		f.approve(t, m.ID)
		head := f.head(t)
		start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
		strayID := fixtureStreamID(t)
		scan := &linkv1.ScanHarnesses{}
		extra := protowire.AppendString(protowire.AppendTag(nil, 40, protowire.BytesType), "extra command field")
		scan.ProtoReflect().SetUnknown(extra)
		command := &linkv1.Command{
			Id: strayID, Attempt: 9, ExpiresAt: timestamppb.New(start.Add(9 * time.Hour)),
			Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: scan},
		}
		command.ProtoReflect().SetUnknown(extra)
		err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
				return err
			}
			return f.state.Enqueue(ctx, tx, m.ID, command, 2*time.Minute)
		})
		if err != nil {
			t.Fatal("The valid scan did not enqueue.")
		}
		end := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
		if outboxCount(t, f) != 1 {
			t.Error("The committed enqueue did not store exactly one command.")
		} else {
			var id, action string
			var payload []byte
			var created, expires time.Time
			var initial bool
			err := f.obs.QueryRow(t.Context(), `SELECT id, action, payload, created_at, expires_at,
				machine_id = $1 AND attempt = 1 AND sent_stream_id IS NULL AND NOT dispatched
				AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL
				FROM link_commands`, m.ID).Scan(&id, &action, &payload, &created, &expires, &initial)
			if err != nil || !streamVersion7.MatchString(id) || id == strayID || !initial ||
				created.Before(start) || created.After(end) || expires.Sub(created) != 2*time.Minute {
				t.Error("The command did not have a fresh version 7 identifier, a two-minute lifetime and an unassigned first attempt.")
			}
			// An empty action can be stored on its own or in an action-only command envelope.
			var decoded linkv1.Command
			clean := &linkv1.Command{Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}}}
			if action != "scan_harnesses" || (len(payload) != 0 &&
				(proto.Unmarshal(payload, &decoded) != nil || !proto.Equal(&decoded, clean))) {
				t.Error("The scan payload retained envelope or unknown fields.")
			}
		}
		count := storetest.Scalar[int64](t, f.obs, `SELECT count(*) FROM feed
			WHERE position > $1 AND topic = 'machine_outbox' AND scope = '' AND key = $2 AND change = 'saved'`, head, m.ID)
		if count != 1 || storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM feed WHERE position > $1", head) != 1 {
			t.Error("The enqueue did not commit exactly one saved outbox wake.")
		}
	})

	for _, tc := range []struct {
		name    string
		command *linkv1.Command
		ttl     time.Duration
	}{
		{"missing action", &linkv1.Command{Attempt: 1}, time.Minute},
		{"zero lifetime", &linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}}, 0},
		{"negative lifetime", &linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}}, -time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPresenceFixture(t, nil)
			m := f.machine(t, bytes.Repeat([]byte{1}, 32), "invalid enqueue")
			f.approve(t, m.ID)
			head := f.head(t)
			err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
				if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
					return err
				}
				return f.state.Enqueue(ctx, tx, m.ID, tc.command, tc.ttl)
			})
			if err == nil || outboxCount(t, f) != 0 || f.head(t) != head {
				t.Error("The invalid enqueue did not fail without a command or wake.")
			}
		})
	}

	t.Run("failed commit", func(t *testing.T) {
		atCommit := make(chan uint32, 1)
		release, resume := storetest.Signal(t)
		f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
			cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
				if pid, _ := ctx.Value(enqueueCommitKey{}).(*uint32); pid != nil &&
					conn.PgConn().PID() == *pid && strings.EqualFold(query.SQL, "commit") {
					atCommit <- *pid
					select {
					case <-release:
					case <-ctx.Done():
					}
				}
				return ctx
			})
		})
		m := f.machine(t, bytes.Repeat([]byte{1}, 32), "rollback")
		f.approve(t, m.ID)
		before := outboxPartBRecords(t, f)
		unchanged := func() {
			t.Helper()
			if outboxCount(t, f) != 0 || outboxPartBRecords(t, f) != before {
				t.Error("The command, feed or caller changes became visible without the caller committing.")
			}
		}
		ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
		defer cancel()
		var callerPID uint32
		task := storetest.Start(t, func() error {
			return f.s.Write(context.WithValue(ctx, enqueueCommitKey{}, &callerPID), func(ctx context.Context, tx *store.Tx) error {
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&callerPID); err != nil {
					return err
				}
				if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
					return err
				}
				changed, err := f.r.SetName(ctx, tx, m.ID, "Uncommitted name")
				if err != nil {
					return err
				}
				if !changed {
					return store.NewDomainError("The caller did not change the fixture name.")
				}
				command := &linkv1.Command{Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}}}
				return f.state.Enqueue(ctx, tx, m.ID, command, 2*time.Minute)
			})
		})
		defer func() { resume(); task.Wait(t) }()
		var pid uint32
		select {
		case pid = <-atCommit:
		case <-task.Done():
			t.Fatal("The enqueue write returned before reaching its commit boundary.")
		case <-ctx.Done():
			t.Fatal("The enqueue write did not reach its commit boundary.")
		}
		unchanged()
		if !storetest.Scalar[bool](t, f.obs, "SELECT pg_terminate_backend($1, 5000)", int32(pid)) {
			t.Fatal("PostgreSQL did not terminate the held enqueue transaction.")
		}
		resume()
		if task.Wait(t) == nil {
			t.Error("The enqueue write succeeded after its commit was rejected.")
		}
		unchanged()
	})
}

func outboxCount(t *testing.T, f *presenceFixture) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_commands")
}
