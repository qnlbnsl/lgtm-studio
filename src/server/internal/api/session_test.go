package api_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth/authtest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	sessionRequired      = "Sign in to continue."
	sessionUnavailable   = "Web session storage is unavailable."
	outcomeUnknown       = "The database did not confirm whether the write committed."
	requestKeyDisallowed = "This call does not accept a request key."
	sessionCommitGate    = int64(3186761742)
)

func sessionHandler(t *testing.T, fixture sessionDatabase, origin string, allowed ...string) http.Handler {
	t.Helper()
	return api.NewHandler(api.Options{
		Sessions: fixture.sessions,
		Origins:  testOriginPolicy(t, origin, allowed...),
		Session:  api.NewSessionService(fixture.password, fixture.sessions),
	})
}

func clockSessionDatabase(t *testing.T, at time.Time) (sessionDatabase, func(time.Time)) {
	t.Helper()
	db, observer, setClock := authtest.ClockStore(t, at, auth.Migrations())
	password := newTestPassword(t, testPassword)
	return sessionDatabase{password, auth.NewSessions(db, password), observer}, setClock
}

func passwordJSON(password string) string {
	quoted, _ := json.Marshal(password)
	return `{"password":` + string(quoted) + "}"
}

func signInRequest(t *testing.T, password string) *http.Request {
	t.Helper()
	return jsonProcedure(t, webrpc.SessionServiceSignInProcedure, passwordJSON(password))
}

func requireNoCookie(t *testing.T, header http.Header) {
	t.Helper()
	if len(header.Values("Set-Cookie")) != 0 {
		t.Error("The response changed a cookie.")
	}
}

func requireNoStore(t *testing.T, header http.Header) {
	t.Helper()
	if !headerListContains(header, "Cache-Control", "no-store") {
		t.Error("The Session response lacks Cache-Control: no-store.")
	}
}

func requireSignInResult(t *testing.T, status int, body []byte, want webv1.SignInResult) {
	t.Helper()
	var msg webv1.SignInResponse
	if err := protojson.Unmarshal(body, &msg); status != http.StatusOK || err != nil || msg.GetResult() != want {
		t.Fatalf("SignIn returned HTTP %d and result %s with decoding error %v, want HTTP 200 and %s.", status, msg.GetResult(), err, want)
	}
}

func requireSessionCookie(t *testing.T, res *http.Response, name string, sameSite http.SameSite, secure, cleared bool) string {
	t.Helper()
	if len(res.Header.Values("Set-Cookie")) != 1 || len(res.Cookies()) != 1 {
		t.Fatal("The response must carry one valid Set-Cookie header.")
	}
	cookie := res.Cookies()[0]
	if cookie.Name != name || cookie.SameSite != sameSite || cookie.Secure != secure ||
		cookie.Path != "/" || !cookie.HttpOnly || cookie.Domain != "" || cookie.Partitioned {
		t.Error("The response cookie has the wrong name or attributes for its origin.")
	}
	if cleared {
		if cookie.Value != "" || cookie.MaxAge >= 0 || !cookie.Expires.Equal(time.Unix(0, 0)) ||
			!strings.Contains(res.Header.Get("Set-Cookie"), "Max-Age=0") {
			t.Error("SignOut must clear the selected cookie on the wire.")
		}
	} else {
		verifierOf(t, cookie.Value)
		if cookie.MaxAge != 2592000 {
			t.Errorf("The session cookie has Max-Age %d, want 2592000.", cookie.MaxAge)
		}
	}
	return cookie.Value
}

func requireServer(t *testing.T, client webrpc.SessionServiceClient, signedIn bool) {
	t.Helper()
	res, err := client.GetServer(t.Context(), connect.NewRequest(&webv1.GetServerRequest{}))
	if err != nil {
		t.Fatalf("GetServer failed: %v.", err)
	}
	msg := res.Msg
	if msg.GetProduct() != "lgtm-studio" || msg.GetVersion() != "0.1.0" || msg.GetContract() != 1 ||
		msg.GetSample() || msg.GetSignedIn() != signedIn {
		t.Errorf("GetServer returned %v, want product lgtm-studio, version 0.1.0, contract 1, sample false and signed_in %t.", msg, signedIn)
	}
	requireNoStore(t, res.Header())
}

func sessionRows(t *testing.T, observer *pgx.Conn) [][]byte {
	t.Helper()
	return storetest.Rows[[]byte](t, observer, "SELECT verifier FROM web_sessions ORDER BY created_at, verifier")
}

func sessionSnapshot(t *testing.T, observer *pgx.Conn) []string {
	t.Helper()
	return storetest.Rows[string](t, observer, "SELECT to_jsonb(s)::text FROM web_sessions s ORDER BY created_at, verifier")
}

func feedRows(t *testing.T, observer *pgx.Conn) int {
	t.Helper()
	return storetest.Scalar[int](t, observer, "SELECT count(*) FROM feed")
}

func requireNoReceipts(t *testing.T, observer *pgx.Conn) {
	t.Helper()
	if got := storetest.Scalar[int](t, observer, "SELECT count(*) FROM receipts"); got != 0 {
		t.Errorf("The auth calls left %d receipts, want zero.", got)
	}
}

func TestSessionContract(t *testing.T) {
	t.Run("credentials and independent jars", testSessionJars)
	t.Run("expired session", func(t *testing.T) {
		at := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
		fixture, setClock := clockSessionDatabase(t, at)
		handler := sessionHandler(t, fixture, loopbackOrigin)
		token := issueTestSession(t, fixture.sessions)
		for _, tc := range []struct {
			name string
			at   time.Time
			want bool
		}{
			{"still live", at.Add(30*24*time.Hour - time.Microsecond), true},
			{"expired", at.Add(30 * 24 * time.Hour), false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				setClock(tc.at)
				req := jsonProcedure(t, webrpc.SessionServiceGetServerProcedure, "{}")
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
				res := serveRequest(handler, req)
				var msg webv1.GetServerResponse
				if err := protojson.Unmarshal(res.Body.Bytes(), &msg); res.Code != http.StatusOK || err != nil || msg.GetSignedIn() != tc.want {
					t.Errorf("GetServer returned HTTP %d and signed_in %t with decoding error %v, want HTTP 200 and signed_in %t.", res.Code, msg.GetSignedIn(), err, tc.want)
				}
			})
		}
		req := jsonProcedure(t, webrpc.SessionServiceSignOutProcedure, "{}")
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		res := serveRequest(handler, req)
		requireNoCookie(t, res.Header())
		requireConnectJSONError(t, res, http.StatusUnauthorized, connect.CodeUnauthenticated, sessionRequired)
	})

	t.Run("optional verification and store errors", func(t *testing.T) {
		fixture := newSessionDatabase(t)
		handler := sessionHandler(t, fixture, loopbackOrigin)
		token := issueTestSession(t, fixture.sessions)
		breakSessionReads(t, fixture.observer)
		for _, value := range []string{"", "noncanonical", token} {
			req := jsonProcedure(t, webrpc.SessionServiceGetServerProcedure, "{}")
			if value != "" {
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: value})
			}
			res := serveRequest(handler, req)
			requireNoStore(t, res.Header())
			requireNoCookie(t, res.Header())
			if value == token {
				requireConnectJSONError(t, res, http.StatusServiceUnavailable, connect.CodeUnavailable, sessionUnavailable)
				continue
			}
			var msg webv1.GetServerResponse
			if err := protojson.Unmarshal(res.Body.Bytes(), &msg); res.Code != http.StatusOK || err != nil || msg.GetSignedIn() {
				t.Errorf("GetServer returned HTTP %d and signed_in %t with decoding error %v, want an available anonymous reply.", res.Code, msg.GetSignedIn(), err)
			}
		}
	})

	t.Run("cookie modes", testSessionCookieModes)
	t.Run("read limit", testSessionReadLimit)
	t.Run("confirmed rollback", testSessionRollback)
	t.Run("unknown commit", testSessionUnknownCommit)
}

func testSessionJars(t *testing.T) {
	fixture := newSessionDatabase(t)
	handler := sessionHandler(t, fixture, loopbackOrigin)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	makeClient := func() (*http.Client, webrpc.SessionServiceClient) {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
		return client, webrpc.NewSessionServiceClient(client, server.URL)
	}
	httpA, clientA := makeClient()
	httpB, clientB := makeClient()

	t.Run("anonymous metadata", func(t *testing.T) { requireServer(t, clientA, false) })
	t.Run("rejected credentials", func(t *testing.T) {
		for _, req := range []*http.Request{
			signInRequest(t, "wrong"),
			signInRequest(t, strings.Repeat("é", 2049)),
			jsonProcedure(t, webrpc.SessionServiceSignInProcedure+"?password="+url.QueryEscape(testPassword), "{}"),
			func() *http.Request {
				req := signInRequest(t, "")
				req.Header.Set("Password", testPassword)
				req.Header.Set("X-Password", testPassword)
				return req
			}(),
		} {
			res := serveRequest(handler, req)
			requireNoCookie(t, res.Header())
			requireNoStore(t, res.Header())
			requireSignInResult(t, res.Code, res.Body.Bytes(), webv1.SignInResult_SIGN_IN_RESULT_REJECTED)
		}
		if len(sessionRows(t, fixture.observer)) != 0 {
			t.Error("Rejected passwords left sessions.")
		}
	})

	t.Run("independent sign-in and revocation", func(t *testing.T) {
		signIn := func(client *http.Client) string {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
				server.URL+webrpc.SessionServiceSignInProcedure, strings.NewReader(passwordJSON(testPassword)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Connect-Protocol-Version", "1")
			res, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			requireNoStore(t, res.Header)
			requireSignInResult(t, res.StatusCode, body, webv1.SignInResult_SIGN_IN_RESULT_SIGNED_IN)
			token := requireSessionCookie(t, res, auth.CookieName, http.SameSiteStrictMode, false, false)
			for name, values := range res.Header {
				if strings.EqualFold(name, "Set-Cookie") {
					continue
				}
				if strings.Contains(strings.Join(values, "\n"), token) || strings.Contains(strings.Join(values, "\n"), testPassword) {
					t.Errorf("The response disclosed a credential in %s.", name)
				}
			}
			if bytes.Contains(body, []byte(token)) || bytes.Contains(body, []byte(testPassword)) {
				t.Error("The SignIn response body disclosed a credential.")
			}
			return token
		}

		firstToken := signIn(httpA)
		requireServer(t, clientA, true)
		requireServer(t, clientB, false)
		if signIn(httpB) == firstToken {
			t.Error("Independent sign-in returned the same credential.")
		}
		requireServer(t, clientA, true)
		requireServer(t, clientB, true)

		signedOut, err := clientA.SignOut(t.Context(), connect.NewRequest(&webv1.SignOutRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		requireNoStore(t, signedOut.Header())
		requireSessionCookie(t, &http.Response{Header: signedOut.Header()}, auth.CookieName, http.SameSiteStrictMode, false, true)
		requireServer(t, clientA, false)
		requireServer(t, clientB, true)

		stale := connect.NewRequest(&webv1.GetServerRequest{})
		stale.Header().Set("Cookie", auth.CookieName+"="+firstToken)
		got, err := clientA.GetServer(t.Context(), stale)
		if err != nil || got.Msg.GetSignedIn() {
			t.Fatalf("GetServer did not report the revoked token signed out. Error was %v.", err)
		}
		for _, procedure := range []string{webrpc.SessionServiceSignOutProcedure, webrpc.ClientServiceApproveClientProcedure} {
			req := jsonProcedure(t, procedure, "{}")
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: firstToken})
			res := serveRequest(handler, req)
			requireNoCookie(t, res.Header())
			requireConnectJSONError(t, res, http.StatusUnauthorized, connect.CodeUnauthenticated, sessionRequired)
		}
	})
}

func testSessionCookieModes(t *testing.T) {
	fixture := newSessionDatabase(t)
	const public = "https://console.example.test"
	const crossSite = "https://dashboard.other.test"
	handler := sessionHandler(t, fixture, public, crossSite)

	signIn := func(origin string, name string, sameSite http.SameSite) string {
		req := signInRequest(t, testPassword)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res := serveRequest(handler, req)
		requireSignInResult(t, res.Code, res.Body.Bytes(), webv1.SignInResult_SIGN_IN_RESULT_SIGNED_IN)
		return requireSessionCookie(t, res.Result(), name, sameSite, true, false)
	}
	strict := signIn("", auth.SecureCookieName, http.SameSiteStrictMode)
	cross := signIn(crossSite, auth.CrossSiteCookieName, http.SameSiteNoneMode)
	bothCookies := []*http.Cookie{{Name: auth.SecureCookieName, Value: strict}, {Name: auth.CrossSiteCookieName, Value: cross}}

	check := func(origin string, cookies []*http.Cookie, want bool) {
		req := jsonProcedure(t, webrpc.SessionServiceGetServerProcedure, "{}")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		res := serveRequest(handler, req)
		var msg webv1.GetServerResponse
		if err := protojson.Unmarshal(res.Body.Bytes(), &msg); res.Code != http.StatusOK || err != nil || msg.GetSignedIn() != want {
			t.Errorf("GetServer for origin %q returned HTTP %d and signed_in %t with decoding error %v, want HTTP 200 and signed_in %t.", origin, res.Code, msg.GetSignedIn(), err, want)
		}
	}
	check("", bothCookies[1:], false)
	check(crossSite, bothCookies[:1], false)
	check(crossSite, bothCookies, true)

	req := jsonProcedure(t, webrpc.SessionServiceSignOutProcedure, "{}")
	req.Header.Set("Origin", crossSite)
	for _, cookie := range bothCookies {
		req.AddCookie(cookie)
	}
	res := serveRequest(handler, req)
	if res.Code != http.StatusOK {
		t.Fatalf("Cross-site SignOut returned HTTP %d, want 200.", res.Code)
	}
	requireSessionCookie(t, res.Result(), auth.CrossSiteCookieName, http.SameSiteNoneMode, true, true)
	check("", bothCookies, true)
	check(crossSite, bothCookies, false)
}

type failedDiscard struct{}

func (failedDiscard) Read([]byte) (int, error) {
	return 0, errors.New("fixture body ended during discard")
}

func testSessionReadLimit(t *testing.T) {
	fixture := newSessionDatabase(t)
	handler := sessionHandler(t, fixture, loopbackOrigin)
	// Trailing JSON whitespace puts a valid SignIn one byte above the transport limit.
	prefix := passwordJSON(testPassword)
	body := prefix + strings.Repeat(" ", (32<<10)+1-len(prefix))
	var compressed bytes.Buffer
	zipper := gzip.NewWriter(&compressed)
	if _, err := io.WriteString(zipper, body); err != nil {
		t.Fatal(err)
	}
	if err := zipper.Close(); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, encoding string
		reader         io.Reader
		fragment       string
	}{
		{"complete plain", "", strings.NewReader(body), "message size 32769"},
		{"complete compressed", "gzip", bytes.NewReader(compressed.Bytes()), "message size 32769"},
		{"interrupted discard", "", io.MultiReader(strings.NewReader(body), failedDiscard{}), "unable to determine message size"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, loopbackOrigin+webrpc.SessionServiceSignInProcedure, tc.reader)
			req.Header.Set("Connect-Protocol-Version", "1")
			req.Header.Set("Content-Type", "application/json")
			if tc.encoding != "" {
				req.Header.Set("Content-Encoding", tc.encoding)
			}
			res := serveRequest(handler, req)
			requireNoCookie(t, res.Header())
			requireNoStore(t, res.Header())
			var response struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			err := json.Unmarshal(res.Body.Bytes(), &response)
			if res.Code != http.StatusTooManyRequests || err != nil || response.Code != "resource_exhausted" ||
				!strings.Contains(response.Message, "configured max 32768") || !strings.Contains(response.Message, tc.fragment) {
				t.Errorf("The oversized request returned HTTP %d and %q, want resource_exhausted with the configured maximum.", res.Code, res.Body.String())
			}
			if len(sessionRows(t, fixture.observer)) != 0 || feedRows(t, fixture.observer) != 0 {
				t.Error("An oversized SignIn changed session state.")
			}
		})
	}

	t.Run("maximum password and message accepted", func(t *testing.T) {
		password := strings.Repeat("é", 2048)
		fixture := sessionDatabaseWithPassword(t, password)
		handler := sessionHandler(t, fixture, loopbackOrigin)
		input := passwordJSON(password)
		req := jsonProcedure(t, webrpc.SessionServiceSignInProcedure, input+strings.Repeat(" ", (32<<10)-len(input)))
		res := serveRequest(handler, req)
		requireSignInResult(t, res.Code, res.Body.Bytes(), webv1.SignInResult_SIGN_IN_RESULT_SIGNED_IN)
		requireSessionCookie(t, res.Result(), auth.CookieName, http.SameSiteStrictMode, false, false)
	})
}

func testSessionRollback(t *testing.T) {
	for _, tc := range []struct {
		name, event, sqlState string
		status                int
		code                  connect.Code
		message               string
	}{
		{"SignIn rollback", "INSERT", "23514", http.StatusServiceUnavailable, connect.CodeUnavailable, sessionUnavailable},
		{"SignIn retryable rollback", "INSERT", "40001", http.StatusConflict, connect.CodeAborted, "The database was busy, retry the request."},
		{"SignOut rollback", "DELETE", "23514", http.StatusServiceUnavailable, connect.CodeUnavailable, sessionUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newSessionDatabase(t)
			handler := sessionHandler(t, fixture, loopbackOrigin)
			req := signInRequest(t, testPassword)
			if tc.event == "DELETE" {
				token := issueTestSession(t, fixture.sessions)
				req = jsonProcedure(t, webrpc.SessionServiceSignOutProcedure, "{}")
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
			}
			rowsBefore, feedBefore := sessionSnapshot(t, fixture.observer), feedRows(t, fixture.observer)
			storetest.Exec(t, fixture.observer, fmt.Sprintf(`
CREATE FUNCTION fixture_reject_session() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'fixture-private-database-diagnostic' USING ERRCODE = '%s';
END;
$$;
CREATE CONSTRAINT TRIGGER fixture_reject_session AFTER %s ON web_sessions
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fixture_reject_session();`, tc.sqlState, tc.event))

			res := serveRequest(handler, req)
			requireNoCookie(t, res.Header())
			requireNoStore(t, res.Header())
			requireConnectJSONError(t, res, tc.status, tc.code, tc.message)
			if !slices.Equal(sessionSnapshot(t, fixture.observer), rowsBefore) || feedRows(t, fixture.observer) != feedBefore {
				t.Error("A confirmed rollback changed the session rows or feed.")
			}
		})
	}
}

func installSessionCommitGate(t *testing.T, observer *pgx.Conn) {
	t.Helper()
	storetest.Exec(t, observer, `
CREATE SEQUENCE fixture_issue_attempts;
CREATE FUNCTION fixture_count_issue() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM nextval('fixture_issue_attempts');
    RETURN NEW;
END;
$$;
CREATE TRIGGER fixture_count_issue BEFORE INSERT ON web_sessions
    FOR EACH ROW EXECUTE FUNCTION fixture_count_issue();`)
	storetest.Exec(t, observer, fmt.Sprintf(`
CREATE FUNCTION fixture_hold_commit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(%d::bigint);
    RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER fixture_hold_commit AFTER INSERT ON feed
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fixture_hold_commit();`, sessionCommitGate))
	// The Store flushes deferred constraints before writing the feed. Defer the
	// fixture gate again before its feed event is queued so it fires in COMMIT.
	storetest.Exec(t, observer, `
CREATE FUNCTION fixture_defer_commit_gate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    SET CONSTRAINTS fixture_hold_commit DEFERRED;
    RETURN NULL;
END;
$$;
CREATE TRIGGER fixture_defer_commit_gate BEFORE INSERT ON feed
    FOR EACH STATEMENT EXECUTE FUNCTION fixture_defer_commit_gate();`)
}

func interruptSessionCommit[R any](t *testing.T, observer *pgx.Conn, run func(context.Context) R) R {
	t.Helper()
	blocker := storetest.Scalar[int32](t, observer, "SELECT pg_backend_pid()")
	release := storetest.HoldAdvisoryGate(t, observer, sessionCommitGate)
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout+5*time.Second)
	defer cancel()
	attempt := storetest.Start(t, func() R { return run(ctx) })

	var writer int32
	storetest.Poll(t, "the SignIn writer waiting inside COMMIT", func() bool {
		select {
		case <-attempt.Done():
			t.Fatal("The SignIn operation returned before reaching COMMIT.")
		default:
		}
		writer = storetest.Scalar[int32](t, observer, `
SELECT COALESCE((
    SELECT pid FROM pg_stat_activity
    WHERE datname = current_database() AND state = 'active' AND upper(query) = 'COMMIT'
      AND $1 = ANY (pg_blocking_pids(pid))
    LIMIT 1
), 0)`, blocker)
		return writer != 0
	})
	storetest.WaitForBlock(t, observer, writer, blocker, storetest.BigintKey(sessionCommitGate))
	if !storetest.Scalar[bool](t, observer, "SELECT pg_terminate_backend($1, 3000)", writer) {
		t.Fatal("PostgreSQL did not terminate the committing backend.")
	}
	release()
	return attempt.Wait(t)
}

func testSessionUnknownCommit(t *testing.T) {
	fixture := newSessionDatabase(t)
	observer := fixture.observer
	handler := sessionHandler(t, fixture, loopbackOrigin)
	installSessionCommitGate(t, observer)
	req := signInRequest(t, testPassword)
	res := interruptSessionCommit(t, observer, func(ctx context.Context) *httptest.ResponseRecorder {
		return serveRequest(handler, req.WithContext(ctx))
	})
	requireNoCookie(t, res.Header())
	requireNoStore(t, res.Header())
	requireConnectJSONError(t, res, http.StatusServiceUnavailable, connect.CodeUnavailable, outcomeUnknown)
	if count := storetest.Scalar[int64](t, observer, "SELECT CASE WHEN is_called THEN last_value ELSE 0 END FROM fixture_issue_attempts"); count != 1 {
		t.Errorf("An unconfirmed SignIn attempted %d inserts, want one.", count)
	}
	if len(sessionRows(t, observer)) != 0 || feedRows(t, observer) != 0 {
		t.Error("The interrupted transaction or a retry left committed changes.")
	}
	requireNoReceipts(t, observer)
}

func TestSessionClockRefusal(t *testing.T) {
	at := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	fixture, setClock := clockSessionDatabase(t, at)
	handler := sessionHandler(t, fixture, loopbackOrigin)
	setClock(at.Add(time.Minute))
	token := issueTestSession(t, fixture.sessions)
	setClock(at)
	rows := sessionSnapshot(t, fixture.observer)
	before := feedRows(t, fixture.observer)

	res := serveRequest(handler, signInRequest(t, testPassword))
	requireNoCookie(t, res.Header())
	requireNoStore(t, res.Header())
	requireConnectJSONError(t, res, http.StatusConflict, connect.CodeAborted, "The server could not issue a session, retry sign-in.")
	verifier := verifierOf(t, token)
	if got := sessionRows(t, fixture.observer); !slices.EqualFunc(got, [][]byte{verifier[:]}, bytes.Equal) ||
		!storetest.Scalar[bool](t, fixture.observer, "SELECT created_at = $1 FROM web_sessions WHERE verifier = $2", at.Add(time.Minute), verifier[:]) ||
		!slices.Equal(sessionSnapshot(t, fixture.observer), rows) ||
		feedRows(t, fixture.observer) != before {
		t.Error("Clock refusal changed the original session or the feed.")
	}
}

func TestSessionRequestKeys(t *testing.T) {
	fixture := newSessionDatabase(t)
	handler := sessionHandler(t, fixture, loopbackOrigin)
	for _, tc := range []struct {
		name string
		keys []string
	}{
		{"present", []string{"fixture-key"}},
		{"empty", []string{""}},
		{"repeated", []string{"", "second-fixture-key"}},
	} {
		t.Run("SignIn/"+tc.name, func(t *testing.T) {
			req := signInRequest(t, testPassword)
			req.Header[http.CanonicalHeaderKey(store.RequestKeyHeader)] = tc.keys
			res := serveRequest(handler, req)
			requireNoCookie(t, res.Header())
			requireNoStore(t, res.Header())
			requireConnectJSONError(t, res, http.StatusBadRequest, connect.CodeInvalidArgument, requestKeyDisallowed)
			if len(sessionRows(t, fixture.observer)) != 0 || feedRows(t, fixture.observer) != 0 {
				t.Error("A keyed SignIn changed session state.")
			}
			requireNoReceipts(t, fixture.observer)
		})
	}

	t.Run("missing session precedes key refusal", func(t *testing.T) {
		req := jsonProcedure(t, webrpc.SessionServiceSignOutProcedure, "{}")
		req.Header.Set(store.RequestKeyHeader, "fixture-key")
		res := serveRequest(handler, req)
		requireNoCookie(t, res.Header())
		requireConnectJSONError(t, res, http.StatusUnauthorized, connect.CodeUnauthenticated, sessionRequired)
	})

	t.Run("lost sign-in reply and sign-out retry", func(t *testing.T) {
		for range 7 {
			issueTestSession(t, fixture.sessions)
		}
		original := sessionRows(t, fixture.observer)
		if len(original) != 7 {
			t.Fatalf("The fixture holds %d original sessions, want seven.", len(original))
		}

		signIn := func() string {
			res := serveRequest(handler, signInRequest(t, testPassword))
			requireSignInResult(t, res.Code, res.Body.Bytes(), webv1.SignInResult_SIGN_IN_RESULT_SIGNED_IN)
			return requireSessionCookie(t, res.Result(), auth.CookieName, http.SameSiteStrictMode, false, false)
		}
		first := signIn()
		firstVerifier := verifierOf(t, first)
		if got := sessionRows(t, fixture.observer); !slices.EqualFunc(got, append(slices.Clone(original), firstVerifier[:]), bytes.Equal) {
			t.Error("The first committed SignIn did not retain the seven original sessions.")
		}
		// Discard the first reply as a caller would after losing it and send a fresh request.
		second := signIn()
		secondVerifier := verifierOf(t, second)
		expected := append(slices.Clone(original[1:]), firstVerifier[:], secondVerifier[:])
		if second == first || !slices.EqualFunc(sessionRows(t, fixture.observer), expected, bytes.Equal) {
			t.Error("A fresh SignIn must issue a distinct token, keep eight rows and evict only the oldest original session.")
		}

		signOut := func(keys []string) *httptest.ResponseRecorder {
			req := jsonProcedure(t, webrpc.SessionServiceSignOutProcedure, "{}")
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: second})
			if keys != nil {
				req.Header[http.CanonicalHeaderKey(store.RequestKeyHeader)] = keys
			}
			return serveRequest(handler, req)
		}

		head := feedRows(t, fixture.observer)
		keyed := signOut([]string{""})
		requireNoCookie(t, keyed.Header())
		requireConnectJSONError(t, keyed, http.StatusBadRequest, connect.CodeInvalidArgument, requestKeyDisallowed)
		if !slices.EqualFunc(sessionRows(t, fixture.observer), expected, bytes.Equal) || feedRows(t, fixture.observer) != head {
			t.Error("A refused keyed SignOut changed the session rows or feed.")
		}
		get := jsonProcedure(t, webrpc.SessionServiceGetServerProcedure, "{}")
		get.AddCookie(&http.Cookie{Name: auth.CookieName, Value: second})
		get.Header.Set(store.RequestKeyHeader, "fixture-key")
		res := serveRequest(handler, get)
		var server webv1.GetServerResponse
		if err := protojson.Unmarshal(res.Body.Bytes(), &server); res.Code != http.StatusOK || err != nil || !server.GetSignedIn() {
			t.Errorf("GetServer with a request key returned HTTP %d and signed_in %t with decoding error %v, want a signed-in success.", res.Code, server.GetSignedIn(), err)
		}

		res = signOut(nil)
		if res.Code != http.StatusOK {
			t.Fatalf("SignOut returned HTTP %d, want 200.", res.Code)
		}
		requireSessionCookie(t, res.Result(), auth.CookieName, http.SameSiteStrictMode, false, true)
		if feedRows(t, fixture.observer) != head+1 {
			t.Error("A successful SignOut must record one feed row.")
		}
		res = signOut(nil)
		requireNoCookie(t, res.Header())
		requireConnectJSONError(t, res, http.StatusUnauthorized, connect.CodeUnauthenticated, sessionRequired)
		if feedRows(t, fixture.observer) != head+1 ||
			!slices.EqualFunc(sessionRows(t, fixture.observer), expected[:len(expected)-1], bytes.Equal) {
			t.Error("The repeated SignOut changed the feed or another session.")
		}
		requireNoReceipts(t, fixture.observer)
	})
}
