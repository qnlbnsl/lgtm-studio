# Round 4 findings folded into the server store plan

The reviewer examined the [frozen plan](plan-v4.md). The 2026-11-26 fold covers three major findings. This is the last plan review round before the build.

## Not reviewed before the build

Each place below is marked `UNREVIEWED FIX` in the plan. Check these cases first after the build.

| Plan locations | Change and first post-build check |
|---|---|
| Item 1 commit classification, item 4 `TestFeedCommitErrorOutcome` and the Commit outcomes check. | Set a three-second lock timeout at the held commit gate. Expect `55P03`, `OutcomeRolledBack` and `Retryable`. In a separate statement-timeout run, expect the commit to keep waiting until the gate is released. The frozen test establishes the lock-timeout result. The earlier statement-timeout rollback assertion is withdrawn. |
| Rollout steps 2 and 3, Stack handoff row, item 1 fixture, first workstream verification, and the Integration and Fixture checks. | Use the configured instance with the existing Go flags. Read `SHOW max_connections` and sample the bounded root run. Expect an accepted local endpoint, peak usage within the available connections and a passing check 12. This check covers the bounded test run. |
| Key decisions, item 8 follower behavior, and the Earlier restore check. | Restore an earlier disposable state while reads remain available. Existing streams can stay silent below their old cursor. Restart and reopen to receive a fresh snapshot. Automatic recovery remains outside wave 1. |
| Rollout step 1, item 1 file ownership, Guardrails triggered, Execution sizing, and the Dependency check. | Apply the 2026-11-26 epic ruling. Expect the builder to add `github.com/jackc/pgx/v5 v5.13.0` with the first import and the orchestrator to verify that change. |
| Post-build Lag and reconnects check. | Cancel or panic in a snapshot loader. Expect the next snapshot to acquire the slot. This is a supplementary observation of the existing design. |

## Findings and outcomes

The identifiers refer to design decisions (D), rules (R) and checks (C). There are no duplicate findings in this round.

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| logic-r4 1 | major | R4 | fixed | Use the observed lock timeout to prove a retryable rollback at commit. Check statement-timeout behavior separately after the build. |
| logic-r4 2 | major | R10 | fixed | Use the reviewer's fallback for the completed Stack plan. Remove the hard floor and run the suite within the instance's capacity using existing Go flags. |
| logic-r4 3 | major | R5 | fixed | Delete the contradictory history-reset rule. Record the restore consequence and restart procedure in the overview. |

## Points and stuck statements

Each major contributes three points. The round total is nine points. No statement reaches the 12-point and 30-percent thresholds.

| Statement | Points |
|---|---|
| R4 | 3 |
| R5 | 3 |
| R10 | 3 |
| Total | 9 |

## Patches

The marked patch locations are listed under [Not reviewed before the build](#not-reviewed-before-the-build). The attack log now links to this register and the post-build checklist gives the marked changes priority.

The three majors affect separate mechanisms. The transaction still holds the feed lock through commit, the Store fixture still uses real PostgreSQL, and Watch still starts from a snapshot. The changes remove a test assertion, an unsupported fixture floor and a contradictory follower rule.

## Top finding

Major, logic-r4 1. PostgreSQL 16 disables the test's statement timer before running the deferred trigger at commit. The original frozen case could not prove a rollback. The held trigger provides a real server lock-timeout case.

## Choice for the owner

Threat-r1 2 remains open. A credential-bearing call can refuse request keys and require manual recovery, record completion and return a separate recovery response, or use the vault to replay the original credential. Completion evidence changes the replayed response. Vault replay adds partial failure handling across stores.

## Reviewer verdict and grade

| Reviewer | Verdict | Grade | Findings |
|---|---|---|---|
| Logic | Revise. | 6 out of 10. | Three majors. |
