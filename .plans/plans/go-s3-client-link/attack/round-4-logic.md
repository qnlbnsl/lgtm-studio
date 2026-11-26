REVISE

# Client link, round 4, logic seat

- Task identifier: review-go-s3-client-link-r4-logic. Nonce: b9f89e.
- Change attacked: `git diff 4a22a9dd 42be50c9` on `plan.md` and `decisions.md` of this plan folder, with the register `attack/round-3-findings.md`.
- Finding identifiers use the prefix `logic-r4`. Line numbers of the plan are those of `plan.md` at the current commit. Only blockers and majors are reported, as round 4 requires.

## Top finding

The permanent stop with exit status 0 has four causes that look the same from outside, and the plan names no diagnostic for two of them, so the post-build check that was meant to separate them has nothing to read (logic-r4 1).

## Findings

### logic-r4 1. MAJOR (a stop for good on a local fault cannot be told from removal, and its check has no writer). Statement D16.

- Evidence.
  - `plan.md:199` gives `(RunStopped, nil)` to exhausted numbering and to an impossible mark, "after logging". It names no message and no code for either. Item 8 (`plan.md:260`) maps that pair to exit 0 and the invalid pair to a logged exit 1. A verified removal and a cancellation return the same pair.
  - `plan.md:298` (post-build row "Installed service with a store fault") says "The installer points to logs, where the local fault and verified removal have distinct diagnostics". Only the removal diagnostic exists: `msg=link_refused` with `code=unknown_machine` (`plan.md:198`). The local-fault diagnostic is unnamed in items 7b and 8, and `TestRetryFailureExitsAndKeepsStore` (`plan.md:202`) asserts "one diagnostic" without saying which.
  - The register's reason for architect-r3 1 is "The installer already points to the journal". The Stack plan points to the journal only when the unit is not active ten seconds after the install (`.plans/plans/go-s3-stack-client/plan.md:210`). Its one test of a stop for good is `live_removed_machine_stays_stopped` (`.plans/plans/go-s3-stack-client/plan.md:292`), which covers removal only. `deploy/install-client.sh` does not exist yet, so no builder has fixed its text.
  - Decision row 25 (`decisions.md:32`) already says an impossible mark "stops the run with a local diagnostic". That sentence has no matching text in an item.
- Failure scenario. The state folder of a machine is restored from a snapshot. The welcome mark is above the client's next number, `newReplay` returns `ErrAckAhead`, and the process exits 0. The service manager shows an inactive unit that succeeded, the page shows the machine offline, and nothing in the page, the installer text or a test separates this from a removal or a deliberate stop. The owner restarts the unit, and the same mark stops it again. The builder of item 7b may log any line, because no frozen test names one.
- Fix.
  - Item 7b names one error line for each local stop, `msg=link_stopped` with `code=sequence_exhausted` or `code=impossible_ack`, and a fixed sentence that says the machine's own events need an owner's decision before a restart. Cancellation logs nothing new.
  - `TestRetryFailureExitsAndKeepsStore` asserts that line and code for both causes. `TestAppRunStoreStatus` asserts that the line is present when item 8 returns 0 for them.
  - The Stack plan owns a line in its installer's final text, "If the unit stops with status 0, read the journal for `link_stopped` or `link_refused`", and one live check that injects an impossible mark and greps the journal for the code. Name that row in the Stack plan's traceability. Change the register's outcome from post-build check to fixed.
  - Row 25 and the post-build row use the same two codes.

## Answers to the questions of the task file

1. Does each patch remove the failure that its finding described, and does it break anything?
   - logic-r3 2 and architect-r3 (ceiling at handoff). Removed. The ceiling rises when `next` hands the event over, so a reply that arrives before `Send` returns is at or below the ceiling. A reply for an event that was not handed over stays above it, and `ErrAckAhead` fires before `Store.Ack` runs (`src/client/internal/link/outbox.go`, `Ack` refuses `n >= next`). A failed send discards the replay, and the next welcome sets the mark again, so an ambiguous partial write cannot lose an event. I traced the repeat-reply rule of `events.proto:19-20`: because the server reads the welcome mark after the older stream stops being current (`link.proto:104`), a repeat reply cannot exceed what the new stream has handed out. Nothing broken.
   - architect-r3 2 and logic-r3 4 (busy slot). Removed for existing receipts. `Begin(c, c.ExpiresAt)` reads the receipt first inside `db.View` and returns `DecisionExpired` for an absent receipt before any write (`src/client/internal/link/receipts.go:78-100`), so the probe is read-only. The table is complete: finished gives replay, a started receipt in memory gives running, a started receipt not in memory gives interrupted, absent gives the plan's decision from the real bound. One naming gap: `TestCommandBusySlotWritesNoReceipt` lists "expired" distinct scans, and an absent expired scan is declined, which writes a receipt (`plan.md:243`). The test must say whether its expired case holds an existing receipt. That is a test detail, not a failure of the rule.
   - logic-r3 5 (restart settles the scan). Removed. The order is stated, and the test asserts failed before the restart commits. Choosing failed over interrupted is the plan's own choice and consistent with "cancellation before commit finishes failed" (`plan.md:245`).
   - logic-r3 3 (held scan). The table of logic-r3 3 row 2 now matches the combined test of `plan.md:297`, which expects failed and then done for a later scan. Scans are not serialized (`src/client/internal/harness/scan.go` holds only a wait group), so a held scan cannot block the newer inventory that the test waits for.
   - logic-r3 1 and architect-r3 3 and 5 (exit pairs). The fault of logic-r3 1 is removed: every open error returns an error and exit 1, and the unit restarts after five seconds (`.plans/plans/go-s3-stack-client/plan.md:99`, `StartLimitIntervalSec=0`). Zero is invalid and exits 1. The remaining gap is logic-r4 1.
   - architect-r3 4 (receive cap). The arithmetic of round 3 still holds. Nothing found.
2. `UNREVIEWED MECHANISM` places, attacked with the lens.
   - Exit pair (`plan.md:199`). I built the table below and found logic-r4 1.
   - Busy slot (`plan.md:243`). I checked a flap behind a running scan. The server resends the running scan first (it answers running on the open stream) and then the distinct scan closes the stream again. A scan lasts at most the 3-second scan timeout, and the backoff does not reset for streams shorter than 30 seconds, so the flap lasts a few seconds and ends. The queued inventory of each welcome falls under the full-buffer rule. Nothing to report.
   - Restart and scan (`plan.md:246`). The owner serializes the restart and the settle, and the result of the scan is persisted before the restart result. Nothing to report.
   - Scan outcome (`plan.md:245`). Agrees with the table of logic-r3 3.
3. Rejected outcomes. The register has none. I checked its post-build outcome for architect-r3 1: the evidence is only partly real (logic-r4 1).
4. Names added or changed.
   - `newReplay`, `next`, `ack`: writer item 4, reader item 7b, stated. `sent` is removed, and no text reads it any more.
   - `Store.Decline`: writer item 6 in `receipts.go`, reader item 6 in `commands.go`. Complete.
   - `RunOutcome` pair: writer 7b, readers item 8 and the Stack plan. Complete except for the log line of logic-r4 1.
   - `msg=link_stopped` does not exist yet, so its writer and readers are the fix of logic-r4 1.
5. Interactions.

   Exit pair, as the plan states it.

   | Cause | Pair | Exit | Service afterwards | Distinct diagnostic named |
   |---|---|---|---|---|
   | Cancellation | `(RunStopped, nil)` | 0 | stopped | not needed |
   | Verified removal | `(RunStopped, nil)` | 0 | stopped for good | yes, `link_refused` |
   | Exhausted numbering | `(RunStopped, nil)` | 0 | stopped for good | no |
   | Impossible mark, at welcome or live | `(RunStopped, nil)` | 0 | stopped for good | no |
   | Any open error, store read or failed commit | `(RunStopped, err)` | 1 | restarts in 5 seconds | yes, the error line |
   | Accepted restart | `(RunRestart, nil)` | 75 | restarts | yes, `link_restart_requested` |
   | Zero outcome | invalid | 1 | restarts | yes, item 8 logs it |

   The stop for good is right for an impossible mark: a client that kept running would reuse numbers that the server counts as repeats and lose events silently. The defect is only visibility.

   Stale text, not counted as a finding. The Risks row at `plan.md:210` and decision row 25 (`decisions.md:32`) still say live acknowledgements are bounded by "successful writes". Row 58 and item 4 now say handoff. The frozen test catches the wrong reading, but both lines should read "handoff".
6. Open blockers or majors of round 3 that the register calls fixed: none that the register calls fixed. The major that it defers, architect-r3 1, is still open (logic-r4 1).
7. Lens questions touched by the patches: see below.

## Answers to the lens questions

1. Sequences and ties. The welcome mark equal to the next number is refused, the mark equal to the next number minus one is accepted, and a lower repeat mark does nothing. A reply at the ceiling is accepted, and a reply one above it is refused. No gap, duplicate or wrap problem found. Numbers wrap only after 2^64 events, and that case is a stop.
2. Failures. A failed send, an ambiguous commit, a failed decline commit and an open error each have a stated result and a repair path, except that the local-fault stop has no named diagnostic (logic-r4 1).
3. Truth tables. The exit pair table above. The busy slot against the receipt state, which is complete. The scan result table of round 3, which matches the plan.
4. Zero values. A zero outcome exits 1 and logs. A zero acknowledgement mark is harmless. A zero expiry returns `ErrInvalidCommand` in `Begin`, and `Decline` shares the check.
5. Two actors. The sole sender and the owner of acknowledgements share the ceiling, and the plan makes the owner admit both under `-race`. Restart and a running scan are ordered by the owner.
6. Risky mutations. "Raise the ceiling after send returns" fails `TestSessionAcceptsImmediateAck`. "Let a mark above the ceiling reach `Store.Ack`" fails `TestReplayRejectsImpossibleMarks`. "Return `(RunStopped, nil)` for an open error" fails `TestRetryFailureExitsAndKeepsStore`. A mutation that no test catches: log any line for a local stop, or none (logic-r4 1).
7. Cheapest wrong implementation that passes. For the slot, closing the stream for every distinct scan while busy still fails the finished, interrupted and expired cases. For the exit pair, an exit 0 with no log passes every listed test.
8. A shape that makes the wrong state impossible. One typed stop reason in `RunOutcome`, instead of an unlabeled pair, would make a silent stop impossible to build.

## Post-build checks

- Inject an impossible mark on an intact fixture of an installed client. Expect exit 0, no restart after 15 seconds, and one `link_stopped` line with `code=impossible_ack` in the journal.
- Make `client.db` unreadable to the service user, then restore access. Expect exit 1, retries every five seconds, and recovery. A client that exits 0 shows a regression of logic-r3 1.
- Run 200 loopback sessions under `-race` with a server that acknowledges each event at once. Expect no `ErrAckAhead` and every event removed in order.
- Hold a scan across a reconnect, wait for the new inventory, release it. Expect failed on the latest attempt and done for the next scan.
- Send a distinct finished scan while another scan runs. Expect one answer with the stored result and the stream still open.
- Accept a restart while a scan runs. Expect the scan receipt failed before restart done, one restart, and both answered the same way after relaunch.

## Grade

7.5 out of 10. Blockers 0, majors 1, minors 0 reported (one stale-text note is listed in answer 5 and not counted). The round-3 patches remove the failures they were written for, and the busy-slot probe and the handoff ceiling hold under my tracing. The one open point is a missing named diagnostic for the stop that the patches kept permanent.
