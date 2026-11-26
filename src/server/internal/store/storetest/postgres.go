// Package storetest holds the fixtures of the store's tests against real PostgreSQL.
package storetest

import (
	"context"
	"crypto/rand"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var schemaName = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// NewDatabase copies the disposable maintenance connection into a new database.
// The caller registers application connection cleanup after NewDatabase returns.
func NewDatabase(t testing.TB, schema string) *pgxpool.Config {
	t.Helper()
	setting := os.Getenv("LGTM_TEST_POSTGRES")
	if setting == "" {
		t.Skip("LGTM_TEST_POSTGRES is not set. Run through deploy/test/with-postgres.sh.")
	}

	config, err := pgxpool.ParseConfig(setting)
	if err != nil {
		t.Fatal("The PostgreSQL fixture setting is invalid.")
	}
	maintenance := config.ConnConfig
	if maintenance.Database != "lgtm_store_test" {
		t.Fatal("The maintenance database must be lgtm_store_test.")
	}
	network, _ := pgconn.NetworkAddress(maintenance.Host, maintenance.Port)
	addr, _ := netip.ParseAddr(maintenance.Host)
	if len(maintenance.Fallbacks) != 0 || maintenance.TLSConfig != nil ||
		(network != "unix" && !addr.Unmap().IsLoopback()) {
		t.Fatal("The PostgreSQL fixture needs one plaintext loopback or Unix socket endpoint.")
	}
	if !schemaName.MatchString(schema) {
		t.Fatal("The PostgreSQL fixture schema is invalid.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, maintenance.Copy())
	if err != nil {
		t.Fatal("The PostgreSQL fixture maintenance database is unavailable.")
	}
	defer closeConnection(conn)

	var canCreate bool
	err = conn.QueryRow(ctx, "SELECT rolcreatedb FROM pg_roles WHERE rolname = current_user").Scan(&canCreate)
	if err != nil || !canCreate {
		t.Fatal("The PostgreSQL fixture login needs the CREATEDB privilege.")
	}

	database := "lgtm_test_" + strings.ToLower(rand.Text())
	quotedDatabase := pgx.Identifier{database}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+quotedDatabase+" TEMPLATE template0"); err != nil {
		t.Fatal("The PostgreSQL fixture database could not be created.")
	}
	t.Cleanup(func() { dropDatabase(t, maintenance, quotedDatabase) })

	app := config.Copy()
	app.MaxConns = max(app.MaxConns, 8)
	app.ConnConfig.Database = database
	if app.ConnConfig.RuntimeParams == nil {
		app.ConnConfig.RuntimeParams = make(map[string]string)
	}
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	app.ConnConfig.RuntimeParams["search_path"] = quotedSchema

	setup, err := pgx.ConnectConfig(ctx, app.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The PostgreSQL fixture database is unavailable.")
	}
	defer closeConnection(setup)
	createSchema := "CREATE SCHEMA "
	if schema == "public" {
		// The template database already contains the public schema.
		createSchema += "IF NOT EXISTS "
	}
	if _, err := setup.Exec(ctx, createSchema+quotedSchema); err != nil {
		t.Fatal("The PostgreSQL fixture schema could not be created.")
	}
	return app
}

func dropDatabase(t testing.TB, maintenance *pgx.ConnConfig, quotedDatabase string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, maintenance.Copy())
	if err != nil {
		t.Error("The PostgreSQL fixture maintenance database was unavailable during cleanup.")
		return
	}
	defer closeConnection(conn)
	if _, err := conn.Exec(ctx, "DROP DATABASE "+quotedDatabase+" WITH (FORCE)"); err != nil {
		t.Error("The PostgreSQL fixture database could not be dropped.")
	}
}

func closeConnection(conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = conn.Close(ctx)
}
