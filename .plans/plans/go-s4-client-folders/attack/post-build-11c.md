# Post-build review 11c of the plan go-s4-client-folders: the repair after review 10

Cross-vendor rule: met. The test writers and the builders of this repair were Codex workers, and this review was done by a Claude worker (claude-sonnet-5-5, high). The reviewer did not build the code and did not write its tests.

Task identifier: postbuild-go-s4-client-folders-11c. Nonce: 5640ea. Branch: `sprint-0`. Range: `git diff a1a7b850~1..6ed68e9c -- src/client`.

## Verdict

REVISE.

## Top finding

A sign-in file read that stalls at reconnect is reported as `ErrStoreDamaged`, which ends the client with a terminal recovery stop that tells the owner to restore `client.db` or enroll again, although the store is intact (post-11c 1).

## What the repair closes

- Post-10 1 is partly closed. The folder settlement path is bounded: the 1 second screen runs off the owner loop, a report that cannot be screened is withheld, and the command ends FAILED at the current watermark. The frozen test `TestStalledFolderSettlementSignInReadDoesNotBlockLink` passes. A persistent hang of the sign-in reader still takes the control path away, because the frozen test holds only the worker's read and the append's read, not every read (post-11c 2, post-11c 1).
- Post-10 2 is closed for overlapping `refreshWithin` samplers. The mutation that removes the `sampling` wait is killed by the tests. One caller still samples without registering the value before other text is classified (post-11c 4).
- Post-10 3 is closed. It is a tests-only item, and the test passes.

## Findings

### post-11c 1 — MAJOR — A stalled screening at reconnect is reported as store damage and is terminal

Statement: `newReplayWithFilter` maps a failed `lockSecrets` to `ErrStoreDamaged`, and `session.go` turns that into a `storeFault`, so the owner is told to restore or re-enroll.

Evidence: `src/client/internal/link/replay.go:38-40` and `replay.go:117-123` return `ErrStoreDamaged` when `lockSecrets` fails. The retained-event branch in `session.go` does the same for `errScreeningUnavailable`. `session.go:391` maps `ErrStoreDamaged` to the recovery stop, and the app exits with status 0 and the text "Restore the original client.db history or remove this machine and enroll again with fresh state."

Failure scenario: The sign-in reader hangs for longer than one second (a slow or stuck file system). The stream dies at the result gate (post-11c 2), the client reconnects, and replay screening times out after one second. In a scratch run with every read held after the worker's, the log showed `stopForStore code store_failed` and then `run returned status 0`. The store was intact. A transient stall becomes a terminal state, and an owner who follows the text may discard the history.

Fix: Return a retryable error from a failed screening (the link retries with backoff), and keep `ErrStoreDamaged` for real damage. Design tie: plan item 6 (the owner can still receive heartbeats, replays and restart) and the recovery rules in the foundation design.

### post-11c 2 — MAJOR — A persistent hang of the sign-in reader still removes the control path

Statement: The result-send gate and the command-accept gate fail the whole stream or the command when a one-second screen of the peer identifiers fails, so a hung reader blocks results, heartbeats and a signed Restart.

Evidence: `session.go:750-757` returns `errInvalidFrame` when `lockSecrets(ctx, peerIdentifiers)` fails, and `errInvalidFrame` ends the stream (`session.go:368`). `commands.go:168-171` returns `ErrInvalidCommand` for the same failure, so a Restart command is refused while the reader hangs. Each gate also blocks the owner goroutine for up to one second per attempt.

Failure scenario: With every read after the worker's held, the scratch run showed the result gate fail at about five seconds, the FAILED result for the folder command never sent, and no heartbeat, no restart result and no exit 75. The client log shows nothing, because the outgoing handler drops records when screening is unavailable. The plan's item 6 says the owner can still receive heartbeats, replays and restart while a read is stalled.

Fix: Check peer identifiers against the values already registered, without a fresh sample, or fail only the one operation and not the stream. Move the blocking waits off the owner goroutine. Add a test that holds every refresh call, not only the worker's.

### post-11c 3 — MAJOR — A spurious `ErrStoreDamaged` from a race in `newReplayWithFilter`

Statement: `screened := through < store.Next()-1` is read before `validateReplay` takes the store lock, so an append between the two turns an ordinary replay into reported damage.

Evidence: `replay.go:36` computes `screened`, and `replay.go:63-64` returns `ErrStoreDamaged` when `!screened && through < s.events.next-1`. A scratch stress test with three appender goroutines and no filter, under `-race`, produced `tries=62 appended=308 spurious ErrStoreDamaged=1`.

Failure scenario: The owner loop appends a report while a reconnect builds its replay. The stream setup fails with store damage and the client takes the terminal recovery stop of post-11c 1.

Fix: Compute `screened` inside `validateReplay` under the store lock, or retry the construction once.

### post-11c 4 — MINOR — The path check samples the sign-in file without registering it

Statement: `folderPathUnsafe` calls `f.refresh()` outside the lock and does not count in `sampling`, so a sample read there is registered late and another gate can classify text in between.

Evidence: `filter.go:306` onward. A scratch test held the path check's first read, ran `MaskText` with that first sampled value in the text, and the output kept the value: `LEAK: output gate emitted the unregistered first sample`.

Failure scenario: A sign-in file is replaced while a folder path check is reading it. A log line or report that contains the old value is emitted unmasked during the window. This is the same class as post-10 2, in the one caller the repair did not move to `refreshWithin`.

Fix: Count the path check's sample in `sampling` and register it before releasing the permit, or use `refreshWithin` there.

### post-11c 5 — MINOR — A stalled screen holds a single slot, and later folder reports fail at once

Statement: `folderScreens` has capacity one and is released only when `appendFilteredReport` returns, so a stalled screen makes every later folder report end FAILED immediately.

Evidence: `commands.go:117` (capacity one), `folder_command.go:142-151`. The `refreshWithin` slots (two for local text, one for peer identifiers) are held in the same way until `refresh()` returns, and `refreshSecrets` called directly from `appendFilteredReport` and `screenFolderReport` bypasses them.

Failure scenario: One hung sign-in read makes each later `LookAtFolder` command fail until the read returns, with no diagnostic for the owner.

Fix: Say in the plan that this is the intended bound, give the owner a diagnostic code, and add a test that the second command recovers after the read returns.

### post-11c 6 — MINOR — The one-second screen deadline starts before the owner picks up the completion

Statement: The context from `finishFolder` is created when screening starts, and the prepared closure checks `ctx.Err()` again on the owner, so owner delay can fail a report that was screened in time.

Evidence: `folder_command.go:128-131` and the `ctx.Err() != nil` check in the prepared closure in `outbox.go`.

Failure scenario: A busy owner loop (for example during a restart send, which is allowed two seconds) picks up the completion after one second, and a screened report ends FAILED. The result is safe but needlessly lossy. The one-second allowance is separate from and after the five second inspection budget, so a command can take six seconds before FAILED, still inside the 30 second stream silence limit.

Fix: Start the deadline for the screen only, and check it once when screening ends.

### post-11c 7 — MINOR — Local text can starve under sustained overlapping samplers, and records are dropped silently

Statement: `canScreen` for local text needs `sampling == 0`, so a steady stream of overlapping refreshes can hold a screen until its one-second timeout, and `MaskText` then returns an empty string and `Push` returns an empty string with no warning.

Evidence: `filter.go:203`, `app/outgoing.go` `Handle` drops the record.

Failure scenario: Under load, diagnostics vanish without a trace. This is safe for secrets but hard to debug.

Fix: Count the dropped records and emit one summary line after screening recovers.

### post-11c 8 — MINOR — Test gap: no test fails when the damaged-history check is removed

Statement: The mutation that disables `!screened && through < s.events.next-1` in `validateReplay` passes the whole link package.

Evidence: In the scratch copy, changing `replay.go:63` to `false && through < s.events.next-1` left `go test -race -count=1 ./src/client/internal/link` printing `ok ... 54.250s`.

Failure scenario: A wrong implementation that accepts a replay ceiling below the store tail without screening is accepted by every test, and a lost or unscreened event would be sent.

Fix: Add a test with a store whose ceiling is below its tail and a filter that is not screening, and expect `ErrStoreDamaged`.

### post-11c 9 — MINOR — Test gap: the frozen settlement test holds only two reads

Statement: The stall fixture matches the worker's path-check read and the append read by call-stack inspection, so the peer-identifier gates, the reconnect replay and the restart path are not exercised with a hung reader.

Evidence: `src/client/internal/app/folder_command_review10_test.go`; my scratch variant that holds every read failed at the settlement, heartbeat and restart checks (see post-11c 1 and post-11c 2).

Failure scenario: The defects of post-11c 1 and 2 pass the suite.

Fix: Add a variant that holds every read, with the expectations of the plan's item 6.

## Order, loss and duplication of events waiting for screening

I found no loss, duplication or reorder. In the code read, the handoff ceiling advances only after the event is handed off, registration stays locked through handoff in `replay.next`, and a current-run event that cannot be screened waits for the next heartbeat tick (`eventScreenDeferred`) without leaving the outbox. The only way an event stops being sent is the terminal exit of post-11c 1 and 3, which keeps the event in the store.

## The places no reviewer read before the build

The item 6 deadline wrapper (threat-r1 07): the scenario of a stalled read at settlement was run and passes (the frozen test). The scenario of a read that never returns, with a restart, was run in a scratch variant and fails (post-11c 2).

## Tests run (scratch copy of the head commit, as an ordinary user, with `-race`)

- `go test -race -count=1 ./src/client/internal/link` with the filter, folder lifecycle, event budget and unacknowledged window tests: `ok 5.385s` for the named subset, and `ok 54.250s` for the whole package after restoring the files.
- `./src/client/internal/app`: `ok 26.535s`, including `TestAppLogRedactsSecrets` and `TestStalledFolderSettlementSignInReadDoesNotBlockLink`.
- `./src/client/clienttest`: `ok 1.188s`.
- Mutation, removing the `sampling` wait (`canScreen` always true): the filter tests fail, so it is killed.
- Mutation, disabling the damaged-history check in `validateReplay`: nothing fails (post-11c 8).
- The paired suite `src/linktest` and the full `./src/client/...` suite were not run. The builder's result file says the paired suite passes.

## Grade

6 out of 10. The settlement path and the overlapping refresh are well bounded and tested, and the ordering of waiting events is sound. The repair turns a stalled reader into a terminal false store-damage state and still lets a hung reader take away the control path.
