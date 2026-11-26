REVISE

# Review of the plan go-s3-client-link, round 1, seat logic

- Task identifier: review-go-s3-client-link-r1-logic
- Nonce: bd8229
- Model and effort: claude-sonnet-5-5, high
- Plan reviewed: `.plans/plans/go-s3-client-link/plan.md` as it stands today, read together with `decisions.md` and `scouting.md`.
- Checked against: `src/contracts/proto/lgtm/link/v1/link.proto`, `commands.proto` and `events.proto`, `.plans/plans/go-s1-client-local/plan.md`, and the real code in `src/client/internal/enroll/` and `src/client/internal/state/`.

## Top finding

The decline path for an expired command in item 6 (`plan.md:244`) records the refusal by calling `Store.Begin` with a made-up earlier time, so a crash before `Finish` makes the next attempt answer INTERRUPTED for a command that never started.

## Findings

### logic-r1 1. MAJOR. Ties to R6 (a command runs once and its result is recorded), and to the contract's meaning of INTERRUPTED.

- **Evidence.** `plan.md:244` says that on `DecisionExpired` the client reserves a receipt "with a nonzero pre-expiry value passed to `Begin` solely to record the decline" and finishes it as expired before replying. The Sprint 1 plan (`.plans/plans/go-s1-client-local/plan.md`, the `Begin` and `DecisionInterrupted` rows near lines 406 to 426) says a `started` receipt left by an earlier process gives `DecisionInterrupted`. `src/contracts/proto/lgtm/link/v1/commands.proto` defines INTERRUPTED as "started, effect unknown".
- **Failure scenario.** A command arrives with an expiry that the client's bound says has passed. The client calls `Begin(cmd, fakePreExpiryNow)`, which writes a `started` receipt. The process dies, for example by a restart, a kill or a power loss, before `Finish`. The server resends the command, which is still expired. `Begin` now finds the `started` receipt from an earlier process and returns `DecisionInterrupted`. The client answers INTERRUPTED, and the server records that a command ran and left an unknown effect. In truth, the command never started. This is a wrong final outcome for a command such as a restart or a scan.
- **Second effect.** The made-up time defeats the store's own guard, because `Begin` returns `DecisionExpired` without a write only when it sees the real time. The plan therefore depends on a trick that the store was built to prevent.
- **Fix.** Ask the Client Local plan for a store operation that records a decline in one write, for example `Store.Decline(id, expiresAt)`, which stores a finished expired receipt directly. If that is refused, state in the plan that this crash gives INTERRUPTED, say that the contract allows it, and add a test that kills the process between `Begin` and `Finish` and asserts the stated outcome.

### logic-r1 2. MAJOR. Ties to D16 (the client retries and the machine returns by itself) and C11 (events buffered while the server is down arrive once, in order).

- **Evidence.** `plan.md:197`: "A process that has seen approval captures facts at outage detection and every 10 seconds until a fresh standing is established, across retries. A process that has never seen approval queues no reports during an outage." Nothing in items 2, 4 or 7 stores an approval marker. The Sprint 1 folder has no such file, and `identity.json` is written at join time, before any approval.
- **Failure scenario.** The machine is approved and online. The server goes down. The owner restarts the client machine, or the service policy restarts the client, while the server is still down. The new process has never seen approval, so it queues no reports during the rest of the outage. The check C11 expects at least three reports buffered during the outage, and they have no producer. The test `TestRetryReportsOnApprovalAndOutage` can pass, because it most likely uses one process that saw approval first.
- **Fix.** Persist an approval marker in the state folder when the first approved `Welcome` arrives, and read it at start. Or decide that a restarted process counts as having seen approval whenever the outbox or `identity.json` shows an earlier session. Add a test that starts a new process against an existing state folder with the server down and observes the reports.

### logic-r1 3. MAJOR. Ties to D16 ("a client retries at least every 15 seconds") and C9.

- **Evidence.** `plan.md:195` caps the wait at 15 seconds: "Wait 1, 2, 4, 8 and then at most 15 seconds". `plan.md:155` gives each handshake exchange 10 seconds, and the Hello, Challenge, Proof and Welcome exchanges follow the dial.
- **Failure scenario.** The server accepts the connection and then stalls, for example during its own restart. Each attempt spends up to 10 seconds or more on one exchange, and then the client waits 15 seconds. The gap between two attempt starts is 25 seconds or more, so the client does not retry at least every 15 seconds. `TestRetryBackoffAndSilence` checks the wait values, so it passes.
- **Fix.** Measure the 15 seconds between the starts of attempts, not between an end and the next start. Subtract the time the attempt took from the wait, or shorten the timeout of the dial and the handshake. Add a test where the fake server stalls and the starts of attempts are compared.

### logic-r1 4. MINOR. Ties to R6.

- **Evidence.** `plan.md:246` says `Finish` removes finished receipts whose stored expiry precedes the machine's wall clock by more than 24 hours, and "a fast machine clock can shorten retention". The plan discloses the clock risk. It does not say what outcome follows.
- **Failure scenario.** The machine clock runs a day ahead. A restart command finishes with DONE, and its receipt is removed on the next `Finish`. The server resends the command because the reply was lost. The receipt is gone and the expiry has passed by the server's estimate, so the client answers EXPIRED for a command that did run.
- **Fix.** State the outcome in the plan, or prune by the elapsed clock the plan already builds. At minimum, add a post-build check with a fast clock.

### logic-r1 5. MINOR. Ties to D16.

- **Evidence.** `plan.md:195` retries an authenticated `event_gap` forever at the 15-second cap, and `plan.md:168` leaves the outbox intact on a gap.
- **Failure scenario.** A server database is restored from an older copy, so its mark is higher than anything the client can honestly send, or the client's numbering does not meet the server's. Each welcome gives the same mark, each stream ends with `event_gap`, and the client loops every 15 seconds with no change and no escalation. The owner sees a machine that is never online and a log line that does not change.
- **Fix.** After a stated number of consecutive gaps with the same mark, stop with a diagnostic that names the outbox, as the plan already does for `ErrAckAhead`.

### logic-r1 6. MINOR. Ties to C11.

- **Evidence.** `plan.md:197` captures facts every 10 seconds during an outage and again on each approved reconnect.
- **Failure scenario.** A long outage appends six facts events each minute. The 10,000-event outbox of Sprint 1 fills in about 28 hours. After that, `TestReportBackpressure` behavior applies: new reports are refused and the oldest facts stay. Useful reports, such as a harness change, are displaced by equal snapshots.
- **Fix.** Replace an unsent facts event that is equal in content instead of appending, or capture only when the facts changed.

### logic-r1 7. MINOR. Ties to the plan.

- **Evidence.** `decisions.md` rows 12, 14, 15, 17, 20, 27 and 30 are still `unanswered` with an "Assume" text, and rows 25, 26, 29, 33 and 34 to 40 now say something different about the same subjects.
- **Failure scenario.** A builder or a later reviewer reads an old row, follows it, and builds the superseded behavior. The tests are written from the plan text, so a mismatch shows only at the merge.
- **Fix.** Close or strike each superseded row and name the row that replaced it.

## Answers to the lens questions

1. **Order, tie, gap, duplicate, wrap and restart.** Events keep their original numbers and the server's mark decides the committed prefix at each welcome, so a tie or a repeated lower mark is harmless, a gap ends one stream and repairs itself at the next welcome, and a mark above the ceiling stops the run. Retry waits reset after 30 seconds on a valid stream. Revisions order `SetState`. The open items are finding 3, where the interval of retries has no tie between wait and attempt time, and finding 4, where the machine clock moves the expiry of receipts. A wrap of a 64-bit number was not found to matter.
2. **Failure permits or denies, and repair.** The plan says for each failure what happens: pin mismatch, bad signature and `aborted` are refused with a retry or warning, `unknown_machine` and `link_revision_mismatch` are terminal, and store errors prevent new work. The gaps in repair are the crash in finding 1, the missing restart state in finding 2, and the endless gap loop in finding 5.
3. **Truth tables.** I built the table of decision by outcome (Replay, Running, Interrupted, Expired, Run) against the recorded result and the expiry bound. The cell "expired, no receipt" is the one that needs a record, and its implementation is finding 1. The cell "receipt removed by a fast clock" is finding 4. The table of "seen approval" by "outage" has an empty cell for a restarted process, which is finding 2.
4. **Zero value.** An empty host name is allowed. A heartbeat with absent load is allowed and a present zero is kept separate. A zero `ExpiresAt` is not defined by the plan, because `Begin` takes a nonzero value in the decline path only. I did not find a place where an unset value is read as permission. I did not find an item that says what a zero `ExpiresAt` from the server does, so I ask the planner to name it.
5. **Two actors on the same state.** One writer feeds the stream, and acknowledgements are serialized with the send bookkeeping. A command that reconnects while its scan runs is covered by `TestCommandReconnectWhileScanRuns`. The identity file and the key are protected by the state folder lock. A second process on the same key causes `aborted` and a warning. I found no unprotected pair other than the pruning clock in finding 4.
6. **Does the named mutation fail the tests.** The mutations of items 3, 4 and 6 name a test that would fail. The mutation of item 6, "answer EXPIRED without recording", is caught only for the live path. A crash between `Begin` and `Finish` in the decline path would escape, because no listed test kills the process there (finding 1). For item 7, the test of the cadence escapes finding 3.
7. **Cheapest wrong implementation that passes the listed tests.** A retry loop that sleeps up to 15 seconds after each attempt, however long the attempt took, passes the backoff test. A reporter that queues outage facts only after one approved session in the same process passes the outage test. Both are findings 2 and 3.
8. **A shape that makes the wrong state impossible.** Store the decline as one finished receipt in one write, so no `started` receipt can exist for a command that never started (finding 1). Persist approval as a marker so a restart cannot lose it (finding 2). Measure retries from the start of an attempt (finding 3).

## Answers to the questions about this plan

- **a. Joining and the proof.**
  - The client keeps its own key, written durably before the first request. It keeps the server's key digest and the pinned certificate digest from the token. It writes `identity.json` after `CreateStore`. It checks the digest of the Ed25519 key at join and checks the signature of every `Challenge` before it sends any proof, so a server with the wrong key is refused on both sides (check C10).
  - When the join reply is lost, the client retries with the same key and the same token inside 15 minutes, and the server returns its first answer. After the token expires, the repair is manual: remove the waiting record and join again with a fresh token and a fresh folder. Nothing found to change.
- **b. Reconnecting.**
  - Waits are 1, 2, 4, 8 and then 15 seconds. The interval is not bounded from one attempt start to the next (finding 3). Terminal stops are limited to verified refusals: `unknown_machine`, `link_revision_mismatch`, and a verified removal. An unverified refusal is retried, so a hostile server cannot stop the client. That part holds.
  - Events are sent once and in order after the server returns, because the welcome mark acknowledges the prefix and replay starts at mark plus one. The three buffered reports of C11 have no producer after a restart (finding 2).
- **c. Running a command.**
  - A command that arrives twice runs once. The second arrival gets `DecisionReplay` or `DecisionRunning`. A restart commits `Finish` first, then sends, then exits 75, so the relaunch replays DONE and does not loop. An expired command is answered EXPIRED without work, and its recording has the weakness in finding 1.
  - I read the plan's names against `go-s0-contract` and `go-s1-client-local` as they are today. `Welcome` fields 1 to 5, the five `CommandOutcome` values, `Store.Begin(Command{ID, ExpiresAt}, now)`, `Finish(id, result)`, `Ack`, `Read`, `Next`, `Changed`, `ErrAckAhead` and `Event{Seq, Payload}` all match. The Sprint 1 plan lists them as planned names, and the code for the store does not exist in the checkout yet, so the match is to the plan text.

## Post-build checks

1. Kill the process between the decline's `Begin` and `Finish`, restart it, resend the expired command, and observe the outcome that the plan states.
2. With the server down, start a new process on a state folder that was approved, and observe at least three ordered reports in the outbox. Then start the server and observe each report once.
3. With a server that accepts the connection and stalls, record the times at which attempts start, and observe gaps of at most 15 seconds.
4. Set the machine clock a day ahead, finish a command, resend it, and observe the outcome.
5. Send `event_gap` repeatedly with the same mark and observe that the client escalates.
6. Call `Ack(0)` on a fresh store and observe that nothing is lost.
7. Let the outage run longer than the outbox could hold at 6 facts events per minute, and observe that useful reports remain.

## Grade

6 out of 10. Blockers: 0. Majors: 3. Minors: 4.
