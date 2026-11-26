package contracts_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// The values of the worked example in the plan go-s0-contract, item 6.
const (
	proofServerKey    = "8a88e3dd7409f195fd52db2d3cba5d72ca6709bf1d94121bf3748801b40f6f5c"
	proofMachineKey   = "8139770ea87d175f56a35466c34c7ecccb8d8a91b4ee37a25df60f5b8fc9b394"
	proofMachineID    = "0193a2b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b"
	proofServerSig    = "f754334e7dd8404dca1420b2a5a17113b431cb0160e57ed2ff5665d0054474b265a01059a52647d78a4fb4d19bc29570daefe451c0344ae17aecd07a21a4b60a"
	proofMachineSig   = "977cea1412c854aa10e7408112dcf89eca4a508f2ea37714032f5c98bf343b0fffd5904eba1d5e89bb9b139d54daf983d76206000ee48b457e12a41cc7cc500e"
	proofServerLabel  = "lgtm-link-v1 server"
	proofMachineLabel = "lgtm-link-v1 machine"
)

// TestOpenMatchesItsDescription holds the Open call and its messages to their description (item 6).
func TestOpenMatchesItsDescription(t *testing.T) {
	matchDescription(t, "testdata/golden/open.txt")
}

// headerLine matches the first line of a block of a description file.
var headerLine = regexp.MustCompile(`^(rpc|message|enum) (\S+)`)

// TestEveryLinkNameIsDescribed makes sure that no message, enum or method of the link goes without a description (item 6).
func TestEveryLinkNameIsDescribed(t *testing.T) {
	named := map[string]int{}
	for _, f := range []string{"events", "commands", "join", "open"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "golden", f+".txt"))
		if err != nil {
			t.Fatalf("cannot read the description %s.txt: %v", f, err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if m := headerLine.FindStringSubmatch(line); m != nil {
				named[m[2]]++
			}
		}
	}
	seen := 0
	var visit func(protoreflect.FileDescriptor)
	check := func(name protoreflect.FullName) {
		seen++
		if n := named[string(name)]; n != 1 {
			t.Errorf("%s is named by %d description files, want exactly 1", name, n)
		}
	}
	visit = func(f protoreflect.FileDescriptor) {
		for i := 0; i < f.Messages().Len(); i++ {
			check(f.Messages().Get(i).FullName())
		}
		for i := 0; i < f.Enums().Len(); i++ {
			check(f.Enums().Get(i).FullName())
		}
		for i := 0; i < f.Services().Len(); i++ {
			ms := f.Services().Get(i).Methods()
			for j := 0; j < ms.Len(); j++ {
				check(ms.Get(j).FullName())
			}
		}
	}
	protoregistry.GlobalFiles.RangeFilesByPackage("lgtm.link.v1", func(f protoreflect.FileDescriptor) bool {
		visit(f)
		return true
	})
	if seen == 0 {
		t.Errorf("the package lgtm.link.v1 holds no message, enum or method yet")
	}
}

var (
	declaration = regexp.MustCompile(`^(service|rpc|message|enum|oneof)\s+\w+`)
	fieldLine   = regexp.MustCompile(`^((optional|repeated)\s+)?(map<[^>]+>|[A-Za-z_][\w.]*)\s+\w+\s*=\s*\d+`)
	valueLine   = regexp.MustCompile(`^\w+\s*=\s*\d+`)
	unspecified = regexp.MustCompile(`^\w+_UNSPECIFIED\s*=\s*0`)
	twoLetters  = regexp.MustCompile(`[A-Za-z]{2,}`)
)

// isDeclaration says whether a source line declares something that needs a comment.
func isDeclaration(line string) bool {
	for _, skip := range []string{"syntax", "package", "option", "import", "reserved", "}", "//", "extensions"} {
		if strings.HasPrefix(line, skip) && !declaration.MatchString(line) {
			return false
		}
	}
	return declaration.MatchString(line) || fieldLine.MatchString(line) || valueLine.MatchString(line)
}

// TestEveryLinkRuleIsWritten reads the three link files as text, because the generated Go keeps no comments (item 6).
func TestEveryLinkRuleIsWritten(t *testing.T) {
	texts := map[string]string{}
	for _, name := range []string{"link.proto", "commands.proto", "events.proto"} {
		raw, err := os.ReadFile(filepath.Join("proto", "lgtm", "link", "v1", name))
		if err != nil {
			t.Fatalf("cannot read %s: %v", name, err)
		}
		texts[name] = string(raw)
		lines := strings.Split(string(raw), "\n")
		for i, l := range lines {
			line := strings.TrimSpace(l)
			if !isDeclaration(line) || unspecified.MatchString(line) {
				continue
			}
			var block []string
			for j := i - 1; j >= 0; j-- {
				prev := strings.TrimSpace(lines[j])
				if !strings.HasPrefix(prev, "//") {
					break
				}
				block = append(block, strings.TrimPrefix(prev, "//"))
			}
			if words := twoLetters.FindAllString(strings.Join(block, " "), -1); len(words) < 3 {
				t.Errorf("%s:%d has no comment of three words above it: %s", name, i+1, line)
			}
		}
	}
	for _, w := range []string{"permission_denied", "unauthenticated", "failed_precondition", "invalid_argument", "deadline_exceeded", "aborted", "unavailable", "Lgtm-Link-Token", "Lgtm-Join-Refusal", "Lgtm-Link-Refusal"} {
		if !strings.Contains(texts["link.proto"], w) {
			t.Errorf("link.proto does not name %s", w)
		}
	}
	if !strings.Contains(texts["events.proto"], "failed_precondition") {
		t.Errorf("events.proto does not name failed_precondition")
	}
}

// TestProofExamplesHoldTogether checks the worked examples of the signed proof texts (item 6).
func TestProofExamplesHoldTogether(t *testing.T) {
	raw, err := os.ReadFile("testdata/link-proof.json")
	if err != nil {
		t.Fatalf("the worked example is missing: %v", err)
	}
	var ex map[string]string
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatalf("the worked example does not parse: %v", err)
	}
	dec := func(field string) []byte {
		b, err := hex.DecodeString(ex[field])
		if err != nil || ex[field] == "" {
			t.Fatalf("%s is missing or not hexadecimal", field)
		}
		return b
	}
	serverKey := ed25519.NewKeyFromSeed(dec("server_seed"))
	machineKey := ed25519.NewKeyFromSeed(dec("machine_seed"))
	serverPub, machinePub := serverKey.Public().(ed25519.PublicKey), machineKey.Public().(ed25519.PublicKey)
	if hex.EncodeToString(serverPub) != proofServerKey || ex["server_public_key"] != proofServerKey {
		t.Errorf("the server's public key is not the one in the plan")
	}
	if hex.EncodeToString(machinePub) != proofMachineKey || ex["machine_public_key"] != proofMachineKey {
		t.Errorf("the machine's public key is not the one in the plan")
	}
	if ex["machine_id"] != proofMachineID {
		t.Errorf("machine_id = %q, want %q", ex["machine_id"], proofMachineID)
	}
	build := func(label string) []byte {
		var b []byte
		b = append(b, label...)
		b = append(b, 0, byte(len(ex["machine_id"])))
		b = append(b, ex["machine_id"]...)
		b = append(b, dec("machine_nonce")...)
		return append(b, dec("server_nonce")...)
	}
	serverText, machineText := build(proofServerLabel), build(proofMachineLabel)
	if hex.EncodeToString(serverText) != ex["server_text"] {
		t.Errorf("server_text is %s, built from the rule it is %x", ex["server_text"], serverText)
	}
	if hex.EncodeToString(machineText) != ex["machine_text"] {
		t.Errorf("machine_text is %s, built from the rule it is %x", ex["machine_text"], machineText)
	}
	if ex["server_signature"] != proofServerSig {
		t.Errorf("server_signature is not the value in the plan")
	}
	if ex["machine_signature"] != proofMachineSig {
		t.Errorf("machine_signature is not the value in the plan")
	}
	serverSig, machineSig := mustHex(t, proofServerSig), mustHex(t, proofMachineSig)
	if !ed25519.Verify(serverPub, serverText, serverSig) {
		t.Errorf("the server's signature does not verify against the server's text")
	}
	if !ed25519.Verify(machinePub, machineText, machineSig) {
		t.Errorf("the machine's signature does not verify against the machine's text")
	}
	if ed25519.Verify(serverPub, machineText, serverSig) {
		t.Errorf("the server's signature verifies against the machine's text")
	}
	if ed25519.Verify(machinePub, serverText, machineSig) {
		t.Errorf("the machine's signature verifies against the server's text")
	}
}
