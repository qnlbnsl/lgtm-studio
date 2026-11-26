package link

import (
	"embed"
	"io/fs"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

//go:embed migrations/0001_link.sql
var linkMigrationStub embed.FS

// Migrations is a stub of the test writer. It supplies a valid empty migration.
func Migrations() store.Migrations {
	files, _ := fs.Sub(linkMigrationStub, "migrations")
	return store.Migrations{Part: "link", Files: files}
}
