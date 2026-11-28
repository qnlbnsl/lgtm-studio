package projects

import (
	"context"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const maxResolvedPathBytes = 8192

var errInvalidFolderReply = store.NewDomainError(messageProjectInternal)

// ApplyFolderSeen applies a report in Link's write under the authenticated machine lock.
func (s *Service) ApplyFolderSeen(ctx context.Context, tx *store.Tx, machineID string, seen *linkv1.FolderSeen) error {
	if !projectID.MatchString(seen.GetOperationId()) || !projectID.MatchString(seen.GetCommandId()) {
		return nil
	}
	var facts *webv1.FolderFacts
	var failure string
	switch result := seen.GetResult().(type) {
	case *linkv1.FolderSeen_Facts:
		if result == nil {
			return nil
		}
		facts = translateFolderFacts(result.Facts)
		if facts == nil {
			return nil
		}
	case *linkv1.FolderSeen_Refusal:
		if result == nil {
			return nil
		}
		failure = folderReportFailure(result.Refusal)
		if failure == "" {
			return nil
		}
	default:
		return nil
	}

	var id, kind string
	var targetID *string
	err := tx.QueryRow(ctx, `
SELECT id, kind, target_repository_id FROM folder_operations
WHERE id = $1 AND command_id = $2 AND machine_id = $3 AND state = 'open'
FOR UPDATE`, seen.GetOperationId(), seen.GetCommandId(), machineID).Scan(&id, &kind, &targetID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if failure != "" {
		_, err := tx.Exec(ctx, `UPDATE folder_operations SET state = 'failed', reply = NULL, failure = $2 WHERE id = $1`, id, failure)
		return err
	}

	var response proto.Message
	switch kind {
	case "inspect":
		response = &webv1.InspectFolderResponse{Facts: facts}
	case "add":
		add, err := s.attachRepository(ctx, tx, machineID, targetID, facts)
		if refusal, ok := err.(attachmentRefusal); ok {
			_, err := tx.Exec(ctx, `UPDATE folder_operations SET state = 'failed', reply = NULL, failure = $2 WHERE id = $1`, id, string(refusal))
			return err
		}
		if err != nil {
			return err
		}
		response = add
	default:
		return errInvalidFolderReply
	}
	reply, err := proto.Marshal(response)
	if err != nil {
		return errInvalidFolderReply
	}
	_, err = tx.Exec(ctx, `UPDATE folder_operations SET state = 'succeeded', reply = $2, failure = NULL WHERE id = $1`, id, reply)
	return err
}

// SettleCommand records the outcome of a folder command that still awaits a valid report.
func (s *Service) SettleCommand(ctx context.Context, tx *store.Tx, machineID, commandID, outcome string) error {
	var failure string
	switch outcome {
	case "done", "failed":
		failure = "no_answer"
	case "expired", "unsupported":
		failure = "not_taken"
	case "interrupted", "timed_out":
		failure = "unknown"
	case "removed":
		failure = "machine_removed"
	default:
		return nil
	}
	_, err := tx.Exec(ctx, `
UPDATE folder_operations SET state = 'failed', reply = NULL, failure = $3
WHERE machine_id = $1 AND command_id = $2 AND state = 'open'`, machineID, commandID, failure)
	return err
}

func folderReportFailure(refusal linkv1.FolderRefusal) string {
	switch refusal {
	case linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER:
		return "not_a_folder"
	case linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY:
		return "not_a_git_repository"
	case linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE:
		return "not_readable"
	case linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE:
		return "path_not_usable"
	default:
		return ""
	}
}

func translateFolderFacts(facts *linkv1.FolderFacts) *webv1.FolderFacts {
	if facts == nil {
		return nil
	}
	resolved := facts.GetResolvedPath()
	if len(resolved) > maxResolvedPathBytes || !path.IsAbs(resolved) ||
		path.Clean(resolved) != resolved || !validFolderFactText(resolved) ||
		facts.GetName() != path.Base(resolved) || !validFolderFactText(facts.GetMainBranch()) ||
		(facts.CheckedOutBranch != nil && !validFolderFactText(*facts.CheckedOutBranch)) ||
		(facts.FirstCommit != nil && !validFolderFactText(*facts.FirstCommit)) {
		return nil
	}
	if facts.Remote != nil {
		if _, valid := normalizeOrigin(*facts.Remote); facts.GetRemoteOmitted() || !valid {
			return nil
		}
	}
	return &webv1.FolderFacts{
		ResolvedPath:     resolved,
		Name:             facts.GetName(),
		MainBranch:       facts.GetMainBranch(),
		CheckedOutBranch: facts.CheckedOutBranch,
		Remote:           facts.Remote,
		FirstCommit:      facts.FirstCommit,
		Clean:            facts.GetClean(),
		RemoteOmitted:    facts.GetRemoteOmitted(),
	}
}

func validFolderFactText(value string) bool {
	return value != "" && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

// attachRepository reuses or saves a checkout while Link holds the reporting machine lock.
func (s *Service) attachRepository(ctx context.Context, tx *store.Tx, machineID string, targetID *string, facts *webv1.FolderFacts) (*webv1.AddRepositoryResponse, error) {
	folder := &webv1.RepositoryFolder{ClientId: machineID, ResolvedPath: facts.GetResolvedPath()}
	var repositoryID string
	err := tx.QueryRow(ctx, `
SELECT id, repository_id FROM repository_folders
WHERE machine_id = $1 AND resolved_path = $2`, machineID, folder.ResolvedPath).Scan(&folder.Id, &repositoryID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	saved := err == nil

	if targetID != nil {
		// Lock the explicit target before deciding path ownership so that an unknown
		// target gets the same refusal for either path state.
		var target webv1.Repository
		err := tx.QueryRow(ctx, `
SELECT remote, first_commit, remote_omitted FROM repositories WHERE id = $1 FOR UPDATE`, *targetID).
			Scan(&target.Remote, &target.FirstCommit, &target.RemoteOmitted)
		if err == pgx.ErrNoRows {
			return nil, repositoryNotFound
		}
		if err != nil {
			return nil, err
		}
		if saved && repositoryID != *targetID {
			return nil, folderOwnedElsewhere
		}
		if !saved && !repositoryMatchesFolder(&target, facts) {
			return nil, repositoryMismatch
		}
		repositoryID = *targetID
	}

	if !saved {
		if targetID == nil {
			repositoryID, err = insertRepository(ctx, tx, facts)
			if err != nil {
				return nil, err
			}
		}
		folder.Id, err = store.NewID()
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `
INSERT INTO repository_folders (id, repository_id, machine_id, resolved_path) VALUES ($1, $2, $3, $4)`,
			folder.Id, repositoryID, folder.ClientId, folder.ResolvedPath)
		if err != nil {
			return nil, err
		}
		if err := tx.Record(store.ChangeRef{Topic: "repositories", Key: repositoryID, Change: store.Saved}); err != nil {
			return nil, err
		}
	}

	repository, found, err := loadRepository(ctx, tx, repositoryID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errInvalidFolderReply
	}
	return &webv1.AddRepositoryResponse{Repository: repository, Folder: folder}, nil
}
