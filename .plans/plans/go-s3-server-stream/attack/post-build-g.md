# REVISE

An admission panic can leave a candidate stream handle in memory and strand a committed connection row.

Review `postbuild-go-s3-server-stream-g`, nonce `edef5c`, covers the server diff from `3df1afb1` through `f5ba7347` on `sprint-0`. That diff contains the three checks and repairs from issue 1668 and the sign-out test repair from issue 1672. The folded register points to checks P7 and P11 for startup and authorization. Check P8 covers shutdown and interrupted admissions. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s3-server-stream-g.md) · [Issue 1668](../../../../docs/issues-closed.md#L16913) · [Issue 1672](../../../../docs/issues-closed.md#L16937) · [Folded register](round-1-findings.md#L85) · [Post-build checks](../plan.md#L338)

## Findings

### post-g 1 · MINOR · A panicked Open keeps its candidate in the hub

**Statement.** Plan item 2 calls for unregistering the exact handle and dropping the machine's in-memory entry once its final candidate and gate user leave. [Plan item 2](../plan.md#L99)

**Evidence.** The hub defers clearing its admission marker and leaving the gate. The Open handler calls `Unregister` only on the ordinary return path after the handshake. The hub keeps the machine entry while any candidates remain in its handle map. [Admission cleanup](../../../../src/server/internal/link/hub.go#L99) · [Idle entry removal](../../../../src/server/internal/link/hub.go#L93) · [Open cleanup](../../../../src/server/internal/link/handshake.go#L115) · [Candidate registration](../../../../src/server/internal/link/handshake.go#L218)

**Failure scenario.** I added a scratch assertion to the frozen panic test immediately after the replacement Open ended. The hub still held two handles. The original live stream accounted for one. The leaked handle also retains its request context, and ordinary reconnects and disconnects leave that handle in the map. The frozen test checks the surviving stream's command gate and subsequent reconnection, so it accepts this leak. [Frozen panic and gate assertions](../../../../src/server/internal/link/review_c_admission_panic_test.go#L41) · [Handle fields and unregister](../../../../src/server/internal/link/hub.go#L28) · [Unregister logic](../../../../src/server/internal/link/hub.go#L178)

**Fix.** Register stream teardown as soon as Open starts owning resources. During unwind, cancel the stream's lifetime subscription, close and join its reader, and unregister its exact handle. Assert after the panic that the failed candidate is gone. [Stream lifetime subscription](../../../../src/server/internal/link/run.go#L47) · [Open entry](../../../../src/server/internal/link/handshake.go#L81)

### post-g 2 · MINOR · A panic after commit bypasses exact-stream cleanup

**Statement.** Plan item 2 and P8 require cleanup of the exact candidate when admission may have committed without a Welcome. [Plan item 2](../plan.md#L103) · [Plan P8](../plan.md#L348)

**Evidence.** The handshake sets its cleanup flag after `BecomeCurrent` returns. A panic from the commit bypasses that assignment and the later call that queues the disconnect. The retry worker accepts exact stream identifiers from that queue. [Admission callback](../../../../src/server/internal/link/handshake.go#L220) · [Disconnect queue call](../../../../src/server/internal/link/handshake.go#L123) · [Cleanup worker](../../../../src/server/internal/link/run.go#L178)

**Failure scenario.** In a scratch probe, a PostgreSQL driver tracer panicked after it received a successful `COMMIT` for the admission. In two runs an independent database connection saw generation 1, the client received no Welcome, and the connection row remained after three seconds with the cleanup worker running. The frozen panic test injects its fault before sending the commit, so it covers a rollback. [Commit call](../../../../src/server/internal/store/pool.go#L332) · [Frozen panic injection](../../../../src/server/internal/link/review_c_admission_panic_test.go#L17)

**Fix.** Record the candidate's exact cleanup obligation before entering the commit window and clear it on a confirmed rollback. On panic with an uncertain outcome, fence the previous holder and queue cleanup of the candidate during unwind. Add a post-commit fault check that observes the candidate row disappear and verifies that a later replacement keeps its row and event mark. [Existing unknown-outcome fence](../../../../src/server/internal/link/hub.go#L119) · [Exact-stream delete](../../../../src/server/internal/link/presence.go#L161) · [Lost-reply test](../../../../src/server/internal/link/handshake_test.go#L1118)

## Startup, shutdown, admission and sign-out checks

| Scenario | Evidence |
|---|---|
| P7 and issue 1668, startup order. | While PostgreSQL held deletion of the stale connection, a connection probe received a refusal. After release, startup deleted the row and the listener opened. The partial-cleanup test verified the fixed diagnostic, withheld readiness, a released instance lock and preserved marks and generations after restart. The receipt check also exercised ledger refusal with the previous migration set and replayed the saved receipt. [Listener test](../../../../src/server/internal/app/review_c_startup_test.go#L41) · [Partial cleanup](../../../../src/server/internal/app/link_stream_test.go#L256) · [Receipt replay and older ledger](../../../../src/server/internal/app/link_stream_test.go#L444) |
| P8 and issue 1668, shutdown. | Pre-admission, admitted and live streams received `unavailable` while another cleanup was held. A separate test held a proved request past the five-second drain and observed the stream close diagnostic before the instance unlock and Store close. The outer request wrapper also holds its read lock through the handler's return. The builder records that a request that ignores cancellation keeps both resources held until it returns. [Shutdown status](../../../../src/server/internal/app/link_stream_test.go#L330) · [Drain test](../../../../src/server/internal/app/review_c_drain_test.go#L25) · [Handler wrapper](../../../../src/server/internal/app/start.go#L291) · [Join and release order](../../../../src/server/internal/app/start.go#L336) · [Builder result](../../../../.foundry/go-rewrite/results/s3-small-build-n.md) |
| P8 and issue 1668, admission. | The before-commit panic cleared the marker so the previous stream regained its command gate and the next admission got the next generation. A scratch variant ran the command loop on the original stream and delivered a queued restart there in both runs. The ordinary lost-reply tests covered both deleting the uncertain row and preserving a replacement's row and mark. The two panic cleanup gaps are recorded above. [Frozen panic test](../../../../src/server/internal/link/review_c_admission_panic_test.go#L14) · [Lost-reply test](../../../../src/server/internal/link/handshake_test.go#L1118) |
| P11 and issue 1672, sign-out on the direct listener. | The test waits for a save with a connection time before signing out, then commits a report and receives `unauthenticated` without a further record. The original test passed five of eight runs and failed three at the assertion after sign-out. The repaired test passed eight of eight. An unsigned watch and a watch with a disallowed origin were refused in the direct-listener tests. [Sign-out test](../../../../src/server/internal/app/link_stream_test.go#L401) · [Unsigned watch](../../../../src/server/internal/app/link_stream_test.go#L122) |

The old test could stop at a save projected before Welcome. A watch that projects before Welcome requests another save after Welcome finishes. The connection time appears only once the hub holds the stream, so the repaired test waits for the correct observation. These checks used the in-process server listener and the previous migration set. The deployed Caddy path and the historical server binary remain unverified in this review. [Welcome completion](../../../../src/server/internal/link/handshake.go#L239) · [Watch-triggered save](../../../../src/server/internal/link/handshake.go#L101) · [Connection time projection](../../../../src/server/internal/machines/project.go#L36)

## Scratch mutations

Each mutation ran alone against disposable PostgreSQL with the Go race detector. Each failed at the named assertion.

| Mutation | Observed failure |
|---|---|
| Open the listener before clearing stale connections, with the cleanup error path retained. | `TestStartupClearsStalePresenceBeforeListening` reported that the listener accepted a connection while deletion was blocked. [Assertion](../../../../src/server/internal/app/review_c_startup_test.go#L56) |
| Skip startup cleanup. | `TestStreamStartClearsStalePresence` reported that the first watch did not show the retained machine offline with its saved last contact. [Assertion](../../../../src/server/internal/app/link_stream_test.go#L230) |
| Ignore a failed startup cleanup. | `TestPartialLinkCleanupRefusesReadiness` reported that the listener started after the second deletion failed. [Assertion](../../../../src/server/internal/app/link_stream_test.go#L282) |
| Omit the handler join after the drain. | `TestShutdownWaitsForHandlerAfterDrainTimeout` printed that the instance lock and Store were both released early. [Assertion](../../../../src/server/internal/app/review_c_drain_test.go#L104) |
| Detach the link service from the server lifetime. | `TestShutdownEndsLinkStreams` got `internal` where it expected the fixed `unavailable` response. [Assertion](../../../../src/server/internal/app/link_stream_test.go#L390) |
| Restore the old hub admission cleanup. | `TestAdmissionCommitPanicAllowsCommandsAfterReconnect` reported that the surviving stream's command gate stayed closed. [Assertion](../../../../src/server/internal/link/review_c_admission_panic_test.go#L52) |
| Remove the session check for each watch change. | `TestOpenReportsDoNotReachRevokedWatch` failed at the assertion after sign-out. [Assertion](../../../../src/server/internal/app/link_stream_test.go#L424) |

## Test command output

The full server gate ran with a disposable PostgreSQL instance and structured Go test output.

```sh
deploy/test/with-postgres.sh -- go test -json -race -count=1 ./src/server/...
```

The command exited 0. Thirteen packages reported `ok` and four helper packages had no test files. The vault test that changes file ownership skipped under the unprivileged account. The four changed or added tests all passed.

| Package or test | Result |
|---|---|
| `src/server/internal/app` | `ok`, 279.362 seconds. |
| `src/server/internal/link` | `ok`, 470.361 seconds. |
| `src/server/servertest` | `ok`, 66.948 seconds. |
| `TestShutdownWaitsForHandlerAfterDrainTimeout` | `PASS`, 16.23 seconds. |
| `TestStartupClearsStalePresenceBeforeListening` | `PASS`, 7.45 seconds. |
| `TestAdmissionCommitPanicAllowsCommandsAfterReconnect` | `PASS`, 3.48 seconds. |
| `TestOpenReportsDoNotReachRevokedWatch` | `PASS`, 7.21 seconds. Its separate eight-run check printed `ok` after 65.118 seconds. |

The combined checks used the unchanged `src/linktest` test binary built with `go test -race -c`. The direct attempt stopped at enrollment with `state_open_failed` because the sandbox's root directory failed the client's ownership check. The rerun used disposable PostgreSQL and executed the binary as an unprivileged user in a local container with a root-owned filesystem. It exited 0 and printed `PASS`. [State folder ownership check](../../../../src/client/internal/state/folder.go#L166)

| Combined check | Result |
|---|---|
| `TestLinkPresenceAcrossRestart` | `PASS`, 11.64 seconds. |
| `TestLinkOutboxReplayAfterServerFailure` | `PASS`, 34.04 seconds. |

## Grade

**8 out of 10.** The shutdown ordering and sign-out assertions held under the reviewed faults. The two panic cleanup gaps keep the review at REVISE.
