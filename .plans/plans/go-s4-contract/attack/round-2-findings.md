# Sprint 4 contract findings from round 2

The two reviewers read [the frozen round 2 plan](plan-v2.md) at `1cf3deb0`. The fold checked their findings against `4699ad06`. The later round 1 edits already required replay acknowledgements before `SetState` in **Run gate and result fence** and **Post-build checks**. This answers the frame-order part of logic-r2 P2. The fold adds its precise approval timing. All thirteen numbered findings still applied.

D labels number foundation decisions, R labels number foundation rules, and C labels number foundation checks.

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r2 01 | Major | C11 | fixed another way | The requested rule uses a later successful ready admission, so incumbent traffic cannot erase evidence of a refusal behind the mark. |
| architect-r2 02 | Major | R7 | fixed | Ask Client and Stack to prove one local instruction and a terminal stop through both installed supervisors. The dependent plans choose how to convey the stop. |
| architect-r2 03 | Major | R4 | fixed | Projects and Store must preserve the saved result once replay wins the retirement race, with a test paused between receipt lookup and result read. |
| architect-r2 04 | Major | D5 | fixed | Withhold account-relative Secure Shell origins and exercise a cross-machine pair with equal relative paths and different first commits. |
| architect-r2 05 | Major | R9 | fixed | Require the application startup path attributes to pass the shared filter or be omitted. Check the actual process log and resulting reports. |
| architect-r2 06 | Minor | R10 | fixed | Link and Folders supply fixture boundaries before database commit and before receipt completion. The orchestrator proves what survived each interruption. |
| logic-r2 01 | Major | R7 | fixed | Only the latest admitted pair can reconnect. Superseded runs stay refused through restart and at equal boundaries. |
| logic-r2 02 | Major | R4 | fixed | Repeats architect-r2 03. Protect the original result when replay wins and apply fresh-request rules when retirement wins. |
| logic-r2 03 | Major | C11 | fixed another way | Repeats architect-r2 01. Use later readiness or removal as the clear condition. The Machines field stays boolean. |
| logic-r2 04 | Major | D5 | fixed | Projects must prove a single orphan removal across machines and the two valid outcomes of attachment racing cleanup. Projects owns the locking choice. |
| logic-r2 05 | Major | D5 | fixed | Repeats architect-r2 04. Send the account-relative origin as absent with the omission flag, and keep comparison for absolute forms. |
| logic-r2 06 | Minor | R7 | fixed | Require the existing database error on an uncertain diagnostic commit, preservation of the incumbent and an atomic flag and feed outcome. |
| logic-r2 07 | Minor | R8 | fixed | At attachment, an unknown explicit target returns the fixed `not_found` reply even when the resolved folder is already saved elsewhere. |

## Statement points and stuck rule

Count each duplicate group once at its highest severity.

| Statement | Counted findings | Points |
|---|---|---|
| C11 | architect-r2 01 | 3 |
| D5 | architect-r2 04, logic-r2 04 | 6 |
| R4 | architect-r2 03 | 3 |
| R7 | architect-r2 02, logic-r2 01, logic-r2 06 | 7 |
| R8 | logic-r2 07 | 1 |
| R9 | architect-r2 05 | 3 |
| R10 | architect-r2 06 | 1 |
| Total | Seven major groups and three minor groups. | 24 |

No statement meets 12 points and 30% in round 2. Round 1 recorded R7 as stuck at 12 of 40. It has 7 of 24 in this round.

The two warning-clear reports identify the same defect. An incumbent's progress cannot establish that a later admission recovered, so the old clear predicate was wrong. The replacement uses only the readiness of a later successful admission or machine removal. The uncertainty case also shows why a diagnostic refusal must never replace an incumbent. The two receipt reports identify one race. Caller-and-key reservation still prevents conflicting requests, but serialization of receipt lookup alone cannot protect the accepted replay. The two repository groups identify different defects. Account-relative text is unsafe evidence of identity, and separate machine locks alone cannot enforce repository-wide orphan cleanup. The fold removes the relative comparison and leaves cross-machine coordination to Projects.

## Plan headings changed in the final fold

1. **What the sprint 4 contract enables** describes the flag and leaves Machines display to the owner.
2. **Item 2. Revision 2 fields, folders, and comments** updates **Run gate and result fence**, **Buffer loss** and **Outgoing text**. The contract assertions cover supersession and origin omission. Receipt and supervisor choices stay with Client.
3. **Item 3. Repository calls and watch records** updates **Replay and settlement**, **Folder ownership and origin identity** and **Fixed refusals for the website**.
4. **Item 4. Additive website fields and generated consumer checks** retains the boolean and its additive tests.
5. **Requests to the dependent plans** and **Post-build checks** specify the warning lifecycle, replay retirement, cross-machine invariants, startup filtering, installed stop and fixture boundaries.
6. **Attack log** records both rounds. This is the final fold before build.

## Top finding and reviewer verdicts

The top finding is the premature recovery-flag clear, a major (architect-r2 01 and logic-r2 03). Both reviewers reached it independently. The Logic reviewer ranked stale-run admission highest (logic-r2 01), also a major.

| Reviewer | Verdict | Grade | Reported findings |
|---|---|---|---|
| Architect | Revise | 6 out of 10 | 0 blockers, 5 majors and 1 minor. |
| Logic | Revise | 6 out of 10 | 0 blockers, 5 majors and 2 minors. |
