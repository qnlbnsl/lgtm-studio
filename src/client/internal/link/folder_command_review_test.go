package link

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestExpiredFolderCommandWhileInspectionIsHeld(t *testing.T) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		peer := newFolderPeer(t, 1, 0)
		saved, held, expired := folderTestCommand(1), folderTestCommand(2), folderTestCommand(3)
		saved.ExpiresAt = timestamppb.New(time.Now().Add(-time.Second))
		expired.ExpiresAt = timestamppb.New(time.Now().Add(-time.Second))
		// Seed a completed inspection from before its expiry. The server has
		// acknowledged its report and can retry the original command unchanged.
		store, err := OpenStore(peer.opts.Folder, StoreOptions{})
		if err != nil {
			t.Fatal("could not open the store to seed the saved folder receipt")
		}
		defer store.Close()
		if decision, _, err := store.Begin(receiptCommand(saved), saved.ExpiresAt.AsTime().Add(-time.Second)); err != nil || decision != DecisionRun {
			t.Fatal("could not seed the started folder receipt")
		}
		report := &linkv1.Event{
			HappenedAt: timestamppb.New(saved.ExpiresAt.AsTime().Add(-time.Second)),
			Report: &linkv1.Event_FolderSeen{FolderSeen: &linkv1.FolderSeen{
				CommandId: saved.GetId(), OperationId: saved.GetLookAtFolder().GetOperationId(),
				Result: &linkv1.FolderSeen_Refusal{Refusal: folderNotGit},
			}},
		}
		if number, err := store.appendFilteredReport(t.Context(), nil, report); err != nil || number != 1 {
			t.Fatal("could not seed the saved command's durable folder report")
		}
		if err := store.Finish(saved.GetId(), []byte{byte(commandDone)}); err != nil {
			t.Fatal("could not finish the saved folder receipt")
		}
		if err := store.Close(); err != nil {
			t.Fatal("could not close the seeded folder store")
		}

		disk := observeFolderDisk(t, "", expired)
		cancel, done := peer.start(t)
		defer cancel()
		leg := peer.next(t, 0)
		peer.welcome(t, leg, runWelcome(approvedApproval, 1, 1))

		leg.send(t, commandFrame(held))
		read := requireFolderStart(t, calls, held)
		replay := attemptCommand(saved, 2)
		leg.send(t, commandFrame(replay))
		result, wireReport := nextFolderResult(t, leg, replay)
		requireFolderCommandResult(t, result, replay, commandDone, 1)
		if wireReport != nil {
			t.Fatal("replaying the saved result produced a second folder report")
		}
		requireEmpty(t, calls, "replaying the saved result started a second inspection")

		leg.send(t, commandFrame(expired))
		requireEmpty(t, calls, "the expired command started a second inspection")
		requireEmpty(t, leg.closed, "the expired command closed the occupied stream before returning its expired result")
		result, wireReport = nextFolderResult(t, leg, expired)
		requireFolderCommandResult(t, result, expired, commandExpired, 1)
		record := disk.record(t, expired)
		requireFolderReceipt(t, record, commandExpired, 1)
		if wireReport != nil || record.reportCommits != 0 {
			t.Fatal("the expired command produced a folder report")
		}
		if read.ctx.Err() != nil {
			t.Fatal("declining the expired command canceled the held inspection")
		}

		// The original inspection must settle on the same signed stream.
		refusal := folderReadResult{refusal: folderNotGit}
		read.result <- refusal
		result, wireReport = nextFolderResult(t, leg, held)
		requireFolderCommandResult(t, result, held, commandDone, 2)
		requireFolderWireReport(t, leg, held, wireReport, 2, refusal)
		requireEmpty(t, calls, "the peer observed another inspection after the held read settled")
		requireEmpty(t, peer.peer.opens, "the client opened a replacement stream")
		cancel()
		peer.stop(t, done, nil)
	})
}

func TestFolderCommandInspectsCheckoutWithDefaultReader(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("could not resolve the fixture home")
	}
	root := filepath.Join(home, "fixture-checkout")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal("could not create the checkout fixture")
	}
	git := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = root
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"), "HOME=" + home, "XDG_CONFIG_HOME=" + home,
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull,
			"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
			"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		}
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("fixture Git command %s failed with error type %T", args[0], err)
		}
		return strings.TrimSpace(string(out))
	}
	writeTracked := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte(content), 0o600); err != nil {
			t.Fatal("could not write the checkout fixture")
		}
	}
	git("init", "--quiet", "--template=", "--initial-branch=main")
	writeTracked("committed\n")
	git("add", "tracked.txt")
	git("commit", "--quiet", "-m", "First fixture commit")
	first := git("rev-parse", "HEAD")
	git("commit", "--quiet", "--allow-empty", "-m", "Second fixture commit")
	git("checkout", "--quiet", "-b", "topic")
	const origin = "https://git.example.invalid/team/fixture.git"
	git("remote", "add", "origin", origin)
	git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	writeTracked("dirty\n")

	readCheckoutFile := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("could not read checkout fixture file %s", name)
		}
		return data
	}
	before := make(map[string][]byte)
	for _, name := range []string{
		".git/HEAD", ".git/index", ".git/config", ".git/refs/heads/main",
		".git/refs/heads/topic", ".git/refs/remotes/origin/HEAD", "tracked.txt",
	} {
		before[name] = readCheckoutFile(name)
	}

	synctest.Test(t, func(t *testing.T) {
		peer := newFolderPeer(t, 1, 0)
		peer.opts.ScanOptions.Home = home
		cmd := folderTestCommand(1)
		cmd.GetLookAtFolder().Path = "~/fixture-checkout"
		disk := observeFolderDisk(t, "", cmd)
		cancel, done := peer.start(t)
		defer cancel()
		leg := peer.next(t, 0)
		peer.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		leg.send(t, commandFrame(cmd))

		want := folderReadResult{facts: &linkv1.FolderFacts{
			ResolvedPath: root, Name: "fixture-checkout", MainBranch: "main",
			CheckedOutBranch: proto.String("topic"), FirstCommit: proto.String(first),
			Remote: proto.String(origin), Clean: false,
		}}
		result, report := nextFolderResult(t, leg, cmd)
		requireFolderCommandResult(t, result, cmd, commandDone, 1)
		requireFolderWireReport(t, leg, cmd, report, 1, want)
		requireFolderDiskReport(t, disk, cmd, 1, want)
		requireFolderReceipt(t, disk.record(t, cmd), commandDone, 1)
		cancel()
		peer.stop(t, done, nil)
	})
	for name, original := range before {
		if !bytes.Equal(readCheckoutFile(name), original) {
			t.Errorf("the folder command changed checkout fixture file %s", name)
		}
	}
}
