# Register of review round 1 — go-s0-contract

This file records how the planner folded the five reports of review round 1 into the plan, on 2026-11-26.
The reports are `round-1-architect.md`, `round-1-critic.md`, `round-1-logic.md`, `round-1-logic2.md` and `round-1-threat.md`.
The plan they read is frozen in `plan-v1.md`.

A statement is cited as in the plan: D for a decision of the design, R for a rule, C for a check, and `plan` for the plan itself.
The design is `docs/specs/2026-11-25-go-rewrite-foundation-design.md`.

## Every finding and its outcome

The 59 findings fall into 22 groups.
The first finding named in a group is its lead, and every other finding of the group repeats the lead.
A group that several reviewers reached on their own is strong evidence, and the column "Repeats" shows it.

| Finding | Severity | Statement | Group | Repeats | Outcome | Reason |
|---|---|---|---|---|---|---|
| architect-r1 01 | blocker | C11 | 1 | — | fixed | The server now applies only the next number, commits the effect with the mark, and acknowledges after the commit. Items 4 and 6, rules of events. |
| critic-r1 5 | major | C11 | 1 | architect-r1 01 | fixed | `EventAck.through` and `Welcome.last_event_number` are now one committed mark, and an unknown report is consumed. |
| logic-r1 01 | blocker | C11 | 1 | architect-r1 01 | fixed | One ordered sender per stream, the next number only, a copy acknowledged again, and a gap that ends the stream. |
| threat-r1 3 | blocker | C11 | 1 | architect-r1 01 | fixed another way | The ordered prefix is as asked. A server mark behind the machine's is handled by the new field `Hello.acked_event_number`, marked UNREVIEWED MECHANISM. |
| logic2-r1 9 | minor | C11 | 1 | architect-r1 01 | fixed | The acknowledgement follows the commit, and an event of an unknown kind is consumed. |
| logic2-r1 2 | major | C11 | 2 | — | fixed | A machine never reuses a number, raises its next number above `Welcome.last_event_number`, and sends its facts and harnesses after every `Welcome`. |
| architect-r1 02 | blocker | D7 | 3 | — | fixed | The commit of a removal deletes the key and the server then ends the stream. The notice is a courtesy, and the client stops on a verified refusal. |
| logic-r1 04 | major | D7 | 3 | architect-r1 02 | fixed another way | The refusal is `unauthenticated` after a signed `Challenge`, in place of a distinct code backed by a record of removed ids. The place is marked UNREVIEWED MECHANISM. The revision rules are as asked. |
| threat-r1 1 | blocker | C10 | 3 | architect-r1 02 | fixed | Revocation is authoritative at the commit, a stream becomes current only after its proof and a fresh read, and writes are fenced to the current stream. |
| critic-r1 7 | major | C8 | 3 | architect-r1 02 | fixed | The terminal refusal, a kept key on a transport failure, a terminal message for another link revision, the client's 30-second silence limit and the retry within 15 seconds. |
| architect-r1 09 | major | C9 | 4 | — | fixed | Install after proof, fencing by the current stream, the client's two 10-second handshake limits, and the code table. |
| logic-r1 06 | major | C8 | 4 | architect-r1 09 | fixed | Presence belongs to the current stream, the server closes a stream that is silent for 30 seconds, and `HeartbeatAck` answers only the current stream. |
| logic2-r1 4 | major | C9 | 4 | architect-r1 09 | fixed | The end of a replaced stream changes nothing, and only `Welcome` marks a machine online. |
| logic-r1 09 | major | R8 | 5 | — | fixed | The legal message of each phase, both peers' limits, the value rules, and the code table with its stop and retry rows. |
| logic2-r1 6 | major | C10 | 5 | logic-r1 09 | fixed another way | The code table is written, but `unauthenticated` after a verified `Challenge` stops the client, and `aborted` waits for the normal retry. The place is marked UNREVIEWED MECHANISM. |
| critic-r1 10 | minor | R8 | 5 | logic-r1 09 | fixed | Exact sizes, present timestamps, zero rules, refusal of an empty handshake message, and limits measured apart. |
| logic2-r1 10 | minor | R6 | 5 | logic-r1 09 | fixed | An unspecified or unknown approval counts as waiting, an unspecified or unknown outcome is stored as failed, and nonces are exactly 32 bytes. |
| architect-r1 12 | minor | R2 | 5 | logic-r1 09 | fixed | The value rules cover zero and missing values. The description helper now prints reserved ranges and names, so a reserved range cannot slip in. |
| architect-r1 06 | major | C5 | 6 | — | fixed | One transaction uses up the token, a replay is defined exactly, and the order of refusals is fixed. |
| critic-r1 3 | major | C5 | 6 | architect-r1 06 | fixed | First use and replay are distinct cases, the compared fields are named, and the client keeps its key before the first `Join`. Post-build check 1 tests both cases. |
| logic-r1 05 | major | C5 | 6 | architect-r1 06 | fixed | Revoking a used token removes a waiting machine and leaves an approved one, and a failure before the commit returns `unavailable`. |
| threat-r1 6 | major | C5 | 6 | architect-r1 06 | fixed | A different key, a revoked token and a removed machine are refused, and the dialog's revocation is defined. |
| logic2-r1 8 | minor | C5 | 6 | architect-r1 06 | fixed | A replay needs the machine that the token created to still exist. |
| architect-r1 07 | major | R6 | 7 | — | fixed | `COMMAND_OUTCOME_INTERRUPTED = 5`, every other outcome final, future attempts ignored, one expiry, the server's clock, and a recorded result before expiry. |
| critic-r1 6 | major | R6 | 7 | architect-r1 07 | fixed | Expiry gates only the first start, the newest attempt carries the result, and a scan is done only once its report is on disk. |
| logic-r1 02 | major | R6 | 7 | architect-r1 07 | fixed | The machine counts the server's time from `Welcome.server_time` and its monotonic time since `Proof`, as asked. |
| logic-r1 03 | major | R6 | 7 | architect-r1 07 | fixed | Attempts are stored before they are sent, only the current attempt on the current stream counts, and an interruption has its own outcome. |
| logic2-r1 5 | major | R6 | 7 | architect-r1 07 | fixed | The outcome table, the server's own expiry, and a receipt kept 24 hours after expiry, which the client-local default already gives. |
| threat-r1 9 | major | R6 | 7 | architect-r1 07 | fixed | One expiry for every attempt, results accepted only from the current stream for the outstanding attempt, and rules for absent and zero values. |
| logic2-r1 3 | major | R6 | 8 | — | fixed | The `Restart` receipt is on disk before the result is sent, and the program ends after both. |
| threat-r1 8 | major | R6 | 8 | logic2-r1 3 | fixed | The outcome of a `Restart` means that the restart was accepted, the interrupted mapping is frozen, and a scan relates to its report on disk. |
| architect-r1 03 | major | D16 | 9 | — | for the owner | How the Machines page follows the new meaning of `ReconnectClient` is a visible change of the page, which needs the owner. Point 1 in the plan. |
| critic-r1 2 | major | D16 | 9 | architect-r1 03 | for the owner | As above. |
| logic-r1 07 | major | D16 | 9 | architect-r1 03 | for the owner | As above. The plan's false claim of a comment-only change is corrected. |
| logic2-r1 1 | major | D16 | 9 | architect-r1 03 | for the owner | As above. The revision stays at 1 in sprint 0, and point 1 says when it becomes 2. |
| threat-r1 12 | major | D16 | 9 | architect-r1 03 | for the owner | As above for restart. Its enrollment half is fixed by group 10. |
| architect-r1 04 | major | C6 | 10 | — | fixed | The token's address is the only address the client dials, a different one is refused, and the page's command is a handoff to the orchestrator. |
| critic-r1 1 | major | C6 | 10 | architect-r1 04 | fixed | As above, with the normalization rule and post-build check 7, which pastes the page's command. |
| logic-r1 08 | major | C6 | 10 | architect-r1 04 | fixed | As above. `CreateClientTokenResponse.token` carries the encoded token, and the waiting machine keeps its `token_id`. |
| logic2-r1 7 | major | D7 | 10 | architect-r1 04 | fixed | The comment of `CreateClientTokenResponse.token` now says that it holds the `JoinToken` text, as asked. |
| architect-r1 05 | major | D17 | 11 | — | fixed | One configured address and pin feed every token, and the session exemption gets a route test in the agreements. Whether a pin of the key meets the design is part of point 2 for the owner. |
| critic-r1 4 | major | D17 | 11 | architect-r1 05 | fixed | Both token makers use one source, and the server refuses an `https` token without a pin. |
| threat-r1 10 | major | D17 | 11 | architect-r1 05 | fixed | The client checks the pin before it sends the secret and on every stream, follows no redirect, and joins again when a key changes. |
| threat-r1 2 | blocker | D17 | 12 | — | for the owner | Whether the signed challenge protects the link behind the web server is a risk choice that the design does not settle. Point 2 in the plan. |
| architect-r1 08 | major | D7 | 13 | — | for the owner | The design says a waiting machine can only connect, and today's page shows its facts. Point 3 in the plan. |
| threat-r1 7 | major | D7 | 13 | architect-r1 08 | for the owner | As above. |
| threat-r1 4 | major | R9 | 14 | — | fixed | `CreateClientToken` is never replayed from a stored reply, so the secret is never kept. |
| threat-r1 5 | major | R9 | 15 | — | for the owner | The design writes the token as an argument of the command. Changing that is a product choice. Point 4 in the plan. |
| threat-r1 11 | major | R9 | 16 | — | fixed | `CommandResult.message` is bounded and written from fixed sentences, strings have limits, facts are self-reported, and identity never comes from a string of a machine. |
| architect-r1 10 | major | plan | 17 | — | fixed | The untracked probe is a valid file, a guard probe proves the guard, and the runner of the checks is named. |
| critic-r1 8 | major | plan | 17 | architect-r1 10 | fixed | As above, in a separate checkout with a clean check first. |
| logic-r1 12 | minor | plan | 17 | architect-r1 10 | fixed | As above. The new comment test adds a mutation of a rule, as asked. |
| critic-r1 11 | minor | plan | 18 | — | fixed | Absence checks use `! grep -q`, the session rule is scoped to `lgtm.v1`, and the toolchain floor and the link revision are checked. |
| logic2-r1 12 | minor | plan | 19 | — | fixed | `TestEveryLinkRuleIsWritten` checks every comment of the link files, the drafts now comment every part, and the guardrail row is corrected. |
| architect-r1 11 | minor | C7 | 20 | — | fixed | `Client.address` is the connection's address for showing only, memory becomes gigabytes of 2^30 bytes rounded, `harnesses_scanned` follows the first report, and the scouting row is corrected. |
| critic-r1 9 | minor | C7 | 20 | architect-r1 11 | fixed | `hosts_server` comes from a token of the server's own command on its machine, never from a client. |
| logic2-r1 11 | minor | C7 | 20 | architect-r1 11 | fixed | The machine sends its facts and harnesses after every `Welcome`, and an approved machine is joining until both arrived on its current stream. |
| logic-r1 10 | minor | C7 | 21 | — | fixed | Both load fields of `Heartbeat` are now `optional`, from 0 to 100. |
| logic-r1 11 | minor | R5 | 22 | — | fixed | A first position of 0 means an empty feed, and a Go server always sets the position. |

Counts by outcome: 47 fixed, 3 fixed another way, 0 rejected, 0 post-build checks, 9 for the owner, and 0 not folded.
The severities were 6 blockers, 40 majors and 13 minors.

## Points per statement

A minor counts 1 point, a major 3 and a blocker 9.
A group counts once, at its highest severity, under the statement of its lead.

| Statement | Groups | Points |
|---|---|---|
| C11 | 1 (blocker), 2 (major) | 12 |
| D7 | 3 (blocker), 13 (major) | 12 |
| D17 | 11 (major), 12 (blocker) | 12 |
| R9 | 14, 15, 16 (each major) | 9 |
| R6 | 7, 8 (each major) | 6 |
| plan | 17 (major), 18, 19 (each minor) | 5 |
| C9 | 4 (major) | 3 |
| R8 | 5 (major) | 3 |
| C5 | 6 (major) | 3 |
| D16 | 9 (major) | 3 |
| C6 | 10 (major) | 3 |
| C7 | 20, 21 (each minor) | 2 |
| R5 | 22 (minor) | 1 |

The round's total is 74 points.

## Stuck statements

A statement is stuck when it has at least 12 points and at least 30% of the round's points, which is 22.2 points here.
No statement is stuck.
C11, D7 and D17 each reach 12 points, which is 16% of the round.

## Mechanisms that held more than one serious finding

- **The order of events (C11, groups 1 and 2).** The mechanism was wrong. Gaps in the numbers and an acknowledgement that covered dropped events let a client delete an event that the server never applied. The plan replaces it with a contiguous prefix that the server commits before it acknowledges.
- **Removal (D7, group 3).** The mechanism was wrong. It forgot the key only after a message that may never arrive. Removal now takes effect at its commit, and the notice is a courtesy.
- **The certificate pin and the transport (D17, groups 11 and 12).** The pin's source was missing, and that is fixed. Whether the mechanism as a whole is right depends on point 2 for the owner, because nothing protects the messages after the handshake on the hop behind the web server.
- **The life of a command (R6, groups 7 and 8).** The mechanism of a stored command, an attempt number and a receipt is sound. Its rules were incomplete, and the plan now writes them all.
- **R9 (groups 14, 15 and 16)** holds three majors in three separate mechanisms: a stored reply, the argument of a command, and text from a machine. No one mechanism is at fault.

## Patches that round 2 attacks

Round 2 attacks only these places.

In `plan.md`:

1. Section "Overview", the paragraphs on the website and on the four points for the owner.
2. Section "Key decisions", the rows on events, commands, removal, the current stream, restart and the proof of a machine.
3. Section "Points for the owner", which is new.
4. "Public names that other teams build against": the rows `Hello.acked_event_number`, `COMMAND_OUTCOME_INTERRUPTED`, the optional load of `Heartbeat`, the error codes and what a client does with them, the value rules, `ReconnectClient` and `CreateClientTokenResponse.token`.
5. "The description format that the tests compare", rule 7 on reserved ranges and names.
6. Item 2, its tests written first and its acceptance.
7. Item 3, its files, statements, "Rules that item 3 writes" and "Not a comment-only change for the website".
8. Item 4, its rules of commands, the standing, events, heartbeats and facts, and the golden files `events.txt` and `commands.txt`.
9. Item 5, its rules of joining.
10. Item 6, its statements and mutations, the handshake, the value rules, the code table, the rules after `Welcome`, the current stream, removal, event marks and a waiting machine, the test `TestEveryLinkRuleIsWritten`, and the golden file `open.txt`.
11. Item 8, what changes and its tests written first.
12. "Done when", rows 2, 5 and 6.
13. "Risks", the rows on clocks and on a renewed certificate, and the four new rows.
14. "Guardrails triggered", the rows on comments and on the contract revision.
15. "Agreements with other teams, from review round 1", which is new.
16. "Post-build checks", which is new.
17. "Traceability", source items 8 and 9 and the three new rows.
18. "Attack log", round 1.

In the drafts: `draft/link.proto`, `draft/commands.proto`, `draft/events.proto`, `draft/common.proto` and `draft/existing-files.diff`, which together carry the rules above.

In `decisions.md`: rows 38 to 53.
In `scouting.md`: the corrected row on `message Client`, the five new anchors, and runs 17 to 21.

The three places marked UNREVIEWED MECHANISM need the closest attack:

- `Hello.acked_event_number`, decision 39.
- A signed `Challenge` for every well-formed `Hello`, followed by `unauthenticated`, as the trustworthy refusal of a removed machine, decision 42.
- The code table's rows for `unauthenticated` and `aborted`, decision 44.

## The top finding of the round

The top finding is that an acknowledgement could delete an event that the server never applied, because the server took any higher number and acknowledged dropped events.
It was a blocker.
Three reviewers rated it a blocker on their own (architect-r1 01, logic-r1 01, threat-r1 3), and a fourth rated it a major (critic-r1 5).

## Verdicts and grades of the reviewers

The grades use the scale from 0 to 10.

| Reviewer | Verdict | Grade | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| architect | REVISE | 4 | 2 | 8 | 2 |
| critic | REVISE | 5 | 0 | 8 | 3 |
| logic | REVISE | 5 | 1 | 8 | 3 |
| logic2 | REVISE | 5 | 0 | 7 | 5 |
| threat | REVISE | 4 | 3 | 9 | 0 |
