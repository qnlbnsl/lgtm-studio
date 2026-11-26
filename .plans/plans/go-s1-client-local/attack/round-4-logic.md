REVISE

Count-based pruning can erase a command’s only execution record while the server can still retry it, so the client can run the command twice during normal operation.

## Findings

### logic-r4 01 — BLOCKER — R6 — A receipt can disappear before the server retires its command

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:421`–`:423` runs a valid unexpired command with no receipt and prunes finished receipts by count. Lines 474 and 483–484 promise at-most-once execution and also accept a replay after eviction. `.plans/plans/go-s0-contract/draft/commands.proto:4`–`:13` keeps an unresolved command eligible for delivery after its expiry. `.plans/plans/go-s3-server-machines/plan.md:891`–`:894` places no count boundary on the unresolved commands it can issue. The approved design promises one execution and replay of the recorded result in `docs/specs/2026-11-25-go-rewrite-foundation-design.md:281`–`:286`.

**Failure.** Finish command A and lose its result on the way to the server. Finish 1,000 newer commands before the server retries A. Those completions evict A. A retry before the unchanged expiry returns `DecisionRun` and executes A again. A retry at or after expiry returns `DecisionExpired`, so the server can record a false final outcome for work the machine already performed. With `MaxReceipts` set to 1, one newer completion is enough to reproduce both branches. The existing pruning test assumes that starts and finishes occur in the same order. If A starts first and finishes after the window has filled, retaining its original `order` can evict its result in the transaction that just recorded it.

**Fix.** Keep the two written states and count-based pruning. Require an agreed server delivery rule that prevents a retryable result from falling out of the client’s retention window. A cap on simultaneously pending commands alone is insufficient because resolved commands could keep passing an older pending one. Define whether `order` measures start or finish and test the chosen rule with overlapping commands. Before the two teams freeze the contract, test an unresolved command through one full retention window and retry it both before and after expiry. If the count is intended to permit duplicate effects or loss of the result, send that change to the approved guarantee to the owner under ruling 8.

### logic-r4 02 — MAJOR — C11 — The store can open a queue with a missing tail or wrap its next number

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:412`–`:419` accepts adjacent event keys whose last key is below `next`. Line 431 tests an interior gap and a last key equal to `next`. The predecessor at `b68adba0`, lines 652–665, also specified refusal before exhausting the sequence. `.plans/plans/go-s0-contract/draft/events.proto:8` and `:14`–`:16` require a gap-free sequence at the server.

**Failure.** Open a structurally valid file with event keys 8 and 9 and `next = 11`. Its walk succeeds. With the server at 7, it applies 8 and 9. The next append takes 11, and the server refuses it because 10 is missing. A cumulative acknowledgement can only remove a prefix, so this file could never have been produced by the stated transactions. Separately, a file with `next = MaxUint64` can open and the next append increments `next` to zero. `Ack` then treats every number as ahead, and another append can assign zero.

**Fix.** Require `next` to be an eight-byte positive number and require a nonempty queue to contain ordinary event values under positive consecutive keys ending at `next - 1`. Keep the empty queue valid for positive `next`. Restore a permanent exhaustion error before incrementing would cross the chosen ceiling. Define the sender’s terminal response to that error. Add fixtures for a missing tail, a zero key, zero `next` and the exhaustion boundary. These checks need no sent mark or restore protocol.

### logic-r4 03 — MAJOR — C11 — Existing-file validation still opens a path that writes before refusing

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:414`–`:416` avoids the empty-file initializer but uses a writable `bolt.Open` before the read transaction. Lines 431–432 cover conventional files. `.plans/plans/go-s1-client-local/decisions.md:64` permits bbolt v1.4.3. Its `db.go:298`–`:318` writes the free-page list in a transaction before returning when that list was previously left unsynced. This write is inside the library’s open call and bypasses the plan’s commit hook at line 411.

**Failure.** Create a nonempty file with the three buckets, an unsupported application version, and bbolt’s `NoFreelistSync` option. With the permitted v1.4.3 fallback, a local probe confirmed that opening under the default writable options changes the file’s bytes before the application can refuse the version. The same fixture with a synced free-page list retained identical bytes. The round 3 refusal invariant still fails for the first fixture.

**Fix.** Validate existing files through a read-only handle before opening them for writes. Keep the folder claim across the transition. Require a store format and writable-open configuration that cannot normalize the file implicitly, or refuse an incompatible free-page-list format during preflight. Run the unsupported-version fixture with an unsynced free-page list on every permitted library version and require identical bytes on refusal.

### logic-r4 04 — MAJOR — R6 — The caller can lose the newest attempt at completion

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:409` and `:503`–`:505` move the attempt out of the store and tell the caller to remember it. The former caller rule at `b68adba0`, lines 845–846, explicitly used the latest attempt at completion and the arriving attempt on replay. `.plans/plans/go-s0-contract/draft/commands.proto:52`–`:60` accepts only the result for the latest attempt on the current stream. `.plans/plans/go-s3-client-link/plan.md:245`–`:251` still takes the latest attempt from the old receipt interface.

**Failure.** A scan is running under attempt 1. The new stream’s handler calls `Begin` for attempt 2 and receives `DecisionRunning`. Before that handler records attempt 2, the worker finishes and picks attempt 1. The server discards that result. The `DecisionRunning` branch sent nothing, so the command stays pending on a healthy stream until another reconnect or the server’s expiry sweep. A replay on a fresh process also needs the arriving attempt to replace any old attempt in the saved result.

**Fix.** Give the Sprint 3 Client command handler one serialization point for classifying a delivery, publishing the highest valid attempt with its stream, and selecting the attempt at completion. If delivery wins, completion uses the new attempt. If completion wins, the new delivery immediately replays the final result under its own attempt. Specify that the durable payload is attempt-independent or that the caller rebuilds the wire envelope on every replay. Restore the reconnect-during-scan case and force both completion orders.

### logic-r4 05 — MAJOR — plan — The Sprint 3 reader still builds against removed store methods

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:408`–`:410` and `:497`–`:505` replace the earlier interface. `.plans/plans/go-s3-client-link/plan.md:170`–`:175` still calls `Reconcile` and `MarkSent` and expects renumbering. Lines 245–251 still require `Get`, `Prune`, attempt fields and a pruning clock. At `809ab00b`, `.plans/plans/go-s0-contract/draft/events.proto:11`–`:18` likewise requires renumbering. The working copy of that draft already removes it at lines 8–21. The requests exist in `.foundry/go-rewrite/results/fold-go-s1-client-local-r3.md:51`–`:59`, and the consumer plan still names the old interface.

**Failure.** The Sprint 1 builder implements the published store. The Sprint 3 builder writes the replay and command tests as named in the current plan. The calls to the deleted methods fail to compile, and the assertion that unsent events move to a new number is incompatible with either the patched store or the revised event draft. The published checks can pass Sprint 1 while this handoff remains broken.

**Fix.** Before freezing the Sprint 3 Client plan, update its method calls, data shapes, clock and tests to match the revised Sprint 1 interface. Make the contract and Server plan agree with the new acknowledgement behavior and the resolved receipt retry rule. Record the writer, every reader and an explicit dependency on that synchronized revision. Keep the outage producer specified in the Sprint 3 Client plan.

### logic-r4 06 — MAJOR — D19 — The installer’s idempotence check still reads the old enrollment marker

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:123`–`:136` names `identity.json` and lists its Sprint 3 Client and user-flow readers. The Stack request appears in `.foundry/go-rewrite/results/fold-go-s1-client-local-r3.md:56`–`:58`. `.plans/plans/go-s3-stack-client/plan.md:224`–`:225`, `:236`, `:253` and `:306` still use `server.json`. That Stack plan already passes the same override to its initial join and service at lines 225 and 232. `deploy/test/checks.sh:219` and `:233` still set the old variable, while line 520 reads the marker under the fixture folder.

**Failure.** Install and join once using the patched client, then rerun the installer without `--rejoin`. The old marker is absent, so the installer requests another token and calls `join`. Rule 8 at `.plans/plans/go-s1-client-local/plan.md:509` refuses the existing identity, so the normal reinstall exits in failure. The frozen installer shim fabricates the old marker, so its listed rerun test would pass. The check script can also reach the wrong state tree until it passes the new override.

**Fix.** Add the installer as a reader of `identity.json`. Make the Stack plan, its shim and both rerun tests observe that file. Update the isolated fixture’s `join` and `run` calls and the test documentation to use `LGTM_CLIENT_STATE_DIR` for the same absolute directory that contains the marker. Confirm the handoff against an installed client and an independent fixture client.

### Risk scores and responsible teams

| Finding | Category and label | Severity, fallout and probability | Score and band | Evidence | Responsible teams |
|---|---|---|---|---|---|
| logic-r4 05 | Design, major | 6 for a broken integration, 5 for coordinated rework, and 10 if built as written. | 300, before release. The build gate must pass. | Confirmed by reading both plans and the contract at the reviewed commit. | Client, Contract and Server. |
| logic-r4 06 | Design, major | 6 for a failed reinstall, 3 for an owner-recoverable failure, and 6 for an ordinary rerun. | 108, schedule. | Confirmed by reading the two plans and the live test script. | Stack and Client. |
| logic-r4 02 | Integrity, major | 6 for a stuck queue, 5 for recovery work, and 2 for a rare damaged or exhausted sequence. | 60, schedule. | Confirmed by applying the written predicates and checking the server rule. | Client. |
| logic-r4 01 | Integrity, blocker | 8 for an irreversible duplicate effect or false final record, 7 for an effect that cannot be undone, and 1 for eviction and loss of the result during the retry window under the default. | 56, schedule. | Confirmed by reading both policies. The deployed frequency is unknown. | Client, Server and Contract. |
| logic-r4 04 | Timing, major | 4 for a result stuck until retry or sweep, 3 for recovery through reconnect, and 4 for a disconnect during work. | 48, record. | Confirmed by the interleaving allowed by the plan. | Client. |
| logic-r4 03 | Integrity, major | 6 for modifying a file pending recovery, 3 for one affected machine, and 2 for an unusual on-disk format. | 36, record. | Confirmed live with bbolt v1.4.3. | Client. |

## Answers to the round 4 assignment

1. **Each register outcome and patch.** The two tables below cover the 13 finding groups and the ten patch entries in the register’s list of patches.
2. **Each unreviewed mechanism.** Decision 81’s event walk catches interior gaps, while a missing tail and zero values pass the written predicates. Findings 02 and 03 cover its validation and open ordering. Decision 83’s computed interruption handles stored outcomes after ambiguous commits without a repair write. Findings 01 and 04 cover its interactions with eviction and retries. The still-live marks on decisions 55, 58 and 61 concern the shared store, temporary-file creation and ambiguous commits. The synced temporary file and no-replace link serve creation, and the one lock now serializes failure handling. Findings 02 and 03 cover the remaining risks in opening the shared store. Decisions 41, 51 and 60 describe superseded mechanisms.
3. **Rejections.** Nothing found. The register records zero rejections and marks group M for the owner.
4. **Writers and readers.** The name table below identifies which updates already have an owner and which readers still need an amended plan or test.
5. **Interacting rules.** The outcome tables below cover the amended event sequence, receipt decisions, lifecycle and state-directory rules.
6. **Round 3 findings still open.** Group I still permits an existing-file write before refusal and admits a discontinuity at the queue’s tail. Group H’s common resolver is specified, but the installer marker and fixture readers still need the requested changes. Group M remains the explicit owner choice. The other round 3 failure scenarios are removed, superseded or delegated as listed below.
7. **Lens coverage.** All eight logic questions touch the patch set. The answers below cover sequence bounds, failed calls, overlapping rules, zero values, actors, mutation tests, cheapest wrong builds and constraints that would eliminate bad states.

### What each round 3 group now does

| Group | Does the patch remove the described failure? |
|---|---|
| A | Yes for the supported scope. Removing restore reconciliation and renumbering removes its divergent-history failure. Restoring a prior copy remains outside wave 1. A store with an extant but smaller allocation history refuses a server number at or above its `next`. |
| B | Yes under the stated producer and consumer assumptions. Only an authenticated acknowledgement of a committed server prefix reaches `Ack`. Duplicate marks below `next` are harmless even when the queue is empty. The linked client still expects the removed sent mark, as finding 05 records. |
| C | Yes. The helper-trust promise and folder trust checks are gone under the ruling. The deadline and the restricted environment remain in item 5. |
| D | Yes. Item 6 owns creation, validation, events and receipts and depends on the completed item 2. There is no later receipt item it must import. |
| E | Yes in this plan. The linked Client plan already specifies periodic facts from an approved process during an outage at line 201, and the test at line 204 requires three durable reports. The contract treats facts as numbered reports and keeps heartbeats separate. |
| F | Yes. The patch removes producer-time ordering and the Server request for it. The linked Client plan at line 201 preserves the original time for display and follows wire sequence for application. |
| G | Yes for the faulty clock and global watermark. The replacement permits eviction while a retry remains legal. Finding 01 covers that new interaction. |
| H | Partly. The installer’s join and service already use the same override. The fixture variable and the installer’s marker reader still need the coordinated edits in finding 06. |
| I | Partly. The empty-file and interior-gap cases are covered. The writable open and missing suffix check leave the outcomes in findings 02 and 03. |
| J | Yes in the design. Admission, transaction and failure publication share one lock, and a waiting writer sees the stop. The mutation needs a deterministic test barrier. |
| K | Yes for the reported ambiguous finish and repair loop. A surviving finish replays, a surviving started receipt is interrupted, and malformed data produces interruption without a repair write. The result-pruning and attempt interactions are findings 01 and 04. |
| L | Yes. The scan no longer rejects ordinary links under the deleted trust policy. Its candidates still have to satisfy the exit and version rules. |
| M | For the owner. The fallback can remove a different waiting machine with the same host name, as the options explicitly state. The approved design supplies no narrower recovery, and the plan still carries the choice. |

### What each registered patch entry changes

| Entry | Check |
|---|---|
| 1. Overview, decisions, workstreams, rollout and owner choices | The work order is consistent with the merge. The at-most-once summary and its accepted eviction case conflict in finding 01. |
| 2. Implementation rules and layout | The folder rule has one environment override. Its Stack readers still need finding 06. |
| 3. Item 1 | `TestStateDirRule` proves that the old variable has no effect with or without the new one set. Relative overrides and an absent or relative home are refused as specified. |
| 4. Items 2 and 3 | The merged tests still assert file permissions, the no-replace path, crash order and uncertain placement. The plan’s risky tests describe the synchronization points around atomic writes. |
| 5. Item 5 | The trust code and diagnostic were removed together. Candidate order, exact version text, the common deadline and fallback behavior remain testable. |
| 6. Item 6 | The merge and failure lock close their assigned problems. Findings 01–04 identify the remaining interactions in pruning, reopening and receipt handoff. |
| 7. Sprint 3 Client obligations | The old producer and freshness rules are gone. Findings 04–05 identify the caller changes the shortened fold still needs. |
| 8. Post-build checks | The replay checks still exercise downtime and reconnect. Reinstate the attempt handoff check with the exact result the server accepts, as in finding 04. |
| 9. Traceability, guardrails, sizing and attack log | Six work items and the stated 27 launches agree with the item table. The claim that findings are mapped does not establish the reader updates in findings 05–06. |
| 10. Decision rows and scouting | Decisions 72–86 record the mechanisms and ownership requests. The empty-file and conventional existing-file probes hold for their samples. The additional v1.4.3 case in finding 03 limits what those samples establish. The repository paths for the draft contract and the Stack fixture still show the mismatches listed above. |

### Writers and readers of changed names

| Name | Assignment and remaining reader work |
|---|---|
| `Ack`, `Next`, `ErrAckAhead`, and removed event marks | Item 6 owns the methods. The Sprint 3 Client sender consumes the methods and acknowledges only current-stream frames. The Contract team owns `Welcome.last_event_number` and `EventAck.through`, the Server writes those values, and the Client reads them. The fold names those parties, but the sender’s old calls and expectations remain in finding 05. |
| `Begin`, `Finish`, `DecisionInterrupted`, the receipt payload, and removed receipt fields and methods | Item 6 owns the on-disk format and storage operations. The Sprint 3 Client handler supplies authenticated time and consumes the decisions and payload. The Contract team owns wire outcomes and retry rules. The Server owns pending commands and consumes the result. The handoff still needs the retry and attempt rules in findings 01, 04 and 05. |
| `LGTM_CLIENT_STATE_DIR` and `identity.json` | Item 1 owns lookup, the Stack installer supplies the path to join and the service, and the Stack fixture must supply it too. The Sprint 3 Client writes the identity and reads it on run. The Stack installer and the user-flow check both read it. The layout omits the installer’s read. Finding 06 gives the affected tests. |
| `ProblemUntrusted` and the deleted trust files | Item 5 owned them. The local tests dropped the old result. The Sprint 3 Client caller still references trust checks at `.plans/plans/go-s3-client-link/plan.md:185`. Its inventory writer sends names and versions to the Server and needs no local trust diagnostic. |
| Outage producer and `link_event_buffered.sequence` | The Sprint 3 Client owns the producer and writes the log after `Append`. The Stack check reads the log and compares it with the Server’s applied log. These writers and readers are present in the linked plan and script. |

### Event and store outcomes

| State or input | Required result and the patch’s outcome |
|---|---|
| New store or an empty queue after full acknowledgement | `next = 1` for a new store. An emptied queue can have `next > 1`, and a repeated `Ack(next - 1)` succeeds without deleting anything. |
| Nonempty queue with keys `8, 9`, `next = 10` | Open succeeds. On a server at 7, the suffix starts at 8 and the next append takes 10. |
| Nonempty queue with keys `8, 10`, `next = 11` | Open refuses because the walk detects the interior gap. |
| Nonempty queue with keys `8, 9`, `next = 11` | The stated walk accepts it. The next append takes 11 and delivery stops at the missing 10. |
| Zero key or zero `next` | The patch names no positive bound. An otherwise adjacent zero key can pass. An empty queue with zero `next` passes the walk in the absence of a separate range check. |
| `next` at the largest unsigned 64-bit value | The stated append wraps. Restore exhaustion refusal before the increment. |
| Acknowledgement below `next` or at or above `next` | Below `next` the store removes the prefix and repeats are harmless. At or above `next` the store refuses without change and the caller stops. This assumes the server acknowledges only committed events for the current identity. |
| Expected refusal or ambiguous commit failure | An expected refusal keeps the store usable, as the full-buffer and oversized-result tests require. A failed commit marks it failed before another writer enters. Reopening reads the result that survived. |
| Failed store or closed store and a value-only getter | The claim that every call returns an error cannot apply literally to `Next`, `Len`, `Bytes` or `Changed`. Line 435 preserves the snapshots and observes the closed channel. State the failed-store getter outcome explicitly in the interface. |

### Receipt and attempt outcomes

| State on delivery | Before expiry | At or after expiry |
|---|---|---|
| No receipt for a new identifier | Commit `started`, then run. | Return expired without a write. |
| Valid `started` in the active store | Return running and publish the newer attempt. | Same result. The expiry controls admission to work. |
| Valid `started` from a prior store lifetime, or malformed value | Return interrupted without changing the file. | Same result. |
| Valid `finished` | Replay its result under this delivery’s attempt. | Same result while the server may still ask for it. |
| Evicted `finished` while the server still retries | Runs again under the written rule. This duplicates an effect. | Returns expired under the written rule. This replaces a result that existed. |
| Bad identifier, zero `ExpiresAt` or zero `now` | Refuse before receipt lookup. | Refuse before receipt lookup. |
| New attempt arrives while finish is recording the outcome | If publication wins, finish uses the new attempt. If finish wins, delivery replays under the new attempt. The patch has no shared serialization point for these two cases. | Same ordering. |

### State folder outcomes

| Actor and setting | Outcome |
|---|---|
| Join and service with the same absolute override | Both resolve the same state folder. The installer must observe `identity.json` there on a rerun. |
| Fixture sets only `XDG_STATE_HOME` | Both commands fall back to the home-based folder. The test then looks for the marker in its disposable folder and can fail or interact with existing state. |
| `LGTM_CLIENT_STATE_DIR` empty or unset | The implementation’s layout falls back to the absolute home-based path. The decision row should use the same treatment of an empty value. |
| Override relative or home absent without an override | Resolution fails before using the folder. |

## Answers to the logic lens

1. **Order, ties, gaps, duplicate numbers, wrap and restart.** The store lock orders append commits, and cumulative acknowledgements make lower duplicate marks harmless. The walk needs the suffix and exhaustion bounds in finding 02. Receipt order also needs a definition for overlapping starts and finishes. Attempt order is a maximum per command on the current stream and needs the two branches in finding 04. Expiry equality refuses a new start. After restart the persisted receipt takes precedence over the expiry decision.
2. **Failure and repair.** Temporary-file creation and atomic placement leave a complete store name or no store name. The failing writer stops its successors. An ambiguous finish recovers as replay or interruption according to the disk. Findings 01 and 03 identify the remaining receipt-loss and open-before-refusal cases. An expired join with an ambiguous hostname still goes to the owner. A conflicting wall clock is outside the local clock calculation, which uses the authenticated sample and monotonic elapsed time.
3. **Rules acting on the same state.** The tables above show the missing tail, zero and wrap cases, the evicted receipt cases, the completion race, the failed-store getter ambiguity and the fixture using the wrong directory.
4. **Zero and missing values.** Event number zero is invalid for a stored key and meaningful as an initial server acknowledgement. The patch needs a positive `next`. A zero `Decision` has no affirmative action. Invalid command input must never authorize a run, and option zero uses the documented defaults. The layout treats an empty directory override as absent. `Read` should retain the former empty result for a nonpositive limit.
5. **Concurrent actors.** Two folder holders contend on the operating-system lock, and a second store holder contends on the folder claim. The store lock orders `Append`, `Ack`, `Begin`, `Finish` and `Close`. The replay path still needs a single current-stream writer, and the command receiver and worker need one decision point for the current attempt.
6. **Risky mutations.** Item 2’s placement, sync, permissions, exclusive lock and no-replace mutations match the listed assertions. A stale claim release clearing a later claim can pass them. Item 3’s overwrite, permissions and link mutations match the merged assertions. The formatting assertion checks the seed and one displayed string, so it can miss removal of a redundant formatter. Item 6’s fixtures exercise the named ahead, full, creation, damage and failed-write mutations. The waiting-writer test must observe that the second call reached admission before the first releases the lock. The listed receipt tests can miss a `Begin` that returns permission to run alongside a commit error. Keep both former failed-`Begin` disk outcomes and require zero side effects. Test the oversized-result retry without any premature result send.
7. **Cheapest wrong implementation that can pass.** Item 1 can implement the resolver correctly while the Stack fixture still sets the removed variable. Item 2 can keep a claim release callable after the claim was replaced. Item 3 can leave the key otherwise protected but mishandle an uncertain placement in the caller. Item 4 had no behavior change in the patch. Its listed tests still allow a caller to emit a made-up load on probe failure, while the linked plan already specifies absent fields. Item 5 can pass the output fixtures while retaining the entire child output. Item 6 can accept keys `8, 9` with `next = 11`, or prune a still-retryable command. The suite has no fixture for either state.
8. **Shapes that rule out wrong states.** Keep one owner of each event number and require the unacknowledged keys to form its suffix. Preserve one admission lock and avoid implicit writes during preflight. Couple the server retry boundary to the client retention boundary and use one caller to bind attempts to results. Use the same absolute folder and marker for each reader of enrollment state.

## Post-build checks

| Scenario | Expected result and evidence | Responsible team |
|---|---|---|
| Cut power after the store’s create steps and after an append, acknowledgement or finish returns, with a distinct record of each returned operation. Run on Linux and macOS where available. | The store opens with the last complete durable state or diagnoses damaged storage without modifying it. Every numbered event that returned remains queued or acknowledged. Each command effect appears at most once. Keep the original file and the event and effect logs. | Client. |
| Allow the server to commit an event, drop its acknowledgement, and then reconnect. Keep another committed local event that the server has never seen. | The committed prefix is discarded exactly once. The unsent event reaches the server under its original number. Keep the two ordered logs and the server’s applied records. | Client and Server. |
| Drop a stream during a requested scan. Force both orders between delivery of attempt 2 and finish on the worker. Repeat once after a process restart with the final receipt on disk. | One scan effect is recorded. The current stream returns the right result under attempt 2, and the server records that result immediately. Keep both attempt identifiers and the durable effect evidence. | Client and Server. |
| Rerun the real installer with a joined machine, then run the isolated client fixture under the same user. | The installer reuses its identity and requests no token. The fixture uses its own explicit path and observes the enrollment marker there. Keep the token-request count and identity hashes. | Stack and Client. |

## Grade

5 out of 10. There is 1 blocker, 5 majors and 0 minors.
