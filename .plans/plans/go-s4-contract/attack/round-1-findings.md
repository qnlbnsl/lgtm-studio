# Sprint 4 contract findings from round 1

Reviewed on 2026-11-27 against [the frozen plan](plan-v1.md). The table retains each report's statement label. The labels R3, R4, R6, R7, R8, R9 and R10 in this register refer to the numbered foundation rules when the finding concerns those rules. The sprint checks use their own R numbering in the plan. D labels are decision numbers and C labels are foundation check numbers.

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r1 01 | Major | D11 | fixed | Register all three procedures in the router's dispatch and preflight tables, with a signed-in cross-origin check. |
| architect-r1 02 | Major | R4 | fixed | Repeats logic-r1 04. The original Store receipt reserves the caller and key for the operation's replay window. |
| architect-r1 03 | Major | R7 | fixed | Repeats logic-r1 01. The fixed server diagnostic appears on the Machines page while the existing refusal stays retryable. |
| architect-r1 04 | Major | C7 | fixed | Let a waiting client replay the old prefix before its gated `SetState`. Existing approval rules decide which reports apply. |
| architect-r1 05 | Major | R6 | fixed | Repeats logic-r1 05. Store and replay the standing command's first result watermark while retaining its revision and expiry rules. |
| architect-r1 06 | Major | R3 | fixed | Land the reviewed identifier-returning enqueue and transactional hooks before Projects builds, and preserve the Machines queue surface. |
| architect-r1 07 | Major | plan | fixed | Repeats logic-r1 06. Folders writes safe origin text and Projects compares the specified normal form. |
| architect-r1 08 | Major | R9 | fixed | Repeats critic-r1 3. Specify the credential inputs and use the shared filter before existing and new report or log writes. |
| architect-r1 09 | Minor | R8 | fixed | Repeats logic-r1 07. Pin exact codes and sentences in the schema comments and test them at the real handlers. |
| architect-r1 10 | Minor | R10 | fixed | Assign the in-process tests and real PostgreSQL invocation to the orchestrator, with the installed upgrade assigned to Stack. |
| critic-r1 1 | Major | C11 | fixed another way | Repeats logic-r1 01. Use the authenticated Machines watch for the recovery instruction because the machine is refused before Welcome. |
| critic-r1 2 | Major | R6 | fixed | Repeats logic-r1 06. Specify origin equivalence and check equivalent and unequal pairs in Projects. |
| critic-r1 3 | Major | R9 | fixed | Put existing Facts and inventory under the filter and check the required legacy suffix before replay without renumbering it. |
| critic-r1 4 | Minor | R6 | fixed | Repeats logic-r1 05. Include the original watermark on a retried state command after a newer event. |
| critic-r1 5 | Minor | R8 | fixed | Repeats logic-r1 07. Pin refusal mappings and require a replacement snapshot after an aborted watch. |
| critic-r1 6 | Minor | plan | fixed | Use the readme assertion for red and the existing comparator for characterization. Bypass the comparison to break all four named rejections. |
| critic-r1 7 | Minor | C3 | fixed | Start fresh browser contexts, sign in through each transport and observe calls at their respective boundaries. |
| logic-r1 01 | Major | C11 | fixed another way | Publish an additive Machines flag after an authenticated history contradiction. The owner can recover without adding a link reason. |
| logic-r1 02 | Major | R7 | fixed | Keep the pending and ready pair durable and immutable for the same run. Replacement cannot move a pending boundary backward. |
| logic-r1 03 | Major | R7 | fixed | Limit both callbacks to open operations and acknowledge an eligible late report without a Projects change. |
| logic-r1 04 | Major | R4 | fixed | Keep one Store-owned caller-and-key reservation through operation retirement by holding the original receipt. This removes the Projects fallback. |
| logic-r1 05 | Major | R7 | fixed | Save the first watermark for `SetState` and audit every result path. Preserve the standing command's longer offer life. |
| logic-r1 06 | Major | D5 | fixed | Return the stored folder before comparing origins for a new attachment. Pin the comparison and omission rules. |
| logic-r1 07 | Minor | R8 | fixed | Check the method comments against an independent table, exercise the handler mapping, and use the generated discovery name. |
| threat-r1 01 | Major | R7 | fixed another way | Repeats logic-r1 01. The Machines instruction handles server-detected loss. The existing local fault still stops a client that detects loss. |
| threat-r1 02 | Major | R7 | fixed | Seal the old process's watermark durably before a new process appends. Grant legacy zero only to valid revision 1 receipts or genuinely empty history. |
| threat-r1 03 | Major | R9 | fixed | Limit folder reading to local read-only facts. Verify that an inert side-effect sentinel stays untouched. |
| threat-r1 04 | Major | R9 | fixed | Repeats critic-r1 3. Omit origins with ambiguous or credential-bearing query or fragment text and assert the filter across stored and visible surfaces. |
| threat-r1 05 | Major | D11 | fixed | Repeats architect-r1 01. Check listed and unlisted origins through the real router for each procedure. |
| threat-r1 06 | Minor | R8 | fixed | Repeats logic-r1 07. Use fixed malformed-body text on the unary and streaming routes, with no operation created. |

## Statement points and the stuck rule

Each group counts once at its highest reported severity, under its canonical finding's statement. The history group goes under C11 because event replay is the behavior that fails. The standing receipt group stays under R7 because the first result must survive a restart. The origin group goes under D5 because both the saved record and identity proof govern the decision. The shared filter group includes the unsafe remote input as a case. Read-only inspection is a separate exposure even with a complete filter.

| Statement | Counted findings | Points |
|---|---|---|
| C3 | critic-r1 7 | 1 |
| C7 | architect-r1 04 | 3 |
| C11 | logic-r1 01 | 3 |
| D5 | logic-r1 06 | 3 |
| D11 | architect-r1 01 | 3 |
| R3 | architect-r1 06 | 3 |
| R4 | logic-r1 04 | 3 |
| R6 | Duplicate labels only. | 0 |
| R7 | logic-r1 02, logic-r1 03, logic-r1 05, threat-r1 02 | 12 |
| R8 | logic-r1 07 | 1 |
| R9 | critic-r1 3, threat-r1 03 | 6 |
| R10 | architect-r1 10 | 1 |
| plan | critic-r1 6 | 1 |
| Total | 12 major groups and 4 minor groups. | 40 |

R7 is stuck at 12 points and 30% of the round. These groups exposed two durability gaps around reconnect and two gaps around result finality. The fences still serve the approved sprint and retain their existing wire shape. The old plan's ready-only run record and its deferred receipt boundary were insufficient. The fold makes pending admission durable and saves the previous process boundary before another append. The operation is also an open-to-terminal transition, so a late report cannot change the result. The next review should test these boundaries together. The separate caller-and-key fallback was the wrong mechanism because it ran after Store decided who owned the key. The original receipt now supplies the reservation.

## Plan headings for the next round

1. **What the sprint 4 contract enables** gives the owner-facing recovery path.
2. **Item 1. Append rule and history tests** separates baseline characterization, a red assertion, and comparator mutations.
3. **Item 2. Revision 2 fields, folders, and comments** strengthens the source assertions and mutations. Its **Run gate and result fence**, **Recovery on the Machines page**, **Folder facts and byte limits**, and **Outgoing text** paragraphs specify the amended link rules.
4. **Item 3. Repository calls and watch records** adds a refusal assertion. **Replay and settlement**, **Folder ownership and origin identity**, and **Fixed refusals for the website** pin the amended service behavior.
5. **Item 4. Additive website fields and generated consumer checks** adds the Machines flag and makes the two sample probes observe calls after sign-in.
6. **Handoffs across the server and browser** and **Post-build checks** assign the reviewed Go seam, origin handling, and the runtime proof. The acceptance, traceability, sizing and attack tables reflect those patches.

## Top finding and reviewer verdicts

The top finding is the server-only detection of missing event history (logic-r1 01), a major. All four reviewers identified it independently, and three chose it as their top finding. The Machines watch diagnostic is an unreviewed mechanism for the next round.

| Reviewer | Verdict | Grade | Reported findings |
|---|---|---|---|
| Architect | Revise | 6 out of 10 | 0 blockers, 8 majors and 2 minors. |
| Critic | Revise | 7 out of 10 | 0 blockers, 3 majors and 4 minors. |
| Logic | Revise | 6 out of 10 | 0 blockers, 6 majors and 1 minor. |
| Threat | Revise | 6 out of 10 | 0 blockers, 5 majors and 1 minor. |
