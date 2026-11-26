# Review round 3 of `go-s2-stack-web`: the register of findings

Two reviewers read `plan-v3.md` alone: the architect and the logic reviewer.
They reported 21 findings: 2 blockers, 16 majors and 3 minors.
Merged where both reviewers found the same thing, they form 12 groups, named G1 to G12 below.
These groups are new to this round, and their numbers do not refer to the groups of earlier rounds.
The fold was written on 2026-11-26, under the orchestrator's four rulings for this round.
C1 to C13 are the checks of the design, and D and R numbers are its decisions and rules. PB numbers are the post-build checks of this plan.

## Every finding

A duplicate names the finding it repeats, which is the first finding of its group.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r3 1 | blocker | plan | G1 | fixed another way | The other side of the circular gate is the Server plan, whose planner is applying the joint-gate rule to its items 8 and 9, as the orchestrator's first ruling says. Request S11 in the result file repeats the exact change, including the acceptance clause of item 8. |
| logic-r3 1 | blocker | plan | G1 | fixed another way | Repeats architect-r3 1. |
| architect-r3 2 | major | C4 | G2 | fixed | The removal reads no Compose model. It stops the project's `server` and `web` containers by their labels, then runs the stopped server's own image with its `data` network and its database password bind alone, so a broken settings file or a missing vault credential cannot stop it (decisions row 43). |
| logic-r3 2 | major | C4 | G2 | fixed | Repeats architect-r3 2. A missing container, a container still running after the stop, and a run without the committed line each report an unconfirmed removal and leave both services stopped. The test breaks the settings file before the removal. |
| logic-r3 5 | major | C4 | G2 | fixed | Repeats architect-r3 2 for the image. The removal runs the stopped container's image identifier and prints it, and the test compares it with the container's. Its point about the stream fixture's tag is fixed with G6. |
| architect-r3 3 | major | R9 | G3 | fixed another way | The launcher runs from `deploy/`, so Compose never reads a `.env` at the repository root, as the round 3 probe shows. This replaces the reviewer's explicit file list. Marked `UNREVIEWED MECHANISM`. The vault suite sets an empty settings file of its own. |
| logic-r3 3 | major | R9 | G3 | fixed another way | Repeats architect-r3 3. A file replaced between two Compose calls, and Compose's message that can hold a value, are a nicer diagnostic under the third ruling, so PB4 records them and request I3 asks for an issue. |
| architect-r3 4 | major | C1 | G4 | fixed another way | The second ruling removes the launcher's port read, and with it the line format the reviewers attacked. Compose, the server and Docker each refuse a wrong port with their own message. Marked `UNREVIEWED MECHANISM`. |
| logic-r3 4 | major | C1 | G4 | fixed another way | Repeats architect-r3 4. |
| architect-r3 5 | major | C6 | G5 | fixed | The guide tells other tools to run `docker compose port web 8080` from `deploy/`, after the launcher, with the launcher's own project name. Requests P1 and D5 now ask the installer and the driver for the same rule, and ask C1 to read the port after it launches. |
| logic-r3 7 | major | C6 | G5 | fixed | Repeats architect-r3 5. |
| architect-r3 9 | major | C9 | G6 | fixed | The compose file names `lgtm-server:${LGTM_IMAGE_TAG:-local}` and `lgtm-web:${LGTM_IMAGE_TAG:-local}`, each fixture sets its own tag and removes it, and the stream test compares bytes with `docker compose exec -T` (decisions row 44). |
| architect-r3 6 | major | D17 | G7 | fixed another way | This plan cannot edit the receiving plans. Requests E1 to E3 now name each exact check, and decisions row 41 holds the move only once those plans record it. Marked `UNREVIEWED MECHANISM`. |
| logic-r3 8 | major | D17 | G7 | fixed another way | Repeats architect-r3 6. |
| architect-r3 7 | major | C3 | G8 | fixed | Decisions row 34 now names request S12, and the result file repeats S12 to the Server plan and asks the Machines plan to own the first snapshot and the browser's sign-out check. |
| logic-r3 6 | major | C3 | G9 | fixed | The tests decode `GetServer` with Protocol Buffers defaults, so an absent `signedIn` field is false. After the removal, `SignOut` with the old cookie must return `unauthenticated`, and criterion 2d accepts a private window's reply without `signedIn: true`. |
| architect-r3 8 | major | C2 | G10 | post-build check | PB1 now holds the sessions lock for 5 seconds, inside the Store's ten-second lock wait, and expects a commit. It holds it for 15 seconds in a second run and expects the unconfirmed line, intact rows and a later commit. Both runs use the removal command, not the driver. |
| logic-r3 9 | major | C1 | G10 | post-build check | Repeats architect-r3 8. The driver's deadlines stay request D4 to its owner. |
| architect-r3 10 | minor | plan | G11 | fixed another way | The test case that registered a Git worktree checked the settings file's mode under an empty selector. That check is gone under the second ruling, so the case is gone, and no worktree is made. |
| logic-r3 10 | minor | plan | G11 | fixed another way | Repeats architect-r3 10. |
| logic-r3 11 | minor | C8 | G12 | fixed | `other_routes_keep_the_default_limit` also holds a silent body on `/lgtm.link.v1.LinkService/Join`, and widening the handler's path to `/lgtm.link.v1.*` is now the mutation that must turn it red. |

## Points by statement

A group counts once, at its highest severity: a blocker is 9 points, a major 3 and a minor 1.

| Group | Highest severity | Statement | Points |
|---|---|---|---|
| G1, the circular merge gate | blocker | plan | 9 |
| G2, the removal of the password | major | C4 | 3 |
| G3, the settings file Compose reads | major | R9 | 3 |
| G4, the launcher's port read | major | C1 | 3 |
| G5, the port for other tools | major | C6 | 3 |
| G6, the shared image tags | major | C9 | 3 |
| G7, the encrypted cases of other plans | major | D17 | 3 |
| G8, the browser split with the Server plan | major | C3 | 3 |
| G9, the omitted `signedIn` field | major | C3 | 3 |
| G10, the held lock and the driver's deadlines | major | C2 | 3 |
| G11, the Git worktree of a test | minor | plan | 1 |
| G12, the default limit on `Join` | minor | C8 | 1 |

| Statement | Points |
|---|---|
| plan | 10 |
| C3 | 6 |
| C1 | 3 |
| C2 | 3 |
| C4 | 3 |
| C6 | 3 |
| C9 | 3 |
| D17 | 3 |
| R9 | 3 |
| C8 | 1 |
| Total of the round | 38 |

## Stuck statements

A statement is stuck with at least 12 points and at least 30% of the round's points, which is 11.4 points.
No statement reaches 12 points, so none is stuck.
The highest is the plan itself with 10 points, nine of them from the circular gate that the Server plan closes.

Two mechanisms drew several majors, and both were wrong rather than incomplete:

- **The launcher's settings check** (decisions row 37) drew G3 and G4. It read the port from a line format that a multiline value breaks, and it checked a file that Compose did not read first. The second ruling removes it, and row 42 keeps only the start from `deploy/`.
- **The removal through the Compose model** (decisions row 38) drew three majors in G2. The model failed on a broken settings file before the stop, needed the vault credential's bind, and ran a shared tag. Row 43 replaces it with a removal that reads only the stopped container.

## Patches for the next round

The next round attacks only these places of `plan.md`, by heading. Round 4 runs only for blockers and majors.

- Key decisions, every entry, which became a list.
- WS-2: criteria 2b and 2d; work item 4 (the images of `server` and `web`, the settings of `server` written as one list, `web_fixture_create`, `web_fixture_cleanup` and `images_are_built_from_the_pinned_files`); work item 5 ("What it reads", the phase `web`, "Removing the password", `stack-vault.sh`, the tests `up_names_a_taken_port`, `up_remove_password_ends_sessions` and `up_remove_password_reports_unconfirmed`, the removal of `up_refuses_bad_settings`, the mutations, the acceptance and the public names); work item 6 (its title, the guide's "What the host needs", "Settings", "Remove the password" and "The port in other tools", and the acceptance); the risks.
- WS-3: work item 7 (the full fixture's build and `exec -T`, `other_routes_keep_the_default_limit` and the strength check).
- "Post-build checks from rounds 1 to 3": PB1, PB4 and PB7.
- Traceability, which became a shorter table.
- Guardrails, whose rows for habits 1 to 3 became one row.
- Execution sizing, the row of item 5.
- Attack log, the row of round 3.
- Across the plan, lines of prose were joined into paragraphs, and each item's work area, complexity and scope became one line, with no change of content.

`decisions.md` gained rows 42 to 44, replaced rows 37 and 38, and changed rows 34 and 41. `scouting.md` gained the section "Round 3 probes".

## The top finding of the round

The top finding is G1, a blocker found by both reviewers alone: the Server plan's items 8 and 9 still gate their merge on deployed checks that need this plan's images, so the circular gate stays open on that side.
The orchestrator ruled that the Server planner closes it, and request S11 gives the exact change.

## Verdicts and grades

| Reviewer | Verdict | Grade | Findings (B/M/m) |
|---|---|---|---|
| architect | REVISE | 5 out of 10 | 1 / 8 / 1 |
| logic | REVISE | 4 out of 10 | 1 / 8 / 2 |
