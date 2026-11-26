# Review of the plan go-s1-server-store, round 1, lens logic

- Task identifier: review-go-s1-server-store-r1-logic
- Nonce: a3fe4c
- Model and effort: claude-sonnet-5-5, high
- Text reviewed: `.plans/plans/go-s1-server-store/plan.md` as it stood on 2026-11-26, with `decisions.md` and `scouting.md`.
- Limit of this review: no PostgreSQL server program is installed on this machine, only client tools, and `LGTM_TEST_POSTGRES` is unset. Every statement about PostgreSQL behavior below comes from reading the plan, the design and the PostgreSQL manual, not from a live run. Findings 1 and 3 name checks that the build must run on a real server.

## Verdict

REVISE

## Top finding

The test that proves commit order cannot tell a correct flush from one that allocates its position before it takes the lock, so the one rule that the whole feed depends on is proven only for the mutation "remove the lock".

## Findings

### logic-r1 1. The commit-order test does not catch a lock taken after the insert

- Severity: MAJOR.
- Tied to: C12.
- Evidence: `plan.md:250-257` gives the order lock, insert, notify, and `plan.md:263` gates writer A at the commit, after both its insert and its lock. The only mutation named is removal of the lock (`plan.md:267`, `plan.md:340`). `plan.md:264` pauses writer A in its callback, before the flush, so it does not exercise the order of the flush statements either.
- Failure scenario: An implementation flushes with the insert first and the lock second, for example by letting the identity default fire in an earlier statement or by reordering the statements. Writer A inserts and takes position 1, then waits for the lock behind the gate at commit. Writer B inserts and takes position 2, then blocks on the lock. The test sees B blocked by A and invisible, so it passes. All three item 4 tests pass. In production, A can be delayed between its insert and its lock. B then takes the lock, commits position 2, and a reader that reaches 2 never sees position 1 when A commits later. This is the failure that the design's "Positions must follow commit order" section exists to prevent.
- Fix: Add one assertion to `TestFeedPositionsFollowCommitOrder`. While B is blocked behind A, read `last_value` of the feed identity sequence from `pg_sequences` on the observer connection, and assert that it still equals A's position, which proves that B has not allocated a position before it holds the lock. Add a second named mutation to the risky paragraph: move the lock statement after the insert, and require that the same test fails. Optionally, also add a database guard that makes the wrong order impossible. A before-insert trigger on `feed` can raise an error unless `pg_locks` shows the feed advisory lock granted to the current backend, so that any code path that inserts without the lock fails at once. The sequence assertion is the required part, and the trigger is the optional part.

### logic-r1 2. Lock waits have no bound, and every waiter holds a pool connection

- Severity: MAJOR.
- Tied to: D3.
- Evidence: The feed lock is taken at `plan.md:251` and held through the inserts, the notification and the commit (`plan.md:248`). The per-key lock is taken at `plan.md:295` and held across the whole domain callback. `plan.md:103` and `plan.md:259` set no lock timeout, statement timeout or idle-in-transaction timeout, and `plan.md:103` bounds only the rollback cleanup. The plan never says what a slow writer blocks. The pool size and a reserve for readers are not stated (`plan.md:105` asks only for at least four connections in tests).
- Failure scenario: One writer stalls between its lock statement and its commit, for example on a disk flush, a half-open client socket or a later synchronous standby wait. Every other write that records a change queues on the feed lock, and each queued write holds one pool connection while it waits. The same happens to a client that retries a slow keyed call every second, because each retry waits on the per-key lock with its own connection. When the pool is exhausted, `Read` cannot get a connection. Watch snapshots, the reader's poll and the pruner then fail too, so a single slow commit takes the whole server down until the holder ends. A wait with no caller deadline never ends by itself.
- Fix: State in the plan that a slow feed writer delays every writer that records a change, and nothing else. Then bound it. Run the flush statements, and the receipt lookup, with `SET LOCAL lock_timeout` and `SET LOCAL statement_timeout` of a few seconds, and map a lock timeout to a retryable error such as Connect `aborted`. Send the lock, the inserts and the notification in one `pgx.Batch` round trip, so that the lock is held for one round trip plus the commit and not for one round trip per change. Cap the writers' share of the pool so that at least one connection stays free for `Read`, the reader and the pruner. Add a test: hold the feed lock on a raw connection, call `Write` with a short timeout, assert that it returns the lock error, that its connection returns to the pool, and that `Read` still works.

### logic-r1 3. Nothing proves that the lock is released on cancel, panic or rollback

- Severity: MAJOR.
- Tied to: C12.
- Evidence: The design requires a transaction-level lock, and the plan names it at `plan.md:251`. `plan.md:265` forces one commit-time failure, then commits a second write, and `plan.md:110` tests cancel and panic only in the pool item, where no lock exists yet. The mutation proofs remove the lock but never replace it with the session-level lock.
- Failure scenario: An implementation uses `pg_advisory_lock` and unlocks after the commit, which passes every listed test. When a write is cancelled or panics after the lock statement, the unlock is skipped. The pooled connection keeps the session lock, and every later feed write on any other connection waits forever. The failing test in `TestFeedRollbackAndNotification` would not notice, because its second write probably reuses the same pooled connection, and PostgreSQL grants a session its own lock again.
- Fix: Add a test that, for each of three failures (context cancelled after the lock statement, a panic in the flush path, and a commit-time failure), checks `pg_locks` for advisory locks held by any backend and then runs a write on a different pooled connection that must finish within a deadline. Add the mutation "replace the transaction-level lock with the session-level lock and a deferred unlock" to the risky paragraph. Run the same check for the per-key lock of item 5.

### logic-r1 4. A failing reader leaves open streams silently stale

- Severity: MINOR.
- Tied to: plan.
- Evidence: `plan.md:385` says that poll errors defer to the next wake or poll and that errors can be reported at the lifecycle boundary. `plan.md:420` has no rule for subscribers while the reader fails.
- Failure scenario: The reader's connection breaks while the writers' pool still works, or a migration lock holds its queries. For minutes no new row reaches the follower. Every open Watch looks healthy and delivers nothing, so the website shows old data without any sign. It repairs itself when the reader recovers, but the person looking at the screen cannot tell.
- Fix: Define a staleness bound. After a fixed number of consecutive failed reads, for example the last successful read older than ten seconds, the follower cancels every active subscriber with a retryable cause, so that the website opens the call again, and it reports the failure once. Add a test with the listener disabled and every read failing.

### logic-r1 5. The cut-off at 2,048 changes is stated three ways and not exactly

- Severity: MINOR.
- Tied to: D5.
- Evidence: The design says "falls 2,048 changes behind". `plan.md:352` says "At 2,048 unacknowledged changes". `plan.md:420` says "2,048 actual committed rows follow its acknowledged cursor". `plan.md:422` says the count "includes a pending matching row", and decisions row 15 says "including an in-flight row". `plan.md:426` says "At 2,047 rows assert the slow context is still open".
- Failure scenario: Two builders read the sentences differently. One counts the in-flight row inside the 2,048, and the other counts it on top. The test commits 2,047 and 2,048 rows from different starting points, and a boundary change from `>=` to `>` can pass or fail by one row depending on the reading.
- Fix: Replace the sentences with one rule and one worked example. For an acknowledged cursor c, the subscriber is cut off when the follower has read the row at the 2,048th committed position after c, counting the in-flight row, so that rows c+1 up to c+2047 leave it open and row c+2048 cuts it off. Say that the count covers every row of the feed, not only the rows of the subscriber's topic and scope, and that it covers only rows the follower has read. Say that the in-flight row counts once. Make `TestFollowerLagAt2048` commit rows one at a time around that boundary.

### logic-r1 6. Empty values of a request and of a source do nothing visible

- Severity: MINOR.
- Tied to: plan.
- Evidence: `plan.md:276-281` and `plan.md:293` reject only a bad key. An empty `Caller` or `Procedure` is not mentioned. `plan.md:437-441` and `plan.md:464` do not reject an empty `Topic` or a missing `Snapshot` or `Load`.
- Failure scenario: An adapter forgets to fill `Procedure`. The digest then covers only the caller and the body. Two different calls with the same key and the same body return the first call's stored reply for the second, and the second never runs. In the Watch case, a `Source` with an empty topic matches no row, because the feed's check constraint forbids an empty topic, so `Serve` sends an empty snapshot and then stays silent forever.
- Fix: Reject an empty `Caller` or an empty `Procedure` on a keyed call with a named error before any lock. Reject an empty `Topic`, a nil `Snapshot` and a nil `Load` in `Serve` before it registers the subscription. Add one subtest for each.

### logic-r1 7. Cheap wrong implementations pass the tests of items 5 and 7

- Severity: MINOR.
- Tied to: C12.
- Evidence: For item 5, `plan.md:293` demands a length prefix before each field, and `plan.md:300` changes one field wholesale in each subtest. For item 7, `plan.md:383` demands that a failed read keep the last applied position, and `plan.md:392` makes the read fail before any page has been applied.
- Failure scenario: A digest that concatenates caller, procedure and body without lengths passes, and the pair (caller "ab", procedure "c") then collides with (caller "a", procedure "bc"), so one request is answered with another's reply. A reader that resets its cursor to its starting position after any failed read passes too, because the starting position and the last applied position are the same when nothing has been applied. After a failure in the middle of a long catch-up, it delivers the first pages again.
- Fix: Add to `TestWriteOnceMismatch` a subtest that moves one byte across the caller and procedure boundary and asserts a conflict. Add to `TestFeedReaderCatchupAndRetry` a case with more than 1,024 rows, where the connection is broken with `pg_terminate_backend` after the first page has been applied, and assert that each row is delivered exactly once and in order after the next poll.

### logic-r1 8. A single future timestamp can stop pruning for as long as the clock error

- Severity: MINOR.
- Tied to: D5.
- Evidence: `plan.md:321` deletes only the consecutive expired beginning of the oldest page and stops at the first unexpired row, and `plan.md:326` tests exactly this. `plan.md:153` fills `written_at` from the wall clock. The design says "Rows older than one day are removed from the feed".
- Failure scenario: The server's clock is wrong by two days for one minute, for example after a virtual machine resumes before the Network Time Protocol (NTP) correction. A row is written with a timestamp two days ahead. After the correction, that row and every later row cannot be removed for two days, the table grows, and the sweep returns zero counts and no error, so nobody learns of it. A step of one year blocks pruning for a year.
- Fix: Choose the pruning position differently. Take the highest position on the page whose `written_at` is before the cutoff, and delete every row up to and including it, then set `pruned_through` to that position. The watermark still covers an ordered prefix, so the cursor rule is unchanged, and a future-dated row is removed together with the first later row that expires, so the stall lasts at most one retention period. Rows younger than a day can then be removed early by the size of the clock error, which no reader needs, because a cut-off reader reopens with a fresh snapshot. Rewrite `TestPrunePreservesFeedPrefix` for this rule. Also have `Prune` report when the oldest retained row is older than twice the retention.

### logic-r1 9. The plan does not say which goroutine owns the reader's cursor

- Severity: MINOR.
- Tied to: D5.
- Evidence: `plan.md:385` says the reader "polls every two seconds with a separate timer from its notification path". `plan.md:383` advances the cursor per iteration. No sentence says that one goroutine runs both paths, and `plan.md:391` never has a notification and a poll deliver the same row at once.
- Failure scenario: A builder runs the poll and the wake drain in two goroutines that share the cursor. Both read the page after position 100 at the same moment, and both apply rows 101 to 512. Subscribers see each row twice, and the ring counts them twice, which brings the 2,048 cut-off closer. If the second page is applied first, the ring holds rows out of order, and a subscriber that ignores rows up to its cursor skips a row.
- Fix: State that one goroutine owns the cursor and runs a select over the timer and the wake channel. State that the follower rejects a page whose first position is not greater than the last applied position. Add a test that sends a wake and a poll signal together with the same new row and asserts one delivery.

### logic-r1 10. The first start on an empty database is not tested for concurrency

- Severity: MINOR.
- Tied to: C12.
- Evidence: `plan.md:142` requires the startup lock before the ledger is created. `plan.md:178` bootstraps the ledger once before it starts two concurrent calls. `plan.md:182` names only the mutation "remove the startup lock".
- Failure scenario: An implementation creates the ledger with `CREATE TABLE IF NOT EXISTS` first and takes the lock second. It passes every listed test. When an old and a new server process overlap on a fresh database, both create the ledger at the same moment, and one fails with a duplicate catalog entry, so the start fails and needs a restart.
- Fix: Add a case that runs two `Migrate` calls on a completely empty database and asserts that both succeed. Add the mutation "create the ledger before the lock" to the risky paragraph.

### logic-r1 11. A record that moves to another scope stays on the old scope's watchers

- Severity: MINOR.
- Tied to: D5.
- Evidence: `plan.md:246` says "A watched record uses one topic and one scope per entry". `plan.md:464` skips rows whose scope differs. The design defines scope as the project the record belongs to.
- Failure scenario: A record changes project from P1 to P2. The writer records `saved` under P2. A watcher of P1 skips that row, so its list keeps the record until it reopens the call.
- Fix: State in the `Record` documentation that a change of scope records `removed` under the old scope and `saved` under the new scope in the same write, or state that a record's scope never changes. Add one `watch_test.go` case for the rule that is chosen.

## Answers to the attack questions

Questions of the lens file:

1. Positions come from one identity column that is read under the feed lock, so they follow commit order and cannot tie, because they are the primary key. Rollbacks and crashes leave gaps, which the plan marks as harmless and tests. A duplicate is prevented by `position > $1`. A wrap needs 2^63 rows. A restart keeps the sequence, and the follower rebuilds its ring from the head. Timestamps can disagree with positions, see finding 8. Migration versions are contiguous per part.
2. A crash in the middle of a write rolls back both state and feed rows, and a retry with the same key is safe. An ambiguous commit is retried with the key. The database down and a stuck lock are not bounded, see findings 2 and 4. Clocks use the database clock only, which is consistent on one node, see finding 8. An expired token is outside this plan.
3. The lag rules form a table of four causes (count at 2,048, cursor below evicted, cursor below pruned, a cursor ahead of the follower), and each cell has a stated result. The empty cells are the reader failing (finding 4) and the exact count (finding 5). A receipt that is pruned while a replay holds its key lock lets the replay run again, and both outcomes are legal at the retention edge.
4. The empty change, topic and key are rejected in Go and by check constraints. The empty scope means the whole server. Position 0 means an empty feed. An empty reply is stored as zero bytes. Empty `Caller`, `Procedure` and `Source` fields are not handled, see finding 6.
5. Two feed writers serialize on the feed lock. Two requests with one key serialize on the per-key lock. Two pruners serialize on the retention row. Two migrators serialize on the startup lock. The pruner and the writers touch different rows, because every in-flight position is higher than every committed position. A replay and the pruner on one receipt are legal either way. The cursor owner of the reader is open, see finding 9. One consumer per subscription is stated and not enforced.
6. Removing the feed lock, the per-key lock, the digest comparison, the poll, `>=`, the watermark update, the prefix rule, the actual-row count and the one-transaction snapshot would each fail a named test. The mutation of item 1 depends on a test of item 9 that does not exist at item 1's close, which is acceptable. Wrong changes the tests miss are in findings 1, 3, 7 and 10.
7. Item 1: nothing found, I checked the cancel, panic and read-only tests. Item 2: nothing found, the exact timestamp and byte test is tight. Item 3: lock after ledger, finding 10. Item 4: lock after insert and session-level lock, findings 1 and 3. Item 5: a digest without lengths, finding 7. Item 6: nothing found, both tests fail for an age-only delete and for a missing watermark. Item 7: the cursor reset, finding 7. Item 8: nothing found, the gap and boundary tests are tight. Item 9: subscribing after the snapshot passes every test, because the ring keeps 2,048 rows, which shows that the rule "register first" at `plan.md:420` protects nothing. State what it protects or drop it.
8. A trigger that checks the feed lock (finding 1), a `before update` trigger that refuses a lower `pruned_through`, a rule on scope changes (finding 11) and non-empty requests (finding 6) would each make a wrong state impossible. Positions already cannot be supplied by a part, because the column is `GENERATED ALWAYS`.

Questions of the task file:

a. Positions follow commit order, because the lock is transaction-level, is taken in the flush after the callback and is released after the commit is visible. The plan states the order of the statements and a test proves the lock exists, but not that it comes before the allocation (finding 1) or that it is released on every path (finding 3). A slow writer blocks every writer that records a change, and through the pool it can block reads (finding 2).
b. The follower cannot miss a row through a lost notification, because the two-second poll reads everything after the cursor. It cannot miss a row through a long transaction, because a writer allocates its position only under the lock, so no lower position can commit later. A rollback gap is harmless. A restart rebuilds the ring from the head. A repeat is prevented by `position > $1`, except where two goroutines share the cursor (finding 9). The cut-off at 2,048 is not stated exactly (finding 5).
c. The same key with a different body gives `ErrRequestKeyConflict` and no write. Two identical requests at one moment serialize on the per-key lock, and the second returns the stored reply, but its wait is unbounded (finding 2). A crash between the write and the receipt is impossible, because both are in one transaction, and an ambiguous commit is retried with the key. After pruning at 24 hours, the same key runs again, which the plan states, and a replay at the exact retention edge may do either.

## Post-build checks

- On the real server, block writer B behind writer A and read `pg_sequences` for the feed identity. Expected: the value equals A's position until A commits.
- With `pg_terminate_backend`, end the backend of a writer that holds the feed lock in the middle of its flush. Expected: the lock is gone at once and the next write succeeds with a higher position.
- Run the full suite with the mutations "lock after insert", "session-level lock", "ledger before lock" and "reader cursor reset". Expected: each fails a named test.
- Measure the lock hold time with 100 concurrent writers that record 50 changes each. Expected: the hold time is dominated by the commit, and the pool keeps a free connection.
- Confirm on the target PostgreSQL version that a single multi-row `INSERT ... VALUES` into an identity column assigns positions in row order. Expected: yes, and the flush relies on it.

## Grade

6 out of 10. Blockers: 0. Majors: 3. Minors: 8.
