REVISE

The removal rule forbids a machine from receiving a `Welcome` after removal commits, even when the server sent it before the commit and the transport delayed its delivery.

## Findings

### architect-r4 01 · MAJOR · D7 · Put removal and `Welcome` in an achievable order

By reading, the patched rule forbids receipt of `Welcome` once removal commits, and check 10 only pauses the server before it sends `Welcome`. The same check expects refusal in the race between approval and revocation even though the revocation leaves an already approved machine in place. Evidence is `.plans/plans/go-s0-contract/plan.md:553`, `.plans/plans/go-s0-contract/plan.md:764`, `.plans/plans/go-s0-contract/draft/link.proto:110` and `.plans/plans/go-s0-contract/draft/link.proto:71`.

The server sends `Welcome` for stream B, then the owner removes the machine while the proxy still holds those bytes. When the proxy resumes, B can receive the already sent `Welcome` followed by the refusal. The server cannot retract the first message. In the approval race, approval can commit first, revocation can leave the machine approved, and B can correctly receive `Welcome`. The test would still demand a refusal.

Order the commit against the server's emission of `Welcome`. If removal wins, the server emits the terminal refusal without `Welcome`. If emission wins, the peer can observe `Welcome` followed by the removal refusal, and the token ends at removal. State both observations in the draft and check 10. Require the refusal in the revocation race only when revocation actually removes the machine. Server Machines still owns how the server orders those operations.

### architect-r4 02 · MAJOR · C11 · Define which event validation can spend a number

By reading, the blanket value rule exempts only the next event's report, while the event order says that a repeat is acknowledged and a gap is refused before either report is examined. The file rule also requires valid Unicode Transformation Format, 8-bit (UTF-8) text. Evidence is `.plans/plans/go-s0-contract/plan.md:360`, `.plans/plans/go-s0-contract/plan.md:361`, `.plans/plans/go-s0-contract/draft/link.proto:9`, `.plans/plans/go-s0-contract/draft/link.proto:18`, `.plans/plans/go-s0-contract/draft/events.proto:13` and `.plans/plans/go-s0-contract/draft/events.proto:18`.

With a committed mark of 42, a decodable `Facts` report with a 65-byte `os` at number 42 gets either `invalid_argument` or acknowledgement through 42 under the two rules. At number 44, the same report gets either `invalid_argument` or the terminal `event_gap`. At number 43, the draft promises to consume an invalid report, but the pinned Protocol Buffers decoder rejects a report containing invalid UTF-8 before the stream handler can apply the event rule. A scratch run with generated Go confirmed that the over-limit string decodes and the invalid UTF-8 string fails to decode. See `.plans/plans/go-s0-contract/draft/events.proto:61` and `go.mod:10`.

Specify three steps. A failure to decode the message ends the stream with `invalid_argument` without advancing the mark. For a decoded event, check the envelope and sequence before checking any report fields, and let repeats and gaps take their stated paths regardless of report content. For the next decoded event, consume an unusable report without setting readiness. Make the link-level exception and check 3 cover every row in the event table below.

### architect-r4 03 · MAJOR · C10 · Carry the link refusal in the terminal status metadata

By reading, the public-name table and the draft call `Lgtm-Link-Refusal` a header, and the handoff assigns Stack Web to pass headers unchanged. The same draft permits the server to produce this reason after `Challenge`, or long after `Welcome`. Evidence is `.plans/plans/go-s0-contract/plan.md:196`, `.plans/plans/go-s0-contract/plan.md:738`, `.plans/plans/go-s0-contract/draft/link.proto:41`, `.plans/plans/go-s0-contract/draft/link.proto:109` and `deploy/test/checks.sh:505`.

Confirmed live with the pinned Connect handler and an in-process response recorder, a refusal produced after an ordinary stream message left the initial Hypertext Transfer Protocol (HTTP) headers empty. Connect carried the reason in its end-of-stream envelope. The gRPC protocol carried it in HTTP trailers. A client that reads the initial header as the contract directs gets no reason from either transport. It therefore cannot classify the removal that check C10 expects.

Specify the logical key in terminal remote procedure call (RPC) error metadata. Name its carrier for Connect and for gRPC. The server writes it when terminating `Open`, the client reads it from the terminal error, and Stack Web preserves the stream body and trailers. Keep the unary `Join` reason in response error metadata and test both refusal paths through the proxy.

### architect-r4 04 · MAJOR · R6 · Give an expired `SetState` one answer

By reading, an attempt after expiry only collects a recorded result, and the new exception says a machine applies `SetState` whatever its expiry. The draft also defines `Command.expires_at` as the cutoff for every action. Evidence is `.plans/plans/go-s0-contract/plan.md:338`, `.plans/plans/go-s0-contract/plan.md:342`, `.plans/plans/go-s0-contract/plan.md:760`, `.plans/plans/go-s0-contract/draft/commands.proto:6`, `.plans/plans/go-s0-contract/draft/commands.proto:16` and `.plans/plans/go-s0-contract/draft/commands.proto:33`.

A waiting machine receives its first approval `SetState` after expiry on the same stream, with no prior result. The collection rule gives the client no result to return without starting work. The exception requires it to apply the approval. If the client follows the collection rule and replies `COMMAND_OUTCOME_EXPIRED`, the server treats that as final, and the machine can keep the waiting state until a new stream.

Apply the exception to both sides of the command life cycle. State that a first delivery of `SetState` can take effect after expiry, that the client applies it only when its revision advances, and that a valid stale or already-applied state returns the specified successful no-op outcome. Preserve recorded outcomes on replay. State separately when the server stops sending the command and how the latest standing is conveyed if the command is no longer pending. Make the late-approval check observe the result and the two reports on the same stream.

### architect-r4 05 · MAJOR · R6 · Define the expiry result when the wire cannot establish the start time

By reading, round 3 removed the explicit permission to refuse early. It retains an expiry outcome that means the command arrived after its expiry, while promising never to start a command after expiry by the server's clock. `Welcome.server_time` gives one server timestamp before transmission. A later command carries the deadline and no fresh time sample. Evidence is `.plans/plans/go-s0-contract/plan.md:341`, `.plans/plans/go-s0-contract/plan.md:343`, `.plans/plans/go-s0-contract/draft/link.proto:237` and `.plans/plans/go-s0-contract/draft/commands.proto:75`.

An unstarted `Restart` arrives close to its deadline. Two connections can give the client the same `Welcome.server_time`, the same elapsed time on its clock and the same deadline, while placing the actual server time on opposite sides of that deadline. Different delays before and after the timestamp was sampled are enough. Starting work can violate the deadline in one connection. Returning `COMMAND_OUTCOME_EXPIRED` can assert an arrival after expiry that did not happen in the other connection. The server closes the command on that result.

Allow the client to decline an unstarted expiring command when the available timing evidence cannot establish that it can start before the deadline. Define the outcome so it includes that case and specify that a recorded result takes precedence during the collection window. The other valid choice is to put sufficient timing evidence on the wire and define what happens when it is unavailable. Keep the local clock and receipt implementation in the Client Link plan.

### architect-r4 06 · MAJOR · C10 · Give the post-challenge checks a precedence

By reading, the revised handshake prescribes three different refusals after `Challenge`, and the only ordering constraint is that the challenge comes first. A missing or late `Proof` also has a prescribed code. The same status table promises that the server repeats the same answer until the owner acts. Evidence is `.plans/plans/go-s0-contract/plan.md:529`, `.plans/plans/go-s0-contract/plan.md:532`, `.plans/plans/go-s0-contract/plan.md:536`, `.plans/plans/go-s0-contract/draft/link.proto:36`, `.plans/plans/go-s0-contract/draft/link.proto:79` and `.plans/plans/go-s0-contract/draft/link.proto:85`.

A removed machine with an obsolete revision presents a well-formed `Hello`. After issuing the challenge, a server checking the identifier first sends `unknown_machine`. One checking the revision first sends `link_revision_mismatch`. If the server instead waits for `Proof`, the same valid `Hello` can end with `deadline_exceeded` when no proof arrives. A known machine with an unsupported revision and an invalid signature also has both a final and a retryable rule. For `Join`, a request with a verified secret for a token bound to another key receives `token_used` right before expiry and `token_expired` right after it, with no owner action. The reason is part of the refusal answer.

Choose one post-challenge order. For example, after the well-formed `Hello` and `Challenge`, check whether the machine lookup succeeded, whether the machine exists, and whether the revision is supported. Only a recognized machine at a supported revision proceeds to the `Proof` deadline, value checks and signature check. State that early refusals need only the verified server challenge and can occur before the client completes `Proof`. Define finality as a class of the refusal at that point in the exchange, with reasons evaluated against current state on each new request. Keep retry behavior in Client Link.

### architect-r4 07 · MAJOR · C7 · Make the machine status cases disjoint

By reading, the patched joining condition covers an approved machine until the server has applied both valid reports from its current stream. It does not require a current stream to exist. The same table prescribes offline when the approved machine has no current stream. The online line also omits the current-stream predicate explicitly stated for joining. Evidence is `.plans/plans/go-s0-contract/plan.md:371`, `.plans/plans/go-s0-contract/plan.md:372`, `.plans/plans/go-s0-contract/plan.md:374`, `.plans/plans/go-s0-contract/plan.md:758` and `.plans/plans/go-s0-contract/draft/link.proto:97`.

An owner approves an enrolled machine while it is disconnected and it has never reported. It has no current stream and has applied neither of the required reports. A server that evaluates the written clauses in order can publish joining indefinitely instead of offline. A server that keeps the readiness from an older stream can publish online in the equivalent case after a disconnect. These outcomes change the status in `WatchClients`.

Require a live current stream for both joining and online. While waiting, publish waiting. While approved without a current stream, publish offline. While approved with a current stream, publish online after both valid reports were applied on that stream while approved, and publish joining otherwise. Keep check 4's receiving-end observation of the offline transition.

| Finding | Category and evidence status | Responsible builders | Risk score |
|---|---|---|---|
| 03 | Wire compatibility, confirmed with a recorded response. | Contract, Client Link, Server Link and Stack Web. | 180 = 6 for unclassified removal, 5 for coordinated repair and 6 for an ordinary removal. |
| 07 | Protocol state, confirmed by reading. | Contract and Server Machines. | 108 = 6 for a status that can stay wrong, 3 for recovery on reconnect and 6 for an ordinary disconnect. |
| 04 | Protocol state, confirmed by reading. | Contract, Server Link and Client Link. | 72 = 6 for an approval that can stay unapplied, 3 for recovery on reconnect and 4 for delayed delivery. |
| 05 | Protocol state, confirmed by reading. | Contract, Client Link and Server Link. | 72 = 6 for a command closed on an ambiguous outcome, 3 for a fresh command and 4 for delivery near expiry. |
| 06 | Wire compatibility, confirmed by reading. | Contract, Server Link and Client Link. | 72 = 6 for a wrong final disposition, 3 for recovery on retry or update and 4 for overlapping refusal conditions. |
| 01 | Protocol order, confirmed by reading. | Contract and Server Machines. | 36 = 6 for a false admission observation, 3 for the following terminal refusal and 2 for coincident delivery and removal. The conflicting check also needs correction for the release gate. |
| 02 | Protocol order and wire compatibility, confirmed in the text and by a decoder probe. | Contract and Server Link. | 36 = 6 for a queue that fails to drain, 3 for a corrected event and 2 for an invalid report interacting with sequence handling. |

## Answers to the assignment's seven questions

1. The table below covers all 14 patch sites in the register and identifies the failures that remain.
2. The event markers and decisions 54 and 65 preserve the server-owned committed mark and remove renumbering. Finding 02 covers the validation order that remains. Decision 55 and the handshake and refusal markers are covered by findings 03 and 06. Decision 56 and the stream-order marker are covered by finding 01. Decision 68 and the command and receipt markers are covered by findings 04 and 05. For the issuance marker, the token address and pin are expressible and the transport-facing producers and readers are identified below. Restore and two live copies remain outside wave 1.
3. Nothing found. The outcome table has no rejected item to validate. The one owner referral is the choice of transport protection, and its added consequences match the fact that acknowledgements and `Welcome.server_time` are outside the proof texts. `.plans/plans/go-s0-contract/attack/round-3-findings.md:55`, `.plans/plans/go-s0-contract/attack/round-3-findings.md:59`, `.plans/plans/go-s0-contract/draft/link.proto:218`
4. The public-name table below names the producers, consumers and intermediaries. The incomplete handoff is the terminal carrier and its reader in finding 03.
5. The event, handshake, command and projection tables below give the cases that now intersect. Removal has an additional observable order in finding 01.
6. The round 3 secret-check blocker is closed on both first use and replay. The repeat-number contradiction and the collision from renumbering are gone. The removal and refusal group, the invalid-report precedence group and the command timing promise still need the changes in findings 01 through 06. The projection repair fixes validity and stream ownership for reports, with the remaining overlap in finding 07.
7. The eight architect answers below cover the patched anchors, named interfaces, placement, rules, dependencies, independent merges, wave 2, and versions.

### What each registered patch resolves

| Register row | Result of the patch |
|---|---|
| 1. Key decisions | The restore and copy exclusion bounds the one-outbox assumption. The projection, removal and command timing summaries still depend on findings 01, 04, 05 and 07. |
| 2. Rollout and owner points | Step 9 and the epic name the same freeze milestone. Some unchanged summary language still refers to the merge, but the rollout and the amended decision set the operative gate. Point 2 now describes the previously omitted transport consequences without settling the owner choice. |
| 3. Public names | The same committed number is published for repeats. The interrupted outcome and reserved result field retain their numbers. The issuance reply represents an omitted sample address as empty. The streaming refusal carrier and validation precedence remain incomplete. |
| 4. Item 2 | Confirmed live, both cited commits are ancestors of the reviewed branch. The shortened patch applies to the reviewed sources. Scouting's assertion about `master` does not describe those refs in this checkout. |
| 5. Item 4 | No renumbering remains, and reports must be applied to confer readiness. Invalid report precedence, expiry and production of the four website states still need findings 02, 04, 05 and 07. The interrupted outcome and the no-text result have clear wire meanings. |
| 6. Item 5 | The secret check precedes claim and replay. Simultaneous first uses converge for the same key and refuse the other key. The address and pin are expressible. The refusal status definition and the revocation race still need findings 01 and 06. |
| 7. Item 6 | Challenge precedes classification, and a waiting `Welcome` has an empty token. The new post-challenge classes, current stream boundary and removal refusal need findings 01, 03 and 06. Both refusal names appear in the draft and the comment check. |
| 8. Risks | The restore and copy case is explicitly outside wave 1. The plaintext hop remains an owner choice. The reassigned clock and stop behavior leaves the wire timing and finality issues in findings 03, 05 and 06. |
| 9. Agreements | The event, command, approval and token producers and consumers are identifiable. The proxy handoff needs the terminal metadata carrier in finding 03. |
| 10. Post-build checks | Checks 3 through 6 and 10 through 15 expose the intended wire behavior. Checks 3, 6 and 10 need the additional or corrected cases specified in the findings. Check 12 exercises the previously missing secret paths. Check 14 measures the address on the wire, and check 15 constructs the reserved field explicitly. |
| 11. Traceability and attack log | The register outcomes and new decision rows are linked. Earlier notes on local stop behavior are historical under decision 64. The closed rows that depend on findings 01 through 07 need those findings resolved. |
| 12. Draft files | Confirmed live, the patched draft lints, generates in both languages and builds in Go. The wire defects reside in the rules attached to the compiling fields. |
| 13. Decisions | The amended rows and new rows 63 to 68 record the later freeze, wire scope, event numbering, secret check and projection. Rows 65, 67 and 68 carry the unresolved rules covered in the findings. |
| 14. Scouting | Confirmed live, the commits, generators and script anchors resolve on the reviewed branch. By reading, `OpenStore`, `Ack` and `Next` resolve in the client-local plan, and the obsolete `Reconcile` anchor is gone. The retained `MaxReceipts` description reflects earlier scouting. The release script has the refusal-log readers at `deploy/test/checks.sh:335`, `deploy/test/checks.sh:343` and `deploy/test/checks.sh:513`. |

### Public names and their users

| Patched wire name | Writer, reader and intermediary |
|---|---|
| `Event.number`, `EventAck.through` and `Welcome.last_event_number` | Client Link assigns the event number using the local outbox. Server Link reads it, writes both returned marks after a committed event or from the existing committed mark, and Client Link reads both. Server Store performs the transaction on the server side. |
| `SetState`, `MachineState.revision`, `Command.expires_at` and `Welcome.server_time` | Server Link writes the command and timestamp. Server Machines supplies the standing and revision. Client Link reads them, reports the outcome and sends the new facts and harness reports when approved. Server Link consumes those results and reports. |
| `CommandResult.outcome`, `COMMAND_OUTCOME_INTERRUPTED` and reserved field 4 | Client Link writes the result and interrupted outcome. Server Link reads the named fields and discards unknown fields. Server Machines creates the website's display text from the outcome. The report does not consider the reserved field consumed merely because it was reserved. |
| `Restart` and `COMMAND_OUTCOME_DONE` | Server Link sends the request and reads the result. Client Link receives the request, writes the acceptance outcome and opens its next stream from the new process. |
| `CreateClientTokenResponse.address` field 4 and `JoinToken.address` field 3 | Server Machines writes field 4 and its matching value in the token. The sample server can omit field 4. The website data adapter and pairing dialog are the readers of field 4. Server Machines and the server's token command produce token address and pin with Stack Web's endpoint and certificate. Client Enrollment reads them and Client Link uses them for the enrolled connection. The installer relays the token. |
| `Lgtm-Join-Refusal` and its five reasons | Server Machines writes it, Client Enrollment reads it, and Stack Web relays the unary response. The release check consumes the client's matching diagnostic. |
| `Lgtm-Link-Refusal`, `unknown_machine`, `event_gap`, `link_revision_mismatch` and the final `failed_precondition` | Server Link writes the terminal refusal. Client Link reads it. Stack Web needs to preserve the end-of-stream envelope and trailers as applicable. The release check consumes the client's matching diagnostic. Finding 03 identifies the missing contract sentence. |
| `Welcome.connection_token` and `Lgtm-Link-Token` | Server Link writes it, and Client Link reads it. Later client calls present it in `Lgtm-Link-Token`, which those server calls read. An empty value while waiting makes the wave 1 boundary explicit. |
| `Client.state` in `WatchClients` | Server Machines writes the projected state from standing, stream presence and the reports the server has applied. The website's data adapter and Machines screen read it. |

### Interacting rules

| Event input with server mark 42 | Observable result under the proposed ordering |
|---|---|
| The message cannot be decoded, or the decoded number is absent or zero. | The stream ends with `invalid_argument`, and the mark stays at 42. |
| The decoded number is 42 or below and positive. | The server acknowledges through 42 without checking the report or applying it. |
| The decoded number is 43 and the report is known and valid. | The server applies it, commits 43 and acknowledges through 43. |
| The decoded number is 43 and the report is missing, unknown or invalid. | The server commits 43 and acknowledges through 43 without applying the report or setting readiness. |
| The decoded number is 44 or higher. | The stream ends with `failed_precondition` and `event_gap`. The mark stays at 42. |

| First matching `Join` condition | Reply after the round 3 patch |
|---|---|
| The token identifier is unknown or the submitted secret does not match. | The server refuses with `token_unknown`, before inspecting token state. |
| The secret matches and expiry has passed. | The server refuses with `token_expired`, even if the token is revoked or used. |
| The secret matches, expiry is open and the token is revoked. | The server refuses with `token_revoked`. |
| The token is still usable and bound to another key. | The server refuses with `token_used`, including when that key won a concurrent first use. |
| The token is still usable and bound to the same key, and the machine was removed. | The server refuses with `machine_removed`. |
| The token is still usable and bound to the same key, and the machine exists. | The server returns the original reply, including when that key won a concurrent first use. |
| The token is still usable and unused. | One claim creates one waiting machine. A failure before commit leaves it unused and returns `unavailable`. |

| Handshake input after a well-formed `Hello` and `Challenge` | Result if the precedence in finding 06 is adopted |
|---|---|
| Lookup fails. | The server returns `unavailable`. |
| The machine is unknown or removed, with any revision. | The server returns `failed_precondition` with `unknown_machine`. |
| The machine is known and the revision is unsupported. | The server returns `failed_precondition` with `link_revision_mismatch`. |
| The machine and revision are accepted, and `Proof` is late or absent. | The server returns `deadline_exceeded`. |
| `Proof` is on time but breaks a value rule. | The server returns `invalid_argument`. |
| `Proof` passes its value rules but its signature fails. | The server returns `unauthenticated`. |
| `Proof` verifies. | The server sends `Welcome` if admission succeeds and removal has not won the race. |

| Command state | Result the command rules need to specify |
|---|---|
| A completed receipt exists within the collection window. | The client returns the recorded outcome under the newest attempt. Expiry and standing revision change neither the outcome nor its meaning. |
| The command is still running. | The eventual result uses the newest received attempt. |
| There is no receipt for a `SetState` with a newer revision, and the deadline has passed. | The client applies the state and reports its outcome. The server treats this case as more than receipt collection. |
| There is no receipt for a `SetState` with the same or an older revision. | The state stays unchanged. A valid no-op gets the specified successful outcome, and conflicting content at the same revision follows the server-fault rule. |
| There is no receipt for a known ordinary command and the client establishes it can start before expiry. | The client starts it and later sends its result. |
| There is no receipt for an ordinary command and the client cannot establish a timely start. | The client does not start it and reports the deadline outcome defined by finding 05. |

| Standing | Current stream | Valid reports applied from that stream while approved | Website state |
|---|---|---|---|
| Waiting. | Either. | Any. | Waiting. |
| Approved. | No. | No qualifying current reports exist. | Offline. |
| Approved. | Yes. | Fewer than two distinct required report types. | Joining. |
| Approved. | Yes. | Both required types. | Online. |

### Answers to the eight architect questions

1. Confirmed live, both implementation commits are on the reviewed branch and the patch applies. Go 1.26.0 and the pinned tools ran, and the drafts linted, generated in both languages and built. By reading, the website adapter at `src/frontend/src/server/sync.ts:117` currently drops the reply address, and the sample server at `src/mock-server/src/engine.ts:193` omits it, as the compatibility rule allows. The live branch check puts the two implementation commits on `sprint-0`. The scouting claim about `master` is stale in this checkout. `.plans/plans/go-s0-contract/scouting.md:95`, `.plans/plans/go-s0-contract/plan.md:258`, `src/contracts/buf.gen.yaml:4`, `scripts/check-go.sh:16`
2. The public-name table identifies the writers and readers. The new address reaches the website through its adapter, and the terminal reason reaches Client Link through protocol error metadata.
3. The files and wire rules belong to Contract. Eligibility and durable state belong to Server Machines, receipt and timer behavior to Client Link, and listener and relay behavior to Stack Web. This matches the epic's edit boundaries. `.plans/epics/go-rewrite/epic.md:120`
4. Rule 1 is kept by contract placement. Rule 2 is kept by the later freeze before dependent builds. Rules 3 and 4 have no patched contract change. Rule 5 is kept at the shared `Watch` boundary, with the projected status still ambiguous in finding 07. Rule 6 is kept for stored commands and incomplete for expiry in findings 04 and 05. Rule 7 has no patched contract change. Rule 8 is incomplete for terminal errors in findings 03 and 06. Rule 9 is kept by secret-first `Join` and by the no-text result with explicit unknown-field discard. Rule 10 keeps the combined test with real PostgreSQL, with the required observations corrected in the findings. Rule 11 does not apply to the patches. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:358`
5. The plan supplies the shared contract to the server and client, consumes the local outbox only through the Client Link handoff, and relies on Stack Web to carry the named protocol metadata. These are the epic's link and local-state dependencies. Nothing found that changes the test entry-point agreement. Differences in neighboring plans belong to their scheduled alignment pass. `.plans/epics/go-rewrite/epic.md:174`
6. Confirmed live, the existing-files patch matches the current branch and the complete draft generates and builds. By reading, item 4 writes the envelopes, item 5 adds only unary `Join` with its three messages, and item 6 adds the streaming method and imports the earlier envelopes. The round's field definitions add no intermediate build dependency. Separate staged gates for items 3 to 6 were not exercised. `.plans/plans/go-s0-contract/plan.md:329`, `.plans/plans/go-s0-contract/plan.md:464`, `.plans/plans/go-s0-contract/plan.md:520`
7. Nothing found. The patches preserve unknown-kind handling, the empty token for waiting machines, and room for later calls without adding a wave 2 service. `.plans/plans/go-s0-contract/draft/link.proto:31`, `.plans/plans/go-s0-contract/draft/link.proto:92`, `.plans/plans/go-s0-contract/draft/link.proto:104`
8. Nothing found. The patches add no tool, dependency or image. Confirmed live, the reviewed drafts ran with Go 1.26.0, Buf 1.75.0, Connect for Go 1.21.1 and the Go Protocol Buffers generator 1.36.12. `go.mod:3`, `go.mod:10`, `.plans/plans/go-s0-contract/plan.md:169`

## Post-build checks

1. Delay delivery of a server-emitted `Welcome`, then remove the machine, then release the waiting transport. The client observes the ordered `Welcome` and terminal removal refusal. In a second run, hold the server before emission and remove the machine. The client observes only `Challenge` and the terminal refusal.
2. Run a direct link and a link through the stack web server. After `Challenge`, remove a machine and also exercise removal on an established stream. The client reads `unknown_machine` from the terminal error metadata in both cases, and the release script sees the matching diagnostic.
3. On an approved stream at mark 42, send the event cases in the table as isolated fixtures and check the mark, terminal status and readiness. Reconnect a single unchanged client after a lost acknowledgement. The committed prefix resumes without renumbering or duplicate effects.
4. Deliver a first approval `SetState` after expiry on a waiting stream. The client applies the new revision, returns the specified outcome and sends the two reports on that stream. Retransmission of the same state does not undo a higher revision. An ordinary expired command has no effect, and a replay during collection yields its older recorded result.
5. Approve a disconnected machine, open its stream, apply one valid report and then both, and finally close the stream. `WatchClients` shows offline, joining, online and offline in that order, with no transition to online from a malformed report or a superseded stream.

## Grade

5 out of 10. The review found 0 blockers, 7 majors and 0 minors.
