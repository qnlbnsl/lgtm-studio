# REVISE

An expired folder command closes the current stream without a result when a different inspection occupies the folder slot.

The test writer and builder were OpenAI workers, the same vendor as this reviewer. I reviewed item 6 of `go-s4-client-folders` on `sprint-0` at `e03e91b6~1..f7130c3f`. Source line references refer to the pinned commit. The task identifier is `postbuild-go-s4-client-folders-6` and the nonce is `98e85c`.

## Findings

### post-6 1 | MAJOR | An expired folder command closes a healthy stream

The plan and the reviewed command contract require an expired, unstarted command to receive an expired result. The dispatcher first looks for a saved receipt. If it finds none and the folder slot is occupied, it returns the busy error even when its clock already puts the incoming command past expiry. The existing scan path checks the actual admission time before returning busy. The stream owner returns the folder-busy error from its receive loop, which ends the stream. [Plan](../plan.md#L164) · [Command contract](../../../../src/contracts/proto/lgtm/link/v1/commands.proto#L27) · [Receipt lookup](../../../../src/client/internal/link/receipts.go#L236) · [Busy branch](../../../../src/client/internal/link/commands.go#L210) · [Stream exit](../../../../src/client/internal/link/session.go#L717)

Confirmed live in a scratch probe, holding one read and delivering a second command one second past expiry returned `link: another folder command is running` in place of an expired result. On a live stream that forces a reconnect and delays settlement. The frozen tests cover a fresh command with an occupied slot and an expired command with a free slot. Apply the scan path's admission-time check to folder contention, preserve saved-receipt precedence, and assert an expired result on the same stream with no second read. [Expiry fixture](../../../../src/client/internal/link/folder_command_test.go#L462) · [Busy fixture](../../../../src/client/internal/link/folder_command_test.go#L505)

### post-6 2 | MAJOR | A stalled credential read can still block timeout settlement

The unreviewed mechanism in the plan requires a stalled read to release the slot and lets the link continue receiving commands. Before invoking the checkout reader, the detached task checks the path through the shared filter. The application filter refreshes its local credential file while holding its mutex. Timeout settlement uses the same filter to append the refusal. If the credential file stalls on a slow filesystem during the worker's refresh, the timer can fire while the link owner waits on the worker's mutex. Heartbeats and a restart command can then wait behind settlement. [Plan](../plan.md#L165) · [Worker path check](../../../../src/client/internal/link/folder_command.go#L48) · [Refusal append](../../../../src/client/internal/link/folder_command.go#L77) · [Filter lock](../../../../src/client/internal/link/filter.go#L87) · [Credential file read](../../../../src/client/internal/app/outgoing.go#L31) · [Link owner](../../../../src/client/internal/link/session.go#L702)

Confirmed live with an injected credential reader that paused only in the worker, the wrapper published its fixed refusal at five seconds, and the link owner could commit it only after the reader was released. The frozen deadline tests hold the checkout reader after the filter check. Read the credential file outside the lock that the worker and link owner share. Publish newly learned spellings under that lock and keep screening through the commit. Bound how long command settlement can wait for a refresh and add a regression that holds it across timeout and restart. [Held checkout fixture](../../../../src/client/internal/link/folder_command_test.go#L87) · [Restart fixture](../../../../src/client/internal/link/folder_command_test.go#L533)

### post-6 3 | MINOR | The frozen tests can pass with the production reader disconnected

Replacing the default inspector with an implementation that always returns the fixed not-readable refusal left every test in the item 6 acceptance command green. The lifecycle and budget fixtures substitute their own inspector before the command runs. A wiring regression could therefore make every real checkout appear unreadable while the item gate passes. Add a signed-link case that inspects a disposable Git working tree with the default reader and asserts facts on disk and on the wire. This covers the client side of the plan's paired folder checks and foundation check C11. [Production inspector](../../../../src/client/internal/link/folder_command.go#L13) · [Fixture replacement](../../../../src/client/internal/link/folder_command_test.go#L87) · [Item gate](../plan.md#L171) · [Foundation check C11](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L388)

**Grade 7/10.**

## Deadline wrapper and folder settlement checks

The plan marks the wrapper as unreviewed under `threat-r1 07`. Confirmed live, the offline fixture persisted one fixed refusal before finishing the receipt, reopened the command slot with the first read still held, and observed no second append from its late return. A full outbox retained its number and produced a failed result. Two unresolved reads refused further work until one exited. Restart returned its durable result and normal process outcome while the new checkout read remained held. Finding `post-6 2` records the credential-refresh case that those fixtures miss. [Review register](round-1-findings.md#L45) · [Unreviewed mechanism](../plan.md#L165) · [Offline timeout and restart](../../../../src/client/internal/link/folder_command_test.go#L533) · [Full outbox](../../../../src/client/internal/link/folder_command_test.go#L595) · [Unresolved reads](../../../../src/client/internal/link/folder_command_test.go#L633)

Confirmed live, the signed peer held one read across a disconnect and received one durable report and the result on the newest attempt. The uncertain local commit stopped further writes and sent no result. Separate filter controls refused an unsafe branch and omitted an unsafe origin learned during the read. A scratch case with the default inspector returned the expected dirty checkout, branch, origin and first commit on a signed link and observed the durable report. The native Linux private-index cancellation fixture passed and checked source preservation and temporary-view cleanup. By reading, Git runs under the worker context, cancellation signals its process group, and `exec.Cmd.Run` waits for the child in the detached worker. [Retry fixture](../../../../src/client/internal/link/folder_command_test.go#L373) · [Uncertain commit fixture](../../../../src/client/internal/link/folder_command_test.go#L672) · [Filter fixtures](../../../../src/client/internal/link/folder_command_test.go#L699) · [Private-index cancellation](../../../../src/client/internal/folders/status_test.go#L404) · [Git cancellation](../../../../src/client/internal/folders/checkout.go#L411)

The paired PostgreSQL rollback and acknowledgement-loss scenarios, the full executable credential-rotation capture, and native macOS acceptance remain release-wave checks. The signed-peer runs above exercise the local half of settlement and filtering. [Post-build checks](../plan.md#L265) · [Builder handoff](../../../../.foundry/go-rewrite/results/s4-client-6-build.md#L55)

## Linux acceptance and mutation results

The pinned source ran as an unprivileged user in a Linux container with Go 1.26.0 and Git 2.39.5. The frozen checksum matched. The plan command exited 0 with three top-level passes, zero skips, and the final lines `PASS` and `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 4.669s`.

```sh
go test -v -race ./src/client/internal/link -run 'TestLookAtFolderLifecycle|TestLookAtFolderEventBudget|TestReviewGUnacknowledgedEventWindow' -count=1
```

The budget fixture passed at 49,152 bytes with a multi-byte event number and observed the fixed refusal one byte over at the same candidate number. The command and outer-message edge checks passed, including the 65,536-byte receive and send caps. A cumulative acknowledgement opened one position in the full 32-event window. [Budget fixture](../../../../src/client/internal/link/folder_command_test.go#L875) · [Command boundary](../../../../src/client/internal/link/folder_command_test.go#L956) · [Outer cap](../../../../src/client/internal/link/folder_command_test.go#L991)

Each planned mutation ran independently in the scratch copy and exited 1 with `FAIL`. The review restored the scratch source byte for byte between mutations. [Planned mutations](../plan.md#L170)

| Mutation | Frozen assertion that failed |
|---|---|
| Finish the receipt first. | The fixture said that the finished receipt was committed before its report. [folder_command_test.go](../../../../src/client/internal/link/folder_command_test.go#L423) |
| Record done with a full outbox. | The fixture saw done at watermark 10,000 and expected failed at watermark 10,000. [folder_command_test.go](../../../../src/client/internal/link/folder_command_test.go#L619) |
| Start after expiry during admission. | The fixture saw the newly expired command reach the worker. [folder_command_test.go](../../../../src/client/internal/link/folder_command_test.go#L459) |
| Size the event before numbering. | Facts one byte over the numbered limit did not become the fixed refusal. [folder_command_test.go](../../../../src/client/internal/link/folder_command_test.go#L911) |
| Keep the slot occupied by a retired call. | The admitted follow-up command failed to start. [folder_command_test.go](../../../../src/client/internal/link/folder_command_test.go#L569) |
