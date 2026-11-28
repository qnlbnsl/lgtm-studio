---
topic: go-hist-server
status: attack
tier: M
created: 2026-11-28
updated: 2026-11-28
epic: go-rewrite
---

# The server reconciles a machine's history when it reconnects

## Overview

A machine can reconnect after losing recent history, or after the server loses a part of the history it once accepted. On an authenticated connection, the server compares its saved event number with the range the machine reports. It either admits the machine at the saved number or raises that number to the last acknowledgement the machine retained. The replacement stream can take work when the server has covered the events from before its run.

When a machine is behind, the server records the outcomes it already received. It fails work that may already have reached the machine and has no known result, so a restart cannot run a second time. Work still waiting for a first send stays queued. An unfinished folder look returns an error to its caller. Failed scans and restarts reach the website through its machine watch, where they are available for ten minutes or until a new request of the same kind is stored.

## Key decisions

The current command record remembers whether the latest attempt was sent. A new attempt overwrites that information. The server will keep one additional record of whether a command ever entered the transport send path. When a process or connection fails at that boundary, the server can know that delivery was possible. It cannot know whether the machine received the frame.

The server will reuse the command ledger and the machine change feed for the website. Expiry hides an entry at its deadline. The server then publishes a saved machine record even if the machine has gone quiet.

The owner approved the behavior on 2026-11-28 in the history design. Full decision log → [decisions.md](decisions.md).

## Workstreams

### The command ledger remembers a possible handoff

The first eligible transport send leaves a durable record. A work item that only reached the queue or claim step stays distinguishable after a server restart. Tests will cut the stream before send and at the send commit to prove both cases.

### Admission reconciles the event mark and settles the ledger

The reset or mark raise commits with the replacement stream. A failed write leaves the previous stream and ledger intact when the database confirms rollback. Tests will read Welcome, the committed results and the next commands through the server's usual entry points.

### A lost folder report ends the website call

A folder look that might have run and still lacks its report returns a stored error to the waiter. A folder report that already finished the operation keeps its reply. Tests will hold actual website calls open while the server admits the older history.

### The machine watch announces failed requests and their removal

A watch receives the failed scan or restart with its time and command identifier. A new request, removal of the machine, or expiry produces a watch change. Tests will read the delivered watch records, including expiry on a machine with no active client.

## Rollout order

1. Land the Contract plan and use its generated fields and amended history base.
2. Build the command handoff item, then the admission item. Complete the folder and watch items with the server's PostgreSQL suite and Go gate.
3. Bring in the Client plan, which can build alongside this plan once the contracts land. The Website plan follows both, removes field 19, and runs the paired checks with PostgreSQL.

---
<!-- ⛔ Implementation details and gate evidence follow. -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1 — The command ledger remembers a possible handoff**

Item 1 of 4, scope `server/backend`. Backend, medium, because the existing send fence needs one durable transition and one bounded recheck. It needs the Contract plan for the first build batch. Items 2 through 4 take its ledger and outcome names.

A claim can commit before the writer starts sending. A later claim resets `dispatched`, and a transport send can succeed before its database write fails. Keep a sticky `link_commands.handoff_started boolean NOT NULL DEFAULT false`. The reset will use `handoff_started OR dispatched`. A row with both flags false has only waited or been claimed. The sticky bit belongs to the command, while `dispatched` still belongs to the current stream and attempt.

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | Queueing or claiming alone leaves the bit false. A command that reached send retains the bit across reassignment and across a lost dispatch commit. | `TestCommandHandoffSurvivesReassignment`, `TestCommandHandoffSurvivesDispatchFailure`. |
| 2 | The schema permits the new terminal outcome for eligible work and rejects it for a claimed-only command or standing notice. | `TestHistoryRestoredCommandSchema`. |

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 1 | Amend schema change 0003 in place. Add the sticky bit. Admit `history_restored` in the terminal outcome constraint and admit it in the dispatch constraint when the bit is true. Require possible handoff, no stage, and an action other than `set_state` for that outcome. Add `OutcomeHistoryRestored SettlementOutcome = "history_restored"` to the existing type. The client outcome enum and staged outcome constraint retain their current members. | `src/server/internal/link/migrations/0003_link_revision_two.sql` (`link_commands_stage`), `src/server/internal/link/migrations/0002_link_commands.sql` (`link_commands_outcome`, `link_commands_result_dispatched`, `link_commands_state_outcome`), `src/server/internal/link/parts.go` (`SettlementOutcome`). |
| 2 | In the current send transaction, do the existing machine, stream, readiness, attempt, terminal, stage and cutoff checks. On the first eligible ordinary send, commit the sticky bit before calling transport. Only when that write reports success, repeat the checks in a second write and use the existing locked send and post-send `dispatched` update. Use the same two-second budget across both writes. An already marked command uses the existing single send write. A standing notice uses that path as well. Keep the existing after-claim seam before either write. Keep the assigned attempt and existing error mapping. | `src/server/internal/link/outbox_send.go` (`State.SendNext`, `State.sendClaimed`), `src/server/internal/link/outbox_result.go` (`State.AcceptResult`, `State.lockCommandStream`). |
| 3 | On assignment, preserve the bit and carry forward any true `dispatched` value into it before resetting the per-attempt flag. Keep a matching dispatched attempt as the condition for accepting a result. | `src/server/internal/link/outbox_claim.go` (`assignCommand`), `src/server/internal/link/outbox_result.go` (`State.AcceptResult`). |

**Exact files.** Change `src/server/internal/link/migrations/0003_link_revision_two.sql`, `src/server/internal/link/parts.go`, `src/server/internal/link/outbox_send.go`, `src/server/internal/link/outbox_claim.go`, and `src/server/internal/link/migration_v2_test.go`. Create `src/server/internal/link/history_handoff_test.go`.

**Tests written first.** In `history_handoff_test.go`, `TestCommandHandoffSurvivesReassignment` stores two ordinary commands and stops one after claim. It lets the other send and verifies the peer receives that attempt. It replaces the stream and pauses a later attempt after claim. Using committed rows, it proves the first command still has no handoff and the second retains its handoff even while `dispatched` is false. `TestCommandHandoffSurvivesDispatchFailure` cuts the dispatch commit before delivery to PostgreSQL after a peer reads the frame. It proves the bit remains set and a matching result still requires `dispatched`. Its control cuts the marker commit before delivery and proves there is no frame. It also covers the second write losing the current stream after the marker commit and before the transport call. `TestHistoryRestoredCommandSchema` starts from the assembled schema and checks valid ordinary terminal transitions from both durable handoff forms, invalid transitions from claimed-only, staged or standing work, and exclusion of the new outcome from a staged result. Use the same terminal field clearing as `finishCommand` for valid fixtures. For the invalid staged transition, check the terminal-plus-stage invariant. In `migration_v2_test.go`, have the writer also update `TestLinkRevisionTwoMigration` through `linkMigrationOldRows` to compare prior fields while ignoring the new handoff column.

**Risky mutations.** Move the marker to claim, move it to the dispatch commit, clear it on reassignment, or send without the final current-stream recheck. The handoff and replacement assertions must fail for the corresponding mutation. Allowing the bit alone to authorize a result must fail the result assertion.

| Risk | Severity | Mitigation |
|---|---|---|
| A restart reaches a client before its dispatch write is durable. | High | Commit possible handoff before the transport call. The current attempt still requires a successful dispatch write before a result can be accepted. |
| Admission wins between the marker and the transport call. | Medium | Recheck in the existing send fence. Treat that narrow interval as delivery uncertainty, so reset favors avoiding a second execution. |

**Verify** with `TestCommandHandoffSurvivesReassignment`, `TestCommandHandoffSurvivesDispatchFailure`, `TestHistoryRestoredCommandSchema`, `TestCommandSendRechecksBeforeWrite`, `TestCommandSendDeadline`, `TestCommandBacklogYieldsToHeartbeats` and `TestCommandSendPoolHeadroom`. The environment uses the disposable PostgreSQL wrapper. Device checks are unnecessary.

**Interfaces and design.** Item 2 uses `link_commands.handoff_started` and `link.OutcomeHistoryRestored`. Item 4 uses the terminal outcome and the existing `finished_at`. This item supports H3 and H4 and foundation rules R4 and R6.

**WS-2 — Admission reconciles the event mark and settles the ledger**

Item 2 of 4, scope `server/backend`. Backend, medium, because it changes the existing locked admission transaction and reuses command settlement and run promotion. It needs item 1. Item 3 and item 4 need it. They may run in parallel with each other once it lands.

The server checks the reported range after Proof verifies. It decides under the same machine row lock that serializes a result, an event, a send and a removal. The following rules apply to the verified machine alone.

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 3 | Contradictory or unrepresentable numbers get `invalid_argument` and the incumbent keeps its connection, mark, ledger and feed head. Invalid Proof wins over a validly encoded bad history. | `TestRevisionTwoProofAndRunAdmission`. |
| 4 | A raised mark appears in Welcome and consumes only the covered stages. Replay continues from the next number and sends no work before its run prefix is covered. A normal replay keeps its former mark and waits at its barrier. | `TestAdmissionRaisesHistoryMark`, `TestRunAdmissionPersistsAcrossRestart`, `TestRunBarrierDoesNotUseCommandPermit`. |
| 5 | The older client gets the unchanged mark and a ready run. It can send fresh reports. The previous pending boundary no longer gates it. Staged outcomes survive. Previously handed scan and restart commands fail once. The claimed-only and waiting work and the unsettled standing notice remain eligible in their respective order. | `TestAdmissionResetsOlderClientHistory`, `TestResultAndRunAdmissionUseOneMachineLock`. |
| 6 | An admission rollback preserves the incumbent and the previous ledger. Losing Welcome after a commit leaves one terminal settlement per command and fresh admission on redial. Recovery warning storage and projection are gone. | `TestAdmissionHistoryCommitReplyLost`, `TestAdmissionResetsOlderClientHistory`, `TestLinkRevisionTwoMigration`. |
| 7 | An aged reset outcome stays terminal through the timeout pass and is removed by the existing terminal retention pass. | `TestHistoryRestoredSurvivesCommandSweep`. |

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 4 | Carry `RunAnnouncement{ID, FirstEventNumber, AcknowledgedEventNumber, LastEventNumber}` from the verified Hello. Require a canonical version 7 universally unique run identifier, first in `[1, 2^63]`, and last and acknowledged in `[0, 2^63-1]`. Require acknowledged ≤ last and first minus one ≤ last. Check first before subtraction and bounds before signed conversion. Keep the run-pair check and refusal of a known superseded run. Reconnect of the latest run repeats first and may report fresh values for the other two numbers. | `src/server/internal/link/handshake.go` (`Service.handshake`), `src/server/internal/link/admission.go` (`RunAnnouncement`, `classifyRun`). |
| 5 | Read the mark, latest pair and pending boundary under the machine lock. Apply the three cases below before committing the new connection. Save the final mark with `admitted_run_id` and generation in the existing upsert, including first admission where the old mark is zero. Keep the saved first-minus-one pair per run. Reset replaces the old pending pointer with the admitted run and promotes it immediately. Keep the superseded row for future refusal. | `src/server/internal/link/presence.go` (`State.BecomeCurrent`), `src/server/internal/link/admission.go` (`classifyRun`, `State.promoteRun`, `runReady`). |
| 6 | Settle the selected commands through `finishCommand` and the same registered part routes. Do this after the connection is stored and before the admission and readiness callbacks. Check and propagate their errors. After known commit, log the reconciliation in the handshake before Welcome and before the after-admission seam. Keep unknown admission commit handling through the normal Hub cleanup path. | New `src/server/internal/link/history_admission.go` follows `src/server/internal/link/inbox.go` (`State.settleCoveredResults`) and `src/server/internal/link/outbox_result.go` (`State.finishCommand`). `src/server/internal/link/handshake.go` (`Service.handshake`), `src/server/internal/link/parts.go` (`PartHooks`). |
| 7 | Remove the warning-only error and refusal write, its generation column in schema change 0003, and the warning read and clear conditions in admission, promotion and presence. Remove `machines.Presence.EventHistoryNeedsRecovery` and the assignment in the machine projection. Keep connection generation for stream fencing. Use the existing promotion callback only when the ready pointer changes. | `src/server/internal/link/migrations/0003_link_revision_two.sql` (`recovery_set_generation`), `src/server/internal/link/admission.go` (`errRunBehindMark`, `recoveryDiagnosticError`, `classifyRun`, `State.promoteRun`), `src/server/internal/link/presence.go` (`State.BecomeCurrent`, `State.Presence`), `src/server/internal/machines/link.go` (`Presence`), `src/server/internal/machines/project.go` (`ProjectWithPresence`). |
| 8 | Keep the terminal row in the existing retention query and out of the pending timeout query. Use the existing terminal result path to clear payload and stage, record finish time and notify registered parts with a nil event watermark for the synthesized end. | `src/server/internal/link/outbox_result.go` (`State.finishCommand`), `src/server/internal/link/outbox_sweep.go` (`expiredCommandPage`, `finishedCommandPage`, `State.sweepCommandBatch`). |

**Comparison and write order.**

1. If the mark is below acknowledged, raise it to acknowledged. Call `settleCoveredResults` for stages at or below the new mark. Keep stages above it and leave result-free commands pending. The new run's own readiness still uses its prefix. Log `link_history_mark_raised` with `machine_id`, `old_mark` and `new_mark`.
2. Otherwise, if the mark is above last, keep it. Read and settle every staged result in watermark and sequence order, including stages above the retained mark. Pass the original outcome and watermark. Then settle pending, unstaged, ordinary commands with possible handoff in command sequence order, using `OutcomeHistoryRestored`. A never-handed command or a result-free standing notice stays pending. Previously terminal commands keep their outcome. Mark this run ready using its saved prefix. Log `link_history_restored` with `machine_id`, `mark` and `client_last_event_number`. The terminal selection and feed writes make repeat admission safe.
3. Otherwise, keep the mark and normal replay. An unseen run whose announced first number is at or below the mark is ready without the old pending and staged admission fences. For a higher first number in the normal case, keep the existing unseen-run checks against staged watermarks and a pending run's boundary. A ready run uses first minus one as its replay threshold. A newer run whose prefix is still ahead waits through its own threshold.

Only cases 1 and 2 log a reconciliation line. The checks in the raise and reset cases use the new range rule and skip the old staged and pending refusal tests. A replacement still fences the prior stream from the instant the write commits. Reports and acknowledgements use the existing number-first handling, including rejection of a gap. The state touch in the admitting write serves the machine feed. A known rollback publishes no history change or reconciliation log. An ambiguous commit returns the existing unavailable status, and the next admission reads the durable result.

**Exact files.** Change `src/server/internal/link/{admission.go,handshake.go,presence.go,migrations/0003_link_revision_two.sql,machinetest/machine.go,admission_v2_test.go,recovery_v2_test.go,migration_v2_test.go,settlement_joined_v2_test.go,run_gate_test.go}` and `src/server/internal/machines/{link.go,project.go}`. Create `src/server/internal/link/history_admission.go` and `src/server/internal/link/history_admission_test.go`. The scripted peer gains explicit `AcknowledgedEventNumber, LastEventNumber uint64` fields. Tests should give a healthy peer the highest fixture event already present on that peer, with at least first minus one. The helper `submitAdmission` can use the observed fixture mark for ordinary tests of persistence and replay, and takes explicit range input through a second helper for mismatch and validation cases. Do not infer missing numbers in production. In `run_gate_test.go` update the three direct nonempty-prefix fixture literals and the literal in `testLatestAdmittedRun` with their known last number. Keep the warning-free signed watch helpers in `recovery_v2_test.go` that the existing waiting replay test uses.

**Tests written first.**

- `admission_v2_test.go`, `TestRevisionTwoProofAndRunAdmission`. Verify signed invalid cases for acknowledged above last, first minus one above last, each range edge above the signed storage bound, first zero, and a valid empty history. Accept last = acknowledged = 2^63−1 and first = 2^63. Retain malformed run, unsupported revision, invalid Proof, noncanonical identifier and incumbent invariance checks. For H9 use an incumbent with an ordinary pending command and compare its state, stream, command row and feed head.
- `history_admission_test.go`, `TestAdmissionRaisesHistoryMark`. Admit with old mark 5, acknowledged 8, last 12 and first 11. Stage results at 7, 8 and 9 before admission. Before Welcome, check mark 8, original outcomes and watermarks at 7 and 8, a pending stage at 9, an ordinary result-free command untouched and a second machine unaffected. Expect one log with the three specified fields. Hold work until events 9 and 10 commit. Check settlement at 9, same-run readiness at 10 and command delivery after its final replay acknowledgement. Include a first connection with no Link row and a normal replay control with acknowledged ≤ mark ≤ last. In a second normal control, announce an unseen first of 6, last of 12 and acknowledged of 8 at mark 10, with an old pending boundary and staged watermark above 10. It is ready at admission, keeps the mark and all unsettled outcomes, and can replay from 11.
- `history_admission_test.go`, `TestAdmissionResetsOlderClientHistory`. Start with mark 10, a previous ready run and a pending run with replay-through 13. Announce first 6, last 5, acknowledged 0. Before consuming fresh events, observe the unchanged Welcome mark, the new admitted and ready run, exactly one readiness callback and rejection of the old run after reconnect. Observe one reset log with the specified fields. Create genuinely dispatched scan and restart, an earlier handoff whose latest assignment has `dispatched = false`, a claimed-only command, another never-claimed command, a standing notice, and staged outcomes above and below the retained mark. Check durable outcome, finish time, nil watermark for reset settlements, original stage watermarks for staged settlements, exact callback counts and order, and absence of failed identifiers on the new stream and a redial. Read the standing notice and both waiting commands in their existing order, using different kinds or completing each same-kind head. Send fresh Facts and HarnessesFound at 11 and 12 and observe their acknowledgements and an Online projection. Where field 19 still exists, check absence of a true value using the message descriptor by field number. Assert the database column is absent.
- `recovery_v2_test.go`, replace the two warning tests with `TestAdmissionHistoryCommitReplyLost` and move reusable, warning-free fixture helpers only if referenced. Exercise rollback before commit and a known reset commit with Welcome blocked before transport, then redial the same pair and values. Also keep an unknown commit reply subcase. Observe committed mark, previous or new connection as appropriate, terminal rows, transaction-backed callback counts and the exact feed change at the transaction boundary. Maintenance or redial may remove an ambiguous stream according to the existing Hub fence. Verify command settlement never repeats and allow a new admission log for the new admission.
- `settlement_joined_v2_test.go`, replace the warning-dependent portions of `TestResultAndRunAdmissionUseOneMachineLock`. Drive both lock orders against reset. When a valid staged result wins it keeps its outcome. When admission wins it settles the possible handoff and the old result is rejected as stale. Observe the contested PostgreSQL lock before releasing the winner. Keep a normal-path staged versus pending fence control in `TestRunAdmissionPersistsAcrossRestart`.
- `history_admission_test.go`, `TestHistoryRestoredSurvivesCommandSweep`. Put a history-restored command past the ordinary pending timeout with a recent finish time and observe no second callback or change in outcome. Age its terminal time beyond seven days, run the existing sweep and observe deletion. Include a neighboring eligible pending timeout and ensure it still settles.

**Existing tests to replace or adapt.** The frozen-file inventory is in [scouting.md](scouting.md). The writer updates these before the build and establishes a new freeze for the affected assertions.

| File | Existing test or helper | Reason |
|---|---|---|
| `src/server/internal/link/admission_v2_test.go` | `TestRevisionTwoProofAndRunAdmission`, including `AtMaximumMark` and `ScriptedProcessRepeatsItsPair`. | Replace the behind-mark refusal with maximum-mark admission. Supply valid ranges for the replay peer and preserve the proof and validation controls. |
| `src/server/internal/link/admission_v2_test.go` | `TestRunAdmissionPersistsAcrossRestart` and `submitAdmission`. | Supply an explicit or observed valid range to preserve the pending and staged normal-case controls, immutable pairs, and the callbacks and restart checks. |
| `src/server/internal/link/recovery_v2_test.go` | `TestRecoveryWarningFollowsLaterReadyAdmission` and `TestRecoveryDiagnosticCommitReplyLost`. | Remove the warning-only expectation and obsolete helpers. Replace lost-commit controls with the real admission and reset cases. |
| `src/server/internal/link/migration_v2_test.go` | `TestLinkRevisionTwoMigration`. | Replace column-exists, warning generation and negative-generation checks with column absence. Retain schema 3, row preservation, run-fence and removal assertions, and compare the prior fields using the helper already adjusted in item 1. |
| `src/server/internal/link/settlement_joined_v2_test.go` | `TestResultAndRunAdmissionUseOneMachineLock` and `TestLinkRemovalSettlesAndCallsParts/FlaggedMachineRollsBackAndRetries`. | Replace the warning-dependent race and replace the flagged removal fixture with `StagedMachineRollsBackAndRetries`. Keep rollback, staged outcome, and part-operation assertions. |
| `src/server/internal/link/run_gate_test.go` | `TestRunBarrierDoesNotUseCommandPermit` and `TestReadyRunNoticeIsTransactional`. | Supply last numbers to direct admissions to keep the replay barrier and stale-acknowledgement tests exercising the original scenarios. |

The shared `submitAdmission` helper also serves existing `TestStagedResultsSettleInReplayOrder`, `TestStagedResultsAndWriterTurns`, `TestWaitingReplayAcknowledgesBeforeStanding`, `TestFolderReportPrecedesSettlement`, `TestRevisionTwoEventBudgetsAndAcks`, and `TestLookAtFolderCommandBudgets` in the link package. Their test bodies, names and outcome assertions can stay as they are. The simpler Hello fixtures with a boundary of one can use empty history when their observation concerns the connection or Welcome and has no pending handed-over work. No runtime compatibility handling for omitted high numbers is needed.

**Risky mutations.** Move reset or raise before Proof, use the wrong machine identifier for selection, move the history write outside admission, skip the pending-run promotion, fail staged work with the synthesized outcome, filter reset settlements by `dispatched` only, fail claimed-only or standing work, or call a settlement hook for a terminal row on redial. Mutate the normal replay gate to send before the last replay acknowledgement. The decision-order, transaction, callback, readiness and command-frame assertions must detect their corresponding mutations.

| Risk | Severity | Mitigation |
|---|---|---|
| A reset and an old stream result can select incompatible outcomes. | High | Use one machine lock. Test both lock orders and verify the winning result, terminal row and old-stream fence. |
| A mark raise can settle work or release the run too early. | High | Keep uncovered stages, exercise a second machine and wait for the new run's remaining replay prefix before releasing its first command. |

**Verify** with the tests in rows 3–7, `TestEventTable`, and `TestReadyRunNoticeIsTransactional`. Use signed scripted peers and the disposable PostgreSQL wrapper. Device checks are unnecessary.

**Interfaces and design.** The wire uses `Hello.acknowledged_event_number` (6), `Hello.last_event_number` (7), and `Welcome.last_event_number`. Keep `State.BecomeCurrent(ctx context.Context, machineID, streamID, address string, announcement RunAnnouncement) (Stream, error)` and existing `PartHooks` signatures. The history checks are H1, H2, H3, H4, H5, H6, H7, H8, H9 and the server projection part of H11. This item preserves foundation D5, D7, R3, R4, R6 and C10–C12.

**WS-3 — A lost folder report ends the website call**

Item 3 of 4, scope `server/backend`. Backend, low, because an existing settlement hook and stored failure mapping carry the change. It needs item 2 and may run beside item 4.

A folder operation already ends through the registered settlement route when its command's outcome arrives without a valid folder report. Add the history-restored outcome to that route. The waiting website call reads the durable failure on its next poll.

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 8 | A handed folder look without a report fails while its original call waits. A lost report with an already staged result fails the folder call and preserves the command's own outcome. A finished report keeps its reply. A never-handed look can still complete. | `TestProjectCallEndsAfterHistoryRestore`. |
| 9 | Machine removal rolls back or completes while a staged folder operation is open, including the large repository refresh. | `TestProjectStartupRepositoryWatch/LargeStagedRemoval`. |

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 9 | Route `history_restored` to the existing `no_answer` failure for an open matching folder operation. Use the same stored error code `deadline_exceeded` and sentence `The machine did not answer the folder request.` A stage settled as `done` with no report already takes that route. Keep the outcome-specific mapping for other real outcomes. A completed operation ignores later settlement. | `src/server/internal/projects/reports.go` (`Service.SettleCommand`, `Service.ApplyFolderSeen`), `src/server/internal/projects/errors.go` (`storedFolderFailure`), `src/server/internal/projects/operations.go` (`waitFolderOperation`), `src/server/internal/app/projects_hooks.go` (`registerProjectHooks`). |
| 10 | Replace the artificial warning setup in the large-removal test with the existing prepared stage and open operation. Retain the 257-repository fixture, rollback and retry, and command and operation checks. | `src/server/internal/app/projects_test.go` (`TestProjectStartupRepositoryWatch`, `testProjectAppLargeRemoval`). |

**Exact files.** Change `src/server/internal/projects/reports.go` and `src/server/internal/app/projects_test.go`. Create `src/server/internal/app/history_projects_test.go`. The existing frozen `TestProjectStartupRepositoryWatch/LargeFlaggedRemoval` is replaced by `LargeStagedRemoval` because its refused-run warning setup is obsolete. Update `testProjectAppLargeRemoval` with the replacement setup. Its unflagged `LargeRemoval` control keeps the same coverage.

**Test written first.** In `history_projects_test.go`, `TestProjectCallEndsAfterHistoryRestore` uses the real application and a signed scripted peer. Start a keyed folder call, read its command, hold back the report and wait for a durable dispatched attempt. Restore with a last number below the server's mark. Assert the same waiting call returns the stored `deadline_exceeded` with the fixed sentence within the observation window, well before its own 25-second timeout. Arm the database fault through the existing application fixture connection configuration. The durable command is `history_restored`, its operation is failed with `no_answer`, and the same request key returns that error without queueing new work. Repeat with an eligible staged `done` above the old mark and verify the command ends `done` while the folder request fails. In separate controls, send a report before reset and prove the reply stays available by key. Hold another request before transport handoff, then report it on the ready stream and prove the original call succeeds. Use matched machine, operation and command identifiers and an actual part registration in the application. Repeat the failure check through both InspectFolder and AddRepository so the website's dialog path is observed.

**Risky mutations.** Drop the settlement mapping, remove the open-state predicate, or settle the operation outside the Link write. Test with a later failing admission hook or a pre-commit admission fault to show the folder call and operation remain open when reset rolls back. Prove each mutation violates a matching assertion.

| Risk | Severity | Mitigation |
|---|---|---|
| A call waits until its ordinary timeout or a successful folder record is overwritten. | High | Assert the stored operation and the prompt response together. Preserve the open-state predicate and test a successful report followed by a reset. |

**Verify** with `TestProjectCallEndsAfterHistoryRestore`, `TestProjectStartupRepositoryWatch`, and `TestFolderReportPrecedesSettlement` against the disposable PostgreSQL environment. Device checks are unnecessary.

**Interfaces and design.** Keep `projects.Service.SettleCommand(ctx context.Context, tx *store.Tx, machineID, commandID, outcome string) error`, `folder_operations.state`, and `folder_operations.failure = 'no_answer'` as the call boundary. This item covers the server portion of H5 and foundation R3, R4, R6 and R8.

**WS-4 — The machine watch announces failed requests and their removal**

Item 4 of 4, scope `server/backend`. Backend, medium, because arrivals, retries, removal and idle expiry publish into the same ordered feed. It needs item 2 and may run beside item 3.

Use command rows as the source for the list. One nullable deadline represents an entry eligible for display and expiry publication. Clearing it records that the entry has left while keeping the original command outcome available for the usual command retention.

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 10 | A snapshot or saved update has one entry per eligible scan or restart with the command identifier, kind and finish time. The list survives a restart within its lifetime. Staged and standing work, folder looks and other outcomes are absent. | `TestWatchClientsListsRestoredRequests`. |
| 11 | A new same-kind request, removal, and the ten-minute expiry each remove the entry from a live watch. A rolled-back or replayed request key cannot dismiss an entry. An idle expiry needs no other machine event. | `TestWatchClientsDismissesRestoredRequestsOnRetryAndRemoval`, `TestWatchClientsExpiresRestoredRequestsWhileIdle`. |

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 11 | In schema change 0003 add `link_commands.failed_request_until timestamptz` and a partial due index over `(failed_request_until, machine_id, seq)` for rows with a deadline. Require a terminal `history_restored` outcome on scan or restart whenever the deadline is non-null. Require a finish time before that deadline. In `finishCommand` sample the database clock once for `finished_at` and the deadline. Set the deadline only for those two kinds. Item 2's same-write machine touch publishes arrival. | `src/server/internal/link/migrations/0003_link_revision_two.sql` (`link_commands_stage`), `src/server/internal/link/outbox_result.go` (`State.finishCommand`), `src/server/internal/link/presence.go` (`State.BecomeCurrent`). |
| 12 | Load failures for the requested machine identifiers, with a batched query in the same repeatable read as the watch's snapshot or saved record. Use one database time per query and `failed_request_until > time`. Sort by `finished_at DESC, seq DESC`. Map scan and restart to the matching generated enum values. Put the list on `machines.Presence.FailedRequests []*lgtmv1.FailedClientRequest` and copy it into the client projection even when the stream is offline. Use `WatchingPresence` to avoid loading the list for action eligibility and other callers. | New `src/server/internal/link/failed_requests.go` follows `src/server/internal/link/presence.go` (`State.Presence`). `src/server/internal/machines/link.go` (`Presence`), `src/server/internal/machines/project.go` (`ProjectWithPresence`), `src/server/internal/machines/watch.go` (`WatchClients`, `WatchingPresence`), `src/server/internal/watch/watch.go` (`Serve`, `readChange`). |
| 13 | In the locked successful enqueue write, clear deadlines of earlier failures for the same machine and kind. Work queued before that failure does not dismiss it. Touch Machines once if the update cleared a row. Run the clear only after the insert succeeds. Store request receipt replay never reaches that insert. In `Forget`, clear outstanding deadlines for the removed machine in the removal write. The registry already publishes the removed key. | `src/server/internal/link/outbox.go` (`State.EnqueueCommand`), `src/server/internal/link/presence.go` (`State.Forget`), `src/server/internal/machines/commands.go` (`Service.queueConnectedAction`), `src/server/internal/machines/store.go` (`Registry.Touch`, `Registry.Delete`). |
| 14 | Start a small expiry pass with the existing Link lifecycle. Scan the due index in bounded pages at startup and on a one-second tick. Advance its page cursor past busy or failed machines and retry them on the next pass. Group per machine and use the sweep's no-wait machine lock pattern. Recheck the selected deadline in the write, clear it and touch Machines once per successful machine batch. Busy machines are revisited on a later tick and cannot prevent other machines' expiry. A fresh snapshot hides a due entry even before that pass. Join the worker on shutdown. Keep the hourly command sweep cadence and terminal retention separate. | New `src/server/internal/link/failed_requests.go` follows `src/server/internal/link/outbox_sweep.go` (`State.sweepCommandBatch`, `State.RunCommandSweep`). `src/server/internal/link/run.go` (`Service.Run`). |

**Exact files.** Change `src/server/internal/link/{migrations/0003_link_revision_two.sql,outbox_result.go,outbox.go,presence.go,run.go,migration_v2_test.go}` and `src/server/internal/machines/{link.go,project.go}`. Create `src/server/internal/link/failed_requests.go` and `src/server/internal/link/failed_requests_test.go`. Set the private `State.failedRequestLifetime time.Duration` to ten minutes in `NewState`. Configure it before workers start in package tests. Drive short expiration via that value and the real worker or an injected tick patterned on `WithCommandSweepTicks`. A deadline is stable across server restarts. The test writer updates the existing frozen `TestLinkRevisionTwoMigration` through `linkMigrationOldRows` so its historical row comparison excludes this new nullable column. Keep `WatchClients`, the generic watcher, and existing signed-in action entry points.

**Tests written first.** In `failed_requests_test.go` use a signed-in Machines watch, signed scripted clients and an observer against PostgreSQL.

- `TestWatchClientsListsRestoredRequests` opens a watch before reset and sees scan and restart entries with the exact command identifiers, kinds and database finish times in the saved frame. Verify the standard deadline equals finish plus ten minutes without waiting. Check that the schema rejects a deadline on a pending row. Include a stage, standing notice, folder look, ordinary failure and another machine. Confirm a restarted server's snapshot includes only the eligible entries. Lose a reset Welcome, redial and verify no second entry for the same identifier and no changed finish time. A same-kind command queued before failure does not suppress the entry.
- `TestWatchClientsDismissesRestoredRequestsOnRetryAndRemoval` first observes two kinds on one machine and the same kind on another. Enqueue the scan through the signed-in action after fresh facts permit it, using a fresh request key. Read a saved frame with the scan gone and the restart and other machine unaffected. Prove a failed or rolled-back enqueue and replay of a key from before a later failure leave that entry. Read a removed-key frame on machine removal and a fresh snapshot without the machine. Check the deadlines and feed for retry/removal serialization.
- `TestWatchClientsExpiresRestoredRequestsWhileIdle` uses a short configurable lifetime and sends no traffic during expiry. Subscribe before reset and observe the arrival before letting the expiry pass publish. Read the entry first, then the saved frame that removes it. Opening a fresh watch after the exact deadline also yields no entry, including when the publishing write is held. Verify one expiry change, no new settlement call, and continued expiry for a second machine while the first machine is locked. Retry the first batch after releasing its lock. Verify an expiry write that rolls back leaves the deadline available for retry.

**Risky mutations.** Remove the arrival or expiry feed reference, clear a deadline in a write separate from enqueue, clear both scan and restart kinds on a scan enqueue, skip the deadline recheck, or use a memory-only expiry list. The live watch, rollback, competing removal and restarted snapshot assertions must catch the respective changes.

| Risk | Severity | Mitigation |
|---|---|---|
| The bell remains stale after an idle expiry or after a retry. | High | Publish under the same write as the departure, assert the delivered watch frames, and test a fresh snapshot at the exact deadline. |
| A busy machine or failed feed write delays expiry for other machines. | Medium | Use bounded pages and a no-wait lock. Test a locked machine and a separate expiring machine in the same pass. |

**Verify** with the three watch tests, `TestWatchClientsSharedFollower`, and the Machines and Link package suites. The environment uses the disposable PostgreSQL wrapper. Device checks are unnecessary.

**Interfaces and design.** `Client.failed_requests` (20) contains `FailedClientRequest{id, kind, failed_at}` with `FAILED_CLIENT_REQUEST_KIND_SCAN_HARNESSES` or `FAILED_CLIENT_REQUEST_KIND_RESTART`. `id` is `link_commands.id` and `failed_at` is `finished_at`. The normal Machines saved and removed frames carry the changes. This item covers the notice side of H3, H4, H8 and H12 and foundation D5, R3–R5 and C12.

**Rollout detail for steps 1–3.** From the repository root after Contract lands, read `.foundry/go-rewrite/link-base.txt` and use its `LGTM_LINK_BASE`, currently `699ce4889434422458580c3ea94820f62d533bf9` before the amendment. The orchestrator supplies the amended commit for the build. Use the commands below for the standalone build. Expect both to exit zero and the server suites to run their PostgreSQL tests.

```sh
. .foundry/go-rewrite/link-base.txt
env -u LGTM_TEST_POSTGRES LGTM_LINK_BASE="$LGTM_LINK_BASE" bash scripts/check-go.sh
deploy/test/with-postgres.sh -- go test -count=1 ./src/server/...
```

The first command that runs a gate omits the database variable so the old paired tests skip their execution while the tree still compiles. The second command gates all server packages on PostgreSQL and must run their database cases. On the final merged tree after the Website plan replaces the old paired fixtures, run `deploy/test/with-postgres.sh -- env LGTM_LINK_BASE="$LGTM_LINK_BASE" bash scripts/check-go.sh`. After each risky mutation, rerun its selected test with the wrapper and `-count=1`. Expect its named assertion to fail, restore the implementation, and rerun the test before acceptance. The writer records the failing assertion and new hashes of the changed tests before the builder starts.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe is the six numbered deliveries in the Server assignment, H1 to H12 in `docs/specs/2026-11-28-server-authority-on-history-design.md`, the relevant foundation decisions D5 and D7, rules R3, R4, R5, R6, R8 and R10, checks C10, C11 and C12, and issue 1714.

| Source item | Workstream | Status |
|---|---|---|
| Delivery 1, admission comparison | WS-2 | Server proof. |
| Delivery 2, older-client reset | WS-1, WS-2, WS-3 | Server proof. |
| Delivery 3, older-server raise | WS-2 | Server proof. |
| Delivery 4, terminal outcome, callbacks and sweep | WS-1, WS-2, WS-3 | Server proof. |
| Delivery 5, failed requests | WS-4 | Server proof. |
| Delivery 6, warning removal | WS-2 | Server projection and schema proof. |
| H1 | WS-2 | Scripted older history, Welcome, fresh reports and Online. Website plan owns the paired proof. |
| H2 | WS-2 | Empty-range server acceptance. Website plan owns the missing-file paired proof. |
| H3 | WS-1, WS-2, WS-4 | Handed and queued scan paths and failed notice. Website plan owns the paired proof. |
| H4 | WS-1, WS-2, WS-4 | No restart command re-offer and correct notice kind. Website plan owns the paired proof. |
| H5 | WS-2, WS-3 | Staged result and waiting folder call. Website plan owns the paired proof. |
| H6 | WS-2 | Normal replay and command barrier. Website plan owns the paired proof. |
| H7 | WS-2 | Covered stages, retained suffix and Welcome at raised mark. Website plan owns the paired proof. |
| H8 | WS-2, WS-4 | Redial keeps terminal outcome and one notice. Website plan owns the paired proof. |
| H9 | WS-2 | Signed malformed-range tests. |
| H10 | Outside Server | The Client plan owns the file check and local exit. |
| H11 | WS-2 | The server stops projecting the field. Website owns the screen and field reservation. |
| H12 | WS-4 | Live list, enqueue, removal and expiry frames. Website owns the bell and repeat action. |
| Foundation D5 | WS-2, WS-4 | Retain the saved state and ordered feed. |
| Foundation D7 | WS-2 | Keep machine key proof ahead of an admitting write. |
| Foundation R3 | WS-2, WS-3, WS-4 | Use part interfaces for other parts' records. |
| Foundation R4 | WS-1, WS-2, WS-3, WS-4 | Keep changes within Store writes. |
| Foundation R5 | WS-4 | Use the common Watch source and follower. |
| Foundation R6 | WS-1, WS-2, WS-3 | Keep every affected command in the Link ledger. |
| Foundation R8 | WS-3 | Return the established Connect error sentence. |
| Foundation R10 | WS-1, WS-2, WS-3, WS-4 | Run server proofs against PostgreSQL and hand paired checks to Website. |
| Foundation C10 | WS-2 | Keep authentication and refusal precedence. |
| Foundation C11 | WS-2 | Exercise repeat, gap and replay ordering with real event writes. |
| Foundation C12 | WS-2, WS-4 | Build gate plus real PostgreSQL server tests for joined history and feed writes. |
| Issue 1714 | WS-2 | Authenticated machine-scoped mark write. The hardening note is in the planner handoff. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, What this project is. The server is written in Go and the parts meet through the contract. | The server consumes generated Go types after the Contract plan and publishes through the existing watch. |
| `AGENTS.md`, Where things are. The old engine and mock are references and contract changes start in `src/contracts/proto/`. | The Server scope is `src/server/`. Requests for wire and website changes go to their respective plans. |
| `AGENTS.md`, Habits 1 and 2. Credit copied code and record findings in the repository. | The plan reuses local code. The bounded result records the existing issue 1714 and hands dependencies to the orchestrator. |
| `AGENTS.md`, Habits 3 and 4. Work reaches the base through a pull request and the Herdr Synapse project is outside the push scope. | The planner hands off the files locally. The orchestrator owns integration. |
| `AGENTS.md`, Writing style for documents. Keep decisions dated and private details out. | The plan uses dated, attributed decisions and repository-relative paths. Verify it with the private-data check and direct review. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1, command handoff | backend | `server/backend` | medium | A durable send boundary extends the current command fence and ledger constraints. | Yes. It changes the transport write path. | Contract. |
| 2, admission and warning removal | backend | `server/backend` | medium | The existing admission transaction combines the three history cases with settlement and readiness. | Yes. It changes verified stream admission. | Item 1. |
| 3, folder settlement | backend | `server/backend` | low | One additional outcome follows the existing route for a stored error. | Yes. It changes the outcome of a callable website operation. | Item 2. |
| 4, failed-request list and expiry | backend | `server/backend` | medium | A projection and a bounded expiry worker reuse the command ledger and machine feed. | Yes. It changes the watch and enqueues. | Item 2. |

Items 3 and 4 edit disjoint files and can run together. Items 1, 2 and 4 edit the Link schema in order. The scope file is `.plans/scopes/server/backend/scope.json`.

Load. One vendor supplies this plan's planner, test writers, builders and code reviewers. Budget one planner, four test writers, four builders and four code reviewers, for thirteen launches from that vendor before any repeated reviews. The orchestrator schedules the launches against the vendor's remaining allowance, which is unavailable to this assignment, and owns the planning review round.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | Pending. | The orchestrator runs the plan review after planner self-check. |

</details>
