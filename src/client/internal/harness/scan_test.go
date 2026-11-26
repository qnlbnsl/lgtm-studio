package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// writeFake writes an executable script named name into dir and returns its path.
func writeFake(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// absSleep returns the absolute path of sleep, because a fake runs with a PATH of only its own folder.
func absSleep(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep program on this machine")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func find(t *testing.T, got []Found, name Name) Found {
	t.Helper()
	for _, f := range got {
		if f.Harness == name {
			return f
		}
	}
	t.Fatalf("the scan returned no result for %s: %+v", name, got)
	return Found{}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestScanFindsFakePrograms(t *testing.T) {
	dir := t.TempDir()
	claude := writeFake(t, dir, "claude", "echo '2.1.325 (Claude Code)'")
	codex := writeFake(t, dir, "codex", "echo 'codex-cli 0.98.0'")
	gemini := writeFake(t, dir, "gemini", "echo 0.68.0")
	cursor := writeFake(t, dir, "cursor-agent", "echo 2026.11.20-abc1234")

	got := Scan(context.Background(), ScanOptions{SearchPath: []string{dir}, Home: t.TempDir()})
	if len(got) != 4 {
		t.Fatalf("got %d results: %+v", len(got), got)
	}
	want := []struct {
		name    Name
		version string
		path    string
	}{
		{Claude, "2.1.325", claude},
		{Codex, "0.98.0", codex},
		{Gemini, "0.68.0", gemini},
		{Cursor, "2026.11.20-abc1234", cursor},
	}
	for i, w := range want {
		f := got[i]
		if f.Harness != w.name || !f.Installed || f.Version != w.version || f.Path != w.path || f.Problem != ProblemNone {
			t.Errorf("result %d is %+v, want %s %s %s", i, f, w.name, w.version, w.path)
		}
	}
}

func TestScanEmptySearchPathFindsNothing(t *testing.T) {
	dir := t.TempDir()
	writeFake(t, dir, "claude", "echo '2.1.325 (Claude Code)'")
	t.Setenv("PATH", dir)
	got := Scan(context.Background(), ScanOptions{Home: t.TempDir()})
	if len(got) != 4 {
		t.Fatalf("got %d results", len(got))
	}
	for _, f := range got {
		if f.Installed || f.Problem != ProblemNotFound {
			t.Errorf("%s is %+v, want not installed and %q", f.Harness, f, ProblemNotFound)
		}
	}
}

func TestScanRefusesExtraText(t *testing.T) {
	dir := t.TempDir()
	writeFake(t, dir, "claude", "echo '2.1.325 (Claude Code) token=sk-planted-secret'")
	writeFake(t, dir, "codex", "echo 'warning: key 1.2.3-sk-planted'\necho 'codex-cli 0.98.0'")
	writeFake(t, dir, "gemini", "echo 0.68.0\necho 'token=sk-planted-later'")
	got := Scan(context.Background(), ScanOptions{SearchPath: []string{dir}, Home: t.TempDir()})
	for _, name := range []Name{Claude, Codex} {
		f := find(t, got, name)
		if f.Installed || f.Problem != ProblemNoVersion || f.Version != "" {
			t.Errorf("%s is %+v, want %q", name, f, ProblemNoVersion)
		}
	}
	// Only the first line counts, so text after it is read and thrown away.
	if f := find(t, got, Gemini); !f.Installed || f.Version != "0.68.0" {
		t.Errorf("gemini is %+v", f)
	}
	if text := fmt.Sprintf("%+v %#v", got, got); strings.Contains(text, "sk-planted") {
		t.Errorf("a result holds the planted secret: %s", text)
	}
}

func TestScanGivesOnlyVersionAndCleanEnvironment(t *testing.T) {
	t.Setenv("LGTM_PLANTED_SECRET", "planted")
	dir := t.TempDir()
	home := t.TempDir()
	writeFake(t, dir, "claude", `{
for a in "$@"; do echo "arg=$a"; done
export -p
echo "wd=$(pwd -P)"
read line
echo "stdin=[$line]"
} >> "$0.out"
echo '2.1.325 (Claude Code)'`)
	got := Scan(context.Background(), ScanOptions{
		SearchPath: []string{"", "rel", dir, dir},
		Home:       home,
	})
	if f := find(t, got, Claude); !f.Installed {
		t.Fatalf("claude is %+v", f)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "claude.out"))
	if err != nil {
		t.Fatalf("the fake did not run: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "planted") {
		t.Errorf("the child saw the planted secret:\n%s", text)
	}
	if n := strings.Count(text, "arg="); n != 1 || !strings.Contains(text, "arg=--version\n") {
		t.Errorf("the arguments are not exactly --version (or it ran twice):\n%s", text)
	}
	if !strings.Contains(text, "stdin=[]") {
		t.Errorf("the input is not empty:\n%s", text)
	}
	realHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "wd="+realHome+"\n") {
		t.Errorf("the folder of the child is not the home %s:\n%s", realHome, text)
	}
	vars := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		rest, found := strings.CutPrefix(line, "export ")
		if !found {
			rest, found = strings.CutPrefix(line, "declare -x ")
		}
		if !found {
			continue
		}
		k, v, ok := strings.Cut(rest, "=")
		if !ok {
			continue
		}
		vars[k] = strings.Trim(v, `'"`)
	}
	if vars["PATH"] != dir {
		t.Errorf("PATH is %q, want %q", vars["PATH"], dir)
	}
	if vars["HOME"] != home {
		t.Errorf("HOME is %q, want %q", vars["HOME"], home)
	}
	if vars["LANG"] != "C" {
		t.Errorf("LANG is %q", vars["LANG"])
	}
	shellOwn := map[string]bool{"PATH": true, "HOME": true, "LANG": true, "PWD": true, "OLDPWD": true, "SHLVL": true, "_": true, "IFS": true, "PS1": true, "PS2": true, "PS4": true, "OPTIND": true, "LINENO": true}
	for k := range vars {
		if !shellOwn[k] {
			t.Errorf("the child got the unexpected variable %s", k)
		}
	}
}

func TestScanNeverUsesShell(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "bin $(touch INJECTED);x")
	home := t.TempDir()
	writeFake(t, dir, "claude", "echo '2.1.325 (Claude Code)'")
	t.Cleanup(func() { os.Remove("INJECTED") })
	got := Scan(context.Background(), ScanOptions{SearchPath: []string{dir}, Home: home})
	if f := find(t, got, Claude); !f.Installed || f.Version != "2.1.325" {
		t.Errorf("claude is %+v", f)
	}
	for _, p := range []string{filepath.Join(home, "INJECTED"), filepath.Join(dir, "INJECTED"), filepath.Join(root, "INJECTED"), "INJECTED"} {
		if exists(p) {
			t.Errorf("a shell ran the folder name: %s exists", p)
		}
	}
}

func TestScanTimesOut(t *testing.T) {
	sleep := absSleep(t)
	dir := t.TempDir()
	gemini := writeFake(t, dir, "gemini", fmt.Sprintf(`: > "$0.started"
%s 30`, sleep))
	opts := ScanOptions{SearchPath: []string{dir}, Home: t.TempDir(), Timeout: 500 * time.Millisecond}

	start := time.Now()
	got := Scan(context.Background(), opts)
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("the scan took %s", took)
	}
	f := find(t, got, Gemini)
	if f.Installed || f.Problem != ProblemTimedOut {
		t.Errorf("gemini is %+v, want %q", f, ProblemTimedOut)
	}
	if !exists(gemini + ".started") {
		t.Error("the fake never started, so the test proves nothing")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start = time.Now()
	got = Scan(ctx, ScanOptions{SearchPath: []string{dir}, Home: t.TempDir(), Timeout: 10 * time.Second})
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("the scan with a cancelled context took %s", took)
	}
	for _, f := range got {
		if f.Installed {
			t.Errorf("%s is installed after a cancelled context", f.Harness)
		}
	}
}

func TestScanDoesNotWaitForGrandchild(t *testing.T) {
	sleep := absSleep(t)
	dir := t.TempDir()
	gemini := writeFake(t, dir, "gemini", fmt.Sprintf(`%s 30 &
echo $! > "$0.pid"
echo 0.68.0`, sleep))
	start := time.Now()
	got := Scan(context.Background(), ScanOptions{SearchPath: []string{dir}, Home: t.TempDir(), Timeout: 10 * time.Second})
	took := time.Since(start)

	raw, err := os.ReadFile(gemini + ".pid")
	if err != nil {
		t.Fatalf("the fake did not run: %v", err)
	}
	var pid int
	if _, err := fmt.Sscan(strings.TrimSpace(string(raw)), &pid); err != nil || pid <= 1 {
		t.Fatalf("bad pid file %q", raw)
	}
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })

	if took > 3*time.Second {
		t.Errorf("the scan waited for the grandchild: %s", took)
	}
	if f := find(t, got, Gemini); !f.Installed || f.Version != "0.68.0" {
		t.Errorf("gemini is %+v", f)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Errorf("the grandchild is gone, so the test proves nothing: %v", err)
	}
}

func TestScanSharesOneBudget(t *testing.T) {
	sleep := absSleep(t)

	// One budget of 500 milliseconds is spent by the first candidate.
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	third := filepath.Join(root, "third")
	writeFake(t, first, "cursor-agent", fmt.Sprintf("%s 30", sleep))
	a1 := writeFake(t, second, "agent", fmt.Sprintf(": > \"$0.started\"\n%s 30", sleep))
	a2 := writeFake(t, third, "agent", fmt.Sprintf(": > \"$0.started\"\n%s 30", sleep))
	start := time.Now()
	got := Scan(context.Background(), ScanOptions{SearchPath: []string{first, second, third}, Home: t.TempDir(), Timeout: 500 * time.Millisecond})
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("the scan took %s", took)
	}
	if f := find(t, got, Cursor); f.Installed || f.Problem != ProblemTimedOut {
		t.Errorf("cursor is %+v, want %q", f, ProblemTimedOut)
	}
	for _, p := range []string{a1, a2} {
		if exists(p + ".started") {
			t.Errorf("%s started after the budget was spent", p)
		}
	}

	// A candidate that ends early leaves the rest of the budget to the next one.
	root = t.TempDir()
	first = filepath.Join(root, "first")
	second = filepath.Join(root, "second")
	writeFake(t, first, "cursor-agent", fmt.Sprintf("%s 1.5\nexit 1", sleep))
	a3 := writeFake(t, second, "agent", fmt.Sprintf(": > \"$0.started\"\nexec %s 30", sleep))
	start = time.Now()
	got = Scan(context.Background(), ScanOptions{SearchPath: []string{first, second}, Home: t.TempDir(), Timeout: 2 * time.Second})
	if took := time.Since(start); took > 3500*time.Millisecond {
		t.Errorf("the scan took %s", took)
	}
	if !exists(a3 + ".started") {
		t.Error("the later agent never started although budget was left")
	}
	if f := find(t, got, Cursor); f.Installed || f.Problem != ProblemFailed {
		t.Errorf("cursor is %+v, want the problem of the first candidate, %q", f, ProblemFailed)
	}
}

func TestScanReportsFailureAndNoVersion(t *testing.T) {
	dir := t.TempDir()
	writeFake(t, dir, "claude", "exit 1")
	writeFake(t, dir, "gemini", "echo 0.68.0\nexit 1")
	writeFake(t, dir, "codex", "echo hello")
	otherAgent := t.TempDir()
	writeFake(t, otherAgent, "agent", "echo 1.4.2")
	got := Scan(context.Background(), ScanOptions{SearchPath: []string{dir, otherAgent}, Home: t.TempDir()})
	checks := map[Name]Problem{Claude: ProblemFailed, Gemini: ProblemFailed, Codex: ProblemNoVersion, Cursor: ProblemNoVersion}
	for name, want := range checks {
		f := find(t, got, name)
		if f.Installed || f.Problem != want || f.Version != "" {
			t.Errorf("%s is %+v, want %q", name, f, want)
		}
	}

	real := t.TempDir()
	cursorPath := writeFake(t, real, "agent", "echo 2026.11.20-abc1234")
	got = Scan(context.Background(), ScanOptions{SearchPath: []string{otherAgent, real}, Home: t.TempDir()})
	f := find(t, got, Cursor)
	if !f.Installed || f.Version != "2026.11.20-abc1234" || f.Path != cursorPath || f.Problem != ProblemNone {
		t.Errorf("cursor is %+v, want installed from %s", f, cursorPath)
	}
}
