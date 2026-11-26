package machines_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
)

// These tests close the gaps post-a 3 and post-a 4 of the review of the built items 1 and 3.
// No test prints the text of a token.

const base64url = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// TestDecodeTokenCanonicalGuards: one token has one text. Each case below decodes to the same
// message as a good text, or to a near copy of it, so only a guard of DecodeToken refuses it.
func TestDecodeTokenCanonicalGuards(t *testing.T) {
	// A server address without a port gives a payload of 124 bytes, whose last character of
	// base64 holds four unused bits.
	e := newTokEnv(t, "http://127.0.0.1")
	_, msg := e.create(t, machines.TokenOptions{})
	text, err := machines.EncodeToken(msg)
	if err != nil {
		t.Fatalf("EncodeToken failed: %v", err)
	}
	body := strings.TrimPrefix(text, "lgtm1_")
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || len(payload)%3 != 1 {
		t.Fatalf("the fixture's payload has %d bytes (error %v), want a length of 1 modulo 3", len(payload), err)
	}
	if _, err := machines.DecodeToken(text); err != nil {
		t.Fatalf("the canonical text does not decode: %v", err)
	}

	last := strings.IndexByte(base64url, body[len(body)-1])
	cases := map[string]string{
		// The first field is token_id, written twice. The last one wins, so this parses, and
		// only the canonical check refuses it.
		"a field written twice": "lgtm1_" + base64.RawURLEncoding.EncodeToString(append(append([]byte{}, payload...), payload[:2+len(msg.GetTokenId())]...)),
		// The unused bits of the last character are not zero. Only strict decoding refuses it.
		"unused bits that are not zero": "lgtm1_" + body[:len(body)-1] + string(base64url[last^1]),
		"a line feed in the middle":     "lgtm1_" + body[:10] + "\n" + body[10:],
		"a carriage return":             "lgtm1_" + body[:10] + "\r" + body[10:],
		"a line feed at the end":        text + "\n",
	}
	for name, bad := range cases {
		if bad == text {
			t.Fatalf("the case %q equals the good text", name)
		}
		got, err := machines.DecodeToken(bad)
		if err == nil {
			t.Errorf("DecodeToken accepted the text with %s and returned the token %q", name, got.GetTokenId())
		}
	}
}

// TestTokenAddressGuards: the cases that the earlier tests of NormalizeAddress leave out.
func TestTokenAddressGuards(t *testing.T) {
	for _, address := range []string{
		"http://127.0.0.1:0",
		"http://127.0.0.1:",
		"http://[::1]:",
		"http://127.0.0.1:65536",
		"http://127.0.0.1:-1",
		"http://127.0.0.1%3a8787",
		"http://127.0.0.1%25",
		"http://127.0.0.1:8787 ",
		" http://127.0.0.1:8787",
		"http://127.0.0.1:8787\n",
		"http://127.0.0.1:8 787",
		"http://127.0.0.1:8787\t",
	} {
		got, err := machines.NormalizeAddress(address)
		if !errors.Is(err, machines.ErrInvalidAddress) {
			t.Errorf("NormalizeAddress(%q) returned %q and %v, want ErrInvalidAddress", address, got, err)
		}
		if got != "" {
			t.Errorf("NormalizeAddress(%q) returned the address %q with its refusal, want none", address, got)
		}
	}
}
