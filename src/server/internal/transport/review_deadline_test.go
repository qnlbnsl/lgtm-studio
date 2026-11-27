package transport_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

type reviewDeadlineWriter struct {
	*httptest.ResponseRecorder
	set func(time.Time)
}

func (w reviewDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.set(deadline)
	return nil
}

func reviewDeadlineRequest(t *testing.T, ctx context.Context, set func(time.Time), run func(context.Context)) {
	t.Helper()
	w := reviewDeadlineWriter{ResponseRecorder: httptest.NewRecorder(), set: set}
	handler := transport.WithWriteDeadline(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		run(r.Context())
	}))
	handler.ServeHTTP(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
}

func TestSendReportsCancellationWhileClearingDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cleared := false
	reviewDeadlineRequest(t, ctx, func(deadline time.Time) {
		if deadline.IsZero() {
			cleared = true
			cancel()
		}
	}, func(ctx context.Context) {
		calls := 0
		err := transport.Send(ctx, func() error {
			calls++
			return nil
		})
		if !cleared || calls != 1 {
			t.Fatal("Send did not write once and enter deadline clearing.")
		}
		if !errors.Is(err, context.Canceled) {
			t.Error("Send did not return cancellation that arrived while clearing the deadline.")
		}
	})
}

type reviewDeadlineContext struct {
	context.Context
	afterErr func()
}

func (ctx reviewDeadlineContext) Err() error {
	err := ctx.Context.Err()
	ctx.afterErr()
	return err
}

func TestSendReportsCancellationWhileJoiningDeadlineExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		expiring, releaseExpiry := make(chan struct{}), make(chan struct{})
		finished := make(chan error, 1)
		var afterWrite atomic.Bool
		go reviewDeadlineRequest(t, ctx, func(deadline time.Time) {
			switch {
			case deadline.IsZero():
				// A sender that reads the context only after cleanup reaches this hook first.
				afterWrite.Store(false)
				cancel()
			case deadline.Equal(time.Now()):
				close(expiring)
				<-releaseExpiry
			}
		}, func(ctx context.Context) {
			observed := reviewDeadlineContext{Context: ctx, afterErr: func() {
				if afterWrite.CompareAndSwap(true, false) {
					// Cancellation wins just after the sender reads a live context.
					cancel()
					<-expiring
				}
			}}
			finished <- transport.Send(observed, func() error {
				afterWrite.Store(true)
				return nil
			})
		})
		synctest.Wait()
		select {
		case <-expiring:
			select {
			case <-finished:
				t.Error("Send returned while the cancellation handler still held the writer.")
			default:
			}
		default:
			if ctx.Err() == nil {
				t.Error("The fixture did not cancel the context after the write.")
			}
		}
		close(releaseExpiry)
		synctest.Wait()
		select {
		case err := <-finished:
			if !errors.Is(err, context.Canceled) {
				t.Error("Send did not return cancellation after joining the deadline expiry handler.")
			}
		default:
			t.Error("Send did not return after deadline cleanup ended.")
		}
	})
}

func TestSendPreservesCallbackErrorDuringDeadlineCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errWrite := errors.New("The fixture response callback failed.")
	reviewDeadlineRequest(t, ctx, func(deadline time.Time) {
		if deadline.IsZero() {
			cancel()
		}
	}, func(ctx context.Context) {
		err := transport.Send(ctx, func() error { return errWrite })
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("The fixture did not cancel during deadline cleanup.")
		}
		if !errors.Is(err, errWrite) {
			t.Error("Send did not preserve the response callback error across deadline cleanup.")
		}
	})
}

type reviewFlushWriter struct {
	*commandFlushWriter
	onClear func()
}

func (w *reviewFlushWriter) SetWriteDeadline(deadline time.Time) error {
	if deadline.IsZero() && w.onClear != nil {
		w.onClear()
	}
	return nil
}

func TestSendPreservesFirstFlushErrorDuringDeadlineCleanup(t *testing.T) {
	for _, protocol := range wireProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			first := errors.New("The first fixture flush failed.")
			second := errors.New("The second fixture flush failed.")
			writer := &reviewFlushWriter{commandFlushWriter: &commandFlushWriter{
				commandDeadlineWriter: commandDeadlineWriter{ResponseRecorder: httptest.NewRecorder()},
				flushErrors:           []error{first, second},
			}}
			_, route := linkv1connect.NewLinkServiceHandler(transportLinkHandler{
				open: func(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					if _, err := stream.Receive(); err != nil {
						t.Fatal("The protocol did not accept the fixture request.")
					}
					ctx, cancel := context.WithCancel(ctx)
					defer cancel()
					writer.onClear = cancel
					err := transport.Send(ctx, func() error {
						for range 2 {
							if err := stream.Send(&linkv1.OpenResponse{}); err != nil {
								return err
							}
						}
						return nil
					})
					if writer.frameFlushes != 2 || !errors.Is(ctx.Err(), context.Canceled) {
						t.Fatal("The fixture did not fail both flushes and cancel during deadline clearing.")
					}
					if !errors.Is(err, first) {
						t.Error("Send did not preserve the first flush error across deadline cleanup.")
					}
					return nil
				},
			})
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
				linkv1connect.LinkServiceOpenProcedure, bytes.NewReader(make([]byte, 5)))
			req.Proto, req.ProtoMajor = "HTTP/2.0", 2
			req.Header.Set("Content-Type", protocol.contentType)
			req.Header.Set("Connect-Protocol-Version", "1")
			handler := transport.WithLinkStream(transport.WithWriteDeadline(route), openFrameLimit)
			handler.ServeHTTP(writer, req)
			if writer.onClear == nil {
				t.Fatal("The generated handler did not run the fixture send.")
			}
		})
	}
}

func TestSendSkipsCallbackWhenDeadlineArmingCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	armed := false
	reviewDeadlineRequest(t, ctx, func(deadline time.Time) {
		if !armed && !deadline.IsZero() {
			armed = true
			cancel()
		}
	}, func(ctx context.Context) {
		calls := 0
		err := transport.Send(ctx, func() error {
			calls++
			return nil
		})
		if calls != 0 {
			t.Error("Send invoked the response callback after arming the deadline canceled its context.")
		}
		if !errors.Is(err, context.Canceled) {
			t.Error("Send did not return cancellation triggered while arming the deadline.")
		}
	})
}
