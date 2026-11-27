package link

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

type openClientStream = connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]
type openServerStream = connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]

const (
	openUnknown   = "The server does not know this machine. Join it again."
	openTimeout   = "The machine did not answer in time."
	openSignature = "The machine's signature did not verify."
	openInvalid   = "The link message broke a rule of the contract."
	openDatabase  = "The server cannot reach its database, try again."
	openReplaced  = "A newer connection of this machine replaced this one."
)

var handshakeProtocols = []struct {
	name    string
	options []connect.ClientOption
}{
	{"Connect", nil},
	{"gRPC", []connect.ClientOption{connect.WithGRPC()}},
}

type handshakeFixture struct {
	*presenceFixture
	svc       *Service
	serverKey ed25519.PublicKey
	tokens    *machines.Tokens
}

type handshakeFixtureOptions struct {
	logger       *slog.Logger
	configure    func(*pgxpool.Config)
	seams        Seams
	privateStack bool
}

func newHandshakeFixture(t *testing.T, opts handshakeFixtureOptions) *handshakeFixture {
	t.Helper()
	f := newPresenceFixture(t, opts.configure)
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{21}, ed25519.SeedSize))
	pub := key.Public().(ed25519.PublicKey)
	tokens := machines.NewTokens(f.s, f.r, machines.Issuer{PublicURL: "http://127.0.0.1:8787", ServerKey: pub})
	svc := NewStreamingService(t.Context(), f.s, tokens, f.r, f.state, f.hub, key,
		Options{PrivateStack: opts.privateStack, Seams: opts.seams, Logger: opts.logger})
	return &handshakeFixture{presenceFixture: f, svc: svc, serverKey: pub, tokens: tokens}
}

type handshakeHandler struct {
	*Service
	continueOpen func(context.Context, *Handle, <-chan receivedFrame, *openServerStream) error
}

func (h handshakeHandler) Open(ctx context.Context, s *openServerStream) error {
	return h.Service.open(ctx, s, h.continueOpen)
}

func (f *handshakeFixture) route(options ...connect.HandlerOption) http.Handler {
	options = append(options, connect.WithReadMaxBytes(64<<10))
	_, route := linkv1connect.NewLinkServiceHandler(handshakeHandler{
		Service: f.svc,
		continueOpen: func(ctx context.Context, handle *Handle, frames <-chan receivedFrame, s *openServerStream) error {
			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			case frame, ok := <-frames:
				if !ok || errors.Is(frame.err, io.EOF) {
					return nil
				}
				// Read current durable identity before classifying or answering the next frame.
				_, err := f.state.CheckCurrent(ctx, handle.machineID, handle.streamID)
				if errors.Is(err, ErrUnknownMachine) {
					return openRefusal(openUnknown, "unknown_machine")
				}
				if errors.Is(err, ErrNotCurrent) {
					return connect.NewError(connect.CodeAborted, errors.New(openReplaced))
				}
				if err != nil {
					return connect.NewError(connect.CodeUnavailable, errors.New(openDatabase))
				}
				if frame.err != nil || frame.request.GetHeartbeat() == nil {
					return connect.NewError(connect.CodeInvalidArgument, errors.New(openInvalid))
				}
				return transport.Send(ctx, func() error {
					return s.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_HeartbeatAck{HeartbeatAck: &linkv1.HeartbeatAck{}}})
				})
			}
		},
	}, options...)
	return route
}

func openRefusal(message, reason string) error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(message))
	err.Meta().Set("Lgtm-Link-Refusal", reason)
	return err
}

func handshakeHTTPClient(t *testing.T) *http.Client {
	t.Helper()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	client := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	t.Cleanup(client.CloseIdleConnections)
	return client
}

type handshakeWireOptions struct {
	controls *handshakeControls
	codec    connect.Codec
	wrap     func(http.Handler) http.Handler
}

func (f *handshakeFixture) wire(t *testing.T, opts handshakeWireOptions, clientOptions ...connect.ClientOption) linkv1connect.LinkServiceClient {
	t.Helper()
	var options []connect.HandlerOption
	if opts.codec != nil {
		options = append(options, connect.WithCodec(opts.codec))
	}
	handler := transport.WithLinkStream(transport.WithTimeout(f.route(options...), 5*time.Second), 64<<10)
	if opts.controls != nil {
		next := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), handshakeControlsKey{}, opts.controls)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	if opts.wrap != nil {
		handler = opts.wrap(handler)
	}
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server := httptest.NewUnstartedServer(handler)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	return linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), server.URL, clientOptions...)
}

func (f *handshakeFixture) startRunner(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	follower, err := store.StartFollower(ctx, f.s, nil)
	if err != nil {
		t.Fatal("The link fixture follower could not start.")
	}
	t.Cleanup(follower.Close)
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		storetest.Receive(t, "the link fixture runner exiting", done)
	})
	go func() {
		defer close(done)
		f.svc.Run(ctx, follower)
	}()
}

func (f *handshakeFixture) register(t *testing.T, host string) (machines.Machine, ed25519.PrivateKey) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal("A fixture machine key could not be generated.")
	}
	return f.machine(t, pub, host), key
}

func startHandshake(t *testing.T, client linkv1connect.LinkServiceClient, headers map[string]string) *openClientStream {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	t.Cleanup(cancel)
	s := client.Open(ctx)
	for name, value := range headers {
		s.RequestHeader().Set(name, value)
	}
	t.Cleanup(func() {
		cancel()
		_ = s.CloseRequest()
		_ = s.CloseResponse()
	})
	return s
}

func helloFrame(id string, revision uint32, nonce byte) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Hello{Hello: &linkv1.Hello{
		MachineId: id, LinkRevision: revision, MachineNonce: bytes.Repeat([]byte{nonce}, 32),
		RunId: id, FirstEventNumber: 1,
	}}}
}

func sendHandshake(t *testing.T, s *openClientStream, frame *linkv1.OpenRequest) {
	t.Helper()
	if err := s.Send(frame); err != nil && !errors.Is(err, io.EOF) {
		t.Fatal("The fixture could not send an Open frame.")
	}
}

// The fixture derives its signatures directly from the frozen contract.
func fixtureProofText(label string, hello *linkv1.Hello, nonce []byte) []byte {
	text := append([]byte(label), 0, byte(len(hello.GetMachineId())))
	text = append(text, hello.GetMachineId()...)
	text = append(text, hello.GetMachineNonce()...)
	return append(text, nonce...)
}

type openReceiveResult struct {
	frame *linkv1.OpenResponse
	err   error
}

func receiveOpenFrame(s *openClientStream) openReceiveResult {
	frame, err := s.Receive()
	return openReceiveResult{frame, err}
}

func requireChallenge(t *testing.T, s *openClientStream, key ed25519.PublicKey, hello *linkv1.Hello) *linkv1.Challenge {
	t.Helper()
	return checkChallenge(t, s, receiveOpenFrame(s), key, hello)
}

func checkChallenge(t *testing.T, s *openClientStream, got openReceiveResult, key ed25519.PublicKey, hello *linkv1.Hello) *linkv1.Challenge {
	t.Helper()
	c := got.frame.GetChallenge()
	if got.err != nil || c == nil || len(c.GetServerNonce()) != 32 ||
		!ed25519.Verify(key, fixtureProofText("lgtm-link-v1 server", hello, c.GetServerNonce()), c.GetServerSignature()) {
		t.Fatalf("The first frame was not a verifiable Challenge. The receive code was %v.", connect.CodeOf(got.err))
	}
	if s.ResponseHeader().Get("Cache-Control") != "no-store" {
		t.Error("The Challenge committed headers without Cache-Control no-store.")
	}
	return c
}

func proofFrame(key ed25519.PrivateKey, hello *linkv1.Hello, c *linkv1.Challenge) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{Proof: &linkv1.Proof{
		MachineSignature: ed25519.Sign(key, fixtureProofText("lgtm-link-v1 machine", hello, c.GetServerNonce())),
	}}}
}

func requireWelcome(t *testing.T, s *openClientStream) *linkv1.Welcome {
	t.Helper()
	msg, err := s.Receive()
	if err != nil || msg.GetWelcome() == nil {
		t.Fatalf("The stream did not receive Welcome. The receive code was %v.", connect.CodeOf(err))
	}
	return msg.GetWelcome()
}

func requireOpenEnd(t *testing.T, s *openClientStream, code connect.Code, sentence, reason string) {
	t.Helper()
	msg, err := s.Receive()
	checkOpenEnd(t, msg, err, code, sentence, reason)
}

func checkOpenEnd(t *testing.T, msg *linkv1.OpenResponse, err error, code connect.Code, sentence, reason string) {
	t.Helper()
	if msg != nil {
		t.Fatal("The stream sent another frame where its terminal status was expected.")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != code {
		t.Fatalf("Open ended with code %v, want %v.", connect.CodeOf(err), code)
	}
	if got := ce.Meta().Values("Lgtm-Link-Refusal"); (reason == "" && len(got) != 0) ||
		(reason != "" && (len(got) != 1 || got[0] != reason)) {
		t.Error("Open returned incorrect refusal metadata for the expected decision.")
	}
	if ce.Message() != sentence || len(ce.Details()) != 0 {
		t.Error("Open did not return only the expected fixed sentence.")
	}
}

func TestProofTextsMatchContractExamples(t *testing.T) {
	data, err := os.ReadFile("../../../contracts/testdata/link-proof.json")
	var example map[string]string
	if err != nil || json.Unmarshal(data, &example) != nil {
		t.Fatal("The frozen proof examples could not be read.")
	}
	decode := func(field string) []byte {
		decoded, err := hex.DecodeString(example[field])
		if err != nil || len(decoded) == 0 {
			t.Fatal("A frozen proof example is missing its bytes.")
		}
		return decoded
	}
	for _, tc := range []struct {
		role string
		text func(string, []byte, []byte) []byte
	}{{"server", ServerProofText}, {"machine", MachineProofText}} {
		t.Run(tc.role, func(t *testing.T) {
			text := tc.text(example["machine_id"], decode("machine_nonce"), decode("server_nonce"))
			if !bytes.Equal(text, decode(tc.role+"_text")) {
				t.Error("The proof text does not match the frozen byte example.")
			}
			key := ed25519.NewKeyFromSeed(decode(tc.role + "_seed"))
			if !bytes.Equal(ed25519.Sign(key, text), decode(tc.role+"_signature")) {
				t.Error("The signature does not match the frozen example.")
			}
		})
	}
}

type handshakeLog struct {
	mu  sync.Mutex
	log bytes.Buffer
}

func (l *handshakeLog) Write(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.log.Write(data)
}

func (l *handshakeLog) contains(marker string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return marker != "" && strings.Contains(l.log.String(), marker)
}

func TestHandshakeOpensStream(t *testing.T) {
	logs := &handshakeLog{}
	f := newHandshakeFixture(t, handshakeFixtureOptions{logger: slog.New(slog.NewTextHandler(logs, nil))})
	client := f.wire(t, handshakeWireOptions{})
	m, key := f.register(t, "station")
	a := startHandshake(t, client, map[string]string{"X-Forwarded-For": presenceAddress})
	a.RequestHeader().Add("X-Forwarded-For", "malformed")
	helloA := helloFrame(m.ID, LinkRevision, 11)
	sendHandshake(t, a, helloA)
	cA := requireChallenge(t, a, f.serverKey, helloA.GetHello())
	start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	sendHandshake(t, a, proofFrame(key, helloA.GetHello(), cA))
	waiting := requireWelcome(t, a)
	end := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	if waiting.GetLinkRevision() != LinkRevision || waiting.GetLastEventNumber() != 0 ||
		!proto.Equal(waiting.GetState(), &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_WAITING, Revision: 1}) ||
		waiting.GetConnectionToken() != "" || waiting.GetServerTime().CheckValid() != nil ||
		waiting.GetServerTime().AsTime().Before(start) || waiting.GetServerTime().AsTime().After(end) {
		t.Error("The waiting Welcome violated the revision, mark, standing, token or database time rules.")
	}
	rowA := f.current(t, m.ID)
	awaitHeld(t, f.hub, m.ID, rowA.StreamID)
	if rowA.Address != "127.0.0.1" {
		t.Error("Direct admission did not store the transport peer address.")
	}
	if !streamVersion7.MatchString(rowA.StreamID) || rowA.Generation != 1 {
		t.Error("The first admitted stream did not have a live version 7 identifier.")
	}
	storetest.Exec(t, f.obs, "UPDATE link_machines SET event_mark = 42 WHERE machine_id = $1", m.ID)
	b := startHandshake(t, client, nil)
	helloB := helloFrame(m.ID, LinkRevision, 11)
	sendHandshake(t, b, helloB)
	cB := requireChallenge(t, b, f.serverKey, helloB.GetHello())
	if bytes.Equal(cA.GetServerNonce(), cB.GetServerNonce()) {
		t.Error("Two Open calls reused the server nonce for the same Hello.")
	}
	// Proof has not arrived. Approval must be read when admission locks the row.
	f.approve(t, m.ID)
	sendHandshake(t, b, proofFrame(key, helloB.GetHello(), cB))
	approved := requireWelcome(t, b)
	if approved.GetLinkRevision() != LinkRevision || approved.GetLastEventNumber() != 42 ||
		!proto.Equal(approved.GetState(), &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_APPROVED, Revision: 2, Name: "Approved machine"}) ||
		!validConnectionToken(approved.GetConnectionToken()) {
		t.Error("Welcome did not use the standing and mark committed at approved admission.")
	}
	rowB := f.current(t, m.ID)
	if !streamVersion7.MatchString(rowB.StreamID) || rowB.StreamID == rowA.StreamID || rowB.Generation != 2 {
		t.Error("Replacement did not allocate a fresh version 7 identifier and generation.")
	}
	requireOpenEnd(t, a, connect.CodeAborted, openReplaced, "")
	c := startHandshake(t, client, nil)
	sendHandshake(t, c, helloFrame(m.ID, LinkRevision, 12))
	challenge := requireChallenge(t, c, f.serverKey, helloFrame(m.ID, LinkRevision, 12).GetHello())
	sendHandshake(t, c, proofFrame(key, helloFrame(m.ID, LinkRevision, 12).GetHello(), challenge))
	fresh := requireWelcome(t, c)
	if !validConnectionToken(fresh.GetConnectionToken()) || fresh.GetConnectionToken() == approved.GetConnectionToken() {
		t.Error("Consecutive approved Welcomes did not have different valid tokens.")
	}
	if validConnectionToken(approved.GetConnectionToken()) && validConnectionToken(fresh.GetConnectionToken()) {
		for _, token := range []string{approved.GetConnectionToken(), fresh.GetConnectionToken()} {
			if saved := storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
				SELECT FROM (
					SELECT to_jsonb(m)::text AS row FROM link_machines m
					UNION ALL SELECT to_jsonb(c)::text FROM link_connections c
					UNION ALL SELECT to_jsonb(m)::text FROM machines m
					UNION ALL SELECT to_jsonb(t)::text FROM client_join_tokens t
					UNION ALL SELECT to_jsonb(f)::text FROM feed f
				) all_rows WHERE strpos(row, $1) > 0)`, token); saved {
				t.Error("An approved connection token was saved in a machine, link or feed row.")
			}
		}
		checkOwnerRefusesConnectionToken(t, f, fresh.GetConnectionToken())
	}
	_ = c.CloseRequest()
	if _, err := c.Receive(); !errors.Is(err, io.EOF) {
		t.Error("The third stream did not finish normally.")
	}
	if logs.contains(approved.GetConnectionToken()) || logs.contains(fresh.GetConnectionToken()) {
		t.Error("The server logger received an approved connection token.")
	}

}

func checkOwnerRefusesConnectionToken(t *testing.T, f *handshakeFixture, token string) {
	t.Helper()
	if err := f.s.Migrate(t.Context(), auth.Migrations(), machines.Migrations(), Migrations()); err != nil {
		t.Fatal("The session migration did not apply to the owner-call fixture.")
	}
	origins, err := auth.NewOriginPolicy("http://127.0.0.1:8787", nil)
	if err != nil {
		t.Fatal("The loopback origin policy could not be initialized.")
	}
	password, err := auth.NewPassword("owner-call fixture password")
	if err != nil {
		t.Fatal("The session fixture password could not be initialized.")
	}
	sessions := auth.NewSessions(f.s, password)
	router := api.NewHandler(api.Options{Origins: origins, Sessions: sessions,
		Clients: machines.NewService(f.s, sessions, f.r, f.tokens)})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, lgtmv1connect.ClientServiceRenameClientProcedure, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var body struct{ Code string }
	if w.Code != http.StatusUnauthorized || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Code != "unauthenticated" {
		t.Error("The owner call accepted a connection token as a session cookie.")
	}
	if bytes.Contains(w.Body.Bytes(), []byte(token)) {
		t.Error("The refused owner reply echoed the connection token.")
	}
}

func TestReplayedProofRefused(t *testing.T) {
	for _, p := range handshakeProtocols {
		t.Run(p.name, func(t *testing.T) {
			f := newHandshakeFixture(t, handshakeFixtureOptions{})
			client := f.wire(t, handshakeWireOptions{}, p.options...)
			m, key := f.register(t, "station")
			a := startHandshake(t, client, nil)
			hello := helloFrame(m.ID, LinkRevision, 13)
			sendHandshake(t, a, hello)
			cA := requireChallenge(t, a, f.serverKey, hello.GetHello())
			proof := proofFrame(key, hello.GetHello(), cA)
			sendHandshake(t, a, proof)
			requireWelcome(t, a)
			storetest.Exec(t, f.obs, "UPDATE link_machines SET event_mark=42 WHERE machine_id=$1", m.ID)
			before, row := f.head(t), f.current(t, m.ID)
			awaitHeld(t, f.hub, m.ID, row.StreamID)
			b := startHandshake(t, client, nil)
			sendHandshake(t, b, hello)
			cB := requireChallenge(t, b, f.serverKey, hello.GetHello())
			if bytes.Equal(cA.GetServerNonce(), cB.GetServerNonce()) {
				t.Error("Replaying Hello reused the server nonce.")
			}
			sendHandshake(t, b, proof)
			requireOpenEnd(t, b, connect.CodeUnauthenticated, openSignature, "")
			f.requireSaved(t, before)
			if after := f.current(t, m.ID); after.StreamID != row.StreamID || after.Generation != row.Generation || !after.Held {
				t.Error("Replaying Proof changed the admitted stream.")
			}
			if storetest.Scalar[int64](t, f.obs, "SELECT event_mark FROM link_machines WHERE machine_id=$1", m.ID) != 42 {
				t.Error("Replaying Proof changed the durable mark.")
			}
		})
	}
}

func TestOpenRefusesUnusableRegisteredKey(t *testing.T) {
	for _, p := range handshakeProtocols {
		t.Run(p.name, func(t *testing.T) {
			f := newHandshakeFixture(t, handshakeFixtureOptions{})
			var verified atomic.Int32
			client := f.wire(t, handshakeWireOptions{controls: &handshakeControls{verify: func(ed25519.PublicKey, []byte, []byte) bool {
				verified.Add(1)
				return true
			}}}, p.options...)
			for _, tc := range unusableKeys(t) {
				t.Run(tc.name, func(t *testing.T) {
					m := f.machine(t, tc.bytes, "seeded")
					s := startHandshake(t, client, nil)
					hello := helloFrame(m.ID, LinkRevision, 15)
					sendHandshake(t, s, hello)
					requireChallenge(t, s, f.serverKey, hello.GetHello())
					storetest.Exec(t, f.obs, "INSERT INTO link_machines(machine_id,event_mark) VALUES ($1,42)", m.ID)
					before := f.head(t)
					sendHandshake(t, s, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{Proof: &linkv1.Proof{MachineSignature: make([]byte, 64)}}})
					requireOpenEnd(t, s, connect.CodeUnauthenticated, openSignature, "")
					if verified.Load() != 0 || f.connectionCount(t, m.ID) != 0 ||
						storetest.Scalar[int64](t, f.obs, "SELECT event_mark FROM link_machines WHERE machine_id=$1", m.ID) != 42 ||
						storetest.Scalar[int64](t, f.obs, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", m.ID) != 0 {
						t.Error("The unusable key reached signature verification or changed the connection, generation or mark.")
					}
					f.requireSaved(t, before)
				})
			}
			t.Run("the verifier sees a usable key", func(t *testing.T) {
				m, key := f.register(t, "usable")
				s := startHandshake(t, client, nil)
				submitProof(t, f, s, m.ID, key, 16)
				requireWelcome(t, s)
				if verified.Load() != 1 {
					t.Error("The accepting verifier spy was not called for the usable key.")
				}
			})
		})
	}
}

type openClock struct {
	base     time.Time
	elapsed  atomic.Int64
	timers   chan chan time.Time
	received chan time.Time
}

func newOpenClock() *openClock {
	return &openClock{base: time.Now(), timers: make(chan chan time.Time, 4), received: make(chan time.Time, 8)}
}

func (c *openClock) controls() *handshakeControls {
	return &handshakeControls{
		now: func() time.Time { return c.base.Add(time.Duration(c.elapsed.Load())) },
		timer: func(_ time.Duration) (<-chan time.Time, func()) {
			fire := make(chan time.Time, 1)
			c.timers <- fire
			return fire, func() {}
		},
		afterReceive: func(_ *linkv1.OpenRequest, _ error, at time.Time) { c.received <- at },
	}
}

func (c *openClock) set(elapsed time.Duration) { c.elapsed.Store(int64(elapsed)) }

func (c *openClock) timer(t *testing.T, phase string) chan time.Time {
	t.Helper()
	select {
	case timer := <-c.timers:
		return timer
	case <-time.After(2 * time.Second):
		t.Fatalf("The %s phase did not install its clock timer.", phase)
		return nil
	}
}

func startTimedHello(t *testing.T, clock *openClock, s *openClientStream) (chan time.Time, *storetest.Task[openReceiveResult]) {
	t.Helper()
	sendHandshake(t, s, nil)
	first := storetest.Start(t, func() openReceiveResult { return receiveOpenFrame(s) })
	select {
	case timer := <-clock.timers:
		return timer, first
	case <-first.Done():
		t.Fatal("The handler ended before it installed the Hello timer for a headers-only request.")
	case <-time.After(2 * time.Second):
		t.Fatal("The Hello phase did not arm its timer for a headers-only request.")
	}
	return nil, nil
}

func (c *openClock) observed(t *testing.T, when time.Duration) {
	t.Helper()
	if at := storetest.Receive(t, "the reader timestamp", c.received); !at.Equal(c.base.Add(when)) {
		t.Error("The reader did not record the time at receive completion.")
	}
}

type handshakeQueryTrace func(context.Context, *pgx.Conn, pgx.TraceQueryStartData) context.Context

func (trace handshakeQueryTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return trace(ctx, conn, data)
}

func (handshakeQueryTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

type cleanupQueryKey struct{}

type cleanupQueryTrace struct {
	handshakeQueryTrace
	finished chan<- struct{}
}

func (trace cleanupQueryTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if ctx.Value(cleanupQueryKey{}) == true {
		trace.finished <- struct{}{}
	}
}

type handshakeCodec struct {
	onWelcome      func()
	malformedHello bool
}

func (handshakeCodec) Name() string { return "proto" }
func (c handshakeCodec) Marshal(message any) ([]byte, error) {
	if response, ok := message.(*linkv1.OpenResponse); ok && response.GetWelcome() != nil && c.onWelcome != nil {
		c.onWelcome()
	}
	if request, ok := message.(*linkv1.OpenRequest); ok && request.GetHello() != nil && c.malformedHello {
		return []byte{0x0a, 0x02, 0xff}, nil
	}
	return proto.Marshal(message.(proto.Message))
}

func (handshakeCodec) Unmarshal(data []byte, message any) error {
	return proto.Unmarshal(data, message.(proto.Message))
}

func TestHandshakeDecisionOrder(t *testing.T) {
	// These rows use revision 1 once the server switches to revision 2.
	unsupportedRevision := uint32(1)
	if LinkRevision == 1 {
		unsupportedRevision = 2
	}
	revisionSentence := fmt.Sprintf("The server speaks revision %d of the link.", LinkRevision)
	for _, p := range handshakeProtocols {
		t.Run(p.name, func(t *testing.T) {
			for _, tc := range []struct {
				name                 string
				revision             uint32
				unknown, unavailable bool
				proof                string
				code                 connect.Code
				sentence, reason     string
			}{
				{"lookup failure", unsupportedRevision, false, true, "late", connect.CodeUnavailable, openDatabase, ""},
				{"unknown takes precedence", unsupportedRevision, true, false, "late", connect.CodeFailedPrecondition, openUnknown, "unknown_machine"},
				{"revision takes precedence", unsupportedRevision, false, false, "late", connect.CodeFailedPrecondition, revisionSentence, "link_revision_mismatch"},
				{"early valid proof", LinkRevision, false, false, "early", 0, "", ""},
				{"proof exactly at cutoff", LinkRevision, false, false, "exact", 0, "", ""},
				{"late valid proof", LinkRevision, false, false, "late", connect.CodeDeadlineExceeded, openTimeout, ""},
				{"late malformed proof", LinkRevision, false, false, "late malformed", connect.CodeDeadlineExceeded, openTimeout, ""},
				{"missing proof", LinkRevision, false, false, "missing", connect.CodeDeadlineExceeded, openTimeout, ""},
				{"malformed proof", LinkRevision, false, false, "malformed", connect.CodeInvalidArgument, openInvalid, ""},
				{"wrong kind instead of Proof", LinkRevision, false, false, "wrong kind", connect.CodeInvalidArgument, openInvalid, ""},
				{"oversized Proof", LinkRevision, false, false, "oversize", connect.CodeInvalidArgument, openInvalid, ""},
				{"wrong signature", LinkRevision, false, false, "wrong signature", connect.CodeUnauthenticated, openSignature, ""},
			} {
				t.Run(tc.name, func(t *testing.T) {
					lookupStarted := make(chan struct{}, 1)
					released, releaseLookup := storetest.Signal(t)
					var armed atomic.Bool
					f := newHandshakeFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
						cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
							if strings.Contains(strings.ToLower(data.SQL), "from machines") && armed.CompareAndSwap(true, false) {
								lookupStarted <- struct{}{}
								select {
								case <-released:
								case <-ctx.Done():
								}
							}
							return ctx
						})
					}})
					defer releaseLookup()
					m, key := f.register(t, "station")
					id := m.ID
					if tc.unknown {
						id = fixtureStreamID(t)
					}
					if tc.unavailable {
						storetest.Exec(t, f.obs, "ALTER TABLE machines RENAME TO unavailable_machines")
					}
					if tc.proof == "malformed" {
						storetest.Exec(t, f.obs, "INSERT INTO link_machines (machine_id, event_mark) VALUES ($1, 42)", m.ID)
					}
					before := f.head(t)
					clock := newOpenClock()
					client := f.wire(t, handshakeWireOptions{controls: clock.controls()}, p.options...)
					s := startHandshake(t, client, nil)
					armed.Store(true)
					_, first := startTimedHello(t, clock, s)
					clock.set(3 * time.Second)
					hello := helloFrame(id, tc.revision, 17)
					sendHandshake(t, s, hello)
					challenge := checkChallenge(t, s, first.Wait(t), f.serverKey, hello.GetHello())
					proofTimer := clock.timer(t, "Proof")
					clock.observed(t, 3*time.Second)
					storetest.Receive(t, "the paused registry lookup after Challenge", lookupStarted)

					const cutoff = 13 * time.Second
					when := cutoff - time.Second
					proof := proofFrame(key, hello.GetHello(), challenge)
					switch tc.proof {
					case "exact":
						when = cutoff
					case "late":
						when = cutoff + time.Nanosecond
					case "malformed", "late malformed":
						proof = &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{Proof: &linkv1.Proof{MachineSignature: []byte{1}}}}
						if tc.proof == "late malformed" {
							when = cutoff + time.Nanosecond
						}
					case "wrong kind":
						proof = &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}}
					case "oversize":
						proof.ProtoReflect().SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 20, protowire.BytesType), make([]byte, 64<<10)))
					case "wrong signature":
						proof.GetProof().MachineSignature[0] ^= 1
					}
					if tc.proof != "missing" {
						clock.set(when)
						sendHandshake(t, s, proof)
						// The reader must read Proof independently of the outstanding lookup.
						clock.observed(t, when)
					}
					clock.set(2 * cutoff)
					proofTimer <- clock.base.Add(2 * cutoff)
					releaseLookup()
					if tc.code == 0 {
						requireWelcome(t, s)
						if p := f.current(t, m.ID); p.Generation != 1 {
							t.Error("An on-time Proof did not admit after its slow lookup completed.")
						}
						return
					}
					requireOpenEnd(t, s, tc.code, tc.sentence, tc.reason)
					f.requireSaved(t, before)
					if f.connectionCount(t, id) != 0 {
						t.Error("A refused handshake created an admission.")
					}
				})
			}
		})
	}
}

func TestHandshakeValueRules(t *testing.T) {
	for _, p := range handshakeProtocols {
		t.Run(p.name, func(t *testing.T) {
			f := newHandshakeFixture(t, handshakeFixtureOptions{})
			m, _ := f.register(t, "station")
			before := f.head(t)
			for _, tc := range []struct {
				name          string
				makeFrame     func() *linkv1.OpenRequest
				wireMalformed bool
			}{
				{"empty", func() *linkv1.OpenRequest { return &linkv1.OpenRequest{} }, false},
				{"wrong kind", func() *linkv1.OpenRequest {
					return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}}
				}, false},
				{"missing identifier", func() *linkv1.OpenRequest { return helloFrame("", LinkRevision, 14) }, false},
				{"noncanonical identifier", func() *linkv1.OpenRequest {
					return helloFrame("0193A2B4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", LinkRevision, 14)
				}, false},
				{"malformed nonce", func() *linkv1.OpenRequest {
					f := helloFrame(m.ID, LinkRevision, 14)
					f.GetHello().MachineNonce = make([]byte, 31)
					return f
				}, false},
				{"malformed protobuf", func() *linkv1.OpenRequest { return helloFrame(m.ID, LinkRevision, 14) }, true},
				{"oversized first frame", func() *linkv1.OpenRequest {
					f := helloFrame(m.ID, LinkRevision, 14)
					f.ProtoReflect().SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 20, protowire.BytesType), make([]byte, 64<<10)))
					return f
				}, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					opts := append([]connect.ClientOption(nil), p.options...)
					if tc.wireMalformed {
						opts = append(opts, connect.WithCodec(handshakeCodec{malformedHello: true}))
					}
					client := f.wire(t, handshakeWireOptions{}, opts...)
					s := startHandshake(t, client, nil)
					sendHandshake(t, s, tc.makeFrame())
					requireOpenEnd(t, s, connect.CodeInvalidArgument, openInvalid, "")
				})
			}
			for _, tc := range []struct {
				name   string
				when   time.Duration
				silent bool
			}{
				{"silence with request headers", 10 * time.Second, true},
				{"Hello exactly at cutoff", 10 * time.Second, false},
				{"late Hello", 10*time.Second + time.Nanosecond, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					clock := newOpenClock()
					client := f.wire(t, handshakeWireOptions{controls: clock.controls()}, p.options...)
					s := startHandshake(t, client, nil)
					timer, first := startTimedHello(t, clock, s)
					clock.set(tc.when)
					hello := helloFrame(m.ID, LinkRevision, 15)
					if !tc.silent {
						sendHandshake(t, s, hello)
						clock.observed(t, tc.when)
					}
					timer <- clock.base.Add(tc.when)
					got := first.Wait(t)
					if tc.silent || tc.when > 10*time.Second {
						checkOpenEnd(t, got.frame, got.err, connect.CodeDeadlineExceeded, openTimeout, "")
					} else {
						checkChallenge(t, s, got, f.serverKey, hello.GetHello())
					}
				})
			}
			// Guard fixture queries behind a successful protocol result while the migration is a stub.
			if !t.Failed() {
				f.requireSaved(t, before)
				if f.connectionCount(t, m.ID) != 0 {
					t.Error("A first-frame refusal created an admission.")
				}
			}
		})
	}
}

func awaitHeld(t *testing.T, hub *Hub, machineID, streamID string) {
	t.Helper()
	storetest.Poll(t, "the admitted handle finishing Welcome", func() bool {
		return hub.Holds(machineID, streamID)
	})
}

func submitProof(t *testing.T, f *handshakeFixture, s *openClientStream, id string, key ed25519.PrivateKey, nonce byte) {
	t.Helper()
	hello := helloFrame(id, LinkRevision, nonce)
	sendHandshake(t, s, hello)
	challenge := requireChallenge(t, s, f.serverKey, hello.GetHello())
	sendHandshake(t, s, proofFrame(key, hello.GetHello(), challenge))
}

func TestRemovalOrdersAdmission(t *testing.T) {
	for _, p := range handshakeProtocols {
		for _, tc := range []struct {
			name             string
			admitted, revoke bool
		}{
			{"remove before admission", false, false},
			{"remove after admission", true, false},
			{"revoke before admission", false, true},
			{"revoke after admission", true, true},
		} {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				entered := make(chan struct{}, 1)
				wait, release := storetest.Signal(t)
				var pause atomic.Bool
				stage := func(ctx context.Context, _ string) {
					if pause.CompareAndSwap(true, false) {
						entered <- struct{}{}
						select {
						case <-wait:
						case <-ctx.Done():
						}
					}
				}
				seams := Seams{AfterProof: stage}
				if tc.admitted {
					seams = Seams{AfterAdmission: stage}
				}
				f := newHandshakeFixture(t, handshakeFixtureOptions{seams: seams})
				defer release()
				var id, tokenID string
				var key ed25519.PrivateKey
				if tc.revoke {
					created, err := f.tokens.Create(t.Context(), machines.TokenOptions{})
					if err != nil {
						t.Fatal("The fixture token could not be created.")
					}
					token, err := machines.DecodeToken(created.Text)
					if err != nil {
						t.Fatal("The fixture token could not be decoded.")
					}
					tokenID = token.GetTokenId()
					pub, machineKey, err := ed25519.GenerateKey(nil)
					if err != nil {
						t.Fatal("The fixture machine key could not be generated.")
					}
					key = machineKey
					joined, err := f.tokens.Redeem(t.Context(), machines.Redemption{
						TokenID: tokenID, Secret: token.GetSecret(), PublicKey: pub, HostName: "revoked station",
					})
					if err != nil {
						t.Fatal("The fixture token could not be redeemed.")
					}
					id = joined.MachineID
				} else {
					m, machineKey := f.register(t, "removed station")
					id, key = m.ID, machineKey
				}
				client := f.wire(t, handshakeWireOptions{}, p.options...)
				s := startHandshake(t, client, nil)
				before := f.head(t)
				pause.Store(true)
				submitProof(t, f, s, id, key, 19)
				storetest.Receive(t, "the selected admission barrier", entered)
				if tc.admitted {
					if f.connectionCount(t, id) != 1 {
						t.Fatal("AfterAdmission ran before the connection commit was visible.")
					}
					f.requireSaved(t, before, id)
				} else if f.connectionCount(t, id) != 0 {
					t.Fatal("AfterProof ran after the stream acquired a connection row.")
				}
				err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
					if tc.revoke {
						removed, err := f.tokens.Revoke(ctx, tx, tokenID)
						if err != nil || removed != id {
							return store.NewDomainError("Revoking the used token did not remove its waiting machine.")
						}
						if deleted, err := f.r.DeleteWaiting(ctx, tx, removed); err != nil || !deleted {
							return err
						}
					} else if err := f.r.Delete(ctx, tx, id); err != nil {
						return err
					}
					return f.state.Forget(ctx, tx, id)
				})
				if err != nil {
					t.Fatal("The fixture removal or revocation could not commit.")
				}
				after := f.head(t)
				f.hub.EndRemoved(id)
				release()
				first, endErr := s.Receive()
				if tc.admitted && first.GetWelcome() != nil {
					if first.GetWelcome().GetState().GetApproval() == linkv1.Approval_APPROVAL_REMOVED {
						t.Error("Welcome carried the removed standing.")
					}
					sendHandshake(t, s, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}})
					first, endErr = s.Receive()
				}
				checkOpenEnd(t, first, endErr, connect.CodeFailedPrecondition, openUnknown, "unknown_machine")
				later := startHandshake(t, client, nil)
				hello := helloFrame(id, LinkRevision, 20)
				sendHandshake(t, later, hello)
				requireChallenge(t, later, f.serverKey, hello.GetHello())
				requireOpenEnd(t, later, connect.CodeFailedPrecondition, openUnknown, "unknown_machine")
				if f.connectionCount(t, id) != 0 || storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_machines WHERE machine_id=$1", id) != 0 {
					t.Error("The removed machine retained link rows after refusal.")
				}
				f.requireSaved(t, after)
			})
		}
	}
}

func TestStaleAdmissionCannotStartWelcome(t *testing.T) {
	for _, p := range handshakeProtocols {
		for _, closeNewer := range []bool{false, true} {
			name := "replacement still live"
			if closeNewer {
				name = "replacement already closed"
			}
			t.Run(p.name+"/"+name, func(t *testing.T) {
				f := newHandshakeFixture(t, handshakeFixtureOptions{})
				m, key := f.register(t, "station")
				hold, release := storetest.Signal(t)
				defer release()
				marshaling := make(chan struct{}, 1)
				var first atomic.Bool
				first.Store(true)
				client := f.wire(t, handshakeWireOptions{codec: handshakeCodec{onWelcome: func() {
					if first.CompareAndSwap(true, false) {
						marshaling <- struct{}{}
						<-hold
					}
				}}}, p.options...)
				a := startHandshake(t, client, nil)
				submitProof(t, f, a, m.ID, key, 22)
				storetest.Receive(t, "the older Welcome entering serialization before its first write", marshaling)
				rowA := f.current(t, m.ID)
				if rowA.Generation != 1 || rowA.Held {
					t.Fatal("The barrier did not catch committed admission before Welcome finished.")
				}
				b := startHandshake(t, client, nil)
				submitProof(t, f, b, m.ID, key, 23)
				requireWelcome(t, b)
				rowB := f.current(t, m.ID)
				awaitHeld(t, f.hub, m.ID, rowB.StreamID)
				if rowB.Generation != 2 || rowB.StreamID == rowA.StreamID {
					t.Fatal("The new stream did not commit while the older send was paused.")
				}
				if closeNewer {
					_ = b.CloseRequest()
					if frame, err := b.Receive(); frame != nil || !errors.Is(err, io.EOF) {
						t.Fatal("The fixture replacement did not end normally.")
					}
					storetest.Poll(t, "unregistering the replacement", func() bool {
						return !f.hub.Holds(m.ID, rowB.StreamID)
					})
				}
				before := f.head(t)
				release()
				requireOpenEnd(t, a, connect.CodeAborted, openReplaced, "")
				f.requireSaved(t, before)
				if current := f.current(t, m.ID); current.StreamID != rowB.StreamID || current.Generation != 2 {
					t.Error("A displaced first send changed the replacement row.")
				}
			})
		}
	}
}

// openWriteBarrier starts observing only after the generated codec begins to marshal Welcome.
type openWriteBarrier struct {
	http.ResponseWriter
	nextWelcome *atomic.Bool
	entered     chan<- struct{}
	release     <-chan struct{}
}

func (w *openWriteBarrier) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *openWriteBarrier) Flush()                      { _ = http.NewResponseController(w.ResponseWriter).Flush() }
func (w *openWriteBarrier) Write(data []byte) (int, error) {
	if len(data) > 0 && w.nextWelcome.CompareAndSwap(true, false) {
		w.entered <- struct{}{}
		<-w.release
	}
	return w.ResponseWriter.Write(data)
}

func TestWelcomeStartedBeforeReplacement(t *testing.T) {
	for _, p := range handshakeProtocols {
		t.Run(p.name, func(t *testing.T) {
			f := newHandshakeFixture(t, handshakeFixtureOptions{})
			m, key := f.register(t, "station")
			hold, release := storetest.Signal(t)
			defer release()
			entered := make(chan struct{}, 1)
			var once atomic.Bool
			var nextWelcome atomic.Bool
			client := f.wire(t, handshakeWireOptions{
				codec: handshakeCodec{onWelcome: func() {
					if once.CompareAndSwap(false, true) {
						nextWelcome.Store(true)
					}
				}},
				wrap: func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						next.ServeHTTP(&openWriteBarrier{ResponseWriter: w, nextWelcome: &nextWelcome, entered: entered, release: hold}, r)
					})
				},
			}, p.options...)
			a := startHandshake(t, client, nil)
			submitProof(t, f, a, m.ID, key, 24)
			storetest.Receive(t, "the older Welcome's entered transport write", entered)
			b := startHandshake(t, client, nil)
			submitProof(t, f, b, m.ID, key, 25)
			requireWelcome(t, b)
			row := f.current(t, m.ID)
			if row.Generation != 2 {
				t.Fatal("The replacement did not admit during the occupied transport write.")
			}
			before := f.head(t)
			// The canceled stream may reject this send at the transport, or discard it after reading.
			_ = a.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{CpuPercent: proto.Uint32(9)}}})
			release()
			response, err := a.Receive()
			if err == nil && response.GetWelcome() == nil {
				t.Fatal("The displaced stream delivered an unexpected frame after its started write.")
			}
			if err == nil {
				response, err = a.Receive()
				if response != nil || err == nil {
					t.Fatal("An already started Welcome let the displaced stream send another response.")
				}
			}
			if connect.IsWireError(err) {
				checkOpenEnd(t, nil, err, connect.CodeAborted, openReplaced, "")
			}
			after := f.current(t, m.ID)
			if after.StreamID != row.StreamID || after.Generation != row.Generation ||
				after.CPUPercent != nil || !after.LastHeardAt.Equal(row.LastHeardAt) {
				t.Error("A started Welcome allowed the displaced stream to change the replacement presence.")
			}
			f.requireSaved(t, before)
		})
	}
}

type admissionReplyConn struct {
	net.Conn
	armed   atomic.Bool
	arrived chan<- struct{}
	release <-chan struct{}
}

func (c *admissionReplyConn) Read(p []byte) (int, error) {
	intercept := c.armed.CompareAndSwap(true, false)
	n, err := c.Conn.Read(p)
	if intercept && n > 0 {
		c.arrived <- struct{}{}
		<-c.release
		return 0, io.ErrUnexpectedEOF
	}
	return n, err
}

func TestAdmissionCommitReplyLost(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		name := "delete the uncertain candidate"
		if replacement {
			name = "preserve a replacement"
		}
		t.Run(name, func(t *testing.T) {
			arrived := make(chan struct{}, 1)
			replyHold, dropReply := storetest.Signal(t)
			cleanupEntered := make(chan struct{}, 1)
			cleanupFinished := make(chan struct{}, 1)
			var cleanupBackend atomic.Int32
			cleanupHold, releaseCleanup := storetest.Signal(t)
			var loseCommit, holdCleanup atomic.Bool
			wrongConn := make(chan struct{}, 1)
			f := newHandshakeFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
				dial := cfg.ConnConfig.DialFunc
				cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
					conn, err := dial(ctx, network, address)
					if err != nil {
						return nil, err
					}
					return &admissionReplyConn{Conn: conn, arrived: arrived, release: replyHold}, nil
				}
				trace := handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
					sql := strings.ToLower(strings.TrimSpace(d.SQL))
					if sql == "commit" && loseCommit.CompareAndSwap(true, false) {
						pipe, ok := conn.PgConn().Conn().(*admissionReplyConn)
						if !ok {
							wrongConn <- struct{}{}
						} else {
							pipe.armed.Store(true)
						}
					}
					if strings.Contains(sql, "nowait") && holdCleanup.CompareAndSwap(true, false) {
						cleanupBackend.Store(int32(conn.PgConn().PID()))
						cleanupEntered <- struct{}{}
						<-cleanupHold
					}
					if (sql == "commit" || sql == "rollback") && cleanupBackend.CompareAndSwap(int32(conn.PgConn().PID()), 0) {
						return context.WithValue(ctx, cleanupQueryKey{}, true)
					}
					return ctx
				})
				cfg.ConnConfig.Tracer = cleanupQueryTrace{handshakeQueryTrace: trace, finished: cleanupFinished}
			}})
			defer dropReply()
			defer releaseCleanup()
			f.startRunner(t)
			m, key := f.register(t, "station")
			queued := make(chan [2]string, 4)
			client := f.wire(t, handshakeWireOptions{controls: &handshakeControls{onQueueDisconnect: func(id, stream string) {
				queued <- [2]string{id, stream}
			}}})
			s := startHandshake(t, client, nil)
			hello := helloFrame(m.ID, LinkRevision, 27)
			sendHandshake(t, s, hello)
			challenge := requireChallenge(t, s, f.serverKey, hello.GetHello())
			f.admit(t, m.ID, 1)
			storetest.Exec(t, f.obs, "UPDATE link_machines SET event_mark=42 WHERE machine_id=$1", m.ID)
			before := f.head(t)
			holdCleanup.Store(true)
			loseCommit.Store(true)
			sendHandshake(t, s, proofFrame(key, hello.GetHello(), challenge))
			select {
			case <-arrived:
			case <-wrongConn:
				t.Fatal("The tracer could not arm the committing connection's reader.")
			case <-time.After(storetest.ObservationTimeout):
				t.Fatal("The admission commit reply did not reach the wire barrier.")
			}
			row := f.current(t, m.ID)
			if row.Generation != 2 || row.Held {
				t.Fatal("The independent reader did not see the committed, unpublished connection.")
			}
			f.requireSaved(t, before, m.ID)
			dropReply()
			requireOpenEnd(t, s, connect.CodeUnavailable, openDatabase, "")
			if got := storetest.Receive(t, "the exact uncertain cleanup obligation", queued); got != [2]string{m.ID, row.StreamID} {
				t.Error("The uncertain admission queued cleanup for a different identifier.")
			}
			storetest.Receive(t, "the queued cleanup reaching its nonwaiting lock", cleanupEntered)
			if p := f.current(t, m.ID); p.Held || p.StreamID != row.StreamID {
				t.Error("The uncertain admission granted liveness or lost its row before the held cleanup.")
			}
			var newer string
			if replacement {
				b := startHandshake(t, client, nil)
				submitProof(t, f, b, m.ID, key, 28)
				w := requireWelcome(t, b)
				if w.GetLastEventNumber() != 42 {
					t.Error("The replacement after a lost commit reply did not see the durable mark.")
				}
				newer = f.current(t, m.ID).StreamID
			}
			releaseCleanup()
			storetest.Receive(t, "the held cleanup's transaction completion", cleanupFinished)
			if replacement {
				if p := f.current(t, m.ID); p.StreamID != newer || p.Generation != 3 {
					t.Error("Cleanup of the uncertain identifier changed the replacement row.")
				}
			} else {
				storetest.Poll(t, "the uncertain candidate being deleted", func() bool { return f.connectionCount(t, m.ID) == 0 })
			}
			if mark := storetest.Scalar[int64](t, f.obs, "SELECT event_mark FROM link_machines WHERE machine_id=$1", m.ID); mark != 42 {
				t.Error("Cleanup of the uncertain connection changed the saved mark.")
			}
		})
	}
	t.Run("known rollback leaves no cleanup obligation", func(t *testing.T) {
		f := newHandshakeFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "station")
		queued := make(chan struct{}, 1)
		client := f.wire(t, handshakeWireOptions{controls: &handshakeControls{onQueueDisconnect: func(string, string) { queued <- struct{}{} }}})
		s := startHandshake(t, client, nil)
		hello := helloFrame(m.ID, LinkRevision, 29)
		sendHandshake(t, s, hello)
		challenge := requireChallenge(t, s, f.serverKey, hello.GetHello())
		storetest.Exec(t, f.obs, `CREATE FUNCTION reject_admission() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'fixture rejected the admission' USING ERRCODE='23514'; END $$;
			CREATE CONSTRAINT TRIGGER reject_admission AFTER INSERT OR UPDATE ON link_connections
			DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_admission()`)
		before := f.head(t)
		sendHandshake(t, s, proofFrame(key, hello.GetHello(), challenge))
		requireOpenEnd(t, s, connect.CodeUnavailable, openDatabase, "")
		select {
		case <-queued:
			t.Error("A confirmed rollback queued a disconnect.")
		default:
		}
		if f.connectionCount(t, m.ID) != 0 {
			t.Error("A confirmed rollback left a connection row.")
		}
		f.requireSaved(t, before)
	})
}

func TestScriptedMachineJoinsAndConnects(t *testing.T) {
	for _, p := range handshakeProtocols {
		t.Run(p.name, func(t *testing.T) {
			f := newHandshakeFixture(t, handshakeFixtureOptions{privateStack: true})
			server := machinetest.NewServer(t, f.route(), 5*time.Second)
			if server == nil {
				t.Fatal("The scripted machine server did not start.")
			}
			client := linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), server.URL, p.options...)
			rawCtx, rawCancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer rawCancel()
			raw := machinetest.Dial(rawCtx, client)
			if raw == nil {
				t.Fatal("The scripted machine could not open a raw stream.")
			}
			sendHandshake(t, raw, &linkv1.OpenRequest{})
			requireOpenEnd(t, raw, connect.CodeInvalidArgument, openInvalid, "")
			_ = raw.CloseRequest()
			_ = raw.CloseResponse()
			token, err := f.tokens.Create(t.Context(), machines.TokenOptions{})
			if err != nil {
				t.Fatal("The scripted machine's token could not be minted.")
			}
			machine, err := machinetest.Join(t.Context(), client, token.Text, "scripted station")
			if err != nil || machine == nil || !streamVersion7.MatchString(machine.ID) ||
				len(machine.Key) != ed25519.PrivateKeySize || !bytes.Equal(machine.ServerKey, f.serverKey) {
				t.Fatal("The scripted machine did not join and keep the pinned identity.")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()
			s, w, err := machine.Connect(ctx, client)
			if err != nil || s == nil || w == nil || w.GetLinkRevision() != LinkRevision ||
				w.GetState().GetApproval() != linkv1.Approval_APPROVAL_WAITING || w.GetConnectionToken() != "" {
				t.Fatal("The scripted machine did not complete the signed handshake on the generated route.")
			}
			defer func() {
				cancel()
				_ = s.CloseRequest()
				_ = s.CloseResponse()
			}()
			presence := f.current(t, machine.ID)
			if presence.Address != "" {
				t.Error("Admission stored an address on the private stack.")
			}
			awaitHeld(t, f.hub, machine.ID, presence.StreamID)

			msg, err := machines.DecodeToken(token.Text)
			if err != nil {
				t.Fatal("The scripted test token could not be decoded.")
			}
			other, _, err := ed25519.GenerateKey(nil)
			if err != nil {
				t.Fatal("The fixture replay key could not be generated.")
			}
			_, err = client.Join(ctx, connect.NewRequest(&linkv1.JoinRequest{
				TokenId: msg.GetTokenId(), TokenSecret: msg.GetSecret(), HostName: "other station", MachinePublicKey: other,
			}))
			var ce *connect.Error
			if !errors.As(err, &ce) || ce.Code() != connect.CodePermissionDenied ||
				ce.Meta().Get("Lgtm-Join-Refusal") != "token_used" {
				t.Error("The second key did not receive the fixed used-token refusal on the generated route.")
			}
		})
	}
}
