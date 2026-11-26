// Package migrated provisions a database, opens the store and applies migrations.
package migrated

import (
	"testing"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Stub of the test writer: the builder of work item 3 replaces this file.

func NewStore(t testing.TB, schema string, parts ...store.Migrations) *store.Store {
	t.Helper()
	return nil
}
