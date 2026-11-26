package machines

import (
	"embed"
	"io/fs"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// The migration files are copied from the plan by the test writer, so that the tests can reach the tables.
// The builder owns them from here on.
//
//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Migrations provides the embedded machine schema to the store.
func Migrations() store.Migrations {
	files, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(err)
	}
	return store.Migrations{Part: "machines", Files: files}
}
