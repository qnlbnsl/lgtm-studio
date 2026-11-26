package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/scrypt"
)

// bindingPassword has spaces and non-ASCII characters on purpose: the comparison is over exact bytes.
const bindingPassword = "  pässword with spaces 日本語 "

func mustPassword(t *testing.T, value string) Password {
	t.Helper()
	p, err := NewPassword(value)
	if err != nil {
		t.Fatalf("NewPassword(%d bytes) failed: %v", len(value), err)
	}
	return p
}

func TestPasswordMatchesExactBytes(t *testing.T) {
	p := mustPassword(t, bindingPassword)

	if !p.Matches(bindingPassword) {
		t.Fatalf("the exact configured password was refused")
	}
	rejected := map[string]string{
		"trimmed":         strings.TrimSpace(bindingPassword),
		"leading space":   " " + bindingPassword,
		"trailing space":  bindingPassword + " ",
		"empty":           "",
		"prefix":          bindingPassword[:len(bindingPassword)-1],
		"upper case":      strings.ToUpper(bindingPassword),
		"changed byte":    bindingPassword[:5] + "X" + bindingPassword[6:],
		"changed last":    bindingPassword[:len(bindingPassword)-1] + "x",
		"normalized form": strings.Replace(bindingPassword, "ä", "ä", 1),
		"overlong":        bindingPassword + strings.Repeat("a", MaxPasswordBytes),
	}
	for name, candidate := range rejected {
		if p.Matches(candidate) {
			t.Errorf("the candidate %q was accepted", name)
		}
	}

	// The 4096-byte boundary: valid when configured, and exactly matched.
	edge := strings.Repeat("k", MaxPasswordBytes)
	pe, err := NewPassword(edge)
	if err != nil {
		t.Fatalf("a password of %d bytes was refused: %v", MaxPasswordBytes, err)
	}
	if !pe.Matches(edge) {
		t.Errorf("a candidate of %d bytes equal to the password was refused", MaxPasswordBytes)
	}
	if pe.Matches(edge + "k") {
		t.Errorf("a candidate of %d bytes was accepted over a password of %d bytes", MaxPasswordBytes+1, MaxPasswordBytes)
	}
	if pe.Matches(edge[:MaxPasswordBytes-1] + "j") {
		t.Errorf("a candidate that differs in its last byte was accepted")
	}

	// Constructor refusals, without the value in the message.
	secret := strings.Repeat("s", MaxPasswordBytes+1)
	refused := map[string]string{
		"empty":    "",
		"not UTF8": "abc\xff\xfedef",
		"overlong": secret,
	}
	for name, value := range refused {
		_, err := NewPassword(value)
		if err == nil {
			t.Errorf("NewPassword accepted the %s value", name)
			continue
		}
		if value != "" && strings.Contains(err.Error(), value) {
			t.Errorf("the %s error repeats the password", name)
		}
	}

	// The plan gates the constant-time primitive in this test, so it checks the source as well.
	checkDigestSource(t)
}

// checkDigestSource parses password.go: the helper that compares digests must use the
// constant-time primitive, and Matches must return the result of that helper.
func checkDigestSource(t *testing.T) {
	t.Helper()
	source, err := os.ReadFile("password.go")
	if err != nil {
		t.Fatalf("password.go could not be read: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "password.go", source, 0)
	if err != nil {
		t.Fatalf("password.go does not parse: %v", err)
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			funcs[fn.Name.Name] = fn
		}
	}

	equal := funcs["equalDigest"]
	if equal == nil || equal.Body == nil {
		t.Fatalf("password.go has no function equalDigest")
	}
	usesCompare := false
	ast.Inspect(equal.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ConstantTimeCompare" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "subtle" {
			usesCompare = true
		}
		return true
	})
	if !usesCompare {
		t.Errorf("equalDigest does not call subtle.ConstantTimeCompare on the two digests")
	}
	imported := false
	for _, spec := range file.Imports {
		if spec.Path.Value == `"crypto/subtle"` {
			imported = true
		}
	}
	if !imported {
		t.Errorf("password.go does not import crypto/subtle")
	}

	matches := funcs["Matches"]
	if matches == nil || matches.Body == nil {
		t.Fatalf("password.go has no method Matches")
	}
	returnsHelper := false
	ast.Inspect(matches.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, result := range ret.Results {
			ast.Inspect(result, func(m ast.Node) bool {
				if call, ok := m.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "equalDigest" {
						returnsHelper = true
					}
				}
				return true
			})
		}
		return true
	})
	if !returnsHelper {
		t.Errorf("Matches does not return the result of equalDigest")
	}
}

func TestEqualDigest(t *testing.T) {
	var a, b [32]byte
	for i := range a {
		a[i] = byte(i)
		b[i] = byte(i)
	}
	if !equalDigest(a, b) {
		t.Errorf("equal digests were reported different")
	}
	b[31] ^= 1
	if equalDigest(a, b) {
		t.Errorf("digests that differ in the last bit were reported equal")
	}
	b[31] ^= 1
	b[0] ^= 0x80
	if equalDigest(a, b) {
		t.Errorf("digests that differ in the first bit were reported equal")
	}
}

func bindingFixture() (salt [16]byte, verifier [32]byte, created time.Time) {
	for i := range salt {
		salt[i] = byte(i)
	}
	for i := range verifier {
		verifier[i] = byte(i)
	}
	return salt, verifier, time.UnixMicro(1795674600123456)
}

func hexTag(tag [32]byte) string { return hex.EncodeToString(tag[:]) }

func TestPasswordBindingVector(t *testing.T) {
	const want = "bb95f4bbe7cfe1874b6899129e91dc7c7566b70670fd95355088eab81641e1df"
	salt, verifier, created := bindingFixture()
	p := mustPassword(t, "fixture password")

	key, err := deriveBindingKey(p, salt)
	if err != nil {
		t.Fatalf("deriveBindingKey failed: %v", err)
	}
	tag := bindingTag(key, verifier, created)
	if got := hexTag(tag); got != want {
		t.Fatalf("binding tag = %s, want %s", got, want)
	}

	// bindPassword composes the two helpers, and the zone of the time does not matter.
	composed, err := bindPassword(p, salt, verifier, created.In(time.FixedZone("test", 5*3600+1800)))
	if err != nil {
		t.Fatalf("bindPassword failed: %v", err)
	}
	if got := hexTag(composed); got != want {
		t.Errorf("bindPassword tag = %s, want %s", got, want)
	}

	// Each input changes the tag.
	otherSalt := salt
	otherSalt[15] ^= 1
	otherVerifier := verifier
	otherVerifier[0] ^= 1
	changes := map[string]func() ([32]byte, error){
		"password": func() ([32]byte, error) {
			return bindPassword(mustPassword(t, "fixture passwore"), salt, verifier, created)
		},
		"salt":      func() ([32]byte, error) { return bindPassword(p, otherSalt, verifier, created) },
		"verifier":  func() ([32]byte, error) { return bindPassword(p, salt, otherVerifier, created) },
		"timestamp": func() ([32]byte, error) { return bindPassword(p, salt, verifier, created.Add(time.Microsecond)) },
	}
	for name, change := range changes {
		got, err := change()
		if err != nil {
			t.Errorf("changed %s: bindPassword failed: %v", name, err)
			continue
		}
		if got == tag {
			t.Errorf("changing the %s left the tag unchanged", name)
		}
	}
}

// The oracle rebuilds the documented layout with the libraries themselves, so a wrong cost,
// label, byte order or time unit fails here as well as against the frozen vector.
func TestPasswordBindingLayout(t *testing.T) {
	salt, verifier, created := bindingFixture()
	p := mustPassword(t, "fixture password")

	key, err := scrypt.Key([]byte("fixture password"), salt[:], 65536, 8, 1, 32)
	if err != nil {
		t.Fatalf("scrypt failed: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("lgtm-web-session-v1"))
	mac.Write([]byte{0})
	mac.Write(verifier[:])
	var micro [8]byte
	binary.BigEndian.PutUint64(micro[:], uint64(created.UnixMicro()))
	mac.Write(micro[:])
	want := hex.EncodeToString(mac.Sum(nil))

	got, err := deriveBindingKey(p, salt)
	if err != nil {
		t.Fatalf("deriveBindingKey failed: %v", err)
	}
	if hex.EncodeToString(key) != hex.EncodeToString(got[:]) {
		t.Errorf("the derived key differs from scrypt with N=65536, r=8, p=1 and 32 bytes")
	}
	if have := hexTag(bindingTag(got, verifier, created)); have != want {
		t.Errorf("binding tag = %s, want %s", have, want)
	}
}
