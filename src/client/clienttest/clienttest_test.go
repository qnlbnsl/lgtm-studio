package clienttest_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

const (
	machineID             = "0195977c-0000-7000-8000-000000000008"
	tokenID               = "0195977c-0000-7000-8000-000000000009"
	folderConnectionToken = "fixture-folder-connection-token"
)

var runIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type testClientPeer struct {
	linkv1connect.UnimplementedLinkServiceHandler
	public         ed25519.PublicKey
	private        ed25519.PrivateKey
	secret         []byte
	open           chan struct{}
	joins          atomic.Int32
	refuse         atomic.Bool
	machine        atomic.Pointer[linkv1.JoinRequest]
	folderCommand  atomic.Pointer[linkv1.Command]
	folderAnswered chan struct{}
}

func (p *testClientPeer) Join(_ context.Context, request *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	p.joins.Add(1)
	if request.Msg.GetTokenId() != tokenID || !bytes.Equal(request.Msg.GetTokenSecret(), p.secret) ||
		len(request.Msg.GetMachinePublicKey()) != ed25519.PublicKeySize {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("wrong enrollment payload"))
	}
	if p.refuse.Load() {
		refusal := connect.NewError(connect.CodePermissionDenied, errors.New(base64.StdEncoding.EncodeToString(p.secret)))
		refusal.Meta().Set("Lgtm-Join-Refusal", "token_expired")
		return nil, refusal
	}
	p.machine.Store(request.Msg)
	return connect.NewResponse(&linkv1.JoinResponse{MachineId: machineID, ServerPublicKey: p.public}), nil
}

func (p *testClientPeer) Open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	frame, err := stream.Receive()
	if err != nil {
		return err
	}
	hello := frame.GetHello()
	if hello.GetMachineId() != machineID || hello.GetLinkRevision() != 2 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("wrong machine identity"))
	}
	first := hello.GetFirstEventNumber()
	if !runIDPattern.MatchString(hello.GetRunId()) || first == 0 || first > 1<<63 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid run announcement"))
	}
	nonce := make([]byte, 32)
	rand.Read(nonce)
	proofText := func(role string) []byte {
		text := append([]byte("lgtm-link-v1 "+role+"\x00"), byte(len(machineID)))
		text = append(text, machineID...)
		text = append(text, hello.GetMachineNonce()...)
		return append(text, nonce...)
	}
	if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{
		ServerNonce: nonce, ServerSignature: ed25519.Sign(p.private, proofText("server")),
	}}}); err != nil {
		return err
	}
	frame, err = stream.Receive()
	if err != nil {
		return err
	}
	if !ed25519.Verify(p.machine.Load().GetMachinePublicKey(), proofText("machine"), frame.GetProof().GetMachineSignature()) {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("wrong machine proof"))
	}
	command := p.folderCommand.Load()
	welcome := &linkv1.Welcome{
		State:      &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_WAITING, Revision: 1},
		ServerTime: timestamppb.Now(), LinkRevision: 2,
	}
	if command != nil {
		welcome.State.Approval = linkv1.Approval_APPROVAL_APPROVED
		welcome.ConnectionToken = folderConnectionToken
	}
	if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: welcome}}); err != nil {
		return err
	}
	frame, err = stream.Receive()
	if err != nil {
		return err
	}
	if frame.GetHeartbeat() == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("missing heartbeat"))
	}
	select {
	case p.open <- struct{}{}:
	default:
	}
	if command != nil {
		if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: command}}); err != nil {
			return err
		}
		for {
			frame, err := stream.Receive()
			if err != nil {
				return err
			}
			var reply *linkv1.OpenResponse
			switch {
			case frame.GetHeartbeat() != nil:
				reply = &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}}
			case frame.GetEvent() != nil:
				reply = &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{EventAck: &linkv1.EventAck{Through: frame.GetEvent().GetNumber()}}}
			case frame.GetCommandResult().GetCommandId() == command.GetId():
				select {
				case p.folderAnswered <- struct{}{}:
				default:
				}
			}
			if reply != nil {
				if err := stream.Send(reply); err != nil {
					return err
				}
			}
		}
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestClienttestSharesApp(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("could not generate the fixture key")
	}
	p := &testClientPeer{
		public: public, private: private, secret: make([]byte, 32), open: make(chan struct{}, 1),
		folderAnswered: make(chan struct{}, 1),
	}
	rand.Read(p.secret)
	_, handler := linkv1connect.NewLinkServiceHandler(p)
	server := httptest.NewUnstartedServer(handler)
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	t.Cleanup(server.Close)

	pin := sha256.Sum256(public)
	wire, err := proto.Marshal(&linkv1.JoinToken{
		TokenId: tokenID, Secret: p.secret, Address: server.URL, ServerKeySha256: pin[:],
	})
	if err != nil {
		t.Fatal("could not encode the token fixture")
	}
	token := "lgtm1_" + base64.RawURLEncoding.EncodeToString(wire)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil || os.Chmod(home, 0o700) != nil {
		t.Fatal("could not prepare a private fixture home")
	}
	var log bytes.Buffer
	opts := clienttest.Options{
		StateDir: filepath.Join(home, "explicit"), Stderr: &log, Version: "0.0.0-test",
		Getenv: func(name string) string {
			switch name {
			case "HOME":
				return home
			case "LGTM_CLIENT_STATE_DIR":
				return filepath.Join(home, "unused")
			}
			return ""
		},
	}

	t.Run("requires a disposable path", func(t *testing.T) {
		for _, dir := range []string{"", "relative-state"} {
			bad := opts
			bad.StateDir = dir
			bad.Stderr = nil
			err := clienttest.Join(ctx, bad, server.URL, token)
			if err == nil {
				t.Error("Join accepted a missing or relative state directory")
			} else {
				assertClienttestRedaction(t, p, token, err.Error())
			}
			if code := clienttest.Run(ctx, bad); code != 1 {
				t.Errorf("Run returned %d for a missing or relative state directory, want 1", code)
			}
		}
		if p.joins.Load() != 0 {
			t.Fatal("an invalid directory reached enrollment")
		}
	})

	t.Run("enrollment and cancellation", func(t *testing.T) {
		if err := clienttest.Join(ctx, opts, server.URL, token); err != nil {
			assertClienttestRedaction(t, p, token, err.Error())
			t.Fatal("the in-process entry did not join the fixture endpoint")
		}
		if p.joins.Load() != 1 {
			t.Fatal("the in-process entry did not send one enrollment request")
		}
		if info, err := os.Stat(filepath.Join(opts.StateDir, "identity.json")); err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Fatal("Join did not save a private identity under the explicit directory")
		}
		runCtx, stop := context.WithCancel(ctx)
		done := make(chan int, 1)
		go func() { done <- clienttest.Run(runCtx, opts) }()
		defer func() {
			stop()
			select {
			case <-done:
			case <-ctx.Done():
				t.Error("Run did not exit after cancellation")
			}
		}()
		select {
		case <-p.open:
		case code := <-done:
			done <- code
			t.Fatalf("Run exited with %d before it kept a live link", code)
		case <-ctx.Done():
			t.Fatal("the listener did not receive a heartbeat")
		}
		if f, err := state.Open(opts.StateDir); !errors.Is(err, state.ErrFolderLocked) {
			if f != nil {
				_ = f.Close()
			}
			t.Error("Run did not hold the state lock while connected")
		}
		stop()
		select {
		case code := <-done:
			done <- code
			if code != 0 {
				t.Errorf("Run returned %d on cancellation, want 0", code)
			}
		case <-ctx.Done():
			t.Fatal("Run did not return after cancellation")
		}
		assertClienttestRedaction(t, p, token, log.String())
		if f, err := state.Open(opts.StateDir); err != nil {
			t.Error("Run did not release its state lock before returning")
		} else if f.Close() != nil {
			t.Error("the reopened state lock did not close")
		}
	})

	t.Run("folder callbacks reach the real run", func(t *testing.T) {
		command := &linkv1.Command{
			Id: "0195977c-0000-7000-8000-000000000010", Attempt: 1,
			ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
			Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
				OperationId: "0195977c-0000-7000-8000-000000000011", Path: home,
			}},
		}
		p.folderCommand.Store(command)
		defer p.folderCommand.Store(nil)

		folderOpts := opts
		var folderLog bytes.Buffer
		folderOpts.Stderr = &folderLog
		type start struct{ operationID, commandID string }
		starts := make(chan start, 2)
		appends := make(chan uint64, 2)
		folderOpts.OnFolderInspectionStart = func(_ context.Context, operationID, commandID string) {
			select {
			case starts <- start{operationID, commandID}:
			default:
				t.Error("the app called the inspection start callback too many times")
			}
		}
		folderOpts.AfterFolderReport = func(_ context.Context, number uint64) error {
			select {
			case appends <- number:
			default:
				t.Error("the app called the durable append callback too many times")
			}
			return errors.New("fixture interruption after the folder report")
		}

		runCtx, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			clienttest.Run(runCtx, folderOpts)
		}()
		defer func() {
			stop()
			select {
			case <-done:
				assertClienttestRedaction(t, p, token, folderLog.String())
			case <-ctx.Done():
				t.Error("the folder callback run did not return after cancellation")
			}
		}()
		select {
		case <-done:
		case <-p.folderAnswered:
			t.Error("the real client answered the folder command despite the interruption callback")
		case <-ctx.Done():
			t.Error("the folder append callback did not stop the real client")
		}
		select {
		case got := <-starts:
			if got.operationID != command.GetLookAtFolder().GetOperationId() || got.commandID != command.GetId() {
				t.Error("the app changed the inspection start identifiers")
			}
		default:
			t.Error("the app did not forward the inspection start callback")
		}
		select {
		case number := <-appends:
			if number == 0 {
				t.Error("the app reported a zero durable folder event number")
			}
		default:
			t.Error("the app did not forward the durable append callback")
		}
		if len(starts) != 0 || len(appends) != 0 {
			t.Error("the app called a folder callback more than once")
		}
	})

	t.Run("refusal matches the command diagnostic", func(t *testing.T) {
		p.refuse.Store(true)
		log.Reset()
		refused := opts
		refused.StateDir = filepath.Join(home, "refused")
		err := clienttest.Join(ctx, refused, server.URL, token)
		if err == nil {
			t.Error("Join accepted an expired token")
		} else {
			assertClienttestRedaction(t, p, token, err.Error())
		}
		assertClienttestRedaction(t, p, token, log.String())
		decoder := json.NewDecoder(bytes.NewReader(log.Bytes()))
		refusals := 0
		for decoder.More() {
			var line struct{ Msg, Code, Message string }
			if decoder.Decode(&line) != nil {
				t.Fatal("a client diagnostic is not valid JSON")
			}
			if line.Msg == "join_refused" && line.Code == "token_expired" && line.Message != "" {
				refusals++
			}
		}
		if refusals != 1 {
			t.Errorf("got %d join_refused/token_expired records, want one", refusals)
		}
	})
}

func assertClienttestRedaction(t *testing.T, p *testClientPeer, token, text string) {
	t.Helper()
	if strings.Contains(text, token) || strings.Contains(text, folderConnectionToken) {
		t.Error("the in-process entry exposed the token")
	}
	for _, value := range [][]byte{p.secret, p.private, p.private.Seed()} {
		if strings.Contains(text, string(value)) || strings.Contains(text, base64.StdEncoding.EncodeToString(value)) {
			t.Error("the in-process entry exposed fixture key or secret material")
		}
	}
}
