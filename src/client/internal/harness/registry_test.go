package harness

import (
	"testing"
	"time"
)

func TestDefinitionsOrderAndPrograms(t *testing.T) {
	defs := Definitions()
	wantNames := []Name{"claude", "codex", "gemini", "cursor"}
	if len(defs) != len(wantNames) {
		t.Fatalf("got %d definitions, want %d", len(defs), len(wantNames))
	}
	for i, n := range wantNames {
		if defs[i].Name != n {
			t.Errorf("definition %d is %q, want %q", i, defs[i].Name, n)
		}
		if defs[i].Version == nil {
			t.Errorf("definition %q has no pattern", n)
		}
	}
	if Claude != "claude" || Codex != "codex" || Gemini != "gemini" || Cursor != "cursor" {
		t.Error("the Name constants have the wrong values")
	}
	programs := map[Name][]string{
		"claude": {"claude"},
		"codex":  {"codex"},
		"gemini": {"gemini"},
		"cursor": {"cursor-agent", "agent"},
	}
	for _, d := range defs {
		want := programs[d.Name]
		if len(d.Programs) != len(want) {
			t.Errorf("%s programs are %v", d.Name, d.Programs)
			continue
		}
		for i := range want {
			if d.Programs[i] != want[i] {
				t.Errorf("%s programs are %v", d.Name, d.Programs)
			}
		}
	}
}

func TestVersionPatterns(t *testing.T) {
	cases := []struct {
		name Name
		line string
		want string // the first group, or empty when the line must not match
	}{
		{Claude, "2.1.325 (Claude Code)", "2.1.325"},
		{Claude, "2.1.325", ""},
		{Claude, "2.1.325 (Claude Code) token=sk-planted", ""},
		{Claude, " 2.1.325 (Claude Code)", ""},
		{Codex, "codex-cli 0.98.0", "0.98.0"},
		{Codex, "codex-cli 0.98.0-alpha.1", "0.98.0-alpha.1"},
		{Codex, "codex-cli 0.98.0 extra", ""},
		{Codex, "warning: key 1.2.3-sk-planted", ""},
		{Gemini, "0.68.0", "0.68.0"},
		{Gemini, "0.68.0-nightly.20261120", "0.68.0-nightly.20261120"},
		{Gemini, "v0.68.0", ""},
		{Gemini, "0.68.0 and more", ""},
		{Cursor, "2026.11.20-abc1234", "2026.11.20-abc1234"},
		{Cursor, "2026.1.5-0123456789ab", "2026.1.5-0123456789ab"},
		{Cursor, "1.4.2", ""},
		{Cursor, "2026.11.20-ABC1234", ""},
		{Cursor, "2026.11.20-abc12", ""},
		{Cursor, "2026.11.20-abc1234567890", ""},
	}
	byName := map[Name]Definition{}
	for _, d := range Definitions() {
		byName[d.Name] = d
	}
	for _, c := range cases {
		d, ok := byName[c.name]
		if !ok || d.Version == nil {
			t.Errorf("no pattern for %s", c.name)
			continue
		}
		m := d.Version.FindStringSubmatch(c.line)
		got := ""
		if len(m) > 1 {
			got = m[1]
		}
		if got != c.want {
			t.Errorf("%s on %q gave %q, want %q", c.name, c.line, got, c.want)
		}
	}
}

func TestDefaultScanOptions(t *testing.T) {
	env := map[string]string{"PATH": "/first/bin:/second/bin", "HOME": "/home/someone"}
	opts := DefaultScanOptions(func(k string) string { return env[k] })
	if len(opts.SearchPath) != 2 || opts.SearchPath[0] != "/first/bin" || opts.SearchPath[1] != "/second/bin" {
		t.Errorf("SearchPath is %v", opts.SearchPath)
	}
	if opts.Home != "/home/someone" {
		t.Errorf("Home is %q", opts.Home)
	}
	if DefaultTimeout != 3*time.Second || WaitDelay != time.Second {
		t.Error("DefaultTimeout or WaitDelay has the wrong value")
	}
}
