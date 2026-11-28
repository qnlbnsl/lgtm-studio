package projects

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// RemoveMachine fails open folder operations and removes repository membership
// in Link's write while Machines holds the machine lock.
func (s *Service) RemoveMachine(ctx context.Context, tx *store.Tx, machineID string) error {
	if _, err := tx.Exec(ctx, `
UPDATE folder_operations SET state = 'failed', reply = NULL, failure = 'machine_removed'
WHERE machine_id = $1 AND state = 'open'`, machineID); err != nil {
		return err
	}

	// Attachments and removals share these locks. Sorting the repository rows
	// makes concurrent removals acquire overlapping locks in the same order.
	rows, err := tx.Query(ctx, `
SELECT id FROM repositories
WHERE id IN (SELECT repository_id FROM repository_folders WHERE machine_id = $1)
ORDER BY id
FOR UPDATE`, machineID)
	if err != nil {
		return err
	}
	repositoryIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(repositoryIDs) == 0 {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM repository_folders WHERE machine_id = $1`, machineID); err != nil {
		return err
	}
	orphanIDs, err := deleteOrphanRepositories(ctx, tx, repositoryIDs)
	if err != nil {
		return err
	}

	// Leave room for the Machines reference and one spare reference.
	if len(repositoryIDs) > store.MaxWriteChanges-2 {
		return tx.Record(store.ChangeRef{Topic: "repositories", Key: repositoryRefreshKey, Change: store.Saved})
	}

	orphans := make(map[string]struct{}, len(orphanIDs))
	for _, id := range orphanIDs {
		orphans[id] = struct{}{}
	}
	for _, id := range repositoryIDs {
		change := store.Saved
		if _, removed := orphans[id]; removed {
			change = store.Removed
		}
		if err := tx.Record(store.ChangeRef{Topic: "repositories", Key: id, Change: change}); err != nil {
			return err
		}
	}
	return nil
}

// deleteOrphanRepositories checks for remaining folders after the caller has
// locked the repositories and removed the machine's folders.
func deleteOrphanRepositories(ctx context.Context, tx *store.Tx, repositoryIDs []string) ([]string, error) {
	rows, err := tx.Query(ctx, `
DELETE FROM repositories r
WHERE r.id = ANY($1) AND NOT EXISTS (
    SELECT 1 FROM repository_folders f WHERE f.repository_id = r.id
)
RETURNING r.id`, repositoryIDs)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
