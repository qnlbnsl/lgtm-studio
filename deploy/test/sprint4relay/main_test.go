package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	fixtureMachine = "018bcfe5-6800-7000-8000-000000000001"
	fixtureRun     = "018bcfe5-6800-7000-8000-000000000002"
	fixtureCommand = "018bcfe5-6800-7000-8000-000000000003"
	otherCommand   = "018bcfe5-6800-7000-8000-000000000004"
	fixtureTokenID = "018bcfe5-6800-7000-8000-000000000005"
	processMarker  = "--sprint4relay-test-process"
	waitLimit      = 10 * time.Second
)

type wireStream = connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]

type frameSelector struct {
	MachineID   string `json:"machine_id"`
	Direction   string `json:"direction"`
	Frame       string `json:"frame"`
	EventNumber uint64 `json:"event_number,omitempty"`
	CommandID   string `json:"command_id,omitempty"`
	Attempt     uint32 `json:"attempt,omitempty"`
}

type observation struct {
	frameSelector
	StreamID   json.RawMessage `json:"stream_id"`
	Generation uint64          `json:"generation"`
	Action     string          `json:"action"`
	Digest     string          `json:"digest"`
	Outcome    json.RawMessage `json:"outcome"`
}

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == processMarker {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		main()
		return
	}
	os.Exit(m.Run())
}

func TestSprintFourRelayPreservesProofAndSelectsFrames(t *testing.T) {
	t.Run("SelectsFrames", func(t *testing.T) {
		upstream := newRelayPeer(t, false)
		client, controlDir := startRelay(t, upstream.address, nil)
		serverKey := checkJoin(t, client, upstream, false)

		old, cancelOld := openLink(t, t.Context(), client, upstream, serverKey, 1)
		expectAttempt(t, old, 1)
		oldStream := waitObservation(t, controlDir, func(o observation) bool {
			return o.Action == "forwarded" && o.frameSelector == commandSelector(1)
		}).StreamID
		requireStreamID(t, oldStream)

		invalid := []frameSelector{
			eventSelector(0),
			{MachineID: fixtureMachine, Direction: "up", Frame: "command_result", Attempt: 1},
			resultSelector(0),
			{Direction: "up", Frame: "event", EventNumber: 1},
			{MachineID: fixtureMachine, Direction: "wrong", Frame: "event", EventNumber: 1},
			{MachineID: fixtureMachine, Direction: "up", Frame: "wrong", EventNumber: 1},
		}
		for i, selector := range invalid {
			number := uint64(i + 1)
			replaceControl(t, controlDir, oldStream, number, eventSelector(number), selector)
			sendFrame(t, old, eventFrame(number))
			expectAck(t, old, number, "A malformed arm blocked the valid rule's event.")
		}

		const generation = 20
		const heldNumber = 7
		eventRule, resultRule := eventSelector(heldNumber), resultSelector(1)
		replaceControl(t, controlDir, oldStream, generation, eventRule, resultRule)
		for _, rule := range []frameSelector{eventRule, resultRule} {
			waitRule(t, controlDir, oldStream, generation, "armed", rule)
		}
		replaceControl(t, controlDir, oldStream, generation, eventRule, resultRule)

		heldEvent := eventFrame(heldNumber)
		sendFrame(t, old, heldEvent)
		sendFrame(t, old, resultFrame(fixtureCommand, 1))
		sendFrame(t, old, eventFrame(heldNumber+1))
		sendFrame(t, old, resultFrame(fixtureCommand, 2))
		sendFrame(t, old, resultFrame(otherCommand, 1))
		heartbeatBarrier(t, old)
		held := waitRule(t, controlDir, oldStream, generation, "held", eventRule)
		requireEventDigest(t, held, heldEvent.GetEvent())
		requireDone(t, waitRule(t, controlDir, oldStream, generation, "held", resultRule))
		oldTraffic := []string{"event:1", "event:2", "event:3", "event:4", "event:5", "event:6",
			"result:scan:2", "result:other:1", "heartbeat"}
		upstream.requireTraffic(t, 1, oldTraffic...)

		cancelOld()
		_ = old.CloseRequest()
		_ = old.CloseResponse()
		for _, rule := range []frameSelector{eventRule, resultRule} {
			waitRule(t, controlDir, oldStream, generation, "discarded", rule)
		}
		waitObservation(t, controlDir, func(o observation) bool {
			return o.Action == "closed" && o.Generation == generation && bytes.Equal(o.StreamID, oldStream)
		})
		upstream.waitOldClosed(t)
		upstream.requireTraffic(t, 1, oldTraffic...)

		next, _ := openLink(t, t.Context(), client, upstream, serverKey, 2)
		// An old copy would reach the upstream before the client sends the replay.
		heartbeatBarrier(t, next)
		upstream.requireTraffic(t, 2, "heartbeat")
		sendFrame(t, next, heldEvent)
		expectAck(t, next, heldNumber, "The replacement stream did not forward the fresh replay.")
		replay := waitObservation(t, controlDir, func(o observation) bool {
			return o.Action == "forwarded" && o.frameSelector == eventRule
		})
		nextStream := replay.StreamID
		requireStreamID(t, nextStream)
		if bytes.Equal(nextStream, oldStream) {
			t.Fatal("The replacement stream reused the retired stream identifier.")
		}
		requireEventDigest(t, replay, heldEvent.GetEvent())

		// Even control rewritten to name this stream cannot reuse a retired generation.
		replayedResult := resultSelector(2)
		replaceControl(t, controlDir, nextStream, generation, eventSelector(heldNumber+1), replayedResult)
		sendFrame(t, next, eventFrame(heldNumber+1))
		expectAck(t, next, heldNumber+1, "The retired generation blocked the replay's following event.")
		expectAttempt(t, next, 2)
		sendFrame(t, next, resultFrame(fixtureCommand, 2))
		heartbeatBarrier(t, next)
		upstream.requireTraffic(t, 2, "heartbeat", "event:7", "event:8", "result:scan:2", "heartbeat")

		requireDone(t, waitObservation(t, controlDir, func(o observation) bool {
			return o.Action == "forwarded" && bytes.Equal(o.StreamID, nextStream) && o.frameSelector == replayedResult
		}))
		requireRetirement(t, observations(t, controlDir), oldStream, nextStream, generation, eventRule, resultRule)
	})

	t.Run("DiscardsOnConnectionClose", func(t *testing.T) {
		upstream := newRelayPeer(t, false)
		client, controlDir := startRelay(t, upstream.address, nil)
		serverKey := checkJoin(t, client, upstream, false)

		var socket struct {
			sync.Mutex
			conn net.Conn
		}
		ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) {
				socket.Lock()
				socket.conn = info.Conn
				socket.Unlock()
			},
		})
		old, _ := openLink(t, ctx, client, upstream, serverKey, 1)
		t.Cleanup(func() {
			_ = old.CloseRequest()
			_ = old.CloseResponse()
		})
		expectAttempt(t, old, 1)
		oldStream := waitObservation(t, controlDir, func(o observation) bool {
			return o.Action == "forwarded" && o.frameSelector == commandSelector(1)
		}).StreamID
		requireStreamID(t, oldStream)

		const generation = 20
		const heldNumber = 7
		eventRule, resultRule := eventSelector(heldNumber), resultSelector(1)
		replaceControl(t, controlDir, oldStream, generation, eventRule, resultRule)
		for _, rule := range []frameSelector{eventRule, resultRule} {
			waitRule(t, controlDir, oldStream, generation, "armed", rule)
		}
		sendFrame(t, old, eventFrame(heldNumber))
		sendFrame(t, old, resultFrame(fixtureCommand, 1))
		heartbeatBarrier(t, old)
		for _, rule := range []frameSelector{eventRule, resultRule} {
			waitRule(t, controlDir, oldStream, generation, "held", rule)
		}

		// Close the active socket while the stream context and both halves are still open.
		socket.Lock()
		conn := socket.conn
		socket.Unlock()
		if conn == nil {
			t.Fatal("Could not identify the connection carrying the old stream.")
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("Could not close the old connection: %v", err)
		}
		for _, rule := range []frameSelector{eventRule, resultRule} {
			waitRule(t, controlDir, oldStream, generation, "discarded", rule)
		}
		waitObservation(t, controlDir, func(o observation) bool {
			return o.Action == "closed" && o.Generation == generation && bytes.Equal(o.StreamID, oldStream)
		})
		upstream.waitOldClosed(t)
		upstream.requireTraffic(t, 1, "heartbeat")
	})

	t.Run("PreservesRefusals", func(t *testing.T) {
		upstream := newRelayPeer(t, true)
		client, _ := startRelay(t, upstream.address, nil)
		serverKey := checkJoin(t, client, upstream, true)
		ctx, cancel := context.WithTimeout(t.Context(), waitLimit)
		defer cancel()
		stream := client.Open(ctx)
		sendHelloAndVerifyChallenge(t, stream, serverKey, 1)
		reply, err := stream.Receive()
		if reply != nil {
			t.Fatal("The revision 1 refusal sent another frame after Challenge.")
		}
		requireRefusal(t, err, connect.CodeFailedPrecondition, "Lgtm-Link-Refusal", "link_revision_mismatch")
	})

	t.Run("TLSFlags", func(t *testing.T) {
		upstream := newRelayPeer(t, true)
		config, pair := fixtureTLS(t, upstream.serverKey)
		t.Run("Paired", func(t *testing.T) {
			client, _ := startRelay(t, upstream.address, config, pair...)
			checkJoin(t, client, upstream, false)
		})
		for _, flags := range [][]string{pair[:2], pair[2:]} {
			t.Run(strings.TrimPrefix(flags[0], "--"), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), waitLimit)
				defer cancel()
				child := relayCommand(t, ctx, "127.0.0.1:0", upstream.address, t.TempDir(), flags...)
				var exit *exec.ExitError
				if err := child.Run(); !errors.As(err, &exit) || exit.ExitCode() <= 0 {
					t.Fatal("The relay did not reject a one-sided certificate or key flag.")
				}
			})
		}
	})
}

func relayCommand(t *testing.T, ctx context.Context, address, upstream, controlDir string, flags ...string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal("Could not locate the relay test binary.")
	}
	args := []string{processMarker, "--listen", address, "--upstream", upstream, "--control-dir", controlDir}
	return exec.CommandContext(ctx, executable, append(args, flags...)...)
}

func startRelay(t *testing.T, upstream string, tlsConfig *tls.Config, flags ...string) (linkv1connect.LinkServiceClient, string) {
	t.Helper()
	controlDir := t.TempDir()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("Could not reserve a private relay address.")
	}
	address := listener.Addr().String()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil || port == 8787 {
		t.Fatal("Could not select a nondefault relay port.")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cmd := relayCommand(t, ctx, address, upstream, controlDir, flags...)
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		cancel()
		t.Fatal("Could not start the private relay process.")
	}
	ready, done := make(chan struct{}, 1), make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "relay ready" || strings.HasPrefix(line, "relay ready ") {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		}
		_ = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	select {
	case <-done:
		t.Fatal("The private relay exited before announcing that its listener was ready.")
	case <-time.After(waitLimit):
		t.Fatal("The private relay did not announce that its listener was ready.")
	case <-ready:
		select {
		case <-done:
			t.Fatal("The private relay exited after announcing readiness.")
		default:
		}
	}
	protocols := new(http.Protocols)
	scheme := "http"
	if tlsConfig == nil {
		protocols.SetUnencryptedHTTP2(true)
	} else {
		protocols.SetHTTP2(true)
		scheme = "https"
	}
	transport := &http.Transport{Protocols: protocols, TLSClientConfig: tlsConfig}
	t.Cleanup(transport.CloseIdleConnections)
	return linkv1connect.NewLinkServiceClient(&http.Client{Transport: transport}, scheme+"://"+address), controlDir
}

func fixtureTLS(t *testing.T, key ed25519.PrivateKey) (*tls.Config, []string) {
	t.Helper()
	clock := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: clock.Add(-time.Hour), NotAfter: clock.Add(time.Hour),
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, key.Public(), key)
	if err != nil {
		t.Fatal("Could not create the relay fixture certificate.")
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal("Could not encode the relay fixture certificate key.")
	}
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	privateKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey})
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "leaf.pem"), filepath.Join(dir, "leaf-key.pem")
	if os.WriteFile(certPath, certificate, 0600) != nil || os.WriteFile(keyPath, privateKey, 0600) != nil {
		t.Fatal("Could not write the relay fixture certificate pair.")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificate) {
		t.Fatal("Could not trust the relay fixture certificate.")
	}
	return &tls.Config{RootCAs: roots, Time: func() time.Time { return clock }}, []string{"--tls-cert", certPath, "--tls-key", keyPath}
}

type relayPeer struct {
	address       string
	serverKey     ed25519.PrivateKey
	machineKey    ed25519.PrivateKey
	tokenSecret   []byte
	refusesLegacy bool
	oldClosed     chan struct{}

	mu      sync.Mutex
	traffic map[uint32][]string
}

func newRelayPeer(t *testing.T, refusesLegacy bool) *relayPeer {
	t.Helper()
	_, serverKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("Could not generate the fixture server's signing key.")
	}
	_, machineKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("Could not generate the fixture machine's signing key.")
	}
	tokenSecret := make([]byte, 32)
	if _, err := rand.Read(tokenSecret); err != nil {
		t.Fatal("Could not generate the fixture enrollment secret.")
	}
	p := &relayPeer{
		serverKey: serverKey, machineKey: machineKey, tokenSecret: tokenSecret, refusesLegacy: refusesLegacy,
		oldClosed: make(chan struct{}, 1), traffic: make(map[uint32][]string),
	}
	_, handler := linkv1connect.NewLinkServiceHandler(p)
	server := httptest.NewUnstartedServer(handler)
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	t.Cleanup(server.Close)
	if server.Listener.Addr().(*net.TCPAddr).Port == 8787 {
		t.Fatal("The fixture peer selected the reserved port.")
	}
	p.address = server.URL
	return p
}

func (p *relayPeer) Join(_ context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	if req.Msg.GetTokenId() != fixtureTokenID || !bytes.Equal(req.Msg.GetTokenSecret(), p.tokenSecret) ||
		!bytes.Equal(req.Msg.GetMachinePublicKey(), p.machineKey.Public().(ed25519.PublicKey)) ||
		req.Msg.GetHostName() != "relay-fixture" {
		refusal := connect.NewError(connect.CodePermissionDenied, errors.New("The fixture enrollment was refused."))
		refusal.Meta().Set("Lgtm-Join-Refusal", "token_unknown")
		return nil, refusal
	}
	return connect.NewResponse(&linkv1.JoinResponse{
		MachineId: fixtureMachine, ServerPublicKey: p.serverKey.Public().(ed25519.PublicKey),
	}), nil
}

func (p *relayPeer) Open(_ context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	first, err := stream.Receive()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	revision := hello.GetLinkRevision()
	if hello.GetMachineId() != fixtureMachine || len(hello.GetMachineNonce()) != 32 ||
		(revision != 1 && revision != 2) ||
		(revision == 2 && (hello.GetRunId() != fixtureRun || hello.GetFirstEventNumber() != 9)) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("The fixture received an invalid Hello."))
	}
	serverNonce := make([]byte, 32)
	if _, err := rand.Read(serverNonce); err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("The fixture could not generate a nonce."))
	}
	challenge := &linkv1.Challenge{ServerNonce: serverNonce,
		ServerSignature: ed25519.Sign(p.serverKey, proofText("server", hello.GetMachineNonce(), serverNonce))}
	if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: challenge}}); err != nil {
		return err
	}
	if revision == 1 && p.refusesLegacy {
		stream.ResponseTrailer().Set("Lgtm-Link-Refusal", "link_revision_mismatch")
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("The fixture speaks revision 2."))
	}
	if revision == 1 {
		defer func() {
			select {
			case p.oldClosed <- struct{}{}:
			default:
			}
		}()
	}
	second, err := stream.Receive()
	if err != nil {
		return err
	}
	if !ed25519.Verify(p.machineKey.Public().(ed25519.PublicKey),
		proofText("machine", hello.GetMachineNonce(), serverNonce), second.GetProof().GetMachineSignature()) {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("The fixture could not verify Proof."))
	}

	fixedTime := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	welcome := &linkv1.Welcome{
		LinkRevision: revision, ConnectionToken: "fixture-only-connection-token",
		ServerTime: timestamppb.New(fixedTime),
		State:      &linkv1.MachineState{Revision: 1, Approval: linkv1.Approval_APPROVAL_APPROVED},
	}
	if revision == 2 {
		welcome.LastEventNumber = 6
	}
	if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: welcome}}); err != nil {
		return err
	}
	cmd := &linkv1.Command{
		Id: fixtureCommand, Attempt: revision, ExpiresAt: timestamppb.New(fixedTime.Add(time.Minute)),
		Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}},
	}
	command := &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: cmd}}
	if revision == 1 {
		if err := stream.Send(command); err != nil {
			return err
		}
	}
	candidateCommandSent := false
	for {
		frame, err := stream.Receive()
		if err != nil {
			return err
		}
		switch {
		case frame.GetEvent() != nil:
			number := frame.GetEvent().GetNumber()
			if !proto.Equal(frame, eventFrame(number)) {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("The fixture event payload changed in transit."))
			}
			p.record(revision, fmt.Sprintf("event:%d", number))
			ack := &linkv1.EventAck{Through: number}
			err = stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{EventAck: ack}})
			if err == nil && revision == 2 && number+1 == hello.GetFirstEventNumber() && !candidateCommandSent {
				err = stream.Send(command)
				candidateCommandSent = true
			}
		case frame.GetCommandResult() != nil:
			result := frame.GetCommandResult()
			if !proto.Equal(frame, resultFrame(result.GetCommandId(), result.GetAttempt())) {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("The fixture result payload changed in transit."))
			}
			command := "unexpected"
			switch result.GetCommandId() {
			case fixtureCommand:
				command = "scan"
			case otherCommand:
				command = "other"
			}
			p.record(revision, fmt.Sprintf("result:%s:%d", command, result.GetAttempt()))
		case frame.GetHeartbeat() != nil:
			p.record(revision, "heartbeat")
			ack := &linkv1.HeartbeatAck{}
			err = stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: ack}})
		default:
			return connect.NewError(connect.CodeInvalidArgument, errors.New("The fixture received an unexpected frame."))
		}
		if err != nil {
			return err
		}
	}
}

func (p *relayPeer) record(revision uint32, frame string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.traffic[revision] = append(p.traffic[revision], frame)
}

func (p *relayPeer) requireTraffic(t *testing.T, revision uint32, want ...string) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if got := p.traffic[revision]; !slices.Equal(got, want) {
		t.Fatalf("The revision %d upstream saw %v. Expected %v.", revision, got, want)
	}
}

func (p *relayPeer) waitOldClosed(t *testing.T) {
	t.Helper()
	select {
	case <-p.oldClosed:
	case <-time.After(waitLimit):
		t.Fatal("Canceling the old stream did not close its upstream.")
	}
}

func checkJoin(t *testing.T, client linkv1connect.LinkServiceClient, p *relayPeer, checkRefusal bool) ed25519.PublicKey {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), waitLimit)
	defer cancel()
	req := &linkv1.JoinRequest{TokenId: fixtureTokenID, TokenSecret: p.tokenSecret,
		MachinePublicKey: p.machineKey.Public().(ed25519.PublicKey), HostName: "relay-fixture"}
	resp, err := client.Join(ctx, connect.NewRequest(req))
	if err != nil || resp.Msg.GetMachineId() != fixtureMachine ||
		!bytes.Equal(resp.Msg.GetServerPublicKey(), p.serverKey.Public().(ed25519.PublicKey)) {
		t.Fatal("Join did not return the fixture identity and server proof key.")
	}
	if checkRefusal {
		bad := proto.CloneOf(req)
		bad.TokenSecret = make([]byte, 32)
		_, err := client.Join(ctx, connect.NewRequest(bad))
		requireRefusal(t, err, connect.CodePermissionDenied, "Lgtm-Join-Refusal", "token_unknown")
	}
	return resp.Msg.GetServerPublicKey()
}

func requireRefusal(t *testing.T, err error, code connect.Code, header, reason string) {
	t.Helper()
	var refusal *connect.Error
	if !errors.As(err, &refusal) || refusal.Code() != code {
		t.Fatalf("The relay did not preserve the %s refusal code.", code)
	}
	if !slices.Equal(refusal.Meta().Values(header), []string{reason}) {
		t.Fatalf("The relay did not preserve exactly one %s refusal reason.", header)
	}
}

func proofText(role string, machineNonce, serverNonce []byte) []byte {
	text := append([]byte("lgtm-link-v1 "+role+"\x00"), byte(len(fixtureMachine)))
	text = append(text, fixtureMachine...)
	text = append(text, machineNonce...)
	return append(text, serverNonce...)
}

func sendHelloAndVerifyChallenge(t *testing.T, stream *wireStream, serverKey ed25519.PublicKey, revision uint32) ([]byte, []byte) {
	t.Helper()
	machineNonce := make([]byte, 32)
	if _, err := rand.Read(machineNonce); err != nil {
		t.Fatal("Could not generate the fixture machine's nonce.")
	}
	hello := &linkv1.Hello{MachineId: fixtureMachine, MachineNonce: machineNonce, LinkRevision: revision}
	if revision == 2 {
		hello.RunId = fixtureRun
		hello.FirstEventNumber = 9
	}
	sendFrame(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Hello{Hello: hello}})
	frame, err := stream.Receive()
	challenge := frame.GetChallenge()
	serverNonce := challenge.GetServerNonce()
	if err != nil || len(serverNonce) != 32 ||
		!ed25519.Verify(serverKey, proofText("server", machineNonce, serverNonce), challenge.GetServerSignature()) {
		t.Fatal("The relay did not preserve a verifiable Challenge.")
	}
	return machineNonce, serverNonce
}

func openLink(t *testing.T, parent context.Context, client linkv1connect.LinkServiceClient, p *relayPeer, serverKey ed25519.PublicKey, revision uint32) (*wireStream, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 3*waitLimit)
	t.Cleanup(cancel)
	stream := client.Open(ctx)
	machineNonce, serverNonce := sendHelloAndVerifyChallenge(t, stream, serverKey, revision)
	proof := &linkv1.Proof{MachineSignature: ed25519.Sign(p.machineKey, proofText("machine", machineNonce, serverNonce))}
	sendFrame(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{Proof: proof}})
	frame, err := stream.Receive()
	welcome := frame.GetWelcome()
	if err != nil || welcome.GetLinkRevision() != revision ||
		welcome.GetConnectionToken() != "fixture-only-connection-token" {
		t.Fatal("The upstream did not verify machine Proof and return Welcome intact.")
	}
	return stream, cancel
}

func sendFrame(t *testing.T, stream *wireStream, frame *linkv1.OpenRequest) {
	t.Helper()
	if stream.Send(frame) != nil {
		t.Fatal("Could not send a fixture frame through the relay.")
	}
}

func expectAttempt(t *testing.T, stream *wireStream, attempt uint32) {
	t.Helper()
	frame, err := stream.Receive()
	cmd := frame.GetCommand()
	if err != nil || cmd.GetId() != fixtureCommand || cmd.GetAttempt() != attempt || cmd.GetScanHarnesses() == nil {
		t.Fatal("The relay did not forward the fixture command and attempt.")
	}
}

func heartbeatBarrier(t *testing.T, stream *wireStream) {
	t.Helper()
	sendFrame(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}})
	frame, err := stream.Receive()
	if err != nil || frame.GetHeartbeatAck() == nil {
		t.Fatal("Unrelated heartbeat traffic could not cross in both directions.")
	}
}

func expectAck(t *testing.T, stream *wireStream, number uint64, failure string) {
	t.Helper()
	frame, err := stream.Receive()
	if err != nil || frame.GetEventAck().GetThrough() != number {
		t.Fatal(failure)
	}
}

func eventFrame(number uint64) *linkv1.OpenRequest {
	event := &linkv1.Event{Number: number}
	if number == 7 {
		event.Report = &linkv1.Event_HarnessesFound{HarnessesFound: &linkv1.HarnessesFound{
			Harnesses: []*linkv1.Harness{{Name: "codex"}},
		}}
	} else {
		event.Report = &linkv1.Event_Facts{Facts: &linkv1.Facts{Os: "Linux", Cores: 2}}
	}
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}}
}

func resultFrame(command string, attempt uint32) *linkv1.OpenRequest {
	result := &linkv1.CommandResult{
		CommandId: command, Attempt: attempt, Outcome: linkv1.CommandOutcome_COMMAND_OUTCOME_DONE,
	}
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: result}}
}

func eventSelector(n uint64) frameSelector {
	return frameSelector{MachineID: fixtureMachine, Direction: "up", Frame: "event", EventNumber: n}
}

func resultSelector(attempt uint32) frameSelector {
	return frameSelector{MachineID: fixtureMachine, Direction: "up", Frame: "command_result", CommandID: fixtureCommand, Attempt: attempt}
}

func commandSelector(attempt uint32) frameSelector {
	return frameSelector{MachineID: fixtureMachine, Direction: "down", Frame: "command", CommandID: fixtureCommand, Attempt: attempt}
}

func replaceControl(t *testing.T, dir string, stream json.RawMessage, generation uint64, rules ...frameSelector) {
	t.Helper()
	record := struct {
		Generation uint64          `json:"generation"`
		StreamID   json.RawMessage `json:"stream_id"`
		Action     string          `json:"action"`
		Rules      []frameSelector `json:"rules"`
	}{generation, stream, "arm", rules}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal("Could not encode the relay control.")
	}
	tmp := filepath.Join(dir, "control.next")
	if os.WriteFile(tmp, data, 0600) != nil || os.Rename(tmp, filepath.Join(dir, "control.json")) != nil {
		t.Fatal("Could not atomically replace the relay control.")
	}
}

func observations(t *testing.T, dir string) []observation {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "observations.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal("Could not read private relay observations.")
	}
	var records []observation
	for {
		line, remaining, complete := bytes.Cut(data, []byte{'\n'})
		if !complete {
			return records
		}
		var record observation
		if json.Unmarshal(line, &record) != nil {
			t.Fatal("The relay wrote a malformed observation record.")
		}
		records = append(records, record)
		data = remaining
	}
}

func waitObservation(t *testing.T, dir string, matches func(observation) bool) observation {
	t.Helper()
	timer := time.NewTimer(waitLimit)
	defer timer.Stop()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		for _, record := range observations(t, dir) {
			if matches(record) {
				return record
			}
		}
		select {
		case <-timer.C:
			t.Fatal("The relay did not write the required observation.")
		case <-poll.C:
		}
	}
}

func waitRule(t *testing.T, dir string, stream json.RawMessage, generation uint64, action string, rule frameSelector) observation {
	t.Helper()
	t.Logf("Waiting for the %s observation for %s.", action, rule.Frame)
	return waitObservation(t, dir, func(o observation) bool {
		return o.Action == action && o.Generation == generation && bytes.Equal(o.StreamID, stream) && o.frameSelector == rule
	})
}

func requireStreamID(t *testing.T, id json.RawMessage) {
	t.Helper()
	var value any
	if json.Unmarshal(id, &value) == nil {
		switch value := value.(type) {
		case string:
			if value != "" {
				return
			}
		case float64:
			if value > 0 {
				return
			}
		}
	}
	t.Fatal("The relay did not assign a usable stream identifier.")
}

func requireEventDigest(t *testing.T, record observation, event *linkv1.Event) {
	t.Helper()
	copy := proto.CloneOf(event)
	copy.Number = 0
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(copy)
	digest := sha256.Sum256(encoded)
	if err != nil || !strings.EqualFold(record.Digest, hex.EncodeToString(digest[:])) {
		t.Fatal("The relay did not record the digest of the unnumbered event payload.")
	}
}

func requireDone(t *testing.T, record observation) {
	t.Helper()
	var label string
	var number int
	if json.Unmarshal(record.Outcome, &label) == nil {
		label = strings.TrimPrefix(strings.ToLower(label), "command_outcome_")
		if label == "done" {
			return
		}
	}
	if json.Unmarshal(record.Outcome, &number) == nil && number == int(linkv1.CommandOutcome_COMMAND_OUTCOME_DONE) {
		return
	}
	t.Fatal("The result observation did not preserve the done outcome.")
}

func requireRetirement(t *testing.T, records []observation, old, next json.RawMessage, generation uint64, rules ...frameSelector) {
	t.Helper()
	for _, rule := range rules {
		var actions []string
		for _, o := range records {
			if o.Action == "armed" && o.Generation != generation {
				t.Fatal("The relay armed a rule from a malformed control.")
			}
			if bytes.Equal(o.StreamID, next) && (o.Action == "armed" || o.Action == "held") {
				t.Fatal("The replacement stream accepted a retired generation.")
			}
			if bytes.Equal(o.StreamID, old) && o.frameSelector == rule {
				if o.Generation != generation {
					t.Fatal("The relay used the wrong generation for an old hold.")
				}
				actions = append(actions, o.Action)
			}
		}
		if !slices.Equal(actions, []string{"armed", "held", "discarded"}) {
			t.Fatalf("The relay did not arm, hold and discard the %s rule exactly once and in order.", rule.Frame)
		}
	}
}
