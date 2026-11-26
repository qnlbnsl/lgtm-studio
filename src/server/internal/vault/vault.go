// Package vault reads the server's AppRole credential and loads the link signing key from OpenBao.
package vault

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sync"
	"syscall"
	"time"
)

const (
	loginPath = "/v1/auth/approle/login"
	keyPath   = "/v1/lgtm/data/server/link/signing-key"

	// stackHost and stackPort name the one private service that may use plaintext after opt-in.
	stackHost = "openbao"
	stackPort = "8200"

	requestTimeout     = 5 * time.Second
	maxResponseBytes   = 64 << 10
	maxCredentialBytes = 4 << 10
	refreshMargin      = time.Minute
	maxLeaseSeconds    = 1 << 31
)

// The errors carry fixed sentences, so no credential, token or response value can reach a log.
var (
	// ErrCredential reports a missing, malformed or unprotected credential file.
	ErrCredential = errors.New("The LGTM_OPENBAO_CREDENTIAL_FILE credential is missing or invalid.")
	// ErrAddress reports an OpenBao address that is malformed or not permitted.
	ErrAddress = errors.New("The LGTM_OPENBAO_ADDR setting is invalid.")
	// ErrUnavailable reports an unreachable or sealed vault, a refused login or a refused read.
	ErrUnavailable = errors.New("The server could not authenticate to OpenBao.")
	// ErrEstablish reports that the first signing key or its public pin could not be established.
	ErrEstablish = errors.New("The server could not establish its OpenBao signing key.")
	// ErrReplaced reports an established signing key that is absent from OpenBao or differs from its pin.
	ErrReplaced = errors.New("Restore the established OpenBao signing key before restarting the server.")
	// ErrNoPin reports a load without permission to initialize on an installation that has no pin.
	ErrNoPin = errors.New("The server has not established its signing key yet.")
)

// Client holds the AppRole credential, one bounded HTTP client and the cached batch token.
type Client struct {
	base     string
	roleID   string
	secretID string
	http     *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

// Open validates the address and reads the protected credential file.
// It does not contact OpenBao.
func Open(addr string, credentialFile string, allowPrivateStack bool) (*Client, error) {
	base, err := validAddress(addr, allowPrivateStack)
	if err != nil {
		return nil, err
	}
	roleID, secretID, err := readCredential(credentialFile)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            (&net.Dialer{Timeout: requestTimeout, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    requestTimeout,
		ResponseHeaderTimeout:  requestTimeout,
		MaxResponseHeaderBytes: maxResponseBytes,
		MaxIdleConns:           2,
		IdleConnTimeout:        30 * time.Second,
		ForceAttemptHTTP2:      true,
	}
	return &Client{
		base:     base,
		roleID:   roleID,
		secretID: secretID,
		http: &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// SigningKeyFingerprint returns lowercase hexadecimal SHA-256 over the raw public key.
func SigningKeyFingerprint(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])
}

// validAddress accepts HTTPS with verified TLS, plaintext to a literal loopback address,
// and plaintext to the private stack service only after opt-in.
func validAddress(addr string, allowPrivateStack bool) (string, error) {
	u, err := url.Parse(addr)
	if err != nil || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery ||
		u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Hostname() == "" {
		return "", ErrAddress
	}
	switch u.Scheme {
	case "https":
	case "http":
		literal, err := netip.ParseAddr(u.Hostname())
		loopback := err == nil && literal.IsLoopback() && literal.Zone() == ""
		stack := allowPrivateStack && u.Hostname() == stackHost && u.Port() == stackPort
		if !loopback && !stack {
			return "", ErrAddress
		}
	default:
		return "", ErrAddress
	}
	return u.Scheme + "://" + u.Host, nil
}

// readCredential reads the AppRole document from a regular file owned by this process
// that only its owner can read or write.
func readCredential(path string) (string, string, error) {
	if path == "" {
		return "", "", ErrCredential
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", "", ErrCredential
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", "", ErrCredential
	}
	if mode := info.Mode(); mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 ||
		(mode.Perm() != 0o400 && mode.Perm() != 0o600) {
		return "", "", ErrCredential
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return "", "", ErrCredential
	}
	data, err := io.ReadAll(io.LimitReader(f, maxCredentialBytes+1))
	defer clear(data)
	if err != nil || len(data) > maxCredentialBytes {
		return "", "", ErrCredential
	}
	return parseCredential(data)
}

// parseCredential requires exactly one nonempty string each for role_id and secret_id.
// It walks the tokens so that duplicate, differently cased or extra fields are refused.
func parseCredential(data []byte) (string, string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return "", "", ErrCredential
	}
	fields := make(map[string]string, 2)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || (name != "role_id" && name != "secret_id") {
			return "", "", ErrCredential
		}
		if _, seen := fields[name]; seen {
			return "", "", ErrCredential
		}
		token, err = decoder.Token()
		value, ok := token.(string)
		if err != nil || !ok || value == "" {
			return "", "", ErrCredential
		}
		fields[name] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return "", "", ErrCredential
	}
	if _, err := decoder.Token(); err != io.EOF {
		return "", "", ErrCredential
	}
	if len(fields) != 2 {
		return "", "", ErrCredential
	}
	return fields["role_id"], fields["secret_id"], nil
}

// currentToken returns the cached token while more than one minute of its lease remains.
// Otherwise it logs in again. The lock serializes refresh, so concurrent loads share one login.
func (c *Client) currentToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.expires) > refreshMargin {
		return c.token, nil
	}
	c.token, c.expires = "", time.Time{}
	token, expires, err := c.login(ctx)
	if err != nil {
		return "", err
	}
	c.token, c.expires = token, expires
	return token, nil
}

// login exchanges the AppRole credential for a token.
// The lease counts from just before the request, so slow replies shorten it conservatively.
func (c *Client) login(ctx context.Context) (string, time.Time, error) {
	started := time.Now()
	body, err := json.Marshal(struct {
		RoleID   string `json:"role_id"`
		SecretID string `json:"secret_id"`
	}{c.roleID, c.secretID})
	if err != nil {
		return "", time.Time{}, ErrUnavailable
	}
	defer clear(body)
	status, payload, err := c.do(ctx, http.MethodPost, loginPath, "", body)
	defer clear(payload)
	if err != nil || status != http.StatusOK {
		return "", time.Time{}, ErrUnavailable
	}
	var reply struct {
		Auth *struct {
			ClientToken   *string `json:"client_token"`
			LeaseDuration *int64  `json:"lease_duration"`
		} `json:"auth"`
	}
	if json.Unmarshal(payload, &reply) != nil || reply.Auth == nil ||
		reply.Auth.ClientToken == nil || !validToken(*reply.Auth.ClientToken) ||
		reply.Auth.LeaseDuration == nil || *reply.Auth.LeaseDuration <= 0 {
		return "", time.Time{}, ErrUnavailable
	}
	lease := min(*reply.Auth.LeaseDuration, maxLeaseSeconds)
	return *reply.Auth.ClientToken, started.Add(time.Duration(lease) * time.Second), nil
}

// validToken accepts a nonempty token of visible ASCII characters, which is safe in a header.
func validToken(token string) bool {
	if token == "" {
		return false
	}
	for i := 0; i < len(token); i++ {
		if token[i] < 0x21 || token[i] > 0x7e {
			return false
		}
	}
	return true
}

// keyState classifies one read of the signing-key entry.
type keyState int

const (
	keyFound keyState = iota
	// keyAbsent is a 404 that carries no version metadata, so the entry never existed.
	keyAbsent
	// keyInvalid is a malformed success or a deleted or destroyed entry.
	keyInvalid
	// keyUnavailable is a transport failure, a refusal, a sealed vault or any other status.
	keyUnavailable
)

// storedKey is the canonical seed and the positive version that OpenBao returned.
type storedKey struct {
	seed    [32]byte
	version int
}

// readKey reads the signing-key entry once.
func (c *Client) readKey(ctx context.Context, token string) (storedKey, keyState) {
	status, payload, err := c.do(ctx, http.MethodGet, keyPath, token, nil)
	defer clear(payload)
	if err != nil {
		return storedKey{}, keyUnavailable
	}
	switch status {
	case http.StatusOK:
		key, ok := parseKey(payload)
		if !ok {
			return storedKey{}, keyInvalid
		}
		return key, keyFound
	case http.StatusNotFound:
		var reply struct {
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(payload, &reply) != nil {
			return storedKey{}, keyInvalid
		}
		if len(reply.Data) != 0 && !bytes.Equal(reply.Data, []byte("null")) {
			// A deleted or destroyed entry keeps its version metadata and must never be recreated.
			return storedKey{}, keyInvalid
		}
		return storedKey{}, keyAbsent
	default:
		return storedKey{}, keyUnavailable
	}
}

// parseKey requires a canonical standard base64 encoding of exactly 32 bytes and a positive version.
func parseKey(payload []byte) (storedKey, bool) {
	var reply struct {
		Data *struct {
			Data *struct {
				Seed *string `json:"private_seed_b64"`
			} `json:"data"`
			Metadata *struct {
				Version *int64 `json:"version"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if json.Unmarshal(payload, &reply) != nil || reply.Data == nil ||
		reply.Data.Data == nil || reply.Data.Data.Seed == nil ||
		reply.Data.Metadata == nil || reply.Data.Metadata.Version == nil ||
		*reply.Data.Metadata.Version <= 0 || *reply.Data.Metadata.Version > maxLeaseSeconds {
		return storedKey{}, false
	}
	encoded := *reply.Data.Data.Seed
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	defer clear(decoded)
	var key storedKey
	if err != nil || len(decoded) != len(key.seed) || base64.StdEncoding.EncodeToString(decoded) != encoded {
		return storedKey{}, false
	}
	copy(key.seed[:], decoded)
	key.version = int(*reply.Data.Metadata.Version)
	return key, true
}

// createKey sends the one create-only write with compare-and-set version zero.
// Its reply is ignored: the caller always reads the stored entry afterwards.
func (c *Client) createKey(ctx context.Context, token string, seed *[32]byte) {
	encoded := base64.StdEncoding.EncodeToString(seed[:])
	body, err := json.Marshal(map[string]any{
		"options": map[string]int{"cas": 0},
		"data":    map[string]string{"private_seed_b64": encoded},
	})
	if err != nil {
		return
	}
	defer clear(body)
	_, payload, _ := c.do(ctx, http.MethodPost, keyPath, token, body)
	clear(payload)
}

// do sends one request under the five-second budget and reads at most 64 KiB of the reply.
func (c *Client) do(ctx context.Context, method, path, token string, body []byte) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(payload) > maxResponseBytes {
		clear(payload)
		return 0, nil, errResponseTooLarge
	}
	return resp.StatusCode, payload, nil
}

var errResponseTooLarge = errors.New("The OpenBao reply could not be read within its limit.")
