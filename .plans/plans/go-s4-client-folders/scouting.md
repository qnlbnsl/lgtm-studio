# Source and verification for the sprint 4 client plan

## Sources read on 2026-11-27

| Source | Evidence for the plan |
|---|---|
| `AGENTS.md` and `CLAUDE.md`. | The current repository boundaries put new client work in Go. The instructions cover source credit, issue records and document privacy. |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, Decisions, The link between the server and a machine, Rules, and How wave 1 is proven done. | The link is outbound and authenticated. It has stored commands, receipts, numbered events and cumulative acknowledgement. The paired fixture must run both real programs with PostgreSQL. |
| `docs/specs/2026-11-27-running-an-agent-design.md`, Common rules, Sprint 4 and Sprint 5. | The run barrier and result watermark supplement the old queue. Sprint 4 adds inspection and outgoing filtering. Sprint 5 reuses the restart and folder primitives and adds producing-run reports and agents. |
| `.plans/epics/go-rewrite/epic.md`, The teams and row 15 of Plans. | The Client team owns the client tree. The orchestrator owns the change of the sprint 4 row from planned to linked. |
| `docs/specs/2026-11-05-build-loop-design.md`, Phase 4. | Tests fail on assertions before code, become frozen, and fail again under named risky mutations. |
| `.plans/plans/go-s4-contract/plan.md`, Run gate and result fence, Buffer loss, Folder facts and byte limits, and Post-build checks. | A client can replay the retained prefix before approval. The reviewed stop, origin rules, numbered byte tests and five combined cases set the boundaries. |
| `.foundry/go-rewrite/results/fold-go-s4-contract-r2.md`, Requests to the dependent plans and Names the other plans of sprint 4 build against. | This fixes the wire spelling and the Client team proof, including the executable log and real interruption. |
| `.plans/designs/running-an-agent/round-3-logic.md` and `round-3-fit.md`, For the plans. | Logic supplies the combined send-permit case. Fit supplies the byte boundaries. Other closing lines have their individual dispositions in the decision log. |
| `.plans/plans/go-s3-client-link/plan.md`, work items 3 to 9 and the result. Its review register `attack/round-4-findings.md` and post-build rounds E through G. | The proof order and planned final revision refusal are existing commitments. The post-build fixes protect the acknowledgement ceiling, transport retirement, exact window and restart result delivery. |
| `docs/issues.md`, issues 1632 to 1635, 1670 and 1676. | The paired test packages and sizing keep the sprint 3 approach. The client proof and the public certificate key are separate. The macOS keychain and indistinguishable loss cases remain recorded. |
| `floorplan/plan.json` and `docs/specs/2026-11-27-running-an-agent-design.md`, The parts. | The newest design is the anchor for Folders. The floorplan is an older survey of the three-part structure. |
| The world-class-plan skill, its template and Tier M self-check, and the work-area and complexity rows in the model sheet. | The assignment adapts the review and scope probes because it stops before review and the orchestrator integrates planning files. |

## Existing code anchors by reading

| File | Symbol or section and observation |
|---|---|
| `src/client/internal/link/conn.go` | `linkRevision` is 1 at the baseline. `openSession` sends the three handshake messages. `validateWelcome` checks structure before returning a revision mismatch. `proofText` uses the existing labels. `session.receiveError` reads trusted refusal reasons from trailing metadata. |
| `src/client/internal/link/retry.go` | `supervise` stops on a verified wire refusal. An incompatible Welcome currently returns from its attempt without the wire-only stop flag. |
| `src/client/internal/link/session.go` | `Run` opens the state store and sets up the process owner. `clientRun.attempt` opens and retires the transport. `liveSession.serve` keeps independent send and receive workers, the 32-event window and the pre-handoff acknowledgement ceiling. `stopForStore` emits fixed diagnostics but maps damage and missing files differently from impossible marks. |
| `src/client/internal/link/store.go` | `inspectBuckets` checks numeric order of the suffix. `Store.update` and `Store.updateContext` fence further writes after uncertain commit. `OpenStore` does not recreate a missing database. |
| `src/client/internal/link/outbox.go` | `Store.appendContext` writes the event and `next` together under the store lock. `Read`, `Ack` and `Next` expose the retained range and cumulative prefix. Acknowledgement does not reset next. |
| `src/client/internal/link/receipts.go` | `readReceipt`, `findReceipt` and `writeFinishedReceipt` show the version 1 envelope, existing interrupted default and prune boundary. `Begin`, `Decline` and `Finish` use one store lock. |
| `src/client/internal/link/commands.go` | `commandHandler.handle` has a separate standing path. `startScan`, `settleScan` and `finishCommand` show the worker and load-bearing receipt paths. `restart` joins the scan and records the result within the current send budget. |
| `src/client/internal/link/replay.go` | `newReplay` applies the Welcome mark. `replay.next` decodes an immutable payload and assigns its number on send. `encodeReport`, `decodeReport` and `isUnnumberedReport` pin the existing versioned payload and known arms. |
| `src/client/internal/link/report.go` | `reporter.publish`, `captureFacts` and `scanHarnesses` produce two coalesced snapshot kinds. `reportString` enforces UTF-8 and byte lengths. `reportWorkspace` abbreviates a workspace relative to home. |
| `src/client/internal/app/client.go` | `Run` verifies saved enrollment, loads key and settings, and logs structured state and workspace paths before calling Link. `withDefaults` creates a structured logger and `Join` uses fixed enrollment diagnostics. |
| `src/client/internal/enroll/credential.go` | `Load`, `decodeCredential` and `Credential.Format` show the protected regular-file key load, the private-key encoding and the public-only formatter. |
| `src/client/internal/state/folder.go` | `Folder.Claim`, `prepareParent` and `checkParent` explain single-owner state and the environment prerequisite of native tests. |
| `src/client/internal/config/config.go` and `src/client/internal/harness/scan.go`. | `LoadSettings` consumes an explicit home and repository setting. `Scan` and `readVersion` use bounded process probes. Extend the pattern inside Folders with stricter checkout isolation. |
| `src/client/cmd/lgtm-client/main.go` and `src/client/clienttest/clienttest.go`. | `runContext` and `main` propagate the app status. `clienttest.Options` and `Run` delegate to the same app with disposable state. |
| `src/linktest/outbox_test.go` | `authenticateScriptedLink` uses a manual revision 1 Hello. The combined fixture owner can update it for current-server tests and keep a separate refusal case. |
| `src/client/internal/link/conn_test.go`, `retry_test.go`, `session_test.go`, `receipts_test.go`, `commands_test.go`, and `replay_test.go`. | These provide the proof, retry, waiting, store, command and replay fixture anchors named in the frozen change register. |
| `src/client/internal/link/review_f_ack_test.go`, `review_f_restart_test.go`, `review_g_window_test.go` and `store_crash_test.go`. | They supply immediate acknowledgement, restart half-close, exact window and actual process death patterns. |
| `src/client/internal/app/client_test.go`, `src/client/cmd/lgtm-client/link_test.go` and `src/client/clienttest/clienttest_test.go`. | `TestAppRunStoreStatus`, `TestRunCommandExitStatus` and `TestClienttestSharesApp` cover process status and delegation. Their mock peers have explicit revision 1 constants. |
| `deploy/lgtm-client.service` and `deploy/dev.lgtmstudio.client.plist`. | The Linux service uses `Restart=on-failure`. The macOS service has `KeepAlive.SuccessfulExit=false`. Both have a five-second restart setting. Stack proves the actual behavior and owns the configuration. |

## New build paths and their starting patterns

| Target path | Existing pattern |
|---|---|
| `src/client/internal/link/{run_identity,filter,folder_command}.go`. | Use `conn.go` for proof inputs, `report.go` for report text and `commands.go` for asynchronous command work. |
| `src/client/internal/folders/{inspect,origin,status}.go`. | Use the bounded process pattern at `src/client/internal/harness/scan.go` and `config.LoadSettings` for explicit home input. |
| `src/client/internal/app/outgoing.go`. | Use startup and logger creation at `app.Run` and `withDefaults`. Inspect key formats from `enroll.Load`. |
| `src/client/internal/link/{revision_two_session_test,revision_two_receipts_test,revision_two_recovery_test,folder_command_test,filter_test}.go`. | Use the signed peer, store crash and fixture seams in `session_test.go`, `store_crash_test.go`, `commands_test.go` and `replay_test.go`. |
| `src/client/internal/link/testdata/client-v1.db`. | Produce a disposable enrolled state with the sprint 3 executable. The close, copy and reopen pattern is in `store_crash_test.go`. |
| `src/client/internal/folders/{inspect_test,status_test}.go`. | Use temporary directories and bounded subprocess probes as in `src/client/internal/harness/scan_test.go`. |
| `src/client/internal/app/{revision_two_process_test,outgoing_test}.go`. | Use signed-peer and disposable state fixtures in `app/client_test.go` and the executable subprocess approach in `cmd/lgtm-client/link_test.go`. |

## Read-only probes confirmed live

- `git show 44c931d9:src/client/internal/link/receipts.go` confirmed that this sprint 3 baseline writes version 1 started and finished receipts. The recorded baseline uses link revision 1 and the existing unnumbered payload form.
- The client scope passed `scope.py check . client/backend`. The scope grants `src/client/**`. The task narrows the production paths and the orchestrator owns its integration.
- The installed Connect source exposes both `WithReadMaxBytes` and `WithSendMaxBytes`. The current client sets the first in `newRunLinkClient`.
- The baseline signed proof fixture passed. The baseline exact-window and receipt process-death probes stopped at the environment's state-directory precondition. The filesystem root has a different owner from both root and the test user. The precondition is in `state.checkParent`. The window and receipt assertions were not reached in this environment. Those tests remain build gates on supported native hosts.
- The planned revision 2 tests and both supervisor runs are build evidence. This assignment writes the plan.

## Original Tier M self-check

| Gate | Result |
|---|---|
| 1. Decisions | Every decision has a date, rationale and either a closed status or an explicit assumption. Scope cites the dated design, assignment or written contract. |
| 2. Acceptance | The eight items and workstream rows name assertions, commands or observed service steps. All items say where the native proof runs. |
| 3. Design artifacts | No screen or hierarchy changes are in this client plan. |
| 4. Traceability | The mapping covers each named source category and uses only the three workstreams. The exact individual closing-list dispositions appear in the decision log. |
| 5. Risks | Each workstream has a risk and observable mitigation. Issue 1670 remains open. |
| 6. Verification | Each workstream has gating tests and a device or environment condition. The plan has one rollout sequence. |
| 7. Review | Pending the orchestrator's review, as required at this handoff. The attack log carries that state. |
| 8. Anchors | Existing paths and symbols were read and indexed above. Each new path has an existing pattern, and tests from the old release are identified by name. |
| 9. Guardrails | The guardrails table cites applicable repository rules and the task boundary. |
| 10. Sizing | Eight medium backend rows use the passing client scope. The launch count states the one-vendor budget. Live allowance is checked at dispatch by the orchestrator. |

The format probe covers frontmatter, template section order, the overview divider, five folds, owner-facing scope, and relative paths. The epic row is present as planned. The bounded assignment reserves its linkage edit to the orchestrator.

## Evidence for the round 1 fold

| Source | Observation |
|---|---|
| The four reports in `attack/` and the original plan and result. | By reading, the reports contain 36 numbered findings. The frozen `plan-v1.md` matches the plan at `22ca76c6` byte for byte. |
| `src/client/internal/link/store.go` at `inspectBuckets` and `updateContext`, and `outbox.go` at `Store.appendContext`. | By reading, inspection accepts an eight-byte nonzero `next`. Append already exhausts after the last signed 64-bit number. The store lock orders writes and the existing cancellation check is inside commit. The `meta` bucket's `next` key is big-endian. |
| `src/client/internal/link/conn.go` at `openSession` and `validateWelcome`, `commands.go` at `serverClock.upperBound`, `commandHandler.setSession`, `handle` and `reply`, and `receipts.go` at `writeFinishedReceipt`. | By reading, the authenticated Welcome supplies server time. Command admission already computes an upper bound from the client's proof send. Existing receipt pruning uses local wall time. The handler sends standing results before the ordinary dispatch path. |
| `src/client/internal/link/session.go` at `Run`, `clientRun.attempt`, `clientRun.queueResult` and `liveSession.serve`. | By reading, worker ownership outlives a stream and current cleanup can settle a worker. Result handoff and stream fencing need to cover that lifecycle. |
| `src/client/internal/link/report.go` at `reporter.publish`, `flush`, `appendSnapshot`, `reportString` and `reportWorkspace`. | By reading, flush passes the previously encoded candidate to the same append path. The existing workspace transformation removes zero bytes after abbreviation. |
| `src/client/internal/link/commands_test.go` at `TestCommandBusySlotWritesNoReceipt` and `TestDeclineClockOffsets`, and `receipts_test.go` at `TestFinishPrunesByAge`. | By reading, the frozen write counts and fast-clock expectations need the planned revisions. The old expiry fixture preserves equality at the margin. |
| `src/linktest/link_test.go` at `TestLinkJoinReplay`, and `command_fixture_test.go` at `savedCommandReceipt`, `requireReceipt` and `installVersionProbe`. | By reading, the first test starts both programs. The receipt helper requires version 1, and `commands_test.go` at `testCompletedScanReplay` re-encodes the receipt to age it. The probe provides an existing accepted version-line pattern. |
| `src/server/servertest/server.go` at `Options.LinkSeams`, Server Link's plan and result at the fixture boundary. | By reading, the paired fixture uses nested seams. The planned precommit callback takes authenticated machine and event number. The acknowledgement observer is separate. |
| `src/client/internal/app/client.go` at `Join` and `Run`, `src/client/cmd/lgtm-client/main.go` at `runContext`, and `src/client/internal/harness/registry.go` at `Definitions`. | By reading, the command supports direct and standard-input tokens. Early state errors can include private text. A simple three-part version reaches the existing production parser. |
| `deploy/test/checks-s4.sh:184`, `deploy/test/checks-s4.sh:201` and `deploy/test/README-s4.md:66`. | Confirmed live by comparing with `22ca76c6`. They use the direct token form of join, the run command and a private `LGTM_CLIENT_STATE_DIR` ending in `lgtm-client`. The frozen R11 proves old-client refusal. Stack's separate installer check covers the retained-state upgrade. |
| `deploy/lgtm-client.service` and `deploy/dev.lgtmstudio.client.plist`, and Stack's proposed check. | By reading, the two installed definitions restart on unsuccessful exit. Stack will prove the real supervisor transition. |
| `src/client/internal/folders/{checkout,origin,checkout_test}.go` and `{inspect,status,status_test}.go`. | These are new targets. The former group follows the bounded reads in `src/client/internal/harness/scan.go` (`Scan`) and explicit home in `config.LoadSettings`. The latter consumes the former's private snapshot and uses the existing disposable-directory test pattern. |
| `src/client/clienttest/{clienttest,clienttest_test}.go` and `src/client/internal/link/folder_command_test.go`. | The fixture builds on `clienttest.Join`, `Run`, `Options` and `TestClienttestSharesApp`. The new append observation follows the interruption pattern in `store_crash_test.go`. |

The Client scope passed the scope checker. A local version probe printed Git 2.47.3. Comparing `44c931d9` and `22ca76c6` found no changes under `src/client` or the generated Go Link contract paths. Git status behavior recorded by the reviewers is reproduced in the build plan's fixture requirements. The native Client, paired PostgreSQL and installed-service cases remain build gates.

## Round 1 Tier M self-check

| Gate | Observation |
|---|---|
| 1. Decisions | Dated rows 29 to 37 give assumptions or closed outcomes. The fold stays under the existing Client assignment and records the two alternate mechanisms and their reasons. |
| 2. Acceptance | Nine numbered items tie their gates to named tests or observable deployed results. The native commands require explicit passes and zero skips. |
| 3. Design artifacts | Client changes no screen or visual hierarchy. |
| 4. Traceability | The original source universe still maps to the three workstreams. The fixture is item 9 under the Folders workstream. |
| 5. Risks | All three workstreams retain risk, severity and observable mitigation. The macOS keychain issue stays recorded. |
| 6. Verification | Each workstream names its gating environment. There is one rollout order and one post-build check section. |
| 7. Review | The register covers all 36 numbered findings. Each blocker and major has a plan patch or a labeled alternate mechanism. |
| 8. Anchors | The read paths above supplement the original anchors. New paths have existing patterns. The paired fixture and checkout handoff specify both sides. |
| 9. Guardrails | The plan uses the existing Go and repository boundaries and exact Contract names. Only the assigned planning files and result are written. |
| 10. Sizing | Nine backend items have valid scope, complexity and a reason. The remaining launch allowance belongs to the orchestrator at dispatch. |

The plan keeps the template order, frontmatter, overview divider and five folds. The epic row is present at `planned`. Its linkage remains the orchestrator's edit under this assignment.

The repository privacy check, scoped privacy check and diff whitespace check passed. The structural probe verified nine item gates, six source anchors and all 36 reviewed dispositions. It recomputed 20 groups and 66 points. The plan is the only stuck statement.
