package link

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

func newServerStreamFixture(t *testing.T, opts handshakeFixtureOptions) *handshakeFixture {
	t.Helper()
	if opts.logger == nil {
		opts.logger = slog.New(slog.DiscardHandler)
	}
	return newHandshakeFixture(t, opts)
}

type streamTestOptions struct {
	controls      *streamControls
	codec         connect.Codec
	wrap          func(http.Handler) http.Handler
	clientOptions []connect.ClientOption
	timeout       time.Duration
	clientWindow  int
}

func steadyWire(t *testing.T, f *handshakeFixture, opts streamTestOptions) linkv1connect.LinkServiceClient {
	t.Helper()
	var options []connect.HandlerOption
	if opts.codec != nil {
		options = append(options, connect.WithCodec(opts.codec))
	}
	options = append(options, connect.WithReadMaxBytes(64<<10))
	_, route := linkv1connect.NewLinkServiceHandler(f.svc, options...)
	if opts.timeout == 0 {
		opts.timeout = 5 * time.Second
	}
	handler := transport.WithLinkStream(transport.WithTimeout(route, opts.timeout), 64<<10)
	if opts.controls != nil {
		next := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), streamControlsKey{}, opts.controls)))
		})
	}
	if opts.wrap != nil {
		handler = opts.wrap(handler)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	t.Cleanup(server.Close)
	client := handshakeHTTPClient(t)
	if opts.clientWindow > 0 {
		client.Transport.(*http.Transport).HTTP2 = &http.HTTP2Config{MaxReceiveBufferPerStream: opts.clientWindow}
	}
	return linkv1connect.NewLinkServiceClient(client, server.URL, opts.clientOptions...)
}

func openSteady(t *testing.T, f *handshakeFixture, client linkv1connect.LinkServiceClient, id string, key ed25519.PrivateKey, nonce byte) (*openClientStream, *linkv1.Welcome) {
	t.Helper()
	s := startHandshake(t, client, nil)
	submitProof(t, f, s, id, key, nonce)
	return s, requireWelcome(t, s)
}

func eventFrame(event *linkv1.Event) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}}
}

func heartbeatFrame(cpu, memory *uint32) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{CpuPercent: cpu, MemoryPercent: memory}}}
}

func resultFrame(id string, attempt uint32) *linkv1.OpenRequest {
	return &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: &linkv1.CommandResult{CommandId: id, Attempt: attempt}}}
}

func unknownFrame() *linkv1.OpenRequest {
	req := new(linkv1.OpenRequest)
	req.ProtoReflect().SetUnknown([]byte{0x72, 0x00})
	return req
}

func requireEventAck(t *testing.T, s *openClientStream, through uint64) {
	t.Helper()
	response, err := s.Receive()
	if err != nil || response.GetEventAck() == nil || response.GetEventAck().Through != through {
		t.Fatalf("Open did not acknowledge committed events through %d.", through)
	}
}

func requireHeartbeatAck(t *testing.T, s *openClientStream) {
	t.Helper()
	response, err := s.Receive()
	if err != nil || response.GetHeartbeatAck() == nil {
		t.Fatal("Open did not answer the heartbeat.")
	}
}

func TestHeartbeatIsAnswered(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "heartbeat")
	s, _ := openSteady(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, 1)
	sendHandshake(t, s, heartbeatFrame(proto.Uint32(7), proto.Uint32(28)))
	requireHeartbeatAck(t, s)
	p := f.current(t, m.ID)
	if p.CPUPercent == nil || *p.CPUPercent != 7 || p.MemoryPercent == nil || *p.MemoryPercent != 28 {
		t.Fatal("The heartbeat acknowledgement preceded the saved load.")
	}
	sendHandshake(t, s, heartbeatFrame(proto.Uint32(101), nil))
	requireOpenEnd(t, s, connect.CodeInvalidArgument, openInvalid, "")
}

type watchdogTick struct {
	after time.Duration
	fire  chan time.Time
}

type steadyClock struct {
	elapsed atomic.Int64
	created chan watchdogTick
}

func newSteadyClock() *steadyClock         { return &steadyClock{created: make(chan watchdogTick, 64)} }
func (c *steadyClock) now() time.Time      { return time.Unix(0, c.elapsed.Load()) }
func (c *steadyClock) set(d time.Duration) { c.elapsed.Store(int64(d)) }
func (c *steadyClock) controls() *streamControls {
	return &streamControls{now: c.now, timer: func(d time.Duration) (<-chan time.Time, func()) {
		fire := make(chan time.Time, 1)
		c.created <- watchdogTick{after: d, fire: fire}
		return fire, func() {}
	}}
}

func TestSilenceEndsStream(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			f.startRunner(t)
			m, key := f.register(t, "silent stream")
			clock := newSteadyClock()
			controls := clock.controls()
			readDone := make(chan struct{}, 1)
			controls.readerDone = func() { readDone <- struct{}{} }
			s, _ := openSteady(t, f, steadyWire(t, f, streamTestOptions{controls: controls, clientOptions: protocol.options}), m.ID, key, 1)
			p := f.current(t, m.ID)
			tick := storetest.Receive(t, "the watchdog starting at Welcome", clock.created)
			if tick.after != HeartbeatTimeout {
				t.Fatal("Welcome did not start the production heartbeat interval.")
			}
			clock.set(HeartbeatTimeout - time.Second)
			sendHandshake(t, s, eventFrame(factsEvent(1, "Report traffic")))
			requireEventAck(t, s, 1)
			clock.set(HeartbeatTimeout)
			tick.fire <- clock.now()
			requireOpenEnd(t, s, connect.CodeDeadlineExceeded, openTimeout, "")
			select {
			case <-readDone:
			default:
				t.Fatal("Open returned a terminal status before its request reader exited.")
			}
			storetest.Poll(t, "the silent stream's connection clearing", func() bool { return !runnerConnection(t, f, m.ID, p.StreamID) })
		})
	}
}

type streamCommitGate struct {
	armed   atomic.Bool
	pid     chan uint32
	release <-chan struct{}
}

func (g *streamCommitGate) configure(cfg *pgxpool.Config) {
	cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
		if strings.EqualFold(data.SQL, "commit") && g.armed.CompareAndSwap(true, false) {
			g.pid <- conn.PgConn().PID()
			select {
			case <-g.release:
			case <-ctx.Done():
			}
		}
		return ctx
	})
}

func appliedDiagnostics(t *testing.T, diagnostics *handshakeLog, id string, want ...uint64) {
	t.Helper()
	diagnostics.mu.Lock()
	data := diagnostics.log.String()
	diagnostics.mu.Unlock()
	var got []uint64
	for line := range strings.SplitSeq(data, "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			Message  string `json:"msg"`
			Machine  string `json:"machine_id"`
			Sequence uint64 `json:"sequence"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil {
			t.Fatal("A captured log entry was not structured JSON.")
		}
		if entry.Message == "link_event_applied" {
			if entry.Machine != id {
				t.Fatal("The applied diagnostic did not carry the matching machine identifier.")
			}
			got = append(got, entry.Sequence)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("The applied diagnostic sequences were %v, want %v.", got, want)
	}
}

func TestEventsAckedAfterCommit(t *testing.T) {
	logs := new(handshakeLog)
	release, resume := storetest.Signal(t)
	defer resume()
	gate := &streamCommitGate{pid: make(chan uint32, 1), release: release}
	f := newServerStreamFixture(t, handshakeFixtureOptions{logger: slog.New(slog.NewJSONHandler(logs, nil)), configure: gate.configure})
	m, key := f.register(t, "wire reports")
	installReportAudit(t, f.presenceFixture)
	client := steadyWire(t, f, streamTestOptions{})
	s, _ := openSteady(t, f, client, m.ID, key, 1)
	report := factsEvent(1, "First report")
	report.GetFacts().ProtoReflect().SetUnknown([]byte{0x7a, 0x0d, 'r', 'e', 'p', 'o', 'r', 't', '-', 'm', 'a', 'r', 'k', 'e', 'r'})
	sendHandshake(t, s, eventFrame(report))
	requireEventAck(t, s, 1)
	for _, pair := range [][2]uint64{{2, 2}, {3, 3}, {2, 3}} {
		sendHandshake(t, s, eventFrame(factsEvent(pair[0], "Later report")))
		requireEventAck(t, s, pair[1])
	}
	appliedDiagnostics(t, logs, m.ID, 1, 2, 3)
	sendHandshake(t, s, eventFrame(factsEvent(5, "Gap")))
	requireOpenEnd(t, s, connect.CodeFailedPrecondition, "The event leaves a gap after the server's mark.", "event_gap")
	s, welcome := openSteady(t, f, client, m.ID, key, 2)
	if welcome.LastEventNumber != 3 {
		t.Fatal("The reconnect Welcome did not carry the last committed mark after a gap.")
	}
	for _, n := range []uint64{4, 5} {
		sendHandshake(t, s, eventFrame(factsEvent(n, "Recovered report")))
		requireEventAck(t, s, n)
	}
	gate.armed.Store(true)
	sendHandshake(t, s, eventFrame(factsEvent(6, "Uncommitted report")))
	reading := storetest.Start(t, func() openReceiveResult { return receiveOpenFrame(s) })
	pid := storetest.Receive(t, "event six reaching commit", gate.pid)
	select {
	case <-reading.Done():
		t.Fatal("Event six had a wire response before its commit was released.")
	case <-time.After(250 * time.Millisecond):
	}
	if eventMark(t, f.presenceFixture, m.ID) != 5 {
		t.Fatal("An independent reader saw the uncommitted report mark.")
	}
	reportAudit(t, f.presenceFixture, 0, 1, 2, 3, 4)
	appliedDiagnostics(t, logs, m.ID, 1, 2, 3, 4, 5)
	if !storetest.Scalar[bool](t, f.obs, "SELECT pg_terminate_backend($1, 5000)", int32(pid)) {
		t.Fatal("The held event backend could not be terminated.")
	}
	resume()
	got := reading.Wait(t)
	checkOpenEnd(t, got.frame, got.err, connect.CodeUnavailable, openDatabase, "")
	reportAudit(t, f.presenceFixture, 0, 1, 2, 3, 4)
	appliedDiagnostics(t, logs, m.ID, 1, 2, 3, 4, 5)
	if logs.contains("report-marker") {
		t.Fatal("An unknown report field leaked into the diagnostics.")
	}
}

type malformedSteadyCodec struct{ mode string }

func (malformedSteadyCodec) Name() string { return "proto" }
func (c malformedSteadyCodec) Marshal(message any) ([]byte, error) {
	if request, ok := message.(*linkv1.OpenRequest); ok && request.GetEvent() != nil {
		switch c.mode {
		case "malformed":
			return []byte{0x22, 0x02, 0xff}, nil
		case "complete oversize", "stalled oversize":
			return make([]byte, (64<<10)+1), nil
		}
	}
	return proto.Marshal(message.(proto.Message))
}
func (malformedSteadyCodec) Unmarshal(data []byte, message any) error {
	return proto.Unmarshal(data, message.(proto.Message))
}

// prefixStallBody supplies an oversized third-frame prefix and waits for Close
// if the Open reader tries to consume its payload.
type prefixStallBody struct {
	io.ReadCloser
	closed    chan struct{}
	once      sync.Once
	frame     int
	remaining uint32
	prefix    [5]byte
	offset    int
	probe     chan struct{}
}

func (b *prefixStallBody) Close() error {
	b.once.Do(func() { close(b.closed) })
	return b.ReadCloser.Close()
}
func (b *prefixStallBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.frame == 2 && b.offset == 5 {
		select {
		case b.probe <- struct{}{}:
		default:
		}
		<-b.closed
		return 0, io.EOF
	}
	if b.remaining == 0 {
		n, err := b.ReadCloser.Read(p[:min(len(p), 5-b.offset)])
		copy(b.prefix[b.offset:], p[:n])
		b.offset += n
		if b.offset == 5 && b.frame < 2 {
			b.remaining = uint32(b.prefix[1])<<24 | uint32(b.prefix[2])<<16 | uint32(b.prefix[3])<<8 | uint32(b.prefix[4])
			if b.remaining == 0 {
				b.frame++
				b.offset = 0
			}
		}
		return n, err
	}
	n, err := b.ReadCloser.Read(p[:min(uint32(len(p)), b.remaining)])
	b.remaining -= uint32(n)
	if b.remaining == 0 {
		b.frame++
		b.offset = 0
	}
	return n, err
}

func TestMalformedSteadyFramesAreRefused(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		for _, mode := range []string{"malformed", "complete oversize", "stalled oversize"} {
			t.Run(protocol.name+"/"+mode, func(t *testing.T) {
				f := newServerStreamFixture(t, handshakeFixtureOptions{})
				m, key := f.register(t, "invalid wire report")
				codec := malformedSteadyCodec{mode: mode}
				opts := append([]connect.ClientOption{}, protocol.options...)
				opts = append(opts, connect.WithCodec(codec))
				probe := make(chan struct{}, 1)
				var wrap func(http.Handler) http.Handler
				if mode == "stalled oversize" {
					wrap = func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							body := &prefixStallBody{ReadCloser: r.Body, closed: make(chan struct{}), probe: probe}
							defer body.Close()
							r.Body = body
							next.ServeHTTP(w, r)
						})
					}
				}
				s, _ := openSteady(t, f, steadyWire(t, f, streamTestOptions{clientOptions: opts, wrap: wrap}), m.ID, key, 1)
				sendHandshake(t, s, eventFrame(factsEvent(1, "Malformed report")))
				requireOpenEnd(t, s, connect.CodeInvalidArgument, openInvalid, "")
				select {
				case <-probe:
					t.Fatal("Open tried to drain the stalled oversized frame.")
				default:
				}
				if eventMark(t, f.presenceFixture, m.ID) != 0 {
					t.Fatal("An undecodable or oversize frame advanced the mark.")
				}
			})
		}
	}
}

func TestReplacementWelcomeCarriesCommittedMark(t *testing.T) {
	for _, admitFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("admission first %t", admitFirst), func(t *testing.T) {
			locked := make(chan struct{}, 1)
			release, resume := storetest.Signal(t)
			defer resume()
			var armed atomic.Bool
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			m, key := f.register(t, "replacement mark")
			// The test holds the first registry operation while the other backend
			// demonstrably waits for the same row on a separate connection.
			f.state.afterLock = func(ctx context.Context, _ string) {
				if armed.CompareAndSwap(true, false) {
					locked <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
					}
				}
			}
			client := steadyWire(t, f, streamTestOptions{})
			a, _ := openSteady(t, f, client, m.ID, key, 1)
			eventSQL(t, f.presenceFixture, "UPDATE link_machines SET event_mark = 40 WHERE machine_id = $1", m.ID)
			installReportAudit(t, f.presenceFixture)
			armed.Store(true)
			b := startHandshake(t, client, nil)
			h := helloFrame(m.ID, LinkRevision, 2)
			sendHandshake(t, b, h)
			challenge := requireChallenge(t, b, f.serverKey, h.GetHello())
			proof := proofFrame(key, h.GetHello(), challenge)
			if admitFirst {
				sendHandshake(t, b, proof)
				storetest.Receive(t, "the replacement holding the registry row", locked)
				sendHandshake(t, a, eventFrame(factsEvent(41, "Racing event")))
			} else {
				sendHandshake(t, a, eventFrame(factsEvent(41, "Racing event")))
				storetest.Receive(t, "the event holding the registry row", locked)
				sendHandshake(t, b, proof)
			}
			storetest.Poll(t, "the second writer waiting on the registry row", func() bool {
				return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE datname = current_database() AND wait_event_type = 'Lock'
					AND lower(query) LIKE '%for no key update%' AND cardinality(pg_blocking_pids(pid)) > 0)`)
			})
			resume()
			welcome := requireWelcome(t, b)
			if admitFirst {
				if welcome.LastEventNumber != 40 {
					t.Fatal("Replacement included a report whose stream lost the registry lock.")
				}
				requireOpenEnd(t, a, connect.CodeAborted, openReplaced, "")
				reportAudit(t, f.presenceFixture)
			} else {
				if welcome.LastEventNumber != 41 {
					t.Fatal("Replacement lost a report that committed before admission.")
				}
				reportAudit(t, f.presenceFixture, 40)
			}
		})
	}
}

func TestUnknownKindIgnored(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "unknown frame")
	s, _ := openSteady(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, 1)
	sendHandshake(t, s, unknownFrame())
	sendHandshake(t, s, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, s)
	if eventMark(t, f.presenceFixture, m.ID) != 0 {
		t.Fatal("An unknown kind advanced the mark.")
	}
}

func TestCommandResultFrameRules(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "command result rules")
	client := steadyWire(t, f, streamTestOptions{})
	s, _ := openSteady(t, f, client, m.ID, key, 1)
	sendHandshake(t, s, resultFrame("invalid", 0))
	sendHandshake(t, s, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, s)
	f.approve(t, m.ID)
	id := fixtureStreamID(t)
	for i, frame := range []*linkv1.OpenRequest{resultFrame("invalid", 1), resultFrame(id, 0)} {
		s, _ := openSteady(t, f, client, m.ID, key, byte(i+2))
		sendHandshake(t, s, frame)
		requireOpenEnd(t, s, connect.CodeInvalidArgument, openInvalid, "")
	}
	s, _ = openSteady(t, f, client, m.ID, key, 4)
	before := f.head(t)
	sendHandshake(t, s, resultFrame(id, 1))
	sendHandshake(t, s, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, s)
	f.requireSaved(t, before, m.ID)
	if eventMark(t, f.presenceFixture, m.ID) != 0 {
		t.Fatal("A result without a command hook advanced the event mark.")
	}
}

func TestRemovalRefusesNextFrame(t *testing.T) {
	for _, displaced := range []bool{false, true} {
		for name, frame := range map[string]*linkv1.OpenRequest{
			"unknown":           unknownFrame(),
			"waiting result":    resultFrame("invalid", 0),
			"invalid load":      heartbeatFrame(proto.Uint32(101), nil),
			"wrong phase hello": helloFrame("invalid", LinkRevision, 1),
			"wrong phase proof": {Kind: &linkv1.OpenRequest_Proof{Proof: &linkv1.Proof{}}},
			"malformed":         eventFrame(factsEvent(1, "Malformed")),
			"late event":        eventFrame(factsEvent(1, "Displaced")),
		} {
			t.Run(fmt.Sprintf("displaced %t/%s", displaced, name), func(t *testing.T) {
				f := newServerStreamFixture(t, handshakeFixtureOptions{})
				m, key := f.register(t, "identity before frame")
				installReportAudit(t, f.presenceFixture)
				opts := streamTestOptions{}
				if name == "malformed" {
					opts.clientOptions = []connect.ClientOption{connect.WithCodec(malformedSteadyCodec{mode: "malformed"})}
				}
				s, _ := openSteady(t, f, steadyWire(t, f, opts), m.ID, key, 1)
				if displaced {
					b := f.admit(t, m.ID, 2)
					before := f.head(t)
					sendHandshake(t, s, frame)
					requireOpenEnd(t, s, connect.CodeAborted, openReplaced, "")
					if eventMark(t, f.presenceFixture, m.ID) != 0 || f.head(t) != before || !runnerConnection(t, f, m.ID, b.ID) {
						t.Fatal("A displaced frame changed the durable replacement.")
					}
				} else {
					eventSQL(t, f.presenceFixture, "DELETE FROM machines WHERE id = $1", m.ID)
					before := f.head(t)
					sendHandshake(t, s, frame)
					requireOpenEnd(t, s, connect.CodeFailedPrecondition, openUnknown, "unknown_machine")
					if f.head(t) != before {
						t.Fatal("A frame after removal wrote a feed reference.")
					}
				}
				reportAudit(t, f.presenceFixture)
			})
		}
	}
	t.Run("idle removal after lag", func(t *testing.T) {
		f := newServerStreamFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "idle removed stream")
		s, _ := openSteady(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, 1)
		eventSQL(t, f.presenceFixture, "DELETE FROM machines WHERE id = $1", m.ID)
		// Recovery wakes a live handler even when the deletion fell outside
		// the recovered subscription's start position.
		f.startRunner(t)
		requireOpenEnd(t, s, connect.CodeFailedPrecondition, openUnknown, "unknown_machine")
	})
}

type turnHooks struct {
	send func(context.Context, func(*linkv1.Command) error) (bool, error)
}

func (h turnHooks) SendNext(ctx context.Context, _, _ string, send func(*linkv1.Command) error) (bool, error) {
	return h.send(ctx, send)
}
func (turnHooks) AcceptResult(context.Context, string, string, *linkv1.CommandResult) error {
	return nil
}

func withTestCommands(t *testing.T, f *handshakeFixture, life context.Context, hooks CommandHooks) {
	t.Helper()
	opts := Options{Logger: slog.New(slog.DiscardHandler)}
	// Item 7's prerequisite Options stub lacks the field until item 5 lands.
	field := reflect.ValueOf(&opts).Elem().FieldByName("Commands")
	if !field.IsValid() || !field.CanSet() || !reflect.TypeOf(hooks).AssignableTo(field.Type()) {
		t.Fatal("Streaming Options did not expose its planned command hook.")
	}
	field.Set(reflect.ValueOf(hooks))
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{21}, ed25519.SeedSize))
	f.svc = NewStreamingService(life, f.s, f.tokens, f.r, f.state, f.hub, key, opts)
}

func testCommand(id string) *linkv1.Command {
	return &linkv1.Command{Id: id, Attempt: 1, ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
		Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}}}
}

func TestWriterSharesTurns(t *testing.T) {
	for _, scenario := range []string{"steady turns", "valid heartbeat refreshes", "report does not refresh", "first cause wins"} {
		t.Run(scenario, func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			m, key := f.register(t, "writer turns")
			f.approve(t, m.ID)
			life, shutdown := context.WithCancel(t.Context())
			defer shutdown()
			clock := newSteadyClock()
			controls := clock.controls()
			received := make(chan *linkv1.OpenRequest, 24)
			controls.afterReceive = func(req *linkv1.OpenRequest, err error) {
				if err == nil {
					received <- req
				}
			}
			held := make(chan context.Context, 1)
			release, resume := storetest.Signal(t)
			defer resume()
			var turns atomic.Int32
			command := testCommand(fixtureStreamID(t))
			withTestCommands(t, f, life, turnHooks{send: func(ctx context.Context, send func(*linkv1.Command) error) (bool, error) {
				n := turns.Add(1)
				if n == 1 {
					held <- ctx
					<-release
				}
				if n > 3 {
					return false, nil
				}
				err := send(command)
				return err == nil, err
			}})
			s, _ := openSteady(t, f, steadyWire(t, f, streamTestOptions{controls: controls}), m.ID, key, 1)
			ctx := storetest.Receive(t, "the writer held inside its first command turn", held)
			tick := storetest.Receive(t, "the active steady watchdog", clock.created)
			for n := uint64(1); n <= 16; n++ {
				sendHandshake(t, s, eventFrame(factsEvent(n, "Queued report")))
				got := storetest.Receive(t, "the reader decoding a queued report", received)
				if got.GetEvent().GetNumber() != n {
					t.Fatal("The reader did not buffer the sixteen frames in order.")
				}
			}
			clock.set(HeartbeatTimeout - time.Second)
			last := eventFrame(factsEvent(17, "Saturated report"))
			if scenario == "valid heartbeat refreshes" || scenario == "first cause wins" {
				last = heartbeatFrame(nil, nil)
			}
			sendHandshake(t, s, last)
			storetest.Receive(t, "the reader decoding a frame with the sixteen-slot queue full", received)
			sendHandshake(t, s, unknownFrame())
			select {
			case <-received:
				t.Fatal("The reader decoded an eighteenth frame while its sixteen-slot queue and pending send were full.")
			case <-time.After(250 * time.Millisecond):
			}
			switch scenario {
			case "first cause wins":
				shutdown()
				storetest.Receive(t, "the held hook receiving shutdown", ctx.Done())
				clock.set(2 * HeartbeatTimeout)
				tick.fire <- clock.now()
				resume()
				requireOpenEnd(t, s, connect.CodeUnavailable, "The server is stopping.", "")
			case "report does not refresh":
				clock.set(HeartbeatTimeout)
				tick.fire <- clock.now()
				storetest.Receive(t, "the watchdog ending a stream full of reports", ctx.Done())
				resume()
				requireOpenEnd(t, s, connect.CodeDeadlineExceeded, openTimeout, "")
			default:
				if scenario == "valid heartbeat refreshes" {
					clock.set(HeartbeatTimeout + time.Second)
					tick.fire <- clock.now()
					select {
					case refreshed := <-clock.created:
						if refreshed.after <= 0 || refreshed.after > HeartbeatTimeout {
							t.Fatal("The full queue did not preserve the valid heartbeat's refreshed deadline.")
						}
					case <-ctx.Done():
						t.Fatal("The watchdog expired before the writer could drain the queued heartbeat.")
					case <-time.After(storetest.ObservationTimeout):
						t.Fatal("The watchdog did not observe the queued heartbeat's refreshed deadline.")
					}
				}
				resume()
				for n := uint64(0); n < 3; n++ {
					if n > 0 {
						requireEventAck(t, s, n)
					}
					r, err := s.Receive()
					if err != nil || r.GetCommand() == nil {
						t.Fatal("The writer did not alternate queued frames and one command turn.")
					}
				}
				for n := uint64(3); n <= 16; n++ {
					requireEventAck(t, s, n)
				}
				if scenario == "valid heartbeat refreshes" {
					requireHeartbeatAck(t, s)
				} else {
					requireEventAck(t, s, 17)
				}
			}
		})
	}
}

func TestBlockedMachineEndsStream(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "blocked response")
	f.approve(t, m.ID)
	clock := newSteadyClock()
	controls := clock.controls()
	readerDone := make(chan struct{}, 1)
	controls.readerDone = func() { readerDone <- struct{}{} }
	command := testCommand(fixtureStreamID(t))
	random := rand.NewPCG(1, 2)
	payload := make([]byte, 63<<10)
	for i := range payload {
		payload[i] = byte(random.Uint64())
	}
	command.ProtoReflect().SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 15, protowire.BytesType), payload))
	failed := make(chan struct{}, 1)
	begin, releaseWriter := storetest.Signal(t)
	defer releaseWriter()
	withTestCommands(t, f, t.Context(), turnHooks{send: func(ctx context.Context, send func(*linkv1.Command) error) (bool, error) {
		select {
		case <-begin:
		case <-ctx.Done():
			return false, context.Cause(ctx)
		}
		err := send(command)
		if err != nil {
			select {
			case failed <- struct{}{}:
			default:
			}
		}
		return err == nil, err
	}})
	f.startRunner(t)
	openSteady(t, f, steadyWire(t, f, streamTestOptions{controls: controls, timeout: 200 * time.Millisecond, clientWindow: 65535}), m.ID, key, 1)
	p := f.current(t, m.ID)
	releaseWriter()
	storetest.Receive(t, "the blocked HTTP/2 command write hitting its deadline", failed)
	storetest.Receive(t, "the stream reader exiting after the write deadline", readerDone)
	storetest.Poll(t, "cleanup after the bounded send failure", func() bool { return !runnerConnection(t, f, m.ID, p.StreamID) })
}
