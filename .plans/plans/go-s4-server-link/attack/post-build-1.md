# SHIP

The frozen tests still pass when the report dispatcher discards a part callback error that should roll back the write.

The frozen tests and the build were both written by OpenAI workers, the same vendor as this reviewer. I reviewed item 1 on `sprint-0` at `358eb541`. The frozen tests are from `8574e2dd`. The server at the reviewed commit speaks Link revision 1. [Revision constant](../../../../src/server/internal/link/handshake.go#L24)

Task `postbuild-go-s4-server-link-1`. Nonce `9ce8fa`.

## Finding

### post-1 1 · MINOR · The report test leaves callback errors unchecked

Foundation rule 4 and the plan require each part callback to share the Link transaction and abort it on error. The shipped dispatcher returns the report callback's error. The registration test only deliberately fails an admission callback. Its report callback takes the success path. In the scratch copy, replacing `return hook(ctx, tx, machineID, event)` with `_ = hook(ctx, tx, machineID, event)` left both frozen tests and all five enqueue subtests passing. The mutated test command exited 0. [Foundation rule 4](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L365) · [Plan](../plan.md#L87) · [Report dispatcher](../../../../src/server/internal/link/parts.go#L93) · [Admission failure](../../../../src/server/internal/link/parts_test.go#L449) · [Report dispatch in the test](../../../../src/server/internal/link/parts_test.go#L474)

Once a caller wires folder reports through this dispatcher, a part could return a domain error after writing a folder record and recording a feed change. With the mutation, the caller could then commit those changes, advance the event mark and acknowledge the report. Add a report callback case that writes a sentinel and feed reference before returning a domain error. Assert that the caller receives the error and that both sentinels and the feed reference roll back. [Planned report transaction](../plan.md#L248) · [Acknowledgement rule](../plan.md#L250)

**Grade** 9/10.

## Post-build checks

The plan and the round 1 register identify the Projects feed refresh for large removals as the unreviewed mechanism. It belongs to item 6. The item 1 registration scenario passed at the Link boundary. Two parts received their own typed routes and shared the caller's write. The lifecycle callbacks ran in registration order, and a duplicate in either route map left the registration untouched. The startup refusal before serving is assigned to the startup gate. The paired alias scenario requires the folder route from item 7. [Round 1 register](round-1-findings.md#L55) · [Item 6](../plan.md#L238) · [Post-build scenarios](../plan.md#L337) · [Startup ownership](../plan.md#L133) · [Builder's result](../../../../.foundry/go-rewrite/results/s4-link-1-build.md#L19)

## Plan acceptance

- By reading, the adapter delegates to `EnqueueCommand`. Enqueue checks that the machine exists, inserts in the caller's transaction, and records one wake per machine per write. The existing approval, rename, scan and reconnect paths take the machine row lock before enqueue. The claim path takes that lock too. [Queue](../../../../src/server/internal/link/outbox.go#L21) · [Approval](../../../../src/server/internal/machines/store.go#L128) · [Rename](../../../../src/server/internal/machines/records.go#L121) · [Scan and reconnect](../../../../src/server/internal/machines/commands.go#L42) · [Claim](../../../../src/server/internal/link/outbox_claim.go#L45) · [Claim lock](../../../../src/server/internal/link/outbox_result.go#L41)
- Confirmed live, the second writer waited on the same machine row. The revision 1 stream delivered both commands in commit order, with the identifiers stored by their callers. One write committed and delivered 257 commands with one wake. A forced rollback left no commands, caller records, feed references or delivery. Enqueue after deletion in the same transaction returned `ErrUnknownMachine` with an empty identifier, and the deletion committed alone. [Commit order](../../../../src/server/internal/link/parts_test.go#L156) · [Bulk enqueue](../../../../src/server/internal/link/parts_test.go#L183) · [Rollback](../../../../src/server/internal/link/parts_test.go#L213) · [Missing machine](../../../../src/server/internal/link/parts_test.go#L245)
- By reading, `NewState` owns both route maps. Registration checks both maps for duplicates before copying routes and appending lifecycle hooks. Each dispatcher uses the passed context and transaction and returns callback errors. The startup diagnostic contains fixed text. [State construction](../../../../src/server/internal/link/presence.go#L43) · [Registration and dispatch](../../../../src/server/internal/link/parts.go#L62)

## Mutations on PostgreSQL

| Scratch change | Observed result |
|---|---|
| Enqueue caller B's command before its machine lock. | Exit 1. The test printed `Caller B allocated a command sequence before acquiring the machine lock.` |
| Move enqueue into a separate Store write. | Exit 1. The test printed `The caller rollback left command rows or caller records committed.` |
| Discard the report callback error. | Exit 0. Both frozen item 1 tests passed. |

The first two mutations ran the corresponding `TestPartEnqueueIsAtomicAndOrdered` subtest. The third ran both frozen tests. All three changed files were restored and compared with the pinned originals before the verification rerun. [Plan mutations](../plan.md#L136)

## PostgreSQL verification

The following command ran against a disposable PostgreSQL database in the restored copy of `358eb541`. It exited 0 with eight named tests passing and zero skips.

```sh
bash deploy/test/with-postgres.sh -- go test -v -count=1 -p 1 \
  ./src/server/internal/link ./src/server/internal/machines ./src/server/internal/store \
  -run '^(TestPartEnqueueIsAtomicAndOrdered|TestLinkPartRegistration|TestEnqueueStoresBeforeSend|TestScanAndReconnectQueueCommands|TestApproveQueuesStateAtomically|TestRenameQueuesStateOnce|TestRecord(Once)?ValidationAndLimit)$'
```

```text
--- PASS: TestEnqueueStoresBeforeSend (2.90s)
--- PASS: TestPartEnqueueIsAtomicAndOrdered (5.44s)
--- PASS: TestLinkPartRegistration (0.42s)
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/link      8.771s
--- PASS: TestScanAndReconnectQueueCommands (1.25s)
--- PASS: TestApproveQueuesStateAtomically (2.85s)
--- PASS: TestRenameQueuesStateOnce (3.01s)
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/machines  7.145s
--- PASS: TestRecordValidationAndLimit (0.32s)
--- PASS: TestRecordOnceValidationAndLimit (1.90s)
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/store     2.237s
```

The three Link tests also passed with the race detector enabled. The command exited 0 with zero skips and no race diagnostics.

```sh
bash deploy/test/with-postgres.sh -- go test -race -v -count=1 -p 1 \
  ./src/server/internal/link \
  -run '^(TestPartEnqueueIsAtomicAndOrdered|TestLinkPartRegistration|TestEnqueueStoresBeforeSend)$'
```

```text
--- PASS: TestEnqueueStoresBeforeSend (3.25s)
--- PASS: TestPartEnqueueIsAtomicAndOrdered (6.95s)
--- PASS: TestLinkPartRegistration (0.43s)
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/link     11.668s
```
