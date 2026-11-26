REVISE

# Client link, round 2, logic seat

- Task identifier: review-go-s3-client-link-r2-logic. Nonce: 266b8d.
- Change attacked: `git diff 6d416f17 80e5857a` on `plan.md` and `decisions.md` of this plan folder.
- Finding identifiers use the prefix `logic-r2`.

## Top finding

Item 6 builds its false-start repair on a `Store.Decline` method that exists nowhere (not in `src/client/internal/link/receipts.go`, not in the Client Local plan), that the item's own file list does not create, and whose "freeze after Client Local agrees" gate has no owner (logic-r2 1).

## Findings

### logic-r2 1. MAJOR (the repaired false start cannot be built as written). Statement R6.

- Evidence.
  - `plan.md:248` calls a proposed `Store.Decline(c Command, result []byte) (Decision, []byte, error)` and names its writer as "Client Local".
  - `plan.md:243` says "Freeze the tests after Client Local agrees to `Decline`". `plan.md:46` (Rollout step 1) says only "Confirm the Sprint 1 receipt interface for item 6".
  - `src/client/internal/link/receipts.go` has `Begin` (line 65) and `Finish`, and no `Decline`. `.plans/plans/go-s1-client-local/plan.md` contains no `Decline`. Client Local is already built, so no open Client Local item will write it.
  - The Files list of item 6 (`plan.md:242`) names only `commands.go` and `commands_test.go`.
  - The only test of the method, `TestDeclineSurvivesKillBeforeFinish` (`plan.md:252`), sits in `commands_test.go`, and it needs a real `Decline` to run. No store-level test is listed for the method itself.
- Failure scenario. The builder of item 6 reaches the call at `plan.md:248`, finds no method, and has three choices. It can stop and wait for an agreement that nobody owns. It can write the method inside `commands.go` through the store's private fields, which is a seam around Sprint 1 code that the owner's "edit, do not work around" rule forbids. Or it can use `Finish` on an absent identifier, which fails with `ErrNotRunning` (`Finish` requires `s.running[id]`), so the decline is never recorded and the false-start failure of architect-r1 01 returns unchanged. Any of the three means the repair is not removed "as written".
- Fix. Make item 6 edit `src/client/internal/link/receipts.go` and `src/client/internal/link/receipts_test.go` directly, and add both to its Files list. Delete the agreement gate from `plan.md:243` and `plan.md:46`. Write the store-level tests first, in `receipts_test.go`, with these cells.
  - Absent identifier: writes one finished receipt holding the given result, returns `DecisionExpired`, and leaves `s.running` without the identifier.
  - A finished receipt exists: returns `DecisionReplay` with the stored bytes, and does not overwrite them.
  - A started receipt exists and is in `s.running`: returns `DecisionRunning`.
  - A started receipt exists and is not in `s.running`: returns `DecisionInterrupted`.
  - A receipt that fails to decode: returns `DecisionInterrupted`.
  - An oversize result, an invalid identifier and a zero `ExpiresAt` return the same errors as `Begin`.
  - A reopen after the commit replays the stored result through `Begin`.

### logic-r2 2. MAJOR (a requested scan can end with no defined result). Statement C7.

- Evidence. `plan.md:180` says "A newer commit discards older unnumbered snapshots of that kind, including late scan completions" and "A requested scan whose report cannot commit fails". `plan.md:250` says a scan yields DONE "only after the report commits" and FAILED for "a full buffer or canceled scan". A scan whose report was discarded because a newer inventory committed first is in neither sentence.
- Failure scenario.
  - A `SCAN` command starts and its scan runs for several seconds.
  - The stream drops and reconnects. The approved welcome queues a fresh inventory (`plan.md:204`), which starts after the scan and finishes first.
  - The scan's own report is now older than a committed inventory, so item 5 discards it.
  - Under the DONE rule the command never reaches DONE, because its report never committed. Under the FAILED rule it reaches FAILED only if "cannot commit" counts a discard, which the text does not say.
  - Either the command hangs in `DecisionRunning` with no outcome or the server records FAILED for work that was in fact superseded by a fresher, committed inventory. `TestLinkCommandAttemptsAcrossReconnect` ("accept its result on the latest attempt") cannot pass deterministically.
- Fix. State in `plan.md:250` that a requested scan answers DONE when its own report commits, or when an inventory whose capture started after the command was received has committed. It answers FAILED only when neither can happen (full buffer with no later commit, cancellation). Add one truth table to item 6 with the rows "own report commits first", "newer inventory commits and own report is discarded", "buffer full and nothing newer commits" and "canceled", and add the second row to `TestCommandScanRequiresDurableReport` and to `TestReportBackpressure`.

### logic-r2 3. MAJOR (a repairable local fault ends the service for good). Statement D16.

- Evidence.
  - `plan.md:203` makes `ErrStoreMissing`, `ErrStoreUnsafe`, `ErrStoreDamaged`, `ErrSequenceExhausted` and impossible acknowledgements "exit 0 to stop the service loop until repair" (decision row 46).
  - The service restarts only on non-zero exits (`.plans/plans/go-s3-stack-client/plan.md:113`, `Restart=on-failure`; macOS `KeepAlive` with `SuccessfulExit` false). Exit 0 therefore stops the machine until a person intervenes, and nothing in any plan restarts it after the repair.
  - D16 says the machine returns by itself. `ErrStoreMissing` is returned when `client.db` is absent while the store is opened without create (`src/client/internal/link/store.go:117-118`). A state volume that is not yet mounted at boot, or a partial restore, produces exactly that. An impossible acknowledgement comes from a server that has just passed a verified challenge, so a server defect also stops every machine for good.
  - The mapping is not defined where the exit is made. `plan.md:200` says `Run` returns `(RunOutcome, error)` and "A nonnil error takes precedence". `plan.md:262` (item 8) returns 0 "on cancellation or permanent stop" and 1 "for other failures". A local permanent stop is an error, so it is either "permanent stop" or "other failure", and the text names the sentinels only in item 7b.
  - `TestRetryFailureExitsAndKeepsStore` (`plan.md:206`) claims to check "status 0" but lives in the `link` package, where only `(RunOutcome, error)` is visible. `TestAppPreservesStateOnRemoval` checks status 0 for removal only, and `TestRunCommandExitStatus` (`plan.md:277`) lists the cases "cancellation or permanent stop" without a local store stop.
- Failure scenario. The cheapest wrong implementation returns the store error from `Run`, so the app returns 1. Every test passes. The service then restarts the client every 5 seconds forever with an error line each time, which is the opposite of the plan's intent, and for a mounted-late volume it would have recovered by itself while the plan's own choice of 0 would not.
- Fix. Choose one of two shapes and write it into item 7b and item 8.
  - Preferred: keep exit 1 and the service retry for `ErrStoreMissing` and `ErrStoreUnsafe`, which an operator or a later mount repairs. Reserve exit 0 for the verified removal and for cancellation. For `ErrStoreDamaged`, `ErrSequenceExhausted` and impossible acknowledgements use a distinct status (for example 78) and add it to the Stack templates as `RestartPreventExitStatus` (systemd) and to the macOS `SuccessfulExit` rule. The Stack plan then owns that line, and the plan names the reader.
  - Either way, give `Run` a typed local-stop error (`ErrLocalStop` wrapping the cause) so item 8 can map it without a sentinel list, and add a test in `client_test.go` or `main_test.go` that drives the real mapping to a status for each sentinel. Add an installer-level check next to `live_reconnect_restarts_service` for the chosen status.

### logic-r2 4. MINOR (the accepted retention limit is described wrongly and is untested). Statement R6.

- Evidence. `plan.md:26` and decision row 49 say a fast clock can make a retry "lose its outcome or repeat work". `Begin` (`src/client/internal/link/receipts.go:98`) checks a finished receipt first and expiry second. `Finish` prunes by the local wall clock (`plan.md:250`).
- Failure scenario. A clock that is fast by less than 24 hours prunes a finished receipt only after the real expiry has passed. A retry then arrives with an admission bound at or after expiry, `Begin` returns `DecisionExpired`, and `Decline` records EXPIRED for a command that actually ran. The server records a wrong final result, and the contract line "a machine answers every attempt until 24 hours after expires_at" (`src/contracts/proto/lgtm/link/v1/commands.proto:12`) is broken. Repeated work needs a skew of 24 hours or more, so that the receipt is pruned while the real time is still before expiry. The plan's text names only the second case.
- Fix. The owner's ruling stands, so keep the limit but state both outcomes in `plan.md:26`, and add a test `TestFinishPrunesByAge` cell that pins the first outcome (EXPIRED after a prune by a fast clock for a command already run). The blocker logic2-r1 01 is therefore "accepted", not "fixed", and the register should say so.

### logic-r2 5. MINOR (`Decline` has no pruning and no failure path). Statement R6.

- Evidence. Only `Finish` prunes (`plan.md:250`). `plan.md:248` says "Store errors prevent work" and "Reply after commit" but does not say what the machine sends when the `Decline` commit fails.
- Failure scenario. A server that resends many expired commands under unique identifiers on every stream causes one finished receipt per identifier. A machine that never runs work never calls `Finish`, so the store grows to its limit. When the decline commit fails, no reply is sent, the server resends it on the next stream, and the loop continues until the store recovers, with no stated result.
- Fix. Have `Decline` run the same prune as `Finish` inside its transaction, and state in item 6 that a failed `Decline` commit sends no reply, closes the stream, and returns the store error to the supervisor (item 7b maps `ErrStoreFailed` to exit 1). Add the case to `TestDeclineSurvivesKillBeforeFinish`.

### logic-r2 6. MINOR (the C10 fixture depends on an unbuilt change in another plan). Statement C10.

- Evidence. Rollout step 3 (`plan.md:47`) requires Stack Client to pass `LGTM_CLIENT_STATE_DIR`. `deploy/test/checks.sh:219` and `deploy/test/checks.sh:233` still use `XDG_STATE_HOME`, and `check_c10` reads `$state_home/lgtm-client/identity.json` (`deploy/test/checks.sh:521`).
- Failure scenario. If Stack Client lands first in a different form, `check_c10` edits a file the client never reads and `wait_refusal server_key_mismatch` times out after 20 seconds. The fixture also rewrites `identity.json` with `printf ... >` after `jq`, which keeps mode 0600 only while the file already exists, and `LoadServer` requires a private regular file (`plan.md:106`).
- Fix. Keep the rollout gate and add it to the post-build checks below. No change to the plan text is needed beyond naming the check.

## Answers to the lens questions

1. Sequences, ties, gaps, duplicates, wraps and restarts. Checked the reconnect targets (1, 2, 4, 8, 15 seconds, cancel at 14 seconds plus 1 second of teardown, so at most 15 seconds between starts) and found no defect. The 30-second silence rule fits the server's 10-second heartbeat acknowledgement (`events.proto:109-111`). The event gap still ends only one stream (`events.proto:21`). One new defect: a discarded scan completion has no outcome (logic-r2 2).
2. Failures and repair. The decline commit failure and the local-store stop have no defined repair (logic-r2 3, logic-r2 5). Other patched failures (unknown reasons retry, removal stops, gap reconnects) are stated and consistent with the contract table (`link.proto:40-56`).
3. Truth tables. Built for `Begin` × `Decline`: a finished receipt, a started receipt in `s.running` and one not in it, an absent receipt before or after expiry, all cover the cells listed in logic-r2 1. Built for the exit status: cancellation, removal, accepted restart, local stop, failed commit. The local-stop cell is contradictory between `plan.md:200` and `plan.md:262` (logic-r2 3). Built for scan completion: one cell is empty (logic-r2 2).
4. Zero values. `RunStopped=0` is also the zero value of `RunOutcome`, so an unset outcome with no error reads as a clean stop. Because `Run` returns an error on every failure path in the plan, this is not a defect today. A zero `ExpiresAt` or `now` is rejected by `Begin` (`receipts.go:71`). `Ack(0)` and the first number were checked and are consistent.
5. Two actors on the same state. Two attempts of one command calling `Begin` then `Decline` are ordered by the one process owner (`plan.md:249`), and a second `Decline` returns the first one's finished receipt. Nothing found beyond logic-r2 1 and logic-r2 5.
6. Risky items and mutations. Item 6's mutation "skip recording a decline" fails `TestDeclineSurvivesKillBeforeFinish` only once `Decline` exists (logic-r2 1). Item 7b's mutation "stop on a gap" fails `TestRetryResumesAfterEventGap`. No listed test fails the mutation "return the store error instead of status 0" (logic-r2 3).
7. Cheapest wrong implementation. Item 6: answer FAILED for every scan whose report was discarded; it passes every listed test (logic-r2 2). Item 7b: return the store error and exit 1 (logic-r2 3). Items 7a and 5: nothing found.
8. Shapes that make a wrong state impossible. A typed local-stop error (logic-r2 3) and one store transaction that both declines and prunes (logic-r2 5) would remove the two ambiguities found. Nothing else found.

## Answers to the task questions

1. Patches against the register. The Decline patch (architect-r1 01, logic-r1 1, logic2-r1 03) removes the false start only once the method exists (logic-r2 1). The snapshot ordering patch (logic2-r1 06) removes the ordering failure and creates the undefined scan outcome (logic-r2 2). The cadence patch (logic2-r1 04, logic-r1 3, critic-r1 4) removes its failure and breaks nothing. The exit 0 patch (critic-r1 5, architect-r1 05) conflicts with D16 (logic-r2 3). The C10 diagnostics patch (critic-r1 2, architect-r1 02) is consistent with `deploy/test/checks.sh:506-532`. The rest were read and break nothing.
2. `UNREVIEWED MECHANISM` places. Item 7b exit 0 (logic-r2 3); the retention limit at `plan.md:26` (logic-r2 4); the 64-kibibyte read cap (item 2, `plan.md:104`) and the trust ruling in Rollout step 1: nothing found for the last two.
3. Rejected findings. logic-r1 2: the evidence holds, since `check_c11` (`deploy/test/checks.sh:550-588`) uses `kill -STOP` and `kill -CONT` on the same client process. logic-r1 5: holds (`events.proto:21`, lines 26-28 and issues 1641 and 1642). logic-r1 6: holds (`checks.sh:560` requires three buffered lines, and `events.proto` preserves numbered reports). critic-r1 12: holds (`events.proto:30`).
4. Writers and readers. The `Decline` list is incomplete (the writer file is missing, logic-r2 1). The `link_refused` fields have writer item 7b and reader `deploy/test/checks.sh`, which is complete. The `golang.org/x/sys` v0.50.0 pin is present in the module cache with `ClockGettime` for linux and darwin, and the root `go.mod` pins v0.29.0 indirect, so the first import needs the direct pin that item 3 names. The exit-status reader (Stack templates) is not named (logic-r2 3).
5. Contradictions between patches. Item 5 against item 6 on discarded scans (logic-r2 2). Item 7b against item 8 on local stops (logic-r2 3). Item 6's retention sentence against decision row 49 is consistent but imprecise (logic-r2 4).
6. Round 1 blockers or majors still open. Blocker logic2-r1 01 stays open by the owner's ruling, not fixed (logic-r2 4). Blocker logic2-r1 02 (contract parity) is handled by the retry default, which matches the contract table. The false-start majors are not fixed until logic-r2 1 is applied. Major critic-r1 5 is replaced by logic-r2 3.
7. Lens questions touched by patches. Answered above: 1, 2, 3, 4, 5, 6, 7 and 8.

## Post-build checks

1. Drive a scan whose own report is discarded by a newer committed inventory. Expect DONE (or the outcome the plan chooses) in the server's command record, never a stuck running command.
2. Kill the client between the `Decline` commit and the reply, then restart and let the server resend. Expect the stored EXPIRED result on the first attempt and no work done.
3. Delete `client.db` after a join, start the client under the real service template, and watch two restarts. Expect the status the plan chooses and, for a restartable fault, the client back online once the file returns.
4. Set the clock 12 hours fast, finish a restart command, advance past expiry, replay the command. Expect the recorded outcome the plan states for that skew.
5. Run `deploy/test/checks.sh --only C10` after Stack Client lands. Expect both `link_refused` codes and 13 passes in the full run.

## Grade

6 out of 10. Blockers: 0. Majors: 3. Minors: 3.
