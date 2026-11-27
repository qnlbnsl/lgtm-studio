---
topic: go-s4-server-link
status: attack
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# The server applies a machine's history before sending new work

## Overview

Sprint 4 lets you add a repository from a real machine. This plan builds the server side of the connection that Projects uses to ask about a folder and receive what the machine found.

When the client restarts, the server lets it replay events from the previous process before handing it new work. When a result arrives ahead of its report, the server saves the result and waits. If the server restarts during that wait, it still applies the report first and settles the command once.

The server accepts revision 2. An existing revision 1 client needs the updated installer with its local history kept. If a new process announces history older than the server's saved mark, the Machines watch carries a recovery warning. Restoring the missing history can let a later connection clear it.

## Key decisions

The server keeps the latest admitted process and the last ready process in the database. It also remembers the identifiers of superseded processes until the machine is removed. Two processes can announce the same first event number, so the number alone cannot identify the one allowed to reconnect.

When a command result refers to an event the server has yet to apply, the server saves the result beside its command. A restart keeps both the result and its place in history. If the client lost the required history, the result waits for actual replay or machine removal. The Machines watch shows when the server refused a process whose announced history was below its saved mark.

The link calls the server parts inside the same write that changes the link. Projects can save a folder before the result for its command settles. A failed write rolls both changes back. [Decision log](decisions.md)

## Workstreams

### Server parts can store a command with their own records

Projects can obtain the identifier of a stored command while it saves a folder operation. Two commands for the same machine arrive in commit order. The upgrade preserves the prior command ledger and event mark.

### A new client process drains the history that came before it

The client proves its identity with the existing signed challenge. The server uses the announced process and event number to decide when to send its first command. A held command send on another machine cannot block replay acknowledgements. A failed replacement can raise the recovery warning while the incumbent continues to run.

### A folder report and its command result settle in order

A folder report reaches Projects before a result that depends on it. A server restart cannot cause a saved early result to be resent or swept. Machine removal ends the remaining work in the same write that removes the machine.

### The assembled server exposes the same behavior

Startup applies the migration and supplies the same link state to the stream handler and the Machines service. The server test fixture can interrupt a write after Projects has handled a report. The assembled test then observes that no folder, event mark, command outcome or acknowledgement survived that failed write.

## Rollout order

1. Land the reviewed revision 2 contract and generated Go files. Review the server part interface with Projects before its build consumes it.
2. Freeze the new tests and the explicit revisions to prior tests. Build the server changes against a database made by sprint 3. Interrupt startup after the migration and verify that the server starts from that database again.
3. Assemble Link, Machines and Projects in one server. Verify both programs together on PostgreSQL, then run the stack checks. The deployed server refuses the old link revision, so update the client with the sprint 3 installer while keeping its local events and receipts.
4. Use the upgraded client to replay old events and a saved result. Observe one report application and no rerun of the receipted command. Add a repository through the deployed website and reload it.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1. Server parts can store a command with their own records**

The database machine lock orders writes that enqueue commands. The same link schema holds the durable admission and result state used by the later workstreams.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | A part commits its own record with the returned command identifier. A second same-machine writer queues after it, and a rolled-back writer leaves no command or wake reference. | `TestPartEnqueueIsAtomicAndOrdered`, `TestLinkPartRegistration`. |
| 2 | A sprint 3 schema upgrades with its rows and payloads intact. The new schema rejects invalid run boundaries and staged result combinations. | `TestLinkRevisionTwoMigration`, `TestCommandMigrationUpgradesPartB`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Publish the transactional part interface and retain the Machines queue adapter. | `src/server/internal/link/outbox.go`, `State.Enqueue`. `src/server/internal/link/presence.go`, `State` and `NewState`. `src/server/internal/machines/link.go`, `CommandQueue`. |
| 2 | Append the migration for runs, the warning and staged results. | `src/server/internal/link/migrate.go`, `Migrations`. `src/server/internal/link/migrations/0002_link_commands.sql`, `link_commands_terminal_pair`. `src/server/internal/link/outbox_test.go`, `TestCommandMigrationUpgradesPartB`. |

**Item 1. Transactional part interface and ordered enqueue.** Change `src/server/internal/link/outbox.go` and `presence.go`. Add `src/server/internal/link/parts.go` and `parts_test.go`. Item 2 can run in parallel. This implements Link interface points 1 to 3, foundation decisions D2, D4 and D5, foundation rules 3, 4, 6 and 7, and newer check R8.

- Publish `func (s *State) EnqueueCommand(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) (string, error)`. The caller locks the machine with `(*machines.Registry).Lock` in that write before locking any other domain row. The caller keeps the returned identifier only if the write commits. The old `State.Enqueue` delegates to that implementation and discards the identifier, preserving `machines.CommandQueue` and the existing call sites. Use the existing sequence allocation, expiry clock and `machine_outbox` saved reference.
- The machine-row-first rule covers approval and rename that enqueue `SetState`, both Machines actions, a fresh Projects request that queues `LookAtFolder`, and same-machine hook writes that enqueue. Store's caller-and-key advisory lock is taken before the fresh callback. Its existing receipt-row lock runs only on a replay, which enqueues nothing. The feed lock occurs at the end. A gated stream cannot skip an uncommitted earlier command because both append and claim use the machine row. A command for another machine can receive an interleaved sequence number.
- Publish `func (s *State) RegisterPart(hooks PartHooks) error`. Call it during startup before link or sweep goroutines are started. Install named routes as one registration, copy caller-supplied maps, reject duplicate command or report routes with a startup error, and permit several parts to receive lifecycle calls in registration order. The callbacks receive the same `*store.Tx` and context as the write. They may write their own tables and record feed references. They return an error to roll back the whole write. A callback runs again if the previous attempt rolled back. Establish eligibility checks in the part and use nil for a valid no-op. Callbacks may enqueue for the locked machine, including from admission or a run change. They must not open a nested write, take another machine lock, wait for a client, or do external work.

```go
type CommandKind string
const CommandLookAtFolder CommandKind = "look_at_folder"
type ReportKind string
const ReportFolderSeen ReportKind = "folder_seen"

type SettlementOutcome string
const (
    OutcomeDone SettlementOutcome = "done"
    OutcomeFailed SettlementOutcome = "failed"
    OutcomeExpired SettlementOutcome = "expired"
    OutcomeUnsupported SettlementOutcome = "unsupported"
    OutcomeInterrupted SettlementOutcome = "interrupted"
    OutcomeTimedOut SettlementOutcome = "timed_out"
    OutcomeRemoved SettlementOutcome = "removed"
)

type Settlement struct {
    MachineID string
    Command *linkv1.Command
    Outcome SettlementOutcome
    LastEventNumber *uint64
}

type Admission struct {
    MachineID, StreamID, RunID string
    FirstEventNumber uint64
    Ready bool
}

type RunChange struct {
    MachineID, PreviousRunID, RunID string
    FirstEventNumber uint64
}

type PartHooks struct {
    CommandSettled map[CommandKind]func(context.Context, *store.Tx, Settlement) error
    ReportApplied map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error
    StreamAdmitted func(context.Context, *store.Tx, Admission) error
    RunChanged func(context.Context, *store.Tx, RunChange) error
    MachineRemoved func(context.Context, *store.Tx, string) error
}
```

- The string on `ReportApplied` and `MachineRemoved` is the authenticated machine identifier. `Settlement.Command` contains the stored declared action, command identifier, attempt and expiry. `LastEventNumber` is present for a client result, including zero, and absent for timeout or removal. `Admission.Ready` describes whether the replay boundary is already covered in the admission write. `PreviousRunID` is empty for a machine's first ready run. The registry keys allow another part to add command and report kinds in later revisions. Projects only needs the folder report, folder settlement and removal callbacks.
- Write `TestPartEnqueueIsAtomicAndOrdered` in `parts_test.go` first. In a test part, hold transaction A immediately after it inserts a command and writes a sentinel using its returned identifier. Start B for the same machine and observe that its first domain row lock waits. Release A. Use the sprint 3 direct command-send fixture with the existing restart action and an admitted stream. Read both sent command identifiers in order and match both rows and the test part's committed records. The assembled revision 2 wire check runs after item 5. Force another transaction to roll back after enqueue and require no row, feed reference or delivery for that identifier. Also check that the existing error-only interface is satisfied at compile time.
- Write `TestLinkPartRegistration` in the same file first. Register two parts with different routes and lifecycle listeners. Add an overlapping route and expect a startup error and no partial registration. Change a caller's map after registration and expect dispatch to keep the original route. Use a real database write with a sentinel row to verify callback context and transaction identity.
- The item is risky because a command can pass another one if any caller allocates its sequence before taking the machine lock. Mutate the enqueue path in the test caller to lock late and prove B can pass A or that the lock-order assertion fails. Mutate the enqueue path to use a separate write and prove the rollback assertion fails. The legacy adapter's existing callers and their lock anchors are listed in the scouting record.

**Item 2. Upgrade the link schema.** Add only `src/server/internal/link/migrations/0003_link_revision_two.sql` and `migration_v2_test.go` under Link. Adjust the legacy migration fixture in `outbox_test.go` and its old-schema seeding only. No earlier migration bytes change. Item 1 is independent. This implements D4 and D5, foundation rules 1, 4 and 7, and newer R9 to R11.

- Add nullable `link_machines.admitted_run_id` and `ready_run_id` as text. Add nullable nonnegative bigint `recovery_set_generation`. A present generation represents a true recovery flag. The initial state of each field is null. Add `link_runs(machine_id text, run_id text, replay_through bigint)` keyed by `(machine_id, run_id)` and constrained to nonnegative boundaries. Reference `link_machines(machine_id)` with removal cascading into the history. `replay_through` is the first number minus one, so the allowed first event number of 2 to the power of 63 fits without a signed overflow. Preserve one row per admitted run. Superseded identifiers are retained to reject replay at equal boundaries.
- Extend the action constraint to `look_at_folder` and add nullable `link_commands.staged_outcome` and `staged_event_number`. Require both staged fields together, a positive event number, a client outcome, successful dispatch with a stream assignment, and an unfinished command with its payload. Keep standing notice expiry mapping and terminal invariants. Rebuild `link_commands_pending` and `link_commands_expirable` with the stage predicate. Index `(machine_id, staged_event_number, seq)` for pending results. A final transition clears both staged fields.
- Write `TestLinkRevisionTwoMigration` in `migration_v2_test.go` first. Build the old store using migrations 1 and 2 of Link as a restricted manifest. Seed an approved and a waiting machine, a connection, mark 42, a dispatched and an unsent command, a terminal command, and a keyed receipt. Take a before image of every old column and byte payload. Apply the current manifest twice, and reopen the store. Expect version 3, the same old columns, null run and stage metadata and the original receipt. Expect a manifest that ends at 2 to be refused. Assert that the new action and a valid stage can be inserted. Reject a half-stage, a stage on an undispatched or terminal row, a negative boundary and a duplicate run identifier for that machine. Test removal of the link machine cascades the run rows while old terminal commands stay.
- Keep the old 1-to-2 assertion in `TestCommandMigrationUpgradesPartB` by passing an explicit manifest ending at 2. Update the live-schema version assertions in `TestBecomeCurrent`, `TestServerCommandSweepLifecycle` and `TestServerRestartsAfterInterruptedCommandMigration` to expect version 3. Seed its version 1 connection and mark with version 1 statements. The new runtime admission requires tables introduced later.
- The item is risky because a bad default could hide existing commands or abandon an event mark. Mutate to reset the mark and to allow half a staged result. Require the migration test's old-record and constraint assertions to fail.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A command identifier escapes from an aborted write, or a faster request skips a slower request's command. | High | Share the transaction and lock the machine before any other domain row. Hold real concurrent writes in the test. |
| The migration resets old history or changes the eligibility of an old command. | High | Migrate stored version 2 data and compare all prior columns, including attempt, mark and payload. |

**Verify.** Run the three named tests with PostgreSQL. Run the existing Machines command tests as a compile and compatibility check. Device quality assurance is covered by rollout step 4.

**WS-2. A new client process drains the history that came before it**

A stream can reconnect with the same run or present a new run whose retained history must be drained first. A candidate refused for losing committed history reports a problem without becoming the current stream.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3 | Only revision 2 is admitted. History checks are behind proof, and the admitted run and its boundary survive restart. | `TestRevisionTwoProofAndRunAdmission`, `TestRunAdmissionPersistsAcrossRestart`. |
| 4 | The recovery warning appears in a live watch and a new snapshot. Incumbent traffic leaves it set. A later ready admission clears it, and an uncertain diagnostic commit keeps the incumbent current. | `TestRecoveryWarningFollowsLaterReadyAdmission`, `TestRecoveryDiagnosticCommitReplyLost`. |
| 5 | Replay proceeds with three command permits occupied. Neither a queued command nor a standing notice is claimed before the last required acknowledgement. | `TestRunBarrierDoesNotUseCommandPermit`, `TestWaitingReplayAcknowledgesBeforeStanding`, `TestReadyRunNoticeIsTransactional`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Authenticate and classify revision 2 admissions, then persist the accepted pair. | `src/server/internal/link/handshake.go`, `Service.handshake`. `src/server/internal/link/presence.go`, `State.BecomeCurrent`. `src/server/internal/link/hub.go`, `Hub.Admit`. `src/server/internal/link/machinetest/machine.go`, `Machine.handshake`. |
| 4 | Publish the history recovery warning in Machines. | `src/server/internal/link/presence.go`, `State.Presence`. `src/server/internal/machines/project.go`, `ProjectWithPresence`. `src/server/internal/machines/watch.go`, `Service.WatchClients`. `src/server/internal/link/handshake_test.go`, `TestAdmissionCommitReplyLost`. |
| 5 | Open the send gate on admission or after the required prefix. | `src/server/internal/link/stream.go`, `Service.serve` and `Service.dispatch`. `src/server/internal/link/inbox.go`, `State.ApplyEvent`. `src/server/internal/link/outbox_claim.go`, `State.claimNext`. `src/server/internal/link/outbox_send.go`, `State.sendClaimed` and `commandSlots.acquire`. |

**Item 3. Revision 2 authentication and run admission.** Change `src/server/internal/link/handshake.go`, `presence.go` and `machinetest/machine.go`. Add `src/server/internal/link/admission.go` and `admission_v2_test.go`. Update only the old admission fixtures listed below. Needs items 1 and 2. Item 6 can start in parallel on its outbox files, and item 7 waits for item 5. This implements D7, foundation rules 4, 7 and 8 and C6 to C11, newer Link interface point 3, R9 and R11, and the contract's proof and run rules.

- Set `LinkRevision = 2`. Set the Welcome revision from that constant. Use the sentence `The server speaks revision 2 of the link.` for `errLinkRevision`. Preserve the bytes in the `lgtm-link-v1 server` and `lgtm-link-v1 machine` labels. A well-formed initial identity and nonce gets a signed Challenge. Preserve the order of registry unavailable, unknown machine, revision mismatch, and then proof errors. The revision 1 client must verify the Challenge before treating `failed_precondition` with `Lgtm-Link-Refusal: link_revision_mismatch` as final. Check revision 2 history only after a valid machine Proof. A history refusal is the existing retryable `invalid_argument`.
- Validate the run as a canonical version 7 universally unique identifier (UUID). Validate the first number in the inclusive range from 1 to 2 to the power of 63 before subtracting one. Define `RunAnnouncement{ID string, FirstEventNumber uint64}` in Link and pass it as the final argument to `State.BecomeCurrent`. Under the Machines row lock read the mark, latest admitted pair, latest ready run and largest watermark still staged. Preserve the current stream and its command ledger if admission is refused.
- Classify a known run first. Only the latest admitted run with the original boundary is eligible. This includes a pending run after server restart and a ready run whose mark has since advanced past its original boundary. Refuse any known superseded run or changed pair before evaluating the numeric fences. For an unseen run, refuse if `first_event_number <= event_mark` and mark that reason for item 4. Otherwise require it to start strictly above each staged watermark and at or above the pending run's first number. An unseen run at an equal pending boundary may replace it. A refused unseen run is not added to `link_runs`.
- Use the admission write to save the run identifier and boundary, advance the existing stream generation, and install its connection row. Invoke `StreamAdmitted` on each successful admission, including a pending stream and a same-run reconnect. Define the ready transition helper here. If the mark is at least the replay boundary, store the ready identifier after the admission callback and invoke the ready-run callback. Item 5 reuses that helper when an event finishes the replay. Return Welcome only after confirmed admission and the existing hub publication check. Preserve the prior genuine-admission uncertain-commit fallback and the existing exact-stream cleanup.
- Update `machinetest.Machine` with `RunID string` and `FirstEventNumber uint64`. `Join` initializes a fresh run and first number 1. Repeated `Connect` on the same scripted process sends that pair. Tests of a new run set an explicit fresh pair. Keep independent proof construction. Use constants local to the test fixture so it does not import the production Link package.
- Write `TestRevisionTwoProofAndRunAdmission` in `admission_v2_test.go` first. Use both existing transport modes. A revision 1 client verifies the Challenge and sees the exact mismatch trailer without Welcome. A revision 2 bad signature with a behind-mark announcement gets `unauthenticated` and no row or feed change. A valid signed revision 2 client gets revision 2 Welcome. Test invalid UUID versions, zero and the first value above the upper bound, and acceptance of the upper bound with a seeded mark at its predecessor. An announcement one below the boundary covered by the maximum mark is refused without signed overflow.
- Write `TestRunAdmissionPersistsAcrossRestart` in the same file first. Admit A ready at first 6 with mark 5. Admit B at first 10 and restart mid-drain. Refuse A and a changed B pair, accept B at 10, reject an unseen C at 9 solely because of the pending fence, then accept C at 10. Verify B remains refused after restart. Independently admit two ready runs at first 6 with mark still 5 and verify the older run is refused, including after restart. Test an unseen first number equal to a staged watermark with a smaller mark, and its successor. Use item 2's schema to seed the stage until item 6 runs.
- The item is risky because authenticating too late or forgetting an old run can change the current connection. Mutate to check history before verifying Proof, to drop the stored pair on disconnect, and to compare only numeric boundaries. Require the corresponding signed, restarted and equal-boundary tests to fail.

**Item 4. Recovery warning and incumbent preservation.** Change `src/server/internal/link/admission.go`, `handshake.go` and `presence.go`, and `src/server/internal/machines/link.go` and `project.go`. Add `src/server/internal/link/recovery_v2_test.go`. Needs item 3. Item 6 may continue in its outbox files. This implements the Contract team's recovery request, foundation D5 and rules 3 to 5 and 7, and supports R9 and R10.

- In the behind-committed-mark branch of the admission write, save the current `link_machines.stream_generation` in `recovery_set_generation`. Call `registry.Touch` when the flag changes from false to true. If it was already true, refresh the generation fence on each qualifying refusal. Return nil from the write to commit the diagnostic and return `ErrInvalidMessage` to the candidate only after the confirmed commit. Carry the diagnostic branch classification out of the write even if the commit result is unknown. On an error, return the existing database error from Open. In the callback to `Hub.Admit`, report an uncertain diagnostic as a failed candidate with no possible replacement. Candidate cleanup does not disconnect, cancel or clear the incumbent. Ordinary admission uncertainty still uses the old fallback. No diagnostic runs on a staged-only or pending-only fence, malformed pair, superseded run, bad proof or an unsupported revision.
- Clear only inside an admission or readiness write for the same machine whose persisted generation strictly exceeds the recorded fence, and only when that admission is ready. The clear works for a later reconnect of a ready run. Keep a pending stream admitted before the refusal from clearing merely by reaching its barrier. Use the enclosing write's existing `registry.Touch` to publish the true-to-false change once. Machine removal deletes the Link row and publishes the existing machine removal reference.
- Add `EventHistoryNeedsRecovery bool` to `machines.Presence`. Change the Link-owned presence query to start from `link_machines` and left join `link_connections`. Map absent connection data to empty presence fields and consult the hub only for a nonempty stream identifier. `ProjectWithPresence` copies `EventHistoryNeedsRecovery` whenever it has presence data. The `Held` predicate still gates liveness data. The same Machines watch snapshot and saved-row path therefore project the flag even for offline or waiting machines and without a newly admitted stream. Machines reads through its Link interface.
- Write `TestRecoveryWarningFollowsLaterReadyAdmission` in `recovery_v2_test.go` first. With committed mark 10, reject a signed unseen first 9, observe the authenticated watch update and a restarted snapshot, and let the incumbent heartbeat and apply valid and consumed events. Require true throughout, also when no connection remains. Test a waiting machine and a pending incumbent that reaches readiness after the flag was set. Before item 5, exercise that transition through the shared promotion helper under the real machine lock. Repeat through the stream in item 5. Admit a later pending run and expect the flag through replay until it becomes ready. In another case reconnect a later stream for the same ready run and expect false. Race another behind-mark refusal after a pending admission and require its readiness to leave true until a subsequent admission. Test stage-only and pending-only refusals with a false starting flag, plus removal.
- Write `TestRecoveryDiagnosticCommitReplyLost` in the same file first. Reuse the PostgreSQL commit-reply wrapper from the existing handshake test. With an incumbent live, lose the diagnostic commit reply after the database commits, and in a second case lose the connection before the commit reaches PostgreSQL. The candidate gets the existing `unavailable` database error with no Welcome. Observe the same incumbent identifier and generation and read a subsequent heartbeat acknowledgement. On an independent connection require the flag and its saved feed row either both present or both absent according to where the reply was cut. Show the same saved flag after restart in the committed case.
- The item is risky because mixing the two kinds of uncertain commit can end a healthy stream. Mutate the diagnostic branch to use the admission uncertainty path and to clear the flag on any incumbent readiness or heartbeat. Require both tests to fail. Mutate the projection to read only held connections and the offline snapshot must fail.

**Item 5. Permit-free replay and ready-run transition.** Change `src/server/internal/link/admission.go`, `handshake.go`, `hub.go`, `presence.go`, `inbox.go`, `stream.go`, `outbox_claim.go` and `outbox_send.go`. Add `src/server/internal/link/run_gate_test.go`. Needs item 4 and coordinates with item 6 before their changes to `outbox_claim.go`, `outbox_send.go` and `inbox.go`. Item 8 can prepare fixture options in parallel with this item. This implements foundation rules 4, 6, 7 and 10, C8 and C11, newer Link interface point 3, R8 and R9, and the permit observation from round 3 logic.

- Promote a run when its durable mark is at least `replay_through`. Only the latest admitted run becomes ready, and the machine and exact stream fence are already locked. Set `ready_run_id` and call `RunChanged` once in that write, after `StreamAdmitted` when they share the admission. On the event that completes the prefix, run this transition after report application, advancement and due settlements. A later admission of the same already-ready run repeats only `StreamAdmitted` and checks warning clearance. A callback error rolls back ready-run state, mark, reports, settlements and feed together.
- Pass the committed admission mark and immutable replay boundary to the one writer through its exact stream handle. Let the writer open its local gate from the mark returned by a successful event transaction only after its acknowledgement is sent. Keep the existing `EventResult` shape for direct database callers. Until ready, it processes heartbeats and events, bypasses the permit queue entirely and never calls `SendNext`. Recheck durable readiness with the machine lock in `claimNext` before changing its cursor or attempt and in `sendClaimed` before making a write to the network. A same-stream turn may start claiming immediately after the writer sends the final replay acknowledgement. On admission with an empty prefix, it may claim after Welcome. Preserve the first Welcome fence and command-send deadlines.
- Keep current-stream and number checks first. While waiting for approval, apply valid Facts and consume other validly numbered events without a report effect. Approved processing uses the standing read under the event write's lock. Client Link holds fresh reports until approval. Existing `SetState` rules apply once ready. The writer sends the acknowledgement for the final replay frame before trying commands, even if approval happens while that acknowledgement is held. Retain the existing heartbeat and exact-stream watchdog.
- Write `TestRunBarrierDoesNotUseCommandPermit` in `run_gate_test.go` first. Occupy the three real shared permits on other machines. Start target run B at first 10 with mark 5 and two waiting commands. Replay 6 to 9 on the same stream, verify each acknowledgement and a heartbeat, and inspect the permit queue, attempts and cursor for no early claim. Then free one permit and expect both commands in order. Repeat with an empty prefix and a same-run reconnect. Also call the claim fence directly with a nonzero sentinel cursor while gated and expect it unchanged.
- Write `TestWaitingReplayAcknowledgesBeforeStanding` in the same file first. Send numbered retained Facts and FolderSeen for a waiting machine. Verify visible Facts, no folder callback, and one cumulative acknowledgement per apply or repeat. Pause after the final application but before its acknowledgement and approve. Assert no claim or `SetState` before the acknowledgement, then read that notice on the same stream. In a separate stream send a gap and expect no acknowledgement and the `event_gap` refusal.
- Write `TestReadyRunNoticeIsTransactional` in the same file first. Record each hook's view of the mark and the run row in a test-part table. Fail the ready callback after its insert. Verify rollback and no acknowledgement, then replay and require report, mark and ready callbacks in their planned order with one committed transition. Repeat an immediate admission with no prefix and verify one callback followed by none on same-run reconnect. Assert the admission hook can enqueue without opening the gate.
- The item is risky because early dispatch can run a command ahead of its history. Mutate to acquire a permit before checking readiness, to advance the cursor while gated and to emit the standing notice before the last acknowledgement. Require the respective permit, order and waiting tests to fail. The report and settlement order is additionally checked in items 6 and 7.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A stale process with the same boundary replaces a newer one. | High | Match the last admitted identifier and immutable boundary in PostgreSQL, including after restart. |
| A diagnostic refusal or old stream clears the warning or displaces the incumbent. | High | Persist the generation of the refusal and distinguish diagnostic uncertainty from admission uncertainty. |
| Replay takes a send permit or lets approval bypass the gate. | High | Cover the three occupied permits and pause before the last replay acknowledgement while approving. |

**Verify.** Run the seven named workstream tests in PostgreSQL and the prior Link watchdog, proof and action tests. Check waiting and offline flag states on an authenticated Machines watch in the application fixture. The Stack team owns the deployed installer and proxy checks.

**WS-3. A folder report and its command result settle in order**

A result can arrive before the report it describes. The command must stop being offered as soon as the first valid result is staged. The report and any settlement it unlocks then commit in order.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 6 | The first eligible result is durable, is omitted from resends and sweeps, and settles once at its event number or as removed. | `TestStagedResultSurvivesRestartAndSweep`, `TestResultFenceKeepsAttemptAndCommandRules`, `TestLinkRemovalSettlesAndCallsParts`. |
| 7 | The folder handler runs after identity and report validation. A failed write leaves its changes, the mark and the queued result unchanged. Size and acknowledgement edges follow the contract. | `TestFolderReportPrecedesSettlement`, `TestRevisionTwoEventBudgetsAndAcks`, `TestLookAtFolderCommandBudgets`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 6 | Persist and settle command results in the ledger. | `src/server/internal/link/outbox_result.go`, `State.AcceptResult`. `src/server/internal/link/outbox_sweep.go`, `State.sweepCommandBatch`. `src/server/internal/link/presence.go`, `State.Forget`. `src/server/internal/link/outbox_claim.go`, `readClaimPage` and `revisitScans`. |
| 7 | Route the folder action and event and attach the fixture failure to the actual event write. | `src/server/internal/link/outbox.go`, `commandAction`. `src/server/internal/link/inbox.go`, `applyReport` and `ApplyEvent`. `src/server/internal/link/stream.go`, `Service.dispatch`. `src/server/internal/api/server.go`, `linkReadLimit`. |

**Item 6. First eligible result, eventual settlement and removal.** Change `src/server/internal/link/outbox_result.go`, `outbox_claim.go`, `outbox_send.go`, `outbox_sweep.go`, `presence.go` and `inbox.go`. Add `src/server/internal/link/settlement_v2_test.go`. Needs items 1 and 2. Build the result and sweep work in parallel with item 3. Land the inbox integration after item 5 and the other shared files in item order. This implements D5, foundation rules 3, 4, 6 and 7, newer Link interface points 3 and 4, and R6, R7, R10 to R12.

- Under the current stream and machine lock, keep the old approval, machine identifier, current assignment, matching attempt, dispatched and unfinished constraints. Validate `CommandResult.last_event_number` in the inclusive range from zero to the largest signed 64-bit integer. Preserve failed for unspecified and unknown outcomes, and the existing `SetState` mapping from expired to failed. The field is a scalar. A revision 1 receipt replayed by revision 2 is zero, and a revision 2 receipt can also report zero. Accept the dead process's reported watermark on an interrupted result. Receipt provenance and choosing that watermark belong to Client Link.
- For an eligible unstaged row, settle immediately when its watermark is covered by the mark. Otherwise persist the normalized outcome and its watermark on the command. A further result cannot change either value. Exclude staged rows in claim, deferred scan lookup, send recheck, sweep page reads and sweep batch recheck. Wake or re-evaluate with the existing mechanism so removing a staged scan from eligibility does not hold up following scans. Keep the 23-hour resend grace, 24-hour receipt margin, hourly 24-hour sweep of unanswered ordinary commands, seven-day terminal retention and the existing `SetState` exceptions.
- Use a common terminal write for client results, timeouts and removal. Read its original action payload before clearing it, then update outcome, finished time, payload and staged fields. Call its registered settlement hook inside the write. Collect selected rows before calling any hook so the same database transaction can be used by the callback. On an eligible next event, apply or consume it, advance the mark, then settle the covered staged rows by watermark and command sequence. Promote the run last. Record a single Link outbox saved reference per machine per write when a stage or settlement changes eligibility. A late report for a settled operation may still be an eligible numbered event. Its part decides whether to act.
- In `State.Forget`, settle each unfinished command as `removed`, including staged ones. Keep their terminal records. Call each registered machine removal hook even if the machine had no unfinished command. Then delete its connection and Link row in the existing write. Preserve the registry and feed removal transaction and the existing hub removal notification. A failing callback rolls back registry, all Link changes and part changes. A command result racing removal or sweep gets one terminal transition under the machine lock. The ordinary timeout sweep excludes staged results indefinitely. Recovery consists of applying the real prefix or removing the machine.
- Write `TestStagedResultSurvivesRestartAndSweep` in `settlement_v2_test.go` first. Stage a dispatched result above the mark, then send a conflicting outcome and watermark for the same attempt. Reopen the store and service on the same database, reconnect with the saved run pair and age the command beyond its sweep cutoff. Observe retained payload and original stage, no extra attempt, no time out and no settlement callback. Apply the missing events, including a consumed next event that reaches the watermark. Assert one terminal row, cleared staging, one hook with the mark visible in its transaction and the expected saved outcome. Repeat and require no further settlement. In another case admit a pending run whose first number is the staged watermark plus one. Assert that the settlement hook sees the old ready run and the ready-run hook sees the settlement.
- Write `TestResultFenceKeepsAttemptAndCommandRules` in the same file first. Assert no stage for a wrong machine, obsolete stream, stale attempt or a claimed but undelivered command. Exercise zero, an equal watermark, an interrupted result with a lower old-run watermark and an invalid out-of-range value. Stage a standing notice and verify the prior expiry and sweep exceptions and no resend. Hold the sweeper after selection, stage the candidate, and require the locked recheck to skip it. Check the result-first and sweep-first terminal outcomes and callback counts.
- Write `TestLinkRemovalSettlesAndCallsParts` in the same file first. Remove an approved machine with one staged and one unfinished command, then remove a machine with neither. Also exercise waiting token revocation and a rollback from a failing lifecycle callback. Read the registry, warning, queue and callback rows and the feed through a separate connection. Observe one removal hook for each actual removal, no hook on approved token revocation, and no partial effect after rollback. Deliver an old frame after removal and require no callback or acknowledgement.
- The item is risky because incorrect eligibility or a split transaction could rerun work or lose its report. Mutate to resend or sweep a staged row, replace its first outcome, settle before updating the mark, promote the run before the same event's due settlement, or call machine removal only when a pending command exists. Require the named tests to fail. Also remove the locked sweep predicate and exercise the selected-then-staged race.

**Item 7. Folder routing, limits and exact event rollback point.** Change `src/server/internal/link/outbox.go`, `inbox.go`, `stream.go`, `presence.go` and `run.go`. Add `src/server/internal/link/folders_v2_test.go`. Needs items 1, 5 and 6. Item 8 can prepare application and fixture wiring first. This implements the newer byte budget, Link interface points 1, 3 and 4, foundation rules 2 to 4, 6, 8 to 10, and the Link side of R1 to R7 and R9 to R12.

- Store `Command_LookAtFolder` under `CommandLookAtFolder` after rebuilding only the declared `LookAtFolder.operation_id` and `path` fields. Require a canonical version 7 operation identifier, a nonempty absolute path or home marker by the reviewed grammar, valid Unicode Transformation Format, 8-bit (UTF-8) text, no zero byte, and at most 4,096 path bytes. Projects performs the same contract path check at its own entry. Link owns the encoded command limit before storing any supported action. Size the cleaned command with its actual server-assigned `id` and `expires_at` and with the widest `attempt`. Obtain expiry from the same sampled database clock used for insertion. Reject a complete command over 49,152 bytes without writing the row or wake. Outer link frames are limited to 65,536 bytes, including the Protocol Buffers wrapper and measured after decompression on receive. Reuse the existing transport and handler limits.
- In `ApplyEvent`, check the number under the current stream fence before inspecting the report. Ignore the payload on a repeat and acknowledge the committed mark once. Consume a decoded next event over 49,152 bytes, or an invalid or unsupported next report, without running a part. An eligible `Event_FolderSeen` reaches `ReportFolderSeen` only for the approved machine. Validate both version 7 identifiers, a present `FolderSeen_Facts` or a `FolderSeen_Refusal` with a defined nonzero value, the text constraints and nonempty present optional values. Check the absolute resolved path and 8,192-byte limit, the derived name, the nonempty main branch, the 2,048-byte remote limit and the contradiction between a present remote and `remote_omitted=true`. Keep the contract's origin omission semantics. Projects matches authenticated machine, operation and command and interprets identity. A syntactically valid stale report is consumed even when Projects changes nothing.
- Expose `link.Seams.BeforeEventCommit func(context.Context, string, uint64) error`. Invoke it in the real event transaction after the successful report callback and before the mark or due results are written. Use it on next-number processing only. The string is the authenticated machine identifier and the integer is the event number. Returning an error makes Open return the database failure with no acknowledgement. Production supplies a nil callback. Preserve `BeforeEventAck` as the separate after-commit observation. Recheck exact current identity before sending after a held observation. The write lock determines eligibility for a received frame, and the existing transport guard bounds any acknowledgement already being written.
- Emit one `EventAck.through` per repeat or committed next event after the write completes. Send none for zero, an out-of-range number, gap, failed write, obsolete stream or undecodable frame. On a gap leave the mark intact and end with `failed_precondition` and `Lgtm-Link-Refusal: event_gap`. The 32-event credit is a Client Link send rule. The server keeps a bounded reader and accounts for every accepted event with its own acknowledgement. It does not add a stream counter based on the arrival of transport bytes.
- Write `TestFolderReportPrecedesSettlement` in `folders_v2_test.go` first. Use a registered test part whose report writes a folder sentinel and whose settlement records whether the marker is visible. Stage a result for the report's number. With the new seam returning an error, observe no committed sentinel, mark change, terminal outcome or acknowledgement. Disable the fault and replay. Expect the folder sentinel, mark, and one settlement with the sentinel visible to the hook. Test a result already covered before any report, and then a late report for the closed operation, which advances the mark without changing the result. Test unknown operation, wrong machine, wrong command and obsolete stream with the part's checks. Use both facts and refusal arms.
- Write `TestRevisionTwoEventBudgetsAndAcks` in the same file first. Build valid numbered events of exactly 49,152 encoded bytes and one more byte with the generated messages. Expect one callback at the inner edge and none above it, with one acknowledgement for each committed next number and for repeats. Use complete 65,536- and 65,537-byte encoded request frames, including a compressed case above the limit. Expect the larger outer request refused without mark or acknowledgement and the exact-limit decodable eligible next frame consumed. Try a gap, invalid event numbers, missing or invalid report arms and inconsistent omitted remote. Verify no gap acknowledgement and unchanged mark. With the observer paused, queue 32 numbered events before releasing acknowledgement and expect one cumulative acknowledgement per input. The real client fixture proves it withholds the 33rd event until it has credit.
- Write `TestLookAtFolderCommandBudgets` in the same file first. Record and decode the returned folder command. Verify its exact route, operation identifier, generated command identifier and path at 4,096 bytes. Reject 4,097 bytes and invalid path or identifier without command or wake. Test the complete envelope-sizing helper with synthetic payloads at 49,152 and 49,153 encoded bytes using the widest attempt. The current three legacy actions and folder path limit keep every valid declared action under the aggregate budget.
- The item is risky because a report can mutate a part before identity or validation. Mutate to apply on a duplicate, use the command instead of the authenticated stream as the machine source, skip the part rollback, acknowledge before the write commits, or skip the complete size check. Require the relevant callback, rollback and wire assertions to fail.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A restarted stream reruns a command whose result is already saved. | High | Persist and exclude the first stage in all claim, send and sweep eligibility queries. |
| An acknowledgement hides a partial write or a result incorrectly wins over its own report. | High | Fault between report and mark, inspect durable state and replay the same report before settling. |
| Many settlements at one mark or removal can create a large transaction. | Medium | Keep callbacks database-only, record the Link wake once per machine and transaction, and collect settlement rows in deterministic order. Ask Projects and Store to account for the existing feed reference cap on bulk folder removal. |

**Verify.** Run the six named tests against PostgreSQL. Use both supported stream protocols and the running combined fixtures for the limits and receipt observations. Keep payloads and database errors out of logs.

**WS-4. The assembled server exposes the same behavior**

The production startup path uses the same transaction boundaries as the fixtures. The cross-program proves those boundaries through the real server and client and through a deployed upgrade.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 8 | The server starts from prior data, wires the rollback observation into the actual write, and builds its three server parts together. | `TestServerLinkRevisionTwoWiring`. After Projects lands, `go test ./src/server/... -run '^$'` exits 0 and the combined and deployed gates below pass. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 8 | Wire the application and fixture and verify the package boundary. | `src/server/internal/app/start.go`, `LinkSeams` and `startup.run`. `src/server/servertest/server.go`, `Options` and `Server.launch`. `src/server/servertest/link_stream_test.go`, `TestLinkSeamsReachTheService`. |

**Item 8. Application and integration fixture.** Change `src/server/internal/app/start.go` and `src/server/servertest/server.go`. Add `src/server/servertest/link_v2_test.go`. Wire the rollback hook from `app.LinkSeams` and `servertest.LinkSeams` to `link.Seams` through the existing conversion. Allow `servertest.Options.ConfigurePostgres func(*pgxpool.Config)` to configure the fixture-owned pool before startup, retaining it across fixture restarts, for independent commit-reply tests. Production startup uses the same migrated State for Link and Machines. Projects registers its own routes before any link or sweep worker starts. It mounts its service and migrations in its own assignment. File preparation can overlap items 5 to 7. The item depends on all of them for its executable acceptance. This implements D1, D4 and D5, foundation rules 1 to 5 and 10, C6 to C12 and the Link contribution to newer R5, R6 and R8 to R12 and R14.

- Write `TestServerLinkRevisionTwoWiring` in `src/server/servertest/link_v2_test.go` first. Start the real application fixture with the new seam armed, join and approve via the existing services, and send a next event after revision 2 Welcome. The test seam sees the expected machine and event and refuses the write. The live handshake has no acknowledgement for that input and the restarted Welcome has the earlier mark. Let the next write succeed. Confirm one acknowledgement and a later Welcome with the committed mark. The schema observer checks version 3.
- This item is risky because startup could wire the seam only after commit or use separate Link and Machines state. Detach the seam or move it to the acknowledgement point and require the test's restart mark to fail. Test the real Projects subscriber in the combined folder settlement case.

**Sprint 3 frozen test changes**

The following is the allowlist for the test writer. When a test uses a shared helper, update the helper inputs once and preserve its inherited assertions. Keep negative old revision coverage explicit. The orchestrator updates frozen hashes after reviewing these edits.

| Existing test or fixture | Required revision 2 edit |
|---|---|
| `handshake_test.go` with `TestHandshakeOpensStream`, `TestReplayedProofRefused`, `TestOpenRefusesUnusableRegisteredKey`, `TestHandshakeDecisionOrder`, `TestHandshakeValueRules`, `TestRemovalOrdersAdmission`, `TestStaleAdmissionCannotStartWelcome`, `TestWelcomeStartedBeforeReplacement`, `TestAdmissionCommitReplyLost` and `TestScriptedMachineJoinsAndConnects`. | Use revision 2 for valid peers and Welcome assertions, revision 1 for the mismatch row and its updated sentence. Add a stable valid run and first number to `helloFrame` and `submitProof`. Use the same run on repeated calls and when `TestHandshakeOpensStream` advances the mark to 42. A malformed-proof fixture with a preloaded mark still exercises proof before history validation. Retain proof vectors, cutoff and refusal-order checks. Fixtures sensitive only to a Challenge may keep either revision. |
| `review_admission_test.go` with `TestRemovalAfterReplacementCommitBeforePublication`, `review_c_admission_panic_test.go` with `TestAdmissionCommitPanicAllowsCommandsAfterReconnect`, `review_g_admission_panic_test.go` with `TestReviewGAdmissionCommitPanicUnregistersFailedHandle` and `TestReviewGCommittedAdmissionPanicCleansExactStream`, `review_handshake_test.go` with `TestProofRequestClosureAtCutoff`, and `review_outbox_test.go` with `TestWatchdogAbortsDisplacedStreamBeforeReplacementPublication`. | Use revision 2 on the directly constructed current peer. Preserve each assertion about lost commits, panics, cutoff and terminal stream behavior. `TestReplacementWelcomeCarriesCommittedMark` in `stream_test.go` already uses the constant and needs the stable pair supplied by the shared helper. |
| `outbox_send_test.go` with `openCommandPeer` for `TestCommandSendDeadline` and `TestCommandSendPoolHeadroom`. | Send revision 2 and a valid run pair. Preserve the three-permit, blocked-write and heartbeat assertions. |
| `presence_test.go` with the `presenceFixture.admit` helper and `TestBecomeCurrent`; `inbox_test.go`, `oneline_test.go`, `outbox_result_test.go` and `outbox_send_test.go` with direct replacement calls. | Supply the new `RunAnnouncement` to direct admissions. Reuse a fixture run per machine and pick a valid initial boundary for the seeded mark. The direct callers are `TestEventFormsOneLine`, `TestWritesOfOneMachineFormOneLine`, `TestResultAndReplacementFormOneLine` and `TestCommandSendRechecksBeforeWrite`. Keep their PostgreSQL and stale-stream assertions. Update the live manifest version in `TestBecomeCurrent` to 3. |
| `src/server/internal/machines/commands_test.go` and `stream_projection_test.go` with direct calls to `BecomeCurrent`. | Update the shared presence fixture and the direct replacement cases to pass the same pair. Keep the read and queue assertions in `TestProjectStreamPresence`, `TestWatchClientsStreamPresence` and `TestScanAndReconnectQueueCommands`. |
| `outbox_test.go` with `TestCommandMigrationUpgradesPartB`. | Cap the tested manifest at migration 2 and seed the migration 1 connection without invoking runtime revision 2. Preserve the old-schema behavior while item 2 proves the next migration. |
| `src/server/internal/app/link_stream_test.go` and `review_commands_test.go` with `appChallenge` and `openCommandReviewPeer`. | Send the scripted run pair and revision 2. In `TestServerRestartsAfterInterruptedCommandMigration`, check latest migration 3 instead of latest 2. Keep the startup and shutdown assertions. All callers of the scripted `machinetest.Machine.Connect` get the revision and stable pair from that fixture. |
| `src/server/internal/app/commands_test.go` with `TestServerCommandSweepLifecycle` and its `seedStoppedCommandSweep` helper. | Expect startup to apply manifest version 3. Preserve the pending and timed-out command assertions. |

Transport tests that marshal a synthetic revision 1 Welcome to test raw framing keep those bytes. The sprint 3 proof examples and historical migration files remain frozen. The source survey records the exact affected helper callers.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| The integration assembly applies the migration but never registers or uses the new link boundary. | High | Exercise a real application write, compile Server Link, Projects and Machines together and run the real two-program tests. |

**Verify.** In rollout step 2 run `bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/link ./src/server/internal/machines ./src/server/internal/app ./src/server/servertest -count=1`. Require each named new test to report an explicit pass and zero skips. Then run the same packages with `-race`. During rollout step 3, run `LGTM_LINK_BASE=fcfa96c3 bash scripts/check-go.sh` as the reviewed contract requests before the revision 2 pin moves. Run `bash deploy/test/with-postgres.sh -- go test ./... -count=1` with Projects and both programs. Run `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^TestSprintFour' -count=1` and require explicit passes from `TestSprintFourHistoryRecovery`, `TestSprintFourPendingRun`, `TestSprintFourWaitingReplay`, `TestSprintFourReceiptReplay` and `TestSprintFourFolderSettlement`, with zero skips. In rollout step 4, the Stack team observes the verified revision refusal and in-place upgrade, the recovery outcomes and the deployed R14 repository flow.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The prefix WS names a workstream. Foundation decision, rule and check numbers follow their source order. The source universe consists of assignment points 1 to 7, the foundation rules 1 to 11 and checks C1 to C13, the newer design's four Link points, sprint 4 checks R1 to R14 and open question 2, the Link row and names in the contract fold result, both round 3 closing lists, the two sprint 3 plans and their review registers, issues 1632 to 1635, 1667 and 1676, choice 18, and build loop phase 4. Foundation decisions D1 to D5, D7, D13, D16 and D17 are the decisions that the Link boundary touches.

| Source item | Workstream | Status |
|---|---|---|
| Assignment 1 to 2 and newer Link points 1 to 3. Command identifier, locks, part hooks and revision. | WS-1 to WS-4 | Items 1 and 3 publish the names and items 3 to 8 wire them. |
| Assignment 3 to 5, newer Link point 4, open question 2 and issue 1676. | WS-2 and WS-3 | Items 3 to 7 specify admission, warning, replay, ordering and recovery. Restored server history and indistinguishable client history remain outside the reviewed guarantee. |
| Assignment 6 and 7. Folder routing and migration. | WS-1 and WS-3 | Items 2 and 7 pin migration and packet names. |
| Foundation D1 to D5 and rules 1 to 5 and 7. Contract and database ownership. | WS-1 to WS-4 | Link uses its numbered migration, the store's transaction, the Machines watch and transactional callbacks. |
| Foundation D7, D13, D16 and D17 and rules 6, 8, 9 and 11. Trust, removal, reconnect, stored commands, refusals and credit. | WS-2 to WS-4 | Items 3 and 6 to 8 preserve verified handshake, cleanup, clocks, fixed errors and safe logging. The plan copies no external code. |
| Foundation rule 10 and C12. PostgreSQL and integrated tests. | WS-1 to WS-4 | Server-level tests use PostgreSQL and the combined gate uses both real programs. |
| Foundation C1 to C5 and C13. Stack, sessions, joining token and vault. | WS-4 | Existing full suite and stack gates cover these. The change starts with Open, after enrollment and startup prerequisite checks. |
| Foundation C6 to C11. Waiting, approval, connection loss, restart, proof and ordered event replay. | WS-2 and WS-4 | Existing and new stream, watch and combined tests apply. |
| Newer R1 to R5, R7. Folder facts, operations, retries, expiry and attachment. | WS-1, WS-3 and WS-4 | Link proves dispatch, persistence and routing. Projects, Client Link and combined fixtures own the product observations. |
| Newer R6, R10. Interrupted receipt and early result across restart. | WS-3 and WS-4 | Items 6 and 7 and the combined receipt and folder settlement cases. |
| Newer R8 and R9. Commit order and replay before a new command. | WS-1, WS-2 and WS-4 | Item 1's concurrent writes and item 5's permit and replay tests. |
| Newer R11. Verified revision refusal and old receipted work. | WS-2 to WS-4 | Item 3 verifies refusal, item 6 accepts zero and Stack upgrades the real client. |
| Newer R12. Machine removal. | WS-2 to WS-4 | Link removes its warning and settles staged and ordinary commands. Projects supplies cleanup in the same transaction. |
| Newer R13 and R14. Sample and real modes and deployed repository flow. | WS-4 | Website and Stack observations are dependencies of combined delivery. |
| Contract fold, Link request and cross-program cases. | WS-1 to WS-4 | Every Link obligation has a test above. The handoff names the server fixture and callback signatures. |
| Round 3 logic and fit closing lists. | WS-2 to WS-4 | Item 5 covers the three permits and cursor. Item 7 covers byte edges. Item 8 provides the startup boundary. The decision log records the other owners. |
| Sprint 3 stream and commands plans and their reviews, issue 1667 and choice 18. | WS-2 to WS-4 | Carry exact-stream fencing, durable dispatch, scan and sweep rules. Keep the open terminal-ordering choice outside this change. |
| Issues 1632 to 1635 and build loop phase 4. | WS-4 | Continue the established stack trust, paired fixtures and sizing. The test writer freezes red assertions before each build. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, the three parts meet through Protocol Buffers. | The Server uses reviewed generated contract code. It proposes no protocol edit. |
| `AGENTS.md`, new rewrite code belongs in the Go server. | The build touches `src/server/` only. The orchestrator owns `src/linktest/` and the Stack team owns `deploy/`. |
| `AGENTS.md`, credit copied code. | Reuse existing repository code. Any external source requires the license and `NOTICE` entry before use. |
| `AGENTS.md`, record issues and take finished work to the base through a pull request. | The result reports scope and review points to the orchestrator. The orchestrator owns staging, commits and integration for this assignment. |
| `AGENTS.md`, repository writing rules. | Use relative paths, expand first uses, give dates and sources for decisions, and run the repository privacy check. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

The work areas use the repository's sizing sheet. The Server scope is `server/backend`. Apply the dependencies in this table when scheduling shared files.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | backend | server/backend | medium | A small public boundary needs a concurrency proof and compatibility with the existing queue. | yes | Reviewed contract. |
| 2 | backend | server/backend | medium | One additive migration and two prior-schema fixtures preserve command state. | yes | Reviewed contract. |
| 3 | backend | server/backend | medium | Extend the existing handshake and one locked admission with a finite set of predicates. | yes | 1 and 2. |
| 4 | backend | server/backend | medium | A single generation fence joins diagnostic persistence to the existing watch projection. | yes | 3. |
| 5 | backend | server/backend | medium | Gate one writer and use the existing claim lock and event transaction. | yes | 4. |
| 6 | backend | server/backend | medium | Reuse one terminal transition under the existing lock across result, event, sweep and removal. | yes | 1 and 2 for outbox work, then 4 and 5 for shared files. |
| 7 | backend | server/backend | medium | Route one action and one report with checked sizes and a transaction observation point. | yes | 1, 5 and 6. |
| 8 | backend | server/backend | low | Extend the existing fixture wiring and run the assembled server test. | yes | 3 to 7 for acceptance. |

Load. OpenAI supplies the planner, all test writers, all builders and all reviewers. The first-pass estimate is 29 launches, including the current planner, four plan review seats, eight test writers, eight builders and eight focused build reviewers. The repository supplies no current remaining weekly allowance. Before dispatch, the orchestrator records the vendor's live remaining capacity and reserves complete units with their test, build and review seats. Defer the next unreserved unit to the next window if the measured capacity cannot cover it. Do not switch vendors. Rework and combined test runs use additional capacity.

</details>

<details><summary><b>Attack log (gate 7, Tier M)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Pending orchestrator review | Pending. | The planner hands off at `attack` after the Tier M self-check. The orchestrator owns the review and findings. |

</details>
