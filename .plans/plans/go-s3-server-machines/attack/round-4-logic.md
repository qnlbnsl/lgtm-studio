REVISE

The skeleton leaves the link service constructor without a builder and changes the application start signature before its test fixture is scheduled to adopt it.

I reviewed the changes from `ac88c866` to `8533e992` and their consequences, and checked the referenced source at checkout `b55b9b71`. The findings are by reading. The server that implements this plan was unavailable for runtime verification. Citations to `plan.md` and `decisions.md` refer to this plan's directory.

## Findings

### logic-r4 01 — BLOCKER — R10 — The skeleton and the next items cannot meet their own gates

Item 0 changes `app.Start` to take four arguments and explicitly migrates the command caller. Item 14 owns forwarding the hooks from the existing `servertest.Start` and `Restart`. The sprint 2 fixture calls the three-argument entry point, and its tests are required to pass at item 0. That leaves either a compile error at item 0 or an unassigned edit to the existing fixture. Separately, item 8 needs `link.NewService` to create the service lifetime context. Item 4's target table names that constructor, but its actual file assignment omits `link/service.go`. Item 0 only completes constructors that keep their arguments, and item 8 cannot edit that file. The handshake test can therefore compile against a constructor that returns zero and then fail at setup. The literal signature `NewState(s, r, h *Hub)` also types all three arguments as a hub, although item 14 builds a store, registry and hub for it. The sentinel tests require real, distinct domain errors at the same time that the skeleton's universal red-test rule requires those tests to fail by assertion before their declarations exist. Evidence in `plan.md:122`, `plan.md:130`, `plan.md:132` to `plan.md:136`, `plan.md:205`, `plan.md:251`, `plan.md:347`, `plan.md:350`, `plan.md:579`, `plan.md:580`, `.plans/plans/go-s2-server-signin/plan.md:475`, `.plans/plans/go-s2-server-signin/plan.md:487`, `.plans/plans/go-s2-server-signin/plan.md:545` and `src/server/internal/store/pool.go:75`.

Keep the skeleton, and give it an explicit compatibility pass over every existing `app.Start` caller, including both fixture paths, with empty hooks and the original bodies intact. Give the later builder responsibility for forwarding real hooks. Write typed constructor signatures and make each required constructor and migration accessor usable before the first gate that calls it. Name the owner of `link/service.go` and the service lifetime initialization, and make the handshake depend on it. Specify a compiling red state for the sentinel tests or declare them an interface smoke check with its own gate. Run both the old suite and the dependency-only item suites at their named commits.

### logic-r4 02 — MAJOR — C10 — A superseded admission has incompatible first-frame and registration rules

Hold stream A after its admission commits. Admit B and keep its higher-generation handle registered. Releasing A must give `aborted` before the send under `Register`, but the patched `TestNewStreamReplacesOld` requires A to send `Welcome` and get `aborted` on its first heartbeat. If B unregisters before A resumes, the same map has no higher generation to reject A, so the declared algorithm can install a handle for a stream the database has already fenced out. The handshake-only test for this case merely requires that A eventually end and that B's stored generation remain. Removal has a related gap. Once a removal was consumed while A was absent from the hub, an unknown frame is ignored and a valid command result on the removed stream can return `false`, so neither is guaranteed to trigger the promised refusal of the next frame. Evidence in `plan.md:327`, `plan.md:357`, `plan.md:358`, `plan.md:360`, `plan.md:368`, `plan.md:376`, `plan.md:393`, `plan.md:394`, `plan.md:455`, `decisions.md:85` and `src/contracts/proto/lgtm/link/v1/link.proto:100` to `src/contracts/proto/lgtm/link/v1/link.proto:115`.

Preserve the one admission commit. Specify the first frame and terminal status separately from permission to install a live handle. Use the database generation or a generation fence that survives unregistration to prevent A from taking over the hub after B has left. If the selected wire rule sends an authorized `Welcome` on a displaced stream, use a path that sends that frame alone and then emits the displacement cause before dispatching any frame. Freeze the same policy in the contract and test it with B still registered and with B already disconnected. In the removal cases test a first frame other than a heartbeat and verify the terminal reason and absence of writes.

### logic-r4 03 — MAJOR — C8 — A canceled handle can still authorize commands during cleanup

The hub cancels a stream but `Holds` tests whether its handle is registered. A watchdog can cancel an approved, online stream before the handler enters cleanup. A page call can then lock the machine while the handler is returning or before `Disconnect` takes that lock. The old row and registered handle satisfy the patched scan and restart checks, so a command is stored against an ended stream even with a healthy database. If the handler calls `Disconnect` before `Unregister`, the same window includes a cleanup stalled before acquiring the machine lock. The patched test exercises a row left after the handler and its cleanup have returned. Evidence in `plan.md:328`, `plan.md:330`, `plan.md:358`, `plan.md:368`, `plan.md:380` to `plan.md:383`, `plan.md:570`, `plan.md:571` and `decisions.md:86`.

Define the action's liveness decision at the read of the live handle for the database's exact stream identifier while the action owns the machine lock. Make a terminal handle ineligible at cancellation, independently of the delete and physical unregistration. State that a command admitted before that point remains stored for the normal retry rules if the stream then ends. Gate a terminal handler before its cleanup begins and assert that a fresh scan and restart refuse the retained row without waiting for the delete. Keep the degraded display allowed by decision 79.

### logic-r4 04 — MAJOR — C8 — A one-command turn can still spend its time on an unbounded prefix

Removing the bulk expiry update shrinks the write set. The sole pending index shown is ordered by machine and sequence. With a prefix of pending rows that already went to this stream or that are now outside the send window, a claim can walk past every row in the prefix under the machine lock before finding the first eligible command. The sent prefix can grow on every turn when replies have not arrived. This makes a one-command result a poor bound on time spent before a queued heartbeat and can consume the store's statement timeout. The test drives periodic traffic with 64 aged commands but does not ensure input is continuously ready or bound how much a claim can examine. A writer that drains input greedily can pass that schedule when its queue empties between arrivals. Evidence in `plan.md:375` to `plan.md:380`, `plan.md:397`, `plan.md:450`, `plan.md:454` and `src/server/internal/store/pool.go:301` to `src/server/internal/store/pool.go:318`.

Choose an eligibility lookup or a bounded, stream-scoped scan with a continuation so each turn caps work even when many rows are filtered. Define continuation, wake and cancellation behavior when a bounded scan finds no eligible row yet. Keep expiry maintenance separate and bound its lock hold. Gate the server's writer with both input and a command ready, then assert the turn order at the send boundary. Require the test to reject both a writer that drains input and one that starts another command before servicing a ready heartbeat.

### logic-r4 05 — MAJOR — R6 — A reserved command can still be sent after its final outcome commits

Claim a scan just before expiry plus 23 hours and pause the writer briefly after the claim commits. Step the database clock past the 24-hour boundary and let the sweep commit `expired` while the stream is still live. The writer has the command in memory and no pre-send permission check, so it can send the already finalized attempt. The one-hour margin is a selection rule and does not order a send against the sweep. The current contract still promises pending retries on each new stream until finalization and contains no 23-hour send cutoff. Evidence in `plan.md:34`, `plan.md:375`, `plan.md:454` to `plan.md:457`, `plan.md:465`, `plan.md:466`, `decisions.md:90` and `src/contracts/proto/lgtm/link/v1/commands.proto:3` to `src/contracts/proto/lgtm/link/v1/commands.proto:18`.

Define a send-authorization boundary under the machine lock, ordered against a current-stream change and finalization, with an explicit deadline on the authorized send. If an already authorized frame may still arrive after finalization, publish that exception and its treatment with the 23-hour cutoff and first-stored-outcome rule in the contract. Recheck eligibility at the writer boundary using the chosen clock and drop a stale claim without sending it. Keep network waits outside the transaction. Freeze a test that gates the writer after claim, crosses the cutoff, finalizes the row, releases the writer and observes no newly authorized frame.

### logic-r4 06 — MAJOR — C11 — The revised gate can still pass a replay that writes twice

The combined check now distinguishes successive effects, so it can catch a dropped event overwritten by a later one. A faulty server can still run the setter again on a duplicate of event 41 with the same payload, leave the mark at 41 and produce the same observed record. The proposed check rereads the effect and mark, so both observations match. The check also needs to cut a real connection after the event commit and before its acknowledgement, but the public fixture defines only handshake seams. The fold makes the script revision an external request. The current script still treats ordered log lines as the deciding comparison and its sprint gate still names the log interface. Evidence in `plan.md:110`, `plan.md:149` to `plan.md:153`, `plan.md:478`, `plan.md:481`, `plan.md:702`, `src/server/internal/store/feed.go:102`, `deploy/test/checks.sh:577`, `deploy/test/checks.sh:584`, `.plans/plans/go-s3-stack-client/plan.md:318` and `.foundry/go-rewrite/results/fold-go-s3-server-machines-r3.md:74`.

Freeze a transaction-observable assertion that the replay adds no effect write, for example a fixture audit counter for committed report updates and the corresponding feed position. Retain the distinct per-event assertions. Name the real in-process fixture mechanism and its owner for holding a commit and cutting the wire between that commit and the acknowledgement. Put the revised Stack check and the combined test in the same sprint dependency list. Require the Stack check to use the stored mark and drained client outbox, and keep server logs for diagnosis.

### logic-r4 07 — MAJOR — C9 — A held cleanup can delay delivery of the shutdown cause

The first-cause rule works once a terminal signal has reached the stream. The new rule puts repeat cleanup and hourly maintenance in `Run`, and also gives `Run` the service cancellation step. If a retry is already inside `Disconnect` with its independent five-second context, merely observing the process context on the next iteration can use the entire application drain window before any other stream receives `unavailable`. That implementation can pass the patched test that holds a stream's claim or heartbeat, because neither holds the maintenance runner. Evidence in `plan.md:328`, `plan.md:350`, `plan.md:351`, `plan.md:383`, `plan.md:400`, `plan.md:579` and `src/server/internal/store/pool.go:325` to `src/server/internal/store/pool.go:329`.

Make shutdown set the service cause independently of the retry and sweep execution, as soon as draining starts. Give runners separate cancellation and shutdown deadlines, and leave any still-fenced cleanup to start-up. Hold a cleanup retry before shutting down with an unrelated idle stream still attached. Require the idle stream to report `unavailable` while the retry is held and require the drain to stay within budget.

### logic-r4 08 — MAJOR — R5 — The blocked-watch gate expects a faster revocation signal than the shared watch supplies

Open a watch just after its authorization check, block one send, revoke the session immediately and write one machine change. The blocked sender cannot get to the next message check. The shared watch specifies a periodic authorization check every 30 seconds while blocked. A single later change is insufficient to force lag, and the transport's ordinary deadline is 10 seconds. With those two deadlines, the sender can correctly reset at 10 seconds without ever seeing the revocation. The new sprint check expects the context to stop it sooner, but does not control when the blocked-watch recheck runs. Evidence in `plan.md:309`, `plan.md:538`, `plan.md:548`, `plan.md:707`, `.plans/plans/go-s1-server-store/plan.md:613`, `.plans/plans/go-s1-server-store/plan.md:615`, `.plans/plans/go-s1-server-store/plan.md:629` and `src/server/internal/watch/watch.go:38` to `src/server/internal/watch/watch.go:59`.

Keep the requirement that a blocked send ends before its write deadline when the shared watch detects revocation first. Give P11 a controlled way to trigger that recheck after revocation, or stage the send near a known scheduled recheck and allow two seconds for authorization. Add a separate case where the ordinary deadline wins, with no post-revocation message in either case. If the product needs earlier cancellation for revocation at any time, assign a revocation signal to the shared Watch owner before freezing the gate.

## Evidence and risk

The evidence status below refers to the specified plan and gate, as read at the reviewed commits. The scores estimate runtime consequences. Required gate failures carry no score.

| Finding | Category | Evidence status and owner | Severity, fallout and probability | Score and band |
|---|---|---|---|---|
| 01 | Design | Confirmed by reading. Server lead. | The first required build and test gate is internally inconsistent. | Required gate |
| 06 | Design | Confirmed by reading. Stack test writer and orchestrator. | The required sprint evidence can miss repeated durable work. | Required gate |
| 08 | Design | Confirmed by reading. Shared Watch and Server Machines. | The required check can fail against the specified watch and transport. | Required gate |
| 03 | Bug | Confirmed by reading. Server Link and Machines. | 4 for a command queued against an ended stream, 3 for one machine, and 6 for an action during ordinary disconnect. | 72, schedule |
| 02 | Design | Confirmed by reading. Server Link and Contract. | 4 for failed or displaced reconnect, 3 for one machine, and 4 for an overlapping admission. | 48, record |
| 04 | Bug | Confirmed by reading. Server Link. | 4 for interrupted command and heartbeat progress, 3 for one machine, and 4 for a large backlog or slow lookup. | 48, record |
| 07 | Bug | Confirmed by reading. Server Link and application. | 4 for delayed shutdown, 3 for machines that reconnect, and 4 for shutdown during a held retry. | 48, record |
| 05 | Design | Confirmed by reading. Server Link and Contract. | 4 for a finalized command sent again, 3 for one machine, and 2 for a stall across the long deadline. | 24, record |

## Answers to the assignment questions

### Assignment 1. What each patch group closes

| Group | Effect of the patch |
|---|---|
| A | The skeleton removes the missing-sentinel dependency between items 6 and 7 and moves the shared helper into item 5. Constructor lifecycle and the changed fixture caller still break isolated gates. The general locking rule names an `AcceptResult` race case, while the item 5 table and item 6 tests name only `Claim` for that shared helper. See 01 and lens answers 6 and 7. |
| B | One commit now fences the old stream and captures its final mark. Registration and the new first-frame expectations disagree for an already displaced stream. See 02. |
| C | The accepted stale display is explicit and a row with no registered handle denies actions. A canceled registered handle still passes the new check. See 03. |
| D | The candidate identifier exists before the write and the retry can cover a lost commit reply, including a later replacement. The test for a failed delete does not simulate a lost commit reply. See the post-build check for an unknown admission outcome. |
| E | The turn rule removes the explicit unbounded drain of incoming messages and states what happens to a full buffer. The claim and the scheduling test can still hide starvation. See 04. |
| F | Successive, distinct observations catch overwritten or omitted events. A same-value replay can escape, and both sprint gates need the same revision. See 06. |
| G | Claim stops bulk expiry and separates selection from the first stored outcome. The margin leaves a delayed-send and sweep race. See 05. |
| H | P5 now covers both receipt states. With the receipt intact its result wins. A pruned receipt and expired start window produce `expired`. |
| I | Joining the started callback removes the specified after-return write, and the two stop outcomes have different deadline cleanup. See the post-build transport check. |
| J | The adapter passes the watch's context into the send. P11 needs a recheck that falls inside the blocked send's deadline. See 08. |
| K | The recorded cause now controls operation-error mapping after cancellation. An active maintenance call can still postpone the original cancellation. See 07. |
| L | The plan and fold move the encrypted check to the proxy work. No logic issue found in the settled cleartext rule. The Contract team still owns the matching change before freeze. |
| M | A next event at the signed ceiling returns a domain error before reaching PostgreSQL. A larger gap still follows the earlier gap rule in item 7, while the contract request describes all numbers above the ceiling together. |
| N | Known domain and keyed errors now have precedence and a retryable page lock timeout maps to `aborted`. The store preserves an unwrapped domain error and treats a rolled-back lock timeout as retryable. See `src/server/internal/store/pool.go:63` and `src/server/internal/store/pool.go:99`. |

2. I attacked both unreviewed mechanisms. The skeleton's lifecycle and caller ownership fail its gate in 01. The hub hold still authorizes actions after cancellation in 03. The state tables and all eight lens answers below cover their ordering, failure, defaults, actors and tests.
3. There are zero rejected findings in the register and fold counts. I checked all outcome cells and the fold's count at `.foundry/go-rewrite/results/fold-go-s3-server-machines-r3.md:15` to `.foundry/go-rewrite/results/fold-go-s3-server-machines-r3.md:25`.
4. The changed names are only partly covered. Item 0 declares `WithTimeout` for the scripted fixture and transport callers. Item 9 writes the hub check, item 5 fills `Held`, and item 13 reads it. The hub registration needs an explicit stream identifier and live-state input to support that check. Item 8 writes the admission call and uses its candidate identifier for `Run` cleanup. The exported seam is declared by item 0 and forwarded by item 14 to the handshake and combined fixture. The `app.Start` fixture callers and the owner of the link constructor are incomplete. Item 5 owns `checkOneLine`, item 7 names its caller, and item 6 lacks the `AcceptResult` subcase. Items 2 and 14 are the reader and writer of the new router option. See `plan.md:126`, `plan.md:132`, `plan.md:150`, `plan.md:307`, `plan.md:338`, `plan.md:368` and `plan.md:579`.
5. The admission, presence and command truth tables below identify the contradictory and empty cells. The transport callback and periodic authorization rules also conflict with the timing P11 expects.
6. The round 3 blocker A stays open as a build-gate problem. The majors B, C, E, F, G and K are partly open for the cases in 02 through 07. D's repair route now exists and needs the lost-reply check below. I closes the stated callback ordering by reading. L is recorded as a Contract handoff.
7. The patches touch all eight lens questions. The answers below cover the ordering and zero values, interacting gates, concurrent actors, mutations and how to rule out the new wrong states.

### Truth table for first frames after proof

| What commits or registers before the held stream resumes | Current stream in the database | First-frame and hub result |
|---|---|---|
| Removal commits before admission. | None. | No `Welcome`. The server reports `unknown_machine`. |
| Admission commits and no displacement follows. | The admitted stream. | The handle registers and the stream sends `Welcome`. |
| Admission commits, then removal is consumed before registration. | None. | The patched test expects `Welcome`. The next heartbeat is refused. Refusal of an ignored kind or result is unspecified or contradicted by dispatch. |
| Admission commits, then a newer handle registers and stays. | The newer stream. | The registry returns `aborted` before the send. The patched test expects `Welcome` before that code. |
| Admission commits, then a newer handle registers and leaves. | None after the newer disconnect. | The older generation is still fenced in the database. The empty hub map permits the older handle to register. |
| Admission commits but the reply is lost. | Candidate row until cleanup or replacement. | No handle and no `Welcome`. The candidate's fenced cleanup is retried. |

### Truth table for page actions

| Standing and row | Is the exact handle live? | Result at the action's check |
|---|---|---|
| Waiting, with any presence. | Either. | Refuse both. |
| Approved, no row. | Either. | Refuse both. |
| Approved, row without both reports. | Yes. | Refuse scan and allow restart. |
| Approved, row with both reports. | Yes. | Allow both. |
| Approved, row with either report state. | Missing or an identifier mismatch. | Refuse both. The display still uses the row. |
| Approved, row with either report state. | Canceled and still registered. | The plan treats it as live. This contradicts the terminal-stream rule. |

### Truth table for a pending action and the expiry sweep

| State at action selection or at result storage | Claim | Matching current-stream result or sweep |
|---|---|---|
| Pending, before cutoff, eligible for this stream. | Stores the next attempt. | A matching result may finish it. The sweep leaves it pending. |
| At the 23-hour selection cutoff through the 24-hour sweep threshold. | No new claim. | A matching result still finishes it. The sweep uses strict `<`, so equality stays pending. |
| Past 24 hours, sweep has yet to commit. | No new claim. | A matching result and the sweep compete under one machine lock. The first terminal write wins. |
| Final outcome committed before a new claim. | No claim. | No later result changes it. |
| Final outcome committed after a claim while the writer holds the frame. | The earlier attempt is still in memory. | The send path has no rule for invalidating this reserved frame. |
| Pending `SetState`. | Claimed once per stream, regardless of expiry. | A matching result finishes it. The expiry sweep leaves it pending. |

### Answers to the eight logic lens questions

1. Admission generations order under the machine row lock, start at one, and persist through restart. The live hub forgets a higher generation on unregistration, leaving the delayed older registration in 02. Equal-generation registration and generation or attempt exhaustion have no explicit rule beyond the database's signed integer range. Event zero, duplicates, gaps and the unrepresentable next number have separate outcomes, and the mark persists through restart. Command sequence gaps do not affect its ordering, and the table above gives the new cutoff boundaries.
2. Unknown admission commits now queue identifier-fenced cleanup, a rejected delete retries until success or restart, and a dead handle must fail the action gate while cleanup waits. Shutdown must signal independently of maintenance. The two expiry clocks and result precedence are covered by 05 and P5. See `src/server/internal/store/pool.go:124` and `src/server/internal/store/pool.go:338` for unknown commit classification.
3. The three tables name the conflicting cells. For two competing terminal causes, whichever first cancels the stream must win, and a previously reset transport can carry only its transport error.
4. An absent handle denies an action. A canceled existing handle is the missing equivalent. A zero event number refuses, zero stored mark admits event one, zero attempt refuses as a result, and a present zero feed position is tested. The skeleton's nil constructor result or zero migration accessor is unsafe when the next gate calls it.
5. The handshake and hub can act on the same generation, the watchdog and a page call on the same liveness decision, and the sweep and the writer on the same command. The row lock covers the database writes, while the missing boundaries in 02, 03 and 05 concern process state or the wire.
6. The mutation table below checks each risky item's named changes by reading the stated assertions. It also identifies the missing `AcceptResult` case. Dropping that lock can pass the listed sequential result tests if it reads current, pauses for replacement, then updates before B claims.
7. The mutation table gives an example for every item, including the skeleton and the transport. A possible survivor is an assessment of the listed assertions. Unchanged behavior outside the cited patch consequences is outside this round's findings.
8. Keep a registration fence after handle removal, make terminal state part of the exact-handle check, and give finalization and send permission an explicit order. Use a compatibility gate with real migration accessors and constructors to rule out a skeleton that builds but cannot initialize its first fixtures.

### Mutation coverage and examples that could still pass

| Item | What the named tests observe by reading | Wrong behavior that could escape those assertions |
|---|---|---|
| 0. Skeleton | The sentinel check proves distinct identity and store propagation. | It leaves a service constructor unusable for its first fixture. |
| 1. Registry | The second lock detects the weaker lock, and the approved case detects an unconditional delete. | It drops the identifier tie-break when two machines share a join timestamp. |
| 2. Deadlines | The started callback and transport cancellation tests distinguish ordinary expiry from cancellation. No risky mutation is assigned. | It silently ignores a failure to install the write deadline. |
| 3. Tokens | The verifier assertion detects saving the secret, and the duration assertion detects 16 minutes. | It ignores the batch limit when pruning more than 200 eligible rows. |
| 4. Joining | The replay after expiry, parser check and Connect and gRPC metadata tests cover the named expiry, comparison and header mutations. | It fails to preserve replay semantics when PostgreSQL commits and the reply is lost. The test retries a call whose original outcome was delivered. |
| 5. Presence | The stale heartbeat test detects the missing stream predicate. The ordering test detects the removed registry lock if the hook still runs at the gate. | It treats a canceled but registered handle as live. |
| 6. Commands | The attempt and cross-machine cases reject the named result mutations. The commit gate and indefinite `SetState` case detect early return and expiry, respectively. | It omits the registry lock from `AcceptResult` and closes an old result in the replacement window. |
| 7. Events | The invalid duplicate and invalid gap cases detect checking payload first. The commit gate and ordering test cover the split transaction and missing registry lock. | It writes the same report twice inside one commit. |
| 8. Handshake | The examples, refusal order, removal gate and replay check catch the swapped proof, early revision check, early send and repeated nonce. | It drops candidate cleanup after an unknown commit, which the fixture never produces. |
| 9. Hub | The test with generation 2 still registered catches replacing it with generation 1. | It forgets generation 2 when it unregisters and then installs generation 1. |
| 10. Stream | The commit hold catches an early acknowledgement. The transport test should identify a bypass by requiring reset at its 200-millisecond deadline before the watchdog can end the stream. | It drains a genuinely saturated input buffer before serving a command. |
| 11. Watch | The saved-message assertion detects a missing position. The live-session case catches unconditional authorization if it drives a matching change after revocation. | It passes a context correctly but exercises revocation only when no send is blocked at the recheck. |
| 12. Records | The request-key and approved-machine revocation cases distinguish both named mutations. | It leaves link rows behind on removal of a waiting machine that already connected. The explicit connected-removal and used-token-revocation cases are different paths. |
| 13. Actions | The failed enqueue and joining scan cases catch separate approval storage and a relaxed scan gate. The receipt replay catches a reply rebuilt after later edits. It does not force the original projection to precede every interleaving after commit. | It accepts a terminal handle before unregistration or projects after commit and stores the same reply before the test starts the later rename. |
| 14. Wiring | The unsigned link join and oversized request cases detect the wrong handler or missing route options. | It delays that fixture's shutdown signal behind a held maintenance operation. |

## Post-build checks

- Lose the admission commit reply after PostgreSQL has made the candidate row visible. Confirm that the candidate has no handle, an action refuses that row, and the retry removes it. Repeat with a later admitted stream and require the later row to survive. This differs from rejecting the commit before it reaches PostgreSQL.
- Force cancellation before a transport callback starts, during a started callback and after a send succeeds but before it unwinds. Run the Go race detector and require that the callback cannot touch the response after the handler returns. Where the response is still writable, check that its terminal status follows the chosen first cause.
- With the real proxy, hold a previously authorized first frame in transit while removing or replacing the machine. Require the selected terminal outcome after the first frame, and verify through the database that no subsequent message changes the machine.
- Restart the server with a failed delete and an existing command, and require startup to clear the stale presence while preserving the machine, event mark and pending command. A later stream may claim the command only while its send window permits it.

## Grade

5/10. One blocker, seven majors and zero minors.
