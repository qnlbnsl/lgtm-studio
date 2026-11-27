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

Projects uses the server link to ask a real machine about a folder. The server sends one folder inspection at a time so overlapping requests can share one connection.

After a client restart, the server lets it replay earlier events before sending new work. An early result stays saved across a server restart and settles once after its report. If a new process announces history older than the server's saved mark, Machines shows a recovery warning. A later connection with the required history clears it.

The revision 2 cutover uses the existing installer to update the client and keep its local history.

## Key decisions

The database remembers admitted and ready process identifiers until machine removal. An event number can be shared by two process announcements, so reconnect also checks the identifier.

A staged result keeps its command and watermark until real replay covers it or the machine is removed. Projects saves folder effects in the Link write, so a failed write rolls back the report and command settlement together. [Decision log](decisions.md)

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
2. Freeze the new tests and the listed fixture edits. Items 1, 2 and 8 can land with revision 1 still spoken. Test them on PostgreSQL with sprint 3 data and verify a restart after migration.
3. Build items 3 to 7 on isolated work commits. Assemble Link, Machines and Projects in one server. The orchestrator commits the server switch, client switch and paired fixtures together after the integrated candidate gate passes on the integrated PostgreSQL candidate. Then run `bash deploy/test/checks-s4.sh`. Update the deployed client with the sprint 3 installer while preserving its local events and receipts. (architect-r1 05, logic-r1 05, critic-r1 05)
4. Use the upgraded client to replay old events and a saved result. Observe one report application and no rerun of the receipted command. Add a repository through the deployed website and reload it.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

Here, WS means workstream.

**WS-1. Server parts can store a command with their own records**

The machine lock orders command writes. The Link schema holds the admission and result state.

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

**Item 1. Transactional part interface and ordered enqueue.** Change `src/server/internal/link/outbox.go` and `presence.go`. Add `src/server/internal/link/parts.go` and `parts_test.go`. Item 2 can run in parallel. Needs the Store wake operation below. This implements Link interface points 1 to 3, foundation decisions D2, D4 and D5, foundation rules 3, 4, 6 and 7, and newer check R8.

- Publish `func (s *State) EnqueueCommand(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) (string, error)`. The caller locks the machine with `(*machines.Registry).Lock` in that write before locking any other domain row. The caller keeps the returned identifier only if the write commits. The old `State.Enqueue` delegates to that implementation and discards the identifier, preserving `machines.CommandQueue` and the existing call sites. Use the existing sequence allocation and expiry clock. Before insertion, use `(*machines.Registry).Get` in the locked caller write. Translate `machines.ErrNoMachine` to the existing `link.ErrUnknownMachine` with an empty command identifier and no queue or feed effect. Link returns the error and parts can handle it during removal. (logic-r1 04, critic-r1 03, threat-r1 04) Link records `store.ChangeRef{Topic: "machine_outbox", Key: machineID, Change: store.Saved}` with `func (tx *store.Tx) RecordOnce(change store.ChangeRef) error`. Server Store owns this method. Link enqueue and item 6 staging call it. It coalesces exact references within one write, uses ordinary validation for a new reference and propagates a prior sticky error. Ordinary `Record` keeps its current limit and behavior. (architect-r1 07)
- The machine-row-first rule covers approval and rename that enqueue `SetState`, both Machines actions, a fresh Projects request that queues `LookAtFolder`, and same-machine hook writes that enqueue. Store's caller-and-key advisory lock is taken before the fresh callback. Its existing receipt-row lock runs only on a replay, which enqueues nothing. The feed lock occurs at the end. A gated stream cannot skip an uncommitted earlier command because both append and claim use the machine row. A command for another machine can receive an interleaved sequence number.
- Publish `func (s *State) RegisterPart(hooks PartHooks) error`. Call it during startup before link or sweep goroutines are started. Install named routes as one registration, copy caller-supplied maps, reject duplicate command or report routes with a startup error, and permit several parts to receive lifecycle calls in registration order. The callbacks receive the same `*store.Tx` and context as the write. They may write their own tables and record feed references. They return an error to roll back the whole write. A callback runs again if the previous attempt rolled back. Establish eligibility checks in the part and use nil for a valid no-op. Callbacks for a live machine may enqueue for it, including from admission, run change and ordinary settlement. The removal-induced settlements and removal callback run after the registry deletion and get the missing-machine refusal if they try to enqueue. They must not open a nested write, take another machine lock, wait for a client, or do external work.

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

- The string on `ReportApplied` and `MachineRemoved` is the authenticated machine identifier. `Settlement.Command` contains the stored declared action, command identifier, attempt and expiry. `LastEventNumber` is present for a client result, including zero, and absent for timeout or removal. `Admission.Ready` describes whether the replay boundary is already covered in the admission write. `PreviousRunID` is empty for a machine's first ready run. The registry keys allow another part to add command and report kinds in later revisions. Link defines and dispatches these types. Projects supplies the three service methods and owns the app-local adapter `registerProjectHooks(state *link.State, service *projects.Service) error` in `src/server/internal/app/start.go`. It calls `state.RegisterPart` with `ReportFolderSeen` forwarding the machine identifier and `event.GetFolderSeen()` to `ApplyFolderSeen`, `CommandLookAtFolder` forwarding `settlement.MachineID`, `settlement.Command.GetId()` and `string(settlement.Outcome)` to `SettleCommand`, and `MachineRemoved` bound to `service.RemoveMachine`. All three receive the caller context and transaction. Projects propagates registration failure before serving or starting Link workers. Item 1 freezes the Go types. Real folder queue and report tests need item 7, and real startup registration follows item 8. (architect-r1 01 and 02, logic-r1 01, critic-r1 02, threat-r1 03)
- Write `TestPartEnqueueIsAtomicAndOrdered` in `parts_test.go` first. Pause caller A after storing a restart command and a sentinel with its identifier. On the same machine, B waits for its first domain row lock. Release A and use the existing send fixture to match commit order, both sent identifiers and both caller records. Have the test part’s caller write enqueue more than 256 commands for the same locked machine in one write. Require all rows, ordered delivery and one wake. Force another such write to roll back and require no rows, feed reference or delivery. (architect-r1 07) Also check that the existing error-only interface is satisfied at compile time.
- Write `TestLinkPartRegistration` in the same file first. Register two parts with different routes and lifecycle listeners. Reject a duplicate route without partial registration. Mutate a caller map and require dispatch to keep the copied route. Call the registered test hook directly through item 1’s dispatcher in a real write. Verify its context and transaction with a sentinel.
- The item is risky because a command can pass another one if any caller allocates its sequence before taking the machine lock. Mutate the enqueue path in the test caller to lock late and prove B can pass A or that the lock-order assertion fails. Mutate the enqueue path to use a separate write and prove the rollback assertion fails. The legacy adapter's existing callers and their lock anchors are listed in the scouting record.

**Item 2. Upgrade the link schema.** Add `src/server/internal/link/migrations/0003_link_revision_two.sql` and `src/server/internal/link/migration_v2_test.go`. Change schema assertions or historical seeding in `src/server/internal/link/outbox_test.go` and `src/server/internal/link/presence_test.go`, and in `src/server/internal/app/commands_test.go` and `src/server/internal/app/review_commands_test.go`. (critic-r1 07) No earlier migration bytes change. Item 1 is independent. This implements D4 and D5, foundation rules 1, 4 and 7, and newer R9 to R11.

- Add nullable `link_machines.admitted_run_id` and `ready_run_id` as text. Add nullable nonnegative bigint `recovery_set_generation`. A present generation represents a true recovery flag. The initial state of each field is null. Add `link_runs(machine_id text, run_id text, replay_through bigint)` keyed by `(machine_id, run_id)` and constrained to nonnegative boundaries. Reference `link_machines(machine_id)` with removal cascading into the history. `replay_through` is the first number minus one, so the allowed first event number of 2 to the power of 63 fits without a signed overflow. Preserve one row per admitted run. Superseded identifiers are retained to reject replay at equal boundaries.
- Extend the action constraint to `look_at_folder` and add nullable `link_commands.staged_outcome` and `staged_event_number`. Require both staged fields together, a positive event number, a client outcome, successful dispatch with a stream assignment, and an unfinished command with its payload. Keep standing notice expiry mapping and terminal invariants. Rebuild `link_commands_pending` and `link_commands_expirable` with the stage predicate. Index `(machine_id, staged_event_number, seq)` for pending results. A final transition clears both staged fields.
- Write `TestLinkRevisionTwoMigration` in `migration_v2_test.go` first. Build with Link migrations 1 and 2 only. Seed approved and waiting machines, a connection, mark 42, dispatched, unsent and terminal commands, and a keyed receipt. Apply the current manifest twice and reopen. Require version 3, matching old values and payload bytes, null new metadata, the same receipt, and refusal of the old manifest. Assert that the new action and a valid stage can be inserted. Reject a half-stage, a stage on an undispatched or terminal row, a negative boundary and a duplicate run identifier for that machine. Test removal of the link machine cascades the run rows while old terminal commands stay.
- Keep the old 1-to-2 assertion in `TestCommandMigrationUpgradesPartB` by passing an explicit manifest ending at 2. Update the live-schema version assertions in `TestBecomeCurrent`, `TestServerCommandSweepLifecycle` and `TestServerRestartsAfterInterruptedCommandMigration` to expect version 3. In `TestCommandMigrationUpgradesPartB`, seed the version 1 connection and mark with version 1 statements. The new runtime admission requires tables introduced later.
- The item is risky because a bad default could hide existing commands or abandon an event mark. Mutate to reset the mark and to allow half a staged result. Require the migration test's old-record and constraint assertions to fail.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A command identifier escapes from an aborted write, or a faster request skips a slower request's command. | High | Share the transaction and lock the machine before any other domain row. Hold real concurrent writes in the test. |
| The migration resets old history or changes the eligibility of an old command. | High | Migrate stored version 2 data and compare all prior columns, including attempt, mark and payload. |

**Verify.** Run the three named tests with PostgreSQL. Run the existing Machines command tests as a compile and compatibility check. Device quality assurance is covered by rollout step 4.

**WS-2. A new client process drains the history that came before it**

A reconnect repeats its run pair. A new run drains its retained history first. A behind-mark refusal reports the problem while the incumbent remains current.

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

**Item 3. Revision 2 authentication and run admission.** Change `src/server/internal/link/handshake.go`, `presence.go` and `machinetest/machine.go`. Add `src/server/internal/link/admission.go` and `admission_v2_test.go`. Update only the old admission fixtures listed below. Needs items 1, 2 and 8 for integration. Item 6 can prepare its result and sweep files in parallel. This implements D7, foundation rules 4, 7 and 8 and C6 to C11, newer Link interface point 3, R9 and R11, and the contract's proof and run rules.

- Set `LinkRevision = 2`. Set the Welcome revision from that constant. Use the sentence `The server speaks revision 2 of the link.` for `errLinkRevision`. Preserve the bytes in the `lgtm-link-v1 server` and `lgtm-link-v1 machine` labels. A well-formed initial identity and nonce gets a signed Challenge. Preserve the order of registry unavailable, unknown machine, revision mismatch, and then proof errors. The revision 1 client must verify the Challenge before treating `failed_precondition` with `Lgtm-Link-Refusal: link_revision_mismatch` as final. Check revision 2 history only after a valid machine Proof. A history refusal is the existing retryable `invalid_argument`.
- Validate the run as a canonical version 7 universally unique identifier (UUID). Validate the first number in the inclusive range from 1 to 2 to the power of 63 before subtracting one. Link defines `RunAnnouncement{ID string, FirstEventNumber uint64}` and `func (s *State) BecomeCurrent(ctx context.Context, machineID, streamID, address string, announcement RunAnnouncement) (Stream, error)`. The handshake and the direct Link and Machines fixtures pass the pair. Under the Machines row lock read the mark, latest admitted pair, latest ready run and largest watermark still staged. Preserve the current stream and its command ledger if admission is refused.
- Classify a known run first. Only the latest admitted run with the original boundary is eligible. This includes a pending run after server restart and a ready run whose mark has since advanced past its original boundary. Refuse any known superseded run or changed pair before evaluating the numeric fences. For an unseen run, refuse if `first_event_number <= event_mark` and mark that reason for item 4. Otherwise require it to start strictly above each staged watermark and at or above the pending run's first number. An unseen run at an equal pending boundary may replace it. A refused unseen run is not added to `link_runs`.
- Use the admission write to save the run identifier and boundary, advance the existing stream generation, and install its connection row. Invoke `StreamAdmitted` on each successful admission, including a pending stream and a same-run reconnect. Define the ready transition helper here. If the mark is at least the replay boundary, store the ready identifier after the admission callback and invoke the ready-run callback. Item 5 reuses that helper when an event finishes the replay. Return Welcome only after confirmed admission and the existing hub publication check. Preserve the prior genuine-admission uncertain-commit fallback and the existing exact-stream cleanup.
- Update `machinetest.Machine` with `RunID string` and `FirstEventNumber uint64`. `Join` initializes a fresh run and first number 1. Repeated `Connect` on the same scripted process sends that pair. Tests of a new run set an explicit fresh pair. Keep independent proof construction. Use constants local to the test fixture so it does not import the production Link package.
- Write `TestRevisionTwoProofAndRunAdmission` in `admission_v2_test.go` first. Use both existing transport modes. A revision 1 client verifies the Challenge and sees the exact mismatch trailer without Welcome. A revision 2 bad signature with a behind-mark announcement gets `unauthenticated` and no row or feed change. A valid peer gets revision 2 Welcome. Test the accepted upper bound with mark one below it, plus an invalid identifier version, zero and one above the bound. An announcement one below the boundary covered by the maximum mark is refused without signed overflow.
- Write `TestRunAdmissionPersistsAcrossRestart` in the same file first. Admit A ready at first 6 with mark 5. Admit B at first 10 and restart mid-drain. Refuse A and a changed B pair, accept B at 10, reject an unseen C at 9 solely because of the pending fence, then accept C at 10. Verify B remains refused after restart. Independently admit two ready runs at first 6 with mark still 5 and verify the older run is refused, including after restart. Test an unseen first number equal to a staged watermark with a smaller mark, and its successor. Use item 2's schema to seed the stage until item 6 runs.
- The item is risky because proof and run fences protect the incumbent. Mutate to check history before verifying Proof, to drop the stored pair on disconnect, and to compare only numeric boundaries. Require the corresponding signed, restarted and equal-boundary tests to fail.

**Item 4. Recovery warning and incumbent preservation.** Change `src/server/internal/link/admission.go`, `handshake.go` and `presence.go`, and `src/server/internal/machines/link.go` and `project.go`. Add `src/server/internal/link/recovery_v2_test.go`. Needs item 3. Item 6 may continue in its outbox files. This implements the Contract team's recovery request, foundation D5 and rules 3 to 5 and 7, and supports R9 and R10.

- In the behind-committed-mark branch of the admission write, save the current `link_machines.stream_generation` in `recovery_set_generation`. Call `registry.Touch` when the flag changes from false to true. If it was already true, refresh the generation fence on each qualifying refusal. Return nil from the write to commit the diagnostic and return `ErrInvalidMessage` to the candidate only after the confirmed commit. Carry the diagnostic branch classification out of the write even if the commit result is unknown. On an error, return the existing database error from Open. In the callback to `Hub.Admit`, report an uncertain diagnostic as a failed candidate with no possible replacement. Candidate cleanup does not disconnect, cancel or clear the incumbent. Ordinary admission uncertainty still uses the old fallback. No diagnostic runs on a staged-only or pending-only fence, malformed pair, superseded run, bad proof or an unsupported revision.
- Clear only inside an admission or readiness write for the same machine whose persisted generation strictly exceeds the recorded fence, and only when that admission is ready. The clear works for a later reconnect of a ready run. Keep a pending stream admitted before the refusal from clearing merely by reaching its barrier. Use the enclosing write's existing `registry.Touch` to publish the true-to-false change once. Machine removal deletes the Link row and publishes the existing machine removal reference.
- Add `EventHistoryNeedsRecovery bool` to `machines.Presence`. Change the Link-owned presence query to start from `link_machines` and left join `link_connections`. Map absent connection data to empty presence fields and consult the hub only for a nonempty stream identifier. `ProjectWithPresence` copies `EventHistoryNeedsRecovery` whenever it has presence data. The `Held` predicate still gates liveness data. The same Machines watch snapshot and saved-row path therefore project the flag even for offline or waiting machines and without a newly admitted stream. Machines reads through its Link interface.
- Write `TestRecoveryWarningFollowsLaterReadyAdmission` in `recovery_v2_test.go` first. With committed mark 10, reject a signed unseen first 9, observe the authenticated watch update and a restarted snapshot, and let the incumbent heartbeat and apply valid and consumed events. Require true throughout, also when no connection remains. Test a waiting machine and a pending incumbent that reaches readiness after the flag was set. Before item 5, exercise that transition through the shared promotion helper under the real machine lock. Repeat on the item 5 stream. Admit a later pending run and expect the flag through replay until it becomes ready. In another case reconnect a later stream for the same ready run and expect false. Race another behind-mark refusal after a pending admission and require its readiness to leave true until a subsequent admission. Test stage-only and pending-only refusals with a false starting flag, plus removal.
- Write `TestRecoveryDiagnosticCommitReplyLost` in the same file first. Reuse the PostgreSQL commit-reply wrapper from the existing handshake test. With an incumbent live, lose the diagnostic commit reply after the database commits, and in a second case lose the connection before the commit reaches PostgreSQL. The candidate gets the existing `unavailable` database error with no Welcome. Observe the same incumbent identifier and generation and read a subsequent heartbeat acknowledgement. On an independent connection require the flag and its saved feed row either both present or both absent according to where the reply was cut. Show the same saved flag after restart in the committed case.
- The item is risky because mixing the two kinds of uncertain commit can end a healthy stream. Mutate the diagnostic branch to use the admission uncertainty path and to clear the flag on any incumbent readiness or heartbeat. Require both tests to fail. Mutate the projection to read only held connections and the offline snapshot must fail.

**Item 5. Permit-free replay and ready-run transition.** Change `src/server/internal/link/admission.go`, `handshake.go`, `hub.go`, `presence.go`, `inbox.go`, `stream.go`, `outbox_claim.go` and `outbox_send.go`. Add `src/server/internal/link/run_gate_test.go`. Needs item 4 and coordinates with item 6 before their changes to `outbox_claim.go`, `outbox_send.go` and `inbox.go`. This implements foundation rules 4, 6, 7 and 10, C8 and C11, Link point 3, R8 and R9, and the round 3 permit check.

- Promote a run when its durable mark is at least `replay_through`. Only the latest admitted run becomes ready, and the machine and exact stream fence are already locked. Set `ready_run_id` and call `RunChanged` once in that write, after `StreamAdmitted` when they share the admission. On the event that completes the prefix, run this transition after the existing report effect and mark advancement. Item 6 inserts due settlement between the mark and the run change. (architect-r1 01, critic-r1 04) A later admission of the same already-ready run repeats only `StreamAdmitted` and checks warning clearance. A callback error rolls back ready-run state, mark, reports, settlements and feed together.
- Pass the committed admission mark and immutable replay boundary to the one writer through its exact stream handle. Let the writer open its local gate from the mark returned by a successful event transaction only after its acknowledgement is sent. Keep the existing `EventResult` shape for direct database callers. Until ready, it processes heartbeats and events, bypasses the permit queue entirely and never calls `SendNext`. Recheck durable readiness with the machine lock in `claimNext` before changing its cursor or attempt and in `sendClaimed` before making a write to the network. A same-stream turn may start claiming immediately after the writer sends the final replay acknowledgement. On admission with an empty prefix, it may claim after Welcome. Recheck stream identity after a held acknowledgement observation before sending. Preserve the first Welcome fence and command deadlines.
- Keep current-stream and number checks first. While waiting for approval, apply valid Facts and consume other validly numbered events without a report effect. Approved processing uses the standing read under the event write's lock. Client Link holds fresh reports until approval. Existing `SetState` rules apply once ready. The writer sends the acknowledgement for the final replay frame before trying commands, even if approval happens while that acknowledgement is held. Retain the existing heartbeat and exact-stream watchdog.
- Write `TestRunBarrierDoesNotUseCommandPermit` in `run_gate_test.go` first. Occupy the three real shared permits on other machines. Start target run B at first 10 with mark 5 and two waiting commands. Replay 6 to 9 on the same stream, verify each acknowledgement and a heartbeat, and inspect the permit queue, attempts and cursor for no early claim. Then free one permit and expect both commands in order. Repeat with an empty prefix and a same-run reconnect. Call the claim fence directly with a nonzero sentinel cursor while gated and expect it unchanged. Independently seed a valid current assignment, call `sendClaimed` on the gated stream and expect no write or dispatch marker. (logic-r1 06)
- Write `TestWaitingReplayAcknowledgesBeforeStanding` in the same file first. Send numbered retained Facts and HarnessesFound for a waiting machine. Verify visible Facts, unchanged inventory, and one cumulative acknowledgement per apply or repeat. Item 7 tests the same approval boundary with a registered folder route. (logic-r1 06, critic-r1 04) Pause after the final application but before its acknowledgement and approve. Assert no claim or `SetState` before the acknowledgement, then read that notice on the same stream. In a separate stream send a gap and expect no acknowledgement and the `event_gap` refusal.
- Write `TestReadyRunNoticeIsTransactional` in the same file first. Use retained Facts at the last replay number. The test part’s ready hook reads the Facts value, mark and run row in the same write. Fail the hook after a sentinel insert and check from a separate connection that the Facts value, mark, ready run, sentinel and feed stayed unchanged. Expect no acknowledgement. Replay and require the new Facts value and mark to be visible to the ready hook and exactly one ready transition to commit. (architect-r1 01, critic-r1 04) Repeat an immediate admission with no prefix and verify one callback followed by none on same-run reconnect. Assert the admission hook can enqueue without opening the gate.
- The item is risky because early dispatch can run a command ahead of its history. Mutate to acquire a permit before checking readiness, advance the cursor while gated, bypass the send recheck, or emit the standing notice before the last acknowledgement. Require the respective permit, order and waiting tests to fail. The report and settlement order is additionally checked in items 6 and 7.

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

**Item 6. First eligible result, eventual settlement and removal.** Change `src/server/internal/link/outbox_result.go`, `outbox_claim.go`, `outbox_send.go`, `outbox_sweep.go`, `presence.go` and `inbox.go`. Add `src/server/internal/link/settlement_v2_test.go`. Needs items 1 and 2. Prepare and review the result and sweep files during items 3 and 4. Take the inbox, claim, send and presence files after item 5 and run the joined gate. This implements D5, foundation rules 3, 4, 6 and 7, newer Link interface points 3 and 4, and R6, R7, R10 to R12.

- Under the current stream and machine lock, keep the old approval, machine identifier, current assignment, matching attempt, dispatched and unfinished constraints. Validate `CommandResult.last_event_number` in the inclusive range from zero to the largest signed 64-bit integer. Preserve failed for unspecified and unknown outcomes, and the existing `SetState` mapping from expired to failed. The field is a scalar. A revision 1 receipt replayed by revision 2 is zero, and a revision 2 receipt can also report zero. Accept the dead process's reported watermark on an interrupted result. Receipt provenance and choosing that watermark belong to Client Link.
- For an eligible unstaged row, settle immediately when its watermark is covered by the mark. Otherwise persist the normalized outcome and its watermark on the command. A further result cannot change either value. Exclude staged rows in claim, deferred scan lookup, send recheck, sweep page reads and sweep batch recheck. A newly staged result records one coalesced outbox wake using the item 1 reference. The writer rechecks its deferred entries after handling the result frame. An immediate result uses that next writer turn without a Link feed write. The ordinary sweep and removal emit no Link outbox reference of their own. A heartbeat after sweep gives the writer another turn. Preserve part references and the Machines removal reference. (architect-r1 04 and 07, threat-r1 01) Keep the 23-hour resend grace, 24-hour receipt margin, hourly 24-hour sweep of unanswered ordinary commands, seven-day terminal retention and the existing `SetState` exceptions.
- Use a common terminal write for client results, timeouts and removal. Read its original action payload before clearing it, then update outcome, finished time, payload and staged fields. Call its registered settlement hook inside the write. Collect selected rows before calling any hook so the same database transaction can be used by the callback. For a next event, apply or consume the report, advance the mark, settle by watermark and command sequence, then promote the run. Dispatch settlement by the stored action kind. The item 6 test part registers `CommandKind("restart")`, which already exists in the ledger. (architect-r1 01, critic-r1 04) A late report for a settled operation may still be an eligible numbered event. Its part decides whether to act.
- In `State.Forget`, settle each unfinished command as `removed`, including staged ones. Keep their terminal records. Call each registered machine removal hook even if the machine had no unfinished command. Then delete its connection and Link row in the existing write. Preserve the registry and feed removal transaction and the existing hub removal notification. A failing callback rolls back registry, all Link changes and part changes. A command result racing removal or sweep gets one terminal transition under the machine lock. The ordinary timeout sweep excludes staged results indefinitely. Recovery consists of applying the real prefix or removing the machine. **UNREVIEWED MECHANISM.** Projects owns the bounded `repositories` feed refresh. This uses its existing saved-row loader and requires no new Store or shared watch behavior. Projects removal writes one `store.Saved` reference with empty scope and key `refresh` above `store.MaxWriteChanges - 2` affected repositories. Its watch loader reads that reserved key and returns `store.ErrWatchLagged`. The existing watch returns `aborted` and Website reopens to a positioned snapshot. The Store commit ordering and ordinary cap stay in force. The conservative threshold leaves room for the Machines reference and an additional part reference. (architect-r1 03, logic-r1 03, critic-r1 01, threat-r1 02)
- Write `TestStagedResultSurvivesRestartAndSweep` in `settlement_v2_test.go` first. Stage a dispatched result above the mark, then send a conflicting outcome and watermark for the same attempt. Reopen the store and service on the same database, reconnect with the saved run pair and age the command beyond its sweep cutoff. Observe retained payload and original stage, no extra attempt, no time out and no settlement callback. Apply the missing events, including a consumed next event that reaches the watermark. Assert one terminal row, cleared staging, one hook with the mark visible in its transaction and the expected saved outcome. Repeat and require no further settlement. In another case admit a pending run whose first number is the staged watermark plus one. Assert that the settlement hook sees the old ready run and the ready-run hook sees the settlement.
- Write `TestResultFenceKeepsAttemptAndCommandRules` in the same file first. Assert no stage for a wrong machine, obsolete stream, stale attempt or a claimed but undelivered command. Exercise zero, an equal watermark, an interrupted result with a lower old-run watermark and an invalid out-of-range value. Stage a standing notice and verify its expiry and sweep exceptions and no resend. Stage scan A above the mark and observe its deferred successor B delivered while A still waits for an event. Require one stage wake. With an immediate result, observe B on the next writer turn with the feed unchanged. Preserve the frozen sweep and removal feed assertions. (architect-r1 04, threat-r1 01) Hold the sweeper after selection, stage the candidate, and require the locked recheck to skip it. Check both result and sweep orders. Race a dispatched result at watermark 9 on current stream A, mark 5, against unseen B announcing first 9. If A commits its stage first, B gets `invalid_argument` without Welcome, A stays current, and the stage and attempt stay intact. If B commits first, its Welcome carries mark 5, A’s frame is fenced, and the attempt stays unstaged until eligible redispatch. Neither order changes the recovery flag. Show the first fence read and admission share the machine lock. (critic-r1 06)
- Write `TestLinkRemovalSettlesAndCallsParts` in the same file first. Remove an approved machine with one staged and one unfinished command, then remove a machine with neither. Have both removal-origin callbacks attempt enqueue and handle `ErrUnknownMachine`. Require successful removal with no new command or outbox reference. Have a live ordinary settlement enqueue two follow-ups and retain their identifiers in the same write. Require two rows and one wake. Also exercise waiting token revocation and rollback from a failing lifecycle callback. (logic-r1 04, critic-r1 03, threat-r1 04) Read the registry, warning, queue and callback rows and the feed through a separate connection. Observe one removal hook for each actual removal, no hook on approved token revocation, and no partial effect after rollback. Deliver an old frame after removal and require no callback or acknowledgement.
- The item is risky because incorrect eligibility or a split transaction could rerun work or lose its report. Mutate to resend or sweep a staged row, replace its first outcome, settle before updating the mark, promote the run before the same event's due settlement, call machine removal only when a pending command exists, or omit the enqueue presence check. Require the named tests to fail. Also remove the locked sweep predicate and exercise the selected-then-staged race.

**Item 7. Folder routing, queue and limits.** Change `src/server/internal/link/outbox.go`, `inbox.go` and `outbox_claim.go`, and `src/server/internal/api/server.go`. Add `src/server/internal/link/folders_v2_test.go` and `src/server/internal/api/link_decode.go`. Needs items 5, 6 and 8. Projects tests that enqueue a real folder command or send its report need this item. This implements the newer byte budget, Link interface points 1, 3 and 4, foundation rules 2 to 4, 6, 8 to 10, and the Link side of R1 to R7 and R9 to R12.

- Store `Command_LookAtFolder` under `CommandLookAtFolder` after rebuilding only the declared `LookAtFolder.operation_id` and `path` fields. Require a canonical version 7 operation identifier, a nonempty absolute path or home marker by the reviewed grammar, valid Unicode Transformation Format, 8-bit (UTF-8) text, no zero byte, and at most 4,096 path bytes. Projects performs the same contract path check at its own entry. Link owns the encoded command limit before storing any supported action. Size the cleaned command with its actual server-assigned `id` and `expires_at` and with the widest `attempt`. Obtain expiry from the same sampled database clock used for insertion. Reject a complete command over 49,152 bytes without writing the row or wake. Outer link frames are limited to 65,536 bytes, including the Protocol Buffers wrapper and measured after decompression on receive. Reuse the existing transport and handler limits.
- Keep a separate folder head and deferred sequence using the existing scan cursor pattern under the machine lock. A folder waits behind the first still-offerable, unstaged, unfinished folder for that machine. On reconnect, offer that same head first with the new attempt when eligible. Staging, terminal settlement or the existing 23-hour offer cutoff releases its successor. Item 6 supplies the stage wake and the normal writer turn rechecks the cursor after settlement. Scan and other command eligibility stay independent. (logic-r1 02)
- In `ApplyEvent`, check the number under the current stream fence before inspecting the report. Ignore the payload on a repeat and acknowledge the committed mark once. Consume a decoded next event over 49,152 bytes, or an invalid or unsupported next report, without running a part. An eligible `Event_FolderSeen` reaches `ReportFolderSeen` only for the approved machine. Validate both version 7 identifiers, a present `FolderSeen_Facts` or a `FolderSeen_Refusal` with a defined nonzero value, the text constraints and nonempty present optional values. Check the absolute resolved path and 8,192-byte limit, the derived name, the nonempty main branch, the 2,048-byte remote limit and the contradiction between a present remote and `remote_omitted=true`. Keep the contract's origin omission semantics. Projects matches authenticated machine, operation and command and interprets identity. A syntactically valid stale report is consumed even when Projects changes nothing.
- For inbound binary Open, use the existing handler codec pattern to measure actual decompressed event payload bytes, excluding the outer field tag and length. Decode normally first. For a decoded Event, count the payload bytes of its field occurrences, resetting the count when a different valid Open arm replaces the Event. If over budget, clear the decoded report and leave the event number. Normal Link processing then checks identity and numbering before consuming the next event or acknowledging a repeat. Keep the decoded Protocol Buffers size check for direct calls and JavaScript Object Notation (JSON) Open. The codec is local to binary Open and adds no wire field or cross-package signature. Item 8 owns the pre-commit seam. (threat-r1 05)
- Emit one `EventAck.through` per repeat or committed next event after the write completes. Send none for zero, an out-of-range number, gap, failed write, obsolete stream or undecodable frame. On a gap leave the mark intact and end with `failed_precondition` and `Lgtm-Link-Refusal: event_gap`. The 32-event credit is a Client Link send rule. The server keeps a bounded reader and accounts for every accepted event with its own acknowledgement. It does not add a stream counter based on the arrival of transport bytes.
- Write `TestFolderReportPrecedesSettlement` in `folders_v2_test.go` first. Use a registered test part whose report writes a folder sentinel and whose settlement records whether the marker is visible. Stage the result on the original stream. Admit a new run at the watermark plus one and register the ready hook. Fault the final replay report before mark update. Expect no committed report, mark advance, settlement, ready transition or acknowledgement. Disable the fault and replay. Require report, mark advance, settlement and ready hook in order. Settlement sees the report and mark, and the ready hook sees the settlement. Send a valid next folder report while waiting and expect an acknowledgement with no callback. Approve that machine and send an equivalent report at a fresh next number and expect one callback. (architect-r1 01, critic-r1 04, logic-r1 06) Test a result already covered before any report, and then a late report for the closed operation, which advances the mark without changing the result. Test unknown operation, wrong machine, wrong command and obsolete stream with the part's checks. Use both facts and refusal arms.
- Write `TestRevisionTwoEventBudgetsAndAcks` in the same file first. Build valid numbered events of exactly 49,152 encoded bytes and one more byte with the generated messages. Expect one callback at the inner edge and none above it, with one acknowledgement for each committed next number and for repeats. Send a valid binary Event with redundant field encoding whose received size exceeds the inner budget and whose decoded size fits. Expect consumption with one acknowledgement and no callback on both stream protocols. (threat-r1 05) Use an over-budget inner event and pad the outer request with an unknown wrapper field to 65,536 and 65,537 bytes. Include compression in the above-limit case. At the edge, the next number is consumed without a callback. Above it, expect no mark advance or acknowledgement. Try a gap, invalid event numbers, missing or invalid report arms and inconsistent omitted remote. Verify no gap acknowledgement and unchanged mark. With the observer paused, queue 32 numbered events before releasing acknowledgement and expect one cumulative acknowledgement per input. The real client fixture proves it withholds the 33rd event until it has credit.
- Write `TestLookAtFolderCommandBudgets` in the same file first. Record and decode the returned folder command. Verify its exact route, operation identifier, generated command identifier and path at 4,096 bytes. Reject 4,097 bytes and invalid path or identifier without command or wake. Test the complete envelope-sizing helper with synthetic payloads at 49,152 and 49,153 encoded bytes using the widest attempt. Also enqueue two folder commands alongside a scan and hold the first folder attempt. Require continued heartbeat, no second folder assignment and independent scan delivery. Stage the first result above the mark and require the next folder command in order on the same stream. Repeat with an immediately settled result, a reconnect before settlement and the offer cutoff. (logic-r1 02)
- The item is risky because a report can mutate a part before identity or validation. Mutate to apply on a duplicate, use the command instead of the authenticated stream as the machine source, skip the part rollback or approval check, acknowledge before commit, allow a busy folder successor, or size inbound binary Events only after re-encoding. Require the relevant callback, rollback and wire assertions to fail.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A restarted stream reruns a command whose result is already saved. | High | Persist and exclude the first stage in all claim, send and sweep eligibility queries. |
| An acknowledgement hides a partial write or a result incorrectly wins over its own report. | High | Fault between report and mark, inspect durable state and replay the same report before settling. |
| Many settlements at one mark or removal can create a large transaction. | Medium | Keep callbacks database-only and collect settlements in deterministic order. Coalesce Link enqueue and staging wakes. Projects uses a bounded watch refresh for large removals. |

**Verify.** Run the six named tests against PostgreSQL. Use both stream protocols for limits and paired fixtures for receipts. Keep payloads and database errors out of logs.

**WS-4. The assembled server exposes the same behavior**

The production startup path uses the same transaction boundaries as the fixtures. The paired-program tests exercise those boundaries in the real server and client. The Stack team verifies the deployed upgrade.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 8 | The real fixture injects a pre-commit failure, retains its pool configuration on restart, and observes the committed mark. | `TestServerLinkRevisionTwoWiring` before and after the wire switch. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 8 | Wire the application and fixture and verify the package boundary. | `src/server/internal/app/start.go`, `LinkSeams` and `startup.run`. `src/server/servertest/server.go`, `Options` and `Server.launch`. `src/server/servertest/link_stream_test.go`, `TestLinkSeamsReachTheService`. |

**Item 8. Application and paired-program fixture entry points.** Change `src/server/internal/app/start.go`, `src/server/internal/link/run.go`, `inbox.go` and `stream.go`, and `src/server/servertest/server.go`. Add `src/server/servertest/link_v2_test.go`. Needs item 2. Land while revision 1 is active and before item 3 integrates or Projects edits startup. This implements D1, D4 and D5, foundation rules 1 to 5 and 10, C6 to C12 and the Link contribution to newer R5, R6 and R8 to R12 and R14. (architect-r1 06, logic-r1 01, threat-r1 03)

| Exact fixture entry point | Writer, reader and timing |
|---|---|
| `func servertest.Start(t testing.TB, options servertest.Options) *servertest.Server` and `func (s *servertest.Server) Restart(t testing.TB)`. | Server Link supplies the existing entry points. Projects and the paired tests start the real app, then can restart it with the same database and fixture options. |
| `servertest.Options.LinkSeams.BeforeEventCommit func(context.Context, string, uint64) error`. | Server Link supplies the field through the matching `app.LinkSeams` and `link.Seams` fields. Projects and paired tests set it. The arguments are the authenticated machine identifier and event number. Call after report handling for an eligible next event, before updating the mark or settling due results in the same transaction. An error rolls back and returns the database failure without an acknowledgement. Production passes nil. |
| `servertest.Options.LinkSeams.BeforeEventAck func(context.Context, string, uint64)`. | Server Link preserves the independent observer after commit and before the acknowledgement. Test writers use it to hold the writer. The arguments are the machine identifier and committed mark. Item 5 rechecks exact stream identity after the observation and before sending. |
| `servertest.Options.ConfigurePostgres func(*pgxpool.Config)`. | Server Link supplies the field. The paired tests set up the commit-reply observation. Each fixture launch copies its stored PostgreSQL configuration, calls the option on that copy, then passes the configured copy into the real app. Restart repeats this without mutating the saved baseline. |
| `clienttest.Options.AfterFolderReport func(context.Context, uint64) error`. | Client Folders supplies the field. The paired tests use the event number to observe or interrupt the client after durable folder report append and before receipt completion or handoff. An error ends the fixture run. |

- Write `TestServerLinkRevisionTwoWiring` in `src/server/servertest/link_v2_test.go` first. Start the real application fixture with both observers and a pool configuration callback that records an actual connection trace. Join and approve through the existing services and send valid Facts after Welcome using the active revision. Run the same test again on the integrated revision 2 candidate. The test seam sees the expected machine and event and refuses the write. The live handshake has no acknowledgement for that input and the restarted Welcome has the earlier mark. Let the next write succeed. Confirm that the acknowledgement observer runs only after commit, one acknowledgement arrives and a later Welcome carries the committed mark. Check migration 3 and the pool trace on both launches and assert a fresh configuration copy at each launch.
- This item is risky because startup could wire the seam only after commit or use separate Link and Machines state. Detach the pre-commit seam, move it after commit, or bypass the configured pool on restart. Require the restart mark or connection trace assertion to fail. The integrated folder settlement case exercises the real Projects subscriber.

**Sprint 3 frozen test changes**

The following is the allowlist for the test writer. When a test uses a shared helper, update the helper inputs once and preserve its inherited assertions. Keep negative old revision coverage explicit. The orchestrator updates frozen hashes after reviewing these edits.

| Existing test or fixture | Required revision 2 edit |
|---|---|
| `handshake_test.go` with `TestHandshakeOpensStream`, `TestReplayedProofRefused`, `TestOpenRefusesUnusableRegisteredKey`, `TestHandshakeDecisionOrder`, `TestHandshakeValueRules`, `TestRemovalOrdersAdmission`, `TestStaleAdmissionCannotStartWelcome`, `TestWelcomeStartedBeforeReplacement`, `TestAdmissionCommitReplyLost` and `TestScriptedMachineJoinsAndConnects`. | Use revision 2 for valid peers and Welcome assertions, revision 1 for the mismatch row and its updated sentence. Add a stable valid run and first number to `helloFrame` and `submitProof`. Use the same run on repeated calls and when `TestHandshakeOpensStream` advances the mark to 42. A malformed-proof fixture with a preloaded mark still exercises proof before history validation. Retain proof vectors and cutoff. In `TestHandshakeDecisionOrder`, use unsupported revision 1 in the unavailable, unknown machine and mismatch rows so refusal precedence stays under test. (critic-r1 05) Fixtures sensitive only to a Challenge may keep either revision. |
| `review_admission_test.go` with `TestRemovalAfterReplacementCommitBeforePublication`, `review_c_admission_panic_test.go` with `TestAdmissionCommitPanicAllowsCommandsAfterReconnect`, `review_g_admission_panic_test.go` with `TestReviewGAdmissionCommitPanicUnregistersFailedHandle` and `TestReviewGCommittedAdmissionPanicCleansExactStream`, `review_handshake_test.go` with `TestProofRequestClosureAtCutoff`, and `review_outbox_test.go` with `TestWatchdogAbortsDisplacedStreamBeforeReplacementPublication`. | Use revision 2 on the directly constructed current peer. Preserve each assertion about lost commits, panics, cutoff and terminal stream behavior. `TestReplacementWelcomeCarriesCommittedMark` in `stream_test.go` already uses the constant and needs the stable pair supplied by the shared helper. |
| `outbox_send_test.go` with `openCommandPeer` for `TestCommandSendDeadline` and `TestCommandSendPoolHeadroom`. | Send revision 2 and a valid run pair. Preserve the three-permit, blocked-write and heartbeat assertions. |
| `presence_test.go` with the `presenceFixture.admit` helper and `TestBecomeCurrent`; `inbox_test.go`, `oneline_test.go`, `outbox_result_test.go` and `outbox_send_test.go` with direct replacement calls. | Supply the new `RunAnnouncement` to direct admissions. Reuse a fixture run per machine and pick a valid initial boundary for the seeded mark. The direct callers are `TestEventFormsOneLine`, `TestWritesOfOneMachineFormOneLine`, `TestResultAndReplacementFormOneLine` and `TestCommandSendRechecksBeforeWrite`. Keep their PostgreSQL and stale-stream assertions. Update the live manifest version in `TestBecomeCurrent` to 3. |
| `src/server/internal/machines/commands_test.go` and `stream_projection_test.go` with direct calls to `BecomeCurrent`. | Update the shared presence fixture and the direct replacement cases to pass the same pair. Keep the read and queue assertions in `TestProjectStreamPresence`, `TestWatchClientsStreamPresence` and `TestScanAndReconnectQueueCommands`. |
| `outbox_test.go` with `TestCommandMigrationUpgradesPartB`. | Cap the tested manifest at migration 2 and seed the migration 1 connection without invoking runtime revision 2. Preserve the old-schema behavior while item 2 proves the next migration. |
| `src/server/internal/app/link_stream_test.go` and `review_commands_test.go` with `appChallenge` and `openCommandReviewPeer`. | Send the scripted run pair and revision 2. In `TestServerRestartsAfterInterruptedCommandMigration`, check latest migration 3 instead of latest 2. Keep the startup and shutdown assertions. All callers of the scripted `machinetest.Machine.Connect` get the revision and stable pair from that fixture. |
| `src/server/internal/app/commands_test.go` with `TestServerCommandSweepLifecycle` and its `seedStoppedCommandSweep` helper. | Expect startup to apply manifest version 3. Preserve the pending and timed-out command assertions. |
| Orchestrator fixture `src/linktest/outbox_test.go`, helper `authenticateScriptedLink`, and caller in `src/linktest/reserved_result_test.go`. | Pass revision 2 with an explicit fresh version 7 run and first event number. In `TestLinkOutboxReplayAfterServerFailure` use its known committed mark plus one. In `TestLinkReservedCommandResult` use 1 for the fresh machine. Preserve proof, repeats, gaps, unknown fields and reserved result assertions. Keep the separate revision 1 refusal test. The orchestrator owns these edits and the frozen hashes. (architect-r1 05, logic-r1 05, critic-r1 05) |

Transport tests that marshal a synthetic revision 1 Welcome to test raw framing keep those bytes. The sprint 3 proof examples and historical migration files remain frozen. The source survey records the exact affected helper callers.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| The integration assembly applies the migration but never registers or uses the new link boundary. | High | Exercise a real application write, compile Server Link, Projects and Machines together and run the real two-program tests. |

**Verify.** Run `bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/link ./src/server/internal/machines ./src/server/internal/api ./src/server/internal/app ./src/server/servertest -count=1` and repeat with `-race`. Use the applicable package tests on the isolated work commits. Require explicit passes and zero skips for the named tests.

#### Integrated candidate gate

The orchestrator uses `LGTM_LINK_BASE` set to the immutable landed revision 2 contract commit and runs `bash deploy/test/with-postgres.sh -- bash scripts/check-go.sh` with Link, Machines, Projects, both programs and the updated paired fixtures. The revision 1 pin `fcfa96c3` is the Contract review baseline. Run `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^Test(SprintFour|LinkOutboxReplayAfterServerFailure$|LinkReservedCommandResult$)' -count=1`. Require explicit passes without skips for both retained tests and `TestSprintFourHistoryRecovery`, `TestSprintFourPendingRun`, `TestSprintFourWaitingReplay`, `TestSprintFourReceiptReplay` and `TestSprintFourFolderSettlement`. Run real Projects startup and rollback using item 8’s fixture. Keep `deploy/test/checks-s4.sh` at commit `22ca76c6`. Stack requires passes for R1, R2, R4, R11, R12 and R13, and observes R14 on the deployed stack. (architect-r1 02, 05, 06 and 08)

#### Post-build checks

| Scenario | Expected result |
|---|---|
| Hold the first real folder worker at the client seam while a second call queues a link or subfolder of the same checkout. Allow a heartbeat before releasing the first within its lifetime. | The stream stays current, the second command waits, each inspection and operation completes once in order, and both add calls return the same stored folder. (logic-r1 02 and P1) |
| Hold the last replay acknowledgement. Replace the stream or remove the machine after commit. Repeat with the event write faulted before commit. On the current stream exercise event 33. | The obsolete writer starts no acknowledgement or command. The new Welcome reports the durable mark. A successfully replayed report commits once. The live client waits for credit before handing over event 33. (logic-r1 P2, threat-r1 PB3) |
| Stage three results, two at equal watermarks and one at a lower watermark with a higher command sequence. Restart and send a repeat and then the needed events. | Each settles once in watermark and then sequence order, with no resend or sweep. (logic-r1 P3) |
| Race an applied Projects report against timeout and removal in both machine-lock orders. Try waiting and wrong-machine reports and an unlisted browser origin against the assembled app. | Report-first preserves its terminal reply. Timeout or removal first preserves the corresponding failure. An eligible late report can advance the mark, but never changes terminal data. Removal cleans up folders. Only eligible authorized inputs change Projects, and a denied browser call reads no records. (logic-r1 P4, threat-r1 PB1) |
| Lose the diagnostic commit reply before and after commit. Separately hold three command permits and fault actual Projects handling after the report callback. Reopen and retry. | The incumbent remains, and the warning and its feed row agree. The interrupted report leaves its folder, operation, mark, stage and feed unchanged, with no acknowledgement. Retry commits one report and settlement. (threat-r1 PB2) |
| Capture real logs, feeds and replies with synthetic credential markers on rejected and malformed inputs. During the verified legacy upgrade check private fixture modes and both certificate pin outcomes. | Logs, feeds and replies contain no credential or raw diagnostic. A mismatched key forwards no request. An upgraded client replays the retained event once and returns the legacy zero receipt without rerunning work. (threat-r1 PB4) |
| Remove a flagged machine with folders in 257 repositories, both shared and orphaned, plus a staged command. Fault the cleanup and then retry. Separately force natural watch lag with several legal writes. | Rollback leaves domain rows and feed intact. A committed retry settles the command as removed and the live repository watch reopens after one refresh. Its positioned snapshot keeps only surviving repositories and folders. Natural lag still uses the existing watch behavior. (architect-r1 03, logic-r1 03, critic-r1 01, threat-r1 02) |
| Register two parts in startup order and repeat with a duplicate route. | Both lifecycle callbacks share the write. Each typed callback sees only its route. Duplicate registration stops startup before serving. (architect-r1 02) |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Foundation decision, rule and check numbers follow their source order. The source universe consists of assignment points 1 to 7, the foundation rules 1 to 11 and checks C1 to C13, the newer design's four Link points, sprint 4 checks R1 to R14 and open question 2, the Link row and names in the contract fold result, both round 3 closing lists, the two sprint 3 plans and their review registers, issues 1632 to 1635, 1667 and 1676, choice 18, and build loop phase 4. Foundation decisions D1 to D5, D7, D13, D16 and D17 are the decisions that the Link boundary touches.

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
| 1 | backend | server/backend | medium | A small public boundary needs a concurrency proof and compatibility with the existing queue. | yes | Reviewed contract and Server Store `RecordOnce`. |
| 2 | backend | server/backend | medium | One additive migration and two prior-schema fixtures preserve command state. | yes | Reviewed contract. |
| 3 | backend | server/backend | medium | Extend the existing handshake and one locked admission with a finite set of predicates. | yes | 1, 2 and 8. |
| 4 | backend | server/backend | medium | A single generation fence joins diagnostic persistence to the existing watch projection. | yes | 3. |
| 5 | backend | server/backend | medium | Gate one writer and use the existing claim lock and event transaction. | yes | 4. |
| 6 | backend | server/backend | medium | Reuse one terminal transition under the existing lock across result, event, sweep and removal. | yes | 1 and 2 for outbox work, then 4 and 5 for shared files. |
| 7 | backend | server/backend | medium | Route one action and one report with the existing cursor and codec patterns. | yes | 5, 6 and 8. |
| 8 | backend | server/backend | low | Carry two fixture options through the existing start and restart paths. | yes | 2. |

OpenAI supplies the planner, review fold, test writers, builders and reviewers. The plan counts 30 launches across planning, the fold, four plan reviewers and three seats for each of eight build items. The orchestrator checks live allowance before dispatch and reserves complete test, build and review units. Rework and combined tests need separate capacity.

</details>

<details><summary><b>Attack log (gate 7, Tier M)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| [Round 1](attack/round-1-findings.md) | 26 findings in 14 groups. | Folded the repairs and recorded integrated observations. The orchestrator owns the build handoff. |

</details>
