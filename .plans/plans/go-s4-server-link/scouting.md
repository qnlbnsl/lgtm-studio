# Source scouting for the sprint 4 server link

The source scan used repository commit `44c931d9` as its base on 2026-11-27. The sprint 4 contract names come from its reviewed plan and fold result. The checked-in revision 1 source is the baseline for the Go and Protocol Buffers anchors.

## Sources that set the scope

| Source | Sections or facts checked |
|---|---|
| [Repository instructions](../../../AGENTS.md), [assignment](../../../.foundry/go-rewrite/tasks/plan-go-s4-server-link.md) and [common planner instructions](../../../.foundry/go-rewrite/common-planner.md). | The assignment restricts planning writes and defers integration. It fixes the test order, scope, review handoff and output format. |
| [Foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md). | Read completely. Numbered the decisions and rules in source order. Used the database and feed rules, client proof, command receipts, event order and the thirteen foundation checks. |
| [Running an agent design](../../../docs/specs/2026-11-27-running-an-agent-design.md). | Read completely. Used the four Link interface points, run barrier, result clocks, budgets, folder operation and removal rules and R1 to R14. Read sprint 5 to check the start identifier, stop enqueue, exact run change and removal call sites the interface needs to permit. |
| [Contract plan](../go-s4-contract/plan.md) and [round 2 fold result](../../../.foundry/go-rewrite/results/fold-go-s4-contract-r2.md). | Read the Link source edits, run fence, waiting approval, result fence, buffer loss, budget table and both binding handoff tables. The fold request distinguishes a diagnostic refusal from a possible replacement. |
| [Logic closeout](../../designs/running-an-agent/round-3-logic.md#for-the-plans) and [fit closeout](../../designs/running-an-agent/round-3-fit.md#for-the-plans). | Traced the three permits, cursor, startup registration and byte boundaries to items 5, 7 and 8. Logged all other closing lines in the decision record. |
| [Stream plan](../go-s3-server-stream/plan.md), [stream findings](../go-s3-server-stream/attack/round-1-findings.md), [post-build stream review](../go-s3-server-stream/attack/post-build-e.md), [command plan](../go-s3-server-commands/plan.md) and [command findings](../go-s3-server-commands/attack/round-1-findings.md). | Checked the writer permit and current stream fences, proof precedence, the durable dispatch marker, sweep cutoff and scan cursor. Reviewed the command post-build findings on the next scan, reserved result, startup sweep and shutdown. The terminal outcome in issue 1667 stays open. |
| [Epic teams and plans](../../epics/go-rewrite/epic.md), [build loop phase 4](../../../docs/specs/2026-11-05-build-loop-design.md#4-build), [issues](../../../docs/issues.md#1632-what-the-design-of-the-go-rewrite-leaves-open) and [choice 18](../../../.foundry/go-rewrite/owner-choices.md). | Checked the Server scope, test freeze, plan slot, stack trust, real paired fixtures, size bands and excluded replacement-terminal choice. Searched `floorplan/plan.json` for the existing server outbox paths. The epic row awaits the orchestrator's integration edit. |

## Code anchors read in the checkout

The anchors below describe revision 1 by reading its source. The new test and runtime names under them are planned.

| File and symbol | What the source establishes |
|---|---|
| [Link outbox](../../../src/server/internal/link/outbox.go#L22), `State.Enqueue` and `commandAction`. | The error-only queue allocates an identifier, strips envelope fields and records `machine_outbox` inside the caller's write. The command payload survives claim and send. |
| [Link presence](../../../src/server/internal/link/presence.go#L23), `State`, `NewState`, `State.BecomeCurrent`, `State.Presence`, `State.Forget`, `State.ClearConnections` and `State.CheckCurrent`. | Admission takes the Machines lock and updates the generation and connection in one write. Current presence reads the connection row, so an offline recovery warning needs a query rooted in the durable Link record. Removal finishes the ledger then removes the Link rows. Startup clears exact connection rows while preserving the machine mark. |
| [Link handshake](../../../src/server/internal/link/handshake.go#L149), `Service.handshake`, `ServerProofText`, `MachineProofText` and `openStatus`. | Challenge precedes lookup and revision decisions. Successful Proof precedes the admission write. Live revision 1 compares revision before checking Proof. |
| [Link hub](../../../src/server/internal/link/hub.go#L99), `Hub.Admit`, `Hub.BeginWelcome`, `Hub.holds` and `Hub.removalCause`. | A potentially committed replacement discards the incumbent handle. A known failed candidate does not. A refusal write that never installed a connection belongs to the second path even if its diagnostic commit reply is lost. |
| [Link writer](../../../src/server/internal/link/stream.go#L28), `Service.serve`, `Service.dispatch` and `streamingService.checkFrame`. | One writer dispatches inbound messages and writes acknowledgements before its next command turn. Unknown and malformed frames use the current stream check. `BeforeEventAck` runs after `ApplyEvent` returns from the database. |
| [Link inbox](../../../src/server/internal/link/inbox.go#L27), `State.ApplyEvent`, `applyReport` and `validReportText`. | The current stream and number decide eligibility before report inspection. A write applies or consumes the next event, advances the mark and touches Machines. Repeats return the committed mark. |
| [Link claim](../../../src/server/internal/link/outbox_claim.go#L38), `State.claimNext`, `readClaimPage`, `revisitScans` and `assignCommand`. | The cursor advances only with a successful claim write. Approval resets the cursor. A deferred scan is reconsidered if the current scan is no longer eligible. |
| [Link send](../../../src/server/internal/link/outbox_send.go#L42), `commandSlots.acquire`, `State.SendNext` and `State.sendClaimed`. | The real shared permit precedes the claim. The fenced network write precedes the persisted dispatch marker. The replay wait must stay outside the shared permit code. |
| [Link result](../../../src/server/internal/link/outbox_result.go#L12), `State.AcceptResult`, `lockCommandStream` and `commandOutcome`. | The durable stream, dispatch, approval and attempt qualify a result. The old result path immediately clears the payload. The retained action is needed by a part's settlement hook. |
| [Link sweep](../../../src/server/internal/link/outbox_sweep.go#L50), `State.SweepCommands`, `sweepCommandBatch` and `RunCommandSweep`. | Select bounded pages, lock by machine and check eligibility again. The clock's terminal path keeps a distinct `timed_out` outcome. |
| [Link startup and workers](../../../src/server/internal/link/run.go#L17), `Options`, `Seams`, `NewStreamingService` and `Service.Run`. | The application passes command state and fixture callbacks here. The follower and hourly sweeper start from this service. |
| [Embedded migration list](../../../src/server/internal/link/migrate.go#L13), `Migrations`, and [migration 2](../../../src/server/internal/link/migrations/0002_link_commands.sql), `link_commands_terminal_pair`, `link_commands_pending` and `link_commands_expirable`. | Startup loads numbered files by glob. The command constraint names and predicates are explicit, so migration 3 can append safely. |
| [Machines boundary](../../../src/server/internal/machines/link.go), `Presence`, `Link` and `CommandQueue`, [projection](../../../src/server/internal/machines/project.go#L19), `ProjectWithPresence`, and [watch](../../../src/server/internal/machines/watch.go#L28), `Service.WatchClients`. | The snapshot and saved changes already read through Link. The current projection gates connection details on `Held`, independently of the proposed flag. |
| [Machines registry](../../../src/server/internal/machines/store.go#L227), `Registry.Lock`, `SetApproved`, `SetName` and `Touch`, [action routes](../../../src/server/internal/machines/commands.go), `queueConnectedAction`, and [record routes](../../../src/server/internal/machines/records.go), `ApproveClient`, `RenameClient`, `RemoveClient` and `RevokeClientToken`. | The machine is the first domain row involved when a command is added. Remove and waiting token revocation already call `Forget` in the deleting write. |
| [Store receipts](../../../src/server/internal/store/receipts.go#L44), `Store.WriteOnce`, and [store writes](../../../src/server/internal/store/feed.go#L23), `MaxWriteChanges`, `Tx.Record` and `Store.Write`. | The request-key advisory lock comes before the fresh callback. A receipt replay never calls enqueue. Changes are appended under one feed lock at the end of the write. There is a capacity of 256 recorded references per write. |
| [Link transport limits](../../../src/server/internal/api/server.go#L32), `linkReadLimit` and [transport tests](../../../src/server/internal/transport/link_stream_test.go). | The existing limit is 65,536 bytes and the generated handler also bounds decompressed input. Tests of framing already use both stream protocols and a compressed decoded-size case. |
| [Scripted peer](../../../src/server/internal/link/machinetest/machine.go), `Machine.handshake`, `Machine.Connect` and `wireProof`. | The peer hardcodes revision 1 and independently checks the signed challenge. The plan updates the accepted revision and gives a scripted process one stable run pair. |
| [Application startup](../../../src/server/internal/app/start.go#L96), `LinkSeams` and `startup.run`, and [server fixture](../../../src/server/servertest/server.go#L32), `Options`, `Start` and `Server.launch`. | The application migrates before listening. Both service paths already use one State. The fixture reuses its options across a stop and restart. |

## New files and their closest existing patterns

| Target under `src/server/` | Neighbor and symbol |
|---|---|
| `internal/link/parts.go` and `parts_test.go`. | `internal/link/outbox.go`, `State.Enqueue`, and `internal/link/outbox_test.go`, `TestEnqueueStoresBeforeSend`. |
| `internal/link/migrations/0003_link_revision_two.sql` and `internal/link/migration_v2_test.go`. | `internal/link/migrations/0002_link_commands.sql`, `link_commands`, and `internal/link/outbox_test.go`, `TestCommandMigrationUpgradesPartB`. |
| `internal/link/admission.go` and `admission_v2_test.go`. | `internal/link/presence.go`, `State.BecomeCurrent`, and `internal/link/handshake_test.go`, `TestHandshakeDecisionOrder` and `TestAdmissionCommitReplyLost`. |
| `internal/link/recovery_v2_test.go`. | `internal/link/handshake_test.go`, `TestAdmissionCommitReplyLost` and its commit-reply wrapper. `internal/machines/review_rollback_watch_test.go`, `TestWatchKeepsJoiningThroughReplacementRollback`. |
| `internal/link/run_gate_test.go`. | `internal/link/outbox_send_test.go`, `TestCommandSendPoolHeadroom`, and `internal/link/stream_test.go`, `TestEventsAckedAfterCommit`. |
| `internal/link/settlement_v2_test.go`. | `internal/link/outbox_result_test.go`, `TestAcceptResultRules`, and `internal/link/outbox_sweep_test.go`, `TestSweepReleasesMachineBetweenBatches` and `TestCommandRemovalIsAtomic`. |
| `internal/link/folders_v2_test.go`. | `internal/link/inbox_test.go`, `TestEffectAndMarkCommitTogether`, and `internal/link/stream_test.go`, `TestEventsAckedAfterCommit` and `TestRemovalRefusesNextFrame`. |
| `servertest/link_v2_test.go`. | `servertest/link_stream_test.go`, `TestLinkSeamsReachTheService`, and `internal/app/review_commands_test.go`, `TestServerRestartsAfterInterruptedCommandMigration`. |

## Frozen test survey

Searched `helloFrame`, direct `Hello` constructions, `GetLinkRevision`, `BecomeCurrent`, `Enqueue`, migration version assertions, the scripted `Connect` and the application-level seam. The matching files and tests appear in the plan's allowlist. Shared helper users keep behavior assertions. The current manifest assertions are in `TestBecomeCurrent`, `TestServerCommandSweepLifecycle` through `seedStoppedCommandSweep`, and `TestServerRestartsAfterInterruptedCommandMigration`. `store.Migrate` has one version row per part, so those checks must expect 3.

The raw command fixture's `openCommandPeer` is used for both the send deadline and shared permit tests. The direct admissions in `TestEventFormsOneLine`, `TestWritesOfOneMachineFormOneLine`, `TestResultAndReplacementFormOneLine` and `TestCommandSendRechecksBeforeWrite` require the new pair. `TestScanAndReconnectQueueCommands`, `TestProjectStreamPresence` and `TestWatchClientsStreamPresence` use the Machines fixture or a direct replacement of it.

`TestApplyNextEvent` and `TestEventFormsOneLine` compare `EventResult` values. The planned writer derives readiness without adding a field to that result. `TestStreamStartClearsStalePresence` seeds an obsolete connection after the original runtime admission and must reuse the same pair through its application helper. The transport-only synthetic Welcome in `internal/transport/link_stream_test.go` and `internal/transport/review_welcome_test.go` tests framing and keeps its old literal.

## Planner probes

| Probe | Result on 2026-11-27 |
|---|---|
| Gate 1 and writing | Compared all decisions with the assignment and owner approval. Closed rows cite their sources. Unanswered rows give the assumption and rationale. The machine-lock decision lists fresh writers and replay exclusions. Read the new files for the repository writing rules. |
| Gates 2, 3, 5 and 6 | Read the owner and technical sections against each other. Checked eight observed done criteria, 17 first-written tests, risks and environment checks in four workstreams, and one rollout sequence. The plan changes no screen or visual hierarchy. |
| Gate 4 | Compared the named source universe with the mapping, including both complete round 3 closing lists and all sprint 4 checks. Checked that each mapping points to a workstream. |
| Gate 7 | The attack fold is ready for the orchestrator. The common planner assignment delegates the review beyond this handoff. |
| Gate 8 | Checked every task anchor against this record, spot-checked live source symbols, and checked relative source links and neighboring patterns for new files. |
| Gates 9 and 10 | Checked the applicable repository rules, file scope, work area, complexity and dependency for each item. The scope check for `server/backend` passed. No current vendor allowance was present in the repository or dated ledger rows. The sizing fold gives the same-vendor launch count and the dispatch calculation. |
| Canonical format and epic | Checked frontmatter, section order, folds, date and the existing epic slot. The task allows only this plan folder and its result to change, so the orchestrator handles the reverse link and other roadmap work. |
| Privacy command | The required Node version ran `scripts/check-private.mjs` and exited 0. Ran the same scanner's rules against the untracked plan files using an explicit file list, also with exit 0. |

The commands compared names, paths, status, test identifiers and the written requirements. Runtime and deployment checks are build acceptance.
