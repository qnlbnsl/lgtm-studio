REVISE

A restored outbox can permanently delete an event the server never received because a sent number does not identify its payload across different histories of the store.

## Findings

The review covers the changes from `9ec34833` through `b68adba0` and the rules they affect. Unless marked as a working copy or a live probe, the evidence is by reading at `b68adba0`.

### logic-r3 01 — BLOCKER — C11 — A restored sent mark can discard the wrong payload

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:666` records the send intent before writing to the stream. Lines 669–671 use the server's number to remove rows in that sent range, including in a restored store. The high-mark test at line 694 explicitly expects those rows to disappear. `.plans/plans/go-s0-contract/draft/events.proto:15` identifies the committed prefix only by number.

**Failure.** A backup contains event 1 with payload A and a sent mark of 1, taken just before sending A. A was never sent. The machine starts a fresh store under the same enrollment, which the patch supports, and the server applies event 1 with payload B. Restoring the backup now makes the server's valid mark of 1 delete A. A server mark of 2 would delete A through the other reconciliation branch as well. On an ordinary live stream, the durable pre-send mark alone also cannot distinguish a queued frame from a frame actually offered to the transport. The current Sprint 3 Client working copy adds a per-stream bound at `.plans/plans/go-s3-client-link/plan.md:169`, which addresses that live-stream distinction but leaves the restored history unresolved.

**Fix.** Authorize deletion only with evidence that the acknowledged numbers belong to the same store history. If restoration under the same machine identity is supported, agree on durable event identity and matching server deduplication before enabling automatic reconciliation. Otherwise make restoring or replacing the store an explicit repair, preserve the old queue, and stop on unproved continuity. Require the Sprint 3 sender to gate live acknowledgements by the current stream and its offered prefix. Add the two-payload restore case to the frozen tests.

### logic-r3 02 — BLOCKER — R6 — Restoring the shared store rolls back evidence of command execution

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:620` puts events and receipts in one file. Line 671 promises recovery from a restored store. Lines 738–743 authorize execution if the identifier has no receipt and has a future expiry. `.plans/plans/go-s0-contract/draft/commands.proto:6` permits another attempt while the server has no result.

**Failure.** The owner takes a backup, then a valid command runs and its result is lost on disconnect. The owner restores the backup before the command expires. The server retries the same pending identifier. The restored file contains neither its receipt nor any pruning evidence, so `Begin` authorizes the effect a second time. This also applies to the proposed repair that starts an empty store under the old enrollment.

**Fix.** Make the restore and damaged-store policies cover both records. Keep same-enrollment restore or reset disabled unless the server can prove which commands from the prior store may still arrive and fence them before the replacement starts. The alternative is explicit recovery under a new enrollment that retires the old machine and its pending commands. Test a restore from before `Begin` after an effect whose result the server did not receive.

### logic-r3 03 — BLOCKER — plan — Item 6 needs code assigned to its successor

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:134` and line 714 require item 6 to merge before item 7 starts. Item 6 must call the receipt recovery step before returning from `OpenStore` at line 660. The recovery step and its only production file are assigned to item 7 at lines 713 and 746. `.plans/epics/go-rewrite/epic.md:75` requires passing checks on each work item's exact merge commit.

**Failure.** Item 6 is built and tested in isolation as scheduled. A call to the item 7 recovery function cannot compile. Omitting it until item 7 would give the later builder an undeclared edit to `store.go` and leave the first work item with a different open contract.

**Fix.** Assign the open and receipt recovery seam, including a compilable implementation, to item 6, then assign only the remaining receipt methods to item 7. Alternatively merge items 6 and 7 as a single passing unit. State which item owns every file and test at the boundary.

### logic-r3 04 — MAJOR — C11 — A failed write needs a store-wide gate across transactions

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:653` routes writes through `DB.Update`. Lines 672–674 promise a shared failed state, concurrent calls, and stable close behaviour. In the allowed bbolt v1.4.3 fallback, `db.go:816` acquires the transaction write lock and `tx.go:357` releases it during transaction close. The test at plan line 685 starts its second append only after the first returns.

**Failure.** Two producers pass a healthy check before the first commit has finished. The second waits for bbolt's write lock. A commit error releases that lock before the application records the failure. The second producer can commit and report success during the failure window. A command handler is another writer and could receive `DecisionRun` in that window. The stated behaviour says the store stops using the file after the first uncertain commit.

**Fix.** Give one store owner or one lock authority over admission, the transaction, classification of its result and publication of failed or closed state. Define `Close` as legal after failure and specify what no-error accessors return. Ensure the link stops buffered sends when the failure is published. Hold the first write at an injected commit error while a second write is waiting and assert that the second never enters its transaction.

### logic-r3 05 — MAJOR — C11 — Opening an existing store can write to damage before refusing it

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:656` gives initialization to a temporary file, but lines 657–662 pass an existing regular file to read-write `bolt.Open` before checking its contents. The allowed bbolt v1.4.3 fallback initializes a zero-length file in `db.go:262–274`. A live isolated probe opened an existing zero-byte file successfully and wrote 16,384 bytes. The plan also says only the first and last keys are used to check event continuity and counts at line 659.

**Failure.** A previously used `client.db` is truncated to zero. The preflight accepts the filename and permissions. bbolt writes new metadata before the application discovers missing buckets, contradicting the promise that a refusal leaves the file untouched. A separate valid bbolt file with event keys 3 and 5 and metadata that claims three pending rows can pass a first-and-last check. The next send hits a gap and the link cannot drain the queue.

**Fix.** Separate existing-file validation from creation. Disallow initialization for an existing file and do preflight validation without changing it. Check the exact metadata sizes and ranges, contiguous decoded keys, row count, byte total and the syntax of an existing prune mark before receipt repair writes. Bound the walk by the stored data. Add zero-length and interior-gap fixtures that must leave the bytes unchanged. Define missing and damaged store behaviour for an already enrolled machine separately from a new enrollment.

### logic-r3 06 — MAJOR — R6 — The prune clock can remove a live receipt and suppress a different command

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:735` and line 747 use the same estimated time for expiry and retention. Lines 734 and 741 use one maximum expiry for every command. `.plans/plans/go-s0-contract/draft/commands.proto:7` allows late attempts to collect a result, and line 13 requires at least 24 hours of retention after expiry. The current Sprint 3 Client working copy requests a separate retention clock at `.plans/plans/go-s3-client-link/plan.md:244`.

**Failure.** A welcome arrives when server time is one second short of expiry plus 24 hours. The proof-based estimate is two seconds ahead. The client prunes the result while the server can still ask for it and replays `expired` in place of the saved outcome. Independently, after a server clock correction, a brand-new identifier can have a valid expiry earlier than an unrelated pruned command's future-dated watermark. It is refused even though it never ran. The forward-jump test at plan line 768 can pass with no removal and no watermark.

**Fix.** Use an agreed conservative retention clock or an explicit server retirement boundary, and make the receipt lookup and expiry decision separate. Replace the global expiry watermark with an identifier-scoped retirement record, or agree on a server-proven bound that no new identifier can fall below. Specify when retirement records may be deleted. Force an actual prune, reconnect after a backward sample, and assert that the old identifier stays retired while an unrelated valid identifier still runs. The link draft at the reviewed commit has `Welcome.server_time`. The concurrently edited working copy of `.plans/plans/go-s0-contract/draft/link.proto:224` has removed it while the command draft still requires it. Settle the clock source and its readers jointly before freezing the plans.

### logic-r3 07 — MAJOR — R6 — Receipt recovery has no canonical state for all disk outcomes

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:734`–746 defines persisted fields, validation and reopening. Lines 741–742 return an expired receipt without defining its persisted fields. Line 745 promises interruption after any failed `Finish`, while line 746 only changes a receipt still marked `started`. The failed-`Begin` test at line 772 already recognizes both disk outcomes of a commit error.

**Failure.** `Finish` commits `done` and the final sync reports failure. The reopened store can contain `done`. The plan tells the caller to report `interrupted`, but its recovery reads and replays `done`. In a different case, an unreadable receipt has no decodable attempt or times, and startup has no server clock yet. The plan gives its replacement the states `interrupted` and pinned but leaves the required fields undefined, so a second open can classify the replacement as damaged again. The current test only checks the same external outcome and would pass repeated writes.

**Fix.** Treat the durable version on reopen as authoritative. Replay the persisted terminal receipt after a surviving finish, and convert a surviving `started` receipt once. Define a valid persisted form for a pinned damage record that does not require invented attempt or expiry data. Define complete times and fields for valid transitions. State whether an expired unseen command returns a synthetic receipt without writing. Test both injected `Finish` outcomes and assert that reopening a repaired receipt again makes no repair write.

### logic-r3 08 — MAJOR — C11 — The outage producer and freshness rule disagree with the wire

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:835` orders reports by producer time and line 839 defaults to emitting a numbered load report. `.plans/plans/go-s0-contract/draft/events.proto:38` makes the timestamp display-only, lines 41–46 list only facts and harness inventory as numbered reports, and lines 92–101 carry load on an unnumbered heartbeat. `deploy/test/checks.sh:583` checks every expected applied sequence in exact order. The current Sprint 3 Client working copy uses periodic facts instead at `.plans/plans/go-s3-client-link/plan.md:197`.

**Failure.** A builder follows the default and cannot encode a numbered load report. If it sends an empty or unknown report instead, the draft permits consuming the number without applying a report. Local buffering logs can then claim a report was durably captured while the server receives no useful data. Independently, a machine clock correction can stamp new facts earlier than an old stored report. Applying latest producer time keeps the old facts, and equal times have no tie rule. This also defeats the uncertain-append recovery claim for regenerated reports.

**Fix.** Adopt the supported periodic facts trigger for approved machines and keep load on the live heartbeat, or get an explicit contract and Server team agreement for numbered load. Define supersedable reports and publish a single freshness order independent of machine wall-clock corrections, with an explicit equal-case rule. One option is to serialize capture through one producer per report kind and apply the resulting event order, after resolving restore semantics. Assert that the exact buffered payloads were applied in order and that a fresh report wins after a backward clock jump.

### logic-r3 09 — MAJOR — C7 — The scanner's new folder filter contradicts its trust tests

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:536` drops refused directories before it finds candidates. Lines 537–545 give `ProblemNotFound` when no candidate existed and apply the no-write-bits rule to every link. Lines 561 and 565 expect `ProblemUntrusted` from a candidate located only in a dropped directory. A live Linux metadata probe reported mode `0777` on an ordinary symbolic link whose directory was private.

**Failure.** If the only fake harness is in a rejected search directory, the planned scan never inspects it. Both tests nevertheless require a result for an inspected, refused candidate. A common harness with a clean link from a trusted directory to a trusted regular executable is also refused by a literal mode check of the link itself.

**Fix.** Define a link-specific rule using the link's ownership and its containing directory, then validate each target and directory. Test one valid link and one refused intermediate step. Choose the result for a refused search directory and align both fixtures with it. Isolate the Linux access control list (ACL) assertion from a mode-bit refusal and distinguish a missing attribute from an attribute read failure.

### logic-r3 10 — MAJOR — R9 — A trusted search directory does not certify its helper files

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:536`–540 validates search directories and the directly launched harness. Lines 559 and 606 treat that directory filtering as the basis for the helper guarantee.

**Failure.** A legitimate harness needs a helper whose directory passes the filter but whose file or resolved target would fail `trustedProgram`. The child searches the directory and launches the helper without that file check. The existing positive and negative fixtures vary directory trust only, so they pass with this gap.

**Fix.** Give the helper policy a stated scope and a mechanism that matches it. Where the scanner promises helper trust, require a controlled helper and interpreter set whose executable paths meet the file and directory policy. Otherwise restrict which wrappers can be used and state what they must provide. Test a compliant directory with a noncompliant helper fixture and check that the helper is not started.

### logic-r3 11 — MAJOR — D19 — The state-path rule assumes services inherit shell settings

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:202`–209 still allows two environment overrides. Line 864 removes the service override and assumes all entry points resolve the same folder. The current Stack working copy of `.plans/plans/go-s3-stack-client/plan.md:224`–236 still uses `server.json` and a service state-folder override. `deploy/test/checks.sh:520` reads `identity.json`.

**Failure.** The installer runs from a shell with an absolute `XDG_STATE_HOME` and writes the identity there. A service started without that shell variable looks under the home default and refuses to start. Applying the Stack working copy unchanged instead makes an installer rerun look for the old filename and try to join an already enrolled folder.

**Fix.** Define one resolved-path agreement across the installer, both service managers, and manual overrides. Make the installer and both templates and test stand-ins consume `identity.json`. Test a rerun and a service start with shell overrides present but absent in the service environment. Preserve the user-flow fixture's ability to select a separate store.

### logic-r3 12 — MAJOR — C6 — The expired-join procedure cannot identify the record to remove

**Evidence.** `.plans/plans/go-s1-client-local/plan.md:197` and lines 855–857 place the final identity after confirmation. The Sprint 3 Client working copy at `.plans/plans/go-s3-client-link/plan.md:106` puts the machine identifier in that identity. Rule 20 asks the owner to remove the half-joined machine after expiry. `.plans/plans/go-s0-contract/draft/link.proto:59`–66 rejects an expired token before returning a same-key reply. Lines 148 and 154 distinguish the host name sent by the machine from the machine identifier returned by the server.

**Failure.** The server commits a waiting machine, its reply is lost, and the token expires. The local folder has only a key. The Machines page can contain two waiting machines with the same host name, and the procedure gives the owner no way to tell which record belongs to this enrollment. Following it can remove an unrelated machine or leave the half-joined entry behind.

**Fix.** Persist the intended server and a nonsecret enrollment reference before the first join. Give the owner a lookup that identifies the exact waiting record by that reference or the machine public-key fingerprint. Distinguish no server commit, an existing waiting record and a valid final local marker. Preserve the key until that resolution, and test a lost reply with two equal host names and an expired token.

### Risk scores and responsible teams

The scores use severity, fallout and probability in that order. The planner and responsible teams own the disposition and closing evidence.

| Finding | Category and evidence | Factors and score | Responsible team and band |
|---|---|---|---|
| 03 | Design. Confirmed by reading. | Required merge gate. | Client. Before merge. |
| 08 | Design. Confirmed by reading. | 6 for a failed core flow, 5 for coordinated repair, 10 whenever the specified outage producer runs. 300. | Client, Contract and Server. Fix before release. |
| 09 | Bug. Confirmed by reading and a Linux metadata probe. | 6 for a missing scan result or failing gate, 5 for a coordinated correction, 8 for ordinary linked installs. 240. | Client. Fix before release. |
| 04 | Bug. Confirmed by reading. | 6 for continued operations after failure, 5 for recovery, 4 for a storage fault while writing. 120. | Client. Schedule. |
| 06 | Bug and design. Confirmed by reading. | 6 for a lost result or refused command, 5 for repair, 4 for a delayed boundary or clock correction. 120. | Client and Contract. Schedule. |
| 01 | Integrity. Confirmed by reading. | 8 for permanent event loss, 7 for loss of the original pending record, 2 for divergent histories followed by a restore. 112. | Client, Contract and Server. Schedule. |
| 02 | Integrity. Confirmed by reading. | 8 for a repeated effect, 7 for a potentially irreversible effect, 2 for a restore while a command is pending. 112. | Client and Server. Schedule. |
| 10 | Isolation. Confirmed by reading. | 8 for effects under the client's identity, 7 for potentially irreversible effects, 2 for an affected wrapper and helper configuration. 112. | Client. Schedule. |
| 11 | Design. Confirmed by reading. | 6 for a client that cannot start, 3 for a path correction, 6 for supported custom paths and reinstall. 108. | Client and Stack. Schedule. |
| 05 | Integrity and bug. Confirmed for empty-file opening by live probe and for the gap by reading. | 6 for losing startup or delivery, 5 for repair, 2 for a damaged store. 60. | Client. Schedule. |
| 07 | Bug. Confirmed by reading. | 6 for contradictory command outcomes, 3 for replay after recovery, 2 for ambiguous finish or malformed stored data. 36. | Client. Record. |
| 12 | Design. Confirmed by reading. | 6 for an ambiguous removal or duplicate enrollment, 3 for operator repair, 2 for a lost reply that outlives the token. 36. | Client and Server. Record. |

## Answers to the task's attack questions

1. **Each patch in the register.** The table below follows the 21 patch entries at `.plans/plans/go-s1-client-local/attack/round-2-findings.md:88`. The outcome checks also cover groups A through S.
2. **Each unreviewed mechanism.** The single-file layout, bootstrap, transaction failure rule, outbox, receipts, sizing rows and Sprint 3 fold are covered by logic-r3 01–08. The trust mechanism is covered by logic-r3 09–10. The historical epoch marker in decision 41 is superseded by decisions 55 and 60. Its live replacement is covered by logic-r3 01. The clone-default marker at plan line 243 works for a deliberately separate state root, while its interaction with installer state selection fails in logic-r3 11. The remaining markers in the attack log describe earlier rounds.
3. **Rejected findings.** Nothing found. I checked each outcome at register lines 21–54 against the zero rejected count at line 131. Group R was passed to the owner and has an explicit default.
4. **Changed names and their owners.** The handoff table below names the writers and readers and identifies the missing agreements.
5. **Interactions.** The tables below show the decisive cells for the cursor, receipt and startup rules. The report-time conflict is between two readers of the same event timestamp. The installer conflict is between two callers of the same state resolver with different environments.
6. **Round 2 blockers and majors still open.** A and O still lack a safe proof for destructive reconciliation. E protects the offered list of directories but still lacks per-helper file trust. G solves interrupted first creation but does not preserve a damaged existing file. H fixes key durability and same-token replay while the token is valid, but the expired-token procedure lacks an identifying reference. I defaults to a report the draft cannot encode. J names an owner but still permits a concurrent write and a stale regenerated report. K fixes the ordinary newer-attempt case and the outcome names, while its failed-finish outcome is still contradictory. L still lacks a common launch environment. P replaces the stored clock with a global expiry cutoff that reproduces false refusal after a server clock correction. B, C, M and N close their reported failure. D closes the split prune commit. F removes the old epoch format, Q removes replay writes, and S adds the numerical edge cases, subject to the findings on their replacement mechanisms.
7. **Lens questions reached by the patches.** The next eight answers cover numbering, time and recovery, ownership, zero values, tests and the shape of stored state.

### The 21 patch entries

| Patch | Outcome and effect on other statements |
|---|---|
| 1. Overview | One transaction now protects each local change. Its claims of crash recovery and no lost work overstate the restore and failed-write rules. See logic-r3 01–05. |
| 2. Key decisions | The on-disk bound improves A. The pre-send mark still proves intent only, and the restore case can delete the wrong row. The new module has an explicit version choice. See logic-r3 01 and 05. |
| 3. Workstreams | A shared store removes C's split mark and removal. Replacing that store now also removes command evidence. See logic-r3 02. |
| 4. Rollout | Item 2 supplies the folder claim. Item 7 cannot follow a passing item 6 as currently specified. See logic-r3 03. |
| 5. Owner choices | R is genuinely open and its strict default is explicit. The damaged-store default preserves state, but the alternate reset paths must fence old commands. See logic-r3 02 and 10. |
| 6. Shared implementation rules | The distinct helper names prevent test-name collisions. The dependency is concrete, but the epic assigns edits after sprint 0 to the orchestrator at `.plans/epics/go-rewrite/epic.md:129`–131. The Contract team owns them during sprint 0. |
| 7. State layout | The file names distinguish key, enrollment marker and shared store. The user-flow test confirms the rename, and the installer handoff is incomplete. See logic-r3 11. |
| 8. Item 2 | Syncing all resolved ancestors on each open fixes B. Returning a copy and deriving claim paths from the held folder fixes M. The release callback still needs to be idempotent for a claim-and-reclaim race. |
| 9. Item 2 done-when rows | Rows 4 and 6 now check the claimed folder and the previously unsynced ancestor. Nothing found in those assertions that conflicts with B or M. |
| 10. Item 3 | A sync followed by `Load` fixes the uncertain-key path. The fold should spell out that same reload as its next step. Expired recovery remains open in logic-r3 12. |
| 11. Item 5 | E's refused directories are no longer offered to children and each link step is examined. The file and link rules and the fixtures still conflict. See logic-r3 09–10. |
| 12. Item 5 tasks and risks | The two timeout fixtures cover N and its remaining-time case. The updated risk row still overstates helper trust and the access-list fixture does not isolate that check. See logic-r3 09–10. |
| 13. Shared store and receipts | C's separate mark and D's split prune have been removed. A, F, G, J, K, O and P must be checked under the new failure states. See logic-r3 01–07. |
| 14. Store tasks and tests | S covers boundaries numerically and Q can check no replay commit. The tests still bless deletion after a divergent restore and omit real queued-write failure. See logic-r3 01 and 04. |
| 15. Sprint 3 fold | A current-stream sender, retry delay, result mapping and producer ownership are named. The contract, restore, freshness, retry publication and state path still need the agreements in logic-r3 01, 04, 06–08 and 11–12. |
| 16. Post-build checks | The new rows cover an actual disk sync failure and redelivery during a running attempt. Row 5 needs independent fixtures for each owner error because a foreign-owned state folder would stop the test before `OpenStore`. Row 15 should name the exact sequence comparison that the script already makes. |
| 17. Traceability | The brief's buffer size and full-buffer rules still point at item 7 at plan lines 913–914. Item 6 now owns both. The same renumbering leaves item 3 advertising parallel work with a nonexistent item 8 at line 386. |
| 18. Execution sizing | The numbers total 32 launches after folding to seven items. The merge order and cross-team decisions remain required before those units can run as described. See logic-r3 03, 06 and 08. |
| 19. Attack log | The log records the replaced mechanism and dependency. Its claim each old outbox finding is fixed is too broad. The sprint cut marked open at line 1001 was confirmed by the owner in `.plans/epics/go-rewrite/epic.md:103`. |
| 20. Decisions 55–71 | The new records replace the event files, clock file and old marker. Explicitly mark earlier decisions 9, 15, 36, 39 and 46 as historical where their names, options or states changed. Resolve the policy contradictions in logic-r3 01–02 and 06–12. |
| 21. Scouting | The addition correctly identifies the creation and sync paths in the tested fallback and the script's state marker. The v1.5.0 build is still unverified in this record. Its event and clock notes describe the pinned draft, and the old file targets and dependency at scouting lines 62–66 and 98 need updating. The live probe adds the zero-length existing-file case in logic-r3 05. |

### Changed writers and readers

| Name or boundary | Writer and reader check |
|---|---|
| `Folder.Claim`, `Paths`, `StoreFile`, temporary names and claim errors | Item 2 owns the private paths and removes temporary names. Item 6 owns the claim lifecycle and writes the initial store. Both store items read the file. The link supervisor must open the folder before the store and close it afterwards. The claim writer and reader are named. |
| `client.db`, its three buckets and `pruned_through` | Item 6 owns creation, metadata and event transactions. Item 7 owns receipt transitions and retention. Both depend on the same failure and close state. Move the open-time repair ownership as in logic-r3 03. The helper test names already have distinct owners. |
| `IdentityFile`, `Paths.Identity`, `identity.json`, `clientPublicKey` and `serverPublicKey` | Item 2 publishes the name and Sprint 3 Client writes and reads the document. `deploy/test/checks.sh:520` consumes it. The Stack installer's existence check, templates and stand-ins are also readers or writers of this boundary. They must use the same resolved location and marker. |
| `MarkSent`, `Acked`, `Reconcile`, `Hello.acked_event_number`, `Welcome.last_event_number` and `EventAck.through` | The Store owns local marks, the Sprint 3 Client sender reads and writes them, the Contract team owns fields, and Server owns the committed cursor. The pinned contract's greeting raises the server mark at `.plans/plans/go-s0-contract/draft/link.proto:199`. The working copy's `Hello` at line 195 has removed the field. Resolve the cursor and rollback policy jointly. |
| `Receipt.LatestAttempt`, `ReceiptUnsupported`, `ReceiptInterrupted` and `ErrResultTooLarge` | Item 7 writes them. Sprint 3 Client maps them to wire results, and Server validates the attempt against the current stream and stores the outcome. The Contract draft defines both wire outcomes at `.plans/plans/go-s0-contract/draft/commands.proto:80`–84. |
| `StoreOptions.Now`, `Welcome.server_time` and proposed `PruneNow` | Server supplies the time sample, Contract owns the field, Sprint 3 Client publishes the estimates, item 7 consumes them for admission and retention. The pinned draft exposes a sample, the working copy removes it, and the local plan has no separate retention option. See logic-r3 06. |
| `Event.happened_at`, the outage report, `link_event_buffered` and `sequence` | The Client producer writes the capture time and log after commit. Client Link encodes the event. Server consumes the report and logs committed sequences. The user-flow script consumes both logs. The proposed load event and timestamp ordering are missing shared definitions. |
| `trustedFolder`, `trustedProgram`, `hasAccessList`, `PATH` and the repository path | Item 5 owns the unexported trust names and child environment. Its result stays local except for harness name and version. Item 1 resolves `Settings.RepositoriesFolder`. Sprint 3 Client supplies `Facts.workspace` and wave 2 creates the clones. Server and website read the reported path, whose maximum is in `.plans/plans/go-s0-contract/draft/events.proto:70`. |

## Answers to the logic lens

1. **Order, ties, gaps, wrap and restart.** The transaction orders local `Append` calls from 1. Lower acknowledgements are idempotent. An ordinary reconnect may use an authenticated committed prefix. The same number under a divergent history is ambiguous. Define the final usable sequence, refuse before addition if the server number or moved count would cross it, and give a permanent `ErrSequenceExhausted` a stop or repair decision in the fold. Attempt 0 is invalid, and larger running attempts win without local arithmetic. The server's draft prohibits attempt wrap at `.plans/plans/go-s0-contract/draft/commands.proto:31`. Producer timestamps have neither a stable order across clock correction nor a tie policy.
2. **Failures and repair.** A failed folder sync denies use until a successful reopen. A failed store commit denies new work and calls for a restart, subject to the admission race. A failed `Finish` has two valid disk outcomes. Expired join recovery and corrupt existing-store preflight still need rules. Server unavailability preserves pending rows, assuming the report is already committed and flow resumes with matching history.
3. **Interacting rules.** The tables below identify the ambiguous cells. The grouping is by input a builder can observe, so the same visible marks under different histories appear in separate rows.
4. **Zero and empty values.** `Acked` and `Sent` start at 0, and `Next` starts at 1. Read limit 0 is empty, and the `Read(4, 10)` assertion at plan line 688 makes `from` inclusive. Empty payload admission and wire consumption should agree with the unknown-report rule. Numeric option 0 means default. Command attempt 0 and an empty identifier or expiry refuse. No clock denies `Begin` and `Prune`. Decision 0 never grants work. An existing zero-length store differs from a missing store. An absent prune mark differs from malformed metadata. An empty cleaned search path needs an explicit no-helper-search meaning.
5. **Concurrent actors.** The folder lock and claim serialize normal openers. The store must coordinate producer appends, sender acknowledgements, command `Begin` and `Finish`, pruning and close. The command receiver and result publisher must share the current-stream attempt association. If attempt 2 arrives before `Finish` commits, label the result 2 on that stream. If it arrives after `Finish`, return terminal replay on its arrival. If a stream is replaced without a new attempt, preserve the terminal result for replay rather than publish it under an attempt that stream never supplied.
6. **Risky mutations.** In item 2 the named tests exercise the new ancestor and claim mutations, but a stale release of a prior claim can clear a new claim without failing those tests. In item 3 the new uncertain-key path has no unit mutation. Ignoring the caller's failed directory sync would pass the listed item 3 tests. In item 6 the injected error tests stop sequential writes, and the named acknowledgement mutation is killed by the explicit unsent bound. A writer already waiting for bbolt can pass. In item 7 the named no-expiry and wrong-attempt mutations are covered. The prune mutation is only guaranteed to fail if the hook explicitly fails between the two commits a mutant would make. Inject both possible disk outcomes and assert exact receipt and watermark pairs.
7. **Cheapest wrong implementation per item.** Item 1 can require identical service and shell environments while all its resolver tests pass. Item 2 can allow a stale release callback to drop a later claim. Item 3 can omit the caller's resync and reload after uncertain placement. Item 4 did not change. A caller can discard `ReadLoad` errors and manufacture a zero offline measurement while its item 4 tests pass. Item 5 can filter folders and run a helper whose file is outside the stated trust policy. Item 6 can skip checking interior keys and keep passing the listed damage fixtures. Item 7 can rewrite a malformed receipt on every open, and its existing reopen assertions will still pass.
8. **State shapes that rule out faults.** Match event deletion to stable history and payload identity. Preserve command deduplication across the allowed repair paths. Give one owner authority to admit writes and record failure. Validate existing records before writing. Use an identifier-scoped retirement record and explicit started, terminal and damaged receipt forms.

### Cursor and write failure cases

| Condition | Patched outcome | Result of review |
|---|---|---|
| Live acknowledgement at or below `acked` | No change. | A harmless repeat. |
| Live acknowledgement above `acked` through `sent` | Deletes the prefix. | Safe when the current stream and its offered prefix confirm it. Pre-send intent alone leaves this cell unproved. |
| Live acknowledgement above `sent` | Refuse and reconnect. | The original above-bound hole is closed. |
| Welcome below `acked` | Refuse and retry. | Preserves pending rows. Restoring an older server has no agreed recovery if the greeting field is absent. |
| Welcome at `acked` | No change. | Coherent. Includes zero with a new store. |
| Welcome above `acked` through `sent` | Deletes the prefix. | Correct for a matching history. A divergent restored history loses rows. |
| Welcome above `sent` and below the ceiling | Deletes the sent range and moves the remaining rows. | A fresh empty store can advance safely. A nonempty restored store can discard a different payload or resend one that the server already applied. |
| Welcome or the moved range reaches the sequence ceiling | Refuses the transaction in the tested case. | Decide empty-queue and first-next-number boundaries before doing arithmetic. The caller's permanent failure handling is missing. |
| First failed commit is absent or present on disk | Reopen decides which version survived. | Sound if later operations cannot leave the failed run. The queued-writer cell is open. |

### Receipt and startup cases

| Condition | Patched outcome | Result of review |
|---|---|---|
| Invalid command, zero attempt or no clock | Error and no execution. | Define the precedence when both validation and clock fail. |
| No receipt and valid future expiry above the prune mark | Persist `started` and execute once. | Safe for one uninterrupted history. An earlier store restore erases the duplicate evidence. |
| No receipt, true expiry reached | Replay `expired`. | Define the synthetic or persisted receipt. |
| No receipt, expiry still future but at or below the prune mark | Replay `expired`. | Correct for a retired identifier. Refuses a different new identifier after a clock correction. |
| `started` and greater attempt before finish | Raise the attempt and return running. | The result must go only to the stream that supplied that attempt. |
| `started` and equal or lower attempt | Return running with no attempt change. | Keep the original effect and the highest established attempt. |
| Terminal receipt at any valid attempt and expiry | Replay the saved outcome without a write. | Coherent. The server ignores old attempts, and the handler should avoid publishing a result to a replacement stream with no matching attempt. |
| Failed finish and durable row still `started` | Reopen turns it into `interrupted`. | Specify canonical finish metadata before further validation. |
| Failed finish and durable row already terminal | Reopen retains it. | Contradicts the unconditional interruption promise. |
| Missing store, no prior enrollment activity | Initialize through a temporary file and sync. | Coherent if only a complete store is linked. |
| Existing zero-length or inconsistent store | Refuse and preserve the evidence. | Literal open can write pages before refusal and first/last inspection cannot prove a gap-free queue. |
| Key exists, marker missing, token still valid | Reuse the key. | The draft's same-key replay makes this coherent after the key directory sync. |
| Key exists, marker missing, token expired | Operator removes a half-joined record. | The record cannot always be identified. |
| Marker exists with uncertain sync | Resync before trusting it. | Define the caller's validation and repair of a present invalid marker before accepting it or asking to rejoin. |

## Post-build checks

| Check | Scenario and expected result | Evidence and responsible area |
|---|---|---|
| PB1 | Cut power during initial placement and in a running store on Linux and macOS. After restart, a committed event remains pending or acknowledged, and every command effect occurs at most once. An existing damaged file is retained verbatim. | Compare the durable store and effects ledger before and after. Client storage. |
| PB2 | Cause a real disk sync to fail while a producer, command receiver and sender are active. The process stops without another store transaction or send after failure publication. Reopen settles the ambiguous operation from disk. | Record operation times, resulting rows and received event identities. Client storage and link. |
| PB3 | Reconnect a real client after buffering three distinguishable reports. Verify the server applies the exact list once in order. Repeat with a backward machine wall-clock correction and verify the most recent valid facts still win. | Capture client commit and server application logs, plus the displayed facts. Client and Server link. |
| PB4 | Reconnect a real stream while a command runs, then race its completion with the higher attempt. The command runs once and the server accepts the terminal result on the current attempt. | Record the command identifier, attempt and one effect. Client and Server link. |
| PB5 | Run normal linked harness installs and the approved access-list policy on a real Mac, then exercise the Linux ACL fixture. The accepted harnesses are found and a refused helper fixture never starts. | Collect scanner results and the controlled fixture's start markers. Client harness scan. |
| PB6 | Install and re-run the service with the shell and service manager given different state variables. Both the supported service launch and manual entry point use the documented enrollment record, and the rerun registers no additional machine. | Compare resolved paths and machine identifiers. Stack installer and Client enrollment. |

## Grade

**4 out of 10.** 3 blockers, 9 majors and 0 minors.
