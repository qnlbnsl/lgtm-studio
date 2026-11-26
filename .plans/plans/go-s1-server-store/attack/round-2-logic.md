# Round 2 review of the server store plan, seat logic

- Task identifier: review-go-s1-server-store-r2-logic
- Nonce: 1a2779
- Plan: `.plans/plans/go-s1-server-store/plan.md`, as changed by the round 1 patches.
- Scope: only the patches and what follows from them. No PostgreSQL server was run, and no Go code exists yet, so every claim below comes from reading the plan text.

## Verdict

REVISE.

Top finding: the feed sweep deletes at most one page of 512 rows per hourly run, so a feed that gains more than 512 rows an hour grows without bound, and the plan's own prune test contradicts its prose (logic-r2 1).

## Findings

There are 0 blockers, 2 majors and 4 minors.

### logic-r2 1. The feed sweep removes one page per run and cannot keep up

- Severity: MAJOR.
- Tied statement: D5, which retains feed rows for one day. It also touches R4, the rule that state and feed rows share a transaction.
- Evidence: `plan.md:404` says to "fetch the earliest 512 feed positions and delete only their consecutive prefix", with no repeat. `plan.md:406` says receipts "repeat completed 512-row pages". `plan.md:402` bounds observation to 512 per table, while `plan.md:404` says only "stamp unmarked feed rows in ascending position order", with no bound. `plan.md:410` seeds "multiple pages of observed old rows" for the feed, and `plan.md:400` speaks of "committed batches" in the plural.
- Failure scenario: One write may carry 256 changes (`MaxWriteChanges`), so two writes in an hour add 512 feed rows. The sweep runs hourly, observes and deletes at most 512 feed rows, and stops. A builder who follows `plan.md:404` literally passes every sentence of that item and fails the frozen test at `plan.md:410`. A builder who follows the test adds a loop the prose does not describe. Under the literal reading, a steady 1,000 rows an hour leaves the table growing by about 488 rows an hour forever, and the earliest expired rows are never reached. The snapshot and reader costs grow with the table, and the stalled-prefix report at `plan.md:404` never fires because the prefix is simply slow.
- Fix: State that the feed sweep repeats stamping batches and deletion batches of 512 until a batch stamps or deletes fewer than 512 rows, exactly as `plan.md:406` does for receipts. Say that each deletion batch is its own committed transaction that advances `pruned_through`. Add to `TestPruneCutoffAndWatermark` a seed of at least 1,300 expired feed rows and require all of them gone after one sweep. Name the one-page loop as a mutation target for the pruning item.

### logic-r2 2. The lock-removal script has no stated pass rule that rejects a broken run

- Severity: MAJOR.
- Tied statement: C12, the check that removing the feed lock makes the ordering test fail.
- Evidence: `plan.md:339` describes `check-feed-lock.sh` as removing "exactly the batch statement that acquires the feed lock" and exiting successfully "only for the expected early-visibility assertion". It does not say how the statement is found, how a count other than one is handled, or how the expected assertion is told apart from other failures. `deploy/test/checks.sh:600` treats any zero exit status of the hook as success and discards its output. `plan.md:339` also says "the three lock mutations" while `plan.md:330` names two, and the script performs only one.
- Failure scenario: The cheapest script runs `sed` to delete a line, runs `go test`, and exits zero when the test exits non-zero. If the pattern matches nothing, the baseline still fails nothing, and the script reports a pass only when the mutated run fails, so that case is caught. If the pattern matches the wrong line or two lines, the edit breaks compilation, the test run exits non-zero, and the script exits zero. C12 then reports success while the lock-removal test was never exercised. The builder owns both the script and the production file, so a rename of the lock statement silently turns the check into a compile error that passes.
- Fix: Require the script to do the following, in order. Run the baseline test and require a zero status. Find the lock statement by a marker that appears exactly once in the production file, and exit non-zero if the count is not one. Run `go build ./...` on the mutated copy and exit non-zero if it fails. Run the named test and require both a non-zero status and the exact text `writer B became visible before writer A` in its output. Exit zero only then. Correct "three" to the real number and name each mutation, or say that the script covers only the first and that the second is covered by the test's own sequence assertion.

### logic-r2 3. A successful empty read may or may not reset the stale-stream clock

- Severity: MINOR.
- Tied statement: R5, the shared Watch lifecycle.
- Evidence: `plan.md:533` reads "If no page succeeds for at least ten seconds, including an empty successful read, report one sanitized incident and end streams". `plan.md:545` tests only a read that is held until the attempts time out.
- Failure scenario: One reading is that an empty successful read counts as a success. Another is that the ten seconds expire even though empty reads succeed. A test writer who picks the second reading ends every stream on an idle server every ten seconds with `ErrFeedUnavailable`. No test covers the idle case, so either reading passes the frozen test.
- Fix: Say that any read that returns without an error, including an empty page, counts as a success and resets the clock. Add to `TestFollowerEndsStaleStreams` an idle phase of at least twelve fixture seconds with empty successful polls, and require that no stream ends and no report is made.

### logic-r2 4. The migration runner has no stated time limits, and the pool defaults can cut it off

- Severity: MINOR.
- Tied statement: D3, the one-transaction numbered database changes.
- Evidence: `plan.md:113` installs `statement_timeout=15s` as a pool default. `plan.md:117` sets `lock_timeout` to ten seconds only inside `withWrite`, which has a 30-second budget. `plan.md:166` runs `Migrate` as one transaction and runs each file whole in simple query mode, but names no limits.
- Failure scenario: A migration file that creates an index on a large table takes more than 15 seconds. The pool default cancels it, `Migrate` returns an error, and the server refuses to start on every retry until someone raises the setting. A second starter that waits on the migration lock is cancelled after 15 seconds for the same reason, and the error is not one of the retryable classes at `plan.md:119`. The same default also caps any legitimate single statement in a write at 15 seconds, so the 30-second write budget is unreachable for one statement.
- Fix: State that `Migrate` sets its own limits inside its transaction, with a longer statement timeout and no lock timeout on the migration lock, and says which. State that a write's statements may run up to the operation limit, or lower the operation limit in the text to match the 15-second default. Add a migration test that holds the migration lock for twelve fixture seconds and requires the second starter to wait and then succeed.

### logic-r2 5. The semantic fingerprint rules leave version handling undefined

- Severity: MINOR.
- Tied statement: R4, the receipt rules.
- Evidence: `plan.md:352` adds `FingerprintVersion` to `Request`. `plan.md:368` rejects a nonpositive version. `plan.md:372` frames "the caller, procedure and semantic fingerprint" in the digest, and the version is not in that list. The versioned replay test at `plan.md:380` says "a missing verifier conflicts".
- Failure scenario: A retry arrives after an upgrade that removed the version 1 encoder. The store reports `ErrRequestKeyConflict`, the same error as a real reuse of the key with a different body. A caller cannot tell a harmless upgrade gap from a client bug, and may give up on a request that already committed. Separately, an empty fingerprint for a supplied version is neither rejected nor accepted by the text, so two different test writers can freeze two behaviors.
- Fix: Add a distinct sentinel for a retained receipt whose version has no supplied encoder, and say it is a deployment error rather than a conflict. State that a zero-length selected fingerprint is rejected before lock acquisition. State whether the stored version is part of the digest input, and test the choice with two versions that produce equal bytes.

### logic-r2 6. The writer-starvation test needs more time than the shared fixture deadline

- Severity: MINOR.
- Tied statement: plan, the test seams.
- Evidence: `plan.md:132` runs eight writers on an eight-connection Store. At most five are admitted. The other three are admitted only after the first wave fails at the ten-second lock wait, and then each waits for its own ten seconds. `plan.md:326` says the shared helper waits at most 15 seconds, and the writes carry a 30-second budget (`plan.md:117`).
- Failure scenario: A test that waits for all eight outcomes needs about 20 seconds. Under the 15-second helper deadline it times out for the second wave, and the builder shortens the lock wait in the test or removes the assertion about the second wave to make it pass. The mutation "remove writer admission" then no longer distinguishes the two cases.
- Fix: Say in the test text that it asserts the first wave of five at ten seconds, then releases the gate, and that the second wave is asserted to complete without a lock error. State the per-wait deadline the test uses, and keep it above 20 seconds.

## Answers to the seven task questions

1. Each patch against its finding. The round 1 register lists 49 findings, and I read the patch for each group.
   - Driver and hook ownership (architect-r1 1): removes the missing-owner failure. It leaves the pass rule open in logic-r2 2.
   - Writer admission, deadlines and errors (logic-r1 2 and its repeats): removes the pool-starvation failure. It adds logic-r2 4 and logic-r2 6.
   - Cold-start ledger, lock registry and migrated fixture (critic-r1 2, architect-r1 8 and 9): remove their failures, and I found no new break.
   - Feed flush batch, guard and deferred checks (architect-r1 3, critic-r1 1, logic2-r1 4): remove the deadlock-cycle failure. The deferred-check order holds in the truth table below.
   - Receipt namespace, fingerprint and reply cap (threat-r1 6 and 7, logic2-r1 3): remove cross-principal reuse. They leave logic-r2 5.
   - Retention clock (logic2-r1 1, logic-r1 8): removes the early-expiry failure, but the sweep cannot keep up (logic-r2 1).
   - Follower, lag and history reset (critic-r1 4, architect-r1 6): remove their failures. The stale cutoff is ambiguous (logic-r2 3).
   - Watch authorization and mapping (threat-r1 1, logic-r1 11): nothing found.
2. `UNREVIEWED MECHANISM` places, attacked as new designs. The feed lock timing at `plan.md:322`, the retention clock at `plan.md:402` and the receipt digest at `plan.md:372` are covered by logic-r2 1, 4 and 5. The Stack wrapper at `deploy/test/with-postgres.sh` is outside this plan's text, and I read only its stated contract. The migrated fixture and lock registry: nothing found.
3. Rejected findings. The register has no row with the outcome "rejected". One row, threat-r1 2, is "for the owner", and the plan states the open choice at `plan.md:374`. Nothing found.
4. Writers and readers of changed names. `Error`, `ConnectionPolicy`, `locks.go`, `Tx`, the sentinels and the receipt columns each name their writer and their readers, and the lists are complete. The one gap is the new sentinel that logic-r2 5 asks for, which has no reader yet.
5. Interacting rules. See the truth tables below. The only contradiction is the feed sweep prose against its test (logic-r2 1).
6. Round 1 majors that stay open. logic2-r1 1 is fixed for the early-expiry failure but reopens as an unbounded-growth failure (logic-r2 1). logic-r1 1 and critic-r1 3, the two C12 majors, are fixed in the tests and stay open in the script that drives them (logic-r2 2). The others hold.
7. Lens questions touched by a patch are answered in the next section.

## Answers to the eight lens questions

1. Ordering with ties, gaps, wraps and restarts. Positions are unique and gaps are allowed. Feed stamping is monotonic in position because rows are stamped in position order. A restart after a long outage stamps every row at the same instant, which only lengthens retention. The sequence never resets. Nothing found beyond logic-r2 1.
2. Failure, permit or deny, and repair. A failed stamp or deletion batch leaves the next sweep to repair it. A failed migration refuses to start the server and retries the same files (logic-r2 4). Lock-wait failures are retryable and sanitized.
3. Truth tables for interacting rules. See below.
4. Zero values. A zero-byte reply is stored and replayed. A present zero feed position is carried by presence. An empty fingerprint is undefined (logic-r2 5). An empty feed after full pruning still yields higher positions (`plan.md:410`).
5. Two actors on one row. A replay and the sweeper both lock the receipt row, and the sweeper waits within the lock deadline (`plan.md:406`, `plan.md:413`). The pruner takes no advisory lock, so no lock cycle exists. Two writers on the feed are ordered by the feed lock.
6. Mutations of risky items. The feed lock, the session-lock replacement, the unframed digest and the missing writer admission are all named. The one-page feed sweep and the script's pass rule are not (logic-r2 1 and 2).
7. The cheapest wrong implementation. A feed sweep of one page (logic-r2 1), a script that passes on a compile error (logic-r2 2) and a stale cutoff that ignores empty reads (logic-r2 3).
8. Shapes that make wrong states impossible. The insert trigger and flush flag make a direct feed insert fail. The composite receipt key makes cross-principal reuse impossible. A retained receipt with no verifier has no representable state (logic-r2 5).

### Truth tables

Receipt state against the order of replay and sweep. The replay takes the row lock first or the sweeper does. Both orders end in a defined state.

| Receipt stamp | Age against the cutoff | Replay locks first | Sweep locks first |
|---|---|---|---|
| Null | Not applicable | Replay returns the saved reply, and the sweeper stamps afterwards. | The sweeper stamps, then the replay returns the saved reply. |
| Set | Younger than the cutoff | Replay returns the saved reply. | Nothing is deleted, and the replay returns the saved reply. |
| Set | At the cutoff | Replay returns the saved reply. | Not deleted, because the rule is strictly older. |
| Set | Older than the cutoff | Replay returns the saved reply, and the sweep then deletes it. | The row is deleted, and the retry runs as a new write. |

Pool capacity with eight connections. Writers are admitted at most five at a time, and three slots stay outside admission.

| Slot | Holder | Limit that frees it |
|---|---|---|
| 1 to 5 | Admitted writers | Ten-second lock wait, 30-second budget |
| 6 | Follower page read | Statement timeout of 15 seconds |
| 7 | `Read` or Watch snapshot | Statement timeout of 15 seconds |
| 8 | Maintenance worker | The same limits |

A second concurrent reader waits for a slot, which the plan allows.

Stale cutoff against read results, once logic-r2 3 is fixed.

| Last attempt | Counts as success | Clock |
|---|---|---|
| Page with rows | Yes | Reset |
| Empty page | Yes, by the fix | Reset |
| Error or timeout | No | Not reset, ten seconds then ends streams |

Deferred check order inside one write: `SET CONSTRAINTS ALL IMMEDIATE`, then the `afterConstraints` hook re-defers the test gate trigger, then the feed lock and insert batch, then the commit-time trigger. No two steps contradict each other.

## Post-build checks

| Check | Scenario | Expected result |
|---|---|---|
| Feed sweep volume | Seed 1,300 expired feed rows and run one sweep. | All are deleted, and `pruned_through` equals the last position. |
| Hook rejects a broken edit | Run `check-feed-lock.sh` after renaming the lock marker so that it matches twice. | The script exits non-zero. |
| Hook rejects a compile error | Make the mutated copy fail to build. | The script exits non-zero and names the build failure. |
| Idle stream | Open a stream on an empty feed and wait twelve seconds. | The stream stays open and no incident is reported. |
| Long migration | Run a migration file that sleeps 20 seconds, with a second starter waiting. | Both starters finish and the ledger records the version once. |
| Upgrade gap | Retry a keyed request whose stored version has no encoder. | The distinct sentinel is returned, not a conflict. |
| Starvation timing | Run `TestWritersCannotStarveReaders` with the second wave. | The second wave completes without a lock error after the gate is released. |

## Grade

7 out of 10. There are 0 blockers, 2 majors and 4 minors.
