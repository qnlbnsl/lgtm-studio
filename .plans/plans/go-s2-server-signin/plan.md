---
topic: go-s2-server-signin
status: attack
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# The Go server starts and the owner signs in

## Overview

The existing website will be able to sign in to the Go server. An ordinary restart keeps the owner's browser signed in. A password change ends the existing sessions, and the owner signs in again with the new password.

The Go server stores session records in PostgreSQL and retrieves the machine-link signing key from OpenBao. It finishes both checks before accepting connections. This keeps a previously issued cookie valid only when its password is still current, and gives the later machine link a stable server identity.

The bundled website and server share one address. A separately hosted website can sign in when its origin is listed and its browser permits the required cookie. The release checks cover sign-in, restart, password revocation and signing-key persistence on the Compose stack.

## Key decisions

The single owner signs in with the password supplied to the server. The browser holds a cookie that its page scripts cannot read. PostgreSQL holds a verifier and the metadata that binds the session to the configured password. An ordinary restart keeps the session, and changing the password revokes the existing sessions before the server accepts requests.

The bundled website uses a strict cookie. A website on a different origin must be explicitly listed. If that origin is also on another site, both sides use encrypted web addresses and the browser must permit cross-site cookies.

The server starts serving after it has reconciled existing sessions and confirmed the machine-link signing key in OpenBao. If OpenBao cannot provide the key, the server exits and the stack can retry startup. The key remains the same when the stack is recreated.

The plan assumes that sign-in refuses a retry key because its reply issues a credential. After a lost reply the owner can sign in again. The earlier session remains subject to the same expiry and eight-session limit.

The full choice log is in [decisions.md](decisions.md).

## Workstreams

### The owner's browser session survives an ordinary restart

The server stores enough information to recognize a valid cookie and reject an expired or revoked one. Concurrent sign-ins must leave at most eight sessions and remove the oldest first. The server proves password revocation by refusing a cookie issued before a password change, even if the old password is later restored.

### The existing website can sign in to the Go server

The website receives the same product name, contract revision and sign-in results it already reads. A browser from another origin must pass an exact allowlist check. An unauthenticated call to a protected procedure receives a sign-in error, including when the service behind the procedure has yet to ship.

### The server verifies its prerequisites before accepting requests

Credentials and settings come from the environment and the stack's protected files. The server migrates PostgreSQL, reconciles sessions and reads the signing key before opening its listener. A Go test can start and restart the same server in its own process for the later machine-link checks.

## Rollout order

1. Merge the generated contract, the database layer and the stack's OpenBao setup. Give the build tests a disposable PostgreSQL with database creation permission.
2. Land the session schema and the server's request policies, then exercise startup and shutdown through the same entry point used by the server command. The first startup writes the auth schema. A binary that lacks this migration manifest cannot be used against that database without a compatible manifest.
3. Start the stack with the configured web password. Exercise sign-in and a server restart before changing the password in a disposable stack. Changing or removing it ends existing browser sessions. Recreate the stack and compare the two public signing-key fingerprints.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Build loop.** The Server team implements nine items within `src/server/`. Each test writer starts from the named files, assertions and real fixtures below. Compile against stubs and require the first run to fail at the observable assertion. A build error, crash or fixture timeout does not count as red. Freeze the approved tests. Check the builder's exact commit, run the named mutation in a disposable merge or branch, and require the intended assertion to fail before dropping the mutation. Run merged tests before advancing the team branch. [Build loop, phase 4](../../../docs/specs/2026-11-05-build-loop-design.md#4-build).

**WS-1. The owner's browser session survives an ordinary restart**

PostgreSQL supplies the order and the current session record. The server can check a cookie without re-deriving its password binding on each request. Startup checks the binding before a session can be used.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | The server accepts the exact configured password and refuses a different or oversized input. | `TestPasswordMatchesExactBytes` and `TestPasswordBindingVector`. |
| 2 | A session survives reconciliation with the same password. A changed or removed password leaves zero session rows and keeps old cookies invalid after password restoration. | `TestReconcilePassword`, `TestStartRevokesBeforeListen` and `deploy/test/checks.sh --only C4`. |
| 3 | A new cookie contains 32 random bytes. Issuance, expiry and logout preserve the eight-session cap and affect the intended records. | `TestSessionLifecycle`, `TestSessionIssueSerializes`, `TestSessionWriteRollsBack` and `TestSessionFeedRefs`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 2 | Verify the password and construct the per-session binding. | Target `src/server/internal/auth/password.go`, `Password.Matches`. S2 gives the neighboring pattern. |
| 3 | Migrate and reconcile session rows before startup. | Target `src/server/internal/auth/reconcile.go`, `Reconcile`, and `auth/migrations/0001_web_sessions.sql`, `web_sessions`. S3 gives the store pattern. |
| 4 | Issue, read and revoke sessions through the store. | Target `src/server/internal/auth/sessions.go`, `Sessions.Issue`, `Sessions.CheckInRead`. S4 gives the neighboring pattern. |

**Item 2. Password verification and binding**

Create `src/server/internal/auth/password.go` and `src/server/internal/auth/password_test.go`. This item needs the root dependency handoff for `golang.org/x/crypto/scrypt`. It can run alongside items 1, 5 and 8. It implements R9 and C3 and supplies the binding used for C4.

```go
const MaxPasswordBytes = 4096
type Password struct { /* private material */ }
func NewPassword(value string) (Password, error)
func (p Password) Matches(candidate string) bool
```

Use the exact 8-bit Unicode Transformation Format (UTF-8) input bytes. Reject a configured empty, non-UTF-8 or overlong password in `NewPassword` and reject an empty or overlong candidate in `Matches`. Compare fixed 32-byte Secure Hash Algorithm 256 (SHA-256) digests with `crypto/subtle.ConstantTimeCompare`. The length gate bounds work without comparing the candidate length to the configured length. Keep the comparison in a private `equalDigest(a, b [32]byte) bool` so the test can inspect the security primitive without unreliable wall-clock timing.

The private `bindPassword(p Password, salt [16]byte, verifier [32]byte, createdAt time.Time) ([32]byte, error)` derives 32 bytes with scrypt using `N=65536`, `r=8` and `p=1`. Use those bytes as a Hash-based Message Authentication Code (HMAC) SHA-256 key. Authenticate the concatenation of the ASCII (American Standard Code for Information Interchange) label `lgtm-web-session-v1`, one zero byte, the binary verifier, and the creation time as a signed big-endian 64-bit Unix microsecond count. Clear the derived key after use. Provide a private derivation function so issuance can derive before taking the database lock and compute the HMAC after obtaining the creation time. The constructor and method retain no formatted credential output.

Write these tests first in `password_test.go`.

- `TestPasswordMatchesExactBytes` checks a password containing spaces and non-ASCII characters. Assert rejection after trimming, changing a byte, passing the empty string or exceeding 4096 bytes. Assert the 4096-byte valid boundary and that the constructor rejects 4097 bytes. Parse `password.go` as Go source and check that the equality helper uses `subtle.ConstantTimeCompare` on the two digests and that `Matches` uses that helper's result.
- `TestPasswordBindingVector` uses salt bytes 0 through 15, verifier bytes 0 through 31, Unix microseconds `1795674600123456` and password `fixture password`. Assert the tag equals `bb95f4bbe7cfe1874b6899129e91dc7c7566b70670fd95355088eab81641e1df`. Assert that changing the password, salt, verifier or timestamp changes the tag.

This item is risky. Replacing the constant-time primitive with ordinary array equality must fail `TestPasswordMatchesExactBytes`. Changing the scrypt cost from `65536` to `32768` must fail `TestPasswordBindingVector`. The two named tests gate acceptance.

**Item 3. Session schema and password reconciliation**

Create `src/server/internal/auth/migrations/0001_web_sessions.sql`, `src/server/internal/auth/reconcile.go` and `src/server/internal/auth/reconcile_test.go`. This item needs item 2 and `go-s1-server-store`. It can run alongside items 1, 5 and 8. It implements D3, D4, D5, R1, R3, R4, R7, R9, R10, C2 and C4.

```go
func Migrations() store.Migrations
func Reconcile(ctx context.Context, s *store.Store, password *Password) error
```

`Migrations()` returns `store.Migrations{Part: "auth", Files: ...}`, with the embedded files rooted at `fs.Sub(embedded, "migrations")`. The application registers the auth manifest before running `Store.Migrate`. The migration creates just `web_sessions`.

| Column | Type and constraint | Meaning |
|---|---|---|
| `verifier` | `bytea PRIMARY KEY CHECK (octet_length(verifier) = 32)` | SHA-256 of the binary random token. |
| `created_at` | `timestamptz NOT NULL CHECK (isfinite(created_at))` | The database creation time, with microsecond precision. |
| `password_salt` | `bytea NOT NULL CHECK (octet_length(password_salt) = 16)` | An independent random salt. |
| `password_binding` | `bytea NOT NULL CHECK (octet_length(password_binding) = 32)` | The tag from item 2. |

Define private helpers `lockSessions` and `recordSessionChange` in `reconcile.go` for items 3 and 4. Run auth changes within `Store.Write`. Acquire `LOCK TABLE web_sessions IN SHARE ROW EXCLUSIVE MODE` as the first domain statement. It serializes changes and allows ordinary readers to finish. Use the primary connection for authorization decisions. For each changed row call `tx.Record` with topic `web_sessions`, scope `""`, and the lowercase hexadecimal SHA-256 of `lgtm-web-session-feed-v1`, a zero byte and the binary verifier as key. The feed reference is derived independently of the token and the password fields. It cannot authorize a request. This table has no browser-visible record identifier. Use the store's own feed flush and surface its commit error.

A nil password means the operator removed `LGTM_WEB_PASSWORD`. Delete the rows and record their removal. With a configured password, read the live set under the lock, recompute its bindings and compare fixed-size tags in constant time. If any live binding fails, delete the whole web session set together, including expired rows. Otherwise remove only expired rows. Each expiry comparison uses database time and `INTERVAL '720 hours'`. Return success only after commit. No startup check reads a local session file.

Write these tests first in `reconcile_test.go`. Use `storetest/migrated.NewStore` or call `Store.Migrate` on the empty database from `storetest.NewDatabase`. Build row fixtures with independent tags from the item 2 layout and control expiry with database timestamps.

- `TestSessionMigration` runs the manifest twice. Inspect the catalog for exactly the four named columns with the stated constraints and the `auth=1` ledger entry. Insert an invalid verifier, salt or binding size separately and require a check violation. Assert the valid fixture can be inserted.
- `TestReconcilePassword` starts with multiple valid live rows and one expired row. Assert the same password keeps precisely the live rows. Reconcile with a changed password and observe zero rows and one removal reference per deleted row. Repeat for nil and for one tampered live binding mixed with a valid binding. Restore the former password and assert the deleted live rows never reappear.
- `TestReconcileRollsBack` installs a test trigger that refuses one delete in a multi-row purge. Assert that reconciliation returns an error and that an independent read sees the original rows and feed head.

This item is risky. Skipping recomputation for a valid-length binding must fail `TestReconcilePassword`. Returning success before a committed purge must fail `TestReconcileRollsBack`. The three named tests gate acceptance.

**Item 4. Session issuance, verification and sign-out**

Create `src/server/internal/auth/sessions.go` and `src/server/internal/auth/sessions_test.go`. This item needs items 2 and 3 and the Store write and read handles. It can run alongside items 5 and 8. It implements D3, D4, D5, R3, R4, R7, R9, R10, C3 and C4.

```go
type Verifier [32]byte
type Sessions struct { /* private store and configured password */ }

const SessionLimit = 8
const SessionMaxAgeSeconds = 30 * 24 * 60 * 60
var ErrNoSession error

func NewSessions(s *store.Store, password Password) *Sessions
func TokenVerifier(token string) (Verifier, bool)
func (s *Sessions) Issue(ctx context.Context) (string, error)
func (s *Sessions) Check(ctx context.Context, verifier Verifier) error
func (s *Sessions) CheckInRead(ctx context.Context, tx *store.ReadTx, verifier Verifier) error
func (s *Sessions) Revoke(ctx context.Context, verifier Verifier) error
```

Only the verified password path in item 7 calls `Issue`. Generate 32 token bytes and a separate 16-byte salt with `crypto/rand` for each attempt. Expose the token only as canonical unpadded Uniform Resource Locator (URL)-safe base64. Before hashing a presented token, require exactly 43 characters and canonical re-encoding of 32 decoded bytes. The durable verifier is the digest of the binary token. Draw the entropy and derive the binding key before opening a write transaction. A random-source, derivation, insert or commit failure returns an empty token. An ambiguous commit may leave a session that expires or is evicted. There is no automatic second issue.

Under the item 3 table lock, sample `clock_timestamp()` once after acquiring the lock. Prune the expired rows. Select the creation time as the greater of that sample and the latest surviving creation time plus one microsecond. Insert the row with that timestamp and its bound tag. Keep the eight newest rows and remove the rest by `created_at DESC, verifier DESC`. Record the saved and removed references in the same transaction. Return the token after commit. Use the database clock as authoritative. Its movement can affect elapsed-time expiry, and the monotonic insertion time breaks ties.

`Check` calls `CheckInRead` from `Store.Read`. The read looks up only the provided verifier with `created_at > clock_timestamp() - INTERVAL '720 hours'`. A missing or expired row yields `ErrNoSession`. A database failure retains its error identity. `Revoke` takes the same lock and deletes one matching live row with the same expiry predicate. Zero matches yields `ErrNoSession`. The sprint 3 Machines service calls `CheckInRead` from its `watch.Source.Authorize` callback using the verifier from the router's context. It maps `ErrNoSession` to `watch.ErrWatchUnauthorized` and delegates backend failures to the Watch error path. Its browser request principal for keyed writes is the constant `owner`. The token and verifier stay out of that principal and out of receipts.

Write these tests first in `sessions_test.go`, using real PostgreSQL.

- `TestSessionLifecycle` issues nine distinct tokens, verifies canonical decoding and checks the four-column persisted row against the independent binding. Assert the first token fails and the remaining eight pass. Mark a fixture row exactly 720 hours old by advancing its setup timestamp relative to the database clock. Assert it fails authorization. Revoke one live token and prove the other live tokens still pass. Assert repeated revoke yields `ErrNoSession`.
- `TestSessionIssueSerializes` starts with seven committed rows. A test-only insert trigger blocks writer A with a PostgreSQL advisory gate after it has reached the insert. Start writer B using a different backend. Query `pg_locks` and `pg_blocking_pids`, and require B to wait on A's table lock before reaching the trigger. Release A and wait for both successes. Assert that precisely the original oldest token is gone, the other six original tokens plus A and B still authorize, and the observed creation times put A before B. The test queries the lock mode and uses a separate observer. A timeout waiting for a fixture is separate from failure of the lock assertion.
- `TestSessionWriteRollsBack` starts with eight live rows, installs a trigger that fails eviction, and invokes `Issue`. Require an empty token on error, the original eight verifiers and the original feed head from another transaction.
- `TestSessionFeedRefs` issues and revokes one cookie. Read the two feed rows and check their topic, empty scope, derived opaque key and `saved` then `removed` changes. Assert the references and stored rows contain neither the raw token nor its encoded text. Assert published reference keys differ from the row verifier and contain no password or binding.

This item is risky. Removing the table lock must fail the lock observation in `TestSessionIssueSerializes`. Returning the token on a failed transaction must fail `TestSessionWriteRollsBack`. Removing the expiry predicate must fail `TestSessionLifecycle`. The four named tests gate acceptance.

**Design artifact.** The foundation design's Sign-in and web sessions section defines the behavior. The existing screen supplies the browser entry point.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A failed or overlapping sign-in issues a cookie for the wrong durable state. | High | Return a token after commit. Serialize insert and eviction and freeze the two-backend lock test. |
| A password change leaves an old cookie active. | High | Reconcile before serving and test restoration of the original password after a committed purge. |

**Verify.** Run `go test -race -count=1 ./src/server/internal/auth` with a real disposable PostgreSQL through the stack's test wrapper. Run C4 on the disposable stack after item 9. One browser pass is described under WS-2.

**Rollout detail.** For rollout steps 1 and 2, run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/auth`. Startup registers `auth` beside the built-in `store` migration. The owner can restore the configured password after the disposable C4 run, and then signs in again.

**WS-2. The existing website can sign in to the Go server**

The browser boundary selects the cookie policy before any session can be written. The request router applies that boundary and the session check consistently across the normal generated services.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 4 | Same-site sign-in uses Strict. A listed cross-site browser gets None and Secure. An unlisted or unsafe origin receives a refusal before a session is written. | `TestOriginPolicies`, `TestCookieAttributes`, `TestBrowserOriginGate` and the browser checks below. |
| 5 | The right password returns Signed In and the wrong password returns Rejected. A credential is issued only in the cookie. | `TestSessionContract` and `deploy/test/checks.sh --only C3`. |
| 6 | The two public web calls remain reachable. Protected unary and streaming calls require a cookie across supported transports. An authenticated unsupported service returns `unimplemented`. | `TestWebRoutesRequireSession`, `TestProtectedStreamAndProtocolCodes` and `TestSessionRequestKeys`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 5 | Validate the web origin and derive cookie attributes. | Target `src/server/internal/auth/origin.go`, `NewOriginPolicy` and `src/server/internal/auth/cookie.go`, `ReadCookie`. S5 gives the neighboring pattern. |
| 6 | Route generated services and guard the web boundary. | Target `src/server/internal/api/server.go`, `NewHandler` and `src/server/internal/api/session_check.go`, `SessionVerifier`. S6 gives the neighboring pattern. |
| 7 | Implement the generated Session service. | Target `src/server/internal/api/session.go`, `NewSessionService`. S7 gives the descriptor and consumer. |

**Item 5. Browser origins and cookies**

Create `src/server/internal/auth/origin.go`, `src/server/internal/auth/origin_test.go`, `src/server/internal/auth/cookie.go` and `src/server/internal/auth/cookie_test.go`. This item needs the root dependency handoff for `golang.org/x/net/publicsuffix`. It can run alongside items 1, 2, 3, 4 and 8 once their prerequisites are met. It implements D11, R8, R9 and C3.

```go
const CookieName = "lgtm_session"

type OriginPolicy struct { /* private public origin and allowlist */ }
type CookieMode struct {
    Secure   bool
    SameSite http.SameSite
}

func NewOriginPolicy(publicURL string, allowed []string) (*OriginPolicy, error)
func (p *OriginPolicy) Check(r *http.Request) (CookieMode, error)
func ReadCookie(r *http.Request) (string, bool)
func SessionCookie(token string, mode CookieMode) *http.Cookie
func ClearSessionCookie(mode CookieMode) *http.Cookie
```

Canonicalize scheme and ASCII host case and remove default ports. Accept a configured origin with an optional root slash. Require a serialized origin with no slash in a request's `Origin` header. Reject empty or multiple header values, `null`, user information, paths beyond the configured root slash, queries, fragments, trailing dots, wildcards, malformed ports and non-ASCII host spellings. The administrator uses browser-serialized names. Allow HTTPS and plaintext on exact `localhost` or literal loopback addresses. Never resolve a name to decide whether it is loopback. Compare normalized origins as exact scheme, host and port tuples. The configured public origin is accepted without an allowlist entry. A different origin is accepted only if it is explicitly listed. Reject a cross-site entry at startup unless both origins are HTTPS.

Determine the site from scheme and the effective registrable domain using the public suffix list, including its private entries. Use the exact normalized host for literal addresses or a host without a registrable domain. Ports affect origin equality and do not affect site equality. Use Strict for a same-site request, with Secure derived from the configured public scheme. Use None and Secure for a listed cross-site request. An absent Origin uses the public-origin cookie policy for native clients and same-origin requests. Refuse a request reporting `Sec-Fetch-Site: cross-site` with no Origin. Do not consult `Host`, `Forwarded`, `X-Forwarded-Host` or `X-Forwarded-Proto` for admission or cookie mode.

Inspect the request's cookie header lines for exactly one `lgtm_session` occurrence. Treat a malformed or duplicate occurrence as absent, including a duplicate on another `Cookie` line. The router checks canonical encoding through item 4 before using a returned value. Issue `Path=/`, `HttpOnly`, `Max-Age=2592000`, the selected `SameSite`, conditional `Secure` and no `Domain`. Clear with the same attributes, an empty value, `Max-Age=0` on the wire, and an expiry at the Unix epoch. Use a negative Go `http.Cookie.MaxAge` to serialize that deletion. Emit one cookie for a successful mutation.

Write these tests first in `origin_test.go` and `cookie_test.go`.

- `TestOriginPolicies` covers same origin, a listed different port or subdomain on the same site, two tenants under a private suffix, listed unrelated HTTPS sites, an unlisted same-site origin, an unlisted cross-site origin, plaintext loopback and rejected plaintext non-loopback. Check normalized default ports, invalid allowlist entries, duplicate and opaque Origin headers, absence with and without cross-site fetch metadata, and a forged forwarded scheme. Assert the exact mode or refusal for each case.
- `TestCookieAttributes` parses the emitted cookie strings. Assert the exact name, host-only behavior, path, age, HttpOnly, SameSite, Secure and expiry rules for all three modes. Verify deletion is serialized with `Max-Age=0` and check that a cookie jar deletes the matching host-only entry.
- `TestReadsOneCanonicalCookie` accepts a single well-formed fixture and rejects absent, malformed and duplicate values, including a malformed and valid occurrence on different header lines. Assert unrelated cookies do not affect the result.

This item is risky. Replacing the allowlist check with same-site admission must fail `TestOriginPolicies`. Removing Secure from the cross-site branch must fail `TestCookieAttributes`. The three named tests gate acceptance.

**Item 6. Request handling and the session guard**

Create `src/server/internal/api/server.go`, `src/server/internal/api/server_test.go`, `src/server/internal/api/session_check.go` and `src/server/internal/api/session_check_test.go`. This item needs items 4 and 5 and the sprint 0 generated contract. It can run alongside item 8. It implements D1, D11, R2, R8, R9 and C3.

```go
type Options struct {
    Sessions *auth.Sessions
    Origins  *auth.OriginPolicy
    Session  lgtmv1connect.SessionServiceHandler
    Clients  lgtmv1connect.ClientServiceHandler
    Link     linkv1connect.LinkServiceHandler
}

func NewHandler(options Options) http.Handler
func SessionVerifier(ctx context.Context) (auth.Verifier, bool)
```

Import web messages from `github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1` and the generated handlers from its `lgtmv1connect` child. Import machine-link handlers from the corresponding `gen/go/lgtm/link/v1/linkv1connect` package. Use the generated `NewSessionServiceHandler` and `NewClientServiceHandler` when implemented. Require the Connect protocol version header on Connect unary calls, and configure the error writer with the same option. Recognize the generated web procedures for `SessionService`, `ClientService`, `MapService` and `PreviewService`. A recognized web procedure for an absent implementation returns `unimplemented` after the guard. An unrecognized request in the protocol namespace uses the same guard and fallback. A non-protocol request outside these namespaces is a normal 404. Apply authentication to the same exact, canonical path that is dispatched. Return HTTP 400 for noncanonical aliases before routing.

Only `/lgtm.v1.SessionService/GetServer` and `/lgtm.v1.SessionService/SignIn` skip a mandatory web session. Exact link procedures `/lgtm.link.v1.LinkService/Join` and `/lgtm.link.v1.LinkService/Open` are a separate transport family. They return `unimplemented` in sprint 2, and sprint 3 supplies their token and key checks when it registers `Options.Link`. A supplied `Options.Clients` handler remains behind the web guard. Add an empty `GET /healthz` with status 204 once the application opens the listener. It serves readiness and carries no application data. The browser contract uses POST. Return HTTP 405 with `Allow: POST` for other methods on a procedure path, apart from browser preflights.

For web procedures, run the origin check before decoding request bodies or examining cookies. On an allowed request with Origin, return `Access-Control-Allow-Origin` with that exact origin, `Access-Control-Allow-Credentials: true` and `Vary: Origin`. Never emit a wildcard. A browser's cross-origin resource sharing (CORS) preflight must request POST, a known web procedure and only supported headers. Compare requested names case-insensitively. Allow `Content-Type`, `Connect-Protocol-Version`, `Connect-Timeout-Ms`, `Connect-Accept-Encoding`, `Connect-Content-Encoding`, `Grpc-Timeout`, `X-Grpc-Web`, `X-User-Agent` and `Lgtm-Request-Key`. Return 204, `Access-Control-Allow-Methods: POST`, the allowed requested headers, and also vary by `Access-Control-Request-Method` and `Access-Control-Request-Headers`. On allowed actual responses expose `Grpc-Status`, `Grpc-Message` and `Grpc-Status-Details-Bin`. A refused preflight returns 403 without CORS permission or service dispatch.

On a protected web procedure, read and verify the cookie through `auth.Sessions.Check`. Store only the verified digest and the selected cookie mode in request context. The raw credential never becomes context identity. `GetServer` does its own optional verification in item 7. Apply `Cache-Control: no-store` to the three Session procedures, including errors. Support Connect and Google Remote Procedure Call (gRPC) handlers on the same listener. Use the pinned Connect `ErrorWriter` for protocol-aware middleware errors, including streaming. Refused browser requests without a recognized media type receive bare 403. Use fixed one-sentence messages for application errors.

| Condition | Connect code | Public sentence |
|---|---|---|
| Absent, malformed, expired or revoked session on a protected procedure. | `unauthenticated` | Sign in to continue. |
| Refused origin on an actual protocol request. | `permission_denied` | This website origin is not allowed. |
| Authorized or independent Link request for an unbuilt procedure. | `unimplemented` | This call is not implemented. |
| Retryable store error. | `aborted` | The database was busy, retry the request. |
| Store or session state unavailable. | `unavailable` | Web session storage is unavailable. |
| Request canceled or deadline expired. | `canceled` or `deadline_exceeded` | The request was canceled. / The request timed out. |
| Unexpected application failure outside storage. | `internal` | The server could not complete the request. |

Tests first in the two named test files. Use a real migrated database wherever a test reads or creates a session. Supply generated handler probes that record entry without implementing feature behavior.

- `TestWebRoutesRequireSession` checks the two public procedures, each protected service family, readiness, and attempts to reach a public method using an encoded or noncanonical path. Require no protected handler entry without a cookie, entry with a valid cookie, and `unimplemented` on an authenticated Map or Preview call. Verify the early Link placeholder has its independent policy at the exact two paths only.
- `TestBrowserOriginGate` sends allowed and denied preflights, a preflight with an unexpected header, and actual denied calls that would otherwise reach SignIn. Assert the expected status, headers, lack of dispatch and lack of cookie. Assert that the allowed actual browser calls include the exact credentialed headers even on an unauthenticated response.
- `TestProtectedStreamAndProtocolCodes` uses a Connect JSON unary client, a generated Connect streaming client and a generated gRPC client over HTTP/2. Require `unauthenticated` for a missing, noncanonical or duplicate cookie on unary and streaming requests. Supply a valid cookie and confirm the handler reads the expected verifier from context without gaining access to a plaintext credential through that API. Inject a store failure into a real fixture and assert that its fixed application error contains no fixture secrets.

This item is risky. Extending the session exception to the entire Session service prefix must fail `TestWebRoutesRequireSession`. Moving origin validation behind dispatch must fail `TestBrowserOriginGate`. The three named tests gate acceptance.

**Item 7. The three Session calls**

Create `src/server/internal/api/session.go` and `src/server/internal/api/session_test.go`. This item needs items 2, 4, 5 and 6. It can run alongside item 8. It implements D1, D11, R2, R8, R9, C3 and C4.

```go
const Product = "lgtm-studio"
const ContractRevision uint32 = 1
const Version = "0.1.0"

func NewSessionService(password auth.Password, sessions *auth.Sessions) lgtmv1connect.SessionServiceHandler
```

Use the generated `SessionService` methods `GetServer`, `SignIn` and `SignOut` without changing the contract. Return `GetServerResponse.product`, `version`, `contract`, `sample=false`, and `signed_in` from an optional cookie lookup. Missing, bad or expired cookies produce `signed_in=false`. Database failure for a syntactically valid cookie produces the mapped storage error. Anonymous `GetServer` can respond without querying for a session.

`SignIn` accepts only `SignInRequest.password` from the decoded body. Limit the generated Session handler's message reads to 32 KiB and the password itself to 4096 bytes. Never read the password from a query or header. Reject any present `store.RequestKeyHeader`, including empty or repeated fields, with `invalid_argument` and “This call does not accept a request key.” Without that header, a wrong password returns `SIGN_IN_RESULT_REJECTED` in a successful protocol reply with no new cookie. A valid password calls `Issue`. A committed issue returns `SIGN_IN_RESULT_SIGNED_IN` and the cookie selected by the origin guard. Return `resource_exhausted` for an over-limit protocol body. Neither the password nor any token appears in a response body, response metadata other than Set-Cookie, debug log or receipt.

`SignOut` first passes the guard. A present request key receives the same `invalid_argument` refusal as SignIn. It revokes the context verifier through `Sessions.Revoke` and clears the cookie only after commit. A session that vanished after the guard produces `unauthenticated`. A store failure leaves the cookie unchanged. GetServer ignores the header. No Session call uses `Store.WriteOnce`. Retryable keyed web mutations in later services authenticate and authorize first, then use `Store.WriteOnce` with principal `owner`, a generated procedure name and the service's semantic fingerprint. The sprint 3 join-token creation call gets the same credential-returning exception under the stated assumption.

Write these tests first in `session_test.go` over the normal router with a real migrated database, using two independent cookie jars where needed.

- `TestSessionContract` checks anonymous GetServer and the exact product, version, revision and sample flag. Assert wrong password returns `REJECTED` with no cookie. A password offered in a query or header with an empty body password is rejected. The right password returns `SIGNED_IN` and only a Set-Cookie credential. Follow GetServer before and after sign-out in both jars. After a successful sign-out, require the old token to fail and the other jar to remain signed in. Cause issuance or revocation to fail with a fixture trigger, and assert no cookie issue or deletion on error.
- `TestSessionRequestKeys` sends present, empty and repeated request-key headers with a correct SignIn password. Assert `invalid_argument`, no cookie, no new session and no receipt. Retry an unkeyed successful SignIn with its reply deliberately discarded, then assert a new token is issued and both resulting rows match the retry rule. Check the missing-cookie priority for SignOut. Check that a keyed authenticated SignOut refuses the key and leaves the session alive. Inspect the receipts table for zero auth receipts after the sequence.

This item is risky. Moving cookie issue before commit must fail the failure case in `TestSessionContract`. Accepting a request key on SignIn must fail `TestSessionRequestKeys`. The two named tests gate acceptance.

**Design artifact.** The current browser uses the generated Session client through `src/frontend/src/server/api.ts`. This workstream keeps that entry point.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An unapproved origin reaches a credential-changing handler. | High | Gate by exact origin and exercise both preflight and actual requests. |
| An auth exception reaches a protected procedure or exposes a session. | High | Match exact procedure names, cover multiple transports and inspect response and receipt contents. |

**Verify.** Run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/api`. Run C3 on the disposable stack. In an ordinary browser, open the bundled website, sign in, reload and sign out. In a browser profile that allows cross-site cookies, host the unchanged website at a distinct allowed HTTPS site. Confirm sign-in and a reload retain the session and inspect None and Secure on the cookie. Remove that origin from the allowlist, restart and confirm its request receives no permission or session. Also exercise same-site different origin with the listed port. Browser policy controls cross-site cookie availability.

**Rollout detail.** In rollout step 3, leave the allowlist empty for the bundled website. Set listed origins only if serving another copy of the site. Use the same configured public origin that the proxy serves. Run `deploy/test/checks.sh --only C1,C2,C3,C4,C13` against the disposable Compose stack.

**WS-3. The server verifies its prerequisites before accepting requests**

The server command starts the same application used by the in-process test entry point. It checks the database and the vault before making its private listener available to the proxy.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 7 | The configured password is required, and migrations and password reconciliation finish before the first connection can be accepted. | `TestReadServerSettings`, `TestStartRevokesBeforeListen` and `deploy/test/checks.sh --only C2`. |
| 8 | First startup records one signing key in OpenBao. Recreation loads the same key and reports its public fingerprint. A vault failure leaves the listener closed. | `TestLinkSigningKeyPersistence`, `TestLinkSigningKeyCreateRace`, `TestStartWithUnavailableVault` and `deploy/test/checks.sh --only C13`. |
| 9 | A Go test outside the server's internal packages can start and stop the application and restart it at the same address with the same database and key. | `TestStartAndRestartServer`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Parse the stack and local settings and assemble the database connection. | Target `src/server/internal/config/config.go`, `Load`. S1 gives the Stack and Store patterns. |
| 8 | Read the OpenBao credential and load the persisted key. | Target `src/server/internal/vault/vault.go`, `Open` and `Client.LinkSigningKey`. S8 gives the Stack and wire patterns. |
| 9 | Start the command and expose the in-process test entry point. | Targets `src/server/internal/app/start.go`, `Start`, `src/server/cmd/lgtm-server/main.go`, `main`, and `src/server/servertest/server.go`, `Start`. S9 gives the neighboring commands and probes. |

**Item 1. Server settings**

Create `src/server/internal/config/config.go` and `src/server/internal/config/config_test.go`. This item needs the sprint 1 Store and Stack interfaces. It can run alongside items 2, 3, 5 and 8 as those become ready. It implements D3, D4, D10, D11, R9 and C2.

```go
type Config struct {
    ListenAddr         string
    PublicURL          string
    AllowedOrigins     []string
    Password           string
    Postgres           *pgxpool.Config
    AllowPrivateStack  bool
    OpenBaoAddr        string
    OpenBaoCredentialFile string
}

func Load(lookup func(string) (string, bool)) (Config, error)
```

`Load(os.LookupEnv)` is the command's environment entry point. Never print the assembled configuration. In the table, required file paths are deployment paths on the server, and the Stack plan supplies their mounts.

| Setting | Standalone value if unset | Compose value or rule |
|---|---|---|
| `LGTM_WEB_PASSWORD` | Required to serve. An unset or empty value is retained as absent for the reconciliation step. | Pass the operator's exact value through. The service must be creatable when unset so the executable can refuse startup for C2. |
| `LGTM_SERVER_LISTEN_ADDR` | `127.0.0.1:8080`. | `:8080` on the private container network. Do not publish this port. |
| `LGTM_PUBLIC_URL` | `http://127.0.0.1:8787`. | Set to the proxy's public origin, including the actual published port when loopback. This is an origin, without a deployment subpath. |
| `LGTM_ALLOWED_ORIGINS` | Empty. | Optional comma-separated browser origins. Trim outer whitespace of entries. Reject empty entries in a nonempty list. |
| `LGTM_POSTGRES_HOST` | `127.0.0.1`. | `postgres`. |
| `LGTM_POSTGRES_PORT` | `5432`. | `5432`. |
| `LGTM_POSTGRES_DATABASE` | `lgtm_app`. | `lgtm_app`. |
| `LGTM_POSTGRES_USER` | `lgtm_app`. | `lgtm_app`. |
| `LGTM_POSTGRES_PASSWORD_FILE` | Required. | `/run/lgtm/postgres.password`, a protected file with 64 lowercase hexadecimal characters and a trailing line feed. |
| `LGTM_ALLOW_PRIVATE_STACK` | `false`. | `true`, permitting plaintext to the two exact private service names for the isolated stack. |
| `LGTM_OPENBAO_CREDENTIAL_FILE` | Required to serve. | `/run/lgtm/openbao.json`. |
| `LGTM_OPENBAO_ADDR` | Required to serve. | The `openbao` service on port 8200 over plain Hypertext Transfer Protocol (HTTP) inside the stack. |

`LGTM_WEB_PORT` belongs to Stack and the check script. Stack translates it into `LGTM_PUBLIC_URL` for the server. The proxy uses the configured public host and routes the web and Link procedure paths to the private service. The health probe performs `GET /healthz` on the Go listener and requires 204. Include no web password in that probe.

Accept exactly one PostgreSQL host and explicit port. Parse with `pgxpool.ParseConfig` using explicit host, port, database, user, password and Transport Layer Security (TLS) mode. Use a single target and a pool size of eight. Use an explicit mode of `disable` for literal loopback, Unix sockets or the exact `postgres` host with private-stack opt-in. Use `verify-full` for other hosts. Reject driver service configuration or fallback that would substitute a different endpoint or weaken verification. Install no connection-string logging. Validate database settings first and let item 9 check public origin, listener and vault prerequisites after the password reconciliation. A malformed database setting prevents revocation and is reported together with the missing-password diagnostic when applicable.

Write tests first in `config_test.go`.

- `TestReadServerSettings` uses a lookup fixture and a protected temporary database password file. Check loopback defaults, the explicit Stack settings, opt-in behavior, a nondefault published web port passed as the public origin, literal password bytes and the missing password surviving configuration loading for reconciliation. Assert that removing opt-in stops plaintext to a service-name host. Check verified TLS for a non-loopback host.
- `TestSettingsErrorsRedactValues` uses invalid port, private-stack flag and file formats. Assert the applicable setting name appears and fixture secret values or full connection strings do not appear.

This item is risky. Treating arbitrary service names as eligible for plaintext must fail `TestReadServerSettings`. The two named tests gate acceptance.

**Item 8. OpenBao and the signing key**

Create `src/server/internal/vault/vault.go`, `src/server/internal/vault/vault_test.go` and `src/server/internal/vault/vaulttest/openbao.go`. This item needs the credential and policy contract in `go-s1-stack-vault`. It can run alongside items 1 through 7 as their dependencies allow. It implements D7, D9, D10, R9 and C13.

```go
type Client struct { /* private credentials and lease state */ }

func Open(addr string, credentialFile string, allowPrivateStack bool) (*Client, error)
func (c *Client) LinkSigningKey(ctx context.Context) (ed25519.PrivateKey, error)
func SigningKeyFingerprint(pub ed25519.PublicKey) string
```

Only this package handles OpenBao's credentials and wire requests. The application receives a signing key and passes it in memory to the later link code. The public fingerprint is lowercase hexadecimal SHA-256 over the 32-byte raw Ed25519 public key. It is distinct from the proxy certificate fingerprint. There is no signing key in the application database or a local fallback file.

Read a bounded, protected regular file as JavaScript Object Notation (JSON). Enforce exactly one nonempty string `role_id` and one `secret_id`. Reject duplicate, unknown or missing fields and trailing JSON values. Inspect the opened descriptor. Require the file to be owned by the process user and accessible only to that owner, with mode `0400` or `0600`. The Stack supplies numeric user and group 10001 and mode `0400`. Use one bounded HTTP client for login and key operations. Refuse redirects, disable proxy discovery from the environment and verify TLS for nonlocal endpoints. Plain HTTP is allowed for literal loopback and exact service `openbao` on port 8200 with private-stack opt-in. Validate addresses before sending credentials.

Send the two credential fields to `POST /v1/auth/approle/login`. Use `auth.client_token` in `X-Vault-Token` and honor the positive `auth.lease_duration`. The Stack job provisions a nonrenewable 15-minute batch token. Serialize refresh and use a fresh login when the cached token has at most one minute left. Expiration starts conservatively from just before the successful login request. A rejected login or expired lease that cannot be replaced fails the operation. Do not log the raw response or credential contents.

Read `GET /v1/lgtm/data/server/link/signing-key`. The field `data.data.private_seed_b64` is a canonical standard base64 encoding of exactly 32 seed bytes. On a missing entry, generate a candidate with `crypto/rand` and send `POST` to the same path with `{"options":{"cas":0},"data":{"private_seed_b64":"..."}}`. Here `cas` is OpenBao's compare-and-set option. An absent-entry 404 without version metadata may begin creation. A deleted or destroyed entry with version metadata, a malformed success, an authorization error or a sealed response fails startup. Never overwrite an existing entry.

On acknowledged creation or a compare-and-set conflict, read and use the persisted seed. Confirm the pinned vault's HTTP 400 conflict response in the fixture and classify only that validation response as a conflict. A timed-out create with an uncertain outcome permits one fresh read while the startup context remains live. Accept a confirmed stored seed, and otherwise fail without posting a second candidate. A transport or gateway failure during an initial read cannot authorize creation. Use fixed error categories for credential, login, read, create and decode failures. Bound each request to five seconds inside the application's startup deadline and cap response bodies at 64 KiB. Clear temporary seed buffers where practical. Sprint 3 receives the private key through application composition, and uses its own certificate fingerprint settings under issue 1633.

The internal `vaulttest` package provides `Start(t testing.TB, seed [32]byte) (address string, credentialFile string)`. It writes a fixture-owned credential file and serves login plus the read path with the supplied persisted seed on loopback. It registers cleanup. Item 9 consumes it for in-process transport tests. Keep scriptable failure, concurrency and time controls private to `vault_test.go`.

Write these tests first in `vault_test.go` using an HTTP wire fixture. The actual OpenBao policy and storage are covered by C13 and Stack's sprint 1 checks.

- `TestLinkSigningKeyPersistence` checks exact login metadata and read and create paths, including the seed encoding and `options.cas=0`. Assert that creation and a fresh client see the same persisted public key. Assert the creation code performs a follow-up GET and the second client sends no create. Check the stable fingerprint format.
- `TestLinkSigningKeyCreateRace` holds two clients at the absent read and releases both. Have one create succeed and return the version conflict to the other. Assert that both return the stored winner even though their attempted seeds differ. Repeat with a lost create response and require a single follow-up read with no second create.
- `TestOpenBaoLeaseAndFailures` advances a fixture clock across the refresh boundary. Assert login occurs before another secret request would use the old token. Exercise malformed credentials and seed, expired lease, forbidden login and read, versioned deletion, transport refusal, sealed response and a redirect. Assert errors contain no values and that read errors send no create.

This item is risky. Sending the create without compare-and-set or returning the locally generated candidate after a conflict must fail `TestLinkSigningKeyCreateRace`. The three named tests and the real-vault C13 observation gate acceptance.

**Item 9. The runnable application and the in-process test entry point**

Create `src/server/internal/app/start.go`, `src/server/internal/app/start_test.go`, `src/server/cmd/lgtm-server/main.go`, `src/server/servertest/server.go` and `src/server/servertest/server_test.go`. This item needs items 1, 3, 4, 5, 6, 7 and 8, and the Store's follower, pruner and test fixtures. It has no parallel item within this plan. It implements D1, D2, D3, D4, D7, D9, D10, D11, R7, R8, R9, R10, C2, C3, C4, C12 and the server clause of C13. It gives C6 through C11 a later test entry point.

```go
// Package app, under src/server/internal.
type Server struct { /* private listener and lifecycle */ }
func Start(ctx context.Context, cfg config.Config, log *slog.Logger) (*Server, error)
func (s *Server) Addr() net.Addr
func (s *Server) SigningPublicKey() ed25519.PublicKey
func (s *Server) Wait() error
func (s *Server) Close() error
```

The `lgtm-server` command with no arguments reads the settings and serves until interrupted. Use a signal context for orderly shutdown. Reserve `lgtm-server join-token` for the sprint 3 owners. First open the Store with `ConnectionPolicy{AllowPrivateStackPostgres: cfg.AllowPrivateStack}`. Apply `s.Migrate(ctx, auth.Migrations())` and reconcile auth with the configured password. Pass nil for a missing, empty or invalid configured password. With the password absent or invalid, report `LGTM_WEB_PASSWORD` and exit nonzero after a committed purge. If the purge cannot commit, report the failure separately, without opening a port. When neither PostgreSQL nor the settings needed to reach it are available, the operator must repeat the removal while PostgreSQL is available before restoring the old password. The server runs as a single instance. Deployment stops the old process before starting one with a different password.

Use a two-minute context for startup prerequisites, including the initial database connection, alongside the Store and vault per-operation limits. With a password present, construct the origin policy and require the OpenBao key next. Start `store.StartFollower` and `Store.RunPruner` with a process-lifetime context. Register the HTTP handler with `Options.Clients` and `Options.Link` unset in sprint 2. Validate that a standalone listener binds a literal loopback address. The explicit private-stack option allows the private Compose bind. Only then call `net.Listen`. Enable HTTP/1.1 and unencrypted HTTP/2 using `http.Server.Protocols` on the private or loopback listener. Set a five-second header timeout and an idle connection timeout of 60 seconds. Avoid a global read or write deadline covering a long-running RPC. The proxy owns internet-facing TLS. Once the listener opens, log one JSON event per startup with `msg: link_signing_key_loaded`, `source: openbao` and the public `fingerprint`, then serve. Startup failures return fixed diagnostics without environment values or raw transport errors. The command exits nonzero on fatal startup or unexpected serving failure. Clean up acquired resources on failure, and close the server and cancel background work before closing the Store on normal shutdown. On shutdown, allow five seconds for the HTTP server to drain, then close remaining connections and join the background work.

The public test package has exactly this exported surface. Issue 1634's third-package tests import `github.com/qnlbnsl/lgtm-studio/src/server/servertest`.

```go
type Options struct {
    Password string
}

type Server struct {
    URL              string
    HTTPClient       *http.Client
    SigningPublicKey ed25519.PublicKey
}

func Start(t testing.TB, options Options) *Server
func (s *Server) Stop(t testing.TB)
func (s *Server) Restart(t testing.TB)
```

`Start` provisions a real disposable PostgreSQL database using `storetest.NewDatabase(t, "server_test")`. It starts the same application as the command using an ephemeral loopback port. Its `HTTPClient` has a fresh cookie jar and uses unencrypted HTTP/2. Return the actual bound URL and a copy of the public key. Use a randomly generated fixture seed held by the internal OpenBao wire fixture across successive starts of this server fixture. Configure the production vault client to read it over loopback. `Start` and `Restart` fail the test on errors and register cleanup through `testing.TB`. `Stop` closes the running server and is idempotent. `Restart` stops a running instance if needed and starts at the same address, with the same database, cookie jar and seed. Close the old transport's idle connections before restart. Retain the fixture database until test cleanup, and close server and vault resources before dropping it. Do not store package-global credentials. The later machine work plugs its handlers and migrations into the same application so the link tests need no internal imports.

Write these tests first in `start_test.go` and `servertest/server_test.go`.

- `TestStartRevokesBeforeListen` uses an isolated PostgreSQL and a reserved test address released for the app to bind. Create a session through the RPC, stop the app and hold a database gate that delays password reconciliation. Start with a changed password. Before releasing the gate, verify a connection cannot be accepted. Release it and require ready plus refusal of the old cookie. Run a separate removal case with PostgreSQL reachable and the vault unavailable. Require the app to report the missing setting, leave the port closed and durably purge the session before a start with the original password. Repeat with an invalid, overlong configured password and check the same purge. Repeat with a trigger aborting the purge and require a fixed failure message and no listener.
- `TestStartWithUnavailableVault` uses migrated real PostgreSQL with a refused or malformed OpenBao wire fixture. Require startup failure, no listener, no success log and no secrets in diagnostics. After the fixture becomes readable, require startup, a 204 health response and one parseable key event with the expected public fingerprint.
- `TestStartAndRestartServer` runs as `servertest_test`, imports the public package and signs in through the generated client. Stop the server, observe the listener is closed, restart it and assert the same URL, same public key and a signed-in GetServer through the existing jar. Check the same HTTP/2 listener answers a generated gRPC GetServer client. Start another fixture and prove there is no database or cookie state shared between fixtures. An empty-password fixture must fail through the normal startup path, covered by the app test.

This item is risky. Opening the port before reconciliation must fail `TestStartRevokesBeforeListen`. Starting with a transient key when OpenBao fails must fail `TestStartWithUnavailableVault`. Recreating the test database or key on restart must fail `TestStartAndRestartServer`. The named tests and C2 and C13 gate acceptance.

**Design artifact.** The foundation design's compose and startup checks specify the existing website entry point. There is no changed screen hierarchy.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Startup serves before validating existing cookies or securing the link identity. | High | The app defers listening until validation completes. The tests block a prerequisite and probe before allowing startup to proceed. |
| A vault error or a racing first startup supplies a different key. | High | Use a create-only write and require a persisted read before logging readiness. Exercise the error and the competing first request. |
| An uncommitted password removal leaves a prior session in storage. | High | Fail closed and report the purge failure. Verify a successful removal is durable before restoring the old password. |

**Verify.** Run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...`. Run `deploy/test/checks.sh --only C2,C3,C4,C13` on the disposable Compose stack and require four passing Test Anything Protocol (TAP) lines with exit status zero. C13 verifies the real vault. No device check is needed here.

**Rollout detail.** In steps 1 and 2 use the merged sprint 1 fixture and the merged root module requirements. In step 3 run `deploy/up.sh` and the scoped C2, C3, C4 and C13 command after Stack's proxy is available. The full C5 through C11 machine suite waits for sprint 3. C12 uses the existing feed mutation hook and a reachable disposable PostgreSQL instance.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The universe is the original sign-in brief's eight items and four planning points, issues 1632 to 1640 as they affect the handoff, and these parts of the [foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md). It includes D1, D2, D3, D4, D5, D7, D9, D10, D11, R1, R2, R3, R4, R7, R8, R9, R10, R11, C2, C3 and C4, the database fixture dependency of C12, and the server half of C13. It includes the design's Sign-in and web sessions, Serving the contract, Secrets live in OpenBao and server-facing compose settings. It includes the names consumed by the [wave 1 check driver](../../../deploy/test/README.md).

| Source item | Workstream | Status |
|---|---|---|
| D1, D2 and R2. | WS-2, WS-3 | Generated web routes and a Go application in items 6, 7 and 9. |
| D3, D4, D5, R1, R3, R4 and R7. | WS-1, WS-3 | Items 1, 3, 4 and 9 use the store's migration, current-state and ordered feed interfaces. |
| D7, D9 and D10. | WS-3 | Items 8 and 9 supply the signing key the later proof uses. |
| D11, R8 and R9. | WS-1, WS-2, WS-3 | Items 2 and 4 through 9 cover the protected cookie, safe errors and private credential handling. |
| R10 and C12's database prerequisite. | WS-1, WS-2, WS-3 | Real PostgreSQL for data tests and the public server entry point for in-process link tests. The feed mutation stays with Store and Stack. |
| R11 and the brief's legacy credit point. | WS-1, WS-2 | Items 2 through 5 retain design behavior, with original Go code and standard library calls. No external source travels into this plan's build. |
| Sign-in and web sessions, and C2 to C4. | WS-1, WS-2, WS-3 | Items 1 through 7 and 9 give the settings, cookie behavior, startup refusal, restart persistence and password revocation. |
| Serving the contract. | WS-2, WS-3 | Items 6, 7 and 9 serve the generated handlers and own the auth errors. |
| OpenBao, server-facing compose settings and the server clause of C13. | WS-3 | Items 1, 8 and 9 follow the credential, key, listener, readiness and signing-log handoffs. |
| Original brief items 1 to 8. | WS-1, WS-2, WS-3 | Brief item 1 maps to items 1 and 9. Brief items 2 and 3 map to item 6. Brief item 4 maps to item 2. Brief item 5 maps to items 3 and 4. Brief items 6, 7 and 8 map to items 5, 7 and 8 respectively. |
| Four planning points of the brief. | WS-1, WS-2, WS-3 | Item 7 sets receipt policy. Items 2 to 5 carry reference behavior in original code. Item 8 refuses unavailable vault startup. Items 5 and 6 assign the extra cookie and guard files. |
| Issues 1632, 1633 and 1634. | WS-1, WS-2, WS-3 | Item 3 keeps state in PostgreSQL. Item 9 accepts private unencrypted HTTP/2 behind the proxy and exports the in-process fixture. |
| Issues 1635 and 1640. | WS-1, WS-2, WS-3 | The sizing fold follows issue 1635. Real test verification requires the network access identified in issue 1640. |
| Issues 1636, 1637, 1638 and 1639. | WS-2, WS-3 | The later Stack, Client and Server plans carry join syntax, address semantics, machine placement facts and rotation policy. Items 6 and 9 leave their integration entry points. |
| Wave 1 check names. | WS-1, WS-2, WS-3 | Items 1, 6, 7, 8 and 9 preserve server-observed names. Sprint 1 Stack owns the C5 database helper correction described in the planning handoff. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, What this project is and Where things are. | Write only new Go in `src/server/`. Use the generated contract and treat the old engine as a reference. |
| `AGENTS.md`, habit 1, credits copied code. | Use standard and pinned package interfaces in original code. No copied file needs a license or notice entry. |
| `AGENTS.md`, habit 2, records issues. | Record plan and check-driver dependencies in the assigned planning receipt so the orchestrator can track them in the shared issue record. |
| `AGENTS.md`, habit 3, sends finished work to the base branch through a pull request. | The build lead integrates after exact-commit tests. The orchestrator controls the base-branch pull request. |
| `AGENTS.md`, Writing style for documents. | Use repository-relative source anchors and keep personal paths, addresses and quotes out of the artifacts. Scan the authored files before handoff. |
| The Go rewrite epic, The teams, and what each may edit. | Request root dependencies and deployment changes from their owners. `server/backend` controls the implementation paths. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on | Can run alongside | Design statements |
|---|---|---|---|---|---|---|---|---|
| 1. Settings. | backend | server/backend | medium | It validates one configuration and its private transport boundary. | Yes. | Sprint 1 Store and Stack. | 2, 3, 5, 8. | D3, D4, D10, D11, R9, C2. |
| 2. Password. | backend | server/backend | medium | It isolates the password comparison and binding format in one package and gives them independent tests. | Yes. | Root scrypt dependency handoff. | 1, 5, 8. | R9, C3, C4. |
| 3. Schema and reconciliation. | migrations | server/backend | medium | It applies one additive migration and tests the startup purge independently of runtime session writes. | Yes. | 2 and the Store migration and write surfaces. | 1, 5, 8. | D3, D4, D5, R1, R3, R4, R7, R9, R10, C2, C4. |
| 4. Sessions. | backend | server/backend | medium | It tests deterministic runtime mutation order separately from schema and password reconciliation. | Yes. | 2, 3 and Store read and write handles. | 5, 8. | D3, D4, D5, R3, R4, R7, R9, R10, C3, C4. |
| 5. Browser origins and cookies. | backend | server/backend | medium | It computes web policy with independent origin and cookie fixtures. | Yes. | Root public suffix dependency handoff. | 1, 2, 3, 4, 8. | D11, R8, R9, C3. |
| 6. Router and guard. | backend | server/backend | medium | It uses generated routing and exercises a single admission boundary across transport modes. | Yes. | 4, 5 and the sprint 0 generated handlers. | 8. | D1, D11, R2, R8, R9, C3. |
| 7. Session methods. | backend | server/backend | medium | It joins the existing password, session and origin components through three generated methods. | Yes. | 2, 4, 5, 6. | 8. | D1, D11, R2, R8, R9, C3, C4. |
| 8. Vault reader. | backend | server/backend | medium | It limits the wire integration to a credential, an expiring lease and one persisted key, with race and failure tests. | Yes. | Sprint 1 Stack credential and policy. | 1 through 7 once ready. | D7, D9, D10, R9, C13. |
| 9. Application and fixture. | backend | server/backend | medium | It wires the independently tested components into a listener and reuses that startup for the public test package. | Yes. | 1, 3, 4, 5, 6, 7, 8 and Store lifecycle and fixture exports. | None within this plan. | D1, D2, D3, D4, D7, D9, D10, D11, R7, R8, R9, R10, C2, C3, C4, C12, C13. |

Load for nine test writers, nine builders, the first review round and nine build review or mutation seats is estimated at 13 OpenAI, 16 Anthropic and 4 xAI launches. That uses a nine and nine split for test writing and building, a one, four and one split for the review round, and three build reviews per vendor. Weekly remaining allowances were unavailable to this planner. The orchestrator compares the estimate against the meters before launch and schedules any dependent work that exceeds an allowance after its reset.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (blocker, major, minor) | Outcome |
|---|---|---|
| First orchestrated round. | Pending. | The bounded planner hands off at the start of attack. The orchestrator records findings and dispositions here under the build loop. |

</details>
