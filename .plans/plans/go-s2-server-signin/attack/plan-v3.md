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

The bundled website and server share one loopback address in wave 1. Deployed checks cover sign-in, restart, password revocation and signing-key persistence. The separate-site HTTPS browser checks move to the later proxy work.

## Key decisions

The single owner signs in with the password supplied to the server. The browser holds a cookie that its page scripts cannot read. PostgreSQL holds a verifier and the metadata that binds the session to the configured password. An ordinary restart keeps the session, and changing the password revokes the existing sessions before the server accepts requests.

The bundled website uses a strict cookie. Listed websites on another site use a separate secure cookie. Their sign-ins and sign-outs are independent in the same browser profile. A cross-site deployment requires a browser that permits unpartitioned cookies and is verified in the later proxy work. (logic2-r1 5, critic-r1 7)

The server reconciles sessions and checks the OpenBao signing key before listening. After the first start, PostgreSQL records the public fingerprint. The server refuses readiness if OpenBao loses the key or supplies a different one. An ordinary recreation keeps its identity. (logic2-r1 4)

The plan provisionally refuses a retry key on sign-in. After a lost reply a second sign-in can consume another slot and evict an existing session. The owner needs to settle this exception to the general retry rule before the tests are frozen. (logic2-r1 6)

The full choice log is in [decisions.md](decisions.md).

## Workstreams

### The owner's browser session survives an ordinary restart

Sign-in keeps the eight newest sessions. A committed password transition revokes earlier sessions, including if the password is later restored. The owner still needs to settle revocation intent during a database outage and expiry after a backward database clock change. (threat-r1 4, logic2-r1 2, logic2-r1 3)

### The existing website can sign in to the Go server

The website receives the same product name, contract revision and sign-in results it already reads. A browser from another origin must pass an exact allowlist check or use a bundled alias of the plaintext public loopback address at its port. (critic-r1 2, logic-r2 3) An unauthenticated call to a protected procedure receives a sign-in error, including when the service behind the procedure has yet to ship.

### The server verifies its prerequisites before accepting requests

Credentials and settings come from the environment and the stack's protected files. The server migrates PostgreSQL, reconciles sessions and reads the signing key before opening its listener. A Go test can start and restart the same server in its own process for the later machine-link checks.

## Rollout order

1. Merge the generated contract, the database layer and the stack's OpenBao setup. Give the build tests a disposable PostgreSQL with database creation permission.
2. Land the session schema and the server's request policies, then exercise startup and shutdown through the same entry point used by the server command. The first startup writes the session and public identity schemas. A binary that lacks this migration manifest cannot be used against that database without a compatible manifest.
3. Once all Server and Stack Web items are in the sprint branch, the orchestrator runs the joint gate `deploy/test/checks.sh --only C1,C2,C3,C4,C13` on the disposable stack and requires five passing lines and exit 0. Browser sign-in evidence is in PB1. Every work item gates on its own named tests.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Build loop.** Each of the nine Server items starts with its named tests and real fixtures. Freeze them after the named red assertion fails against stubs. The lead verifies the exact commit and named mutation, then reruns the merged tests. [Build loop, phase 4](../../../docs/specs/2026-11-05-build-loop-design.md#4-build).

**Modules and first consumers**

**UNREVIEWED MECHANISM** (critic-r1 3, architect-r1 9, logic2-r1 10, threat-r1 7)

| Module | Version | First item in this plan and package |
|---|---|---|
| `github.com/jackc/pgx/v5` | `v5.13.0` | Item 1 uses `pgxpool`. |
| `golang.org/x/crypto` | `v0.59.0` | Item 2 uses `scrypt`. |
| `golang.org/x/net` | `v0.63.0` | Item 5 uses `publicsuffix`. |
| `connectrpc.com/connect` | `v1.21.1` | Item 6 uses the Connect handlers and error writer. |
| `google.golang.org/protobuf` | `v1.36.12` | Item 6 uses generated descriptors from sprint 0. |

The builders of items 2 and 5 add their listed direct module in the same change as the first import. Go 1.26's module selection also allows the versions below. Record only Go's required entries and sums. The builders run `go mod tidy -diff`, `go mod verify` and the Store suite against disposable PostgreSQL after their dependency change. **UNREVIEWED MECHANISM** (architect-r2 5, threat-r2 4). The graph and verification commands distinguish required sums from extra direct requirements.

| Module graph dependency | Selected version | Trigger |
|---|---|---|
| `golang.org/x/net` | `v0.62.0` if item 2 lands first, then `v0.63.0` | Crypto metadata, then the direct public-suffix import. |
| `golang.org/x/sys` | `v0.50.0` | Crypto and network metadata. |
| `golang.org/x/term` | `v0.48.0` | Crypto and network metadata. No explicit root requirement without an import. |
| `golang.org/x/text` | `v0.44.0` | Crypto and network metadata, and the existing PostgreSQL driver's imports. |
| `golang.org/x/sync` | `v0.25.0` | Text metadata and the existing Store pool's dependency graph. |

The integration reviewer traces new sums to the tagged graph in [scouting.md](scouting.md).

**WS-1. The owner's browser session survives an ordinary restart**

PostgreSQL supplies the order and current session record. The server authenticates the current binding at startup and during authorization. It reuses a successful verification only while the complete row matches. (logic-r1 1)

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | The server accepts the exact configured password and refuses a different or oversized input. | `TestPasswordMatchesExactBytes` and `TestPasswordBindingVector`. |
| 2 | A session survives an ordinary restart. A committed password transition leaves zero old session rows. An overlapping process cannot serve, and a wrong password binding is refused. | `TestSessionMigration`, `TestReconcilePassword`, `TestReconcileRollsBack`, `TestCheckRejectsForeignBinding`, `TestStartRefusesSecondInstance`, `TestStartRevokesBeforeListen`. |
| 3 | A new cookie contains 32 random bytes. Issuance, expiry and logout preserve the eight-session cap and affect the intended records. | `TestSessionLifecycle`, `TestSessionIssueSerializes`, `TestSessionWriteRollsBack`, `TestSessionIssueTieAndBackwardClock` and `TestSessionFeedRefs`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 2 | Verify the password and construct the per-session binding. | Target `src/server/internal/auth/password.go`, `Password.Matches`. S2 gives the neighboring pattern. |
| 3 | Migrate and reconcile session rows before startup. | Target `src/server/internal/auth/reconcile.go`, `Reconcile`, and `auth/migrations/0001_web_sessions.sql`, `web_sessions`. S3 gives the store pattern. |
| 4 | Issue, read and revoke sessions through the store. | Target `src/server/internal/auth/sessions.go`, `Sessions.Issue`, `Sessions.CheckInRead`. S4 gives the neighboring pattern. |

**Item 2. Password verification and binding**

Create `src/server/internal/auth/password.go` and `src/server/internal/auth/password_test.go`. Its builder adds the listed crypto module with the first scrypt import. It can run alongside items 1, 5 and 8. It implements R9 and C3 and supplies the binding used for C4.

```go
const MaxPasswordBytes = 4096
type Password struct { /* private material */ }
func NewPassword(value string) (Password, error)
func (p Password) Matches(candidate string) bool
func deriveBindingKey(p Password, salt [16]byte) ([32]byte, error)
func bindingTag(key [32]byte, verifier [32]byte, createdAt time.Time) [32]byte
```

Use the exact 8-bit Unicode Transformation Format (UTF-8) input bytes. Reject a configured empty, non-UTF-8 or overlong password in `NewPassword` and reject an empty or overlong candidate in `Matches`. Compare fixed 32-byte Secure Hash Algorithm 256 (SHA-256) digests with `crypto/subtle.ConstantTimeCompare`. The length gate bounds work without comparing the candidate length to the configured length. Keep the comparison in a private `equalDigest(a, b [32]byte) bool` so the test can inspect the security primitive without unreliable wall-clock timing.

The private `bindPassword(p Password, salt [16]byte, verifier [32]byte, createdAt time.Time) ([32]byte, error)` derives 32 bytes with scrypt using `N=65536`, `r=8` and `p=1`. Use those bytes as a Hash-based Message Authentication Code (HMAC) SHA-256 key. Authenticate the concatenation of the ASCII (American Standard Code for Information Interchange) label `lgtm-web-session-v1`, one zero byte, the binary verifier, and the creation time as a signed big-endian 64-bit Unix microsecond count. Clear the derived key after use. `bindPassword` composes the two helpers. Item 2 writes them and item 4 derives the key before taking the database lock, then computes the tag with the chosen creation time. (critic-r1 10) The constructor and method retain no formatted credential output.

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

Define private helpers `lockSessions` and `recordSessionChange` in `reconcile.go` for items 3 and 4. Run auth changes within `Store.Write`. Acquire `LOCK TABLE web_sessions IN SHARE ROW EXCLUSIVE MODE` as the first domain statement. It serializes changes and allows ordinary readers to finish. Use the primary connection for authorization decisions. For each changed row call `tx.Record(store.ChangeRef{...})` with topic `web_sessions`, scope `""`, and the lowercase hexadecimal SHA-256 of `lgtm-web-session-feed-v1`, a zero byte and the binary verifier as key. The feed reference is derived independently of the token and the password fields. It cannot authorize a request. This table has no browser-visible record identifier. Use the store's own feed flush and surface its commit error.

A nil password means the operator removed `LGTM_WEB_PASSWORD`. Delete the rows and record their removal. With a configured password, read the live set under the lock, recompute its bindings and compare via item 2's `equalDigest`. If any live binding fails, delete the whole web session set together, including expired rows. Otherwise remove expired and future-dated rows using the predicate from item 4. Each deleted row emits a `store.Removed` reference, including expiry removals. (architect-r1 8, logic-r1 3) Return success only after commit. No startup check reads a local session file.

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
var ErrNoSession = store.NewDomainError("Sign in to continue.")

func NewSessions(s *store.Store, password Password) *Sessions
func TokenVerifier(token string) (Verifier, bool)
func (s *Sessions) Issue(ctx context.Context) (string, error)
func (s *Sessions) Check(ctx context.Context, verifier Verifier) error
func (s *Sessions) CheckInRead(ctx context.Context, tx *store.ReadTx, verifier Verifier) error
func (s *Sessions) Revoke(ctx context.Context, verifier Verifier) error
```

Only the verified password path in item 7 calls `Issue`. Generate 32 token bytes and a separate 16-byte salt with `crypto/rand` for each attempt. Expose the token only as canonical unpadded Uniform Resource Locator (URL)-safe base64. Before hashing a presented token, require exactly 43 characters and canonical re-encoding of 32 decoded bytes. The durable verifier is the digest of the binary token. Draw the entropy and derive the binding key before opening a write transaction. A random-source, derivation, insert or commit failure returns an empty token. An ambiguous commit may leave a session that expires or is evicted. There is no automatic second issue.

Under the item 3 table lock, sample PostgreSQL `clock_timestamp()` once. A row is live when its creation time is strictly greater than the sample minus 720 hours and at most one minute ahead of the sample. Use PostgreSQL's unqualified `clock_timestamp()` in reconciliation, issue, authorization and revoke. Production uses the built-in through its implicit catalog search path. Prune rows outside it before choosing the creation time. Use the sample, or one microsecond after the latest surviving row if necessary for order. If the increment would exceed the bound, abort with `ErrSessionClock` from `store.NewDomainError`. Auth writes the sentinel and item 7 maps it to `aborted` with “The server could not issue a session, retry sign-in.” **UNREVIEWED MECHANISM** (logic-r2 4). This clock limit is a domain refusal and carries no fabricated Store error code. Keep the eight newest rows by `created_at DESC, verifier DESC` and record changes in the same transaction. This keeps a stray future timestamp from moving the lifetime of subsequent sessions. Continuous clock rollback across an expiry boundary is outside the wave 1 operating envelope. (logic-r1 3, logic-r1 5)

`Check` calls `CheckInRead` in `Store.Read`. Read the full live row on each call. Cache successful full tuples by verifier, at most eight per password-bound `Sessions`. Drop a missing or invalid tuple and evict the least recently used entry when inserting at capacity. Under one cache mutex, look up the complete tuple and serialize missed derivations. Recheck the context after acquiring the mutex. **UNREVIEWED MECHANISM** (architect-r2 6). This caps running derivations and shares a success across a burst without tracking separate per-verifier jobs. Use item 2's `equalDigest` for constant-time binding comparisons. Auth writes `ErrNoSession` with Store's `NewDomainError`, and returns it directly for a missing, expired or mismatched row. Sprint 2 routing and sprint 3 Machines consume it. (logic-r1 1, logic2-r1 7, threat-r2 5)

`Revoke` reads and verifies the live tuple through `Store.Read` before starting the write. Under the table lock, re-read it against the new PostgreSQL sample. Delete only if it is still live and matches the verified full tuple. A missing or changed row returns `ErrNoSession`. **UNREVIEWED MECHANISM** (logic-r2 7, threat-r2 5). This leaves scrypt outside the write lock and closes the interval between verification and deletion. The Machines watch calls `CheckInRead` through `watch.Source.Authorize` using `api.SessionVerifier`. It maps `ErrNoSession` to `watch.ErrWatchUnauthorized` and delegates database failures to the Watch error path. Later eligible keyed web writes use principal `owner`.

Write these tests first in the `auth` package's `sessions_test.go`. Use a real test schema with a `clock_timestamp() RETURNS timestamptz` function reading one fixture row. Put that schema explicitly before `pg_catalog` in the test Store configuration. The production calls go through the normal methods. **UNREVIEWED MECHANISM** (architect-r2 2, logic-r2 4). A single fixture clock fixes the timestamp and exposes any substitution of the Go wall clock. Item 4 writes a private `Sessions.deriveKey` field with the signature of `deriveBindingKey`. The constructor sets the production helper for issuance and cache misses. Tests instrument it before starting workers.

- `TestSessionLifecycle` issues nine distinct tokens, verifies canonical decoding and checks the four-column persisted row against the independent binding. Assert the first token fails and the remaining eight pass. Advance the fixture clock to one microsecond before expiry, to expiry, and one microsecond after. Assert only the first timestamp is live. (critic-r1 8, logic2-r1 9, architect-r2 2) Revoke one live token and prove the other live tokens still pass. Assert repeated revoke yields `ErrNoSession`.
- `TestSessionIssueSerializes` starts with seven committed rows. A test-only insert trigger blocks writer A with a PostgreSQL advisory gate after it has reached the insert. Start writer B using a different backend. Observe B after its binding derivation returned and require it to be waiting on A's table lock in `pg_locks` and `pg_blocking_pids`. Treat B reaching the trigger or committing early as an assertion failure. Keep the gated observation below the Store lock timeout. (logic2-r1 9, critic-r1 10) Release A and wait for both successes. Assert that precisely the original oldest token is gone, the other six original tokens plus A and B still authorize, and the observed creation times put A before B. The test queries the lock mode and uses a separate observer. A timeout waiting for a fixture is separate from failure of the lock assertion.
- `TestSessionWriteRollsBack` starts with eight live rows, installs a trigger that fails eviction, and invokes `Issue`. Require an empty token on error, the original eight verifiers and the original feed head from another transaction.
- `TestCheckRejectsForeignBinding` inserts a live row bearing another password's valid tag. Assert `ErrNoSession` in both read paths and revoke. Warm the cache with a valid row, change its salt or tag and assert refusal. (logic-r1 1, threat-r1 3)
- `TestSessionIssueTieAndBackwardClock` uses the fixture clock alongside real Store writes. Seed equal and slightly future creation times, plus a row over a minute ahead. Require eviction in the declared tie order, removal of the outlier and survival of the newest issue. Check that the feed records those removals in the same order. (logic-r1 3, logic-r1 5)
- `TestSessionsUseDatabaseClock` sets fixture time far from the Go clock, asserts the issued row has that timestamp, then checks and revokes around its exact expiry. Seed a row exactly one minute ahead and require `ErrSessionClock`, no token and no state change on issue. (logic-r2 4)
- `TestSessionBindingCache` warms eight entries, issues a ninth and makes 100 parallel checks of the newest token. Count one derivation for that tuple and confirm the cache remains bounded. Repeat the checks on a cold instance.
- `TestRevokeVerifiesBeforeLock` holds a cold derivation at the hook while a separate sign-in finishes. Gate only the derivation for the revoking row's salt. Change that row on another connection before releasing the hook and require revoke to refuse it and preserve the row. Inspect the binding comparison for `equalDigest`, whose primitive item 2 freezes. (architect-r2 6, threat-r2 5, logic-r2 7)
- `TestSessionFeedRefs` issues and revokes one cookie. Read the two feed rows and check their topic, empty scope, derived opaque key and `saved` then `removed` changes. Assert the references and stored rows contain neither the raw token nor its encoded text. Assert published reference keys differ from the row verifier and contain no password or binding.

This item is risky. Removing the table lock must fail the lock observation in `TestSessionIssueSerializes`. Returning the token on a failed transaction must fail `TestSessionWriteRollsBack`. Weakening the expiry boundary must fail `TestSessionLifecycle`. Using the Go clock must fail `TestSessionsUseDatabaseClock`. Moving revocation's derivation under the lock must fail `TestRevokeVerifiesBeforeLock`. The named tests gate acceptance.

**Design artifact.** The foundation design's Sign-in and web sessions section defines the behavior. The existing screen supplies the browser entry point.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A failed or overlapping sign-in issues a cookie for the wrong durable state. | High | Return a token after commit. Serialize insert and eviction and freeze the two-backend lock test. |
| A password change leaves an old cookie active. | High | Hold exclusive server admission, reconcile before serving and check the running password against any current row. **UNREVIEWED MECHANISM** (critic-r1 6). The lock enforces the supported single-server deployment and the row check guards against stale or altered bindings. |
| PostgreSQL is unavailable during a password transition. | High | The old sessions cannot be changed until a purge commits. Stop the serving instance, complete a start with the new or removed value and confirm revocation before restoring the former password. (architect-r1 7, logic2-r1 2) |
| The database clock moves backward across an expiry boundary. | Medium | The supported operating envelope uses nondecreasing PostgreSQL wall time across expiry and restart. The clock decision is recorded for the owner. (logic2-r1 3) |

**Verify.** Run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/auth`. The orchestrator checks deployed C4 at the sprint gate.

**WS-2. The existing website can sign in to the Go server**

The browser boundary selects the cookie policy before any session can be written. The request router applies that boundary and the session check consistently across the normal generated services.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 4 | Same-site sign-in uses Strict. A listed cross-site browser gets None and Secure. An unlisted or unsafe origin receives a refusal before a session is written. | `TestOriginPolicies`, `TestCookieAttributes`, `TestReadsOneCanonicalCookie`, `TestBrowserOriginGate` and the same-profile sign-in and sign-out steps in PB2. |
| 5 | The right password returns Signed In and the wrong password returns Rejected. A credential is issued only in the cookie. | `TestSessionContract`. |
| 6 | The two public web calls remain reachable. Protected unary and streaming calls require a cookie across supported transports. An authenticated unsupported service returns `unimplemented`. | `TestWebRoutesRequireSession`, `TestProtectedStreamAndProtocolCodes` and `TestSessionRequestKeys`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 5 | Validate the web origin and derive cookie attributes. | Target `src/server/internal/auth/origin.go`, `NewOriginPolicy` and `src/server/internal/auth/cookie.go`, `ReadCookie`. S5 gives the neighboring pattern. |
| 6 | Route generated services and guard the web boundary. | Target `src/server/internal/api/server.go`, `NewHandler` and `src/server/internal/api/session_check.go`, `SessionVerifier`. S6 gives the neighboring pattern. |
| 7 | Implement the generated Session service. | Target `src/server/internal/api/session.go`, `NewSessionService`. S7 gives the descriptor and consumer. |

**Item 5. Browser origins and cookies**

Create `src/server/internal/auth/origin.go`, `src/server/internal/auth/origin_test.go`, `src/server/internal/auth/cookie.go` and `src/server/internal/auth/cookie_test.go`. Its builder adds the listed network module with the first public-suffix import. It can run alongside items 1, 2, 3, 4 and 8 once their prerequisites are met. It implements D11, R8, R9 and C3.

```go
const CookieName = "lgtm_session"
const SecureCookieName = "__Host-lgtm_session"
const CrossSiteCookieName = "__Host-lgtm_cross_site_session"

type OriginPolicy struct { /* private public origin and allowlist */ }
type CookieMode struct {
    Name     string
    Secure   bool
    SameSite http.SameSite
}

func NewOriginPolicy(publicURL string, allowed []string) (*OriginPolicy, error)
func (p *OriginPolicy) Check(r *http.Request) (CookieMode, error)
func ReadCookie(r *http.Request, mode CookieMode) (string, bool)
func SessionCookie(token string, mode CookieMode) *http.Cookie
func ClearSessionCookie(mode CookieMode) *http.Cookie
```

Canonicalize scheme and ASCII host case and remove default ports. Accept a configured origin with an optional root slash. Require a serialized origin with no slash in a request's `Origin` header. Reject empty or multiple header values, `null`, user information, paths beyond the configured root slash, queries, fragments, trailing dots, wildcards, malformed ports and non-ASCII host spellings. The administrator uses browser-serialized names. Allow HTTPS and plaintext on exact `localhost` or literal loopback addresses. Never resolve a name to decide whether it is loopback. Compare normalized origins as exact scheme, host and port tuples. The configured public origin is accepted without an allowlist entry. Accept a different origin only if listed. When the configured public origin is plaintext loopback, also admit `127.0.0.1`, `localhost` and `[::1]` as plaintext same-site aliases at its exact port. An HTTPS public origin admits no plaintext alias. (architect-r2 4, logic-r2 3, threat-r2 2) Reject a cross-site entry at startup unless both origins use HTTPS (Hypertext Transfer Protocol Secure). **UNREVIEWED MECHANISM** (architect-r1 3, critic-r1 2). The exact aliases keep the set independent of proxy headers.

Determine the site from scheme and the effective registrable domain using the public suffix list, including its private entries. Use the exact normalized host for literal addresses or a host without a registrable domain. Ports affect origin equality and do not affect site equality. Use Strict and `CookieName` on plaintext loopback. Use Strict, Secure and `SecureCookieName` for HTTPS same-site requests. Only an explicit, listed cross-site Origin selects None, Secure and `CrossSiteCookieName`. An absent Origin selects the public same-site mode, including for native clients. **UNREVIEWED MECHANISM** (logic2-r1 5, threat-r1 5, threat-r1 6). Auth item 5 writes the names and `ReadCookie` signature. Items 6 and 7 read the selected name for lookup, issue and deletion. Native loopback checks retain `lgtm_session`. Refuse a request reporting `Sec-Fetch-Site: cross-site` with no Origin. Do not consult `Host`, `Forwarded`, `X-Forwarded-Host` or `X-Forwarded-Proto` for admission or cookie mode.

Inspect the cookie header lines for exactly one occurrence of the selected name. Other mode names are ignored. Treat a malformed or duplicate occurrence as absent, including a duplicate on another `Cookie` line. The router checks canonical encoding through item 4 before using a returned value. Issue `Path=/`, `HttpOnly`, `Max-Age=2592000`, the selected `SameSite`, conditional `Secure`, no `Domain` and no `Partitioned` attribute. (critic-r1 7) Clear with the same attributes, an empty value, `Max-Age=0` on the wire, and an expiry at the Unix epoch. Use a negative Go `http.Cookie.MaxAge` to serialize that deletion. Emit one cookie for a successful mutation.

Write these tests first in `origin_test.go` and `cookie_test.go`.

- `TestOriginPolicies` covers same origin, a listed different port or subdomain on the same site, two tenants under a private suffix, listed unrelated HTTPS sites, an unlisted same-site origin, an unlisted cross-site origin, plaintext loopback and rejected plaintext non-loopback. At plaintext port 18787 accept the three aliases and refuse them unlisted at port 8787. With an HTTPS public origin, including HTTPS on loopback, refuse plaintext aliases at the listener's port and at every other tested port. (architect-r2 4, logic-r2 3, threat-r2 2) Check normalized default ports, invalid allowlist entries, duplicate and opaque Origin headers, absence with and without cross-site fetch metadata, and a forged forwarded scheme. Assert a cross-site cookie is never selected without its explicit admitted origin.
- `TestCookieAttributes` parses the emitted cookie strings. Assert the exact name, host-only behavior, path, age, HttpOnly, SameSite, Secure and expiry rules for all three modes. Verify deletion is serialized with `Max-Age=0` and check that a cookie jar deletes the matching host-only entry.
- `TestReadsOneCanonicalCookie` accepts a single well-formed fixture and rejects absent, malformed and duplicate values, including a malformed and valid occurrence on different header lines. Assert different mode names do not affect the result, even when a cross-site cookie is presented on an Origin-less native call.

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

On a protected web procedure, read and verify the cookie through `auth.Sessions.Check`. Store only the verified digest and the selected cookie mode in request context. The raw credential never becomes context identity. `GetServer` does its own optional verification in item 7. Apply `Cache-Control: no-store` to the three Session procedures, including errors. Support Connect and Google Remote Procedure Call (gRPC) handlers on the same listener. Use the pinned Connect `ErrorWriter` for protocol-aware middleware errors, including streaming. Refused browser requests without a recognized media type receive bare 403. The early 400, 403 and 405 transport rejections have empty bodies. Application errors and valid size overruns use the one-sentence messages below. (critic-r1 9)

| Condition | Connect code | Public sentence |
|---|---|---|
| Absent, malformed, expired or revoked session on a protected procedure. | `unauthenticated` | Sign in to continue. |
| Refused origin on an actual protocol request. | `permission_denied` | This website origin is not allowed. |
| Authorized or independent Link request for an unbuilt procedure. | `unimplemented` | This call is not implemented. |
| A request key is present on SignIn or authenticated SignOut. | `invalid_argument` | This call does not accept a request key. |
| A complete Session message exceeds the read bound. | `resource_exhausted` | Connect reports `message size <bytes> is larger than configured max 32768`. |
| Store commit outcome is unknown. | `unavailable` | The database did not confirm whether the write committed. |
| Retryable store error. | `aborted` | The database was busy, retry the request. |
| Session creation time cannot preserve the order within the allowed skew. | `aborted` | The server could not issue a session, retry sign-in. |
| Store or session state unavailable. | `unavailable` | Web session storage is unavailable. |
| Request canceled or deadline expired. | `canceled` or `deadline_exceeded` | The request was canceled. / The request timed out. |
| Unexpected application failure outside storage. | `internal` | The server could not complete the request. |

Use Store's `Error.Outcome()` before `Retryable()`. Store writes those identities and the web router and Session handler consume them. For an unknown outcome leave the cookie untouched and do not automatically repeat an unkeyed action. Pass `WithReadMaxBytes(32<<10)` to the generated Session handler. The pinned Connect transport writes the overrun sentence before handler dispatch and enforces the same bound after decompression. Read failures stay under the pinned transport's error handling. The Session wire test reads the code and sentence. **UNREVIEWED MECHANISM** (critic-r1 9). (logic2-r1 7)

Tests first in the two named test files. Use a real migrated database wherever a test reads or creates a session. Supply generated handler probes that record entry without implementing feature behavior.

- `TestWebRoutesRequireSession` checks the two public procedures, each protected service family, readiness, and attempts to reach a public method using an encoded or noncanonical path. Require no protected handler entry without a cookie, entry with a valid cookie, and `unimplemented` on an authenticated Map or Preview call. Verify the early Link placeholder has its independent policy at the exact two paths only.
- `TestBrowserOriginGate` sends allowed and denied preflights, a preflight with an unexpected header, and actual denied calls that would otherwise reach SignIn. Assert the expected status, headers, lack of dispatch and lack of cookie. Include a real browser-style Origin matching a nondefault loopback port. Assert that the allowed actual browser calls include the exact credentialed headers even on an unauthenticated response.
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

`SignIn` accepts only `SignInRequest.password` from the decoded body. Limit the generated Session handler's message reads to 32 KiB and the password itself to 4096 bytes. Never read the password from a query or header. Under the provisional retry decision, reject any present `store.RequestKeyHeader`, including empty or repeated fields, with `invalid_argument` and “This call does not accept a request key.” Without that header, a wrong password returns `SIGN_IN_RESULT_REJECTED` in a successful protocol reply with no new cookie. A valid password calls `Issue`. A committed issue returns `SIGN_IN_RESULT_SIGNED_IN` and the cookie selected by the origin guard. Return `resource_exhausted` for an over-limit protocol body. Neither the password nor any token appears in a response body, response metadata other than Set-Cookie, debug log or receipt.

`SignOut` first passes the guard. A present request key receives the same `invalid_argument` refusal as SignIn. It revokes the context verifier through `Sessions.Revoke` and clears the cookie only after commit. A session that vanished after the guard produces `unauthenticated`. A store failure leaves the cookie unchanged. A repeated SignOut after a committed revoke remains `unauthenticated` and performs no second write. GetServer ignores the header. The provisional SignIn and SignOut policy uses no receipts. (logic2-r1 6) Retryable keyed web mutations in later services authenticate and authorize first, then use `Store.WriteOnce` with principal `owner`, a generated procedure name and the service's semantic fingerprint. The sprint 3 join-token creation call gets the same credential-returning exception under the stated assumption.

Write these tests first in `session_test.go` over the normal router with a real migrated database, using two independent cookie jars where needed.

- `TestSessionContract` checks anonymous GetServer and the exact product, version, revision and sample flag. Assert wrong password returns `REJECTED` with no cookie. A password offered in a query or header with an empty body password is rejected. The right password returns `SIGNED_IN` and only a Set-Cookie credential. Follow GetServer before and after sign-out in both jars. After a successful sign-out, require the canonical revoked token to report signed out through GetServer and `unauthenticated` on protected routes. Assert the other jar stays signed in. Repeat the lookup with an expired canonical token. Send an uncompressed 32,769-byte Connect JSON body and the same payload compressed. Expect `resource_exhausted` with `message size 32769 is larger than configured max 32768`. Assert no cookie or new session. Assert the creation-clock refusal returns the specified `aborted` sentence. Force a confirmed rollback with a rejecting deferred session trigger. For an unknown outcome, hold a deferred trigger on a fixture advisory gate, observe the writer in COMMIT, and terminate its PostgreSQL backend. Use Store's `storetest.HoldAdvisoryGate` and `WaitForBlock` to synchronize. Require the mapped code and sentence, no Set-Cookie and no automatic second mutation. (logic2-r1 7, critic-r1 9)
- `TestSessionRequestKeys` sends present, empty and repeated request-key headers with a correct SignIn password. Assert `invalid_argument`, no cookie, no new session and no receipt. Preload seven sessions, discard the reply from a committed SignIn and then repeat without a key. Require a distinct token, exactly eight rows and one eviction from the original seven. Verify a second SignOut returns `unauthenticated` with no second feed row. The browser transport is checked without a request key in PB2. (logic2-r1 6, logic-r1 4) Check the missing-cookie priority for SignOut. Check that a keyed authenticated SignOut refuses the key and leaves the session alive. Inspect the receipts table for zero auth receipts after the sequence.

This item is risky. Moving cookie issue before commit must fail the failure case in `TestSessionContract`. Accepting a request key on SignIn must fail `TestSessionRequestKeys`. The two named tests gate acceptance.

**Design artifact.** The current browser uses the generated Session client through `src/frontend/src/server/api.ts`. This workstream keeps that entry point.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An unapproved origin reaches a credential-changing handler. | High | Gate by exact origin and exercise both preflight and actual requests. |
| An auth exception reaches a protected procedure or exposes a session. | High | Match exact procedures and test the response and receipt contents. |
| Another service on the same loopback host can replace the plaintext cookie across ports. | Low | PB1 confirms refusal and no access when a sibling port overwrites the cookie. (threat-r2 3) |

**Verify.** Run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/api`. `TestBrowserOriginGate` and `TestSessionContract` cover C3 at item level. PB1 supplies the sprint 2 browser evidence. Home and the website's sign-out wait for Machines in sprint 3. PB2 waits for the later HTTPS proxy. (Stack Web requests S12 and E2)

**WS-3. The server verifies its prerequisites before accepting requests**

The command and in-process tests use the same application and verify prerequisites before it listens.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 7 | The configured password is required, and migrations and password reconciliation finish before the first connection can be accepted. | `TestReadServerSettings`, `TestSettingsErrorsRedactValues`, `TestHealthCommand`, `TestStartupDiagnostics`, `TestStartRefusesSecondInstance`, `TestStartupMigrationBudget`, `TestStartRevokesBeforeListen`. |
| 8 | First startup records one signing key in OpenBao. Recreation loads the same key and reports its public fingerprint. A vault failure leaves the listener closed. | `TestLinkSigningKeyPersistence`, `TestLinkSigningKeyCreateRace`, `TestOpenBaoLeaseAndFailures`, `TestLinkIdentityRefusesReplacement`, `TestStartWithUnavailableVault`. |
| 9 | A Go test outside the server's internal packages can start and stop the application and restart it at the same address with the same database and key. | `TestStartAndRestartServer`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Parse the stack and local settings and assemble the database connection. | Target `src/server/internal/config/config.go`, `Load`. S1 gives the Stack and Store patterns. |
| 8 | Read the OpenBao credential and load the persisted key. | Target `src/server/internal/vault/vault.go`, `Open` and `LoadLinkIdentity`. S8 gives the Stack and wire patterns. |
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
| `LGTM_WEB_PASSWORD` | Required to serve. An unset or empty value is retained as absent for the reconciliation step. | `${LGTM_WEB_PASSWORD:-}`. The service can start without it so the executable can refuse startup for C2. |
| `LGTM_LISTEN_ADDR` | `127.0.0.1:8080`. | `:8080` on the private container network. Do not publish this port. |
| `LGTM_PUBLIC_URL` | If empty or unset, derive a plaintext origin from the standalone loopback listener. | `http://127.0.0.1:${LGTM_WEB_PORT:-8787}`. Stack derives it from the effective published port. |
| `LGTM_ALLOWED_ORIGINS` | Optional comma-separated browser origins. Trim outer whitespace and validate after reconciliation. | Absent in wave 1. |
| `LGTM_POSTGRES_HOST` | `127.0.0.1`. | `postgres`. |
| `LGTM_POSTGRES_PORT` | `5432`. | `5432`. |
| `LGTM_POSTGRES_DATABASE` | `lgtm_app`. | `lgtm_app`. |
| `LGTM_POSTGRES_USER` | `lgtm_app`. | `lgtm_app`. |
| `LGTM_POSTGRES_PASSWORD_FILE` | Required. | `/run/lgtm/postgres.password`, a protected file containing 64 lowercase hexadecimal characters and one trailing line feed. |
| `LGTM_ALLOW_PRIVATE_STACK` | `false`. | `true`, permitting plaintext to the two exact private service names for the isolated stack. |
| `LGTM_OPENBAO_CREDENTIAL_FILE` | Required to serve. | `/run/lgtm/openbao.json`. |
| `LGTM_OPENBAO_ADDR` | Required to serve. | `http://openbao:8200` on the private stack network. |

The Server item writes this settings contract. Stack writes the Compose environment and health check, which item 9 reads and implements. Stack item 4's `server_settings_are_named` checks these eleven emitted names. Compose omits `LGTM_ALLOWED_ORIGINS` and `LGTM_TLS_CERT_FILE`. The command warns on present retired `LGTM_POSTGRES_URL` or `LGTM_SERVER_LISTEN_ADDR`, naming only the variable. (Stack Web request S1, architect-r1 2, critic-r1 1, threat-r1 2)

Open the database credential without following a final-component symbolic link. Check regular type, owner or effective group, no other-user access and no group write on the opened descriptor. The parent path is a deployment trust boundary. Stack owns the trusted parent directories and mounts the file read-only as owner 70, group 10001 and mode `0440`. Standalone operators supply a parent path others cannot replace. Compose requires the exact hexadecimal format. Standalone accepts one nonempty password line. (architect-r1 5, threat-r2 7)

Parse the one explicit database endpoint through `pgxpool.ParseConfig` and use eight pooled connections. Use `sslmode=disable` on literal loopback, Unix sockets and the named private `postgres` service with opt-in. All other hosts use `verify-full`. Store validates the final destination and TLS (Transport Layer Security) policy. Reject driver settings that substitute an endpoint or weaken verification. `Load` fills `Config.Password` and the raw later-stage settings even on a database setting error. The application validates origins, listener and vault settings only after session reconciliation. The item 9 command reports the fixed diagnostics. (architect-r1 8, critic-r1 4)

Write tests first in `config_test.go`.

- `TestReadServerSettings` uses a lookup fixture and a protected temporary database password file. Check loopback defaults, the explicit Stack settings, opt-in behavior, a nondefault published web port passed as the public origin, literal password bytes and the missing password surviving configuration loading for reconciliation. Assert that removing opt-in stops plaintext to a service-name host. Check verified TLS for a non-loopback host, mode `0440` for a fixture owned by the process group, refusal of `0644` and symlinks, and the complete Compose environment from the table. Assert malformed origin configuration is retained for later validation.
- `TestSettingsErrorsRedactValues` uses invalid port, private-stack flag and file formats. Assert the applicable setting name appears and fixture secret values or full connection strings do not appear. Check that a database error still leaves the password fields available to the caller.

This item is risky. Treating arbitrary service names as eligible for plaintext must fail `TestReadServerSettings`. The two named tests gate acceptance.

**Item 8. OpenBao and the signing key**

Create `src/server/internal/vault/vault.go`, `src/server/internal/vault/vault_test.go`, `src/server/internal/vault/identity.go`, `src/server/internal/vault/identity_test.go`, `src/server/internal/vault/migrations/0001_link_identity.sql` and `src/server/internal/vault/vaulttest/openbao.go`. This item needs the Stack credential and policy contract and the Store migration, read and write interfaces. It can run alongside items 1 through 7 as their dependencies allow. It implements D7, D9, D10, R1, R4, R9, R10 and C13.

```go
type Client struct { /* private credentials and lease state */ }

func Open(addr string, credentialFile string, allowPrivateStack bool) (*Client, error)
type LinkIdentity struct {
    SigningKey ed25519.PrivateKey
    Fingerprint string
    VaultVersion int
}
func Migrations() store.Migrations
func LoadLinkIdentity(ctx context.Context, s *store.Store, c *Client, allowInitialize bool) (LinkIdentity, error)
func SigningKeyFingerprint(pub ed25519.PublicKey) string
```

The vault package handles credentials and returns an in-memory key. The fingerprint is lowercase hexadecimal SHA-256 over its raw 32-byte Ed25519 public key. PostgreSQL stores the public pin.

Read a bounded, protected regular file as JavaScript Object Notation (JSON). Enforce exactly one nonempty string `role_id` and one `secret_id`. Reject duplicate, unknown or missing fields and trailing JSON values. Inspect the opened descriptor. Require the file to be owned by the process user and accessible only to that owner, with mode `0400` or `0600`. The Stack supplies numeric user and group 10001 and mode `0400`. Use one bounded HTTP client for login and key operations. Refuse redirects, disable proxy discovery from the environment and verify TLS for nonlocal endpoints. Plain HTTP is allowed for literal loopback and exact service `openbao` on port 8200 with private-stack opt-in. Validate addresses before sending credentials.

Send the two credential fields to `POST /v1/auth/approle/login`. Use `auth.client_token` in `X-Vault-Token` and honor the positive `auth.lease_duration`. The Stack job provisions a nonrenewable 15-minute batch token. Serialize refresh and use a fresh login when the cached token has at most one minute left. Expiration starts conservatively from just before the successful login request. A rejected login or expired lease that cannot be replaced fails the operation. Do not log the raw response or credential contents.

Read `GET /v1/lgtm/data/server/link/signing-key`. The field `data.data.private_seed_b64` is a canonical standard base64 encoding of exactly 32 seed bytes. On a missing entry, generate a candidate with `crypto/rand` and send `POST` to the same path with `{"options":{"cas":0},"data":{"private_seed_b64":"..."}}`. Here `cas` is OpenBao's compare-and-set option. An absent-entry 404 without version metadata may begin creation only if the installation has no public pin. A deleted or destroyed entry with version metadata, a malformed success, an authorization error or a sealed response fails startup. Never overwrite an existing entry.

After the single create attempt, read the entry once if the startup context permits, including after HTTP 400, 403 or an uncertain write. Continue only with a canonical stored seed. An initial read error other than absence never permits a create. Stack writes the reviewed `lgtm-server` policy with read, create and update on `lgtm/data/server/link/signing-key`. Server uses read and initial compare-and-set creation only. **UNREVIEWED MECHANISM** (architect-r2 1, threat-r2 1). The public pin detects a replacement even with Stack's current permission. PB4 uses the real vault for ordinary creation and failure states, and the wire fixture controls competing replies. Bound each request to five seconds and each response to 64 KiB. Sanitize credential, login and key errors. (architect-r1 10, logic-r1 7, threat-r1 8)

**UNREVIEWED MECHANISM** (logic2-r1 4, logic-r1 8). The vault part's version-1 migration creates `server_link_identity(singleton boolean PRIMARY KEY CHECK (singleton), fingerprint text NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$'))`. Under the application’s exclusive startup lock, read the pin before OpenBao. A missing pin allows initial creation or adoption of the canonical seed already stored only when `allowInitialize` is true. With false, an absent pin fails before contacting OpenBao. Write the first pin through `Store.Write`, recording `store.Saved` through `Tx.Record(store.ChangeRef{...})` with topic `server_link_identity`, empty scope and key `link`. Return the identity only after commit. An existing pin requires a matching OpenBao read. Absence or mismatch fails startup before a write or a listen. No private key or AppRole credential enters PostgreSQL. A running server keeps its fixed key. Rotation and restoring an earlier database state are outside wave 1. (threat-r2 6)

The Server vault item owns this table, manifest, `LinkIdentity` and `LoadLinkIdentity`. App item 9 registers the manifest, consumes the result and logs OpenBao's positive `data.metadata.version` as integer `vault_version`. Sprint 3 Link and the token command consume the same validated identity. An established pin makes loss or replacement observable without storing a secret or relying on an earlier log.

Vault item 8 writes `vaulttest.Start(t testing.TB, seed [32]byte) (address string, credentialFile string)` for item 9. It supplies credentials, loopback login, a persisted seed and cleanup. Vault wire tests own their private failure, race and time controls.

Write these tests first in `vault_test.go` using an HTTP wire fixture. Deployed vault evidence comes from the sprint joint gate and PB4.

- `TestLinkSigningKeyPersistence` checks exact login metadata and read and create paths, including the seed encoding and `options.cas=0`. Assert that creation and a fresh client see the same persisted public key. Assert the creation code performs a follow-up GET and the second client sends no create. Check the stable fingerprint format.
- `TestLinkSigningKeyCreateRace` holds two clients at the absent read and releases both. Have one create succeed and return the version conflict to the other. Assert that both return the stored winner even though their attempted seeds differ. Repeat with a conflict returned as HTTP 403 and a lost response. In both cases require one follow-up read and zero additional creates. Assert a write denial followed by an absent read exits without a key.
- `TestOpenBaoLeaseAndFailures` advances a fixture clock across the refresh boundary. Assert login occurs before another secret request would use the old token. Exercise malformed credentials and seed, expired lease, forbidden login and read, versioned deletion, transport refusal, sealed response and a redirect. Assert errors contain no values and that read errors send no create.

- `TestLinkIdentityRefusesReplacement` runs `identity_test.go` with a migrated real PostgreSQL database. Establish a pin and assert an unchanged restart reads the same identity and produces one feed reference total. Replace the fixture seed with a different valid seed, then exercise a separate absent case. Require either startup load to fail, with the pin unchanged and no creation request. An interrupted first pin commit must allow a restart to pin the same already stored seed.

This item is risky. Removing compare-and-set or using the local losing candidate must fail `TestLinkSigningKeyCreateRace`. Skipping the pin comparison must fail `TestLinkIdentityRefusesReplacement`. The named tests gate item 8, including the wire fixture for the server half of C13.

**Item 9. The runnable application and the in-process test entry point**

Create `src/server/internal/app/start.go`, `src/server/internal/app/start_test.go`, `src/server/internal/store/instance.go`, `src/server/internal/store/instance_test.go`, `src/server/cmd/lgtm-server/main.go`, `src/server/cmd/lgtm-server/main_test.go`, `src/server/servertest/server.go` and `src/server/servertest/server_test.go`. This item needs items 1, 3, 4, 5, 6, 7 and 8, and the Store's follower, pruner and test fixtures. It has no parallel item within this plan. It implements D1, D2, D3, D4, D7, D9, D10, D11, R7, R8, R9, R10, C2, C3, C4, C12 and the server clause of C13. It gives C6 through C11 a later test entry point.

```go
// Package app, under src/server/internal.
type Server struct { /* private listener and lifecycle */ }
func Start(ctx context.Context, cfg config.Config, log *slog.Logger) (*Server, error)
func (s *Server) Addr() net.Addr
func (s *Server) SigningPublicKey() ed25519.PublicKey
func (s *Server) Wait() error
func (s *Server) Close() error
```

Item 9's `app.Start` calls a private `start(ctx context.Context, cfg config.Config, log *slog.Logger, budgets phaseBudgets) (*Server, error)`. Item 9 writes `type phaseBudgets struct { admission, migration, prerequisites time.Duration }` with production values ten seconds, five minutes and two minutes. Its same-package `start_test.go` passes shorter values to exercise the real phase deadlines. Command and fixture callers use `Start`. (architect-r1 8, critic-r1 4, architect-r2 2)

1. Validate the password and database settings. Preserve both diagnoses if both are unusable. Defer handling of unrelated settings until after session reconciliation.
2. Open Store using its `ConnectionPolicy{AllowPrivateStackPostgres: cfg.AllowPrivateStack}`. Acquire the serving lock within the admission budget. Start its monitor before calling `Migrate`, using a lifetime context that cancels startup and serving upon loss. (architect-r1 4, logic-r1 1, threat-r1 1, architect-r2 3)
3. Give `Store.Migrate` the independent migration context with `auth.Migrations()` and `vault.Migrations()`. Run remaining checks under the prerequisite context. Stack sets an eight-minute health grace period and a 540-second launcher wait. (logic2-r1 8)
4. On every start, reconcile with the configured password or nil for a missing, empty or invalid value. An unusable password prints the revocation outcome followed by the password-setting line, then exits before the vault or listener.
5. Validate the origin policy, listener and OpenBao settings. Load and verify the key and public pin through `vault.LoadLinkIdentity` with `allowInitialize=true`.
6. Call `store.StartFollower(ctx, s, report)` and run `s.RunPruner(ctx, report)` under the lifetime context, then require a successful lock probe immediately before opening the listener. Clients and Link are unset this sprint. Standalone binds only literal loopback. Compose exposes cleartext HTTP/2 (Hypertext Transfer Protocol, version 2) on its opted-in private network. Set five-second header and 60-second idle deadlines. Leave long-running calls independent of a global read or write deadline.

Item 9 writes `func (s *Store) AcquireServerInstance(ctx context.Context) (*InstanceLock, error)` in `store/instance.go`. Reuse `serverInstanceLockID` from `store/locks.go`. Use a dedicated unpooled connection opened from Store's validated pool configuration and final dial policy. Run `pg_advisory_lock` with the admission deadline. An acquisition deadline while waiting returns the fixed busy diagnostic. **UNREVIEWED MECHANISM** (architect-r2 3, logic-r2 1, logic-r2 6). PostgreSQL queues the wait directly and avoids a client retry loop.

Item 9 writes `InstanceLock.Check(ctx context.Context) error` and `InstanceLock.Close(ctx context.Context) error`, which App calls. On the same connection, serialize five-second `SELECT 1` probes immediately after acquisition, one second after each completion, and before listening. A database or connection error or an exhausted probe budget cancels the app and closes serving. **UNREVIEWED MECHANISM** (architect-r2 3, logic-r2 2). A single longer probe tolerates the reported transient pause without retrying a timed-out driver connection. After stopping the monitor and draining handlers and workers, run `pg_advisory_unlock` on the live connection with a fresh bounded cleanup context. Close it even if unlock fails, then close the Store.

`main` calls `run(ctx context.Context, args []string, lookup func(string) (string, bool), stdout, stderr io.Writer) int` with `os.LookupEnv`. No argument serves. Stack's `health` command reads only `LGTM_LISTEN_ADDR` with default `127.0.0.1:8080`. Map empty or wildcard hosts to loopback and send `GET /healthz` with a two-second limit, no redirects and no proxy lookup. Exit 0 on 204 and 1 on probe failure. Extra arguments or unknown commands exit 2. The reserved `join-token` exits 2 with “The join-token command is unavailable in this build.” Sprint 3 installs `joinTokenCommand`, adapts the environment lookup and opens the same Store. It loads the established identity with `allowInitialize=false` without the serving lock. (architect-r1 1, logic2-r1 1)

On readiness, log `msg: link_signing_key_loaded`, `source: openbao`, public `fingerprint` and integer `vault_version` to standard output with `slog.NewJSONHandler` and `AddSource: false`. C13 reads the first three fields. Clean up failed startup. Drain HTTP for five seconds during shutdown, then close remaining connections, join workers and call `Follower.Close`. (architect-r1 11, threat-r1 8)

On failure, the command exits 1 and writes applicable sentences to standard error. Name invalid listener, origin and vault settings without values. C2 reads the password-setting line.

| Condition | Sentence on standard error |
|---|---|
| Missing, empty, invalid or overlong password. | LGTM_WEB_PASSWORD is missing or invalid. |
| Malformed database setting. | The PostgreSQL settings are invalid. |
| Database unavailable. | The database connection failed, check the PostgreSQL connection settings. |
| Instance lock busy. | Another server is already using this database. |
| Migration failed. | The server could not migrate its PostgreSQL database. |
| A database, admission or auth failure prevented session reconciliation. | Session reconciliation did not complete. |
| Password is unusable and a purge did not commit or could not be verified, including exit before attempting it. | Session revocation was not confirmed. |
| Purge committed while the password was unusable. | Session revocation committed. |
| Missing, inaccessible or incorrectly protected OpenBao credential. | The LGTM_OPENBAO_CREDENTIAL_FILE credential is missing or invalid. |
| Vault unreachable, sealed or login refused. | The server could not authenticate to OpenBao. |
| Signing key lost, unreadable or different from its pin. | OpenBao did not provide the established signing key. |

Write the applicable database or admission failure first, then the reconciliation failure if startup could not finish reconciliation. For an unusable password, always follow those details with exactly one revocation outcome and then the password-setting line. Only a verified commit selects the committed outcome. (logic-r2 5, Stack Web requests S10 and S13) Stack item 5 calls this program through `deploy/up.sh --remove-password` and reads both exact revocation lines. Removal stops the regular server and runs its image with the empty password, five PostgreSQL settings and private-stack opt-in only. (critic-r1 4, architect-r1 7, logic-r1 6, threat-r1 4)

The public test package has exactly this exported surface. The link tests import `github.com/qnlbnsl/lgtm-studio/src/server/servertest`.

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
func (s *Server) Environment() map[string]string
```

`Start` provisions a disposable PostgreSQL database in its existing `public` schema through `storetest.NewDatabase(t, "public")`. The existing Store helper already supports that schema. Use item 1 settings and a fixture-only owner-readable database password file for both the app and subsequent command tests. It starts the same application as the command using an ephemeral loopback port. Return the actual URL, a copied public key and a fresh cookie jar on unencrypted HTTP/2. The loopback vault fixture keeps its random seed across starts. `Stop` is idempotent. `Restart` closes old idle connections and reuses the address, database, jar and seed. Start and restart fail the test on error. Cleanup closes application and vault resources before dropping the fixture database. Credentials belong to the fixture. Item 9 writes `Environment` to return a new map of the exact fixture settings on each call. The sprint 3 command test passes a lookup over that map to `joinTokenCommand`. Keep fixture settings off logs. Sprint 3 Server edits `app.Start` directly to register the additional handlers and migrations. (architect-r1 6)

Write these tests first in the named files. Item 9's private helpers and the tests that use them share their respective Go packages.

- `TestStartRevokesBeforeListen` issues a session, stops the app and starts with a changed password. Observe the startup backend waiting at a real reconciliation gate and require the port closed. Release the gate and require readiness and old-cookie refusal. For missing and overlong passwords with the vault unavailable, require committed purge, fixed diagnostics and no listener. Starting again with the original password leaves the old cookie invalid. Force a purge rollback and require the unconfirmed line and closed port. (logic2-r1 9)
- `TestStartWithUnavailableVault` uses migrated real PostgreSQL with a refused or malformed OpenBao wire fixture. Require startup failure, no listener, no success log and no secrets in diagnostics. After the fixture becomes readable, require startup, a 204 health response and one decoded key event with the exact message, source, fingerprint and integer vault version. (architect-r1 11)
- `TestStartRefusesSecondInstance` starts one app and signs in. Start another using the same database and a different password. Require the busy diagnostic, preservation of the original session and a closed second listener. Stop the first, then start the second and require rejection of the original cookie. Terminate the dedicated backend in a separate run and require closure of its listener within the probe bound. (architect-r1 4, logic-r1 1, threat-r1 1)
- `TestStartupMigrationBudget` passes shorter durations to `start`. Use a real migration gate, record the waiter and release it after the prerequisite duration would have elapsed but before the migration deadline. Require readiness. In a separate case terminate the lock backend while the gate is held and require startup to exit without readiness. (logic2-r1 8, architect-r2 2, architect-r2 3)
- `TestServerInstanceLock` in Store's `instance_test.go` asserts three distinct reserved lock identifiers. Hold the lock on one backend and observe the second waiting before releasing it within the budget. Require acquisition. Hold it beyond a short test deadline and require the busy diagnostic. Close a held lock, then acquire it immediately on another backend. A PostgreSQL driver query tracer delays only the first `SELECT 1` by 1.5 seconds. Require the probe to succeed on the same backend. `TestStartRefusesSecondInstance` covers loss of the serving connection. (architect-r2 3, logic-r2 1, logic-r2 2, logic-r2 6)
- `TestStartupDiagnostics` in `main_test.go` exercises the table through `run`, including an unusable password with malformed database settings, database refusal and an aborted purge. Test a committed removal with only the removal route's seven settings. Assert status 1, exact applicable lines and order, empty standard output and absence of fixture values. A retired variable warning names the variable only. (critic-r1 4, architect-r1 2)
- `TestHealthCommand` in `main_test.go` checks 204 from the started app with wildcard mapping, and failure on a closed port, redirect and non-204 response. Assert no settings beyond `LGTM_LISTEN_ADDR` are read. (architect-r1 1)
- `TestStartAndRestartServer` runs as `servertest_test`, imports the public package and signs in through the generated client. Stop the server, observe the listener is closed, restart it and assert the same URL, same public key and a signed-in GetServer through the existing jar. Check the same HTTP/2 listener answers a generated gRPC GetServer client. Start another fixture and prove database and cookie isolation. Mutate a returned environment map and check a fresh copy and the running fixture are unchanged. An empty-password fixture must fail through the normal startup path, covered by the app test.

This item is risky. Opening the port before reconciliation must fail `TestStartRevokesBeforeListen`. Bypassing exclusive admission or the bounded health probe must fail the corresponding new test. Starting with a transient key when OpenBao fails must fail `TestStartWithUnavailableVault`. Recreating the test database or key on restart must fail `TestStartAndRestartServer`. The named tests gate item 9. `TestHealthCommand` covers readiness for C1. `TestStartupDiagnostics` and `TestStartRevokesBeforeListen` cover refusal and revocation for C2 and C4. `TestStartAndRestartServer` and `TestStartWithUnavailableVault` cover key continuity and failure for C13 using the wire fixture.

**Design artifact.** The foundation design's compose and startup checks specify the existing website entry point. There is no changed screen hierarchy.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Startup serves before validating existing cookies or securing the link identity. | High | The app defers listening until validation completes. The tests block a prerequisite and probe before allowing startup to proceed. |
| A vault error or a racing first startup supplies a different key. | High | Use a create-only write and require a persisted read before logging readiness. Exercise the error and the competing first request. |
| An uncommitted password removal leaves a prior session in storage. | High | Fail closed and report the purge failure. Verify a successful removal is durable before restoring the old password. |

**Verify.** Run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...`. The orchestrator runs the deployed joint gate in rollout step 3. No device check is needed for this workstream.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe. The brief's eight items and four planning points, issues 1632 through 1640, the wave 1 driver names and the following foundation statements. D1–D5, D7, D9–D11, R1–R4, R7–R11, C1–C4, C12's database fixture and C13's server clause. Design sections are Sign-in and web sessions, Serving the contract, Secrets live in OpenBao and Compose settings. [Foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md). [Release driver](../../../deploy/test/README.md).

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
| OpenBao, server-facing Compose settings, C1 readiness and the server clause of C13. | WS-3 | Items 1, 8 and 9 cover settings, key and listener with named tests. The orchestrator runs the deployed joint gate. |
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
| 2. Password. | backend | server/backend | medium | It isolates the password comparison and binding format in one package and gives them independent tests. | Yes. | Lists its scrypt pin. | 1, 5, 8. | R9, C3, C4. |
| 3. Schema and reconciliation. | migrations | server/backend | medium | It applies one additive migration and tests the startup purge independently of runtime session writes. | Yes. | 2 and the Store migration and write surfaces. | 1, 5, 8. | D3, D4, D5, R1, R3, R4, R7, R9, R10, C2, C4. |
| 4. Sessions. | backend | server/backend | medium | It tests deterministic runtime mutation order separately from schema and password reconciliation. | Yes. | 2, 3 and Store read and write handles. | 5, 8. | D3, D4, D5, R3, R4, R7, R9, R10, C3, C4. |
| 5. Browser origins and cookies. | backend | server/backend | medium | It computes web policy with independent origin and cookie fixtures. | Yes. | Lists its public-suffix pin. | 1, 2, 3, 4, 8. | D11, R8, R9, C3. |
| 6. Router and guard. | backend | server/backend | high | Freeze admission and preflight/error tests as separate groups. Both exercise the same routing boundary. (critic-r1 11) | Yes. | 4, 5 and the sprint 0 generated handlers. | 8. | D1, D11, R2, R8, R9, C3. |
| 7. Session methods. | backend | server/backend | medium | It joins the existing password, session and origin components through three generated methods. | Yes. | 2, 4, 5, 6. | 8. | D1, D11, R2, R8, R9, C3, C4. |
| 8. Vault reader. | backend | server/backend | high | It combines the wire reader with a small, independently tested public-pin migration. | Yes. | Sprint 1 Stack credential and Store interfaces. | 1 through 7 once ready. | D7, D9, D10, R1, R4, R9, R10, C13. |
| 9. Application and fixture. | backend | server/backend | high | Freeze app and command fixtures before the public test-package checks. The exact-commit gate covers their shared startup. (critic-r1 11) | Yes. | 1, 3, 4, 5, 6, 7, 8 and Store lifecycle and fixture exports. | None within this plan. | D1, D2, D3, D4, D7, D9, D10, D11, R7, R8, R9, R10, C2, C3, C4, C12, C13. |

Load for nine test writers, nine builders, the first review round and nine build review or mutation seats is estimated at 13 OpenAI, 16 Anthropic and 4 xAI launches. That uses a nine and nine split for test writing and building, a one, four and one split for the review round, and three build reviews per vendor. Weekly remaining allowances were unavailable to this planner. The orchestrator compares the estimate against the meters before launch and schedules any dependent work that exceeds an allowance after its reset.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (blocker, major, minor) | Outcome |
|---|---|---|
| 1 | 2 blocker, 19 major and 28 minor reports before grouping. | The [round 1 register](attack/round-1-findings.md) records scores, patches and owner choices. |
| 2 | 0 blocker, 7 major and 13 minor reports before grouping. | The [round 2 register](attack/round-2-findings.md) records the Stack handoff, corrected lock and test seams, and the follow-up issues. |

</details>

<details><summary><b>Post-build checks</b></summary>

| Check | Scenario | Expected observation |
|---|---|---|
| PB1. Sprint 2 Compose and browser evidence. | Run the joint gate at the default port and with `LGTM_WEB_PORT=18787`. Check the rendered setting names and the bundled loopback aliases. Sign in with the existing website and reload. Query GetServer in that profile and a fresh one. Replace the cookie from a sibling loopback port with an invalid fixture value. Run C4 with a 4096-byte fixture password. (architect-r1 1, logic2-r1 11, threat-r2 3, Stack Web request S12) | Readiness and five release checks pass. The browser sends no request key on SignIn. GetServer is signed in after reload, and signed out in the fresh profile and after the cookie replacement. The replacement grants no access and causes no server error. C4 uses an independently generated valid replacement password. |
| PB2. HTTPS browser cookie isolation after wave 1. | In the later proxy work, sign in and out from the bundled and admitted cross-site HTTPS sites in both orders. Try a sibling-domain cookie, an Origin-less cross-site cookie and the same request after removing its origin. Repeat in a browser that blocks cross-site cookies. (Stack Web request E2) | Each HTTPS mode leaves the other cookie alone. Unlisted or Origin-less cross-site requests gain no access. The bundled same-site address works when the browser blocks cross-site cookies. Home and website sign-out on loopback are covered at the Stack Web plan's Machines gate in sprint 3. |
| PB3. Password transition and exclusive startup. | With A serving, start B with a changed password against its database. Then invoke Stack's `deploy/up.sh --remove-password` with PostgreSQL interrupted, and repeat after recovery while OpenBao is unreachable. Restore the former password only after confirmed removal. (architect-r1 7, logic2-r1 2, Stack Web request S13) | B cannot reconcile or serve during A's tenure. The failed removal reports unconfirmed revocation and serving stays stopped. The retry confirms revocation before the vault step. The old cookie remains invalid with the restored password. |
| PB4. Real OpenBao creation and continuity. | On the pinned disposable vault exercise an absent key, versioned deletion and a sealed vault. Recreate the normal stack and compare fingerprint and version. Inspect the active role against Stack's built policy test. The competing 400, 403 and lost-reply scenarios run in the scripted wire fixture. (architect-r2 1, threat-r2 1) | One initial version persists across recreation. Deletion and sealing close readiness. The role matches the existing policy, and Server performs no update. Diagnostic output contains no fixture secrets. |
| PB5. Signing identity continuity. | After first startup, simulate an established key replaced by a different valid value or absent from OpenBao in isolated fixtures. (logic2-r1 4) | Readiness remains closed, the public pin is unchanged and the server sends no creation request. |
| PB6. Watch revocation. | Once Machines supplies its streaming handler, revoke a session while its watch is quiet or a send is blocked. Start another watch with the old token. | The existing stream stops within the Store recheck bound. The client receives `unauthenticated` when the transport can write it, and a fresh watch is refused. |
| PB7. Unconfirmed sign-in. | With eight live rows, exercise an unconfirmed commit and, separately, discard a reply after a confirmed commit. Observe the original and repeated unkeyed actions under the provisional retry policy. (logic-r1 2, logic2-r1 7) | The internal retry count is zero, the unknown outcome sets no cookie, and the cap holds. A committed undisclosed session can displace the single oldest session, and a separate explicit sign-in can displace another. |

</details>
