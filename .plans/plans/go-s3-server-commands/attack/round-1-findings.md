# Round 1 findings for stored server commands

The round reviewed [revision 1 of the plan](plan-v1.md). D, R and C refer to the numbered decisions, rules and checks of the foundation design. The plan stays at `attack` after this fold.

## Dispositions

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r1 01 | Blocker | R10 | fixed | Item 1 uses a revision 1 manifest for the upgrade, updates `TestBecomeCurrent` and gates on the whole link package. |
| architect-r1 02 | Major | R6 | fixed another way | Items 1, 2, 3 and 4b use a durable marker set after the flush. A stream-local entry marker would still admit a result after a failed flush. |
| architect-r1 03 | Major | R10 | fixed | Item 8 ages a committed claim for the sweep-first case and seeds separate due candidates for lock isolation. |
| architect-r1 04 | Major | R10 | fixed | The combined gate and a client-owned real-store fixture prove the two sides of the clock boundary without a public clock hook. |
| architect-r1 05 | Major | D1 | fixed | The website handoff asks for queue acknowledgement, watch-driven inventory and observations on success and disconnect. |
| architect-r1 06 | Minor | R10 | fixed | The result fixture uses a mismatched machine with the exact stream, command and attempt to isolate the predicate. |
| critic-r1 01 | Major | R6 | fixed | This duplicates architect-r1 02. The persisted marker requires successful dispatch. The skipped-claim and delivered cases prove both outcomes. |
| critic-r1 02 | Major | C8 | fixed | Item 4b caps network-held writers, wakes the next live waiter and exercises the minimum pool. |
| critic-r1 03 | Major | R10 | fixed | Item 4b leaves zero stream credit after Welcome and observes a valid restart at the real fenced callback. |
| critic-r1 04 | Major | R10 | fixed | This duplicates architect-r1 03. The sweep-first fixture ages the claim under the machine lock. |
| critic-r1 05 | Major | R10 | fixed | This duplicates architect-r1 04 for clock ownership. The combined tests also stage both ages for first delivery of the approval notice. |
| critic-r1 06 | Minor | C12 | fixed | The rollout runs C12 separately under the PostgreSQL wrapper. |
| critic-r1 07 | Minor | R10 | fixed | The item command anchors the test names and requires test-level pass events, with missing and skipped tests failing the gate. |
| critic-r1 08 | Minor | plan | fixed | The dispatch record must show measured capacity, the complete units and the next deferred seat or unit. |
| logic-r1 01 | Major | C8 | fixed | This duplicates critic-r1 02. The gate holds real writes and observes a heartbeat and disconnect cleanup with the permits occupied. |
| logic-r1 02 | Major | D16 | fixed another way | Item 3 records the first deferred scan in the cursor and revisits it on the existing result and heartbeat turns. This avoids a new timer or feed event. |
| logic-r1 03 | Major | R6 | fixed | This duplicates architect-r1 02. A claim resets the dispatch marker and a successful fenced send publishes it. |
| logic-r1 04 | Major | R10 | fixed another way | This duplicates architect-r1 03. Separate due rows exercise the actual lock refusal with the real clock. Same-row results and removal prove competing finalization. |
| logic-r1 05 | Minor | R10 | fixed | Items 1 and 3 require the mutation pass. The migration fixture separately checks both attempt bounds and both mismatched terminal pairs. |
| logic-r1 06 | Minor | R7 | fixed | Only the sweep writes `timed_out`. A matching machine reply retains `expired` and the tests distinguish the two. |
| threat-r1 01 | Major | R6 | fixed | This duplicates architect-r1 02. A failed flush or ambiguous marker commit ends the stream and leaves a pending retry. |
| threat-r1 02 | Major | R6 | fixed | The transport captures real flush errors inside the send deadline and checks cancellation after the callback. |
| threat-r1 03 | Major | C8 | fixed | This duplicates critic-r1 02. Capacity is reserved before claiming, and the minimum-pool gate exercises it. |
| threat-r1 04 | Minor | R6 | fixed another way | This duplicates logic-r1 06. A distinct internal timeout value preserves the source without adding a source column. Machine outcomes require a dispatch marker. |
| threat-r1 05 | Minor | R9 | for the owner | The owner still needs to choose the at-rest policy for machine metadata, including pending notice names and retained command metadata. |

## Points and stuck statements

A blocker earns nine points, a major earns three and a minor earns one. Each duplicate group earns its highest severity once. The expiry-source group is charged to R7. The table keeps R6 on the threat report row.

| Statement | Points |
|---|---:|
| D1 | 3 |
| D16 | 3 |
| R6 | 6 |
| R7 | 1 |
| R9 | 1 |
| R10 | 21 |
| C8 | 3 |
| C12 | 1 |
| plan | 1 |
| **Total** | **40** |

R10 is stuck with 21 of 40 points, or 52.5 percent. The failing baseline assertion, sweep staging, flow-control fixture and split clock proof account for 18 of those points. The new observations await a review of the patches and a build.

## Mechanisms examined for overlapping findings

The original durable assignment cannot establish that bytes reached transport. A callback return cannot establish that its flush succeeded either. The fold keeps the machine lock, captures the underlying flush and makes the guarded dispatch marker the sole durable result gate. The cap on network-held writers addresses the separate fleet-wide contention several reports identified. Removing the lock would reopen the send versus removal race. The marker and the shared permit need review together.

The original sweep race required one row to be in disjoint time windows. The fold keeps one database clock and tests the two cases with real due candidates. A separate fixture proves the earlier matching result takes precedence over later maintenance. The fold separates real server cutoff evidence from deterministic client receipt-skew evidence for the same reason.

## Plan patches for the next review

| Heading | Changed material |
|---|---|
| Overview, Workstreams and Build and test conventions | Queued scan handoff, dispatch and writer capacity, feed scope and per-test execution evidence. |
| Item 1. Migration 2 and enqueue | Upgrade fixture, inherited ledger assertion and migration gate. The schema adds a dispatch marker, distinguishes timeouts and clears terminal payloads. |
| Item 2. Match results to the current attempt | Dispatch threshold, exact machine mutation fixture and terminal update. |
| Item 5. Expiry, removal and retention | Timeout disposition and cleared terminal payload. |
| Item 3. Claim one command in a bounded turn | Marker reset, scan deferral and bounded rewind. |
| Item 4a. Honor an earlier response deadline | Error capture from the underlying link flush and the focused transport gate. |
| Item 4b. Fence one command write | Admission budget, flush and marker commit, real response credit and fleet isolation. |
| Item 7. Scan and restart calls | Scan success means queue acceptance. The website handoff preserves the empty reply. |
| Item 8. Server composition and production-route checks | Real sweep order, source-aware outcomes and cancellation at a database gate. |
| Combined acceptance requested from the orchestrator and Post-build checks | Clock ownership, both notice ages, restart bypass, real-stack and cross-team checks. |
| Rollout detail and Execution sizing | Exact C12 invocation, mutation coverage for items 1 and 3 and usable allowance handoff. |

## Owner decision

The Server and Stack owners can retain the current restricted database volume and receipt policy. That keeps deployment unchanged and accepts access by a reader of a raw database copy. They can instead require encryption of the database volume and backups before rollout. That reduces disk-copy exposure and adds key management, recovery and upgrade work. The choice also covers the pending command payloads that carry machine names. The local row drops its payload when the command finishes. Backups and older database pages still follow the storage policy. This is the open metadata decision already recorded by [the stream plan](../../go-s3-server-stream/decisions.md).

## Reviewer verdicts

| Reviewer | Verdict | Grade | Findings reported |
|---|---|---|---|
| Architect | REVISE | 6 of 10 | One blocker, four majors and one minor. |
| Critic | REVISE | 6 of 10 | Five majors and three minors. |
| Logic | REVISE | 6 of 10 | Four majors and two minors. |
| Threat | REVISE | 6 of 10 | Three majors and two minors. |

The top finding was architect-r1 01, a blocker against R10. The existing link test would fail on the first migration 2 merge and the original item had no authority to update it.
