package folders

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func workingTreeClean(ctx context.Context, checkout checkoutPaths) (bool, error) {
	return checkoutClean(ctx, checkout, "")
}

type statusEntry struct {
	record, path, object, mode string
	assumed, skipped, merged   bool
}

func checkoutClean(ctx context.Context, checkout checkoutPaths, expectedHead string) (bool, error) {
	clean, children, err := privateCheckoutStatus(ctx, checkout, expectedHead)
	if err != nil {
		return false, err
	}
	for _, child := range children {
		childClean, err := submoduleClean(ctx, checkout.root, child)
		if err != nil {
			return false, err
		}
		clean = clean && childClean
	}
	return clean, nil
}

func privateCheckoutStatus(ctx context.Context, checkout checkoutPaths, expectedHead string) (bool, []statusEntry, error) {
	// Read filter definitions as data and verify the same configuration after status.
	// Git sees only the private configuration produced by captureStatus.
	view, err := newCheckoutView(ctx, checkout)
	if err != nil {
		return false, nil, err
	}
	snapshot, err := captureStatus(ctx, checkout)
	if err != nil {
		return false, nil, err
	}
	defer snapshot.cleanup()
	view.snapshot = snapshot

	headMatches := true
	if expectedHead != "" {
		out, err := view.statusCommand(nil, false, "rev-parse", "--verify", "--quiet", "--end-of-options", "HEAD")
		switch {
		case errors.Is(err, errGitNoResult):
			headMatches = false
		case err != nil || !validFolderObject(strings.TrimSuffix(out, "\n"), len(expectedHead)/2):
			return false, nil, errFolderUnreadable
		default:
			headMatches = out == expectedHead+"\n"
		}
	}

	entries, err := view.statusEntries()
	if err != nil {
		return false, nil, err
	}
	if err := view.checkStatusFilters(entries); err != nil {
		return false, nil, err
	}
	children, err := view.unsuppressStatusEntries(entries)
	if err != nil {
		return false, nil, err
	}
	// Parent status skips gitlinks completely. A cached diff still counts changes
	// to their index entries, and it compares only the index with the commit.
	_, staged := view.statusCommand(nil, false, "-c", "core.sparseCheckout=false",
		"diff", "--cached", "--quiet", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", "--")
	if staged != nil && !errors.Is(staged, errGitNoResult) {
		return false, nil, errFolderUnreadable
	}
	out, err := view.statusCommand(nil, true, "-c", "core.sparseCheckout=false",
		"status", "--porcelain=v1", "-z", "--untracked-files=no", "--ignore-submodules=all")
	if err != nil {
		return false, nil, errFolderUnreadable
	}
	clean := headMatches && staged == nil && out == ""
	if clean {
		untracked, err := view.hasUntrackedFiles(entries)
		if err != nil {
			return false, nil, err
		}
		clean = !untracked
	}
	if !view.unchanged() {
		return false, nil, errFolderUnreadable
	}
	return clean, children, nil
}

func (v *checkoutView) statusCommand(stdin io.Reader, useWorkTree bool, args ...string) (string, error) {
	var out folderGitOutput
	var workTree string
	if useWorkTree {
		workTree = v.snapshot.root
	}
	args = append([]string{"-c", "advice.sparseIndexExpanded=false"}, args...)
	err := runFolderGit(v.ctx, v.git, v.snapshot.gitDir, workTree, stdin, &out, args...)
	return out.String(), err
}

func (v *checkoutView) statusEntries() ([]statusEntry, error) {
	out, err := v.statusCommand(nil, false, "-c", "core.sparseCheckout=false", "ls-files", "-v", "--stage", "-z")
	if err != nil || out != "" && !strings.HasSuffix(out, "\x00") {
		return nil, errFolderUnreadable
	}
	var entries []statusEntry
	for record := range strings.SplitSeq(strings.TrimSuffix(out, "\x00"), "\x00") {
		if record == "" {
			continue
		}
		if len(record) < 3 || record[1] != ' ' {
			return nil, errFolderUnreadable
		}
		header, path, ok := strings.Cut(record[2:], "\t")
		fields := strings.Split(header, " ")
		if !ok || !filepath.IsLocal(path) || filepath.Clean(path) != path ||
			len(fields) != 3 || len(fields[2]) != 1 || fields[2][0] < '0' || fields[2][0] > '3' {
			return nil, errFolderUnreadable
		}
		mode, object := fields[0], fields[1]
		if !validFolderObject(object, 20) && !validFolderObject(object, 32) {
			return nil, errFolderUnreadable
		}
		tag := record[0]
		entries = append(entries, statusEntry{
			record: record[2:], path: path, object: object, mode: mode,
			assumed: tag >= 'a' && tag <= 'z', skipped: tag == 'S' || tag == 's', merged: fields[2] == "0",
		})
	}
	return entries, nil
}

func (v *checkoutView) checkStatusFilters(entries []statusEntry) error {
	drivers := make(map[string]bool)
	for key, value := range v.config {
		suffix, ok := strings.CutPrefix(key, "filter.")
		end := strings.LastIndexByte(suffix, '.')
		if !ok || end <= 0 {
			continue
		}
		switch suffix[end+1:] {
		case "clean", "process":
			if value == nil || *value != "" {
				drivers[suffix[:end]] = true
			}
		case "required":
			enabled, err := v.config.boolean(key)
			if enabled || err != nil {
				drivers[suffix[:end]] = true
			}
		}
	}
	if len(drivers) == 0 {
		return nil
	}
	var paths strings.Builder
	var count int
	for _, entry := range entries {
		if entry.merged && (entry.mode == "100644" || entry.mode == "100755") {
			paths.WriteString(entry.path)
			paths.WriteByte(0)
			count++
		}
	}
	if count == 0 {
		return nil
	}
	out, err := v.statusCommand(strings.NewReader(paths.String()), true, "check-attr", "-z", "--stdin", "filter")
	if err != nil || !strings.HasSuffix(out, "\x00") {
		return errFolderUnreadable
	}
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	if len(fields) != count*3 {
		return errFolderUnreadable
	}
	for i := 0; i < len(fields); i += 3 {
		if fields[i+1] != "filter" || drivers[fields[i+2]] {
			return errFolderUnreadable
		}
	}
	return nil
}

func (v *checkoutView) unsuppressStatusEntries(entries []statusEntry) ([]statusEntry, error) {
	omitted, err := v.absentOutsideSparseSelection(entries)
	if err != nil {
		return nil, err
	}
	var children []statusEntry
	var refresh strings.Builder
	for _, entry := range entries {
		if !entry.merged || omitted[entry.path] {
			continue
		}
		if entry.mode == "160000" {
			children = append(children, entry)
		}
		if entry.assumed || entry.skipped {
			// Reinsert the same staged object with fresh flags and an empty stat cache.
			refresh.WriteString(entry.record)
			refresh.WriteByte(0)
		}
	}
	if refresh.Len() != 0 {
		_, err := v.statusCommand(strings.NewReader(refresh.String()), false, "-c", "core.sparseCheckout=false",
			"update-index", "-z", "--index-info")
		if err != nil {
			return nil, errFolderUnreadable
		}
	}
	return children, nil
}

func (v *checkoutView) absentOutsideSparseSelection(entries []statusEntry) (map[string]bool, error) {
	sparse, err := v.config.boolean("core.sparsecheckout")
	if err != nil || !sparse {
		return nil, err
	}
	absent := make(map[string]bool)
	var paths strings.Builder
	for _, entry := range entries {
		if !entry.merged || !entry.skipped {
			continue
		}
		path := relativeGitPath(v.snapshot.root, entry.path)
		parent, name, err := openContainedParent(v.ctx, []string{v.snapshot.root}, path)
		if err == nil {
			var entryInfo unix.Stat_t
			err = unix.Fstatat(parent, name, &entryInfo, unix.AT_SYMLINK_NOFOLLOW)
			_ = unix.Close(parent)
		}
		if errors.Is(err, fs.ErrNotExist) {
			absent[entry.path] = true
			paths.WriteString(entry.path)
			paths.WriteByte(0)
		} else if err != nil {
			return nil, errFolderUnreadable
		}
	}
	if len(absent) == 0 {
		return nil, nil
	}
	out, err := v.statusCommand(strings.NewReader(paths.String()), true, "sparse-checkout", "check-rules", "-z")
	if err != nil || out != "" && !strings.HasSuffix(out, "\x00") {
		return nil, errFolderUnreadable
	}
	for path := range strings.SplitSeq(strings.TrimSuffix(out, "\x00"), "\x00") {
		delete(absent, path)
	}
	return absent, nil
}

func (v *checkoutView) hasUntrackedFiles(entries []statusEntry) (bool, error) {
	ignoreCase, err := v.config.boolean("core.ignorecase")
	if err != nil {
		return false, err
	}
	key := func(path string) string { return path }
	if ignoreCase {
		key = foldStatusPath
	}
	tracked := make(map[string]string)
	directories := make(map[string]bool)
	for _, entry := range entries {
		path := key(entry.path)
		tracked[path] = entry.mode
		for parent := filepath.Dir(path); parent != "." && !directories[parent]; parent = filepath.Dir(parent) {
			directories[parent] = true
		}
	}

	// Git's untracked scan can open a nested repository's Git directory to
	// identify it. Walk directory entries here and ask Git only about ignores.
	pending := []string{""}
	unknownGitMarker := false
	for len(pending) != 0 {
		dir := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		root := v.snapshot.root
		if dir != "" {
			root = relativeGitPath(root, dir)
		}
		fd, err := openAnchoredDirectory(v.ctx, root)
		if err != nil {
			return false, errFolderUnreadable
		}
		folder := os.NewFile(uintptr(fd), "")
		dirTracked := dir == "" || directories[key(dir)]
		untracked, marker, err := v.scanUntrackedDirectory(folder, dir, key, tracked, directories, dirTracked, &pending)
		_ = folder.Close()
		if untracked || err != nil {
			return untracked, err
		}
		unknownGitMarker = unknownGitMarker || marker
	}
	if unknownGitMarker {
		return false, errFolderUnreadable
	}
	return false, nil
}

// Git runs in the C locale and folds only ASCII path bytes.
func foldStatusPath(path string) string {
	firstUpper := strings.IndexAny(path, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if firstUpper < 0 {
		return path
	}
	folded := []byte(path)
	for i := firstUpper; i < len(folded); i++ {
		if folded[i] >= 'A' && folded[i] <= 'Z' {
			folded[i] += 'a' - 'A'
		}
	}
	return string(folded)
}

func (v *checkoutView) scanUntrackedDirectory(folder *os.File, dir string, key func(string) string,
	tracked map[string]string, directories map[string]bool, dirTracked bool, pending *[]string,
) (bool, bool, error) {
	unknownGitMarker := false
	for {
		if v.ctx.Err() != nil {
			return false, false, errFolderUnreadable
		}
		items, err := folder.ReadDir(128)
		if err != nil && err != io.EOF {
			return false, false, errFolderUnreadable
		}
		var candidates []string
		for _, item := range items {
			name := item.Name()
			if strings.EqualFold(name, ".git") {
				unknownGitMarker = unknownGitMarker || !dirTracked
				continue
			}
			kind := item.Type()
			if !kind.IsRegular() && kind != fs.ModeDir && kind != fs.ModeSymlink {
				continue
			}
			path := filepath.Join(dir, name)
			mode, known := tracked[key(path)]
			if mode == "160000" || known && !item.IsDir() {
				continue
			}
			if item.IsDir() {
				if directories[key(path)] {
					*pending = append(*pending, path)
					continue
				}
				path += "/"
			}
			candidates = append(candidates, path)
		}
		if len(candidates) != 0 {
			ignored, err := v.statusIgnores(candidates)
			if err != nil {
				return false, false, err
			}
			for _, path := range candidates {
				if ignored[path] {
					continue
				}
				dir, directory := strings.CutSuffix(path, "/")
				if !directory {
					return true, false, nil
				}
				*pending = append(*pending, dir)
			}
		}
		if err == io.EOF {
			return false, unknownGitMarker, nil
		}
	}
}

func (v *checkoutView) statusIgnores(paths []string) (map[string]bool, error) {
	input := strings.Join(paths, "\x00") + "\x00"
	out, err := v.statusCommand(strings.NewReader(input), true, "-c", "core.sparseCheckout=false",
		"check-ignore", "--no-index", "-z", "--stdin")
	if err != nil && !errors.Is(err, errGitNoResult) || out != "" && !strings.HasSuffix(out, "\x00") {
		return nil, errFolderUnreadable
	}
	ignored := make(map[string]bool)
	for path := range strings.SplitSeq(strings.TrimSuffix(out, "\x00"), "\x00") {
		ignored[path] = true
	}
	return ignored, nil
}

func submoduleClean(ctx context.Context, root string, entry statusEntry) (bool, error) {
	child := relativeGitPath(root, entry.path)
	if child == root || !folderWithin(root, child) || !validFolderPath(child, maxResolvedFolderBytes) {
		return false, errFolderUnreadable
	}
	fd, err := openAnchoredDirectory(ctx, child)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errFolderUnreadable
	}
	var marker unix.Stat_t
	err = unix.Fstatat(fd, ".git", &marker, unix.AT_SYMLINK_NOFOLLOW)
	_ = unix.Close(fd)
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil || marker.Mode&unix.S_IFMT != unix.S_IFDIR && marker.Mode&unix.S_IFMT != unix.S_IFREG {
		return false, errFolderUnreadable
	}
	paths, refusal := checkoutDirectories(ctx, child)
	if refusal != folderOK {
		return false, errFolderUnreadable
	}
	return checkoutClean(ctx, paths, entry.object)
}
