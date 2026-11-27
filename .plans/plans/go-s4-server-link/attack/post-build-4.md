# SHIP

The pre-commit fault test can report a failure after the expected rollback if its fault signal and terminal reply are both ready.

The test writer, the builder and this reviewer are OpenAI workers. I reviewed Server Link item 4 on `sprint-0` at `d7c7a7e9`, with the frozen tests at `ec4bb9fe`. Task `postbuild-go-s4-server-link-4`, nonce `bfe79f`. [Worker record](../../../../.foundry/go-rewrite/run-log.md#L611) · [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s4-server-link-4.md#L12)

## post-4 1 · MINOR · The pre-commit fault test can report a false failure

The plan requires the test to prove that the incumbent remains current and that neither the warning nor the feed row commits when PostgreSQL never receives the commit command. The fixture signals the cut and returns the connection error immediately. Its caller selects between the buffered cut signal and the buffered terminal reply. Confirmed live, a scratch fixture let the terminal reply arrive before entering the existing `select` and confirmed that the cut signal was already buffered. Five of twelve runs failed with “The candidate ended before its diagnostic reached the commit boundary.” The server code was the reviewed build throughout. [Plan](../plan.md#L192) · [Select](../../../../src/server/internal/link/recovery_v2_test.go#L356) · [Pre-commit connection fault](../../../../src/server/internal/link/recovery_v2_test.go#L401)

If the test goroutine resumes after the rejected candidate's reply arrives, the `select` can choose that reply even though the cut happened first. Make the pre-commit fault wait for the existing release signal after announcing the cut, as the post-commit fault already does. This lets the test observe the cut before the candidate can return. [Post-commit connection fault](../../../../src/server/internal/link/handshake_test.go#L1119)

## post-4 2 · MINOR · The frozen tests accept a warning left set after a new run is ready at admission

The plan requires a newer ready admission to clear the warning. The frozen tests cover a ready reconnect and a distinct run that becomes ready after replay. Both passed when an overlay preserved the warning only on the admission of an unseen run that was already ready. The assembled application probe caught that mutation. [Plan](../plan.md#L189) · [Immediate readiness in the build](../../../../src/server/internal/link/presence.go#L113) · [Reconnect test](../../../../src/server/internal/link/recovery_v2_test.go#L124) · [Replay cases](../../../../src/server/internal/link/recovery_v2_test.go#L151)

With committed mark 1, a signed candidate at first event 1 raises the warning. A different run at first event 2 then receives Welcome with mark 1. Confirmed live, the build cleared the warning and the mutation left it visible in a fresh Machines snapshot. Add that distinct process to the frozen suite and require the cleared fence and saved Machines update. [Clear in the readiness write](../../../../src/server/internal/link/admission.go#L98) · [Machines contract](../../../../src/contracts/proto/lgtm/v1/clients.proto#L73)

## post-4 3 · MINOR · The frozen tests accept one machine's warning on another machine

The plan requires Machines to project the warning from each machine's own Link presence. The frozen test sends traffic from a second machine and rechecks the first machine's saved fence. It does not check the second machine's projected warning. Both frozen tests passed with an overlay that set the projected flag on every Link row whenever any machine had a warning. [Plan](../plan.md#L190) · [Unrelated traffic test](../../../../src/server/internal/link/recovery_v2_test.go#L76) · [Per-machine query in the build](../../../../src/server/internal/link/presence.go#L275)

When machine A needs recovery and machine B has an ordinary connection, the mutation puts the warning on both rows. Confirmed live, the assembled application probe read both rows in one authenticated snapshot and caught the incorrect warning on B. Extend the frozen watch test to require A's warning to be true and B's to be false in the same snapshot, then observe a saved update for B while A remains flagged. [Snapshot projection](../../../../src/server/internal/machines/watch.go#L70)

**Grade** 9 out of 10.

## Post-build scenarios

- The round 1 register names the bounded repository refresh on bulk removal as the unreviewed mechanism. Link item 6 and Projects own the staged result and the refresh. The item 4 portion ran against PostgreSQL. Authenticated removal of an offline flagged machine deleted its Link row and emitted the machine identifier on the Machines watch. The combined 257-repository cleanup and the fault and retry of a Projects report require their later handlers. [Register](round-1-findings.md#L55) · [Post-build scenarios](../plan.md#L337) · [Removal assertions](../../../../src/server/internal/link/recovery_v2_test.go#L84)
- Confirmed live, a connection cut before PostgreSQL received the diagnostic commit left the flag clear and created no saved feed row. Cutting the reply after commit left the warning and exactly one saved feed row visible to an independent database connection. Both candidates received `unavailable` without Welcome. The incumbent kept its identifier and generation and answered another heartbeat. The committed flag survived restart. [Diagnostic case](../../../../src/server/internal/link/recovery_v2_test.go#L308) · [Plan scenario](../plan.md#L345)
- Confirmed live, the assembled application probe failed the diagnostic update with a PostgreSQL exception containing a synthetic credential. The candidate received `unavailable`, the feed head and warning stayed unchanged, and the incumbent answered a heartbeat. The marker was absent from the reply and the captured application logs. The inherited malformed-run marker cases also passed on Connect and Google Remote Procedure Call (gRPC). Certificate pins, installed-client replay and private state modes belong to the Stack and paired checks. [Public error mapping](../../../../src/server/internal/link/handshake.go#L233) · [Log fields](../../../../src/server/internal/link/handshake.go#L112) · [Inherited privacy assertions](../../../../src/server/internal/link/admission_v2_test.go#L138) · [Plan scenario](../plan.md#L346)

## Item 4 acceptance

- By reading, the server authenticates the signed candidate before classification. The machine row lock covers classification and the warning write. An unseen run behind the committed mark saves the current generation and records a Machines feed reference on the transition from clear to set. A repeated refusal advances the fence when the generation has changed. The same Store transaction writes the warning and its feed reference, as required by foundation rule 4. On a diagnostic error, the candidate returns through the failed-admission branch of the hub and the incumbent stays current. The ordinary admission uncertainty case retains its existing behavior. [Proof and diagnostic mapping](../../../../src/server/internal/link/handshake.go#L205) · [Classifier](../../../../src/server/internal/link/admission.go#L35) · [Warning write](../../../../src/server/internal/link/presence.go#L53) · [Hub branches](../../../../src/server/internal/link/hub.go#L119) · [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L358)
- By reading, Machines uses Link's presence interface and the shared watch loader, as required by foundation rules 3 and 5. Link reads the warning from the durable machine row, satisfying rule 7, and the existing held-stream check gates liveness fields. The row iterator closes on all return paths. Confirmed live, waiting and offline records exposed the flag without a live connection. The flag stayed set while the incumbent sent a heartbeat, a valid report and a consumed event. [Link presence](../../../../src/server/internal/link/presence.go#L270) · [Machines projection](../../../../src/server/internal/machines/project.go#L18) · [Watch reads](../../../../src/server/internal/machines/watch.go#L70) · [Live watch test](../../../../src/server/internal/link/recovery_v2_test.go#L29)
- By reading, the readiness write clears the fence only when the current generation is strictly later. An already ready run can reconnect and clear the flag without a second readiness callback. Confirmed live, the ready reconnect and immediately ready new run cleared it. The helper-based replay cases showed that only a pending stream admitted after the most recent refusal can clear it at readiness. Staged-only, pending-only, superseded, changed-pair, malformed-pair, bad-proof and wrong-revision refusals left a clear flag clear. [Readiness update](../../../../src/server/internal/link/admission.go#L81) · [Replay cases](../../../../src/server/internal/link/recovery_v2_test.go#L151) · [Refusal controls](../../../../src/server/internal/link/recovery_v2_test.go#L240) · [Done when](../plan.md#L159)

## Mutations detected by the frozen tests

Each change was isolated in a Go overlay against the same archived commit. The four planned mutations each exited 1 on the expected assertion. Both additional mutations described in the findings exited 0 on the full frozen item 4 suite and exited 1 on the assembled application probe. The probe on the reviewed build exited 0 with `PASS` and package time `2.820s`. [Planned mutations](../plan.md#L193)

| Scratch mutation | Assertion that failed |
|---|---|
| Route the unconfirmed diagnostic commit through ordinary admission uncertainty. | `recovery_v2_test.go:375`. The refusal displaced the incumbent's held connection or generation. |
| Let incumbent readiness clear the warning at the same generation. | `recovery_v2_test.go:224`. The saved generation was -1, with 1 expected. |
| Clear the warning in the incumbent heartbeat write. | `recovery_v2_test.go:56`. The saved generation was -1, with 1 expected. |
| Project the warning only for held connections. | `recovery_v2_test.go:92`. The restarted offline snapshot showed false, with true expected. |

## PostgreSQL verification

```sh
bash deploy/test/with-postgres.sh -- go test -count=1 -p 1 ./src/server/internal/link -v -run '^(TestRecoveryWarningFollowsLaterReadyAdmission|TestRecoveryDiagnosticCommitReplyLost|TestAdmissionCommitReplyLost|TestRevisionTwoProofAndRunAdmission|TestRunAdmissionPersistsAcrossRestart)$'
bash deploy/test/with-postgres.sh -- go test -count=1 -p 1 ./src/server/internal/link ./src/server/internal/machines -v -run '^(TestHeartbeatIsAnswered|TestSilenceEndsStream|TestRemovedIdleStreamRefusesBeforeHeartbeatTimeout|TestCommandResultFrameRules|TestCommandSendRechecksBeforeWrite|TestProofTextsMatchContractExamples|TestReplayedProofRefused|TestHandshakeDecisionOrder|TestWatchClientsFirstMessageAndChanges|TestWatchClientsStreamPresence|TestWatchKeepsJoiningThroughReplacementRollback)$'
```

The first command exited 0 and printed `PASS` for all five named tests and `ok` for Link in `29.196s`. The second command exited 0 and printed `PASS` for all eleven named tests and `ok` for Link in `18.374s` and Machines in `5.960s`. Neither selection included tests listed as intentionally red. [Regression requirement](../plan.md#L213) · [Shared exclusions](../../../../.foundry/go-rewrite/common-build.md#L41)

The two frozen item 4 tests also ran with `go test -race -count=1 -p 1` through the same PostgreSQL wrapper. Both printed `PASS`. The package printed `ok` in `43.686s`, with zero race reports.

The item 5 handoff command with `LGTM_TEST_LINK_STREAM_PROMOTION=1` exited 1 as expected. `PendingIncumbentCannotClear`, `LaterPendingRunClearsAtReadiness` and `RefusalDuringReplayRefreshesFence` all printed “The stream acknowledged the final replay event before committing run readiness.” The Link package printed `FAIL` in `4.305s`. The baseline uses the plan's real locked readiness helper for these cases. [Item 5 handoff](../../../../.foundry/go-rewrite/results/s4-link-4-build.md#L65) · [Fixture helper](../../../../src/server/internal/link/recovery_v2_test.go#L446)
