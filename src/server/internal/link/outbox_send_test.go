package link

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type testCommandWireKey struct{}

type commandWireProbe struct {
	afterClaim func(context.Context, *linkv1.Command) error
	controls   *streamControls
	flushing   chan commandFlush
	closed     chan struct{}
	failFlush  bool
	didFlush   atomic.Bool
}

type commandFlush struct {
	deadline, entered time.Time
	frame             *linkv1.Command
	done              <-chan error
}

func newCommandWireProbe() *commandWireProbe {
	return &commandWireProbe{flushing: make(chan commandFlush, 8), closed: make(chan struct{})}
}

type commandProbeWriter struct {
	http.ResponseWriter
	probe    *commandWireProbe
	mu       sync.Mutex
	deadline time.Time
	size     int
	command  *linkv1.Command
}

func (w *commandProbeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *commandProbeWriter) SetWriteDeadline(at time.Time) error {
	w.mu.Lock()
	w.deadline = at
	w.mu.Unlock()
	return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(at)
}

func (w *commandProbeWriter) Write(p []byte) (int, error) {
	// The pinned protocol writes the five-byte envelope prefix and payload separately.
	if w.size != 0 {
		frame := new(linkv1.OpenResponse)
		if len(p) == w.size && proto.Unmarshal(p, frame) == nil {
			w.command = frame.GetCommand()
		}
		w.size = 0
	} else if len(p) == 5 && p[0] == 0 {
		w.size = int(binary.BigEndian.Uint32(p[1:]))
	}
	return w.ResponseWriter.Write(p)
}

func (w *commandProbeWriter) FlushError() error {
	command := w.command
	w.command = nil
	if command == nil {
		return http.NewResponseController(w.ResponseWriter).Flush()
	}
	w.mu.Lock()
	deadline := w.deadline
	w.mu.Unlock()
	done := make(chan error, 1)
	w.probe.flushing <- commandFlush{deadline, time.Now(), command, done}
	var err error
	if w.probe.failFlush {
		err = errors.New("The fixture interrupted the command flush.")
	} else {
		err = http.NewResponseController(w.ResponseWriter).Flush()
	}
	if err == nil {
		w.probe.didFlush.Store(true)
	}
	done <- err
	return err
}

func commandListener(t *testing.T, f *handshakeFixture, probes map[string]*commandWireProbe) *httptest.Server {
	t.Helper()
	previous := f.svc.streaming
	options := previous.options
	options.Commands = f.state
	f.svc = NewStreamingService(previous.life, f.s, f.tokens, f.r, f.state, f.hub, previous.key, options)
	route := api.NewHandler(api.Options{Link: f.svc})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := probes[r.Header.Get("X-Command-Fixture")]; p != nil {
			defer close(p.closed)
			ctx := context.WithValue(r.Context(), testCommandWireKey{}, p)
			if p.afterClaim != nil {
				ctx = context.WithValue(ctx, afterCommandClaimKey{}, p.afterClaim)
			}
			if p.controls != nil {
				ctx = context.WithValue(ctx, streamControlsKey{}, p.controls)
			}
			w, r = &commandProbeWriter{ResponseWriter: w, probe: p}, r.WithContext(ctx)
		}
		route.ServeHTTP(w, r)
	})
	server := httptest.NewUnstartedServer(handler)
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func watchCommandFlush(t *testing.T, p *commandWireProbe) commandFlush {
	t.Helper()
	select {
	case flush := <-p.flushing:
		return flush
	case <-p.closed:
		t.Fatal("Open ended before the claimed command reached the wrapped writer's flush.")
	case <-time.After(4 * time.Second):
		t.Fatal("The claimed command did not reach the wrapped writer's flush.")
	}
	return commandFlush{}
}

func openCommandStream(t *testing.T, f *handshakeFixture, client linkv1connect.LinkServiceClient, m machines.Machine, key ed25519.PrivateKey, label string) *openClientStream {
	t.Helper()
	s, _ := openCancellableCommandStream(t, f, client, m, key, label)
	return s
}

func openCancellableCommandStream(t *testing.T, f *handshakeFixture, client linkv1connect.LinkServiceClient, m machines.Machine, key ed25519.PrivateKey, label string) (*openClientStream, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	s := client.Open(ctx)
	s.RequestHeader().Set("X-Command-Fixture", label)
	t.Cleanup(func() {
		cancel()
		_ = s.CloseRequest()
		_ = s.CloseResponse()
	})
	submitProof(t, f, s, m.ID, key, 1)
	requireWelcome(t, s)
	return s, cancel
}

func queueSendRestart(t *testing.T, f *presenceFixture, machine string, remaining time.Duration) claimFixtureRow {
	t.Helper()
	var row claimFixtureRow
	err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.r.Lock(ctx, tx, machine); err != nil {
			return err
		}
		if err := f.state.Enqueue(ctx, tx, machine, restartAction(), 2*time.Minute); err != nil {
			return err
		}
		if remaining != 0 {
			return tx.QueryRow(ctx, `WITH sampled AS (SELECT clock_timestamp() AS at)
				UPDATE link_commands SET expires_at = sampled.at - interval '23 hours'
					+ $2::bigint * interval '1 microsecond', created_at = sampled.at - interval '24 hours'
				FROM sampled WHERE id = (SELECT id FROM link_commands WHERE machine_id = $1 ORDER BY seq DESC LIMIT 1)
				RETURNING id, expires_at`, machine, remaining.Microseconds()).Scan(&row.id, &row.expires)
		}
		return tx.QueryRow(ctx, `SELECT id, expires_at FROM link_commands
			WHERE machine_id = $1 ORDER BY seq DESC LIMIT 1`, machine).Scan(&row.id, &row.expires)
	})
	if err != nil {
		t.Fatal("The production queue did not save the fixture restart.")
	}
	row.action = restartAction()
	return row
}

func requireMachineWritable(t *testing.T, f *presenceFixture, machineID string) {
	t.Helper()
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		_, err := f.r.TryLock(ctx, tx, machineID)
		return err
	}); err != nil {
		t.Fatal("The finished command write did not release the machine lock.")
	}
}

func requireDispatch(t *testing.T, f *presenceFixture, row claimFixtureRow, streamID string, attempt int, dispatched bool) {
	t.Helper()
	if !storetest.Scalar[bool](t, f.obs, `SELECT sent_stream_id = $2 AND attempt = $3 AND dispatched = $4
		AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL FROM link_commands WHERE id = $1`,
		row.id, streamID, attempt, dispatched) {
		t.Error("The separate database observer did not see the expected pending attempt and dispatch marker.")
	}
}

func TestCommandSendRechecksBeforeWrite(t *testing.T) {
	for _, mode := range []string{"removed", "replaced", "past cutoff", "dispatched", "failed flush", "lost marker commit reply"} {
		t.Run(mode, func(t *testing.T) {
			probe := newCommandWireProbe()
			probe.failFlush = mode == "failed flush"
			claim, resumeClaim := storetest.Signal(t)
			defer resumeClaim()
			claimed := make(chan *linkv1.Command, 1)
			probe.afterClaim = func(ctx context.Context, cmd *linkv1.Command) error {
				claimed <- cmd
				select {
				case <-claim:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			marker, resumeMarker := storetest.Signal(t)
			defer resumeMarker()
			atMarker, replyHeld := make(chan struct{}, 1), make(chan struct{}, 1)
			f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
				if mode == "lost marker commit reply" {
					dial := cfg.ConnConfig.DialFunc
					cfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
						conn, err := dial(ctx, network, addr)
						if err != nil {
							return nil, err
						}
						return &admissionReplyConn{Conn: conn, arrived: replyHeld, release: marker}, nil
					}
				}
				cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
					req, _ := ctx.Value(testCommandWireKey{}).(*commandWireProbe)
					if req != probe || !strings.EqualFold(strings.TrimSpace(q.SQL), "commit") ||
						!probe.didFlush.CompareAndSwap(true, false) {
						return ctx
					}
					if mode == "lost marker commit reply" {
						if pipe, ok := conn.PgConn().Conn().(*admissionReplyConn); ok {
							pipe.armed.Store(true)
						}
					}
					atMarker <- struct{}{}
					if mode == "dispatched" {
						select {
						case <-marker:
						case <-ctx.Done():
						}
					}
					return ctx
				})
			}})
			m, key := f.register(t, "command fence")
			f.approve(t, m.ID)
			row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
			server := commandListener(t, f, map[string]*commandWireProbe{"first": probe})
			client := linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), server.URL)
			s := openCommandStream(t, f, client, m, key, "first")
			stream := f.current(t, m.ID).StreamID
			select {
			case cmd := <-claimed:
				row.requireFrame(t, cmd, 1)
			case <-probe.closed:
				t.Fatal("Open ended before observing a claim committed ahead of its send fence.")
			case <-time.After(3 * time.Second):
				t.Fatal("Open did not commit the queued attempt and reach the post-claim barrier.")
			}
			requireDispatch(t, f.presenceFixture, row, stream, 1, false)
			result := commandResult(row.id, 1, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE)
			requireResultError(t, f.presenceFixture, m.ID, stream, result, nil)
			requirePendingResult(t, f.presenceFixture, row.id)

			switch mode {
			case "removed":
				if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
					if err := f.r.Delete(ctx, tx, m.ID); err != nil {
						return err
					}
					return f.state.Forget(ctx, tx, m.ID)
				}); err != nil {
					t.Fatal("The fixture could not commit removal in the claim gap.")
				}
			case "replaced":
				replacement := fixtureStreamID(t)
				err := checkOneLine(t, f.presenceFixture, func(ctx context.Context) error {
					_, err := f.state.BecomeCurrent(ctx, m.ID, replacement, presenceAddress)
					return err
				}, func(ctx context.Context) error {
					return f.state.AcceptResult(ctx, m.ID, stream, result)
				}, false)
				if !errors.Is(err, ErrNotCurrent) {
					t.Error("A result that waited for replacement did not see the new stream fence.")
				}
			case "past cutoff":
				if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
					if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
						return err
					}
					_, err := tx.Exec(ctx, `UPDATE link_commands SET created_at = now() - interval '25 hours',
						expires_at = now() - interval '24 hours' WHERE id = $1`, row.id)
					return err
				}); err != nil {
					t.Fatal("The fixture could not move the send cutoff past the claimed attempt.")
				}
			}
			resumeClaim()

			switch mode {
			case "removed", "replaced":
				if frame, err := s.Receive(); frame != nil || err == nil || connect.CodeOf(err) == connect.CodeDeadlineExceeded {
					t.Fatal("A displaced claim sent a frame or failed to end promptly.")
				}
			case "past cutoff":
				sendHandshake(t, s, heartbeatFrame(nil, nil))
				requireHeartbeatAck(t, s)
				requireResultError(t, f.presenceFixture, m.ID, stream, result, nil)
				requireDispatch(t, f.presenceFixture, row, stream, 1, false)
			case "dispatched":
				flush := watchCommandFlush(t, probe)
				row.requireFrame(t, flush.frame, 1)
				if storetest.Receive(t, "command flush completion", flush.done) != nil {
					t.Fatal("The eligible command did not flush.")
				}
				storetest.Receive(t, "the dispatch marker commit", atMarker)
				frame, err := s.Receive()
				if err != nil || frame.GetCommand() == nil {
					t.Fatal("The eligible command did not reach the peer before its marker commit.")
				}
				row.requireFrame(t, frame.GetCommand(), 1)
				requireDispatch(t, f.presenceFixture, row, stream, 1, false)
				var lockErr *store.Error
				err = f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
					_, err := f.r.TryLock(ctx, tx, m.ID)
					return err
				})
				if !errors.As(err, &lockErr) || lockErr.SQLState != "55P03" {
					t.Error("The command writer released its machine lock before its dispatch commit.")
				}
				resumeMarker()
				storetest.Poll(t, "the committed dispatch marker", func() bool {
					return storetest.Scalar[bool](t, f.obs, "SELECT dispatched FROM link_commands WHERE id = $1", row.id)
				})
				sendHandshake(t, s, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: result}})
				sendHandshake(t, s, heartbeatFrame(nil, nil))
				requireHeartbeatAck(t, s)
				stored := readCommandResult(t, f.presenceFixture, row.id)
				if stored.outcome != "done" || stored.finished == nil || stored.hasPayload {
					t.Error("The dispatched wire attempt did not save its final result and clear the payload.")
				}
				if flush.deadline.IsZero() || flush.deadline.Sub(flush.entered) > 2*time.Second {
					t.Fatal("The successful command did not use the short write deadline.")
				}
				timer := time.NewTimer(time.Until(flush.deadline))
				defer timer.Stop()
				<-timer.C
				sendHandshake(t, s, heartbeatFrame(nil, nil))
				requireHeartbeatAck(t, s)
			case "failed flush", "lost marker commit reply":
				flush := watchCommandFlush(t, probe)
				row.requireFrame(t, flush.frame, 1)
				failed := storetest.Receive(t, "the command flush returning", flush.done) != nil
				if failed != probe.failFlush {
					t.Fatal("The wrapped writer did not observe the planned flush result.")
				}
				if mode == "lost marker commit reply" {
					storetest.Receive(t, "the dispatch marker commit", atMarker)
					storetest.Receive(t, "the committed marker's reply at the wire gate", replyHeld)
					requireDispatch(t, f.presenceFixture, row, stream, 1, true)
					resumeMarker()
				}
				storetest.Receive(t, "the stream ending after the failed send or commit reply", probe.closed)
				requireDispatch(t, f.presenceFixture, row, stream, 1, !failed)
				requireMachineWritable(t, f.presenceFixture, m.ID)
				retry := openCommandStream(t, f, client, m, key, "")
				frame, err := retry.Receive()
				if err != nil || frame.GetCommand() == nil {
					t.Fatal("The replacement stream did not deliver the pending command.")
				}
				row.requireFrame(t, frame.GetCommand(), 2)
			}
		})
	}
}

// commandPeer grants credit for one envelope at a time on its HTTP/2 stream.
type commandPeer struct {
	framer *http2.Framer
}

const commandPeerStream = 1

func openCommandPeer(t *testing.T, f *handshakeFixture, server *httptest.Server, m machines.Machine, key ed25519.PrivateKey, label string) *commandPeer {
	t.Helper()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), storetest.ObservationTimeout)
	if err != nil {
		t.Fatal("The loopback command peer could not connect.")
	}
	t.Cleanup(func() { _ = conn.Close() })
	if conn.SetDeadline(time.Now().Add(storetest.ObservationTimeout)) != nil {
		t.Fatal("The loopback command peer could not bound its I/O.")
	}
	p := &commandPeer{framer: http2.NewFramer(conn, conn)}
	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil ||
		p.framer.WriteSettings(http2.Setting{ID: http2.SettingInitialWindowSize, Val: 0}) != nil {
		t.Fatal("The command peer could not advertise zero initial stream credit.")
	}
	var block bytes.Buffer
	enc := hpack.NewEncoder(&block)
	for _, field := range []hpack.HeaderField{
		{Name: ":method", Value: "POST"},
		{Name: ":scheme", Value: "http"},
		{Name: ":authority", Value: server.Listener.Addr().String()},
		{Name: ":path", Value: linkv1connect.LinkServiceOpenProcedure},
		{Name: "content-type", Value: "application/grpc"},
		{Name: "te", Value: "trailers"},
		{Name: "x-command-fixture", Value: label},
	} {
		if enc.WriteField(field) != nil {
			t.Fatal("The command peer could not encode its request headers.")
		}
	}
	if p.framer.WriteHeaders(http2.HeadersFrameParam{StreamID: commandPeerStream, BlockFragment: block.Bytes(), EndHeaders: true}) != nil {
		t.Fatal("The command peer could not open its request stream.")
	}
	hello := helloFrame(m.ID, 1, 1)
	p.send(t, hello)
	challenge := p.readEnvelope(t).GetChallenge()
	if challenge == nil || !ed25519.Verify(f.serverKey, fixtureProofText("lgtm-link-v1 server", hello.GetHello(), challenge.GetServerNonce()), challenge.GetServerSignature()) {
		t.Fatal("The command peer did not receive a verifiable Challenge.")
	}
	p.send(t, proofFrame(key, hello.GetHello(), challenge))
	if p.readEnvelope(t).GetWelcome() == nil {
		t.Fatal("The command peer did not receive Welcome.")
	}
	storetest.Poll(t, "the peer's Welcome publishing its exact hold", func() bool {
		return f.current(t, m.ID).Held
	})
	return p
}

func (p *commandPeer) send(t *testing.T, frame *linkv1.OpenRequest) {
	t.Helper()
	body, err := proto.Marshal(frame)
	if err != nil {
		t.Fatal("The command peer could not encode its request frame.")
	}
	data := make([]byte, 5, 5+len(body))
	binary.BigEndian.PutUint32(data[1:], uint32(len(body)))
	if p.framer.WriteData(commandPeerStream, false, append(data, body...)) != nil {
		t.Fatal("The command peer could not send its request frame.")
	}
}

func (p *commandPeer) grant(t *testing.T, bytes uint32) {
	t.Helper()
	if p.framer.WriteWindowUpdate(commandPeerStream, bytes) != nil {
		t.Fatal("The command peer could not grant exact stream credit.")
	}
}

func (p *commandPeer) readCredit(t *testing.T, size uint32) []byte {
	t.Helper()
	p.grant(t, size)
	data := make([]byte, 0, size)
	for len(data) < int(size) {
		frame, err := p.framer.ReadFrame()
		if err != nil {
			t.Fatal("The command peer could not read the granted response bytes.")
		}
		switch frame := frame.(type) {
		case *http2.SettingsFrame:
			if !frame.IsAck() && p.framer.WriteSettingsAck() != nil {
				t.Fatal("The command peer could not acknowledge the server settings.")
			}
		case *http2.DataFrame:
			if frame.StreamID != commandPeerStream || len(data)+len(frame.Data()) > int(size) || frame.StreamEnded() {
				t.Fatal("The server exceeded the command peer's stream credit or ended the stream early.")
			}
			data = append(data, frame.Data()...)
			if len(frame.Data()) > 0 && p.framer.WriteWindowUpdate(0, uint32(len(frame.Data()))) != nil {
				t.Fatal("The command peer could not restore connection credit.")
			}
		case *http2.RSTStreamFrame, *http2.GoAwayFrame:
			t.Fatal("The transport ended before the expected command peer frame.")
		}
	}
	return data
}

func (p *commandPeer) readEnvelope(t *testing.T) *linkv1.OpenResponse {
	t.Helper()
	prefix := p.readCredit(t, 5)
	size := binary.BigEndian.Uint32(prefix[1:])
	if prefix[0] != 0 || size == 0 || size > 64<<10 {
		t.Fatal("The command peer did not receive an uncompressed, bounded response envelope.")
	}
	frame := new(linkv1.OpenResponse)
	if proto.Unmarshal(p.readCredit(t, size), frame) != nil {
		t.Fatal("The command peer could not decode its response envelope.")
	}
	return frame
}

func TestCommandSendDeadline(t *testing.T) {
	for _, cutoff := range []bool{false, true} {
		name := "store cap and next attempt"
		if cutoff {
			name = "earlier send cutoff"
		}
		t.Run(name, func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			m, key := f.register(t, "command write deadline")
			f.approve(t, m.ID)
			probe := newCommandWireProbe()
			server := commandListener(t, f, map[string]*commandWireProbe{"held": probe})
			f.startRunner(t)
			openCommandPeer(t, f, server, m, key, "held")
			stream := f.current(t, m.ID).StreamID
			var remaining time.Duration
			if cutoff {
				remaining = 1250 * time.Millisecond
			}
			row := queueSendRestart(t, f.presenceFixture, m.ID, remaining)
			flush := watchCommandFlush(t, probe)
			row.requireFrame(t, flush.frame, 1)
			requireDispatch(t, f.presenceFixture, row, stream, 1, false)
			var lockErr *store.Error
			err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
				_, err := f.r.TryLock(ctx, tx, m.ID)
				return err
			})
			if !errors.As(err, &lockErr) || lockErr.SQLState != "55P03" {
				t.Error("The fenced send did not hold its machine lock during the blocked flush.")
			}
			if !flush.deadline.After(flush.entered) || flush.deadline.Sub(flush.entered) > 2*time.Second {
				t.Error("The blocked flush did not have a live deadline within the two-second write budget.")
			}
			if cutoff {
				drift := flush.deadline.Sub(row.expires.Add(23 * time.Hour))
				if drift < -200*time.Millisecond || drift > 200*time.Millisecond {
					t.Error("The transport did not arm the earlier database send cutoff.")
				}
			}
			select {
			case <-flush.done:
				t.Fatal("The real flush returned before the peer granted command credit or its deadline elapsed.")
			default:
			}
			if err := storetest.Receive(t, "the flow-controlled flush ending", flush.done); err == nil ||
				time.Now().Before(flush.deadline) || time.Since(flush.deadline) > time.Second {
				t.Error("The flow-controlled flush did not fail when its shorter deadline elapsed.")
			}
			storetest.Receive(t, "Open closing after the write deadline", probe.closed)
			requireDispatch(t, f.presenceFixture, row, stream, 1, false)
			requireMachineWritable(t, f.presenceFixture, m.ID)
			if !cutoff {
				client := linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), server.URL)
				s := openCommandStream(t, f, client, m, key, "")
				frame, err := s.Receive()
				if err != nil || frame.GetCommand() == nil {
					t.Fatal("The unexpired command did not recover on a replacement stream.")
				}
				row.requireFrame(t, frame.GetCommand(), 2)
			}
		})
	}
}

func TestCommandBacklogYieldsToHeartbeats(t *testing.T) {
	const prefixPages = 20
	probe := newCommandWireProbe()
	received := make(chan struct{}, 20)
	probe.controls = &streamControls{afterReceive: func(frame *linkv1.OpenRequest, err error) {
		if err == nil && frame.GetHeartbeat() != nil {
			received <- struct{}{}
		}
	}}
	pageEntered := make(chan struct{}, 1)
	hold, release := storetest.Signal(t)
	defer release()
	var pages, firstHeartbeatAt atomic.Int32
	firstHeartbeatAt.Store(-1)
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
			if ctx.Value(testCommandWireKey{}) != probe {
				return ctx
			}
			sql := strings.ToLower(q.SQL)
			if strings.Contains(sql, "from link_commands") && strings.Contains(sql, "order by seq") && pages.Add(1) == 1 {
				pageEntered <- struct{}{}
				select {
				case <-hold:
				case <-ctx.Done():
				}
			}
			if strings.Contains(sql, "update link_connections") && strings.Contains(sql, "last_heard_at") {
				firstHeartbeatAt.CompareAndSwap(-1, pages.Load())
			}
			return ctx
		})
	}})
	m, key := f.register(t, "command backlog")
	f.approve(t, m.ID)
	outboxFixtureExec(t, f.obs, `INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
		SELECT 'send-prefix-' || n, $1, 'restart', $3, now() - interval '2 days', now() - interval '25 hours'
		FROM generate_series(1, $2::int) n ORDER BY n`, m.ID, prefixPages*32, []byte{})
	row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	server := commandListener(t, f, map[string]*commandWireProbe{"backlog": probe})
	client := linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), server.URL)
	s := openCommandStream(t, f, client, m, key, "backlog")
	select {
	case <-pageEntered:
	case <-probe.closed:
		t.Fatal("Open ended before examining the first ineligible command page.")
	case <-time.After(3 * time.Second):
		t.Fatal("The writer did not examine its first ineligible command page.")
	}
	for range 17 {
		sendHandshake(t, s, heartbeatFrame(nil, nil))
		storetest.Receive(t, "a heartbeat decoded while the first claim page is held", received)
	}
	sendHandshake(t, s, heartbeatFrame(nil, nil))
	select {
	case <-received:
		t.Fatal("The reader crossed the receive barrier while its sixteen slots and pending send were full.")
	case <-time.After(100 * time.Millisecond):
	}
	release()
	acks, commands := 0, 0
	for range 19 {
		frame, err := s.Receive()
		switch {
		case err != nil:
			t.Fatalf("The writer stopped with %d heartbeat acknowledgements and %d tail commands.", acks, commands)
		case frame.GetHeartbeatAck() != nil:
			acks++
		case frame.GetCommand() != nil:
			row.requireFrame(t, frame.GetCommand(), 1)
			commands++
		default:
			t.Fatal("The writer sent an unexpected frame while draining the backlog.")
		}
	}
	if firstHeartbeatAt.Load() != 1 {
		t.Error("The writer did not commit a ready heartbeat between the first and second ineligible pages.")
	}
	if acks != 18 || commands != 1 {
		t.Error("The writer did not acknowledge every heartbeat and wake itself to deliver the tail command once.")
	}
	if pages.Load() < prefixPages+1 {
		t.Error("The writer returned the tail without examining each bounded prefix page.")
	}
}

func TestCommandSendPoolHeadroom(t *testing.T) {
	probes := map[string]*commandWireProbe{}
	for _, label := range []string{"first", "second", "third", "healthy", "canceled waiter", "live waiter", "cleanup"} {
		probes[label] = newCommandWireProbe()
	}
	var canceledPages, livePages atomic.Int32
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
		cfg.MaxConns = 8
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
			sql := strings.ToLower(q.SQL)
			if strings.Contains(sql, "from link_commands") && strings.Contains(sql, "order by seq") {
				switch ctx.Value(testCommandWireKey{}) {
				case probes["canceled waiter"]:
					canceledPages.Add(1)
				case probes["live waiter"]:
					livePages.Add(1)
				}
			}
			return ctx
		})
	}})
	machinesByLabel := map[string]machines.Machine{}
	keys := map[string]ed25519.PrivateKey{}
	for label := range probes {
		m, key := f.register(t, label)
		if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			return f.r.SetApproved(ctx, tx, m.ID, label)
		}); err != nil {
			t.Fatal("The pool fixture could not approve its machine.")
		}
		machinesByLabel[label], keys[label] = m, key
	}
	server := commandListener(t, f, probes)
	f.startRunner(t)
	peers := map[string]*commandPeer{}
	for _, label := range []string{"first", "second", "third"} {
		peers[label] = openCommandPeer(t, f, server, machinesByLabel[label], keys[label], label)
	}
	client := linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), server.URL)
	streams := map[string]*openClientStream{}
	cancels := map[string]context.CancelFunc{}
	for _, label := range []string{"healthy", "canceled waiter", "live waiter", "cleanup"} {
		streams[label], cancels[label] = openCancellableCommandStream(t, f, client, machinesByLabel[label], keys[label], label)
	}
	rows := map[string]claimFixtureRow{}
	flushes := map[string]commandFlush{}
	for _, label := range []string{"first", "second", "third"} {
		if label == "third" {
			s := streams["healthy"]
			sendHandshake(t, s, heartbeatFrame(nil, nil))
			requireHeartbeatAck(t, s)
			row := queueSendRestart(t, f.presenceFixture, machinesByLabel["healthy"].ID, 0)
			frame, err := s.Receive()
			if err != nil || frame.GetCommand() == nil {
				t.Fatal("The healthy stream could not deliver its command while two flushes held permits.")
			}
			row.requireFrame(t, frame.GetCommand(), 1)
		}
		rows[label] = queueSendRestart(t, f.presenceFixture, machinesByLabel[label].ID, 0)
		flushes[label] = watchCommandFlush(t, probes[label])
		rows[label].requireFrame(t, flushes[label].frame, 1)
	}
	requireHeld := func(label string) {
		t.Helper()
		select {
		case <-flushes[label].done:
			t.Fatal("A held flush completed before the independent operations had demonstrated pool headroom.")
		default:
		}
	}
	for _, label := range []string{"first", "second", "third"} {
		requireHeld(label)
	}

	for _, label := range []string{"canceled waiter", "live waiter"} {
		rows[label] = queueSendRestart(t, f.presenceFixture, machinesByLabel[label].ID, 0)
	}
	for _, label := range []string{"canceled waiter", "live waiter", "healthy"} {
		sendHandshake(t, streams[label], heartbeatFrame(nil, nil))
		requireHeartbeatAck(t, streams[label])
	}
	waiterPageCounts := []int32{canceledPages.Load(), livePages.Load()}
	for _, label := range []string{"canceled waiter", "live waiter"} {
		requireClaimStored(t, f.presenceFixture, rows[label].id, "", 1)
	}
	closeAndObserveCleanup := func(label string) {
		t.Helper()
		cancels[label]()
		storetest.Receive(t, "the closed stream's handler exiting", probes[label].closed)
		storetest.Poll(t, "disconnect cleanup committing while all command permits are held", func() bool {
			requireHeld("first")
			requireHeld("second")
			requireHeld("third")
			return f.connectionCount(t, machinesByLabel[label].ID) == 0
		})
	}
	closeAndObserveCleanup("cleanup")
	closeAndObserveCleanup("canceled waiter")
	requireClaimStored(t, f.presenceFixture, rows["live waiter"].id, "", 1)
	if canceledPages.Load() != waiterPageCounts[0] || livePages.Load() != waiterPageCounts[1] {
		t.Error("A saturated waiter queried more command pages without a permit release.")
	}

	rows["first"].requireFrame(t, peers["first"].readEnvelope(t).GetCommand(), 1)
	frame, err := streams["live waiter"].Receive()
	if err != nil || frame.GetCommand() == nil {
		t.Fatal("The oldest remaining waiter did not receive a command after one permit was released.")
	}
	rows["live waiter"].requireFrame(t, frame.GetCommand(), 1)
	requireHeld("second")
	requireHeld("third")
	requireClaimStored(t, f.presenceFixture, rows["canceled waiter"].id, "", 1)
}
