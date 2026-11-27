# REVISE

The migration can save a staged result for a command whose stream identifier is empty, which the claim code treats as unassigned.

The frozen test writer and both build workers were OpenAI agents, the same vendor as this reviewer. I reviewed `608ab9c9` on `sprint-0` on 2026-11-27 for task `postbuild-go-s4-server-link-2`, nonce `87a778`.

## Findings

### post-2 1 · MAJOR · An unassigned dispatch can carry a stage

**Statement.** Plan item 2 requires successful dispatch with a stream assignment before a result can be staged. By reading, `link_commands_stage` checks only that `sent_stream_id` is present. The claim code represents an unassigned stream with an empty string and only increments the attempt when the stream identifier is nonempty. [Plan item 2](../plan.md#L141) · [Stage constraint](../../../../src/server/internal/link/migrations/0003_link_revision_two.sql#L25) · [Claim projection](../../../../src/server/internal/link/outbox_claim.go#L93) · [Assignment](../../../../src/server/internal/link/outbox_claim.go#L144)

**Failure scenario.** Confirmed live in PostgreSQL. The migration accepted `dispatched = true`, `sent_stream_id = ''`, `staged_outcome = 'done'`, and `staged_event_number = 43`. A writer that saves an empty assignment can therefore leave a staged success for the later replay path to settle when it reaches event 43. The current revision 1 runtime produces no stages.

**Fix.** Require a nonempty saved stream identifier in the stage constraint and add a PostgreSQL rejection case for the empty string.

### post-2 2 · MINOR · The migration test misses valid and forbidden staged outcomes

**Statement.** Plan item 2 requires the stage to accept client outcomes and preserve the standing-notice expiry rule. By reading, the migration test stages `interrupted` successfully and checks expiry on a terminal `set_state` row. It has no successful `done` stage or rejection of an expired standing stage. [Plan item 2](../plan.md#L141) · [Stage outcome rules](../../../../src/server/internal/link/migrations/0003_link_revision_two.sql#L23) · [Test cases](../../../../src/server/internal/link/migration_v2_test.go#L132) · [Standing terminal check](../../../../src/server/internal/link/migration_v2_test.go#L173)

**Failure scenario.** Confirmed live in separate mutations. Removing `done` from the allowlist passed the focused tests but made PostgreSQL reject a valid successful stage with check violation `23514`. Removing the standing-notice guard also passed those tests. PostgreSQL then accepted an expired standing stage and rejected its terminal transition with `23514` on `link_commands_state_outcome`.

**Fix.** Exercise each allowed staged outcome with a successfully dispatched command. Add a direct rejection case for an expired stage on a dispatched `set_state` row.

### post-2 3 · MINOR · The expiry tests pass without the moved stage filter

**Statement.** The moved item 6 expiry page must skip staged rows. Confirmed live, the focused tests passed after I removed only `staged_outcome IS NULL` from the expiry page. The migration test exercises index membership, and the sweep tests exercise unstaged queues. [Plan item 6](../plan.md#L236) · [Expiry page](../../../../src/server/internal/link/outbox_sweep.go#L35) · [Migration index test](../../../../src/server/internal/link/migration_v2_test.go#L176)

**Failure scenario.** Confirmed live in PostgreSQL. With the filter removed, 48 of the first 64 expiry candidates were staged. The current expiry update on that batch failed with `23514` on `link_commands_stage`. The sweep returns that batch error. [Sweep error handling](../../../../src/server/internal/link/outbox_sweep.go#L100) · [Expiry update](../../../../src/server/internal/link/outbox_sweep.go#L138)

**Fix.** Seed an aged staged row beside an aged ordinary row in the sweep test. Require a successful sweep that times out the ordinary row and leaves the stage and payload intact.

## Post-build checks and the early page filters

By reading, the only unreviewed mechanism marked in the plan and the round-one register is the Projects bulk-removal refresh in item 6. Applying replay, settling once, and exercising the removal race belong to the revision 2 runtime and paired gate. The server in the reviewed snapshot still speaks revision 1. [Review register](round-1-findings.md#L53) · [Post-build scenarios](../plan.md#L337) · [Wire revision](../../../../src/server/internal/link/handshake.go#L21)

Confirmed live for the storage portion of the three-stage replay scenario. After reconnecting to PostgreSQL, stages written in sequence order with watermarks 45, 45, and 43 read back as the third, first, and second rows. A query ordered by watermark and sequence reached those rows through `link_commands_staged`. The built claim and expiry pages returned 32 and 64 rows respectively, with zero stages in either page. The migration test also verified that removing one Link machine cascades its run history, preserves the other machine's history, and retains the old terminal command. [Stage index](../../../../src/server/internal/link/migrations/0003_link_revision_two.sql#L39) · [Removal assertion](../../../../src/server/internal/link/migration_v2_test.go#L230)

Moving both filters forward was correct. The rebuilt partial indexes require the stage predicate, and legacy commands receive null stage fields. In the PostgreSQL probe with 10,000 other commands, 48 stages, and 97 ordinary target commands, the moved queries used `link_commands_pending` and `link_commands_expirable`. With 160 ordinary target commands, the claim planner chose `link_commands_seq_key` and filtered 10,048 rows before returning 32. The same broader scan occurred with zero stages on both the old and the new schemas, filtering 10,000 rows in each case. With the original expiry query on the upgraded schema, PostgreSQL scanned and sorted 10,208 rows for its 64-row page. The move restores index eligibility. The item 6 lock-time rechecks still need to cover rows that acquire a stage after a page is read. [Rollout order](../plan.md#L44) · [Index change](../../../../src/server/internal/link/migrations/0003_link_revision_two.sql#L30) · [Builder's blocker](../../../../.foundry/go-rewrite/results/s4-link-2-build.md#L15) · [Builder's handoff](../../../../.foundry/go-rewrite/results/s4-link-2-build-b.md#L9)

## PostgreSQL tests and required mutations

Confirmed live. I used `deploy/test/with-postgres.sh` and the archived `608ab9c9` source. The 15 selected tests all printed `PASS` with zero skips. The diff of migrations 1 and 2 was empty. The upgrade test observed the old rows, payload bytes, mark and keyed reply through the version 3 upgrade and reopen. The historical fixture still exercised version 1 to 2, and the application fixtures exercised version 3 during startup and after an interrupted startup. [Upgrade fixture](../../../../src/server/internal/link/migration_v2_test.go#L65) · [Historical fixture](../../../../src/server/internal/link/outbox_test.go#L38) · [Sweep startup fixture](../../../../src/server/internal/app/commands_test.go#L268) · [Interrupted startup fixture](../../../../src/server/internal/app/review_commands_test.go#L233)

| Tests | Package output |
| --- | --- |
| `TestLinkRevisionTwoMigration`, `TestCommandMigrationUpgradesPartB`, `TestBecomeCurrent`, `TestClaimBoundsIneligiblePrefix`, `TestSweepCommands`, `TestSweepReleasesMachineBetweenBatches`, `TestCommandSweepStops` | `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/link 7.119s` |
| `TestServerCommandSweepLifecycle`, `TestServerRestartsAfterInterruptedCommandMigration` | `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/app 6.070s` |
| `TestScanAndReconnectQueueCommands`, `TestActionRequestKeyReplays`, `TestScanAndReconnectRefused`, `TestActionRequestKeyConflictsAcrossRoutes` | `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/machines 25.469s` |
| `TestRecordValidationAndLimit`, `TestRecordOnceValidationAndLimit` | `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/store 3.164s` |
| Full Link package with `-race -count=1` | `PASS` and `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/link 208.353s`. The runner recorded 368 passing tests and subtests, with zero skips and zero race reports. |
| Full Link package with the three coverage mutations combined | `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/link 164.813s` |

Resetting `event_mark` to zero exited 1. `migration_v2_test.go:94` printed “The migration did not preserve the approved machine's event mark of 42.” Allowing a stage with only one staged field exited 1. Both directions printed “The schema accepted an invalid record expected to fail with SQLSTATE 23514” at `migration_v2_test.go:167`. [Required mutations](../plan.md#L144) · [Mark assertion](../../../../src/server/internal/link/migration_v2_test.go#L94) · [Pair assertions](../../../../src/server/internal/link/migration_v2_test.go#L156)

**Grade.** 8 out of 10.
