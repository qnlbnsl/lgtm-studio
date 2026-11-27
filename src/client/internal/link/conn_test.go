package link

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/transport"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

// openCeiling is a ceiling for one call, so that a broken client fails a test and does not
// hang it. It is never a delay that a test waits for.
const openCeiling = 20 * time.Second

const (
	serverLabel  = "lgtm-link-v1 server"
	machineLabel = "lgtm-link-v1 machine"
)

// proofFixture is src/contracts/testdata/link-proof.json.
type proofFixture struct {
	ServerSeed       string `json:"server_seed"`
	ServerPublicKey  string `json:"server_public_key"`
	MachineSeed      string `json:"machine_seed"`
	MachinePublicKey string `json:"machine_public_key"`
	MachineID        string `json:"machine_id"`
	MachineNonce     string `json:"machine_nonce"`
	ServerNonce      string `json:"server_nonce"`
	ServerText       string `json:"server_text"`
	ServerSignature  string `json:"server_signature"`
	MachineText      string `json:"machine_text"`
	MachineSignature string `json:"machine_signature"`
}

func loadProofFixture(t *testing.T) (f proofFixture, raw map[string][]byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "testdata", "link-proof.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	var texts map[string]string
	if err := json.Unmarshal(data, &texts); err != nil {
		t.Fatal(err)
	}
	raw = map[string][]byte{}
	for k, v := range texts {
		if k == "machine_id" {
			continue
		}
		b, err := hex.DecodeString(v)
		if err != nil {
			t.Fatalf("fixture field %s: %v", k, err)
		}
		raw[k] = b
	}
	return f, raw
}

// keys of the fixture.
type fixtureKeys struct {
	serverKey  ed25519.PrivateKey
	machineKey ed25519.PrivateKey
	machineID  string
}

func fixtureKeysOf(t *testing.T) fixtureKeys {
	t.Helper()
	f, raw := loadProofFixture(t)
	k := fixtureKeys{
		serverKey:  ed25519.NewKeyFromSeed(raw["server_seed"]),
		machineKey: ed25519.NewKeyFromSeed(raw["machine_seed"]),
		machineID:  f.MachineID,
	}
	if !bytes.Equal(k.serverKey.Public().(ed25519.PublicKey), raw["server_public_key"]) ||
		!bytes.Equal(k.machineKey.Public().(ed25519.PublicKey), raw["machine_public_key"]) {
		t.Fatal("the fixture's seeds and public keys disagree")
	}
	return k
}

// credentialOf writes the key to a private file and loads it as the credential of the machine.
func credentialOf(t *testing.T, key ed25519.PrivateKey) *enroll.Credential {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "machine.key")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := enroll.Load(path)
	if err != nil {
		t.Fatalf("enroll.Load: %v", err)
	}
	return c
}

// frames is what the fake server saw.
type frames struct {
	mu     sync.Mutex
	hellos []*linkv1.Hello
	proofs []*linkv1.Proof
	others int
}

func (f *frames) record(req *linkv1.OpenRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch k := req.GetKind().(type) {
	case *linkv1.OpenRequest_Hello:
		f.hellos = append(f.hellos, k.Hello)
	case *linkv1.OpenRequest_Proof:
		f.proofs = append(f.proofs, k.Proof)
	default:
		f.others++
	}
}

func (f *frames) counts() (hellos, proofs, others int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.hellos), len(f.proofs), f.others
}

// scene is the server side of one test: a script that runs for each stream.
type scene struct {
	linkv1connect.UnimplementedLinkServiceHandler
	keys   fixtureKeys
	script func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error
	seen   frames
	done   chan struct{}
}

func (s *scene) Open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	defer func() {
		select {
		case s.done <- struct{}{}:
		default:
		}
	}()
	return s.script(s, stream)
}

// readHello receives the first frame, which must be a Hello.
func (s *scene) readHello(stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) (*linkv1.Hello, error) {
	req, err := stream.Receive()
	if err != nil {
		return nil, err
	}
	s.seen.record(req)
	if req.GetHello() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("not a hello"))
	}
	return req.GetHello(), nil
}

// readProof receives the next frame, which should be a Proof, and returns what it got.
func (s *scene) readProof(stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) (*linkv1.Proof, error) {
	req, err := stream.Receive()
	if err != nil {
		return nil, err
	}
	s.seen.record(req)
	return req.GetProof(), nil
}

func (s *scene) serverSignature(hello *linkv1.Hello, serverNonce []byte) []byte {
	return ed25519.Sign(s.keys.serverKey, proofText(serverLabel, s.keys.machineID, hello.GetMachineNonce(), serverNonce))
}

func (s *scene) challenge(hello *linkv1.Hello, serverNonce []byte) *linkv1.OpenResponse {
	return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{
		ServerNonce: serverNonce, ServerSignature: s.serverSignature(hello, serverNonce),
	}}}
}

func (s *scene) verifiesProof(hello *linkv1.Hello, serverNonce []byte, proof *linkv1.Proof) bool {
	if proof == nil || len(proof.GetMachineSignature()) != ed25519.SignatureSize {
		return false
	}
	text := proofText(machineLabel, s.keys.machineID, hello.GetMachineNonce(), serverNonce)
	return ed25519.Verify(s.keys.machineKey.Public().(ed25519.PublicKey), text, proof.GetMachineSignature())
}

var fixedServerNonce = bytes.Repeat([]byte{0x22}, 32)

func welcomeFrame(w *linkv1.Welcome) *linkv1.OpenResponse {
	return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: w}}
}

var fixtureRunIdentity = runIdentity{id: "019ac47a-cc00-7000-8000-000000000001", firstEventNumber: 8}

func goodWelcome() *linkv1.Welcome {
	return &linkv1.Welcome{
		ConnectionToken: "fixture-approved-token",
		State:           &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_APPROVED, Name: "build-1", Revision: 3},
		LastEventNumber: 7,
		ServerTime:      timestamppb.New(time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)),
		LinkRevision:    linkRevision,
	}
}

// honest is a server that follows the protocol and answers with the given welcome.
func honest(w *linkv1.Welcome) func(*scene, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	return func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		hello, err := s.readHello(stream)
		if err != nil {
			return err
		}
		if err := stream.Send(s.challenge(hello, fixedServerNonce)); err != nil {
			return err
		}
		proof, err := s.readProof(stream)
		if err != nil {
			return err
		}
		if !s.verifiesProof(hello, fixedServerNonce, proof) {
			return connect.NewError(connect.CodeUnauthenticated, errors.New("bad proof"))
		}
		return stream.Send(welcomeFrame(w))
	}
}

type protocol struct {
	name string
	opts []connect.ClientOption
}

var protocols = []protocol{
	{"connect", nil},
	{"grpc", []connect.ClientOption{connect.WithGRPC()}},
}

// startScene serves the scene on a loopback listener over cleartext HTTP/2 and returns a
// client that is built the way the product builds it.
func startScene(t *testing.T, p protocol, script func(*scene, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error) (*scene, linkv1connect.LinkServiceClient, enroll.Server, *enroll.Credential) {
	t.Helper()
	keys := fixtureKeysOf(t)
	s := &scene{keys: keys, script: script, done: make(chan struct{}, 8)}
	mux := http.NewServeMux()
	mux.Handle(linkv1connect.NewLinkServiceHandler(s))
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on loopback: %v", err)
	}
	srv := httptest.NewUnstartedServer(mux)
	_ = srv.Listener.Close()
	srv.Listener = l
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	t.Cleanup(srv.Close)

	httpClient, err := transport.NewHTTPClient(srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(httpClient.CloseIdleConnections)
	opts := append([]connect.ClientOption{connect.WithReadMaxBytes(64 << 10)}, p.opts...)
	client := linkv1connect.NewLinkServiceClient(httpClient, srv.URL, opts...)
	server := enroll.Server{
		Address: srv.URL, MachineID: keys.machineID,
		ClientPublicKey: keys.machineKey.Public().(ed25519.PublicKey),
		PublicKey:       keys.serverKey.Public().(ed25519.PublicKey),
	}
	return s, client, server, credentialOf(t, keys.machineKey)
}

func open(t *testing.T, client linkv1connect.LinkServiceClient, server enroll.Server, cred *enroll.Credential) (*session, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openCeiling)
	defer cancel()
	// The session outlives the ceiling of the call only through its own context, so keep it
	// alive by giving the session a context that the test cancels at its end.
	sessionCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	done := make(chan struct{})
	var s *session
	var err error
	go func() {
		defer close(done)
		s, err = openSession(sessionCtx, client, server, cred, fixtureRunIdentity)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		stop()
		t.Fatal("openSession did not return within the ceiling")
	}
	if s != nil {
		t.Cleanup(func() { _ = s.close() })
	}
	return s, err
}

func useShortHandshake(t *testing.T, d time.Duration) {
	t.Helper()
	old := handshakeTimeout
	handshakeTimeout = d
	t.Cleanup(func() { handshakeTimeout = old })
}

func isTerminal(err error) (string, bool) {
	var v terminalRefusal
	if errors.As(err, &v) {
		return v.Code, true
	}
	var p *terminalRefusal
	if errors.As(err, &p) && p != nil {
		return p.Code, true
	}
	return "", false
}

func wantRetryable(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: openSession succeeded", what)
	}
	if code, terminal := isTerminal(err); terminal {
		t.Fatalf("%s: the error is a final refusal %q, want a retryable error: %v", what, code, err)
	}
}

func wantTerminal(t *testing.T, err error, want, what string) {
	t.Helper()
	code, terminal := isTerminal(err)
	if !terminal || code != want {
		t.Fatalf("%s: error %v: final refusal %q (is final: %v), want %q", what, err, code, terminal, want)
	}
}

func TestOpenProofFixture(t *testing.T) {
	f, raw := loadProofFixture(t)
	keys := fixtureKeysOf(t)
	machine := credentialOf(t, keys.machineKey)

	// The golden texts and signatures of the fixture bind the text format and the signer.
	if got := proofText(serverLabel, f.MachineID, raw["machine_nonce"], raw["server_nonce"]); !bytes.Equal(got, raw["server_text"]) {
		t.Fatal("the server proof text differs from the contract fixture")
	}
	if got := proofText(machineLabel, f.MachineID, raw["machine_nonce"], raw["server_nonce"]); !bytes.Equal(got, raw["machine_text"]) {
		t.Fatal("the machine proof text differs from the contract fixture")
	}
	if !ed25519.Verify(keys.serverKey.Public().(ed25519.PublicKey), raw["server_text"], raw["server_signature"]) {
		t.Fatal("the fixture's server signature does not verify")
	}
	if got := machine.Sign(proofText(machineLabel, f.MachineID, raw["machine_nonce"], raw["server_nonce"])); !bytes.Equal(got, raw["machine_signature"]) {
		t.Fatal("the machine signature differs from the contract fixture")
	}
	// Swapped nonces give another text.
	if bytes.Equal(proofText(machineLabel, f.MachineID, raw["server_nonce"], raw["machine_nonce"]), raw["machine_text"]) {
		t.Fatal("proofText ignores the order of the nonces")
	}
	if bytes.Equal(proofText(serverLabel, f.MachineID, raw["machine_nonce"], raw["server_nonce"]),
		proofText(machineLabel, f.MachineID, raw["machine_nonce"], raw["server_nonce"])) {
		t.Fatal("proofText ignores the label")
	}

	for _, p := range protocols {
		t.Run(p.name, func(t *testing.T) {
			var order []string
			var mu sync.Mutex
			note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
			welcome := goodWelcome()
			welcome.LinkRevision = 2
			s, client, server, cred := startScene(t, p, func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
				hello, err := s.readHello(stream)
				if err != nil {
					return err
				}
				note("hello")
				if hello.GetMachineId() != f.MachineID || hello.GetLinkRevision() != 2 || len(hello.GetMachineNonce()) != 32 {
					return connect.NewError(connect.CodeInvalidArgument, errors.New("malformed hello"))
				}
				if hello.GetRunId() != fixtureRunIdentity.id || hello.GetFirstEventNumber() != fixtureRunIdentity.firstEventNumber {
					return connect.NewError(connect.CodeInvalidArgument, errors.New("wrong process announcement"))
				}
				if bytes.Equal(hello.GetMachineNonce(), make([]byte, 32)) {
					return connect.NewError(connect.CodeInvalidArgument, errors.New("zero nonce"))
				}
				if err := stream.Send(s.challenge(hello, raw["server_nonce"])); err != nil {
					return err
				}
				note("challenge")
				proof, err := s.readProof(stream)
				if err != nil {
					return err
				}
				note("proof")
				if !s.verifiesProof(hello, raw["server_nonce"], proof) {
					return connect.NewError(connect.CodeUnauthenticated, errors.New("bad proof"))
				}
				note("verified")
				if err := stream.Send(welcomeFrame(welcome)); err != nil {
					return err
				}
				note("welcome")
				// Keep the stream open until the client has sent one more frame.
				_, err = stream.Receive()
				return err
			})
			before, beforeErr := elapsedClock()
			sess, err := open(t, client, server, cred)
			after, afterErr := elapsedClock()
			if err != nil {
				t.Fatalf("openSession: %v", err)
			}
			mu.Lock()
			got := append([]string(nil), order...)
			mu.Unlock()
			want := []string{"hello", "challenge", "proof", "verified", "welcome"}
			if len(got) != len(want) {
				t.Fatalf("frame order %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("frame order %v, want %v", got, want)
				}
			}
			hellos, proofs, others := s.seen.counts()
			if hellos != 1 || proofs != 1 || others != 0 {
				t.Fatalf("the server saw %d hellos, %d proofs and %d other frames", hellos, proofs, others)
			}
			if !proto.Equal(sess.welcome, welcome) {
				t.Fatal("the session changed the approved welcome")
			}
			if sess.welcome.GetLastEventNumber() != 7 {
				t.Fatalf("the session lost the last event number")
			}
			if sess.welcome.GetConnectionToken() != welcome.GetConnectionToken() {
				t.Fatal("the approved session did not retain its token")
			}
			if beforeErr == nil && afterErr == nil {
				if !sess.sentAtOK || sess.sentAt < before || sess.sentAt > after {
					t.Fatalf("the proof was stamped at %v (ok %v), want between %v and %v", sess.sentAt, sess.sentAtOK, before, after)
				}
			}

			// The stream is usable after the proof.
			if err := sess.send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}}); err != nil {
				t.Fatalf("send after the proof: %v", err)
			}
			if err := sess.close(); err != nil && !errors.Is(err, context.Canceled) {
				t.Logf("close returned %v", err)
			}
		})
	}

	t.Run("nonces are fresh", func(t *testing.T) {
		var mu sync.Mutex
		var nonces [][]byte
		_, client, server, cred := startScene(t, protocols[0], func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
			hello, err := s.readHello(stream)
			if err != nil {
				return err
			}
			mu.Lock()
			nonces = append(nonces, bytes.Clone(hello.GetMachineNonce()))
			mu.Unlock()
			if err := stream.Send(s.challenge(hello, fixedServerNonce)); err != nil {
				return err
			}
			proof, err := s.readProof(stream)
			if err != nil {
				return err
			}
			if !s.verifiesProof(hello, fixedServerNonce, proof) {
				return connect.NewError(connect.CodeUnauthenticated, errors.New("bad proof"))
			}
			return stream.Send(welcomeFrame(goodWelcome()))
		})
		for i := 0; i < 3; i++ {
			if _, err := open(t, client, server, cred); err != nil {
				t.Fatalf("openSession %d: %v", i, err)
			}
		}
		mu.Lock()
		defer mu.Unlock()
		if len(nonces) != 3 {
			t.Fatalf("%d hellos, want 3", len(nonces))
		}
		for i := range nonces {
			for j := i + 1; j < len(nonces); j++ {
				if bytes.Equal(nonces[i], nonces[j]) {
					t.Fatalf("hellos %d and %d carry the same nonce", i, j)
				}
			}
		}
	})
}

func TestOpenRejectsWrongProofAndOrder(t *testing.T) {
	type attempt struct {
		name   string
		script func(*scene, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error
		// proofAllowed is true when the client may send a proof before the case ends.
		proofAllowed bool
		// mismatch is true when the cause is a signature that the saved key did not make.
		mismatch bool
	}
	var otherKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x09}, 32))

	// sendChallenge answers a Hello with a challenge that the case builds.
	sendChallenge := func(build func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse) func(*scene, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		return func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
			hello, err := s.readHello(stream)
			if err != nil {
				return err
			}
			if err := stream.Send(build(s, hello)); err != nil {
				return err
			}
			// A well-behaved client sends no proof. Wait for the stream to end.
			if _, err := s.readProof(stream); err != nil {
				return nil
			}
			return nil
		}
	}
	signWith := func(key ed25519.PrivateKey, label string, swap bool, machineID string) func(*scene, *linkv1.Hello) *linkv1.OpenResponse {
		return func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			a, b := hello.GetMachineNonce(), fixedServerNonce
			if swap {
				a, b = b, a
			}
			sig := ed25519.Sign(key, proofText(label, machineID, a, b))
			return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{ServerNonce: fixedServerNonce, ServerSignature: sig}}}
		}
	}
	fixture, _ := loadProofFixture(t)
	keys := fixtureKeysOf(t)

	rejected := []attempt{
		{name: "bad signature", mismatch: true, script: sendChallenge(func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			c := s.challenge(hello, fixedServerNonce)
			c.GetChallenge().ServerSignature[0] ^= 1
			return c
		})},
		{name: "signature of another key", mismatch: true, script: sendChallenge(signWith(otherKey, serverLabel, false, fixture.MachineID))},
		{name: "signature with the machine label", mismatch: true, script: sendChallenge(signWith(keys.serverKey, machineLabel, false, fixture.MachineID))},
		{name: "signature with swapped nonces", mismatch: true, script: sendChallenge(signWith(keys.serverKey, serverLabel, true, fixture.MachineID))},
		{name: "signature for another machine", script: sendChallenge(signWith(keys.serverKey, serverLabel, false, "0193a2b4-5c6d-7e8f-9a0b-000000000000"))},
		{name: "stale signature from an earlier connection", mismatch: true, script: sendChallenge(func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			old := bytes.Repeat([]byte{0x11}, 32) // the nonce of the fixture, not this connection's
			sig := ed25519.Sign(s.keys.serverKey, proofText(serverLabel, s.keys.machineID, old, fixedServerNonce))
			return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{ServerNonce: fixedServerNonce, ServerSignature: sig}}}
		})},
		{name: "signature over another server nonce", mismatch: true, script: sendChallenge(func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			c := s.challenge(hello, bytes.Repeat([]byte{0x33}, 32))
			c.GetChallenge().ServerNonce = fixedServerNonce
			return c
		})},
		{name: "empty signature", script: sendChallenge(func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			c := s.challenge(hello, fixedServerNonce)
			c.GetChallenge().ServerSignature = nil
			return c
		})},
		{name: "short signature", script: sendChallenge(func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			c := s.challenge(hello, fixedServerNonce)
			c.GetChallenge().ServerSignature = c.GetChallenge().ServerSignature[:63]
			return c
		})},
		{name: "short nonce", script: sendChallenge(func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			return s.challenge(hello, fixedServerNonce[:31])
		})},
		{name: "long nonce", script: sendChallenge(func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			return s.challenge(hello, append(bytes.Clone(fixedServerNonce), 0))
		})},
		{name: "empty challenge", script: sendChallenge(func(*scene, *linkv1.Hello) *linkv1.OpenResponse {
			return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{}}}
		})},
		{name: "a welcome in place of the challenge", script: sendChallenge(func(*scene, *linkv1.Hello) *linkv1.OpenResponse {
			return welcomeFrame(goodWelcome())
		})},
		{name: "a heartbeat acknowledgement in place of the challenge", script: sendChallenge(func(*scene, *linkv1.Hello) *linkv1.OpenResponse {
			return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}}
		})},
		{name: "an empty frame in place of the challenge", script: sendChallenge(func(*scene, *linkv1.Hello) *linkv1.OpenResponse {
			return &linkv1.OpenResponse{}
		})},
		{name: "a challenge of 70 KiB", script: sendChallenge(func(s *scene, hello *linkv1.Hello) *linkv1.OpenResponse {
			nonce := bytes.Repeat([]byte{0x22}, 70<<10)
			return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{
				ServerNonce: nonce, ServerSignature: s.serverSignature(hello, nonce),
			}}}
		})},
	}

	// After a good proof the server sends something other than a welcome.
	afterProof := func(frame func() *linkv1.OpenResponse) func(*scene, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		return func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
			hello, err := s.readHello(stream)
			if err != nil {
				return err
			}
			if err := stream.Send(s.challenge(hello, fixedServerNonce)); err != nil {
				return err
			}
			if _, err := s.readProof(stream); err != nil {
				return err
			}
			return stream.Send(frame())
		}
	}
	badAfter := []attempt{
		{name: "a second challenge in place of the welcome", proofAllowed: true, script: afterProof(func() *linkv1.OpenResponse {
			return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{ServerNonce: fixedServerNonce}}}
		})},
		{name: "a heartbeat acknowledgement in place of the welcome", proofAllowed: true, script: afterProof(func() *linkv1.OpenResponse {
			return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}}
		})},
		{name: "a command in place of the welcome", proofAllowed: true, script: afterProof(func() *linkv1.OpenResponse {
			return &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: &linkv1.Command{}}}
		})},
		{name: "an empty frame in place of the welcome", proofAllowed: true, script: afterProof(func() *linkv1.OpenResponse { return &linkv1.OpenResponse{} })},
		{name: "a welcome without a state", proofAllowed: true, script: afterProof(func() *linkv1.OpenResponse {
			w := goodWelcome()
			w.State = nil
			return welcomeFrame(w)
		})},
		{name: "a welcome without a server time", proofAllowed: true, script: afterProof(func() *linkv1.OpenResponse {
			w := goodWelcome()
			w.ServerTime = nil
			return welcomeFrame(w)
		})},
		{name: "an empty welcome", proofAllowed: true, script: afterProof(func() *linkv1.OpenResponse { return welcomeFrame(&linkv1.Welcome{}) })},
		{name: "a waiting welcome that carries a token", proofAllowed: true, script: afterProof(func() *linkv1.OpenResponse {
			w := goodWelcome()
			w.State.Approval = linkv1.Approval_APPROVAL_WAITING
			w.State.Name = ""
			w.ConnectionToken = "fake-token-1"
			return welcomeFrame(w)
		})},
	}

	for _, p := range protocols {
		for _, c := range append(rejected, badAfter...) {
			t.Run(p.name+"/"+c.name, func(t *testing.T) {
				s, client, server, cred := startScene(t, p, c.script)
				sess, err := open(t, client, server, cred)
				if err == nil {
					t.Fatal("openSession accepted a malformed handshake")
				}
				if sess != nil {
					t.Fatal("openSession returned a session together with an error")
				}
				if code, terminal := isTerminal(err); terminal {
					t.Fatalf("a bad handshake is a final refusal %q; only a verified server may end the machine's retries: %v", code, err)
				}
				if hellos, _, _ := s.seen.counts(); hellos != 1 {
					t.Fatalf("the server saw %d hellos, want 1", hellos)
				}
				// The server finishes its script once the client closes the stream.
				select {
				case <-s.done:
				case <-time.After(openCeiling):
					t.Fatal("the server's stream did not end")
				}
				_, proofs, others := s.seen.counts()
				if !c.proofAllowed && proofs != 0 {
					t.Fatalf("the client sent %d proofs to a server that did not prove itself", proofs)
				}
				if others != 0 {
					t.Fatalf("the client sent %d unexpected frames", others)
				}
				if c.mismatch && !errors.Is(err, errServerKeyMismatch) {
					t.Fatalf("error %v is not errServerKeyMismatch", err)
				}
			})
		}
	}

	t.Run("a bad signature is a key mismatch", func(t *testing.T) {
		_, client, server, cred := startScene(t, protocols[0], sendChallenge(signWith(otherKey, serverLabel, false, fixture.MachineID)))
		if _, err := open(t, client, server, cred); !errors.Is(err, errServerKeyMismatch) {
			t.Fatalf("error %v is not errServerKeyMismatch", err)
		}
	})

	for _, p := range protocols {
		t.Run(p.name+"/approval", func(t *testing.T) {
			cases := []struct {
				name      string
				approval  linkv1.Approval
				token     string
				wantToken string
			}{
				{"approved keeps its token", linkv1.Approval_APPROVAL_APPROVED, "fake-token-2", "fake-token-2"},
				{"waiting without a token", linkv1.Approval_APPROVAL_WAITING, "", ""},
				{"unspecified discards its token", linkv1.Approval_APPROVAL_UNSPECIFIED, "fake-token-3", ""},
				{"unknown discards its token", linkv1.Approval(99), "fake-token-4", ""},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					w := goodWelcome()
					w.State.Approval = c.approval
					if c.approval == linkv1.Approval_APPROVAL_WAITING {
						w.State.Name = ""
					}
					w.ConnectionToken = c.token
					_, client, server, cred := startScene(t, p, honest(w))
					sess, err := open(t, client, server, cred)
					if err != nil {
						t.Fatalf("openSession: %v", err)
					}
					if got := sess.welcome.GetConnectionToken(); got != c.wantToken {
						t.Fatal("the session kept or discarded the wrong token")
					}
					if c.approval == linkv1.Approval(99) || c.approval == linkv1.Approval_APPROVAL_UNSPECIFIED {
						if sess.welcome.GetState().GetApproval() != linkv1.Approval_APPROVAL_WAITING {
							t.Fatalf("the approval %v is kept, want waiting", sess.welcome.GetState().GetApproval())
						}
					}
				})
			}
		})

		t.Run(p.name+"/the largest planned welcome fits", func(t *testing.T) {
			w := goodWelcome()
			w.ConnectionToken = string(bytes.Repeat([]byte{'t'}, 64))
			w.LastEventNumber = 1<<63 - 1
			w.State.Revision = 1<<63 - 1
			w.State.Name = ""
			// Grow the name until the wrapped message is 161 bytes, the size that the plan sets.
			for proto.Size(welcomeFrame(w)) < 161 {
				w.State.Name += "n"
			}
			if got := proto.Size(welcomeFrame(w)); got != 161 {
				t.Fatalf("the fixture welcome is %d bytes, want 161", got)
			}
			_, client, server, cred := startScene(t, p, honest(w))
			sess, err := open(t, client, server, cred)
			if err != nil {
				t.Fatalf("openSession: %v", err)
			}
			if !proto.Equal(sess.welcome, w) {
				t.Fatal("the session changed the largest planned welcome")
			}
		})
	}
}

func TestOpenExpiryAndRefusal(t *testing.T) {
	if handshakeTimeout != 10*time.Second {
		t.Fatalf("each exchange of the handshake is capped at %v, want 10 seconds", handshakeTimeout)
	}

	for _, p := range protocols {
		t.Run(p.name+"/deadlines", func(t *testing.T) {
			stalls := map[string]func(*scene, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error{
				"no answer to the hello": func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					if _, err := s.readHello(stream); err != nil {
						return err
					}
					_, err := stream.Receive() // blocks until the client gives up
					return err
				},
				"no welcome after the proof": func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					hello, err := s.readHello(stream)
					if err != nil {
						return err
					}
					if err := stream.Send(s.challenge(hello, fixedServerNonce)); err != nil {
						return err
					}
					if _, err := s.readProof(stream); err != nil {
						return err
					}
					_, err = stream.Receive()
					return err
				},
			}
			for name, script := range stalls {
				t.Run(name, func(t *testing.T) {
					useShortHandshake(t, 300*time.Millisecond)
					_, client, server, cred := startScene(t, p, script)
					start := time.Now()
					_, err := open(t, client, server, cred)
					if err == nil {
						t.Fatal("openSession returned a session although the server went silent")
					}
					wantRetryable(t, err, name)
					if time.Since(start) > openCeiling/2 {
						t.Fatalf("openSession needed %v to give up on a silent server", time.Since(start))
					}
				})
			}

			t.Run("a cancelled context", func(t *testing.T) {
				_, client, server, cred := startScene(t, p, func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					if _, err := s.readHello(stream); err != nil {
						return err
					}
					_, err := stream.Receive()
					return err
				})
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() { _, err := openSession(ctx, client, server, cred, fixtureRunIdentity); done <- err }()
				time.Sleep(50 * time.Millisecond) // lets the hello leave; only a lower bound for the cancel
				cancel()
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("openSession succeeded after its context was cancelled")
					}
					wantRetryable(t, err, "cancellation")
				case <-time.After(openCeiling):
					t.Fatal("openSession did not return after its context was cancelled")
				}
			})
		})

		t.Run(p.name+"/refusals", func(t *testing.T) {
			// refuse builds a script. When verified is true the server proves itself first. When
			// readProof is true it then reads the proof before it refuses. A reason may sit in the
			// leading headers instead of the error's own metadata.
			refuse := func(code connect.Code, reason string, verified, readProof, inHeaders bool) func(*scene, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
				return func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					hello, err := s.readHello(stream)
					if err != nil {
						return err
					}
					refusal := connect.NewError(code, errors.New("A sentence from the server."))
					if reason != "" && !inHeaders {
						refusal.Meta().Set("Lgtm-Link-Refusal", reason)
					}
					if inHeaders {
						stream.ResponseHeader().Set("Lgtm-Link-Refusal", reason)
					}
					if verified {
						if serr := stream.Send(s.challenge(hello, fixedServerNonce)); serr != nil {
							return serr
						}
						if readProof {
							if _, rerr := s.readProof(stream); rerr != nil {
								return rerr
							}
						}
					}
					return refusal
				}
			}
			final := []struct {
				name     string
				reason   string
				readProf bool
			}{
				{"unknown_machine", "unknown_machine", false},
				{"unknown_machine", "unknown_machine", true},
				{"link_revision_mismatch", "link_revision_mismatch", false},
				{"link_revision_mismatch", "link_revision_mismatch", true},
			}
			for _, c := range final {
				name := "verified " + c.name
				if c.readProf {
					name += " after the proof"
				}
				t.Run(name, func(t *testing.T) {
					_, client, server, cred := startScene(t, p, refuse(connect.CodeFailedPrecondition, c.reason, true, c.readProf, false))
					_, err := open(t, client, server, cred)
					wantTerminal(t, err, c.reason, name)
					if err != nil && (bytes.Contains([]byte(err.Error()), []byte("A sentence from the server"))) {
						t.Fatalf("the error repeats the text of the server: %v", err)
					}
				})
			}

			retry := []struct {
				name     string
				code     connect.Code
				reason   string
				verified bool
				headers  bool
			}{
				{"unknown_machine before any challenge", connect.CodeFailedPrecondition, "unknown_machine", false, false},
				{"link_revision_mismatch before any challenge", connect.CodeFailedPrecondition, "link_revision_mismatch", false, false},
				{"event_gap before any challenge", connect.CodeFailedPrecondition, "event_gap", false, false},
				{"event_gap after a verified challenge", connect.CodeFailedPrecondition, "event_gap", true, false},
				{"failed_precondition without a reason", connect.CodeFailedPrecondition, "", true, false},
				{"failed_precondition with an unknown reason", connect.CodeFailedPrecondition, "machine_exploded", true, false},
				{"failed_precondition with a reason in another case", connect.CodeFailedPrecondition, "UNKNOWN_MACHINE", true, false},
				{"unknown_machine with another code", connect.CodePermissionDenied, "unknown_machine", true, false},
				{"unknown_machine with unauthenticated", connect.CodeUnauthenticated, "unknown_machine", true, false},
				{"unauthenticated", connect.CodeUnauthenticated, "", true, false},
				{"unauthenticated before any challenge", connect.CodeUnauthenticated, "", false, false},
				{"aborted", connect.CodeAborted, "", true, false},
				{"invalid_argument", connect.CodeInvalidArgument, "", true, false},
				{"invalid_argument before any challenge", connect.CodeInvalidArgument, "", false, false},
				{"deadline_exceeded", connect.CodeDeadlineExceeded, "", true, false},
				{"unavailable", connect.CodeUnavailable, "", true, false},
				{"unavailable before any challenge", connect.CodeUnavailable, "", false, false},
				{"out_of_range", connect.CodeOutOfRange, "", true, false},
			}
			for _, c := range retry {
				t.Run(c.name, func(t *testing.T) {
					_, client, server, cred := startScene(t, p, refuse(c.code, c.reason, c.verified, false, c.headers))
					_, err := open(t, client, server, cred)
					wantRetryable(t, err, c.name)
				})
			}

			t.Run("a refusal after a bad challenge stays retryable", func(t *testing.T) {
				_, client, server, cred := startScene(t, p, func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					hello, err := s.readHello(stream)
					if err != nil {
						return err
					}
					c := s.challenge(hello, fixedServerNonce)
					c.GetChallenge().ServerSignature[3] ^= 1
					if err := stream.Send(c); err != nil {
						return err
					}
					e := connect.NewError(connect.CodeFailedPrecondition, errors.New("x"))
					e.Meta().Set("Lgtm-Link-Refusal", "unknown_machine")
					return e
				})
				_, err := open(t, client, server, cred)
				wantRetryable(t, err, "a refusal from a server that failed the proof")
				if !errors.Is(err, errServerKeyMismatch) {
					t.Fatalf("error %v: a challenge that failed verification is a key mismatch", err)
				}
			})

			for _, revision := range []uint32{0, 1, 1 << 31} {
				t.Run("a welcome of another revision", func(t *testing.T) {
					w := goodWelcome()
					w.LinkRevision = revision
					_, client, server, cred := startScene(t, p, honest(w))
					_, err := open(t, client, server, cred)
					wantTerminal(t, err, "link_revision_mismatch", "welcome revision")
				})
			}
		})
	}
}

func TestOpenElapsedClockMoves(t *testing.T) {
	a, err := elapsedClock()
	if err != nil {
		t.Fatalf("elapsedClock: %v", err)
	}
	if a <= 0 {
		t.Fatalf("the elapsed clock reads %v", a)
	}
	b, err := elapsedClock()
	if err != nil || b < a {
		t.Fatalf("the elapsed clock went from %v to %v (%v)", a, b, err)
	}
}
