# Round 2 review of go-s1-client-local — lens logic, seat logic2

- Task identifier: review-go-s1-client-local-r2-logic2
- Nonce: 2a9e74
- Model and effort: claude-sonnet-5-5, high
- Reviewed: the patches of round 1 in `plan.md` and `decisions.md` (commit 9ec34833 against fb992dc9), the register `attack/round-1-findings.md`, the Contract plan `.plans/plans/go-s0-contract/plan.md`, and its draft files under `.plans/plans/go-s0-contract/draft/`.
- I ran no code, because no Go code exists yet. Every claim below rests on the plan text, the register and the contract files, with the line named.

**Verdict: REVISE**

**Top finding:** a crash between the write of `outbox/epoch` and the write of the mark in a brand-new outbox leaves a folder that the patched recovery calls damaged, so the client refuses to start for ever until a person deletes files by hand (logic2-r2 1).

## Findings

### logic2-r2 1 — MAJOR — a new outbox that stops half-way becomes damaged for ever

- **Tied statement:** C11, item 6.
- **Evidence:** `plan.md:637` makes a new outbox in two writes, first `epoch` through `state.CreateExclusive`, then the mark 0. `plan.md:638` says that a missing mark in an outbox that holds other files gives `ErrOutboxCorrupt`, and the epoch file is such a file. `plan.md:640` says that `OpenOutbox` then fails until the owner rules (decision 48 is unanswered). `plan.md:727` starts `TestOutboxCrashAtEachWriteStep` from five existing events, and no test crashes while a new outbox is made.
- **Failure scenario:** a person runs `join` for the first time, and the machine loses power, or the service manager kills the process, between the end of the epoch write and the end of the mark write. The folder now holds `epoch`, no mark and no event file. The next `OpenOutbox` reads "epoch present, mark missing" as damage, returns `ErrOutboxCorrupt`, and `run` exits with one sentence (fold rule 7). Nothing was ever numbered, so nothing was lost, and still no repair exists except deleting `outbox/epoch` by hand. The same state results when the epoch write returns `ErrPlacedNotSynced`, which `recoverOutbox` does not describe.
- **Fix:** write down that a folder holding a valid `epoch`, no mark and no event file is a new outbox that stopped half-way. `recoverOutbox` then writes the mark 0 and goes on. This is safe, because a number is handed out only after `OpenOutbox` returns. A missing mark beside event files stays damage. Add `TestRecoverFinishesHalfMadeOutbox`, and a crash test that stops a child at each of the five steps of the epoch write and of the mark write while the outbox is new. After each stop, `OpenOutbox` must succeed with `next` 1 and one epoch. State what `OpenOutbox` returns when the epoch write gives `ErrPlacedNotSynced`.

### logic2-r2 2 — MAJOR — the epoch and `Reconcile` disagree with the Contract plan, and rule 6 names the wrong fields

- **Tied statement:** C11, item 7, the fold rule 6.
- **Evidence:** `plan.md:870` says that the client tells the server its epoch after both ends have proven themselves. The link order in `.plans/plans/go-s0-contract/draft/link.proto:71` is Hello, Challenge, Proof, Welcome, and Welcome already carries the server's number (`link.proto:231`). The Contract plan has no field for an epoch (a search for "epoch" in its plan and drafts finds none). It decides that the server raises its mark to `Hello.acked_event_number` before Welcome (`.plans/plans/go-s0-contract/plan.md:535`), and that the machine's next number is higher than `Welcome.last_event_number` (`.plans/plans/go-s0-contract/plan.md:344`, `link.proto:231-234`). `plan.md:699` and `plan.md:870` of this plan say the opposite: the client never moves its counter, and `ErrServerAhead` closes the connection. Decision 41 is `unanswered`.
- **Failure scenario:** the Contract team keeps its own rules. Plan `go-s3-client-link` then builds a client that, by rule 6, sends an epoch in a message that does not exist, never writes `Hello.acked_event_number`, and must stop on a server number that the contract tells it to accept. Truth table of the server's number against the mark M and `Next()` N: below M, the plan stops, and the contract has the server raise its mark, so the cell is empty for a server that follows the contract and permanent for a server whose number is per epoch and was lost (the server reports 0 for an epoch it does not know, and M is above 0). From M to N minus 1, both agree. At N or above, the plan stops and the contract moves the counter. Two of three cells contradict.
- **Fix:** put the epoch in `Hello` (it must arrive before Welcome, which carries the answer). Add to rule 6, and to the writers and readers of item 7, that plan `go-s3-client-link` writes `Hello.acked_event_number` from `Acked()`. Record in decision 41 the fallback for the case that the Contract team declines the epoch, and name the cell it changes: the cell at N or above. Then the Contract plan and this plan can be made to agree in one sitting.

### logic2-r2 3 — MAJOR — `ErrServerAhead` and `ErrServerBehind` have no repair and a retry without end

- **Tied statement:** C11, item 7, the fold rule 6.
- **Evidence:** `plan.md:699` makes both errors change nothing. `plan.md:870` says to close the connection, log one sentence and "retry later". No rule names who repairs the state, how often the client retries, or what the website shows. Decision 48 covers a damaged outbox only. The register (group C, `fixed another way`) counts this as closed.
- **Failure scenario:** a person restores the state folder from a backup, or reverts a virtual machine snapshot. The epoch is the same, the client's counter is behind what the server applied, and `Reconcile` returns `ErrServerAhead` on every connection. The machine reconnects, fails and reconnects. It may show as online and offline in turn, and it never sends another event. The only way out, deleting `outbox/`, is not written anywhere, and the pending events are then lost without a word. The epoch was added so that the counter never has to move, but this is exactly the case where a new epoch is safe, because fold rule 4 already makes every report safe to apply twice.
- **Fix:** add a third owner choice beside rows 48 and 49: what the client does when the server's number is above its own. Options: refuse and say which folder to delete (now), start a new epoch by itself and send the pending events again under numbers from 1, or refuse until a person runs a repair command. Whatever the choice, rule 6 must name a back-off between retries, the sentence in the log, and the website's view of the machine. Add a test that a restored folder either recovers or stops with the sentence, never silently.

### logic2-r2 4 — MAJOR — a clock that jumped forward once stops every command for ever

- **Tied statement:** R6, item 8, decision 45.
- **Evidence:** `plan.md:765` says that `now` is the later of `Now()` and the stored time, and that the stored time rises whenever `now` moves more than one minute past it. Nothing lowers it. `plan.md:777` prunes by that `now`. `TestClockHighWaterSurvivesRollback` (`plan.md:797`) tests only a rollback. The draft `.plans/plans/go-s0-contract/draft/commands.proto:14-16` lets the machine count the server's time instead.
- **Failure scenario:** a machine wakes from a snapshot, or boots with a wrong real-time clock, and `Now()` reads a day too late for one minute. `receipts/clock` records it. After the clock is corrected, `now` stays a day ahead. Every new command whose expiry is less than a day away is written as `expired` and not run, including `Restart` and `SetState`. `Prune` on the next open deletes every receipt older than `Keep` relative to the wrong time. The state does not heal. A person must know to delete `receipts/clock`, and nothing says so. Decision 49 covers a client clock behind the server's, not one ahead.
- **Fix:** say what `Now` is. If plan `go-s3-client-link` passes the contract's count of the server's time, a forward jump of the client's clock cannot reach the file, and the plan should say so in the fold. If the wall clock stays, cap the stored value at the latest `ExpiresAt` of any command seen (the mark only has to protect a command that can still arrive), and cap `now` for `Prune` the same way. Add `TestClockForwardJumpHeals`: set `Now` a day ahead for one decision, then back, and require a new command with a one-hour expiry to run.

### logic2-r2 5 — MAJOR — the failed state of the receipts has no way out, and a replay can cause it

- **Tied statement:** R6, item 8, the fold rules 5 and 8.
- **Evidence:** `plan.md:773` puts the receipts into `ErrReceiptsFailed` until they are closed and opened again. Fold rule 5 (`plan.md:869`) tells the producer of an event to reopen the outbox, and no rule says who reopens the receipts. `plan.md:769` makes a replay first rewrite the stored expiry when the arriving one is later. That write can return `ErrPlacedNotSynced`, which `plan.md:773` turns into the failed state. Fold rule 8 (`plan.md:875`) says that any error from `Begin` means the command does not run, and `Restart` and `SetState` are commands.
- **Failure scenario:** one folder sync fails during the expiry rewrite of a replay, or during any `Begin`. The receipts fail. Every later command returns `ErrReceiptsFailed`, and the owner cannot send `Restart` from the website, because `Restart` itself needs `DecisionRun`. The client stays in this state until someone restarts the service by hand. For a replay the failure is also needless, because the stored outcome is already on disk and the expiry rewrite can be tried again.
- **Fix:** add a rule to the fold that on `ErrReceiptsFailed` plan `go-s3-client-link` closes and opens the receipts at once, and refuses commands only while that fails. Treat a failed expiry rewrite of a replay as not fatal: return the stored receipt, and keep the receipts open, because the rewrite changes nothing the next arrival cannot repeat. Add a test for both.

### logic2-r2 6 — MINOR — the trust rule refuses common installs

- **Tied statement:** R9 and D19, item 5 and item 2.
- **Evidence:** `plan.md:534` refuses a program when the file, or any folder from it up to the root, has a write bit for group or others (superuser folders with the sticky bit excepted). `plan.md:294` applies the same rule to every folder above the state folder. Post-build check 7 (`plan.md:908`) installs each harness once and names no setup with group write.
- **Failure scenario:** on a Mac with an Intel Homebrew prefix, the folders `/usr/local/bin` and `/usr/local/lib/node_modules` are commonly made group-writable for the `admin` group. A person installs a harness with Homebrew or a global package manager. `trustedProgram` returns `ProblemUntrusted`, the website shows that the harness is not installed, and the person cannot tell why. A home folder with mode 0775 on a shared or network setup makes `state.Open` return `ErrParentUnsafe`, so the client does not start. I have not verified these modes on a real machine.
- **Fix:** decide the rule for a group-writable folder that the user owns and whose group holds only the user, and write a table test with the modes 0755, 0775 and 0777, owners user and root, and a sticky root folder. Add to check 7 an Intel Homebrew prefix and a global package install on a Mac.

### logic2-r2 7 — MINOR — the boundary of `Reconcile` is not tested

- **Tied statement:** C11, item 7, mutation 7.
- **Evidence:** `plan.md:711` tests `Reconcile` with a mark of 2 and events 3 to 5 pending (so `Next()` is 6) at the numbers 1, 2, 4, 6 and the largest 64-bit value. It never tests 5, the last number that was handed out.
- **Failure scenario:** an implementation that tests `serverLast >= Next()-1` for `ErrServerAhead` passes every listed case, because 4 is below 5 and 6 is above it. In production, the server's most common answer after a full delivery, the number of the last event, is refused, and every machine that sent all its events stops at each reconnection.
- **Fix:** add `Reconcile(5)` to `TestReconcileCases`, with the result nil, nothing pending, and `Next()` still 6. Add `Reconcile(3)` to pin the lower boundary of the pending range.

### logic2-r2 8 — MINOR — a damaged or unwritable `receipts/clock` is not covered

- **Tied statement:** R6, item 8.
- **Evidence:** `plan.md:765` and `plan.md:774` say how a damaged receipt file and an unreadable receipt are treated, and say nothing of a clock file that is damaged, unreadable or whose write returns `ErrPlacedNotSynced`. No test in the table of item 8 touches the file. The tests use steps of hours, so the slack of one minute is untested.
- **Failure scenario:** a half-written clock file is read as the zero time, and the protection against a clock set back silently stops. Or the write of the file fails during `Begin`, and the plan does not say whether the command runs.
- **Fix:** treat a missing file as the zero time only when no receipt exists, treat an unreadable or invalid file like an unreadable receipt (`OpenReceipts` fails), and decide that a failed clock write enters the failed state of `plan.md:773`. Add three tests, and one that sets the clock back by 30 seconds.

## Answers to the seven questions

1. **Does each patch remove its failure, and does it break something else?** The patches of groups A, B, F, G, H, I, J, L, O, P, Q, R, S, V, W and X remove their failures as written, and I found nothing that they break (I checked their text against the lines named above). Group A is partly open: the folder sync rules are right, but the recovery path they feed has finding 1. Group C moved the problem: `Ack` is safe now, `Reconcile` introduces findings 2, 3 and 7. Group D, E and R introduce findings 4, 5 and 8. Group K introduces finding 6. Group T made the missing mark a damage, which finding 1 turns against a new outbox. Groups M and N are sound for what they say. Groups U and E (the parts for the owner) are honestly open, see question 6.
2. **The `UNREVIEWED MECHANISM` places.** The epoch (findings 1, 2 and 3), `Reconcile` (findings 2, 3 and 7), `trustedProgram` (finding 6), and the clone folder inside the state folder. For the clone folder I found nothing: the folder is made by wave 2, `Open` does not touch it, and agents already run under the user who owns the key.
3. **Rejected findings.** The register lists zero rejected findings (48 fixed, 7 fixed another way, 2 for the owner). Nothing found. I checked the totals and that no row carries the outcome "rejected".
4. **Writers and readers of changed names.** Incomplete in two places. The epoch has no writer in the contract, and no message that can carry it before Welcome (finding 2). `Hello.acked_event_number` has a writer in the Contract plan and no user in this plan (finding 2). The receipt states have readers, and the outcome `interrupted` now exists in `.plans/plans/go-s0-contract/draft/commands.proto:84`, so row 46 can be closed. The names of item 6 are all consumed by item 7 (`recovered`, `readAcked`, `writeAcked`). Nothing else found.
5. **Contradictions and truth tables.** The table of `Reconcile` against the mark and the next number is in finding 2. The table of `Begin` has no empty or contradictory cell for stored states and attempts (checked with `plan.md:767-771` and `TestReplayTruthTable`), but the cell "write of the expiry fails on a replay" contradicts fold rule 8 (finding 5). The table of the clock against the receipts has the cell "forward jump" empty (finding 4). The table of the outbox folder's contents has the cell "epoch only" wrong (finding 1).
6. **Is a blocker or major of round 1 still open?** Yes, in part. Threat-r1 02 (group T) is open for the new outbox (finding 1). Architect-r1 3 and its group (group C) are open for the case of the server's number being above the client's, as the register itself says "fixed another way" (findings 2 and 3). Logic2-r1 6 and logic-r1 6 are honestly with the owner. Nothing else found.
7. **Lens questions touched by a patch.**
   - Q1 (orders, gaps, restarts): the epoch and the mark in finding 1, the number and the server's number in finding 2, the clock in findings 4 and 8.
   - Q2 (failures and repair): findings 1, 3, 5.
   - Q3 (truth tables): question 5 above.
   - Q4 (zero values): the mark 0 of a new outbox is correct, a zero `serverLast` after a lost server count is in finding 2, and the zero time of a missing clock file in finding 8.
   - Q5 (two actors): `Read`, `Ack` and `Reconcile` share one lock, and a claim stops two stores, so nothing found. The sender and the producer share only `Append`.
   - Q6 (does a mutation fail the listed tests): mutation 7 of item 7 is not caught for the boundary (finding 7). Mutations 1 to 6 of item 7 and 1 to 7 of item 8 are caught by the tests named, and I found no wrong change that passes them, except through finding 8 for the clock file.
   - Q7 (cheapest wrong implementation): for item 7, an off-by-one in `Reconcile` (finding 7). For item 8, a `clock` that is written on every call and never read passes all tests, which finding 8 also asks to close. Items 6 and 5: nothing found.
   - Q8 (a shape that makes the wrong state impossible): finding 1, where one file holding both the epoch and the mark would remove the half-made state, and finding 3, where a rotating epoch would remove the stuck state.

## Sizing

Item 7 stays medium only if the repair of findings 2 and 3 goes into plan `go-s3-client-link` and not into this item. If the new epoch with renumbering lands here, split it into the queue and the reconciliation. Item 8 stays medium. The cap of finding 4 is small.

## Post-build checks

| # | Scenario | Expected result |
|---|---|---|
| 1 | A child makes a new outbox and is killed at each of the five steps of the epoch write and of the mark write, 30 times, at random moments. | Every later `OpenOutbox` succeeds, the epoch never changes once the epoch file is on disk, and the first number is 1. |
| 2 | The state folder is restored from a copy made after 5 events, while the server holds 9 for the same epoch. | The client either starts a new epoch and the server accepts the pending events, or stops with one sentence that names the repair. It never stays silent, and it does not reconnect more than once a minute. |
| 3 | The server's database is restored to a point before the machine's mark. | The machine reconnects, the mark of the server is raised or the client stops with a sentence, and the next event is applied once. |
| 4 | The clock is set one day ahead for two minutes, then back. Then a command with a one-hour expiry arrives. | The command runs. The receipts that are younger than `Keep` stay. |
| 5 | An injected folder sync error hits the receipts during a replay and then during a new `Begin`. | The replay returns its stored outcome. After the second error the receipts reopen, and a `Restart` command from the website then runs. |
| 6 | The scan runs on a Mac with an Intel Homebrew prefix, a harness installed globally with a package manager, and a home folder of mode 0775. | The harness is found, or the sentence says exactly which folder made it untrusted. The client starts, or `ErrParentUnsafe` names the folder. |
| 7 | After a full delivery of events 1 to N, the server answers with N on reconnection. | `Reconcile(N)` succeeds, nothing is pending, and the next number is N plus 1. |

## Grade

5 out of 10. Counts: 0 blockers, 5 majors, 3 minors.
The outbox and the safe write are much stronger than in round 1, and every round 1 blocker now has a mechanism. What remains is the edge of those mechanisms: the creation of the outbox, the disagreement with the server, and the clock. All eight findings have a small fix, and finding 2 needs one decision with the Contract team.
