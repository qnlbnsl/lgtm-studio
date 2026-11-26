# Register of review round 1 of the plan `go-s3-server-machines`

The round had five reports: architect, critic, logic, logic2 and threat. They were folded on 2026-11-26 into `plan.md`, `decisions.md` rows 53 to 63, and `scouting.md`. The plan's status stays `attack`.

## Every finding and its outcome

A group of duplicates has one letter. The first finding of a group carries its points, and the others name the finding they repeat.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r1 01 | blocker | R6 | A | fixed | `AcceptResult` updates only the row whose identifier, machine, sent stream and attempt all match (decision 58). |
| architect-r1 02 | blocker | R10 | B | fixed | Each item's fixture applies only the parts that exist, cross-item assertions moved to the later item, item 8 needs items 2 and 9, item 12 needs item 6, and cyclic tests sit in external test packages. |
| architect-r1 03 | major | C10 | E | fixed | The stream registers, then `State.Confirm` checks the machine and the generation before `Welcome`, as logic-r1 4 prescribed. A per-machine gate in memory was not added, because no lock may span the send (decision 56). |
| architect-r1 04 | major | C8 | D | fixed | A watchdog cancels the stream's context, a cancelled send ends at once, `Presence` ignores a row older than 30 seconds, and `Run` retries a failed `Disconnect` (decision 55). |
| architect-r1 05 | major | R6 | F | fixed | A claim takes at most 64 commands with one feed row, and start-up cleanup and the sweep work in bounded writes (decision 57). |
| architect-r1 06 | major | C11 | G | fixed | Facts and harnesses are stored as their binary encoding, memory in gigabytes is capped, and `facts_at` holds `happened_at` (decision 59). |
| architect-r1 07 | major | C11 | H | post-build check | The order of log lines across a replaced stream is not the commit order. The plan says so, and check P4 reads the mark and the effect. The script `deploy/test/checks.sh` is outside this plan, so the result file asks its owner to change C11. |
| architect-r1 08 | major | R4 | I | fixed | Approval and rename build their reply inside the keyed transaction, and the store's keyed sentinels have codes (decision 61). |
| architect-r1 09 | major | C6 | J | fixed | `TokenOptions` holds only `HostsServer`, and the token always carries the configured address (decision 53). The installer plan's leftover certificate names are a request. |
| architect-r1 10 | major | C6 | K | for the owner | The page's join command and its restart action need a change to the visible page, which needs the owner's approval. Criterion 16 now waits for it. |
| architect-r1 11 | major | R5 | L | fixed | `transport.Send` ends a blocked write when its context ends, through `context.AfterFunc`. |
| architect-r1 12 | major | R6 | M | fixed | The overview now promises at most once, with expiry and interruption named, and a claim marks a command past its grace `expired` instead of sending it. An `expired` result of a `SetState` is stored as `failed`. The client's clock is settled by note 1 of the orchestrator. |
| critic-r1 01 | blocker | C5 | N | fixed | Every error that a caller tells apart is a `store.NewDomainError` sentinel returned unwrapped (decision 54). Confirmed by reading `src/server/internal/store/pool.go`, `sanitizeError`. |
| critic-r1 02 | blocker | R10 | B | fixed | Repeats architect-r1 02. |
| critic-r1 03 | major | C6 | K | for the owner | Repeats architect-r1 10. |
| critic-r1 04 | major | C8 | D | fixed | Repeats architect-r1 04. |
| critic-r1 05 | major | C10 | E | fixed | Repeats architect-r1 03. `TestStaleAdmissionCannotRegister` covers the earlier generation that registers late. |
| critic-r1 06 | major | R6 | F | fixed | Repeats architect-r1 05. |
| critic-r1 07 | major | R6 | A | fixed | Repeats architect-r1 01. |
| critic-r1 08 | major | R4 | I | fixed | Repeats architect-r1 08. |
| critic-r1 09 | major | R6 | M | rejected | It re-opens the client's clock, which note 1 of the orchestrator settles: the machine trusts its own clock, and `.plans/plans/go-s1-client-local/plan.md` lines 47 and 105 record it. The server's half, a claim after the window, is fixed under threat-r1 05. |
| critic-r1 10 | major | C11 | G | fixed | Repeats architect-r1 06. |
| critic-r1 11 | major | C9 | O | fixed | The rollout now says that recovery after the first sprint 3 start goes forward, and check P7 rehearses it (decision 62). |
| critic-r1 12 | minor | C5 | P | fixed | `Create` reads the clock once for both columns. |
| logic-r1 1 | blocker | R6 | A | fixed | Repeats architect-r1 01. |
| logic-r1 2 | major | R6 | F | fixed | Repeats architect-r1 05. The eligibility test is `sent_stream_id IS DISTINCT FROM` the stream. |
| logic-r1 3 | major | C8 | D | fixed | Repeats architect-r1 04. |
| logic-r1 4 | major | C10 | E | fixed | Repeats architect-r1 03, and its fix is the one applied. |
| logic-r1 5 | major | D16 | Q | fixed | Commands leave in the order of `seq`, filled under the machine's lock, and a scan needs an online machine (decisions 57 and 63). |
| logic-r1 6 | major | R6 | M | rejected | Same reason as critic-r1 09. The interrupted outcome is now named in the overview. |
| logic-r1 7 | major | C11 | G | fixed | Repeats architect-r1 06. The mark stays a signed 64-bit number, the comparison is made as `uint64`, and a machine that reached 2^63 events is outside wave 1. |
| logic-r1 8 | major | R4 | I | fixed | Repeats architect-r1 08. |
| logic-r1 9 | major | D16 | K | for the owner | Repeats architect-r1 10. |
| logic-r1 10 | major | C11 | R | fixed | The event and claim tests hold and reject the real commit through a test-only deferred constraint trigger, because the store's commit hooks are private to the store. |
| logic-r1 11 | minor | C5 | P | fixed | Repeats critic-r1 12. |
| logic-r1 12 | minor | R4 | S | fixed | The sweep works one machine to a write under the machine's lock, with a feed row per machine. |
| logic2-r1 1 | major | C10 | T | fixed | The nonce and the stream identifier come from `crypto/rand`, with two tests and a mutation. |
| logic2-r1 2 | major | C8 | D | fixed | Repeats architect-r1 04. Its retry of a failed `Disconnect` is the one applied. |
| logic2-r1 3 | minor | D7 | U | fixed | `Welcome` takes its standing from the admission. |
| logic2-r1 4 | minor | R4 | V | fixed | `TestWritesOfOneMachineFormOneLine` is a table over every write of `State`. |
| logic2-r1 5 | minor | D7 | W | fixed | `TestBecomeCurrent` checks generations 2 and 3, and `TestStaleAdmissionCannotRegister` is added. |
| logic2-r1 6 | minor | C7 | X | fixed | A load of 0 is tested in the heartbeat and in the projection. |
| logic2-r1 7 | minor | C11 | G | fixed | Repeats architect-r1 06. |
| threat-r1 01 | blocker | C10 | Y | for the owner | Whether wave 1 may trust the cleartext loopback and the stack's private network is a choice about risk that the design leaves open, and point 2 of the contract plan still asks it. |
| threat-r1 02 | blocker | R6 | A | fixed | Repeats architect-r1 01. |
| threat-r1 03 | major | C8 | D | fixed | Repeats architect-r1 04. |
| threat-r1 04 | major | C10 | E | fixed | Repeats architect-r1 03. A removal skipped during a lag ends its stream at the next claim. |
| threat-r1 05 | major | R6 | M | fixed | Repeats architect-r1 12. Its cutoff inside the claim is the one applied. Its client half is settled by note 1. |
| threat-r1 06 | major | C11 | G | fixed | Repeats architect-r1 06. |
| threat-r1 07 | major | C11 | H | post-build check | Repeats architect-r1 07. |
| threat-r1 08 | major | R9 | Z | for the owner | How the installer and the page take the token is point 4 of the contract plan. The server's part is fixed: the token reply carries `Cache-Control: no-store`. |
| threat-r1 09 | minor | C7 | Q | fixed | Repeats logic-r1 5. |
| threat-r1 10 | minor | C5 | AA | fixed | Only a first use checks the host name, and a zero byte is refused (decision 60). |

## Points by statement

A minor finding is 1 point, a major is 3, and a blocker is 9. Each group counts once, at its highest severity.

| Statement | Groups | Points |
|---|---|---|
| R6 | A (9), F (3), M (3) | 15 |
| C10 | E (3), T (3), Y (9) | 15 |
| C5 | N (9), P (1), AA (1) | 11 |
| R10 | B (9) | 9 |
| C11 | G (3), H (3), R (3) | 9 |
| C6 | J (3), K (3) | 6 |
| R4 | I (3), S (1), V (1) | 5 |
| C8 | D (3) | 3 |
| R5 | L (3) | 3 |
| C9 | O (3) | 3 |
| D16 | Q (3) | 3 |
| R9 | Z (3) | 3 |
| D7 | U (1), W (1) | 2 |
| C7 | X (1) | 1 |
| **Total** | 27 groups: 4 blockers, 15 majors, 7 minors | **88** |

## Stuck statements

A statement is stuck with at least 12 points and at least 30% of the round's points, which is 26.4 points. No statement is stuck. R6 and C10 have 15 points each, which is 17% of the round.

Two mechanisms hold several blockers and majors.
The stored commands hold groups A, F, M and Q.
Their shape is right: a stored command, a numbered attempt and a result checked under the machine's lock.
The faults were a missing condition in one update, a missing bound on one write, a missing cutoff, and an order taken from the clock. Each fix narrows that shape and adds no part.
The handshake and admission hold groups E, T and Y.
The admission itself is right. The gap was between the commit and the first send, and `State.Confirm` closes it with a read under the same lock. Group Y is a question about the transport, not about this mechanism.

## Places changed in the plan

The next round attacks only these places.

1. Overview: the paragraph on commands.
2. Key decisions: "Wave 1 tokens are for the loopback address only".
3. Workstreams: "The Machines page works against the Go server".
4. Rollout order: the steps and the paragraph after them.
5. Implementation details: "Fixtures shared by every item", "The one rule of locking", the new "Errors that cross the store", "Public names" with its table and signatures, "Log lines", and "Codes and sentences".
6. WS-1: items 1, 3 and 4.
7. WS-2: items 2, 5, 8, 9 and 10, and the table of risks.
8. WS-3: items 6 and 7.
9. WS-4: criterion 16, items 11, 12, 13 and 14, and the table of risks.
10. Traceability: the source universe and two rows.
11. Execution sizing: the rows of items 8 and 12.
12. Attack log: row 1.
13. The new fold "Post-build checks", rows P1 to P10.

## Top finding of the round

The top finding is that a command result was not bound to its own machine and the stream that received it (group A). It was a blocker, and four of the five reviewers found it alone.

## Verdicts and grades

| Reviewer | Verdict | Grade |
|---|---|---|
| architect | REVISE | 4/10 |
| critic | REVISE | 4/10 |
| logic | REVISE | 5/10 |
| logic2 | REVISE | 7/10 |
| threat | REVISE | 5/10 |
