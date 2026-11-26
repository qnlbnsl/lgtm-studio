package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

const (
	appMachineID = "0195977c-0000-7000-8000-000000000008"
	appTokenID   = "0195977c-0000-7000-8000-000000000009"
)

type appPeer struct {
	linkv1connect.UnimplementedLinkServiceHandler
	server *httptest.Server
	pub    ed25519.PublicKey
	key    ed25519.PrivateKey
	secret []byte
	token  string
	join   func(*appPeer, *linkv1.JoinRequest) (*linkv1.JoinResponse, error)
	open   func(*appPeer, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error
}

func newAppPeer(t *testing.T, join func(*appPeer, *linkv1.JoinRequest) (*linkv1.JoinResponse, error), open func(*appPeer, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error) *appPeer {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("could not generate the server fixture key")
	}
	p := &appPeer{pub: pub, key: key, secret: randBytes(), join: join, open: open}
	_, handler := linkv1connect.NewLinkServiceHandler(p)
	p.server = httptest.NewUnstartedServer(handler)
	p.server.Config.Protocols = new(http.Protocols)
	p.server.Config.Protocols.SetUnencryptedHTTP2(true)
	p.server.Start()
	t.Cleanup(p.server.Close)
	pin := sha256.Sum256(pub)
	wire, err := proto.Marshal(&linkv1.JoinToken{
		TokenId: appTokenID, Secret: p.secret, Address: p.server.URL, ServerKeySha256: pin[:],
	})
	if err != nil {
		t.Fatal("could not build the join token fixture")
	}
	p.token = "lgtm1_" + base64.RawURLEncoding.EncodeToString(wire)
	return p
}

func randBytes() []byte {
	data := make([]byte, 32)
	rand.Read(data)
	return data
}

func (p *appPeer) Join(_ context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	if p.join == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected join"))
	}
	reply, err := p.join(p, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(reply), nil
}

func (p *appPeer) Open(_ context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	if p.open == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected stream"))
	}
	return p.open(p, stream)
}

func (p *appPeer) reply() *linkv1.JoinResponse {
	return &linkv1.JoinResponse{MachineId: appMachineID, ServerPublicKey: p.pub}
}

func (p *appPeer) challenge(stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	frame, err := stream.Receive()
	if err != nil {
		return err
	}
	hello := frame.GetHello()
	if hello.GetMachineId() != appMachineID || hello.GetLinkRevision() != 1 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected hello"))
	}
	nonce := randBytes()
	message := append([]byte("lgtm-link-v1 server\x00"), byte(len(appMachineID)))
	message = append(message, appMachineID...)
	message = append(message, hello.GetMachineNonce()...)
	message = append(message, nonce...)
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{
		ServerNonce: nonce, ServerSignature: ed25519.Sign(p.key, message),
	}}})
}

func (p *appPeer) welcome(stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], mark uint64, approved bool) error {
	if err := p.challenge(stream); err != nil {
		return err
	}
	frame, err := stream.Receive()
	if err != nil {
		return err
	}
	if frame.GetProof() == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("missing proof"))
	}
	standing := &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_WAITING, Revision: 1}
	welcome := &linkv1.Welcome{State: standing, LastEventNumber: mark, ServerTime: timestamppb.Now(), LinkRevision: 1}
	if approved {
		standing.Approval, standing.Name = linkv1.Approval_APPROVAL_APPROVED, "test machine"
		welcome.ConnectionToken = "fixture-stream"
	}
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: welcome}})
}

func appOptions(t *testing.T) (Options, *bytes.Buffer) {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil || os.Chmod(home, 0o700) != nil {
		t.Fatal("could not prepare a private fixture home")
	}
	log := new(bytes.Buffer)
	return Options{
		StateDir: filepath.Join(home, "client"), Stderr: log,
		Getenv: func(name string) string {
			if name == "HOME" {
				return home
			}
			return ""
		},
	}, log
}

func appContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func appFolder(t *testing.T, dir string) *state.Folder {
	t.Helper()
	f, err := state.Open(dir)
	if err != nil {
		t.Fatalf("could not prepare or reacquire the fixture state folder: %v", err)
	}
	return f
}

func closeAppFolder(t *testing.T, f *state.Folder) {
	t.Helper()
	if f.Close() != nil {
		t.Fatal("could not close the fixture state folder")
	}
}

func seedApp(t *testing.T, opts Options, p *appPeer, mismatch bool) {
	t.Helper()
	f := appFolder(t, opts.StateDir)
	defer closeAppFolder(t, f)
	credential, err := enroll.Create(f.Paths().Key)
	if err != nil {
		t.Fatal("could not create the machine fixture key")
	}
	machineKey := credential.PublicKey()
	if mismatch {
		machineKey = p.pub
	}
	store, err := link.CreateStore(f, link.StoreOptions{})
	if err != nil || store.Close() != nil {
		t.Fatal("could not prepare the fixture store")
	}
	if enroll.SaveServer(f, enroll.Server{
		Address: p.server.URL, MachineID: appMachineID, PublicKey: p.pub, ClientPublicKey: machineKey,
	}) != nil {
		t.Fatal("could not prepare the fixture enrollment")
	}
}

func appFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("could not read a required state file")
	}
	return data
}

func noAppFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("a file was published before the required operation succeeded")
	}
}

func useAppLink(t *testing.T, fn func(context.Context, link.RunOptions) (link.RunOutcome, error)) {
	t.Helper()
	old := runLink
	runLink = fn
	t.Cleanup(func() { runLink = old })
}

type appDiagnostic struct {
	Msg     string
	Code    string
	Message string
	Level   string
}

func appLogs(t *testing.T, log *bytes.Buffer) []appDiagnostic {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(log.Bytes()))
	var lines []appDiagnostic
	for d.More() {
		var line appDiagnostic
		if d.Decode(&line) != nil {
			t.Fatal("a client diagnostic is not valid JSON")
		}
		lines = append(lines, line)
	}
	return lines
}

func oneAppLog(t *testing.T, lines []appDiagnostic, msg, code string) appDiagnostic {
	t.Helper()
	var found appDiagnostic
	count := 0
	for _, line := range lines {
		if line.Msg == msg && line.Code == code {
			found = line
			count++
		}
	}
	if count != 1 {
		t.Errorf("got %d records for %s/%s, want one", count, msg, code)
	}
	return found
}

func (p *appPeer) checkRedaction(t *testing.T, text string, machineFile []byte) {
	t.Helper()
	if strings.Contains(text, p.token) {
		t.Error("the application exposed the join token")
	}
	sensitive := [][]byte{p.secret, p.key, p.key.Seed(), p.pub, machineFile}
	if block, _ := pem.Decode(machineFile); block != nil {
		sensitive = append(sensitive, block.Bytes)
		if value, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
			if key, ok := value.(ed25519.PrivateKey); ok {
				sensitive = append(sensitive, key, key.Seed(), key.Public().(ed25519.PublicKey))
			}
		}
	}
	for _, value := range sensitive {
		if len(value) == 0 {
			continue
		}
		for _, encoded := range []string{
			string(value), hex.EncodeToString(value), base64.StdEncoding.EncodeToString(value),
			base64.RawURLEncoding.EncodeToString(value),
		} {
			if strings.Contains(text, encoded) {
				t.Error("the application exposed fixture key or secret material")
			}
		}
	}
}

func appJoinRefusal(code, text string) error {
	err := connect.NewError(connect.CodePermissionDenied, errors.New(text))
	err.Meta().Set("Lgtm-Join-Refusal", code)
	return err
}

func TestAppJoinCreatesStoreBeforeIdentity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		badCreate bool
		badClose  bool
	}{
		{name: "success"},
		{name: "create failure", badCreate: true},
		{name: "close failure", badClose: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, log := appOptions(t)
			paths := state.PathsIn(opts.StateDir)
			requests := make(chan *linkv1.JoinRequest, 3)
			var calls atomic.Int32
			p := newAppPeer(t, func(p *appPeer, req *linkv1.JoinRequest) (*linkv1.JoinResponse, error) {
				select {
				case requests <- req:
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected retry"))
				}
				if tc.name == "success" && calls.Add(1) == 1 {
					return nil, connect.NewError(connect.CodeUnavailable, errors.New("the first reply was lost"))
				}
				return p.reply(), nil
			}, nil)
			if tc.badCreate {
				closeAppFolder(t, appFolder(t, opts.StateDir))
				if os.Mkdir(paths.Store, 0o700) != nil {
					t.Fatal("could not obstruct creation of the fixture store")
				}
			}
			var closed bool
			old := closeCreatedStore
			closeCreatedStore = func(s *link.Store) error {
				noAppFile(t, paths.Identity)
				if err := s.Close(); err != nil {
					return err
				}
				closed = true
				if tc.badClose {
					return errors.New(p.token)
				}
				return nil
			}
			t.Cleanup(func() { closeCreatedStore = old })

			err := Join(appContext(t), opts, p.server.URL, p.token)
			wantErr := tc.badCreate || tc.badClose
			if (err != nil) != wantErr {
				t.Error("Join did not return the store operation's expected success or failure")
			}
			if closed == tc.badCreate {
				t.Error("the store did not reach its required close boundary")
			}
			wantCalls := 1
			if !wantErr {
				wantCalls = 2
			}
			if len(requests) != wantCalls {
				t.Fatalf("got %d Join requests, want %d", len(requests), wantCalls)
			}
			req := <-requests
			host, hostErr := os.Hostname()
			credential, keyErr := enroll.Load(paths.Key)
			if keyErr != nil || hostErr != nil || req.GetHostName() != host ||
				req.GetTokenId() != appTokenID || !bytes.Equal(req.GetTokenSecret(), p.secret) ||
				!bytes.Equal(req.GetMachinePublicKey(), credential.PublicKey()) {
				t.Error("the join did not send the expected token, host, and durable machine key")
			}
			if wantCalls == 2 && !proto.Equal(req, <-requests) {
				t.Error("the retry did not reuse the first machine key and join request")
			}
			result := log.String()
			if err != nil {
				result += err.Error()
			}
			p.checkRedaction(t, result, appFile(t, paths.Key))
			f := appFolder(t, opts.StateDir)
			if wantErr {
				noAppFile(t, paths.Identity)
				closeAppFolder(t, f)
				return
			}
			saved, err := enroll.LoadServer(f)
			if err != nil || saved.MachineID != appMachineID || !bytes.Equal(saved.ClientPublicKey, req.GetMachinePublicKey()) {
				t.Error("the published identity does not describe the verified enrollment")
			}
			store, err := link.OpenStore(f, link.StoreOptions{})
			if err != nil || store.Close() != nil {
				t.Error("the published identity does not have a usable store")
			}
			closeAppFolder(t, f)
			log.Reset()
			err = Join(appContext(t), opts, p.server.URL, p.token)
			if err == nil || !strings.Contains(log.String(), "This folder is already joined.") {
				t.Error("a second Join did not direct the owner to the existing enrollment")
			}
		})
	}
}

func TestAppRunRequiresEnrollment(t *testing.T) {
	for _, name := range []string{"missing", "malformed", "key mismatch"} {
		t.Run(name, func(t *testing.T) {
			opts, log := appOptions(t)
			p := newAppPeer(t, nil, nil)
			switch name {
			case "malformed":
				f := appFolder(t, opts.StateDir)
				if os.WriteFile(f.Paths().Identity, []byte("{"), 0o600) != nil {
					t.Fatal("could not write malformed identity fixture")
				}
				closeAppFolder(t, f)
			case "key mismatch":
				seedApp(t, opts, p, true)
			}
			useAppLink(t, func(context.Context, link.RunOptions) (link.RunOutcome, error) {
				t.Error("Run tried to start the link with an untrusted identity")
				return link.RunStopped, nil
			})
			if got := Run(appContext(t), opts); got != 1 {
				t.Errorf("Run returned %d, want 1 for bad enrollment", got)
			}
			if !strings.Contains(strings.ToLower(log.String()), "join") {
				t.Error("Run did not tell the owner to join")
			}
			closeAppFolder(t, appFolder(t, opts.StateDir))
		})
	}
}

func TestAppRunClosesState(t *testing.T) {
	opts, log := appOptions(t)
	p := newAppPeer(t, nil, nil)
	seedApp(t, opts, p, false)
	home := opts.Getenv("HOME")
	paths := state.PathsIn(opts.StateDir)
	opts.StateDir = ""
	opts.Getenv = func(name string) string {
		switch name {
		case "HOME":
			return home
		case "PATH":
			return filepath.Join(home, "bin")
		case "LGTM_CLIENT_STATE_DIR":
			return paths.Dir
		default:
			return ""
		}
	}
	if os.WriteFile(paths.Settings, []byte(`{"version":1,"repositories_folder":"~/work","agent_limit":7}`), 0o600) != nil {
		t.Fatal("could not save fixture settings")
	}
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	useAppLink(t, func(ctx context.Context, cfg link.RunOptions) (link.RunOutcome, error) {
		if cfg.Version != "0.0.0-dev" || cfg.Settings.AgentLimit != 7 ||
			cfg.Settings.RepositoriesFolder != filepath.Join(home, "work") ||
			cfg.Server.MachineID != appMachineID || cfg.Credential == nil ||
			cfg.ScanOptions.Home != home || len(cfg.ScanOptions.SearchPath) != 1 ||
			cfg.ScanOptions.SearchPath[0] != filepath.Join(home, "bin") || cfg.Folder == nil || cfg.Folder.Paths().Dir != paths.Dir {
			t.Error("the link did not receive resolved application options")
		}
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return link.RunStopped, nil
	})
	ctx, cancel := context.WithCancel(appContext(t))
	defer cancel()
	done := make(chan struct{})
	var status int
	go func() {
		defer close(done)
		status = Run(ctx, opts)
	}()
	defer func() {
		cancel()
		close(release)
		<-done
	}()

	awaitAppSignal(t, ctx, entered, done, "Run did not start the link")
	assertAppLocked(t, paths.Dir)
	cancel()
	select {
	case <-canceled:
	case <-done:
		t.Fatal("Run exited before its link observed cancellation")
	case <-time.After(5 * time.Second):
		t.Fatal("the link did not observe cancellation")
	}
	assertAppLocked(t, paths.Dir)
	// Release the worker before inspecting logs and reacquiring the folder.
	t.Cleanup(func() {
		if status != 0 {
			t.Errorf("cancellation returned %d, want 0", status)
		}
		if !strings.Contains(log.String(), paths.Dir) || !strings.Contains(log.String(), filepath.Join(home, "work")) {
			t.Error("startup did not log both resolved folders")
		}
		closeAppFolder(t, appFolder(t, paths.Dir))
	})
}

func awaitAppSignal(t *testing.T, ctx context.Context, signal, done <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-signal:
	case <-done:
		t.Fatal(failure)
	case <-ctx.Done():
		t.Fatal(failure)
	}
}

func assertAppLocked(t *testing.T, dir string) {
	t.Helper()
	f, err := state.Open(dir)
	if !errors.Is(err, state.ErrFolderLocked) {
		if f != nil {
			_ = f.Close()
		}
		t.Error("the state folder was available while the link worker was still running")
	}
}

func TestAppLogRedactsSecrets(t *testing.T) {
	for _, tc := range []struct {
		name, code, reported string
		failures             int
	}{
		{name: "used", code: "token_used", reported: "token_used"},
		{name: "unrecognized refusal metadata", reported: "token_refused"},
		{name: "pending expiry", code: "token_expired", reported: "token_expired", failures: 2},
		{name: "loopback signing mismatch", reported: "server_key_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, log := appOptions(t)
			var calls atomic.Int32
			p := newAppPeer(t, func(p *appPeer, req *linkv1.JoinRequest) (*linkv1.JoinResponse, error) {
				attempt := calls.Add(1)
				peerText := p.token + " " + base64.StdEncoding.EncodeToString(req.GetTokenSecret()) +
					" " + base64.StdEncoding.EncodeToString(req.GetMachinePublicKey())
				if int(attempt) <= tc.failures {
					return nil, connect.NewError(connect.CodeUnavailable, errors.New(peerText))
				}
				if tc.name == "loopback signing mismatch" {
					reply := p.reply()
					reply.ServerPublicKey = bytes.Clone(p.pub)
					reply.ServerPublicKey[0] ^= 1
					return reply, nil
				}
				code := tc.code
				if code == "" {
					code = p.token
				}
				return nil, appJoinRefusal(code, peerText)
			}, nil)
			err := Join(appContext(t), opts, p.server.URL, p.token)
			if err == nil {
				t.Error("Join succeeded after the peer refused or returned the wrong key")
			}
			paths := state.PathsIn(opts.StateDir)
			if _, statErr := os.Stat(paths.Key); statErr != nil {
				t.Fatal("the join did not leave the expected pending key")
			}
			text := log.String()
			if err != nil {
				text += err.Error()
			}
			p.checkRedaction(t, text, appFile(t, paths.Key))
			noAppFile(t, paths.Identity)

			lines := appLogs(t, log)
			if refusal := oneAppLog(t, lines, "join_refused", tc.reported); refusal.Message == "" {
				t.Error("the refusal did not contain fixed message text")
			}
			switch tc.name {
			case "pending expiry":
				retry := oneAppLog(t, lines, "join_retrying", "")
				if retry.Message != "The server has not answered. Retrying enrollment." {
					t.Error("the first retry did not log its fixed sentence")
				}
				text = strings.ToLower(log.String())
				for _, want := range []string{"saved key", "waiting", "approved", "host name", "join time", "remove", "new token", "fresh folder"} {
					if !strings.Contains(text, want) {
						t.Errorf("expiry after a pending join omitted recovery guidance about %s", want)
					}
				}
			case "loopback signing mismatch":
				text = strings.ToLower(log.String())
				if !strings.Contains(text, "revoke") || !strings.Contains(text, "token") {
					t.Error("the loopback mismatch did not direct the owner to revoke the token")
				}
			}
		})
	}
}

func TestAppPreservesStateOnRemoval(t *testing.T) {
	opts, log := appOptions(t)
	p := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		if err := p.challenge(stream); err != nil {
			return err
		}
		frame, err := stream.Receive()
		if err != nil {
			return err
		}
		if frame.GetProof() == nil {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("missing proof"))
		}
		refusal := connect.NewError(connect.CodeFailedPrecondition, errors.New(p.token))
		refusal.Meta().Set("Lgtm-Link-Refusal", "unknown_machine")
		return refusal
	})
	seedApp(t, opts, p, false)
	paths := state.PathsIn(opts.StateDir)
	saved := map[string][]byte{
		paths.Identity: appFile(t, paths.Identity),
		paths.Key:      appFile(t, paths.Key),
		paths.Store:    appFile(t, paths.Store),
	}
	ctx := appContext(t)
	if got := Run(ctx, opts); got != 0 || ctx.Err() != nil {
		t.Errorf("verified removal returned %d, want 0 before cancellation", got)
	}
	p.checkRedaction(t, log.String(), saved[paths.Key])
	oneAppLog(t, appLogs(t, log), "link_refused", "unknown_machine")
	for path, before := range saved {
		if !bytes.Equal(before, appFile(t, path)) {
			t.Error("verified removal changed a saved state file")
		}
	}
	closeAppFolder(t, appFolder(t, opts.StateDir))
}

func TestAppRefusesUnsyncedIdentity(t *testing.T) {
	opts, log := appOptions(t)
	p := newAppPeer(t, nil, nil)
	seedApp(t, opts, p, false)
	dirs := 0
	t.Cleanup(state.SetSyncFolderForTest(func(dir string) error {
		if dir == opts.StateDir {
			dirs++
			if dirs > 1 {
				return errors.New(p.token)
			}
		}
		return nil
	}))
	useAppLink(t, func(context.Context, link.RunOptions) (link.RunOutcome, error) {
		t.Error("Run started the link after failing to sync the identity")
		return link.RunStopped, nil
	})
	if got := Run(appContext(t), opts); got != 1 {
		t.Errorf("unsynced identity returned %d, want 1", got)
	}
	if dirs < 2 || !strings.Contains(strings.ToLower(log.String()), "join") {
		t.Error("Run did not check the saved identity's durability and direct the owner to join")
	}
	p.checkRedaction(t, log.String(), appFile(t, state.PathsIn(opts.StateDir).Key))
}

func TestAppRunStoreStatus(t *testing.T) {
	t.Run("retryable local fault", func(t *testing.T) {
		opts, log := appOptions(t)
		p := newAppPeer(t, nil, nil)
		seedApp(t, opts, p, false)
		useAppLink(t, func(context.Context, link.RunOptions) (link.RunOutcome, error) {
			return link.RunStopped, errors.Join(link.ErrStoreFailed, errors.New(p.token))
		})
		if got := Run(appContext(t), opts); got != 1 {
			t.Errorf("retryable local fault returned %d, want 1", got)
		}
		p.checkRedaction(t, log.String(), appFile(t, state.PathsIn(opts.StateDir).Key))
		closeAppFolder(t, appFolder(t, opts.StateDir))
	})
	t.Run("invalid outcome", func(t *testing.T) {
		opts, log := appOptions(t)
		p := newAppPeer(t, nil, nil)
		seedApp(t, opts, p, false)
		useAppLink(t, func(context.Context, link.RunOptions) (link.RunOutcome, error) {
			return 0, nil
		})
		if got := Run(appContext(t), opts); got != 1 {
			t.Errorf("invalid outcome returned %d, want 1", got)
		}
		var invalidLogged bool
		for _, line := range appLogs(t, log) {
			text := strings.ToLower(line.Msg + " " + line.Code + " " + line.Message)
			if line.Level == "ERROR" && (strings.Contains(text, "invalid") || strings.Contains(text, "unexpected")) {
				invalidLogged = true
			}
		}
		if !invalidLogged {
			t.Error("Run did not log an invalid link outcome as an error")
		}
		closeAppFolder(t, appFolder(t, opts.StateDir))
	})
	for _, tc := range []struct {
		name     string
		code     string
		mark     uint64
		approved bool
	}{
		{"sequence exhaustion", "sequence_exhausted", math.MaxInt64, true},
		{"impossible welcome mark", "impossible_ack", 1, false},
		{"impossible live mark", "impossible_ack", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, log := appOptions(t)
			p := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
				if err := p.welcome(stream, tc.mark, tc.approved); err != nil {
					return err
				}
				if tc.name == "impossible live mark" {
					if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{
						EventAck: &linkv1.EventAck{Through: 1},
					}}); err != nil {
						return err
					}
				}
				for {
					if _, err := stream.Receive(); err != nil {
						return err
					}
				}
			})
			seedApp(t, opts, p, false)
			paths := state.PathsIn(opts.StateDir)
			if tc.approved {
				seedExhaustedAppStore(t, paths.Store)
			}
			before := appFile(t, paths.Store)
			ctx := appContext(t)
			if got := Run(ctx, opts); got != 0 || ctx.Err() != nil {
				t.Errorf("fatal store condition returned %d, want 0 before cancellation", got)
			}
			record := oneAppLog(t, appLogs(t, log), "link_stopped", tc.code)
			if record.Level != "ERROR" || record.Message != "An owner must decide how to recover this machine's saved events before restarting." {
				t.Error("fatal store condition did not log the exact recovery diagnostic")
			}
			if !bytes.Equal(before, appFile(t, paths.Store)) {
				t.Error("fatal store condition changed client.db")
			}
			closeAppFolder(t, appFolder(t, opts.StateDir))
		})
	}
}

func seedExhaustedAppStore(t *testing.T, path string) {
	t.Helper()
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		t.Fatal("could not open the fixture database")
	}
	err = db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("meta")).Put([]byte("next"), binary.BigEndian.AppendUint64(nil, math.MaxInt64+1))
	})
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatal("could not save the exhausted sequence fixture")
	}
}
