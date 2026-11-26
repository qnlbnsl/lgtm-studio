package machines

import (
	"context"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// This file is a stub of the test writer for item 4 (task s3-machines-tests-b).
// The builder replaces every body. The names and signatures are the test writer's choice,
// because the plan names only the behaviour. The builder may keep them.

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
func RefusalOf(err error) (Refusal, bool) { return "", false }

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
	return Joined{}, nil
}
