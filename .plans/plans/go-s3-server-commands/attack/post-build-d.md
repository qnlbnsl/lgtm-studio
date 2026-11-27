# REVISE

The combined outage replay test failed because it counted an in-flight report as if the report had already committed.

## Findings

### post-d 1 — MAJOR — The outage replay test races against new reports at shutdown

Foundation check C11 requires buffered events to reach the server once and in order. The inherited replay test waits until it sees an acknowledgement beyond the old backlog and then stops the client. A periodic report can reach the relay before shutdown, after the acknowledgement used as the stop signal. The relay records that report before its upstream send, while the test requires every recorded send to have committed and been acknowledged. [Foundation check C11](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L388) · [Final recovery assertion](../../../../src/linktest/outbox_test.go#L170) · [Relay event observation](../../../../src/linktest/relay_test.go#L241)

Confirmed live, two PostgreSQL runs failed at line 189. The first reached durable mark 11 with sends 4 through 12 and acknowledgements 4 through 11. The second reached mark 12 with sends 4 through 13 and acknowledgements 4 through 12. That observation can occur when shutdown interrupts a new report while the recovered prefix is intact. Stop generating fixture reports before the final drain, then require that the client durably applies the last acknowledgement before shutdown. [Final recovery assertion](../../../../src/linktest/outbox_test.go#L178)

### post-d 2 — MAJOR — The application tests miss loss of the periodic sweep

Rule R10 requires tests against real PostgreSQL. Item 8 wires the command sweep into the application lifetime. The pinned runner starts and joins `RunCommandSweep`, which sweeps at startup and hourly. Both lifecycle subtests exercise the startup pass. The worker unit test drives an hourly tick by calling `RunCommandSweep` directly. [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L360) · [Item 8](../plan.md#L235) · [Runner](../../../../src/server/internal/link/run.go#L86) · [Sweep schedule](../../../../src/server/internal/link/outbox_sweep.go#L142) · [Application lifecycle test](../../../../src/server/internal/app/commands_test.go#L196) · [Worker unit test](../../../../src/server/internal/link/outbox_sweep_test.go#L237)

Confirmed live, replacing the worker call in the runner with one cancellable call to `SweepCommands` passed all five named item 8 tests, both inherited server fixture tests in the filter and all ten combined tests. A scratch probe that sends an hourly tick to the live application passed on the pinned runner and failed on the changed runner because no later sweep query arrived. An uptime of several days could then leave timed-out actions pending and old results retained. Drive the existing private tick through the application fixture, insert a due row after the startup pass, and assert that a later pass finishes the row before shutdown. [Lifecycle fixture](../../../../src/server/internal/app/commands_test.go#L196) · [Worker tick check](../../../../src/server/internal/link/outbox_sweep_test.go#L275)

### post-d 3 — MAJOR — The reconnect test accepts a result before the scan finishes

The plan and the frozen contract require the running scan to report its eventual result under its latest attempt. The combined test checks the newer stream’s inventory, releases the held executable and then searches the relay’s entire result history for a failure. The history also includes results sent before release. [Combined acceptance](../plan.md#L273) · [Result contract](../../../../src/contracts/proto/lgtm/link/v1/commands.proto#L61) · [Active scan case](../../../../src/linktest/commands_test.go#L24) · [Result lookup](../../../../src/linktest/command_fixture_test.go#L154)

Confirmed live, a scratch change immediately sent `failed` on the newer attempt while the original scan remained running. The frozen active-scan case passed. A separate observation of the same change confirmed that the relay saw the result and PostgreSQL committed it while the original executable was still blocked. In a queue with another scan waiting, early finalization frees the server’s scan reservation before the client’s first scan finishes. Assert that the delivered attempt stays pending while the probe is held, then observe exactly one result on the latest attempt after release. [Client attempt handoff](../../../../src/client/internal/link/commands.go#L200) · [Server scan reservation](../../../../src/server/internal/link/outbox_claim.go#L107)

### post-d 4 — MAJOR — The committed suite misses both assembled shutdown scenarios in P4

P4 calls for stopping the application while a command flush and a sweep query are both held. It also calls for interrupting startup after migration 2 commits and before the listener opens. The application suite stops with a dispatched restart and separately stops with a blocked startup sweep. The standalone ordering fixture holds the flush but then completes it and keeps its listener alive. The older migration checks hold the migration itself or start a migrated application to completion. [P4](../plan.md#L287) · [Dispatched restart](../../../../src/server/internal/app/commands_test.go#L85) · [Blocked startup sweep](../../../../src/server/internal/app/commands_test.go#L213) · [Held flush](../../../../src/server/internal/link/outbox_integration_test.go#L198) · [Migration budget test](../../../../src/server/internal/app/start_test.go#L500) · [Receipt preservation test](../../../../src/server/internal/app/link_stream_test.go#L438)

Confirmed live, both assembled scenarios passed in scratch. The committed tests leave a gap for a shutdown regression that only occurs when the two database operations compete, or for an interruption after the migration ledger advances but before readiness. Keep both probes as regression tests. Observe the live wait, the original shutdown cause, released database resources and the next durable attempt on restart. Verify the existing record and keyed reply in the interrupted-upgrade case. [Application release](../../../../src/server/internal/app/start.go#L314) · [Migration and prerequisites](../../../../src/server/internal/app/start.go#L176)

### post-d 5 — MINOR — The combined suite leaves the reserved command result outside the wire check

Rule R9 keeps secrets out of logs and public records. P7 and contract check 15 require a marked, reserved `CommandResult` field to traverse a proxy, commit the outcome and disappear from storage, logs and public replies. The committed result test calls the state writer directly. The combined scripted test sends a marker in an event and sends an unmarked result when a notice arrives. A regression that logs the raw incoming result before passing it to the state writer can pass both tests. [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L360) · [P7](../plan.md#L290) · [Contract check 15](../../go-s0-contract/plan.md#L894) · [Direct result test](../../../../src/server/internal/link/outbox_result_test.go#L165) · [Combined scripted frames](../../../../src/linktest/outbox_test.go#L339)

Confirmed live, sending a synthetic marker in the reserved result field through the loopback relay committed the result and cleared the payload. A malformed attempt carrying the same marker returned `invalid_argument`. The marker stayed absent from the observed database records, server log and public replies. Retain the marked-result scenario in the combined suite and exercise it through the deployed private proxy for P7. [Result dispatch](../../../../src/server/internal/link/stream.go#L166)

**Grade.** 7 out of 10.

## Post-build observations on the pinned revision

I reviewed `2d1045ed..a7d9bad8` on `sprint-0` and ran the experiments against one source archive of `a7d9bad8`. The newest pre-build register uses a dispositions table. I used its entries for the mechanisms marked `UNREVIEWED MECHANISM` in the server plan and the two combined cases marked `UNREVIEWED FIX` in the client plan. The overlay experiments used the same archive and disposable PostgreSQL databases. [Server review register](round-1-findings.md#L9) · [Server scenarios](../plan.md#L268) · [Client scenarios](../../go-s3-client-link/plan.md#L308)

| Scoped scenario | Confirmed live |
|---|---|
| Dispatch marker and sweep ordering. | The actual sweep finished an aged claim before send, the stream acknowledged a heartbeat and the claim never reached the flush. In the inverse fixture PostgreSQL refused the busy machine’s nonwaiting lock. The control row timed out in that pass and a later pass finished the first machine’s due row. The unaged replay kept the original expiry. [Held claim](../../../../src/server/internal/link/outbox_integration_test.go#L43) · [Held flush](../../../../src/server/internal/link/outbox_integration_test.go#L171) · [Unaged replay](../../../../src/server/internal/link/outbox_integration_test.go#L264) |
| Machine expiry, server timeout and removal. | The matching expiry report won as `expired` when first. The sweep won as `timed_out` when first. Removal and result each kept their first disposition in the two orders tested. Terminal payloads were cleared and late results left the first finish time unchanged. [Expiry ordering](../../../../src/server/internal/link/outbox_integration_test.go#L292) · [Removal ordering](../../../../src/server/internal/link/outbox_integration_test.go#L349) |
| Scan reservation, retries and lost results. | A restart passed a deferred scan while the active probe was held. The second scan also followed completion on the same stream in the separate variant. The active scan sent its result on the next attempt. Completed and restart replies replayed on a new connection. The aged receipt replayed at 22 hours and 30 minutes with Welcome delayed eight seconds. After the 23-hour cutoff, a later scan reached the client while the aged command stayed pending. The server’s fixed database-clock fixture also passed the exact equality check. [Retry cases](../../../../src/linktest/commands_test.go#L22) · [Scan reservation cases](../../../../src/linktest/commands_test.go#L306) · [Decision-time boundaries](../../../../src/server/internal/link/review_claim_test.go#L54) |
| Approval, duplicate standing and owner calls. | The original waiting connection refused both actions and got the approval, including notices whose expiry was five seconds or 25 hours in the past. Joining appeared before the fresh reports committed. Both reports made the original generation online and the repeated revision produced no extra report. The app’s four routes and keyed replay rejected the four denied credentials or origins and preserved the database digest. The signed-in owner queued one scan with one receipt and wake. [Approval variants](../../../../src/linktest/commands_test.go#L186) · [Owner boundary](../../../../src/server/internal/app/commands_test.go#L305) |
| Busy scan expiry and removal notice. | A saved expiry replay used no write. A fresh expired command recorded one receipt while the active probe stayed held. The active scan then completed on its current stream. A removal notice alone left heartbeats running. Ordinary disconnect led to a fresh dial, and the verified refusal stopped the client after the actual removal. [Busy scan](../../../../src/linktest/commands_test.go#L386) · [Removal notice](../../../../src/linktest/link_test.go#L205) |
| Inherited client and server checks. | Join replay reused the saved identity and waiting record. The server restart brought the client back on a newer stream. A wrong pin sent no proof. The outbox test recovered across a terminated PostgreSQL transaction and a cut after commit. Its audit recorded each report once in order and the unknown event marker stayed out of the observed records, replies and log. [Join and presence](../../../../src/linktest/link_test.go#L24) · [Wrong signing key](../../../../src/linktest/link_test.go#L155) · [Two cut points](../../../../src/linktest/outbox_test.go#L101) · [Scripted marker](../../../../src/linktest/outbox_test.go#L370) |
| Application interruption and worker recovery. | With an actual stream given no response credit and the sweep’s due query waiting in PostgreSQL, terminating the instance connection preserved the lock-loss diagnostic. The held resources cleared and restart delivered the same identifier and expiry at attempt two, which committed its matching result. A separate probe canceled the application after the migration 2 ledger committed and before listening, at the session reconciliation lock. The next start held one instance connection, ran one startup sweep and replayed the old keyed approval without creating another command. [Startup](../../../../src/server/internal/app/start.go#L162) · [Shutdown](../../../../src/server/internal/app/start.go#L314) |
| Standalone client process. | A scratch build of the client executable exited 75 once when a relay dropped its stored restart result. Relaunch on the same state replayed the result on attempt two, completed a scan and exited zero on a termination signal. The committed combined suite uses in-process application status because its assigned public fixtures have no subprocess launcher. The installed service gate belongs to Stack. [Combined restart](../../../../src/linktest/commands_test.go#L157) · [Documented handoff](../../../../src/linktest/doc.go#L4) · [Writer result](../../../../.foundry/go-rewrite/results/s3-linktest-b.md#L50) |

By reading, startup holds the instance lock through migration 2 and passes the same state to the machine command queue and link hook. The public server fixture starts and restarts through the application constructor. The sweep uses the application lifetime, joins the other workers and reports a fixed diagnostic. The assembly code matched item 8. The scope yielded no confirmed production defect in the pinned assembly. [Startup wiring](../../../../src/server/internal/app/start.go#L252) · [Fixture launch](../../../../src/server/servertest/server.go#L140) · [Worker join](../../../../src/server/internal/link/run.go#L72)

The deployed checks C5 through C11, the installed service restart, the deployed private proxy and the at-rest policy remain Stack or owner work. The near-cutoff client clock test from contract check 26 belongs to the Client team. The combined run establishes the server retry boundary and the aged replay on the normal client clock. [Rollout](../plan.md#L253) · [Clock split](../plan.md#L276) · [At-rest decision](../decisions.md#L28)

## Commands and results

The first invocation used `deploy/test/with-postgres.sh -- go test -json -race -count=1 ./src/server/... ./src/linktest/...`. It exited 1 on the inherited `TestOpenReportsDoNotReachRevokedWatch` assertion at `link_stream_test.go:419`. The first run reported no data race and every other server package with tests passed. A focused diagnostic retry passed three times. The serialized retry passed the application package in 143.038 seconds and the Link package in 325.681 seconds. I stopped that retry after those two packages passed. [Inherited assertion](../../../../src/server/internal/app/link_stream_test.go#L401)

The first `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12` invocation exited 1 and printed `not ok 12 - C12 go test ./... failed against the configured PostgreSQL.` The check suppresses Go test output and only starts the feed-lock mutation after the root tests pass. A separate diagnostic and a rerun of C12 with Go test output captured both reproduced the outbox assertion in post-d 1. The C12 rerun also reported “Could not open the independent PostgreSQL observer” before the restart test issued its restart request. The observer suppresses the underlying connection error. Both combined tests passed a focused PostgreSQL retry in 28.580 seconds. [C12 check](../../../../deploy/test/checks.sh#L636) · [Outbox assertion](../../../../src/linktest/outbox_test.go#L189) · [PostgreSQL observer](../../../../src/linktest/fixture_test.go#L295)

| Item 8 test in the first run | Result |
|---|---|
| `TestServerRoutesStoredCommands` | PASS (7.98s). |
| `TestServerCommandSweepLifecycle` | PASS (20.27s). Both subtests ran. |
| `TestServerCommandOwnerBoundary` | PASS (8.42s). All five subtests ran. |
| `TestCommandSendAndSweep` | PASS (9.87s). All five subtests ran. |
| `TestCommandResultAndRemoval` | PASS (7.65s). Both orders ran. |
| `TestLinkSeamsReachTheService` | PASS (25.10s). |

The next two runs used `go test -json -race -count=1 ./src/linktest/...` through the same PostgreSQL wrapper. All ten scenarios and their subtests passed in all three runs. The package summaries printed `ok` at 195.737, 159.657 and 161.749 seconds. Without the database variable, all ten tests explicitly skipped and Go exited zero. Formatting and vet produced no output.

| Combined scenario | Run 1 | Run 2 | Run 3 |
|---|---:|---:|---:|
| `TestLinkJoinReplay` | 8.41s | 6.87s | 5.65s |
| `TestLinkPresenceAcrossRestart` | 10.62s | 10.20s | 10.36s |
| `TestLinkKeysAndRemoval` | 9.52s | 8.50s | 6.42s |
| `TestLinkRemovalNoticeAndRefusal` | 15.34s | 14.68s | 15.40s |
| `TestLinkOutboxReplayAfterServerFailure` | 37.67s | 34.98s | 34.72s |
| `TestLinkCommandAttemptsAcrossReconnect` | 51.66s | 40.67s | 40.43s |
| `TestLinkRestartReplaysAfterDroppedResult` | 10.00s | 6.90s | 7.32s |
| `TestLinkSameStreamApproval` | 25.51s | 16.99s | 20.83s |
| `TestLinkScanDoesNotDelayRestart` | 16.23s | 12.68s | 13.25s |
| `TestLinkBusyScanExpiry` | 9.64s | 6.05s | 6.25s |

Each listed plan mutation ran separately through the PostgreSQL wrapper with an anchored filter and exited 1 at the recorded assertion. The unmodified tests passed.

| Scratch mutation | Named failure |
|---|---|
| Omit the queue. | `TestServerRoutesStoredCommands` reported “The application approval route did not persist one state notice.” [Assertion](../../../../src/server/internal/app/commands_test.go#L45) |
| Omit the live command hook. | `TestServerRoutesStoredCommands` reported “The application link did not deliver the queued state notice on the original stream.” [Assertion](../../../../src/server/internal/app/commands_test.go#L47) |
| Omit the sweep worker. | `TestServerCommandSweepLifecycle/startup_pass` reported “The application did not time out the due action on its first command sweep.” [Assertion](../../../../src/server/internal/app/commands_test.go#L202) |
| Store an explicit machine expiry as `timed_out`. | `TestCommandSendAndSweep/reported_expiry_wins_before_sweep` reported “The dispatched result did not store expired and a database finish time with no payload.” [Assertion caller](../../../../src/server/internal/link/outbox_integration_test.go#L33) |

The startup-only sweep change passed the assigned application and link filter and the combined package in 139.521 seconds. The held-shutdown probe failed on that change with “did not observe the sweep database query waiting behind the table lock within 15s.” The early scan-result change passed the frozen active-scan case in 5.60 seconds. The separate observation reported “The failed result reached the wire and committed while the first requested scan was still held.”

The scratch probe run exited zero. The application probes printed a committed migration before listening, a single sweep on restart, preservation of the instance-lock loss diagnostic and a successful attempt two. The relay probe printed the committed result and cleared payload. The executable probe printed one exit 75, a replay and a clean exit zero.
