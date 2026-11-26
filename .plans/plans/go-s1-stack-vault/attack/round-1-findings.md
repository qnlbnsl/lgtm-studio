# Round 1 findings for PostgreSQL and OpenBao

The fold uses the approved foundation design, the baseline snapshot and the five available reports. The reviewed commit is `3ef69e50`. The database and vault are planned services. Repository code available for this fold was the wave 1 test driver and its contract with the plans. The reviewer runtime observations and this session's Docker access denial are recorded in scouting.

## Finding outcomes

| Finding | Severity | Reported statement | Outcome | Evidence and disposition |
|---|---|---|---|---|
| architect-r1 1 | major | C13 | fixed | The proof now uses the production identity, database owner and negative key controls. The replay remains gated because local Docker access was denied. |
| architect-r1 2 | major | C12 | fixed another way | A loopback-only disposable command wrapper supplies the real database and the database-creation privilege without adding a production Compose service. |
| architect-r1 3 | major | D9 | fixed | Grouped with logic2-r1 1. Publication flushes and renames a hidden file within the destination bind mount. The test runs with both mounts present. |
| architect-r1 4 | major | D9 | fixed another way | Grouped with threat-r1 1. The initial policy stays narrow. An explicit operator command can apply a later policy without retaining a root token or modifying state on startup. |
| architect-r1 5 | major | D4 | fixed | Retryable reconciliation restores missing role and database setup before the health wait. |
| architect-r1 6 | major | C13 | fixed | Grouped with logic-r1 2. The completed path checks active storage and a real server-role login. The guide defines both mismatch directions and the guarded reset. |
| architect-r1 7 | minor | D10 | fixed | The Compose images and helper base use the recorded digests. The helper pins its dependencies and only builds when absent. |
| architect-r1 8 | minor | D4 | fixed | Grouped with logic-r1 1. Each login owns its own database and OpenBao creates its own table without a public creation grant. |
| architect-r1 9 | minor | plan | fixed | A single sprint 2 table gives the database file, vault settings, runtime user, network and launcher contracts. |
| architect-r1 10 | minor | R9 | fixed | File binds forbid host path creation, and the guide and preparer identify the supported first-start path. |
| critic-r1 1 | major | D10 | fixed | Grouped with logic-r1 1. The owner grants, external connection test and restricted OpenBao proof cover the reproduced failure. |
| critic-r1 2 | major | plan | fixed | Test-owned response gates and command shims cover interrupt points. Fixed phase lines expose launcher order. |
| critic-r1 3 | major | D10 and C1 | fixed another way | Grouped with logic-r1 3. State tables group equivalent states. Unrecoverable initialization uses an explicit bound-installation reset rather than a guessed empty-vault test. |
| critic-r1 4 | major | plan | fixed | The plan names directory modes and owners. A privileged fixture helper inspects and cleans the two disposable installations. |
| critic-r1 5 | minor | D4 | fixed another way | Grouped with logic-r1 4. Initialization now requires passwords on network and socket connections. The wrong-password test preserves the authenticated readiness claim. |
| critic-r1 6 | minor | D10 and C13 | fixed | The planner records the existing reviewer transcript. Builders send sanitized exact-commit evidence to the planner or orchestrator. |
| critic-r1 7 | minor | plan | fixed | Items 5a and 5b have separate scripts and frozen tests for initialization and publication. |
| critic-r1 8 | minor | plan | fixed | An independent fixture item owns the shared test function names and precedes parallel service work. |
| critic-r1 9 | minor | D9 | fixed | Grouped with threat-r1 1. The guide and operator command define how to replace and retire the existing identifier after initial root revocation. |
| critic-r1 10 | minor | R10 | fixed | Grouped with architect-r1 7. The image and package pins, context, missing-source binds and bounded health settings are specified. |
| critic-r1 11 | minor | C13 | fixed | The plan requires executable startup and one result line per test. It uses capability settings and removes the unsupported configuration field. |
| critic-r1 12 | minor | plan | not folded | No separate style patch. Revised technical sections use lists or tables, and AppRole is expanded at first use. |
| logic-r1 1 | major | D10 | fixed | Database owners, storage-table creation and denial of broad grants are explicit. |
| logic-r1 2 | major | D10 and C13 | fixed | The checkpoint and state table define initialization, completion, stale files, unavailable material and the repair action. |
| logic-r1 3 | major | D10 | fixed another way | A confirmed whole-installation reset is documented. A locked or unauthenticated vault cannot reliably prove that it is empty. |
| logic-r1 4 | major | D4 and R10 | fixed | Database initialization requires SCRAM for network and local access. The health path rejects a wrong password. |
| logic-r1 5 | major | C13 | fixed | Grouped with architect-r1 1. The proof uses the restricted owner and production launcher, adds the right-length wrong-key control, and records the review run separately. |
| logic-r1 6 | minor | D10 | fixed | Both local-state-only guards have assertions. Startup prints classified errors without relaying raw logs. |
| logic-r1 7 | minor | D10 | fixed | The OpenBao wrapper, config path, server command and PostgreSQL storage check are named. |
| logic-r1 8 | minor | D10 | fixed | Grouped with architect-r1 7. Daemon and helper references use the recorded digest and the helper pins both packages. |
| logic-r1 9 | minor | plan | fixed | Grouped with critic-r1 8. The shared test library comes first. The items that share Compose and prepare files remain sequential. |
| logic2-r1 1 | blocker | D10 | fixed | The destination-mount atomic rename removes the deterministic publication failure after revocation. |
| logic2-r1 2 | major | D9 | fixed another way | Grouped with threat-r1 1. The local operator identity has management over the one named policy and the server's AppRole identifiers, with no root-generation endpoint. |
| logic2-r1 3 | major | D4 | fixed | Grouped with logic-r1 4. Authentication is tested with an incorrect password while the role remains enabled. |
| logic2-r1 4 | major | D4 | fixed | Grouped with architect-r1 5. Existing cluster state and role setup completion are separate. Reconciliation uses the original passwords and volume. |
| logic2-r1 5 | major | D10 | fixed another way | Staging survives until the file and checkpoint are durable. A recovery or completed run uses one read-only login to rule out stale published credentials. |
| logic2-r1 6 | major | D10 | fixed another way | The state-root lock and atomic claim on the daemon's project volume reject a foreign root before changing service mounts. |
| logic2-r1 7 | minor | C13 | fixed | The init status selects the branch. Bounded active health gates the first privileged request or application-role login. |
| logic2-r1 8 | minor | plan | fixed | Grouped with architect-r1 7. Item 1 records the platform. The Compose pin and the helper base are compared with the proof. |
| threat-r1 1 | major | D9 | fixed another way | Separate management authority covers operator credential maintenance. It does not enable the deprecated unauthenticated root path. |
| threat-r1 2 | major | D10 | fixed another way | Grouped with logic-r1 3. The response is made durable before parsing. Lost material has an explicit confirmed reset path without a purported empty-vault heuristic. |
| threat-r1 3 | major | C13 | fixed | Grouped with logic-r1 2. The launcher refuses the mismatched volume or uninitialized store with stage-specific messages. The reset is tested in an isolated project. |
| threat-r1 4 | major | R9 | fixed | Grouped with logic-r1 4. The specified initdb rules and wrong-password mutation remove the loopback trust loophole. |
| threat-r1 5 | minor | plan | fixed | Grouped with logic-r1 1. Role ownership and access to the vault's storage table are tested independently. |
| threat-r1 6 | minor | D10 | fixed | Grouped with architect-r1 7. Image digests, helper dependency versions and a digest assertion are part of the build handoff. |
| threat-r1 7 | minor | D10 | fixed | Internal networks, reduced capabilities and no new privileges are specified and verified. |
| threat-r1 8 | minor | R9 | fixed | The helper context excludes state and has no project copy instruction. A sentinel fixture checks the boundary. |
| threat-r1 9 | minor | D10 | fixed another way | Grouped with logic2-r1 5. Checkpoint interruption tests cover registration and publication. The pinned image fixture measures the absence response before the builder uses it. |

## Points and stuck statements

The 48 reported findings form 26 groups. Each group contributes only its highest severity. When reports cite different statements, the points go to the one most directly affected by the common mechanism. Database ownership and authentication go to D4. Installation matching goes to the restart observation in C13. Evidence ownership belongs to the plan workflow.

| Statement | Group representatives | Points |
|---|---|---|
| D4 | architect-r1 5, logic-r1 1, logic-r1 4. | 9 |
| D9 | threat-r1 1. | 3 |
| D10 | logic2-r1 1, architect-r1 7, logic-r1 3, logic-r1 6, logic-r1 7, logic2-r1 5, logic2-r1 6, threat-r1 7. | 22 |
| R9 | architect-r1 10, threat-r1 8. | 2 |
| C12 | architect-r1 2. | 3 |
| C13 | architect-r1 1, logic-r1 2, critic-r1 11, logic2-r1 7. | 8 |
| plan | architect-r1 9, critic-r1 2, critic-r1 4, critic-r1 6, critic-r1 7, critic-r1 8, critic-r1 12. | 11 |

The total is 58 points. The groups comprise 1 blocker, 12 majors and 13 minors.
The stuck statement is D10 with 22 of 58 points (37.9%). No other statement meets both thresholds.

## Patches for the next round

| Plan heading | Changes to review |
|---|---|
| Overview, Key decisions, Workstreams and Rollout order | The baseline facts now cite the reviewer runs. The rollout gates the production shape and observes operator maintenance. |
| Work item 1 and Work item 8 | Production-role proof, negative key controls, planner-owned receipt, isolated fixtures and disposable host PostgreSQL. |
| Work item 2 | Database ownership, authenticated readiness, retryable role setup, image pins and project-volume binding. |
| Work item 3 | Explicit OpenBao invocation, key guards and diagnostics, internal network and capability checks. |
| Work item 4 | Scoped operator role, reproducible tools context, identifier lookup classification and access tests. |
| Work item 5a and Work item 5b | Separate initialization and publication scripts, active readiness, checkpoint transitions and a destination-mount rename. |
| Contract for sprint 2 | Runtime user, database password file, vault settings and executable launcher ownership. |
| Work item 6 and Operator recovery | Installation binding, explicit reset and restore paths, deterministic fixture signals and portable private-file teardown. |
| Work item 7 | Local operator policy application and credential rotation after initial root revocation. |
| Post-build checks, Traceability, Guardrails, Execution sizing and Attack log | Connect remaining runtime observations, nine scoped work items and this register. |

The former bootstrap mechanism falls short as a unit. Multiple major findings and the blocker stem from mixing local file existence, vault health and root revocation in one vague completion test. The split and the checkpoint table replace that logic. The database's one-time role setup also had two independent major failure modes. Reconciliation and explicit authentication rules address them separately.

## Post-build observations

| Plan checks | Report sources |
|---|---|
| PB1 and PB2 | Architecture checks 1 to 3 and 7. Logic checks 1, 4, 6 and 7. Logic 2 checks 1 to 3. Critic checks 1 and 2. |
| PB3, PB5, PB6 and PB7 | Threat checks 1 to 4 and 6 to 8. |
| PB4 and the fixture cleanup assertion | Architecture checks 4 to 6. Logic checks 2, 3 and 5. Critic checks 4 to 6. Threat check 5. |
| PB8 | Logic 2 check 4 and critic check 3. The existing runner checks both halves of C13, as stated in `deploy/test/README.md:24` and `deploy/test/checks.sh:619`. The standalone sprint 1 suite checks the vault half. |

## Top finding and reviewer verdicts

The top finding is the blocker logic2-r1 1. A rename across two Docker bind mounts can fail after revoking the original administrator token. The patch stages and renames the file in the destination mount and retains the journal until publication is durable.

| Reviewer | Verdict | Grade out of 10 | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| [Architect](round-1-architect.md) | REVISE | 5 | 0 | 6 | 4 |
| [Critic](round-1-critic.md) | REVISE | 5 | 0 | 4 | 8 |
| [Logic](round-1-logic.md) | REVISE | 5 | 0 | 5 | 4 |
| [Logic 2](round-1-logic2.md) | REVISE | 5 | 1 | 5 | 2 |
| [Threat](round-1-threat.md) | REVISE | 6 | 0 | 4 | 5 |

## Owner decisions

No finding requires a product decision in this fold. D10 meets the stuck threshold under the build loop. The orchestrator should carry it to the owner with the points and remedies the register records.
