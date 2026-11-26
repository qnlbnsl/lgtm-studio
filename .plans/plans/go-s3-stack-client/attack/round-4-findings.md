# Round 4 findings of the plan `go-s3-stack-client`

The round had one reviewer, logic. It reported 5 findings: no blocker, 5 majors and no minor.
This is the last fold of the plan. No reviewer reads it before the build.

## Not reviewed before the build

Each place below is marked `UNREVIEWED FIX` in `plan.md` and has the post-build check named beside it.

| Place in `plan.md` | Finding | Post-build check |
|---|---|---|
| Work item 2, step 11 of "Behaviour": an installed unit whose folder holds no identity is refused. | logic-r4 03 | PB6 |
| Work item 2, the stand-ins, the `docker` shim's `SHIM_EXEC_FAIL`, and the test `refuses_docker_failure`. | logic-r4 05 | PB8 |
| Work item 2, the test `rerun_uses_state_folder_of_installed_unit`, its fourth and fifth runs. | logic-r4 03, logic-r4 04 | PB6, PB7 |
| Work item 4, "How the test runs", the fourth bullet: the state rules of `--cleanup` and the use of the recorded `COMPOSE_FILE`. | logic-r4 01 | PB4 |
| "The sprint gate", row G1: standard error captured and reported as empty or not. | logic-r4 05 | PB8 |
| "The sprint gate", row G4: the overlay of check C2 and the shared address file. | logic-r4 02 | PB5 |

## Every finding

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| logic-r4 01 | major | C9 | G1 | fixed | `--cleanup` now refuses only a unit of another folder, works without a record or a unit, reports a key without an identity as `not ok` for the person, runs its exit test only after a running service, and both later parts use the recorded `COMPOSE_FILE`. |
| logic-r4 02 | major | C6 | G2 | fixed | Gate G4 states that C2 appends its private overlay to `COMPOSE_FILE` when it is set and uses `-f` twice otherwise, and that every check reads the published address from a file, written in C1 after the launch and in C13 after the recreation. |
| logic-r4 03 | major | D19 | G3 | fixed | A fresh join needs both no installed unit and no identity, on both systems. Decision 32 is closed as superseded by row 52. |
| logic-r4 04 | major | plan | G4 | fixed | The rerun test allows the read-only probe of the service manager, asserts that nothing changed, and runs on Linux and on macOS. |
| logic-r4 05 | major | R9 | G5 | fixed | Gate G1 captures standard error in a private file and reports only whether it is empty. A new stand-in case fails only the token call and checks that its diagnostic is not printed. |

## Points by statement

A minor is 1 point, a major 3 and a blocker 9.

| Statement | Groups | Points |
|---|---|---|
| C9 | G1 (3) | 3 |
| C6 | G2 (3) | 3 |
| D19 | G3 (3) | 3 |
| plan | G4 (3) | 3 |
| R9 | G5 (3) | 3 |
| Total | 5 groups | 15 |

## Stuck statements

None. A statement is stuck with at least 12 points and at least 30% of 15. Each statement has 3 points.

No two majors sit in one mechanism. G1 sits in the cleanup of the live test, which round 3 also touched, and the fix narrows its preconditions rather than adding a record.

## Patches

1. Work item 2: step 11, the stand-ins of the `docker` shim, and the tests `refuses_docker_failure` and `rerun_uses_state_folder_of_installed_unit`.
2. Work item 4: the fourth bullet of "How the test runs".
3. "The sprint gate": rows G1 and G4.
4. "Post-build checks": rows PB4 to PB8.
5. Traceability and the attack log.

## Top finding

G1, a major: an interrupted install could leave the live fixture in a state that its own cleanup refused. The fix lets cleanup run from every stop.

## Verdict and grade

| Reviewer | Verdict | Grade | Count |
|---|---|---|---|
| logic | REVISE | 6 of 10 | 0 blockers, 5 majors, 0 minors |
