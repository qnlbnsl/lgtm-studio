package auth

import (
	"context"
	"embed"
	"io/fs"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Migrations is a stub of the test writer. The builder of item 3 owns this file.
func Migrations() store.Migrations {
	files, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(err)
	}
	return store.Migrations{Part: "auth", Files: files}
}

// Reconcile is a stub of the test writer. It does nothing.
func Reconcile(ctx context.Context, s *store.Store, password *Password) error {
	return nil
}
