REVISE

# Round 3 review, seat architect, plan go-s1-server-store

- Task identifier: review-go-s1-server-store-r3-architect
- Nonce: 9758b1
- Lens: architect
- Plan text reviewed: `.plans/plans/go-s1-server-store/plan.md`, 752 lines, at commit `ec90898b`, with `decisions.md` and `attack/round-2-findings.md`.
- Model and effort requested: claude-sonnet-5-5, high.
- Scope: the patches of round 2 and what follows from them. Unchanged text is not reviewed again.
- One input is uncommitted text. The working tree of `.plans/plans/go-s2-server-signin/plan.md` differs from its last commit, and finding architect-r3 2 cites the working-tree text.

## Top finding

The round 2 fix for the C12 mutation check promises an exact failure sentence, but the order of steps in `TestFeedPositionsFollowCommitOrder` can produce a different first failure, so the hook may never pass (architect-r3 1).

## Findings

### architect-r3 1. MAJOR. The exact failure sentence of the C12 mutation is not guaranteed by the test order

- Tied to: C12.
- Round 2 finding it answers: logic-r2 2, which the register marks fixed.
- Evidence:
  - `plan.md:341` lists three actions in a row: assert the sequence value equals A's allocated value, state that with the lock removed B becomes visible first, and report that case with the exact text `writer B became visible before writer A`. The text never says which assertion runs first.
  - `plan.md:335` gives `WaitForBlock` a 15-second poll that waits for B to block behind A on the feed lock. The text never says what the test does when B returns instead of blocking.
  - `plan.md:352` makes the hook require the exact assertion text before it prints its pass line.
- Failure scenario: The hook deletes the marked lock line and reruns the test. B no longer waits, so `WaitForBlock` polls for 15 seconds and fails with its own timeout message. Alternatively the sequence assertion runs first, reads the value 2 instead of 1 and fails with the sequence message. Either way the exact sentence never appears. The hook exits nonzero on a correct implementation, and C12 fails on the final check although the lock is present and effective.
- Fix:
  - In `TestFeedPositionsFollowCommitOrder`, after starting B, wait on two events at once: B's write returns, or the observer sees B blocked behind A on `feedLockID`.
  - If B's write returns while A still holds the gate, release A, join both writers, read the visible order and fail with `writer B became visible before writer A`. Do this before any sequence assertion and before any gate timeout.
  - Only when B is observed blocked, assert the sequence value. A gate timeout fails with a distinct fixture message, and the script treats that message as a hook failure.
  - Add one sentence to item 4 saying that the sequence assertion belongs to the second mutation, the lock moved behind the inserts.

### architect-r3 2. MAJOR. The list of names handed to later teams is incomplete in both directions

- Tied to: R2, plan.
- Round 2 finding it answers: architect-r2 8, marked fixed another way. The table is marked `UNREVIEWED MECHANISM`.
- Evidence:
  - `plan.md:52-59` lists about 20 names. Names that consumers already build against and that the table omits:
    - `ReadTx`, used by `CheckInRead` at `.plans/plans/go-s2-server-signin/plan.md:161`.
    - `Migrations`, used at `.plans/plans/go-s2-server-signin/plan.md:120` and `:424`.
    - `storetest.NewDatabase` and `storetest/migrated.NewStore`, used at `.plans/plans/go-s2-server-signin/plan.md:137` and `:521`.
    - `Tx`, `ChangeRef`, `Saved`, `Removed`, `NewID`, `Follower`, `Subscription`, `Snapshot`, `Prune`, `PruneResult`, `MaxWatchLag`, `ErrSubscriptionState`, `ErrTooManyChanges` and `watch.Serve`.
  - The table has only one direction. The working-tree sign-in plan at `.plans/plans/go-s2-server-signin/plan.md:478` writes `func (s *Store) AcquireServerInstance` in `store/instance.go` and registers the lock key `serverInstanceLockID int64 = 841941100003` "with the Store plan". A search of `plan.md` and `decisions.md` for that key, the file and the method finds nothing. `plan.md:190` says parts may not allocate keys in the reserved spaces and lists only three constants.
  - `plan.md:739` expects that no production advisory lock survives transaction end. The instance lock is a session lock held for the process lifetime by design.
  - `plan.md:59` says the epic and the consuming plans are outside this assignment, so nothing compares the list with them.
- Failure scenario: Sprint 2 adds `store/instance.go` to a package that item 3 and item 4 own, with a fourth lock key that no Store test knows. The lock-cleanup check at `plan.md:739` then fails on a running server, or the Store team adds a fourth constant that collides with the key. Nothing in the handoff table would have exposed either problem.
- Fix:
  - Replace the table with one row per exported name of `store`, `store/storetest`, `store/storetest/migrated` and `watch`. Give each row its writer and its readers by plan and sprint.
  - Add the reverse direction: a section titled "Names that later plans write into Store folders", with `store/instance.go`, `AcquireServerInstance`, `InstanceLock` and `serverInstanceLockID`. Add the key to `locks.go` in the list at `plan.md:190` and state the owner of the file.
  - Say that the orchestrator compares the final `go doc -all` output of the four packages with the consuming plans, not this table.
  - State that the lock-cleanup check at `plan.md:739` excludes `serverInstanceLockID`.

### architect-r3 3. MAJOR. The contract between the test database wrapper and the Store fixture has two gaps

- Tied to: R10.
- Round 2 findings it answers: architect-r2 3 and threat-r2 8.
- Evidence:
  - `plan.md:133` allows plaintext only for a Unix socket, a literal loopback address and the exact Compose service. `plan.md:141` makes the fixture read `LGTM_TEST_POSTGRES`.
  - `.plans/plans/go-s1-stack-vault/plan.md:84` says only that the wrapper publishes a dynamic port on loopback and sets the variable. It never states the host form or the TLS setting, and the Store plan states neither.
  - `plan.md:131` makes every Store hold a main pool of at least 8 connections, a page connection and a listener connection. The working-tree sign-in plan adds one more session for the instance lock at `.plans/plans/go-s2-server-signin/plan.md:478`. Parallel tests and parallel packages each open a Store.
  - `deploy/test/checks.sh:597-598` runs `go test -count=1 ./...` as the first leg of C12.
  - A search of the Stack plan, the Store plan and the design for `max_connections` finds nothing.
- Failure scenario one: The wrapper writes a valid connection string with the host `localhost` or with the default TLS mode, which falls back to plaintext. `Open` refuses it under the rule at `plan.md:133`. Every database test fails on its first run, and C12 fails before it reaches the hook.
- Failure scenario two: Ten parallel tests each hold at least ten backends. The disposable instance runs with the default limit of 100 connections. The next `Open` fails with `53300 too_many_connections`. C12 fails at random, and a rerun passes.
- Fix:
  - Add to `plan.md:141` and to the Stack plan item 8 the exact form of the setting: a literal loopback address or a Unix socket path, an explicit TLS-disabled mode, the database `lgtm_store_test` and no fallback hosts. Add a Stack test that parses the value with the Store's own policy function and requires acceptance.
  - Make the wrapper start the instance with an explicit connection limit of at least 500. Give the fixture a first check that reads the limit and fails with a fixed diagnostic when it is below 300.
  - State the per-Store connection budget in item 1 as `MaxConns + 2`, plus one for the instance lock when the server runs.

### architect-r3 4. MINOR. The test gate hook that makes the gate fire at commit is optional in the text

- Tied to: C12.
- Round 2 finding it answers: architect-r2 10, with the earlier patch from logic2-r1 4.
- Evidence:
  - `plan.md:314` runs `SET CONSTRAINTS ALL IMMEDIATE` before the feed lock. In PostgreSQL that setting lasts to the end of the transaction.
  - `plan.md:337` says the `afterConstraints` hook "can" set `feed_test_gate` back to deferred.
  - `plan.md:341` and `plan.md:350` assume the gate fires at commit.
- Truth table:

| Hook installed | Gate fires | Effect on `TestFeedCommitErrorOutcome` |
|---|---|---|
| Yes, sets the gate deferred | At commit | A backend terminated while waiting at the trigger gives the unknown outcome, as designed. |
| No | At the insert statement inside the batch | The rejection arrives as a statement error before commit and gives the rolled back outcome. The unknown outcome is unreachable. |

- Failure scenario: A test writer follows "can" literally and omits the hook in the outcome test. The test passes by asserting only the first row of the table. The unknown outcome path has no real proof.
- Fix: Change "can" to "must" at `plan.md:337` for the tests that assert commit-time behavior, namely `TestFeedPositionsFollowCommitOrder`, `TestFeedRollbackAndNotification`, `TestFeedLockEndsWithTransaction` and `TestFeedCommitErrorOutcome`. Add an assertion in each that the trigger is deferred before the batch runs.

### architect-r3 5. MINOR. The hook's evidence cannot reach the driver, and it tests a different tree than the first leg

- Tied to: C12.
- Round 2 finding it answers: architect-r2 1.
- Evidence:
  - `plan.md:48` says the wrapper "passes through output and exit status". `plan.md:352` says the script prints a pass line and records the tested commit on standard error.
  - `deploy/test/checks.sh:600` runs the hook with standard output and standard error sent to the null device. Only the exit status reaches the driver.
  - `plan.md:352` archives `HEAD`, while `deploy/test/checks.sh:598` tests the working tree.
- Failure scenario: A lead runs C12 on a checkout with uncommitted changes. The first leg passes on the working tree. The hook tests the older `HEAD` and also passes. The report names no commit, because the commit appears only on discarded standard error. The merge rule needs a passing report on that exact commit.
- Fix:
  - Make the script exit nonzero when `git status --porcelain --untracked-files=no` shows changes under `src`, `go.mod` or `go.sum`.
  - Remove the claim that the pass line and the commit reach the driver. Say that the lead records `git rev-parse HEAD` before and after the C12 run, and that the script's own exit status is the only evidence the driver sees.

### architect-r3 6. MINOR. A reconnect storm can take every main pool connection and end every stream

- Tied to: R5.
- Round 2 finding it answers: architect-r2 5.
- Evidence:
  - `plan.md:131` limits writers to `MaxConns - 3` and puts the page reader on its own pool. It puts no limit on `Read`.
  - `plan.md:611` caps each authorization check at two seconds, including acquisition, and uses the main pool.
  - `plan.md:516` proves only that the follower applies a page while eight snapshots are held.
- Failure scenario: A lag cut-off or a history reset closes every stream. All browsers reopen at once, and each runs a snapshot read on the main pool. Eight snapshots hold all eight connections. Authorization checks of the streams that are still open wait more than two seconds and end with `ErrAuthorizationUnavailable`. Those clients reconnect and repeat the load.
- Fix: Cap concurrent `Snapshot` reads at `MaxConns - 4` with a two-second queue wait that returns `ErrAuthorizationUnavailable`. Add a test that holds the cap and shows that an authorization check and a keyed write still finish. State the cap in the handoff list of finding 2.

## Answers to the questions of the lens file

1. Claims about existing code and tools: pgx `v5.13.0` resolves on the module proxy, so the pin is real. `deploy/test/checks.sh:11,590-602` matches the description of the driver except for the discarded output (architect-r3 5). The Stack plan items 8 name `with-postgres.sh`, `mutate-feed-lock.sh`, `lgtm_store_test` and `CREATEDB` as the Store plan says. The local toolchain is Go 1.23.3 while the plan requires 1.26, which matters at build time and not for the plan. Generated protobuf code is committed in the contract plan, so `go build ./...` in an archived `HEAD` can work.
2. Names and their writers and readers: incomplete in both directions (architect-r3 2). The environment variable `LGTM_TEST_POSTGRES` has a writer and a reader but no stated value form (architect-r3 3).
3. Placement: the Store, Watch and fixture code sit in `src/server/`, the wrapper in `deploy/test/` and the module files with the orchestrator, as the edit table says. One exception is the sign-in plan writing `store/instance.go` into the Store folder (architect-r3 2).
4. Rules R1 to R11. R2 and R10 are touched and found open in findings 2 and 3. R4 is kept: the guard triggers, the batch and the outcome type hold, and the truth table of the guard flags below shows no hole. R5 is kept apart from finding 6. R9 is kept: the allowlist and the receipt safety declaration are consistent with item 9. R1, R3, R6, R7, R8 and R11 are nothing found in the patched text.
5. Comparison with neighbors: the sign-in plan reads `Outcome` before `Retryable`, maps `ErrNoSession` to the unauthorized sentinel and gives `Migrate` its own five-minute context in the working tree, so those three points match. Mismatches are the instance lock (architect-r3 2) and the fixture string (architect-r3 3). The Machines plan is a brief and has no store detail to compare.
6. Merging items alone: items 1, 2, 3 and 6 can merge alone. Items 4, 5, 7, 8 and 9 follow the stated chain. A break between items 3 and 4 needs the feed guard to exist before the writer, and the order in the plan provides it. Item 6 stays independent because it uses raw SQL, but the receipt and prune race is only proved after merge (post-build check 3).
7. Room for wave 2: nothing found. The plan builds no machine or client table, and the feed carries opaque references.
8. Dependencies: one direct dependency, pgx `v5.13.0`, pinned by the orchestrator. The proxy confirms the version. `go mod tidy -diff` is the check, and the first import appears in item 1's tests, so the pin must land before the test phase of item 1. The plan's wording "immediately before item 1" supports that reading.

## Answers to the questions of the task

1. Patches in the register, grouped:
   - architect-r2 1, 3, 10 and logic-r2 2: the wrapper, the fixture and the lock constants remove their failures as written, except that the exact text and the fixture contract remain open (findings 1, 3, 5).
   - architect-r2 2: the orchestrator's pin removes the joint edit and agrees with the epic.
   - architect-r2 4: keyed deadlock retry in item 5 and the merged race check remove the dependency of item 6 on item 4. Nothing found.
   - architect-r2 5: the page pool removes page starvation. It does not protect the readers of streams (finding 6).
   - architect-r2 6, 7, 9 and logic-r2 1, 3, 4, 5, 6: nothing found. The 1,300-row fixture, the empty page reset, the migration budget, the versioned digest and the writer test timing match the text and the neighbors.
   - architect-r2 8: incomplete (finding 2).
   - threat-r2 1 to 9: nothing found beyond finding 3, which touches threat-r2 8. The allowlist, the outcome type, the receipt declaration, the loopback and private service check, the retention diagnostic and the guard triggers agree with each other.
2. `UNREVIEWED MECHANISM` places, attacked as new designs:
   - The handoff table: finding 2.
   - The page pool at `plan.md:131`: finding 6. The connection accounting adds one backend per Store (finding 3).
   - The error allowlist at `plan.md:137`: nothing found. A callback error that wraps a SQLSTATE error is mapped to the fixed `*Error`, and `TestDatabaseErrorsHideValues` covers it.
   - The migrated fixture package at `plan.md:192`: nothing found. The subpackage avoids the import cycle.
   - The feed batch at `plan.md:316`: the order lock, flag, insert, notify is sound. Finding 4 covers its test seam.
   - The marker script at `plan.md:354`: findings 1 and 5.
   - The test gate key at `plan.md:337`: its key `841941100901` shares the high word 196 with the production keys, and the classes do not collide with the two-integer receipt lock because the receipt tag uses `objsubid = 2`.
   - The prune independence at `plan.md:430`: nothing found. The pruner takes row locks and no advisory lock, and `WriteOnce` takes the advisory lock first and one row second, so no lock cycle exists.
   - The stale-clock reset at `plan.md:554` and the snapshot cut-off at `plan.md:548`: nothing found.
3. Rejected findings: none exist in the register, so there is nothing to check.
4. Writer and reader lists: complete inside the patches for `NewDomainError`, the receipt sentinels, the lock constants and the test seams. Incomplete in the rollout table (finding 2).
5. Contradictions: no two patches contradict each other. Truth table for the feed guard triggers:

| Operation | Flush flag | Prune flag | Result |
|---|---|---|---|
| Insert | on | any | Allowed. |
| Insert | off | on | Refused with `23514`, `feed_requires_store_flush`. |
| Update or delete | any | on | Allowed. |
| Update or delete | on | off | Refused with `23514`, `feed_requires_store_prune`. |
| Truncate | any | any | Refused with `23514`, `feed_refuses_truncate`. |

   The stamp phase of item 6 needs the prune flag and the fixture inserts of items 6 and 8 need the flush flag, as the tests state. The regression test of item 8 disables the truncate trigger inside one transaction and nothing else needs that. The interaction of `SET CONSTRAINTS ALL IMMEDIATE` and the test gate is in finding 4.
6. Round 2 blockers and majors still open: logic-r2 2 is open in part (finding 1). architect-r2 1 is open in part (finding 5). architect-r2 3 is open in part (finding 3). architect-r2 4, architect-r2 5, architect-r2 2, threat-r2 1, 2, 3: closed on the text I checked.
7. Lens questions touched by the patches: 1, 2, 4, 5, 6 and 8, answered above.

## Post-build checks

| Check | Scenario | Expected result |
|---|---|---|
| Hook order | Delete the marked line and run `TestFeedPositionsFollowCommitOrder` by hand with the gate held. | The first failure line is `writer B became visible before writer A`, and it appears in under five seconds. |
| Hook on a dirty tree | Edit a tracked file under `src`, then run C12. | The script exits nonzero with a fixed diagnostic about uncommitted changes. |
| Fixture string | Parse the wrapper's `LGTM_TEST_POSTGRES` value with the Store's connection policy in the wrapper's own test. | The policy accepts it with no fallback host and no TLS negotiation. |
| Connection budget | Run `go test -count=1 ./...` with the full Server suite on the wrapper's instance and sample `pg_stat_activity` once a second. | The peak count is below the instance limit, and no test reports `53300`. |
| Gate deferral | In `TestFeedCommitErrorOutcome`, read `pg_trigger` state for `feed_test_gate` after the hook runs. | The trigger is deferred before the batch. |
| Instance lock | Start the server, then run the lock-cleanup check of `plan.md:739`. | The check ignores `serverInstanceLockID` and finds no other advisory lock after transaction end. |
| Reconnect storm | Close eight streams at once with a history reset and reopen them together while writes continue. | Authorization checks and keyed writes finish within two seconds, and no stream ends with `ErrAuthorizationUnavailable`. |

## Grade

5 out of 10. Counts: 0 blockers, 3 majors and 3 minors. The points are 12.
