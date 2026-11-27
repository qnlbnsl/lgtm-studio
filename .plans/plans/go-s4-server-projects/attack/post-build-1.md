# SHIP

The frozen tests pass even if deleting a repository also deletes the saved Add reply needed for a retry.

OpenAI supplied the test writer, the builder and this reviewer. I reviewed Projects item 1 on `sprint-0` using `57a8942c~1..476b98bc`. The task identifier is `postbuild-go-s4-server-projects-1` and the nonce is `a8412a`. [Builder result](../../../../.foundry/go-rewrite/results/s4-projects-1-build.md#L3)

## Findings

### post-1 1 — MINOR — The frozen tests overlook deletion of a retained result

**Statement.** Plan item 1 requires settled operations to survive removal of the target repository so a same-key retry can recover the original reply. [Plan](../plan.md#L116)

**Evidence.** The shipped schema keeps the target identifier as plain text. The storage test inserts a successful operation whose target is absent, which catches a foreign key. Its repository deletion case runs while folders still reference the repository and before it inserts any operations. Both frozen tests passed with a scratch `AFTER DELETE` trigger that deleted operations targeting a removed repository. [Schema](../../../../src/server/internal/projects/migrations/0001_projects.sql#L22) · [Deletion case](../../../../src/server/internal/projects/migrate_test.go#L188) · [Terminal fixture](../../../../src/server/internal/projects/migrate_test.go#L201)

**Failure scenario.** An Add succeeds and the reply is lost. The last folder is removed and the repository is deleted before the retry. In a PostgreSQL probe, deletion left one matching operation with the original bytes under the reviewed schema and zero under the trigger mutation. The remaining request receipt would point to a missing operation in the mutated schema. [Removal and retirement boundary](../plan.md#L201)

**Fix.** Seed a terminal Add against a real target, delete its last folder and then the repository, and assert that the operation still has the target identifier and identical reply bytes.

### post-1 2 — MINOR — The index assertions overlook reversed state filters

**Statement.** Plan item 1 requires an index by machine for open operations and an index by retention stamp and operation identifier for terminal operations. [Plan](../plan.md#L114)

**Evidence.** The shipped index predicates select the intended rows. The test checks the key columns and whether a predicate exists. Both frozen tests passed with the two predicates swapped. [Index definitions](../../../../src/server/internal/projects/migrations/0001_projects.sql#L41) · [Catalog assertion](../../../../src/server/internal/projects/migrate_test.go#L232)

**Failure scenario.** In PostgreSQL with sequential scans disabled for the probe, the reviewed schema gave a machine-key index scan for open operations and an ordered index scan for mature operations. With the predicates swapped, the open lookup filtered by machine after the index scan, and the mature lookup used a bitmap scan followed by a sort. The swap removes the intended access paths from machine removal and retirement. [Retirement query requirements](../plan.md#L133) · [Machine removal requirements](../plan.md#L201)

**Fix.** Check the state predicate of each partial index through the PostgreSQL catalog in addition to its key columns.

## Post-build checks and plan conformance

By reading the plan and newest register, item 1 has zero places marked `UNREVIEWED FIX`. The unreviewed refresh mechanism belongs to attachment, removal and watch in items 5 through 7. The Upgrade scenario still has a storage check for item 1. [Register](round-1-findings.md#L9) · [Refresh mechanism](../plan.md#L191) · [Upgrade](../plan.md#L285)

Confirmed live on PostgreSQL 16.16, a trigger refused the Projects ledger insert after the migration statements ran. The three tables and their ledger entry rolled back. Retrying created three empty tables and recorded Projects version 1. Reapplying the migration preserved the saved repository, folder and operation. The old manifest refused the upgraded database. The live session and the original Machines reply survived the interruption and upgrade. The ordinary Machines receipt replayed just before its 24-hour cutoff and pruned just after it. This satisfies the storage criterion for item 1. The pinned source has only Store migration 1. Mounted discovery belongs to item 8, and the combined deployed upgrade belongs to Stack. [Done when](../plan.md#L66) · [Upgrade fixture](../../../../src/server/internal/projects/migrate_test.go#L28) · [Cutoff assertions](../../../../src/server/internal/projects/migrate_test.go#L113) · [Startup gate](../plan.md#L243)

By reading the migration, the three tables, their columns and constraints, and their four secondary indexes match item 1. The migration follows the part-owned numbering rule. Store serializes and applies the schema and ledger writes in one transaction. The migration adds no runtime worker or file handle, and Store rolls back and sanitizes errors at the transaction boundary. I found no implementation defect in the bounded item. [Projects registration](../../../../src/server/internal/projects/migrate.go#L10) · [Schema](../../../../src/server/internal/projects/migrations/0001_projects.sql#L1) · [Migration transaction](../../../../src/server/internal/store/migrate.go#L36) · [Transaction cleanup](../../../../src/server/internal/store/pool.go#L320) · [Foundation migration rule](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L184)

## PostgreSQL verification

I ran the frozen test gate in a disposable copy of `476b98bc` with a writable Go build cache.

```sh
deploy/test/with-postgres.sh -- go test -count=1 -p 1 ./src/server/internal/projects -v -run '^TestProject(Migration|StorageConstraints)$'
```

Both tests and all 20 storage subtests passed with zero skips. The command exited 0.

```text
--- PASS: TestProjectMigration (2.64s)
--- PASS: TestProjectStorageConstraints (0.47s)
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  3.129s
```

I applied each mutation independently in the same scratch copy. The failure output below includes the Structured Query Language (SQL) state code where the frozen assertion prints one. [Planned mutations](../plan.md#L118)

| Mutation | Result |
|---|---|
| Drop the local folder-to-repository foreign key. | Exit 1. `migrate_test.go:161` printed `The invalid Projects fixture statement succeeded, want SQLSTATE 23503.` |
| Drop the operation result-shape check. | Exit 1. `migrate_test.go:227` printed `The invalid Projects fixture statement succeeded, want SQLSTATE 23514.` |
| Drop the separate state enumeration while keeping the result-shape check. | Exit 0. The remaining result-shape check still rejects unknown states. |
| Put the full resolved path in the machine index. | Exit 1. `migrate_test.go:181` printed `The valid Projects fixture statement failed with SQLSTATE 54000.` |
| Add a cascading foreign key from an operation to its target repository. | Exit 1. `migrate_test.go:201` printed `The valid Projects fixture statement failed with SQLSTATE 23503.` |
| Delete targeted operations with an `AFTER DELETE` trigger. | Both frozen tests passed and exited 0. The independent deletion probe reproduced the lost result. |
| Swap the state predicates on the two partial indexes. | Both frozen tests passed and exited 0. The independent query plans showed that the intended scans were unavailable. |

**Grade.** 9 out of 10.
