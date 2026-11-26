# Register of review round 3 — go-s1-client-local

Two reviewers attacked the frozen plan `attack/plan-v3.md` alone: architect and logic.
They reported 24 findings: 7 blockers, 17 majors and no minors.
The planner grouped duplicates into 13 groups, checked each finding against the plan, the design, the contract drafts and the repository, and gave each one outcome.
The fold was done on 2026-11-26.

## The rulings of the orchestrator for this round

The task of this fold carried eight rulings of the orchestrator of 2026-11-26, whose aim is a smaller plan.

1. Restoring the state folder or the store to an earlier copy is outside wave 1.
2. There is no sent mark. An acknowledgement removes every event up to its number, and a number the store never handed out is refused.
3. A receipt has two written states, `started` and `finished`, and receipts are pruned by count without a clock.
4. This plan offers the outbox and the receipts as an interface, and does not decide which events exist or how fresh they must be.
5. The harness scan does what the design says and no more, so the rules about who may write a folder or a helper are removed.
6. The state folder is `LGTM_CLIENT_STATE_DIR` when it is set, and otherwise the path of the design.
7. No item needs code that a later item writes.
8. A finding that only a new mechanism could fix goes to the owner or to a post-build check.

Decisions 72 to 80 record the rulings.
A finding whose fix is a ruling has the outcome "fixed another way", and its reason names the ruling.
The plan went from 1,009 lines to 604, and from about 21,500 words to about 12,500.
It did not reach the aim of 500 lines without removing tests that the build loop requires each item to name.

## Every finding and its outcome

A duplicate names the first finding of its group in the column "Repeats".

| Finding | Severity | Statement | Group | Repeats | Outcome | Reason |
|---|---|---|---|---|---|---|
| architect-r3 01 | blocker | C11 | A | | fixed another way | Ruling 1: a restore is outside wave 1, and the move of unsent events in `Reconcile` is gone. A store with fewer numbers than the server gets `ErrAckAhead` on the first acknowledgement, and the client stops. |
| logic-r3 01 | blocker | C11 | A | architect-r3 01 | fixed another way | Ruling 1, as for architect-r3 01. With no sent mark, no deletion relies on a payload from another history. |
| logic-r3 02 | blocker | R6 | A | architect-r3 01 | fixed another way | Ruling 1: a restore that rolls back receipts is outside wave 1, and the risk table of WS-4 names it. |
| architect-r3 02 | blocker | C11 | B | | fixed another way | Ruling 2: the sent mark is gone. The server is proven before it acknowledges, and it acknowledges only what it applied. |
| architect-r3 03 | blocker | R9 | C | | fixed another way | Ruling 5: the trust rules of the scan are removed, because the client and the harnesses run as the same user in wave 1. |
| logic-r3 10 | major | R9 | C | architect-r3 03 | fixed another way | Ruling 5, as for architect-r3 03. |
| architect-r3 04 | blocker | plan | D | | fixed | The reviewer's second option: items 6 and 7 are merged into item 6, and the open of the store needs no receipt code (ruling 7). |
| logic-r3 03 | blocker | plan | D | architect-r3 04 | fixed | The same fix. The old reference to item 8 and the comparison of the trust code with the state folder are gone. |
| architect-r3 05 | major | C11 | E | | fixed another way | Ruling 4: the producer of events during an outage is removed. Which events exist is for plan `go-s3-client-link` and the contract. |
| logic-r3 08 | major | C11 | E | architect-r3 05 | fixed another way | Ruling 4: the producer and the rule of the latest producer time are both removed. |
| architect-r3 10 | major | C11 | F | | fixed another way | Ruling 4: the rule of the latest producer time is removed, and the request to the Server team is withdrawn. |
| architect-r3 06 | major | R6 | G | | fixed another way | Ruling 3: pruning uses no clock, and keeps the newest 1,000 finished receipts. |
| logic-r3 06 | major | R6 | G | architect-r3 06 | fixed another way | Ruling 3: the watermark of expiries and its clock are gone, so no unrelated command can be refused by it. |
| architect-r3 07 | major | D19 | H | | fixed another way | Ruling 6: one rule for the state folder, and the installer sets the variable for the service. The result file asks the Stack team to pass the same variable to `join` and to read `identity.json`. |
| logic-r3 11 | major | D19 | H | architect-r3 07 | fixed another way | Ruling 6, as for architect-r3 07. `XDG_STATE_HOME` is no longer read, and decision 86 asks the Stack team to change the user-flow test. |
| architect-r3 08 | major | C11 | I | | fixed | `OpenStore` refuses an empty file before the library opens it, writes nothing to an existing file, and releases the claim on a refusal. `TestOpenStoreRefusesDamagedFile` and `TestOpenStoreWritesNothingToExistingFile`. A probe confirmed the library's write, as `scouting.md` records. |
| logic-r3 05 | major | C11 | I | architect-r3 08 | fixed | The same fix, with a walk of every event key in a read transaction. |
| architect-r3 09 | major | C11 | I | architect-r3 08 | fixed | The walk checks that every key is consecutive and below `next`. The counts are computed by the walk and no longer stored, so they cannot disagree. |
| logic-r3 04 | major | C11 | J | | fixed | One lock guards admission, the transaction, the record of a failure and `Close`. `TestFailureStopsWaitingWriter`. |
| architect-r3 11 | major | R6 | K | | fixed another way | Ruling 3: the stored state is the truth after a crash. A `finished` receipt replays, a `started` one is interrupted, and nothing is repaired at open. `TestFinishCommitErrorKeepsStoredState`. UNREVIEWED MECHANISM. |
| logic-r3 07 | major | R6 | K | architect-r3 11 | fixed another way | Ruling 3, as for architect-r3 11. A damaged value is reported as interrupted without a write, so it needs no invented fields. |
| architect-r3 12 | major | C7 | L | | fixed another way | Ruling 5: the trust rules, the checks of links and the access-list test are removed. |
| logic-r3 09 | major | C7 | L | architect-r3 12 | fixed another way | Ruling 5, as for architect-r3 12. |
| logic-r3 12 | major | C6 | M | | for the owner | The fix needs a new mechanism, so ruling 8 sends it to the owner. Decision 79 and the section "For the owner" give three options and their costs. |

## Points for each statement

A group counts once, at its highest severity, under the statement of its first finding.
Group A counts under C11, although logic-r3 02 names R6.

| Statement | Groups and points | Points | Share of the round |
|---|---|---|---|
| C11 | A 9, B 9, E 3, F 3, I 3, J 3 | 30 | 48% |
| R9 | C 9 | 9 | 14% |
| plan | D 9 | 9 | 14% |
| R6 | G 3, K 3 | 6 | 10% |
| D19 | H 3 | 3 | 5% |
| C7 | L 3 | 3 | 5% |
| C6 | M 3 | 3 | 5% |
| **Total of the round** | | **63** | 100% |

## Stuck statements

C11, "buffered events arrive once, in order", is stuck for the third round, with 30 points and 48% of the round.
No other statement reaches 12 points.

Two blocker groups and four major groups sit in C11.
Groups A and B sat in the sent mark and in `Reconcile`, which existed for a restored store, and the planner agrees that this mechanism was wrong for wave 1: each answer to a restore opened a case of a different history.
Rulings 1 and 2 removed it instead of patching it.
Groups I and J sat in the open and the failed state of the store, which stay, and they are fixed in item 6.
Groups E and F sat in rules about producers, which ruling 4 moved to plan `go-s3-client-link`.

## The patches, by heading of the plan

The plan was rewritten, so every heading changed. The next round attacks these places first.

1. Overview, Key decisions, Workstreams, Rollout order and For the owner: shortened, with the two owner choices of decisions 79 and 85.
2. Implementation details, the rules of every item and the layout: the rule for the state folder (ruling 6).
3. Item 1: `StateDir` reads no `XDG_STATE_HOME`, and its tests are `TestStateDirRule` and `TestLoadSettings`.
4. Items 2 and 3: tests merged, and the mutations kept.
5. Item 5: the trust rules, `ProblemUntrusted` and the files `trust*.go` are removed (ruling 5).
6. WS-4, item 6: one item for the store, the events and the receipts. The new places are the check at open, the counts that are not stored, the one lock, `Ack` without a sent mark, the two states of a receipt, the interrupted state that is computed, pruning by count in `Finish`, and `Begin` with its argument `now`.
7. The fold "What plan go-s3-client-link must do": 11 rules, with no producer and no freshness rule.
8. The fold "Post-build checks": six rows.
9. Traceability, Guardrails, Execution sizing and Attack log: shortened for six items.
10. `decisions.md`: rows 72 to 86.
11. `scouting.md`: the section of round 3.

## The top finding of the round

The top finding is architect-r3 01, a blocker, which logic-r3 01 found alone as well.
A store restored from an older copy, under the same machine, could have its unsent events renumbered over numbers the server had already applied, so a report could apply twice or a command could run again.

## The verdict and the grade of each reviewer

| Reviewer | Verdict | Grade | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| architect | REVISE | 4 of 10 | 4 | 8 | 0 |
| logic | REVISE | 4 of 10 | 3 | 9 | 0 |

## Counts by outcome

| Outcome | Findings |
|---|---|
| fixed | 6 |
| fixed another way | 17 |
| rejected | 0 |
| post-build check | 0 |
| for the owner | 1 |
| not folded | 0 |
| **Total** | **24** |

All 17 findings fixed another way name a ruling of the orchestrator.
The reviewers' lists of checks that need the code are covered by the six rows of the fold "Post-build checks".
