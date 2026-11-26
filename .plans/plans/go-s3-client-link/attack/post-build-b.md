# REVISE

The top finding is that both supported stream protocols can turn a leading response header into a terminal refusal after a valid challenge, even when the error itself carries no refusal reason.

The builder was a worker from my own vendor, so this review did not meet the requirement for review by another vendor.

Task `postbuild-go-s3-client-link-b`, nonce `49e73f`. I reviewed items 1b, 2 and 3 at `7dd7cbb8` against the frozen tests at `2ea7e4c0` and the frozen Protocol Buffers contract. The findings cover the outbox sequence limit, enrollment, the stream handshake and its elapsed clock.

## Findings

### post-b 1 · MAJOR · Leading response metadata can stop the client

**Statement.** Decision 16 (D16) and plan item 3 require another dial when a `failed_precondition` error has no recognized refusal reason. The frozen contract places that reason in the end-of-stream metadata for Connect and in the trailers for gRPC Remote Procedure Calls (gRPC). [Client plan](../plan.md#L161) · [Frozen link contract](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L47)

**Evidence, confirmed live.** The classifier reads the first value in `remote.Meta()` after verifying the challenge. Connect Go 1.21.1 constructs that map by copying leading headers and appending trailing metadata in both protocols. After an honest challenge and proof exchange, a local fixture supplied `unknown_machine` only in the leading headers and returned `failed_precondition` with no reason in its error metadata. Both Connect and gRPC returned a terminal `unknown_machine` instead of an error that permits retry. The frozen tests exercise recognized reasons in trailing metadata and leave the leading-header flag false in every retry case. [Classifier](../../../../src/client/internal/link/conn.go#L209) · [Frozen refusal tests](../../../../src/client/internal/link/conn_test.go#L886)

**Failure scenario.** A stale header added by an intermediary survives an otherwise retryable error from a server whose challenge verified. The classifier reports a permanent removal, which the reconnect supervisor would treat as a stop.

**Fix.** Read the terminal reason from the stream's trailing metadata after the receive error and require one recognized value. Preserve the verified-challenge gate and fixed error text. Test a header-only reason and conflicting header and trailer reasons in both protocols.

### post-b 2 · MAJOR · Frame diagnostics write enrollment secrets and connection tokens to logs

**Statement.** Rule 9 (R9) prohibits secrets in logs. The frozen contract identifies the join token as secret and uses the connection token as a credential for later calls. [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370) · [Join token](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L180) · [Connection token](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L288)

**Evidence, confirmed live.** Enrollment puts the secret in the request body and the approved welcome carries the connection token in its response body. With Go 1.26.0 frame debugging enabled through `GODEBUG=http2debug=2`, I found the synthetic enrollment secret in 10 logged request frames and recovered the synthetic connection token from two logged response frames after decompressing the frame data. The same fixtures emitted zero frame logs and zero literal occurrences of the synthetic values with frame debugging disabled. The toolchain's `net/http/h2_bundle.go` formats up to 256 bytes of each data frame at line 3301. Sanitizing the returned error cannot affect that logging path. [Enrollment request](../../../../src/client/internal/enroll/join.go#L212) · [Welcome receive path](../../../../src/client/internal/link/conn.go#L147) · [Hypertext Transfer Protocol version 2 transport](../../../../src/client/internal/transport/transport.go#L110)

**Failure scenario.** The client inherits frame debugging during a connection diagnosis. Standard error or the service journal then holds enough data to recover credentials exchanged while debugging was enabled.

**Fix.** Disable payload logging for the credential-bearing transport or stop before sending when the inherited debug settings enable it. Cover both enrollment and an approved stream in a subprocess test with synthetic credentials and captured standard error.

### post-b 3 · MAJOR · Enrollment confuses local response classification with a token refusal

**Statement.** Plan item 2 and rule 8 (R8) require the two defined enrollment refusals to stop, a bad reply to stop with a fixed diagnostic, and transient failures to retry with the same credentials. [Client plan](../plan.md#L114) · [Frozen enrollment statuses](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L47) · [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L369)

**Evidence, confirmed live.** `joinRefusal` accepts `permission_denied` and `invalid_argument` without checking whether the peer sent a remote procedure call (RPC) error. Connect Go maps a plain Hypertext Transfer Protocol (HTTP) 403 to a locally synthesized `permission_denied`. A local plain-text 403 stopped enrollment as `token_refused` after one request, with no Link service refusal. A plain-text HTTP 200 went the other way and retried 63 times on the fake clock before returning `join_timeout`. The frozen bad-reply cases cover invalid message fields and oversize bodies. [Retry and classification](../../../../src/client/internal/enroll/join.go#L315) · [Refusal switch](../../../../src/client/internal/enroll/join.go#L346) · [Frozen enrollment cases](../../../../src/client/internal/enroll/join_test.go#L586)

**Failure scenario.** A proxy's temporary access rule produces a token error that sends the owner looking for a new token. A route accidentally serving a successful web response spends the full enrollment budget repeating a request that cannot succeed in that configuration.

**Fix.** Require a wire RPC error for either contract refusal and keep the fixed fallback for a genuine permission denial with no recognized reason. Recognize unusable successful HTTP replies as `invalid_reply` using response state or a typed error, without forwarding body text. Add both response shapes to the tests.

### post-b 4 · MAJOR · A malformed welcome can stop the client on revision mismatch

**Statement.** Plan item 3 requires a valid welcome and a stop on an incompatible welcome revision. The frozen contract also requires an empty token and machine name for explicit waiting standing and forbids removed standing in a welcome. [Client plan](../plan.md#L158) · [Waiting and removal rules](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L155) · [Machine state](../../../../src/contracts/proto/lgtm/link/v1/commands.proto#L123)

**Evidence, confirmed live.** `validateWelcome` checks the link revision before checking the approval and token combination. A signed handshake followed by a waiting welcome with a token and another revision produced a terminal `link_revision_mismatch`. A waiting welcome with the current revision and a nonempty machine name opened successfully. The approved branch also accepts an empty token, and the frozen proof fixture explicitly expects it despite the contract assigning a token to an approved stream. [Welcome validation](../../../../src/client/internal/link/conn.go#L167) · [Frozen proof fixture](../../../../src/client/internal/link/conn_test.go#L445)

**Failure scenario.** A server rollout emits an invalid combination of fields and a different revision. The client classifies it as an incompatible peer and stops on a frame that should have been rejected as malformed.

**Fix.** Validate the state, token and server time before treating a revision mismatch as terminal. Keep the fallback that maps unknown approval to waiting and discards its token. Correct the fixtures and cover mixed-invalid frames, a named waiting state and an approved stream without a token.

### post-b 5 · MINOR · The stream tests accept a clock that stops during suspend

**Statement.** Plan item 3 and D16 require the reading at proof send to count time spent asleep. The contract requires the client to decline new work when it cannot establish that it will start before expiry. [Clock requirement](../plan.md#L159) · [Command expiry](../../../../src/contracts/proto/lgtm/link/v1/commands.proto#L26)

**Evidence, confirmed live.** Replacing Linux `CLOCK_BOOTTIME` with `CLOCK_MONOTONIC` in a scratch copy left every `TestOpen` test passing with the race detector. The fixture compares the send instant with two readings from the same helper. The clock test checks that successive readings are positive and ordered. The built code selects the specified clock on each platform. [Linux clock](../../../../src/client/internal/link/clock_linux.go#L10) · [macOS clock](../../../../src/client/internal/link/clock_darwin.go#L10) · [Proof timing assertion](../../../../src/client/internal/link/conn_test.go#L454) · [Clock test](../../../../src/client/internal/link/conn_test.go#L955)

**Failure scenario.** A later simplification uses a monotonic clock that excludes suspend. The machine wakes after a command expires and estimates that it can still start work.

**Fix.** Assert which clock the production helper requests and inject a failed clock read to verify that it leaves the session without an expiry bound. Run the macOS sleep check at the integrated command entry point.

## Checks of the changes added after plan review

The round 4 register's named changes are in later items. The freeze amendment explicitly assigns the sequence ceiling, enrollment host name and refusal handling, and stream refusal fallback to items 1b, 2 and 3. I used the component scenarios below and the applicable parts of the combined check table. [Round 4 register](round-4-findings.md#L3) · [Outbox amendment](../plan.md#L94) · [Enrollment amendment](../plan.md#L113) · [Stream amendment](../plan.md#L161) · [Combined checks](../plan.md#L308)

| Scenario | Observation from this review |
|---|---|
| Outbox last legal number and first refusal | The store appended 9,223,372,036,854,775,807, refused the next number without changing the outbox, and still read and acknowledged the final event after reopening. The store remained exhausted on the next reopen. |
| Host name value rules | The outgoing host name omitted zero bytes, remained valid Unicode Transformation Format, 8-bit (UTF-8), and kept whole characters within 255 bytes. The suite also exercised malformed UTF-8. |
| Enrollment status table | Both specified statuses stopped without metadata. An unfamiliar permission denial reason returned `token_refused`. A lost reply retried with the durable key. The builder recorded that the frozen suite omitted the planned unary gRPC case. A separate scratch probe confirmed that gRPC trailers return the fixed `token_used` diagnostic without echoing the fixture credentials. Finding post-b 3 covers responses outside the frozen cases. |
| Stream status table and read bound | Before a verified challenge and after an invalid signature, the component returned no terminal refusal. After a verified challenge, the frozen tests confirmed removal and revision refusals, and the retry cases covered an event gap and missing or unfamiliar reasons. An oversized challenge yielded no proof and the largest planned welcome fit. Finding post-b 1 covers a leading header that the frozen cases omit. |
| Blocked send and elapsed clock | A scratch test held the request-body reader until cancellation. The handshake returned `context.DeadlineExceeded` in ten runs with the race detector. The pinned tree cross-compiled for macOS and `go mod tidy -diff` produced no changes. The actual macOS sleep and subsequent command expiry require the integrated client on macOS. |

The pinned tree has no `src/linktest/` package. The real-server and PostgreSQL scenarios, Machines page, installed service diagnostics, and the integrated removal-notice and immediate-acknowledgement scenarios still require later items. This review observed the handshake portion of refusal and the local outbox boundary. The recovery of an outbox from an earlier copy and two live copies of one machine are outside wave 1. [Combined checks](../plan.md#L308)

## Trust, key durability and file handling

A Hypertext Transfer Protocol Secure (HTTPS) scratch enrollment confirmed that the wrong certificate key caused zero handler calls. The correct certificate followed by the wrong signing key produced `server_key_mismatch` and left the enrollment absent. With both keys correct, enrollment succeeded. On plain loopback, the frozen Join reply supplies public key bytes for fingerprint comparison. Possession of the corresponding signing key is first proven by the challenge on `Open`. The frozen stream tests observed zero machine proofs for wrong and stale server signatures. [Transport certificate check](../../../../src/client/internal/transport/transport.go#L198) · [Enrollment fingerprint check](../../../../src/client/internal/enroll/join.go#L218) · [Frozen Join reply](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L215) · [Stream proof gate](../../../../src/client/internal/link/conn.go#L101)

The enrollment file loader uses a bounded read, closes its descriptor, rejects symbolic links and checks regular-file type, ownership and private permissions. The tests covered a failed directory sync before the request and a failed identity sync after placement. The transient and refusal paths return fixed diagnostics in the ordinary test runs. Finding post-b 2 covers the separate HTTP frame log path. [Identity loader](../../../../src/client/internal/enroll/join.go#L87) · [Identity publication](../../../../src/client/internal/enroll/join.go#L146) · [Key durability](../../../../src/client/internal/enroll/join.go#L272) · [Sync tests](../../../../src/client/internal/enroll/join_test.go#L1003)

## Pinned test results and mutation checks

The commands ran from an archive of `7dd7cbb8`. The sandbox maps the root directory to an untrusted user, so I reran the state-dependent tests with host ownership and the real file checks enabled. The pinned enrollment test repair uses an HTTPS origin for the certificate-pin persistence fixture and adds explicit rejection of a pin with plain HTTP. All three test files match the recorded freeze manifest at the review pin. [Enrollment test repair](../../../../src/client/internal/enroll/join_test.go#L323)

| Acceptance command | Output |
|---|---|
| `go test -race -count=1 ./src/client/internal/link -run 'TestAckRefusesAhead\|TestAppendRefusesWhenFull'` | `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 1.081s` |
| `go test -count=1 ./src/client/internal/enroll` | `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll 0.487s` |
| `go test -count=1 ./src/client/internal/link -run 'TestOpen'` | `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 2.036s` |
| `go test -race -count=1 ./src/client/internal/link -run 'TestOpen'` | `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 7.513s` |

| Change in an isolated scratch copy | Observed result with the race detector |
|---|---|
| Reject the last legal outbox number. | The boundary test failed when the final legal append was refused. [Assertion](../../../../src/client/internal/link/outbox_test.go#L147) |
| Allow the first illegal outbox number. | The boundary test failed when the append after reopening succeeded. [Assertion](../../../../src/client/internal/link/outbox_test.go#L163) |
| Skip the enrollment reply digest comparison. | The bad-reply test failed when enrollment accepted a different key. [Assertion](../../../../src/client/internal/enroll/join_test.go#L355) |
| Use a fresh machine key after an unavailable response. | The retry test failed when the two public keys differed. [Assertion](../../../../src/client/internal/enroll/join_test.go#L451) |
| Swap the nonces in the machine proof. | The proof fixture failed during stream opening. [Assertion](../../../../src/client/internal/link/conn_test.go#L427) |
| Accept an invalid server signature. | The wrong-proof test failed because a proof was sent. [Assertion](../../../../src/client/internal/link/conn_test.go#L681) |
| Read only the leading headers for the refusal reason. | The verified removal test failed because the real reason arrived with the error. [Assertion](../../../../src/client/internal/link/conn_test.go#L879) |
| Replace the Linux suspend-inclusive clock with the ordinary monotonic clock. | All `TestOpen` tests passed. See post-b 5. |

## Grade

**6 out of 10.** The sequence boundary, durable enrollment and mutually verified proof pass their component tests. The refusal classifier, welcome validation and frame logging need revision.
