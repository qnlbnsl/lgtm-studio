package projects

import (
	"context"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const repositoryRefreshKey = "refresh"

type attachmentRefusal string

const (
	repositoryNotFound   attachmentRefusal = "repository_not_found"
	folderOwnedElsewhere attachmentRefusal = "folder_owned_elsewhere"
	repositoryMismatch   attachmentRefusal = "repository_mismatch"
)

func (refusal attachmentRefusal) Error() string { return string(refusal) }

func repositoryMatchesFolder(repository *webv1.Repository, facts *webv1.FolderFacts) bool {
	if repository.Remote != nil && facts.Remote != nil {
		saved, savedValid := normalizeOrigin(*repository.Remote)
		reported, reportedValid := normalizeOrigin(*facts.Remote)
		return savedValid && reportedValid && saved == reported
	}
	savedAbsent := repository.Remote == nil && !repository.RemoteOmitted
	reportedAbsent := facts.Remote == nil && !facts.RemoteOmitted
	return (savedAbsent || reportedAbsent) && repository.FirstCommit != nil && facts.FirstCommit != nil &&
		*repository.FirstCommit == *facts.FirstCommit
}

func insertRepository(ctx context.Context, tx *store.Tx, facts *webv1.FolderFacts) (string, error) {
	id, err := store.NewID()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO repositories (id, name, main_branch, remote, first_commit, remote_omitted)
VALUES ($1, $2, $3, $4, $5, $6)`, id, facts.Name, facts.MainBranch, facts.Remote, facts.FirstCommit, facts.RemoteOmitted)
	return id, err
}

// loadRepository reads a complete repository projection from a single database snapshot.
func loadRepository(ctx context.Context, q machines.Querier, id string) (*webv1.Repository, bool, error) {
	rows, err := q.Query(ctx, `
SELECT r.name, r.main_branch, r.remote, r.first_commit, r.remote_omitted,
       f.id, f.machine_id, f.resolved_path
FROM repositories r LEFT JOIN repository_folders f ON f.repository_id = r.id
WHERE r.id = $1
ORDER BY f.id`, id)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var repository *webv1.Repository
	for rows.Next() {
		if repository == nil {
			repository = &webv1.Repository{Id: id}
		}
		var folderID, machineID, resolvedPath *string
		err := rows.Scan(&repository.Name, &repository.MainBranch, &repository.Remote, &repository.FirstCommit,
			&repository.RemoteOmitted, &folderID, &machineID, &resolvedPath)
		if err != nil {
			return nil, false, err
		}
		if folderID != nil {
			repository.Folders = append(repository.Folders, &webv1.RepositoryFolder{
				Id: *folderID, ClientId: *machineID, ResolvedPath: *resolvedPath,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return repository, repository != nil, nil
}
