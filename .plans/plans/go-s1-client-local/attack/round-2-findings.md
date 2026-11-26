# Register of review round 2 — go-s1-client-local

Three reviewers attacked the frozen plan `attack/plan-v2.md` alone: architect, logic and logic2.
They reported 32 findings: 9 blockers, 18 majors and 5 minors.
The planner grouped duplicates into 19 groups, checked each finding against the plan, the design, the contract drafts and the repository, and gave each one outcome.
The fold was done on 2026-11-26.

## The ruling of the orchestrator for this round

The task of this fold carried a ruling of the orchestrator of 2026-11-26.
Two rounds in a row found blockers in the same mechanism, the outbox built from one file for each event, an epoch file and a mark.
The rule of the review is that such a mechanism is wrong, and that it is replaced, not patched again.
The outbox and the receipts of commands therefore moved into one embedded store with transactions, `go.etcd.io/bbolt`, in the file `client.db`.
Every finding that sat in the old outbox or the old receipts has the outcome "fixed another way", and its reason names this ruling.
Decision 55 records the ruling, and every changed place in the plan is marked `UNREVIEWED MECHANISM`.

## Every finding and its outcome

A duplicate names the first finding of its group in the column "Repeats".

| Finding | Severity | Statement | Group | Repeats | Outcome | Reason |
|---|---|---|---|---|---|---|
| architect-r2 1 | blocker | C11 | A | | fixed another way | By the ruling, the store records a sent mark before each send, and `Ack` refuses a number above it. `TestAckRefusesUnsent`. |
| logic-r2 01 | blocker | C11 | A | architect-r2 1 | fixed another way | The same fix, by the ruling. `Reconcile` moves unsent events instead of deleting them. |
| architect-r2 2 | blocker | C11 | B | | fixed | `state.Open` syncs every folder of the resolved chain on every open. `TestOpenSyncsWholeChainOnReopen` and `TestOpenCrashAfterCreatingParent`. |
| logic-r2 03 | blocker | C11 | B | architect-r2 2 | fixed | The same fix, with the second open required to sync `<tmp>/a`. |
| architect-r2 3 | blocker | C11 | C | | fixed another way | By the ruling, recovery is opening the store, and there is no mark file and no removal of event files. |
| architect-r2 4 | blocker | R6 | D | | fixed another way | By the ruling, the receipts are in the store. `Prune` raises `pruned_through` in the same transaction as the removal. `TestPruneRaisesWatermarkInSameCommit`. |
| logic-r2 04 | blocker | R6 | D | architect-r2 4 | fixed another way | The same fix. The watermark the reviewer prescribed is `pruned_through`, written in the commit that removes, so no removal can outlive it. |
| logic2-r2 8 | minor | R6 | D | architect-r2 4 | fixed another way | By the ruling, the file `receipts/clock` is gone, and the clock is the count of the server's time. |
| architect-r2 5 | blocker | R9 | E | | fixed | The child searches only trusted folders, each link is checked step by step, Linux access lists are refused, and the rule is named as the client's own. Access lists on macOS are post-build check 12. |
| logic-r2 07 | major | R9 | E | architect-r2 5 | fixed | The same fix, with `TestScanHelperInRefusedFolderIsNotFound` and `TestScanChecksEveryLinkStep`. |
| architect-r2 6 | major | C11 | F | | fixed another way | By the ruling, the epoch is gone. `Reconcile` follows the draft's `Hello.acked_event_number` and `Welcome.last_event_number`, and moves unsent events above a higher server number. |
| logic-r2 06 | major | C11 | F | architect-r2 6 | fixed another way | The same fix. No epoch remains whose text could be damaged. |
| logic2-r2 2 | major | C11 | F | architect-r2 6 | fixed another way | The same fix. Rule 3 of the fold now writes `Hello.acked_event_number` from `Acked()`. |
| architect-r2 7 | major | C11 | G | | fixed another way | By the ruling, a new store is made under a temporary name and linked when whole. `TestOpenStoreCrashDuringCreation`. |
| logic-r2 05 | major | C11 | G | architect-r2 7 | fixed another way | The same fix. |
| logic2-r2 1 | major | C11 | G | architect-r2 7 | fixed another way | The same fix. |
| architect-r2 8 | major | D7 | H | | fixed | `join` syncs the folder after an uncertain key and calls the server only after the sync succeeds. `identity.json` must be durable before success, and rule 20 gives the procedure after an expired token. |
| logic-r2 08 | major | C6 | H | architect-r2 8 | fixed | The same fix. Post-build check 4 covers each crash point. |
| architect-r2 9 | major | C11 | I | | fixed | Rule 8 of the fold names a producer: a load report every 10 seconds while the link is down, logged as `link_event_buffered`, which check C11 reads. Decision 71. |
| architect-r2 10 | major | C11 | J | | fixed another way | By the ruling, a failed commit stops the store, `run` exits, and the next start reads the disk. Producers make their reports anew, and the server keeps the latest. Decision 61. |
| logic-r2 02 | blocker | C11 | J | architect-r2 10 | fixed another way | The same fix. The run loop owns recovery, because the process restarts. `TestCommitErrorFailsStore` covers both disk outcomes. |
| architect-r2 11 | major | R6 | K | | fixed | `Begin` raises `LatestAttempt` while a command runs, the result goes out under it, `Finish` takes `unsupported`, and rule 12 maps every state to the wire. |
| logic-r2 09 | major | R6 | K | architect-r2 11 | fixed | The same fix. A failed `Finish` stops the store, and the next open records `interrupted`. |
| architect-r2 12 | major | D19 | L | | fixed | The installer sets no variable for the state folder, so every entry point uses the same default. Rule 23 maps the folder of clones to `Facts.workspace`. |
| logic-r2 11 | minor | D19 | L | architect-r2 12 | fixed | The same fix. Decision 68 withdraws the request of round 1. |
| logic-r2 10 | major | C11 | M | | fixed | `Folder.Claim` returns the paths of the locked folder, `Paths()` returns a copy, and a claim on a closed or zero folder is refused. `TestClaimUsesTheLockedFolder`. |
| logic-r2 12 | minor | C7 | N | | fixed | The budget test now proves that no later candidate starts once the first uses the budget, and a second test checks the remaining budget. |
| logic2-r2 3 | major | C11 | O | | fixed another way | By the ruling, `Reconcile` handles a server number above the sent mark by moving unsent events, so a restored folder recovers by itself. Rule 6 names the wait between retries. |
| logic2-r2 4 | major | R6 | P | | fixed another way | By the ruling, the clock is the count of the server's time, and pruning keeps a mark of expiries instead of times. `TestClockForwardJumpHeals`. |
| logic2-r2 5 | major | R6 | Q | | fixed another way | By the ruling, a replay writes nothing, and a failed commit restarts the client, which is the way out. `TestReplayWritesNothing`. |
| logic2-r2 6 | minor | R9 | R | | for the owner | Whether to trust a folder that a group can write is a choice between usability and risk. Decision 56 and the section "For the owner" give three options. Post-build check 13 records real installs. |
| logic2-r2 7 | minor | C11 | S | | fixed another way | By the ruling, the new `TestReconcileCases` pins both edges of the sent range, including the last number that was sent. |

## Points for each statement

A group counts once, at its highest severity, under the statement of its first finding.
Group H counts under D7, because its first finding names D7, and logic-r2 08 names C6.
Group R counts under R9, its first statement, although logic2-r2 6 also names D19.

| Statement | Groups and points | Points | Share of the round |
|---|---|---|---|
| C11 | A 9, B 9, C 9, J 9, F 3, G 3, I 3, M 3, O 3, S 1 | 52 | 60% |
| R6 | D 9, K 3, P 3, Q 3 | 18 | 21% |
| R9 | E 9, R 1 | 10 | 11% |
| D7 | H 3 | 3 | 3% |
| D19 | L 3 | 3 | 3% |
| C7 | N 1 | 1 | 1% |
| **Total of the round** | | **87** | 100% |

## Stuck statements

C11, "buffered events arrive once, in order", is stuck again, with 52 points and 60% of the round.
No other statement reaches 12 points and 30% of the round. R6 has 18 points but 21% of the round.

Four blocker groups sit in C11: A, B, C and J.
Groups A, C and J sat in the mechanism of the outbox, which round 1 also found wrong.
The planner agrees with the ruling that the mechanism was wrong: each patch of round 1 opened a new edge, such as the first write of the epoch, the order of the mark and the removals, and the meaning of an uncertain append.
The store replaces all of these with one commit for each change.
Group B is not in the outbox. It is the sync of the folders above the state folder, which item 2 keeps, and it is fixed in item 2.
Round 3 attacks the store as a new design.

## The patches, by heading of the plan

The next round attacks only these places.

1. Overview: the paragraphs on the two records and on the owner's choices.
2. Key decisions: "Every event is on disk before it gets a number", "The outbox deletes only events that were sent and that the server applied", and "The plan needs one module outside Go's standard library".
3. Workstreams: "Nothing the client must send or has done is lost in a crash".
4. Rollout order: steps 1, 3 and 4.
5. For the owner: rewritten.
6. Implementation details, the rules of every item: the module and the helper tests.
7. Implementation details, the layout of the state folder: rewritten, with `identity.json` and `client.db`.
8. Item 2: rewritten.
9. Tasks of WS-1: done-when rows 4 and 6.
10. Item 3: the behaviour after `ErrPlacedNotSynced`.
11. Item 5: the files, the public names, the cleaning of the search path, the trust rule, and seven tests.
12. Tasks of WS-3: task 5.3, done-when rows 4 and 5, and the risk of a planted program.
13. WS-4: rewritten. Items 6, 7 and 8 became item 6, the store and the outbox, and item 7, the receipts.
14. Tasks of WS-4, done-when, risks and verify: rewritten.
15. The fold "What plan go-s3-client-link must do": rewritten.
16. The fold "Post-build checks": rows 1, 4 and 5 changed, and rows 11 to 15 added.
17. Traceability: source 7, the rows that named items 6 to 8, and the rows of round 2.
18. Execution sizing: units 2, 5, 6 and 7, and the load line.
19. Attack log: the row of round 2 and the known gaps.
20. `decisions.md`: rows 55 to 71.
21. `scouting.md`: the two new sections of round 2.

The planner also found, while folding, that the user-flow test reads `identity.json` and not `server.json`. The layout and the fold now use that name, as decision 66 records.

## The top finding of the round

The top finding is architect-r2 1, a blocker, which logic-r2 01 found alone as well.
Both paths of acknowledgement used the highest number on disk as their limit, so an acknowledgement or a number from the server could delete an event that was stored but never sent.

## The verdict and the grade of each reviewer

| Reviewer | Verdict | Grade | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| architect | REVISE | 5 of 10 | 5 | 7 | 0 |
| logic | REVISE | 4 of 10 | 4 | 6 | 2 |
| logic2 | REVISE | 5 of 10 | 0 | 5 | 3 |

## Counts by outcome

| Outcome | Findings |
|---|---|
| fixed | 13 |
| fixed another way | 18 |
| rejected | 0 |
| post-build check | 0 |
| for the owner | 1 |
| not folded | 0 |
| **Total** | **32** |

Of the 18 findings fixed another way, all 18 name the ruling of the orchestrator.
The reviewers' lists of checks that need the code went into the fold "Post-build checks" as rows 11 to 15.
