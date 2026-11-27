package projects

import (
	"embed"
	"io/fs"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Migrations provides the embedded Projects schema to the store.
func Migrations() store.Migrations {
	files, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(err)
	}
	return store.Migrations{Part: "projects", Files: files}
}
