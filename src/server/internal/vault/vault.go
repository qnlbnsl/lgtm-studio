package vault

import "crypto/ed25519"

// Client is a stub of the test writer.
type Client struct{}

// Open is a stub of the test writer.
func Open(addr string, credentialFile string, allowPrivateStack bool) (*Client, error) {
	return &Client{}, nil
}

// SigningKeyFingerprint is a stub of the test writer.
func SigningKeyFingerprint(pub ed25519.PublicKey) string {
	return ""
}
