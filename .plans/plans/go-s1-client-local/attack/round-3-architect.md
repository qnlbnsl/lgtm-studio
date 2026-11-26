REVISE

Restoring an older `client.db` under the same machine identity can apply reports twice and let a command run again because the restore rolls back both the sent mark and the command receipts.

## Findings

The review uses the client-local patch from `9ec34833` to `b68adba0`. Evidence from neighboring plans is identified as draft evidence where it comes from the working tree. The Sprint 0 wire contract is still a draft.

### architect-r3 01 — BLOCKER — C11 — A local restore loses the history needed for safe reconciliation

**By reading.** The store treats a new database and a restored database alike when `Welcome.last_event_number` exceeds `sent`. It automatically creates a database when the file is absent. Rule 3 still writes `Hello.acked_event_number`, and the server-behind rule assumes the server raises its mark from that field. The field exists in the committed draft. The working Contract draft, Client Link plan and Server Machines decisions now give the server the sole mark and omit that field. The working event draft treats a lower server mark as `failed_precondition`, while rule 6 retries it indefinitely. See `.plans/plans/go-s1-client-local/plan.md:656`, `.plans/plans/go-s1-client-local/plan.md:668`, `.plans/plans/go-s1-client-local/plan.md:834`, `.plans/plans/go-s0-contract/draft/link.proto:208`, `.plans/plans/go-s0-contract/draft/events.proto:17`, `.plans/plans/go-s3-client-link/plan.md:157` and `.plans/plans/go-s3-server-machines/decisions.md:27`.

**Failure.** A local snapshot has `sent = 1`, with reports 2 and 3 queued. The original client then sends both, and the server commits through 3. The restored client moves reports 2 and 3 to 4 and 5. The server applies them a second time. A command that ran after the snapshot can also be redelivered before expiry while its receipt is absent. A missing `client.db` under an existing identity has the same receipt problem. A Sprint 3 client that follows rule 3 against the working Contract draft also refers to a field that the generated code will lack.

**Fix.** Agree on the server-only mark in the Contract, Client Link and Server Machines handoffs, including the outcome for a server mark below the local acknowledged mark. For wave 1, scope recovery to reopening the current database after a crash. Remove the guarantee of restoring old client state under an existing identity. Define first initialization as part of enrollment so an established client whose database is missing refuses to start. When its history cannot be recovered, require explicit re-enrollment under a new machine identity. If same-identity restoration is required, design durable report identities and a command execution history that survive the restore before enabling it. Test a stale snapshot taken before a send and before a command ran.

### architect-r3 02 — BLOCKER — C11 — The live acknowledgement bound still accepts a number that was never sent

**By reading.** Marking a whole batch before any write makes `sent` an upper bound on what may have been offered. The client-local handoff requires the acknowledgement to come from the current stream, and `Ack` checks only the durable bound. The Client Link working draft independently requires a per-stream bound raised after each successful send. See `.plans/plans/go-s1-client-local/plan.md:666`, `.plans/plans/go-s1-client-local/plan.md:667`, `.plans/plans/go-s1-client-local/plan.md:690`, `.plans/plans/go-s1-client-local/plan.md:834` and `.plans/plans/go-s3-client-link/plan.md:173`.

**Failure.** The sender records `MarkSent(3)` for events 1 to 3 and writes only event 1. A faulty server acknowledges 3 on the current stream. The local handoff permits the call to `Ack(3)`, which deletes 2 and 3 permanently. The local test rejects an acknowledgement above a mark of 1, leaving this batch case uncovered.

**Fix.** Make the per-stream rule explicit in both plans. Record one attempt just before each send. Advance the in-memory acknowledgement ceiling only after that send succeeds, and serialize the update with acknowledgement processing. An acknowledgement above that ceiling must end the stream before it reaches `Store.Ack`. Reconcile an ambiguous send using the next authenticated `Welcome` under the agreed machine history. Test a receiver that acknowledges 3 after the first of three writes and a receiver whose immediate valid acknowledgement is accepted.

### architect-r3 03 — BLOCKER — R9 — The launcher policy leaves unchecked helper targets and macOS write permissions

**By reading.** `trustedProgram` validates a top-level candidate and its links. The child gets a program search path (`PATH`) that validates directories only, although the plan claims the same trust for helpers. The macOS branch reports no access list, and the proposed device check records the gap. See `.plans/plans/go-s1-client-local/plan.md:533`, `.plans/plans/go-s1-client-local/plan.md:536`, `.plans/plans/go-s1-client-local/plan.md:539` and `.plans/plans/go-s1-client-local/plan.md:886`.

**Failure.** The search directory passes, while a helper in that directory resolves to a location that `trustedProgram` would refuse. The launcher can still reach the helper. On macOS, a folder can also pass the mode and owner checks while an access control list grants another user write permission. In both cases the scan can execute code outside its promised trust boundary with access to the local client's files.

**Fix.** Make support for helper lookup conditional on a runtime policy that enforces the same boundary at each executable resolution. Fix which launch forms are supported and give unsupported launchers a local diagnostic. Apply effective write-permission checks on macOS before executing there. Treat a permissions check that cannot be performed as a refusal. Require tests with benign helper fixtures for trusted and refused resolved targets, and require the macOS access-list fixture to refuse execution.

### architect-r3 04 — BLOCKER — plan — Item 6 depends on receipt code assigned to item 7

**By reading.** `OpenStore` is owned by item 6 and must run item 7's receipt recovery. Item 7 owns the receipt format, and waits for item 6 to merge. Item 6 must pass its own tests on the exact intermediate commit. See `.plans/plans/go-s1-client-local/plan.md:133`, `.plans/plans/go-s1-client-local/plan.md:628`, `.plans/plans/go-s1-client-local/plan.md:660`, `.plans/plans/go-s1-client-local/plan.md:709`, `.plans/plans/go-s1-client-local/plan.md:713`, `.plans/plans/go-s1-client-local/plan.md:746` and `.plans/epics/go-rewrite/epic.md:75`.

**Failure.** A builder implements `OpenStore` as specified in item 6 and references the recovery that is reserved for item 7. The intermediate commit fails to compile. Implementing or editing the item 7 recovery from item 6 also breaks the listed file ownership.

**Fix.** Make the item 6 open complete with an empty reserved receipts bucket. Assign item 7 an explicit sequential edit to the open path and its tests to add receipt recovery, or put the recovery and its format into item 6. State the required checks on both intermediate commits and assign every edited file to the corresponding item. Remove the old item 8 reference and give the harness trust code's state-folder comparison an existing reference or a documentation-only dependency.

### architect-r3 05 — MAJOR — C11 — The chosen offline producer has no numbered wire message

**By reading.** Rule 8 and decision 71 choose load reports in place of heartbeats. The contract has numbered `Facts` and `HarnessesFound` events. `Heartbeat` carries load and is never numbered. The Client Link working draft already proposes a periodic `Facts` capture after loss of an approved stream. The fixed stack check requires three client sequences, a later server sequence and the exact ordered occurrence of each expected sequence. See `.plans/plans/go-s1-client-local/plan.md:839`, `.plans/plans/go-s1-client-local/plan.md:889`, `.plans/plans/go-s1-client-local/decisions.md:78`, `.plans/plans/go-s0-contract/draft/events.proto:3`, `.plans/plans/go-s0-contract/draft/events.proto:35`, `.plans/plans/go-s0-contract/draft/events.proto:92`, `.plans/plans/go-s3-client-link/plan.md:201` and `deploy/test/checks.sh:549`.

**Failure.** The client following the chosen default can log three durable load records but has no agreed report case to send them in. An empty or unknown event can advance the server mark without delivering any load. Keeping load exclusively in heartbeats and omitting an alternative producer yields no durable entries during the outage.

**Fix.** Adopt the agreed `Facts` producer in the local handoff and decision 71. Start it on loss of an approved stream, run it every 10 seconds through retries, and keep heartbeat load out of the outbox. Keep the client and server log contracts explicit, including the server's `machine_id` and `sequence` after committed application. Carry the exact C11 comparison into the post-build row. Assign any future numbered load report to the Contract team with both consumers named.

### architect-r3 06 — MAJOR — R6 — The clock that is safe for expiry prunes too early

**By reading.** `StoreOptions.Now` is shared by `Begin` and `Prune`. It adds all elapsed time since proof send to a server sample taken after proof reached the server. The working contract draft requires keeping receipts until at least 24 hours after expiry, and separately tells the client to use an estimate that can lead the server by up to 10 seconds for pruning. The Client Link working draft asks for `StoreOptions.PruneNow`, which the local interface lacks. See `.plans/plans/go-s1-client-local/plan.md:642`, `.plans/plans/go-s1-client-local/plan.md:735`, `.plans/plans/go-s1-client-local/plan.md:747`, `.plans/plans/go-s0-contract/draft/commands.proto:12` and `.plans/plans/go-s3-client-link/plan.md:248`.

**Failure.** An authenticated clock estimate leads the server by eight seconds. The client can measure the retention boundary as passed while the server is still within the 24 hours in which it may request a final result. `Prune` removes `done`. The next attempt receives `expired` from the watermark, even though the command ran and its final result was available moments earlier.

**Fix.** Have Contract and Client Link specify separate trusted clocks for starting commands and pruning. Use the proof-based upper estimate for admission and the welcome-receipt-based lower estimate for pruning, as the neighboring plan proposes. Refuse to prune before a validated lower estimate exists, and enforce the production retention minimum. Keep the atomic watermark for commands whose receipts can safely be removed. Test a delayed welcome one second before the server retention cutoff, with a pending retry for a finished command.

### architect-r3 07 — MAJOR — D19 — The installer still reads the old marker and the same default can resolve to two folders

**By reading.** The local plan renames the file and withdraws the service setting. The installer working draft still reads `server.json` on reinstall, passes the explicit state variable to join, and embeds it in both service formats. The shell and service can start with different values of `XDG_STATE_HOME`. The stack user-flow test reads and writes `identity.json`. See `.plans/plans/go-s1-client-local/plan.md:197`, `.plans/plans/go-s1-client-local/plan.md:202`, `.plans/plans/go-s1-client-local/plan.md:864`, `.plans/plans/go-s3-stack-client/plan.md:223`, `.plans/plans/go-s3-stack-client/plan.md:271` and `deploy/test/checks.sh:520`.

**Failure.** A reinstall of a joined machine misses the new marker, requests another token and invokes `join`. `join` refuses because the machine is already enrolled. If the installer simply drops its explicit setting, an enrollment from a shell with a custom state-home value can still differ from the service's enrollment, lock and repository path.

**Fix.** Record one managed-install resolution policy across both plans. Use the same resolved directory for the installer's join, its marker check and both service managers. Document how a foreground run selects that managed directory or limit managed installation to an explicitly agreed default which all three entry points can reproduce. Update the installer fixture and reinstall checks to use `identity.json`. Test distinct shell and service environments, as well as repeat installation with an existing marker.

### architect-r3 08 — MAJOR — C11 — Opening a damaged empty file writes into it before validation

**By reading.** The prescribed writer open follows only a type, owner and mode check. In both considered bbolt tags, `Open` initializes an existing file whose size is zero. The application only checks its buckets afterward. See `.plans/plans/go-s1-client-local/plan.md:657`, `.plans/plans/go-s1-client-local/plan.md:659` and `.plans/plans/go-s1-client-local/plan.md:683`. [bbolt v1.5.0 `Open`](https://github.com/etcd-io/bbolt/blob/v1.5.0/db.go#L246-L258), [bbolt v1.4.3 `Open`](https://github.com/etcd-io/bbolt/blob/v1.4.3/db.go#L241-L253).

**Failure.** An existing `client.db` is truncated to zero bytes. `OpenStore` accepts its mode and calls the library. The library writes metadata pages and the client then refuses the file for its missing buckets. The bytes on disk changed despite the damaged-store decision.

**Fix.** For an existing database, reject zero length before any library open. Require nonmutating validation of existing data before opening for normal writes and receipt recovery. Only the temporary first-creation path may initialize an empty database. Add an empty existing file to the byte-preservation tests, and check that both the library handle and the folder claim are released on refusal.

### architect-r3 09 — MAJOR — C11 — First and last keys do not prove a contiguous queue

**By reading.** The format requires contiguous event numbers and bounded counts and bytes. The validator is limited to the first and last event key. The server closes on an event number above its expected next number. See `.plans/plans/go-s1-client-local/plan.md:654`, `.plans/plans/go-s1-client-local/plan.md:659`, `.plans/plans/go-s1-client-local/plan.md:683` and `.plans/plans/go-s0-contract/draft/events.proto:15`.

**Failure.** A structurally valid database has keys 1 and 3, metadata claiming three events, and `next = 4`. Its endpoints satisfy the proposed open checks. The sender delivers 1 and then reaches 3. The server refuses the gap. The same store opens and stalls again after restart. A wrong persisted byte count can independently defeat the buffer limit.

**Fix.** Validate the bounded event bucket in one read transaction. Check every key's encoding and its increment from the previous key, and compare the observed count and byte total with the persisted values. Validate the numerical limits and the empty-queue case as well. Return `ErrStoreDamaged` before writing on a mismatch. Add interior-hole, wrong-counter and no-pending-events ceiling cases to the frozen tests.

### architect-r3 10 — MAJOR — C11 — A producer clock can suppress newer reports and disagrees with the contract

**By reading.** Rules 4 and 5 require the server to select the latest producer time after an ambiguous append. The event contract expressly uses `Event.happened_at` for display and uses event numbers for order. The Server Machines brief gives incoming events to the server in that order. See `.plans/plans/go-s1-client-local/plan.md:835`, `.plans/plans/go-s1-client-local/decisions.md:68`, `.plans/plans/go-s0-contract/draft/events.proto:15`, `.plans/plans/go-s0-contract/draft/events.proto:35` and `.plans/plans/go-s3-server-machines/plan.md:51`.

**Failure.** A machine records its inventory with a clock one day ahead. It corrects the clock and captures an updated inventory after a restart. The client sends the observations in the correct sequence. A server that follows the local handoff retains the older inventory for a day. The contract permits the server to apply the later sequence immediately.

**Fix.** Make the ordered stream the authority for report replacement. Serialize report publication so an older pending observation cannot be appended after its replacement. After an uncertain append, drain the recovered prefix and create a fresh observation. The server and its integration test should verify that the newest numbered snapshot wins even across a machine clock correction. Leave `happened_at` for display.

### architect-r3 11 — MAJOR — R6 — An ambiguous finish can already have stored the final result

**By reading.** The generic store failure rule allows a transaction to land before returning an error. The receipt rule says that a failed `Finish` becomes `interrupted` and that the server gets that outcome. Recovery only changes valid receipts still in `started`. The frozen tests cover both outcomes for append and begin, with no equivalent test for finish. See `.plans/plans/go-s1-client-local/plan.md:672`, `.plans/plans/go-s1-client-local/plan.md:685`, `.plans/plans/go-s1-client-local/plan.md:744` and `.plans/plans/go-s1-client-local/plan.md:772`.

**Failure.** A command completes and its `done` result lands, but `Finish` reports an error. A builder who follows the blanket `interrupted` promise can replace or misreport the stored terminal result. The server may show that completed work as interrupted.

**Fix.** On an uncertain `Finish`, send no result and stop the run. On reopen, keep a valid terminal receipt exactly as stored, and convert a surviving `started` receipt to `interrupted`. Return the resulting state under the next delivered attempt. Add fault tests for both disk outcomes using a completed action with a durable effect counter. Include a case where an interrupted receipt is reopened twice and remains valid without an authenticated clock.

### architect-r3 12 — MAJOR — C7 — The new scanner rules reject ordinary links and disagree with frozen diagnostics

**Confirmed live.** A regular Linux symbolic-link fixture reports mode `0777`. **By reading.** The new rule applies group and other write-bit checks to link entries. Filtering directories before enumeration also prevents an untrusted directory from producing the `ProblemUntrusted` that two tests require. The Linux access-list test may add a group-class write bit when it sets the list, so the test can pass via the old mode check. See `.plans/plans/go-s1-client-local/plan.md:536`, `.plans/plans/go-s1-client-local/plan.md:545`, `.plans/plans/go-s1-client-local/plan.md:561` and `.plans/plans/go-s1-client-local/plan.md:565`.

**Failure.** A normal link from a trusted program directory to a trusted executable is refused by its own write bits. A harness in a wholly filtered directory is reported as missing by candidate enumeration, while the frozen tests expect an untrusted-location error. A developer can also remove the Linux access-list check without making the proposed test fail.

**Fix.** Define separate checks for a symbolic link entry and its target. Test a trusted link that succeeds and an unsafe intermediate target that refuses. Define whether refused search entries count in diagnostics, with one consistent result and matching tests. For the Linux access-list test, use a named entry that leaves all group and other write bits clear, and make the mutation of removing the list check turn that test red. Name the fixture-tool exception to the test acceptance rule or construct the fixture without a tool.

### Risk register

The evidence status refers to the plan defect. Runtime behavior of unbuilt code is covered by the checks below. The three factors are severity, fallout and probability, respectively. Required build gates have no numerical score.

| Finding and label | Category and responsible builder | Evidence status | Factors and score |
|---|---|---|---|
| 03, BLOCKER | Security isolation, Client | Confirmed by reading. | 8 for access to durable client state, 7 for irreversible access, 4 for a host with a permissive helper or access list. The score is 224, fix before release. |
| 01, BLOCKER | Security integrity and design, Client, Contract and Server | Confirmed by reading. | 8 for duplicate effects, 7 for effects that may be irreversible, 4 for a stale restore. The score is 224, fix before release. |
| 05, MAJOR | Design, Client Link and Contract | Confirmed by reading. | 6 for a failed outage flow, 3 for repair by updating the client, 8 for a normal outage. The score is 144, schedule. The required wave 1 check is a build gate. |
| 02, BLOCKER | Security integrity, Client Link | Confirmed by reading. | 8 for lost events, 7 for permanent removal, 2 for a faulty acknowledgement arriving after an incomplete batch. The score is 112, schedule. |
| 07, MAJOR | Design, Stack and Client | Confirmed by reading. | 6 for a failed managed install, 3 for manual recovery, 6 for a reinstall or custom state home. The score is 108, schedule. |
| 12, MAJOR | Design and test evidence, Client | Confirmed by reading and by the Linux mode probe. | 6 for missing harness inventory, 3 for manual recovery, 6 for a linked installation. The score is 108, schedule. |
| 09, MAJOR | Security integrity, Client | Confirmed by reading. | 6 for a stuck replay, 5 for manual recovery, 2 for a damaged queue that retains plausible endpoints. The score is 60, schedule. |
| 10, MAJOR | Bug and design, Client Link and Server | Confirmed by reading. | 4 for stale inventory until the clock catches up, 3 for recovery, 4 for a clock correction. The score is 48, record. |
| 06, MAJOR | Bug and design, Client Link | Confirmed by reading. | 6 for an incorrect final outcome, 3 for recovery by a new command, 2 for a retry in the retention gap. The score is 36, record. |
| 11, MAJOR | Design, Client | Confirmed by reading. | 6 for the wrong final outcome, 3 for recovery, 2 for an uncertain finish followed by retry. The score is 36, record. |
| 08, MAJOR | Security integrity, Client | Confirmed by library source. | 4 for altered repair evidence, 3 for local recovery, 2 for an already truncated database. The score is 24, record. |
| 04, BLOCKER | Design, Client planner | Confirmed by reading. | This is the per-item build gate. |

## Answers to the seven round-three questions

### 1. What each registered patch fixes

The numbers follow the list in `.plans/plans/go-s1-client-local/attack/round-2-findings.md:84`. The group letters refer to the outcomes in that register.

| Patch | Result |
|---|---|
| 1. Overview | The shared store and restart owner are clear. The once-only promises need the acknowledgement and restore fixes in findings 01 and 02. |
| 2. Key decisions | Append now binds the returned number to the durable event. The sent boundary and store version selection remain open in findings 01 and 02 and lens question 8. |
| 3. Workstreams | The store replaces removal of individual files after a mark update, closing group C's original failure. The new shared history needs the restore policy in finding 01. |
| 4. Rollout | Items 2 and 3 have an ordered dependency. The dependency of item 6 on receipt recovery creates the reverse edge reported in finding 04. |
| 5. Owner choices | Deferring repair and group write permission preserves the current defaults. A missing store can bypass the refuse-and-preserve policy, and macOS still needs an effective permission check. |
| 6. Rules and test helpers | The package-specific helper names and file ownership prevent redeclarations. Module fetching and the separate commit gate still need the explicit steps in lens questions 6 and 8. |
| 7. State layout | The new marker matches the stack test, and the single store eliminates the old file names. The installer is an omitted reader, and restoration now affects both durable records. |
| 8. Item 2 | Full-chain sync and a claim derived from the locked folder close groups B and M as specified. Folder-sync portability and failed-open cleanup need built-code checks. |
| 9. State acceptance | The tests name the reopened ancestor and a locked-folder path. The acceptance claims follow the new public surface. |
| 10. Item 3 | The key must be synced before any enrollment request, and the identity marker must be durable before success. The expired-token procedure exists in the handoff. A separate test after marker placement is still needed. |
| 11. Item 5 | Group N's deadline has a start marker and a remaining-budget test. Group E's permission and lookup failure survives through helper targets and macOS access lists, and ordinary links are rejected. |
| 12. Scanner tasks and acceptance | The new test selection and trust rules are named. The outcomes for refused directories and the optional `setfacl` tool contradict the acceptance text, as described in finding 12. |
| 13. Store and receipts | Groups C and G no longer rely on multi-file first publication or mark cleanup. Group D's deletion and watermark are atomic. Groups A, F, J, K, O and P are addressed only in part, with findings 01, 02, 06, 10 and 11. Group Q's replay writes nothing, provided the implementation actually uses a read path. Group S adds the equality case at the sent mark. |
| 14. Store tasks and checks | New tests give the create, acknowledgement and prune operations owners. `NextSequence` exists as a library example, while this format uses `meta.next`. The acceptance assertions do not catch findings 02, 08, 09 or 11. |
| 15. Client Link handoff | The key and command attempt mappings are present. The wire mark, real send ceiling, report kind, replacement order, pruning clock and three state-directory entry points need the fixes in findings 01, 02, 05, 06, 07 and 10. |
| 16. Post-build checks | The new scenarios are useful. The macOS access-list check presently accepts the trust breach as an observation. The C11 row needs the actual exact sequence comparison, and the join check needs the marker-sync boundary. |
| 17. Traceability | The outbox verification row points at item 6, but the two brief points for buffer limits still point at item 7 in `.plans/plans/go-s1-client-local/plan.md:913`. Put those brief points under item 6. |
| 18. Sizing | The arithmetic is 28 normal seats and four break reviewers. The listed dependency for item 6 omits receipt recovery. The main unresolved work is separating the two mergeable units. |
| 19. Attack log | It records the repeated C11 failure and the mechanism replacement. Its dependency owner and the request about the `Welcome` comment need to reflect the epic and the working Contract draft. |
| 20. Decisions 55 to 71 | The new store, full-chain sync, folder claim and marker name have readable decisions. The conflicting or incomplete rules are 57, 59, 60, 61, 63, 67, 68, 69 and 71. The numbered findings and lens question 8 give their corrections. |
| 21. Scouting | The file, log-field and library symbols have evidence. The older scouting table still calls for `eventfile.go` and item 8. The new source rows establish tagged behavior and a locally recorded fallback build, while the preferred version still needs an actual four-target build. |

### 2. What the unreviewed mechanisms introduce

The shared store, `client.db` layout, epoch removal, failed-commit restart, Client Link fold and sizing rows are covered by findings 01, 02, 04, 06 and 08 through 11. The trusted-launch mechanism is covered by findings 03 and 12. The inherited marker on `repositories/` was checked where the new installer rule touches it. Distinct folders give distinct default clones, while replacing a state folder also replaces its default clone path. Document retention before wave 2 creates any clones. See `.plans/plans/go-s1-client-local/plan.md:199` and `.plans/plans/go-s1-client-local/plan.md:243`.

### 3. Whether any rejection lacks evidence

Nothing found. All 32 dispositions were counted in the register. None is marked rejected. The group-write question is deferred to the owner under group R. See `.plans/plans/go-s1-client-local/attack/round-2-findings.md:125`.

### 4. Who writes and reads the changed names

| Name | Writers and readers |
|---|---|
| `state.Paths`, `Folder.Claim`, `IdentityFile`, `StoreFile`, `TempPrefix` | Item 2 owns the definitions and lock. Items 3 and 6 use the returned paths. Item 6 claims and creates the store. Client Link joins and runs through the locked folder. State cleanup removes temporary files written by items 2 and 6. Add the installer as an `IdentityFile` existence reader. The stack C10 fixture also reads both public-key fields in `identity.json` and rewrites `serverPublicKey`. |
| `client.db`, `meta`, `events`, `receipts`, `pruned_through` | Item 6 creates the file, declares the format version and owns the event data and counters. Item 7 reads and writes receipt values and the pruning watermark. The Client Link run loop opens one store. Its producers and replay reader use the public store methods. It never reads the buckets directly. Item 7 needs ownership of the receipt recovery edit. |
| `StoreOptions`, `Acked`, `MarkSent`, `Read`, `Ack`, `Reconcile`, `ErrStoreFailed` | Items 6 and 7 own the APIs. Client Link produces events, sends and acknowledges them, handles the command clock and closes the store. The session owner needs explicit ownership of the live send ceiling. The receipt clock handoff lacks the `PruneNow` field requested by its writer. |
| `Welcome.last_event_number` (field 3), `EventAck.through` (field 1), removed `Hello.acked_event_number` (former field 4) | The Contract team owns wire names and numbers. The Server Machines handshake writes the welcome mark. Its event transaction writes the durable mark and its stream sends acknowledgements. Client Link reads both. Under the working ruling there is no `Hello` mark writer or reader. |
| `Event.number` (field 1), `happened_at` (field 2), `Facts.workspace` (field 5) | Client Link attaches the stored sequence and producer time and maps the resolved setting into `workspace`. Server Machines validates and applies the reports. The website receives the resulting machine state through the server's website contract. Stack check C11 consumes the client and server log sequences. |
| `LatestAttempt`, `ReceiptUnsupported`, `Welcome.server_time` (field 4) | Server Machines writes the time sample and delivers each durable attempt. Item 7 stores attempts and terminal outcomes. Client Link supplies trusted clock estimates and maps local states to `CommandResult.outcome` (field 3). Server Machines reads the current attempt and final outcome. The draft constants give unsupported 4 and interrupted 5. |
| `trustedFolder`, `trustedProgram`, `hasAccessList` | Item 5 owns these private names and uses them through `Scan`. Client Link consumes `Found.Harness` and the optional version. The installer supplies the program search path. Wave 2 does not yet call the launcher trust code. |

### 5. The interacting rules and their outcomes

In the first table, `A` is the local acknowledged mark, `S` the durable attempted-send mark, and `L` the live stream's completed-send ceiling. `W` is the authenticated welcome mark.

| Input and state | As written | Required outcome |
|---|---|---|
| Current-stream acknowledgement at or below `A`. | No change. | No change. |
| Current-stream acknowledgement above `A` and at or below `L`. | Remove the prefix if it is at or below `S`. | Remove the confirmed prefix. |
| Current-stream acknowledgement above `L` and at or below `S`. | The local rules remove events. | Refuse before reaching the store. |
| Acknowledgement from an old stream, or above `S`. | Ignore the old stream or refuse an excessive number. | Preserve the queue. |
| `W` below `A`. | Refuse, while the local plan expects the server to have raised `W`. | Preserve local history and diagnose a server rollback under the server-only ruling. |
| `W` from `A` through `S` on the current database. | Remove the confirmed prefix. | Remove the confirmed prefix. |
| `W` above `S` on an old snapshot. | Reassign its remaining numbers. | Same-identity restore requires a separate recovery policy. |

| Receipt and clock state | Outcome |
|---|---|
| No receipt, and expiry is at or below `pruned_through`. | Return expired even after the local estimate moves backward. |
| No receipt, expiry is above the watermark, and the admission clock is before expiry. | Commit `started` before running. |
| A command is running and a newer attempt arrives. | Raise `LatestAttempt`. Reply under that attempt if the current stream supplied it. |
| A terminal receipt exists. | Replay its state under the arriving attempt. |
| A finish error leaves `started` on disk. | Reopen converts it to interrupted. |
| A finish error leaves a valid terminal receipt on disk. | Reopen preserves the result. The blanket interrupted rule contradicts this case. |
| A terminal record has passed the estimated retention boundary but can still be requested by the server. | Keep the original result until the lower estimate passes the required retention boundary. |

### 6. Which earlier blockers or majors remain open

Groups A and E remain open in findings 02 and 03. Groups F and O remain open across the restore and wire-policy handoff in finding 01. Group I is served by periodic facts in the Client Link draft, while the local patch still chooses an unavailable report. Group J has a restart owner, while report freshness remains open. Group K has the attempt and outcome mapping, while its finish error has conflicting outcomes. Group L remains open across the installer. The exact earlier failures in B, C, D, G, H and M have a specified repair. Group P removes the persisted wall-clock high-water mark, while finding 06 exposes a distinct retention error.

### 7. Which architect questions the patches touch

They touch all eight. The answers below cover the current anchors, readers and writers, placement, design rules, upstream and downstream agreements, merge order, wave scope and dependency pin.

## Answers to the eight architect questions

1. **Existing anchors.** Confirmed live at `b68adba0`, the root module and Go client are still planned targets. The installed default Go is 1.23.3 and Go 1.26.0 is cached. By reading, `recordManagedInstallation`, `WorkflowStore`, the harness `inspect` function, and the floorplan's settings and event entries exist. The amended C10 and C11 script claims resolve at `deploy/test/checks.sh:520` and `deploy/test/checks.sh:533`. The real C11 comparison ends at line 586. The Go source has Linux `Getxattr` and macOS directory sync uses the file-sync path. `setfacl` was absent here. A regular symbolic link's mode was checked locally. `Open` and `NextSequence` resolve on the preferred library tag. [bbolt v1.5.0 `Open`](https://github.com/etcd-io/bbolt/blob/v1.5.0/db.go#L178), [bbolt v1.5.0 `NextSequence`](https://github.com/etcd-io/bbolt/blob/v1.5.0/bucket.go#L524).
2. **Public names.** The writer and reader table answers the changed interfaces, files, generated fields and logs. The missing parties are the installer and C10 fixture as marker readers, the Client Link session owner for the live ceiling, and the clock writer's absent pruning option. The Server Machines event transaction and its log reader are essential to C11.
3. **Placement.** The lock, store and receipt recovery belong to the Client team under `src/client/`. Contract changes belong to Sprint 0 or return to phase 3 after the freeze. The installer belongs under `deploy/`. The orchestrator owns module edits after Sprint 0, even though the local plan currently asks the Contract team to add the dependency. See `.plans/epics/go-rewrite/epic.md:114` and `.plans/epics/go-rewrite/epic.md:127`.
4. **Design rules.** R1 is kept through client-only packages. R2 is broken at the handoff where rule 3 names a removed field and rule 8 names an undefined report. R3, R4 and R5 do not apply to this local database, since they govern server tables, server writes and `Watch`. R6 is broken in the restore and retention cases. R7 and R8 govern the server and its website error boundary, so they do not apply here. R9 is broken at launcher trust. R10 is kept in scope through real local stores and processes, with the real one-process link test assigned to Sprint 3. R11 does not apply because no code is copied. See `docs/specs/2026-11-25-go-rewrite-foundation-design.md:358`.
5. **Neighboring plans.** Contract supplies generation and the one server mark. Client Link consumes the disk layout, queue, reports and command receipts. Server Machines owns the authoritative mark, timestamp, command attempts and event application log. Stack owns the managed state environment and marker check. Findings 01, 02, 05, 06, 07 and 10 give the mismatches. Agreement point 5 and issue 1634 remain with the Sprint 3 application entry points and combined test. Issues 1632 and 1633 affect Sprint 3 protocol and transport. Issue 1635 is reflected in low, medium and high sizing. See `.plans/epics/go-rewrite/epic.md:172` and `docs/issues.md:7501`.
6. **Intermediate merges.** Items 1, 2, 4 and 5 can compile separately once the root module exists and item 5's pattern is clearly a reference. Item 3 needs item 2. Item 6 needs item 2, a prefetched pinned module and a complete first-open path that does not call an absent item 7. Item 7 adds receipt recovery once it owns that edit. No Go test is executable at this commit because the root module and client code are still future work.
7. **Wave 2.** The opaque local store and reserved clone path leave room for future reports and per-agent work. No clone, agent launcher, secret store or terminal stream is added. Same-identity backup recovery and a new load event exceed what the wave 1 proof needs. Current reports can exercise C11.
8. **Dependency and tools.** `go.etcd.io/bbolt` v1.5.0 exists, uses Go 1.25.0 and requires `golang.org/x/sys` v0.45.0. The fallback v1.4.3 exists, uses Go 1.23 and requires `golang.org/x/sys` v0.29.0. Both minimum versions are compatible by reading with the planned Go 1.26.0. Tagged source shows Linux `fdatasync` and file sync on macOS. The version 1.5.0 build could not be reproduced in this session because outbound module access was blocked. Choose one pin with the orchestrator, resolve the root module's indirect versions, and fetch them before the network-off four-target gate. [v1.5.0 module](https://github.com/etcd-io/bbolt/blob/v1.5.0/go.mod), [v1.4.3 module](https://github.com/etcd-io/bbolt/blob/v1.4.3/go.mod), [Linux sync](https://github.com/etcd-io/bbolt/blob/v1.5.0/bolt_linux.go), [macOS sync path](https://github.com/etcd-io/bbolt/blob/v1.5.0/boltsync_unix.go).

## Post-build checks

| Scenario | Expected result and evidence |
|---|---|
| The Client team kills a process after a directory entry appears and before the final sync, on Linux and macOS. | A second open establishes the full path and database name's durability before the first payload is accepted. In a real power-loss run with no disk fault, an ordinary crash leaves the database openable and every returned sequence accounted for. Record the sync and reopen results. |
| The Client team pauses a failing commit and queues two more writes, then retries `OpenStore` after a folder-sync failure. | No queued write can commit after the ambiguous failure. The failed open releases its claim and database handle. Record both injected disk outcomes and the subsequent open result. |
| The Client Link and Server teams interrupt three queued facts sends by stopping the running server. | The stack check observes every outage sequence once and in ascending order, and then a later sequence. Record both logs and the result of `deploy/test/checks.sh --only C11`. |
| The Client Link team fails directory sync after the enrollment marker has been placed. | No successful join is reported before sync. A later run validates and syncs the persisted marker under the lock before connecting. The server shows one enrollment. Record the transaction count and directory-sync trace. |
| The Client team runs the finished trust checks against supported launchers on Linux and macOS. | A permitted symbolic link and its benign helper work. Refused targets and another user's effective write grant stop execution. Record the decision for each fixture and the actual version lines for supported installations. |

## Grade

4 out of 10. The review found 4 blockers, 8 majors and 0 minors.
