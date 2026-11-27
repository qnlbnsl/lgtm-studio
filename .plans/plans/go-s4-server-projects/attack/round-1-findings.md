# Round 1 findings for the server Projects plan

Foundation rules are marked with their source. The sprint 4 checks use the running-an-agent design. A duplicate retains its own severity and disposition. Scoring uses its primary row.

## Findings and dispositions

| Finding | Severity | Design statement | Outcome | Reason |
|---|---|---|---|---|
| logic-r1 01 | Blocker | R3, foundation | fixed | Item 4 uses Link's routed callbacks and item 8 propagates registration errors. [Link plan](../../go-s4-server-link/plan.md#L78) |
| logic-r1 02 | Major | R4, foundation | fixed another way | **UNREVIEWED MECHANISM**. Items 6 and 7 use one repository refresh above the removal budget. The batch of individual references would exceed Store's 256-change limit. [Store limit](../../../../src/server/internal/store/feed.go#L70) |
| logic-r1 03 | Major | R5, foundation | fixed | Item 7 commits bounded batches and checks the exact follower threshold. [Follower cutoff](../../../../src/server/internal/store/follow.go#L197) |
| logic-r1 04 | Major | plan | for the owner | Sprint 5 supplies a repository and machine without choosing between two saved paths. See Owner decisions below. [Sprint 5 call](../../../../docs/specs/2026-11-27-running-an-agent-design.md#L347) |
| logic-r1 05 | Major | R10, foundation | fixed | The paired gate names real-client subcases for aliases, restart, disconnect, ordering and expiry. [Sprint 4 checks](../../../../docs/specs/2026-11-27-running-an-agent-design.md#L287) |
| logic-r1 06 | Major | R7, foundation | fixed | Duplicates architect-r1 02. Store owns the prerequisite item and tests before Projects consumes it. |
| logic-r1 07 | Minor | R9, foundation | fixed | Items 4 and 5 check the explicit slash-home origin, withheld facts and absolute shorthand. [Contract grammar](../../go-s4-contract/plan.md#L234) |
| threat-r1 01 | Blocker | R3, foundation | fixed | Duplicates logic-r1 01. |
| threat-r1 02 | Blocker | R7, foundation | fixed | Duplicates architect-r1 02. |
| threat-r1 03 | Major | R4, foundation | fixed another way | Duplicates logic-r1 02. |
| threat-r1 04 | Major | R9, foundation | for the owner | The command can persist a browser-supplied value before the client can filter it. See Owner decisions below. [Outbox storage](../../../../src/server/internal/link/outbox.go#L21) |
| threat-r1 05 | Major | R10, foundation | fixed | Duplicates architect-r1 05. |
| threat-r1 06 | Major | R5, foundation | fixed | Duplicates logic-r1 03. |
| threat-r1 07 | Minor | plan | fixed | Item 8 uses the existing Session-size budget for Projects message reads and tests compressed and plain bodies. [Router pattern](../../../../src/server/internal/api/server.go#L129) |
| critic-r1 01 | Blocker | R3, foundation | fixed | Duplicates logic-r1 01. |
| critic-r1 02 | Major | R4, foundation | fixed another way | Duplicates logic-r1 02. |
| critic-r1 03 | Major | C12 | fixed | Duplicates architect-r1 02. |
| critic-r1 04 | Major | R5, foundation | fixed | Duplicates logic-r1 03. |
| critic-r1 05 | Major | plan | fixed | Duplicates architect-r1 05. |
| critic-r1 06 | Minor | R9, foundation | fixed | Duplicates logic-r1 07. |
| critic-r1 07 | Minor | plan | fixed | Execution sizing counts all four completed review seats and the extra Store work. |
| architect-r1 01 | Blocker | R3, foundation | fixed | Duplicates logic-r1 01. Item 4 also records the Link pre-commit interruption signature. |
| architect-r1 02 | Blocker | C12 | fixed | Item 9 names the frozen ledger assertions and adds real upgrade, replay, prune and rollback tests. [Frozen Store ledger](../../../../src/server/internal/store/migrate_test.go#L173) |
| architect-r1 03 | Major | R4, foundation | fixed another way | Duplicates logic-r1 02. |
| architect-r1 04 | Major | R5, foundation | fixed | Duplicates logic-r1 03. |
| architect-r1 05 | Major | plan | fixed | Items 2, 3 and 7 use local fixtures. Items 4, 5 and 8 own the production transitions and assembled gates. |
| architect-r1 06 | Major | R10, foundation | fixed | Duplicates logic-r1 05. Rollout names the existing Stack driver and deployed owner check. [Epic](../../../epics/go-rewrite/epic.md#L201) |
| architect-r1 07 | Minor | R9, foundation | fixed | Duplicates logic-r1 07. |

## Points by statement

| Statement | Primary groups | Points |
|---|---|---:|
| R3, foundation | logic-r1 01 | 9 |
| R4, foundation | logic-r1 02 | 3 |
| R5, foundation | logic-r1 03 | 3 |
| R9, foundation | threat-r1 04 and logic-r1 07 | 4 |
| R10, foundation | logic-r1 05 | 3 |
| C12 | architect-r1 02 | 9 |
| Plan | logic-r1 04, threat-r1 07, architect-r1 05 and critic-r1 07 | 8 |
| **Round total** | **11 groups** | **39** |

The Store group's points belong to C12 because the same prerequisite repairs the missing Store work and the versioned integration gate. The local test dependency group's points belong to the plan. The origin grammar group's points belong to foundation rule 9.

**Stuck statements.** None. The highest statement scores are 9 of 39. A statement needs both 12 points and 30 percent of the round to qualify.

**Top finding.** The Link registration mismatch is a blocker. All four reviewers found it independently. The Link interface has a single owner and supports the required operations. The second blocker group showed that held receipts were sound as a transaction boundary but incomplete as a work item. The Store migration and its frozen tests now have one owner and gate. The removal and watch-lag groups both exposed misuse of the same 256-change cap. The normal feed remains suitable. Unbounded per-repository publication in one removal write is the part that needs replacing.

## Owner decisions

- **logic-r1 04. Choose a checkout for sprint 5.** A sprint 5 folder picker and explicit identifier give the owner control at the cost of a website and contract change. A documented deterministic default keeps the dialog as designed but can pick a different checkout than the owner intended. Restricting each repository to one checkout per machine is simple for project creation but changes sprint 4 attachment behavior and could require resolving existing duplicates. Sprint 4 retains both paths under distinct folder identifiers.
- **threat-r1 04. Choose how to treat a credential pasted into a browser path.** The current path travels in a pending command before the client can recognize its own credential. Accepting this boundary preserves exact offline delivery at the cost of exposure to a database reader while the payload is stored. Protecting stored folder payloads with a server key preserves delivery and adds key management, encryption, restart coverage and rollout work in Link and the vault. The plan keeps the reviewed filtering checks for client-derived facts. The browser-input storage decision remains open. [Client filter design](../../../../docs/specs/2026-11-27-running-an-agent-design.md#what-leaves-a-machine-passes-one-filter)

## Plan patches

- **Overview, Key decisions and Rollout order.** Name the large-removal refresh, the later checkout choice and the Store prerequisite.
- **Folder requests survive a lost reply.** Add Store item 9, list the five frozen ledger assertions, make admission and retirement independently testable, use the published Link callbacks, and pin the home-relative origin and rollback cases.
- **Saved folders follow the machines that hold them.** Preserve the approved attachment rule and prove the two-account case. Keep cleanup in one write and refresh the repository watch for a removal above its feed budget.
- **The website can discover and follow saved repositories.** Fix the lag fixture, read the refresh marker, use the existing guarded body limit pattern, and separate the router and startup gates. Use real paired subcases and the existing stack entry points.
- **Post-build checks.** Combine overlapping restart, removal, commit uncertainty, migration, watcher and filtering scenarios.
- **Traceability, Guardrails triggered, Execution sizing and Attack log.** Record the Server-owned Store work, the four reviewer seats and the review disposition.

## Reviewer verdicts and grades

| Reviewer | Verdict | Grade |
|---|---|---:|
| Logic | Revise | 5/10 |
| Threat | Revise | 5/10 |
| Critic | Revise | 6/10 |
| Architect | Revise | 5/10 |
