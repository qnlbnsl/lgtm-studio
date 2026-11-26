package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionPrintsVersion(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	for _, v := range []string{old, "9.8.7-test"} {
		version = v
		var out, errOut bytes.Buffer
		code := run([]string{"version"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("version %q: return code %d, want 0", v, code)
		}
		if out.String() != v+"\n" {
			t.Fatalf("version %q: stdout %q, want %q", v, out.String(), v+"\n")
		}
	}
}

func TestJoinAndRunAreNotBuiltYet(t *testing.T) {
	for _, cmd := range []string{"join", "run"} {
		var out, errOut bytes.Buffer
		code := run([]string{cmd}, &out, &errOut)
		if code != 1 {
			t.Errorf("%s: return code %d, want 1", cmd, code)
		}
		if out.Len() != 0 {
			t.Errorf("%s: stdout %q, want empty", cmd, out.String())
		}
		if !strings.Contains(errOut.String(), cmd) || !strings.Contains(errOut.String(), "not built yet") {
			t.Errorf("%s: stderr %q must name the command and say \"not built yet\"", cmd, errOut.String())
		}
	}
}

func TestUnknownCommandPrintsUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"serve"}} {
		var out, errOut bytes.Buffer
		code := run(args, &out, &errOut)
		if code != 2 {
			t.Errorf("args %v: return code %d, want 2", args, code)
		}
		for _, want := range []string{"join", "run", "version"} {
			if !strings.Contains(errOut.String(), want) {
				t.Errorf("args %v: usage %q does not name %q", args, errOut.String(), want)
			}
		}
	}
}
