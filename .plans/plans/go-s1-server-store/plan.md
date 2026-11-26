---
topic: go-s1-server-store
status: attack
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

Each Watch starts with a snapshot. One reader per server listens for database notifications and polls every two seconds. A stream ends when it falls behind or loses authorization.

See [the decision log](decisions.md).

## Workstreams

### Parts can store state and upgrade their own tables

Each part brings its own numbered database changes. The tests start the store twice against one empty database and check that each change is applied once. Parallel tests can separately create tables with the same names because they each get their own database.

### Writes arrive in commit order and safe retries reuse replies

The ordering test proves the reader cannot pass an uncommitted lower position. The retry test shows two matching calls create one change and return the same reply.

### Watch starts with a snapshot and follows live changes

A gated test proves the list and cursor share one database view. A held sender test proves a slow stream can be cut off while another stream keeps receiving changes.

## Rollout order

1. In sprint 0, Contract delivers the root Go module and shared Watch shape. In sprint 1, the orchestrator runs `go get github.com/jackc/pgx/v5@v5.13.0` immediately before item 1, the first import. This is the only direct module dependency for this plan. The orchestrator lands `go.mod` and `go.sum` in the commit with that import and checks `go mod tidy -diff`. From sprint 1 onward dependency file edits belong to the orchestrator. (architect-r2 2, architect-r1 1, critic-r1 5)
2. Run startup and the database tests through the Stack team’s `deploy/test/with-postgres.sh -- <command>`. Stack supplies `LGTM_TEST_POSTGRES` for the disposable `lgtm_store_test` maintenance database with `CREATEDB`. Server’s `storetest.NewDatabase` consumes it. (architect-r2 3, threat-r2 8)
3. Run `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12` on the committed checkout. Server item 4 supplies the executable `src/server/internal/store/check-feed-lock.sh`. The Stack vault plan owns `deploy/test/mutate-feed-lock.sh`, which executes the Server path from the repository root with no arguments, inherits `LGTM_TEST_POSTGRES` and passes through output and exit status. The existing check driver consumes the wrapper. Compare the Store handoffs below before the next Server sprint. (architect-r2 1, architect-r2 8)
4. Later Server sprints wire startup, session authorization and transport. The owner settles keyed credential behavior before those calls use any receipt path.


| Server Store or Watch writes | Later work that consumes it |
|---|---|
| `Open`, `ConnectionPolicy.AllowPrivateStackPostgres`, `Migrate`, `StartFollower` and `RunPruner`. | Sprint 2 bootstrap supplies configuration and lifecycle. Stack supplies the private database endpoint and the test fixture. |
| `Write`, `Tx.Record`, `MaxWriteChanges`, `Request`, `RequestKeyHeader`, `WriteOnce` and the receipt errors. | Sprint 2 and 3 services own the mutation, authorized caller identity and receipt safety declaration. Keyed adapters supply and retain versioned fingerprints. |
| `NewDomainError`, `Error.Outcome` and `Error.Retryable`. | Sprint 2 and 3 adapters declare fixed domain sentinels and map write errors, including unknown outcomes. |
| `Source.Authorize`, `Message`, `ErrWatchUnauthorized`, `ErrAuthorizationUnavailable`, `ErrFeedUnavailable` and `ErrWatchLagged`. | Sprint 2 sign-in supplies session lookup. Sprint 3 Machines binds it in the Watch source and maps the envelope and stream errors to the shared contract. |

Server supplies these names. **UNREVIEWED MECHANISM** (architect-r2 8). Use this local list for the comparison because the epic and consuming plans are outside this assignment.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

The Server team builds the Store and Watch folders in Go 1.26 or newer. Structured Query Language (SQL) examples use the configured PostgreSQL schema. All connections use the same database endpoint in this sprint.

In phase 4 of the build loop, freeze tests after they fail on the named assertion, using stubs where needed. A race test proves gate arrival before observing the behavior. A timeout alone is a fixture failure. The lead tests the exact commit and runs the named mutations on a disposable copy before merging.

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

Create `src/server/internal/store/pool.go`, `src/server/internal/store/pool_test.go` and `src/server/internal/store/storetest/postgres.go`. This item follows the sprint 0 module and the dependency pin in rollout step 1. Item 2 can run beside it. It implements D2, D3, D4, R3, R7, R10 and C12. (architect-r2 2)

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

`Open` copies and pings the configuration in five seconds. On failure return “The database connection failed, check the PostgreSQL connection settings.” The main pool needs at least eight connections and admits at most `MaxConns - 3` domain writers before acquisition. Its defaults are `idle_in_transaction_session_timeout=30s` and `statement_timeout=15s`. A separate single-connection pool serves the follower’s startup and page reads. The listener has its own connection. All use the same validated configuration, with 30-second transmission control protocol (TCP) keepalives. `Close` closes both pools after follower shutdown. Store writes the configuration rules, sprint 2 startup configures the main pool and item 7 reads the private page pool. **UNREVIEWED MECHANISM** (architect-r2 5). The independent page pool prevents page starvation without a second admission limit on domain reads. (logic-r1 2, threat-r1 3, critic-r1 1, critic-r1 8)

Before dialing, validate every endpoint and fallback for Transport Layer Security (TLS) with a verified certificate chain and hostname. Plaintext exceptions are Unix socket paths, literal loopback addresses and the exact Compose service `postgres` with explicit `AllowPrivateStackPostgres`. For the Compose exception require one host and no fallback to another host. In the final dial path require the resolved address to pass `netip.Addr.IsPrivate` and dial that same literal address. Store writes `ConnectionPolicy`, sprint 2 bootstrap sets it and Stack owns the private network binding. (threat-r1 9, threat-r2 5)

`Read` uses a read-only `REPEATABLE READ` transaction. The private `func (s *Store) withWrite(ctx context.Context, write func(context.Context, pgx.Tx) error) error` uses `READ COMMITTED`. The 30-second write budget covers admission through commit. Each statement has a 15-second limit and lock waits have a ten-second limit. Pruning uses those same limits per batch without domain-writer admission. Respect shorter caller deadlines. On error, cancellation or panic, roll back with a fresh one-second cleanup budget and discard a connection if cleanup fails. Callbacks close rows and leave transaction control to Store. Store never reruns a callback. (architect-r1 7, critic-r1 1, logic-r1 2, logic-r2 4, architect-r2 7)

Sanitize errors at the Store boundary, including row iteration, callbacks and commit. Before commit, normalize cancellation to the context error. Pass through direct Store and `NewDomainError` sentinels. Domain callers declare those sentinels with fixed public messages. Map every other error to `*Error` with a fixed message and no cause. Retain a recognized PostgreSQL error code (SQLSTATE) and a declared constraint identifier when present. Store writes this API and later adapters declare and read the safe sentinels. **UNREVIEWED MECHANISM** (threat-r2 1, threat-r1 8). An allowlist covers driver, connection and wrapped callback errors without enumerating their concrete types.

An error before commit, or one with an affirmative PostgreSQL rollback response, yields `OutcomeRolledBack`. A failure after a commit may have been sent, with no confirmed outcome, yields `OutcomeUnknown`, including a lost connection or timeout. `Retryable` is true only for a rolled-back `55P03`, `40P01` or `40001`. Service adapters read `Outcome` before `Retryable`. They map the latter to `aborted` with “The database was busy, retry the request.” For an unknown outcome they use `unavailable` with “The database did not confirm whether the write committed.” They may retry with the same key and must never automatically repeat an unkeyed mutation. Store writes the outcomes. The sweep, follower and services read sanitized errors. (threat-r2 2, architect-r1 7)

`func NewDatabase(t testing.TB, schema string) *pgxpool.Config` skips with “Set LGTM_TEST_POSTGRES to run PostgreSQL tests.” when the setting is absent. A configured malformed value, a database name other than `lgtm_store_test`, or a role without `CREATEDB` fails with a fixed diagnostic before any database creation where applicable. Never print the connection string. The fixture creates a quoted, randomly named database with `TEMPLATE template0`, then sets `search_path` to the validated package schema. Only an actual run with the disposable database counts for Server acceptance or C12. The C12 driver already rejects an absent setting. Stack writes the maintenance database name and connection setting. The Store fixture reads them. (architect-r2 3, threat-r2 8)

Register cleanup immediately. Close application connections before `DROP DATABASE <quoted-name> WITH (FORCE)` under a fresh ten-second context. The Stack wrapper discards the disposable instance if the test process dies before cleanup. (critic-r1 8)

A fixture wait defaults to 15 seconds unless an item gives another bound. Assert arrival at a gate before evaluating a race. Silence checks last 250 milliseconds after their registration or synchronization barrier. Shared database gates belong to item 4. (critic-r1 3, critic-r1 10)

Tests first in `pool_test.go`.

- `TestDatabasesAreIsolated`. Run two parallel subtests with the same schema name. Before creating an identical table, assert it is absent from both databases. Insert a distinct value in each and read exactly that value. Assert that the database names differ. In the parent, after the parallel group completes, query the maintenance database for both names and assert that both are gone.
- `TestTransactionsReleaseConnection`. Use an eight-connection pool. Record pool acquisition counts before and after each case. Write a row, cancel the context in the callback and return its error. Open an independent transaction through the pool and assert that it can read within one second and that the row is absent. Repeat with a callback panic caught by the test. In a read transaction attempt an insert and assert the PostgreSQL read-only error. Check that opening a refused connection returns an error without returning a store.

- `TestWritersCannotStarveReaders`. Hold an advisory gate on a separate connection. Start eight writes that all wait on it. With an eight-connection pool, require at most five acquired writer connections and a `Read` within one second. Require all five first-wave waiters to return retryable `55P03` before releasing the gate. The other three must then succeed within a 25-second test budget. Cancel a waiter in a separate case and check pool release. Removing admission must fail. (threat-r1 3, logic-r1 2, logic-r2 6)
- `TestOpenRefusesPlainRemote`. Cover Unix socket, literal loopback, the exact Compose service resolving to a private or public address, a loopback alias and verified remote TLS. Refuse a plaintext remote fallback even when a loopback entry comes first. A dial spy proves rejected addresses are never dialed. (threat-r1 9, threat-r2 5)
- `TestDatabaseErrorsHideValues`. Cause real check and unique violations, a driver-side encoding failure, a closed connection mid-transaction and a callback wrapping a SQLSTATE error. Check `%v`, `%+v` and the whole error chain for absence of fixture values and the endpoint. Assert recognized codes and approved constraint identifiers survive, and a direct safe domain sentinel keeps its identity. (threat-r1 8, threat-r2 1)

- `TestNewDatabaseRejectsWrongMaintenanceName`. Run the fixture with a configured maintenance name other than `lgtm_store_test`. Assert its refusal before connection setup. (threat-r2 8)

This item is risky because it sets the isolation level and controls transaction lifetime. Changing `Read` to a writable transaction must fail the read-only assertion. Item 1 gates on the named pool and policy tests. Item 9 owns the read-isolation mutation.

**2. Version 7 identifiers**

Create `src/server/internal/store/ids.go` and `src/server/internal/store/ids_test.go`. This item needs Go from the root module. It can run beside items 1 and 3. It implements D2, D3 and C12.

The public entry point is `func NewID() (string, error)`. Use Go's secure random reader for the 74 random bits and a clock for the 48-bit big-endian Unix timestamp in milliseconds. Set the version nibble to 7 and the two high variant bits to binary 10. Return the canonical lowercase hexadecimal form with hyphens. The private test seam is `func uuidV7(at time.Time, random io.Reader) (string, error)`. Keep no counter or node address. The time component sorts identifiers from distinct milliseconds in a test with an advancing clock.

Tests first in `ids_test.go`.

- `TestNewIDVersionAndTime`. Decode two deterministic identifiers from times one millisecond apart and a supplied byte vector. Assert the exact timestamp, version, variant and remaining bits. Assert that lexical order follows those two timestamps. Generate 4,096 real identifiers across concurrent goroutines and assert that none repeat.
- `TestUUIDv7EntropyFailure`. Pass a reader with fewer than the required bytes. Assert that the generator returns an error and an empty identifier.

This item is risky because it sets the sortable time component. Encoding the timestamp in little-endian order must fail `TestNewIDVersionAndTime`. Both tests are the acceptance observations.

**3. Numbered database changes and store tables**

Create `src/server/internal/store/migrate.go`, `src/server/internal/store/migrate_test.go`, `src/server/internal/store/locks.go`, `src/server/internal/store/migrations/0001_store.sql`, `src/server/internal/store/storetest/migrated/store.go` and `src/server/internal/store/storetest/migrated/store_test.go`. This item needs item 1. It can run beside item 2. It implements D3, D4, D5, R1, R3, R7, R10 and C12.

```go
type Migrations struct {
    Part  string
    Files fs.FS
}

func (s *Store) Migrate(ctx context.Context, parts ...Migrations) error
```

The startup caller passes each part explicitly. A part embeds `migrations/*.sql` and exposes a manifest whose `Files` starts at `fs.Sub(embedded, "migrations")`. The store adds its own embedded `store` manifest. Reserve that name. Validate the complete manifest before changing the database. Part names match `[a-z][a-z0-9_]*`. Files use `NNNN_description.sql` with four digits and a lowercase description, with contiguous numbers beginning at 1 for each part. Reject duplicate part names, invalid filenames, duplicate versions, gaps and a recorded version newer than the supplied files. Refuse startup when the database already records a part absent from the manifest. Rollback to an older binary therefore requires a compatible manifest and is an operator decision. The list of parts and same-part prefix history remain explicit.

Run startup in one read-write transaction with a five-minute budget including acquisition and commit, subject to a shorter caller deadline. Set local `statement_timeout` to five minutes and `lock_timeout` to zero. Acquire `pg_advisory_xact_lock($1::bigint)` with `migrationLockID` before touching the ledger. Apply the store part first, then other parts in lexical order, and apply numbered files in order. Execute each trusted embedded SQL file whole in simple query mode. Update the ledger in the same transaction. Append a new migration for a correction. A failure rolls back and startup refuses to serve. The sprint 2 bootstrap calls `Migrate` with a migration budget separate from other startup prerequisites. (logic-r2 4, architect-r2 7, architect-r2 10)

The Store team writes the reserved lock constants in `locks.go`. `feedLockID int64 = 841941100001` is used by the feed writer. `migrationLockID int64 = 841941100002` is used by `Migrate`. `receiptLockNamespace int32 = 1279743053` is used by `WriteOnce`. Parts may not allocate keys in these spaces. The first two locks use the single-integer key space. The receipt lock uses two integers. (architect-r1 9)

The Server team also writes `func migrated.NewStore(t testing.TB, schema string, parts ...store.Migrations) *store.Store` in the dedicated `storetest/migrated` package. It provisions a database, opens it with the default local policy, applies migrations and registers Store close before database drop. Later part tests call this fixture. Store's own in-package tests import the database fixture without importing back into Store. **UNREVIEWED MECHANISM** (architect-r1 9). A subpackage avoids a Go import cycle. Freeze `TestNewMigratedStore` in the fixture package. Assert a registered part table exists on return and the database disappears after subtest cleanup.

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

The Server migration owns the store tables and feed guards. The insert trigger requires transaction-local `lgtm.feed_flush=on` or raises `23514` with `feed_requires_store_flush`. A row trigger requires `lgtm.feed_prune=on` for updates and deletes and names `feed_requires_store_prune`. The statement trigger always refuses `TRUNCATE` with `feed_refuses_truncate`. Both also use `23514`. Item 4 sets the flush flag. Item 6 sets the prune flag in feed stamp and delete transactions. Fixturing may set those flags to seed history. The guards catch accidental changes by trusted callers. Store alone changes the watermark. The feed stores opaque references and a blank scope denotes server-wide records. The receipt writer and pruner share the composite key. (logic2-r1 1, logic2-r1 3, threat-r1 5, threat-r1 6, critic-r1 7, architect-r1 3, threat-r2 7)

Tests first in `migrate_test.go`.

- `TestMigrateColdConcurrentStarts`. Hold the migration advisory lock on an observer connection while two calls start on a database with no ledger. Wait for both advisory-lock waiters in `pg_locks`. Assert the waiting backends have started no ledger creation by observing their active statement and lock records. An uncommitted ledger can be invisible to another transaction, so invisibility alone is insufficient. Release the lock. Assert that both succeed and that the ledger contains `store=1` once. (critic-r1 2, logic-r1 10)
- `TestMigrateConcurrentStarts`. Bootstrap the store ledger once, then register two new fixture parts with version 1 and a second change in one part. Launch two startup calls on separate connections. Have the first hold a gate inside its first migration. Assert that the second waits for the first on the startup advisory lock before the gate is released. Use `pg_locks` and `pg_blocking_pids` to distinguish the startup lock from the test gate and a catalog lock. Release the gate. Both calls succeed. Observe the store schema through the catalog. Assert the ledger values `store=1`, the first part at 2 and the second at 1. Assert that non-idempotent fixture inserts occur once and in version order. A third call leaves the rows and versions unchanged.
- `TestMigrateUsesStartupBudget`. After observing a starter waiting on the migration lock, hold it for twelve seconds. Release it and run a migration with a 20-second statement. Both startups must succeed. The fixture completion budget is 60 seconds. Restoring the normal statement timeout or the write lock timeout must fail. (logic-r2 4, architect-r2 7)
- `TestMigrateFailureRollsBack`. Start with version 1 of a fixture part, then attempt version 2 that creates a new table followed by a failing statement. Assert an error, absence of the new table, and ledger version 1. Replace only the unmerged fixture file and assert a rerun records version 2.
- `TestMigrateValidatesManifest`. Submit duplicate names, duplicate numbers, missing numbers and invalid paths as table-driven cases. Assert a refusal before any ledger or table appears. After a valid migration, submit an older or incomplete manifest and assert a refusal with the previous ledger intact.

This item is risky because it controls schema order. Removing the startup advisory lock must fail `TestMigrateConcurrentStarts`. Moving ledger creation before the lock must fail `TestMigrateColdConcurrentStarts`. The named tests are the acceptance observations.

**Design artifact.** The database layer in the approved design supplies the brief. This workstream has no screen.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Parallel tests or a canceled transaction leave pooled connections or test databases behind. | Medium | Use a fresh cleanup context and prove teardown against the maintenance database in item 1. |
| Two startups apply the same migration or advance its ledger before its statements are durable. | High | Use one transaction and the startup advisory lock. Prove concurrent and failed starts in item 3. |

**Verify.** Run `go test -race -count=1 ./src/server/internal/store/...` with `LGTM_TEST_POSTGRES` pointing to a disposable PostgreSQL 16 or newer. Use the Stack wrapper or an equivalent disposable `lgtm_store_test` database with `CREATEDB`. No device check is needed.

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

Create `src/server/internal/store/feed.go`, `src/server/internal/store/feed_test.go`, `src/server/internal/store/storetest/gate.go` and `src/server/internal/store/check-feed-lock.sh`. This item owns `Tx` and needs items 1 and 3. Items 5, 7, 8 and 9 consume the test gates without editing them. (critic-r1 10) It implements D3, D4, D5, R4, R7, R10 and C12. It can run beside item 2 if that item remains open, and it precedes items 5 and 7.

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

**Test seams.** Server item 4 writes `func WaitForBlock(t testing.TB, observer *pgx.Conn, waiter, blocker int32, key int64)` in `storetest/gate.go`. It polls `pg_locks` and `pg_blocking_pids` every 10 milliseconds for at most 15 seconds. Use `objsubid = 1` and reconstruct the bigint key as `(classid::bigint << 32) | objid::bigint`. The SQL helper `func HoldAdvisoryGate(t testing.TB, observer *pgx.Conn, key int64) func()` takes a session lock and returns a cleanup-registered idempotent release function. The test observer reads writer identities using `SELECT pg_backend_pid()` emitted from each transaction callback. Use fixture `records(id text PRIMARY KEY, value text NOT NULL)`, topic `records`, scope `''` and disjoint keys `a` and `b`.

The feed tests install a deferred constraint trigger `feed_test_gate` on `feed`. Server item 4 reserves `feedTestGateID = 841941100901` in `feed_test.go`. The trigger and observer read it. **UNREVIEWED MECHANISM** (architect-r2 10). Keep the fixture key in the test file so it stays out of production. Its body runs `PERFORM pg_advisory_xact_lock(<feedTestGateID>::bigint)` only for `NEW.key = 'a'` and then returns `NEW`. In failure tests its body can instead raise for that key. Store item 4 defines a private per-Store fixture seam with `afterConstraints func(context.Context, pgx.Tx) error` and `beforeCommit func(context.Context, pgx.Tx) error` callbacks, nil in production. The first hook can execute `SET CONSTRAINTS feed_test_gate DEFERRED` after the production constraint check. The second runs after batch close and can cancel or panic. Use the standard fixture deadlines for waits and 250 milliseconds for notification silence. (critic-r1 3, logic2-r1 4)

Tests first in `feed_test.go`.

- `TestFeedPositionsFollowCommitOrder`. Hold the test gate and wait for A to block at commit. Assert that A reached the gate after its insert. Start B on a separate backend. With the lock, B waits for A on `feedLockID` and neither row is visible. Assert `last_value` of the identity sequence from `pg_sequences` equals A's allocated value seen at the gate. Resolve the sequence name with `pg_get_serial_sequence`. With the lock removed, B commits and becomes visible first. Release A before asserting failure. Report that case with the exact text `writer B became visible before writer A`. In the unmodified run, require A then B in visible order and an advancing reader to see both. Move only the lock behind all inserts as the second mutation. Keep the guard flag before inserts. It must fail the sequence assertion. (logic-r1 1)
- `TestFeedLockStartsAtFlush`. Pause A's callback after its state write and `Record`. Require B to commit before A resumes. Assert B then A in the feed.
- `TestFeedRollbackAndNotification`. Register a raw listener. Force a commit error with the test-only deferred trigger. Assert no state, feed row or notification. Commit a second write with 256 marked references. Assert all positions are increasing in recorded order. Assert one flush batch through the driver's batch tracing seam and one committed notification. (critic-r1 1, threat-r1 4)
- `TestFeedRejectsInsertOutsideFlush`. In separate callbacks try direct insert, update, delete and truncate. Assert `23514` and the migration’s named constraint for each. Seed real rows for update and delete. Require those rows to remain and a normal `Record` to succeed. Removing any guard must fail. (architect-r1 3, threat-r1 5, threat-r2 7)
- `TestRecordValidationAndLimit`. Exercise an empty topic, empty key, zero byte, invalid change and the 257th reference. Assert the failure is returned at `Record` and keeps state and feed rows out even if the callback returns nil. (critic-r1 6, threat-r1 4)
- `TestFeedLockDoesNotBlockUnrecordedWrites`. Hold the feed lock on a separate backend. Complete a no-change write while one buffered writer waits. In separate cases cancel the waiter and let another reach the lock timeout. Assert the connection is returned and that both error causes are distinguishable. (critic-r1 1, architect-r1 7)
- `TestFeedLockEndsWithTransaction`. Cancel after batch close, panic there and make the deferred trigger fail at commit in three subtests. Keep the writer backend alive where possible. Inspect `pg_locks` and then use a confirmed different backend to write. Require it to finish within one second without a leftover lock. Replacing the transaction lock with a session lock must fail. Exercise cleanup after an aborted transaction even if the mutant attempts to unlock through that transaction. (logic-r1 3)
- `TestFeedChecksDeferredDomainConstraintsBeforeLock`. Give B a `FOR UPDATE` lock on a parent row. A inserts a child with a deferred foreign key to that row and attempts a feed write. Assert A waits on B's parent transaction before acquiring the feed lock. Let B flush and commit. Both writes must finish. Move the constraint check behind the feed lock and require the test to fail. In a separate domain-only race, take two row locks in opposite order. Assert the victim returns a retryable deadlock error and `OutcomeRolledBack`. Item 5 owns the keyed retry test. (architect-r2 4) (logic2-r1 4, architect-r1 7, threat-r1 10)

- `TestFeedCommitErrorOutcome`. On separate fixture writes, make the deferred trigger reject a commit and terminate a backend observed waiting at that trigger during commit. The first error is `OutcomeRolledBack`. The connection loss is `OutcomeUnknown` and not retryable. (threat-r2 2)

This item is risky because it establishes feed ordering. From the repository root run `src/server/internal/store/check-feed-lock.sh` with no arguments and the configured test database. The script archives `HEAD` into a temporary directory and records the tested commit on standard error. In that copy, run `go test -count=1 -v ./src/server/internal/store -run '^TestFeedPositionsFollowCommitOrder$'` and require the named test’s pass line. In `feed.go`, require exactly one marker and delete its entire line. Require `go build ./...` to succeed. Rerun the same test and require a nonzero exit with the named test’s failure and exact assertion `writer B became visible before writer A`. Only then print `PASS C12 feed-lock mutation: writer B became visible before writer A` to standard output and exit zero. Otherwise fail and report the reason on standard error. Always remove the copy. Stack's wrapper calls this Server script. (architect-r2 1, logic-r2 2, architect-r2 6)

**UNREVIEWED MECHANISM** (architect-r2 6). Keep the hook to the one C12 mutation. On disposable copies the item 4 lead also moves the lock behind inserts and substitutes a session lock. The sequence and lock-release assertions above must fail respectively.

**5. Request receipts**

Create `src/server/internal/store/receipts.go` and `src/server/internal/store/receipts_test.go`. This item needs item 4 and can run beside item 7. It implements D3, D5, R4, R7, R9, R10 and C12.

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

- `TestWriteOnceConcurrentRetry`. Hold A's callback after a state insert. B uses the same authenticated principal, method, key and fingerprint. Require the observer to show B waiting on A with `objsubid = 2` and `classid = receiptLockNamespace`. (architect-r2 10) Release A. Assert one callback, one receipt, one state change and one feed row. Require identical replies across both callers and a reopened store. (critic-r1 7)
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
    OldestUnobservedReceiptAt time.Time
}

func (s *Store) Prune(ctx context.Context) (PruneResult, error)
func (s *Store) RunPruner(ctx context.Context, report func(error))
```

`RunPruner` sweeps at startup and hourly. It reports sanitized failures and retries at the next tick. Use private `runPruner` with a fixture tick channel and `pruneBefore(ctx, cutoff)` for expiry tests. `Prune` samples `clock_timestamp() - interval '24 hours'` once and reports only committed counts. In `PruneResult.OldestUnobservedReceiptAt`, return the oldest unobserved receipt’s `written_at` seen before stamping, or zero if absent. Store writes the result. Report a fixed diagnostic to startup when that time is over two intervals old, even if a later batch fails. (threat-r2 6, critic-r1 6, critic-r1 11)

Stamp only visible, unmarked rows after acquiring their row locks. Sample PostgreSQL time after acquiring the locks. A row is eligible once `retention_started_at < cutoff`. A late commit therefore gets a full day from observation if the database clock advances normally. `written_at` is for diagnostics. An administrative forward jump can expire more than one day of receipts in one run. A backward adjustment can extend retention. (threat-r2 6, logic2-r1 1, logic-r1 8)

Run separate stamp and delete phases for feed and receipts. Repeat each phase in committed batches of up to 512 until its batch is short. For feed batches, lock the `feed_retention` singleton `FOR UPDATE` and set the local prune flag. Stamp ascending positions. Delete only the expired prefix of the earliest 512 positions and advance `pruned_through` in that transaction. A retained position ends deletion for the sweep. Preserve the sequence. Report an observed row older than twice the retention blocked by that prefix, without row values. (logic-r2 1, threat-r2 7, critic-r1 11, logic-r1 8)

For receipts, select eligible rows in index order and lock `FOR UPDATE`. A replay that holds the row first completes with its saved reply. If deletion commits first, a later retry can run anew. The batch uses item 1’s maintenance limits. Item 6 owns no calls to `WriteOnce` or item 4’s test gates. Use raw SQL for its row and advisory fixtures. The merged receipt check covers both commit orders. **UNREVIEWED MECHANISM** (architect-r2 4). This keeps pruning independently mergeable. (critic-r1 7)

Tests first in `prune_test.go`.

- `TestPruneCutoffAndWatermark`. Seed at least 1,300 expired feed rows in addition to multiple unstamped feed pages and old receipts. A single sweep stamps every visible unmarked row, deletes all expired feed rows and receipts, retains rows at or after the cutoff and preserves part-owned state. Assert committed counts and the last deleted position. Run again for zero deletions. After pruning the whole feed in a separate case, the next raw fixture insert must get a higher position. A one-page stamp or delete mutant must fail. (logic-r2 1)
- `TestPrunePreservesFeedPrefix`. Seed old and fresh observed times out of position order. Assert cleanup stops before the fresh row and reports a blocked stale prefix when another row is old enough. Use a future `written_at` on a row with an old observation time and prove that the diagnostic time does not block its deletion. (logic-r1 8, critic-r1 11)
- `TestPruneStartsAgeAfterCommit`. Set backdated insertion times for a receipt and feed row in an uncommitted fixture transaction. Run cleanup before and after releasing commit. Assert observation is null until visible, then newer than the cutoff for both rows. Run a cutoff sweep and confirm the receipt survives. (architect-r2 4) (logic2-r1 1)
- `TestPruneWaitsForReplayOfSameKey`. Hold an eligible receipt row `FOR UPDATE` from the raw replay fixture. Start pruning. Observe that receipt cleanup waits but committed feed cleanup can progress. The holder reads the original reply and commits. Require the sweep to delete the expired row. Seed a receipt exactly at the cutoff and verify it survives until the cutoff advances. The merged check uses the public replay path in both lock orders. (critic-r1 7)
- `TestRunPrunerLifecycle`. Use a controlled tick channel. Require a startup sweep, an explicit tick, a safe error from a separately gated bad sweep, recovery and cancellation. Seed an unobserved receipt with a diagnostic age over two hours. Require the oldest observation field and the fixed delay diagnostic. Assert the production interval is one hour. (critic-r1 6, threat-r2 6)

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
| 8 | A subscriber is cut off at the lag limit, on loss of history or while the feed is unavailable. A draining subscriber can continue during another subscriber’s lag. | `TestFollowerLagAt2048`, `TestFollowerPrunedCursor`, `TestFollowerDetectsHeadRegression` and `TestFollowerEndsStaleStreams`. |
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

type feedPage struct {
    Head          int64
    PrunedThrough int64
    Rows          []FeedRow
}

func (s *Store) readFeedPage(ctx context.Context, after int64) (feedPage, error)
```

`Snapshot` reads the head before running `load` in the same `Read` transaction. The head is the greater of the highest present position and `pruned_through`. A fresh feed starts at zero. `readHead(ctx context.Context, tx *ReadTx) (int64, error)` supplies this calculation. (critic-r1 6, logic2-r1 5)

`readFeedPage` uses item 1’s dedicated page pool. It returns the head, the pruning floor and up to 512 rows from one read-only transaction. A head below the supplied cursor detects replaced history. In that case return the new head without rows. Otherwise query `WHERE position > $1 ORDER BY position ASC LIMIT 512` using the greater of the cursor and the pruning floor. Close the result set before returning. Advance only after the page is accepted. If a non-reset page starts at or below the last applied position, report the invalid page and preserve the cursor. After a full page drain immediately. A failed page preserves the last applied position. (architect-r1 6, critic-r1 11, logic-r1 7, logic-r1 9)

Define private `runFeedReader(ctx context.Context, s *Store, after int64, options feedReaderOptions, apply func(feedPage) error)`. One goroutine owns the cursor. It selects over a two-second poll and a wake channel of size one. Another goroutine only manages the dedicated `LISTEN lgtm_feed_changed` connection and requests wakes. The listener wakes the reader on registration and on notifications. Dial and registration together have a three-second bound. Reconnect at most once per poll interval. A standby or an unavailable listener leaves polling active. Each page attempt uses a two-second context for acquisition and the transaction, including queries. Rollback has at most one additional second. A timeout leaves a later poll free to resume. These are production limits. (logic2-r1 2, logic-r1 9)

**Test seams.** The Server reader owns the private options below. Items 7 and 8 use them. The default has no callbacks, uses `time.Now` and a two-second ticker, and enables the listener. For a fixture, a nil tick channel uses the production ticker. `afterAcquire` runs in the reading transaction before the head query. It may wait on a real PostgreSQL test advisory lock. `onPage` is invoked after application completes, including an empty accepted page, before the next read. `onListen` fires after registration and receives the listener backend identifier. Test callbacks use buffered channels and do not block production shutdown. (critic-r1 3)

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
- `TestSnapshotRetainsPrunedHead`. Remove the feed with the fixture prune flag and update the durable floor in the same transaction. Assert that `Snapshot` returns the floor, even with no rows left. (critic-r1 6)

- `TestFollowerProgressesUnderReadSaturation`. Start the reader, then hold eight snapshots after their first query so all eight main pool connections stay occupied. Commit one normal feed row from a second Store connected to the same database. On a poll require the page to apply within its two-second budget while all snapshots are still held. Routing the page through the main pool must fail. (architect-r2 5)

Removing polling, letting two drains apply the same page, using an inclusive cursor, resetting the cursor on error or removing the production page deadline must fail the tests above. The item is risky because it can lose a committed entry.

**8. Bounded broadcast and subscriber lifetime**

Create `src/server/internal/store/follow.go` and `src/server/internal/store/follow_test.go`. This item needs item 7 and can run beside item 6 when ready. It implements D3, D5, R5, R7, R10 and C12.

```go
const MaxWatchLag = 2048

var ErrWatchLagged error
var ErrFeedUnavailable error
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

The server starts one follower after migrations. Private `startFollower(ctx context.Context, s *Store, report func(error), options feedReaderOptions) (*Follower, error)` reads the initial head through the dedicated page pool before starting the reader. (architect-r2 5) The exported entry point uses production options. The startup head is the initial unavailable-through marker. The follower retains at most the last 2,048 observed rows in a shared ring. It tracks the latest evicted position and the durable pruning floor. A process lock protects the ring and subscriptions. It does not surround application calls or database queries.

**Lag rule.** Lag is the count of actual observed committed feed rows with position greater than the subscriber's acknowledged cursor. Count rows across all topics and scopes. The in-flight row counts once until acknowledged. Cancel with `ErrWatchLagged` at lag greater than or equal to `MaxWatchLag`. Evaluate on `Start` and for each row as a page is applied. A cursor older than the unavailable, evicted or pruned marker is separately invalid because history is missing. Equality with the marker is usable. For example, one in-flight row plus 2,046 later rows leaves the stream open. One additional row ends it. (architect-r1 5, critic-r1 4, logic-r1 5)

`Subscribe` registers the stream before its snapshot as required by the design. `Start(after)` sets the cursor once. A cursor can be ahead of the follower when the snapshot sees a newer commit. Ignore rows through that cursor. A snapshot may take longer than the lag window under sustained writes. Report an immediate lag at `Start` once per consecutive incident until a stream starts successfully. The existing website reconnects with backoff. **UNREVIEWED MECHANISM** (critic-r1 11). Use the same cut-off for slow snapshot loading and for blocked delivery.

`Next` delivers one in-flight row. It returns `ErrSubscriptionState` before `Start` or while a row is still outstanding. `Ack` accepts only the outstanding position. Invalid `Start` and `Ack` calls leave state unchanged. A consumer acknowledges filtered rows as it skips them. A matching row is acknowledged only after the send returns. Cancel the subscription context on lag so ongoing work can stop before another `Next`. `Close` is idempotent on the follower and subscription. Follower close cancels and joins the reader and listener. Subscriber close removes the subscription. The Store team writes these methods and the state sentinel. The Watch helper reads them. (critic-r1 6, logic2-r1 7)

When a page reports a lower head than the read cursor, close subscribers with `ErrWatchLagged` and reinitialize cursor and ring at the new head. Report a sanitized history-reset event. Starting on the new head lets a new snapshot cover everything already committed there. A different history with an overlapping head cannot be detected by comparing positions. Operations stop the server before replacing the database in that case. Keep a stable endpoint throughout a follower lifetime. (architect-r1 6, critic-r1 11)

Each successful page, including an empty page, resets the ten-second stale clock. Evaluate it after a failed attempt. On expiry report once, cancel streams and refuse new ones with `ErrFeedUnavailable`. The next successful page clears the incident. Keep the last applied cursor unless history regressed. Store writes the sentinel, Server startup supplies the reporter, and Watch and service adapters read the error. (logic-r2 3, logic-r1 4, logic2-r1 2, critic-r1 8)

**Test seams.** Use the item 7 configuration to disable the listener, request a poll, advance fixture time and check when a page applied. Use `records(id text PRIMARY KEY, value text NOT NULL)` and source topic `records`. Use the item 4 gates and deadlines. For a deliberately over-limit single commit, tests may seed entries directly in an isolated database under the feed lock with the guard flag. The public writer remains capped. (critic-r1 3, critic-r1 4, threat-r1 4)

Tests first in `follow_test.go`.

- `TestFollowerLagAt2048`. Start a slow and a fast subscriber at the same head. Run boundary cases with no `Next`, with an unacknowledged first row and with a mix of filtered rows. Check lag 2,046 and 2,047 stay open. In the in-flight case these counts already include that row. At 2,048 require cancellation and `ErrWatchLagged`. In a separate case apply a full 512-row page that crosses the limit. The fast subscriber starts with enough acknowledged progress to receive the boundary and subsequent rows. (architect-r1 5, critic-r1 4, logic-r1 5)
- `TestFollowerBulkWriteCutsOffEveryone`. Seed one 2,047-row test commit and then a separate 2,048-row test commit in an independent database. Without acknowledgements, all subscribers stay open after the first case and are canceled after the second. Use the fixture injection because the production path rejects either oversized single write. (critic-r1 4, threat-r1 4)
- `TestFollowerPrunedCursor`. Activate one subscriber with a row still in flight. In one case, seed eviction before an older subscriber activates. In another, commit fixture deletion past the old cursor while polls are paused. On the next poll require cancellation even though the first subscriber never calls `Next` again. A new snapshot at the head can receive a later row.
- `TestFollowerIgnoresSequenceGaps`. Create a large rolled-back sequence gap in the isolated database. One committed row after it must not trigger lag. An ahead-of-reader subscriber ignores positions through its cursor and receives the next higher committed row.
- `TestFollowerSubscriptionLifecycle`. Reject `Next` before `Start`. After `Next` for position 41, reject a second `Next` and an incorrect `Ack`. Accept `Ack(41)` and deliver the next row. Reject a second `Start`. Repeated `Close` on each object is harmless and closes blocked waits. (logic2-r1 7, critic-r1 6)
- `TestFollowerDetectsHeadRegression`. Move an active follower to a high position. As the fixture administrator, temporarily disable the truncate guard, truncate the feed with identity reset, reset its pruning floor and restore the guard in the same transaction. (threat-r2 7) Poll. Assert old subscriptions end with `ErrWatchLagged` and the callback reports once. Start at the new head and receive a subsequent lower-numbered row. (architect-r1 6, critic-r1 11)
- `TestFollowerEndsStaleStreams`. For at least twelve fixture seconds, accept empty polls and assert the stream stays open without reports. Then hold a real read gate until page attempts time out. At ten seconds since the last accepted page, require existing and new streams to fail with `ErrFeedUnavailable` and report once. Release the lock and require recovery after a successful page. (logic-r2 3, logic-r1 4, logic2-r1 2, critic-r1 8)

This item is risky because cursor and buffer movement determine which changes get through. A numeric-distance count or a strict greater-than limit fails the gap or boundary tests. Disabling regression detection or the stale-stream cutoff fails its dedicated test.

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
| `.plans/epics/go-rewrite/epic.md`, The teams, and what each may edit | Server owns its implementation paths. Contract establishes the module and wire shape in sprint 0. The orchestrator handles later module changes. (architect-r2 2) |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, Rules that every later part follows | Parts own their tables and use the store's write path. The common Watch helper reads current state. Integration tests use PostgreSQL. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Backend covers server logic and SQL here. Item 3 is a startup runner that also provides the first schema on an empty database. There is no conversion of existing data. The `server/backend` scope is kept by the orchestrator. The table names the required review pass for boundary or ordering changes. All item-level acceptance tests remain mandatory. Work can run in parallel once the named prerequisites have merged. Each item owns a separate set of files.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | backend | server/backend | medium | The pool, isolation and fixture cross the PostgreSQL connection boundary. | Yes | Sprint 0 module and the orchestrator’s sprint 1 pin. |
| 2 | backend | server/backend | low | The format uses one documented byte layout and a small pure test seam. | Yes | Sprint 0 module. |
| 3 | backend | server/backend | medium | One runner applies a validated manifest in a serialized startup transaction. | Yes | 1. |
| 4 | backend | server/backend | medium | One ordered flush reuses a shared database gate. Separate gate setup and transaction assertions. | Yes | 1 and 3. |
| 5 | backend | server/backend | medium | The receipt check wraps the existing write path and adds one per-key lock. | Yes | 4. |
| 6 | backend | server/backend | medium | Bounded observation and deletion preserve the committed age and watermark. | Yes | 1 and 3. |
| 7 | backend | server/backend | medium | One reader reconciles an advisory signal with ordered reads and a periodic fallback. | Yes | 4. |
| 8 | backend | server/backend | medium | One bounded memory structure owns subscribers and their cancellation. | Yes | 7. |
| 9 | backend | server/backend | medium | One snapshot-to-stream helper reuses the subscriber and session callback. | Yes | 8 and sprint 0 Watch shape. |

Load for the first review and build is 12 OpenAI launches, 15 Anthropic launches and five xAI launches. It includes nine build and nine independent test-writing launches, the first plan review, two post-build slices and two focused checks. Each additional full plan round adds one OpenAI, four Anthropic and one xAI launch. The orchestrator checks the remaining allowances before dispatch. The review changes fit the nine items.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (blocker, major, minor) | Outcome |
|---|---|---|
| 1 | 0 blockers, 19 majors and 30 minors before deduplication. | See [the round 1 register](attack/round-1-findings.md). |
| 2 | 0 blockers, 10 majors and 15 minors before deduplication. | See [the round 2 register](attack/round-2-findings.md). |

</details>

<details><summary><b>Post-build checks from the review rounds</b></summary>

Run the database checks on PostgreSQL 16 and 17 when both are available. Keep the tested commit, commands and observations. These checks supplement the frozen tests. They need the built Store or the later service adapter.

| Area | Scenario | Expected result |
|---|---|---|
| Integration | Run `deploy/test/with-postgres.sh -- go build ./...`, `deploy/test/with-postgres.sh -- go test -count=1 ./src/server/...` and rollout step 3 on the merged sprint 1 commit. | Provisioning, real database tests and the mutation hook pass. The wrapper cleans up. Without the setting, ordinary root Go tests skip database cases and C12 refuses the run. (architect-r2 3) |
| Dependency | Run `go mod tidy -diff` and `govulncheck ./...`. Record the selected PostgreSQL driver version. | The root module is tidy. The driver matches the integration pin and the scan reports no known vulnerability in the used dependency path. |
| Feed ordering | Repeat the gated commit race 50 times with two writer backends reused by the pool. Trace the lock, the observer's sequence value and the first visible row. | The test gate stalls only A. The sequence stays at A while B waits. The lock-removal mutation shows B early. The lock-after-allocation mutation fails the sequence assertion. |
| Lock cleanup | Terminate a backend holding the feed lock. Repeat abort, panic and deferred commit failure with the session-lock mutations for feed and receipts. | No production advisory lock survives transaction end. A different backend can write. Reconstruct the bigint feed key from both `pg_locks` halves with `objsubid = 1`. The receipt namespace uses `objsubid = 2`. |
| Writer isolation | Hold the feed lock past the wait deadline. Queue eight writers. Also run a non-feed write and a read. Repeat with the deferred-foreign-key race and 100 concurrent requests recording 50 references each. | A read uses reserved capacity and returns within one second. A separately admitted non-feed write does not wait on the feed lock. Waiters time out consistently. Deferred checking happens before the feed lock, and the batch traces contain one flush batch per successful write. |
| Feed bound | Attempt one write with 3,000 references in front of three open streams. | The 257th reference fails and rolls back the write. Streams receive no changes from that write. |
| Notification | In 1,000 commits observe notifications and then read each commit through a new snapshot. Disconnect the listener while writes continue, then reconnect during further writes. | Each notified row is already visible. Polling and registration wakes deliver every retained position once and in order. With notifications refused, the bounded polling path still makes progress. |
| Migration | On a fresh database repeat two startup calls 50 times. Move ledger creation before the migration lock for a test run. | The baseline starts both callers and records `store=1` once. The mutation fails the pre-ledger lock observation. |
| Receipts | On the merged Store, delay the first commit past the age cutoff and replay after observation. Also race `WriteOnce` with `Prune` twice, gating first the replay and then the deletion. Repeat a lost-response retry after restart with a zero-byte reply and an older encoder. | A retained replay returns the original bytes with one committed change. A retry after committed expiry may run anew. (architect-r2 4) |
| Retention clock | Delay a transaction past the cutoff and sweep after it commits. Seed a future insertion timestamp and an expired row behind a retained prefix. Seed a retained receipt, then advance the fixture cutoff by 25 hours to model a forward database clock step and retry its key. | Newly visible rows are retained. A diagnostic timestamp cannot block expiry. The blocked prefix is reported. A forward step can delete several receipt batches at once and permit a second mutation. (threat-r2 6) |
| Restore and lag | Regress the database head while a subscriber is active. Run eight bounded writers during slow and fast subscriptions, and prune behind an in-flight row. Make snapshot loading fall behind the lag limit, then retry without the write load. | The history reset closes old streams. The slow stream ends at exactly 2,048 observed rows. The fast stream sees each row it acknowledges. A new stream starts with current state. |
| Real Watch transport | Hold a sender open, trigger lag, and reopen through the real Machines handler. Open another watch and revoke its session, first during idle and then during a blocked send. | The Machines adapter emits `aborted` for lag when writable and stops the blocked transport otherwise. The new stream sends the full snapshot, with a present zero position if appropriate. Revocation ends the stream without a later unauthorized record. |
| Error details | Format the encoding, connection and wrapped SQLSTATE errors from the built Store. | Only the fixed sentence and allowed metadata appear. (threat-r2 1) |
| Commit outcomes | Kill a connection after the backend reaches commit. Also drop its reply after a confirmed commit. Repeat for a keyed and unkeyed call through the later adapter. | Both lost replies yield an unknown outcome. Retrying the key commits once or replays the receipt. The adapter never automatically repeats the unkeyed write. (threat-r2 2) |
| Receipt declarations | Audit the sign-in, session and join-token adapters for keyed `WriteOnce` and `ReceiptSafe`. | Credential-bearing calls never set `ReceiptSafe`. Any special credential replay follows the owner’s recorded choice. (threat-r2 3) |

</details>
