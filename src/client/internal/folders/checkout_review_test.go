package folders

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
)

// These tests cover the repair of item 4 after its review (post-4 1 to post-4 4).
// They reuse the fixtures of checkout_test.go, which stays unchanged.

// Fixed times let a test see whether a read updated a file's access time.
var (
	outsideAccessStamp = time.Unix(1_000_000_000, 0)
	outsideModifyStamp = time.Unix(1_000_000_600, 0)
)

// outsideReads observes whether inspection read a file through the file system.
// A read moves the access time away from an older fixed stamp on file systems that
// record access times; the observer first checks that this one does.
type outsideReads struct {
	tracked bool
}

func newOutsideReads(t *testing.T, dir string) *outsideReads {
	t.Helper()
	probe := filepath.Join(dir, "access-probe")
	writeCheckoutFile(t, probe, []byte("probe\n"), 0o600)
	stampOutsideFile(t, probe)
	readCheckoutFile(t, probe)
	tracked := !outsideAccessTime(t, probe).Equal(outsideAccessStamp)
	if err := os.Remove(probe); err != nil {
		t.Fatal("could not remove the access-time probe")
	}
	if !tracked {
		t.Log("this file system does not record reads; the test observes refusals and private views only")
	}
	return &outsideReads{tracked: tracked}
}

func stampOutsideFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Chtimes(path, outsideAccessStamp, outsideModifyStamp); err != nil {
		t.Fatal("could not set the fixed access time of an outside file")
	}
}

func outsideAccessTime(t *testing.T, path string) time.Time {
	t.Helper()
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		t.Fatal("could not read the access time of an outside file")
	}
	return time.Unix(stat.Atim.Unix())
}

// watch stamps a file after the fixture's own Git commands, which may read it, are done.
// The check runs at cleanup, so it also runs after a fatal assertion.
func (o *outsideReads) watch(t *testing.T, path string) {
	t.Helper()
	want := readCheckoutFile(t, path)
	stampOutsideFile(t, path)
	t.Cleanup(func() {
		if o.tracked && !outsideAccessTime(t, path).Equal(outsideAccessStamp) {
			t.Error("inspection read a file outside the checkout and its Git folders")
		}
		if !bytes.Equal(readCheckoutFile(t, path), want) {
			t.Error("inspection changed a file outside the checkout")
		}
	})
}

func outsideCheckout(t *testing.T) (checkoutFixture, string, string) {
	t.Helper()
	f := newCheckoutFixture(t)
	root := f.repo(t, "checkout", "main")
	writeCheckoutFile(t, filepath.Join(root, "tracked"), []byte("tracked\n"), 0o600)
	f.gitCommand(t, root, "add", "tracked")
	f.commit(t, root, "root")
	outside := filepath.Join(f.base, "outside")
	makeCheckoutDir(t, outside)
	return f, root, outside
}

// writeOutsideSource writes bytes that are a valid excludes, attributes and sparse file.
func writeOutsideSource(t *testing.T, path string) string {
	t.Helper()
	writeCheckoutFile(t, path, []byte("/outside-ignored\n"), 0o600)
	return path
}

func makeCheckoutInfo(t *testing.T, root string) string {
	t.Helper()
	info := filepath.Join(root, ".git", "info")
	makeCheckoutDir(t, info)
	return info
}

func moveCheckoutPath(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatal("could not move a fixture path outside the checkout")
	}
	linkCheckoutPath(t, to, from)
}

// privateViewIgnored asks Git which names the private view ignores. Git runs with an empty
// home, so only the private view's own files and configuration can name an ignore source.
func privateViewIgnored(t *testing.T, f checkoutFixture, snapshot statusSnapshot, names ...string) string {
	t.Helper()
	home := filepath.Join(f.base, "empty-home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal("could not make the empty fixture home")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	args := []string{"--no-optional-locks", "--git-dir=" + snapshot.gitDir, "--work-tree=" + snapshot.root,
		"check-ignore", "--no-index", "--"}
	cmd := exec.CommandContext(ctx, f.git, append(args, names...)...)
	cmd.Dir = snapshot.root
	cmd.Env = []string{
		"PATH=" + filepath.Dir(f.git), "HOME=" + home, "XDG_CONFIG_HOME=" + home, "LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull,
	}
	out, err := cmd.Output()
	// Exit status 1 means that no name is ignored.
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		t.Fatalf("the private view's ignore check failed: %T", err)
	}
	return strings.TrimSpace(string(out))
}

// TestCheckoutReviewOutsideFiles covers post-4 1. Whatever the checkout's configuration and
// links say, inspection never opens or copies a file outside the checkout and its Git
// folders. It refuses with the fixed not-readable refusal instead. Independently selected
// service defaults and sources inside the checkout keep working.
func TestCheckoutReviewOutsideFiles(t *testing.T) {
	notReadable := linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE
	type setup func(t *testing.T, f checkoutFixture, root, outside string) string

	// The status capture reads these sources; discovery does not need them.
	for _, tc := range []struct {
		name  string
		setup setup
	}{
		{"absolute excludes file", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "excludes"))
			f.gitCommand(t, root, "config", "--local", "core.excludesFile", source)
			return source
		}},
		{"home-relative excludes file", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(f.home, "outside-excludes"))
			f.gitCommand(t, root, "config", "--local", "core.excludesFile", "~/outside-excludes")
			return source
		}},
		{"parent-relative excludes file", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "excludes"))
			f.gitCommand(t, root, "config", "--local", "core.excludesFile", "../outside/excludes")
			return source
		}},
		{"excludes file below a linked folder", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "excludes"))
			linkCheckoutPath(t, outside, filepath.Join(root, "rules"))
			f.gitCommand(t, root, "config", "--local", "core.excludesFile", "rules/excludes")
			return source
		}},
		{"linked excludes file", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "excludes"))
			linkCheckoutPath(t, source, filepath.Join(root, "rules"))
			f.gitCommand(t, root, "config", "--local", "core.excludesFile", "rules")
			return source
		}},
		{"absolute attributes file", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "attributes"))
			f.gitCommand(t, root, "config", "--local", "core.attributesFile", source)
			return source
		}},
		{"linked local exclude", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "exclude"))
			linkCheckoutPath(t, source, filepath.Join(makeCheckoutInfo(t, root), "exclude"))
			return source
		}},
		{"linked local attributes", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "attributes"))
			linkCheckoutPath(t, source, filepath.Join(makeCheckoutInfo(t, root), "attributes"))
			return source
		}},
		{"linked info folder", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "exclude"))
			linkCheckoutPath(t, outside, filepath.Join(root, ".git", "info"))
			return source
		}},
		{"linked sparse selection", func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := writeOutsideSource(t, filepath.Join(outside, "sparse-checkout"))
			f.gitCommand(t, root, "config", "--local", "core.sparseCheckout", "true")
			linkCheckoutPath(t, source, filepath.Join(makeCheckoutInfo(t, root), "sparse-checkout"))
			return source
		}},
	} {
		t.Run("capture refuses "+tc.name, func(t *testing.T) {
			f, root, outside := outsideCheckout(t)
			reads := newOutsideReads(t, f.base)
			source := tc.setup(t, f, root, outside)
			scratch := captureTempDir(t)
			reads.watch(t, source)
			requireCaptureError(t, t.Context(), fixtureCheckoutPaths(root), scratch)
		})
	}

	// Discovery and the status capture both read these sources. Discovery gives the fixed
	// not-readable refusal, or the not-a-repository refusal where Git itself agrees.
	for _, tc := range []struct {
		name          string
		notRepository bool
		setup         setup
	}{
		{"linked HEAD", true, func(t *testing.T, f checkoutFixture, root, outside string) string {
			source := filepath.Join(outside, "HEAD")
			writeCheckoutFile(t, source, []byte("ref: refs/heads/main\n"), 0o600)
			head := filepath.Join(root, ".git", "HEAD")
			if err := os.Remove(head); err != nil {
				t.Fatal("could not remove the fixture HEAD")
			}
			// Git itself does not accept this folder as a repository.
			linkCheckoutPath(t, source, head)
			return source
		}},
		{"linked configuration", false, func(t *testing.T, f checkoutFixture, root, outside string) string {
			f.gitCommand(t, root, "config", "--local", "remote.origin.url", "https://example.invalid/outside")
			source := filepath.Join(outside, "config")
			moveCheckoutPath(t, filepath.Join(root, ".git", "config"), source)
			if f.gitCommand(t, root, "config", "--get", "remote.origin.url") != "https://example.invalid/outside" {
				t.Fatal("installed Git does not read the linked configuration")
			}
			return source
		}},
		{"linked packed references", false, func(t *testing.T, f checkoutFixture, root, outside string) string {
			head := f.gitCommand(t, root, "rev-parse", "HEAD")
			f.gitCommand(t, root, "pack-refs", "--all")
			source := filepath.Join(outside, "packed-refs")
			moveCheckoutPath(t, filepath.Join(root, ".git", "packed-refs"), source)
			if f.gitCommand(t, root, "rev-parse", "--verify", "refs/heads/main") != head {
				t.Fatal("installed Git does not read the linked packed references")
			}
			return source
		}},
		{"linked loose reference", false, func(t *testing.T, f checkoutFixture, root, outside string) string {
			head := f.gitCommand(t, root, "rev-parse", "HEAD")
			source := filepath.Join(outside, "main")
			moveCheckoutPath(t, filepath.Join(root, ".git", "refs", "heads", "main"), source)
			if f.gitCommand(t, root, "rev-parse", "--verify", "refs/heads/main") != head {
				t.Fatal("installed Git does not read the linked loose reference")
			}
			return source
		}},
		{"linked reference folder", false, func(t *testing.T, f checkoutFixture, root, outside string) string {
			head := f.gitCommand(t, root, "rev-parse", "HEAD")
			moveCheckoutPath(t, filepath.Join(root, ".git", "refs"), filepath.Join(outside, "refs"))
			if f.gitCommand(t, root, "rev-parse", "--verify", "refs/heads/main") != head {
				t.Fatal("installed Git does not read the linked reference folder")
			}
			return filepath.Join(outside, "refs", "heads", "main")
		}},
		{"linked shallow boundary", false, func(t *testing.T, f checkoutFixture, root, outside string) string {
			head := f.gitCommand(t, root, "rev-parse", "HEAD")
			source := filepath.Join(outside, "shallow")
			writeCheckoutFile(t, source, []byte(head+"\n"), 0o600)
			linkCheckoutPath(t, source, filepath.Join(root, ".git", "shallow"))
			if f.gitCommand(t, root, "rev-parse", "--is-shallow-repository") != "true" {
				t.Fatal("installed Git does not read the linked shallow boundary")
			}
			return source
		}},
	} {
		t.Run("discovery and capture refuse "+tc.name, func(t *testing.T) {
			f, root, outside := outsideCheckout(t)
			reads := newOutsideReads(t, f.base)
			source := tc.setup(t, f, root, outside)
			scratch := captureTempDir(t)
			reads.watch(t, source)
			_, facts, refusal := readCheckout(t.Context(), root, f.home)
			if facts != nil || refusal != notReadable &&
				!(tc.notRepository && refusal == linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY) {
				t.Errorf("got refusal %s and facts present %t, want a fixed refusal without facts", refusal, facts != nil)
			}
			requireCaptureError(t, t.Context(), fixtureCheckoutPaths(root), scratch)
		})
	}

	// Controls: a repair that refuses every ignore source fails these.
	t.Run("excludes file inside the worktree is kept", func(t *testing.T) {
		f, root, _ := outsideCheckout(t)
		writeCheckoutFile(t, filepath.Join(root, "inside-rules"), []byte("inside-ignored\n"), 0o600)
		f.gitCommand(t, root, "config", "--local", "core.excludesFile", "inside-rules")
		scratch := captureTempDir(t)
		checkoutMetadata(t, root, f.home, root)
		withCheckoutSnapshot(t, fixtureCheckoutPaths(root), scratch, func(snapshot statusSnapshot) {
			if privateViewIgnored(t, f, snapshot, "inside-ignored", "tracked") != "inside-ignored" {
				t.Error("the private view lost an excludes file inside the worktree")
			}
		})
	})

	t.Run("service default excludes file is kept", func(t *testing.T) {
		f, root, _ := outsideCheckout(t)
		// The fixture sets the service's XDG_CONFIG_HOME to its home.
		makeCheckoutDir(t, filepath.Join(f.home, "git"))
		writeCheckoutFile(t, filepath.Join(f.home, "git", "ignore"), []byte("service-ignored\n"), 0o600)
		scratch := captureTempDir(t)
		checkoutMetadata(t, root, f.home, root)
		withCheckoutSnapshot(t, fixtureCheckoutPaths(root), scratch, func(snapshot statusSnapshot) {
			if privateViewIgnored(t, f, snapshot, "service-ignored", "tracked") != "service-ignored" {
				t.Error("the private view lost the service's default excludes file")
			}
		})
	})
}

// TestCheckoutReviewObjectStores covers post-4 2. Inspection never reads an object outside
// the checkout's own object folder: not through an alternate store, and not through a link
// inside the object folder. Discovery gives the fixed not-readable refusal and the status
// capture refuses. Packed objects in the checkout's own folder keep working.
func TestCheckoutReviewObjectStores(t *testing.T) {
	notReadable := linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE
	looseObject := func(objects, id string) string { return filepath.Join(objects, id[:2], id[2:]) }

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, f checkoutFixture) (root, object string)
	}{
		{"alternate store of a shared clone", func(t *testing.T, f checkoutFixture) (string, string) {
			store := f.repo(t, "store", "main")
			head := f.commit(t, store, "stored root")
			root := filepath.Join(f.base, "checkout")
			f.gitCommand(t, f.base, "clone", "--quiet", "--shared", "--template=", store, root)
			if _, err := os.Stat(filepath.Join(root, ".git", "objects", "info", "alternates")); err != nil {
				t.Fatal("the shared clone has no alternate store")
			}
			if _, err := os.Lstat(looseObject(filepath.Join(root, ".git", "objects"), head)); !errors.Is(err, fs.ErrNotExist) {
				t.Fatal("the shared clone holds its own copy of the commit")
			}
			if f.gitCommand(t, root, "cat-file", "-t", head) != "commit" {
				t.Fatal("installed Git does not read the alternate store")
			}
			return root, looseObject(filepath.Join(store, ".git", "objects"), head)
		}},
		{"linked object folder", func(t *testing.T, f checkoutFixture) (string, string) {
			root := f.repo(t, "checkout", "main")
			head := f.commit(t, root, "root")
			outside := filepath.Join(f.base, "outside")
			makeCheckoutDir(t, outside)
			moveCheckoutPath(t, filepath.Join(root, ".git", "objects"), filepath.Join(outside, "objects"))
			if f.gitCommand(t, root, "cat-file", "-t", head) != "commit" {
				t.Fatal("installed Git does not read the linked object folder")
			}
			return root, looseObject(filepath.Join(outside, "objects"), head)
		}},
		{"linked loose object folder", func(t *testing.T, f checkoutFixture) (string, string) {
			root := f.repo(t, "checkout", "main")
			head := f.commit(t, root, "root")
			outside := filepath.Join(f.base, "outside")
			makeCheckoutDir(t, outside)
			moveCheckoutPath(t, filepath.Join(root, ".git", "objects", head[:2]), filepath.Join(outside, head[:2]))
			if f.gitCommand(t, root, "cat-file", "-t", head) != "commit" {
				t.Fatal("installed Git does not read the linked loose object folder")
			}
			return root, filepath.Join(outside, head[:2], head[2:])
		}},
	} {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			f := newCheckoutFixture(t)
			reads := newOutsideReads(t, f.base)
			root, object := tc.setup(t, f)
			scratch := captureTempDir(t)
			reads.watch(t, object)
			checkoutRefusal(t, root, f.home, notReadable)
			requireCaptureError(t, t.Context(), fixtureCheckoutPaths(root), scratch)
		})
	}

	// Control: a repair that refuses a packed object folder fails this.
	t.Run("packed objects are kept", func(t *testing.T) {
		f := newCheckoutFixture(t)
		root := f.repo(t, "checkout", "main")
		head := f.commit(t, root, "root")
		f.gitCommand(t, root, "repack", "-a", "-d", "-q")
		if _, err := os.Lstat(looseObject(filepath.Join(root, ".git", "objects"), head)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("the fixture commit is still a loose object")
		}
		scratch := captureTempDir(t)
		_, facts := checkoutMetadata(t, root, f.home, root)
		checkoutBranches(t, facts, "main", "main", head)
		withCheckoutSnapshot(t, fixtureCheckoutPaths(root), scratch, func(snapshot statusSnapshot) {
			if f.snapshotGit(t, snapshot, "cat-file", "-t", head) != "commit" {
				t.Error("the private view lost access to a packed local commit")
			}
		})
	})
}

// symbolicHeadCheckout gives the main and topic branches separate roots and leaves HEAD on main.
func symbolicHeadCheckout(t *testing.T) (f checkoutFixture, root, mainRoot, topicRoot, topicTip string) {
	t.Helper()
	f = newCheckoutFixture(t)
	root = f.repo(t, "checkout", "main")
	mainRoot = f.commit(t, root, "main root")
	f.gitCommand(t, root, "checkout", "--quiet", "--orphan", "topic")
	topicRoot = f.commit(t, root, "topic root")
	topicTip = f.commit(t, root, "topic tip")
	f.gitCommand(t, root, "checkout", "--quiet", "main")
	return f, root, mainRoot, topicRoot, topicTip
}

// linkCheckoutHead replaces HEAD with Git's traditional symbolic link representation.
func linkCheckoutHead(t *testing.T, f checkoutFixture, root, target string) {
	t.Helper()
	head := filepath.Join(root, ".git", "HEAD")
	if err := os.Remove(head); err != nil {
		t.Fatal("could not remove the fixture HEAD")
	}
	linkCheckoutPath(t, target, head)
	if f.gitCommand(t, root, "symbolic-ref", "HEAD") != target {
		t.Fatal("installed Git does not read a symbolic link HEAD as a symbolic reference")
	}
}

func requireLinkedHead(t *testing.T, root, target string) {
	t.Helper()
	got, err := os.Readlink(filepath.Join(root, ".git", "HEAD"))
	if err != nil || got != target {
		t.Error("inspection changed the checkout's symbolic link HEAD")
	}
}

// TestCheckoutReviewSymbolicHead covers post-4 3. A HEAD that is a symbolic link to a branch
// reference names that branch as checked out, in discovery and in the private view. A link
// to a reference that is not a branch gets the same refusal as a textual one.
func TestCheckoutReviewSymbolicHead(t *testing.T) {
	const topic = "refs/heads/topic"
	for _, tc := range []struct {
		name   string
		packed bool
	}{
		{"loose branch", false},
		{"packed branch", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, root, _, topicRoot, topicTip := symbolicHeadCheckout(t)
			if tc.packed {
				f.gitCommand(t, root, "pack-refs", "--all")
				if _, err := os.Lstat(filepath.Join(root, ".git", "refs", "heads", "topic")); !errors.Is(err, fs.ErrNotExist) {
					t.Fatal("the fixture branch is still a loose reference")
				}
			}
			linkCheckoutHead(t, f, root, topic)
			scratch := captureTempDir(t)
			_, facts := checkoutMetadata(t, root, f.home, root)
			checkoutBranches(t, facts, "topic", "topic", topicRoot)
			withCheckoutSnapshot(t, fixtureCheckoutPaths(root), scratch, func(snapshot statusSnapshot) {
				if f.snapshotGit(t, snapshot, "rev-parse", "--symbolic-full-name", "HEAD") != topic {
					t.Error("the private view does not keep HEAD on the checked-out branch")
				}
				if f.snapshotGit(t, snapshot, "rev-parse", "HEAD") != topicTip {
					t.Error("the private view's HEAD does not resolve to the branch tip")
				}
			})
			requireLinkedHead(t, root, topic)
		})
	}

	t.Run("link to a tag", func(t *testing.T) {
		f, root, mainRoot, _, _ := symbolicHeadCheckout(t)
		f.gitCommand(t, root, "tag", "v1", mainRoot)
		linkCheckoutHead(t, f, root, "refs/tags/v1")
		scratch := captureTempDir(t)
		checkoutRefusal(t, root, f.home, linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE)
		requireCaptureError(t, t.Context(), fixtureCheckoutPaths(root), scratch)
		requireLinkedHead(t, root, "refs/tags/v1")
	})
}

// multibyteResolvedPath builds an absolute path of exactly length bytes. Its components fit
// a file system's name limit, and most bytes belong to three-byte characters, so a limit
// counted in characters accepts both lengths.
func multibyteResolvedPath(length int) string {
	var b strings.Builder
	for b.Len() < length {
		left := length - b.Len() - 1
		size := min(left, 240)
		if left-size == 1 {
			// A final component needs at least one byte after its separator.
			size--
		}
		b.WriteByte('/')
		b.WriteString(strings.Repeat("界", size/3))
		b.WriteString(strings.Repeat("d", size%3))
	}
	return b.String()
}

// TestCheckoutReviewResolvedPathLimit covers post-4 4. It sends both edges of the resolved-path
// limit through discovery's own validation without a folder of that depth. The native cases in
// TestInspectFolderRefusals stay for full path resolution (issue 1678).
func TestCheckoutReviewResolvedPathLimit(t *testing.T) {
	f := newCheckoutFixture(t)
	root := f.repo(t, "checkout", "topic")
	first := f.commit(t, root, "root")
	f.gitCommand(t, root, "config", "--local", "remote.origin.url", "https://example.invalid/Team/Repo.git")
	captureTempDir(t)
	paths, facts, refusal := readCheckout(t.Context(), root, f.home)
	if refusal != linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED || facts == nil {
		t.Fatalf("discovery of the short fixture gave refusal %s", refusal)
	}
	checkoutBranches(t, facts, "topic", "topic", first)

	t.Run("resolved discovery agrees with discovery", func(t *testing.T) {
		gotPaths, gotFacts, gotRefusal := readResolvedCheckout(t.Context(), paths, root)
		if gotRefusal != refusal || gotPaths != paths || !proto.Equal(gotFacts, facts) {
			t.Error("discovery after resolution did not return discovery's own paths and facts")
		}
	})

	for _, length := range []int{testResolvedPathBytes, testResolvedPathBytes + 1} {
		long := multibyteResolvedPath(length)
		if len(long) != length || !utf8.ValidString(long) || utf8.RuneCountInString(long) > testResolvedPathBytes/2 {
			t.Fatal("the long fixture path does not have the intended bytes and characters")
		}
		resolved := paths
		resolved.root = long
		t.Run(fmt.Sprintf("resolved path of %d bytes", length), func(t *testing.T) {
			gotPaths, gotFacts, gotRefusal := readResolvedCheckout(t.Context(), resolved, long)
			if length > testResolvedPathBytes {
				if gotRefusal != linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE || gotFacts != nil {
					t.Errorf("got refusal %s and facts present %t, want the path refusal", gotRefusal, gotFacts != nil)
				}
				return
			}
			switch {
			case gotRefusal == linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE:
				t.Error("discovery refused a resolved path at the byte limit")
			case gotRefusal != linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED:
				// A later step may refuse the fabricated root for another reason.
				if gotFacts != nil {
					t.Error("discovery returned facts with a refusal")
				}
			case gotFacts == nil || gotPaths.root != long || gotFacts.GetResolvedPath() != long ||
				gotFacts.GetName() != path.Base(long):
				t.Error("discovery did not report the resolved path at the byte limit and its basename")
			}
		})
	}
}
