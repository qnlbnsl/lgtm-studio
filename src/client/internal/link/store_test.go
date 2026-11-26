package link

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
)

// The test time is far in the future, so that the real clock never prunes a receipt by accident.
var tNow = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)

// uid returns the n-th lowercase identifier in canonical form.
func uid(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }

func newState(t *testing.T) (*state.Folder, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "state")
	f, err := state.Open(dir)
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f, f.Paths().Dir
}

func mustCreate(t *testing.T, f *state.Folder, opts StoreOptions) *Store {
	t.Helper()
	s, err := CreateStore(f, opts)
	if err != nil || s == nil {
		t.Fatalf("CreateStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustOpen(t *testing.T, f *state.Folder, opts StoreOptions) *Store {
	t.Helper()
	s, err := OpenStore(f, opts)
	if err != nil || s == nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// reopen closes the store and opens the same file again with OpenStore.
func reopen(t *testing.T, f *state.Folder, s *Store, opts StoreOptions) *Store {
	t.Helper()
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return mustOpen(t, f, opts)
}

func swapCommit(fn func(db *bolt.DB, f func(*bolt.Tx) error) error) (restore func()) {
	old := commit
	commit = fn
	return func() { commit = old }
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	slices.Sort(out)
	return out
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func be(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// fixture describes a store file that the tests write with the library directly.
type fixture struct {
	version        string
	next           []byte
	keys           []uint64
	receipts       map[string]string
	noFreelistSync bool
	payload        int
}

func goodFixture(next uint64, keys ...uint64) fixture {
	return fixture{version: "1", next: be(next), keys: keys, payload: 5}
}

func writeFixture(t *testing.T, path string, fx fixture) {
	t.Helper()
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second, NoFreelistSync: fx.noFreelistSync})
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		meta, err := tx.CreateBucket([]byte("meta"))
		if err != nil {
			return err
		}
		events, err := tx.CreateBucket([]byte("events"))
		if err != nil {
			return err
		}
		receipts, err := tx.CreateBucket([]byte("receipts"))
		if err != nil {
			return err
		}
		if err := meta.Put([]byte("version"), []byte(fx.version)); err != nil {
			return err
		}
		if err := meta.Put([]byte("next"), fx.next); err != nil {
			return err
		}
		for _, k := range fx.keys {
			if err := events.Put(be(k), bytes.Repeat([]byte{'x'}, fx.payload)); err != nil {
				return err
			}
		}
		for id, v := range fx.receipts {
			if err := receipts.Put([]byte(id), []byte(v)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

// editStore opens the closed store file with the library and lets fn change it.
func editStore(t *testing.T, path string, fn func(tx *bolt.Tx) error) {
	t.Helper()
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(fn); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func setNext(t *testing.T, path string, n uint64) {
	t.Helper()
	editStore(t, path, func(tx *bolt.Tx) error { return tx.Bucket([]byte("meta")).Put([]byte("next"), be(n)) })
}

func putReceipts(t *testing.T, path string, receipts map[string]string) {
	t.Helper()
	editStore(t, path, func(tx *bolt.Tx) error {
		for id, v := range receipts {
			if err := tx.Bucket([]byte("receipts")).Put([]byte(id), []byte(v)); err != nil {
				return err
			}
		}
		return nil
	})
}

func readReceipts(t *testing.T, path string) map[string]string {
	t.Helper()
	out := map[string]string{}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("receipts")).ForEach(func(k, v []byte) error {
			out[string(k)] = string(v)
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestOpenStoreMissingAfterJoin(t *testing.T) {
	f, dir := newState(t)
	before := names(t, dir)
	s, err := OpenStore(f, StoreOptions{})
	if s != nil || !errors.Is(err, ErrStoreMissing) {
		t.Fatalf("OpenStore on a folder without a store gave %v, %v", s, err)
	}
	if after := names(t, dir); !slices.Equal(before, after) {
		t.Fatalf("OpenStore changed the folder: %v became %v", before, after)
	}

	s = mustCreate(t, f, StoreOptions{})
	for _, p := range []string{"a", "b"} {
		if _, err := s.Append([]byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := CreateStore(f, StoreOptions{})
	if err != nil || again == nil {
		t.Fatalf("CreateStore on a valid store: %v", err)
	}
	defer again.Close()
	if again.Next() != 3 || again.Len() != 2 {
		t.Fatalf("CreateStore replaced the store: next %d, length %d", again.Next(), again.Len())
	}
}

func TestOpenStoreCreatesSyncedPrivateFile(t *testing.T) {
	f, dir := newState(t)
	var mu sync.Mutex
	var synced []string
	failSync := false
	restore := state.SetSyncFolderForTest(func(d string) error {
		mu.Lock()
		defer mu.Unlock()
		synced = append(synced, d)
		if failSync {
			return errors.New("sync failed")
		}
		return nil
	})
	t.Cleanup(restore)
	syncsOfFolder := func() int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, d := range synced {
			if d == dir {
				n++
			}
		}
		return n
	}

	s := mustCreate(t, f, StoreOptions{})
	info, err := os.Lstat(f.Paths().Store)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 || !info.Mode().IsRegular() {
		t.Errorf("client.db has mode %v", info.Mode())
	}
	if s.Next() != 1 {
		t.Errorf("Next is %d", s.Next())
	}
	if s.noSync() {
		t.Error("the store is opened with NoSync")
	}
	for _, n := range names(t, dir) {
		if len(n) >= len(state.TempPrefix) && n[:len(state.TempPrefix)] == state.TempPrefix {
			t.Errorf("temporary name %s is left", n)
		}
	}
	if syncsOfFolder() == 0 {
		t.Error("CreateStore did not sync the state folder")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	synced = nil
	mu.Unlock()
	s = mustOpen(t, f, StoreOptions{})
	if syncsOfFolder() == 0 {
		t.Error("OpenStore did not sync the state folder")
	}
	s.Close()

	// A failing sync gives an error and no store, and the claim is let go.
	f2, _ := newState(t)
	mu.Lock()
	failSync = true
	mu.Unlock()
	if s, err := OpenStore(f, StoreOptions{}); err == nil || s != nil {
		t.Errorf("OpenStore with a failing sync gave %v, %v", s, err)
	}
	if s, err := CreateStore(f2, StoreOptions{}); err == nil || s != nil {
		t.Errorf("CreateStore with a failing sync gave %v, %v", s, err)
	}
	mu.Lock()
	failSync = false
	mu.Unlock()
	mustOpen(t, f, StoreOptions{}).Close()
	s2 := mustCreate(t, f2, StoreOptions{})
	if s2.Next() != 1 {
		t.Errorf("Next after the failed attempt is %d", s2.Next())
	}
}

func TestOpenStoreRefusesUnsafeFile(t *testing.T) {
	f, dir := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	if _, err := s.Append([]byte("x")); err != nil {
		t.Fatal(err)
	}
	s.Close()
	store := f.Paths().Store
	real := filepath.Join(dir, "real.db")
	if err := os.Rename(store, real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, store); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(*state.Folder, StoreOptions) (*Store, error){"OpenStore": OpenStore, "CreateStore": CreateStore} {
		if s, err := fn(f, StoreOptions{}); s != nil || !errors.Is(err, ErrStoreUnsafe) {
			t.Errorf("%s on a link gave %v, %v", name, s, err)
		}
	}
	if err := os.Remove(store); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(real, store); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(store, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(*state.Folder, StoreOptions) (*Store, error){"OpenStore": OpenStore, "CreateStore": CreateStore} {
		if s, err := fn(f, StoreOptions{}); s != nil || !errors.Is(err, ErrStoreUnsafe) {
			t.Errorf("%s on mode 0644 gave %v, %v", name, s, err)
		}
	}
	if info, _ := os.Stat(store); info.Mode().Perm() != 0o644 {
		t.Errorf("the refused file's mode was changed to %v", info.Mode().Perm())
	}
	if err := os.Chmod(store, 0o600); err != nil {
		t.Fatal(err)
	}

	first := mustOpen(t, f, StoreOptions{})
	if s, err := OpenStore(f, StoreOptions{}); s != nil || !errors.Is(err, state.ErrStoreOpen) {
		t.Errorf("a second OpenStore gave %v, %v", s, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	mustOpen(t, f, StoreOptions{}).Close()
}

func TestOpenStoreRefusesDamagedFile(t *testing.T) {
	random := make([]byte, 64*1024)
	rng := rand.New(rand.NewPCG(7, 11))
	for i := range random {
		random[i] = byte(rng.IntN(256))
	}
	cases := []struct {
		name  string
		build func(t *testing.T, path string)
	}{
		{"empty file", func(t *testing.T, p string) { os.WriteFile(p, nil, 0o600) }},
		{"random bytes", func(t *testing.T, p string) { os.WriteFile(p, random, 0o600) }},
		{"version 2", func(t *testing.T, p string) {
			fx := goodFixture(2, 1)
			fx.version = "2"
			writeFixture(t, p, fx)
		}},
		{"keys 1 and 3", func(t *testing.T, p string) { writeFixture(t, p, goodFixture(4, 1, 3)) }},
		{"last key equals next", func(t *testing.T, p string) { writeFixture(t, p, goodFixture(2, 1, 2)) }},
		{"keys 3 and 4 with next 6", func(t *testing.T, p string) { writeFixture(t, p, goodFixture(6, 3, 4)) }},
		{"key 0", func(t *testing.T, p string) { writeFixture(t, p, goodFixture(2, 0)) }},
		{"next 0", func(t *testing.T, p string) { writeFixture(t, p, goodFixture(0)) }},
		{"next of 4 bytes", func(t *testing.T, p string) {
			fx := goodFixture(2, 1)
			fx.next = []byte{0, 0, 0, 2}
			writeFixture(t, p, fx)
		}},
		{"version 2 without a stored freelist", func(t *testing.T, p string) {
			fx := goodFixture(3, 1, 2)
			fx.version = "2"
			fx.noFreelistSync = true
			writeFixture(t, p, fx)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, dir := newState(t)
			store := f.Paths().Store
			c.build(t, store)
			if err := os.Chmod(store, 0o600); err != nil {
				t.Fatal(err)
			}
			bytesBefore := readBytes(t, store)
			namesBefore := names(t, dir)
			for round := 0; round < 2; round++ {
				for name, fn := range map[string]func(*state.Folder, StoreOptions) (*Store, error){"OpenStore": OpenStore, "CreateStore": CreateStore} {
					s, err := fn(f, StoreOptions{})
					if s != nil || !errors.Is(err, ErrStoreDamaged) {
						t.Fatalf("%s round %d gave %v, %v", name, round, s, err)
					}
					if errors.Is(err, state.ErrStoreOpen) {
						t.Fatalf("%s left the claim held: %v", name, err)
					}
					if !bytes.Equal(readBytes(t, store), bytesBefore) {
						t.Fatalf("%s changed the file", name)
					}
					if after := names(t, dir); !slices.Equal(after, namesBefore) {
						t.Fatalf("%s changed the folder: %v became %v", name, namesBefore, after)
					}
				}
			}
		})
	}

	// A store cut in half makes the library fault when it reads a page beyond the end,
	// so the check runs in a child process and a crash fails only this case.
	for _, fn := range []string{"open", "create"} {
		t.Run("cut in half, "+fn, func(t *testing.T) {
			f, dir := newState(t)
			store := f.Paths().Store
			fx := goodFixture(301, seqRange(1, 300)...)
			fx.payload = 1000
			writeFixture(t, store, fx)
			info, err := os.Stat(store)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(store, info.Size()/2); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			before := readBytes(t, store)
			out, err := runChild(t, "open-check", "LGTM_TEST_DIR="+dir, "LGTM_TEST_FUNC="+fn)
			if err != nil || !bytes.Contains(out, []byte("result damaged")) {
				t.Fatalf("the check on a cut store gave %v and %q", err, out)
			}
			if !bytes.Equal(readBytes(t, store), before) {
				t.Error("the check changed the file")
			}
		})
	}
}

func seqRange(from, to uint64) []uint64 {
	var out []uint64
	for n := from; n <= to; n++ {
		out = append(out, n)
	}
	return out
}

func TestOpenStoreWritesNothingToExistingFile(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	for _, p := range []string{"one", "two", "three"} {
		if _, err := s.Append([]byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	if d, _, err := s.Begin(Command{ID: uid(1), ExpiresAt: tNow.Add(time.Hour)}, tNow); err != nil || d != DecisionRun {
		t.Fatalf("Begin gave %v, %v", d, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before := readBytes(t, f.Paths().Store)
	for i := 0; i < 2; i++ {
		s := mustOpen(t, f, StoreOptions{})
		if s.Len() != 3 || s.Next() != 4 {
			t.Fatalf("open %d sees length %d and next %d", i, s.Len(), s.Next())
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(readBytes(t, f.Paths().Store), before) {
			t.Fatalf("open %d changed the file", i)
		}
	}
}

func TestCommitErrorFailsStore(t *testing.T) {
	// A commit that reaches the disk and then reports an error.
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
		if err := db.Update(fn); err != nil {
			return err
		}
		return errors.New("the commit failed after writing")
	})
	_, err := s.Append([]byte("A"))
	restore()
	if !errors.Is(err, ErrStoreFailed) {
		t.Fatalf("the failed Append gave %v", err)
	}
	if _, err := s.Append([]byte("B")); !errors.Is(err, ErrStoreFailed) {
		t.Fatalf("the next Append gave %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after a failed commit gave %v", err)
	}
	s = mustOpen(t, f, StoreOptions{})
	events, err := s.Read(0, 10)
	if err != nil || len(events) != 1 || events[0].Seq != 1 || string(events[0].Payload) != "A" {
		t.Fatalf("after the reopen the events are %v, %v", events, err)
	}
	if n, err := s.Append([]byte("C")); err != nil || n != 2 {
		t.Fatalf("the next number is %d, %v", n, err)
	}

	// A commit that never reaches the disk.
	f2, _ := newState(t)
	s2 := mustCreate(t, f2, StoreOptions{})
	restore = swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
		return errors.New("the commit failed before writing")
	})
	_, err = s2.Append([]byte("A"))
	restore()
	if !errors.Is(err, ErrStoreFailed) {
		t.Fatalf("the failed Append gave %v", err)
	}
	s2 = reopen(t, f2, s2, StoreOptions{})
	if s2.Len() != 0 || s2.Next() != 1 {
		t.Fatalf("after the reopen the length is %d and next is %d", s2.Len(), s2.Next())
	}
	if n, err := s2.Append([]byte("B")); err != nil || n != 1 {
		t.Fatalf("the first number is %d, %v", n, err)
	}
}

func TestFailureStopsWaitingWriter(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	var calls atomic.Int32
	restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		return errors.New("the disk failed")
	})
	defer restore()

	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { _, err := s.Append([]byte("A")); first <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the first Append never reached commit")
	}
	go func() { _, err := s.Append([]byte("B")); second <- err }()
	time.Sleep(200 * time.Millisecond)
	close(release)

	for name, ch := range map[string]chan error{"first": first, "second": second} {
		select {
		case err := <-ch:
			if !errors.Is(err, ErrStoreFailed) {
				t.Errorf("the %s Append gave %v", name, err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("the %s Append never returned", name)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("commit was entered %d times", n)
	}
}

func TestCloseKeepsSnapshot(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	for _, p := range []string{"aa", "bbb", "c"} {
		if _, err := s.Append([]byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	changed := s.Changed()
drain:
	for {
		select {
		case <-changed:
		default:
			break drain
		}
	}
	returned := make(chan struct{})
	go func() {
		<-s.Changed()
		close(returned)
	}()
	select {
	case <-returned:
		t.Fatal("Changed signalled before any change or close")
	case <-time.After(100 * time.Millisecond):
	}
	next, length, size := s.Next(), s.Len(), s.Bytes()
	if next != 4 || length != 3 || size != 6 {
		t.Fatalf("before Close: next %d, length %d, bytes %d", next, length, size)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("a goroutine waiting on Changed did not return after Close")
	}
	if s.Next() != next || s.Len() != length || s.Bytes() != size {
		t.Errorf("after Close: next %d, length %d, bytes %d", s.Next(), s.Len(), s.Bytes())
	}
	if _, err := s.Append([]byte("x")); !errors.Is(err, ErrStoreClosed) {
		t.Errorf("Append after Close gave %v", err)
	}
	if _, err := s.Read(0, 1); !errors.Is(err, ErrStoreClosed) {
		t.Errorf("Read after Close gave %v", err)
	}
	if err := s.Ack(1); !errors.Is(err, ErrStoreClosed) {
		t.Errorf("Ack after Close gave %v", err)
	}
	if _, _, err := s.Begin(Command{ID: uid(1), ExpiresAt: tNow.Add(time.Hour)}, tNow); !errors.Is(err, ErrStoreClosed) {
		t.Errorf("Begin after Close gave %v", err)
	}
	if err := s.Finish(uid(1), nil); !errors.Is(err, ErrStoreClosed) {
		t.Errorf("Finish after Close gave %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("the second Close gave %v", err)
	}

	// Close works in the failed state too.
	f2, _ := newState(t)
	s2 := mustCreate(t, f2, StoreOptions{})
	restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error { return errors.New("failed") })
	s2.Append([]byte("x"))
	restore()
	if err := s2.Close(); err != nil {
		t.Errorf("Close after a failed commit gave %v", err)
	}
}
