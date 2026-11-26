# Review round 1 of `go-s2-stack-web`: the register of findings

Four reviewers read `plan-v1.md` alone: the architect, the critic, the logic reviewer and the threat reviewer.
They reported 46 findings: 8 blockers, 34 majors and 4 minors.
Merged where two or more reviewers found the same thing, they form 16 groups, named G1 to G16 below.
The fold was written on 2026-11-26.
PB4, PB5 and PB8 name post-build checks of the sprint 1 plan, and PB1 to PB5 those of this plan. C1 to C13 are the checks of the design, and D and R numbers are its decisions and rules.

## Every finding

A duplicate names the finding it repeats, which is the first finding of its group.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r1 1 | blocker | C1 | G1 | fixed | The server receives the eleven keys of the Server plan's settings table, with `LGTM_ALLOW_PRIVATE_STACK=true` and no URL or certificate setting. |
| critic-r1 1 | blocker | C1 | G1 | fixed | Repeats architect-r1 1. |
| logic-r1 1 | blocker | C1 | G1 | fixed | Repeats architect-r1 1. |
| threat-r1 1 | blocker | C1 | G1 | fixed | Repeats architect-r1 1, and its point on the origin is fixed with G3. |
| architect-r1 2 | blocker | plan | G2 | fixed | This plan writes `deploy/test/web-testlib.sh`, drops the reset, runs `--until vault` before any web work, changes `stack-vault.sh`, and traces PB8, PB4 and the C5 owner as sprint 1 now has them. |
| critic-r1 4 | major | plan | G2 | fixed | Repeats architect-r1 2. |
| logic-r1 4 | major | C1 | G2 | fixed | Repeats architect-r1 2. |
| threat-r1 5 | major | C13 | G2 | fixed | Repeats architect-r1 2. The fixture refuses the project name `lgtm`. |
| critic-r1 6 | major | C1 | G2 | fixed | Repeats architect-r1 2. The vault-only route reads no web setting and builds no image. |
| architect-r1 3 | major | C3 | G3 | fixed | The server's origin is `http://127.0.0.1:${LGTM_WEB_PORT:-8787}`, the bind is fixed at `127.0.0.1`, and a test signs in with a browser `Origin` at a port other than 8787 under both loopback names. |
| critic-r1 3 | major | C3 | G3 | fixed | Repeats architect-r1 3. |
| logic-r1 2 | major | C3 | G3 | fixed | Repeats architect-r1 3. |
| critic-r1 9 | major | C1 | G3 | fixed | Repeats architect-r1 3. The launcher prints the address that Docker reports, and the public part is gone with row 27. |
| critic-r1 2 | blocker | C3 | G14 | fixed another way | The browser check observes sign-in, not Home, because the server serves no `WatchClients` in sprint 2. Home is post-build check PB2, and request S9 asks the Server team for an empty snapshot. Marked `UNREVIEWED MECHANISM`. |
| architect-r1 5 | major | C1 | G4 | fixed | An eight-minute start period, a two-second probe during it, and a 540-second launcher wait; the rollout driver's deadlines are post-build check PB1. |
| critic-r1 5 | major | C1 | G4 | fixed | Repeats architect-r1 5. |
| logic-r1 3 | major | C1 | G4 | fixed | Repeats architect-r1 5. |
| threat-r1 6 | major | C1 | G4 | fixed | Repeats architect-r1 5. |
| architect-r1 11 | major | C1 | G5 | fixed another way | The launcher reads no setting at all. Compose validates the port, and the launcher classifies Docker's own bind error at publication. Marked `UNREVIEWED MECHANISM`. |
| logic-r1 10 | major | C1 | G5 | fixed another way | Repeats architect-r1 11. |
| logic-r1 9 | major | C1 | G5 | fixed | Repeats architect-r1 11. The taken port is caught where Docker publishes it, and the other services stay up. |
| threat-r1 10 | major | C1 | G5 | fixed another way | Repeats architect-r1 11. The wider bind and the public port are gone with row 27. |
| architect-r1 4 | major | C4 | G6 | fixed another way | The ordinary launcher is the removal path, because Compose recreates the server, which ends the sessions before it exits. A test proves an old cookie fails after the password returns. Marked `UNREVIEWED MECHANISM`. |
| critic-r1 7 | major | C2 | G6 | fixed another way | Repeats architect-r1 4. |
| logic-r1 5 | major | C2 | G6 | fixed another way | Repeats architect-r1 4. |
| threat-r1 4 | major | C4 | G6 | fixed another way | Repeats architect-r1 4. |
| architect-r1 6 | major | plan | G7 | fixed | Item 1 follows Server item 9, item 4 no longer prepares folders, the public switch is gone, and the sizing table gives one dependency order. |
| architect-r1 7 | major | plan | G8 | fixed | The stream override clears `build` with `!reset null`, names the entry point and command, and the test requires no `build` in the rendered file. The web service has no bind left to prepare. |
| architect-r1 8 | major | C9 | G8 | fixed | Every run has a deadline, the upward shape ends with an answer and its trailer, and the soak runs all three shapes on the merged image. Only loopback exists in wave 1. The loss of a peer is post-build check PB3. |
| critic-r1 8 | major | C9 | G8 | fixed | Repeats architect-r1 8. The early `--rules-only` run needs only item 2, and item 7 no longer edits the website's Dockerfile. |
| logic-r1 6 | major | C9 | G8 | fixed | Repeats architect-r1 8. |
| threat-r1 8 | major | C9 | G8 | fixed | Repeats architect-r1 8. |
| architect-r1 9 | major | C1 | G9 | fixed another way | The public mode is cut to outside wave 1 (decisions row 27), so no certificate exists to reload. |
| critic-r1 10 | major | D17 | G9 | fixed another way | Repeats architect-r1 9. |
| logic-r1 7 | major | D17 | G9 | fixed another way | Repeats architect-r1 9. |
| logic-r1 8 | major | D17 | G9 | fixed another way | Repeats architect-r1 9. |
| threat-r1 9 | major | D17 | G9 | fixed another way | Repeats architect-r1 9. |
| threat-r1 3 | blocker | D17 | G10 | fixed another way | The public mode is cut to outside wave 1, so no browser meets a certificate in wave 1. The trust of a first visit belongs to the owner's later proxy. |
| architect-r1 10 | major | D17 | G11 | for the owner | Whether a machine presents its own certificate on the link is a choice between the design and the contract plan. The result file gives three options. |
| critic-r1 11 | major | D7 | G11 | for the owner | Repeats architect-r1 10. |
| threat-r1 2 | blocker | R9 | G12 | fixed | The guide makes the settings file with `install -m 600`, the launcher refuses a settings file that others can read or that is a link, and the acceptance uses a password made for the check. The check script's use of `jq` is a request to its owner and an issue. |
| threat-r1 7 | major | C8 | G13 | fixed another way | The stack keeps Caddy 2.11.9, which carries the fixed `golang.org/x/net`, and sets `read_body_idle 10m` for the whole loopback listener instead of one route. Marked `UNREVIEWED MECHANISM`. |
| architect-r1 12 | minor | C4 | G15 | post-build check | The check script belongs to another owner. Check PB5 and request D2 cover the 4,096-byte case. |
| logic-r1 11 | minor | C4 | G15 | post-build check | Repeats architect-r1 12. |
| critic-r1 12 | minor | plan | G16 | fixed | Item 3's person sees the screen for an unreachable server, item 4 compares the context with the repository root, the egress test names its probe, target and deadline, and the item that needed a shell in the server is gone. |
| threat-r1 11 | minor | plan | G9 | fixed another way | The certificate tool is gone with the public mode, so no package of it needs pinning. Counted with G9. |

## Points by statement

A group counts once, at its highest severity: a blocker is 9 points, a major 3 and a minor 1.

| Group | Highest severity | Statement | Points |
|---|---|---|---|
| G1, the server's settings | blocker | C1 | 9 |
| G2, the fixtures and the vault-only route | blocker | plan | 9 |
| G3, the browser origin and the printed address | major | C3 | 3 |
| G4, the readiness budget | major | C1 | 3 |
| G5, how the launcher reads settings and a taken port | major | C1 | 3 |
| G6, removing the password from a running stack | major | C4 | 3 |
| G7, the order of the merges | major | plan | 3 |
| G8, the stream fixture and its coverage | major | C9 | 3 |
| G9, the certificate's lifecycle | major | D17 | 3 |
| G10, the browser's trust on a first public visit | blocker | D17 | 9 |
| G11, the machine's certificate | major | D17 | 3 |
| G12, the settings file's mode | blocker | R9 | 9 |
| G13, the old Caddy release | major | C8 | 3 |
| G14, Home after sign-in | blocker | C3 | 9 |
| G15, the C4 replacement password | minor | C4 | 1 |
| G16, the test observations | minor | plan | 1 |

| Statement | Points |
|---|---|
| C1 | 15 |
| D17 | 15 |
| plan | 13 |
| C3 | 12 |
| R9 | 9 |
| C4 | 4 |
| C8 | 3 |
| C9 | 3 |
| Total of the round | 74 |

## Stuck statements

A statement is stuck with at least 12 points and at least 30% of the round's points, which is 22.2 points.
No statement reaches 22.2 points, so none is stuck.

Two mechanisms drew several blockers and majors, and both were wrong rather than incomplete.
The launcher's checks before any phase drew G2, G5 and G6, and the fold removed them.
The public mode drew G9, G10 and part of G3, G8 and G13, and the fold cut it to outside wave 1.

## Patches for the next round

The next round attacks only these places of `plan.md`, by heading.

- Overview, its third and fourth paragraphs.
- Key decisions, every entry.
- Workstreams, "One command starts everything, and the owner signs in" and "The link's long stream passes through the web server".
- Rollout order.
- The opening of "Implementation details".
- WS-1: criterion 1b, task 1, work item 1 (its needs), work item 2 (its files, the rules, the tests and the mutation), work item 3 (the runtime stage, the allow-list, `web_image_carries_the_rules`, `web_image_runs_unprivileged`, the mutation and the acceptance), the risks and the rollout detail.
- WS-2: its opening, criteria 2a, 2b and 2d, tasks 4 and 5, work item 4 (the whole item), work item 5 (the whole item), work item 6 (the guide and the acceptance), the risks, the verify line and the rollout detail.
- WS-3: its opening, criteria 3a and 3b, the task, and work item 7 (the files, the needs, the stand-in's `quiet-up` and `ticks`, the two fixtures, the tests, the strength check and the acceptance), the risks, the verify line and the rollout detail.
- The new fold "Post-build checks from round 1".
- Traceability, every row that names a public address, a certificate, C5, PB4, PB8 or the Server plan.
- Execution sizing, the table, the order and the load.
- The former WS-4, removed as a whole.

`decisions.md` gained rows 27 to 36 and changed rows 8, 17 and 24. `scouting.md` gained the section "Round 1 probes".

## The top finding of the round

The top finding is G1, a blocker: the compose file gave the server a database address that the Server plan no longer reads and left out its permission for the private stack, so a clean start could not reach sign-in.
All four reviewers found it alone.

## Verdicts and grades

| Reviewer | Verdict | Grade | Findings (B/M/m) |
|---|---|---|---|
| architect | REVISE | 4 out of 10 | 2 / 9 / 1 |
| critic | REVISE | 5 out of 10 | 2 / 9 / 1 |
| logic | REVISE | 5 out of 10 | 1 / 9 / 1 |
| threat | REVISE | 4 out of 10 | 3 / 7 / 1 |
