# REVISE

A stalled sign-in file read in the link owner's own timeout settlement still stops folder results, heartbeats and restart.

The test writers and builders were OpenAI Codex workers, the same vendor as this reviewer. This review covers the item 6 and item 9 repairs on `sprint-0` in `2381e7ba~1..dd5c4fa1`. The task identifier is `postbuild-go-s4-client-folders-10` and the nonce is `112d7e`. The source references and test runs use `dd5c4fa1`.

## Findings

### post-10 1 | MAJOR | Timeout settlement can block the link owner on its own sign-in read

The item 6 plan requires an inspection's filesystem work to leave the link owner free to process heartbeats, acknowledgements and restart. The repair closes the lock contention in post-6 2. The frozen test reproduced the original stall with the old lock ordering and passed with the repair. That fixture deliberately stalls only the worker's refresh. The stream loop still calls `finishFolder` synchronously, and report admission performs its own file refresh before it can take the registration lock. The production reader bounds the file type and byte count. Its open, stat and read calls have no time bound. [Plan](../plan.md#L121) [Timeout requirements](../plan.md#L164) [Frozen worker-only fault](../../../../src/client/internal/app/folder_command_review_test.go#L34) [Stream owner](../../../../src/client/internal/link/session.go#L716) [Settlement](../../../../src/client/internal/link/folder_command.go#L99) [Append gate](../../../../src/client/internal/link/outbox.go#L26) [Refresh and read](../../../../src/client/internal/link/filter.go#L87) [File reader](../../../../src/client/internal/app/outgoing.go#L31)

Confirmed live, I extended the signed-peer fixture in scratch to hold the refresh on the `finishFolder` call stack as well as the worker's refresh. The wrapper delivered completion at the five-second deadline and the link owner entered its own refresh. The peer sent restart. At 12 seconds it had seen no folder report, folder result, next heartbeat or restart result, and the application was still running. The builder's stated limit is outside the plan's liveness guarantee. Move potentially blocking refresh work off the link owner and give the affected operation a bounded wait. If screening cannot finish, fail the affected operation without emitting unscreened output. Keep the registration and output check together, and add a signed-peer regression that holds the owner's refresh and checks both control traffic and the outcome. [Builder's stated limit](../../../../.foundry/go-rewrite/results/s4-client-6r-build.md#L14) [Screening requirements](../plan.md#L226)

### post-10 2 | MINOR | A sampled sign-in value can miss screening during rotation

Item 8 requires refresh and classification to be serialized with registration. Foundation rule R9 keeps secrets out of logs. The repair lets one gate read a credential outside the lock and pause before registering it. A second gate can then read a replacement, register it and screen output while the first sample is still unpublished. [Item 8](../plan.md#L226) [Foundation rule R9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370) [Refresh order](../../../../src/client/internal/link/filter.go#L87) [Log screening](../../../../src/client/internal/app/outgoing.go#L122)

Confirmed live, a scratch probe used the production sign-in reader and logger with a disposable file. It paused the first caller just after reading a synthetic credential, replaced the file and logged text containing the first value. The logger emitted the value before the first caller could register it. Preserve ordering between completed credential samples and subsequent dynamic output. Withhold output when the required screening state cannot be established in time, and cover the rotation with overlapping refreshes in a regression. This security finding does not determine the verdict.

### post-10 3 | MINOR | The two-permit test misses false inspection starts

Item 9 requires zero start notifications when the client declines before reading. The implementation declines a third inspection when both read permits are occupied. The retired-read test checks for zero additional reader calls and the correct refusal but installs no start observer. [Item 9](../plan.md#L175) [Permit refusal](../../../../src/client/internal/link/folder_command.go#L48) [Retired-read test](../../../../src/client/internal/link/folder_command_test.go#L706)

Confirmed live, notifying the observer in the out-of-permits branch without invoking the reader left all 11 client packages green under the race detector. I excluded the inherited sparse-checkout subtest with `-skip '^TestInspectFolderReadOnly$/^sparse_entry_intentionally_absent$'`, which the build handoff records as failing with the container's Git version. A paired fixture could then count a third inspection that never ran. Install the observer in the retired-read test, consume the notifications from the first two admitted reads and assert zero notifications for the third command before releasing a permit. [Build environment limit](../../../../.foundry/go-rewrite/results/s4-client-6r-build.md#L50)

## Repair decisions

| Prior finding | Result at the reviewed commit |
|---|---|
| post-6 1 | Closed. The signed stream replayed a saved result before considering expiry. While the first inspection was still held, a separate expired command saved and returned `EXPIRED` at watermark 1 on that stream, without a report or another read. The held inspection then settled at watermark 2. [Admission](../../../../src/client/internal/link/commands.go#L210) [Signed-peer test](../../../../src/client/internal/link/folder_command_review_test.go#L52) |
| post-6 2 | Open in part. The worker's stalled refresh no longer blocks settlement through the registration lock. The separate synchronous refresh in settlement produces post-10 1. [Prior finding](post-build-6.md#post-6-2--major--a-stalled-credential-read-can-still-block-timeout-settlement) |
| post-6 3 | Closed. The normal reader returned the physical path, name, branches, first commit, origin and dirty status of a real Git checkout. The expected report and correlation were observed both on disk and on the signed stream. The fixture compared the contents of the checked files before and after inspection. [Default reader](../../../../src/client/internal/link/folder_command.go#L13) [Signed-peer test](../../../../src/client/internal/link/folder_command_review_test.go#L150) |
| post-9 1 | Closed. Releasing the held callback after expiry produced no read or report, left the next event number at 1 and saved and returned `EXPIRED` at watermark 0. Releasing it after cancellation produced no read. [Final worker checks](../../../../src/client/internal/link/folder_command.go#L71) [Held-callback test](../../../../src/client/internal/link/folder_command_expiry_review_test.go#L37) |
| post-9 2 | Closed for the original scenarios. Receipt-write expiry and refusals of unsafe typed and expanded paths caused zero observer calls and zero reads. The revised fixtures detected an observer moved ahead of those checks. Post-10 3 concerns a different pre-read decline. [Expiry case](../../../../src/client/internal/link/folder_command_test.go#L488) [Path cases](../../../../src/client/internal/link/folder_command_test.go#L955) |
| post-9 3 | Closed. The test confirmed that bbolt could open the database through its second header. An injected input/output fault then blocked the initial full-page probes and allowed short fallback reads. The client classified the store as a plain failure and preserved its bytes. [Metadata fallback](../../../../src/client/internal/link/store.go#L303) [Fixture and controls](../../../../src/client/internal/link/revision_two_store_read_review_test.go#L92) |

## Deadline and settlement scenarios

The plan calls item 6's deadline wrapper an unreviewed mechanism. The review register names it under `threat-r1 07`. Item 9 adds no separate unreviewed mechanism in this repair. Confirmed live, an offline read timed out at five seconds, persisted one refusal and settled the newest attempt after reconnect. A second command obtained the inspection slot while the retired read was still held. Its predecessor's late return added no event. An accepted restart canceled the new read and exited while that call was still held. With an already full outbox, timeout recorded failure at the unchanged watermark. With two retired reads held, a third got a fixed refusal and a fourth could start after one permit was released. Cancellation also cleaned up the private Git index and preserved source files. [Register](round-1-findings.md#L45) [Plan](../plan.md#L165) [Offline timeout and restart](../../../../src/client/internal/link/folder_command_test.go#L590) [Full buffer and two permits](../../../../src/client/internal/link/folder_command_test.go#L652) [Private index](../../../../src/client/internal/folders/status_test.go#L404)

The signed-peer retry fixture observed one inspection and one report across stream replacement, with the result sent on the newest attempt. It also exercised heartbeat and acknowledgement traffic while a checkout read was held. The interruption fixture observed folder report 1 before receipt completion, allowed inventory to advance the watermark to 2, and recovered attempts 3 and 5 as interrupted at watermark 2. These runs cover the local parts of the Done when criteria and the ordered-replay behavior in foundation check C11. The pinned revision has neither of the named Sprint 4 paired tests in `src/linktest`. The PostgreSQL rollback and acknowledgement-loss cases, integrated server and website filter capture, and native macOS acceptance remain unverified here. [Done when](../plan.md#L123) [Retry fixture](../../../../src/client/internal/link/folder_command_test.go#L432) [Interruption fixture](../../../../src/client/internal/link/folder_command_test.go#L817) [Paired checks](../plan.md#L265) [Foundation check C11](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L388)

## Linux acceptance and mutation results

The pinned source ran as an ordinary user in an isolated Linux container with Go 1.26.0 and Git 2.39.5. The container provides the root-directory ownership required by the state fixtures. The focused runs used the race detector.

```sh
go test -v -race ./src/client/internal/link -run 'TestLookAtFolderLifecycle|TestLookAtFolderEventBudget|TestReviewGUnacknowledgedEventWindow' -count=1
go test -v -race ./src/client/clienttest ./src/client/internal/link -run 'TestClienttestSharesApp|TestLookAtFolderLifecycle|TestAfterFolderReport' -count=1
go test -v -race ./src/client/internal/link ./src/client/internal/app -run '^Test(ExpiredFolderCommandWhileInspectionIsHeld|StalledFolderSignInReadDoesNotBlockLink|FolderCommandInspectsCheckoutWithDefaultReader|FolderCommandHeldStartObserver|RevisionTwoStoreMetadataRead)$' -count=1
```

| Run | Output |
|---|---|
| Item 6 | Exit 0. The three named tests printed `PASS` with zero skips. Link printed `ok` at 9.344 seconds. |
| Item 9 | Exit 0. The three named tests printed `PASS` with zero skips. The public test adapter printed `ok` at 1.453 seconds and Link printed `ok` at 5.329 seconds. |
| Focused repair tests | Exit 0. All five named tests and their subtests printed `PASS` with zero skips. Link printed `ok` at 15.128 seconds and App printed `ok` at 11.226 seconds. |

Each mutation below used a separate Go overlay in the same scratch copy and exited 1 at the specified assertion. [Item 6 mutations](../plan.md#L170)

| Mutation | Failing assertion |
|---|---|
| Return busy for an expired folder command. | The occupied stream closed before returning an expired result. [Assertion](../../../../src/client/internal/link/folder_command_review_test.go#L71) |
| Read the sign-in file under the registration lock. | The refusal and result missed the seven-second limit. The fixture also missed the heartbeat and restart result and exit. [Assertion](../../../../src/client/internal/app/folder_command_review_test.go#L170) |
| Replace the default reader with fixed not-readable refusal. | The wire report lacked the expected checkout facts. [Assertion](../../../../src/client/internal/link/folder_command_review_test.go#L177) |
| Remove the post-callback expiry or cancellation check, one at a time. | Both mutations entered the reader. The expiry mutation committed one report and returned `DONE` at watermark 1. [Cancellation assertion](../../../../src/client/internal/link/folder_command_expiry_review_test.go#L47) [Expiry assertions](../../../../src/client/internal/link/folder_command_expiry_review_test.go#L61) |
| Call the observer before the expiry and path checks. | Expiry during admission and both unsafe path cases detected one unwanted start. [Expiry assertion](../../../../src/client/internal/link/folder_command_test.go#L510) [Path assertion](../../../../src/client/internal/link/folder_command_test.go#L997) |
| Read only the first metadata header. | The fixture received a damage error where it required a plain store failure. [Assertion](../../../../src/client/internal/link/revision_two_store_read_review_test.go#L205) |
| Finish the receipt before committing the report. | The fixture observed the finished receipt before its report. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L468) |
| Record done on a full outbox. | The fixture expected failed at watermark 10,000 and received done. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L688) |
| Skip the expiry check before starting the reader. | A command that expired during admission reached the reader. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L508) |
| Size the event before assigning its number. | Facts one byte over the numbered limit failed to become the fixed refusal. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L1152) |
| Keep waiting for the retired reader after timeout. | There was no durable refusal at the deadline. [Assertion](../../../../src/client/internal/link/folder_command_test.go#L612) |

## Grade

7 out of 10. The repaired admission, default reading, observer checks and store classification have discriminating tests. The link owner's synchronous refresh still violates the timeout and restart requirement.
