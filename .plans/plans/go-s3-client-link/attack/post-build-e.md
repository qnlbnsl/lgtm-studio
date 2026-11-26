# REVISE

A client can close a responsive stream for silence when local reporting delays delivery of received frames.

The builder was a worker of my own vendor, so the requirement for a reviewer from another vendor was not met.

Task `postbuild-go-s3-client-link-e`, nonce `fbdffc`. I reviewed items 8 and 9 and the three assigned reconnect repairs on 2026-11-26. I used the committed range `9513088f..520f8ffb` on `sprint-0`. The accepted restart can cancel the request before the result reaches the peer. A builder already owns that fault. [Assigned review](../../../../.foundry/go-rewrite/tasks/postbuild-go-s3-client-link-e.md) · [Known restart fault](../../../../.foundry/go-rewrite/results/s3-link-tests-j.md#L50)

## Findings

### post-e 1 | MAJOR | A busy session can treat a responsive peer as silent

Plan item 7b, decision 16 (D16) and check 9 (C9) call for ending the stream after 30 seconds without receiving a frame. The receiver updates the deadline for one frame, then blocks handing it to the session owner through an unbuffered channel. While the owner captures facts, the receiver cannot consume a second frame from the transport. The timer can then cancel the request with a newer frame waiting in the transport. [Session plan](../plan.md#L212) · [Frozen silence rule](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L138) · [Receive and handoff](../../../../src/client/internal/link/session.go#L402) · [Facts setup](../../../../src/client/internal/link/session.go#L467)

Confirmed live with the in-memory Hypertext Transfer Protocol version 2 (HTTP/2) peer and a held approval facts read. The peer completed sends of heartbeat acknowledgements at stream ages 1 and 29 seconds. At 31 seconds the client closed the stream, two seconds after the second send. A slow local operation can therefore cause a reconnect while the peer is still replying. Keep wire receipt and deadline updates independent of local reporting. If buffering is used, bound it and give overflow its own explicit outcome. Add the two-frame case alongside the existing silent-peer test. [Existing silence test](../../../../src/client/internal/link/session_timing_regression_test.go#L16)

### post-e 2 | MAJOR | The join command cannot be interrupted while it waits for a token

Plan item 9 requires a context canceled by interrupt or termination. The command registers for both signals, then calls the synchronous standard input reader before the enrollment code can see the context. The reader has no cancellation path. [Command plan](../plan.md#L289) · [Signal registration](../../../../src/client/cmd/lgtm-client/main.go#L21) · [Token read](../../../../src/client/cmd/lgtm-client/main.go#L43) · [Blocking read](../../../../src/client/cmd/lgtm-client/main.go#L62)

Confirmed live with the built binary. Start `lgtm-client join <address> -` with an open, empty input pipe and send an interrupt or termination signal. In both cases the process was still running 750 milliseconds later. Closing the writer let it exit with status 2 and print usage. Make the real command's input read interruptible and test that the subprocess exits while the pipe remains open. [Existing subprocess cases](../../../../src/client/cmd/lgtm-client/link_test.go#L427)

### post-e 3 | MINOR | A frame rejected after local setup can reset the reconnect backoff

Plan item 7a requires 30 seconds of a valid stream before resetting the backoff. The receive worker records when a transport error occurs. A decoded but disallowed frame carries no timestamp to the owner, and rejection takes the time when the owner exits the session. [Backoff plan](../plan.md#L201) · [Reply receipt](../../../../src/client/internal/link/session.go#L419) · [End timestamp](../../../../src/client/internal/link/session.go#L377) · [Invalid frame](../../../../src/client/internal/link/session.go#L619) · [Backoff reset](../../../../src/client/internal/link/retry.go#L46)

Confirmed live with the fake clock. The second attempt received a valid `Welcome` and entered held facts setup. The peer sent a second, disallowed `Welcome` when the stream was 29 seconds old. I released the facts read two seconds later. After an immediate third-attempt failure, the fourth attempt started at 33 seconds from the first dial. Keeping the old two-second backoff would start it at 34 seconds. Carry each reply's receive timestamp to the owner and use it when the frame itself ends the stream. Extend the frozen teardown test with a protocol rejection delayed behind local work. [Existing teardown test](../../../../src/client/internal/link/session_timing_regression_test.go#L74)

### post-e 4 | MINOR | An already oversized token can keep the input read waiting

Plan item 9 and acceptance row 1d require a usage error for a token line over 4,096 bytes. The reader allows 4,098 bytes for a possible carriage return and line feed. It waits for a line feed, end of input or all 4,098 bytes before checking the token length. At 4,097 ordinary bytes the input is already too long. [Acceptance row 1d](../plan.md#L71) · [Command plan](../plan.md#L290) · [Read limit](../../../../src/client/cmd/lgtm-client/main.go#L60)

Confirmed live with the built binary. The command kept waiting after 4,097 ordinary bytes from an open pipe. One more byte made it exit with status 2. The frozen open-pipe test writes 4,100 bytes. The entire committed client suite also passed when I lowered the command's limit to 2,048 bytes. A scratch probe with a valid 4,096-byte token passed with the committed command and failed with exit 2 under the mutation. The probe covered a line feed, a carriage return plus line feed, and end of input. Reject at byte 4,097 when that byte cannot be the start of the line ending. Add that open-pipe case and a valid token at the full limit to the frozen checks. [Oversize fixtures](../../../../src/client/cmd/lgtm-client/link_test.go#L474)

## Checks added after plan review

- Confirmed live at the application entry point. Sequence exhaustion and impossible marks in both `Welcome` and a live acknowledgement returned status 0. Each preserved `client.db`, released the lock and emitted one error with the exact recovery sentence and the expected `sequence_exhausted` or `impossible_ack` code. Retryable store faults and the logged invalid zero outcome returned status 1. [Round 4 register](round-4-findings.md#L3) · [Application assertions](../../../../src/client/internal/app/client_test.go#L670)
- Confirmed live at the command entry point. Both join forms reached enrollment with the same fixture credentials. The positional form proceeded with standard input held open. Empty and oversized input produced fixed usage and no enrollment. During a live standard input join the token was absent from the Linux process name and argument list. A scratch refusal probe put the fixture token and secret in the peer's error text. The standard input command returned status 1 and logged one fixed `join_refused/token_used` record with no fixture credential in either output stream. The application test also inspected the returned error text, and the enrollment frame-debug test passed. [Input and process assertions](../../../../src/client/cmd/lgtm-client/link_test.go#L427) · [Application error redaction](../../../../src/client/internal/app/client_test.go#L532) · [Frame-debug guard test](../../../../src/client/internal/enroll/join_frame_log_test.go#L15)
- Confirmed live against the three post-d regressions. The canceled Hypertext Transfer Protocol Secure (HTTPS) dial retired before the next one. A silent stream was canceled 30 seconds after `Welcome` or a later approval frame while the facts read was held. A stream that broke at 29 seconds retained its backoff after two seconds of sender teardown. The two additional timing cases above failed against the same snapshot. [Dial retirement](../../../../src/client/internal/link/https_dial_retirement_test.go#L18) · [Session timing](../../../../src/client/internal/link/session_timing_regression_test.go#L16)
- Confirmed live in the link fixtures. Busy scan expiry retained its stored receipt behavior. A removal notice alone received a done result and a waiting heartbeat. An ordinary break caused another attempt, and a verified removal refusal ended the link. Event-gap replay resumed at the new welcome mark plus one. The immediate acknowledgement committed before send returned. The retry waited for the retired receiver and replayed the second event. [Busy scan expiry](../../../../src/client/internal/link/commands_test.go#L268) · [Removal and gap scenarios](../../../../src/client/internal/link/session_test.go#L219) · [Immediate acknowledgement](../../../../src/client/internal/link/session_test.go#L678)

## Application lifecycle and command entry points

By reading, one state lock spans enrollment through store close and identity publication. A scratch probe held the peer reply and confirmed that another opener received the lock error and a concurrent run returned status 1. The folder could be reopened after enrollment returned. In the item 8 suite, retry reused the saved key. Store creation or close failure left the enrollment unpublished. The run path rejected missing or invalid enrollment, an unsynced directory and a mismatched machine key before dialing. Cancellation and verified removal released the lock. [Application tests](../../../../src/client/internal/app/client_test.go#L304) · [Store-before-identity implementation](../../../../src/client/internal/app/client.go#L35) · [Run implementation](../../../../src/client/internal/app/client.go#L72)

By reading, the in-process test entry exports the three planned names and delegates to the shared app with an explicit absolute state directory. Its external test ran successfully. The builder documented the optional trailing reader on `run` as the compatibility choice for the frozen call sites. The executable passes standard input explicitly. [Test entry](../../../../src/client/clienttest/clienttest.go#L13) · [External test](../../../../src/client/clienttest/clienttest_test.go#L112) · [Builder result](../../../../.foundry/go-rewrite/results/s3-link-build-g.md#L27)

## Mutations rejected by the frozen tests

Each mutation ran in a scratch overlay and exited 1 at the named assertion. The lowered input limit was the escaping mutation in post-e 4.

| Mutation | Observation |
|---|---|
| Dial before enrollment verification. | `TestAppRunRequiresEnrollment` reported that the link started with an untrusted identity. [Assertion](../../../../src/client/internal/app/client_test.go#L422) |
| Release the folder before calling the link. | `TestAppRunClosesState` could take the lock while the link worker was still running. [Assertion](../../../../src/client/internal/app/client_test.go#L497) |
| Echo arguments in usage. | `TestRunCommandExitStatus` detected the fixture token in command output. [Assertion](../../../../src/client/cmd/lgtm-client/link_test.go#L581) |
| Return 0 for an accepted restart. | `TestRunCommandExitStatus` received 0 and expected 75. [Assertion](../../../../src/client/cmd/lgtm-client/link_test.go#L723) |
| Suppress the impossible acknowledgement diagnostic. | `TestAppRunStoreStatus` counted zero records and expected one for both marks. [Assertion](../../../../src/client/internal/app/client_test.go#L745) |
| Let attempt retirement return during the old dial. | `TestRetryWaitsForCanceledHTTPSDial` observed an overlapping replacement dial. [Assertion](../../../../src/client/internal/link/https_dial_retirement_test.go#L94) |
| Restore the timer that starts after report setup. | Both cases of `TestSessionSilenceExpiresDuringApprovalSetup` missed cancellation at 30 seconds. [Assertion](../../../../src/client/internal/link/session_timing_regression_test.go#L62) |
| Take the end timestamp after joining the workers. | `TestRetryTeardownDoesNotCountTowardStableStream` saw the fourth attempt at 33 seconds and expected 34. [Assertion](../../../../src/client/internal/link/session_timing_regression_test.go#L124) |

## Acceptance commands and environment limits

The deployed check 10 (C10) exercises both refusal paths through the stack. Go 1.26.0 compiled the tests. The test executables ran as a regular user in disposable Linux containers with ordinary root-directory ownership. This kept the original state-folder validator active. I disabled version-control stamping in the archived snapshot.

| Command | Output |
|---|---|
| `go test -race -count=1 ./src/client/internal/app` | `ok` for `internal/app` in 7.281 seconds. |
| `go test -race -count=1 ./src/client/cmd/lgtm-client ./src/client/clienttest` | `ok` for the command in 11.131 seconds and for `clienttest` in 2.746 seconds. |
| `go test -race -count=1 ./src/client/internal/link -run 'TestRetry\|TestCommand\|TestDecline\|TestFinishPrunesByAge'` | `ok` for `internal/link` in 6.407 seconds. |
| `go test -race -count=1 ./src/client/...` | Exit 0. All ten packages printed `ok`. The link package took 20.287 seconds. |
| `go test -race -count=200 ./src/client/internal/link -run '^TestSessionAcceptsImmediateAck$'` | `ok` for `internal/link` in 29.072 seconds. |
| Build with `-ldflags '-X main.version=0.0.0-test'` and run `version`. | Exit 0 with `0.0.0-test` on standard output. |
| `go test -race -count=1 -v ./src/linktest/...` | Exit 1 at setup. The combined package directory was absent from the pinned commit. |
| `deploy/test/checks.sh --only C10` | Exit 1 with `not ok 10 - C10 The approved machine did not come online on WatchClients.` |

The isolated stack started and the fixture joined. A diagnostic rerun saw the machine go from waiting for approval to offline, and the client logged `link_disconnected/unimplemented`. By reading the pinned server source, `Open` and its steady loop still return `unimplemented` at lines 16 through 24 of `520f8ffb:src/server/internal/link/stream.go`. The deployed check could not reach its refusal assertions. The command fixtures did observe both refusal codes. [Deployed prerequisite](../../../../deploy/test/checks.sh#L332) · [Command refusal assertions](../../../../src/client/cmd/lgtm-client/link_test.go#L608)

The installed-service journal, service-manager retry timing and macOS suspend checks remain unverified. The Linux environment could not connect to a user service manager. The command suite built Linux and macOS binaries for both architectures, and it ran the native Linux binary. [Plan environment checks](../plan.md#L306) · [Cross-build test](../../../../src/client/cmd/lgtm-client/build_test.go#L30)

## Grade

**7 out of 10.** The enrollment lifecycle, credential redaction and committed regression cases passed. The client can still cut a responsive stream short, and a join waiting for input can ignore both shutdown signals.
