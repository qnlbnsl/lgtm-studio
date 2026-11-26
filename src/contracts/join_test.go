package contracts_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/proto"
)

// The values of the worked example in the plan go-s0-contract, item 5.
const (
	joinTokenID        = "0193a2b4-0000-7000-8000-000000000001"
	joinSecretHex      = "3333333333333333333333333333333333333333333333333333333333333333"
	joinAddress        = "http://127.0.0.1:8787"
	joinServerKeyHex   = "8a88e3dd7409f195fd52db2d3cba5d72ca6709bf1d94121bf3748801b40f6f5c"
	joinServerKeySHA   = "34750f98bd59fcfc946da45aaabe933be154a4b5094e1c4abf42866505f3c97e"
	joinTokenText      = "lgtm1_CiQwMTkzYTJiNC0wMDAwLTcwMDAtODAwMC0wMDAwMDAwMDAwMDESIDMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzGhVodHRwOi8vMTI3LjAuMC4xOjg3ODciIDR1D5i9Wfz8lG2kWqq-kzvhVKS1CU4cSr9ChmUF88l-"
	joinCertificateKey = ""
)

// TestJoinMatchesItsDescription holds the Join call and its messages to their description (item 5).
func TestJoinMatchesItsDescription(t *testing.T) {
	matchDescription(t, "testdata/golden/join.txt")
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hexadecimal %q: %v", s, err)
	}
	return b
}

// TestJoinTokenExampleDecodes checks the worked example of a join token (item 5).
func TestJoinTokenExampleDecodes(t *testing.T) {
	raw, err := os.ReadFile("testdata/join-token.json")
	if err != nil {
		t.Fatalf("the worked example is missing: %v", err)
	}
	var ex struct {
		TokenID              string `json:"token_id"`
		Secret               string `json:"secret"`
		Address              string `json:"address"`
		ServerPublicKey      string `json:"server_public_key"`
		ServerKeySHA256      string `json:"server_key_sha256"`
		CertificateKeySHA256 string `json:"certificate_key_sha256"`
		Text                 string `json:"text"`
	}
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatalf("the worked example does not parse: %v", err)
	}
	// The example equals the table of the plan.
	for name, pair := range map[string][2]string{
		"token_id":               {ex.TokenID, joinTokenID},
		"secret":                 {ex.Secret, joinSecretHex},
		"address":                {ex.Address, joinAddress},
		"server_public_key":      {ex.ServerPublicKey, joinServerKeyHex},
		"server_key_sha256":      {ex.ServerKeySHA256, joinServerKeySHA},
		"certificate_key_sha256": {ex.CertificateKeySHA256, joinCertificateKey},
		"text":                   {ex.Text, joinTokenText},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q", name, pair[0], pair[1])
		}
	}
	if !strings.HasPrefix(ex.Text, "lgtm1_") {
		t.Fatalf("the text must start with lgtm1_, it is %q", ex.Text)
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ex.Text, "lgtm1_"))
	if err != nil {
		t.Fatalf("the text after the prefix is not unpadded base64url: %v", err)
	}
	msg := newMessage(t, "lgtm.link.v1.JoinToken")
	if err := proto.Unmarshal(data, msg.Interface()); err != nil {
		t.Fatalf("the text does not unmarshal as a JoinToken: %v", err)
	}
	fields := msg.Descriptor().Fields()
	str := func(n string) string { return msg.Get(fields.ByName(protoName(n))).String() }
	by := func(n string) []byte { return msg.Get(fields.ByName(protoName(n))).Bytes() }
	if got := str("token_id"); got != ex.TokenID {
		t.Errorf("token_id = %q, want %q", got, ex.TokenID)
	}
	if got := by("secret"); !bytes.Equal(got, mustHex(t, ex.Secret)) {
		t.Errorf("secret = %x, want %s", got, ex.Secret)
	}
	if got := str("address"); got != ex.Address {
		t.Errorf("address = %q, want %q", got, ex.Address)
	}
	if got := by("server_key_sha256"); !bytes.Equal(got, mustHex(t, ex.ServerKeySHA256)) {
		t.Errorf("server_key_sha256 = %x, want %s", got, ex.ServerKeySHA256)
	}
	if got := by("certificate_key_sha256"); !bytes.Equal(got, mustHex(t, ex.CertificateKeySHA256)) {
		t.Errorf("certificate_key_sha256 = %x, want %q", got, ex.CertificateKeySHA256)
	}
	sum := sha256.Sum256(mustHex(t, ex.ServerPublicKey))
	if hex.EncodeToString(sum[:]) != ex.ServerKeySHA256 {
		t.Errorf("the SHA-256 of server_public_key is %x, want %s", sum, ex.ServerKeySHA256)
	}
	again, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg.Interface())
	if err != nil || !bytes.Equal(again, data) {
		t.Errorf("marshalling the token again gives other bytes (error %v)", err)
	}
}
