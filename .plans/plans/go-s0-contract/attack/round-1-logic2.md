# Review of `go-s0-contract`, round 1, seat `logic2`

- Task identifier: review-go-s0-contract-r1-logic2
- Nonce: 9fdbdc
- Lens: logic and failure policy
- Plan read: `.plans/plans/go-s0-contract/plan.md`, with `decisions.md`, `scouting.md` and the files in `draft/`.

## Verdict

REVISE

## Top finding

The plan keeps the contract revision at 1 although the website's only use of `ReconnectClient` is on offline machines, which the new comment makes the one case the server always refuses.

## Findings

### logic2-r1 1: the revision stays at 1, but the website's reconnect button can then never succeed

- Severity: MAJOR.
- Tied to: D16.
- Evidence:
  - The plan says that the website keeps working without a change and that only a comment changes (`plan.md:37-39`, `plan.md:58`, `plan.md:648`).
  - The website offers the action "Try to reconnect" only when a machine is offline (`src/frontend/src/screens/service.tsx:351`, the item guarded by `off &&`).
  - The sample server accepts the call only for an offline machine (`src/mock-server/src/engine.ts:236-239`).
  - The new comment makes the real server refuse exactly that case with `failed_precondition` (`draft/existing-files.diff`, lines 17-19).
  - The contract's README says that a change that breaks an older website raises the revision (`plan.md:648` quotes the rule).
- Failure scenario: the owner opens the Machines page against the Go server, sees an offline machine and chooses "Try to reconnect". The server answers `failed_precondition` every time, because the machine is not connected. The button is always an error toast, and the sample server and the real server now disagree on the precondition.
- Fix: the plan must stop calling this a comment-only change. It must name the meaning change of `ReconnectClient` as a break for a revision-1 website. It must then do one of two things, and say which. Either raise the revision to 2 in this sprint, and name the files that carry the number (`src/frontend/src/server/api.ts:15`, `src/mock-server/src/engine.ts:47`) as an agreed change for the orchestrator. Or keep revision 1 and add, to the agreements with other teams, a required change of `service.tsx` (show the action for online machines) and of the sample server (precondition "online"), merged in the same pull request as this sprint. The "Done when" row 5 must then check that change, not only the `grep` of the revision.

### logic2-r1 2: an event buffer that restarts at 1 is dropped silently and then acknowledged

- Severity: MAJOR.
- Tied to: C11.
- Evidence: `draft/events.proto:17-19` (the server applies an event only when its number is higher than the last one it applied, and drops it otherwise), `draft/events.proto:31` (`EventAck.through` covers events that are "applied or dropped"), `draft/link.proto:141-142` (`last_event_number`), `plan.md:258-259`. Nothing says where the machine's next number comes from after the machine lost its counter.
- Failure scenario: a machine has had events up to number 40 applied. Its state folder is restored from an older copy, or its event file is deleted while its key stays. The machine counts again from 1. Its new `Facts` event with number 1 is dropped as a duplicate, and the server acknowledges it, so the machine forgets it. Every report of this machine is lost until its counter passes 40. The website shows old facts and an old list of harnesses, and nothing reports an error.
- Fix: write one rule in the comments of `Event.number` and of `Welcome.last_event_number`: the next number a machine uses is greater than the highest number it ever used and greater than `Welcome.last_event_number`, and the machine keeps the highest number it used even when its buffer is empty. Add a second rule to the same comments: after every `Welcome` the machine sends its current `Facts` and `HarnessesFound` as new events, because they are snapshots and a server whose rows were lost can then recover them. Add a post-build check for the first rule (see the checks below).

### logic2-r1 3: a `Restart` can repeat forever, because nothing says the receipt is written first

- Severity: MAJOR.
- Tied to: D16.
- Evidence: `draft/commands.proto:28-30` and `plan.md:256` say only that "the machine sends its result before it ends". `draft/commands.proto:3-5` says that a machine keeps a receipt of each command it ran, without saying when.
- Failure scenario: the server sends `Restart` with attempt 1. The client ends its process, and the receipt that would say "done" was only in memory, or the result never left the machine because the process ended first. The new process opens a stream. The server has no result, so it sends `Restart` again as attempt 2. The client has no receipt, runs it, and ends again. The machine restarts every few seconds and never settles. The receipt rule of the design cannot help, because the receipt does not exist.
- Fix: add to the comment of `Restart` and to item 4: the machine writes the receipt of a `Restart` with the outcome `COMMAND_OUTCOME_DONE` to disk and flushes it before it sends the result, and it ends only after both. A `Restart` that already has a receipt is answered from the receipt with the new attempt number and is not run again.

### logic2-r1 4: the end of a replaced stream can mark a connected machine offline

- Severity: MAJOR.
- Tied to: C9.
- Evidence: `draft/events.proto:60-61` (the server marks a machine offline after 30 seconds without a heartbeat), `plan.md:260` (and the design's line "marks a machine offline when its stream closes"), `draft/link.proto:34` (a new stream replaces the older one, which ends with `aborted`). No sentence says which stream owns the machine's presence.
- Failure scenario: a network blip leaves the server holding stream A. The machine dials stream B. The server handles `Welcome` on B and marks the machine online. Stream A then ends with `aborted`, and the handler of A runs "stream closed, mark offline" after the handler of B. The machine is connected on B and shown offline. If only `Welcome` sets a machine online, it stays offline until the next restart of the client, which breaks check C9. Two handlers also write the same "last event applied" row of this machine in the same moments, and the plan does not say they share one transaction rule.
- Fix: add to the comment of `Open` and to item 6: the end of a stream marks the machine offline only when that stream is still the machine's current one. The 30 second timer belongs to the current stream. A heartbeat never marks a machine online, and only a verified `Welcome` does. The last applied event number is read and written in one database transaction that locks the machine's row.

### logic2-r1 5: the life of a command has no table for outcomes, expiry or clocks

- Severity: MAJOR.
- Tied to: R6.
- Evidence: `draft/commands.proto:19-22` (expiry "by its own clock"), `draft/commands.proto:36-38`, `plan.md:252-254`, and the risk row at `plan.md:562` ("the client team decides what it does with" the server time).
- Failure scenario: a client's clock runs two minutes fast, and the server gives commands an expiry one minute ahead. Every command arrives "expired" by the client's clock. The client answers `COMMAND_OUTCOME_EXPIRED` for every `ScanHarnesses`, and the harness list never fills. A second case: a machine ran a command, the stream broke, and its result arrives after the server's own expiry time. The plan does not say whether the server stores it or ignores it, and it does not say whether the server sends a command again after `FAILED`, `EXPIRED` or `UNSUPPORTED`. The plan also does not say how long the machine keeps receipts. A receipt dropped before `expires_at` lets a command run twice.
- Fix: add one table to the comment of `Command` and say it in item 4. First: `DONE`, `FAILED`, `EXPIRED` and `UNSUPPORTED` are final, and the server stores them and never sends that command again. Second: the server marks a command expired itself when its expiry passes by the server's clock without a result, and a result for the newest attempt that arrives later is still stored. Third: the machine tests expiry against its own clock corrected by the difference between `Welcome.server_time` and its own time when `Welcome` arrived. Fourth: the machine keeps a receipt at least until one hour after `expires_at`.

### logic2-r1 6: the client has no table of which codes mean "stop" and which mean "try again"

- Severity: MAJOR.
- Tied to: C10.
- Evidence: `plan.md:133` lists seven codes without any consequence. `decisions.md:32` gives as its reason that the client team needs this, and then does not give it. `draft/link.proto` states no retry rule. No code is listed for a database that is down or a server that is stopping.
- Failure scenario: the owner removes a machine while it is offline. The server has forgotten its key. The machine reconnects and gets `unauthenticated`, which is also what a wrong key gets. One client team reads it as "stop for good", another as "retry every 15 seconds". A server that is stopping or whose database is down answers with a code that the table does not list, and a client that stops on an unknown code never comes back (check C9). A second case: a connection that has gone silent with no close. No rule tells the client when to give up, although the server gives up after 30 seconds. The client shows itself connected while the server shows it offline.
- Fix: add a table to the comment of `LinkService`. Rows: `Join` with `permission_denied` stops and tells the user; `Join` with any other code retries every 15 seconds; `Open` with `aborted` reconnects at once; `Open` with `unauthenticated`, `failed_precondition`, `deadline_exceeded`, `unavailable` or any code not listed retries every 15 seconds, and the client logs one clear sentence for `unauthenticated` ("the server does not know this machine"). Add the rule: a client that receives no message from the server for 30 seconds closes the stream and reconnects, and the server's `HeartbeatAck` makes that silence a signal.

### logic2-r1 7: nothing ties the token the website shows to the `JoinToken` text

- Severity: MAJOR.
- Tied to: D7.
- Evidence: `src/contracts/proto/lgtm/v1/clients.proto:102-108` (`CreateClientTokenResponse.token` is "the secret a client presents once"), `plan.md:127` and `draft/link.proto:40-44` (the token's text is `lgtm1_` plus the encoded `JoinToken`, and the website only shows it), `plan.md:361` (`lgtm-client join <address> <token>` has both an address and a token that holds an address).
- Failure scenario: the server team fills `CreateClientTokenResponse.token` with the encoded secret, as the old comment says. The client team reads `JoinToken` text. `lgtm-client join` cannot parse the token, and sprint 3 finds it. A second case: the command line says `https://a.example` and the token says `https://b.example`, and nothing says which one the client dials.
- Fix: item 3 already edits `clients.proto`. Let it also replace the comment of `CreateClientTokenResponse.token`: it holds the text of a `JoinToken`, and `expires_at` is 15 minutes after the token was made. Add to the comment of `JoinToken.address`: the address in the token is the one the client dials, and a client refuses a command-line address that differs from it.

### logic2-r1 8: `Join` has cells in its truth table that contradict each other

- Severity: MINOR.
- Tied to: C5.
- Evidence: `draft/link.proto:26-29` and `plan.md:349-350` say that the same token with the same key returns the same reply, and also that a used token is refused.
- Failure scenario: the table has token states (fresh, used by key K, expired, revoked) against requests (key K, another key). "Used by K" with K is both a success and a refusal. The case "machine M joined with this token, the owner removed M, the same client sends `Join` again within 15 minutes" has no cell. The server may answer with the machine id of a machine that no longer exists, and the next `Open` then fails with `unauthenticated`.
- Fix: say "used" means used by another key. Say that a replay with the same key returns the same reply only while the machine that the token registered still exists, and that otherwise the server refuses with `permission_denied`.

### logic2-r1 9: acknowledgements have no timing, and an event of an unknown kind has no rule

- Severity: MINOR.
- Tied to: C11.
- Evidence: `draft/events.proto:28-32` (what `EventAck` means, not when it is sent), `plan.md:415` (a receiver ignores a message of a kind it does not know), `plan.md:34` (wave 2 adds report kinds without a new link revision).
- Failure scenario: a server sends `EventAck` before it commits the event, crashes, and the machine has already deleted the event. A second case: a client of wave 2 sends event number 9 with a report kind that a wave-1 server does not know. "Ignores" can mean "no acknowledgement", so the machine keeps number 9 and sends it again after every reconnection, for ever.
- Fix: add to the comment of `EventAck`: the server sends it only after the database commit of the events through that number. Add: an event whose report the server does not know is dropped and is covered by the acknowledgement like any dropped event.

### logic2-r1 10: zero and unknown values of the standing and of the outcome have no meaning

- Severity: MINOR.
- Tied to: R6.
- Evidence: `draft/commands.proto:46-48` and `draft/commands.proto:74-76` (the zero values are `_UNSPECIFIED`, which is the right shape). No sentence says what a receiver does with `APPROVAL_UNSPECIFIED`, with a value it does not know, with a `Welcome` that has no `state`, or with `COMMAND_OUTCOME_UNSPECIFIED`. `Hello.machine_nonce` has no stated length rule either (`draft/link.proto:107-108`).
- Failure scenario: a newer server sends an approval value 4. A client written as "run work unless waiting or removed" runs work for a machine the owner did not approve. A buggy client sends an outcome of 0, and a server written as "anything but failed is done" stores it as done. A machine nonce of 3 bytes is accepted and signed.
- Fix: add to the comments: a machine treats `APPROVAL_UNSPECIFIED`, an unknown approval and a missing `state` as waiting, and does no work; a server stores `COMMAND_OUTCOME_UNSPECIFIED` and an unknown outcome as failed; both nonces are exactly 32 bytes, and any other length ends the stream with `invalid_argument`.

### logic2-r1 11: no message or rule says when a machine counts as ready, or who sends the first facts

- Severity: MINOR.
- Tied to: C7.
- Evidence: `src/contracts/proto/lgtm/v1/clients.proto:66-75` (`CLIENT_STATE_JOINING` is "approved, not ready for agents yet"), `draft/events.proto:34-35` (facts are sent "when it starts and when one changes"), `draft/commands.proto:24` (`HarnessesFound` is sent after a scan command).
- Failure scenario: the Server team builds "online" as "approved and has a stream". The Client team sends `HarnessesFound` only after a `ScanHarnesses` command. The machine shows online with `harnesses_scanned` false, and the Server team starts sending `ScanHarnesses` on every connection, which piles up stored commands with expiry times. Check C7 needs "online, with its facts, its load and the harnesses it found".
- Fix: write in the comment of `Facts` and `HarnessesFound` that the machine sends both without being asked after every `Welcome` (this is the same rule as the second rule of finding 2). Write in the comment of `Welcome` or in the README that `CLIENT_STATE_JOINING` ends when the server holds a `Facts` and a `HarnessesFound` from the current stream's machine.

### logic2-r1 12: the tests do not hold any rule that lives in a comment, and one guardrail claim is false

- Severity: MINOR.
- Tied to: plan.
- Evidence: `describe` writes names, numbers and types only (`plan.md:143-147`). Every rule of items 4 to 6 lives in comments (`plan.md:251-261`, `plan.md:348-353`, `plan.md:405-415`). The guardrail table says that every message, field and call has a comment (`plan.md:649`), but `JoinRequest`, `JoinResponse`, `ScanHarnesses`, `Restart`, `SetState`, `Harness` and the enum `CommandOutcome` have none in `draft/link.proto` and `draft/commands.proto`.
- Failure scenario: the cheapest wrong implementation of items 4 to 6 copies the description files and writes the `.proto` files with no comments at all. Every listed test passes, and both teams are left without the rules that they build from. A change of a comment (`aborted` to `unavailable`, or deleting the sentence about the receipt) also passes every test.
- Fix: add one test, `TestEveryLinkRuleIsWritten`, that reads the descriptors' source comments (or the `.proto` files) and asserts that each message, enum, field and method of `lgtm.link.v1` has a leading comment, and that each error code of the table at `plan.md:133` and the header `Lgtm-Link-Token` occur in the comments of `link.proto`. Correct the guardrail row, or add the missing comments to the drafts.

## Answers to the attack questions

### Lens questions

1. Sequences and counters: event numbers have no rule for a lost counter (finding 2), attempts and expiry have no outcome table (finding 5), `MachineState.revision` is sound (a lower revision is ignored, an old command sent again changes nothing) but is per machine id and the contract does not say a rejoin resets it (see post-build check 5), `FeedPosition` has gaps by design and the comment says positions of two servers cannot be compared, and there is no wrap at 64 bits.
2. Failures: a crash in a `Restart` (finding 3), a crash between commit and acknowledgement (finding 9), a database that is down and a stopping server (finding 6), a duplicate `Join` (finding 8), an expired token (finding 8), clocks that disagree (finding 5), a silent connection (finding 6). Heartbeat duplicates and lost heartbeats are harmless, because they carry no state.
3. Truth tables: `Join` has a contradictory cell (finding 8). Presence has an empty cell for a replaced stream (finding 4). Command outcomes have empty cells (finding 5). `Welcome.state` against a stored `SetState` is consistent because of the revision, and an expired `SetState` is recovered by `Welcome`, which I checked.
4. Zero values: the first value of both new enumerations is `_UNSPECIFIED`, which is meaningful in no way, so that part is right. The behaviour of a receiver for those values is missing (finding 10). `Event.number` 0, `CommandResult.attempt` 0 and `last_event_number` 0 all behave safely: an event of 0 is dropped, and a result for attempt 0 is older than every real attempt.
5. Two actors on one row: the old and the new stream of a machine (finding 4), a `Join` replay against a removal (finding 8), and the owner's `SetState` against a machine's own `Facts`, which touch different fields and need nothing.
6. Risky items: I traced the mutations of items 3 to 6. Removing `position`, changing `FeedPosition.value`, changing `attempt`, removing `revision`, renumbering `server_key_sha256`, writing `lgtm_` and swapping the nonces in the example each make a listed test fail. A wrong change that the tests would not catch is any change of a comment, for example replacing `aborted` by `unavailable` in the rules (finding 12).
7. Cheapest wrong implementation: for items 4 to 6, the description files copied with no comments pass every test (finding 12). For item 2, items 7 and 8, the listed probes defeat the obvious shortcuts, and I found no cheap wrong version.
8. A different shape that would make a wrong state impossible: `Welcome` could carry the first event number the machine must use, in place of `last_event_number` alone, which would make the restart of the counter impossible (finding 2). Presence could belong to a stream generation number, which would make the late "offline" impossible (finding 4).

### Questions of the task file

a. The Server team's end. The message set is nearly complete: `Join`, `Hello`, `Challenge`, `Proof`, `Welcome`, `Command`, `CommandResult`, `Event`, `EventAck`, `Heartbeat` and `HeartbeatAck` cover each part named. No new message is missing. Rules and one field comment are missing, and each needs a contract change before the Server team can build without guessing: the outcome and expiry table (finding 5), the event numbering and acknowledgement timing (findings 2 and 9), the owner of presence (finding 4), the meaning of "used" for a token (finding 8), the text of `CreateClientTokenResponse.token` (finding 7) and the trigger for `CLIENT_STATE_JOINING` ending (finding 11). A code for a stopping server or a down database is also missing from the table of codes (finding 6).
b. The Client team's end. Also no new message is missing. The rules that the client needs and that are not written: the retry and stop table and the silence timeout (finding 6), the counter rule (finding 2), the receipt order for `Restart` and the receipt retention (findings 3 and 5), the clock correction (finding 5), the first facts after `Welcome` (finding 11), and the winning address (finding 7).
c. Numbers and the revision. No field number is reused. `position = 4` of `WatchClientsResponse` is free (the oneof uses 1 to 3, checked in `src/contracts/proto/lgtm/v1/clients.proto:85-95`), and `git apply --check` accepts `draft/existing-files.diff` at the current commit. The envelopes keep 1 to 3 for fields and start their oneofs at 10, and the two stream messages use 1 to 5, which leaves numbers free for wave 2. The first value of each new enumeration is `_UNSPECIFIED`. The generated TypeScript of the website still builds, because a message field is optional there (`position?: FeedPosition`), and the link files are not in the export map. The revision does not stay where the plan says: the new field is harmless, but the meaning of `ReconnectClient` changes in a way the revision-1 website cannot use (finding 1).

## Post-build checks

1. Restart loop. Scenario: the client receives `Restart`, and the test kills it after it sent the result and before the receipt is flushed, then after the flush. Expected: with the flush first the new process runs the `Restart` zero more times and the machine settles after one restart.
2. Replaced stream. Scenario: a machine opens stream B while the server still holds stream A, and A ends 50 milliseconds after B's `Welcome`. Expected: the machine stays online on B, and no heartbeat is needed to correct it.
3. Counter restart. Scenario: a machine with last applied event 40 starts with an empty event file. Expected: its next event has a number above 40, and the server applies it.
4. Acknowledgement after commit. Scenario: the server process is killed between the database commit of event 5 and the send of the acknowledgement, and again between the send and the commit in a build that sends early. Expected: the first leaves event 5 applied once after a resend, and the build must not have the second ordering.
5. Rejoin resets the revision. Scenario: a machine is removed, joins again with a new machine id, and the server's first `MachineState.revision` for it is 1. Expected: the client applies it, so its stored applied revision is kept per machine id.
6. Half-open connection. Scenario: a proxy between the client and the server stops passing bytes without closing. Expected: the client closes and reconnects within 30 seconds plus one retry interval, and the server shows the machine offline within 30 seconds.
7. Clock skew. Scenario: the machine's clock is five minutes ahead of the server's. Expected: a command with a one minute expiry still runs, because the machine corrects by the server time of `Welcome`.
8. Web server in front. Scenario: the stream runs through the compose stack's web server for ten minutes with only heartbeats. Expected: no cut, which issue 1633 asks and no check has shown.

## Grade

5 out of 10. The message set is sound and nothing is reused or ambiguous in its numbers. The plan still leaves the failure policy of the link to comments that do not exist yet, and one claim about the revision is wrong. Blockers: 0. Majors: 7. Minors: 5.
