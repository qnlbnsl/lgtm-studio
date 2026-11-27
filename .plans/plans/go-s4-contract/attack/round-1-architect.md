REVISE

An allowed cross-origin website can discover the repository service and then have its browser block all three repository calls because the handoff names handler registration and omits the separate preflight table.

The planner, test writers, builders, and architect reviewer are all OpenAI workers in separate sessions. I reviewed plan version 1 at `19873537`. The working plan and the frozen review copy had the same digest.

## Findings

### architect-r1 01 — MAJOR — D11

**Register the repository procedures for browser preflight.** The Projects handoff specifies the generated handler, session gate, write deadline, and advertisement. The current router stores dispatch registrations in `r.web`. Its separate Cross-Origin Resource Sharing preflight path checks `webProcedures`. The existing request-key header is already allowed. Evidence by reading: `.foundry/go-rewrite/results/plan-go-s4-contract.md:42`, `src/server/internal/api/server.go:58`, `src/server/internal/api/server.go:88`, `src/server/internal/api/server.go:173`, `src/server/internal/api/server.go:245`, and `src/server/internal/api/server.go:309`.

**Failure.** An allowed website on a different origin signs in and discovers `ProjectService`. If the builder follows the named dispatch and advertisement steps, the browser's preflight gets 403 and the requests never reach their handlers. The default same-origin stack would miss this.

**Fix.** In the Projects handoff, require dispatch and preflight registration for all three generated procedure constants and keep them behind the session gate. Require an allowed-origin preflight for each procedure, including one with `Lgtm-Request-Key`, followed by protected calls through the real router.

Design. Server Projects owns the fix. The separate route tables are confirmed by reading. The risk score is 120 from severity 6 for a blocked core flow, fallout 5 for a route fix, and probability 4 for the supported cross-origin setup.

### architect-r1 02 — MAJOR — R4

**Keep the request key reserved in the store until the folder operation's replay window ends.** The plan gives the operation a durable request identity after the generic receipt is pruned. `WriteOnce` locks and looks up a key by principal and key before calling the Projects write. The procedure is part of the digest. The Machines keyed routes use the same owner principal. Evidence by reading: `.plans/plans/go-s4-contract/plan.md:223`, `src/server/internal/store/receipts.go:69`, `src/server/internal/store/receipts.go:81`, `src/server/internal/store/receipts.go:90`, `src/server/internal/store/receipts.go:100`, `src/server/internal/store/prune.go:267`, and `src/server/internal/machines/records.go:155`.

**Failure.** An open `AddRepository` with key K outlives generic receipt pruning while an event is missing. Another keyed web call for the owner, such as a machine rename, can now claim K. The original add retry then gets `already_exists` before Projects can find the saved operation. Its retained request identity cannot enforce the promised replay.

**Fix.** Assign the Store and Projects handoff a store-owned reservation for the existing principal and key, bound to the operation's procedure and request fingerprint until replay ends. Use the existing key serialization so a conflicting procedure cannot claim that key after pruning. Require a real database test that prunes, attempts a different keyed procedure, then repeats the original intent and sees one original command.

Design. Server Store and Projects own the fix. The store's lookup order and shared key scope are confirmed by reading. The risk score is 60 from severity 6 for a broken operation retry, fallout 5 for recovery requiring operator help, and probability 2 for pruning followed by key reuse.

### architect-r1 03 — MAJOR — R7

**Give the owner a recovery signal when only the server can detect lost history.** The plan refuses a contradictory run announcement before Welcome using `invalid_argument` and keeps the existing retry behavior. The client logs this generically and receives neither the server mark nor the staged watermark. The local recovery check runs after Welcome. Evidence by reading: `.plans/plans/go-s4-contract/plan.md:14`, `.plans/plans/go-s4-contract/plan.md:149`, `.plans/plans/go-s4-contract/plan.md:151`, `src/client/internal/link/conn.go:133`, `src/client/internal/link/conn.go:224`, `src/client/internal/link/session.go:147`, and `src/client/internal/link/session.go:257`.

**Failure.** The server mark is 8 and a staged result waits for 10. The owner restores a structurally sound client snapshot with event 9 and next number 10. The client cannot inspect a Welcome because the server rejects the new boundary first. It redials indefinitely with a generic error while the command stays open. The overview's stop-and-recover behavior gives the owner no way to tell this state from another bad frame.

**Fix.** Keep the existing wire status, retry behavior, and durable fences. Specify a distinct, fixed owner-facing server diagnostic for a verified admission rejected because its history contradicts the committed mark or a staged watermark. Give the diagnostic the restore-or-remove instruction and keep it free of client-supplied text. State that this case keeps retrying until history is restored or the machine is removed. Require this exact stale-snapshot test in the server and stack handoffs.

Design. Server Link owns the diagnostic and Stack owns the recovery proof. The status path is confirmed by reading. The risk score is 120 from severity 6 for a stuck client, fallout 5 for operator recovery, and probability 4 for one bad restore.

### architect-r1 04 — MAJOR — C7

**Allow required replay while approval still waits.** The plan gates every command, including `SetState`, until old events are accounted for. The old contract permits a waiting machine to send Facts and tells the server how to consume other next-number reports. The built client only hands off any event after local approval, and a live approval is delivered through a queued `SetState`. Neither handoff explicitly changes this dependency. Evidence by reading: `.plans/plans/go-s4-contract/plan.md:138`, `.plans/plans/go-s4-contract/plan.md:143`, `src/contracts/proto/lgtm/link/v1/link.proto:155`, `src/client/internal/link/session.go:537`, and `src/server/internal/machines/records.go:78`.

**Failure.** A waiting machine reconnects with a retained Facts event numbered 1, a first event number of 2, and server mark 0. This backlog is allowed on the wire. The owner approves it after Welcome. The client waits for `SetState` before sending event 1 and the server waits for event 1 before sending `SetState`. Heartbeats can keep the stream open throughout.

**Fix.** Define and assign replay of the retained pre-run prefix before approval, with the existing waiting-machine report application rules. Continue to gate new work and new reports under the existing approval rules. Require a paired-program test using a waiting Welcome and a retained event, then approve on the same stream and observe the event acknowledgement before `SetState`.

Design. Client Folders and Server Link own the fix. The old sender gate and approval path are confirmed by reading. The risk score is 60 from severity 6 for stalled approval, fallout 5 for manual recovery, and probability 2 for a waiting machine with a retained prefix.

### architect-r1 05 — MAJOR — R6

**Cover the existing `SetState` result writer in the receipt rule.** The design requires the watermark of each result to survive a repeat. The plan promises the saved watermark but its client handoff names receipt replay generally and the new folder action specifically. The existing `SetState` fast path applies standing and replies before `Store.Begin` or `Store.Finish`. Evidence by reading: `docs/specs/2026-11-27-running-an-agent-design.md:88`, `.plans/plans/go-s4-contract/plan.md:145`, `.foundry/go-rewrite/results/plan-go-s4-contract.md:43`, `src/client/internal/link/commands.go:141`, `src/client/internal/link/commands.go:179`, and `src/client/internal/link/commands.go:299`.

**Failure.** A standing change is applied at watermark 10. The reply is lost before the server accepts it, and the process buffers event 11. On the next attempt a builder who adds the watermark to the common reply path sends 11 for the same result. The server can now wait on an event that the first reply never depended on.

**Fix.** Name `SetState` in the client handoff. Persist its first result and watermark before replying, while preserving the standing revision and expiry exceptions. Test replay after a lost result, another appended event, and a restart. Require the old watermark with the new attempt number. Apply the same receipt audit to the scan, restart, decline, and interrupted paths.

Design. Client Folders owns the fix. The fast path is confirmed by reading. The risk score is 64 from severity 4 for a wrongly extended wait, fallout 4 for possible recovery work, and probability 4 for losing a reply during an ordinary standing change.

### architect-r1 06 — MAJOR — R3

**Give Link and Projects one agreed Go boundary and a merge order.** The plan defers the server Go interface to Link. The handoff mentions routing reports and results, but it omits the identifier-returning enqueue and explicit transactional removal registration needed by Projects. The current `State.Enqueue` hides the identifier and implements `machines.CommandQueue`, whose callers expect only an error. The current machine removal calls `State.Forget`, which removes link rows. Evidence by reading: `docs/specs/2026-11-27-running-an-agent-design.md:81`, `docs/specs/2026-11-27-running-an-agent-design.md:252`, `.plans/plans/go-s4-contract/plan.md:287`, `.foundry/go-rewrite/results/plan-go-s4-contract.md:41`, `src/server/internal/link/outbox.go:21`, `src/server/internal/machines/link.go:22`, `src/server/internal/machines/records.go:67`, and `src/server/internal/link/presence.go:245`.

**Failure.** Projects can store `operation_id` and enqueue an action, but it cannot save the returned command identifier with the current interface. A direct signature change also breaks Machines and its fixtures. A report callback alone leaves Projects without a route to clean up a removed machine that had folders and no pending command.

**Fix.** Require the Link plan to publish a reviewed interface before Projects implements against it. Cover identifier-returning enqueue under the machine lock and hooks for settlement, report application, admission, run change, and removal. Keep the existing Machines interface through a wrapper or migrate its callers and fixtures in the same Link work item. Name the transaction that invokes each hook and require compilation of both owners at their handoff.

Design. Server Link and Projects own the interface. The current signature and removal path are confirmed by reading. The risk score is 60 from severity 6 for an incomplete core path, fallout 1 for a build-time handoff defect, and probability 10 when the consumer tries to reuse the existing interface.

### architect-r1 07 — MAJOR — plan

**Specify who normalizes Git remote identities and what equality means.** The plan says the client strips credentials and the server compares normal forms, but it defines neither the owner of canonicalization nor the equivalence rules. Its tests pin presence and byte limits. They do not pin a pair of differently spelled remotes that must compare equal. Evidence by reading: `docs/specs/2026-11-27-running-an-agent-design.md:202`, `.plans/plans/go-s4-contract/plan.md:155`, `.plans/plans/go-s4-contract/plan.md:200`, `.plans/plans/go-s4-contract/plan.md:225`, and `.foundry/go-rewrite/results/plan-go-s4-contract.md:42`.

**Failure.** Two checkouts on separate machines refer to the same origin with different letter case in the host name. One builder can treat the credential-free strings as canonical and another can fold the host name for comparison. The same add request either succeeds or rejects the known repository without a shared example deciding which result the contract promises.

**Fix.** Put the normal-form comparison under Projects and state that the Folders client supplies credential-free remote text. Define the supported address forms and the transformations that preserve identity. Pin equal and unequal pairs, including host case, case-sensitive path, and different remotes with the same first commit. Keep the absent and withheld cases explicit.

Design. Server Projects owns equality and Client Folders owns sanitization. The missing comparison rule is confirmed by reading. The risk score is 108 from severity 6 for a rejected attachment, fallout 3 for an affected owner, and probability 6 for an ordinary second checkout.

### architect-r1 08 — MAJOR — R9

**Carry the filter's sprint 4 inputs and placement into the frozen rule.** The plan says to screen folder facts and links the general outgoing filter in traceability. It leaves out the design's requirement that the same filter cover buffered text and client logs and know both the client key and the connection token in the forms the client holds. The Folders handoff only specifies remote sanitization and omission. Evidence by reading: `docs/specs/2026-11-27-running-an-agent-design.md:145`, `.plans/plans/go-s4-contract/plan.md:157`, `.plans/plans/go-s4-contract/plan.md:287`, and `.foundry/go-rewrite/results/plan-go-s4-contract.md:43`.

**Failure.** A credential is present in a local folder or branch name. A Folders implementation can strip remote credentials and enforce the byte and string rules without recognising that credential. It can place the path or branch in the durable report and send it to the server.

**Fix.** State the sprint 4 filter inputs and require one client-side path before buffer or log writes. Carry the fixed refusal for unsafe keys and omission for an unsafe remote into the Folders handoff. Require a paired fixture with planted client credentials in folder facts and inspect the local buffer, logs, and server reply. Leave agent output and harness sign-in filtering with the agent work.

Design and security integrity. Client Folders owns the fix. The scope omission is confirmed by reading. The risk score is 84 from severity 6 for credential exposure, fallout 7 for a secret already written, and probability 2 for a credential-bearing fact.

### architect-r1 09 — MINOR — R8

**Pin the fixed website refusal contract in a test.** Item 3 names tests for the descriptors, common watch shape, request-key comments, and a successful identity join. No named assertion pins the matrix of Connect statuses and sentences. The generic stack checks exercise a subset. Evidence by reading: `.plans/plans/go-s4-contract/plan.md:185`, `.plans/plans/go-s4-contract/plan.md:199`, `.plans/plans/go-s4-contract/plan.md:227`, and `.foundry/go-rewrite/results/plan-go-s4-contract.md:117`.

**Failure.** A builder maps a valid `FOLDER_REFUSAL_NOT_READABLE` to `invalid_argument` or copies the sentence for a missing folder. The schema, success fixture, request-key assertions, and the design's test for a non-repository can all pass.

**Fix.** Add a table-driven assertion for the fixed cases and their sentences. Assign the runtime mapping to Projects and the inherited session and watch errors to their existing owners. Include the two distinct `already_exists` cases and the terminal outcome distinctions beneath `deadline_exceeded`.

Design. Contract and Server Projects own the proof. The missing assertions are confirmed by reading. The risk score is 18 from severity 2 for a misleading refusal, fallout 3 for one owner, and probability 3 for a less common folder failure.

### architect-r1 10 — MINOR — R10

**Name the owner and command for the paired-program tests.** The handoff assigns the in-process checks to Stack and uses the existing program fixtures when available. The existing suite already uses `servertest.Start` and `clienttest.Join` and `Run`. The epic limits Stack edits to `deploy/`, and the earlier client plan assigns `src/linktest/` to the orchestrator. The existing fixture skips without `LGTM_TEST_POSTGRES`. Evidence by reading: `.foundry/go-rewrite/results/plan-go-s4-contract.md:45`, `.plans/epics/go-rewrite/epic.md:135`, `.plans/plans/go-s3-client-link/plan.md:365`, `src/linktest/fixture_test.go:20`, and `src/linktest/fixture_test.go:27`.

**Failure.** Stack can write a deployed smoke script within its scope. The required paired-program recovery tests may be left unwritten, or a plain Go test invocation may report a passing package that skipped them.

**Fix.** Assign the new in-process cases in `src/linktest/` to the orchestrator and the deployed cases to Stack. Name the fixtures, the PostgreSQL wrapper, and the individual checks that must execute and pass. Keep the revision 1 installer upgrade as a separate deployed check.

Design. The orchestrator and Stack own the proof. The writable paths and skip are confirmed by reading. The risk score is 10 from severity 1 and fallout 1 for a verification gap, and probability 10 if the database wrapper is omitted.

## Answers to the architect questions

1. **Existing code and tools.** Confirmed live at `19873537`: plan and frozen copy matched, the baseline was an ancestor, the three source hashes matched the record, the contract package passed, and a scratch schema generated both language outputs. By reading, the named declarations, reflection helpers, command and event symbols, router, and package scripts resolved. The assumed handoff coverage missed the files and readers identified above. `.plans/plans/go-s4-contract/plan.md:70`, `.plans/plans/go-s4-contract/plan.md:105`, and `.plans/plans/go-s4-contract/plan.md:194`.
2. **Writers and readers.** The inventory below assigns the result table's wire, generated, and operational names. The overlooked paths are the browser preflight consumer, the standing result writer, the shared receipt reserver, and the removal hook. See findings 01, 02, 05, and 06.
3. **Placement.** By reading, the Protocol Buffers additions and generated files stay with Contract, repository state with server Projects, the spool and folder reader with the client, and discovery and view state with the website. The Go boundary and the test author need explicit ownership as in findings 06 and 10. `.plans/epics/go-rewrite/epic.md:129`, `docs/specs/2026-11-27-running-an-agent-design.md:185`, and `.plans/plans/go-s4-contract/plan.md:305`.
4. **Foundation rules.** R1 is kept by part and migration placement. R2 is kept by the contract-first review and freeze. R3 is incomplete at the Link and Projects Go handoff. R4 is specified for writes and feed changes, with a gap in request-key reservation. R5 is kept by `watch.Serve`. R6 is incomplete at the standing receipt and identifier handoffs. R7 is incomplete at the replay and diagnostic boundaries. R8 specifies fixed errors and leaves gaps in their proof and router coverage. R9 leaves filter inputs and placement incomplete. R10 specifies real fixtures, with an ownership and execution gap. R11 does not apply to the contract changes, and the Projects handoff preserves credit if it reuses code. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:358` and `.plans/plans/go-s4-contract/plan.md:279`.
5. **Plan dependencies.** The sprint 0 contract and freeze feed all five plans. The sprint 1 store supplies transactions, the feed, and request receipts to Projects. The existing state folder and sprint 3 client receipts feed Folders. The current vault credential has no new dependency here. The sprint 3 public program fixtures feed the paired tests. I found no brief for the five planned-only sprint 4 neighbors, so I used the epic rows and five handoffs. The mismatches are Store's key retention, Link's Go interface, the revision 1 fixtures each program must update, and the owner of tests in `src/linktest/`. `.plans/epics/go-rewrite/epic.md:175`, `.plans/epics/go-rewrite/epic.md:201`, and `.foundry/go-rewrite/results/plan-go-s4-contract.md:39`.
6. **Intermediate merges.** Nothing found in the contract merge order. Item 1 leaves the revision 1 schema in place. Item 2 lands the source, generation, and current freeze together after item 1. Item 3 can follow either and includes its shared-watch assertions. Item 4 follows all three and extends discovery additively. Generator runs need serialization in a shared checkout and the old pin applies through review. These are prospective checks of the four items. `.plans/plans/go-s4-contract/plan.md:72`, `.plans/plans/go-s4-contract/plan.md:107`, `.plans/plans/go-s4-contract/plan.md:197`, and `.plans/plans/go-s4-contract/plan.md:249`.
7. **Later work.** Nothing found from sprint 5 in the build items. Sprint 4 is the first approved repository work of wave 2 and therefore adds behavior beyond the wave 1 checks. The new run and result fields also support later agent work, with no agent command, event, service, or runner implementation here. `docs/specs/2026-11-27-running-an-agent-design.md:51` and `.plans/plans/go-s4-contract/plan.md:117`.
8. **Dependencies.** Nothing new is added. Confirmed live in the review environment: Go 1.26.0, Node.js 26.13.0, pnpm 12.16.1, Buf 1.75.0, and the JavaScript and TypeScript generator version 2.17.0. Scratch generation exercised the existing Go Protocol Buffers and Connect tools pinned at 1.36.12 and 1.21.1. `go.mod:3`, `go.mod:10`, `pnpm-workspace.yaml:34`, and `src/contracts/buf.gen.yaml:5`.

### Who writes and reads the shared names

Contract writes the schema declarations, descriptions, and generated outputs. The table identifies the runtime writers and readers, and the checks that bind them to the plan.

| Shared names | Writer | Reader | Plan proof |
|---|---|---|---|
| `LinkService.Open`, `Hello.run_id`, `Hello.first_event_number`, `Hello.link_revision`, and `Welcome.link_revision` | Client Link starts the stream and writes Hello. Server Link implements Open and writes Welcome. | Server Link reads Hello. Client Link reads Welcome. Stack exercises both revisions. | Item 2 compares descriptors and decodes revision 1 examples. Server, Client, and Stack own admission behavior. |
| `CommandResult.last_event_number`, `timed_out`, `removed`, and `EventAck.through` | Client Link writes the result from a receipt. Server Link writes settlements and acknowledgements. | Server Link validates and stages the result. Server Projects reads settlement. Client Link reclaims acknowledged events. | Item 2 pins the fields and the comments. Paired checks cover replay and loss. The standing writer needs finding 05. |
| `Command.action.look_at_folder`, `LookAtFolder`, and both payload fields | Projects supplies the operation identifier and typed path. Server Link persists and sends the command. | Client Link dispatches to Folders. Projects reads the stored command identifier for correlation. | Item 2 pins the action, generated wrapper, and encoding. The Go handoff needs finding 06. |
| `Event.report.folder_seen`, `FolderSeen`, both identifiers, both result arms, and all five `FolderRefusal` values | Client Folders produces facts or a fixed refusal. Client Link numbers, persists, and sends it. | Server Link validates and routes it. Server Projects correlates it and applies it. | Item 2 pins every member and both wrapper types. Runtime refusal mapping needs finding 09. |
| Both `FolderFacts` messages and all their fields | Client Folders writes the link form. Server Projects maps the inspection to the website form. | Server Projects reads the link form. Website reads the inspection facts. | Items 2 and 3 pin the descriptors, presence, and representative values. Findings 07 and 08 cover their shared semantics. |
| Established revision refusal, gap refusal, and link `invalid_argument` | Server Link emits the status and existing refusal metadata. | Client Link applies the terminal or redial behavior. Stack reads the upgrade and gap results. | Item 2 keeps the code table and proof fixtures. Finding 03 adds the required recovery observation. |
| Every field in `link-v2-limits.json` | Contract writes the fixture. Server Link and Projects apply inbound, command, and typed-path bounds. Client Link and Folders apply the event, replay, outbound fact, and window bounds. | Each end's boundary tests read the respective values. | Item 2 proves full encoding sizes and the window in the contract. Program behavior is a downstream check. |
| `ProjectService`, its three procedures, and their request and response messages | Website sends the requests and the shared header. The authenticated Server Projects handler writes the responses. | Projects reads the machine, typed path, and optional repository. Website reads the watch, facts, and add reply. | Item 3 pins the procedures, direction, fields, and request-key rule. Finding 01 covers browser routing. |
| `WatchRepositoriesResponse.change` and its arms, `position`, `RepositoryList.repositories`, `Repository`, `RepositoryFolder`, and all record fields | Projects writes whole records and removal keys through the shared watch. The store assigns feed positions. Projects also produces the repository and folder in the add response. | Website replaces snapshots, replaces or removes records, and joins the add reply by both stored identifiers. | Item 3 uses the shared shape assertion, the descriptor golden file, and the identity join fixture. |
| `GetServerResponse.services` and the full `lgtm.v1.ProjectService` value | The real server's session handler advertises a registered service. The two sample configurations produce the existing reply. | Website reads the service list and sample flag. | Item 4 pins tag 6 and the additive decoding rule, with old-consumer probes. |
| `Lgtm-Request-Key` and the website statuses `unauthenticated`, `aborted`, `unavailable`, `invalid_argument`, `already_exists`, `not_found`, `failed_precondition`, and `deadline_exceeded` | Website sets the key. The store owns the receipt and conflicts. The session guard, watch, and Projects select their statuses. | Projects checks the header and receipt. Website interprets the status and fixed sentence. | Item 3 specifies the cases. Findings 02 and 09 cover retained conflicts and missing status assertions. |
| `LGTM_LINK_BASE` and `link-frozen.json` | Contract appends the revision. The orchestrator supplies the reviewed pin. | The Go check script reads the pin, and the freeze tests read the record and source bytes. | Items 1 and 2 pin line identity and the current source digests. |

## Answers to the assignment questions

a. **Sprint scope.** By reading, the plan covers the new link fields, fixed folder types, revisions, historical freeze, repository procedures, watch shape, discovery, and old reply compatibility. The required contract and downstream gaps are in findings 01 through 10. Nothing found from sprint 5 in the build items. `docs/specs/2026-11-27-running-an-agent-design.md:166` and `.plans/plans/go-s4-contract/plan.md:269`.

b. **Revision 1 freeze.** Confirmed live that the three current source digests and the revision 1 record match `fcfa96c3`. Item 2 changes source bytes, including comments, under a new current entry. The actual history comparator accepted the leading-comma append, rejected a trailing comma and a moved revision 1 line, accepted the fixed revision 2 base, and rejected a change to its hash. The existing contract tests also passed. `src/contracts/freeze_test.go:57`, `src/contracts/testdata/link-frozen.json:2`, and `scripts/check-go.sh:30`.

c. **Revision 3 room.** Nothing found. The existing command arms occupy 10 through 12 and the existing event arms occupy 10 and 11. After this plan uses command 13 and event 12, six commands fit in available tags 14 through 19 and seven events fit in 13 through 19. The later files can use their own message tags. `src/contracts/proto/lgtm/link/v1/commands.proto:51`, `src/contracts/proto/lgtm/link/v1/events.proto:54`, `.plans/plans/go-s4-contract/plan.md:134`, and `docs/specs/2026-11-27-running-an-agent-design.md:319`.

d. **Lost buffer.** By reading, the server already serializes the durable mark under the machine lock and the client already persists its next counter and retained suffix. Both can implement the preservation rule. Current result acceptance settles immediately, and current receipts have neither a watermark nor a process boundary. The dependent builds need durable staging and durable receipt evolution. Waiting approval, standing replies, and the diagnostic for server-only loss detection need findings 03 through 05. `src/server/internal/link/inbox.go:27`, `src/server/internal/link/outbox.go:21`, `src/server/internal/link/outbox_result.go:25`, `src/client/internal/link/store.go:320`, and `src/client/internal/link/receipts.go:34`.

e. **Samples.** Confirmed live that the scratch schema lints and generates, the mock type-checks with the generated additions, and the frontend type-checks with the additive discovery field. A scratch frontend build passed with a shim for the sandbox's blocked network-interface enumeration. Two in-process sample engine configurations returned revision 1 with no services, and the in-page Machines snapshot arrived. By reading, `serve.ts` exposes the same engine through the network adapter and the frontend keeps its revision check. The full browser and network mock probes remain for the built contract. `src/frontend/src/server/api.ts:33`, `src/mock-server/src/engine.ts:147`, and `src/mock-server/src/serve.ts:53`.

f. **Handoff names.** All new wire layouts have a work item and a named descriptor or encoding check. The inventory above maps the rest of the table. The fixed website statuses lack a complete named test, the key needs a shared lifetime rule, and registration and the new Go dependencies need explicit handoffs. See findings 01, 02, 06, and 09.

g. **Vendor.** OpenAI supplies the planner, test writers, builders, and this reviewer. They work in separate sessions. `.plans/plans/go-s4-contract/plan.md:327` and `.foundry/go-rewrite/results/plan-go-s4-contract.md:35`.

## Post-build checks

1. **History and compatibility.** Run items 1 through 4 at their exact candidate commits. Generate both languages cleanly and run the full Go gate using the revision 1 pin. Advance to the reviewed landed revision 2 commit and confirm the same gate rejects a changed revision 2 line. Contract owns the commit outputs and orchestrator owns the pin.
2. **Lost result and acknowledgements.** Stage a result while its event waits, restart the server, and hold all three send permits. Replay a repeat, a consumed next event, and the awaited report before releasing a permit. Require one cumulative acknowledgement for each eligible receipt, one stored report effect, one settlement, and no claim until the gate opens. Send a gap in a separate stream and require no event acknowledgement. Server Link and the paired-program test owner capture the database marks and exact response sequence.
3. **Disk upgrade.** Start a revision 1 client with an unacknowledged event and a finished receipt, observe its verified revision refusal, then upgrade the installed client against the same disk state. The event applies once and the completed command returns from its original receipt with watermark zero. Stack captures the refusal and the resulting server state.
4. **Sample and real discovery.** Run one built website in the in-page sample, network mock, older real, and newly enabled real modes. The two sample modes keep the same Machines snapshot and sample workspace and issue no repository watch. The older real mode retains its existing behavior. The newly enabled mode opens the repository watch and uses the real records. Website and Stack capture browser requests and the visible workspace.
5. **Successful add and removal.** Return an add reply before the watch update, repeat with watch first, then remove the machine while another folder operation is open. The website joins the stored folder and repository in both orderings. The same removal transaction ends the operation and emits the required saved or removed repository changes. Projects and Website capture the database state, watch frames, and rendered tree.

## Grade

6 out of 10. There are 0 blockers, 8 majors, and 2 minors.
