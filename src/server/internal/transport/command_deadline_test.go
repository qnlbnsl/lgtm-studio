package transport_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

type commandDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines   []time.Time
	beforeWrite func()
}

func (w *commandDeadlineWriter) SetWriteDeadline(at time.Time) error {
	w.deadlines = append(w.deadlines, at)
	return nil
}

func (w *commandDeadlineWriter) Write(p []byte) (int, error) {
	if w.beforeWrite != nil {
		w.beforeWrite()
	}
	return w.ResponseRecorder.Write(p)
}

func TestSendUsesEarlierContextDeadline(t *testing.T) {
	for _, tc := range []struct {
		name          string
		callerTimeout time.Duration
		want          time.Duration
	}{
		{"default timeout", 0, transport.SendTimeout},
		{"earlier context deadline", time.Second, time.Second},
		{"later context deadline", time.Minute, transport.SendTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := &commandDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
				ctx := t.Context()
				if tc.callerTimeout > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, tc.callerTimeout)
					defer cancel()
				}
				want := time.Now().Add(tc.want)
				called := false
				handler := transport.WithWriteDeadline(http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
					err := transport.Send(r.Context(), func() error {
						called = true
						if len(w.deadlines) != 1 || !w.deadlines[0].Equal(want) {
							t.Error("Send did not arm the earlier write deadline before invoking the callback.")
						}
						_, err := wr.Write([]byte("fixture"))
						return err
					})
					if err != nil {
						t.Error("Send failed while its write and context were healthy.")
					}
				}))
				handler.ServeHTTP(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
				if !called {
					t.Error("Send did not call the live write callback.")
				}
				if len(w.deadlines) != 2 || !w.deadlines[1].IsZero() {
					t.Error("A successful Send did not clear its write deadline.")
				}
			})
		})
	}

	t.Run("canceled before entry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		w := &commandDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
		handler := transport.WithWriteDeadline(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			called := false
			err := transport.Send(r.Context(), func() error {
				called = true
				return nil
			})
			if called {
				t.Error("Send invoked the callback with a context already canceled.")
			}
			if !errors.Is(err, context.Canceled) {
				t.Error("Send did not return cancellation at entry.")
			}
		}))
		handler.ServeHTTP(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
		if len(w.deadlines) != 0 {
			t.Error("Send touched the write deadline with a context already canceled.")
		}
	})

	t.Run("canceled during a held write", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			entered := make(chan struct{})
			release := make(chan struct{})
			finished := make(chan error, 1)
			w := &commandDeadlineWriter{
				ResponseRecorder: httptest.NewRecorder(),
				beforeWrite: func() {
					close(entered)
					<-release
				},
			}
			handler := transport.WithWriteDeadline(http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
				finished <- transport.Send(r.Context(), func() error {
					_, err := wr.Write([]byte("fixture"))
					return err
				})
			}))
			go handler.ServeHTTP(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
			synctest.Wait()
			select {
			case <-entered:
			default:
				t.Error("Send did not enter the held write.")
			}

			cancel()
			synctest.Wait()
			if len(w.deadlines) != 2 || !w.deadlines[1].Equal(time.Now()) {
				t.Error("Cancellation did not expire the deadline while the write was held.")
			}
			close(release)
			synctest.Wait()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) {
					t.Error("Send did not return cancellation after the held write completed.")
				}
			default:
				t.Error("Send did not return after the held write was released.")
			}
		})
	})
}

type commandFlushWriter struct {
	commandDeadlineWriter
	flushErrors  []error
	flushedBytes int
	frameFlushes int
}

func (w *commandFlushWriter) FlushError() error {
	// A protocol flush can fail only after the response accepted more bytes.
	if w.Body.Len() > w.flushedBytes {
		w.flushedBytes = w.Body.Len()
		w.frameFlushes++
		if w.frameFlushes <= len(w.flushErrors) {
			return w.flushErrors[w.frameFlushes-1]
		}
	}
	w.ResponseRecorder.Flush()
	return nil
}

func TestLinkSendReportsFlushFailure(t *testing.T) {
	for _, protocol := range wireProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				firstFlushError := errors.New("The first fixture flush failed.")
				laterFlushError := errors.New("A later fixture flush failed.")
				w := &commandFlushWriter{
					commandDeadlineWriter: commandDeadlineWriter{ResponseRecorder: httptest.NewRecorder()},
					flushErrors:           []error{firstFlushError, laterFlushError},
				}
				opened := false
				_, rpc := linkv1connect.NewLinkServiceHandler(transportLinkHandler{
					open: func(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
						if _, err := stream.Receive(); err != nil {
							t.Fatal("The generated handler did not receive the fixture request frame.")
						}
						opened = true
						frame := &linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: &linkv1.Command{
							Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}},
						}}}
						sendFrame := func() error { return stream.Send(frame) }

						err := transport.Send(ctx, func() error {
							for range 2 {
								if err := sendFrame(); err != nil {
									return err
								}
							}
							return nil
						})
						if w.frameFlushes != 2 {
							t.Fatal("The protocol did not reach both failed flushes after accepting their writes.")
						}
						if !errors.Is(err, firstFlushError) {
							t.Error("Send did not return the first underlying flush error.")
						}
						if err := transport.Send(ctx, sendFrame); err != nil {
							t.Error("The next Send inherited a previous flush error.")
						}
						if w.frameFlushes != 3 {
							t.Error("The next Send did not reach a successful frame flush.")
						}

						expiring, cancel := context.WithTimeout(ctx, time.Second)
						defer cancel()
						entered := false
						err = transport.Send(expiring, func() error {
							entered = true
							<-expiring.Done()
							return nil
						})
						if !entered {
							t.Fatal("Send did not enter the callback before its context expired.")
						}
						if !errors.Is(err, context.DeadlineExceeded) {
							t.Error("Send did not report a context deadline that expired inside its callback.")
						}
						return nil
					},
				})
				// Both protocols accept a five-byte envelope for an empty request message.
				req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
					linkv1connect.LinkServiceOpenProcedure, bytes.NewReader(make([]byte, 5)))
				req.Proto, req.ProtoMajor, req.ProtoMinor = "HTTP/2.0", 2, 0
				req.Header.Set("Content-Type", protocol.contentType)
				req.Header.Set("Connect-Protocol-Version", "1")
				handler := transport.WithLinkStream(transport.WithWriteDeadline(rpc), openFrameLimit)
				handler.ServeHTTP(w, req)
				if !opened {
					t.Fatal("The generated protocol handler rejected the fixture request.")
				}
			})
		})
	}
}
