# REVISE

The code of the direct screening is correct as far as I could judge it, but four of the mutants that remove the sign-in read from one emission path survive every test, so a later change could stop screening the retained events, the command results, the command admission or the log lines without any test failing.

## Reviewer and vendors

The vendors differ. The test writer and the builder were OpenAI Codex workers. I am a Claude worker, model `claude-sonnet-5-5`, and I neither built this code nor wrote its tests.
The task was `postbuild-go-s4-client-folders-13c` with nonce `615835`.
The review covers `git diff de9c12b7~1..6f87ac41 -- src/client` on the branch `sprint-0`.
The commits of this part are `de9c12b7` (tests) and `6f87ac41` (build). The commit `31792bc3` belongs to another part, and I did not review it.
I ran all tests in a scratch copy of `6f87ac41`.

## What I ran

- `go test -race -count=1 ./src/client/...` printed `ok` for all eleven packages. Four tests skip on this host, as they did before: `TestLinkCommandProcess`, `TestOpenStoreRefusesFileOfAnotherOwner` and the two resolved-path cases of `TestInspectFolderRefusals`.
- The paired suite `bash deploy/test/with-postgres.sh -- go test -count=1 ./src/linktest` is listed under "Paired suite" below.
- I applied the mutations below, one at a time, to the scratch copy and ran `go test ./src/client/internal/link` or `./src/client/internal/app` after each one.

| Mutation | Result |
| --- | --- |
| The heartbeat path takes the lock without reading the sign-in file | Killed by `TestDirectScreeningWaitsBeforeHeartbeat`. |
| `appendFilteredReport` takes the lock without reading | Killed by `TestDirectScreeningLeavesStoreAvailableDuringRead`. |
| `screenReplay` takes the lock without reading | Killed by `TestDirectScreeningLeavesStoreAvailableDuringRead`. |
| `screenReplay` holds the store lock while it reads | Killed by `TestDirectScreeningLeavesStoreAvailableDuringRead`. |
| `folderPathUnsafe` takes the lock without reading | Killed by `TestFolderPathSampleIsRegisteredBeforeTextEmissionReview11`. |
| `lockSecrets` reads before it takes the lock (the old race) | Killed by both amended fixtures. |
| `lockSecrets` reads only while the registry is empty | Killed by both amended fixtures and by `TestDirectScreeningOverlappingReportsDoNotWaitForHeartbeat`. |
| The command result path takes the lock without reading | **Survived.** |
| `replay.next` takes the lock without reading | **Survived.** |
| `commandHandler.handle` takes the lock without reading | **Survived.** |
| The log handler emits the screened record after the lock is released | **Survived.** |

## Findings

### post-13c 1 — MAJOR — Four emission paths can lose their direct read without any test failing

**Statement.** Decision 39 says that the sign-in file is read directly before each emission.
The tests require that read on four paths: the heartbeat, the report admission, the replay creation and the folder path check.
No test requires it on four other paths, namely the emission of each retained event by `replay.next`, the emission of a command result, the admission of a command, and the emission of a log record.

**Evidence.**
- `src/client/internal/link/replay.go:133` takes `r.filter.lockSecrets()` before `reportTextSafe` and the handoff of each event. With `lockSecrets` replaced by a lock that does not read, the whole `./src/client/internal/link` package still prints `ok`.
- `src/client/internal/link/session.go:739` does the same before a command result is sent, and `src/client/internal/link/commands.go:169` does it for the admission of a command. Both mutants survive.
- `src/client/internal/app/outgoing.go:134` emits the log record inside `WithTextMask`. A mutant that stores the handler call and runs it after `WithTextMask` has returned survives all tests of `./src/client/internal/app`, so no test requires that registration and emission happen under one lock for log lines.
- I wrote one extra probe test in the scratch copy. It creates a replay on an empty store, appends an event whose client version is a value, then makes the sign-in reader return that value, then calls `next`. The built code returns `ErrStoreDamaged` and hands nothing off. The mutant for `replay.next` hands the event off with the value in it. The probe is the evidence that the survival is a gap in the tests and not an artifact of my mutation. I did not keep the probe.

**Failure scenario.** A later change moves the read out of `replay.next` into the replay creation, for speed. A person replaces the sign-in file after the replay is created. The next retained event with the old or new value in its client version leaves the client unmasked, and every test passes. The same holds for a result whose command identifier contains a value that was written after the command was admitted.
This ties to the plan's item 8 ("each recheck is a direct read on the path that emits"), and to the secrecy check (a) of the plan's "Post-build checks".

**Fix.** Add four tests that hold the read at the path and then change the file.
- For `replay.next`, use the probe shape above.
- For the command result, queue a result and hold the reader. Require that no frame leaves, and that the frame is refused when the identifier contains the value that the released reader returns.
- For `handle`, send a command whose identifier contains a value that only the second read returns, and require `ErrInvalidCommand`.
- For the log handler, hold the writer and require that a second record waits for the lock until the first is written.

### post-13c 2 — MINOR — A held read also stops the shutdown, and issue 1709 does not say so

**Statement.** The owner loop of `serve` cannot see `ctx.Done()` while it is inside `lockSecrets`, because the read has no context. After the signal handler of the client cancels the process context, the loop stays in the read, and the closing `link_stopped` log line waits on the same lock.

**Evidence.** `src/client/internal/link/session.go:737` to `src/client/internal/link/session.go:758` read and send under `lockSecrets` inside the `case <-dispatch` branch, and `src/client/cmd/lgtm-client/main.go:26` handles `SIGTERM` by cancelling a context only. Issue 1709 in `docs/issues.md` states that the client sends nothing and logs nothing, that a restart command does not arrive, and that the service manager or a person restarts it. It does not say that a normal stop request is also ignored.

**Failure scenario.** The sign-in file is on a stalled network mount. A person runs the service manager's stop. The client does not exit until the manager's own timeout ends it with a kill, and its state folder is left without a clean close. This is within the spirit of the accepted limit, so I rate it MINOR.

**Fix.** Add one sentence to issue 1709 and to item 8 of the plan: a stop request does not end the process until the read returns, and the manager's kill is the exit. No code change is needed under decision 39.

### post-13c 3 — MINOR — The filter lock now covers the disk commit and the write of the log line

**Statement.** The one lock is held while the store commits (`appendFilteredReport` calls `appendAtNext` under it, and `commandHandler.handle` holds it across `begin`, `recordResult` and `finish`) and while the log handler writes the screened record to its output. A slow disk or a full error-output pipe now stops the link, the heartbeat and every other log line, even when the sign-in file reads quickly.

**Evidence.** `src/client/internal/link/outbox.go:30` to `src/client/internal/link/outbox.go:60` and `src/client/internal/link/commands.go:169` hold the lock across durable commits. `src/client/internal/app/outgoing.go:134` calls `handler.Handle` inside the lock; the diff shows that before this build the handler was called after `WithTextMask` returned.

**Failure scenario.** The service manager's log collector stops reading the error output, and the pipe fills. The next log line blocks in its write while it owns the filter lock. The owner loop cannot send a heartbeat or a result, and the server shows the machine offline. The accepted limit in issue 1709 names a held read of the sign-in file, not a blocked log sink.

**Fix.** Either accept this in issue 1709 in one sentence, or move the durable commit and the log write outside the lock after the screening, keeping the read, the registration and the classification of one record under one lock. If the log write moves out, post-13c 1 needs the fourth test to check that no record is classified with an older registry than the one that was read for it.

## Judgement on the five questions of the task

**(a) Secrecy.** I found no weakening in the code.
The masking, the refused folders and the withheld origins still go through `encodeFilteredReport`, which this build did not change.
A retained event is checked by `reportTextSafe` under the read at `src/client/internal/link/replay.go:133`, and a missing screening gives `ErrStoreDamaged` at `src/client/internal/link/replay.go:66`.
The path sample is registered under the same lock as the text, and two mutants of the old race were killed.
The replay decision uses `replayRange` under the store lock at `src/client/internal/link/replay.go:46` to `src/client/internal/link/replay.go:50` and releases it before the read, and `TestDirectScreeningLeavesStoreAvailableDuringRead` fails if the store lock is held during the read.
The lock order is always the filter lock and then the store lock, and the reporter's lock comes before both. I found no path where the store lock is held while the filter lock is requested, and no logging call under the filter lock inside the link package, so the non-reentrant lock cannot deadlock on the paths I read.
Gaps in how well the tests hold this are post-13c 1.

**(b) Loss, doubling and order.** A held read blocks only the owner loop, which is the one goroutine that hands anything to the sender, so frames keep their order. `replay.next` sets the ceiling after the handoff, so an acknowledgement cannot get ahead of what was handed over. The heartbeat's next tick is computed after the read, so a long hold sends one heartbeat and not a burst. `TestDirectScreeningRestartDrainsQueuedResults` checks the order and the number of queued results through the Restart. If the two-second Restart budget runs out during a held read, some queued frames are not sent, but their receipts are already finished, and the server's re-sent command is answered from the receipt after the restart, so nothing is lost or doubled.

**(c) Removed machinery.** `grep` over `src/client` finds none of `errScreeningUnavailable`, `secretRefreshBudget`, `folderScreens`, `preparedReport`, `settling`, `startupReports`, `refreshResult`, `canScreen` or `reportScreen`. The findings post-12 1 and post-12 2 are gone. `TestDirectScreeningOverlappingReportsDoNotWaitForHeartbeat` covers post-12 1 and the third finding, post-12 3, which the stall tests missed.

**(d) Retired tests and amended fixtures.** The four retired tests required behavior that decision 39 removes: progress while a read is held, settlement under the former deadline, and a retry after a screening timeout. They were the only tests in the three deleted files, so no other check was lost with them. The old race mutants are killed by the two amended fixtures, so they are as strong as before for what they claim. One note on them: when the first reader owns the lock, the `synctest.Wait()` before the second call is skipped, so the check that runs before the release is weak. The check after the release uses a buffered channel and still sees a leak, so the fixtures keep their strength.

**(e) Folder inspection deadline.** `startFolder` is unchanged by this diff. The five-second budget, the permit pool of two and the wrapper that returns an unreadable result at the deadline are as before, and the deadline still applies when the inner call waits on the filter lock.

## Paired suite

I ran `bash deploy/test/with-postgres.sh -- go test -count=1 ./src/linktest` once on the scratch copy. It printed `ok` after 138.6 seconds. I did not repeat it, so I add nothing to the builder's three passes and eight passes of the two formerly unsteady tests. The script removed the container that it started.

## Grade

7 out of 10. The production change is small, matches decision 39, and removes the machinery that caused post-12 1 and post-12 2. The grade is held down by the four paths whose direct read no test protects.
