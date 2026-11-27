package link

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"google.golang.org/protobuf/proto"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

const (
	peerOrigin    = "http://127.0.0.1"
	fixtureToken  = "fixture-connection-token-private"
	fixtureSecret = "fixture-secret-private"
)

func TestRetryBackoffAndSilence(t *testing.T) {
	t.Run("targets are measured from start and reset after a settled stream", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			origin := time.Now()
			type step struct {
				start   time.Duration
				work    time.Duration
				welcome bool
			}
			steps := []step{
				{start: 0, work: 400 * time.Millisecond},
				{start: time.Second, work: 1500 * time.Millisecond},
				{start: 3 * time.Second, work: time.Second},
				{start: 7 * time.Second, work: 5 * time.Second},
				{start: 15 * time.Second, work: 3 * time.Second},
				{start: 30 * time.Second, work: 29 * time.Second, welcome: true},
				{start: 59 * time.Second, work: 200 * time.Millisecond},
				{start: 74 * time.Second, work: 30 * time.Second, welcome: true},
				{start: 104 * time.Second, work: 200 * time.Millisecond},
				{start: 105 * time.Second, work: 200 * time.Millisecond},
			}
			starts := make(chan time.Duration, len(steps)+1)
			welcomeEnd := make(chan time.Duration, 2)
			done := make(chan error, 1)
			go func() {
				index := 0
				done <- supervise(ctx, func(attemptCtx context.Context, welcomed func()) error {
					start := time.Since(origin)
					starts <- start
					if index >= len(steps) {
						cancel()
						return context.Canceled
					}
					s := steps[index]
					index++
					if s.welcome {
						welcomed()
					}
					select {
					case <-attemptCtx.Done():
					case <-time.After(s.work):
					}
					if s.welcome {
						welcomeEnd <- time.Since(origin) - start
					}
					return io.EOF
				})
			}()

			for i, want := range steps {
				got := receiveWithin(t, starts, 31*time.Second, "a scheduled attempt did not start")
				if got != want.start {
					t.Errorf("attempt %d started at %v, want %v", i+1, got, want.start)
				}
			}
			for _, want := range []time.Duration{29 * time.Second, 30 * time.Second} {
				if got := receiveNow(t, welcomeEnd, "the welcomed stream did not finish"); got != want {
					t.Errorf("the welcomed stream lasted %v, want %v", got, want)
				}
			}
			time.Sleep(200 * time.Millisecond)
			synctest.Wait()
			cancel()
			receiveNow(t, done, "the supervisor did not stop when its retry wait was cancelled")
			requireEmpty(t, starts, "a cancelled supervisor started another attempt")
		})
	})

	t.Run("blocked dial respects the teardown budget", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			origin := time.Now()
			type observation struct {
				when   time.Duration
				active int32
			}
			var active atomic.Int32
			starts := make(chan observation, 2)
			closed := make(chan time.Duration, 2)
			done := make(chan error, 1)
			go func() {
				done <- supervise(ctx, func(attemptCtx context.Context, _ func()) error {
					n := active.Add(1)
					defer active.Add(-1)
					starts <- observation{time.Since(origin), n}
					<-attemptCtx.Done()
					closed <- time.Since(origin)
					select {
					case <-ctx.Done():
					case <-time.After(time.Second):
					}
					return attemptCtx.Err()
				})
			}()

			if got := receiveNow(t, starts, "the blocked attempt did not start"); got != (observation{0, 1}) {
				t.Fatalf("first attempt started at %v with %d active", got.when, got.active)
			}
			time.Sleep(14*time.Second - time.Nanosecond)
			requireEmpty(t, closed, "the supervisor cancelled before the teardown window")
			time.Sleep(time.Nanosecond)
			if got := receiveNow(t, closed, "the attempt did not cancel at 14s"); got != 14*time.Second {
				t.Errorf("attempt cancelled at %v, want 14s", got)
			}
			requireEmpty(t, starts, "a new attempt overlapped teardown")
			time.Sleep(time.Second)
			if got := receiveNow(t, starts, "the next attempt did not start at 15s"); got != (observation{15 * time.Second, 1}) {
				t.Errorf("next attempt started at %v with %d active, want 15s and one active", got.when, got.active)
			}
			cancel()
			receiveNow(t, done, "cancellation did not join the unfinished attempt")
			if active.Load() != 0 {
				t.Error("the supervisor returned with an attempt still active")
			}
		})
	})

	t.Run("silent proof shares the dial budget", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			peer, client, server, cred := newWirePeer(t, protocols[0])
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			origin := time.Now()
			starts := make(chan time.Duration, 2)
			done := make(chan error, 1)
			go func() {
				done <- supervise(ctx, func(ctx context.Context, welcomed func()) error {
					starts <- time.Since(origin)
					session, err := openSession(ctx, client, server, cred, fixtureRunIdentity)
					if err != nil {
						return err
					}
					defer session.close()
					welcomed()
					return nil
				})
			}()
			first := receiveNow(t, peer.opens, "the silent handshake attempt did not open a stream")
			hello := first.hello(t)
			time.Sleep(8 * time.Second)
			peer.prove(t, first, hello)
			time.Sleep(6*time.Second - time.Nanosecond)
			requireEmpty(t, first.closed, "the client abandoned the proof before the attempt budget ran out")
			time.Sleep(time.Nanosecond)
			receiveNow(t, first.closed, "the client kept waiting for welcome after the 14-second deadline")
			second := receiveWithin(t, peer.opens, time.Second, "the client did not open another stream by 15s")
			second.hello(t)
			receiveNow(t, starts, "the first attempt start was not recorded")
			if got := receiveNow(t, starts, "the second attempt start was not recorded"); got > 15*time.Second {
				t.Errorf("the second attempt started after %v, want at most 15s", got)
			}
			cancel()
			receiveNow(t, done, "the supervisor did not join the cancelled handshake")
		})
	})
}

func receiveWithin[T any](t *testing.T, c <-chan T, limit time.Duration, missing string) T {
	t.Helper()
	select {
	case value := <-c:
		return value
	case <-time.After(limit + time.Nanosecond):
		t.Fatal(missing)
		var zero T
		return zero
	}
}

func receiveNow[T any](t *testing.T, c <-chan T, missing string) T {
	t.Helper()
	synctest.Wait()
	select {
	case value := <-c:
		return value
	default:
		t.Fatal(missing)
		var zero T
		return zero
	}
}

func requireEmpty[T any](t *testing.T, c <-chan T, unexpected string) {
	t.Helper()
	synctest.Wait()
	select {
	case <-c:
		t.Fatal(unexpected)
	default:
	}
}

// wirePeer leaves all control of each server stream with the test. The
// generated client and handler exchange real frames on an in-memory HTTP/2 connection.
type wirePeer struct {
	linkv1connect.UnimplementedLinkServiceHandler
	opens chan *wireLeg
	keys  fixtureKeys
}

type wireLeg struct {
	stream   *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]
	refuse   chan error
	closed   chan struct{}
	work     chan func()
	hasToken bool
}

func (p *wirePeer) Open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	leg := &wireLeg{
		stream: stream, refuse: make(chan error, 1), closed: make(chan struct{}),
		work: make(chan func()), hasToken: stream.RequestHeader().Get("Lgtm-Link-Token") != "",
	}
	defer close(leg.closed)
	p.opens <- leg
	for {
		select {
		case work := <-leg.work:
			work()
		case err := <-leg.refuse:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func newWirePeer(t *testing.T, p protocol) (*wirePeer, linkv1connect.LinkServiceClient, enroll.Server, *enroll.Credential) {
	t.Helper()
	keys := fixtureKeysOf(t)
	peer := &wirePeer{opens: make(chan *wireLeg, 32), keys: keys}
	_, handler := linkv1connect.NewLinkServiceHandler(peer)
	h1 := &http.Server{ErrorLog: log.New(io.Discard, "", 0)}
	h2 := new(http2.Server)
	// ConfigureServer gives this peer its own pool of channels inside the bubble.
	if err := http2.ConfigureServer(h1, h2); err != nil {
		t.Fatal("could not configure the in-memory server")
	}
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, _, _ string, _ *tls.Config) (net.Conn, error) {
			client, server := net.Pipe()
			stop := context.AfterFunc(t.Context(), func() {
				_ = client.Close()
				_ = server.Close()
			})
			go func() {
				defer stop()
				defer client.Close()
				defer server.Close()
				h2.ServeConn(server, &http2.ServeConnOpts{
					Handler:    handler,
					BaseConfig: h1,
				})
			}()
			return client, nil
		},
	}
	t.Cleanup(transport.CloseIdleConnections)
	opts := append([]connect.ClientOption{connect.WithReadMaxBytes(64 << 10)}, p.opts...)
	client := linkv1connect.NewLinkServiceClient(&http.Client{Transport: transport}, peerOrigin, opts...)
	server := enroll.Server{
		Address:         peerOrigin,
		MachineID:       keys.machineID,
		PublicKey:       keys.serverKey.Public().(ed25519.PublicKey),
		ClientPublicKey: keys.machineKey.Public().(ed25519.PublicKey),
	}
	return peer, client, server, credentialOf(t, keys.machineKey)
}

type wireRead struct {
	frame *linkv1.OpenRequest
	err   error
}

func (l *wireLeg) read() <-chan wireRead {
	result := make(chan wireRead, 1)
	go func() {
		select {
		case l.work <- func() {
			frame, err := l.stream.Receive()
			result <- wireRead{frame, err}
		}:
		case <-l.closed:
			result <- wireRead{err: io.EOF}
		}
	}()
	return result
}

func (l *wireLeg) next(t *testing.T) *linkv1.OpenRequest {
	t.Helper()
	read := receiveNow(t, l.read(), "the client did not send the expected frame")
	if read.err != nil {
		t.Fatal("the client stream closed before sending the expected frame")
	}
	return read.frame
}

func (l *wireLeg) send(t *testing.T, frame *linkv1.OpenResponse) {
	t.Helper()
	result := make(chan error, 1)
	select {
	case l.work <- func() { result <- l.stream.Send(frame) }:
		if err := receiveNow(t, result, "the fixture could not send the scripted frame"); err != nil {
			t.Fatal("the fixture could not send the scripted frame")
		}
	case <-l.closed:
		t.Fatal("the stream closed before the fixture could send the scripted frame")
	}
}

func (l *wireLeg) hello(t *testing.T) *linkv1.Hello {
	t.Helper()
	if l.hasToken {
		t.Error("the new stream sent a connection token")
	}
	hello := l.next(t).GetHello()
	if hello == nil || hello.GetLinkRevision() != linkRevision {
		t.Fatal("the first frame is missing a hello of the active link revision")
	}
	return hello
}

func (p *wirePeer) prove(t *testing.T, l *wireLeg, hello *linkv1.Hello) {
	t.Helper()
	s := scene{keys: p.keys}
	l.send(t, s.challenge(hello, fixedServerNonce))
	if !s.verifiesProof(hello, fixedServerNonce, l.next(t).GetProof()) {
		t.Fatal("the machine did not send a valid proof")
	}
}

func peerError(code connect.Code, key, reason string, extra ...string) error {
	text := strings.Join(append([]string{fixtureToken, fixtureSecret}, extra...), " ")
	err := connect.NewError(code, errors.New(text))
	if key != "" && reason != "" {
		err.Meta().Set(key, reason)
	}
	return err
}

func checkNoFixtureSecrets(t *testing.T, text string, extras ...string) {
	t.Helper()
	for _, value := range append([]string{fixtureToken, fixtureSecret}, extras...) {
		if value != "" && strings.Contains(text, value) {
			t.Error("client output disclosed fixture credentials")
			return
		}
	}
}

type retryCodeRow struct {
	Call     string   `json:"call"`
	Code     string   `json:"code"`
	Action   string   `json:"action"`
	Key      string   `json:"key"`
	Reasons  []string `json:"reasons"`
	Verified bool     `json:"after_verified_challenge"`
}

func TestRetryCodeTable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "testdata", "link-codes.json"))
	if err != nil {
		t.Fatal("could not read the frozen code table")
	}
	var rows []retryCodeRow
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		t.Fatal("could not decode the frozen code table")
	}
	rows = append(rows,
		retryCodeRow{Call: "Open", Code: "out_of_range", Action: "dial again", Verified: true},
		retryCodeRow{Call: "Open", Code: "failed_precondition", Action: "dial again", Verified: true},
		retryCodeRow{Call: "Open", Code: "failed_precondition", Action: "dial again", Verified: true, Key: "Lgtm-Link-Refusal", Reasons: []string{"fixture_unrecognized"}},
	)
	for i, row := range rows {
		if row.Action != "stop" && row.Action != "dial again" {
			t.Fatalf("row %d has an unknown action", i+1)
		}
		var code connect.Code
		if err := code.UnmarshalText([]byte(row.Code)); err != nil {
			t.Fatalf("row %d has an unknown status code", i+1)
		}
		if row.Call == "Join" || row.Call == "both" {
			t.Run(fmt.Sprintf("%02d_Join_%s", i+1, row.Code), func(t *testing.T) {
				checkJoinCodeRow(t, row, code)
			})
		}
		if row.Call != "Open" && row.Call != "both" {
			if row.Call != "Join" {
				t.Fatalf("row %d has an unknown call", i+1)
			}
			continue
		}
		reasons := row.Reasons
		if len(reasons) == 0 {
			reasons = []string{""}
		}
		for j, reason := range reasons {
			t.Run(fmt.Sprintf("%02d_Open_%s_%d", i+1, row.Code, j+1), func(t *testing.T) {
				checkOpenCodeRow(t, row, code, reason, row.Verified, row.Action == "stop")
				if row.Verified && row.Action == "stop" {
					checkOpenCodeRow(t, row, code, reason, false, false)
				}
			})
		}
	}
}

func checkOpenCodeRow(t *testing.T, row retryCodeRow, code connect.Code, reason string, verified, stop bool) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		peer, client, server, cred := newWirePeer(t, protocols[0])
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			done <- supervise(ctx, func(ctx context.Context, welcomed func()) error {
				s, err := openSession(ctx, client, server, cred, fixtureRunIdentity)
				if err != nil {
					return err
				}
				defer s.close()
				welcomed()
				_, err = s.receive()
				return err
			})
		}()
		leg := receiveNow(t, peer.opens, "the supervisor did not dial the scripted peer")
		hello := leg.hello(t)
		if verified {
			peer.prove(t, leg, hello)
		}
		leg.refuse <- peerError(code, row.Key, reason)
		if stop {
			err := receiveNow(t, done, "the verified refusal did not stop the supervisor")
			var refusal terminalRefusal
			if !errors.As(err, &refusal) || refusal.Code != reason {
				t.Error("the supervisor did not return the verified final reason")
			}
			if err != nil {
				checkNoFixtureSecrets(t, err.Error())
			}
			time.Sleep(15 * time.Second)
			requireEmpty(t, peer.opens, "the supervisor dialled after a verified final refusal")
		} else {
			second := receiveWithin(t, peer.opens, 15*time.Second, "the supervisor did not dial again after a transient refusal")
			second.hello(t)
			cancel()
			receiveNow(t, done, "the supervisor did not stop after cancellation")
		}
	})
}

type retryJoinPeer struct {
	linkv1connect.UnimplementedLinkServiceHandler
	calls atomic.Int32
	reply *linkv1.JoinResponse
	err   error
}

func (p *retryJoinPeer) Join(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	if p.calls.Add(1) == 1 {
		return nil, p.err
	}
	return connect.NewResponse(p.reply), nil
}

func checkJoinCodeRow(t *testing.T, row retryCodeRow, code connect.Code) {
	t.Helper()
	folder, _ := newState(t)
	keys := fixtureKeysOf(t)
	public := keys.serverKey.Public().(ed25519.PublicKey)
	digest := sha256.Sum256(public)
	secret := make([]byte, 32)
	copy(secret, fixtureSecret)
	peer := &retryJoinPeer{
		reply: &linkv1.JoinResponse{MachineId: keys.machineID, ServerPublicKey: public},
		err:   peerError(code, row.Key, "token_expired"),
	}
	_, handler := linkv1connect.NewLinkServiceHandler(peer)
	server := httptest.NewUnstartedServer(handler)
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.Start()
	t.Cleanup(server.Close)
	raw, err := proto.Marshal(&linkv1.JoinToken{
		TokenId: uid(1), Secret: secret, Address: server.URL, ServerKeySha256: digest[:],
	})
	if err != nil {
		t.Fatal("could not encode the fixture join token")
	}
	token := "lgtm1_" + base64.RawURLEncoding.EncodeToString(raw)
	ctx, cancel := context.WithTimeout(t.Context(), openCeiling)
	defer cancel()
	retries := 0
	_, err = enroll.Join(ctx, folder, server.URL, token, "", func() { retries++ })
	if err != nil {
		checkNoFixtureSecrets(t, err.Error(), token, hex.EncodeToString(secret), base64.StdEncoding.EncodeToString(secret))
	}
	switch row.Action {
	case "stop":
		var refusal enroll.JoinError
		if !errors.As(err, &refusal) || peer.calls.Load() != 1 || retries != 0 {
			t.Error("the join did not stop on the first refusal")
		}
	case "dial again":
		if err != nil || peer.calls.Load() != 2 || retries != 1 {
			t.Error("the join did not retry once and accept the next reply")
		}
	}
}

func TestRetryFailedElapsedReadLeavesNoExpiryBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer, client, server, cred := newWirePeer(t, protocols[0])
		old := readElapsedClock
		var calls atomic.Int32
		readElapsedClock = func() (time.Duration, error) {
			calls.Add(1)
			return time.Minute, errors.New("the elapsed clock could not be read")
		}
		t.Cleanup(func() { readElapsedClock = old })

		type handshake struct {
			session *session
			err     error
		}
		done := make(chan handshake, 1)
		go func() {
			s, err := openSession(t.Context(), client, server, cred, fixtureRunIdentity)
			done <- handshake{s, err}
		}()
		leg := receiveNow(t, peer.opens, "the client did not start the handshake")
		peer.prove(t, leg, leg.hello(t))
		leg.send(t, welcomeFrame(goodWelcome()))
		got := receiveNow(t, done, "the proven session did not open")
		if got.err != nil || got.session == nil {
			t.Fatal("a failed elapsed reading prevented a valid welcome")
		}
		defer got.session.close()
		if calls.Load() != 1 {
			t.Error("the proof send did not perform exactly one injected elapsed reading")
		}
		if got.session.sentAtOK {
			t.Error("the failed proof sample was marked usable")
		}
		h := newCommandHandler(nil, nil, nil, func() (time.Duration, error) { return got.session.sentAt + time.Second, nil }, nil)
		h.setSession(got.session)
		if !h.clock.upperBound().IsZero() {
			t.Error("the failed proof sample gave the session an expiry bound")
		}
	})
}
