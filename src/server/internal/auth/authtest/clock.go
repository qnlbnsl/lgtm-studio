// Package authtest holds the fixtures of the tests of the session code against real PostgreSQL.
package authtest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const fixtureSchema = "clockfix"

// ClockStore opens a migrated store whose unqualified clock_timestamp() returns the time
// that the returned setter last stored, and starts at the time at.
// The returned connection is a fixture connection on the same database for the test's own queries.
// The migrations of the parts are applied on the original path, before the fixture function exists,
// so that the tables are in the fixture schema and column defaults stay bound to the built-in clock.
func ClockStore(t testing.TB, at time.Time, parts ...store.Migrations) (*store.Store, *pgx.Conn, func(time.Time)) {
	t.Helper()
	config := storetest.NewDatabase(t, fixtureSchema)
	ctx := t.Context()

	original, err := store.Open(ctx, config, store.ConnectionPolicy{})
	if err != nil {
		t.Fatalf("opening the fixture store failed: %v", err)
	}
	if err := original.Migrate(ctx, parts...); err != nil {
		original.Close()
		t.Fatalf("applying the fixture migrations failed: %v", err)
	}

	observer, err := pgx.ConnectConfig(ctx, config.ConnConfig.Copy())
	if err != nil {
		original.Close()
		t.Fatalf("the fixture connection could not connect: %v", err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observer.Close(cleanup)
	})

	for _, statement := range []string{
		"CREATE TABLE " + fixtureSchema + ".fixture_clock (at timestamptz NOT NULL)",
		"CREATE FUNCTION " + fixtureSchema + ".clock_timestamp() RETURNS timestamptz LANGUAGE sql STABLE AS " +
			"$$ SELECT at FROM " + fixtureSchema + ".fixture_clock $$",
	} {
		if _, err := observer.Exec(ctx, statement); err != nil {
			original.Close()
			t.Fatalf("installing the fixture clock failed: %v", err)
		}
	}
	if _, err := observer.Exec(ctx, "INSERT INTO "+fixtureSchema+".fixture_clock (at) VALUES ($1)", at.UTC()); err != nil {
		original.Close()
		t.Fatalf("starting the fixture clock failed: %v", err)
	}
	original.Close()

	// Listing the fixture schema before pg_catalog makes the fixture function win the lookup.
	shifted := config.Copy()
	shifted.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{fixtureSchema}.Sanitize() + ", pg_catalog"
	s, err := store.Open(ctx, shifted, store.ConnectionPolicy{})
	if err != nil {
		t.Fatalf("reopening the fixture store failed: %v", err)
	}
	t.Cleanup(s.Close)

	var got time.Time
	if err := s.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		return tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&got)
	}); err != nil {
		t.Fatalf("reading the fixture clock through the store failed: %v", err)
	}
	if want := at.Truncate(time.Microsecond); !got.Equal(want) {
		t.Fatalf("the store's clock_timestamp() returned %v, want the fixture time %v", got, want)
	}

	set := func(next time.Time) {
		t.Helper()
		if _, err := observer.Exec(t.Context(), "UPDATE "+fixtureSchema+".fixture_clock SET at = $1", next.UTC()); err != nil {
			t.Fatalf("moving the fixture clock failed: %v", err)
		}
	}
	return s, observer, set
}
