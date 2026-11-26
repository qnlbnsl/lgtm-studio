package api_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const sessionLookupTimeout = 250 * time.Millisecond

type deadlineProtocol struct {
	name, timeoutHeader, timeoutValue string
	option                            connect.ClientOption
}

func (p deadlineProtocol) setHeaders(header http.Header, token string) {
	header.Set("Cookie", (&http.Cookie{Name: auth.CookieName, Value: token}).String())
	header.Set(p.timeoutHeader, p.timeoutValue)
}

func TestProtocolDeadlineIncludesSessionLookup(t *testing.T) {
	millis := strconv.FormatInt(sessionLookupTimeout.Milliseconds(), 10)
	for _, protocol := range []deadlineProtocol{
		{"Connect", "Connect-Timeout-Ms", millis, connect.WithProtoJSON()},
		{"gRPC", "Grpc-Timeout", millis + "m", connect.WithGRPC()},
	} {
		t.Run(protocol.name, func(t *testing.T) {
			t.Run("SignOut", func(t *testing.T) {
				t.Parallel()
				testLockedSignOut(t, protocol)
			})
			t.Run("ApproveClient", func(t *testing.T) {
				t.Parallel()
				testLockedApproveClient(t, protocol)
			})
		})
	}
}

func testLockedSignOut(t *testing.T, protocol deadlineProtocol) {
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	if err := fixture.sessions.Check(t.Context(), verifierOf(t, token)); err != nil {
		t.Fatalf("The fixture session failed verification. %v", err)
	}
	before := sessionSnapshot(t, fixture.observer)
	server := startHTTP2(t, sessionHandler(t, fixture, loopbackOrigin))
	server.Client().Timeout = 0
	client := webrpc.NewSessionServiceClient(server.Client(), server.URL, protocol.option)
	req := connect.NewRequest(&webv1.SignOutRequest{})
	protocol.setHeaders(req.Header(), token)

	var header http.Header
	err := callWithSessionTableLocked(t, fixture.observer, func(ctx context.Context) error {
		res, err := client.SignOut(ctx, req)
		if res != nil {
			header = res.Header()
		}
		var publicErr *connect.Error
		if errors.As(err, &publicErr) {
			header = publicErr.Meta()
		}
		return err
	})
	requireNoCookie(t, header)
	if !slices.Equal(sessionSnapshot(t, fixture.observer), before) {
		t.Error("SignOut changed the session row after the request deadline.")
	}
	requireRPCError(t, err, connect.CodeDeadlineExceeded, "The request timed out.")
}

func testLockedApproveClient(t *testing.T, protocol deadlineProtocol) {
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	probe := &clientProbe{}
	server := startHTTP2(t, api.NewHandler(api.Options{
		Sessions: fixture.sessions,
		Origins:  testOriginPolicy(t, loopbackOrigin),
		Clients:  probe,
	}))
	server.Client().Timeout = 0
	client := webrpc.NewClientServiceClient(server.Client(), server.URL, protocol.option)
	req := connect.NewRequest(&webv1.ApproveClientRequest{})
	protocol.setHeaders(req.Header(), token)

	err := callWithSessionTableLocked(t, fixture.observer, func(ctx context.Context) error {
		_, err := client.ApproveClient(ctx, req)
		return err
	})
	if calls := probe.unaryCalls.Load(); calls != 0 {
		t.Errorf("ApproveClient entered its handler %d times after the request deadline, want zero.", calls)
	}
	requireRPCError(t, err, connect.CodeDeadlineExceeded, "The request timed out.")
}

func callWithSessionTableLocked(t *testing.T, observer *pgx.Conn, run func(context.Context) error) error {
	t.Helper()
	tx, err := observer.Begin(t.Context())
	if err != nil {
		t.Fatalf("Starting the session table lock transaction failed. %v", err)
	}
	defer func() { _ = tx.Rollback(t.Context()) }()
	storetest.Exec(t, tx, "LOCK TABLE web_sessions IN ACCESS EXCLUSIVE MODE")

	// The local call has no deadline. Only the protocol header can time out the lookup.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	call := storetest.Start(t, func() error { return run(ctx) })
	storetest.Poll(t, "the session lookup waiting for the table lock or its reply", func() bool {
		select {
		case <-call.Done():
			// The deadline may have expired before the observer could sample the waiter.
			return true
		default:
		}
		var blocked bool
		err := tx.QueryRow(t.Context(), `
SELECT EXISTS (
    SELECT FROM pg_locks
    WHERE locktype = 'relation' AND relation = 'web_sessions'::regclass
      AND NOT granted AND pg_backend_pid() = ANY (pg_blocking_pids(pid))
)`).Scan(&blocked)
		if err != nil {
			t.Fatalf("Observing the session table lock failed. %v", err)
		}
		return blocked
	})

	// The observed lookup has already started. After one full protocol budget,
	// any deadline that started before the lookup has expired, even under load.
	// Release the lock then if the call has not already replied.
	budget := time.NewTimer(sessionLookupTimeout)
	defer budget.Stop()
	select {
	case <-call.Done():
	case <-budget.C:
	}
	if err := tx.Rollback(t.Context()); err != nil {
		t.Fatalf("Releasing the session table lock failed. %v", err)
	}
	return call.Wait(t)
}
