# Round 3 findings for PostgreSQL and OpenBao

The two reports reviewed the second fold through `040b6ce0`. The architecture review also read four further working-copy changes. The orchestrator's ruling on 2026-11-26 directs the fold to remove work with no requirement or check in wave 1. [Current scouting](../scouting.md#round-3-sources-and-current-anchors) records the sources used to check the findings.

## Finding outcomes

| Finding | Severity | Reported statement | Outcome | Evidence and disposition |
|---|---|---|---|---|
| architect-r3 1 | major | D9 | fixed | Item 4 saves the one-time administrator response in one protected file. The extra role, rotation and maintenance item are removed. The server keeps its exact-path policy. |
| architect-r3 2 | major | D10 | fixed | Items 2, 3 and 6 keep the password and seal guards and the directory lock. The volume claim and automated reset are removed. The guide covers a known unfinished installation. |
| architect-r3 3 | major | D10 | fixed | Item 4 uses the saved response and final file as the two facts. Publication remains within one directory. The proxy, shims and step file are removed. PB2 checks the remaining initialization boundary. |
| architect-r3 4 | major | D10 | fixed | Item 4 uses the daemon's pinned image and its command-line interface. The helper image, exact package installs and helper tests are removed. |
| architect-r3 5 | major | C12 and R10 | fixed | Item 8 creates `lgtm_store_test`, names its writers and readers, and asserts the connection name, owner and database-creation privilege. |
| architect-r3 6 | major | plan | fixed | The ruling removes the rotation journal and the startup refusal that made a stopped stack unrecoverable. |
| architect-r3 7 | minor | plan | fixed | The helper image and its cross-item test are removed. The retained setup explicitly depends on the pinned daemon from item 3. |
| architect-r3 8 | minor | plan | fixed | The contract gives the web plan its existing launcher fallback and makes item 2 the only writer of the C5 helper and guide entry. The result relays the five Server settings. |
| logic-r3 1 | major | D10 and D9 | fixed | Grouped with architect-r3 1. Save the administrator response at the unseal key's owner and mode. Keep scope to first start and defer rotation. |
| logic-r3 2 | major | D10 | fixed | Grouped with architect-r3 2. Remove the paired identities and claim protocol. Keep private directories, authentication, and the seal and stale-state diagnostics. |
| logic-r3 3 | major | D10 | fixed | Delete the reset flags, marker, manifest and orphan flow. Item 6 documents manual cleanup for a known unfinished disposable installation. |
| logic-r3 4 | major | D10 | fixed | Grouped with architect-r3 3. The saved response and final credential replace all named checkpoints. Repeat launch leaves both facts unchanged. |
| logic-r3 5 | major | plan and C12 | fixed | Grouped with architect-r3 3. Remove special boundary fixtures. Plain tests use disposable projects and keep the actual C12 database and hook scripts. |
| logic-r3 6 | major | D10 | fixed | Removing orphan reset removes the empty-root replay contradiction. |
| logic-r3 7 | major | D10 | fixed | Startup has no reset marker to block on, and the manual guide does not operate on an unrelated volume. |
| logic-r3 8 | minor | D10 | fixed | The removed volume claim leaves no losing-claim scratch file to clean. |
| logic-r3 9 | minor | D10 | fixed | The orphan reset and its required Compose labels are gone. |
| logic-r3 10 | minor | plan | fixed | Grouped with architect-r3 8. The contract links the Server settings and Stack web consumers. The result asks the orchestrator to relay it into the epic and web brief. |

## Points and stuck statements

The 18 findings form 13 groups. Count each group once at the highest severity. Assign the common bootstrap and boundary-fixture finding to D10. Count defects in the removed reset and rotation independently when they describe different failure scenarios.

| Statement | Group representatives | Points |
|---|---|---|
| D9 | architect-r3 1. | 3 |
| D10 | architect-r3 2, architect-r3 3, architect-r3 4, logic-r3 3, logic-r3 6, logic-r3 7, logic-r3 8, logic-r3 9. | 20 |
| C12 | architect-r3 5. | 3 |
| plan | architect-r3 6, architect-r3 7, architect-r3 8. | 5 |

The total is 31 points across nine major and four minor groups. D10 remains stuck at 20 of 31 points, or 64.5%. No other statement meets both thresholds. The score describes the reviewed plan before the cuts.

## Patches for the next round

| Plan heading | Change |
|---|---|
| Overview, Key decisions, Workstreams and Rollout order | Reduce the first-start, support and restart scope to the required behavior. State the protected administrator file and manual recovery boundary. |
| Work item 1 and Work item 8 | Keep the production-shape proof and C12. Name the disposable database and remove shared interruption fixtures. |
| Work item 2 and Work item 3 | Remove the claim protocol. Retain authenticated PostgreSQL setup, private files, pinned daemons and the seal and storage checks. |
| Work item 4 | Combine initialization, provisioning and publication. Keep two facts, one restricted role and the pinned image's commands. |
| Contract for sprint 2 | Name both readers, the C5 edit owner and the web plan's launcher fallback. |
| Work item 6 and Post-build checks | Replace reset with documented manual cleanup for a known unfinished instance. Keep an ordinary second launch and the whole-stack restart check. |
| Traceability, Guardrails, Execution sizing and Attack log | Update the six units, the sources and the effect on previous rounds. |

The prior recovery machinery was too broad as a unit. The identity and reset depended on each other, and the reset refused its own replay in one scenario. Removing both preserves the checks that already guard passwords and the seal. The operator identity and its rotation introduced a startup dead end while duplicating local administrative authority. One administrator file and the server's existing credential boundary provide the behavior wave 1 needs.

The ruling supersedes earlier patches for operator authorization and maintenance, rotation and root revocation, binding and reset, and their checkpoint and fixture machinery. The previous registers remain as evidence of those mechanisms. Decision rows 34 to 37 explain their removal. The fixes for database ownership, password authentication, private mounts, the final credential format and ordinary vault restart remain active.

## Post-build observations

| Retained check | Review source and expected result |
|---|---|
| PB1 | Architecture check 1. A clean host starts using the two existing images and requires no helper build. |
| PB2 and PB3 | Logic checks A and D. A saved response before provisioning leads to one published credential. Only the two bootstrap facts remain after first start. |
| PB4 and the launcher suite | Architecture checks 4 and 5 and logic check B. Startups sharing a state directory serialize. After restart the vault opens and the signing-key read stays the same. |
| PB5 | Architecture check 2 and logic check C. The actual disposable maintenance name is accepted, and the hook returns the expected mutation result. |

The conditional rotation and orphan reset probes lapsed with those features. The first-start retry stops if a server identifier already exists and the final file is absent. No extra journal or recovery command is needed for that state in wave 1.

## Top finding and reviewer verdicts

The top finding is a major, architect-r3 1, independently reported as logic-r3 1. The operator role and rotation added standing administrative authority and startup states without serving a wave 1 check. The fold retains the first-start response and limits the server to its signing-key path.

| Reviewer | Verdict | Grade out of 10 | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| [Architect](round-3-architect.md) | REVISE | 5 | 0 | 6 | 2 |
| [Logic](round-3-logic.md) | REVISE | 5 | 0 | 7 | 3 |

## Owner decisions

No finding requires a new product decision in this fold. The orchestrator carries the repeated D10 stuck result and the narrower scope through the build loop.
