package store

import (
	"context"
	"embed"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	storePart        = "store"
	migrationTimeout = 5 * time.Minute
)

//go:embed migrations/*.sql
var storeMigrations embed.FS

var (
	migrationPartName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	migrationFileName = regexp.MustCompile(`^[0-9]{4}_[a-z][a-z0-9_]*\.sql$`)
	errManifest       = NewDomainError("The migration manifest is invalid.")
	errLedger         = NewDomainError("The migration manifest does not cover the recorded schema versions.")
)

// Migrations lists a part's numbered SQL files at the root of Files.
type Migrations struct {
	Part  string
	Files fs.FS
}

// Migrate applies all outstanding part migrations in a single startup transaction.
func (s *Store) Migrate(ctx context.Context, parts ...Migrations) error {
	ctx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()
	manifest, err := loadMigrations(parts)
	if err != nil {
		return err
	}
	return withTransaction(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite},
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '5min'; SET LOCAL lock_timeout = 0"); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::bigint)", migrationLockID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    part text PRIMARY KEY,
    version integer NOT NULL CHECK (version > 0)
)`); err != nil {
				return err
			}
			return applyMigrations(ctx, tx, manifest)
		})
}

func loadMigrations(parts []Migrations) (map[string][]string, error) {
	builtin, err := fs.Sub(storeMigrations, "migrations")
	if err != nil {
		return nil, errManifest
	}
	manifest := make(map[string][]string, len(parts)+1)
	for _, part := range append([]Migrations{{Part: storePart, Files: builtin}}, parts...) {
		if !migrationPartName.MatchString(part.Part) || part.Files == nil {
			return nil, errManifest
		}
		if _, exists := manifest[part.Part]; exists {
			return nil, errManifest
		}
		entries, err := fs.ReadDir(part.Files, ".")
		if err != nil || len(entries) == 0 {
			return nil, errManifest
		}
		files := make([]string, len(entries))
		for i, file := range entries {
			name := file.Name()
			if !file.Type().IsRegular() || !migrationFileName.MatchString(name) {
				return nil, errManifest
			}
			version, err := strconv.Atoi(name[:4])
			if err != nil || version != i+1 {
				return nil, errManifest
			}
			content, err := fs.ReadFile(part.Files, name)
			if err != nil {
				return nil, errManifest
			}
			files[i] = string(content)
		}
		manifest[part.Part] = files
	}
	return manifest, nil
}

func applyMigrations(ctx context.Context, tx pgx.Tx, manifest map[string][]string) error {
	rows, err := tx.Query(ctx, "SELECT part, version FROM schema_migrations")
	if err != nil {
		return err
	}
	defer rows.Close()

	versions := make(map[string]int, len(manifest))
	for rows.Next() {
		var part string
		var version int
		if err := rows.Scan(&part, &version); err != nil {
			return err
		}
		if version < 1 || version > len(manifest[part]) {
			return errLedger
		}
		versions[part] = version
	}
	if err := rows.Err(); err != nil {
		return err
	}

	order := make([]string, 0, len(manifest))
	order = append(order, storePart)
	for part := range manifest {
		if part != storePart {
			order = append(order, part)
		}
	}
	slices.Sort(order[1:])

	for _, part := range order {
		files := manifest[part]
		for i := versions[part]; i < len(files); i++ {
			if _, err := tx.Exec(ctx, files[i], pgx.QueryExecModeSimpleProtocol); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
INSERT INTO schema_migrations (part, version) VALUES ($1, $2)
ON CONFLICT (part) DO UPDATE SET version = EXCLUDED.version`, part, i+1); err != nil {
				return err
			}
		}
	}
	return nil
}
