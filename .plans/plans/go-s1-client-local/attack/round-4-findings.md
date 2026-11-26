# Register of review round 4 — go-s1-client-local

Two reviewers attacked the frozen plan `attack/plan-v4.md` alone: architect and logic.
They reported 15 findings: 4 blockers, 11 majors and no minors.
The planner grouped duplicates into 10 groups, checked each finding against the plan, the design, the contract drafts and the repository, and gave each one outcome.
The fold was done on 2026-11-26.
Round 4 was the last round without the owner, so no reviewer reads this fold before the owner decides about a fifth round.

## Not reviewed before the build

Each place below is marked `UNREVIEWED FIX` in the plan, and each has a row in the plan's fold "Post-build checks".

| # | Place in the plan | What changed | Post-build check |
|---|---|---|---|
| 1 | Item 6, the format and the behaviour of the receipts; Key decisions; For the owner | A finished receipt is removed when the machine's clock is more than 24 hours past its stored `expires_at`. `MaxReceipts` and `order` are gone, and `wallClock` is the clock for tests (ruling 1). | Row 7 |
| 2 | Item 6, the public names and the behaviour of `CreateStore` and `OpenStore`; rules 9 and 11 of the fold "What plan go-s3-client-link must do" | Only `join` makes the store, through `CreateStore`. `OpenStore` returns `ErrStoreMissing`, and `run` stops before it dials (ruling 2). | Row 8 |
| 3 | Rule 2 of the fold "What plan go-s3-client-link must do" | The sender keeps the highest number it wrote on the connection and ends the stream on an acknowledgement above it (ruling 3). | Row 9 |
| 4 | Key decisions; Rollout order step 1; the rules of every item; item 6, "Needs first"; Execution sizing | `go.etcd.io/bbolt` v1.4.3 is added by the builder of item 6 in the same change as its first import (ruling 4). | Row 10 |
| 5 | Item 6, the behaviour of the open and of `Append` | The check runs on a read-only handle. `next` and every key are checked for 0, the last key must be `next` minus 1, and `Append` refuses at the largest number (ruling 6). | Row 11 |
| 6 | Item 6, the format and the behaviour of the receipts | A receipt that decodes but breaks the format is invalid, is reported as interrupted, and is never removed (ruling 6). | Row 12 |
| 7 | Rule 5 of the fold "What plan go-s3-client-link must do" | The bytes given to `Finish` hold no attempt, and the caller adds the attempt when it sends. | Row 13 |

The tests of item 6 changed with these places: `TestOpenStoreMissingAfterJoin` and `TestFinishPrunesByAge` are new, and `TestOpenStoreRefusesDamagedFile`, `TestAppendRefusesWhenFull`, `TestStartedFromEarlierRunIsInterrupted` and `TestStoreKillKeepsEveryNumberedEvent` gained cases.

## The rulings of the orchestrator for this round

The task of this fold carried six rulings of the orchestrator of 2026-11-26, which decisions 87 to 92 record.

1. Receipts are removed by age, as the design says, and the count of round 3 is withdrawn.
2. The store is made when the machine joins, and `run` without a store stops.
3. The store's `Ack` checks only what the store knows, and the bound of a connection lives in the sender.
4. The builder of the first item that imports the store adds it to `go.mod` in the same change.
5. A plan of sprint 3 that still names removed methods is that plan's to fix.
6. For the remaining majors, the reviewer's fix is taken when it is a check or a refusal of a few lines.

## Every finding and its outcome

A duplicate names the first finding of its group in the column "Repeats".

| Finding | Severity | Statement | Group | Repeats | Outcome | Reason |
|---|---|---|---|---|---|---|
| architect-r4 01 | blocker | R6 | A | | fixed | The reviewer's fix, which ruling 2 confirms: `CreateStore` in `join` before `identity.json`, and `OpenStore` returns `ErrStoreMissing`, so `run` stops before it dials. |
| architect-r4 02 | blocker | C11 | B | | fixed | The reviewer's fix, which ruling 3 confirms: the sender keeps the highest number written on the connection, and refuses an acknowledgement above it. |
| architect-r4 03 | blocker | R6 | C | | fixed another way | Ruling 1: receipts are removed by age, 24 hours after their stored expiry, as the design and the contract say, and the count is gone. |
| logic-r4 01 | blocker | R6 | C | architect-r4 03 | fixed another way | Ruling 1, as for architect-r4 03. Without `order`, no question of start or finish order is left. |
| architect-r4 04 | major | C11 | D | | fixed | The last key must be `next` minus 1, and the kill test records acknowledgements in its own lines. The check of a lower server mark at the welcome is a request to plan `go-s3-client-link`, and the server already refuses the gap. |
| logic-r4 02 | major | C11 | D | architect-r4 04 | fixed | The same check, with `next` and every key not 0, and `ErrSequenceExhausted` before the number would wrap. |
| architect-r4 05 | major | C11 | E | | fixed | The check runs on a read-only handle, and the file is opened for writing only after it passes. |
| logic-r4 03 | major | C11 | E | architect-r4 05 | fixed | The same fix, with the fixture of `version` 2 written with `NoFreelistSync`. |
| architect-r4 06 | major | R6 | F | | fixed | One rule of a valid receipt, shared by `Begin` and the removal in `Finish`. An invalid value is interrupted and never removed. |
| architect-r4 07 | major | plan | G | | fixed another way | Ruling 5: the places in plan `go-s3-client-link`, and the admission of `SetState` whatever its expiry, are requests in the result file. This plan does not change for them. |
| logic-r4 05 | major | plan | G | architect-r4 07 | fixed another way | Ruling 5, as for architect-r4 07. |
| logic-r4 04 | major | R6 | H | | fixed | Rule 5 now says that the bytes given to `Finish` hold no attempt, and that a replay uses the attempt that just arrived. The single point that orders a delivery and a finish is a request to plan `go-s3-client-link`. |
| architect-r4 08 | major | D19 | I | | fixed another way | Ruling 5: the installer, its stand-ins and the user-flow test are requests to the Stack team in the result file. |
| logic-r4 06 | major | D19 | I | architect-r4 08 | fixed another way | Ruling 5, as for architect-r4 08. |
| architect-r4 09 | major | plan | J | | fixed another way | Ruling 4: the plan names v1.4.3, and the builder of item 6 adds it in the same change as the first import. |

## Points for each statement

A group counts once, at its highest severity, under the statement of its first finding.

| Statement | Groups and points | Points | Share of the round |
|---|---|---|---|
| R6 | A 9, C 9, F 3, H 3 | 24 | 50% |
| C11 | B 9, D 3, E 3 | 15 | 31% |
| plan | G 3, J 3 | 6 | 13% |
| D19 | I 3 | 3 | 6% |
| **Total of the round** | | **48** | 100% |

## Stuck statements

R6, "commands through the link's stored commands", is stuck, with 24 points and 50% of the round.
C11, "buffered events arrive once, in order", is stuck for the fourth round, with 15 points and 31% of the round.

Both R6 blockers sat in the receipts after round 3: pruning by count, and a store made again under an existing machine.
The planner agrees that pruning by count was the wrong mechanism, because it departed from the design, and ruling 1 replaced it with the design's own rule.
The C11 blocker sat at the boundary between the store and its caller, and ruling 3 places the bound where the knowledge is.
The C11 majors are checks at open, and each is a refusal of a few lines.

## The patches, by heading of the plan

The next round, if the owner allows one, attacks only these places.

1. Overview, Key decisions: the removal by age, and the module at v1.4.3.
2. Rollout order, step 1.
3. For the owner: the clock of the machine as an accepted limit, and the note on round 4.
4. Implementation details, the rules of every item: the version of the module.
5. Item 6: "Needs first", the mutations, the public names, the format, the behaviour of `CreateStore` and `OpenStore`, of `Append` and of the receipts, the tests, the tasks 6.1 and 6.3, done-when rows 2 and 6, and the risks.
6. The fold "What plan go-s3-client-link must do": rules 2, 3, 5, 9 and 11.
7. The fold "Post-build checks": rows 7 to 13.
8. Traceability, Execution sizing and Attack log.
9. `decisions.md`: rows 87 to 92.
10. `scouting.md`: the section of round 4.

## The top finding of the round

The top finding is architect-r4 03, a blocker, which logic-r4 01 found alone as well.
Pruning by count could remove the only record of a command while the server could still send it, so the command could run twice or get a false result of expired.

## The verdict and the grade of each reviewer

| Reviewer | Verdict | Grade | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| architect | REVISE | 4 of 10 | 3 | 6 | 0 |
| logic | REVISE | 5 of 10 | 1 | 5 | 0 |

## Counts by outcome

| Outcome | Findings |
|---|---|
| fixed | 8 |
| fixed another way | 7 |
| rejected | 0 |
| post-build check | 0 |
| for the owner | 0 |
| not folded | 0 |
| **Total** | **15** |

All 7 findings fixed another way name a ruling of the orchestrator.
