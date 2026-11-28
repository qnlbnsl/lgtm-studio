# REVISE

A start callback held across a command's expiry lets the client inspect the checkout late and record the command as done.

The test writers and builders were OpenAI Codex workers, the same vendor as this reviewer. This review covers item 9 on `sprint-0` at `b13bb5f6~1..35785b09` and the store-open repair in `3df72d3f` with its frozen regression from `c527b59c`. File and line references refer to the reviewed commits.

## Findings

### post-9 1 | MAJOR | A held start callback can run an expired inspection

The plan requires the final expiry and cancellation checks before the reader starts. The worker checks both, calls `OnFolderInspectionStart` synchronously, then enters the reader without checking again. The frozen retry fixture deliberately uses the callback to hold the worker before reader entry. [Plan items 6 and 9](../plan.md#L164) · [Worker](../../../../src/client/internal/link/folder_command.go#L63) · [Held observer](../../../../src/client/internal/link/folder_command_test.go#L402)

Confirmed live, a scratch probe admitted a command with one second left on the authenticated clock. The start callback advanced that clock by two seconds while the read context was still live. The inspector was called, and supplying its facts advanced the store's next number from 1 to 2 and produced `COMMAND_OUTCOME_DONE` at watermark 1. A paired retry near expiry can therefore observe a successful report from a read that began late. [Expiry calculation](../../../../src/client/internal/link/commands.go#L63) · [Report and receipt settlement](../../../../src/client/internal/link/folder_command.go#L99)

Recheck cancellation and authenticated expiry after the observer returns and before entering the reader. Settle a still-running command as expired if its expiry passed before reader entry. Freeze a case that holds the observer across expiry and requires an expired result with no read or report.

### post-9 2 | MINOR | The suite accepts start notifications for pre-read declines

The plan says pre-read declines never notify the observer. The fixture for expiry during the start-receipt write leaves the observer unset. The cases that refuse unsafe paths also leave it unset. The separate already-expired case does install an observer, but admission rejects that command before it can reach the worker. [Plan item 9](../plan.md#L175) · [Expiry fixtures](../../../../src/client/internal/link/folder_command_test.go#L488) · [Path fixtures](../../../../src/client/internal/link/folder_command_test.go#L950)

Confirmed live, moving the observer call ahead of the worker's expiry and path-refusal branches left all 11 client packages green with the unrelated container-only case excluded as described below. Under that mutation, a path the worker refuses before reading would still count as one inspection. Install the observer in the late-expiry and unsafe-path cases and assert zero notifications alongside zero reader calls.

### post-9 3 | MINOR | The metadata regression accepts a first-page-only fallback

The post-8 1 repair must establish damage before returning the recovery outcome. The frozen regression blocks all positional reads in one case and clears both metadata pages in the other. Both cases accept a fallback that checks only page zero. [Repair requirement](post-build-8.md#post-8-1) · [Fallback](../../../../src/client/internal/link/store.go#L303) · [Frozen regression](../../../../src/client/internal/link/revision_two_store_read_review_test.go#L38)

Confirmed live, checking only page zero left all 11 client packages green with the same exclusion. A scratch control created a database with 16-kibibyte pages, cleared the first metadata page and reopened it with bbolt through the surviving page. The reviewed fallback returned a plain store failure for the ambiguous open. The mutant returned `ErrStoreDamaged`. If an initial read fault clears before fallback on a file with only the second header intact, that mutation can give restore advice and an exit status of 0 for a file bbolt can still open. Extend the regression with a readable surviving second header and an initial metadata read failure that clears before fallback. [Recovery diagnostic](../../../../src/client/internal/link/session.go#L398) · [Application exit mapping](../../../../src/client/internal/app/client.go#L130)

## Post-build and acceptance checks

By reading, the plan and the round 1 register assign no `UNREVIEWED FIX` entry to item 9 or this repair. Item 9 does exercise the inherited unreviewed deadline wrapper. Confirmed live, the local fixture held one read across a disconnect, retried on a new stream and received one report and the result for the newest attempt. A timed-out read released the slot while the original reader was still held. The held reader's late return added no report, and a later requested restart completed while another reader was held. [Register](round-1-findings.md#L45) · [Post-build scenarios](../plan.md#L265) · [Retry fixture](../../../../src/client/internal/link/folder_command_test.go#L402) · [Offline timeout fixture](../../../../src/client/internal/link/folder_command_test.go#L584)

Confirmed live, both callbacks reached the real Link run through the application and public test adapter. In the interruption fixture the callback received durable report number 1 while the receipt was started. Inventory appended at number 2 while the callback was held. Returning an error stopped the run before it handed off a report or result. After reopening twice and appending fresh inventory, attempts 3 and 5 each returned `interrupted` with watermark 2 and no repeat inspection. This establishes the local part of item 9's Done when row. Native macOS and the paired PostgreSQL proofs remain release-wave checks. [Done when](../plan.md#L130) · [Application fixture](../../../../src/client/clienttest/clienttest_test.go#L275) · [Interruption fixture](../../../../src/client/internal/link/folder_command_test.go#L766) · [Release handoff](../../../../.foundry/go-rewrite/results/s4-client-9-build.md#L87)

Confirmed live, the store repair returned a plain failure after the injected input/output error. The executable exited 1 and printed one `store_failed` diagnostic with `Could not use client.db.` The database bytes stayed identical and the intact file opened after the fault cleared. The readable invalid metadata control still classified the file as damaged. [Assertions](../../../../src/client/internal/link/revision_two_store_read_review_test.go#L68)

The direct Linux test run stopped in fixture setup because the sandbox presents `/` as owned by user identifier 65534. I reran the checks in an isolated Linux container as an ordinary user using Go 1.26.0 and Git 2.39.5. The source came from the committed snapshot. Both item 9 frozen test files matched the recorded checksums.

```sh
go test -v -race ./src/client/clienttest ./src/client/internal/link -run 'TestClienttestSharesApp|TestLookAtFolderLifecycle|TestAfterFolderReport' -count=1
go test -v -race -count=1 ./src/client/internal/link -run '^(TestRevisionTwoStoreMetadataRead|TestRevisionTwoStoreOpenErrorClassification|TestRevisionTwoStoreOpenPermissionFailure|TestOpenStoreRefusesDamagedFile)$'
go test -v -race ./src/client/internal/link ./src/client/internal/app -run 'TestRevisionTwoTerminalHistory|TestRevisionTwoExecutableStop|TestRetryFailureExitsAndKeepsStore|TestAppRunStoreStatus' -count=1
go test -v -race ./src/client/internal/app ./src/client/internal/link -run 'TestOutgoingFilterExecutable|TestOutgoingFilterStructuredLog|TestAppLogRedactsSecrets|TestOpenFrameDebugDoesNotLogConnectionToken' -count=1
```

| Run | Output |
|---|---|
| Item 9 | Exit 0. All three named tests printed `PASS` with zero skips. The adapter and Link packages printed `ok` at 1.185s and 2.887s. The restored run also passed all three. |
| Store-open repair | Exit 0. All four named tests printed `PASS`, including both metadata controls. Link printed `ok` at 15.232s. |
| Item 3 | Exit 0. All four named tests printed `PASS`. Link and App printed `ok` at 1.852s and 8.085s. |
| Item 8 | Exit 0. All four named tests printed `PASS`. App and Link printed `ok` at 5.426s and 2.130s. |

The wider client run printed `ok` for ten packages, including every package in the assignment. The inherited sparse-checkout case failed under the container's Git version, as recorded in the build handoff. For the two additional coverage mutations I excluded only that subtest with `-skip '^TestInspectFolderReadOnly$/^sparse_entry_intentionally_absent$'`. Each isolated run then printed `ok` for all 11 packages. [Build handoff](../../../../.foundry/go-rewrite/results/s4-client-9-build.md#L81)

## Planned mutation results

Each mutation was applied independently and exited 1 at its intended assertion. [Plan mutations](../plan.md#L180)

| Mutation | Observed failure |
|---|---|
| Count a finished receipt replay as another read. | The replay produced an additional start notification. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L479) |
| Invoke the append callback after finishing the receipt. | The callback observed a finished receipt. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L817) |
| Recover at the folder report's number. | Attempt 3 returned watermark 1 where the fixture required 2. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L877) |
| Recover at the new process's current watermark. | Attempt 3 returned watermark 3 where the fixture required 2. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L877) |
| Treat every ambiguous bbolt metadata error as damage. | The temporary read fault returned a damage error and the executable exited 0 with the recovery instruction. [Assertions](../../../../src/client/internal/link/revision_two_store_read_review_test.go#L68) |

## Grade

7 out of 10. The committed Linux fixtures prove the interruption and watermark behavior, and the store repair passes its executable regression. The held observer still permits a late read, and two narrower mutations expose gaps in the frozen assertions.
