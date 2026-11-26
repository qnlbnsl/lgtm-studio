package transport_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protowire"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

const openFrameLimit = 64 << 10

type wireProtocol struct {
	name, contentType, encodingHeader string
	options                           []connect.ClientOption
}

var wireProtocols = []wireProtocol{
	{"Connect", "application/connect+proto", "Connect-Content-Encoding", nil},
	{"gRPC", "application/grpc", "Grpc-Encoding", []connect.ClientOption{connect.WithGRPC()}},
}

type transportLinkHandler struct {
	linkv1connect.UnimplementedLinkServiceHandler
	open func(context.Context, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error
}

func (h transportLinkHandler) Open(ctx context.Context, s *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	return h.open(ctx, s)
}

func transportLinkServer(t *testing.T, p wireProtocol, h http.Handler) linkv1connect.LinkServiceClient {
	t.Helper()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server := httptest.NewUnstartedServer(h)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	t.Cleanup(client.CloseIdleConnections)
	return linkv1connect.NewLinkServiceClient(client, server.URL, p.options...)
}

func transportOpen(t *testing.T, client linkv1connect.LinkServiceClient) *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse] {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	stream := client.Open(ctx)
	t.Cleanup(func() {
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	})
	if err := stream.Send(&linkv1.OpenRequest{}); err != nil {
		t.Fatal("The fixture could not start Open.")
	}
	return stream
}

type welcomeTransportWriter struct {
	http.ResponseWriter
	inSend   *atomic.Bool
	write    func()
	flush    func()
	deadline func(time.Time)
}

func (w *welcomeTransportWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *welcomeTransportWriter) Write(b []byte) (int, error) {
	if w.inSend.Load() && len(b) > 0 && w.write != nil {
		w.write()
	}
	return w.ResponseWriter.Write(b)
}

func (w *welcomeTransportWriter) Flush() { _ = w.FlushError() }

func (w *welcomeTransportWriter) FlushError() error {
	if w.inSend.Load() && w.flush != nil {
		w.flush()
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *welcomeTransportWriter) SetWriteDeadline(at time.Time) error {
	if w.inSend.Load() && w.deadline != nil {
		w.deadline(at)
	}
	return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(at)
}

func TestLinkFirstWriteGate(t *testing.T) {
	for _, p := range wireProtocols {
		for _, blocked := range []bool{true, false} {
			name := "entered"
			if blocked {
				name = "refused"
			}
			t.Run(p.name+"/"+name, func(t *testing.T) { testLinkFirstWrite(t, p, blocked) })
		}
	}
}

func testLinkFirstWrite(t *testing.T, p wireProtocol, refused bool) {
	t.Helper()
	replacement := connect.NewError(connect.CodeAborted, errors.New("A newer connection replaced this one."))
	inCallback := make(chan context.CancelCauseFunc, 1)
	releaseCallback, letCallbackWrite := storetest.Signal(t)
	atWrite := make(chan int32, 1)
	releaseWrite, letWriteReturn := storetest.Signal(t)
	finished := make(chan error, 1)
	var begins, writes, flushes, deadlines atomic.Int32
	var inSend atomic.Bool

	_, rpc := linkv1connect.NewLinkServiceHandler(transportLinkHandler{open: func(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		if _, err := stream.Receive(); err != nil {
			return err
		}
		if err := transport.Send(ctx, func() error {
			return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{}}})
		}); err != nil {
			return err
		}
		sendCtx, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		inSend.Store(true)
		err := transport.SendLinkWelcome(sendCtx, func() error {
			begins.Add(1)
			return context.Cause(sendCtx)
		}, func() error {
			inCallback <- cancel
			<-releaseCallback
			frame := &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: &linkv1.Welcome{LinkRevision: 1}}}
			if err := stream.Send(frame); err != nil || refused {
				return err
			}
			return stream.Send(frame)
		})
		inSend.Store(false)
		finished <- err
		if refused && errors.Is(err, replacement) {
			return replacement
		}
		return nil
	}})
	middleware := transport.WithLinkStream(transport.WithTimeout(rpc, 3*time.Second), openFrameLimit)
	client := transportLinkServer(t, p, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		middleware.ServeHTTP(&welcomeTransportWriter{
			ResponseWriter: w, inSend: &inSend,
			write: func() {
				if writes.Add(1) == 1 {
					atWrite <- begins.Load()
					<-releaseWrite
				}
			},
			flush:    func() { flushes.Add(1) },
			deadline: func(time.Time) { deadlines.Add(1) },
		}, r)
	}))
	defer letCallbackWrite()
	defer letWriteReturn()

	stream := transportOpen(t, client)
	first, err := stream.Receive()
	if err != nil || first.GetChallenge() == nil {
		t.Fatal("The generated stream did not send the fixture Challenge.")
	}
	var cancel context.CancelCauseFunc
	select {
	case cancel = <-inCallback:
	case <-finished:
		t.Fatal("SendLinkWelcome returned without invoking its send callback.")
	case <-time.After(5 * time.Second):
		t.Fatal("The Welcome callback was not reached.")
	}
	if begins.Load() != 0 || deadlines.Load() != 0 {
		t.Error("Welcome began or armed a deadline before the first transport write.")
	}
	if refused {
		cancel(replacement)
	}
	letCallbackWrite()
	if refused {
		if err := storetest.Receive(t, "the refused Welcome send", finished); !errors.Is(err, replacement) {
			t.Error("A pre-entry refusal did not return the original replacement cause.")
		}
		if begins.Load() > 1 || writes.Load() != 0 || flushes.Load() != 0 || deadlines.Load() != 0 {
			t.Error("The refused Welcome attempted a write, flush or transport deadline.")
		}
		msg, err := stream.Receive()
		if msg != nil || connect.CodeOf(err) != connect.CodeAborted {
			t.Error("A stream with a refused Welcome did not deliver the replacement status as its next result.")
		}
		return
	}
	if at := storetest.Receive(t, "entry into the underlying transport write", atWrite); at != 1 {
		t.Error("The underlying transport entered before the first-write check.")
	}
	letWriteReturn()
	for range 2 {
		frame, err := stream.Receive()
		if err != nil || frame.GetWelcome() == nil {
			t.Fatal("A permitted send did not deliver its two fixture frames.")
		}
	}
	if err := storetest.Receive(t, "the permitted Welcome send", finished); err != nil || begins.Load() != 1 {
		t.Error("The permitted send did not authorize exactly one first write across both frames.")
	}
}

type linkReadObserver struct {
	io.ReadCloser
	started chan struct{}
	armed   atomic.Bool
}

func (r *linkReadObserver) Read(b []byte) (int, error) {
	if r.armed.CompareAndSwap(true, false) {
		close(r.started)
	}
	return r.ReadCloser.Read(b)
}

func TestLinkReadCanFinishBeforeTerminalStatus(t *testing.T) {
	for _, p := range wireProtocols {
		t.Run(p.name, func(t *testing.T) {
			readerEntered := make(chan struct{})
			observe := make(chan *linkReadObserver, 1)
			receiveEnded := make(chan error, 1)
			_, rpc := linkv1connect.NewLinkServiceHandler(transportLinkHandler{open: func(ctx context.Context, s *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
				if _, err := s.Receive(); err != nil {
					return err
				}
				if err := s.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: &linkv1.Challenge{}}}); err != nil {
					return err
				}
				(<-observe).armed.Store(true)
				done := make(chan error, 1)
				go func() {
					frame, err := s.Receive()
					if frame != nil || err == nil {
						err = nil
					}
					done <- err
				}()
				<-readerEntered
				if err := transport.CloseLinkRead(ctx); err != nil {
					receiveEnded <- nil
					return err
				}
				if err := transport.CloseLinkRead(ctx); err != nil {
					receiveEnded <- nil
					return err
				}
				select {
				case err := <-done:
					receiveEnded <- err
				case <-time.After(time.Second):
					receiveEnded <- nil
					return connect.NewError(connect.CodeInternal, errors.New("The reader did not leave the handler."))
				}
				transport.BoundLinkEnd(ctx, time.Second)
				return connect.NewError(connect.CodeDeadlineExceeded, errors.New("The machine did not answer in time."))
			}})
			middleware := transport.WithLinkStream(transport.WithTimeout(rpc, 3*time.Second), openFrameLimit)
			client := transportLinkServer(t, p, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := &linkReadObserver{ReadCloser: r.Body, started: readerEntered}
				r.Body = body
				observe <- body
				middleware.ServeHTTP(w, r)
			}))
			stream := transportOpen(t, client)
			msg, err := stream.Receive()
			if err != nil || msg.GetChallenge() == nil {
				t.Fatal("The test Challenge did not arrive.")
			}
			if err := storetest.Receive(t, "the joined reader before returning terminal status", receiveEnded); err == nil {
				t.Error("CloseLinkRead did not finish the blocked receive before the handler returned.")
			}
			if msg, err := stream.Receive(); msg != nil || connect.CodeOf(err) != connect.CodeDeadlineExceeded {
				t.Error("The Open request with its send side still open did not receive the terminal deadline status.")
			}
		})
	}
}

type boundedEndWriter struct {
	*httptest.ResponseRecorder
	mu       sync.Mutex
	deadline time.Time
}

func (w *boundedEndWriter) SetWriteDeadline(at time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deadline = at
	return nil
}

func (w *boundedEndWriter) Write([]byte) (int, error) {
	w.mu.Lock()
	at := w.deadline
	w.mu.Unlock()
	if at.IsZero() {
		return 0, errors.New("The terminal write has no deadline.")
	}
	timer := time.NewTimer(time.Until(at))
	defer timer.Stop()
	<-timer.C
	return 0, context.DeadlineExceeded
}

func TestLinkTerminalWriteBounded(t *testing.T) {
	t.Run("terminal write", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const limit = 100 * time.Millisecond
			w := &boundedEndWriter{ResponseRecorder: httptest.NewRecorder()}
			var writeErr error
			h := transport.WithLinkStream(transport.WithTimeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				transport.BoundLinkEnd(r.Context(), limit)
				_, writeErr = w.Write([]byte("terminal status"))
			}), 10*time.Second), openFrameLimit)
			start := time.Now()
			h.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
			if !errors.Is(writeErr, context.DeadlineExceeded) || time.Since(start) != limit {
				t.Fatal("The blocked terminal write did not finish at the configured deadline.")
			}
		})
	})
	t.Run("send cancellation and terminal deadline", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const limit = 100 * time.Millisecond
			w := &boundedEndWriter{ResponseRecorder: httptest.NewRecorder()}
			h := transport.WithLinkStream(transport.WithTimeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx, cancel := context.WithCancel(r.Context())
				defer cancel()
				timer := time.AfterFunc(limit, cancel)
				defer timer.Stop()
				if err := transport.SendLinkWelcome(ctx, func() error { return nil }, func() error {
					_, err := w.Write([]byte("started Welcome"))
					return err
				}); err == nil {
					t.Error("The blocked Welcome did not finish with a send error.")
				}
				transport.BoundLinkEnd(r.Context(), 2*limit)
				_, _ = w.Write([]byte("terminal status"))
			}), limit), openFrameLimit)
			start := time.Now()
			h.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
			if elapsed := time.Since(start); elapsed < limit || elapsed > 3*limit {
				t.Error("Cancellation racing the send deadline left the terminal write unbounded.")
			}
		})
	})
}

func testEnvelope(flags byte, payload []byte) []byte {
	frame := make([]byte, 5, 5+len(payload))
	frame[0] = flags
	binary.BigEndian.PutUint32(frame[1:], uint32(len(payload)))
	return append(frame, payload...)
}

type openPrefixBody struct {
	ctx          context.Context
	prefix       *bytes.Reader
	payload      *bytes.Reader
	oversizeRead bool
	stall        bool
}

func (b *openPrefixBody) Read(p []byte) (int, error) {
	if b.prefix.Len() != 0 {
		// Fragment the prefix to exercise partial reads in the admission wrapper.
		return b.prefix.Read(p[:min(2, len(p))])
	}
	if b.payload == nil {
		return 0, io.EOF
	}
	b.oversizeRead = true
	if !b.stall {
		return b.payload.Read(p)
	}
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (*openPrefixBody) Close() error { return nil }

func TestOpenOversizeStopsAtPrefix(t *testing.T) {
	const limit = openFrameLimit
	// An unknown length-delimited field gives an otherwise valid OpenRequest an exact decoded length.
	valid := protowire.AppendBytes(protowire.AppendTag(nil, 15, protowire.BytesType), make([]byte, limit-4))
	if len(valid) != limit {
		t.Fatal("The exact-limit fixture frame has the wrong length.")
	}
	oversized := append(bytes.Clone(valid), 0x78, 0x01)
	var compressed bytes.Buffer
	zip := gzip.NewWriter(&compressed)
	_, _ = zip.Write(oversized)
	if err := zip.Close(); err != nil {
		t.Fatal("The fixture could not compress the decoded oversize frame.")
	}
	short := testEnvelope(0, []byte{0x0a, 0x00})
	for _, p := range wireProtocols {
		for _, tc := range []struct {
			name                        string
			prior                       int
			oversize, stall, compressed bool
		}{
			{"complete first", 0, true, false, false},
			{"stalled first", 0, true, true, false},
			{"complete after one", 1, true, false, false},
			{"stalled after one", 1, true, true, false},
			{"valid at limit", 0, false, false, false},
			{"multiple valid including exact limit", 2, false, false, false},
			{"compressed decoded oversize", 1, false, false, true},
		} {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				wire := bytes.Repeat(short, tc.prior)
				body := &openPrefixBody{ctx: ctx, stall: tc.stall}
				switch {
				case tc.oversize:
					wire = append(wire, testEnvelope(0, oversized)[:5]...)
					body.payload = bytes.NewReader(oversized)
				case tc.compressed:
					wire = append(wire, testEnvelope(1, compressed.Bytes())...)
				default:
					wire = append(wire, testEnvelope(0, valid)...)
				}
				body.prefix = bytes.NewReader(wire)
				var received int
				var receiveErr error
				handler := api.NewHandler(api.Options{Link: transportLinkHandler{open: func(_ context.Context, s *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					for range tc.prior + 1 {
						_, receiveErr = s.Receive()
						if receiveErr != nil {
							return connect.NewError(connect.CodeInvalidArgument, errors.New("The link message broke a rule of the contract."))
						}
						received++
					}
					_, receiveErr = s.Receive()
					return nil
				}}})
				req := httptest.NewRequestWithContext(ctx, http.MethodPost, linkv1connect.LinkServiceOpenProcedure, body)
				req.Proto, req.ProtoMajor, req.ProtoMinor = "HTTP/2.0", 2, 0
				req.Header.Set("Content-Type", p.contentType)
				req.Header.Set("Connect-Protocol-Version", "1")
				if tc.compressed {
					req.Header.Set(p.encodingHeader, "gzip")
				}
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				if w.Result().Header.Get("Cache-Control") != "no-store" {
					t.Error("The Open route did not set no-store before sending headers.")
				}
				switch {
				case tc.oversize:
					if received != tc.prior || !errors.Is(receiveErr, transport.ErrLinkFrameTooLarge) || body.oversizeRead || ctx.Err() != nil {
						t.Error("The Open route did not reject the oversize prefix before reading payload bytes or reaching the deadline.")
					}
				case tc.compressed:
					if received != tc.prior || connect.CodeOf(receiveErr) != connect.CodeResourceExhausted {
						t.Error("The generated protocol did not enforce the decoded limit on compressed payloads.")
					}
				default:
					if received != tc.prior+1 || !errors.Is(receiveErr, io.EOF) {
						t.Error("The Open reader did not preserve exact-limit frames and ordinary end of file.")
					}
				}
			})
		}
	}
}
