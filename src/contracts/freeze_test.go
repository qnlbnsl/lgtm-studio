package contracts_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The tests of this file hold the freeze of the link (plan go-s0-contract, amendment "The freeze of
// the link", item F5). The record testdata/link-frozen.json is an array with one object per
// revision, each on one line: "revision", and "files", which maps the name of each file of
// proto/lgtm/link/v1/ (such as "link.proto") to its SHA-256 in lowercase hexadecimal.

const frozenRecord = "testdata/link-frozen.json"

type frozenRevision struct {
	Revision int               `json:"revision"`
	Files    map[string]string `json:"files"`
}

// TestLinkIsFrozenAtItsRevision holds the three files of the link to the last revision of the record.
func TestLinkIsFrozenAtItsRevision(t *testing.T) {
	raw, err := os.ReadFile(frozenRecord)
	if err != nil {
		t.Fatalf("the record of the frozen link is missing: %v", err)
	}
	var revisions []frozenRevision
	if err := json.Unmarshal(raw, &revisions); err != nil {
		t.Fatalf("the record of the frozen link does not parse: %v", err)
	}
	if len(revisions) == 0 {
		t.Fatalf("the record of the frozen link holds no revision")
	}
	// Each entry is on one line, because the check of the history compares lines.
	if n := strings.Count(string(raw), `"revision"`); n != len(revisions) {
		t.Errorf("the record holds %d entries and %d lines with \"revision\", want one line for each entry", len(revisions), n)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, `"revision"`) && !strings.Contains(line, `"files"`) {
			t.Errorf("an entry of the record is not on one line: %q", line)
		}
	}
	for i, r := range revisions {
		if r.Revision != i+1 {
			t.Errorf("entry %d is revision %d, want %d", i, r.Revision, i+1)
		}
	}
	last := revisions[len(revisions)-1]

	onDisk, err := filepath.Glob(filepath.Join(linkDir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range onDisk {
		names = append(names, filepath.Base(p))
	}
	sort.Strings(names)
	var recorded []string
	for n := range last.Files {
		recorded = append(recorded, n)
	}
	sort.Strings(recorded)
	if strings.Join(names, ",") != strings.Join(recorded, ",") {
		t.Errorf("revision %d names the files %v, but %s holds %v", last.Revision, recorded, linkDir, names)
	}
	for _, n := range names {
		body, err := os.ReadFile(filepath.Join(linkDir, n))
		if err != nil {
			t.Fatalf("cannot read %s: %v", n, err)
		}
		sum := sha256.Sum256(body)
		if got, want := hex.EncodeToString(sum[:]), last.Files[n]; got != want {
			t.Errorf("%s has the SHA-256 %s, but revision %d records %q: a change of the link needs revision %d", n, got, last.Revision, want, last.Revision+1)
		}
	}

	// The revision that both programs name is the last revision of the record.
	link := strings.Join(readLines(t, "link.proto"), "\n")
	named := regexp.MustCompile(`speaks, which is (\d+)\.`).FindAllStringSubmatch(link, -1)
	if len(named) != 2 {
		t.Errorf("link.proto names the revision of the client and of the server %d times with \"which is N\", want 2", len(named))
	}
	for _, m := range named {
		if n, _ := strconv.Atoi(m[1]); n != last.Revision {
			t.Errorf("link.proto says \"which is %s\", but the last revision of the record is %d", m[1], last.Revision)
		}
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("cannot read README.md: %v", err)
	}
	says := regexp.MustCompile("is frozen at revision (\\d+)").FindAllStringSubmatch(string(readme), -1)
	if len(says) != 1 {
		t.Fatalf("README.md says \"is frozen at revision N\" %d times, want once", len(says))
	}
	if n, _ := strconv.Atoi(says[0][1]); n != last.Revision {
		t.Errorf("README.md says the link is frozen at revision %s, but the last revision of the record is %d", says[0][1], last.Revision)
	}
}

const frozenMessage = "The link is frozen, so set LGTM_LINK_BASE to the commit of the freeze."

// TestFreezeCheckFailsClosed runs the real scripts/check-go.sh in a scratch repository, so that the
// check of the record's history is seen to fail closed (UNREVIEWED FIX, logic-r5 08). The scratch
// repository holds an empty Go module, so that the other steps of the script pass.
func TestFreezeCheckFailsClosed(t *testing.T) {
	script, err := os.ReadFile("../../scripts/check-go.sh")
	if err != nil {
		t.Fatalf("cannot read scripts/check-go.sh: %v", err)
	}
	rootMod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("cannot read go.mod: %v", err)
	}
	goLine := regexp.MustCompile(`(?m)^go \S+$`).FindString(string(rootMod))
	if goLine == "" {
		t.Fatalf("go.mod has no go line")
	}

	dir := t.TempDir()
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "LGTM_LINK_BASE=") || strings.HasPrefix(e, "GIT_") {
			continue
		}
		env = append(env, e)
	}
	run := func(extra []string, name string, args ...string) (string, int) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(append([]string{}, env...), extra...)
		out, err := cmd.CombinedOutput()
		var ee *exec.ExitError
		switch {
		case err == nil:
			return string(out), 0
		case errors.As(err, &ee):
			return string(out), ee.ExitCode()
		default:
			t.Fatalf("cannot run %s: %v", name, err)
			return "", -1
		}
	}
	git := func(args ...string) string {
		t.Helper()
		out, code := run(nil, "git", append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if code != 0 {
			t.Fatalf("git %v failed: %s", args, out)
		}
		return strings.TrimSpace(out)
	}
	write := func(rel, body string, mode os.FileMode) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	check := func(base string) (string, int) {
		var extra []string
		if base != "" {
			extra = []string{"LGTM_LINK_BASE=" + base}
		}
		return run(extra, "bash", "scripts/check-go.sh")
	}
	expect := func(what string, base string, wantCode int, wantText string) {
		t.Helper()
		out, code := check(base)
		if code != wantCode {
			t.Errorf("%s: scripts/check-go.sh exited %d, want %d. Output:\n%s", what, code, wantCode, out)
			return
		}
		if wantText != "" && !strings.Contains(out, wantText) {
			t.Errorf("%s: the output does not hold %q. Output:\n%s", what, wantText, out)
		}
	}

	git("init", "-q")
	write("go.mod", "module example.test/scratch\n\n"+goLine+"\n", 0o644)
	write("scratch.go", "package scratch\n", 0o644)
	write("scripts/check-go.sh", string(script), 0o755)
	git("add", "-A")
	git("commit", "-q", "-m", "before the freeze")
	before := git("rev-parse", "HEAD")

	one := strings.Repeat("a", 64)
	record := "[\n{\"revision\": 1, \"files\": {\"commands.proto\": \"" + one + "\", \"events.proto\": \"" + one + "\", \"link.proto\": \"" + one + "\"}}\n]\n"
	write("src/contracts/testdata/link-frozen.json", record, 0o644)

	// The build of item F5 adds the record, first in the working tree and then in HEAD itself.
	expect("the record is only in the working tree", "", 0, "")
	git("add", "-A")
	git("commit", "-q", "-m", "freeze the link at revision 1")
	freeze := git("rev-parse", "HEAD")
	expect("the commit of the freeze itself, with no baseline", "", 0, "")

	// A later commit has a commit before HEAD that holds the record.
	write("notes.txt", "later work\n", 0o644)
	git("add", "-A")
	git("commit", "-q", "-m", "later work")
	expect("a later commit with no baseline", "", 1, frozenMessage)
	expect("a later commit with the baseline of the freeze", freeze, 0, "")
	expect("a baseline before the freeze, which holds no record", before, 1, "")

	// A branch that changes a hash of revision 1 fails against the baseline.
	changed := strings.Replace(record, "\"link.proto\": \""+one, "\"link.proto\": \""+strings.Repeat("b", 64), 1)
	write("src/contracts/testdata/link-frozen.json", changed, 0o644)
	git("add", "-A")
	git("commit", "-q", "-m", "change the hash of revision 1")
	expect("a changed line of revision 1", freeze, 1, "")
	expect("a changed line of revision 1 with no baseline", "", 1, frozenMessage)
}
