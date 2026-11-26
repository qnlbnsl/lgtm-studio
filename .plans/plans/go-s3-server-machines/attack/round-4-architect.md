REVISE

The skeleton declares the names, but item 8 still calls behavior that only item 10 would implement, so the handshake cannot pass its independent merge gate.

## Findings

### architect-r4 01 · BLOCKER · R10 · The skeleton leaves required behavior behind later merge gates

The skeleton replaces unfinished bodies with `errNotBuilt`. Item 8's handshake calls `serve` after `Welcome`, and its stale-admission case explicitly disconnects stream B before resuming stream A. The stub closes B as soon as `Welcome` is sent, so the test cannot control that exit or distinguish a stale rejection from the default stub failure. More directly, item 8 sends ambiguous admissions to the retry that only item 10 implements. No owner is assigned to install the service context in the body of `link.NewService`: item 0 completes only constructors that keep their arguments, item 4 no longer owns `link/service.go`, and item 8 prescribes creating the context but also omits that file. The new four-argument `app.Start` has the same sequencing problem if item 0 changes its signature and only item 14 adapts `servertest.Start` and `Restart`. The predecessor fixture starts the application in both cases. See `plan.md:132`, `plan.md:133`, `plan.md:251`, `plan.md:347`, `plan.md:350`, `plan.md:357`, `plan.md:358`, `plan.md:360`, `plan.md:580`, `plan.md:669`, and `.plans/plans/go-s2-server-signin/plan.md:479`, `:487`, and `:545`.

Give item 8 the service constructor and an interim context-aware wait after `Welcome`. Let item 10 supply the final loop in a separate file. Give item 8 a real way to hand off cleanup without blocking before `Run` exists, and assign the producer and worker to named files. Adapt the command and fixture call sites in the same commit that changes `app.Start`, preserving their sprint 2 behavior. Give `NewState` a full typed signature in item 0. As printed, `NewState(s, r, h *Hub)` gives all three parameters type `*Hub`, although the state uses the store and registry as well as the hub. Require an assertion failure from the usable predecessor for each subsequent item. Also use a separate, mutation-tested case for `AcceptResult` in item 6's shared lock-line test, which `plan.md:126` requires and the item 5 cross-reference at `plan.md:338` omits.

Category is design. Evidence is by reading. The Server planner owns the changes. The independent merge gate is required by `.plans/epics/go-rewrite/epic.md:71`–`:76`.

### architect-r4 02 · MAJOR · C8 · A registered handle can outlive the live stream

The proposed `Hub.Register(machineID, generation, cancel)` accepts neither the stream identifier nor a way to read the stream's cancellation state. `Hub.Holds(machineID, streamID)` is specified as a registration lookup, although the guard needs to know whether the registered stream can still serve. For example, the watchdog cancels a fully reported stream, its `Disconnect` runs or fails, and the handle remains registered until `Unregister`. A page action that obtains the machine lock after the failed delete returns and before `Unregister` sees the old row and `Held=true`. It can commit a restart or scan for a stream whose terminal cause is already set. The degraded case permits the old display but explicitly requires refusing such an action. See `plan.md:328`, `plan.md:330`, `plan.md:351`, `plan.md:368`, `plan.md:380`–`:383`, `plan.md:570`, and `decisions.md:86`.

Have item 8 register the exact stream identifier and a liveness check from the stream context. Have item 9 make `Holds` require the matching handle and a live context, and invalidate the hold before attempting fallible database cleanup. Item 5 joins that result to the row and item 13 makes the decision inside its checked write. Define the linearization point as the hold check within the machine-locked transaction. Pause teardown after cancellation and before unregistering, keep the old row by refusing the delete, and require both actions to refuse without adding a command. Repeat across replacement to prove that the old stream cannot clear the new hold.

Category is design. Evidence is by reading. The Server planner owns the change.

### architect-r4 03 · MAJOR · C10 · Replacement and removal still have incompatible admission tests

Hold stream A at `AfterAdmission`. Admit and register B, then release A while B is registered. The hub rejects A before `Open` sends `Welcome`, but `TestNewStreamReplacesOld` requires A to receive `Welcome` before that refusal. If B disconnects first, its handle disappears, so the same registration rule accepts superseded A. The durable generation still belongs to B. Item 8's stale-admission test asks only that A ends and that B's generation remains, so the `serve` stub can conceal this registration defect. There is a separate disagreement on removal: the Server test requires `Welcome` when removal followed admission, while combined check 10 of the Contract plan requires no `Welcome` in that very case. The wire comment allows a frame already sent, without describing the authorized frame that is still held at the seam. See `plan.md:327`, `plan.md:358`, `plan.md:360`, `plan.md:368`, `plan.md:393`–`:394`, `.plans/plans/go-s0-contract/plan.md:766`, and `src/contracts/proto/lgtm/link/v1/link.proto:129`–`:137`.

Keep the single admission commit. For replacement, keep the highest admitted generation available to the registration check for as long as any older admission can be resumed, even if the newer handle exited. Require `aborted` before `Welcome` when A is already superseded, with B still registered and after B has disconnected. Assert that A never captures a live handle. For removal, put the round 3 ruling into the Contract's comment and check 10 before the freeze. Require no `Welcome` when removal precedes admission. Permit an already authorized `Welcome` when removal follows admission, then refuse the next state-changing frame with `unknown_machine`.

Category is design. Evidence is by reading. The Server and Contract planners own the changes.

### architect-r4 04 · MAJOR · C11 · The outage gate still lacks a committed-effect count and an assigned script change

P4's intermediate reads fix the earlier final-state problem. They still cannot distinguish event 41 taking effect twice from event 41 taking effect once when both writes store the same facts and leave the mark at 41. A replay at the same mark can also rewrite those facts without changing either sampled value. The combined fixture exposes admission seams and wire clients, and names no event-commit barrier or durable effect counter. The Stack plan's gate still lists the server logs. Its script item changes the state directory, and the script still fails if an applied log line is missing. The fold requests an empty client outbox, but supplies no agreed observation for a running client. The existing `Len` belongs to the client's internal store. See `plan.md:110`, `plan.md:149`–`:154`, `plan.md:481`–`:490`, `plan.md:702`, `deploy/test/checks.sh:541`–`:587`, `deploy/test/README.md:86`, `.plans/plans/go-s3-stack-client/plan.md:213`, `:318`, `.foundry/go-rewrite/results/fold-go-s3-server-machines-r3.md:74`–`:78`, and `src/client/internal/link/outbox.go:116`.

Assign the Stack script and guide change before the joint gate. Name the Server as writer of `link_machines.event_mark`, the Stack script as its reader through `psql_in_stack`, and an observation that the Client owns to prove the fixture drained the intended outage prefix. Use the mark for recovery and progress and leave server logs diagnostic. For the combined test, assign a test-only pre-commit barrier and a transactional effect counter, such as a fixture audit table populated by a trigger for the selected facts writes. Read each effect count, value, and mark in order. Force a replay with identical payload and require no second audit row. Force a commit with no acknowledgement and no log and require the combined gate to pass.

Category is design. Evidence is by reading. The orchestrator and the Server, Client, and Stack planners own the changes.

### architect-r4 05 · MAJOR · R6 · Contract still requires a retry outside the Server's new window

Lose a stored result and reconnect 23 hours and 59 minutes after expiry. Contract post-build check 11 requires that retry and expects the recorded result. The Server's new claim refuses to send the action after 23 hours, so even a correct client with an intact receipt cannot pass the combined check through the real server. The wire comment still promises to send a pending command on each new stream until it has a result, and the Server register credits a request to replace that promise as a finished fix. See `plan.md:454`–`:457`, `plan.md:466`, `plan.md:703`, `.plans/plans/go-s0-contract/plan.md:737`–`:738`, `:767`, and `src/contracts/proto/lgtm/link/v1/commands.proto:3`–`:19`.

Agree before the freeze on the 23-hour claim cutoff for actions other than `SetState`, first-outcome precedence, and the sweep no earlier than 24 hours. Exercise a retry inside the window, a connection after cutoff that sends nothing, and a matching late result before and after the sweep. If the client still needs a boundary-focused receipt test at 23 hours and 59 minutes, inject delivery in a client test instead of requiring the combined Server to send it. State the effect of the machine's own pruning clock and keep the separate bound on when a new action can start.

Category is design. Evidence is by reading. The Contract, Server, and Client planners own the changes.

### architect-r4 06 · MAJOR · R2 · The receiving plan still requires an encrypted join in wave 1

The Server now defers the encrypted case and explicitly rejects a Hypertext Transfer Protocol Secure (HTTPS) token request. The Contract's combined gate still asks for a real encrypted join, a self-signed certificate behind the deployed web server, and a wrong-pin refusal. The wave 1 stack cannot produce the required token or serve that path. The result file requests the change, and the receiving gate still includes it. See `plan.md:52`–`:54`, `plan.md:242`, `.foundry/go-rewrite/results/fold-go-s3-server-machines-r3.md:70`–`:73`, and `.plans/plans/go-s0-contract/plan.md:753`–`:758`.

Have Contract apply the scope change before the freeze. Keep the real loopback proxy join, heartbeat and restart in wave 1, and move the encrypted proxy and negative certificate pin cases to the later proxy gate. Retain the client-only pinned listener test and the wire fields.

Category is design. Evidence is by reading. The Contract planner owns the change with Stack Web.

### architect-r4 07 · MAJOR · R5 · The blocked-watch revocation check requires an earlier notification than Watch supplies

Start a blocked send just after a periodic authorization check, revoke the session, and write one matching change. The store's Watch plan rechecks while blocked on a 30-second tick. The next change reauthorizes before its own send, after the in-flight send. With one change and no lag, the first send hits the ordinary 10-second transport deadline before Watch has reason to cancel it. P11 requires it to end sooner through Watch's context. A correct implementation can therefore fail P11. Passing the correct context into `Send` fixes propagation after detection. See `plan.md:309`, `plan.md:538`, `plan.md:707`, `.plans/plans/go-s1-server-store/plan.md:611`–`:621`, and `src/server/internal/watch/watch.go:39`–`:58`.

Use the Store's controlled authorization tick for the blocked-send scenario, or explicitly stage revocation before a known imminent tick. Assert that an observed revocation cancels the sender before its transport deadline. Separately allow the transport deadline to end a blocked send when no authorization poll falls within it, and require that no change written after revocation is authorized for delivery.

Category is design. Evidence is by reading. The Store and Server Machines planners own the changes.

### Merge gates and runtime risk

Findings 01 and 03 through 07 concern required gates with incompatible or insufficient assertions. Those gates require correction before the corresponding merge. Finding 02 scores 48. The action remains wrong until the owner retries it or the link recovers (severity 4), affects one user's machine and is recoverable (fallout 3), and requires a cleanup failure with an overlapping page action (probability 4).

## Answers to the task questions

1. The table covers every group in the round 3 register. Repeated findings share their group's result.

| Group | What the patch closes and what still fails |
|---|---|
| A | The skeleton removes the missing-sentinel dependency from item 6 to item 7. The files, app call sites and behavior needed by item 8 still prevent the independent gate. See finding 01. |
| B | One commit now fixes the database order and eliminates `Confirm`. Registration and the two plans' acceptance tests disagree on the authorized first frame. See finding 03. |
| C | The revised display and retry policy honor the allowed degraded case. A canceled registered handle can still admit an action. See finding 02. |
| D | Matching cleanup to the preallocated stream identifier protects a replacement row. Item 8 invokes the handoff before its worker exists. See finding 01 and the ambiguous-commit check below. |
| E | One inbound frame and one claim per turn, a finite receive channel, a cancellable store call, and the removal of bulk expiry give each side a bounded turn. The new test covers a held claim. A full channel is specified to lead to watchdog teardown if the reader cannot reach a heartbeat. |
| F | Intermediate values and marks improve proof of order. Identical duplicate effects and the receiving script remain open. See finding 04. |
| G | The plan now separates claim, acceptance and sweep. The Contract and combined replay test still require the later send. See finding 05. |
| H | P5 now distinguishes a retained receipt from one pruned by the client's clock. Both outcomes match the client's planned replay-before-expiry check. See `.plans/plans/go-s3-client-link/plan.md:238`–`:246`. |
| I | A canceled sender joins a started deadline callback before returning. The installed Go 1.26.0 source documents that the stop function does not wait, so the new join addresses the race. See `plan.md:309`–`:310` and Go `src/context/context.go:319`. |
| J | The Machines adapter now passes Watch's own sender context. P11 requires cancellation before Watch is scheduled to detect revocation. See finding 07. |
| K | The first-cause rule now applies on every exit, including a held store call. The plan sends failed shutdown cleanup to startup, and the new test covers both shutdown and watchdog errors. The worker's drain and leftover row still need the post-build check below. |
| L | The Server's gate now states the deferral. Contract still requires the encrypted run. See finding 06. |
| M | The signed maximum case is tested and remains distinct from a gap at an ordinary mark. The contract handoff still needs to preserve that precedence. `decisions.md:93` describes every oversized input as invalid, while `plan.md:493` still expects an oversized gap at mark 100. |
| N | The named store errors have one mapping order and normalization moves before `WriteOnce`. The current store exposes the required sentinels and constraint metadata. See `src/server/internal/store/receipts.go:33`–`:38` and `src/server/internal/store/pool.go:88`–`:118`. |

2. I attacked both mechanisms marked `UNREVIEWED MECHANISM`. The skeleton lacks a complete first usable commit and the hold lookup lacks a live stream identity and cancellation check. See findings 01 and 02.

3. Nothing found. No row in the round 3 register has the outcome `rejected`.

4. I traced the changed interfaces and their callers in the table below. Findings 01, 02, 04, and 05 identify missing writers, consumers, or fixture inputs.

| Changed names | Writers and readers |
|---|---|
| `app.Hooks`, `app.Start`, `api.Options.LinkOptions`, `servertest.Options.LinkSeams`, `Clients`, and `Link` | Item 0 declares them. The command and both fixture start paths call `app.Start`. Item 14 forwards the hooks to the service, supplies the handler options and implements fixture clients. Item 2 passes the route options to the generated handler. The orchestrator and the Stack command tests consume the fixture clients. The timing of the fixture caller edits remains open. |
| `BecomeCurrent`, `State.Confirm`, `AfterAdmission`, and stream identifiers | Item 5 owns the admission and conditional disconnect writes. Item 8 supplies the identifier, invokes admission and registers the handle. Item 10 consumes the admitted stream and runs retries. `Confirm` and its seam are removed. The fixture, Server tests, and Contract combined checks consume `AfterAdmission`. Those tests disagree about its result. |
| `Hub.Holds`, `Presence.Held`, and `CancelCauseFunc` | Item 9 owns the hub. Item 8 supplies the registration and cancellation inputs. Item 5 supplies presence to items 11 and 13. Item 11 projects row-based display, and item 13 reads the hold for actions. Item 10 ends streams and unregisters them. The registration input lacks the identifier and liveness information read by the hold. |
| `transport.WithTimeout`, `Send` and the Watch sender context | Item 0 declares the functions and item 2 implements them. Item 8's scripted server and items 10 and 11's transport tests consume the timeout wrapper. Items 8, 10, and 11 call `Send`. The Store Watch helper supplies the sender context. |
| `afterLock`, `afterClaim`, and `checkOneLine` | Item 0 declares the private seams. Item 5 fills and owns the lock seam and writes the helper. Item 6 fills the claim seam and uses the shared helper, as does item 7. Item 10 uses the hold and cancellation tests. The item 6 call for `AcceptResult` needs an explicit test case. |
| The four schema files, `link_machines.event_mark`, and `ErrInvalidMessage` | Item 0 declares the sentinel and owns the four schema files. Item 14 registers the production migrations and the migrated fixture reads the same schemas. Item 7 writes the mark, item 5 reads it for admission, item 10 sends it as an acknowledgement, and the combined suite and proposed Stack query read it. Items 6 and 7 return the sentinel and item 10 maps it. The Contract and Client own their side of the number boundary. |
| `Issuer`, `TokenOptions`, `NewToken`, `NewTokens`, `Tokens.Create`, and `DecodeToken` | Item 0 declares these. Item 3 supplies the behavior. The Server command for the installer and its tests read them, item 12 uses them for the page, and item 14 constructs the issuer from the public half of the vault key. The scripted machine uses `DecodeToken`, and Client enrollment consumes the token's wire fields. The Stack command already waits for item 14. See `.plans/plans/go-s3-stack-client/plan.md:85`, `:126`–`:127`. |

5. The interaction table shows where the patched rules agree and where one required branch fails.

| Inputs | Expected branch | Result of the patch |
|---|---|---|
| No row, or a row for a stream this process does not hold | Both live-stream actions refuse. The row may still drive the degraded display. | Correct when the hold actually returns false. |
| Approved row and live matching hold, with zero or one report flag | Restart queues and scan refuses. | The action rules agree. |
| Approved row and live matching hold, with both flags | Both actions can queue. | The action rules agree. |
| Approved row with both flags and a canceled handle still registered | Neither action queues. | A map-only hold returns true. See finding 02. |
| Removal commits before admission | No `Welcome`, then `unknown_machine`. | Both plans can require this branch. |
| Admission commits before removal, and `Welcome` is held at the seam | `Welcome` is permitted, and the next state-changing frame is refused. | The Server and Contract tests disagree. See finding 03. |
| Newer admission exists, and its handle is present or has just exited | The delayed older stream cannot register in either case. | A live-only map rejects the first case and admits the second. See finding 03. |
| Ordinary pending command before 23 hours after expiry, from 23 to 24 hours, or after the sweep | Claim before cutoff. After cutoff, accept a matching late result while pending. After sweep, keep the recorded expiry. | Server rules agree. Contract's 23-hour-and-59-minute retry does not. See finding 05. |
| Next event crosses the signed maximum, or the decoded number is larger than the next event | The immediate next event beyond the ceiling gives `invalid_argument`. A gap follows the explicit gap rule. | The tests distinguish these branches. The Contract request and decision 86 need the same distinction. |
| Blocked watch and revocation before or after the last possible authorization tick within the send | Cancellation can beat the deadline only if revocation is detected before it. | P11 has no condition on tick phase. See finding 07. |

6. Yes. The round 3 blocker in group A remains as a testable behavior dependency. Groups B, C, F, G, and L remain open in findings 02 through 06. Group D's cleanup rule protects the replacement, but its item 8 handoff depends on item 10. Groups E, I, and K have the needed policy or local mechanism, with implementation checks below.

7. The patch touches all eight architect questions. In particular, it changes the build order, half of the stream lifecycle, and the receiving checks across teams. The following answers give the evidence for each question.

## Answers to the architect questions

1. **Source and tools.** Confirmed live, the review uses `ac88c866` through `8533e992` and Go 1.26.0. By reading, `Store.Write`, `WriteOnce`, `Snapshot`, the migrated fixture and the `watch.Serve` sender signature resolve. The follower and Watch bodies are still predecessor stubs. The generated Link handler accepts variadic handler options. The sign-in router, app, command, and `servertest` are predecessor plan targets at this commit. The vault field is an Ed25519 private key and the issuer requires the public key. See `src/server/internal/store/feed.go:104`, `src/server/internal/store/receipts.go:44`, `src/server/internal/store/read_feed.go:35`, `src/server/internal/store/storetest/migrated/store.go:12`, `src/server/internal/store/follow.go:24`–`:46`, `src/server/internal/watch/watch.go:39`–`:58`, `src/contracts/gen/go/lgtm/link/v1/linkv1connect/link.connect.go:280`, and `src/server/internal/vault/identity.go:16`–`:20`.

2. **Shared names.** The task answer above traces the package writers and cross-item readers. The unresolved handoffs are the skeleton's callers and constructor, liveness inputs, event proof seams, and the shared timing contract.

3. **Placement.** The four migrations and the new hub and transport behavior are Server work. Contract owns the wire comments, Stack owns its script, and the orchestrator owns the third-package combined test using the two public fixtures. The Server writer of the installer command edits `src/server/cmd/lgtm-server/` after the machines wiring merges. This respects the epic's edit table. See `.plans/epics/go-rewrite/epic.md:115`–`:137` and `.plans/plans/go-s3-stack-client/plan.md:124`–`:129`.

4. **Rules.** The following checks cover the numbered design rules at `docs/specs/2026-11-25-go-rewrite-foundation-design.md:362`–`:372`.

| Rule | Result for the patches |
|---|---|
| R1 | Kept. The separate server parts retain their schemas and files. The skeleton's within-team ownership questions are in finding 01. |
| R2 | Broken at the joint freeze. Admission, the send window, and the encrypted test are out of agreement across the plans. |
| R3 | Kept. Link calls the registry, and Machines calls `machines.Link`. The Stack's proposed direct read is a disposable integration test. |
| R4 | Kept. Admission, the mark, and cleanup each use the Store. Shared lock-line proof needs the case named in finding 01. |
| R5 | Kept in placement and context propagation. The blocked-revocation check requires the correction in finding 07. |
| R6 | Kept in storage placement. The real combined retry is impossible as specified. See finding 05. |
| R7 | Kept for durable state. PostgreSQL orders admission and owns presence rows and marks. The hub describes local delivery capability and needs the correction in finding 02. |
| R8 | Kept. The page's keyed errors have a defined order and the stream's terminal cause takes precedence once set. |
| R9 | Kept. The changed observations need marks, facts, and test counts. They require no secret or raw exception in logs. |
| R10 | The required database and one-process link checks remain, but the independent build and combined replay evidence have gaps. See findings 01 and 04. |
| R11 | Does not apply. The patch copies no third-party code. |

5. **Neighboring plans.** Store supplies the transactions, follower and Watch context, and Sign-in supplies the router, instance lock and test fixture. Stack Web supplies the loopback route and vault configuration. Stack Client consumes machine token creation and runs the script. Client Link consumes admission and command timing and supplies the real client fixture. The epic's five agreements are at `.plans/epics/go-rewrite/epic.md:175`–`:183`. Findings 01 and 03 through 07 cover the mismatches that touch the patched agreements.

6. **Independent merges.** Steps 1 to 5 generally place schema declarations, shared tests and direct dependencies before callers. Item 8 still cannot implement its prescribed interim lifecycle on its declared predecessors. The skeleton also has to migrate all existing signature callers at once. See finding 01.

7. **Wave 2.** The patch preserves room for new link frames, keeps certificate pin fields for the later proxy, and does not add a token reader or a parallel machine service. The extra fixture observations in finding 04 are needed for wave 1 proof and can remain test-only.

8. **Dependencies.** Nothing found. The patch adds no module or image. Confirmed live, the installed version is Go 1.26.0. By reading, `go.mod` pins Connect 1.21.1 and the PostgreSQL Go driver 5.13.0, and the pinned toolchain has `context.AfterFunc` and `http.ResponseController.SetWriteDeadline`. See `go.mod:3`, `:10`–`:14`, Go `src/context/context.go:319`, and Go `src/net/http/responsecontroller.go:105`.

## Post-build checks

- The Server test should lose the reply to a `BecomeCurrent` commit whose row a fresh database connection can already read. The retry should clear that exact row. In a second run, admit a replacement before retrying and require its row to survive. Terminating a backend before `COMMIT` only proves the rollback case.
- The Server test should cancel the service with a stream held inside a claim and another held before registration, with both responses writable. Both should report `unavailable`. If the final cleanup write is refused, the handler should return without waiting for retry intake and the next start should clear the old row under the instance lock before accepting traffic.
- The Server test should send a sustained mix of events, heartbeats, and commands behind an expired prefix. It should observe progress in both directions and verify that a deliberately full receive handoff ends through the watchdog when no heartbeat reaches the reader. A writer blocked in transport should reset the stream, and the held write callback should finish before the handler releases the response.

## Grade

5/10. There is 1 blocker, 6 majors, and 0 minors.
