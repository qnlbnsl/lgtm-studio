REVISE

The patched queue still permanently deletes events that an authenticated server acknowledges within the local sequence range even when the sender never sent them.

## Findings

The review compares `fb992dc9` with `9ec34833`. The client plan, decisions and scouting at `f86d7d71` match the reviewed snapshot. The Contract team's concurrent working drafts are cited as drafts where they differ from the snapshot.

### architect-r2 1 — BLOCKER — C11 — An allocated number is mistaken for a sent event

**By reading.** Both deletion paths use `Next()` as their upper bound. The handoff gives the sender no way to record the highest number it sent. The check for a new outbox tries 0 and 500 with two pending events and skips 1 and 2. See `.plans/plans/go-s1-client-local/plan.md:695`, `.plans/plans/go-s1-client-local/plan.md:698`, `.plans/plans/go-s1-client-local/plan.md:699`, `.plans/plans/go-s1-client-local/plan.md:710`, and `.plans/plans/go-s1-client-local/plan.md:867`.

**Failure.** The client appends events 1 and 2 while offline. Its first greeting receives a wrong mark of 2 for this epoch. `Reconcile(2)` deletes both. The same deletion is possible when the live sender has sent only event 1 and receives `Ack(2)`.

**Fix.** Give the ordered sender an explicit sent boundary for live acknowledgements and a durable per-epoch boundary for reconnect reconciliation. Define the ordering between recording eligibility, sending and committing the server's acknowledgement, including an ambiguous send. A fresh epoch must accept only zero on first contact. Test an authenticated reply inside the pending range with a smaller send boundary, and the retry of a committed event whose acknowledgement was lost.

### architect-r2 2 — BLOCKER — C11 — Reopening can leave a new ancestor uncommitted

**By reading.** The initial open syncs parents as it creates missing ancestors. A retry syncs only the state folder's immediate parent, the state folder and its two store directories. The retry portion of the new test expects that shorter sequence. See `.plans/plans/go-s1-client-local/plan.md:295`, `.plans/plans/go-s1-client-local/plan.md:299`, and `.plans/plans/go-s1-client-local/plan.md:315`.

**Failure.** The client creates `a` under an existing directory and dies before syncing the existing directory. The operating system still sees `a`. A second open creates `a/b/state`, syncs `a` and its descendants, and accepts a key and events. A later power loss can lose the directory entry for `a` and the entire state tree beneath it.

**Fix.** On every successful open, make each path entry from a known durable ancestor to the state folder durable, including entries found on a retry. Define that anchor or sync the full checked chain. Require the retry test to observe the parent whose prior sync failed, and add a stop between creating a non-leaf ancestor and syncing its parent.

### architect-r2 3 — BLOCKER — C11 — Recovery unlinks events before syncing the recovered mark

**By reading.** A failed mark sync puts the queue in a failed state. The calling contract allows closing and reopening the queue on the same held folder. Recovery reads the visible mark, deletes the acknowledged files and syncs only at the end. The new mark-error test exercises an in-process reopen with working syncs through completion. See `.plans/plans/go-s1-client-local/plan.md:636`, `.plans/plans/go-s1-client-local/plan.md:639`, `.plans/plans/go-s1-client-local/plan.md:696`, and `.plans/plans/go-s1-client-local/plan.md:716`.

**Failure.** The mark changes from 0 to 3 and its directory sync fails. A same-process reopen sees 3 and unlinks events 1 through 3. A power loss before the final sync can leave the old mark with missing event files. Even with server effects committed, the client can no longer reopen the queue and deliver event 4.

**Fix.** After validating the on-disk mark, establish its durability before the first unlink. A failed sync at this barrier must leave all event files in place. Add a reopen test that fails each sync and records which deletions were attempted. If recovery also removes temporary files, describe that exception to the promise that a damaged outbox is untouched.

### architect-r2 4 — BLOCKER — R6 — Pruning can outlive the clock value that justified it

**By reading.** The clock is written when it advances by more than a minute. The patch never requires the clock write and directory sync to succeed before a prune unlinks a receipt, and it defines the failed state only for writes by `Begin` and `Finish`. The rollback test performs the operations with successful writes and no restart between pruning and retrying. The concurrent Contract draft chooses server time, so the caller's clock also needs explicit alignment. See `.plans/plans/go-s1-client-local/plan.md:765`, `.plans/plans/go-s1-client-local/plan.md:773`, `.plans/plans/go-s1-client-local/plan.md:777`, `.plans/plans/go-s1-client-local/plan.md:797`, and `.plans/plans/go-s0-contract/draft/commands.proto:14`.

**Failure.** A command ran at hour 48 and expires at hour 49. At hour 74, a prune unlinks its receipt while the new clock value is still uncertain. After a crash and clock rollback to hour 48, the old clock and missing receipt make the same command eligible to run again.

**Fix.** Persist and sync a clock value sufficient to enforce the expiry of every receipt being pruned before the first unlink. A clock read or write failure must suspend the affected decisions and preserve receipts. Specify which authenticated clock feeds the receipt store before pruning in Sprint 3. Test the clock barrier before and after placement, plus reopen after every deletion boundary with time set back.

### architect-r2 5 — BLOCKER — R9 — The scan trusts the entry point but passes untrusted search directories to its child

**By reading.** The patch filters the executable it starts. It passes every absolute search directory to the child, including directories it rejected as candidates. The observed OpenSSH rule rejects group- and world-writable canonical directories without the plan's sticky-bit exception. See `.plans/plans/go-s1-client-local/plan.md:532`, `.plans/plans/go-s1-client-local/plan.md:534`, and `.plans/plans/go-s1-client-local/plan.md:535). [OpenSSH canonical path and file checks](https://github.com/openssh/openssh-portable/blob/master/misc.c#L2220-L2309).

**Failure.** A trusted harness launcher resolves its runtime through its inherited program search path. If that list includes a directory controlled by another local user, the launcher can start a runtime from it with the client's privileges. The top-level trust check still passes.

**Fix.** Define a trusted runtime lookup policy for both search and child execution. Verify the checked canonical ancestry and any link transitions that subsequent operations still traverse. Pass only validated directories, with a rule for launchers that need an interpreter or helper. Treat access-control lists as part of the trust check on filesystems that support them. Extend the test with a trusted launcher that needs a helper and a helper directory that is deliberately refused. Describe the rule as the client's own policy.

### architect-r2 6 — MAJOR — C11 — The epoch has no agreed wire meaning or server owner

**By reading.** The approved design and committed draft keep one mark per machine. The concurrent draft likewise has a machine-wide mark, and its `Welcome` comment tells the client to jump the counter. The patch asks for an epoch in the greeting but places the exchange after proof without specifying which message carries it before `Welcome` is built. Neither receipt of a proposal nor a committed client-local API freezes the contract. See `docs/specs/2026-11-25-go-rewrite-foundation-design.md:290`, `.plans/plans/go-s1-client-local/decisions.md:48`, `.plans/plans/go-s1-client-local/plan.md:870`, `.plans/plans/go-s0-contract/draft/link.proto:191`, `.plans/plans/go-s0-contract/draft/link.proto:230`, `.plans/plans/go-s3-server-machines/plan.md:49`, and `.plans/epics/go-rewrite/epic.md:118`.

**Failure.** A machine whose old outbox reached 50 opens a new epoch with events 1 and 2. A server built to the draft returns 50. The client raises `ErrServerAhead` and retries forever. A client built to the concurrent draft can instead discard the pending reports and jump to 51.

**Fix.** Settle the design and Contract plan before contract freeze. If epochs stand, specify the field, number, validation, proof and current-stream binding, the scope of `Welcome.last_event_number` and `EventAck.through`, and how first, current, retired and missing epochs interact with a restore. Give the server handshake the read and the event transaction the atomic read/write of an explicitly owned PostgreSQL checkpoint. If the contract keeps one mark per machine, revise local recovery to match a single agreed reset policy. Make the client and server briefs and the contract fixtures carry the same decision.

### architect-r2 7 — MAJOR — C11 — A crash on first open permanently strands a clean outbox

**By reading.** First open places and syncs `epoch`, then writes `acked`. Recovery calls a folder with an epoch and no mark corrupt. The crash tests exercise append and acknowledgement after initialization. See `.plans/plans/go-s1-client-local/plan.md:637`, `.plans/plans/go-s1-client-local/plan.md:638`, `.plans/plans/go-s1-client-local/plan.md:652`, and `.plans/plans/go-s1-client-local/plan.md:727`.

**Failure.** First startup dies after creating the epoch and before placing the zero mark. No event ever existed. Every later startup stops with `ErrOutboxCorrupt`.

**Fix.** Publish the initial epoch and zero mark together in one atomic durable record, or add an explicit initialization state with an unambiguous way to finish it. Keep the fail-closed behavior for a genuinely missing mark in a previously used store. Test first open at every placement and sync boundary.

### architect-r2 8 — MAJOR — D7 — The join calls the server with an uncertain key and treats an uncertain marker as enrolled

**By reading.** After uncertain key placement the join merely calls `Load`, which does no sync. The `server.json` writer uses the same uncertain write primitive, but the follow-up join refuses any folder where the marker is visible. The recovery test groups every point between key creation and marker creation into one scenario. The draft also promises same-token replay only before expiry. See `.plans/plans/go-s1-client-local/plan.md:303`, `.plans/plans/go-s1-client-local/plan.md:405`, `.plans/plans/go-s1-client-local/plan.md:410`, `.plans/plans/go-s1-client-local/plan.md:883`, `.plans/plans/go-s1-client-local/plan.md:905`, and `.plans/plans/go-s0-contract/draft/link.proto:58`.

**Failure.** The key's directory sync fails. `Load` succeeds from the running filesystem, the server commits the enrollment, and power fails before another successful sync. The server retains an enrollment whose client no longer has the key. A crash after server commit with a durable key but an expired token also falls outside the promised replay.

**Fix.** Require the existing key to be durable before any `Join` call. Make the final marker durable and valid before reporting success. Under the lock, give a visible marker after an uncertain write a defined validation and sync path. Test pre-commit, post-commit before local publication, and marker-sync failures separately. State how the owner recovers a partial enrollment if the token has expired.

### architect-r2 9 — MAJOR — C11 — No wave-one producer is scheduled to buffer reports during the outage

**By reading.** The new check expects 50 offline appends. The existing user-flow script waits for three durable buffer records while the server is stopped and requires a later applied event. The Client team brief schedules facts on connection, scans on command and heartbeats while running. The drafts leave heartbeats unnumbered and schedule current reports after `Welcome`. See `.plans/plans/go-s1-client-local/plan.md:863`, `.plans/plans/go-s1-client-local/plan.md:904`, `.plans/plans/go-s3-client-link/plan.md:46`, `.plans/plans/go-s0-contract/draft/events.proto:3`, `.plans/plans/go-s0-contract/draft/events.proto:24`, and `deploy/test/checks.sh:549`.

**Failure.** A stationary approved machine loses its stream. During the outage it receives no scan commands and has no new connection. It can follow both briefs and emit no numbered reports, causing the Sprint 3 check to time out before replay begins.

**Fix.** Name the producer that can append reports offline and give it a concrete trigger, or have the agreed real-client test inject these reports through an explicit local test entry point. Define how the unchanged stack check observes the same events. Keep heartbeats outside the outbox and agree on which logs or durable state prove application and order.

### architect-r2 10 — MAJOR — C11 — An ambiguous append has no owner who can determine when to retry it

**By reading.** An ambiguous append gives the producer an error and no sequence. The handoff tells that producer to close and reopen a queue shared by all producers and the sender, then treat the report as possibly stored. The accepted duplicate rule promises an idempotent report but does not identify how the server handles a retry older than a newer report. See `.plans/plans/go-s1-client-local/plan.md:684`, `.plans/plans/go-s1-client-local/plan.md:696`, and `.plans/plans/go-s1-client-local/plan.md:865`.

**Failure.** An append places a scan report and its final sync fails. On reopen the sender can read and acknowledge the report before the producer checks the queue. Missing from the pending set then has two meanings, either delivered or never made durable. A producer that gives up can lose the latter. A producer that retries can also replay stale facts after the server applied a newer report.

**Fix.** Give the run loop ownership of queue failure and replacement and pause its sender and producers around recovery. Give the ambiguous append a stable report identity or a recoverable assigned number and define how the producer proves delivery or safely retries. If wave-one report replacement is the intended recovery, give the server a deterministic rule that suppresses stale retries. State which reports may be replaced and how the client settles the associated command.

### architect-r2 11 — MAJOR — R6 — The command handoff cannot complete a live retry or represent every terminal result

**By reading.** The patch only relabels replays of ended commands. `DecisionRunning` returns the existing receipt, and `Finish` has no attempt argument. The concurrent Contract draft requires sending a result under the newest received attempt even if that attempt arrived while the command was running. Both drafts define the `UNSUPPORTED` outcome, which the patched receipt states and `Finish` cannot express. The concurrent draft now has `INTERRUPTED` as value 5. See `.plans/plans/go-s1-client-local/plan.md:753`, `.plans/plans/go-s1-client-local/plan.md:759`, `.plans/plans/go-s1-client-local/plan.md:768`, `.plans/plans/go-s1-client-local/plan.md:778`, `.plans/plans/go-s0-contract/draft/commands.proto:47`, and `.plans/plans/go-s0-contract/draft/commands.proto:71`.

**Failure.** Attempt 1 begins a scan. The stream breaks and attempt 2 arrives while the scan still runs. It gets `DecisionRunning`. Completion under attempt 1 is ignored and a healthy replacement stream leaves the command pending. An unknown action also has no receipt outcome that round-trips through replay.

**Fix.** Assign the client command dispatcher ownership of the newest accepted attempt per running identifier and serialize registration with finishing and sending on the current stream. Define the full receipt-to-wire mapping, including `UNSUPPORTED`, and specify that a successfully stored terminal oversize failure is sent even though `Finish` returns `ErrResultTooLarge`. Test a live reconnect during an effect and an unknown action that is redelivered.

### architect-r2 12 — MAJOR — D19 — The installer setting has no matching writer for enrollment or a manual run

**By reading.** The new mechanism derives the default clone location from `stateDir`. The installer rule sets `LGTM_CLIENT_STATE_DIR` in the service file and claims a manual invocation will resolve the same directory. The Stack brief owns the installer and the service but does not assign this setting to the installer-driven join. See `.plans/plans/go-s1-client-local/plan.md:204`, `.plans/plans/go-s1-client-local/plan.md:237`, `.plans/plans/go-s1-client-local/plan.md:244`, `.plans/plans/go-s1-client-local/plan.md:891`, and `.plans/plans/go-s3-stack-client/plan.md:39`.

**Failure.** The installer supplies a custom state directory only to the service. An installer-driven or manual `join` resolves the default directory and succeeds. The service exits because the enrollment marker is missing in its custom directory. A manual invocation also resolves a different clone root from the service.

**Fix.** Choose one resolved absolute directory for installation. Pass it to the join, the service configuration and any documented manual entry point, or use the shared defaults for all three. Assign the Client report builder and Server facts reader the mapping from `Settings.RepositoriesFolder` to `Facts.workspace`. Check the wire length and the display form before enqueueing a path. Keep actual clone creation in wave 2 and declare its dependence on validated settings and on the installation's state retention policy.

## Answers to the round-two attack questions

### 1. What each registered patch fixes and what it breaks

The numbered rows follow the patch list in `.plans/plans/go-s1-client-local/attack/round-1-findings.md:102`. The letters point to its grouped outcomes.

| Patch | Review result |
|---|---|
| 1. Overview | A now distinguishes placement from durability. The generic claim about reopening still needs the recovery barriers in findings 2 to 4 and 8. |
| 2. Key decisions | C stops a counter jump but still deletes allocated events under finding 1. D preserves damaged receipts. K leaves the runtime lookup open under finding 5. Q consistently defers the Mac test to sprint 3 when needed. |
| 3. Workstreams | H and I retain the key and bound reads, with the enrollment gap in finding 8. K's safety promise is stronger than the trust check. C's statement that the server applied each removed event still fails. |
| 4. Rollout | Phase 5 on the merged client and the Sprint 3 handoff are explicit. The contract agreement in finding 6 must precede the Sprint 0 freeze. A handoff only before Sprint 3 is too late for that agreement. |
| 5. Owner choices | U gives damaged storage a concrete fail-closed behavior. The clock choice remains open in this plan and the concurrent contract draft already chooses server time. The damage-repair options that continue across a gap also need an agreed receiver rule because the concurrent event draft rejects gaps. |
| 6. Shared implementation rules | N gives distinct process helper names to items 2, 7 and 8. The filesystem and process probes remain local to the client. |
| 7. State layout | F documents the lock lifetime and the common store directory. A, H and T add useful file rules but need the barriers in findings 3 and 8. The epoch, clock and clone path add the integration work in findings 4, 6, 7 and 12. |
| 8. Item 1 | V fixes the multi-package build invocation. Z differentiates the clone roots of two state folders, but leaves the installer, manual invocation and facts report in finding 12. |
| 9. Item 2 | A prevents overwriting an existing linked record and reports uncertain placement. B's initial sync improves the write path but its retry misses ancestors. F covers a second process and a second store claim. G checks the resolved ancestry and opened lock, with the path policy qualification below. |
| 10. First workstream checks | The new tests exercise unsafe files, exclusivity and failed final syncs. The ancestor retry test blesses a shorter sync sequence under finding 2. |
| 11. Item 3 | I covers nonblocking open, type, read limit and trailing content. O moves key material behind an unexported pointer and tests value formatting. A and H still need the explicit durability rule before join. |
| 12. Second workstream checks | The named-pipe and formatting checks target the reported faults. The key-reuse claim still depends on the cases in finding 8. |
| 13. Item 4 | Q names a Mac command and numeric tolerances. X assigns both Linux fixture files to item 4. Actual Mac values remain a post-build check. |
| 14. Item 5 | J replaces the fake that exits early, defines a shared deadline and rejects nonzero exits. K and its child environment conflict under finding 5. L checks the whole first line, but a passing version still needs the Contract draft's size bound before it can be sent. |
| 15. Third workstream checks | The checks now name the fake's execution proof and Mac test. Tests for a writable candidate and its resolved target do not exercise the child's runtime lookup. The common deadline test's three-second threshold alone does not rule out separate half-second budgets. |
| 16. Item 6 | T detects a changed decimal mark and validates pending files before deleting acknowledged files. C introduces the interrupted initialization in finding 7. The order of recovery sync and deletion reopens A in finding 3. |
| 17. Item 7 | The sequence ceiling blocks wraparound, and P closes and retains the stated snapshot. F serializes reads and acknowledgements. C bounds both acknowledgement calls by allocation, so finding 1 remains. A and Y still need an ambiguous-append recovery owner. |
| 18. Item 8 | D preserves malformed receipts by pinning them. R makes oversize results terminal, and S defines local replay outcomes. E's clock ordering still permits rerunning an expired command. S leaves live retry and the full wire outcomes in finding 11. |
| 19. Fourth workstream checks | N checks crash stages and the durable mark on the normal path. The done-when rows miss an in-range unsent number, initialization crashes and a restart during pruning. The recovery fixture for corrupt data should include a temporary file if the promise is that literally nothing changes. |
| 20. Sprint 3 handoff | M gives the sender ownership of sequence order and obliges a producer to handle a full queue. The producer, queue recovery and server application duties remain incomplete under findings 9 and 10. Command, enrollment and installer readers have the gaps in findings 8, 11 and 12. |
| 21. Post-build checks | The new rows describe device and integration observations. The 50-event outage needs a producer, and the join row must identify which side of commit failed. The power-loss row must distinguish preexisting damage from damage produced by one of the planned crash windows. |
| 22. Traceability | The new source and rows point to the register, caller obligations, post-build checks, and owner decisions. Their mapping does not settle the failures behind those links. |
| 23. Execution sizing | W resolves the scope path. Treat items 2, 5, 7 and 8 as high until their durability, trust or lifecycle work is partitioned. Item 6 can remain medium if its first publication has one atomic record. |
| 24. Attack log | The register's count and stated unresolved decisions match the fold. The log counts three mechanisms while the clone default is a fourth. Its open sprint-cut note was superseded by `.plans/epics/go-rewrite/epic.md:103` at the current commit. |
| 25. Decisions 36 to 54 | The new rows describe the revised local behavior, but 37, 40, 41, 45, 46, 50, 51 and 52 need the corresponding findings resolved. Row 17's old counter jump is superseded by row 40 and should be marked as such when reconciling the handoff. |
| 26. Scouting additions | The scope, one-package build anchor and workflow pattern resolve. The Contract absence statements are accurate at the fold snapshot. The working draft now includes `INTERRUPTED = 5` and a client-reported acknowledged number. Its still-missing epoch and incompatible recovery behavior are in finding 6. |

All register groups are accounted for. A through C and E retain failure paths in findings 1 to 4 and 6 to 8. D closes the immediate prune-on-repair failure. F closes duplicate opens under the held-folder interface. G improves the resolved path check, with the trust limits in the next answer. H needs finding 8. I resolves the blocking and over-read examples at the design level. J corrects the fixture's early exit. K needs finding 5. L resolves the extra-line example. M needs findings 9 and 10. N fixes the common helper collision and strengthens the normal crash probes. O and P answer their original examples. Q reconciles Mac test timing. R and S need the dispatcher and outcome mapping in finding 11. T detects the changed-digit example and still needs finding 3. U stays with the owner. V through X fix the invocation, scope and fixture anchors. Y and Z need findings 10 and 12.

### 2. What the new mechanisms do

- **Epoch.** Local generation and same-directory storage fit the client. The Contract team must own the field and how it enters an authenticated stream. The Server team must own its checkpoint and lifetime. A split first write and a machine-wide remote mark defeat clean recovery today. See findings 6 and 7.
- **Reconciliation.** The client must keep the queue unchanged for a lower mark, a mark from the wrong epoch, or a mark above its recorded send boundary. Marks inside the allocated range are insufficient evidence. Recovery also has to make the visible cursor durable before deletion. See findings 1 and 3.
- **Trusted program.** The direct executable check omits the runtime's lookup and makes a stronger claim than its path checks prove. For state and scan paths, use the checked canonical path for subsequent operations or secure any original path components that remain in use. The sticky-directory exception and the handling of non-mode permissions need to be explicit. Test valid system and per-user installations too. See finding 5.
- **Private clone default.** Separate, valid state directories give separate defaults and wave 2 creates the clones. The installer must make enrollment, the service and manual operation resolve the same state directory for that promise to survive deployment. See finding 12.

### 3. Which rejected findings have real evidence

Nothing found. The outcome column records 48 fixed, seven fixed another way and two for the owner, with zero rejected. The rejected counter-jump in decision 41 describes a mechanism the planner discarded, rather than a finding dismissed by the register. I checked the outcome rows and totals in `.plans/plans/go-s1-client-local/attack/round-1-findings.md:13` and `.plans/plans/go-s1-client-local/attack/round-1-findings.md:149`.

### 4. Who writes and reads each changed boundary

| Boundary | Writers and readers |
|---|---|
| `StateDirVariable`, `DefaultSettings(stateDir)`, `LoadSettings(path, home, stateDir)`, and `RepositoriesDir` | Item 1 writes the config interface and item 2 writes the path constants. The owner or installer writes settings and the override. Both command entry points resolve the state directory, the run loop reads settings, the Sprint 3 report builder sends the workspace, the server stores it, and wave 2 creates and uses the clone root. The patch omits the join environment and the workspace report handoff. |
| `Folder.Claim`, `ErrStoreOpen`, `ErrParentUnsafe`, and `ErrLockUnsafe` | Item 2 implements them. Items 7 and 8 claim and release stores. The Sprint 3 join and run commands hold the folder and interpret open and close errors. The owner of shared queue replacement is missing. State tests also consume the new errors. |
| `ErrPlacedNotSynced`, `SetCrashHookForTest`, and `SetSyncFolderForTest` | Item 2 writes these. Enrollment, outbox initialization, append, mark updates, receipt and clock writes, and the Sprint 3 marker writer consume the uncertain-write result. The item 7 and 8 crash tests consume the hooks. The marker and clock consumers lack transition rules. |
| `machine.key`, `Credential`, `ErrKeyUnreadable`, and `server.json` | Item 3 writes and reads the key and provides the signing interface and errors. Sprint 3 enrollment reads the key and writes the marker after server success. The client handshake and run command read the marker. The server verifies the public key. The stack's key-mismatch fixture is an external consumer of the persisted server key and currently looks for `identity.json` at `deploy/test/checks.sh:520`. Its owner must align it with the final marker schema. |
| `epoch`, `acked`, `maxSeq`, `ErrOutboxCorrupt`, `OpenOutbox`, `Epoch`, `Reconcile`, and queue errors | Item 6 writes and reads the format and item 7 implements the queue. The Sprint 3 ordered sender and receiver use it. The Contract team defines the eventual wire fields. The server's handshake, event transaction and checkpoint migration, and the stack's C11 test, are downstream owners and consumers. Field names, bounds, active-stream scope and checkpoint lifetime remain unresolved. |
| `Found.Problem`, `Found.Path`, timeout options and version output | Item 5 writes these. The local scan caller and diagnostics consume them. The report builder puts only the name and accepted version on the wire. The server and Machines page consume the report. The installer supplies the service lookup environment and is missing from the trust handoff. |
| `receipts/clock`, `ReceiptState`, `Receipt.Note`, `Receipt.Pinned`, `OpenReceipts`, `Finish`, and receipt errors | Item 8 writes and reads these. Sprint 3 run owns the receipt lifecycle, the dispatcher reads decisions and writes results, and the server's command result handler consumes the translated outcome and attempt. The contract owns the outcome values. The clock adapter, the in-flight attempt owner and the result-error mapping are missing. |

### 5. Where rules interact

For an open, healthy queue, let `M` be the local acknowledged mark, `S` the highest number eligible to be acknowledged, and `N` the next allocated number. `S` expresses the proposed send bound from finding 1. The plan has only `M` and `N`.

| Reply or state | Patched live acknowledgement | Patched reconnect reconciliation | Result required by the patch's stated promise |
|---|---|---|---|
| Number below `M`. | It is an ignored duplicate. | It is a server-behind error. | These distinct treatments are consistent if a restore's policy is agreed. |
| Number equals `M`. | The call does nothing. | The call does nothing. | The call changes nothing. |
| Number is above `M` and at most `S`. | The call advances the mark. | The call advances the mark. | The correctly scoped server reply may advance the mark after commit. |
| Number is above `S` and below `N`. | The call advances the mark. | The call advances the mark. | The queue must preserve these events. |
| Number is at least `N`. | The call returns `ErrAckAhead`. | The call returns `ErrServerAhead`. | The queue preserves its events and next number. |
| The reply belongs to a former epoch or former stream. | The queue cannot check the scope. | The queue receives only a number. | The link must reject the reply before either call. |

| First open or recovery state | Patched result | Interaction |
|---|---|---|
| No epoch, no mark and no event. | Create two records. | The first publication can leave an intermediate state. |
| An epoch alone after the first write. | Refuse as corruption. | Finding 7 needs a retry-safe initialization that remains distinguishable from damage. |
| The epoch and mark are valid but final mark sync previously failed. | Remove the files through the visible mark, then sync. | Finding 3 requires the mark's durability ahead of deletion. |
| Required metadata is missing and event files exist. | Refuse, leaving event files. | The owner-dependent repair policy can stay fail-closed. |

| Command state and arriving copy | Patched decision | Caller obligation |
|---|---|---|
| No receipt and the clock is before expiry. | Persist `started` and run. | Establish the clock before the comparison. |
| No receipt and the clock is at or after expiry. | Persist `expired` and replay. | Send the terminal result for the received attempt. |
| Running receipt and a newer attempt arrives. | Return `DecisionRunning` and send nothing now. | Register the newer attempt before the current execution finishes. |
| Ended receipt and any accepted attempt. | Replay and extend the expiry if it changed. | Send the stored outcome on the newest attempt. A changed expiry is forbidden by the Sprint 3 handoff, so explicitly choose how to handle noncompliant input. |
| Receipt pruned but its new clock value did not survive. | A rolled-back clock can allow execution. | Make the pruning clock durable before removing the receipt. |

### 6. Which round-one blockers and majors are still open

The original overwrite is closed on the regular append path. Uncertain acknowledgement recovery still violates its durability ordering. The ancestor durability, unsent-event deletion and trusted executable groups remain open in findings 1 to 3 and 5. The rollback case in the clock group remains open in finding 4. The join, producer and result handoffs still have the major failures in findings 8 to 11. The two explicitly owner-dependent decisions remain so until the contract and design agree.

### 7. Which architect questions the patches touch

All eight are touched. The local tools and source anchors support the mechanics that were checked. The new interfaces and external owners, placement, design rules, neighboring briefs, merge order, wave boundary and toolchain are assessed below.

## Answers to the architect lens

1. **Existing code and tools.** Confirmed live that the client scope exists, there is no root Go module or client/server Go package in either reviewed commit, and `deploy/test/checks.sh` passes shell syntax checking. The anchored stack build is in `deploy/test/checks.sh:196`, and its test invocations set the X Desktop Group state variable at `deploy/test/checks.sh:219` and `deploy/test/checks.sh:233`. The workflow pattern syncs its temporary file before link or rename at `packages/engine/src/workflows/store.ts:141`. The harness and blueprint anchors resolve at `packages/engine/src/harnesses/programs.ts:124`, `packages/engine/src/harnesses/programs.ts:199`, `packages/engine/src/harnesses/recipes.ts:24`, `floorplan/plan.json:1239` and `floorplan/plan.json:1267`. The scope is limited to client sources in `.plans/scopes/client/backend/scope.json:7`. See finding 5 for the mismatch with the cited OpenSSH policy.
2. **Public names and formats.** The boundary table above assigns changed names to both sides. The largest missing readers and writers are the current-stream transport, the server checkpoint migration and event transaction, the command dispatcher, the installer-driven join and the stack check of the saved key.
3. **Placement.** The local queue, receipt clock file, key, scanner and clone default fit under `src/client/`. The wire fields belong to Contract, committed checkpoints and business application to Server, and launch environment and smoke tests to Stack. No client item should make changes in those folders. The team and agreement tables are in `.plans/epics/go-rewrite/epic.md:120` and `.plans/epics/go-rewrite/epic.md:165`.
4. **Rules R1 to R11.** R1 is kept by the local package boundaries. R2 is kept for standalone local items and is unresolved for the proposed epoch until the wire contract is reviewed and frozen. R3, R4 and R5 do not apply to local files, and the server checkpoint will need R4. R6 is broken by the receipt pruning and live result cases. R7 and R8 do not apply to these client-local stores and errors. R9 is broken by the scanner's inherited runtime lookup. R10 is kept by the local test design, with the real in-process link test entry points still open in issue 1634 for Sprint 3. R11 does not apply because the cited repository files are patterns and the patch says it copies no third-party code. The source is `docs/specs/2026-11-25-go-rewrite-foundation-design.md:358`.
5. **Neighboring plans.** Contract supplies the module and the eventual wire format. Client Link consumes the local interfaces, and Server Machines owns join replay, checkpoint application and result acceptance. Stack Client owns the installer environment and deploy checks. Their mismatches are findings 6 and 8 to 12. The stack check's existing saved-key fixture is an additional consumer named in the table above. Agreements 1, 4 and 5 in the epic are the relevant handoffs.
6. **Intermediate commits and full checks.** Items 1, 2, 4 and 5 can stay independent after the module if the pattern in item 5 remains a reference. Item 6 must merge before item 7, and items 3, 6 and 8 must wait for item 2. Item 6's tests should use its own recovery interface. If the recovery change uses the removal hook currently defined in item 7, move that hook to item 6 so its earlier commit builds alone. Regenerate and check the contract in Sprint 0 before starting a Sprint 3 builder against new fields. The report reviews a plan and no client package at these commits can be compiled yet.
7. **Wave 2.** The opaque payload and private clone default leave space for later agents without building them. Wave 2 still creates clones and owns their lifecycle. The epoch and stream-state expansion need justification for wave 1 if the owner chooses fail-closed recovery of a lost outbox instead. Neither approach should add wave-two report types or agent startup here.
8. **Tools and versions.** Confirmed live that the system Go is 1.23.3 and cached Go 1.26.0 runs. A throwaway two-package build with version-control stamping disabled reproduced the old multi-package output error and passed the two replacement build forms. The previous fake with an empty search path exited 127 before sleeping. By reading Go 1.26.0 source, the plan's symbols resolve in `src/syscall/syscall_bsd.go:434`, `src/syscall/syscall_linux.go:518`, `src/syscall/zsyscall_darwin_arm64.go:1010`, `src/testing/testing.go:1862`, `src/os/exec/exec.go:125`, and `src/internal/poll/fd_fsync_darwin.go:16`. This patch adds no external module or image. The built client and real Mac measurements are pending implementation.

## Post-build checks

| Check | Scenario and expected result | Evidence and owner |
|---|---|---|
| Durability of the state tree | Interrupt creation of a nested state directory and retry. After a power cut on a supported filesystem, the reopened client still has each key, event and receipt that it accepted as durable. | Keep the sync trace and post-restart state inventory. The Client team runs it on Linux and macOS. |
| Epoch and reconnection | Run the real client and server, lose the reply after applying a report, then reconnect. The server records one effect and the client removes only events eligible for acknowledgement on the current epoch. Simulate a lower durable server mark and a replaced stream using the agreed policy. | Keep the server transaction results and client pending numbers. The Server and Client teams run it together. |
| Join recovery | Stop the client immediately before and after the server enrolls it, and during final marker publication. An unexpired retry keeps the same key and leaves one machine. A later retry follows the agreed expiry recovery path. | Keep the client file inventory and count of server enrollments. The Client and Server teams own the test. |
| Command retry and pruning | Break the connection during a visible, long-running command and send a newer attempt. The effect happens once and the current stream receives the terminal result under the newest attempt. Repeat the clock rollback after a successful prune across restart. The old identifier never runs. | Keep the effect count, terminal result and on-disk receipt clock. The Client and Server teams own the test. |
| Discovery and installation | Run from the installed service environment, probe the four real harness versions and compare macOS load values with the supported system tool. The harnesses and load appear as expected, and enrollment, service startup and the documented manual invocation use the selected state directory. | Keep the fixture observations and the facts visible on the Machines page. The Client and Stack teams own the test. |

## Grade

5/10. There are 5 blockers, 7 majors and 0 minors.
