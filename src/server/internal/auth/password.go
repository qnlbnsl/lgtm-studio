package auth

import "time"

// This file holds stubs of the test writer. The builder of item 2 replaces them.

// MaxPasswordBytes is the longest password or candidate the server accepts.
const MaxPasswordBytes = 4096

// Password is a stub of the test writer.
type Password struct{}

// NewPassword is a stub of the test writer.
func NewPassword(value string) (Password, error) { return Password{}, nil }

// Matches is a stub of the test writer.
func (p Password) Matches(candidate string) bool { return false }

// equalDigest is a stub of the test writer.
func equalDigest(a, b [32]byte) bool { return false }

// deriveBindingKey is a stub of the test writer.
func deriveBindingKey(p Password, salt [16]byte) ([32]byte, error) { return [32]byte{}, nil }

// bindingTag is a stub of the test writer.
func bindingTag(key [32]byte, verifier [32]byte, createdAt time.Time) [32]byte { return [32]byte{} }

// bindPassword is a stub of the test writer.
func bindPassword(p Password, salt [16]byte, verifier [32]byte, createdAt time.Time) ([32]byte, error) {
	return [32]byte{}, nil
}
