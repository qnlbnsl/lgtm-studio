# Round 2 findings for PostgreSQL and OpenBao

The three reports reviewed the patches through the first fold at `cd46223f`. The current planning checkout was `f86d7d71` when this fold began. The approved foundation design, the round 2 snapshot and the existing rollout driver ground the dispositions below. [Scouting](../scouting.md#round-2-sources-and-runtime-limits) records the current source checks and the reviewers' runtime observations.

## Finding outcomes

| Finding | Severity | Reported statement | Outcome | Evidence and disposition |
|---|---|---|---|---|
| architect-r2 1 | major | D4 and C5 | fixed | Item 2 updates the Stack-owned query helper to use the mounted application password. Its own test exercises the exact helper with standard input. |
| architect-r2 2 | major | plan | fixed another way | Item 2 owns the binding protocol and private directories. Item 8 owns fixture key operations. Item 6 consumes both. The uninitialized-state test now belongs to item 5a. |
| architect-r2 3 | major | C12 | fixed | Item 8 writes the executable mutation wrapper and documents the full driver invocation. It tests the missing Server script and exact exit status propagation. |
| architect-r2 4 | major | D10 and C13 | fixed another way | The recovery table covers missing volume, orphaned volume, moved state, foreign identity and retained unclaimed data. The orphan case requires an explicit identifier and separate fixture. |
| architect-r2 5 | minor | plan | fixed | The sprint 2 table freezes all database setting names and states the approved vault-path procedure. The result asks the orchestrator to relay those names. |
| architect-r2 6 | minor | D9 and D10 | fixed | Both maintenance commands require the published checkpoint, and the frozen test checks that earlier checkpoints cause no write. |
| architect-r2 7 | minor | D9 and R9 | fixed | Grouped with threat-r2 1. The direct-access assertion and operator guide now describe the effective administrator authority of the protected file. |
| architect-r2 8 | minor | D4 and R9 | fixed | The item 2 table names exact users and capability sets. Running the database image as its existing numerically identified user avoids a root switch. Privileged jobs declare why each added capability is needed. |
| architect-r2 9 | minor | plan | fixed | The acceptance command in each item is the full gate. Workstream summaries name representative observations. |
| logic-r2 1 | major | D10 | fixed another way | Production publication uses a rename-only command. The fixture requires that command, its option and success on the real layout. The cross-mount mutation and direct-copy mutation fail the positive assertion. |
| logic-r2 2 | major | D10 and R9 | fixed | A durable reset marker covers both removals. The lock uses the retained directory inode. The interruption test proves the same command can resume. |
| logic-r2 3 | major | D10 | fixed another way | Grouped with architect-r2 4. Path is diagnostic only, so a matching identity survives relocation without rewriting ownership. Failed claims remove their own provisional record. |
| logic-r2 4 | major | D9 | fixed | Rotation owns a separate journal and candidate file. Startup detects the pending journal and stops. Bootstrap cleanup names only its own files. |
| logic-r2 5 | minor | D10 and C13 | fixed | Key, root token, response and checkpoint validity have explicit conditions. Truncation and mode cases have named assertions. |
| logic-r2 6 | minor | D10 | fixed another way | The material table records required files per state. The recovery share is transient because the chosen operator path never uses it. Completed setup retains the static seal and operator credential. |
| logic-r2 7 | minor | D4 and R10 | fixed | The readiness test substitutes each role password independently. Testing only one role fails the other control. |
| logic-r2 8 | minor | D4 | fixed | Grouped with architect-r2 8. The reconciler runs as user and group 70 without added capabilities. Frozen inspection checks both. |
| logic-r2 9 | minor | D10 | fixed | Grouped with architect-r2 2. Item 2 owns and exercises the claim before the launcher exists. Item 3 consumes the existing directory layout. |
| logic-r2 10 | minor | D9 | fixed | Grouped with logic-r2 4. Rotation tests four interruption boundaries and requires a single surviving candidate with no orphan accessors. |
| logic-r2 11 | minor | D10 | fixed | The setup job measures 60 seconds from its start using elapsed monotonic time. Timeout preserves the checkpoint and emits a retryable health diagnostic. |
| threat-r2 1 | major | D9 | fixed | The operator file is standing administrator material. A disposable test checks its direct restrictions and the policy-change effect. Applying the reviewed policy requires a visible diff and explicit confirmation. |
| threat-r2 2 | major | D10 | fixed | Grouped with logic-r2 2. A private marker supports retry. The allowlisted manifest rejects foreign entries, symbolic links and nested mounts before any removal. |
| threat-r2 3 | minor | R9 | fixed | The reconciler rejects unsafe or invalid authentication rules. A retained-volume test verifies the failure seen by the reviewer. |
| threat-r2 4 | minor | D10 | fixed | The plan and operator guide account for the observed Docker bridge access to host listeners. PB9 checks it on the build host. |
| threat-r2 5 | minor | R9 | fixed | The disposable password uses a protected file. Fixture reports contain only status, boundary names and counts. Cleanup and command arguments are checked. |
| threat-r2 6 | minor | D9 | fixed | Grouped with logic-r2 6. The operator identifier lasts until reset in this sprint. Setup deletes its unused recovery share after provisioning is durable. |
| threat-r2 7 | minor | D10 | fixed | The helper test requires exact installed package versions and records the built image identifier. An unavailable pin stops the build with a diagnostic. |
| threat-r2 8 | minor | D10 | fixed | Grouped with architect-r2 8. Each running service and added job is checked against its named capability set. |
| threat-r2 9 | minor | D10 | fixed | The file table names the writers, readers and lifetimes. The operator file is durable before revocation. The absent publication material state gets its own restore diagnostic. |
| threat-r2 10 | minor | C13 | fixed | The sprint 2 table names the Server writer and the existing driver's exact signing-key event and field names. |

## Points and stuck statements

The 30 findings form 22 groups. Count each group once at its highest severity. Cross-statement groups go to the statement most directly affected by the shared mechanism. Service and job capability limits belong to D10, the unit ownership gap belongs to the plan, and the recovery-file lifetime belongs to the bootstrap contract in D10.

| Statement | Group representatives | Points |
|---|---|---|
| D4 | architect-r2 1, logic-r2 7. | 4 |
| D9 | architect-r2 6, logic-r2 4, threat-r2 1. | 7 |
| D10 | architect-r2 4, architect-r2 8, logic-r2 1, logic-r2 2, logic-r2 5, logic-r2 6, logic-r2 11, threat-r2 4, threat-r2 7, threat-r2 9. | 16 |
| R9 | threat-r2 3, threat-r2 5. | 2 |
| C12 | architect-r2 3. | 3 |
| C13 | threat-r2 10. | 1 |
| plan | architect-r2 2, architect-r2 5, architect-r2 9. | 5 |

The total is 38 points across eight major and 14 minor groups. D10 remains stuck at 16 of 38 points, or 42.1%. It was also stuck in round 1. No other statement meets both thresholds.

## Patches for the next round

| Plan heading | Changes to review |
|---|---|
| Key decisions and Workstreams | Describe the authority of the local operator and make each item's acceptance command a full gate. |
| Work item 8 | Add the mutation hook, fixture key operations, credential handling and interruption cleanup. |
| Work item 2, Installation binding and private directories, and Container users and capability limits | Own the binding before the launcher exists. Check restored database authentication rules, both role passwords and the existing rollout query helper. Name exact process privileges. |
| Work item 3 | Consume the earlier file layout. Check malformed keys and document the host-bridge boundary. |
| Work item 4 | State the operator's effective authority and lifetime. Freeze helper image and package observations. |
| Work item 5a | Define valid material and the retryable health deadline. Check sealed, interrupted and invalid setup states. |
| Work item 5b and Contract for sprint 2 | Name the files and checkpoint lifetimes. Require rename-only publication. Freeze settings, policy ownership and signing-key event fields. |
| Work item 6 and Operator recovery | Retain the lock through reset. Bound deletion and cover interrupted reset, relocation and missing records without crossing installation boundaries. |
| Work item 7 | Require a published bootstrap, preview and confirm policy changes, and isolate rotation state across interruptions. |
| Post-build checks, Traceability, Execution sizing and Attack log | Record the exact runtime follow-ups, both database-driver read paths and the round 2 fold. |

The original round 1 recovery mechanism was incomplete as a unit. It made the only paired proof of ownership disappear before reset completed and made a legitimate installation move look foreign. The revised mechanism keeps durable reset intent and uses stable installation identity. The bootstrap and rotation also shared a final credential path without clear ownership of staging. The patch gives each process its own files and makes an interrupted rotation visible to startup.

## Post-build observations

| Plan checks | Report sources |
|---|---|
| PB1 and PB13 | Architect checks D and E, logic check 6 and threat checks 6 and 7. The documented protected-file jobs are the only file-permission-override exceptions. |
| PB3 and item 8 fixture tests | Threat check 5. Inspect actual disposable arguments and private fixture reports. |
| PB9 | Threat check 3. Record host bridge reachability. |
| PB10 and item 2 database tests | Architect checks A and B and threat check 2. Check the old-volume rules and the rollout helpers. |
| PB11 | Architect check C, logic checks 2 and 3 and threat check 4. |
| PB12 | Logic checks 1, 4 and 5. The rename-only assertion fails before a cross-mount copy can start. |
| PB14 | Threat checks 1 and 8. The direct root-generation denial is observed for the operator token and without a token. The authority from changing the server policy is acknowledged. No supported maintenance step retains or uses a recovery share after provisioning. |

## Top finding and reviewer verdicts

The top finding is a major, logic-r2 1. The original fixture observed only the boundaries of a move that could silently copy across mounts. The replacement observes a required rename-only call. It makes a cross-mount attempt fail without publishing a partial server credential.

| Reviewer | Verdict | Grade out of 10 | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| [Architect](round-2-architect.md) | REVISE | 6 | 0 | 4 | 5 |
| [Logic](round-2-logic.md) | REVISE | 6 | 0 | 4 | 7 |
| [Threat](round-2-threat.md) | REVISE | 6 | 0 | 2 | 8 |

## Owner decisions

No finding requires a separate product decision. The orchestrator carries the repeated stuck result for D10 to the owner under the build loop. The remaining build evidence is listed in the plan and scouting.
