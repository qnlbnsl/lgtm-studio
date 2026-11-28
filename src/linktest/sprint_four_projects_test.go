package linktest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

const folderAction = "look_at_folder"

type projectFixture struct {
	commandFixture
	projects lgtmv1connect.ProjectServiceClient
}

func newProjectFixture(t *testing.T, options servertest.Options, controls relayControls) *projectFixture {
	t.Helper()
	options.Password = fixturePassword
	f := new(projectFixture)
	f.server = servertest.Start(t, options)
	f.page = f.server.Clients(t)
	f.projects = projectService(f.server)
	f.relay = startRelayWithControls(t, f.server.Link(), controls)
	f.opts = newClient(t, new(diagnostics))
	f.probe = installVersionProbe(t, &f.opts)
	f.id = join(t, f.page, f.opts, f.relay.address)
	f.db = openObserver(t, f.server)
	return f
}

func projectService(server *servertest.Server) lgtmv1connect.ProjectServiceClient {
	return lgtmv1connect.NewProjectServiceClient(copyProjectHTTPClient(server.HTTPClient), server.URL)
}

func gitCheckout(t *testing.T) (root, subfolder, alias string) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(parent, "checkout")
	subfolder = filepath.Join(root, "subfolder")
	alias = filepath.Join(parent, "alias")
	if err := os.MkdirAll(subfolder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subfolder, "tracked"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "--initial-branch=main"}, {"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "Fixture"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Could not prepare the disposable Git checkout: %v. %s", err, output)
		}
	}
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	return root, subfolder, alias
}

type addResult struct {
	reply *webv1.AddRepositoryResponse
	err   error
}

func addFolder(ctx context.Context, service lgtmv1connect.ProjectServiceClient, machineID, key, path string) addResult {
	req := connect.NewRequest(&webv1.AddRepositoryRequest{ClientId: machineID, Path: path})
	req.Header().Set("Lgtm-Request-Key", key)
	res, err := service.AddRepository(ctx, req)
	if err != nil {
		return addResult{err: err}
	}
	return addResult{reply: res.Msg}
}

func beginAdd(t *testing.T, service lgtmv1connect.ProjectServiceClient, machineID, key, path string) <-chan addResult {
	t.Helper()
	results := make(chan addResult, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	t.Cleanup(cancel)
	go func() {
		defer cancel()
		results <- addFolder(ctx, service, machineID, key, path)
	}()
	return results
}

func finishAdd(t *testing.T, result <-chan addResult) addResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	case <-time.After(36 * time.Second):
		t.Fatal("The Projects caller did not return within its fixture deadline.")
	}
	return addResult{}
}

type projectOperation struct {
	id, command, state, failure string
	reply                       []byte
}

func readProjectOperation(t *testing.T, db *pgx.Conn, key string) projectOperation {
	t.Helper()
	var op projectOperation
	err := db.QueryRow(t.Context(), `
SELECT id, command_id, state, COALESCE(failure,''), reply FROM folder_operations WHERE request_key=$1`, key).
		Scan(&op.id, &op.command, &op.state, &op.failure, &op.reply)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	return op
}

func waitProjectOperation(t *testing.T, db *pgx.Conn, key string) projectOperation {
	t.Helper()
	var op projectOperation
	waitFor(t, 10*time.Second, "Projects to accept "+key, func() bool {
		op = readProjectOperation(t, db, key)
		return op.id != "" && op.command != ""
	})
	return op
}

func requireOpenProject(t *testing.T, op projectOperation) {
	t.Helper()
	if op.id == "" || op.command == "" || op.state != "open" || op.failure != "" || op.reply != nil {
		t.Fatalf("Projects did not preserve the accepted open operation: %+v.", op)
	}
}

func requireProjectCommand(t *testing.T, op projectOperation, command commandDelivery) {
	t.Helper()
	if command.id != op.command || command.operationID != op.id {
		t.Fatalf("The Link command does not match the accepted operation: %+v.", command)
	}
}

func requireAddReply(t *testing.T, got addResult, machineID, path string) *webv1.AddRepositoryResponse {
	t.Helper()
	if got.err != nil {
		t.Fatalf("Projects Add failed: %v.", got.err)
	}
	reply := got.reply
	folder, repo := reply.GetFolder(), reply.GetRepository()
	if folder.GetId() == "" || repo.GetId() == "" || folder.GetClientId() != machineID ||
		folder.GetResolvedPath() != path || len(repo.GetFolders()) != 1 || !proto.Equal(repo.GetFolders()[0], folder) {
		t.Fatalf("Projects returned unexpected saved identities for the checkout: %v.", reply)
	}
	return reply
}

func requireAddRefusal(t *testing.T, got addResult, code connect.Code, message string) {
	t.Helper()
	var refusal *connect.Error
	if got.reply != nil || !errors.As(got.err, &refusal) || refusal.Code() != code || refusal.Message() != message {
		t.Fatalf("Projects Add returned %v, want %s with %q.", got.err, code, message)
	}
}

func requireProjectSaveCount(t *testing.T, db *pgx.Conn, count int) {
	t.Helper()
	if got := scalar[int](t, db, "SELECT count(*) FROM feed WHERE topic='repositories' AND change='saved'"); got != count {
		t.Fatalf("Projects published %d repository saves, want %d.", got, count)
	}
}

func requireProjectRows(t *testing.T, db *pgx.Conn, count int) {
	t.Helper()
	if !scalar[bool](t, db, `
SELECT (SELECT count(*) FROM repositories)=$1 AND (SELECT count(*) FROM repository_folders)=$1`, count) {
		t.Fatalf("Projects does not hold exactly %d repositories and folders.", count)
	}
}

func requireProjectAck(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], number uint64) {
	t.Helper()
	if ack := receivePairedFrame(t, stream).GetEventAck(); ack == nil || ack.GetThrough() != number {
		t.Fatalf("The server did not acknowledge report %d.", number)
	}
}

func readPairedFolderCommand(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], op projectOperation) *linkv1.Command {
	t.Helper()
	cmd := receivePairedFrame(t, stream).GetCommand()
	requireProjectCommand(t, op, commandDelivery{id: cmd.GetId(), operationID: cmd.GetLookAtFolder().GetOperationId()})
	return cmd
}

func settlePairedApproval(t *testing.T, db *pgx.Conn, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]) {
	t.Helper()
	cmd := receivePairedFrame(t, stream).GetCommand()
	if cmd.GetSetState() == nil {
		t.Fatal("The scripted approved stream did not receive SetState.")
	}
	waitFor(t, 5*time.Second, "the scripted approval dispatch marker", func() bool {
		return scalar[bool](t, db, "SELECT dispatched FROM link_commands WHERE id=$1", cmd.GetId())
	})
	sendPairedFrame(t, stream, pairedResult(cmd.GetId(), cmd.GetAttempt(), 0))
	waitForStoredResult(t, db, commandDelivery{id: cmd.GetId(), attempt: cmd.GetAttempt()}, "done")
}

func projectFolderFacts(number uint64, op projectOperation, path string) *linkv1.Event {
	event := pairedFolder(number, op.id, op.command)
	event.GetFolderSeen().Result = &linkv1.FolderSeen_Facts{Facts: &linkv1.FolderFacts{
		ResolvedPath: path, Name: filepath.Base(path), MainBranch: "main", Clean: true,
	}}
	return event
}
