REVISE

# Client-link round 2, architect lens

Task identifier: review-go-s3-client-link-r2-architect. Nonce: 0440c0. Finding prefix: `architect-r2`.

Top finding: the new `Store.Decline` that item 6 depends on has no owner, because Sprint 1 is already built without it and item 6 lists no edit to `src/client/internal/link/receipts.go`.

Scope: the patches between commits 6d416f17 and 80e5857a of `plan.md` and `decisions.md`, and what follows from them. I checked claims against `src/client/internal/link/receipts.go`, `outbox.go` and `store.go`, `go.mod`, `src/contracts/proto/lgtm/link/v1/`, `deploy/test/checks.sh`, the Sprint 1 decision log and the neighbouring Stack Client plan.

## Findings

### architect-r2 1 — MAJOR — R6 — `Store.Decline` has no owner and no specification

- **Statement.** R6, item 6, rollout step 1.
- **Evidence.**
  - `.plans/plans/go-s3-client-link/plan.md:248` calls "proposed `Store.Decline(c Command, result []byte) (Decision, []byte, error)` (writer: Client Local. Reader: item 6)".
  - `plan.md:242` lists only `commands.go` and `commands_test.go` as item 6 files.
  - `plan.md:243` says the tests freeze "after Client Local agrees to `Decline`".
  - `src/client/internal/link/receipts.go:65` and `:116` have `Begin` and `Finish` and no `Decline`. `grep -rn Decline src .plans/plans/go-s1-client-local .plans/epics` finds nothing outside this plan.
  - `decisions.md` row 43 is "unanswered".
  - The built Sprint 1 store is committed in 3951598b, and the working tree shows its tests being edited now, with no `Decline` in the edits.
- **Failure scenario.** The Client Local plan is finished and built. No team is assigned to the agreement the plan waits for. Item 6 cannot freeze its tests, items 7b, 8 and 9 depend on item 6, and the whole link build stops at the freeze gate. If a builder bypasses the wait and writes a decline by hand with `Begin` and `Finish`, the false-start defect of architect-r1 01 returns. A crash after the `started` commit and before the `Finish` commit then makes a command that was only declined replay as interrupted.
- **Prescribed fix.** Item 6 owns the change, as the rule "edit, do not work around" requires. Both sides are the client-backend scope.
  - Add `src/client/internal/link/receipts.go` and `receipts_test.go` to item 6's Files.
  - Remove the words "writer: Client Local" and the wait on an agreement, and rewrite the freeze sentence at line 243.
  - Write the semantics of `Decline` into item 6 as a truth table over the stored receipt:

    | Stored receipt for the id | Result of `Decline` |
    |---|---|
    | absent | one transaction writes a finished receipt with the given result and the command's expiry, and returns `DecisionExpired` |
    | finished | `DecisionReplay` with the stored bytes, nothing written |
    | started and running in this process | `DecisionRunning` |
    | started and not running | `DecisionInterrupted` |
    | invalid | `DecisionInterrupted`, as `Begin` does |

  - Add a test that kills before and after the commit, next to `TestDeclineSurvivesKillBeforeFinish`.
  - Correct the stale row 69 of `.plans/plans/go-s1-client-local/decisions.md`. It says the clock arrives as `StoreOptions.Now`, but the built code takes `now` as an argument of `Begin` (`receipts.go:65`) and prunes with the package variable `wallClock` (`receipts.go:137`).
  - Change "Sprint 1 receipt methods" in the sizing table to "Sprint 1 receipt methods and the item 6 `Decline`".

### architect-r2 2 — MAJOR — R6 — The order of `Begin` and the scan slot is unspecified, and the natural order strands a command

- **Statement.** R6 and decision 22.
- **Evidence.** `plan.md:248` says `Begin` returns `DecisionRun`, then ends with "Close the stream if a distinct scan cannot enter the slot". Nothing says the slot is taken before `Begin`. `receipts.go:99-111` shows that `Begin` writes the `started` receipt and sets `s.running[c.ID]` (line 111) before it returns `DecisionRun`. Decision 22 allows one active scan per process, and a newer attempt joins only the running command with the same id.
- **Failure scenario.** Scan A runs. The server sends the stored pending scan B on the same stream. The handler calls `Begin(B)`, which commits `started` and returns `DecisionRun`. The slot is busy, so the handler closes the stream. B never ran, but its receipt is `started` and `s.running[B]` is set. On the next stream, `Begin(B)` returns `DecisionRunning`, so the handler waits for a completion nobody will produce, and the server keeps B pending until it records EXPIRED itself 24 hours after expiry. After a process restart, `Begin(B)` returns `DecisionInterrupted`, and the server stores a final INTERRUPTED outcome for a command that never started. That contradicts `commands.proto`, where INTERRUPTED means the machine started the command.
- **Prescribed fix.** In item 6, say that the handler takes the scan slot before it calls `Begin`. On a busy slot for a distinct id it writes no receipt and either queues the command in memory behind A or closes the stream with no receipt. Add a test, `TestCommandBusySlotWritesNoReceipt`, that sends B during A, closes the stream, reconnects after A finishes, and expects B to run once and to finish `done`.

### architect-r2 3 — MAJOR — D16 — Exit 0 now has two meanings and the Stack Client plan knows only one

- **Statement.** D16, item 7b, item 8, and the neighbouring Stack Client plan.
- **Evidence.**
  - `plan.md:203` makes `ErrStoreMissing`, `ErrStoreUnsafe`, `ErrStoreDamaged`, `ErrSequenceExhausted` and impossible acknowledgements exit 0 "to stop the service loop until repair". Decision 46 repeats it.
  - `.plans/plans/go-s3-stack-client/plan.md:30` says "run exits 0 for a stop for good". Line 115 says a removal ends `run` with status 0 and that "a run of the installer after that reports the stopped client and names `--rejoin`". Line 246 prints "run `deploy/install-client.sh --rejoin` if the machine was removed". The tests `live_removed_machine_stays_stopped` and `rerun_reports_stopped_client` follow from those rows.
  - Neither the Stack Client plan nor its README row names a store fault.
- **Failure scenario.** A bit flip damages `client.db`. The client logs one error line to the journal, exits 0, and the unit stays inactive. The Machines page shows the machine offline, with no reason. The owner reruns the installer, which says the client stopped and names `--rejoin`. `--rejoin` makes a fresh identity and leaves the old machine row, when restoring or removing `client.db` was the repair. The plan's "until repair" names no repair step and no reader of the new meaning.
- **Prescribed fix.** Pick one and write it into both plans.
  - Keep exit 0 and add the new meaning to the Stack Client rows at lines 30, 115 and 246, the guide, and the test `rerun_reports_stopped_client`. The installer message then says "removed, or the client logged a local store fault; read the journal first, and run `--rejoin` only if the machine was removed". Add one sentence to item 8 that names the repair for each listed error.
  - Or, if the Stack Client owner prefers not to change, make the faults exit 1, let the 5-second service loop log the error line each time, and delete the exit-0 list from item 7b. Without this, the plan relies on a policy the neighbour never agreed to.

### architect-r2 4 — MINOR — D16 — The path from a permanent local fault to status 0 contradicts "a non-nil error takes precedence"

- **Statement.** D16, items 7b and 8.
- **Evidence.** `plan.md:200` says `Run` returns `(RunOutcome, error)` and "A nonnil error takes precedence". `plan.md:203` and `plan.md:263` need status 0 for a store fault but status 1 for `ErrStoreFailed`. `RunStopped=0` and `RunRestart=1` are the only outcome values. Each of the store faults is also an error value by its name.
- **Failure scenario.** A builder of 7b returns the sentinel error after logging, as the sentinel names suggest. The app reads "non-nil error takes precedence" and exits 1. `TestRetryFailureExitsAndKeepsStore` in the `link` package cannot see a process status and passes on either path. The mismatch appears only in `TestRunCommandExitStatus` or in the installer test. That is the last item to build.
- **Prescribed fix.** In item 7b, say that `link.Run` logs the fault and returns `(RunStopped, nil)` for the listed permanent local faults and a non-nil error only for the exit-1 class. Make `TestRetryFailureExitsAndKeepsStore` assert the returned pair.

### architect-r2 5 — MINOR — R6 — The accepted retention limit is described wider than it is, and has no test at its boundary

- **Statement.** R6, `UNREVIEWED MECHANISM` at `plan.md:26`, `plan.md:250`, decisions rows 26 and 49.
- **Evidence.** `receipts.go:137` prunes finished receipts whose stored expiry is more than 24 hours before `wallClock()`. `receipts.go:99` shows `Begin` returns `DecisionExpired` for an absent receipt when `now` is not before the expiry. `plan.md:26` says "a retry can lose its outcome or repeat work".
- **Analysis.** A machine clock ahead of the server by D prunes a receipt D early. Repeated work needs D greater than 24 hours plus the time left before expiry, because only then can an absent receipt meet a bound before expiry. For ordinary skew, the only effect is that a retry in the last D of the retention window gets EXPIRED instead of the stored DONE. The server then stores EXPIRED for a command that ran. That is a wrong result, but not a second run. The plan states the limit too widely, and no test marks where it changes.
- **Failure scenario.** A builder tests only "a fast clock loses the receipt" and does not separate the cases. A later reviewer sees the repeated-work wording and reopens the owner's ruling.
- **Prescribed fix.** In item 6 and decision 49, replace "or repeat work" with the two cases above. Add to `TestFinishPrunesByAge` two rows: a clock 2 hours ahead gives EXPIRED and no run after server-time expiry, and a clock 30 hours ahead on a command with 1 hour left gives a repeat. The second row documents the accepted limit and does not claim to fix it.

### architect-r2 6 — MINOR — R9 — The 64 KiB read cap is justified by a server cap that no plan states

- **Statement.** R9, item 2, decision 50.
- **Evidence.** `plan.md:103` says `connect.WithReadMaxBytes(64 << 10)` is "matching the server cap". A search for read limits in `.plans/plans/go-s3-server-machines/plan.md`, `.plans/plans/go-s2-server-signin/plan.md`, `link.proto` and the foundation design finds none for the link. The Sign-in plan uses `WithReadMaxBytes(32<<10)`. A server cap would limit what the server reads from the client, and the client's cap limits what the client reads from the server, so they are different directions. Decision 50 is "unanswered".
- **Failure scenario.** The server later sends a legitimate message above 64 KiB, such as a `Welcome` that carries a larger state. The client closes every stream with a read error and retries forever, and no document says whose limit was wrong.
- **Prescribed fix.** Remove "matching the server cap". State the largest legal server-to-client message from `link.proto` and `commands.proto`, and set the client cap with a factor from that value. Record the number once in the contract comments so the server's `Welcome` builder can check against it.

### architect-r2 7 — MINOR — plan — Several statements still say "item 7" after the split into 7a and 7b

- **Statement.** plan.
- **Evidence.** `plan.md:160` says "Item 7 binds it to a session". The traceability table says "Items 3, 6, 7 and 9", "Items 6 and 7 consume only commands", and "Items 1 to 7 use the drafted messages". The sizing table has no row 7, and the sizing row for item 6 does not name its dependency on 7a.
- **Failure scenario.** A builder of item 4 reads "Item 7 binds it" and waits on 7a, which has no session, or a reviewer fails to find the owner of the session wiring.
- **Prescribed fix.** Replace each "7" with "7a" or "7b" as meant: the cursor binds to a session in 7b, `supervise` is 7a, and commands in R6 rows belong to 6 and 7b.

### architect-r2 8 — MINOR — D16 — Item 4 owns frame priority, but the frames belong to item 7b and the replay interface is unnamed

- **Statement.** D16, items 4 and 7b.
- **Evidence.** `plan.md:163` names only `replay`, `encodeReport` and `decodeReport` as shared. `plan.md:166` says item 4 gives heartbeat and result frames priority between events, and `plan.md:201` says item 7b's single owner orders frames. Item 4's tests exercise no heartbeat or result.
- **Failure scenario.** Item 7b needs to call the cursor, the ceiling update, the acknowledgement check and the discard on disconnect. Its builder invents the signatures, and item 4's tests, already frozen, do not match. A priority rule cannot be tested in item 4 without frames that do not exist yet.
- **Prescribed fix.** Add the methods of `replay` to item 4's "Names shared" with writer item 4 and reader item 7b. Move the priority sentence to item 7b with its test.

## Answers to the task questions

1. **Does each patch remove its failure without breaking another statement?** Mostly. Atomic decline (finding 1 and 2), the exit-0 stop (findings 3 and 4), the ack ceiling (nothing found beyond finding 8), the 14-second attempt deadline (nothing found: the 1, 2, 4, 8 and 15-second targets measured from each start stay within 15 seconds, and the 10-second exchange cap lies inside the 14-second cutoff), the C10 log fields (nothing found: `link_refused` with `code` matches `deploy/test/checks.sh:514` and `:530`, and `identity.json` fields `clientPublicKey` and `serverPublicKey` match `plan.md:106`), the single publisher (nothing found), and the direct `golang.org/x/sys` pin (nothing found: `go.mod` lists v0.29.0 indirect, v0.50.0 is in the module cache, and `ClockGettime` and `CLOCK_MONOTONIC_RAW` exist for Linux and macOS in `zsyscall_*.go` and `zerrors_darwin_*.go`).
2. **`UNREVIEWED MECHANISM` places.** There are four: the retention clock (finding 5), the 64 KiB cap (finding 6), the exit-0 stop (findings 3 and 4), and the stale-contract gate for Sprint 0 (nothing found: `commands.proto` and `events.proto` match the plan's use of `server_time`, the expiry rule and the mark rule).
3. **Rejected findings.** All four are backed by evidence that I confirmed.
   - logic-r1 2: `deploy/test/checks.sh:554` stops only the server, and `:566` and `:574` stop and resume the same client process.
   - logic-r1 5: `events.proto:21-22` ends one stream on a gap, and `:27-29` leave restored state out of wave 1.
   - logic-r1 6: `events.proto:8` keeps numbered events, and `checks.sh:560` needs at least three events.
   - critic-r1 12: `events.proto:30` requires fresh facts and inventory on approval.
4. **Writers and readers inside fixes.**
   - `Decline`: incomplete (finding 1).
   - The meaning of exit 0: incomplete (finding 3).
   - The replay interface: incomplete (finding 8).
   - The C10 fields: complete (writer item 7b, reader `deploy/test/checks.sh`).
   - `LGTM_CLIENT_STATE_DIR`: complete, since Stack Client rows at its plan lines 199 and 252 change the script, and `config.StateDir` at `src/client/internal/config/config.go:28` (variable at `:14`) reads the variable.
   - `RunOptions` and `clienttest.Options`: complete.
5. **Contradictions and truth tables.** One truth table is in finding 1, for the receipt states, and one in finding 5, for clock offset. Finding 2 is an interaction between the slot rule and the receipt rule. I found no other contradiction. Exit codes now read 0 for removal and for the listed faults, 75 for an accepted restart, and 1 for other failures (finding 3 and 4).
6. **Is an earlier blocker or major still open?** Yes. architect-r1 01, "fixed", is open: it depends on `Decline`, which does not exist (finding 1). The fix for logic2-r1 07 still leaves a `started` receipt without work on the busy-slot path (finding 2).
7. **Lens questions touched by patches.**
   - Q1 (claims about existing code): the `Decline` anchor does not resolve (finding 1). The pin, the clocks and the C10 anchors resolve.
   - Q2 (writers and readers): findings 1, 3 and 8.
   - Q3 (placement): `Decline` belongs to the same client-backend scope, so it moves into item 6 (finding 1).
   - Q4 (rules R1 to R11): R6 is open (findings 1, 2 and 5), R9 has finding 6, and the others are kept or do not apply.
   - Q5 (neighbouring plans): the Stack Client plan at its lines 30, 115 and 246 (finding 3). The Server Machines plan has no read cap to match (finding 6). The Sign-in plan pins `x/sys` v0.50.0 too, which makes the client's `go get` a no-op if it lands second, with no conflict.
   - Q6 (merge alone): item 6 cannot merge without `Decline`, and 7b, 8 and 9 follow it (finding 1).
   - Q7 (room for wave 2): the outage snapshot every 10 seconds exists to feed check C11 (`checks.sh:560`). It is bounded by the outbox limit of 10,000 events. Nothing found beyond that.
   - Q8 (dependencies): `golang.org/x/sys` v0.50.0 is real and has the two functions. The plan's phrase "matches `golang.org/x/net` v0.63.0" has no client use, since the client imports no `x/net`. It is harmless.

## Post-build checks

| Check | Scenario | Expected result |
|---|---|---|
| Decline atomic | Kill the client process between the `Decline` call and its commit, then after it, then retry the command. | Before the commit, no receipt exists and the retry declines again. After it, the retry replays EXPIRED and no work runs. |
| Busy slot | Run scan A, send pending scan B on the same stream, close the stream, wait for A to finish, reconnect. | No receipt for B exists after the close. B runs once and finishes `done`. |
| Store fault stop | Truncate `client.db` and start the client under the installed unit. | One error line names `client.db`, the status is 0, the unit stays inactive, zero dials happen, and a rerun of the installer says the cause may be a local fault, not only a removal. |
| Failed commit | Make the disk full during a `Finish` commit. | Status 1, and the unit restarts after 5 seconds. |
| Retention offsets | Set the test clock 2 hours and 30 hours ahead, and retry a finished command. | At 2 hours: EXPIRED, no run, stored receipt replaced by a declined one. At 30 hours with 1 hour of life left: the command runs again, as the accepted limit says. |
| Cap | Send a 70 KiB message from a test server. | The client closes the stream, logs one fixed line, and retries with no proof sent after the oversized challenge. |
| Ack race | Acknowledge the event being sent before `Send` returns, and acknowledge one above the sent ceiling. | The first is accepted. The second stops the run with one diagnostic that names `client.db`. |
| C10 | Run `deploy/test/checks.sh --only C10` with `LGTM_CLIENT_STATE_DIR` set by Stack Client. | `link_refused` with `unknown_machine`, then `server_key_mismatch`, within 20 seconds each. |
| C11 | Run `deploy/test/checks.sh --only C11`. | At least three buffered events during the outage, each applied once in order after recovery. |

## Grade

Grade: 6 of 10. Counts: blockers 0, majors 3, minors 5.

The patches answer most round-1 findings, and the C10 fields, the clock pin, the ack ceiling and the 14-second deadline all hold against the code. The main fault is that the central R6 patch depends on a store method that nobody owns. A second fault is that the exit-0 patch changes a rule that the Stack Client plan states differently. The fixes are local and the plan can ship after one more round.
