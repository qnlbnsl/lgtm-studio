package state

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

var crashSteps = []string{"temp-created", "temp-written", "temp-synced", "placed", "dir-synced"}

func bigData(b byte) []byte { return bytes.Repeat([]byte{b}, 256*1024) }

// TestHelperStateProcess is the child of the crash tests. It returns at once in a normal run.
func TestHelperStateProcess(t *testing.T) {
	mode := os.Getenv("LGTM_TEST_HELPER")
	if mode == "" {
		return
	}
	dir, step := os.Getenv("LGTM_TEST_DIR"), os.Getenv("LGTM_TEST_STEP")
	fail := func(err error) {
		fmt.Println("error:", err)
		os.Exit(3)
	}
	switch mode {
	case "hold":
		if _, err := Open(dir); err != nil {
			fail(err)
		}
		fmt.Println("ready")
		time.Sleep(time.Minute)
		os.Exit(0)
	case "write", "create":
		if mode == "write" {
			if err := WriteAtomic(dir, "x", []byte("A")); err != nil {
				fail(err)
			}
		}
		SetCrashHookForTest(func(s string) {
			if s == step {
				os.Exit(86)
			}
		})
		var err error
		if mode == "write" {
			err = WriteAtomic(dir, "x", bigData('B'))
		} else {
			err = CreateExclusive(dir, "k", bigData('B'))
		}
		if err != nil {
			fail(err)
		}
		os.Exit(0)
	default:
		fail(fmt.Errorf("unknown mode %q", mode))
	}
}

func runCrashChild(t *testing.T, mode, dir, step string) int {
	t.Helper()
	cmd, out := startHelper(t, mode, dir, step)
	text, _ := io.ReadAll(out)
	err := cmd.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if exit.ExitCode() != 86 {
			t.Logf("child output: %s", text)
		}
		return exit.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("child output: %s", text)
	return 0
}

func TestWriteAtomicCrashAtEachStep(t *testing.T) {
	for i, step := range crashSteps {
		placed := i >= 3
		t.Run("write-"+step, func(t *testing.T) {
			dir := newRoot(t)
			if code := runCrashChild(t, "write", dir, step); code != 86 {
				t.Fatalf("child exit code %d, want 86 at step %s", code, step)
			}
			got := readFile(t, filepath.Join(dir, "x"))
			want := []byte("A")
			if placed {
				want = bigData('B')
			}
			if !bytes.Equal(got, want) {
				t.Errorf("after a crash at %s, x holds %d bytes starting %q; want %d bytes starting %q",
					step, len(got), head(got), len(want), head(want))
			}
		})
		t.Run("create-"+step, func(t *testing.T) {
			dir := newRoot(t)
			if code := runCrashChild(t, "create", dir, step); code != 86 {
				t.Fatalf("child exit code %d, want 86 at step %s", code, step)
			}
			got, err := os.ReadFile(filepath.Join(dir, "k"))
			if placed {
				if err != nil || !bytes.Equal(got, bigData('B')) {
					t.Errorf("after a crash at %s, k must be whole: %d bytes, %v", step, len(got), err)
				}
				return
			}
			if !errors.Is(err, os.ErrNotExist) {
				t.Errorf("after a crash at %s, k must be missing: %d bytes, %v", step, len(got), err)
			}
		})
	}
}

func head(b []byte) []byte {
	if len(b) > 8 {
		return b[:8]
	}
	return b
}
