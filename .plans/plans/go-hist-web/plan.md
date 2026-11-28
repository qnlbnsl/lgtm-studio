---
topic: go-hist-web
status: attack
tier: M
created: 2026-11-28
updated: 2026-11-28
epic: go-rewrite
---

# Show a failed machine request and prove history recovery

## Overview

The Machines page returns to showing whether a machine is online, offline or pairing. A machine with an older copy of its history can come back online on its own. The old history status and explanation leave the page.

If that recovery interrupts a scan or restart someone requested, the bell shows that it did not finish. The person can choose the entry to ask for the same action again. The server removes the entry when the person asks again or when ten minutes pass.

The paired tests will run the real client and server. They will show that an older copy or a missing history file recovers, retained events still replay in order, and a command that might already have run stays failed. A lost reply during reconnection must still leave just one failure entry.

## Key decisions

The bell reads failed requests from the same stream that updates the Machines page. Keeping that stream as the source of truth means a retry, expiry, or machine removal updates an open page without a reload.

The server and client changes land before the paired proof. The unused contract field is reserved after every reader and writer has stopped using it.

Full log → [decisions.md](decisions.md).

## Workstreams

### Remove the history warning from Machines

An offline machine uses the usual offline text and details. A waiting machine uses the pairing flow. A browser test opens both kinds of records and checks that the history warning never appears.

### Put failed scans and restarts on the bell

The existing bell gets one action for each failed command, with its age on a second line. Each action names the machine. Browser tests cover the retry calls and the watch updates that remove the entries. The online Buildbox sample makes the scan entry available without a real server.

### Prove recovery with both programs

The tests replace or remove a stopped client's history file and hold particular messages while the programs reconnect. They read the saved event number before new reports can advance it. They also check which command identifiers cross the link, so a duplicate restart cannot hide behind a successful reconnect.

### Reserve the former warning field

The contract test checks the generated description of the machine record. Number 19 and its former name must remain reserved after regeneration.

## Rollout order

1. Build the Contract plan. It adds the new fields and preserves field 19 for the readers that still compile against it. Use its new frozen history base for this plan's checks.
2. Build the Server and Client plans side by side on that contract. Put both builds in the paired test writers' tree before they start.
3. Build items 1 and 2 in order. Item 2 uses the machine projection and action code from item 1. Review only the status removal and bell changes in the existing browser layouts.
4. Run the paired test work in items 3 and 4. Freeze the new assertions and rerun the existing replay, repository and revision 1 proofs with them.
5. Reserve field 19 and regenerate the contract in item 5. Run the full browser suite, the paired suite and the Go gate. The orchestrator updates the Stack references and epic linkage.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1: Remove the history warning from Machines**

Item 1 returns machine status and details to their ordinary branches. A waiting record continues to be available to the pairing dialog. The design artifact for both visible items is [What leaves and what arrives on the website](../../../docs/specs/2026-11-28-server-authority-on-history-design.md#what-leaves-and-what-arrives-on-the-website).

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | Ordinary statuses and details render across a snapshot and a saved update. The history status, its reason and its separate waiting row are absent. | `machines show their ordinary state without a history warning` in `machine-status.spec.ts`, at both configured widths. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1 | Change `src/frontend/src/screens/service.tsx`, `src/frontend/src/server/sync.ts` and `src/frontend/src/types.ts`. Add `src/frontend/e2e/machine-status.spec.ts`. Delete `src/frontend/e2e/machine-recovery.spec.ts` and `src/frontend/e2e/machine-recovery-review.spec.ts`. | `service.tsx` `Machines`, `MachineDetail`, `MachineRecoveryReason`, `PairMachine` and `machineAction`. `sync.ts` `toMachine`, `toJoining` and `applyClientChange`. `types.ts` `Machine` and `JoiningClient`. The target browser file follows `machine-recovery.spec.ts` `openMachines` and the shared `controlledWatch`. |

Delete the diagnostic label, detail callout, special joining row and both projections of the warning. Restore the standard offline detail, the existing host removal guard and the usual removal confirmation. Keep the watched online count, approved machine actions and pairing behavior. Read `src/frontend/src/dialogs.tsx` and the Add repository dialog as regression boundaries. The test warning fixtures are the current sample records that set field 19. The standalone mock currently has no field 19 seed.

First write `machines show their ordinary state without a history warning`. Use an authenticated website whose machine watch starts with an offline worker, an online host and a waiting enrollment tied to the token shown by Add machine. Deliver field 19 set to true in the red fixture by decoding its legacy Protocol Buffers tag into the fixture message. Keep this test fixture independent of the generated property so it compiles after reservation. Assert the exact Offline and Online cells, open approved rows, and assert absence of the exact status and reason strings. Require the usual offline detail, normal host removal protection and access to the waiting enrollment through the pairing dialog. Send a watched update that moves the worker online and assert the updated state and absence again. The initial run before item 1 must fail on the old status or reason. The raw tag is confined to this test fixture.

This item implements H11 and the second history decision. It uses foundation D1 and D5, rules R2, R5 and R8, and preserves the website observations in C6 to C9. It is low complexity and has no sign-in, key, token or ordering change. The test opens the existing pairing flow to observe the waiting record.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| Removing the diagnostic branch hides the waiting enrollment or drops ordinary offline context. | Medium | Observe the pairing dialog and expanded ordinary details in the same controlled watch fixture. |

**Verify:** Run the website type check and the Machines spec. The two configured Chromium widths provide the environment check. Compare the ordinary machine rows and expanded details against a baseline with field 19 false.

**WS-2: Put failed scans and restarts on the bell**

Item 2 reads failed requests from the machine watch and reuses the bell menu and the machine actions. The same mock engine serves the in-page and standalone samples.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2 | A failed scan appears with error tone, the action and the elapsed time. Its selection calls the original procedure for that machine. The next watched record clears it. | `failed scan retries the watched machine and clears on save`. |
| 3 | Restart uses its own procedure. Multiple failures remain distinct, a stale scope cannot send or surface a late error, and a removed machine loses its entries. | `failed restart and long names stay inside the bell` and `failed requests follow watch replacement and session loss`. |
| 4 | The entry remains until the ten-minute expiry update. The online Buildbox sample shows a failed scan and clears it on retry. | `an expiring failed request leaves on the watch` and `the mock Buildbox offers one failed scan`. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 2 | Change `src/frontend/src/app.tsx`, `src/frontend/src/types.ts`, `src/frontend/src/server/sync.ts`, `src/frontend/src/screens/service.tsx`, `src/frontend/src/css/base.css`, `src/mock-server/src/sample.ts` and `src/mock-server/src/engine.ts`. Add `src/frontend/e2e/failed-machine-requests.spec.ts). Extend `src/frontend/e2e/support/repository-server.ts` with restart procedure recording. Capture retry target identifiers in the browser request observer or the fixture procedure records. | `app.tsx` `Bell`. `ui.tsx` `Menu` and `MenuItem.hint`. `base.css` `.menu-item.two` and `.count.error`. `lib.tsx` `ago` and `useNow`. `sync.ts` `captureSession`, `machineCall`, `applyClientChange` and `clearMachines`. `service.tsx` `machineAction`. `sample.ts` `sampleClients`. `engine.ts` `createEngine`, `watchClients`, `save` and `scanClientHarnesses`. The target spec follows `machine-recovery.spec.ts` `openMachines` and `repository-server.ts` `serveRepositorySession`, `controlledWatch` and `startStandaloneMock`. |

Project `Client.failed_requests` into the optional `Machine.failedRequests` list of `{ id, kind, failedAt }` with milliseconds from `timestampMs`. Accept `FailedClientRequestKind.SCAN_HARNESSES` and `FailedClientRequestKind.RESTART` with a usable timestamp. The two supported kinds exhaust the current published list. Keep the mapping on the ordinary machine watch path. Snapshot replacement, saved records, removal and session reset already replace or clear the corresponding machines. Keep the entry's command identifier for identity and the owning machine identifier for the call.

Keep the current menu, open-request entries, header and footer. Keep existing badge and trigger strings when there are zero failed requests. Add the failed entries above those entries without applying the existing six-open-request limit to them. Use the existing error count style when there is a failure. The trigger's accessible name includes the failed count and the open count when present. Give each entry a two-line menu item. Use “Scan harnesses on <name> again” or “Restart <name> again” and the elapsed-time line from decision 11. Use a bell-specific wrap rule even for an unbroken machine name. Keep the menu's existing size and keyboard navigation.

Reuse `scanClientHarnesses(scope: SessionScope, id: string): Promise<void>` and `reconnectClient(scope: SessionScope, id: string): Promise<void>` in `sync.ts`. Export the existing `machineAction(scope: SessionScope | null, call: (scope: SessionScope) => Promise<void>, done?: () => void): void` from `service.tsx` for reuse. Capture the active scope for the rendered entries, and check it on selection and error. Require an entry still present on that current machine before sending. One selection makes one call. Treat a successful reply as acknowledgment of the call. The saved watch record controls the entry's removal. Keep the existing error toast if the same session's call is refused.

Seed one Buildbox scan with a real `failed_at` timestamp. Use the mock's ordinary save to publish a cleared list when the scan is accepted or expires. Use one deadline against the seed's failure time, independent of the mock simulation pace. Re-read the machine and the request identifier on expiry. A removed or already cleared record needs no save. The sample's returned record on the next watch snapshot also has an empty list after expiry.

First write these tests in `failed-machine-requests.spec.ts`.

| Test | Assertions before production code |
|---|---|
| `failed scan retries the watched machine and clears on save` | Start with an empty watch, then send a saved machine bearing one failed scan two minutes old. Open the bell and assert an error count of one, the exact label, and “Did not finish 2 min ago.” Activate it and inspect one authenticated `ScanClientHarnesses` request with the machine identifier and zero `ReconnectClient` calls. Hold the saved update beyond the call reply and reopen the bell to prove watch ownership. Send an empty failed list and assert removal without reload. |
| `failed restart and long names stay inside the bell` | Use a snapshot with two distinct failure identifiers for one machine, including restart, and an unaffected machine. Use an unbroken long name at both widths. Assert both entries and their full accessible names. Compare the menu's scroll and client widths and each entry's width. Activate restart by keyboard and assert one `ReconnectClient` with the owning identifier, no scan, and no action on the other machine. |
| `failed requests follow watch replacement and session loss` | Keep an open bell as a saved frame removes a failure or a removed frame removes its machine. Replace the watch with an empty snapshot and assert no retained count. Hold a retry reply across sign-out and sign-in to a fresh scope. Assert that the old result cannot show a toast in the new session and the old entry cannot issue a request there. Also reject a retry while current and assert the existing error toast with the entry still present. |
| `an expiring failed request leaves on the watch` | Install the browser clock, open the watched entry at failure age below ten minutes, and advance to one moment before expiry. Assert it remains. Advance to the ten-minute boundary and release the server fixture's empty saved record. Assert the open menu and count clear with no calls or reload. The Server plan supplies the real deadline and feed proof. |
| `the mock Buildbox offers one failed scan` | Open the standalone sample through `startStandaloneMock`. Assert one Buildbox scan entry, choose it, and assert the watch clears it. Smoke the same fixture record in the in-page sample. |

This item implements the visible part of H12 and history decisions three and four. It uses foundation D1 and D16, and rules R2, R5, R6 and R8. It is medium complexity because the watch, menu, actions and sample cross existing boundaries. It is risky because a delayed call can cross a session change. Show the retry test fails if the wrong remote procedure or machine is used. Show the session test fails when the callback uses whichever session is current at click time without checking the rendered scope, or when a stale rejection displays its toast. Show the expiry test fails when an empty saved list is merged into the old list.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A stale action targets a second server or a signed-out session. | High | Use the current authenticated scope and the machine and command identifiers. Freeze the delayed checked calls and assert the stale route fails. |
| A long name changes the menu width or hides an action on a phone. | Medium | Wrap only the failed label and assert overflow and keyboard selection at both widths. |
| Reload or retry reintroduces a failure that already left the list. | Medium | Derive rows directly from watch snapshots and changes. Assert removal after reply, replacement, removal and expiry. |

**Verify:** Run the website and mock type checks and the new bell spec. Run the spec in desktop and phone Chromium, and observe the sample in both transports. The oracle for expiration in the actual stack is the Server plan's authenticated live-watch test with controlled time.

**WS-3: Prove recovery with both programs**

Items 3 and 4 replace the history-proof family. Start their test work after the server and client builds are both present. Item 3 supplies the shared file and observation helpers before item 4 freezes its cases.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 5 | Restored and missing histories recover for the same enrollment. A server rollback within the retained range replays the suffix, and a larger rollback raises the server mark. | `TestServerAuthorityOnHistory/H1`, `/H2`, `/H6` and `/H7`. |
| 6 | The ambiguous delivered commands are settled once, the undelivered scan runs, and a waiting Projects call gets its failure. | `TestServerAuthorityOnHistoryCommands/H3`, `/H4`, `/H5` and `/H8`. |
| 7 | The existing revision 1 proof, ordinary readiness and replay, and damaged receipt cases still pass in the real PostgreSQL suite. | `TestLinkRevisionOneRefusal`, `TestSprintFourPendingRun`, `TestSprintFourReceiptReplay` and the entire `src/linktest` suite with the race detector and zero skipped history cases. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 3 | Replace the contents of `src/linktest/sprint_four_history_test.go`. Adapt shared Hello fixtures in `src/linktest/sprint_four_run_test.go`, `src/linktest/outbox_test.go` and `src/linktest/relay_test.go`. | `sprint_four_history_test.go` `TestSprintFourHistoryRecovery`, `pairedMachineWatch` and `requireSingleHistoryStop`. `sprint_four_run_test.go` `pairedScript.attempt` and `TestSprintFourPendingRun`. `outbox_test.go` `authenticateScriptedLink`, `installAudit`, `checkAudit` and `waitForAck`. `relay_test.go` `legObservation`, `helloObservation`, `recordRequest` and `relayControls`. |
| 4 | Add `src/linktest/sprint_four_history_commands_test.go` for H3, H4, H5 and H8. | The target follows `commands_test.go` `TestLinkRestartReplaysAfterDroppedResult` and `sprint_four_settlement_test.go` `testPairedStagedFolder`. Use `command_fixture_test.go` `newCommandFixture`, `requirePending` and `waitForStoredResult`, `sprint_four_projects_test.go` `newProjectFixture`, `beginAdd` and `waitProjectOperation`, and `sprint_four_history_test.go` helpers from item 3. |

Use `servertest.Start(t testing.TB, options servertest.Options) *servertest.Server` and `clienttest.Run(ctx context.Context, opts clienttest.Options) int` through the existing fixtures. Use a separate PostgreSQL database per fixture. Use `relayControls.beforeReply`, `holdEvent`, and `dropResult` or the existing server test seams at the stated phase. Gate signals are keyed to the relevant machine, event number or command. Release them on cancellation and cleanup. Before copying `client.db`, stop the real client and join its cleanup. Save and restore only that file under the disposable state directory, using the file-copy pattern in `testLegacyScanReplay` and `withClosedClientDB`. Add package-local helpers `saveHistory(t *testing.T, opts clienttest.Options) []byte` and `restoreHistory(t *testing.T, opts clienttest.Options, data []byte)`. Add a read-only closed-history observer returning next, retained numbers and its inferred acknowledged number. Reuse `checkAcknowledgedOutbox` as the shape for its bbolt view. The observer reads `meta.next` as the existing eight-byte big-endian integer and the keys of the `events` bucket. Capture Hello's two new fields on `legObservation` outside the immutable `helloObservation` used by reconnect equality assertions.

Update the scripted peer defaults for valid revision 2 announces. `authenticateScriptedLink` uses acknowledged and last equal to `firstEventNumber - 1` for its ready, no-suffix fixtures. `pairedScript.attempt` uses last equal to `first - 1` and acknowledged zero for its existing prospective replay and equal-ready fixtures. That preserves their modeled suffix and does not raise the mark. Keep the previous observation of ready, admitted and replay-through values. In `RestartMidDrain`, compare the keyed mark, generation and run state instead of the removed warning column. Retain `requireSingleHistoryStop` in the replacement file for the damaged receipt tests. Retain its single `link_stopped` and `store_failed` check, and leave the exact remediation words to the Client plan's corruption proof. Keep the startup and unchanged-byte assertions of the callers. Continue to run the standalone `TestLinkRevisionOneRefusal` and `LegacyReceiptAndBuffer` for the revision 1 behavior.

First write these four cases in `sprint_four_history_test.go`.

| Test | Existing helpers and fixture | New observation and assertable result |
|---|---|---|
| H1 | Use `newCommandFixture`, `holdEvent`, `waitForAck`, `waitForClientToProcessReplies`, the Facts version option and the version probe. Let the real client get Online, hold a numbered real scan report, and stop once the report is buffered but withheld. Save the stopped file with that retained row. Resume the original file, release normal delivery, drain through a larger mark, stop again, and restore the older bytes. | Compare the restored file's last number to the captured mark. In the restored leg, use `beforeReply` on Welcome and hold the first new event from the server with the relay. Require a proved Welcome at the stored mark and a database read of that same mark before any new commit. Assert the first new event is mark plus one and no restored overlapping number is sent. Release, wait for both fresh report kinds, their acknowledgments and the distinct version and inventory in watched state for the same machine. After a clean stop, confirm the outbox has acknowledged the new mark without holes. |
| H2 | Start and stop a real approved fixture after acknowledged Facts and inventory. Use `join`, `startClient`, `waitForClient`, the Welcome observer and `mark`. Delete only `client.db`, leaving the original key and identity. | Read and compare the identity and key files before and after. Assert an accepted empty Hello with acknowledged and last both zero, a Welcome at the original nonzero mark, first report at mark plus one and Online for the original identifier. Confirm a new history file after a clean stop. |
| H6 | Reuse the ack cut and ordered-effect observations in `TestLinkOutboxReplayAfterServerFailure`. Let the real client consume an acknowledged prefix. Drop the next event acknowledgment in `beforeReply` after the server has committed that event. This cuts the leg before the client forgets it. Keep redial blocked and stop the client before reading its closed history. Require the original mark to be above the acknowledged number and the retained suffix to contain the dropped acknowledgment’s event. Lower the disconnected machine's keyed mark to the saved acknowledged number. | Read the still-retained suffix, record the audit cursor after setup, and hold its first event until Welcome has been observed. Require Welcome at the lowered mark and one contiguous, ordered application and acknowledgment of the retained suffix before fresh reports. Use `installAudit` and `checkAudit` on the replay slice, or their keyed prior-mark read bounded to that slice. Mutation evidence must distinguish replay from a jump to the last number. |
| H7 | Use `drainPairedInventory` and `requireAcknowledgedOutbox` to establish an acknowledged number above zero, then stop. Lower the disconnected machine's mark below that number with `execute` and read it keyed by machine. | Hold new report handoff and assert the real client's Hello floor minus one, Welcome at that number and committed database mark at that number before any report applies. Release reports, assert fresh Facts and HarnessesFound apply, and observe Online and a normal stop. The test fails if the client keeps redialing at the old mark. |

First write these four cases in `sprint_four_history_commands_test.go`. Use the same WatchClients helper for keyed snapshots and saved records. Register it before releasing the admission gate when an absence-to-presence observation is required.

| Test | Existing helpers and fixture | New observation and assertable result |
|---|---|---|
| H3 | Use `newCommandFixture`, the file-copy helper, `requestScan`, the version probe's `arm` and `heldPID`, `waitForCommand`, `requirePending` and `waitForStoredResult`. Save a legitimate earlier file before advancing the real client's mark. Gate the first in-progress scan before any result, then request a second scan while the scan head is occupied. Read its identifier and sequence by machine and action. | Before cut, require a committed dispatch for the first command and no staged result. Require the second command to be unassigned and undispatched in the database. Restore the older file. The first row ends as `history_restored`. The second command arrives with its original identifier, completes and is stored as `done`. Check later legs through successful processing of the queued scan and one more welcomed reconnect for absence of the first identifier. Observe exactly one failed scan with its identifier, kind and failure time. Ask a new scan after the failure and assert its acceptance removes the entry on that same watch. A scan pending before the failure must not hide the notice. |
| H4 | Save a stopped, enrolled client's file before any restart request. Start it and drain a fresh report above that saved file's last number. Follow `TestLinkRestartReplaysAfterDroppedResult` through the primary restart. Put back the saved history without the restart receipt. Use `dropNext`, `waitForResult`, and `expectExit(t, 75)` for the original restart. | Confirm the dropped result reached the relay and did not reach the ledger. Relaunch with the restored file. Require `history_restored`, zero copies of the old identifier on the new stream, and a separately requested scan that completes before the replacement client stops with code zero. Verify the failed watch record names the original command as a restart. |
| H5 | Use `newProjectFixture`, `holdEvent`, `waitForResult` and the staged-row reads in `testPairedStagedFolder`. Save an older real history. Advance beyond that history. Hold the scan's event so its result reaches the server and stages above the current mark. Start `beginAdd` for a valid fixture checkout, and gate the real `OnFolderInspectionStart` by that operation and command so it has no report. | Before cut, require a keyed staged scan outcome and an open dispatched folder command without its report. Restore and keep the new report gate shut during admission. The scan row settles with its original `done` and no staged value. The original Add call gets the existing no-answer error before its normal deadline. The expected public result is `deadline_exceeded` with “The machine did not answer the folder request.” Read the same operation identifier and its `failed` state with failure `no_answer` and no reply or repository write. Neither command is reoffered after reset and neither is a failed bell entry. |
| H8 | Use the H3 file setup and one dispatched result-free scan. Use `beforeReply` to cut the first reset's Welcome exactly once and `relayLeg.done` to await that cut. Record the failure on the real machine watch before allowing the next Welcome to proceed. | The cut leg has a proved Hello, an upstream Welcome and no delivered Welcome. Read the first terminal outcome and `finished_at`. The next leg repeats the run and boundary and observes the same mark. It becomes Online. Rename the machine and observe the resulting `SetState` complete as a positive control while the scan failure stays visible. Assert the original terminal time is unchanged, the old command is absent from later frames, and the failed list has exactly one entry with the same identifier and timestamp throughout. Count list membership transitions, because ordinary presence saves can repeat the whole machine record. |

Both items are medium complexity and risky because ordering is observed at stream, transaction and file boundaries. Use brief channel gates and bounded condition loops. No case waits ten real minutes. H1 and H2 must fail if the client keeps the old stop or drops the fresh reports. H6 must fail if admission jumps to the last number. H7 must fail if the server leaves the mark below the acknowledged number. H3 must fail if reset fails all pending scans or retries the dispatched one. H4 must fail if the dispatched restart survives reset. H5 must fail if a staged success is overwritten or the folder call is left waiting. H8 must fail if repeating admission duplicates the failure entry or rewrites the terminal time. Use temporary source reversions or equivalent local mutations for these assertion failures. The integrated programs are already expected to pass the newly written tests. A compilation failure, timeout at fixture startup or stale generated field is not red evidence.

Items 3 and 4 implement H1 to H8 and exercise the first, third, fifth and sixth history decisions. They cover foundation D1, D4, D5, D7 and D16, rules R4 to R6, R8 and R10, and C6 to C11. Sprint 4 R5, R6, R9, R10 and R11 keep their ordinary replay, settlement and revision proofs beside the new history cases.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| Later events hide a bad mark or an early command delivery. | High | Gate events and read committed marks, terminal rows and run identifiers before releasing them. Check stream order and positive controls. |
| A header or prepared row looks like execution even though the real program never took the action. | High | Use client process results, a real version probe or folder inspection gate, and real command receipts or results as appropriate. |
| A late redial or gate races disposal and makes an absence assertion meaningless. | High | Observe complete relevant legs or successful later work before checking absence. Join the process before file operations. |
| Updating the shared scripted Hello changes the meaning of an existing proof. | Medium | Tell fixtures their modeled last and acknowledged values explicitly. Re-run the independent revision 1, stream fencing and deferred result cases. |

**Verify:** Run `deploy/test/with-postgres.sh -- go test -race -count=1 -timeout 25m ./src/linktest/...`. The test environment uses real PostgreSQL, the server fixture and a disposable client state folder. Require exit 0 and both history test families to run. For a readable per-case receipt add `-v` and a filter for the two history test families on a separate run.

**WS-4: Reserve the former warning field**

Item 5 reserves field 19 and its former name once all readers and writers have moved.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 8 | The live descriptor lacks field 19 and reserves its number and name. Field 20 and its shape are retained. Generated Go and TypeScript are reproducible. | `TestClientHistoryFieldIsReserved`, contract lint and a second identical generation. |
| 9 | The integrated tree passes the repository gate with the amended frozen history base. | Commands under rollout step 5 exit 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 5 | Change `src/contracts/proto/lgtm/v1/clients.proto`. Regenerate `src/contracts/gen/go/lgtm/v1/clients.pb.go`, `src/contracts/gen/lgtm/v1/clients_pb.d.ts` and `src/contracts/gen/lgtm/v1/clients_pb.js`. Add `src/contracts/client_history_reservation_test.go`. | `clients.proto` `Client`. `src/contracts/buf.gen.yaml` generation outputs. `src/contracts/package.json` `generate` and `lint`. The new test follows `service_discovery_test.go` `TestGetServerServiceDiscoveryIsAdditive` and `projects_test.go` `TestProjectsMatchItsDescription`. |

Before editing the schema, write `TestClientHistoryFieldIsReserved` in the target Go file. Use the generated `Client` message descriptor to assert `Fields().ByNumber(19)` and `Fields().ByName("event_history_needs_recovery")` are absent, and `ReservedRanges().Has(19)` and `ReservedNames().Has("event_history_needs_recovery")` are true. Assert that `failed_requests` is repeated field 20 of message type `lgtm.v1.FailedClientRequest`. Before the reservation it fails on field 19. Keep the Contract plan's independent field 20 and watch-shape tests. Run `pnpm --filter @lgtm/api generate` and re-run the same command against the resulting files to prove identical generated content. The Connect service declarations are unaffected by the record field removal.

This implements the website reservation of the history design and the removal order in the name table. It uses foundation D1 and R2. It is low complexity. The edit changes a descriptor, and it adds no sign-in, key, token or ordering behavior.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A field is dropped without reserving its name, or a remaining reader fails compilation. | Medium | Freeze the reservation test first. Inspect historical readers before regeneration, and run type checks for both website packages and the Go gate. |

**Verify:** Run `go test ./src/contracts -run 'TestClientHistoryFieldIsReserved' -count=1` and `pnpm --filter @lgtm/api lint`. Run the generation check in a clean built commit with `pnpm --filter @lgtm/api check`. There is no device check for this item.

**Existing tests this plan replaces or adapts**

| File and existing test | Change and reason |
|---|---|
| `src/frontend/e2e/machine-recovery.spec.ts` — `machine recovery follows snapshots and updates` and `recovery state survives stale approval and clears on watched readiness`. Frozen in `frozen-s4-web-8.sha256` and `frozen-s4-web-wrap.sha256`. | Delete with the warning-specific fixtures. The H11 test proves ordinary states and no diagnostic. Existing repository session tests continue to cover stale approval. |
| `src/frontend/e2e/machine-recovery-review.spec.ts` — `open removal confirmation follows a watched recovery flag` and `a saved frame clears the warning and keeps the waiting machine`. Frozen in `frozen-s4-web-8r.sha256` and `frozen-s4-web-wrap.sha256`. | Delete the obsolete confirmation and diagnostic row assertions. H11 keeps the pairing observation. |
| `src/linktest/sprint_four_history_test.go` — `TestSprintFourHistoryRecovery`, `RevisionOneRefusal`, `WarningSurvivesRestart/WithoutStagedResult`, `WarningSurvivesRestart/WithStagedResult`, `RemoveInstead`, `DiagnosticCommit/BeforeDelivery`, `DiagnosticCommit/AfterCommit`, `ClientStop/MissingHistory`, `ClientStop/CorruptRequiredPayload` and `ClientStop/IncompatibleWelcomeMark`. Frozen in `frozen-s4-paired-1.sha256`. | Replace the suite with H1 to H8. The standalone `TestLinkRevisionOneRefusal` already runs the identical revision-fence helper. Keep ordinary removal and damaged receipt tests. H10 keeps the corruption assertion under the Client plan. |
| `src/linktest/sprint_four_run_test.go` — `TestSprintFourPendingRun/RestartMidDrain` and `/EqualReadyBoundary`. Frozen in `frozen-s4-paired-1.sha256`. | Remove the obsolete column read under RestartMidDrain and use a consistent scripted Hello in both subtests. Preserve superseded-run, readiness and gap assertions. |
| `src/linktest/outbox_test.go` — `TestLinkOutboxReplayAfterServerFailure` and `src/linktest/reserved_result_test.go` — `TestLinkReservedCommandResult`. The helper is frozen in `frozen-linktest-a.sha256` and `frozen-s4-paired-1.sha256`. | Keep the existing test bodies. The common ready Hello fixture announces the existing acknowledged prefix under the new handshake. |
| `src/linktest/sprint_four_waiting_test.go` — `TestSprintFourWaitingReplay/StreamFencing`, `src/linktest/sprint_four_run_test.go` — `TestSprintFourPendingRun/EqualReadyBoundary/ProjectsEffect`, and `src/linktest/sprint_four_settlement_test.go` — `TestSprintFourFolderSettlement/R10/StagedAcrossRestart`. Frozen in `frozen-s4-paired-1.sha256`. | Keep the test bodies and update the common `pairedScript.attempt` fixture. These uses still prove ordinary stream fencing and settlement. |
| `src/linktest/sprint_four_receipt_test.go` — `TestSprintFourReceiptReplay/DamagedMetadata/MissingWatermark` and `/InvalidWatermark`. Frozen in `frozen-s4-paired-1.sha256`. | Preserve the bodies and shared clean-stop observation. Drop only the test helper's reliance on historical remediation text. |
| `src/frontend/e2e/support/repository-server.ts` and `src/linktest/relay_test.go`. Frozen with the website and paired suites. | Extend shared fixture capability. Keep existing callers and immutable run-pair assertions. Existing named assertions using these fixtures are unchanged. |
| `src/contracts/service_discovery_test.go` — `TestClientHistoryRecoveryIsAdditive`, subtests `all` and `saved`. Frozen in `frozen-s4contract-a.sha256`. | Ask Contract to replace the obsolete field-count, warning and old-wire assertions at its additive step, while field 19 still exists. Its replacement pins field 20 through both watch arms without asserting an exact field count across the scheduled removal. Keep `TestGetServerServiceDiscoveryIsAdditive`. |

**Rollout detail:** At steps 1 and 2, the ordinary `scripts/check-go.sh` runs with the amended `LGTM_LINK_BASE` from `.foundry/go-rewrite/link-base.txt`. The legacy PostgreSQL history cases remain pending replacement until step 4. The pre-Website check can use its normal compile and non-database test mode with `env -u LGTM_TEST_POSTGRES LGTM_LINK_BASE="$LGTM_LINK_BASE" bash scripts/check-go.sh`. Read that value from the name table’s history base file as in step 5. A database-backed gate at either earlier step requires an orchestrator-owned test handoff to prevent the obsolete assertions from running against the new behavior. At step 3 run the website and mock checks. At step 4 freeze and run the new paired cases with the PostgreSQL wrapper before the full suite. At step 5 run the following on the integrated tree. The wrapper supplies the environment for the Go gate.

```bash
pnpm --filter @lgtm/frontend typecheck
pnpm --filter @lgtm/mock-server typecheck
pnpm exec playwright test --config src/frontend/playwright.config.ts
deploy/test/with-postgres.sh -- go test -race -count=1 -timeout 25m ./src/linktest/...
LGTM_LINK_BASE=$(sed -n 's/^LGTM_LINK_BASE=//p' .foundry/go-rewrite/link-base.txt)
deploy/test/with-postgres.sh -- env LGTM_LINK_BASE="$LGTM_LINK_BASE" bash scripts/check-go.sh
```

At every item, the test writer freezes the named tests and verifies an assertion-level red before implementation or under the specified focused reversion. The build worker uses that frozen test commit. Review records the green command, the risky mutations that go red, and the regenerated files. The orchestrator updates the frozen lists or records the superseding lists. This plan assigns no edit to `deploy/` or any frozen list.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe is the five task deliverables, the six history decisions in table order, checks H1 to H12, the shared name table, the applicable foundation decisions, rules and C checks, and the changed parts of Sprint 4 checks R5, R6, R9, R10 and R11.

| Source item | Workstream | Status |
|---|---|---|
| Task 1, history decision 2, H11 | WS-1 | Covered by item 1. |
| Task 2, history decisions 3 and 4, H12 | WS-2 and WS-3 | Browser and mock prove the entry and request. H3 proves the real watch and retry. The Server owns controlled real expiry and early removal. |
| Task 3, website names table and field 19 removal order | WS-4 | Covered by item 5. |
| Task 4, history decisions 1, 5 and 6, H1, H2, H6 and H7 | WS-3 | Covered by item 3. |
| Task 4, history decision 3, H3, H4, H5 and H8 | WS-3 | Covered by item 4. |
| Task 5 and the history-related portions of Sprint 4 R5, R6, R9, R10 and R11 | WS-3 | H1 to H8 replace warning and stop checks. Preserve ordinary Sprint 4 proofs and send the exact Stack wording to the orchestrator. |
| H9 and H10 | WS-3 | Server and Client test ownership. Paired tests use valid announcements and preserve existing damaged receipt coverage. |
| Shared Hello fields, Welcome mark and `history_restored` | WS-3 | Fixture reads, operation and command assertions from the published names. |
| Shared field 20 message, kinds and watched lifetime | WS-2 and WS-3 | Covered by browser menu and H3, H4, H5 and H8 watch assertions. |
| Foundation D1 and D5, rules R2, R5 and R8, C6 to C9 | WS-1 and WS-2 | Reuse the attached machine watch, actions and status. |
| Foundation D4, D7 and D16, rules R4, R6 and R10, C10 and C11 | WS-2 and WS-3 | Use the real authenticated pair, preserve event order and run the unchanged revision fence. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md` directs website work to `src/frontend/` and contract changes to `src/contracts/proto/`. | The visible changes use the existing website. The field change starts in `clients.proto`. The task supplies the mock and paired-test exceptions. |
| `AGENTS.md` treats the old TypeScript engine as reference and requires credit for copied external code. | The work uses local code and established fixtures. The plan introduces no external source. |
| `AGENTS.md` says the three programs meet through the contract. | Pair fixtures start the public test entry points. The website reads the generated client records. Database reads in the paired tests are fixture observations keyed to disposable records. |
| `AGENTS.md` records repository problems and sends finished work to the base through a pull request. | The bounded result carries the new seam and Stack findings for the orchestrator. The orchestrator handles the eventual integration. |
| `AGENTS.md` requires full sentences and repository-relative paths, and the assignment restricts file edits. | The planner runs the privacy check and reads the new files, leaving the recipe and Stack edits in the result handoff. |
| The build loop freezes tests and requires a mutation for risky changes. | Each item specifies the tests and observation. Items 2 to 4 name the assertions the mutations must break. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| Item 1 | `mobile-ui` | `website/frontend` | low | Remove one conditional presentation and its fixtures while keeping ordinary rows. | No | Contract. |
| Item 2 | `mobile-ui` | `website/frontend` | medium | One existing menu joins authenticated actions to a watched record and two sample transports. | Yes | Item 1 and Contract. |
| Item 3 | `backend` | `website/backend` | medium | Four paired fixtures reuse one client file pattern and existing stream controls. | Yes | Item 2 and both program builds. |
| Item 4 | `backend` | `website/backend` | medium | Four cases use existing command and Projects gates and item 3's history observer. | Yes | Item 3 and both program builds. |
| Item 5 | `contracts` | `contract/contracts` | low | Mechanical reservation and regeneration with a descriptor proof. | No | Items 1 to 4 and the server's last removed consumer. |

The work-area sheet lists the user interface area as `mobile-ui`. Those two units use the Website scope at desktop and phone widths. The items run in order so they can share the watch and test fixture files. The Server and Client plans can run in parallel before the paired work.

Load is 16 OpenAI launches. Count one planner, three plan reviewers, five test writers, five builders and two post-build reviewers. All four roles use that one vendor. Fifteen launches remain after this planner. The live remaining allowance is available to the orchestrator at dispatch. Compare it with those 15 seats and schedule any overflow after reset with the same vendor. The orchestrator checks the two Website scopes against the task's file limits before dispatch. The existing `contract/contracts` scope passed its check during planning.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | Pending | The orchestrator runs this round after the bounded planner's self-check. |

</details>
