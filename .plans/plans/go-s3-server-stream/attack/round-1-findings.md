# Round 1 findings for the server stream

The fold compares the frozen review copy with the approved design, the revision 1 link and the checkout at commit `e8733544`. The [scouting record](../scouting.md) lists the source anchors. C numbers identify wave 1 checks, D numbers identify design decisions, and R numbers identify foundation rules. An outcome applies to every report row, including duplicates.

## Dispositions

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r1 01 | Major | C11 | fixed | Item 10 relays the acknowledgement barrier and P4 assigns a database commit barrier to the combined suite. |
| architect-r1 02 | Major | C10 | fixed | Item 7 records when Proof completed and preserves that result across an independently bounded lookup. |
| architect-r1 03 | Major | C6 | fixed | The rollout gates Part A on its PostgreSQL tests and runs deployed C5 and C6 after the Client and Stack prerequisites. |
| architect-r1 04 | Major | C11 | fixed | Stack item 2.3 owns the deployed mark observation and the orchestrator requires the independent P4 effect proof for sign-off. |
| architect-r1 05 | Major | C8 | fixed another way | Items 3 and 5 prioritize first attempts in a separate bounded lane and requeue lock contention without waiting on the contended machine. |
| architect-r1 06 | Minor | plan | fixed | The orchestrator must pin and verify the passing Part A merge before dispatch. The implementation and the fixture are still pending at the committed baseline. |
| architect-r1 07 | Minor | R4 | fixed | An applied report updates the connection's last-heard time within the event write. |
| architect-r1 08 | Minor | D3 | fixed | The handshake uses the store's version 7 universally unique identifier (UUID) generator and tests the stored format. |
| architect-r1 09 | Minor | plan | fixed | Item 4 checks the returned and durable marks, and item 8 tests diagnostic emission through Open. |
| critic-r1 01 | Blocker | R10 | fixed | The scripted test uses the generated route with the private handshake continuation in `handshake_test.go`. |
| critic-r1 02 | Major | C10 | fixed another way | The first-write guard arms the Welcome deadline after authorizing the write and preserves the response for a pre-entry refusal. |
| critic-r1 03 | Major | R10 | fixed | The website work item needs item 4's committed report operations. |
| critic-r1 04 | Major | R8 | fixed another way | The Open wrapper refuses a declared oversize before the protocol reader can drain the payload. |
| critic-r1 05 | Major | C11 | fixed | This duplicates architect-r1 01. P4 observes exact outbox entries, database effects and the Welcome mark at the two distinct cuts. |
| critic-r1 06 | Major | C11 | fixed | Item 8 checks Welcome's wire mark in both orders of the event-41 admission race. |
| critic-r1 07 | Major | C9 | fixed | Item 10 fixes the startup phase and diagnostic and tests a partial clear followed by successful startup on the same database. |
| critic-r1 08 | Minor | plan | fixed | This duplicates architect-r1 09. Item 8 observes that repeats and rolled-back writes emit no applied diagnostic. |
| critic-r1 09 | Minor | plan | fixed another way | Item 3 absorbs the small registry operations while retaining their assertion-red tests and the epic's nine-item limit. |
| critic-r1 10 | Minor | R10 | fixed | P1 and P10 cover the Join metadata and owner-call token boundary. P5 assigns the command-specific observations. |
| logic-r1 01 | Major | R6 | fixed | Item 5 closes failed subscription attempts and retries transient failures from the head read. |
| logic-r1 02 | Major | C10 | fixed | The stream checks identity before frame rules and mutating frames check under the write lock. |
| logic-r1 03 | Major | D7 | fixed | This duplicates architect-r1 02. The phase timestamp and tests preserve both lookup precedence and on-time receives. |
| logic-r1 04 | Major | C8 | fixed another way | This duplicates architect-r1 05. The cleanup lanes use the nonwaiting lock and accept obligations only after a confirmed or possible commit. |
| logic-r1 05 | Minor | R4 | fixed | The approval loader observes the uncommitted revision and a forced failure rolls back approval, feed and receipt. |
| threat-r1 01 | Major | D7 | fixed | Item 7 validates the registered key before signature verification and specifies the pinned dependency and seeded-row test. |
| threat-r1 02 | Major | C8 | fixed another way | This duplicates architect-r1 05. Item 5 bounds concurrency and separates the first attempt from retries. |
| threat-r1 03 | Minor | D7 | fixed another way | The server removes forwarding-header trust and leaves the optional address empty on the private stack. |
| threat-r1 04 | Minor | R9 | fixed | Open sets `Cache-Control: no-store` before Challenge and P10 checks the header and credential redaction through the proxy. |
| threat-r1 05 | Minor | R9 | for the owner | The design protects signing material and leaves the disk-copy risk for ordinary machine metadata open. |

## Points and stuck statements

A minor finding earns one point, a major earns three, and a blocker earns nine. Each duplicate group earns its maximum once. The Proof timing group is charged to C10, where it first appears, with D7 retained on the logic report row.

| Statement | Points |
|---|---:|
| D3 | 1 |
| D7 | 4 |
| R4 | 2 |
| R6 | 3 |
| R8 | 3 |
| R9 | 2 |
| R10 | 13 |
| C6 | 3 |
| C8 | 3 |
| C9 | 3 |
| C10 | 9 |
| C11 | 9 |
| plan | 3 |
| **Total** | **58** |

No statement is stuck. R10 reaches 13 points and accounts for 22.4 percent of the round. The threshold also requires 30 percent.

## Mechanisms examined for overlapping findings

The three cleanup reports identify the same serial queue failure. Per-attempt timeouts cannot stop a retried row from delaying other machines. The fold replaces the single queue worker with separate first-attempt and retry service and a nonwaiting row lock. This mechanism needs review.

The replay acceptance has two independent major gaps. It needs a reachable boundary on either side of commit and an observation of the mark in the replacement's actual Welcome. It also had incompatible log-based evidence at the deployed gate. The fold keeps the transactional inbox and replaces the evidence with a reachable commit barrier, an acknowledgement hook, a wire observation and an independent deployed mark check.

The handshake has a blocker in test reachability and major findings in phase timing, the first-write boundary and registered-key validation. Keep the frozen handshake sequence and the private continuation. The replacement finding requires moving the transport deadline to the first permitted write. The registered-key finding requires validating the stored key independently of that timing.

## Place for the next review

| Plan heading | Changed area |
|---|---|
| What the owner sees when a machine connects | The optional address on the private stack and the volume risk decision. |
| Key decisions | The database presence and process action boundary. |
| Rollout order and the workstreams' rollout details | Part A merge pin, deployed C5 and C6 prerequisites, Stack G4 and P4. |
| Implementation details and build conventions | Registry locking and the writer of committed diagnostics. |
| Workstream 1, items 2 and 3 | Registry and admission grouping, nonwaiting cleanup lock and heartbeat precedence. |
| Workstream 1, items 6 and 7 | First-write deadlines, early oversize refusal, connection response headers, phase timestamps, key validation, identifier and reachable test continuation. |
| Workstream 2, items 4, 8 and 9 | Report contact time, durable and diagnostic evidence, frame refusal order, acknowledgement barrier, wire mark and keyed approval proof. |
| Workstream 3, items 5 and 10 | Feed recovery, independent cleanup, address provenance, combined fixture and startup failure gate. |
| Traceability, guardrails, execution sizing and attack log | Review links, module handoff, nine build items and new dependencies. |
| Post-build checks and downstream seams | Updated P1 to P5, P7, and P10 to P12 observations. |

## Post-build evidence proposed by the reviewers

| Proposal | Plan evidence |
|---|---|
| ARCH-P1 and the critic's transport closure | P3 observes reader exit, terminal status and bounded blocked writes through the proxy. |
| ARCH-P2 and the critic's partial startup cleanup | P7 rehearses the ledger refusal, the old receipt and recovery after a partial clear. |
| ARCH-P3 and logic-r1 P1 | P2 and the handshake tests cover both sides of the Welcome write fence. |
| ARCH-P4 and TH2 | P11 checks the revised optional-address policy and the existing ingress boundary. |
| logic-r1 P2 and the critic's heartbeat timing | P1 checks a connection surviving the proxy threshold. P3 and P12 cover silence and saturation. |
| logic-r1 P3 and the critic's sprint replay | P4 observes both deterministic cuts and the durable stores. |
| logic-r1 P4 | Item 10 and P7 check readiness and lock release after a failed clear. |
| TH1, TH3, TH4, TH5 and TH6 | P10 checks proof, credentials and the installer. P11 checks authorization. P2 and P3 check removal and cleanup. |

## Decision for the owner

Threat-r1 05 concerns a copied PostgreSQL volume. Workspace text, inventories, addresses and saved approval replies live in that volume. The foundation sets a one-day receipt retention rule and protects the signing key through OpenBao. It leaves ordinary machine metadata under the deployment's existing volume controls. The fold keeps the existing policy pending the owner's choice.

- Require encrypted host storage for the PostgreSQL volume. The Stack team must document setup, startup behavior and recovery on supported hosts. A copied volume gives less information when the storage key is held separately.
- Accept the existing volume policy for wave 1 and document its exposure. This keeps installation and recovery as they are. An operator who can read a copy of the plaintext volume can read machine metadata and saved replies still within retention.

## Review verdicts and grades

| Reviewer | Verdict | Grade | Findings reported |
|---|---|---:|---|
| Architect | REVISE | 6/10 | Zero blockers, five majors and four minors. |
| Critic | REVISE | 5/10 | One blocker, six majors and three minors. |
| Logic | REVISE | 7/10 | Zero blockers, four majors and one minor. |
| Threat | REVISE | 7/10 | Zero blockers, two majors and three minors. |

The top finding is the R10 blocker, critic-r1 01. The original item 7 test could not exercise the route it was required to prove at its merge gate.
