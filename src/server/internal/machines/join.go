package machines

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Refusal is the reason that a join is refused. Its text is the value of the metadata key
// `Lgtm-Join-Refusal`.
type Refusal string

// The five reasons of the link's contract.
const (
	RefusalUnknown        Refusal = "token_unknown"
	RefusalExpired        Refusal = "token_expired"
	RefusalRevoked        Refusal = "token_revoked"
	RefusalUsed           Refusal = "token_used"
	RefusalMachineRemoved Refusal = "machine_removed"
)

// The sentinels of the refusals. Each crosses the store unwrapped, and RefusalOf maps it back.
var (
	ErrTokenUnknown    = store.NewDomainError("The server does not know this join token.")
	ErrTokenExpired    = store.NewDomainError("This join token has expired. Make a new one.")
	ErrTokenRevoked    = store.NewDomainError("This join token was revoked. Make a new one.")
	ErrTokenUsed       = store.NewDomainError("This join token was already used. Make a new one.")
	ErrMachineRemoved  = store.NewDomainError("The machine that used this join token was removed. Make a new token.")
	ErrInvalidHostName = store.NewDomainError("The host name is not valid.")
)

// RefusalOf returns the reason of a refusal error, and false for every other error.
func RefusalOf(err error) (Refusal, bool) {
	switch err {
	case ErrTokenUnknown:
		return RefusalUnknown, true
	case ErrTokenExpired:
		return RefusalExpired, true
	case ErrTokenRevoked:
		return RefusalRevoked, true
	case ErrTokenUsed:
		return RefusalUsed, true
	case ErrMachineRemoved:
		return RefusalMachineRemoved, true
	default:
		return "", false
	}
}

// Redemption is what a machine sends to join.
type Redemption struct {
	TokenID   string
	Secret    []byte
	PublicKey []byte
	HostName  string
}

// Joined is the answer of a redemption.
type Joined struct {
	MachineID string
	// Replay is true when the same key had used the token before and nothing was written.
	Replay bool
}

// Redeem decides one use of a token in one write.
func (t *Tokens) Redeem(ctx context.Context, in Redemption) (Joined, error) {
	var joined Joined
	err := t.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		var verifier, usedKey []byte
		var expiresAt time.Time
		var revoked, hostsServer bool
		var usedBy string
		err := tx.QueryRow(ctx, `
SELECT secret_sha256, expires_at, revoked_at IS NOT NULL, used_key, COALESCE(used_by, ''), hosts_server
FROM client_join_tokens WHERE id = $1 FOR UPDATE`, in.TokenID).
			Scan(&verifier, &expiresAt, &revoked, &usedKey, &usedBy, &hostsServer)
		if err == pgx.ErrNoRows {
			return ErrTokenUnknown
		}
		if err != nil {
			return err
		}
		digest := tokenVerifier(in.Secret)
		matches := subtle.ConstantTimeCompare(verifier, digest[:])
		if matches != 1 {
			return ErrTokenUnknown
		}

		// Read the clock after acquiring the row lock so that time spent waiting counts.
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if !expiresAt.After(now) {
			return ErrTokenExpired
		}
		if revoked {
			return ErrTokenRevoked
		}
		if usedKey != nil {
			if subtle.ConstantTimeCompare(usedKey, in.PublicKey) != 1 {
				return ErrTokenUsed
			}
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM machines WHERE id = $1)`, usedBy).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrMachineRemoved
			}
			joined = Joined{MachineID: usedBy, Replay: true}
			return nil
		}

		if len(in.HostName) > 255 || !utf8.ValidString(in.HostName) || strings.ContainsRune(in.HostName, 0) {
			return ErrInvalidHostName
		}
		machine, err := t.registry.CreateWaiting(ctx, tx, NewMachine{
			PublicKey: in.PublicKey, HostName: in.HostName, TokenID: in.TokenID, HostsServer: hostsServer,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
UPDATE client_join_tokens SET used_at = clock_timestamp(), used_key = $2, used_by = $3
WHERE id = $1`, in.TokenID, in.PublicKey, machine.ID); err != nil {
			return err
		}
		if err := recordTokenChange(tx, in.TokenID); err != nil {
			return err
		}
		joined = Joined{MachineID: machine.ID}
		return nil
	})
	if err != nil {
		return Joined{}, err
	}
	return joined, nil
}
