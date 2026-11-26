REVISE

# Round 1 review, seat architect, plan go-s1-server-store

- Task identifier: review-go-s1-server-store-r1-architect
- Nonce: 023e89
- Lens: architect
- Plan text reviewed: `.plans/plans/go-s1-server-store/plan.md`, 579 lines, the same text as `attack/plan-v1.md`.
- Model and effort requested: claude-sonnet-5-5, high.

## Top finding

The Server team needs three things from other teams, and no plan delivers any of them: a `github.com/jackc/pgx/v5` entry in `go.mod`, a PostgreSQL test role with the database creation privilege, and the executable `deploy/test/mutate-feed-lock.sh` that check C12 runs (architect-r1 1).

## Findings

### architect-r1 1. MAJOR. Three prerequisites have no owner

- Tied to: C12, R10, plan.
- Evidence:
  - `plan.md:46` and `plan.md:105` require a test role with `CREATEDB`. `decisions.md:15` records that assumption as "unanswered".
  - `.plans/plans/go-s1-stack-vault/plan.md:125` says neither `lgtm_app` nor `lgtm_bao` can create databases or roles, and it plans no third role.
  - `plan.md:82` says item 1 needs the driver entries from the Contract team. The Contract team's go.mod work in `.plans/plans/go-s0-contract/plan.md:180` is `go mod tidy` over generated code, which imports no PostgreSQL driver. A search of the s0, s2, s3 and client plans finds no `pgx` at all.
  - `deploy/test/checks.sh:591-600` makes C12 fail unless `deploy/test/mutate-feed-lock.sh` exists and is executable. `deploy/test/README.md:44-47` and `:76` describe that hook. The file does not exist on `master`.
  - `plan.md:340` describes the lock-removal mutation as a manual run on a disposable branch. It never delivers the hook.
  - `.plans/epics/go-rewrite/epic.md:123` lets the Server team edit only `src/server/`, so the Server team cannot add the hook, the test role or the dependency itself.
- Failure scenario: item 1 starts on a checkout where `go build ./...` cannot resolve `pgx`, and the Server team may not edit `go.mod`. If the dependency is added, `NewDatabase` fails at its first `CREATE DATABASE` with "permission denied to create database" against the Stack team's roles. At wave 1 verification C12 reports a missing hook even though every Go test passes.
- Fix:
  - Name an owner and a sprint for each of the three items in the plan's Rollout order, and add each to "What the teams must agree on" in the epic.
  - Suggested owners: the Contract team adds `pgx/v5` at an exact version in sprint 0. The Stack team provisions the test role, or the plan states that `LGTM_TEST_POSTGRES` points at a superuser in a disposable container. The plan also states where the hook lives and who writes it.
  - If the hook must live under `deploy/test/`, either the epic edit table gives the Server team that one file, or the Stack team writes a thin script that runs a Server-owned mutation under `src/server/`.
  - Replace the "assume" in decisions row 8 with a decision.

### architect-r1 2. MAJOR. The plan and the sprint 0 draft end a lagging Watch call with different Connect codes

- Tied to: R8, R2, C6 to C9.
- Evidence:
  - `plan.md:470` and `decisions.md:23` map lag to `resource_exhausted`.
  - `.plans/plans/go-s0-contract/draft/common.proto:17` says a lagging caller is ended with `aborted`.
  - `.plans/plans/go-s0-contract/plan.md:126` repeats `aborted`.
  - `src/frontend/src/server/sync.ts` reconnects with backoff on every error except `unauthenticated`, so the website treats both codes the same way.
- Failure scenario: the Machines service in sprint 3 follows the plan and returns `resource_exhausted`. The contract test or a reviewer who follows `common.proto` expects `aborted`. One of the two plans is then wrong at the moment C8 or C9 is written. A later reader of the contract cannot tell which code a client may rely on.
- Fix: choose one code and edit both plans. `aborted` fits better, because the call is meant to be reopened at once, and it is already in the contract that the Contract team reviews. If the plan keeps `resource_exhausted`, sprint 0 must change its comment and its plan row. Record the choice in `decisions.md` and give a status message that the website can show.

### architect-r1 3. MAJOR. "A part never inserts into `feed`" is a convention, and a breach reopens the main risk

- Tied to: R4, D5, C12.
- Evidence:
  - `plan.md:246` says a part executes no SQL on the store's tables.
  - `plan.md:98-100` gives parts a raw `Tx.Exec`, `Query` and `QueryRow`.
  - `plan.md:259` says only the store allocates positions, and `plan.md:263-267` tests only the store's own flush.
- Failure scenario:
  - A later part writes `INSERT INTO feed (...)` inside its callback, to record a change without waiting for the flush. The identity value is taken before the advisory lock, at the start of the callback.
  - A slow domain write follows. Another writer allocates a higher position, takes the lock, commits, and the follower advances past the lower position. The lower position then commits and the website never sees that change. That is the lost update that the plan exists to prevent.
  - No test in the plan fails.
- Fix:
  - Add a `BEFORE INSERT` trigger on `feed` that raises an error unless `current_setting('lgtm.feed_flush', true) = 'on'`. `flushFeed` sets that value with `set_config('lgtm.feed_flush', 'on', true)` right after taking the lock.
  - Add `TestFeedRejectsInsertOutsideFlush`, which runs a raw insert through `Tx.Exec` and expects the error.
  - This keeps R4 true for every later part without a seam, and the mutation proof of item 4 stays valid.

### architect-r1 4. MINOR. The Watch envelope does not match the draft field names or the position type, and no side owns the mapping

- Tied to: R2, D1.
- Evidence:
  - `plan.md:239` and `plan.md:445-453` use `int64` for the position and the names `Snapshot`, `Saved` and `RemovedKey`.
  - `.plans/plans/go-s0-contract/draft/common.proto:6-9` names the variants `all`, `saved` and `removed_id`. Its `FeedPosition.value` is `uint64` (`common.proto:33`).
  - `plan.md:468` only says that the comparison with sprint 0 checks "the signed 64-bit position range".
- Failure scenario: sprint 3 converts `int64` to `uint64` in each adapter on its own. One adapter forgets the zero case, or maps `RemovedKey` to a field of a different name, and the shared shape test of sprint 0 fails late.
- Fix: state in the plan that positions are never negative, that the adapter converts with a checked cast, and that `Message.Snapshot`, `Saved` and `RemovedKey` map to `all`, `saved` and `removed_id`. Make the same note in sprint 0's brief so the comparison in Rollout order step 3 has a fixed list.

### architect-r1 5. MINOR. The 2,048 cut-off is worded four ways, and the in-flight row makes the boundary ambiguous

- Tied to: D5, R5, C6 to C9.
- Evidence:
  - The design says "falls 2,048 changes behind" (`docs/specs/2026-11-25-go-rewrite-foundation-design.md:170`).
  - The plan says "falls 2,048 changes behind" (`plan.md:26`), "At 2,048 unacknowledged changes" (`plan.md:352`), "2,048 actual committed rows follow its acknowledged cursor" (`plan.md:420`) and "cancel at 2,048 actual rows, including the in-flight row" (`plan.md:487`).
  - `plan.md:418` says the ring retains the last 2,048 observed rows. `plan.md:420` checks the condition "when publishing a page", and a page holds up to 512 rows.
  - `plan.md:426` gives the test points 2,047 and 2,048 but does not say whether the unacknowledged row from `Next` is one of them.
- Failure scenario: a builder counts the in-flight row as extra. The cut-off then happens at 2,049, and the test with the in-flight row still passes by accident. The reverse reading cuts at 2,047 and breaks the boundary test of the other builder.
- Fix: write one formula in one place, for example "a subscriber is lagged when the number of feed rows with position greater than its acknowledged cursor is 2,048 or more, counting the row returned by `Next` and not yet acknowledged". Evaluate it per row, at `Start`, and per published page. Test the pair "2,047 rows including the in-flight row stays open, 2,048 is cut" and a page of 512 that crosses the line.

### architect-r1 6. MINOR. A replaced or restored database leaves the follower ignoring every new row

- Tied to: R7, D3.
- Evidence:
  - `.plans/plans/go-s0-contract/draft/common.proto:30` says positions of one server whose database was replaced cannot be compared.
  - `plan.md:385` says only that switching to a different history requires a restart. `plan.md:420` ignores rows up to the cursor.
- Failure scenario: an operator restores PostgreSQL from a backup while the server runs. The head falls from 9,000 to 7,500. The reader keeps asking for `position > 9,000`, sees nothing, and every stream stays open and silent until the server restarts.
- Fix: in `readFeedPage`, when the database head is below the cursor, report a history reset. The follower then cancels every subscriber with `ErrWatchLagged` and restarts from the new head. Add `TestFollowerDetectsHeadRegression`, which uses a fixture that truncates the feed and resets the identity.

### architect-r1 7. MINOR. The feed lock is held through the commit, and the plan states no policy for lock waits, deadlocks or a slow commit

- Tied to: D4, D5, R4.
- Evidence: `plan.md:248-259` takes the lock as the last statement before the commit, and `plan.md:263-264` tests only that domain work before the flush blocks nobody. `plan.md:103` says `withWrite` does no automatic retries.
- Failure scenario:
  - A writer's deferred constraint trigger, a commit fsync or a synchronous replica wait runs while it holds the lock, so every other feed writer waits behind it. Readers are never blocked.
  - A writer that holds a domain row lock and waits for the feed lock can deadlock with the holder whose commit-time check needs that row. PostgreSQL aborts one of them with error 40P01, and the plan does not say what the caller sees.
- Fix: state in the plan that the lock is held across the commit and that this is the accepted cost on one node. Set a `lock_timeout` on the lock statement. Map deadlock and serialization failures to one retryable error that `WriteOnce` callers can repeat with the same key. Add one test where two writers deadlock on a domain row and the loser retries.

### architect-r1 8. MINOR. The plan says two different things about who creates `schema_migrations`

- Tied to: R1, D3.
- Evidence:
  - `plan.md:78` and `scouting.md` (anchor S3) list `schema_migrations` among the tables created by `0001_store.sql`.
  - `plan.md:142` creates the ledger with `CREATE TABLE IF NOT EXISTS` in Go before it is read, and `plan.md:144-172` shows a `0001_store.sql` that has no ledger.
- Failure scenario: one builder adds the ledger to `0001_store.sql`, and the other keeps it in `Migrate`. On an empty database the file then fails with "relation already exists", or the store's own version is never recorded.
- Fix: state that `Migrate` creates the ledger and `0001_store.sql` creates only `feed`, `feed_retention` and `receipts`, and remove `schema_migrations` from the file list in the task table and in S3.

### architect-r1 9. MINOR. Later parts get no helper that opens a migrated store, and advisory lock ids have no registry

- Tied to: R10, wave 2 room.
- Evidence:
  - `plan.md:105` gives `NewDatabase` as returning only a `*pgxpool.Config`.
  - The advisory lock ids `841941100001`, `841941100002` and the pair starting at `1279743053` are spread over `plan.md:140`, `:251` and `:295`.
  - `.plans/plans/go-s2-server-signin/plan.md` and `.plans/plans/go-s3-server-machines/plan.md` both need a store with their own migrations applied in tests.
- Failure scenario: each of the next three plans writes its own open, migrate and close code in its tests, and two of them copy the helper differently. A later part picks a lock id that collides with an existing one and delays feed writers.
- Fix: add `storetest.NewStore(t, schema, parts ...store.Migrations) *store.Store` that opens, migrates and registers cleanup. Put the three lock ids as named constants in one file with a comment that says parts must not take ids from this space.

## Answers to the attack questions

1. Anchors. All code anchors resolve at the current commit: `floorplan/plan.json`, `packages/engine/src/log/migrations.ts`, `packages/engine/src/log/EventLog.ts`, `src/mock-server/src/engine.ts`, `clients.proto`, `map.proto`, `preview.proto`, `scripts/check-private.mjs` and the design lines cited in the plan. Two anchors point to files that do not exist yet: `src/contracts/proto/lgtm/v1/common.proto` (only the draft under `.plans/plans/go-s0-contract/draft/` exists) and the root `go.mod`. Both are declared as sprint 0 outputs, so this is not a finding, but findings 1 and 4 follow from it.
2. Writers and readers.
   - `feed`: written only by `flushFeed` (convention only, finding 3). Read by `readFeedPage`, `Snapshot`, `Prune`.
   - `feed_retention`: written by `Prune`. Read by `readFeedPage` and `readHead`.
   - `receipts`: written by `WriteOnce` and `Prune`. Read by `WriteOnce`.
   - `schema_migrations`: written and read by `Migrate` (finding 8).
   - Public Go names: `Store` and its methods are called by every later part and by `watch`. `Follower` and `Subscription` are called by `watch.Serve`. `Request` and `RequestKeyHeader` are called by the service adapters of sprints 2 and 3. `NewID` is called by the parts that own identifiers. Nothing else is forgotten, apart from the unowned items in finding 1.
3. Placement. Store, follower and `watch` are server internals under `src/server/internal/`, as D1 and the epic edit table say. The wire shape stays in `src/contracts/`. The only misplaced items are the hook and the test role in finding 1.
4. Rules. R1 kept (each part has its manifest). R2 kept in intent, with the mismatches in findings 2 and 4. R3 kept (parts own their queries). R4 kept by convention only (finding 3). R5 kept (all Watch calls go through `watch.Serve`). R6 does not apply. R7 kept (the ring is rebuilt from the database, and finding 6 covers a regression). R8 kept with finding 2. R9 kept (digest and receipt-safe reply only, no key or reply in diagnostics). R10 kept (real PostgreSQL), with the role gap in finding 1. R11 kept (Go, nothing copied).
5. Neighbour mismatches. Findings 1, 2, 4 and 9. The s2 and s3 plans consume the store as described, and no further mismatch with "What the teams must agree on" was found.
6. Merge independence. Items 1 to 3 merge alone once finding 1 is resolved. Items 4 to 8 each compile against the earlier ones. Item 9 compiles without generated types, because `watch` uses its own envelope. At the commit between items 7 and 8 there is a reader without a follower, which breaks nothing. Nothing else found.
7. Room for wave 2. Enough room: the manifest, the `Record` call, the generic `Serve` and the key space for receipts serve every later part. The plan builds nothing that no wave 1 check needs, since receipts and pruning are required by the design's safe-retry and retention sections. Nothing found.
8. Dependencies. The only new dependency is `github.com/jackc/pgx/v5`, with no version in the plan. The module proxy lists v5.10 to v5.13, so the major version is real and current, and it is compatible with Go 1.26. The go toolchain available to this review is 1.23, so a build under 1.26 could not be run here. The version must be pinned by the Contract team (finding 1).

### Questions of the task

a. Commit order, lock level and what a slow writer blocks. Nothing wrong found. The lock is `pg_advisory_xact_lock`, so it is held by the transaction and released after the commit is visible. It is taken as the last statement before the insert and the commit (`plan.md:251`). A later writer therefore gets a higher position only after the earlier one is visible, and a snapshot sees a prefix of committed positions. Domain work before the flush blocks no one (`TestFeedLockStartsAtFlush`). A slow commit, a deferred check or a synchronous replica wait blocks every other feed writer, and readers never wait. The deadlock and timeout policy is missing (finding 7). The single-statement insert of several rows keeps their order, and a rollback leaves only gaps.

b. Follower: miss or repeat. Nothing wrong found for these cases.
   - A lost notification is covered by the two-second poll, and the test for the poll without a listener exists.
   - A long transaction holds no position, because positions are allocated at the end under the lock.
   - Gaps are not counted (`TestFollowerIgnoresSequenceGaps`).
   - A restart takes a fresh head before the reader starts, and the reader asks for `position > head`, so a row committed in between is read once.
   - Two items remain: a restored database (finding 6) and the cut-off wording. The cut-off is not yet stated exactly, because the in-flight row and the page check leave room for 2,047 or 2,049 (finding 5).

c. Safe retries. Nothing wrong found for these cases.
   - Same key, different body, caller or procedure: the digest differs, so the second request gets `ErrRequestKeyConflict` without the prior reply and without calling the callback.
   - Two identical requests at once: the per-key advisory lock serializes them, the second finds the receipt and returns the stored reply, and the callback runs once.
   - A crash between the write and the receipt: both are in one transaction, so both exist or neither does.
   - Pruning: after 24 hours the receipt is gone and a retry with that key runs the write again. The plan states that limit (`plan.md:295`) and the caller owns the choice of retry window. `hashtext` is an internal function, and a collision only delays unrelated work.

## Post-build checks

| # | Scenario | Expected result |
|---|---|---|
| 1 | On a clean checkout with only the Contract team's go.mod, run `go build ./...` and then `go test ./src/server/internal/store/...` with `LGTM_TEST_POSTGRES` set to the Stack team's documented test role. | The build passes, and `NewDatabase` creates and drops a database without a privilege error. |
| 2 | Run `deploy/test/checks.sh --only C12`. | The hook exists, the mutation without the feed lock fails `TestFeedPositionsFollowCommitOrder` for B becoming visible first, and C12 passes. |
| 3 | In a part's test callback run `INSERT INTO feed (topic, scope, key, change) VALUES ('t', '', 'k', 'saved')` through `Tx.Exec`. | PostgreSQL raises the trigger error and nothing is written. |
| 4 | Make one subscriber hold one unacknowledged row, then commit 2,046 more rows, then one more. | The subscriber's context stays open at 2,047 counted rows including the in-flight row, and is cancelled with `ErrWatchLagged` at 2,048. |
| 5 | Truncate the feed and reset its identity while a follower runs at position 9,000, then commit one row. | Every subscriber is cancelled with `ErrWatchLagged`, a new subscriber receives the new row, and no stream stays silent. |
| 6 | Make two writers deadlock on a domain row and the feed lock. | One writer gets the retryable error, retries with the same key, and the feed shows each change once. |
| 7 | Feed a lagging stream through the real Connect adapter. | The website's call ends with the single code the contract names, and `src/frontend/src/server/sync.ts` reopens it with a fresh first message. |
| 8 | Start the store twice against one empty database. | The ledger shows `store=1`, `Migrate` created `schema_migrations` once, and the second start changes nothing. |

## Grade

7 out of 10. Blockers 0, majors 3, minors 6.
