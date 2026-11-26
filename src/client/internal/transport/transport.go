// Package transport decides where a machine may send its join token and its
// reports, and how it checks the server it reaches.
package transport

import (
	"errors"
	"net/http"
)

// ErrCertificateKeyMismatch reports that the server presented a certificate
// whose public key is not the pinned one.
//
// STUB of the test writer: the builder replaces this value and keeps its name.
var ErrCertificateKeyMismatch = errors.New("transport stub: certificate key mismatch")

// ParseOrigin returns the canonical form of an origin, or an error when the
// origin may not receive a token.
//
// STUB of the test writer: it accepts nothing and returns an empty string.
func ParseOrigin(raw string) (string, error) {
	return "", nil
}

// NewHTTPClient returns a client that talks only to the given origin and, for
// an encrypted origin, trusts only the leaf whose public key has the given
// SHA-256 digest.
//
// STUB of the test writer: it returns no client and no error.
func NewHTTPClient(origin string, certificateKeySHA256 []byte) (*http.Client, error) {
	return nil, nil
}
