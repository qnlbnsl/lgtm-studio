---
topic: go-s4-server-projects
status: attack
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# The server can inspect, save and follow repositories on machines

## Overview

A signed-in owner will be able to inspect a checkout on a connected machine and save it as a repository. The website will follow its saved folders, including checkouts attached from other machines.

The server will save each request so a retry can recover its result after a restart. Machine removal commits with folder and operation cleanup. A large removal makes the open repository list reload from the committed data.

The machine connection must apply the folder report before command settlement. The database keeps the request key until its result can retire.

## Key decisions

PostgreSQL will retain the key while the operation is open and for at least one day after settlement.

Aliases resolve to one folder. Attachment and removal share a repository lock. Sprint 5 needs a decision about which checkout to run when a machine holds two distinct folders for one repository.

[Decision log](decisions.md)

## Workstreams

### Folder requests survive a lost reply

A delayed report can settle after the website stops waiting. The original result remains available even if the machine goes offline.

### Saved folders follow the machines that hold them

Attachment checks that the checkout belongs to the selected repository. Removing a machine keeps shared repositories that still have folders elsewhere.

### The website can discover and follow saved repositories

A signed-in website gets the current list and later changes through the existing guards. Discovery advertises Projects when its routes are mounted.

## Rollout order

1. Land Contract, Store and Link before Projects integration. Update the machine client to revision 2.
2. Migrate before listening. Mount the guarded routes and supervise retirement.
3. Run combined checks, then verify discovery and both deployed repository entry points.

---
<!-- ⛔ implementation details and gate evidence below -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

Application programming interface (API) and workstream (WS) appear in the headings and source names. D1 to D23 and C1 to C13 refer to foundation decisions and checks. R1 to R14 are sprint 4 checks. Identifiers are canonical lowercase version 7 universally unique identifiers (UUIDs) with the standard variant. Text uses Unicode Transformation Format, 8-bit (UTF-8). A kibibyte is 1,024 bytes. Braces enumerate files.

Use the root Go module, reviewed Protocol Buffers bindings and real PostgreSQL via `storetest.NewDatabase`. Use the signed `link/machinetest` fixture for controlled Link events. Paired and deployed tests run the real client. Freeze tests after they fail their named assertion against a compiling stub or existing code. Run each risky mutation in isolation and restore the code before integration.

**WS-1. Folder requests survive a lost reply**

The first committed answer survives retries and restarts.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 9 | Held receipts survive upgrade and pruning, share caller-wide keys and retire with their operations. | `TestHeldReceiptUpgrade`, `TestHeldReceiptSharedKeys`, `TestHeldReceiptPruneAndRetire`. |
| 1 | A previous server database gains the three empty tables in one numbered migration, and a repeated startup preserves saved data. | `TestProjectMigration`, `TestProjectStorageConstraints`. |
| 2 | Acceptance commits one operation, one 20-second command and its held receipt. Replay recovers the same result. | `TestProjectOperationAtomicStart`, `TestProjectOperationReplayAfterRestart`, `TestProjectOperationDeadlineAndDisconnect`. |
| 3 | The key stays protected during open and retained terminal states. A replay that wins the retirement race returns the original result. | `TestProjectReceiptRetention`, `TestProjectReplayRetirementRace`. |
| 4 | The authenticated machine, operation and command must agree. A valid report or an eligible outcome finishes an open operation once. | `TestProjectReportIdentityAndFacts`, `TestProjectFolderSettlement`, `TestProjectReportRollback`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 9 | Add held admission, replay and retirement to Store. | `src/server/internal/store/receipts.go` → `Store.WriteOnce`. `src/server/internal/store/prune.go` → `stampReceipts`, `deleteReceipts`. `src/server/internal/store/migrate.go` → `loadMigrations`, `applyMigrations`. |
| 1 | Define the Projects migration and durable records. | Projects `migrate.go` and migration follow `src/server/internal/machines/migrate.go` → `Migrations` and `migrations/0001_machines.sql` → `machines`. |
| 2 | Validate the keyed calls, enqueue work, and read its result outside the write. | New `src/server/internal/projects/operations.go` follows `src/server/internal/machines/commands.go` → `queueConnectedAction`, and `src/server/internal/machines/records.go` → `recordRequest`. `src/server/internal/store/pool.go` → `Store.Read`. |
| 3 | Retire terminal operations together with their held request keys. | New `src/server/internal/projects/retire.go` follows `src/server/internal/store/prune.go` → `stampReceipts`, `deleteReceipts`, `RunPruner`. `src/server/internal/store/receipts.go` → `Store.WriteOnce`. |
| 4 | Apply a folder report or command outcome inside the Link write. | New `src/server/internal/projects/reports.go` follows `src/server/internal/link/inbox.go` → `State.ApplyEvent`, and `src/server/internal/link/outbox_result.go` → `commandOutcome`. |

**Item 9. Store prerequisite for held receipts.** The Server team changes `src/server/internal/store/{receipts.go,prune.go,migrate_test.go}` and adds `src/server/internal/store/{held_receipts_test.go,migrations/0002_held_receipts.sql}`. No predecessor. Run alongside item 1 and land before items 2 and 3. This implements foundation rules 3, 4 and 7, C12 and sprint 4 R5. (logic-r1 06, threat-r1 02, critic-r1 03, architect-r1 02)

Store owns the two methods below and `receipts.held boolean NOT NULL DEFAULT false`. Projects calls the methods. Ordinary pruning skips held rows. Keep the merged migration and ordinary `WriteOnce` contract. Use the same advisory and row locks and fingerprint check. The retained call requires a key. On a miss, run `start` and insert its identifier bytes in a held receipt. On a match, run `replay` under the receipt lock. Retirement checks hold and equal bytes under both locks before calling `remove`. True deletes the receipt in the same write. False means the callback made no change. Keep Store's rollback and uncertain-commit classification.

```go
func (s *Store) WriteOnceRetained(
    ctx context.Context, req Request,
    start func(context.Context, *Tx) ([]byte, error),
    replay func(context.Context, *Tx, []byte) error,
) ([]byte, error)

func (s *Store) RetireReceipt(
    ctx context.Context, caller string, key string, expectedReply []byte,
    remove func(context.Context, *Tx) (bool, error),
) (bool, error)
```

**Tests written first.** In `store/migrate_test.go`, the test writer updates only the Store version expectations. `TestMigrateColdConcurrentStarts` and `TestMigrateConcurrentStarts` now load version 2 from the current manifest. Both assertions in `TestMigrateFailureRollsBack` expect Store version 2 because the first migration already loaded it. `TestMigrateValidatesManifest` also expects version 2 in the current-manifest baseline. Keep the ordering, concurrency, rollback and old-manifest checks.

In `held_receipts_test.go` write three tests.

- `TestHeldReceiptUpgrade` applies the merged Store version 1 file in the fixture, seeds the version 1 ledger and inserts an ordinary receipt using the Machines procedure and fingerprint. Migrate twice. Compare old columns, check `held=false`, replay identical bytes and prune at the ordinary cutoff.
- `TestHeldReceiptSharedKeys` races ordinary and retained procedures under one key in both winning orders. Expect one fresh callback and `ErrRequestKeyConflict` for the other call. The matching retained replay only invokes `replay` with saved bytes.
- `TestHeldReceiptPruneAndRetire` ages ordinary and held receipts. Pruning only stamps and deletes the ordinary row. Retirement with wrong bytes, an ordinary receipt or a declined removal leaves the rows. Fault after operation deletion and require both rows to remain. Retry and require both to disappear.

**Risky.** Bypass the shared key lock, prune a held receipt or split retirement into separate commits. Each corresponding assertion must fail.

**Item 1. Schema and migration.** Add `src/server/internal/projects/{migrate.go,migrations/0001_projects.sql,migrate_test.go}`. Run alongside item 9 and Contract. This implements foundation D2, D4 and D5, rules 1, 3, 4 and 7, and the sprint 4 storage assignment.

| Table | Columns and constraints |
|---|---|
| `repositories` | `id text` primary key, `name text` and `main_branch text` nonempty, `remote text` and `first_commit text` nullable and nonempty when present, and `remote_omitted boolean` required. A withheld remote has a null `remote`. |
| `repository_folders` | `id text` primary key, `repository_id text` required with a restrictive foreign key into `repositories`, `machine_id text` required, and `resolved_path text COLLATE "C"` required and 1 through 8,192 bytes. Index by `repository_id` and by `machine_id`. Within the machine lock, find the exact path before inserting. A machine-only index avoids indexing paths that can exceed PostgreSQL's index entry limit. |
| `folder_operations` | `id text` primary key, `machine_id text` and `request_key text` required, `kind text` with values `inspect` and `add`, `target_repository_id text` nullable for add and absent for inspect, `command_id text` required and unique, `state text` with values `open`, `succeeded` and `failed`, `reply bytea` and `failure text` nullable with state-shape checks, and `retention_started_at timestamptz` nullable and absent while open. Index open rows by machine and terminal rows by retention stamp and identifier. |

Projects allocates the operation, repository and folder identifiers with `store.NewID`. Link allocates the command identifier and persists the exact typed path. See the browser-input storage [decision](attack/round-1-findings.md#owner-decisions). (threat-r1 04) Keep cross-part identifiers without foreign keys so terminal replay survives machine, command and repository removal. The repository-to-folder foreign key is local. Sprint 5 can add a project reference, reservations on the stable folder row and a project check to the orphan rule.

**Tests written first.** In `migrate_test.go`, `TestProjectMigration` checks the three empty tables and Projects version after upgrade and saved rows after rerun. `TestProjectStorageConstraints` rejects an orphan folder, duplicate command identifier, missing or dual terminal results and retention stamp on an open row. Accept a valid operation and a noncompressible 8,192-byte path. **Risky.** Drop the foreign key or state check and require its assertion to fail.

**Item 2. Short request write and durable wait.** Add `src/server/internal/projects/{service,operations,errors,operations_test}.go`. Requires items 1 and 9 and Link items 1 and 7. Item 3 uses its replay path. This implements foundation D1, D5, rules 3, 4, 6 to 8, and R1, R2, R5, R7 and R8. (threat-r1 05, critic-r1 05, architect-r1 05)

- Provide `projects.NewService(db *store.Store, sessions *auth.Sessions, registry *machines.Registry, commands CommandLink, follower *store.Follower, shutdown context.Context) *Service`. Define `CommandLink` with `EnqueueCommand(context.Context, *store.Tx, string, *linkv1.Command, time.Duration) (string, error)` and the existing `Presence(context.Context, machines.Querier, []string) (map[string]machines.Presence, error)`. The Link plan supplies the former. Implement the generated `InspectFolder` and `AddRepository` methods.
- Require one nonempty `Lgtm-Request-Key` of at most 256 bytes, with valid UTF-8 and no zero byte. Validate machine and present target UUIDs. Permit an absolute typed path, `~` or `~/…`, with valid UTF-8, no zero byte and at most 4,096 bytes. Preserve all path bytes. Validate before admission. For caller `owner`, frame machine, exact path and optional target under fingerprint version 1. Omitted Add target gives an empty third field. Store checks procedure and caller-wide key conflicts.
- Use `Store.WriteOnceRetained`. Fresh requests take `Registry.Lock` before other domain locks, after receipt admission. Unknown machines get the fixed unknown-machine reply. Unapproved machines or machines without held presence get unavailable. Link applies its run gate independently. In that write allocate the operation, enqueue `Command_LookAtFolder` with its identifier and path, and insert the open operation with the returned command identifier. The receipt holds only operation identifier bytes. Unavailable and rolled-back starts leave the key available.
- Read the operation in the retained replay callback and copy any terminal reply while holding the receipt lock. Treat a fresh insert as open. After commit, read immediately and poll at 250 milliseconds if open, with no connection held between reads. End waiting at 25 seconds or earlier caller cancellation or deadline. An empty wait returns the fixed deadline and leaves the row open. A failed or uncertain commit returns the fixed database error and allows the same-key retry.
- Store and Link own the receipt and command ledgers. Use the [contract's refusal table](../go-s4-contract/plan.md) and existing fixed infrastructure errors. Exclude typed paths, client text, cookies, request keys and database diagnostics from Projects errors and operation logs. See decision 26.

**Tests written first.** `operations_test.go` → `TestProjectOperationAtomicStart`. Use real Store and Link. Check one operation, one command with exact path and 20-second expiry, and a receipt with only the operation identifier. A fault after enqueue rolls all rows and the wake back and frees the key. Race one key and expect one command. Hold the first of two distinct-key machine writes and observe the second blocked, then check queue order. Cover invalid identifiers, path and header boundaries, duplicate headers, and a changed path, procedure or target.
`TestProjectOperationReplayAfterRestart` reconstructs Store and Projects on the same database and completes the original row with fixture reply bytes. Retry offline and check the original identifiers and bytes with one command. `TestProjectOperationDeadlineAndDisconnect` checks the 25-second deadline on an open row, an independent write completing before it and an early cancellation that preserves replay. Write a terminal refusal using the fixture and check retry. A fresh unavailable request creates no rows. **Risky.** Check availability first, wait in the write or enqueue separately and require the matching failure. Item 4 and paired tests prove real delivery. (threat-r1 05, critic-r1 05, architect-r1 05)

**Item 3. Receipt retention and retirement.** Add `src/server/internal/projects/{retire,retire_test}.go`. Requires 1, 2 and 9. Run with item 4 using real operation fixtures and item 2's replay path. This implements foundation rules 3, 4 and 7, R5 and the contract's retirement rule. (critic-r1 05, architect-r1 05)

Stamp terminal rows when a later write first sees settlement committed. Run a bounded startup sweep and then hourly under `RunRetirement(ctx context.Context, report func(error))`. Page unstamped rows by identifier and mature rows by timestamp and identifier. Honor shutdown. Select candidates without a row lock after stamping. Use the database clock and a 24-hour cutoff. For each candidate call `Store.RetireReceipt` with caller `owner`, key and expected identifier bytes. Under the receipt lock, delete the matching still-mature terminal row. Report a fixed invariant diagnostic if a still-present operation has lost its held receipt.

Replay first captures the terminal result under the lock or sees an open row that cannot age out during the wait. Retirement first leaves a receipt miss and a fresh request. Settlement takes no receipt lock. An open operation can outlive ordinary pruning during Link recovery.

**Tests written first.** In `retire_test.go`, `TestProjectReceiptRetention` ages an open row, a recently settled old request and a terminal operation within its retained day. Same-key retries find the original command and operation. Changed path, procedure or a Machines action gives the fixed conflict. After the cutoff both rows retire and unrelated receipts follow ordinary pruning.

`TestProjectReplayRetirementRace` pauses the first operation read inside matching replay with a test query tracer. Observe retirement blocked on the key. Release replay and compare the original bytes even if both rows are gone by reply time. Reverse the lock order and expect fresh offline refusal, then one new online operation and command. Race two retirement workers and require a single delete. Fault retirement and require both rows to survive. **Risky.** Read after unlocking, admit holds to ordinary pruning or split the deletes. Require the matching assertion to fail.

**Item 4. Transactional folder reports and outcomes.** Add `src/server/internal/projects/{reports,origins,reports_test}.go`. Requires 1, 2 and Link items 1 and 5 to 7. Run with item 3. This implements foundation rules 3, 4 and 6 to 10 and R1, R2, R5 to R7, R10 and R12. (threat-r1 05, architect-r1 05)

- Projects supplies `ApplyFolderSeen(context.Context, *store.Tx, string, *linkv1.FolderSeen) error` and `SettleCommand(context.Context, *store.Tx, string, string, string) error`. Item 8 and the signed fixture adapt Link's `(*link.State).RegisterPart(link.PartHooks) error`. `ReportApplied[link.ReportFolderSeen]` takes `func(context.Context, *store.Tx, string, *linkv1.Event) error` and passes the authenticated machine and `event.GetFolderSeen()`. `CommandSettled[link.CommandLookAtFolder]` takes `func(context.Context, *store.Tx, link.Settlement) error` and passes `MachineID`, `Command.GetId()` and `string(Outcome)`. Link invokes both under the machine lock. The event applies or consumes the report before updating its mark and settling due results. Settlement also runs in the eligible result, timeout or removal write. A callback storage error rolls back that write. (logic-r1 01, threat-r1 01, critic-r1 01, architect-r1 01)
- Update only an open row matching authenticated machine, operation and command. Consume ineffective reports without changing it. Translate valid facts to `lgtm.v1.FolderFacts`. Check field presence, nonempty text, zero bytes, an absolute 8,192-byte resolved path limit and final component as name, and the 2,048-byte remote limit. Apply item 5's origin grammar with `normalizeOrigin(raw string) (string, bool)` in Projects and rely on Link's whole-event limit. Store fixed refusal keys `not_a_folder`, `not_a_git_repository`, `not_readable` and `path_not_usable`. (logic-r1 07, critic-r1 06, architect-r1 07)
- Persist Protocol Buffers reply bytes or a fixed failure key. Supply item 5's `attachRepository` seam in `reports.go` with a fixed domain error that rolls back a valid Add report during the local red run. The report handler and tests call it. Item 5 replaces the stub before mounting and persists the whole Add reply in the attachment write. (critic-r1 05, architect-r1 05)

| Link outcome without a prior valid report | Stored failure key | Connect code and sentence |
|---|---|---|
| `done`, `failed` | `no_answer` | `deadline_exceeded`. The machine did not answer the folder request. |
| `expired`, `unsupported` | `not_taken` | `deadline_exceeded`. The machine did not take the folder command. |
| `interrupted`, `timed_out` | `unknown` | `deadline_exceeded`. The outcome of the folder request is unknown. |
| `removed` | `machine_removed` | `not_found`. The machine was removed. |

**Tests written first.** Register the real Link routes in the signed fixture. In `reports_test.go`, `TestProjectReportIdentityAndFacts` sends crossed identifiers and invalid, duplicate and late reports. An eligible ineffective report advances the mark without changing the row. Include empty optional values, a credential-bearing origin, a contradictory omission flag and a present Secure Shell (SSH) URL whose first path component is a home marker, as in `/~/`. Accept absolute shorthand, known absence and withheld remote. Check precise Inspect facts for dirty, non-main and detached checkouts and the path and remote byte boundaries. Inspect writes no membership. Valid Add facts hit the temporary rollback stub.
`TestProjectFolderSettlement` checks the outcome table, four report refusals and mismatched machine. Stage a result, reopen and replay. The report wins. In the opposite order only the eligible late mark and acknowledgement advance. `TestProjectReportRollback` uses Link's `link.Seams.BeforeEventCommit func(context.Context, string, uint64) error` between the callback and mark. Expect no committed report effect or acknowledgement, then one application on retry. Link supplies the seam and both fixture adapters. **Risky.** Drop an identity match, allow terminal overwrite or settle before the report. Require the corresponding assertion to fail. (logic-r1 01 and 07, threat-r1 05, critic-r1 06, architect-r1 01 and 07)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A retry sends a second command when the first call or server stopped. | High | Persist command, operation and receipt together. Read the receipt before availability. |
| Retirement removes the result between key lookup and its first read. | High | Serialize replay admission and retirement on the existing receipt lock. Capture the terminal response under that lock. |
| A failed or late result overwrites facts that the machine already sent. | High | Apply the authenticated report under the machine lock and only transition an open row. |

**Verify** Run `bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/store ./src/server/internal/projects -run '^Test(HeldReceipt|Migrate|Prune|WriteOnce|RecordValidationAndLimit|Project(Migration|Storage|Operation|Receipt|Replay|Report|FolderSettlement))' -count=1`. Require explicit passes for the named cases with zero skips. Use the same PostgreSQL database across fixture restarts and use signed events for the item 4 gate. The paired gate verifies client execution and expiry.

**WS-2. Saved folders follow the machines that hold them**

Use the resolved checkout to save the same identity once. Serialize changes to the membership of a repository across machines.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 5 | Aliases give one stored folder and both calls return it. A second machine attaches only under the identity rule. Unknown explicit targets use the same refusal for saved and unsaved paths. | `TestProjectAttachmentAliases`, `TestProjectAttachmentIdentity`, `TestProjectAttachmentRollback`. |
| 6 | Removal fails open operations, removes the machine's folders, and projects the remaining repository membership. Two concurrent final removals emit one orphan removal. An attachment racing final removal either stays attached or gets the fixed missing-target refusal. | `TestProjectMachineRemoval`, `TestProjectConcurrentOrphanRemoval`, `TestProjectAttachAgainstRemoval`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 5 | Resolve ownership, verify origin identity and save the folder and repository. | New `src/server/internal/projects/repositories.go` follows `src/server/internal/machines/store.go` → `Registry.Lock`, `recordMachineChange`. Use `src/server/internal/projects/origins.go` → `normalizeOrigin` from item 4 and the reviewed contract's folder identity rules. |
| 6 | Fail open operations and remove folders in the machine removal write. | New `src/server/internal/projects/removal.go` follows `src/server/internal/machines/records.go` → `Service.RemoveClient`, and `src/server/internal/link/presence.go` → `State.Forget`. |

**Item 5. Attachment and repository identity.** Add `src/server/internal/projects/{repositories,repositories_test}.go` and replace the stub in `reports.go`. Requires item 4. Item 7 can prepare seeded tests and runs with this item's projection. This implements foundation D5, rules 3, 4 and 7 to 10, and R2 to R4.

- Projects supplies `attachRepository(ctx context.Context, tx *store.Tx, machineID string, targetID *string, facts *webv1.FolderFacts) (*webv1.AddRepositoryResponse, error)` to the report handler. It stores fixed refusals and rolls back storage errors. It supplies `loadRepository(ctx context.Context, q machines.Querier, id string) (*webv1.Repository, bool, error)` to reports and watch. Load each projection in one join with stable folder order.
- Under the machine lock, look up the exact resolved path. Lock an explicit target with `FOR UPDATE` before checking ownership. A missing target returns the contract's `not_found` even if the path is saved. An existing folder with omitted or matching target returns its identity and current repository, even if the reported origin changed. A different known target gets the fixed `already_exists`. The machine lock keeps an existing folder stable while its repository is read.
- A new folder without a target creates a distinct repository using the reported name, main branch, sanitized remote and first commit. A new folder with a target needs equal normalized present origins, or equal present first commits if at least one origin is known absent. Withholding gives no evidence of absence. Distinct present origins always refuse. Preserve the target's metadata. Store `repository_not_found`, `folder_owned_elsewhere` or `repository_mismatch` for attachment refusals using the reviewed table. Distinct valid paths on one machine may both attach. Sprint 5 selection is an [owner decision](attack/round-1-findings.md#owner-decisions). (logic-r1 04)
- Normalize network Uniform Resource Locators (URLs) for `http`, `https`, `ssh` and `git` and independently absolute SSH shorthand. Fold scheme and host case and remove default ports. Keep transport, nondefault ports, path case, suffix and trailing slash significant. Reject account-relative origins including `/~/`, local paths, user information, queries, fragments, escaped paths and dot segments. Use the standard library and a small shorthand parser without lookups.
- Record one `store.ChangeRef{Topic: "repositories", Scope: "", Key: repositoryID, Change: store.Saved}` per insertion. Reuse emits nothing. **UNREVIEWED MECHANISM** (logic-r1 02, threat-r1 03, critic-r1 02, architect-r1 03). `repositories.go` defines `repositoryRefreshKey = "refresh"` for removal and watch.

**Tests written first.** In `repositories_test.go`, `TestProjectAttachmentAliases` races two reports for one resolved path. Expect one folder and feed save. Replay with a changed valid origin. Verify the same path on another machine and a second eligible path on the first machine both have distinct folder identifiers.

`TestProjectAttachmentIdentity` checks normalized absolute origins between differently named checkouts, SSH shorthand, port and case rules, and paths with significant suffix or trailing slash. Check distinct present origins with equal commits, known absence, missing commits, withheld with present or known-absent origins, and two withheld account-relative origins from different accounts with dissimilar commits. Require the fixed refusals and zero saves for mismatches, both missing-target cases and known ownership conflict. (critic-r1 06, architect-r1 07)

`TestProjectAttachmentRollback` stages the result and aborts after report application. Expect no membership, terminal reply, feed, mark or acknowledgement. Retry and see one save. **Risky.** Remove exact-path lookup, ignore distinct present origins, count withholding as absence or check ownership before a missing target. Require each affected assertion to fail.

**Item 6. Machine removal and repository locking.** Add `src/server/internal/projects/{removal,removal_test}.go`. Requires 4, 5 and Link item 6. Run with 7. This implements foundation D5 and D13, rules 3, 4, 7 and 10, and R12.

- Projects supplies `(*Service).RemoveMachine(context.Context, *store.Tx, string) error` to startup for Link's `MachineRemoved`. Link invokes it in `Forget` after command settlement and before Link and connection deletion, even with no pending command. Machines calls `Forget` in the locked registry write. Fail only open Projects operations with `machine_removed`. (logic-r1 01, threat-r1 01, critic-r1 01, architect-r1 01)
- Lock affected repositories in ascending order after any operation locks. Delete the machine's folders in one statement. In the next statement, Projects removal's `deleteOrphanRepositories` deletes the locked repositories with no remaining folder and returns their identifiers for publication. Sprint 5 can add a project check there. Commit domain and feed together. Disconnect retains folders. **UNREVIEWED MECHANISM** (logic-r1 02, threat-r1 03, critic-r1 02, architect-r1 03).
- **UNREVIEWED MECHANISM** (logic-r1 02, threat-r1 03, critic-r1 02, architect-r1 03). Through `store.MaxWriteChanges - 2` affected repositories, record individual changes. Leave room for the Machines reference and one spare. Above that threshold, record one `store.Saved` for topic `repositories`, empty scope and `repositoryRefreshKey`. This key cannot be a repository identifier. Removal writes it and watch reads it.
- **Tests written first.** `removal_test.go` → `TestProjectMachineRemoval`. Remove an orphan and a shared folder. Fail the open operation without a pending command and preserve terminal replay. Verify the saved survivor and removed orphan. Run removals at 254, 255 and 256 repositories with a pending command. Check the ordinary references through 254 and one refresh above it, with at most 256 total references. Fault cleanup and verify every domain, Link and feed row rolls back. Item 8 observes the reopen.
- `TestProjectConcurrentOrphanRemoval` uses a PostgreSQL trigger and advisory gate after the repository lock at folder deletion. Observe the second remover blocked, then one orphan reference and no folder or repository. `TestProjectAttachAgainstRemoval` gates at folder insert and delete to exercise both lock orders. Attachment first survives. Removal first yields fixed missing-target and zero saves. **Risky.** Drop the lock or callback, count before locking or record individual changes past the cap. Require the corresponding failure.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Two machines both conclude that another folder remains, leaving an orphan. | High | Serialize repository membership changes with a row lock and count after acquiring it. |
| An equal first commit masks a different configured origin. | High | Preserve presence and withholding, apply the fixed precedence, and test both sides of each identity branch. |
| A retry emits a duplicate repository change or exposes half of a mutation. | Medium | Emit a feed reference in the same Store write only for persisted topology changes. |

**Verify** Run `bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/projects -run '^TestProject(Attachment|MachineRemoval|ConcurrentOrphanRemoval|AttachAgainstRemoval)' -count=1`. Require an explicit pass for all six named cases with zero skips. Use PostgreSQL barriers and blocked-transaction observation for both lock orderings. The paired-program check and deployed R14 provide environment coverage for actual path resolution.

**WS-3. The website can discover and follow saved repositories**

Mount the generated handlers behind the same web guards that protect Machines. Use the shared watch to send a complete repository view in each saved message.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 7 | The watch starts with a positioned snapshot and sends full saved repositories or removed identifiers. It ends on expiry or lag, and a reopened watch gets a replacement snapshot. | `TestProjectWatchSnapshotAndChanges`, `TestProjectWatchSessionExpiry`, `TestProjectWatchLagAndReopen`. |
| 8 | All three routes are guarded and discovery matches mounted handlers. The assembled server publishes committed changes and refreshes after a large removal. | `TestProjectRouterPreflight`, `TestProjectRouterMalformedJSON`, `TestProjectRouterDiscovery`, `TestProjectRouterReadLimit`, `TestProjectStartupAndFixedRefusals`, `TestProjectStartupRepositoryWatch`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 7 | Implement the authenticated shared repository watch. | New `src/server/internal/projects/watch.go` follows `src/server/internal/machines/watch.go` → `Service.WatchClients`, `watchResponse`, and `src/server/internal/watch/watch.go` → `Serve`. |
| 8 | Wire dispatch, preflight, discovery and startup. | `src/server/internal/api/server.go` → `Options`, `webProcedures`, `NewHandler`. `src/server/internal/api/session_decode.go` → `redactingClientService`. `src/server/internal/api/session.go` → `sessionService.GetServer`. `src/server/internal/app/start.go` → `startup.run`. |

**Item 7. Authenticated repository watch.** Create `src/server/internal/projects/{watch.go,watch_test.go}`. Requires items 1, 2 and 5 for schema, constructor and projection. Prepare its seeded tests alongside items 3 to 5 and run them independently of item 6. The live mutations run in item 8. (critic-r1 05, architect-r1 05) This implements foundation D5, rules 3 through 5 and 8, and R2 through R4 and R12.

Use `watch.Serve` with the process follower, topic `repositories` and empty scope. `listRepositories(context.Context, machines.Querier) ([]*webv1.Repository, error)` builds the snapshot in one ordered join at its feed position. Use `loadRepository` for saved records. Reuse `auth.Sessions.CheckInRead`, Machines terminal mapping, shutdown and `transport.Send` deadlines. Emit positioned `all`, `saved` and `removed_id` frames. The empty snapshot uses `all`.

**UNREVIEWED MECHANISM** (logic-r1 02, threat-r1 03, critic-r1 02, architect-r1 03). The Projects saved-row loader returns `store.ErrWatchLagged` for `repositoryRefreshKey`. Shared watch forwards the error. Projects sends `aborted` with “The watch fell behind and must be reopened.” The normal reopen gives a replacement snapshot.

**Tests written first.** In `watch_test.go`, `TestProjectWatchSnapshotAndChanges` seeds rows and feed, then commits an independent change after the snapshot head read. Expect the earlier snapshot, a positioned full saved record and a removed identifier. Other topics and scopes produce no frame. `TestProjectWatchSessionExpiry` checks fixed closure and no further data. `TestProjectWatchLagAndReopen` holds a cancellable saved send and commits batches of at most `store.MaxWriteChanges`. Count the held reference. A second subscriber sees reference 2,047 while the held one stays open. Reference 2,048 yields fixed `aborted` and reopening gives the complete positioned snapshot. **Risky.** Remove authorization, separate the snapshot and head or return partial records. Require the matching failure. (logic-r1 03, threat-r1 06, critic-r1 04 and 05, architect-r1 04 and 05)

**Item 8. Guarded router and startup.** Change `src/server/internal/api/{server,session,session_decode}.go` and `src/server/internal/app/start.go`. Create `src/server/internal/api/projects_test.go` and `src/server/internal/app/projects_test.go`. Startup requires items 1 through 7 and 9, and Link item 8's startup and fixture handoff. The router tests and dispatch code can be prepared with generated stubs while the Projects logic is built. Integrate startup only after the Link writer has landed or handed over `startup.run`. This implements foundation D1 and rules 1, 2, 5, 8 through 10, preserves C3 and C12, and covers R12 through R14.

- Accept `api.Options.Projects lgtmv1connect.ProjectServiceHandler`. Mount `lgtmv1connect.NewProjectServiceHandler` with the existing protocol requirement, session request gate, malformed JavaScript Object Notation (JSON) codec and `transport.WithWriteDeadline`. Use `connect.WithReadMaxBytes(32 << 10)` on all three routes before message decoding, using the same size as Session. The cap covers the worst-case six JSON bytes for each permitted path byte plus the fixed fields. Apply it after the existing origin and session gates and to decompressed messages. It is set by the router and observed by its tests. (threat-r1 07) Use `redactingProjectService` to reject a marked first request in `WatchRepositories`, mirroring the Clients wrapper. Preserve binary Protocol Buffers behavior. Register `lgtmv1connect.ProjectServiceWatchRepositoriesProcedure`, `lgtmv1connect.ProjectServiceInspectFolderProcedure` and `lgtmv1connect.ProjectServiceAddRepositoryProcedure` in dispatch and `webProcedures`. The existing origin gate and documented preflight handling apply to these exact paths.
- `api.NewHandler` adds `lgtm.v1.ProjectService` once to `GetServer` when mounted. Preserve existing Session behavior and `api.ContractRevision=1`. Absent handlers remain unimplemented behind authentication.
- In `startup.run`, migrate Projects before readiness and construct the service from the existing Store, sessions, registry, Link state, follower and shutdown context. App writes `registerProjectHooks(state *link.State, service *projects.Service) error` with the item 4 adapters and item 6 binding. Return the `state.RegisterPart` error before workers or listener start. App tests call that same helper on a duplicate route. Supervise `RunRetirement` with the existing lifetime and mount the handler. Use `servertest.Start` outside app and keep the signed-in cookie jar. Use a fixture client timeout above 25 seconds for the full wait. (logic-r1 01, threat-r1 01, critic-r1 01, architect-r1 01)

**Tests written first.** In `api/projects_test.go`, `TestProjectRouterPreflight` uses generated stubs and checks the three preflights with listed and unlisted origins, including the request-key header on unary procedures. Expect 204 with credentials for the listed origin and 403 for the unlisted one. Check signed-in calls and denial before handler invocation with missing credentials or an unlisted origin. `TestProjectRouterMalformedJSON` checks the exact fixed `invalid_argument` and “The request body is invalid.” for both unary procedures and the first framed watch request. Check missing-session precedence and a valid binary watch containing an unknown field. `TestProjectRouterDiscovery` checks mounted and absent handlers in public and signed-in discovery, the unchanged revision and the authenticated unimplemented fallback. `TestProjectRouterReadLimit` expects `resource_exhausted` on plain and compressed over-limit messages before dispatch. Send a heavily escaped valid 4,096-byte path and require the handler to receive the exact path. Denied origin and missing session must still win over a large body. (threat-r1 07)
In `app/projects_test.go`, `TestProjectStartupAndFixedRefusals` checks migration, discovery and the reviewed fixed refusals through the real application. Use the real Link routes to apply a report and separately settle an open operation without a report. Use `RemoveClient` to dispatch removal even when no commands are pending. Pre-register an overlapping report in an independent State and check that `registerProjectHooks` returns the error from Link. Verify malformed and over-limit requests leave the operation and command counts unchanged. Restart and verify a current snapshot. `TestProjectStartupRepositoryWatch` exercises real attachment and ordinary removal, with matching reply and full watch identifiers. An open watch on a 256-repository removal receives the fixed `aborted` reply from the refresh marker. Reopen after commit and observe the removed machine's folders and orphans gone, the other machine's folders retained and the current position. A rolled-back removal publishes no marker. **Risky.** Omit a preflight, origin or session check, redaction, the read cap or the write deadline. The corresponding router or held-stream assertion must fail. (critic-r1 01 and 05, architect-r1 01 and 05, threat-r1 07)

**Design artifact.** No screen work. Website owns the approved screen changes.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Preflight or discovery disagrees with mounted routes. | High | Check the same generated procedures in dispatch, preflight and discovery. |
| Bad requests leak body text or a revoked session receives more data. | High | Verify fixed errors, the read limit and live watch closure. |

**Verify** Use `bash deploy/test/with-postgres.sh -- go test -v` for the separate local gates `./src/server/internal/api -run '^TestProjectRouter' -count=1`, `./src/server/internal/projects -run '^TestProjectWatch' -count=1` and `./src/server/internal/app -run '^TestProjectStartup' -count=1`. Require an explicit pass for every named case with zero skips. Use an actual server request writer to observe the streaming deadline and PostgreSQL for the session and app fixtures.

**Rollout detail.** Compile Link, Machines and Projects together against the published interface before freezing the assembled fixture. Run `bash deploy/test/with-postgres.sh -- go test -v ./src/server/... -count=1` for the combined Server gate. The orchestrator runs `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^TestSprintFour' -count=1` and requires all five reviewed cases and their named subcases to pass without skips. Stack runs `deploy/test/checks-s4.sh` for R1, R2, R4, the revision-refusal half of R11, R12 and R13. The owner performs R14 on the deployed stack from both browser entry points and reloads the tree. (logic-r1 05, architect-r1 06)

**Paired-program acceptance.** The orchestrator writes the following subcases under the five reviewed names in `src/linktest/`, using `servertest.Start`, `clienttest.Join` and `clienttest.Run`. Projects supplies the requests and persisted observations. Link and Client Folders supply deterministic interruption barriers and an inspection counter or equivalent observation. Keep the five existing cases in the [contract plan](../go-s4-contract/plan.md).

| Paired test | Added or confirmed subcase and observation |
|---|---|
| `TestSprintFourHistoryRecovery` | Keep history warning, recovery and removal. Removal gives an open operation the fixed removal refusal. |
| `TestSprintFourPendingRun` | `R8/CommitOrder` and `R9/RunGate`. Queue two real Projects calls with the first write held under the machine lock. Occupy three send permits during replay. Require old events to apply before claim or cursor advance. After both gates open, the client receives both identifiers in commit order. Repeat the run gate with no old events. |
| `TestSprintFourWaitingReplay` | Keep retained replay before approval and stream fencing. A replaced, removed or other machine cannot finish the target operation. |
| `TestSprintFourReceiptReplay` | `R5/ServerRestart` holds an accepted call and its report across server restart. The same key returns the original operation, command and result with one real inspection. `R6/InterruptedAfterAppend` interrupts the client after durable append. The replayed report wins over the interrupted result. `R11/LegacyReceiptAndBuffer` applies the old event once and answers from the legacy receipt with watermark zero and no reexecution. |
| `TestSprintFourFolderSettlement` | `R3/SimultaneousAliases` starts the real link-path and subfolder Adds together. Both return one saved folder. `R7/DisconnectBeforeDelivery` holds delivery and disconnects after acceptance. After 25 seconds the call returns the fixed deadline with the operation still open. After expiry the reconnected client declines, the row becomes failed and the inspection count stays zero. `R10/StagedAcrossRestart` proves no resend or sweep while staged and one settlement after replay. Keep rollback and the late-event acknowledgement assertions. |

**Post-build checks.** Use both real programs and PostgreSQL, and capture the persisted rows, replies and feed or event positions involved in each case.

| Check and owner | Scenario and expected observation |
|---|---|
| Restart and removal. Projects, Link and orchestrator. | Run keyed Add and Inspect through server restart while waiting and client interruption after durable append. Race another machine's attachment against removal before and after report commit. Require one inspection per executed command and one terminal answer. Report first gives unchanged reply bytes. Removal first gives open operations the fixed removal refusal. Inspect changes no membership. The repository follows the winning membership write. Interrupt inside the report write to verify rollback and no acknowledgement. (logic-r1 P1 and P2, threat-r1 P2 and P3, critic-r1 P1, architect-r1 P2 and P4) |
| Uncertain commit. Store and Projects. | Lose acceptance and retirement commit replies after both committed and rolled-back writes. Reopen the same database. Committed acceptance replays the original identifiers and result with one inspection. Rolled-back acceptance allows one fresh admission. Retirement leaves both rows or neither. Pause admitted replay before its first read and verify both retirement lock orders. (logic-r1 P3, threat-r1 P3, architect-r1 P1) |
| Upgrade. Store, Projects and Stack. | Interrupt migration of a version 1 database with an ordinary Machines receipt and live session. The schema and receipt remain intact. Retry the migration and observe mounted discovery and unchanged receipt replay through its existing cutoff. An old manifest fails against the upgraded database. (critic-r1 P2, architect-r1 P3) |
| Watch and guards. Projects and Website. | Race a queued saved reference and earlier Add reply against repository removal. Observe a removed identifier or a replacement snapshot, and no restored folder in the browser. Large removal commits in one write and reopens to a complete snapshot. Deny unlisted origins and invalid sessions on all three routes. Sign-out or expiry ends a live watch before the next change is sent. (logic-r1 P4, threat-r1 P1 and P3) |
| Filtering and private state. Client Folders, Link, Projects and Stack. | Plant synthetic credentials in client-derived path, branch and origin facts. Expect the reviewed refusal or omission and absence from logs, events, records and browser replies. Also verify exact keyed replay for a safe typed path with significant spaces. Measure presence in the pending and settled command separately when the browser types the synthetic value. The register records the open decision on that boundary. Stack keeps its off-loopback certificate, private-hop and state permission checks. (threat-r1 P4, critic-r1 P3, architect-r1 P4) |

Restoring the database to an earlier point is outside wave 1 and sprint 4. A restart in this plan opens the same database with its committed history.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is the six task deliverables and their cross-plan instructions, foundation decisions D1, D2, D4, D5 and D13, foundation rules 1 through 11 and checks C1 through C13, the approved sprint 4 sections and R1 through R14, the sprint 5 compatibility boundary, the reviewed contract's Projects, Store, paired and website checks, both round-three design closing lists, the four round 1 plan reports, issues 1632 through 1635, and Build Loop Phase 4.

| Source item | Workstream | Status |
|---|---|---|
| Task 1. Service, gates, key and discovery. | WS-1, WS-3 | Items 2, 7 and 8. |
| Task 2 and 3. Tables and durable folder operations. | WS-1 | Items 1 through 4 and Store prerequisite 9. |
| Task 4 and 5. Attach and remove. | WS-2 | Items 5 and 6. |
| Task 6. Repository watch. | WS-3 | Item 7. |
| Foundation D1, D2, D4, D5 and D13. | WS-1, WS-2, WS-3 | Go and PostgreSQL, one feed, a reviewed contract, and explicit machine removal. |
| Foundation rules 1 through 4. | WS-1, WS-2, WS-3 | Part-owned migrations and tables, reviewed bindings, Go interfaces and one Store write. |
| Foundation rules 5 through 7. | WS-1, WS-2, WS-3 | Shared watch, durable commands, state in PostgreSQL. |
| Foundation rules 8 through 11. | WS-1, WS-2, WS-3 | Fixed errors, filtered inputs and logs, real database and Link fixtures, and source credit rules. |
| Foundation C1, C2, C4 through C11 and C13. | WS-1, WS-3 | Stack, sign-in, join, Link and vault checks stay with their owners. |
| Foundation C3 and C12. | WS-1, WS-3 | Exercise the mounted auth guards and preserve the real PostgreSQL and feed-lock proof. |
| Sprint 4 four Link changes and R8 through R11. | WS-1, WS-3 | Items 2, 4 and 8 consume Link's published boundary. The paired and Stack gates cover order, admission, staged settlement and upgrade. |
| Sprint 4 R1, R2, R3 and R4. | WS-1, WS-2 | Inspect fields, absolute stored path, alias identity and origin precedence. |
| Sprint 4 R5, R6 and R7. | WS-1 | Durable keyed retry, report-before-outcome and expiry while the caller stops waiting. |
| Sprint 4 R12. | WS-2, WS-3 | Removal transaction and live watch update. |
| Sprint 4 R13 and R14. | WS-3 | Mounted-only real discovery. The Website and Stack plans run the browser checks. |
| Sprint 5 boundary. | WS-1, WS-2 | Stable identifiers, reusable operation plumbing and the future project reference and orphan guard. See decisions 3 and 15. |
| Contract. Receipt retention, replay race, deletion races and origin precedence. | WS-1, WS-2 | Items 3 through 6 and 9. |
| Contract. Three generated preflights, fixed JSON and refusal replies, protected watch and discovery. | WS-1, WS-3 | Items 2, 4, 7 and 8. |
| Contract. Paired recovery, receipt, settlement and folder proof. | WS-1, WS-2, WS-3 | Projects observes durable records and hands off exact Link hooks and real-stack acceptance. |
| Round-three design closing lists and the four plan reviews. | WS-1, WS-2, WS-3 | Decisions 12 through 16 and 19 through 27, and the round 1 register. |
| Issues 1632 through 1635 and Build Loop Phase 4. | WS-1, WS-2, WS-3 | Reuse existing watch and locks, hand transport and paired test ownership to existing owners, and size and freeze each test-first unit. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository rule | How this plan satisfies it |
|---|---|
| `AGENTS.md` → The three parts meet through the contract. | Consume the reviewed sprint 4 generated bindings and leave contract changes to Contract. |
| `AGENTS.md` → The new server is Go and legacy code is a reference. | Build in Projects, Store and the Server team's startup and API registration. |
| `AGENTS.md` → Credit copied code. | The build reuses repository patterns and uses the standard library. If the plan changes to use copied code, the orchestrator arranges the required `third_party/` and `NOTICE` changes before it merges. |
| `AGENTS.md` → Record discoveries and reach the base branch through a pull request. | The result carries cross-plan discoveries to the orchestrator, who owns issue recording and integration. |
| `AGENTS.md` → This repository is independent of Herdr Synapse. | Follow the epic's normal base-branch integration path. |
| `AGENTS.md` → Documents use dates, descriptive headings and repository-relative paths. | The dated decision log and the privacy check cover the assignment. |
| Epic team boundary and bounded Projects assignment. | Projects, Store and both registration paths stay under `src/server/`. Link changes remain a handoff. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 9 | backend | `server/backend` | medium | One existing receipt lock spans held replay and retirement. The additive migration has an upgrade test. | Yes | None. |
| 1 | backend | `server/backend` | medium | The part adds three related tables with local integrity and real database probes. | No | Reviewed contract for field semantics. |
| 2 | backend | `server/backend` | medium | Two unary calls use one existing request flow and a bounded read loop. | Yes | 1, 9 and Link items 1 and 7. |
| 3 | backend | `server/backend` | medium | A retention worker uses one Store key boundary and one operation deletion. | Yes | 1, 2 and 9. |
| 4 | backend | `server/backend` | medium | Two transaction callbacks apply one guarded state machine. | Yes | 1, 2 and Link items 1, 5 to 7. |
| 5 | backend | `server/backend` | medium | One report attachment transaction owns origin checks and the resolved folder identity. | No | 4. |
| 6 | backend | `server/backend` | medium | One removal callback locks and updates the affected repositories. | Yes | 4, 5 and Link item 6. |
| 7 | backend | `server/backend` | medium | One new stream reuses the existing follower and session checks. | Yes | 1, 2 and 5. Integrate live changes in 8. |
| 8 | backend | `server/backend` | medium | Existing routing, discovery and startup patterns wire one service. | Yes | 1 through 7, 9 and Link item 8 for startup. Run the router gate with generated stubs. |

OpenAI supplies both planners, the test writers, the builders and the reviewers. The four completed plan review seats give 25 baseline launches with the original eight test writers and eight builders. Allow 28 with the fold planner and one Store test writer and builder. The estimate includes four focused code reviewers. The orchestrator checks remaining allowance before dispatch. (critic-r1 07)

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Round 1 | 28 findings in 11 groups. | The [register](attack/round-1-findings.md) records all dispositions. The plan stays at `attack` for build verification. |

</details>
