REVISE

The client can permanently delete an event it has never sent because both acknowledgement paths use the highest number allocated on disk as their limit.

## Findings that remain in the round 2 patches

### logic-r2 01 — BLOCKER — C11

**Failure.** Event 6 was sent, event 7 was only queued, and `Next()` is 8. A mistaken acknowledgement of 7 passes `Ack` and removes both. A reconnect with a mistaken server cursor of 7 does the same through `Reconcile`. The two new tests reject numbers beyond the queue and explicitly allow acknowledging numbers for which the test records no send.

**Fix.** Have the authenticated sender own a cumulative send bound for the current stream. Accept a live acknowledgement only from that stream and within that bound. On reconnect, require durable evidence of the sent prefix for the same epoch before deleting from the server cursor, or retain and resend the pending prefix and advance the mark only on a live acknowledgement within its send bound. If using the latter approach, specify how the server acknowledges duplicate events without returning a cursor ahead of this stream's sent prefix. Add one queued-but-unsent event to both tests and a lost-acknowledgement replay test.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:695`, `.plans/plans/go-s1-client-local/plan.md:698`, `.plans/plans/go-s1-client-local/plan.md:699`, `.plans/plans/go-s1-client-local/plan.md:710`, `.plans/plans/go-s1-client-local/plan.md:711`, `.plans/plans/go-s1-client-local/plan.md:826`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:290`.

### logic-r2 02 — BLOCKER — C11

**Failure.** An append fails after placement. Reopen can find the event, or a power loss can leave it absent. The producer receives neither its candidate sequence nor a durable event identity and is asked only to treat it as possibly stored. Dropping the producer's copy loses the event in the absent case. Appending it again in the present case can add a second numbered copy and send it after a later report. Equal payloads can also be distinct events, so a payload comparison can discard legitimate work.

**Fix.** Put recovery under the outbox owner and give the producer a durable event identity or a reservation handle that resolves the specific uncertain append as committed or absent. Keep new appends and delivery behind that resolution. Retry the same identity only when necessary. Test both disk outcomes with an identical payload from a different event, and test that later work cannot overtake recovery. Define separately which report types the producer may supersede.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:684`, `.plans/plans/go-s1-client-local/plan.md:696`, `.plans/plans/go-s1-client-local/plan.md:715`, `.plans/plans/go-s1-client-local/plan.md:865`, `.plans/plans/go-s1-client-local/plan.md:868`, `.plans/plans/go-s1-client-local/plan.md:869`, `.plans/plans/go-s1-client-local/attack/round-1-logic.md:15`.

### logic-r2 03 — BLOCKER — C11

**Failure.** A new state path is `<tmp>/a/b/state`. The first open syncs the parent of `a`, then creates `b` and fails to sync `a`. A second open sees `b` already present, creates `state`, and syncs `b`, `state` and its subfolders. It can report success without ever syncing the entry for `b` in `a`. A subsequent power loss can take the entire state tree, including events whose appends succeeded.

**Fix.** On a successful open, sync every parent that could contain an entry left by an interrupted creation. Sync the resolved ancestor chain on reopen, or carry durable proof of which ancestor entries were synced. Extend the existing injected failure test to require the second open to sync `a` before returning. Keep the real power-loss check separate.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:294`, `.plans/plans/go-s1-client-local/plan.md:295`, `.plans/plans/go-s1-client-local/plan.md:299`, `.plans/plans/go-s1-client-local/plan.md:315`, `.plans/plans/go-s1-client-local/decisions.md:44`.

### logic-r2 04 — BLOCKER — R6

**Failure.** The persisted clock is T. With `Keep` set to 10 seconds, a command finishes before its expiry at T plus 15 seconds. At T plus 30 seconds, pruning removes the receipt while the one-minute threshold leaves the saved clock at T. After restart with the wall clock at T plus 5 seconds, the same identifier gets `DecisionRun`. The existing test advances the clock by many hours and cannot expose this path. Failure to persist the clock before a prune also lacks a stated outcome.

**Fix.** Require a valid durable clock watermark at least as high as the expiry of every receipt about to be removed. Persist it before any removal, and refuse removal on any failed or uncertain clock write. Define missing and invalid clock recovery for used stores. If clock observations themselves are promised never to go backwards, keep the highest observation in memory and define which observations must survive restart. Test the short retention case, a restart and a watermark sync failure immediately before pruning.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:756`, `.plans/plans/go-s1-client-local/plan.md:765`, `.plans/plans/go-s1-client-local/plan.md:770`, `.plans/plans/go-s1-client-local/plan.md:777`, `.plans/plans/go-s1-client-local/plan.md:797`, `.plans/plans/go-s1-client-local/decisions.md:52`.

### logic-r2 05 — MAJOR — C11

**Failure.** The initial epoch is durable before the first acknowledgement mark is written. A process stop or a write failure between the two leaves an epoch and no mark. The next open classifies that state as corruption and stops the client indefinitely, although no event was ever appended. An epoch with no mark and no events could also be a used outbox whose mark disappeared after cleanup. An automatic mark reset based on that shape would reuse numbers.

**Fix.** Publish initial identity and mark zero in one atomic write, for example in one versioned, checksummed metadata record that thereafter updates the mark while preserving the epoch. Distinguish an unpublished initialization from a damaged published one. Crash or fail each bootstrap write step, reopen twice and assert either a safely new queue or the same published epoch. Keep the missing-metadata test for a used store. State whether temporary-file cleanup is the sole exception to the promise to leave a damaged folder untouched.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:637`, `.plans/plans/go-s1-client-local/plan.md:638`, `.plans/plans/go-s1-client-local/plan.md:640`, `.plans/plans/go-s1-client-local/plan.md:652`, `.plans/plans/go-s1-client-local/plan.md:658`, `.plans/plans/go-s1-client-local/plan.md:727`.

### logic-r2 06 — MAJOR — C11

**Failure.** The new format allows event 1 in two epochs for one machine. The link draft at the reviewed commit still has one machine-wide cursor and no epoch in `Hello`, `Proof`, `Welcome` or `EventAck`. A replacement outbox can receive the old cursor and remain stuck or lose its pending events if that cursor fits its local range. The plan also accepts any syntactically valid epoch file. A damaged epoch whose text still looks valid can register as new and let a previously applied but unacknowledged event be applied again.

**Fix.** Settle the epoch in the contract and both sprint 3 plans before the interfaces freeze. Specify how each proof and acknowledgement is bound to the current authenticated machine, epoch and stream, and reject missing or mismatched identity before accepting any cursor. Name the server's transactional writer for contiguous application and its cursor, plus the readers that emit the greeting and cumulative acknowledgements. Define what happens to old streams and old epoch records. Protect the on-disk epoch with the same integrity check as the mark and cover corruption that preserves the epoch's text format. Keep any version transition explicit.

**Evidence by reading at `9ec34833`.** `.plans/plans/go-s1-client-local/plan.md:195`, `.plans/plans/go-s1-client-local/plan.md:631`, `.plans/plans/go-s1-client-local/plan.md:638`, `.plans/plans/go-s1-client-local/plan.md:870`, `.plans/plans/go-s1-client-local/decisions.md:48`, `.plans/plans/go-s0-contract/draft/link.proto:105`, `.plans/plans/go-s0-contract/draft/link.proto:124`, `.plans/plans/go-s0-contract/draft/link.proto:140`, `.plans/plans/go-s0-contract/draft/events.proto:28`, `.plans/plans/go-s3-server-machines/plan.md:51`.

### logic-r2 07 — MAJOR — R9

**Failure.** The scan approves the resolved executable and selected directories, then passes every cleaned absolute search directory to the child. A legitimate wrapper that resolves a helper through its environment can therefore use a directory the scanner would refuse for a top-level executable. The trust rule also leaves intermediate steps of symlink resolution outside the checks. C7 can then report a version from a launch made outside the intended trust policy.

**Fix.** Apply the ownership and mutability rule to all path components used to select the executable and to directories offered for child program lookup. Define the accepted interpreter and helper policy for wrappers, and the scope of the guarantee when the operating system can resolve dependencies outside the supplied search path. Treat metadata or resolution failures as a refusal. Add a review check that verifies the approved execution paths and child environment before starting a version probe.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:532`, `.plans/plans/go-s1-client-local/plan.md:534`, `.plans/plans/go-s1-client-local/plan.md:535`, `.plans/plans/go-s1-client-local/plan.md:552`, `.plans/plans/go-s1-client-local/plan.md:558`, `.plans/plans/go-s1-client-local/decisions.md:58`.

### logic-r2 08 — MAJOR — C6

**Failure.** The key placement reports `ErrPlacedNotSynced`. The prescribed recovery reads that file and may enroll immediately, although a successful read proves only that its name is visible. If the server commits and the client loses power before the folder is synced, the enrollment can survive without its private key. Even with a durable key, a committed join whose reply or local marker was lost can only reuse the token until expiry. After expiry the prescribed retry is refused and the plan gives the owner no recovery path.

**Fix.** Require a successful folder sync or a successful close and reopen before using a key whose placement was uncertain. Bind the pending enrollment to the intended server before contacting it. Specify recovery of a committed same-key enrollment after token expiry through an agreed authenticated lookup, or give a concrete operator procedure that resolves the existing machine record without guessing or silently creating another. Specify how `server.json` placement uncertainty and invalid content are repaired without overwriting a completed enrollment. Test crashes before sending, after the server commits, and after the final marker is placed, including a retry after expiry.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:303`, `.plans/plans/go-s1-client-local/plan.md:405`, `.plans/plans/go-s1-client-local/plan.md:883`, `.plans/plans/go-s1-client-local/plan.md:884`, `.plans/plans/go-s1-client-local/plan.md:885`, `.plans/plans/go-s1-client-local/plan.md:886`, `.plans/plans/go-s0-contract/draft/link.proto:26`.

### logic-r2 09 — MAJOR — R6

**Failure.** A command starts under attempt 1. The stream ends and attempt 2 arrives while it is running. `Begin` returns the original running receipt and its caller sends nothing. If the handler sends the receipt's attempt 1 when it finishes, the server ignores the result. The rule for relabelling applies only to a replay. A separate pre-placement write failure during `Finish` also leaves the command marked running after the handler has finished, with no retry or recovery obligation for the caller.

**Fix.** Give the command handler sole ownership of the highest observed attempt while a command is running, and require publication of the committed result under that attempt on the current stream. On a pre-placement `Finish` failure, keep the terminal result and retry publication to disk without repeating the effect, or stop and reopen under the interruption policy. On uncertain placement, reopen before deciding which terminal receipt to send. Forward the stored `failed` result returned with `ErrResultTooLarge`. Define the durable handoff for a commanded restart before exit. Test redelivery during execution, a pre-placement finish error, and a restart redelivery.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:768`, `.plans/plans/go-s1-client-local/plan.md:772`, `.plans/plans/go-s1-client-local/plan.md:773`, `.plans/plans/go-s1-client-local/plan.md:778`, `.plans/plans/go-s1-client-local/plan.md:875`, `.plans/plans/go-s1-client-local/plan.md:877`, `.plans/plans/go-s3-client-link/plan.md:46`, `.plans/plans/go-s0-contract/draft/commands.proto:34`.

### logic-r2 10 — MAJOR — C11

**Failure.** Passing a `*state.Folder` ties each claim to a handle, but the store path comes from the handle's public, mutable `Paths`. A caller holding a lock on folder A can accidentally hand the store the paths of folder B. The claim on A then succeeds while the real holder of B also writes or removes temporary files in B. Exclusive event placement helps with name collisions, but does not protect the acknowledgement mark or the receipts against a second owner.

**Fix.** Make the store claim depend on a live, private lock capability and return paths derived from that capability. Keep exposed paths as read-only snapshots returned by value. Specify refusal after folder close and for a zero-value handle. Test that two handles cannot make successful claims that address the same store, including an alias of the same directory.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:284`, `.plans/plans/go-s1-client-local/plan.md:285`, `.plans/plans/go-s1-client-local/plan.md:286`, `.plans/plans/go-s1-client-local/plan.md:683`, `.plans/plans/go-s1-client-local/plan.md:724`, `.plans/plans/go-s1-client-local/plan.md:757`.

### logic-r2 11 — MINOR — D19

**Failure.** The clone defaults isolate distinct state directories. A service-only setting for `LGTM_CLIENT_STATE_DIR` does not reach a manual shell. If the service uses an override and the shell falls back to the default, the manual client uses a different enrollment, lock and clone directory. Logging the resolved paths happens after that selection and does not make them agree.

**Fix.** Have the installer use the same default for the service and its join command, or pass the same explicit setting to service, installer join, and documented manual invocations. Name the sprint 3 Stack team and the client's configuration reader on the handoff. Test one install and one manual invocation with a nondefault directory and verify that both address the same enrollment and clone path.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:204`, `.plans/plans/go-s1-client-local/plan.md:244`, `.plans/plans/go-s1-client-local/plan.md:887`, `.plans/plans/go-s1-client-local/plan.md:891`, `.plans/plans/go-s3-stack-client/plan.md:42`, `deploy/test/checks.sh:217`.

### logic-r2 12 — MINOR — C7

**Failure.** A wrong scanner gives every Cursor candidate its own 500 milliseconds. Four hanging candidates can still finish in about two seconds plus cleanup, inside the three-second assertion of `TestScanBudgetCoversAllCandidates`. The single-candidate timeout and the live-grandchild test cannot distinguish that implementation from the specified shared deadline.

**Fix.** Record which candidates start under a controllable deadline. Consume the entire budget in the first candidate and assert that no later candidate starts. Keep the wall-clock assertion as a generous guard against a hung test. Add a separate case with quick early failures and a later candidate receiving only the remaining budget.

**Evidence by reading.** `.plans/plans/go-s1-client-local/plan.md:536`, `.plans/plans/go-s1-client-local/plan.md:559`, `.plans/plans/go-s1-client-local/plan.md:560`, `.plans/plans/go-s1-client-local/plan.md:561`, `.plans/plans/go-s1-client-local/plan.md:597`.

### Risk scores and responsible builders

The three ratings are severity, fallout and probability. The evidence establishes what the plan permits. The Go implementation is still a target at the reviewed commit.

| Finding | Category | Ratings and reasons | Score and band | Responsible builder |
|---|---|---|---|---|
| logic-r2 01 | Integrity and design | 8 for a lost event, 7 for irreversible deletion, 4 for a faulty cursor. | 224, fix before release. | Client and server link. |
| logic-r2 02 | Integrity and design | 8 for a lost event, 7 for irreversible loss, 4 for one failed disk write. | 224, fix before release. | Client outbox and link. |
| logic-r2 06 | Design and integrity | 6 for failed reconnection, 5 for repair across teams, 6 for the ordinary case of an outbox replacement under incompatible contract rules. | 180, fix before release. | Contract and both link builders. |
| logic-r2 05 | Recovery | 6 for a client that stops starting, 5 for manual repair, 4 for one interruption during setup. | 120, schedule. | Client outbox. |
| logic-r2 07 | Trust boundary | 6 for a launch outside the intended trust policy, 5 for repair of one affected client, 4 for an unsafe search directory and a wrapper. | 120, schedule. | Client harness scan. |
| logic-r2 08 | Recovery and design | 6 for a blocked enrollment, 5 for operator repair, 4 for one uncertain join followed by an unavailable token or key. | 120, schedule. | Client enrollment and server machines. |
| logic-r2 09 | Timing and design | 6 for a command whose final result cannot be collected, 5 for recovery, 4 for a disconnect or a failed storage write. | 120, schedule. | Client command handling and receipts. |
| logic-r2 03 | Integrity | 8 for a lost state tree, 7 for irreversible loss, 2 for a failed creation sync followed by power loss. | 112, schedule. | Client state. |
| logic-r2 04 | Integrity | 8 for a repeated command effect, 7 for an effect that can be irreversible, 2 for a short retention setting and rollback. | 112, schedule. | Client receipts. |
| logic-r2 10 | Ownership | 6 for a second writer reaching a store, 5 for manual repair, 3 for an incorrect caller combined with another open. | 90, schedule. | Client state and both stores. |
| logic-r2 11 | Configuration | 2 for a manual invocation using a different folder, 3 for owner correction, 6 for an ordinary invocation with a service override. | 36, record. | Stack installer and client configuration. |
| logic-r2 12 | Test gap | 1 for a missed timing regression, 1 for a test-only gap, 2 for the configured fixture. | 2, record. | Client harness test writer. |

## Answers to the round 2 assignment

1. **Each patch.** The table below follows each finding group in the round 1 register and maps the numbered patch locations that carry its fix.
2. **Unreviewed mechanisms.** The epoch and reconciliation are covered by findings 01, 05 and 06. The trusted-program rules are covered by finding 07. The default clone directory is covered by finding 11. The attack log omits the clone default from its count of new mechanisms.
3. **Rejected findings.** Nothing found in the rejected-outcome column. The register lists zero rejected findings. The old counter-jump rule was rejected as a design choice in decision 41, which is different from rejecting a finding. The round 1 failure scenario and the old decision 17 agree on what that jump did.
4. **Changed names and owners.** The table names the local and external readers where a patch changes them. The unresolved handoffs are the sender's proof of send, epoch fields and their server readers, command-result publication, and the installer setting's manual callers. The contract draft at the reviewed commit has no `INTERRUPTED` outcome. Its writer is the Contract team, its producer is the client command handler, and its consumers are the server command store and any status mapping to the website. The later briefs identify the link, command and reporting packages that must pick up those handoffs.
5. **Interacting rules.** The decision tables below cover cursors, bootstrap, receipts and enrollment. The concrete conflicts are a pending cursor without evidence of send, an epoch-only directory indistinguishable from damage, a prune outrunning the saved clock, and a join marker policy with no recovery after token expiry. The trusted candidate can also receive a search path that fails its own trust rule. The expiry extension code is defensive for arrivals while a receipt exists, and the server must keep expiry immutable for a command identifier after any prune.
6. **Round 1 blockers or majors still open.** Group A closes replacement but leaves uncertain producer recovery incomplete. B leaves an ancestor sync hole on reopen. C still deletes unsent events. F leaves the claimed paths mutable. H leaves enrollment recovery incomplete. K leaves child lookup outside the trust policy. N leaves key test assertions too weak. M and Y leave recovery and deduplication underspecified. R closes the oversized-result state in the store and leaves publication, restart and finish-failure handling open. E still permits rollback after prune. U remains explicitly with the owner.
7. **Lens coverage.** The following numbered answers cover all eight logic questions for the changed text. Treat work items 7 and 8 as high until the reconciliation and result-publication decisions are settled. Item 7 can split the local queue from authenticated reconciliation. Item 8 can split receipt lifecycle and clock persistence from the command handler's attempt and restart policy.

### Outcomes for the round 1 finding groups and patch locations

The numbered locations refer to `.plans/plans/go-s1-client-local/attack/round-1-findings.md:106` through line 131. The patch evidence is the comparison from `fb992dc9` to `9ec34833`. The plan and decisions match the reviewed commit in the checkout.

| Group | Patch locations | Effect of the patch and affected owners |
|---|---|---|
| A | 1, 7, 9, 10, 11, 12, 17, 19, 20, 25 | Exclusive placement and `ErrPlacedNotSynced` stop the next append replacing the first. State writes the outcome. Enrollment, outbox, receipts and the sprint 3 callers read it. The caller still cannot resolve the producer's event, and reading an uncertain key is insufficient before joining. See findings 02 and 08. |
| B | 9, 10, 19, 21, 25 | Syncing each newly created parent fixes uninterrupted setup. Reopen can miss a failed ancestor sync. See finding 03. |
| C | 2, 3, 4, 7, 16, 17, 19, 20, 21, 23, 24, 25 | The counter no longer jumps and the new ceiling prevents numeric wrap. The epoch writer is item 6 and its queue reader is item 7. The authenticated sender, receiver and server cursor writers need a shared contract. The new paths still accept an unsent cursor and add a torn bootstrap. See findings 01, 05 and 06. |
| D | 2, 3, 18, 19, 21, 25 | Pinning prevents the reported deletion and the unreadable-file rule denies use. Item 8 owns the receipt and the sprint 3 handler reads its outcome. A repair of an unreadable-as-data receipt with no decoded times needs a canonical pinned form so it can survive validation without being repaired on every open. |
| E | 5, 7, 18, 19, 25 | Equality and later expiry during terminal replay are explicit. Item 8 reads and writes the saved clock. Throttled persistence and incomplete prune ordering leave a rollback case. Server clock skew is a separate open decision. See finding 04. |
| F | 7, 9, 10, 17, 18, 20, 25 | A second claim on the same live handle is refused. State owns the lock and both stores own their claims. The mutable paths weaken the ownership guarantee across handles. See finding 10. |
| G | 3, 7, 9, 10, 20, 21, 25 | The lock has an open-file check and both `join` and `run` hold it. A complete trust rule must cover the path used after resolution as well as the resolved ancestors. The two-process alias case is still absent from the required tests. See finding 10. |
| H | 3, 7, 11, 12, 20, 21, 25 | A marker written last and a reused key remove the first failure while the token is valid. Client enrollment writes and reads the marker. The server owns the token replay record and its expiry. A lost or uncertain commit still needs a complete rule. See finding 08. |
| I | 3, 7, 11, 12, 21, 25 | Nonblocking open, descriptor validation, bounded input and a distinct unreadable error handle the reported cases. The enrollment package writes these errors and the two sprint 3 entry points read them. The size fixture should contain a valid-looking prefix up to the old read limit to isolate the size check. |
| J | 2, 3, 14, 15, 21, 25, 26 | An absolute fixture helper and its start marker fix the immediate exit. Exit failure wins over a matching line. Scanner callers read the new `Found.Problem` locally. The shared-budget assertion remains weak. See finding 12. |
| K | 2, 3, 14, 15, 21, 24, 25 | The scanner denies directly tested writable candidates. It still supplies unchecked directories to child lookup. See finding 07. |
| L | 2, 3, 14, 15, 21, 25 | Whole-line matching handles the two extra-text cases. The version parser must distinguish a complete first line from a prefix cut off at the output cap. The scanner writes the normalized version and the link reads it. Real output remains a device check. |
| M | 2, 3, 4, 17, 19, 20, 21 | The handoff gives full-queue refusal a producer and names one sender. A producer holding an unsaved report needs a recovery rule across restart, and the uncertain-outcome owner is still missing. See finding 02. |
| N | 1, 6, 9, 10, 17, 19, 21 | Separate helpers and exact append crash assertions improve isolation. The acknowledgement crash test still accepts either mark at every step, and the removal recorder checks visibility of the mark rather than its completed sync. The 100-number floor measures activity, not the uncovered orderings. |
| O | 11, 12, 25 | The value methods and pointer-backed representation cover the named credential shapes. The credential package writes the printed representation. The signer wrapper remains a separate value whose formatting should be checked if a caller can log it. |
| P | 17, 19 | The queue owns channel closure and preserves the last values. Client sender and monitoring code read the closed-channel signal and snapshot. Nothing found in the patched second-close rule. |
| Q | 2, 4, 13, 15, 21, 25 | The real Mac command and deadline agree. The macOS builder writes the values and the device runner observes them. A comparison against the same system counter still needs stable sampling when load changes. |
| R | 18, 19, 20, 21, 25 | An oversized result now writes a terminal failure. The receipt store writes it and the command handler must explicitly send it despite the accompanying error. A disk error and a commanded restart still need dispositions. See finding 09. |
| S | 2, 7, 18, 19, 20, 24, 25, 26 | Ended receipts replay across attempt changes. Contract owns the interrupted outcome. The client sends it, the server stores it, and any website status reader needs a mapping. Running redelivery still lacks result-attempt ownership in this plan. See finding 09. |
| T | 7, 16, 19, 25, 26 | The checksum detects the stated changed-digit case. Pending records are checked before acknowledged events are removed. Temporary files are removed first even when the directory later proves damaged, so the untouched-folder promise needs that exception or a validation-before-cleanup rule. See finding 05. |
| U | 1, 5, 16, 19, 20, 24, 25 | Existing damage is a deliberate refusal pending the owner's ruling. The same policy now triggers for an ordinary interrupted bootstrap. See finding 05. |
| V | 8, 10, 26 | The build test checks all packages in an output-free build and writes only the command binary by name. The deployment test uses the one-package form. Nothing found in the patched split. |
| W | 23, 26 | Confirmed live that `.plans/scopes/client/backend/scope.json` is tracked at the reviewed commit. Nothing found in the updated scope reference. |
| X | 6, 10, 12, 13, 15, 19, 22, 26 | The fixtures, helper ownership, task anchors and traceability are now named. They describe where to build and test. They cannot supply an assertion for a missing failure rule. See findings 03, 05 and 12. |
| Y | 17, 19, 20, 21 | The append crash test proves a placed event is readable after restart. It does not prove producer resolution or that same-content reports arriving under different numbers preserve freshness. See finding 02. |
| Z | 7, 8, 20, 25 | Item 1 computes the default, sprint 3 reports it, wave 2 creates and fills it, and the Stack team owns the service setting. The two resolved paths differ when only one launch path gets an override. See finding 11. |

## Answers to the logic lens

1. **Ordering and restart.** The queue starts at 1, the acknowledged mark at 0, and the last usable sequence is one below the ceiling. The restart derives the next number from the mark and highest pending event, and gaps refuse open. Ties and lower cursors follow the table below. Receipt attempts start at 1 and the identifier owns the execution decision. Terminal replay uses the incoming attempt. The running case, action on attempt exhaustion at the server, epoch replacement and saved-clock ordering need agreement. The first acceptable harness candidate wins in configured order, with one stated deadline per harness.
2. **Failure permissions and repair.** A pre-placement event write can be retried. An uncertain placement suspends the store, but the producer has no way to settle its own result. A corrupt outbox and unreadable receipts refuse use. The bootstrap, ancestor, clock, command finish and enrollment failures require the repair rules in findings 02 through 05, 08 and 09. The stated client-clock default remains subject to the owner's separate skew choice. A relative lifetime measured from receipt, as one proposed option describes, would need to account for time in transit before it can preserve a server deadline.
3. **Interacting states.** The tables below identify the cursor, outbox setup, receipt and enrollment cells. For the harness rule, a missing candidate is skipped, a directly untrusted candidate is refused, and an accepted candidate must exit successfully with a complete matching first line. A trusted wrapper with unresolved helper trust remains an unsafe acceptance case. A zero exit with the Go wait-delay error is a special case only when the complete line has already been seen.
4. **Zero and empty.** The zero mark is valid for a fresh epoch, `Ack(0)` is a no-op and `Read(0, n)` starts at the first pending number. A zero event sequence and zero command attempt are refused. A zero or negative read limit returns nothing. A zero harness timeout uses its default and an empty harness problem accompanies success. The zero command decision must never authorize execution. Missing epoch in a used queue is corruption, while a missing clock in a used receipt store and missing time fields in a repaired pinned receipt need valid durable interpretations.
5. **Concurrent actors.** The folder lock and store claims ordinarily give each store one owner. Claiming mutable paths breaks that relationship. The sender and a producer can interleave acknowledgement and uncertain append recovery without a single owner of recovery. A command worker and the next connection can observe different attempts, so the worker must publish through an owner that knows the latest attempt. The server's current stream and its replaced stream also need an agreed fence on writes to the epoch cursor.
6. **Named risky mutations.** In item 2, stripping exclusivity fails the replacement test, but skipping the deeper sync on a repeated open passes. In item 3, removing nonblocking open can hang the named pipe case, but removing only the size guard can still pass the current oversized fixture because its truncated prefix has forbidden trailing spaces. In item 6, dropping the mark checksum fails the changed-digit test, but an epoch-only bootstrap stop is untested. In item 7, removing a file before making the mark visible fails the recorder, but removing it after placement and before the folder sync can pass that recorder and the permissive acknowledgement crash test. In item 8, using the wall clock alone fails the large rollback test, but retaining the one-minute write threshold and pruning on a shorter retention passes.
7. **Cheapest wrong implementation per item.** Item 1 can require all settings fields when a settings file is present, since the listed tests use a missing file or fully populated settings. Item 2 can omit the earlier ancestor on reopen. Item 3 can accept a key owned by another user in the ordinary suite, which the plan leaves to a privileged check. Item 4 can reuse the production conversion in the Mac expected value, allowing a shared rounding defect past the device comparison. Item 5 can give every candidate a fresh timeout. Item 6 can publish initialization in two writes. Item 7 can authorize acknowledgement solely with `Next()`. Item 8 can persist the clock only on large jumps and still pass its rollback case.
8. **Shapes that rule out wrong states.** Use one atomic metadata record for initial outbox identity and mark, a private capability tying the store's path to its held lock, explicit evidence of send owned by the authenticated sender, a durable event identity for retry, and a persisted clock barrier before pruning. Give one command handler ownership of both the highest attempt and terminal-result publication.

### Cursor and bootstrap decisions

The cursor table assumes a valid current epoch and an open, healthy queue. A failed or closed queue refuses the operations before these comparisons.

| Input relative to local state | Live `Ack` now | `Reconcile` now | Assessment |
|---|---|---|---|
| Below acknowledged mark | No change. | `ErrServerBehind`. | The difference is coherent for a repeated acknowledgement versus evidence that the server lost progress. |
| Equal to acknowledged mark | No change. | No change. | Coherent, including zero on a new epoch. |
| Inside the pending range with evidence of send | Advances the mark. | Advances the mark. | Safe when the server committed the contiguous prefix for this epoch. |
| Inside the pending range with no evidence of send | Advances the mark. | Advances the mark. | Both cells can delete queued-only data. |
| At or above `Next()` | `ErrAckAhead`. | `ErrServerAhead`. | Both refuse without advancing the counter. |

| On-disk state on open | Written result | Assessment |
|---|---|---|
| No epoch, mark or event | Initialize. | Coherent for a new or fully lost outbox with a properly scoped new epoch. |
| Epoch only | Refuse as corruption. | A normal write interruption and a missing mark in a previously used queue have this same shape. |
| Mark or event, missing epoch | Refuse as corruption. | Safe refusal, including the stated owner repair policy. |
| Valid epoch and mark, contiguous pending events | Recover and continue. | Coherent when the epoch is the one originally published. |
| Syntactically valid but changed epoch, valid mark and events | Recover locally. | Registration of the wrong epoch can duplicate application or prevent reconnect depending on the mark. |

### Receipt and enrollment decisions

| Receipt on arrival | Expiry in the future | At or past expiry | Assessment |
|---|---|---|---|
| None | Persist `started`, then run. | Persist `expired`, then replay. | The future cell is unsafe if a receipt was pruned before its clock barrier was durable. |
| Running in this process | Return running. | Return running. | The original execution may finish, but a higher attempt needs a result assigned to it. |
| Ended or pinned | Replay the stored outcome. | Replay the stored outcome. | A later observed expiry can extend retention while the receipt exists. The server's immutable expiry is needed after pruning. |
| Read failure or uncertain receipt write | Refuse execution. | Refuse execution. | Safe refusal for `Begin`. The running handler still needs a result recovery rule after `Finish` fails. |

| Enrollment state | Written action | Assessment |
|---|---|---|
| No key and no server marker | Create a key and join. | Continue only once the key placement is durable. |
| Key and no marker, same token still valid | Reuse the key, then write the marker after confirmation. | The committed and uncommitted server cases can share this path under the draft. |
| Key and no marker, token expired | The prescribed retry is refused. | Recovery is unspecified for a committed server record. |
| Valid marker present | `join` refuses and `run` can use it. | Coherent. |
| Invalid marker present | `join` refuses and `run` says to run `join`. | The prescribed repair loops back to the refusal. |

## Post-build checks that need the running client and server

| Check | Scenario and expected result | Evidence and responsible area |
|---|---|---|
| Power loss after a recovered setup | Inject a failed directory sync at a higher ancestor, reopen successfully, acknowledge one local append and then remove power. After restart, that event is present and the same epoch opens. | Capture the sync trace and the event bytes before and after restart. Client state and outbox. |
| Old stream finishes after replacement | Delay an event transaction on the old stream while the machine reconnects. The server's committed application and cursor agree, and the new stream delivers each pending event once in order. | Record event identities, committed rows and acknowledgements on both streams. Client and server link. |
| Command redelivery during execution | Redeliver a running command under a new attempt. Finish its one observable effect, then lose the first response. The current attempt receives the stored terminal result and the effect count is one. | Record attempts, the final result, receipt contents and effect count. Client command handling and server commands. |
| Installed wrapper and device probes | Run the scanner with the supported installed harnesses. The start trace and child search paths satisfy the frozen trust policy, and macOS values agree with the system within the specified tolerance. | Record the tested harness versions, approved launch paths and paired macOS samples. Client harness and machine facts. |

## Grade

4 out of 10. The report contains 4 blockers, 6 majors and 2 minors.
