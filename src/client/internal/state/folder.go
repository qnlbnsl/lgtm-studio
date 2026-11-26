// Package state keeps the private state folder of the client and writes files into it safely.
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
)

// Names inside the state folder.
const (
	LockFile        = "lock"
	SettingsFile    = "settings.json"
	KeyFile         = "machine.key"
	IdentityFile    = "identity.json"
	StoreFile       = "client.db"
	RepositoriesDir = "repositories"
	TempPrefix      = ".tmp-"
)

var (
	ErrNotAFolder      = errors.New("state: not a folder")
	ErrFolderExposed   = errors.New("state: folder can be used by others")
	ErrFolderNotOwned  = errors.New("state: folder belongs to another user")
	ErrFolderLocked    = errors.New("state: folder is held by another client")
	ErrFolderClosed    = errors.New("state: folder is closed")
	ErrParentUnsafe    = errors.New("state: a parent folder can be changed by others")
	ErrBadName         = errors.New("state: bad file name")
	ErrStoreOpen       = errors.New("state: store is open")
	ErrPlacedNotSynced = errors.New("state: file is in place but may not be synced")
)

// Paths names every path of the state folder.
type Paths struct {
	Dir, Lock, Settings, Key, Identity, Store, Repositories string
}

// PathsIn names the files and folders inside dir.
func PathsIn(dir string) Paths {
	return Paths{
		Dir:          dir,
		Lock:         filepath.Join(dir, LockFile),
		Settings:     filepath.Join(dir, SettingsFile),
		Key:          filepath.Join(dir, KeyFile),
		Identity:     filepath.Join(dir, IdentityFile),
		Store:        filepath.Join(dir, StoreFile),
		Repositories: filepath.Join(dir, RepositoriesDir),
	}
}

// Folder is the open, locked state folder.
type Folder struct {
	paths  Paths
	mu     sync.Mutex
	lock   *os.File
	claims map[string]struct{}
}

// Open prepares the private folder and holds its exclusive lock until Close.
func Open(dir string) (*Folder, error) {
	if dir == "" {
		return nil, ErrNotAFolder
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	uid := uint32(os.Geteuid())
	if parent := filepath.Dir(dir); parent != dir {
		resolved, err := prepareParent(parent, uid)
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(resolved, filepath.Base(dir))
	}
	info, err := ensureDirectory(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNotAFolder, dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%w: %s", ErrFolderExposed, dir)
	}
	if info.Sys().(*syscall.Stat_t).Uid != uid {
		return nil, fmt.Errorf("%w: %s", ErrFolderNotOwned, dir)
	}

	paths := PathsIn(dir)
	lock, err := os.OpenFile(paths.Lock, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFolder(lock, dir); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return &Folder{paths: paths, lock: lock, claims: make(map[string]struct{})}, nil
}

func lockFolder(lock *os.File, dir string) error {
	info, err := lock.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("state: %s must be a regular file", lock.Name())
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("%w: %s", ErrFolderLocked, dir)
		}
		return fmt.Errorf("state: lock %s: %w", lock.Name(), err)
	}
	if err := RemoveTemps(dir); err != nil {
		return err
	}
	for _, path := range directoryChain(dir) {
		if err := SyncDir(path); err != nil {
			return err
		}
	}
	return nil
}

// prepareParent checks the resolved ancestors before creating missing folders.
func prepareParent(path string, uid uint32) (string, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		parent, err := prepareParent(filepath.Dir(path), uid)
		if err != nil {
			return "", err
		}
		path = filepath.Join(parent, filepath.Base(path))
		info, err := ensureDirectory(path)
		if err != nil {
			return "", err
		}
		return path, checkParent(path, info, uid)
	}
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrParentUnsafe, err)
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrParentUnsafe, err)
	}
	for _, dir := range directoryChain(resolved) {
		info, err := os.Lstat(dir)
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrParentUnsafe, err)
		}
		if err := checkParent(dir, info, uid); err != nil {
			return "", err
		}
	}
	return resolved, nil
}

func checkParent(path string, info os.FileInfo, uid uint32) error {
	owner := info.Sys().(*syscall.Stat_t).Uid
	trustedOwner := owner == uid || owner == 0
	safeMode := info.Mode().Perm()&0o022 == 0 ||
		(owner == 0 && info.Mode()&os.ModeSticky != 0)
	if !info.IsDir() || !trustedOwner || !safeMode {
		return fmt.Errorf("%w: %s", ErrParentUnsafe, path)
	}
	return nil
}

func ensureDirectory(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if !errors.Is(err, os.ErrNotExist) {
		return info, err
	}
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	return os.Lstat(path)
}

func directoryChain(dir string) []string {
	var chain []string
	for {
		chain = append(chain, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	slices.Reverse(chain)
	return chain
}

// Paths returns a copy of the paths of the folder.
func (f *Folder) Paths() Paths { return f.paths }

// Close releases the lock after every claim has been released.
func (f *Folder) Close() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.claims) > 0 {
		return ErrStoreOpen
	}
	if f.lock == nil {
		return nil
	}
	err := f.lock.Close()
	f.lock = nil
	return err
}

// Claim reserves a name until the returned release function is called.
func (f *Folder) Claim(name string) (Paths, func(), error) {
	if f == nil {
		return Paths{}, nil, ErrFolderClosed
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lock == nil {
		return Paths{}, nil, ErrFolderClosed
	}
	if _, exists := f.claims[name]; exists {
		return Paths{}, nil, ErrStoreOpen
	}
	f.claims[name] = struct{}{}
	release := sync.OnceFunc(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.claims, name)
	})
	return f.paths, release, nil
}
