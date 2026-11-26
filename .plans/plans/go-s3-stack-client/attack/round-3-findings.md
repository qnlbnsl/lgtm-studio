# Round 3 findings of the plan `go-s3-stack-client`

The round had two reviewers: logic and threat.
They reported 18 findings: no blocker, 12 majors and 6 minors.
The fold grouped them into 12 groups, named G1 to G12, and verified each against the plan, the neighbouring plans and the checkout.
By the orchestrator's notes, each fix is the smallest change that removes the defect, and no mechanism was added. `plan.md` stays at 398 lines.

## Every finding

A finding that repeats another names the first finding of its group.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| logic-r3 01 | major | D16 | G1 | fixed | The proof of a start before login is a person who sees the machine online from outside the fixture user, the observer the threat reviewer offers. The comparison of sessions and `connectedAt` is removed. |
| threat-r3 03 | major | D16 | G1 | fixed | Repeats logic-r3 01. |
| logic-r3 02 | major | C6 | G2 | fixed | Gate G1 uses the installer's Compose rule, and gate G4 states the rule itself, names check C2 at `deploy/test/checks.sh:378`, and reads the port in C1 after the launcher. |
| threat-r3 02 | major | C6 | G2 | fixed | Repeats logic-r3 02. |
| logic-r3 03 | major | D19 | G3 | fixed | With an installed unit or agent, the folder comes from it, and another override is refused before anything is done. `--remove` ignores the override. |
| threat-r3 01 | major | D19 | G3 | fixed | Repeats logic-r3 03. |
| logic-r3 04 | major | C9 | G4 | fixed | The fixture record is written before the install, with the address and `COMPOSE_FILE`. The machine comes from `identity.json`. Cleanup works from any earlier stop and takes `not_found` as removed. |
| logic-r3 05 | major | C6 | G5 | fixed | Gate G5 names a website writer for the token adapter and the dialog, the token's address, a terminal with the installed client on `PATH` and a new state folder, and the cleanup. |
| threat-r3 07 | major | C6 | G5 | fixed | Repeats logic-r3 05. |
| threat-r3 04 | major | D19 | G6 | post-build check | Protection against another user of the host, by note 2 of the orchestrator. Post-build check PB3 records the result, and the result file asks for an issue. |
| threat-r3 05 | major | R9 | G7 | fixed | Gate G4 states the script's credential rules itself, and the live test turns tracing off, passes the body on standard input, and signs out in a trap. |
| logic-r3 08 | minor | R9 | G7 | fixed | Repeats threat-r3 05. The traced run with a made-up password is part of post-build check PB2. |
| threat-r3 06 | major | R9 | G8 | fixed | Every Compose call of the installer and gate G1 discards Compose's standard error. `refuses_docker_failure` asserts that a marker on that stream is not printed. |
| logic-r3 06 | minor | D16 | G9 | fixed | The unit uses `Type=exec`, as prescribed. |
| logic-r3 07 | minor | plan | G10 | fixed | Post-build check PB1 stops the client's process group at a barrier and adds the case in which the client finishes. |
| threat-r3 08 | minor | D16 | G10 | fixed | Repeats logic-r3 07. |
| logic-r3 09 | minor | C6 | G11 | fixed | The stand-in client also records the macOS agent file, and the macOS test asserts its absence at the join and after a failed join. |
| threat-r3 09 | minor | D7 | G12 | fixed | The guide says to remove the old machine on the Machines page and to keep the moved folder private until it is deleted. |

## Points by statement

A group counts once, at its highest severity and under the statement of its first finding. A minor is 1 point, a major 3 and a blocker 9.

| Statement | Groups | Points |
|---|---|---|
| C6 | G2 (3), G5 (3), G11 (1) | 7 |
| D19 | G3 (3), G6 (3) | 6 |
| R9 | G7 (3), G8 (3) | 6 |
| D16 | G1 (3), G9 (1) | 4 |
| C9 | G4 (3) | 3 |
| plan | G10 (1) | 1 |
| D7 | G12 (1) | 1 |
| Total | 12 groups | 28 |

## Stuck statements

None. A statement is stuck with at least 12 points and at least 30% of 28. C6 has 7 points, which is 25%.

No two groups of major findings sit in one mechanism of the installer. G1 and G4 both sit in the live test's reboot fixture, and the fix of G1 removed its comparison of sessions instead of adding a record of logins.

## Patches

The next round attacks only these places of `plan.md`.

1. "Workstreams", the second workstream's second paragraph.
2. Work item 2: steps 6 and 9 of "Behaviour", "The unit template", the stand-ins of the `go` and `docker` shims, and the tests `refuses_docker_failure` and `rerun_uses_state_folder_of_installed_unit`.
3. Work item 3: the first bullet of "Behaviour" and the test `installs_macos_launch_agent`.
4. Work item 4: the third and fourth bullets of "How the test runs", the test `live_online_after_reboot`, and "The guide".
5. The risk table of WS-2, its first row.
6. "The sprint gate", rows G1, G2, G4 and G5.
7. "Post-build checks", rows PB1, PB2 and PB3.
8. Traceability, the rows of the findings and of D16 and D19, and the attack log.

## Top finding

G3, a major: a second run with another state folder could join a second machine and replace the working service's identity. Both reviewers found it alone. The fold makes the installed service's folder win.

## Verdicts and grades

| Reviewer | Verdict | Grade | Count |
|---|---|---|---|
| logic | REVISE | 5 of 10 | 0 blockers, 5 majors, 4 minors |
| threat | REVISE | 5 of 10 | 0 blockers, 7 majors, 2 minors |
