# Review round 2 of `go-s2-stack-web`: the register of findings

Three reviewers read `plan-v2.md` alone: the architect, the logic reviewer and the threat reviewer.
They reported 28 findings: 1 blocker, 21 majors and 6 minors.
Merged where two or more reviewers found the same thing, they form 17 groups, named G1 to G17 below.
These groups are new to this round, and their numbers do not refer to the groups of round 1.
The fold was written on 2026-11-26.
C1 to C13 are the checks of the design, and D and R numbers are its decisions and rules. PB numbers are the post-build checks of this plan.

## Every finding

A duplicate names the finding it repeats, which is the first finding of its group.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r2 1 | blocker | plan | G1 | fixed | The orchestrator's ruling makes C1 to C4 and C13 joint gates of the sprint (decisions row 40). Items 1 and 5 name the files of Server item 9 that they need, and no item waits for that item's gate. |
| architect-r2 2 | major | plan | G2 | fixed | The launcher calls Compose without `-f` and sets `COMPOSE_FILE` only when it is unset, and `web_fixture_override` adds an override to that list. Each stand-in clears `build` and sets its entry point and command together, and the test checks the rendered service and a marker in the log. |
| logic-r2 5 | major | C1 | G2 | fixed | Repeats architect-r2 2. The failed stand-in keeps `restart: unless-stopped`, so a restart count that stays the same proves that the launcher stopped it. |
| architect-r2 3 | major | C1 | G3 | fixed | The vault-only and removal routes give Compose the port 8787 in their own environment, and `up_until_vault_needs_no_web_settings` sets `LGTM_WEB_PORT=bad` in the file and in the shell. The probe is in scouting, "Round 2 probes". |
| logic-r2 2 | major | C1 | G3 | fixed | Repeats architect-r2 3. The project, the state folder and the volumes stay the same on both routes, because only the port's value changes. |
| threat-r2 3 | major | C1 | G3 | fixed | Repeats architect-r2 3. |
| threat-r2 4 | major | C1 | G4 | fixed | On the full route, the launcher reads the effective port from `docker compose config --environment`, refuses anything but one decimal port from 1 to 65535, and exports that number for the server's origin and the publication. `up_refuses_bad_settings` tries a range, `0` and a leading zero. |
| logic-r2 8 | minor | C1 | G4 | fixed | Repeats threat-r2 4. |
| logic-r2 7 | major | R9 | G5 | fixed | The launcher takes `${COMPOSE_ENV_FILES:-deploy/.env}`, so an empty selector means `deploy/.env`, as Compose reads it. `up_refuses_bad_settings` covers the unset and the empty selector. |
| architect-r2 4 | major | C4 | G6 | fixed | `deploy/up.sh --remove-password` stops the server, runs only the database phase, and runs the installed server once with an empty password, whatever the shell and the files hold (decisions row 38). It reports success only for this run's committed line. |
| logic-r2 1 | major | C4 | G6 | fixed | Repeats architect-r2 4. The test keeps the password exported and in the file, and stops OpenBao before the removal. |
| threat-r2 1 | major | C4 | G6 | fixed | Repeats architect-r2 4. The guide tells the owner to restore a password only after the line `every session has ended`. |
| architect-r2 5 | major | C3 | G7 | fixed | Sprint 2's browser evidence is the cookie and a `GetServer` reply of `signedIn: true` after a reload. Home and the browser's sign-out move to PB2 at the Machines gate (decisions row 34), and request S12 asks the Server plan for the same split. |
| logic-r2 3 | major | C3 | G7 | fixed | Repeats architect-r2 5. A private window must report `signedIn: false`, and the removal test reads `GetServer`, which does not change when the Server team serves an empty snapshot. |
| threat-r2 7 | minor | C3 | G7 | fixed | Repeats architect-r2 5. The removal test signs out of nothing before the removal, so the cookie under test stays valid until then. |
| architect-r2 6 | major | D17 | G8 | fixed | The encrypted cases of the contract, Server and Machines plans move to the later proxy work (decisions row 41), and requests E1 to E3 ask those plans to record the move with its date. The owner's choice on machine certificates stays its own gate. |
| architect-r2 7 | major | C9 | G9 | fixed | The full run builds the website image from the checkout under test, checks the running image and the Caddyfile inside it byte for byte, replaces the stand-in's mounts with `!override`, and checks the rendered service. Item 7's gate is both runs. |
| logic-r2 6 | major | C9 | G10 | fixed | `quiet-down` now fails on an early byte, sends `mark` while its side is open, reads `count`, and requires a clean end and the trailer `down-closed`. `dial` sends on a numbered schedule from a monotonic start, and the test requires exactly 11 messages. |
| threat-r2 9 | minor | C9 | G10 | fixed | Repeats logic-r2 6. |
| architect-r2 8 | major | C1 | G11 | post-build check | The driver's deadlines belong to the owner of `deploy/test/checks.sh`. PB1 now holds a lock for 30 seconds during C2 and times each recreation, and request D4 asks the driver's owner to wait within the Server plan's budgets. |
| architect-r2 9 | major | C1 | G12 | fixed | The launcher refuses a Docker Engine older than 28, the guide names Engine 28 and Compose 5.5, and PB6 checks from a second computer that the published port cannot be reached. |
| threat-r2 5 | major | C1 | G12 | fixed | Repeats architect-r2 9. The guide's forward keeps the same port on both ends. |
| threat-r2 2 | major | R9 | G13 | fixed | The launcher's settings check discards Compose's output and prints a fixed line, and `up_refuses_bad_settings` plants a marker in an unterminated quote. A file that Compose has read once cannot fail later calls on its settings. Request D3 stays with the driver's owner. |
| logic-r2 9 | minor | R9 | G13 | post-build check | Repeats threat-r2 2 for the driver. PB4 runs after request D3 lands and reads each program's arguments from a wrapper where it starts, so the check does not depend on when `ps` samples. |
| logic-r2 4 | major | C6 | G14 | fixed | The guide tells other tools to read the port from `docker compose -p lgtm port web 8080`, and to forward the same port on both ends. Request P1 asks `go-s3-stack-client` and request D5 asks the driver's owner to read it that way. |
| architect-r2 10 | minor | C4 | G15 | post-build check | The driver belongs to another owner. PB5 now covers C3's wrong password and C4's replacement, and request D2 asks for both. |
| threat-r2 8 | minor | plan | G16 | fixed | Each fixture is a folder of its own from `mktemp -d`, with its state folder inside it. Cleanup checks the folder's place and name and mounts only that folder, and the stat helper mounts it read-only. |
| threat-r2 6 | major | C8 | G17 | fixed | The global `read_body_idle 10m` is gone. The handler `timeouts /lgtm.link.v1.LinkService/Open { read_timeout 10m }` holds only the link's route open, and `other_routes_keep_the_default_limit` proves another route is still cut at about 60 seconds. Probes on 2026-11-26 passed six-minute soaks in all three shapes (decisions row 39). |

## Points by statement

A group counts once, at its highest severity: a blocker is 9 points, a major 3 and a minor 1.

| Group | Highest severity | Statement | Points |
|---|---|---|---|
| G1, the circular merge gate | blocker | plan | 9 |
| G2, how a test's override reaches the launcher | major | plan | 3 |
| G3, the web port on the vault-only route | major | C1 | 3 |
| G4, the port that Compose accepts | major | C1 | 3 |
| G5, the empty selector of settings files | major | R9 | 3 |
| G6, removing the password | major | C4 | 3 |
| G7, the browser check of sprint 2 | major | C3 | 3 |
| G8, the encrypted cases without a supplier | major | D17 | 3 |
| G9, the image and the fixture of the stream test | major | C9 | 3 |
| G10, the downward stream and the message count | major | C9 | 3 |
| G11, the driver's deadlines | major | C1 | 3 |
| G12, the Docker Engine floor | major | C1 | 3 |
| G13, a password in a diagnostic | major | R9 | 3 |
| G14, the port for other readers | major | C6 | 3 |
| G15, the password size in the driver | minor | C4 | 1 |
| G16, the fixture cleanup's reach | minor | plan | 1 |
| G17, the timeout on every route | major | C8 | 3 |

| Statement | Points |
|---|---|
| plan | 13 |
| C1 | 12 |
| R9 | 6 |
| C9 | 6 |
| C4 | 4 |
| C3 | 3 |
| C6 | 3 |
| C8 | 3 |
| D17 | 3 |
| Total of the round | 53 |

## Stuck statements

A statement is stuck with at least 12 points and at least 30% of the round's points, which is 15.9 points.
No statement reaches 15.9 points, so none is stuck.
The highest are the plan itself with 13 points, nine of them from the blocker that the ruling settles, and C1 with 12.

Two mechanisms drew several majors, and both were wrong rather than incomplete:

- **Compose as the only reader of the settings** (decisions row 29) drew G3, G4, G5 and G13. Compose validates the whole project before it selects a service, accepts a wider port grammar than the server, reads the default file under an empty selector, and prints a value in one message. Row 37 replaces it with a short check in the launcher.
- **The ordinary launcher as the removal path** (decisions row 31) drew three majors in G6. A password in the shell kept the old server, and an earlier phase could stop the launcher before it reached the server. Row 38 replaces it with `deploy/up.sh --remove-password`.

The stream test drew G9 and G10, which were gaps in its assertions, and the test's mechanism stays.

## Patches for the next round

The next round attacks only these places of `plan.md`, by heading.

- Overview, its fifth paragraph.
- Key decisions, the entries "The launcher checks only what it uses", "One command removes the password" and "The sprint's deployed checks are joint gates".
- Rollout order, steps 2 and 4.
- The opening of "Implementation details", its sentence on joint gates.
- WS-1: work item 1 (its needs), work item 2 (the rules and its `timeouts` handler, and `rules_refuse_other_host_names`), and the rollout detail.
- WS-2: its opening, criteria 2b, 2c and 2d, task 6, work item 4 (its needs, `web_fixture_create`, `web_fixture_override`, `web_fixture_cleanup`, `web_fixture_stat` and the public names), work item 5 (its needs, the settings check, the phases, the removal, the ten tests, the mutations, the acceptance and the public names), work item 6 (the needs, the guide, the tests and the acceptance), the risks, the verify line and the rollout detail.
- WS-3: its opening, criteria 3a and 3b, the task's anchor, and work item 7 (the needs, `quiet-down`, `dial`, `shaped -shape quiet-down`, the full fixture, the seven tests, the strength check, the acceptance and the public names), and the risks.
- "Post-build checks from rounds 1 and 2": PB1, PB2, PB4 and PB5 changed, and PB6 and PB7 are new.
- Traceability: the source universe, and the rows for the owner's sign-in, point 2, the Server plan's item 9 and check PB3, C1, C2, C3, C4, C6, C13 and R9.
- Execution sizing: the rows of items 1, 4, 5 and 6, and the order.
- Attack log, the row of round 2.

`decisions.md` gained rows 37 to 41, replaced rows 29, 31 and 32, and changed row 34. `scouting.md` gained the section "Round 2 probes".

## The top finding of the round

The top finding is G1, a blocker found by the architect: the Server plan's item 9 waited for deployed checks that needed this plan's images, while this plan's server image waited for that item to merge, so each team waited for the other.
The orchestrator's ruling settles it, and decisions row 40 records the ruling.

## Verdicts and grades

| Reviewer | Verdict | Grade | Findings (B/M/m) |
|---|---|---|---|
| architect | REVISE | 5 out of 10 | 1 / 8 / 1 |
| logic | REVISE | 5 out of 10 | 0 / 7 / 2 |
| threat | REVISE | 5 out of 10 | 0 / 6 / 3 |
