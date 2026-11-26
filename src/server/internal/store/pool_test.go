package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// The helpers of this file carry the prefix "pool" so that the tests of other
// work items in the same package cannot collide with them.

// poolOpen provisions a database, opens a store with eight connections and
// returns the store, its configuration and an observer connection.
func poolOpen(t *testing.T, schema string) (*Store, *pgxpool.Config, *pgx.Conn) {
	t.Helper()
	cfg := storetest.NewDatabase(t, schema)
	if cfg == nil {
		t.Fatal("NewDatabase returned no configuration")
	}
	cfg.MaxConns = 8
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s, err := Open(ctx, cfg, ConnectionPolicy{})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(s.Close)
	return s, cfg, poolObserver(t, cfg)
}

// poolObserver opens an independent connection to the database of the configuration.
func poolObserver(t *testing.T, cfg *pgxpool.Config) *pgx.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatalf("the observer could not connect: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Close(ctx)
	})
	return conn
}

func poolBackendPID(t *testing.T, conn *pgx.Conn) int32 {
	t.Helper()
	var pid int32
	if err := conn.QueryRow(context.Background(), "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatalf("pg_backend_pid failed: %v", err)
	}
	return pid
}

// poolIdleInTransaction counts backends of this database that wait inside an open transaction.
func poolIdleInTransaction(t *testing.T, obs *pgx.Conn) int {
	t.Helper()
	var n int
	err := obs.QueryRow(context.Background(), `
SELECT count(*) FROM pg_stat_activity
WHERE datname = current_database() AND pid <> pg_backend_pid() AND state LIKE 'idle in transaction%'`).Scan(&n)
	if err != nil {
		t.Fatalf("counting open transactions failed: %v", err)
	}
	return n
}

// poolWaitNoOpenTransaction requires that no transaction stays open for more than one second.
func poolWaitNoOpenTransaction(t *testing.T, obs *pgx.Conn) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		n := poolIdleInTransaction(t, obs)
		if n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d backends still hold an open transaction one second after the call returned", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// poolBlockedPIDs lists the backends that wait for the advisory lock of the key.
func poolBlockedPIDs(t *testing.T, obs *pgx.Conn, key storetest.AdvisoryKey) []int32 {
	t.Helper()
	rows, err := obs.Query(context.Background(), `
SELECT pid FROM pg_locks
WHERE locktype = 'advisory' AND NOT granted
  AND classid::bigint = $1 AND objid::bigint = $2 AND objsubid = $3`,
		int64(key.ClassID), int64(key.ObjectID), int16(key.SubID))
	if err != nil {
		t.Fatalf("listing blocked backends failed: %v", err)
	}
	defer rows.Close()
	var pids []int32
	for rows.Next() {
		var pid int32
		if err := rows.Scan(&pid); err != nil {
			t.Fatal(err)
		}
		pids = append(pids, pid)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return pids
}

// poolWaitBlocked waits until exactly want backends wait for the key and returns them.
func poolWaitBlocked(t *testing.T, obs *pgx.Conn, key storetest.AdvisoryKey, want int) []int32 {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		pids := poolBlockedPIDs(t, obs, key)
		if len(pids) == want {
			return pids
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d backends wait for the gate after 15 seconds, want %d", len(pids), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func poolReadOne(t *testing.T, s *Store, within time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	called := false
	err := s.Read(ctx, func(ctx context.Context, tx *ReadTx) error {
		var one int
		if err := tx.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
			return err
		}
		called = one == 1
		return nil
	})
	if err != nil || !called {
		t.Fatalf("a read did not finish within %v: called=%v err=%v", within, called, err)
	}
}

// poolLeakText gathers every text that an error shows to a caller.
func poolLeakText(err error) []string {
	texts := []string{fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)}
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		texts = append(texts, e.Error())
		switch u := e.(type) {
		case interface{ Unwrap() error }:
			walk(u.Unwrap())
		case interface{ Unwrap() []error }:
			for _, inner := range u.Unwrap() {
				walk(inner)
			}
		}
	}
	walk(err)
	return texts
}

func TestDatabasesAreIsolated(t *testing.T) {
	if os.Getenv("LGTM_TEST_POSTGRES") == "" {
		t.Skip("LGTM_TEST_POSTGRES is not set; run through deploy/test/with-postgres.sh")
	}
	const schema = "isolated"
	var mu sync.Mutex
	var names []string
	var barrier sync.WaitGroup
	barrier.Add(2)

	t.Run("group", func(t *testing.T) {
		for _, want := range []string{"value-from-the-first-database", "value-from-the-second-database"} {
			t.Run(want, func(t *testing.T) {
				t.Parallel()
				arrived := false
				arrive := func() {
					if !arrived {
						arrived = true
						barrier.Done()
					}
				}
				defer arrive()

				cfg := storetest.NewDatabase(t, schema)
				if cfg == nil {
					t.Fatal("NewDatabase returned no configuration")
				}
				conn := poolObserver(t, cfg)
				mu.Lock()
				names = append(names, cfg.ConnConfig.Database)
				mu.Unlock()

				// Both databases exist before either table is made.
				arrive()
				barrier.Wait()

				ctx := context.Background()
				var current string
				if err := conn.QueryRow(ctx, "SELECT current_schema()").Scan(&current); err != nil || current != schema {
					t.Fatalf("current_schema = %q (%v), want %q", current, err, schema)
				}
				var existing *string
				if err := conn.QueryRow(ctx, "SELECT to_regclass('shared_table')::text").Scan(&existing); err != nil {
					t.Fatal(err)
				}
				if existing != nil {
					t.Fatalf("table shared_table already exists as %q before this test made it", *existing)
				}
				if _, err := conn.Exec(ctx, "CREATE TABLE shared_table (v text NOT NULL)"); err != nil {
					t.Fatalf("creating the identical table failed: %v", err)
				}
				if _, err := conn.Exec(ctx, "INSERT INTO shared_table (v) VALUES ($1)", want); err != nil {
					t.Fatal(err)
				}
				rows, err := conn.Query(ctx, "SELECT v FROM shared_table")
				if err != nil {
					t.Fatal(err)
				}
				got, err := pgx.CollectRows(rows, pgx.RowTo[string])
				if err != nil || len(got) != 1 || got[0] != want {
					t.Fatalf("read %v (%v), want exactly %q", got, err, want)
				}
			})
		}
	})

	if len(names) != 2 || names[0] == names[1] {
		t.Fatalf("database names = %v, want two different names", names)
	}
	maintenance, err := pgx.ParseConfig(os.Getenv("LGTM_TEST_POSTGRES"))
	if err != nil {
		t.Fatalf("the maintenance connection setting is not valid: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, maintenance)
	if err != nil {
		t.Fatalf("the maintenance database is not reachable: %v", err)
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(ctx, "SELECT datname FROM pg_database WHERE datname = ANY ($1)", names)
	if err != nil {
		t.Fatal(err)
	}
	left, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("databases %v still exist after their tests finished", left)
	}
}

func TestTransactionsReleaseConnection(t *testing.T) {
	s, cfg, obs := poolOpen(t, "release")
	ctx := context.Background()
	if _, err := obs.Exec(ctx, "CREATE TABLE items (id text PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}

	assertClean := func(t *testing.T) {
		t.Helper()
		poolWaitNoOpenTransaction(t, obs)
		readCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		var count int
		err := s.Read(readCtx, func(ctx context.Context, tx *ReadTx) error {
			return tx.QueryRow(ctx, "SELECT count(*) FROM items").Scan(&count)
		})
		if err != nil {
			t.Fatalf("an independent read did not finish within one second: %v", err)
		}
		if count != 0 {
			t.Fatalf("%d rows of the rolled back writes are visible", count)
		}
	}

	// Twelve repetitions exceed the eight connections of the pool, so a leak shows.
	t.Run("cancel in callback", func(t *testing.T) {
		for i := range 12 {
			callCtx, cancel := context.WithCancel(ctx)
			err := s.withWrite(callCtx, func(ctx context.Context, tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, "INSERT INTO items (id) VALUES ($1)", fmt.Sprint("cancel-", i)); err != nil {
					return err
				}
				cancel()
				return ctx.Err()
			})
			cancel()
			if err == nil {
				t.Fatalf("repetition %d returned no error", i)
			}
			assertClean(t)
		}
	})

	t.Run("panic in callback", func(t *testing.T) {
		for i := range 12 {
			func() {
				defer func() { _ = recover() }()
				_ = s.withWrite(ctx, func(ctx context.Context, tx pgx.Tx) error {
					if _, err := tx.Exec(ctx, "INSERT INTO items (id) VALUES ($1)", fmt.Sprint("panic-", i)); err != nil {
						return err
					}
					panic("callback panic of the test")
				})
			}()
			assertClean(t)
		}
	})

	t.Run("read transaction refuses writes", func(t *testing.T) {
		err := s.Read(ctx, func(ctx context.Context, tx *ReadTx) error {
			_, err := tx.Exec(ctx, "INSERT INTO items (id) VALUES ('read-only')")
			return err
		})
		var se *Error
		if !errors.As(err, &se) {
			t.Fatalf("the insert in a read transaction returned %v, want *Error", err)
		}
		if se.SQLState != "25006" {
			t.Fatalf("SQLState = %q, want 25006 (read-only transaction)", se.SQLState)
		}
		assertClean(t)
	})

	t.Run("refused connection returns no store", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
		refused := cfg.Copy()
		refused.ConnConfig.Host = "127.0.0.1"
		refused.ConnConfig.Port = uint16(port)
		refused.ConnConfig.Fallbacks = nil
		openCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		st, err := Open(openCtx, refused, ConnectionPolicy{})
		if err == nil || st != nil {
			if st != nil {
				st.Close()
			}
			t.Fatalf("Open to a refused port returned store %v and error %v", st, err)
		}
	})
}

func TestWritersCannotStarveReaders(t *testing.T) {
	s, _, obs := poolOpen(t, "starve")
	const gateKey int64 = 841941100777
	key := storetest.BigintKey(gateKey)
	ctx := context.Background()

	gatedWrite := func(ctx context.Context) error {
		return s.withWrite(ctx, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::bigint)", gateKey)
			return err
		})
	}

	t.Run("cancelled waiter returns its connection", func(t *testing.T) {
		release := storetest.HoldAdvisoryGate(t, obs, gateKey)
		defer release()
		callCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- gatedWrite(callCtx) }()
		poolWaitBlocked(t, obs, key, 1)
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("a cancelled waiter returned %v, want the context error", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a cancelled waiter did not return within five seconds")
		}
		poolWaitNoOpenTransaction(t, obs)
		poolWaitBlocked(t, obs, key, 0)
		poolReadOne(t, s, time.Second)
	})

	t.Run("eight writers on an eight-connection pool", func(t *testing.T) {
		release := storetest.HoldAdvisoryGate(t, obs, gateKey)
		defer release()
		start := time.Now()
		results := make(chan error, 8)
		for range 8 {
			go func() { results <- gatedWrite(ctx) }()
		}

		// Five writers are admitted and wait on the gate. The other three wait for admission.
		poolWaitBlocked(t, obs, key, 5)
		time.Sleep(250 * time.Millisecond)
		if got := len(poolBlockedPIDs(t, obs, key)); got != 5 {
			t.Fatalf("%d writers hold a connection while waiting, want at most 5", got)
		}
		poolReadOne(t, s, time.Second)

		for i := range 5 {
			select {
			case err := <-results:
				var se *Error
				if !errors.As(err, &se) || se.SQLState != "55P03" || !se.Retryable() {
					t.Fatalf("first-wave waiter %d returned %v, want a retryable 55P03 error", i, err)
				}
			case <-time.After(25 * time.Second):
				t.Fatalf("first-wave waiter %d did not time out within 25 seconds", i)
			}
		}

		release()
		for i := range 3 {
			select {
			case err := <-results:
				if err != nil {
					t.Fatalf("second-wave writer %d failed: %v", i, err)
				}
			case <-time.After(25 * time.Second):
				t.Fatalf("second-wave writer %d did not finish within 25 seconds", i)
			}
		}
		if elapsed := time.Since(start); elapsed > 25*time.Second {
			t.Fatalf("the case took %v, want at most 25 seconds", elapsed)
		}
		poolWaitNoOpenTransaction(t, obs)
	})
}

// poolSpy records every address the driver dials and every name it resolves.
type poolSpy struct {
	mu      sync.Mutex
	dials   []string
	lookups []string
}

func (p *poolSpy) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	p.mu.Lock()
	p.dials = append(p.dials, addr)
	p.mu.Unlock()
	return nil, errors.New("the dial spy never connects")
}

func (p *poolSpy) lookup(table map[string][]string) func(context.Context, string) ([]string, error) {
	return func(ctx context.Context, host string) ([]string, error) {
		p.mu.Lock()
		p.lookups = append(p.lookups, host)
		p.mu.Unlock()
		if addrs, ok := table[host]; ok {
			return addrs, nil
		}
		return nil, fmt.Errorf("the lookup spy knows no host %q", host)
	}
}

func TestOpenRefusesPlainRemote(t *testing.T) {
	const password = "fixture-password-7731"
	tests := []struct {
		name     string
		dsn      string
		lookup   map[string][]string
		policy   ConnectionPolicy
		wantDial string // the literal address that must be dialed; empty means no dial at all
	}{
		{name: "unix socket", dsn: "postgres://u:" + password + "@/db?host=/run/lgtm-fixture&sslmode=disable", wantDial: "/run/lgtm-fixture/.s.PGSQL.5432"},
		{name: "literal loopback", dsn: "postgres://u:" + password + "@127.0.0.1:5432/db?sslmode=disable", wantDial: "127.0.0.1:5432"},
		{name: "literal loopback v6", dsn: "postgres://u:" + password + "@[::1]:5432/db?sslmode=disable", wantDial: "[::1]:5432"},
		{name: "verified remote tls", dsn: "postgres://u:" + password + "@db.fixture.test:5432/db?sslmode=verify-full", lookup: map[string][]string{"db.fixture.test": {"203.0.113.7"}}, wantDial: "203.0.113.7:5432"},
		{name: "compose service on a private address", dsn: "postgres://u:" + password + "@postgres:5432/db?sslmode=disable", lookup: map[string][]string{"postgres": {"10.2.3.4"}}, policy: ConnectionPolicy{AllowPrivateStackPostgres: true}, wantDial: "10.2.3.4:5432"},

		{name: "compose service without the explicit policy", dsn: "postgres://u:" + password + "@postgres:5432/db?sslmode=disable", lookup: map[string][]string{"postgres": {"10.2.3.4"}}},
		{name: "compose service on a public address", dsn: "postgres://u:" + password + "@postgres:5432/db?sslmode=disable", lookup: map[string][]string{"postgres": {"203.0.113.9"}}, policy: ConnectionPolicy{AllowPrivateStackPostgres: true}},
		{name: "compose service with a second host", dsn: "postgres://u:" + password + "@postgres:5432,postgres:5433/db?sslmode=disable", lookup: map[string][]string{"postgres": {"10.2.3.4"}}, policy: ConnectionPolicy{AllowPrivateStackPostgres: true}},
		{name: "another private name", dsn: "postgres://u:" + password + "@database:5432/db?sslmode=disable", lookup: map[string][]string{"database": {"10.2.3.4"}}, policy: ConnectionPolicy{AllowPrivateStackPostgres: true}},
		{name: "loopback alias", dsn: "postgres://u:" + password + "@alias.fixture.test:5432/db?sslmode=disable", lookup: map[string][]string{"alias.fixture.test": {"127.0.0.1"}}},
		{name: "plaintext remote", dsn: "postgres://u:" + password + "@db.fixture.test:5432/db?sslmode=disable", lookup: map[string][]string{"db.fixture.test": {"203.0.113.7"}}},
		{name: "opportunistic remote", dsn: "postgres://u:" + password + "@db.fixture.test:5432/db?sslmode=prefer", lookup: map[string][]string{"db.fixture.test": {"203.0.113.7"}}},
		{name: "unverified remote tls", dsn: "postgres://u:" + password + "@db.fixture.test:5432/db?sslmode=require", lookup: map[string][]string{"db.fixture.test": {"203.0.113.7"}}},
		{name: "chain without hostname check", dsn: "postgres://u:" + password + "@db.fixture.test:5432/db?sslmode=verify-ca", lookup: map[string][]string{"db.fixture.test": {"203.0.113.7"}}},
		{name: "plaintext remote fallback after loopback", dsn: "postgres://u:" + password + "@127.0.0.1:5432,db.fixture.test:5432/db?sslmode=disable", lookup: map[string][]string{"db.fixture.test": {"203.0.113.7"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := pgxpool.ParseConfig(tc.dsn)
			if err != nil {
				t.Fatalf("the fixture connection string does not parse: %v", err)
			}
			cfg.MaxConns = 8
			spy := &poolSpy{}
			cfg.ConnConfig.DialFunc = spy.dial
			cfg.ConnConfig.LookupFunc = spy.lookup(tc.lookup)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			st, err := Open(ctx, cfg, tc.policy)
			if st != nil {
				defer st.Close()
			}
			if err == nil || st != nil {
				t.Fatalf("Open returned store %v and error %v, want an error and no store", st, err)
			}
			for _, text := range poolLeakText(err) {
				for _, secret := range []string{password, "fixture.test", "10.2.3.4"} {
					if strings.Contains(text, secret) {
						t.Fatalf("the error shows %q: %s", secret, text)
					}
				}
			}
			spy.mu.Lock()
			defer spy.mu.Unlock()
			if tc.wantDial == "" {
				if len(spy.dials) != 0 {
					t.Fatalf("rejected endpoint was dialed: %v", spy.dials)
				}
				return
			}
			if len(spy.dials) == 0 || spy.dials[0] != tc.wantDial {
				t.Fatalf("dialed addresses = %v, want the literal %q first", spy.dials, tc.wantDial)
			}
			for _, addr := range spy.dials {
				if addr != tc.wantDial {
					t.Fatalf("dialed %q beyond the one allowed literal %q", addr, tc.wantDial)
				}
			}
		})
	}
}

func TestDatabaseErrorsHideValues(t *testing.T) {
	s, cfg, obs := poolOpen(t, "hidevalues")
	const secret = "fixture-secret-value-4417"
	ctx := context.Background()
	for _, statement := range []string{
		`CREATE TABLE records (
			id text PRIMARY KEY,
			amount integer NOT NULL,
			CONSTRAINT records_amount_check CHECK (amount > 0)
		)`,
		"INSERT INTO records (id, amount) VALUES ('" + secret + "', 1)",
	} {
		if _, err := obs.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	endpoint := net.JoinHostPort(cfg.ConnConfig.Host, fmt.Sprint(cfg.ConnConfig.Port))
	hidden := []string{secret, endpoint, cfg.ConnConfig.Database}
	if cfg.ConnConfig.Password != "" {
		hidden = append(hidden, cfg.ConnConfig.Password)
	}
	assertHidden := func(t *testing.T, err error) {
		t.Helper()
		for _, text := range poolLeakText(err) {
			for _, value := range hidden {
				if strings.Contains(text, value) {
					t.Fatalf("the error shows %q: %s", value, text)
				}
			}
		}
	}
	write := func(callback func(context.Context, pgx.Tx) error) error {
		return s.withWrite(ctx, callback)
	}

	t.Run("check violation", func(t *testing.T) {
		err := write(func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO records (id, amount) VALUES ($1, -1)", secret+"-new")
			return err
		})
		var se *Error
		if !errors.As(err, &se) || se.SQLState != "23514" || se.Constraint != "records_amount_check" {
			t.Fatalf("got %v, want *Error with SQLState 23514 and constraint records_amount_check", err)
		}
		if se.Outcome() != OutcomeRolledBack {
			t.Fatalf("Outcome = %q, want %q", se.Outcome(), OutcomeRolledBack)
		}
		assertHidden(t, err)
	})

	t.Run("unique violation", func(t *testing.T) {
		err := write(func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO records (id, amount) VALUES ($1, 1)", secret)
			return err
		})
		var se *Error
		if !errors.As(err, &se) || se.SQLState != "23505" || se.Constraint != "records_pkey" {
			t.Fatalf("got %v, want *Error with SQLState 23505 and constraint records_pkey", err)
		}
		assertHidden(t, err)
	})

	t.Run("server rejects a value of the wrong type", func(t *testing.T) {
		err := write(func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO records (id, amount) VALUES ('other', $1::text::integer)", secret)
			return err
		})
		var se *Error
		if err == nil || !errors.As(err, &se) {
			t.Fatalf("got %v, want *Error", err)
		}
		assertHidden(t, err)
	})

	t.Run("driver cannot encode an argument", func(t *testing.T) {
		err := write(func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO records (id, amount) VALUES ('other', $1)", struct{ Value string }{secret})
			return err
		})
		var se *Error
		if err == nil || !errors.As(err, &se) {
			t.Fatalf("got %v, want *Error", err)
		}
		assertHidden(t, err)
	})

	t.Run("connection closed mid-transaction", func(t *testing.T) {
		err := write(func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "SELECT '"+secret+"', pg_terminate_backend(pg_backend_pid())")
			return err
		})
		var se *Error
		if !errors.As(err, &se) {
			t.Fatalf("got %v, want *Error", err)
		}
		if se.Outcome() != OutcomeRolledBack {
			t.Fatalf("Outcome = %q, want %q for a loss before commit", se.Outcome(), OutcomeRolledBack)
		}
		assertHidden(t, err)
		poolReadOne(t, s, 5*time.Second)
	})

	t.Run("callback wraps a SQLSTATE error", func(t *testing.T) {
		err := write(func(ctx context.Context, tx pgx.Tx) error {
			return fmt.Errorf("while saving %s: %w", secret, &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "records_pkey",
				Message:        "duplicate key value " + secret,
				Detail:         "Key (id)=(" + secret + ") already exists.",
			})
		})
		var se *Error
		if !errors.As(err, &se) || se.SQLState != "23505" || se.Constraint != "records_pkey" {
			t.Fatalf("got %v, want *Error with SQLState 23505 and constraint records_pkey", err)
		}
		assertHidden(t, err)
	})

	t.Run("row iteration error in a read", func(t *testing.T) {
		err := s.Read(ctx, func(ctx context.Context, tx *ReadTx) error {
			rows, err := tx.Query(ctx, "SELECT amount / (amount - 1), '"+secret+"' FROM records")
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
			}
			return rows.Err()
		})
		var se *Error
		if !errors.As(err, &se) || se.SQLState != "22012" {
			t.Fatalf("got %v, want *Error with SQLState 22012", err)
		}
		assertHidden(t, err)
	})

	t.Run("domain sentinel keeps its identity", func(t *testing.T) {
		sentinel := NewDomainError("The fixture request cannot be saved.")
		if sentinel == nil {
			t.Fatal("NewDomainError returned nil")
		}
		err := write(func(ctx context.Context, tx pgx.Tx) error { return sentinel })
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v, want the sentinel itself", err)
		}
		if err.Error() != "The fixture request cannot be saved." {
			t.Fatalf("the sentinel message changed to %q", err.Error())
		}
	})
}

// poolRecordingTB records the outcome of a fixture that needs a test to stop.
type poolRecordingTB struct {
	testing.TB
	mu       sync.Mutex
	failed   bool
	skipped  bool
	messages []string
	cleanups []func()
}

func (r *poolRecordingTB) Helper() {}

func (r *poolRecordingTB) record(failed, skipped bool, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = r.failed || failed
	r.skipped = r.skipped || skipped
	r.messages = append(r.messages, fmt.Sprint(args...))
}

func (r *poolRecordingTB) Fatal(args ...any) { r.record(true, false, args...); runtime.Goexit() }
func (r *poolRecordingTB) Fatalf(format string, args ...any) {
	r.record(true, false, fmt.Sprintf(format, args...))
	runtime.Goexit()
}
func (r *poolRecordingTB) Error(args ...any) { r.record(true, false, args...) }
func (r *poolRecordingTB) Errorf(format string, args ...any) {
	r.record(true, false, fmt.Sprintf(format, args...))
}
func (r *poolRecordingTB) Fail()    { r.record(true, false) }
func (r *poolRecordingTB) FailNow() { r.record(true, false); runtime.Goexit() }
func (r *poolRecordingTB) Skip(args ...any) {
	r.record(false, true, args...)
	runtime.Goexit()
}
func (r *poolRecordingTB) Skipf(format string, args ...any) {
	r.record(false, true, fmt.Sprintf(format, args...))
	runtime.Goexit()
}
func (r *poolRecordingTB) SkipNow()                { r.record(false, true); runtime.Goexit() }
func (r *poolRecordingTB) Log(args ...any)         { r.record(false, false, args...) }
func (r *poolRecordingTB) Logf(f string, a ...any) { r.record(false, false, fmt.Sprintf(f, a...)) }
func (r *poolRecordingTB) Name() string            { return "recording" }
func (r *poolRecordingTB) Cleanup(f func()) {
	r.mu.Lock()
	r.cleanups = append(r.cleanups, f)
	r.mu.Unlock()
}

func TestNewDatabaseRejectsWrongMaintenanceName(t *testing.T) {
	const password = "fixture-password-9902"
	// The port is closed, so a fixture that dials before it checks the name cannot succeed either.
	t.Setenv("LGTM_TEST_POSTGRES", "postgres://fixture_user:"+password+"@127.0.0.1:1/some_other_database?sslmode=disable")

	rec := &poolRecordingTB{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		storetest.NewDatabase(rec, "refused")
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("NewDatabase did not return within 30 seconds")
	}
	for i := len(rec.cleanups) - 1; i >= 0; i-- {
		rec.cleanups[i]()
	}

	if rec.skipped {
		t.Fatalf("NewDatabase skipped instead of refusing: %v", rec.messages)
	}
	if !rec.failed {
		t.Fatal("NewDatabase accepted a maintenance database with another name")
	}
	joined := strings.Join(rec.messages, "\n")
	if !strings.Contains(joined, "maintenance") && !strings.Contains(joined, "lgtm_store_test") {
		t.Fatalf("the diagnostic does not name the maintenance database: %q", joined)
	}
	for _, secret := range []string{password, "fixture_user"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("the diagnostic shows %q: %s", secret, joined)
		}
	}
}
