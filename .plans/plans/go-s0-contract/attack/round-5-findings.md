# Review round 5 — register of findings

This register folds the one report of round 5, `round-5-logic.md`.
The round reviewed only the section "Amendment: the freeze of the link" and the drafts of the three files of the link.
The rest of the plan is built and was not reopened.

## 1. Every finding

The severities are B for a blocker, M for a major and m for a minor.
No two findings repeat each other, because one reviewer wrote them all.

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| logic-r5 01 | M | C10 | fixed | Verified at `.plans/plans/go-s3-client-link/plan.md:238`, which gives `SetState` "up to two seconds to reply before a removal stop". The comment of `APPROVAL_REMOVED` now says the machine answers done and does not stop. Check 30 tests the three cases. The client plan's change is a request in the result file. |
| logic-r5 02 | M | D7 | fixed | One send boundary for removal and a newer admission, in "The current stream" of `Open`. Checks 10 and 23 now expect what the comment allows. Plan `go-s3-server-stream` owns the generation check, also when the newer stream already ended. |
| logic-r5 03 | M | C5 | fixed | `Join` names two phases of value rules: the token fields and the key before the lookup, the host name only before a waiting machine is created. Check 28 sends one bad name on four paths. |
| logic-r5 04 | M | C11 | fixed | The first step of the table of events refuses 0 and numbers above 2^63 − 1 before any comparison. Report rules apply only at the next number. The client's edge is inclusive, and the stored `next` of 2^63 marks exhaustion. Check 29 tests both marks. |
| logic-r5 05 | M | C5 | fixed | The fallback for a missing or unknown reason is scoped to `Open failed_precondition`, and both stops of `Join` hold whatever the reason. Item 6 and the public names no longer call a gap final. `TestCodeFallbackIsWritten` is new, and `TestLinkCodesAreWritten` checks the verified-challenge condition. |
| logic-r5 06 | M | R6 | fixed | The reviewer's second choice: the replay limit is stated as the clock offset plus the time from send to lookup, and the false expiry beyond it is the accepted limit. The 23 hours stay, because other plans build against them. Check 26 holds the boundary case. Plan `go-s3-server-commands` gets the strict cutoff as a request. |
| logic-r5 07 | M | C7 | fixed | The part "Waiting" of `Open` now says the server applies, acknowledges and shows a valid waiting `Facts`, with no readiness, and consumes any other event. Verified at `.plans/plans/go-s3-server-machines/handoff-stream-and-commands.md:32`, which says a waiting machine can do nothing else. Check 27 reads the result from the page's watch. |
| logic-r5 08 | M | plan | fixed | Verified: `git merge-base HEAD master` is `f1dcbb7d`, which holds no record (scouting run 37). The check now reads a baseline that the orchestrator records at the freeze, a baseline without a record fails, and the check lives in `scripts/check-go.sh`, which the Contract scope may edit. `scripts/check-link-frozen.sh` is dropped. |
| logic-r5 09 | M | C6 | fixed | Verified at `src/client/cmd/lgtm-client/main.go:13`, whose `run` takes no input reader. The exact request to plan `go-s3-client-link` is in the result file, as the orchestrator asked. That plan is not edited. |

The counts by outcome are 9 fixed, and none fixed another way, rejected, post-build check, for the owner or not folded.

Two majors sit in the mechanism of `Join`'s refusals (logic-r5 03 and logic-r5 05). The mechanism is right, but the amendment wrote one value rule for two moments of the call. Naming the two phases and scoping the fallback fixes both.
Two majors sit in removal (logic-r5 01 and logic-r5 02). The mechanism is right too: the contract and the other plans read the same rule in two ways, so the rule is now stated once and the other plans are asked to point to it.

## 2. Points per statement

A minor is 1 point, a major 3 and a blocker 9.

| Statement | Findings | Points |
|---|---|---|
| C5 | logic-r5 03, logic-r5 05 | 6 |
| C10 | logic-r5 01 | 3 |
| D7 | logic-r5 02 | 3 |
| C11 | logic-r5 04 | 3 |
| R6 | logic-r5 06 | 3 |
| C7 | logic-r5 07 | 3 |
| plan | logic-r5 08 | 3 |
| C6 | logic-r5 09 | 3 |

The round's total is 27 points.

## 3. Stuck statements

None.
A statement is stuck at 12 points and 30% of the round. C5 has 6 points, which is 22% of the round.

## 4. Patches by heading

The next round attacks only these places.

1. `plan.md`, amendment, "Rulings and owner defaults that hold": the waiting `Facts` (logic-r5 07).
2. `plan.md`, amendment, "Changes to built code": the client's inclusive edge, the three rules stated once, and the requests to plans `go-s3-client-link` and `go-s3-server-commands` (logic-r5 01, 02, 03, 04, 06, 07, 09).
3. `plan.md`, amendment, item F4: `TestLinkCodesAreWritten` and the new `TestCodeFallbackIsWritten` (logic-r5 05).
4. `plan.md`, amendment, item F5: the baseline, the check in `scripts/check-go.sh`, and its "Done when" (logic-r5 08).
5. `plan.md`, amendment, "Which rules have a test": the row of the freeze.
6. `plan.md`, public names: the error key and the row of codes (logic-r5 05).
7. `plan.md`, item 6, "The codes" (logic-r5 05).
8. `plan.md`, post-build checks 10 and 23, and new checks 26 to 30.
9. `draft/link.proto`: the fallback, the two phases of `Join`, the send boundary of a newer admission, the waiting `Facts`, and the resume after a gap.
10. `draft/events.proto`: the first step of the table, and the report's rules at the next number only.
11. `draft/commands.proto`: the limit of at most once, and `APPROVAL_REMOVED`.
12. `decisions.md`, rows 87 to 95; `scouting.md`, runs 37 to 39.

## 5. The top finding

A courtesy notice of removal could stop the client before the server's signed refusal arrived, and leave its service stopped after an ordinary break (logic-r5 01).
It was a major, and the reviewer scored it highest with logic-r5 09.

## 6. Each reviewer's verdict and grade

| Reviewer | Verdict | Grade | Counts |
|---|---|---|---|
| logic | REVISE | 5 | 0 blockers, 9 majors, 0 minors |

## 7. The recheck

The reviewer checked the fold in `round-5-logic-recheck.md`, with the verdict REVISE.
It closed logic-r5 03 and logic-r5 06.
logic-r5 01, 07 and 09 stayed open in the texts of other plans, which their own planners changed at the orchestrator's request, so this plan does not touch them.
The four below were folded, and each changed place is marked UNREVIEWED FIX. No review round follows, and the plan's status is `done`.

| Finding | Open point | Outcome | Place |
|---|---|---|---|
| logic-r5 02 | Check 10 gave the stream's test seams and order to plan `go-s3-server-machines`. | fixed | Post-build check 10 now gives them to plan `go-s3-server-stream`, as the amendment does. |
| logic-r5 04 | A report whose only fault is a zero byte or a number above its limit in `link.proto` had no stated outcome at the next number. | fixed | One sentence in `draft/events.proto` and in item 4 of the plan: it is consumed with an `EventAck` and no effect. Item F2 names it. |
| logic-r5 05 | The targeted command skipped `TestCodeFallbackIsWritten`, and the fixture could drop `after_verified_challenge` from a stop row. | fixed | Item F4: the command runs `Fallback`. The condition is checked both ways, and every stop row of `Open` must hold it. |
| logic-r5 08 | A run without `LGTM_LINK_BASE` passed. | fixed | Item F5: the check fails closed when a commit before `HEAD` holds the record, and the build of item F5 is told apart by git. |

