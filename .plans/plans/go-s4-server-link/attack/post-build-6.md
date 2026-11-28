# REVISE

A recurring timeout callback failure on one machine prevents the sweep from expiring later machines’ commands and from pruning any old terminal records.

The test writers, builder, and reviewer were OpenAI Codex workers. The reviewer did not write the code or tests. This review covers Server Link item 6 on `sprint-0`, at commit `35e1f8ec`, using `d36d0c3e~1..35e1f8ec` for the Link diff. The early tests were read from the pinned commit. The task identifier is `postbuild-go-s4-server-link-6`, and the nonce is `272220`. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s4-server-link-6.md) · [Builder result](../../../../.foundry/go-rewrite/results/s4-link-6-build.md)

## post-6 1 · MAJOR · A timeout callback failure stops unrelated command maintenance

**Statement.** Plan item 6 preserves the hourly timeout sweep and seven-day terminal retention. A settlement callback should roll back the machine’s batch when it fails. [Plan item 6](../plan.md#L233)

**Evidence and failure scenario.** Each batch already has its own transaction. The new timeout callback can return an error from that transaction, and the existing batch loop returns immediately on that error. `SweepCommands` then exits before running terminal retention. In a PostgreSQL probe, machine A had the oldest due command and a callback that always failed. Machine B had a due command with a healthy callback and an eight-day-old terminal record. Two consecutive sweeps each returned A’s error. A’s write rolled back, B’s command stayed pending without a callback, and its old terminal record survived. The hourly worker retries the same ordering, so a persistent failure keeps causing the same outcome. [Timeout and retention entry point](../../../../src/server/internal/link/outbox_sweep.go#L49) · [Batch loop](../../../../src/server/internal/link/outbox_sweep.go#L96) · [Timeout callback](../../../../src/server/internal/link/outbox_sweep.go#L138) · [Hourly worker](../../../../src/server/internal/link/outbox_sweep.go#L154)

**Fix.** Keep the failed timeout batch rolled back and eligible for a later retry. Remember that machine’s error, continue to the remaining machines and run terminal retention before returning the error to the worker. Add a two-machine PostgreSQL test that checks both healthy writes while the failed batch remains intact.

## post-6 2 · MINOR · The frozen replay tests accept settlement under the wrong machine’s lock

**Statement.** Plan item 6 requires a result to settle by its own machine’s committed mark. Foundation check 11 requires replayed client events to arrive once and in order. [Event and settlement order](../plan.md#L237) · [Foundation check 11](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L388)

**Evidence and failure scenario.** The built query correctly filters by machine. Removing that filter passed all eight focused test groups, including the two frozen settlement files and the admission and removal controls. The staged-watermark admission test checks a second machine’s Welcome. The joined replay test advances only the machine that owns its three stages. In a two-machine scratch probe, the pinned code left B’s staged command intact after A applied event 1 and settled it only after B applied event 1. Under the mutation, A’s event made B’s command terminal while B’s mark was still zero. [Built replay query](../../../../src/server/internal/link/inbox.go#L90) · [Second-machine admission control](../../../../src/server/internal/link/admission_v2_test.go#L296) · [Joined replay fixture](../../../../src/server/internal/link/settlement_joined_v2_test.go#L24)

**Fix.** Put a staged command on B in the replay fixture. Advance A to the same number, then assert that B retains its original outcome, watermark, payload, and dispatch assignment. Advance B to that number and require exactly one settlement.

## Post-build scenarios and item 6 acceptance

| Scenario | Observation at the pinned commit |
|---|---|
| Stage three results, restart, repeat, sweep, and replay. | A single PostgreSQL scratch probe staged results through `AcceptResult` with watermarks 9, 9, and 7 in insertion order. It repeated a conflicting result before restart and again on the new stream. All three survived the sweep and retained their original assignment and attempt. The signed stream acknowledged events 6 through 9. The hooks ran once each, in order 7, 9, 9, with command sequence breaking the tie. The joined frozen test confirmed that the settlement hook could read the Facts report inside the same write. A later ready-hook error rolled back the final mark, settlements, part writes, and readiness. [Post-build check](../plan.md#L342) · [Frozen replay test](../../../../src/server/internal/link/settlement_joined_v2_test.go#L54) |
| Remove a flagged machine with a stage and 257 repositories. | The review register marks the bounded repository refresh as unreviewed and assigns it to Projects. At the pinned commit the Projects directory has its schema and migration tests. The real cleanup, watch reopen, and natural lag scenario require the Projects removal and watch handlers. The Link portion ran against PostgreSQL. The flagged removal fixture proved rollback with the stage and warning intact, then a successful retry that retained terminal commands, cleared the old Link rows, committed only the Machines and part feed references, and refused a late event. [Review register](round-1-findings.md#L55) · [Projects boundary](../../go-s4-server-projects/plan.md#L199) · [Flagged Link removal](../../../../src/server/internal/link/settlement_joined_v2_test.go#L502) |
| Race real folder report application against timeout and removal, and fault Projects after its callback. | These scenarios require the registered folder route and Projects handler. The pinned Link tests passed both result-versus-removal orders, both result-versus-admission orders, and the selected-before-stage and timeout-first sweep cases. The replay test also checked withheld acknowledgement and retry when a hook failed during the write. [Builder handoff](../../../../.foundry/go-rewrite/results/s4-link-6-build.md#L22) · [Plan scenarios](../plan.md#L343) |

By reading the built result path, approval, exact stream, assignment, attempt, dispatch, and terminal state are checked under the machine lock before the first eligible result is staged or settled. The write preserves the standing-notice outcome mapping and records a stage wake once. Claim, deferred scan lookup, send, and both sweep selections exclude stages. Replay collects its due rows before calling hooks and places settlement after the report and mark and before run promotion. Removal settles unfinished rows and calls the lifecycle hooks before deleting Link state. The terminal write preserves the original action for the callback. The sweep failure above is the production difference from item 6’s acceptance and retention rules. [Result and terminal write](../../../../src/server/internal/link/outbox_result.go#L18) · [Claim and deferred scan](../../../../src/server/internal/link/outbox_claim.go#L96) · [Send recheck](../../../../src/server/internal/link/outbox_send.go#L131) · [Replay write](../../../../src/server/internal/link/inbox.go#L64) · [Removal write](../../../../src/server/internal/link/presence.go#L308)

The sweep reports a fixed warning. The staged commands and their payloads stay in PostgreSQL, and the event log records a machine identifier and mark. Issue 1699 already tracks the inherited acknowledgement window and readiness fixture gaps. [Sweep warning](../../../../src/server/internal/link/run.go#L88) · [Event log](../../../../src/server/internal/link/stream.go#L144) · [Issue 1699](../../../../docs/issues.md#1699-three-smaller-points-of-the-servers-replay-and-promotion)

## PostgreSQL test and mutation output

The repository PostgreSQL wrapper ran `go test -v -count=1 -p 1 ./src/server/internal/link -run "$selector"` at the pinned commit, with the selector encompassing the eight tests below. The same command ran again with `-race`. Both runs exited 0 with zero skips. The normal run printed `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/link 15.643s`. The race run printed `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/link 41.247s`, with no race report.

```text
--- PASS: TestRunAdmissionPersistsAcrossRestart (3.05s)
--- PASS: TestCommandResultAndRemoval (2.21s)
--- PASS: TestStagedResultsSettleInReplayOrder (0.64s)
--- PASS: TestStagedResultsAndWriterTurns (1.68s)
--- PASS: TestResultAndRunAdmissionUseOneMachineLock (0.91s)
--- PASS: TestLinkRemovalSettlesAndCallsParts (2.79s)
--- PASS: TestStagedResultSurvivesRestartAndSweep (0.53s)
--- PASS: TestResultFenceKeepsAttemptAndCommandRules (3.79s)
```

The builder’s compatibility selector also exited 0 with zero skips across nine tests. The Link, Machines, and application packages printed `ok` in `14.962s`, `4.524s`, and `5.322s`. Both frozen settlement files matched their recorded checksums. The pinned Link diff passed `git diff --check`. The deliberately red paired and Projects router tests were outside these invocations. [Compatibility selector](../../../../.foundry/go-rewrite/results/s4-link-6-build.md#L53) · [Deliberately red tests](../../../../.foundry/go-rewrite/common-build.md#L41)

Each mutation ran independently through a Go overlay in one disposable copy and used real PostgreSQL. The removed sweep predicate failed both the real-result and preseeded variants of the selected-then-staged race.

| Mutation | Exit | Test observation |
|---|---:|---|
| Allow stages in the reconnect claim page. | 1 | The next writer turn did not deliver the expected command. [Assertion](../../../../src/server/internal/link/settlement_joined_v2_test.go#L226) |
| Keep a staged scan at the deferred head. | 1 | The successor did not arrive in the next writer turn. [Assertion](../../../../src/server/internal/link/settlement_joined_v2_test.go#L200) |
| Allow stages in the sweep page. | 1 | The page selected more than the aged ordinary command. [Assertion](../../../../src/server/internal/link/migration_v2_review_test.go#L86) |
| Allow stages in the locked sweep batch. | 1 | Both race variants lost the stage after selection. [Assertion](../../../../src/server/internal/link/settlement_v2_test.go#L313) |
| Accept a conflicting duplicate for a stage. | 1 | The first stage changed or settled early. [Assertion](../../../../src/server/internal/link/settlement_v2_test.go#L252) |
| Settle before advancing the mark. | 1 | The hook observed the old mark. [Assertion](../../../../src/server/internal/link/settlement_joined_v2_test.go#L133) |
| Promote the run before settlement. | 1 | The hook rejected the ordering and the stream did not acknowledge event 9. [Assertion](../../../../src/server/internal/link/settlement_joined_v2_test.go#L129) |
| Reverse the sequence of results at the same watermark. | 1 | The callback order differed. [Assertion](../../../../src/server/internal/link/settlement_joined_v2_test.go#L148) |
| Skip removal hooks on an empty ledger. | 1 | The lifecycle observations and feed references were missing. [Assertion](../../../../src/server/internal/link/settlement_joined_v2_test.go#L591) |
| Permit enqueue after the registry row is gone. | 1 | The removal retry failed the required missing-machine refusal. [Assertion](../../../../src/server/internal/link/settlement_joined_v2_test.go#L544) |
| Remove the explicit staging predicate from the locked send query. | 0 | The `NOT dispatched` check still rejected the stage. The schema requires every stage to be dispatched. Removing both predicates produced a transport write and exited 1. [Schema constraint](../../../../src/server/internal/link/migrations/0003_link_revision_two.sql#L18) · [Send assertion](../../../../src/server/internal/link/settlement_joined_v2_test.go#L261) |
| Settle another machine’s stages during replay. | 0 | All eight frozen and inherited groups passed. The two-machine scratch control passed on the pinned code and failed with the mutation, as described in post-6 2. |

**Grade. 8 out of 10.**
