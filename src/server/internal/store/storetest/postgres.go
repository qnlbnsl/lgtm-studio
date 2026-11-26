// Package storetest holds the fixtures of the store's tests against real PostgreSQL.
package storetest

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Stub of the test writer: the builder of work item 1 replaces this file.

// NewDatabase skips the test when LGTM_TEST_POSTGRES is absent.
func NewDatabase(t testing.TB, schema string) *pgxpool.Config {
	t.Helper()
	if os.Getenv("LGTM_TEST_POSTGRES") == "" {
		t.Skip("LGTM_TEST_POSTGRES is not set; run through deploy/test/with-postgres.sh")
	}
	return nil
}
