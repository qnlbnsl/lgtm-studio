REVISE

On an ordinary reconnect, the new event rule can assign an unsent report a number already held by a sent report that the server still needs.

## Findings

The reviewed changes run from `fca037e0` to `eb9f2710`. The inspected plan, decisions, drafts and scouting match `eb9f2710`. Evidence is by reading except where explicitly marked confirmed live.

### logic-r3 01 — BLOCKER — C11 — Event resumption assigns the same number to two reports

**Evidence.** The draft rebases every unsent event after the server mark. The outbox keeps existing numbers when the mark is at or below the highest sent number. The draft also says a duplicate is acknowledged at its own number, while the acknowledgement field is documented as the server's committed mark. `.plans/plans/go-s0-contract/plan.md:355`, `.plans/plans/go-s0-contract/draft/events.proto:11`, `.plans/plans/go-s0-contract/draft/events.proto:48`, `.plans/plans/go-s1-client-local/plan.md:664`.

**Failure.** The server has committed through 40, the client marked 41 and 42 sent before its write, the stream dropped before they committed, and the client queued 43. Rebasing 43 directly after 40 assigns it 41, which is still occupied. A client that refuses the collision cannot reconnect, and a client that overwrites it loses a durable report. The collision is confirmed live by arithmetic.

**Fix.** Put the three `Reconcile` branches into the wire rule. A mark below the locally acknowledged mark stops with files intact. From the acknowledged mark through the highest sent mark, discard the confirmed prefix and preserve every remaining number. Above the highest sent mark, discard only sent reports and atomically rebase the unsent suffix. Use a checked ceiling before adding one. For a duplicate, reply with the current committed mark. Add a mixed sent-and-unsent replay case to check 3.

### logic-r3 02 — BLOCKER — plan — The same merge both freezes the link and leaves it open

**Evidence.** The rollout freezes the contract at the sprint merge, while the owner section and decision 57 both allow item 6 to merge with its options open. The epic makes the frozen contract a dependency of later work. `.plans/plans/go-s0-contract/plan.md:105`, `.plans/plans/go-s0-contract/plan.md:113`, `.plans/plans/go-s0-contract/plan.md:715`, `.plans/plans/go-s0-contract/decisions.md:64`, `.plans/epics/go-rewrite/epic.md:114`.

**Failure.** Item 6 passes with the transport choice open. The fallback permits its merge, and items 7 and 8 can finish. The sprint gate then says to freeze at merge even though a pending transport choice can change the contract and the next sprint's routing. A lead has no consistent release gate.

**Fix.** Say explicitly whether item 6 may merge provisionally. Gate the sprint merge and the dependent Server Sign-in and Stack Web work on a recorded ruling for points 1 and 2 and on review of any affected schema or transport. Use that same gate in the rollout, risks and decision 57.

### logic-r3 03 — MAJOR — C10 — The server still sends the retry code for a removed machine

**Evidence.** The contract stops only on `failed_precondition` after a verified challenge. The Server Machines plan and its frozen-test instructions still use `unauthenticated` for a missing or removed machine, use `out_of_range` for a gap, and allow checking the link revision before the challenge. The Client Link plan already uses the new stop code. `.plans/plans/go-s0-contract/plan.md:528`, `.plans/plans/go-s0-contract/plan.md:538`, `.plans/plans/go-s3-server-machines/plan.md:738`, `.plans/plans/go-s3-server-machines/plan.md:768`, `.plans/plans/go-s3-server-machines/plan.md:795`, `.plans/plans/go-s3-server-machines/plan.md:812`, `.plans/plans/go-s3-client-link/plan.md:159`.

**Failure.** Remove an offline machine and start it. The server responds with the planned `unauthenticated` and the client keeps dialing. With the corrected code, the advertised recovery also needs a local path. The client keeps its enrollment, and `Join` refuses an existing identity file. `.plans/plans/go-s0-contract/draft/link.proto:35`, `.plans/plans/go-s3-client-link/plan.md:108`.

**Fix.** Update the Server Machines error table, handshake, event path, removal path and tests before freezing them. Keep `unauthenticated` for bad proof, and deliver the terminal code after the signed challenge for the named cases. For an incompatible revision, give a version-matched recovery instruction. For removal and event divergence, agree on an explicit, owner-initiated re-enrollment path that preserves old state until the new enrollment commits.

### logic-r3 04 — MAJOR — C11 — A malformed report can still block the whole outbox

**Evidence.** The events draft consumes the next report when invalid, and the link header broadly refuses broken value rules. The Server Machines plan explicitly keeps the mark unchanged for an invalid report and tests that outcome. `.plans/plans/go-s0-contract/draft/events.proto:19`, `.plans/plans/go-s0-contract/draft/link.proto:9`, `.plans/plans/go-s0-contract/plan.md:535`, `.plans/plans/go-s3-server-machines/plan.md:924`, `.plans/plans/go-s3-server-machines/plan.md:938`.

**Failure.** An older client has an oversize workspace string at event 41 and valid inventory at 42. Following the server plan leaves the mark at 40 and returns `invalid_argument`. Every reconnect sends 41 first and the inventory never arrives.

**Fix.** Give event handling precedence over the blanket value rule after `Welcome`. Validate the envelope and positive sequence first, then classify duplicate, next or gap. At the exact next number, consume and acknowledge an unusable report in one transaction with a diagnostic, and set neither inventory flag. Amend the server inbox and stream tests to require valid event 42 to apply on that same stream.

### logic-r3 05 — MAJOR — D7 — Removal can pass a verified stream between the database and the live hub

**Evidence.** The fold does hand the write fence to Server Machines. That plan installs the generation in a database transaction, then registers the stream and sends `Welcome` separately. The removal follower ends the handle present when the removal reaches it. The round 3 check pauses B before it becomes current, leaving the publication interval uncovered. `.foundry/go-rewrite/results/fold-go-s0-contract-r2.md:65`, `.plans/plans/go-s0-contract/plan.md:769`, `.plans/plans/go-s3-server-machines/plan.md:700`, `.plans/plans/go-s3-server-machines/plan.md:738`, `.plans/plans/go-s3-server-machines/plan.md:768`, `.plans/plans/go-s3-server-machines/plan.md:1024`.

**Failure.** Pause B after its database installation and before its hub registration. Commit removal and let the follower process it. The follower has no handle for B. B then registers and sends a successful `Welcome` for an already removed machine. A later write can reject the stale generation, but it cannot retract the published welcome.

**Fix.** Keep this decision in Server Machines. Require one order for registration, revocation and publication of `Welcome`. A reservation or invalidation must cover the interval from database admission through registration and welcome publication. Put barriers on both sides of admission in check 10. If removal wins before publication, B gets the terminal removal result and never receives a token in a welcome.

### logic-r3 06 — MAJOR — C5 — The join check treats a refusal exit as success

**Evidence.** The link table defines stop as exit 0 and uses stop for both join refusals. The shell check tests the join command's exit before it looks for the refusal values in its structured log. `.plans/plans/go-s0-contract/draft/link.proto:35`, `.plans/plans/go-s0-contract/plan.md:537`, `deploy/test/checks.sh:217`, `deploy/test/checks.sh:329`, `deploy/test/checks.sh:425`.

**Failure.** Try the used-token fixture with a different key. The server refuses correctly and the client logs `token_used` and exits 0. The shell check fails before reading that log. The exit-status branch is confirmed live with a reduced shell probe.

**Fix.** Give interactive `join` a nonzero exit for a refusal, bad input and exhausted retries. Keep exit 0 for the terminal stop of the supervised `run` command. Name and align the exact diagnostic carrier and the log fields `msg=join_refused` and `code=token_used` or `code=token_expired`. The current server sends a sentence and a separate reason header, and the client reads that header, while the contract fixes the reason as the error message. Pick one carrier and test the CLI exit and the structured log together. `.plans/plans/go-s3-server-machines/plan.md:575`, `.plans/plans/go-s3-client-link/plan.md:109`.

### logic-r3 07 — MAJOR — C6 — Default pairing and the installer have no shared address rule

**Evidence.** The contract allows configured public issuance with a pin or command-only loopback without a pin. The default stack has no public setting, and the live check makes a token from the website in that mode. Server Machines rejects explicit addresses different from its public setting and omits `address` from the reply. The browser adapter currently drops any reply address, and the sample server does not supply one. `.plans/plans/go-s0-contract/plan.md:471`, `.plans/plans/go-s0-contract/plan.md:745`, `.plans/plans/go-s2-stack-web/plan.md:263`, `deploy/test/checks.sh:206`, `.plans/plans/go-s3-server-machines/plan.md:535`, `.plans/plans/go-s3-server-machines/plan.md:1022`, `src/frontend/src/server/sync.ts:117`, `src/mock-server/src/engine.ts:193`.

**Failure.** A default installation cannot issue a website token under either stated context. With public service enabled, the local installer asks for loopback and the documented issuer rejects the address mismatch. A website that reads the newly added field sees an empty address if its server follows the current Server Machines reply plan.

**Fix.** Define the effective website endpoint in default mode, including the actual configured port. Let the website issue a regular loopback token there without the host-machine mark. Allow the installer to use the configured local endpoint in public mode and derive the host mark from its issuance authority. Pass the exact token origin through Server Machines, the browser adapter and dialog. Specify the sample fallback and make the live script read the returned address. Cover both modes in check 14 and send the handoff to all of those owners.

### logic-r3 08 — MAJOR — R6 — A receipt can expire before the server has finished collecting it

**Evidence.** The patch uses an upper estimate of server time for both starting a command and pruning its receipt. The server collects results until 24 hours after expiry. Client Local uses that same clock for pruning, while Client Link already proposes a separate lower estimate and a `PruneNow` option the store does not expose. `.plans/plans/go-s0-contract/plan.md:335`, `.plans/plans/go-s0-contract/plan.md:337`, `.plans/plans/go-s0-contract/plan.md:736`, `.plans/plans/go-s1-client-local/plan.md:735`, `.plans/plans/go-s1-client-local/plan.md:747`, `.plans/plans/go-s3-client-link/plan.md:248`.

**Failure.** The client ran a scan before expiry, but its result never arrived. Near the end of the collection window, the client's estimate is five seconds fast and the server still has three seconds to collect. Pruning deletes the successful receipt. A retry in that interval answers expired for work that completed. The overlapping intervals are confirmed live by arithmetic.

**Fix.** Use an upper bound for admission and a lower bound for retention, under the same explicit clock assumptions. Agree on the Client Local interface and when a sample is valid. A receipt stays readable through the latest server time at which a result can still be accepted. Order server expiry and result acceptance in the same command transaction, and check the disputed interval with a delayed handshake and a lost result.

### logic-r3 09 — MAJOR — R6 — A command can pass its expiry check before it actually starts

**Evidence.** The plan's only expiry test is at receipt admission. The client store commits that receipt before it returns, and a blocking scan slot can further delay dispatch. `.plans/plans/go-s0-contract/plan.md:337`, `.plans/plans/go-s0-contract/plan.md:341`, `.plans/plans/go-s1-client-local/plan.md:736`, `.plans/plans/go-s3-client-link/plan.md:250`.

**Failure.** Admission reads count 101 and an expiry of 103. The receipt commit blocks until count 104. Dispatch then starts work under the durable permission despite the expired command. This counterexample requires neither a clock jump nor skew and was confirmed live by arithmetic. The monotonic estimate also needs a valid elapsed-time assumption across suspend and resume.

**Fix.** Define the start boundary at dispatch. Acquire capacity first and check again immediately before starting work. If the deadline passed while the receipt was being written, finish it as expired without an effect. Bound sample age and specify how the dispatcher invalidates a sample after resume or loss of its elapsed-time guarantee. In check 6, use a comfortably eligible command for wall-clock-skew coverage and add a case held at the receipt commit across expiry.

### logic-r3 10 — MAJOR — C7 — Same-stream approval can expire before it reaches the client

**Evidence.** The new report trigger requires the client to apply approval. The same-stream check depends on `SetState`, which is subject to ordinary command expiry. Server Machines queues approval for one hour. The plan also says joining requires missing reports on the current stream, while check 4 expects B to stay online without requiring that it supply them. `.plans/plans/go-s0-contract/plan.md:338`, `.plans/plans/go-s0-contract/plan.md:362`, `.plans/plans/go-s0-contract/plan.md:368`, `.plans/plans/go-s0-contract/plan.md:763`, `.plans/plans/go-s0-contract/plan.md:772`, `.plans/plans/go-s3-server-machines/plan.md:1047`.

**Failure.** Keep the stream healthy by heartbeats while approval delivery is held past expiry. The server records approval, the client remains waiting, and a server that finalizes the expired command has no prescribed way to deliver the new standing on that stream. No approval reports arrive. If B replaces A, B also correctly starts at joining until it supplies both reports, which can fail check 4 as written.

**Fix.** Give the server a convergence rule for its durable standing revision. If the current stream has yet to apply the latest revision, deliver a fresh state command or renew the stream before declaring the transition complete. Define work eligibility and token authority during the interval between database approval and client application. In check 4, acknowledge both reports on B before asserting it stays online.

### logic-r3 11 — MAJOR — C5 — The replay branch omits the verifier and two race outcomes

**Evidence.** The patched item names the secret check only in the unused-token branch. The service draft's same-key replay rule omits it. Decision 46 and the receiving Server Machines plan both require it. The new conditional-update rule also does not say how a losing identical request is classified, or how revocation's waiting check is serialized against approval. `.plans/plans/go-s0-contract/plan.md:463`, `.plans/plans/go-s0-contract/draft/link.proto:61`, `.plans/plans/go-s0-contract/decisions.md:53`, `.plans/plans/go-s3-server-machines/plan.md:537`, `.plans/plans/go-s3-server-machines/plan.md:565`.

**Failure.** With a used token, an identically named public key and an incorrect token secret, the literal replay branch can return success while the server plan refuses it. Two concurrent identical valid requests can also produce one success and one terminal refusal if the loser never rechecks the binding. If revocation reads a waiting machine and approval commits before its delete, a stale waiting check can remove an approved machine.

**Fix.** Require verifier success for all redemptions. Serialize claim, replay and revocation on the token row. If using a conditional update, reread after a lost claim and apply the same decision table. For the waiting delete, lock or condition the machine transition against approval in the same transaction, with a shared lock order and an explicit winning outcome. Add parallel identical requests and approval-versus-revocation fixtures.

### logic-r3 12 — MAJOR — R9 — A client still builds against the removed result field

**Evidence.** The draft reserves both the number and name of `CommandResult.message`. The handoff names the server's new text writer, while Client Link still tells its builder to set the generated message field. Check 15 asks a typed client to send text in a field the type no longer has. `.plans/plans/go-s0-contract/draft/commands.proto:63`, `.plans/plans/go-s0-contract/plan.md:192`, `.plans/plans/go-s0-contract/plan.md:774`, `.plans/plans/go-s3-client-link/plan.md:252`.

**Failure.** Generate Go from this draft and follow the client mapping. Its result constructor has no `Message` field. If a test writer drops the text fixture to make the test compile, the check never exercises an older sender's unknown field.

**Fix.** Remove the field from the Client Link writer and its fixtures, and keep the website text writer entirely on the server. Specify check 15 with a test-only older descriptor or a constructed unknown field at the reserved tag. Require the current server to discard it before logging, storing or displaying the outcome.

## Answers to the assignment

1. **Patch dispositions.** The table below covers every patch entry in `.plans/plans/go-s0-contract/attack/round-2-findings.md:119`.
2. **Unreviewed mechanisms.** Ruling 1 closes client-authored server progress and introduces the overlap in finding 01. Ruling 2 keeps restoration outside wave 1 and defines a terminal code, with the implementation and recovery gaps in finding 03. Ruling 3 puts the fence in the correct plan, where the publication interval remains open in finding 05. These cover the repeated markers in decisions 54 through 56.
3. **Rejections.** Nothing found. The outcome column contains zero rejected findings. The transport concern is assigned to the owner. `.plans/plans/go-s0-contract/attack/round-2-findings.md:56`, `.plans/plans/go-s0-contract/attack/round-2-findings.md:73`.
4. **Writers and readers.** The event handoff needs the stale Client Local instructions removed. The status table needs the Server Machines, Client enrollment, Stack Client and check-script consumers aligned. The address and result handoffs are incomplete as detailed in findings 07 and 12. The Store, Sign-in and Server Machines ownership is named for the keyed token refusal. The proposed certificate setting still belongs to Stack Web and its server-side reader.
5. **Interacting rules.** The tables below identify the replay, refusal, publication, approval and command cells with conflicts or missing outcomes.
6. **Open round 2 defects.** G2 still leaves a removal race. G3 still leaves the release gate inconsistent. G5 still disagrees with the Server Machines status writer. G6 still permits premature pruning. G7, G8, G9, G11 and G15 still leave a core path or an affected consumer interlocked with an older rule. G1 removes the old server-mark claim and fixes empty-store rebase, with the newly exposed overlap in finding 01. G4's missing-script dependency and G10's keyed-token behavior are addressed.
7. **Lens coverage.** The eight answers below apply the logic lens to the patched surfaces.

### Each of the 17 patches

| Register entry | Disposition by reading |
|---|---|
| 1. Key decisions | Server-only progress, bounded restart and a single presence projection address the old rules. Event overlap, terminal recovery, clocks, approval and the address still fail in findings 01, 03 and 07 through 10. |
| 2. Owner points | The transport cost is recorded for the owner. The new deadline contradicts the merge fallback and rollout in finding 02. |
| 3. Public names | The new shapes and owners are listed. The removed fields, added address, changed refusal codes and local store API still disagree with their consumers in findings 01, 03, 06, 07 and 12. |
| 4. Description format | The inclusive range and reserved name pin the deletion. Nothing found in the format itself. The descriptor gate cannot update a downstream writer. |
| 5. Go generation | The direct commands remove the missing-script gate. Confirmed live that package-scoped execution finds the pinned binary in the contract folder. The guard probe explicitly reuses the untracked probe state, so it needs one clean baseline before the pair. |
| 6. Website fields | The singular position check and address field test cover their descriptors. The keyed refusal precedes issuance in the specified behavior. The address writer and default pairing still fail in finding 07. |
| 7. Commands and events | Removing result text and bounding restart address their immediate failures. Findings 01, 04, 08, 09, 10 and 12 trace the remaining event, approval, clock and result problems. |
| 8. Joining | The server clock, revocation precedence and retry budget address parts of G8 and G9. Findings 06, 07 and 11 record the exit, address and transactional cases still open. |
| 9. Stream and proof | The smaller hello, split proof and registry failures, and substantive comment check address the earlier gaps. The server status writer, bad report path, publication race and approval interplay remain open. |
| 10. README and completion | Both website-session rules are scoped to the website package. The exception names and absent phrases have direct checks. Rows 6 and 8 accurately describe their structural gates. Nothing new found there. |
| 11. Risks | The restore exclusion and owner transport question are correctly retained. The clock, publication and deadline rows repeat the guarantees whose counterexamples are in findings 02, 05, 08 and 09. |
| 12. Cross-team agreements | Server Store and Sign-in are named for the request-key exception. The assignments for the other changed names need the receiving plans amended as described in answer 4. |
| 13. Post-build checks | Check 3 covers mixed backlog in its setup but needs explicit numbers in the interrupted case. Checks 5 and 10 through 15 add coverage, with missing assertions or incompatible fixtures in findings 03, 05, 07 through 10 and 12. In check 14, call keyed `CreateClientToken` separately from the `Join` retry. |
| 14. Traceability and attack log | The rulings and finding counts are carried forward. The links establish ownership and source coverage. Findings 01 through 12 identify where claiming the earlier behavior resolved is premature. |
| 15. Draft files | Confirmed live that the revised existing-files diff applies cleanly. Its intended wire changes are consistent with the descriptor list. The draft comments retain the event, refusal, clock and address conflicts in the findings. |
| 16. Decision rows | Rows 54 through 62 record the new choices. Rows 38, 45, 47 and 49 still read as active rules about the old gap code, revision validation, address and report trigger. Mark the superseding rows on those entries, and align row 57 with the release gate. |
| 17. Scouting | The corrected `Ack` and `Reconcile` anchors match the local plan's described branches. Run 25 correctly identifies the log values but omits the shell exit requirement in finding 06. The reserved-range output is specified by the plan, while run 23 says its prototype did not exercise it. |

### Logic lens questions

1. **Ordering.** Commit orders the server mark. An ordinary acknowledgement is limited by the sent prefix. The rebase branches and initial zero mark are in the replay table. Exhaustion needs a shared checked ceiling. Wall time orders token expiry, while receipt time and actual dispatch diverge in findings 08 and 09.
2. **Failure and repair.** A failed event commit leaves the prefix unchanged, an unknown or broken next report needs a durable consume without an effect, and an unavailable database calls for retry. A regressed mark or terminal registry refusal stops and preserves local files. The join exit, failed approval delivery and owner-initiated re-enrollment need the repairs in findings 03, 06 and 10.
3. **Truth tables.** The tables below identify the conflicting and empty cells using the client marks, the server status, the machine standing and the command lifecycle.
4. **Zero values.** Zero is the empty server mark and an invalid event number. The initial attempt and standing revision are 1. Missing standing is invalid. Unspecified approval yields waiting and an unspecified outcome becomes failed. A zero hello revision is unsupported after challenge. The client-side action when a welcome reports an unexpected revision needs the same explicit version policy as the server path. An empty response address needs a defined sample fallback.
5. **Concurrent writers.** Only the server commits the event mark. The client outbox transaction serializes append and reconciliation. Stream B, removal and follower publication need one order, as do redeem, revoke and approval. Findings 05 and 11 give the interleavings.
6. **Risky mutations.** For item 3 the shape test detects removal of position and the common descriptor detects the narrower scalar. For item 4 the golden descriptors detect the wider attempt and missing revision. For item 5 the token descriptor and example detect the new field number and wrong prefix. For item 6 the proof example detects nonce reversal, the welcome descriptor detects the missing mark, and the comment check detects a missing or bare comment. All four items can still accept incorrect comments on their behavior. The planned behaviors that survive are listed in the next answer.
7. **Cheapest passing defects by item.** Item 1 could require Go 1.26.1 and still pass on a machine with that version cached. Item 2 could rely on an uncommitted generated file whose name is ignored by Git, which its visible-file probe misses. Item 3 can omit the token-address meaning while its descriptor stays correct. Item 4 can keep every field and give an unsent report the number of an unconfirmed sent report. Item 5 can omit the replay verifier while passing both format tests. Item 6 can supply any three-word comment and retain a wrong status rule. Item 7 can skip vet and still catch the formatting and module probes. Item 8 can mention the expected words without writing the scoped session rule.
8. **Representing safe states.** Use a transaction to tie an event's effect to its mark, and a single generation authority to tie stream validity to writes and publication. Use separate time bounds for command admission and retention. A credential-issuing path outside receipt persistence makes plaintext receipt storage unrepresentable.

### Replay, refusal, approval and command truth tables

Let A be the locally acknowledged number, S the locally marked-sent number and W the server's committed number in `Welcome`.

| Replay input | Required result | Conflicting or empty cell |
|---|---|---|
| W is below A | Stop, report and retain the outbox. | The older Client Local instructions still retry. `.plans/plans/go-s1-client-local/plan.md:837`. |
| A is at most W, and W is below S | Discard the confirmed prefix and preserve the sent and unsent suffix. | The draft can renumber an unsent report on top of a pending sent one. |
| W equals S | Discard the confirmed prefix and leave the next unsent number in place. | Both descriptions give the same numbers. |
| W is above S | Rebase unsent reports after W, atomically and below the ceiling. | The empty-store case is covered by the local plan. The wire plan needs an explicit exhaustion outcome. |
| Positive incoming number is at or below the mark | Treat it as a duplicate and emit the committed mark. | The draft also says to emit the duplicate's own number. |
| Incoming number is next and the report is valid | Commit the effect and advance the mark, then acknowledge. | No conflict on an approved machine. The owner's waiting policy governs the effect for a waiting machine. |
| Incoming number is next and the report is unusable | Commit the mark without the report effect, then acknowledge. | The server plan rejects a known invalid report and leaves the mark unchanged. |
| Incoming number is above next | Fail without changing the mark. | The two plans give different terminal codes. |
| Incoming number is zero | Refuse the envelope as invalid. | Check the positive number before treating anything as a duplicate. |

| Stream or join input | Required or missing result |
|---|---|
| No verified challenge, or failed proof after a verified challenge | Retry. A bare terminal code has no terminal effect. Bad proof is `unauthenticated`. |
| Verified challenge and missing identity, unsupported revision, or event gap | Stop on the terminal code. The current Server Machines plan disagrees in all three cases. The recovery instruction depends on the reason. |
| Failed send with a verified challenge and a pending terminal status | Read the terminal status within the handshake budget and then stop. A database read failure instead returns `unavailable` and retries. |
| Removal precedes admission, or falls between admission and hub publication | Both paths need terminal refusal before a welcome can be published. The second interval is unaccounted for. |
| Invalid secret, expired or revoked token, other bound key, or removed machine | Refuse, with the precedence fixed in one place. The verifier must apply to the replay branch. |
| Eligible unused token, same-key eligible replay, or two identical first requests | Create once, replay once, and classify the losing concurrent request from the committed binding. The last cell lacks a contract rule. |
| Revocation races with approval of a waiting machine | Revocation first removes it, or approval first keeps it. A stale waiting read is insufficient to choose the outcome. |

| Standing, receipt or delivery state | Required or missing result |
|---|---|
| Waiting with or without a stream | Show waiting. Apply the owner's report policy and permit only state commands. |
| Approved without a stream | Show offline. The absence of a current stream is explicit. |
| Approved with a stream and fewer than two usable reports | Show joining. If the approval command expired, the plan needs a repair trigger. |
| Approved with two usable reports on the current stream | Show online. Replacement returns to joining until the new stream supplies both reports. |
| No receipt, with admissible time at dispatch | Start once after the durable admission. A receipt may commit after the deadline and needs a dispatch check. |
| No receipt at or beyond expiry | Record expiry and perform no work. |
| A running or final receipt, before or after expiry | Preserve the running work or replay the final outcome under the newest attempt. Pruning cannot erase a result the server can still collect. |

## Post-build checks

- Use a real link and a blocked receipt commit to hold a command across expiry. Observe zero action starts and an expired result. Repeat with a lost completed result near the retention boundary and observe the original result on retry.
- Cut the stream after the restart receipt commits and before sending the result. Observe one exit with status 75, one supervised restart and one replay of the saved outcome. Run this under both supported service managers.
- Hold a verified stream on either side of database installation while another request removes the machine. Observe a single durable outcome, no late welcome for the removed identity, and no extra machine change in the website watch.
- Run the real pairing flow with the default loopback configuration and with public mode plus the installer. Observe an exact match between the reply origin and token origin, the host mark only for the authorized installer request, and the expected nonzero exit and reason on an expired token.
- On the same healthy stream, approve the machine, withhold the first state update through its deadline, then allow delivery. Observe recovery to online after both usable reports land.

## Grade

4 out of 10. There are 2 blockers, 10 majors and 0 minors.
