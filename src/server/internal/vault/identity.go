package vault

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"embed"
	"errors"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// LinkIdentity is the machine-link signing key that OpenBao holds and PostgreSQL pins.
type LinkIdentity struct {
	SigningKey   ed25519.PrivateKey
	Fingerprint  string
	VaultVersion int

	// adopted records that the first pin took a key this load did not create.
	adopted bool
}

const (
	pinTopic = "server_link_identity"
	pinKey   = "link"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations returns the vault part's schema, which holds only the public pin.
func Migrations() store.Migrations {
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic("The embedded vault migrations are missing.")
	}
	return store.Migrations{Part: "vault", Files: files}
}

// LoadLinkIdentity reads the public pin, then loads the signing key from OpenBao.
// An existing pin requires the same key in OpenBao, and the load never writes in that case.
// Without a pin and with allowInitialize, the load adopts a stored key or creates one,
// and returns it only after the pin has committed.
// The application calls it under its exclusive startup lock.
func LoadLinkIdentity(ctx context.Context, s *store.Store, c *Client, allowInitialize bool) (LinkIdentity, error) {
	pin, pinned, err := readPin(ctx, s)
	if err != nil {
		return LinkIdentity{}, ErrEstablish
	}
	if !pinned && !allowInitialize {
		return LinkIdentity{}, ErrNoPin
	}
	token, err := c.currentToken(ctx)
	if err != nil {
		return LinkIdentity{}, err
	}

	key, state := c.readKey(ctx, token)
	if pinned {
		switch state {
		case keyFound:
			identity := newIdentity(key)
			if identity.Fingerprint != pin {
				return LinkIdentity{}, ErrReplaced
			}
			return identity, nil
		case keyUnavailable:
			return LinkIdentity{}, ErrUnavailable
		default:
			return LinkIdentity{}, ErrReplaced
		}
	}

	adopted := true
	switch state {
	case keyFound:
	case keyAbsent:
		key, state, adopted = c.createAndReread(ctx, token)
		if state != keyFound {
			if state == keyUnavailable {
				return LinkIdentity{}, ErrUnavailable
			}
			return LinkIdentity{}, ErrEstablish
		}
	case keyUnavailable:
		return LinkIdentity{}, ErrUnavailable
	default:
		return LinkIdentity{}, ErrEstablish
	}

	identity := newIdentity(key)
	identity.adopted = adopted
	if err := writePin(ctx, s, identity.Fingerprint); err != nil {
		return LinkIdentity{}, err
	}
	return identity, nil
}

// createAndReread makes the single create attempt and then reads the entry once.
// Whatever the write's reply, only the stored seed is used, so a losing candidate is discarded.
// The flag reports that the stored seed is not this load's candidate, because a competing create won.
func (c *Client) createAndReread(ctx context.Context, token string) (storedKey, keyState, bool) {
	var candidate [32]byte
	defer clear(candidate[:])
	if _, err := rand.Read(candidate[:]); err != nil {
		return storedKey{}, keyInvalid, false
	}
	c.createKey(ctx, token, &candidate)
	if ctx.Err() != nil {
		return storedKey{}, keyUnavailable, false
	}
	key, state := c.readKey(ctx, token)
	return key, state, key.seed != candidate
}

func newIdentity(key storedKey) LinkIdentity {
	signing := ed25519.NewKeyFromSeed(key.seed[:])
	clear(key.seed[:])
	return LinkIdentity{
		SigningKey:   signing,
		Fingerprint:  SigningKeyFingerprint(signing.Public().(ed25519.PublicKey)),
		VaultVersion: key.version,
	}
}

// readPin returns the established public fingerprint, if any.
func readPin(ctx context.Context, s *store.Store) (string, bool, error) {
	var pin string
	err := s.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		return tx.QueryRow(ctx, "SELECT fingerprint FROM server_link_identity WHERE singleton").Scan(&pin)
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return pin, true, nil
}

// writePin records the first pin with its feed reference in one transaction.
// A pin that another writer committed first is accepted only if it matches.
func writePin(ctx context.Context, s *store.Store, fingerprint string) error {
	mismatch := false
	err := s.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		var inserted bool
		err := tx.QueryRow(ctx, `
INSERT INTO server_link_identity (singleton, fingerprint) VALUES (true, $1)
ON CONFLICT (singleton) DO NOTHING
RETURNING true`, fingerprint).Scan(&inserted)
		if errors.Is(err, pgx.ErrNoRows) {
			var existing string
			if err := tx.QueryRow(ctx, "SELECT fingerprint FROM server_link_identity WHERE singleton").Scan(&existing); err != nil {
				return err
			}
			if existing != fingerprint {
				mismatch = true
				return ErrReplaced
			}
			return nil
		}
		if err != nil {
			return err
		}
		return tx.Record(store.ChangeRef{Topic: pinTopic, Key: pinKey, Change: store.Saved})
	})
	switch {
	case mismatch:
		return ErrReplaced
	case err != nil:
		return ErrEstablish
	}
	return nil
}
