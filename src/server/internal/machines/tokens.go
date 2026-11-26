package machines

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Errors of the address of a token.
var (
	ErrInvalidAddress   = store.NewDomainError("The server address is not a loopback address.")
	ErrNoCertificatePin = store.NewDomainError("This server has no certificate to pin for an https address.")
	errInvalidServerKey = errors.New("The server does not have a valid signing public key.")
	errInvalidTokenText = errors.New("The join token text is invalid.")
)

const joinTokenPrefix = "lgtm1_"

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
type Tokens struct {
	store    *store.Store
	registry *Registry
	issuer   Issuer
}

// NewTokens returns the tokens of one server.
func NewTokens(s *store.Store, r *Registry, issuer Issuer) *Tokens {
	issuer.ServerKey = slices.Clone(issuer.ServerKey)
	return &Tokens{store: s, registry: r, issuer: issuer}
}

// Create makes one token that works once and for 15 minutes.
func (t *Tokens) Create(ctx context.Context, o TokenOptions) (NewToken, error) {
	address, err := NormalizeAddress(t.issuer.PublicURL)
	if err != nil {
		return NewToken{}, err
	}
	if len(t.issuer.ServerKey) != ed25519.PublicKeySize {
		return NewToken{}, errInvalidServerKey
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return NewToken{}, err
	}
	id, err := store.NewID()
	if err != nil {
		return NewToken{}, err
	}
	verifier := tokenVerifier(secret[:])
	keyHash := sha256.Sum256(t.issuer.ServerKey)
	text, err := EncodeToken(&linkv1.JoinToken{
		TokenId: id, Secret: secret[:], Address: address, ServerKeySha256: keyHash[:],
	})
	if err != nil {
		return NewToken{}, err
	}

	var expiresAt time.Time
	err = t.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		err := tx.QueryRow(ctx, `
WITH sampled AS (SELECT clock_timestamp() AS at)
INSERT INTO client_join_tokens (id, secret_sha256, created_at, expires_at, hosts_server)
SELECT $1, $2, at, at + interval '15 minutes', $3 FROM sampled
RETURNING expires_at`, id, verifier[:], o.HostsServer).Scan(&expiresAt)
		if err != nil {
			return err
		}
		return recordTokenChange(tx, id)
	})
	if err != nil {
		return NewToken{}, err
	}
	return NewToken{ID: id, Text: text, Address: address, ExpiresAt: expiresAt}, nil
}

// Revoke marks the token revoked and returns the machine that used it, or an empty string.
func (t *Tokens) Revoke(ctx context.Context, tx *store.Tx, id string) (string, error) {
	var usedBy string
	var revoked bool
	err := tx.QueryRow(ctx, `
SELECT COALESCE(used_by, ''), revoked_at IS NOT NULL
FROM client_join_tokens WHERE id = $1 FOR UPDATE`, id).Scan(&usedBy, &revoked)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if revoked {
		return usedBy, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE client_join_tokens SET revoked_at = clock_timestamp() WHERE id = $1`, id); err != nil {
		return "", err
	}
	if err := recordTokenChange(tx, id); err != nil {
		return "", err
	}
	return usedBy, nil
}

// NormalizeAddress checks and normalizes the address that a token carries.
func NormalizeAddress(address string) (string, error) {
	if strings.ContainsAny(address, "?#%") || strings.ContainsFunc(address, unicode.IsSpace) {
		return "", ErrInvalidAddress
	}
	origin, err := url.Parse(address)
	if err != nil || origin.Host == "" || origin.User != nil ||
		(origin.Path != "" && origin.Path != "/") || strings.HasSuffix(origin.Host, ":") {
		return "", ErrInvalidAddress
	}
	var port uint64
	if rawPort := origin.Port(); rawPort != "" {
		port, err = strconv.ParseUint(rawPort, 10, 16)
		if err != nil || port == 0 {
			return "", ErrInvalidAddress
		}
	}
	if origin.Scheme == "https" {
		return "", ErrNoCertificatePin
	}
	if origin.Scheme != "http" {
		return "", ErrInvalidAddress
	}
	host := strings.ToLower(origin.Host)
	if rawPort := origin.Port(); rawPort != "" {
		host = strings.TrimSuffix(host, ":"+rawPort)
	}
	switch host {
	case "127.0.0.1", "[::1]", "localhost":
	default:
		return "", ErrInvalidAddress
	}
	if port != 0 && port != 80 {
		host += ":" + strconv.FormatUint(port, 10)
	}
	return "http://" + host, nil
}

// EncodeToken returns the text of a token.
func EncodeToken(token *linkv1.JoinToken) (string, error) {
	payload, err := marshalToken(token)
	if err != nil {
		return "", err
	}
	return joinTokenPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodeToken parses the canonical text of a token.
func DecodeToken(text string) (*linkv1.JoinToken, error) {
	body, ok := strings.CutPrefix(text, joinTokenPrefix)
	if !ok || body == "" || strings.ContainsAny(body, "\r\n") {
		return nil, errInvalidTokenText
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(body)
	if err != nil {
		return nil, errInvalidTokenText
	}
	token := new(linkv1.JoinToken)
	if err := proto.Unmarshal(payload, token); err != nil {
		return nil, errInvalidTokenText
	}
	canonical, err := marshalToken(token)
	if err != nil || !bytes.Equal(payload, canonical) {
		return nil, errInvalidTokenText
	}
	return token, nil
}

func marshalToken(token *linkv1.JoinToken) ([]byte, error) {
	if token == nil || len(token.ProtoReflect().GetUnknown()) != 0 {
		return nil, errInvalidTokenText
	}
	payload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(token)
	if err != nil || len(payload) == 0 {
		return nil, errInvalidTokenText
	}
	return payload, nil
}

func tokenVerifier(secret []byte) [sha256.Size]byte {
	digest := sha256.New()
	digest.Write([]byte("lgtm-join-token-v1\x00"))
	digest.Write(secret)
	var sum [sha256.Size]byte
	digest.Sum(sum[:0])
	return sum
}

func recordTokenChange(tx *store.Tx, id string) error {
	return tx.Record(store.ChangeRef{Topic: "client_join_tokens", Key: id, Change: store.Saved})
}
