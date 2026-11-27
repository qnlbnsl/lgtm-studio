# REVISE

A finished receipt with conflicting watermarks passes startup validation and can replay at zero before the server commits the events the first result depended on.

OpenAI supplied both test writers, the builder and this reviewer. I reviewed item 2 on `sprint-0` at `e1783f29`, against `0ddf0e47~1`. The task identifier is `postbuild-go-s4-client-folders-2` and the nonce is `3905fe`.

## Findings

### post-2 1 — MAJOR — Duplicate watermark fields can change the replay fence

Item 2 requires damaged metadata to fault and the first durable watermark to survive every retry. Foundation rule R7 requires durable state to be recoverable. [Item 2 receipt format](../plan.md#L86) · [Foundation rule R7](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L368)

Confirmed live. I inserted a synthetic finished receipt holding two conflicting `last_event_number` fields into a private store with two durable events. Preparation returned nil and the next attempt returned Done at watermark zero. The decoder collapses duplicate fields before the range check. A server still at zero would be able to settle that result immediately, bypassing the fence that watermark two would impose. [Receipt decoder](../../../../src/client/internal/link/receipts.go#L58) · [Startup validation](../../../../src/client/internal/link/receipts.go#L145) · [Server settlement contract](../../../../src/contracts/proto/lgtm/link/v1/commands.proto#L12)

Reject duplicate receipt fields before decoding and retain the original bytes on the resulting `ErrStoreDamaged`. Test two individually legal but conflicting numbers.

### post-2 2 — MAJOR — Frozen tests let receipt validation move past the handshake

Item 2 requires startup preparation before link activity. The reviewed implementation calls it before constructing the run. The signed test checks that sealing precedes startup reports, and the corrupt-metadata cases call preparation directly. [Item 2 startup](../plan.md#L87) · [Run startup](../../../../src/client/internal/link/session.go#L84) · [Signed run](../../../../src/client/internal/link/revision_two_receipts_test.go#L479) · [Corrupt-metadata test](../../../../src/client/internal/link/revision_two_receipts_test.go#L558)

Confirmed live. Moving preparation to immediately after a successful Welcome still passed the race tests in all ten in-scope client packages. Link printed `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 93.462s`. A separate probe put a corrupt receipt in an enrolled store and made the transport unavailable. The reviewed code returned `ErrStoreDamaged` with zero transport initializations. The mutation attempted transport and failed the probe. With that ordering, an offline client can keep retrying with damaged local history and defer the recovery diagnosis until it reaches a compatible peer. [Transport construction](../../../../src/client/internal/link/session.go#L138) · [Handshake call](../../../../src/client/internal/link/session.go#L153)

Add a `Run` test with a malformed receipt, assert the typed store fault and unchanged file, and count zero transport initializations.

### post-2 3 — MINOR — The amended age test drops preservation during pruning

Item 2 calls for corrupt metadata to fault without rewriting it. The old age test kept a malformed row in the store across a pruning write and checked that the row survived. The original fixture and assertion are at `0ddf0e47~1:src/client/internal/link/receipts_test.go:265` and line 295. The revised age test uses valid rows. The malformed decline case inserts its bad row after the trusted write and then looks up that same row, so it exercises lookup rather than pruning. [Item 2 corrupt metadata](../plan.md#L91) · [Revised age fixtures](../../../../src/client/internal/link/receipts_test.go#L273) · [Malformed decline](../../../../src/client/internal/link/commands_test.go#L1126)

Confirmed live. A pruner that deletes a malformed unrelated row passed the age, decline and revision 2 receipt tests. A separate probe inserted a malformed row after startup and then finished an unrelated command with a trusted clock. The reviewed implementation faulted and preserved the row. The mutated pruner erased it and recorded the unrelated result. [Finish transaction](../../../../src/client/internal/link/receipts.go#L325) · [Pruning validation](../../../../src/client/internal/link/receipts.go#L339)

Keep the valid age fixture and add a separate write after successful preparation with an unrelated damaged row. Require that the write reports the store fault and preserves both rows' pre-write state.

### post-2 4 — MINOR — The new started-receipt case has other reasons to fail

Item 2 requires a version 2 started receipt to have an explicit null watermark and no result. The new case named “revision two start with a result” omits both `standing` and `last_event_number`. The decoder rejects those omissions before it checks the result, and the separate result case in the new metadata table is for an already interrupted receipt. [Item 2 receipt format](../plan.md#L86) · [Started receipt fixture](../../../../src/client/internal/link/receipts_test.go#L128) · [Interrupted receipt fixture](../../../../src/client/internal/link/revision_two_receipts_test.go#L539)

Confirmed live. A decoder that accepts a result on a version 2 started receipt passed the focused receipt suites. A separate probe supplied otherwise valid metadata plus a result. The reviewed code rejected the row. The mutation accepted it and rewrote it as interrupted during preparation. [Started validation](../../../../src/client/internal/link/receipts.go#L75) · [Sealing](../../../../src/client/internal/link/receipts.go#L151)

Give the fixture a present `standing: false` and a present null watermark. Plant a recognized outcome so the result field is the only defect.

### post-2 5 — MINOR — The frozen decline test drops the first helper return value

Item 2 keeps the store helpers used by frozen tests. The original `TestDeclineDecisions` asserted that the helper's first successful call returned `DecisionExpired` at `0ddf0e47~1:src/client/internal/link/commands_test.go:1051`. Its replacement exercises the production command handler and checks the result and write count. The other successful direct call runs in a child killed at commit and cannot observe the return value. [Item 2 helper requirement](../plan.md#L87) · [Revised decline call](../../../../src/client/internal/link/commands_test.go#L1081) · [Child call](../../../../src/client/internal/link/commands_test.go#L1269)

Confirmed live. Forcing the helper to return `DecisionRun` on a new decline passed the existing decline tests. A separate helper probe failed on that return. A caller that relied on the first decision would treat a durable expiry as permission to run. The production handler uses a different entry point and was unaffected by this mutation. [Store helper](../../../../src/client/internal/link/receipts.go#L217) · [Handler call](../../../../src/client/internal/link/commands.go#L207)

Retain the handler clock check and separately assert the helper's first decision, empty replay bytes and one committed receipt.

## Post-build scenarios and plan conformance

The plan and the newest findings register contain no item 2 entry marked `UNREVIEWED FIX`. The fold records zero findings assigned to post-build checks. For the post-build upgrade scenario, the signed client fixture replayed a finished receipt and an interrupted start from the archived revision 1 database at zero on two runs. The original event and receipt bytes survived. The database digest matched the test writer's record of the signed revision 1 producer at `44c931d9`. The real server replay and installed upgrade remain integration checks. [Review register](round-1-findings.md) · [Fold result](../../../../.foundry/go-rewrite/results/fold-go-s4-client-folders-r1.md#L9) · [Post-build checks](../plan.md#post-build-checks) · [Legacy producer](../../../../.foundry/go-rewrite/results/s4-client-2-tests.md#legacy-producer-and-numbers)

By reading the code, the event buckets and encoding are unchanged. The store derives the first watermark from its committed counter under the same lock that writes the receipt, retains the uncertain-commit fence, and seals pending starts in one write transaction after validating their envelopes. On retry it loads the saved outcome and watermark. The standing path records a receipt before it applies the revision and sends the reply. The restart send budget remains two seconds. The verified Welcome receive-time sample supplies the pruning lower bound while the proof-send sample remains the admission upper bound. Apart from the findings above, I found no item 2 discrepancy, new race, leaked handle, credential disclosure, or permission change. [Store open](../../../../src/client/internal/link/store.go#L91) · [Receipt write](../../../../src/client/internal/link/receipts.go#L306) · [Commit fence](../../../../src/client/internal/link/store.go#L387) · [Receipt replay](../../../../src/client/internal/link/receipts.go#L244) · [Standing path](../../../../src/client/internal/link/commands.go#L159) · [Restart](../../../../src/client/internal/link/commands.go#L299) · [Clock bounds](../../../../src/client/internal/link/commands.go#L29) · [Welcome sample](../../../../src/client/internal/link/conn.go#L139)

## Frozen assertions and fixture changes

Confirmed live. All fifteen amended tests passed together with the race detector. The package printed `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 17.633s`. I compared both changesets, including the five fixture replacements made after the original freeze. [First test writer](../../../../.foundry/go-rewrite/results/s4-client-2-tests.md) · [Second test writer](../../../../.foundry/go-rewrite/results/s4-client-2s-tests.md)

| Frozen test | Effect on the assertion |
|---|---|
| `TestBeginWritesStartedBeforeReturn` | It adds field-presence and type checks and retains the stored start and expiry checks. The disk read follows `Close` in both versions. |
| `TestStartedFromEarlierRunIsInterrupted` | It adds wire checks for a sealed nonzero watermark and a legacy zero. It retains all former malformed examples, with the independently unchecked version 2 result condition described in post-2 4. |
| `TestFinishPrunesByAge` | It strengthens the trusted-clock, equality, elapsed-time and byte-preservation controls for valid rows. It drops the pruning-path control for a malformed row, as described in post-2 3. |
| `TestCommandStateAndUnknownAction` | It replaces the intentional no-receipt expectations with durable standing checks, including in the removal reply callback. It retains the state revision, conflict, expiry and unknown-action controls. |
| `TestCommandRestartAndRecoveredReceipt` | It prepares the reopened database and adds an assertion for the sealed receipt. The dropped-result and restart checks remain. |
| `TestDeclineDecisions` | It retains the outcomes and commit counts for replay, running and invalid inputs and adds a typed fault and preservation check for corrupt lookup. It drops the first helper decision, as described in post-2 5. |
| `TestCommandBusySlotWritesNoReceipt` | It retains the zero-write ordinary replay and busy controls and separates the first standing write from the zero-write standing retry. |
| `TestDeclineClockOffsets` | It replaces the former early-pruning and rerun expectations as required and now proves replay of the first outcome and watermark in both fast-clock cases after reopen and another write. |
| `TestCommandScanRequiresDurableReport` and `TestCommandScanCancellationAtReportCommit` | They add exact watermark checks and retain the outcome, stream, durability and cancellation assertions. |
| `TestBeginRunsAndReplays` | The understood one-byte outcome replaces the arbitrary multi-byte fixture. Replay and copy isolation still use the same expected bytes before and after reopen. |
| `TestOnlyRunAndFinishWrite` and `TestFinishCommitErrorKeepsStoredState` | The recognized outcome reaches the original write-count and failed-commit fence assertions. |
| `TestBeginRefusesExpiredAndBadInput` | It moves the configurable boundary from eight bytes to one, the length of a supported outcome. The revised test caught a strict size comparison at equality. The excess-size and repeated-finish checks remain. |
| `TestReceiptsKillNeverRunsTwice` | The new outcome lets the child reach the same completion marker. The parent still observes kills, restarts and duplicate effects. |

## Acceptance and planned mutations

I ran the plan's command under Go 1.26.0 on Linux in a disposable copy, as an unprivileged user with the file ownership checks active. The sandbox's root-directory ownership required a container. A native macOS host was unavailable.

```sh
go test -v -race ./src/client/internal/link -run 'TestRevisionTwoReceipt|TestRevisionTwoLegacyStore|TestReviewFRestartHalfClosesBeforeCancel' -count=1
```

All four top-level groups printed `PASS` with zero skips. The groups were `TestRevisionTwoReceiptWatermarks`, `TestRevisionTwoReceiptMetadata`, `TestRevisionTwoLegacyStore` and `TestReviewFRestartHalfClosesBeforeCancel`. The package printed `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 5.133s` and exited zero. After restoring the pinned sources, the race suite also passed in all ten in-scope client packages. The Link package printed `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 59.786s`. Each planned mutation ran in isolation and exited 1 on these frozen assertions.

| Mutation | Diagnostic |
|---|---|
| Resample on resend. | `revision_two_receipts_test.go:190` reported 4 where the first watermark was 3. |
| Use queue length. | `revision_two_receipts_test.go:165` reported 1 instead of 3. |
| Use the acknowledgement mark. | `revision_two_receipts_test.go:165` reported 2 instead of 3. |
| Default a missing watermark to zero. | `revision_two_receipts_test.go:563` reported that preparation accepted corrupt metadata. |
| Reseal an interruption after an append. | `revision_two_receipts_test.go:390` reported a rewrite, and line 396 reported 4 instead of 3. |
| Prune a standing receipt. | `revision_two_receipts_test.go:238` reported lost or changed bytes, and line 242 reported 2 instead of 1. |
| Prune by local wall time. | `commands_test.go:1318` reported that an unrelated write pruned the first result in both clock-offset cases. |

The pinned paired package listed no `TestSprintFourReceiptReplay`. The plan’s PostgreSQL command exited 1. It printed `FAIL github.com/qnlbnsl/lgtm-studio/src/linktest 389.197s` after two inherited groups passed and nine failed. The client-driven cases timed out waiting for Welcome or online presence. The pinned server declares revision 1 and the client requests revision 2. The plan puts the coordinated revision switch and matching fixtures into the integration gate under foundation rule R10. [Client revision](../../../../src/client/internal/link/conn.go#L22) · [Server revision](../../../../src/server/internal/link/handshake.go#L24) · [Paired cutover](../plan.md#L265) · [Foundation rule R10](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L371)

**Grade.** 7 out of 10.
