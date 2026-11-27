package projects

import (
	"embed"
	"io/fs"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

//go:embed migrations/0001_projects.sql
var projectMigrationStub embed.FS

// Migrations is a stub of the test writer that supplies an empty Projects migration.
func Migrations() store.Migrations {
	files, err := fs.Sub(projectMigrationStub, "migrations")
	if err != nil {
		panic(err)
	}
	return store.Migrations{Part: "projects", Files: files}
}
