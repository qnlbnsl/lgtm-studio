# Round 3 findings folded into the server store plan

The two reviewers examined the [frozen plan](plan-v3.md). The 2026-11-26 fold covers 15 findings, with six majors and nine minors. No reviewer reported a blocker.

The identifiers refer to design decisions (D), rules (R) and checks (C). Each distinct finding contributes points to one primary statement. A duplicate names the finding that carries the points.

## Findings and outcomes

| Finding | Severity | Statements cited | Points statement or duplicate of | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r3 1 | major | C12 | C12 | fixed | Observe either a blocked B or its visible row while A is still at commit. Check the sequence only on the blocked branch. |
| architect-r3 2 | major | R2, plan | R2 | fixed another way | Group shared names by reader, compare built exports and reserve the sprint 2 instance lock in Store. |
| architect-r3 3 | major | R10 | R10 | fixed | Specify the fixture endpoint and capacity. Check the parsed settings and observe connections during the root suite. |
| architect-r3 4 | minor | C12 | C12 | fixed | Require deferral before the batch. Active commit waits prove the trigger fired at commit. |
| architect-r3 5 | minor | C12 | C12 | fixed | Reject an altered source tree and record the checkout commit around C12. Collect the script’s output separately. |
| architect-r3 6 | minor | R5 | R5 | fixed another way | One snapshot slot leaves room for a full writer limit, one pruner and an authorization read. |
| logic-r3 1 | major | R4, R9 | R4 | fixed another way | Classify failures during commit and test cancellation and both timeout sources. Preserve the mappings of known pre-commit sentinels. |
| logic-r3 2 | major | C12 | architect-r3 1 | fixed | The observer checks for visible B while A is uncommitted. Only that observation can produce the hook’s required sentence. |
| logic-r3 3 | major | plan | plan | fixed | Item 1 owns the shared helper. Its key includes both words and the key-form identifier. |
| logic-r3 4 | minor | R5 | R5 | fixed another way | Only an accepted page resets the clock. Test invalid pages and remove the database-restore branch. |
| logic-r3 5 | minor | D5 | D5 | fixed | A flagged delete still requires coverage by the durable floor. Advance it and delete the prefix in one transaction. |
| logic-r3 6 | minor | R2 | architect-r3 2 | fixed | The handoff includes read handles and both database fixture packages. |
| logic-r3 7 | minor | D5 | D5 | fixed another way | Drop the receipt alert. A single worker schedules the next sweep an hour after completion. |
| logic-r3 8 | minor | C12 | C12 | fixed | Record each extra mutation, the checked commit, the failing test and its message. |
| logic-r3 9 | minor | D3 | D3 | fixed another way | Assert the live migration timeout and exercise a shorter caller deadline through the existing context. |

## Points and stuck statements

A minor contributes one point, a major three and a blocker nine. The two duplicate groups count once each at their highest severity. The 13 distinct findings contribute 23 points.

| Statement | Points |
|---|---|
| C12 | 6 |
| D3 | 1 |
| D5 | 2 |
| R2 | 3 |
| R4 | 3 |
| R5 | 2 |
| R10 | 3 |
| plan | 3 |
| Total | 23 |

No statement reached 12 points. The stuck threshold also requires 30 percent of the round's points.

## Patches for the next round

| Plan heading | Changes to review |
|---|---|
| Key decisions and Rollout order | Treat restore as outside wave 1. Check the exact commit and compare Store exports with their consumers, including the sign-in lock. |
| 1. Connection pool, query handles and test isolation | Fixture endpoint and capacity, shared test gate and two advisory key forms, and safe commit outcome handling. |
| 3. Numbered database changes and store tables | Instance lock reservation, the delete guard and the live migration budget checks. |
| 4. Feed writes and commit order | Commit gate proof, competing event observation, cancellation and rejection outcomes, clean-checkout validation and mutation records. |
| 5. Request receipts | Observe the two-integer lock through the common test helper. |
| 6. Feed and receipt retention | Check the committed floor during delete. Drop the overdue receipt alert and schedule from completion. |
| 7. Read committed pages with notifications and polling | Limit snapshot admission. Remove the head field from pages and reject invalid page order. |
| 8. Bounded broadcast and subscriber lifetime | Only accepted pages reset staleness. Remove restore reset behavior and its test. |
| Attack log and Post-build checks from the review rounds | Record the hook evidence and exercise the fixture, reload burst and later instance-lock boundary. |

The overlapping major findings concern proof of feed ordering. The transaction advisory lock still fits the design. The test must prove commit deferral and check visibility before a gate timeout or sequence assertion can hide the ordering failure. The typed commit outcome is separate. Its tests now distinguish what the driver observed around commit.

## Top finding

Major, architect-r3 1 and logic-r3 2. The required mutation could fail on a timeout or sequence assertion before proving the later writer became visible. The test now checks the two observations while the first writer is still gated.

## Choice for the owner

Threat-r1 2 remains open. Credential-bearing calls can refuse request keys and require manual recovery, store keyed completion evidence with a recovery response, or use vault-backed replay of the original credential. The second option changes what replay returns. The third requires handling partial failures between stores.

## Reviewer verdicts and grades

| Reviewer | Verdict | Grade | Findings before deduplication |
|---|---|---|---|
| Architect | Revise. | 5 out of 10. | Three majors and three minors. |
| Logic | Revise. | 7 out of 10. | Three majors and six minors. |
