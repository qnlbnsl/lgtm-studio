package machines

// This file is a STUB of the test writer. The builder replaces every body.

import (
	"context"
	"crypto/ed25519"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Errors of the address of a token.
var (
	ErrInvalidAddress   = store.NewDomainError("The server address is not a loopback address.")
	ErrNoCertificatePin = store.NewDomainError("This server has no certificate to pin for an https address.")
)

// Issuer is what the server knows about itself.
type Issuer struct {
	PublicURL string
	ServerKey ed25519.PublicKey
}

// TokenOptions are the choices of the owner.
type TokenOptions struct{ HostsServer bool }

// NewToken is the answer of Create.
type NewToken struct {
	ID, Text, Address string
	ExpiresAt         time.Time
}

// Tokens makes and revokes join tokens.
type Tokens struct{}

// NewTokens returns the tokens of one server.
func NewTokens(s *store.Store, r *Registry, issuer Issuer) *Tokens { return &Tokens{} }

// Create makes one token that works once and for 15 minutes.
func (t *Tokens) Create(ctx context.Context, o TokenOptions) (NewToken, error) {
	return NewToken{}, nil
}

// Revoke marks the token revoked and returns the machine that used it, or an empty string.
func (t *Tokens) Revoke(ctx context.Context, tx *store.Tx, id string) (string, error) {
	return "", nil
}

// NormalizeAddress checks and normalizes the address that a token carries.
func NormalizeAddress(address string) (string, error) { return "", nil }

// EncodeToken returns the text of a token.
func EncodeToken(token *linkv1.JoinToken) (string, error) { return "", nil }

// DecodeToken parses the text of a token.
func DecodeToken(text string) (*linkv1.JoinToken, error) { return nil, nil }
