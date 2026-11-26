package machines_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

const (
	loopbackURL    = "http://127.0.0.1:8787"
	tokenTextStart = "lgtm1_"
	// verifierLabel is the label that the secret's verifier is made with.
	verifierLabel = "lgtm-join-token-v1"
)

// verifierOf is SHA-256 of the label, one zero byte and the secret.
func verifierOf(secret []byte) []byte {
	sum := sha256.Sum256(append(append([]byte(verifierLabel), 0), secret...))
	return sum[:]
}

func testServerKey() ed25519.PublicKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
}

// tokEnv is a regEnv with the tokens of one issuer.
type tokEnv struct {
	*regEnv
	tokens *machines.Tokens
	key    ed25519.PublicKey
}

func newTokEnv(t *testing.T, publicURL string) *tokEnv {
	t.Helper()
	e := newRegEnv(t)
	key := testServerKey()
	return &tokEnv{
		regEnv: e,
		tokens: machines.NewTokens(e.s, e.reg, machines.Issuer{PublicURL: publicURL, ServerKey: key}),
		key:    key,
	}
}

// create makes a token and decodes its text. It never prints the text.
func (e *tokEnv) create(t *testing.T, o machines.TokenOptions) (machines.NewToken, *linkv1.JoinToken) {
	t.Helper()
	tok, err := e.tokens.Create(t.Context(), o)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !strings.HasPrefix(tok.Text, tokenTextStart) {
		t.Fatalf("the token text does not start with %q (the text is not printed)", tokenTextStart)
	}
	msg, err := machines.DecodeToken(tok.Text)
	if err != nil || msg == nil {
		t.Fatalf("the token text does not decode: %v (the text is not printed)", err)
	}
	return tok, msg
}

type tokenRow struct {
	secretSHA256 []byte
	createdAt    time.Time
	expiresAt    time.Time
	hostsServer  bool
	revokedAt    *time.Time
	usedAt       *time.Time
	usedKey      []byte
	usedBy       *string
}

func (e *tokEnv) row(t *testing.T, id string) tokenRow {
	t.Helper()
	var r tokenRow
	err := e.obs.QueryRow(t.Context(), `
SELECT secret_sha256, created_at, expires_at, hosts_server, revoked_at, used_at, used_key, used_by
FROM client_join_tokens WHERE id = $1`, id).
		Scan(&r.secretSHA256, &r.createdAt, &r.expiresAt, &r.hostsServer, &r.revokedAt, &r.usedAt, &r.usedKey, &r.usedBy)
	if err != nil {
		t.Fatalf("reading the token row %q failed: %v", id, err)
	}
	return r
}

func (e *tokEnv) tokenCount(t *testing.T) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, e.obs, "SELECT count(*) FROM client_join_tokens")
}

func TestCreateTokenText(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	tok, msg := e.create(t, machines.TokenOptions{})

	if !uuidV7.MatchString(tok.ID) {
		t.Errorf("NewToken.ID is %q, want a version 7 identifier", tok.ID)
	}
	if msg.GetTokenId() != tok.ID {
		t.Errorf("the text holds the token id %q, want %q", msg.GetTokenId(), tok.ID)
	}
	if len(msg.GetSecret()) != 32 || bytes.Equal(msg.GetSecret(), make([]byte, 32)) {
		t.Errorf("the secret has %d bytes (or is all zero), want 32 random bytes", len(msg.GetSecret()))
	}
	if msg.GetAddress() != loopbackURL || tok.Address != loopbackURL {
		t.Errorf("the address is %q in the text and %q in NewToken, want %q in both", msg.GetAddress(), tok.Address, loopbackURL)
	}
	if want := sha256.Sum256(e.key); !bytes.Equal(msg.GetServerKeySha256(), want[:]) {
		t.Errorf("server_key_sha256 is %x, want the SHA-256 of the issuer's key %x", msg.GetServerKeySha256(), want)
	}
	if len(msg.GetCertificateKeySha256()) != 0 {
		t.Errorf("the certificate pin is %x, want none in wave 1", msg.GetCertificateKeySha256())
	}
	if tok.ExpiresAt.IsZero() {
		t.Error("NewToken.ExpiresAt is empty")
	}

	// Another token has another identifier and another secret.
	other, otherMsg := e.create(t, machines.TokenOptions{})
	if other.ID == tok.ID || bytes.Equal(otherMsg.GetSecret(), msg.GetSecret()) {
		t.Error("two tokens share an identifier or a secret")
	}

	// The change feed names the token by its identifier, in the topic of the table, with an empty scope.
	var saved int
	for _, entry := range e.feed(t) {
		if entry.Topic == "client_join_tokens" && entry.Scope == "" && entry.Change == "saved" && entry.Key == tok.ID {
			saved++
		}
	}
	if saved != 1 {
		t.Errorf("the feed holds %d saved entries of the token, want 1", saved)
	}

	// The worked example of the contract.
	raw, err := os.ReadFile("../../../contracts/testdata/join-token.json")
	if err != nil {
		t.Fatalf("reading the contract's example failed: %v", err)
	}
	var ex struct {
		TokenID, Secret, Address, ServerKeySHA256, CertificateKeySHA256, Text string
	}
	var fields map[string]string
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}
	ex.TokenID, ex.Secret, ex.Address = fields["token_id"], fields["secret"], fields["address"]
	ex.ServerKeySHA256, ex.CertificateKeySHA256, ex.Text = fields["server_key_sha256"], fields["certificate_key_sha256"], fields["text"]
	secret, err1 := hex.DecodeString(ex.Secret)
	keyHash, err2 := hex.DecodeString(ex.ServerKeySHA256)
	if err1 != nil || err2 != nil || len(ex.Text) == 0 {
		t.Fatalf("the example's fields do not parse (%v, %v)", err1, err2)
	}
	example := &linkv1.JoinToken{TokenId: ex.TokenID, Secret: secret, Address: ex.Address, ServerKeySha256: keyHash}
	encoded, err := machines.EncodeToken(example)
	if err != nil {
		t.Fatalf("EncodeToken failed: %v", err)
	}
	if encoded != ex.Text {
		t.Errorf("EncodeToken of the contract's example gave a text that differs from the example's text")
	}
	decoded, err := machines.DecodeToken(ex.Text)
	if err != nil || decoded == nil {
		t.Fatalf("DecodeToken of the contract's example failed: %v", err)
	}
	if decoded.GetTokenId() != ex.TokenID || !bytes.Equal(decoded.GetSecret(), secret) || decoded.GetAddress() != ex.Address ||
		!bytes.Equal(decoded.GetServerKeySha256(), keyHash) || len(decoded.GetCertificateKeySha256()) != 0 {
		t.Errorf("DecodeToken of the contract's example gave other fields: id %q, address %q", decoded.GetTokenId(), decoded.GetAddress())
	}

	// DecodeToken refuses another prefix, padding and trailing bytes.
	body := strings.TrimPrefix(ex.Text, tokenTextStart)
	rawBody, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatalf("the example's body does not decode: %v", err)
	}
	refused := map[string]string{
		"another prefix":    "lgtm2_" + body,
		"no prefix":         body,
		"an empty body":     tokenTextStart,
		"padding":           ex.Text + strings.Repeat("=", (4-len(body)%4)%4+4),
		"a trailing byte":   tokenTextStart + base64.RawURLEncoding.EncodeToString(append(append([]byte{}, rawBody...), 0)),
		"trailing text":     ex.Text + "AAAA",
		"standard alphabet": tokenTextStart + base64.RawStdEncoding.EncodeToString(append(append([]byte{}, rawBody...), 0xfb, 0xff)),
		"a space inside":    ex.Text[:20] + " " + ex.Text[20:],
	}
	for what, text := range refused {
		if got, err := machines.DecodeToken(text); err == nil {
			t.Errorf("DecodeToken accepted %s and returned a token with the id %q", what, got.GetTokenId())
		}
	}
}

func TestTokenLifetime(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	before := time.Now()
	tok, _ := e.create(t, machines.TokenOptions{})
	after := time.Now()

	row := e.row(t, tok.ID)
	exact := storetest.Scalar[bool](t, e.obs,
		"SELECT expires_at - created_at = interval '15 minutes' FROM client_join_tokens WHERE id = $1", tok.ID)
	if !exact {
		t.Errorf("the token lasts %v, want exactly 15 minutes", row.expiresAt.Sub(row.createdAt))
	}
	if !tok.ExpiresAt.Equal(row.expiresAt) {
		t.Errorf("NewToken.ExpiresAt is %v, want the row's expires_at %v", tok.ExpiresAt, row.expiresAt)
	}
	if row.createdAt.Before(before.Add(-5*time.Second)) || row.createdAt.After(after.Add(5*time.Second)) {
		t.Errorf("created_at is %v, want a time between %v and %v", row.createdAt, before, after)
	}
	if row.revokedAt != nil || row.usedAt != nil || row.usedKey != nil || row.usedBy != nil {
		t.Errorf("a new token is %+v, want unrevoked and unused", row)
	}
}

func TestTokenStoresOnlyVerifier(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	tok, msg := e.create(t, machines.TokenOptions{})
	secret := msg.GetSecret()
	if len(secret) != 32 {
		t.Fatalf("the secret has %d bytes, want 32", len(secret))
	}

	row := e.row(t, tok.ID)
	if !bytes.Equal(row.secretSHA256, verifierOf(secret)) {
		t.Error("secret_sha256 is not the SHA-256 of the label, a zero byte and the secret")
	}
	if plain := sha256.Sum256(secret); bytes.Equal(row.secretSHA256, plain[:]) {
		t.Error("secret_sha256 is the SHA-256 of the bare secret, want the labelled verifier")
	}
	if bytes.Equal(row.secretSHA256, secret) {
		t.Error("secret_sha256 holds the secret itself")
	}

	forms := map[string]string{
		"raw bytes":        string(secret),
		"hexadecimal":      hex.EncodeToString(secret),
		"upper hex":        strings.ToUpper(hex.EncodeToString(secret)),
		"base64":           base64.StdEncoding.EncodeToString(secret),
		"unpadded base64":  base64.RawStdEncoding.EncodeToString(secret),
		"base64url":        base64.URLEncoding.EncodeToString(secret),
		"unpadded base64u": base64.RawURLEncoding.EncodeToString(secret),
	}
	dumps := map[string][]string{
		"the token table": storetest.Rows[string](t, e.obs, "SELECT row_to_json(t)::text FROM client_join_tokens t"),
		"the feed":        storetest.Rows[string](t, e.obs, "SELECT row_to_json(f)::text FROM feed f"),
		"the machines":    storetest.Rows[string](t, e.obs, "SELECT row_to_json(m)::text FROM machines m"),
	}
	for where, dump := range dumps {
		joined := strings.Join(dump, "\n")
		for form, needle := range forms {
			if strings.Contains(joined, needle) {
				t.Errorf("the secret appears in %s as %s", where, form)
			}
		}
	}
	// The dump of the table is not empty, so that the search above looked at something.
	if len(dumps["the token table"]) != 1 {
		t.Errorf("the token table dump holds %d rows, want 1", len(dumps["the token table"]))
	}
}

func TestTokenAddressRules(t *testing.T) {
	ok := map[string]string{
		"HTTP://127.0.0.1:8787/": "http://127.0.0.1:8787",
		"http://127.0.0.1:8787":  "http://127.0.0.1:8787",
		"http://127.0.0.1":       "http://127.0.0.1",
		"http://127.0.0.1:80":    "http://127.0.0.1",
		"http://[::1]:80":        "http://[::1]",
		"http://[::1]:8787":      "http://[::1]:8787",
		"http://localhost":       "http://localhost",
		"http://LOCALHOST:8787/": "http://localhost:8787",
	}
	for in, want := range ok {
		got, err := machines.NormalizeAddress(in)
		if err != nil || got != want {
			t.Errorf("NormalizeAddress(%q) = %q, %v, want %q and no error", in, got, err, want)
		}
	}
	invalid := []string{
		"http://example.test:8787",
		"http://192.0.2.1:8787",
		"http://127.0.0.2:8787",
		"ftp://127.0.0.1",
		"ws://127.0.0.1",
		"http://127.0.0.1:8787/path",
		"http://127.0.0.1/a/",
		"http://127.0.0.1:8787?x=1",
		"http://127.0.0.1:8787#frag",
		"http://user@127.0.0.1:8787",
		"http://user:pass@127.0.0.1:8787",
		"127.0.0.1:8787",
		"//127.0.0.1:8787",
		"",
		"http://",
	}
	for _, in := range invalid {
		if got, err := machines.NormalizeAddress(in); !errors.Is(err, machines.ErrInvalidAddress) {
			t.Errorf("NormalizeAddress(%q) = %q, %v, want ErrInvalidAddress", in, got, err)
		}
	}
	for _, in := range []string{"https://box.example", "https://127.0.0.1:8787", "HTTPS://localhost"} {
		got, err := machines.NormalizeAddress(in)
		if !errors.Is(err, machines.ErrNoCertificatePin) || errors.Is(err, machines.ErrInvalidAddress) {
			t.Errorf("NormalizeAddress(%q) = %q, %v, want ErrNoCertificatePin and not ErrInvalidAddress", in, got, err)
		}
	}

	// Create passes the issuer's address through the same rules, and writes nothing when they refuse it.
	e := newTokEnv(t, "HTTP://127.0.0.1:8787/")
	tok, msg := e.create(t, machines.TokenOptions{})
	if tok.Address != loopbackURL || msg.GetAddress() != loopbackURL {
		t.Errorf("Create with the address HTTP://127.0.0.1:8787/ gave %q and %q, want %q", tok.Address, msg.GetAddress(), loopbackURL)
	}
	rows, head := e.tokenCount(t), e.feedHead(t)
	for _, in := range append(invalid[:len(invalid):len(invalid)], "https://box.example") {
		bad := machines.NewTokens(e.s, e.reg, machines.Issuer{PublicURL: in, ServerKey: e.key})
		made, err := bad.Create(t.Context(), machines.TokenOptions{})
		if err == nil {
			t.Errorf("Create with the address %q succeeded (the text is not printed)", in)
			continue
		}
		if made.Text != "" || made.ID != "" {
			t.Errorf("Create with the address %q returned a token together with its error", in)
		}
		if !errors.Is(err, machines.ErrInvalidAddress) && !errors.Is(err, machines.ErrNoCertificatePin) {
			t.Errorf("Create with the address %q returned %v, want ErrInvalidAddress or ErrNoCertificatePin", in, err)
		}
	}
	if got := e.tokenCount(t); got != rows {
		t.Errorf("a refused address left %d token rows, want %d", got, rows)
	}
	if got := e.feedHead(t); got != head {
		t.Errorf("a refused address moved the feed head from %d to %d", head, got)
	}
}

func TestHostsServerMark(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	marked, _ := e.create(t, machines.TokenOptions{HostsServer: true})
	plain, _ := e.create(t, machines.TokenOptions{})
	if !e.row(t, marked.ID).hostsServer {
		t.Error("a token made with HostsServer has hosts_server false")
	}
	if e.row(t, plain.ID).hostsServer {
		t.Error("a token made without HostsServer has hosts_server true")
	}
}

func TestRevoke(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	revoke := func(id string) (string, error) {
		var usedBy string
		err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
			var err error
			usedBy, err = e.tokens.Revoke(ctx, tx, id)
			return err
		})
		return usedBy, err
	}

	tok, _ := e.create(t, machines.TokenOptions{})
	head := int(e.feedHead(t))
	usedBy, err := revoke(tok.ID)
	if err != nil || usedBy != "" {
		t.Fatalf("Revoke of an unused token returned %q and %v, want an empty string and no error", usedBy, err)
	}
	row := e.row(t, tok.ID)
	if row.revokedAt == nil {
		t.Fatal("revoked_at is empty after the revocation")
	}
	if moved := e.feedSince(t, head); len(moved) != 1 || moved[0] != (regFeedEntry{"client_join_tokens", "", tok.ID, "saved"}) {
		t.Errorf("the revocation wrote the feed entries %+v, want one saved entry of the token", moved)
	}

	// A second revocation keeps the first time.
	first := *row.revokedAt
	if _, err := revoke(tok.ID); err != nil {
		t.Fatalf("a second Revoke failed: %v", err)
	}
	if again := e.row(t, tok.ID).revokedAt; again == nil || !again.Equal(first) {
		t.Errorf("a second revocation changed revoked_at from %v to %v", first, again)
	}

	// An unknown identifier is no error, and makes no row.
	rows, head := e.tokenCount(t), int(e.feedHead(t))
	usedBy, err = revoke(randomID(t))
	if err != nil || usedBy != "" {
		t.Errorf("Revoke of an unknown token returned %q and %v, want an empty string and no error", usedBy, err)
	}
	if got := e.tokenCount(t); got != rows {
		t.Errorf("Revoke of an unknown token changed the number of rows from %d to %d", rows, got)
	}
	if got := int(e.feedHead(t)); got != head {
		t.Errorf("Revoke of an unknown token moved the feed head from %d to %d", head, got)
	}

	// A token that a machine used returns the machine.
	used, _ := e.create(t, machines.TokenOptions{})
	machine := randomID(t)
	storetest.Exec(t, e.obs,
		"UPDATE client_join_tokens SET used_at = clock_timestamp(), used_key = $1, used_by = $2 WHERE id = $3",
		bytes.Repeat([]byte{9}, 32), machine, used.ID)
	usedBy, err = revoke(used.ID)
	if err != nil || usedBy != machine {
		t.Errorf("Revoke of a used token returned %q and %v, want the machine %q", usedBy, err, machine)
	}
	if e.row(t, used.ID).revokedAt == nil {
		t.Error("revoked_at of the used token is empty after the revocation")
	}
}

// TestTokenCreateKeepsExpiredRows checks the decision that no pruning exists in wave 1 (decision 92).
func TestTokenCreateKeepsExpiredRows(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	old, _ := e.create(t, machines.TokenOptions{})
	storetest.Exec(t, e.obs,
		"UPDATE client_join_tokens SET created_at = created_at - interval '3 days', expires_at = expires_at - interval '3 days' WHERE id = $1",
		old.ID)
	e.create(t, machines.TokenOptions{})
	e.create(t, machines.TokenOptions{})
	if got := e.tokenCount(t); got != 3 {
		t.Errorf("the table holds %d tokens after two more were made, want 3 (an expired token stays)", got)
	}
	if e.row(t, old.ID).expiresAt.After(time.Now()) {
		t.Error("the fixture did not expire the first token")
	}
}
