REVISE

Machine removal can roll back indefinitely once that machine has folders in enough repositories to exceed the Store's 256-reference limit.

OpenAI supplies the planner, the test writers, the builders, and this reviewer.

The source baseline is `0f5f9c72`. A byte comparison confirmed that `plan.md` matches the frozen `plan-v1.md`. The consequences below follow from the plan and the source. The sprint 4 runtime checks are build acceptance.

## Findings

### critic-r1 01 · MAJOR · R4 · Removal exceeds the change feed limit

The Link plan leaves large removals to Projects and Store. The Projects plan records one change per affected repository, and a machine removal already records a Machines change. Store aborts a write at reference 257. Its frozen test checks that the domain write and feed both roll back. The sprint 4 design requires all affected folders, operations, and orphan repositories to change in the removal write. Evidence: `.plans/plans/go-s4-server-link/plan.md:231`, `.plans/plans/go-s4-server-link/plan.md:254`, `.plans/plans/go-s4-server-projects/plan.md:166`, `src/server/internal/machines/store.go:187`, `src/server/internal/store/feed.go:14`, `src/server/internal/store/feed.go:70`, `src/server/internal/store/feed_test.go:405`, and `docs/specs/2026-11-27-running-an-agent-design.md:253`.

Remove a machine with one folder in each of 256 repositories. The machine change alone puts the write over the cap. With a pending command, the Link wake can reach the cap a repository earlier. A retry against the same data fails again. The existing recovery action has the same failure for a flagged machine.

Resolve the Store and Projects handoff before freezing these tests. Specify how a single committed removal can notify every affected watch within a bounded write protocol. For example, agree on a Store and watch invalidation reference that forces a replacement snapshot for this case and leaves the existing per-record cap intact. Freeze a real database test for 256 affected repositories and a staged command, then check the machine and command changes and the resulting repository snapshot through live watches. The Projects lag test also needs several legal writes to cross the 2,048-position watch bound. Its proposed single write cannot reach that bound. Evidence: `.foundry/go-rewrite/results/plan-go-s4-server-link.md:26` and `.plans/plans/go-s4-server-projects/plan.md:203`.

Category and evidence. Bug, confirmed by reading. Server Projects and Store own the repair, with Link checking the combined write. Risk score 120 = 6 × 5 × 4. An owner loses a core removal action, repair requires intervention, and accumulating this many repositories on one machine is uncommon.

### critic-r1 02 · MAJOR · R3 · Projects and Link specify incompatible callback types

Link specifies `RegisterPart(PartHooks) error` with `ReportApplied` and `CommandSettled` maps. The report receives `*linkv1.Event` and the settlement receives `link.Settlement`. Projects requests `RegisterPart(PartHooks)` with no return, an `OnFolderSeen` field that receives `*linkv1.FolderSeen`, and a settlement field that receives three strings. Its startup step names the three service methods without an adapter. Evidence: `.plans/plans/go-s4-server-link/plan.md:80`, `.plans/plans/go-s4-server-link/plan.md:117`, `.foundry/go-rewrite/results/plan-go-s4-server-link.md:26`, `.foundry/go-rewrite/results/plan-go-s4-server-projects.md:41`, `.foundry/go-rewrite/results/plan-go-s4-server-projects.md:52`, and `.plans/plans/go-s4-server-projects/plan.md:209`.

A Projects test writer can freeze a startup registration that uses `OnFolderSeen: service.ApplyFolderSeen`. It cannot compile against the Link type. The two plans agree on the enqueue and presence signatures, the removal callback arguments, and the shared startup file.

Publish one exact registration contract in both handoffs. If the Link type stands, specify `(*projects.Service).LinkHooks() link.PartHooks`, including the closures that extract `GetFolderSeen()`, the stored command identifier, and the normalized outcome. Check the registration error before starting workers. Freeze the adapter check and compile the three server parts with their fixtures at the shared handoff.

Category and evidence. Design, confirmed by reading. Server Link and Projects own the repair. The combined compilation gate applies.

### critic-r1 03 · MAJOR · R6 · Removal callbacks can leave new commands for a deleted machine

The plan permits a transactional callback to enqueue for its locked machine. Removal selects the unfinished commands before calling settlement and machine-removal hooks. The current Machines entry point deletes the registry row before calling `Forget`. The command table has no foreign key to that row, and the sweep skips unfinished commands for an absent machine. Evidence: `.plans/plans/go-s4-server-link/plan.md:80`, `.plans/plans/go-s4-server-link/plan.md:230`, `src/server/internal/machines/records.go:59`, `src/server/internal/link/migrations/0002_link_commands.sql:1`, and `src/server/internal/link/outbox_sweep.go:117`.

A part follows the advertised callback contract and enqueues from an `OutcomeRemoved` settlement or removal hook after Link has collected its batch. The removal can commit with an unfinished command that no stream can claim and the sweep cannot finish. The generic boundary will be used by parts that enqueue from other lifecycle and settlement callbacks in sprint 5. Evidence: `docs/specs/2026-11-27-running-an-agent-design.md:493` and `docs/specs/2026-11-27-running-an-agent-design.md:684`.

Declare removal and removal settlements ineligible to enqueue and enforce that rule at the enqueue boundary using the registry in the same write. Extend `TestLinkRemovalSettlesAndCallsParts` with a removal callback that tries to enqueue. Assert the fixed refusal, no command or wake from the attempted enqueue, and successful completion for a callback that handles the refusal.

Category and evidence. Design, confirmed by reading. Server Link owns the repair. Risk score 60 = 6 × 5 × 2. An orphan ledger row persists without repair, the owner needs help to clear it, and it requires a removal callback that tries to enqueue.

### critic-r1 04 · MAJOR · R10 · The replay work item asserts behavior built in later items

Item 5 expects its test to observe a report hook before the ready transition, yet item 7 is the first item that routes `ReportFolderSeen` and it depends on item 5. The same item 5 waiting test expects no folder callback without an approved positive control. At present Facts and Harnesses Found go directly to the registry, with no registered report hook. Evidence: `.plans/plans/go-s4-server-link/plan.md:188`, `.plans/plans/go-s4-server-link/plan.md:194`, `.plans/plans/go-s4-server-link/plan.md:195`, `.plans/plans/go-s4-server-link/plan.md:237`, `src/server/internal/link/inbox.go:75`, and `.plans/epics/go-rewrite/epic.md:69`.

The test writer can freeze a positive FolderSeen callback observation for item 5. That item cannot turn the assertion green inside its own scope. If the test checks only the absence of a callback for a waiting machine, it passes even if folder routing is absent for approved machines.

Use an existing Facts effect for item 5's rollback and callback-order fixture, such as a database sentinel from the Facts write that the ready hook can read. Assert that a failed hook leaves the Facts effect and acknowledgement absent, with the mark and ready run unchanged. Move the FolderSeen positive control and the FolderSeen waiting assertion into item 7 or the paired test, and retain the item 5 acknowledgement-before-`SetState` test using the existing event kinds. Specify that item 6 adds due-settlement ordering and owns the test that observes it.

Category and evidence. Design, confirmed by reading. Server Link owns the repair. The per-item frozen-test gate applies.

### critic-r1 05 · MAJOR · C11 · Two frozen paired tests still send a revision 1 greeting

The Link allowlist covers the server-side peers, but the full-suite gate also runs the two-program tests. `authenticateScriptedLink` hardcodes revision 1 and expects Welcome. It is used by `TestLinkOutboxReplayAfterServerFailure` after the real client has advanced the mark, and by `TestLinkReservedCommandResult` for a newly approved machine. The Client Folders plan already assigns this helper to the orchestrator. Evidence: `.plans/plans/go-s4-server-link/plan.md:279`, `.plans/plans/go-s4-server-link/plan.md:302`, `src/linktest/outbox_test.go:326`, `src/linktest/outbox_test.go:408`, `src/linktest/reserved_result_test.go:55`, and `.plans/plans/go-s4-client-folders/plan.md:243`.

On a revision 2 server the helper receives the revision refusal. Changing the revision alone still fails. Using first number 1 for both callers also fails when the replay test hands off from the real client at a positive mark. The replay and reserved-field observations never run.

Put the orchestrator-owned exception in both runbooks and in the shared frozen-test handoff. Pass an explicit new run identifier with first number `current+1` for the completed replay case and 1 for the newly enrolled machine. Keep the existing inherited assertions. In `TestHandshakeDecisionOrder`, also keep an unsupported revision in its database-unavailable and unknown-machine rows, to preserve their precedence assertions when revision 2 becomes supported. Evidence: `.plans/plans/go-s4-server-link/plan.md:285` and `src/server/internal/link/handshake_test.go:646`.

Category and evidence. Bug, confirmed by reading. The orchestrator and Server Link own the fixture changes. The frozen paired-suite gate applies.

### critic-r1 06 · MINOR · C11 · The admission tests omit the two orders of a stage racing a replacement

The plan tests a preseeded stage against a new admission and tests an obsolete stream with a result. It does not specify a paired assertion where those writes race with the same candidate and original stream. Evidence: `.plans/plans/go-s4-server-link/plan.md:172`, `.plans/plans/go-s4-server-link/plan.md:176`, `.plans/plans/go-s4-server-link/plan.md:228`, and `.plans/plans/go-s4-server-link/plan.md:233`.

With mark 5, a dispatched attempt on A can stage watermark 9. An unseen B can announce first number 9. If the result commits first, B is refused at the stage fence and A stays current. If B commits first, the old frame is obsolete and the command has no stage. The two independent tests can pass while an implementation splits the durable fence read from admission.

Use the machine lock to freeze both orderings and assert the current stream, stage, warning, attempt, and acknowledgement or response for each. Keep the settlement assertion separate from any final status choice for the displaced stream.

Category and evidence. Design, confirmed test gap by reading. Server Link owns the repair. This adds a test to the admission and result gate.

### critic-r1 07 · MINOR · plan · Item 2 omits files it tells the test writer to edit

Item 2 says it changes only the new migration and test plus `outbox_test.go`. It then directs updates to live version assertions in three existing test files. Its last instruction about seeding a version 1 connection follows a list of four tests, so its referent is unclear. Evidence: `.plans/plans/go-s4-server-link/plan.md:131`, `.plans/plans/go-s4-server-link/plan.md:136`, `.plans/plans/go-s4-server-link/plan.md:288`, `.plans/plans/go-s4-server-link/plan.md:290`, `.plans/plans/go-s4-server-link/plan.md:291`, and `.plans/plans/go-s4-server-link/plan.md:292`.

A test writer following the item-level file fence keeps the existing startup assertion for version 2. That assertion turns red once migration 3 lands, despite correct production behavior. A writer following the later allowlist enters files absent from the unit's edit list.

Add the exact paths of `presence_test.go`, `src/server/internal/app/commands_test.go`, and `src/server/internal/app/review_commands_test.go` to the migration item's fixture edits. Give the raw version 1 seeding direction its own sentence naming `TestCommandMigrationUpgradesPartB`. Anchor the inherited version assertions to their current symbols. Evidence: `src/server/internal/link/outbox_test.go:40`, `src/server/internal/app/commands_test.go:268`, and `src/server/internal/app/review_commands_test.go:233`.

Category and evidence. Design, confirmed by reading. The Server test writer and planner own the repair. The work-item anchor and frozen-test gates apply.

## Answers to the attack questions

1. Items 1, 3, and 4 name tests, inputs, and outcomes for their local gates. Item 2 needs the explicit fixture ownership in finding 07. Item 5 needs a report input it can exercise before item 7. Item 6 needs exact opposite-order expectations for replacement and a result, and the removal callback enqueue case. For item 7, specify where the padding goes in the 65,536-byte outer test so the test writer knows whether the inner event is applied or consumed. Item 8 covers standalone startup and mark rollback. It relies on the paired Projects rollback test for folder, outcome, and feed observations. Evidence: `.plans/plans/go-s4-server-link/plan.md:127`, `:135`, `:175`, `:184`, `:193`, `:232`, `:243`, and `:276`.
2. All eight acceptance rows name an observation. No bare “works,” “correct,” or “fast” appears. “Invalid” and “eligible” depend on the enumerated fixtures. Criterion 8's “combined and deployed gates” points to the verification section, while the owner-facing rollback claim at line 44 needs the paired Projects case identified there. Evidence: `.plans/plans/go-s4-server-link/plan.md:64`, `:154`, `:214`, `:264`, and `:302`.
3. The callback handoffs conflict as shown in finding 02. Item 2's file claim conflicts with its test instructions as shown in finding 07. The same-vendor rule in `.plans/plans/go-s4-server-link/decisions.md:5` and `.plans/plans/go-s4-server-link/plan.md:360` differs from `.plans/epics/go-rewrite/epic.md:72` and `:91`. The later, task-specific instruction governs this review and is explicit in `.foundry/go-rewrite/tasks/plan-go-s4-server-link.md:40`.
4. All implementation paths are named within `src/server/`, allowed by `.plans/epics/go-rewrite/epic.md:135`. Item 2's test paths need the additions in finding 07. Paired-fixture changes go in the orchestrator handoff as in finding 05.
5. Every item is rated low or medium. Split item 6 into result and covered-event settlement, followed by sweep and removal, if one session must produce each frozen gate. Give the first part staged, restarted, and event-settlement tests and the second part a locked sweep and removal test. Items 3 and 4 can overlap only with item 6's `outbox_result.go` and `outbox_sweep.go` work. Items 5 and 6 share `inbox.go`, `outbox_claim.go`, and `outbox_send.go`. Follow the sizing table's serial handoff here and on `presence.go`. Projects waits for the `start.go` handoff. Evidence: `.plans/plans/go-s4-server-link/plan.md:168`, `:188`, `:226`, `:349`, and `.plans/plans/go-s4-server-projects/plan.md:205`.
6. A fresh checkout needs the reviewed generated contract and the PostgreSQL test wrapper. Tests skip without `LGTM_TEST_POSTGRES`. Without PostgreSQL or its settings, startup stops before listening with the existing database diagnostic. A missing web password stops startup after session reconciliation. A second server on the same database gets the existing instance-lock refusal. A second client run can wait behind replay or be refused without displacing the incumbent. After a partial startup, committed migrations survive while startup must reacquire the instance lock and reconcile again. After a partial client run, the durable admission and stage survive. Evidence: `src/server/internal/store/storetest/postgres.go:28`, `src/server/internal/app/start.go:28`, `:153`, `:178`, `:198`, and `.plans/plans/go-s4-server-link/plan.md:172`, `:232`.
7. Gate 1 has closed or expressly assumed decisions with dated sources. Gate 2 names all observations, with the test dependency in finding 04. Gate 3 is inapplicable because the plan adds no screen. Gate 4 names and maps the source universe, but the blanket coverage claim at `.plans/plans/go-s4-server-link/plan.md:326` needs the handoff corrections. Gate 5 has risk and mitigation rows for all four workstreams. Gate 6 has four verification entries and one rollout sequence, with the paired fixture gap in finding 05. Gate 7 is pending in the plan's attack fold at `:368` and can close after adjudication. Gate 8's spot checks of `State.Enqueue`, `Migrations`, and `State.ApplyEvent` match `src/server/internal/link/outbox.go:22`, `migrate.go:14`, and `inbox.go:27`. Its item paths still need finding 07. Gate 9 cites applicable `AGENTS.md` rules. Gate 10 has valid areas and size labels, and gives a launch estimate of 29. The live balance is pending and item 6 should be split as above. Required frontmatter and gate folds exist. The epic reverse link stays pending at `.plans/epics/go-rewrite/epic.md:176`.
8. The plan names the vendor and gives no model or harness name. It first uses `WS` at `.plans/plans/go-s4-server-link/plan.md:58` and expands it at `:308`. Move the expansion to its first use. Turn the `Load.` fragment at `:360` into a sentence or heading under `AGENTS.md:43`. No private paths, network addresses, or personal quotations appeared in the reviewed plan text.

a. The plan addresses the four Link interface changes, revision 2 and the verified revision refusal, run and result fences, folder routing, acknowledgement and size limits, durable recovery projection, and the migration. Findings 01 through 05 leave removal, collaboration, and proof incomplete. Sprint 5 behavior stays outside scope. Immediate and deferred ready callbacks and identifier-returning enqueue support its later start and stop work when the removal exception is specified. Evidence: `docs/specs/2026-11-27-running-an-agent-design.md:82`, `:169`, `:318`, and `:493`.
b. The revision constant and Welcome, signed proof and fixture changes, supersession, warning and uncertain commit, staging and ordered settlement, cumulative acknowledgements, gap handling, size and credit rules, real failure seam, and old Machines queue each have named mechanisms and tests. The wire field names, wrapper names, refusal codes, and Machines flag match the fold. Compilation against Projects and two frozen paired fixtures remain open in findings 02 and 05. The client proves the 32-event sending limit and the server proves one acknowledgement per eligible event. Evidence: `.foundry/go-rewrite/results/fold-go-s4-contract-r2.md:24`, `:34`, `:65`, and `:98`.
c. Contract can land its reviewed generated bindings and limits fixture. Projects matches enqueue, presence, and removal naming but disagrees on the registration and the other callbacks, and no frozen Store-compatible bulk removal mechanism is agreed. Client Folders retains `Join` and `Run` and publishes `AfterFolderReport func(context.Context, uint64) error`. Website uses the generated flag and the approved waiting and offline states. Stack names the same wire, paired cases, and deployment checks. The paired owner needs the explicit frozen helper exception in finding 05. Evidence: `.foundry/go-rewrite/results/plan-go-s4-server-link.md:21`, `.plans/plans/go-s4-client-folders/plan.md:164`, `.plans/plans/go-s4-web-repositories/plan.md:247`, and `.plans/plans/go-s4-stack-checks/plan.md:183`.
d. The proposed items are bounded enough to start once the interfaces agree. Item 5 needs an executable independent test and item 6 is the clearest split. Item 8 can prepare fixture plumbing early and needs the real Projects subscriber for paired acceptance. See lens answers 1 and 5.
e. Several orders intentionally differ. The two tables below record the outcome by winning write and exact stream. A refused candidate and a staged result are independent of the connected stream state.
f. The server allowlist changes the valid Link handshakes, scripted peer, raw command peer, direct admissions, migration assertions, and historical seeding. The fixture table below gives the reason for each group. The independent signed bytes, old unsupported-revision precedence, and synthetic framing cases still apply. Add the two omitted paired-test callers from finding 05 and preserve their original assertions.
g. OpenAI supplies the planner, the test writers, the builders, and this reviewer, as stated at the top.

### States for one machine

| State | Durable and visible behavior | What can move it |
|---|---|---|
| No stream | A registered machine retains the event mark, admitted and ready runs, warning, queued commands, and stages. There is no current sender. | Successful admission installs a stream. Removal ends retained work. Restart may sweep a due ordinary command. |
| Stream waiting behind the barrier | The latest admitted run is durable. The previous ready run may still be recorded. Heartbeats and eligible events proceed without command claims or permits. | The committed mark reaches the boundary. The writer sends the last replay acknowledgement before it opens its send gate. |
| Ready stream | The latest run is ready, and the writer can send after Welcome or the final replay acknowledgement. Approval rules still apply. | Successful replacement, disconnect, restart, or removal ends the exact stream. A later ready admission can clear the warning. |
| Refused new run | The candidate has no Welcome. The incumbent, if present, retains its state. A signed, unseen, behind-mark run sets or refreshes the warning. Other run fences leave the warning as it was. | A later ready admission clears the warning when its generation is above the recorded fence. Removal deletes it. |
| Result waiting for an event | The first eligible result remains staged across any stream state. It is excluded from offers and the timeout sweep. | A real eligible event covers the watermark and settles it in the same write. Removal settles it as removed. |

### Orders that can overlap

These rows use the same machine, an eligible related event and result, and a due sweep where applicable. The machine lock orders committed writes. An old frame means a frame on the pre-replacement transport. Evidence: `.plans/plans/go-s4-server-link/plan.md:172`, `:181`, `:190`, `:228`, `:231`, and `:242`.

| Pair | First order | Reverse order and convergence |
|---|---|---|
| Reconnect and event | Successful reconnect displaces the old stream. Its old frame gets no acknowledgement. | The event can advance or promote first. An unseen reconnect can then be refused as behind the new mark. Valid replay on a current stream gives one report effect. |
| Reconnect and result | A successful replacement makes the old result ineligible. | An eligible result can stage or settle first. A new run with first number at or below an existing stage can be refused. The two orders can differ. |
| Reconnect and sweep | An admitted stream leaves an unanswered, sweep-eligible command subject to cutoff. | The sweep may settle it before admission. For a fixed eligible cutoff and absent a further result, the same timeout stands. Existing stages survive both orders. |
| Reconnect and removal | Admission may commit before removal and may become visible briefly. | Removal first leaves no machine to admit. Both orders end with the machine removed. |
| Reconnect and restart | An admission committed before restart survives as a run record, and its live connection ends. | A reconnect after restart can be live. Its valid run pair identifies the same durable admission, while the connection state differs. |
| Event and result | The real report and mark commit. The eligible result settles immediately when its watermark is covered. | The same result stages first and is settled by the event write. Both orders let the report win when the result's watermark includes its report. A result whose watermark was already covered can close the operation before a later report. |
| Event and sweep | A real report can complete Projects before the sweep times out an unanswered command. | The sweep first can close the operation, and the later report is consumed without reopening it. An existing stage is ineligible for either sweep. |
| Event and removal | A valid report can commit a terminal success before removal cleans up the saved folder. | Removal first closes the open operation as removed and the old event has no effect. Terminal replay can differ. |
| Event and restart | A committed mark and report survive even if the acknowledgement never arrives. | Restart first ends the old stream. Valid replay on a new admission can apply the event once or acknowledge the prior mark. |
| Result and sweep | An eligible result stages or settles, so the sweep skips it. | A timeout can win first and the result cannot overwrite it. This is tested in item 6. |
| Result and removal | A covered result can settle first. An early stage still ends as removed. | Removal first leaves no eligible attempt for the old result. A result that had already settled can retain a different terminal outcome. |
| Result and restart | A committed stage or final result survives restart. | Restart first ends the old attempt's live stream. A receipted result can return on a new eligible attempt if it is still offered. The timing can differ. |
| Sweep and removal | A committed timeout remains the command's terminal record. | Removal first writes `removed` for unfinished commands and the sweep skips them. The terminal outcomes can differ. |
| Sweep and restart | A committed timeout is durable. Stages remain. | Startup runs its own sweep. With the same cutoff and eligibility, it converges on one terminal outcome. |
| Removal and restart | A committed removal survives restart. | Restart rolls back an uncommitted removal. A retry can complete it against the same durable state. |

### Frozen sprint 3 fixtures that require a revision 2 edit

These are the groups in `.plans/plans/go-s4-server-link/plan.md:283` through `:292`, plus the paired tests in finding 05.

| Frozen tests or helpers | Why the revision requires the edit |
|---|---|
| `TestHandshakeOpensStream`, `TestReplayedProofRefused`, `TestOpenRefusesUnusableRegisteredKey`, `TestHandshakeDecisionOrder`, `TestHandshakeValueRules`, `TestRemovalOrdersAdmission`, `TestStaleAdmissionCannotStartWelcome`, `TestWelcomeStartedBeforeReplacement`, `TestAdmissionCommitReplyLost`, and `TestScriptedMachineJoinsAndConnects`. | The shared current-peer Hello and Welcome use revision 2 and a valid run pair. Failure rows that rely on an unsupported revision must still send one. A fixture that only needs a Challenge can keep its revision. |
| `TestRemovalAfterReplacementCommitBeforePublication`, `TestAdmissionCommitPanicAllowsCommandsAfterReconnect`, `TestReviewGAdmissionCommitPanicUnregistersFailedHandle`, `TestReviewGCommittedAdmissionPanicCleansExactStream`, `TestProofRequestClosureAtCutoff`, `TestWatchdogAbortsDisplacedStreamBeforeReplacementPublication`, and `TestReplacementWelcomeCarriesCommittedMark`. | The valid admissions require the same current-peer input change. Commit, panic, cutoff, and replacement assertions carry forward. |
| `openCommandPeer` for `TestCommandSendDeadline` and `TestCommandSendPoolHeadroom`. | The shared peer needs a valid revision 2 admission before either test can exercise send behavior. |
| `presenceFixture.admit`, `TestBecomeCurrent`, `TestEventFormsOneLine`, `TestWritesOfOneMachineFormOneLine`, `TestResultAndReplacementFormOneLine`, `TestCommandSendRechecksBeforeWrite`, `TestProjectStreamPresence`, `TestWatchClientsStreamPresence`, and `TestScanAndReconnectQueueCommands`. | Direct admissions need the new argument and a boundary appropriate for the fixture mark. `TestBecomeCurrent` also observes current schema version 3. |
| `TestCommandMigrationUpgradesPartB`. | Use a manifest ending at 2 to retain the historical assertion. Seed the version 1 connection directly because the current admission needs schema version 3. |
| `appChallenge`, `openCommandReviewPeer`, `machinetest.Machine.Connect`, `TestServerRestartsAfterInterruptedCommandMigration`, and `TestServerCommandSweepLifecycle`. | Scripted current peers need the run pair. The two current startup checks need schema version 3. |
| `TestLinkOutboxReplayAfterServerFailure` and `TestLinkReservedCommandResult`, through `authenticateScriptedLink`. | The orchestrator's scripted peer needs revision 2 with an explicit new run and the correct first number for each caller. |

## Post-build checks

- Hold the acknowledgement immediately after the final replay event commits, then replace or remove the stream. The old stream sends no later command or acknowledgement after it loses its fence. An eligible repeat on the current stream receives the saved mark with one committed report effect.
- With Projects mounted and three send permits occupied, stage a folder result and interrupt the real event write after Projects saves the folder. Observe the operation, folder, result, mark, change feed, and acknowledgements through independent endpoints. The failed attempt leaves the stage and prior mark intact. Replay commits the folder once and settles the result once.
- Lose the diagnostic commit reply while the incumbent can still send. On an independent database connection compare the flag with its feed change. Reopen the authenticated Machines watch and check the same state. The candidate gets the existing database error and the incumbent continues with the same stream and generation.

## Grade

6/10. Zero blockers, five majors, and two minors.
