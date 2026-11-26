# Round 2 findings of the plan `go-s3-stack-client`

The round had three reviewers: architect, logic and threat.
They reported 32 findings: no blocker, 25 majors and 7 minors.
The fold grouped them into 15 groups, named G1 to G15, and verified each against the plan, the neighbouring plans and the checkout.

## Every finding

A finding that repeats another names the first finding of its group.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r2 01 | major | D16 | G1 | fixed another way | `--rejoin` is removed, so no run stops a working service or moves its folder. Marked UNREVIEWED MECHANISM in "Key decisions". |
| logic-r2 01 | major | D16 | G1 | fixed another way | Repeats architect-r2 01. An interrupted join leaves a key without an identity, which the next run refuses, as post-build check PB1 states. |
| threat-r2 01 | major | D16 | G1 | fixed another way | Repeats architect-r2 01. |
| architect-r2 02 | major | D19 | G2 | fixed another way | The lock is removed with `--rejoin`. Two runs at once by one user are named outside wave 1, and the guide says to run one at a time. |
| logic-r2 02 | major | D19 | G2 | fixed another way | Repeats architect-r2 02. |
| threat-r2 02 | major | D19 | G2 | fixed another way | Repeats architect-r2 02. |
| architect-r2 03 | major | D19 | G3 | fixed another way | The installer never moves a folder, so the check of its names is removed. |
| logic-r2 03 | major | D19 | G3 | fixed another way | Repeats architect-r2 03. |
| threat-r2 03 | major | D19 | G3 | post-build check | Its part on moved folders is gone with G3. Its part on writable parents of the program and the unit is post-build check PB3, and the result file asks for an issue. |
| architect-r2 04 | major | C6 | G4 | fixed | The installer calls Compose as `deploy/up.sh` does, with `-f` by default, and gate G4 names the script's change of `compose` as a prerequisite. |
| logic-r2 04 | major | C6 | G4 | fixed | Repeats architect-r2 04. |
| threat-r2 09 | major | C5 | G4 | fixed | Repeats architect-r2 04. |
| architect-r2 05 | major | C9 | G5 | fixed | The live test refuses an existing fixture folder, passes a new state folder inside it, and its later parts require the unit to name that folder. |
| logic-r2 05 | major | C9 | G5 | fixed | Repeats architect-r2 05. |
| threat-r2 07 | major | C6 | G5 | fixed | Repeats architect-r2 05. |
| architect-r2 06 | major | D16 | G6 | fixed another way | The 5-second watch is removed. A successful run now promises only a started service, and the live test proves the connection. |
| logic-r2 08 | major | D16 | G6 | fixed another way | Repeats architect-r2 06. The finding offers this narrower promise as one of its two options. |
| threat-r2 08 | major | C6 | G6 | fixed another way | Repeats architect-r2 06. The comparison of the address is removed, as in G7. |
| logic-r2 07 | major | C6 | G7 | fixed another way | The text comparison of the address is removed. A changed port needs a new install by hand, which the guide says. |
| architect-r2 07 | major | D16 | G8 | fixed another way | `live_online_before_login` compares the server's `connectedAt` with the sessions of class `user`, and requires at least one. Marked UNREVIEWED MECHANISM. |
| threat-r2 06 | major | D16 | G8 | fixed another way | Repeats architect-r2 07. |
| logic-r2 06 | major | C6 | G9 | fixed | Gate G5 pastes the command of a real Machines page, and names the page's change as its prerequisite. |
| threat-r2 05 | major | C6 | G9 | fixed | Repeats logic-r2 06. |
| threat-r2 04 | major | R9 | G10 | fixed | Gate G4 names the script's change of `signin` and `signin_rejected` as a prerequisite, and every run of the gate uses a password made for it. |
| architect-r2 09 | minor | C6 | G11 | fixed | The stand-in client records whether the program or the unit exists when `join` runs, and the unit's restart lines are asserted on their own. |
| logic-r2 09 | major | plan | G11 | fixed | Repeats architect-r2 09. |
| architect-r2 08 | minor | D17 | G12 | fixed | Item 1 spells `Tokens.Create(ctx, machines.TokenOptions{HostsServer: hostsServer})`, and decision 37 says that the issuer writes the address. |
| threat-r2 10 | minor | D17 | G12 | fixed | Repeats architect-r2 08. The stale row of `scouting.md` is corrected too. |
| architect-r2 10 | minor | D19 | G13 | fixed | The check of tools comes after the branch of `--remove`. Test `remove_needs_no_build_tools`. |
| logic-r2 12 | minor | plan | G13 | fixed | Repeats architect-r2 10. |
| logic-r2 10 | minor | C12 | G14 | fixed | Gate G4 runs inside `deploy/test/with-postgres.sh`, and item 1 names that its database runs in Docker. |
| logic-r2 11 | minor | C6 | G15 | fixed | The stand-in states its three cases: unset gives 8787, empty prints nothing, and any other value gives that port. |

## Points by statement

A group counts once, at its highest severity and under the statement of its first finding. A minor is 1 point, a major 3 and a blocker 9.

| Statement | Groups | Points |
|---|---|---|
| C6 | G4 (3), G7 (3), G9 (3), G11 (3), G15 (1) | 13 |
| D16 | G1 (3), G6 (3), G8 (3) | 9 |
| D19 | G2 (3), G3 (3), G13 (1) | 7 |
| C9 | G5 (3) | 3 |
| R9 | G10 (3) | 3 |
| D17 | G12 (1) | 1 |
| C12 | G14 (1) | 1 |
| Total | 15 groups | 37 |

G11 counts under C6, its first finding's statement, at the major severity of logic-r2 09.

## Stuck statements

C6 is stuck: it has 13 points, which is 35% of 37, so it passes both 12 points and 30%. Its groups are the Compose selection (G4), the address comparison (G7), the page's command (G9), the order test (G11) and the port stand-in (G15). G7 is fixed by removing the comparison, G4 by one rule shared with the launcher, and G9 by a gate on the page's change. No other statement reaches 12 points.

Three groups of major findings sat in one mechanism, `--rejoin` with its lock and its folder check (G1, G2 and G3). Each was found alone by all three reviewers, after the same mechanism drew five majors in round 1. That mechanism was wrong for wave 1: no check and no statement of the design needs it. The fold removed it.

Two more groups sat in the second run's health check, the address comparison and the 5-second watch (G6 and G7). That mechanism promised more than an installer can see. The fold removed it and narrowed the promise.

## Patches

The next round attacks only these places of `plan.md`.

1. "Overview", paragraphs 1 and 3.
2. "Key decisions", the bullets "Nothing is installed before the join succeeds", "The installer joins once and never moves a state folder" and "The installed client dials the stack's own loopback port".
3. "Workstreams", the second workstream's first and second paragraphs.
4. "Rollout order", step 4.
5. "Public names", the rows of the installer and the unit, and the table of names read.
6. "The service, the address and the exit statuses", the first three rows and the last one.
7. Work item 1: "Behaviour", the call of `Tokens.Create`; "Tests written first", the database; "Acceptance"; and the task row.
8. WS-2 "Done when", rows 2c and 2d, and the tasks 2.1 and 4.1.
9. Work item 2: "Risky", steps 1 and 6 to 18 of "Behaviour", the stand-ins, and the tests `installs_linux_user_service`, `asks_server_for_hosts_server_token`, `joins_before_service_starts`, `join_failure_installs_no_service`, `refuses_unfinished_join`, `remove_needs_no_build_tools`, `usage_on_unknown_argument` and "Acceptance".
10. Work item 3: "Behaviour".
11. Work item 4: "How the test runs", `live_rerun_keeps_machine`, `live_online_before_login`, "The guide" and "Acceptance".
12. The risk table of WS-2.
13. "The sprint gate", rows G2, G4 and G5; "Post-build checks"; and "Outside wave 1".
14. Traceability, guardrails and the attack log.

## Top finding

G1, a major: a failed or interrupted `--rejoin` could leave the working client stopped with its identity in an archive. All three reviewers found it alone. The fold removed `--rejoin`.

## Verdicts and grades

| Reviewer | Verdict | Grade | Count |
|---|---|---|---|
| architect | REVISE | 5 of 10 | 0 blockers, 7 majors, 3 minors |
| logic | REVISE | 5 of 10 | 0 blockers, 9 majors, 3 minors |
| threat | REVISE | 5 of 10 | 0 blockers, 9 majors, 1 minor |
