REVISE

Top finding: the patched commit-outcome rule leaves cancellation or timeout during commit untested, and a wrong classifier passes every listed test (logic-r3 1).

# Round 3 review of the server store plan, logic lens

- Task identifier: review-go-s1-server-store-r3-logic
- Nonce: a511bf
- Model and effort: claude-sonnet-5-5, high
- Scope: the diff from commit 1712f47c to commit ec90898b of `plan.md` and `decisions.md` in this folder, plus what the patches touch.
- Finding identifier prefix: `logic-r3`.
- Severity words follow the consequence rule. A blocker is cross-account exposure or permanent corruption. A major is a core promised flow that fails or a shipped feature that breaks. A minor is a limited defect.

## Findings, most severe first

### logic-r3 1. Major. Commit outcome is untested for cancellation, and the adapter rule has gaps

- Design statement: R4 (a write is applied once or reported as not applied), with R9 (errors do not leak).
- Evidence: `plan.md:137`, `plan.md:139`, `plan.md:350`, `plan.md:59`.
- What the patch says. An error before commit, or one with an affirmative PostgreSQL rollback response, is `OutcomeRolledBack`. A failure after a commit may have been sent, with no confirmed outcome, is `OutcomeUnknown`, "including a lost connection or timeout". Line 137 separately says that before commit, cancellation is normalized to the context error.
- What the test covers. `TestFeedCommitErrorOutcome` has exactly two cases: a deferred trigger rejects the commit (rolled back), and a backend is terminated while waiting at the trigger during commit (unknown).
- Failure scenario.
  1. A caller context expires, or the 30-second write budget ends, while `COMMIT` is in flight. The driver returns a context error with no SQLSTATE (Structured Query Language state code).
  2. A classifier of the form "has a SQLSTATE, so rolled back, otherwise unknown" passes both listed cases. A classifier of the form "context error means cancelled before commit, so rolled back" also passes both cases.
  3. The second classifier reports `OutcomeRolledBack` for a commit that the server may have applied. The adapter then treats the call as safely failed, and a user or client repeats an unkeyed mutation. The result is a duplicate write.
  4. A server-side statement timeout (SQLSTATE 57014) during commit is a PostgreSQL rollback and must be `OutcomeRolledBack`, while a client-side context expiry at the same instant is unknown. The plan does not separate these two, and no test names either.
- Gaps in the adapter rule on line 139 and in the table on line 59.
  - Truth table for what an adapter reads: `*Error` with a rolled-back outcome, `*Error` with an unknown outcome, a direct Store sentinel, a `NewDomainError` sentinel, and a bare context error. Only the first two have an `Outcome`. The other three have none, and the plan does not say what an adapter does with them. The zero value of `Outcome` is not defined either: line 115 gives two constants and no empty value.
  - `ErrTxCommitRollback`-style results, where the driver reports that a commit became a rollback, are not named. They are rolled back, and a test should say so.
- Fix.
  - State one rule: after the commit message is sent, any error that is not an affirmative PostgreSQL error response is `OutcomeUnknown`, including context expiry and the 30-second budget.
  - State that a non-`*Error` value has no outcome, that the zero value of `Outcome` is invalid, and that adapters must treat a missing outcome as unknown for any error returned from a write.
  - Add two cases to `TestFeedCommitErrorOutcome`: cancel the caller context while the commit waits at the gate, which must be unknown, and set a short statement timeout so the server cancels the commit, which must be rolled back.
- Cheapest wrong implementation that passes today's tests: the context-error-is-rolled-back classifier above.

### logic-r3 2. Major. The C12 mutation check requires text that the test as written may never print

- Design statement: C12 (the feed lock is proven by a failing test when it is removed).
- Evidence: `plan.md:341`, `plan.md:335`, `plan.md:352`.
- What the patch says. The script deletes the marked lock line, requires the build to pass, reruns `TestFeedPositionsFollowCommitOrder`, and requires a nonzero exit with the exact assertion `writer B became visible before writer A`. Otherwise it fails.
- What the test says. It first waits for A at the gate, starts B, and with the lock requires that B waits for A on `feedLockID` (through `WaitForBlock`, which polls for up to 15 seconds). Only after that does it say "with the lock removed, B commits and becomes visible first", release A, and report the exact text.
- Failure scenario.
  1. With the lock line deleted, B never waits on `feedLockID`. B reaches the gate trigger, which is held by A's gate. The two cases the plan describes happen in different orders depending on how the test is written.
  2. If the "B waits for A" check comes first, `WaitForBlock` times out after 15 seconds and fails the test with its own message. The script requires the exact text from the second assertion and reports a failed mutation check, so C12 can never print its PASS line, even though the lock is doing its job.
  3. If the builder fixes that by checking visibility first, the check races: B may not have reached commit yet when it is sampled, and the test passes in the mutated run or reports the wrong text intermittently.
- Fix. Write the test as one explicit sequence. After starting B, poll for 15 seconds for exactly one of two events: B appears as blocked on `feedLockID`, or B's row becomes visible while A's is not. Only the second event uses the exact text and fails. Also state that no other failing assertion in this test may print that text, and that `WaitForBlock` timing out in the mutated run is not acceptable as the failing assertion.
- Why this is open under the patch. Round 2 logic-r2 2 asked for the exact assertion. The patch added the exact text to the script but left the test's order of checks ambiguous, so the script and the test can disagree.

### logic-r3 3. Major. The wait helper is built after the items that use it, and it cannot see the receipt lock

- Design statement: plan (the order of items and the contents of their tests).
- Evidence: `plan.md:335`, `plan.md:234`, `plan.md:236`, `plan.md:395`, `plan.md:513`, item dependencies in the rollout section.
- What the patch says. Item 4 writes `WaitForBlock` and `HoldAdvisoryGate` in `storetest/gate.go`. Its signature takes `key int64` and reconstructs a bigint from `objsubid = 1`.
- Failure scenarios.
  - Order. Item 3's `TestMigrateColdConcurrentStarts` and `TestMigrateUsesStartupBudget` wait for advisory-lock waiters in `pg_locks`. Item 3 comes before item 4, and item 4 needs item 3. A test writer for item 3 has no helper to call. Either each writer re-implements the polling query, or item 3's tests cannot compile in the order the plan gives. The loop requires tests to fail on their assertion, not on a missing symbol.
  - Key space. Item 5's `TestWriteOnceConcurrentRetry` requires the observer to show B waiting with `objsubid = 2` and `classid = receiptLockNamespace`. `WaitForBlock` accepts one bigint and filters on `objsubid = 1`. It cannot express the two-int4 key. The test must use a different query, and the plan names none, so the shipped helper and the shipped test disagree.
  - Item 7's `TestFeedReaderBoundsStalledQuery` also uses an observer-held lock. It needs item 4, which item 7 already depends on, so that one is fine.
- Fix. Move `storetest/gate.go` into item 1 (it already owns `storetest`), or into item 3 as a new first sub-step, and widen the helper: one function that takes the two-column key as `(classid, objid, objsubid)` and a convenience wrapper for the bigint form. State the dependency in the item header of each user.

### logic-r3 4. Minor. "Successful page" and "accepted page" are used as one thing, and the invalid-page cell is empty

- Design statement: R5 (stale streams end within a bounded time).
- Evidence: `plan.md:495` (`onPage` after "an empty accepted page"), `plan.md:554` ("each successful page, including an empty page, resets the ten-second stale clock"), the invalid-page rule in item 7.
- Truth table for what resets the stale clock:

| Page outcome | Resets the clock? | Stated? |
|---|---|---|
| Read error or two-second timeout | No | Yes (evaluate after a failed attempt) |
| Empty page, accepted | Yes | Yes |
| Rows page, accepted | Yes | Yes |
| Page with head below the cursor (history reset) | Yes, by implication | No |
| Non-reset page starting at or below the last applied position (invalid) | Not stated | **Empty cell** |

- Failure scenario. A broken replica or a bug returns invalid pages every poll. The read succeeds, so an implementation that resets on "successful read" never ends the streams and never raises the unavailable error, while subscribers receive nothing. An implementation that resets only on "accepted" ends them correctly. Both pass the listed tests, which cover empty polls and timeouts only.
- Fix. Say that only an accepted page resets the clock, list the history-reset page as accepted, and add an invalid-page case to `TestFollowerEndsStaleStreams`.

### logic-r3 5. Minor. The feed guard does not enforce the watermark agreement that decision 35 claims

- Design statement: D5 (feed history is pruned in step with its durable watermark).
- Evidence: `decisions.md:42` (row 35: "A deleted feed row must agree with the durable watermark"), `plan.md:230`, item 6 text.
- Failure scenario. The triggers only check the transaction-local flag `lgtm.feed_prune`. Any trusted code that sets the flag can delete rows above `pruned_through`, or delete below it without advancing it, and the guard accepts. The plan says "the guards catch accidental changes by trusted callers", which is weaker than the decision. The same-transaction rule for `pruned_through` is enforced only by the pruner's own code and its test.
- Fix. Either weaken row 35 to say the flag only blocks accidental changes, or make the delete trigger compare `OLD.position` with `feed_retention.pruned_through` read in the same transaction and raise 23514 when the delete is not covered. The second form is cheap and removes the wrong state, which is lens question 8.

### logic-r3 6. Minor. The writers and readers table omits names the consumers already use

- Design statement: R2 (one owner for each shared name).
- Evidence: `plan.md:44` to `plan.md:59`. The sign-in plan uses `store.Migrations` and `*store.ReadTx` and the test helpers `storetest.NewDatabase` and `migrated.NewStore`.
- Missing from the table: `Migrations`, `ReadTx`, `Read`, `Store`, `FeedRow`, `Snapshot`, `Follower.Subscribe` and the `Subscription` methods, `ErrSubscriptionState`, `ErrInvalidSource`, `ErrWatchLagged` is listed but `MaxWatchLag` is not, and the two test helper packages. Round 2 architect-r2 8 asked for a complete list, and the patch marks its own list `UNREVIEWED MECHANISM`.
- Fix. Add one row for the read-side names with Sprint 2 sign-in as reader, and one row for the test helpers with Sprint 2 and later Server plans as readers.

### logic-r3 7. Minor. The overdue-receipt diagnostic fires after any long downtime and the sweep has no cap

- Design statement: D5.
- Evidence: `plan.md:419`, `plan.md:412`.
- Failure scenario.
  1. The server is stopped for three hours. On restart the first sweep sees unobserved receipts older than two intervals and reports the diagnostic, though nothing is wrong. Every restart after a short outage trains the owner to ignore it.
  2. The stamp and delete phases repeat until a batch is short. With a very large backlog and no per-sweep cap, one sweep can run for a long time, and the next hourly tick is skipped or overlaps. The plan does not say which.
- Fix. Compare against the time the pruner last completed a sweep or the process start time, not the wall clock alone, or report only when the process has been running for two intervals. State that a tick during a running sweep is dropped.

### logic-r3 8. Minor. The extra mutations leave no record

- Design statement: C12.
- Evidence: `plan.md:354` (marked `UNREVIEWED MECHANISM`).
- Moving the lock behind the inserts and substituting a session lock are done by the item 4 lead on disposable copies. Nothing is written down when they ran, and the post-build table does not ask for a result. I checked the first one by reasoning: with the lock after the inserts, B allocates position 2 before A releases, so the `last_value` assertion fails as the plan says. The second one is covered by `TestFeedLockEndsWithTransaction`. The reasoning holds, but the check relies on a person remembering to do it.
- Fix. Put a line for each mutant into the item's report, with the failing test name and message, and list that report in post-build checks.

### logic-r3 9. Minor. The five-minute migration bound is not tested

- Design statement: D3.
- Evidence: `plan.md:188`, `plan.md:236`.
- `TestMigrateUsesStartupBudget` proves that twelve seconds of waiting and a 20-second statement succeed. An implementation that sets `statement_timeout` to zero, or sets it to sixty seconds, passes. Nothing tests that a migration is stopped when the five-minute bound is reached.
- Fix. Add a case with a fixture bound of a few seconds (a private option, like the other seams) and require the statement to be cancelled and the ledger to stay unchanged.

## Answers to the seven questions of the task

1. **Does each patch remove its finding's failure, and break nothing else?** Most patches do. Open results: logic-r2 2 is only half fixed (finding 2), threat-r2 2 is fixed for two cases only (finding 1), architect-r2 10 and architect-r2 4 depend on a helper that cannot express the receipt key (finding 3), and threat-r2 7 is fixed for flags only (finding 5). The rest hold. I checked the page pool sequencing, the dedicated page connection during follower startup, the writer starvation timeline of `TestWritersCannotStarveReaders`, the receipt lock namespace with `objsubid = 2`, and the versioned digest prefix. I found nothing wrong in them.
2. **Places marked `UNREVIEWED MECHANISM`.** There are five. Architect-r2 8 (the names table) is finding 6. Architect-r2 6 (the extra mutations) is finding 8. The fixture gate constant and `feed_test_gate` trigger at `plan.md:337`: nothing found, the constant is in its own test file and the trigger is deferred, so a rejected commit is observable. The critic-r1 11 rule for slow snapshots (`plan.md:548`): nothing found beyond one note. The reported lag is "once per consecutive incident until a stream starts", and a stream that starts and lags at once again counts as a new incident, which is acceptable. The architect-r2 4 split of receipt and prune checks (`plan.md:425`): nothing found, and the merged check is listed in post-build checks.
3. **Rejected outcomes.** None exist. For the "fixed another way" outcomes: architect-r2 4 (keyed deadlock retry in item 5) is sound, because the test needs only items 1, 3 and 4. For the deadlock test, PostgreSQL chooses which transaction is the victim, and the plan says "the victim", so the test must accept either one; I saw no text that forbids this. Architect-r2 5 (page pool) is sound. Architect-r2 6 is finding 8. Architect-r2 8 is finding 6. Architect-r2 10 is sound for the production constant, and finding 3 covers the helper. Threat-r2 1 (the allowlist) is sound in design, and finding 1 covers its commit-time gap.
4. **Complete writers and readers for each changed name.** No. See finding 6. The names `Outcome` and `Retryable` are listed, but the rule for a non-`*Error` value is missing (finding 1).
5. **Contradictions and truth tables.** Two tables are built above: adapter inputs (finding 1) and what resets the stale clock (finding 4). One contradiction was found between decision 35 and the guard text (finding 5), and one between the C12 script and the test order (finding 2). Another table, retry policy by outcome, is consistent: rolled back with 55P03, 40P01 or 40001 retries, rolled back with another code does not, unknown retries only with the same key.
6. **Round 2 blockers and majors still open although marked fixed.** There were no blockers. Of the majors, logic-r2 2 is partly open (finding 2) and threat-r2 2 is partly open (finding 1). The other majors hold: logic-r2 1 (batches and the 1,300-row fixture), threat-r2 1, threat-r2 3 (the safety declaration, with the owner choice still open), architect-r2 1 to 5.
7. **Lens questions touched by each patch.** See the next section.

## Answers to the eight lens questions

1. **Sequences, positions, expiries.** Feed positions: gaps from rollback are allowed, ties cannot occur, the lock orders commits, duplicates are blocked by the cursor rule, and wrap at 2^63 is out of reach. Restart: head and `pruned_through` give a floor, and a regression is detected. Timestamps: `written_at` from `clock_timestamp()` can precede commit, which only matters to the receipt diagnostic (finding 7). The stale clock has the empty cell in finding 4.
2. **Failures.** Database down, timeout and duplicate request are covered. Commit with unknown outcome is covered for connection loss only (finding 1). The migration over-run is covered by budget and not tested (finding 9). Clock disagreement is covered by the cutoff sampled once.
3. **Truth tables.** Findings 1 and 4. Retry policy and the connection policy (loopback, socket, Compose host) show no empty cell.
4. **Zero values.** The zero `Outcome` is undefined (finding 1). A zero position is handled: a new database sends zero as a present value. A blank scope means server-wide and is stated. A zero `OldestUnobservedReceiptAt` means none, which is stated.
5. **Two actors on the same row or stream.** Receipt replay against pruning is covered by `FOR UPDATE` and the merged check. Two writers on the feed are ordered by the lock. Two drains on one page are prevented by one reader goroutine. Pruner against a second Store instance: the `feed_retention` singleton is taken `FOR UPDATE`, which is sound.
6. **Does the named mutation fail the tests.** Lock removal: only if finding 2 is fixed. Lock behind inserts: yes by reasoning (finding 8). Session lock: yes. Polling removed, inclusive cursor, cursor reset on error: yes per the item 7 tests. A wrong change the tests would not catch: the commit classifier in finding 1, the stale clock reset on invalid pages in finding 4, and a zero statement timeout in finding 9.
7. **Cheapest wrong implementation.** Finding 1 (outcome by SQLSTATE presence), finding 4 (reset on any successful read), finding 5 (flag set and delete without watermark), and finding 9.
8. **A different shape.** A trigger that checks the watermark (finding 5) and an `Outcome` type with no valid zero value (finding 1) each make a wrong state impossible. The helper should be one type with the full lock key (finding 3).

## Post-build checks to add or keep

1. Run `TestFeedCommitErrorOutcome` with the two new cases (cancel at commit, server-side statement timeout) and read the outcome in each.
2. Run `src/server/internal/store/check-feed-lock.sh` and read the printed PASS line, then also read the failing test output in the mutated copy and confirm the exact text comes from the visibility assertion and not from a helper timeout.
3. Record, in the item 4 report, the result of the two extra mutations with the failing test name.
4. Run an invalid-page fixture against the follower for twelve seconds and confirm the unavailable error is raised once.
5. Attempt a guarded delete above `pruned_through` as the fixture administrator and confirm it is refused (if finding 5 is taken).
6. Confirm that the receipt wait test finds B through the same helper as the feed tests.
7. Restart a fixture server after a three-hour shifted clock and confirm no overdue diagnostic appears (if finding 7 is taken).

## Grade

Grade: 7 out of 10. The designs mostly hold, and the open points are gaps in tests and in text.

Counts: 0 blockers, 3 majors, 6 minors. Verdict: REVISE. The three majors are each a short text change and need no new mechanism.
