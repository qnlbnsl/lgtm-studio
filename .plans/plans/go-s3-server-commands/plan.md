---
topic: go-s3-server-commands
status: attack
tier: M
created: 2026-11-26
updated: 2026-11-26
epic: go-rewrite
---

# Stored commands for machines

## Overview

The server will save an approval, rename, scan or restart command before asking the machine to act. A machine that is already connected when the owner approves it will receive the approval on that connection. The machine can then report its facts and harnesses and appear online without reconnecting.

When a connection drops after a scan or restart, the next connection can carry another attempt of the same command. The machine can answer from its saved result. The server checks the attempt against the current connection before accepting the result. A queue of commands that can no longer be sent leaves the connection time to answer heartbeats.

The server will accept restart requests for approved machines with a live connection and scan requests for machines that have finished reporting on that connection. The change to the visible Machines page and its website contract revision waits for owner choice 6.

## Key decisions

The approval or rename and its notice commit together. If the owner retries a successful request with the same request key, the server returns the original reply without adding another command.

A scan or restart can be sent again before 23 hours have passed since its expiry. A matching result can still close it until the server records the expiry, after at least 24 hours. The machine uses its own clock to prune saved results. A scan or restart runs at most once while the machine clock leads by less than an hour minus the time from sending an attempt to looking up its saved result. Beyond that margin a retry can report expired after an earlier success. A clock more than a day fast can allow a repeat after pruning, within the limit accepted for the first release. An approval or rename notice keeps its original revision and remains pending until the machine answers or the owner removes the machine.

If the database temporarily cannot clear a closed connection, the page may still show the machine online. A new scan or restart request checks that the server still holds the exact connection.

[Full decision log](decisions.md)

## Workstreams

### The server keeps a record of each command

Each command keeps the same identifier and expiry across all of its attempts. Removing a machine ends its pending commands in the same write. The stored result shows a single final disposition even when a result and the expiry job race.

### The connection sends commands and continues serving reports

The server takes the oldest eligible command in small steps. It checks the stored attempt again before writing the frame. A busy machine can still receive heartbeat answers and can reconnect after a blocked write.

### The owner can approve, rename, scan and restart a machine

Approval reaches a waiting machine that already has a connection. The open Machines watch shows the transition to online after the machine reports. Scan and restart use the existing owner calls and keep their write and retry behavior.

## Rollout order

1. Start from the merged Machines records and link stream. Record their passing commit before adding commands.
2. Apply the numbered link migration through normal server startup. Verify the upgrade with existing machines and saved request replies, and verify the new command storage and dispatch before enabling owner calls.
3. Wire the commands and their cleanup worker into the server. Check the owner calls, an open Machines watch and a client that reconnects with its saved results.
4. Run the combined client and server tests and the deployed sprint checks. Send the page change to the owner under choice 6.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Build and test conventions**

D1 to D23, R1 to R11 and C1 to C13 identify decisions, rules and checks in the [foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md). A deployed check is a function in `deploy/test/checks.sh`. A contract check is a post-build row in the [contract plan](../go-s0-contract/plan.md). Its frozen wire is Protocol Buffers link revision 1. The schema below uses PostgreSQL Structured Query Language (SQL). UUID means universally unique identifier. HTTP means Hypertext Transfer Protocol. All file paths within a lane's item table are rooted at the repository.

Each item owns only its listed files. Freeze tests after a run that reaches the assertion and fails with the item's own stubs. Run each mutation separately on a temporary branch. Record the named failing assertion and then restore the item. Use `deploy/test/with-postgres.sh -- go test -race -count=1 -run '<named tests>' <package>` for each item and require every named test to run without skips. Test code in `src/server/internal/link` uses the existing scripted peer and the real database. Owner tests that import link use package `machines_test`. The real client and server together are a sprint gate owned by the orchestrator.

Use `newPresenceFixture` for link storage tests and a separate database connection to observe committed rows. For a held commit use a scoped `pgx.QueryTracer` at the real `COMMIT`, on the connection for the operation being tested. Release it or terminate that backend to distinguish commit from rollback. Use `checkOneLine` when both writers act on one machine. Observe the barrier or the PostgreSQL lock wait before asserting that another operation, a send or a reply has not happened. Keep the stream suite's test names, including `TestCommandResultFrameRules`, `TestWriterSharesTurns`, `TestBlockedMachineEndsStream`, `TestFollowWakesAndEnds`, `TestFollowResubscribes` and `TestWritesOfOneMachineFormOneLine`.

The store key lock comes before the machine row for keyed calls. The token row comes before the machine row for revocation. A command write for an existing machine takes its machine row before link rows. Removal already owns that row by deleting it in the enclosing transaction. The PostgreSQL lock is the durable ordering point. The hub supplies liveness for owner calls and wakes the link writer through the existing follower.

Reuse the stream plan's error mapping and first terminal cause. `ErrUnknownMachine` becomes `failed_precondition` with `unknown_machine` in the link refusal metadata. `ErrNotCurrent` becomes `aborted`. `ErrInvalidMessage` becomes `invalid_argument`. A store failure is retryable as `unavailable` on Open. Use fixed diagnostics without raw frames, payloads, credentials or request keys. Part A and Part B here mean the records and stream slices named in the [handoff](../go-s3-server-machines/handoff-stream-and-commands.md).

**WS-1: The server keeps a record of each command**

The command row carries the action and the server's record of attempts and their final disposition. The same receipt can reach the server after a disconnect. The server accepts it only for the exact attempt it last assigned to the current stream.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | A committed enqueue stores attempt 1 and appends one wake reference. A rollback leaves neither. | `TestEnqueueStoresBeforeSend` |
| 2 | A result changes one pending row only when the machine, stream and attempt match. | `TestAcceptResultRules`, `TestResultAndReplacementFormOneLine` |
| 5 | Commands expire, survive retention, or end on removal according to their recorded disposition. | `TestSweepCommands`, `TestSweepReleasesMachineBetweenBatches`, `TestCommandRemovalIsAtomic` |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Upgrade the link schema and enqueue one command. | `src/server/internal/link/migrations/0002_link_commands.sql`, `link_commands` and `src/server/internal/link/outbox.go`, `State.Enqueue`. Pattern S5. |
| 2 | Close the matching attempt. | `src/server/internal/link/outbox_result.go`, `State.AcceptResult`. Pattern S6. |
| 5 | Expire, retain and end pending commands on removal. | `src/server/internal/link/outbox_sweep.go`, `State.SweepCommands` and `src/server/internal/link/presence.go`, `State.Forget`. Patterns S7 and S8. |

**Item 1. Migration 2 and enqueue**

- **Files and names.** Create `src/server/internal/link/migrations/0002_link_commands.sql`, `src/server/internal/link/outbox.go` and `src/server/internal/link/outbox_test.go`. Declare `(*State).Enqueue(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) error`, `outboxTopic = "machine_outbox"`, `CommandGrace = 24 * time.Hour` and `CommandRetention = 7 * 24 * time.Hour`. Add no name to a Part A file. The existing `Migrations()` embeds the new file.
- **Dependencies and size.** Needs Part B migration 1 and its passing registry and link gates. The test writer for item 2 can prepare its cases while this item builds. Work area `migrations`, complexity medium, scope `server/backend`. The additive change needs an upgrade fixture and an atomic enqueue. Statements D4, D5, R1, R3, R4, R6, R7 and R10.
- **Schema.** The table `link_commands` has `id text PRIMARY KEY`, `machine_id text NOT NULL`, `action text NOT NULL`, `payload bytea NOT NULL`, `created_at timestamptz NOT NULL`, `expires_at timestamptz NOT NULL`, `attempt bigint NOT NULL DEFAULT 1`, `sent_stream_id text`, `finished_at timestamptz`, `seq bigint GENERATED ALWAYS AS IDENTITY` and nullable `outcome text`. Constrain `attempt` to 1 through 4294967295. A null stream identifier signifies an unsent first attempt and requires attempt 1. Constrain `action` to `scan_harnesses`, `set_state` or `restart`. Constrain a nonnull `outcome` to `done`, `failed`, `expired`, `unsupported`, `interrupted` or the storage-only disposition `removed`. Pair outcome and finish time with a null-equivalence check. Disallow `expired` for `set_state`. An expired unsent action and a removed unsent command may retain a null stream identifier. Use fixed elapsed hours for the cutoff and retention intervals. Use indexes `link_commands_pending (machine_id, seq) WHERE outcome IS NULL`, `link_commands_expirable (expires_at, machine_id, seq) WHERE outcome IS NULL AND action <> 'set_state'`, and `link_commands_finished (finished_at, machine_id, seq) WHERE outcome IS NOT NULL`. Keep terminal rows after removal without a foreign key to a live machine or stream. Item 5 prunes them.
- **Enqueue.** The caller owns the registry row lock and the transaction. Generate the command identifier through `store.NewID`. Build the payload anew from the declared action and, for `SetState`, the declared approval, name and positive revision. Reject missing or unsupported actions and invalid typed payloads. Ignore supplied envelope fields and unknown fields. Read the database clock once for `created_at` and the positive lifetime added to it for `expires_at`. Insert attempt 1 with a null stream identifier. Record a `store.ChangeRef` with topic `machine_outbox`, empty scope, machine identifier as key and `store.Saved`. Return any insertion or record error to the enclosing write. Only enqueue records this topic.
- **Tests written first.** In `outbox_test.go`, `TestCommandMigrationUpgradesPartB` applies migrations through link 1, creates a waiting machine and a connected one, and saves a keyed reply and an event mark. Apply migration 2 twice through the real migrator. Observe link revision 2 in the store's `schema_migrations` ledger, zero command rows and identical saved machine, link and receipt data. Reopen with the link 1 manifest and observe the ledger refusing the downgrade, then verify that the full manifest still starts. Attempt a bound or terminal-pair violation and require a database constraint error. `TestEnqueueStoresBeforeSend` enqueues a scan with a two-minute lifetime. Observe a version 7 identifier, first attempt, null stream identifier, immutable stored expiry, declared-only payload, and exactly one wake reference. Compare expiry to the single sampled creation time. Use a real rejected commit to observe zero inserted commands, feed references and caller record changes. A missing action fails before insertion; a supported action with stray envelope or unknown fields stores only its declared action.
- **Risk and mutations.** Risky because queue order and visibility depend on one write. Moving the insertion to a separate committed write or skipping the feed reference fails `TestEnqueueStoresBeforeSend`. Dropping the final-state check fails `TestCommandMigrationUpgradesPartB`. The item passes when both tests pass in `./src/server/internal/link`.

**Item 2. Match results to the current attempt**

- **Files and names.** Create `src/server/internal/link/outbox_result.go` and `src/server/internal/link/outbox_result_test.go`. Implement `(*State).AcceptResult(ctx context.Context, machineID, streamID string, result *linkv1.CommandResult) error` with the frozen `CommandHooks` signature. Define private `lockCommandStream(ctx context.Context, tx *store.Tx, machineID, streamID string) (machines.Machine, error)` on `State` for this item and items 3 and 4. It calls `Registry.Lock`, `State.locked` for the existing lock barrier, and checks the exact connection row in that transaction. Add no name to a Part A file.
- **Dependencies and size.** Needs item 1 and merged Part B. The item 5 builder can work in parallel after item 1. Work area `backend`, complexity medium, scope `server/backend`. This is one guarded update with a finite outcome table. Statements D7, R3, R4, R6, R7, R9 and R10.
- **Result rules.** A missing machine or displaced stream yields the existing stream fence error. It changes no command. A current waiting machine's result is ignored before payload validation. For an approved machine, reject a missing result, noncanonical identifier or attempt zero as `ErrInvalidMessage`. Reuse `canonicalUUID` from `rules.go`. Match `id`, `machine_id`, nonnull exact `sent_stream_id`, exact `attempt` and null `outcome`. Read only `command_id`, `attempt` and `outcome` from the input. Store the five known final outcomes by their lower-case names. Store unspecified, unknown, or `EXPIRED` for a `SetState` as `failed`. Set `finished_at = clock_timestamp()` in the same update. A duplicate, unknown command, or mismatched attempt changes nothing. A matching result after the send cutoff or after 24 hours still wins if the row is pending. Return no result payload and record no feed reference. The stream checks the frame independently when its hook is nil.
- **Tests written first.** `TestAcceptResultRules` seeds attempts and fixed stream associations directly in the disposable command table. On attempt 2, send attempts 1 and 3, an unknown identifier, a valid result on an unrelated machine and a result on the new current stream before its claim. Observe no changes. Observe the stale-stream fence error for the old stream. Cover malformed current approved and malformed waiting results and the normalized outcomes. Give the approved matching result a Protocol Buffers wire field numbered 4 with a marked fixture string. Assert that the result closes the row and the text reaches no database field, log or website response. Test matching results after both time thresholds before sweeping and after an outcome is already stored. Observe the same original `finished_at` after a duplicate and no new wake or machine reference.
- **Ordering test.** `TestResultAndReplacementFormOneLine` holds a result under the machine lock with `checkOneLine`. A replacement waiting on that lock follows the committed result. Reverse the order and observe the displaced error and no final write. Seed equal attempt numbers on different rows to make the machine and stream filters separately observable.
- **Risk and mutations.** Risky because an unrelated or earlier attempt must not close a command. Dropping any of the machine, stream, attempt or pending predicates fails `TestAcceptResultRules`. Using a pre-lock stream lookup in place of the in-transaction check fails `TestResultAndReplacementFormOneLine`. Both tests pass in `./src/server/internal/link`.

**Item 5. Expiry, removal and retention**

- **Files and names.** Create `src/server/internal/link/outbox_sweep.go` and `src/server/internal/link/outbox_sweep_test.go`. Declare `(*State).SweepCommands(ctx context.Context) error` and `(*State).RunCommandSweep(ctx context.Context, report func(error))`. Change only the body of `State.Forget` in `src/server/internal/link/presence.go`. Extend that existing Part B method without a new name in a Part A file. Applications call the worker method from item 8.
- **Dependencies and size.** Needs item 1 plus Part B removal. Can run alongside items 2, 3 and the calls as their dependencies permit. Work area `backend`, complexity medium, scope `server/backend`. The item handles a bounded maintenance write and a pre-existing atomic removal. Statements D3, R3, R4, R6, R7 and R10.
- **Maintenance.** Run one cancellable worker with an initial pass and hourly passes. A pass snapshots a database time and indexes due candidates by `(expires_at, machine_id, seq)` or `(finished_at, machine_id, seq)`. Use keyset pages of 64 references outside machine locks. Include the command identifier in each reference. Group a page by machine. Open a store write per machine, take `Registry.TryLock` and recheck each candidate against the transaction's current state and the pass cutoff. A batch with the nonwaiting lock refusal, PostgreSQL code `55P03`, is skipped for this pass and revisited on the next pass. Keep a cancellation check between writes and pages. A new due row or terminal row produced during a pass can wait for the next pass. Never hold two machine rows at once. Only a pending nonstate action with `expires_at + CommandGrace <` the database cutoff can be marked `expired`. Set outcome and database finish time together. A terminal row with `finished_at` strictly older than seven days is eligible for deletion. For a missing machine, delete only terminal rows that meet retention. Identifiers are never reused. Return other database failures through the worker's fixed diagnostic and retry at the next interval. Keep successful writes made before a failure.
- **Removal.** In the caller's existing removal or waiting-token revocation transaction, `State.Forget` finalizes pending commands as `removed` and sets their finish time. It keeps any outcome already stored and deletes the connection and mark through the existing statements. The registry deletion and the machines feed are in that transaction. There is no removal frame to deliver and no new receipt or wake reference from finalization.
- **Tests written first.** `TestSweepCommands` uses an expired action at 25 hours, one at 23 hours, and a `SetState` at 25 hours. Observe only the first becoming expired. Manually age a final row by eight days and keep one aged six. Check pruning by finish time, no new feed reference, stable outcome on a repeat sweep and the first result winning when already stored. `TestSweepReleasesMachineBetweenBatches` seeds several pages, holds A's registry lock, and observes B's expiry or prune before release. Verify each write covers at most 64 candidates, cancel with work remaining, then run a fresh pass and verify the rest finish. Hold a single machine at a test-owned matching result update or at the expiry write in both orders. Observe one final value and finish time. Item 8 exercises the real send hook and sweeper together. `TestCommandRemovalIsAtomic` seeds pending actions and notices for an approved machine and a waiting machine. Exercise remove and waiting-token revocation through the real record path. Fail the enclosing commit after `Forget` and observe the registry, link rows, feed, request receipt and commands unchanged. Commit and observe ended commands, retained earlier outcomes, no later match or send, and eventual pruning of the removed machine's terminal rows. An approved machine's redeemed token revocation leaves its commands unchanged. `TestCommandSweepStops` observes an initial pass, an hourly tick through a private test timer and cancellation during a database lock or commit.
- **Risk and mutations.** Risky because a competing result, sweep and removal must agree on one terminal state. Expiring `SetState` or dropping the pending guard fails `TestSweepCommands`. Expiring without a machine lock or using an unbounded single-machine update for maintenance fails `TestSweepReleasesMachineBetweenBatches`. Omitting finalization on waiting removal or moving it out of the removal write fails `TestCommandRemovalIsAtomic`. Leaving the worker alive after cancellation fails `TestCommandSweepStops`. All four tests pass in `./src/server/internal/link`.

**Design artifact.** The [frozen life of a command](../../../src/contracts/proto/lgtm/link/v1/commands.proto) and Part C of the [handoff](../go-s3-server-machines/handoff-stream-and-commands.md) describe the storage and removal behavior.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An acknowledged owner write has no saved action. | High | Gate storage and feed on the same real commit. |
| A later or unrelated result overwrites the final result. | High | Use the machine lock, the exact-attempt update and two competing transaction orders. |

**Verify.** Run the named PostgreSQL tests, including `TestCommandMigrationUpgradesPartB`, `TestAcceptResultRules`, `TestCommandRemovalIsAtomic` and `TestCommandSweepStops`. No device is needed.

**Rollout detail.** For step 2 apply migration 2 through `link.Migrations()` in the ordinary start path and through the upgrade fixture. No existing rows need backfill. Once the ledger records link revision 2, a server that only knows revision 1 refuses startup. Recover with a corrected build that still carries migration 2. Retain the table and its outcomes.

**WS-2: The connection sends commands and continues serving reports**

The writer spends a bounded turn looking for one command. It records an attempt before sending. Immediately before sending it checks that the attempt is still pending on the current connection, and it gives the write a deadline.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3 | A claim returns one oldest eligible command after commit and yields while passing an ineligible prefix. | `TestClaimCommitsBeforeReturn`, `TestClaimRaisesAttemptOncePerStream`, `TestClaimOrder`, `TestClaimBoundsIneligiblePrefix` |
| 4a | The response writer honors an earlier context deadline. | `TestSendUsesEarlierContextDeadline` |
| 4b | A held attempt cannot start after finalization or the send cutoff. A full reader still receives heartbeat responses. | `TestCommandSendRechecksBeforeWrite`, `TestCommandSendDeadline`, `TestCommandBacklogYieldsToHeartbeats`, contract check 11 |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Select a bounded page and persist one attempt. | `src/server/internal/link/outbox_claim.go`, `State.claimNext`. Patterns S6 and S9. |
| 4a | Cap a response write by its supplied context. | `src/server/internal/transport/deadline.go`, `Send`. Pattern S10. |
| 4b | Connect the persistent claim to the stream writer and its deadline. | `src/server/internal/link/outbox_send.go`, `State.SendNext` and `src/server/internal/link/stream.go`, `Service.serve`. Patterns S9 and S10. |

**Item 3. Claim one command in a bounded turn**

- **Files and names.** Create `src/server/internal/link/outbox_claim.go` and `src/server/internal/link/outbox_claim_test.go`. Declare private `commandCursor` containing the last examined `seq` and seen approval, `claimPageSize = 32`, and `(*State).claimNext(ctx context.Context, machineID, streamID string, cursor *commandCursor) (cmd *linkv1.Command, more bool, err error)`. Add no name to a Part A file. The stream adapter owns a fresh cursor per current stream.
- **Dependencies and size.** Needs items 1 and 2. It can run alongside item 5 and item 6. Work area `backend`, complexity medium, scope `server/backend`. It is one ordered query and attempt update with a commit gate. Statements R3, R4, R6, R7, R10 and C8.
- **Bound the query before filtering.** Call `lockCommandStream` in a store write. Under that lock reset the cursor to zero on the first observed transition from waiting to approved. Seek `link_commands_pending` by `machine_id` and `seq > cursor`, in sequence order, with `LIMIT 32` inside a materialized page. Read small metadata first. Test eligibility only among that page's rows. An unsent row uses attempt 1. A row with a different sent stream can advance one attempt up to the maximum unsigned 32-bit value. The current stream skips its own assigned attempts, including ambiguous sends. A waiting row can take only `set_state`. Other actions require the sampled database time to be strictly earlier than `expires_at + CommandGrace - 1 hour`. Finished rows are outside the pending index. Do not sort the whole eligible set, aggregate the prefix or walk successive pages under this lock. Check the query plan on a sufficiently large analyzed fixture. Use a two-second context for this write, including lock wait. Bound elapsed database work even when the index has dead entries.
- **Advance after commit.** For the oldest eligible row in the page, read its payload, persist its attempt and stream and reconstruct the generated `Command` with the stored identifier and expiry. If selected, the cursor advances through only that row. If none is selected, advance it through the page. Return a continuation when the page may have more entries, including an entire ineligible page. Keep the frontier when idle because enqueues take the same lock and allocate larger sequence numbers. A pending row already past the cutoff stays skipped on this stream. At the attempt ceiling keep the row pending and allow later commands to progress without wrapping. Return no command and do not advance on rollback or a lost commit reply. The stream ends on such a failure. A later stream can see an attempt number higher than the last one visible on the wire when an earlier durable claim never made it to the client. A claim never finalizes a command or appends a feed reference.
- **Tests written first.** `TestClaimRaisesAttemptOncePerStream` observes attempt 1 and the stored expiry on A, no duplicate on A, then the same identifier, action and expiry with attempt 2 on B. Exercise the unsigned ceiling using a seeded row and observe progress of the next command. `TestClaimCommitsBeforeReturn` holds the real commit, sees no returned command or visible changed attempt, and checks the values after release. Exercise forced rollback and lost commit reply separately. A fresh stream reads the durable outcome. `TestClaimOrder` uses reversed creation times, work on the cutoff's two sides, an old `SetState`, and a waiting machine that later approves on the same stream. Check one lowest eligible `seq` per claim, stale-stream refusal, eligibility after approval and no feed writes. `TestClaimBoundsIneligiblePrefix` plants several pages of expired, already assigned, or disallowed pending rows before one eligible row. Check the actual page-query shape and measured rows at the bounded page node with the pending index. Assert each empty turn reports continuation and releases the registry lock. A competing heartbeat can commit before the next turn, and the writer eventually reaches the eligible row.
- **Risk and mutations.** Risky because both attempt order and heartbeat latency depend on this item. Filtering before limiting, scanning another page under the same lock or jumping the cursor past an eligible row fails `TestClaimBoundsIneligiblePrefix` or `TestClaimOrder`. Returning early fails `TestClaimCommitsBeforeReturn`. Reusing the first stream's marker or wrapping the attempt fails `TestClaimRaisesAttemptOncePerStream`. All four tests pass in `./src/server/internal/link`.

**Item 4a. Honor an earlier response deadline**

- **Files and names.** Change `src/server/internal/transport/deadline.go` at `Send`. Create `src/server/internal/transport/command_deadline_test.go`. Keep the existing Part A function signature. Add no Part A declaration.
- **Dependencies and size.** Needs the built Part A deadline middleware. Can run alongside items 1 through 3 and 5 through 7 as their prerequisites allow. Work area `backend`, complexity medium, scope `server/backend`. This change uses the existing deadline controller and cancellation path. Statements R6, R10, C8 and C9.
- **Behavior.** Set the response write deadline to the earlier of the normal deadline and the supplied context deadline, if any. Recheck the supplied context before invoking the response callback. Preserve the existing cancellation hook, deadline cleanup and error handling. A canceled context before write entry cannot invoke the callback.
- **Tests written first.** `TestSendUsesEarlierContextDeadline` records the deadline sent to the response controller. The ordinary context uses the configured transport timeout. An earlier live context uses its exact deadline. A canceled context sends nothing. Cancel while the callback is blocked and observe the forced deadline and callback exit before the writer is released.
- **Risk and mutations.** Risky because a response could start or stay blocked after a command's authorization ends. Ignoring the shorter deadline or running the callback with a canceled context fails `TestSendUsesEarlierContextDeadline`. The new test and `go test -race -count=1 ./src/server/internal/transport` pass.

**Item 4b. Fence one command write**

- **Files and names.** Create `src/server/internal/link/outbox_send.go` and `src/server/internal/link/outbox_send_test.go`. Change `src/server/internal/link/stream.go` at `Service.serve`. Declare `(*State).SendNext(ctx context.Context, machineID, streamID string, send func(*linkv1.Command) error) (sent bool, err error)` to match `CommandHooks`, and private `commandSession`, its context key and `State.sendClaimed`. A session belongs to the sole writer and carries the cursor and the one callback's write context. A private test control `afterCommandClaim func(context.Context, *linkv1.Command) error` runs after the claim's commit and before the send check, through a test-only context value. Add no name to a Part A file. Keep both hook signatures.
- **Dependencies and size.** Needs items 3 and 4a and the merged Part B stream loop. Item 5 can build independently. Item 4b tests can seed a terminal row through a locked fixture until the real sweeper is available. Work area `backend`, complexity medium, scope `server/backend`. The item joins two existing write and transport boundaries with one bounded authorization check. Statements R3, R4, R6, R7, R10, C8 and C9.
- **Writer turns.** Initialize the private session once in `Service.serve`. After each ready input frame give the hook one opportunity, as Part B already does. When a page has more but sends nothing, use the existing hub wake to schedule the next turn. Return `sent = true` only for an actual successful send. Reschedule after a discarded committed claim so later rows can progress. A callback from an ordinary Part B test hook retains the ordinary stream context. The production callback uses the tighter write context when `sendClaimed` supplies it. Before each turn and after the claim barrier check cancellation. No reader starts a send. The send uses the regular `transport.Send` wrapper.
- **Send fence.** After the durable claim, open a second short store transaction. Take the same machine row and recheck the exact connection and current approval. Read the row by identifier, machine, stream, attempt and null outcome. Recheck the strict 23-hour cutoff with a fresh database clock sample. Skip a finalized or time-ineligible row and leave its recorded attempt as it stands. The preflight plus its one transport write runs within a two-second context. Keep the PostgreSQL row lock through that single deadline-bounded send and its flush. A sweep, result, removal or replacement that wins before the second lock is visible at the recheck. If the send holds the lock first, the conflicting write can commit only after the send returns or is canceled. The transaction sends no feed rows and does no command update. It uses `Store.Write`, whose callback runs once. Keep a callback send failure local so the transaction can release the row before the stream handles that failure. An error after bytes may have reached the peer ends the stream and leaves the earlier committed attempt for retry on a new stream. Do not retry the callback.
- **Deadline calculation.** For a nonstate action, take a process monotonic clock sample before asking PostgreSQL for the authorization time. Add the positive difference between the stored send cutoff and the database sample to that process sample. Use the earlier of this deadline and the two-second operation deadline. If elapsed before dispatch, skip and wake. Item 4a applies that deadline to the actual write and retains cancellation for shutdown and the watchdog. Fail a partial, blocked or canceled write through the stream's normal terminal path. The database clock and monotonic interval assume no forward clock step during this bounded write. A new check uses the database's current time. The store and the server share the sprint stack clock. Frames that finished this send may still be in transit when later state changes commit.
- **Tests written first.** In `outbox_send_test.go`, `TestCommandSendRechecksBeforeWrite` pauses after claim commit. Commit a matching result, a replacement or removal, or mark the row expired through a locked fixture. Release the pause and observe no command frame. Use database-relative fixture expiry to cross the 23-hour boundary during a separate hold and observe a pending row with no frame. Verify a send before the cutoff and a matching result after it. Hold an admitted send with no completed write. Prove that a competing expiry/result or removal cannot commit before the send releases its row lock. After item 5 repeat the expiry case with `SweepCommands` in `TestCommandSendAndSweep` of item 8. `TestCommandSendDeadline` holds a real HTTP/2 write behind flow control. Observe release of the registry lock and stream termination at the shorter deadline, with the original attempt still pending. Cancel at the barrier in another run and observe the existing first terminal cause. `TestCommandBacklogYieldsToHeartbeats` supplies a full incoming buffer with the existing `streamControls.afterReceive` barrier and several pages of ineligible commands. Observe a heartbeat acknowledgement between successive command turns while the queue is full, then one eligible delivery. Test a full page with no eligible command and no new enqueue notice. Its own wake must continue the search.
- **Risk and mutations.** Risky because a stalled writer can outlive its attempt. Sending after the first commit without reacquiring the lock, dropping the outcome predicate, or releasing the lock before the callback makes `TestCommandSendRechecksBeforeWrite` fail. Discarding the shorter context at the call site fails `TestCommandSendDeadline`. Removing the continuation wake or looping through pages in one writer turn fails `TestCommandBacklogYieldsToHeartbeats`. The named tests pass in `./src/server/internal/link`.

**Design artifact.** The [frozen command comments](../../../src/contracts/proto/lgtm/link/v1/commands.proto), Part B's existing `CommandHooks` and [post-build contract check 11](../go-s0-contract/plan.md) define the send and result boundary. Round 4's earlier request to send after 23 hours is superseded by the frozen window and the updated check at 22 hours and 30 minutes.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An old queue hides an eligible command or starves input. | High | Use a bounded seek, resume from the examined position and exercise it with a saturated reader. |
| A claimed attempt starts after finalization, the cutoff or replacement. | High | Recheck under the same lock as the competing writers and bound the actual response write. |
| A blocked send holds a database writer slot. | Medium | Apply a two-second ceiling to the second transaction and verify release after cancellation. |

**Verify.** Run the named PostgreSQL and HTTP/2 tests, including `TestClaimBoundsIneligiblePrefix`, `TestSendUsesEarlierContextDeadline`, `TestCommandSendRechecksBeforeWrite` and `TestCommandBacklogYieldsToHeartbeats`. The combined receipt replay checks use the Sprint 3 client and a controlled loopback proxy. No device is needed for server acceptance.

**Rollout detail.** In step 2 run item gates against the upgraded fixture. In steps 3 and 4 run the merged `./src/server/...` suite, then the named combined tests in the last workstream.

**WS-3: The owner can approve, rename, scan and restart a machine**

The owner calls use the existing authenticated routes. They save the command in the same retryable write as their reply. The connection must be live for scan and restart. Approval and rename also work with no live connection.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 6 | Approval and a changed rename each save one notice and keep the first keyed reply. | `TestApproveQueuesStateAtomically`, `TestRenameQueuesStateOnce`, `TestKeyedCallsReplay` |
| 7 | Scans require an online machine. Restarts accept a joining or online approved machine while its exact stream is live. | `TestScanAndReconnectQueueCommands`, `TestActionRequestKeyReplays` |
| 8 | A waiting machine receives approval on its existing stream. A finished restart replays its result after the next connection. | `TestServerRoutesStoredCommands`, `TestLinkSameStreamApproval`, `TestLinkRestartReplaysAfterDroppedResult` |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 6 | Save approval and rename notices with their owner writes. | `src/server/internal/machines/records.go`, `ApproveClient` and `RenameClient`, and `src/server/internal/machines/link.go`, `Link`. Patterns S11 and S12. |
| 7 | Implement scan and restart acceptance. | `src/server/internal/machines/commands.go`, `ScanClientHarnesses` and `ReconnectClient`. Pattern S11. |
| 8 | Activate the real route and sweep worker and prove the composed lifecycle. | `src/server/internal/app/start.go`, `Start`, and `src/server/internal/link/run.go`, `Service.Run`. Patterns S9 and S13. |

**Item 6. Approval and rename notices**

- **Files and names.** Change `src/server/internal/machines/records.go`, `link.go` and `service.go`. Create `src/server/internal/machines/state_commands_test.go`. Add a machines-owned `CommandQueue` interface with `Enqueue(context.Context, *store.Tx, string, *linkv1.Command, time.Duration) error`. Add `(*Service).WithCommands(queue CommandQueue) *Service` and optional private `Service.commands`. These are the additions to the Part A service file and the Part B link interface file. Keep `Link`, `WithLink` and the existing constructors and test doubles intact. Add private `Service.enqueueStanding(ctx context.Context, tx *store.Tx, machine Machine) error` to the Part A records file. The two handlers keep their names, wire replies and Part B removal and presence edits.
- **Dependencies and size.** Needs item 1, Part B item 9 and Part A's persisted request handling. Can run alongside items 2 and 5, and alongside items 3, 4a and 4b once ready. Work area `backend`, complexity medium, scope `server/backend`. Two existing handlers reuse an optional interface and share one transaction boundary. Statements D5, D7, R3, R4, R6, R8, R10 and C7.
- **Approval.** Preserve name normalization before `recordRequest`. Within the existing `Store.WriteOnce` callback, set approved, read the updated machine in the same transaction and enqueue `SetState{state: Standing(machine)}` for one hour. Build the returned projection with Part B's `projectOne(ctx, tx, machine)` in that callback. Save those bytes with the approval and notice. Propagate enqueue or projection failure so the write rolls back. A fixture constructed without `WithCommands` keeps the current record-only behavior. Production supplies the queue in item 8.
- **Rename.** Lock the machine before testing its standing. Refuse waiting with `errRenameWaiting`. Use `SetName` and its change flag. If changed, read the updated row and enqueue its new standing. If unchanged, preserve the revision and do not enqueue. Use `WriteOnce` through the built receipt helper with the existing version 1 fingerprint and the frozen empty `RenameClientResponse`. Keep compatibility with saved empty replies. The same normalized name and request key can replay after a later rename or removal. It cannot re-run the callback. A stale retry for a different body gets the existing key conflict.
- **Tests written first.** `TestApproveQueuesStateAtomically` uses a real repository, an authenticated owner route and the real queue. Approve a waiting machine with a name padded by spaces. Observe the trimmed name, revision 2, one `set_state` containing approved standing with that revision, a machines reference and one outbox reference. Fail enqueue or fail the enclosing commit and observe the original waiting record and no new receipt or feed references. Drop a successful reply, rename and remove the machine, then replay the approval key. Observe the original reply bytes and no new notice. Preserve the frozen missing, taken, invalid and already-approved errors. `TestRenameQueuesStateOnce` observes that the changed name gets the next revision in exactly one command. Same-name rename leaves the count and revision unchanged. A keyed replay after a later rename returns the saved empty response and enqueues nothing. Check waiting refusal, absent machine and rollback on queue error. Hold an approval after it acquires the machine lock and before commit. Start rename while the committed row still says waiting. Release approval and observe rename commit with the next revision. Preload a receipt in the format saved by Part A and assert replay leaves the command table empty. Use the existing `TestApproveClient`, `TestRenameClient`, `TestKeyedCallsReplay` and Part B's `TestWatchClientsStreamPresence` as regression gates.
- **Risk and mutations.** Risky because approval controls which machines can act and the reply is replayed. Moving either enqueue after commit or building the approval reply outside `WriteOnce` fails `TestApproveQueuesStateAtomically`. Skipping the change flag, the row lock or the keyed callback for rename fails `TestRenameQueuesStateOnce`. The named tests pass in `./src/server/internal/machines`.

**Item 7. Scan and restart calls**

- **Files and names.** Change `src/server/internal/machines/records.go` to move the two existing stub methods. Create `src/server/internal/machines/commands.go` and `src/server/internal/machines/commands_test.go`. Keep the `ScanClientHarnesses` and `ReconnectClient` method signatures and the refusal sentences. Declare private `Service.queueConnectedAction` in the new file. Remove the old `refuseOfflineCall` after moving its refusal behavior. Add no declaration to Part A's records file. Move any exclusively used refusal errors to the new file with their names intact.
- **Dependencies and size.** Needs item 6 for the queue field and its prior edit to `records.go`, and Part B presence. It can run alongside items 4b and 5. Work area `backend`, complexity medium, scope `server/backend`. Two routes share one eligibility and retry path. Statements D16, R3, R4, R6, R8 and R10.
- **Calls.** Use `recordRequest` and `writeRecordChange` with the existing procedure and version 1 id fingerprint. Inside the callback lock the machine and load `Link.Presence` using the same `*store.Tx`. Scan requires approval, a current row, its exact `Held` value, and both report flags. Restart requires approval and the same current live row, including a joining machine. The liveness decision is the `Held` observation under the row lock. A subsequent stream failure leaves the saved action pending for retry. Absent machine maps to `not_found`. Known ineligible scan and restart map to `failed_precondition` with `This machine is not online.` and `This machine is not connected.`, respectively. Nil link or queue gives the same refusal for a known machine and does not create a receipt. Declare both refusals with `store.NewDomainError` so they survive transaction error sanitization. Map them in the existing Part A `recordError` branch for failed preconditions. The keyed conflict remains `already_exists`, and a retryable database lock error remains `aborted`. Enqueue `ScanHarnesses` or `Restart` for two minutes. Return the frozen empty response.
- **Tests written first.** `TestScanAndReconnectQueueCommands` exercises missing, waiting, offline, joining and online. Use both report flags separately. Cancel the live handle, force the delete to fail and exercise both calls against the retained presence row. Replace A with B and verify that A's hold cannot authorize a call against B's row. No refusal writes a queue row or a receipt. Online accepts one command of each kind. Joining accepts restart only. Verify the two-minute expiry from `created_at`. Keep `TestScanAndReconnectRefused` for the unconfigured fixture. `TestActionRequestKeyReplays` races two identical requests with a key through the authenticated route and observes one command and one receipt. Replay after the stream ends and compare the original response. A different request body with that key conflicts. Hold the machine lock long enough to observe the existing retryable error and verify the request can later succeed with the same key and a single command.
- **Risk and mutations.** Risky because actions must use current liveness and one command per keyed request. Removing one report flag, trusting a stale row or allowing a waiting machine to restart fails `TestScanAndReconnectQueueCommands`. Checking presence before keyed replay or using an unkeyed write fails `TestActionRequestKeyReplays`. Both tests and the frozen refusal test pass in `./src/server/internal/machines`.

**Item 8. Server composition and production-route checks**

- **Files and names.** Change `src/server/internal/app/start.go` and `src/server/internal/link/run.go`. Create `src/server/internal/app/commands_test.go` in package `app_test` and `src/server/internal/link/outbox_integration_test.go` in package `link`. Update the existing application construction to pass `Options.Commands: state` and `WithCommands(state)`, alongside `WithLink(state)`. Attach `State.RunCommandSweep` to the existing `Service.Run` lifetime only for production's concrete state composition. Use a private type assertion to `*State` on the hook value so Part B's fake hooks keep their two-method contract. Start and join the worker with the feed and cleanup workers. Add no name to Part A's start file and no field to the public fixture. Use Part B's existing startup and fixture seams.
- **Dependencies and size.** Needs items 2, 4b, 5 and 7 and Part B item 10. Server tests use only server-owned code. The combined-suite writer can prepare the requested cases while these items build. Work area `backend`, complexity medium, scope `server/backend`. This composes existing components and verifies the route and lifecycle they expose. Statements D1, D3, D5, D7, D16, R1, R4, R6, R7, R8, R10 and C7 through C12.
- **Composition.** Run under the existing startup lifetime and instance lock. Apply migration 2 before starting the listener. Preserve Part B's primary shutdown cause and cancellation of live streams before waiting for workers. A sweep failure reports a fixed diagnostic and does not stop stream shutdown. Construct the same route from `servertest.Start` and after `Server.Restart`. Keep owner-session checks on the four owner calls. The command frame uses the machine link and its existing authenticated stream.
- **Tests written first.** `TestServerRoutesStoredCommands` uses the actual app listener and scripted machine. Use a waiting stream and a signed-in watch, approve the machine and observe `SetState` on that exact stream. Confirm joining, then send approved facts, harnesses and load and see online. While waiting, a machine token or absent owner session cannot enqueue an owner action. A signed-in online scan produces the command frame, and its report changes the open watch. Leave a restart pending through server shutdown and reconnect the same machine. Observe the same command identifier, next attempt where previously claimed, unchanged expiry, and the matching outcome in PostgreSQL. `TestServerCommandSweepLifecycle` starts the server to apply migration 2, stops it, and seeds due rows before restart. Observe the initial real sweep without expiry of a pending `SetState`. In separate runs, observe startup and shutdown while a test gate holds the sweep. Check released instance lock and surviving commands on restart. In `outbox_integration_test.go`, `TestCommandSendAndSweep` lets a real sweep win during the pause after claim, then observes no command frame or overwritten expiry. In the inverse order, hold the fenced write, observe that a busy sweep skips the locked row, release the send and advance the fixture age. A later sweep finalizes only while the row is pending. `TestCommandResultAndRemoval` makes the real result writer win once and the real removal win once. Use the machine lock or a scoped commit gate to establish order. Inspect first outcome, finish time and the removed watch record. Admit no further command or matching result after removal.
- **Risk and mutations.** Risky because this enables a remote owner action over a machine stream. Omitting the session guard, queue or command hook fails `TestServerRoutesStoredCommands`. Dropping the worker or ignoring cancellation fails `TestServerCommandSweepLifecycle`. Moving the send outside its lock or finalizing a row after a matching result fails `TestCommandSendAndSweep`. Letting removal overwrite a committed result or accepting a result after removal fails `TestCommandResultAndRemoval`. The named tests pass in `./src/server/internal/app` and `./src/server/internal/link`. Keep `TestWebRoutesRequireSession` and `TestRouterWrapsLongStreams` in the merged suite.

**Design artifact.** The [Machines route contract](../../../src/contracts/proto/lgtm/v1/clients.proto) and [Part C of the handoff](../go-s3-server-machines/handoff-stream-and-commands.md) specify the owner calls. This workstream delivers server behavior through the existing routes. The owner-facing menu and any website revision change are a separate, owner-gated request in choice 6.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An offline or waiting machine takes work. | High | Authorize through the exact held current connection and test the refusal cases on the route. |
| Approval on an existing connection leaves the machine waiting. | High | Check the notice and both approved reports on the same stream through the generated clients. |
| A lost reply duplicates a command. | High | Observe the first stored reply and exact command count after a concurrent retry and replay. |

**Verify.** Run each item against PostgreSQL, including `TestApproveQueuesStateAtomically`, `TestActionRequestKeyReplays` and `TestServerRoutesStoredCommands`. Then run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...`. The Sprint 3 gate uses the loopback stack and a disposable installation of the real client. The Linux service check belongs to Stack. No device is needed for the server-only tests.

**Rollout detail.** In step 1 run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...` on the Part A and Part B merge and retain the exact commit and report. In step 3 start the built stack through `deploy/up.sh` and confirm migration 2 applied. In step 4 run `deploy/test/checks.sh --only C5,C6,C7,C8,C9,C10,C11,C12` with the Sprint 3 fixture and require each check to pass. Run the combined tests against real PostgreSQL after both ends land. Use `servertest.Start`, `Clients`, `Link`, `Environment`, `Stop` and `Restart`. Use the planned `clienttest.Join` and `Run`.

| Deployed check | Contribution from this plan |
|---|---|
| C5 and C6 | Part A owns token issue and waiting records. Run both as regression checks. |
| C7 | This plan delivers the live approval notice and supplies the server half of approval on the existing connection. The current script starts a connection after approval, which Part B and the client otherwise supply. The combined test is required to prove the notice. |
| C8 and C9 | Part B and the client own normal presence and reconnect. Exercise those paths again with commands enabled. Restart through the owner call has a separate Stack service check. |
| C10 | This plan ends pending commands within removal. Part B owns the refusal of the removed identity. The client owns refusal of a mismatched server key. |
| C11 | Part B, the client and Stack own ordered event replay. Keep the existing sprint gate and Stack's durable event-mark request. Command retry evidence is separate. |
| C12 | This plan adds real PostgreSQL regressions to the root suite. The Store and Stack teams own the independent mutation that removes the feed lock. |

**Combined acceptance requested from the orchestrator**

| Check | Observation |
|---|---|
| `TestLinkSameStreamApproval` | Start a real waiting client, assert a waiting Welcome and no scan or restart frame, and use the owner calls to verify waiting refusals. Approve while it is connected. Observe a `SetState` and a result on that stream. Hold the approved report frames in the proxy until the watch shows joining. Release them and observe fresh approved facts and harnesses, then online within 30 seconds of approval at the same generation. Repeat with an already-applied state and a notice past its expiry. Observe one final outcome and no additional report caused by reapplying the same revision. |
| `TestLinkCommandAttemptsAcrossReconnect` | Hold a requested scan while cutting the stream, and reconnect on the same client state. Capture attempt and stream identity on both deliveries. Allow the newer connection to report inventory before releasing the scan. Observe the client plan's failed result on the latest attempt in that fixture, one stored final result, and success for a later scan. Run the completed-scan variant by dropping the first result after its receipt is durable, restarting the client and capturing the replayed result. Count requested scan effects with a fixture version probe distinct from the initial scan. |
| `TestLinkRestartReplaysAfterDroppedResult` | Drop the first result after the restart receipt commits. The launched client returns 75 exactly once. Relaunch it with the same directory, then observe the retry's unchanged identifier and expiry, incremented attempt and replayed `done`. Require no second exit 75. The Linux installer gate `live_reconnect_restarts_service` observes the new process and one service restart. |
| Contract checks 11, 19 and 26 and the inherited P5 | In a disposable fixture record a real completed receipt and drop the result. Stop both writers before aging fixture rows. Keep the same identifier and age the server row and on-disk receipt to the same expiry relative to the database clock for replay at 22 hours and 30 minutes. Use a loopback proxy to delay the next Welcome eight seconds. Verify the retry's persisted expiry equals the aged fixture expiry and it replays the saved receipt. Use the unmodified-expiry test above and the server claim test to prove expiry is immutable in production. Another pending action aged past 23 hours is not sent. Exercise the client receipt-clock boundary with the client team, including a two-hour-fast clock before and after a pruning write and the 59-minute-59-second case with the stated hop delay. With the client team's controlled wall clock, a saved receipt replays `done`. A pruned receipt for already-expired work reports `expired` with no repeated effect. Require a late matching result to win before the sweep and to lose after the terminal update. |
| Contract checks 6 and 20 | Keep the client's existing start-time and interruption suite. Verify its latest-attempt closure against the real server. Use the client's clock fixtures for delayed Welcome, expiry with a five-minute-slow machine and recovery after a start without a final receipt. |
| Contract checks 9 and 15 | Reuse the existing token and session-boundary fixture. Use a scripted machine for reserved-field and invalid result messages the real client cannot produce. Scan the test database, captured logs and returned watch frames for the marked reserved field while observing the valid result commit. |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is the foundation design and phase 4 of the build loop, Part C and its named decisions in the [handoff](../go-s3-server-machines/handoff-stream-and-commands.md), items 6 and 13 and P3 and P5 in the [reviewed draft](../go-s3-server-machines/attack/plan-v4.md), the command-specific findings in the four draft registers, request 4 in the [stream result](../../../.foundry/go-rewrite/results/plan-go-s3-server-stream.md), the frozen command and opening comments and code table, the client plan's command and fixture items, choices 1, 3, 6 and 12, issue 1632.3 and issues 1633 through 1635, and deployed C5 through C12.

| Source item | Workstream | Status |
|---|---|---|
| Reviewed item 6, rows 30, 40, 46, 47, 57, 58, 66, 70, 71, 75, 82 and 83 | WS-1, WS-2 | Items 1 through 5, with 4a and 4b. The updated frozen attempt and timing rules govern. |
| Reviewed item 13, rows 38, 61, 63, 79, 85 and 89 | WS-3 | Items 6 and 7 keep reply, eligibility and keyed semantics. |
| Handoff rows 36, 41, 65, 77, 78, 87 and 88 | WS-1, WS-2, WS-3 | The scripted fixture, exact current stream, split and declaration rules are build prerequisites. |
| Handoff rows 11, 20, 42 through 45, 55, 67 through 69, 80, 81 and 86 | WS-2, WS-3 | Part B owns waiting reports, protocol errors, deadlines, ordering and shutdown. This plan exercises the existing boundaries. |
| D1, D3 through D5, D7, D16, R1 through R10 and the build loop's phase 4 | WS-1, WS-2, WS-3 | Each item cites its statements. The common checks freeze tests, name mutations and use the real database. |
| Draft review rounds 1 through 3 | WS-1, WS-2, WS-3 | Items 1 through 7 carry the settled ordering, result matching, wake and receipt decisions. Combined P5 carries the fast-clock residual. |
| Draft round 4, architect-r4 05, logic-r4 04 and logic-r4 05 | WS-2 | Items 3, 4a and 4b bound the prefix and fence the send. The updated contract check settles the send-window conflict. |
| Stream result request 4 and stream finding critic-r1 10 | WS-1, WS-2, WS-3 | Items 1 through 8, the same-stream approval gate and the waiting case. |
| Frozen `commands.proto`, `link.proto`, `events.proto` and `link-codes.json` | WS-1, WS-2, WS-3 | The command state machine, current stream, reason codes and report gate follow revision 1. |
| Contract checks 6, 9, 11, 13, 15, 19, 20, 26 and 30 | WS-1, WS-2, WS-3 | Server and combined checks cover the relevant queue and result decisions. Client-side start-clock behavior and optional removal-notice handling remain with the client. |
| Client plan items 6, 7b, 8 and 9, reviewed P3 and P5 | WS-2, WS-3 | Command attempts, restart, fast clocks and stale presence map to the named combined and service gates. |
| Owner choices 1, 3, 6 and 12 | WS-1, WS-2, WS-3 | Keep credential calls unkeyed, keep machine-clock receipt pruning, implement connected restart on the server and preserve the accepted degraded presence case. |
| Issue 1632.3 and issues 1633 through 1635 | WS-2, WS-3 | Use connected restart, reuse the stream and proxy boundary, request combined tests and size the work in units. |
| Deployed C5 through C12 | WS-1, WS-2, WS-3 | Ownership and the exact rollout command appear in WS-3. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository rule | How the plan satisfies it |
|---|---|
| `AGENTS.md`, What this project is | The plan changes the Go server and uses generated message types at its boundaries. A separate owner request covers the website. |
| `AGENTS.md`, Where things are | New implementation lives under `src/server/`. The established Protocol Buffers files are already frozen. The old engine and sample server are references. |
| `AGENTS.md`, Habits 1 through 4 | Reuse repository code. Record unresolved decisions and requests in the scoped planning artifacts for the orchestrator. Build integration follows the epic's pull request path. No push goes to the reference project. |
| `AGENTS.md`, Writing style for documents | Record dated rulings, use relative file paths and validate with the repository privacy rules. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | migrations | server/backend | medium | One additive migration and one atomic enqueue. | No | Part B migration 1 and registry |
| 2 | backend | server/backend | medium | One guarded result update and one stream check. | Yes | 1 |
| 3 | backend | server/backend | medium | One bounded seek and a committed attempt. | No | 1, 2 |
| 4a | backend | server/backend | medium | The response controller honors the earlier deadline. | Yes | Part A deadline middleware |
| 4b | backend | server/backend | medium | One bounded send uses a second authorization check. | Yes | 3, 4a and Part B writer |
| 5 | backend | server/backend | medium | One end-of-life path for due and removed commands. | Yes | 1 and Part B removal |
| 6 | backend | server/backend | medium | Two existing keyed handlers add one transactional notice each. | Yes | 1 and Part B owner projection |
| 7 | backend | server/backend | medium | Two existing routes share one current-stream check. | Yes | 6 |
| 8 | backend | server/backend | medium | Existing app and worker entry points compose the completed units. | Yes | 2, 4b, 5, 7 and Part B application |

Load. The assignment uses one vendor for plan review, tests, builds and post-build review. Reserve four plan-review launches, nine test-writer launches, nine builder launches and nine post-build review launches on that vendor, for 31 first-pass launches. The orchestrator checks the remaining weekly allowance before dispatch. If capacity is below 31, schedule the later units and reviews in the next allowance window. The separate test writer, builder and reviewer seats provide independent passes within the single-vendor assignment. The post-build review allocation includes the named mutation evidence. Rework launches and combined tests are scheduled separately.

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings (blocker/major/minor) | Outcome |
|---|---|---|
| Inherited review of Part C | See the four source registers. | Decisions, dates and carried findings are linked in Traceability. The bounded prefix, held send and old check 11 timing have explicit dispositions and acceptance tests. |
| This plan's review, round 1 | Pending. | The bounded planning assignment ends at `attack`. The orchestrator owns this review and its findings register. |

</details>
