package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"
)

func deadlineServer(t *testing.T, handler http.Handler, window int) (*http.Client, string) {
	t.Helper()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server := httptest.NewUnstartedServer(handler)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{
		Protocols: protocols,
		HTTP2:     &http.HTTP2Config{MaxReceiveBufferPerStream: window},
	}}
	t.Cleanup(client.CloseIdleConnections)
	return client, server.URL
}

func openDeadlineResponse(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("Opening the response failed: %v", err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	if res.ProtoMajor != 2 || res.StatusCode != http.StatusOK {
		t.Fatalf("The response used %s and status %d, want HTTP/2 and status 200.", res.Proto, res.StatusCode)
	}
	return res
}

type blockedSendResult struct {
	err         error
	started     time.Time
	ended       time.Time
	laterErr    error
	laterCalled bool
}

func blockedDeadlineServer(t *testing.T, timeout time.Duration, ctx context.Context) (*http.Response, <-chan blockedSendResult) {
	t.Helper()
	done := make(chan blockedSendResult, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result blockedSendResult
		defer func() {
			result.ended = time.Now()
			done <- result
		}()
		controller := http.NewResponseController(w)
		if err := controller.Flush(); err != nil {
			result.err = err
			return
		}
		sendCtx, cancel := context.WithCancel(r.Context())
		defer cancel()
		stop := context.AfterFunc(ctx, cancel)
		defer stop()
		result.err = Send(sendCtx, func() error {
			result.started = time.Now()
			payload := make([]byte, 64<<10)
			payload[0] = 'x'
			if _, err := w.Write(payload); err != nil {
				return err
			}
			return controller.Flush()
		})
		if ctx.Err() != nil {
			result.laterErr = Send(sendCtx, func() error {
				result.laterCalled = true
				return nil
			})
		}
	})
	client, url := deadlineServer(t, WithTimeout(handler, timeout), 128)
	res := openDeadlineResponse(t, client, url)
	var first [1]byte
	if _, err := io.ReadFull(res.Body, first[:]); err != nil || first[0] != 'x' {
		t.Fatalf("The response began with %q and %v, want the start of the blocked send.", first[:], err)
	}
	return res, done
}

func awaitBlockedSend(t *testing.T, done <-chan blockedSendResult, within time.Duration) blockedSendResult {
	t.Helper()
	select {
	case result := <-done:
		if result.err == nil || result.started.IsZero() {
			t.Fatalf("The send returned %v, want a write error after entering its callback.", result.err)
		}
		return result
	case <-time.After(within):
		t.Fatalf("The blocked send and handler did not return within %v.", within)
		return blockedSendResult{}
	}
}

func TestDeadlineEndsBlockedStream(t *testing.T) {
	const timeout = 200 * time.Millisecond
	res, done := blockedDeadlineServer(t, timeout, t.Context())
	result := awaitBlockedSend(t, done, 2*time.Second)
	if elapsed := result.ended.Sub(result.started); elapsed < timeout/2 || elapsed > 2*time.Second {
		t.Errorf("The blocked send lasted %v, want a write deadline near %v and within two seconds.", elapsed, timeout)
	}
	if _, err := io.Copy(io.Discard, res.Body); err == nil {
		t.Error("The reader reached a clean end, want an HTTP/2 stream error.")
	}
}

func TestDeadlineClearedAfterSend(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"success", nil},
		{"callback error", errors.New("The first callback failed after writing.")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := make(chan struct{})
			finished := make(chan [2]error, 1)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				controller := http.NewResponseController(w)
				if err := controller.Flush(); err != nil {
					finished <- [2]error{err, nil}
					return
				}
				write := func(text string) error {
					if _, err := io.WriteString(w, text); err != nil {
						return err
					}
					return controller.Flush()
				}
				called := false
				first := Send(r.Context(), func() error {
					called = true
					if err := write("first\n"); err != nil {
						return err
					}
					return tc.err
				})
				if !called || !errors.Is(first, tc.err) {
					finished <- [2]error{first, nil}
					return
				}
				select {
				case <-next:
				case <-r.Context().Done():
				}
				second := Send(r.Context(), func() error { return write("second\n") })
				finished <- [2]error{first, second}
			})
			client, url := deadlineServer(t, WithTimeout(handler, 200*time.Millisecond), 0)
			res := openDeadlineResponse(t, client, url)
			var first [6]byte
			if _, err := io.ReadFull(res.Body, first[:]); err != nil || string(first[:]) != "first\n" {
				t.Fatalf("The first send arrived as %q with %v.", first[:], err)
			}
			// A timer controls when the second write may run, after the first deadline would expire.
			timer := time.AfterFunc(400*time.Millisecond, func() { close(next) })
			defer timer.Stop()
			body, err := io.ReadAll(res.Body)
			if err != nil || string(body) != "second\n" {
				t.Fatalf("The reader got %q and %v after the first send, want the second send to arrive.", body, err)
			}
			select {
			case got := <-finished:
				if !errors.Is(got[0], tc.err) || got[1] != nil {
					t.Fatalf("The sends returned %v, want [%v <nil>].", got, tc.err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("The handler did not return after the second send.")
			}
		})
	}
}

func TestSendNeedsTheMiddleware(t *testing.T) {
	called := false
	err := Send(t.Context(), func() error { called = true; return nil })
	if !errors.Is(err, ErrNoDeadline) || called {
		t.Fatalf("Send without middleware returned %v and called its callback %t, want ErrNoDeadline and no call.", err, called)
	}
}

// callbackWriter pauses the write deadline setter when the cancellation callback expires it.
type callbackWriter struct {
	*httptest.ResponseRecorder
	expiring chan struct{}
	release  chan struct{}
	calls    chan time.Time
}

func (w *callbackWriter) SetWriteDeadline(deadline time.Time) error {
	w.calls <- deadline
	if !deadline.IsZero() && !deadline.After(time.Now()) {
		close(w.expiring)
		<-w.release
	}
	return nil
}

func TestSendWaitsForCallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &callbackWriter{httptest.NewRecorder(), make(chan struct{}), make(chan struct{}), make(chan time.Time, 8)}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		defer close(w.release)
		sending := make(chan struct{})
		finishSend := make(chan struct{})
		finished := make(chan struct{})
		defer close(finishSend)
		handler := WithWriteDeadline(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			defer close(finished)
			_ = Send(r.Context(), func() error {
				close(sending)
				<-finishSend
				return io.ErrClosedPipe
			})
		}))
		go handler.ServeHTTP(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
		synctest.Wait()
		select {
		case <-sending:
		default:
			t.Fatal("Send did not enter its callback.")
		}
		cancel()
		synctest.Wait()
		select {
		case <-w.expiring:
		default:
			t.Fatal("Cancellation did not enter the deadline setter.")
		}
		// Give the send callback a result while the cancellation callback still owns the writer.
		finishSend <- struct{}{}
		synctest.Wait()
		select {
		case <-finished:
			t.Fatal("Send returned before the cancellation callback released the writer.")
		default:
		}
		w.release <- struct{}{}
		synctest.Wait()
		select {
		case <-finished:
		default:
			t.Fatal("Send did not return after the cancellation callback finished.")
		}
		if len(w.calls) != 2 {
			t.Fatalf("The deadline setter was called %d times, want one arm and one expiration.", len(w.calls))
		}
		if armed, expired := <-w.calls, <-w.calls; !armed.Equal(time.Now().Add(10*time.Second)) || !expired.Equal(time.Now()) {
			t.Errorf("The deadline setter received %v and %v, want a ten-second deadline followed by expiration.", armed, expired)
		}
	})
}

func TestSendEndsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, done := blockedDeadlineServer(t, 10*time.Second, ctx)
	start := time.Now()
	cancel()
	result := awaitBlockedSend(t, done, 300*time.Millisecond)
	if elapsed := result.ended.Sub(start); elapsed < 0 || elapsed > 300*time.Millisecond {
		t.Fatalf("The send returned %v after cancellation, want within 300 milliseconds.", elapsed)
	}

	if !errors.Is(result.laterErr, context.Canceled) || result.laterCalled {
		t.Errorf("A later Send returned %v and called its callback %t, want context.Canceled and no call.", result.laterErr, result.laterCalled)
	}
}
