REVISE

The event contract tells a client with an empty outbox to resume after the server's last event, but the Client Local plan rejects that mark and cannot advance the counter.

## Findings

The reviewed patch ends at `fca037e0`. The plan, decisions, scouting and drafts match that revision at the inspected checkout, `040b6ce0`. The findings below are by reading unless a check says confirmed live.

### logic-r2 01 — BLOCKER — C11 — An empty outbox cannot resume as specified

**Evidence.** The contract says that the outbox already accepts a server mark beyond its next number. The Client Local plan now rejects that exact input from both of its public operations and gives the outbox a local epoch that the wire messages do not carry. `.plans/plans/go-s0-contract/plan.md:533`, `.plans/plans/go-s0-contract/plan.md:733`, `.plans/plans/go-s1-client-local/plan.md:683`, `.plans/plans/go-s1-client-local/plan.md:698`, `.plans/plans/go-s0-contract/draft/link.proto:190`.

**Failure.** Keep the machine's enrollment and empty its event folder. Its next number is 1, and the server reports 40. Both `Ack(40)` and `Reconcile(40)` fail while preserving next number 1. Reconnecting repeats the condition, and sending event 1 makes it a duplicate. Confirmed live by checking the example against the specified comparison.

**Fix.** Settle one outbox lifetime and reconciliation contract in both plans before freezing the message. Either negotiate and authenticate a new outbox epoch with a fresh sequence, or specify a separately authorized, atomic rebase for an empty outbox with a known identity. Name the local API and the client and server owners. Preserve strict bounds on normal acknowledgements and test cold recovery and a crash during the reset.

### logic-r2 02 — BLOCKER — C11 — A database restore turns lost effects into acknowledged effects

**Evidence.** The normal rule commits the effect with the event mark. The restore rule raises that same mark from a claim in `Hello`. The claim has no outbox epoch, and the signed proof text covers the identifier and nonces only. The check expects the restore to succeed when facts are refreshed. `.plans/plans/go-s0-contract/plan.md:346`, `.plans/plans/go-s0-contract/plan.md:535`, `.plans/plans/go-s0-contract/plan.md:733`, `.plans/plans/go-s0-contract/draft/link.proto:199`, `.plans/plans/go-s0-contract/draft/link.proto:213`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:288`.

**Failure.** The client acknowledged through 42 and deleted those files. The database is restored to mark 40, and the server moves its mark to 42 with neither effect. A later acknowledgement now treats 41 and 42 as consumed. Refreshing the two current report types can restore today's display, but the same rule also applies to later event types whose effects cannot be regenerated. A behind mark alone also provides no evidence that this was a restore of the same history.

**Fix.** Distinguish the durable applied position from recovery state. Fail closed on divergent histories, report the missing interval and require a defined reset or an authoritative replay. If snapshots repair wave-one reports, limit the repair to those reports and define when it completes. Scope positions to the authenticated machine and outbox epoch, and accept reconciliation input only after proof and within the stream-install transaction. Bind the accepted reconciliation parameters to the authenticated exchange. Align Client Local, Server Link and Server Store on the stored epoch and marks.

### logic-r2 03 — BLOCKER — D7 — Removal and stream replacement have no shared commit fence

**Evidence.** A stream becomes current after a fresh read. Removal commits before the server closes the current stream. The patch states the exclusivity invariant but names no atomic decision shared by installation, removal and commits from an existing stream. `.plans/plans/go-s0-contract/plan.md:523`, `.plans/plans/go-s0-contract/plan.md:528`, `.plans/plans/go-s0-contract/plan.md:712`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:114`.

**Failure.** Stream B verifies proof and reads a live record. Removal then commits and closes stream A. B can still install from its earlier read and issue a connection token. Separately, an event or result from A that passed a current-stream check before B installed can commit after B becomes current unless the same fence is checked at commit. Closing the socket cannot revoke either write.

**Fix.** Server Machines and Server Link must use a single per-machine authority serialized with the store transaction. Make stream install conditional on the still-active identity. In the same authority, condition event and result commits, presence updates and token acceptance on the current stream generation. Invalidate that generation at removal commit. Prove both orders with barriers around the reads and commits.

### logic-r2 04 — BLOCKER — plan — The mandatory gate calls a script before its work item exists

**Evidence.** The new rule runs both checks on the exact commit of every item from 2 to 8. Item 7 creates the Go check after item 2 and can run beside items 3 to 6. The epic requires a passing report on the exact work-item commit. `.plans/plans/go-s0-contract/plan.md:266`, `.plans/plans/go-s0-contract/plan.md:598`, `.plans/epics/go-rewrite/epic.md:69`.

**Failure.** Item 2 is ready to merge. Its clean commit has the generated packages and lacks `scripts/check-go.sh`. The mandatory Go check fails at command lookup and item 7 depends on item 2. Confirmed live that the current repository has no tracked copy of the script.

**Fix.** Run the contract check for items 2 through 8. Begin gating on the Go script with the first commit that contains it and run both on the sprint merge. If item 2 needs equivalent Go checks, spell out the direct commands in its own gate.

### logic-r2 05 — MAJOR — R6 — The expiry estimate rejects eligible commands and lacks a shared receipt clock

**Evidence.** The server samples its clock after proof. The client adds elapsed time starting from before that sample. The post-build check expects eligible commands to run. Client Local independently gates `Begin` and prunes receipts using its default wall clock and a persisted high-water mark. `.plans/plans/go-s0-contract/plan.md:324`, `.plans/plans/go-s0-contract/plan.md:736`, `.plans/plans/go-s1-client-local/plan.md:756`, `.plans/plans/go-s1-client-local/plan.md:764`.

**Failure.** In milliseconds, proof is sent at 0, the server samples 200, and the command reaches the client at 260 with expiry 400. The estimate is 460 and the client stores a final expiry while the server would still permit the start. Confirmed live by arithmetic. A forward server clock adjustment after the sample can also put the estimate behind the server. Separately, with Client Local's default clock, a machine whose wall clock is ahead can reject a command that the link cleared or prune a receipt before the server's collection window has closed.

**Fix.** Specify the expiry boundary and clock assumptions together. If the client uses a conservative estimate, define the uncertainty window and its retryable result, and test that boundary rather than promising every eligible command starts. Specify the response to a server clock discontinuity. Give Client Link and Client Local one explicit time policy for starting, collecting and pruning, including startup before a server sample and migration from a persisted local-clock mark.

### logic-r2 06 — MAJOR — C10 — A verified challenge does not establish the later refusal

**Evidence.** The signature proves the peer at the challenge phase. The same terminal code can then come from an absent record or a proof failure. The code table gives `failed_precondition` terminal meaning regardless of phase, while a package value rule permits `invalid_argument` for the same unsupported revision. The transport protection decision is open. `.plans/plans/go-s0-contract/plan.md:123`, `.plans/plans/go-s0-contract/plan.md:509`, `.plans/plans/go-s0-contract/draft/link.proto:9`, `.plans/plans/go-s0-contract/draft/link.proto:36`, `.plans/plans/go-s0-contract/draft/link.proto:213`.

**Failure.** If a healthy machine gets a generic authentication failure after a valid challenge, the client cannot tell whether the server made a final registry decision, rejected the proof, or a transport intermediary returned the status. It exits and requires a new join. A removed machine can also be closed immediately after the challenge, while the client is still trying to send proof. A client that handles that send failure as a broken connection can miss the receive-side refusal and loop.

**Fix.** Define exactly which authenticated, attempt-bound terminal response establishes an absent or removed identity. Authorize terminal outcomes only over a channel authenticated for that response, or authenticate them directly. Keep proof failure and infrastructure errors distinct. Specify send and receive completion so a final response can be read after a failed send. Give unsupported revision explicit precedence and phase semantics. Define a positive normal wait for `aborted` and hand its retry behavior and the terminal exit status to Stack Client. If one credential may legitimately be active from two processes, define which process may reclaim it.

### logic-r2 07 — MAJOR — C11 — An invalid durable report blocks all later reports

**Evidence.** Unknown reports advance the mark. Known reports that fail validation close the stream and preserve the mark. The client keeps the numbered report until acknowledgement and retries `invalid_argument` with its files intact. `.plans/plans/go-s0-contract/plan.md:343`, `.plans/plans/go-s0-contract/plan.md:518`, `.plans/plans/go-s0-contract/draft/events.proto:58`.

**Failure.** Event 41 is durable with a workspace string over the new byte limit. The server rejects 41 and stays at 40. Every connection then replays 41 first. The client can collect newer, valid reports and never deliver them. Reusing or dropping 41 locally would violate the numbering rule.

**Fix.** Validate a report before reserving the sequence. Specify a repair for a persisted rejected head from an older client or changed validator. For wave-one snapshots, define a durable rejection for the exact next number that records the lost report, produces a visible diagnostic and permits a fresh snapshot. Separate that from an invalid envelope or number, which should stop for repair. Give duplicate, gap and report validation one precedence.

### logic-r2 08 — MAJOR — C5 — A revoked token can still qualify for replay

**Evidence.** The replay predicate omits revocation. The following refusal list applies to every other request. Revoking an approved machine's used token leaves its machine in place. The retry limit also depends on expiry, which the token format does not carry to the command-line client. `.plans/plans/go-s0-contract/plan.md:446`, `.plans/plans/go-s0-contract/plan.md:479`, `.plans/plans/go-s0-contract/draft/link.proto:55`, `.plans/plans/go-s0-contract/draft/link.proto:114`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:382`.

**Failure.** A token bound to the same key still has an approved machine after revocation. The replay rule returns the first reply, and the refusal rule expects a denial. During an outage an invocation with only the token text cannot know how much of its lifetime remains. A transaction alone also leaves a choice between two concurrent first uses unless the token is claimed conditionally.

**Fix.** State that revocation and expiry at the server clock take precedence over replay. Serialize claim, replays and revoke on the token row, and serialize revoke against approval on the machine row with a defined lock order or equivalent conditional updates. Name the exact lost-reply exception to C5 in the plan and user-flow check. Give the client a fixed retry budget or an expiry bound it actually receives, and retain its key when the commit status is uncertain.

### logic-r2 09 — MAJOR — C7 — Approval, live transport and readiness have contradictory transitions

**Evidence.** One rule marks a machine online at `Welcome`. A second keeps an approved machine joining until both reports arrive on its current stream. Owner options A and B bar at least one of those reports until approval, but the trigger for sending both is a `Welcome` at which both are allowed. The existing website distinguishes waiting records from approved records. `.plans/plans/go-s0-contract/plan.md:133`, `.plans/plans/go-s0-contract/plan.md:351`, `.plans/plans/go-s0-contract/plan.md:356`, `.plans/plans/go-s0-contract/plan.md:526`, `src/contracts/proto/lgtm/v1/clients.proto:66`, `src/frontend/src/server/sync.ts:25`.

**Failure.** An approved client reconnects and sends one report. One builder publishes online and another publishes joining. With option A or B, a connected waiting client that is approved via `SetState` can remain joining forever because there is no second `Welcome`. A server can also send work before that state change reaches the client, because other messages can follow `Welcome` in any order.

**Fix.** Use a single website state projection. Define transport live at successful installation, waiting before approval, joining until both current reports, online thereafter and offline when an approved stream ends. Apply the approval policy to token scope and queued events. On approval, require a fresh stream or an explicit transition that enables telemetry, reissues any newly authorized token and orders eligibility before work. Choose that mechanism before freezing any affected fields.

### logic-r2 10 — MAJOR — C6 — The token address has inconsistent writers and no reliable source for the copied command

**Evidence.** This patch requires one configured public address for both token makers and rejects a different command-line address. Stack Client requires its command to use the supplied address and installs against loopback even when a public address is configured. The website's current token adapter has no address from the reply. The dialog uses its selected server label, and the contract package exports only the website-side generated files. `.plans/plans/go-s0-contract/plan.md:452`, `.plans/plans/go-s0-contract/plan.md:717`, `.plans/plans/go-s3-stack-client/plan.md:136`, `.plans/plans/go-s3-stack-client/decisions.md:36`, `src/frontend/src/server/sync.ts:117`, `src/frontend/src/screens/service.tsx:410`, `src/contracts/package.json:8`.

**Failure.** Configure public HTTPS and run the local installer. Its command asks for loopback but the single-address rule yields the public address. The client refuses the mismatch. A browser using a different valid alias likewise has no way under the stated handoff to put the token's exact address in the copied command.

**Fix.** Agree on named permitted issuance contexts with Stack Web, Server Machines and Stack Client. Decide whether the installer uses the configured public endpoint or has a separate configured loopback context with its own pin policy. For the browser, specify how the same canonical address reaches the command, for example a public response field or the token-only form of the command if the owner approves it. Name who updates issuance, enrollment, the page, the mock and installer together.

### logic-r2 11 — MAJOR — D16 — Restart completion depends on an unspecified send failure and exit status

**Evidence.** The command reports done after a durable receipt and before ending the program. The Stack Client plan only restarts nonzero exits and reserves 75 for a requested restart and 0 for a permanent stop. The contract's handoff says only that the receipt is durable before the send. `.plans/plans/go-s0-contract/plan.md:335`, `.plans/plans/go-s0-contract/plan.md:710`, `.plans/plans/go-s3-stack-client/decisions.md:25`, `.plans/plans/go-s3-stack-client/plan.md:230`, `.plans/plans/go-s3-stack-client/plan.md:269`.

**Failure.** The receipt commits and sending the result times out. A client that stays running after the error returns the final receipt on replay and never restarts. A client that ends normally is also left stopped by either service manager. The server can hold a successful result while the user sees the machine offline.

**Fix.** Define the durable receipt as acceptance of the restart. After it commits, attempt the result send for a bounded time and exit with the agreed supervised restart status regardless of delivery. Retry the result from the receipt in the next process. Reserve the terminal stop status for authenticated terminal outcomes. Test both service managers and a lost result.

### logic-r2 12 — MAJOR — D3 — Token creation silently bypasses the shared retry contract

**Evidence.** The design says a repeated mutating request happens once and defines the keyed reply policy. This patch makes every `CreateClientToken` call mint a new credential, even when the same key is supplied. The Sign-in plan assumes the join-token endpoint rejects the key under its credential-returning exception. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:112`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:176`, `.plans/plans/go-s0-contract/plan.md:281`, `.plans/plans/go-s0-contract/draft/existing-files.diff:15`, `.plans/plans/go-s2-server-signin/plan.md:290`.

**Failure.** An authenticated caller supplies a request key, loses the reply and repeats that key. The contract plan creates a second live token. The Sign-in plan refuses both calls. Both avoid saving the secret, but they expose different behavior on the same route.

**Fix.** Obtain an explicit exception to the keyed retry rule or define a secret-safe replay design. For a nonreplayable endpoint, refuse a present key before minting, name the exact code and include the behavior in the Server Machines, Store and Sign-in handoffs. Test keyed refusal and ambiguous unkeyed retry without ever persisting plaintext token replies.

## Answers to the assignment

1. **Each patch.** The table records the disposition of every patch surface and every group in the round-one register. Group numbers below refer to `.plans/plans/go-s0-contract/attack/round-1-findings.md:16`. Patch names follow its list at `.plans/plans/go-s0-contract/attack/round-1-findings.md:120`.
2. **Unreviewed mechanisms.** A claimed event mark has no proof of the durable history it advances and no shared epoch. A challenge proves the server at that phase and does not by itself prove the terminal status. The code table needs scoped terminal outcomes, and a normal `aborted` wait needs a positive bound and a replacement policy. See findings 01, 02 and 06.
3. **Rejected findings.** Nothing found. The register records zero rejected findings, and the outcome column contains no rejected row. `.plans/plans/go-s0-contract/attack/round-1-findings.md:16`, `.plans/plans/go-s0-contract/attack/round-1-findings.md:78`.
4. **Writers and readers.** The table following the patch dispositions identifies the missing handoffs and the ones that are complete for the current scope.
5. **Interacting rules.** The decision tables below cover the contradictory and empty cells for events, streams, enrollment and commands. Findings 01 through 12 name the consequences.
6. **Open findings marked fixed.** Groups 1 and 2 still have a history divergence and an outbox interface break, group 3 still lacks a removal fence, group 5 still lacks a reliable terminal policy, group 6 still has ambiguous revoked replay, groups 7 and 8 still have command failures, groups 10 and 11 still disagree on issuance, and group 17 still requires an unavailable check. The other four owner decisions retain their stated disposition.
7. **Questions touched by the patches.** All eight logic questions apply. The numbered lens answers below tie them to the specific rules, boundary states and tests.

### Disposition of every patch

| Patch surface | Result of the review |
|---|---|
| 1. Overview | The report now identifies the revision 1 website behavior break and keeps the four owner items visible. The claim that they cost nothing before sprint 3 fails when an option changes frozen fields or Stack Web's sprint 2 route. See `.plans/plans/go-s0-contract/plan.md:104` and `.plans/plans/go-s0-contract/plan.md:129`. |
| 2. Key decisions | The normal event prefix and durable command receipt answer parts of groups 1, 7 and 8. The clock, remove-then-close and newest-stream summaries omit the failure cases in findings 02, 03, 05 and 11. |
| 3. Owner points | Groups 9, 12, 13 and 15 are accurately surfaced as decisions. Point 1 option B still breaks the old page's only use despite keeping revision 1, whose rule is based on behavior. Points 2 and 3 have a field or transport dependency before the freeze, and point 4 changes the copied command if selected. See `src/contracts/README.md:29` and `.plans/plans/go-s0-contract/plan.md:119`. |
| 4. Public names | The optional load fields and interrupted outcome have clear protocol values. The event mark, terminal code, token and address still lack agreed readers or failure semantics. See findings 01, 02, 06, 10 and 12. |
| 5. Descriptor rule 7 | Added reserved ranges and names now change the descriptor comparison for every described message and enumeration. This part of group 5 holds. |
| 6. Item 2 | A valid probe, a clean baseline and the same run without the guard distinguish an untracked output from a lint or generation failure. The group 17 runner rule fails on its earliest commit. Finding 04. |
| 7. Item 3 | The website field is additive and the first position has a defined zero. The revision problem is now acknowledged. Token creation retries and revocation still need aligned authority. Groups 6, 9, 14 and 22. Findings 08 and 12. |
| 8. Item 4 | The goldens distinguish an absent load from zero and pin value 5 for interruption. The normal ordered prefix closes the original crash window. Restore, invalid persisted reports, expiration and readiness still have gaps. Groups 1, 2, 7, 8, 16, 20 and 21. Findings 01, 02, 05, 07, 09 and 11. |
| 9. Item 5 | Storing a key before enrollment and defining token validity improve the lost-reply path. A revoked replay, a locally unknowable retry deadline and inconsistent address selection leave groups 6, 10 and 11 open in those cells. Findings 08 and 10. |
| 10. Item 6 | The handshake phases, timeouts and named codes improve the happy path. The terminal status, installation fence, zero and missing values, event negotiation and join-to-ready transition remain incomplete. A test of comment placement accepts changed semantics. Groups 1 to 5, 13 and 19. Findings 01 to 03, 06 and 09. |
| 11. Item 8 | The named absence commands now use `! grep -q`. The description of the command still names `grep -c`. The current README also has a second blanket missing-session rule, at line 39, which the anchored pattern in item 8 does not catch. Group 18 is only partly closed. `.plans/plans/go-s0-contract/plan.md:630`, `.plans/plans/go-s0-contract/plan.md:640`, `src/contracts/README.md:39`. |
| 12. Done criteria | The guard test documents its expected exit status and the revision check covers sprint 0. Passing the comment test proves comment placement and selected words. The criterion overstates that result as proof that every rule is written. `.plans/plans/go-s0-contract/plan.md:650`. |
| 13. Risks | The delayed acknowledgement and replaced-stream cases are named. The mitigation omits history divergence, commit races and clock steps. The assumption about the late owner gate is conditional on the chosen design. Findings 02, 03, 05 and 06. |
| 14. Guardrails | The revision timing is clear for sprint 0. The comment guard protects coverage and selected terms in the link. It accepts an incorrect sentence immediately above every declaration. `.plans/plans/go-s0-contract/plan.md:559`, `.plans/plans/go-s0-contract/plan.md:822`. |
| 15. Team agreements | Every row names a writer and reader. Several omit the other component holding the same authority or a dependent reader. See the handoff table. |
| 16. Post-build checks | Lost replies, a late old stream, service restarts and a real proxy are concrete integration cases. Checks 3, 4 and 6 need the corrected expected results of findings 01, 02, 05, 09 and 11. |
| 17. Traceability | All added decisions and the register are mapped. The word covered in the table records a location and does not settle the cross-plan contradictions. `.plans/plans/go-s0-contract/plan.md:802`. |
| 18. Attack log | The totals match the register. The claims of closure need the findings in this report folded into the next outcome. `.plans/plans/go-s0-contract/plan.md:854`. |
| Drafts | The five files carry the declared wire changes. The event draft still identifies its mark as the highest consumed number, and the handshake draft advances it without observing those effects. The command draft requires current state before commands, while its missing-state fallback conflicts with the package-wide requirement that `Welcome.state` be present. The draft does not spell out the plan's waiting traffic policy. `.plans/plans/go-s0-contract/draft/events.proto:15`, `.plans/plans/go-s0-contract/draft/link.proto:17`, `.plans/plans/go-s0-contract/draft/link.proto:199`, `.plans/plans/go-s0-contract/draft/commands.proto:104`. |
| Decision rows 38 to 53 | Rows 38 and 39 need a single history policy. Rows 40 and 41 need the command clock and exit rules. Rows 42 to 45 need the commit fence and terminal states. Rows 46 and 47 need enrollment precedence and endpoint ownership. Rows 48 and 51 state their zero and absent meanings. Row 49 needs the readiness transition. Row 50 needs the request-key exception. Row 52 is conditional on owner choices. Row 53 fixes the probes and does not resolve the item gate. `.plans/plans/go-s0-contract/decisions.md:45`. |
| Scouting | The field and frontend references match their named locations. The assertion that `Ack` raises the next number no longer matches the Client Local source plan. The comment mutation and guard runs establish the asserted narrow property, while leaving later runtime behavior open. `.plans/plans/go-s0-contract/scouting.md:33`, `.plans/plans/go-s0-contract/scouting.md:83`, `.plans/plans/go-s1-client-local/plan.md:698`. |

### Handoffs for changed names and meanings

| Name or rule | Writer, reader and missing agreement |
|---|---|
| `Hello.acked_event_number`, `EventAck.through`, `Welcome.last_event_number` | Client Local persists the epoch and local mark. Client Link writes Hello and reconciles the two replies. Server Link reads Hello and writes both replies. Server Store persists the scoped applied mark with the effects. The listed mechanism omits a common epoch and a compatible reset interface. |
| `COMMAND_OUTCOME_INTERRUPTED`, attempt and expiry | Client Local recovers the receipt. Client Link maps and labels it. Server Link accepts it and records final state. Server Machines displays that state. Both Client teams must agree on the clock for receipt operations, and Server Link and Store must agree on the atomic result-versus-expiry transition. |
| `Heartbeat` load, machine facts, `Client.address` and `hosts_server` | Client Link sends optional measured values. Server Link receives them. Server Machines projects presence and facts. The website currently maps both absent load and 0 to 0 at `src/frontend/src/server/sync.ts:22`. The installer and privileged token command set the issuance mark, which Server Machines reads. The address projection needs the Stack Web decision about the source observed behind the proxy. |
| Link codes, revision, values and `Lgtm-Link-Token` | Server Machines implements enrollment and Server Link implements Open. Client Enrollment and Client Link consume their respective statuses. Both link peers validate the shape. Server Sign-in owns dispatch, Stack Web owns forwarding and Stack Client consumes process exit status. The later side calls consume the token, so their acceptance must use the same stream and approval authority. The code agreement currently names only Server Link as writer. |
| `ReconnectClient` and `Restart` | Website and mock are part of compatibility. Server Machines submits the command, Server Link sends it, Client Link accepts and exits, and Stack Client observes the exit. The agreement must include the service manager and the one user-facing compatibility rule. |
| `CreateClientTokenResponse.token`, its expiry and its address | Server Machines and the command issue it. The website and installer pass it on. Client Enrollment reads the address, pin and secret, while Client Link uses the persisted address and pin. Server Sign-in and Store enforce the request-key policy. An opaque website token still needs a named source for its command address. |
| `FeedPosition` and description names | Server Store writes positions and the watch layer publishes them. The existing website and mock tolerate absence or ignore the added field. Contract owns the goldens and both code generators consume the descriptors. Nothing further found for this sprint. |

### Decision tables for interacting rules

Let M be the server's durable event mark and N the received event number. The comparisons assume a valid positive number.

| State | Written behavior and missing or contradictory cell |
|---|---|
| N ≤ M | Acknowledge M. The general validation rule also rejects a malformed old report. The precedence is empty. |
| N = M + 1, recognized and valid | Commit the effect and the new mark together. A crash before commit preserves M, and a crash after commit turns a retry into a duplicate. |
| N = M + 1, absent or unknown report | The mark advances without an effect by design. Future report compatibility depends on knowing whether discarding this kind is allowed. |
| N = M + 1, recognized and invalid | The mark remains unchanged. Reconnect recreates the same rejected head. Finding 07. |
| N > M + 1 | End with `out_of_range`. The invalid-report rule leaves precedence empty for an invalid gap. |
| Server mark lower, equal or higher than the local acknowledged mark | Lower currently advances the server and can skip lost effects. Equal can resume. Higher can consume items already allocated locally, but at or beyond local `Next()` Client Local refuses. Findings 01 and 02. |

| Stream and approval state | Written behavior and missing or contradictory cell |
|---|---|
| Proof verified, registry read succeeds, then removal or replacement commits | Current-stream authority can change before installation or a later write commits. Finding 03. |
| Unverified challenge or unavailable registry | Keep the saved key and retry. Unknown syntactic input and unsupported revisions need an explicit code precedence. |
| Verified challenge followed by generic `unauthenticated` | The table stops for all sources of that status. The terminal registry decision remains unproven. Finding 06. |
| `aborted` after replacement | Retry with the key. A maximum of 15 seconds permits zero delay and gives no stable owner when both processes continue. |
| Unapproved and connected | The draft grants events and connection credentials pending the owner choice. Only the heartbeat and state paths are common to all options. |
| Approved and connected, fewer than two current reports | `Welcome` says online and the facts rule says joining. |
| Approved and connected, both current reports | Online is consistent if the stream remains current. An old stream must have no effect. |
| Approved and disconnected | Offline. A stream that failed installation must leave the current stream and projection alone. |
| Missing `Welcome.state` | Required by the package and treated as waiting by the commands rule. Precedence is contradictory. Unknown approval can be safely waiting with a valid revision. |

| Token state with otherwise valid input | Written behavior and missing or contradictory cell |
|---|---|
| Unused and live | One transactional claim, with exactly one winner required for concurrent uses. |
| Used, unexpired and unrevoked, same key and existing machine | Replay the first reply. Make the C5 exception for this same enrollment explicit. |
| Used, different key or removed machine | Refuse. |
| Expired at the server's clock | Refuse. A client retry deadline for outage is unspecified when no reply is available. |
| Revoked, same key, machine already approved | The replay predicate succeeds and the refusal policy says to deny. Finding 08. |
| Database unavailable or outcome of a commit is uncertain | Retry with the same key. A known rollback changes nothing. An unknown outcome is resolved by the transactional replay predicate. |

| Command state | Written behavior and missing or contradictory cell |
|---|---|
| No receipt, before expiry | Start after writing the receipt. The early-expiry interval in finding 05 contradicts this cell. |
| No receipt, at or after expiry | Return expired without starting. The draft sets this boundary inclusive. |
| Running receipt, any expiry | Finish once and answer under the newest observed attempt. A racing replacement must fence result acceptance. |
| Final receipt, any expiry | Replay the stored outcome under the received attempt. A server that already finalized keeps its result. |
| Current stream and current attempt race with the server's 24-hour expiry write | The plan defines both writers and leaves their atomic precedence empty. Specify one conditional final transition under the same row lock, plus whether a result at the deadline is accepted. |
| Old stream or old, future or duplicate attempt | Ignore its result. Attempt exhaustion is forbidden from wrapping, but the terminal transition is unspecified. |
| Restart receipt committed, send fails | Outcome is already recorded. Process exit remains unspecified for the failed-send branch. Finding 11. |

## Answers to the logic lens

1. **Ordering and numbers.** Store commits order the feed, and gaps in a filtered feed are expected. Local allocation orders event files and the server consumes exact successors. Decision 39 does not preserve this invariant across a divergence. Attempts and standing revisions start at 1 and promise no wrap, but exhaustion has no outcome. Client Local's sequence cap is lower than the wire type's and must be shared by the contract. Standing ties keep the old state, which leaves a client holding a later revision when the newly supported database restore brings back older standing. The plan needs an explicit restore policy for that state as well. See `.plans/plans/go-s0-contract/draft/common.proto:25`, `.plans/plans/go-s0-contract/draft/commands.proto:104`, and `.plans/plans/go-s1-client-local/plan.md:632`.
2. **Failure policy.** A pre-commit crash retries and a post-commit duplicate is normally idempotent. Database read failure returns unavailable. In-flight removal, history divergence, invalid queue heads, command clock discontinuity, an uncertain Join commit and a failed restart-result send need the explicit repairs above. After Welcome, the server measures heartbeats on its elapsed-time clock and the client measures receipt silence.
3. **Rule interactions.** The four tables identify contradictory or empty cells for malformed duplicates, divergent marks, missing standing, revision validation, revoked replay, readiness and the final-command race.
4. **Zero values.** Zero event number, command attempt and state revision are invalid. Zero event mark means no acknowledged events. A present zero feed position is distinct from an absent position. Absent load is distinct from 0 on the wire. Unspecified or unknown approval becomes waiting and unspecified or unknown result becomes failed when accepted. Missing Welcome state has two behaviors. Timestamp presence in the draft is explicit for expiry and server time, but `Event.happened_at` has no corresponding present-and-valid rule. `.plans/plans/go-s0-contract/draft/link.proto:9`, `.plans/plans/go-s0-contract/draft/events.proto:34`.
5. **Concurrent actors.** Old and new streams, removal and install, token claim and revoke, approval and revoke, and result acceptance and expiry each need a shared serialization point. Two local outbox operations do have one common lock. `.plans/plans/go-s1-client-local/plan.md:697`.
6. **Risk mutations.** Item 3's Watch check catches removing position, and its golden catches narrowing its type. Item 4's goldens catch changed attempt width and missing standing revision. Item 5's golden catches the moved field, and the example rejects a changed prefix. Item 6's signed example catches swapped nonces, its golden catches the missing mark and the comment scan catches deleting the only comment on Restart. A reversed expiry sentence, acknowledgement before commit or an unauthenticated terminal decision can pass the corresponding contract tests. These are assertions about the tests as specified. Typing and signature checks do not observe the future link behavior.
7. **Cheapest wrong implementations.** For item 1, an incorrect patch-level Go floor can pass the version-prefix check when the installed toolchain is already at the intended version. For item 2, omit `go test ./...` from the contract check. The probes still test generation, and the item has no Go test that distinguishes the omission. For item 3, describe an incorrect position meaning while preserving its type and number. For item 4, state that an acknowledgement may precede commit. For item 5, replay revoked tokens using the written replay predicate. For item 6, keep all comment lines and status words while reversing a behavioral rule. For item 7, omit vet and test while keeping formatting and module checks, which still exposes the two negative probes. For item 8, include the tested words while leaving the second blanket session requirement intact. Items 1 and 7 are cited only for their interaction with the new gate. `.plans/plans/go-s0-contract/plan.md:224`, `.plans/plans/go-s0-contract/plan.md:248`, `.plans/plans/go-s0-contract/plan.md:559`, `.plans/plans/go-s0-contract/plan.md:608`, `.plans/plans/go-s0-contract/plan.md:634`.
8. **Shapes that rule out bad states.** Scope a number to a verified machine and event epoch, and give applied and repaired positions separate meanings. Serialize authority with a current-stream generation. Derive displayed presence from approved identity, live transport and current reports. Make terminal handshake outcomes typed and attempt-bound and final command writes conditional on a pending row.

## Post-build checks

| Check | Scenario | Expected result and evidence |
|---|---|---|
| L2-P1 | Run the real client and server against a disposable PostgreSQL database. Hold an old stream's event or result before commit, then replace or remove its identity. Release the old operation. | The new generation owns the record. A removed generation can neither install nor write after the fence. The database and Watches record no post-fence mutation by the old stream. Server Link, Server Machines and Store. |
| L2-P2 | Reopen an empty outbox for an existing identity. In a separate run restore the server to two events behind. Test the agreed reset during a client crash. | The negotiated recovery state is observable. A missing interval never appears as applied, held reports are retained, and current reports are restored as specified. Compare durable client state and database rows before and after restart. Client Local, Client Link and Server Link. |
| L2-P3 | Introduce a valid future expiry with a delayed handshake, an equality at expiry, a clock step, and a client clock offset. Race a final result against collection expiry. | The agreed conservative or exact rule is visible at each boundary, no stale command starts, and one final outcome wins. Check the command row and durable receipt. Client Link, Client Local and Server Link. |
| L2-P4 | Close a valid stream between persisting the Restart receipt and sending its result. Separately remove a live client while its notice is undelivered. | A supervised process comes back exactly once for Restart and replays its result. A removed identity stops with the agreed status after a trusted refusal. Compare supervisor events and the machine's observed connections. Stack Client and both Link teams. |
| L2-P5 | Race two enrollment keys against one token. Verify both orders of approval and revocation, and run the copied website command while the browser uses an alias and the local installer uses its configured endpoint. | Only one enrollment is created. Replay and revoke obey the stated order. The copied and installed commands use the endpoint in their token. Check the database, actual connection and Machines page. Server Machines, Client Enrollment, Stack Client and the website owner. |

## Grade

4/10. Four blockers, eight majors and zero minors.
