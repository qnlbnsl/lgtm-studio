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

A signed-in owner will be able to inspect a checkout on a connected machine, save it as a repository, and follow the saved repository from the website. The server will store the resolved checkout. The owner can attach a checkout on another machine to the same repository.

The server will save the request before waiting for the machine. If the call ends before the answer comes back, a retry can recover the original result. The server will also keep the repository list current when the owner removes a machine. It will preserve any repository that another machine still holds.

The build depends on two server changes. The machine connection must apply a folder report before settling its command. The database must keep the request key until the saved operation can be retired. Tests will force the timing of a retry and of simultaneous changes from two machines before the service is enabled.

## Key decisions

PostgreSQL will hold the operation and its result. If the website loses its reply, the same request key will recover that operation. The server will keep the key while the operation is open and for at least a day after it settles.

Two spellings of the same checkout will return the same saved folder. A repository can have folders on several machines. The server will coordinate additions and removals of those folders so an attachment cannot disappear because of a concurrent machine removal.

Full log → [decisions.md](decisions.md)

## Workstreams

### Folder requests survive a lost reply

The server will save work for the chosen machine and wait briefly for the answer. A delayed report can still finish the saved operation after the call ends. A retry will return the saved outcome even if the machine has since gone offline.

### Saved folders follow the machines that hold them

Adding a checkout returns its saved identity. Attaching a second checkout checks that it belongs to the chosen repository. Removing a machine removes its folders and ends its open operations. Another machine's folder keeps a shared repository visible.

### The website can discover and follow saved repositories

A signed-in website will receive the repository list, then changes to it. The server will advertise the service when its routes are mounted. The existing session and origin checks will protect the new routes.

## Rollout order

1. Land the reviewed contract build and agree on the Store and Link interfaces before integrating Projects. Use the revision 2 installer for machines that will answer folder requests.
2. Apply the new numbered migrations before opening the server listener. Start the retirement worker and mount the protected routes as part of the server start.
3. Run the real database and paired-program checks on the combined build. Check discovery with the website build, then add a repository from both approved entry points on the deployed stack.

---
<!-- ⛔ implementation details and gate evidence below -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

Application programming interface is abbreviated API in file and type names. The abbreviation WS denotes a workstream. Foundation decisions are D1 through D23, foundation rules are rule 1 through rule 11, and foundation checks are C1 through C13. Sprint 4 checks are R1 through R14. Dates and sources are in [decisions.md](decisions.md). A universally unique identifier (UUID) is in canonical lowercase form with version 7 and the standard variant. Text uses Unicode Transformation Format, 8-bit (UTF-8). A kibibyte is 1,024 bytes. Paths in a brace expression enumerate the named files in one directory.

Use one Go module and the reviewed Protocol Buffers bindings. Use `storetest.NewDatabase` and real PostgreSQL for persistence and concurrency tests. The Server suite can use the signed `link/machinetest` fixture for controlled event and outcome delivery. The paired-program and deployment cases use the real client. Freeze tests after a first failure on the named assertion. A minimal stub can preserve compilation, and compilation failure alone supplies no red evidence. Run risky mutations independently on a temporary build branch and restore the code before the integrated pass.

**WS-1. Folder requests survive a lost reply**

Store the intent before waiting. Keep the first definitive answer so a retransmitted key can recover it through a restart.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | A previous server database gains the three empty tables in one numbered migration, and a repeated startup preserves saved data. | `TestProjectMigration`, `TestProjectStorageConstraints`. |
| 2 | An accepted request stores one operation and command with a 20-second expiry, and a receipt containing only the operation identifier. A lost or timed-out caller can recover the operation without a second command. | `TestProjectOperationAtomicStart`, `TestProjectOperationReplayAfterRestart`, `TestProjectOperationDeadlineAndDisconnect`. |
| 3 | The key stays protected during open and retained terminal states. A replay that wins the retirement race returns the original result. | `TestProjectReceiptRetention`, `TestProjectReplayRetirementRace`. |
| 4 | The authenticated machine, operation and command must agree. A valid report or an eligible outcome finishes an open operation once. | `TestProjectReportIdentityAndFacts`, `TestProjectFolderSettlement`, `TestProjectReportRollback`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Define the Projects migration and durable records. | New `src/server/internal/projects/migrate.go` follows `src/server/internal/machines/migrate.go` → `Migrations`. New numbered migration follows `src/server/internal/machines/migrations/0001_machines.sql` → `machines`. |
| 2 | Validate the keyed calls, enqueue work, and read its result outside the write. | New `src/server/internal/projects/operations.go` follows `src/server/internal/machines/commands.go` → `queueConnectedAction`, and `src/server/internal/machines/records.go` → `recordRequest`. `src/server/internal/store/pool.go` → `Store.Read`. |
| 3 | Retire terminal operations together with their held request keys. | New `src/server/internal/projects/retire.go` follows `src/server/internal/store/prune.go` → `stampReceipts`, `deleteReceipts`, `RunPruner`. `src/server/internal/store/receipts.go` → `Store.WriteOnce`. |
| 4 | Apply a folder report or command outcome inside the Link write. | New `src/server/internal/projects/reports.go` follows `src/server/internal/link/inbox.go` → `State.ApplyEvent`, and `src/server/internal/link/outbox_result.go` → `commandOutcome`. |

**Item 1. Schema and migration.** Create `src/server/internal/projects/migrate.go`, `src/server/internal/projects/migrations/0001_projects.sql`, and `src/server/internal/projects/migrate_test.go`. This item implements foundation D2, D4 and D5, rules 1, 3, 4 and 7, and the sprint 4 table assignment. Items 2 and 7 can start after it. The reviewed service bindings may land in parallel with this migration.

| Table | Columns and constraints |
|---|---|
| `repositories` | `id text` primary key, `name text` and `main_branch text` nonempty, `remote text` and `first_commit text` nullable and nonempty when present, and `remote_omitted boolean` required. A withheld remote has a null `remote`. |
| `repository_folders` | `id text` primary key, `repository_id text` required with a restrictive foreign key into `repositories`, `machine_id text` required, and `resolved_path text COLLATE "C"` required and 1 through 8,192 bytes. Index by `repository_id` and by `machine_id`. Within the machine lock, find the exact path before inserting. A machine-only index avoids indexing paths that can exceed PostgreSQL's index entry limit. |
| `folder_operations` | `id text` primary key, `machine_id text` and `request_key text` required, `kind text` with values `inspect` and `add`, `target_repository_id text` nullable for add and absent for inspect, `command_id text` required and unique, `state text` with values `open`, `succeeded` and `failed`, `reply bytea` and `failure text` nullable with state-shape checks, and `retention_started_at timestamptz` nullable and absent while open. Index open rows by machine and terminal rows by retention stamp and identifier. |

Use `store.NewID` for operation, repository and folder identifiers. The Link creates the command identifier. Save the exact typed path only in the stored command. The receipt fingerprint identifies its use, and the operation needs no copy. Use local identifiers for cross-part references. Operations must survive removal of their machine, repository and command, so those references have no cascading deletion or foreign key. Keep the repository-to-folder foreign key inside Projects. The next sprint can add the project-to-repository reference and extend the orphan check while taking the same repository lock. Folder reservations can be added to the stable folder row. Sprint 4 creates precisely the three tables above.

**Tests written first.** In `migrate_test.go`, `TestProjectMigration` migrates a sprint 3 fixture, queries the three empty tables and the recorded Projects version, inserts a legal repository and folder, and reruns migrations with the fixture still present. `TestProjectStorageConstraints` attempts an orphan folder, a duplicate command identifier, a success without a stored result, a failure without a reason, and an open operation with a retention stamp. Each insert is rejected. Accept a valid operation and a synthetic noncompressible 8,192-byte path. The first query for the Projects part fails its assertion before implementation. **Risky.** Remove the folder foreign key or the operation state check and require the respective assertions to fail.

**Item 2. Short request write and durable wait.** Create `src/server/internal/projects/{service,operations,errors}.go` and `src/server/internal/projects/operations_test.go`. Requires item 1 and the Store and Link interfaces in the handoff. Item 3 can proceed alongside item 2 after the shared Store interface is agreed. Item 7 can proceed using the agreed service constructor and dependencies. This implements foundation D1, D5 and rules 3, 4, 6, 7 and 8, and R1, R2, R5, R7 and the Projects contribution to R8.

- Provide `projects.NewService(db *store.Store, sessions *auth.Sessions, registry *machines.Registry, commands CommandLink, follower *store.Follower, shutdown context.Context) *Service`. Define `CommandLink` with `EnqueueCommand(context.Context, *store.Tx, string, *linkv1.Command, time.Duration) (string, error)` and the existing `Presence(context.Context, machines.Querier, []string) (map[string]machines.Presence, error)`. The Link plan supplies the former. Implement the generated `InspectFolder` and `AddRepository` methods.
- Require exactly one nonempty `Lgtm-Request-Key`. Use the existing 256-byte Store text limit and reject invalid UTF-8 and zero bytes. Validate canonical machine and explicit repository UUIDs. A present empty target is invalid. Accept a typed path of at most 4,096 bytes with valid UTF-8 and no zero byte. Accept a leading slash, `~` alone, or `~/`. Preserve the exact path, including spaces that belong to it. Validate syntax before receipt admission. The shared caller is `owner`, as in Machines. Use fingerprint version 1 and its length-framed strings for the machine, exact path, and optional target. Encode absence as an empty third field on Add. Store records the procedure independently. A conflicting admitted key gives the existing Store conflict across Projects and Machines.
- Use `Store.WriteOnceRetained` as requested in the result. In its fresh callback, take `Registry.Lock` as the first domain row lock. The shared receipt advisory lock and the receipt miss precede it. Translate a missing machine to the contract's fixed unknown-machine response. Require approval and the existing held-connection check via `Presence`. A waiting machine or an unheld connection gets the fixed unavailable response. Let Link gate delivery for a run draining its old events. Generate the operation identifier, enqueue a `Command_LookAtFolder` with `operation_id` and exact `path`, and insert the open operation with the returned `command_id` in the same transaction. Return only the identifier bytes for the receipt. Unavailable or rolled-back starts leave the key eligible for a fresh request.
- Read the first state of an existing operation while the Store receipt lock is still held, using a plain query in the replay callback. Capture a copy of a terminal result before releasing the lock. The fresh write can seed the open state from its own insert. After successful commit, wait up to 25 seconds in a caller-bound context with short database reads and a 250-millisecond polling interval. Check the database immediately after an open starting state. Hold neither a write nor a pool connection between reads. A short caller deadline or cancellation stops the wait. A server-side empty wait gives the fixed folder deadline response. At the time limit leave the state untouched. An uncertain commit uses the fixed database response and the caller can retry the same key.
- The Projects code writes no command or machine table directly. Link owns the report watermark, delivery and after-commit wake. Store owns receipt conflicts and database error sanitization. The website gets one fixed sentence for every contract refusal. The reviewed contract plan contains the table headed “Fixed refusals for the website.” [Contract plan](../go-s4-contract/plan.md) Existing infrastructure errors use fixed mappings. Never include typed paths, raw client text, cookie values, request keys or database diagnostics in website errors or operation logs.

**Tests written first.** In `operations_test.go`, `TestProjectOperationAtomicStart` uses real Store and Link. It checks one row in each ledger, exact path bytes in the command, its 20-second interval from the database timestamps and only the identifier in the receipt. Gate an insert fault after enqueue and before commit. Expect none of the operation, command, receipt or wake change to commit. Then remove the fault and reuse the key. Race two same-key requests and require one command. Use two distinct keys with the first write held under the machine lock. Observe the second write blocked, then require both committed commands in order of their writes. Include the 4,096-byte valid multibyte boundary and 4,097 bytes, both home forms, invalid path forms and identifier cases, 256 and 257 key bytes, duplicate header fields, and a changed path or procedure under the same key.
`TestProjectOperationReplayAfterRestart` stops the first server while the call waits, reopens the same database, delivers one report, and retries, including with the machine offline. It compares the stored reply and command identifier and counts one command. `TestProjectOperationDeadlineAndDisconnect` holds delivery, runs an actual 25-second operation wait with a longer request context and a test transport timeout above 25 seconds, and expects the exact folder deadline error with the row open. A separate write to the same machine during the wait must commit before the answer. Then deliver the expiry result after the 20-second expiry and require a terminal decline without a folder inspection. Fresh offline and waiting calls leave the three ledgers empty for their keys. **Risky.** Move availability ahead of replay, move the wait inside the write, or commit enqueue outside the operation transaction. The offline replay, concurrent write or rollback assertion must fail, respectively.

**Item 3. Receipt retention and retirement.** Create `src/server/internal/projects/{retire.go,retire_test.go}`. Requires item 1 and the Store extension. It can run alongside items 2 and 7 after the Store interface is fixed. Its race proof composes with item 2. This implements foundation rules 3, 4 and 7, R5, and the reviewed contract's retirement requirement.

Stamp `retention_started_at` on terminal rows only when a later write can see that settlement committed. Start one worker with `RunRetirement(ctx context.Context, report func(error))`. Run one bounded sweep on startup and then hourly, with keyset pages in timestamp and identifier order. Use the database clock for the 24-hour age check. Entry to the sweep and a long page must honor shutdown. Finish stamping before selecting mature terminal candidates. Read candidates without a row lock. For each, call the requested `Store.RetireReceipt` with caller `owner`, the saved key and expected identifier bytes. Under the Store receipt lock, delete exactly the still-terminal and mature operation in the same transaction that removes its held receipt. A stale candidate cannot delete the receipt of a new use of the key. Report a missing retained receipt for a still-present candidate through a fixed invariant diagnostic. Ignore a stale candidate whose operation has already been retired. Keep ordinary receipt and feed pruning unchanged for other callers.

The first replay read occurs with the receipt protected. Run the read as part of the same receipt transaction and keep its terminal bytes in the request scope. If retirement gets that lock first and commits, a later call sees a receipt miss and starts under fresh rules. Replay acquiring it first captures a terminal response or sees an open row that cannot age out through the 25-second wait. Settlement does not take a receipt lock. An operation may remain open beyond the ordinary daily pruning interval during Link recovery. No wall clock in Projects settles it.

**Tests written first.** In `retire_test.go`, `TestProjectReceiptRetention` advances database fixture timestamps past ordinary Store pruning. It checks an old open operation, a just-settled old request and a terminal operation immediately inside the observation-day cutoff. In those cases the same request finds one command and the original operation. A changed path, different Projects procedure, and a Machines action using the same held key return the fixed conflict. After the full day, a sweep removes both operation and receipt together and preserves unrelated receipts under their existing rules. `TestProjectReplayRetirementRace` uses a fixture barrier at the start of the replay callback, immediately after receipt lookup and before the first result read, with a terminal row that is otherwise mature. Start retirement, observe its database wait, release replay, and compare its old serialized result even if rows are gone by the time the response is sent. Reverse the lock order and assert the fresh offline error, then retry when online and require a new operation identifier and one new command. Run two retirement workers against one candidate and check one deletion. Induce a failed retirement transaction and require both rows to remain. **Risky.** Let ordinary pruning see held rows, read the first replay result after unlocking, or delete the operation in a separate transaction. Require the retention, paused replay or failed-retirement assertions to fail.

**Item 4. Transactional folder report and command outcomes.** Create `src/server/internal/projects/{reports.go,origins.go,reports_test.go}`. Requires items 1 and 2 and the reviewed Link callback boundary. It can start while items 3 and 7 proceed. This implements foundation rules 3, 4, 6 through 10, R1, R2, R5 through R7 and the Projects side of R10 and R12.

- Provide `ApplyFolderSeen(context.Context, *store.Tx, string, *linkv1.FolderSeen) error` and `SettleCommand(context.Context, *store.Tx, string, string, string) error` on `projects.Service`. For settlement the strings are machine identifier, command identifier and outcome. Link holds the machine lock for either callback. For a report, require the authenticated machine, stored command and stored operation to agree on an open row. Unknown, mismatched, malformed and late reports leave operation state unchanged. Tell Link by returning nil so it can consume an eligible event. Return storage failures to roll back report, mark and any staged settlement together.
- Use a guarded terminal update, with `state = 'open'` and the machine and command predicates. Apply the fixed outcomes below. For valid facts, translate field presence into `lgtm.v1.FolderFacts`. Check required strings, optional nonempty strings, remote presence versus withholding, zero bytes, the 8,192-byte resolved path and the 2,048-byte remote. Require an absolute resolved path and its last component as the name. Parse present origins with `normalizeOrigin(raw string) (string, bool)`, following item 5's allowed forms, before accepting either call's facts. Rely on Link's complete message budget, and the Client plan for deriving and filtering local facts. Keep typed path, branch, remote and commit out of error text. An absent report arm, unknown or zero refusal, or invalid facts is consumed without effect. The four known refusal values produce the matching fixed contract responses. Their stored reason keys are `not_a_folder`, `not_a_git_repository`, `not_readable` and `path_not_usable`.
- In `reply`, encode the successful `InspectFolderResponse` or `AddRepositoryResponse` as Protocol Buffers bytes by operation kind. In `failure`, store a fixed reason key and derive the Connect code and one sentence from the reviewed table. Inspect success stores only facts in its operation. Add success and attachment refusals use item 5 inside the same write. Use an implementation seam for the Add path during the initial red run. Materialize and persist the whole add response in the attachment transaction so retries preserve the first response after later folder changes or machine removal.

| Link outcome without a prior valid report | Stored failure key | Connect code and sentence |
|---|---|---|
| `done`, `failed` | `no_answer` | `deadline_exceeded`. The machine did not answer the folder request. |
| `expired`, `unsupported` | `not_taken` | `deadline_exceeded`. The machine did not take the folder command. |
| `interrupted`, `timed_out` | `unknown` | `deadline_exceeded`. The outcome of the folder request is unknown. |
| `removed` | `machine_removed` | `not_found`. The machine was removed. |

**Tests written first.** In `reports_test.go`, `TestProjectReportIdentityAndFacts` sends wrong-machine, wrong-command, wrong-operation, malformed and duplicate reports before and after a valid one. It checks the operation row and exact Inspect response for dirty, alternate checked-out branch, detached, withheld and known-absent fixtures. Check a resolved path of 8,192 bytes and one byte over, and a valid remote of 2,048 bytes and one byte over. Use matching multibyte facts inside the complete event budget. Include empty optional strings, a present withheld remote and a credential-bearing origin among ineffective reports. No repository or folder row is created by inspect. At the Link edge, require the mark to advance for an eligible but ineffective report. `TestProjectFolderSettlement` checks each row of the outcome table, the four fixed folder refusals and both event-result orders. Stage a result above the mark, restart, then apply the report and require the successful response to survive settlement in that write. Settle an outcome first and then send a valid eligible report. Expect only the mark and acknowledgement to advance. `TestProjectReportRollback` drives the Link-provided failure after the callback but before the mark and result commit. Check open operation and unchanged mark, no acknowledgement, rollback of the staged transition, then one successful application on retry. Item 5 extends the same assertion to records and feed. **Risky.** Remove the machine or command match, permit terminal overwrite, or settle before the report. The identity, late-event or report-wins assertion must fail.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A retry sends a second command when the first call or server stopped. | High | Persist command, operation and receipt together. Read the receipt before availability. |
| Retirement removes the result between key lookup and its first read. | High | Serialize replay admission and retirement on the existing receipt lock. Capture the terminal response under that lock. |
| A failed or late result overwrites facts that the machine already sent. | High | Apply the authenticated report under the machine lock and only transition an open row. |

**Verify** Run `bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/projects -run '^TestProject(Migration|Storage|Operation|Receipt|Replay|Report|FolderSettlement)' -count=1`. Require an explicit pass for each named case and zero skips. For environment verification, use one test database across server restart and the real signed Link fixture. The paired client restart and expiry checks run in the orchestrator's combined environment.

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

**Item 5. Attachment, ownership and repository identity.** Create `src/server/internal/projects/{repositories.go,repositories_test.go}`. Change `src/server/internal/projects/reports.go` to call `attachRepository`. Requires item 4. Item 7 can work in its own files against the agreed repository query shape. This implements D5, rules 3, 4, 7 through 10, and R2 through R4.

- Expose the internal seams `attachRepository(ctx context.Context, tx *store.Tx, machineID string, targetID *string, facts *webv1.FolderFacts) (*webv1.AddRepositoryResponse, error)` and `loadRepository(ctx context.Context, q machines.Querier, id string) (*webv1.Repository, bool, error)`. The report handler distinguishes fixed attachment refusals from storage errors. It commits the former as an operation failure and returns the latter for rollback. Use a single joined query per projected repository, with folder identifiers in stable order. The report handler persists the resulting response with the operation in the same write.
- Link's machine lock serializes the exact-path lookup and insert for a machine, including aliases, independently of wall clock. Lock an explicit target's `repositories` row with `FOR UPDATE` before considering the path owner's refusal. A missing target returns `not_found` with “The repository was not found.” for a saved or unsaved path. If the folder already belongs to an omitted or matching target, return the saved identities and current full repository view, even if its reported origin has changed. If it belongs to a different known target, return `already_exists` with “The folder is already saved for another repository.” An omitted target with an existing folder may lock that folder's repository before loading the reply. The machine lock keeps the saved folder stable.
- For a new folder and omitted target, create a repository named from `FolderFacts.name` with its main branch, sanitized remote presence and first commit. Do not deduplicate repositories by name or remote. For a new folder and existing target, accept two matching normalized present origins, or equal present first commits when at least one origin is known absent. A withheld origin supplies no comparison value or evidence of absence. Two present but distinct origins are refused even if their first commits match. A mismatch returns `failed_precondition` with “The folder does not belong to the selected repository.” Store the terminal attachment failures as `repository_not_found`, `folder_owned_elsewhere` or `repository_mismatch`. Preserve an existing repository's metadata during attachment.
- Normalize the contracted network Uniform Resource Locator (URL) forms `http`, `https`, `ssh` and `git` and independently absolute Secure Shell (SSH) shorthand for comparison. Fold scheme and host case and remove each scheme's default port. Keep transport, nondefault port, path case, suffix and trailing slash significant. Treat absolute SSH shorthand like the equivalent SSH URL. Validate credential-free origin text before storage. Reject a report with an invalid present origin, including an account-relative origin, a local path, user information, a query, fragment, escaped path or dot segment. Contracted client reports with a withheld origin retain that fact separately. Use standard library parsing and a small absolute-shorthand parser with no external dependency or lookups.
- Record `store.ChangeRef{Topic: "repositories", Scope: "", Key: repositoryID, Change: store.Saved}` exactly once when a folder is inserted, including creation of its repository. Repeating the saved folder changes no repository record and produces no repository feed row.

**Tests written first.** In `repositories_test.go`, `TestProjectAttachmentAliases` holds one report transaction while a link-path and subfolder request target the same machine and resolved path. Both return one persistent folder and repository identity. Require one saved feed row and an unchanged identity after a report with a changed valid origin. A second machine can keep its own folder even if the path string is identical. `TestProjectAttachmentIdentity` checks different checkout names, same normalized absolute origins across case and default-port differences, absolute SSH shorthand against its URL form, and significant transport, ports, path case, suffix and trailing slash. Check known absence fallback, two missing commits, both withheld, and one withheld with one present. Cover withheld with known absence and equal commits, where the known absence enables the comparison. Require fixed replies for both kinds of unknown target, known target conflict and identity mismatch, with no saved feed entry for refusals. `TestProjectAttachmentRollback` stages an Add result, injects the item 4 Link interruption after a valid report callback, and observes no repository, folder, terminal operation, feed change, mark or acknowledgement. Retry and observe one save. **Risky.** Drop the machine-scoped exact-path check, compare equal commits when two present origins differ, treat withheld as known absent, or take the explicit-target check after the path-owner conflict. The matching scenarios must fail independently.

**Item 6. Machine removal and repository locking.** Create `src/server/internal/projects/{removal.go,removal_test.go}`. Requires items 4 and 5 and the Link removal boundary. It can run alongside item 7. This implements foundation D5 and D13, rules 3, 4, 7 and 10, and R12.

- Provide `RemoveMachine(context.Context, *store.Tx, string) error` on the service. Link invokes it in the removal write, even if it has no command to settle. The machine lock is already held or the machine row was deleted in the same transaction after that lock. Fail that machine's open operations with `machine_removed`. Preserve previously terminal results. An outcome callback for a pending command can run on either side of this transition, since both use an open-state guard. Do not rescan or touch other machines' operation rows.
- Read the affected repository identifiers for that machine. Lock their repository rows in ascending identifier order before deleting folders. Use the same `FOR UPDATE` row lock as attachment. After obtaining the locks, delete only folders of the removed machine. In a separate read-committed statement, test the remaining membership of each locked repository. Publish one saved change for a repository with other folders. Delete a repository with no folders and publish one removed change. Sprint 4 has no project rows. Keep the orphan decision in one named local function, `deleteOrphanRepository`, where sprint 5 can add the project existence check under the same lock. No repository write takes another machine lock. Acquire a Projects operation row when finishing it before repository locks where both are needed. Keep feed positions under Store's final flush.
- The owner-only machine removal is the sole cleanup trigger for this workstream. Transient link loss leaves folders in place. A storage error rolls back cleanup and machine removal together. Receipt retirement takes the receipt lock and then an operation row, without acquiring a machine or repository row.

**Tests written first.** In `removal_test.go`, `TestProjectMachineRemoval` removes a machine with an open operation whose command is absent from the pending set, a terminal operation, a sole folder and a shared repository folder. Expect the operation to fail with the fixed refusal, the terminal replay to stay identical, the sole repository to disappear once, and a saved change carrying exactly the surviving folder of the shared repository. Run an injected storage failure and require the original machine and Projects rows to remain. `TestProjectConcurrentOrphanRemoval` uses two machine rows and a repository with one folder on each. Gate one removal on its folder deletion after it acquires the repository lock. Use a test-only database trigger and advisory barrier. Observe the second blocked on the same repository with PostgreSQL blocking process identifiers, then let both commit. Require one orphan removal feed entry, no repository and no folders at completion. `TestProjectAttachAgainstRemoval` uses the same lock barrier in both orders. If attachment wins, the target and new folder survive. If removal wins, attachment returns the exact `not_found` response and saves nothing. **Risky.** Omit the repository lock from one path or count membership before the lock. Require the blocked-transaction assertion or final state assertion to fail. Remove the cleanup callback and require the open operation with no pending command to stay incorrectly open in the mutation run.

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
| 8 | All three generated routes use origin and session guards and return fixed errors. Discovery advertises Projects only when mounted and remains at revision 1. Startup migrates and wires removals and reports before accepting calls. | `TestProjectRouterPreflight`, `TestProjectRouterMalformedJSON`, `TestProjectRouterDiscovery`, `TestProjectStartupAndFixedRefusals`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 7 | Implement the authenticated shared repository watch. | New `src/server/internal/projects/watch.go` follows `src/server/internal/machines/watch.go` → `Service.WatchClients`, `watchResponse`, and `src/server/internal/watch/watch.go` → `Serve`. |
| 8 | Wire dispatch, preflight, discovery and startup. | `src/server/internal/api/server.go` → `Options`, `webProcedures`, `NewHandler`. `src/server/internal/api/session_decode.go` → `redactingClientService`. `src/server/internal/api/session.go` → `sessionService.GetServer`. `src/server/internal/app/start.go` → `startup.run`. |

**Item 7. Authenticated repository watch.** Create `src/server/internal/projects/{watch.go,watch_test.go}`. Requires item 1 and the service constructor and record projection signatures from items 2 and 5. Build the source alongside items 3 through 6 with those seams. Run the integrated save and remove scenarios after item 6. This implements foundation D5, rules 3 through 5 and 8, and R2 through R4 and R12.

Use `watch.Serve` with topic `repositories` and the empty scope. Give it the existing process follower and read through its `store.ReadTx`. Implement `listRepositories(context.Context, machines.Querier) ([]*webv1.Repository, error)` using one ordered join and stable grouping. Use `loadRepository` for a saved row within the shared watch read. The snapshot and its position share the transaction. Reuse the Machines watch pattern for the session verifier, `auth.Sessions.CheckInRead`, terminal error mapping, server shutdown and `transport.Send` deadlines. Bind cancellation to the server lifetime and give terminal frames the same deadline handling as Machines. Set `FeedPosition` on every frame. Map the three arms to `WatchRepositoriesResponse_All` with `RepositoryList.repositories`, `WatchRepositoriesResponse_Saved` and `WatchRepositoriesResponse_RemovedId`. The initial empty list still uses the snapshot arm. The shared source ignores unrelated feed topics and may read the repository's later committed state when it serves a queued saved change.

**Tests written first.** In `watch_test.go`, `TestProjectWatchSnapshotAndChanges` seeds two machines, opens the stream, and compares the first frame and head position with committed records. Pause after the snapshot transaction reads its head, commit an attachment from another connection, and require it in a later saved frame with a greater position. Add and remove folders, and require full saved records sharing the same stable identifiers as the Add replies. Unrelated changes produce no repository frame. Observe a removed identifier when the last folder is removed. `TestProjectWatchSessionExpiry` expires a real session while the watch waits and checks the fixed unauthenticated closure, with no data afterward. `TestProjectWatchLagAndReopen` holds a stream send using a fixture barrier, commits more than 2,048 feed references in a real Store write, then observes the fixed `aborted` closure. Reopen and require a full replacement list at the new position. **Risky.** Remove `Authorize`, generate the snapshot position in a separate read, or return a partial saved record with just the new folder. Require the live expiry, concurrent snapshot-cut or full-record assertion to fail.

**Item 8. Guarded router and startup.** Change `src/server/internal/api/{server,session,session_decode}.go` and `src/server/internal/app/start.go`. Create `src/server/internal/api/projects_test.go` and `src/server/internal/app/projects_test.go`. Requires items 1 through 7 and Link's agreed registration seam. The router tests and dispatch code can be prepared with generated stubs while the Projects logic is built. Integrate startup only after the Link writer has landed or handed over `startup.run`. This implements foundation D1 and rules 1, 2, 5, 8 through 10, preserves C3 and C12, and covers R12 through R14.

- Accept `api.Options.Projects lgtmv1connect.ProjectServiceHandler`. Mount `lgtmv1connect.NewProjectServiceHandler` with the existing protocol requirement, session request gate, malformed JavaScript Object Notation (JSON) codec and `transport.WithWriteDeadline`. Use `redactingProjectService` to reject a marked first request in `WatchRepositories`, mirroring the Clients wrapper. Preserve binary Protocol Buffers behavior. Register `lgtmv1connect.ProjectServiceWatchRepositoriesProcedure`, `lgtmv1connect.ProjectServiceInspectFolderProcedure` and `lgtmv1connect.ProjectServiceAddRepositoryProcedure` in dispatch and `webProcedures`. The existing origin gate and documented preflight handling apply to these exact paths.
- Derive advertisement from successful registration in `api.NewHandler`. Decorate the Session service's `GetServer` response with `lgtm.v1.ProjectService` exactly once when all three routes are mounted. Preserve other service entries and existing Session behavior. The omitted Projects handler leaves the service unadvertised and calls at its paths unimplemented after authentication. Keep `api.ContractRevision=1`.
- In `startup.run`, include `projects.Migrations()` with the other migrations before readiness. Build the Projects service from the existing store, session manager, machine registry, Link state, follower and shutdown context. Register `ApplyFolderSeen`, `SettleCommand` and `RemoveMachine` with Link before starting its worker or listener. Start one supervised `RunRetirement` worker with the startup lifetime and existing worker drain. Pass the service to the API router. The final startup probe still precedes opening the listener. For tests outside the app package, use `servertest.Start` and construct the generated Projects client from the fixture transport and server address. For the full wait case, copy the fixture client and lengthen its 10-second total timeout. Retain the signed-in cookie jar.

**Tests written first.** In the API package, `TestProjectRouterPreflight` sends browser preflights for the three generated procedure constants with a listed and unlisted synthetic loopback origin. Include the request-key header on both unary paths. Expect 204 with credentials and the requested headers for the listed origin, and 403 for the unlisted one. Follow with a signed-in call for each procedure using both origins, and repeat without a session. A denied call cannot reach its handler. `TestProjectRouterMalformedJSON` sends malformed bodies holding a synthetic marker through those same paths. Test unary JSON and the watch's first framed JSON request. Require `invalid_argument` and exactly “The request body is invalid.” Verify no handler effect and no operation creation through the full app fixture. Check missing-session precedence and that a valid binary watch carrying the marker as an unknown field still decodes. `TestProjectRouterDiscovery` queries signed-in and public discovery with and without a mounted service. Expect one fully qualified entry only when mounted, an unchanged revision and an authenticated `unimplemented` fallback when absent.
In the app package, `TestProjectStartupAndFixedRefusals` starts the real PostgreSQL-backed server through the existing startup fixture and drives all three procedures through the generated client or raw router fixture. Check the exact code and sentence for each row of the reviewed contract table. Use the watch cases in item 7 for lag and expiry. Verify the migration's three tables, report persistence, and removal through `RemoveClient`. Check that malformed requests leave the operation and command counts unchanged. Check that new signed-in discovery returns `sample=false`, `contract=1` and `lgtm.v1.ProjectService`. Restart with saved rows and verify discovery and a current snapshot. **Risky.** Remove one preflight constant at a time, skip the origin or session gate, remove unary or watch JSON redaction, or advertise an absent handler. The matching guarded-router assertions must fail.

**Design artifact.** The sprint 4 server adds no screen or change of hierarchy. The approved website changes are in the sprint 4 design and owned by the Website plan.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A browser can call a procedure that the preflight table missed, or the service is advertised without its routes. | High | Run the same procedure list through dispatch, preflight and discovery tests. Derive advertisement from the mounted routes. |
| A malformed body leaks its contents, or an expired session keeps receiving changes. | High | Reuse redaction and live reauthorization. Verify fixed router output and actual watch closure. |

**Verify** Run `bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/projects ./src/server/internal/api ./src/server/internal/app -run '^TestProject(Watch|Router|Startup)' -count=1`. Require an explicit pass for every named watch, router and startup case with zero skips. Use signed-in calls, an actual server request writer with streaming deadlines, and PostgreSQL for the session and app fixtures. Stack verifies discovery through the deployed web server and the website test matrix.

**Rollout detail.** Step 1 consumes the landed reviewed contract build and the Link revision 2 boundary. Step 2 relies on the existing startup migration runner and verifies the existing database before readiness. At step 3 run `bash deploy/test/with-postgres.sh -- go test -v ./src/server/... -count=1` and require a successful combined Server suite. The orchestrator runs `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^TestSprintFour' -count=1` and requires five explicit passes with zero skips. Stack owns the deployed R14 observation.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is the six task deliverables and its cross-plan instructions, the foundation decisions D1, D2, D4, D5 and D13, foundation rules 1 through 11 and checks C1 through C13, the approved sprint 4 sections and checks R1 through R14, the sprint 5 compatibility boundary, the reviewed contract's Projects and Store row and its paired and website checks, both round-three closing lists, issues 1632 through 1635, and Build Loop Phase 4. The boundary rows below identify the owner for checks that require another plan.

| Source item | Workstream | Status |
|---|---|---|
| Task 1. Service, gates, key and discovery. | WS-1, WS-3 | Items 2, 7 and 8. |
| Task 2 and 3. Tables and durable folder operations. | WS-1 | Items 1 through 4. |
| Task 4 and 5. Attach and remove. | WS-2 | Items 5 and 6. |
| Task 6. Repository watch. | WS-3 | Item 7. |
| Foundation D1, D2, D4, D5 and D13. | WS-1, WS-2, WS-3 | Go and PostgreSQL, one feed, a reviewed contract, and explicit machine removal. |
| Foundation rules 1 through 4. | WS-1, WS-2, WS-3 | Part-owned migrations and tables, reviewed bindings, Go interfaces and one Store write. |
| Foundation rules 5 through 7. | WS-1, WS-2, WS-3 | Shared watch, durable commands, state in PostgreSQL. |
| Foundation rules 8 through 11. | WS-1, WS-2, WS-3 | Fixed errors, filtered inputs and logs, real database and Link fixtures, and source credit rules. |
| Foundation C1, C2, C4 through C11 and C13. | WS-1, WS-3 | Underlying stack, sign-in, join, link and vault checks stay with their owners. The build runs the combined Server suite and deployment owns the full stack. |
| Foundation C3 and C12. | WS-1, WS-3 | Exercise the mounted auth guards and preserve the real PostgreSQL and feed-lock proof. |
| Sprint 4 four Link changes and R8 through R11. | WS-1, WS-3 | The Link request defines callbacks, machine lock and report-before-settlement. Link and stack own permits, admission, staged result recovery and revision upgrade. |
| Sprint 4 R1, R2, R3 and R4. | WS-1, WS-2 | Inspect fields, absolute stored path, alias identity and origin precedence. |
| Sprint 4 R5, R6 and R7. | WS-1 | Durable keyed retry, report-before-outcome and expiry while the caller stops waiting. |
| Sprint 4 R12. | WS-2, WS-3 | Removal transaction and live watch update. |
| Sprint 4 R13 and R14. | WS-3 | Mounted-only real discovery. The Website and Stack plans run the browser checks. |
| Sprint 5 boundary. | WS-1, WS-2 | Stable identifiers, reusable operation plumbing and the future project reference and orphan guard. See decisions 3 and 15. |
| Contract. Receipt retention, replay race, deletion races and origin precedence. | WS-1, WS-2 | Items 3, 5 and 6. |
| Contract. Three generated preflights, fixed JSON and refusal replies, protected watch and discovery. | WS-1, WS-3 | Items 2, 4, 7 and 8. |
| Contract. Paired recovery, receipt, settlement and folder proof. | WS-1, WS-2, WS-3 | Projects observes durable records and hands off exact Link hooks and real-stack acceptance. |
| Round-three logic and fit closing lists. | WS-1, WS-2, WS-3 | Decisions 12 through 16 and the handoffs cover the applicable points. |
| Issues 1632 through 1635 and Build Loop Phase 4. | WS-1, WS-2, WS-3 | Reuse existing watch and locks, hand transport and paired test ownership to existing owners, and size and freeze each test-first unit. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md` → The three parts meet through the contract. | Consume the reviewed sprint 4 generated bindings and leave contract changes to Contract. |
| `AGENTS.md` → The new server is Go and legacy code is a reference. | Build only in the current Go server part and its startup and API registration. |
| `AGENTS.md` → Credit copied code. | The build reuses repository patterns and uses the standard library. If the plan changes to use copied code, the orchestrator arranges the required `third_party/` and `NOTICE` changes before it merges. |
| `AGENTS.md` → Record discoveries and reach the base branch through a pull request. | The result carries cross-plan discoveries to the orchestrator, who owns issue recording and integration. |
| `AGENTS.md` → This repository is independent of Herdr Synapse. | Follow the epic's normal base-branch integration path. |
| `AGENTS.md` → Documents use dates, descriptive headings and repository-relative paths. | The dated decision log and the privacy check cover the assignment. |
| Epic team boundary and bounded Projects assignment. | Projects build paths are confined to the Projects package and API and startup registration. Store and Link changes are handoff requests. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | backend | `server/backend` | medium | The part adds three related tables with local integrity and real database probes. | No | Reviewed contract for field semantics. |
| 2 | backend | `server/backend` | medium | Two unary calls use one existing request flow and a bounded read loop. | Yes | 1 and the Store and Link interfaces. |
| 3 | backend | `server/backend` | medium | A retention worker uses one Store key boundary and one operation deletion. | No | 1 and Store. Integrate the race with 2. |
| 4 | backend | `server/backend` | medium | Two transaction callbacks apply a small guarded state machine. | Yes | 1, 2 and Link callbacks. |
| 5 | backend | `server/backend` | medium | One report attachment transaction owns origin checks and the resolved folder identity. | No | 4. |
| 6 | backend | `server/backend` | medium | One removal callback implements the repository lock protocol. | No | 4, 5 and Link removal. |
| 7 | backend | `server/backend` | medium | One new stream reuses the existing follower and session checks. | Yes | 1 and interfaces from 2 and 5. Integrate with 6. |
| 8 | backend | `server/backend` | medium | Existing routing, discovery and startup patterns wire one service. | Yes | 1 through 7 and the Link startup handoff. |

Load. OpenAI supplies the planner, test writers, builders and reviewers. Size 23 launches including this planner, eight test writers, eight builders, two plan reviewers and four focused code reviewers. One plan reviewer can cover logic and the other can cover fit. The focused code reviewers can cover the schema and operations, retirement, attachment and removal, and watch and router groups. Use one writer per file set, and hand over the shared startup file in order. No live remaining weekly balance was supplied. The orchestrator checks it before dispatch and schedules later batches if the remaining allowance cannot cover the next group. No alternate vendor is planned.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Round 1 | Pending. | At `attack`, the orchestrator runs the plan review and records findings in this folder. |

</details>
