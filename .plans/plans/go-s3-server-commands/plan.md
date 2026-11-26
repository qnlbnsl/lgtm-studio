---
topic: go-s3-server-commands
status: done
tier: M
created: 2026-11-26
updated: 2026-11-26
epic: go-rewrite
---

# Stored commands for machines

## Overview

The server will save an approval, rename, scan or restart command before asking the machine to act. A machine that is already connected when the owner approves it will receive the approval on that connection. The machine can then report its facts and harnesses and appear online without reconnecting.

When a connection drops after a scan or restart, the next connection can carry another attempt of the same command. The machine can answer from its saved result. The server accepts a result only after the attempt has been written and flushed on the current connection. A queue of commands that can no longer be sent leaves the connection time to answer heartbeats.

The server will accept restart requests for approved machines with a live connection and scan requests for machines that have finished reporting on that connection. The website needs to acknowledge a queued scan and update inventory when the watch reports it. Moving the restart action to the connected Machines menu and updating the website contract revision await owner choice 6.

## Key decisions

The approval or rename and its notice commit together. If the owner retries a successful request with the same request key, the server returns the original reply without adding another command.

A scan or restart can be retried before 23 hours past expiry. A matching result can close it until the server records a timeout after 24 hours. The machine prunes receipts with its own clock. At most once holds while that clock leads by less than an hour minus delivery and lookup time. The accepted first-release limit allows a repeat after pruning on a clock more than a day fast. Approval and rename notices keep their revision and remain pending until answered or removed.

If the database temporarily cannot clear a closed connection, the page may still show the machine online. A new scan or restart request checks that the server still holds the exact connection.

[Full decision log](decisions.md)

## Workstreams

### The server keeps a record of each command

Each command keeps the same identifier and expiry across all of its attempts. Removing a machine ends its pending commands in the same write. The stored result shows a single final disposition even when a result and the expiry job race.

### The connection sends commands and continues serving reports

The server takes the oldest eligible command in small steps. It checks the stored attempt again before writing the frame. A second scan waits while an earlier scan can still run, and a restart can pass it. The server limits how many blocked sends can hold database slots so other machines can still record and acknowledge heartbeats.

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

D1 to D23, R1 to R11 and C1 to C13 identify decisions, rules and checks in the [foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md). WS means workstream. A deployed check is a function in `deploy/test/checks.sh`. A contract check is a post-build row in the [contract plan](../go-s0-contract/plan.md). Its frozen wire is Protocol Buffers link revision 1. The schema below uses PostgreSQL Structured Query Language (SQL). UUID means universally unique identifier. HTTP means Hypertext Transfer Protocol. All file paths within a lane's item table are rooted at the repository.

Freeze the tests after an assertion-red run with the item’s stubs. Run each mutation separately. Record its named failing assertion and restore the code. Use an anchored filter, for example `deploy/test/with-postgres.sh -- go test -json -race -count=1 -run '^(TestAcceptResultRules|TestResultAndReplacementFormOneLine)$' ./src/server/internal/link`. Require exit zero and a test-level pass event for each name. Fail on missing or skipped tests. JSON means JavaScript Object Notation. (critic-r1 07) Owner tests importing link use `machines_test`.

Use `newPresenceFixture` except for the upgrade case. Inspect committed rows from a separate connection. For a held commit use `pgx.QueryTracer` at `COMMIT`. Release it to commit or terminate its database backend to roll back. Use `checkOneLine` and observe the lock wait for same-machine writers. Keep the Part B suite's existing test names.

Acquire the request key or revocation token row before the machine row, then lock link rows. Removal already owns the machine row. Report enqueue with a feed reference. Guarded attempts, dispatch and finalization are internal Store writes without a watch consumer.

Keep the stream’s first terminal cause. Map `ErrUnknownMachine` to `failed_precondition` with `unknown_machine`, `ErrNotCurrent` to `aborted` and `ErrInvalidMessage` to `invalid_argument`. Store failures on Open return `unavailable`. Keep diagnostics fixed. [Part A and Part B handoff](../go-s3-server-machines/handoff-stream-and-commands.md)

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

- **Files and names.** Change `src/server/internal/link/presence_test.go` so `TestBecomeCurrent` requires ledger revision 2 and retains both existing table and admission assertions. Item 1 also owns `src/server/internal/link/migrate.go` if its embed still names revision 1 alone. Retain `Migrations` and use the migration glob. (architect-r1 01) Create `src/server/internal/link/migrations/0002_link_commands.sql`, `src/server/internal/link/outbox.go` and `src/server/internal/link/outbox_test.go`. Declare `(*State).Enqueue(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) error`, `outboxTopic = "machine_outbox"`, `CommandGrace = 24 * time.Hour` and `CommandRetention = 7 * 24 * time.Hour`. Add no name to a Part A file. The existing `Migrations()` embeds the new file.
- **Dependencies and size.** Needs Part B migration 1 and its passing registry and link gates. The test writer for item 2 can prepare its cases while this item builds. Work area `migrations`, complexity medium, scope `server/backend`. The additive change needs an upgrade fixture and an atomic enqueue. Statements D4, D5, R1, R3, R4, R6, R7 and R10.
- **Schema.** `link_commands` has `id text PRIMARY KEY`, `machine_id text NOT NULL`, `action text NOT NULL`, `payload bytea`, `created_at timestamptz NOT NULL`, `expires_at timestamptz NOT NULL`, `attempt bigint NOT NULL DEFAULT 1`, `sent_stream_id text`, `dispatched boolean NOT NULL DEFAULT false`, `finished_at timestamptz`, `seq bigint GENERATED ALWAYS AS IDENTITY UNIQUE` and `outcome text`. Attempts range from 1 to 4294967295. A null stream requires attempt 1 and `dispatched = false`. Item 3 resets the marker on claim, item 4b sets it after flush and item 2 requires it. UNREVIEWED MECHANISM (architect-r1 02). A write entry marker could survive a later flush failure. (critic-r1 01, logic-r1 03, threat-r1 01)
- **Storage constraints.** Actions are `scan_harnesses`, `set_state` or `restart`. Machine outcomes require dispatch and use `done`, `failed`, `expired`, `unsupported` or `interrupted`. The sweep alone writes `timed_out` for a missing result. Removal writes `removed`. Item 2 writes `expired` for an explicit machine refusal. The result and maintenance tests read both meanings. UNREVIEWED MECHANISM (logic-r1 06, threat-r1 04). Pending rows require a payload, null outcome and null finish time. Terminal rows require the inverse. State notices cannot expire or time out. Keep terminal rows without a foreign key to a live machine or stream.
- **Indexes.** Use `link_commands_pending (machine_id, seq) WHERE outcome IS NULL`, `link_commands_expirable (expires_at, machine_id, seq) WHERE outcome IS NULL AND action <> 'set_state'`, and `link_commands_finished (finished_at, machine_id, seq) WHERE outcome IS NOT NULL`.
- **Enqueue.** The caller owns the machine lock and transaction. Generate the identifier with `store.NewID`. Rebuild the typed action from declared fields only, with approved standing, the normalized name and positive revision for `SetState`. Reject missing or invalid actions and lifetimes. Write nonnull payload bytes, including for an empty action. Sample the database clock once for creation and add the supplied lifetime for expiry. Insert attempt 1 with no assigned stream. Record `store.Saved` on `machine_outbox` with empty scope and the machine identifier as key. Propagate errors to the enclosing write.
- **Tests written first.** `TestCommandMigrationUpgradesPartB` opens the machines manifest and a link manifest with `0001_link.sql` alone. Seed a waiting machine, a connected machine, a keyed reply and an event mark. Apply the full manifest twice. Require revision 2, an empty queue and identical earlier records. Require the old manifest to be refused and the full one to start again. Assert separate constraint errors for attempts 0 and 4294967296 and for each mismatched outcome and finish-time pair. (logic-r1 05)
- In `TestEnqueueStoresBeforeSend` enqueue a two-minute scan. Observe a version 7 identifier, creation plus two minutes, attempt 1, no stream, the declared-only action and one wake. Add stray envelope and unknown fields, then submit a missing action separately. Reject the latter and store no extras for the former. Reject a real commit and require no queue, feed or caller changes.
- **Risk and mutations.** Risky because queue order and visibility depend on one write. Moving the insertion to a separate committed write or skipping the feed reference fails `TestEnqueueStoresBeforeSend`. Removing either the attempt bound or the terminal-pair check fails its own assertion in `TestCommandMigrationUpgradesPartB`. Run the entire `./src/server/internal/link` package on the item 1 merge candidate, including `TestBecomeCurrent`. (architect-r1 01, logic-r1 05)

**Item 2. Match results to the current attempt**

- **Files and names.** Create `src/server/internal/link/outbox_result.go` and `src/server/internal/link/outbox_result_test.go`. Implement `(*State).AcceptResult(ctx context.Context, machineID, streamID string, result *linkv1.CommandResult) error` with the frozen `CommandHooks` signature. Define private `lockCommandStream(ctx context.Context, tx *store.Tx, machineID, streamID string) (machines.Machine, error)` on `State` for this item and items 3 and 4. It calls `Registry.Lock`, `State.locked` for the existing lock barrier, and checks the exact connection row in that transaction. Add no name to a Part A file.
- **Dependencies and size.** Needs item 1 and merged Part B. The item 5 builder can work in parallel after item 1. Work area `backend`, complexity medium, scope `server/backend`. This is one guarded update with a finite outcome table. Statements D7, R3, R4, R6, R7, R9 and R10.
- **Result rules.** Apply the existing fence error for missing or displaced streams. Ignore a current waiting machine’s result. For approved machines require a canonical identifier and nonzero attempt, or return `ErrInvalidMessage`. Reuse `canonicalUUID`. Match `id`, `machine_id`, `sent_stream_id`, `attempt`, `dispatched = true` and null outcome. (architect-r1 02, critic-r1 01, logic-r1 03, threat-r1 01) Read only the three declared fields. Normalize unknown, unspecified and `SetState EXPIRED` to `failed`. Store one outcome and database finish time, and clear the payload. A matching dispatched attempt can finish after either cutoff while pending. Ignore duplicates and mismatches. Return no payload or feed reference.
- **Tests written first.** `TestAcceptResultRules` seeds attempts and stream associations. Ignore attempts 1 and 3 for dispatched attempt 2, a missing command, a different machine and a new current stream before claim. Separately give A’s command B’s exact stream and dispatched attempt. B’s matching result leaves A’s row pending. (architect-r1 06) Require a stale-stream error from the old stream. Cover malformed waiting and approved results and each outcome normalization. Only a dispatched claim can close. A reserved field 4 marker reaches no database, log or website output. Accept a matching result past both cutoffs while pending. A duplicate preserves the original finish time and adds no feed row.
- **Ordering test.** `TestResultAndReplacementFormOneLine` uses `checkOneLine`. A result under the lock can commit before replacement. When replacement wins, the old result must fail the fence and leave the command unchanged.
- **Risk and mutations.** Risky because an unrelated or earlier attempt must not close a command. Dropping any of the machine, stream, attempt, dispatch or pending predicates fails `TestAcceptResultRules`. Using a pre-lock stream lookup in place of the in-transaction check fails `TestResultAndReplacementFormOneLine`. Both tests pass in `./src/server/internal/link`.

**Item 5. Expiry, removal and retention**

- **Files and names.** Create `src/server/internal/link/outbox_sweep.go` and `src/server/internal/link/outbox_sweep_test.go`. Declare `(*State).SweepCommands(ctx context.Context) error` and `(*State).RunCommandSweep(ctx context.Context, report func(error))`. Change only the body of `State.Forget` in `src/server/internal/link/presence.go`. Extend that existing Part B method without a new name in a Part A file. Applications call the worker method from item 8.
- **Dependencies and size.** Needs item 1 plus Part B removal. Can run alongside items 2, 3 and the calls as their dependencies permit. Work area `backend`, complexity medium, scope `server/backend`. The item handles a bounded maintenance write and a pre-existing atomic removal. Statements D3, R3, R4, R6, R7 and R10.
- **Maintenance.** Run at startup and hourly. Sample database time once per pass. Read due references by keyset from the expiry and finished indexes in pages of 64. Group by machine and use one `Registry.TryLock` per machine write. Recheck state and time in that transaction. Skip PostgreSQL lock refusal `55P03` until the next pass. Time out only pending scans and restarts strictly past expiry plus 24 elapsed hours. Set outcome and finish time and clear the payload. Prune terminal rows strictly older than seven elapsed days. A missing machine permits only eligible terminal cleanup. Check cancellation between writes. Report other failures with the fixed diagnostic and retry on the next pass.
- **Removal.** In the caller's existing removal or waiting-token revocation transaction, `State.Forget` finalizes pending commands as `removed`, sets their finish time and clears their payload. It keeps any outcome already stored and deletes the connection and mark through the existing statements. The registry deletion and the machines feed are in that transaction. There is no removal frame to deliver and no new receipt or wake reference from finalization.
- **Tests written first.** `TestSweepCommands` times out an action aged 25 hours and clears its payload. Keep the action at 23 hours, an old state notice and a previously recorded result. Prune a terminal row aged eight days and retain one aged six. Observe no feed reference or new finish time on repeat.
- `TestSweepReleasesMachineBetweenBatches` holds A’s lock while due work for B finishes. Trace the lock refusal and maximum 64 candidates per write. Cancel with work remaining and finish it on a fresh pass. `TestCommandRemovalIsAtomic` removes approved and waiting machines through the actual routes. Fail the enclosing commit and observe registry, queue, link, feed and receipts unchanged. Commit and observe ended commands with cleared payloads, preserved earlier outcomes and eventual pruning. Revoking an approved machine’s redeemed token leaves commands unchanged. `TestCommandSweepStops` observes the initial pass, a private hourly tick and cancellation at a database gate.
- **Risk and mutations.** Risky because a competing result, sweep and removal must agree on one terminal state. Expiring `SetState` or dropping the pending guard fails `TestSweepCommands`. Expiring without a machine lock or using an unbounded single-machine update for maintenance fails `TestSweepReleasesMachineBetweenBatches`. Omitting finalization on waiting removal or moving it out of the removal write fails `TestCommandRemovalIsAtomic`. Leaving the worker alive after cancellation fails `TestCommandSweepStops`. All four tests pass in `./src/server/internal/link`.

**Design artifact.** The [frozen life of a command](../../../src/contracts/proto/lgtm/link/v1/commands.proto) and Part C of the [handoff](../go-s3-server-machines/handoff-stream-and-commands.md) describe the storage and removal behavior.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An acknowledged owner write has no saved action. | High | Gate storage and feed on the same real commit. |
| A later or unrelated result overwrites the final result. | High | Use the machine lock, the exact-attempt update and two competing transaction orders. |

**Verify.** Run the named PostgreSQL tests, including `TestCommandMigrationUpgradesPartB`, `TestAcceptResultRules`, `TestCommandRemovalIsAtomic` and `TestCommandSweepStops`. No device is needed.

**Rollout detail.** Pin the passing Part B commit and inspect its embed. Apply migration 2 at startup and in the upgrade fixture. Its ledger entry prevents startup with revision 1 alone. Repair forward with both migrations and the existing queue.

**WS-2: The connection sends commands and continues serving reports**

The writer spends a bounded turn looking for one command. It records an attempt before sending. Immediately before sending it checks that the attempt is still pending on the current connection, and it gives the write a deadline.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3 | A claim returns one oldest eligible command after commit and yields while passing an ineligible prefix. | `TestClaimCommitsBeforeReturn`, `TestClaimRaisesAttemptOncePerStream`, `TestClaimOrder`, `TestClaimBoundsIneligiblePrefix` |
| 4a | The response writer honors an earlier deadline and returns an error when the link flush fails. | `TestSendUsesEarlierContextDeadline`, `TestLinkSendReportsFlushFailure` |
| 4b | An unsent attempt cannot close. Held writes leave capacity for heartbeat and cleanup. | `TestCommandSendRechecksBeforeWrite`, `TestCommandSendDeadline`, `TestCommandBacklogYieldsToHeartbeats`, `TestCommandSendPoolHeadroom` |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Select a bounded page and persist one attempt. | `src/server/internal/link/outbox_claim.go`, `State.claimNext`. Patterns S6 and S9. |
| 4a | Cap a response write by its supplied context. | `src/server/internal/transport/deadline.go`, `Send`. Pattern S10. |
| 4b | Connect the persistent claim to the stream writer and its deadline. | `src/server/internal/link/outbox_send.go`, `State.SendNext` and `src/server/internal/link/stream.go`, `Service.serve`. Patterns S9 and S10. |

**Item 3. Claim one command in a bounded turn**

- **Files and names.** Create `src/server/internal/link/outbox_claim.go` and `src/server/internal/link/outbox_claim_test.go`. Declare private `commandCursor` containing the last examined `seq` and seen approval, an earlier retryable scan identifier and the first deferred scan sequence, `claimPageSize = 32`, and `(*State).claimNext(ctx context.Context, machineID, streamID string, cursor *commandCursor) (cmd *linkv1.Command, more bool, err error)`. Add no name to a Part A file. The stream adapter owns a fresh cursor per current stream.
- **Dependencies and size.** Needs items 1 and 2. It can run alongside item 5 and item 6. Work area `backend`, complexity medium, scope `server/backend`. It is one ordered query and attempt update with a commit gate. Statements R3, R4, R6, R7, R10 and C8.
- **Bound the query before filtering.** In a two-second store write, use `lockCommandStream`. Reset the cursor on waiting-to-approved transition. Materialize the first 32 metadata rows using the pending index and `seq > cursor` before testing eligibility. Waiting machines can take only `set_state`. Skip a row already assigned to the current stream and any nonstate action at or beyond expiry plus 23 elapsed hours. Keep the unsigned attempt ceiling. Test the plan on a large analyzed fixture and bound the write even if the index has dead entries.
- **Scan eligibility.** The first pending scan within cutoff reserves the scan slot, including an older-stream claim. Defer distinct scans while this head remains eligible by time, and let restart pass. Point-read the head under the machine lock. On settlement or cutoff rewind to the first deferred sequence and clear the head. Keep the page limit. An inbound result or the next heartbeat revisits the cursor without a timer or feed change. A new stream starts at zero and discovers the head again. UNREVIEWED MECHANISM (logic-r1 02).
- **Advance after commit.** Choose the oldest eligible page row. Read its payload. Bind attempt 1 if unassigned, else increment and bind the new stream. Reset `dispatched` and reconstruct the frame with the stored identifier and expiry. Advance through the selected row, or through the page if no selection. A full page may continue on another turn. Keep the frontier when idle. At the ceiling leave the row pending without wrapping. Rollback or a lost commit reply yields no command and ends the stream. A later stream can observe an attempt gap. Claim adds no feed reference.
- **Tests written first.** `TestClaimRaisesAttemptOncePerStream` observes attempt 1 and the stored expiry on A, no duplicate on A, then the same identifier, action and expiry with attempt 2 on B. Exercise the unsigned ceiling using a seeded row and observe progress of the next command. `TestClaimCommitsBeforeReturn` holds the real commit, sees no returned command or visible changed attempt, and checks the values after release. Exercise forced rollback and lost commit reply separately. A fresh stream reads the durable outcome. `TestClaimOrder` uses reversed creation times, work on the cutoff's two sides, an old `SetState`, and a waiting machine that later approves on the same stream. Check one lowest eligible `seq` per claim, stale-stream refusal, eligibility after approval and no feed writes. `TestClaimOrder` also holds scan A pending, queues scan B and restart C, and sees C bypass B on A’s stream and after reconnect. Finish A and see B advance after its result turn. In a separate case see B advance on the first heartbeat after A’s send cutoff. `TestClaimBoundsIneligiblePrefix` plants several pages of expired, already assigned, or disallowed pending rows before one eligible row. Check the actual page-query shape and measured rows at the bounded page node with the pending index. Assert each empty turn reports continuation and releases the registry lock. A competing heartbeat can commit before the next turn, and the writer eventually reaches the eligible row.
- **Risk and mutations.** Risky because both attempt order and heartbeat latency depend on this item. Filtering before limiting, scanning another page under the same lock or omitting scan deferral or its rewind fails `TestClaimBoundsIneligiblePrefix` or `TestClaimOrder`. Returning early fails `TestClaimCommitsBeforeReturn`. Reusing the first stream's marker or wrapping the attempt fails `TestClaimRaisesAttemptOncePerStream`. All four tests pass in `./src/server/internal/link`.

**Item 4a. Honor an earlier response deadline**

- **Files and names.** Change `src/server/internal/transport/deadline.go` at `Send` and `src/server/internal/transport/link_stream.go` at `linkWriter.FlushError`. Create `src/server/internal/transport/command_deadline_test.go`. Keep the Part A and Part B signatures. Add no Part A declaration. The transport writer records the first actual flush error in a private send-scoped field on `linkWriter`. `Send` reads and clears that field. (threat-r1 02)
- **Dependencies and size.** Needs the built Part A deadline middleware. Can run alongside items 1 through 3 and 5 through 7 as their prerequisites allow. Work area `backend`, complexity medium, scope `server/backend`. This change uses the existing deadline controller, flush and cancellation path. Statements R6, R10, C8 and C9.
- **Behavior.** `Send` sets the earlier of the usual timeout and the caller’s context deadline before calling the response callback. Check the context again at entry and after the callback. When it wraps a link send, retain the first underlying flush error even when the protocol only invokes the errorless flusher. Observe during that invocation and clear the capture on return. A callback, flush or context error prevents dispatch confirmation. Preserve cancellation and deadline cleanup on normal return. The separate Welcome gate and terminal writer remain outside this path. (threat-r1 02)
- **Tests written first.** `TestSendUsesEarlierContextDeadline` checks the normal timeout, a shorter live deadline, cancellation before entry and cancellation during a held write. A success clears its deadline. In `TestLinkSendReportsFlushFailure` use the real wrapped Connect and Google Remote Procedure Call (gRPC) handlers. Accept the buffered frame write and fail the underlying flush. Require the enclosing `Send` to return that error, to report a context expiry during a callback that returns nil, and to reset flush capture for the next write. (threat-r1 02)
- **Risk and mutations.** Risky because a late or failed write can look like dispatch. Ignoring the shorter deadline or running the callback with a canceled context fails `TestSendUsesEarlierContextDeadline`. Suppressing the underlying flush error or the post-callback context check fails `TestLinkSendReportsFlushFailure`. Run both and the transport package. (threat-r1 02)

**Item 4b. Fence one command write**

- **Files and names.** Create `src/server/internal/link/outbox_send.go` and `src/server/internal/link/outbox_send_test.go`. Add private `commandSlots` to Part A’s `Service` in `src/server/internal/link/service.go`. The Part B writer in `src/server/internal/link/stream.go` acquires and releases permits. `SendNext` reads the grant from its private `commandSession`, which also carries the cursor and write context. Implement `(*State).SendNext(ctx context.Context, machineID, streamID string, send func(*linkv1.Command) error) (sent bool, err error)` and private `State.sendClaimed`. The test-only context callback `afterCommandClaim func(context.Context, *linkv1.Command) error` runs after claim commit. Keep both hook signatures. (critic-r1 02, logic-r1 01, threat-r1 03)
- **Dependencies and size.** Needs items 3 and 4a and the merged Part B stream loop. Item 5 can build independently. Item 4b tests can seed a terminal row through a locked fixture until the real sweeper is available. Work area `backend`, complexity medium, scope `server/backend`. The item joins two existing write and transport boundaries with one bounded authorization check. Statements R3, R4, R6, R7, R10, C8 and C9.
- **Shared budget.** Allow three concurrent command permits. The minimum pool has five writer slots, which leaves two outside this path. Acquire without waiting before claim. On saturation register the current handle once and yield without touching the database, advancing the cursor or ending the stream. Release after the write transaction or other claim exit. Grant the oldest live waiter and signal its exact handle through the buffered wake channel. On stream exit remove the waiter or release its unused grant, with the same handoff. No mutex is held during database or transport work. Apply the same budget to the initial and follower-triggered turn. (critic-r1 02, logic-r1 01, threat-r1 03)
- **Writer turns.** `Service.serve` owns the session, cursor and write callback. Keep one command opportunity after each ready input frame and use a buffered wake for continuation or a discarded claim. Check cancellation before the turn and after the claim barrier. Ordinary fake hooks keep the stream context. The production callback calls `transport.Send` with the tighter one. Return sent only after flush and dispatch commit.
- **Send fence.** After claim commit, take the machine lock in a second two-second Store write. Recheck current stream, standing, identifier, attempt, pending outcome and the database cutoff. Skip rows that lose eligibility. Hold the lock through the one callback and flush. Set `dispatched = true` in that transaction and commit before the writer handles another frame. Publish no feed reference. Roll back on callback error and preserve the original error for terminal handling. Any ambiguous flush or marker commit ends the stream. A later stream can retry the same identifier and expiry at the next attempt, including when the client already saved a result. (architect-r1 02, critic-r1 01, logic-r1 03, threat-r1 01, threat-r1 02)
- **Deadline calculation.** For scan and restart sample process monotonic time before the database check. Add the remaining database interval and cap the deadline by the two-second context. Skip and wake when elapsed. State notices use only the two-second cap. The cutoff assumes no forward database clock step during the write. A flushed frame can still be in transit.
- **Tests written first.** In `outbox_send_test.go`, `TestCommandSendRechecksBeforeWrite` pauses after claim commit. A direct matching result before dispatch leaves the row pending. In separate runs let removal, replacement or the cutoff win, then observe zero command frames. A matching result after a cutoff discard still leaves the row pending. In the positive run observe the actual frame, a committed dispatch marker and one final result. Inject a failed flush and a lost marker commit reply separately. Require stream termination, lock release, a pending row and a higher attempt after reconnect. (architect-r1 02, critic-r1 01, logic-r1 03, threat-r1 01, threat-r1 02)
- **Transport and capacity fixtures.** `TestCommandSendDeadline` uses the real link listener with a loopback HTTP/2 peer using the pinned frame library. Advertise zero initial stream receive credit. Grant only the five-byte prefix and the declared payload for each uncompressed Challenge and Welcome frame. Keep connection credit available and send no stream credit after Welcome. Enqueue a valid restart through the real production queue. Observe the fenced callback through the wrapped writer before asserting its blocked flush, the shorter deadline, released machine lock and recoverable attempt. Reject an oversized fake frame as a substitute. `TestCommandSendPoolHeadroom` uses the same peer and an eight-connection database pool. Hold two sends while a healthy machine acknowledges a heartbeat and delivers a command. Then occupy all three permits. Start additional turns and observe no claim for a queued waiter. Require an independent heartbeat and committed disconnect cleanup before release. Cancel one waiting stream, release one held send and observe one wake and command progress for a live waiter, with no busy retries. `TestCommandBacklogYieldsToHeartbeats` preserves the Part B receive barrier, observes a heartbeat between ineligible pages, and requires the continuation wake without new enqueue. (critic-r1 02, critic-r1 03, logic-r1 01, threat-r1 03)
- **Risk and mutations.** Risky because a stalled writer can outlive its attempt. Dropping the second lock, marking dispatch before flush or accepting a result for a skipped claim fails `TestCommandSendRechecksBeforeWrite`. Dropping the shorter context fails `TestCommandSendDeadline`. Removing the cap or wake handoff fails `TestCommandSendPoolHeadroom`. Removing the continuation wake or draining multiple pages per turn fails `TestCommandBacklogYieldsToHeartbeats`. The named tests pass in `./src/server/internal/link`.

**Design artifact.** The [frozen command comments](../../../src/contracts/proto/lgtm/link/v1/commands.proto), Part B's existing `CommandHooks` and [post-build contract check 11](../go-s0-contract/plan.md) define the send and result boundary. Round 4's earlier request to send after 23 hours is superseded by the frozen window and the updated check at 22 hours and 30 minutes.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An old queue hides an eligible command or starves input. | High | Use a bounded seek, resume from the examined position and exercise it with a saturated reader. |
| A claimed attempt starts after finalization, the cutoff or replacement. | High | Recheck under the same lock as the competing writers and bound the actual response write. |
| A blocked send holds a database writer slot. | Medium | Apply a two-second ceiling to the second transaction and verify release after cancellation. |

**Verify.** Run the named PostgreSQL and HTTP/2 tests, including `TestClaimBoundsIneligiblePrefix`, `TestSendUsesEarlierContextDeadline`, `TestCommandSendRechecksBeforeWrite` and `TestCommandBacklogYieldsToHeartbeats`. The combined receipt replay checks use the Sprint 3 client and a controlled loopback proxy. No device is needed for server acceptance.

**Rollout detail.** Verify dispatch on the upgraded fixture before enabling owner routes. The merged suite and combined gates follow in WS-3.

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
- **Rename.** Lock before checking standing. Refuse waiting with `errRenameWaiting`. Use `SetName` and enqueue the updated standing only on change. Keep version 1 fingerprints and the empty `RenameClientResponse` in `WriteOnce`. A saved normalized-name request replays after rename or removal. A different body conflicts.
- **Tests written first.** `TestApproveQueuesStateAtomically` uses the signed-in route and real queue. Approve a padded name. Observe trimmed name, revision 2, matching standing and one machines and one outbox reference. Fail enqueue and the enclosing commit separately. Require the waiting record and no new receipt or feed entries. Drop the success reply, rename, remove and replay the original key. Observe the saved reply without a new notice.
- `TestRenameQueuesStateOnce` checks the changed name and next revision in its single notice. An unchanged name does not increment or enqueue. Replay the saved empty reply after another rename and replay a receipt from Part A without enqueuing. Cover waiting refusal, missing machine and failed queue. Hold approval before commit, start rename and require the next revision only after approval commits. Run `TestApproveClient`, `TestRenameClient`, `TestKeyedCallsReplay` and `TestWatchClientsStreamPresence` as regressions.
- **Risk and mutations.** Risky because approval controls which machines can act and the reply is replayed. Moving either enqueue after commit or building the approval reply outside `WriteOnce` fails `TestApproveQueuesStateAtomically`. Skipping the change flag, the row lock or the keyed callback for rename fails `TestRenameQueuesStateOnce`. The named tests pass in `./src/server/internal/machines`.

**Item 7. Scan and restart calls**

- **Files and names.** Change `src/server/internal/machines/records.go` to move the two existing stub methods. Create `src/server/internal/machines/commands.go` and `src/server/internal/machines/commands_test.go`. Keep the `ScanClientHarnesses` and `ReconnectClient` method signatures and the refusal sentences. Declare private `Service.queueConnectedAction` in the new file. Remove the old `refuseOfflineCall` after moving its refusal behavior. Add no declaration to Part A's records file. Move any exclusively used refusal errors to the new file with their names intact.
- **Dependencies and size.** Needs item 6 for the queue field and its prior edit to `records.go`, and Part B presence. It can run alongside items 4b and 5. Work area `backend`, complexity medium, scope `server/backend`. Two routes share one eligibility and retry path. Statements D16, R3, R4, R6, R8 and R10.
- **Calls.** Use `recordRequest` and `writeRecordChange` with the existing version 1 identifier fingerprint. Read `Link.Presence` under the machine lock in the same transaction. Both calls require approval and an exact live `Held` connection. Scan also requires both report flags. Absent machine maps to `not_found`. Ineligibility or missing queue or link maps to `failed_precondition` with the original scan and restart sentences. Declare the refusal errors with `store.NewDomainError` and map them in `recordError`. Keep key conflicts as `already_exists` and retryable lock errors as `aborted`. Enqueue for two minutes and return the frozen empty response on acceptance. The watch carries reported inventory. (architect-r1 05)
- **Tests written first.** `TestScanAndReconnectQueueCommands` covers missing, waiting, offline, joining and online, with both report flags separately. Cancel a handle and fail its row deletion. Refuse stale presence and an old hold after replacement. Refusals save neither command nor receipt. Joining accepts only restart and online accepts both with two-minute expiry.
- `TestActionRequestKeyReplays` races identical keyed requests for each route and observes one command and receipt. Replay after disconnection, conflict on a different body, then force a retryable lock error and succeed with the same key. Keep `TestScanAndReconnectRefused`.
- **Risk and mutations.** Risky because actions must use current liveness and one command per keyed request. Removing one report flag, trusting a stale row or allowing a waiting machine to restart fails `TestScanAndReconnectQueueCommands`. Checking presence before keyed replay or using an unkeyed write fails `TestActionRequestKeyReplays`. Both tests and the frozen refusal test pass in `./src/server/internal/machines`.

**Item 8. Server composition and production-route checks**

- **Files and names.** Change `src/server/internal/app/start.go` and `src/server/internal/link/run.go`. Create `src/server/internal/app/commands_test.go` in `app_test` and `src/server/internal/link/outbox_integration_test.go` in `link`. Start with `Options.Commands: state`, `WithCommands(state)` and `WithLink(state)`. In `Service.Run` use a private assertion to `*State` to join its sweep with the feed and cleanup workers. Preserve Part A declarations, public fixtures and Part B’s two-method fake hooks.
- **Dependencies and size.** Needs items 2, 4b, 5 and 7 and Part B item 10. Server tests use only server-owned code. The combined-suite writer can prepare the requested cases while these items build. Work area `backend`, complexity medium, scope `server/backend`. This composes existing components and verifies the route and lifecycle they expose. Statements D1, D3, D5, D7, D16, R1, R4, R6, R7, R8, R10 and C7 through C12.
- **Composition.** Apply migration 2 under the instance lock before listening. Preserve the primary shutdown cause, cancel streams and join workers. Report sweep errors with fixed diagnostics. Start the same routes in `servertest.Start` and `Server.Restart`. Keep owner authorization on every command route.
- **Tests written first.** `TestServerRoutesStoredCommands` uses the app listener, a waiting stream and a signed-in watch. Approve on that stream. Observe the notice, joining, fresh reports and online. Send a signed-in scan and observe the reported inventory on the watch. Leave a restart pending through server shutdown. Reconnect and capture the same identifier, unchanged expiry, the next attempt when previously claimed and the matching durable result. `TestServerCommandSweepLifecycle` applies migration 2, stops the server and seeds due rows before restart. Observe the initial pass time out an action and keep a pending `SetState`. In a separate run hold the sweep’s due-row query with a conflicting table lock in an independent PostgreSQL session. Observe its lock wait, then stop and restart the application. Require cancellation, released instance lock and surviving queued work.
- **Ordering fixtures.** In `outbox_integration_test.go`, `TestCommandSendAndSweep` holds the writer after observing a durable claim. Age that test row under the machine lock past the grace boundary and observe the real sweep select and finish it. Release the writer and require no command frame on the live stream. In the inverse case, hold a fenced send and seed a separate already-due command for that machine and one for a control machine before the hold. Trace selection and PostgreSQL’s nonwaiting lock refusal. Require the control row to finish, the first machine’s due row to stay pending and a later pass to finish it. Keep send expiry immutability in a separate unaged replay. UNREVIEWED MECHANISM (logic-r1 04). (architect-r1 03, critic-r1 04).
- **Terminal states.** `TestCommandResultAndRemoval` runs the real result and removal writers in both orders after a successful dispatch. Inspect the first outcome, finish time, cleared payload and removed watch record. Admit no later command or result after removal. In `TestCommandSendAndSweep` also use two separately aged fixtures after a real flush. Commit the current machine’s expiry report before the sweep in one case and run the sweep first in the other. Require `expired` in the first case and `timed_out` in the second, with null payloads and stable finish times. (logic-r1 06, threat-r1 04)
- **Risk and mutations.** Risky because this enables a remote owner action over a machine stream. Omitting the queue or command hook fails `TestServerRoutesStoredCommands`. Dropping the worker or ignoring cancellation fails `TestServerCommandSweepLifecycle`. Releasing the send lock before the flush, omitting the maintenance lock or conflating a timeout with a matching expiry report fails `TestCommandSendAndSweep`. Letting removal overwrite a committed result or accepting a result after removal fails `TestCommandResultAndRemoval`. Run the named app and link tests and keep the inherited route tests. In the app test file, add `TestServerCommandOwnerBoundary` for P5. Bypassing a route’s session guard fails that test.

**Design artifact.** The [Machines contract](../../../src/contracts/proto/lgtm/v1/clients.proto) and [Part C handoff](../go-s3-server-machines/handoff-stream-and-commands.md) specify the routes. Website owns acknowledging a queued scan and using `WatchClients` for inventory. Test accepted scans with changed inventory and with connection loss. Choice 6 governs the connected restart menu and website revision. (architect-r1 05)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An offline or waiting machine takes work. | High | Authorize through the exact held current connection and test the refusal cases on the route. |
| Approval on an existing connection leaves the machine waiting. | High | Check the notice and both approved reports on the same stream through the generated clients. |
| A lost reply duplicates a command. | High | Observe the first stored reply and exact command count after a concurrent retry and replay. |

**Verify.** Run each item against PostgreSQL, including `TestApproveQueuesStateAtomically`, `TestActionRequestKeyReplays` and `TestServerRoutesStoredCommands`. Then run `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...`. The Sprint 3 gate uses the loopback stack and a disposable installation of the real client. The Linux service check belongs to Stack. No device is needed for the server-only tests.

**Rollout detail.** Record the Part A and Part B merge and its passing server suite from the Verify command. Start the assembled stack with `deploy/up.sh` and check migration 2. Run `deploy/test/checks.sh --only C5,C6,C7,C8,C9,C10,C11` and `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12`. The wrapper supplies `LGTM_TEST_POSTGRES`. (critic-r1 06) Run the combined cases after both ends land through `servertest.Start` and `clienttest.Join` and `Run`.

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
| `TestLinkSameStreamApproval` | Start a real waiting client, assert a waiting Welcome and no scan or restart frame, and use the owner calls to verify waiting refusals. Approve while it is connected. Observe a `SetState` and a result on that stream. Hold the approved report frames in the proxy until the watch shows joining. Release them and observe fresh approved facts and harnesses, then online within 30 seconds of approval at the same generation. For contract check 19, start a fresh waiting fixture for each age. Use a disposable insert trigger restricted to that notice to place its first delivery a few seconds and more than 24 hours past expiry, respectively. Verify the expiry in the received frame. Require the first delivery, `done`, and both reports on the original stream in both cases. Script a repeated revision and require no additional report from reapplying it. (critic-r1 05) |
| `TestLinkCommandAttemptsAcrossReconnect` | Hold a requested scan while cutting the stream, and reconnect on the same client state. Capture attempt and stream identity on both deliveries. Allow the newer connection to report inventory before releasing the scan. Observe the client plan's failed result on the latest attempt in that fixture, one stored final result, and success for a later scan. Run the completed-scan variant by dropping the first result after its receipt is durable, restarting the client and capturing the replayed result. Count requested scan effects with a fixture version probe distinct from the initial scan. |
| `TestLinkScanDoesNotDelayRestart` | Let the initial inventory report finish. Gate only the first requested scan’s disposable executable probe within its normal time budget. Enqueue a second scan and then a restart. Capture the restart on the current stream while the second scan is still unclaimed. Observe cancellation of the held scan and one service exit. In a separate run let the first scan finish and observe delivery of the second on the same stream. The orchestrator owns the combined writer and observes the frames. (logic-r1 02) |
| `TestLinkRestartReplaysAfterDroppedResult` | Drop the first result after the restart receipt commits. The launched client returns 75 exactly once. Relaunch it with the same directory, then observe the retry's unchanged identifier and expiry, incremented attempt and replayed `done`. Require no second exit 75. The Linux installer gate `live_reconnect_restarts_service` observes the new process and one service restart. |
| Contract checks 11 and 26 and the inherited P5 | The orchestrator covers real transport and server selection. With both writers stopped, age a completed receipt and its pending row together for replay at 22 hours and 30 minutes. Delay Welcome eight seconds and verify the same identifier, the fixture expiry and the saved result. Use a separate unaged retry for expiry immutability. In another fixture require no send at and after 23 hours. The Client team owns `TestCommandReceiptSkewAtRetryCutoff` in `src/client/internal/link/commands_test.go`. Use the real store, scripted retry delivery, the private wall clock and a barrier at the intervening pruning commit. Issue at expiry plus 22 hours, 59 minutes and 59.5 seconds. Advance delivery two seconds. With a clock 59 minutes and 59 seconds fast, commit another receipt 1.75 seconds after issue and then observe a pruned receipt return `expired` with no new effect. An on-time clock returns `done`. The two-hour-fast inherited case also compares lookup before and after a pruning commit. Gate 26 requires both reports. (architect-r1 04, critic-r1 05) |
| Contract checks 6 and 20 | Keep the client's existing start-time and interruption suite. Verify its latest-attempt closure against the real server. Use the client's clock fixtures for delayed Welcome, expiry with a five-minute-slow machine and recovery after a start without a final receipt. |
| Contract checks 9 and 15 | Reuse the existing token and session-boundary fixture. Use a scripted machine for reserved-field and invalid result messages the real client cannot produce. Scan the test database, captured logs and returned watch frames for the marked reserved field while observing the valid result commit. |

**Post-build checks**

| Check and owner | Scenario and expected observation |
|---|---|
| P1. Server transport and link | Run the held-write, flush-error and pool-headroom gates in item 4b. After a successful short-deadline command, let its old deadline pass and receive a heartbeat acknowledgement on the same stream. |
| P2. Server link | Capture pending and due index plans with actual rows at scale. Enqueue eligible work after a short ineligible page idles and observe the wake. Use fixed decision times to prove no send at 23 hours, no sweep at 24 hours and no prune at seven days. Advance past the maintenance boundaries to observe the changes. A dispatched matching result remains eligible until finalization. |
| P3. Server link and orchestrator | Lose the reply to each committed write separately. A lost claim reply emits no command. The independent row and new stream show the same identifier and expiry and a higher attempt. Replay an earlier saved client result without a second effect. |
| P4. Server app and orchestrator | Stop with a held flush and sweep query. Require the original cause, freed database resources and the next durable attempt after restart. Separately interrupt after migration 2 commits but before listen. Restart with one instance lock, one sweep worker and the prior machine records and receipts. |
| P5. Server app | Use otherwise eligible machines for all four owner routes. Try anonymous calls, an approved machine token alone, a valid owner cookie from a foreign origin and the same cookie after sign-out. Include a previously valid request key. Require `permission_denied` for the foreign origin and `unauthenticated` for the other denied cases. Observe no replay, command, receipt, registry write or wake. A valid signed-in owner still queues exactly one command. |
| P6. Orchestrator and Stack | Run the combined approval and dropped-result cases. Require restart eligibility at approved live presence, scan after both report flags, a replayed result and one installed service restart. |
| P7. Server and Stack | Run the reserved-field fixture through the private proxy hop. Require a committed result and no marker, credential or request key in storage, logs or public replies. Check cleared terminal payloads and the owner’s at-rest policy after approval. |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is the foundation design, build loop, [Part C handoff](../go-s3-server-machines/handoff-stream-and-commands.md), [reviewed draft](../go-s3-server-machines/attack/plan-v4.md) and its four registers, [stream result](../../../.foundry/go-rewrite/results/plan-go-s3-server-stream.md), frozen link and contract checks, client plan, owner choices, rewrite issues, deployed checks and the four Part C review reports.

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
| Deployed C5 through C12 | WS-1, WS-2, WS-3 | Ownership and both rollout commands appear in WS-3. |
| Part C review round 1 | WS-1, WS-2, WS-3 | Folded findings are marked at the changed items. The register records the remaining owner choice and the checks. |

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
| 1 | migrations | server/backend | medium | One additive migration and one atomic enqueue. | Yes | Part B migration 1 and registry |
| 2 | backend | server/backend | medium | One guarded result update and one stream check. | Yes | 1 |
| 3 | backend | server/backend | medium | One bounded seek and a committed attempt. | Yes | 1, 2 |
| 4a | backend | server/backend | medium | The response controller honors the earlier deadline. | Yes | Part A deadline middleware |
| 4b | backend | server/backend | medium | One fenced send holds a shared permit and commits dispatch. | Yes | 3, 4a and Part B writer |
| 5 | backend | server/backend | medium | One end-of-life path for due and removed commands. | Yes | 1 and Part B removal |
| 6 | backend | server/backend | medium | Two existing keyed handlers add one transactional notice each. | Yes | 1 and Part B owner projection |
| 7 | backend | server/backend | medium | Two existing routes share one current-stream check. | Yes | 6 |
| 8 | backend | server/backend | medium | Existing app and worker entry points compose the completed units. | Yes | 2, 4b, 5, 7 and Part B application |

Load. One vendor fills separate reviewer, test writer and builder seats. There are four plan reviews and three launches per unit, or 31 first-pass launches. Before dispatch record the measured allowance, complete units and review seats that fit. Reserve each unit’s test writer, builder and code reviewer together. Put the first deferred seat or unit at the head of the next window. Rework and combined tests have separate allocations. (critic-r1 08)

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings (blocker/major/minor) | Outcome |
|---|---|---|
| Inherited review of Part C | See the four source registers. | Decisions, dates and carried findings are linked in Traceability. The bounded prefix, held send and old check 11 timing have explicit dispositions and acceptance tests. |
| This plan's review, round 1 | One blocker, eight majors and seven minors after grouping. | The [round 1 register](attack/round-1-findings.md) records R10 as stuck and the patches for review. |

</details>
