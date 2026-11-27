package projects_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth/authtest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
)

func TestProjectMigration(t *testing.T) {
	ctx := t.Context()
	at := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	previous := []store.Migrations{auth.Migrations(), vault.Migrations(), machines.Migrations(), link.Migrations()}
	s, observer, setClock := authtest.ClockStore(t, at, previous...)
	requireProjectMigrationTables(t, observer, false)

	checkLegacy := seedProjectUpgrade(t, s)
	next := append(previous, projects.Migrations())

	// Refuse the ledger write after the migration's statements have executed.
	storetest.Exec(t, observer, `
CREATE FUNCTION refuse_project_migration() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.part = 'projects' THEN
        RAISE EXCEPTION 'The fixture interrupted the Projects migration.' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER refuse_project_migration BEFORE INSERT ON schema_migrations
FOR EACH ROW EXECUTE FUNCTION refuse_project_migration();`)
	var migrationError *store.Error
	if err := s.Migrate(ctx, next...); !errors.As(err, &migrationError) || migrationError.SQLState != "23514" {
		t.Fatal("The Projects ledger fault did not interrupt migration with a check violation.")
	}
	requireProjectMigrationTables(t, observer, false)
	if storetest.Scalar[bool](t, observer, "SELECT EXISTS (SELECT FROM schema_migrations WHERE part = 'projects')") {
		t.Fatal("The interrupted migration left a Projects ledger entry.")
	}
	checkLegacy()

	storetest.Exec(t, observer, "DROP TRIGGER refuse_project_migration ON schema_migrations")
	if err := s.Migrate(ctx, next...); err != nil {
		t.Fatal("The previous server database could not apply the Projects migration.")
	}
	requireProjectMigrationTables(t, observer, true)
	for _, table := range []string{"repositories", "repository_folders", "folder_operations"} {
		if got := storetest.Scalar[int](t, observer, "SELECT count(*) FROM "+table); got != 0 {
			t.Errorf("Table %s has %d rows after upgrade, want zero.", table, got)
		}
	}
	if got := storetest.Scalar[int](t, observer, "SELECT version FROM schema_migrations WHERE part = 'projects'"); got != 1 {
		t.Fatalf("The Projects ledger records version %d, want 1.", got)
	}
	checkLegacy()

	repositoryID, folderID, operationID := projectMigrationID(t), projectMigrationID(t), projectMigrationID(t)
	machineID, commandID := projectMigrationID(t), projectMigrationID(t)
	storetest.Exec(t, observer, `
INSERT INTO repositories (id, name, main_branch, remote, first_commit, remote_omitted)
VALUES ($1, 'sample', 'main', NULL, 'first-commit', true)`, repositoryID)
	storetest.Exec(t, observer, `
INSERT INTO repository_folders (id, repository_id, machine_id, resolved_path)
VALUES ($1, $2, $3, '/workspace/sample')`, folderID, repositoryID, machineID)
	execProjectMigrationSQL(t, observer, `
INSERT INTO folder_operations (id, machine_id, request_key, kind, target_repository_id, command_id, state, reply)
VALUES ($1, $2, 'sample-add', 'add', $3, $4, 'succeeded', $5)`,
		operationID, machineID, repositoryID, commandID, []byte{1, 2, 3})

	if err := s.Migrate(ctx, next...); err != nil {
		t.Fatal("Repeating the server migration with saved Projects rows failed.")
	}
	if !storetest.Scalar[bool](t, observer, `
SELECT EXISTS (
    SELECT FROM repositories r
    JOIN repository_folders f ON f.repository_id = r.id
    JOIN folder_operations o ON o.target_repository_id = r.id AND o.machine_id = f.machine_id
    WHERE r.id = $1 AND r.name = 'sample' AND r.main_branch = 'main'
      AND r.remote IS NULL AND r.first_commit = 'first-commit' AND r.remote_omitted
      AND f.id = $2 AND f.machine_id = $3 AND f.resolved_path = '/workspace/sample'
      AND o.id = $4 AND o.command_id = $5 AND o.kind = 'add'
      AND o.state = 'succeeded' AND o.reply = $6 AND o.failure IS NULL
      AND o.retention_started_at IS NULL
)`, repositoryID, folderID, machineID, operationID, commandID, []byte{1, 2, 3}) {
		t.Fatal("Repeating the migration did not preserve the saved repository, folder and operation.")
	}
	if got := storetest.Scalar[int](t, observer, "SELECT version FROM schema_migrations WHERE part = 'projects'"); got != 1 {
		t.Fatalf("The repeated migration records Projects version %d, want 1.", got)
	}
	if err := s.Migrate(ctx, previous...); err == nil {
		t.Fatal("The previous manifest accepted a database with the Projects ledger entry.")
	}
	checkLegacy()

	if _, err := s.Prune(ctx); err != nil {
		t.Fatal("The first pruning pass failed to observe the legacy receipt.")
	}
	setClock(at.Add(24*time.Hour - time.Microsecond))
	if result, err := s.Prune(ctx); err != nil || result.ReceiptRows != 0 {
		t.Fatal("The legacy receipt did not survive until its ordinary retention cutoff.")
	}
	checkLegacy()
	setClock(at.Add(24*time.Hour + time.Microsecond))
	if result, err := s.Prune(ctx); err != nil || result.ReceiptRows != 1 {
		t.Fatal("The legacy receipt did not retire after its ordinary retention cutoff.")
	}
}

func TestProjectStorageConstraints(t *testing.T) {
	at := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	_, observer, _ := authtest.ClockStore(t, at, projects.Migrations())
	requireProjectMigrationTables(t, observer, true)

	const insertRepo = `
INSERT INTO repositories (id, name, main_branch, remote, first_commit, remote_omitted)
VALUES ($1, $2, $3, $4, $5, $6)`
	repositoryID, machineID := projectMigrationID(t), projectMigrationID(t)
	execProjectMigrationSQL(t, observer, insertRepo, repositoryID, "sample", "main", nil, nil, false)
	for _, tc := range []struct {
		name     string
		repoName string
		branch   string
		remote   any
		commit   any
		omitted  bool
	}{
		{name: "empty repository name", branch: "main"},
		{name: "empty main branch", repoName: "sample"},
		{name: "empty present remote", repoName: "sample", branch: "main", remote: ""},
		{name: "empty present first commit", repoName: "sample", branch: "main", commit: ""},
		{name: "withheld remote with text", repoName: "sample", branch: "main", remote: "https://example.com/sample", omitted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rejectProjectMigrationSQL(t, observer, "23514", insertRepo, projectMigrationID(t),
				tc.repoName, tc.branch, tc.remote, tc.commit, tc.omitted)
		})
	}

	const insertFolder = `
INSERT INTO repository_folders (id, repository_id, machine_id, resolved_path)
VALUES ($1, $2, $3, $4)`
	t.Run("orphan folder", func(t *testing.T) {
		rejectProjectMigrationSQL(t, observer, "23503", insertFolder, projectMigrationID(t),
			projectMigrationID(t), machineID, "/workspace/orphan")
	})
	t.Run("empty resolved path", func(t *testing.T) {
		rejectProjectMigrationSQL(t, observer, "23514", insertFolder, projectMigrationID(t), repositoryID, machineID, "")
	})
	t.Run("resolved path over byte limit", func(t *testing.T) {
		rejectProjectMigrationSQL(t, observer, "23514", insertFolder, projectMigrationID(t),
			repositoryID, machineID, "/"+strings.Repeat("é", 4096))
	})

	// The deterministic path has enough entropy to expose a full-path index.
	const pathAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	random := rand.New(rand.NewPCG(1, 2))
	path := make([]byte, 8192)
	path[0] = '/'
	for i := 1; i < len(path); i++ {
		path[i] = pathAlphabet[random.IntN(len(pathAlphabet))]
	}
	folderID := projectMigrationID(t)
	execProjectMigrationSQL(t, observer, insertFolder, folderID, repositoryID, machineID, string(path))
	execProjectMigrationSQL(t, observer, insertFolder, projectMigrationID(t), repositoryID, machineID, "/workspace/second-checkout")
	if !storetest.Scalar[bool](t, observer, `
SELECT resolved_path = $1 AND octet_length(resolved_path) = 8192 AND COLLATION FOR (resolved_path) = '"C"'
FROM repository_folders WHERE id = $2`, string(path), folderID) {
		t.Fatal("The folder did not keep the full 8,192-byte path under the C collation.")
	}
	t.Run("restrict deletion with a saved folder", func(t *testing.T) {
		rejectProjectMigrationSQL(t, observer, "23503", "DELETE FROM repositories WHERE id = $1", repositoryID)
	})

	const insertOperation = `
INSERT INTO folder_operations
    (id, machine_id, request_key, kind, target_repository_id, command_id, state, reply, failure, retention_started_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	commandID := projectMigrationID(t)
	execProjectMigrationSQL(t, observer, insertOperation, projectMigrationID(t), machineID,
		"first-operation", "inspect", nil, commandID, "open", nil, nil, nil)
	execProjectMigrationSQL(t, observer, insertOperation, projectMigrationID(t), machineID,
		"new-repository-operation", "add", nil, projectMigrationID(t), "open", nil, nil, nil)
	execProjectMigrationSQL(t, observer, insertOperation, projectMigrationID(t), machineID,
		"successful-operation", "add", projectMigrationID(t), projectMigrationID(t), "succeeded", []byte{1}, nil, nil)
	execProjectMigrationSQL(t, observer, insertOperation, projectMigrationID(t), machineID,
		"failed-operation", "inspect", nil, projectMigrationID(t), "failed", nil, "not_a_folder", at)
	t.Run("duplicate command identifier", func(t *testing.T) {
		rejectProjectMigrationSQL(t, observer, "23505", insertOperation, projectMigrationID(t), projectMigrationID(t),
			"another-operation", "inspect", nil, commandID, "open", nil, nil, nil)
	})
	for _, tc := range []struct {
		name, kind, state string
		target, failure   any
		reply             []byte
		retention         any
	}{
		{name: "unknown kind", kind: "other", state: "open"},
		{name: "unknown state", kind: "inspect", state: "other"},
		{name: "inspect with a target", kind: "inspect", state: "open", target: repositoryID},
		{name: "open with a reply", kind: "inspect", state: "open", reply: []byte{1}},
		{name: "open with a failure", kind: "inspect", state: "open", failure: "not_a_folder"},
		{name: "open with a retention stamp", kind: "inspect", state: "open", retention: at},
		{name: "success missing a reply", kind: "inspect", state: "succeeded"},
		{name: "failure missing a reason", kind: "inspect", state: "failed"},
		{name: "success with both results", kind: "inspect", state: "succeeded", reply: []byte{1}, failure: "not_a_folder"},
		{name: "failure with both results", kind: "inspect", state: "failed", reply: []byte{1}, failure: "not_a_folder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rejectProjectMigrationSQL(t, observer, "23514", insertOperation, projectMigrationID(t), machineID,
				"invalid-operation", tc.kind, tc.target, projectMigrationID(t), tc.state, tc.reply, tc.failure, tc.retention)
		})
	}

	for _, index := range []struct {
		table, first, second string
		partial, singleKey   bool
	}{
		{table: "repository_folders", first: "repository_id"},
		{table: "repository_folders", first: "machine_id", singleKey: true},
		{table: "folder_operations", first: "machine_id", partial: true},
		{table: "folder_operations", first: "retention_started_at", second: "id", partial: true},
	} {
		if !storetest.Scalar[bool](t, observer, `
SELECT EXISTS (
    SELECT FROM pg_index i
    WHERE i.indrelid = to_regclass($1) AND i.indisvalid AND (i.indpred IS NOT NULL) = $4
      AND pg_get_indexdef(i.indexrelid, 1, true) = $2
      AND ($3::text = '' OR pg_get_indexdef(i.indexrelid, 2, true) = $3)
      AND (NOT $5::boolean OR i.indnkeyatts = 1)
)`, index.table, index.first, index.second, index.partial, index.singleKey) {
			t.Errorf("The planned index on %s beginning with %s is missing.", index.table, index.first)
		}
	}
}

func requireProjectMigrationTables(t *testing.T, observer *pgx.Conn, want bool) {
	t.Helper()
	for _, table := range []string{"repositories", "repository_folders", "folder_operations"} {
		if got := storetest.Scalar[bool](t, observer, "SELECT to_regclass($1) IS NOT NULL", table); got != want {
			t.Errorf("Projects table %s exists = %t, want %t.", table, got, want)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

func seedProjectUpgrade(t *testing.T, s *store.Store) func() {
	t.Helper()
	ctx := t.Context()
	password, err := auth.NewPassword("Projects upgrade fixture password")
	if err != nil {
		t.Fatal("The prior server could not configure its session fixture.")
	}
	token, err := auth.NewSessions(s, password).Issue(ctx)
	if err != nil {
		t.Fatal("The prior server could not issue a session.")
	}
	verifier, ok := auth.TokenVerifier(token)
	if !ok {
		t.Fatal("The prior server issued an invalid session.")
	}

	public, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("The prior server could not create a fixture machine identity.")
	}
	registry := machines.NewRegistry()
	tokenID := projectMigrationID(t)
	var machine machines.Machine
	if err := s.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		var err error
		machine, err = registry.CreateWaiting(ctx, tx, machines.NewMachine{
			PublicKey: public, HostName: "Upgrade machine", TokenID: tokenID,
		})
		return err
	}); err != nil {
		t.Fatal("The prior server could not save the fixture machine.")
	}

	service := machines.NewService(s, nil, registry, nil)
	approve := func() *connect.Request[webv1.ApproveClientRequest] {
		req := connect.NewRequest(&webv1.ApproveClientRequest{Id: machine.ID, Name: "Machine at approval"})
		req.Header().Set(store.RequestKeyHeader, "projects-upgrade-approval")
		return req
	}
	saved, err := service.ApproveClient(ctx, approve())
	if err != nil || saved.Msg.GetClient().GetName() != "Machine at approval" {
		t.Fatal("The prior server could not save the ordinary Machines reply.")
	}
	if _, err := service.RenameClient(ctx, connect.NewRequest(&webv1.RenameClientRequest{
		Id: machine.ID, Name: "Machine after approval",
	})); err != nil {
		t.Fatal("The prior server could not change the machine after saving the approval reply.")
	}

	return func() {
		t.Helper()
		if err := auth.NewSessions(s, password).Check(ctx, verifier); err != nil {
			t.Fatal("The live session from the prior server could not be verified.")
		}
		got, err := service.ApproveClient(ctx, approve())
		if err != nil || !proto.Equal(got.Msg, saved.Msg) {
			t.Fatal("The ordinary Machines replay did not return the original approval reply.")
		}
	}
}

func projectMigrationID(t *testing.T) string {
	t.Helper()
	id, err := store.NewID()
	if err != nil {
		t.Fatal("The fixture could not allocate an identifier.")
	}
	return id
}

func execProjectMigrationSQL(t *testing.T, observer *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := observer.Exec(t.Context(), sql, args...); err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) {
			t.Fatalf("The valid Projects fixture statement failed with SQLSTATE %s.", constraint.Code)
		}
		t.Fatal("The valid Projects fixture statement failed without a PostgreSQL diagnostic code.")
	}
}

func rejectProjectMigrationSQL(t *testing.T, observer *pgx.Conn, want string, sql string, args ...any) {
	t.Helper()
	_, err := observer.Exec(t.Context(), sql, args...)
	if err == nil {
		t.Fatalf("The invalid Projects fixture statement succeeded, want SQLSTATE %s.", want)
	}
	var constraint *pgconn.PgError
	if !errors.As(err, &constraint) {
		t.Fatalf("The invalid Projects fixture statement failed without a PostgreSQL diagnostic code, want %s.", want)
	}
	if constraint.Code != want {
		t.Fatalf("The invalid Projects fixture statement returned SQLSTATE %s, want %s.", constraint.Code, want)
	}
}
