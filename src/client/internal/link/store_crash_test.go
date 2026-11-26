package link

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
)

// runChild runs the test binary again as a helper process in the given mode and returns its output.
func runChild(t *testing.T, mode string, env ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperStoreProcess$")
	cmd.Env = append(os.Environ(), append([]string{"LGTM_TEST_HELPER=" + mode}, env...)...)
	return cmd.CombinedOutput()
}

func startChild(t *testing.T, mode string, env ...string) (*exec.Cmd, *bufio.Scanner) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperStoreProcess$")
	cmd.Env = append(os.Environ(), append([]string{"LGTM_TEST_HELPER=" + mode}, env...)...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, bufio.NewScanner(out)
}

// TestHelperStoreProcess is the body of the child process. It does nothing in a normal run.
func TestHelperStoreProcess(t *testing.T) {
	mode := os.Getenv("LGTM_TEST_HELPER")
	if mode == "" {
		return
	}
	dir := os.Getenv("LGTM_TEST_DIR")
	f, err := state.Open(dir)
	if err != nil {
		fmt.Println("state error:", err)
		os.Exit(3)
	}
	switch mode {
	case "open-check":
		var s *Store
		if os.Getenv("LGTM_TEST_FUNC") == "create" {
			s, err = CreateStore(f, StoreOptions{})
		} else {
			s, err = OpenStore(f, StoreOptions{})
		}
		switch {
		case s != nil && err == nil:
			fmt.Println("result opened")
		case errors.Is(err, ErrStoreDamaged):
			fmt.Println("result damaged")
		default:
			fmt.Println("result other:", err)
		}
		os.Exit(0)
	case "create-crash":
		step := os.Getenv("LGTM_TEST_STEP")
		createHook = func(s string) {
			if s == step {
				os.Exit(86)
			}
		}
		CreateStore(f, StoreOptions{})
		os.Exit(0)
	case "create-or-open":
		s, err := OpenStore(f, StoreOptions{})
		if errors.Is(err, ErrStoreMissing) {
			s, err = CreateStore(f, StoreOptions{})
		}
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(4)
		}
		fmt.Println("next", s.Next())
		s.Close()
		os.Exit(0)
	case "append-loop":
		s, err := OpenStore(f, StoreOptions{})
		if errors.Is(err, ErrStoreMissing) {
			s, err = CreateStore(f, StoreOptions{})
		}
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(4)
		}
		payload := bytes.Repeat([]byte{'p'}, 200)
		for {
			n, err := s.Append(payload)
			if err != nil {
				fmt.Println("error:", err)
				os.Exit(5)
			}
			fmt.Println("n", n)
		}
	case "receipt-loop":
		s, err := OpenStore(f, StoreOptions{})
		if errors.Is(err, ErrStoreMissing) {
			s, err = CreateStore(f, StoreOptions{})
		}
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(4)
		}
		effects, err := os.OpenFile(os.Getenv("LGTM_TEST_EFFECTS"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(6)
		}
		for i := 1; ; i++ {
			c := Command{ID: uid(i), ExpiresAt: tNow.Add(time.Hour)}
			d, _, err := s.Begin(c, tNow)
			if err != nil {
				fmt.Println("error:", err)
				os.Exit(5)
			}
			if d != DecisionRun {
				continue
			}
			fmt.Fprintf(effects, "%d\n", i)
			effects.Sync()
			if err := s.Finish(c.ID, []byte("ok")); err != nil {
				fmt.Println("error:", err)
				os.Exit(5)
			}
			fmt.Println("done", i)
		}
	}
	os.Exit(2)
}

func TestOpenStoreCrashDuringCreation(t *testing.T) {
	steps := []string{"temp-created", "temp-initialized", "placed", "dir-synced"}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			f, dir := newState(t)
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			out, err := runChild(t, "create-crash", "LGTM_TEST_DIR="+dir, "LGTM_TEST_STEP="+step)
			var ee *exec.ExitError
			if !errors.As(err, &ee) || ee.ExitCode() != 86 {
				t.Fatalf("the child did not stop at %s: %v %q", step, err, out)
			}
			f, err = state.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { f.Close() })
			placed := step == "placed" || step == "dir-synced"
			s, err := OpenStore(f, StoreOptions{})
			if placed {
				if err != nil || s == nil || s.Next() != 1 || s.Len() != 0 {
					t.Fatalf("OpenStore after %s gave %v, %v", step, s, err)
				}
				s.Close()
			} else {
				if s != nil || !errors.Is(err, ErrStoreMissing) {
					t.Fatalf("OpenStore after %s gave %v, %v", step, s, err)
				}
				if _, err := os.Lstat(f.Paths().Store); !os.IsNotExist(err) {
					t.Fatalf("OpenStore made the file: %v", err)
				}
				s, err = CreateStore(f, StoreOptions{})
				if err != nil || s == nil || s.Next() != 1 {
					t.Fatalf("CreateStore after %s gave %v, %v", step, s, err)
				}
				s.Close()
			}
			// A temporary name left by the crash is never taken for the store.
			s = mustOpen(t, f, StoreOptions{})
			if n, err := s.Append([]byte("x")); err != nil || n != 1 {
				t.Fatalf("Append gave %d, %v", n, err)
			}
		})
	}
}

// killChild reads numbers from the child's output until it has seen at least min of them, then kills it at once.
func killAfter(t *testing.T, cmd *exec.Cmd, sc *bufio.Scanner, prefix string, count int) (last uint64, seen int) {
	t.Helper()
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "error") {
			t.Fatalf("the child reported %s", line)
		}
		v, ok := strings.CutPrefix(line, prefix+" ")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			continue
		}
		last, seen = n, seen+1
		if seen >= count {
			break
		}
	}
	cmd.Process.Signal(syscall.SIGKILL)
	// Collect what was printed before the signal arrived.
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), prefix+" "); ok {
			if n, err := strconv.ParseUint(v, 10, 64); err == nil {
				last, seen = n, seen+1
			}
		}
	}
	cmd.Wait()
	return last, seen
}

func TestStoreKillKeepsEveryNumberedEvent(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	t.Logf("seed %d", seed)
	rng := rand.New(rand.NewPCG(seed, 5))
	f, dir := newState(t)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var printed uint64
	total := 0
	for round := 0; round < 30; round++ {
		cmd, sc := startChild(t, "append-loop", "LGTM_TEST_DIR="+dir)
		last, seen := killAfter(t, cmd, sc, "n", 1+rng.IntN(40))
		total += seen
		if last > printed {
			printed = last
		}
		f, err := state.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		s, err := OpenStore(f, StoreOptions{})
		if err != nil || s == nil {
			t.Fatalf("round %d: OpenStore after a kill gave %v", round, err)
		}
		events, err := s.Read(0, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if uint64(len(events)) < printed {
			t.Fatalf("round %d: %d events stored but the child printed number %d", round, len(events), printed)
		}
		for i, e := range events {
			if e.Seq != uint64(i+1) {
				t.Fatalf("round %d: event %d has number %d", round, i, e.Seq)
			}
		}
		if s.Next() != uint64(len(events))+1 {
			t.Fatalf("round %d: next is %d with %d events", round, s.Next(), len(events))
		}
		printed = uint64(len(events))
		s.Close()
		f.Close()
	}
	if total < 100 {
		t.Fatalf("the children printed only %d numbers, so the test proves nothing", total)
	}
}

func TestStoreKeepsOrderAndOnceAcrossRestart(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	t.Logf("seed %d", seed)
	rng := rand.New(rand.NewPCG(seed, 9))
	f, dir := newState(t)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var serverLast uint64 // the highest number the parent, as the server, has taken and acknowledged
	for round := 0; round < 20; round++ {
		cmd, sc := startChild(t, "append-loop", "LGTM_TEST_DIR="+dir)
		killAfter(t, cmd, sc, "n", 5+rng.IntN(30))
		f, err := state.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		s, err := OpenStore(f, StoreOptions{})
		if err != nil || s == nil {
			t.Fatalf("round %d: %v", round, err)
		}
		events, err := s.Read(serverLast+1, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		for i, e := range events {
			if want := serverLast + uint64(i) + 1; e.Seq != want {
				t.Fatalf("round %d: the server expected %d and got %d", round, want, e.Seq)
			}
		}
		if len(events) > 0 {
			take := 1 + rng.IntN(len(events))
			serverLast = events[take-1].Seq
			if err := s.Ack(serverLast); err != nil {
				t.Fatal(err)
			}
		}
		if s.Next() <= serverLast {
			t.Fatalf("round %d: next %d is not above the acknowledged %d", round, s.Next(), serverLast)
		}
		s.Close()
		f.Close()
	}
	if serverLast == 0 {
		t.Fatal("the server never took an event")
	}
}

func TestReceiptsKillNeverRunsTwice(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	t.Logf("seed %d", seed)
	rng := rand.New(rand.NewPCG(seed, 13))
	f, dir := newState(t)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	effects := dir + "-effects"
	if err := os.WriteFile(effects, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(effects) })
	total := 0
	for round := 0; round < 30; round++ {
		// The child counts from 1 each time, so the ids that it already ran come round again.
		cmd, sc := startChild(t, "receipt-loop", "LGTM_TEST_DIR="+dir, "LGTM_TEST_EFFECTS="+effects)
		_, seen := killAfter(t, cmd, sc, "done", 1+rng.IntN(20))
		total += seen
	}
	data, err := os.ReadFile(effects)
	if err != nil {
		t.Fatal(err)
	}
	ran := map[string]int{}
	for _, l := range strings.Fields(string(data)) {
		ran[l]++
		if ran[l] > 1 {
			t.Fatalf("command %s ran %d times", l, ran[l])
		}
	}
	if total < 30 || len(ran) == 0 {
		t.Fatalf("the children finished only %d commands, so the test proves nothing", total)
	}
}
