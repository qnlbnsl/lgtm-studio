# Round 2 findings folded into the server store plan

The three reviewers examined the [frozen plan](plan-v2.md). The 2026-11-26 fold covers 25 findings, with 10 majors and 15 minors. No reviewer reported a blocker.

The cited identifiers name the design's decisions (D), rules (R) and checks (C). The points column assigns each distinct finding to its primary statement. A duplicate points to the finding that carries the score.

## Findings and outcomes

| Finding | Severity | Statements cited | Points statement or duplicate of | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r2 1 | major | C12, R10, plan | C12 | fixed | The Stack plan writes the wrapper. Server item 4 names its executable and its exact result. |
| architect-r2 2 | major | R10, plan | R10 | fixed | The orchestrator pins the driver with the first Server import, under the round ruling. |
| architect-r2 3 | major | C12, R10, plan | C12 | fixed | Ordinary Go tests can skip without the setting. Store acceptance and C12 require real PostgreSQL. |
| architect-r2 4 | major | C12, plan | C12 | fixed another way | Item 5 owns keyed deadlock retry. Combined prune and replay checks run after merge so item 6 remains independent. |
| architect-r2 5 | major | D4, D5, R5 | R5 | fixed another way | A dedicated startup and page pool bypasses saturated snapshots. The added test fills the ordinary pool. |
| architect-r2 6 | minor | C12 | logic-r2 2 | fixed another way | The exact marker and committed tree cover the duplicate. The two additional mutants are item 4 acceptance checks. |
| architect-r2 7 | minor | D3, plan | logic-r2 4 | fixed | The separate migration and maintenance budgets are stated and tested. |
| architect-r2 8 | minor | R2, plan | R2 | fixed another way | The Store plan names the writers and readers. The result sends these names to the orchestrator for the epic comparison. |
| architect-r2 9 | minor | R8, R2 | threat-r2 9 | fixed | Mark the lag status superseded and list the prerequisite handoffs in the result. |
| architect-r2 10 | minor | R10 | R10 | fixed another way | Bind the production lock constant and assert its namespace. Keep the test gate key in its own test file. |
| logic-r2 1 | major | D5, R4 | D5 | fixed | Repeat stamp and delete phases separately. A 1,300-row fixture defeats a one-page implementation. |
| logic-r2 2 | major | C12 | C12 | fixed | Verify the baseline, mutate one marked line, require a successful build and check the exact failure. |
| logic-r2 3 | minor | R5 | R5 | fixed | Empty successful reads reset the clock. An idle fixture stays open across the threshold. |
| logic-r2 4 | minor | D3 | D3 | fixed | Startup has its own budget. A real test exceeds both ordinary lock and statement limits. |
| logic-r2 5 | minor | R4 | R4 | fixed | Validate selected bytes, hash the version and report a missing retained encoder distinctly. |
| logic-r2 6 | minor | plan | plan | fixed | The writer test observes the first timeout wave before releasing the gate and gives the queued writers time to finish. |
| threat-r2 1 | major | R9 | R9 | fixed another way | An allowlist of safe sentinels and a default sanitizer cover driver and arbitrary wrapped errors. |
| threat-r2 2 | major | R4, R8 | R4 | fixed | Publish the sanitized outcome and test rejection and connection loss at commit. |
| threat-r2 3 | major | R9, D3 | R9 | fixed | Require the adapter's receipt-safety declaration and check its use. The policy choice from round 1 stays open. |
| threat-r2 4 | minor | R5, D5 | R5 | fixed | Every authorization failure ends the stream with an explicit error. The tests cover an arbitrary error and a panic. |
| threat-r2 5 | minor | D4 | D4 | fixed | Allow literal loopback or socket locally. Check the opted-in private service's actual dial target. |
| threat-r2 6 | minor | R4, D5 | D5 | fixed | Report overdue unobserved receipts. Check forward clock effects with a shifted fixture cutoff. |
| threat-r2 7 | minor | R4 | R4 | fixed | Feed triggers guard update, delete and truncate. The pruner uses a transaction-local flag. |
| threat-r2 8 | minor | R10 | R10 | fixed | Require a named disposable maintenance database before creating a fixture. Stack supplies the name. |
| threat-r2 9 | minor | plan | plan | fixed | Close the superseded global key and lag status decisions and retain their history. |

## Points and stuck statements

A minor contributes one point, a major three and a blocker nine. The three duplicate groups count once each at their highest severity. The 22 distinct findings contribute 42 points.

| Statement | Points |
|---|---|
| C12 | 12 |
| D3 | 1 |
| D4 | 1 |
| D5 | 4 |
| R2 | 1 |
| R4 | 5 |
| R5 | 5 |
| R9 | 6 |
| R10 | 5 |
| plan | 2 |
| Total | 42 |

There are no stuck statements. C12 reaches 12 points, or 28.6 percent. A stuck statement must reach both 12 points and 30 percent.

## Patches for the next round

| Plan heading | Changes to review |
|---|---|
| Overview, Key decisions, Workstreams and Rollout order | Shorter descriptions, the orchestrator's dependency pin, Stack's fixture and wrapper, and the downstream handoffs. |
| 1. Connection pool, query handles and test isolation | Dedicated page capacity, verified dial behavior, safe error allowlist, commit outcome, disposable database validation and test timing. |
| 3. Numbered database changes and store tables | Migration-specific timeout, the bound advisory lock key and the full feed guard. |
| 4. Feed writes and commit order | Marker and archived script result, separate lead mutations, the fixture gate constant, feed-only test ownership and commit outcome tests. |
| 5. Request receipts | Receipt safety declaration, versioned digest, missing-encoder error, namespace assertion and keyed deadlock retry. |
| 6. Feed and receipt retention | Repeated stamp and delete batches, raw test fixtures, overdue receipt diagnostics and guard use. |
| 7. Read committed pages with notifications and polling | Dedicated page pool and the saturation test. |
| 8. Bounded broadcast and subscriber lifetime | Dedicated startup read, empty-page staleness behavior and guarded restore fixture. |
| 9. Typed Watch streams | Authorization results, timeout and panic handling, safe sentinel creation and adapter mapping. |
| Guardrails triggered, Execution sizing, Attack log and Post-build checks from the review rounds | Corrected integration ownership, proof of the combined receipt and prune behavior, and new post-build checks. |

The proposed error boundary was incomplete. Suppressing selected PostgreSQL messages leaves other causes available and loses commit certainty. An allowlist and a separate transaction outcome preserve both invariants. Re-review them together.

The transaction-level feed lock remains the ordering mechanism. The C12 issues concern setup, test order and proving the mutation result. Re-review the driver and both wrapper layers as one check. The atomic receipt path remains suitable for receipt-safe calls. The owner choice about replaying credentials still applies.

## Top finding

Major, logic-r2 2. A hook can otherwise mistake an unrelated test failure for proof that the feed lock matters. The named assertion is the evidence required for a passing mutation check.

## Choice for the owner

Threat-r1 2 remains open. Credential-bearing calls can refuse the key and require manual recovery, store keyed completion evidence with a separate recovery response, or use the vault to replay the original credential and handle cross-store failures. The generic path requires an explicit safety declaration and excludes credentials and password-derived fingerprints.

## Reviewer verdicts and grades

| Reviewer | Verdict | Grade | Findings before deduplication |
|---|---|---|---|
| Architect | Revise. | 6 out of 10. | 5 majors and 5 minors. |
| Logic | Revise. | 7 out of 10. | 2 majors and 4 minors. |
| Threat | Revise. | 7 out of 10. | 3 majors and 6 minors. |
