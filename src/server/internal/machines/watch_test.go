package machines

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest/migrated"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/watch"
)

type clientsWatchFixture struct {
	store    *store.Store
	registry *Registry
	sessions *auth.Sessions
	verifier auth.Verifier
	cookie   string
}

func newClientsWatchFixture(t *testing.T) *clientsWatchFixture {
	t.Helper()
	s := migrated.NewStore(t, "machines_watch", Migrations(), auth.Migrations())
	password, err := auth.NewPassword("watch fixture password")
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessions(s, password)
	token, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatalf("Issuing the watch fixture's session failed: %v", err)
	}
	verifier, ok := auth.TokenVerifier(token)
	if !ok {
		t.Fatal("The issued watch fixture token did not produce a verifier.")
	}
	return &clientsWatchFixture{
		store: s, registry: NewRegistry(), sessions: sessions, verifier: verifier,
		cookie: (&http.Cookie{Name: auth.CookieName, Value: token}).String(),
	}
}

// timedClientsWatch substitutes a timeout at the service boundary after the router has run its session guard.
type timedClientsWatch struct {
	*Service
	writer  http.ResponseWriter
	request *http.Request
	timeout time.Duration
}

func (s *timedClientsWatch) WatchClients(ctx context.Context, req *connect.Request[lgtmv1.WatchClientsRequest], stream *connect.ServerStream[lgtmv1.WatchClientsResponse]) (err error) {
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		err = s.Service.WatchClients(r.Context(), req, stream)
	})
	transport.WithTimeout(handler, s.timeout).ServeHTTP(s.writer, s.request.WithContext(ctx))
	return err
}

func (f *clientsWatchFixture) serve(t *testing.T, window int, timeout time.Duration) (*http.Client, string, <-chan time.Time) {
	t.Helper()
	origins, err := auth.NewOriginPolicy("http://127.0.0.1:8787", nil)
	if err != nil {
		t.Fatal(err)
	}
	tokens := NewTokens(f.store, f.registry, Issuer{})
	service := NewService(f.store, f.sessions, f.registry, tokens)
	opts := api.Options{Origins: origins, Sessions: f.sessions, Clients: service}
	handler := api.NewHandler(opts)
	done := make(chan time.Time, 1)
	if timeout != 0 {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() { done <- time.Now() }()
			opts := opts
			opts.Clients = &timedClientsWatch{service, w, r, timeout}
			api.NewHandler(opts).ServeHTTP(w, r)
		})
	}
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
	return client, server.URL, done
}

func (f *clientsWatchFixture) create(t *testing.T, names ...string) []Machine {
	t.Helper()
	machines := make([]Machine, 0, len(names))
	err := f.store.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		for _, name := range names {
			machine, err := f.registry.CreateWaiting(ctx, tx, NewMachine{
				PublicKey: bytes.Repeat([]byte{0x4d}, 32), HostName: name, TokenID: "fixture-token",
			})
			if err != nil {
				return err
			}
			machines = append(machines, machine)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Writing %d machine changes failed: %v", len(names), err)
	}
	return machines
}

func openClientsWatch(t *testing.T, client lgtmv1connect.ClientServiceClient, cookie string, within time.Duration) (*connect.ServerStreamForClient[lgtmv1.WatchClientsResponse], error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), within)
	t.Cleanup(cancel)
	req := connect.NewRequest(&lgtmv1.WatchClientsRequest{})
	if cookie != "" {
		req.Header().Set("Cookie", cookie)
	}
	stream, err := client.WatchClients(ctx, req)
	if err == nil {
		t.Cleanup(func() { _ = stream.Close() })
	}
	return stream, err
}

func nextClientsWatchMessage(t *testing.T, stream *connect.ServerStreamForClient[lgtmv1.WatchClientsResponse], want string) *lgtmv1.WatchClientsResponse {
	t.Helper()
	if !stream.Receive() {
		t.Fatalf("The watch ended with %v before sending %s.", stream.Err(), want)
	}
	return stream.Msg()
}

func TestWatchClientsFirstMessageAndChanges(t *testing.T) {
	f := newClientsWatchFixture(t)
	initial := f.create(t, "already waiting")[0]
	httpClient, url, _ := f.serve(t, 0, 0)
	client := lgtmv1connect.NewClientServiceClient(httpClient, url)
	stream, err := openClientsWatch(t, client, f.cookie, 15*time.Second)
	if err != nil {
		t.Fatalf("Opening WatchClients with a live session failed: %v", err)
	}
	snapshot := nextClientsWatchMessage(t, stream, "the snapshot")
	list := snapshot.GetAll().GetClients()
	if len(list) != 1 || list[0].GetId() != initial.ID || list[0].GetName() != "already waiting" || snapshot.GetPosition() == nil {
		t.Fatalf("The first message was %v, want all with the existing machine and its position.", snapshot)
	}

	added := f.create(t, "new waiting machine")[0]
	saved := nextClientsWatchMessage(t, stream, "a saved record")
	if saved.GetSaved().GetId() != added.ID || saved.GetSaved().GetName() != "new waiting machine" ||
		saved.GetSaved().GetState() != lgtmv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL {
		t.Fatalf("The next message was %v, want the newly waiting machine.", saved)
	}
	if saved.GetPosition() == nil || saved.GetPosition().GetValue() <= snapshot.GetPosition().GetValue() {
		t.Fatalf("The saved position %v did not advance past the snapshot position %v.", saved.GetPosition(), snapshot.GetPosition())
	}

	if err := f.store.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.registry.Delete(ctx, tx, added.ID)
	}); err != nil {
		t.Fatalf("Deleting the watched machine failed: %v", err)
	}
	removed := nextClientsWatchMessage(t, stream, "a removed identifier")
	if removed.GetRemovedId() != added.ID || removed.GetPosition() == nil || removed.GetPosition().GetValue() <= saved.GetPosition().GetValue() {
		t.Fatalf("The removal was %v, want %s and a position after %v.", removed, added.ID, saved.GetPosition())
	}
}

func TestWatchClientsPositionAlwaysPresent(t *testing.T) {
	f := newClientsWatchFixture(t)
	// Session issuance records one feed row. Clear it in this isolated fixture before any watch starts.
	err := f.store.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		for _, sql := range []string{
			"ALTER TABLE feed DISABLE TRIGGER feed_check_write",
			"TRUNCATE feed RESTART IDENTITY",
			"ALTER TABLE feed ENABLE TRIGGER feed_check_write",
		} {
			if _, err := tx.Exec(ctx, sql); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Preparing an empty feed with a live session failed: %v", err)
	}
	head, err := f.store.Snapshot(t.Context(), func(context.Context, *store.ReadTx) error { return nil })
	if err != nil || head != 0 {
		t.Fatalf("The empty fixture has feed head %d with %v, want position 0.", head, err)
	}
	httpClient, url, _ := f.serve(t, 0, 0)
	client := lgtmv1connect.NewClientServiceClient(httpClient, url)
	stream, err := openClientsWatch(t, client, f.cookie, 15*time.Second)
	if err != nil {
		t.Fatalf("Opening the empty watch failed: %v", err)
	}
	snapshot := nextClientsWatchMessage(t, stream, "the empty snapshot")
	if snapshot.GetAll() == nil || len(snapshot.GetAll().GetClients()) != 0 || snapshot.GetPosition() == nil || snapshot.GetPosition().GetValue() != 0 {
		t.Fatalf("The first message was %v, want empty all with an explicit position of 0.", snapshot)
	}
}

func TestWatchClientsNeedsLiveSession(t *testing.T) {
	f := newClientsWatchFixture(t)
	httpClient, url, _ := f.serve(t, 0, 0)
	client := lgtmv1connect.NewClientServiceClient(httpClient, url)
	t.Run("no cookie", func(t *testing.T) {
		stream, err := openClientsWatch(t, client, "", 15*time.Second)
		if err == nil {
			if stream.Receive() {
				t.Fatalf("A watch without a cookie received %v.", stream.Msg())
			}
			err = stream.Err()
		}
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatalf("A watch without a cookie ended with %v, want unauthenticated.", err)
		}
	})
	t.Run("revoked during the watch", func(t *testing.T) {
		stream, err := openClientsWatch(t, client, f.cookie, 35*time.Second)
		if err != nil {
			t.Fatalf("Opening the live watch failed: %v", err)
		}
		if first := nextClientsWatchMessage(t, stream, "a snapshot before revocation"); first.GetAll() == nil {
			t.Fatalf("The watch opened with %v, want all.", first)
		}
		if err := f.sessions.Revoke(t.Context(), f.verifier); err != nil {
			t.Fatalf("Revoking the current session failed: %v", err)
		}
		f.create(t, "written after revocation")
		if stream.Receive() {
			t.Fatalf("The watch delivered %v after the session was revoked.", stream.Msg())
		}
		requireClientsWatchError(t, stream.Err(), connect.CodeUnauthenticated, "Sign in again to reopen this watch.")
	})
}

func requireClientsWatchError(t *testing.T, err error, code connect.Code, sentence string) {
	t.Helper()
	var rpcError *connect.Error
	if !errors.As(err, &rpcError) || rpcError.Code() != code || rpcError.Message() != sentence {
		t.Fatalf("The watch returned %v, want %v with %q.", err, code, sentence)
	}
}

func TestWatchClientsErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		cause    error
		code     connect.Code
		sentence string
	}{
		{store.ErrWatchLagged, connect.CodeAborted, "The watch fell behind and must be reopened."},
		{store.ErrFeedUnavailable, connect.CodeUnavailable, "The change feed is unavailable, try again."},
		{watch.ErrWatchUnauthorized, connect.CodeUnauthenticated, "Sign in again to reopen this watch."},
		{watch.ErrAuthorizationUnavailable, connect.CodeUnavailable, "The watch authorization check is unavailable, try again."},
	} {
		t.Run(tc.code.String()+"/"+tc.sentence, func(t *testing.T) {
			requireClientsWatchError(t, watchError(tc.cause), tc.code, tc.sentence)
		})
	}
}

func openRawClientsWatch(t *testing.T, client *http.Client, url, cookie string, within time.Duration) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), within)
	t.Cleanup(cancel)
	// A gRPC envelope for the empty WatchClientsRequest is five zero bytes.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+lgtmv1connect.ClientServiceWatchClientsProcedure, bytes.NewReader(make([]byte, 5)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("Te", "trailers")
	req.Header.Set("Cookie", cookie)
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("Opening the blocked-reader watch failed: %v", err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	if res.ProtoMajor != 2 || res.StatusCode != http.StatusOK {
		t.Fatalf("The watch returned %s with status %d, want HTTP/2 and status 200.", res.Proto, res.StatusCode)
	}
	return res
}

func watchFrameSize(t *testing.T, body io.Reader, what string) uint32 {
	t.Helper()
	var header [5]byte
	if _, err := io.ReadFull(body, header[:]); err != nil {
		t.Fatalf("Reading the %s envelope failed: %v", what, err)
	}
	size := binary.BigEndian.Uint32(header[1:])
	if header[0] != 0 || size > 4096 {
		t.Fatalf("The %s envelope has flags %d and size %d, want an uncompressed fixture message.", what, header[0], size)
	}
	return size
}

func TestWatchClientsBlockedReaderEnds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		revoke  bool
		timeout time.Duration
	}{
		{"deadline", false, 200 * time.Millisecond},
		{"revoked while blocked", true, 10 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClientsWatchFixture(t)
			const window = 64
			client, url, handlerDone := f.serve(t, window, tc.timeout)
			res := openRawClientsWatch(t, client, url, f.cookie, 2*tc.timeout+15*time.Second)
			initial := make([]byte, watchFrameSize(t, res.Body, "snapshot"))
			if _, err := io.ReadFull(res.Body, initial); err != nil {
				t.Fatalf("Reading the snapshot failed: %v", err)
			}
			var snapshot lgtmv1.WatchClientsResponse
			if err := proto.Unmarshal(initial, &snapshot); err != nil || snapshot.GetAll() == nil || len(snapshot.GetAll().GetClients()) != 0 {
				t.Fatalf("The watch opened with %v and %v, want an empty snapshot.", &snapshot, err)
			}

			names := make([]string, 500)
			for i := range names {
				names[i] = fmt.Sprintf("%03d-%s", i, strings.Repeat("x", 200))
			}
			f.create(t, names...)
			// Read just the next envelope header. Its body is larger than the receive window.
			blockedSize := watchFrameSize(t, res.Body, "first saved change")
			if blockedSize <= 2*window {
				t.Fatalf("The change has %d bytes, want a frame that cannot fit in the receive window.", blockedSize)
			}
			blocked := time.Now()
			if tc.revoke {
				select {
				case <-handlerDone:
					t.Fatal("The handler ended before the blocked session was revoked.")
				default:
				}
				if err := f.sessions.Revoke(t.Context(), f.verifier); err != nil {
					t.Fatalf("Revoking the blocked session failed: %v", err)
				}
				f.create(t, "written after revocation")
			}

			endBy := blocked.Add(3 * time.Second)
			if tc.revoke {
				endBy = blocked.Add(tc.timeout + time.Second)
			}
			select {
			case ended := <-handlerDone:
				if ended.After(endBy) {
					t.Fatal("The blocked watch returned after its time bound.")
				}
			case <-time.After(time.Until(endBy)):
				t.Fatal("The handler did not return while the reader was stopped.")
			}

			remaining := make([]byte, blockedSize)
			n, err := io.ReadFull(res.Body, remaining)
			if err == nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("The first blocked change delivered %d of %d bytes with %v, want a transport error before any later change.", n, blockedSize, err)
			}
		})
	}
}
