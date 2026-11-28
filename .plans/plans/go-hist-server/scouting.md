# Evidence for the server history plan

The planner read the repository at commit `181e4f44` on 2026-11-28. The working tree and the staged diff had no tracked changes. The scope check returned `ok: server/backend`.

## The assignment and the two designs

| Source | What it establishes |
|---|---|
| `.foundry/go-rewrite/tasks/plan-go-hist-server.md` and `.foundry/go-rewrite/common-planner.md`. | The assignment owns four planning artifacts, and the implementation owns the Server folder. The design and brief replace a planning interview. Review rounds belong to the orchestrator. |
| `docs/specs/2026-11-28-server-authority-on-history-design.md`, read in full, and `.foundry/go-rewrite/names-history.md`. | The new comparison, reset, and notification rules supersede sprint 4. The build order and Protocol Buffers names are fixed for independent planning. |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, read in full. | The Link uses authenticated machine identity, stores the mark and command ledger in PostgreSQL and participates in the ordered feed. |
| `.plans/epics/go-rewrite/epic.md`, its team table and linked Server plans, and `floorplan/plan.json`, its routes and files. | The Server team owns `src/server/`. The history plan awaits its epic registration. The planned floorplan names the product boundaries. |
| `docs/specs/2026-11-05-build-loop-design.md`, phase 4. | A test writer supplies failing assertions and freezes them before the builder changes code. Risky items need failing mutation runs. |
| `docs/issues.md`, 1632–1635, 1701, 1702 point 1 and 1714. | The feed must follow commit order. The paired fixture belongs outside the programs. The amended design gives the restored-history and frame-limit decisions and records the mark's security scope. |
| The world-class planning skill, its plan template, self-check probes, and model sheet lines 86–115. | The canonical tier M plan, scope, sizing and review handoff. |
| `.plans/plans/go-s4-server-link/plan.md` and its `decisions.md`. The findings in `attack/round-1-findings.md` and post-build reviews 2, 3, 4, 5c, 6, 7c and 8. | Keep an independently proven command gate, machine-scoped settlement, staged-result ordering and an authenticated replacement fence. The warning proof is superseded. The review of the send seam keeps its error mapping. |
| `.plans/plans/go-s4-server-projects/plan.md` and its `decisions.md`. The findings in `attack/post-build-p3.md`, `post-build-p4.md` and `post-build-8b.md`. | A folder call can wait independently of its command, and operation settlement belongs in the same write. The paired review requires an actual staged command, valid replay ordering and evidence of the waiting call. |
| `.foundry/go-rewrite/results/scout-history-rule.md`, its report at `459fe15e`. | The scout guided the local reads. A path and line-number walk checked 184 anchors in 71 files against this tree. The only cited file changed since that commit was `docs/issues.md`. Its three later additions include the new history design, the wording decision on the frame limit, and issue 1714. |

## Admission, commands and settlement in the current tree

| Current anchors | What the code shows by reading |
|---|---|
| `src/server/internal/link/handshake.go` · `Service.handshake`, lines 149 and 188–263. | The server verifies Proof before admitting the stream. Hub admission handles definite and ambiguous writes. Welcome uses the admitted mark. |
| `src/server/internal/link/admission.go` · `RunAnnouncement`, `classifyRun`, `runReady`, `State.promoteRun`, `errRunBehindMark`, `recoveryDiagnosticError`. | The run pair is immutable and only the latest known run may reconnect. The current unseen-run fence uses the committed mark, highest stage and pending boundary. Promotion measures first minus one and currently clears the warning as a separate condition. |
| `src/server/internal/link/presence.go` · `State.BecomeCurrent`, `NewState`, `State.Presence`, `State.Forget`. | One Store write locks the registry row, records the admission and calls the part hooks. The warning path uses a diagnostic-only commit, and Presence reads the warning generation. Forget settles pending work and deletes Link's machine and connection records. |
| `src/server/internal/link/migrations/0001_link.sql` · `link_machines.event_mark`. `src/server/internal/link/migrations/0002_link_commands.sql` · `link_commands_outcome`, `link_commands_result_dispatched`, `link_commands_state_outcome`. `src/server/internal/link/migrations/0003_link_revision_two.sql` · `link_runs`, `recovery_set_generation`, `link_commands_stage`. | The mark is a nonnegative PostgreSQL bigint. The command record distinguishes pending, staged and terminal rows. Revision 2 adds the run pair, the warning and staging. |
| `src/server/internal/link/outbox.go` · `State.EnqueueCommand`, `CommandGrace`, `CommandRetention`. `src/server/internal/link/outbox_claim.go` · `State.claimNext`, `assignCommand`. | Commands are stored in a caller-owned locked transaction and ordered by sequence. A claim updates stream and attempt, and overwrites `dispatched` with false before transport. |
| `src/server/internal/link/outbox_send.go` · `State.SendNext`, `State.sendClaimed`, lines 89–183. `src/server/internal/link/stream.go` · `Service.serve`, lines 94–119. | A command uses one writer turn. The after-claim seam runs before the locked send recheck. The existing send calls transport before setting `dispatched`. An error from the database can follow delivery. |
| `src/server/internal/link/outbox_result.go` · `State.AcceptResult`, `State.lockCommandStream`, `readCommandsToSettle`, `State.finishCommand`. `src/server/internal/link/parts.go` · `SettlementOutcome`, `Settlement`, `PartHooks`. | Matching dispatched attempts may settle. The common terminal write clears payload and stage and calls registered parts with the transaction. Part hooks can abort the write. |
| `src/server/internal/link/inbox.go` · `State.applyEvent`, `State.settleCoveredResults`. `src/contracts/proto/lgtm/link/v1/link.proto`, lines 15–23 and 128–147. `src/contracts/proto/lgtm/link/v1/events.proto`, lines 17–44. | Ordinary replay uses the committed mark and applies in sequence. The existing signed storage bounds fit both run thresholds and command watermarks. Covered results settle before readiness. |
| `src/server/internal/link/outbox_sweep.go` · `expiredCommandPage`, `finishedCommandPage`, `State.sweepCommandBatch`, `State.RunCommandSweep`, `WithCommandSweepTicks`. `src/server/internal/link/run.go` · `Service.Run`. | Timeout is for uncompleted, unstaged ordinary commands. The terminal predicate already includes a newly legal outcome. The maintenance worker uses bounded batches and its owner joins it at shutdown. |
| `src/server/internal/link/machinetest/machine.go` · `Machine`, `Machine.handshake`. `src/server/internal/link/admission_v2_test.go` · `submitAdmission`. | Both scripted server peers need a way to supply the new numbers. Scripted fixtures can represent explicit history while keeping the same signing and transport. |

## Folder operations and the machine watch in the current tree

| Current anchors | What the code shows by reading |
|---|---|
| `src/server/internal/app/projects_hooks.go` · `registerProjectHooks`. `src/server/internal/projects/reports.go` · `Service.ApplyFolderSeen`, `Service.SettleCommand`. | The report can finish a folder operation independently of a result. The Link hook sends the string outcome to Projects. Its settlement method affects only an open operation. |
| `src/server/internal/projects/operations.go` · `startFolderOperation`, `readFolderResult`, `Service.waitFolderOperation`. `src/server/internal/projects/errors.go` · `storedFolderFailure`. | The request key points to the durable operation. The waiter polls and returns its fixed error or reply. The existing `no_answer` maps to the chosen caller-visible error. |
| `src/server/internal/app/projects_test.go` · `TestProjectStartupRepositoryWatch`, `testProjectAppLargeRemoval`, `projectApp.online`. | The frozen large removal test stages a real folder command and adds a warning through an old admission refusal. The test can preserve the staged rollback and removal with the latter setup removed. |
| `src/server/internal/machines/link.go` · `Presence`, `Link`. `src/server/internal/machines/project.go` · `ProjectWithPresence`. | The projection copies the warning outside the held-connection branch. The watch can carry a list to an offline machine record through the same projection. |
| `src/server/internal/machines/watch.go` · `Service.WatchClients`, `WatchingPresence`. `src/server/internal/machines/service.go` · `Service.loadPresence`, `Service.projectOne`. `src/server/internal/watch/watch.go` · `Serve`, `readChange`. | Snapshots and saved updates use the same presence projection inside the read transaction. A feed row carries a key and reloads the current record. |
| `src/server/internal/machines/commands.go` · `Service.queueConnectedAction`. `src/server/internal/machines/store.go` · `Registry.Delete`, `Registry.TryLock`, `Registry.Touch`. `src/server/internal/store/feed.go` · `Tx.RecordOnce`. `src/server/internal/store/pool.go` · `Store.Read`. | Commands are enqueued under the machine lock. Touch records a coalesced saved reference. Delete publishes a removed reference. The read transaction uses repeatable read. |
| `src/server/internal/link/recovery_v2_test.go` · `newRecoveryMachinesClient`, `recoverySnapshot`, `recoveryWatchAt`. `src/server/internal/machines/watch_shared_follower_test.go` · `TestWatchClientsSharedFollower`. | Tests can open signed-in live watches through an existing fixture. Some warning-free fixture methods also serve the waiting replay test. |

## New file anchors

| Target | Existing pattern |
|---|---|
| `src/server/internal/link/history_handoff_test.go`. | `src/server/internal/link/outbox_send_test.go` · `TestCommandSendRechecksBeforeWrite`, transport flush gate and commit tracer. |
| `src/server/internal/link/history_admission.go`. | `src/server/internal/link/inbox.go` · `State.settleCoveredResults`, and `src/server/internal/link/outbox_result.go` · `State.finishCommand`. |
| `src/server/internal/link/history_admission_test.go`. | `src/server/internal/link/admission_v2_test.go` · `TestRunAdmissionPersistsAcrossRestart`, and `src/server/internal/link/settlement_joined_v2_test.go` · `TestResultAndRunAdmissionUseOneMachineLock`. |
| `src/server/internal/app/history_projects_test.go`. | `src/server/internal/app/projects_test.go` · `TestProjectStartupAndFixedRefusals` and `testProjectAppLargeRemoval`. |
| `src/server/internal/link/failed_requests.go`. | `src/server/internal/link/presence.go` · `State.Presence`, and `src/server/internal/link/outbox_sweep.go` · `State.sweepCommandBatch`, `State.RunCommandSweep`. |
| `src/server/internal/link/failed_requests_test.go`. | `src/server/internal/link/recovery_v2_test.go` · `newRecoveryMachinesClient`, and `src/server/internal/link/outbox_sweep_test.go` · `TestCommandSweepStops`. |

## Frozen tests and the standalone build boundary

The inventory below was checked against the existing `frozen-*.sha256` lists in `.foundry/go-rewrite/`. These are the affected file freezes. The plan names the tests and reasons.

| Test file under `src/server/` | Existing freeze suffixes |
|---|---|
| `internal/link/admission_v2_test.go`. | `s4-link-3`, `s4-link-7`. |
| `internal/link/recovery_v2_test.go`. | `s4-link-3`, `s4-link-4`, `s4-link-7`. |
| `internal/link/migration_v2_test.go`. | `s4-link-2`, `s4-link-3`, `s4-link-7`. |
| `internal/link/settlement_joined_v2_test.go`. | `s4-link-3`, `s4-link-6`, `s4-link-7`. |
| `internal/link/run_gate_test.go`. | `s4-link-3`, `s4-link-5`, `s4-link-7`. |
| `internal/app/projects_test.go`. | `s4-projects-8`. |

The paired tests are outside Server ownership. `src/linktest/sprint_four_history_test.go` · `TestSprintFourHistoryRecovery` and `src/linktest/sprint_four_run_test.go` · `TestSprintFourPendingRun` are frozen in `s4-paired-1`. They use the old warning column. The paired helpers `pairedScript.attempt` and the scripted Hello in `src/linktest/outbox_test.go` send only the original first number. Those files, `relay_test.go` and `relay_controls_test.go` are a handoff for updating paired probes and controls. The old real client also sends the earlier Hello fields.

`scripts/check-go.sh` runs formatting, module, diagnostics, Go tests and the frozen-link check. `deploy/test/with-postgres.sh` sets `LGTM_TEST_POSTGRES`. `src/server/internal/store/storetest/postgres.go` and `src/linktest/fixture_test.go` skip PostgreSQL tests when that setting is absent. The standalone gate and separate PostgreSQL server suite therefore check the Server slice without exercising the old mixed protocol. The final PostgreSQL gate runs after the paired changes.

## Planner self-check evidence

The planner ran repository status and whitespace checks, verified the scout links and scoped source anchors with `rg` and `git diff`, inspected affected freeze entries, and ran the scope checker on `server/backend`. The shape probe checked four work items, 13 decisions, 11 acceptance criteria, 14 tasks and 60 task symbols. The source read and shape probes concern the plan deliverable. The build commands in the plan are acceptance checks for future code. The private-data check passed on tracked files and its rules were also applied directly to the planning artifacts.

| Probe | Evidence |
|---|---|
| 1. Decisions. | The decision log has closed rows and unanswered rows with assumptions. Scope decisions cite the approved history design and bounded task. The settlement scope names the included and excluded command classes. |
| 2. Acceptance. | Eleven criteria point to exact tests. Each workstream names the behavior and the observation that gates it. |
| 3. Design artifact. | The server work changes no screen hierarchy. The history document supplies the server behavior and the Website plan owns the bell. |
| 4. Traceability. | The named source universe maps its six deliveries, twelve history checks, foundation decisions, rules, checks and issue to workstreams or their respective plan owners. |
| 5. Risks. | The four workstreams have risk, severity and mitigation cells. The concurrency-sensitive tests have mutation expectations. |
| 6. Verification. | Each block names its tests and PostgreSQL environment. There is one rollout order, with standalone and final commands below it. |
| 7. Adversarial review. | The planner checked the review handoff row. Findings are pending the orchestrator's review round under the bounded assignment. |
| 8. Anchors. | Files and symbols in the task rows were checked against this tree. New targets have existing pattern anchors above. |
| 9. Guardrails. | The guardrails table refers to the current repository rule headings and habits. |
| 10. Sizing. | Four units have one accepted work area, scope, complexity and reason. The one-vendor load is counted. The dependency column and file lists show the only parallel pair. |
| Format and linkage. | The frontmatter and headings follow the tier M template and the overview precedes the implementation divider. The scope check passed. The epic exists and still awaits the orchestrator's history-plan row. |
