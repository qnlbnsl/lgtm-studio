package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

const (
	commandMachineID = "0195977c-0000-7000-8000-000000000008"
	commandTokenID   = "0195977c-0000-7000-8000-000000000009"
	commandVersion   = "0.0.0-test"
	clientUsage      = "Usage: lgtm-client {join|run|version}\n"
)

// TestLinkCommandProcess enters main in a subprocess so exit status, signals
// and standard input cross the same boundary as they do in the installer.
func TestLinkCommandProcess(t *testing.T) {
	mode := os.Getenv("LGTM_TEST_LINK_COMMAND_PROCESS")
	if mode == "" {
		t.Skip("subprocess entry")
	}
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		os.Args = append([]string{"lgtm-client"}, os.Args[i+1:]...)
		version = commandVersion
		if mode == "commit-fault" {
			// Open and reads can succeed, but a database page write fails.
			signal.Ignore(syscall.SIGXFSZ)
			var limit unix.Rlimit
			if unix.Getrlimit(unix.RLIMIT_FSIZE, &limit) != nil {
				os.Exit(98)
			}
			limit.Cur = 1
			if unix.Setrlimit(unix.RLIMIT_FSIZE, &limit) != nil {
				os.Exit(98)
			}
		}
		main()
	}
	t.Fatal("the subprocess did not receive the command arguments")
}

type processLog struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	change chan struct{}
}

func (l *processLog) Write(data []byte) (int, error) {
	l.mu.Lock()
	n, err := l.buffer.Write(data)
	l.mu.Unlock()
	select {
	case l.change <- struct{}{}:
	default:
	}
	return n, err
}

func (l *processLog) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}

type commandRecord struct {
	Msg      string
	Code     string
	Message  string
	Sequence uint64
}

func (l *processLog) records() []commandRecord {
	var records []commandRecord
	for _, line := range strings.Split(l.text(), "\n") {
		var record commandRecord
		if json.Unmarshal([]byte(line), &record) == nil {
			records = append(records, record)
		}
	}
	return records
}

func (l *processLog) count(msg, code string) int {
	count := 0
	for _, record := range l.records() {
		if record.Msg == msg && record.Code == code {
			count++
		}
	}
	return count
}

type commandProcess struct {
	ctx    context.Context
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr processLog
	done   chan struct{}
	status int
}

func startLinkCommand(t *testing.T, dir, mode string, in io.Reader, args ...string) *commandProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	executable, err := os.Executable()
	if err != nil {
		cancel()
		t.Fatal("could not find the command fixture program")
	}
	cmd := exec.CommandContext(ctx, executable, append([]string{"-test.run=^TestLinkCommandProcess$", "--"}, args...)...)
	cmd.Args[0] = "lgtm-client"
	cmd.Env = append(os.Environ(), "LGTM_TEST_LINK_COMMAND_PROCESS="+mode,
		"LGTM_CLIENT_STATE_DIR="+dir, "HOME="+filepath.Dir(dir), "PATH=", "GORACE=atexit_sleep_ms=0")
	p := &commandProcess{ctx: ctx, cmd: cmd, stderr: processLog{change: make(chan struct{}, 1)}, done: make(chan struct{})}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, &p.stdout, &p.stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal("could not start the command fixture")
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			var exit *exec.ExitError
			p.status = -1
			if errors.As(err, &exit) {
				p.status = exit.ExitCode()
			}
		}
		close(p.done)
	}()
	t.Cleanup(func() {
		cancel()
		<-p.done
	})
	return p
}

func (p *commandProcess) exit(t *testing.T, want int) {
	t.Helper()
	select {
	case <-p.done:
		if p.status != want {
			t.Errorf("command returned %d, want %d", p.status, want)
		}
	case <-p.ctx.Done():
		t.Fatal("the command did not exit before the fixture deadline")
	}
}

func (p *commandProcess) waitLog(t *testing.T, msg, code string) {
	t.Helper()
	for p.stderr.count(msg, code) == 0 {
		select {
		case <-p.stderr.change:
		case <-p.done:
			if p.stderr.count(msg, code) != 0 {
				return
			}
			t.Fatalf("the command exited before emitting %s/%s", msg, code)
		case <-p.ctx.Done():
			t.Fatalf("the command did not emit %s/%s", msg, code)
		}
	}
}

func waitCommandPeer[T any](t *testing.T, p *commandProcess, received <-chan T) T {
	t.Helper()
	select {
	case value := <-received:
		return value
	case <-p.done:
		select {
		case value := <-received:
			return value
		default:
			t.Fatalf("the command exited with %d before the listener observed the expected request", p.status)
		}
	case <-p.ctx.Done():
		t.Fatal("the command did not reach the listener")
	}
	var zero T
	return zero
}

type commandStream = connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]

type commandPeer struct {
	linkv1connect.UnimplementedLinkServiceHandler
	server *httptest.Server
	public ed25519.PublicKey
	key    ed25519.PrivateKey
	secret []byte
	token  string
	join   func(context.Context, *linkv1.JoinRequest) (*linkv1.JoinResponse, error)
	open   func(context.Context, *commandStream) error
}

func newCommandPeer(t *testing.T) *commandPeer {
	t.Helper()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("could not generate a command fixture key")
	}
	p := &commandPeer{public: public, key: key, secret: make([]byte, 32)}
	rand.Read(p.secret)
	return p
}

func (p *commandPeer) start(t *testing.T) {
	t.Helper()
	_, handler := linkv1connect.NewLinkServiceHandler(p)
	p.server = httptest.NewUnstartedServer(handler)
	p.server.Config.Protocols = new(http.Protocols)
	p.server.Config.Protocols.SetUnencryptedHTTP2(true)
	p.server.Start()
	t.Cleanup(p.server.Close)
	pin := sha256.Sum256(p.public)
	wire, err := proto.Marshal(&linkv1.JoinToken{
		TokenId: commandTokenID, Secret: p.secret, Address: p.server.URL, ServerKeySha256: pin[:],
	})
	if err != nil {
		t.Fatal("could not encode the command fixture token")
	}
	p.token = "lgtm1_" + base64.RawURLEncoding.EncodeToString(wire)
}

func (p *commandPeer) Join(ctx context.Context, request *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	if p.join == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected join"))
	}
	if request.Msg.GetTokenId() != commandTokenID || !bytes.Equal(request.Msg.GetTokenSecret(), p.secret) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("wrong join payload"))
	}
	response, err := p.join(ctx, request.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(response), nil
}

func (p *commandPeer) Open(ctx context.Context, stream *commandStream) error {
	if p.open == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected stream"))
	}
	return p.open(ctx, stream)
}

func (p *commandPeer) challenge(stream *commandStream) error {
	frame, err := stream.Receive()
	if err != nil {
		return err
	}
	hello := frame.GetHello()
	if hello.GetMachineId() != commandMachineID || hello.GetLinkRevision() != 1 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("wrong machine identity"))
	}
	nonce := make([]byte, 32)
	rand.Read(nonce)
	text := append([]byte("lgtm-link-v1 server\x00"), byte(len(commandMachineID)))
	text = append(text, commandMachineID...)
	text = append(text, hello.GetMachineNonce()...)
	text = append(text, nonce...)
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{
		ServerNonce: nonce, ServerSignature: ed25519.Sign(p.key, text),
	}}})
}

func receiveCommandProof(stream *commandStream) error {
	proof, err := stream.Receive()
	if err != nil {
		return err
	}
	if proof.GetProof() == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("missing machine proof"))
	}
	return nil
}

func (p *commandPeer) welcome(stream *commandStream, mark uint64, approved bool) error {
	if err := p.challenge(stream); err != nil {
		return err
	}
	if err := receiveCommandProof(stream); err != nil {
		return err
	}
	standing := &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_WAITING, Revision: 1}
	welcome := &linkv1.Welcome{State: standing, LastEventNumber: mark, ServerTime: timestamppb.Now(), LinkRevision: 1}
	if approved {
		standing.Approval, standing.Name = linkv1.Approval_APPROVAL_APPROVED, "fixture machine"
		welcome.ConnectionToken = "fixture-stream"
	}
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: welcome}})
}

func commandStateDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if os.Chmod(home, 0o700) != nil {
		t.Fatal("could not prepare a private command fixture home")
	}
	return filepath.Join(home, "lgtm-client")
}

func seedCommandState(t *testing.T, dir string, p *commandPeer, buffered, wrongServerKey bool) {
	t.Helper()
	f, err := state.Open(dir)
	if err != nil {
		t.Fatalf("could not open the private command state fixture: %v", err)
	}
	defer func() {
		if f.Close() != nil {
			t.Error("could not close the command state fixture")
		}
	}()
	credential, err := enroll.Create(f.Paths().Key)
	if err != nil {
		t.Fatal("could not create the command fixture machine key")
	}
	store, err := link.CreateStore(f, link.StoreOptions{})
	if err != nil {
		t.Fatal("could not create the command fixture database")
	}
	if buffered {
		saved, err := proto.Marshal(&linkv1.Event{HappenedAt: timestamppb.Now(), Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{
			Os: "fixture platform", Cores: 1,
		}}})
		if err != nil {
			t.Fatal("could not encode the saved event")
		}
		if number, err := store.Append(append([]byte{1}, saved...)); err != nil || number != 1 {
			t.Fatal("could not append event one to the command fixture database")
		}
	}
	if store.Close() != nil {
		t.Fatal("could not close the command fixture database")
	}
	if enroll.SaveServer(f, enroll.Server{
		Address: p.server.URL, MachineID: commandMachineID,
		PublicKey: p.public, ClientPublicKey: credential.PublicKey(),
	}) != nil {
		t.Fatal("could not save the command fixture identity")
	}
	if wrongServerKey {
		path := f.Paths().Identity
		var identity map[string]json.RawMessage
		if json.Unmarshal(commandFile(t, path), &identity) != nil ||
			len(identity["clientPublicKey"]) == 0 || len(identity["serverPublicKey"]) == 0 ||
			bytes.Equal(identity["clientPublicKey"], identity["serverPublicKey"]) {
			t.Fatal("the identity fixture does not have two distinct keys")
		}
		identity["serverPublicKey"] = identity["clientPublicKey"]
		replacement, err := json.Marshal(identity)
		if err != nil || os.WriteFile(path, replacement, 0o600) != nil {
			t.Fatal("could not replace the saved server key as in C10")
		}
		if info, err := os.Stat(path); err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Fatal("the edited identity fixture is exposed")
		}
	}
}

func (p *commandPeer) assertRedacted(t *testing.T, text string) {
	t.Helper()
	if strings.Contains(text, p.token) {
		t.Error("the command exposed the join token")
	}
	for _, value := range [][]byte{p.secret, p.key, p.key.Seed(), p.public} {
		for _, encoded := range []string{
			string(value), hex.EncodeToString(value), base64.StdEncoding.EncodeToString(value),
			base64.RawURLEncoding.EncodeToString(value),
		} {
			if strings.Contains(text, encoded) {
				t.Error("the command exposed fixture key or secret material")
			}
		}
	}
}

func (p *commandProcess) checkOutput(t *testing.T, peer *commandPeer) {
	t.Helper()
	peer.assertRedacted(t, p.stderr.text())
	select {
	case <-p.done:
		peer.assertRedacted(t, p.stdout.String())
	default:
		t.Fatal("the test inspected command output before the process finished")
	}
}

func TestRunJoinTokenInput(t *testing.T) {
	attempts := make(chan *linkv1.JoinRequest, 4)
	reply := make(chan struct{}, 1)
	peer := newCommandPeer(t)
	peer.join = func(ctx context.Context, request *linkv1.JoinRequest) (*linkv1.JoinResponse, error) {
		select {
		case attempts <- request:
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected join"))
		}
		select {
		case <-reply:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &linkv1.JoinResponse{MachineId: commandMachineID, ServerPublicKey: peer.public}, nil
	}
	peer.start(t)

	for _, mode := range []string{"stdin", "positional"} {
		t.Run(mode, func(t *testing.T) {
			dir := commandStateDir(t)
			r, w := commandStdinPipe(t)
			tokenArg := peer.token
			if mode == "stdin" {
				tokenArg = "-"
				if _, err := io.WriteString(w, peer.token+"\r\nignored second line\n"); err != nil {
					t.Fatal("could not supply the first token line")
				}
			}
			// An open pipe makes reading the positional form or waiting for
			// end of input after the first line block the join.
			process := startLinkCommand(t, dir, "normal", r, "join", peer.server.URL, tokenArg)
			request := waitCommandPeer(t, process, attempts)
			peer.assertProcessTitle(t, process.cmd.Process.Pid, mode == "stdin")
			reply <- struct{}{}
			process.exit(t, 0)
			process.checkOutput(t, peer)
			if request.GetTokenId() != commandTokenID || !bytes.Equal(request.GetTokenSecret(), peer.secret) {
				t.Error("the join did not forward the supplied token")
			}
			info, err := os.Stat(filepath.Join(dir, "identity.json"))
			if err != nil || info.Mode().Perm()&0o077 != 0 {
				t.Error("the command did not save a private identity directly beneath LGTM_CLIENT_STATE_DIR")
			}
		})
	}
	for _, tc := range []struct{ name, input string }{
		{"empty", ""},
		{"empty line", "\n"},
		{"oversized", peer.token + strings.Repeat("x", 4097-len(peer.token)) + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := commandStateDir(t)
			process := startLinkCommand(t, dir, "normal", strings.NewReader(tc.input), "join", peer.server.URL, "-")
			process.exit(t, 2)
			process.checkOutput(t, peer)
			if process.stderr.text() != clientUsage || process.stdout.Len() != 0 {
				t.Error("invalid token input did not produce only the fixed usage line on standard error")
			}
			if len(attempts) != 0 {
				t.Error("invalid token input reached the enrollment endpoint")
			}
			if _, err := os.Lstat(filepath.Join(dir, "machine.key")); !errors.Is(err, os.ErrNotExist) {
				t.Error("invalid token input created enrollment state")
			}
		})
	}
	t.Run("oversized before end of input", func(t *testing.T) {
		r, w := commandStdinPipe(t)
		dir := commandStateDir(t)
		process := startLinkCommand(t, dir, "normal", r, "join", peer.server.URL, "-")
		supplied := make(chan struct{})
		go func() {
			defer close(supplied)
			_, _ = io.WriteString(w, peer.token+strings.Repeat("x", 4100-len(peer.token)))
		}()
		defer func() {
			_ = w.Close()
			<-supplied
		}()
		process.exit(t, 2)
		process.checkOutput(t, peer)
		if process.stderr.text() != clientUsage || process.stdout.Len() != 0 || len(attempts) != 0 {
			t.Error("oversized input did not stop at usage before enrollment or end of input")
		}
	})

}

func commandStdinPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal("could not create the command standard input fixture")
	}
	t.Cleanup(func() {
		_ = w.Close()
		_ = r.Close()
	})
	return r, w
}

func (p *commandPeer) assertProcessTitle(t *testing.T, pid int, fromStdin bool) {
	t.Helper()
	if runtime.GOOS == "linux" {
		root := filepath.Join("/proc", strconv.Itoa(pid))
		title, err := os.ReadFile(filepath.Join(root, "comm"))
		if err != nil {
			t.Fatal("could not observe the live command's process name")
		}
		p.assertRedacted(t, string(title))
		if strings.Contains(string(title), "lgtm1_") {
			t.Error("the command set its process name from the join token")
		}
		if fromStdin {
			args, err := os.ReadFile(filepath.Join(root, "cmdline"))
			if err != nil {
				t.Fatal("could not observe the live command's process arguments")
			}
			p.assertRedacted(t, string(args))
		}
		return
	}
	column := "ucomm="
	if fromStdin {
		column = "command="
	}
	out, err := exec.Command("/bin/ps", "-ww", "-p", strconv.Itoa(pid), "-o", column).Output()
	if err != nil {
		t.Fatal("could not observe the live command's process title")
	}
	p.assertRedacted(t, string(out))
	if strings.Contains(string(out), "lgtm1_") {
		t.Error("the command set its process title from the join token")
	}
}

func TestRunCommandExitStatus(t *testing.T) {
	t.Run("version and invalid arguments", func(t *testing.T) {
		dir := commandStateDir(t)
		valid := startLinkCommand(t, dir, "normal", nil, "version")
		valid.exit(t, 0)
		if valid.stdout.String() != commandVersion+"\n" || valid.stderr.text() != "" {
			t.Error("version did not print only the injected version on standard output")
		}
		peer := newCommandPeer(t)
		peer.start(t)
		for _, args := range [][]string{
			nil, {"unknown", peer.token}, {"join"}, {"join", peer.server.URL},
			{"join", peer.server.URL, peer.token, "extra"}, {"run", peer.token}, {"version", peer.token},
		} {
			bad := startLinkCommand(t, dir, "normal", nil, args...)
			bad.exit(t, 2)
			bad.checkOutput(t, peer)
			if bad.stdout.Len() != 0 || bad.stderr.text() != clientUsage {
				t.Error("bad command arity did not print only the fixed usage line on standard error")
			}
		}
	})

	for _, code := range []string{"token_used", "token_expired"} {
		t.Run(code, func(t *testing.T) {
			peer := newCommandPeer(t)
			peer.join = func(_ context.Context, req *linkv1.JoinRequest) (*linkv1.JoinResponse, error) {
				remoteText := peer.token + " " + base64.StdEncoding.EncodeToString(req.GetTokenSecret())
				refusal := connect.NewError(connect.CodePermissionDenied, errors.New(remoteText))
				refusal.Meta().Set("Lgtm-Join-Refusal", code)
				return nil, refusal
			}
			peer.start(t)
			dir := commandStateDir(t)
			p := startLinkCommand(t, dir, "normal", nil, "join", peer.server.URL, peer.token)
			p.exit(t, 1)
			p.checkOutput(t, peer)
			if p.stderr.count("join_refused", code) != 1 || p.stdout.Len() != 0 {
				t.Errorf("C5 did not receive a single join_refused/%s record on standard error", code)
			}
		})
	}

	t.Run("verified removal", func(t *testing.T) {
		peer := newCommandPeer(t)
		peer.open = func(_ context.Context, stream *commandStream) error {
			if err := peer.challenge(stream); err != nil {
				return err
			}
			if err := receiveCommandProof(stream); err != nil {
				return err
			}
			refusal := connect.NewError(connect.CodeFailedPrecondition, errors.New(peer.token))
			refusal.Meta().Set("Lgtm-Link-Refusal", "unknown_machine")
			return refusal
		}
		peer.start(t)
		dir := commandStateDir(t)
		seedCommandState(t, dir, peer, false, false)

		p := startLinkCommand(t, dir, "normal", nil, "run")
		p.exit(t, 0)
		p.checkOutput(t, peer)
		if p.stderr.count("link_refused", "unknown_machine") != 1 {
			t.Error("C10 did not receive link_refused/unknown_machine")
		}
	})
	t.Run("mismatched saved server key", func(t *testing.T) {
		peer := newCommandPeer(t)
		proof := make(chan bool, 1)
		peer.open = func(_ context.Context, stream *commandStream) error {
			if err := peer.challenge(stream); err != nil {
				return err
			}
			frame, err := stream.Receive()
			select {
			case proof <- frame.GetProof() != nil:
			default:
			}
			return err
		}
		peer.start(t)
		dir := commandStateDir(t)
		seedCommandState(t, dir, peer, false, true)

		p := startLinkCommand(t, dir, "normal", nil, "run")
		p.waitLog(t, "link_refused", "server_key_mismatch")
		if waitCommandPeer(t, p, proof) {
			t.Error("the command sent a machine proof to a mismatched server")
		}
		stopLinkCommand(t, p, syscall.SIGTERM)
		p.checkOutput(t, peer)
	})
	t.Run("failed commit", func(t *testing.T) {
		peer := newCommandPeer(t)
		welcomed := make(chan struct{}, 1)
		peer.open = func(ctx context.Context, stream *commandStream) error {
			if err := peer.welcome(stream, 1, false); err != nil {
				return err
			}
			select {
			case welcomed <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return ctx.Err()
		}
		peer.start(t)
		dir := commandStateDir(t)
		seedCommandState(t, dir, peer, true, false)
		path := filepath.Join(dir, "client.db")
		before := commandFile(t, path)
		p := startLinkCommand(t, dir, "commit-fault", nil, "run")
		waitCommandPeer(t, p, welcomed)
		p.exit(t, 1)
		p.checkOutput(t, peer)
		if p.stderr.count("link_stopped", "store_failed") != 1 {
			t.Error("the command did not report the failed acknowledgement commit")
		}
		if !bytes.Equal(before, commandFile(t, path)) {
			t.Error("the failed acknowledgement commit changed the durable store")
		}
	})

	t.Run("accepted restart", func(t *testing.T) {
		peer := newCommandPeer(t)
		outcome := make(chan *linkv1.CommandResult, 1)
		const restartID = "0195977c-0000-7000-8000-000000000010"
		peer.open = func(_ context.Context, stream *commandStream) error {
			if err := peer.welcome(stream, 0, true); err != nil {
				return err
			}
			command := &linkv1.Command{
				Id: restartID, Attempt: 1, ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
				Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}},
			}
			if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: command}}); err != nil {
				return err
			}
			for {
				frame, err := stream.Receive()
				if err != nil {
					return err
				}
				if result := frame.GetCommandResult(); result != nil {
					select {
					case outcome <- result:
					default:
					}
					return nil
				}
			}
		}
		peer.start(t)
		dir := commandStateDir(t)
		seedCommandState(t, dir, peer, false, false)

		p := startLinkCommand(t, dir, "normal", nil, "run")
		p.exit(t, 75)
		p.checkOutput(t, peer)
		if p.status == 75 {
			select {
			case result := <-outcome:
				if result.GetCommandId() != restartID || result.GetAttempt() != 1 ||
					result.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_DONE {
					t.Error("the link did not report the accepted restart")
				}
			case <-p.ctx.Done():
				t.Error("the link did not deliver its restart result")
			}
		}
		if p.stderr.count("link_restart_requested", "") != 1 {
			t.Error("the command did not log the foreground restart request")
		}
	})
}

func commandFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("could not read the command fixture file")
	}
	return b
}

func stopLinkCommand(t *testing.T, p *commandProcess, sig os.Signal) {
	t.Helper()
	if err := p.cmd.Process.Signal(sig); err != nil {
		t.Error("could not signal the live command")
	}
	p.exit(t, 0)
}

func TestRunCommandReportsAndReconnects(t *testing.T) {
	type eventObservation struct {
		facts     *linkv1.Facts
		harnesses *linkv1.HarnessesFound
		load      *linkv1.Heartbeat
	}
	observed := make(chan eventObservation, 4)
	breakStream := make(chan struct{})
	peer := newCommandPeer(t)
	var streams, applied atomic.Uint64
	peer.open = func(ctx context.Context, stream *commandStream) error {
		attempt := streams.Add(1)
		if err := peer.welcome(stream, applied.Load(), true); err != nil {
			return err
		}
		var report eventObservation
		for report.facts == nil || report.harnesses == nil || report.load == nil {
			frame, err := stream.Receive()
			if err != nil {
				return err
			}
			if load := frame.GetHeartbeat(); load != nil {
				report.load = load
			}
			event := frame.GetEvent()
			if event == nil {
				continue
			}
			if facts := event.GetFacts(); facts != nil {
				report.facts = facts
			}
			if inventory := event.GetHarnessesFound(); inventory != nil {
				report.harnesses = inventory
			}
			if event.GetNumber() != applied.Load()+1 {
				return connect.NewError(connect.CodeFailedPrecondition, errors.New("fixture replay skipped a sequence"))
			}
			applied.Store(event.GetNumber())
		}
		if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{
			EventAck: &linkv1.EventAck{Through: applied.Load()},
		}}); err != nil {
			return err
		}
		select {
		case observed <- report:
		default:
			return connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected reconnect"))
		}
		if attempt == 1 {
			select {
			case <-breakStream:
				return connect.NewError(connect.CodeUnavailable, errors.New("fixture restart"))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		<-ctx.Done()
		return ctx.Err()
	}
	peer.start(t)
	dir := commandStateDir(t)
	seedCommandState(t, dir, peer, false, false)

	p := startLinkCommand(t, dir, "normal", nil, "run")
	initial := waitCommandPeer(t, p, observed)
	if initial.facts.GetClientVersion() != commandVersion || initial.facts.GetOs() == "" ||
		initial.facts.GetCores() == 0 || initial.facts.GetWorkspace() == "" || initial.facts.GetAgentLimit() == 0 {
		t.Error("run did not report local facts with the injected version")
	}
	var names []string
	for _, harness := range initial.harnesses.GetHarnesses() {
		names = append(names, harness.GetName())
	}
	if strings.Join(names, ",") != "claude,codex,gemini,cursor" {
		t.Error("run did not report all four harnesses in registry order")
	}
	if initial.load.GetCpuPercent() > 100 || initial.load.GetMemoryPercent() > 100 {
		t.Error("run reported load above the permitted percent range")
	}
	close(breakStream)
	waitCommandPeer(t, p, observed)
	stopLinkCommand(t, p, os.Interrupt)
	p.checkOutput(t, peer)

	count := 0
	var previous uint64
	for _, record := range p.stderr.records() {
		if record.Msg != "link_event_buffered" {
			continue
		}
		if record.Sequence <= previous {
			t.Error("C11 did not receive positive, increasing numeric buffered sequences")
		}
		previous = record.Sequence
		count++
	}
	if count < 3 {
		t.Error("C11 did not receive three durable buffered sequence records")
	}

	// C8 starts the same command again with the original state directory.
	reopened := startLinkCommand(t, dir, "normal", nil, "run")
	waitCommandPeer(t, reopened, observed)
	stopLinkCommand(t, reopened, syscall.SIGTERM)
	reopened.checkOutput(t, peer)
}
