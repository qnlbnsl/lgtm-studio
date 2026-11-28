package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	projectLagSentence    = "The watch fell behind and must be reopened."
	projectSignInSentence = "Sign in to continue."
	projectNoAnswer       = "The machine did not answer the folder request."
	projectMachineRemoved = "The machine was removed."
	projectInspectRoute   = lgtmv1connect.ProjectServiceInspectFolderProcedure
	projectAddRoute       = lgtmv1connect.ProjectServiceAddRepositoryProcedure
	projectWatchRoute     = lgtmv1connect.ProjectServiceWatchRepositoriesProcedure

	// The statements that the read gates hold. They are details of the built session check and
	// saved-record loader, listed in the result file of the test writer.
	projectSessionCheckSQL = "FROM web_sessions WHERE verifier = $1"
	projectSavedLoadSQL    = "WHERE r.id = $1"
)

type projectApp struct {
	cfg      config.Config
	obs      *pgx.Conn
	server   *Server
	http     *http.Client
	web      lgtmv1connect.ProjectServiceClient
	machines lgtmv1connect.ClientServiceClient
	session  lgtmv1connect.SessionServiceClient
	link     linkv1connect.LinkServiceClient
}

func startProjectApp(t *testing.T) *projectApp {
	t.Helper()
	return startProjectAppWithTrace(t, nil)
}

func startProjectAppWithTrace(t *testing.T, tracer pgx.QueryTracer) *projectApp {
	t.Helper()
	cfg, obs := appConfig(t)
	cfg.Postgres.ConnConfig.Tracer = tracer
	owner := appClient(t)
	owner.Timeout = 35 * time.Second
	f := &projectApp{
		cfg: cfg, obs: obs, http: owner,
		web:      lgtmv1connect.NewProjectServiceClient(owner, cfg.PublicURL),
		machines: lgtmv1connect.NewClientServiceClient(owner, cfg.PublicURL),
		session:  lgtmv1connect.NewSessionServiceClient(owner, cfg.PublicURL),
		link:     linkv1connect.NewLinkServiceClient(&http.Client{Transport: owner.Transport}, cfg.PublicURL),
	}
	f.start(t)
	signIn(t, owner, cfg)
	return f
}

func (f *projectApp) start(t *testing.T) {
	t.Helper()
	server, err := Start(t.Context(), f.cfg, nil)
	if err != nil {
		t.Fatal("The assembled application did not start.")
	}
	f.server = server
	t.Cleanup(func() { _ = server.Close() })
}

func (f *projectApp) guest() lgtmv1connect.ProjectServiceClient {
	return lgtmv1connect.NewProjectServiceClient(&http.Client{Transport: f.http.Transport, Timeout: f.http.Timeout}, f.cfg.PublicURL)
}

func projectRequest[T any](msg *T, key string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if key != "" {
		req.Header().Set(store.RequestKeyHeader, key)
	}
	return req
}

func requireProjectStatus(t *testing.T, err error, code connect.Code, sentence string) {
	t.Helper()
	var status *connect.Error
	if !errors.As(err, &status) || status.Code() != code || status.Message() != sentence {
		t.Fatalf("The Projects call did not return %s with %q. Received code %s.", code, sentence, connect.CodeOf(err))
	}
}

func projectTestID(t *testing.T) string {
	t.Helper()
	id, err := store.NewID()
	if err != nil {
		t.Fatal("The fixture could not generate an identifier.")
	}
	return id
}

func (f *projectApp) join(t *testing.T) *machinetest.Machine {
	t.Helper()
	token, err := f.machines.CreateClientToken(t.Context(), connect.NewRequest(&webv1.CreateClientTokenRequest{}))
	if err != nil {
		t.Fatal("The application did not issue an enrollment token.")
	}
	peer, err := machinetest.Join(t.Context(), f.link, token.Msg.GetToken(), "Projects fixture")
	if err != nil {
		t.Fatal("The signed peer did not enroll.")
	}
	_, err = f.machines.ApproveClient(t.Context(), connect.NewRequest(&webv1.ApproveClientRequest{
		Id: peer.ID, Name: "Projects fixture " + peer.ID[len(peer.ID)-12:],
	}))
	if err != nil {
		t.Fatal("The owner could not approve the fixture machine.")
	}
	return peer
}

type projectLinkStream = connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]

type projectPeer struct {
	stream *projectLinkStream
	mark   uint64
}

func (f *projectApp) online(t *testing.T, machine *machinetest.Machine) *projectPeer {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	stream, welcome, err := machine.Connect(ctx, f.link)
	if err != nil {
		t.Fatal("The peer did not complete the revision 2 handshake.")
	}
	t.Cleanup(func() {
		cancel()
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	})
	p := &projectPeer{stream: stream, mark: welcome.GetLastEventNumber()}
	p.event(t, &linkv1.Event{Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{Os: "Projects fixture", Cores: 2}}})
	p.event(t, &linkv1.Event{Report: &linkv1.Event_HarnessesFound{HarnessesFound: &linkv1.HarnessesFound{}}})
	return p
}

func (p *projectPeer) send(t *testing.T, frame *linkv1.OpenRequest) {
	t.Helper()
	if err := p.stream.Send(frame); err != nil {
		t.Fatal("Sending a revision 2 link frame failed.")
	}
}

func (p *projectPeer) result(t *testing.T, cmd *linkv1.Command, outcome linkv1.CommandOutcome, through uint64) {
	t.Helper()
	p.send(t, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: &linkv1.CommandResult{
		CommandId: cmd.GetId(), Attempt: cmd.GetAttempt(), Outcome: outcome, LastEventNumber: through,
	}}})
}

func (p *projectPeer) receive(t *testing.T) *linkv1.OpenResponse {
	t.Helper()
	for {
		frame, err := p.stream.Receive()
		if err != nil {
			t.Fatalf("The signed link ended before its expected frame. Received code %s.", connect.CodeOf(err))
		}
		if cmd := frame.GetCommand(); cmd.GetSetState() != nil {
			p.result(t, cmd, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, p.mark)
			continue
		}
		return frame
	}
}

func (p *projectPeer) event(t *testing.T, event *linkv1.Event) {
	t.Helper()
	event.Number = p.mark + 1
	p.send(t, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}})
	if p.receive(t).GetEventAck().GetThrough() != event.Number {
		t.Fatal("The event did not return the expected cumulative acknowledgement.")
	}
	p.mark = event.Number
}

func (p *projectPeer) command(t *testing.T, path string) *linkv1.Command {
	t.Helper()
	cmd := p.receive(t).GetCommand()
	if cmd.GetId() == "" || cmd.GetAttempt() != 1 ||
		cmd.GetLookAtFolder().GetOperationId() == "" || cmd.GetLookAtFolder().GetPath() != path {
		t.Fatal("The link did not deliver the expected folder operation on its first attempt.")
	}
	return cmd
}

func (p *projectPeer) folder(t *testing.T, cmd *linkv1.Command, facts *linkv1.FolderFacts, refusal linkv1.FolderRefusal) {
	t.Helper()
	seen := &linkv1.FolderSeen{OperationId: cmd.GetLookAtFolder().GetOperationId(), CommandId: cmd.GetId()}
	if facts != nil {
		seen.Result = &linkv1.FolderSeen_Facts{Facts: facts}
	} else {
		seen.Result = &linkv1.FolderSeen_Refusal{Refusal: refusal}
	}
	p.event(t, &linkv1.Event{Report: &linkv1.Event_FolderSeen{FolderSeen: seen}})
	p.result(t, cmd, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, p.mark)
}

type projectRPC[T any] struct {
	reply *connect.Response[T]
	err   error
}

func beginProjectRPC[T any](t *testing.T, call func() (*connect.Response[T], error)) *storetest.Task[projectRPC[T]] {
	t.Helper()
	return storetest.Start(t, func() projectRPC[T] {
		reply, err := call()
		return projectRPC[T]{reply, err}
	})
}

func projectFacts(path, name, remote string) *linkv1.FolderFacts {
	branch, commit := "work", "fixture-first-commit"
	return &linkv1.FolderFacts{
		ResolvedPath: path, Name: name, MainBranch: "main", CheckedOutBranch: &branch,
		Remote: &remote, FirstCommit: &commit,
	}
}

func (f *projectApp) inspect(t *testing.T, p *projectPeer, machineID, path, key string) (*linkv1.Command, *storetest.Task[projectRPC[webv1.InspectFolderResponse]]) {
	t.Helper()
	req := projectRequest(&webv1.InspectFolderRequest{ClientId: machineID, Path: path}, key)
	task := beginProjectRPC(t, func() (*connect.Response[webv1.InspectFolderResponse], error) {
		return f.web.InspectFolder(t.Context(), req)
	})
	return p.command(t, path), task
}

func (f *projectApp) add(t *testing.T, p *projectPeer, machineID, path, key string, target *string, facts *linkv1.FolderFacts) projectRPC[webv1.AddRepositoryResponse] {
	t.Helper()
	req := projectRequest(&webv1.AddRepositoryRequest{ClientId: machineID, Path: path, RepositoryId: target}, key)
	task := beginProjectRPC(t, func() (*connect.Response[webv1.AddRepositoryResponse], error) {
		return f.web.AddRepository(t.Context(), req)
	})
	p.folder(t, p.command(t, path), facts, 0)
	return task.Wait(t)
}

func requireProjectAdd(t *testing.T, got projectRPC[webv1.AddRepositoryResponse], machine, path string) *webv1.AddRepositoryResponse {
	t.Helper()
	if got.err != nil || got.reply == nil {
		t.Fatalf("AddRepository did not return a saved folder. Received code %s.", connect.CodeOf(got.err))
	}
	reply := got.reply.Msg
	if reply.GetRepository().GetId() == "" || reply.GetFolder().GetId() == "" ||
		reply.GetFolder().GetClientId() != machine || reply.GetFolder().GetResolvedPath() != path ||
		!slices.ContainsFunc(reply.GetRepository().GetFolders(), func(folder *webv1.RepositoryFolder) bool {
			return proto.Equal(folder, reply.GetFolder())
		}) {
		t.Fatal("The Add reply did not identify the resolved folder in its repository.")
	}
	return reply
}

func TestProjectStartupAndFixedRefusals(t *testing.T) {
	t.Run("Migration", func(t *testing.T) {
		f := startProjectApp(t)
		if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM schema_migrations WHERE part='projects' AND version=1)
			AND to_regclass('repositories') IS NOT NULL AND to_regclass('repository_folders') IS NOT NULL
			AND to_regclass('folder_operations') IS NOT NULL`) {
			t.Fatal("Startup became ready without applying the Projects migration.")
		}
	})

	t.Run("OverlappingReport", func(t *testing.T) {
		report := link.PartHooks{ReportApplied: map[link.ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
			link.ReportFolderSeen: func(context.Context, *store.Tx, string, *linkv1.Event) error { return nil },
		}}
		settled := link.PartHooks{CommandSettled: map[link.CommandKind]func(context.Context, *store.Tx, link.Settlement) error{
			link.CommandLookAtFolder: func(context.Context, *store.Tx, link.Settlement) error { return nil },
		}}
		for name, overlap := range map[string]link.PartHooks{"report": report, "settlement": settled} {
			state := link.NewState(nil, machines.NewRegistry(), link.NewHub())
			if err := state.RegisterPart(overlap); err != nil {
				t.Fatal("The fixture could not install its first route.")
			}
			want := state.RegisterPart(overlap)
			if want == nil || !errors.Is(registerProjectHooks(state, new(projects.Service)), want) {
				t.Fatalf("Project hook registration did not propagate the overlapping %s error.", name)
			}
		}
		// A fresh state accepts the Projects routes once, so a second registration overlaps both of them.
		state := link.NewState(nil, machines.NewRegistry(), link.NewHub())
		if err := registerProjectHooks(state, new(projects.Service)); err != nil {
			t.Fatal("Project hook registration failed on a fresh link state.")
		}
		for name, overlap := range map[string]link.PartHooks{"report": report, "settlement": settled} {
			if state.RegisterPart(overlap) == nil {
				t.Fatalf("Project hook registration did not install the folder %s route.", name)
			}
		}
	})

	t.Run("DiscoveryAndGuards", testProjectAppGuards)
	t.Run("MalformedAndOversized", testProjectAppMalformed)
	t.Run("FolderRefusalsAndRestart", testProjectAppFolderRefusals)
	t.Run("RemovalWithoutPendingCommands", testProjectAppRemovalWithoutPending)
}

func testProjectAppGuards(t *testing.T) {
	f := startProjectApp(t)
	public := lgtmv1connect.NewSessionServiceClient(&http.Client{Transport: f.http.Transport}, f.cfg.PublicURL)
	for i, session := range []lgtmv1connect.SessionServiceClient{public, f.session} {
		got, err := session.GetServer(t.Context(), connect.NewRequest(&webv1.GetServerRequest{}))
		if err != nil || got.Msg.GetContract() != 1 || got.Msg.GetSample() || got.Msg.GetSignedIn() != (i == 1) {
			t.Fatal("Discovery did not preserve the real server's revision and session state.")
		}
		count := 0
		for _, name := range got.Msg.GetServices() {
			if name == lgtmv1connect.ProjectServiceName {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("Discovery advertised Projects %d times, want once.", count)
		}
	}

	for _, route := range []string{projectInspectRoute, projectAddRoute, projectWatchRoute} {
		for _, origin := range []string{f.cfg.PublicURL, "https://denied.example.test"} {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, f.cfg.PublicURL+route, nil)
			if err != nil {
				t.Fatal("The fixture could not construct a preflight.")
			}
			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			req.Header.Set("Access-Control-Request-Headers", "content-type, connect-protocol-version")
			if route != projectWatchRoute {
				req.Header.Add("Access-Control-Request-Headers", "lgtm-request-key")
			}
			res, err := f.http.Do(req)
			if err != nil {
				t.Fatal("The assembled server did not answer a preflight.")
			}
			_ = res.Body.Close()
			if origin == f.cfg.PublicURL {
				if res.StatusCode != http.StatusNoContent || res.Header.Get("Access-Control-Allow-Origin") != origin ||
					res.Header.Get("Access-Control-Allow-Credentials") != "true" {
					t.Fatal("A mounted Projects procedure did not allow its listed preflight.")
				}
			} else if res.StatusCode != http.StatusForbidden || res.Header.Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("A Projects preflight allowed an unlisted origin.")
			}
		}
	}

	machineID := projectTestID(t)
	for _, tc := range []struct {
		name, origin string
		client       lgtmv1connect.ProjectServiceClient
		code         connect.Code
		sentence     string
	}{
		{"Missing", "", f.guest(), connect.CodeUnauthenticated, projectSignInSentence},
		{"UnlistedOrigin", "https://denied.example.test", f.web, connect.CodePermissionDenied, "This website origin is not allowed."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inspect := projectRequest(&webv1.InspectFolderRequest{ClientId: machineID, Path: "/inspect"}, "guard-inspect")
			add := projectRequest(&webv1.AddRepositoryRequest{ClientId: machineID, Path: "/add"}, "guard-add")
			if tc.origin != "" {
				inspect.Header().Set("Origin", tc.origin)
				add.Header().Set("Origin", tc.origin)
			}
			_, err := tc.client.InspectFolder(t.Context(), inspect)
			requireProjectStatus(t, err, tc.code, tc.sentence)
			_, err = tc.client.AddRepository(t.Context(), add)
			requireProjectStatus(t, err, tc.code, tc.sentence)
			stream, err := tc.client.WatchRepositories(t.Context(), projectWatchRequest(tc.origin))
			requireProjectWatchEnd(t, stream, err, tc.code, tc.sentence)
		})
	}

	stream := f.watch(t)
	requireProjectSnapshot(t, stream, f.head(t), nil)
	req, err := http.NewRequest(http.MethodPost, f.cfg.PublicURL, nil)
	if err != nil {
		t.Fatal("The fixture could not identify the current session.")
	}
	cookies := f.http.Jar.Cookies(req.URL)
	if len(cookies) == 0 {
		t.Fatal("The old-session fixture did not have a cookie to replay.")
	}
	if _, err := f.session.SignOut(t.Context(), connect.NewRequest(&webv1.SignOutRequest{})); err != nil {
		t.Fatal("The owner could not revoke the session.")
	}
	f.publish(t, []store.ChangeRef{{Topic: "repositories", Key: machineID, Change: store.Removed}})
	requireProjectWatchEnd(t, stream, nil, connect.CodeUnauthenticated, "Sign in again to reopen this watch.")
	stale := projectWatchRequest("")
	inspect := projectRequest(&webv1.InspectFolderRequest{ClientId: machineID, Path: "/inspect"}, "stale-inspect")
	add := projectRequest(&webv1.AddRepositoryRequest{ClientId: machineID, Path: "/add"}, "stale-add")
	for _, cookie := range cookies {
		for _, header := range []http.Header{stale.Header(), inspect.Header(), add.Header()} {
			header.Add("Cookie", cookie.String())
		}
	}
	guest := f.guest()
	old, err := guest.WatchRepositories(t.Context(), stale)
	requireProjectWatchEnd(t, old, err, connect.CodeUnauthenticated, projectSignInSentence)
	_, err = guest.InspectFolder(t.Context(), inspect)
	requireProjectStatus(t, err, connect.CodeUnauthenticated, projectSignInSentence)
	_, err = guest.AddRepository(t.Context(), add)
	requireProjectStatus(t, err, connect.CodeUnauthenticated, projectSignInSentence)

	signIn(t, f.http, f.cfg)
	expiring := f.watch(t)
	requireProjectSnapshot(t, expiring, f.head(t), nil)
	storetest.Exec(t, f.obs, "UPDATE web_sessions SET created_at=clock_timestamp()-interval '31 days'")
	f.publish(t, []store.ChangeRef{{Topic: "repositories", Key: machineID, Change: store.Removed}})
	requireProjectWatchEnd(t, expiring, nil, connect.CodeUnauthenticated, "Sign in again to reopen this watch.")
}

func testProjectAppMalformed(t *testing.T) {
	f := startProjectApp(t)
	f.requireMigration(t)
	machine := f.join(t)
	f.online(t, machine)

	for _, route := range []string{projectInspectRoute, projectAddRoute} {
		malformed := []byte(`{"clientId":"` + machine.ID + `","path":"/body-marker",`)
		f.rawProjectError(t, route, malformed, false, "invalid_argument", "The request body is invalid.")
		// The valid fields would start work if a handler read and dispatched this oversized request.
		oversized := []byte(`{"clientId":"` + machine.ID + `","path":"/valid"}` + strings.Repeat(" ", 32<<10))
		for _, zipped := range []bool{false, true} {
			f.rawProjectError(t, route, oversized, zipped, "resource_exhausted", "")
		}
	}
	f.rawProjectWatchError(t, []byte(`{"marker":"/body-marker",`))
	if storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM folder_operations)
		OR EXISTS (SELECT FROM link_commands WHERE action='look_at_folder')
		OR EXISTS (SELECT FROM receipts WHERE held)`) {
		t.Fatal("A malformed or oversized Projects body persisted work or a held receipt.")
	}
}

func (f *projectApp) rawProjectError(t *testing.T, route string, body []byte, zipped bool, code, sentence string) {
	t.Helper()
	if zipped {
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		_, err := writer.Write(body)
		if err != nil || writer.Close() != nil || compressed.Len() >= 32<<10 {
			t.Fatal("The compressed request fixture did not fit under the wire cap.")
		}
		body = compressed.Bytes()
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, f.cfg.PublicURL+route, bytes.NewReader(body))
	if err != nil {
		t.Fatal("The fixture could not construct the raw Projects call.")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set(store.RequestKeyHeader, "raw-project-request")
	if zipped {
		req.Header.Set("Content-Encoding", "gzip")
	}
	res, err := f.http.Do(req)
	if err != nil {
		t.Fatal("The assembled server did not answer a malformed or oversized call.")
	}
	defer res.Body.Close()
	var reply struct{ Code, Message string }
	if json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&reply) != nil ||
		reply.Code != code || sentence != "" && reply.Message != sentence {
		t.Fatalf("Projects did not return the expected %s body refusal.", code)
	}
}

// rawProjectWatchError sends one framed JSON watch request and requires the fixed malformed-body end of stream.
func (f *projectApp) rawProjectWatchError(t *testing.T, message []byte) {
	t.Helper()
	frame := make([]byte, 5, 5+len(message))
	binary.BigEndian.PutUint32(frame[1:], uint32(len(message)))
	frame = append(frame, message...)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, f.cfg.PublicURL+projectWatchRoute, bytes.NewReader(frame))
	if err != nil {
		t.Fatal("The fixture could not construct the framed watch call.")
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("Connect-Protocol-Version", "1")
	res, err := f.http.Do(req)
	if err != nil {
		t.Fatal("The assembled server did not answer a malformed watch request.")
	}
	defer res.Body.Close()
	flags, size := projectEnvelope(t, res.Body)
	var end struct {
		Error struct{ Code, Message string }
	}
	if res.StatusCode != http.StatusOK || flags != 2 || size > 4096 ||
		json.NewDecoder(io.LimitReader(res.Body, size)).Decode(&end) != nil ||
		end.Error.Code != "invalid_argument" || end.Error.Message != "The request body is invalid." {
		t.Fatal("The first framed watch request did not end with the fixed malformed-body refusal.")
	}
}

func (f *projectApp) requireMigration(t *testing.T) {
	t.Helper()
	if !storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM schema_migrations WHERE part='projects' AND version=1)") {
		t.Fatal("The assembled fixture requires startup to apply the Projects migration.")
	}
}

func testProjectAppFolderRefusals(t *testing.T) {
	f := startProjectApp(t)
	f.requireMigration(t)
	m := f.join(t)
	inspect := func(machine, path, key string) error {
		_, err := f.web.InspectFolder(t.Context(), projectRequest(&webv1.InspectFolderRequest{ClientId: machine, Path: path}, key))
		return err
	}
	requireProjectStatus(t, inspect(m.ID, "/offline", "offline"), connect.CodeUnavailable, "The machine is not connected.")
	requireProjectStatus(t, inspect(projectTestID(t), "/unknown", "unknown-machine"), connect.CodeNotFound, "The machine was not found.")
	requireProjectStatus(t, inspect("invalid", "/inspect", "bad-machine"), connect.CodeInvalidArgument,
		"The machine, repository, or folder path is invalid.")
	requireProjectStatus(t, inspect(m.ID, "relative", "bad-path"), connect.CodeInvalidArgument,
		"The machine, repository, or folder path is invalid.")
	requireProjectStatus(t, inspect(m.ID, "/inspect", ""), connect.CodeInvalidArgument, "The request key is invalid.")

	p := f.online(t, m)
	const typed = "~/checkout  "
	facts := projectFacts("/resolved/checkout", "checkout", "https://git.example.test/team/project")
	cmd, call := f.inspect(t, p, m.ID, typed, "inspect-replay")
	p.folder(t, cmd, facts, 0)
	got := call.Wait(t)
	want := &webv1.FolderFacts{
		ResolvedPath: facts.GetResolvedPath(), Name: facts.GetName(), MainBranch: facts.GetMainBranch(),
		CheckedOutBranch: facts.CheckedOutBranch, Remote: facts.Remote, FirstCommit: facts.FirstCommit,
	}
	if got.err != nil || got.reply == nil || !proto.Equal(got.reply.Msg.GetFacts(), want) ||
		storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM repositories) OR EXISTS (SELECT FROM repository_folders)") {
		t.Fatal("A signed folder report did not return its complete inspection facts without saving membership.")
	}
	requireProjectStatus(t, inspect(m.ID, "~/other", "inspect-replay"), connect.CodeAlreadyExists,
		"That request key was already used for a different request.")

	for _, tc := range []struct {
		refusal  linkv1.FolderRefusal
		sentence string
	}{
		{linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER, "The path is not a folder."},
		{linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY, "The folder is not a Git repository."},
		{linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE, "The folder cannot be read."},
		{linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE, "The folder path cannot be used."},
	} {
		t.Run(tc.refusal.String(), func(t *testing.T) {
			command, pending := f.inspect(t, p, m.ID, "/refused", tc.refusal.String())
			p.folder(t, command, nil, tc.refusal)
			requireProjectStatus(t, pending.Wait(t).err, connect.CodeFailedPrecondition, tc.sentence)
		})
	}
	for _, tc := range []struct {
		outcome  linkv1.CommandOutcome
		sentence string
	}{
		{linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, projectNoAnswer},
		{linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED, projectNoAnswer},
		{linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED, "The machine did not take the folder command."},
		{linkv1.CommandOutcome_COMMAND_OUTCOME_UNSUPPORTED, "The machine did not take the folder command."},
		{linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED, "The outcome of the folder request is unknown."},
	} {
		t.Run(tc.outcome.String(), func(t *testing.T) {
			command, pending := f.inspect(t, p, m.ID, "/without-report", tc.outcome.String())
			p.result(t, command, tc.outcome, p.mark)
			requireProjectStatus(t, pending.Wait(t).err, connect.CodeDeadlineExceeded, tc.sentence)
		})
	}

	added := requireProjectAdd(t, f.add(t, p, m.ID, typed, "save-for-restart", nil, facts), m.ID, facts.GetResolvedPath())
	otherFacts := projectFacts("/other/checkout", "checkout", "https://git.example.test/team/other")
	other := requireProjectAdd(t, f.add(t, p, m.ID, "~/different", "save-different", nil, otherFacts), m.ID, otherFacts.GetResolvedPath())
	unknown := projectTestID(t)
	for i, report := range []*linkv1.FolderFacts{facts, projectFacts("/unsaved/checkout", "checkout", facts.GetRemote())} {
		result := f.add(t, p, m.ID, typed, fmt.Sprintf("missing-target-%d", i), &unknown, report)
		requireProjectStatus(t, result.err, connect.CodeNotFound, "The repository was not found.")
	}
	otherID := other.GetRepository().GetId()
	result := f.add(t, p, m.ID, typed, "owned-elsewhere", &otherID, facts)
	requireProjectStatus(t, result.err, connect.CodeAlreadyExists, "The folder is already saved for another repository.")
	addedID := added.GetRepository().GetId()
	result = f.add(t, p, m.ID, "~/mismatch", "mismatch", &addedID,
		projectFacts("/mismatch/checkout", "checkout", otherFacts.GetRemote()))
	requireProjectStatus(t, result.err, connect.CodeFailedPrecondition, "The folder does not belong to the selected repository.")

	// Let the server's wait end while the dispatched operation remains open.
	sweepReq := projectRequest(&webv1.InspectFolderRequest{ClientId: m.ID, Path: "/sweep"}, "restart-sweep")
	pending := beginProjectRPC(t, func() (*connect.Response[webv1.InspectFolderResponse], error) {
		return f.web.InspectFolder(t.Context(), sweepReq)
	})
	expiring := p.command(t, "/sweep")
	<-pending.Done()
	requireProjectStatus(t, pending.Wait(t).err, connect.CodeDeadlineExceeded, projectNoAnswer)
	if !storetest.Scalar[bool](t, f.obs, "SELECT state='open' FROM folder_operations WHERE id=$1", expiring.GetLookAtFolder().GetOperationId()) {
		t.Fatal("The call's wait deadline closed the durable operation.")
	}
	// One terminal refusal is aged past its retained day below. Only the supervised startup sweep can retire it.
	retiredKey := linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER.String()
	keptKey := linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY.String()
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 2 FROM folder_operations o
		WHERE o.request_key IN ($1,$2) AND o.state='failed'
		AND EXISTS (SELECT FROM receipts r WHERE r.request_key=o.request_key AND r.held)`, retiredKey, keptKey) {
		t.Fatal("The retirement fixture did not find two terminal operations with their held receipts.")
	}

	// Move the command's expiry back without waiting for the periodic sweep. Startup settles it.
	_ = f.server.Close()
	storetest.Exec(t, f.obs, "UPDATE link_commands SET expires_at = clock_timestamp() - interval '2 days' WHERE id=$1", expiring.GetId())
	storetest.Exec(t, f.obs, "UPDATE folder_operations SET retention_started_at = clock_timestamp() - interval '2 days' WHERE request_key=$1", retiredKey)
	f.start(t)
	storetest.Poll(t, "the startup retirement sweep removing the aged operation and its held receipt", func() bool {
		return storetest.Scalar[bool](t, f.obs, `SELECT NOT EXISTS (SELECT FROM folder_operations WHERE request_key=$1)
			AND NOT EXISTS (SELECT FROM receipts WHERE request_key=$1)`, retiredKey)
	})
	if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM folder_operations o
		WHERE o.request_key=$1 AND o.state='failed' AND o.retention_started_at IS NOT NULL
		AND EXISTS (SELECT FROM receipts r WHERE r.request_key=o.request_key AND r.held))`, keptKey) {
		t.Fatal("The startup sweep did not stamp and keep a terminal operation within its retained day.")
	}
	// The retired key is free again, so the same request is a fresh admission while the machine is offline.
	requireProjectStatus(t, inspect(m.ID, "/refused", retiredKey), connect.CodeUnavailable, "The machine is not connected.")
	requireProjectStatus(t, inspect(m.ID, "/refused", keptKey), connect.CodeFailedPrecondition, "The folder is not a Git repository.")

	repos := []*webv1.Repository{added.GetRepository(), other.GetRepository()}
	requireProjectSnapshot(t, f.watch(t), f.head(t), repos)

	sweepReply := inspect(m.ID, "/sweep", "restart-sweep")
	requireProjectStatus(t, sweepReply, connect.CodeDeadlineExceeded, "The outcome of the folder request is unknown.")
	if !storetest.Scalar[bool](t, f.obs, "SELECT outcome='timed_out' FROM link_commands WHERE id=$1", expiring.GetId()) {
		t.Fatal("The startup sweep did not record the timed-out command.")
	}
	replay, err := f.web.InspectFolder(t.Context(), projectRequest(&webv1.InspectFolderRequest{ClientId: m.ID, Path: typed}, "inspect-replay"))
	if err != nil || replay == nil || !proto.Equal(replay.Msg, got.reply.Msg) {
		t.Fatal("Restart did not preserve the keyed inspection result and its exact typed path.")
	}
}

func testProjectAppRemovalWithoutPending(t *testing.T) {
	f := startProjectApp(t)
	f.requireMigration(t)
	m := f.join(t)
	p := f.online(t, m)
	facts := projectFacts("/resolved/remove", "remove", "https://git.example.test/team/remove")
	const path = "~/remove"
	added := requireProjectAdd(t, f.add(t, p, m.ID, path, "removed-replay", nil, facts), m.ID, facts.GetResolvedPath())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req := projectRequest(&webv1.InspectFolderRequest{ClientId: m.ID, Path: path}, "remove-open")
	call := beginProjectRPC(t, func() (*connect.Response[webv1.InspectFolderResponse], error) {
		return f.web.InspectFolder(ctx, req)
	})
	inspectCommand := p.command(t, path)
	cancel()
	_ = call.Wait(t)

	addCtx, cancelAdd := context.WithCancel(t.Context())
	defer cancelAdd()
	addRequest := func() *connect.Request[webv1.AddRepositoryRequest] {
		return projectRequest(&webv1.AddRepositoryRequest{ClientId: m.ID, Path: path}, "remove-open-add")
	}
	addCall := beginProjectRPC(t, func() (*connect.Response[webv1.AddRepositoryResponse], error) {
		return f.web.AddRepository(addCtx, addRequest())
	})
	// Link can queue this Add behind the dispatched Inspect. Observe acceptance in the ledger.
	var addCommandID string
	storetest.Poll(t, "the open Add and its queued command", func() bool {
		addCommandID = storetest.Scalar[string](t, f.obs, `SELECT COALESCE((
			SELECT command_id FROM folder_operations WHERE machine_id=$1 AND request_key='remove-open-add' AND state='open'
		), '')`, m.ID)
		return addCommandID != ""
	})
	cancelAdd()
	_ = addCall.Wait(t)

	// Retain both open operations after taking their pending commands out of the fixture.
	_ = f.server.Close()
	storetest.Exec(t, f.obs, "DELETE FROM link_commands WHERE id IN ($1,$2)", inspectCommand.GetId(), addCommandID)
	if storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM link_commands WHERE machine_id=$1 AND outcome IS NULL)", m.ID) ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM folder_operations WHERE machine_id=$1 AND state='open'", m.ID) != 2 {
		t.Fatal("The removal fixture did not retain open Add and Inspect operations without pending commands.")
	}
	f.start(t)
	f.remove(t, m.ID, "remove-without-command")
	_, err := f.web.InspectFolder(t.Context(), projectRequest(&webv1.InspectFolderRequest{ClientId: m.ID, Path: path}, "remove-open"))
	requireProjectStatus(t, err, connect.CodeNotFound, projectMachineRemoved)
	_, err = f.web.AddRepository(t.Context(), addRequest())
	requireProjectStatus(t, err, connect.CodeNotFound, projectMachineRemoved)
	if storetest.Scalar[int64](t, f.obs, `SELECT count(*) FROM folder_operations
		WHERE command_id IN ($1,$2) AND state='failed' AND failure='machine_removed'`,
		inspectCommand.GetId(), addCommandID) != 2 {
		t.Fatal("Removal did not close both retained operations without pending Link work.")
	}
	replay, err := f.web.AddRepository(t.Context(), projectRequest(&webv1.AddRepositoryRequest{ClientId: m.ID, Path: path}, "removed-replay"))
	if err != nil || replay == nil || !proto.Equal(replay.Msg, added) {
		t.Fatal("Removal did not preserve the earlier terminal Add replay.")
	}
	requireProjectSnapshot(t, f.watch(t), f.head(t), nil)
}

type projectWatchStream = connect.ServerStreamForClient[webv1.WatchRepositoriesResponse]

func projectWatchRequest(origin string) *connect.Request[webv1.WatchRepositoriesRequest] {
	req := connect.NewRequest(&webv1.WatchRepositoriesRequest{})
	if origin != "" {
		req.Header().Set("Origin", origin)
	}
	return req
}

func (f *projectApp) watch(t *testing.T) *projectWatchStream {
	t.Helper()
	stream, err := f.web.WatchRepositories(t.Context(), projectWatchRequest(f.cfg.PublicURL))
	if err != nil {
		t.Fatalf("The mounted watch did not open. Received code %s.", connect.CodeOf(err))
	}
	t.Cleanup(func() { _ = stream.Close() })
	return stream
}

func requireProjectWatchEnd(t *testing.T, stream *projectWatchStream, err error, code connect.Code, sentence string) {
	t.Helper()
	if err == nil {
		defer stream.Close()
		if stream.Receive() {
			t.Fatal("The Projects watch sent a data frame in place of the expected refusal.")
		}
		err = stream.Err()
	}
	requireProjectStatus(t, err, code, sentence)
}

func nextProjectFrame(t *testing.T, stream *projectWatchStream) *webv1.WatchRepositoriesResponse {
	t.Helper()
	if !stream.Receive() {
		t.Fatalf("The repository watch ended before the expected frame. Received code %s.", connect.CodeOf(stream.Err()))
	}
	return stream.Msg()
}

func checkProjectSnapshot(t *testing.T, frame *webv1.WatchRepositoriesResponse, head uint64, repos []*webv1.Repository) {
	t.Helper()
	all := frame.GetAll()
	if all == nil || frame.GetPosition() == nil || frame.GetPosition().GetValue() != head ||
		len(all.GetRepositories()) != len(repos) {
		t.Fatalf("The repository snapshot was incomplete or at the wrong position. Want %d repositories at %d.", len(repos), head)
	}
	expected := make(map[string]*webv1.Repository, len(repos))
	for _, repo := range repos {
		expected[repo.GetId()] = repo
	}
	for _, repo := range all.GetRepositories() {
		want := expected[repo.GetId()]
		if want == nil || !proto.Equal(repo, want) {
			t.Fatal("The repository snapshot did not contain the expected complete membership and metadata.")
		}
		delete(expected, repo.GetId())
	}
}

func requireProjectSnapshot(t *testing.T, stream *projectWatchStream, head uint64, repos []*webv1.Repository) {
	t.Helper()
	checkProjectSnapshot(t, nextProjectFrame(t, stream), head, repos)
}

func (f *projectApp) head(t *testing.T) uint64 {
	t.Helper()
	return uint64(storetest.Scalar[int64](t, f.obs, "SELECT COALESCE(max(position), 0) FROM feed"))
}

func (f *projectApp) repositoryPosition(t *testing.T, id, change string) uint64 {
	t.Helper()
	return uint64(storetest.Scalar[int64](t, f.obs, `SELECT COALESCE(max(position), 0) FROM feed
		WHERE topic='repositories' AND scope='' AND key=$1 AND change=$2`, id, change))
}

func (f *projectApp) publish(t *testing.T, refs []store.ChangeRef) {
	t.Helper()
	db, err := store.Open(t.Context(), f.cfg.Postgres, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The fixture could not open an independent feed writer.")
	}
	defer db.Close()
	for offset := 0; offset < len(refs); offset += store.MaxWriteChanges {
		batch := refs[offset:min(offset+store.MaxWriteChanges, len(refs))]
		err := db.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			for _, ref := range batch {
				if err := tx.Record(ref); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal("The fixture could not commit its feed references.")
		}
	}
}

func (f *projectApp) removeRequest(t *testing.T, machineID, key string) error {
	t.Helper()
	_, err := f.machines.RemoveClient(t.Context(), projectRequest(&webv1.RemoveClientRequest{Id: machineID}, key))
	return err
}

func (f *projectApp) remove(t *testing.T, machineID, key string) {
	t.Helper()
	if err := f.removeRequest(t, machineID, key); err != nil {
		t.Fatalf("The owner could not remove the fixture machine. Received code %s.", connect.CodeOf(err))
	}
}

func TestProjectStartupRepositoryWatch(t *testing.T) {
	t.Run("AttachmentAndRemoval", testProjectAppAttachmentAndRemoval)
	t.Run("QueuedSaveAfterRemoval", testProjectAppQueuedSave)
	t.Run("LargeRemoval", func(t *testing.T) { testProjectAppLargeRemoval(t, false) })
	t.Run("LargeFlaggedRemoval", func(t *testing.T) { testProjectAppLargeRemoval(t, true) })
	t.Run("LagAtFrameBoundary", testProjectAppFrameBoundaryLag)
	t.Run("LagDuringSocketWrite", testProjectAppSocketLag)
}

func testProjectAppAttachmentAndRemoval(t *testing.T) {
	f := startProjectApp(t)
	watch := f.watch(t)
	requireProjectSnapshot(t, watch, f.head(t), nil)
	first, second := f.join(t), f.join(t)
	one, two := f.online(t, first), f.online(t, second)

	const origin = "https://git.example.test/team/project"
	facts := projectFacts("/resolved/one", "one", origin)
	initial := requireProjectAdd(t, f.add(t, one, first.ID, "~/alias", "first-folder", nil, facts), first.ID, facts.GetResolvedPath())
	id := initial.GetRepository().GetId()
	firstSave := nextProjectFrame(t, watch)
	if firstSave.GetPosition().GetValue() != f.repositoryPosition(t, id, "saved") ||
		!proto.Equal(firstSave.GetSaved(), initial.GetRepository()) {
		t.Fatal("The first Add reply and its positioned saved frame disagreed.")
	}
	facts = projectFacts("/resolved/two", "two", origin)
	attached := requireProjectAdd(t, f.add(t, two, second.ID, "~/alias", "second-folder", &id, facts), second.ID, facts.GetResolvedPath())
	saved := nextProjectFrame(t, watch)
	if saved.GetPosition().GetValue() <= firstSave.GetPosition().GetValue() ||
		saved.GetPosition().GetValue() != f.repositoryPosition(t, id, "saved") || !proto.Equal(saved.GetSaved(), attached.GetRepository()) ||
		len(saved.GetSaved().GetFolders()) != 2 {
		t.Fatal("The attachment reply and watch did not identify the same two saved folders.")
	}

	f.remove(t, first.ID, "remove-first-folder")
	surviving := nextProjectFrame(t, watch)
	want := proto.CloneOf(attached.GetRepository())
	want.Folders = []*webv1.RepositoryFolder{attached.GetFolder()}
	if surviving.GetPosition().GetValue() <= saved.GetPosition().GetValue() ||
		surviving.GetPosition().GetValue() != f.repositoryPosition(t, id, "saved") || !proto.Equal(surviving.GetSaved(), want) {
		t.Fatal("The ordinary removal did not publish the surviving repository with its other folder.")
	}

	f.remove(t, second.ID, "remove-last-folder")
	removed := nextProjectFrame(t, watch)
	if removed.GetPosition().GetValue() <= surviving.GetPosition().GetValue() ||
		removed.GetPosition().GetValue() != f.repositoryPosition(t, id, "removed") || removed.GetRemovedId() != id {
		t.Fatal("The final removal did not publish the same repository identifier as removed.")
	}
	requireProjectSnapshot(t, f.watch(t), f.head(t), nil)
}

// projectWatchReadGate holds the first server statement that contains match after each arm.
type projectWatchReadGate struct {
	match string
	mu    sync.Mutex
	hold  *projectReadHold
}

type projectReadHold struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *projectReadHold) open() { h.once.Do(func() { close(h.release) }) }

func (g *projectWatchReadGate) arm(t *testing.T) *projectReadHold {
	t.Helper()
	hold := &projectReadHold{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(hold.open)
	g.mu.Lock()
	g.hold = hold
	g.mu.Unlock()
	return hold
}

func (g *projectWatchReadGate) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !strings.Contains(data.SQL, g.match) {
		return ctx
	}
	g.mu.Lock()
	hold := g.hold
	g.hold = nil
	g.mu.Unlock()
	if hold != nil {
		close(hold.entered)
		select {
		case <-hold.release:
		case <-ctx.Done():
		}
	}
	return ctx
}

func (*projectWatchReadGate) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func testProjectAppQueuedSave(t *testing.T) {
	// The session check is the first statement of the watch's read, so holding it delays the read's snapshot.
	gate := &projectWatchReadGate{match: projectSessionCheckSQL}
	f := startProjectAppWithTrace(t, gate)
	machine := f.join(t)
	peer := f.online(t, machine)
	facts := projectFacts("/resolved/race", "race", "https://git.example.test/team/race")
	const typed, key = "~/race", "queued-save-replay"
	request := func() *connect.Request[webv1.AddRepositoryRequest] {
		return projectRequest(&webv1.AddRepositoryRequest{ClientId: machine.ID, Path: typed}, key)
	}
	watch := f.watch(t)
	requireProjectSnapshot(t, watch, f.head(t), nil)
	added := requireProjectAdd(t, f.add(t, peer, machine.ID, typed, key, nil, facts), machine.ID, facts.GetResolvedPath())
	id := added.GetRepository().GetId()
	if !proto.Equal(nextProjectFrame(t, watch).GetSaved(), added.GetRepository()) {
		t.Fatal("The watch did not see the initial repository save before the race.")
	}

	// Hold the watch's first read of the queued save before its transaction takes a snapshot.
	hold := gate.arm(t)
	f.publish(t, []store.ChangeRef{{Topic: "repositories", Key: id, Change: store.Saved}})
	queued := f.head(t)
	storetest.Receive(t, "the queued repository change entering its first read", hold.entered)
	f.remove(t, machine.ID, "remove-before-watch-read")
	hold.open()
	change := nextProjectFrame(t, watch)
	if change.GetPosition().GetValue() != queued || change.GetRemovedId() != id {
		t.Fatal("The queued save restored the repository after removal had committed.")
	}
	removed := nextProjectFrame(t, watch)
	if removed.GetPosition().GetValue() != f.repositoryPosition(t, id, "removed") || removed.GetRemovedId() != id {
		t.Fatal("The watch did not deliver the committed orphan removal.")
	}
	replay, err := f.web.AddRepository(t.Context(), request())
	if err != nil || replay == nil || !proto.Equal(replay.Msg, added) {
		t.Fatal("The earlier Add reply did not remain replayable across removal.")
	}
	requireProjectSnapshot(t, f.watch(t), f.head(t), nil)
}

func seedProjectRemoval(t *testing.T, f *projectApp, count int, removed, remaining string) ([]*webv1.Repository, *webv1.Repository, *webv1.Repository) {
	t.Helper()
	all := make([]*webv1.Repository, count)
	tx, err := f.obs.Begin(t.Context())
	if err != nil {
		t.Fatal("The fixture could not start the repository seed transaction.")
	}
	defer tx.Rollback(context.Background())
	for i := range all {
		id := projectTestID(t)
		folder := &webv1.RepositoryFolder{Id: projectTestID(t), ClientId: removed, ResolvedPath: fmt.Sprintf("/removed/repository-%d", i)}
		repo := &webv1.Repository{Id: id, Name: fmt.Sprintf("Repository %d", i), MainBranch: "main", Folders: []*webv1.RepositoryFolder{folder}}
		storetest.Exec(t, tx, "INSERT INTO repositories(id, name, main_branch, remote_omitted) VALUES ($1,$2,$3,false)",
			repo.GetId(), repo.GetName(), repo.GetMainBranch())
		storetest.Exec(t, tx, "INSERT INTO repository_folders(id, repository_id, machine_id, resolved_path) VALUES ($1,$2,$3,$4)",
			folder.GetId(), id, removed, folder.GetResolvedPath())
		all[i] = repo
	}
	shared := all[0]
	other := &webv1.RepositoryFolder{Id: projectTestID(t), ClientId: remaining, ResolvedPath: "/survivor/checkout"}
	storetest.Exec(t, tx, "INSERT INTO repository_folders(id, repository_id, machine_id, resolved_path) VALUES ($1,$2,$3,$4)",
		other.GetId(), shared.GetId(), remaining, other.GetResolvedPath())
	shared.Folders = append(shared.Folders, other)
	slices.SortFunc(shared.Folders, func(a, b *webv1.RepositoryFolder) int { return strings.Compare(a.GetId(), b.GetId()) })
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal("The fixture could not commit the repository membership.")
	}
	survivor := proto.CloneOf(shared)
	survivor.Folders = []*webv1.RepositoryFolder{other}
	return all, shared, survivor
}

func testProjectAppLargeRemoval(t *testing.T, flagged bool) {
	f := startProjectApp(t)
	f.requireMigration(t)
	machine, remaining := f.join(t), f.join(t)
	count := 256
	var command *linkv1.Command
	if flagged {
		count = 257
		peer := f.online(t, machine)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		req := projectRequest(&webv1.InspectFolderRequest{ClientId: machine.ID, Path: "/staged"}, "large-staged")
		call := beginProjectRPC(t, func() (*connect.Response[webv1.InspectFolderResponse], error) {
			return f.web.InspectFolder(ctx, req)
		})
		command = peer.command(t, "/staged")
		peer.result(t, command, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE, peer.mark+1)
		storetest.Poll(t, "the dispatched folder result staged above the durable event mark", func() bool {
			return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM link_commands c JOIN link_machines m ON m.machine_id=c.machine_id
				WHERE c.id=$1 AND c.dispatched AND c.outcome IS NULL AND c.staged_outcome='done'
				AND c.staged_event_number=m.event_mark+1 AND m.event_mark=$2)`, command.GetId(), int64(peer.mark))
		})
		cancel()
		_ = call.Wait(t)

		gap := *machine
		gap.RunID, gap.FirstEventNumber = projectTestID(t), peer.mark-1
		_, _, err := gap.Connect(t.Context(), f.link)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("The signed unseen run behind the mark did not refuse admission. Received code %s.", connect.CodeOf(err))
		}
		if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM link_machines
			WHERE machine_id=$1 AND recovery_set_generation IS NOT NULL AND event_mark=$2)
			AND NOT EXISTS (SELECT FROM link_runs WHERE machine_id=$1 AND run_id=$3)`, machine.ID, int64(peer.mark), gap.RunID) {
			t.Fatal("The refused run did not persist the warning while preserving the durable event mark.")
		}
		if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM folder_operations o JOIN link_commands c ON c.id=o.command_id
			WHERE o.id=$1 AND o.state='open' AND c.id=$2 AND c.staged_outcome='done' AND c.outcome IS NULL)`,
			command.GetLookAtFolder().GetOperationId(), command.GetId()) {
			t.Fatal("The unseen run changed the staged result or closed its Projects operation.")
		}
	}

	all, shared, survivor := seedProjectRemoval(t, f, count, machine.ID, remaining.ID)
	watch := f.watch(t)
	head := f.head(t)
	requireProjectSnapshot(t, watch, head, all)
	before := projectRemovalState(t, f)
	storetest.Exec(t, f.obs, `CREATE SEQUENCE project_removal_attempts;
		CREATE FUNCTION project_refuse_orphan_delete() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM nextval('project_removal_attempts');
			RAISE EXCEPTION 'The fixture refused an orphan deletion.';
		END $$;
		CREATE TRIGGER project_refuse_orphan_delete BEFORE DELETE ON repositories
		FOR EACH ROW EXECUTE FUNCTION project_refuse_orphan_delete()`)
	if err := f.removeRequest(t, machine.ID, "large-removal"); err == nil {
		t.Fatal("RemoveClient succeeded while the orphan deletion was faulted.")
	}
	if !storetest.Scalar[bool](t, f.obs, "SELECT is_called FROM project_removal_attempts") {
		t.Fatal("RemoveClient failed before reaching the intended orphan deletion.")
	}
	if after := projectRemovalState(t, f); after != before || f.head(t) != head {
		t.Fatal("The failed removal changed the machine, warning, staged command, operation, membership, or feed.")
	}

	storetest.Exec(t, f.obs, "DROP TRIGGER project_refuse_orphan_delete ON repositories")
	if flagged {
		// Link deletes its machine row, which holds the warning, after the Projects removal callback has run.
		storetest.Exec(t, f.obs, `CREATE SEQUENCE project_removal_link_seen;
			CREATE FUNCTION project_refuse_link_delete() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN
				PERFORM nextval('project_removal_link_seen');
				RAISE EXCEPTION 'The fixture refused the link machine deletion.';
			END $$;
			CREATE TRIGGER project_refuse_link_delete BEFORE DELETE ON link_machines
			FOR EACH ROW EXECUTE FUNCTION project_refuse_link_delete()`)
		if err := f.removeRequest(t, machine.ID, "large-removal"); err == nil {
			t.Fatal("RemoveClient succeeded while its link machine deletion was faulted.")
		}
		if !storetest.Scalar[bool](t, f.obs, "SELECT is_called FROM project_removal_link_seen") {
			t.Fatal("RemoveClient failed before its intended link machine deletion fault.")
		}
		if after := projectRemovalState(t, f); after != before || f.head(t) != head {
			t.Fatal("The link deletion fault after the Projects callback left a refresh or another partial removal effect.")
		}
		storetest.Exec(t, f.obs, "DROP TRIGGER project_refuse_link_delete ON link_machines")

		// Store flushes the feed only after the removal callback has recorded its changes.
		storetest.Exec(t, f.obs, `CREATE SEQUENCE project_removal_flush_seen;
			CREATE FUNCTION project_refuse_removal_feed() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN
				PERFORM nextval('project_removal_flush_seen');
				RAISE EXCEPTION 'The fixture refused the removal feed flush.';
			END $$;
			CREATE TRIGGER project_refuse_removal_feed BEFORE INSERT ON feed FOR EACH ROW
			WHEN (NEW.topic='machines' AND NEW.change='removed')
			EXECUTE FUNCTION project_refuse_removal_feed()`)
		if err := f.removeRequest(t, machine.ID, "large-removal"); err == nil {
			t.Fatal("RemoveClient succeeded while its feed flush was faulted.")
		}
		if after := projectRemovalState(t, f); after != before || f.head(t) != head {
			t.Fatal("The feed flush fault left a refresh or another partial removal effect.")
		}
		if !storetest.Scalar[bool](t, f.obs, "SELECT is_called FROM project_removal_flush_seen") {
			t.Fatal("RemoveClient failed before its intended feed flush fault.")
		}
		storetest.Exec(t, f.obs, "DROP TRIGGER project_refuse_removal_feed ON feed")
	}

	// The next committed save proves the original watch stayed usable through both rollback paths.
	f.publish(t, []store.ChangeRef{{Topic: "repositories", Key: shared.GetId(), Change: store.Saved}})
	control := nextProjectFrame(t, watch)
	head = f.head(t)
	if control.GetPosition().GetValue() != head || !proto.Equal(control.GetSaved(), shared) {
		t.Fatal("The open watch did not deliver the intact shared repository after the rollback.")
	}
	f.remove(t, machine.ID, "large-removal")

	if rows := storetest.Rows[string](t, f.obs, `SELECT scope || '/' || key || '/' || change
		FROM feed WHERE topic='repositories' AND position>$1 ORDER BY position`, int64(head)); !slices.Equal(rows, []string{"/refresh/saved"}) {
		t.Fatalf("The committed large removal did not publish exactly one reserved refresh. Received %d repository references.", len(rows))
	}
	if storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM machines WHERE id=$1)
		OR EXISTS (SELECT FROM link_machines WHERE machine_id=$1)
		OR EXISTS (SELECT FROM link_runs WHERE machine_id=$1)
		OR EXISTS (SELECT FROM repository_folders WHERE machine_id=$1)`, machine.ID) {
		t.Fatal("Committed removal left the removed machine, recovery warning, run or folder membership.")
	}
	if flagged {
		if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
			SELECT FROM link_commands c JOIN folder_operations o ON o.command_id=c.id
			WHERE o.id=$1 AND c.id=$2 AND c.outcome='removed' AND c.finished_at IS NOT NULL
			AND c.staged_outcome IS NULL AND c.staged_event_number IS NULL AND c.payload IS NULL
			AND o.state='failed' AND o.failure='machine_removed' AND o.reply IS NULL)`,
			command.GetLookAtFolder().GetOperationId(), command.GetId()) {
			t.Fatal("Committed removal did not settle the command, clear its stage, and refuse the open operation.")
		}
		_, err := f.web.InspectFolder(t.Context(), projectRequest(&webv1.InspectFolderRequest{ClientId: machine.ID, Path: "/staged"}, "large-staged"))
		requireProjectStatus(t, err, connect.CodeNotFound, projectMachineRemoved)
	}
	requireProjectWatchEnd(t, watch, nil, connect.CodeAborted, projectLagSentence)
	requireProjectSnapshot(t, f.watch(t), f.head(t), []*webv1.Repository{survivor})
}

func projectRemovalState(t *testing.T, f *projectApp) string {
	t.Helper()
	return storetest.Scalar[string](t, f.obs, `SELECT jsonb_build_array(
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM machines r),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY machine_id) FROM link_machines r),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY machine_id) FROM link_connections r),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY machine_id, run_id) FROM link_runs r),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM link_commands r),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM folder_operations r),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM repositories r),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM repository_folders r),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY position) FROM feed r)
	)::text`)
}

// fillProjectLag commits other-topic references until total references follow the watch's cursor,
// counting one held reference. A fresh Machines watch witnesses that the shared follower has read the last one.
func (f *projectApp) fillProjectLag(t *testing.T, total int) {
	t.Helper()
	fillers := make([]store.ChangeRef, total-2)
	for i := range fillers {
		fillers[i] = store.ChangeRef{Topic: "fixture-other", Key: projectTestID(t), Change: store.Saved}
	}
	f.publish(t, fillers)
	// The witness opens after the fillers, so they do not count toward its own lag.
	probe, err := f.machines.WatchClients(t.Context(), connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err != nil {
		t.Fatal("The follower witness could not open.")
	}
	defer probe.Close()
	if !probe.Receive() {
		t.Fatal("The follower witness did not receive a snapshot.")
	}
	last := projectTestID(t)
	f.publish(t, []store.ChangeRef{{Topic: "machines", Key: last, Change: store.Removed}})
	if !probe.Receive() || probe.Msg().GetRemovedId() != last {
		t.Fatal("The shared follower did not read the last lag reference.")
	}
}

func testProjectAppFrameBoundaryLag(t *testing.T) {
	// The saved-record load runs after the session check, so a held load leaves the watch between frames.
	gate := &projectWatchReadGate{match: projectSavedLoadSQL}
	f := startProjectAppWithTrace(t, gate)
	f.requireMigration(t)
	all, _, _ := seedProjectRemoval(t, f, 1, projectTestID(t), projectTestID(t))
	repo := all[0]
	watch := f.watch(t)
	requireProjectSnapshot(t, watch, f.head(t), all)

	// The held reference and 2,046 later ones stay below the lag limit.
	hold := gate.arm(t)
	f.publish(t, []store.ChangeRef{{Topic: "repositories", Key: repo.GetId(), Change: store.Saved}})
	saved := f.head(t)
	storetest.Receive(t, "the saved reference entering its record load", hold.entered)
	f.fillProjectLag(t, 2047)
	hold.open()
	if frame := nextProjectFrame(t, watch); frame.GetPosition().GetValue() != saved || !proto.Equal(frame.GetSaved(), repo) {
		t.Fatal("The watch did not send the complete held record at 2,047 outstanding references.")
	}
	// A positioned sentinel shows that the watch has acknowledged every earlier reference.
	sentinel := projectTestID(t)
	f.publish(t, []store.ChangeRef{{Topic: "repositories", Key: sentinel, Change: store.Removed}})
	if frame := nextProjectFrame(t, watch); frame.GetPosition().GetValue() != f.head(t) || frame.GetRemovedId() != sentinel {
		t.Fatal("The watch did not deliver the sentinel after draining the earlier references.")
	}

	// Reference 2,048 arrives while the watch holds a record between frames.
	hold = gate.arm(t)
	f.publish(t, []store.ChangeRef{{Topic: "repositories", Key: repo.GetId(), Change: store.Saved}})
	storetest.Receive(t, "the second saved reference entering its record load", hold.entered)
	f.fillProjectLag(t, 2048)
	hold.open()
	requireProjectWatchEnd(t, watch, nil, connect.CodeAborted, projectLagSentence)
	requireProjectSnapshot(t, f.watch(t), f.head(t), all)
}

func openProjectSocket(t *testing.T, f *projectApp) (*net.TCPConn, io.ReadCloser) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp4", f.cfg.ListenAddr)
	if err != nil {
		t.Fatal("The real HTTP/1.1 watch socket could not connect.")
	}
	tcp := conn.(*net.TCPConn)
	t.Cleanup(func() { _ = tcp.Close() })
	if tcp.SetReadBuffer(4<<10) != nil || tcp.SetDeadline(time.Now().Add(15*time.Second)) != nil {
		t.Fatal("The fixture could not bound the socket's receive buffer and deadline.")
	}
	req, err := http.NewRequest(http.MethodPost, f.cfg.PublicURL+projectWatchRoute, bytes.NewReader(make([]byte, 5)))
	if err != nil {
		t.Fatal("The fixture could not construct the framed watch request.")
	}
	req.Header.Set("Content-Type", "application/connect+proto")
	req.Header.Set("Connect-Protocol-Version", "1")
	for _, cookie := range f.http.Jar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}
	if err := req.Write(tcp); err != nil {
		t.Fatal("The framed watch request could not be written to the real socket.")
	}
	res, err := http.ReadResponse(bufio.NewReader(tcp), req)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatal("The real response writer did not open the repository stream.")
	}
	return tcp, res.Body
}

func projectEnvelope(t *testing.T, body io.Reader) (byte, int64) {
	t.Helper()
	var header [5]byte
	if _, err := io.ReadFull(body, header[:]); err != nil {
		t.Fatal("The HTTP/1.1 stream ended before its next envelope header.")
	}
	return header[0], int64(binary.BigEndian.Uint32(header[1:]))
}

func testProjectAppSocketLag(t *testing.T) {
	f := startProjectApp(t)
	tcp, body := openProjectSocket(t, f)
	t.Cleanup(func() {
		_ = tcp.Close()
		_ = body.Close()
	})
	flags, size := projectEnvelope(t, body)
	if flags != 0 || size > 128 {
		t.Fatal("The real watch did not start with one small uncompressed snapshot frame.")
	}
	var frame webv1.WatchRepositoriesResponse
	snapshot, err := io.ReadAll(io.LimitReader(body, size))
	if err != nil || int64(len(snapshot)) != size || proto.Unmarshal(snapshot, &frame) != nil {
		t.Fatal("The initial watch envelope did not contain a complete snapshot.")
	}
	checkProjectSnapshot(t, &frame, f.head(t), nil)

	machine := f.join(t)
	large := &webv1.Repository{Id: projectTestID(t), Name: "Large repository", MainBranch: "main"}
	storetest.Exec(t, f.obs, "INSERT INTO repositories(id,name,main_branch,remote_omitted) VALUES ($1,$2,$3,false)",
		large.GetId(), large.GetName(), large.GetMainBranch())
	// This frame exceeds the socket's buffers, even when the peer reads its envelope header.
	const folderCount = 1536
	pathPrefix := "/folder/" + strings.Repeat("a", 7900)
	storetest.Exec(t, f.obs, `INSERT INTO repository_folders(id,repository_id,machine_id,resolved_path)
		SELECT '01900000-0000-7000-8000-' || lpad(i::text,12,'0'), $1, $2, $3 || i::text
		FROM generate_series(1,$4) AS i`, large.GetId(), machine.ID, pathPrefix, folderCount)
	for i := 1; i <= folderCount; i++ {
		large.Folders = append(large.Folders, &webv1.RepositoryFolder{
			Id: fmt.Sprintf("01900000-0000-7000-8000-%012d", i), ClientId: machine.ID,
			ResolvedPath: fmt.Sprintf("%s%d", pathPrefix, i),
		})
	}
	f.publish(t, []store.ChangeRef{{Topic: "repositories", Key: large.GetId(), Change: store.Saved}})
	flags, size = projectEnvelope(t, body)
	if flags != 0 || size < 10<<20 {
		t.Fatal("The real HTTP/1.1 stream did not begin its oversized saved frame.")
	}
	if err := tcp.SetReadDeadline(time.Now().Add(9 * time.Second)); err != nil {
		t.Fatal("The fixture could not bound observation of the socket interruption.")
	}

	// The in-flight saved reference counts toward lag, and 2,047 later references reach the limit.
	f.fillProjectLag(t, 2048)

	if err := tcp.SetReadBuffer(1 << 20); err != nil {
		t.Fatal("The fixture could not restore the socket's receive capacity.")
	}
	if _, err := io.CopyN(io.Discard, body, size); err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, syscall.ECONNRESET) {
			t.Fatal("The partially written watch frame did not end with a closed connection.")
		}
	} else {
		flags, size := projectEnvelope(t, body)
		if flags != 2 || size > 4096 {
			t.Fatal("The lagged socket sent another data frame instead of its terminal response.")
		}
		var end struct {
			Error struct{ Code, Message string }
		}
		if json.NewDecoder(io.LimitReader(body, size)).Decode(&end) != nil ||
			end.Error.Code != "aborted" || end.Error.Message != projectLagSentence {
			t.Fatal("The lagged socket did not return the fixed aborted response.")
		}
	}
	requireProjectSnapshot(t, f.watch(t), f.head(t), []*webv1.Repository{large})
}
