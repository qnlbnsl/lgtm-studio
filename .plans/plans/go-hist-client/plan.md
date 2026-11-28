---
topic: go-hist-client
status: attack
tier: M
created: 2026-11-28
updated: 2026-11-28
epic: go-rewrite
---

# The client reconnects when its saved history is older than the server's

## Overview

When a machine loses or restores its saved event history, the client can reconnect using the last event number the server has applied. A client with intact enrollment can create a missing history file, learn the server's number, and send fresh facts and harness inventory without enrolling again.

The client keeps the numbered reports that the server still needs. If the server has already passed the client's last number, the client empties the old events and advances its local counter in one write. The tests check both parts of the write on failure and verify that the next reports use the new numbers.

A mark that is too low closes only that connection. An unreadable or damaged history still stops the client and leaves the file available for inspection. A signed test peer verifies what the client sends. The later paired tests verify that the real server applies it.

## Key decisions

The client sends the event numbers it has saved at each new connection. It also sends a boundary that identifies where this process started writing. That boundary stays fixed even when the server advances the event numbers.

Taking the server's mark uses one write. The client removes its old numbered events and saves the next number together. Command receipts keep the event numbers they originally recorded, so a repeat result carries its original ordering information.

An unreadable or damaged history still stops the client with a diagnostic. The approved design specifies exit status zero for both. The running client currently exits with status one for an actual read failure, so the build must change that case.

Full log → [decisions.md](decisions.md).

## Workstreams

### Each connection announces the client's saved numbers

A server can tell whether it already has part of the client's history. A signed test peer observes the saved numbers on successive connections while the process boundary stays fixed.

### A higher server mark is saved in one write

A successful write empties the old events and saves the new next number. A reopened history shows both effects together. The fault test checks that an aborted write keeps the previous events and number together.

### The client reconnects and reports its current state

A low mark closes the stream and the client tries again. A usable higher mark lets the client send fresh machine facts and harness inventory. The signed peer also checks that a replayed command receipt keeps the number it held before adoption.

### A missing history starts empty after enrollment is verified

The executable makes an empty history when the key and saved identity are usable. The executable proof also covers an unreadable history and a damaged history. These cases stop and leave the file available for inspection.

## Rollout order

1. Land the Contract team's revision 2 amendment first so the new Hello fields are available to the client.
2. Build and verify the client with its own signed peer. The Server team can build its admission change at the same time.
3. Land the Website and paired-test change after both sides are ready. Run the real server and client together against PostgreSQL before taking this history rule into sprint 5.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1: Each connection announces the client's saved numbers**

Item 1 takes one snapshot of the committed event counters just before the client opens a stream. It uses the process announcement from startup for the process boundary and run identifier. Its scope is `client/backend`. It needs the Contract build. Item 2 follows it because both edit the outbox file. The test writers for items 1 and 4 can prepare their separate cases in parallel.

**Files.** Change `src/client/internal/link/outbox.go` and `src/client/internal/link/conn.go`. Change the attempt call site in `src/client/internal/link/session.go`. Add `src/client/internal/link/history_hello_test.go`.

**Design.** History decisions 1 and 6, the Hello table and the immutable boundary under “What the client stops doing”. Client evidence for H6, H7, H8 and the valid announcements corresponding to H9.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | A dial reports the retained bounds and a later dial reports the new bounds with the same run identifier and boundary. An already empty file still announces its last acknowledged number. | `TestHistoryHelloUsesCommittedBounds` |
| 2 | A reply lost before Welcome leaves the saved announcement bounds and process pair available for the next dial. | `TestHistoryHelloRepeatsAfterLostWelcome` |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1.1 | Add the package-private `historyBounds` value and `func (s *Store) historyBounds() historyBounds`. Read `next` and `count` under one store lock. The last number is `next - 1`, and the acknowledged number is `next - uint64(count) - 1`. Use the existing committed cache. | `src/client/internal/link/outbox.go` (`Store.Ack`, `Store.Next`) and `src/client/internal/link/store.go` (`eventSnapshot`, `inspectBuckets`). |
| 1.2 | Pass the snapshot separately from `runIdentity` in `clientRun.attempt` to `openSessionWithRevisionFence`. Send `Hello.AcknowledgedEventNumber` and `Hello.LastEventNumber` with `Hello.FirstEventNumber`. Keep the direct test wrapper `openSession` callable with its current parameters. For this wrapper, use an unacknowledged fixture with last equal to first minus one. | `src/client/internal/link/session.go` (`Run`, `clientRun.attempt`), `src/client/internal/link/run_identity.go` (`runIdentity`), and `src/client/internal/link/conn.go` (`openSession`, `openSessionWithRevisionFence`). |

**Tests written first.** In `src/client/internal/link/history_hello_test.go`, follow `TestRevisionTwoRunAnnouncement`.

- `TestHistoryHelloUsesCommittedBounds`. Seed events 1 to 4 and acknowledge 2 before starting the client. The first Hello reports first 5, acknowledged 2 and last 4. Give the client a signed waiting Welcome at 2. Receive and acknowledge 3 and 4. Disconnect after the acknowledgement is processed. The second Hello reports first 5, acknowledged 4 and last 4 with the same run identifier. In the new test file, use the commit seam to signal deletion of event 4 after commit. The existing acknowledgement helper is the pattern and only watches 1 and 2.
- `TestHistoryHelloRepeatsAfterLostWelcome`. Seed retained events with a nonzero acknowledgement. Read Hello and verify Proof. Close the first attempt before sending Welcome. The next Hello repeats both bounds and the process pair. A second process using that history gets a different run identifier. Keep the new test's assertion on the nonzero wire numbers even if the process pair check already passes.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An inconsistent sample or stale number can cause the server to settle history using the wrong bounds. | High | The wire test observes both numbers before and after a durable acknowledgement. The run boundary has its own storage. |

**Mutation proof.** This item is risky because the announced order affects admission. Cache the two numbers once at process startup and show `TestHistoryHelloUsesCommittedBounds` fail on the redial. Compute the acknowledged number from last alone and show it fail on the first dial.

**Verify.** Run `go test -race -count=1 -v ./src/client/internal/link -run '^(TestHistoryHelloUsesCommittedBounds|TestHistoryHelloRepeatsAfterLostWelcome|TestRevisionTwoRunAnnouncement)$'` on native Linux and macOS. Require the named passes and no skips.

**Names.** The wire fields are `lgtm.link.v1.Hello.acknowledged_event_number` and `last_event_number`, unsigned 64-bit fields 6 and 7. The existing `run_id`, `first_event_number`, and link revision 2 keep their meaning. The private `Store.historyBounds` method can be used by the later Welcome tests. There is no new exported Go name.

**WS-2: A higher server mark is saved in one write**

Item 2 supplies the storage operation. It follows item 1. Item 3 needs it to apply Welcome. Item 4 changes separate production files if its startup call-site edit is integrated after item 3.

**Files.** Change `src/client/internal/link/outbox.go`. Add `src/client/internal/link/history_store_test.go`.

**Design.** History decisions 1 and 5, the Welcome branches under “How the two sides compare their numbers”, and the receipt exclusion under “What the client stops doing”. Client evidence for H1, H6, H7 and H10.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | Reopening after a higher mark shows an empty outbox, the adopted next number and byte-identical receipts. | `TestHistoryStoreAdoptsHigherMark` |
| 2 | An injected abort rolls back the event deletion and counter change together and leaves the in-memory event snapshot unpublished. | `TestHistoryStoreAdoptionRollsBack` |
| 3 | In-range, stale and unrepresentable marks leave the store unchanged at this step. The last legal mark can be adopted and reopened without overflow. | `TestHistoryStoreWelcomeBoundaries` |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 2.1 | Add `func (s *Store) adoptWelcomeMark(ctx context.Context, mark uint64) (adopted bool, err error)`. With the lock held, check store health and cancellation. Return `errInvalidWelcome` for a mark below the acknowledged number or above `math.MaxInt64`. Return false with no write for an in-range mark. For a higher legal mark, replace the contents of the events bucket and write `meta.next = mark + 1` in one `updateContext` call. Change the cached snapshot to that next number and zero count and bytes after commit. | `src/client/internal/link/outbox.go` (`Store.Ack`, `Store.appendAtNext`), `src/client/internal/link/store.go` (`initializeStore`, `Store.updateContext`, `inspectBuckets`), and `src/client/internal/link/conn.go` (`errInvalidWelcome`). |
| 2.2 | Preserve receipt and enrollment records. Apply the existing failed-write fence if the transaction errors, including an uncertain commit outcome. A later process reopens the file and inspects the durable state again. Do not call `Store.Ack` for adoption. | `src/client/internal/link/store.go` (`Store.ready`, `Store.updateContext`) and `src/client/internal/link/outbox.go` (`Store.Ack`). |

**Tests written first.** In `src/client/internal/link/history_store_test.go`, follow `TestAckRemovesUpToNumber` and the `swapCommit` fixture.

- `TestHistoryStoreAdoptsHigherMark`. Seed events 1 to 4, acknowledge 2 and save a receipt. Adopt 9. Observe one commit and zero retained bytes and rows. Reopen and read next 10 and bounds 9 and 9. Compare the saved receipt bytes. Append the next report and receive 10.
- `TestHistoryStoreAdoptionRollsBack`. Use the commit seam to run the reset callback in a real transaction and then abort before commit. Assert that the callback attempted both state changes. Inspect the still-old cached snapshot and fence, and reopen on a fresh object to observe the old counter, numbered rows and receipts. Add an uncertainty case that commits successfully and then reports an error. Assert that no more writes occur through the failing store object and reopening reveals the complete adopted state.
- `TestHistoryStoreWelcomeBoundaries`. Check the acknowledged and last endpoints on a retained suffix and on an empty advanced store. Check one below the floor and marks above the legal ceiling, including the maximum unsigned value. No transaction runs for these cases. Add a structurally sound store with one unreadable event payload below the higher mark. Require adoption to discard that covered row. From a small next number, adopt `math.MaxInt64`, reopen at `math.MaxInt64+1`, and prove a later append cannot allocate another event.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A partial reset can resurrect the old events under a new counter, or lose them under the old counter. | High | Assert one transaction, inject rollback after the callback, and inspect both sides after reopening. |
| A large unsigned mark can produce an invalid saved counter. | High | Check the event ceiling before adding one and retain the existing exhausted-store behavior. |

**Mutation proof.** This item is risky because it deletes numbered rows. Remove the event deletion, commit the counter separately, or publish the in-memory snapshot before the commit result. Each mutation fails an adoption or rollback assertion. Remove the legal ceiling check and show the boundary test fail.

**Verify.** Run `go test -race -count=1 -v ./src/client/internal/link -run '^(TestHistoryStoreAdoptsHigherMark|TestHistoryStoreAdoptionRollsBack|TestHistoryStoreWelcomeBoundaries|TestAckRefusesAhead)$'` on native Linux and macOS. Require the named passes and no skips.

**Names.** Item 3 calls the private `(*Store).adoptWelcomeMark` with the existing `Welcome.last_event_number`. `client.db` keeps its `meta`, `events` and `receipts` buckets. The next counter remains the existing unsigned 64-bit big-endian value under `meta.next`. There is no new exported Go name.

**WS-3: The client reconnects and reports its current state**

Item 3 uses the Welcome classification before the client exposes a live session to commands or starts gathering fresh reports. It follows items 1 and 2. Item 4 lands after this item because both edit the session and the old terminal-history test file.

**Files.** Change `src/client/internal/link/session.go`, `src/client/internal/link/receipts.go`, `src/client/internal/link/report.go` and `src/client/internal/link/retry.go`. Add `src/client/internal/link/history_session_test.go`. Change `src/client/internal/link/replay_test.go`, `src/client/internal/link/session_test.go`, `src/client/internal/link/revision_two_recovery_test.go`, `src/client/internal/link/review_f_ack_test.go`, `src/client/internal/link/receipts_test.go`, `src/client/internal/link/revision_two_receipts_test.go`, `src/client/internal/link/revision_two_receipts_review_test.go` and `src/client/internal/app/client_test.go` as inventoried below.

**Design.** History decisions 1, 3 and 5, the three Welcome branches and the final paragraph under “How the two sides compare their numbers”, and the receipt and stop rules under “What the client stops doing”. Client evidence for H1 and H3 to H8. A local fault still satisfies H10.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | A valid higher Welcome drops old numbered events and sends current Facts and HarnessesFound starting above the mark. A reconnect repeats the run boundary and announces the adopted bounds. | `TestHistoryWelcomeAdoptsAndReports` |
| 2 | A stale Welcome closes only that stream. The later usable mark permits replay. An in-range Welcome still screens the required suffix before dropping the prefix. | `TestHistoryWelcomeRetries`, `TestReplayResumesAboveFreshWelcomeAfterGap`, `TestRevisionTwoTerminalHistory` |
| 3 | The peer receives original numbers on finished and interrupted receipt replays and a bounded number on a newly finished result. The repeat scan does no second work, and repeating SetState preserves its standing-result semantics. | `TestHistoryWelcomePreservesReceipts` |
| 4 | An invalid live acknowledgement is rejected at its arrival-time ceiling and never adopts a mark. The process can reconnect. | `TestReviewFAckCeilingUsesReceipt`, `TestHistoryLiveAckRetries` |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3.1 | Reconcile the mark after `stopOutage` has joined the previous worker and before command dispatch, fresh reports or sender handoff. Handle `errInvalidWelcome` as an ordinary retry error. Keep a bad Welcome outside the local store-fault path and avoid starting a new outage write just because that Welcome was rejected. In `supervise`, use the existing failed-connect delay for `errInvalidWelcome` even when transport already reported a verified Welcome. Keep the established-stream retry behavior for the streams that were usable. Compare the current committed store counters here. A late, durable write above the mark remains in the replay suffix. | `src/client/internal/link/session.go` (`liveSession.serve`, `clientRun.attempt`, `clientRun.stopOutage`, `linkFailureCode`) and `src/client/internal/link/retry.go` (`supervise`). |
| 3.2 | For an in-range mark, call `newReplayWithFilter` so preflight validates the required suffix before deletion. After adoption, use the same replay initialization from the now-empty outbox. Initialize the replay and handoff ceilings at the Welcome mark, as the existing stream does. Keep corruption found during open, receipt inspection or required replay on the local stop path. A covered event payload has no replay obligation, so the normal path and adoption can discard it without decoding it. | `src/client/internal/link/replay.go` (`newReplayWithFilter`, `screenReplay`, `replayRange`, `replay.next`) and `src/client/internal/link/session.go` (`liveSession.serve`, `stopLocal`). |
| 3.3 | Change `func (s *Store) prepareRun(watermark uint64) error` and call it with `firstEventNumber - 1` once per process after usable Welcome reconciliation. Keep its validation and transaction and reject a watermark above the store's durable last number. Existing finished and interrupted receipts replay their stored number, which is at or below the pre-adoption last number and therefore below the adopted mark. The inherited start is only sealed once, under the prior process boundary. New finishes use the last committed number at the time of the finish. Until a fresh event is written, that number can equal the adopted mark. Because next never decreases, none of these values can exceed the last committed number at result handoff. Keep the existing append-before-result and validation paths. | `src/client/internal/link/receipts.go` (`prepareRun`, `findReceipt`, `writeFinishedReceipt`), `src/client/internal/link/store.go` (`checkRunReceipts`), `src/client/internal/link/commands.go` (`handle`, `reply`), and `src/client/internal/link/session.go` (`queueResult`, `liveSession.serve`). |
| 3.4 | Clear the two pending unnumbered snapshots through a small private reporter method under the reporter lock when adoption succeeds. The outage worker is already joined and the current stream has yet to start its report worker. Let the existing approved Welcome branch read current Facts and HarnessesFound. Leave regular replay, report ordering and outbound screening in place. | `src/client/internal/link/report.go` (`reportQueue`, `reporter.publish`, `reporter.flush`) and `src/client/internal/link/session.go` (`liveSession.serve`). |
| 3.5 | Keep the live acknowledgement arrival and replay ceilings. Map an above-ceiling live acknowledgement to `errInvalidFrame` and end that stream. Keep the private replay and store validation errors for defensive calls, including the exported `ErrAckAhead` used by `Store.Ack` callers. Remove `impossible_ack` from `recoveryStoreCode`. Do not change how the client handles genuine local faults or verified revision refusal. | `src/client/internal/link/session.go` (`liveSession.serve`, `liveSession.receive`, `recoveryStoreCode`), `src/client/internal/link/replay.go` (`replay.ack`), and `src/client/internal/link/outbox.go` (`Store.Ack`). |

**Tests written first.** In `src/client/internal/link/history_session_test.go`, use the signed `runPeerFixture` and the actual `Run` entry point.

- `TestHistoryWelcomeAdoptsAndReports`. Seed old Facts under 1 and old HarnessesFound under 2. Read Hello with first 3 and last 2. Verify Proof and send an approved Welcome at 9. Observe current fixture Facts under 10 and the current harness scan under 11. Distinguish the old values from the new probe values. Require no old numbered event handoff. Acknowledge both and disconnect. The next Hello keeps the process pair and reports bounds at least 11, allowing the existing disconnect Facts append to advance last. Include a same-process retry variant. Seed the default 10,000-event limit in one fixture transaction. Use an approved Welcome and withhold event acknowledgements so an earlier Facts or inventory sample stays pending. Wait for the first report worker to quiesce, disconnect, and deliberately hold the fresh Facts probe on the next approved Welcome at a higher mark. Use the commit seam to reject any old snapshot published before releasing the probe. Reopen to read the resulting next number.
- `TestHistoryWelcomeRetries`. Use separate cases for a retained suffix, an empty advanced queue and an out-of-domain mark. With acknowledged 2 and events 3 and 4 retained, send a verified Welcome at 1. Observe the stream close with no event, result or history write. The same run dials again and a Welcome at 2 replays 3 and 4. With an empty store at next 5, reject a mark below 4 and accept 4 on retry. Reject a mark above the event number ceiling without wrapping. For two consecutive unusable Welcomes, use virtual time to assert the existing failed-connect delay and then observe another dial. In a separate late-write case, use the shared-store attempt fixture. After Hello with last 4, append 5 and 6 while the peer holds Welcome. Release Welcome at 5 and require event 6 with its original number and payload. Keep the corruption and full preflight tests in the existing suite.
- `TestHistoryWelcomePreservesReceipts`. Seed three events, an unexpired finished scan at number 2, a finished standing receipt at number 3 and an inherited started receipt. Give a signed, approved Welcome at 9. Issue repeat attempts from the double to exercise the receipt paths. Require finished results at their original numbers and the inherited result at 3 with outcome interrupted. Assert identifiers and attempt numbers, preserved terminal receipt bytes and unchanged scan invocation count for repeats. Issue a different fresh scan, observe one scan and its durable report, and assert the new result number against the counter in the last committed transaction at handoff. Assert that every result number is at or below the actual durable last number at handoff. The real Server plan owns which commands are resent.
- `TestHistoryLiveAckRetries`. Use a waiting peer with retained events 1 and 2. On the first stream only, park the sender after handoff of 1 while 2 is still unhanded. Send `EventAck.through` for 2. Require stream closure, zero acknowledgement writes, the same retained suffix on a second dial, and successful replay after a usable Welcome. Require a stream diagnostic and zero `link_stopped` lines before explicit cancellation.
- Replace the old terminal mark cases as inventoried below. Preserve the late-worker write-fence test by making event 3, the outage Facts append, unreadable through the existing commit seam after two good, acknowledged events. Give the second proven stream mark 2 and hold the requested scan until that stream closes. Assert that corruption of required event 3 still fences the late scan before its append or receipt write. Keep the separate valid last-event and revision refusal tests.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A low Welcome or live acknowledgement could drop valid queued events or permanently fence writes. | High | Check events and the next attempt on the wire and keep the delayed acknowledgement test. |
| Late writes or a receipt reseal could either reuse a number or create a false ordering barrier. | High | Join the outage owner before reconciliation and pass the original boundary explicitly to receipt preparation. Test both inherited and current receipts. |
| An old pending report could become the first new report after a reset. | Medium | Clear the two pending slots only after adoption and observe the new payloads as well as their numbers. |

**Mutation proof.** This item is risky because it controls ordering. Classify a low mark as a store fault, accept a live acknowledgement at the processing-time ceiling, seal an inherited receipt at the adopted mark, or start fresh reports before adoption. Show the relevant retry, ceiling, receipt or fresh-report test turn red. Bypass required suffix screening and show the retained corruption assertion fail.

**Verify.** On native Linux and macOS, run `go test -race -count=1 -v ./src/client/internal/link ./src/client/internal/app -run '^(TestHistoryWelcomeAdoptsAndReports|TestHistoryWelcomeRetries|TestHistoryWelcomePreservesReceipts|TestHistoryLiveAckRetries|TestReviewFAckCeilingUsesReceipt|TestReplayRejectsUnhandedAcknowledgement|TestReplayResumesAboveFreshWelcomeAfterGap|TestRevisionTwoTerminalHistory|TestAppRunStoreStatus)$'`. Require named passes and no skips. Run the full client suite on both systems for receipt and outgoing filter coverage.

**Names.** Keep `Welcome.last_event_number`, `EventAck.through` and `CommandResult.last_event_number`. Keep the existing `receipts` values and the exported `clienttest.Options` and `clienttest.Run` seams. The simulated retried commands use existing outcomes, including `COMMAND_OUTCOME_INTERRUPTED`. The server-only state `history_restored` never appears in a result sent by this client.

**WS-4: A missing history starts empty after enrollment is verified**

Item 4 uses the existing safe creation path at the same point where Run now opens history. It follows item 3 for its first Welcome proof and for shared files. The existing application path validates the saved enrollment and matching key first.

**Files.** Change `src/client/internal/link/store.go` and `src/client/internal/link/session.go`. Change `src/client/internal/app/revision_two_process_test.go`, `src/client/internal/app/client_test.go`, `src/client/internal/link/revision_two_recovery_test.go`, `src/client/internal/link/revision_two_store_open_review_test.go` and `src/client/internal/link/revision_two_store_read_review_test.go` as inventoried below.

**Design.** History decision 1 and the two file cases under “What the client stops doing”. H2 and H10. Keep the process mapping in `app.Run` and the installed service definitions.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | With intact enrollment, the real executable recreates the missing history, announces first 1 and both bounds zero, and sends the first fresh report above a higher Welcome. The saved key and identity bytes match. | `TestRevisionTwoExecutableStop/missing history resumes` |
| 2 | A true read failure or damaged history stops with status zero and one `link_stopped` line. A readable file with only write access denied retains status one. | `TestRevisionTwoExecutableStop/unreadable store`, `TestRevisionTwoExecutableStop/invalid counter`, `TestRevisionTwoStoreOpenPermissionFailure`, and Linux `TestRevisionTwoStoreMetadataRead/temporary read fault` |
| 3 | Missing or mismatched enrollment prevents history creation and transport. | `TestRevisionTwoExecutableStop/missing history without usable enrollment` |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 4.1 | Call `CreateStore` from `link.Run` after the caller has validated the enrollment. The creation path runs only after the store path is confirmed missing. Reuse the folder claim, private temporary file, atomic placement and directory sync. Keep `OpenStore` available as the open-only entry point. | `src/client/internal/app/client.go` (`Run`, `Join`), `src/client/internal/link/session.go` (`Run`), `src/client/internal/link/store.go` (`CreateStore`, `OpenStore`, `openStore`, `createStoreFile`), and `src/client/internal/state/folder.go` (`Open`), and `src/client/cmd/lgtm-client/main.go` (`run`). |
| 4.2 | Identify a read failure during inspection of an existing file separately from a lock, unsafe path, write-only open or creation error. Use an internal marker while preserving the existing `ErrStoreFailed` inspection classification. Include positional metadata read failure and read permission denial. Map that marker at the runtime stop boundary to `RunRecoveryStop` with one plain `link_stopped` and `store_failed` line. Keep the damaged-file diagnostic and applicable write fences. Preserve the public error behavior for the open-only helper, the existing outcome for write failures, and the distinction between an invalid file and a failed read. | `src/client/internal/link/store.go` (`inspectStore`, `openExistingBolt`, `classifyBoltMetadata`, `storeOpenError`), `src/client/internal/link/session.go` (`stopForStore`), and `src/client/internal/app/client.go` (`Run`). |

**Tests written first.**

- `src/client/internal/app/revision_two_process_test.go`, `TestRevisionTwoExecutableStop/missing history resumes`. Seed valid enrollment, remove only `client.db`, then start the built binary. Add the test-only optional `observeHello func(*linkv1.Hello)` field on `appPeer` and call it in `challenge` after basic validation. Use the signed peer to observe the zero bounds and first 1. Give the peer a higher mark such as 7 and require reports 8 and 9. Send the process a termination signal after both observations, wait for clean exit, and verify a durable store and unchanged enrollment bytes. Require zero history-stop logs before cancellation. Use the same process fixture for the missing or mismatched enrollment control. Observe status one, a startup error, no new file and zero connections.
- `src/client/internal/app/revision_two_process_test.go`, `TestRevisionTwoExecutableStop/unreadable store`. As an ordinary user, deny reads on an existing private file. Require exit zero before cancellation, exactly one plain `link_stopped` with `store_failed`, no dial, and byte-identical history after restoring permissions to inspect the bytes. Preserve the existing invalid-counter, invalid-required-event and revision-refusal subtests.
- `src/client/internal/link/revision_two_store_open_review_test.go`, `TestRevisionTwoStoreOpenPermissionFailure`. Change the unreadable subtest to require `RunRecoveryStop` with no returned error. Keep the write-only open denial subtest at `RunStopped` with `ErrStoreFailed`. Both cases keep the separate `OpenStore` error classification, zero transport and the file bytes.
- `src/client/internal/link/revision_two_store_read_review_test.go`, `TestRevisionTwoStoreMetadataRead/temporary read fault`. Reuse the Linux metadata read fault with the real executable. Expect a successful process exit with the same one plain diagnostic, and verify the bytes and successful open once the fault is absent. Preserve the readable-header and invalid-metadata controls.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Creating history after an arbitrary failure can replace important evidence or bypass enrollment. | High | Reuse the missing-only store operation and assert enrollment, unreadable and corruption controls at the executable. |
| Treating write or lock failures as unreadable would change supervisor behavior. | Medium | Verify the read versus write distinction. Preserve the existing store opening and failure classifications. |

**Mutation proof.** This item is risky because it touches the enrollment gate and saved history. Recreate a store on a read failure, move creation ahead of enrollment verification, or treat every existence or access error as a missing file. Show the executable controls fail. Return status one for the unreadable case and show the executable and Linux metadata tests fail.

**Verify.** Run `go test -race -count=1 -v ./src/client/internal/link ./src/client/internal/app -run '^(TestRevisionTwoExecutableStop|TestRevisionTwoStoreOpenPermissionFailure|TestRevisionTwoStoreMetadataRead|TestOpenStoreMissingAfterJoin|TestOpenStoreCrashDuringCreation)$'` as an ordinary user on native Linux and macOS. Require each applicable named pass. The metadata fault test is explicitly Linux-only. The portable executable and permission tests prove the same stop policy on macOS. Require no permission subtest skips.

**Names.** Keep `machine.key`, `identity.json` and `client.db`. Keep the `clienttest.Run` signature and application exit mapping. The new missing-file path uses the existing `CreateStore` function and creates the existing empty format.

**Existing test changes**

Only the replacements and call-site adaptations below edit existing Client tests. For each touched frozen file, the orchestrator freezes the reviewed replacement before the corresponding builder starts. The old proof stays in the earlier commit. The test writers and builders work from the replacement expectation.

| File | Existing test or helper | Change and reason | Prior freeze |
|---|---|---|---|
| `src/client/internal/link/revision_two_recovery_test.go` | `TestRevisionTwoTerminalHistory` | Remove `welcome at next`, `empty suffix below floor` and `welcome below retained floor` in item 3. Replace them with adoption and retry assertions in the new session tests. In item 4 remove `missing database` and its table-only assertions in favor of the executable case. Change `late scan cannot settle after recovery` to use a damaged required event. Preserve genuine terminal cases and handoff controls. | Sprint 4 Client 3 and 8. |
| `src/client/internal/link/replay_test.go` | `TestReplayRejectsImpossibleMarks` | Keep the live replay ceiling part and name it `TestReplayRejectsUnhandedAcknowledgement`. Remove the old assertion that a high Welcome is terminal. Direct `newReplay` remains the normal replay primitive. | Sprint 3 Link B and Sprint 4 Client 1, 2, 3 and 8. |
| `src/client/internal/link/session_test.go` | `TestRetryFailureExitsAndKeepsStore` | Remove the impossible Welcome and live mark rows. The new session tests verify adoption and retry. Keep open, read, commit, exhaustion and cancellation cases. | Sprint 3 Link B and Sprint 4 Client 1, 2, 3 and 8. |
| `src/client/internal/link/review_f_ack_test.go` | `TestReviewFAckCeilingUsesReceipt` | Keep the arrival-time ceiling, stream-end timestamp and retained events. Give the manually constructed run a valid immutable boundary. Assert the stream protocol error and disconnect diagnostic in place of the store fault and terminal diagnostic. | Sprint 3 Link B and Sprint 4 Client 2, 3 and 8. |
| `src/client/internal/app/client_test.go` | `TestAppRunStoreStatus`, `appPeer.challenge` | In item 3 drop both impossible-mark rows and their peer scripts. Keep the sequence exhaustion and status mapping cases. In item 4 add the optional Hello observer for the executable fixture. Current peer behavior is preserved when the observer is absent. | Sprint 3 Link B and Sprint 4 Client 1b, 3 and 8. |
| `src/client/internal/link/receipts_test.go` | `TestStartedFromEarlierRunIsInterrupted` | Pass the fixture's previous last number to the direct `prepareRun` call. Keep the malformed-receipt and version 1 assertions. | Sprint 1 Client B, Sprint 3 Link B and Sprint 4 Client 2, 3 and 8. |
| `src/client/internal/link/revision_two_receipts_test.go` | `requireReceiptPreparation`, `TestRevisionTwoReceiptMetadata` | Pass the fixture's previous last number to the direct calls. Keep the preparation, atomic sealing and metadata rejection assertions. | Sprint 4 Client 2, 3 and 8. |
| `src/client/internal/link/revision_two_receipts_review_test.go` | `TestRevisionTwoReceiptRejectsDuplicateWatermark`, `TestRevisionTwoReceiptRejectsResultOnStarted` | Pass the fixture's previous last number to the direct calls. Keep both malformed-receipt assertions. | Sprint 4 Client 2r, 3 and 8. |
| `src/client/internal/app/revision_two_process_test.go` | `TestRevisionTwoExecutableStop` | Replace the old missing-history case with the executable recovery and enrollment controls. Change the unreadable exit assertion to zero. Retain the existing damaged-store, verified revision refusal, restart and cleanup controls. | Sprint 4 Client 3 and 8. |
| `src/client/internal/link/revision_two_store_open_review_test.go` | `TestRevisionTwoStoreOpenPermissionFailure` | Change only the runtime expectation for the actual read denial. Keep the readable but unwritable and open-only controls. | Sprint 4 Client 8. |
| `src/client/internal/link/revision_two_store_read_review_test.go` | `TestRevisionTwoStoreMetadataRead/temporary read fault` | Change the real executable metadata read-fault exit from one to zero. Keep the diagnostic and open-level classification. | Sprint 4 Client 8. |

The paired owner replaces `src/linktest/sprint_four_history_test.go` (`TestSprintFourHistoryRecovery`) in its own plan. Keep its revision 1 refusal and required corruption controls when replacing the obsolete missing and incompatible Welcome assertions. Server and browser test changes remain with their plans.

**Rollout detail**

1. Stage the generated Contract revision 2 fields before item 1. Export `LGTM_LINK_BASE` from `.foundry/go-rewrite/link-base.txt`. Use the updated revision 2 freeze commit supplied by Contract.
2. For each item, run its tests against the old behavior or the smallest compiling stub, observe the intended assertion fail, and freeze the reviewed files. After implementation, rerun the same tests and the stated mutations before integration. After the Client slice lands on top of the Contract slice, run `go test -race -count=1 -v ./src/client/...` on native Linux and macOS as an ordinary user. Then run the Client merge gate with the optional paired database variable unset, using the environment below. Require exit zero from both runs. The unmodified paired history test is gated by the database variable and cannot validate the intermediate change.
3. After Server and Website have landed, run the Go gate with real PostgreSQL and the named paired H1 to H8 tests with `bash deploy/test/with-postgres.sh -- go test -race -count=1 -v ./src/linktest`. Require the paired cases to pass with no skips. The paired test owner updates the prior frozen history test and scripted admissions before this step.

```sh
set -a
. .foundry/go-rewrite/link-base.txt
set +a
env -u LGTM_TEST_POSTGRES scripts/check-go.sh
```

When running the integrated gate with disposable PostgreSQL, run `bash deploy/test/with-postgres.sh -- scripts/check-go.sh` after exporting the same history base.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe. The history design of 2026-11-28 supplies its six decisions in table order, its three Welcome branches, client stop and receipt rules, inter-team work and checks H1 to H12. The bounded assignment supplies the five Client deliverables. The names table supplies the fields and build order. The foundation design supplies decisions D1 to D23, rules R1 to R11 and checks C1 to C13 in document order. The affected sprint 4 proof references are R5, R6, R9, R10 and R11 as named by the history design. Issues 1632 to 1635, 1701 and 1702 point 1 and build loop phase 4 are the other source items.

| Source item | Workstream | Status |
|---|---|---|
| History decisions 1 and 5. Server authority and dropping covered events. | WS-1 to WS-4. | Client applies the mark and produces current reports. Server makes the admitting decision. |
| History decisions 2 to 4. Standing, commands and notification. | WS-3. | Server and Website own the settling and display. Client proves persisted receipt and result behavior and sends the ordinary result vocabulary. |
| History decision 6. Revision 2 is amended. | WS-1. | Build on Contract fields at revision 2. |
| History Hello table, Welcome branches, fresh reports, client stops and receipts. | WS-1 to WS-4. | Implement all assigned client behavior. |
| History H1 and H2. | WS-2 to WS-4. | Store, signed peer and executable proofs here. The paired owner proves Online. |
| History H3 to H5. | WS-3. | Client proves receipt replay and result bounds. Server and the paired owner prove command and folder outcomes. |
| History H6 and H7. | WS-1 to WS-3. | Replay, inclusive endpoints, announced floor and stale retry are tested. The pair proves server mark advancement. |
| History H8. | WS-1 and WS-3. | Loss before Welcome and the post-adoption redial are tested locally. The pair proves idempotent server settlement and bell display. |
| History H9 and H10. | WS-1, WS-3 and WS-4. | Client sends consistent Hello values. Server tests refusal. Client tests terminal damage. |
| History H11 and H12. | WS-3. | Website owns the display. Server owns the failed-request data. |
| Assignment points 1, 2 and 3. | WS-1, WS-2, WS-3 and WS-4. | Announce, reconcile, retry and create a missing history after verification. |
| Assignment points 4 and 5. | WS-2 and WS-3. | Keep receipts and verify bounded results and fresh state. |
| Shared names and build order. | WS-1 to WS-4. | Keep contract field names and the independent client gate. |
| Foundation D1, D2, D4, D5, D7, D16, D17 and D19. | WS-1 to WS-4. | Keep component boundaries, Go, real PostgreSQL for paired tests, the server's held mark, proof and outbound retry. |
| Foundation D3, D6, D8 to D15, D18 and D20 to D23. | WS-1 to WS-4. | These belong to other parts or later waves. |
| Foundation R1 to R5. | WS-1 and WS-2. | Client edits its own folder and uses the reviewed contract. Database and Watch behavior belong to Server. |
| Foundation R6 to R11. | WS-1 to WS-4. | Keep stored commands, durable local numbering, screened reports and verification boundaries. The paired owner exercises the real link and database. |
| Foundation C6 to C11. | WS-1 to WS-4. | Keep enrollment, approval, liveness, authenticated peer and ordered replay for existing valid inputs. |
| Foundation C1 to C5, C12 and C13. | WS-1 to WS-4. | Stack and Server retain their own checks. The client's merge gate still runs repository-wide. |
| Sprint 4 R5, R6, R9, R10 and R11. | WS-1 to WS-4. | Preserve folder and receipt ordering, the run barrier and existing revision 1 behavior. Replace superseded history expectations in the Client suite and hand off the paired changes. |
| Issues 1632 to 1635. | WS-1 to WS-4. | Keep outbound retries and the existing proof, the public test entry points and complexity sizing. |
| Issue 1701 and issue 1702 point 1. | WS-1 to WS-4. | Implement the Client share of history reconciliation. The wire-size wording belongs to Contract. |
| Build loop phase 4. | WS-1 to WS-4. | Write tests first, freeze them, assign one writer for shared files and verify mutations for risky changes. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, “What this project is” and “Where things are”. | All build edits stay in the Go Client tree. Shared fields are requested from Contract. |
| `AGENTS.md`, “Habits”. | Reuse repository code without importing external material. Put new coordination requests in the bounded result. The orchestrator owns integration by pull request and any issue record. |
| `AGENTS.md`, “Writing style for documents”. | Use dated decisions, relative paths and no private addresses or quotations. Run the private-detail guard. |
| `.plans/epics/go-rewrite/epic.md`, “The teams, and what each may edit”. | Apply the `client/backend` scope. The plan requests the paired change from Website and the contract fields from Contract. No new dependencies. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | backend | `client/backend` | low | Adds two fields from a locked sample to an existing announcement, with a signed fixture. | Yes | Contract build. |
| 2 | backend | `client/backend` | medium | Adds one durable transaction and tests rollback and uncertain commit in the existing store. | Yes | Item 1 for the shared outbox file. |
| 3 | backend | `client/backend` | medium | Selects the existing replay or a store operation at one stream boundary, with focused receipt and reporter changes. | Yes | Items 1 and 2. |
| 4 | backend | `client/backend` | medium | Reuses file creation and separates actual read failures from other open errors, with executable tests. | Yes | Item 3 for shared files and the first Welcome proof. |

Load. One vendor supplies the planner, four test writers, four builders and four post-build reviewers. Budget up to five more seats for the plan review, for a total of 18 launches including this planner. The orchestrator checks that vendor's remaining allowance before dispatch and schedules overflow after its reset. This assignment has no live allowance reading.

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| 1 | Pending. | The orchestrator runs the review after planner self-check, as assigned. |

</details>
