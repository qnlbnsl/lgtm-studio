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

func TestJoinAndRunRequireInputAndEnrollment(t *testing.T) {
	t.Setenv("LGTM_CLIENT_STATE_DIR", commandStateDir(t))
	for _, tc := range []struct {
		cmd        string
		wantStatus int
		wantText   string
	}{
		{"join", 2, "Usage:"},
		{"run", 1, "join"},
	} {
		var out, errOut bytes.Buffer
		code := run([]string{tc.cmd}, &out, &errOut)
		if code != tc.wantStatus {
			t.Errorf("%s: return code %d, want %d", tc.cmd, code, tc.wantStatus)
		}
		if out.Len() != 0 {
			t.Errorf("%s: stdout must be empty", tc.cmd)
		}
		if !strings.Contains(strings.ToLower(errOut.String()), strings.ToLower(tc.wantText)) {
			t.Errorf("%s: stderr did not include the required instruction", tc.cmd)
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
