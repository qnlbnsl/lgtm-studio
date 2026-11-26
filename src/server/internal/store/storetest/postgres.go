// Package storetest holds the fixtures of the store's tests against real PostgreSQL.
package storetest

import (
	"context"
	"crypto/rand"
	"errors"
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
	// A loaded host can take longer than ten seconds to finish a forced drop, for instance behind a checkpoint.
	// The drop returns as soon as it completes, so the longer deadline only delays a real failure.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, maintenance.Copy())
	if err != nil {
		t.Error("The PostgreSQL fixture maintenance database was unavailable during cleanup.")
		return
	}
	defer closeConnection(conn)
	// A forced drop signals every backend of the database and fails with SQLSTATE 42501 when one of them
	// belongs to another role, for instance an autovacuum worker that is attached for a moment. The worker
	// leaves soon, so the drop is repeated until the deadline instead of failing a test that is not at fault.
	var dropErr error
	for {
		_, dropErr = conn.Exec(ctx, "DROP DATABASE "+quotedDatabase+" WITH (FORCE)")
		var pgErr *pgconn.PgError
		if dropErr == nil || !errors.As(dropErr, &pgErr) || pgErr.Code != "42501" || ctx.Err() != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := dropErr; err != nil {
		reason := "the cleanup deadline passed"
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			reason = "SQLSTATE " + pgErr.Code
		} else if ctx.Err() == nil {
			reason = "the connection failed"
		}
		t.Errorf("The PostgreSQL fixture database could not be dropped (%s).", reason)
	}
}

func closeConnection(conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = conn.Close(ctx)
}
