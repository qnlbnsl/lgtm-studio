package vault

import (
	"context"
	"crypto/ed25519"
	"embed"
	"io/fs"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// LinkIdentity is a stub of the test writer.
type LinkIdentity struct {
	SigningKey   ed25519.PrivateKey
	Fingerprint  string
	VaultVersion int
}

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations exposes the no-op migration stub of the test writer.
func Migrations() store.Migrations {
	files, _ := fs.Sub(migrations, "migrations")
	return store.Migrations{Part: "vault", Files: files}
}

// LoadLinkIdentity is a stub of the test writer.
func LoadLinkIdentity(ctx context.Context, s *store.Store, c *Client, allowInitialize bool) (LinkIdentity, error) {
	return LinkIdentity{}, nil
}
