# Round 1 findings of the plan `go-s3-stack-client`

The round had five reviewers: architect, critic, logic, threat and a second logic seat (logic2).
They reported 54 findings: 5 blockers, 37 majors and 12 minors.
The fold grouped them into 20 groups, named G1 to G20, and verified each against the plan, the neighbouring plans and the checkout.

## Every finding

A finding that repeats another names the first finding of its group.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r1 01 | blocker | D17 | G1 | fixed | The command now uses `Issuer{PublicURL, ServerKey}`, `ConnectionPolicy{AllowPrivateStackPostgres}` and `Store.Close`, and calls no vault close, as `go-s3-server-machines/plan.md:139` and `src/server/internal/store/pool.go:35` define. |
| critic-r1 01 | blocker | D7 | G1 | fixed | Repeats architect-r1 01. |
| logic-r1 01 | blocker | R3 | G1 | fixed | Repeats architect-r1 01. |
| threat-r1 1 | blocker | D17 | G1 | fixed | Repeats architect-r1 01. The `https` refusal is tested by `TestTokenAddressRules` of plan `go-s3-server-machines`, so the command adds no test for it. |
| critic-r1 02 | blocker | C6 | G2 | fixed | The stand-in uses `${SHIM_WEB_PORT-8787}`, so an empty value prints no mapping. |
| architect-r1 12 | minor | plan | G2 | fixed | Repeats critic-r1 02. |
| logic-r1 09 | major | plan | G2 | fixed | Repeats critic-r1 02. Its second half, the gate, is fixed with G3. |
| threat-r1 10 | minor | plan | G2 | fixed | Repeats critic-r1 02. |
| architect-r1 08 | major | plan | G3 | fixed | Items are gated by their own tests, and the new section "The sprint gate" holds the runs against the stack. |
| critic-r1 09 | major | plan | G3 | fixed | Repeats architect-r1 08. Item 1's three files are done by Server seats. |
| logic2-r1 03 | major | plan | G3 | fixed another way | Instead of a table of every name the script reads, gate G4 names the plans and items that write them. The test writer of the script keeps it in line, by note 4 of the orchestrator. |
| logic2-r1 01 | major | D16 | G4 | fixed | The token is fetched first, and a failed join restores the old folder and starts the service, as prescribed. Test `rejoin_failure_restores_old_state`. |
| architect-r1 04 | major | D16 | G4 | fixed | Repeats logic2-r1 01. The program and unit are installed only after a join. |
| critic-r1 03 | major | C6 | G4 | fixed | Repeats logic2-r1 01. A crash between steps is post-build check PB1. |
| logic-r1 03 | major | D16 | G4 | fixed | Repeats logic2-r1 01. |
| threat-r1 4 | major | C6 | G4 | fixed | Repeats logic2-r1 01. |
| critic-r1 04 | major | D19 | G5 | fixed another way | `--rejoin` moves only a folder that holds only the client's own names and is not a symbolic link, the archive is reserved with `mktemp -d`, and a `mkdir` lock serializes runs. Marked UNREVIEWED MECHANISM. |
| logic-r1 02 | major | D19 | G5 | fixed another way | Repeats critic-r1 04. |
| threat-r1 3 | major | D19 | G5 | fixed another way | Repeats critic-r1 04. Writable ancestors of the program and unit are not checked, because security issues do not block the build. |
| logic-r1 05 | major | D19 | G5 | fixed another way | Repeats critic-r1 04. A foreground client started by hand is outside the installer's lock, and the guide says to stop it. |
| logic2-r1 06 | minor | plan | G5 | fixed another way | Repeats critic-r1 04. `mkdir` replaces `flock`, which macOS lacks. |
| architect-r1 03 | major | C6 | G6 | fixed | A key without an identity is refused with the steps to remove the waiting machine and to run `--rejoin`. Test `refuses_unfinished_join`. |
| logic-r1 04 | major | D7 | G6 | fixed | Repeats architect-r1 03. |
| logic-r1 08 | major | C6 | G7 | fixed | A second run compares the `address` of `identity.json` with the selected stack. Test `rerun_refuses_changed_address`. |
| architect-r1 05 | major | D16 | G7 | fixed | Repeats logic-r1 08. |
| threat-r1 5 | major | C6 | G7 | fixed | Repeats logic-r1 08. The state folder of the installed unit is used when the override is unset. Test `rerun_uses_state_folder_of_installed_unit`. |
| logic2-r1 02 | major | D16 | G7 | fixed | Repeats logic-r1 08. A client that stops within 5 seconds of its start is reported with `--rejoin`. Test `rerun_reports_stopped_client`. |
| architect-r1 02 | major | R9 | G8 | for the owner | Whether the client reads the token from standard input is the owner's choice, which `go-s3-client-link/plan.md:24` already records. The tracing and usage parts are fixed. |
| critic-r1 05 | major | R9 | G8 | for the owner | Repeats architect-r1 02. |
| logic-r1 06 | major | R9 | G8 | for the owner | Repeats architect-r1 02. A failed write to standard output now gives exit 1. |
| threat-r1 2 | major | D7 | G8 | for the owner | Repeats architect-r1 02. The command's usage sentence repeats no argument. |
| logic2-r1 04 | minor | R9 | G8 | fixed | The risk row now says that every local user can read the token during the join, and drops "Low", as prescribed. |
| architect-r1 06 | major | D19 | G9 | rejected | D19 at `docs/specs/2026-11-25-go-rewrite-foundation-design.md:44` makes one user per machine the rule, and the orchestrator settled it. The plan names a second user as outside wave 1. |
| logic2-r1 05 | minor | D19 | G9 | fixed | Its three cases are fixed: the folder of the installed unit, a sentence after `--rejoin`, and the guide's rule of one user. |
| logic-r1 10 | major | D16 | G10 | fixed | The live test keeps a fixture record with the boot identifier and the machine, requires a new boot, and reads status 75 from the journal of the ended run. |
| architect-r1 09 | major | D16 | G10 | fixed | Repeats logic-r1 10. macOS requires a graphical session, and Docker must be enabled at boot. |
| critic-r1 07 | major | D19 | G10 | fixed | Repeats logic-r1 10. The overview states the Linux scope and the macOS login start. |
| critic-r1 08 | major | C9 | G10 | fixed | Repeats logic-r1 10. |
| threat-r1 9 | major | D16 | G10 | fixed | Repeats logic-r1 10. |
| architect-r1 11 | major | C9 | G10 | fixed | Repeats logic-r1 10. The live test refuses an existing program or state folder and acts only on the recorded machine. |
| logic-r1 11 | minor | C5 | G11 | fixed | Gate G4 states that a port other than 8787 is exported in the shell, as the script's guide says. |
| architect-r1 07 | major | C6 | G11 | fixed | Repeats logic-r1 11. The change of the script's Compose calls is a request in the result file, by note 4 of the orchestrator. |
| critic-r1 06 | major | C6 | G11 | fixed | Repeats logic-r1 11. |
| threat-r1 8 | major | C5 | G11 | fixed | Gate G4 names `psql_in_stack` of plan `go-s1-stack-vault`, item 2, as a prerequisite. |
| architect-r1 10 | major | C6 | G12 | rejected | The page is outside this plan's folders by the epic's table, and `go-s3-client-link/plan.md:76` already observes the pasted command in a real Machines page. The result file asks the orchestrator to assign `src/frontend/src/screens/service.tsx:410`. |
| logic-r1 12 | minor | C11 | G13 | not folded | It concerns the server's link test and C11, which this plan does not write. Forwarded in the result file. |
| threat-r1 7 | major | R9 | G14 | fixed | The live test passes the password to `jq` on standard input and removes its private files. The same change to `deploy/test/checks.sh` is a request. |
| logic-r1 07 | major | D16 | G15 | fixed | The promise is narrowed, as the finding offers: a removed client stops at its next connection, and the service does not start it again until the next boot or login. |
| logic2-r1 07 | minor | plan | G16 | fixed | Missing tools and a failed `docker compose` each have their own sentence and test. |
| logic2-r1 08 | minor | plan | G17 | fixed | New stand-in switches and the tests `refuses_stack_not_answering`, `build_failure_stops_installer` and `rerun_reports_stopped_client`, and the build flags are asserted. |
| critic-r1 10 | minor | C6 | G17 | fixed | The readiness check now uses the predicates of `get_server` at `deploy/test/checks.sh:60`, with a timeout, and the program is installed only after the join. |
| logic2-r1 09 | minor | plan | G18 | fixed | Each test sets `TMPDIR` inside its folder, and the whole folder is scanned. The client's own redaction is credited to `TestAppLogRedactsSecrets`. |
| critic-r1 11 | minor | plan | G19 | fixed | Superseded rows are closed with their successor named, row 4 names no tool, row 32 states the refusal, absence tests first prove their step ran, and WS and `MainPID` are expanded. |
| threat-r1 6 | major | D7 | G20 | for the owner | Whether the installer refuses a remote Docker endpoint is a choice of risk that the design does not settle. |

## Points by statement

A group counts once, at its highest severity and under the statement of its first finding. A minor is 1 point, a major 3 and a blocker 9.

| Statement | Groups | Points |
|---|---|---|
| plan | G2 (9), G3 (3), G16 (1), G18 (1), G19 (1) | 15 |
| C6 | G6 (3), G7 (3), G11 (3), G12 (3), G17 (1) | 13 |
| D17 | G1 (9) | 9 |
| D16 | G4 (3), G10 (3), G15 (3) | 9 |
| D19 | G5 (3), G9 (3) | 6 |
| R9 | G8 (3), G14 (3) | 6 |
| D7 | G20 (3) | 3 |
| C11 | G13 (1) | 1 |
| Total | 20 groups | 62 |

## Stuck statements

None. A statement is stuck with at least 12 points and at least 30% of 62, which is 18.6 points. `plan` has 15 points, which is 24%. C6 has 13 points, which is 21%.

Two groups of major findings sat in one mechanism, the order of `--rejoin` (G4 and G5). That mechanism was wrong: it destroyed the old state before the new join was known to work. The fold replaced it, so nothing is changed before the join succeeds.

## Patches

The next round attacks only these places of `plan.md`.

1. "Overview", paragraphs 1, 2 and 5.
2. "Key decisions", the bullets "Nothing that works is changed before the join succeeds" and "A second run is safe".
3. "Workstreams", the second workstream's second and third paragraphs.
4. "Rollout order", steps 2 and 4.
5. "How the statements of the design are cited", the lines on R8 and WS.
6. "Public names", the rows of the command, the unit, the archive and the lock, and the table of names read, its rows on the store, the machines part and the state folder.
7. "The service, the address and the exit statuses", every row and the last paragraph.
8. Work item 1: "Done when", "Files", "Behaviour", the tests `TestJoinTokenRefusesUnknownArgument` and `TestJoinTokenFailsOnWriteError`, "Acceptance", "Risks" and "Verify".
9. WS-2 "Done when", every row.
10. Work item 2: "Needs first", "Risky", every step of "Behaviour", "The change of the user-flow script", "The stand-ins", the tests listed in criteria 2c and 2d, `installs_linux_user_service`, `asks_server_for_hosts_server_token`, `joins_before_service_starts`, `token_never_printed`, `second_run_waits_for_lock` and "Acceptance".
11. Work item 3: "Behaviour", `installs_macos_launch_agent` and "Acceptance".
12. Work item 4: "How the test runs", every test, "The guide" and "Acceptance".
13. The risk table of WS-2 and its "Verify".
14. The new sections "The sprint gate", "Post-build checks" and "Outside wave 1".
15. The traceability rows of the round, D19, R8, R9, and the gate rows.
16. The guardrail row of the epic's teams, and the load line of execution sizing.

## Top finding

G1, a blocker: the token command named a setting, an issuer field and a vault close method that the Server plans do not define, so it could not compile. Four reviewers found it alone.

## Verdicts and grades

| Reviewer | Verdict | Grade | Count |
|---|---|---|---|
| architect | REVISE | 4 of 10 | 1 blocker, 10 majors, 1 minor |
| critic | REVISE | 5 of 10 | 2 blockers, 7 majors, 2 minors |
| logic | REVISE | 5 of 10 | 1 blocker, 9 majors, 2 minors |
| threat | REVISE | 5 of 10 | 1 blocker, 8 majors, 1 minor |
| logic2 | REVISE | 6 of 10 | 0 blockers, 3 majors, 6 minors |
