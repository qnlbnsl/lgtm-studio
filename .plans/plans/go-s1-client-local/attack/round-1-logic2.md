REVISE

Top finding: `Ack` beyond `Next()` deletes the new, unsent events that a client with an empty outbox produced before its first welcome from the server (logic2-r1 1).

Task identifier: review-go-s1-client-local-r1-logic2. Nonce: 89fb5f. Lens: logic. Plan reviewed: `.plans/plans/go-s1-client-local/plan.md`.

## Findings

### logic2-r1 1. MAJOR. Tied to C11.

**Statement:** an acknowledgement beyond the next number deletes events that were never sent.

**Evidence:** `.plans/plans/go-s1-client-local/plan.md:588` says `Ack(upTo)` removes the event files up to `upTo` and sets `next` to the larger of `next` and `upTo + 1`. `plan.md:590` says plan `go-s3-client-link` uses `Ack` beyond `Next()` when the server reports a higher last number. Decision 17 in `decisions.md:24` gives the same rule. `.plans/plans/go-s0-contract/plan.md:412` says `Welcome.last_event_number` is the highest number the server applied. `plan.md:540` gives `next` 1 on an empty folder. `TestAckBeyondNextMovesCounter` (`plan.md:599`) runs with nothing appended.

**Failure scenario:**
1. A machine loses its state folder, or is reinstalled with the same key, so the outbox is empty.
2. The client starts. Producers append a facts event and a harness event while the link is still connecting. They get the numbers 1 and 2.
3. The server answers with `last_event_number` 500.
4. The link calls `Ack(500)`. The outbox removes files 1 and 2, which are new events that were never sent, and moves `next` to 501.
5. The two events are lost, and nothing reports it. C11 says nothing is lost.

If the link instead sends first, the server drops numbers 1 and 2 as repeats, which loses them in the same way.

**Fix:** Add an explicit operation to item 7, for example `func (o *Outbox) Rebase(serverLast uint64) error`. When `serverLast` is higher than the mark, it renumbers every pending event to `serverLast+1` and upward, in order. For each event it writes the new file through `state.WriteAtomic`, then removes the old file, and a recovery rule finishes a crash in the middle. It then sets the mark to `serverLast`. Keep `Ack` for numbers that name events that really were sent, and make `Ack` refuse a number above `Next()-1` with an error. Add a test `TestRebaseKeepsUnsentEvents`: two appends on an empty outbox, `Rebase(500)`, then `Read` returns the same two payloads numbered 501 and 502. Add a kill test of `Rebase` at each step of the safe write. Change decision 17 and the line at `plan.md:590` to name `Rebase`.

### logic2-r1 2. MAJOR. Tied to D7.

**Statement:** a `join` that stops in the middle strands the machine, and the plan has no repair path.

**Evidence:** `plan.md:334` says `Create` writes through `state.CreateExclusive` and an existing file gives `ErrKeyExists`. `plan.md:382` says `CreateExclusive` never replaces a file and that plan `go-s3-client-link` decides what a second `join` does. `plan.md:149` and `plan.md:150` give the key and `server.json`, and `server.json` is only reserved. The layout (`plan.md:144` to `plan.md:156`) has no marker of how far a join got. The design's step 3 of "Joining" (design line 249) keeps the key on the machine.

**Failure scenario:**
1. `join` creates `machine.key`.
2. The call to `Join` fails: the server is down, the token is mistyped, the token is single use and the answer was lost, or the process is killed.
3. `server.json` is never written.
4. The next `join` finds a key that the server never learned. `Create` returns `ErrKeyExists`. The plan offers no way to replace the key, because every writer of the key refuses to replace it.
5. The owner must delete a file by hand. The program cannot tell "key exists and the server knows it" from "key exists and the server never saw it".

**Fix:** Add to the layout and to the contract of item 3 a state that tells these cases apart. For example, `server.json` is written only after the server confirmed the join, so "key present and no `server.json`" means "not joined". Add `func Replace(path string) (*Credential, error)` to item 3. It moves the old key to `machine.key.old-<unix time>` with a rename through the safe write, creates the new key, and is allowed only when `server.json` is absent. State in item 3 that `join` calls `Replace` in that case, and add `TestReplaceMovesOldKeyAside`, `TestReplaceRefusesWhenJoined` and a crash test of `Replace` at each step. Record in the layout table that plan `go-s3-client-link` must write `server.json` last.

### logic2-r1 3. MAJOR. Tied to R6 (design "Commands go down", lines 283 to 286).

**Statement:** receipts are keyed by the identifier alone, and the attempt number is stored but never compared, so a retry can be replayed as a failure forever.

**Evidence:** `plan.md:643` to `plan.md:646` give the four branches of `Begin`, and none compares `Attempt`. `plan.md:644` replays any receipt that is not `started`. `plan.md:650` says the replayed result is the result of the attempt that ran it. `plan.md:640` shows that an `interrupted` or `expired` receipt has an empty `result` and no status field that says failure. Decision 19 (`decisions.md:26`) says "the server can send a new command with a new identifier", while the design gives the command an attempt number. `TestSecondArrivalReplaysResult` (`plan.md:657`) covers only `finished`.

**Failure scenario:**
1. The client begins command C with attempt 1 and is killed. Its receipt becomes `interrupted`.
2. The server retries C as attempt 2 with a later expiry, because it never saw a result.
3. `Begin` returns `DecisionReplay` with the `interrupted` receipt, which has an empty result.
4. The link has nothing to send but an empty result. The server cannot tell success from failure. If it retries again, it gets the same replay for ever.
5. The same holds for `expired`: a command that arrives late the first time is replayed as `expired` for every later attempt, even with a fresh expiry.

**Fix:** In item 8, define what each replayed state means on the wire: add the field `Outcome` with the values `ok`, `interrupted` and `expired`, and make `Receipt` carry it, so the link sends a failure status and not an empty result. Decide in one sentence, in decisions.md, whether a higher attempt after `interrupted` or `expired` runs the command. Under the at-most-once rule of decision 19 the safe answer is that it does not run and the server must send a new identifier. If that is chosen, state it in item 8 and in the brief of plan `go-s3-client-link`. Add `TestReplayOfInterruptedCarriesFailureOutcome`, `TestReplayOfExpiredIgnoresLaterAttempt` and a truth-table test over the states none, started, finished, interrupted and expired, crossed with the same and a higher attempt, and with the expiry before and after `Now`.

### logic2-r1 4. MAJOR. Tied to R9.

**Statement:** `TestCredentialNeverPrintsKey` cannot fail when its own mutation is applied, so the guard of the private key is unproven.

**Evidence:** `plan.md:322` names mutation 4: removing `String` and `GoString` must make `TestCredentialNeverPrintsKey` fail. `plan.md:351` says the test fails only when the output holds the base64 or the hexadecimal text of the 32-byte seed. `plan.md:325` says `Credential` has unexported fields. Without the methods, `fmt` prints a struct by reflection and prints a byte slice as decimal numbers for `%v` and `%+v`, as raw bytes for `%s`, and as a Go literal for `%#v`. None of these is base64 or hexadecimal. `String` and `GoString` are on a pointer receiver (`plan.md:331`), so printing a `Credential` value, or a struct that holds one, uses no method at all.

**Failure scenario:**
1. A builder removes the two methods, or moves them to a value that no caller uses.
2. `fmt.Sprintf("%v", cred)` prints the key as `[12 200 41 …]`.
3. The test searches for base64 and hexadecimal text and finds none, so it stays green.
4. A log line leaks the private key, which R9 forbids.

**Fix:** Change the assertion of `TestCredentialNeverPrintsKey` to search for four forms of the 32-byte seed in every output: base64, hexadecimal, the decimal list `[a b c …]`, and the raw bytes. Format both `cred` and `*cred`, and a struct that holds a `Credential` value, with `%v`, `%+v`, `%#v`, `%s`, `%x` and `%q`. Give `Credential` value receivers for `String`, `GoString` and `Format(fmt.State, rune)`, or keep the key in a field of a pointer type that has its own `String` method. Add that the test also runs `json.Marshal` and `encoding/gob` over a struct that holds a `Credential` and that neither contains the seed.

### logic2-r1 5. MAJOR. Tied to C11.

**Statement:** an acknowledgement near the top of the 64-bit range wraps the counter to 0 and deletes real events, and a wrong high number can never be corrected.

**Evidence:** `plan.md:588` sets `next` to `upTo + 1` with no upper bound. `plan.md:540` says `recoverOutbox` removes every event file whose number is at most the mark. `plan.md:588` also says `Ack` does nothing when `upTo` is at most the mark, so the mark never goes down. The number comes from the server (`.plans/plans/go-s0-contract/plan.md:412`), and R9 and decision 22 treat text from outside as data. `TestAckBeyondNextMovesCounter` (`plan.md:599`) tests 41 only.

**Failure scenario:**
1. A buggy or restored server reports `last_event_number` 18446744073709551615, and the link calls `Ack(18446744073709551615)`.
2. `next` becomes `upTo + 1`, which wraps to 0. The mark is the largest 64-bit number.
3. The next `Append` writes `00000000000000000000.event`. On the next start, `recoverOutbox` removes it, because 0 is at most the mark.
4. For any smaller but wrong high number, such as 10 to the 15th power, the mark is stuck there and every later number starts above it. The plan has no way to lower the mark.

**Fix:** In item 7, make `Ack` and `Rebase` (finding 1) refuse with a new error `ErrAckOutOfRange` any number above a stated ceiling, for example `1<<62`, and any number that would make `upTo + 1` overflow. State in item 6 that `decodeEvent` and `recoverOutbox` refuse sequence number 0. Add `TestAckRefusesNumberNearMaxUint64`, which calls `Ack` with the maximum value and with `1<<62 + 1`, expects `ErrAckOutOfRange`, and checks that the next append still returns the old next number. Add `TestDecodeEventRefusesZeroSeq`.

### logic2-r1 6. MAJOR. Tied to C11.

**Statement:** one damaged outbox file denies the whole outbox for ever, and the plan names no repair.

**Evidence:** `plan.md:540` says `recoverOutbox` refuses with `ErrOutboxCorrupt` a file whose name and header disagree, a file that does not decode, and a gap. `plan.md:539` says an `acked` file that is not one decimal number gives `ErrOutboxCorrupt`. `plan.md:584` says `OpenOutbox` runs recovery, so `OpenOutbox` fails. `TestRecoverRefusesGap` (`plan.md:554`) tests only the refusal.

**Failure scenario:**
1. A disk error flips one bit in `00000000000000000007.event`, or an editor leaves a stray `acked` file with a second line.
2. Every start of the client returns `ErrOutboxCorrupt` from `OpenOutbox`.
3. The client cannot send events 8 and later, which are intact. The owner has to delete files by hand, and then loses all the events, which the plan did not choose.
4. Nothing says whether the client still runs without an outbox.

**Fix:** State the policy in item 6 and in decisions.md. Recommended: `recoverOutbox` moves a file that fails to decode into a new folder `outbox/corrupt/` through a rename, keeps the events before it, and returns in `recovered` a list `lost` of the numbers that were damaged. A damaged `acked` file is read as 0 and moved aside the same way. `OpenOutbox` then succeeds and the link reports the numbers in `lost` to the server. If the plan prefers refusing, state which command repairs the folder and what the client does while it is refused, and add the layout row for it. Add `TestRecoverQuarantinesDamagedFile` and `TestOpenOutboxSurvivesDamagedMark`.

### logic2-r1 7. MAJOR. Tied to C11.

**Statement:** the named mutation of `Append` and the steps of `Ack` are proven only by a timing test, and `Ack` has no crash test at all.

**Evidence:** `plan.md:568` says mutation 1 of item 7, returning the number before the file is written, must make `TestOutboxKillKeepsEveryNumberedEvent` fail. That test (`plan.md:607`) kills a child after a random 0 to 30 millisecond wait, 30 times. `TestOutboxCrashAtEachWriteStep` (`plan.md:608`) exits inside the write, so it cannot see a number that was returned too early. `TestAckIsDurableBeforeDelete` (`plan.md:601`) tests the order only at the first removal. No test crashes `Ack` between the mark and the removals, or at each step of the write of the mark.

**Failure scenario:**
1. A builder writes the file in a goroutine and returns the number at once. On a fast disk the write ends in under 30 milliseconds, so the kills mostly land after it, and the test stays green in most runs. The mutation survives, and a crash at a bad moment gives a number to an event that is not on disk.
2. A builder swaps the order in `Ack` so that the removal of one file happens before the mark, but not at the first removal. `TestAckIsDurableBeforeDelete` looks only at the first call and passes.
3. A crash during `Ack` between the mark and the removals is not exercised, so the claim of `plan.md:588` that recovery finishes the removal is untested at the outbox level.

**Fix:** Add three tests. `TestAppendReturnsOnlyAfterPlaced` sets the crash hook at the step `placed` to block, starts `Append` in a goroutine, and fails when `Append` returns before the hook is released. `TestAckDeletesOnlyAfterMarkAtEveryRemoval` makes the recorder of `removeFile` check the mark on disk at every call, not only the first. `TestOutboxCrashDuringAck`: for each of the five steps of the write of the mark, and for a stop after the mark and before each removal, a child exits during `Ack(3)` with five events, and after `OpenOutbox` the pending events are exactly 4 and 5, or 1 to 5 with the mark 0, and the next append returns 6. Add to item 7 that the timing test is a second line of defence and not the proof of mutation 1.

### logic2-r1 8. MINOR. Tied to `plan`.

**Statement:** the safe write can return an error after the file is already in place.

**Evidence:** `plan.md:246` says `WriteAtomic` renames, then syncs the folder, and "on any error it removes the temporary file". `plan.md:247` gives `CreateExclusive` the same shape after the link. Neither says what the caller must assume when the folder sync fails after the placement. `plan.md:646` and `plan.md:334` call the writers and treat an error as "not written".

**Failure scenario:**
1. `Begin` writes the `started` receipt. The rename works, and the folder sync fails with an input and output error.
2. `Begin` returns an error, and the caller does not run the command.
3. The receipt is `started` in this process, so every later arrival of the command gets `DecisionRunning` until a restart, and `Finish` is never called.
4. The same happens to `Create`: the key is in place, `Create` returns an error, and the retry returns `ErrKeyExists`.

**Fix:** In item 2, state that an error from `WriteAtomic` after the placement step wraps a new error `ErrPlacedNotSynced`, and that the file may or may not survive a power loss. Say in items 3, 7 and 8 what each caller does with it. For `Begin`, the receipt is rewritten as `interrupted` in memory and the command is not run. For `Create`, the key counts as created and `join` continues. Add `TestWriteAtomicReportsPlacedNotSynced` with `syncFolder` replaced by a failing function.

### logic2-r1 9. MINOR. Tied to C11.

**Statement:** an event that is on disk but whose `Append` never returned is repeated in content after a restart.

**Evidence:** `plan.md:586` writes the file and only then takes the number. `plan.md:608` allows event 6 to be "pending whole" after a crash, so event 6 is sent although the producer never learned its number. The design (lines 290 to 292) says events are kept until acknowledged and says nothing about producers that retry.

**Failure scenario:**
1. A producer appends "harness claude found" and the process is killed after the placement and before the return.
2. After the restart, the file is pending and is sent as number 6.
3. The producer, which never got a number, appends the same fact again as number 7.
4. The server applies both. The numbers are in order and unique, but the content is repeated, and an event that is not idempotent is applied twice.

**Fix:** State in item 7 that a producer must treat an `Append` that did not return as "may have been stored", and that every event of wave 1 must be idempotent or must carry a producer-chosen key. Add the sentence to the brief of plan `go-s3-client-link` and to the item 7 risk table, and name in the item 7 test table that `TestOutboxCrashAtEachWriteStep` is the proof of this accepted repeat.

### logic2-r1 10. MINOR. Tied to `plan`.

**Statement:** the outbox and the receipts are opened by path and not through the folder lock, and `Read` races with `Ack`.

**Evidence:** `plan.md:575` gives `OpenOutbox(dir string, …)` and `plan.md:633` gives `OpenReceipts(dir string, …)`, so a caller can open them without holding the `flock` of item 2 (`plan.md:243`, decision 12). `plan.md:587` says `Read` reads the payloads from disk. `plan.md:588` says `Ack` removes files. `plan.md:589` says all methods are safe to call at the same time, but the plan does not say whether `Read` and the removals share one lock.

**Failure scenario:**
1. Two clients with the same state folder both call `OpenOutbox` on `outbox/` without `state.Open`, which is possible because the function takes a path. Both hand out the same number.
2. In one client, the link calls `Read(0, 100)` while a second goroutine calls `Ack(50)`. `Read` lists number 20 and then finds its file removed, and returns an error or an event that was already acknowledged.

**Fix:** Change both open functions to take the held `*state.Folder`, and take `Paths.Outbox` and `Paths.Receipts` from it. State in item 7 that `Read` and `Ack` use the same mutex, and that `Read` copies every payload while it holds it. Add `TestReadDuringAckNeverFails`, which runs `Read` and `Ack` in two goroutines for 1,000 rounds under `-race`.

### logic2-r1 11. MINOR. Tied to `plan` (design "Commands go down").

**Statement:** "a command runs at most once" depends on a client clock that can move backward, and the plan has no guard.

**Evidence:** `plan.md:649` says `Prune` removes receipts more than `Keep` before `Now()`. `plan.md:645` decides expiry by `Now()` after `ExpiresAt`. Decisions 20 and 21 (`decisions.md:27`, `decisions.md:28`) record the clock as an assumption. `plan.md:711` rates the risk as medium and offers only the option `Now`.

**Failure scenario:**
1. A command finishes, and its receipt is pruned 24 hours later.
2. The clock of the client is set back by two days, for example after a wrong time on a restored virtual machine, or the server's expiry is long.
3. A duplicate of the command arrives with an expiry that is still in the future by the clock now. There is no receipt, so `Begin` returns `DecisionRun`, and the command runs a second time.

**Fix:** Keep a file `receipts/clock` that holds the highest `Now()` seen, written through `state.WriteAtomic` when it grows by more than a minute. `OpenReceipts` uses the larger of the real clock and that value as `Now()` for pruning and for expiry. Add `TestPruneNeverUsesClockBeforeHighWater` and `TestBeginUsesHighWaterAfterClockMovesBack`.

### logic2-r1 12. MINOR. Tied to D19.

**Statement:** the state folder is found from the environment at every start, so two starts of the same client can use different folders, and two clients share the default repositories folder.

**Evidence:** `plan.md:158` to `plan.md:162` resolve the folder from `LGTM_CLIENT_STATE_DIR`, then `XDG_STATE_HOME`, then `HOME`. `plan.md:196` gives the default repositories folder `<home>/.local/share/lgtm-client/repositories`, which is outside the state folder. The brief of plan `go-s3-client-link` runs `join` once and `run` as a service.

**Failure scenario:**
1. The owner runs `join` in a shell where `XDG_STATE_HOME` is set. The key and `server.json` go to that folder.
2. The service unit starts `run` with no `XDG_STATE_HOME`. It resolves `$HOME/.local/state/lgtm-client`, finds no key, and reports "not joined", or, after a second `join`, creates a second identity for the same machine.
3. Two clients with different state folders for two servers on one machine both clone into the same default repositories folder and can overwrite each other's clones.

**Fix:** State in item 1 that `run` writes the resolved folder to its first log line, and that the installer of plan `go-s3-client-link` sets `LGTM_CLIENT_STATE_DIR` in the service unit. Change the default of `repositories_folder` to a path inside the resolved state folder's parent that includes the folder's name, or require `settings.json` to name it once the state folder is not the default. Add `TestDefaultRepositoriesFolderDiffersPerStateDir`.

## Answers to the attack questions

**Lens 1 (orderings, ties, gaps, wraps, restarts).**
- Event numbers: the lock and the recovery give no tie, duplicate or gap, but the wrap and the stuck mark are open (finding 5) and sequence number 0 is not refused (finding 5).
- Restart: `next` is the larger of the mark and the highest number plus 1, which holds after a crash.
- A restored or lost outbox: the counter moves only through `Ack`, and that deletes unsent events (finding 1).
- Attempt numbers are stored and never compared (finding 3).
- Expiry and prune use a clock that can go back (finding 11).

**Lens 2 (each failure: permit or deny, and repair).**
- A crash in a write: permits and repairs, through the recovery of item 6 and the interruption of item 8.
- A damaged file: denies for ever, with no repair (finding 6).
- A failed join: denies, with no repair (finding 2).
- A sync error after placement: ambiguous (finding 8).
- A duplicate command after an interruption: replays an empty result (finding 3).
- Clocks that disagree: only an option, not a rule (finding 11).

**Lens 3 (truth tables).**
- Receipt state (none, started, finished, interrupted, expired) crossed with attempt (same, higher) and expiry (before, after `Now`): the cells "interrupted and a higher attempt" and "expired and a higher attempt with a later expiry" are empty, because the plan defines no outcome (finding 3).
- `acked` mark crossed with event files (above, at, below, missing): the cell "mark lost and files above 1 remain" is a gap error with no repair (finding 6).
- Key crossed with `server.json`: the cell "key present and no `server.json`" has no meaning (finding 2).

**Lens 4 (zero values).**
- Zero `Decision` is invalid, which is good.
- `Attempt` 0 and a zero `ExpiresAt` are refused.
- `OutboxOptions` fields of 0 mean the defaults, so no limit can be set to zero.
- Sequence number 0 is not refused by `decodeEvent` (finding 5).
- The `ReceiptState` zero text and an unknown state in a receipt file are not said to be refused. This is a post-build check.
- `Read(0, n)` is defined.

**Lens 5 (two actors on the same state).**
- Two clients on one folder: the `flock` of item 2 stops them, but only when the opener holds it (finding 10).
- `Read` and `Ack` on the outbox (finding 10).
- `join` and `run` on the key (finding 2).
- Two `Begin` calls for one command: serialised, with `TestConcurrentBeginRunsOnce`.

**Lens 6 (risky items: do the mutations bite, and what slips through).**
- Item 2: the five mutations each have a test that goes red. A change that slips through is a wrong owner check in `Open`, which has no test, because the owner check cannot be tested without the rights of a superuser.
- Item 3: mutation 4 does not go red (finding 4). The owner branch of `Load` has no test.
- Item 6: the four mutations bite. A change that slips through is accepting sequence number 0.
- Item 7: mutation 1 is caught only by a timing test (finding 7). A change that slips through is removing a file between the mark and the first removal, in a loop.
- Item 8: the four mutations bite. A change that slips through is running a higher attempt after `interrupted`, which the plan has not decided (finding 3).

**Lens 7 (the cheapest wrong implementation that passes every test).**
- Item 3: print a `Credential` by value (finding 4).
- Item 7: a goroutine that writes the file after the number is returned, which passes on a fast disk (finding 7).
- Item 8: accept any 36-character string of hexadecimal digits and hyphens as an identifier. It passes the listed tests, and it is safe as a file name, so it is not a defect.
- Item 2: `Open` that calls `Chmod` on a folder that exists passes all tests but one, and `TestOpenRefusesExposedFolder` does check that the mode is unchanged.

**Lens 8 (a different shape that would make a wrong state impossible).**
- A join state recorded in the folder would make "key without a server" impossible (finding 2).
- Opening the outbox through the held folder would make a second writer impossible (finding 10).
- An `Outbox.Rebase` that is separate from `Ack` would make the deletion of unsent events impossible (finding 1).
- A value type for the credential, with its own `Format`, would make a printed key impossible (finding 4).

**Task question a (the machine key).**
- The file is made atomically: `CreateExclusive` writes a temporary file of mode 0600, syncs it, links it to `machine.key` so that it never replaces a file, removes the temporary file and syncs the folder (`plan.md:247`, `plan.md:334`).
- When the file exists: `ErrKeyExists`, and the file is unchanged.
- When it is unreadable, exposed, a link, not regular, or owned by another user: a named error from `Load` (`plan.md:335`). The owner branch has no test.
- When it is damaged: `ErrKeyInvalid`, with no repair.
- When it was never learned by the server: no repair and no marker (finding 2).

**Task question b (outbox and receipts after a crash).**
- The next number is the larger of the mark and the highest event file number, plus 1. In `TestOutboxCrashAtEachWriteStep` it is 6 when event 6 is missing and 7 when it is pending.
- No event that got a number is lost, because the number is returned after the placed and synced file.
- An event without a returned number can be repeated in content (finding 9).
- Reordering cannot happen inside one client, because the lock gives numbers in order and `Read` sorts by number. The reordering after a lost outbox is the deletion of finding 1.
- Proofs: `TestOutboxKillKeepsEveryNumberedEvent`, `TestOutboxCrashAtEachWriteStep`, `TestOutboxNeverReusesNumbers` and `TestRecoverNextAfterAllAcked`. Their gaps are in findings 1, 5, 7 and 9.
- Receipts: `TestReceiptsKillNeverRunsTwice` and `TestReceiptsCrashAtEachWriteStep` prove at-most-once. They do not prove a useful answer to a later attempt (finding 3).

**Task question c (the layout of the state folder).**
- It holds the key, `server.json`, `outbox/` and `receipts/`, which plan `go-s3-client-link` needs for join, connect, reconnect and commands.
- It lacks: a join-progress marker (finding 2), an outbox identity or a rebase rule for the first connect (finding 1), a place for quarantined files (finding 6) and a clock mark (finding 11).
- Two clients on one machine collide through the default repositories folder, and through different environments that resolve different folders (finding 12). The `flock` stops two clients on the same folder, but only when both hold it (finding 10).

## Post-build checks

1. **Fsync on macOS.** Scenario: run `TestWriteAtomicCrashAtEachStep` and the kill tests on a Mac with an APFS volume. Expected: the same results as on Linux, and `F_FULLFSYNC` used by `os.File.Sync`.
2. **Hard links.** Scenario: put the state folder on a file system without hard links, such as some network shares. Expected: `Create` fails with a clear error, and no `.tmp-` file is left.
3. **`flock` on a network file system.** Scenario: two clients on one shared folder. Expected: the second refuses, or the plan documents that a shared folder is not supported.
4. **Owner check of `Load` and `Open`.** Scenario: run as a superuser, `chown` the key to another user, and call `Load`. Expected: `ErrKeyNotOwned`. Do the same for the folder and `ErrFolderNotOwned`.
5. **Printing a credential by value.** Scenario: format `Credential`, `*Credential` and a struct that holds one with every verb. Expected: no form of the seed appears (finding 4).
6. **Unknown receipt state.** Scenario: write a receipt file with `"state":""` and then `"state":"weird"`, and open the receipts. Expected: the file counts as damaged and becomes `interrupted`, and `Begin` never returns `DecisionRun`.
7. **Sequence number 0 and the largest number.** Scenario: place `00000000000000000000.event`, and call `Ack` with the largest 64-bit number. Expected: recovery refuses the first, and `Ack` refuses the second (finding 5).
8. **Kill tests under load.** Scenario: run the kill tests with `-race` and `-count=20` on a slow disk. Expected: no flake, and the logged seed reproduces a failure.
9. **Real loss of power.** Scenario: cut the power of a test machine during a run of appends and acknowledgements. Expected: every number that was returned is pending or acknowledged, in order.
10. **Cross-compile.** Scenario: `go build` of `./src/client/...` for linux and darwin on both processor families with `CGO_ENABLED=0`. Expected: all four exit 0.

## Grade

5 out of 10. Blockers: 0. Majors: 7. Minors: 5.

The crash-safe write method, the file formats, and the tests for each crash step are sound and well tied to the design. The majors are about what the plan leaves undecided around them: a rebase of the outbox, a repair path for a failed join and for a damaged outbox, the meaning of a replay after an interruption, and tests that cannot fail when their mutation is applied.
