# Register of review round 4 of the plan `go-s3-server-machines`

The round had two reports: architect and logic. They were folded on 2026-11-26 under the orchestrator's ruling that cuts the plan in three. Part A stays in `plan.md`, and parts B and C moved to `handoff-stream-and-commands.md`. The plan went from 711 to 357 lines, and its status stays `attack`. The decision log gains rows 87 to 94.

## Not reviewed before the build

No reviewer reads this fold before the build. Each place below is marked `UNREVIEWED FIX` in the plan, and each has a post-build check.

| Place in `plan.md` | What changed | Post-build check |
|---|---|---|
| Overview, the paragraph on the cut | The plan keeps part A only, and names the two future plans (decision 87). | P1 |
| Key decisions, "Wave 1 tokens are for the loopback address only" | The encrypted join waits for the later proxy work, and the Contract team is asked to move it (architect-r4 06). | P5 |
| Implementation details, "How every item is built" | The skeleton is removed, and each item declares its own names (decision 88). | P1 |
| Item 3, "No pruning" | `Tokens.Prune` is removed (decision 92). | P6 |
| Item 2, "No new option of the router" | The read limit of `Join` is a constant in `api.NewHandler`, and `api.Options.LinkOptions` and `app.Hooks` are gone (decision 91). | P2 |
| Item 5, "What `Project` does" | An approved machine is always offline in part A (decision 90). | P3 |
| Item 5, `TestWatchClientsBlockedReaderEnds` | A revocation ends a blocked send at its deadline at the latest (architect-r4 07, logic-r4 08, decision 94). | P4 |
| Item 6, `ApproveClient` | An approval stores no command in part A (decision 89). | P3 |
| Item 6, `ScanClientHarnesses` and `ReconnectClient` | Both refuse every known machine in part A (decision 90). | P3 |
| Item 7, the header | The wiring is a change to sign-in item 9 in the shape it has once built, with no new signature (decision 93). | P1 and P2 |

## Every finding and its outcome

A group of duplicates has one letter. The first finding of a group carries its points, and the others name the finding they repeat. The outcome "handed off" is the ruling's outcome for a finding of parts B or C: it is listed as open in `handoff-stream-and-commands.md` and is not folded here.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r4 01 | blocker | R10 | A | fixed another way | UNREVIEWED FIX, by the ruling. The plan keeps seven items of part A, each declaring its own names, and changes no signature of built code. Its parts that concern item 8, `NewState` and the case of `AcceptResult` in item 6 moved to the handoff. |
| architect-r4 02 | major | C8 | B | handed off | Part B. `Hub.Holds` must check the stream identifier and a live context. |
| architect-r4 03 | major | C10 | C | handed off | Part B. A superseded admission and the contract's check 10 disagree on the first frame. The contract sentence is in the result file. |
| architect-r4 04 | major | C11 | D | handed off | Part B. The outage gate needs a count of committed effects and an assigned change of `deploy/test/checks.sh`. |
| architect-r4 05 | major | R6 | E | handed off | Part C. The contract's check 11 requires a retry after the 23-hour window. The contract sentence is in the result file. |
| architect-r4 06 | major | R2 | F | fixed | The plan's key decision keeps the deferral, and the first request of the result file asks the Contract team to move the encrypted cases out of the combined acceptance of wave 1. P5 checks the loopback join. |
| architect-r4 07 | major | R5 | G | fixed | UNREVIEWED FIX. The reviewer's second option: the watch's test allows the ordinary deadline to end the send and requires that nothing written after the revocation arrives. P4 runs both cases, and the result file asks the Store team for a controlled authorization tick. |
| logic-r4 01 | blocker | R10 | A | fixed another way | Repeats architect-r4 01. The `app.Start` caller and fixture problem is gone, because part A changes no signature of sign-in item 9. |
| logic-r4 02 | major | C10 | C | handed off | Repeats architect-r4 03. |
| logic-r4 03 | major | C8 | B | handed off | Repeats architect-r4 02. |
| logic-r4 04 | major | C8 | H | handed off | Part B. A claim can walk past an unbounded prefix of ineligible rows under the machine's lock. |
| logic-r4 05 | major | R6 | I | handed off | Part C. A claimed command can be sent after the sweep finalized it. |
| logic-r4 06 | major | C11 | D | handed off | Repeats architect-r4 04. |
| logic-r4 07 | major | C9 | J | handed off | Part B. A held cleanup retry can delay the shutdown cause of other streams. |
| logic-r4 08 | major | R5 | G | fixed | Repeats architect-r4 07. |

## Points by statement

A minor finding is 1 point, a major is 3, and a blocker is 9. Each group counts once, at its highest severity.

| Statement | Groups | Points |
|---|---|---|
| R10 | A (9) | 9 |
| C8 | B (3), H (3) | 6 |
| R6 | E (3), I (3) | 6 |
| C10 | C (3) | 3 |
| C11 | D (3) | 3 |
| R2 | F (3) | 3 |
| R5 | G (3) | 3 |
| C9 | J (3) | 3 |
| **Total** | 10 groups: 1 blocker, 9 majors, 0 minors | **36** |

## Stuck statements

A statement is stuck with at least 12 points and at least 30% of the round's points, which is 10.8 points, so 12 points decide. No statement is stuck in this round. R10 has 9 points, which is 25%.

R10 has carried the blocker in rounds 2, 3 and 4, through a table of names and then a skeleton. That mechanism was wrong, because the plan was too large for every item to get a passing gate on paper, and the ruling replaces it with the cut.

## Mechanisms with several faults

- **The hub and its hold on a stream.** Groups B, C and J sit in it. The hold of round 3 lacked the stream identifier and a live check, and registration lacked a fence that survives an unregistration. Part B must redesign it, and the handoff lists all three.
- **The send window of commands.** Groups E and I sit in it. The server's window, the contract's check 11 and the sweep disagree. Part C must settle it with the Contract team before the freeze.

## Places changed in the plan

The plan was rewritten for part A. Every heading of `plan.md` changed, and the list of `UNREVIEWED FIX` places above names the new mechanisms. No reviewer attacks them before the build.

## Top finding of the round

The top finding is the blocker on R10 (group A): work items could not meet their own gates, for the third round. Both reviewers found it alone.

## Verdicts and grades

| Reviewer | Verdict | Grade |
|---|---|---|
| architect | REVISE | 5/10 |
| logic | REVISE | 5/10 |
