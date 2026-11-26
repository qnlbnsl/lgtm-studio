REVISE

# Round 1 review of the plan go-s1-server-store, seat critic

- Task identifier: review-go-s1-server-store-r1-critic
- Nonce: 64e36a
- Reviewed text: `.plans/plans/go-s1-server-store/plan.md` as it stands, which equals `attack/plan-v1.md`.
- Lens: `.foundry/go-rewrite/lenses/critic.md`.
- Finding prefix: `critic-r1`.

## Top finding

The one global feed lock is held from the first flush statement to the commit with no timeout, no minimized hold window and no stated bound on what a slow writer blocks (critic-r1 1).

## What was and was not verified

- Read in full: the plan, `decisions.md`, `scouting.md`, the design, the epic, the build-loop rules named by the plan, issues 1632 to 1635, and the sprint 0 plan where this plan depends on it.
- Spot-checked the anchors of `scouting.md`. The files `packages/engine/src/log/migrations.ts`, `packages/engine/src/log/EventLog.ts` (it has `RequestIdReused` at line 80), `src/mock-server/src/engine.ts` (it has `watchClients` at line 164), `src/frontend/src/server/sync.ts` and `floorplan/plan.json` exist.
- `src/server/` and the root `go.mod` do not exist yet, which matches `scouting.md`. `src/contracts/proto/lgtm/v1/common.proto` does not exist yet, and the sprint 0 plan creates it.
- No PostgreSQL server was available. Every statement below about lock behaviour rests on the documented PostgreSQL semantics and on the text of the plan. None of it was run.

## Findings

### critic-r1 1. MAJOR. Statement D3. The hold window of the global feed lock is unbounded and not minimized.

- Evidence: `plan.md:248-259` (flush statements and lock text), `plan.md:263-265` (tests), `plan.md:246` (the buffer of `Record` calls has no cap), design `docs/specs/2026-11-25-go-rewrite-foundation-design.md:148` ("The lock is held only from that step to the commit"). A search of `plan.md` for `lock_timeout`, `statement_timeout`, `SET CONSTRAINTS` and `unnest` finds nothing.
- Failure scenario one: writer A runs the lock statement and then stalls. Its connection hangs, its process is paused, or its commit waits. Every other writer that has pending feed changes now waits on A with no deadline except the caller's own context. One stuck writer stops all feed-writing writes of the server.
- Failure scenario two: a part uses a deferrable foreign key. At commit, A's deferred check needs a row lock on a parent row that writer Y holds. Y finished its callback and now waits for A's feed lock. This is a deadlock across a row lock and an advisory lock. PostgreSQL ends it after its deadlock timeout by aborting one of the two writers, so a valid write fails and the other stalls for about one second. The plan's own `TestFeedRollbackAndNotification` uses a deferred trigger, which shows that checks at commit are expected.
- Failure scenario three: the flush runs one `INSERT` per buffered change (line 253 shows one `VALUES` row), so a write of N changes holds the lock for N round trips.
- Prescribed fix:
  - (a) Run the pending deferred checks before the lock statement, for example with `SET CONSTRAINTS ALL IMMEDIATE`, and say so in the item. Keep the test gate trigger working in that mode.
  - (b) Insert all buffered changes in one multi-row statement. Assert in `TestFeedRollbackAndNotification` that two changes of one write get ascending positions in recorded order.
  - (c) Run `SET LOCAL lock_timeout` for the lock statement, with a stated value and a named error that the caller can map.
  - (d) State a cap on buffered changes per write and test the refusal.
  - (e) Add one sentence that says what a slow writer blocks: writers with pending changes, from the holder's lock statement to its commit. Writers with no pending changes and all readers never wait.
  - (f) Add `TestFeedLockDoesNotBlockUnrecordedWrites` and a test that cancels a writer waiting on the lock.

### critic-r1 2. MAJOR. Statements D3 and C12. The cold concurrent migration start is untested, so the lock-before-ledger order is unprotected.

- Evidence: `plan.md:142` ("Acquire `SELECT pg_advisory_xact_lock(841941100002::bigint)` before creating the migration ledger"), `plan.md:178` (the test says "Bootstrap the store ledger once"), `plan.md:182` (the only mutation removes the lock), `plan.md:191` (the plan rates this risk High).
- Failure scenario: a build that creates the ledger first and takes the lock second passes all three migration tests, because the test has already created the ledger. In production, two server processes starting for the first time on an empty database race on `CREATE TABLE IF NOT EXISTS`. One fails with a catalog error, or both apply the store part.
- Prescribed fix: add `TestMigrateColdConcurrentStarts` on an empty database with no bootstrap. The observer holds the advisory lock 841941100002. Two `Migrate` calls start. Assert through `pg_locks` that both wait on that lock and that `schema_migrations` does not exist yet. Release the lock. Assert that both succeed and the ledger shows `store=1`. Add a second mutation to item 3: move the lock after the ledger creation. The new test must fail.

### critic-r1 3. MAJOR. Statement C12. The tests of items 7, 8 and 9 and the gate test of item 4 cannot be written from the plan, because their private seams and fixtures are not defined.

- Evidence:
  - `plan.md:385-387`: `feedReaderOptions` has no named fields. The text only says that the options let tests "turn off the listener, observe registration and read errors, control poll signals, and set a bounded database read deadline".
  - `plan.md:418`: `startFollower` takes the same options.
  - `plan.md:58, 263, 265, 387`: the words "bounded" and "bounded waits" have no value.
  - `plan.md:427`: "while the reader is paused". No pause seam exists in the options.
  - `plan.md:474`: "wait for the follower to observe the writer". No seam reports which position the follower has seen.
  - `plan.md:474-476`: `Source[T]` fixtures have no table, column, topic or scope names.
  - `plan.md:263`: the gate trigger has no function body, no gate lock key, and no way for a callback to report its backend identifier.
- Failure scenario: the test writer never sees the code. Tests written against stubs invent the field names. The builder, who must keep the frozen tests, invents different ones. The build loop stops at the first compile failure, and the fix reopens frozen tests.
- Prescribed fix: add a "Test seams" paragraph to items 4, 7, 8 and 9. Each gives the struct with field names, types and defaults. Each names the seams for pause, "observed position" and registration. Each gives the numeric value of every wait, for example 10 seconds for a fixture wait and 250 milliseconds for a silence check. Each gives the fixture table, its columns and the topic and scope strings. Name the gate helper and its signature.

### critic-r1 4. MAJOR. Statements D5 and R5. The 2,048 cut-off is stated in seven ways and has no single formula, so its boundary is ambiguous.

- Evidence: design `docs/specs/2026-11-25-go-rewrite-foundation-design.md:170` ("falls 2,048 changes behind"), `plan.md:26` ("2,048 changes behind"), `plan.md:352` ("2,048 unacknowledged changes"), `plan.md:401` (the constant), `plan.md:418` ("retains the last 2,048 observed feed rows"), `plan.md:420` ("when 2,048 actual committed rows follow its acknowledged cursor"), `plan.md:422` ("This includes a pending matching row in the backlog count"), `plan.md:426` ("At 2,047 rows ... At 2,048"), `plan.md:487` ("including the in-flight row").
- Failure scenario one: the test at line 426 does not say what the 2,047 counts: all rows since attach, or rows after the acknowledged cursor, and whether the in-flight row is one of them. An implementation that counts rows after the in-flight row cuts off one row later. Depending on the reading, it still passes.
- Failure scenario two: the ring has 2,048 slots and the threshold is also 2,048, so the plan never says which rule fires first. Line 420 lists both rules, but no test makes the second one fire on its own.
- Failure scenario three: the count is over the whole feed of every topic and scope (line 418, "process-wide"). One `Write` that records 2,048 or more changes, for example a bulk import, cuts off every open stream at once, including a stream that is fully caught up. Neither the plan nor the design says so, and no test covers it.
- Prescribed fix: state one formula. For example: "lag of a subscriber is the number of feed rows the follower has observed with a position above the acknowledged cursor, and the stream is cut off when lag is at least `MaxWatchLag`. The in-flight row is not acknowledged, so it counts." Replace every other phrasing with a reference to that sentence. Add a boundary table to `TestFollowerLagAt2048` with lag 2,046, 2,047 and 2,048, with and without an in-flight row, and with filtered rows. Add `TestFollowerBulkWriteCutsOffEveryone` with one commit of 2,047 changes (nobody cut off) and one commit of 2,048 (every stream cut off). Say in the plan that the count is global.

### critic-r1 5. MAJOR. Statements R2, R8 and `plan` (item 1 prerequisites). The plan relies on sprint 0 for three things that the sprint 0 plan does not deliver or states differently.

- Evidence:
  - The driver. `plan.md:82` says item 1 "needs the root module and the driver's dependency entries from the Contract team". The sprint 0 plan gives `go.mod` requirements for `connectrpc.com/connect` and `google.golang.org/protobuf` only (`.plans/plans/go-s0-contract/plan.md:161`). Its check script rejects a changed `go.mod` that `go mod tidy -diff` would alter (`.plans/plans/go-s0-contract/plan.md:486-492`). The epic gives `go.mod` and `go.sum` to the Contract team alone and `src/server/` to the Server team alone (`.plans/epics/go-rewrite/epic.md:122-123`).
  - The code for a lagged stream. `plan.md:470` and `decisions.md:23` map lag to `resource_exhausted`. The sprint 0 plan writes into `common.proto` that the code `aborted` means "The caller fell too far behind and opens the call again" (`.plans/plans/go-s0-contract/plan.md:126`). The design does not name a code.
  - The position type. The store uses a signed 64-bit `int64` (`plan.md:237, 382, 450`). The contract uses `uint64 value = 1` (`.plans/plans/go-s0-contract/plan.md:123`). `plan.md:468` leaves the check to "the comparison with sprint 0".
- Failure scenario one: nobody may add `pgx` to `go.mod` for item 1. The Contract team's check refuses an unused requirement ahead of its use, and the Server team may not edit the file. Item 1 stops at its first compile.
- Failure scenario two: the later Machines adapter follows one document and the contract comment says the other, so the website and the server disagree on what a cut-off looks like.
- Prescribed fix:
  - (a) Name who adds `github.com/jackc/pgx/v5` to `go.mod` and `go.sum`, and when. Either the sprint 0 plan adds it with a use, or the epic lets the Server team edit those two files for this one change.
  - (b) Choose one code for lag in this plan and in the sprint 0 `common.proto` comment, and record the decision in both `decisions.md` files.
  - (c) State the conversion between `int64` and `uint64`: positions are at least 1, so a negative value is a programming error and the adapter must refuse it.

### critic-r1 6. MINOR. Plan. Behaviours that the item text states have no test.

- Evidence and what is missing:
  - `Record` validation (`plan.md:246`): empty topic, empty key, invalid text, an unknown change value. No test among `plan.md:263-265` covers it.
  - `Start` and `Ack` misuse, and `Close` being idempotent on both objects (`plan.md:422`). No test among `plan.md:426-428` covers them.
  - `RunPruner` (`plan.md:319`): the startup sweep, the hourly sweep, cancellation, and the error report. Both tests of item 6 call the private `pruneBefore` only (`plan.md:325-326`). The public `Prune` and its cutoff from `clock_timestamp()` have no test, and neither has the loop "until neither batch found another full page".
  - `Snapshot` after the whole feed has been pruned must return `pruned_through` (`plan.md:381`). Only an indirect assertion at `plan.md:325` touches it.
  - `Serve` sets exactly one variant per message (`plan.md:464`). The tests of item 9 do not assert it.
- Failure scenario: each behaviour can regress or ship wrong while all named tests pass.
- Prescribed fix: name one test for each behaviour above, with its input and expected result.

### critic-r1 7. MINOR. Statements D5 and R9. Safe retries: the receipt tests and the retention behaviour have gaps.

- Evidence: `plan.md:299` ("Prove the second backend waits for the first's advisory lock or records a forbidden second callback attempt"), `plan.md:321` ("A concurrent valid replay holding a receipt row lock completes before deletion"), `plan.md:293` (receipt-safe replies are a caller promise), design `docs/specs/2026-11-25-go-rewrite-foundation-design.md:182` ("Receipts older than one day are removed").
- The test at line 299 accepts either of two behaviours. Choose the first: the second backend is blocked by the first, observed in `pg_locks`, and the callback count is one.
- No test makes `Prune` meet a replay. The plan does not say whether the receipt delete waits for a held row or skips it (`FOR UPDATE SKIP LOCKED`). It also does not say whether the wait happens inside the transaction that holds the `feed_retention` row.
- A retry that arrives after the receipt is pruned runs the write again. This is the design's intent. The plan never tells the adapter that the key window is 24 hours, and no test replays a key at the exact cutoff.
- The reply is stored as `bytea` with no size cap.
- Prescribed fix: remove the "or" at line 299. State the receipt delete rule and add `TestPruneWaitsForReplayOfSameKey`. Put the 24-hour window in the comment of `Request`, and add a test for a key replayed just before and just after the cutoff. State a maximum reply size and test its refusal.

### critic-r1 8. MINOR. Plan. Gap states: some states show the person nothing, and the follower fails silently.

- Evidence: `plan.md:105` (the helper "fails the integration test" with no message text), `plan.md:103` (`Open` pings and returns an error, with no message), `plan.md:142` ("Startup returns an error before serving"), `plan.md:385` ("Poll errors defer to the next wake or poll"), `plan.md:408` (`StartFollower(ctx, s)` takes no reporter, while `RunPruner(ctx, report)` at line 316 does), `plan.md:105` (teardown runs through the test framework).
- What a person sees today:
  - Fresh checkout with no `LGTM_TEST_POSTGRES`: a failed test with unspecified text.
  - A role without `CREATEDB`: the same.
  - PostgreSQL down at start: an error with unspecified text.
  - A killed or timed-out test run: `t.Cleanup` does not run, so disposable databases stay behind with nothing to remove them.
  - A listener that never registers, for example on a standby, or poll errors that never stop: open streams stay quiet and the website looks frozen, with no log line.
- Prescribed fix: give each message as one sentence naming the setting and the action. Give the helper a sweep that drops leftover test databases by name prefix and age before it creates its own. Add a reporter to `StartFollower`, or an `Err` method, and state when the follower gives up and cancels its streams.

### critic-r1 9. MINOR. Plan. The place that creates `schema_migrations` is stated twice, differently.

- Evidence: `plan.md:78` and `scouting.md:24` (S3) say `0001_store.sql` creates `schema_migrations`. `plan.md:142-144` say Go creates the ledger with `CREATE TABLE IF NOT EXISTS` before reading it, and the SQL block at `plan.md:146-172` lists `feed`, `feed_retention` and `receipts` only.
- Failure scenario: a builder who follows line 78 puts the ledger in the file, and the cold start of finding 2 breaks.
- Prescribed fix: keep the Go bootstrap. Correct line 78 and S3. State that `Migrate`, not the file, records the `store` row in the ledger.

### critic-r1 10. MINOR. Plan. File ownership and order: shared test helpers have no owner, `Tx` is declared in two places, and item 6 depends on item 5 without need.

- Evidence: `plan.md:263` (a gate helper), `plan.md:271` and `plan.md:365` (items 5 and 7 can run together), `plan.md:307` and `plan.md:398` (items 6 and 8 can run together), `plan.md:84-89` (item 1 declares `Tx`), `plan.md:84` ("Item 4 implements `Tx`"), `scouting.md:25` (S4 puts `Tx` in `feed.go`), `plan.md:307` and the sizing table at `plan.md:563` (item 6 depends on item 5), `plan.md:325` (item 6's tests insert receipt rows with fixture SQL).
- Failure scenario one: two parallel items each add the gate and wait helpers to `storetest/postgres.go` or to their own test files. The result is a merge conflict or two copies.
- Failure scenario two: item 1 and item 4 both believe they own the `Tx` type.
- Failure scenario three: the dependency of item 6 on item 5 lengthens the chain 1, 3, 4, 5, 6 for no reason. Item 6 needs only items 1 and 3.
- Prescribed fix: add `src/server/internal/store/storetest/gate.go` to the files that item 4 creates. Items 5, 7, 8 and 9 import it and do not edit it. Say that `Tx` is declared in `feed.go` by item 4 and that item 1 declares only `ReadTx`. Make item 6 depend on items 1 and 3.

### critic-r1 11. MINOR. Statements D5 and R5. Edge cases of scale and time are neither stated nor tested.

- Evidence: `plan.md:420` (a cursor that is already at the limit is cut off at `Start`), `plan.md:464` (`Serve` takes its snapshot before `Start`), `plan.md:385` ("Switching to a different history requires a restart"), `plan.md:321` (the prefix stops at the first row within retention).
- Failure scenario one: a snapshot read that takes long while 2,048 rows commit makes `Start` return the lag error at once. The website opens the call again with the same slow snapshot and loops. This is unlikely for one user, but the plan does not name the loop or give it an end.
- Failure scenario two: a database restored from a backup has a lower head than the follower's cursor. The reader finds nothing after the cursor and delivers no change until positions catch up, with no error.
- Failure scenario three: a forward step of the clock leaves a row stamped in the future at the head of the retained prefix. Pruning stops until that time passes. The sweep reports zero rows and no error.
- Prescribed fix: state in the plan that a snapshot slower than the lag window is an accepted limit, and that a repeated cut-off at `Start` is reported once. Make the poll compare the head with the cursor and fail loudly when the head is lower. Have the sweep report when the oldest retained row is older than twice the retention.

### critic-r1 12. MINOR. Plan, gate 1 and the writing rules. Housekeeping of the documents.

- Evidence: `decisions.md:12` (row 5 has the status `open`, while row 6 supersedes it), `decisions.md:14-25` (rows 7 to 18 have the status `unanswered`), `plan.md:60` and following (the abbreviation "WS" is never expanded), `plan.md:105, 259, 263, 321` (paragraphs of many claims in single sentences, which breaks "one claim in each sentence" of `AGENTS.md`), `plan.md:568` (the load figures name three vendors), `decisions.md:11` (row 4 names harnesses).
- Failure scenario: gate 1 of the self-check requires zero open rows, so row 5 fails it. The writing rules ask for one claim per sentence and for acronyms expanded on first use.
- Prescribed fix: close row 5 as superseded by row 6. Mark rows 7 to 18 as assumptions the build may rely on, or have the orchestrator settle them. Expand "WS" as "workstream" at its first use. Split the four long paragraphs into numbered steps. Describe load by seat counts, for example "review seats and build pairs", and keep vendor names with the orchestrator's allowance notes. Row 4 records an owner decision, so it may keep its words if the owner used them.

## Answers to the numbered lens questions

1. Test writer without the code: items 1, 2 and 3 are writable. Items 4 to 9 are not fully writable, because of the missing seams, helpers and fixtures (findings 3, 6, 7).
2. Acceptance observations: every "Done when" row and every "Verify" line names tests or a command, and the mutation lines at `plan.md:340` give exit statuses. The bare words are "promptly" (`plan.md:110`), "bounded" (`plan.md:58, 263, 265, 387`), "ordinary operation" (`plan.md:259`) and "as soon as registration is effective" (`plan.md:385`).
3. Contradictions: the place of `schema_migrations` (`plan.md:78` against `plan.md:142-144`, finding 9); the lag code (`plan.md:470` against `.plans/plans/go-s0-contract/plan.md:126`, finding 5); the 2,048 rule stated seven ways (finding 4); `decisions.md:12` against `decisions.md:13` (finding 12). The design and the epic are otherwise followed.
4. Exact files: every item names its files, and all lie under `src/server/`, which the epic gives to the Server team (`.plans/epics/go-rewrite/epic.md:123`). The `go.mod` and `go.sum` entries for `pgx` belong to the Contract team (`.plans/epics/go-rewrite/epic.md:122`), and the sprint 0 plan does not add them (finding 5). The shared test helper has no named file (finding 10).
5. Splitting and order: no item is rated high. Item 6 needlessly waits for item 5 (finding 10). Items 5 and 7, and items 6 and 8, may run together and would both add helpers (finding 10).
6. Gap states: see finding 8. A second run of `Migrate` is covered by the third call at `plan.md:178`. A half-finished earlier run is covered by `TestMigrateFailureRollsBack`. The cold start is not covered (finding 2).
7. Gates: gate 1 fails on row 5 and the unanswered rows (finding 12). Gate 2 passes except for the bare words of question 2. Gates 3, 4, 5, 6, 8 and 9 pass. Gate 7 has no rounds yet, which is expected. Gate 10 passes, and the load arithmetic checks out: 9 + 1 + 2 gives 12, 9 + 4 + 2 gives 15, and 1 + 2 + 2 gives 5.
8. Writing rules: see finding 12. The plan breaks "one claim in each sentence" in four paragraphs, leaves "WS" unexpanded, and names vendors at `plan.md:568`. No private path, address or quotation was found.

## Answers to the task questions

- (a) Commit order with concurrent writers: yes. The lock statement runs after the callback and any receipt insert, and the position is allocated by the insert after the lock (`plan.md:248-259`). The lock is transaction-level, so it is released when the commit ends, and a snapshot sees a prefix of positions. It is taken last, as design line 148 says. What a slow writer blocks: every other writer that has pending feed changes, from the holder's lock statement to its commit. Writers with no changes and all readers never wait. The plan does not state this and sets no deadline (finding 1).
- (b) The follower:
  - A lost notification is covered by the two-second poll. A long transaction cannot hide a lower position, because positions follow commit order and the page query uses `position > $1`.
  - A gap in positions is ignored by design and is tested (`plan.md:428`).
  - A restart loses the ring and the streams. Watchers reopen with a new snapshot. `startFollower` reads the head before the reader starts, so nothing falls between them.
  - A restored database with a lower head is not detected (finding 11).
  - The 2,048 cut-off is not stated exactly (finding 4).
- (c) Safe retries by request key:
  - The same key with a different body, caller or procedure gives `ErrRequestKeyConflict` with no callback (`plan.md:295, 300`).
  - Two identical simultaneous requests serialize on the per-key lock. The second sees the receipt and replays it. The test accepts two outcomes (finding 7).
  - A crash between the write and the receipt cannot happen, because both are in one transaction. An ambiguous commit is resolved by retrying with the same key, which replays the reply if the commit happened and runs again if it did not.
  - After pruning, a retry older than 24 hours runs the write again. This is the design's intent but is not told to callers. The prune against a replay held on a receipt row is unstated and untested (finding 7).

## Post-build checks

Each check runs on the merged Server work against PostgreSQL 16 and again against 17.

1. Commit-order gate. Run `TestFeedPositionsFollowCommitOrder` 50 times in a row. Expected: it passes every time. With the production lock removed, it fails every time on B becoming visible first.
2. Cold migration race. Start two `Migrate` calls on an empty database 50 times. Expected: both succeed every time and the ledger shows `store=1`. With the ledger created before the lock, the run fails at least once.
3. Slow writer. Hold one write open after its flush for the lock timeout of finding 1 plus one second. Expected: a second writer with changes returns the named timeout error, and a writer with no changes finishes at once.
4. Stalled listener. Pause the listener process for a minute while a writer commits 10,000 changes. Expected: commits succeed, the reader catches up on its next poll, and no change is missed or repeated.
5. Poll only. Run the follower with `LISTEN` refused. Expected: every change arrives within about two seconds and in order.
6. Ring boundary under real concurrency. Run 8 writers against one slow stream. Expected: the stream is cut off exactly when lag reaches 2,048, and a fast stream sees every row once.
7. Prune against a replay. Replay a receipt while `Prune` selects the same row. Expected: the sweep waits, the replay returns the stored reply, and the next sweep deletes the row.
8. Sequence cache. Run the commit-order test through a pool of 2 connections that are reused. Expected: positions are ascending in commit order and no session holds a range of future positions.
9. Forward clock step. Step the database clock forward by a day, write, step it back, and run a sweep. Expected: the sweep reports the stalled prefix and deletes nothing it should keep.
10. Slow snapshot. Open a Watch whose snapshot reads for several seconds while a writer commits 2,100 rows. Expected: the stream returns the lag error, and a second attempt without the write load succeeds.

## Grade

6 out of 10. Counts: 0 blockers, 5 majors, 7 minors.

The design is sound, and its central idea (the lock taken last, the prefix property and the ring) is correct as written. The plan needs the hold window of the lock bounded, a cold-start test, defined test seams, one exact formula for the cut-off, and agreement with the sprint 0 plan on the driver entries, the lag code and the position type before builders start.
