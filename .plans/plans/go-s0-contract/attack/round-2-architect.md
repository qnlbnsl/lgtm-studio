REVISE

The reconnect handshake can advance the server’s event mark past reports that are absent from its database, then tell the client that those reports were committed.

## Findings

### architect-r2 01 · BLOCKER · C11

The event rule commits the report and its mark together, but the new recovery rule independently raises the same mark from `Hello.acked_event_number`. Both signatures omit that field. The plan then uses the raised value for `Welcome.last_event_number` and `EventAck.through`. (.plans/plans/go-s0-contract/plan.md:346, :350, :535, .plans/plans/go-s0-contract/draft/link.proto:199, :215, :231)

A database restored at event 40 has no effect from events 41 and 42. The client already deleted their files after receiving an acknowledgement through 42. The server raises its mark to 42 and accepts 43. The new facts and harness reports can refresh the machine’s current display. They cannot establish that the effects of 41 and 42 exist in the restored database. With an altered handshake field, even events still in the client’s queue can fall below the invented mark.

Keep the committed prefix separate from any recovery baseline. Resume the same event history only from a prefix whose effects are present. When a restore loses an acknowledged prefix, require a recovery decision that records the loss and establishes a new baseline or restores those effects from durable data. Bind any values used in that decision to the authenticated handshake and decide how the selected transport authenticates subsequent acknowledgements. Change the recovery acceptance row to verify the event effects, the reported mark and what the client deletes.

Category is data integrity and design. Evidence is by reading. The Contract planner owns the rule with the Server and Client leads. Risk is 224, from severity 8 for lost data, fallout 7 for a deleted prefix and probability 4 for a database restore.

### architect-r2 02 · BLOCKER · C11

The claim that the Client Local outbox accepts an acknowledgement above `Next()` is reversed. `Ack` and `Reconcile` explicitly refuse it. `Reconcile` takes a mark for the current outbox epoch, which is a durable identifier that the patch never exchanges. (.plans/plans/go-s0-contract/plan.md:534, :733, .plans/plans/go-s1-client-local/plan.md:678, :686, :698, :710, :841)

If the server is at 40 and the machine’s event folder is empty, the proposed recovery reaches `ErrServerAhead`. If a new local folder has already numbered its first 50 events, the same server mark of 40 is inside the local range. Treating it as a mark for the new files deletes 40 reports the server never saw.

Have Contract, Client Local, Client Link and Server Machines agree on the identity of an event history and the action for a missing or replaced outbox. Carry that identity in the contract if the same machine can start a new history. Agree on the stored key for the prefix, the `Acked()` and `Reconcile()` calls, and the return codes for mismatched histories. Limit live acknowledgements to the prefix actually sent for that history. Replace the claim about `Ack` and test a nonempty replacement outbox as well as an empty one.

Category is data integrity and design. Evidence is by reading. The Contract and Client Local planners own the interface with both link leads. Risk is 168, from severity 8 for deleted reports, fallout 7 for unrecoverable local files and probability 3 for an outbox replacement followed by reconnection.

### architect-r2 03 · BLOCKER · D7

A fresh registry read and a check that a stream is current specify two checks without fixing their order relative to removal, stream installation and durable writes. The plan promises revocation at the commit, then invalidates the stream after that commit. Token revocation also removes a waiting machine and must take the same path. (.plans/plans/go-s0-contract/plan.md:449, :524, :529, :530, :713, .plans/plans/go-s0-contract/draft/link.proto:91, docs/specs/2026-11-25-go-rewrite-foundation-design.md:254)

A stream proves the key and reads the machine. The owner removes the machine and invalidates the stream the hub currently holds. The proven stream installs itself after that, using its earlier read. In another interleaving, a stream starts a write while current and commits after its replacement takes over. Testing a late heartbeat sent after the replacement’s `Welcome` exercises neither race.

Define one per-machine fence with a total order for proof installation, supersession, removal, and the commit of each stream-owned effect. Specify the Server Machines and Server Link interface that shares the fence. The removal commit must invalidate the old authority even if cancellation or the notice is delayed. A new installation or effect based on authority that lost the fence must fail. Use the same transition for removal through token revocation.

Category is authorization and data integrity. Evidence is by reading. Server Machines and Server Link own the fence, with the Contract planner recording its semantics. Risk is 168, from severity 7 for revoked authority retaining access, fallout 6 for effects the owner may have to repair and probability 4 for an overlapping removal and reconnect.

### architect-r2 04 · BLOCKER · D17

The plan allows all four owner choices to wait until before sprint 3, but freezes the link after sprint 0. Point 2 offers an option that adds fields to `Hello` and `Challenge` and another that changes the transport in sprint 2. Point 1 offers a new contract method, too. (.plans/plans/go-s0-contract/plan.md:104, :110, :120, :129, :691, .plans/epics/go-rewrite/epic.md:118, :127, .plans/plans/go-s2-stack-web/plan.md:42, :65)

If the owner picks authenticated messages at the stated deadline, the signed bytes and the description tests have already been frozen. If the owner picks transport pass-through then, the proxy and server listener have already been built against the other topology. An encrypted connection that the front server cannot inspect also needs a routing discriminator other than the procedure path.

Separate the decisions required before freezing the schema and proof from the choices that only affect presentation or command-line input. Resolve contract-changing choices before the relevant sprint 0 items merge, or explicitly leave that part unfrozen and plan a return to phase 3 before its first consumer. Resolve the transport and certificate handoff before Stack Web and Server Sign-in build them. Record the end point that terminates Transport Layer Security (TLS), the route or listener that serves it, and the rule for the machine certificate.

Category is architecture and build sequencing. Evidence is by reading. The orchestrator owns the decision with the Contract, Stack Web and Server Sign-in leads. Risk is 180, from severity 6 for a link the client cannot use, fallout 5 for coordinated rework and probability 6 because the plan explicitly offers both incompatible routes at the later deadline.

### architect-r2 05 · BLOCKER · plan

The patched acceptance requires the Contract reviewer to run both the contract check and the Go check on each item’s commit from 2 through 8. Item 7 creates `scripts/check-go.sh` and depends on item 2. The rollout puts item 7 after item 2 as well. (.plans/plans/go-s0-contract/plan.md:96, :265, :602, :618, .plans/epics/go-rewrite/epic.md:75)

On the exact commit for item 2, the required script has no file to execute. Item 7 cannot land first under the stated dependency. The prescribed merge gate therefore blocks its own prerequisite. The guard probe also invokes bare `buf` after `cd`, although the plan locates it only in a workspace dependency. A clean worktree needs dependency setup and an explicit way to resolve that executable.

Give item 2 an executable gate that uses the listed Go commands directly. Require `scripts/check-go.sh` on exact commits beginning with item 7 or move its creation into item 2 and revise the item boundaries. Use the workspace package runner or an explicit binary path for every Buf probe. Describe the dependency setup and cleanup for each throwaway checkout.

Category is a verification gate. Evidence is by reading for the dependency cycle and confirmed live for the executable location in this session. The Contract planner and sprint test lead own the gate. A required failing gate carries no product risk score.

### architect-r2 06 · MAJOR · C10

The signed `Challenge` proves the server key and the nonces. It contains no verdict about the machine. The proposal stops when an `unauthenticated` status follows that challenge and stops on `failed_precondition` even before a verified challenge. The same `unauthenticated` status also covers a bad machine proof. The unit that runs the client restarts every nonzero exit. (.plans/plans/go-s0-contract/plan.md:511, :515, :518, .plans/plans/go-s0-contract/draft/link.proto:36, :73, :215, .plans/plans/go-s3-stack-client/plan.md:30, :232)

With the unprotected hop described in point 2, the client can see a valid challenge followed by a status that the server did not authenticate. It stops a valid identity. A wrong machine signature also tells the owner to enroll again even when the registry still has the machine. If a permanent refusal exits nonzero, the shipped supervisor starts the same loop again.

Make stopping depend on an authenticated terminal verdict bound to the connection attempt, or on a transport that the chosen architecture explicitly trusts end to end. Define which identity, proof and revision failures are terminal. Treat an unverified status or a broken transport as retryable. Align the terminal exit with Stack Client’s exit 0 rule and its restart exit 75. Give `aborted` a bounded retry schedule, and document that two processes with one key can still replace each other after each wait.

Category is authentication and availability. Evidence is by reading. The Contract and Client Link planners own the stop rule with Server Link and Stack Client. Risk is 120, from severity 6 for a stranded client, fallout 5 for recovery that can require the owner and probability 4 for a false terminal status or misclassified proof failure.

### architect-r2 07 · MAJOR · R6

The command clock calculation adds the entire local elapsed time since `Proof` to a server timestamp sampled later. It counts the outbound proof transit and server processing twice. The client-local receipt store defaults to the machine’s wall clock, persists its high-water mark, and prunes at open. No handoff chooses another clock for either check. (.plans/plans/go-s0-contract/plan.md:327, :328, :736, .plans/plans/go-s0-contract/draft/commands.proto:14, .plans/plans/go-s1-client-local/plan.md:756, :765, :770, :776)

If proof transit and verification take two seconds, a command can have one second left on the server when it reaches the client and be declared expired by the estimate. A server clock that jumps ahead after its sample can make the estimate late instead. Separately, a client whose wall clock advanced by more than a day can prune a final receipt before the server finishes collecting its result. A five-minute offset test of the link calculation by itself misses both cases.

Define what the client guarantees in the presence of transit delay, clock jumps and suspension. State whether refusing early within the uncertainty window is allowed. Have Client Link and Client Local select the time input for `Begin`, `OpenReceipts` and `Prune`, including the time available before a first verified `Welcome`. Keep the final receipt until the server’s result collection window is over in that time base. Align the acceptance test with the resulting guarantee.

Category is command lifecycle and data integrity. Evidence is by reading. Contract, Client Local and Client Link own the time and receipt agreement with Server Link. Risk is 120, from severity 6 for a refused valid command or lost result, fallout 5 for owner intervention and probability 4 for a slow handshake or clock adjustment.

### architect-r2 08 · MAJOR · C7

The patch says `Welcome` marks a machine online and also says an approved machine is joining until both reports arrive on that stream. It sends fresh reports only after a `Welcome` that permits events. The website has distinct waiting, joining and online projections. (.plans/plans/go-s0-contract/plan.md:351, :356, :526, :536, .plans/plans/go-s0-contract/draft/link.proto:97, src/frontend/src/server/sync.ts:17, :25)

An approved machine reconnects when it has no room to write its fresh reports. One implementation publishes online at `Welcome`, while another publishes joining. If the owner chooses the heartbeat-only waiting policy, a machine can receive approval on the same open stream and remain joining forever because it never receives a second `Welcome` to trigger reporting.

Use separate states for a proven live stream and the status published through `WatchClients`. Keep waiting visible until approval, show an approved live machine as joining until the current stream commits both reports, and publish online after that. If the waiting policy limits reports, trigger the missing reports when a higher standing revision first permits them, including on an existing stream. Define which messages advance the per-stream readiness flags and when replacement clears them.

Category is state projection and design. Evidence is by reading. Server Machines and both link leads own the projection and the reporting trigger. Risk is 144, from severity 6 for enrollment stuck in joining, fallout 3 for recovery by reconnect and probability 8 because approval on an open stream is an ordinary path under the narrower policy.

### architect-r2 09 · MAJOR · D17

The new agreement makes one configured address and certificate pin authoritative for both token producers. The Stack Client plan still requires `join-token --address` and always supplies a plain loopback origin from the installer. Server Sign-in names `LGTM_PUBLIC_URL`, but the agreement gives the certificate pin no setting, file or load step. Client Local has a generic certificate fingerprint slot and still describes making a machine certificate. (.plans/plans/go-s0-contract/plan.md:454, :717, .plans/plans/go-s3-stack-client/plan.md:138, :142, :219, :225, .plans/plans/go-s2-server-signin/plan.md:359, :370, .plans/plans/go-s1-client-local/plan.md:193, :376)

On a public Hypertext Transfer Protocol Secure (HTTPS) deployment, the local installer supplies loopback while the token maker follows the configured HTTPS origin. It either fails to make the token or the client rejects the address mismatch. If the token maker uses the argument, it violates the one-origin contract. A pin checked at enrollment can also have a different meaning after a client restart if `run` loads a certificate digest where enrollment used the key digest.

Have Stack Web specify the exact pin bytes, where they come from and the named setting or file it writes. Have Server Sign-in and both token producers use one validated origin-and-pin interface. Decide whether the installer uses that same origin or has an explicit separately configured local enrollment route. Treat `--address` according to that decision and align the installer tests. Name the client’s persisted fields and assign their writer to Client Enrollment and their reader to Client Link. Update Client Local’s machine-certificate assumption when the transport decision lands.

Category is deployment and contract design. Evidence is by reading. Stack Web, Server Sign-in, Server Machines, Stack Client and Client Link own the handoff through the orchestrator. Risk is 240, from severity 6 for failed enrollment, fallout 5 for coordinated configuration repair and probability 8 for a public deployment that runs its local installer.

### architect-r2 10 · MAJOR · C6

The patch makes `JoinToken.address` the only dial address, says the website only passes through the token text, and still requires the website to print a separate address argument. The page currently sources that argument from a display label with the scheme removed. The sample server still returns a bare secret, while the new field comment unconditionally promises an encoded `JoinToken` at the same website revision. (.plans/plans/go-s0-contract/plan.md:192, :452, :718, :719, .plans/plans/go-s0-contract/draft/existing-files.diff:64, src/frontend/src/screens/service.tsx:410, src/frontend/src/server/address.ts:44, src/mock-server/src/engine.ts:182, :193, src/contracts/README.md:29)

In a deployment with a separate website origin or an alias, the browser’s display label can differ from the origin in a valid token. The owner pastes the offered command and enrollment refuses it. A shared dialog that learns to decode the new token can also reject the sample server’s revision-1 secret.

Give the dialog an authoritative argument source, either a defined reader for the token’s origin or a separate canonical address from the token producer. Alternatively, settle a token-only command and remove the second source. Explain how the revised dialog distinguishes or upgrades the sample producer. Put that compatibility change, including any revision change, in the same coordinated handoff that assigns the frontend writer for restart. Test the actual copy action with a separate browser origin and with the sample server.

Category is compatibility and enrollment. Evidence is by reading. The orchestrator owns the frontend and sample handoff with Contract and Client Enrollment. Risk is 180, from severity 6 for a command that cannot join, fallout 3 for a corrected command and probability 10 for the current scheme-free label used with a strict origin comparison.

### architect-r2 11 · MAJOR · C5

Client Enrollment must retry a transient `Join` error until the token expires. The encoded `JoinToken` has no expiry. `CreateClientTokenResponse.expires_at` reaches the browser, while the server command supplies only the token text. (.plans/plans/go-s0-contract/plan.md:283, :450, :479, .plans/plans/go-s0-contract/draft/link.proto:114, :134, .plans/plans/go-s3-stack-client/plan.md:88, :225, src/frontend/src/server/sync.ts:117)

The owner starts a command shortly before expiry. The endpoint is unavailable until after the token expires. The client has no input that distinguishes this situation from a freshly made token that still has most of its life. A limit measured from invocation would leave the installer waiting after the actual expiry.

Either include the expiry in the token and add its producers, readers and test vector to item 5, or give enrollment a bounded local attempt policy with the server as the only authority on expiry. Keep the same saved key across retryable errors. Record the outcome when the server cannot reply by the limit separately from a permanent token refusal.

Category is enrollment failure policy. Evidence is by reading. Contract, Server Machines and Client Enrollment own the retry input with Stack Client. Risk is 72, from severity 6 for a stalled installer, fallout 3 for starting with a fresh token and probability 4 for an outage during enrollment.

### architect-r2 12 · MAJOR · R9

The patch prevents a join token secret from entering a request receipt by declaring that every `CreateClientToken` call creates a new token, including a repeated request key. The approved database design says a retried mutation happens once. The Store plan leaves credential-bearing receipt policy for the owner. (.plans/plans/go-s0-contract/plan.md:281, :716, .plans/plans/go-s0-contract/draft/existing-files.diff:17, docs/specs/2026-11-25-go-rewrite-foundation-design.md:117, :176, .plans/plans/go-s1-server-store/plan.md:49, :374)

The server commits token A and loses the reply. The same request key creates token B on the retry. The dialog learns only B’s identifier. Closing it can revoke B, while A remains in the registry until it expires. The secret is absent from stored replies, but the advertised request-key guarantee fails and the first token cannot be managed by the caller.

Settle and record the credential exception with Server Store and Server Machines. If the method does not take request keys, document and test how it rejects them and how an unknown outcome is handled. If it accepts keys, define a duplicate response or replacement operation that retains the correct number of live tokens without persisting plaintext secrets. Test a lost reply, the same key and the caller’s ability to revoke the live token.

Category is secret handling and retry consistency. Evidence is by reading. Server Store and Server Machines own the policy with the Contract planner and the orchestrator. Risk is 72, from severity 6 for a core retry guarantee that requires intervention, fallout 3 for tokens that expire on their own and probability 4 for a lost reply.

## Answers to the assignment’s seven questions

1. The patch audit below checks all 18 plan locations, the five draft files, the new decision rows and the scouting additions against their first-round failures.
2. `Hello.acked_event_number` does not prove a restored prefix or identify an outbox history. The signed `Challenge` does not authenticate the later refusal. The new `unauthenticated` and `aborted` branches need the trust and process agreements in findings 01–03 and 06.
3. The register records zero rejected findings. I checked the outcome rows and the outcome total. (.plans/plans/go-s0-contract/attack/round-1-findings.md:18, :78)
4. The writer and reader table below identifies the missing consumers and handoffs for changed names and behaviors.
5. The tables below exercise the interacting event, presence, join, command and refusal rules. The contradictions affect recovery, revocation, expiry and the order of the merge gates.
6. First-round groups 1 and 2 remain open for recovery, group 3 for install and commit fencing and authenticated refusal, group 4 for presence, group 5 for terminal-code handling, group 6 for the enrollment retry deadline, group 7 for the expiry clock, groups 10 and 11 for the producer and consumer handoff, group 14 for idempotence, and group 17 for the merge gate. Group 12 remains with the owner and now has an earlier architectural dependency.
7. All eight architect questions apply to the patches. The answers below identify which parts have source or command evidence and which agreements still need a decision.

### Result of each patch in the register

The numbers match the patch list in `.plans/plans/go-s0-contract/attack/round-1-findings.md:124`.

| Patch | Result |
|---|---|
| 1. Overview | The connected-restart incompatibility is accurately described. The join-token semantics also change for the browser, and the proposed deadline is too late for the transport choice. Findings 04 and 10. |
| 2. Key decisions | The normal event path now has a contiguous committed prefix. Removal is independent of delivery of the notice. The restore, replacement and terminal refusal exceptions still break those guarantees. Findings 01–03 and 06. |
| 3. Owner points | The four choices are explicit. The restart method and message authentication alternatives can change the frozen contract. The transport alternative also changes sprint 2. The waiting choice needs an approval-on-live-stream transition. Findings 04 and 08. |
| 4. Public names | The new enum value and optional load presence have defined meanings. The event mark, retry codes and token semantics lack the full agreement recorded below. Findings 01, 02, 06 and 09–11. |
| 5. Descriptor format | Reserved ranges and names appear in comparisons of described messages and enums. The separate shape assertion at `plan.md:292` still identifies `position` by kind and placement without asserting singular cardinality. |
| 6. Item 2 | Confirmed live in a throwaway tree, the stale probe changed both generated languages and the new source passed lint and generated exactly three additional files. The before-and-after guard comparison is in the plan. The exact-commit Go gate and the bare executable in the guard reproduction remain unresolved. Finding 05. |
| 7. Item 3 | First-position zero and optional old-browser decoding agree. The role and address projections now name their source. The token’s new response semantics and request-key exemption still need agreement. Findings 09, 10 and 12. |
| 8. Item 4 | The new golden values match the draft. Current attempts, interrupted receipts, restart ordering, unknown reports and optional load now have stated rules. The clock input, restored event history and public presence rules still disagree. Findings 01, 02, 07 and 08. |
| 9. Item 5 | Atomic first use and the narrow replay exception fix the original used-token ambiguity. The client cannot implement the stated retry cutoff, and the address sources still diverge. Findings 09–11. |
| 10. Item 6 | The handshake phases and deadlines, descriptor field 4 and the comment mutation are specified. The value rules require `Welcome.state`, while the standing comment says missing state counts as waiting. Scope that fallback to an explicitly allowed absent value, or use the invalid-message path consistently. The three new mechanisms, write fencing and presence need findings 01–03, 06 and 08. |
| 11. Item 8 | The negated assertions express absence and the package boundary is stated. The existing unqualified instruction to answer a call without a session should also be scoped when the document is revised. (`src/contracts/README.md:35`, :39) |
| 12. Completion rows | Rows 2, 5 and 6 now cover the guard, the retained revision and actual declaration comments. A comment-presence test establishes that declarations are documented. The protocol rules still need the semantic checks below. |
| 13. Risks | The new risks name the right areas. Their mitigations assume the unresolved restore, write-fence, clock and refusal properties. The listed owner deadline misses the sprint 2 dependency. Findings 01–04, 06 and 07. |
| 14. Guardrails | The comment check and revision assertion now have named evidence. The revision handoff needs to include the changed join-token consumer alongside restart. Finding 10. |
| 15. Team agreements | The rows name both sides of most link exchanges. The outbox epoch, terminal process status, proxy pin and browser address still lack an agreed path. Findings 02, 06, 09 and 10. |
| 16. Post-build checks | The lost-reply, proxy and old-browser checks are useful. The restore test only observes arrival of new reports. The stream test misses in-flight commits, and the clock test misses delay and persisted receipt time. Findings 01, 03 and 07. |
| 17. Traceability | The new rows point to the decisions and register. Coverage at those references still depends on closing the mechanisms and owner gates identified here. |
| 18. Attack log | The counts agree with the register. Fixed and fixed-another-way entries on event recovery and removal need to reopen. Findings 01–03 and 06. |

The patches to `draft/link.proto` add the mark and code rules but leave the proof bytes and the durable fence insufficient. The patches to `draft/commands.proto` define outcome 5 and preserve the restart receipt before exit, with the unresolved expiry clock. The patches to `draft/events.proto` enforce the ordinary prefix and optional load, with the recovery exception in `link.proto`. The patch to `draft/common.proto` fixes the empty-feed position. The patch to `draft/existing-files.diff` changes the browser token, retry, revocation and address behavior covered above.

Decision rows 38 and 39 fix the ordinary prefix and introduce the restore defect. Rows 40 and 41 fix the outcome and restart receipt ordering, leaving the clock handoff open. Rows 42 through 45 leave the refusal, fencing, presence and missing-state issues above. Rows 46 and 47 settle server-side join replay and a canonical dial address, leaving the retry deadline and the cross-team origin and pin unsettled. Row 48 fixes optional load. Row 49 still gives conflicting readiness transitions. Row 50 keeps the token out of receipts and opens the request-key gap. Row 51 defines zero consistently. Row 52 defers some choices past their first consumers. Row 53 improves the probes while leaving the exact-commit gate impossible. (.plans/plans/go-s0-contract/decisions.md:45, :60)

The changed `scouting.md` anchors for the machine projection, menu, dialog and sample handlers resolve. Its corrected address source removes the erroneous `Facts` attribution. Its outbox claim does not hold against the current Client Local plan. The generation evidence covers file shape and pins. It cannot establish link behavior. (.plans/plans/go-s0-contract/scouting.md:33, :38, :83, :87)

### Writers and readers of the changed contract

| Name or rule | Writers and readers |
|---|---|
| `Hello.acked_event_number`, `Welcome.last_event_number`, `EventAck.through` | Client Local writes the durable outbox mark and epoch, Client Link reads them and sends `Hello`, Server Link reads it and writes the stored mark through Server Store, and Client Link consumes both server marks through the outbox. The epoch and store key have no shared contract. |
| `COMMAND_OUTCOME_INTERRUPTED`, `Command.attempt`, `Command.expires_at`, `Welcome.server_time` | Server Link writes the attempts, timestamp and expiry. Client Link and the Client Local receipts read them. Client Local writes the recovered state, Client Link writes the result, and Server Link and Server Machines read it. The receipt clock input and its startup reader are absent from the agreement. |
| Optional `Heartbeat` load, `Facts`, `HarnessesFound`, `Client.state` and the machine projection | Client Local measures and scans, Client Link writes reports, Server Link ingests them, and Server Machines writes the projection read by the website. Server Link must distinguish readiness from connection liveness. The display address needs a trusted proxy source or an empty projection, and the server command and installer own the provenance of `hosts_server`. |
| `CreateClientTokenResponse.token`, `JoinToken.address`, `certificate_key_sha256`, `expires_at` | Server Machines and the server command produce tokens. Stack Web must publish the address and pin, Server Sign-in must load them, the website and installer pass the token onward, Client Enrollment reads it and persists settings, and Client Link reads the persisted origin and pin. The sample writer, the dialog’s address reader and the command-line expiry reader are missing from the agreement. |
| `permission_denied`, `unauthenticated`, `failed_precondition`, `aborted` and other link codes | Server Machines writes join refusals, Server Link writes stream outcomes and the independent route may return an early status. Client Enrollment and Client Link consume their respective codes. Stack Client consumes terminal and restart process exit codes. A signed challenge writes no terminal verdict. |
| `ReconnectClient` and website revision | Server Machines and the sample server write the operation’s behavior, Server Link sends `Restart`, Client Link exits through the supervisor, and the Machines page reads `GetServer` and invokes the action. Point 1 names a coordinated rollout as one option. The join-dialog change needs the same version agreement. |
| `FeedPosition.value` and `WatchClientsResponse.position` | Server Store owns the input position. Server Machines owns the response mapping through the server’s watch code. The website currently ignores the position, and the sample response omits it. |
| Link procedure paths and `Lgtm-Link-Token` | Contract generates the procedure names, Server Sign-in mounts them, Stack Web forwards them, and Client Enrollment and Client Link call them. Server Link issues and invalidates the connection token. The adjacent-call readers belong to wave 2. |

### Truth tables for the patched rules

For events, the server mark is `S`, the durable local acknowledgement is `A` and `N` is the next local event number. An event number has meaning within one outbox history.

| Event case | Result under the patch | Judgment |
|---|---|---|
| Same history, `A <= S < N`, with evidence of the committed prefix | Replay above `S`. | The server can acknowledge a committed duplicate without applying it again. |
| Same history, incoming event numbered `S + 1` with a known valid report | Commit the effect and the mark together, then acknowledge. | The ordinary crash path holds. |
| Incoming event numbered `S + 1` with an unknown report or an invalid known report | Consume the unknown report. Refuse the invalid report and leave `S` unchanged. | Both branches are stated. The invalid-report path needs a recoverable client stop or repair if the same persisted event fails on every retry. |
| Incoming event above `S + 1` | Return `out_of_range`, leave `S` unchanged and retry. | A real gap needs remediation beyond reopening the same stream. |
| Database restored with `S < A` | The proposed handshake raises `S` to `A` and resumes. | A fresh report cannot prove the missing prefix. |
| Empty outbox, `S >= N` | The plan expects an acknowledgement to jump the counter. | Client Local explicitly refuses the acknowledgement. |
| Replaced outbox with new files and `A <= S < N` | The patch uses `S` as if both sides name the same events. | New reports can be discarded as duplicates. |

| Link status or machine state | Result under the patch | Judgment |
|---|---|---|
| `Join` with `permission_denied` or `invalid_argument` | Stop enrollment. | The draft spells out both rows. The summary at `plan.md:518` omits `Join invalid_argument`. |
| `Join` with `unavailable` or a lost response | Keep the key and retry. | Exact replays can succeed. The token text supplies no client-side expiry deadline. |
| `Open` with `unauthenticated` before a verified challenge | Retry. | No identity verdict was established. |
| `Open` with `unauthenticated` after a verified challenge | Stop and tell the owner to join again. | The proof does not authenticate this status and the same row covers bad proof and removal. |
| `Open` with `failed_precondition` | Stop, including before a verified challenge. | The validation rule also calls a revision other than 1 `invalid_argument`. Specify precedence and authorize the terminal verdict. |
| `Open` with `aborted`, `out_of_range`, `invalid_argument`, `deadline_exceeded`, `permission_denied` or `unavailable` | Keep local state and retry within 15 seconds. | A replacement delays a duplicate process. It does not eliminate ongoing replacement. A persisted malformed event needs the recovery policy above. |
| Waiting with a live stream | `Welcome` marks online, but the owner has not approved the machine. | Publish waiting independently of stream liveness. Apply the owner’s eventual message policy. |
| Approved and live with fewer than two current-stream reports | `Welcome` marks online, but item 4 says joining. | Publish joining until both reports commit. |
| Approved and live with both reports, approved with no live stream, or removed | The respective public states should be online, offline and absent. | A superseded stream must not change any of these projections. |

| Join, command or receipt case | Result under the patch |
|---|---|
| Valid first use | Consume the token and create the waiting machine in one transaction. |
| Same secret and key, live binding, unexpired and unrevoked | Replay the first reply. A wrong secret, changed key, expiry, revocation or removed binding refuses the request. A database failure is retryable. |
| No receipt and command before or at expiry | Before expiry, write the started receipt before work. At or after expiry, return expired. The clocks required to classify that boundary disagree. |
| Running receipt and higher attempt, or a final receipt and any accepted attempt | A running command eventually labels the result with the latest received attempt. A final receipt replays even after expiry. |
| Result from replaced stream or stale or future attempt | Ignore it. The authority and attempt must still be current when the effect commits. |
| Current result with an unknown or unspecified outcome | Store failed. This normalization closes the command even though the `UNSPECIFIED` enum entry is itself nonfinal. |

## Answers to the architect’s eight questions

1. **Existing code and tools.** By reading, the menu only offers reconnect while offline, the sample enforces that precondition, the pairing dialog revokes on close, the website ignores feed position, and both revision constants are 1. Confirmed live, `git apply --check` accepts the draft against `fca037e0`. Buf reports 1.75.0. The contract package exposes Buf through its workspace dependencies. The outbox mismatch is between plans for future implementations. (`src/frontend/src/screens/service.tsx:351`, :374, `src/frontend/src/server/sync.ts:45`, `src/frontend/src/server/api.ts:15`, `src/mock-server/src/engine.ts:47`, :236, `src/contracts/package.json:11`, `pnpm-workspace.yaml:34`)
2. **Writers and readers.** The table above lists the affected boundaries. The missing consumers that change the design are Client Local’s event epoch and receipt clock, the sample token writer, the dialog’s origin reader, the persisted pin reader and the service supervisor.
3. **Placement.** The draft files and generated code belong to Contract. Registry and event authority belong to Server and local durable state to Client. Stack owns the proxy and installer. The frontend handoff and the cross-program test folder need an explicit orchestrator assignment outside those edit scopes. (.plans/epics/go-rewrite/epic.md:114, .plans/plans/go-s0-contract/plan.md:719, :826)
4. **Rules.** R1 kept for contract ownership. R2 broken at the proposed freeze for owner options that change the contract. R3 kept in the proposed team placement, with a missing interface for the shared removal fence. R4 broken by the restored mark without its event effects. R5 kept by the feed position and reconnect shape. R6 broken at expiry and receipt-time boundaries. R7 kept in principle, with durable authority still needing the fence in finding 03. R8 broken by ambiguous terminal statuses and conflicting revision validation. R9 keeps plaintext out of receipts but needs the credential retry exception. R10 is assigned to a combined in-process test against real PostgreSQL, with a separate browser and real proxy fixture needed for their own user flows. R11 does not apply to the patches because they carry no copied implementation.
5. **Neighboring plans.** Epic agreement 1 has the mismatches in findings 01–04 and 06–12. Agreement 2 supplies Store transactions and keyed receipts, with the credential exception and removal fence still open. Agreement 3 supplies the vault credential, which is separate from the missing certificate pin input. Agreement 4 conflicts with the stated `Ack` behavior and lacks an epoch and clock agreement. Agreement 5 has the server fixture in the Sign-in plan and an open client entry point in the Client Link brief. The combined test’s owner needs both before it can run. (.plans/epics/go-rewrite/epic.md:165, .plans/plans/go-s2-server-signin/plan.md:449, .plans/plans/go-s3-client-link/plan.md:58)
6. **Independent merges.** Item 1 has a module-only gate. Item 2 cannot pass its new exact-commit requirement because item 7 supplies the script after item 2. Items 3 through 6 otherwise have ordered generation steps and item 8 needs the finished link and script. Finding 05 specifies the gate split.
7. **Wave 2.** The unreserved oneof numbers allow expansion, and the adjacent-call token names a future seam. The patch builds no wave-2 calls. Application message authentication or encrypted pass-through would be a wave-1 transport decision if selected. The approval policy must specify whether a waiting machine can hold a token before future readers build against it.
8. **Dependencies.** The patches add no new dependency pin. The inherited pins are Go 1.26.0, Connect 1.21.1, Protocol Buffers Go 1.36.12 and Buf 1.75.0. The cached Connect module declares Go 1.26.0 and the cached Protocol Buffers module declares Go 1.23. Confirmed live with those pins, the drafts pass lint, generate both languages twice without a difference, build and pass vet in a throwaway copy of `fca037e0`. After the specified `go mod tidy`, its diff check is clean. The live check covers generation and compilation. The plan’s future descriptor tests and the complete Git guard comparison were outside that probe. (.plans/plans/go-s0-contract/plan.md:220, :246, `pnpm-workspace.yaml:34`)

## Post-build checks

| Check | Scenario and expected result | Evidence and owner |
|---|---|---|
| architect-r2 P1 | Restore a database below a previously acknowledged mark, reconnect with both an unchanged and a replaced outbox, and use a separately numbered pending report. The settled recovery rule preserves pending reports and does not claim missing effects were committed. | Compare PostgreSQL effects and marks with local filenames, their contents and the local acknowledgement. Both link teams and the combined test owner. |
| architect-r2 P2 | Pause stream installation after the proof’s final read, and separately pause an event or result write before commit. Remove or replace the machine while each is paused. Registry authority and durable effects follow the defined order. | Capture barrier ordering, resulting server rows, token validity and stream outcomes. Server Machines and Server Link. |
| architect-r2 P3 | Drive the real client through a verified challenge followed by an unauthenticated transport failure, a confirmed permanent refusal, a revision mismatch and an aborted stream. Only the terminal authenticated cases stop the supervised process. | Record the response evidence, client exit status, retained state and supervisor restart count. Client Link and Stack Client. |
| architect-r2 P4 | Add proof transit delay near expiry, advance the server clock, and reopen the client with a skewed clock after a final receipt was written. The command runs and the receipt is pruned only when the settled timing policy permits it. | Record the server sample and expiry, client receipt, action count and terminal result. Client Local and both link teams. |
| architect-r2 P5 | Approve a waiting machine on its existing stream, then replace the stream before the second fresh report arrives. The browser moves through the agreed waiting, joining and online states without an early online projection or a stuck joining state. | Record both streams and the `WatchClients` sequence. Client Link and Server Machines. |
| architect-r2 P6 | Copy the join command from a browser using a different origin from the configured link, run the local installer on the same deployment, restart the joined client and exercise the sample dialog. All supported paths use the intended token format, canonical address and certificate pin. | Capture procedures and pin checks without token contents. The orchestrator, Stack Web, Stack Client and Client Enrollment. |

## Grade

4 out of 10, with 5 blockers, 7 majors and 0 minors.
