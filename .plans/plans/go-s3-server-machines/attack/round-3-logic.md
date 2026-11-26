REVISE

If PostgreSQL commits an admission and the server loses the reply, the patched presence rule can leave the dead connection visible indefinitely.

Reviewed the changes from `3b6e381e` to `ac88c866` through the logic and failure policy lens. The findings are by reading the plan, the contract and the available source at the review commit. The proposed sprint 3 handlers were unavailable for runtime verification.

## Findings

### logic-r3 01 — MAJOR — C8 — An unconfirmed admission can leave a dead connection visible indefinitely

**Failure.** PostgreSQL commits stream B's replacement of stream A and loses the commit reply. The store can return `OutcomeUnknown`. `Open` returns `unavailable` from `BecomeCurrent` before it reaches the cleanup path. A's cleanup is fenced out, B never registers, and `Run` only retries a `Disconnect` that was attempted and failed. An approved machine can stay joining indefinitely if it stops trying to reconnect. `ReconnectClient` can still accept a restart for that dead stream because the row counts as presence. Evidence in `.plans/plans/go-s3-server-machines/plan.md:338`, `.plans/plans/go-s3-server-machines/plan.md:339`, `.plans/plans/go-s3-server-machines/plan.md:342`, `.plans/plans/go-s3-server-machines/plan.md:369`, `.plans/plans/go-s3-server-machines/plan.md:370`, `.plans/plans/go-s3-server-machines/plan.md:398`, `.plans/plans/go-s3-server-machines/plan.md:585` and `src/server/internal/store/pool.go:124`, `src/server/internal/store/pool.go:338`.

**Fix.** Allocate the candidate stream identifier before the write and keep it available on an unknown outcome. Route that case through the same generation-fenced cleanup and retry path as a confirmed admission. Test a commit that reached PostgreSQL with its reply lost, followed by a later generation that must survive cleanup of the candidate.

### logic-r3 02 — MAJOR — C10 — Current-stream authority moves before the named admission

**Failure.** Hold B at `AfterAdmission` after `BecomeCurrent`, while A is online. A's next heartbeat gets `aborted` and the page can already show B as joining, even though B has yet to run `Confirm`, the named moment of admission to send `Welcome`. B can fail that check and send no `Welcome`. The wire gives the old stream authority until the new stream is admitted, and says its final event number is read after that moment. Separately, removing B while held after `Confirm` but before the first send produces a newly sent `Welcome` in the server test. The current contract permits a `Welcome` sent before removal and its joint check 10 forbids an unsent one at the after-admission seam. Evidence in `.plans/plans/go-s3-server-machines/plan.md:338`, `.plans/plans/go-s3-server-machines/plan.md:340`, `.plans/plans/go-s3-server-machines/plan.md:369`, `.plans/plans/go-s3-server-machines/plan.md:370`, `.plans/plans/go-s3-server-machines/plan.md:374`, `.plans/plans/go-s3-server-machines/decisions.md:71`, `src/contracts/proto/lgtm/link/v1/link.proto:100`, `src/contracts/proto/lgtm/link/v1/link.proto:129` and `.plans/plans/go-s0-contract/plan.md:766`, `.plans/plans/go-s0-contract/plan.md:772`.

**Fix.** Use one ordered transition for making a stream current, capturing the old stream's final event number and granting permission to send `Welcome`. A provisional step must leave A current. Name which side of that transition each seam observes. If permission to send is the wire boundary, change the contract and checks 10 and 16 before freeze to permit the first send after a subsequent removal. Test removal and replacement on both sides of the boundary. When a held event or result owns the machine lock, release it and test both commit orders explicitly.

### logic-r3 03 — MAJOR — C8 — A dead stream's row still authorizes actions while cleanup waits

**Failure.** An approved stream loses all traffic and the watchdog fires at second 30. If its delete fails or waits on the machine lock, both a fresh read and the watch still present the old online flags, and a scan or restart can be accepted against those flags. A retry tick does not bound recovery to a second. Each delete can take five seconds, and the plan permits the same runner to process other machines and hourly sweeps first. With a clean stop, the same cleanup path is used, so the fact that the client closed promptly does not distinguish these cases. Evidence in `.plans/plans/go-s3-server-machines/plan.md:29`, `.plans/plans/go-s3-server-machines/plan.md:339`, `.plans/plans/go-s3-server-machines/plan.md:342`, `.plans/plans/go-s3-server-machines/plan.md:395`, `.plans/plans/go-s3-server-machines/plan.md:398`, `.plans/plans/go-s3-server-machines/plan.md:552`, `.plans/plans/go-s3-server-machines/plan.md:585`, `.plans/plans/go-s3-server-machines/plan.md:714`, `src/contracts/proto/lgtm/link/v1/events.proto:98` and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:294`.

**Fix.** Define what the page and action calls see from watchdog expiry until a delete commits. Use the existing stream generation to fence a process-owned indication that the stream ended. Invalidate affected watches through a defined unavailable response when the database cannot publish offline, return the same unavailable state from fresh reads, and refuse online-dependent calls until the stale generation is cleared or replaced. Budget cleanup and feed delivery explicitly, keep retry work independent of hourly work, and verify an open watch, a new read and an action call while cleanup is held. Test recovery without reconnect separately from replacement before the retry.

### logic-r3 04 — MAJOR — C8 — A one-command claim leaves the heartbeat and command schedules unbounded

**Failure.** The writer drains every received frame before issuing a claim. A continuously replenished input queue can prevent any claim. In the opposite direction, a heartbeat arriving just after the drain must wait for the claim's database work as well as the command send. The claim can wait on a lock and updates every overdue pending command under it, even when it will send only one. The store has a ten-second lock limit and a fifteen-second statement limit. The reader can also stop observing new heartbeats while handing off a previous frame. A client that reads one frame at a time does not bound the frames already accepted by the transport, so the new test's read order does not establish the server's order at the scheduling boundary. Evidence in `.plans/plans/go-s3-server-machines/plan.md:389`, `.plans/plans/go-s3-server-machines/plan.md:395`, `.plans/plans/go-s3-server-machines/plan.md:412`, `.plans/plans/go-s3-server-machines/plan.md:469`, `.plans/plans/go-s3-server-machines/decisions.md:73` and `src/server/internal/store/pool.go:301`, `src/server/internal/store/pool.go:313`.

**Fix.** Limit what the writer does before it checks validated heartbeats, stream end and pending output again. Give both inbound work and commands a bounded turn. Select the oldest eligible command across an overdue prefix and bound expiry cleanup independently. Define what a full handoff buffer does and prevent it from blocking the watchdog's observation silently. Test at the server's send boundary with a gate on a claim and on a send, plus a sustained inbound queue. Require a heartbeat acknowledgement and continuing command progress. Verify that the test detects a writer that drains commands first.

### logic-r3 05 — MAJOR — R6 — The grace cutoff is sampled too early to fence a send and too late to fence a result

**Failure.** A scan is claimed just before its expiry plus 24 hours. The process stalls before sending it and resumes after the cutoff. The command still leaves as the claimed attempt. On a separate run, a matching `done` arrives after the cutoff and before the next claim or hourly sweep and is accepted. If a claim ran first, the same result is discarded as the row now holds `expired`. The patch makes that precedence explicit, but the wire describes server expiry at 24 hours and forbids sends after a final result. The client can prune the receipt around that boundary. Evidence in `.plans/plans/go-s3-server-machines/plan.md:389`, `.plans/plans/go-s3-server-machines/plan.md:469`, `.plans/plans/go-s3-server-machines/plan.md:470`, `.plans/plans/go-s3-server-machines/plan.md:471`, `.plans/plans/go-s3-server-machines/plan.md:472`, `.plans/plans/go-s3-server-machines/plan.md:483` and `src/contracts/proto/lgtm/link/v1/commands.proto:3`.

**Fix.** Agree with the Contract team on the boundary used by send permission, server expiry and client receipt retention, including equality and a send delayed after permission. If the deadline fences results, expire a matching overdue pending row under the machine lock and refuse the late result. If the first persisted terminal outcome wins, state that in the contract. In either case, define the final send check, what a command already in flight can do, and how a still-pending row resumes on the next stream. Gate a claim across the cutoff and test both result and expiry orders with a controllable clock.

### logic-r3 06 — MAJOR — C11 — The revised sprint gate can miss duplicate or unapplied effects

**Failure.** At mark 40, a wrong implementation skips the valid report at 41, applies 42 and 43 and advances the mark for each. The final record can look right because the last report overwrote the visible fields. A sorted set of log numbers also produces the expected values if event 41 ran twice and produced two lines. Conversely, a process that dies after committing 41 can correctly omit its log line on replay. The plan allows that loss while asking the Stack team to compare the log numbers as a set. The Stack gate at the reviewed commit also lacks this revision as a prerequisite. Evidence in `.plans/plans/go-s3-server-machines/plan.md:110`, `.plans/plans/go-s3-server-machines/plan.md:169`, `.plans/plans/go-s3-server-machines/plan.md:495`, `.plans/plans/go-s3-server-machines/plan.md:715`, `.foundry/go-rewrite/requests-checks-sh.md:14`, `.plans/plans/go-s3-stack-client/plan.md:319` and `deploy/test/checks.sh:541`, `deploy/test/checks.sh:577`, `deploy/test/checks.sh:584`.

**Fix.** Require the Stack gate and the server rollout to depend on the same assigned revision. Drive distinct effects at each event number and observe the committed mark and effect after each released commit, including a replay. Use the database and the gated test as the evidence for order and once-only application. Treat log order and a missing post-commit log as diagnostic information. If the script compares log numbers in a run with no loss, sort without dropping duplicate values.

### logic-r3 07 — MAJOR — C9 — Shutdown can lose its terminal cause inside a write or send

**Failure.** Shutdown cancels a stream while it is inside `State.Heartbeat` and the response is still writable. The store returns `context.Canceled`. The general mapping gives `canceled`, while the shutdown rule requires `unavailable`. The loop only reads the stream's cause in its context select arm. The same ambiguity exists when the watchdog cancels a send or a write and its error returns first. Running cleanup or sweeps before canceling the service context can also spend the application's drain budget. The named shutdown test holds admission seams or a connected stream without requiring a gated store call. Evidence in `.plans/plans/go-s3-server-machines/plan.md:128`, `.plans/plans/go-s3-server-machines/plan.md:320`, `.plans/plans/go-s3-server-machines/plan.md:363`, `.plans/plans/go-s3-server-machines/plan.md:389`, `.plans/plans/go-s3-server-machines/plan.md:393`, `.plans/plans/go-s3-server-machines/plan.md:397`, `.plans/plans/go-s3-server-machines/plan.md:398`, `.plans/plans/go-s3-server-machines/plan.md:415`, `.plans/plans/go-s3-server-machines/plan.md:594` and `src/server/internal/store/pool.go:99`, `src/server/internal/store/pool.go:307`, `src/server/internal/store/pool.go:332`.

**Fix.** Propagate shutdown to the service context as soon as shutdown starts. Send every exit from the handshake and the loop through one terminal decision that preserves an applicable stream cancellation cause before translating operation errors. Distinguish request cancellation, watchdog expiry and a transport reset. Test a held store call and a blocked send with shutdown and with watchdog expiry. Require the specified code when a response is writable and a bounded reset when it is blocked.

### logic-r3 08 — MAJOR — R10 — The rollout still assigns conflicting work at the frozen test gate

**Failure.** Rollout step 3 puts items 6 and 7 in parallel, and both extend the same `TestWritesOfOneMachineFormOneLine` table owned by item 5. If each builder extends its cases there, they edit the same frozen test file at the same time. The fold says item 4 no longer runs beside item 11, but item 4 still says it does. A dispatch that follows that sentence can start item 11 before item 6 has delivered a `State` that satisfies `machines.Link`. Evidence in `.plans/plans/go-s3-server-machines/plan.md:105`, `.plans/plans/go-s3-server-machines/plan.md:126`, `.plans/plans/go-s3-server-machines/plan.md:139`, `.plans/plans/go-s3-server-machines/plan.md:142`, `.plans/plans/go-s3-server-machines/plan.md:262`, `.plans/plans/go-s3-server-machines/plan.md:350`, `.plans/plans/go-s3-server-machines/plan.md:452`, `.plans/plans/go-s3-server-machines/plan.md:488`, `.plans/plans/go-s3-server-machines/plan.md:549`, `.foundry/go-rewrite/results/fold-go-s3-server-machines-r2.md:35` and `.plans/epics/go-rewrite/epic.md:69`.

**Fix.** Let item 5 own a reusable test function. Have each later item add its own test in its own file and name that test in its acceptance command and mutation. Remove item 11 from item 4's parallel list. In the public names table, add item 11's `Service.WatchClients` and identify its callers in the router and the tests of items 12 and 14.

### logic-r3 09 — MINOR — R6 — The fast-clock check omits the receipt-pruning write

**Failure.** P5 shifts the machine clock by five minutes and drops a scan result. At expiry plus 23 hours and 58 minutes by the server's clock, the receipt still replays `done` if no intervening operation pruned it. The client store reads an existing finished receipt before checking expiry and prunes on a receipt write. Fast time alone cannot produce the expected `expired`. The normal wire guarantee and the accepted fast-clock limitation also still differ. Evidence in `.plans/plans/go-s3-server-machines/plan.md:33`, `.plans/plans/go-s3-server-machines/plan.md:34`, `.plans/plans/go-s3-server-machines/plan.md:716`, `src/client/internal/link/receipts.go:78`, `src/client/internal/link/receipts.go:98`, `src/client/internal/link/receipts.go:137`, `.plans/plans/go-s3-client-link/plan.md:241`, `.plans/plans/go-s3-client-link/plan.md:245`, `.plans/plans/go-s3-client-link/plan.md:248` and `src/contracts/proto/lgtm/link/v1/commands.proto:12`.

**Fix.** Run one offset case with the saved receipt intact and another after a documented pruning write, and assert both receipt states before replay. In the second case expect `expired` and no new effect. State the accepted clock limit in all three plans and the contract. For the repeat case, name both required conditions, successful pruning before server expiry and a retry that still meets the start bound.

### logic-r3 10 — MINOR — R8 — The keyed fallback overlaps page error mapping

**Failure.** `ApproveClient` runs through `WriteOnce`. A duplicate approved name returns `ErrNameTaken` and is specified as `already_exists`. The keyed fallback also covers that error with `unavailable`. Similarly, a lock timeout in a keyed scan matches the page's retryable `aborted` rule and the keyed fallback's `unavailable` rule. The store leaves a domain error intact and marks a rolled-back lock timeout as retryable, so these inputs reach the overlap. Evidence in `.plans/plans/go-s3-server-machines/plan.md:128`, `.plans/plans/go-s3-server-machines/plan.md:177`, `.plans/plans/go-s3-server-machines/plan.md:183`, `.plans/plans/go-s3-server-machines/plan.md:582`, `.plans/plans/go-s3-server-machines/plan.md:586` and `src/server/internal/store/pool.go:63`, `src/server/internal/store/pool.go:99`.

**Fix.** Give the handler one precedence order for known domain errors, receipt errors, operation cancellation, retryable page store errors and the final fallback. Test a taken name in a keyed approval, retryable database contention in a keyed page call and the same contention in `Join` or `Open`.

## Evidence and risk

Every finding below is confirmed by reading for the described plan behavior. Runtime results for the proposed implementation remain to be checked.

| Finding | Category | Severity, fallout and probability | Score | Responsible builder |
|---|---|---|---|---|
| 01 | Bug | 6 for a connection state that can persist until repair. 3 for one machine that can reconnect. 4 for a lost commit reply. | 72 | Server Link |
| 03 | Bug | 4 for temporary false presence. 3 for one machine and its page. 4 for cleanup contention or an outage. | 48 | Server Link and Machines |
| 04 | Bug | 4 for interrupted command or heartbeat progress. 3 for one recoverable stream. 4 for a backlog or slow dependency. | 48 | Server Link |
| 07 | Bug | 4 for an interrupted shutdown flow. 3 for a reconnecting machine. 4 for shutdown during database or transport work. | 48 | Server Link and application |
| 05 | Design | 4 for a wrong recorded outcome. 5 for a result the owner may need to inspect. 2 for a retry around the end of the grace window. | 40 | Server Link and Contract |
| 02 | Design | 4 for a prematurely replaced stream or an unexpected first reply. 3 for one machine that can reconnect. 3 for a narrow admission race. | 36 | Server Link and Contract |
| 10 | Design | 2 for a wrong code and retry instruction. 3 for the affected owner. 6 for an ordinary naming conflict if the fallback wins. | 36 | Server Machines |
| 09 | Design | 1 for the incomplete clock fixture. 1 for a test-only failure. 2 for the injected clock offset. | 2 | Server Link, Client Link and Contract |
| 06 | Design | The required sprint check does not prove its claim. | Gate | Stack test writer and orchestrator |
| 08 | Design | The required item gate has inconsistent ownership and prerequisites. | Gate | Server lead |

## Answers to the assignment questions

### Assignment 1. What the round 2 patches close

| Register group | Result |
|---|---|
| A | The stale results now agree with their methods, the apply mutation moved to item 7, item 10 no longer needs a real watch, and item 5 only deletes its own tables. Ownership of the shared lock test and item 4's dispatch sentence still conflict. See 08. |
| B | The added seam reaches the gap and the normal path now specifies cleanup after the admission. That cleanup misses an unconfirmed commit, and two different transitions claim current-stream authority. See 01 and 02. |
| C | Fresh reads and watches now agree about the row while a delete fails. They can agree on a dead connection, and the retry interval alone does not bound publication. See 01 and 03. |
| D | The new qualification describes fast-clock pruning. P5 is missing the write that produces it, and the contracted replay guarantee still uses a different condition. See 09. |
| E | Stopping or leaving the callback's deadline in place addresses the previous clear-after-cancel race. A send reset can carry no terminal status. Cancellation during a store call or send still needs precedence across exit paths. See 07. |
| F | A claim now returns one command, and even a long expired prefix leaves a later eligible command selectable. The database work itself and the two directions of the writer are still unbounded by that selection count. See 04. |
| G | Rollout names a prerequisite for the revised Stack check. Its proposed evidence and the peer gate still leave the prior proof gap. See 06. |
| H | The service context reaches streams before registration. The selected error path can lose the shutdown cause. See 07. |
| I | `LinkOptions` reaches the generated handler, and production and fixture paths are named for `app.Hooks`. The oversized `Join` and restart tests cover the stated paths. Check both seams after restart and preserve the sprint 2 startup phase-budget tests. |
| J | Removing `facts_at` removes the unused write. The current contract still promises to show `happened_at`. The result file assigns that text change to the Contract team before freeze. |
| K | The timestamp can no longer stall the queue. The new tests cover both absent and out-of-range timestamps. Nothing found in the next-number rule from this removal. |
| L | Page and link store errors now have separate rules, and `Confirm` has a database error branch. The keyed fallback and context-cause overlaps remain. See 07 and 10. |
| M | The query tracer can stop the commit after the feed flush. PostgreSQL's driver uses a lowercase `commit` command through the traced execution path. The new gate can prove definite rollback, and 01 covers the distinct unknown outcome. Evidence in `src/server/internal/store/pool.go:338` and `src/server/internal/store/feed.go:133`. |
| N | Dropping both ownership predicates now admits the second machine's otherwise matching answer in the test. The mutation reaches the original flaw. |
| O | The constraint and both terminal writes pair `outcome` with `finished_at`. The pending-row precedence is explicit, and it still needs reconciliation with wire expiry. See 05. |
| P | Copying declared fields closes the unknown-field path for both report shapes. Extend the marker test to the top-level `HarnessesFound` message as well as each entry. |
| Q | One claim per send removes queueing behind a batch. A pause between claim and transport still permits a send after the grace cutoff. See 05. |
| R | The tests distinguish the sentences for a scan and a restart. Their behavior on a stale connection row follows the presence gap in 03. |

The cleartext edit records the settled transport choice. The traceability and attack-log edits add no runtime rule. The event-number ceiling still needs the assigned Contract decision for the next number after the largest signed mark.

### Assignment 2. The two new mechanisms

The row-only presence mechanism closes inconsistent reads and opens the unknown-admission and delayed-cleanup cases in 01 and 03. The one-command mechanism removes batch delay and leaves the scheduling and cutoff cases in 04 and 05. Groups F and Q mark the same command mechanism.

### Assignment 3. Rejected findings

Nothing found. There are no rejected outcomes in the round 2 register at `.plans/plans/go-s3-server-machines/attack/round-2-findings.md:9`.

### Assignment 4. Writers and readers of changed names

Item 6 owns `Claim` and its renamed return shape, and item 10 reads its single command. `Enqueue` is the remaining `machine_outbox` writer, and `Hub.Follow` is its reader. Item 8 owns the service context and the private `afterConfirm` hook, item 10's `Run` cancels that context, and item 14 wires the cancellation into the application. Item 14 owns `LinkOptions`, `app.Hooks` and `servertest.LinkSeams`. The command, fixture start and fixture restart are listed. The prior sign-in plan's private `start` still needs to receive the hooks from `app.Start`, or the public wrapper must wire them before readiness. `facts_at`, `CommandBatch` and `Hub.CloseAll` have no live caller named in the patch. `happened_at`, the contract's admission seam semantics and the Stack script's new durable mark reader are cross-team changes still to reconcile. The missing public `WatchClients` entry and lock-test ownership are in 08.

### Assignment 5. Truth tables for interacting rules

| State or order | Patched outcome | Conflict or empty cell |
|---|---|---|
| Stop or watchdog fires, delete commits, no replacement | Offline after the commit and watch delivery. | The 30-second statement has no allowance for those final steps after a watchdog at second 30. |
| Stop or watchdog fires, delete waits or fails, no replacement | An online row stays online in both reads, and a scan or restart can be accepted. | The degraded page and action behavior conflicts with the dead stream. |
| Stale cleanup runs after replacement | The old identifier deletes nothing, and the new row is retained. | This cell has a fence. |
| Admission rolls back, or commits but the reply is lost | Both can end before the cleanup path. | Definite rollback needs no delete. The committed case has no owner for cleanup. |
| A is live, B's `BecomeCurrent` commits, B has not run `Confirm` | A loses write authority and the row points to B. | The contract says A loses authority at the admission to send. |
| Removal or replacement commits before `Confirm` | B gives `unknown_machine` or `aborted` and sends no `Welcome`. | This cell has a check. |
| Removal commits after `Confirm` but before `Send` | The server test requires `Welcome` followed by the refusal. | The current wire and joint check distinguish an unsent `Welcome` from a sent one. |
| Removal commits after a writable first send | The buffered `Welcome` can arrive before the refusal. | This cell agrees with the in-transit case. |
| A current, matching result reaches a pending row at or before the cutoff | It closes the row, and equality does not expire it. | This cell has a rule. |
| Matching result after the cutoff reaches a pending row first | It closes the row with the reported outcome. | The server's wire expiry boundary and receipt policy differ. |
| Expiry wins the machine lock first, or the result is stale, foreign or repeated | An expired or otherwise final row is immutable, and other nonmatching results are ignored. | This cell has a fence. |
| A non-`SetState` claim is made before the cutoff and its send starts after it | The returned attempt can still be sent. | The send permission's lifetime is undefined. |
| A pending `SetState` crosses the cutoff | It stays pending and is retried. An expired result is stored as failed. | This cell has a separate rule. |
| Shutdown or watchdog cause is set while an operation returns its own cancellation error | A context branch has the prescribed cause, and an operation branch can return `canceled`. | Error precedence is empty outside the context branch. |
| A keyed page call gets a domain refusal or a retryable store error | The domain or page mapping and the keyed fallback both apply. | The precedence is empty. |

### Assignment 6. Round 2 blockers and majors still open

Group A's original missing-name build blockers are addressed individually. The dispatch and ownership instructions still conflict. Groups B, C, F, G, H and Q retain material parts of the round 2 failure through the changed mechanisms or their joint proofs. See 01 to 08. Group D describes the limitation but its new failure rehearsal is incomplete. Group M reaches the commit it claimed to test.

### Assignment 7. Lens questions touched by the patches

The patches touch all eight questions. The answers below cover counters and cutoffs, failure and recovery, rule combinations, empty values, concurrent actors, mutations, test gaps and constraints.

## Answers to the logic lens

1. **Ordering and restart.** The machine lock orders generations and per-machine enqueue `seq`, so clock ties and identity gaps do not affect claim order. The event mark rejects zero and gaps, acknowledges duplicates, and survives a restart. The signed event ceiling needs the contract action already requested. Attempts start at zero and increase on a new stream, and `SetState` has no expiry transition. The proposed signed database counters fail on exhaustion without a specified recovery. The patches define cutoff equality for storage and leave the send and result boundary in 05.
2. **Failure and repair.** Explicit rollback sends no acknowledgement and retries on a later stream. A normal failed delete retries by stream identifier, and startup clears dead process rows. Unknown admission commit, delayed cleanup and cancellation during an operation still leave the cases in 01, 03 and 07.
3. **Interacting rules.** The assignment truth table names the contradictory and empty cells for presence, admission, grace expiry, result precedence and error mapping.
4. **Empty and zero values.** A command starts with attempt zero in storage and zero is invalid on a result. Unknown and unspecified outcomes map to failed. A null terminal time requires a null outcome. An absent or out-of-range report timestamp no longer affects processing. Zero load is distinct from absent load, zero watch position remains present, and an empty handler-options slice retains the generated defaults. Nothing found in those cases.
5. **Concurrent actors.** A and B, a removal, `AcceptResult`, `Claim`, the expiry sweep and delayed `Disconnect` serialize under the machine lock. The hub and the stream writer have separate authority over termination and sending. Their gap at first send remains in 02, and cancellation during other work remains in 07.
6. **Risky mutations.** The named mutations and one way to evade the specified tests appear by item below. The strongest new commit assertion is the gate after the feed flush. The heartbeat test needs a bound on the observation point to prove a failed mutation.
7. **Cheapest wrong implementation.** The final column below names a wrong implementation compatible with each item's listed assertions. The examples for items 3 and 4 concern the interfaces and dependencies affected by the patch.
8. **Constraints and ownership.** The paired outcome and terminal-time constraint rules out the old retention leak. Make the admission transition a single authority point, give one owner the candidate identifier through every cleanup outcome, and give each lock-test file one writer. Those changes remove reachable wrong states in 01, 02 and 08.

| Item | Named mutation and its signal | Wrong implementation the listed checks leave open |
|---|---|---|
| 1 | A second `Lock` distinguishes `FOR SHARE` from `FOR NO KEY UPDATE`. Concurrent approval detects deleting an approved row. | Sanitize nested harness entries while retaining an unknown field on the outer harness report. |
| 2 | The item has no required mutation. Its cancel and ordinary deadline tests exercise different exit paths. | Clear a deadline after the cancellation callback starts in the narrow race between a successful send and deadline clearing. |
| 3 | The verifier and lifetime assertions detect storing the secret or using 16 minutes. | Omit the token feed reference on revoke while preserving the asserted row values. |
| 4 | Expired replay, the digest source check and both metadata transports target the three mutations. | Return a permanent error for an unconfirmed redemption commit. The rollback fixture fails the insert, and the replay fixture starts with a known successful commit. |
| 5 | Stale heartbeat and both lock orders detect a missing stream predicate or lock. | Lose the candidate identifier on an unconfirmed admission commit. |
| 6 | Wrong attempts, wrong machine or stream, held commit and `SetState` retention detect their respective mutations. | Start sending an already returned claim after the cutoff. No gated test crosses the cutoff between claim and send. |
| 7 | Event zero, repeat and gap rules detect validating the payload too early. The held commit and lock cases detect separation of effect and mark and loss of the machine lock. | Accept duplicate harness names. The listed invalid-report cases use facts. |
| 8 | The proof fixture, refusal priority, fresh nonce and pre-confirm removal exercise the named mutations. | Invalidate A before B is allowed to send `Welcome`. No listed check writes on A while B is held in that gap and asserts A remains current. |
| 9 | Registering a lower generation detects unconditional handle replacement. | Wake handles after resubscription while omitting the initial wake for a handle already present at first subscription. |
| 10 | The held commit detects an early acknowledgement. Backpressure detects a missing send deadline. | Starve commands under a sustained inbound queue or lose shutdown's cause during a store call. |
| 11 | Missing positions and unconditional authorization each break their named assertions. | Keep delivering a stale online projection during failed disconnect cleanup. No item 11 test holds that cleanup. |
| 12 | The keyed token case and the approved-machine revocation race detect both named mutations. | Bypass `WriteOnce` for revoke. The replay assertion concerns remove. |
| 13 | Failed enqueue, a scan in joining state and approval replay detect the named transaction or eligibility mutations. | Give `unavailable` to a retryable store error on a keyed scan. |
| 14 | Swapping handlers breaks the routing test. The added oversized `Join` catches a dropped limit for that call. | Apply a read limit to `Join` alone. The `Open` path is exercised with a well-formed `Hello`. |

## Post-build checks

1. **Commit gate targeting.** Tag the operation under test. Hold its actual commit after the feed flush, observe that nothing leaves, then release a successful commit and observe one result. In another run reject the held commit and observe the unchanged report, mark, command attempt and feed. This distinguishes proper gating from a fixture that catches a different transaction.
2. **Cancellation at the transport boundary.** Release a successful or blocked send as cancellation begins, and make one later write on a separate live stream on the same Hypertext Transfer Protocol version 2 (HTTP/2) connection. The cancelled stream ends by the selected status or a bounded reset, and the live stream's write succeeds.
3. **Router and fixture propagation.** Send an oversized frame to `Open` and an oversized request to `Join` through the real router. The oversized messages are rejected within the agreed wire limit and change no records. Reuse the fixture after restart and verify that both configured admission hooks reach the restarted service.
4. **Declared report fields.** Send a valid harness report with distinct markers in unknown outer and nested fields. Verify that the mark advances and the known entries project correctly, with neither marker in the saved data, emitted records or logs.

## Grade

5/10. There are 0 blockers, 8 majors and 2 minors.
