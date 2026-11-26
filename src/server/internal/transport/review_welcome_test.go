package transport_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

type failingWelcomeFlushWriter struct {
	http.ResponseWriter
	flushErr error
	inSend   *bool
	wrote    bool
	failed   bool
}

func (w *failingWelcomeFlushWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *failingWelcomeFlushWriter) Flush()                      { _ = w.FlushError() }

func (w *failingWelcomeFlushWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if *w.inSend && err == nil && n == len(data) && n > 0 {
		w.wrote = true
	}
	return n, err
}

func (w *failingWelcomeFlushWriter) FlushError() error {
	if *w.inSend && w.wrote && !w.failed {
		w.failed = true
		return w.flushErr
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func TestSendLinkWelcomeReturnsFlushFailure(t *testing.T) {
	for _, protocol := range wireProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			injected := errors.New("The fixture could not flush the accepted Welcome bytes.")
			type result struct {
				sent, guarded error
			}
			finished := make(chan result, 1)
			failed := make(chan bool, 1)
			var inSend bool
			_, rpc := linkv1connect.NewLinkServiceHandler(transportLinkHandler{
				open: func(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					if _, err := stream.Receive(); err != nil {
						return err
					}
					var sent error
					inSend = true
					guarded := transport.SendLinkWelcome(ctx, func() error { return nil }, func() error {
						sent = stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{
							Welcome: &linkv1.Welcome{LinkRevision: 1},
						}})
						return sent
					})
					inSend = false
					finished <- result{sent, guarded}
					return guarded
				},
			})
			handler := transport.WithLinkStream(transport.WithTimeout(rpc, 3*time.Second), openFrameLimit)
			client := transportLinkServer(t, protocol, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writer := &failingWelcomeFlushWriter{ResponseWriter: w, flushErr: injected, inSend: &inSend}
				handler.ServeHTTP(writer, r)
				failed <- writer.failed
			}))
			stream := transportOpen(t, client)
			_ = stream.CloseRequest()
			got := storetest.Receive(t, "the guarded Welcome send result", finished)
			if !storetest.Receive(t, "the writer failing a flush after accepting the frame", failed) {
				t.Fatal("The fixture did not inject the flush failure after a successful write.")
			}
			if got.sent != nil && !errors.Is(got.sent, injected) {
				t.Fatal("The generated Welcome send failed independently of the injected flush error.")
			}
			if !errors.Is(got.guarded, injected) {
				t.Error("The Welcome guard lost the underlying writer's flush error.")
			}
		})
	}
}
