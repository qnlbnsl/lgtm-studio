// Package enroll holds the key pair that proves a machine to the server.
package enroll

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
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

const (
	keyBlockType = "PRIVATE KEY"
	maxKeySize   = 4096
)

// Credential is the key of the machine.
type Credential struct {
	key *credentialKey
}

type credentialKey struct {
	public ed25519.PublicKey
	// The function keeps private key bytes out of reflective formatting.
	sign func(io.Reader, []byte, crypto.SignerOpts) ([]byte, error)
}

func newCredential(key ed25519.PrivateKey) *Credential {
	return &Credential{key: &credentialKey{
		public: key.Public().(ed25519.PublicKey),
		sign:   key.Sign,
	}}
}

// Create generates the machine key and places it without replacing any file.
func Create(path string) (*Credential, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, ErrKeyInvalid
	}
	data := pem.EncodeToMemory(&pem.Block{Type: keyBlockType, Bytes: der})
	if err := state.CreateExclusive(filepath.Dir(path), filepath.Base(path), data); err != nil {
		if !errors.Is(err, state.ErrPlacedNotSynced) && errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w: %w", ErrKeyExists, err)
		}
		return nil, err
	}
	return newCredential(key), nil
}

// Load reads a private key file owned by the current user.
func Load(path string) (*Credential, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return nil, ErrKeyNotRegular
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrKeyMissing
		}
		// Some special files, such as sockets, fail during open itself.
		if info, statErr := os.Lstat(path); statErr == nil && !info.Mode().IsRegular() {
			return nil, ErrKeyNotRegular
		}
		return nil, fmt.Errorf("%w: %w", ErrKeyUnreadable, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKeyUnreadable, err)
	}
	if !info.Mode().IsRegular() {
		return nil, ErrKeyNotRegular
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, ErrKeyExposed
	}
	if info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return nil, ErrKeyNotOwned
	}

	data, err := io.ReadAll(io.LimitReader(f, maxKeySize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKeyUnreadable, err)
	}
	if len(data) > maxKeySize {
		return nil, ErrKeyInvalid
	}
	return decodeCredential(data)
}

func decodeCredential(data []byte) (*Credential, error) {
	firstLine, body, ok := bytes.Cut(data, []byte("\n"))
	if !ok || string(bytes.TrimSuffix(firstLine, []byte("\r"))) != "-----BEGIN PRIVATE KEY-----" ||
		bytes.Contains(body, []byte("-----BEGIN ")) {
		return nil, ErrKeyInvalid
	}
	end, hasCRLF := bytes.CutSuffix(body, []byte("\r\n"))
	if !hasCRLF {
		end = bytes.TrimSuffix(body, []byte("\n"))
	}
	if !bytes.HasSuffix(end, []byte("\n-----END PRIVATE KEY-----")) {
		return nil, ErrKeyInvalid
	}
	block, rest := pem.Decode(data)
	if block == nil || len(rest) != 0 || block.Type != keyBlockType {
		return nil, ErrKeyInvalid
	}
	// The key parser accepts trailing bytes after the encoded value.
	var value asn1.RawValue
	if rest, err := asn1.Unmarshal(block.Bytes, &value); err != nil || len(rest) != 0 {
		return nil, ErrKeyInvalid
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok {
		return nil, ErrKeyInvalid
	}
	return newCredential(key), nil
}

// PublicKey returns a copy of the public half of the machine key.
func (c Credential) PublicKey() ed25519.PublicKey {
	if c.key == nil {
		return nil
	}
	return bytes.Clone(c.key.public)
}

// Sign signs the message with the machine key.
func (c Credential) Sign(message []byte) []byte {
	if c.key == nil {
		return nil
	}
	sig, _ := c.key.sign(rand.Reader, message, crypto.Hash(0))
	return sig
}

// Signer returns a signer that exposes only the public key and signing.
func (c Credential) Signer() crypto.Signer {
	return credentialSigner{credential: c}
}

type credentialSigner struct {
	credential Credential
}

func (s credentialSigner) Public() crypto.PublicKey {
	return s.credential.PublicKey()
}

func (s credentialSigner) Sign(random io.Reader, message []byte, opts crypto.SignerOpts) ([]byte, error) {
	if s.credential.key == nil {
		return nil, ErrKeyInvalid
	}
	return s.credential.key.sign(random, message, opts)
}

// Fingerprint returns the public key hash in unpadded base64.
func (c Credential) Fingerprint() string {
	sum := sha256.Sum256(c.PublicKey())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// String identifies the machine key by its public fingerprint.
func (c Credential) String() string { return "machine key " + c.Fingerprint() }

// GoString identifies the machine key in Go-syntax formatting.
func (c Credential) GoString() string { return c.String() }

// Format writes the public fingerprint for every formatting verb it receives.
func (c Credential) Format(f fmt.State, verb rune) {
	io.WriteString(f, c.String())
}
