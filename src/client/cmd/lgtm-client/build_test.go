package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test folder")
		}
		dir = parent
	}
}

func TestBuildsForLinuxAndMacOS(t *testing.T) {
	if testing.Short() {
		t.Skip("cross builds are skipped under -short")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("the go tool is not on the PATH: %v", err)
	}
	root := moduleRoot(t)
	for _, goos := range []string{"linux", "darwin"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			t.Run(goos+"-"+goarch, func(t *testing.T) {
				env := append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
				build := func(args ...string) {
					cmd := exec.Command(goTool, args...)
					cmd.Dir = root
					cmd.Env = env
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
					}
				}
				build("build", "./src/client/...")
				program := filepath.Join(t.TempDir(), "lgtm-client")
				build("build", "-o", program, "./src/client/cmd/lgtm-client")
				if _, err := os.Stat(program); err != nil {
					t.Fatalf("the program file is missing: %v", err)
				}
				if goos == runtime.GOOS && goarch == runtime.GOARCH {
					out, err := exec.Command(program, "version").Output()
					if err != nil {
						t.Fatalf("running the built program: %v", err)
					}
					if strings.TrimSpace(string(out)) != "0.0.0-dev" {
						t.Fatalf("built program printed %q, want 0.0.0-dev", out)
					}
				}
			})
		}
	}
}
