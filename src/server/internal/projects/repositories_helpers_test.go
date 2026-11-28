package projects_test

import (
	"context"
	"path"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type repositoryOrigin struct {
	remote, first *string
	omitted       bool
}

func (o repositoryOrigin) report(op reportOperation, resolved string) *linkv1.FolderSeen {
	seen := op.facts()
	facts := seen.GetFacts()
	facts.ResolvedPath = resolved
	facts.Name = path.Base(resolved)
	facts.Remote, facts.RemoteOmitted, facts.FirstCommit = o.remote, o.omitted, o.first
	return seen
}

func repositoryCall(key, machineID, typed string, target *string) operationCall {
	return operationCall{key: key, machine: machineID, path: typed, target: target, add: true}
}

func startRepositoryAdd(t *testing.T, f *reportFixture, machineID string, target *string, typed string) (reportOperation, *storetest.Task[operationResult]) {
	t.Helper()
	call := repositoryCall(projectMigrationID(t), machineID, typed, target)
	caller := call.start(t, t.Context(), f.service)
	id, command := f.awaitOperation(t, call, caller)
	return reportOperation{call: call, id: id, command: command}, caller
}

func applyRepositoryAdd(t *testing.T, f *reportFixture, machineID, streamID string, mark *uint64, target *string,
	resolved string, origin repositoryOrigin) (reportOperation, *webv1.AddRepositoryResponse) {
	t.Helper()
	op, caller := startRepositoryAdd(t, f, machineID, target, "~/typed  folder ")
	applyRepositoryEvent(t, f, streamID, op, mark, origin.report(op, resolved))
	_, reply := requireRepositoryAdd(t, f, op, caller.Wait(t))
	return op, reply
}

func repositoryEvent(mark uint64, seen *linkv1.FolderSeen) *linkv1.Event {
	return &linkv1.Event{Number: mark, Report: &linkv1.Event_FolderSeen{FolderSeen: seen}}
}

type repositoryEventResult struct {
	result link.EventResult
	err    error
}

func startRepositoryEvent(t *testing.T, f *reportFixture, streamID string, op reportOperation, mark uint64,
	seen *linkv1.FolderSeen) *storetest.Task[repositoryEventResult] {
	t.Helper()
	return storetest.Start(t, func() repositoryEventResult {
		result, err := f.state.ApplyEvent(t.Context(), op.call.machine, streamID, repositoryEvent(mark, seen))
		return repositoryEventResult{result: result, err: err}
	})
}

func requireRepositoryEvent(t *testing.T, got repositoryEventResult, mark uint64) {
	t.Helper()
	if got.err != nil || !got.result.Applied || got.result.Mark != mark {
		t.Fatal("The Add report did not commit an attachment decision and advance the Link mark.")
	}
}

func applyRepositoryEvent(t *testing.T, f *reportFixture, streamID string, op reportOperation, mark *uint64, seen *linkv1.FolderSeen) {
	t.Helper()
	next := *mark + 1
	result, err := f.state.ApplyEvent(t.Context(), op.call.machine, streamID, repositoryEvent(next, seen))
	requireRepositoryEvent(t, repositoryEventResult{result: result, err: err}, next)
	*mark = next
	f.requireMark(t, op.call.machine, next)
}

func applyRepositoryEventUnderTargetLock(t *testing.T, f *reportFixture, streamID string, op reportOperation, mark *uint64, seen *linkv1.FolderSeen) {
	t.Helper()
	gate, err := f.observer.Begin(t.Context())
	if err != nil {
		t.Fatal("The fixture could not begin the explicit target lock transaction.")
	}
	defer func() { _ = gate.Rollback(context.Background()) }()
	// A key-share lock conflicts with FOR UPDATE, so a plain target lookup
	// would let the request return before the locked ownership check.
	if _, err := gate.Exec(t.Context(), "SELECT id FROM repositories WHERE id = $1 FOR KEY SHARE", *op.call.target); err != nil {
		t.Fatal("The fixture could not lock the explicit target.")
	}
	next := *mark + 1
	event := startRepositoryEvent(t, f, streamID, op, next, seen)
	awaitRepositoryBlock(t, f, event, int32(f.observer.PgConn().PID()))
	if err := gate.Commit(t.Context()); err != nil {
		t.Fatal("The fixture could not release the explicit target.")
	}
	requireRepositoryEvent(t, event.Wait(t), next)
	*mark = next
	f.requireMark(t, op.call.machine, next)
}

func awaitRepositoryBlock(t *testing.T, f *reportFixture, event *storetest.Task[repositoryEventResult], blocker int32) {
	t.Helper()
	storetest.Poll(t, "the competing report waiting for the database lock", func() bool {
		select {
		case <-event.Done():
			t.Fatal("The competing report returned before waiting for the expected database lock.")
		default:
		}
		return repositoryObserve[bool](t, f, `
SELECT EXISTS (SELECT FROM pg_locks WHERE pid IS NOT NULL AND NOT granted
               AND $1 = ANY(pg_blocking_pids(pid)))`, blocker)
	})
}

func requireRepositoryAdd(t *testing.T, f *reportFixture, op reportOperation, result operationResult) (reportRow, *webv1.AddRepositoryResponse) {
	t.Helper()
	row := f.row(t, op)
	var reply webv1.AddRepositoryResponse
	if row.state != "succeeded" || row.failure != "" || proto.Unmarshal(row.reply, &reply) != nil ||
		reply.GetRepository() == nil || reply.GetFolder() == nil {
		t.Fatal("The Add operation did not store a complete successful repository reply.")
	}
	requireOperationReply(t, result, row.reply)
	return row, &reply
}

func requireRepositoryFolder(t *testing.T, folder *webv1.RepositoryFolder, machineID, resolved string) {
	t.Helper()
	if !operationUUID.MatchString(folder.GetId()) || folder.GetClientId() != machineID || folder.GetResolvedPath() != resolved {
		t.Fatal("The saved folder did not have a server identifier, reporting machine and exact resolved path.")
	}
}

func requireRepositoryMetadata(t *testing.T, got, want *webv1.Repository) {
	t.Helper()
	metadata := proto.CloneOf(got)
	metadata.Folders = nil
	if !proto.Equal(metadata, want) {
		t.Fatal("The repository reply did not preserve its saved name, branch and origin metadata.")
	}
}

func requireRepositoryProjection(t *testing.T, f *reportFixture, repository *webv1.Repository, folders ...*webv1.RepositoryFolder) {
	t.Helper()
	if !operationUUID.MatchString(repository.GetId()) || len(repository.GetFolders()) != len(folders) {
		t.Fatal("The repository reply did not have a server identifier and all of its folders.")
	}
	stored := &webv1.Repository{Id: repository.GetId()}
	err := f.observer.QueryRow(t.Context(), `
SELECT name, main_branch, remote, first_commit, remote_omitted FROM repositories WHERE id = $1`,
		stored.Id).Scan(&stored.Name, &stored.MainBranch, &stored.Remote, &stored.FirstCommit, &stored.RemoteOmitted)
	if err != nil {
		t.Fatal("The independent observer could not read the saved repository.")
	}
	requireRepositoryMetadata(t, repository, stored)
	count := repositoryObserve[int](t, f, "SELECT count(*) FROM repository_folders WHERE repository_id = $1", stored.Id)
	if count != len(folders) {
		t.Fatal("The database did not contain exactly the expected membership.")
	}
	for _, folder := range folders {
		if !slices.ContainsFunc(repository.GetFolders(), func(candidate *webv1.RepositoryFolder) bool {
			return proto.Equal(candidate, folder)
		}) {
			t.Fatal("The repository reply omitted a saved folder or changed its identity.")
		}
		if !repositoryObserve[bool](t, f, `SELECT EXISTS (SELECT FROM repository_folders
WHERE repository_id = $1 AND id = $2 AND machine_id = $3 AND resolved_path = $4)`,
			stored.Id, folder.GetId(), folder.GetClientId(), folder.GetResolvedPath()) {
			t.Fatal("The reply's folder did not match its stored membership.")
		}
	}
}

func seedRepository(t *testing.T, f *reportFixture, machineID string, origin repositoryOrigin) (*webv1.Repository, *webv1.RepositoryFolder) {
	t.Helper()
	repository := &webv1.Repository{Id: projectMigrationID(t), Name: "Original name", MainBranch: "stable",
		Remote: origin.remote, RemoteOmitted: origin.omitted, FirstCommit: origin.first}
	folder := &webv1.RepositoryFolder{Id: projectMigrationID(t), ClientId: machineID, ResolvedPath: "/seed/" + repository.GetId()}
	f.exec(t, `INSERT INTO repositories (id, name, main_branch, remote, first_commit, remote_omitted)
VALUES ($1, $2, $3, $4, $5, $6)`, repository.Id, repository.Name, repository.MainBranch,
		repository.Remote, repository.FirstCommit, repository.GetRemoteOmitted())
	f.exec(t, `INSERT INTO repository_folders (id, repository_id, machine_id, resolved_path)
VALUES ($1, $2, $3, $4)`, folder.Id, repository.Id, folder.ClientId, folder.ResolvedPath)
	return repository, folder
}

type repositoryCounts struct {
	repositories, folders int
}

func readRepositoryCounts(t *testing.T, f *reportFixture) repositoryCounts {
	t.Helper()
	var counts repositoryCounts
	if err := f.observer.QueryRow(t.Context(), "SELECT (SELECT count(*) FROM repositories), (SELECT count(*) FROM repository_folders)").
		Scan(&counts.repositories, &counts.folders); err != nil {
		t.Fatal("The observer could not count the repository and folder rows.")
	}
	return counts
}

func requireRepositoryCounts(t *testing.T, f *reportFixture, repositories, folders int) {
	t.Helper()
	if got := readRepositoryCounts(t, f); got != (repositoryCounts{repositories, folders}) {
		t.Fatalf("Expected %d repositories and %d folders, got %d and %d.", repositories, folders, got.repositories, got.folders)
	}
}

func repositoryFeedHead(t *testing.T, f *reportFixture) int64 {
	t.Helper()
	return repositoryObserve[int64](t, f, "SELECT COALESCE(max(position), 0) FROM feed")
}

func requireRepositorySaves(t *testing.T, f *reportFixture, want ...string) {
	t.Helper()
	requireRepositorySavesAfter(t, f, 0, want...)
}

func requireRepositorySavesAfter(t *testing.T, f *reportFixture, head int64, want ...string) {
	t.Helper()
	rows, err := f.observer.Query(t.Context(),
		"SELECT scope, key, change FROM feed WHERE topic = 'repositories' AND position > $1 ORDER BY position", head)
	if err != nil {
		t.Fatal("The observer could not query the committed repository references.")
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var scope, key, change string
		if rows.Scan(&scope, &key, &change) != nil || index >= len(want) ||
			scope != "" || key != want[index] || change != "saved" {
			t.Fatal("The repository feed contained an unexpected reference.")
		}
		index++
	}
	if rows.Err() != nil || index != len(want) {
		t.Fatal("The repository feed did not contain exactly one saved reference per expected insertion.")
	}
}

func repositoryStreamID(t *testing.T, f *reportFixture, machineID string) string {
	t.Helper()
	return repositoryObserve[string](t, f, "SELECT stream_id FROM link_connections WHERE machine_id = $1", machineID)
}

func repositoryObserve[V any](t *testing.T, f *reportFixture, sql string, args ...any) V {
	t.Helper()
	var result V
	if err := f.observer.QueryRow(t.Context(), sql, args...).Scan(&result); err != nil {
		t.Fatal("The independent observer could not read the attachment fixture state.")
	}
	return result
}
