REVISE

The replacement rules can leave two streams current at once, so the ends can disagree about which stream may advance the machine's event mark.

## Findings

The reviewed patch runs from `eb9f2710` to `b043731f`. The plan, decisions, scouting and drafts matched `b043731f` in the checkout. Evidence is by reading unless marked confirmed live. The numbered findings cover the wire and include no dependency on restoring state or running two live copies of one machine.

### logic-r4 01 — MAJOR — C11 — Stream replacement has two cutover points

**Evidence.** A stream is current from `Welcome` until it ends, but a newer verified `Proof` replaces it. Check 4 keeps stream A open until 50 milliseconds after stream B's `Welcome`, and expects late traffic on A to change nothing. Only the current stream's accepted events may advance the mark. `.plans/plans/go-s0-contract/plan.md:540`, `.plans/plans/go-s0-contract/plan.md:545`, `.plans/plans/go-s0-contract/plan.md:758`, `.plans/plans/go-s0-contract/draft/link.proto:89`.

**Failure.** A carries event 41 and the committed mark is 40. B proves itself and prepares a `Welcome` with mark 40. If A is still current until it ends, event 41 can commit and A can return an acknowledgement before it ends. B can then expose 40 after the same outbox already observed 41. The contract calls that regression a restore or copy. The same ordering also lets A qualify as current when the new readiness test requires that A have no effect. `.plans/plans/go-s0-contract/plan.md:358`, `.plans/plans/go-s0-contract/plan.md:362`, `.plans/plans/go-s0-contract/plan.md:372`.

**Fix.** Define one cutover when the server admits B to send `Welcome`, after verifying B's proof. At cutover, A ceases to be current and its connection token loses authority, even while its terminal status is in transit. B's `Welcome` includes every event A committed before cutover. Later traffic on A has no effect and cannot advance the mark. Add a check that holds A's event 41 and B's `Welcome` on either side of cutover. The server plan can choose how to implement that order.

### logic-r4 02 — MAJOR — D7 — Removal cannot retract a Welcome in transit

**Evidence.** The patch forbids receipt of `Welcome` after removal commits. Check 10 covers barriers before the server sends the frame. The transport may still hold the frame after the server sends it. `.plans/plans/go-s0-contract/plan.md:553`, `.plans/plans/go-s0-contract/plan.md:764`, `.plans/plans/go-s0-contract/draft/link.proto:109`.

**Failure.** The server sends B's `Welcome` while the machine exists. The web server holds those bytes. Removal commits and the Go server sends the terminal refusal. The web server forwards the buffered `Welcome` and then the refusal in stream order. Even perfectly ordered server writes cannot satisfy the ban on receipt.

**Fix.** Order the removal commit against the server's send of `Welcome`. If removal wins, the server sends no `Welcome` and ends B with `failed_precondition` and `unknown_machine`. If the send wins, an already sent `Welcome` may arrive before the refusal, but removal invalidates its token and subsequent messages have no effect. Keep the pre-send barriers and add a separate held-frame case that checks the second branch.

### logic-r4 03 — MAJOR — C10 — The stream refusal reason needs a terminal carrier

**Evidence.** The contract calls `Lgtm-Link-Refusal` a header and requires it for refusals after `Challenge`, including event gaps long after `Welcome`. Connect sends leading response headers with the first response and carries trailing metadata in the final envelope. This matches the pinned Go dependency's `BidiStream.ResponseHeader`, `BidiStream.ResponseTrailer` and `Error.Meta` documentation at version `1.21.1`. `.plans/plans/go-s0-contract/plan.md:196`, `.plans/plans/go-s0-contract/draft/link.proto:39`, `.plans/plans/go-s0-contract/draft/link.proto:79`, `go.mod:11`. [Connect protocol reference](https://connectrpc.com/docs/protocol/#error-and-endstreamresponse)

**Failure.** The server sends `Challenge` and `Welcome`, then receives a gap. A server builder who writes to the named response header can no longer send `event_gap` there. A client builder who reads the leading headers cannot recover the reason even when the server includes it in the terminal metadata. The same mismatch breaks the removal check's required `unknown_machine` reason. `.plans/plans/go-s0-contract/plan.md:759`, `.plans/plans/go-s0-contract/plan.md:764`.

**Fix.** Define `Lgtm-Link-Refusal` as one value in the terminal status metadata of `Open`. For Connect, put it in the final envelope, and keep the unary `Lgtm-Join-Refusal` in the error response headers. Name the server status writer, the client terminal-status reader and the Stack Web forwarding path. Check the actual metadata after both an early refusal and a refusal after `Welcome`.

### logic-r4 04 — MAJOR — C7 — The expiry exception does not keep approval deliverable

**Evidence.** A post-expiry attempt only collects a recorded result, the server finalizes unanswered commands as expired at 24 hours, and the patch tells the machine to apply `SetState` regardless of expiry. An approved machine supplies the readiness reports when it applies the state. `.plans/plans/go-s0-contract/plan.md:338`, `.plans/plans/go-s0-contract/plan.md:342`, `.plans/plans/go-s0-contract/plan.md:364`, `.plans/plans/go-s0-contract/draft/commands.proto:6`, `.plans/plans/go-s0-contract/decisions.md:75`.

**Failure.** A waiting machine stays connected. The server commits approval revision 2, but delivery of the command is held before the first send. At one second after expiry, the command rules describe the attempt as result collection only and the state rule says its first delivery must apply the approval. If delivery is held until after automatic finalization, the server must never send that command again. The connected machine can stay at revision 1 and never send either approval report. Check 6 injects an already expired `SetState`, so it cannot expose a server that never delivers it. `.plans/plans/go-s0-contract/plan.md:760`, `.plans/plans/go-s0-contract/plan.md:767`.

**Fix.** State explicitly that a `SetState` attempt may apply a newer revision on first receipt after expiry. Preserve the rule that retries do not repeat the same revision's effects. If the server finalizes the envelope before the current stream learns the latest standing, require the server to deliver that standing in another `SetState` or to end the stream so the next `Welcome` supplies it. Check both a late first send and a command that finalizes before any send.

### logic-r4 05 — MAJOR — R6 — The wire lost the condition that makes the expiry promise possible

**Evidence.** The replaced decision limits the old guarantee to a server clock that has not jumped since `Welcome`. The revised rule guarantees that no ordinary command starts after expiry by the server's clock, using a time sample taken before sending `Welcome`. No later frame carries a clock sample or an indication that the old sample lost validity. `.plans/plans/go-s0-contract/decisions.md:66`, `.plans/plans/go-s0-contract/plan.md:341`, `.plans/plans/go-s0-contract/plan.md:710`, `.plans/plans/go-s0-contract/draft/link.proto:237`, `.plans/plans/go-s0-contract/draft/events.proto:103`.

**Failure.** An ordinary command arrives with expiry 110 after the server sampled time 100. The machine is ready to start at the instant the server would ordinarily read 105. In another run the server clock jumps forward to 115 after the command was sent. The client sees the same frames and the same elapsed time in both runs. Starting the command in the first run starts it late in the second. A local upper estimate cannot bound an unannounced server clock step.

**Fix.** State the clock condition under which the wire promises never to start late. Restore the condition that the server clock stays within the bounds promised for the session after its sample, or specify a shared lease or time protocol with an explicit validity condition. Make the expiry boundary inclusive or exclusive in the same clause. Keep the local dispatch and receipt mechanics with their owning plans.

### logic-r4 06 — MAJOR — C11 — Invalid payloads have competing repeat and gap outcomes

**Evidence.** The general value rule refuses invalid messages and names a single exception for a report at the exact next number. The patched event rules classify numbers first and say that only the exact next number reaches its report. `.plans/plans/go-s0-contract/plan.md:360`, `.plans/plans/go-s0-contract/plan.md:534`, `.plans/plans/go-s0-contract/draft/link.proto:9`, `.plans/plans/go-s0-contract/draft/events.proto:13`.

**Failure.** With mark 42, a decodable repeat of event 42 includes a workspace string longer than 256 bytes. Number-first handling returns `EventAck.through = 42` and keeps the stream. The general rule can instead end the stream with `invalid_argument`. Event 44 with the same payload can similarly get either `invalid_argument` or `failed_precondition` with `event_gap`. One client can continue its ordered backlog while the other loses its stream over a payload that the event rules say is never read. `.plans/plans/go-s0-contract/draft/events.proto:70`.

**Fix.** Give the entire event decision table precedence over the general value rule after `Welcome`. For a decodable event, check that the number is positive, then handle a repeat or a gap without inspecting the report. For the exact next number, apply a valid report or consume an unusable one. Keep undecodable messages subject to the transport's decode rule. Add the invalid repeat and invalid gap to check 3.

### logic-r4 07 — MAJOR — C11 — A final event gap can clear without owner action

**Evidence.** The added code table defines final as the same answer until the owner acts, and labels a proven `Open` refusal for `event_gap` final. The event rules compute the gap from the current committed mark. `.plans/plans/go-s0-contract/plan.md:536`, `.plans/plans/go-s0-contract/draft/link.proto:36`, `.plans/plans/go-s0-contract/draft/events.proto:13`.

**Failure.** With mark 40, an event numbered 42 ends an established stream with `event_gap`. On a later stream the same machine sends 41, then the same event 42. The event rule requires acceptance of 42. The final-code definition requires the same refusal until an owner acts. The contract gives no persistent latch, and a server that adds one would reject a now-in-order report. A revoked join token crossing expiry also changes its refusal reason without owner action.

**Fix.** Define the named codes and reasons against the request and the server state at that decision. A gap terminates the stream on which it occurs. State explicitly whether a later stream can proceed after the missing number is supplied. Keep the client's decision to dial again in the client plan. Make the same state qualification apply to the ordered reasons from `Join`.

### logic-r4 08 — MAJOR — C5 — Concurrent first use has an unconditional replay outcome across expiry

**Evidence.** The patched rule tests expiry and revocation before a same-key replay, and also says a same-key loser of a simultaneous first use gets the winner's reply. Check 12 exercises the simultaneous pair without varying eligibility. `.plans/plans/go-s0-contract/plan.md:466`, `.plans/plans/go-s0-contract/plan.md:467`, `.plans/plans/go-s0-contract/plan.md:766`, `.plans/plans/go-s0-contract/draft/link.proto:60`.

**Failure.** Two identical valid first requests reach the server just before expiry. A claims the token and commits. B reaches its binding decision just after expiry. The concurrent-first-use rule says that B gets A's reply. Applying the same refusal table to the state B now sees gives `token_expired`. The machine may learn either that it joined or that it must get a new token, despite the same committed claim. The same ambiguity occurs when revocation wins between A's claim and B's decision.

**Fix.** Define eligibility at each claim or replay decision and explicitly apply the full ordered table to the losing request at that point. A same-key loser gets the first reply while the token remains eligible and its machine exists. A different-key loser gets `token_used` while the earlier refusal conditions are clear. Add one fixture where the loser reaches its decision after expiry and one where revocation wins before the replay decision.

## Answers to the round-four assignment

1. **Patch dispositions.** The table below covers all 14 entries in the round-three register, including changes to the drafts and scouting. `.plans/plans/go-s0-contract/attack/round-3-findings.md:94`.
2. **Unreviewed mechanisms.** Permanent event numbers remove the original collision, and the one-outbox premise covers the excluded copy case. Ordinary replacement still needs a consistent mark, and repeat, gap and validation precedence is incomplete. Actual application of late `SetState` helps once the frame arrives, but the command rules can prevent delivery. Timestamp sufficiency and the observable removal fence remain open. Delegating installation, exit status and retry mechanics leaves the refusal and token wire shapes intact once the carrier is fixed. See findings 01 through 07.
3. **Rejections.** Nothing found. The register's outcome column contains no rejected finding. The threat finding concerning the unencrypted hop is assigned to the owner, and the patch states its consequences for acknowledgements and server time. `.plans/plans/go-s0-contract/attack/round-3-findings.md:55`, `.plans/plans/go-s0-contract/attack/round-3-findings.md:59`.
4. **Writers and readers.** The table below accounts for every changed public name in register entry 3 and its observable handoff. The main gap is the word `header` for a late `Open` status, which gives readers and the writer different possible carriers.
5. **Rules that interact.** The decision tables below give the concrete cells for events, stream admission, join and command delivery. The unresolved cells are numbered as findings.
6. **Round-three blockers and majors still open.** The event collision and the secret bypass are closed. The removal finding stays open at the wire boundary, event validation has two remaining branches, and the late approval and clock findings remain open in findings 02, 04, 05 and 06. The registry refusal code is consistent by itself and its new carrier is finding 03. The former sprint-zero freeze condition survives in old prose covered by the ruling in rollout step 9.
7. **Logic lens.** The eight answers below apply the lens to the changed wire rules. The tests of shape and comments are separate evidence from the post-build behavior checks.

### Each patch in the round-three register

| Register entry | Disposition by reading |
|---|---|
| 1. Key decisions | One outbox and immutable numbering close the renumbering collision. The response address still identifies the token's endpoint. Current-stream cutover, receipt of a buffered `Welcome`, and the clock guarantee remain open in findings 01, 02 and 05. |
| 2. Rollout order and owner points | Step 9 and decision 63 put the freeze after both reviews, and the added transport cost is stated. The workstream and first risk row still say changes go back to planning after the sprint merge. The explicit ruling governs this process wording. `.plans/plans/go-s0-contract/plan.md:93`, `.plans/plans/go-s0-contract/plan.md:108`, `.plans/plans/go-s0-contract/plan.md:705`. |
| 3. Public names | Event marks, result fields and address have named writers and readers. The public names table still calls a late `Open` reason a header, repeats the final-code rule, and names the narrower event exception. Findings 03, 06 and 07 apply. |
| 4. Item 2 merge note | Confirmed live that `28208cb4` contains the Go module and generation changes and `ac1ff06a` contains the check script. `git apply --check` accepts the rebased draft diff. Nothing found in the wire. `.plans/plans/go-s0-contract/plan.md:258`, `src/contracts/proto/lgtm/v1/clients.proto:11`. |
| 5. Item 4 | Permanent numbers close the prior collision. The result field stays reserved and the projection requires both valid reports. The late state, clock, overlap and event classification concerns are findings 01 and 04 through 07. The accepted `Restart` outcome still has a defined meaning. |
| 6. Item 5 | A well-formed bad secret and an unknown token now share one reason, including on a would-be replay. The address scheme and conditional pin presence remain specified. The race outcomes need the eligibility qualification in finding 08. |
| 7. Item 6 | A well-formed `Hello` receives `Challenge` before refusal; a supported proof can proceed to `Welcome`; a waiting `Welcome` gets an empty token. Cutover, removal, late refusal metadata, finality and event checks need findings 01 through 03, 06 and 07. |
| 8. Risks | Copies and restores remain outside the supported state space. One live outbox still needs a consistent replacement mark. Describing the sharp edges of the unencrypted hop leaves the owner choice open. The clock and refusal risks repeat the disputed promises. |
| 9. Agreements | The handoffs cover the changed fields, codes, addresses and result text. The stream agreement repeats the in-flight receipt ban. The writers and readers need the shared carrier and order in findings 01 through 03. |
| 10. Post-build checks | Checks 4, 12, 14 and 15 add missing replacement, join, endpoint and reserved-field assertions. Check 3 establishes ordinary immutable replay. Checks 3, 6, 10 and 12 still miss the conflicting branches described in findings 01, 02, 04, 06 and 08. Check 11 measures a recorded answer near the collection boundary but holds the server clock steady. |
| 11. Traceability and attack log | The log records the owner rulings and every prior group's disposition. The earlier collision appears closed, while the removal, validation, expiry and late approval rows still need the wire corrections above. |
| 12. Draft files | Confirmed live that the rebased `existing-files.diff` applies. The three link draft changes are comments, and they preserve field declarations and the reservation. The draft comments repeat the wire inconsistencies in findings 01 through 08. |
| 13. Decision rows | Rows 63, 65 and 67 settle the freeze, no renumbering and readiness. Rows 64 and 68 remove internal prescriptions but leave the disputed clock, state delivery and receive boundary. Row 66 repeats the streaming-header ambiguity. |
| 14. Scouting | Run 29's commit and patch claims were confirmed live. Current tracked contract files include the generated web contract and no built machine link. Runs 27 and 28 record earlier tool results, and runs 30 and 31 describe proposed writers. Run 32's script consumer exists at `deploy/test/checks.sh:345` and `deploy/test/checks.sh:505`. |

### Writers and readers of changed wire names

| Wire name | Writer and reader check |
|---|---|
| `Event.number`, `Welcome.last_event_number`, `EventAck.through` | Client Link writes immutable event numbers from its outbox. Server Link reads them and writes the two committed marks. Client Link consumes both. The missing piece is the cross-stream mark rule in finding 01. |
| `COMMAND_OUTCOME_INTERRUPTED` and removed `CommandResult.message` | Client Link writes the outcome and no field 4. Server Link reads the typed outcome and discards unknown fields. Server Machines owns the text read by the website. Check 15 is the only intended writer of the old tag in current tests. |
| `CreateClientTokenResponse.address` | Server Machines supplies the embedded token's address. The website reads the reply; a sample server may leave it empty. Enrollment reads the address from `JoinToken`. |
| `Lgtm-Join-Refusal`, including `machine_removed` | Server Machines writes it and Client enrollment reads it. Stack Web passes it through. The wave-one check reads the client's diagnostic, as confirmed by `.plans/plans/go-s0-contract/plan.md:195` and `deploy/test/checks.sh:339`. |
| `Lgtm-Link-Refusal` and the three reason values | Server Link and Server Machines write it, Client Link reads it, Stack Web carries it and the check reads the client's diagnostic. Finding 03 supplies the missing terminal carrier. |
| `Welcome.connection_token`, the status codes and value rules | Server Link writes the token, including the empty waiting token. Client Link and later side calls read it. The server writes the checks' status codes, both ends validate and the clients read those codes. Findings 06 and 07 identify inconsistent cases in those rules. |

### Logic lens answers

1. **Ordering.** A committed mark starts at 0, a positive next number advances it, and a duplicate repeats the committed mark. An outbox replay preserves numbers after a restart. State revisions and command attempts start at 1 and do not wrap. Event-number exhaustion has no specified successor in the finite type and does not affect the reviewed wave-one scenarios. Cutover, the token cutoff and the server time sample have separate findings.
2. **Failure and repair.** A failed event commit leaves the mark unchanged, and a lost acknowledgement is repaired from the next `Welcome`. A database read failure gives `unavailable`. Repeat replies, expired joins and malformed frames are covered by the decision tables. Removal in transit and convergence after a delayed approval still need the described wire outcomes.
3. **Truth tables.** The event table covers the valid and invalid payload in each sequence class. The stream and command table covers overlapping authority and delivery. The join table checks refusal precedence and the two losing claim branches.
4. **Zero values.** Zero means no committed events in `Welcome` and an invalid incoming `Event.number`. Unspecified approval means waiting, so no connection token is issued to a waiting `Welcome`. A missing state is invalid and an unspecified command outcome is recorded as failed. An unsupported `Hello.link_revision`, including zero, is answered after the challenge. The sample reply may omit its address.
5. **Concurrent actors.** One outbox can use a replacement stream while A's messages are still in transit. A and B need one authority boundary and the later `Welcome` needs the corresponding mark. A removal can race either admission or delivery. Two first uses can race each other, and a revocation can race either a replay or approval.
6. **Risky mutations.** Item 3's shape test detects loss of `position`, and its common golden description detects narrowing the type. Item 4's golden descriptions detect a wider attempt and a missing state revision. Item 5's descriptor detects the renumbered hash and its example detects the wrong prefix. Item 6's example detects the reversed nonces, its descriptor detects the missing mark, and the comment test detects a missing or empty comment. Each of these gates can still pass with an incorrect comment on an observable rule.
7. **Cheapest wrong implementation by item.** Item 1 could declare a higher patch of Go 1.26 and pass where that version is cached. Item 2 could leave an ignored generated file undetected by its visible untracked-file probe. Item 3 could document the address as the browser origin and retain the correct descriptor. Item 4 could reject an invalid repeat. Item 5 could skip verifying the secret on replay while preserving all bytes in the example. Item 6 could check only the leading headers for `Open` refusals while preserving every descriptor and comment keyword. Item 7 could omit `go vet` and still reject both listed probes. Item 8 could mention the expected words without explaining their rule. Only the patched wire cases count toward the verdict.
8. **Representing safe states.** One authoritative stream cutover makes it impossible for two streams to advance one machine after B is admitted. A single event decision table prevents generic payload validation from changing a repeat or gap. A separate terminal metadata carrier makes refusal after the first response representable. A state revision can represent convergence independently of the lifetime of the command that carries it.

### Event decision table

The server mark is 42. The payload is decodable in every row.

| Incoming number | Valid report | Missing or unknown report | Known report that breaks a value rule |
|---|---|---|---|
| 0 or absent | `invalid_argument` | `invalid_argument` | `invalid_argument` |
| 1 through 42 | Acknowledge 42 without applying the report. | Acknowledge 42 without applying the report. | The number rule says acknowledge 42. The value rule can refuse. Finding 06. |
| 43 | Apply and acknowledge 43 after commit. | Consume and acknowledge 43 after commit, with no readiness. | Consume and acknowledge 43 after commit, with no readiness. |
| 44 or higher | End with `failed_precondition` and `event_gap`. | End with `failed_precondition` and `event_gap`. | The number rule says gap. The value rule can say `invalid_argument`. Finding 06. |

If 43 is accepted on a later stream, the same 44 is now the next event. The event rule and final-code definition disagree in that cell. Finding 07.

### Stream, approval and command decision table

| State | Wire outcome or unresolved cell |
|---|---|
| B's proof verifies while A is open, before B's `Welcome` | The replacement clause can end A now, while the current-stream clause and the `Welcome` declaration do not yet grant authority to B. Finding 01. |
| B's `Welcome` takes effect while A's terminal status is pending | B is current. The literal until-end rule can still qualify A and let it advance the mark. Finding 01. |
| Removal commits before the server sends B's `Welcome` | Send no `Welcome`. End B with `failed_precondition` and `unknown_machine`, conveyed in terminal metadata. Findings 02 and 03 distinguish the send and the carrier. |
| Removal commits after B's `Welcome` is sent and before the machine reads it | The ordered frames can deliver `Welcome` then the final status. The receive ban conflicts with that order. Finding 02. |
| Waiting `Welcome` or approved reconnect | A waiting `Welcome` carries an empty token. A stream opened after approval can get a token. An approved stream with fewer than two applied valid reports is joining, and one with both is online. An approved machine without a current stream is offline. |
| Ordinary command has a running or completed receipt, followed by a higher attempt | The result uses the newest received attempt. A result from an older attempt or stream has no effect. A finished answer remains the same across expiry during collection. |
| Ordinary command has no receipt and only an old time sample before a server clock step | The client cannot distinguish a timely start from a late one. Finding 05. |
| `SetState` has a higher revision and no receipt, and arrives after expiry but before automatic finalization | The state exception says apply it. The attempt rule says collect a result only. Finding 04. |
| `SetState` has never arrived when the server finalizes the command | The command can never be sent again, and the wire gives no convergence step while the stream stays open. Finding 04. |

### Join decision table

Read the rows in priority order, after the request passes its value rules.

| State at decision | Wire outcome or unresolved cell |
|---|---|
| Unknown identifier or wrong secret | `permission_denied` with `token_unknown`, including a used token that would otherwise replay. |
| Correct secret, token expired | `permission_denied` with `token_expired`, including an expired and revoked token. |
| Correct secret, token unexpired and revoked | `permission_denied` with `token_revoked`. Revocation removes a still-waiting machine and leaves an already approved one. |
| Eligible token bound to another key | `permission_denied` with `token_used`. |
| Eligible token bound to this key, machine removed | `permission_denied` with `machine_removed`. |
| Eligible token bound to this key, machine exists | Replay the first reply. |
| Eligible and unused | Claim once, or return `unavailable` if the claim fails before commit. The losing request's reply is clear while eligibility remains unchanged. |
| Same-key first-use loser reaches its decision after expiry or revocation | The race clause promises the reply and the refusal table denies it. Finding 08. |

## Post-build checks

- Hold B's start at the cutover while A sends the next event. If A commits first, B's `Welcome` carries that mark. If B takes over first, the delayed A frame has no effect. Inspect both streams and the durable mark.
- Relay a pre-removal `Welcome` only after removal has committed. Inspect the ordered frames and final metadata. An already sent `Welcome` may arrive, followed by `unknown_machine`, and subsequent messages on the stream change no record.
- Through the real web server, read the terminal metadata after a challenge-only refusal and after a gap on a welcomed stream. Expect the exact code and single reason value in each terminal status.
- Delay the first delivery of approval on a healthy waiting stream past the command's expiry and then separately past finalization. Expect the current standing to reach the client and both valid approval reports to reach the server, by the agreed delivery rule.
- At mark 42, send an invalid repeat, an invalid gap on a separate stream, and an unusable exact-next report followed by a valid one. Expect respectively acknowledgement of 42, termination with `event_gap`, and advancement without readiness for the unusable report.
- Let a token claim commit before expiry and release a same-key replay decision after expiry. Expect only one machine and the refusal chosen by the specified eligibility rule for the later decision.

## Grade

5 out of 10. There are 0 blockers, 8 majors and 0 minors.
