package contracts_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The tests of this file read the text of the comments of the link. They cannot run the rules, which
// belong to the built programs. They hold the rules that a reader can mistake, so that a change to
// the text is seen (plan go-s0-contract, amendment "The freeze of the link", item F4).

const linkDir = "proto/lgtm/link/v1"

// readLines returns the lines of one proto file of the link.
func readLines(t *testing.T, file string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(linkDir, file))
	if err != nil {
		t.Fatalf("cannot read %s: %v", file, err)
	}
	return strings.Split(string(raw), "\n")
}

// commentBefore returns the raw lines of the comment block that ends directly above the first line
// whose trimmed text starts with decl.
func commentBefore(t *testing.T, lines []string, decl string) []string {
	t.Helper()
	at := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), decl) {
			at = i
			break
		}
	}
	if at < 0 {
		t.Fatalf("the declaration %q is not in the file", decl)
	}
	start := at
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "//") {
		start--
	}
	return lines[start:at]
}

var spaces = regexp.MustCompile(`\s+`)

// normalize joins comment lines into one text with single spaces and no comment markers.
func normalize(lines []string) string {
	var parts []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "//") {
			continue
		}
		parts = append(parts, strings.TrimPrefix(l, "//"))
	}
	return strings.TrimSpace(spaces.ReplaceAllString(strings.Join(parts, " "), " "))
}

// codeRow is one object of testdata/link-codes.json.
type codeRow struct {
	Call                   string   `json:"call"`
	Code                   string   `json:"code"`
	Action                 string   `json:"action"`
	Key                    string   `json:"key"`
	Reasons                []string `json:"reasons"`
	AfterVerifiedChallenge bool     `json:"after_verified_challenge"`
}

// tableRow is one row of the table of codes in the comment of LinkService.
type tableRow struct {
	call, code, action, text string
}

var (
	tableStart = regexp.MustCompile(`^//   (Join|Open|both)\s+(\S+)\s+(stop|dial again)\s+(.*)$`)
	tableCont  = regexp.MustCompile(`^//\s{20,}(\S.*)$`)
)

// readTable returns the rows of the table of codes, in order. A row's text runs over its
// continuation lines.
func readTable(t *testing.T) []tableRow {
	t.Helper()
	block := commentBefore(t, readLines(t, "link.proto"), "service LinkService {")
	var rows []tableRow
	for _, l := range block {
		if m := tableStart.FindStringSubmatch(l); m != nil {
			rows = append(rows, tableRow{call: m[1], code: m[2], action: m[3], text: m[4]})
			continue
		}
		if m := tableCont.FindStringSubmatch(l); m != nil && len(rows) > 0 {
			rows[len(rows)-1].text += " " + m[1]
		}
	}
	for i := range rows {
		rows[i].text = spaces.ReplaceAllString(rows[i].text, " ")
	}
	return rows
}

const verifiedSentence = "After a Challenge whose signature verified."

// TestLinkCodesAreWritten holds the table of codes in the comment of LinkService to the data file
// that plans go-s3-client-link and go-s3-server-stream read.
func TestLinkCodesAreWritten(t *testing.T) {
	raw, err := os.ReadFile("testdata/link-codes.json")
	if err != nil {
		t.Fatalf("the table of codes as data is missing: %v", err)
	}
	var want []codeRow
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("the table of codes does not parse: %v", err)
	}
	if len(want) != 9 {
		t.Fatalf("link-codes.json holds %d rows, want 9", len(want))
	}
	for i, w := range want {
		if w.Action != "stop" && w.Action != "dial again" {
			t.Errorf("row %d: the action is %q, want \"stop\" or \"dial again\"", i, w.Action)
		}
		// Every stop on Open waits for a verified signature: the answer before it is not proof.
		if w.Call == "Open" && w.Action == "stop" && !w.AfterVerifiedChallenge {
			t.Errorf("row %d: a stop of Open must hold after_verified_challenge", i)
		}
	}
	got := readTable(t)
	if len(got) != len(want) {
		t.Fatalf("the table in the comment of LinkService has %d rows, want %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.call != w.Call || g.code != w.Code || g.action != w.Action {
			t.Errorf("row %d reads %q %q %q, want %q %q %q", i, g.call, g.code, g.action, w.Call, w.Code, w.Action)
			continue
		}
		if w.Key != "" && !strings.Contains(g.text, "`"+w.Key+"`") {
			t.Errorf("row %d (%s %s) does not name its key `%s`", i, g.call, g.code, w.Key)
		}
		for _, r := range w.Reasons {
			if !strings.Contains(g.text, r) {
				t.Errorf("row %d (%s %s) does not name its reason %s", i, g.call, g.code, r)
			}
		}
		if has := strings.Contains(g.text, verifiedSentence); has != w.AfterVerifiedChallenge {
			t.Errorf("row %d (%s %s): the text says %q is %v, the data says after_verified_challenge is %v",
				i, g.call, g.code, verifiedSentence, has, w.AfterVerifiedChallenge)
		}
	}
}

// TestCodeFallbackIsWritten holds the rule for a reason that a machine does not know to Open
// failed_precondition alone, and says that both stops of Join hold whatever the reason.
func TestCodeFallbackIsWritten(t *testing.T) {
	text := normalize(commentBefore(t, readLines(t, "link.proto"), "service LinkService {"))
	phrases := []string{
		"For Open failed_precondition, a reason the machine does not know, or a missing reason, means dial again.",
		"Both stops of Join hold whatever the reason",
	}
	for _, p := range phrases {
		if n := strings.Count(text, p); n != 1 {
			t.Errorf("the comment of LinkService holds %q %d times, want once", p, n)
		}
	}
	if n := strings.Count(text, "a reason the machine does not know"); n != 1 {
		t.Errorf("the fallback for an unknown reason is written %d times, want once, for Open failed_precondition only", n)
	}
}

var joinReasonLine = regexp.MustCompile(`^  //   - (\w+):`)

// TestJoinRefusalOrderIsWritten holds the order in which Join checks a token.
func TestJoinRefusalOrderIsWritten(t *testing.T) {
	block := commentBefore(t, readLines(t, "link.proto"), "rpc Join(")
	var got []string
	for _, l := range block {
		if m := joinReasonLine.FindStringSubmatch(l); m != nil {
			got = append(got, m[1])
		}
	}
	want := []string{"token_unknown", "token_expired", "token_revoked", "token_used", "machine_removed"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the reasons of Join read %v in the comment, want %v", got, want)
	}
	if !strings.Contains(normalize(block), "checking in this order against its state at the moment it decides") {
		t.Errorf("the comment of Join no longer says that the refusals are checked in this order")
	}
}

// TestHandshakeOrderIsWritten holds the six decisions of the server after Challenge, as exact
// consecutive lines.
func TestHandshakeOrderIsWritten(t *testing.T) {
	block := commentBefore(t, readLines(t, "link.proto"), "rpc Open(")
	want := []string{
		"  //   - `unavailable` when it cannot look the machine up.",
		"  //   - `failed_precondition` with unknown_machine when it does not know the machine id.",
		"  //   - `failed_precondition` with link_revision_mismatch when it does not speak the revision.",
		"  //   - `deadline_exceeded` when Proof is not there 10 seconds after it sent Challenge.",
		"  //   - `invalid_argument` when Proof breaks a value rule.",
		"  //   - `unauthenticated` when the machine's signature does not verify.",
	}
	at := -1
	for i, l := range block {
		if l == want[0] {
			at = i
			break
		}
	}
	if at < 0 || at+len(want) > len(block) {
		t.Fatalf("the first decision after Challenge is not written as the exact line %q", want[0])
	}
	for i, w := range want {
		if block[at+i] != w {
			t.Errorf("decision %d reads %q, want %q", i+1, block[at+i], w)
		}
	}
	if !strings.Contains(normalize(block), "Then it decides in this order") {
		t.Errorf("the comment of Open no longer says that the server decides in this order")
	}
}

// TestLinkNumbersAreWritten holds the numbers of the link to exact phrases.
func TestLinkNumbersAreWritten(t *testing.T) {
	want := map[string][]string{
		"link.proto": {
			"Hello or Proof came more than 10 seconds late",
			"when Proof is not there 10 seconds after it sent Challenge",
			"when Hello is not there 10 seconds after the stream opened",
			"A machine that hears nothing from the server for 30 seconds ends the stream and dials again",
			"Event.number is at most 2^63 - 1, which is 9223372036854775807",
			"Heartbeat.cpu_percent and Heartbeat.memory_percent, when present, are 0 to 100",
			"HarnessesFound holds at most 16 harnesses",
			`"lgtm-link-v1 server"`,
			`"lgtm-link-v1 machine"`,
		},
		"events.proto": {
			"30 seconds without one, measured by the server's monotonic clock",
			"A machine that hears nothing from the server for 30 seconds ends the stream and dials again",
			"A number of 0, or above 2^63 - 1, ends the stream with `invalid_argument`",
			"At most 16 harnesses.",
			"The load of the processor, from 0 to 100",
			"The memory in use, from 0 to 100",
		},
		"commands.proto": {
			"until 23 hours after expires_at, and never later",
			"no earlier than 24 hours after expires_at",
			"until 24 hours after expires_at by its own clock",
		},
	}
	files := make([]string, 0, len(want))
	for f := range want {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		text := normalize(readLines(t, f))
		for _, p := range want[f] {
			if !strings.Contains(text, p) {
				t.Errorf("%s does not hold the phrase %q", f, p)
			}
		}
	}
}

// part is one piece of a proof text, with its place.
type part struct {
	name   string
	offset int
}

// TestProofCommentMatchesTheExample holds the comment of Proof to the order of the proof texts in
// the worked example, which TestProofExamplesHoldTogether builds and signs.
func TestProofCommentMatchesTheExample(t *testing.T) {
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
		if err != nil || len(b) == 0 {
			t.Fatalf("%s is missing or not hexadecimal", field)
		}
		return b
	}
	comment := normalize(commentBefore(t, readLines(t, "link.proto"), "message Proof {"))
	// The phrase that names each part in the comment.
	phrase := map[string]string{
		"label":         "a label",
		"zero byte":     "one zero byte",
		"length":        "one byte that holds the length of the machine id",
		"machine id":    "the machine id in UTF-8",
		"machine nonce": "the machine nonce",
		"server nonce":  "the server nonce",
	}
	id := []byte(ex["machine_id"])
	for _, c := range []struct{ field, label string }{
		{"machine_text", "lgtm-link-v1 machine"},
		{"server_text", "lgtm-link-v1 server"},
	} {
		text := dec(c.field)
		label := []byte(c.label)
		if !bytes.HasPrefix(text, label) || text[len(label)] != 0 || int(text[len(label)+1]) != len(id) {
			t.Fatalf("%s does not start with its label, a zero byte and the length of the machine id", c.field)
		}
		if !strings.Contains(comment, `"`+c.label+`"`) {
			t.Errorf("the comment of Proof does not name the label %q", c.label)
		}
		parts := []part{
			{"label", bytes.Index(text, label)},
			{"zero byte", len(label)},
			{"length", len(label) + 1},
			{"machine id", bytes.Index(text, id)},
			{"machine nonce", bytes.Index(text, dec("machine_nonce"))},
			{"server nonce", bytes.Index(text, dec("server_nonce"))},
		}
		sort.Slice(parts, func(i, j int) bool { return parts[i].offset < parts[j].offset })
		from := 0
		for _, p := range parts {
			i := strings.Index(comment[from:], phrase[p.name])
			if i < 0 {
				t.Errorf("%s: the comment of Proof does not name the %s (%q) after the parts before it", c.field, p.name, phrase[p.name])
				break
			}
			from += i + len(phrase[p.name])
		}
	}
}
