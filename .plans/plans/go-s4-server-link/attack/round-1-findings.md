# Round 1 findings for the server link

The four reviewers examined [the frozen plan](plan-v1.md). In the table, C refers to a foundation check and sprint R refers to a sprint 4 check. Foundation D and R refer to the foundation decisions and rules. The fold date is 2026-11-27.

| Finding | Severity | Design statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r1 01 | Blocker | plan | fixed | Item 5 proves rollback using Facts. Item 6 uses the existing restart settlement route. Item 7 owns the three registered folder callbacks and makes the real folder route available to Projects. |
| architect-r1 02 | Blocker | Foundation R3 | fixed | Keep typed Link hooks and name the Projects adapter, startup error path and real compile and rollback gate. |
| architect-r1 03 | Major | Foundation D5 and sprint R12 | fixed another way | Use the Projects-owned reserved refresh reference and loader to reopen the existing watch. Keep the Store reference cap. |
| architect-r1 04 | Major | Foundation R4 | fixed | Keep immediate legacy results and ordinary sweep feed-neutral. A new stage records a coalesced wake, and the writer rechecks deferred scans. See the higher-severity duplicate threat-r1 01. |
| architect-r1 05 | Major | C12 | fixed | Add both frozen paired callers and land the server switch, client switch and paired fixtures together after a PostgreSQL gate. |
| architect-r1 06 | Major | Foundation R10 | fixed | Item 8 gives the server and paired test writers exact fixture signatures, timing and restart behavior. |
| architect-r1 07 | Major | Foundation R4 | fixed | Store supplies transaction-local reference coalescing for enqueues and new stages, and tests retain ordinary recording limits. |
| architect-r1 08 | Minor | Foundation R2 | fixed | Run the integrated gate with the immutable landed revision 2 pin and real PostgreSQL. |
| logic-r1 01 | Blocker | Foundation R3 | fixed | Duplicates architect-r1 02. Its fixture mismatch also repeats architect-r1 06. |
| logic-r1 02 | Major | Sprint R8 | fixed | Add an independent folder queue discipline using the scan cursor pattern. The real-client check requires both inspections and one folder. |
| logic-r1 03 | Major | Sprint R12 | fixed another way | Duplicates architect-r1 03. The Projects watch reopens from the committed snapshot. |
| logic-r1 04 | Major | Foundation R6 | fixed | Check for the registry row before Link inserts. Test both removal paths and a live settlement that enqueues. |
| logic-r1 05 | Major | C12 | fixed | Duplicates architect-r1 05. The paired replay caller uses the known mark plus one. |
| logic-r1 06 | Minor | Sprint R9 | fixed | Move the registered folder approval proof to item 7 and test the independent send-time gate in item 5. |
| critic-r1 01 | Major | Foundation R4 and sprint R12 | fixed another way | Duplicates architect-r1 03. The separate natural lag test uses several legal Store writes. |
| critic-r1 02 | Major | Foundation R3 | fixed | Duplicates architect-r1 02. |
| critic-r1 03 | Major | Foundation R6 | fixed | Duplicates logic-r1 04. |
| critic-r1 04 | Major | Foundation R10 | fixed | Duplicates architect-r1 01. Its missing approval control also repeats logic-r1 06. |
| critic-r1 05 | Major | C11 | fixed | Duplicates architect-r1 05. It adds the fresh reserved-result caller and the unsupported-revision precedence rows. |
| critic-r1 06 | Minor | C11 | fixed | Freeze both machine-lock orders for a stage competing with a replacement. |
| critic-r1 07 | Minor | plan | fixed | Name all migration fixture files in item 2 and tie raw version 1 seeding to its historical test. |
| threat-r1 01 | Blocker | plan and foundation R4 | fixed | Remove terminal and removal wakes and preserve the frozen feed assertions. New stages and enqueues alone record Link outbox wakes. |
| threat-r1 02 | Major | Foundation D13 and sprint R12 | fixed another way | Duplicates architect-r1 03. Test removal of a flagged machine with a staged command, orphaned rows and shared rows. |
| threat-r1 03 | Major | Foundation R3 | fixed | Duplicates architect-r1 02. Its fixture mismatch also repeats architect-r1 06. |
| threat-r1 04 | Major | Foundation R6 | fixed | Duplicates logic-r1 04. |
| threat-r1 05 | Minor | Foundation R2 | fixed | Enforce the actual binary Event payload size and preserve number-first behavior. Test input whose received and re-encoded sizes differ. |

## Points and stuck statements

Each duplicate group is counted once at its highest severity and charged to one primary statement. Rows with related effects retain those citations above.

| Statement | Counted groups | Points |
|---|---|---:|
| plan | architect-r1 01 and critic-r1 07 | 10 |
| Foundation R2 | architect-r1 08 and threat-r1 05 | 2 |
| Foundation R3 | architect-r1 02 | 9 |
| Foundation R4 | threat-r1 01 and architect-r1 07 | 12 |
| Foundation R6 | logic-r1 04 | 3 |
| Foundation R10 | architect-r1 06 | 3 |
| C11 | critic-r1 06 | 1 |
| C12 | architect-r1 05 | 3 |
| Sprint R8 | logic-r1 02 | 3 |
| Sprint R9 | logic-r1 06 | 1 |
| Sprint R12 | architect-r1 03 | 3 |
| **Total** | **14 groups** | **50** |

There are no stuck statements. Foundation R4 has 12 points, or 24 percent of the round. A statement needs at least 12 points and 30 percent.

The wake findings identify two problems in one mechanism. The unconditional terminal wake would break the established feed contract, and one reference for every enqueue can exhaust the transaction. The plan removes the terminal wake and coalesces the references that the queue still needs. The bulk removal group identifies a separate failure in per-repository fanout. One committed refresh replaces that fanout only for large removals. The plan marks the Projects-owned mechanism unreviewed because the bounded reference uses the existing saved-row loader and needs no new Store or watch change.

## Plan patches for the next check

| Plan heading | Patch |
|---|---|
| Overview and Rollout order | Explain folder serialization, the compatible work items and the joint cutover. |
| Item 1. Transactional part interface and ordered enqueue | Pin the Projects adapter, missing-machine refusal, and one wake per machine per write. |
| Item 2. Upgrade the link schema | List the historical and current migration fixtures. |
| Item 3. Revision 2 authentication and run admission | Sequence the fixture prerequisite before the integrated switch. |
| Item 5. Permit-free replay and ready-run transition | Use Facts for the ready rollback, and prove both claim and send gates. |
| Item 6. First eligible result, eventual settlement and removal | Preserve frozen feed behavior, give settlement a built route, cover both admission race orders and pin bulk removal to Projects. |
| Item 7. Folder routing, queue and limits | Own registered folder callbacks, approval, folder serialization and real received byte limits. |
| Item 8. Application and paired-program fixture entry points | Publish the one shared fixture table and independent early test. |
| Sprint 3 frozen test changes | Update both paired callers and preserve the unsupported-revision precedence. |
| Integrated candidate gate and Post-build checks | Verify the pinned contract, full paired candidate and real runtime scenarios. |
| Execution sizing and Attack log | Align dependencies, cutover and review evidence. |

## Top finding and reviewer verdicts

The top finding was **architect-r1 01, blocker**. The replay work item depended on a folder route supplied by its own successor. A build could not satisfy the work-item acceptance test in that order.

| Reviewer | Verdict | Grade out of 10 |
|---|---|---:|
| Architect | Revise | 5 |
| Critic | Revise | 6 |
| Logic | Revise | 5 |
| Threat | Revise | 6 |
