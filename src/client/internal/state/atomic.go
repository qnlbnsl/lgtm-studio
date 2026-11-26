package state

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// crashHook is called at each step of a write. A test sets it.
var crashHook func(step string)

// syncFile and syncFolder are the syncs of a write. A test replaces them.
var (
	syncFile   = (*os.File).Sync
	syncFolder = syncDirectory
)

// WriteAtomic replaces a file with the whole, synced contents of data.
func WriteAtomic(dir, name string, data []byte) error {
	return writeAtomic(dir, name, data, false)
}

// CreateExclusive places a synced file only when the target name is free.
func CreateExclusive(dir, name string, data []byte) error {
	return writeAtomic(dir, name, data, true)
}

func writeAtomic(dir, name string, data []byte, exclusive bool) error {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsRune(name, filepath.Separator) || strings.HasPrefix(name, TempPrefix) {
		return fmt.Errorf("%w: %q", ErrBadName, name)
	}

	temp, err := createTemp(dir)
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	err = writeAndSync(temp, data)
	closeErr := temp.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}

	target := filepath.Join(dir, name)
	if exclusive {
		err = os.Link(tempPath, target)
	} else {
		err = os.Rename(tempPath, target)
	}
	if err != nil {
		return err
	}
	reachStep("placed")

	var removeErr error
	if exclusive {
		removeErr = os.Remove(tempPath)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
	}
	cleanup = removeErr != nil
	if err := SyncDir(dir); err != nil {
		return fmt.Errorf("%w: %w", ErrPlacedNotSynced, errors.Join(err, removeErr))
	}
	reachStep("dir-synced")
	return removeErr
}

func createTemp(dir string) (*os.File, error) {
	for {
		var random [8]byte
		rand.Read(random[:])
		path := filepath.Join(dir, TempPrefix+hex.EncodeToString(random[:]))
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if !errors.Is(err, os.ErrExist) {
			return f, err
		}
	}
}

func writeAndSync(temp *os.File, data []byte) error {
	reachStep("temp-created")
	if _, err := temp.Write(data); err != nil {
		return err
	}
	reachStep("temp-written")
	if err := syncFile(temp); err != nil {
		return err
	}
	reachStep("temp-synced")
	return nil
}

func reachStep(step string) {
	if crashHook != nil {
		crashHook(step)
	}
}

// RemoveTemps removes temporary entries left by earlier writes.
func RemoveTemps(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), TempPrefix) {
			err := os.Remove(filepath.Join(dir, entry.Name()))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

// SyncDir syncs the directory entries of dir to disk.
func SyncDir(dir string) error {
	return syncFolder(dir)
}

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}
