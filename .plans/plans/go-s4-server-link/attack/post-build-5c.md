# SHIP

The durable readiness check in the claim and send stages accepts a wrong rule, "any run was ever ready", without a single frozen test failing.

The cross-vendor rule is met by this review. The test writer and the builder were Codex workers, and this reviewer is a Claude worker. I reviewed Server Link item 5 on `sprint-0`, in the range `cc944f8f~1..21676fdf` for `src/server/internal/link`. Task `postbuild-go-s4-server-link-5c`, nonce `7c8ca2`. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s4-server-link-5c.md) · [Builder result](../../../../.foundry/go-rewrite/results/s4-link-5-build.md)

The unfinished item 6 test files in the working tree were not reviewed. The two settlement tests that wait for item 6 were skipped, as the shared rules require.

## post-5c 1 · MINOR · The frozen tests accept a readiness check that only asks whether any run was ever ready

The plan requires that commands are claimed and sent only for a ready run, and decision 11 makes the database recheck under the machine lock the fence for that rule. The recheck reads `admitted_run_id IS NOT NULL AND admitted_run_id IS NOT DISTINCT FROM ready_run_id`. [Readiness query](../../../../src/server/internal/link/admission.go#L83) · [Decision 11](../decisions.md)

A mutation that replaced the query with `ready_run_id IS NOT NULL` passed all three item 5 test groups, with exit 0. The claim and send fence tests build their machine with `pendingCommandFixture`, which admits one run on a machine that never had a ready run. So `ready_run_id` is empty in every fence case, and the case that matters is never built. [Fence fixture](../../../../src/server/internal/link/run_gate_test.go#L218)

The case that matters is a machine whose earlier run A is ready and whose newer run B is admitted and still replaying. With the mutation, a claim or send called for B's stream would take the queued command, move the cursor and reach the transport. The stream writer's local gate stops this today, so the failure needs a caller other than the writer or a writer whose local gate disagrees with the database. Decision 11 exists to cover exactly that disagreement.

Fix: give the fence fixture a first run that is ready, then admit a second run with a later first event number, queue a command, and require the same idle claim, unchanged cursor and unsent dispatch that the existing fence tests require.

Design reference: plan item 5 and decision 11.

## post-5c 2 · MINOR · The strict promotion check in the recovery test is still opt-in through a variable that nothing sets

`requireRecoveryReplayReady` promotes the run itself in the fixture unless `LGTM_TEST_LINK_STREAM_PROMOTION` is `1`. The comment says item 4 exercises the shared promotion before the event path calls it in item 5. Item 5 is now built, and nothing in the repository sets the variable. The builder ran the strict mode by hand and it passed. [Fixture](../../../../src/server/internal/link/recovery_v2_test.go#L479)

Confirmed live in a scratch copy: with the promotion call removed from the event write, `TestRecoveryWarningFollowsLaterReadyAdmission` still exited 0 in its default mode. The same happened with the off-by-one acknowledgement mutation. The item 5 tests did catch both mutations, so nothing escaped as a whole. The recovery test alone no longer proves that the event path promotes, and a future change that breaks the promotion only for the warning fence would not be caught by the default run.

Fix: delete the variable and the fixture promotion, so the test always requires that the stream committed readiness before the final acknowledgement. Item 5's builder result records that the strict mode passes.

Design reference: rule 4 of the foundation rules (every write goes through the store's transaction) and plan item 5.

## post-5c 3 · MINOR · The identity recheck before an acknowledgement still leaves a short window, and it costs one read per event

`dispatch` calls `CheckCurrent` after the event commits and after the observer, and sends the acknowledgement right after. The check is its own read, outside any transaction. A replacement or removal that commits between that read and the send still lets an acknowledgement go out on a stream that is no longer current. [Recheck and send](../../../../src/server/internal/link/stream.go#L158)

I did not run this window. I found it by reading. The frozen replacement and removal tests hold the acknowledgement before the check, so they prove the check works and cannot reach the window. The harm is small: an acknowledgement changes nothing on the machine, and the client's own mark does not depend on it. It does differ from the contract's wording that a removed machine's stream receives no acknowledgement after the removal commits. The same check adds one database read to each applied or repeated event, which is up to 32 reads during one replay.

Fix: either state in the contract and the builder notes that the guarantee covers acknowledgements that begin after the commit, or accept the cost and leave it. Do not add a lock to close a window that cannot change stored state.

Design reference: check C11 (buffered events arrive once, in order) and the contract's Removal section.

**Grade** 8 out of 10.

## Post-build scenarios

- The plan marks one place `UNREVIEWED MECHANISM`, the bounded refresh on bulk removal. It belongs to item 6 and to Projects, so no unreviewed place belongs to item 5. [Plan post-build checks](../plan.md#L337)
- Hold the final replay acknowledgement, then replace the stream or remove the machine. Both frozen tests ran against PostgreSQL and passed. The replaced stream ended with `aborted`, the removed stream with `failed_precondition` and `unknown_machine`, and neither sent an obsolete acknowledgement or command. The queued restart stayed unsent after removal and was delivered to the replacement stream.
- Fail the event commit before it commits during replay. The frozen test passed: the mark stayed at 31, no feed row was written, no acknowledgement reached the observer, and the retry on a new stream replayed event 32 and delivered the restart after its acknowledgement.
- Hold three command permits on other machines while a waiting run replays. The test passed: replay and heartbeats continued, and the permit queue showed three used permits and one waiter throughout.
- A failing readiness callback rolled back the report, the mark, the sentinel row and the feed row together, and the retry committed exactly one run transition.

## Item 5 acceptance

- A waiting run replays and is acknowledged before any command: `TestWaitingReplayAcknowledgesBeforeStanding` passed, including approval at a consumed event with the final acknowledgement held.
- The run becomes ready inside the event write: `TestReadyRunNoticeIsTransactional` passed, and removing the promotion call failed seven subtests.
- Commands are claimed and sent only for a ready run: `TestRunBarrierDoesNotUseCommandPermit` passed. Its claim fence keeps a sentinel cursor unchanged, and its send fence writes nothing to the transport. Finding 1 shows that the fence tests do not cover a machine that had an earlier ready run.
- The code matches the plan on every point I checked. Replay never acquires a permit or calls `SendNext`. The gate opens after the final acknowledgement is sent, or after Welcome for a run that is ready at admission. Claim and send recheck readiness under the machine lock. Promotion happens after the mark update and before `Touch`. The builder result explains the one choice the plan left open, `aborted` for replacement.
- I found no data race, leak, swallowed error or secret in a log or error text. The stream handle's two new fields are written before the writer starts. An uncertain admission commit returns `errDatabaseUnavailable` and admits nothing.

## Mutations detected by the frozen tests

Each mutation was applied to a scratch copy of the link package at `21676fdf`, and the three item 5 test groups ran against PostgreSQL. The baseline passed.

| Mutation | Result |
|---|---|
| No readiness check in the claim stage | Failed, `ClaimFence` |
| No identity check before the acknowledgement | Failed, replacement and removal subtests |
| No promotion in the event write | Failed, seven subtests across all three groups |
| Command gate open from the start | Failed, `PendingRun` and `PendingReconnect` |
| Gate opens one acknowledgement late | Failed, three groups |
| Promotion called with the number minus one | Failed, three groups |
| A heartbeat opens the gate | Failed, `PendingRun` and `PendingReconnect` |
| Readiness means "any run was ever ready" | **Passed. Every test accepted it. See finding 1.** |
| Readiness means "a run is admitted" | Failed, `ClaimFence` and `SendFence` |
| The admission mark is not stored on the handle | Failed, the empty prefix and replacement subtests |
| The cursor moves before the readiness check | Failed, `ClaimFence` |

The plan's four named mutations (a permit before the readiness check, a cursor move while gated, a bypassed send recheck, and a standing notice before the last acknowledgement) are covered by the builder's own table. My mutations 1, 4, 7 and 11 are the same class, and each one failed.

## PostgreSQL verification

```sh
deploy/test/with-postgres.sh -- go test -count=1 -p 1 ./src/server/internal/link -run '^(TestRunBarrierDoesNotUseCommandPermit|TestWaitingReplayAcknowledgesBeforeStanding|TestReadyRunNoticeIsTransactional)$'
deploy/test/with-postgres.sh -- env LGTM_TEST_LINK_STREAM_PROMOTION=1 go test -count=1 -p 1 ./src/server/internal/link -run '^TestRecoveryWarningFollowsLaterReadyAdmission$'
deploy/test/with-postgres.sh -- go test -count=1 -p 1 ./src/server/internal/link -run '^TestRecoveryWarningFollowsLaterReadyAdmission$'
deploy/test/with-postgres.sh -- go test -race -count=1 -p 1 ./src/server/internal/link -run '^(TestRunBarrierDoesNotUseCommandPermit|TestWaitingReplayAcknowledgesBeforeStanding|TestReadyRunNoticeIsTransactional)$'
```

All four printed `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/link` with no race report, in 22.9, 14.4, 28.7 and 13.6 seconds. The machine was under heavy load from other teams' full runs, so the times vary.

I did not run the whole link package or the whole server tree. Other teams were running them at the same time, and the builder's result records the six scheduled failures that belong to item 6 and Projects item 8.
