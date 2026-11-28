//go:build linux

package link_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
)

const (
	storeReadChildMode = "LGTM_TEST_STORE_READ_MODE"
	storeReadClient    = "LGTM_TEST_STORE_READ_CLIENT"
)

func TestRevisionTwoStoreMetadataRead(t *testing.T) {
	if mode := os.Getenv(storeReadChildMode); mode != "" {
		runStoreReadFaultChild(t, mode)
		return
	}

	t.Run("temporary read fault", func(t *testing.T) {
		home := t.TempDir()
		dir := filepath.Join(home, "state")
		path := seedStoreReadFixture(t, dir)
		before := storeReadBytes(t, path)
		requireStoreReadOpen(t, dir, nil)

		binary := filepath.Join(home, "lgtm-client")
		buildCtx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		build := exec.CommandContext(buildCtx, "go", "build", "-buildvcs=false", "-o", binary, "../../cmd/lgtm-client")
		// Use the installer's static build so the loader does not need positional reads.
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("could not build the client command: %v\n%s", err, output)
		}

		runFault := func(mode string) (stdout, stderr *bytes.Buffer, err error) {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRevisionTwoStoreMetadataRead$")
			cmd.Env = []string{"HOME=" + home, "LGTM_CLIENT_STATE_DIR=" + dir, storeReadChildMode + "=" + mode, storeReadClient + "=" + binary}
			stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
			cmd.Stdout, cmd.Stderr = stdout, stderr
			err = cmd.Run()
			if ctx.Err() != nil {
				t.Fatal("the child did not stop after the metadata read fault")
			}
			return stdout, stderr, err
		}
		stdout, stderr, err := runFault("open")
		if err != nil {
			t.Errorf("opening the store under the metadata read fault failed its check: %v\n%s%s", err, stdout.Bytes(), stderr.Bytes())
		}
		stdout, stderr, err = runFault("run")
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Errorf("the client command returned %v, want exit status 1", err)
		}
		if stdout.Len() != 0 {
			t.Errorf("the faulted client wrote to standard output: %s", stdout.Bytes())
		}
		checkStoreReadDiagnostic(t, stderr)

		if !bytes.Equal(before, storeReadBytes(t, path)) {
			t.Error("the metadata read fault changed the database bytes")
		}
		requireStoreReadOpen(t, dir, nil)
	})

	t.Run("readable invalid metadata", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "state")
		path := seedStoreReadFixture(t, dir)
		data := storeReadBytes(t, path)
		clear(data[:2*os.Getpagesize()])
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal("could not invalidate the two metadata pages")
		}
		if !bytes.Equal(data, storeReadBytes(t, path)) {
			t.Fatal("the invalid metadata fixture was not readable as written")
		}
		requireStoreReadOpen(t, dir, link.ErrStoreDamaged)
	})
}

func runStoreReadFaultChild(t *testing.T, mode string) {
	t.Helper()
	dir := os.Getenv("LGTM_CLIENT_STATE_DIR")
	// The child exits after the check because a syscall filter cannot be removed.
	runtime.LockOSThread()
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		t.Fatalf("could not set up the metadata read fault: %v", err)
	}
	filter := []unix.SockFilter{
		// The first word of seccomp_data is the syscall number.
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: unix.SYS_PREAD64, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EIO)},
		{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
	}
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	thread, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_SET_MODE_FILTER, unix.SECCOMP_FILTER_FLAG_TSYNC, uintptr(unsafe.Pointer(&program)))
	if errno != 0 || thread != 0 {
		t.Fatalf("could not inject the read fault across the child threads: %v, thread %d", errno, thread)
	}

	file, err := os.Open(state.PathsIn(dir).Store)
	if err != nil {
		t.Fatalf("the fault fixture could not open the database descriptor: %v", err)
	}
	var page [4096]byte
	n, err := file.ReadAt(page[:], 0)
	if closeErr := file.Close(); n != 0 || !errors.Is(err, unix.EIO) || closeErr != nil {
		t.Fatalf("the metadata control read returned %d bytes and %v, want an input/output error", n, err)
	}

	if mode == "open" {
		requireStoreReadOpen(t, dir, link.ErrStoreFailed)
		return
	}
	binary := os.Getenv(storeReadClient)
	if err := unix.Exec(binary, []string{binary, "run"}, os.Environ()); err != nil {
		t.Fatalf("could not execute the client with the metadata read fault: %v", err)
	}
}

func seedStoreReadFixture(t *testing.T, dir string) string {
	t.Helper()
	if err := os.Chmod(filepath.Dir(dir), 0o700); err != nil {
		t.Fatalf("could not make the fixture home private: %v", err)
	}
	folder, err := state.Open(dir)
	if err != nil {
		t.Fatalf("could not prepare the fixture state: %v", err)
	}
	t.Cleanup(func() { _ = folder.Close() })
	paths := folder.Paths()
	credential, err := enroll.Create(paths.Key)
	if err != nil {
		t.Fatal("could not create the fixture key")
	}
	serverKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("could not create the fixture server key")
	}
	if err := enroll.SaveServer(folder, enroll.Server{
		Address: "http://127.0.0.1:1", MachineID: "0195977c-0000-7000-8000-000000000813",
		ClientPublicKey: credential.PublicKey(), PublicKey: serverKey,
	}); err != nil {
		t.Fatalf("could not save the fixture enrollment: %v", err)
	}
	store, err := link.CreateStore(folder, link.StoreOptions{})
	if err != nil {
		t.Fatalf("could not create the fixture store: %v", err)
	}
	if err := errors.Join(store.Close(), folder.Close()); err != nil {
		t.Fatalf("could not close the fixture state: %v", err)
	}
	return paths.Store
}

func requireStoreReadOpen(t *testing.T, dir string, want error) {
	t.Helper()
	folder, err := state.Open(dir)
	if err != nil {
		t.Fatalf("could not reopen the fixture state: %v", err)
	}
	defer folder.Close()
	store, err := link.OpenStore(folder, link.StoreOptions{})
	if store != nil {
		if err := store.Close(); err != nil {
			t.Errorf("could not close the reopened store: %v", err)
		}
	}
	if (store == nil) != (want != nil) || !errors.Is(err, want) ||
		(want == link.ErrStoreFailed && errors.Is(err, link.ErrStoreDamaged)) {
		t.Errorf("OpenStore returned a store = %t and error %v, want error %v", store != nil, err, want)
	}
}

func storeReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read the fixture database: %v", err)
	}
	return data
}

func checkStoreReadDiagnostic(t *testing.T, stderr *bytes.Buffer) {
	t.Helper()
	decoder := json.NewDecoder(stderr)
	var failures int
	for decoder.More() {
		var line struct {
			Level   string `json:"level"`
			Msg     string `json:"msg"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := decoder.Decode(&line); err != nil {
			t.Fatalf("could not decode the client diagnostic: %v", err)
		}
		message := strings.ToLower(line.Message)
		if strings.Contains(message, "restor") || strings.Contains(message, "recover") || strings.Contains(message, "enroll") {
			t.Error("the client gave a history recovery or re-enrollment instruction for a metadata read fault")
		}
		if line.Level == "ERROR" && line.Msg == "link_stopped" && line.Code == "store_failed" && line.Message == "Could not use client.db." {
			failures++
		}
	}
	if failures != 1 {
		t.Errorf("the client logged %d plain store failures, want one", failures)
	}
}
