# Post-build review C: feed reader, follower and watch serving

Task identifier: postbuild-go-s1-server-store-c. Nonce: ee4037.
Reviewer model: claude-sonnet-5-5, high effort. I did not build the code or write its tests.
Branch: `sprint-0`. Scope: plan `go-s1-server-store`, items 7, 8 and 9, and the repair of the pruner's count in commit `9b5048d9`.

## Verdict

**REVISE.**

The top finding is that the only guard that ends streams while a feed page is stuck in progress, the production ten-second staleness timer of the follower, is not exercised by any test, because every test injects a clock and the mutation that removes the timer survives.

I found no defect in the production code of the three items by reading it, by running it, or by the mutations below.
The verdict is REVISE only because two mechanisms the plan calls risky are not proven by the frozen tests.

## Findings

### post-c 1 (MAJOR): the production staleness timer is untested

**Statement.** The goroutine `watchStaleness` starts only when `options.now == nil`, and every follower test passes an injected clock, so the production timer never runs under test.

**Evidence.** `src/server/internal/store/follow.go:102` starts `watchStaleness` only when `options.now == nil`. `follow_test.go` (`flStart`) always sets `now`. The mutation `no_staleness_watcher` (replace the call with a no-op) leaves `TestFollower*` and `TestWatch*` green.

**Failure scenario.** The timer is the only thing that ends streams and refuses new subscribers while a page read hangs without returning an error (`checkStaleness` is otherwise called only from `onReadError`). A change that breaks the timer, for example by never starting it, ships green, and then a stalled database leaves every watcher silently frozen on stale data.

**Fix.** Add one test that starts the follower with the production clock path, or with an injectable ticker for `watchStaleness`, stalls a page, and requires the subscribers to end with `ErrFeedUnavailable`. Design: "Following the feed"; plan item 8.

### post-c 2 (MAJOR): no test proves that a notification wakes the reader

**Statement.** Removing the wake-up on a notification leaves `TestFeedReaderNotificationAndPolling` green in three consecutive runs, so the test does not isolate the notification path.

**Evidence.** `read_feed.go:241` is the `signal()` after `WaitForNotification`. With that line deleted, the first part of the test (`read_feed_test.go:169-177`, "a committed row arrives on the notification alone") still passes. The likely reason is that the wake from the listener registration (`read_feed.go:233`) is consumed late enough to see the first written row, so the row arrives without any notification. I did not prove that explanation.

**Failure scenario.** A reader that only ever polls (with a two-second delay) and never reacts to `NOTIFY` passes every test, and every watch update then lags by up to two seconds. The plan's point of the listener, low latency, is unproven.

**Fix.** Write the row only after the reader has finished its registration read and is idle, for example by waiting for an applied page after registration with an empty feed, then writing and requiring the row without any tick. Plan item 7.

### post-c 3 (MINOR): the stored lag of a subscriber that starts behind is not asserted

**Statement.** `Start` stores `sub.lag = lag`, and the mutation that stores zero survives all follower and watch tests.

**Evidence.** `follow.go:263`. The mutation `start_lag_zero` passes `TestFollower` entirely. `TestFollowerStartCountsRowsAlreadyApplied` kills the `>` instead of `>=` mutation on the local variable at `follow.go:257`, but nothing observes the stored counter afterwards.

**Failure scenario.** A subscriber that starts with 2,000 rows already applied behind its cursor is cut only after 2,048 further rows instead of 48. The ring has the same size, so eviction partly hides the difference. The behaviour is wrong but mostly harmless, which is why this is MINOR.

**Fix.** Add one case that starts a subscriber 2,000 rows behind, appends 48 rows, and requires `ErrWatchLagged`. Plan item 8.

### post-c 4 (MINOR): listener errors are swallowed

**Statement.** `listenForFeed` drops every registration and wait error without a log line or a callback.

**Evidence.** `read_feed.go:230-249`. On an error the loop only sleeps `feedPollInterval` and retries.

**Failure scenario.** If `LISTEN` is refused permanently, for example because the role loses a privilege, delivery silently degrades to the two-second poll and nobody sees it. Rule R10 (no silent failure) applies.

**Fix.** Report the first error of each outage through the follower's error callback or the logger, without including the connection string.

### post-c 5 (MINOR): one global snapshot slot has no load budget

**Statement.** All new watches share one admission slot, and a reconnect storm makes waiters fail after two seconds with `ErrFeedUnavailable`.

**Evidence.** `read_feed.go:26,67-88` (`activeSnapshots`, `snapshotAdmissionTimeout`). The plan mandates this design, so this is an observation about its limit.

**Failure scenario.** After a server restart, 200 browsers reconnect. Each snapshot holds the slot for its full read, the later ones time out, and they retry at the same moment. The retry storm repeats.

**Fix.** Document the limit, and have the client add jitter to its reconnect (plan, later wave).

### post-c 6 (MINOR): every matching change needs a main-pool connection per watcher

**Statement.** `readChange` opens a read on the main pool for each matching change and each watcher.

**Evidence.** `watch.go:142-179`. When the pool is saturated, acquisition ends the stream with `ErrAuthorizationUnavailable` after two seconds.

**Failure scenario.** A burst of writes with many watchers uses all connections, streams end, and every reconnect then needs a snapshot from the single slot (post-c 5).

**Fix.** Record the limit in the plan; consider one authorization read per page instead of per row, in a later wave.

### post-c 7 (MINOR): a callback that ignores its context blocks `Serve` forever

**Statement.** `Serve` waits for the reauthorization goroutine at exit, and that goroutine waits inside `Authorize`.

**Evidence.** `watch.go:79-87` (`<-authorizationDone`). The comments on `Source` and `Serve` state the contract that callbacks must honor the context.

**Failure scenario.** One `Authorize` that ignores its context holds a goroutine and a pooled connection for good.

**Fix.** The contract is documented. A bounded wait that abandons the goroutine after a grace period would remove the hazard.

## Mutation results

I ran each mutation in a scratch copy under `deploy/test/with-postgres.sh`, targeting the tests of the item. KILLED means at least one test failed.

| Mutation | Result |
| --- | --- |
| Cursor `>=` instead of `>` in the page query | KILLED |
| No polling | KILLED |
| Lag by position difference instead of counting rows | KILLED |
| Lag cut at `>` instead of `>=` on append | KILLED |
| Lag cut at `>` instead of `>=` on `Start` | KILLED |
| Rejected page resets the staleness clock | KILLED |
| Cursor reset to zero after a read error | KILLED |
| No page deadline | KILLED |
| Snapshot reads the head after the load | KILLED |
| Head ignores the pruning floor | KILLED |
| No snapshot admission | KILLED |
| No authorization on a matching change | KILLED |
| No reauthorization tick | KILLED |
| No lag decrement on `Ack` | KILLED |
| No cut of subscribers below the pruned floor | KILLED |
| Stale follower still accepts subscribers | KILLED |
| No stale-state recovery | KILLED |
| No staleness check from a read error | KILLED |
| `readChange` ignores the scope | KILLED |
| No `Ack` in `Serve` | KILLED |
| Pages read on the main pool | KILLED |
| Pruner counts rows of every batch, not only committed ones | KILLED |
| No staleness watcher | **SURVIVED** (post-c 1) |
| No wake on a notification | **SURVIVED** (post-c 2) |
| `Start` stores lag zero | **SURVIVED** (post-c 3) |
| `Start` ignores `evictedThrough` | SURVIVED, equivalent: eviction needs a full ring, and a full ring already gives lag 2,048 |

In my first batch two mutations (`no_snapshot_admission`, `prune_count_all_batches`) failed to compile and so only looked killed. I corrected them and reran them; the table shows the rerun.

## Places that no reviewer read before the build

- **Pruner count, `9b5048d9`.** The count is taken inside the callback and added only after the maintenance transaction commits. `TestPruneCountsOnlyCommittedFeedBatches` fails when the count covers every batch. Verified.
- **Cancelled, failing or panicking snapshot loader (Lag and reconnects check).** `TestSnapshotAdmissionLeavesReadCapacity` covers all three and passes; removing the admission slot fails it. Verified.
- **Read-saturation floor of the fixture.** `TestFollowerProgressesUnderReadSaturation` holds exactly eight connections and passes; moving page reads to the main pool fails it. Verified.
- **Restore to an earlier state.** Outside wave 1 by owner decision. I did not run it. By reading, an old stream skips restored positions through its cursor and a reopen gives a fresh snapshot, which matches the builder's note.
- **The pgx dependency row.** The diff does not change `go.mod`; I did not re-evaluate the dependency.

## Test run

I ran `go test -race -count=1` for the store and watch packages through `deploy/test/with-postgres.sh` with one package at a time.

- The store package took 119 seconds and had one failure, the known `TestFollowerPrunedCursor/evicted_history` at `follow_test.go:348`. I do not report it.
- The watch package passed in 14.9 seconds. The `storetest/migrated` package passed.
- `gofmt -l` and `go vet` report nothing for both packages.

## Code against the plan

Ordering (advisory lock, cursor exclusive, rejection of a page whose first row is not above the cursor), the lag arithmetic, the `Start` conditions, the single-transaction snapshot with the head read first, the `AfterFunc` cleanup of subscriptions, the authorization window around each change, and the pruned-floor handling all match the "Done when" rows and the text of items 7, 8 and 9. I found no unexplained difference.

## Grade

**7 of 10.** The production code looks sound and nearly every mutation is caught. The grade is held down by the two risky mechanisms that no test proves (post-c 1 and post-c 2).
