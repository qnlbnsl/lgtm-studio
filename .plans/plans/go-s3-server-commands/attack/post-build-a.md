# REVISE

A canceled command send can return success after its cancellation handler has already expired the writer's deadline.

## Findings

### post-a 1 — MAJOR — A send can lose cancellation during deadline cleanup

**Statement.** Item 4a says a context error prevents dispatch confirmation. Foundation rule R6 requires commands to go through the stored link. [Item 4a](../plan.md#L162) · [R6](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L367)

**Evidence and failure scenario.** By reading, `Send` takes its final context-error snapshot before the deferred cleanup either waits for the cancellation handler or clears the deadline. Two scratch probes confirmed the error live. When cancellation arrived during the deadline clear, `Send` returned nil. When cancellation arrived just after the final snapshot and its handler expired the writer before cleanup finished, `Send` again returned nil. The future dispatch caller can therefore take the success path if the shorter send context is canceled while the enclosing write remains live. [Error snapshot](../../../../src/server/internal/transport/deadline.go#L76) · [Cancellation and cleanup](../../../../src/server/internal/transport/deadline.go#L53) · [Dispatch contract](../plan.md#L174)

**Fix.** Resolve context cancellation after the cleanup has stopped or joined the cancellation handler and attempted any deadline clear. Preserve the first callback or flush error. Add deterministic coverage for cancellation in both cleanup branches.

### post-a 2 — MAJOR — The enqueue tests accept an incorrect state revision

**Statement.** Item 1 requires enqueue to preserve the declared state fields. Foundation rule R10 requires real PostgreSQL coverage. [Item 1 payload](../plan.md#L95) · [R10](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L371)

**Evidence and failure scenario.** By reading, enqueue copies the state revision, and the frozen enqueue test decodes only a successful scan. The result tests create their state rows directly. Confirmed live, hard-coding revision 1 passed all 65 tests in the pinned link package. A separate PostgreSQL probe enqueuing revision 17 detected the wrong value. A connected machine already at revision 2 would ignore a rename sent as revision 1 and still reply `done`. [Enqueue fields](../../../../src/server/internal/link/outbox.go#L58) · [Frozen enqueue test](../../../../src/server/internal/link/outbox_test.go#L171) · [Direct result fixture](../../../../src/server/internal/link/outbox_result_test.go#L228) · [Client revision comparison](../../../../src/client/internal/link/commands.go#L113) · [Client reply](../../../../src/client/internal/link/commands.go#L142)

**Fix.** Enqueue a real notice with a revision above 1, a name needing normalization and an unknown nested field. Decode the committed payload and require the exact declared values. Exercise a valid restart payload and invalid notice inputs at the same boundary.

### post-a 3 — MINOR — The transport tests miss cancellation while arming the deadline

**Statement.** Item 4a requires a context check immediately before invoking the response callback. Foundation rule R10 applies to this gate. [Item 4a](../plan.md#L166) · [R10](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L371)

**Evidence and failure scenario.** By reading, the frozen tests cancel before entering `Send` and after the write is held. Confirmed live, deleting only the check after arming the deadline passed all 12 pinned transport tests. A scratch setter that canceled its context while arming the deadline made that mutant call the write callback after cancellation. The pinned implementation passed the same probe. [Deadline setup](../../../../src/server/internal/transport/deadline.go#L45) · [Second context check](../../../../src/server/internal/transport/deadline.go#L73) · [Frozen cancellation cases](../../../../src/server/internal/transport/command_deadline_test.go#L83)

**Fix.** Cancel the context from the fixture deadline setter and assert that `Send` returns cancellation with zero response callback calls.

**Grade.** 7 out of 10.

## Post-build scenarios at the pinned commit

I reviewed items 1, 2 and 4a in `0ac6aaf8..584d7b80`. I used the plan's `UNREVIEWED MECHANISM` labels and the round register's dispatch, expiry and transport dispositions. Every experiment used one disposable copy of the pinned revision. [Plan](../plan.md#L88) · [Round register](round-1-findings.md#L9) · [Post-build checks](../plan.md#L280)

| Check | Confirmed live | Remaining evidence |
|---|---|---|
| P1. Send, flush and heartbeat. | A short-deadline command reached a peer on a real loopback Hypertext Transfer Protocol version 2 (HTTP/2) stream. After the old deadline elapsed, the same stream acknowledged a heartbeat under Connect and Google Remote Procedure Call (gRPC). The frozen tests also returned the first flush error in both protocols and passed the held-write cancellation case. A separate existing HTTP/2 flow-control test passed. | The stored send fence, the real command with zero stream credit and the pool headroom gates require item 4b. |
| P2. Queue indexes and expiry meanings. | On a 120,000-row analyzed fixture, the pending query used `link_commands_pending` with six matching rows and a bitmap scan plus sort. The due queries used index-only scans on `link_commands_expirable` and `link_commands_finished`, and each returned its 64-row page. The constraints rejected a machine expiry without dispatch, allowed a server timeout without dispatch and allowed an explicit expiry with dispatch. The real result path accepted a dispatched result 25 hours past expiry and cleared its payload. | The item 3 cursor wake and the exact claim, sweep and prune boundaries require the claim and sweep paths. The index plans were from representative queries against the pinned migration. |
| P3. Lost commit replies. | A disposable PostgreSQL connection discarded each commit reply separately after the database sent it. A keyed enqueue replay returned its saved reply with one command and one wake. A duplicate result preserved the original outcome and finish time with no new feed row. | The lost claim reply and next-stream delivery require the claim and stored send paths. |
| P4. Restart after migration. | The application was canceled just after PostgreSQL acknowledged migration 2 and before listen. A fresh start acquired one instance lock and retained the waiting machine, keyed reply and event mark. | The combined shutdown with a held command flush and sweep query, and the sweep worker count, require the later composition. |
| P7. Reserved result field. | A private loopback reverse proxy carried a marked reserved field 4 into the real result hook. It committed a final result with a cleared payload. The marker and raw connection credential were absent from the inspected application tables and captured link diagnostics, and the public machine projection stayed byte-identical. A heartbeat was acknowledged afterward. | The deployed proxy, public watch and owner-approved at-rest policy require the assembled stack. This probe used a scripted machine and an injected result hook. |

By reading, item 1 embeds migration 2 and saves the command and wake in the caller's transaction. Item 2 holds the registry lock for its exact-stream check and closes only the matching pending, dispatched attempt. Item 4a arms the earlier deadline and captures the first underlying flush error. The cleanup race is the difference found in the assigned implementation. [Migration embed](../../../../src/server/internal/link/migrate.go#L10) · [Enqueue write](../../../../src/server/internal/link/outbox.go#L38) · [Result fence](../../../../src/server/internal/link/outbox_result.go#L17) · [Deadline setup](../../../../src/server/internal/transport/deadline.go#L45) · [Flush capture](../../../../src/server/internal/transport/link_stream.go#L123)

## Test and mutation output

The full pinned link and transport packages ran with the race detector and a fresh count. Each named test emitted a JavaScript Object Notation (JSON) pass event. The package runs recorded 65 link tests and 12 transport tests, with zero skips or failures.

```text
$ deploy/test/with-postgres.sh -- go test -json -race -count=1 ./src/server/internal/link
PASS TestAcceptResultRules
PASS TestResultAndReplacementFormOneLine
PASS TestCommandMigrationUpgradesPartB
PASS TestEnqueueStoresBeforeSend
PASS TestBecomeCurrent
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/link  131.041s

$ go test -json -race -count=1 ./src/server/internal/transport/...
PASS TestSendUsesEarlierContextDeadline
PASS TestLinkSendReportsFlushFailure
PASS TestSendLinkWelcomeReturnsFlushFailure
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/transport  2.221s
```

An anchored rerun confirmed all seven assigned top-level tests. The link rerun printed `ok` in 9.004 seconds, and the transport rerun printed `ok` in 1.039 seconds.

Each planned mutation below ran separately in the scratch copy and exited 1 on the cited assertion. The result-predicate tests modified one Structured Query Language (SQL) predicate per run. The independent schema constraint made dropping the dispatch predicate return an error instead of silently ignoring the attempt.

| Mutation | Assertion that failed |
|---|---|
| Remove the attempt-range constraint, or remove the terminal-pair constraint separately. | `TestCommandMigrationUpgradesPartB` failed independently for attempts 0 and 4294967296, and for each mismatched finish-time and outcome pair. [Migration test](../../../../src/server/internal/link/outbox_test.go#L137) |
| Skip the wake, or commit the insertion separately. | `TestEnqueueStoresBeforeSend` reported a missing wake or early visibility, respectively. [Wake assertion](../../../../src/server/internal/link/outbox_test.go#L223) · [Visibility assertion](../../../../src/server/internal/link/outbox_test.go#L309) |
| Drop the machine, stream, attempt, dispatch or pending predicate. | `TestAcceptResultRules` rejected each mutation independently. [Machine](../../../../src/server/internal/link/outbox_result_test.go#L67) · [Stream](../../../../src/server/internal/link/outbox_result_test.go#L75) · [Attempt](../../../../src/server/internal/link/outbox_result_test.go#L57) · [Dispatch](../../../../src/server/internal/link/outbox_result_test.go#L131) · [Pending](../../../../src/server/internal/link/outbox_result_test.go#L213) |
| Check the stream only before taking the registry lock. | `TestResultAndReplacementFormOneLine` found that the old result changed the pending row after replacement committed. [Ordering assertion](../../../../src/server/internal/link/outbox_result_test.go#L348) |
| Ignore the shorter deadline, or remove both checks before invoking a callback. | `TestSendUsesEarlierContextDeadline` found the wrong deadline or a callback that ran on a canceled context. [Deadline assertion](../../../../src/server/internal/transport/command_deadline_test.go#L63) · [Cancellation assertion](../../../../src/server/internal/transport/command_deadline_test.go#L94) |
| Suppress the flush error, overwrite the first flush error, or suppress the context check after the callback. | `TestLinkSendReportsFlushFailure` failed for each of the three changes under both protocols. [Flush assertion](../../../../src/server/internal/transport/command_deadline_test.go#L207) · [Context assertion](../../../../src/server/internal/transport/command_deadline_test.go#L228) |
