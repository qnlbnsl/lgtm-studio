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

The server needs one place to write its records before sign-in or the Machines page starts using PostgreSQL. This sprint gives each server part a way to bring its tables up to date and change them inside a transaction. It keeps a feed of what changed so the website can start from a full list and then receive individual updates.

The main risk is a lost update when two writes commit out of order. PostgreSQL assigns each feed position while the store holds a lock that ends with the transaction. A test pauses the first writer after it has a position. The test must show that removing the lock lets the second writer become visible first.

The sprint ends with tests against real PostgreSQL. Starting the server and opening a stream from the Machines page belong to later sprints.

## Key decisions

A server part writes its own records and tells the store which records changed. The store writes the feed at the end of that same transaction. The feed holds record identifiers, and a reader gets the current record from the part that owns it.

A successful call can keep its reply for retries when the caller supplies a request key. The store checks that the caller, operation and request body match before it returns the saved reply. The retention sweep removes receipts older than a day.

Each open Watch stream begins with a snapshot. One reader per server checks for changes every two seconds and also wakes on a database notification. A stream that falls 2,048 changes behind is disconnected so it can open with a new snapshot.

See [the decision log](decisions.md).

## Workstreams

### Parts can store state and upgrade their own tables

Each part brings its own numbered database changes. The tests start the store twice against one empty database and check that each change is applied once. Parallel tests can separately create tables with the same names because they each get their own database.

### Writes arrive in commit order and safe retries reuse replies

A database transaction holds the record change, the feed entry and any receipt. The commit-order test shows that the website's cursor cannot pass a lower position whose write is still open. The retry test shows one record change and the same reply from two callers presenting the same request key and request.

### Watch starts with a snapshot and follows live changes

A Watch reads its first list and its feed position from one view of the database. A test changes a record during that read and observes the original list followed by the change. Another test holds one stream open without reading and verifies that the other stream keeps receiving changes as the slow stream is disconnected.

## Rollout order

1. Use the root Go module and shared Watch shape from sprint 0. Give the integration tests a PostgreSQL role that can create disposable databases.
2. Establish the tables and numbered migrations on an empty database. Prove that a startup failure leaves the prior schema and version records intact.
3. Run the server tests and the feed-lock mutation on the merged foundation. Compare the store and Watch interfaces with the parts that use them in later sprints.
4. In later sprints, the server opens the store and applies the manifests before serving. It starts the follower and cleanup after the migrations succeed. No existing engine data is moved.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

The Server team builds the two folders named in the brief. The Contract team owns the root Go module, its dependency files and the shared Protocol Buffers shape. The design calls for Go 1.26 or newer and PostgreSQL. Structured Query Language (SQL) examples below name the application's tables in the connection's configured schema. Read-write connections and read connections use the same schema and database endpoint in this sprint.

Phase 4 of the build loop applies to each item. The test writer uses the stated fixtures and compiles the tests against stubs if needed. The first run must fail on the named assertion. A test of a race treats a bounded timeout only as a fixture failure. The lock tests also assert that the writer reaches the expected gate before returning, so an empty stub can fail a behavior assertion. The builder keeps the frozen tests. The lead checks the exact commit, runs the named mutations on a disposable branch, and runs merged tests before moving the lead branch.

**WS-1. Parts can store state and upgrade their own tables**

A later server part gets typed access to its own tables through the store. Startup applies the numbered changes that the part registers. Tests exercise separate databases, including when packages run at the same time.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | Parallel tests can create the same table name in separate empty databases. The helper removes its databases and the store rolls back canceled work. | `TestDatabasesAreIsolated` and `TestTransactionsReleaseConnection`. |
| 2 | An identifier contains the version, variant and timestamp of a version 7 universally unique identifier (UUID). | `TestNewIDVersionAndTime` and `TestUUIDv7EntropyFailure`. |
| 3 | Concurrent startup applies each numbered change once per part. A failed batch leaves its tables and recorded versions unchanged. | `TestMigrateConcurrentStarts`, `TestMigrateFailureRollsBack` and `TestMigrateValidatesManifest`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Open the pool, define the query handles and isolate tests. | `src/server/internal/store/pool.go` `Open`, `Read`, `withWrite` and `src/server/internal/store/storetest/postgres.go` `NewDatabase`. See S1 in [scouting.md](scouting.md). |
| 2 | Generate identifiers in Go. | `src/server/internal/store/ids.go` `NewID`. See S2 in [scouting.md](scouting.md). |
| 3 | Load and apply the migration manifests and create the store tables. | `src/server/internal/store/migrate.go` `Migrate` and `src/server/internal/store/migrations/0001_store.sql` `schema_migrations`, `feed`, `feed_retention` and `receipts`. See S3 in [scouting.md](scouting.md). |

**1. Connection pool, query handles and test isolation**

Create `src/server/internal/store/pool.go`, `src/server/internal/store/pool_test.go` and `src/server/internal/store/storetest/postgres.go`. This item needs the root module and the driver's dependency entries from the Contract team. Item 2 can run beside it. It implements D2, D3, D4, R3, R7, R10 and C12.

The store uses `github.com/jackc/pgx/v5`, the driver named in the design's PostgreSQL check. Its query handles have the following public surface. Item 4 implements `Tx` with the same three query methods as `ReadTx` and adds `Record`.

```go
type Store struct { /* private pool and configuration */ }
type ReadTx struct { /* private transaction */ }
type Tx struct { /* private transaction and pending changes */ }

func Open(ctx context.Context, config *pgxpool.Config) (*Store, error)
func (s *Store) Close()
func (s *Store) Read(ctx context.Context, read func(context.Context, *ReadTx) error) error

func (tx *ReadTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
func (tx *ReadTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
func (tx *ReadTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
func (tx *Tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
func (tx *Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
func (tx *Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
```

`Open` copies the parsed configuration and pings before returning. `Read` begins a read-only `REPEATABLE READ` transaction. The private `func (s *Store) withWrite(ctx context.Context, write func(context.Context, pgx.Tx) error) error` runs a read-write `READ COMMITTED` transaction without automatic retries. Store code alone commits or rolls back. The caller closes query rows and uses the handle only during its callback. If a callback fails, panics or is canceled, roll back using a bounded cleanup context independent of the caller. Return callback or commit errors to the caller. An ambiguous commit error can be retried by the caller with the same request key. Application queries use parameters for values. The part owns its queries and table changes.

The real test helper exports `func NewDatabase(t testing.TB, schema string) *pgxpool.Config`. It reads `LGTM_TEST_POSTGRES` as a PostgreSQL connection string for an existing maintenance database. The test role needs `CREATEDB`. The helper fails the integration test if the variable is absent or provisioning fails. It creates a database from `template0` per test with a process-independent random name under PostgreSQL's identifier length limit. Quote the identifier with `pgx.Identifier.Sanitize`. The administrative connection remains in the maintenance database. There, issue `CREATE DATABASE <quoted-name> TEMPLATE template0` outside a transaction. Copy the parsed connection configuration, replace `ConnConfig.Database` and create a validated package schema such as `store_test` or `watch_test` inside the new database. Set `RuntimeParams["search_path"]` to that schema on the returned configuration. Raw test connections, the pool and the listener all use copies of that configuration. Close application connections before dropping their database. Register teardown immediately after creation so it runs on failure too. Confirm `DROP DATABASE <quoted-name> WITH (FORCE)` succeeds from the maintenance connection with a fresh bounded context. The database drop also removes the package schema. Use at least four connections in a test that needs competing writers, an observer and a listener. Separate tests use separate databases even when two processes run the same test package.

Tests first in `pool_test.go`.

- `TestDatabasesAreIsolated`. Run two parallel subtests with the same schema name. Before creating an identical table, assert it is absent from both databases. Insert a distinct value in each and read exactly that value. Assert that the database names differ. In the parent, after the parallel group completes, query the maintenance database for both names and assert that both are gone.
- `TestTransactionsReleaseConnection`. Use a one-connection pool. Write a row, cancel the context in the callback and return its error. Open an independent transaction through the pool and assert that it can read promptly and that the row is absent. Repeat with a callback panic caught by the test. In a read transaction attempt an insert and assert the PostgreSQL read-only error. Check that opening a refused connection returns an error without returning a store.

This item is risky because it sets the isolation level and controls transaction lifetime. Changing `Read` to a writable transaction must fail the read-only assertion. Changing its isolation to `READ COMMITTED` must fail `TestWatchSnapshotUsesOneTransaction` in item 9. Acceptance is the two tests above and the named downstream isolation mutation.

**2. Version 7 identifiers**

Create `src/server/internal/store/ids.go` and `src/server/internal/store/ids_test.go`. This item needs Go from the root module. It can run beside items 1 and 3. It implements D2, D3 and C12.

The public entry point is `func NewID() (string, error)`. Use Go's secure random reader for the 74 random bits and a clock for the 48-bit big-endian Unix timestamp in milliseconds. Set the version nibble to 7 and the two high variant bits to binary 10. Return the canonical lowercase hexadecimal form with hyphens. The private test seam is `func uuidV7(at time.Time, random io.Reader) (string, error)`. Keep no counter or node address. The time component sorts identifiers from distinct milliseconds in a test with an advancing clock.

Tests first in `ids_test.go`.

- `TestNewIDVersionAndTime`. Decode two deterministic identifiers from times one millisecond apart and a supplied byte vector. Assert the exact timestamp, version, variant and remaining bits. Assert that lexical order follows those two timestamps. Generate 4,096 real identifiers across concurrent goroutines and assert that none repeat.
- `TestUUIDv7EntropyFailure`. Pass a reader with fewer than the required bytes. Assert that the generator returns an error and an empty identifier.

This item is risky because it sets the sortable time component. Encoding the timestamp in little-endian order must fail `TestNewIDVersionAndTime`. Both tests are the acceptance observations.

**3. Numbered database changes and store tables**

Create `src/server/internal/store/migrate.go`, `src/server/internal/store/migrate_test.go` and `src/server/internal/store/migrations/0001_store.sql`. This item needs item 1. It can run beside item 2. It implements D3, D4, D5, R1, R3, R7, R10 and C12.

```go
type Migrations struct {
    Part  string
    Files fs.FS
}

func (s *Store) Migrate(ctx context.Context, parts ...Migrations) error
```

The startup caller passes each part explicitly. A part embeds `migrations/*.sql` and exposes a manifest whose `Files` starts at `fs.Sub(embedded, "migrations")`. The store adds its own embedded `store` manifest. Reserve that name. Validate the complete manifest before changing the database. Part names match `[a-z][a-z0-9_]*`. Files use `NNNN_description.sql` with four digits and a lowercase description, with contiguous numbers beginning at 1 for each part. Reject duplicate part names, invalid filenames, duplicate versions, gaps and a recorded version newer than the supplied files. Refuse startup when the database already records a part absent from the manifest. Rollback to an older binary therefore requires a compatible manifest and is an operator decision. The list of parts and same-part prefix history remain explicit.

Run startup in one read-write transaction. Acquire `SELECT pg_advisory_xact_lock(841941100002::bigint)` before creating the migration ledger with `CREATE TABLE IF NOT EXISTS` or reading it. Apply the embedded `store` part first and other parts in lexical order by name. Apply each part in number order. Update its last version in the same transaction as the migration. Run each trusted embedded SQL file as a whole using the driver's simple query mode without parameters. Migrations use transactional PostgreSQL statements and the part's own table names. Files already merged are append-only in the sense that a correction gets a new file and number. Startup returns an error before serving if validation or execution fails.

The bootstrap ledger is `schema_migrations(part text PRIMARY KEY, version integer NOT NULL CHECK (version > 0))`. The first store migration creates the following exact tables.

```sql
CREATE TABLE feed (
    position bigint GENERATED ALWAYS AS IDENTITY (START WITH 1 CACHE 1) PRIMARY KEY,
    topic text NOT NULL CHECK (topic <> ''),
    scope text NOT NULL,
    key text NOT NULL CHECK (key <> ''),
    change text NOT NULL CHECK (change IN ('saved', 'removed')),
    written_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE feed_retention (
    singleton boolean PRIMARY KEY CHECK (singleton),
    pruned_through bigint NOT NULL CHECK (pruned_through >= 0)
);

INSERT INTO feed_retention (singleton, pruned_through) VALUES (true, 0);

CREATE TABLE receipts (
    request_key text PRIMARY KEY CHECK (octet_length(request_key) BETWEEN 1 AND 256),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    reply bytea NOT NULL,
    written_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX receipts_written_at_request_key_idx
    ON receipts (written_at, request_key);
```

Feed keys and scopes are opaque text. An empty scope denotes a server-wide record. A receipt contains a digest and the successful reply bytes. The feed has no record payload. The primary key index serves ordered feed reads and prefix cleanup. The same store migration owns the retention row, which prevents a reader with an old cursor from silently stepping over deleted history. The store tables belong to the store. Future parts declare their own tables in their own manifests.

Tests first in `migrate_test.go`.

- `TestMigrateConcurrentStarts`. Bootstrap the store ledger once, then register two new fixture parts with version 1 and a second change in one part. Launch two startup calls on separate connections. Have the first hold a gate inside its first migration. Assert that the second waits for the first on the startup advisory lock before the gate is released. Use `pg_locks` and `pg_blocking_pids` to distinguish the startup lock from the test gate and a catalog lock. Release the gate. Both calls succeed. Observe the store schema through the catalog. Assert the ledger values `store=1`, the first part at 2 and the second at 1. Assert that non-idempotent fixture inserts occur once and in version order. A third call leaves the rows and versions unchanged.
- `TestMigrateFailureRollsBack`. Start with version 1 of a fixture part, then attempt version 2 that creates a new table followed by a failing statement. Assert an error, absence of the new table, and ledger version 1. Replace only the unmerged fixture file and assert a rerun records version 2.
- `TestMigrateValidatesManifest`. Submit duplicate names, duplicate numbers, missing numbers and invalid paths as table-driven cases. Assert a refusal before any ledger or table appears. After a valid migration, submit an older or incomplete manifest and assert a refusal with the previous ledger intact.

This item is risky because it controls schema order. Removing the startup advisory lock must fail `TestMigrateConcurrentStarts`. The three tests are the acceptance observations.

**Design artifact.** The database layer in the approved design supplies the brief. This workstream has no screen.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Parallel tests or a canceled transaction leave pooled connections or test databases behind. | Medium | Use a fresh cleanup context and prove teardown against the maintenance database in item 1. |
| Two startups apply the same migration or advance its ledger before its statements are durable. | High | Use one transaction and the startup advisory lock. Prove concurrent and failed starts in item 3. |

**Verify.** Run `go test -race -count=1 ./src/server/internal/store/...` with `LGTM_TEST_POSTGRES` pointing to a disposable PostgreSQL 16 or newer. The required environment is a real instance with the database creation privilege. No device check is needed.

**Rollout detail.** Step 1 makes the root Go module available and gives tests a PostgreSQL account. Step 2 applies `Migrate` before serving. Later server startup must close the store and report a migration error if startup cannot finish.

**WS-2. Writes arrive in commit order and safe retries reuse replies**

A part changes its current state and marks what changed in one transaction. The store keeps that update and its feed rows together. A part can wrap a keyed call in the same transaction so a retry sees its original reply.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 4 | A later writer cannot expose a position past a lower uncommitted position. A failed commit leaves no visible state or feed entry. | `TestFeedPositionsFollowCommitOrder`, its lock-removal mutation, `TestFeedRollbackAndNotification` and `TestFeedLockStartsAtFlush`. |
| 5 | A concurrent retry returns one stored reply. A mismatched retry is refused and a failed attempt leaves the key reusable. | `TestWriteOnceConcurrentRetry`, `TestWriteOnceMismatch` and `TestWriteOnceRollbackAndUnkeyed`. |
| 6 | Cleanup removes expired feed entries and receipts while retaining the cutoff entries and part-owned tables. | `TestPruneCutoffAndWatermark` and `TestPrunePreservesFeedPrefix`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 4 | Flush buffered feed changes under the transaction-level advisory lock. | `src/server/internal/store/feed.go` `Write`, `Record` and `flushFeed`. See S4 in [scouting.md](scouting.md). |
| 5 | Serialize callers that present the same request key. | `src/server/internal/store/receipts.go` `WriteOnce` and `Request`. See S5 in [scouting.md](scouting.md). |
| 6 | Prune old feed entries and receipts. | `src/server/internal/store/prune.go` `Prune` and `RunPruner`. See S6 in [scouting.md](scouting.md). |

**4. Feed writes and commit order**

Create `src/server/internal/store/feed.go` and `src/server/internal/store/feed_test.go`. This item needs items 1 and 3 and uses the `Tx` handle from item 1. It implements D3, D4, D5, R4, R7, R10 and C12. It can run beside item 2 if that item remains open, and it precedes items 5 and 7.

```go
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

The callback writes current state and calls `Record` for each changed domain record using a non-secret reference. Treat a `Record` error as an error of the callback. Validate nonempty topic and key, valid PostgreSQL text, and the two change values before buffering. Callbacks that change no domain record may leave the buffer empty. A part executes no SQL on the store's tables and its callback performs no external side effects. A watched record uses one topic and one scope per entry. The server-wide scope is the empty string. Keys for new records come from `NewID` when the domain owns the identifier.

After the callback and any receipt insert succeed, a write with pending changes runs these statements on its transaction. The flush inserts buffered changes in their recorded order. The request's current transaction commits immediately after the notification statement.

```sql
SELECT pg_advisory_xact_lock(841941100001::bigint);

INSERT INTO feed (topic, scope, key, change)
VALUES ($1, $2, $3, $4);

SELECT pg_notify('lgtm_feed_changed', '');
```

Only the store allocates positions. The identity uses a sequence with cache size 1, so a pooled session cannot hold a range of future positions. Gaps from rolled-back allocations are valid. Positions, rather than timestamps or notification order, are the cursor. PostgreSQL releases the advisory lock at transaction end. A database snapshot can therefore observe a prefix of committed positions. This applies when the snapshot reads the one writer or its physical standby. The empty notification is a wake-up hint. PostgreSQL makes it visible on commit with the rows. Position allocation and `written_at` occur in the final flush. `written_at` is the database clock at insertion, immediately before the commit attempt in ordinary operation. PostgreSQL cannot fill a column with the exact commit timestamp from that same transaction. This approximation is recorded in [decisions.md](decisions.md).

Tests first in `feed_test.go`.

- `TestFeedPositionsFollowCommitOrder`. Use two independent writer connections, a raw observer and two different fixture rows. Through a test-only PostgreSQL constraint trigger on `feed`, defer a gate for writer A until `COMMIT`. The gate waits for a separate advisory lock already held by the observer. Capture A's and B's backend identifiers from inside their callbacks. Wait for PostgreSQL to report A blocked on the gate after the feed insert has allocated its position. Start B and observe one of two outcomes before releasing the gate. With the production lock, `pg_blocking_pids` reports B blocked by A and a fresh observer query sees neither writer. Without the lock, B finishes and a fresh observer query sees B alone, because A remains blocked. In both cases release A and drain both writers with bounded waits. Assert that B waited for A, that after release the ordered feed reads A followed by B, and that a reader that advanced only on committed observations saw both rows. Any assertion failure records the observed blockers and visible keys. Removal of the production lock must fail on B becoming visible first. Removing the gate during teardown must release both connections even on test failure.
- `TestFeedLockStartsAtFlush`. Pause A's callback after its state write and `Record` on a channel. B writes a disjoint record and commits while A remains in the callback. Assert B is visible before releasing A. Then assert the ordered feed is B followed by A. This catches holding the advisory lock around domain work.
- `TestFeedRollbackAndNotification`. Start a raw listening connection and wait for registration before writes. Use a deferred trigger to make a write fail during commit after it allocated a position. Assert that neither its state nor its feed row is visible. Commit a second write that records two changes and assert their order, the matching current rows and one committed notification. Assert the aborted write did not notify. The test may use two bounded listener waits to prove silence and later delivery against a fresh database.

This item is risky because it establishes commit ordering. Remove only the production `pg_advisory_xact_lock` statement. `TestFeedPositionsFollowCommitOrder` must fail for B's early visibility. Restore it and rerun the test on the original commit. The three named tests and the mutation report are the acceptance observations.

**5. Request receipts**

Create `src/server/internal/store/receipts.go` and `src/server/internal/store/receipts_test.go`. This item needs item 4 and can run beside item 7. It implements D3, D5, R4, R7, R9, R10 and C12.

```go
const RequestKeyHeader = "Lgtm-Request-Key"

type Request struct {
    Key       string
    Caller    string
    Procedure string
    Body      []byte
}

var ErrInvalidRequestKey error
var ErrRequestKeyConflict error

func (s *Store) WriteOnce(
    ctx context.Context,
    req Request,
    write func(context.Context, *Tx) ([]byte, error),
) ([]byte, error)
```

The authenticated caller supplies a stable, non-secret principal and method name. It encodes the parsed body to stable bytes for identical retries. For a current Protocol Buffers caller use deterministic serialization of the same parsed request. The store hashes an unambiguous encoding of principal, method and body, using an unsigned 64-bit big-endian byte length before each field and the 256-bit Secure Hash Algorithm (SHA-256). It persists only the digest and reply bytes for the key. The header is optional. An empty key delegates the callback to `Write` without a receipt. With a nonempty key reject invalid 8-bit Unicode Transformation Format (UTF-8), a zero byte or more than 256 bytes with `ErrInvalidRequestKey`. The adapter treats key strings as case-sensitive. It passes only receipt-safe replies and never writes raw credentials into this table. It authenticates and authorizes before using the store. The later service owns the policy for calls returning credentials.

Inside the `Write` transaction, use `SELECT pg_advisory_xact_lock(1279743053, hashtext($1))` keyed by the exact request key. This two-integer lock space is distinct from the feed and migration lock space. A hash collision delays unrelated work without changing its result. After acquiring the lock, use `SELECT request_hash, reply FROM receipts WHERE request_key = $1 FOR UPDATE`. A matched digest returns the stored reply without calling `write`. A different digest returns `ErrRequestKeyConflict` without exposing the prior reply. The adapter maps it to Connect `already_exists` with “That request key was already used for a different request.” The adapter maps `ErrInvalidRequestKey` to `invalid_argument` with “The request key is invalid.” With no row, call `write`, insert its digest and successful reply and let `Write` flush its changes. Encode an empty successful reply as zero bytes. Return bytes only after the transaction succeeds. Write-once behavior is for keyed calls during receipt retention. A failed operation stores no result. No key, digest or reply is included in diagnostics.

Tests first in `receipts_test.go`.

- `TestWriteOnceConcurrentRetry`. Hold the first callback after a state insert. Start a retry on another connection with the same caller, procedure, key and body. Prove the second backend waits for the first's advisory lock or records a forbidden second callback attempt. Release the first. Assert two identical replies, one callback invocation, one receipt with the expected digest and one committed state update and feed row. Retry after reopening the store and assert the original reply comes from the database.
- `TestWriteOnceMismatch`. Store one successful receipt. Retry its key with a different body, caller or procedure in three subtests. Assert `ErrRequestKeyConflict`, no reply, no callback and no additional feed entry. Check that an oversized key and a key with a zero byte get `ErrInvalidRequestKey` before the callback.
- `TestWriteOnceRollbackAndUnkeyed`. Cause a commit-time failure after the receipt insert and feed allocation. Assert an error and no receipt, state or feed row. Reuse that key with different bytes and assert success. Make two successful unkeyed calls with a callback returning an empty successful reply and assert that both run without increasing the receipt count.

This item is risky because request-key matching and call ordering protect the write and reply. Delete the per-key lock or bypass the digest comparison. The respective concurrent and mismatch test must fail. Those test results and the rollback test are the acceptance observations.

**6. Feed and receipt retention**

Create `src/server/internal/store/prune.go` and `src/server/internal/store/prune_test.go`. This item needs item 5 and can run beside items 7 and 8 when their dependencies are ready. It implements D3, D5, R7, R10 and C12.

```go
type PruneResult struct {
    FeedRows    int64
    ReceiptRows int64
}

func (s *Store) Prune(ctx context.Context) (PruneResult, error)
func (s *Store) RunPruner(ctx context.Context, report func(error))
```

`RunPruner` runs one sweep at startup and then hourly until context cancellation. A sweep takes its cutoff from PostgreSQL as `clock_timestamp() - interval '24 hours'`. It prunes in committed batches of up to 512 feed rows and 512 receipt rows until neither batch found another full page. Report an error for the failed sweep and try again on the next scheduled sweep. `Prune` returns the count from committed batches, so an error after a batch can accompany nonzero counts. The caller supplies a sanitized diagnostic reporter. The sweep worker exposes private `func (s *Store) pruneBefore(ctx context.Context, cutoff time.Time) (PruneResult, error)` for deterministic retention tests.

For each batch acquire the `feed_retention` singleton row with `FOR UPDATE`. Select the earliest retained feed rows by primary key. Delete the consecutive beginning of that page for which `written_at < cutoff`. Stop at the first row still within retention. Advance `pruned_through` to the largest deleted position in the same transaction. PostgreSQL wall-clock adjustments can put an older timestamp after a newer one. The older row then waits for that prefix to expire. This preserves the meaning of the cursor when a watcher misses a day. Leave the sequence alone. Delete up to 512 receipts whose `written_at < cutoff` in timestamp and key order, holding the selected row locks through the delete. A concurrent valid replay holding a receipt row lock completes before deletion. Cleanup does not acquire the feed writer's advisory lock. The data removed here is limited to expired store feed entries and receipts. A part's current rows and its separate conversation or audit history are outside retention.

Tests first in `prune_test.go`.

- `TestPruneCutoffAndWatermark`. Use the private sweep entry point with a fixed cutoff taken from the real database. Insert more than one batch of fixture feed and receipt rows strictly older than the cutoff, plus rows at the cutoff and later. Assert that the older feed prefix and receipts are deleted and counted, cutoff entries remain, part-owned records remain, and `pruned_through` equals the greatest deleted feed position. Re-run at the same cutoff and assert zero deletions. Advance the cutoff until the rest of the feed is pruned. Then write a new entry and assert its position exceeds the previous head.
- `TestPrunePreservesFeedPrefix`. Insert one expired row, a newer row and a higher-position row with an older timestamp. Assert that the sweep deletes only the first row and places its watermark there. Advance the cutoff and assert the later sweep removes the remaining eligible prefix and advances to its last position.

This item is risky because a deleted row affects ordering. Change cleanup to delete by age without respecting the prefix, or skip updating `pruned_through`. The respective retention test must fail. Both tests are the acceptance observations.

**Design artifact.** The feed, ordering and safe-retry sections of the approved design supply the brief. This workstream has no screen.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A cursor advances past a late commit and skips a change. | High | Allocate positions only after acquiring the final-flush advisory lock and prove the exact failure when it is removed. |
| A repeated key causes a second write or exposes another reply. | High | Serialize same-key writes in PostgreSQL. Bind the digest to the authorized caller and procedure as well as the body. |
| Cleanup makes an old cursor silently cross a deleted entry. | High | Delete an ordered prefix and record its watermark in the deletion transaction. |

**Verify.** Run `go test -race -count=1 ./src/server/internal/store/...` with the same real database setup. On a disposable branch run `go test -count=1 ./src/server/internal/store -run '^TestFeedPositionsFollowCommitOrder$'` after removing the production lock call. The mutated run must exit nonzero on B's early visibility. The restored command must exit 0. Review reports carry the test name and the exact tested commit. No device check is needed.

**Rollout detail.** Step 2 installs the embedded store migration on the empty application database. Step 3 proves the mutation with two writer connections and a separate observer. A later server startup runs `RunPruner` after `Migrate` and cancels it before closing the store.
**WS-3. Watch starts with a snapshot and follows live changes**

One reader per process fetches committed rows. A bounded buffer shares those rows among the open streams. Each stream reads records and its starting position together, so a change can occur while the stream is opening without being lost.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 7 | A committed feed row reaches the reader on a notification. With no notification it arrives on a poll, in order and once. | `TestFeedReaderNotificationAndPolling` and `TestFeedReaderCatchupAndRetry`. |
| 8 | At 2,048 unacknowledged changes a subscriber is cut off. A draining subscriber can continue. | `TestFollowerLagAt2048`, `TestFollowerPrunedCursor` and `TestFollowerIgnoresSequenceGaps`. |
| 9 | The first message contains the entire result, followed by one message per later matching row. A blocked stream can be canceled and reopened with a fresh snapshot. | `TestWatchSnapshotUsesOneTransaction`, `TestWatchFiltersAndLoadsCurrentRecords` and `TestWatchCanRestartAfterLag`. |

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
    PrunedThrough int64
    Rows          []FeedRow
}

func (s *Store) readFeedPage(ctx context.Context, after int64) (feedPage, error)
```

`Snapshot` calls `load` and reads the high-water position inside the same `Read` transaction. It returns the greater of the highest extant position and `feed_retention.pruned_through`. At initialization with no feed row or pruning it returns zero. Keep `readHead(ctx context.Context, tx *ReadTx) (int64, error)` private so follower initialization uses the same calculation. The retention watermark preserves the logical position when cleanup has removed every row.

`readFeedPage` reads the pruning watermark and fetches up to 512 rows in the same read-only snapshot. Its indexed select is `WHERE position > $1 ORDER BY position ASC LIMIT 512`. Bind the greater of `after` and `pruned_through` to `$1`. Close its result set before returning. Each iteration advances to the highest position actually read or to the pruning watermark. When a page is full, drain another page without waiting. On a failed read, keep the last successfully applied position.

Define private `runFeedReader(ctx context.Context, s *Store, after int64, options feedReaderOptions, apply func(feedPage))`. It polls every two seconds with a separate timer from its notification path. It also drains on a wake from a single dedicated PostgreSQL connection. The listener executes `LISTEN lgtm_feed_changed` on that connection and signals a wake as soon as registration is effective. It uses a coalescing channel of capacity one. A lost listener reconnects at most once per poll interval and wakes the reader again on registration. A standby that cannot register still serves the periodic read path. Poll errors defer to the next wake or poll without resetting the cursor. Transient errors can be reported at the lifecycle boundary without credentials or arguments. Shutdown cancels waits and closes the listener. The follower and snapshot queries use one configured database endpoint that preserves position progress. Switching to a different history requires a restart of the follower and its streams.

Private options let tests turn off the listener, observe registration and read errors, control poll signals, and set a bounded database read deadline. The production default uses a real two-second ticker and attempts to listen. The data source is PostgreSQL in both modes.

Tests first in `read_feed_test.go`.

- `TestFeedReaderNotificationAndPolling`. Start the reader with test-controlled poll signals and no tick. Wait for real listener registration. Commit a row and assert it is delivered before the test signals a poll. Start a reader with the listener disabled, commit another row after its starting position and signal one poll. Assert that it receives the second row even though no notification can reach it. Signal another poll and assert no duplicate row. Assert the production poll interval is two seconds.
- `TestFeedReaderCatchupAndRetry`. Commit more than 512 real feed entries. Disable the listener and hold the only pooled connection. Signal a poll and observe a read failure when connection acquisition reaches its deadline. Release the connection and signal the next poll. Assert ordered, exactly-once reading of all committed entries across the page break, without another notification. Use fixture SQL to delete a prefix and update the watermark in one transaction. Read after the deleted prefix and assert that the page reports its watermark along with any surviving rows.

This item is risky because a missed wake or a bad cursor can lose a committed entry. Removing the poll must fail the poll-only case of `TestFeedReaderNotificationAndPolling`. Replacing `position > $1` with `position >= $1` must fail the no-duplicate assertion. The two tests are the acceptance observations.

**8. Bounded broadcast and subscriber lifetime**

Create `src/server/internal/store/follow.go` and `src/server/internal/store/follow_test.go`. This item needs item 7 and can run beside item 6 when ready. It implements D3, D5, R5, R7, R10 and C12.

```go
const MaxWatchLag = 2048

var ErrWatchLagged error

type Follower struct { /* private reader and subscriber state */ }
type Subscription struct { /* private cursor and cancellation state */ }

func StartFollower(ctx context.Context, s *Store) (*Follower, error)
func (f *Follower) Close()
func (f *Follower) Subscribe(ctx context.Context) (*Subscription, error)
func (sub *Subscription) Context() context.Context
func (sub *Subscription) Start(after int64) error
func (sub *Subscription) Next() (FeedRow, error)
func (sub *Subscription) Ack(position int64) error
func (sub *Subscription) Close()
```

The server starts one follower after migrations. Private `startFollower(ctx context.Context, s *Store, options feedReaderOptions) (*Follower, error)` reads the current head before starting `runFeedReader`. The exported entry point uses production options. Initialize the unavailable-through marker to the starting head so an arbitrarily old cursor cannot skip history loaded before startup. A first subscriber can then take its own snapshot at that head or later. The follower retains the last 2,048 observed feed rows in a process-wide ordered ring. Track the latest actual evicted position and the durable `pruned_through` separately. This state can be rebuilt from the database by opening a fresh Watch. A lock in this process protects the ring and subscription state. It never allocates positions or surrounds a database call, record loader or stream send.

`Subscribe` registers the stream before its snapshot query. `Start(after)` sets its cursor once. A cursor may be ahead of the follower if the database snapshot saw a recent commit first. Ignore rows through that cursor as the follower catches up. Cancel an active subscriber with cause `ErrWatchLagged` when 2,048 actual committed rows follow its acknowledged cursor, or when the cursor is below an evicted or pruned position. Recheck the same condition when starting a cursor and publishing a page. Use a cancellation cause so losing a stream interrupts its in-progress work even when it is outside `Next`. A per-subscriber wake channel may coalesce to one signal. The shared ring and one in-flight row replace per-subscriber backlogs.

`Next` returns the next row and records it as the only in-flight row. `Ack` accepts only its position and advances the cursor. The stream acknowledges filtered rows when it skips them, and matching rows after the caller's send returns successfully. This includes a pending matching row in the backlog count. A subscriber with an incomplete snapshot is subject to the threshold when it sets its cursor. Repeated or invalid `Start` and `Ack` calls return an error without moving the cursor. The Watch path uses one consumer per subscription. `Close` is idempotent on both objects. Follower close cancels and joins the reader and listener and unblocks subscribers. Closing a subscription removes it from the follower. No subscriber can delay the reader or another subscriber.

Tests first in `follow_test.go`.

- `TestFollowerLagAt2048`. Attach a slow and fast subscriber at the same head. Give the slow subscriber one row without acknowledging it. Drive feed commits through the real store, allowing the fast subscriber to acknowledge each batch before the next. At 2,047 rows assert the slow context is still open. At 2,048 assert its cause is `ErrWatchLagged` even though it has not called `Next` again. Assert the fast subscriber receives the boundary row and a subsequent row once each.
- `TestFollowerPrunedCursor`. Subscribe without activating it. Start it after its proposed cursor has fallen below the eviction watermark. Assert `ErrWatchLagged`. In a separate case write an old feed prefix, delete it and update the watermark together using fixture SQL while the reader is paused, then signal a poll. An active subscriber whose cursor is behind the pruning floor is canceled. A fresh subscriber starting at the current head receives the next new row.
- `TestFollowerIgnoresSequenceGaps`. In the isolated database create enough rolled-back or unused sequence allocations to make a large position gap. Then append one committed row. Assert the subscriber gets that row and stays open. Start a subscriber at a known head while the reader is behind it. Assert that it ignores earlier and equal positions and reads the first later one. Close the follower with a subscriber blocked in `Next` and assert that the call returns.

This item is risky because the cursor and buffer determine which changes get through. Replace actual row counts with subtraction of positions or change the boundary from `>=` to `>`. The gap and threshold tests respectively must fail. Those tests and the pruned-cursor test are the acceptance observations.

**9. Typed Watch streams**

Create `src/server/internal/watch/watch.go` and `src/server/internal/watch/watch_test.go`. This item needs item 8 and the shared Watch shape from sprint 0. It can run beside item 6 if that item remains open. It implements D3, D5, R2, R3, R5, R7, R8, R10 and C12. It provides the stream behavior that the Machines service needs for C6 through C9.

```go
type Source[T any] struct {
    Topic    string
    Scope    string
    Snapshot func(context.Context, *store.ReadTx) ([]T, error)
    Load     func(context.Context, *store.ReadTx, string) (T, bool, error)
}

type Snapshot[T any] struct {
    Records []T
}

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

The service authorizes the requested topic and scope before calling `Serve`. A source supplies readers over its own tables. `Serve` registers the subscription first and defers its close. It runs `source.Snapshot` inside `Store.Snapshot` using the subscription context and starts the subscription at that position. Then it sends a message with a non-nil `Snapshot`, even when the result is empty. Exactly one variant is set in any message. Every emitted position identifies the snapshot's database point or the corresponding feed row.

`Serve` skips and acknowledges rows whose topic or scope differs. A matching `removed` row emits its key. For a matching `saved` row, run `source.Load` in a new `Store.Read` transaction. Send the current record if found. If it is already gone, send the key and the row's position. Its later removal entry still receives its own message if it matches. Never hold a database transaction open during a send. Acknowledge after each successful send. Errors from a source or sender end the stream. A subscriber canceled for lag returns `store.ErrWatchLagged`, even if the in-progress load or send returned context cancellation first. Lag cancellation also applies while the first message is being sent.

This generic envelope follows the design's shared Watch shape. The future service adapter maps snapshot, saved and removed-key variants into its generated response and carries the feed position in the field fixed by sprint 0. The store and generic helper need no generated-type import. The comparison with sprint 0 checks the three variants, the signed 64-bit position range and the first empty snapshot. Map and preview adapters are built with their services.

The sender must honor the subscription context even when a client blocks the transport. A later Connect adapter maps lag to `resource_exhausted` with “The watch fell behind and must be reopened.” when the transport can still emit a status. A blocked response must be interrupted at the transport, such as with a per-response write deadline triggered by subscription cancellation. Treat a closed transport as a reconnect signal. Keep writes serialized. The later service must test cancellation on a real blocked stream as well as through this helper's callback.

Tests first in `watch_test.go`.

- `TestWatchSnapshotUsesOneTransaction`. Establish a real follower and a known row. Pause the snapshot loader after its first read. Commit a change to that row on another connection and wait for the follower to observe the writer. Resume the loader. Assert the first message contains the old value at the old position and the next carries the new value at its later position. Run the same barrier when starting with an empty table and assert there is still an empty snapshot first. In a test-only query or an isolation mutation demonstrate that two read-committed statements can see different database points under this fixture.
- `TestWatchFiltersAndLoadsCurrentRecords`. Verify an empty first result on the selected topic and scope. Commit rows for another topic and scope, alongside a matching save and removal. Delay handling the save until a subsequent update is committed and assert it emits the current value while preserving the saved row's position. In a second case remove a record before a saved row is loaded and assert a removal message for the saved position and one for the removal position. Send a matching marker as the final row and check the exact message positions through that marker. This proves the nonmatching rows produce no messages. Verify that another write commits while a send callback is held.
- `TestWatchCanRestartAfterLag`. Hold the sending callback until its context closes. Commit 2,048 actual rows after the last acknowledged position and let the follower drain. Assert the held callback wakes with the lag cause and `Serve` returns that error. Start a new Watch and assert the first message reflects the whole current result, including the committed changes. In a separate subtest cancel the caller and assert that the subscription and wait are released with caller cancellation.

This item is risky because snapshot and feed ordering determine whether a reader sees a change. Split the state read and position lookup into separate transactions. `TestWatchSnapshotUsesOneTransaction` must fail on the missing update. The three tests are the acceptance observations.

**Design artifact.** The approved design's Watch procedure and the existing Machines stream are the shape references. This workstream has no screen.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A notification is missed or a read is interrupted. | Medium | Keep a database cursor, use a two-second poll, and prove polling without a listener. |
| A slow network stream holds an unbounded backlog or blocks another stream. | High | Share one fixed ring and cancel at 2,048 actual rows, including the in-flight row. Require a cancellable sender. |
| A write lands between the initial record read and the cursor read. | High | Use one read-only repeatable-read transaction and a gated two-connection test. |

**Verify.** Against disposable PostgreSQL, run `go test -race -count=1 ./src/server/internal/store/... ./src/server/internal/watch/...`. Run the notification-disabled branch without a listener to exercise the standby delivery path on a real primary. This sprint needs no device. The actual standby and the Connect transport adapter are outside the sprint's deployment. The later Machines service owns the blocked-transport test.

**Rollout detail.** Step 3 checks the buffer and Watch helper on real PostgreSQL. Run `go test -count=1 ./src/server/...` on the merged Server work. At the full wave 1 verification run `go test ./...` against the same test setup for C12. In later server startup, initialize one follower after migrations. On shutdown stop streams and join follower and pruning work before closing PostgreSQL connections.

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
| D9, D10 | WS-2 | Keep credentials out of receipts. Vault setup and secret access belong to the stack and sign-in plans. |
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
| `.plans/epics/go-rewrite/epic.md`, The teams, and what each may edit | All implementation files are in the allowed Server folders. The Contract team supplies root module entries and the shared message shape. |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, Rules that every later part follows | Parts own their tables and use the store's write path. The common Watch helper reads current state. Integration tests use PostgreSQL. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Backend covers server logic and SQL here. Item 3 is a startup runner that also provides the first schema on an empty database. There is no conversion of existing data. The `server/backend` scope is kept by the orchestrator. The table names the required review pass for boundary or ordering changes. All item-level acceptance tests remain mandatory. Work can run in parallel once the named prerequisites have merged. Each item owns a separate set of files.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | backend | server/backend | medium | The pool, isolation and database fixture cross the PostgreSQL connection boundary. | Yes | Sprint 0 module and driver entries. |
| 2 | backend | server/backend | low | The format uses one documented byte layout and a small pure test seam. | Yes | Sprint 0 module. |
| 3 | backend | server/backend | medium | One runner applies a validated manifest in a serialized startup transaction. | Yes | 1. |
| 4 | backend | server/backend | medium | One flush path needs a controlled commit race across two connections. | Yes | 1 and 3. |
| 5 | backend | server/backend | medium | The receipt check wraps the existing write path and adds one per-key lock. | Yes | 4. |
| 6 | backend | server/backend | medium | A bounded deletion must preserve a feed prefix and a durable watermark. | Yes | 5. |
| 7 | backend | server/backend | medium | One reader reconciles an advisory signal with ordered reads and a periodic fallback. | Yes | 4. |
| 8 | backend | server/backend | medium | One bounded memory structure owns subscribers and their cancellation. | Yes | 7. |
| 9 | backend | server/backend | medium | One snapshot-to-stream helper reuses the bounded subscriber and exposes a sender contract. | Yes | 8 and sprint 0 Watch shape. |

Load for the first full plan review and build is 12 OpenAI launches, 15 Anthropic launches and five xAI launches. This assumes nine build and nine independent test-writing launches split evenly across the first two vendors. The initial plan review adds one OpenAI, four Anthropic and one xAI launch under the epic's assignments. Two post-build slices add one OpenAI, one Anthropic and one xAI launch per slice. The budget includes two additional xAI launches for focused checks. Risky mutations run in those review seats on disposable branches. A further full plan round adds one OpenAI, four Anthropic and one xAI launch. The remaining weekly allowances are held by the orchestrator and were unavailable for this assignment. Compare these counts with those readings before scheduling. If an allowance is short, schedule the dependent work after the relevant reset.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (blocker, major, minor) | Outcome |
|---|---|---|
| Initial self-check | Plan review has not run. | The orchestrator runs the review rounds after this handoff under the epic's phase 3 rules. |

</details>
