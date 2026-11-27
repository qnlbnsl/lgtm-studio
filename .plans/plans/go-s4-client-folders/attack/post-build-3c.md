# Post-build review 3c: item 3 of the plan go-s4-client-folders

## Cross-vendor statement

The builders and both test writers of this item were OpenAI Codex workers.
This review was written by a Claude reviewer, claude-sonnet-5-5 at high effort, so the cross-vendor rule is met.
The task identifier is `postbuild-go-s4-client-folders-3c`, and the nonce is `260c8c`.
The reviewed range is `git diff 8acd8fd6..02170d09 -- src/client/internal/link src/client/internal/app` on the branch `sprint-0`.
The range was extracted with `git archive` into a scratch folder and tested there, because later commits of item 7 exist on the branch.

## Verdict

**REVISE.**

The top finding is that every failure to open `client.db` is classified as damaged history, so a permission fault now ends in a clean exit and an instruction to throw the saved state away (post-3c 1).

## What was run

The plan has no `UNREVIEWED FIX` entry for item 3, so there was no marked place to check first.
The post-build scenario "History, upgrade and supervisor" belongs to the stack team and needs the integrated candidate, so it was not run here.
No native macOS run was made.

The plan's acceptance command was run in the scratch copy at `02170d09`:

```
go test -v -race ./src/client/internal/link ./src/client/internal/app -run 'TestRevisionTwoTerminalHistory|TestRevisionTwoExecutableStop|TestRetryFailureExitsAndKeepsStore|TestAppRunStoreStatus' -count=1
```

It printed `PASS` for all four tests, then `ok` for the link package at 1.948 seconds and `ok` for the app package at 2.556 seconds.
The amended tests, the new `TestRevisionTwoHandshakeFencesBeforeClose` and the full `./src/client/...` run also passed with `-race -count=1`, with no skips and all 11 packages `ok`.
The builder's workaround for a sandbox that owns the root folder as user 65534 was not needed here.

## The code against the plan

Item 3 does what its "Done when" row says.
The saved counter is limited to 1 through 2^63 at inspection time, receipt envelopes are checked before the first dial, and the replay range is validated before the Welcome mark is acknowledged.
A required history fault or an incompatible Welcome returns `RunRecoveryStop` with a nil error and one fixed diagnostic, the app exits 0, and a requested restart still exits 75.
The write fence is taken before transport teardown on both the store-fault path and the verified revision refusal, and the handshake cleanup in `conn.go` fences before `s.close()`.
The one difference from the plan text is that receipts are sealed after the first valid Welcome instead of at startup.
The builder explained it in its result file, and the order is safe because nothing handles a command before the seal.

## Test amendments

None of the three amendments weakened a test.
`TestAppendRefusesWhenFull` now expects `ErrStoreDamaged` and unchanged bytes for the two counters outside 1 through 2^63, and a scratch mutant that removed the event-count limit still failed it on the original assertion.
`TestRevisionTwoReceiptRunValidatesBeforeTransport` and `TestRevisionTwoRunAnnouncement` now expect `RunRecoveryStop` with a nil error, which is the plan's outcome, and both keep their zero-dial, byte-preservation and one-diagnostic assertions.
The valid counter at 2^63 is still tested by "last legal event drains".

## Mutations tried

Each mutation was applied alone in the scratch copy and run against the link and app tests.

| Mutation | Result |
|---|---|
| Acknowledge before validating the range | Three subtests of `TestRevisionTwoTerminalHistory` fail. |
| Accept a saved counter above 2^63 | `TestAppendRefusesWhenFull`, the "next above limit" subtest and the executable "invalid counter" subtest fail. |
| Retry a verified revision refusal | `TestRevisionTwoHandshakeFencesBeforeClose` and the "incompatible welcome" subtest fail. |
| Remove the event size limit in replay decoding | The "oversized required event" subtest fails. |
| Remove the store write fence | `TestRevisionTwoHandshakeFencesBeforeClose` fails, and nothing else does. |
| Move the handshake fence after `s.close()` | `TestRevisionTwoHandshakeFencesBeforeClose` fails. |
| Skip the missing-number check in replay validation | No test fails. |
| Remove the below-floor check in replay validation | No test fails. |
| Accept a mark equal to `next` | No test fails. |
| Settle the scan after a store-fault recovery | No test fails. |
| Settle the scan after a store-fault recovery and remove the write fence | The "late scan cannot settle after recovery" subtest and the handshake test fail. |

The surviving mutants are equivalent to the original behavior, as post-3c 3 explains, so no wrong implementation that every test accepts was found.

## Findings

### post-3c 1

**Severity: MAJOR. Design: the section "The client in wave 1" and `plan` item 3.**

Statement: every failure to open `client.db`, whatever its cause, is turned into damaged history, so a permission or input and output fault ends in a clean recovery stop with an instruction to restore history or enroll again.

Evidence: `storeOpenError` in `src/client/internal/link/store.go` (about line 371) wraps every error except a timeout, a symbolic-link loop and the two store errors in `ErrStoreDamaged`.
`recoveryStoreCode` in `session.go` (line 355) maps that to `store_failed`, `stopForStore` (line 366) logs "Restore the original client.db history or remove this machine and enroll again with fresh state" and returns `RunRecoveryStop` with a nil error, and `src/client/internal/app/client.go` exits 0 on that outcome.
The function itself is older than the range, but item 3 is what made the classification matter, because before item 3 the same error exited 1.
A scratch test made `client.db` mode 0 and called `OpenStore`.
It returned `client.db: link: store is damaged: open …/client.db: permission denied`, with `ErrStoreDamaged` true and `recoveryStoreCode` equal to `store_failed`.
No test covers an open failure that is not a history fault.

Failure scenario: an owner restores files from a backup tool that leaves `client.db` unreadable, or the disk reports an input and output error during boot.
The client prints that it cannot be fixed without removing the machine, exits 0, and a service that restarts only on failure leaves the machine offline for good.
The owner may wipe the state, and the saved events and receipts are lost, although the file is intact.

Fix: classify only errors that prove the file is damaged, such as a bolt format, checksum or version error and a missing or empty file, as `ErrStoreDamaged`.
Map the remaining open errors to `ErrStoreFailed` with the plain message, and add a test that an unreadable `client.db` does not produce the recovery instruction.

### post-3c 2

**Severity: MINOR. Design: `plan` item 3, risk "supervisor restart loop".**

Statement: the faults that are not history faults, namely an unsafe store file and a failed write, still exit 1 with the one line "Could not use client.db." and no instruction.

Evidence: the tail of `stopForStore` in `session.go` (lines 376 to 383) returns `RunStopped` with an error for `ErrStoreUnsafe` and `ErrStoreFailed`, and `client.go` turns an error into exit 1.

Failure scenario: a store file whose mode lets others read it is refused at every start, and a supervisor that restarts on failure retries every few seconds without end, with no text telling the owner to change the mode.

Fix: for the unsafe case, name the mode problem in the fixed message, and decide with the stack team whether the service unit limits restarts.
This is not a regression, because the behavior is the one before item 3.

### post-3c 3

**Severity: MINOR. Design: `plan` item 3, rule "validate before acknowledging".**

Statement: several guards in the new code are redundant with each other, so no test can tell a missing one from a present one, and only the pair of the stop-time settle guard and the write fence is tested together.

Evidence: with the missing-number check removed, `validateReplayBatch` in `replay.go` (line 57) still fails, because the cursor then runs past the end and meets an empty payload.
The below-floor check at `validateReplay` (line 45) is also covered by that effect, and a mark equal to `next` is stopped by `Ack`, which reports `ErrAckAhead`.
In `Run` (`session.go`, lines 112 to 117) the skip of `settleScan` and the fence in `updateContext` (`store.go`, line 415) each stop a late write on their own.
Removing both makes the "late scan cannot settle after recovery" subtest fail, and removing only the fence fails only the handshake test.
With a started receipt added to the "welcome below retained floor" fixture, removing both replay checks still passed, so the saved bytes stay unchanged for that reason too.

Failure scenario: a later change that deletes one guard looks safe to every test, and a later change that also deletes its twin is the first to be noticed.
No present behavior is wrong.

Fix: none is needed now.
If the redundancy is intended, name it in a comment at each guard, so that a later builder does not remove the last one.

## Defects checked and not found

No race was found.
Every write path (`appendContext`, `Ack`, `begin`, `recordResult`, `prepareRun` and the receipt writers) holds the store lock when it reaches `updateContext`, and `fenceWrites` takes the same lock, so the `fenced` flag is read and written under it.
No leak was found, because `store.Close()` runs on every return of `Run` after a successful open, and the app ignores a folder close error only after a recovery stop.
No secret reaches a log or an error text on the recovery path, because the recovery diagnostics are fixed sentences and the underlying open error with its path is dropped before it is returned.
The bolt panic guard in `openExistingBolt` still maps a memory fault to `ErrStoreDamaged`.

## Grade

7.5 out of 10.

The item does what its row says, the frozen tests are strong, and the three amendments follow the contract.
The grade is held back by the misclassification in post-3c 1, which has a plain fix and one missing test.
