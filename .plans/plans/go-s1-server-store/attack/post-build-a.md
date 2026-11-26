# Post-build review A of the plan go-s1-server-store, work items 1, 2 and 3

Task identifier: postbuild-go-s1-server-store-a. Nonce: 06d037.
Reviewer role: reviewer of built code, model claude-sonnet-5-5 at high effort. I did not build this code and did not write its tests.
Reviewed range: `git diff 02419b5c..4d3d0430 -- src/server go.mod` on the branch `sprint-0`, plus the frozen test files read from the folder.
Items covered: the pool (item 1), the identifiers (item 2) and the numbered database changes with the store tables (item 3).

## Verdict

**SHIP.** There is no blocker and no major finding. Ten minor findings follow.

## Top finding

The tests of items 1 to 3 pass and catch every mutation that the plan names, but they leave the connection defaults, the isolation level of `Read`, the order of migration parts and the copying of the caller's configuration unprotected.

## What I ran

All database tests ran through `deploy/test/with-postgres.sh`, which started its own disposable PostgreSQL 16.16 container and removed it afterwards. I started no other container and stopped none.

1. **Tests with the race detector.** The shared tree does not compile its `store` test package right now. `feed_test.go`, which belongs to item 4, uses `storetest.WaitOnRow`, and that function does not exist yet (`feed_test.go:622:13: undefined: storetest.WaitOnRow`). That is another team's work in progress, so I do not report it as a finding. I ran items 1 to 3 in a scratch copy of `src/server`, `go.mod` and `go.sum` without `feed_test.go` and `receipts_test.go`.
   - Run with `go test -race -count=1 -v ./src/server/internal/store/...` under `GOFLAGS=-p=1 GOMAXPROCS=2`.
   - These passed: `TestNewIDVersionAndTime`, `TestUUIDv7EntropyFailure`, `TestMigrateColdConcurrentStarts` (1.42 s), `TestMigrateConcurrentStarts` (1.00 s), `TestMigrateUsesStartupBudget` (41.74 s), `TestMigrateFailureRollsBack`, `TestMigrateValidatesManifest`, `TestDatabasesAreIsolated`, `TestTransactionsReleaseConnection`, `TestWritersCannotStarveReaders` (10.98 s), `TestOpenRefusesPlainRemote`, `TestDatabaseErrorsHideValues`, `TestNewDatabaseRejectsWrongMaintenanceName` and, in the fixture package, `TestNewMigratedStore`.
   - The five prune tests failed in the same run. Pruning is item 6 and only a stub exists, so those failures belong to another team.
2. **A run without the database setting.** With `LGTM_TEST_POSTGRES` unset, the database cases skip and the pure cases pass. `TestMigrateUsesStartupBudget` then reports PASS while both of its subtests are skipped.
3. **The fixture and the connection budget.** `SHOW max_connections` printed 100. I sampled `pg_stat_activity` every 50 milliseconds during a full run, and the peak was 8 client backends. No test printed `53300`.
4. **The dependency row.**
   - `go.mod` pins `github.com/jackc/pgx/v5 v5.13.0`.
   - `go mod tidy -diff` printed nothing.
   - `gofmt -l src/server` printed nothing.
   - `go vet ./src/server/...` printed nothing.
   - `govulncheck` was not installed, so I built it in a scratch folder with the repository's Go 1.26.0. It reported 15 vulnerabilities in the standard library that the code reaches, and none in `pgx`, `puddle`, `x/sync` or `x/text` (finding post-a 7).
5. **Mutations in scratch copies.** The first table lists the mutations that the plan names. The second lists my own.

| Mutation | Tests I ran | Result |
|---|---|---|
| `Read` made writable (`pgx.ReadWrite`) | pool tests | `TestTransactionsReleaseConnection` fails at `pool_test.go:339`. |
| Timestamp encoded little-endian | identifier tests | `TestNewIDVersionAndTime` fails at `ids_test.go:52`. |
| Migration lock deleted | migrate tests | `TestMigrateColdConcurrentStarts` and `TestMigrateConcurrentStarts` fail (`migrate_test.go:140`, `:220`). |
| Ledger creation moved before the lock | migrate tests | `TestMigrateColdConcurrentStarts` fails at `migrate_test.go:140` with one waiter instead of two. |
| Writer admission removed | pool tests | `TestWritersCannotStarveReaders` fails at `pool_test.go:415`. |

| My mutation | Result |
|---|---|
| Private-address check made always true | `TestOpenRefusesPlainRemote` fails (`pool_test.go:534`). |
| Variant bits changed to `0xc0` | `TestNewIDVersionAndTime` fails. |
| Migration statement timeout changed from five minutes to 15 seconds | `TestMigrateUsesStartupBudget` fails. |
| Write lock timeout set to zero | `TestWritersCannotStarveReaders` fails (`pool_test.go:427`). |
| Pre-commit rollback removed | The test run hangs until the ten-minute limit, so it is caught only by timeout. |
| `Read` changed to `READ COMMITTED` | **Survives.** Finding post-a 2. |
| Session `statement_timeout` removed | **Survives.** Finding post-a 1. |
| Session `idle_in_transaction_session_timeout` removed | **Survives.** Finding post-a 1. |
| Keepalive not examined by any test | No test reads it. Finding post-a 1. |
| Migration parts applied in reverse lexical order | **Survives.** Finding post-a 3. |
| `Open` uses the caller's configuration instead of a copy | **Survives.** Finding post-a 4. |
| Page pool size raised from one to two | **Survives.** The plan places the page pool in items 4 and 5, so I do not report it. |

Three more mutations survived only because I had to remove item 4's test file: the commit outcome of an unknown error, the commit outcome of a lost connection, and the cancellation check before commit. The plan gives those to `TestFeedCommitErrorOutcome`, which I could not compile. I could not judge `sanitizeCommitError` and the check before commit (`pool.go:124-140`, `pool.go:334-336`), and the post-build check "Commit outcomes" must run once item 4's tests compile.

## Findings

### post-a 1, MINOR: the connection defaults are not tested

The plan says to default to 30-second keepalives, 30 seconds idle in a transaction and 15 seconds per statement (`plan.md` line 135). `configureConnection` sets all three (`pool.go:240-247`, `pool.go:281`). No test reads any of them. The only `statement_timeout` assertions are in the migration budget test.

Failure scenario: A later edit drops `idle_in_transaction_session_timeout` or the session `statement_timeout`. Every test still passes. A read callback that stops between statements then holds a pooled connection and its snapshot until the caller gives up. A slow read has no server-side limit, because only the write path sets `SET LOCAL statement_timeout`.

Fix: Add one test that opens a store and reads `current_setting` for both timeouts on a read connection, and reads the keepalive configuration from the connection that the dial function returned.

Tie: plan, item 1.

### post-a 2, MINOR: the isolation level of `Read` is not tested

`Read` uses a read-only `REPEATABLE READ` transaction (`pool.go:297`, `plan.md` line 139). Changing it to `READ COMMITTED` passes every test of items 1 to 3, because only the read-only half is asserted (`pool_test.go:339`). The plan does name a gated test for this in the watch work (`plan.md` line 625), but that test comes later and is about the loader.

Failure scenario: Someone changes the level. A reader that queries two tables sees one write between its two queries. The snapshot and the feed position then disagree, which is the failure that the design of the change feed exists to prevent.

Fix: Add a test in item 1 that opens a read, lets a second connection commit a row, queries again inside the same read and requires the old result. Run it once with the level removed to see it fail.

Tie: D5 (tables hold the state and the feed sits beside them), plan item 1.

### post-a 3, MINOR: the order of the migration parts is not tested

The plan says to apply the store part first and then the other parts in lexical order (`plan.md` line 194, `migrate.go:131`). Replacing `slices.Sort(order[1:])` with `slices.Reverse(order[1:])` passes every migration test. `TestMigrateConcurrentStarts` uses the parts `alpha` and `beta`, but the rows they write do not depend on each other, so the order cannot be observed (`migrate_test.go:191-247`).

Failure scenario: Two parts later get a foreign key from one to the other. Their startup order depends on the sort. The order becomes wrong, or random if the sort is lost, and the fault shows only on the first startup of a clean database.

Fix: Make the `beta` file read a row that the `alpha` file wrote and make the `alpha` file read nothing. Also make a part named `zeta` write before `alpha` if the map were walked in a random order. Check that the ledger rows were written in that order.

Tie: plan, item 3.

### post-a 4, MINOR: `Open` copying the configuration is not tested

The plan says `Open` copies the configuration (`plan.md` line 135, `pool.go:147`). Replacing `config.Copy()` with the caller's pointer passes every test.

Failure scenario: Sprint 2 bootstrap passes one `*pgxpool.Config` to `Open` twice, for example after a failed migration. The second call wraps `DialFunc` and `BeforeConnect` a second time, and it sees the runtime parameters that the first call added. With the copy gone, the first caller's values leak into the second pool, and the second `Open` can accept a configuration that the first call changed after validation.

Fix: Add a test that calls `Open` and then compares the caller's `RuntimeParams`, `DialFunc` and `BeforeConnect` with their values before the call.

Tie: plan, item 1.

### post-a 5, MINOR: two failures give the operator no useful sentence

First, a migration failure reaches the caller as "The database operation failed." with at most a five-character SQLSTATE. The error does not name the part or the file that failed, and `TestMigrateUsesStartupBudget` shows this in its failure text (`migrate_test.go:281`). Second, `Open` returns "The database connection could not be opened." when `MaxConns` is below eight (`pool.go:144`), although the connection is fine. The configuration is wrong.

Failure scenario: A migration of part `machines` fails on a clean database during the first start of the stack. The operator sees one sentence and reads no log, because the error text is deliberately free of values. They cannot tell which of the ten files to look at. A second operator sets `pool_max_conns=4` and spends time on the network.

Fix: Allow the part name and the file name in the error, since both come from the trusted embedded manifest and not from data. Return a separate fixed sentence for a pool that is too small.

Tie: R8 (an error is one sentence that can be shown), plan items 1 and 3.

### post-a 6, MINOR: a run without the database setting looks green

With `LGTM_TEST_POSTGRES` unset, `TestMigrateUsesStartupBudget` and its two subtests report `--- PASS` for the parent while both subtests skip, and the package prints `ok`. The plan accepts that ordinary tests skip and relies on C12 to refuse that run (`plan.md` line 744). The refusal is not part of items 1 to 3, and I did not find it in the folders I read.

Failure scenario: A builder runs `go test ./...` on a laptop with no database, reads `ok`, and merges. Nothing in items 1 to 3 told them that the migration, pool and fixture tests did not run.

Fix: Check once that the root hook that C12 names refuses a run without the setting. In the parent test, call `t.Skip` before the subtests so that the line says SKIP and not PASS.

Tie: C12, R10 (tests run against a real PostgreSQL).

### post-a 7, MINOR: the vulnerability scan is not clean

The plan's dependency row expects `govulncheck ./...` to report no known vulnerability in the used paths (`plan.md` line 744). With the installed Go 1.26.0, `govulncheck ./src/server/...` reports 15 vulnerabilities in the standard library that the code reaches, in `crypto/x509`, `crypto/tls`, `net`, `net/url`, `encoding/asn1` and `encoding/xml`. The fixes are in Go 1.26.1 to 1.26.11. It reports no reached vulnerability in `pgx`, and 26 in required modules that the code does not call. `go.mod` has `go 1.26.0` and no `toolchain` line (`go.mod:3`).

Failure scenario: The server is built with the pinned minimum version and keeps the 15 standard library faults, among them certificate checks that the verified-TLS path in `Open` relies on.

Fix: Add a `toolchain` line for the newest Go 1.26 release, or build with it, and run `govulncheck` again. The owner has decided that security issues do not block the build, so this is a note and not a gate.

Tie: plan (the dependency row of the post-build checks).

### post-a 8, MINOR: `Close` can block forever and no test covers it

`Open` closes the pools it built on each failure path (`pool.go:168-180`), and I found no leak there. `Close` closes the page pool and then the main pool (`pool.go:288-292`). `pgxpool.Close` blocks until every connection is released, so a callback that never returns makes `Close` block forever. No test calls `Close` while a callback is running.

Failure scenario: A shutdown handler calls `Close` while a read callback waits for a lock that nobody will release. The server never exits, and the stack's stop timeout kills it.

Fix: Document that callers must cancel their contexts before `Close`, or give `Close` a context with a deadline. Add a test that calls `Close` with a cancelled callback in flight.

Tie: plan item 1 ("`Close` closes the pools after the follower stops").

### post-a 9, MINOR: a nested error loses its outcome when it is sanitized again

`sanitizeError` turns any nested `*Error` into a new error with `OutcomeRolledBack`, whatever the inner outcome was (`pool.go:117-120`). That is correct inside a transaction before commit, where nothing is committed. It is wrong if a callback returns an error that an earlier `Store` call produced with `OutcomeUnknown`, for example a write called inside a read that wraps a result.

Failure scenario: Code in a later part calls `WriteOnce` from inside another store callback and returns its error unchanged. The outer call reports a rolled-back write, although the inner write may have committed.

Fix: Keep the inner outcome when the nested error says unknown, or refuse a store call from inside a store callback. Add a test for the chosen rule.

Tie: plan item 1, R4 (every write goes through the store's transaction and says what changed).

### post-a 10, MINOR: the fixture does not check the server's connection limit

`storetest.NewDatabase` raises the pool size to at least eight (`storetest/postgres.go:70`) and opens one database per test, but it never reads `max_connections`. The plan's fixture row asks for a measured peak that fits (`plan.md` line 743). I measured it: the limit is 100 and the peak was 8, so it holds today.

Failure scenario: A later suite runs tests in parallel across packages against a server with a low `max_connections`. The tests fail with `53300` in a way that looks like a store fault.

Fix: In the fixture, read `SHOW max_connections` once per run and skip or fail with a plain sentence when the planned peak exceeds it.

Tie: R10, C12.

## Grade

**8 out of 10.** The code does what its "Done when" rows say. The five mutations that the plan names each fail the intended test, the endpoint policy, the rollback handling and the migration lock behave as designed, and the race detector found nothing. The grade is held back by four behaviours that the plan states and no test guards (post-a 1 to 4) and by the clean-scan expectation that the installed Go version does not meet (post-a 7).

## Not covered

- The commit outcome classification and the cancellation check before commit, until `feed_test.go` compiles.
- The post-build rows for the feed, receipts, pruning, notification and watch work. They belong to other items.
- The PostgreSQL 17 repeat. Only PostgreSQL 16.16 was available.
