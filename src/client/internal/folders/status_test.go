package folders

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func statusCheckout(t *testing.T) (checkoutFixture, string, string) {
	t.Helper()
	f := newCheckoutFixture(t)
	root := f.repo(t, "checkout", "main")
	writeCheckoutFile(t, filepath.Join(root, "tracked"), []byte("tracked\n"), 0o600)
	f.gitCommand(t, root, "add", "tracked")
	return f, root, f.commit(t, root, "first")
}

// checkStatus preserves the source as a whole, including the index, refs, config and worktree.
func checkStatus(t *testing.T, f checkoutFixture, path string, check func(*linkv1.FolderFacts, linkv1.FolderRefusal)) {
	t.Helper()
	scratch := captureTempDir(t)
	before := checkoutTree(t, f.base)
	defer requireCheckoutTree(t, f.base, before)
	defer requireCaptureTempEmpty(t, scratch)
	facts, refusal := Inspect(t.Context(), path, f.home)
	check(facts, refusal)
}

func requireStatusFacts(t *testing.T, facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal, clean bool) {
	t.Helper()
	if refusal != linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED || facts == nil {
		t.Fatalf("got refusal %s and facts present %t, want folder facts", refusal, facts != nil)
	}
	if facts.GetClean() != clean {
		t.Errorf("got clean %t, want %t", facts.GetClean(), clean)
	}
}

func requireStatusRefusal(t *testing.T, facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal) {
	t.Helper()
	if refusal != linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE || facts != nil {
		t.Errorf("got refusal %s and facts present %t, want the fixed not-readable refusal", refusal, facts != nil)
	}
}

func checkStatusClean(t *testing.T, f checkoutFixture, root string, clean bool) {
	t.Helper()
	checkStatus(t, f, root, func(facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal) {
		requireStatusFacts(t, facts, refusal, clean)
	})
}

func checkSuppressedStatus(t *testing.T, f checkoutFixture, root string, clean bool) {
	t.Helper()
	checkStatus(t, f, root, func(facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal) {
		if refusal != linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED {
			requireStatusRefusal(t, facts, refusal)
			return
		}
		requireStatusFacts(t, facts, refusal, clean)
	})
}

func setStatusExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal("could not set the tracked file's executable bit")
	}
}

func TestInspectFolderFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, checkoutFixture, string)
	}{
		{name: "pristine"},
		{name: "staged", change: func(t *testing.T, f checkoutFixture, root string) {
			writeCheckoutFile(t, filepath.Join(root, "tracked"), []byte("staged\n"), 0o600)
			f.gitCommand(t, root, "add", "tracked")
		}},
		{name: "unstaged", change: func(t *testing.T, _ checkoutFixture, root string) {
			writeCheckoutFile(t, filepath.Join(root, "tracked"), []byte("unstaged\n"), 0o600)
		}},
		{name: "untracked", change: func(t *testing.T, _ checkoutFixture, root string) {
			writeCheckoutFile(t, filepath.Join(root, "new"), []byte("untracked\n"), 0o600)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, root, first := statusCheckout(t)
			const origin = "https://example.invalid/fixture.git"
			f.gitCommand(t, root, "config", "--local", "remote.origin.url", origin)
			f.gitCommand(t, root, "update-ref", "refs/remotes/origin/main", first)
			f.gitCommand(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
			f.gitCommand(t, root, "checkout", "--quiet", "-b", "topic")
			makeCheckoutDir(t, filepath.Join(root, "subfolder"))
			alias := filepath.Join(f.base, "alias")
			linkCheckoutPath(t, root, alias)
			if tc.change != nil {
				tc.change(t, f, root)
			}
			checkStatus(t, f, filepath.Join(alias, "subfolder"), func(facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal) {
				requireStatusFacts(t, facts, refusal, tc.change == nil)
				if facts.GetResolvedPath() != root || facts.GetName() != filepath.Base(root) {
					t.Error("inspection did not identify the physical working tree and its basename")
				}
				checkoutBranches(t, facts, "main", "topic", first)
				if facts.GetRemote() != origin || facts.GetRemoteOmitted() {
					t.Error("inspection did not return the configured origin")
				}
			})
		})
	}

	for _, setting := range []struct {
		value string
		clean bool
	}{
		{"true", false},
		{"false", true},
	} {
		t.Run("filemode "+setting.value, func(t *testing.T) {
			f, root, _ := statusCheckout(t)
			f.gitCommand(t, root, "config", "--local", "core.filemode", setting.value)
			setStatusExecutable(t, filepath.Join(root, "tracked"))
			checkStatusClean(t, f, root, setting.clean)
		})
	}

	for _, source := range []string{"worktree", "local", "configured", "service default"} {
		for _, ignored := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s ignore matches %t", source, ignored), func(t *testing.T) {
				f, root, _ := statusCheckout(t)
				gitDir := filepath.Join(root, ".git")
				var ignorePath string
				switch source {
				case "worktree":
					ignorePath = filepath.Join(root, ".gitignore")
				case "local":
					ignorePath = filepath.Join(makeCheckoutInfo(t, root), "exclude")
				case "configured":
					ignorePath = filepath.Join(gitDir, "configured-excludes")
					f.gitCommand(t, root, "config", "--local", "core.excludesFile", ignorePath)
				case "service default":
					configDir := filepath.Join(f.home, "git")
					makeCheckoutDir(t, configDir)
					ignorePath = filepath.Join(configDir, "ignore")
				}
				writeCheckoutFile(t, ignorePath, []byte("ignored\n"), 0o600)
				if source == "worktree" {
					f.gitCommand(t, root, "add", ".gitignore")
					f.commit(t, root, "track ignore rules")
				}
				name := "unignored"
				if ignored {
					name = "ignored"
				}
				writeCheckoutFile(t, filepath.Join(root, name), []byte("new\n"), 0o600)
				checkStatusClean(t, f, root, ignored)
			})
		}
	}
}

func TestInspectFolderReadOnly(t *testing.T) {
	t.Run("changed assume-unchanged file", func(t *testing.T) {
		f, root, _ := statusCheckout(t)
		f.gitCommand(t, root, "update-index", "--assume-unchanged", "tracked")
		writeCheckoutFile(t, filepath.Join(root, "tracked"), []byte("changed while suppressed\n"), 0o600)
		if f.gitCommand(t, root, "ls-files", "-v", "tracked") != "h tracked" ||
			f.gitCommand(t, root, "--no-optional-locks", "status", "--porcelain") != "" {
			t.Fatal("the fixture does not hide its changed file from ordinary Git status")
		}
		checkSuppressedStatus(t, f, root, false)
	})

	for _, state := range []string{"intentionally absent", "present modified", "present mode changed", "missing inside selection"} {
		t.Run("sparse entry "+state, func(t *testing.T) {
			f, root, _ := statusCheckout(t)
			for _, dir := range []string{"inside", "outside"} {
				makeCheckoutDir(t, filepath.Join(root, dir))
				writeCheckoutFile(t, filepath.Join(root, dir, "tracked"), []byte("original\n"), 0o600)
			}
			f.gitCommand(t, root, "add", "inside", "outside")
			f.commit(t, root, "add sparse paths")
			f.gitCommand(t, root, "config", "--local", "core.filemode", "true")
			f.gitCommand(t, root, "sparse-checkout", "set", "--cone", "inside")
			if f.gitCommand(t, root, "ls-files", "-v", "outside/tracked") != "S outside/tracked" {
				t.Fatal("the fixture's excluded entry lacks the skip-worktree flag")
			}
			outside := filepath.Join(root, "outside", "tracked")
			requireCheckoutAbsent(t, outside)
			switch state {
			case "intentionally absent":
				checkStatusClean(t, f, root, true)
				return
			case "present modified":
				makeCheckoutDir(t, filepath.Dir(outside))
				writeCheckoutFile(t, outside, []byte("modified\n"), 0o600)
			case "present mode changed":
				makeCheckoutDir(t, filepath.Dir(outside))
				writeCheckoutFile(t, outside, []byte("original\n"), 0o600)
				setStatusExecutable(t, outside)
			case "missing inside selection":
				f.gitCommand(t, root, "update-index", "--skip-worktree", "inside/tracked")
				if err := os.Remove(filepath.Join(root, "inside", "tracked")); err != nil {
					t.Fatal("could not remove the suppressed in-cone file")
				}
			}
			checkSuppressedStatus(t, f, root, false)
		})
	}

	for _, state := range []string{"pristine", "changed target", "dirty child"} {
		t.Run("populated submodule "+state, func(t *testing.T) {
			f, root, child := statusSubmodule(t)
			switch state {
			case "changed target":
				f.commit(t, child, "different target")
			case "dirty child":
				writeCheckoutFile(t, filepath.Join(child, "tracked"), []byte("dirty child\n"), 0o600)
			}
			checkStatusClean(t, f, root, state == "pristine")
		})
	}

	t.Run("linked worktree with split index", func(t *testing.T) {
		f, root, _ := statusCheckout(t)
		linked := filepath.Join(f.base, "linked")
		f.gitCommand(t, root, "worktree", "add", "--quiet", "-b", "topic", linked)
		writeCheckoutFile(t, filepath.Join(linked, "linked-only"), []byte("linked\n"), 0o600)
		f.gitCommand(t, linked, "add", "linked-only")
		f.commit(t, linked, "linked worktree head")
		f.gitCommand(t, linked, "update-index", "--split-index")
		if f.gitCommand(t, linked, "rev-parse", "--shared-index-path") == "" {
			t.Fatal("the linked fixture has no shared index")
		}
		checkStatusClean(t, f, linked, true)
		writeCheckoutFile(t, filepath.Join(linked, "linked-only"), []byte("changed in linked worktree\n"), 0o600)
		checkStatusClean(t, f, linked, false)
	})
}

func statusSubmodule(t *testing.T) (checkoutFixture, string, string) {
	t.Helper()
	f, root, _ := statusCheckout(t)
	child := filepath.Join(root, "child")
	childGit := filepath.Join(f.base, "child-metadata")
	makeCheckoutDir(t, child)
	f.gitCommand(t, child, "init", "--quiet", "--template=", "--initial-branch=main", "--separate-git-dir="+childGit)
	writeCheckoutFile(t, filepath.Join(child, "tracked"), []byte("child\n"), 0o600)
	f.gitCommand(t, child, "add", "tracked")
	tip := f.commit(t, child, "first child commit")
	writeCheckoutFile(t, filepath.Join(root, ".gitmodules"), []byte("[submodule \"child\"]\n\tpath = child\n\turl = ../child-source\n"), 0o600)
	f.gitCommand(t, root, "add", ".gitmodules")
	f.gitCommand(t, root, "update-index", "--add", "--cacheinfo", "160000", tip, "child")
	f.commit(t, root, "track child")
	return f, root, child
}

func statusMarkerProgram(t *testing.T, path, marker string, copyInput bool) {
	t.Helper()
	script := "#!/bin/sh\n: > " + quoteCheckoutShell(marker) + "\n"
	if copyInput {
		script += "exec /bin/cat\n"
	}
	writeCheckoutFile(t, path, []byte(script), 0o700)
}

func TestInspectFolderNoSideEffects(t *testing.T) {
	t.Run("checkout executable and included configuration", func(t *testing.T) {
		f, root, first := statusCheckout(t)
		gitDir := filepath.Join(root, ".git")
		fakeGit := filepath.Join(root, "tools")
		makeCheckoutDir(t, fakeGit)
		pathMarker := filepath.Join(f.base, "checkout-git-ran")
		statusMarkerProgram(t, filepath.Join(fakeGit, "git"), pathMarker, false)
		fsmonitorMarker := filepath.Join(f.base, "fsmonitor-ran")
		monitor := filepath.Join(gitDir, "status-monitor")
		statusMarkerProgram(t, monitor, fsmonitorMarker, false)
		f.gitCommand(t, root, "config", "--local", "core.fsmonitor", monitor)

		include := filepath.Join(gitDir, "included-config")
		f.gitCommand(t, root, "config", "--file", include, "core.excludesFile", ".git/included-excludes")
		f.gitCommand(t, root, "config", "--file", include, "remote.origin.url", "https://example.invalid/included")
		const origin = "https://example.invalid/direct"
		f.gitCommand(t, root, "config", "--local", "remote.origin.url", origin)
		f.gitCommand(t, root, "config", "--local", "core.excludesFile", ".git/direct-excludes")
		// Put the include last so the original configuration would hide the new file.
		f.gitCommand(t, root, "config", "--local", "include.path", include)
		if f.gitCommand(t, root, "config", "--local", "--includes", "--get", "core.excludesFile") != ".git/included-excludes" {
			t.Fatal("the fixture's include does not override the inert setting")
		}
		t.Setenv("PATH", fakeGit+string(os.PathListSeparator)+filepath.Dir(f.git)+string(os.PathListSeparator)+os.Getenv("PATH"))
		t.Cleanup(func() {
			requireCheckoutAbsent(t, pathMarker)
			requireCheckoutAbsent(t, fsmonitorMarker)
		})
		// Keep the checkout-local program untracked but ignored by the direct rules.
		writeCheckoutFile(t, filepath.Join(gitDir, "direct-excludes"), []byte("/tools/\n"), 0o600)
		writeCheckoutFile(t, filepath.Join(gitDir, "included-excludes"), []byte("/tools/\nhidden-by-include\n"), 0o600)
		checkStatus(t, f, root, func(facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal) {
			requireStatusFacts(t, facts, refusal, true)
			checkoutBranches(t, facts, "main", "main", first)
			if facts.GetRemote() != origin || facts.GetRemoteOmitted() {
				t.Error("discovery did not use the directly configured origin")
			}
		})
		writeCheckoutFile(t, filepath.Join(root, "hidden-by-include"), []byte("untracked\n"), 0o600)
		checkStatusClean(t, f, root, false)
	})

	t.Run("populated child avoids implicit traversal", func(t *testing.T) {
		f, root, child := statusSubmodule(t)
		childGit := filepath.Join(f.base, "child-metadata")
		marker := filepath.Join(f.base, "child-filter-ran")
		probe := filepath.Join(childGit, "status-filter")
		statusMarkerProgram(t, probe, marker, true)
		attributes := filepath.Join(childGit, "included-attributes")
		writeCheckoutFile(t, attributes, []byte("tracked filter=fixture\n"), 0o600)
		include := filepath.Join(childGit, "included-config")
		f.gitCommand(t, child, "config", "--file", include, "core.attributesFile", attributes)
		f.gitCommand(t, child, "config", "--file", include, "filter.fixture.clean", quoteCheckoutShell(probe))
		f.gitCommand(t, child, "config", "--file", include, "filter.fixture.required", "true")
		f.gitCommand(t, child, "config", "--local", "include.path", include)
		info := filepath.Join(childGit, "info")
		makeCheckoutDir(t, info)
		writeCheckoutFile(t, filepath.Join(info, "exclude"), []byte("ignored-child-file\n"), 0o600)
		writeCheckoutFile(t, filepath.Join(child, "ignored-child-file"), []byte("ignored\n"), 0o600)
		// A changed stat entry makes an implicit child status run the included filter.
		replacement := filepath.Join(child, "replacement")
		writeCheckoutFile(t, replacement, []byte("child\n"), 0o600)
		if err := os.Rename(replacement, filepath.Join(child, "tracked")); err != nil {
			t.Fatal("could not replace the child file with identical content")
		}
		t.Cleanup(func() {
			if _, err := os.Lstat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Error("parent status ran the child's repository-configured program")
			}
		})
		checkStatusClean(t, f, root, true)
	})

	for _, conversionRequired := range []bool{false, true} {
		t.Run(fmt.Sprintf("conversion required %t", conversionRequired), func(t *testing.T) {
			f, root, _ := statusCheckout(t)
			name := "unused"
			if conversionRequired {
				name = "tracked"
			}
			attributes := filepath.Join(root, ".gitattributes")
			writeCheckoutFile(t, attributes, []byte(name+" filter=fixture\n"), 0o600)
			f.gitCommand(t, root, "add", ".gitattributes")
			f.commit(t, root, "track attributes")
			marker := filepath.Join(f.base, "conversion-ran")
			probe := filepath.Join(root, ".git", "status-filter")
			statusMarkerProgram(t, probe, marker, true)
			f.gitCommand(t, root, "config", "--local", "filter.fixture.clean", quoteCheckoutShell(probe))
			f.gitCommand(t, root, "config", "--local", "filter.fixture.required", "true")
			t.Cleanup(func() { requireCheckoutAbsent(t, marker) })
			if !conversionRequired {
				checkStatusClean(t, f, root, true)
				return
			}
			writeCheckoutFile(t, filepath.Join(root, "tracked"), []byte("changed\n"), 0o600)
			checkStatus(t, f, root, func(facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal) {
				requireStatusRefusal(t, facts, refusal)
			})
		})
	}

	t.Run("missing local object never requests transport", func(t *testing.T) {
		f, root, _ := statusCheckout(t)
		tree := f.gitCommand(t, root, "rev-parse", "HEAD^{tree}")
		if len(tree) < 3 {
			t.Fatal("the fixture does not have a tree object")
		}
		marker := filepath.Join(f.base, "transport-requested")
		probe := filepath.Join(f.base, "remote")
		statusMarkerProgram(t, probe, marker, false)
		// The ext transport can only try to launch this local marker program.
		probe = strings.NewReplacer("%", "%%", " ", "% ").Replace(probe)
		f.gitCommand(t, root, "config", "--local", "core.repositoryFormatVersion", "1")
		f.gitCommand(t, root, "config", "--local", "extensions.partialClone", "origin")
		f.gitCommand(t, root, "config", "--local", "remote.origin.promisor", "true")
		f.gitCommand(t, root, "config", "--local", "remote.origin.url", "ext::/bin/sh "+probe)
		f.gitCommand(t, root, "config", "--local", "protocol.ext.allow", "always")
		t.Cleanup(func() { requireCheckoutAbsent(t, marker) })
		if err := os.Remove(filepath.Join(root, ".git", "objects", tree[:2], tree[2:])); err != nil {
			t.Fatal("could not remove the required tree object")
		}
		checkoutMetadata(t, root, f.home, root)
		checkStatus(t, f, root, func(facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal) {
			requireStatusRefusal(t, facts, refusal)
		})
	})

	t.Run("cancellation with a private index", func(t *testing.T) {
		f, root, _ := statusCheckout(t)
		scratch := captureTempDir(t)
		before := checkoutTree(t, f.base)
		defer requireCheckoutTree(t, f.base, before)
		defer requireCaptureTempEmpty(t, scratch)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		ctxWithIndex := &cancelStatusAtIndex{Context: ctx, cancel: cancel, scratch: scratch}
		facts, refusal := Inspect(ctxWithIndex, root, f.home)
		requireStatusRefusal(t, facts, refusal)
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Error("inspection never reached a cancellation point with a private index")
		}
	})
}

// cancelStatusAtIndex cancels on the first context check after the private index appears.
type cancelStatusAtIndex struct {
	context.Context
	cancel  context.CancelFunc
	scratch string
}

func (c *cancelStatusAtIndex) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	entries, _ := os.ReadDir(c.scratch)
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(c.scratch, entry.Name(), "index")); err == nil {
			c.cancel()
			break
		}
	}
	return c.Context.Err()
}

func TestInspectFolderRefusesWhitespaceInLinkedHead(t *testing.T) {
	f, root, first := statusCheckout(t)
	const ref = "refs/heads/main"
	scratch := captureTempDir(t)
	defer requireCaptureTempEmpty(t, scratch)
	linkCheckoutHead(t, f, root, ref)
	_, facts := checkoutMetadata(t, root, f.home, root)
	checkoutBranches(t, facts, "main", "main", first)

	target := ref + " "
	path := filepath.Join(root, ".git", "HEAD")
	if err := os.Remove(path); err != nil {
		t.Fatal("could not remove the fixture HEAD")
	}
	linkCheckoutPath(t, target, path)
	t.Cleanup(func() { requireLinkedHead(t, root, target) })
	cmd := exec.CommandContext(t.Context(), f.git, "--no-optional-locks", "rev-parse", "--verify", "HEAD")
	cmd.Dir = root
	cmd.Env = []string{
		"PATH=" + filepath.Dir(f.git),
		"HOME=" + f.home,
		"XDG_CONFIG_HOME=" + f.home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_ALLOW_PROTOCOL=",
		"LC_ALL=C",
	}
	var exit *exec.ExitError
	if !errors.As(cmd.Run(), &exit) {
		t.Fatal("installed Git did not reject the whitespace-suffixed branch link")
	}
	_, facts, refusal := readCheckout(t.Context(), root, f.home)
	if facts.GetCheckedOutBranch() != "" || facts != nil ||
		refusal != linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE {
		t.Error("discovery must return the fixed not-readable refusal without reporting a branch")
	}
}
