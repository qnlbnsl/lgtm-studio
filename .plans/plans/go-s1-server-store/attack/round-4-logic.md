# Round 4 review of the server store plan, seat logic

- Task identifier: review-go-s1-server-store-r4-logic
- Nonce: 05c181
- Plan: `.plans/plans/go-s1-server-store/plan.md`, patches from commit `ec90898b` to `81561292`.
- Lens: logic and failure policy.

## Verdict

REVISE

## Top finding

`TestFeedCommitErrorOutcome` requires a server `statement_timeout` to end a commit that waits in a deferred trigger, and PostgreSQL 16 does not enforce that timeout during commit, so the test cannot pass as written.

## Findings

### logic-r4 1. MAJOR. R4.

The commit outcome test relies on a server statement timeout that never fires at commit.

- Evidence: `plan.md:142` says a PostgreSQL rejection "including server `statement_timeout` with `57014`" confirms rollback during commit. `plan.md:355` says to "hold for a three-second server statement timeout with the caller still live" and to "require rolled back for rejection and server `57014`". `plan.md:756` repeats it as "A server-side timeout rejection confirms rollback".
- Check I ran: a throwaway PostgreSQL 16 container with a deferred constraint trigger that takes `pg_advisory_xact_lock` for key `a`, another session holding that lock for twelve seconds, and a transaction that ran `SET CONSTRAINTS ... DEFERRED`, `SET LOCAL statement_timeout='3s'`, the insert and then `COMMIT`. The `COMMIT` returned success after 10.8 seconds. No `57014` was raised. The server disables the statement timeout before it runs the commit-time work, and deferred triggers run in that work. I then repeated the run with `SET LOCAL lock_timeout='3s'`. The commit failed with a lock timeout error from the trigger body, so `lock_timeout` does reach the commit phase. The container was removed afterwards.
- Failure scenario: the builder writes the case as specified. The gated writer waits until the observer releases the gate or the 15-second fixture deadline expires. Then it either commits successfully or hits the fixture deadline. Neither is `57014`. The test cannot pass, and the builder cannot freeze it "after it fails on the named assertion" in the way the plan means. The classifier line at `plan.md:142` also promises a behavior that nothing can produce or test at commit.
- Fix: replace the statement-timeout case. Use `beforeCommit` to run `SET LOCAL lock_timeout` to three seconds while the gate is held. The trigger's lock wait then fails with `55P03`. Require `OutcomeRolledBack` and `Retryable` true for that case, which also exercises the retryable path at commit. If `57014` stays in the list of confirmed rollbacks, cover it by having the observer call `pg_cancel_backend` on the gated writer's backend, and say so in the test. I did not run that variant. Remove "server `statement_timeout`" from `plan.md:142`, from `plan.md:355` and from the post-build row at `plan.md:756`, or reword each to the case you can observe.

### logic-r4 2. MAJOR. R10.

The fixture requires `max_connections` of at least 500 from Stack, and no Stack text provides it.

- Evidence: `plan.md:144` says Stack's disposable instance sets `max_connections` to at least 500 and the fixture refuses below 300. Stack's plan `.plans/plans/go-s1-stack-vault/plan.md:84` describes the `with-postgres.sh` instance as the pinned image with temporary storage, a dynamic loopback port, the maintenance database and `CREATEDB`. It names no connection setting, no `sslmode` value and no fallback rule. A search of that plan and its scouting for `max_connections` finds nothing. That plan is marked done after its fourth round. The Store plan's own handoff table (`plan.md:52` to `plan.md:57`) and rollout step 2 (`plan.md:47`) do not list this duty for Stack.
- Failure scenario: sprint 1 starts. The pinned PostgreSQL image runs with its default of 100 connections. The first call of `storetest.NewDatabase` reads `SHOW max_connections`, sees 100, and refuses below 300. Every Server database test and the C12 check fail on the first run. Nothing in the orchestrator's `go doc -all` comparison (`plan.md:62`) can detect this, because it compares exported names only.
- Fix: write the duty into both plans. In the Store plan, add one row to the handoff table naming Stack as writer of the instance setting (`max_connections` of at least 500 and an explicit plaintext loopback or socket URL with no fallback host) and Server's fixture as reader, and mention it in rollout step 2. Have the orchestrator amend `.plans/plans/go-s1-stack-vault/plan.md:84` before sprint 1 starts to start the instance with `-c max_connections=500`. If the vault plan stays closed, drop the hard floor of 300 and let the fixture derive its limit from what `SHOW max_connections` reports, and state that the parallel root suite must not exceed it.

### logic-r4 3. MAJOR. R5.

The patch for logic-r3 4 removed head regression detection but left the paragraph that requires it.

- Evidence: `plan.md:558` still says "When a page reports a lower head than the read cursor, close subscribers with `ErrWatchLagged` and reinitialize cursor and ring at the new head. Report a sanitized history-reset event." The patch removed the `Head` field from `feedPage` (`plan.md:487`), made `readFeedPage` return only the floor and rows (`plan.md:496`), deleted `TestFollowerDetectsHeadRegression` and dropped it from criterion 8. The new paragraph at `plan.md:560` says "Wave 1 omits history reset", and the overview at `plan.md:26` says restoring an earlier database is outside wave 1.
- Failure scenario: the builder of item 8 reads `plan.md:558` as a requirement. No page carries a head, and `feedPage` belongs to item 7, which is already frozen. The builder either adds a head query and changes item 7's struct and tests, or skips the paragraph and a reviewer finds an untested requirement. Both cost rework. If the paragraph is skipped and the database is restored behind the cursor, the reader asks for `position > cursor`, gets an empty page that counts as accepted and resets the stale clock. Streams then stay open and deliver nothing. The plan accepts that for wave 1, but only if the paragraph is gone, so that the plan states one behavior.
- Fix: delete the paragraph at `plan.md:558`, including its sentences about a history-reset event, overlapping heads and operations stopping the server. Move the one remaining fact into the overview sentence: after a restore to an earlier state, open streams go silent and the server must be restarted. This keeps decisions row 42 and the text in agreement.

## Answers to the attack questions

Task questions:

1. Patches against their findings. `architect-r3 1` and `logic-r3 2` (observer, either-branch race): the failure is removed. I traced the unmodified run, the lock-removed mutant and the lock-after-inserts mutant. The first ends in the "B waits" branch, the second in the visible-B branch with the exact sentence, and the third in the sequence assertion because B has allocated position 2 while A is at 1. `logic-r3 1` (commit outcomes): partly removed, see finding 1. `architect-r3 3` (fixture capacity): see finding 2. `logic-r3 4` (stale clock, restore): see finding 3. `logic-r3 3` (shared advisory key helper): removed, the key holds both words and the sub-identifier, and the pids pin the match across databases. `logic-r3 5` (guarded delete): removed. Advancing the floor first makes the row trigger see its own update, and a prefix of existing rows cannot be undercut by a later commit because positions follow commit order. `logic-r3 7` (completion-based schedule): removed. `logic-r3 9` (live setting and caller deadline): removed, because a mutant that keeps the production `lock_timeout` also fails the twelve-second hold. `architect-r3 4`, `architect-r3 5`, `logic-r3 8` (hook details): nothing found.
2. `UNREVIEWED MECHANISM` places. Name grouping and `go doc -all` comparison (`plan.md:62`): it compares exported names only, while the reserved `serverInstanceLockID` is unexported. The names agree in the sign-in plan (`.plans/plans/go-s2-server-signin/plan.md:480`), so nothing breaks, but the comparison cannot catch a mismatch there. Commit classifier: finding 1. Snapshot admission: the slot count against the pool (five writers at most, one snapshot, one pruner, one read connection left at eight) holds, and the busy-slot error shares `ErrFeedUnavailable` with the stale-feed error, which both map to `unavailable`. Nothing found. Completion-based pruner schedule: nothing found, except that a failed sweep waits a full hour, which the test names as recovery. Stale clock on accepted pages: finding 3 for the leftover text. The page rejection rule cannot fire in production because the query is `position > cursor`, so it is test-only. Nothing found beyond that.
3. Rejected findings. The register of round 3 has no finding with the outcome "rejected". Nothing found. For "fixed another way", the evidence for dropping restore handling is the design's statement that a position is "a number that only grows", which is real, but the leftover text contradicts it (finding 3).
4. Names with readers and writers. The old `WaitForBlock(..., key int64)` and the item 4 ownership of the gates changed. No other plan under `.plans/plans/` uses either, which I checked by search. The handoff table lists `NewDatabase` and `migrated.NewStore` but not the `storetest` gate names, which only items 3 to 9 of this plan read. The Stack side of the fixture contract is incomplete (finding 2).
5. Interacting rules. I built the truth table for the feed guards (insert needs the flush flag, update and delete need the prune flag, delete above the floor is refused, truncate is refused) against the prune transaction and the seeds. Every cell is defined and the prune transaction satisfies all of them. For the commit classifier I tabulated rejection, backend termination (`FATAL` with a code), cancellation, deadline, lock timeout, statement timeout and `pgx.ErrTxCommitRollback`. The statement-timeout cell is unreachable (finding 1). The head-regression text against the new stale-clock rule is the contradiction in finding 3.
6. Open blockers or majors. No blocker was open. `logic-r3 1` is fixed in text but its statement-timeout case is unreachable, so it is open in effect (finding 1). The majors `architect-r3 1`, `architect-r3 2`, `architect-r3 3`, `logic-r3 2` and `logic-r3 3` are closed, except that `architect-r3 3` has a gap on the Stack side (finding 2).
7. Lens questions touched. See the lens answers below.

Lens questions:

1. Sequences, counters, expiries. Feed position, floor, the one-hour schedule, the ten-second clock. Ties and gaps are handled (cache size 1, rolled-back gaps ignored, the cursor is exclusive). The wrap case does not arise at 64 bits. A restore (a backward step) is the only unhandled case, and the plan now says so (finding 3).
2. Failures. Crash mid-write, database down, timeout, duplicate request, expired token: each has a stated outcome and repair. The one broken statement is the commit-time statement timeout (finding 1).
3. Truth tables. See answer 5. The guards table has no empty cell.
4. Zero values. The zero `Outcome` is invalid and treated as unknown. A new feed starts at zero. Nothing found.
5. Two actors on one row. Pruner against replay holds the receipt row `FOR UPDATE`, pruner against writer separates by the floor and the feed lock, and two pruners serialize on the `feed_retention` row. Nothing found.
6. Risky items and mutations. The lock-removal and lock-after-inserts mutations fail the intended assertions (answer 1). Removing snapshot admission fails the queued-snapshot test. A leaked slot after release is not tested directly, but every later Watch test would expose it. Nothing found that needs a finding.
7. Cheapest wrong implementation. For the outcome test, classifying everything during commit as unknown fails the rejection case, and classifying all errors with a code as rolled back fails the backend-termination case. Both are covered. Nothing found.
8. A shape that makes a wrong state impossible. The delete guard tied to the floor in the same transaction already does this for the feed. Nothing found.

## Post-build checks

- Run the commit gate with `SET LOCAL lock_timeout` of three seconds (after the fix to finding 1). Expect `55P03`, `OutcomeRolledBack` and `Retryable` true. Run the same case with `SET LOCAL statement_timeout` and confirm the commit still waits, so the plan's final text carries no statement-timeout claim.
- Run `SHOW max_connections` through the Stack wrapper on the pinned image. Expect at least 500, and a peak under that during the parallel root suite.
- Restore a database dump taken earlier while a stream is open. Expect the documented behavior from the overview sentence and no history-reset event.
- Release the snapshot slot after a canceled loader and after a panic in a loader. Expect the next snapshot to be admitted at once.

## Grade

6 out of 10. Counts: 0 blockers, 3 majors, 0 minors reported (round 4 reports no minors).
