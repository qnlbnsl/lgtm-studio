package enroll

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func privateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func mustCreate(t *testing.T) (*Credential, string) {
	t.Helper()
	path := filepath.Join(privateDir(t), "machine.key")
	c, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c == nil {
		t.Fatal("Create returned no credential and no error")
	}
	return c, path
}

func parseKey(t *testing.T, data []byte) ed25519.PrivateKey {
	t.Helper()
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		t.Fatalf("file is not one PRIVATE KEY block: %q", data)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	priv, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		t.Fatalf("key is %T, want ed25519.PrivateKey", parsed)
	}
	return priv
}

func TestCreateWritesPrivateKeyFile(t *testing.T) {
	c, path := mustCreate(t)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	priv := parseKey(t, data)
	if !bytes.Equal(priv.Public().(ed25519.PublicKey), c.PublicKey()) {
		t.Error("the file's public key differs from PublicKey()")
	}
	pub := c.PublicKey()
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("public key has %d bytes", len(pub))
	}
	orig := bytes.Clone(pub)
	pub[0] ^= 0xff
	if !bytes.Equal(c.PublicKey(), orig) {
		t.Error("PublicKey() returns the credential's own slice, not a copy")
	}
}

func fingerprintOf(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

func TestLoadReturnsSameKey(t *testing.T) {
	c, path := mustCreate(t)
	l, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(l.PublicKey(), c.PublicKey()) || l.Fingerprint() != c.Fingerprint() {
		t.Error("Load gave another key than Create")
	}
	fp := c.Fingerprint()
	if fp != fingerprintOf(c.PublicKey()) || len(fp) != len("SHA256:")+43 {
		t.Errorf("Fingerprint() = %q, want %q", fp, fingerprintOf(c.PublicKey()))
	}
	m := []byte("a message to sign")
	if !ed25519.Verify(c.PublicKey(), m, c.Sign(m)) {
		t.Error("Sign does not verify")
	}
	if ed25519.Verify(c.PublicKey(), []byte("another"), c.Sign(m)) {
		t.Error("a signature verifies for another message")
	}
	if !ed25519.Verify(l.PublicKey(), m, l.Sign(m)) {
		t.Error("Sign of the loaded key does not verify")
	}
	s := c.Signer()
	if s == nil {
		t.Fatal("Signer() is nil")
	}
	sig, err := s.Sign(rand.Reader, m, crypto.Hash(0))
	if err != nil || !ed25519.Verify(c.PublicKey(), m, sig) {
		t.Errorf("Signer().Sign: %v, verifies: %v", err, err == nil && ed25519.Verify(c.PublicKey(), m, sig))
	}
	if pub, ok := s.Public().(ed25519.PublicKey); !ok || !bytes.Equal(pub, c.PublicKey()) {
		t.Error("Signer().Public() is not the public key")
	}
	if _, ok := s.(ed25519.PrivateKey); ok {
		t.Error("Signer() exposes the private key through a type assertion")
	}
}

func TestCreateRefusesExistingKey(t *testing.T) {
	_, path := mustCreate(t)
	before, _ := os.ReadFile(path)
	if c, err := Create(path); !errors.Is(err, ErrKeyExists) || c != nil {
		t.Fatalf("second Create: %v, %v; want ErrKeyExists and no credential", c, err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Error("the key file changed")
	}

	other := filepath.Join(privateDir(t), "machine.key")
	if err := os.WriteFile(other, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(other); !errors.Is(err, ErrKeyExists) {
		t.Errorf("Create over another file: %v, want ErrKeyExists", err)
	}
	if got, _ := os.ReadFile(other); string(got) != "not a key" {
		t.Errorf("the other file changed to %q", got)
	}
}

func TestLoadRefusesExposedKey(t *testing.T) {
	_, path := mustCreate(t)
	for _, mode := range []fs.FileMode{0o640, 0o604, 0o660} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		if c, err := Load(path); !errors.Is(err, ErrKeyExposed) || c != nil {
			t.Errorf("mode %o: Load gave %v, %v; want ErrKeyExposed", mode, c, err)
		}
	}
}

func TestLoadRefusesNonRegular(t *testing.T) {
	_, path := mustCreate(t)
	dir := filepath.Dir(path)

	link := filepath.Join(dir, "link.key")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "folder.key")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "pipe.key")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("no named pipes here: %v", err)
	}
	for name, p := range map[string]string{"link": link, "folder": folder, "named pipe": fifo} {
		done := make(chan error, 1)
		go func() {
			_, err := Load(p)
			done <- err
		}()
		select {
		case err := <-done:
			if !errors.Is(err, ErrKeyNotRegular) {
				t.Errorf("%s: Load error %v, want ErrKeyNotRegular", name, err)
			}
		case <-time.After(time.Second):
			t.Errorf("%s: Load did not return within 1 second", name)
		}
	}
}

func TestLoadRefusesBadContent(t *testing.T) {
	dir := privateDir(t)
	if _, err := Load(filepath.Join(dir, "nothing")); !errors.Is(err, ErrKeyMissing) {
		t.Errorf("missing path: %v, want ErrKeyMissing", err)
	}

	if os.Geteuid() != 0 {
		p := filepath.Join(dir, "unreadable")
		if err := os.WriteFile(p, []byte("x"), 0o200); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(p); !errors.Is(err, ErrKeyUnreadable) || !errors.Is(err, fs.ErrPermission) {
			t.Errorf("mode 0200: %v, want ErrKeyUnreadable wrapping fs.ErrPermission", err)
		}
	}

	_, goodPath := mustCreate(t)
	good, err := os.ReadFile(goodPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(goodPath); err != nil {
		t.Fatalf("a valid key with one newline after the block must load: %v", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(parseKey(t, good).Public())
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaDER, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	variants := map[string][]byte{
		"hello":             []byte("hello"),
		"public key block":  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
		"rsa key":           pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: rsaDER}),
		"junk before":       append([]byte("junk\n"), good...),
		"second block":      append(bytes.Clone(good), good...),
		"x after":           append(bytes.Clone(good), 'x'),
		"5000 spaces after": append(bytes.Clone(good), bytes.Repeat([]byte(" "), 5000)...),
		"empty":             {},
	}
	for name, content := range variants {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "_"))
		if err := os.WriteFile(p, content, 0o600); err != nil {
			t.Fatal(err)
		}
		c, err := Load(p)
		if !errors.Is(err, ErrKeyInvalid) || c != nil {
			t.Errorf("%s: Load gave %v, %v; want ErrKeyInvalid", name, c, err)
		}
		after, _ := os.ReadFile(p)
		if !bytes.Equal(after, content) || mustMode(t, p) != 0o600 {
			t.Errorf("%s: Load changed the file", name)
		}
	}
}

func mustMode(t *testing.T, p string) fs.FileMode {
	t.Helper()
	info, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// leaks reports whether text holds the seed in any common form.
func leaks(text string, key ed25519.PrivateKey) (string, bool) {
	forms := map[string][]byte{"seed": key.Seed(), "private key": key}
	for name, b := range forms {
		candidates := []string{
			string(b),
			hex.EncodeToString(b),
			strings.ToUpper(hex.EncodeToString(b)),
			base64.StdEncoding.EncodeToString(b),
			base64.RawStdEncoding.EncodeToString(b),
			base64.URLEncoding.EncodeToString(b),
			base64.RawURLEncoding.EncodeToString(b),
			fmt.Sprint(b),
			fmt.Sprintf("%#v", b),
			fmt.Sprintf("%d", b),
			strings.Trim(strings.Join(strings.Fields(fmt.Sprint(b)), ","), "[]"),
		}
		for _, c := range candidates {
			if len(c) >= 8 && strings.Contains(text, c) {
				return name + " as " + c[:8] + "...", true
			}
		}
	}
	return "", false
}

func TestCredentialNeverPrintsKey(t *testing.T) {
	c, path := mustCreate(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	key := parseKey(t, data)

	type exported struct{ Cred Credential }
	type unexported struct{ cred Credential }
	values := map[string]any{
		"value":           *c,
		"pointer":         c,
		"exported field":  exported{Cred: *c},
		"exported ptr":    &exported{Cred: *c},
		"unexported":      unexported{cred: *c},
		"unexported ptr":  &unexported{cred: *c},
		"slice of values": []Credential{*c},
		"map of values":   map[string]Credential{"k": *c},
	}
	for name, v := range values {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%x", "%q"} {
			text := fmt.Sprintf(verb, v)
			if form, bad := leaks(text, key); bad {
				t.Errorf("%s with %s prints the %s", name, verb, form)
			}
		}
	}
	for name, v := range map[string]any{"exported": exported{Cred: *c}, "exported ptr": &exported{Cred: *c}, "unexported": unexported{cred: *c}} {
		out, err := json.Marshal(v)
		if err != nil {
			continue
		}
		if form, bad := leaks(string(out), key); bad {
			t.Errorf("json.Marshal of %s holds the %s", name, form)
		}
	}

	const prefix = "machine key SHA256:"
	if !strings.HasPrefix(c.String(), prefix) {
		t.Errorf("String() = %q, want the prefix %q", c.String(), prefix)
	}
	if !strings.HasPrefix(fmt.Sprintf("%v", *c), prefix) || !strings.HasPrefix(fmt.Sprintf("%#v", *c), prefix) {
		t.Errorf("%%v and %%#v of the value must start with %q", prefix)
	}
	if got := c.GoString(); !strings.HasPrefix(got, prefix) {
		t.Errorf("GoString() = %q, want the prefix %q", got, prefix)
	}

	// Every error of a refusal must stay free of key bytes too.
	var texts []string
	add := func(err error) {
		if err != nil {
			texts = append(texts, err.Error())
		}
	}
	_, err = Create(path)
	add(err)
	for name, content := range map[string][]byte{
		"junk before": append([]byte("junk\n"), data...),
		"x after":     append(bytes.Clone(data), 'x'),
		"long":        append(bytes.Clone(data), bytes.Repeat([]byte(" "), 5000)...),
		"two blocks":  append(bytes.Clone(data), data...),
	} {
		p := filepath.Join(filepath.Dir(path), strings.ReplaceAll(name, " ", "_"))
		if err := os.WriteFile(p, content, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(p)
		add(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Load(path)
	add(err)
	if len(texts) < 6 {
		t.Fatalf("only %d refusals produced an error, want 6", len(texts))
	}
	for _, text := range texts {
		if form, bad := leaks(text, key); bad {
			t.Errorf("error %q holds the %s", text, form)
		}
	}
}
