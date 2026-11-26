REVISE

An open Machines watch can keep showing a silent machine as online past the 30-second limit because the patched presence filter publishes no change when the connection row ages out.

## Findings

### logic-r2 01 — MAJOR — C8 — Presence can expire without reaching the page

**Evidence and failure.** Item 5 filters stale rows when somebody reads `Presence`. Item 10 retries a failed `Disconnect` at five-second intervals. The website's open watch only reloads a machine in response to its feed row. If the final heartbeat was at second 0 and the watchdog closes the stream at second 30, a failed delete leaves the watch showing its last online record. A new snapshot can already show offline while the existing watch still shows online. A successful retry at second 35 still misses the limit even with immediate feed delivery. A database clock step also makes the wall-clock filter disagree with the monotonic watchdog. By reading. `.plans/plans/go-s3-server-machines/plan.md:344`, `.plans/plans/go-s3-server-machines/plan.md:347`, `.plans/plans/go-s3-server-machines/plan.md:400`, `.plans/plans/go-s3-server-machines/plan.md:403`, `.plans/plans/go-s3-server-machines/plan.md:555`, `.plans/plans/go-s3-server-machines/plan.md:723`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:385`.

**Fix.** Specify the healthy-path publication budget and the observable fallback when the cleanup cannot publish, including what happens to an existing watch. Give the watch a deadline-driven way to stop presenting an expired value, such as a forced resnapshot with a defined unavailable state when it cannot read. Keep the generation fence on delayed cleanup. Test an already-open watch and a fresh snapshot side by side while the cleanup is blocked for longer than the retry interval. State the clock authority used for the live decision.

### logic-r2 02 — MAJOR — C10 — The last database check still precedes the first send

**Evidence and failure.** Pause stream A after `Confirm` returns and before its first `transport.Send`. Remove the machine, and hold the follower or let it queue a terminal error that `Open` reads only after its send. A can still start sending `Welcome` after the removal commit. The same interval allows an older stream's first send after a replacement became current. Decision 56 treats a completed database check as a send in transit, while the contract and its checks distinguish an unsent message from bytes already sent. The added seam only holds before `Confirm`, so its test passes with the remaining gap. By reading. `.plans/plans/go-s3-server-machines/plan.md:345`, `.plans/plans/go-s3-server-machines/plan.md:375`, `.plans/plans/go-s3-server-machines/plan.md:379`, `.plans/plans/go-s3-server-machines/decisions.md:63`, `src/contracts/proto/lgtm/link/v1/link.proto:100`, `src/contracts/proto/lgtm/link/v1/link.proto:129`, `.plans/plans/go-s0-contract/plan.md:766`, `.plans/plans/go-s0-contract/plan.md:772`.

**Fix.** Agree with the Contract team on one observable send boundary, then order removal and replacement against it. If committing permission to send is the boundary, change the contract and acceptance checks to say so. If the boundary remains the start of the send, specify the serialization of that handoff without holding a database write across blocked transport. Add a seam after the last database check, then cover both the unsent case and a genuinely buffered send through the proxy. Make every exit after admission, including a failed `Confirm` or first send, use the fenced cleanup and retry path.

### logic-r2 03 — MAJOR — C11 — The deferred trigger misses the actual commit failure

**Evidence and failure.** The patch installs a deferred trigger on a write that records feed references. The sprint 1 store design executes `SET CONSTRAINTS ALL IMMEDIATE` before it locks and appends the feed. The trigger therefore holds or rejects that statement. The named tests can pass if a future sender releases an acknowledgement after the feed flush but before PostgreSQL commits. The store's own test defers a trigger placed on `feed` after the immediate-constraint step and checks that the backend is waiting on `COMMIT`. The committed store feed at the initial checkout is a stub. The ordering is specified in the sprint 1 plan and is also present in the working implementation. By reading. `.plans/plans/go-s3-server-machines/plan.md:123`, `.plans/plans/go-s3-server-machines/plan.md:411`, `.plans/plans/go-s3-server-machines/plan.md:482`, `.plans/plans/go-s3-server-machines/plan.md:507`, `.plans/plans/go-s1-server-store/plan.md:320`, `.plans/plans/go-s1-server-store/plan.md:343`, `src/server/internal/store/feed.go:133`.

**Fix.** Agree on a fixture seam using the store's test support, or use the store's database connection fault approach to hold or reject the actual commit. Observe `COMMIT` before asserting that no command or acknowledgement has left. Check the command attempt, report, readiness flags, mark and feed after rejection. Release a second commit successfully and observe the first valid output after release.

### logic-r2 04 — MAJOR — R10 — The frozen item tests still depend on later work

**Evidence and failure.** Item 10 is allowed to start once items 2, 6, 7, 8 and 9 exist, but its new retry test requires a real `WatchClients` from item 11. An allowed merge order can therefore produce an unimplemented watch or a test that cannot compile before its declared prerequisites are met. Item 5 also assigns an `ApplyEvent` lock mutation before item 7 exists. Its expanded lock table says every old operation returns `ErrNotCurrent`, but the stale `Disconnect` row must return `false` with no error. The same implementation cannot satisfy both expectations. By reading. `.plans/plans/go-s3-server-machines/plan.md:125`, `.plans/plans/go-s3-server-machines/plan.md:331`, `.plans/plans/go-s3-server-machines/plan.md:355`, `.plans/plans/go-s3-server-machines/plan.md:356`, `.plans/plans/go-s3-server-machines/plan.md:391`, `.plans/plans/go-s3-server-machines/plan.md:410`, `.plans/plans/go-s3-server-machines/plan.md:551`, `.plans/plans/go-s3-server-machines/plan.md:695`.

**Fix.** Add item 11 to item 10's dependencies in the task and execution table, or move the real-watch assertion to an integration item with both dependencies. Put the `ApplyEvent` mutation in item 7. Give each row of the lock test its own stale outcome and state assertion. Include successful operations held against replacement in both orders.

### logic-r2 05 — MAJOR — R6 — A bounded claim still leaves a heartbeat behind a long send burst

**Evidence and failure.** The single writer sends all of a batch before returning to its select loop, and each send gets a separate ten-second deadline. A reading but slow machine can allow a command every two seconds, so each send succeeds while a heartbeat waits much longer than its ten-second interval. The reader's watchdog cannot write the acknowledgement. With a finite handoff channel it can also stop seeing subsequent heartbeats while trying to deliver a previous one to the writer. The batch test calls the store directly. The transport test stops reading entirely. Neither requires progress for both directions during a sustained send burst. By reading. `.plans/plans/go-s3-server-machines/plan.md:325`, `.plans/plans/go-s3-server-machines/plan.md:370`, `.plans/plans/go-s3-server-machines/plan.md:394`, `.plans/plans/go-s3-server-machines/plan.md:400`, `.plans/plans/go-s3-server-machines/plan.md:416`, `.plans/plans/go-s3-server-machines/plan.md:483`, `src/contracts/proto/lgtm/link/v1/events.proto:98`, `src/contracts/proto/lgtm/link/v1/events.proto:109`.

**Fix.** Keep the database batch limit, and give queued input a bounded turn between command sends. Preserve progress for each direction and keep unsent claimed commands in order while that stream is live. Specify continuation from the number of rows processed or an explicit remaining-work flag, so a full batch of expired rows cannot hide later live work. Emit no feed reference for an empty claim. Exercise a live stream with a slow reader, heartbeats, 300 pending commands and an expired prefix.

### logic-r2 06 — MAJOR — C11 — The new facts timestamp can block every later event

**Evidence and failure.** The facts patch moves the report contents into binary storage but takes `facts_at` from the incoming event and stores it as a PostgreSQL timestamp. The decision table validates the facts fields and supplies no rule for the outer timestamp. A decodable event numbered 43 can have ordinary facts and a timestamp with seconds equal to negative one trillion. Converting and writing that input directly yields a date outside the database's timestamp range. The write can fail before advancing the mark. The client then retries event 43, preventing a normal event 44 from being delivered. By reading. `.plans/plans/go-s3-server-machines/plan.md:235`, `.plans/plans/go-s3-server-machines/plan.md:240`, `.plans/plans/go-s3-server-machines/plan.md:493`, `.plans/plans/go-s3-server-machines/plan.md:497`, `src/contracts/proto/lgtm/link/v1/events.proto:24`, `src/contracts/proto/lgtm/link/v1/events.proto:44`.

**Fix.** Agree on the effect of an absent or invalid `happened_at` before freeze. Either keep a nullable diagnostic timestamp after validating it, or consume the bad next-number event without facts or readiness. Specify the choice for the applicable report kinds. Test a missing timestamp, an out-of-range timestamp and a valid successor on the same stream. Resolve the contract's promise to show the report time against the Machines record, which has no such field.

### logic-r2 07 — MAJOR — C11 — The sprint gate still depends on unordered logs

**Evidence and failure.** The patch correctly allows two post-commit log lines to swap across handlers. The same plan still gates the sprint on the script that compares the raw log order to the buffered sequence. If stream A commits event 41 and pauses before logging, stream B can read that mark, commit 42 and log first. The script can fail a correct build. Post-build check P4 leaves the existing sprint gate intact, and the durable mark plus the current facts alone cannot establish all intermediate effects or their order. By reading. `.plans/plans/go-s3-server-machines/plan.md:109`, `.plans/plans/go-s3-server-machines/plan.md:174`, `.plans/plans/go-s3-server-machines/plan.md:447`, `.plans/plans/go-s3-server-machines/plan.md:724`, `deploy/test/checks.sh:541`, `deploy/test/checks.sh:584`.

**Fix.** Make the Stack team's replacement or revision of the C11 gate an explicit rollout dependency. Have a gated fixture record the durable mark and effect at each numbered commit, or use test instrumentation that observes those commits and proves the effect is visible. Keep the production log as corroboration. Show the gate passing when the two log lines swap and failing if the effect and mark commit separately.

### logic-r2 08 — MINOR — R8 — The new shared error rule disagrees with the link handlers

**Evidence and failure.** The shared rule maps a retryable store error to `aborted`. Item 4 maps every store error in `Join` to `unavailable`. A token row lock timeout produces a retryable store error, so the two instructions give different statuses for the same retry. The shared code table assigns the busy sentence to page calls only. A store failure after admission at `Confirm` also has no explicit branch in the handshake, although the contract expects a nonfinal database failure. By reading. `.plans/plans/go-s3-server-machines/plan.md:127`, `.plans/plans/go-s3-server-machines/plan.md:188`, `.plans/plans/go-s3-server-machines/plan.md:279`, `.plans/plans/go-s3-server-machines/plan.md:375`, `src/server/internal/store/pool.go:63`, `src/contracts/proto/lgtm/link/v1/link.proto:56`.

**Fix.** State the status for retryable, unavailable and canceled store operations for each handler family. Keep `unavailable` for link database failures and `aborted` for a newer stream on the link. Include `ErrInvalidName` in the sentinel policy if name validation runs in a keyed callback. Test a retryable store error separately from a closed store.

### logic-r2 09 — MINOR — R6 — One command ownership mutation cannot fail the prescribed test

**Evidence and failure.** The round 1 ownership bug is fixed in the update. Item 6 also requires that removing `machine_id` alone make `TestAcceptResultRules` fail. Its wrong-machine input uses a second live stream with another identifier. The remaining `sent_stream_id` predicate still rejects that input, and it also rejects the replacement-stream input until that stream claims. The test can stay green for the named mutation. By reading. `.plans/plans/go-s3-server-machines/plan.md:337`, `.plans/plans/go-s3-server-machines/plan.md:458`, `.plans/plans/go-s3-server-machines/plan.md:472`, `.plans/plans/go-s3-server-machines/plan.md:473`, `.plans/plans/go-s3-server-machines/plan.md:485`.

**Fix.** Keep the complete predicate and make the required mutation drop the machine and sent-stream predicates together. The second-machine case then detects the loss of the ownership fence. If each individual predicate must be checked, add a separately named database fixture with an intentionally inconsistent stored send binding.

### logic-r2 10 — MINOR — R6 — Claim expiry needs the same terminal write as sweep expiry

**Evidence and failure.** The newly added claim path sets `outcome` to `expired` and does not specify `finished_at`. Sweep deletes rows by `finished_at`. A claim can set only the outcome, pass the new batch test and leave every such expired row in the database indefinitely. There is also a policy gap when a matching result arrives after the grace cutoff while the row is still pending. `AcceptResult` can close it with the machine's result or the sweep can close it as expired, depending on which runs first. By reading. `.plans/plans/go-s3-server-machines/plan.md:465`, `.plans/plans/go-s3-server-machines/plan.md:472`, `.plans/plans/go-s3-server-machines/plan.md:473`, `.plans/plans/go-s3-server-machines/plan.md:474`, `.plans/plans/go-s3-server-machines/plan.md:483`, `src/contracts/proto/lgtm/link/v1/commands.proto:6`.

**Fix.** Define a single transition that sets `outcome` and `finished_at` together for a final command. State the equality rule for the cutoff and the precedence of a matching late result in relation to it. Test both serialized orders of result and expiry, and verify that a claim-expired command is removed at retention.

## Answers to the assignment questions

### 1. Round 1 patches and their effects

The letters are the groups in `.plans/plans/go-s3-server-machines/attack/round-1-findings.md:9`. Groups with another disposition appear here because the register still tracks them.

| Group | What the patch closes and what remains |
|---|---|
| A | The four-way pending-row update closes the command ownership blocker. The individual `machine_id` mutation needs a different fixture or a combined mutation as described in finding 09. |
| B | Incremental manifests, external test packages and the new dependencies for items 8 and 12 close their listed problems. Item 10 still has an undeclared watch prerequisite, and item 5's expanded test has incompatible expectations. See finding 04. |
| C | Nothing found under this letter because the register assigns no group C. |
| D | Watchdog cancellation and retries improve transport shutdown and eventual cleanup. They leave the existing-watch deadline open. See findings 01 and 05. |
| E | `Confirm` closes the cases where removal or replacement is committed before the check, and a later claim checks missed removals after follower lag. The decision leaves a gap between the successful check and first send. See finding 02. |
| F | The claim now fits the 256-reference cap, and startup cleanup and sweep spread references over bounded writes. The stream still sends a whole batch before servicing input. See finding 05. |
| G | The binary report write can keep the full facts and harness types. The memory cap fits the page's unsigned 32-bit field. The new timestamp write can block the queue. The event-mark test checks a far-ahead number and leaves the actual signed boundary unexercised. See finding 06. |
| H | The new text correctly describes the limits of post-commit logs. The deployed check still depends on the original ordering. See finding 07. |
| I | Approval and rename can return the bytes saved in their keyed transaction. The lost-reply test covers approval. The keyed error and database error rules still need one consistent boundary. See finding 08. |
| J | The current installer work item and page handler both use the configured address. The installer plan still refers to an empty address option in its prose and decision history. The current initializer has the matching two issuer fields. See answer 4. |
| K | The page prerequisite is acknowledged and still needs the owner's choice. Criterion 16 runs joining through the page. The resulting restart action still needs an explicit joint gate that invokes it and observes one restart. |
| L | The cancellation hook supplies a way to unblock an active send. The post-build check below covers the race between clearing its deadline and an already-started cancellation callback. |
| M | The claim now refuses to send stale work beyond the grace period and handles an expired `SetState` result as failed. The documented clock decision supports the two rejections. The terminal timestamps and late-result rule remain open. See finding 10. |
| N | The five unwrapped refusal sentinels can pass through the store, and handler metadata can preserve the specific reasons. The new general error text needs the per-handler correction in finding 08. |
| O | The migration ledger rejects parts missing from the older manifest. The rollout now identifies forward recovery, and check P7 exercises it. Nothing found in this patch. |
| P | A single database-clock reading gives the two token timestamps an exact 15-minute interval. The replay expiry check uses a different reading after its row lock as required. Nothing found in these two rules. |
| Q | The enqueue sequence closes the wall-clock tie, and a scan now requires both current-stream readiness flags. The plan still needs the wire scheduling check in finding 05. |
| R | The new tests request the right commit assertions. Their chosen trigger runs earlier than they claim. See finding 03. |
| S | The sweep now uses the same machine lock as claims and results, closing the reverse acquisition path. The grace transition and the late-result case still need one policy. See finding 10. |
| T | Fresh random nonces with consecutive and cross-stream proof tests cover the specified replay. Stream identifiers use the same random source, but the listed mutation only checks the nonce. |
| U | `Welcome` now uses standing from admission, so standing and token agree if approval happens after the first lookup and before admission. The removal race remains the separate case in finding 02. |
| V | The test table now covers more machine writers, but its stale `Disconnect` result and the location of the `ApplyEvent` mutation conflict with their item rules. See finding 04. |
| W | Three successive admissions and a delayed older admission after the newer stream disconnects exercise the persisted generation. They do not exercise replacement after `Confirm`. See finding 02. |
| X | Both storage and projection tests distinguish present zero load from absence. Nothing found in that patch. |
| Y | The cleartext path is still an owner decision recorded before freeze. It has no new approval or implemented mitigation in this round. |
| Z | The page reply carries `Cache-Control: no-store`, and the server test checks it. The token input and process-list rule still await the owner. |
| AA | First use checks for a zero byte before writing and same-key replay avoids the storage conversion. The wire comment still needs the agreed first-use rule. The checked-in `JoinRequest.host_name` comment currently gives only the length. `src/contracts/proto/lgtm/link/v1/link.proto:165`. |

### 2. Mechanisms marked for review

Nothing is marked `UNREVIEWED MECHANISM` in the two patched documents or the register. I reviewed the added watchdog and retry path, admission confirmation, batch and cutoff policy, binary report storage, keyed replies and deferred-trigger fixture through this lens.

### 3. Rejected findings

The rejections of `critic-r1 09` and `logic-r1 6` rest on a recorded decision. The fold task names local wall-clock pruning as settled, and Client Local applies it to finished receipts. Client Link already discloses that early eviction can lose a result or repeat work. The server overview should qualify its unconditional at-most-once statement with that accepted limit. By reading. `.foundry/go-rewrite/tasks/fold-go-s3-server-machines-r1.md:53`, `.plans/plans/go-s1-client-local/plan.md:47`, `.plans/plans/go-s1-client-local/plan.md:105`, `.plans/plans/go-s1-client-local/plan.md:424`, `.plans/plans/go-s3-client-link/plan.md:26`, `.plans/plans/go-s3-server-machines/plan.md:33`.

### 4. Writers and readers of changed names

| Changed name or shape | Writers, readers and gaps |
|---|---|
| `TokenOptions`, `Issuer.PublicURL`, `NewToken.Address` | Server Machines owns the types and token address. The page handler and installer command are callers. The new command item uses only `HostsServer` and reads `cfg.PublicURL`; the obsolete empty-option wording remains in the installer plan. The web reply and the joining client are downstream readers of the emitted address. `.plans/plans/go-s3-server-machines/plan.md:145`, `.plans/plans/go-s3-server-machines/plan.md:573`, `.plans/plans/go-s3-stack-client/plan.md:99`, `.plans/plans/go-s3-stack-client/plan.md:155`. |
| `RefusalOf`, the five join sentinels and `ErrInvalidHostName` | `Tokens.Redeem` produces the errors, the store passes unwrapped domain errors, and `Service.Join` classifies them. Client enrollment reads the same wire metadata. The new host-name restriction still needs its Contract team writer recorded as a freeze dependency. `.plans/plans/go-s3-server-machines/plan.md:127`, `.plans/plans/go-s3-server-machines/plan.md:278`, `.plans/plans/go-s3-client-link/plan.md:105`. |
| `Confirm` and the retained stream generation | `BecomeCurrent` writes the generation, `Open` calls `Confirm`, and the hub compares registrations. The remove path, replacement path and publication of the first send need the shared handoff rule in finding 02. `.plans/plans/go-s3-server-machines/plan.md:343`, `.plans/plans/go-s3-server-machines/plan.md:375`, `.plans/plans/go-s3-server-machines/plan.md:387`. |
| `CommandBatch`, `seq` and `sent_stream_id` | `Enqueue` assigns `seq`, `Claim` orders by it and writes the send binding, and `AcceptResult` reads that binding. The stream loop is the batch reader. The index and tests reflect the new ordering. The loop still needs an explicit continuation contract for a full batch of expired rows. `.plans/plans/go-s3-server-machines/plan.md:394`, `.plans/plans/go-s3-server-machines/plan.md:466`, `.plans/plans/go-s3-server-machines/plan.md:472`. |
| `facts`, `harnesses`, `facts_at` and `Querier` | Registry methods own the encoded columns. Event application supplies the report time. `Get` and `List` decode the record, and `Project` and the in-transaction keyed replies consume it. The page has no report-time reader, although the wire event comment promises one. `.plans/plans/go-s3-server-machines/plan.md:133`, `.plans/plans/go-s3-server-machines/plan.md:240`, `.plans/plans/go-s3-server-machines/plan.md:554`, `.plans/plans/go-s3-server-machines/plan.md:585`, `src/contracts/proto/lgtm/link/v1/events.proto:44`. |
| `last_heard_at`, watchdog and disconnect retry | Admission and heartbeat write `last_heard_at`. `Presence` reads it for the page and commands, and successful `Disconnect` copies it to the machine. The stream loop queues cleanup for `Run`. An existing watch has no reader triggered solely by passage of time. `.plans/plans/go-s3-server-machines/plan.md:343`, `.plans/plans/go-s3-server-machines/plan.md:347`, `.plans/plans/go-s3-server-machines/plan.md:400`, `.plans/plans/go-s3-server-machines/plan.md:555`. |
| `link_event_applied` and `Cache-Control` | Event application emits the log and the deployed C11 script reads it in order. The patch acknowledges the reader but leaves its release dependency open. The page token handler writes `no-store` and its handler test reads it. `.plans/plans/go-s3-server-machines/plan.md:174`, `.plans/plans/go-s3-server-machines/plan.md:573`, `deploy/test/checks.sh:541`. |

### 5. Interacting patch rules

| Interacting rules | State at the decision | Result as written |
|---|---|---|
| Removal and first send | Removal commits before admission, or after admission but before `Confirm`. | The lookup or `Confirm` refuses with `unknown_machine`. No `Welcome` is sent. |
| Removal and first send | Removal commits after `Confirm` while the first send is still unsent. | `Open` can emit `Welcome` and then the refusal. This is the contradictory cell in finding 02. |
| Removal and first send | A send already left the server before removal. | Arrival may follow the commit, and later stream writes cannot change rows. |
| Replacement and first send | A newer generation commits before the older stream confirms. | The older stream ends with `aborted` even if the newer stream already disconnected. |
| Replacement and first send | A newer generation commits after the older stream confirms and before its first send. | The older stream's writes are fenced. Its first send has no defined fence. |
| Presence, page and actions | Waiting, with or without a fresh row. | The page shows waiting. Scan and restart are refused. A claim can send `SetState`, and results are ignored while the record is waiting. |
| Presence, page and actions | Approved, with a fresh row and an incomplete pair of reports. | The page shows joining. Restart is allowed, and scan is refused. |
| Presence, page and actions | Approved, with a fresh row and both reports. | The page shows online. Scan and restart are allowed. |
| Presence, page and actions | Approved and disconnected, with cleanup committed. | The page sees offline after the feed delivery, and each new action is refused. |
| Presence, page and actions | Approved and silent, with cleanup failing and the row past the freshness limit. | A fresh read shows offline and rejects new actions. An open watch can retain online. The stored stream-identity test and the freshness time test are separate. |
| Result, claim and sweep | Pending, current approved sender, exact machine, stream and attempt, before cutoff. | The result closes the command once. Other senders, attempts and duplicates change nothing. |
| Result, claim and sweep | Pending ordinary command after cutoff, with result and expiry competing. | Claim or sweep may record expiry first. A result may also win while the row remains pending. The cutoff precedence and the completion time for claim expiry are unspecified. |
| Result, claim and sweep | Pending `SetState` after its nominal expiry. | Claim still sends it and sweep leaves it pending. A matching expired result is stored as failed. |
| Batch and input | A full claim yields 64 live commands. | The handler schedules another claim, but all 64 sends can precede service of queued input. |
| Batch and input | A full claim processes 64 expired rows and returns zero commands. | A feed wake can cue more work. The direct continuation rule has no way to distinguish this case from an empty claim. |
| Transaction result and reply | Callback, constraint step or commit fails. | No success reply is allowed. The current constraint fixture proves the earlier failure only. |
| Transaction result and reply | Keyed write commits, the reply is lost, and the record changes. | The retry retrieves the saved bytes while its receipt remains. |

### 6. Round 1 blockers and majors marked fixed

The result-ownership blocker A is closed in the design and the refusal-sentinel blocker N is closed for the five reasons. The freeze-test blocker B still has an item-order and expected-result problem. Majors D, E, F, G and R remain partly open, as shown in findings 01 through 06. The H gate remains work for the Stack team under finding 07. K and Y retain their recorded owner dispositions.

### 7. Lens questions touched by the patch

The patch touches all eight questions. The answers below apply them to the changed mechanisms, with unchanged rules included where they interact.

## Answers to the logic lens

1. **Order and boundaries.** The machine lock orders command allocation and admission generations for one machine, and `seq` tolerates sequence gaps after rollback. Attempts persist across stream replacement and server restart, starting at one on the wire. A signed database integer stops attempts halfway through the wire's unsigned range, and the plan leaves exhaustion behavior empty. The event test proves large gaps are classified before database conversion. At the actual signed boundary the next event has no stated durable disposition. `created_at` and `expires_at` share one token clock sample, and redemption samples again after locking. The report clock orders nothing. Claim and sweep still need one grace-boundary rule. `.plans/plans/go-s3-server-machines/plan.md:259`, `.plans/plans/go-s3-server-machines/plan.md:343`, `.plans/plans/go-s3-server-machines/plan.md:465`, `.plans/plans/go-s3-server-machines/plan.md:472`, `.plans/plans/go-s3-server-machines/plan.md:494`, `src/contracts/proto/lgtm/link/v1/commands.proto:29`.
2. **Failures and repair.** Failed joins roll back and a still-eligible same-key replay recovers the committed reply. Failed event writes must leave no acknowledgement, with the actual commit test still needed. A failed cleanup is retried and fenced against a replacement, but the live watch can miss the deadline. A failed `Confirm` needs cleanup after admission. Receipt pruning's clock assumption is settled and can lose a replay outcome. See findings 01, 02, 03 and 08.
3. **Truth table.** The table in answer 5 covers removal against admission and send, replacement, presence and actions, results and expiry, batch wakeups and the transaction reply. The contradictory or unspecified cells are identified in the table.
4. **Zero and absence.** Absence of load stays null, and present zero survives storage and projection. `sent_stream_id IS DISTINCT FROM` makes an unsent command eligible. Event number zero and result attempt zero refuse, and unspecified or unknown outcome becomes failed. A waiting `Welcome` has an empty token and the first mark can be zero. The new facts timestamp has no missing-value policy. `.plans/plans/go-s3-server-machines/plan.md:357`, `.plans/plans/go-s3-server-machines/plan.md:375`, `.plans/plans/go-s3-server-machines/plan.md:472`, `.plans/plans/go-s3-server-machines/plan.md:473`, `.plans/plans/go-s3-server-machines/plan.md:494`, `.plans/plans/go-s3-server-machines/plan.md:560`.
5. **Concurrent actors.** Token users serialize through the token row, and approval and waiting deletion serialize on the machine row. Old and new stream writes, claims, results and sweeps use the machine lock. A queued cleanup names its own stream. Follower removal and the first network send have the gap in finding 02. The cancellation callback and sender can both update a transport deadline. The handler's retry work also needs a lifecycle across shutdown. `.plans/plans/go-s3-server-machines/plan.md:125`, `.plans/plans/go-s3-server-machines/plan.md:270`, `.plans/plans/go-s3-server-machines/plan.md:325`, `.plans/plans/go-s3-server-machines/plan.md:344`, `.plans/plans/go-s3-server-machines/plan.md:375`, `.plans/plans/go-s3-server-machines/plan.md:403`.
6. **Risk mutations.** Letting replay skip expiry, losing refusal metadata, allowing stale heartbeat writes, swapping proof nonces, accepting a repeated proof, reordering event checks, omitting watch positions, sending unauthorized watch updates, accepting a token-creation request key and splitting approval from enqueue all have concrete listed assertions. The `ApplyEvent` lock mutation belongs in item 7. The individual machine predicate mutation, unsent-`Welcome` interval and acknowledgement after flush are uncovered. The router mutation in item 14 trips its intended assertion. The per-item counterexamples below cover all work items, including the risky ones.
7. **Cheapest wrong implementations.** The table below checks each work item against its named tests. It treats the tests as written in the plan and describes what their stated observations would miss.
8. **States a stronger shape could prevent.** An outcome with null `finished_at` is representable, although retention needs both fields. A check constraint tying them together and one terminal-write helper would prevent that state. A signed `integer` for unsigned attempts creates a narrower storage domain. The lack of an explicit send handoff lets a completed read stand in for work that has not started. `.plans/plans/go-s3-server-machines/plan.md:365`, `.plans/plans/go-s3-server-machines/plan.md:375`, `.plans/plans/go-s3-server-machines/plan.md:462`, `.plans/plans/go-s3-server-machines/plan.md:474`.

| Item | Wrong change the listed assertions can miss |
|---|---|
| 1 | Store `facts_at` as receipt time. The facts test checks the report values and the feed but does not compare the saved capture time. |
| 2 | Let a cancellation callback that's already started set the expired deadline after the sender has cleared it. The tests separate cancellation from ordinary successful sends. |
| 3 | Produce the same secret on consecutive calls. The listed token test checks length, shape and digest, not two freshly minted secrets. |
| 4 | Reject an empty host name on first use. Every positive case specifies a name, while the new rule only imposes encoding and size limits. |
| 5 | Filter stale presence but ignore whether the existing watch learns about the expiry. The direct presence test still passes. |
| 6 | Finalize a claim-expired row without a finish time. The sweep test uses separately seeded final rows, and the batch test checks only the outcome. |
| 7 | Convert the capture timestamp without validation. The event tests vary the body and number and do not send a malformed timestamp. |
| 8 | Send `Welcome` after a removal that commits immediately after `Confirm`. The seam pauses before the check. |
| 9 | Replace a handle on a repeated registration of the same generation. The tests exercise higher and lower generations only. |
| 10 | Serve no inbound message until an entire 64-send batch is drained. The stopped-reader case still terminates, and the single-command case remains green. |
| 11 | Keep an open watch showing online after the freshness limit with no feed change. Fresh projection and ordinary save/remove messages still pass. |
| 12 | Omit the keyed write on revoke while retaining it on remove. The named replay case exercises removal, while revoke is already idempotent in the ordinary cases. |
| 13 | Read the rename reply outside its keyed transaction while approval uses the correct saved projection. The lost-reply test exercises approval alone. |
| 14 | Let shutdown close only handles already in the hub. The item test uses a connected stream. The new post-build P8 exposes streams paused before and just after admission. |

## Checks that need the built server

- **Deadline cancellation.** Cancel the context as the send completes, then release a held deadline callback. Verify the callback cannot restore an expired deadline after cleanup or touch a later send, and that the terminal status is written when the transport allows it.
- **Shutdown with pending admission.** Stop the real server with one stream blocked before admission and one after admission before hub registration. Both should terminate as unavailable within the drain budget. A new process should find no stale presence and retain the previous event mark.
- **Replacement with failed cleanup.** Keep the first cleanup failing through a new admission. Allow the old retry to run after the new stream reports both facts and harnesses. The old retry should remove no current row, and the open watch should retain the new presence.
- **Browser restart.** Once the owner resolves the page change, select the real restart action for a connected approved machine. Observe one stored restart, a new current stream after the replacement process starts, and a return to online.

## Grade

5 out of 10. Zero blockers, seven majors and three minors.
