package link

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type handshakeFlushErrorWriter struct {
	http.ResponseWriter
	welcome       *atomic.Bool
	acceptedWrite bool
	failedFlush   chan<- bool
}

func (w *handshakeFlushErrorWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *handshakeFlushErrorWriter) Flush()                      { _ = w.FlushError() }

func (w *handshakeFlushErrorWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if w.welcome.Load() && err == nil && n == len(data) && n > 0 {
		w.acceptedWrite = true
	}
	return n, err
}

func (w *handshakeFlushErrorWriter) FlushError() error {
	if w.welcome.CompareAndSwap(true, false) {
		w.failedFlush <- w.acceptedWrite
		return errors.New("The fixture could not flush Welcome.")
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func TestFailedWelcomeFlushDoesNotOpenStream(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			var welcome atomic.Bool
			failedFlush := make(chan bool, 1)
			handlerDone := make(chan struct{})
			diagnostics := new(handshakeLog)
			f := newHandshakeFixture(t, handshakeFixtureOptions{
				logger: slog.New(slog.NewTextHandler(diagnostics, nil)),
				seams:  Seams{AfterAdmission: func(_ context.Context, _ string) { welcome.Store(true) }},
			})
			client := f.wire(t, handshakeWireOptions{wrap: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer close(handlerDone)
					next.ServeHTTP(&handshakeFlushErrorWriter{
						ResponseWriter: w, welcome: &welcome, failedFlush: failedFlush,
					}, r)
				})
			}}, protocol.options...)
			m, key := f.register(t, "station with a broken response")
			stream := startHandshake(t, client, nil)
			submitProof(t, f, stream, m.ID, key, 3)
			if !storetest.Receive(t, "the injected Welcome flush error", failedFlush) {
				t.Fatal("The fixture failed a flush before accepting any Welcome bytes.")
			}
			// A failed response can end the request before this frame reaches the server.
			_ = stream.Send(heartbeatFrame(nil, nil))
			_ = stream.CloseRequest()
			for {
				frame, err := stream.Receive()
				if err != nil {
					break
				}
				if frame.GetHeartbeatAck() != nil {
					t.Error("The server answered a live heartbeat after its Welcome flush failed.")
				}
			}
			storetest.Receive(t, "the Open handler ending after the injected failure", handlerDone)
			if diagnostics.contains("link_stream_opened") {
				t.Error("The server opened the stream after its Welcome flush failed.")
			}
			if !diagnostics.contains("link_stream_closed") {
				t.Error("The handler did not record the stream ending.")
			}
		})
	}
}

type proofCloseBody struct {
	io.ReadCloser
	readingProof *atomic.Bool
	readStarted  chan<- struct{}
	bodyClosed   chan<- struct{}
	start        sync.Once
	close        sync.Once
}

func (b *proofCloseBody) Read(data []byte) (int, error) {
	if b.readingProof.Load() {
		b.start.Do(func() { close(b.readStarted) })
	}
	return b.ReadCloser.Read(data)
}

func (b *proofCloseBody) Close() error {
	b.close.Do(func() { close(b.bodyClosed) })
	return b.ReadCloser.Close()
}

func TestProofRequestClosureAtCutoff(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		for _, tc := range []struct {
			name      string
			halfClose bool
			code      connect.Code
			sentence  string
		}{
			{"on-time closure queued behind timer", true, connect.CodeCanceled, "context canceled"},
			{"receive interrupted by timer", false, connect.CodeDeadlineExceeded, openTimeout},
		} {
			t.Run(protocol.name+"/"+tc.name, func(t *testing.T) {
				var readingProof atomic.Bool
				readStarted := make(chan struct{})
				bodyClosed := make(chan struct{})
				completed := make(chan receivedFrame, 1)
				hold, release := storetest.Signal(t)
				defer release()
				clock := newOpenClock()
				controls := clock.controls()
				controls.afterReceive = func(req *linkv1.OpenRequest, err error, at time.Time) {
					if req.GetHello() != nil {
						readingProof.Store(true)
					} else if tc.halfClose {
						completed <- receivedFrame{request: req, err: err, at: at}
						<-hold
					}
				}
				f := newHandshakeFixture(t, handshakeFixtureOptions{})
				client := f.wire(t, handshakeWireOptions{controls: controls, wrap: func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						r.Body = &proofCloseBody{
							ReadCloser: r.Body, readingProof: &readingProof,
							readStarted: readStarted, bodyClosed: bodyClosed,
						}
						next.ServeHTTP(w, r)
					})
				}}, protocol.options...)
				m, _ := f.register(t, "station closing during proof")
				before := f.head(t)
				stream := startHandshake(t, client, nil)
				hello := helloFrame(m.ID, LinkRevision, 4)
				sendHandshake(t, stream, hello)
				requireChallenge(t, stream, f.serverKey, hello.GetHello())
				clock.timer(t, "Hello")
				fire := clock.timer(t, "Proof")
				storetest.Receive(t, "the Proof receive entering the body", readStarted)
				const cutoff = 10 * time.Second
				clock.set(cutoff)
				if tc.halfClose {
					if err := stream.CloseRequest(); err != nil {
						t.Fatal("The fixture could not half-close its request.")
					}
					result := storetest.Receive(t, "the completed Proof receive held before delivery", completed)
					if result.request != nil || !errors.Is(result.err, io.EOF) || !result.at.Equal(clock.base.Add(cutoff)) {
						t.Fatal("The fixture did not capture an ordinary request closure at the exact cutoff.")
					}
				}
				fire <- clock.base.Add(cutoff)
				storetest.Receive(t, "the phase timer closing the request body", bodyClosed)
				release()
				requireOpenEnd(t, stream, tc.code, tc.sentence, "")
				if f.connectionCount(t, m.ID) != 0 {
					t.Error("A Proof receive without a message admitted a stream.")
				}
				f.requireSaved(t, before)
			})
		}
	}
}
