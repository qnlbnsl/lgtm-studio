package state

// SetCrashHookForTest sets the hook of the steps of a write and returns a function that puts the old one back.
// This is a stub of the test writer.
func SetCrashHookForTest(hook func(step string)) (restore func()) {
	old := crashHook
	crashHook = hook
	return func() { crashHook = old }
}

// SetSyncFolderForTest replaces the sync of a folder and returns a function that puts the old one back.
// This is a stub of the test writer.
func SetSyncFolderForTest(fn func(dir string) error) (restore func()) {
	old := syncFolder
	syncFolder = fn
	return func() { syncFolder = old }
}
