# Sprint 4 client folders review round 1

Four OpenAI reviewers returned Revise. They reported 36 findings in 20 groups. The reviewed input is [the frozen plan](plan-v1.md). The identifiers below link each change to its report.

## Finding outcomes

The statement column preserves each report’s label. C9 and the rule labels R6, R7, R9 and R10 refer to the foundation design. Sprint checks use the same R numbers in the plan. For a group reported under different labels, the points go to its primary row.

| Finding | Severity | Design statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r1 01 | Blocker | C9 | fixed | The rollout stages both revision switches and paired fixtures in one merge unit with separate test gates. |
| architect-r1 02 | Major | R10 | fixed | The paired receipt observer reads the new fields and the age writer preserves them. |
| architect-r1 03 | Major | R7 | fixed | Store inspection validates the next counter before sealing or dial and keeps exhaustion at the legal edge. |
| architect-r1 04 | Major | R9 | fixed | Direct publish and pending flush use the final field check under the registration lock. |
| architect-r1 05 | Major | R9 | fixed | Common command admission and result handoff check exact identifiers. |
| architect-r1 06 | Major | R10 | fixed | The paired handoff uses the Server Link fixture signature and retains the after-commit observer. |
| architect-r1 07 | Major | R9 | fixed | All Git calls share trusted execution rules and parent status suppresses automatic child reads. |
| architect-r1 08 | Major | plan | fixed | Items 4 and 5 have an exact private boundary, sequential gates and independently executable tests. |
| architect-r1 09 | Major | R9 | fixed | Repeats architect-r1 04. The same final field check omits the normalized workspace when a zero byte had hidden a match. |
| architect-r1 10 | Minor | R10 | fixed | Repeats logic-r1 06. The callback identifies the report and the closed store supplies the separate final process watermark. |
| architect-r1 11 | Minor | plan | fixed | The plan defines Git capabilities and records both native versions under the installed user. |
| critic-r1 01 | Blocker | plan | fixed | The frozen busy-slot test permits the first standing write and keeps its zero-write retry controls. |
| critic-r1 02 | Major | R9 | fixed | Repeats architect-r1 05. The signed command fixture covers standing, ordinary and replayed identifiers. |
| critic-r1 03 | Major | R10 | fixed | Repeats architect-r1 06. The paired caller selects the server rollback point by machine and event number. |
| critic-r1 04 | Major | plan | fixed | Repeats threat-r1 07. The inspection deadline frees the slot and preserves fixed refusal and failed-append outcomes. |
| critic-r1 05 | Major | plan | fixed | The private capture lists inert mode and ignore settings and tests the fallback refusal. |
| critic-r1 06 | Minor | plan | fixed | The root walk selects the exact local remote-tracking target when available and tests divergent roots. |
| critic-r1 07 | Minor | plan | fixed | Repeats architect-r1 08. The metadata gate precedes the full inspection gate with a concrete snapshot interface. |
| critic-r1 08 | Minor | plan | fixed | Native gates require explicit named passes, zero skips and the full client suite on both hosts. |
| logic-r1 01 | Major | plan | fixed | Status verifies or refuses index-suppressed entries and distinguishes intentionally absent sparse files. |
| logic-r1 02 | Major | R7 | fixed | Repeats architect-r1 03. The upper-edge test drains the last legal event and separately exercises retry on a valid local counter. |
| logic-r1 03 | Major | R7 | fixed | Terminal recovery fences storage before cleanup and the held-scan case preserves the unfinished receipt. |
| logic-r1 04 | Major | R9 | fixed | Repeats architect-r1 04. The new-token fixture inspects the replacement after a full-outbox flush. |
| logic-r1 05 | Major | R10 | fixed | Repeats architect-r1 06. The paired test checks rollback before retry and settlement. |
| logic-r1 06 | Major | R7 | fixed | The fixture distinguishes the report, a later durable event and the unrelated append after restart. |
| logic-r1 07 | Major | plan | fixed | The executable gate observes startup and existing reports before the integrated branch and origin check. |
| logic-r1 08 | Minor | R9 | fixed | Repeats architect-r1 05. The common dispatcher checks every action before receipts or mutation. |
| logic-r1 09 | Minor | plan | fixed another way | Repeats architect-r1 08. Item 4 absorbs capture and item 5 takes status. Item 9 is reserved for the requested standalone fixture. |
| threat-r1 01 | Major | R6 | fixed | Ordinary receipt age uses a lower server-time bound and defers pruning without a verified sample. |
| threat-r1 02 | Major | R9 | fixed | App rereads the bounded credential source at output boundaries and keeps previously observed values. |
| threat-r1 03 | Major | R9 | fixed | Repeats architect-r1 07. Parent status suppresses implicit child inspection and children get their own private views. |
| threat-r1 04 | Major | R9 | fixed | Repeats architect-r1 04. The shared admission check covers new normalization and current secrets at pending flush. |
| threat-r1 05 | Major | R9 | fixed | Repeats architect-r1 05. The shared reply gate also covers results queued before secret registration. |
| threat-r1 06 | Major | R10 | fixed | Repeats architect-r1 06. The paired test injects failure before commit and observes no new acknowledgement or durable effect. |
| threat-r1 07 | Major | R6 | fixed another way | A bounded wrapper detaches stalled read-only calls. This avoids a new process role or public invocation. |
| threat-r1 08 | Major | plan | fixed another way | Repeats architect-r1 08. Items 4 and 5 divide capture and status. The fixture takes the ninth item as required by the fold. |

## Points and stuck statements

Each duplicate group counts once at its highest severity. A blocker earns nine points, a major earns three and a minor earns one.

| Statement | Groups | Points | Share |
|---|---:|---:|---:|
| C9 | 1 | 9 | 13.6% |
| R6 | 2 | 6 | 9.1% |
| R7 | 3 | 9 | 13.6% |
| R9 | 4 | 12 | 18.2% |
| R10 | 2 | 6 | 9.1% |
| plan | 8 | 24 | 36.4% |
| **Round total** | **20** | **66** | **100%** |

The **plan** is stuck with 24 of 66 points, or 36.4%. R9 reaches 12 points and accounts for 18.2% of the round.

## Mechanisms with multiple major findings

- **Outgoing filter.** Separate producer checks were the wrong boundary for new data. The fold shares one append decision between publish and flush and uses the common admission and result paths for command identifiers.
- **Git inspection.** The status isolation remains necessary. The earlier boundary left discovery, original child metadata and status capture open. The fold moves capture into discovery, orders status after it and suppresses parent recursion.
- **Recovery.** Checking only after Welcome missed an invalid local counter and late writes by existing workers. Store validation now precedes dialing and terminal recovery fences writes.
- **Receipt and paired proof.** One durable watermark remains sufficient. The old fixture schema and interruption observation were insufficient. The fold keeps the first watermark and makes the two observation points explicit.

## Patched plan headings

| Plan heading | Patch |
|---|---|
| Overview, Key decisions and Workstreams | Keep the owner summary aligned with the fixture interruption, filter and checkout scope. |
| Rollout order | Stage the revision switch as one paired merge. |
| Item 1. Send the revision and the process boundary | Keep independent acceptance on the coordinated branch. |
| Item 2. Save the first result watermark and accept old receipts | Use authenticated pruning time and identify the legacy producer. |
| Item 3. Stop locally on an unusable replay | Check the counter before dial and fence cleanup writes. |
| Item 4. Resolve the checkout and capture local metadata | Define the private handoff, trusted execution and exact reference choice. |
| Item 5. Read isolated status and return full folder facts | Check index flags and children in the private view. |
| Item 6. Append a folder report before answering its command | Bound the read and preserve late completion and restart behavior. |
| Item 7. Filter outgoing reports, including their replay | Guard final report bytes and common command identifiers. |
| Item 8. Filter logs from process startup | Refresh readable credentials and split independent from integrated evidence. |
| Item 9. Provide the in-process interruption fixture | Publish the fixture signatures and distinguish the two numbers. |
| Sprint 3 frozen test changes | Amend only the assertions and observers the fold needs. |
| Post-build checks | Name the combined scenarios, the server callback, native Git and supervisor evidence. |
| Traceability, Execution sizing and Attack log | Map the ninth item, preserve scope and link this register. |

## Top finding and reviewer grades

The top finding was **architect-r1 01, a blocker**. Either revision switch would break the inherited real-client and real-server check if merged alone. The plan gives the two switches and their shared fixtures one merge boundary.

| Reviewer | Verdict | Grade | Reported severities |
|---|---|---:|---|
| [Architect](round-1-architect.md) | Revise | 5 out of 10 | One blocker, eight majors and two minors. |
| [Critic](round-1-critic.md) | Revise | 6 out of 10 | One blocker, four majors and three minors. |
| [Logic](round-1-logic.md) | Revise | 5 out of 10 | Seven majors and two minors. |
| [Threat](round-1-threat.md) | Revise | 5 out of 10 | Eight majors. |
