# REVISE

The owner-route tests pass even when a restart replays a scan’s receipt and returns success without enqueuing a restart.

## Findings

### post-c 1 — MINOR — The owner-route tests miss request-key conflicts between actions

Item 7 preserves the version-one request fingerprint and maps key conflicts to `already_exists`. Foundation decision D16 makes `ReconnectClient` a restart of a connected client. [Item 7](../plan.md#L230) · [Decision D16](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L41)

By reading, the pinned handlers use distinct procedure names and the Store includes the procedure in the receipt hash. The frozen replay test creates a fresh fixture for each route and checks a different machine identifier within that route. Confirmed live, assigning the scan procedure name to `ReconnectClient` passed all three assigned Machines tests. A separate PostgreSQL probe queued a scan, then used its key to request a restart of the same online machine. The pinned code returned `already_exists`. The changed code returned success, and the database still held only the scan. [Procedure names](../../../../src/server/internal/machines/commands.go#L21) · [Receipt check](../../../../src/server/internal/store/receipts.go#L78) · [Replay fixture](../../../../src/server/internal/machines/commands_test.go#L195)

Extend the replay test to use one key on both routes for the same machine. Require `already_exists` on the second action and confirm the command count, receipt and feed position are unchanged.

### post-c 2 — MINOR — The pool test misses handoff to the oldest of two live waiters

Item 4b requires each released permit to wake the oldest live waiter. Foundation check C8 depends on the link retaining capacity while machines connect and disconnect. [Item 4b](../plan.md#L174) · [Check C8](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L385)

By reading, the pinned allocator grants from the front of its wait list. The frozen pool test cancels one waiter and releases the permit with one live waiter left. Confirmed live, granting from the back passed all nine Link tests in the assigned run. A separate probe held all three permits, queued two live waiters and released one permit. The pinned allocator woke the first waiter. The changed allocator woke the second. With a steady supply of newer waiters, the oldest could keep waiting while permits turn over. [Permit handoff](../../../../src/server/internal/link/outbox_send.go#L62) · [Frozen cancellation and release](../../../../src/server/internal/link/outbox_send_test.go#L779)

Keep two ordered live waiters after the canceled-waiter check. Release one permit and require command delivery to the first live waiter while the second remains unassigned. Release another permit and require the second to progress.

**Grade.** 8 out of 10.

## Post-build scenarios on the pinned revision

I reviewed `2d1045ed` on `sprint-0` in one disposable copy. The supplied `a5d1dfb9..2d1045ed` diff covers the writer, its fixture repairs and the exhausted-scan repair. Item 7 and the deadline repair landed before the lower bound, in `d927da91` and `522845ae` respectively. I read those changes separately. The plan uses `UNREVIEWED MECHANISM` for the relevant entries. I used those labels and the round register’s dispositions to select the scenarios. [Plan labels](../plan.md#L92) · [Round register](round-1-findings.md#L9) · [Post-build checks](../plan.md#L280)

| Check | Confirmed live |
|---|---|
| P1. Dispatch, flush and capacity. | An attempt stayed pending when its result arrived before dispatch. Removal and replacement won in the claim gap and the old stream emitted no command. Moving the cutoff into the past discarded the claim and left its result ineligible. A successful frame flushed while the writer held the machine lock. The dispatch commit became visible before the matching result finished the row and cleared its payload. After the old write deadline elapsed, the same stream acknowledged a heartbeat. [Send fence fixture](../../../../src/server/internal/link/outbox_send_test.go#L220) |
| P1. Held writes. | A loopback Hypertext Transfer Protocol version 2 (HTTP/2) peer granted no stream credit after Welcome. The callback reached the wrapped writer with a real restart and the flush failed at each prescribed deadline. An independent PostgreSQL transaction took the machine lock after disconnect cleanup. The command that remained within its retry window reached a replacement stream at attempt 2. With all three permits held, another stream acknowledged a heartbeat and disconnect cleanup committed. Releasing one permit woke the remaining live waiter after the canceled waiter left. [Deadline fixture](../../../../src/server/internal/link/outbox_send_test.go#L524) · [Pool fixture](../../../../src/server/internal/link/outbox_send_test.go#L679) |
| P2. Scan reservation, rewind and wake. | The attempt-ceiling regression kept the dispatched scan’s reservation on its original stream. A replacement stream claimed the next scan while the exhausted row stayed pending. The order tests let a restart bypass a deferred scan and revisited that scan after a result or a heartbeat past the cutoff. A scratch probe waited for a short ineligible claim page to commit and enqueued a valid restart. The feed woke the idle stream and the peer received the restart without sending another frame. The fixed-clock test checked equality and the available one-microsecond boundary cases for claim, timeout and retention. [Ceiling and clock fixtures](../../../../src/server/internal/link/review_claim_test.go#L22) · [Order fixture](../../../../src/server/internal/link/outbox_claim_test.go#L267) · [Feed wake](../../../../src/server/internal/link/run.go#L239) |
| P3. Lost claim and dispatch replies. | A scratch probe discarded the PostgreSQL reply after a claim committed. The observing connection saw attempt 1 pending without dispatch, the original stream ended with `unavailable` and no command, and its replacement received the same identifier and expiry at attempt 2. The frozen send test discarded a separate dispatch commit reply after independently seeing its marker, and the stream ended and retried at attempt 2. A failed flush also ended the stream and retried, with dispatch still unset on the first attempt. [Claim commit fixture](../../../../src/server/internal/link/outbox_claim_test.go#L124) · [Dispatch retry fixture](../../../../src/server/internal/link/outbox_send_test.go#L378) |
| Repairs from the earlier reviews. | The deadline tests returned cancellation both during the deadline clear and while waiting for the cancellation handler. The original callback and flush errors retained precedence. The scan-ceiling regression proved forward progress after replacement and preserved the current stream’s reservation. Reverting each repair separately brought its corresponding failure back. [Deadline regression](../../../../src/server/internal/transport/review_deadline_test.go#L40) · [Scan regression](../../../../src/server/internal/link/review_claim_test.go#L22) |

The remaining P3 client receipt replay and the P4 through P7 assembled application and deployment scenarios belong to the later server composition, client and Stack gates. The pending-page test exercised the page shape in this run. The due-index plans were part of the earlier sweep review. [Post-build checks](../plan.md#L284) · [Sweep review](post-build-b.md#L33)

## Implementation against the plan

By reading, item 4b acquires a shared permit before claim, yields between pages to ready frames and releases the permit after the turn or on stream exit. Its second Store write rechecks the stream and pending attempt under the machine lock. The write holds that lock through flush, commits dispatch and adds zero feed references. The scan and restart deadlines use the remaining database interval and the two-second cap. The repaired transport checks cancellation after deadline cleanup. The item 4b builder recorded three fixture problems that the pinned fixture commit addresses. [Writer turns](../../../../src/server/internal/link/stream.go#L28) · [Send fence](../../../../src/server/internal/link/outbox_send.go#L131) · [Deadline cleanup](../../../../src/server/internal/transport/deadline.go#L53) · [Builder result](../../../../.foundry/go-rewrite/results/s3-commands-build-e.md#L32)

By reading, item 7 checks approval and exact live presence inside the same locked, keyed write. Scan checks both reports and restart allows Joining. The command and receipt commit together with one wake. Replay happens before the eligibility callback. The frozen route tests confirmed those branches and the original refusal sentences, including refusal after failed disconnect cleanup. [Owner routes](../../../../src/server/internal/machines/commands.go#L20) · [Keyed write and error mapping](../../../../src/server/internal/machines/records.go#L155) · [Exact hold](../../../../src/server/internal/link/presence.go#L218) · [Owner route fixture](../../../../src/server/internal/machines/commands_test.go#L98)

## Test and mutation output

The anchored `deploy/test/with-postgres.sh -- go test -json -race -count=1 -run '<names>'` run exited zero. The JavaScript Object Notation (JSON) output contained a pass event for every listed top-level test and zero test skips.

- Link passed `TestCommandSendRechecksBeforeWrite`, `TestCommandSendDeadline`, `TestCommandBacklogYieldsToHeartbeats`, `TestCommandSendPoolHeadroom`, `TestClaimScanAfterExhaustedAttempt`, `TestClaimCommitsBeforeReturn`, `TestClaimOrder`, `TestClaimBoundsIneligiblePrefix` and `TestCommandDecisionTimeBoundaries`.
- Machines passed `TestScanAndReconnectQueueCommands`, `TestActionRequestKeyReplays` and `TestScanAndReconnectRefused`.
- Transport passed `TestSendReportsCancellationWhileClearingDeadline`, `TestSendReportsCancellationWhileJoiningDeadlineExpiry`, `TestSendPreservesCallbackErrorDuringDeadlineCleanup`, `TestSendPreservesFirstFlushErrorDuringDeadlineCleanup`, `TestSendSkipsCallbackWhenDeadlineArmingCancels`, `TestSendUsesEarlierContextDeadline` and `TestLinkSendReportsFlushFailure`.

```text
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/link        22.046s
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/machines    35.970s
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/transport    1.048s
```

Each of the following isolated changes exited 1 on the stated assertion. The two changes accepted by the frozen tests are recorded in the findings above.

| Change | Failed observation |
|---|---|
| Replace the second machine lock with a plain read and keep the stream check. | `TestCommandSendRechecksBeforeWrite` found that its competing transaction could take the machine lock before dispatch committed. [Assertion](../../../../src/server/internal/link/outbox_send_test.go#L357) |
| Give the flush the two-second cap alone and omit the shorter database cutoff. | `TestCommandSendDeadline` found the wrong armed deadline. [Assertion](../../../../src/server/internal/link/outbox_send_test.go#L561) |
| Grant the permit without signaling the waiter. | `TestCommandSendPoolHeadroom` received no command after one permit was released. [Assertion](../../../../src/server/internal/link/outbox_send_test.go#L789) |
| Remove the continuation signal. | `TestCommandBacklogYieldsToHeartbeats` observed 18 heartbeat acknowledgements and zero tail commands. [Assertion](../../../../src/server/internal/link/outbox_send_test.go#L658) |
| Ignore the facts report or trust a saved stream identifier without its live hold. | In separate runs, `TestScanAndReconnectQueueCommands` found an accepted scan and a persisted write where it expected a refusal. [Assertions](../../../../src/server/internal/machines/commands_test.go#L113) |
| Make the owner action an unkeyed write. | `TestActionRequestKeyReplays` returned before reaching the held keyed commit. [Assertion](../../../../src/server/internal/machines/commands_test.go#L215) |
| Restore the pre-repair deadline cleanup. | Both cancellation regressions failed to observe cancellation. [Clear branch](../../../../src/server/internal/transport/review_deadline_test.go#L59) · [Expiry handler branch](../../../../src/server/internal/transport/review_deadline_test.go#L123) |
| Restore the pre-repair scan reservation. | `TestClaimScanAfterExhaustedAttempt` could not advance on the replacement stream. [Assertion](../../../../src/server/internal/link/review_claim_test.go#L48) |
