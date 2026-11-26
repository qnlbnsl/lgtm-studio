package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Verifier is the SHA-256 digest of the binary session token.
type Verifier [32]byte

// Sessions issues, checks and revokes web sessions.
type Sessions struct {
	store      *store.Store
	password   Password
	issueSlot  chan struct{}
	verifySlot chan struct{}
	deriveKey  func(Password, [16]byte) ([32]byte, error)

	cacheMu sync.Mutex
	cache   []sessionRecord // The least recently used tuple comes first.
}

// SessionLimit is the number of sessions kept after a sign-in.
const SessionLimit = 8

// SessionMaxAgeSeconds is the lifetime of a session in seconds.
const SessionMaxAgeSeconds = 30 * 24 * 60 * 60

// ErrSessionClock indicates that the next creation time would be too far ahead of the database clock.
var ErrSessionClock = store.NewDomainError("The server could not issue a session, retry sign-in.")

// ErrNoSession indicates that the session is absent or its binding is invalid.
var ErrNoSession = store.NewDomainError("Sign in to continue.")

// NewSessions binds session operations to the store and the configured password.
func NewSessions(s *store.Store, password Password) *Sessions {
	return &Sessions{
		store:      s,
		password:   password,
		issueSlot:  make(chan struct{}, 1),
		verifySlot: make(chan struct{}, 1),
		deriveKey:  deriveBindingKey,
	}
}

// TokenVerifier accepts only the canonical unpadded URL-safe encoding of a 32-byte token.
func TokenVerifier(token string) (Verifier, bool) {
	var raw [32]byte
	if len(token) != base64.RawURLEncoding.EncodedLen(len(raw)) {
		return Verifier{}, false
	}
	n, err := base64.RawURLEncoding.Strict().Decode(raw[:], []byte(token))
	if err != nil || n != len(raw) {
		return Verifier{}, false
	}
	return sha256.Sum256(raw[:]), true
}

// Issue creates a session and returns its token after the store confirms the commit.
func (s *Sessions) Issue(ctx context.Context) (string, error) {
	var raw [32]byte
	var row sessionRecord
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	if _, err := rand.Read(row.salt[:]); err != nil {
		return "", err
	}
	row.verifier = sha256.Sum256(raw[:])
	key, err := s.issueKey(ctx, row.salt)
	if err != nil {
		return "", err
	}
	defer clear(key[:])

	err = s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
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
		live := rows[:0]
		for _, existing := range rows {
			if existing.liveAt(sample) {
				live = append(live, existing)
			} else if err := removeSession(ctx, tx, existing.verifier); err != nil {
				return err
			}
		}

		row.createdAt = sample
		if len(live) > 0 {
			next := live[len(live)-1].createdAt.Add(time.Microsecond)
			if next.After(sample.Add(sessionFutureAllowance)) {
				return ErrSessionClock
			}
			if next.After(sample) {
				row.createdAt = next
			}
		}
		row.binding = bindingTag(key, row.verifier, row.createdAt)
		if _, err := tx.Exec(ctx, `
INSERT INTO web_sessions (verifier, created_at, password_salt, password_binding)
VALUES ($1, $2, $3, $4)`, row.verifier[:], row.createdAt, row.salt[:], row.binding[:]); err != nil {
			return err
		}
		if err := recordSessionChange(tx, row.verifier, store.Saved); err != nil {
			return err
		}
		// The new row is strictly newer than every existing row, which is already ordered oldest first.
		for _, existing := range live[:max(0, len(live)+1-SessionLimit)] {
			if err := removeSession(ctx, tx, existing.verifier); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func (s *Sessions) issueKey(ctx context.Context, salt [16]byte) ([32]byte, error) {
	if err := acquireDerivation(ctx, s.issueSlot); err != nil {
		return [32]byte{}, err
	}
	defer func() { <-s.issueSlot }()
	return s.deriveKey(s.password, salt)
}

// Check verifies a current live session using the primary store.
func (s *Sessions) Check(ctx context.Context, verifier Verifier) error {
	_, err := s.readVerifiedSession(ctx, verifier)
	return err
}

// CheckInRead verifies a session in the caller's read snapshot.
func (s *Sessions) CheckInRead(ctx context.Context, tx *store.ReadTx, verifier Verifier) error {
	row, err := s.lookupLiveSession(ctx, tx, verifier)
	if err != nil {
		return err
	}
	return s.verifyBinding(ctx, row)
}

// Revoke deletes the verified session when the same live tuple still exists under the write lock.
func (s *Sessions) Revoke(ctx context.Context, verifier Verifier) error {
	verified, err := s.readVerifiedSession(ctx, verifier)
	if err != nil {
		return err
	}

	err = s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		if err := lockSessions(ctx, tx); err != nil {
			return err
		}
		current, err := s.lookupLiveSession(ctx, tx, verifier)
		if err != nil {
			return err
		}
		if !sameSession(current, verified) {
			return ErrNoSession
		}
		return removeSession(ctx, tx, verifier)
	})
	if err == nil || err == ErrNoSession {
		s.dropCachedSession(verifier)
	}
	return err
}

func (s *Sessions) readVerifiedSession(ctx context.Context, verifier Verifier) (sessionRecord, error) {
	var row sessionRecord
	err := s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		var err error
		row, err = s.lookupLiveSession(ctx, tx, verifier)
		return err
	})
	if err != nil {
		return sessionRecord{}, err
	}
	return row, s.verifyBinding(ctx, row)
}

type sessionQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Sessions) lookupLiveSession(ctx context.Context, tx sessionQuerier, verifier Verifier) (sessionRecord, error) {
	var createdAt, sample time.Time
	var salt, binding []byte
	err := tx.QueryRow(ctx, `
SELECT created_at, password_salt, password_binding, clock_timestamp()
FROM web_sessions WHERE verifier = $1`, verifier[:]).Scan(&createdAt, &salt, &binding, &sample)
	if err == pgx.ErrNoRows {
		s.dropCachedSession(verifier)
		return sessionRecord{}, ErrNoSession
	}
	if err != nil {
		return sessionRecord{}, err
	}
	row := sessionRecord{verifier: verifier, createdAt: createdAt, salt: [16]byte(salt), binding: [32]byte(binding)}
	if !row.liveAt(sample) {
		s.dropCachedSession(verifier)
		return sessionRecord{}, ErrNoSession
	}
	return row, nil
}

func (s *Sessions) verifyBinding(ctx context.Context, row sessionRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.useCachedSession(row) {
		return nil
	}
	if err := acquireDerivation(ctx, s.verifySlot); err != nil {
		return err
	}
	defer func() { <-s.verifySlot }()
	if s.useCachedSession(row) {
		return nil
	}

	key, err := s.deriveKey(s.password, row.salt)
	if err != nil {
		return err
	}
	defer clear(key[:])
	if err := ctx.Err(); err != nil {
		return err
	}
	if !equalDigest(bindingTag(key, row.verifier, row.createdAt), row.binding) {
		return ErrNoSession
	}
	s.cacheSession(row)
	return nil
}

func acquireDerivation(ctx context.Context, slot chan struct{}) error {
	select {
	case slot <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-slot
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func sameSession(a, b sessionRecord) bool {
	return a.verifier == b.verifier && a.createdAt.Equal(b.createdAt) &&
		a.salt == b.salt && equalDigest(a.binding, b.binding)
}

func (s *Sessions) cachedSessionIndex(verifier Verifier) int {
	return slices.IndexFunc(s.cache, func(row sessionRecord) bool { return row.verifier == verifier })
}

func (s *Sessions) useCachedSession(row sessionRecord) bool {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	i := s.cachedSessionIndex(row.verifier)
	if i < 0 {
		return false
	}
	if !sameSession(s.cache[i], row) {
		s.cache = slices.Delete(s.cache, i, i+1)
		return false
	}
	copy(s.cache[i:], s.cache[i+1:])
	s.cache[len(s.cache)-1] = row
	return true
}

func (s *Sessions) cacheSession(row sessionRecord) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if len(s.cache) == SessionLimit {
		s.cache = slices.Delete(s.cache, 0, 1)
	}
	s.cache = append(s.cache, row)
}

func (s *Sessions) dropCachedSession(verifier Verifier) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if i := s.cachedSessionIndex(verifier); i >= 0 {
		s.cache = slices.Delete(s.cache, i, i+1)
	}
}

// cachedVerifiers returns a test snapshot of the cache, least recently used first.
func (s *Sessions) cachedVerifiers() []Verifier {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	verifiers := make([]Verifier, len(s.cache))
	for i, row := range s.cache {
		verifiers[i] = row.verifier
	}
	return verifiers
}
