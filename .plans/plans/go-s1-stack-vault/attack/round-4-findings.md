# Round 4 findings for PostgreSQL and OpenBao

## Not reviewed before the build

| Finding | Changed place | First post-build check |
|---|---|---|
| logic-r4 1 | [Work item 4](../plan.md), in-container extraction and login input. | PB6 requires publication without `jq` or external network access and refusal of a truncated response. |
| logic-r4 2 | [Work item 4](../plan.md), role read before the identifier list. | PB7 requires one identifier on first start and preserves the evidence of an interrupted issuance. |
| logic-r4 3 | [Contract for sprint 2](../plan.md), web migration row. | PB8 runs the three web launcher checks using the revised contract. |

## Finding outcomes

The [logic review](round-4-logic.md) examined the fold at `11ee059d`. The [source checks](../scouting.md#round-4-sources-and-build-checks) cover the pinned commands and the adjacent plan readers.

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| logic-r4 1 | major | D10, with R9 | fixed | The existing setup job saves raw initialization output, extracts the token with `sed`, and reads both server identifiers through `bao`. The job uses its existing `vault` network for OpenBao. |
| logic-r4 2 | major | D10 | fixed | A successful role read and the pinned empty-list result allow the first issuance. Existing accessors and failed probes stop it. |
| logic-r4 3 | major | plan | fixed | The contract gives Web its ordinary disposable fixtures, removes its reset test and replaces stale database and rollout references. |

Both setup majors touch the same mechanism. The two files record distinct facts that still match the needed startup behavior. The failures came from assigning parsing to the host and conflating a known empty list with a failed probe. The fixes use the existing job and OpenBao commands.

## Points and stuck statements

| Statement | Findings | Points |
|---|---|---|
| D10 | logic-r4 1 and logic-r4 2 | 6 |
| plan | logic-r4 3 | 3 |
| Total | Three distinct majors | 9 |

There are no duplicates and no stuck statements. D10 has six points and falls below the 12-point threshold. R9 is a secondary reference for logic-r4 1 and adds no points.

## Patched plan sections

| Plan heading | Change |
|---|---|
| Overview, Key decisions and Workstreams | Tighten the summary and state the boundary on restoring either store to an earlier state. |
| Work item 4. Initialize OpenBao and publish one server credential | Specify the job's own tools and the ordered role and identifier checks. |
| Contract for sprint 2 | Give Web the replacements for the removed names and stale test references. |
| Work item 6. Verify first start and restart of the stack | Remove the host parser from startup prerequisites and document the boundary of manual cleanup. |
| Post-build checks | Add PB6, PB7 and PB8 for the three unreviewed fixes. |
| Attack log | Link this register. |

## Top finding and reviewer verdict

The top finding was the major logic-r4 1. The job's isolated image and the host parser requirement could not both be followed.

| Reviewer | Verdict | Grade out of 10 | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| [Logic](round-4-logic.md) | REVISE | 6 | 0 | 3 | 0 |
