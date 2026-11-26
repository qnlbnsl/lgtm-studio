# Review round 3 — register of findings

This register folds the three reports of round 3: `round-3-architect.md`, `round-3-logic.md` and `round-3-threat.md`.
The reviewed revision is the frozen copy `attack/plan-v3.md`, with the drafts as they stood after round 2.

The orchestrator gave five rulings for this round, and this register applies them.

1. The link is frozen when plans `go-s3-server-machines` and `go-s3-client-link` have both cleared their reviews. Until then, a change to the link files is an ordinary work item of the Contract team.
2. This plan states facts of the wire and nothing else. A rule about what a program does that the other end cannot observe is removed, and it becomes a request to the plan that owns the program.
3. A disagreement with a sprint-3 plan is that plan's to fix, unless this plan is wrong.
4. The contract's own defects are fixed with the smallest change, and no field is added that the design does not need.
5. Round 4 runs only for blockers and majors, and only on facts of the wire.

A finding that rulings 1, 2 or 3 settle has the outcome "fixed another way". The changed place in the plan names the ruling, and a mechanism that no reviewer proposed is marked UNREVIEWED MECHANISM.

## 1. Every finding

The severities are B for a blocker, M for a major and m for a minor.
The group column names the group of duplicates, and the "Repeats" column names the first finding of that group.

| Finding | Severity | Statement | Group | Outcome | Reason | Repeats |
|---|---|---|---|---|---|---|
| architect-r3 01 | B | D7 | G1 | fixed | `Join` checks the secret first, and an unknown id and a wrong secret both get `token_unknown`. This is ruling 4. | — |
| architect-r3 02 | M | C5 | G7 | fixed another way | Ruling 2 removes the two issuance contexts. `JoinToken.address` states only which address the server answers at, and the request goes to Stack Web, Server Machines and Stack Client. | logic-r3 07 |
| architect-r3 03 | M | C5 | G2 | fixed | The contract adopts the header `Lgtm-Join-Refusal` and the words of the Server Machines plan, with `machine_removed` in place of `token_removed`. The exit status goes to the Client Link by ruling 2. | threat-r3 01 |
| architect-r3 04 | M | D7 | G9 | fixed another way | The contract states that no stream receives `Welcome` once a removal commits, and adds the header `Lgtm-Link-Refusal`. The order of the hub is the Server Machines plan's, by ruling 3, and check 10 gains its barriers. | logic-r3 05 |
| architect-r3 05 | M | C11 | G3 | fixed another way | No renumbering. The order of checks is written, and a repeat is acknowledged with the last applied number. UNREVIEWED MECHANISM. | logic-r3 01 |
| architect-r3 06 | M | R6 | G10 | fixed another way | Ruling 2. The contract keeps the observable promise, a recorded answer until 24 hours after the expiry. The receipts' window, and the scouting anchor `OpenStore`, go to the Client Link and client-local plans. | logic-r3 08 |
| architect-r3 07 | M | C7 | G11 | fixed | Readiness now counts valid reports that the server applied from the current stream while approved. Check 4 sends both reports on B. | — |
| architect-r3 08 | M | R9 | G6 | fixed another way | Ruling 3. The contract is right that `message` is gone. The Client Link and Server Machines plans must drop it, and check 15 uses a constructed unknown field. | threat-r3 03 |
| architect-r3 09 | M | D17 | G15 | fixed another way | Ruling 1. Rollout step 9 says once when the link freezes. | logic-r3 02 |
| architect-r3 10 | M | C6 | G7 | fixed another way | Ruling 2. The reply field may be empty from a sample server, and which address a page shows is the website's. | logic-r3 07 |
| architect-r3 11 | M | D16 | G14 | fixed another way | Ruling 2 removes the 60-second wait, so the design's bound of 15 seconds holds. The diagnostic for a shared key goes to the Client Link. | — |
| architect-r3 12 | m | plan | G17 | fixed another way | Ruling 2. Checks 12 and 14 now test what the wire shows, and the checks of exit statuses and the installer move to their plans. Check 10 names its barriers, and check 15 constructs field 4. | — |
| logic-r3 01 | B | C11 | G3 | fixed another way | Ruling 4. A machine never changes an event's number, so two reports cannot share one. This fits the client-local `Ack`, which refuses a number at or above `Next()`. UNREVIEWED MECHANISM. | — |
| logic-r3 02 | B | plan | G15 | fixed another way | Ruling 1 replaces every sentence that froze the link at the merge of sprint 0. | — |
| logic-r3 03 | M | C10 | G8 | fixed another way | Ruling 3. The contract is right: a removed machine gets `failed_precondition`. The Server Machines plan must change `unauthenticated` and read the revision after the `Challenge`. | — |
| logic-r3 04 | M | C11 | G4 | fixed | The value rules name their one exception, the report at the next number, which the server consumes. The Server Machines plan must consume it in place of refusing it. | — |
| logic-r3 05 | M | D7 | G9 | fixed another way | As architect-r3 04. The contract states the observable rule, and the Server Machines plan closes the interval between admission and `Welcome`. | — |
| logic-r3 06 | M | C5 | G2 | fixed another way | Ruling 2. Exit statuses belong to the Client Link and Stack Client plans, and the result file asks for a nonzero exit on a refused join. | threat-r3 01 |
| logic-r3 07 | M | C6 | G7 | fixed another way | Ruling 2, as architect-r3 02. | — |
| logic-r3 08 | M | R6 | G10 | fixed another way | Ruling 2, as architect-r3 06. The request names a lower bound of the server's time for dropping a receipt. | — |
| logic-r3 09 | M | R6 | G10 | fixed another way | Ruling 2. The request names an upper bound of the server's time, checked when the work starts. | logic-r3 08 |
| logic-r3 10 | M | C7 | G12 | fixed another way | A machine applies `SetState` whatever its expiry, so no new rule on the server is needed. UNREVIEWED MECHANISM. Check 6 covers it. | — |
| logic-r3 11 | M | C5 | G1 | fixed | As architect-r3 01, and the outcomes of both races are written. The order of revocation against approval goes to the Server Machines plan. | architect-r3 01 |
| logic-r3 12 | M | R9 | G6 | fixed another way | Ruling 3, as architect-r3 08. | threat-r3 03 |
| threat-r3 01 | B | C5 | G2 | fixed another way | Ruling 2. The contract no longer names exit statuses. The Client Link plan must give `join` a nonzero exit on a refusal, and check C5 reads the header's reason. | — |
| threat-r3 02 | B | C11 | G5 | fixed another way | One machine id has one outbox. Two live copies of one machine's state are outside wave 1, like a restore, and the result file asks for an issue. UNREVIEWED MECHANISM. | — |
| threat-r3 03 | B | R9 | G6 | fixed another way | Ruling 3. Both link plans drop the field, and the server discards it as an unknown field. | — |
| threat-r3 04 | M | C6 | G7 | fixed another way | Ruling 2, as architect-r3 02. The listener of the default stack is Stack Web's. | logic-r3 07 |
| threat-r3 05 | M | C5 | G1 | fixed | As architect-r3 01, and the header carries the reason. | architect-r3 01 |
| threat-r3 06 | M | C10 | G8 | fixed another way | Ruling 3, as logic-r3 03. | logic-r3 03 |
| threat-r3 07 | M | R6 | G10 | fixed another way | Ruling 2, as architect-r3 06. | logic-r3 08 |
| threat-r3 08 | M | C11 | G3 | fixed another way | As logic-r3 01. The repeat's arithmetic is now the committed number in every case. | logic-r3 01 |
| threat-r3 09 | M | D17 | G16 | for the owner | The freeze part is settled by ruling 1. Point 2, option A, now names the added cost: a process on the hop can forge an acknowledgement or `Welcome.server_time`. | — |
| threat-r3 10 | M | D7 | G13 | fixed | A waiting machine's `Welcome` holds an empty connection token, which the Server Machines plan already does. | — |
| threat-r3 11 | m | D16 | G14 | fixed another way | Ruling 2, as architect-r3 11. | architect-r3 11 |

The counts by outcome are 7 fixed, 27 fixed another way, 1 for the owner, and none rejected, post-build check or not folded.

Two mechanisms each held two blockers or majors in earlier rounds and again here: the numbering of events (G3, G4 and G5) and the rules of what a client does on each code (G2 and G14). For the second, the mechanism was wrong for this plan, because it described programs that the other end cannot observe, and ruling 2 removed it. For the first, the mechanism was too large: the renumbering that round 2 copied from `Reconcile` is gone, and a rule without renumbering is the whole mechanism now.

## 2. Points per statement

A group counts once, at its highest severity: a minor is 1 point, a major 3 and a blocker 9.

| Statement | Groups | Points |
|---|---|---|
| C11 | G3 (9), G4 (3), G5 (9) | 21 |
| D7 | G1 (9), G9 (3), G13 (3) | 15 |
| plan | G15 (9), G17 (1) | 10 |
| C5 | G2 (9) | 9 |
| R9 | G6 (9) | 9 |
| C7 | G11 (3), G12 (3) | 6 |
| C6 | G7 (3) | 3 |
| C10 | G8 (3) | 3 |
| R6 | G10 (3) | 3 |
| D16 | G14 (3) | 3 |
| D17 | G16 (3) | 3 |

The round's total is 85 points, in 17 groups.

## 3. Stuck statements

None.
A statement is stuck at 12 points and 30% of the round, which is 25.5 points here.
C11 has 21 points, which is 25% of the round, and D7 has 15 points, which is 18%.
C11 has now led three rounds in a row, so round 4 should attack the rules of events first.

## 4. Patches by heading

Round 4 attacks only these places, for blockers and majors on facts of the wire.

1. `plan.md`, "Key decisions": the restore or copy, facts of the wire only, the clock of commands, removal, the projection and the reply's address.
2. `plan.md`, "Rollout order", step 9, and "Points for the owner": the freeze, and the added cost of point 2, option A.
3. `plan.md`, "Public names that other teams build against": the event number, `COMMAND_OUTCOME_INTERRUPTED`, `CommandResult.message`, `CreateClientTokenResponse.address`, the headers `Lgtm-Join-Refusal` and `Lgtm-Link-Refusal`, the codes and the value rules.
4. `plan.md`, item 2: the note that items 1, 2 and 7 merged.
5. `plan.md`, item 4: the rules of commands, `SetState` and expiry, `Restart`, `CommandResult`, the rules of events and the projection.
6. `plan.md`, item 5: the order of refusal with the secret first, the claim and its races, revocation, the token's address and pin, and what moved to other plans.
7. `plan.md`, item 6: the handshake, the value rules, the codes, after `Welcome`, the current stream, a waiting machine and removal, and the header names in `TestEveryLinkRuleIsWritten`.
8. `plan.md`, "Risks": the clock, the stop, and the restore or copy.
9. `plan.md`, "Agreements with other teams": every row, now facts of the wire.
10. `plan.md`, "Post-build checks": checks 3 to 6 and 10 to 15.
11. `plan.md`, "Traceability" and "Attack log".
12. `draft/link.proto`, `draft/events.proto`, `draft/commands.proto` and `draft/existing-files.diff`.
13. `decisions.md`, rows 43, 44, 57, 59, 61 and 62 amended, and rows 63 to 68.
14. `scouting.md`, the corrected anchor of the client-local plan, and runs 27 to 32.

## 5. The top finding

`Join` could return a reply for a used token without checking the secret, so anyone who held the token's id and the machine's public key could take the server's reply, and the order of refusal told a caller without the secret what state the token was in.
It was a blocker (architect-r3 01), and two more reviewers found it as majors (logic-r3 11, threat-r3 05).
By ruling 4, `Join` now checks the secret first, and an unknown id and a wrong secret get the same `token_unknown`.

## 6. Each reviewer's verdict and grade

| Reviewer | Verdict | Grade | Counts |
|---|---|---|---|
| architect | REVISE | 5 | 1 blocker, 10 majors, 1 minor |
| logic | REVISE | 4 | 2 blockers, 10 majors, 0 minors |
| threat | REVISE | 4 | 3 blockers, 7 majors, 1 minor |
