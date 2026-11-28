package projects_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestProjectAttachmentAliases(t *testing.T) {
	firstWrite := make(chan int32, 1)
	var heldFirst atomic.Bool
	release, allowCommit := storetest.Signal(t)
	defer allowCommit()
	f := newReportFixture(t, reportControls{
		afterReport: func(ctx context.Context, tx *store.Tx, event *linkv1.Event) error {
			if event.GetNumber() != 1 || !heldFirst.CompareAndSwap(false, true) {
				return nil
			}
			var pid int32
			if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
				return err
			}
			firstWrite <- pid
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})

	const checkout = "/workspace/checkout"
	first := repositoryCall("alias one", f.machine, "~/link to checkout", nil)
	second := repositoryCall("alias two", f.machine, "~/checkout/subdirectory", nil)
	firstResult := first.start(t, t.Context(), f.service)
	secondResult := second.start(t, t.Context(), f.service)
	firstID, firstCommand := f.awaitOperation(t, first, firstResult)
	secondID, secondCommand := f.awaitOperation(t, second, secondResult)
	firstOp := reportOperation{call: first, id: firstID, command: firstCommand}
	secondOp := reportOperation{call: second, id: secondID, command: secondCommand}
	streamID := repositoryStreamID(t, f, f.machine)

	sendFolderReport(t, f.stream, 1, firstOp.facts())
	firstAck := storetest.Start(t, func() bool {
		frame, err := f.stream.Receive()
		return err == nil && frame.GetEventAck() != nil && frame.GetEventAck().GetThrough() == 1
	})
	var blocker int32
	select {
	case blocker = <-firstWrite:
	case <-firstAck.Done():
		t.Fatal("The first alias report ended before its attachment reached the commit barrier.")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("The first alias report did not reach the commit barrier.")
	}
	f.requireNoMembership(t)
	f.requireMark(t, f.machine, 0)

	// The signed stream reads frames in order. Enter the same Link write from
	// another worker so the two reports contend while the first is uncommitted.
	secondWrite := startRepositoryEvent(t, f, streamID, secondOp, 2, secondOp.facts())
	awaitRepositoryBlock(t, f, secondWrite, blocker)
	allowCommit()
	if !firstAck.Wait(t) {
		t.Fatal("The first alias report did not receive its acknowledgement.")
	}
	requireRepositoryEvent(t, secondWrite.Wait(t), 2)
	f.requireMark(t, f.machine, 2)

	firstRow, firstReply := requireRepositoryAdd(t, f, firstOp, firstResult.Wait(t))
	_, secondReply := requireRepositoryAdd(t, f, secondOp, secondResult.Wait(t))
	if !proto.Equal(firstReply, secondReply) {
		t.Fatal("The reports for two aliases did not return the same saved repository and folder.")
	}
	firstFolder := firstReply.GetFolder()
	repository := firstReply.GetRepository()
	requireRepositoryFolder(t, firstFolder, f.machine, checkout)
	requireRepositoryProjection(t, f, repository, firstFolder)
	requireRepositoryMetadata(t, repository, &webv1.Repository{
		Id: repository.GetId(), Name: "checkout", MainBranch: "main",
		Remote: new("https://example.test/checkout.git"), FirstCommit: new("first-commit"),
	})
	requireRepositoryCounts(t, f, 1, 1)
	requireRepositorySaves(t, f, repository.GetId())

	mark := uint64(2)
	changed := repositoryOrigin{remote: new("https://example.test/Elsewhere.git"), first: new("another-commit")}
	for _, target := range []*string{nil, new(repository.GetId())} {
		op, reply := applyRepositoryAdd(t, f, f.machine, streamID, &mark, target, checkout, changed)
		if !proto.Equal(reply, firstReply) {
			t.Fatal("Re-adding a saved path with a changed origin did not return its existing identity.")
		}
		requireOperationReply(t, op.call.run(t.Context(), f.service, op.call.key), f.row(t, op).reply)
	}
	requireRepositorySaves(t, f, repository.GetId())

	peer := f.registerPeer(t, "Second repository machine")
	f.open(t, peer, 0)
	peerMark := uint64(0)
	origin := repositoryOrigin{remote: repository.Remote, first: repository.FirstCommit}
	_, peerReply := applyRepositoryAdd(t, f, peer.ID, repositoryStreamID(t, f, peer.ID), &peerMark,
		new(repository.GetId()), checkout, origin)
	peerFolder := peerReply.GetFolder()
	requireRepositoryFolder(t, peerFolder, peer.ID, checkout)
	requireRepositoryProjection(t, f, peerReply.GetRepository(), firstFolder, peerFolder)

	// Exact path equality keeps two checkouts on one machine, including paths
	// which differ only by case.
	_, localReply := applyRepositoryAdd(t, f, f.machine, streamID, &mark, new(repository.GetId()), "/workspace/Checkout", origin)
	localFolder := localReply.GetFolder()
	requireRepositoryFolder(t, localFolder, f.machine, "/workspace/Checkout")
	requireRepositoryProjection(t, f, localReply.GetRepository(), firstFolder, peerFolder, localFolder)
	if firstFolder.GetId() == peerFolder.GetId() || firstFolder.GetId() == localFolder.GetId() || peerFolder.GetId() == localFolder.GetId() {
		t.Fatal("Distinct machine and path memberships shared a folder identifier.")
	}
	_, current := applyRepositoryAdd(t, f, f.machine, streamID, &mark, nil, checkout, changed)
	if !proto.Equal(current.GetRepository(), localReply.GetRepository()) || !proto.Equal(current.GetFolder(), firstFolder) {
		t.Fatal("A fresh reuse did not return the now complete repository in the same folder order.")
	}
	requireRepositoryCounts(t, f, 1, 3)
	requireRepositorySaves(t, f, repository.GetId(), repository.GetId(), repository.GetId())

	// The original held request still returns the full reply as it was saved.
	requireOperationReply(t, first.run(t.Context(), f.service, first.key), firstRow.reply)
	_, distinct := applyRepositoryAdd(t, f, f.machine, streamID, &mark, nil, "/elsewhere/checkout", origin)
	if distinct.GetRepository().GetId() == repository.GetId() || len(distinct.GetRepository().GetFolders()) != 1 {
		t.Fatal("A new path with no target did not create a distinct repository with its own folder.")
	}
	requireRepositoryCounts(t, f, 2, 4)
	requireRepositorySaves(t, f, repository.GetId(), repository.GetId(), repository.GetId(), distinct.GetRepository().GetId())
}

func TestProjectAttachmentIdentity(t *testing.T) {
	f := newReportFixture(t, reportControls{})
	streamID := repositoryStreamID(t, f, f.machine)
	owner := f.registerPeer(t, "Repository owner machine")
	mark := uint64(0)
	const equalCommit = "shared-first-commit"
	present := func(remote string) repositoryOrigin {
		return repositoryOrigin{remote: new(remote), first: new(equalCommit)}
	}
	differentCommit := func(remote string) repositoryOrigin {
		return repositoryOrigin{remote: new(remote), first: new("other-first-commit")}
	}
	absent := repositoryOrigin{first: new(equalCommit)}
	withheld := repositoryOrigin{omitted: true, first: new(equalCommit)}
	configured := present("https://example.test/Git/Shared.git")
	cases := []struct {
		name          string
		saved, report repositoryOrigin
		attach        bool
	}{
		{"HTTPSDefaultPortAndCase", configured, differentCommit("HTTPS://EXAMPLE.test:443/Git/Shared.git"), true},
		{"HTTPDefaultPortAndCase", present("http://example.test/Git/Shared.git"), differentCommit("HTTP://EXAMPLE.test:80/Git/Shared.git"), true},
		{"SSHDefaultPortAndCase", present("ssh://example.test/Git/Shared.git"), differentCommit("SSH://EXAMPLE.test:22/Git/Shared.git"), true},
		{"GitDefaultPortAndCase", present("git://example.test/Git/Shared.git"), differentCommit("GIT://EXAMPLE.test:9418/Git/Shared.git"), true},
		{"SSHAbsoluteShorthand", present("EXAMPLE.test:/Git/Shared.git"), differentCommit("ssh://example.test:22/Git/Shared.git"), true},
		{"SameNondefaultPort", present("ssh://example.test:2222/Git/Shared.git"), differentCommit("SSH://EXAMPLE.test:2222/Git/Shared.git"), true},
		{"DistinctPresentOrigins", configured, present("https://example.test/Git/Other.git"), false},
		{"DistinctTransport", configured, present("http://example.test/Git/Shared.git"), false},
		{"DistinctNondefaultPort", configured, present("https://example.test:8443/Git/Shared.git"), false},
		{"DistinctPathCase", configured, present("https://example.test/git/Shared.git"), false},
		{"DistinctSuffix", configured, present("https://example.test/Git/Shared"), false},
		{"DistinctTrailingSlash", configured, present("https://example.test/Git/Shared.git/"), false},
		{"SavedOriginKnownAbsent", absent, configured, true},
		{"ReportOriginKnownAbsent", configured, absent, true},
		{"TwoKnownAbsentOrigins", absent, absent, true},
		{"DifferentCommits", absent, repositoryOrigin{first: new("other-first-commit")}, false},
		{"MissingSavedCommit", repositoryOrigin{}, absent, false},
		{"MissingReportedCommit", absent, repositoryOrigin{}, false},
		{"BothCommitsMissing", repositoryOrigin{}, repositoryOrigin{}, false},
		{"SavedOriginWithheld", withheld, configured, false},
		{"ReportedOriginWithheld", configured, withheld, false},
		{"WithheldAndKnownAbsent", withheld, absent, true},
		{"KnownAbsentAndWithheld", absent, withheld, true},
		{"TwoWithheldEqualCommits", withheld, withheld, false},
		// These are the facts sent after the client withholds home-relative
		// origins from two accounts. Their original origin text never reaches Link.
		{"TwoAccountsWithWithheldOrigins", withheld, repositoryOrigin{omitted: true, first: new("other-account-first-commit")}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved, priorFolder := seedRepository(t, f, owner.ID, tc.saved)
			before := readRepositoryCounts(t, f)
			head := repositoryFeedHead(t, f)
			resolved := "/workspace/" + tc.name
			op, caller := startRepositoryAdd(t, f, f.machine, new(saved.GetId()), "~/different name")
			seen := tc.report.report(op, resolved)
			applyRepositoryEvent(t, f, streamID, op, &mark, seen)
			if !tc.attach {
				f.requireFailure(t, op, "repository_mismatch", connect.CodeFailedPrecondition, "The folder does not belong to the selected repository.")
				requireOperationRefusal(t, caller.Wait(t), connect.CodeFailedPrecondition, "The folder does not belong to the selected repository.")
				requireRepositoryCounts(t, f, before.repositories, before.folders)
				requireRepositorySavesAfter(t, f, head)
				return
			}
			_, reply := requireRepositoryAdd(t, f, op, caller.Wait(t))
			requireRepositoryFolder(t, reply.GetFolder(), f.machine, resolved)
			requireRepositoryProjection(t, f, reply.GetRepository(), priorFolder, reply.GetFolder())
			requireRepositoryMetadata(t, reply.GetRepository(), saved)
			requireRepositoryCounts(t, f, before.repositories, before.folders+1)
			requireRepositorySavesAfter(t, f, head, saved.GetId())
		})
	}

	t.Run("MissingTargetAndOwnership", func(t *testing.T) {
		owner, folder := seedRepository(t, f, f.machine, configured)
		other, _ := seedRepository(t, f, f.machine, configured)
		missing := projectMigrationID(t)
		before := readRepositoryCounts(t, f)
		head := repositoryFeedHead(t, f)
		for _, tc := range []struct {
			name, resolved, target, failure, message string
			code                                     connect.Code
		}{
			{"MissingTargetForSavedPath", folder.GetResolvedPath(), missing, "repository_not_found", "The repository was not found.", connect.CodeNotFound},
			{"MissingTargetForUnsavedPath", "/workspace/unsaved", missing, "repository_not_found", "The repository was not found.", connect.CodeNotFound},
			{"DifferentKnownTarget", folder.GetResolvedPath(), other.GetId(), "folder_owned_elsewhere", "The folder is already saved for another repository.", connect.CodeAlreadyExists},
		} {
			t.Run(tc.name, func(t *testing.T) {
				op, caller := startRepositoryAdd(t, f, f.machine, new(tc.target), "~/reported")
				seen := configured.report(op, tc.resolved)
				if tc.code == connect.CodeAlreadyExists {
					applyRepositoryEventUnderTargetLock(t, f, streamID, op, &mark, seen)
				} else {
					applyRepositoryEvent(t, f, streamID, op, &mark, seen)
				}
				f.requireFailure(t, op, tc.failure, tc.code, tc.message)
				requireOperationRefusal(t, caller.Wait(t), tc.code, tc.message)
				requireRepositoryCounts(t, f, before.repositories, before.folders)
				requireRepositorySavesAfter(t, f, head)
				if !repositoryObserve[bool](t, f, `SELECT EXISTS (SELECT FROM repository_folders
WHERE id = $1 AND repository_id = $2 AND machine_id = $3 AND resolved_path = $4)`,
					folder.GetId(), owner.GetId(), f.machine, folder.GetResolvedPath()) {
					t.Fatal("A refused attachment changed the saved folder's ownership.")
				}
			})
		}
	})

	t.Run("NotAGitRepositorySavesNothing", func(t *testing.T) {
		before := readRepositoryCounts(t, f)
		head := repositoryFeedHead(t, f)
		op, caller := startRepositoryAdd(t, f, f.machine, nil, "~/not git")
		seen := op.facts()
		seen.Result = &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY}
		applyRepositoryEvent(t, f, streamID, op, &mark, seen)
		f.requireFailure(t, op, "not_a_git_repository", connect.CodeFailedPrecondition, "The folder is not a Git repository.")
		requireOperationRefusal(t, caller.Wait(t), connect.CodeFailedPrecondition, "The folder is not a Git repository.")
		requireRepositoryCounts(t, f, before.repositories, before.folders)
		requireRepositorySavesAfter(t, f, head)
	})

	t.Run("CredentialBearingOriginSavesNothing", func(t *testing.T) {
		const marker = "synthetic-attachment-credential"
		before := readRepositoryCounts(t, f)
		head := repositoryFeedHead(t, f)
		op := f.admit(t, "Add with ineffective origin", true)
		open := f.row(t, op)
		seen := (repositoryOrigin{remote: new("https://fixture:" + marker + "@example.test/Git/Shared.git")}).report(op, "/workspace/unsafe")
		mark++
		sendFolderReport(t, f.stream, mark, seen)
		f.requireAck(t, f.stream, f.machine, mark)
		f.requireUnchanged(t, op, open)
		requireRepositoryCounts(t, f, before.repositories, before.folders)
		requireRepositorySavesAfter(t, f, head)
		if f.logs.containsAny(marker) {
			t.Fatal("The fixture origin credential reached the Link log.")
		}
	})
}

func TestProjectAttachmentRollback(t *testing.T) {
	var fail atomic.Bool
	var writes, acknowledgements atomic.Int32
	controls := reportControls{
		dispatch: true,
		afterReport: func(ctx context.Context, tx *store.Tx, event *linkv1.Event) error {
			var applied bool
			err := tx.QueryRow(ctx, `
SELECT EXISTS (SELECT FROM repositories r JOIN repository_folders f ON f.repository_id = r.id
               JOIN folder_operations o ON o.machine_id = f.machine_id
               WHERE o.id = $1 AND o.state = 'succeeded' AND o.reply IS NOT NULL AND o.failure IS NULL)
`, event.GetFolderSeen().GetOperationId()).Scan(&applied)
			if applied {
				writes.Add(1)
			}
			return err
		},
		Seams: link.Seams{
			BeforeEventCommit: func(context.Context, string, uint64) error {
				if fail.Load() {
					return errors.New("The fixture interrupted the attachment write.")
				}
				return nil
			},
			BeforeEventAck: func(context.Context, string, uint64) { acknowledgements.Add(1) },
		},
	}
	f := newReportFixture(t, controls)
	op := f.admit(t, "attachment rollback", true)
	before := f.row(t, op)
	command := f.deliver(t, op)
	sendReportResult(t, f.stream, op.command, command.GetAttempt(), linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED, 1)
	requireReportHeartbeat(t, f.stream)
	f.requireCommand(t, op, command.GetAttempt(), "", "interrupted", new(uint64(1)))

	seen := (repositoryOrigin{omitted: true, first: new("initial-commit")}).report(op, "/workspace/checkout")
	fail.Store(true)
	sendFolderReport(t, f.stream, 1, seen)
	requireReportWriteFailure(t, f.stream)
	if writes.Load() != 1 {
		t.Error("The injected fault did not follow a completed attachment and terminal reply inside the report write.")
	}
	if acknowledgements.Load() != 0 {
		t.Error("The failed attachment reached the committed acknowledgement hook.")
	}
	f.requireMark(t, f.machine, 0)
	f.requireUnchanged(t, op, before)
	f.requireCommand(t, op, command.GetAttempt(), "", "interrupted", new(uint64(1)))
	f.requireNoMembership(t)

	fail.Store(false)
	f.server.Close()
	f.db.Close()
	f.db = openOperationStore(t, f.config)
	f.startServer(t, controls)
	f.stream = f.open(t, f.peer, 0)
	f.requireCommand(t, op, command.GetAttempt(), "", "interrupted", new(uint64(1)))
	sendFolderReport(t, f.stream, 1, seen)
	f.requireAck(t, f.stream, f.machine, 1)
	saved, reply := requireRepositoryAdd(t, f, op, op.call.run(t.Context(), f.service, op.call.key))
	f.requireCommand(t, op, command.GetAttempt(), "interrupted", "", nil)
	requireRepositoryFolder(t, reply.GetFolder(), f.machine, "/workspace/checkout")
	requireRepositoryProjection(t, f, reply.GetRepository(), reply.GetFolder())
	requireRepositoryMetadata(t, reply.GetRepository(), &webv1.Repository{
		Id: reply.GetRepository().GetId(), Name: "checkout", MainBranch: "main",
		FirstCommit: new("initial-commit"), RemoteOmitted: true,
	})
	requireRepositoryCounts(t, f, 1, 1)
	requireRepositorySaves(t, f, reply.GetRepository().GetId())

	sendFolderReport(t, f.stream, 1, seen)
	f.requireAck(t, f.stream, f.machine, 1)
	f.requireUnchanged(t, op, saved)
	requireRepositoryCounts(t, f, 1, 1)
	requireRepositorySaves(t, f, reply.GetRepository().GetId())
	if writes.Load() != 2 || acknowledgements.Load() != 1 {
		t.Error("Retry and duplicate delivery did not leave exactly one committed attachment and mark advance.")
	}

	f.disconnect(t)
	f.server.Close()
	f.db.Close()
	f.db = openOperationStore(t, f.config)
	f.startServer(t, reportControls{})
	requireOperationReply(t, op.call.run(t.Context(), f.service, op.call.key), saved.reply)
	if counts := f.counts(t); counts.operations != 1 || counts.commands != 1 || counts.receipts != 1 {
		t.Error("The offline replay after restart did not preserve exactly one operation, command and held receipt.")
	}
}
