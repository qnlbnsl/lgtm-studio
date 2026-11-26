package state

import "os"

// crashHook is called at each step of a write. A test sets it.
var crashHook func(step string)

// syncFile and syncFolder are the syncs of a write. A test replaces them.
var (
	syncFile   = func(f *os.File) error { return f.Sync() }
	syncFolder = func(dir string) error { return nil }
)

// WriteAtomic is a stub of the test writer.
func WriteAtomic(dir, name string, data []byte) error { return nil }

// CreateExclusive is a stub of the test writer.
func CreateExclusive(dir, name string, data []byte) error { return nil }

// RemoveTemps is a stub of the test writer.
func RemoveTemps(dir string) error { return nil }

// SyncDir is a stub of the test writer.
func SyncDir(dir string) error { return nil }
