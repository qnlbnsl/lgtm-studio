// Package enroll holds the key pair that proves a machine to the server.
package enroll

import (
	"crypto"
	"crypto/ed25519"
	"errors"
	"fmt"
)

var (
	ErrKeyExists     = errors.New("enroll: key file exists")
	ErrKeyMissing    = errors.New("enroll: key file is missing")
	ErrKeyExposed    = errors.New("enroll: key file can be read by others")
	ErrKeyNotOwned   = errors.New("enroll: key file belongs to another user")
	ErrKeyNotRegular = errors.New("enroll: key file is not a regular file")
	ErrKeyInvalid    = errors.New("enroll: key file content is invalid")
	ErrKeyUnreadable = errors.New("enroll: key file cannot be read")
)

var errStub = errors.New("stub of the test writer")

// Credential is the key of the machine. This type is a stub of the test writer.
type Credential struct {
	key *ed25519.PrivateKey
}

// Create is a stub of the test writer.
func Create(path string) (*Credential, error) { return nil, errStub }

// Load is a stub of the test writer.
func Load(path string) (*Credential, error) { return nil, errStub }

// PublicKey is a stub of the test writer.
func (c Credential) PublicKey() ed25519.PublicKey { return nil }

// Sign is a stub of the test writer.
func (c Credential) Sign(message []byte) []byte { return nil }

// Signer is a stub of the test writer.
func (c Credential) Signer() crypto.Signer { return nil }

// Fingerprint is a stub of the test writer.
func (c Credential) Fingerprint() string { return "" }

// String is a stub of the test writer.
func (c Credential) String() string { return "" }

// GoString is a stub of the test writer.
func (c Credential) GoString() string { return "" }

// Format is a stub of the test writer.
func (c Credential) Format(f fmt.State, verb rune) {}
