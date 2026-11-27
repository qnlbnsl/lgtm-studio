package link

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"math"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestLinkRevisionTwoMigration(t *testing.T) {
	ctx := t.Context()
	cfg := storetest.NewDatabase(t, "public")
	s, err := store.Open(ctx, cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The revision 2 upgrade store did not open.")
	}
	t.Cleanup(s.Close)
	obs, err := pgx.ConnectConfig(ctx, cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The revision 2 upgrade observer did not connect.")
	}
	t.Cleanup(func() { _ = obs.Close(context.Background()) })

	old := linkMigrationManifestThrough(t, "0001_link.sql", "0002_link_commands.sql")
	if s.Migrate(ctx, machines.Migrations(), old) != nil ||
		storetest.Scalar[int](t, obs, "SELECT version FROM schema_migrations WHERE part = 'link'") != 2 {
		t.Fatal("The fixture did not start at Link migration version 2.")
	}
	f := &presenceFixture{s: s, r: machines.NewRegistry(), obs: obs}
	waiting := f.machine(t, bytes.Repeat([]byte{1}, 32), "waiting through migration")
	approved := f.machine(t, bytes.Repeat([]byte{2}, 32), "approved through migration")
	f.approve(t, approved.ID)
	streamID := seedLinkMigrationConnection(t, f, approved.ID)
	outboxFixtureExec(t, obs, "INSERT INTO link_machines (machine_id) VALUES ($1)", waiting.ID)
	outboxFixtureExec(t, obs, `UPDATE link_connections SET facts_received = true, harnesses_received = true,
		cpu_percent = 9, memory_percent = 20 WHERE machine_id = $1`, approved.ID)

	now := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	queued, dispatched, terminal, standing := fixtureStreamID(t), fixtureStreamID(t), fixtureStreamID(t), fixtureStreamID(t)
	restartPayload, err := proto.Marshal(&linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}})
	if err != nil || len(restartPayload) == 0 {
		t.Fatal("The historical pending command did not encode a payload.")
	}
	outboxFixtureExec(t, obs, `INSERT INTO link_commands
		(id, machine_id, action, payload, created_at, expires_at, attempt, sent_stream_id, dispatched, finished_at, outcome)
		VALUES ($1, $5, 'scan_harnesses', ''::bytea, $7, $7::timestamptz + interval '2 minutes', 1, NULL, false, NULL, NULL),
		       ($2, $5, 'restart', $8, $7, $7::timestamptz + interval '3 minutes', 3, $6, true, NULL, NULL),
		       ($3, $5, 'scan_harnesses', NULL, $7, $7::timestamptz + interval '4 minutes', 2, $6, true, $7, 'done'),
		       ($4, $5, 'set_state', ''::bytea, $7, $7::timestamptz + interval '5 minutes', 1, NULL, false, NULL, NULL)`,
		queued, dispatched, terminal, standing, approved.ID, streamID, now, restartPayload)

	request := store.Request{
		Key: "revision-two-upgrade", Caller: "upgrade-owner", Procedure: "upgrade-fixture",
		FingerprintVersion: 1, Fingerprints: map[int16][]byte{1: []byte("revision two fingerprint")},
		ReceiptSafe: true,
	}
	reply := []byte("original upgrade reply")
	if saved, err := s.WriteOnce(ctx, request, func(context.Context, *store.Tx) ([]byte, error) {
		return reply, nil
	}); err != nil || !bytes.Equal(saved, reply) {
		t.Fatal("The keyed reply was not saved before the revision 2 upgrade.")
	}
	before := linkMigrationOldRows(t, obs)
	checkOldRows := func(t *testing.T) {
		t.Helper()
		if !storetest.Scalar[bool](t, obs, `SELECT EXISTS
			(SELECT FROM link_machines WHERE machine_id = $1 AND event_mark = 42)`, approved.ID) {
			t.Fatal("The migration did not preserve the approved machine's event mark of 42.")
		}
		if linkMigrationOldRows(t, obs) != before {
			t.Error("The upgrade changed a prior machine, connection, command, receipt or feed row.")
		}
	}
	for range 2 {
		if s.Migrate(ctx, machines.Migrations(), Migrations()) != nil {
			t.Fatal("The current Link manifest did not apply to the version 2 schema.")
		}
		if storetest.Scalar[int](t, obs, "SELECT version FROM schema_migrations WHERE part = 'link'") != 3 {
			t.Fatal("The upgrade did not record Link migration version 3.")
		}
		checkOldRows(t)
	}
	s.Close()
	reopened, err := store.Open(ctx, cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The upgraded revision 2 store did not reopen.")
	}
	t.Cleanup(reopened.Close)
	if reopened.Migrate(ctx, machines.Migrations(), old) == nil {
		t.Error("The version 2 manifest was accepted after the upgrade.")
	}
	if reopened.Migrate(ctx, machines.Migrations(), Migrations()) != nil {
		t.Fatal("The current manifest did not reopen the upgraded schema after the version 2 refusal.")
	}
	reexecuted := false
	saved, err := reopened.WriteOnce(ctx, request, func(context.Context, *store.Tx) ([]byte, error) {
		reexecuted = true
		return nil, nil
	})
	if err != nil || reexecuted || !bytes.Equal(saved, reply) {
		t.Error("The reopened store did not replay the original keyed reply.")
	}
	checkOldRows(t)

	if !storetest.Scalar[bool](t, obs, `SELECT to_regclass('link_runs') IS NOT NULL
		AND (SELECT bool_and(to_jsonb(m) ?& ARRAY['admitted_run_id', 'ready_run_id', 'recovery_set_generation'])
		     FROM link_machines m)
		AND (SELECT bool_and(to_jsonb(c) ?& ARRAY['staged_outcome', 'staged_event_number'])
		     FROM link_commands c)`) {
		t.Fatal("Link migration version 3 did not add the run, recovery and stage schema.")
	}
	if !storetest.Scalar[bool](t, obs, `SELECT NOT EXISTS (SELECT FROM link_runs)
		AND NOT EXISTS (SELECT FROM link_machines WHERE admitted_run_id IS NOT NULL
			OR ready_run_id IS NOT NULL OR recovery_set_generation IS NOT NULL)
		AND NOT EXISTS (SELECT FROM link_commands WHERE staged_outcome IS NOT NULL OR staged_event_number IS NOT NULL)`) {
		t.Error("The upgrade populated new run, recovery or staging metadata for old rows.")
	}

	t.Run("command stage and indexes", func(t *testing.T) {
		stage := fixtureStreamID(t)
		const stagedWatermark int64 = math.MaxInt64
		folderPayload, err := proto.Marshal(&linkv1.Command{Action: &linkv1.Command_LookAtFolder{
			LookAtFolder: &linkv1.LookAtFolder{OperationId: fixtureStreamID(t), Path: "~/workspace"},
		}})
		if err != nil {
			t.Fatal("The folder command payload could not be encoded.")
		}
		outboxFixtureExec(t, obs, `INSERT INTO link_commands
			(id, machine_id, action, payload, created_at, expires_at, dispatched, sent_stream_id, staged_outcome, staged_event_number)
			VALUES ($1, $2, 'look_at_folder', $5, $3, $3::timestamptz + interval '2 minutes', true, $4, 'interrupted', $6)`,
			stage, approved.ID, now, streamID, folderPayload, stagedWatermark)
		if !storetest.Scalar[bool](t, obs, `SELECT EXISTS (SELECT FROM link_commands WHERE id = $1
			AND action = 'look_at_folder' AND payload = $3 AND dispatched AND sent_stream_id = $2
			AND staged_outcome = 'interrupted' AND staged_event_number = $4
			AND outcome IS NULL AND finished_at IS NULL)`, stage, streamID, folderPayload, stagedWatermark) {
			t.Error("The valid dispatched folder command did not retain its staged result and payload.")
		}
		for _, tc := range []struct {
			name, id string
			outcome  *string
			number   *int64
		}{
			{"stage outcome alone", dispatched, proto.String("done"), nil},
			{"stage number alone", dispatched, nil, proto.Int64(43)},
			{"zero stage number", dispatched, proto.String("done"), proto.Int64(0)},
			{"negative stage number", dispatched, proto.String("done"), proto.Int64(-1)},
			{"timeout in stage", dispatched, proto.String("timed_out"), proto.Int64(43)},
			{"removal in stage", dispatched, proto.String("removed"), proto.Int64(43)},
			{"unknown stage outcome", dispatched, proto.String("unrecognized"), proto.Int64(43)},
			{"stage on an undispatched row", queued, proto.String("done"), proto.Int64(43)},
			{"stage on a terminal row", terminal, proto.String("done"), proto.Int64(43)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				requireLinkMigrationConstraint(t, obs, "23514", `UPDATE link_commands
					SET staged_outcome = $2, staged_event_number = $3 WHERE id = $1`, tc.id, tc.outcome, tc.number)
			})
		}
		requireLinkMigrationConstraint(t, obs, "23514", `UPDATE link_commands SET outcome = 'done',
			finished_at = $2, payload = NULL WHERE id = $1`, stage, now)
		requireLinkMigrationConstraint(t, obs, "23514", `UPDATE link_commands SET outcome = 'expired',
			finished_at = $2, payload = NULL, dispatched = true, sent_stream_id = $3 WHERE id = $1`, standing, now, streamID)

		for _, index := range []string{"link_commands_pending", "link_commands_expirable"} {
			predicate := storetest.Scalar[string](t, obs, `SELECT COALESCE((
				SELECT pg_get_expr(indpred, indrelid) FROM pg_index
				WHERE indexrelid = to_regclass($1) AND indisvalid), '')`, index)
			if predicate == "" {
				t.Errorf("The %s index is missing its partial predicate.", index)
				continue
			}
			eligible := storetest.Scalar[bool](t, obs, `SELECT
				count(*) FILTER (WHERE id IN ($1, $2)) = 2
				AND count(*) FILTER (WHERE id IN ($3, $4)) = 0
				AND (count(*) FILTER (WHERE id = $5) = 1) = $6
				FROM link_commands WHERE `+predicate, queued, dispatched, terminal, stage, standing, index == "link_commands_pending")
			if !eligible {
				t.Errorf("The %s index does not preserve old eligibility while excluding staged results.", index)
			}
		}
		stagedPredicate := storetest.Scalar[string](t, obs, `SELECT COALESCE((
			SELECT COALESCE(pg_get_expr(indpred, indrelid), 'true') FROM pg_index
			WHERE indrelid = 'link_commands'::regclass AND indisvalid
				AND indnkeyatts = 3 AND pg_get_indexdef(indexrelid, 1, false) = 'machine_id'
				AND pg_get_indexdef(indexrelid, 2, false) = 'staged_event_number'
				AND pg_get_indexdef(indexrelid, 3, false) = 'seq'
			LIMIT 1), '')`)
		if stagedPredicate == "" || !storetest.Scalar[bool](t, obs, `SELECT EXISTS (
			SELECT FROM link_commands WHERE id = $1 AND (`+stagedPredicate+`))`, stage) {
			t.Error("No usable index includes the staged result in machine, watermark and sequence order.")
		}
		outboxFixtureExec(t, obs, `UPDATE link_commands SET outcome = staged_outcome, finished_at = $2,
			payload = NULL, staged_outcome = NULL, staged_event_number = NULL WHERE id = $1`, stage, now)
	})

	t.Run("run boundaries and removal", func(t *testing.T) {
		run, superseded := fixtureStreamID(t), fixtureStreamID(t)
		outboxFixtureExec(t, obs, `INSERT INTO link_runs (machine_id, run_id, replay_through)
			VALUES ($1, $2, $4), ($1, $3, $4), ($5, $2, 0)`, approved.ID, run, superseded, int64(math.MaxInt64), waiting.ID)
		outboxFixtureExec(t, obs, `UPDATE link_machines SET admitted_run_id = $2, ready_run_id = $3,
			recovery_set_generation = 0 WHERE machine_id = $1`, approved.ID, run, superseded)
		if !storetest.Scalar[bool](t, obs, `SELECT EXISTS (SELECT FROM link_machines
			WHERE machine_id = $1 AND admitted_run_id = $2 AND ready_run_id = $3 AND recovery_set_generation = 0)
			AND (SELECT count(*) FROM link_runs WHERE machine_id = $1 AND replay_through = $4) = 2
			AND EXISTS (SELECT FROM link_runs WHERE machine_id = $5 AND run_id = $2 AND replay_through = 0)`,
			approved.ID, run, superseded, int64(math.MaxInt64), waiting.ID) {
			t.Error("The schema did not preserve both equal run boundaries or the zero and maximum signed boundaries.")
		}
		requireLinkMigrationConstraint(t, obs, "23514", `UPDATE link_machines
			SET recovery_set_generation = -1 WHERE machine_id = $1`, approved.ID)
		requireLinkMigrationConstraint(t, obs, "23514", `INSERT INTO link_runs (machine_id, run_id, replay_through)
			VALUES ($1, $2, -1)`, approved.ID, fixtureStreamID(t))
		requireLinkMigrationConstraint(t, obs, "23505", `INSERT INTO link_runs (machine_id, run_id, replay_through)
			VALUES ($1, $2, 0)`, approved.ID, run)
		requireLinkMigrationConstraint(t, obs, "23503", `INSERT INTO link_runs (machine_id, run_id, replay_through)
			VALUES ($1, $2, 0)`, fixtureStreamID(t), fixtureStreamID(t))

		outboxFixtureExec(t, obs, "DELETE FROM link_machines WHERE machine_id = $1", approved.ID)
		if !storetest.Scalar[bool](t, obs, `SELECT NOT EXISTS (SELECT FROM link_runs WHERE machine_id = $1)
			AND EXISTS (SELECT FROM link_runs WHERE machine_id = $2 AND run_id = $3)
			AND EXISTS (SELECT FROM link_commands WHERE id = $4 AND machine_id = $1
				AND outcome = 'done' AND finished_at IS NOT NULL AND payload IS NULL)`, approved.ID, waiting.ID, run, terminal) {
			t.Error("Deleting a Link machine did not cascade only its run history while retaining its terminal command.")
		}
	})
}

func linkMigrationManifestThrough(t *testing.T, names ...string) store.Migrations {
	t.Helper()
	current := Migrations()
	files := fstest.MapFS{}
	for _, name := range names {
		body, err := fs.ReadFile(current.Files, name)
		if err != nil {
			t.Fatal("A historical Link migration could not be read.")
		}
		files[name] = &fstest.MapFile{Data: body}
	}
	return store.Migrations{Part: current.Part, Files: files}
}

func seedLinkMigrationConnection(t *testing.T, f *presenceFixture, machineID string) string {
	t.Helper()
	streamID := fixtureStreamID(t)
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.r.Lock(ctx, tx, machineID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO link_machines (machine_id, stream_generation, event_mark)
			VALUES ($1, 1, 42)`, machineID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO link_connections
			(machine_id, stream_id, generation, connected_at, last_heard_at, address)
			VALUES ($1, $2, 1, statement_timestamp(), statement_timestamp(), $3)`, machineID, streamID, presenceAddress); err != nil {
			return err
		}
		return f.r.Touch(ctx, tx, machineID)
	}); err != nil {
		t.Fatal("The revision 1 statements could not seed the historical Link connection and mark.")
	}
	return streamID
}

func linkMigrationOldRows(t *testing.T, obs *pgx.Conn) string {
	t.Helper()
	return storetest.Scalar[string](t, obs, `SELECT jsonb_build_array(
		(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.id) FROM machines m),
		(SELECT jsonb_agg(to_jsonb(m) - ARRAY['admitted_run_id', 'ready_run_id', 'recovery_set_generation']
			ORDER BY m.machine_id) FROM link_machines m),
		(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.machine_id) FROM link_connections c),
		(SELECT jsonb_agg(to_jsonb(c) - ARRAY['staged_outcome', 'staged_event_number'] ORDER BY c.seq)
			FROM link_commands c),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.principal_hash, r.request_key) FROM receipts r),
		(SELECT jsonb_agg(to_jsonb(f) ORDER BY f.position) FROM feed f)
	)::text`)
}

func requireLinkMigrationConstraint(t *testing.T, obs *pgx.Conn, code, statement string, args ...any) {
	t.Helper()
	tx, err := obs.Begin(t.Context())
	if err != nil {
		t.Fatal("The constraint probe could not open a transaction.")
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(t.Context(), statement, args...)
	if err == nil {
		t.Errorf("The schema accepted an invalid record expected to fail with SQLSTATE %s.", code)
		return
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("The invalid record failed for a reason other than SQLSTATE %s.", code)
	}
}
