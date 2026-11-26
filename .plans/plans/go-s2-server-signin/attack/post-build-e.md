# REVISE

A protected call routed to an unimplemented service can wait past its protocol deadline while the server verifies the session.

This review covers items 6 and 7 of the sign-in plan on `sprint-0`, through the application programming interface (API) commit `d9a9326b`. The task is `postbuild-go-s2-server-signin-e` with nonce `442959`.

## Findings

### post-e 1. MINOR. The fallback session lookup ignores the protocol deadline

**Statement.** Item 6 sends recognized but unimplemented procedures and unknown protocol paths through the session guard. It maps an expired request deadline to `deadline_exceeded` with “The request timed out.” This implements design rule R8. [Fallback routing](../plan.md#L290) · [Deadline error](../plan.md#L309) · [Design rule R8](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L369)

**Evidence.** By reading, the registered handlers use Connect's request gate. The fallback passes the original Hypertext Transfer Protocol (HTTP) request context directly to `checkSession`. That check uses the supplied context for the PostgreSQL lookup. The builder records this difference. [Registered gate](../../../../src/server/internal/api/server.go#L125) · [Fallback guard](../../../../src/server/internal/api/server.go#L288) · [Lookup](../../../../src/server/internal/api/session_check.go#L62) · [Builder result](../../../../.foundry/go-rewrite/results/s2-signin-build-e.md#L84)

**Failure scenario.** Confirmed live, a valid session called `ApproveClient` with the Client service absent. A PostgreSQL table lock held the lookup beyond the 250-millisecond timeout sent by the client. Both Connect and Google Remote Procedure Call (gRPC) returned `unimplemented` after the lock was released. The registered calls returned the timeout error in the equivalent frozen test. [Registered deadline cases](../../../../src/server/internal/api/deadline_test.go#L34) · [Lock fixture](../../../../src/server/internal/api/deadline_test.go#L109)

**Fix.** Apply the protocol deadline before the fallback lookup and exercise a locked lookup with both supported timeout headers. Retain the origin and session checks on the fallback.

### post-e 2. MINOR. The frozen tests allow body reads before the origin check

**Statement.** Item 6 puts origin validation before decoding a web request. The bounded review also requires checking that a refused origin reaches no body read. [Origin order](../plan.md#L294) · [Review assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s2-server-signin-e.md)

**Evidence.** By reading, `TestBrowserOriginGate` verifies the refusal for malformed input and checks handler calls. The fixture uses a normal reader and does not count reads. Confirmed live, a scratch mutation that peeked at the body and buffered it before validating the origin passed the full frozen API suite in 18.172 seconds. The same mutation failed the review's instrumented body check. [Refused request test](../../../../src/server/internal/api/session_check_test.go#L192) · [Current origin gate](../../../../src/server/internal/api/server.go#L273)

**Failure scenario.** A future router could read from a slow request carrying a denied origin before sending the refusal. It could still return the expected status and leave the handler call count unchanged, satisfying the current test.

**Fix.** Use a request body that records reads in the frozen origin test and assert zero reads for a denied registered call and a denied fallback call. The built router already satisfies that assertion.

## Grade

**8 out of 10.** The fallback deadline remains the reason to revise. Both repairs work through the registered handlers, and the session and origin checks hold on both routes.

## Post-build scenarios

The plan and review register use the label `UNREVIEWED MECHANISM` for the Session read limit. I used that entry and the round 4 transport scenario because the review register has no “Not reviewed before the build” section. [Plan's read bound](../plan.md#L312) · [Round 3 register](round-3-findings.md#L11) · [Round 4 transport check](round-4-logic.md#post-build-checks)

| Scenario | Confirmed live result |
|---|---|
| A complete request exceeds 32,768 bytes. | Plain and compressed SignIn returned HTTP 429, `resource_exhausted` and `configured max 32768`. Neither changed session state or issued a cookie. The bounded request carrying a 4,096-byte password succeeded. |
| An interrupted request crosses the bound. | The frozen interrupted-discard case passed. A local connection also sent 32,769 bytes and closed its write side before the advertised body ended. The response was HTTP 429 with `resource_exhausted`, `configured max 32768` and an unexpected end-of-file diagnostic. It issued no cookie, and session storage was unchanged. |
| A session creation would exceed the database clock skew limit. | `TestSessionClockRefusal` passed. It asserted HTTP 409, `aborted`, the plan's sentence, an unchanged session snapshot and no cookie. |
| PB1, the router portion at the nondefault loopback port. | The synthetic browser origin signed in. GetServer reported signed in with the issued cookie and signed out with either no cookie or the invalid replacement. The invalid replacement received `unauthenticated` on SignOut without changing session storage. |
| PB2, cookie mode selection. | The frozen mode test verified that both cookies stay independent. GetServer refused each cookie in the other mode. Cross-site SignOut cleared only its own cookie and preserved the strict session. |
| PB7, unconfirmed commit and separate retries at capacity. | With eight sessions, the interrupted commit returned HTTP 503 with the unknown-outcome sentence. It issued no cookie, attempted one insert and preserved all eight rows and the feed. Two subsequent explicit sign-ins each replaced only the oldest row. The cap held and no receipts were written. |

The full PB1 deployed website and restart check belongs to the joint sprint gate. The full PB2 browser check requires the later Hypertext Transfer Protocol Secure (HTTPS) proxy. PB6 requires the Machines watch service. By reading, the server currently registers only the Session service, and the test probe cannot exercise revocation of a quiet production stream. [Rollout gate](../plan.md#L50) · [Post-build checks](../plan.md#L642) · [Server registration](../../../../src/server/internal/app/start.go#L225)

## Registered and fallback request boundaries

- **Criterion 4.** Confirmed live, the origin check returned `permission_denied` with zero handler-level body reads across all 22 generated web procedures and two unknown namespace paths. The probe covered both the configured Session and Client handlers and their absence. It used denied-origin requests with missing, malformed and valid session cookies on protected procedures. The preflight test passed for all 22 generated procedures. [Criteria](../plan.md#L220) · [Origin check](../../../../src/server/internal/api/server.go#L273) · [Preflight assertions](../../../../src/server/internal/api/session_review_test.go#L43)
- **Criterion 5.** Confirmed live, normal SignIn kept its password and cookie behavior. Malformed JavaScript Object Notation (JSON) returned `invalid_argument` with “The request body is invalid.” and no cookie for both Connect media types. Nonempty malformed JSON messages sent over gRPC and gRPC-Web returned the same code and sentence. The frozen test verified that the reply does not repeat a submitted password. [SignIn assertions](../../../../src/server/internal/api/session_review_test.go#L18) · [Session codec](../../../../src/server/internal/api/session_decode.go#L23) · [Design rule R9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370)
- **Criterion 6.** By reading, the registered Session and Client handlers share a gate that exempts only GetServer and SignIn. The fallback guards every other web or unknown protocol path and replies `unimplemented` after successful authentication. Confirmed live, missing and malformed sessions were refused in the procedure matrix. Registered unary and streaming cases passed across the three transports. A fallback WatchMap returned `unauthenticated` with a missing or malformed cookie, `unimplemented` with a valid cookie and `permission_denied` for a denied origin on all three transports. The frozen deadline test confirmed that locked registered SignOut and ApproveClient calls returned the timeout error, issued no clearing cookie, retained the session and left the Client handler uncalled. [Public procedures](../../../../src/server/internal/api/server.go#L35) · [Gate and verification](../../../../src/server/internal/api/session_check.go#L41) · [Stream tests](../../../../src/server/internal/api/session_check_test.go#L306) · [Deadline assertions](../../../../src/server/internal/api/deadline_test.go#L78)

By reading, Connect validates transport metadata before its registered request gate. As the builder records, incorrect transport metadata can receive a protocol refusal before the session check. The origin check still precedes both validations. [Origin and dispatch](../../../../src/server/internal/api/server.go#L273) · [Builder's transport note](../../../../.foundry/go-rewrite/results/s2-signin-build-e.md#L83)

## Scratch mutations and package verification

| Scratch mutation | Confirmed live result |
|---|---|
| Exempt the Session service prefix from verification. | `TestWebRoutesRequireSession` failed. Anonymous SignOut returned HTTP 200, and an unknown Session procedure returned HTTP 501. Both should return HTTP 401. |
| Dispatch a registered request before checking its origin. | `TestBrowserOriginGate` failed. A valid SignIn with a denied origin reached the handler and returned HTTP 200. |
| Publish a SignIn cookie when its database write fails. | Both confirmed rollback cases failed because the response carried a cookie. This mutation simulates publication at the reply boundary. |
| Accept a request key on SignIn. | The present, empty and repeated cases in `TestSessionRequestKeys` failed because the call succeeded and issued a cookie. |
| Map the session clock refusal to a storage failure. | `TestSessionClockRefusal` failed with HTTP 503 and `unavailable` in place of HTTP 409 and `aborted`. |
| Restore the router's session verification before the Connect deadline. | The four frozen deadline cases failed. ApproveClient entered the handler, and SignOut changed the session and cleared the cookie. |
| Remove the Session JSON codec and interceptor. | `TestMalformedSignInRedactsPassword` failed the redaction and fixed-message assertions. |
| Omit RemoveClient from the preflight table. | `TestGeneratedWebProcedurePreflights` failed the RemoveClient case with HTTP 403 in place of HTTP 204. |
| Peek at the body before origin validation. | The full frozen suite passed. The review's body-read probe failed in both router configurations. |

The frozen API tests matched `9be53aec`. The plan's command exited zero on the branch checkout and printed the following result. All supplemental and mutation tests ran through the same PostgreSQL wrapper.

```text
$ deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/api
ok  	github.com/qnlbnsl/lgtm-studio/src/server/internal/api	69.559s
```

The same command exited zero from a scratch copy of commit `d9a9326b` and printed the following result.

```text
ok  	github.com/qnlbnsl/lgtm-studio/src/server/internal/api	67.147s
```

The supplemental route, deadline, body and cookie probes passed in 7.881 seconds. The wire interruption, fallback streaming and malformed message probes passed in 3.829 seconds. The fallback deadline probe asserted the observed late `unimplemented` response so it could distinguish that behavior from a test-fixture failure. No production or frozen test files were changed.
