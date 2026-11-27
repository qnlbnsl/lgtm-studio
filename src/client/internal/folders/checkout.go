package folders

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"golang.org/x/sys/unix"
)

const (
	maxTypedFolderBytes    = 4096
	maxResolvedFolderBytes = 8192
	maxConfigBytes         = 4 << 20
	maxMetadataBytes       = 128 << 20
	maxGitOutputBytes      = 16 << 20

	folderOK            = linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED
	folderNotDirectory  = linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER
	folderNotRepository = linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY
	folderNotReadable   = linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE
	folderPathNotUsable = linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE
)

var (
	errFolderUnreadable = errors.New("The folder cannot be read.")
	errFolderUnusable   = errors.New("The folder path cannot be used.")
)

type checkoutPaths struct {
	root, gitDir, commonDir, indexPath string
}

type statusSnapshot struct {
	root, gitDir string
	cleanup      func()
}

func readCheckout(ctx context.Context, path, home string) (checkoutPaths, *linkv1.FolderFacts, linkv1.FolderRefusal) {
	switch {
	case !validFolderPath(path, maxTypedFolderBytes):
		return checkoutPaths{}, nil, folderPathNotUsable
	case path == "~" || strings.HasPrefix(path, "~/"):
		if !filepath.IsAbs(home) {
			return checkoutPaths{}, nil, folderPathNotUsable
		}
		path = home + path[1:]
	}
	if !filepath.IsAbs(path) || !utf8.ValidString(path) || strings.ContainsRune(path, 0) {
		return checkoutPaths{}, nil, folderPathNotUsable
	}
	dir, physical, err := openFolderDirectory(ctx, path)
	if err != nil {
		switch {
		case errors.Is(err, syscall.ELOOP), errors.Is(err, syscall.ENAMETOOLONG):
			return checkoutPaths{}, nil, folderPathNotUsable
		case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
			return checkoutPaths{}, nil, folderNotDirectory
		default:
			return checkoutPaths{}, nil, folderNotReadable
		}
	}
	paths, refusal := locateCheckout(ctx, dir, physical)
	if refusal != folderOK {
		return checkoutPaths{}, nil, refusal
	}
	return readResolvedCheckout(ctx, paths, physical)
}

// readResolvedCheckout is the part of discovery after the typed path is resolved.
// The physical path is the resolved typed directory, which bounds a configured worktree.
func readResolvedCheckout(ctx context.Context, paths checkoutPaths, physical string) (checkoutPaths, *linkv1.FolderFacts, linkv1.FolderRefusal) {
	if !validFolderPath(paths.root, maxResolvedFolderBytes) {
		return checkoutPaths{}, nil, folderPathNotUsable
	}
	view, err := newCheckoutView(ctx, paths)
	if err != nil {
		return checkoutPaths{}, nil, folderNotReadable
	}
	if view.bare {
		return checkoutPaths{}, nil, folderNotRepository
	}
	if view.workTree != "" {
		if !folderWithin(view.workTree, physical) || folderWithin(paths.gitDir, physical) {
			return checkoutPaths{}, nil, folderNotRepository
		}
		if !validFolderPath(view.workTree, maxResolvedFolderBytes) {
			return checkoutPaths{}, nil, folderPathNotUsable
		}
		paths.root = view.workTree
		view.paths.root = view.workTree
	}
	if err := view.makeSnapshot(); err != nil {
		return checkoutPaths{}, nil, folderNotReadable
	}
	defer view.snapshot.cleanup()
	if err := view.prepare(); err != nil {
		return checkoutPaths{}, nil, folderNotReadable
	}
	facts, err := view.facts(paths.root)
	if errors.Is(err, errFolderUnusable) {
		return checkoutPaths{}, nil, folderPathNotUsable
	}
	if err != nil || !view.unchanged() {
		return checkoutPaths{}, nil, folderNotReadable
	}
	return paths, facts, folderOK
}

func validFolderPath(path string, limit int) bool {
	return path != "" && len(path) <= limit && utf8.ValidString(path) && !strings.ContainsRune(path, 0)
}

// openFolderDirectory resolves directory components before applying a subsequent parent
// component. Descriptor-relative opens also avoid the kernel's whole-path argument limit.
func openFolderDirectory(ctx context.Context, path string) (*os.File, string, error) {
	if !filepath.IsAbs(path) {
		return nil, "", syscall.EINVAL
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if fd >= 0 {
			_ = unix.Close(fd)
		}
	}()

	var physical []string
	pending := strings.Split(path, "/")
	links := 0
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		part := pending[0]
		pending = pending[1:]
		if part == "" || part == "." {
			continue
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.ENOTDIR) {
			var target [maxResolvedFolderBytes + 1]byte
			n, linkErr := unix.Readlinkat(fd, part, target[:])
			if linkErr == nil {
				links++
				if links > 40 {
					return nil, "", syscall.ELOOP
				}
				if n >= len(target) {
					return nil, "", syscall.ENAMETOOLONG
				}
				if target[0] == '/' {
					next, err = unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
					if err != nil {
						return nil, "", err
					}
					_ = unix.Close(fd)
					fd, physical = next, nil
				}
				pending = append(strings.Split(string(target[:n]), "/"), pending...)
				continue
			}
		}
		if err != nil {
			return nil, "", err
		}
		_ = unix.Close(fd)
		fd = next
		if part != ".." {
			physical = append(physical, part)
		} else if len(physical) > 0 {
			physical = physical[:len(physical)-1]
		}
	}

	if err := unix.Faccessat(fd, ".", unix.R_OK|unix.X_OK, unix.AT_EACCESS); err != nil {
		return nil, "", err
	}
	canonical := "/" + strings.Join(physical, "/")
	file := os.NewFile(uintptr(fd), canonical)
	fd = -1
	return file, canonical, nil
}

func folderWithin(parent, path string) bool {
	return path == parent || strings.HasPrefix(path, strings.TrimSuffix(parent, "/")+"/")
}

// relativeGitPath joins a Git path without folding a parent component before resolution.
func relativeGitPath(parent, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return strings.TrimSuffix(parent, "/") + "/" + path
}

func locateCheckout(ctx context.Context, dir *os.File, physical string) (checkoutPaths, linkv1.FolderRefusal) {
	defer func() { _ = dir.Close() }()
	for root := physical; ; root = filepath.Dir(root) {
		var gitMarker unix.Stat_t
		err := unix.Fstatat(int(dir.Fd()), ".git", &gitMarker, unix.AT_SYMLINK_NOFOLLOW)
		if err == nil {
			paths, refusal := checkoutDirectories(ctx, root)
			if refusal != folderNotRepository {
				return paths, refusal
			}
		} else if err != syscall.ENOENT {
			return checkoutPaths{}, folderNotReadable
		}
		if root == "/" || hasGitDirectoryLayout(dir) {
			return checkoutPaths{}, folderNotRepository
		}
		parent, err := unix.Openat(int(dir.Fd()), "..", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		if err != nil || ctx.Err() != nil {
			if err == nil {
				_ = unix.Close(parent)
			}
			return checkoutPaths{}, folderNotReadable
		}
		_ = dir.Close()
		dir = os.NewFile(uintptr(parent), "")
	}
}

func hasGitDirectoryLayout(dir *os.File) bool {
	var head, objects, common unix.Stat_t
	fd := int(dir.Fd())
	return unix.Fstatat(fd, "HEAD", &head, 0) == nil &&
		(unix.Fstatat(fd, "objects", &objects, 0) == nil && objects.Mode&unix.S_IFMT == unix.S_IFDIR ||
			unix.Fstatat(fd, "commondir", &common, 0) == nil)
}

func checkoutDirectories(ctx context.Context, root string) (checkoutPaths, linkv1.FolderRefusal) {
	gitPath := relativeGitPath(root, ".git")
	dir, gitDir, err := openFolderDirectory(ctx, gitPath)
	if errors.Is(err, syscall.ENOTDIR) {
		gitFile, _, fileErr := readFolderFile(ctx, []string{root}, gitPath, maxConfigBytes)
		target, ok := strings.CutPrefix(strings.TrimRight(string(gitFile), "\r\n"), "gitdir: ")
		if fileErr != nil || !ok || !validGitPath(target) {
			return checkoutPaths{}, folderNotReadable
		}
		dir, gitDir, err = openFolderDirectory(ctx, relativeGitPath(root, target))
	}
	if err != nil {
		return checkoutPaths{}, folderNotReadable
	}
	var head unix.Stat_t
	err = unix.Fstatat(int(dir.Fd()), "HEAD", &head, unix.AT_SYMLINK_NOFOLLOW)
	dir.Close()
	if errors.Is(err, fs.ErrNotExist) {
		return checkoutPaths{}, folderNotRepository
	}
	if err != nil {
		return checkoutPaths{}, folderNotReadable
	}

	commonDir := gitDir
	common, _, err := readFolderFile(ctx, []string{gitDir}, relativeGitPath(gitDir, "commondir"), maxConfigBytes)
	if err == nil {
		target := strings.TrimRight(string(common), "\r\n")
		if !validGitPath(target) {
			return checkoutPaths{}, folderNotReadable
		}
		dir, commonDir, err = openFolderDirectory(ctx, relativeGitPath(gitDir, target))
		if err != nil {
			return checkoutPaths{}, folderNotReadable
		}
		dir.Close()
		if !linkedWorktree(ctx, root, gitDir, commonDir) {
			return checkoutPaths{}, folderNotReadable
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return checkoutPaths{}, folderNotReadable
	}
	return checkoutPaths{root: root, gitDir: gitDir, commonDir: commonDir, indexPath: relativeGitPath(gitDir, "index")}, folderOK
}

// linkedWorktree admits a separate common folder only through Git's own relationship for a
// linked worktree: the worktree's metadata folder is an entry of the main repository's
// worktrees folder, and its gitdir file names the .git file of this worktree.
func linkedWorktree(ctx context.Context, root, gitDir, commonDir string) bool {
	if filepath.Dir(gitDir) != relativeGitPath(commonDir, "worktrees") {
		return false
	}
	back, _, err := readFolderFile(ctx, []string{gitDir}, relativeGitPath(gitDir, "gitdir"), maxConfigBytes)
	target := strings.TrimRight(string(back), "\r\n")
	if err != nil || !validGitPath(target) {
		return false
	}
	parent, ok := strings.CutSuffix(relativeGitPath(gitDir, target), "/.git")
	if !ok {
		return false
	}
	dir, worktree, err := openFolderDirectory(ctx, parent)
	if err != nil {
		return false
	}
	dir.Close()
	return worktree == root
}

func validGitPath(path string) bool {
	return path != "" && !strings.ContainsAny(path, "\x00\r\n")
}

// installedFolderGit checks each path component before following any links from it.
func installedFolderGit(checkout checkoutPaths) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.IsAbs(dir) {
			canonical := trustedGitExecutable(relativeGitPath(dir, "git"), checkout)
			if canonical == "" {
				continue
			}
			return canonical
		}
	}
	return ""
}

func trustedGitExecutable(entry string, checkout checkoutPaths) string {
	path := "/"
	if info, err := os.Lstat(path); err != nil || !trustedGitEntry(path, info) {
		return ""
	}
	pending := strings.Split(entry, "/")
	links := 0
	var info fs.FileInfo
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			path = filepath.Dir(path)
			continue
		}
		next := relativeGitPath(path, part)
		if folderWithin(checkout.root, next) || folderWithin(checkout.gitDir, next) || folderWithin(checkout.commonDir, next) {
			return ""
		}
		var err error
		info, err = os.Lstat(next)
		if err != nil {
			return ""
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			links++
			target, err := os.Readlink(next)
			if err != nil || links > 40 {
				return ""
			}
			if filepath.IsAbs(target) {
				path = "/"
			}
			pending = append(strings.Split(target, "/"), pending...)
			continue
		}
		if !trustedGitEntry(next, info) || len(pending) > 0 && !info.IsDir() {
			return ""
		}
		path = next
	}
	if info != nil && info.Mode().IsRegular() && unix.Faccessat(unix.AT_FDCWD, path, unix.X_OK, unix.AT_EACCESS) == nil {
		return path
	}
	return ""
}

func trustedGitEntry(path string, info fs.FileInfo) bool {
	if gitPathReadOnly(path) {
		return true
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0 && os.Geteuid() != 0 && info.Mode().Perm()&0o022 == 0 &&
		unix.Faccessat(unix.AT_FDCWD, path, unix.W_OK, unix.AT_EACCESS) == syscall.EACCES
}

func gitPathReadOnly(path string) bool {
	var info unix.Statfs_t
	// Both supported kernels use the low flag bit for a read-only mount.
	if err := unix.Statfs(path, &info); err != nil {
		return false
	}
	return info.Flags&1 != 0
}

type folderGitOutput struct{ bytes.Buffer }

func (out *folderGitOutput) Write(p []byte) (int, error) {
	if len(p) > maxGitOutputBytes-out.Len() {
		return 0, errFolderUnreadable
	}
	return out.Buffer.Write(p)
}

// runFolderGit runs a built-in Git command against an isolated metadata directory.
func runFolderGit(ctx context.Context, git, gitDir, workTree string, stdin io.Reader, stdout io.Writer, args ...string) error {
	dir := gitDir
	if gitDir == "" {
		// The explicit null directory keeps a config parse outside repository discovery.
		dir, gitDir = "/", os.DevNull
	}
	prefix := []string{"--no-optional-locks", "--git-dir=" + gitDir,
		"-c", "protocol.allow=never", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull,
		"-c", "core.untrackedCache=false", "-c", "core.commitGraph=false"}
	if workTree != "" {
		prefix = append(prefix, "--work-tree="+workTree)
	}
	cmd := exec.CommandContext(ctx, git, append(prefix, args...)...)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + filepath.Dir(git), "HOME=" + gitDir, "XDG_CONFIG_HOME=" + gitDir, "LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_ATTR_NOSYSTEM=1", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1",
		"GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=", "GIT_TERMINAL_PROMPT=0",
	}
	cmd.Stdin, cmd.Stdout = stdin, stdout
	var diagnostic folderGitDiagnostic
	cmd.Stderr = &diagnostic
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return os.ErrProcessDone
		}
		return nil
	}
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	// Preserve only the one ordinary exit status for a lookup that has no result.
	var exit *exec.ExitError
	if ctx.Err() == nil && !diagnostic.reported && errors.As(err, &exit) && exit.ExitCode() == 1 {
		return errGitNoResult
	}
	if err != nil || diagnostic.reported || ctx.Err() != nil {
		return errFolderUnreadable
	}
	return nil
}

// folderGitDiagnostic records warnings without retaining repository-controlled text.
type folderGitDiagnostic struct{ reported bool }

func (d *folderGitDiagnostic) Write(p []byte) (int, error) {
	d.reported = d.reported || len(p) != 0
	return len(p), nil
}

var errGitNoResult = errors.New("The folder cannot be read.")

// openAnchoredDirectory opens a canonical absolute directory without following any link,
// so that a folder found by discovery cannot be swapped for a link to another place.
func openAnchoredDirectory(ctx context.Context, path string) (int, error) {
	if !filepath.IsAbs(path) {
		return -1, errFolderUnreadable
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, errFolderUnreadable
	}
	for part := range strings.SplitSeq(path, "/") {
		if part == "" {
			continue
		}
		next, err := openContainedDirectory(ctx, fd, part)
		_ = unix.Close(fd)
		if err != nil {
			return -1, err
		}
		fd = next
	}
	return fd, nil
}

// openContainedDirectory opens one directory entry below an open directory. A link, a
// parent component and anything other than a directory are refused; absence stays absence.
func openContainedDirectory(ctx context.Context, dirfd int, name string) (int, error) {
	if name == "." || name == ".." || ctx.Err() != nil {
		return -1, errFolderUnreadable
	}
	fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if errors.Is(err, syscall.ENOENT) {
		return -1, syscall.ENOENT
	}
	if err != nil {
		return -1, errFolderUnreadable
	}
	return fd, nil
}

// openContainedParent opens the directory that holds path. The path must lie below one of
// the roots, which are the checkout and its own Git folders as discovery found them. Every
// component after the root is entered without following a link, and no parent component
// is accepted, so the open never leaves the root.
func openContainedParent(ctx context.Context, roots []string, path string) (int, string, error) {
	anchor := ""
	for _, root := range roots {
		if root != "" && len(root) > len(anchor) && folderWithin(root, path) && path != root {
			anchor = root
		}
	}
	if anchor == "" || !validGitPath(path) {
		return -1, "", errFolderUnreadable
	}
	parts := strings.Split(strings.TrimPrefix(path[len(strings.TrimSuffix(anchor, "/")):], "/"), "/")
	name := parts[len(parts)-1]
	if name == "" || name == "." || name == ".." {
		return -1, "", errFolderUnreadable
	}
	fd, err := openAnchoredDirectory(ctx, anchor)
	if err != nil {
		return -1, "", err
	}
	for _, part := range parts[:len(parts)-1] {
		if part == "" || part == "." {
			continue
		}
		next, err := openContainedDirectory(ctx, fd, part)
		_ = unix.Close(fd)
		if err != nil {
			return -1, "", err
		}
		fd = next
	}
	return fd, name, nil
}

// openFolderFile opens a regular file below the roots without following any link.
func openFolderFile(ctx context.Context, roots []string, path string) (*os.File, error) {
	parent, name, err := openContainedParent(ctx, roots, path)
	if err != nil {
		return nil, err
	}
	defer unix.Close(parent)
	return openRegularAt(parent, name, unix.AT_SYMLINK_NOFOLLOW)
}

// openServiceFile opens a file that the service's own environment selects, such as the
// default excludes file. The checkout selects no part of this path, so links in it are the
// service's own choice and are followed.
func openServiceFile(ctx context.Context, path string) (*os.File, error) {
	dir, name := filepath.Split(path)
	parent, _, err := openFolderDirectory(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return openRegularAt(int(parent.Fd()), name, 0)
}

func openRegularAt(dirfd int, name string, flags int) (*os.File, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(dirfd, name, &stat, flags); err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return nil, syscall.ENOENT
		}
		return nil, errFolderUnreadable
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, errFolderUnreadable
	}
	open := unix.O_RDONLY | unix.O_NONBLOCK | unix.O_CLOEXEC | unix.O_NOCTTY
	if flags&unix.AT_SYMLINK_NOFOLLOW != 0 {
		open |= unix.O_NOFOLLOW
	}
	fd, err := unix.Openat(dirfd, name, open, 0)
	if err != nil {
		return nil, errFolderUnreadable
	}
	return os.NewFile(uintptr(fd), ""), nil
}

// readFolderFile reads a regular file below the roots without following any link.
func readFolderFile(ctx context.Context, roots []string, path string, limit int) ([]byte, *unix.Stat_t, error) {
	file, err := openFolderFile(ctx, roots, path)
	if err != nil {
		return nil, nil, err
	}
	return readOpenedFile(ctx, file, limit)
}

func readOpenedFile(ctx context.Context, file *os.File, limit int) ([]byte, *unix.Stat_t, error) {
	defer file.Close()
	var before, after unix.Stat_t
	fd := int(file.Fd())
	if err := unix.Fstat(fd, &before); err != nil || before.Mode&unix.S_IFMT != unix.S_IFREG || before.Size > int64(limit) {
		return nil, nil, errFolderUnreadable
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || len(data) > limit || unix.Fstat(fd, &after) != nil ||
		!sameFolderFile(&before, &after) || ctx.Err() != nil {
		return nil, nil, errFolderUnreadable
	}
	return data, &before, nil
}

func sameFolderFile(a, b *unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Size == b.Size && a.Mode == b.Mode &&
		a.Mtim == b.Mtim && a.Ctim == b.Ctim
}

type checkoutView struct {
	ctx      context.Context
	paths    checkoutPaths
	snapshot statusSnapshot
	git      string
	sources  map[string]folderSource

	config   folderConfig
	bare     bool
	workTree string
	hashSize int
	shallow  map[string]bool
	common   string
}

func newCheckoutView(ctx context.Context, paths checkoutPaths) (*checkoutView, error) {
	git := installedFolderGit(paths)
	if git == "" || ctx.Err() != nil {
		return nil, errFolderUnreadable
	}
	view := &checkoutView{ctx: ctx, paths: paths, git: git, sources: make(map[string]folderSource)}
	if err := view.readLayoutConfig(); err != nil {
		return nil, err
	}
	return view, nil
}

func (v *checkoutView) makeSnapshot() error {
	base, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil || !filepath.IsAbs(base) {
		return errFolderUnreadable
	}
	for _, source := range []string{v.paths.root, v.paths.gitDir, v.paths.commonDir} {
		if folderWithin(source, base) {
			return errFolderUnreadable
		}
	}
	dir, err := os.MkdirTemp(base, "lgtm-checkout-")
	if err != nil {
		return errFolderUnreadable
	}
	v.snapshot = statusSnapshot{root: v.paths.root, gitDir: dir, cleanup: func() { _ = os.RemoveAll(dir) }}
	return nil
}

func (v *checkoutView) gitOutput(stdin io.Reader, args ...string) (string, error) {
	var out folderGitOutput
	err := runFolderGit(v.ctx, v.git, v.snapshot.gitDir, "", stdin, &out, args...)
	return out.String(), err
}

// folderSourceKind says how a source was read, so that the final check reads it the same way.
type folderSourceKind int

const (
	// sourceContained is a regular file below the checkout or its Git folders.
	sourceContained folderSourceKind = iota
	// sourceService is a file that the service's own environment selects.
	sourceService
	// sourceLink is a symbolic link below the Git folders whose target is read as text.
	sourceLink
)

// folderSource is a source's identity, or nil for its absence.
type folderSource struct {
	kind folderSourceKind
	info *unix.Stat_t
}

func (v *checkoutView) roots() []string {
	return []string{v.paths.root, v.paths.gitDir, v.paths.commonDir}
}

// source reads a regular file below the checkout or its Git folders and remembers either its
// identity or its absence for the final check. A link or a path outside those folders fails.
func (v *checkoutView) source(path string, limit int) ([]byte, error) {
	data, info, err := readFolderFile(v.ctx, v.roots(), path, limit)
	return data, v.remember(path, sourceContained, info, err)
}

// serviceSource reads a file that the service's own environment selects.
func (v *checkoutView) serviceSource(path string, limit int) ([]byte, error) {
	var data []byte
	var info *unix.Stat_t
	file, err := openServiceFile(v.ctx, path)
	if err == nil {
		data, info, err = readOpenedFile(v.ctx, file, limit)
	}
	return data, v.remember(path, sourceService, info, err)
}

func (v *checkoutView) remember(path string, kind folderSourceKind, info *unix.Stat_t, err error) error {
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errFolderUnreadable
	}
	if previous, seen := v.sources[path]; seen && (previous.kind != kind || !sameFolderSource(previous.info, info)) {
		return errFolderUnreadable
	}
	v.sources[path] = folderSource{kind: kind, info: info}
	return nil
}

func sameFolderSource(a, b *unix.Stat_t) bool {
	return a == nil && b == nil || a != nil && b != nil && sameFolderFile(a, b)
}

// headSource reads the Git folder's HEAD. Git also accepts HEAD as a symbolic link whose
// target is a reference name; the link is read as that name and never followed.
func (v *checkoutView) headSource() ([]byte, error) {
	path := relativeGitPath(v.paths.gitDir, "HEAD")
	target, info, err := readFolderLink(v.ctx, v.roots(), path)
	if errors.Is(err, errFolderNotLink) {
		return v.source(path, maxConfigBytes)
	}
	if err := v.remember(path, sourceLink, info, err); err != nil || info == nil || target != strings.TrimSpace(target) {
		return nil, errFolderUnreadable
	}
	return []byte("ref: " + target + "\n"), nil
}

var errFolderNotLink = errors.New("The folder cannot be read.")

// readFolderLink reads the target of a symbolic link below the roots without following it.
// An entry that is not a link gives errFolderNotLink.
func readFolderLink(ctx context.Context, roots []string, path string) (string, *unix.Stat_t, error) {
	parent, name, err := openContainedParent(ctx, roots, path)
	if err != nil {
		return "", nil, err
	}
	defer unix.Close(parent)
	var before, after unix.Stat_t
	if err := unix.Fstatat(parent, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return "", nil, syscall.ENOENT
		}
		return "", nil, errFolderUnreadable
	}
	if before.Mode&unix.S_IFMT != unix.S_IFLNK {
		return "", nil, errFolderNotLink
	}
	var target [maxResolvedFolderBytes + 1]byte
	n, err := unix.Readlinkat(parent, name, target[:])
	if err != nil || n >= len(target) || unix.Fstatat(parent, name, &after, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!sameFolderFile(&before, &after) || !validGitPath(string(target[:n])) {
		return "", nil, errFolderUnreadable
	}
	return string(target[:n]), &before, nil
}

// unchanged checks every source again, by the same reader that first read it.
func (v *checkoutView) unchanged() bool {
	for path, before := range v.sources {
		var current *unix.Stat_t
		var err error
		switch before.kind {
		case sourceLink:
			_, current, err = readFolderLink(v.ctx, v.roots(), path)
		default:
			var file *os.File
			if before.kind == sourceService {
				file, err = openServiceFile(v.ctx, path)
			} else {
				file, err = openFolderFile(v.ctx, v.roots(), path)
			}
			if err == nil {
				current = new(unix.Stat_t)
				err = unix.Fstat(int(file.Fd()), current)
				file.Close()
			}
		}
		if errors.Is(err, fs.ErrNotExist) && before.info == nil {
			continue
		}
		if err != nil || !sameFolderSource(before.info, current) {
			return false
		}
	}
	return v.ctx.Err() == nil
}

func (v *checkoutView) write(path string, data []byte) error {
	target := filepath.Join(v.snapshot.gitDir, path)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return errFolderUnreadable
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		return errFolderUnreadable
	}
	return nil
}

func (v *checkoutView) copySource(path, name string, limit int) error {
	data, err := v.source(path, limit)
	if err != nil || data == nil {
		return err
	}
	return v.write(name, data)
}

type folderConfig map[string]*string

func (c folderConfig) get(key string) string {
	if value := c[key]; value != nil {
		return *value
	}
	return ""
}

func (c folderConfig) boolean(key string) (bool, error) {
	value, set := c[key]
	if !set {
		return false, nil
	}
	if value == nil {
		return true, nil
	}
	switch strings.ToLower(*value) {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off", "":
		return false, nil
	}
	number := *value
	if strings.ContainsRune("kKmMgG", rune(number[len(number)-1])) {
		number = number[:len(number)-1]
	}
	n, err := strconv.ParseInt(number, 0, 32)
	if err != nil {
		return false, errFolderUnreadable
	}
	return n != 0, nil
}

func (v *checkoutView) readConfig(path string, config folderConfig) error {
	data, err := v.source(path, maxConfigBytes)
	if err != nil || data == nil {
		return err
	}
	out, err := v.gitOutput(bytes.NewReader(data), "config", "--no-includes", "--file", "-", "--null", "--list")
	if err != nil {
		return errFolderUnreadable
	}
	for entry := range strings.SplitSeq(strings.TrimSuffix(out, "\x00"), "\x00") {
		if entry == "" {
			continue
		}
		key, value, hasValue := strings.Cut(entry, "\n")
		if key == "remote.origin.url" {
			// Multiple different addresses cannot identify a single origin.
			if previous, found := config[key]; found {
				if previous == nil || !hasValue || *previous != value {
					config[key] = nil
				}
				continue
			}
		}
		if hasValue {
			config[key] = &value
		} else {
			config[key] = nil
		}
	}
	return nil
}

func (v *checkoutView) readLayoutConfig() error {
	config := make(folderConfig)
	if err := v.readConfig(relativeGitPath(v.paths.commonDir, "config"), config); err != nil {
		return err
	}
	worktreeConfig, err := config.boolean("extensions.worktreeconfig")
	if err != nil {
		return err
	}
	if worktreeConfig {
		if err := v.readConfig(relativeGitPath(v.paths.gitDir, "config.worktree"), config); err != nil {
			return err
		}
	} else if v.paths.gitDir != v.paths.commonDir {
		// Git applies these shared overrides only to the main worktree.
		delete(config, "core.bare")
		delete(config, "core.worktree")
	}
	v.config = config
	v.bare, err = config.boolean("core.bare")
	if err != nil {
		return err
	}
	v.workTree = config.get("core.worktree")
	if _, present := config["core.worktree"]; present && !v.bare {
		if !validGitPath(v.workTree) {
			return errFolderUnreadable
		}
		root, canonical, err := openFolderDirectory(v.ctx, relativeGitPath(v.paths.gitDir, v.workTree))
		if err != nil {
			return errFolderUnreadable
		}
		root.Close()
		v.workTree = canonical
		if folderWithin(canonical, v.git) {
			return errFolderUnreadable
		}
	}
	return nil
}

func (v *checkoutView) prepare() error {
	// The private view names this folder as its only alternate, one path per line.
	objectPath := relativeGitPath(v.paths.commonDir, "objects")
	if !validGitPath(objectPath) {
		return errFolderUnreadable
	}
	if err := v.checkObjectFolder(objectPath); err != nil {
		return err
	}
	reftable := v.config.get("extensions.refstorage") == "reftable"
	if reftable && v.paths.gitDir != v.paths.commonDir {
		v.common = "common"
		if err := v.write("commondir", []byte("common\n")); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(v.snapshot.gitDir, v.common, "refs"), 0o700); err != nil {
		return errFolderUnreadable
	}
	if err := v.write(filepath.Join(v.common, "objects/info/alternates"), []byte(objectPath+"\n")); err != nil {
		return err
	}
	if err := v.writeConfig(false); err != nil {
		return err
	}

	head, err := v.headSource()
	if err != nil || head == nil {
		return errFolderUnreadable
	}
	target, symbolic := symbolicFolderRef(head)
	if !symbolic && !validFolderObject(strings.TrimSpace(string(head)), v.hashSize) {
		return errFolderUnreadable
	}
	if symbolic && !strings.HasPrefix(target, "refs/heads/") {
		return errFolderUnreadable
	}
	if err := v.write("HEAD", head); err != nil {
		return err
	}
	if reftable {
		if err := v.copyReftables(v.paths.commonDir, v.common); err != nil {
			return err
		}
		if v.common != "" {
			if err := v.copyReftables(v.paths.gitDir, ""); err != nil {
				return err
			}
		}
	} else {
		if err := v.copySource(relativeGitPath(v.paths.commonDir, "packed-refs"), "packed-refs", maxMetadataBytes); err != nil {
			return err
		}
		if symbolic {
			if err := v.copyLooseRefs(target); err != nil {
				return err
			}
		}
	}

	shallow, err := v.source(relativeGitPath(v.paths.commonDir, "shallow"), maxMetadataBytes)
	if err != nil || shallow == nil {
		return err
	}
	v.shallow = make(map[string]bool)
	for id := range strings.FieldsSeq(string(shallow)) {
		if !validFolderObject(id, v.hashSize) {
			return errFolderUnreadable
		}
		v.shallow[id] = true
	}
	return v.write(filepath.Join(v.common, "shallow"), shallow)
}

const (
	maxObjectFolderEntries = 1 << 20
	maxObjectFolderDepth   = 4
)

// checkObjectFolder admits only the checkout's own object folder as Git's object store.
// The folder must be a real directory below the common Git folder, every entry in it must
// be a regular file or a real directory, and it must name no alternate store. Git then has
// no path from the private view to an object outside the approved folder.
func (v *checkoutView) checkObjectFolder(objectPath string) error {
	parent, name, err := openContainedParent(v.ctx, []string{v.paths.commonDir}, objectPath)
	if err != nil {
		return errFolderUnreadable
	}
	fd, err := openContainedDirectory(v.ctx, parent, name)
	_ = unix.Close(parent)
	if err != nil {
		return errFolderUnreadable
	}
	defer unix.Close(fd)
	walk, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return errFolderUnreadable
	}
	entries := 0
	if err := v.checkObjectEntries(walk, 0, &entries); err != nil {
		return err
	}
	// The scan above showed that info is a real directory, if it exists at all.
	var alternates unix.Stat_t
	err = unix.Fstatat(fd, "info/alternates", &alternates, unix.AT_SYMLINK_NOFOLLOW)
	if !errors.Is(err, syscall.ENOENT) {
		return errFolderUnreadable
	}
	return nil
}

// checkObjectEntries walks one object directory and closes it.
func (v *checkoutView) checkObjectEntries(fd, depth int, entries *int) error {
	dir := os.NewFile(uintptr(fd), "")
	defer dir.Close()
	for {
		list, err := dir.ReadDir(256)
		if err != nil && err != io.EOF || v.ctx.Err() != nil {
			return errFolderUnreadable
		}
		for _, entry := range list {
			*entries++
			if *entries > maxObjectFolderEntries {
				return errFolderUnreadable
			}
			switch entry.Type() {
			case 0:
			case fs.ModeDir:
				if depth >= maxObjectFolderDepth {
					return errFolderUnreadable
				}
				child, err := openContainedDirectory(v.ctx, int(dir.Fd()), entry.Name())
				if err != nil {
					return errFolderUnreadable
				}
				if err := v.checkObjectEntries(child, depth+1, entries); err != nil {
					return err
				}
			default:
				// A link, device, pipe or socket could lead Git outside the folder or stall it.
				return errFolderUnreadable
			}
		}
		if err == io.EOF || len(list) == 0 {
			return nil
		}
	}
}

func symbolicFolderRef(data []byte) (string, bool) {
	target, ok := strings.CutPrefix(string(data), "ref:")
	return strings.TrimSpace(target), ok
}

func validFolderObject(id string, size int) bool {
	return len(id) == 2*size && strings.Trim(id, "0123456789abcdef") == ""
}

func (v *checkoutView) copyLooseRefs(ref string) error {
	for i := 0; i < 8; i++ {
		if !strings.HasPrefix(ref, "refs/") || strings.ContainsAny(ref, "\x00\r\n") {
			return errFolderUnreadable
		}
		// Git's refname rules prevent paths outside refs/ before any source read.
		normalized, err := v.gitOutput(nil, "check-ref-format", "--normalize", ref)
		if err != nil || normalized != ref+"\n" {
			return errFolderUnreadable
		}
		data, err := v.source(relativeGitPath(v.paths.commonDir, ref), maxConfigBytes)
		if err != nil || data == nil {
			return err
		}
		if err := v.write(ref, data); err != nil {
			return err
		}
		next, symbolic := symbolicFolderRef(data)
		if !symbolic {
			if !validFolderObject(strings.TrimSpace(string(data)), v.hashSize) {
				return errFolderUnreadable
			}
			return nil
		}
		ref = next
	}
	return errFolderUnreadable
}

func (v *checkoutView) copyReftables(source, destination string) error {
	list, err := v.source(relativeGitPath(source, "reftable/tables.list"), maxConfigBytes)
	if err != nil || list == nil {
		return errFolderUnreadable
	}
	var total int
	for table := range strings.FieldsSeq(string(list)) {
		if !strings.HasSuffix(table, ".ref") || strings.ContainsAny(table, "/\\\x00") {
			return errFolderUnreadable
		}
		name := "reftable/" + table
		data, err := v.source(relativeGitPath(source, name), maxMetadataBytes-total)
		if err != nil || data == nil {
			return errFolderUnreadable
		}
		total += len(data)
		if err := v.write(filepath.Join(destination, name), data); err != nil {
			return err
		}
	}
	return v.write(filepath.Join(destination, "reftable/tables.list"), list)
}

func (v *checkoutView) facts(root string) (*linkv1.FolderFacts, error) {
	const heads = "refs/heads/"
	const origin = "refs/remotes/origin/"
	const originHead = origin + "HEAD"

	if v.config.get("extensions.refstorage") != "reftable" {
		if err := v.copyLooseRefs(originHead); err != nil {
			return nil, err
		}
	}
	checkedOut, err := v.symbolicRef("HEAD")
	if err != nil {
		return nil, err
	}
	remoteHead, err := v.symbolicRef(originHead)
	if err != nil {
		return nil, err
	}

	main, tip := "main", ""
	if branch, ok := strings.CutPrefix(remoteHead, origin); ok && branch != "" && branch != "HEAD" {
		main = branch
		tip, err = v.resolveRef(remoteHead)
		if err != nil {
			return nil, err
		}
	} else if branch, ok := strings.CutPrefix(checkedOut, heads); ok {
		main = branch
	}

	if tip == "" {
		if v.config.get("extensions.refstorage") != "reftable" {
			if err := v.copyLooseRefs(heads + main); err != nil {
				return nil, err
			}
		}
		tip, err = v.resolveRef(heads + main)
		if err != nil {
			return nil, err
		}
	}
	if tip == "" {
		tip, err = v.resolveRef("HEAD")
		if err != nil {
			return nil, err
		}
	}

	facts := &linkv1.FolderFacts{ResolvedPath: root, Name: filepath.Base(root), MainBranch: main}
	if branch, ok := strings.CutPrefix(checkedOut, heads); ok {
		facts.CheckedOutBranch = &branch
	}
	if !utf8.ValidString(main) || !utf8.ValidString(checkedOut) {
		return nil, errFolderUnusable
	}
	if tip != "" {
		out, err := v.gitOutput(nil, "rev-list", "--max-parents=0", "--end-of-options", tip, "--")
		if err != nil {
			return nil, errFolderUnreadable
		}
		first := ""
		for id := range strings.FieldsSeq(out) {
			if !validFolderObject(id, v.hashSize) {
				return nil, errFolderUnreadable
			}
			if v.shallow[id] {
				first = ""
				break
			}
			if first == "" || id < first {
				first = id
			}
		}
		if first != "" {
			facts.FirstCommit = &first
		}
	}
	if _, configured := v.config["remote.origin.url"]; configured {
		if origin := sanitizeOrigin(v.config.get("remote.origin.url")); origin != "" {
			facts.Remote = &origin
		} else {
			facts.RemoteOmitted = true
		}
	}
	return facts, nil
}

func (v *checkoutView) symbolicRef(ref string) (string, error) {
	out, err := v.gitOutput(nil, "symbolic-ref", "--quiet", "--no-recurse", ref)
	if errors.Is(err, errGitNoResult) {
		return "", nil
	}
	return strings.TrimSuffix(out, "\n"), err
}

func (v *checkoutView) resolveRef(ref string) (string, error) {
	out, err := v.gitOutput(nil, "rev-parse", "--verify", "--quiet", "--end-of-options", ref)
	if errors.Is(err, errGitNoResult) {
		return "", nil
	}
	id := strings.TrimSuffix(out, "\n")
	if err != nil || !validFolderObject(id, v.hashSize) {
		return "", errFolderUnreadable
	}
	return id, nil
}

func captureStatus(ctx context.Context, checkout checkoutPaths) (statusSnapshot, error) {
	view, err := newCheckoutView(ctx, checkout)
	if err != nil || view.bare || view.workTree != "" && view.workTree != checkout.root {
		return statusSnapshot{}, errFolderUnreadable
	}
	if err := view.makeSnapshot(); err != nil {
		return statusSnapshot{}, errFolderUnreadable
	}
	captured := false
	defer func() {
		if !captured {
			view.snapshot.cleanup()
		}
	}()

	if err := view.prepare(); err != nil {
		return statusSnapshot{}, errFolderUnreadable
	}
	if err := view.writeConfig(true); err != nil {
		return statusSnapshot{}, errFolderUnreadable
	}
	if err := view.captureIndex(); err != nil {
		return statusSnapshot{}, errFolderUnreadable
	}
	head, err := view.resolveRef("HEAD")
	if err != nil {
		return statusSnapshot{}, errFolderUnreadable
	}
	if head != "" {
		for _, object := range []string{head + "^{commit}", head + "^{tree}"} {
			if _, err := view.gitOutput(nil, "cat-file", "-e", object); err != nil {
				return statusSnapshot{}, errFolderUnreadable
			}
		}
	}

	if !view.unchanged() {
		return statusSnapshot{}, errFolderUnreadable
	}
	captured = true
	return view.snapshot, nil
}

func (v *checkoutView) captureIndex() error {
	index, err := v.source(v.paths.indexPath, maxMetadataBytes)
	if err != nil || index == nil {
		return err
	}
	if err := v.write("index", index); err != nil {
		return err
	}
	// Copy only candidates referenced by the index bytes. Git validates the split
	// index and identifies its exact backing file in the private directory.
	fd, err := openAnchoredDirectory(v.ctx, v.paths.gitDir)
	if err != nil {
		return errFolderUnreadable
	}
	dir := os.NewFile(uintptr(fd), "")
	defer dir.Close()
	var copied []string
	var total int
	for {
		names, err := dir.Readdirnames(128)
		if err != nil && err != io.EOF || v.ctx.Err() != nil {
			return errFolderUnreadable
		}
		for _, name := range names {
			id, ok := strings.CutPrefix(name, "sharedindex.")
			if !ok || !validFolderObject(id, v.hashSize) {
				continue
			}
			objectID, _ := hex.DecodeString(id)
			if !bytes.Contains(index, objectID) {
				continue
			}
			source := relativeGitPath(v.paths.gitDir, name)
			data, err := v.source(source, maxMetadataBytes-total)
			if err != nil || data == nil {
				return errFolderUnreadable
			}
			total += len(data)
			if err := v.write(name, data); err != nil {
				return err
			}
			copied = append(copied, name)
		}
		if err == io.EOF {
			break
		}
	}
	output, err := v.gitOutput(nil, "rev-parse", "--path-format=absolute", "--shared-index-path")
	if err != nil {
		return errFolderUnreadable
	}
	shared := strings.TrimSuffix(output, "\n")
	if shared != "" && !strings.HasPrefix(shared, v.snapshot.gitDir+"/sharedindex.") {
		return errFolderUnreadable
	}
	found := shared == ""
	for _, name := range copied {
		path := filepath.Join(v.snapshot.gitDir, name)
		if path == shared {
			found = true
		} else if err := os.Remove(path); err != nil {
			return errFolderUnreadable
		}
	}
	if !found {
		return errFolderUnreadable
	}
	return nil
}

func (v *checkoutView) writeConfig(status bool) error {
	c := v.config
	version := c.get("core.repositoryformatversion")
	if version != "" && version != "0" && version != "1" {
		return errFolderUnreadable
	}
	var extensions strings.Builder
	v.hashSize = 20
	for key, value := range c {
		if !strings.HasPrefix(key, "extensions.") {
			continue
		}
		switch key {
		case "extensions.objectformat":
			switch c.get(key) {
			case "sha1":
			case "sha256":
				v.hashSize = 32
			default:
				return errFolderUnreadable
			}
		case "extensions.refstorage":
			if c.get(key) != "files" && c.get(key) != "reftable" {
				return errFolderUnreadable
			}
		case "extensions.partialclone", "extensions.worktreeconfig", "extensions.preciousobjects", "extensions.noop",
			"extensions.relativeworktrees":
			continue
		default:
			if version == "1" {
				return errFolderUnreadable
			}
			continue
		}
		if value != nil {
			extensions.WriteString("\t" + strings.TrimPrefix(key, "extensions.") + " = " + *value + "\n")
		}
	}

	var out strings.Builder
	out.WriteString("[core]\n\tbare = false\n")
	if extensions.Len() != 0 || version == "1" {
		out.WriteString("\trepositoryformatversion = 1\n")
	}
	if status {
		if err := v.writeStatusConfig(&out); err != nil {
			return err
		}
	}
	if extensions.Len() != 0 {
		out.WriteString("[extensions]\n")
		out.WriteString(extensions.String())
	}
	return v.write(filepath.Join(v.common, "config"), []byte(out.String()))
}

func (v *checkoutView) writeStatusConfig(out *strings.Builder) error {
	for _, key := range []string{
		"filemode", "ignorecase", "symlinks", "sparsecheckout", "sparsecheckoutcone",
		"trustctime", "precomposeunicode", "protecthfs", "protectntfs",
	} {
		if _, present := v.config["core."+key]; !present {
			continue
		}
		enabled, err := v.config.boolean("core." + key)
		if err != nil {
			return err
		}
		out.WriteString("\t" + key + " = " + strconv.FormatBool(enabled) + "\n")
	}
	for _, setting := range []struct{ key, values string }{
		{"eol", "lf crlf native"}, {"checkstat", "minimal default"},
	} {
		if _, configured := v.config["core."+setting.key]; configured {
			value := strings.ToLower(v.config.get("core." + setting.key))
			if value == "" || !strings.Contains(" "+setting.values+" ", " "+value+" ") || strings.Contains(value, " ") {
				return errFolderUnreadable
			}
			out.WriteString("\t" + setting.key + " = " + value + "\n")
		}
	}
	if _, configured := v.config["core.autocrlf"]; configured {
		value := "input"
		if !strings.EqualFold(v.config.get("core.autocrlf"), value) {
			enabled, err := v.config.boolean("core.autocrlf")
			if err != nil {
				return err
			}
			value = strconv.FormatBool(enabled)
		}
		out.WriteString("\tautocrlf = " + value + "\n")
	}

	if err := v.copySource(relativeGitPath(v.paths.commonDir, "info/exclude"), filepath.Join(v.common, "info/exclude"), maxMetadataBytes); err != nil {
		return err
	}
	if err := v.copySource(relativeGitPath(v.paths.commonDir, "info/attributes"), filepath.Join(v.common, "info/attributes"), maxMetadataBytes); err != nil {
		return err
	}
	if sparse, _ := v.config.boolean("core.sparsecheckout"); sparse {
		data, err := v.source(relativeGitPath(v.paths.gitDir, "info/sparse-checkout"), maxMetadataBytes)
		if err != nil || data == nil {
			return errFolderUnreadable
		}
		if err := v.write("info/sparse-checkout", data); err != nil {
			return err
		}
	}

	for _, setting := range []struct{ key, fallback, private string }{
		{"excludesfile", "ignore", "info/service-excludes"},
		{"attributesfile", "attributes", "info/service-attributes"},
	} {
		if err := v.copyEffectiveGitFile(out, setting.key, setting.fallback, setting.private); err != nil {
			return err
		}
	}
	if _, configured := v.config["index.sparse"]; configured {
		enabled, err := v.config.boolean("index.sparse")
		if err != nil {
			return err
		}
		out.WriteString("[index]\n\tsparse = " + strconv.FormatBool(enabled) + "\n")
	}
	return nil
}

// copyEffectiveGitFile copies the effective excludes or attributes file into the private view.
// A file that the checkout's configuration selects must lie below the checkout or its Git
// folders, and it is refused otherwise, even when it does not exist. Without that setting,
// the service's own default file is used through the service's own environment.
func (v *checkoutView) copyEffectiveGitFile(out *strings.Builder, key, fallback, private string) error {
	path := v.config.get("core." + key)
	var data []byte
	var err error
	if _, configured := v.config["core."+key]; !configured {
		home, _ := os.UserHomeDir()
		base := os.Getenv("XDG_CONFIG_HOME")
		if !filepath.IsAbs(base) && filepath.IsAbs(home) {
			base = filepath.Join(home, ".config")
		}
		if !filepath.IsAbs(base) {
			return nil
		}
		data, err = v.serviceSource(filepath.Join(base, "git", fallback), maxMetadataBytes)
	} else if !validGitPath(path) || strings.HasPrefix(path, "~") || strings.HasPrefix(path, "%(") {
		// A home-relative or prefix-relative file is outside the checkout by construction.
		return errFolderUnreadable
	} else {
		// The contained reader refuses a path outside the roots before any file system access.
		data, err = v.source(relativeGitPath(v.paths.root, path), maxMetadataBytes)
	}
	if err != nil {
		return err
	}
	// An absent file still pins the setting to the private view's own absent copy.
	if data != nil {
		if err := v.write(private, data); err != nil {
			return err
		}
	}
	target := filepath.Join(v.snapshot.gitDir, private)
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n", "\t", "\\t", "\b", "\\b").Replace(target)
	out.WriteString("\t" + key + " = \"" + escaped + "\"\n")
	return nil
}
