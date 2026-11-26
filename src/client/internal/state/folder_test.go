package state

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// newRoot gives a private, symlink-free folder to build state folders in.
func newRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func perm(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func mustOpen(t *testing.T, dir string) *Folder {
	t.Helper()
	f, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%s): %v", dir, err)
	}
	return f
}

func TestOpenCreatesPrivateFolders(t *testing.T) {
	root := newRoot(t)
	dir := filepath.Join(root, "a", "state")
	f := mustOpen(t, dir)
	if got := perm(t, dir); got != 0o700 {
		t.Errorf("state folder mode %o, want 700", got)
	}
	if got := perm(t, filepath.Join(root, "a")); got != 0o700 {
		t.Errorf("created parent mode %o, want 700", got)
	}
	if got := perm(t, filepath.Join(dir, "lock")); got != 0o600 {
		t.Errorf("lock mode %o, want 600", got)
	}
	if got := f.Paths(); got != PathsIn(dir) {
		t.Errorf("Paths() = %+v, want %+v", got, PathsIn(dir))
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, name := range []string{".tmp-abc", ".tmp-abc.db", "keep.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f = mustOpen(t, dir)
	defer f.Close()
	for _, gone := range []string{".tmp-abc", ".tmp-abc.db"} {
		if _, err := os.Lstat(filepath.Join(dir, gone)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s was not removed by Open: %v", gone, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(dir, "keep.json")); err != nil {
		t.Errorf("keep.json was removed: %v", err)
	}

	want := Paths{
		Dir: "/s", Lock: "/s/lock", Settings: "/s/settings.json", Key: "/s/machine.key",
		Identity: "/s/identity.json", Store: "/s/client.db", Repositories: "/s/repositories",
	}
	if got := PathsIn("/s"); got != want {
		t.Errorf("PathsIn(/s) = %+v, want %+v", got, want)
	}
}

func TestOpenRefusesExposedFolder(t *testing.T) {
	root := newRoot(t)
	for _, mode := range []fs.FileMode{0o750, 0o705} {
		dir := filepath.Join(root, "exposed"+mode.String())
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		f, err := Open(dir)
		if !errors.Is(err, ErrFolderExposed) {
			if err == nil {
				f.Close()
			}
			t.Errorf("mode %o: Open error %v, want ErrFolderExposed", mode, err)
		}
		if got := perm(t, dir); got != mode {
			t.Errorf("mode %o: Open changed the mode to %o", mode, got)
		}
	}
}

func TestOpenRefusesSymlinkAndUnsafeParent(t *testing.T) {
	root := newRoot(t)

	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if f, err := Open(link); !errors.Is(err, ErrNotAFolder) {
		if err == nil {
			f.Close()
		}
		t.Errorf("link: Open error %v, want ErrNotAFolder", err)
	}

	file := filepath.Join(root, "plain")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if f, err := Open(file); !errors.Is(err, ErrNotAFolder) {
		if err == nil {
			f.Close()
		}
		t.Errorf("plain file: Open error %v, want ErrNotAFolder", err)
	}

	parent := filepath.Join(root, "shared")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o777); err != nil {
		t.Fatal(err)
	}
	f, err := Open(filepath.Join(parent, "state"))
	if !errors.Is(err, ErrParentUnsafe) {
		if err == nil {
			f.Close()
		}
		t.Errorf("unsafe parent: Open error %v, want ErrParentUnsafe", err)
	}
	entries, rerr := os.ReadDir(parent)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(entries) != 0 {
		t.Errorf("Open created %d entries in an unsafe parent", len(entries))
	}
}

func TestOpenSyncsWholeChain(t *testing.T) {
	root := newRoot(t)
	a := filepath.Join(root, "a")
	b := filepath.Join(a, "b")
	dir := filepath.Join(b, "state")

	var mu sync.Mutex
	var synced []string
	failOn := a
	restore := SetSyncFolderForTest(func(d string) error {
		mu.Lock()
		defer mu.Unlock()
		synced = append(synced, filepath.Clean(d))
		if failOn != "" && filepath.Clean(d) == failOn {
			return errors.New("injected sync failure")
		}
		return nil
	})
	f, err := Open(dir)
	restore()
	if err == nil {
		f.Close()
		t.Fatal("Open succeeded although the sync of the first folder failed")
	}

	real := syncFolder
	synced = nil
	restore = SetSyncFolderForTest(func(d string) error {
		mu.Lock()
		synced = append(synced, filepath.Clean(d))
		mu.Unlock()
		return real(d)
	})
	defer restore()
	f, err = Open(dir)
	if err != nil {
		t.Fatalf("second Open after a failed sync: %v (the first Open must hold no lock)", err)
	}
	defer f.Close()
	for _, want := range []string{root, a, b, dir} {
		found := false
		for _, got := range synced {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Open did not sync %s; synced: %v", want, synced)
		}
	}
}

func TestOpenRefusesSecondHolder(t *testing.T) {
	root := newRoot(t)
	dir := filepath.Join(root, "state")

	f := mustOpen(t, dir)
	if g, err := Open(dir); !errors.Is(err, ErrFolderLocked) {
		if err == nil {
			g.Close()
		}
		t.Fatalf("second Open in the same process: error %v, want ErrFolderLocked", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	f = mustOpen(t, dir)
	f.Close()

	cmd, stdout := startHelper(t, "hold", dir, "")
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if strings.TrimSpace(line) != "ready" {
		t.Fatalf("the child did not take the folder: %q, %v", line, err)
	}
	if g, err := Open(dir); !errors.Is(err, ErrFolderLocked) {
		if err == nil {
			g.Close()
		}
		t.Fatalf("Open while a child holds the folder: error %v, want ErrFolderLocked", err)
	}
	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for {
		g, err := Open(dir)
		if err == nil {
			g.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Open after the holder was killed: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestClaimAllowsOneStoreEach(t *testing.T) {
	root := newRoot(t)
	dir := filepath.Join(root, "state")
	f := mustOpen(t, dir)

	copyOfPaths := f.Paths()
	copyOfPaths.Store = "/changed"

	paths, release, err := f.Claim("store")
	if err != nil {
		t.Fatalf("first Claim: %v", err)
	}
	if paths != PathsIn(dir) {
		t.Errorf("claim paths %+v, want %+v", paths, PathsIn(dir))
	}
	if _, _, err := f.Claim("store"); !errors.Is(err, ErrStoreOpen) {
		t.Errorf("second Claim of the same name: error %v, want ErrStoreOpen", err)
	}
	if _, rel2, err := f.Claim("other"); err != nil {
		t.Errorf("Claim of another name: %v", err)
	} else {
		rel2()
	}
	if err := f.Close(); !errors.Is(err, ErrStoreOpen) {
		t.Errorf("Close with a claim held: error %v, want ErrStoreOpen", err)
	}
	release()
	_, release, err = f.Claim("store")
	if err != nil {
		t.Fatalf("Claim after release: %v", err)
	}
	release()
	if err := f.Close(); err != nil {
		t.Fatalf("Close after release: %v", err)
	}
	if _, _, err := f.Claim("store"); !errors.Is(err, ErrFolderClosed) {
		t.Errorf("Claim after Close: error %v, want ErrFolderClosed", err)
	}

	var zero Folder
	if _, _, err := zero.Claim("store"); !errors.Is(err, ErrFolderClosed) {
		t.Errorf("Claim on a zero Folder: error %v, want ErrFolderClosed", err)
	}
	var none *Folder
	if _, _, err := none.Claim("store"); !errors.Is(err, ErrFolderClosed) {
		t.Errorf("Claim on a nil Folder: error %v, want ErrFolderClosed", err)
	}

	g := mustOpen(t, dir)
	defer g.Close()
	p, rel, err := g.Claim("store")
	if err != nil {
		t.Fatal(err)
	}
	rel()
	if p.Store != PathsIn(dir).Store {
		t.Errorf("claim path %q was changed through a copy of Paths(), want %q", p.Store, PathsIn(dir).Store)
	}
}

// startHelper runs this test binary again as a child that does the work of one helper mode.
func startHelper(t *testing.T, mode, dir, step string) (*exec.Cmd, interface{ Read([]byte) (int, error) }) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestHelperStateProcess$")
	cmd.Env = append(os.Environ(), "LGTM_TEST_HELPER="+mode, "LGTM_TEST_DIR="+dir, "LGTM_TEST_STEP="+step)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, out
}
