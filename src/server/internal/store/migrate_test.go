package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// migrateOpen provisions one database and opens count stores on it.
// Several stores on one database stand for several server processes.
func migrateOpen(t *testing.T, schema string, count int) ([]*Store, *pgx.Conn) {
	t.Helper()
	first, cfg, obs := poolOpen(t, schema)
	stores := []*Store{first}
	for len(stores) < count {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		more, err := Open(ctx, cfg, ConnectionPolicy{})
		cancel()
		if err != nil {
			t.Fatalf("opening another store failed: %v", err)
		}
		t.Cleanup(more.Close)
		stores = append(stores, more)
	}
	return stores, obs
}

func migrateFiles(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return fsys
}

// migrateLedger reads schema_migrations; it reports false when the ledger does not exist.
func migrateLedger(t *testing.T, obs *pgx.Conn) (map[string]int, bool) {
	t.Helper()
	ctx := context.Background()
	var exists *string
	if err := obs.QueryRow(ctx, "SELECT to_regclass('schema_migrations')::text").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists == nil {
		return nil, false
	}
	rows, err := obs.Query(ctx, "SELECT part, version FROM schema_migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ledger := map[string]int{}
	for rows.Next() {
		var part string
		var version int
		if err := rows.Scan(&part, &version); err != nil {
			t.Fatal(err)
		}
		if _, dup := ledger[part]; dup {
			t.Fatalf("the ledger holds part %q twice", part)
		}
		ledger[part] = version
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ledger, true
}

func migrateTableExists(t *testing.T, obs *pgx.Conn, name string) bool {
	t.Helper()
	var exists *string
	if err := obs.QueryRow(context.Background(), "SELECT to_regclass($1)::text", name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists != nil
}

func migrateInts(t *testing.T, obs *pgx.Conn, query string) []int {
	t.Helper()
	rows, err := obs.Query(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func migrateEqualLedger(t *testing.T, got, want map[string]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ledger = %v, want %v", got, want)
	}
	for part, version := range want {
		if got[part] != version {
			t.Fatalf("ledger = %v, want %v", got, want)
		}
	}
}

// migrateStart runs Migrate in a goroutine and returns the channel of its result.
func migrateStart(s *Store, ctx context.Context, parts ...Migrations) <-chan error {
	done := make(chan error, 1)
	go func() { done <- s.Migrate(ctx, parts...) }()
	return done
}

func migrateAwait(t *testing.T, name string, done <-chan error, within time.Duration) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s failed: %v", name, err)
		}
	case <-time.After(within):
		t.Fatalf("%s did not finish within %v", name, within)
	}
}

func TestMigrateColdConcurrentStarts(t *testing.T) {
	stores, obs := migrateOpen(t, "cold", 2)
	ctx := context.Background()
	observerPID := poolBackendPID(t, obs)
	key := storetest.BigintKey(migrationLockID)
	release := storetest.HoldAdvisoryGate(t, obs, migrationLockID)

	first := migrateStart(stores[0], ctx)
	second := migrateStart(stores[1], ctx)
	waiters := poolWaitBlocked(t, obs, key, 2)
	for _, pid := range waiters {
		storetest.WaitForBlock(t, obs, pid, observerPID, key)
	}

	// A waiting backend must not have started the ledger. An uncommitted ledger
	// can be invisible to this connection, so the statement and the locks are read.
	for _, pid := range waiters {
		var query string
		var heldCreate int
		err := obs.QueryRow(ctx, `
SELECT (SELECT query FROM pg_stat_activity WHERE pid = $1),
       (SELECT count(*) FROM pg_locks
        WHERE pid = $1 AND granted AND locktype = 'relation' AND mode = 'AccessExclusiveLock')`, pid).Scan(&query, &heldCreate)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(query, "schema_migrations") {
			t.Fatalf("backend %d runs %q while it waits for the migration lock", pid, query)
		}
		if heldCreate != 0 {
			t.Fatalf("backend %d already holds %d exclusive relation locks before it has the migration lock", pid, heldCreate)
		}
	}

	release()
	migrateAwait(t, "the first cold start", first, 30*time.Second)
	migrateAwait(t, "the second cold start", second, 30*time.Second)

	ledger, exists := migrateLedger(t, obs)
	if !exists {
		t.Fatal("no ledger exists after both starts succeeded")
	}
	migrateEqualLedger(t, ledger, map[string]int{"store": 1})
	for _, table := range []string{"feed", "feed_retention", "receipts"} {
		if !migrateTableExists(t, obs, table) {
			t.Fatalf("store table %q is missing", table)
		}
	}
}

func TestMigrateConcurrentStarts(t *testing.T) {
	stores, obs := migrateOpen(t, "concurrent", 2)
	ctx := context.Background()
	observerPID := poolBackendPID(t, obs)
	if err := stores[0].Migrate(ctx); err != nil {
		t.Fatalf("bootstrapping the store ledger failed: %v", err)
	}

	const gateKey int64 = 841941100801
	parts := []Migrations{
		{Part: "alpha", Files: migrateFiles(map[string]string{
			"0001_first.sql": fmt.Sprintf(`
SELECT pg_advisory_xact_lock(%d);
CREATE TABLE alpha_log (id serial PRIMARY KEY, step integer NOT NULL);
INSERT INTO alpha_log (step) VALUES (1);`, gateKey),
			"0002_second.sql": "INSERT INTO alpha_log (step) VALUES (2);",
		})},
		{Part: "beta", Files: migrateFiles(map[string]string{
			"0001_first.sql": `
CREATE TABLE beta_log (id serial PRIMARY KEY, step integer NOT NULL);
INSERT INTO beta_log (step) VALUES (1);`,
		})},
	}

	releaseGate := storetest.HoldAdvisoryGate(t, obs, gateKey)
	first := migrateStart(stores[0], ctx, parts...)
	holders := poolWaitBlocked(t, obs, storetest.BigintKey(gateKey), 1)
	firstPID := holders[0]

	// The first call now holds the startup lock and waits inside its first migration.
	second := migrateStart(stores[1], ctx, parts...)
	deadline := time.Now().Add(15 * time.Second)
	for {
		waiters := poolBlockedPIDs(t, obs, storetest.BigintKey(migrationLockID))
		if len(waiters) == 1 {
			storetest.WaitForBlock(t, obs, waiters[0], firstPID, storetest.BigintKey(migrationLockID))
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the second call does not wait on the startup lock; %d waiters", len(waiters))
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The observer is not the holder of the startup lock.
	if observerPID == firstPID {
		t.Fatal("the observer and the first call share a backend")
	}

	releaseGate()
	migrateAwait(t, "the first start", first, 30*time.Second)
	migrateAwait(t, "the second start", second, 30*time.Second)

	want := map[string]int{"store": 1, "alpha": 2, "beta": 1}
	ledger, _ := migrateLedger(t, obs)
	migrateEqualLedger(t, ledger, want)
	alpha := migrateInts(t, obs, "SELECT step FROM alpha_log ORDER BY id")
	beta := migrateInts(t, obs, "SELECT step FROM beta_log ORDER BY id")
	if fmt.Sprint(alpha) != "[1 2]" || fmt.Sprint(beta) != "[1]" {
		t.Fatalf("fixture rows alpha=%v beta=%v, want [1 2] and [1], each change applied once and in order", alpha, beta)
	}

	if err := stores[0].Migrate(ctx, parts...); err != nil {
		t.Fatalf("a third start failed: %v", err)
	}
	ledger, _ = migrateLedger(t, obs)
	migrateEqualLedger(t, ledger, want)
	alpha = migrateInts(t, obs, "SELECT step FROM alpha_log ORDER BY id")
	beta = migrateInts(t, obs, "SELECT step FROM beta_log ORDER BY id")
	if fmt.Sprint(alpha) != "[1 2]" || fmt.Sprint(beta) != "[1]" {
		t.Fatalf("a third start changed the rows: alpha=%v beta=%v", alpha, beta)
	}
}

func TestMigrateUsesStartupBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("this test runs for about 35 seconds")
	}
	ctx := context.Background()

	t.Run("waiting and a long statement stay inside the budget", func(t *testing.T) {
		stores, obs := migrateOpen(t, "budget", 2)
		parts := []Migrations{{Part: "slow", Files: migrateFiles(map[string]string{
			"0001_probe.sql": `
CREATE TABLE budget_probe AS
    SELECT current_setting('statement_timeout') AS statement_timeout,
           current_setting('lock_timeout') AS lock_timeout;
SELECT pg_sleep(20);`,
		})}}
		observerPID := poolBackendPID(t, obs)
		key := storetest.BigintKey(migrationLockID)
		release := storetest.HoldAdvisoryGate(t, obs, migrationLockID)
		start := time.Now()
		first := migrateStart(stores[0], ctx, parts...)
		second := migrateStart(stores[1], ctx, parts...)
		for _, pid := range poolWaitBlocked(t, obs, key, 2) {
			storetest.WaitForBlock(t, obs, pid, observerPID, key)
		}
		// Twelve seconds exceed the ten-second lock wait of an ordinary write.
		time.Sleep(12 * time.Second)
		release()
		migrateAwait(t, "the first startup", first, 60*time.Second)
		migrateAwait(t, "the second startup", second, 60*time.Second)
		if elapsed := time.Since(start); elapsed > 60*time.Second {
			t.Fatalf("both startups took %v, want at most 60 seconds", elapsed)
		}
		var statementTimeout, lockTimeout string
		if err := obs.QueryRow(ctx, "SELECT statement_timeout, lock_timeout FROM budget_probe").Scan(&statementTimeout, &lockTimeout); err != nil {
			t.Fatal(err)
		}
		if statementTimeout != "5min" {
			t.Fatalf("statement_timeout during migration = %q, want 5min", statementTimeout)
		}
		if lockTimeout != "0" {
			t.Fatalf("lock_timeout during migration = %q, want 0", lockTimeout)
		}
	})

	t.Run("a shorter caller deadline cancels the migration", func(t *testing.T) {
		stores, obs := migrateOpen(t, "deadline", 1)
		parts := []Migrations{{Part: "sleepy", Files: migrateFiles(map[string]string{
			"0001_sleep.sql": "CREATE TABLE sleepy_table (id integer); SELECT pg_sleep(30);",
		})}}
		callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		done := migrateStart(stores[0], callCtx, parts...)

		// The migration must be running when the deadline arrives.
		started := false
		for end := time.Now().Add(6 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
			var n int
			err := obs.QueryRow(ctx, `
SELECT count(*) FROM pg_stat_activity
WHERE datname = current_database() AND pid <> pg_backend_pid() AND state = 'active' AND query LIKE '%pg_sleep(30)%'`).Scan(&n)
			if err != nil {
				t.Fatal(err)
			}
			if n > 0 {
				started = true
				break
			}
		}
		if !started {
			t.Fatal("the migration did not start before the caller deadline")
		}
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("Migrate returned no error after the caller deadline")
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Migrate returned %v, want the context deadline error", err)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("Migrate did not stop after the caller deadline")
		}
		if _, exists := migrateLedger(t, obs); exists {
			t.Fatal("a ledger exists after a cancelled first migration")
		}
		for _, table := range []string{"sleepy_table", "feed", "receipts"} {
			if migrateTableExists(t, obs, table) {
				t.Fatalf("table %q exists after a cancelled migration", table)
			}
		}
	})
}

func TestMigrateFailureRollsBack(t *testing.T) {
	stores, obs := migrateOpen(t, "rollback", 1)
	s := stores[0]
	ctx := context.Background()

	v1 := map[string]string{"0001_one.sql": "CREATE TABLE gamma_one (id integer);"}
	if err := s.Migrate(ctx, Migrations{Part: "gamma", Files: migrateFiles(v1)}); err != nil {
		t.Fatalf("version 1 failed: %v", err)
	}
	broken := map[string]string{
		"0001_one.sql": v1["0001_one.sql"],
		"0002_two.sql": "CREATE TABLE gamma_two (id integer); SELECT 1 / 0;",
	}
	if err := s.Migrate(ctx, Migrations{Part: "gamma", Files: migrateFiles(broken)}); err == nil {
		t.Fatal("a failing version 2 returned no error")
	}
	if migrateTableExists(t, obs, "gamma_two") {
		t.Fatal("the table of the failed batch exists")
	}
	ledger, _ := migrateLedger(t, obs)
	migrateEqualLedger(t, ledger, map[string]int{"store": 1, "gamma": 1})

	// Only the unmerged fixture file is replaced.
	fixed := map[string]string{
		"0001_one.sql": v1["0001_one.sql"],
		"0002_two.sql": "CREATE TABLE gamma_two (id integer);",
	}
	if err := s.Migrate(ctx, Migrations{Part: "gamma", Files: migrateFiles(fixed)}); err != nil {
		t.Fatalf("the rerun with the repaired file failed: %v", err)
	}
	if !migrateTableExists(t, obs, "gamma_two") {
		t.Fatal("the repaired migration did not create its table")
	}
	ledger, _ = migrateLedger(t, obs)
	migrateEqualLedger(t, ledger, map[string]int{"store": 1, "gamma": 2})
}

func TestMigrateValidatesManifest(t *testing.T) {
	stores, obs := migrateOpen(t, "manifest", 1)
	s := stores[0]
	ctx := context.Background()
	ok := func(files ...string) fstest.MapFS {
		m := map[string]string{}
		for _, name := range files {
			m[name] = "SELECT 1;"
		}
		return migrateFiles(m)
	}

	tests := []struct {
		name  string
		parts []Migrations
	}{
		{"duplicate part names", []Migrations{{Part: "dup", Files: ok("0001_a.sql")}, {Part: "dup", Files: ok("0001_b.sql")}}},
		{"reserved store name", []Migrations{{Part: "store", Files: ok("0001_a.sql")}}},
		{"empty part name", []Migrations{{Part: "", Files: ok("0001_a.sql")}}},
		{"uppercase part name", []Migrations{{Part: "Upper", Files: ok("0001_a.sql")}}},
		{"part name starting with a digit", []Migrations{{Part: "1part", Files: ok("0001_a.sql")}}},
		{"part name with a hyphen", []Migrations{{Part: "bad-name", Files: ok("0001_a.sql")}}},
		{"duplicate numbers", []Migrations{{Part: "p", Files: ok("0001_a.sql", "0001_b.sql")}}},
		{"gap in numbers", []Migrations{{Part: "p", Files: ok("0001_a.sql", "0003_c.sql")}}},
		{"numbers not starting at one", []Migrations{{Part: "p", Files: ok("0002_b.sql")}}},
		{"three digits", []Migrations{{Part: "p", Files: ok("001_a.sql")}}},
		{"uppercase description", []Migrations{{Part: "p", Files: ok("0001_Upper.sql")}}},
		{"empty description", []Migrations{{Part: "p", Files: ok("0001_.sql")}}},
		{"hyphen in description", []Migrations{{Part: "p", Files: ok("0001_a-b.sql")}}},
		{"wrong extension", []Migrations{{Part: "p", Files: ok("0001_a.txt")}}},
		{"stray file", []Migrations{{Part: "p", Files: ok("0001_a.sql", "notes.md")}}},
		{"valid part after an invalid one", []Migrations{{Part: "good", Files: ok("0001_a.sql")}, {Part: "p", Files: ok("0002_b.sql")}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.Migrate(ctx, tc.parts...); err == nil {
				t.Fatal("the invalid manifest was accepted")
			}
			if _, exists := migrateLedger(t, obs); exists {
				t.Fatal("a ledger exists after a refused manifest")
			}
			for _, table := range []string{"feed", "feed_retention", "receipts", "good", "p"} {
				if migrateTableExists(t, obs, table) {
					t.Fatalf("table %q exists after a refused manifest", table)
				}
			}
		})
	}

	valid := Migrations{Part: "delta", Files: migrateFiles(map[string]string{
		"0001_one.sql": "CREATE TABLE delta_one (id integer);",
		"0002_two.sql": "CREATE TABLE delta_two (id integer);",
	})}
	if err := s.Migrate(ctx, valid); err != nil {
		t.Fatalf("the valid manifest failed: %v", err)
	}
	before, _ := migrateLedger(t, obs)
	migrateEqualLedger(t, before, map[string]int{"store": 1, "delta": 2})

	older := Migrations{Part: "delta", Files: migrateFiles(map[string]string{"0001_one.sql": "CREATE TABLE delta_one (id integer);"})}
	if err := s.Migrate(ctx, older); err == nil {
		t.Fatal("a manifest older than the recorded version was accepted")
	}
	if err := s.Migrate(ctx); err == nil {
		t.Fatal("a manifest without the recorded part delta was accepted")
	}
	if err := s.Migrate(ctx, Migrations{Part: "other", Files: ok("0001_a.sql")}); err == nil {
		t.Fatal("a manifest that swaps the recorded part for another was accepted")
	}
	after, _ := migrateLedger(t, obs)
	migrateEqualLedger(t, after, before)
	if migrateTableExists(t, obs, "other") {
		t.Fatal("a refused manifest changed the database")
	}
}
