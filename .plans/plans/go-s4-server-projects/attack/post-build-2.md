# SHIP

A lost acknowledgment for a read-only poll tells the owner that the already committed acceptance write is uncertain.

OpenAI supplied the test writer, the builder and this reviewer. I reviewed Projects item 2 on `sprint-0` at `ea6f6e9d~1..23c61b2c`. The task identifier is `postbuild-go-s4-server-projects-2` and the nonce is `c750dd`. [Builder result](../../../../.foundry/go-rewrite/results/s4-projects-2-build.md)

## Findings

### post-2 1 — MINOR — A polling read can surface the wrong database message

**Statement.** Foundation rule R8 requires a fixed Connect code and a sentence the website can show. Plan item 2 calls for separate acceptance and polling transactions and preserves same-key replay after an uncertain acceptance write. [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L369) · [Plan](../plan.md#L125)

**Evidence.** By reading, the polling path uses a read-only Store transaction. Store classifies a lost commit acknowledgment as `OutcomeUnknown`. Projects uses the same mapper for admission and polling, so both can return `unavailable` with “The database did not confirm whether the write committed.” [Read-only transaction](../../../../src/server/internal/store/pool.go#L295) · [Commit classifier](../../../../src/server/internal/store/pool.go#L124) · [Polling and error dispatch](../../../../src/server/internal/projects/operations.go#L98) · [Projects error mapping](../../../../src/server/internal/projects/errors.go#L57)

**Failure scenario.** Confirmed live, an independent PostgreSQL connection saw the acceptance and saved a terminal fixture reply. The probe then dropped the acknowledgment for the poll's read-only commit. The caller got the uncertain-write sentence. A same-key retry recovered the original reply and identifiers, with one operation, one command, one held receipt and one outbox wake.

**Fix.** Map polling storage failures to the fixed database-unavailable sentence. Preserve commit uncertainty for acceptance and leave caller cancellation and deadline handling intact.

### post-2 2 — MINOR — The deadline assertion accepts a 30-second wait

**Statement.** Plan item 2 and the sprint 4 disconnect check set a 25-second wait after acceptance. [Plan](../plan.md#L125) · [Sprint 4 check R7](../../../../docs/specs/2026-11-27-running-an-agent-design.md#L297)

**Evidence.** By reading, the implementation uses a 25-second timeout. The test starts the elapsed clock before acceptance and allows up to 32 seconds, with a 35-second caller deadline. Confirmed live, changing only `folderWaitLimit` to 30 seconds passed all three frozen item 2 tests. The disconnect subtest printed `PASS` in `30.60s`. [Timeout constant](../../../../src/server/internal/projects/operations.go#L24) · [Deadline assertion](../../../../src/server/internal/projects/operations_test.go#L946)

**Failure scenario.** A future change raises the wait limit to 30 seconds. The owner waits five seconds longer for the fixed disconnect response and all three tests still pass.

**Fix.** Observe the service deadline on the first post-acceptance read and assert its 25-second budget within a small scheduling allowance. Keep the real disconnect and open-row assertions.

## Post-build checks and plan conformance

By reading the plan and the newest register, item 2 has no unreviewed mechanism. The flagged refresh mechanism belongs to items 5 through 7. I checked the acceptance portion of the uncertain-commit scenario and the local restart and disconnect scenarios. [Register](round-1-findings.md#L10) · [Post-build checks](../plan.md#L279)

- Confirmed live, losing the commit acknowledgment after a committed acceptance left one operation, one command, one held receipt and one wake. After reopening Store, the same key recovered the original identifiers and terminal fixture reply while offline. Losing the acknowledgment after a rollback left zero of all four records. Reusing the key admitted one operation and returned its fixture reply. [Lost-commit fixture](../../../../src/server/internal/projects/operations_test.go#L807)
- Confirmed live, both Inspect and targeted Add replayed the stored Protocol Buffers replies, including an unknown field, after Store and Projects were reopened against the same database. A typed path with a trailing space survived admission and replay byte for byte. The same key with the space removed returned the fixed conflict. The signed fixture disconnect left the operation and command open at the fixed deadline. Caller cancellation and an earlier caller deadline preserved replay. [Restart and exact path assertions](../../../../src/server/internal/projects/operations_test.go#L725) · [Disconnect and caller deadline assertions](../../../../src/server/internal/projects/operations_test.go#L946)
- By reading, both unary calls validate the identifiers and path before admission. Store validates the request key before opening the write. Fresh admission locks the machine, checks approval and held presence, and passes the same transaction to Link enqueue and local operation insertion. Store saves the returned identifier under the caller-wide key. The replay callback reads the stored result under the receipt lock. Separate reads release their connection before the polling wait. The frozen tests observed the independent write, same-key serialization, distinct-key command order, and rollback of the operation, command, receipt and wake. [Validation and replay](../../../../src/server/internal/projects/operations.go#L73) · [Request identity](../../../../src/server/internal/projects/operations.go#L121) · [Admission](../../../../src/server/internal/projects/operations.go#L141) · [Polling](../../../../src/server/internal/projects/operations.go#L185) · [Store key lock](../../../../src/server/internal/store/receipts.go#L81)
- By reading, the polling timer and context have explicit cleanup. The forced insert failure exposed none of the synthetic request data or database diagnostic in the error or captured application log. The builder's result explains the chosen fixed sentence for uncertain acceptance. The typed browser path is stored in the pending command under decision 26. [Polling cleanup](../../../../src/server/internal/projects/operations.go#L185) · [Rollback privacy assertions](../../../../src/server/internal/projects/operations_test.go#L523) · [Browser input decision](../decisions.md#L30)

The pinned commit has the real Store and a signed Link fixture. The fixture settles with stored reply bytes and leaves command delivery disabled. Real client inspection counts, client-derived filtering, routed settlement, and both retirement lock orders require the paired checks and the later Projects items. [Fixture](../../../../src/server/internal/projects/operations_test.go#L108) · [Later checks](../plan.md#L267)

## PostgreSQL verification

I copied commit `23c61b2c` to one disposable directory and ran the plan's gate with its PostgreSQL wrapper and a writable Go build cache. The branch's intentionally red router tests and Link test runtime limit are outside this selector. The command exited 0 with 27 top-level passes and zero skips. [Plan gate](../plan.md#L165)

```sh
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/store ./src/server/internal/projects \
  -run '^Test(HeldReceipt|Migrate|Prune|WriteOnce|RecordValidationAndLimit|Project(Migration|Storage|Operation|Receipt|Replay|Report|FolderSettlement))' -count=1
```

```text
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/store     65.564s
--- PASS: TestProjectOperationAtomicStart (6.27s)
--- PASS: TestProjectOperationReplayAfterRestart (2.96s)
--- PASS: TestProjectOperationDeadlineAndDisconnect (30.43s)
PASS
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  42.193s
```

Each mutation used an independent Go overlay in the same disposable copy. I ran `go test -count=1 -p 1 -v` with each overlay and the matching subtest selector through the PostgreSQL wrapper. [Planned mutations](../plan.md#L129)

| Mutation | Result |
|---|---|
| Check availability before replay. | Exit 1. The restart and offline Inspect retry failed at line 752 with `The call did not return the stored operation reply.` |
| Wait inside the acceptance write. | Exit 1. The independent-write subtest failed at line 913 because the operation, command and receipt did not become visible together within 15 seconds. |
| Enqueue in a separate write. | Exit 1. The fault subtest found one command and one wake after rollback at line 560. The retry produced two commands and two wakes at line 567. |
| Raise the wait to 30 seconds. | Exit 0. All three frozen item 2 tests passed in `44.478s`. |

**Grade.** 9 out of 10.
