# REVISE

The frozen feed cancellation test failed in 32 of 60 focused runs because it checked the connection pool before the driver finished discarding the cancelled connection.

This is task `postbuild-go-s1-server-store-b` with nonce `57cf69`. The review covers items 4, 5 and 6 at `bcf920b3c0251afb099de12423dc42d867c82a58` on `sprint-0`. The implementation comparison starts at `319022da`, which already contains the frozen tests. The tests ran from disposable copies of the selected commit. The untracked Store stubs for later items were outside the review. Only this report was written in the shared checkout.

## Findings

### post-b 1 — MAJOR — The feed cancellation test checks the pool before disposal finishes

**Statement.** Item 4 of the plan requires the cancelled writer to return its connection. Rule R10 and acceptance check 12 (C12) depend on a reliable PostgreSQL test of that behavior.

**Evidence, confirmed live.** `TestFeedLockDoesNotBlockUnrecordedWrites/caller_cancellation` failed at `src/server/internal/store/feed_test.go:459` in 32 of 60 runs of the original test. The preceding helper counts only *idle* backends in a transaction. It can return while the cancelled statement is still active or the driver is still closing its socket. The Store defers rollback at `src/server/internal/store/pool.go:325`. The pinned driver releases ownership in `pgxpool/tx.go:37` and starts asynchronous destruction for a closed connection in `pgxpool/conn.go:32`. The pool still counts the connection as acquired until the destructor finishes, as shown in Puddle's `pool.go:73` and `pool.go:695`. A scratch copy changed only the assertion to poll for disposal. All 20 runs passed. In 13 runs the original check saw one acquired connection, and the next poll roughly 11 milliseconds later saw zero.

**Failure scenario.** The driver returns the cancellation to the writer, and the test reads the pool before the cleanup goroutine completes. The test reports a leak even though the pool releases the slot.

**Fix.** Poll for the pool's acquired count to return to its baseline within the fixture deadline. Also require that the cancelled backend no longer holds or waits on the feed lock. Keep the existing assertions about rollback and the caller's cancellation.

### post-b 2 — MAJOR — Prune counts feed deletions from a rolled-back batch

**Statement.** Item 6 of the plan requires `Prune` to return the count of committed deletions in `plan.md:424`.

**Evidence, confirmed live.** `src/server/internal/store/prune.go:208` adds the affected rows inside the transaction callback. The commit runs afterward at `src/server/internal/store/pool.go:338`. On error, `prune.go:212` returns the count already incremented. The receipt deletion path only adds its batch count after a successful commit at `prune.go:288`. A scratch test seeded 513 expired feed rows and added a deferred constraint trigger that rejected the deletion of position 513 at commit. The sweep returned the rollback error and reported 513 deletions. PostgreSQL retained position 513 and kept the watermark at 512.

**Failure scenario.** A prune sweep commits an earlier batch and loses the next batch at commit. A caller reports rows from the failed batch as deleted even though the transaction preserved them and their watermark.

**Fix.** Capture the batch's affected row count inside the callback and add it to the total only after `withMaintenance` succeeds. Preserve the error and the count from the preceding confirmed commits. Add the second-batch rejection case to the retention tests.

### post-b 3 — MAJOR — A slow database checkpoint can fail the pruning prefix test during cleanup

**Statement.** Rule R10 and C12 require a real database fixture. The plan gives database teardown a ten-second deadline at `plan.md:147`.

**Evidence, confirmed live.** The first full Store run passed. The second failed in `TestPrunePreservesFeedPrefix` after 10.66 seconds. Its sole diagnostic was `postgres.go:67: The PostgreSQL fixture database could not be dropped.` The retention assertions had completed. The fixture uses a ten-second context for the forced database drop in `src/server/internal/store/storetest/postgres.go:96` and reports a fixed error at line 105. A focused control passed. With the checkpointer paused in a separate disposable PostgreSQL instance, the same frozen test failed with the same message after 10.74 seconds. The observer sampled checkpoint waits with zero application backends for that database. The original full run recorded no wait event, so its precise database wait remains unverified.

**Failure scenario.** PostgreSQL takes longer than the fixture deadline to finish dropping an otherwise idle database. The test reports a failure after successfully proving retention behavior. A cleanup failure of the same kind could affect any test using the fixture.

**Fix.** Adjust the plan and fixture to allow a bounded cleanup retry or a longer cleanup-only deadline. Report the context outcome or a safe database error code with the fixed diagnostic. Keep the wrapper's disposable instance as the final cleanup boundary.

### post-b 4 — MINOR — The commit timeout test takes two observations of a three-second wait

**Statement.** Item 4 and rule R4 require a held commit gate to prove the retryable rollback after a three-second PostgreSQL lock timeout.

**Evidence, confirmed live.** `src/server/internal/store/feed_test.go:159` waits for the advisory lock observation. The next query independently checks whether the same backend is active on `COMMIT` at line 160. The timeout is set at line 685. The frozen control passed. A local relay in a scratch run delayed only the second query by 3.2 seconds. The unmodified test then failed at line 709 with `backend 109 waited for the fixture gate outside COMMIT`. The delay changed only the observer's query timing.

**Failure scenario.** The observer sees the wait and is delayed until the server has correctly timed out and rolled back. The test fails before examining the writer's result.

**Fix.** Observe the lock wait and active commit in one database query. Independently assert the writer's error from a gate that can only be reached during commit in that case. Keep a gated observation with a longer wait budget to prove the commit phase.

### post-b 5 — MINOR — The frozen tests accept replay without the receipt row lock

**Statement.** Item 6 and rule R4 require a replay and a competing expiry to wait for the transaction that locks the receipt row first. The plan names both public replay orders at `plan.md:751`.

**Evidence, confirmed live.** The replay selects its receipt `FOR UPDATE` at `src/server/internal/store/receipts.go:80`. Pruning locks the same row before deletion at `src/server/internal/store/prune.go:273`. The frozen prune test takes its replay lock directly at `src/server/internal/store/prune_test.go:254`. Removing only `FOR UPDATE` from the production replay query passed the entire frozen Store suite. The public-path scratch test passed on the original code in both lock orders and failed on the mutant. With the deletion gated, the mutant returned the original reply before deletion committed.

**Failure scenario.** A change removes the replay row lock while preserving the keyed advisory lock. The advisory lock still serializes two keyed callers. Pruning ignores that advisory lock and can race either caller.

**Fix.** Keep a regression that gates both orders using `WriteOnce` and pruning. Assert that each losing transaction waits and that the deletion-first retry executes one new mutation.

## Grade

**7 out of 10.** The confirmed feed and receipt ordering checks passed on the original code. The pruning count error and the proven test failures warrant revision.

## PostgreSQL checks and output

All Go tests ran through `deploy/test/with-postgres.sh`. The wrapper supplied PostgreSQL 16.16. PostgreSQL 17 was unavailable in the local fixture.

- **Commit outcomes from the final plan review.** The race-enabled run of `TestFeedCommitErrorOutcome`, `TestFeedStatementTimeoutDuringCommit` and `TestWriteOnceAfterLostCommitReply` passed in 15.029 seconds. The held three-second lock timeout returned state code `55P03`, `OutcomeRolledBack` and a retryable error. The statement timeout left commit waiting until the gate opened. The lost reply, caller cancellation and caller deadline returned an unknown outcome. After a lost reply, a keyed retry replayed the zero-byte reply with the earlier fingerprint encoder. The unkeyed callback ran once.
- **Full Store suite.** The planned `go test -race -count=1 -v ./src/server/internal/store/...` ran with `GOFLAGS=-p=1 GOMAXPROCS=2`. Run 1 printed `ok` for Store in 103.748 seconds and its migrated fixture package in 1.530 seconds. Run 2 printed `FAIL` for Store in 124.906 seconds with the database teardown error in post-b 3. The migrated fixture package printed `ok` in 2.001 seconds. The focused runs established post-b 1 separately. The original orchestrator failure record had no test name, so the original failure remains unattributed.
- **Integration and connection budget from the final plan review.** The clean copy's head was the same commit before and after check 12. The wrapper provided an accepted fixture endpoint. `max_connections` was 100, with three reserved superuser slots and a sampled peak of eight client backends including the sampler. No Store test reported the too-many-connections code `53300`. `go build ./...` exited zero. Combined C12 exited 1 because the root suite exited nonzero in this sandbox outside the reviewed package. An ordinary no-fixture Store run printed `ok` with database cases skipped. C12 refused the same missing setting.
- **Direct feed hook.** The clean copy printed the tested commit and `PASS C12 feed-lock mutation: writer B became visible before writer A`. For this run, `GOFLAGS='-p=1 -buildvcs=false'` disabled Go's version control metadata stamping in the temporary archive. Stamping initially failed because the sandbox exposed an unusable parent Git marker. An untracked Store stub caused the hook to refuse a dirty scratch checkout with exit 1. Unsetting the fixture variable caused the direct hook to refuse with exit 1.
- **Writer isolation and notification.** A scratch run held the feed lock with eight waiting writers. A read and a separately admitted unrecorded write completed within one second. All eight waiters returned retryable lock timeouts. A subsequent 100 concurrent writes produced 5,000 feed rows and exactly 100 flush batches. A write with 3,000 references returned its first `Record` error at 257 and rolled back. A raw listener received 1,000 notifications and each ensuing query saw the corresponding committed position. The follower, open Watch streams and live restore scenarios need the later Store and Watch items.
- **Retention and replay.** The frozen checks covered a delayed commit, future insertion time and a stale row behind a retained prefix. Both public receipt lock orders passed in the scratch run. Advancing the cutoff by 25 hours deleted 1,026 observed receipts in one sweep and allowed the original key to run a second time.

## Mutations caught by the named tests

All rows below ran on `bcf920b3`. Each mutant exited 1 in its listed test. The clean direct feed hook additionally required the precise visibility assertion before accepting lock removal.

| Mutation | Test output |
|---|---|
| Queue the feed lock after the inserts. | `TestFeedPositionsFollowCommitOrder` printed `feed sequence reached 2 before writer A committed, want 1` at `feed_test.go:229`. |
| Use a session lock for feed writes. | All three cases in `TestFeedLockEndsWithTransaction` reported an advisory lock remaining at `feed_test.go:528`. |
| Move deferred domain checks behind the feed lock. | The deferred foreign-key case in `TestFeedChecksDeferredDomainConstraintsBeforeLock` found a feed lock when the writer was blocked at `feed_test.go:622`. |
| Delete the advisory receipt lock. | `TestWriteOnceConcurrentRetry` reported a retry finishing before the original commit and two callback invocations at `receipts_test.go:107`. |
| Use a session lock for receipt writes. | All three cases in `TestWriteOnceReleasesKeyLock` reported a remaining lock at `receipts_test.go:465`. |
| Bypass the receipt digest comparison or remove its length frames. | `TestWriteOnceMismatch` received the stored reply where it expected a conflict at `receipts_test.go:152` and `receipts_test.go:159`, respectively. |
| Ignore the retained feed prefix. | `TestPrunePreservesFeedPrefix` reported the missing blocked-prefix diagnostic at `prune_test.go:171`. |
| Remove the watermark update. | `TestPruneCutoffAndWatermark` reported zero deletions and a database error at `prune_test.go:100`. |
| Expire feed rows and receipts using insertion time. | `TestPruneStartsAgeAfterCommit` reported one deletion of each kind where it expected zero at `prune_test.go:205`. |
| Stop after one feed stamping or deletion page. | `TestPruneCutoffAndWatermark` reported only 512 newly stamped rows at `prune_test.go:115` and only 512 deleted feed rows at `prune_test.go:100`, respectively. |
