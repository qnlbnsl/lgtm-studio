# REVISE

An older scan at the attempt ceiling can hold the scan slot after reconnect, leaving a later scan unassigned until the older scan's retry window ends.

### post-b 1 — MAJOR — An exhausted scan can block the next scan after reconnect

**Statement.** Item 3 requires the oldest eligible command to advance. The plan's attempt-ceiling decision keeps the exhausted row pending and allows subsequent commands to progress. [Item 3](../plan.md#L152) · [Attempt ceiling](../decisions.md#L13)

**Evidence and failure scenario.** By reading, the claim reserves the scan slot before it checks the attempt ceiling. The head lookup then checks only whether that row is still pending and within its time window. On a replacement stream, the old scan cannot receive a new attempt, and its result cannot match the current stream. Confirmed live, a PostgreSQL probe seeded a dispatched scan at attempt 4,294,967,295 on the old stream. The replacement stream took the later restart, then returned no command or continuation. The second scan remained unassigned while the cursor remembered the exhausted scan. The frozen ceiling fixture uses two restarts, and the scan deferral fixture uses attempts 1 and 2. [Claim order](../../../../src/server/internal/link/outbox_claim.go#L65) · [Head lookup](../../../../src/server/internal/link/outbox_claim.go#L105) · [Result fence](../../../../src/server/internal/link/outbox_result.go#L30) · [Ceiling fixture](../../../../src/server/internal/link/outbox_claim_test.go#L105) · [Deferral fixture](../../../../src/server/internal/link/outbox_claim_test.go#L267)

**Fix.** Reserve the slot only for a scan that can still be claimed or can finish on the current stream. Preserve the reservation for an exhausted attempt already dispatched on the current stream. Seed an exhausted attempt on a displaced stream and require the next scan to advance with the old row still pending.

### post-b 2 — MINOR — The sweep tests admit a shorter timeout window

**Statement.** Item 5 uses a strict 24-hour grace after expiry. Post-build check P2 requires testing equality and advancing past it. Foundation rule 10 calls for real PostgreSQL evidence. [Sweep rule](../plan.md#L113) · [Post-build check P2](../plan.md#L285) · [Foundation rule 10](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L371)

**Evidence and failure scenario.** By reading, the frozen expiry fixture keeps a 23-hour-old row and finalizes 25-hour-old rows. Confirmed live, computing the sweep cutoff with a grace of 23 hours and 30 minutes passed all 11 tests in the assigned Link acceptance and regression runs. The fixed-clock probe rejected the change because the 24-hour row was already terminal during the pass at equality. The shorter grace can close a pending command before the plan's timeout boundary and discard a matching result arriving during that half hour. The pinned implementation passed the fixed-clock probe. [Frozen fixture](../../../../src/server/internal/link/outbox_sweep_test.go#L63) · [Cutoff computation](../../../../src/server/internal/link/outbox_sweep.go#L45) · [Pending result guard](../../../../src/server/internal/link/outbox_result.go#L31)

**Fix.** Keep a fixed database decision time in the acceptance fixture. Assert the row is pending at exactly 24 hours and becomes `timed_out` one microsecond later. Keep the corresponding equality assertions for claim and retention.

### post-b 3 — MINOR — The removal tests can miss a pending restart

**Statement.** Item 5 ends every pending action in the machine removal write. Foundation rule 10 calls for real PostgreSQL evidence. [Removal rule](../plan.md#L114) · [Foundation rule 10](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L371)

**Evidence and failure scenario.** By reading, the removal fixture has a pending scan, a pending standing notice and a completed restart. A filter that excludes pending restarts can therefore leave a restart and its payload after the registry row is deleted. The sweep skips pending rows for missing machines, so it cannot repair the orphan. Confirmed live, the pinned owner removal route finalized a pending restart and cleared its payload in a separate PostgreSQL probe. The mutation that added `action <> 'restart'` to `Forget` left that restart pending with its payload and without a finish time. All 11 tests in the assigned Link acceptance and regression runs accepted the mutation. [Removal fixture](../../../../src/server/internal/link/outbox_sweep_test.go#L380) · [Removal update](../../../../src/server/internal/link/presence.go#L244) · [Missing-machine handling](../../../../src/server/internal/link/outbox_sweep.go#L112)

**Fix.** Seed a pending restart in the approved removal case. Require it to survive a rejected commit and finish as `removed` with a cleared payload on retry. Keep the completed restart to check preservation of its outcome.

**Grade.** 8 out of 10.

## Post-build checks on the pinned code

The review covered items 3, 5 and 6 in `584d7b80..84a1eff7` on `sprint-0`. The round register and the plan identify the unreviewed dispatch marker, scan reservation and rewind, and distinction between machine expiry and server timeout. The code references below use the pinned revision's line numbers. [Round register](round-1-findings.md#L9) · [Plan](../plan.md#L92) · [Builder result](../../../../.foundry/go-rewrite/results/s3-commands-build-c.md#L10)

| Check | Observation |
| --- | --- |
| Dispatch marker and lost claim reply, P3. | Confirmed live, the frozen commit test held a real commit, forced a rollback, and dropped the reply after a separate successful database commit. Rollback and the lost reply returned neither work nor continuation. The observing connection saw the durable attempt after reply loss. The replacement stream claimed the same identifier and expiry at attempt 2. The separate attempt test proved that the replacement claim reset an existing dispatch marker. Client receipt replay belongs to the assembled stream and client gate. [Commit cases](../../../../src/server/internal/link/outbox_claim_test.go#L124) · [Dispatch reset](../../../../src/server/internal/link/outbox_claim_test.go#L99) |
| Scan reservation, rewind and enqueue wake, P2. | Confirmed live, the frozen order test let restart pass the deferred scan on the original and replacement streams. A result and a heartbeat after crossing cutoff each let the deferred scan advance. A separate short-page probe let an idle cursor pass an expired restart. A real committed enqueue reached the feed follower, woke the exact idle handle, and the same cursor claimed the new restart. The attempt-ceiling case failed as described in post-b 1. The full wire delivery scenario belongs to the assembled writer. [Order cases](../../../../src/server/internal/link/outbox_claim_test.go#L267) · [Wake forwarding](../../../../src/server/internal/link/run.go#L239) · [Combined writer scenario](../plan.md#L274) |
| Pending and due indexes, P2. | Confirmed live, the frozen pending test used 10,000 background rows. It measured 32 rows from `link_commands_pending` beneath one limit per turn and committed a heartbeat between turns. With 120,000 analyzed rows, both the initial and later maintenance pages used index-only scans on `link_commands_expirable` and `link_commands_finished`. Each scan and its limit returned 64 rows. [Pending observation](../../../../src/server/internal/link/outbox_claim_test.go#L318) · [Maintenance queries](../../../../src/server/internal/link/outbox_sweep.go#L30) |
| Retry, timeout and retention boundaries, P2. | Confirmed live, the probe supplied a fixed PostgreSQL clock through a fixture-local function on the application connection's search path. The claim skipped the row at exactly 23 hours past expiry and took one that was one microsecond inside the window. The sweep kept the pending row at exactly 24 hours and the terminal row at exactly seven days. A one-microsecond advance timed out the first and pruned the second. A matching dispatched result 26 hours after expiry committed as `expired` with a cleared payload while its row was still available to accept a result. The competing-result fixture also preserved a result committed after candidate lookup and before maintenance acquired the machine lock. [Claim cutoff](../../../../src/server/internal/link/outbox_claim.go#L53) · [Sweep cutoff](../../../../src/server/internal/link/outbox_sweep.go#L45) · [Competing result](../../../../src/server/internal/link/outbox_sweep_test.go#L93) |
| Held maintenance query, P4. | Confirmed live, a disposable PostgreSQL transaction held an exclusive lock on the command table. The startup sweep was visible waiting on its due-row query. Canceling the worker cleared the wait before the fixture released its lock. The row was still pending after cancellation and a fresh pass timed it out. The combined application shutdown holds both the sweep and a command flush and belongs to items 4b and 8. [Worker](../../../../src/server/internal/link/outbox_sweep.go#L142) · [Application check](../plan.md#L287) |
| Owner notice persistence and terminal payload clearing, P5 through P7. | Confirmed live, the assigned signed-in route fixtures exercised approval and rename with the real queue. Queue, projection and approval commit failures rolled back as applicable. The first keyed reply replayed, unchanged rename added no notice, and removal preserved the earlier result while clearing the newly terminal payloads. The pinned removal route also cleared a pending restart in the separate probe. The assembled authorization matrix, on-wire approval, proxy and deployed storage checks belong to the app and stack gates. [Owner notices](../../../../src/server/internal/machines/state_commands_test.go#L232) · [Removal cases](../../../../src/server/internal/link/outbox_sweep_test.go#L346) · [App and stack checks](../plan.md#L288) |

By reading, the owner handlers queue standing and save receipts within the existing Store write. The maintenance code uses bounded keyset pages and rechecks each batch under the registry lock. The sweep worker reports its fixed diagnostic, and owner handlers map database failures to public error messages. [Keyed approval](../../../../src/server/internal/machines/records.go#L83) · [Keyed rename](../../../../src/server/internal/machines/records.go#L118) · [Batch recheck](../../../../src/server/internal/link/outbox_sweep.go#L112) · [Worker diagnostic](../../../../src/server/internal/link/outbox_sweep.go#L142) · [Public errors](../../../../src/server/internal/machines/records.go#L207)

## Required test and mutation output

All three commands ran through the real PostgreSQL wrapper with the Go race detector. The JavaScript Object Notation (JSON) events showed a pass for every listed top-level test. Each command exited zero with no test-level failure or skip.

```text
$ deploy/test/with-postgres.sh -- go test -json -race -count=1 -run '^(TestClaimCommitsBeforeReturn|TestClaimRaisesAttemptOncePerStream|TestClaimOrder|TestClaimBoundsIneligiblePrefix)$' ./src/server/internal/link
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/link  11.182s

$ deploy/test/with-postgres.sh -- go test -json -race -count=1 -run '^(TestSweepCommands|TestSweepReleasesMachineBetweenBatches|TestCommandRemovalIsAtomic|TestCommandSweepStops|TestForget|TestCommandMigrationUpgradesPartB|TestAcceptResultRules)$' ./src/server/internal/link
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/link  22.086s

$ deploy/test/with-postgres.sh -- go test -json -race -count=1 -run '^(TestApproveQueuesStateAtomically|TestRenameQueuesStateOnce|TestApproveClient|TestRenameClient|TestKeyedCallsReplay|TestWatchClientsStreamPresence)$' ./src/server/internal/machines
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/machines  50.260s
```

Each planned mutation below ran alone under the same wrapper with the race detector and returned exit 1 on the named assertion. The scratch probe failures for the scan ceiling and the two coverage gaps are recorded with the findings above.

| Item and mutation | Failing assertion |
| --- | --- |
| Item 3. Omit the deferred scan rewind. | `TestClaimOrder` failed after result settlement and after the cutoff heartbeat. Both cases failed to return the deferred command's identifier, expiry and attempt. [Claim assertion](../../../../src/server/internal/link/outbox_claim_test.go#L303) |
| Item 5. Omit the maintenance machine lock. | `TestSweepReleasesMachineBetweenBatches` missed the expected PostgreSQL nonwaiting lock refusal and changed the row belonging to the locked machine. [Lock and pending assertions](../../../../src/server/internal/link/outbox_sweep_test.go#L212) |
| Item 6. Ignore the changed flag on rename. | `TestRenameQueuesStateOnce` found an extra notice or feed effect for the unchanged name. [Unchanged-name assertion](../../../../src/server/internal/machines/state_commands_test.go#L346) |
