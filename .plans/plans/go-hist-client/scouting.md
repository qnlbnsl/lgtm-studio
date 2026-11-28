# Client history planning evidence

## Design and build inputs

| Source | Reading used in the plan |
|---|---|
| `AGENTS.md`, “What this project is”, “Habits” and “Writing style for documents”. | The work uses Go, respects the contract boundary and keeps coordination in the authorized files. |
| `docs/specs/2026-11-28-server-authority-on-history-design.md`, “How the two sides compare their numbers”, “What the client stops doing” and “How it is proven”. | The design supplies the two new numbers, the three Welcome cases, the history-file stop policy and twelve checks. |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, “The link between the server and a machine”, “Rules that every later part follows” and “How wave 1 is proven done”. | The underlying proof, durable numbers, receipt semantics, client retry and real-server test requirements remain the basis of the plan. |
| `docs/specs/2026-11-27-running-an-agent-design.md`, the command result and event-ordering rules at lines 90 to 97 and sprint 4 proofs R5, R6, R9, R10 and R11. | Results still carry their original ordering barrier. The current design supersedes the earlier history stops. |
| `.plans/epics/go-rewrite/epic.md`, “The teams, and what each may edit”, “Plans” and the sprint 4 proof section. | Client owns edits under `src/client/`. The built client already has the process pair and revision 1 coverage. |
| `docs/specs/2026-11-05-build-loop-design.md`, phase 4, especially “Build subitems”. | Named assertions fail before the build. The reviewed test files are frozen and risky ordering changes require mutation proof. |
| `docs/issues.md`, issues 1632 to 1635, 1701 and 1702 point 1. | Keep the current joining and testing seams. The new design owns history reconciliation. Contract owns the wire limit wording. |
| `.foundry/go-rewrite/tasks/plan-go-hist-client.md`, `.foundry/go-rewrite/common-planner.md`, and `.foundry/go-rewrite/names-history.md`. | These fix the bounded assignment, wire names, ownership and build order. |
| `.plans/plans/go-s4-client-folders/plan.md`, first three items and post-build checks, and `attack/round-1-findings.md` in that plan. | Keep the immutable pair, pre-dial checks, required suffix screening and receipt replay fences established in sprint 4. |
| `.plans/plans/go-s4-client-folders/attack/post-build-1.md`, `post-build-2.md`, `post-build-3c.md`, `post-build-8.md` and `post-build-11c.md`. | The established tests cover the waiting prefix, receipt validation, terminal fencing and the read-versus-damage distinction. The stream still joins the outage worker before replay so a late append cannot change the preflight range halfway through setup. |
| `floorplan/plan.json`, the Client Link and outbox entries, and the planning skill's template, self-check and model sheet. | The implementation stays in the built client. Each unit has one work area, one scope and a named observation. |

## Current code and test anchors

The source below was read at commit `181e4f44`. The supplied scout read `459fe15e`. A comparison confirmed 184 linked line anchors across 71 files. The only changed cited file was the issue record, and its cited lines still matched. The tables below use the current checkout.

| File and symbol | What the source establishes |
|---|---|
| `src/client/internal/link/store.go`, `eventSnapshot` at line 34, `initializeStore` at line 219 and `inspectBuckets` at line 371. | The disk format already encodes next and a continuous event suffix. The locked cache reconstructs both bounds from those records. |
| `src/client/internal/link/outbox.go`, `Store.appendAtNext` at line 68, `Store.Ack` at line 134 and `Store.Next` at line 167. | The store owns committed counters. Append and acknowledgement update them after commit. A regular acknowledgement refuses to advance beyond last. |
| `src/client/internal/link/store.go`, `Store.ready` at line 450, `Store.updateContext` at line 467 and `numberKey` at line 496. | A write error fences the store object. The next counter uses big-endian bytes. |
| `src/client/internal/link/run_identity.go`, `runIdentity` and `newRunIdentity` at lines 11 and 16. `src/client/internal/link/session.go`, `Run` at line 84 and `clientRun.attempt` at line 175. | Startup samples the boundary once. The attempt owns the separate per-stream handshake values. |
| `src/client/internal/link/conn.go`, `errInvalidWelcome` at line 30, `openSession` at line 86, `openSessionWithRevisionFence` at line 90 and `validateWelcome` at line 192. | The proof exchange verifies the saved server key. The Welcome is structurally checked after proof. Hello currently serializes the run pair. |
| `src/client/internal/link/session.go`, `clientRun.stopOutage` at line 310, `queueResult` at line 319, `linkFailureCode` at line 349 and `liveSession.serve` at line 436. | The outage worker can write after Hello and retires before replay. The stream establishes its handoff and acknowledgement ceilings from Welcome. It exposes the session and starts fresh reports only after replay and receipt preparation. |
| `src/client/internal/link/retry.go`, `supervise` at line 28. `src/client/internal/link/session.go`, `liveSession.receive` at line 774. | The supervisor can distinguish an unusable Welcome from an established stream. The sender keeps the live arrival-time ceiling, and the receiver checks the replay ceiling. |
| `src/client/internal/link/replay.go`, `newReplayWithFilter` at line 34, `screenReplay` at line 44, `replayRange` at line 79, `validateReplayBatch` at line 91, `replay.next` at line 113 and `replay.ack` at line 145. | The existing replay primitive screens the required suffix before acknowledging the prefix. It checks again at handoff. Its range errors are internal once the stream has classified Welcome. |
| `src/client/internal/link/receipts.go`, `prepareRun` at line 169, `findReceipt` at line 285 and `writeFinishedReceipt` at line 347. `src/client/internal/link/store.go`, `checkRunReceipts` at line 407. | Receipt validation precedes dialing. The preparation currently uses mutable next to seal inherited starts. Replay reads the saved result number back, and a new finish samples durable next minus one under the store lock. |
| `src/client/internal/link/commands.go`, `handle` at line 161 and `reply` at line 360. | Receipts prevent repeat work. A reply carries the existing outcome and stored number under the current attempt. |
| `src/client/internal/link/report.go`, `reportQueue` at line 43, `reporter.captureFacts` at line 103, `reporter.scanHarnesses` at line 143, `reporter.publish` at line 174 and `reporter.flush` at line 195. | The current-state producers already run on approval. Each report type can hold an unnumbered pending sample when the outbox is full. |
| `src/client/internal/app/client.go`, `Join` at line 40 and `Run` at line 83. `src/client/cmd/lgtm-client/main.go`, `run` at line 21. | App verifies saved identity and key before Link Run. The app maps recovery stop to zero and requested restart to 75. The executable supports a termination signal. |
| `src/client/internal/state/folder.go`, `Open` at line 64 and `checkParent` at line 167. `src/client/internal/link/store.go`, `CreateStore`, `OpenStore`, `openStore` and `createStoreFile` at lines 89, 94, 98 and 169. | The private folder and store claim protect missing-only creation. The open-only helper and the existing directory-sync operations are reusable. |
| `src/client/internal/link/store.go`, `openExistingBolt` at line 250, `classifyBoltMetadata` at line 305, `inspectStore` at line 344 and `storeOpenError` at line 421. `src/client/internal/link/session.go`, `stopLocal` at line 233, `recoveryStoreCode` at line 387 and `stopForStore` at line 398. | Read-only inspection is distinct from opening for writes. Read failures and corrupt metadata currently have different store errors. Genuine damage and the obsolete acknowledgement rule currently share the recovery stop mapping. |
| `src/client/internal/link/revision_two_session_test.go`, `TestRevisionTwoRunAnnouncement` and `TestRevisionTwoWaitingPrefix`. `src/client/internal/link/session_test.go`, `runPeerFixture` and `watchFixtureAcks`. | The signed test peer and acknowledgement observer support real Run and deterministic redials. |
| `src/client/internal/link/store_test.go`, `swapCommit`, `setNext` and `TestOpenStoreMissingAfterJoin`. `src/client/internal/link/outbox_test.go`, `TestAckRemovesUpToNumber` and `TestAckRefusesAhead`. | A test can force a transaction rollback or an uncertain success, reopen truth from disk, and preserve the existing open-only and acknowledgement behavior. |
| `src/client/internal/link/session_test.go`, `TestRetryFailureExitsAndKeepsStore` at line 473. `src/client/internal/link/replay_test.go`, `TestReplayRejectsImpossibleMarks` at line 118 and `TestReplayResumesAboveFreshWelcomeAfterGap` at line 183. `src/client/internal/link/review_f_ack_test.go`, `TestReviewFAckCeilingUsesReceipt` at line 19. | The old terminal impossible-mark assertions change. The existing ceiling and valid retry controls remain useful. |
| `src/client/internal/link/revision_two_recovery_test.go`, `TestRevisionTwoTerminalHistory` at line 27 and `testRecoveryWriteFence` at line 352. `src/client/internal/link/revision_two_review_test.go`, `TestRevisionTwoWaitingHoldsEventAtRunBoundary` at line 59. | The obsolete subtests test a missing file and the old upper and lower Welcome outcomes. The write-fence control currently uses the last legal mark as the invalid mark. The direct attempt fixture is suitable for a controlled late append. |
| `src/client/internal/link/receipts_test.go`, `TestStartedFromEarlierRunIsInterrupted` at line 92. `src/client/internal/link/revision_two_receipts_test.go`, `requireReceiptPreparation` at line 39 and `TestRevisionTwoReceiptMetadata` at line 311. `src/client/internal/link/revision_two_receipts_review_test.go`, `TestRevisionTwoReceiptRejectsDuplicateWatermark` at line 17 and `TestRevisionTwoReceiptRejectsResultOnStarted` at line 109. | These are the direct test call sites for receipt preparation. Their assertions continue to exercise malformed receipts, old formats and sealing rollback. |
| `src/client/internal/app/client_test.go`, `appPeer.challenge` at line 109, `appPeer.welcome` at line 132 and `TestAppRunStoreStatus` at line 677. `src/client/internal/app/revision_two_process_test.go`, `TestRevisionTwoExecutableStop` at line 28 and `runRecoveryProcess` at line 327. | The app still checks the outcome mapping. The executable and signed peer are available for forced loss of only the history file. |
| `src/client/internal/link/revision_two_store_open_review_test.go`, `TestRevisionTwoStoreOpenErrorClassification` at line 18 and `TestRevisionTwoStoreOpenPermissionFailure` at line 32. `src/client/internal/link/revision_two_store_read_review_test.go`, `TestRevisionTwoStoreMetadataRead` at line 36. | The portable permission cases distinguish read and write failure. The Linux-only metadata test controls a real read failure and currently expects executable status one. |
| `src/client/clienttest/clienttest.go`, `Options` at line 14 and `Run` at line 35. `src/linktest/sprint_four_history_test.go`, `TestSprintFourHistoryRecovery` at line 26 and `testPairedHistoryStop` at line 194. `src/linktest/fixture_test.go`, `requirePostgres` at line 27. | The paired owner can keep the existing startup entry point. Two old assertions are incompatible with the new independent client. They only run when the real database is supplied. |
| `scripts/check-go.sh`, `LGTM_LINK_BASE` and the `go test ./...` gate. `.foundry/go-rewrite/link-base.txt` and `.foundry/go-rewrite/frozen-*.sha256`. | The gate verifies the module and freeze. A separate inventory identifies the old hashes of each Client test file that will change. |

## New test files and their starting patterns

| Target | Existing pattern |
|---|---|
| `src/client/internal/link/history_hello_test.go` | `revision_two_session_test.go`, `TestRevisionTwoRunAnnouncement`, with `session_test.go` for a signed peer and an acknowledgement observer. |
| `src/client/internal/link/history_store_test.go` | `outbox_test.go`, `TestAckRemovesUpToNumber`, and `store_test.go`, `swapCommit`. |
| `src/client/internal/link/history_session_test.go` | `session_test.go`, `TestRetryResumesAfterEventGap`, and `revision_two_review_test.go`, `TestRevisionTwoWaitingHoldsEventAtRunBoundary`. |

## Read-only probes and review handoff

- The repository head and all named Client paths were checked. The scope check for `client/backend` returned `ok`.
- The private-detail check returned exit zero. The new files also received a separate content review because the script reads tracked files.
- The baseline build, `go build ./...`, returned exit zero with a writable Go build cache.
- The Go gate was attempted with the checked-in history base and the optional paired database unset. A writable Go build cache under the temporary directory got the run past the initial cache error. The baseline tests then failed because the sandbox represents the root directory as owned by neither root nor the process user. `checkParent` rejects the temporary state folder from that environment. Native Linux and macOS acceptance and the real paired run are future build evidence.
- The epic exists and has no `go-hist-client` row yet. This assignment reserves that edit for the orchestrator. The plan can enter review with its existing `epic` field.
- The local planning allowance is unavailable. Repository ledger entries describe prior runs. The orchestrator must use the current vendor allowance before launch.

| Gate | Self-check evidence |
|---|---|
| 1 | Every decision has a choice and rationale. Planner assumptions are marked unanswered. The dated scope includes a list of what adoption discards and what it preserves. |
| 2 | Each completion row names an executable test observation. New test specifications say which assertions expose the old behavior. |
| 3 | The four items touch the client process and its Go tests. The Website plan owns the interface artifact. |
| 4 | The source universe and each assignment point, design check and owning team appear in the traceability fold. |
| 5 | Each item records failure, severity and mitigation. Risky items have assertion-level mutation evidence specified. |
| 6 | Each item has a named native Linux and macOS command. The rollout has one order and separates the Client gate from the real database paired gate. |
| 7 | The bounded assignment ends at the start of review. The attack-log row is pending for the orchestrator. |
| 8 | The task anchors are present above. The store transaction, replay entry and executable entry were checked directly against source. |
| 9 | The guardrail fold names the repository rules in use. The plan and this record stay inside the assigned folder. |
| 10 | Four backend units have low or medium complexity and a passing scope. Launch counts are bounded to one vendor. The allowance check belongs to dispatch. |
