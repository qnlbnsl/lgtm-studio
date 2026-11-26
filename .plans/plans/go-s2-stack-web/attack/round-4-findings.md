# Review round 4 of `go-s2-stack-web`: the register of findings

One reviewer, the logic reviewer, read `plan-v4.md` alone.
It reported 7 findings: no blocker, 7 majors and no minor.
No two findings repeat each other, so they form 7 groups, named G1 to G7 below.
This is the last fold of this plan, and no reviewer reads it before the build.
The fold was written on 2026-11-26.
C1 to C13 are the checks of the design, and D and R numbers are its decisions and rules. PB numbers are the post-build checks of this plan.

## Not reviewed before the build

Each place below is marked `UNREVIEWED FIX` in `plan.md`, and each has a post-build check.

| Place in `plan.md` | Change | Post-build check |
|---|---|---|
| Work item 5, "What it reads", the first marker | `compose` keeps sprint 1's `-f` with the absolute path of `deploy/` when `COMPOSE_FILE` is unset, and drops `-f` only when a fixture sets `COMPOSE_FILE`. | PB10 |
| Work item 5, "What it reads", the second marker | `compose config --quiet` runs first with its output discarded, and a failure prints a fixed line and exits 2. | PB8 |
| Work item 5, "Removing the password" | The command `deploy/up.sh --remove-password` is gone. Removing the password is an ordinary start without one. | PB9 |
| Work item 5, the tests `up_password_removal_ends_sessions` and `up_keeps_the_password_out_of_its_output` | The first replaces the two removal tests. The second is new. | PB9 and PB8 |
| Work item 6, the guide's "Remove the password" | Delete the password, unset it in the shell, and run `deploy/up.sh`. | PB9 |
| Work item 6, the guide's "The port in other tools" | The lookup uses `-f deploy/compose.yaml`, and the joint gate runs at the default port until requests D5 and P1 land. | PB10 |
| Post-build checks, PB1 | The held-lock runs are gone with the removal command, and the run at port 18787 waits for request D5. | PB1 itself |
| Traceability, the row for points 1 and 2 | The contract plan's check PB2 must split before sprint 3's combined link gate is frozen. | none; request E1 |

## Every finding

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| logic-r4 1 | major | C4 | G1 | fixed another way | The removal command chose its project from the shell while the launcher let Compose choose it. The command is gone, and the guide's port lookup now uses the launcher's own `-f` call, so both take the project the same way (decisions rows 45 and 46). |
| logic-r4 2 | major | C4 | G2 | fixed another way | The removal command could misreport a failed stop. With the command gone, the server's own start without a password ends the sessions, and the launcher's existing failure path stops it. |
| logic-r4 3 | major | R9 | G3 | fixed | As the reviewer prescribed, the launcher handles Compose's settings failure at its output boundary. One `compose config --quiet` with the output discarded runs before any phase, including the helpers' calls, and prints a fixed line. `up_keeps_the_password_out_of_its_output` and PB8 look for the marker. |
| logic-r4 4 | major | C1 | G4 | fixed | The launcher and the guide's lookup both call Compose with `-f deploy/compose.yaml`, so they read the same files and project from any working folder. The joint gate runs at the default port until requests D5 and P1 land, and PB10 checks a root `.env` and a file-only project and port. |
| logic-r4 5 | major | C4 | G5 | fixed another way | Both weak mutations belonged to the removal command, which is gone. The new mutation prints Compose's message and must turn `up_keeps_the_password_out_of_its_output` red. |
| logic-r4 6 | major | D17 | G6 | fixed another way | This plan cannot edit the contract plan. Request E1 now gives the exact split of its check PB2, and decisions row 41 records that the Server and Machines plans have moved their cases. |
| logic-r4 7 | major | C4 | G7 | fixed another way | The held-lock runs of PB1 timed the removal command. With the command gone, they are removed, because no check from C1 to C13 needs them. |

## Points by statement

A group counts once, at its highest severity: a blocker is 9 points, a major 3 and a minor 1.

| Group | Highest severity | Statement | Points |
|---|---|---|---|
| G1, the removal's project | major | C4 | 3 |
| G2, the removal's failed stop | major | C4 | 3 |
| G3, a password in Compose's message | major | R9 | 3 |
| G4, the port for the driver and the installer | major | C1 | 3 |
| G5, the removal's mutations | major | C4 | 3 |
| G6, the contract plan's encrypted check | major | D17 | 3 |
| G7, the held-lock check | major | C4 | 3 |

| Statement | Points |
|---|---|
| C4 | 12 |
| C1 | 3 |
| D17 | 3 |
| R9 | 3 |
| Total of the round | 21 |

## Stuck statements

A statement is stuck with at least 12 points and at least 30% of the round's points, which is 6.3 points.
C4 is stuck, with 12 points, 57% of the round.

All four findings on C4 sit in one mechanism, the command `deploy/up.sh --remove-password` of decisions row 43, and the mechanism was wrong.
Rounds 2, 3 and 4 each found new faults in it: a separate route has to find the right installation, stop it, and prove a revocation on its own, and each of those steps can fail apart from the server.
The design's check C4 observes only that a restart keeps a session and that a changed password ends it, and C2 observes that a start without a password fails.
The fold therefore removes the command and keeps the simplest path that both checks observe: the owner deletes the password and runs the ordinary launcher, and the server ends every session before it would open its port (decisions row 46).
A launcher that stops before the phase `server` leaves the old server serving, and the result file names that case as outside wave 1.

## Patches for the next round

No reviewer reads this fold before the build. The places it changed in `plan.md` are:

- Key decisions, the entries "The launcher checks no setting" and "Removing the password is a start without one".
- WS-2: criterion 2b; work item 4, the mount of `server/postgres-app.password`; work item 5, its needs, "What it reads", "Removing the password", the tests `up_password_removal_ends_sessions` and `up_keeps_the_password_out_of_its_output`, the mutations and the public names; work item 6, the guide's "Remove the password" and "The port in other tools"; the risks.
- "Post-build checks from rounds 1 to 4": PB1, and the new PB8, PB9 and PB10.
- Traceability, the rows for points 1 and 2 and for the Server plan.
- Attack log, the row of round 4.

`decisions.md` gained rows 45 and 46, replaced rows 42 and 43, and changed row 41. `scouting.md` gained "Round 4 reading of the built launcher".

## The top finding of the round

The top finding is G3, a major: Compose's own message for an unterminated quote prints the password, and the launcher passed that message on, against design rule R9.
The reviewer confirmed it live, and its risk score of 168 is the highest of the round.

## Verdicts and grades

| Reviewer | Verdict | Grade | Findings (B/M/m) |
|---|---|---|---|
| logic | REVISE | 5 out of 10 | 0 / 7 / 0 |
