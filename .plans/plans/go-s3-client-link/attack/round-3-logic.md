REVISE

# Client link, round 3, logic seat

- Task identifier: review-go-s3-client-link-r3-logic. Nonce: 98e43c.
- Change attacked: `git diff 80e5857a 4a22a9dd` on `plan.md` and `decisions.md` of this plan folder, with the register `attack/round-2-findings.md`.
- Finding identifiers use the prefix `logic-r3`. Line numbers of the plan are those of `plan.md` at the current commit.

## Top finding

The patch that stops the service for good on "damaged" store errors rests on a classification that the built store does not have, because `storeOpenError` labels every unclassified open error, including permission and input-output errors, as damaged (logic-r3 1).

## Findings

### logic-r3 1. MAJOR (a repairable fault still ends the service for good). Statement D16.

- Evidence.
  - `plan.md:201` makes `ErrStoreMissing` and `ErrStoreUnsafe` retryable (status 1) and `ErrStoreDamaged` a permanent stop (status 0), "Preserve state and direct investigation before manual restart". Stack restarts only after a nonzero exit (`.plans/plans/go-s3-stack-client/plan.md:27`).
  - `src/client/internal/link/store.go:353-360` (`storeOpenError`) turns every error that is neither `ErrStoreUnsafe` nor `ErrStoreDamaged`, and is not a lock timeout, into `ErrStoreDamaged`. It is called for a failed `os.Lstat` (`store.go:121`), a failed `inspectStore` (`store.go:129`) and a failed `openExistingBolt` (`store.go:133`).
  - The plan's own repair sentence at `plan.md:201` names "the original volume, ownership and private file mode", which only describes the cases that the store labels missing or unsafe.
- Failure scenario. The state folder is on a volume that is present but not readable yet or not any more: `Lstat` returns `EACCES`, `EIO`, `ESTALE` or `ENOTDIR`. The store reports `ErrStoreDamaged`. Item 7b returns `(RunStopped, nil)`, item 8 exits 0, and the service does not start the client again. When the volume becomes readable the client stays down until a person notices. This is the failure that logic-r2 3 described, and the patch removes it only for a file that is absent or has the wrong mode. The planned post-build check withholds the file by renaming it, which yields `ErrStoreMissing` and would not reveal this.
- Fix. Item 7b owns an edit of `src/client/internal/link/store.go` and `store_test.go` (add both to its Files list). `storeOpenError` labels `ErrStoreDamaged` only for failures that a structural check of the file finds: a bad size, missing buckets, a bad version, a gap in the numbers, a bad `next` value, or a bolt format error or recovered fault. Any other operating-system error from `Lstat`, `OpenFile`, `Stat` or a read keeps its own cause and is wrapped with a new `ErrStoreUnavailable`. Item 7b maps `ErrStoreUnavailable` to `(RunStopped, err)` like `ErrStoreMissing`. Add a store test that makes `Lstat` fail with `EACCES` and expects `ErrStoreUnavailable`, and add the case to `TestRetryFailureExitsAndKeepsStore` and `TestAppRunStoreStatus`. Add to the post-build row for the store fault one run that removes read permission from the state folder.

### logic-r3 2. MAJOR (a harmless race between an acknowledgement and a send ends the service for good, and no frozen test owns it). Statement C11.

- Evidence.
  - `plan.md:161-163` (item 4): `sent` raises the ceiling only after a successful send. `ack` returns `ErrAckAhead` for a mark above the ceiling. Decision 56 gives "send completion, acknowledgement admission" to item 7b.
  - `plan.md:199` (item 7b): "Order send completion before racing acknowledgements from the current stream". No mechanism is named.
  - `plan.md:201`: an impossible mark returns `(RunStopped, nil)`, so the process exits 0 and the service stays down.
  - The only test that names the race is `TestReplayRejectsImpossibleMarks` in `replay_test.go` (`plan.md:166`: "an immediate valid acknowledgement racing send completion"). A `replay` object has no transport. A unit test can only call `sent` and `ack` in some order, and the second order is by the item's own rule an error.
  - The five tests of item 7b (`plan.md:204`) do not drive an acknowledgement that the server sends as soon as it receives an event. The post-build row at `plan.md:301` is not a frozen test.
- Failure scenario. The client sends event 5. The server commits it and sends `EventAck(5)` at once. The receiving worker of the client reads the acknowledgement before the sending worker has returned from `Send` and called `sent(5)`. The ceiling is still 4. `ack(5)` is above the ceiling, returns `ErrAckAhead`, and item 7b stops the run with exit 0. A fast server on a loopback link makes this likely. The cheapest wrong implementation, which calls `ack` straight from the receive loop, passes every listed test, because the only race test cannot reproduce it.
- Fix. Remove the race instead of ordering around it. `next()` marks the returned event as handed to the stream and raises the ceiling at that moment. `sent` is deleted, or becomes a no-op bookkeeping call with no effect on the ceiling. A failed send ends the stream, and the replay object is discarded with it, as the plan already says. `ack` still rejects a mark above the highest number handed out by `next`, so a queued but unsent mark stays impossible. Update the test: `TestReplayRejectsImpossibleMarks` checks a mark for an event handed out but not yet confirmed as valid, and a mark above the last handed-out number as `ErrAckAhead`. Add to item 7b a test `TestSessionAcceptsImmediateAck`, where a stub server acknowledges each event as soon as it arrives, under `-race` and with a send that is held after the write, and expects no stop and no `ErrAckAhead`.

### logic-r3 3. MINOR (a requested scan across a reconnect always fails, and the combined test does not say so). Statement C7.

- Evidence.
  - `plan.md:247`: a requested scan finishes failed when "discard behind newer inventory" happens. `plan.md:175` and `plan.md:200`: every approved welcome queues fresh facts and inventory, and captures are ordered by start time, so a newer commit discards older unnumbered reports.
  - `plan.md:297`: `TestLinkCommandAttemptsAcrossReconnect` "Hold a scan across reconnect and accept its result on the latest attempt". It does not say which outcome is expected.
  - `src/contracts/proto/lgtm/link/v1/commands.proto:88-90` is real: done needs the command's own event on disk, and failed names a buffer that refused the event. It does not say what a discard behind a fresher committed inventory yields, so the plan's rule is a choice and not a quotation.
- Truth table for a requested scan R and the inventory X that the approved welcome queues.

| Start order | Commit order | Result for R |
|---|---|---|
| R before X | R first | done, R numbered, X appended after |
| R before X | X first | failed, R discarded |
| X before R | R first | done, X discarded later |
| any | outbox full at R's append | failed |
| any | canceled before commit | failed |

  Row 2 is the row that a scan held across a reconnect always lands in, because the welcome starts X after R and X completes while R is held.
- Failure scenario. The combined test holds a scan, reconnects, releases the scan, and the builder writes an assertion of done, as the sentence "accept its result" suggests. It fails every run. If the builder asserts any outcome instead, the user-visible behavior is that a scan requested shortly before a reconnect shows failed although the inventory on the page is fresh, and nothing in the plan says this is intended.
- Fix. State the table above in item 6 and make `TestLinkCommandAttemptsAcrossReconnect` assert failed for the held scan and done for a second scan requested after the reconnect. State in item 5 that the refused report of a requested scan is dropped and does not become the pending automatic snapshot of its kind, so that a command answered failed never has its report delivered later.

### logic-r3 4. MINOR (the slot is taken before the receipt is read, so a command that needs no slot closes a healthy stream). Statement R6.

- Evidence. `plan.md:243`: "Before `Begin`, reserve the scan slot... A distinct scan during a busy slot closes the stream before touching its receipt." `Begin` (`src/client/internal/link/receipts.go:65-99`) answers a finished receipt, an interrupted one and an expired one without starting anything.
- Failure scenario. Scan C finished and its result was lost with the stream. The owner requests a new scan D, which is running when the machine reconnects. The server resends C and D. C is distinct from D and the slot is busy, so the stream closes before C's stored result is replayed. This repeats on each stream until D finishes (a scan takes up to the 3-second default timeout in `src/client/internal/harness/scan.go:45`), so heartbeats stop and the machine flaps for the length of the scan, in exchange for an answer that costs nothing.
- Fix. Because one process owner orders deliveries, a read-only look at the receipt followed by the reservation cannot race. Item 6 reads the receipt first through a new read-only `Store.Peek(c Command) (Decision, []byte, error)` that never writes. Only when it reports that a start is possible does the owner reserve the slot, and then call `Begin`. Add one cell to `TestCommandBusySlotWritesNoReceipt`: a busy slot plus a finished, an expired and an interrupted distinct scan answer without closing the stream. If the builder prefers no new method, state in the plan that the stream closes only when `Begin` would return `DecisionRun`, and name how that is known.

### logic-r3 5. MINOR (a restart that arrives while a scan runs has no stated order). Statement R6.

- Evidence. The slot of `plan.md:243` covers scans only. `plan.md:199` says "settle the process-owned scan before closing the store". `plan.md:246` says every admitted command persists its outcome "including on controlled shutdown". Item 6 does not say whether an accepted restart waits for the running scan or cancels it.
- Failure scenario. Scan A runs, restart R arrives and is accepted, `RunRestart` is returned. If A is canceled, its receipt becomes failed (row "canceled" above) and the retry after the restart answers failed for a scan that never had a chance. If A is awaited, `RunRestart` is delayed by up to the scan timeout, with the two-second result send budget of `plan.md:246` unclear. Both are acceptable, but the builder has to guess, and `TestCommandRestartAndRecoveredReceipt` does not distinguish them.
- Fix. Add one sentence to item 6: an accepted restart lets the running scan finish for up to the scan timeout, then cancels it. A canceled scan finishes interrupted, not failed, because the machine stopped it and not the scan. Add the case to `TestCommandRestartAndRecoveredReceipt`. If the owner prefers failed, state that choice.

## Answers to the questions of the task file

1. Does each patch remove its failure, as written, and does it break anything?
   - architect-r2 1 and logic-r2 1 (receipt writer): removed. Item 6 owns `receipts.go`, the cases are listed, and `Begin` at `receipts.go:65-112` has the same decisions as the plan's table. Nothing broken. One consequence: `Decline` runs `pruneReceipts` after its own `Put`, so a decline of a command more than 24 hours past expiry on the machine clock is written and removed in one transaction. It still replies correctly and a retry declines again, so this is consistent, but fixtures for the reopen test must use an expiry inside the window.
   - architect-r2 2 (slot before receipt): removes the stranded running receipt, and creates logic-r3 4.
   - logic-r2 2 (scan with no result): removed, but see logic-r3 3 for the reachable table.
   - logic-r2 5 (decline prune and failed commit): removed. `s.update` sets `s.failed` on a failed commit (`store.go:394-399`), so later writes are fenced, as the plan says.
   - architect-r2 3, logic-r2 3, architect-r2 4 (exit pair): removed for missing and unsafe files only. Open: logic-r3 1.
   - architect-r2 5 and logic-r2 4 (clock limit): the text now names both outcomes. I checked both. A 2-hour fast clock prunes a finished receipt 23 hours after expiry, and the retry then reaches `Begin` with an absent receipt and a bound at or after expiry, so it declines expired. A 30-hour fast clock prunes it one hour before expiry, and a retry with a bound before expiry gets `DecisionRun`. Both match `receipts.go:65-99` and `Finish`'s cutoff.
   - architect-r2 6 (receive size): the arithmetic is consistent. I rebuilt the largest planned welcome (state with a 64-byte name and a ten-byte revision about 81 bytes, timestamp about 14, link revision and last event number about 13, token 45, wrapper about 3) and found about 156 to 161 bytes, so 64 kibibytes is about 400 times that. Nothing found.
   - architect-r2 7 and 8 (7a, 7b split, replay interface): the names exist in both places. The ownership of acknowledgement ordering moved to item 7b, but its frozen test did not (logic-r3 2).
   - logic-r2 6 (C10 fixture): see question 3.
2. `UNREVIEWED MECHANISM` places, three in the patched text.
   - Publisher and requested scan (`plan.md:175`, `plan.md:247`): attacked with the lens. Findings logic-r3 3 and 5. Counters and positions are fine: captures are ordered by start, numbers come from the store, and a duplicate cannot arise from `Append` because ambiguous failures are not repeated.
   - Store-open mapping (`plan.md:201`): logic-r3 1.
   - Slot reservation and `Decline` are not marked, but they are new designs: logic-r3 4 and the first answer above.
3. Findings with the outcome "rejected": the register has none. It has three "fixed another way" and one "post-build check". I checked the evidence of each. `commands.proto:88-90` says what the plan cites, but only for done, so the rule for a discard is a plan choice (logic-r3 3). `.plans/plans/go-s3-stack-client/plan.md:173` (row 2e) owns the move from `XDG_STATE_HOME` to `LGTM_CLIENT_STATE_DIR`, and `deploy/test/checks.sh:219` and `:233` still use `XDG_STATE_HOME`, so the gate and the post-build row for C10 are real and have an owner. The exit-status guidance for the installer has an anchor in Stack step 17, which prints the journal hint when the service is not active after 10 seconds, so the post-build row for a damaged store has a reader.
4. Names added or changed. `newReplay`, `next`, `sent`, `ack`: writer item 4, reader item 7b, both stated. `encodeReport` is read by item 5 and `decodeReport` by item 4. The sentence "Item 5 reads the payload format through `encodeReport`" does not say who writes the encoder. Item 4 has the file and the signature, so I read it as item 4. A line "writer: item 4, reader: item 5" would settle it. `scanHarnesses(ctx, requested bool)`: writer item 5, readers item 6 and item 7b, stated. `Store.Decline`: writer item 6, reader item 6, tested in item 6. `RunOutcome` pair: writer 7b, readers item 8 and Stack, stated, and the exit-0 reading depends on logic-r3 1. The removed phrase "a durable append operation" from item 5 had no other reader.
5. Interactions. Built truth tables for the scan result (logic-r3 3) and for the exit pair. The exit pair table is complete and consistent between items 7b, 8, 9 and the traceability row 3c, except that `ErrStoreDamaged` also covers errors that are not damage (logic-r3 1). The pair of `Decline` and `Finish`'s pruning does not conflict. Decision rows 52 to 55 are marked unanswered while the plan text states them as final, which is the convention of this plan and not a contradiction.
6. Open blockers or majors of round 2 that the register calls fixed: architect-r2 3 and logic-r2 3 are only partly fixed (logic-r3 1). No blocker.
7. Lens questions touched by the patches: see below.

## Answers to the lens questions

1. Sequences and ties. `sent`, the send ceiling and the welcome mark: the tie at `Next()` is rejected by `Store.Ack` (`outbox.go:77-81`), a gap ends one stream, and a repeat of a lower mark does nothing. A wrap needs 2^64 events, and `ErrSequenceExhausted` stops the run. The one defect is the ordering of `sent` against an acknowledgement (logic-r3 2).
2. Failures. The failed decline commit, the busy slot, the lost reply and the fast clock now have stated outcomes. The failed store open does not have a correct repair for non-file errors (logic-r3 1). A clock read that fails or goes backward now durably records expired through `Decline`. This matches the contract, because the server records the same result as final.
3. Truth tables: scan result (logic-r3 3), exit pair (consistent after logic-r3 1), `Begin` result against the caller's action (complete), slot against receipt state (logic-r3 4).
4. Zero values. A zero `ExpiresAt` or `now` returns `ErrInvalidCommand` in `Begin`, and the plan says `Decline` shares the check. An empty result byte slice for `Decline` is not addressed. The plan always passes one byte. Nothing else found.
5. Two actors on one row. The process owner serializes deliveries, finishes and slot changes. The same identifier's second attempt follows its receipt. A distinct restart against a running scan has no stated order (logic-r3 5).
6. Risky items, the named mutation. Item 4: "allow an acknowledgement above the sent ceiling to reach `Store.Ack`" fails `TestReplayRejectsImpossibleMarks`. Item 6: "omit the decline write" fails `TestDeclineDecisions`. "Admit a second scan before it has a slot" fails `TestCommandBusySlotWritesNoReceipt`. A wrong change that the listed tests miss: calling `ack` straight from the receive loop (logic-r3 2).
7. Cheapest wrong implementation that passes every listed test: for item 7b, an acknowledgement handled in the receive loop with no ordering. For item 6, closing the stream whenever the slot is busy, even for a replay (logic-r3 4), passes `TestCommandBusySlotWritesNoReceipt` as written.
8. A shape that makes a wrong state impossible: counting the in-flight event in the ceiling (logic-r3 2), and one read-only receipt look before the slot (logic-r3 4).

## Post-build checks

- Remove read permission from the state folder of an installed client, then restore it. Expected: status 1 and retries, then online. A client that exits 0 shows logic-r3 1 is open.
- A stub server that acknowledges each event at once, run 200 times under `-race` with a loopback link. Expected: no `ErrAckAhead`, no stop, and every event removed from `client.db` in order.
- Hold a scan, reconnect, release it. Expected: the outcome that the plan's table states, and a fresh inventory on the page.
- Send a distinct finished scan while another scan runs. Expected: one answer with the stored result and no stream close.
- Accept a restart while a scan runs. Expected: the order the plan states, one restart, and the scan's receipt answered the same way after relaunch.
- Reopen a store after a decline of a command whose expiry is more than 24 hours old on the machine clock. Expected: expired is answered each time, no receipt remains, and no error.

## Grade

6.5 out of 10. Blockers 0, majors 2, minors 3. The round-2 patches fix the owner of the receipt writer, the decline write, the receive budget and the clock limit with checked evidence. The two majors are consequences of the exit-0 stop that the patches introduced, each caused by a gap between a named rule and the code or the tests that must hold it.
