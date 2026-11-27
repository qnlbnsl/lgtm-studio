# Round 1 findings for the Sprint 4 stack checks

## Findings and outcomes

The design identifiers below are the foundation decisions and checks and the Sprint 4 checks. The critic and logic reports were written independently. Each duplicate points to the finding that carries its points.

| Finding | Severity | Design | Outcome | Reason |
|---|---|---|---|---|
| critic-r1 01 | Blocker | R11 | fixed | G4 and G5 keep the committed driver and supported selectors. G2, G3 and the owner’s R14 step supply the other evidence. |
| critic-r1 02 | Major | C12 | fixed | G1 keeps generation and feed-lock mutation in a disposable checkout with both freeze objects and checks for source drift there. |
| critic-r1 03 | Major | C1 | fixed | Duplicate of logic-r1 01. Fail before mutation on a stale project identity, existing volume, occupied port or missing selector. |
| critic-r1 04 | Major | R11 | fixed | Gate the upgrade on backup, password and prebuild. Inspect the migration ledger after interruption and exercise forward repair through the operator path. |
| critic-r1 05 | Major | R11 | fixed | Stop the old writer and discard its holds. Compare a common unnumbered digest and separately prove a committed application and new-attempt settlement. |
| critic-r1 06 | Major | R6 | fixed | The owning plans’ fixture options replace the proposed names. Compile the paired packages and exercise real rollback. |
| critic-r1 07 | Major | R9 | fixed | G2 requires case-level contract evidence, including diagnostic commit, history fences, waiting replay, saved receipts and the limits. |
| critic-r1 08 | Major | R14 | fixed | G5 assigns the real-mode proof, two separate deployed saves, both reloads and the named Website cases. The independent writer receives the precise frozen-source gaps. |
| critic-r1 09 | Major | D19 | fixed another way | Item 4 removes external prepared selection entirely. Each host starts its owned stack and checks the disposable account, manager and installer destinations. |
| critic-r1 10 | Minor | plan | fixed | Certificate proof gets its own runner. Dual holds and retirement have a focused relay test, and both native supervisors return independent results. |
| critic-r1 11 | Minor | plan | fixed | Expand first uses of workstream, interface and hash terms. Drop the numeric address and the extra Vite command from the plan. |
| logic-r1 01 | Blocker | C1 | fixed | Require an unused selection and a current-run ownership record before provisioning. Recheck recorded identities before reuse, requests, lifecycle actions and cleanup. |
| logic-r1 02 | Major | R4 | fixed | Duplicate of critic-r1 04. The old ledger, committed candidate ledger and partial installer have explicit retry decisions. |
| logic-r1 03 | Major | R10 | fixed | Duplicate of critic-r1 01. The plan keeps the valid selectors and maps the remaining design checks to their actual owners. |
| logic-r1 04 | Major | C1 | fixed | Use the independent browser’s scratch build and require private listener selection. Route the remaining mock and PostgreSQL allocator gaps to their test owners. |
| logic-r1 05 | Major | C11 | fixed | Scope both holds to the old stream generation. Discard them on close and witness a fresh candidate replay. |
| logic-r1 06 | Major | R10 | fixed | Duplicate of critic-r1 06. Use the published append and pre-commit hooks with record correlation. |
| logic-r1 07 | Major | R10 | fixed | Prove real permit occupancy, claim cursor, blocked second write and sweep through paired cases and named component tests. |
| logic-r1 08 | Major | plan | fixed | Duplicate of critic-r1 08. Require a seeded real repository in R13 and join Website warning evidence to the paired history flag case. |
| logic-r1 09 | Minor | R8 | fixed | Keep separate stopped-client remedies and state assertions for terminal history and incompatible Welcome. |

## Points and stuck statements

Each blocker contributes 9 points, each major 3 and each minor 1. A duplicate group carries its highest severity under the primary finding’s statement. All other statements score zero.

| Statement | Points |
|---|---:|
| R11 | 15 |
| C1 | 12 |
| C11 | 3 |
| C12 | 3 |
| D19 | 3 |
| R6 | 3 |
| R9 | 3 |
| R10 | 3 |
| R14 | 3 |
| plan | 2 |
| R8 | 1 |
| **Round total** | **51** |

There are no stuck statements. R11 accounts for 15 of 51 points, or 29.4 percent. C1 accounts for 12 of 51, or 23.5 percent. Each clears 12 points and falls below the 30 percent threshold.

## Mechanisms reconsidered

A resolved model and Compose labels were inadequate evidence of ownership. The gate now requires proof before it can select or touch a stack, and the native supervisor runner cannot accept an external prepared selection. The joint gate’s single proposed shell driver was also the wrong boundary. It gave black-box checks responsibility for database and scheduling internals. The fold uses the frozen driver, the already assigned paired suite and the deployed evidence. The relay remains necessary for the installed-state replay, where stream-bound retirement replaces the one mutable hold. The installed cutover follows the operator launcher and uses the committed migration ledger as its recovery boundary. Rewinding a database remains covered by issue 1676. [Issue log](../../../../docs/issues.md#1676-a-server-database-set-back-in-time-and-a-client-history-that-cannot-be-told-from-the-real-one-are-not-supported)

## Plan patches

| Heading in the plan | Patch |
|---|---|
| Overview, Key decisions, Workstreams and Rollout order | State when the website returns, when recovery must go forward, and what both deployed adds and client stops prove. |
| Implementation details, Item 1 | Isolate source writes, claim fresh fixture resources and record ownership before use. Keep only Stack’s own checkout fixtures. |
| Implementation details, Items 2, 5 and 6 | Specify stream-bound relay retirement, separate the certificate runner, and use its observations and the installed failure procedure in the guide. |
| Implementation details, Items 3 and 4 | Read closed legacy state, prove fresh replay and repair across cutover, and narrow native service setup to an owned stack and user. |
| Implementation details, Exact interfaces for the joint gate | Adopt the published test seams and contract names. |
| Implementation details, Rollout detail, Design check assignment, Additional required case evidence, Deployed browser proof and Post-build checks | Bind commands, owners and boundaries. Add the frozen browser handoff, installed recovery decisions and repeatable runtime scenarios. |
| Traceability, Guardrails, Execution sizing and Attack log | Map the updated scope and evidence, count the six units, and link this round. |

## Top finding and reviewer grades

The top finding was the C1 blocker, logic-r1 01. A gate could issue lifecycle calls and revoke sessions on an existing installation that happened to match its selection. The R11 runner mismatch was also a blocker.

| Reviewer | Verdict | Grade | Findings by severity |
|---|---|---|---|
| [Critic](round-1-critic.md) | Revise | 5 out of 10 | 1 blocker, 8 majors and 2 minors. |
| [Logic](round-1-logic.md) | Revise | 5 out of 10 | 1 blocker, 7 majors and 1 minor. |
