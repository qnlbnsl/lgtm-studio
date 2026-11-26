package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/scrypt"
)

// MaxPasswordBytes is the longest password or candidate the server accepts.
const MaxPasswordBytes = 4096

// Password holds the original bytes for session binding and a digest for sign-in.
type Password struct {
	value  string
	digest [32]byte
}

// NewPassword validates the configured password and preserves its exact bytes.
func NewPassword(value string) (Password, error) {
	if len(value) == 0 || len(value) > MaxPasswordBytes || !utf8.ValidString(value) {
		return Password{}, errors.New("the web password must contain 1 to 4096 valid UTF-8 bytes")
	}
	return Password{value: value, digest: sha256.Sum256([]byte(value))}, nil
}

// Matches compares the candidate's digest with the configured password's digest.
func (p Password) Matches(candidate string) bool {
	if len(candidate) == 0 || len(candidate) > MaxPasswordBytes {
		return false
	}
	return equalDigest(sha256.Sum256([]byte(candidate)), p.digest)
}

func equalDigest(a, b [32]byte) bool {
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

func deriveBindingKey(p Password, salt [16]byte) ([32]byte, error) {
	password := []byte(p.value)
	defer clear(password)
	key, err := scrypt.Key(password, salt[:], 65536, 8, 1, 32)
	if err != nil {
		return [32]byte{}, err
	}
	defer clear(key)
	return [32]byte(key), nil
}

func bindingTag(key [32]byte, verifier [32]byte, createdAt time.Time) [32]byte {
	defer clear(key[:])
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte("lgtm-web-session-v1\x00"))
	mac.Write(verifier[:])
	var micros [8]byte
	binary.BigEndian.PutUint64(micros[:], uint64(createdAt.UnixMicro()))
	mac.Write(micros[:])
	return [32]byte(mac.Sum(nil))
}

func bindPassword(p Password, salt [16]byte, verifier [32]byte, createdAt time.Time) ([32]byte, error) {
	key, err := deriveBindingKey(p, salt)
	if err != nil {
		return [32]byte{}, err
	}
	defer clear(key[:])
	return bindingTag(key, verifier, createdAt), nil
}
