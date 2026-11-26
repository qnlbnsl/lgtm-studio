package auth

import (
	"context"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Everything in this file is a stub of the test writer. The builder of item 4 owns this file.

// Verifier is the SHA-256 digest of the binary session token.
type Verifier [32]byte

// Sessions issues, checks and revokes web sessions.
type Sessions struct {
	store    *store.Store
	password Password
	// deriveKey defaults to deriveBindingKey. Same-package tests replace it before they start workers.
	deriveKey func(Password, [16]byte) ([32]byte, error)
}

const SessionLimit = 8
const SessionMaxAgeSeconds = 30 * 24 * 60 * 60

var ErrSessionClock = store.NewDomainError("The server could not issue a session, retry sign-in.")
var ErrNoSession = store.NewDomainError("Sign in to continue.")

func NewSessions(s *store.Store, password Password) *Sessions {
	return &Sessions{store: s, password: password, deriveKey: deriveBindingKey}
}

func TokenVerifier(token string) (Verifier, bool) { return Verifier{}, false }

func (s *Sessions) Issue(ctx context.Context) (string, error) { return "", nil }

func (s *Sessions) Check(ctx context.Context, verifier Verifier) error { return nil }

func (s *Sessions) CheckInRead(ctx context.Context, tx *store.ReadTx, verifier Verifier) error {
	return nil
}

func (s *Sessions) Revoke(ctx context.Context, verifier Verifier) error { return nil }

// cachedVerifiers is a test hook that the tests of item 4 call. It returns the verifiers of the
// entries of the verification cache, least recently used first. It takes the cache's mutex.
func (s *Sessions) cachedVerifiers() []Verifier { return nil }
