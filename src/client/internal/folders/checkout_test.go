package folders

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// These are the byte limits in the revision 2 contract.
const (
	testTypedPathBytes    = 4096
	testResolvedPathBytes = 8192
	testOriginBytes       = 2048
)

type checkoutFixture struct {
	git, base, home string
}

func newCheckoutFixture(t *testing.T) checkoutFixture {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("the checkout fixture requires installed Git")
	}
	git, err = filepath.Abs(git)
	if err != nil {
		t.Fatal("the checkout fixture requires an absolute Git path")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve the fixture directory: %v", err)
	}
	home := filepath.Join(base, "home")
	makeCheckoutDir(t, home)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	return checkoutFixture{git: git, base: base, home: home}
}

func (f checkoutFixture) gitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.git, args...)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + filepath.Dir(f.git),
		"HOME=" + f.home,
		"XDG_CONFIG_HOME=" + f.home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_AUTHOR_NAME=Fixture",
		"GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=Fixture",
		"GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"LC_ALL=C",
	}
	out, err := cmd.Output()
	if err != nil {
		// Config arguments and Git diagnostics can contain user information.
		t.Fatalf("fixture Git command failed: %T", err)
	}
	return strings.TrimSpace(string(out))
}

func (f checkoutFixture) repo(t *testing.T, name, branch string) string {
	t.Helper()
	root := filepath.Join(f.base, name)
	makeCheckoutDir(t, root)
	f.gitCommand(t, root, "init", "--quiet", "--template=", "--initial-branch="+branch)
	return root
}

func (f checkoutFixture) commit(t *testing.T, root, message string) string {
	t.Helper()
	f.gitCommand(t, root, "commit", "--quiet", "--allow-empty", "-m", message)
	return f.gitCommand(t, root, "rev-parse", "HEAD")
}

func makeCheckoutDir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("make a fixture directory: %v", err)
	}
}

func writeCheckoutFile(t *testing.T, path string, data []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("write a fixture file: %v", err)
	}
}

func readCheckoutFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read a fixture file: %v", err)
	}
	return data
}

func linkCheckoutPath(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("make a fixture link: %v", err)
	}
}

func checkoutRefusal(t *testing.T, typed, home string, want linkv1.FolderRefusal) {
	t.Helper()
	_, facts, refusal := readCheckout(t.Context(), typed, home)
	if refusal != want || facts != nil {
		t.Fatalf("got refusal %s and facts present %t, want %s without facts", refusal, facts != nil, want)
	}
}

func checkoutMetadata(t *testing.T, typed, home, root string) (checkoutPaths, *linkv1.FolderFacts) {
	t.Helper()
	paths, facts, refusal := readCheckout(t.Context(), typed, home)
	if refusal != linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED || facts == nil {
		t.Fatalf("got refusal %s and facts present %t, want metadata", refusal, facts != nil)
	}
	if paths.root != root || facts.GetResolvedPath() != root || facts.GetName() != filepath.Base(root) {
		t.Fatal("metadata did not identify the physical working tree and its basename")
	}
	return paths, facts
}

func checkoutBranches(t *testing.T, facts *linkv1.FolderFacts, main, checkedOut, first string) {
	t.Helper()
	if facts.GetMainBranch() != main {
		t.Error("metadata chose the wrong main branch")
	}
	if (facts.CheckedOutBranch != nil) != (checkedOut != "") || facts.GetCheckedOutBranch() != checkedOut {
		t.Error("metadata reported the wrong checked-out branch or presence")
	}
	if (facts.FirstCommit != nil) != (first != "") || facts.GetFirstCommit() != first {
		t.Error("metadata reported the wrong first commit or presence")
	}
}

func TestInspectFolderMetadata(t *testing.T) {
	t.Run("physical root and aliases", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "topic")
		f.commit(t, root, "root")
		sub := filepath.Join(root, "subdir")
		makeCheckoutDir(t, sub)
		alias := filepath.Join(f.base, "link-to-subdir")
		linkCheckoutPath(t, sub, alias)
		linkCheckoutPath(t, root, filepath.Join(f.home, "source"))
		for _, input := range []string{sub, alias, alias + "/..", "~/source"} {
			checkoutMetadata(t, input, f.home, root)
		}
		checkoutMetadata(t, "~", root, root)
	})

	t.Run("divergent remote head", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "topic")
		local := f.commit(t, root, "local root")
		f.gitCommand(t, root, "checkout", "--quiet", "--orphan", "temporary")
		remote := f.commit(t, root, "remote root")
		tip := f.commit(t, root, "remote tip")
		if f.gitCommand(t, root, "rev-list", "--max-parents=0", tip) != remote || remote == local {
			t.Fatal("the fixture remote branch does not have its own root")
		}
		f.gitCommand(t, root, "update-ref", "refs/remotes/origin/main", tip)
		f.gitCommand(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
		f.gitCommand(t, root, "checkout", "--quiet", "topic")
		f.gitCommand(t, root, "branch", "-D", "temporary")
		_, facts := checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "main", "topic", remote)
	})

	t.Run("remote head without its tracking ref", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "main")
		main := f.commit(t, root, "selected local root")
		f.gitCommand(t, root, "checkout", "--quiet", "--orphan", "topic")
		f.commit(t, root, "checked-out root")
		f.gitCommand(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
		_, facts := checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "main", "topic", main)
	})

	t.Run("local branch and detached fallback", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "topic")
		first := f.commit(t, root, "first")
		tip := f.commit(t, root, "tip")
		_, facts := checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "topic", "topic", first)
		f.gitCommand(t, root, "checkout", "--quiet", "--detach", tip)
		_, facts = checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "main", "", first)
	})

	t.Run("unborn history", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "unborn")
		_, facts := checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "unborn", "unborn", "")
	})

	t.Run("shallow history", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "topic")
		f.commit(t, root, "unavailable root")
		tip := f.commit(t, root, "shallow boundary")
		writeCheckoutFile(t, filepath.Join(root, ".git", "shallow"), []byte(tip+"\n"), 0o600)
		if f.gitCommand(t, root, "rev-parse", "--is-shallow-repository") != "true" {
			t.Fatal("the fixture did not have a shallow boundary")
		}
		_, facts := checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "topic", "topic", "")
	})

	t.Run("multiple roots", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "topic")
		first := f.commit(t, root, "one root")
		f.gitCommand(t, root, "checkout", "--quiet", "--orphan", "other")
		second := f.commit(t, root, "another root")
		low, high := min(first, second), max(first, second)
		tree := f.gitCommand(t, root, "rev-parse", "HEAD^{tree}")
		merge := f.gitCommand(t, root, "commit-tree", tree, "-p", high, "-p", low, "-m", "two roots")
		f.gitCommand(t, root, "update-ref", "refs/heads/topic", merge)
		f.gitCommand(t, root, "checkout", "--quiet", "topic")
		_, facts := checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "topic", "topic", low)
	})

	t.Run("checkout and service environment are isolated", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "topic")
		first := f.commit(t, root, "fixture root")
		decoy := f.repo(t, "decoy", "other")
		f.commit(t, decoy, "other root")

		localBin := filepath.Join(root, "bin")
		unsafeBin := filepath.Join(f.base, "writable", "bin")
		makeCheckoutDir(t, localBin)
		makeCheckoutDir(t, filepath.Dir(unsafeBin))
		makeCheckoutDir(t, unsafeBin)
		marker := filepath.Join(f.base, "executed")
		probe := "#!/bin/sh\n: > " + quoteCheckoutShell(marker) + "\nexit 1\n"
		writeCheckoutFile(t, filepath.Join(localBin, "git"), []byte(probe), 0o555)
		writeCheckoutFile(t, filepath.Join(unsafeBin, "git"), []byte(probe), 0o555)

		t.Setenv("PATH", strings.Join([]string{localBin, unsafeBin, filepath.Dir(f.git)}, string(os.PathListSeparator)))
		t.Setenv("GIT_DIR", filepath.Join(decoy, ".git"))
		t.Setenv("GIT_WORK_TREE", decoy)
		t.Setenv("GIT_CONFIG_COUNT", "1")
		t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
		t.Setenv("GIT_CONFIG_VALUE_0", "https://example.invalid/inherited")
		t.Cleanup(func() { requireCheckoutAbsent(t, marker) })
		_, facts := checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "topic", "topic", first)
		if facts.Remote != nil || facts.GetRemoteOmitted() {
			t.Error("inspection used an inherited origin")
		}
	})
}

func TestInspectFolderRefusals(t *testing.T) {
	f := newCheckoutFixture(t)
	root := f.repo(t, "checkout", "topic")
	bare := filepath.Join(f.base, "bare")
	f.gitCommand(t, f.base, "init", "--quiet", "--template=", "--bare", bare)
	plain := filepath.Join(f.base, "plain")
	makeCheckoutDir(t, plain)
	file := filepath.Join(f.base, "file")
	writeCheckoutFile(t, file, nil, 0o600)
	loop := filepath.Join(f.base, "loop")
	linkCheckoutPath(t, loop, loop)
	missing := filepath.Join(f.base, "missing")
	dangling := filepath.Join(f.base, "dangling")
	linkCheckoutPath(t, missing, dangling)
	denied := filepath.Join(f.base, "denied")
	makeCheckoutDir(t, denied)
	if err := os.Chmod(denied, 0); err != nil {
		t.Fatal("could not deny access to the fixture directory")
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o700) })
	if _, err := os.ReadDir(denied); !errors.Is(err, fs.ErrPermission) {
		t.Fatal("the refusal fixture must run as an ordinary service user")
	}

	pathNotUsable := linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE
	notFolder := linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER
	notRepository := linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY
	notReadable := linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE
	for _, tc := range []struct {
		name, path string
		want       linkv1.FolderRefusal
	}{
		{"relative", "checkout", pathNotUsable},
		{"other tilde expansion", "~someone/checkout", pathNotUsable},
		{"invalid UTF-8", root + "/\xff", pathNotUsable},
		{"zero byte", root + "\x00", pathNotUsable},
		{"symlink loop", loop, pathNotUsable},
		{"absent", missing, notFolder},
		{"dangling link", dangling, notFolder},
		{"non-directory", file, notFolder},
		{"bare repository", bare, notRepository},
		{"plain directory", plain, notRepository},
		{"denied read", denied, notReadable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkoutRefusal(t, tc.path, f.home, tc.want)
		})
	}

	t.Run("no usable Git", func(t *testing.T) {
		t.Setenv("PATH", plain)
		checkoutRefusal(t, root, f.home, notReadable)
	})

	t.Run("typed byte boundary", func(t *testing.T) {
		alias := "界"
		linkCheckoutPath(t, root, filepath.Join(f.home, alias))
		exact := "~/" + strings.Repeat("/", testTypedPathBytes-len("~/"+alias)) + alias
		checkoutMetadata(t, exact, f.home, root)
		checkoutRefusal(t, exact+"/", f.home, pathNotUsable)
	})

	for _, length := range []int{testResolvedPathBytes, testResolvedPathBytes + 1} {
		t.Run(fmt.Sprintf("resolved path of %d bytes", length), func(t *testing.T) {
			alias, physical := deepCheckout(t, length)
			if length == testResolvedPathBytes {
				checkoutMetadata(t, alias, f.home, physical)
			} else {
				checkoutRefusal(t, alias, f.home, pathNotUsable)
			}
		})
	}
}

func deepCheckout(t *testing.T, length int) (string, string) {
	t.Helper()
	f := newCheckoutFixture(t)
	const name = "checkout"
	source := f.repo(t, name, "main")
	dir, err := os.OpenRoot(f.base)
	if err != nil {
		t.Fatalf("open the fixture directory: %v", err)
	}
	defer dir.Close()

	// Short relative links spread across the tree keep the typed path short.
	// File-descriptor-relative operations build the physical path across the system buffer limit.
	var physical, alias string
	remaining := length - len(f.base) - len("/"+name)
	for remaining > 0 {
		var components []string
		for len(components) < 3 && remaining > 0 {
			step := min(remaining, 241)
			if remaining-step == 1 {
				step--
			}
			components = append(components, strings.Repeat("d", step-1))
			remaining -= step
		}
		segment := filepath.Join(components...)
		next := filepath.Join(physical, segment)
		requireDeepCheckoutFixture(t, dir.MkdirAll(next, 0o700))
		requireDeepCheckoutFixture(t, dir.Symlink(segment, filepath.Join(physical, "next")))
		physical = next
		alias = filepath.Join(alias, "next")
	}
	physical = filepath.Join(physical, name)
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(physical, relative)
		if entry.IsDir() {
			return dir.Mkdir(target, 0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return dir.WriteFile(target, data, 0o600)
	})
	requireDeepCheckoutFixture(t, err)
	alias = filepath.Join(f.base, alias, name)
	_, err = os.Stat(filepath.Join(alias, ".git", "HEAD"))
	requireDeepCheckoutFixture(t, err)
	return alias, filepath.Join(f.base, physical)
}

func requireDeepCheckoutFixture(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, syscall.ENAMETOOLONG) {
		t.Skip("the filesystem cannot hold a checkout at the resolved-path byte boundary")
	}
	if err != nil {
		t.Fatalf("could not build the resolved-path boundary fixture: %T", err)
	}
}

func TestInspectFolderOrigin(t *testing.T) {
	f := newCheckoutFixture(t)
	root := f.repo(t, "checkout", "main")
	const origin = "https://example.invalid/"
	const userInfo = "fixture-user:fixture-password@"
	atLimit := origin + strings.Repeat("a", testOriginBytes-len(origin))
	for _, tc := range []struct {
		name, input, want string
		omitted           bool
	}{
		{name: "unconfigured"},
		{"configured empty URL", "", "", true},
		{"HTTP", "http://example.invalid/Team/Repo/", "http://example.invalid/Team/Repo/", false},
		{"HTTPS credentials", "https://" + userInfo + "example.invalid:8443/Team/Repo.git", "https://example.invalid:8443/Team/Repo.git", false},
		{"HTTPS username", "https://fixture-user@example.invalid/Team/Repo", "https://example.invalid/Team/Repo", false},
		{"Git transport", "git://example.invalid/Team/Repo.git", "git://example.invalid/Team/Repo.git", false},
		{"SSH URL", "ssh://fixture-user@example.invalid:2222/Team/Repo.git", "ssh://example.invalid:2222/Team/Repo.git", false},
		{"absolute SSH shorthand", "fixture-user@example.invalid:/Team/Repo.git", "example.invalid:/Team/Repo.git", false},
		{"account-relative shorthand", "fixture-user@example.invalid:Team/Repo.git", "", true},
		{"account-relative tilde shorthand", "fixture-user@example.invalid:~/Repo.git", "", true},
		{"account-relative SSH URL", "ssh://example.invalid/~/repo", "", true},
		{"named SSH home in URL", "ssh://example.invalid/~fixture-user/repo", "", true},
		{"absolute local path", "/Team/Repo.git", "", true},
		{"relative local path", "../Repo.git", "", true},
		{"file URL", "file:///Team/Repo.git", "", true},
		{"ambiguous address", "example.invalid:Team/Repo.git", "", true},
		{"unsupported scheme", "ftp://example.invalid/Team/Repo.git", "", true},
		{"missing host", "https:///Team/Repo.git", "", true},
		{"query", "https://example.invalid/repo?value=one", "", true},
		{"fragment", "https://example.invalid/repo#one", "", true},
		{"escaped path", "https://example.invalid/Team%2FRepo", "", true},
		{"single-dot segment", "https://example.invalid/Team/./Repo", "", true},
		{"parent segment", "ssh://example.invalid/Team/../Repo", "", true},
		{"exact limit after stripping", "https://" + userInfo + strings.TrimPrefix(atLimit, "https://"), atLimit, false},
		{"over limit after stripping", "https://" + userInfo + strings.TrimPrefix(atLimit, "https://") + "a", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name != "unconfigured" {
				f.gitCommand(t, root, "config", "--local", "remote.origin.url", tc.input)
			}
			_, facts := checkoutMetadata(t, root, f.home, root)
			if facts.GetRemoteOmitted() != tc.omitted || (facts.Remote != nil) != (tc.want != "") {
				t.Errorf("origin omission and field presence disagree with the contract")
			}
			if facts.GetRemote() != tc.want {
				t.Error("origin did not preserve exactly the permitted credential-free address")
			}
		})
	}
}

func TestCheckoutSnapshot(t *testing.T) {
	t.Run("linked worktree split index and inert configuration", func(t *testing.T) {
		f := newCheckoutFixture(t)
		main := f.repo(t, "main", "main")
		writeCheckoutFile(t, filepath.Join(main, "tracked"), []byte("committed\n"), 0o600)
		f.gitCommand(t, main, "add", "tracked")
		head := f.commit(t, main, "root")
		linked := filepath.Join(f.base, "linked")
		f.gitCommand(t, main, "worktree", "add", "--quiet", "-b", "topic", linked)
		linkedHead := f.commit(t, linked, "linked worktree head")
		if linkedHead == head {
			t.Fatal("the linked worktree and main checkout have the same HEAD")
		}

		writeCheckoutFile(t, filepath.Join(linked, "linked-only"), []byte("linked\n"), 0o600)
		f.gitCommand(t, linked, "add", "linked-only")
		f.gitCommand(t, linked, "update-index", "--split-index")
		linkedEntries := f.gitCommand(t, linked, "ls-files", "--stage")
		commonEntries := f.gitCommand(t, main, "ls-files", "--stage")
		if linkedEntries == commonEntries || !strings.Contains(linkedEntries, "\tlinked-only") ||
			!strings.Contains(linkedEntries, "\ttracked") || f.gitCommand(t, linked, "rev-parse", "--shared-index-path") == "" {
			t.Fatal("the fixture does not require both the worktree index and its shared index")
		}

		commonDir := filepath.Join(main, ".git")
		gitDir := f.gitCommand(t, linked, "rev-parse", "--absolute-git-dir")
		index := f.gitCommand(t, linked, "rev-parse", "--path-format=absolute", "--git-path", "index")
		f.gitCommand(t, main, "config", "--local", "extensions.worktreeConfig", "true")
		f.gitCommand(t, main, "config", "--local", "core.filemode", "true")
		f.gitCommand(t, main, "config", "--local", "core.ignorecase", "false")
		f.gitCommand(t, main, "config", "--local", "core.symlinks", "false")
		f.gitCommand(t, linked, "config", "--worktree", "core.filemode", "false")
		f.gitCommand(t, linked, "config", "--worktree", "core.ignorecase", "true")
		writeCheckoutFile(t, filepath.Join(linked, ".gitignore"), []byte("tree-ignored\n"), 0o600)
		if err := os.MkdirAll(filepath.Join(commonDir, "info"), 0o700); err != nil {
			t.Fatal("could not make the fixture exclude directory")
		}
		writeCheckoutFile(t, filepath.Join(commonDir, "info", "exclude"), []byte("local-ignored\n"), 0o600)

		scratch := captureTempDir(t)
		before := checkoutTree(t, f.base)
		t.Cleanup(func() { requireCheckoutTree(t, f.base, before) })
		paths := checkoutPaths{root: linked, gitDir: gitDir, commonDir: commonDir, indexPath: index}
		discovered, facts, refusal := readCheckout(t.Context(), linked, f.home)
		if refusal != linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED || facts == nil {
			t.Error("discovery did not return metadata for the linked worktree")
		} else {
			checkoutBranches(t, facts, "topic", "topic", head)
		}
		if discovered != paths {
			t.Error("discovery did not identify the linked worktree metadata and per-worktree index")
		}
		withCheckoutSnapshot(t, paths, scratch, func(snapshot statusSnapshot) {
			if f.snapshotGit(t, snapshot, "rev-parse", "HEAD") != linkedHead ||
				f.snapshotGit(t, snapshot, "cat-file", "-t", linkedHead) != "commit" {
				t.Error("the snapshot did not retain HEAD and access to its local commit")
			}
			if f.snapshotGit(t, snapshot, "ls-files", "--stage") != linkedEntries {
				t.Error("the snapshot did not retain the worktree's complete split index")
			}
			privateIndex := f.snapshotGit(t, snapshot, "rev-parse", "--path-format=absolute", "--git-path", "index")
			if !checkoutPathWithin(snapshot.gitDir, privateIndex) {
				t.Fatal("the status index is outside the private view")
			}
			privateInfo, privateErr := os.Stat(privateIndex)
			sourceInfo, sourceErr := os.Stat(index)
			if privateErr != nil || sourceErr != nil || os.SameFile(privateInfo, sourceInfo) {
				t.Error("the status view does not have a separate index file")
			}
			for _, setting := range []struct{ name, value string }{
				{"core.filemode", "false"}, {"core.ignorecase", "true"}, {"core.symlinks", "false"},
			} {
				if f.snapshotGit(t, snapshot, "config", "--bool", "--get", setting.name) != setting.value {
					t.Errorf("the snapshot changed the effective %s setting", setting.name)
				}
			}
			wantIgnored := "tree-ignored\nlocal-ignored"
			gotIgnored := f.snapshotGit(t, snapshot, "check-ignore", "--no-index", "tree-ignored", "local-ignored")
			if gotIgnored != wantIgnored {
				t.Error("the snapshot did not preserve all three ignore sources")
			}
		})
	})

	t.Run("sparse selection", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "main")
		for _, name := range []string{"included", "excluded"} {
			makeCheckoutDir(t, filepath.Join(root, name))
			writeCheckoutFile(t, filepath.Join(root, name, "tracked"), []byte(name), 0o600)
		}
		f.gitCommand(t, root, "add", ".")
		f.commit(t, root, "root")
		f.gitCommand(t, root, "sparse-checkout", "set", "--cone", "included")
		requireCheckoutAbsent(t, filepath.Join(root, "excluded", "tracked"))
		want := f.gitCommand(t, root, "sparse-checkout", "list")
		if want != "included" {
			t.Fatal("the fixture does not have the intended sparse selection")
		}
		scratch := captureTempDir(t)
		before := checkoutTree(t, f.base)
		t.Cleanup(func() { requireCheckoutTree(t, f.base, before) })
		paths := fixtureCheckoutPaths(root)
		withCheckoutSnapshot(t, paths, scratch, func(snapshot statusSnapshot) {
			if f.snapshotGit(t, snapshot, "sparse-checkout", "list") != want {
				t.Error("the snapshot did not preserve the sparse selection")
			}
		})
	})

	t.Run("does not load executable configuration", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "topic")
		writeCheckoutFile(t, filepath.Join(root, "tracked"), []byte("tracked"), 0o600)
		f.gitCommand(t, root, "add", "tracked")
		f.commit(t, root, "first")
		const origin = "https://example.invalid/local"
		f.gitCommand(t, root, "config", "--local", "remote.origin.url", origin)

		marker := filepath.Join(f.base, "executed")
		program := filepath.Join(root, "probe.sh")
		writeCheckoutFile(t, program, []byte("#!/bin/sh\n: > "+quoteCheckoutShell(marker)+"\n"), 0o700)
		included := filepath.Join(f.base, "included-config")
		writeCheckoutFile(t, included, []byte("[remote \"origin\"]\n\turl = https://example.invalid/included\n"), 0o600)
		f.gitCommand(t, root, "config", "--local", "core.fsmonitor", program)
		f.gitCommand(t, root, "config", "--local", "include.path", included)
		if f.gitCommand(t, root, "config", "--local", "--includes", "--get", "remote.origin.url") == origin {
			t.Fatal("loading the fixture include would not change the origin")
		}

		scratch := captureTempDir(t)
		t.Cleanup(func() { requireCheckoutAbsent(t, marker) })
		before := checkoutTree(t, f.base)
		t.Cleanup(func() { requireCheckoutTree(t, f.base, before) })
		t.Run("discovery", func(t *testing.T) {
			_, facts, refusal := readCheckout(t.Context(), root, f.home)
			if refusal != linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED || facts.GetRemote() != origin || facts.GetRemoteOmitted() {
				t.Error("discovery did not retain the local origin independently of the include")
			}
		})
		t.Run("private view", func(t *testing.T) {
			withCheckoutSnapshot(t, fixtureCheckoutPaths(root), scratch, func(snapshot statusSnapshot) {
				f.snapshotGit(t, snapshot, "status", "--porcelain=v1", "--ignore-submodules=all")
				got := f.snapshotGit(t, snapshot, "config", "--includes", "--list")
				if strings.Contains(got, program) || strings.Contains(got, included) {
					t.Error("the private view retains executable configuration")
				}
			})
		})
	})

	t.Run("missing local object with a promisor remote", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "main")
		head := f.commit(t, root, "root")
		var requests atomic.Int32
		peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			http.NotFound(w, r)
		}))
		t.Cleanup(peer.Close)
		t.Cleanup(func() {
			if requests.Load() != 0 {
				t.Error("capture sent a request for a missing local object")
			}
		})
		f.gitCommand(t, root, "config", "--local", "core.repositoryFormatVersion", "1")
		f.gitCommand(t, root, "config", "--local", "extensions.partialClone", "origin")
		f.gitCommand(t, root, "config", "--local", "remote.origin.promisor", "true")
		f.gitCommand(t, root, "config", "--local", "remote.origin.url", peer.URL+"/fixture")
		scratch := captureTempDir(t)
		paths := fixtureCheckoutPaths(root)
		if err := os.Remove(filepath.Join(root, ".git", "objects", head[:2], head[2:])); err != nil {
			t.Fatal("could not remove the required local commit")
		}
		before := checkoutTree(t, f.base)
		t.Cleanup(func() { requireCheckoutTree(t, f.base, before) })
		requireCaptureError(t, t.Context(), paths, scratch)
	})

	t.Run("unreadable ignore file", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "main")
		f.commit(t, root, "root")
		excludes := filepath.Join(f.home, "required-excludes")
		writeCheckoutFile(t, excludes, []byte("ignored\n"), 0o600)
		f.gitCommand(t, root, "config", "--local", "core.excludesFile", excludes)
		scratch := captureTempDir(t)
		paths := fixtureCheckoutPaths(root)
		if err := os.Chmod(excludes, 0); err != nil {
			t.Fatal("could not deny access to the required ignore file")
		}
		t.Cleanup(func() { _ = os.Chmod(excludes, 0o600) })
		if _, err := os.ReadFile(excludes); !errors.Is(err, fs.ErrPermission) {
			t.Fatal("the ignore fixture must run as an ordinary service user")
		}
		before := checkoutTree(t, root)
		t.Cleanup(func() { requireCheckoutTree(t, root, before) })
		requireCaptureError(t, t.Context(), paths, scratch)
	})

	t.Run("cancellation cleans up", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "main")
		f.commit(t, root, "root")
		scratch := captureTempDir(t)
		before := checkoutTree(t, f.base)
		t.Cleanup(func() { requireCheckoutTree(t, f.base, before) })
		paths := fixtureCheckoutPaths(root)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		requireCaptureError(t, ctx, paths, scratch)
	})

	t.Run("index changes during capture", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "main")
		writeCheckoutFile(t, filepath.Join(root, "tracked"), []byte("original"), 0o600)
		f.gitCommand(t, root, "add", "tracked")
		f.commit(t, root, "root")
		index := filepath.Join(root, ".git", "index")
		original := readCheckoutFile(t, index)
		wantOriginal := f.gitCommand(t, root, "ls-files", "--stage")
		writeCheckoutFile(t, filepath.Join(root, "added"), []byte("staged"), 0o600)
		f.gitCommand(t, root, "add", "added")
		changed := readCheckoutFile(t, index)
		wantChanged := f.gitCommand(t, root, "ls-files", "--stage")
		if wantOriginal == wantChanged {
			t.Fatal("the two fixture indexes have the same entries")
		}
		paths := fixtureCheckoutPaths(root)
		checkCaptureWhileReplacing(t, f, paths, index, original, changed, func(snapshot statusSnapshot) {
			got := f.snapshotGit(t, snapshot, "ls-files", "--stage")
			if got != wantOriginal && got != wantChanged {
				t.Error("capture did not retain either coherent index")
			}
		})
	})

	t.Run("configuration changes during capture", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "main")
		f.commit(t, root, "root")
		config := filepath.Join(root, ".git", "config")
		f.gitCommand(t, root, "config", "--local", "core.filemode", "true")
		f.gitCommand(t, root, "config", "--local", "core.ignorecase", "false")
		original := readCheckoutFile(t, config)
		f.gitCommand(t, root, "config", "--local", "core.filemode", "false")
		f.gitCommand(t, root, "config", "--local", "core.ignorecase", "true")
		changed := readCheckoutFile(t, config)
		paths := fixtureCheckoutPaths(root)
		checkCaptureWhileReplacing(t, f, paths, config, original, changed, func(snapshot statusSnapshot) {
			mode := f.snapshotGit(t, snapshot, "config", "--bool", "--get", "core.filemode")
			ignoreCase := f.snapshotGit(t, snapshot, "config", "--bool", "--get", "core.ignorecase")
			if !(mode == "true" && ignoreCase == "false" || mode == "false" && ignoreCase == "true") {
				t.Error("capture combined settings from different versions of the source configuration")
			}
		})
	})
}

func fixtureCheckoutPaths(root string) checkoutPaths {
	gitDir := filepath.Join(root, ".git")
	return checkoutPaths{root: root, gitDir: gitDir, commonDir: gitDir, indexPath: filepath.Join(gitDir, "index")}
}

func quoteCheckoutShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (f checkoutFixture) snapshotGit(t *testing.T, snapshot statusSnapshot, args ...string) string {
	t.Helper()
	prefix := []string{"--no-optional-locks", "--git-dir=" + snapshot.gitDir, "--work-tree=" + snapshot.root}
	return f.gitCommand(t, snapshot.root, append(prefix, args...)...)
}

func captureTempDir(t *testing.T) string {
	t.Helper()
	scratch := t.TempDir()
	t.Setenv("TMPDIR", scratch)
	return scratch
}

func requireCheckoutAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Error("a marker or private checkout view remains on disk")
	}
}

func requireCaptureTempEmpty(t *testing.T, scratch string) {
	t.Helper()
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Error("capture left files in the temporary directory")
	}
}

func withCheckoutSnapshot(t *testing.T, paths checkoutPaths, scratch string, check func(statusSnapshot)) {
	t.Helper()
	snapshot, err := captureStatus(t.Context(), paths)
	if err != nil {
		t.Fatalf("capture did not provide a private view: %T", err)
	}
	checkPrivateCheckout(t, paths, snapshot, scratch, check)
}

func checkPrivateCheckout(t *testing.T, paths checkoutPaths, snapshot statusSnapshot, scratch string, check func(statusSnapshot)) {
	t.Helper()
	if snapshot.root != paths.root || snapshot.gitDir == "" || snapshot.cleanup == nil {
		t.Fatal("capture did not return the worktree and a cleanable private view")
	}
	physical, err := filepath.EvalSymlinks(snapshot.gitDir)
	if err != nil || !filepath.IsAbs(physical) {
		t.Fatal("capture did not create an absolute private Git directory")
	}
	physicalScratch, err := filepath.EvalSymlinks(scratch)
	if err != nil || !checkoutPathWithin(physicalScratch, physical) {
		t.Fatal("the Git status view is outside the test's temporary directory")
	}
	defer func() {
		snapshot.cleanup()
		requireCheckoutAbsent(t, snapshot.gitDir)
		requireCaptureTempEmpty(t, scratch)
	}()
	for _, original := range []string{paths.root, paths.gitDir, paths.commonDir} {
		if checkoutPathWithin(original, physical) {
			t.Fatal("the Git status view is inside the original checkout or Git metadata")
		}
	}
	check(snapshot)
}

func checkoutPathWithin(parent, path string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, "../")
}

func requireCaptureError(t *testing.T, ctx context.Context, paths checkoutPaths, scratch string) {
	t.Helper()
	snapshot, err := captureStatus(ctx, paths)
	if err == nil {
		t.Fatal("capture returned a status view when it should refuse")
	}
	if snapshot.gitDir != "" {
		requireCheckoutAbsent(t, snapshot.gitDir)
	}
	requireCaptureTempEmpty(t, scratch)
}

type checkoutFileState struct {
	mode fs.FileMode
	hash [sha256.Size]byte
}

func checkoutTree(t *testing.T, root string) map[string]checkoutFileState {
	t.Helper()
	files := make(map[string]checkoutFileState)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		state := checkoutFileState{mode: info.Mode()}
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			state.hash = sha256.Sum256(data)
		}
		files[path] = state
		return nil
	})
	if err != nil {
		t.Fatalf("could not fingerprint the checkout fixture: %T", err)
	}
	return files
}

func requireCheckoutTree(t *testing.T, root string, before map[string]checkoutFileState) {
	t.Helper()
	if !reflect.DeepEqual(checkoutTree(t, root), before) {
		t.Error("capture changed source files or modes")
	}
}

func checkCaptureWhileReplacing(t *testing.T, f checkoutFixture, paths checkoutPaths, file string, original, changed []byte, check func(statusSnapshot)) {
	t.Helper()
	scratch := captureTempDir(t)
	before := checkoutTree(t, paths.root)
	delete(before, file)
	t.Cleanup(func() {
		after := checkoutTree(t, paths.root)
		delete(after, file)
		if !reflect.DeepEqual(after, before) {
			t.Error("capture changed source files outside the fixture writer's file")
		}
	})

	// Atomic replacement gives capture two valid versions and no torn input from the writer.
	var replacements atomic.Int64
	ready := make(chan struct{})
	done := make(chan struct{})
	var replaceErr error
	ctx, cancel := context.WithCancel(t.Context())
	defer func() {
		cancel()
		<-done
		if replaceErr != nil {
			t.Errorf("the fixture writer could not replace its file: %T", replaceErr)
		}
		got := readCheckoutFile(t, file)
		if !bytes.Equal(got, original) && !bytes.Equal(got, changed) {
			t.Error("the source file contains bytes that the fixture writer never installed")
		}
	}()
	go func() {
		defer close(done)
		temp := filepath.Join(f.base, "replacement")
		defer func() { _ = os.Remove(temp) }()
		for n := 0; ctx.Err() == nil; n++ {
			data := original
			if n%2 != 0 {
				data = changed
			}
			if err := os.WriteFile(temp, data, 0o600); err != nil {
				replaceErr = err
				return
			}
			if err := os.Rename(temp, file); err != nil {
				replaceErr = err
				return
			}
			replacements.Add(1)
			if n == 0 {
				close(ready)
			}
		}
	}()
	select {
	case <-ready:
	case <-done:
		t.Fatal("the fixture writer stopped before replacing the source")
	case <-t.Context().Done():
		t.Fatal("the fixture writer did not start")
	}

	for attempt := 0; attempt < 32; attempt++ {
		start := replacements.Load()
		snapshot, err := captureStatus(t.Context(), paths)
		overlapped := replacements.Load() != start
		if err == nil {
			checkPrivateCheckout(t, paths, snapshot, scratch, check)
		} else {
			if snapshot.gitDir != "" {
				requireCheckoutAbsent(t, snapshot.gitDir)
			}
			requireCaptureTempEmpty(t, scratch)
		}
		if overlapped {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("capture did not observe a source replacement during any attempt")
}
