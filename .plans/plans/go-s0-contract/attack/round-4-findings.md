# Review round 4 — register of findings

This register folds the two reports of round 4: `round-4-architect.md` and `round-4-logic.md`.
The reviewed revision is the frozen copy `attack/plan-v4.md`, with the drafts as they stood after round 3.

This is the last fold of this plan before sprint 0 builds it, and no reviewer reads it before the build.
Every fix is therefore marked UNREVIEWED FIX in the plan, and each has a post-build check.
The link is not frozen by sprint 0: the reviewers of plans `go-s3-server-machines` and `go-s3-client-link` read it again before it freezes.

## Not reviewed before the build

Each place below changed in this fold, and the post-build check named with it tests the change.

| Place | What changed | Post-build check |
|---|---|---|
| `plan.md`, "Key decisions", removal; item 6, "Removal"; `draft/link.proto`, the part "Removal" of the `Open` comment | The server orders a removal's commit against the admission of a stream. A `Welcome` sent before the commit may still arrive, and it has no authority after the commit. | 16, and the corrected check 10 |
| `plan.md`, item 4, "Rules of events"; item 6, "The value rules"; public names, "The value rules of the link"; `draft/events.proto`, file comment; `draft/link.proto`, file comment | The table of events comes before the value rules. An undecodable message is `invalid_argument`, a repeat or a gap is decided by its number alone, and only the next number reaches its report. | 17 |
| `plan.md`, public names, `Lgtm-Join-Refusal` and `Lgtm-Link-Refusal`; items 5 and 6; agreements; `draft/link.proto`, the `LinkService` and `Join` comments | Both reasons are in the metadata of the error, which reaches the client after the first message of a stream too. | 18 |
| `plan.md`, item 4, "Rules of commands"; agreements; `draft/commands.proto`, file comment | `SetState` stays pending until it has a result, and a machine always answers it `COMMAND_OUTCOME_DONE`. | 19 |
| `plan.md`, "Key decisions", commands; item 4, "Rules of commands"; risks; agreements; `draft/commands.proto`, file comment and `COMMAND_OUTCOME_EXPIRED` | A command starts only before its expiry. A machine may decline one it cannot start in time, a recorded result comes first, and the promise holds while the server's clock does not jump. | 20 |
| `plan.md`, item 6, "The handshake" and "The codes"; `draft/link.proto`, the `Open` comment | One order of decisions after `Challenge`, which may come before `Proof`. | 21 |
| `plan.md`, item 4, "What the website shows"; agreements; `draft/link.proto`, the projection | Joining and online need a current stream, so the four cases are disjoint. | 22 |
| `plan.md`, item 6, "The current stream"; agreements; `draft/link.proto`, "The current stream" and `Welcome` | A stream becomes current when the server admits it to send `Welcome`, and the older one stops being current then. `Welcome.last_event_number` is read after that moment. | 23 |
| `plan.md`, public names, the codes; item 6, "The codes"; `draft/link.proto`, the table of codes; `draft/events.proto`, the gap | A final code depends on the request and on the server's state when it decides. A gap ends only its stream. | 24 |
| `plan.md`, item 5; `draft/link.proto`, the `Join` comment | The loser of two first uses is decided again by the full order, against the state at that moment. | 25 |

## 1. Every finding

The severities are B for a blocker, M for a major and m for a minor.
The group column names the group of duplicates, and the "Repeats" column names the first finding of that group.

| Finding | Severity | Statement | Group | Outcome | Reason | Repeats |
|---|---|---|---|---|---|---|
| architect-r4 01 | M | D7 | G1 | fixed | A server cannot retract a `Welcome` in transit. The rule now orders the commit against admission, states both observations, and check 10 expects the refusal in the revocation race only when the revocation removed the machine. | — |
| architect-r4 02 | M | C11 | G2 | fixed | The three steps as prescribed: undecodable ends with `invalid_argument`, the number decides a repeat or a gap, and only the next number reaches its report. The scratch decoder probe agrees that invalid UTF-8 fails to decode. | — |
| architect-r4 03 | M | C10 | G3 | fixed | Both reasons move into the error's metadata. The pinned library carries it in headers for a unary error, in the end-of-stream message for a stream, and in trailers for gRPC (scouting run 35). | — |
| architect-r4 04 | M | R6 | G4 | fixed another way | `SetState` stays pending until it has a result, and a machine always answers it `COMMAND_OUTCOME_DONE`. This makes the closed-but-unapplied state impossible. UNREVIEWED MECHANISM. | — |
| architect-r4 05 | M | R6 | G5 | fixed | The first of the two offered choices: a machine may decline an unstarted command it cannot establish to start in time, `COMMAND_OUTCOME_EXPIRED` now means that, and a recorded result comes first. | — |
| architect-r4 06 | M | C10 | G6 | fixed | One order after `Challenge`: lookup, unknown machine, revision, `Proof` deadline, value rules, signature. Early refusals may come before `Proof`. Finality is defined against the state at the decision. | — |
| architect-r4 07 | M | C7 | G7 | fixed | Joining and online need a current stream, offline is listed first, and the four cases are disjoint. | — |
| logic-r4 01 | M | C11 | G8 | fixed | One cutover: the moment the server admits the new stream to send `Welcome`. The older stream stops being current then, and `Welcome.last_event_number` is read after it. | — |
| logic-r4 02 | M | D7 | G1 | fixed | As architect-r4 01. | architect-r4 01 |
| logic-r4 03 | M | C10 | G3 | fixed | As architect-r4 03. | architect-r4 03 |
| logic-r4 04 | M | C7 | G4 | fixed another way | As architect-r4 04. The late first send and the send after 24 hours are both in check 19. | architect-r4 04 |
| logic-r4 05 | M | R6 | G5 | fixed | The clock condition is restored: the promise holds while the server's clock runs without a jump after `Welcome.server_time`. The boundary is exclusive: a command starts only before its expiry. | architect-r4 05 |
| logic-r4 06 | M | C11 | G2 | fixed | As architect-r4 02, with the invalid repeat and the invalid gap in check 17. | architect-r4 02 |
| logic-r4 07 | M | C11 | G9 | fixed | A final code depends on the request and the server's state at the decision. A gap ends only its stream, and a later stream that sends the missing number goes on. The same holds for the order of `Join`. | — |
| logic-r4 08 | M | C5 | G10 | fixed | The loser of two first uses is decided again by the full order against the state at that moment, so an expiry or a revocation in between gives its refusal. | — |

The counts by outcome are 13 fixed, 2 fixed another way, and none rejected, post-build check, for the owner or not folded.

Two mechanisms each held more than one major.
The rules of events held three (G2, G8 and G9). The mechanism is right, but it had three rules that each decided an event without saying which came first; one table now decides every event of the current stream.
The rules of expiry held two (G4 and G5). The mechanism promised more than the wire can carry. It now promises only what an end can keep, and `SetState` leaves the expiry rules altogether.

## 2. Points per statement

A group counts once, at its highest severity: a minor is 1 point, a major 3 and a blocker 9.

| Statement | Groups | Points |
|---|---|---|
| C11 | G2 (3), G8 (3), G9 (3) | 9 |
| C10 | G3 (3), G6 (3) | 6 |
| R6 | G4 (3), G5 (3) | 6 |
| D7 | G1 (3) | 3 |
| C7 | G7 (3) | 3 |
| C5 | G10 (3) | 3 |

The round's total is 30 points, in 10 groups.

## 3. Stuck statements

None.
A statement is stuck at 12 points and 30% of the round, which is 9 points here.
C11 has 9 points, which is 30% of the round, but it is below 12 points.

## 4. Patches by heading

No review round follows this fold. The post-build checks 10 and 16 to 25 test these places, and the reviewers of the two sprint-3 plans read the link again before it freezes.

1. `plan.md`, "Key decisions": removal, and the clock of commands.
2. `plan.md`, "Public names that other teams build against": the two refusal keys, the codes and the value rules.
3. `plan.md`, item 4: the rules of commands, the rules of events and the projection.
4. `plan.md`, item 5: the order of refusal at the decision, the key, and the loser of two first uses.
5. `plan.md`, item 6: the handshake, the value rules, the codes, the current stream and removal.
6. `plan.md`, "Risks": the clock and the stop.
7. `plan.md`, "Agreements with other teams": the rows of commands, streams, codes and the projection.
8. `plan.md`, "Post-build checks": check 10, and checks 16 to 25.
9. `plan.md`, "Sources", "Traceability" and "Attack log".
10. `draft/link.proto`, `draft/events.proto` and `draft/commands.proto`, comments only.
11. `decisions.md`, rows 65 to 68 amended, and rows 69 and 70.
12. `scouting.md`, runs 33 to 36.

## 5. The top finding

The reason of a refused stream was named a header, but a refusal after `Welcome` cannot travel in the leading headers, so a client could never read `unknown_machine` or `event_gap`, and check C10 would fail (architect-r4 03, logic-r4 03).
It was a major, both reviewers found it on their own, and the architect confirmed it with the pinned library.
Both reasons now travel in the metadata of the error.

## 6. Each reviewer's verdict and grade

| Reviewer | Verdict | Grade | Counts |
|---|---|---|---|
| architect | REVISE | 5 | 0 blockers, 7 majors, 0 minors |
| logic | REVISE | 5 | 0 blockers, 8 majors, 0 minors |
