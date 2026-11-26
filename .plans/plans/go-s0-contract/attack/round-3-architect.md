REVISE

The patched `Join` rules omit the secret check on replay, and the draft wire contract also omits it on first use.

## Findings

### architect-r3 01 · BLOCKER · D7 · Require the token secret on both paths

By reading, the new first-use rule mentions the verifier only in the plan, and the replay rule checks only the token's state and the public key. The previous draft required a verified secret on both paths. The approved design authorizes the exchange with the token, and the Server Machines plan already checks the verifier before either path. Evidence is `.plans/plans/go-s0-contract/plan.md:463–465`, `.plans/plans/go-s0-contract/draft/link.proto:59–75`, `.plans/plans/go-s0-contract/decisions.md:53`, `.plans/plans/go-s3-server-machines/plan.md:565–572` and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:247–255`.

A first use with a known token identifier and the wrong 32-byte secret has no prescribed refusal in the draft. A replay using the bound public key and the wrong secret passes every listed replay check. A builder following the draft can consume the owner's invitation without proof of its secret or return a prior enrollment reply to an unauthenticated caller.

Require a valid secret before inspecting token state or accepting either path. Use the same refusal for an unknown identifier and a wrong secret. Decide whether a concurrent retry with the same authenticated key replays after the winning commit, and require a different key to receive `token_used`. Carry the order into the comments, decisions and the Server Machines tests.

Category is authorization and data integrity. Contract and Server Machines own the fix. Risk is 150 (6 for a join the owner has to repair, 5 for coordinated recovery, and 5 for a caller that can obtain the identifier under the stated rules).

### architect-r3 02 · MAJOR · C5 · Give both token producers an address that actually serves the link

By reading, the new rule reserves plain loopback tokens for the server command and requires the browser to use the public address. That setting is empty by default. With the setting enabled, Stack Web serves Transport Layer Security (TLS) only on the published port, and Stack Client dials plain Hypertext Transfer Protocol (HTTP) at that same port. Server Machines also currently rejects an explicit token address that differs from its configured origin. Evidence is `.plans/plans/go-s0-contract/plan.md:469–471,746,773`, `.plans/plans/go-s2-stack-web/plan.md:263–268,500–507`, `.plans/plans/go-s3-stack-client/plan.md:219–225` and `.plans/plans/go-s3-server-machines/plan.md:535,1022`.

On the default stack, the signed-in owner cannot create the token required by check 5 and the later machine checks. In public Hypertext Transfer Protocol Secure (HTTPS) mode, even if the command mints the installer token, its advertised plaintext endpoint cannot carry `Join`. The current installer also preflights that address before asking for a token. The frozen script builds all its machine fixtures using the website's loopback endpoint. Evidence is `deploy/test/checks.sh:32–39,206–229,425–449` and `.plans/plans/go-s3-stack-client/plan.md:219–225`.

Specify a reachable setting and listener for each cell in the issuance table below. Allow browser-issued loopback tokens in default mode without the host mark. For public mode, either use the same pinned HTTPS endpoint for the installer or assign a separate loopback listener and port to Stack Web. Put the host mark on the privileged issuance path and state how `--hosts-server` interacts with it. Name who provides the published port, certificate file, pin and normalized address to both issuers. Add the default stack and a nondefault port to check 14.

Category is deployment and contract design. Contract, Stack Web, Server Sign-in, Server Machines and Stack Client own the fix. Risk is 300 (6 for failed enrollment, 5 for repair across teams, and 10 for the normal default-stack flow).

### architect-r3 03 · MAJOR · C5 · Agree on how a refused enrollment reaches the release check

By reading, the draft defines a token refusal word as the Connect error message and applies exit 0 to every stop in its status table. Server Machines writes a sentence and the `Lgtm-Join-Refusal` header instead, and Client Link reads that header and maps a failed `join` to exit 1. The release check requires both a nonzero exit and a structured record with `msg=join_refused` and `code=token_used` or `token_expired`. Evidence is `.plans/plans/go-s0-contract/draft/link.proto:35–43,59–65`, `.plans/plans/go-s3-server-machines/plan.md:414–425,575`, `.plans/plans/go-s3-client-link/plan.md:109,264,276` and `deploy/test/checks.sh:329–340`.

A used token can receive the correct refusal and still fail check 5. Exit 0 enters the script's success branch. A nonzero exit with the word only in the status message gives the current Client Link plan no classified reason to log.

Give `join` success exit 0 and failed enrollment a nonzero exit. Scope exit 0 on terminal refusal to the supervised `run` command. Pick one wire source and exact set of refusal reasons, keep the user-facing sentence separate if using metadata, and name the server writer, client reader, proxy and script. Correct the test that currently describes the keyed website call as a keyed `Join`.

Category is wire compatibility and release verification. Contract, Server Machines, Client Enrollment and Stack own the fix. This is a failing release gate.

### architect-r3 04 · MAJOR · D7 · Carry the terminal refusal through the server and the proxy

By reading, the handoff assigns the removal fence to Server Machines, and that plan describes transactional checks for current-stream writes. Its handshake still refuses a revision before `Challenge`, and its handshake and removal follower use `unauthenticated` for a removed machine. The new contract and Client Link retry precisely that code. The client also expects a named reason for check 10, which reads `link_refused` with `unknown_machine`. Evidence is `.plans/plans/go-s0-contract/plan.md:528–540,547–555,738–740`, `.plans/plans/go-s3-server-machines/plan.md:700–706,738,777,1023–1024`, `.plans/plans/go-s3-client-link/plan.md:159,199` and `deploy/test/checks.sh:505–514`.

A removed machine reconnects and is refused. Built from the current server plan, the refusal is retryable and check 10 sees no required reason. An unsupported revision also retries if the server terminates before sending the challenge. There is a second ordering case after the durable current-stream write and before the handler registers its stream with the in-memory hub. A removal can pass through the follower before registration, leaving a stale handler free to send `Welcome` even though its authority is gone.

Define the public boundary at which a stream is current. Require the handoff to handle removal between the durable transition and hub registration as well as the two races already in check 10. Publish the same reason contract for live removal and a later dial, and have the client select fixed local diagnostics. An unsupported revision needs an upgrade diagnostic because enrollment does not change the revision. If owner point 2 chooses message authentication, cover terminal statuses explicitly because they are outside the message envelopes. Keep the check for transient errors while the database is unavailable.

Category is authorization, interoperability and availability. Contract, Server Machines, Server Link, Client Link and Stack Web own the fix. Risk is 120 (6 for machines that retry permanently or get stopped incorrectly, 5 for manual recovery, and 4 for removal, incompatibility or the ordering fault).

### architect-r3 05 · MAJOR · C11 · Define the committed prefix identically in both link plans

By reading, the new event draft says to acknowledge the duplicate's number, while its `EventAck.through` field says to acknowledge the server's current committed prefix. Item 4 consumes a malformed report at the next number, but the Server Machines item returns `invalid_argument` and leaves the mark in place. The blanket link value rule also promises `invalid_argument`. The parallel Client Local fold now removes the `Reconcile` interface entirely, so the Contract and Client Link call sites still disagree with the available store interface. Evidence is `.plans/plans/go-s0-contract/plan.md:355–361,535,733–734`, `.plans/plans/go-s0-contract/draft/events.proto:7–24,48–54`, `.plans/plans/go-s3-server-machines/plan.md:795,919–939`, `.plans/plans/go-s1-client-local/plan.md:405–410,417–419,497–501,599–600` and `.plans/plans/go-s3-client-link/plan.md:157,172–173`.

At committed mark 42, a repeated event 40 has two specified acknowledgements. With a decodable but invalid event at 43 followed by a valid report at 44, the server plan rejects 43 without moving the mark. The client retries and starts again at 43, so 44 never arrives. On a replacement store with locally numbered reports 1 and 2 and a server mark of 40, the contract expects both reports to move to 41 and 42. The Client Local fold calls `Ack(40)` and stops. Client Link still calls a method the folded plan has removed.

Define `EventAck.through` as the committed prefix in every case, including a duplicate. Apply separate checks to the envelope, sequence number and decoded report body, in the order shown in the event table below. Exempt the consumed-body case from the blanket value rule. Keep consumed reports separate from applied report effects in the projection and the log. Bring both downstream plans and their tests into the same closure. Settle the higher-mark and regressed-mark cases explicitly against the store and the new ruling about restores. Keep the server-only commit authority, retain buffered reports until the coordinated recovery action, and keep the per-stream sent ceiling in the sender.

Category is protocol design and availability. Contract, Server Link, Client Local and Client Link own the fix. Risk is 120 (6 for a queue that cannot drain, 5 for a state recovery, and 4 for an invalid durable payload).

### architect-r3 06 · MAJOR · R6 · Keep receipts through the real collection window

By reading, the patched rule gives `Begin` and pruning the same deliberately fast clock and uses the same 24-hour boundary on both sides. It also claims a single handshake sample bounds that clock for the life of a stream. The parallel Client Local fold gives `Begin` an explicit time argument and replaces time-based pruning with a limit of 1,000 finished receipts. Client Link still depends on the removed `Store.Prune` method and the old signatures. Evidence is `.plans/plans/go-s0-contract/plan.md:335–341,709,736–737,765,770`, `.plans/plans/go-s0-contract/draft/commands.proto:6–23`, `.plans/plans/go-s1-client-local/plan.md:405–410,420–423,503–505` and `.plans/plans/go-s3-client-link/plan.md:245–252`.

With an eight-second lead, the client can delete a completed scan's receipt five seconds before the server's collection deadline. The server can then accept `EXPIRED` for a command that already ran. Under the Client Local count rule, a command can also run twice if enough newer results evict its receipt before its expiry and a reconnect brings it back. Separately, even without a wall-clock jump, different clock rates can exhaust the initial offset. A client monotonic clock running 100 parts per million slower falls another 17.28 seconds behind over 48 hours, which can make the client start a command late. The clock rule has no maximum sample age or rate assumption.

Agree on one receipt interface and one eviction rule across the three plans. Retain replayable receipts through the agreed server collection deadline, or make the server stop retries before the client can evict them. If a count bound remains, specify what happens at capacity while a receipt is still needed. For time-based eviction use a lower bound on server time or retain through the deadline plus the full stated uncertainty. Use an upper bound for execution admission with a stated maximum sample age and drift or a refreshed sample. Define the suspension boundary too. Correct the scouting anchor from `OpenReceipts` to `OpenStore`. Align checks 6 and 11 with the early-refusal policy, and check the collection cutoff with a completed result that the server has yet to receive.

Category is command lifecycle and data integrity. Contract, Client Local, Client Link and Server Link own the fix. Risk is 96 (6 for an incorrect final outcome or late command, 4 for recovery or investigation, and 4 for a delayed result or long-lived session).

### architect-r3 07 · MAJOR · C7 · Set readiness from committed reports and cover the replacement transition

By reading, the replacement projection says the stream has delivered both reports, while the new consumption rule can acknowledge both reports without applying either. The server plan already sets readiness flags when it applies the matching report and clears them on replacement. Post-build check 4 still expects the machine to stay online after the replacement's `Welcome`, with no reports specified for the new stream. Evidence is `.plans/plans/go-s0-contract/plan.md:361–372,741,763,772`, `.plans/plans/go-s0-contract/draft/link.proto:103–112` and `.plans/plans/go-s3-server-machines/plan.md:700,925,992–996`.

An approved machine can send an unusable pair of reports and receive both acknowledgements. One implementation counts delivery and publishes online without the promised facts, and another stays joining. Alternatively, freeze stream B just after `Welcome` while its reports are blocked. Check 4 then requires online where the replacement projection requires joining.

Set the readiness flags only for valid reports applied on a current, approved stream. Reset the flags on stream replacement and explicitly choose how pre-approval reports relate to them. Cover an approved live stream with a held report, two committed reports, a late close from the old stream and finally a timeout on the current stream. Supersede decision row 43 explicitly with row 60.

Category is state projection and verification. Contract and Server Machines own the fix with Client Link. Risk is 90 (6 for a misleading or stuck status, 3 for a fresh report or reconnect, and 5 for enrollment or reconnection with unusable reports).

### architect-r3 08 · MAJOR · R9 · Remove the old result field from its writers, readers and tests

Confirmed live, the generated Go type has no `CommandResult.message` after the draft is applied in a scratch copy. By reading, Client Link still fills it, and Server Machines still validates it, stores it in `link_commands.message` and tests its byte limit. The changed agreement names only the server's website text writer. Evidence is `.plans/plans/go-s0-contract/plan.md:192,347–348,735`, `.plans/plans/go-s0-contract/draft/commands.proto:63–72`, `.plans/plans/go-s3-client-link/plan.md:251–252` and `.plans/plans/go-s3-server-machines/plan.md:885,893,905`.

Sprint 3 builders following both plans will reference a field the Sprint 0 generator cannot produce. Leaving a passthrough somewhere to make the old tests pass would also undo the outcome-only decision.

Update the Client Link writer, Server Link reader, database result schema and test fixtures together. Keep the reservation and have the server derive its own display text from the committed outcome. Replace the length-limit test with a check that unknown reserved-field content has no effect on the stored outcome or any display text.

Category is build compatibility and data handling. Client Link and Server Machines own the fix with Contract. This is a failing build gate.

### architect-r3 09 · MAJOR · D17 · Use one freeze gate before a dependent build

By reading, the patch advances the deadline for owner points 1 and 2, permits item 6 to merge while they are open, and leaves the rollout saying that the whole sprint merge freezes the contract. The epic also declares it frozen after sprint 0. The text explicitly returns a chosen change to phase 3, but does not name the hold when an option has yet to be chosen. Option B for transport also leaves the address or listener routing discriminator open. Evidence is `.plans/plans/go-s0-contract/plan.md:44–50,97–117,124–136,715,842,864`, `.plans/plans/go-s0-contract/decisions.md:64` and `.plans/epics/go-rewrite/epic.md:114–127,172–180`.

If the sprint merges with points 1 and 2 open, the Stack Web team can read the rollout and epic as permission to build its TLS-terminating route. A later choice of pass-through changes that route and the server listener. A later schema choice invalidates a previously frozen descriptor or proof fixture.

Distinguish an item branch merge from the named contract freeze, and make the freeze and the first dependent build explicit gates. Name the owner decision, the responsible plans, and the tests to rewrite if the selected option changes fields, proof bytes, transport or routing. If the transport choice is pass-through, settle how the proxy distinguishes the encrypted link from website traffic. Carry any exception to the epic's freeze milestone into the epic and both sprint 2 consumers.

Category is release sequencing. The orchestrator owns the gate with Contract, Stack Web and Server Sign-in. This is a release gate.

### architect-r3 10 · MAJOR · C6 · Define which producer fills the new address and which adapter carries it

Confirmed live, the pinned generator emits `address: string` for `CreateClientTokenResponse.address`. By reading, the existing sample server omits the scalar, the frontend's `ClientToken` adapter drops it, and the dialog composes the command from a display label. The Server Machines plan still lists only the old response fields. The agreement names the sample server as a reader and gives no compatibility rule for its omitted address. Evidence is `.plans/plans/go-s0-contract/plan.md:193,292,304,745–748`, `.plans/plans/go-s0-contract/draft/existing-files.diff:59–71`, `.plans/plans/go-s3-server-machines/plan.md:1022,1030`, `src/mock-server/src/engine.ts:182–193`, `src/frontend/src/server/sync.ts:117–120` and `src/frontend/src/screens/service.tsx:367–410`.

A shared dialog switched to the new address receives an empty string from the current sample implementation. Against Go, updating the dialog without the adapter and server writer gives the same outcome. Keeping the existing display label gives the copied command the wrong origin when the website was reached by another name.

Assign the Go writer, the website adapter and the command renderer. Assign the sample server a fixture address or have the website use a documented sample-only fallback for an absent field. Keep the sample's token text opaque. Add end-to-end assertions for the rendered and copied command on sample and Go, using an alias in the Go case.

Category is deployment compatibility. The orchestrator, Server Machines, the website and the sample server owners own the fix. Risk is 180 (6 for a copied command that cannot work, 3 for recovery, and 10 when the common dialog reads an omitted scalar).

### architect-r3 11 · MAJOR · D16 · Keep the approved reconnect bound after replacement

By reading, the new `aborted` rule waits 60 seconds after a second replacement within a minute. The approved design requires a retry at least every 15 seconds, and Client Link explicitly keeps that bound for replacements. Evidence is `.plans/plans/go-s0-contract/plan.md:539,740`, `.plans/plans/go-s0-contract/draft/link.proto:49–51`, `.plans/plans/go-s3-client-link/plan.md:199` and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:41,310–313`.

The machine loses two streams within one minute, and the competing process then ends. The healthy client waits another minute although the server is ready, so it can miss the server-restart check's 35-second observation window. Evidence is `deploy/test/checks.sh:483–502`.

Keep retries within the approved 15 seconds and emit the duplicate-process diagnostic separately. A longer pause requires an explicit change to the owner decision and the check timings.

Category is recovery policy. Contract and Client Link own the fix. Risk is 48 (4 for a temporary outage, 3 for automatic recovery, and 4 for repeated replacement).

### architect-r3 12 · MINOR · plan · Give the new checks runners that can observe their outcomes

By reading, the plan assigns all post-build checks to the combined in-process link suite. Check 12 needs real process exit and service restart on two operating systems, check 14 needs actual stack mode and a browser copy action, and check 15 asks a generated client to send a field removed by the schema. The public Client Link test entry returns an integer from an in-process call and exposes no service or process control. Evidence is `.plans/plans/go-s0-contract/plan.md:756,769–774`, `.plans/plans/go-s3-client-link/plan.md:277` and `.plans/plans/go-s3-stack-client/plan.md:185–191`.

An orchestrator can pass the combined Go test with no service startup on either platform. The check still cannot demonstrate the promised process restart. A generated client also cannot construct the removed string through its generated interface, so the negative test can pass without sending anything.

Keep in-process wire and database checks in `src/linktest/`. Assign real supervisor and stack checks to deploy and the copy action to the website flow. Give check 10 an observable test gate before the database transition and after it, using the Server Machines test seams or an equivalent controlled fixture. For check 15, use an older descriptor or fixture encoding and confirm the current reader discards the reserved field.

Category is verification. The orchestrator, Stack Client, Stack Web, Server Machines and website test owners own the fix. This is a verification gate.

## Answers to the assignment's seven questions

1. Every patch site in the register was checked below. The server-only mark, request-key refusal, status 75 for restart and the item 2 script dependency are sound in isolation. The remaining failures are in the decisions and interfaces that use them.
2. All marked mechanisms were attacked. Both event markers and decision 54 are covered by finding 05 and the event table. The terminal handshake and decision 55 are covered by finding 04 and the status table. The stream ordering marker and decision 56 are covered by findings 04 and 12. A restored database is outside wave 1. For a regressed server mark, the Contract and Client Link texts require a terminal local stop without changing either mark. The parallel Client Local fold has removed the method that exposes that regression. Restarting a client manually with a preserved identity is outside the protection provided by exit 0 alone, as the fold result already records.
3. Nothing found. The register records zero rejected findings at `.plans/plans/go-s0-contract/attack/round-2-findings.md:73–80`. I checked the one owner referral separately. The unprotected hop remains dependent on the transport decision.
4. The public-name handoff below lists producers and consumers. It identifies missing owners or conflicting assumptions for result payload, address, refusal reasons, exits, stream authority and clock.
5. The issuance, event and status tables below show the conflicting combinations. The command expiry and pruning boundary is calculated in finding 06. The presence precedence is waiting first, then approved with no current stream, then approved with a current stream and zero or one valid committed report, then approved with both.
6. Round 2's owner deadline, effective terminal refusal, command clock, presence readiness, installer address and website address remain open in the patched plan or its handoffs. The event mark is no longer client-controlled, and the dependent plans still disagree about reporting and recovery. The fence has an assigned server owner and a remaining registration case. The Go check dependency and the keyed secret receipt are closed in this slice.
7. All eight architect questions are answered below, with the patch-specific source checks, producers and consumers, placement, design rules, neighbouring plans, merge gates, wave 2 and dependency versions.

### Each patch in the register

| Patch | What the new text resolves and what it still breaks |
|---|---|
| 1. Key decisions | Server-only event authority resolves the original claim that a client could raise the mark. Applying the new stop, command clock, token destinations and readiness rules exposes findings 02 and 04–07. |
| 2. Owner points | The earlier deadline and the second transport cost are recorded. Finding 09 covers the conflicting freeze milestones and the missing dependent-build hold. |
| 3. Public names | The field numbers and outcome reservation are specified. The writers and readers still disagree in findings 03–05, 08 and 10. |
| 4. Description format | The inclusive singleton range and its reserved name describe the draft for field 4. Nothing found in the reservation format. The downstream type break is finding 08. |
| 5. Item 2 | The direct Go commands remove the required-script cycle. The workspace executable resolves and the clean checkout has an install and removal step. No other patch conflict found in the dependency gate. |
| 6. Item 3 | Both field 4 additions are in distinct messages. Singular `position` is checked structurally. Sign-in and Server Machines agree on the keyed credential exception. The website and sample address gap remains under finding 10. |
| 7. Item 4 | Approval on either an approved `Welcome` or `SetState` now triggers both reports. The invalid-report, retention, readiness and removed-result interfaces need findings 05–08. |
| 8. Item 5 | Revoked or expired tokens take precedence over replay, and the client can stop waiting without an expiry field. The secret check, reachable token addresses and refusal representation need findings 01–03. |
| 9. Item 6 | The removed field is absent from `Hello`. An absent `Welcome.state` is invalid in the draft, and the comment mutation can fail by construction. The server's terminal codes, stream registration, rejected report policy and 60-second replacement wait disagree in findings 04, 05 and 11. |
| 10. Item 8 and completion | The README's two session rules name the website package, keep the two session exceptions and add the link's own authorization. The `grep -q` absence checks have the stated status. Completion row 8 now allows the direct commands before item 7. Nothing further found in this patch. |
| 11. Risks | The transport decision retains the named status risk, and the restore exclusion is recorded. The retention, drift and cutoff assumptions underlying the clock and terminal rules need findings 04, 06 and 09. |
| 12. Agreements | The patch assigns clock, report, removal and token work to named teams. The Client Local fold and the Contract now disagree on the store interface, and the result-field, address, refusal, settings and supervisor contracts need the updates in findings 02–06, 08 and 10–11. |
| 13. Post-build checks | The new scenarios cover the requested races and failures on paper. Findings 03–04, 06–07 and 12 identify conflicting expectations, missing observables and runner boundaries. |
| 14. Traceability and attack log | The ruling counts and links match the register. The claim of coverage depends on reconciling earlier decision rows and the live handoffs, especially the freeze and readiness rules in findings 07 and 09. |
| 15. Draft files | The full draft patch applies to the pre-item-2 sources and generates. Item 3 needs only its remaining hunks after item 2 adds the package options. The token verifier, event acknowledgement wording and terminal behavior are still inconsistent. The reserved result field and new address were confirmed in generated output. See findings 01, 03–05, 08 and 10. |
| 16. Decision rows | Rows 54–62 record the new decisions, and row 39 points to the removed field's replacement. The old rules for approval, event gaps and token addresses can still be read independently of their replacements. Mark rows 38, 42, 43, 46, 47, 49 and 52 superseded or amended where the new decisions change them. See findings 01, 02, 04, 05, 07 and 09. |
| 17. Scouting | The `Ack` and `Reconcile` summary described the round 2 client-local plan. The parallel fold removes `Reconcile`, the persistent sent mark and `Prune`. The `OpenReceipts` anchor still does not resolve. The store opens in `OpenStore`, and the caller now passes time explicitly to `Begin`. The tool and refusal-log checks were rechecked below. |

### Public names and their producers and consumers

| Interface | Producer and consumer at the patched boundary |
|---|---|
| `Hello.acked_event_number` removed | Client Link no longer sends it, Server Link never reads it, and the parallel Client Local fold removes the old call-site instruction. |
| `Welcome.last_event_number` field 3 and `EventAck.through` field 1 | Server Machines stores the transactional mark through Server Store. Server Link publishes it. Client Link sends the first to `Reconcile` and the second to `Ack`. The parallel Client Local fold offers only `Ack` and `Next` for this check and expects terminal refusal on a mark above the local range. The sender must still enforce its current-stream ceiling. Stack Client consumes the terminal exit. |
| `CreateClientTokenResponse.address` field 4 and `JoinToken.address` field 3 | Server Machines provides the normalized address in both places. The server command issues the other token context. Stack Web writes the externally usable endpoint, and Server Sign-in loads the settings. The website adapter, dialog, installer and Client Enrollment use the address. The sample producer needs an agreed value or fallback. |
| `LGTM_PUBLIC_URL`, `LGTM_WEB_PORT` and `LGTM_TLS_CERT_FILE` | Stack Web publishes the address, port and public certificate. Server Sign-in's current plan omits the TLS setting until sprint 3 at line 400. Server Machines plans to read it at line 1076. The privileged command and the website issuer must use the same source for their chosen endpoints and pins. Client Enrollment writes the verified address and pin to `identity.json`, and Client Link reads them on startup. |
| `JoinRequest.token_secret` field 2 and refusal reasons | Client Enrollment writes the secret. Server Machines checks it for both first use and replay and writes the refusal. Stack Web passes the response through. Client Enrollment maps the agreed reason to a fixed log and exit. The installer reads the exit and check 5 reads the exit and log. |
| `Lgtm-Request-Key` on `CreateClientToken` | The website or another web caller can supply it. Server Machines must reject presence before storing anything. Server Sign-in routes the call, and Server Store must never receive the secret reply as a receipt. The installer command bypasses the web method. |
| `failed_precondition`, `unauthenticated`, `aborted` and refusal reasons | Server Link and Server Machines write the statuses. Stack Web passes them through. Client Link turns them into retry or exit 0, Stack Client observes the exit, and check 10 consumes a stable log reason. |
| `Welcome.state`, `SetState` and `Welcome.connection_token` | Server Machines writes standing and removal. Server Link sends it and owns the token's admission and invalidation. Client Link applies standing before reports and commands. Future side calls read the token, so state whether an initially empty token means reconnect on approval. Current Server Machines issues no token to a waiting stream. |
| `Facts`, `HarnessesFound` and `WatchClientsResponse` | Client Link writes the two reports. Server Link and Server Machines commit them and publish the projected `Client.state` through the Server Store feed. The website adapter consumes the published state. The current stream owns both ready flags. |
| `CommandResult.message` field 4 reserved and `CommandResult.outcome` field 3 | Client Link writes the outcome only. Server Link consumes it, Server Machines derives display text, and the website sees server text. Remove the still-listed client writer, server reader, storage column and tests for field 4. |
| `Welcome.server_time` field 4 and the receipt interface | Server Link writes the sample after proof. Client Link combines it with elapsed monotonic time. Client Local now accepts an explicit `now` argument to `Begin` and prunes on `Finish` by count. Server Link sweeps by time. The plans need a shared retention boundary. |
| Restart and status 75 | Server Link sends the stored command. Client Link finishes its receipt and exits. Server Link reads the replay and Stack Client's service reads the exit. Client Link's two-second send budget is within a five-second upper bound. Agree whether the contract prescribes the upper bound or an exact wait. |

### The rules in combination

| Deployment and request | Token and outcome under the current patches |
|---|---|
| Default loopback, website issues | A public origin is absent and the website may not issue a loopback token. Check 5 cannot start. |
| Default loopback, server command issues | Plain loopback fits the listening port. The contract marks all such tokens as host tokens, while the command's tests also make an unmarked one at that same address. |
| Public HTTPS, website issues | The token can carry the configured address and public-key pin. The response and dialog must use that same origin even when the browser uses a different name. |
| Public HTTPS, installer issues | The specified plain loopback address reaches the HTTPS-only published port. No separate plaintext route or port is assigned. |

| Event on the current proved stream, with committed mark 42 | Required result and conflicting rule |
|---|---|
| A valid duplicate with number 40 or 42 | No new effect. The agreed prefix is 42, although the event draft also says to acknowledge the duplicate's own number. |
| A valid report at 43 | Commit the report effect and mark 43 together, then acknowledge 43. |
| A decodable report at 43 with no known kind or with an invalid body | The contract consumes and logs it and acknowledges 43 without a report effect. Server Machines consumes the unknown kind and rejects the invalid body. |
| An invalid envelope or sequence number 0 | Specify `invalid_argument` and preserve 42. Treating 0 as a duplicate would bypass the minimum-number rule. |
| A well-formed report at 44 | Preserve 42 and send the terminal gap reason. The Server Machines wire test still expects `out_of_range`. |
| The same messages on a replaced stream | No effect and no change to presence or mark. Apply the server-owned fence through the write commit. |
| A new `Welcome` behind the locally acknowledged prefix | The Contract requires a local stop with status 0 and both marks preserved. The parallel Client Local fold removes the interface that detects it. |

| Link result | Client action under the patch |
|---|---|
| `failed_precondition` before a verified challenge | Retry. The server's old revision check therefore cannot produce the expected stop. |
| `failed_precondition` after a verified challenge | End `run` with status 0. The reason must distinguish removal, revision mismatch and a gap. |
| Bad proof or database unavailable | Retry with the same identity. Use `unauthenticated` for bad proof and `unavailable` for the database. |
| `aborted` twice within a minute | The draft says wait 60 seconds. The approved bound and the Client Link plan say at most 15 seconds. |
| Refused or malformed `join` | Stop enrollment with a nonzero status to satisfy check 5. The draft's shared definition of stop gives exit 0. |

## Answers to the architect lens

1. Confirmed live, the Sprint 0 files under review match `eb9f2710`. I checked the parallel Client Local fold while verifying the handoffs. `git apply --check` accepted the draft patch against the pre-item-2 sources. A concurrent item 2 build has since added the package options in the shared workspace. In a disposable copy of the pre-item-2 sources, the pinned tools linted and generated both languages, and `go build ./...`, `go vet ./...`, `go test ./...`, `go mod tidy -diff` and the formatting check passed after post-generation tidy. That copy contained generated packages and no implementation tests. By reading, the old sample reply and frontend adapter omit the new address, the script consumes stable log fields, and the Client Local fold makes the old `Reconcile`, `OpenReceipts` and `Prune` anchors stale. Evidence is `src/contracts/package.json:12–21`, `src/contracts/proto/lgtm/v1/clients.proto:11`, `pnpm-workspace.yaml:34–36`, `src/mock-server/src/engine.ts:182–193`, `src/frontend/src/server/sync.ts:117–120`, `deploy/test/checks.sh:329–340` and `.plans/plans/go-s1-client-local/plan.md:405–423`.
2. The public-name table identifies both ends of each changed name and each missing reader or writer. The sample server is a producer if it fills the new scalar, and the website adapter is the intervening reader between protocol and dialog.
3. The schemas and generated bindings sit in the Contract team's allowed folders. The serialization fence stays with Server Machines, the local store with Client Local, settings and listener provisioning with Stack Web and Server Sign-in, and service exits with Client Link and Stack Client. No website or sample-server owner is in the epic's edit table for the new adapter and sample behavior. The orchestrator needs to assign that handoff. Evidence is `.plans/epics/go-rewrite/epic.md:114–127` and `.plans/plans/go-s0-contract/plan.md:738–750`.
4. Rule 1 is kept for the patched link files and sprint ownership, subject to the website handoff. Rule 2 is broken by the conflicting freeze milestones. Rule 3 is kept by placing registry transactions behind its interface. Rule 4 is kept as a contract requirement for committed events and becomes a server-side check after the fold. Rule 5 is kept by Server Machines' `watch.Serve` entry. Rule 6 is broken by the command-clock and receipt window. Rule 7 applies to the durable mark, command and connection authority, which the server plan places in PostgreSQL. Rule 8 is broken by inconsistent refusal codes and their representation. Rule 9 is broken in the neighbouring plans that still carry machine result text, while the explicit keyed token refusal keeps that secret out of receipts. Rule 10 remains feasible with the separate in-process entry points and platform runners for the new cases. Rule 11 has no copied logic in the patches. Evidence is `docs/specs/2026-11-25-go-rewrite-foundation-design.md:358–372` and findings 03–09 and 12.
5. The epic's link agreement needs the handoffs in findings 02–05 and 08–11. The client-state agreement accounts for the saved enrollment fields. The parallel Client Local fold changes the reconciliation and receipt interfaces and must be reconciled with Contract and Client Link before the tests freeze. The Store Go interface remains sufficient for the atomic event mark and the request-key veto. The vault credentials agreement is outside these patches. The real-program entry points are present in the Server Sign-in and Client Link plans. The added service and browser observations need runners of their own. Evidence is `.plans/epics/go-rewrite/epic.md:172–180`, `.plans/plans/go-s2-server-signin/plan.md:464,517` and `.plans/plans/go-s3-client-link/plan.md:277`.
6. Item 1 has its module-only gate. Item 2 can merge before the Go script exists, with direct commands after generation and tidy. Items 3–6 still serialize on generated files. Item 7 can merge after item 2, and item 8 needs both 6 and 7. The generated code has no unfinished dependency between those items. If item 6 merges under its provisional assumptions, the first broken boundary would be a dependent build treating that provisional revision as frozen. Findings 08 and 09 identify the downstream compile and release boundaries.
7. The patch leaves wave 2 message numbers and side calls open and builds no side call, byte stream or agent feature. Issuing and invalidating the stream token has a design requirement in wave 1. The waiting-to-approved transition needs a documented rule for the future token reader, and the reserved result field needs no replacement. Evidence is `.plans/plans/go-s0-contract/plan.md:542–550` and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:278–279,531–537`.
8. The patches add no dependency version. Confirmed live, the installed Go 1.23.3 selected cached Go 1.26.0 in the scratch module. The pinned Go generators reported Protocol Buffers 1.36.12 and Connect 1.21.1, and `go mod verify` passed. The workspace ran Buf 1.75.0 and used its existing `protoc-gen-es` 2.17.0. The checkout install recipe uses the existing pnpm 12.16.1 lockfile tooling. I did not install a second workspace dependency tree. Evidence is `.plans/plans/go-s0-contract/plan.md:228,254–260`, `package.json:10` and `pnpm-workspace.yaml:34–36`.

## Post-build checks

- Race admission after durable stream selection and before hub registration. Remove the machine or revoke its waiting token and then release the handler. The server exposes no live `Welcome` or accepted subsequent write from stale authority.
- Test the final-result retention boundary with an authenticated clock sample that leads server time and a disconnected client holding a completed result. The real server can still obtain that result through its collection cutoff, and none of the attempts reruns the action. Repeat at the configured receipt-count limit.
- On replacement, hold `Facts` or make the harness report invalid. The website shows joining until both are valid and committed on the new stream. A late close on the old stream cannot change the new stream's projection.
- Exercise the patched issuance matrix through the real stack with a nondefault published port. The website-issued address equals the token's address, the copied command joins against Go, and the sample shows its agreed fixture value. The installer obtains a reachable token with the intended host mark.
- Run the installed service on Linux and macOS. Losing the restart reply after the receipt commits gives one restart and a replayed outcome. A terminal removal leaves the service stopped. A separate command-line process exits nonzero for expired and used tokens and emits the matching structured reason.

## Grade

5 out of 10. There is 1 blocker, 10 majors and 1 minor.
