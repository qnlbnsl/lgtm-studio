package projects_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	removalMachineMessage = "The machine was removed."
	removalTargetMessage  = "The repository was not found."
)

func TestProjectMachineRemoval(t *testing.T) {
	t.Run("WithoutPendingCommands", testProjectRemovalWithoutCommands)
	for _, count := range []int{254, 255, 256} {
		t.Run(fmt.Sprintf("FeedBudget%d", count), func(t *testing.T) {
			testProjectRemovalFeedBudget(t, count)
		})
	}
	t.Run("CleanupFaultRollsBack", testProjectRemovalRollback)
	t.Run("RemovalFencesReport", testProjectRemovalFencesReport)
}

func testProjectRemovalWithoutCommands(t *testing.T) {
	f := newRemovalFixture(t, false)
	other := f.registerPeer(t, "Surviving machine")
	origin := repositoryOrigin{remote: new("https://example.test/shared.git")}
	shared, survivor := seedRepository(t, f.reportFixture, other.ID, origin)
	orphan, _ := seedRepository(t, f.reportFixture, f.machine, repositoryOrigin{})
	f.open(t, other, 0)
	otherOpen, _ := startRepositoryAdd(t, f.reportFixture, other.ID, nil, "~/other folder")
	otherRow := f.row(t, otherOpen)

	add, addCaller := startRepositoryAdd(t, f.reportFixture, f.machine, &shared.Id, "~/shared")
	seen := origin.report(add, "/workspace/shared")
	sendFolderReport(t, f.stream, 1, seen)
	f.requireAck(t, f.stream, f.machine, 1)
	addRow, reply := requireRepositoryAdd(t, f.reportFixture, add, addCaller.Wait(t))
	requireRepositoryProjection(t, f.reportFixture, reply.GetRepository(), reply.GetFolder(), survivor)

	inspect := f.admit(t, "inspect before removal", false)
	failed := f.admit(t, "refused before removal", false)
	open := f.admit(t, "open during removal", true)
	head := repositoryFeedHead(t, f.reportFixture)
	sendFolderReport(t, f.stream, 2, inspect.facts())
	f.requireAck(t, f.stream, f.machine, 2)
	inspectRow := f.requireInspect(t, inspect, expectedReportFacts())
	sendFolderReport(t, f.stream, 3, &linkv1.FolderSeen{
		OperationId: failed.id, CommandId: failed.command,
		Result: &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY},
	})
	f.requireAck(t, f.stream, f.machine, 3)
	failedRow := f.requireFailure(t, failed, "not_a_git_repository", connect.CodeFailedPrecondition, "The folder is not a Git repository.")
	requireRepositorySavesAfter(t, f.reportFixture, head)
	requireRepositoryCounts(t, f.reportFixture, 2, 3)

	f.exec(t, "DELETE FROM link_commands WHERE machine_id = $1", f.machine)
	f.disconnect(t)
	requireRepositoryCounts(t, f.reportFixture, 2, 3)
	if !repositoryObserve[bool](t, f.reportFixture, `SELECT state = 'open' FROM folder_operations WHERE id = $1`, open.id) {
		t.Fatal("Disconnect did not preserve the open operation.")
	}
	head = repositoryFeedHead(t, f.reportFixture)
	requireRemovalSuccess(t, f.remove(t.Context(), f.machine, ""))
	f.requireFailure(t, open, "machine_removed", connect.CodeNotFound, removalMachineMessage)
	f.requireRemovedMachine(t, f.machine)
	requireRepositoryCounts(t, f.reportFixture, 1, 1)
	requireRemovalSurvivor(t, f, shared, survivor)
	f.requireFeed(t, head, removalMachineRef(f.machine),
		removalRepositoryRef(shared.Id, store.Saved), removalRepositoryRef(orphan.Id, store.Removed))

	reopened := openOperationStore(t, f.config)
	service := f.operationFixture.service(t, reopened, link.NewState(reopened, f.registry, link.NewHub()))
	requireOperationReply(t, add.call.run(t.Context(), service, add.call.key), addRow.reply)
	requireOperationReply(t, inspect.call.run(t.Context(), service, inspect.call.key), inspectRow.reply)
	requireOperationRefusal(t, failed.call.run(t.Context(), service, failed.call.key),
		connect.CodeFailedPrecondition, "The folder is not a Git repository.")
	requireOperationRefusal(t, open.call.run(t.Context(), service, open.call.key), connect.CodeNotFound, removalMachineMessage)
	f.requireUnchanged(t, add, addRow)
	f.requireUnchanged(t, inspect, inspectRow)
	f.requireUnchanged(t, failed, failedRow)
	f.requireUnchanged(t, otherOpen, otherRow)
	f.requireFeed(t, head, removalMachineRef(f.machine),
		removalRepositoryRef(shared.Id, store.Saved), removalRepositoryRef(orphan.Id, store.Removed))
}

func requireRemovalSurvivor(t *testing.T, f *removalFixture, repository *webv1.Repository, folder *webv1.RepositoryFolder) {
	t.Helper()
	want := proto.CloneOf(repository)
	want.Folders = []*webv1.RepositoryFolder{folder}
	requireRepositoryProjection(t, f.reportFixture, want, folder)
}

func testProjectRemovalFeedBudget(t *testing.T, count int) {
	f := newRemovalFixture(t, false)
	other := f.registerPeer(t, "Shared repository survivor")
	shared, _ := seedRepository(t, f.reportFixture, f.machine, repositoryOrigin{})
	survivor := f.addFolder(t, shared.Id, other.ID, "/workspace/survivor")
	f.addFolder(t, shared.Id, f.machine, "/workspace/second-checkout")
	ordinary := []store.ChangeRef{removalMachineRef(f.machine), removalRepositoryRef(shared.Id, store.Saved)}
	for index := 1; index < count; index++ {
		orphan, _ := seedRepository(t, f.reportFixture, f.machine, repositoryOrigin{})
		ordinary = append(ordinary, removalRepositoryRef(orphan.Id, store.Removed))
	}
	pending := f.admit(t, "pending at removal", true)
	head := repositoryFeedHead(t, f.reportFixture)
	requireRemovalSuccess(t, f.remove(t.Context(), f.machine, ""))
	if count <= 254 {
		f.requireFeed(t, head, ordinary...)
	} else {
		f.requireFeed(t, head, removalMachineRef(f.machine), removalRepositoryRef("refresh", store.Saved))
	}
	f.requireRemovedMachine(t, f.machine)
	f.requireRemovedCommand(t, pending, 1)
	f.requireFailure(t, pending, "machine_removed", connect.CodeNotFound, removalMachineMessage)
	requireRepositoryCounts(t, f.reportFixture, 1, 1)
	requireRemovalSurvivor(t, f, shared, survivor)
}

func testProjectRemovalRollback(t *testing.T) {
	f := newRemovalFixture(t, true)
	orphan, _ := seedRepository(t, f.reportFixture, f.machine, repositoryOrigin{})
	pending := f.admit(t, "staged at removal", true)
	command := f.deliver(t, pending)
	sendReportResult(t, f.stream, pending.command, command.GetAttempt(), linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED, 1)
	requireReportHeartbeat(t, f.stream)
	f.requireCommand(t, pending, command.GetAttempt(), "", "interrupted", new(uint64(1)))

	f.exec(t, `
CREATE FUNCTION fail_removal_orphan_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'Fixture rejects orphan cleanup' USING ERRCODE = '23514';
END
$$;
CREATE TRIGGER fail_removal_orphan_delete BEFORE DELETE ON repositories
FOR EACH ROW EXECUTE FUNCTION fail_removal_orphan_delete()`)
	before := f.rollbackSnapshot(t)
	const key = "retry machine removal"
	err := f.remove(t.Context(), f.machine, key)
	requireOperationRefusal(t, operationResult{err: err}, connect.CodeUnavailable, operationDatabase)
	after := f.rollbackSnapshot(t)
	for relation, digest := range before {
		if after[relation] != digest {
			t.Errorf("The failed removal changed %s.", relation)
		}
	}
	requireReportHeartbeat(t, f.stream)
	f.exec(t, "DROP TRIGGER fail_removal_orphan_delete ON repositories")
	head := repositoryFeedHead(t, f.reportFixture)
	requireRemovalSuccess(t, f.remove(t.Context(), f.machine, key))
	f.requireRemovedMachine(t, f.machine)
	f.requireRemovedCommand(t, pending, command.GetAttempt())
	f.requireFailure(t, pending, "machine_removed", connect.CodeNotFound, removalMachineMessage)
	requireRepositoryCounts(t, f.reportFixture, 0, 0)
	f.requireFeed(t, head, removalMachineRef(f.machine), removalRepositoryRef(orphan.Id, store.Removed))
}

func testProjectRemovalFencesReport(t *testing.T) {
	f := newRemovalFixture(t, false)
	orphan, _ := seedRepository(t, f.reportFixture, f.machine, repositoryOrigin{})
	op := f.admit(t, "report after removal", true)
	stream := repositoryStreamID(t, f.reportFixture, f.machine)
	head := repositoryFeedHead(t, f.reportFixture)
	release := f.holdFolderWrite(t, false)
	defer release()

	removal := f.startRemoval(t, f.machine)
	remover := awaitRemovalFolderGate(t, f, removal)
	event := startRepositoryEvent(t, f.reportFixture, stream, op, 1, op.facts())
	awaitRemovalBlock(t, f, event, remover, "transactionid")
	release()
	requireRemovalSuccess(t, removal.Wait(t))
	report := event.Wait(t)
	if !errors.Is(report.err, link.ErrUnknownMachine) || report.result.Applied {
		t.Fatal("The losing report applied after removal or missed the removed machine fence.")
	}
	f.requireRemovedCommand(t, op, 1)
	f.requireFailure(t, op, "machine_removed", connect.CodeNotFound, removalMachineMessage)
	requireRepositoryCounts(t, f.reportFixture, 0, 0)
	f.requireFeed(t, head, removalMachineRef(f.machine), removalRepositoryRef(orphan.Id, store.Removed))
}

func TestProjectConcurrentOrphanRemoval(t *testing.T) {
	t.Run("TwoFinalFolders", testProjectTwoFinalRemovals)
	t.Run("AscendingRepositoryLocks", testProjectRemovalLockOrder)
}

func testProjectTwoFinalRemovals(t *testing.T) {
	f := newRemovalFixture(t, false)
	other := f.registerPeer(t, "Second departing machine")
	shared, _ := seedRepository(t, f.reportFixture, f.machine, repositoryOrigin{})
	f.addFolder(t, shared.Id, other.ID, "/workspace/other")
	head := repositoryFeedHead(t, f.reportFixture)
	release := f.holdFolderWrite(t, false)
	defer release()

	first := f.startRemoval(t, f.machine)
	remover := awaitRemovalFolderGate(t, f, first)
	second := f.startRemoval(t, other.ID)
	awaitRemovalBlock(t, f, second, remover, "transactionid")
	requireRepositoryCounts(t, f.reportFixture, 1, 2)
	f.requireFeed(t, head)
	release()
	requireRemovalSuccess(t, first.Wait(t))
	requireRemovalSuccess(t, second.Wait(t))
	f.requireRemovedMachine(t, f.machine)
	f.requireRemovedMachine(t, other.ID)
	requireRepositoryCounts(t, f.reportFixture, 0, 0)
	f.requireFeed(t, head, removalMachineRef(f.machine), removalRepositoryRef(shared.Id, store.Saved),
		removalMachineRef(other.ID), removalRepositoryRef(shared.Id, store.Removed))
}

func testProjectRemovalLockOrder(t *testing.T) {
	f := newRemovalFixture(t, false)
	one, _ := seedRepository(t, f.reportFixture, f.machine, repositoryOrigin{})
	two, _ := seedRepository(t, f.reportFixture, f.machine, repositoryOrigin{})
	lower, higher := one.Id, two.Id
	if lower > higher {
		lower, higher = higher, lower
	}
	head := repositoryFeedHead(t, f.reportFixture)
	probe, err := pgx.ConnectConfig(t.Context(), f.config.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The fixture could not open the independent repository lock probe.")
	}
	t.Cleanup(func() { _ = probe.Close(t.Context()) })
	gate, err := f.observer.Begin(t.Context())
	if err != nil {
		t.Fatal("The fixture could not begin the upper repository lock transaction.")
	}
	defer func() { _ = gate.Rollback(t.Context()) }()
	if _, err := gate.Exec(t.Context(), "SELECT id FROM repositories WHERE id = $1 FOR UPDATE", higher); err != nil {
		t.Fatal("The fixture could not lock the higher repository identifier.")
	}
	removal := f.startRemoval(t, f.machine)
	awaitRemovalBlock(t, f, removal, int32(f.observer.PgConn().PID()), "transactionid")
	_, err = probe.Exec(t.Context(), "SELECT id FROM repositories WHERE id = $1 FOR NO KEY UPDATE NOWAIT", lower)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatal("Removal reached the higher repository identifier before locking the lower one.")
	}
	if err := gate.Commit(t.Context()); err != nil {
		t.Fatal("The fixture could not unlock the higher repository identifier.")
	}
	requireRemovalSuccess(t, removal.Wait(t))
	requireRepositoryCounts(t, f.reportFixture, 0, 0)
	f.requireFeed(t, head, removalMachineRef(f.machine),
		removalRepositoryRef(lower, store.Removed), removalRepositoryRef(higher, store.Removed))
}

func TestProjectAttachAgainstRemoval(t *testing.T) {
	t.Run("AttachmentFirst", func(t *testing.T) { testProjectAttachAgainstRemoval(t, true) })
	t.Run("RemovalFirst", func(t *testing.T) { testProjectAttachAgainstRemoval(t, false) })
}

func testProjectAttachAgainstRemoval(t *testing.T, attachmentFirst bool) {
	f := newRemovalFixture(t, false)
	other := f.registerPeer(t, "Attaching machine")
	f.open(t, other, 0)
	origin := repositoryOrigin{remote: new("https://example.test/shared.git")}
	shared, oldFolder := seedRepository(t, f.reportFixture, f.machine, origin)
	op, caller := startRepositoryAdd(t, f.reportFixture, other.ID, &shared.Id, "~/new checkout")
	stream := repositoryStreamID(t, f.reportFixture, other.ID)
	const resolved = "/workspace/new-checkout"
	seen := origin.report(op, resolved)
	head := repositoryFeedHead(t, f.reportFixture)
	release := f.holdFolderWrite(t, attachmentFirst)
	defer release()

	var removal *storetest.Task[error]
	var event *storetest.Task[repositoryEventResult]
	if attachmentFirst {
		event = startRepositoryEvent(t, f.reportFixture, stream, op, 1, seen)
		attacher := awaitRemovalFolderGate(t, f, event)
		removal = f.startRemoval(t, f.machine)
		awaitRemovalBlock(t, f, removal, attacher, "transactionid")
	} else {
		removal = f.startRemoval(t, f.machine)
		remover := awaitRemovalFolderGate(t, f, removal)
		event = startRepositoryEvent(t, f.reportFixture, stream, op, 1, seen)
		awaitRemovalBlock(t, f, event, remover, "transactionid")
	}
	requireRepositoryCounts(t, f.reportFixture, 1, 1)
	f.requireFeed(t, head)
	release()
	requireRepositoryEvent(t, event.Wait(t), 1)
	requireRemovalSuccess(t, removal.Wait(t))
	f.requireRemovedMachine(t, f.machine)
	f.requireMark(t, other.ID, 1)
	reportReference := store.ChangeRef{Topic: "machines", Key: other.ID, Change: store.Saved}
	if !attachmentFirst {
		requireOperationRefusal(t, caller.Wait(t), connect.CodeNotFound, removalTargetMessage)
		f.requireFailure(t, op, "repository_not_found", connect.CodeNotFound, removalTargetMessage)
		requireRepositoryCounts(t, f.reportFixture, 0, 0)
		f.requireFeed(t, head, reportReference, removalMachineRef(f.machine), removalRepositoryRef(shared.Id, store.Removed))
		return
	}

	row, reply := requireRepositoryAdd(t, f.reportFixture, op, caller.Wait(t))
	requireRepositoryFolder(t, reply.GetFolder(), other.ID, resolved)
	if reply.GetRepository().GetId() != shared.Id || len(reply.GetRepository().GetFolders()) != 2 {
		t.Fatal("The winning attachment did not store its full repository reply before removal.")
	}
	if !slices.ContainsFunc(reply.GetRepository().GetFolders(), func(folder *webv1.RepositoryFolder) bool {
		return proto.Equal(folder, oldFolder)
	}) {
		t.Fatal("The winning attachment reply omitted the old folder that removal later deleted.")
	}
	requireRepositoryCounts(t, f.reportFixture, 1, 1)
	requireRemovalSurvivor(t, f, shared, reply.GetFolder())
	requireOperationReply(t, op.call.run(t.Context(), f.service, op.call.key), row.reply)
	f.requireUnchanged(t, op, row)
	f.requireFeed(t, head, reportReference, removalMachineRef(f.machine),
		removalRepositoryRef(shared.Id, store.Saved), removalRepositoryRef(shared.Id, store.Saved))
}
