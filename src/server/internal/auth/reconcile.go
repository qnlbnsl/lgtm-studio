package auth

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Migrations provides the embedded session schema to the store.
func Migrations() store.Migrations {
	files, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(err)
	}
	return store.Migrations{Part: "auth", Files: files}
}

const sessionFutureAllowance = time.Minute

type sessionRecord struct {
	verifier  Verifier
	createdAt time.Time
	salt      [16]byte
	binding   [32]byte
}

func (row sessionRecord) liveAt(sample time.Time) bool {
	return row.createdAt.After(sample.Add(-SessionMaxAgeSeconds*time.Second)) &&
		!row.createdAt.After(sample.Add(sessionFutureAllowance))
}

// Reconcile removes stale sessions and purges all sessions when the password changes or is removed.
func Reconcile(ctx context.Context, s *store.Store, password *Password) error {
	return s.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		if err := lockSessions(ctx, tx); err != nil {
			return err
		}
		var sample time.Time
		if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&sample); err != nil {
			return err
		}
		rows, err := listSessions(ctx, tx)
		if err != nil {
			return err
		}
		purge := password == nil
		if !purge {
			for _, row := range rows {
				if err := ctx.Err(); err != nil {
					return err
				}
				if !row.liveAt(sample) {
					continue
				}
				want, err := bindPassword(*password, row.salt, row.verifier, row.createdAt)
				if err != nil {
					return err
				}
				if !equalDigest(want, row.binding) {
					purge = true
					break
				}
			}
		}
		for _, row := range rows {
			if purge || !row.liveAt(sample) {
				if err := removeSession(ctx, tx, row.verifier); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func lockSessions(ctx context.Context, tx *store.Tx) error {
	_, err := tx.Exec(ctx, "LOCK TABLE web_sessions IN SHARE ROW EXCLUSIVE MODE")
	return err
}

func listSessions(ctx context.Context, tx *store.Tx) ([]sessionRecord, error) {
	rows, err := tx.Query(ctx, `
SELECT verifier, created_at, password_salt, password_binding
FROM web_sessions ORDER BY created_at, verifier`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (sessionRecord, error) {
		var verifier, salt, binding []byte
		var createdAt time.Time
		if err := row.Scan(&verifier, &createdAt, &salt, &binding); err != nil {
			return sessionRecord{}, err
		}
		return sessionRecord{Verifier(verifier), createdAt, [16]byte(salt), [32]byte(binding)}, nil
	})
}

func removeSession(ctx context.Context, tx *store.Tx, verifier Verifier) error {
	var deleted []byte
	if err := tx.QueryRow(ctx, "DELETE FROM web_sessions WHERE verifier = $1 RETURNING verifier", verifier[:]).Scan(&deleted); err != nil {
		return err
	}
	return recordSessionChange(tx, Verifier(deleted), store.Removed)
}

func recordSessionChange(tx *store.Tx, verifier Verifier, change store.Change) error {
	key := sha256.Sum256(append([]byte("lgtm-web-session-feed-v1\x00"), verifier[:]...))
	return tx.Record(store.ChangeRef{Topic: "web_sessions", Key: hex.EncodeToString(key[:]), Change: change})
}
