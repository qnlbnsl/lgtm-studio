package migrated

import (
	"context"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

func TestNewMigratedStore(t *testing.T) {
	maintenanceURL := os.Getenv("LGTM_TEST_POSTGRES")
	if maintenanceURL == "" {
		t.Skip("LGTM_TEST_POSTGRES is not set; run through deploy/test/with-postgres.sh")
	}
	part := store.Migrations{Part: "widgets", Files: fstest.MapFS{
		"0001_widgets.sql": &fstest.MapFile{Data: []byte("CREATE TABLE widgets (id integer PRIMARY KEY);")},
	}}

	var databaseName string
	// The inner test ends first, so its cleanups have run when the outer test looks again.
	t.Run("use", func(t *testing.T) {
		s := NewStore(t, "migrated_fixture", part)
		if s == nil {
			t.Fatal("NewStore returned no store")
		}
		var hasTable bool
		var ledger int
		err := s.Read(context.Background(), func(ctx context.Context, tx *store.ReadTx) error {
			if err := tx.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, "SELECT to_regclass('widgets') IS NOT NULL").Scan(&hasTable); err != nil {
				return err
			}
			return tx.QueryRow(ctx, "SELECT version FROM schema_migrations WHERE part = 'widgets'").Scan(&ledger)
		})
		if err != nil {
			t.Fatalf("reading the migrated store failed: %v", err)
		}
		if !hasTable || ledger != 1 {
			t.Fatalf("part table present = %v, ledger version = %d, want true and 1", hasTable, ledger)
		}
	})

	if databaseName == "" {
		t.Fatal("the database name was not observed")
	}
	cfg, err := pgx.ParseConfig(maintenanceURL)
	if err != nil {
		t.Fatalf("the maintenance connection setting is not valid: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("the maintenance database is not reachable: %v", err)
	}
	defer conn.Close(context.Background())
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", databaseName).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("database %q still exists after the test finished", databaseName)
	}
}
