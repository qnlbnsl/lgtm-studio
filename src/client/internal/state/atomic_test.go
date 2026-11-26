package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func noTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), TempPrefix) {
			t.Errorf("temporary file %s was left behind", e.Name())
		}
	}
}

func TestWriteAtomicWritesPrivateFile(t *testing.T) {
	dir := newRoot(t)
	path := filepath.Join(dir, "f")
	if err := WriteAtomic(dir, "f", []byte("A")); err != nil {
		t.Fatalf("WriteAtomic A: %v", err)
	}
	if got := readFile(t, path); string(got) != "A" {
		t.Fatalf("file holds %q, want A", got)
	}
	if got := perm(t, path); got != 0o600 {
		t.Errorf("mode %o, want 600", got)
	}
	if err := WriteAtomic(dir, "f", []byte("B")); err != nil {
		t.Fatalf("WriteAtomic B: %v", err)
	}
	if got := readFile(t, path); string(got) != "B" {
		t.Fatalf("file holds %q, want B", got)
	}
	noTemps(t, dir)

	for _, name := range []string{"", ".", "..", "a/b", ".tmp-x"} {
		if err := WriteAtomic(dir, name, []byte("x")); !errors.Is(err, ErrBadName) {
			t.Errorf("WriteAtomic name %q: error %v, want ErrBadName", name, err)
		}
		if err := CreateExclusive(dir, name, []byte("x")); !errors.Is(err, ErrBadName) {
			t.Errorf("CreateExclusive name %q: error %v, want ErrBadName", name, err)
		}
	}
}

// recordSteps records, in order, the steps of a write and the syncs between them.
func recordSteps(t *testing.T) *[]string {
	t.Helper()
	var events []string
	oldHook := crashHook
	crashHook = func(step string) { events = append(events, step) }
	realFile, realFolder := syncFile, syncFolder
	syncFile = func(f *os.File) error {
		events = append(events, "fsync-file")
		return realFile(f)
	}
	restoreFolder := SetSyncFolderForTest(func(d string) error {
		events = append(events, "fsync-dir")
		return realFolder(d)
	})
	t.Cleanup(func() {
		crashHook = oldHook
		syncFile = realFile
		restoreFolder()
	})
	return &events
}

func TestWriteAtomicSyncsBeforeRename(t *testing.T) {
	want := []string{"temp-created", "temp-written", "fsync-file", "temp-synced", "placed", "fsync-dir", "dir-synced"}

	dir := newRoot(t)
	events := recordSteps(t)
	if err := WriteAtomic(dir, "f", []byte("data")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	if !reflect.DeepEqual(*events, want) {
		t.Errorf("WriteAtomic order %v, want %v", *events, want)
	}

	*events = nil
	if err := CreateExclusive(dir, "k", []byte("data")); err != nil {
		t.Fatalf("CreateExclusive: %v", err)
	}
	if !reflect.DeepEqual(*events, want) {
		t.Errorf("CreateExclusive order %v, want %v", *events, want)
	}
}

func TestWriteReportsPlacedNotSynced(t *testing.T) {
	dir := newRoot(t)
	if err := WriteAtomic(dir, "x", []byte("A")); err != nil {
		t.Fatalf("setup: %v", err)
	}

	restore := SetSyncFolderForTest(func(string) error { return errors.New("injected folder sync failure") })
	err := WriteAtomic(dir, "x", []byte("B"))
	if !errors.Is(err, ErrPlacedNotSynced) {
		t.Errorf("WriteAtomic with a failing folder sync: error %v, want ErrPlacedNotSynced", err)
	}
	if got := readFile(t, filepath.Join(dir, "x")); string(got) != "B" {
		t.Errorf("x holds %q, want B (it was placed)", got)
	}
	err = CreateExclusive(dir, "k", []byte("K"))
	if !errors.Is(err, ErrPlacedNotSynced) {
		t.Errorf("CreateExclusive with a failing folder sync: error %v, want ErrPlacedNotSynced", err)
	}
	if got := readFile(t, filepath.Join(dir, "k")); string(got) != "K" {
		t.Errorf("k holds %q, want K (it was placed)", got)
	}
	restore()
	noTemps(t, dir)

	realFile := syncFile
	syncFile = func(*os.File) error { return errors.New("injected file sync failure") }
	defer func() { syncFile = realFile }()
	if err := WriteAtomic(dir, "x", []byte("C")); err == nil || errors.Is(err, ErrPlacedNotSynced) {
		t.Errorf("WriteAtomic with a failing file sync: error %v, want a plain error", err)
	}
	if got := readFile(t, filepath.Join(dir, "x")); string(got) != "B" {
		t.Errorf("x holds %q after a failed file sync, want B", got)
	}
	if err := CreateExclusive(dir, "k2", []byte("K")); err == nil || errors.Is(err, ErrPlacedNotSynced) {
		t.Errorf("CreateExclusive with a failing file sync: error %v, want a plain error", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "k2")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("k2 exists after a failed file sync: %v", err)
	}
	noTemps(t, dir)
}

func TestCreateExclusiveNeverReplaces(t *testing.T) {
	dir := newRoot(t)
	if err := CreateExclusive(dir, "k", []byte("first")); err != nil {
		t.Fatalf("first CreateExclusive: %v", err)
	}
	if got := perm(t, filepath.Join(dir, "k")); got != 0o600 {
		t.Errorf("mode %o, want 600", got)
	}
	err := CreateExclusive(dir, "k", []byte("second"))
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("second CreateExclusive: error %v, want one that wraps os.ErrExist", err)
	}
	if got := readFile(t, filepath.Join(dir, "k")); !bytes.Equal(got, []byte("first")) {
		t.Errorf("k holds %q, want first", got)
	}
	noTemps(t, dir)
}
