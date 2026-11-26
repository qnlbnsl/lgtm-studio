---
topic: go-s1-server-store
status: done
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# The server's database layer

## Overview

The store gives sign-in and the Machines page a way to upgrade their tables and write records. Its change feed lets the website load a full list and then receive updates.

Two writes that commit out of order can hide a change from the website. The store allocates positions under a lock that lasts through commit. The ordering test must catch a later write becoming visible first when the lock is removed.

The sprint ends with tests against real PostgreSQL. Starting the server and opening a stream from the Machines page belong to later sprints.

## Key decisions

Each part writes its own records. Store writes their feed references in the same transaction.

A successful keyed call can save its reply. A retry must match the caller, operation and request body. The owner still needs to choose how to retry calls that contain or return credentials.

Each Watch starts with a snapshot. One reader per server listens for database notifications and polls every two seconds. A stream ends when it falls behind or loses authorization. After an earlier PostgreSQL restore, existing streams can go quiet. Restart the server to reopen them with fresh snapshots. Restore recovery is outside wave 1. **UNREVIEWED FIX** (logic-r4 3)

See [the decision log](decisions.md).

## Workstreams

### Parts can store state and upgrade their own tables

Each part brings its own numbered database changes. The tests start the store twice against one empty database and check that each change is applied once. Parallel tests can separately create tables with the same names because they each get their own database.

### Writes arrive in commit order and safe retries reuse replies

The ordering test proves the reader cannot pass an uncommitted lower position. The retry test shows two matching calls create one change and return the same reply.

### Watch starts with a snapshot and follows live changes

A gated test proves the list and cursor share one database view. A held sender test proves a slow stream can be cut off while another stream keeps receiving changes.

## Rollout order

1. Contract delivers the root module and Watch shape in sprint 0. Item 1 adds the sole direct dependency, `github.com/jackc/pgx/v5 v5.13.0`. Its builder runs `go get github.com/jackc/pgx/v5@v5.13.0` and includes `go.mod` and `go.sum` with the first import. The orchestrator checks the pin and `go mod tidy -diff` before committing. **UNREVIEWED FIX** (2026-11-26 epic ruling)
2. Stack’s `deploy/test/with-postgres.sh` supplies the test connection below. Use `GOFLAGS=-p=1 GOMAXPROCS=2` for multi-package database suites. This runs one package at a time and retains two parallel tests per package. Check actual connection usage after build. **UNREVIEWED FIX** (logic-r4 2)
3. Run `GOFLAGS=-p=1 GOMAXPROCS=2 deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12` on a clean checkout. Record `git rev-parse HEAD` before and after. Stack’s `deploy/test/mutate-feed-lock.sh` calls Server’s `src/server/internal/store/check-feed-lock.sh` without arguments. The driver reads its exit status. Run the Server hook directly under the wrapper to capture its output. **UNREVIEWED FIX** (logic-r4 2, architect-r2 1, architect-r3 5)
4. Later Server sprints wire startup, session authorization and transport. The owner settles keyed credential behavior before those calls use any receipt path.


| Server writer and shared names | Readers |
|---|---|
| Stack item 8 supplies `LGTM_TEST_POSTGRES` for `lgtm_store_test` on literal loopback or a Unix socket, with `sslmode=disable` and one host. **UNREVIEWED FIX** (logic-r4 2) | `storetest.NewDatabase` and C12 read the connection. |
| Item 1 writes `Store`, `Open`, `Close`, `ConnectionPolicy` and `ConnectionPolicy.AllowPrivateStackPostgres`, `Read`, `ReadTx` and its query methods. It writes `Error`, `Outcome`, its constants, `Retryable` and `NewDomainError`. | Sprint 2 bootstrap, session lookups and handlers. Sprint 3 Machines and link parts use the query and error contracts. Stack supplies the private database binding. |
| Items 2 and 3 write `NewID`, `Migrations` and `Migrate`. Items 1 and 3 write `storetest.NewDatabase` and `migrated.NewStore`. | Sprint 2 sign-in and startup. Sprint 3 Machines and link register manifests. Later Server tests reuse both fixtures. |
| Item 4 writes `Write`, `Tx` and its query methods, `Record`, `Change`, `ChangeRef`, `Saved`, `Removed`, `FeedRow`, `MaxWriteChanges` and `ErrTooManyChanges`. Item 5 writes `Request`, `RequestKeyHeader`, `WriteOnce` and the receipt errors. | Sprint 2 and 3 mutation and receipt adapters. Store and Watch read the feed references. |
| Items 6 through 8 write `Prune`, `PruneResult`, `RunPruner`, `Snapshot`, `StartFollower`, `Follower`, `Subscription` and its methods, `MaxWatchLag`, `ErrWatchLagged`, `ErrFeedUnavailable` and `ErrSubscriptionState`. | Sprint 2 bootstrap owns background lifecycle. Sprint 3 Machines and the Watch helper consume snapshots, follower and stream errors. |
| Item 9 writes `watch.Serve`, `Source`, `Message`, `watch.Snapshot` and the Watch errors. | Sprint 2 sign-in supplies session lookup. Sprint 3 Machines supplies the source callbacks, reads the messages and maps the errors. |

Sprint 2 sign-in item 9 owns `store/instance.go`, `Store.AcquireServerInstance` and `InstanceLock` with `Check` and `Close`. Its startup code reads them. Store item 3 reserves `serverInstanceLockID int64 = 841941100003` in `locks.go` and sprint 2 reads it. It is a session lock. Include both directions in the orchestrator’s comparison.

**UNREVIEWED MECHANISM** (architect-r3 2, logic-r3 6). Group names by writer and reader. At rollout step 3 the orchestrator compares `go doc -all` for Store, both fixture packages and Watch with the sign-in and Machines plans. The commands use the built packages as the complete export list.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

The Server team builds the Store and Watch folders in Go 1.26 or newer. Structured Query Language (SQL) examples use the configured PostgreSQL schema. All connections use the same database endpoint in this sprint.

Freeze each test after its named assertion fails. The lead verifies the exact commit and the required disposable mutations.

The tables abbreviate workstream as WS. The decision log records the build assumptions and superseded rows. (architect-r2 9, threat-r2 9)

**WS-1. Parts can store state and upgrade their own tables**

Parts get typed query handles and register their own migrations.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | Parallel tests use separate empty databases. Canceling a write returns the connection. Waiting writers leave room for a reader. | `TestDatabasesAreIsolated`, `TestTransactionsReleaseConnection` and `TestWritersCannotStarveReaders`. |
| 2 | An identifier contains the version, variant and timestamp of a version 7 universally unique identifier (UUID). | `TestNewIDVersionAndTime` and `TestUUIDv7EntropyFailure`. |
| 3 | Concurrent startup applies each numbered change once per part. A failed batch leaves its tables and recorded versions unchanged. | `TestMigrateColdConcurrentStarts`, `TestMigrateConcurrentStarts`, `TestMigrateFailureRollsBack` and `TestMigrateValidatesManifest`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Open the pool, define the query handles and isolate tests. | `src/server/internal/store/pool.go` `Open`, `Read`, `withWrite` and `src/server/internal/store/storetest/postgres.go` `NewDatabase`. See S1 in [scouting.md](scouting.md). |
| 2 | Generate identifiers in Go. | `src/server/internal/store/ids.go` `NewID`. See S2 in [scouting.md](scouting.md). |
| 3 | Load and apply the migration manifests and create the store tables. | `src/server/internal/store/migrate.go` `Migrate` and `src/server/internal/store/migrations/0001_store.sql` `feed`, `feed_retention` and `receipts`. See S3 in [scouting.md](scouting.md). |

**1. Connection pool, query handles and test isolation**

Create `pool.go`, `pool_test.go`, `storetest/postgres.go` and `storetest/gate.go` under `src/server/internal/store/`. Add the rollout pin to the root `go.mod` and `go.sum` with the first import. Item 2 can run beside this item. It implements D2, D3, D4, R3, R7, R10 and C12. **UNREVIEWED FIX** (2026-11-26 epic ruling, logic-r3 3)

Item 1 writes `ReadTx`. Item 4 writes `Tx`. (critic-r1 10)

```go
type Store struct { /* private pool and configuration */ }
type ReadTx struct { /* private transaction */ }

type ConnectionPolicy struct {
    AllowPrivateStackPostgres bool
}

type Error struct {
    SQLState    string
    Constraint string
}

type Outcome string

const (
    OutcomeRolledBack Outcome = "rolled_back"
    OutcomeUnknown    Outcome = "unknown"
)

func (e *Error) Error() string
func (e *Error) Retryable() bool
func (e *Error) Outcome() Outcome
func NewDomainError(message string) error
func Open(ctx context.Context, config *pgxpool.Config, policy ConnectionPolicy) (*Store, error)
func (s *Store) Close()
func (s *Store) Read(ctx context.Context, read func(context.Context, *ReadTx) error) error

func (tx *ReadTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
func (tx *ReadTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
func (tx *ReadTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
```

`Open` copies and pings the configuration within five seconds. It returns a fixed connection diagnostic on failure. The main pool needs eight or more connections and admits `MaxConns - 3` domain writers. Each Store can use `MaxConns + 2` connections, with a one-connection page pool and a dedicated listener. Sprint 2 server startup adds one unpooled instance-lock session. All use the validated configuration. Default to 30-second transmission control protocol (TCP) keepalives, 30 seconds idle in a transaction and 15 seconds per statement. `Close` closes the pools after the follower stops. (architect-r2 5, architect-r3 3)

Before dialing, validate every endpoint and fallback for Transport Layer Security (TLS) with a verified certificate chain and hostname. Plaintext exceptions are Unix socket paths, literal loopback addresses and the exact Compose service `postgres` with explicit `AllowPrivateStackPostgres`. For the Compose exception require one host and no fallback to another host. In the final dial path require the resolved address to pass `netip.Addr.IsPrivate` and dial that same literal address. Store writes `ConnectionPolicy`, sprint 2 bootstrap sets it and Stack owns the private network binding. (threat-r1 9, threat-r2 5)

`Read` uses a read-only `REPEATABLE READ` transaction. The private `func (s *Store) withWrite(ctx context.Context, write func(context.Context, pgx.Tx) error) error` uses `READ COMMITTED`. The 30-second write budget covers admission through commit. Each statement has a 15-second limit and lock waits have a ten-second limit. Pruning uses those same limits per batch without domain-writer admission. Respect shorter caller deadlines. On error, cancellation or panic, roll back with a fresh one-second cleanup budget and discard a connection if cleanup fails. Callbacks close rows and leave transaction control to Store. Store never reruns a callback. (architect-r1 7, critic-r1 1, logic-r1 2, logic-r2 4, architect-r2 7)

Sanitize errors at the Store boundary, including row iteration, callbacks and commit. Before commit, normalize cancellation to the context error. Pass through direct Store and `NewDomainError` sentinels. Domain callers declare those sentinels with fixed public messages. Map every other error to `*Error` with a fixed message and no cause. Retain a recognized PostgreSQL error code (SQLSTATE) and a declared constraint identifier when present. Store writes this API and later adapters declare and read the safe sentinels. **UNREVIEWED MECHANISM** (threat-r2 1, threat-r1 8). An allowlist covers driver, connection and wrapped callback errors without enumerating their concrete types.

Before commit or on confirmed rollback, `*Error` reports `OutcomeRolledBack`. A PostgreSQL rejection of commit, including lock timeout `55P03`, or `pgx.ErrTxCommitRollback` confirms rollback. Connection loss, a fatal session error or caller cancellation or deadline without a definitive result after commit may have been sent yields `OutcomeUnknown`. The commit path returns a sanitized `*Error`. Adapters read `Outcome` then `Retryable`. Retryable rollback codes `55P03`, `40P01` and `40001` map to `aborted`. Unknown or missing outcomes map to `unavailable` with “The database did not confirm whether the write committed.” Retry an ambiguous write only with its original key. Known pre-commit sentinels keep their adapter mappings. The zero `Outcome` is invalid. **UNREVIEWED FIX** (logic-r4 1, logic-r3 1, threat-r2 2)

`func NewDatabase(t testing.TB, schema string) *pgxpool.Config` skips if `LGTM_TEST_POSTGRES` is absent. Check the configured endpoint policy, maintenance database name and `CREATEDB` privilege before creating a randomly named, quoted database from `template0`. Set `search_path` to the validated package schema. Report fixed diagnostics without the connection string. Use the instance’s actual connection capacity for the bounded suite. C12 and Server acceptance require that fixture. **UNREVIEWED FIX** (logic-r4 2, architect-r2 3, threat-r2 8)

Register cleanup immediately. Close application connections before `DROP DATABASE <quoted-name> WITH (FORCE)` under a fresh ten-second context. The Stack wrapper discards the disposable instance if the test process dies before cleanup. (critic-r1 8)

In `storetest/gate.go` item 1 writes `type AdvisoryKey struct { ClassID, ObjectID, SubID uint32 }`, `BigintKey(value int64) AdvisoryKey`, `IsBlocked(ctx context.Context, observer *pgx.Conn, waiter, blocker int32, key AdvisoryKey) (bool, error)`, `WaitForBlock(t testing.TB, observer *pgx.Conn, waiter, blocker int32, key AdvisoryKey)` and `HoldAdvisoryGate(t testing.TB, observer *pgx.Conn, key int64) func()`. The bigint form uses the high and low words with `SubID=1`. The two-integer form uses `SubID=2`. Match the exact ungranted advisory key and blocking backend through `pg_locks` and `pg_blocking_pids`. `WaitForBlock` polls every 10 milliseconds for at most 15 seconds. The hold helper returns an idempotent, cleanup-registered release. Items 3 through 9 use these gates where named. (logic-r3 3, critic-r1 3, critic-r1 10)

Fixture waits default to 15 seconds, unless stated otherwise. Check gate arrival before observing a race and observe silence for 250 milliseconds after synchronization.

Tests first in `pool_test.go`.

- `TestDatabasesAreIsolated`. Run two parallel subtests with the same schema name. Before creating an identical table, assert it is absent from both databases. Insert a distinct value in each and read exactly that value. Assert that the database names differ. In the parent, after the parallel group completes, query the maintenance database for both names and assert that both are gone.
- `TestTransactionsReleaseConnection`. Use an eight-connection pool. Record pool acquisition counts before and after each case. Write a row, cancel the context in the callback and return its error. Open an independent transaction through the pool and assert that it can read within one second and that the row is absent. Repeat with a callback panic caught by the test. In a read transaction attempt an insert and assert the PostgreSQL read-only error. Check that opening a refused connection returns an error without returning a store.

- `TestWritersCannotStarveReaders`. Hold an advisory gate on a separate connection. Start eight writes that all wait on it. With an eight-connection pool, require at most five acquired writer connections and a `Read` within one second. Require all five first-wave waiters to return retryable `55P03` before releasing the gate. The other three must then succeed within a 25-second test budget. Cancel a waiter in a separate case and check pool release. Removing admission must fail. (threat-r1 3, logic-r1 2, logic-r2 6)
- `TestOpenRefusesPlainRemote`. Cover Unix socket, literal loopback, the exact Compose service resolving to a private or public address, a loopback alias and verified remote TLS. Refuse a plaintext remote fallback even when a loopback entry comes first. A dial spy proves rejected addresses are never dialed. (threat-r1 9, threat-r2 5)
- `TestDatabaseErrorsHideValues`. Cause real check and unique violations, a driver-side encoding failure, a closed connection mid-transaction and a callback wrapping a SQLSTATE error. Check `%v`, `%+v` and the whole error chain for absence of fixture values and the endpoint. Assert recognized codes and approved constraint identifiers survive, and a direct safe domain sentinel keeps its identity. (threat-r1 8, threat-r2 1)

- `TestNewDatabaseRejectsWrongMaintenanceName`. Refuse a different maintenance database before connection setup. The merged fixture check verifies Stack’s parsed setting and connection budget. (threat-r2 8, architect-r3 3)

This item is risky because it sets the isolation level and controls transaction lifetime. Changing `Read` to a writable transaction must fail the read-only assertion. Item 1 gates on the named pool and policy tests. Item 9 owns the read-isolation mutation.

**2. Version 7 identifiers**

Create `src/server/internal/store/ids.go` and `src/server/internal/store/ids_test.go`. This item needs Go from the root module. It can run beside items 1 and 3. It implements D2, D3 and C12.

The public entry point is `func NewID() (string, error)`. Use Go's secure random reader for the 74 random bits and a clock for the 48-bit big-endian Unix timestamp in milliseconds. Set the version nibble to 7 and the two high variant bits to binary 10. Return the canonical lowercase hexadecimal form with hyphens. The private test seam is `func uuidV7(at time.Time, random io.Reader) (string, error)`. Keep no counter or node address. The time component sorts identifiers from distinct milliseconds in a test with an advancing clock.

Tests first in `ids_test.go`.

- `TestNewIDVersionAndTime`. Decode two deterministic identifiers from times one millisecond apart and a supplied byte vector. Assert the exact timestamp, version, variant and remaining bits. Assert that lexical order follows those two timestamps. Generate 4,096 real identifiers across concurrent goroutines and assert that none repeat.
- `TestUUIDv7EntropyFailure`. Pass a reader with fewer than the required bytes. Assert that the generator returns an error and an empty identifier.

This item is risky because it sets the sortable time component. Encoding the timestamp in little-endian order must fail `TestNewIDVersionAndTime`. Both tests are the acceptance observations.

**3. Numbered database changes and store tables**

Create `src/server/internal/store/migrate.go`, `src/server/internal/store/migrate_test.go`, `src/server/internal/store/locks.go`, `src/server/internal/store/migrations/0001_store.sql`, `src/server/internal/store/storetest/migrated/store.go` and `src/server/internal/store/storetest/migrated/store_test.go`. This item needs item 1, including its test gates. It can run beside item 2. (logic-r3 3) It implements D3, D4, D5, R1, R3, R7, R10 and C12.

```go
type Migrations struct {
    Part  string
    Files fs.FS
}

func (s *Store) Migrate(ctx context.Context, parts ...Migrations) error
```

The startup caller passes each part explicitly. A part embeds `migrations/*.sql` and exposes a manifest whose `Files` starts at `fs.Sub(embedded, "migrations")`. The store adds its own embedded `store` manifest. Reserve that name. Validate the complete manifest before changing the database. Part names match `[a-z][a-z0-9_]*`. Files use `NNNN_description.sql` with four digits and a lowercase description, with contiguous numbers beginning at 1 for each part. Reject duplicate part names, invalid filenames, duplicate versions, gaps and a recorded version newer than the supplied files. Refuse startup when the database already records a part absent from the manifest. Rollback to an older binary therefore requires a compatible manifest and is an operator decision. The list of parts and same-part prefix history remain explicit.

Run startup in one read-write transaction with a five-minute budget including acquisition and commit, subject to a shorter caller deadline. Set local `statement_timeout` to five minutes and `lock_timeout` to zero. Acquire `pg_advisory_xact_lock($1::bigint)` with `migrationLockID` before touching the ledger. Apply the store part first, then other parts in lexical order, and apply numbered files in order. Execute each trusted embedded SQL file whole in simple query mode. Update the ledger in the same transaction. Append a new migration for a correction. A failure rolls back and startup refuses to serve. The sprint 2 bootstrap calls `Migrate` with a migration budget separate from other startup prerequisites. (logic-r2 4, architect-r2 7, architect-r2 10)

Store item 3 writes `locks.go`. Reserve bigint keys `feedLockID = 841941100001`, `migrationLockID = 841941100002` and `serverInstanceLockID = 841941100003` for feed, migration and sprint 2 startup. `receiptLockNamespace int32 = 1279743053` belongs to the two-integer receipt key space. The corresponding parts read these constants. (architect-r1 9, architect-r3 2)

Item 3 writes `func migrated.NewStore(t testing.TB, schema string, parts ...store.Migrations) *store.Store` in its own fixture subpackage. It provisions the database, opens Store, applies migrations and closes Store before database drop. Later parts use it. Freeze `TestNewMigratedStore` there. Require the part table on return and database removal at cleanup. The separate package avoids an import cycle. (architect-r1 9)

`Migrate` creates and writes the ledger, including the `store` row. `0001_store.sql` creates the three data tables below. (architect-r1 8, critic-r1 9) The bootstrap ledger is `schema_migrations(part text PRIMARY KEY, version integer NOT NULL CHECK (version > 0))`. The first store migration creates the following exact tables.

```sql
CREATE TABLE feed (
    position bigint GENERATED ALWAYS AS IDENTITY (START WITH 1 CACHE 1) PRIMARY KEY,
    topic text NOT NULL CHECK (topic <> ''),
    scope text NOT NULL,
    key text NOT NULL CHECK (key <> ''),
    change text NOT NULL CHECK (change IN ('saved', 'removed')),
    written_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    retention_started_at timestamptz
);

CREATE TABLE feed_retention (
    singleton boolean PRIMARY KEY CHECK (singleton),
    pruned_through bigint NOT NULL CHECK (pruned_through >= 0)
);

INSERT INTO feed_retention (singleton, pruned_through) VALUES (true, 0);

CREATE TABLE receipts (
    principal_hash bytea NOT NULL CHECK (octet_length(principal_hash) = 32),
    request_key text NOT NULL CHECK (octet_length(request_key) BETWEEN 1 AND 256),
    fingerprint_version smallint NOT NULL CHECK (fingerprint_version > 0),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    reply bytea NOT NULL CHECK (octet_length(reply) <= 65536),
    written_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    retention_started_at timestamptz,
    PRIMARY KEY (principal_hash, request_key)
);

CREATE INDEX receipts_retention_idx
    ON receipts (retention_started_at NULLS FIRST, principal_hash, request_key);

```

The migration owns the feed guards. Inserts require `lgtm.feed_flush=on` or raise `23514` with `feed_requires_store_flush`. Updates and deletes require `lgtm.feed_prune=on` or raise `23514` with `feed_requires_store_prune`. A delete above `pruned_through` also raises `23514` with `feed_delete_requires_watermark`. Truncation raises `23514` with `feed_refuses_truncate`. Item 4 sets the flush flag and observes the guard errors. Item 6 sets the prune flag and advances the floor in its deletion transaction. The guards protect against accidental statements by trusted callers. A blank scope denotes server-wide records. (threat-r2 7, logic-r3 5)

Tests first in `migrate_test.go`.

- `TestMigrateColdConcurrentStarts`. Hold the migration advisory lock on an observer connection while two calls start on a database with no ledger. Use item 1’s `WaitForBlock` with `BigintKey(migrationLockID)` to observe both waiters. Assert the waiting backends have started no ledger creation by observing their active statement and lock records. An uncommitted ledger can be invisible to another transaction, so invisibility alone is insufficient. Release the lock. Assert that both succeed and that the ledger contains `store=1` once. (critic-r1 2, logic-r1 10)
- `TestMigrateConcurrentStarts`. Bootstrap the store ledger once, then register two new fixture parts with version 1 and a second change in one part. Launch two startup calls on separate connections. Have the first hold a gate inside its first migration. Assert that the second waits for the first on the startup advisory lock before the gate is released. Use `pg_locks` and `pg_blocking_pids` to distinguish the startup lock from the test gate and a catalog lock. Release the gate. Both calls succeed. Observe the store schema through the catalog. Assert the ledger values `store=1`, the first part at 2 and the second at 1. Assert that non-idempotent fixture inserts occur once and in version order. A third call leaves the rows and versions unchanged.
- `TestMigrateUsesStartupBudget`. Hold the migration lock for twelve seconds after a starter blocks, then release it and run a 20-second statement. Both startups must succeed within 60 seconds. Assert the live migration sets `statement_timeout` to five minutes. In a separate case, set a shorter caller deadline, observe the migration start before it expires and require cancellation without a schema or ledger change. **UNREVIEWED MECHANISM** (logic-r3 9). The existing caller deadline and a live settings assertion exercise both bounds without a second timeout option. (logic-r2 4, architect-r2 7)
- `TestMigrateFailureRollsBack`. Start with version 1 of a fixture part, then attempt version 2 that creates a new table followed by a failing statement. Assert an error, absence of the new table, and ledger version 1. Replace only the unmerged fixture file and assert a rerun records version 2.
- `TestMigrateValidatesManifest`. Submit duplicate names, duplicate numbers, missing numbers and invalid paths as table-driven cases. Assert a refusal before any ledger or table appears. After a valid migration, submit an older or incomplete manifest and assert a refusal with the previous ledger intact.

Deleting the migration lock must fail concurrent startup. Creating the ledger before it must fail the cold-start test.

**Design artifact.** The database layer in the approved design supplies the brief. This workstream has no screen.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Parallel tests or a canceled transaction leave pooled connections or test databases behind. | Medium | Use a fresh cleanup context and prove teardown against the maintenance database in item 1. |
| Two startups apply the same migration or advance its ledger before its statements are durable. | High | Use one transaction and the startup advisory lock. Prove concurrent and failed starts in item 3. |

**Verify.** Run `go test -race -count=1 ./src/server/internal/store/...` with the rollout’s parallelism and disposable PostgreSQL 16 or newer. No device check is needed. **UNREVIEWED FIX** (logic-r4 2)

**Rollout detail.** Step 1 makes the root Go module available and gives tests a PostgreSQL account. Step 2 applies `Migrate` before serving. Later server startup must close the store and report a migration error if startup cannot finish.

**WS-2. Writes arrive in commit order and safe retries reuse replies**

A part changes its current state and marks what changed in one transaction. The store keeps that update and its feed rows together. A part can wrap a keyed call in the same transaction so a retry sees its original reply.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 4 | A later writer cannot expose a position past a lower uncommitted position. A failed commit leaves no visible state or feed entry. | `TestFeedPositionsFollowCommitOrder`, its lock-removal mutation, `TestFeedRollbackAndNotification` and `TestFeedLockStartsAtFlush`. |
| 5 | A concurrent retry returns one stored reply. A mismatched retry is refused and a failed attempt leaves the key reusable. | `TestWriteOnceConcurrentRetry`, `TestWriteOnceMismatch`, `TestWriteOnceVersionedReplay` and `TestWriteOnceRollbackAndUnkeyed`. |
| 6 | Cleanup removes expired feed entries and receipts while retaining the cutoff entries and part-owned tables. | `TestPruneCutoffAndWatermark`, `TestPrunePreservesFeedPrefix` and `TestPruneStartsAgeAfterCommit`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 4 | Flush buffered feed changes under the transaction-level advisory lock. | `src/server/internal/store/feed.go` `Write`, `Record` and `flushFeed`. See S4 in [scouting.md](scouting.md). |
| 5 | Serialize callers that present the same request key. | `src/server/internal/store/receipts.go` `WriteOnce` and `Request`. See S5 in [scouting.md](scouting.md). |
| 6 | Prune old feed entries and receipts. | `src/server/internal/store/prune.go` `Prune` and `RunPruner`. See S6 in [scouting.md](scouting.md). |

**4. Feed writes and commit order**

Create `src/server/internal/store/feed.go`, `feed_test.go` and `check-feed-lock.sh` under `src/server/internal/store/`. This item owns `Tx` and needs items 1 and 3, including item 1’s gates. It can run beside item 2 and precedes items 5 and 7. It implements D3, D4, D5, R4, R7, R10 and C12. (logic-r3 3, critic-r1 10)

```go
type Tx struct { /* private transaction and pending changes */ }
func (tx *Tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
func (tx *Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
func (tx *Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row

const MaxWriteChanges = 256
var ErrTooManyChanges error

type Change string

const (
    Saved   Change = "saved"
    Removed Change = "removed"
)

type ChangeRef struct {
    Topic  string
    Scope  string
    Key    string
    Change Change
}

type FeedRow struct {
    Position int64
    ChangeRef
    WrittenAt time.Time
}

func (s *Store) Write(ctx context.Context, write func(context.Context, *Tx) error) error
func (tx *Tx) Record(change ChangeRef) error
```

The callback writes current state and calls `Record` for each changed domain record using a non-secret reference. `Record` validates topic, scope and key as PostgreSQL text. Topic and key must be nonempty. It accepts only `saved` and `removed`. Its first error poisons the transaction, even if the caller ignores the result. The 257th reference returns `ErrTooManyChanges`. A part whose larger operation permits smaller atomic units splits it explicitly. Otherwise the operation returns the bound error without partial changes. A record that moves scope emits `removed` in the old scope followed by `saved` in the new scope in the same write. The Store team writes the limit and sentinel. Domain services read them and own the operation boundaries. (threat-r1 4, critic-r1 6, logic-r1 11)

After a successful callback and any receipt insert, execute `SET CONSTRAINTS ALL IMMEDIATE` before taking the feed lock. This runs pending deferred checks while no feed lock is held. The production feed migration contains no deferred checks or triggers that can acquire domain locks during commit. Store callbacks use no transaction-control or store-owned configuration statements. The database guard rejects direct feed inserts without the flush flag. It is a guard against accidental use by application code. (critic-r1 1, logic2-r1 4, threat-r1 10, architect-r1 3, threat-r1 5)

If changes are buffered, enqueue the following statements in order in one `pgx.Batch`. For each reference queue one parameterized insert, in `Record` order. Close and drain the batch before committing. A writer with no buffered change skips the batch. **UNREVIEWED MECHANISM** (critic-r1 1, logic-r1 2, threat-r1 4). One ordered batch avoids a request and response for each entry after the lock. It also makes the identity allocation order explicit without relying on execution order inside a multi-row statement.

```sql
SELECT pg_advisory_xact_lock($1::bigint);

SELECT set_config('lgtm.feed_flush', 'on', true);

INSERT INTO feed (topic, scope, key, change)
VALUES ($1, $2, $3, $4);

SELECT pg_notify('lgtm_feed_changed', '');
```

In `feed.go`, queue the lock with `feedLockID` on one line ending in the unique marker `// lgtm:feed-lock`. The Item 4 mutation script reads this marker. (logic-r2 2, architect-r2 6, architect-r2 10)

The identity sequence has cache size 1. Rollback can leave positions unused. Positions are the cursor. They are assigned under the lock and become visible as a committed prefix on the writer or a physical standby. The empty notification takes effect on commit. PostgreSQL releases the lock at transaction end. The store records `written_at` during insertion. Item 6 starts retention from committed visibility instead of this approximation.

A slow commit delays feed writers. Item 1 bounds the wait. Reads and writes without feed changes do not acquire this lock. (architect-r1 7, threat-r1 10)

**Test seams.** Use item 1’s gates and identify each writer with `pg_backend_pid()`. Use fixture `records(id text PRIMARY KEY, value text NOT NULL)`, topic `records`, scope `''` and keys `a` and `b`.

Install fixture constraint trigger `feed_test_gate` on `feed`. It takes the transaction advisory lock `feedTestGateID = 841941100901` only for key `a` and optionally rejects after the wait. The per-Store `afterConstraints(ctx, tx) error` hook must set `SET CONSTRAINTS feed_test_gate DEFERRED` in each commit-gated case of the ordering, rollback, lock release and outcome tests. Require it to succeed before sending the batch. Prove deferral by observing the wait while `pg_stat_activity` shows the writer active on `COMMIT`. The private `beforeCommit(ctx, tx) error` hook runs after the batch and can cancel, panic or adjust the test timeout. Both hooks are nil in production. Store item 4 writes and consumes its fixture key and hooks. (architect-r3 4, architect-r2 10, critic-r1 3, logic2-r1 4)

Tests first in `feed_test.go`.

- `TestFeedPositionsFollowCommitOrder`. Hold A’s test gate and observe A waiting in `COMMIT` after allocating position 1. Start B on a different backend. Poll for either B waiting behind A on `feedLockID` or B’s feed row being visible while A’s row is absent. If B is visible first, release A, join both writers and fail with exactly `writer B became visible before writer A`. This branch runs before any sequence assertion and is the sole source of that sentence. A writer error or missing observation by the fixture deadline is a different failure. If B waits, require neither row to be visible and require the sequence value to equal A’s allocated position. Resolve the name with `pg_get_serial_sequence` and read `pg_sequences.last_value`. The lock-after-inserts mutation fails this sequence assertion. Release A, join both writers and require a cursor to observe A then B. (architect-r3 1, logic-r3 2, logic-r1 1)
- `TestFeedLockStartsAtFlush`. Pause A's callback after its state write and `Record`. Require B to commit before A resumes. Assert B then A in the feed.
- `TestFeedRollbackAndNotification`. Register a raw listener. Force a commit error with the test-only deferred trigger. Assert no state, feed row or notification. Commit a second write with 256 marked references. Assert all positions are increasing in recorded order. Assert one flush batch through the driver's batch tracing seam and one committed notification. (critic-r1 1, threat-r1 4)
- `TestFeedRejectsInsertOutsideFlush`. In separate callbacks attempt direct insert, update, delete and truncate. Check the named guard error. On a seeded row, set the prune flag and delete above the floor. Require `feed_delete_requires_watermark` and no deletion. A normal `Record` succeeds. Each removed guard must fail its case. (logic-r3 5, architect-r1 3, threat-r1 5, threat-r2 7)
- `TestRecordValidationAndLimit`. Exercise an empty topic, empty key, zero byte, invalid change and the 257th reference. Assert the failure is returned at `Record` and keeps state and feed rows out even if the callback returns nil. (critic-r1 6, threat-r1 4)
- `TestFeedLockDoesNotBlockUnrecordedWrites`. Hold the feed lock on a separate backend. Complete a no-change write while one buffered writer waits. In separate cases cancel the waiter and let another reach the lock timeout. Assert the connection is returned and that both error causes are distinguishable. (critic-r1 1, architect-r1 7)
- `TestFeedLockEndsWithTransaction`. Cancel after batch close, panic there and make the deferred trigger fail at commit in three subtests. Keep the writer backend alive where possible. Inspect `pg_locks` and then use a confirmed different backend to write. Require it to finish within one second without a leftover lock. Replacing the transaction lock with a session lock must fail. Exercise cleanup after an aborted transaction even if the mutant attempts to unlock through that transaction. (logic-r1 3)
- `TestFeedChecksDeferredDomainConstraintsBeforeLock`. Give B a `FOR UPDATE` lock on a parent row. A inserts a child with a deferred foreign key to that row and attempts a feed write. Assert A waits on B's parent transaction before acquiring the feed lock. Let B flush and commit. Both writes must finish. Move the constraint check behind the feed lock and require the test to fail. In a separate domain-only race, take two row locks in opposite order. Assert the victim returns a retryable deadlock error and `OutcomeRolledBack`. Item 5 owns the keyed retry test. (architect-r2 4) (logic2-r1 4, architect-r1 7, threat-r1 10)

- `TestFeedCommitErrorOutcome`. Observe the deferred trigger waiting in `COMMIT`. Use `beforeCommit` with `SET LOCAL lock_timeout = '3s'` and keep the gate held. Require `55P03`, `OutcomeRolledBack` and `Retryable`. Separately require rollback on trigger rejection and when `beforeCommit` aborts the transaction and commit returns `pgx.ErrTxCommitRollback`. Test backend loss, caller cancellation and caller deadline as unknown and without a retryable flag or raw cause. This proves the lock timeout at commit. It leaves statement-timeout behavior during commit to the post-build check. **UNREVIEWED FIX** (logic-r4 1, logic-r3 1, threat-r2 2)

This item is risky because it establishes ordering. Run `src/server/internal/store/check-feed-lock.sh` from the repository root with the fixture setting. Refuse tracked or untracked changes reported by `git status --porcelain --untracked-files=all -- src go.mod go.sum`. Archive `HEAD` and record its commit on standard error. In the copy, run `go test -count=1 -v ./src/server/internal/store -run '^TestFeedPositionsFollowCommitOrder$'` and require the test’s pass line. Delete the one marked line in `feed.go`. Require `go build ./...` to pass. Rerun the same test and require a nonzero exit, its failure line and the exact visibility assertion. Only then print `PASS C12 feed-lock mutation: writer B became visible before writer A` and exit zero. Otherwise print a fixed diagnostic and fail. Clean up the copy. Stack’s wrapper consumes the Server script. (architect-r2 1, architect-r2 6, logic-r2 2, architect-r3 5)

The item 4 lead also tests lock-after-inserts and session-lock mutants in disposable copies. For each, record the commit, mutation, failing test and exact message in the item report. The sequence and lock-release assertions must fail respectively. The automated hook only removes the lock. (architect-r2 6, logic-r3 8)

**5. Request receipts**

Create `src/server/internal/store/receipts.go` and `receipts_test.go` in Store. This item needs item 4 and reads item 1’s test gates. It can run beside item 7. It implements D3, D5, R4, R7, R9, R10 and C12. (logic-r3 3)

```go
const RequestKeyHeader = "Lgtm-Request-Key"

type Request struct {
    Key                string
    ReceiptSafe        bool
    Caller             string
    Procedure          string
    FingerprintVersion int16
    Fingerprints        map[int16][]byte
}

var ErrInvalidRequestKey error
var ErrInvalidRequest error
var ErrRequestKeyConflict error
var ErrRequestFingerprintUnavailable error
var ErrReceiptReplyTooLarge error

func (s *Store) WriteOnce(
    ctx context.Context,
    req Request,
    write func(context.Context, *Tx) ([]byte, error),
) ([]byte, error)
```

Authenticate and authorize before writing. An empty key delegates to `Write`. A keyed request fails before taking a lock unless `ReceiptSafe` is true, the caller and procedure are nonempty, the current fingerprint version is positive and its bytes are nonempty. A key is case-sensitive 8-bit Unicode Transformation Format (UTF-8), with no zero byte and at most 256 bytes. Store writes `Request` and the sentinels. Sprint 2 and 3 adapters supply the authenticated principal, procedure, safety declaration and fingerprint. Declare receipt safety only for a procedure whose request fingerprint and reply exclude credentials. Leave keyed credential behavior for the owner. (threat-r2 3, logic-r2 5, logic-r1 6, critic-r1 7)

The adapter documents a versioned semantic fingerprint with length-framed values in fixed order, including presence where it changes the mutation. Encode a request with no mutation fields using a fixed nonempty marker. Keep encoders for retained versions and their replay fixtures. Incompatible mutation semantics require a new procedure identity or a rule refusing incompatible replay. The Store selects the saved version when verifying a receipt. A missing or empty encoder result for that version returns `ErrRequestFingerprintUnavailable`. The adapter maps it to `failed_precondition` with “This server cannot verify the saved request.” The migration writes `receipts.fingerprint_version`. Store and the adapters read the version and sentinel respectively. (logic-r2 5, logic2-r1 3, threat-r1 7)

Hash the authenticated principal with the 256-bit Secure Hash Algorithm (SHA-256) for `principal_hash`. Acquire `pg_advisory_xact_lock($1::int4, hashtext(encode($2::bytea, 'hex') || ':' || $3))` with `receiptLockNamespace`, the principal hash and key. In the same write lock the receipt row `FOR UPDATE` by its composite key. For the request digest, prefix the version as a two-byte big-endian integer, then frame the caller, procedure and semantic bytes with individual eight-byte big-endian lengths. Use the stored version and its bytes on replay. A lock hash collision only delays a different key. (architect-r2 10, logic-r2 5, logic-r1 7, threat-r1 6)

A matching digest returns the stored reply without invoking the callback. On a genuinely different request, return `ErrRequestKeyConflict` for `already_exists` with “That request key was already used for a different request.” Return `ErrInvalidRequest` for malformed metadata or an unset safety declaration, and `ErrInvalidRequestKey` for an invalid key. Both map to `invalid_argument`. With no receipt, run the callback. Return `ErrReceiptReplyTooLarge` and roll back above 65,536 bytes. Store a zero-byte reply for an empty success. Insert the receipt before feed flush and return bytes after commit. Never log the key, hash or reply. A retry after expiry can start a new write. (critic-r1 7, threat-r1 2, threat-r2 3)

Tests first in `receipts_test.go`.

- `TestWriteOnceConcurrentRetry`. Hold A's callback after a state insert. B uses the same authenticated principal, method, key and fingerprint. Use `WaitForBlock` with `SubID=2`, the `receiptLockNamespace` and the hash word computed by PostgreSQL. (architect-r2 10, logic-r3 3) Release A. Assert one callback, one receipt, one state change and one feed row. Require identical replies across both callers and a reopened store. (critic-r1 7)
- `TestWriteOnceMismatch`. With the same principal and key, change the semantic fingerprint or procedure. Assert conflict with no reply or callback. Move a byte from the end of the procedure to the start of an otherwise identical fingerprint. The tuple must remain distinct. With a different authenticated principal the same key succeeds independently. Add invalid key, missing caller, missing procedure, missing or empty current fingerprint and an unset `ReceiptSafe`. The safety refusal takes no receipt lock, invokes no callback and creates no receipt. (threat-r2 3, logic-r2 5) (logic-r1 6, logic-r1 7, threat-r1 6)
- `TestWriteOnceVersionedReplay`. Retain a version 1 receipt. Reopen with a version 2 adapter that supplies a version 1 encoding and require byte-identical replay. A changed body conflicts. Missing and empty encodings of the stored version return `ErrRequestFingerprintUnavailable` with no callback. Hash equal fingerprint bytes with distinct version numbers and require different digests. Test a zero-byte successful reply. (logic-r2 5, logic2-r1 3, threat-r1 7)
- `TestWriteOnceRollbackAndUnkeyed`. Use the commit gate to fail after receipt insertion and feed allocation. Assert no state, receipt or feed row. Retry with a changed fingerprint and assert success. Confirm two unkeyed successes run twice and create no receipt. Reject an oversized reply and assert state and feed rollback. (critic-r1 7)
- `TestWriteOnceRetriesAfterDeadlock`. Use two transactions taking fixture row locks in opposite order. With a key for each call, require the victim’s rolled-back `40P01`, then retry its original key and require one committed change and its replayable receipt. This test needs only items 1, 3 and 4. (architect-r2 4)
- `TestWriteOnceReleasesKeyLock`. Cancel or panic in the keyed callback and force a separate deferred commit failure. Check the receipt advisory namespace and replay on another backend. Closing the failed transaction must release its key lock. Replacing the key lock with a session lock must fail in the aborted cases. (logic-r1 3)

This item is risky because request keys protect writes and replies. Deleting the key lock, replacing it with a session lock, bypassing the digest comparison or hashing an unframed tuple must fail the named tests.

**6. Feed and receipt retention**

Create `src/server/internal/store/prune.go` and `src/server/internal/store/prune_test.go`. This item needs items 1 and 3. It can run beside items 4, 5, 7 and 8 when their dependencies are ready. (critic-r1 10) It implements D3, D5, R7, R10 and C12.

```go
type PruneResult struct {
    FeedRows    int64
    ReceiptRows int64
}

func (s *Store) Prune(ctx context.Context) (PruneResult, error)
func (s *Store) RunPruner(ctx context.Context, report func(error))
```

`RunPruner` sweeps on startup, then waits one hour after each completed sweep. One sweep runs at a time. Use private `runPruner` with a fixture clock and `pruneBefore(ctx, cutoff)` for expiry tests. `Prune` samples `clock_timestamp() - interval '24 hours'` once and returns committed counts. The Store owns `PruneResult` and returns counts to test callers. **UNREVIEWED MECHANISM** (logic-r3 7). A completion-based schedule avoids overlapping runs. Removing the diagnostic avoids an alert that cannot distinguish downtime from a stall. (critic-r1 6, critic-r1 11)

Stamp only visible, unmarked rows after acquiring their row locks. Sample PostgreSQL time after acquiring the locks. A row is eligible once `retention_started_at < cutoff`. A late commit therefore gets a full day from observation if the database clock advances normally. `written_at` is for diagnostics. An administrative forward jump can expire more than one day of receipts in one run. A backward adjustment can extend retention. (threat-r2 6, logic2-r1 1, logic-r1 8)

Complete feed stamp and delete phases before the receipt phases. Repeat committed batches of up to 512 until a batch is short. Each feed batch locks the `feed_retention` singleton and sets the prune flag. Stamp ascending positions. Select the expired prefix of the earliest 512 positions. Advance `pruned_through` first and delete that same prefix in the transaction. A retained position ends deletion for the sweep. Leave the sequence in place. Report a blocked prefix when a retained row prevents deletion of a row older than twice the retention. (logic-r2 1, logic-r3 5, threat-r2 7, critic-r1 11)

For receipts, select eligible rows in index order and lock `FOR UPDATE`. A replay that holds the row first completes with its saved reply. If deletion commits first, a later retry can run anew. The batch uses item 1’s maintenance limits. Item 6 uses its own raw SQL fixtures and item 1’s advisory gates. The merged receipt check covers both commit orders. **UNREVIEWED MECHANISM** (architect-r2 4). This keeps pruning independently mergeable. (critic-r1 7)

Tests first in `prune_test.go`.

- `TestPruneCutoffAndWatermark`. Seed at least 1,300 expired feed rows in addition to multiple unstamped feed pages and old receipts. A single sweep stamps every visible unmarked row, deletes all expired feed rows and receipts, retains rows at or after the cutoff and preserves part-owned state. Assert committed counts and the last deleted position. Run again for zero deletions. After pruning the whole feed in a separate case, the next raw fixture insert must get a higher position. A one-page stamp or delete mutant must fail. (logic-r2 1)
- `TestPrunePreservesFeedPrefix`. Seed old and fresh observed times out of position order. Assert cleanup stops before the fresh row and reports a blocked stale prefix when another row is old enough. Use a future `written_at` on a row with an old observation time and prove that the diagnostic time does not block its deletion. (logic-r1 8, critic-r1 11)
- `TestPruneStartsAgeAfterCommit`. Set backdated insertion times for a receipt and feed row in an uncommitted fixture transaction. Run cleanup before and after releasing commit. Assert observation is null until visible, then newer than the cutoff for both rows. Run a cutoff sweep and confirm the receipt survives. (architect-r2 4) (logic2-r1 1)
- `TestPruneWaitsForReplayOfSameKey`. Hold an eligible receipt row `FOR UPDATE` from the raw replay fixture. Start pruning. Require feed cleanup to commit before the receipt lock wait ends. The holder reads the original reply and commits. Require the sweep to delete the expired row. Seed a receipt exactly at the cutoff and verify it survives until the cutoff advances. The merged check uses the public replay path in both lock orders. (critic-r1 7)
- `TestRunPrunerLifecycle`. Require immediate startup, one sweep per explicit clock advance and a one-hour delay after completion. Advance the clock during a held sweep and require no overlap or queued sweep. In a separate case fail a gated sweep and require a sanitized error, recovery and cancellation. (logic-r3 7, critic-r1 6)

This item is risky because expiration changes progress and permits a key to run again. Ignoring the feed prefix, removing its watermark or expiring directly on `written_at` must fail the real PostgreSQL tests.

**Design artifact.** The feed, ordering and safe-retry sections of the approved design supply the brief. This workstream has no screen.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A cursor advances past a late commit and skips a change. | High | Allocate positions only after acquiring the final-flush advisory lock and prove the exact failure when it is removed. |
| A repeated key causes a second write or exposes another reply. | High | Serialize same-key writes in PostgreSQL. Bind the digest to the authorized caller and procedure as well as the body. |
| Cleanup makes an old cursor silently cross a deleted entry. | High | Delete an ordered prefix and record its watermark in the deletion transaction. |

**Verify.** Run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/store/...`. Use rollout step 3 for the archived lock-removal check. Record the assertion and tested commit. (logic-r2 2)

**Rollout detail.** Step 2 installs the embedded store migration on the empty application database. Step 3 proves the mutation with two writer connections and a separate observer. A later server startup runs `RunPruner` after `Migrate` and cancels it before closing the store.
**WS-3. Watch starts with a snapshot and follows live changes**

One reader per process fetches committed rows. A bounded buffer shares those rows among the open streams. Each stream reads records and its starting position together, so a change can occur while the stream is opening without being lost.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 7 | A committed feed row reaches the reader on a notification. With no notification it arrives on a poll, in order and once. | `TestFeedReaderNotificationAndPolling`, `TestFeedReaderCatchupAndRetry` and `TestFeedReaderBoundsStalledQuery`. |
| 8 | A subscriber is cut off at the lag limit, past the retained cursor or during a feed outage. A draining subscriber continues during another subscriber’s lag. | `TestFollowerLagAt2048`, `TestFollowerPrunedCursor` and `TestFollowerEndsStaleStreams`. |
| 9 | The first message contains the entire result, followed by one message per later matching row. A blocked stream can be canceled and reopened with a fresh snapshot. The stream also stops when authorization ends. | `TestWatchSnapshotUsesOneTransaction`, `TestWatchFiltersAndLoadsCurrentRecords`, `TestWatchCanRestartAfterLag` and `TestWatchRechecksAuthorization`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 7 | Read ordered pages, consume database notifications, and poll when none arrive. | `src/server/internal/store/read_feed.go` `readFeedPage`, `runFeedReader` and `Snapshot`. See S7 in [scouting.md](scouting.md). |
| 8 | Fan out the feed to a bounded shared buffer and cancel lagging subscribers. | `src/server/internal/store/follow.go` `StartFollower`, `Subscribe`, `Start`, `Next` and `Ack`. See S8 in [scouting.md](scouting.md). |
| 9 | Serve a typed snapshot and later updates. | `src/server/internal/watch/watch.go` `Serve`, `Source` and `Message`. See S9 in [scouting.md](scouting.md). |

**7. Read committed pages with notifications and polling**

Create `src/server/internal/store/read_feed.go` and `src/server/internal/store/read_feed_test.go`. This item needs item 4 and can run beside items 5 and 6 when ready. It implements D3, D4, D5, R5, R7, R10 and C12.

```go
func (s *Store) Snapshot(
    ctx context.Context,
    load func(context.Context, *ReadTx) error,
) (int64, error)

var ErrFeedUnavailable error

type feedPage struct {
    PrunedThrough int64
    Rows          []FeedRow
}

func (s *Store) readFeedPage(ctx context.Context, after int64) (feedPage, error)
```

`Snapshot` admits one snapshot at a time before acquiring from the main pool. Return `ErrFeedUnavailable` if the slot stays busy for two seconds, or the caller’s cancellation if earlier. In the acquired read transaction, `readHead(ctx, tx)` returns the greater of the highest present position and `pruned_through` before `load` reads records. A new feed starts at zero. Item 7 writes the sentinel. Item 8 and Watch read it and sprint 3 maps it to `unavailable`. **UNREVIEWED MECHANISM** (architect-r3 6). One snapshot leaves capacity alongside the full writer limit and one pruner. (critic-r1 6, logic2-r1 5)

`readFeedPage` uses the page pool. In one read transaction, read `pruned_through` and query up to 512 rows with `WHERE position > $1 ORDER BY position ASC LIMIT 512`. Start above the greater of the cursor and the floor. Drain again after a full page. Only an accepted page advances the cursor. Reject a page whose first row repeats or precedes the cursor. Treat rejection as a failed attempt. (logic-r3 4, critic-r1 11, logic-r1 7, logic-r1 9)

One `runFeedReader(ctx, s, after, options, apply)` goroutine owns the cursor. Poll every two seconds and drain on a one-slot wake channel. A separate listener wakes it on registrations and notifications for `lgtm_feed_changed`. Allow three seconds to dial and register. Reconnect at most once per poll interval. Polling works without a listener. Each page has two seconds for acquisition and queries and one more second for rollback. (logic2-r1 2, logic-r1 9)

**Test seams.** Items 7 and 8 use these private options. Defaults use the clock and ticker and enable the listener. `afterAcquire` can hold a fixture lock before the query. `onPage` runs after acceptance, even on an empty page. `onListen` observes the listener backend after registration. Use buffered channels in test callbacks. (critic-r1 3)

```go
type feedReaderOptions struct {
    ticks           <-chan time.Time
    now             func() time.Time
    disableListener bool
    onListen        func(int32)
    onReadError     func(error)
    afterAcquire    func(context.Context, *ReadTx, int64) error
    onPage          func(feedPage)
}
```

Tests first in `read_feed_test.go`.

- `TestFeedReaderNotificationAndPolling`. Start with a test-supplied tick channel and wait for actual listener registration. Commit a real row and receive it before sending a poll. With the listener disabled, send a poll after the next commit and require delivery. Exercise a notification and tick together at another commit and require one delivery across accepted pages. Assert the production interval is two seconds. (logic-r1 9)
- `TestFeedReaderCatchupAndRetry`. Commit over 1,024 fixture rows. Let the first page of 512 apply. For the next page, use `afterAcquire` to capture and terminate that backend through a separate connection. Confirm a read error. Resume on a separate tick and assert that all rows arrive exactly once and in order. A later empty page proves the reader reached the final cursor. Use fixture SQL to verify pages read across a pruned prefix. (logic-r1 7)
- `TestFeedReaderBoundsStalledQuery`. The acquisition hook issues a real advisory wait on a lock the observer holds. Keep the lock until the production two-second query budget expires. Require a timeout while the lock remains held. Release the gate and signal the next tick. Assert the next read resumes at the last applied position. (logic2-r1 2)
- `TestSnapshotRetainsPrunedHead`. Advance the durable floor, set the prune flag and remove the feed in one fixture transaction. Assert that `Snapshot` returns the floor, even with no rows left. (critic-r1 6)

- `TestFollowerProgressesUnderReadSaturation`. Hold all eight main pool connections with ordinary `Read` calls. Commit a feed change from a second Store. Require the page on the next poll within its two-second budget. Using the main pool for pages must fail. (architect-r2 5, architect-r3 6)

- `TestSnapshotAdmissionLeavesReadCapacity`. Hold one snapshot in its loader. Queue eight snapshots and require each to return `ErrFeedUnavailable` within its two-second budget, with fixture tolerance, without acquiring a connection. While the first snapshot is held, run a session-style `Read` and an item 4 `Write`, and require both to finish. The merged check uses a keyed write. Removing snapshot admission must fail. (architect-r3 6)

Removing polling, letting two drains apply the same page, using an inclusive cursor, resetting the cursor on error or removing the production page deadline must fail the tests above. The item is risky because it can lose a committed entry.

**8. Bounded broadcast and subscriber lifetime**

Create `src/server/internal/store/follow.go` and `src/server/internal/store/follow_test.go`. This item needs item 7 and can run beside item 6 when ready. It implements D3, D5, R5, R7, R10 and C12.

```go
const MaxWatchLag = 2048

var ErrWatchLagged error
var ErrSubscriptionState error

type Follower struct { /* private reader and subscriber state */ }
type Subscription struct { /* private cursor and cancellation state */ }

func StartFollower(ctx context.Context, s *Store, report func(error)) (*Follower, error)
func (f *Follower) Close()
func (f *Follower) Subscribe(ctx context.Context) (*Subscription, error)
func (sub *Subscription) Context() context.Context
func (sub *Subscription) Start(after int64) error
func (sub *Subscription) Next() (FeedRow, error)
func (sub *Subscription) Ack(position int64) error
func (sub *Subscription) Close()
```

After migrations, `startFollower(ctx, s, report, options)` reads the initial head via the page pool. The follower keeps 2,048 rows in a shared ring and tracks its initial head, evicted positions and the pruning floor. Lock the ring and subscriptions only while updating their state.

**Lag rule.** Lag is the count of actual observed committed feed rows with position greater than the subscriber's acknowledged cursor. Count rows across all topics and scopes. The in-flight row counts once until acknowledged. Cancel with `ErrWatchLagged` at lag greater than or equal to `MaxWatchLag`. Evaluate on `Start` and for each row as a page is applied. A cursor older than the unavailable, evicted or pruned marker is separately invalid because history is missing. Equality with the marker is usable. For example, one in-flight row plus 2,046 later rows leaves the stream open. One additional row ends it. (architect-r1 5, critic-r1 4, logic-r1 5)

`Subscribe` registers before the snapshot. `Start(after)` sets a cursor once. Ignore rows through its cursor if the snapshot saw a newer commit. A snapshot that already exceeds the lag limit receives the same lag error as a slow sender. (critic-r1 11)

`Next` delivers one in-flight row. It returns `ErrSubscriptionState` before `Start` or while a row is still outstanding. `Ack` accepts only the outstanding position. Invalid `Start` and `Ack` calls leave state unchanged. A consumer acknowledges filtered rows as it skips them. A matching row is acknowledged only after the send returns. Cancel the subscription context on lag so ongoing work can stop before another `Next`. `Close` is idempotent on the follower and subscription. Follower close cancels and joins the reader and listener. Subscriber close removes the subscription. The Store team writes these methods and the state sentinel. The Watch helper reads them. (critic-r1 6, logic2-r1 7)

Only accepted pages, including empty pages, reset the ten-second stale clock. On expiry report once, cancel streams and refuse new ones with `ErrFeedUnavailable`. An accepted page clears the outage and leaves the cursor at its applied position. The follower expects increasing positions. Startup supplies the reporter and Watch reads the error. **UNREVIEWED FIX** (logic-r4 3, logic-r3 4, logic-r2 3)

**Test seams.** Use the item 7 configuration to disable the listener, request a poll, advance fixture time and check when a page applied. Use `records(id text PRIMARY KEY, value text NOT NULL)` and source topic `records`. Use the item 1 gates and deadlines. For a deliberately over-limit single commit, tests may seed entries directly in an isolated database under the feed lock with the guard flag. The public writer remains capped. (critic-r1 3, critic-r1 4, threat-r1 4)

Tests first in `follow_test.go`.

- `TestFollowerLagAt2048`. Start a slow and a fast subscriber at the same head. Run boundary cases with no `Next`, with an unacknowledged first row and with a mix of filtered rows. Check lag 2,046 and 2,047 stay open. In the in-flight case these counts already include that row. At 2,048 require cancellation and `ErrWatchLagged`. In a separate case apply a full 512-row page that crosses the limit. The fast subscriber starts with enough acknowledged progress to receive the boundary and subsequent rows. (architect-r1 5, critic-r1 4, logic-r1 5)
- `TestFollowerBulkWriteCutsOffEveryone`. Seed one 2,047-row test commit and then a separate 2,048-row test commit in an independent database. Without acknowledgements, all subscribers stay open after the first case and are canceled after the second. Use the fixture injection because the production path rejects either oversized single write. (critic-r1 4, threat-r1 4)
- `TestFollowerPrunedCursor`. Activate one subscriber with a row still in flight. In one case, seed eviction before an older subscriber activates. In another, advance the floor and delete through it in one flagged fixture transaction while polls are paused. On the next poll require cancellation even though the first subscriber never calls `Next` again. A new snapshot at the head can receive a later row.
- `TestFollowerIgnoresSequenceGaps`. Create a large rolled-back sequence gap in the isolated database. One committed row after it must not trigger lag. An ahead-of-reader subscriber ignores positions through its cursor and receives the next higher committed row.
- `TestFollowerSubscriptionLifecycle`. Reject `Next` before `Start`. After `Next` for position 41, reject a second `Next` and an incorrect `Ack`. Accept `Ack(41)` and deliver the next row. Reject a second `Start`. Repeated `Close` on each object is harmless and closes blocked waits. (logic2-r1 7, critic-r1 6)
- `TestFollowerEndsStaleStreams`. Accept empty pages for twelve fixture seconds and assert no outage. Run separate stalled database and rejected-page cases. In the latter case pass a repeated-position page through the same acceptance path and require no clock reset. After ten seconds without acceptance, existing and new streams fail with `ErrFeedUnavailable` and one report is made. Accept a page and verify recovery. (logic-r3 4, logic-r2 3, logic-r1 4, logic2-r1 2, critic-r1 8)

Gap and boundary tests must fail numeric-distance lag or a strict greater-than cutoff. The outage test must fail if invalid pages reset the clock. (logic-r3 4)

**9. Typed Watch streams**

Create `src/server/internal/watch/watch.go` and `src/server/internal/watch/watch_test.go`. This item needs item 8 and the shared Watch shape from sprint 0. It can run beside item 6 if that item remains open. It implements D3, D5, R2, R3, R5, R7, R8, R10 and C12. It provides the stream behavior that the Machines service needs for C6 through C9.

```go
type Source[T any] struct {
    Topic    string
    Scope    string
    Authorize func(context.Context, *store.ReadTx) error
    Snapshot  func(context.Context, *store.ReadTx) ([]T, error)
    Load     func(context.Context, *store.ReadTx, string) (T, bool, error)
}

type Snapshot[T any] struct {
    Records []T
}

var ErrInvalidSource error
var ErrWatchUnauthorized error
var ErrAuthorizationUnavailable error

type Message[T any] struct {
    Position   int64
    Snapshot   *Snapshot[T]
    Saved      *T
    RemovedKey string
}

func Serve[T any](
    ctx context.Context,
    s *store.Store,
    follower *store.Follower,
    source Source[T],
    send func(context.Context, Message[T]) error,
) error
```

The service checks the requested scope and session before calling `Serve`. `Serve` rejects an empty topic or a nil authorize, snapshot, load or send function with `ErrInvalidSource` before subscribing. A source supplies readers over its own tables and restricts those readers to the authorized topic and scope. The helper registers before taking the snapshot. Its first repeatable read fixes the feed head, checks authorization and loads records in the same snapshot. Activate the subscription before emitting the full snapshot. An empty list still sets the snapshot variant. (logic-r1 6, logic2-r1 5)

Before a matching message, reauthorize using a read-only transaction. A save loads the current record in that same authorization transaction. A removed row emits its key. A save to a now-missing record emits the key too. Acknowledge filtered rows as skipped. Acknowledge sent rows only after a successful send. No transaction stays open during network delivery. A move across scopes yields a removal to the old watcher and a save to the new one. (logic-r1 11, critic-r1 6)

Run the same bounded authorization check before the first snapshot and each matching message, and every 30 seconds while quiet or blocked. Cap authorization at two seconds, including acquisition for separate authorization reads. A nil result permits delivery. `ErrWatchUnauthorized` cancels the stream and maps to `unauthenticated` with “Sign in again to reopen this watch.” Every other error, timeout or recovered authorizer panic cancels with `ErrAuthorizationUnavailable`. The Machines adapter maps it to `unavailable` with “The watch authorization check is unavailable, try again.” Drop the error cause and panic value. The helper writes both sentinels using `store.NewDomainError`. The sign-in part supplies its session reader and Machines binds it to `Source.Authorize`. (threat-r2 4, threat-r1 1)

The Server Watch team writes the internal signed `Message.Position` and the three variant fields. The Contract team writes `FeedPosition.value` as `uint64`. The sprint 3 Server adapter validates that the store value is nonnegative, converts it to unsigned and always emits a present `position`, including zero for a new database. Map `Snapshot`, `Saved` and `RemovedKey` to `all`, `saved` and `removed_id` respectively. Exactly one variant is present. Contract and Server compare these items using the draft and generated descriptors. The current browser consumes the three variants. (architect-r1 4, critic-r1 5, logic2-r1 6)

The sender honors the subscription context even when a client blocks transport. The adapter maps `ErrWatchLagged` to the draft's Connect `aborted` with “The watch fell behind and must be reopened.” Map `ErrFeedUnavailable` to `unavailable`, using “The change feed is unavailable, try again.” Use those statuses when the transport can still write. A blocked response must end at the transport, using a per-response write deadline triggered by cancellation or an equivalent operation supported by the server. The current browser reopens the stream after closure. The later Machines adapter owns this behavior and its real transport test. (architect-r1 2, critic-r1 5, logic2-r1 6)

**Test seams.** Use `records(id text PRIMARY KEY, scope text NOT NULL, value text NOT NULL)` and `sessions(id text PRIMARY KEY, active boolean NOT NULL)`. Use topics `records` and `other`. Use scopes `p1` and `p2`. The authorizer queries `sessions`, returning `ErrWatchUnauthorized` when the selected row is absent or inactive. Watch test code uses the exported `Follower.Subscribe`, `Start`, `Next` and `Ack` to observe a known committed marker without adding an exported testing interface. Private `serve[T any]` takes the same arguments as `Serve` plus `ticks <-chan time.Time`. Nil uses the production 30-second ticker. Reuse the item 4 fixture deadline and silence check. (critic-r1 3)

Tests first in `watch_test.go`.

- `TestWatchSnapshotUsesOneTransaction`. Put the competing write before the loader's first query in one case. The head-first implementation has already established its repeatable read. Put the competing write after the loader's first query in a second case. Assert both cases send the old record and old position followed by the new change. Run with an initially empty table and require a present empty snapshot. Freeze both split mutations. Head-first in a separate transaction yields an old head with new data. Records-first with a separate head read can skip the new entry. The relevant barrier must detect each changed result. (logic2-r1 5)
- `TestWatchFiltersAndLoadsCurrentRecords`. Assert exactly one variant in the empty snapshot and each emitted change. Commit interleaved changes under a different topic and scope, a matching save and a removal. Hold the initial snapshot sender while a subsequent write commits. Then let the change loader open its read transaction and assert that the earlier saved position uses the current data. Use a matching marker to check the exact positions and the absence of nonmatching messages. Verify a now-missing saved record emits a removal and a later removal at its own position. Move a record between scopes and require the old watcher to receive a removal and the new watcher to receive a save. A write can complete while a sender is held. (critic-r1 6, logic-r1 11)
- `TestWatchCanRestartAfterLag`. Hold the sender until cancellation. Write batches of at most 256 changes until the follower reaches the limit. Require the sender and `Serve` to return the lag cause. Reopen and require the first snapshot to cover the new records. Also exercise caller cancellation.
- `TestWatchRejectsInvalidSource`. Supply each missing callback and an empty topic in separate cases. Require `ErrInvalidSource` without a sender call. (logic-r1 6)
- `TestWatchRechecksAuthorization`. Revoke the session after the snapshot, commit a matching change and assert no send. Trigger an idle and a blocked-send tick with revocation, arbitrary error, timeout, and a panic in separate cases. Require the respective unauthorized or unavailable cause and cancellation of the sender. No raw error or panic value reaches output. (threat-r2 4, threat-r1 1)

The tests must fail either split snapshot mutation, a writable or `READ COMMITTED` snapshot or an authorizer that permits delivery after revocation. (architect-r2 4)

**Design artifact.** The approved design's Watch procedure and the existing Machines stream are the shape references. This workstream has no screen.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A notification is missed or a read is interrupted. | Medium | Keep a cursor and bound each attempt. Prove polling without a listener and stream cancellation when reads stay unavailable. |
| A slow network stream holds an unbounded backlog or blocks another stream. | High | Share one fixed ring and apply the lag rule during publication. Require a cancellable sender. |
| A write lands between the initial record read and the cursor read. | High | Use one read-only repeatable-read transaction and a gated two-connection test. |

**Verify.** Against disposable PostgreSQL, run `go test -race -count=1 ./src/server/internal/store/... ./src/server/internal/watch/...`. Run the notification-disabled branch without a listener to exercise the standby delivery path on a real primary. This sprint needs no device. The actual standby and the Connect transport adapter are outside the sprint's deployment. The later Machines service owns the blocked-transport test.

**Rollout detail.** Sprint 2 supplies session lookup and startup. Sprint 3 maps the reply and tests blocked transport. On shutdown, stop streams and join follower and pruning work before closing the Store. (threat-r1 1, architect-r1 2)

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is the approved design's decisions D1 through D23, rules R1 through R11 and wave-one checks C1 through C13. The local sections of that design also define the database layer, and the sprint brief supplies its eight work items and four open points. Statements below follow the order in the design on `master`. [scouting.md](scouting.md) gives the section anchors.

| Source item | Workstream | Status |
|---|---|---|
| D1 | WS-3 | The store and Watch helper remain server internals. The shared message shape is a sprint 0 dependency. |
| D2 | WS-1 | The foundation and identifiers are written in Go. |
| D3, D4 | WS-1, WS-2, WS-3 | Use PostgreSQL for durable state and ordering while running one server. The Stack team owns deployment. |
| D5 | WS-1, WS-2, WS-3 | Current tables, one change feed and reads from current state. |
| D6 to D8 | Outside this plan | Agent steering, machine proof and the agent protocol belong to later work. |
| D9, D10 | WS-2 | Credential-bearing calls require an owner decision before wiring receipts. Vault setup and secret access belong to the stack and sign-in plans. |
| D11 to D23 | Outside this plan | Sign-in cookies, machine and link behavior, secrets and later agent behavior are owned by later work. |
| R1 | WS-1 | Each part registers its own manifest. |
| R2 | WS-3 | The store consumes the reviewed shared Watch semantics. |
| R3 | WS-1, WS-3 | Part-owned queries sit in callbacks and source readers. |
| R4 | WS-2 | Current state, receipt and feed rows share a transaction. |
| R5 | WS-3 | Implement the shared Watch lifecycle. |
| R6 | Outside this plan | Stored machine commands belong to the link. |
| R7 | WS-1, WS-2, WS-3 | PostgreSQL holds durable state. Feed buffering is reconstructible. |
| R8 | WS-2, WS-3 | Define error sentinels and the mapping for service adapters. Handler changes belong to later plans. |
| R9 | WS-2 | Persist a digest and receipt-safe reply. Bind values as parameters and sanitize diagnostics. |
| R10 | WS-1, WS-2, WS-3 | Use real PostgreSQL. Link integration stays with the link plans. |
| R11 | WS-1, WS-2, WS-3 | Implement in Go. Repository reference patterns supply ideas. |
| C1 to C5 | Outside this plan | Stack startup, sign-in and join-token handling belong to the later plans. |
| C6 to C9 | WS-3 | Provide the Watch foundation. The Machines service and link own the full checks. |
| C10, C11 | Outside this plan | Peer proof and replay belong to link integration. |
| C12 | WS-1, WS-2, WS-3 | Run real database tests and observe the independent lock-removal failure. |
| C13 | Outside this plan | Vault restart and signing-key retrieval belong to the stack and server startup. |
| Design database layer, replication rules and numbered changes | WS-1 | Supply query handles, version 7 identifiers, independent migrations and test databases with package-specific schemas. |
| Design current tables, commit order and safe retries | WS-2 | Specify feed and receipt columns, allocation, deduplication and cleanup. |
| Design following the feed and the Watch procedure | WS-3 | Provide polling, wake-ups, cursor retention and the typed stream. |
| Brief items 1 to 3 and open point 1 | WS-1 | Deliver the interfaces for connections, identifiers and migration registration. |
| Brief items 4, 6 and 7 and open point 4 | WS-2 | Deliver ordered feed writes, cleanup and receipts with their file ownership. |
| Brief items 5 and 8 and open points 2 and 3 | WS-3 | Split committed reads from subscriber buffering and specify typed streaming and lag cost. |
| Issues 1632 through 1635 | WS-2, WS-3 | Cover 1632's commit-order and shared-shape points. The encryption and test entry points in 1633 and 1634 belong to the link plans. Use low and medium sizing per 1635. |
| Build-loop design, phase 4 | WS-1, WS-2, WS-3 | Name frozen tests, risky mutations, exact-commit verification and dependency order. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, What this project is | Write Go in the root module. Keep the website and the client outside the store. The shared shape comes from the contract. |
| `AGENTS.md`, Where things are | New implementation paths are in `src/server/internal/store/` and `src/server/internal/watch/`. The old engine and the mock are read as references. |
| `AGENTS.md`, habit 1 | The implementation uses existing patterns as design references. Any copied code needs its source, license and `NOTICE` entry through the repository's normal process. |
| `AGENTS.md`, habit 2 | Record issues and choices in this plan and its result during the bounded assignment. The orchestrator owns changes to the repository-wide issue list. |
| `AGENTS.md`, habit 3 | The orchestrator handles the pull request to the base branch. During the build, the lead checks the exact commit before merging team work. |
| `AGENTS.md`, habit 4 | The server foundation has no work in the referenced external repository. |
| `AGENTS.md`, Writing style for documents | Check these files for concrete wording and private identifiers. Run `node scripts/check-private.mjs` before handing off. |
| `.plans/epics/go-rewrite/epic.md`, The teams, and what each may edit | Contract establishes the module and shared shape. The Server builder adds the listed pin with the first import. The orchestrator checks the change. **UNREVIEWED FIX** (2026-11-26 epic ruling) |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, Rules that every later part follows | Parts own their tables and use the store's write path. The common Watch helper reads current state. Integration tests use PostgreSQL. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Backend covers server logic and SQL here. Item 3 is a startup runner that also provides the first schema on an empty database. There is no conversion of existing data. The `server/backend` scope is kept by the orchestrator. The table names the required review pass for boundary or ordering changes. All item-level acceptance tests remain mandatory. Work can run in parallel once the named prerequisites have merged. Each item owns a separate set of files.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | backend | server/backend | medium | The pool, isolation and fixture cross the PostgreSQL connection boundary. | Yes | Sprint 0 module. The builder lands the pin and import together. **UNREVIEWED FIX** (2026-11-26 epic ruling) |
| 2 | backend | server/backend | low | The format uses one documented byte layout and a small pure test seam. | Yes | Sprint 0 module. |
| 3 | backend | server/backend | medium | One runner applies a validated manifest in a serialized startup transaction. | Yes | 1. |
| 4 | backend | server/backend | medium | One ordered flush reuses a shared database gate. Separate gate setup and transaction assertions. | Yes | 1 and 3. |
| 5 | backend | server/backend | medium | The receipt check wraps the existing write path and adds one per-key lock. | Yes | 4. |
| 6 | backend | server/backend | medium | Bounded observation and deletion preserve the committed age and watermark. | Yes | 1 and 3. |
| 7 | backend | server/backend | medium | One reader reconciles an advisory signal with ordered reads and a periodic fallback. | Yes | 4. |
| 8 | backend | server/backend | medium | One bounded memory structure owns subscribers and their cancellation. | Yes | 7. |
| 9 | backend | server/backend | medium | One snapshot-to-stream helper reuses the subscriber and session callback. | Yes | 8 and sprint 0 Watch shape. |

Load for the first review and build is 12 OpenAI launches, 15 Anthropic launches and five xAI launches. Each further full review adds one OpenAI, four Anthropic and one xAI launch. The orchestrator compares those counts with remaining allowances.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (blocker, major, minor) | Outcome |
|---|---|---|
| 1 | 0 blockers, 19 majors and 30 minors before deduplication. | See [the round 1 register](attack/round-1-findings.md). |
| 2 | 0 blockers, 10 majors and 15 minors before deduplication. | See [the round 2 register](attack/round-2-findings.md). |
| 3 | 0 blockers, 6 majors and 9 minors before deduplication. | See [the round 3 register](attack/round-3-findings.md). |
| 4 | 0 blockers, 3 majors and 0 minors. | See [the round 4 register](attack/round-4-findings.md). Check the unreviewed fixes first after the build. |

</details>

<details><summary><b>Post-build checks from the review rounds</b></summary>

Start with the rows marked `UNREVIEWED FIX`. Use PostgreSQL 16 and 17 when available. Keep the tested commit, commands and observations.

| Area | Scenario | Expected result |
|---|---|---|
| Integration | Run `go build ./...` and rollout step 3 on the merged sprint 1 commit. Also exercise a run without the fixture setting. | Bounded root tests and the hook pass. Ordinary tests skip database cases without the setting and C12 refuses that run. **UNREVIEWED FIX** (logic-r4 2, architect-r2 3) |
| Fixture | Run `SHOW max_connections` through the Stack wrapper and sample `pg_stat_activity` during the bounded C12 suite. Parse the wrapper endpoint in the Server fixture. | The endpoint passes the local policy. The measured peak fits the available slots and no test reports `53300`. **UNREVIEWED FIX** (logic-r4 2, architect-r3 3) |
| Dependency | Inspect item 1’s first driver import and module diff. Run `go mod tidy -diff` and `govulncheck ./...`. | The builder added `github.com/jackc/pgx/v5 v5.13.0` in the same change. The module is tidy and the scan reports no known vulnerability in the used paths. **UNREVIEWED FIX** (2026-11-26 epic ruling) |
| Feed ordering | Run item 4’s marked mutation and inspect its two additional mutant records. On a disposable dirty checkout invoke the Server hook directly. | Early visibility is the only lock-removal failure accepted. The records contain the sequence and lock-release failures. The dirty tree is refused. (architect-r3 1, architect-r3 5, logic-r3 2, logic-r3 8) |
| Lock cleanup | Repeat abort, panic and commit failure with the session-lock mutants. When sprint 2 starts, repeat while its instance lock is held. | Feed and receipt locks end with their transactions. Their lock tags use `objsubid=1` and `objsubid=2` respectively. The intentional sprint 2 instance lock is excluded. (architect-r3 2) |
| Writer isolation | Hold the feed lock past the wait deadline. Queue eight writers. Also run a non-feed write and a read. Repeat with the deferred-foreign-key race and 100 concurrent requests recording 50 references each. | A read uses reserved capacity and returns within one second. A separately admitted non-feed write does not wait on the feed lock. Waiters time out consistently. Deferred checking happens before the feed lock, and the batch traces contain one flush batch per successful write. |
| Feed bound | Attempt one write with 3,000 references in front of three open streams. | The 257th reference fails and rolls back the write. Streams receive no changes from that write. |
| Notification | In 1,000 commits observe notifications and then read each commit through a new snapshot. Disconnect the listener while writes continue, then reconnect during further writes. | Each notified row is already visible. Polling and registration wakes deliver every retained position once and in order. With notifications refused, the bounded polling path still makes progress. |
| Migration | On a fresh database repeat two startup calls 50 times. Move ledger creation before the migration lock for a test run. | The baseline starts both callers and records `store=1` once. The mutation fails the pre-ledger lock observation. |
| Receipts | On the merged Store, delay the first commit past the age cutoff and replay after observation. Also race `WriteOnce` with `Prune` twice, gating first the replay and then the deletion. Repeat a lost-response retry after restart with a zero-byte reply and an older encoder. | A retained replay returns the original bytes with one committed change. A retry after committed expiry may run anew. (architect-r2 4) |
| Retention clock | Delay a transaction past the cutoff and sweep after it commits. Seed a future insertion timestamp and an expired row behind a retained prefix. Seed a retained receipt, then advance the fixture cutoff by 25 hours to model a forward database clock step and retry its key. | Newly visible rows are retained. A diagnostic timestamp cannot block expiry. The blocked prefix is reported. A forward step can delete several receipt batches at once and permit a second mutation. (threat-r2 6) |
| Lag and reconnects | Prune behind an in-flight row. Hold the snapshot slot during a reconnect burst and run an authorization read and a keyed write. Cancel or panic in a separate snapshot loader, then open another snapshot. | The freed slot admits the next snapshot. The follower ends a stream at 2,048 rows or loss of its cursor. Authorization and keyed writes keep working. **UNREVIEWED FIX** (round 4 supplementary check, architect-r3 6) |
| Earlier restore | In a disposable database, put the feed and records back to an earlier state while reads and an existing stream stay open. Write below its old cursor, then restart the server and reopen the stream. | The existing stream sends no rows below its cursor. The reopened stream snapshots the current records on the restored database. **UNREVIEWED FIX** (logic-r4 3) |
| Real Watch transport | Hold a sender open, trigger lag, and reopen through the real Machines handler. Open another watch and revoke its session, first during idle and then during a blocked send. | The Machines adapter emits `aborted` for lag when writable and stops the blocked transport otherwise. The new stream sends the full snapshot, with a present zero position if appropriate. Revocation ends the stream without a later unauthorized record. |
| Error details | Format the encoding, connection and wrapped SQLSTATE errors from the built Store. | Only the fixed sentence and allowed metadata appear. (threat-r2 1) |
| Commit outcomes | Repeat the held commit gate with `SET LOCAL lock_timeout = '3s'`. Separately set `statement_timeout` to three seconds, wait past that point at the gate and release it within the lock deadline. Also observe client cancellation and a lost reply after commit. | The lock timeout gives `55P03`, `OutcomeRolledBack` and `Retryable`. The statement-timeout case waits for gate release and commits. Client cancellation without a definitive result and a lost reply yield unknown. A keyed retry commits once or replays. An unkeyed write is never automatically repeated after ambiguity. **UNREVIEWED FIX** (logic-r4 1, threat-r2 2) |
| Receipt declarations | Audit the sign-in, session and join-token adapters for keyed `WriteOnce` and `ReceiptSafe`. | Credential-bearing calls never set `ReceiptSafe`. Any special credential replay follows the owner’s recorded choice. (threat-r2 3) |

</details>
