package projects_test

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type removalFixture struct {
	*reportFixture
	machines *machines.Service
}

func newRemovalFixture(t *testing.T, dispatch bool) *removalFixture {
	t.Helper()
	f := newReportFixture(t, reportControls{dispatch: dispatch})
	// Register on a fresh state so the signed report fixture keeps its frozen
	// hooks while both paths use the same database, registry and connection hub.
	state := link.NewState(f.db, f.registry, f.hub)
	if err := state.RegisterPart(link.PartHooks{
		CommandSettled: map[link.CommandKind]func(context.Context, *store.Tx, link.Settlement) error{
			link.CommandLookAtFolder: func(ctx context.Context, tx *store.Tx, result link.Settlement) error {
				return f.service.SettleCommand(ctx, tx, result.MachineID, result.Command.GetId(), string(result.Outcome))
			},
		},
		MachineRemoved: f.service.RemoveMachine,
	}); err != nil {
		t.Fatal("The removal fixture could not register the Projects callbacks.")
	}
	return &removalFixture{f, machines.NewService(f.db, nil, f.registry, nil).WithLink(state)}
}

func (f *removalFixture) remove(ctx context.Context, machine, key string) error {
	req := connect.NewRequest(&webv1.RemoveClientRequest{Id: machine})
	if key != "" {
		req.Header().Set(store.RequestKeyHeader, key)
	}
	_, err := f.machines.RemoveClient(ctx, req)
	return err
}

func (f *removalFixture) startRemoval(t *testing.T, machine string) *storetest.Task[error] {
	t.Helper()
	return storetest.Start(t, func() error { return f.remove(t.Context(), machine, "") })
}

func requireRemovalSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("The machine removal failed with code %s.", connect.CodeOf(err))
	}
}

func (f *removalFixture) addFolder(t *testing.T, repository, machine, path string) *webv1.RepositoryFolder {
	t.Helper()
	folder := &webv1.RepositoryFolder{Id: projectMigrationID(t), ClientId: machine, ResolvedPath: path}
	f.exec(t, `INSERT INTO repository_folders (id, repository_id, machine_id, resolved_path)
VALUES ($1, $2, $3, $4)`, folder.Id, repository, machine, path)
	return folder
}

func (f *removalFixture) requireRemovedMachine(t *testing.T, id string) {
	t.Helper()
	if !repositoryObserve[bool](t, f.reportFixture, `
SELECT NOT EXISTS (SELECT FROM machines WHERE id = $1)
   AND NOT EXISTS (SELECT FROM link_machines WHERE machine_id = $1)
   AND NOT EXISTS (SELECT FROM link_connections WHERE machine_id = $1)
   AND NOT EXISTS (SELECT FROM link_runs WHERE machine_id = $1)
   AND NOT EXISTS (SELECT FROM repository_folders WHERE machine_id = $1)`, id) {
		t.Error("Removal left a machine, connection, Link run or attached folder behind.")
	}
}

func (f *removalFixture) requireRemovedCommand(t *testing.T, op reportOperation, attempt uint32) {
	t.Helper()
	f.requireCommand(t, op, attempt, "removed", "", nil)
	if !repositoryObserve[bool](t, f.reportFixture, `
SELECT payload IS NULL AND finished_at IS NOT NULL FROM link_commands WHERE id = $1`, op.command) {
		t.Fatal("Removal did not clear the command payload and stamp the terminal outcome.")
	}
}

func removalMachineRef(id string) store.ChangeRef {
	return store.ChangeRef{Topic: "machines", Key: id, Change: store.Removed}
}

func removalRepositoryRef(id string, change store.Change) store.ChangeRef {
	return store.ChangeRef{Topic: "repositories", Key: id, Change: change}
}

func (f *removalFixture) requireFeed(t *testing.T, head int64, want ...store.ChangeRef) {
	t.Helper()
	rows, err := f.observer.Query(t.Context(), `SELECT topic, scope, key, change FROM feed WHERE position > $1 ORDER BY position`, head)
	if err != nil {
		t.Fatal("The observer could not read the removal feed.")
	}
	defer rows.Close()
	expected := make(map[store.ChangeRef]int, len(want))
	for _, ref := range want {
		expected[ref]++
	}
	count := 0
	for rows.Next() {
		var ref store.ChangeRef
		if rows.Scan(&ref.Topic, &ref.Scope, &ref.Key, &ref.Change) != nil {
			t.Fatal("The observer could not decode a removal feed reference.")
		}
		count++
		if expected[ref] == 0 {
			t.Fatal("The removal published an unexpected, duplicate or incorrectly scoped feed reference.")
		}
		expected[ref]--
	}
	if rows.Err() != nil || count != len(want) || count > store.MaxWriteChanges {
		t.Fatalf("Expected %d committed feed references within the write budget, read %d.", len(want), count)
	}
}

func (f *removalFixture) rollbackSnapshot(t *testing.T) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	for _, relation := range []string{
		"machines", "link_machines", "link_connections", "link_runs", "link_commands",
		"folder_operations", "receipts", "repositories", "repository_folders", "feed",
	} {
		snapshot[relation] = repositoryObserve[string](t, f.reportFixture, `
SELECT md5(COALESCE(string_agg(row_data, E'\n' ORDER BY row_data), ''))
FROM (SELECT to_jsonb(row)::text AS row_data FROM `+relation+` row) AS rows`)
	}
	return snapshot
}

const removalAdvisoryKey int64 = 74326

func (f *removalFixture) holdFolderWrite(t *testing.T, insert bool) func() {
	t.Helper()
	event, row := "DELETE", "OLD"
	if insert {
		event, row = "INSERT", "NEW"
	}
	f.exec(t, fmt.Sprintf(`
CREATE FUNCTION hold_removal_folder_write() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_advisory_xact_lock(%d::bigint);
  RETURN %s;
END
$$;
CREATE TRIGGER hold_removal_folder_write BEFORE %s ON repository_folders
FOR EACH ROW EXECUTE FUNCTION hold_removal_folder_write()`, removalAdvisoryKey, row, event))
	return storetest.HoldAdvisoryGate(t, f.observer, removalAdvisoryKey)
}

func awaitRemovalBlock[R any](t *testing.T, f *removalFixture, task *storetest.Task[R], blocker int32, lockType string) int32 {
	t.Helper()
	var waiter int32
	storetest.Poll(t, "the competing write waiting on the expected PostgreSQL lock", func() bool {
		select {
		case <-task.Done():
			t.Fatal("The competing write returned before waiting on the expected PostgreSQL lock.")
		default:
		}
		waiter = repositoryObserve[int32](t, f.reportFixture, `
SELECT COALESCE((
    SELECT pid FROM pg_locks
    WHERE NOT granted AND locktype = $2 AND $1 = ANY(pg_blocking_pids(pid))
    LIMIT 1
), 0)`, blocker, lockType)
		return waiter != 0
	})
	return waiter
}

func awaitRemovalFolderGate[R any](t *testing.T, f *removalFixture, task *storetest.Task[R]) int32 {
	t.Helper()
	return awaitRemovalBlock(t, f, task, int32(f.observer.PgConn().PID()), "advisory")
}
