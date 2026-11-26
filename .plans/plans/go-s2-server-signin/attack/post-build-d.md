# REVISE

SignOut can revoke a session and clear its cookie after the client's deadline because the router verifies the session before Connect starts the request timer.

This report covers items 6 and 7 of the sign-in plan at commit `85b05640` on `sprint-0`. The task is `postbuild-go-s2-server-signin-d`, with nonce `b9ab98`.

## Findings

### post-d 1. MAJOR. The request deadline starts after the session guard

**Statement.** Item 6 requires the request deadline to produce `deadline_exceeded` with “The request timed out.” This implements design rule R8. [Plan error mapping](../plan.md#L309) · [Design rule R8](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L369)

**Evidence.** By reading, the router calls the session guard before it invokes the generated handler. The guard passes the original Hypertext Transfer Protocol (HTTP) context to the database lookup. Connect v1.21.1 sets the protocol timeout inside its handler at `handler.go:319`. Both Connect and Google Remote Procedure Call (gRPC) parse their timeout headers at that stage. [Router](../../../../src/server/internal/api/server.go#L285) · [Session guard](../../../../src/server/internal/api/session_check.go#L40)

**Failure scenario.** Confirmed live, a SignOut with a 100-millisecond timeout waited behind a 350-millisecond PostgreSQL table lock. It then returned HTTP 200, deleted the only session and sent a clearing cookie. A separate protected Client call with a 25-millisecond timeout also returned HTTP 200 and entered its handler once after the lock was released.

**Fix.** Start the protocol deadline before the session lookup and carry the same deadline through the generated call. Connect's pre-decode request gate already receives the protocol context. Keep the origin check before body decoding and preserve the session guard on the fallback route. Exercise a locked SignOut and assert the timeout reply, an unchanged session and no Set-Cookie.

### post-d 2. MINOR. A malformed sign-in can echo the submitted password

**Statement.** Item 7 keeps passwords out of response fields. Design rules R8 and R9 require a public sentence and keep secrets out of the website. [SignIn rules](../plan.md#L336) · [Design rules R8 and R9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L369)

**Evidence.** By reading, the generated Session handler uses the default JavaScript Object Notation (JSON) decoder. Protocol Buffers v1.36.12 includes the raw token in the string-field type error at `encoding/protojson/decode.go:349`. Connect v1.21.1 wraps and serializes that message in `protocol_connect.go:1182` and `:748` before SignIn receives a decoded request. [Session handler registration](../../../../src/server/internal/api/server.go#L125) · [SignIn method](../../../../src/server/internal/api/session.go#L74)

**Failure scenario.** Confirmed live, a probe configured a numeric fixture password, then submitted the same digits as a JSON number. The response returned HTTP 400 with `invalid_argument`, repeated the submitted digits in the error body and issued no cookie. A client or error collector that surfaces that body can retain the value.

**Fix.** Give Session JSON decoding a fixed public message for malformed input. Preserve the existing error code and the separately specified size error. Cover the response body with a malformed fixture password test.

### post-d 3. MINOR. The frozen preflight cases miss a generated Client procedure

**Statement.** Item 6 admits a supported preflight for every generated web procedure, including procedures whose handlers are pending. [Procedure recognition](../plan.md#L290) · [Preflight rules](../plan.md#L294)

**Evidence.** By reading, admission depends on a manually maintained procedure table. The successful preflight cases in `TestBrowserOriginGate` cover SignIn and WatchClients. Confirmed live, removing only `ClientServiceRemoveClientProcedure` from the preflight table passed all six frozen tests. [Procedure table](../../../../src/server/internal/api/server.go#L54) · [Preflight gate](../../../../src/server/internal/api/server.go#L222) · [Frozen cases](../../../../src/server/internal/api/session_check_test.go#L139)

**Failure scenario.** Confirmed live, the supplemental preflight for RemoveClient returned HTTP 204 on the built code and HTTP 403 on the mutation. An allowed cross-origin website would fail before its RemoveClient call reached the service.

**Fix.** Exercise an allowed preflight for every method in the four generated service descriptors, including methods with no handler registered.

## Grade

**7 out of 10.** One major and two minors. The deadline defect drives the revise verdict.

## Post-build scenarios

The current attack register records the relevant changes under `architect-r3 3` and `logic-r3 1`. The current plan marks the read-bound mechanism as unreviewed. The round 4 report asks for an interrupted read over the transport. [Round 3 register](round-3-findings.md#L11) · [Read-bound mechanism](../plan.md#L312) · [Round 4 checks](round-4-logic.md#post-build-checks)

| Scenario | Confirmed live |
|---|---|
| A complete Session body exceeds 32,768 bytes, plain and compressed. | Both frozen cases returned HTTP 429 with `resource_exhausted` and `configured max 32768`. Neither set a cookie or wrote a session or feed entry. A body at the limit carrying a 4,096-byte password succeeded. |
| A body reader fails after crossing the bound. | The frozen interrupted-discard case passed. Over a local HTTP connection, sending 32,769 bytes and closing only the write side produced HTTP 429 with `resource_exhausted`, `configured max 32768` and an unexpected end-of-file diagnostic. It set no cookie and wrote no session or feed entry. |
| The database clock is at the session creation skew boundary. | `TestSessionClockRefusal` passed with HTTP 409, `aborted` and the plan's sentence. The session row and feed stayed unchanged. |
| Post-build check PB1, the loopback sign-in and sibling-cookie portion. | A browser-style Origin at an ephemeral loopback port signed in. The signed-in cookie jar stayed signed in on the next GetServer, and a fresh jar was signed out. After an invalid cookie arrived from a sibling port, GetServer returned signed out and a protected call returned `unauthenticated`. The original database row stayed present. |
| PB2, the router's cookie mode selection. | The frozen in-process test kept strict and cross-site cookies independent. It refused to use either cookie under the other mode and cleared the cross-site cookie while retaining the strict session. |
| PB7, an unconfirmed commit with eight live rows. | The fixture terminated the database writer while it waited inside COMMIT. The response was HTTP 503 with the unknown-outcome sentence and no cookie. The insert counter advanced once. All eight original rows and the feed stayed unchanged. |
| PB7, a discarded successful reply and a separate explicit retry. | Each request committed a distinct token and evicted only the oldest row. The cap held at eight and no receipts were written. |

The HTTP interruption probe kept the response side of the connection open. A client that closes both sides cannot receive the reply. The unconfirmed-commit fixture covers a lost acknowledgement with rollback. The discarded-success case covers a committed session whose reply the caller loses.

The full PB1 gate requires the runnable server and Stack integration in rollout step 3. The reviewed checkout had no server command or application wiring. The deployed website reload, the two Compose ports and the restart in design check C4 remain sprint-gate evidence. The full PB2 gate requires the later Hypertext Transfer Protocol Secure (HTTPS) proxy and browser cookie policy. PB6 requires the Machines watch handler. [Rollout gate](../plan.md#L50) · [Post-build checks](../plan.md#L642) · [Design check C4](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L381)

## Plan criteria and named mutations

By reading the implementation and confirming with the package suite, criterion 4 holds at the request boundary. The router checks the Origin before reading a cookie or dispatching, and SignIn and SignOut use the selected cookie mode. Criterion 5 holds for decoded sign-in requests. The password comes from the body, the password helper enforces the byte limit, and a committed issue returns its credential in Set-Cookie. The malformed input exception is post-d 2. Criterion 6 holds for the public procedures, the protected unary and streaming cases and the authenticated fallback. The timeout exception is post-d 1. The exact Link procedures use their independent route. [Criteria 4 to 6](../plan.md#L220) · [Routing](../../../../src/server/internal/api/server.go#L156) · [Session methods](../../../../src/server/internal/api/session.go#L38) · [Password limit](../../../../src/server/internal/auth/password.go#L33)

The builder explains its readiness methods, Link preflight refusal and empty replies for requests outside a recognized protocol. The two implementation findings cover behavior outside those explanations. [Builder result](../../../../.foundry/go-rewrite/results/s2-signin-build-d.md#places-where-the-plan-was-unclear)

| Scratch mutation | Frozen test result |
|---|---|
| Exempt the whole Session service from authentication. | `TestWebRoutesRequireSession` failed. Anonymous SignOut returned HTTP 200 and entered the probe. [Assertion](../../../../src/server/internal/api/server_test.go#L124) |
| Dispatch before refusing a denied Origin. | `TestBrowserOriginGate` failed. A denied valid SignIn reached the probe and returned HTTP 200. [Assertion](../../../../src/server/internal/api/session_check_test.go#L224) |
| Stage a SignIn cookie before checking the commit result and send it on error. | `TestSessionContract/confirmed_rollback` failed because the reply changed a cookie. [Assertion](../../../../src/server/internal/api/session_test.go#L470) |
| Accept a request key on SignIn. | `TestSessionRequestKeys/SignIn` failed in the present, empty and repeated cases because SignIn succeeded and set a cookie. [Assertion](../../../../src/server/internal/api/session_test.go#L608) |
| Map the clock refusal to a storage failure. | `TestSessionClockRefusal` failed because it got HTTP 503 and `unavailable` in place of HTTP 409 and `aborted`. [Assertion](../../../../src/server/internal/api/session_test.go#L581) |

The cookie mutation simulates early publication at the reply boundary. The real issuance method returns its token only after a successful commit. [Token return](../../../../src/server/internal/auth/sessions.go#L133)

## Package verification

The frozen test files matched `1038a36b`. The plan's command exited zero and printed the following package result.

```text
$ deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/api
ok  	github.com/qnlbnsl/lgtm-studio/src/server/internal/api	66.812s
```

The supplemental read-limit run passed in 4.202 seconds. The mutation that omitted RemoveClient preflight passed all frozen tests in 15.420 seconds. All five named mutations failed their targeted assertions. The supplemental deadline and malformed-password probes failed against the built code as described above.
