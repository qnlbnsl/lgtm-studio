// Package migrated provisions a database, opens the store and applies migrations.
package migrated

import (
	"testing"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// NewStore opens a new fixture database after applying its migrations.
func NewStore(t testing.TB, schema string, parts ...store.Migrations) *store.Store {
	t.Helper()
	config := storetest.NewDatabase(t, schema)
	ctx := t.Context()
	s, err := store.Open(ctx, config, store.ConnectionPolicy{})
	if err != nil {
		t.Fatalf("Opening the PostgreSQL fixture store failed. %v", err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx, parts...); err != nil {
		t.Fatalf("Applying the PostgreSQL fixture migrations failed. %v", err)
	}
	return s
}
