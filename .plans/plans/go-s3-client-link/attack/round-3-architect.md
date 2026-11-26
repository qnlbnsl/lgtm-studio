Verdict: REVISE

Top finding: Item 7b and decision 54 exit with status 0 for a damaged store, exhausted numbering and an impossible acknowledgement mark, and the Stack service reads status 0 as removal, so no operator-facing text, check or owner exists for these three stops (architect-r3 1).

# Round 3 architect report on the plan go-s3-client-link

Task identifier: review-go-s3-client-link-r3-architect. Nonce: 95df0d. Lens: architect. Patch range: `80e5857a` to `4a22a9dd` of `plan.md` and `decisions.md`.

The patches fix the round-2 majors on their own terms. The store change now has an owner, the busy slot is reserved before the start write, and the replay interface has an owner. The new defects sit in the interaction of the exit-status mapping with the Stack plan, and in the busy-slot rule when it meets the other receipt cases.

## Findings

### architect-r3 1. MAJOR (a permanently stopped client looks like a removed one, and nothing tells the operator which)

- Statement: D16 (local fault policy), reached through decision 54 and item 7b.
- Evidence:
  - `.plans/plans/go-s3-client-link/plan.md:201` returns `(RunStopped, nil)` for damage, exhausted numbering and an impossible mark.
  - `.plans/plans/go-s3-client-link/plan.md:262` maps that pair to status 0.
  - `.plans/plans/go-s3-client-link/decisions.md:61` says Stack must distinguish these exits from removal in its guidance.
  - `.plans/plans/go-s3-stack-client/plan.md:27`, `:99` and `:101` make the service restart on every status except 0, and read status 0 as a stop for good.
  - `.plans/plans/go-s3-stack-client/plan.md:293` and `:303` test only the removal case, `live_removed_machine_stays_stopped`.
  - A search of the Stack plan for damaged, exhausted or impossible-mark guidance finds nothing.
- Failure scenario: An owner restores `client.db` from a volume snapshot that is older than the server's record. The server's `Welcome.last_event_number` is then ahead of the client's next number, so the client logs an impossible mark and exits 0. The service manager shows the unit as `inactive (dead)` with a clean exit, which is exactly what a removed machine looks like. The status cannot tell the two apart, so decision 54's request that Stack "distinguish these exits" cannot be met by Stack alone. The owner sees the machine offline and has no instruction to enroll in a fresh folder. The Stack plan has no step, text or test that mentions it, so the obligation has no owner.
- Prescribed fix:
  1. In item 7b, before returning any `(RunStopped, nil)`, log one JSON line `msg=link_stopped` with a fixed `code` of `removed`, `canceled`, `store_damaged`, `numbering_exhausted` or `mark_impossible`, plus fixed guidance text per code. Add the code check to `TestRetryFailureExitsAndKeepsStore` and `TestRetryStopsForVerifiedRemoval`.
  2. In the Stack Client plan, add one check, `live_damaged_store_stays_stopped`, that replaces `client.db` with a file of the wrong format, reads `code=store_damaged` from the unit's journal and asserts the unit is `inactive` with no restart after 15 seconds. Add a sentence to the installer that, on a rerun with the unit inactive, prints `read journalctl --user -u lgtm-client.service` and names the codes that need a fresh folder.
  3. Change decision 54 so its closing sentence names these two additions as the writer and the reader, instead of asking Stack to distinguish exits it cannot see.

### architect-r3 2. MAJOR (the busy-slot rule is stated only for a distinct scan and breaks other receipt cases)

- Statement: R6 (one durable receipt per command), reached through decision 52 and item 6.
- Evidence:
  - `.plans/plans/go-s3-client-link/plan.md:243` reserves the slot before `Begin`, and closes the stream when a distinct scan finds it busy.
  - `.plans/plans/go-s3-client-link/plan.md:244` lets `Begin` return replay and expired decisions that need no slot.
  - `.plans/plans/go-s3-client-link/plan.md:262` and `:246` say a restart settles the running scan and says nothing about whether a restart or a state change takes the slot.
  - `.plans/plans/go-s3-client-link/plan.md:251` tests only `TestCommandBusySlotWritesNoReceipt` for B during A.
  - `.plans/plans/go-s3-server-machines/plan.md:472` raises the attempt of every pending command on each new stream.
  - `src/client/internal/link/receipts.go` has `Begin` as the only way to read a receipt, and it writes `started` for a new command, so the slot cannot be reserved after looking.
- Failure scenario: Scan A is running and the server resends B, a distinct scan whose finished receipt already exists. The rule reserves the slot for B first, finds it busy and closes the stream, although B needs only a replay. The same happens for an expired B that needs only a decline. A removal or restart command that the server batches behind B waits for the reconnect cadence of item 7a as well. Each new stream raises every pending attempt and, per item 7b, queues fresh facts and inventory on the approved welcome, so one slow scan produces several streams, several attempt increments and several report pairs.

  Truth table of the rule as written:

  | Slot | Command | Receipt | Result as written | Needed |
  |---|---|---|---|---|
  | free | scan B | absent, valid | reserve, run | same |
  | free | scan B | finished | reserve, replay, release | same |
  | busy with A | same id as A | started, live | follow the receipt | same |
  | busy with A | distinct scan B | absent, valid | close the stream | wait, no churn |
  | busy with A | distinct scan B | finished | close the stream | replay at once |
  | busy with A | distinct scan B | absent, expired | close the stream | decline at once |
  | busy with A | restart or state change | any | not stated | not stated |

- Prescribed fix: Replace the close with a first-in, first-out queue of identifiers inside `commandHandler`, bounded by the server's batch of 64. Do not touch a receipt for a queued identifier. When the slot frees, call `Begin` for the head, which then handles replay, expiry and a new run with no extra rule. Close the stream only when the queue is full. State in item 6 that a restart, a state change and an unsupported action do not take the slot and run through `Begin` at once, and that a restart settles the running scan first. Change `TestCommandBusySlotWritesNoReceipt` to assert that B and a finished-receipt C wait on the same stream, that the stream stays open, that no attempt grows, and that a removal queued after B is answered while A still runs. Update decision 52 to say "queues" instead of "closes".

### architect-r3 3. MINOR (a forgotten return becomes a permanent stop)

- Statement: D16.
- Evidence: `.plans/plans/go-s3-client-link/plan.md:198` defines `RunStopped=0` and `RunRestart=1`. `.plans/plans/go-s3-client-link/plan.md:262` maps `(RunStopped, nil)` to status 0, `(RunRestart, nil)` to 75 and any error to 1.
- Failure scenario: The zero value of `RunOutcome` is `RunStopped`. A path in `supervise` that returns `(0, nil)` by mistake, for example after a recovered panic or an empty select, exits 0, which the service never restarts. A builder who writes `os.Exit(int(outcome))` also turns `RunRestart` into status 1 and `RunStopped` into status 0, which conflicts with the three-way mapping.
- Prescribed fix: Declare `RunOutcome` so that its zero value is invalid, with `RunStopped = 1` and `RunRestart = 2`. In item 8, treat any other value with a nil error as status 1 and log it. Add one case for the zero value to `TestAppRunStoreStatus`.

### architect-r3 4. MINOR (the reply-limit budget depends on a contract change that no item owns)

- Statement: R9 (budgets come from the producer's limits).
- Evidence: `.plans/plans/go-s3-client-link/decisions.md:62` says "Contract must document the field bounds at the freeze". `.plans/plans/go-s3-client-link/plan.md:103` and `:152` build the 64 KiB cap and the 161-byte welcome on those bounds. `src/contracts/proto/lgtm/link/v1/link.proto:254` and `:256` document `connection_token` without a length. `.plans/plans/go-s0-contract/plan.md` is `status: done` and has no item for this. The link is not frozen yet, so a change is still an ordinary Contract work item (`.plans/plans/go-s0-contract/plan.md:117`).
- Failure scenario: Nobody adds the bounds. The server later widens the standing name or the token, the welcome grows past the cap, and the client closes every stream as an invalid frame. The one test in item 3 builds its maximal welcome from numbers in this plan, so it cannot notice a producer change.
- Prescribed fix: Add one line to item 3's needs that names the owner, the Server Machines team, and the file, `src/contracts/proto/lgtm/link/v1/link.proto`. The comment on `connection_token` states 43 bytes and the comment on the name field states 64 bytes. Make the item 3 test read both limits from one shared Go constant of the server's limits package, not from the plan's text.

### architect-r3 5. MINOR (unlisted store errors have no exit)

- Statement: D16.
- Evidence: `.plans/plans/go-s3-client-link/plan.md:201` lists missing and unsafe files, damage, exhausted numbering, an impossible mark and a failed commit. `src/client/internal/link/store.go` maps an open timeout to `state.ErrStoreOpen`, and `src/client/internal/link/outbox.go:55` returns raw errors from the database read.
- Failure scenario: A second process holds the store and `OpenStore` returns `state.ErrStoreOpen`, or a read inside the session fails with a database error. The fault table does not list either, so the builder picks a status at will. A guess of 0 stops the service for good over a transient lock.
- Prescribed fix: Add one default row to item 7b: any store error not named in the table returns `(RunStopped, err)`, which exits 1 and retries. Add `state.ErrStoreOpen` and a raw read error as two cases to `TestRetryFailureExitsAndKeepsStore`.

## Answers to the questions of the lens file

1. Claims about existing code. Checked at the current commit: `Store.Begin`, `Store.Finish`, `pruneReceipts` and `receiptAge` in `src/client/internal/link/receipts.go`, `Append`, `Read`, `Ack` and `ErrAckAhead` in `src/client/internal/link/outbox.go`, the failure fence in `src/client/internal/link/store.go`, the proto messages, and the pins of `golang.org/x/sys` v0.50.0 and `golang.org/x/net` v0.63.0 in `go.mod` and the module cache. All hold. The plan's `Store.Decline` is correctly marked as new. Nothing found.
2. Shared names, writers and readers. `Store.Decline` has one writer, item 6, and one reader, `commandHandler`; its tests are named. `RunOutcome` has a writer in item 7b and a reader in item 8, but the zero-value defect is finding 3. The exit status has a writer in item 8 and an unowned reader in Stack for the stop codes (finding 1). The field bounds have no writer (finding 4).
3. Placement. The decline write lives in the client store, the status mapping in the app, the service reaction in Stack. Finding 1 is a placement gap, because the guidance needs a Stack-side owner. Otherwise nothing found.
4. R1 to R11. R6 is touched by findings 2, R9 by finding 4, and D16 by findings 1, 3 and 5. Nothing found for R1 to R5, R7, R8, R10 and R11 in the patched text.
5. Epic and neighbouring plans. The epic's rule that the service restarts on every status except 0 matches Stack plan line 27. The unmatched part is the three new status-0 stops (finding 1). The 64 KiB cap matches the server's `MaxMessageBytes` at `.plans/plans/go-s3-server-machines/plan.md:136`.
6. Merging alone. Item 6 now owns its store method and tests, so it merges with a passing full check. Items 6 and 7a run beside each other without a broken state. Item 8 needs 7b's exit pair, which the plan lists. Nothing found.
7. Room for wave 2. The queue in finding 2 and the stop codes in finding 1 add no wave 2 behavior. Nothing found.
8. Added dependencies. `golang.org/x/sys` v0.50.0 is already in `go.mod` as an indirect requirement and in the module cache, and matches the Sprint 2 sign-in plan. The first direct import only removes the indirect comment. Nothing found.

## Answers to the questions of the task file

1. Each patch against its finding.
   - Item 6 owns the store change and tests (architect-r2 1, logic-r2 1, logic-r2 5, architect-r2 5, logic-r2 4): the failure is removed. The new test list covers every receipt case. Nothing broken.
   - Reserving the slot before `Begin` (architect-r2 2): the stranded running receipt is removed, but the rule breaks the replay and expiry cases (finding 2).
   - Exit mapping (architect-r2 3, logic-r2 3, architect-r2 4): the suppressed recovery is removed for missing, unsafe and failed-commit cases, but the status-0 stops lose their guidance (findings 1, 3 and 5).
   - Replay and frame priority (architect-r2 8): item 4 owns the cursor and item 7b the priority. Removed. Nothing broken.
   - Superseded scan (logic-r2 2): the durable failed result is specified and tested. Removed.
   - Reply cap (architect-r2 6): the budget now rests on a field-bound comment that nobody writes (finding 4).
   - Item names (architect-r2 7): removed.
2. `UNREVIEWED MECHANISM` at plan lines 201 and 247. Line 201 is attacked in findings 1, 3 and 5. Line 247 (a scan that loses its report finishes failed, an ambiguous commit tears down the run) is sound: a failed outcome for a superseded scan matches `commands.proto:88`, and a torn-down run maps to exit 1 through the failed-commit row. Nothing further found.
3. Rejected findings. No finding was rejected in round 2, so nothing to check.
4. Names that others build against. `Store.Decline`, `RunOutcome` and the `msg=link_refused` codes were checked. The list of writers and readers is complete except for the stop codes and the field bounds (findings 1 and 4).
5. Contradictions and truth tables. The slot rule against decision 22's "a newer attempt joins the running command" and against `Begin`'s replay cases is built as a table in finding 2. The pruning examples at plan line 248 agree with `receipts.go` (a clock two hours fast prunes after 22 hours, a clock 30 hours fast prunes an unexpired receipt). Exit status 0 for a damaged store contradicts decision 54's own request to separate it from removal (finding 1).
6. Round-2 blockers or majors marked fixed but open. The register has no blocker. Of the six majors, architect-r2 1, logic-r2 1, logic-r2 2, architect-r2 3 and logic-r2 3 are closed. architect-r2 2 is closed for its own scenario but reopens as finding 2 for the cases around it.
7. Lens questions touched by the patches. Questions 1, 2, 3, 4, 5 and 6 are answered above for the patched text.

## Post-build checks

1. Restore `client.db` from an older copy so the server's last event number is ahead, then start the service. Expected: one `msg=link_stopped` line with `code=mark_impossible`, the unit `inactive`, no restart after 15 seconds, and the Stack text pointing to a fresh enrollment.
2. Corrupt `client.db` and start the service. Expected: `code=store_damaged`, status 0, folder preserved, unit inactive.
3. Send scan A, then a finished-receipt scan B and an expired scan C on one stream. Expected: B is replayed and C is declined as soon as the slot frees, with the stream open and no attempt growth, and a removal sent behind them is answered while A still runs.
4. Remove write access to `client.db` and start the service. Expected: status 1, one error line per start, a restart after 5 seconds, and recovery with no manual step once access returns.
5. Hold the store lock from a second process. Expected: status 1 through the default row, not status 0.
6. Build the maximal welcome from the server's limits constant. Expected: it decodes under the 64 KiB cap, and the test fails if the server's limit grows beyond it.

## Grade

Grade: 6 out of 10. Blockers: 0. Majors: 2. Minors: 3.
