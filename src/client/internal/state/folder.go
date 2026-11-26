// Package state keeps the private state folder of the client and writes files into it safely.
package state

import "errors"

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

// PathsIn is a stub of the test writer.
func PathsIn(dir string) Paths { return Paths{} }

// Folder is the open, locked state folder.
type Folder struct{}

var errStub = errors.New("stub of the test writer")

// Open is a stub of the test writer.
func Open(dir string) (*Folder, error) { return nil, errStub }

// Paths is a stub of the test writer.
func (f *Folder) Paths() Paths { return Paths{} }

// Close is a stub of the test writer.
func (f *Folder) Close() error { return errStub }

// Claim is a stub of the test writer.
func (f *Folder) Claim(name string) (Paths, func(), error) {
	return Paths{}, func() {}, errStub
}
