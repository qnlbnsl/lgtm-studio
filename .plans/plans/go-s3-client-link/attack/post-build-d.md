# REVISE

A canceled Hypertext Transfer Protocol Secure (HTTPS) dial can still be running when the client starts its replacement.

The builder was a worker of my own vendor, so this review did not meet the requirement for a reviewer from another vendor.

Task `postbuild-go-s3-client-link-d`, nonce `d991bf`. I reviewed items 7a and 7b and the six assigned repairs on `sprint-0` on 2026-11-26. Tests ran against a snapshot of `405a7fab`. The client and frozen contract matched the later branch tip when checked. The item 7a and 7b tests were unchanged from their freeze at `3921573f`.

## Findings

### post-d 1 | MAJOR | A replacement dial can overlap transport teardown

Plan item 7a and decision 16 (D16) require the client to retire an attempt before the next dial. The supervisor serializes callbacks. The production attempt calls `CloseIdleConnections` on handshake failure and then returns. Go 1.26.0's HTTPS transport runs the dial separately from the request. `getConn` can return on request cancellation while `dialConnFor` is active. `CloseIdleConnections` signals cancellation to abandoned dials and returns without joining them. [Reconnect plan](../plan.md#L201) · [Decision 16](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L41) · [Retry loop](../../../../src/client/internal/link/retry.go#L27) · [Failed attempt cleanup](../../../../src/client/internal/link/session.go#L147) · [Transport cleanup](../../../../src/client/internal/transport/transport.go#L168)

Confirmed live in a scratch probe that used the production HTTPS transport with a controlled dial function. The first request returned on cancellation. After `CloseIdleConnections` returned, the first dial observed cancellation and stayed in teardown. A second request entered its dial while the first dial was still active. The frozen blocked-dial test holds the attempt callback itself and cannot observe work detached by the transport. [Frozen attempt test](../../../../src/client/internal/link/retry_test.go#L111)

When a handshake times out after the retry target has already passed, the next attempt can open a connection while the prior dial unwinds. Bind in-flight transport dials to the attempt and await their exit before another dial is allowed. Exercise the production HTTPS path with a cancellation that holds the dial in teardown after the request has returned.

### post-d 2 | MAJOR | Report setup extends the first silence window

Plan item 7b and check 9 (C9) require the client to reconnect after 30 seconds of silence on an established stream. The client receives `Welcome`, then reconciles the outbox and synchronously captures and flushes reports before arming the silence timer. [Session plan](../plan.md#L212) · [Frozen silence rule](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L138) · [Check 9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L386) · [Welcome and replay setup](../../../../src/client/internal/link/session.go#L159) · [Report setup and timer](../../../../src/client/internal/link/session.go#L446)

Confirmed live with a fake clock and a signed, approved `Welcome`. The first facts read took 20 seconds and the peer sent nothing further. The stream was still open 30 seconds after `Welcome`. It closed at 50 seconds. The regular silence test uses immediate report setup. [Frozen silence test](../../../../src/client/internal/link/session_test.go#L395)

A slow report read or local database write can postpone cancellation of a silent peer and delay recovery. Measure the deadline from receipt of `Welcome` and then from the latest received frame. Let expiry cancel the transport while local reporting work is pending. Add a case that delays approval setup and expects cancellation 30 seconds after the last frame.

### post-d 3 | MINOR | The frozen tests let teardown count toward a stable stream

Plan item 7a requires 30 seconds of a valid stream before backoff resets. The built code uses the time it detected the stream ending. The frozen timing test returns a plain error at the same instant as the simulated stream failure. [Reconnect plan](../plan.md#L201) · [Stream-end timestamp](../../../../src/client/internal/link/session.go#L380) · [Backoff reset](../../../../src/client/internal/link/retry.go#L46) · [Frozen timing fixture](../../../../src/client/internal/link/retry_test.go#L52)

Confirmed live. A scratch mutation that measured through the end of teardown passed the entire frozen link package under the race detector in 19.863 seconds. A separate check kept a stream live for 29 seconds and then held teardown for two seconds. The original retained its two-second backoff and started the fourth attempt at 34 seconds. The mutant reset to one second and started that attempt at 33 seconds.

A brief connection could erase the backoff merely because a worker took time to retire. Add the delayed teardown case to the cadence test and assert the next disconnected attempt uses the preexisting backoff.

## Scenarios added after plan review

The round 4 register calls out stop diagnostics, the acknowledgement ceiling, busy scan expiry, and immediate acknowledgement. The plan also adds retry status cases, the removal sequence and replay after a gap. The following observations are from this review. [Round 4 register](round-4-findings.md#L3) · [Combined checks](../plan.md#L308)

| Scenario | Observation |
|---|---|
| Retry code table and stable-stream boundary | Confirmed live. The table included `out_of_range` and missing and unfamiliar `failed_precondition` reasons. Retriable statuses redialed. The callback tests checked starts at the 1, 2, 4, 8 and 15-second targets, retirement after cancellation at 14 seconds and the 29 versus 30-second reset boundary. The scratch tests distinguished stream time from teardown time and exposed the production HTTPS overlap. [Code table test](../../../../src/client/internal/link/retry_test.go#L405) · [Timing test](../../../../src/client/internal/link/retry_test.go#L41) |
| Removal notice and refusal | Confirmed live at `Run`. Repeated wrong signatures yielded no proof and one `server_key_mismatch` diagnostic. A refusal before any challenge redialed. A removal notice got a done result and a bare waiting heartbeat. An ordinary break after the notice redialed. `unknown_machine` after a verified challenge stopped retries before and after `Welcome` and produced one removal diagnostic. [Contract check 30](../../go-s0-contract/plan.md#L909) · [Removal tests](../../../../src/client/internal/link/session_test.go#L219) |
| Immediate acknowledgement, retired workers and replay | Confirmed live. The acknowledgement of event 1 committed locally while its send was held. The client waited for the old receiver, discarded its delayed reply and replayed unacknowledged event 2. The test passed 200 runs under the race detector. A separate scratch fixture blocked the transmit worker and observed cancellation on silence, no concurrent stream and a new dial after release. The event-gap fixture resumed at the new welcome mark plus one in both Connect and gRPC Remote Procedure Calls (gRPC). [Immediate acknowledgement](../../../../src/client/internal/link/session_test.go#L678) · [Gap replay](../../../../src/client/internal/link/session_test.go#L312) |
| Exhaustion and impossible acknowledgements | Confirmed live at `Run`. Sequence exhaustion and both impossible marks returned `RunStopped` with a nil error. Each left the store byte-for-byte intact, emitted one exact `link_stopped` record and caused zero subsequent dials. Damaged and failed stores returned errors. An open failure dialed zero times. [Stop diagnostics and preservation](../../../../src/client/internal/link/session_test.go#L472) |
| Approval, outage and busy scan expiry | Confirmed live. Approval released the waiting queue and added facts and inventory. Approved heartbeats carried load. The fixture buffered three facts at 10-second intervals during the outage and replayed them in order before its fresh reports. A saved expiry required no new write while busy. A newly expired scan wrote a receipt and kept the stream. [Reporting test](../../../../src/client/internal/link/session_test.go#L352) · [Busy scan test](../../../../src/client/internal/link/commands_test.go#L268) |

### What the acknowledgement checks establish

By reading, a verified welcome supplies the initial server mark, and the store refuses a mark at or above its next unassigned number. Live acknowledgements use the current stream's cursor and cannot exceed the handoff ceiling. The store removes the acknowledged prefix in one transaction. The client has no independent evidence of server commit for an in-range mark. The frozen wire contract assigns that guarantee to the server. A client-only fixture establishes the deletion bounds, and proof of server commit before deletion requires the real server and database. [Frozen commit rule](../../../../src/contracts/proto/lgtm/link/v1/events.proto#L7) · [Replay bounds](../../../../src/client/internal/link/replay.go#L23) · [Outbox deletion](../../../../src/client/internal/link/outbox.go#L84)

The combined `src/linktest` package was absent from the snapshot. The real PostgreSQL commit checks, the installed-service journal and exit status, and deployed checks 7 through 11 are unverified in this review. The pinned command-line entry points are stubs while items 8 and 9 are built. The macOS suspend check also remains unverified. [Combined and installed checks](../plan.md#L308) · [Pinned entry point](../../../../src/client/cmd/lgtm-client/main.go#L14)

## Repairs from the earlier reviews

| Earlier finding | Result against its requested fix |
|---|---|
| [post-b 1](post-build-b.md#post-b-1--major--leading-response-metadata-can-stop-the-client) | Repaired. The classifier requires one allowlisted terminal reason from the response trailers of a wire `failed_precondition` and a verified challenge. Confirmed live for leading-only and conflicting metadata in both protocols. [Classifier](../../../../src/client/internal/link/conn.go#L204) · [Regression](../../../../src/client/internal/link/conn_validation_test.go#L13) |
| post-b 2 | Repaired. Join and Open both check the inherited frame-debug setting before sending credentials. The two subprocess tests passed with frame debugging enabled and no fixture credential found in captured standard error. [Join guard](../../../../src/client/internal/enroll/join.go#L204) · [Open guard](../../../../src/client/internal/link/conn.go#L85) · [Enrollment regression](../../../../src/client/internal/enroll/join_frame_log_test.go#L15) · [Stream regression](../../../../src/client/internal/link/conn_frame_log_test.go#L28) |
| post-b 3 | Repaired. Contract refusals require a wire error. A genuine permission denial still uses the fixed fallback when needed. Confirmed live that a plain forbidden response retries and a plain successful response stops as `invalid_reply` after one request. Peer response text stays out of the returned diagnostic. [Classifier](../../../../src/client/internal/enroll/join.go#L380) · [Response regressions](../../../../src/client/internal/enroll/join_response_test.go#L10) |
| post-b 4 | Repaired. The validator checks time, standing and token before classifying the revision. Mixed-invalid frames, a named waiting welcome and an approved welcome without a token failed validation in the tests. Unknown approval maps to waiting and discards the token. The builder documented that a syntactically valid welcome of another revision retries in `Run`, while the verified wire refusal stops it. [Validator](../../../../src/client/internal/link/conn.go#L175) · [Regressions](../../../../src/client/internal/link/conn_validation_test.go#L60) · [Builder's explanation](../../../../.foundry/go-rewrite/results/s3-link-build-f.md#L14) |
| [post-c 1](post-build-c.md#post-c-1--major--plan-items-5-and-6) | Repaired. `Run` passes the scan context through to the store, which checks it under the lock at write admission. Confirmed live that cancellation before admission kept the next number at 1, queued zero events and recorded failed. Cancellation after commit kept event 1 and recorded done. [Production binding](../../../../src/client/internal/link/session.go#L90) · [Write admission](../../../../src/client/internal/link/store.go#L391) · [Regression](../../../../src/client/internal/link/report_commit_test.go#L36) |
| post-c 3 | Repaired. The reporter returns the fixed `ErrStoreFailed` sentinel across the reporting and command boundaries. The regression confirmed that the typed failure remained and its error text and log fields omitted the state path. It observed zero success logs and zero replies. [Reporting boundary](../../../../src/client/internal/link/report.go#L212) · [Regression](../../../../src/client/internal/link/report_commit_test.go#L96) |

## Acceptance commands and mutation results

The tests used Go 1.26.0 with normal filesystem ownership. The sandbox maps the root directory to an untrusted owner, which made the initial state-folder fixtures fail before their assertions. The successful reruns used the original tests and validator.

| Command | Output |
|---|---|
| `go test -race -count=1 ./src/client/internal/link -run 'TestRetry'` | `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 3.248s` |
| `go test -race -count=1 ./src/client/internal/link` | `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 19.332s` |
| `go test -race -count=200 ./src/client/internal/link -run '^TestSessionAcceptsImmediateAck$'` | `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 18.401s` |
| `go test -race -count=1 ./src/client/internal/enroll` | `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll 3.422s` |
| `go test -race -count=1 -v ./src/linktest/...` | `FAIL` at setup because the package directory was absent. |

All six planned mutations exited 1 on the following assertions. Each ran against a separate scratch copy.

| Mutation | Assertion |
|---|---|
| Wait the whole interval after a failed attempt. | The second attempt started at 1.4 seconds, with 1 second expected. [Assertion](../../../../src/client/internal/link/retry_test.go#L95) |
| Redial on cancellation before the previous callback returns. | A new attempt overlapped teardown. [Assertion](../../../../src/client/internal/link/retry_test.go#L148) |
| Stop on a removal notice. | The stream closed before the expected done result or heartbeat. [Assertion](../../../../src/client/internal/link/session_test.go#L255) |
| Stop on an event gap. | Both protocols missed the next connection. [Assertion](../../../../src/client/internal/link/session_test.go#L328) |
| Send saved events while waiting. | The client sent a waiting event, making the test's impossible acknowledgement admissible, and missed the expected local stop. [Fixture](../../../../src/client/internal/link/session_test.go#L535) · [Assertion](../../../../src/client/internal/link/session_test.go#L557) |
| Omit the impossible acknowledgement diagnostic. | Both impossible mark cases logged zero errors, with one expected. [Assertion](../../../../src/client/internal/link/session_test.go#L570) |

## Grade

**7 out of 10.** The six repair regressions, the refusal gate and the client-side replay checks passed. The reconnect and silence timing defects warrant revision before accepting these items.
