REVISE

The frozen contract says that a courtesy removal notice keeps the stream open, but the client plan still stops on that notice and can miss the server's actual refusal.

## Findings

### logic-r5 01 — MAJOR — C10 — A courtesy removal can stop the client

By reading, decision 75 and the draft tell the client to treat `APPROVAL_REMOVED` as waiting and keep the stream. Client item 6 and decision 40 still instruct it to reply and stop, and the command test expects that path. Item 7b separately treats honoring an unsigned removal as a regression. Evidence is in `.plans/plans/go-s0-contract/decisions.md:82`, `.plans/plans/go-s0-contract/draft/commands.proto:138`, and `.plans/plans/go-s3-client-link/plan.md:195`, `:238` and `:247`, and `.plans/plans/go-s3-client-link/decisions.md:47`.

An approved client receives a valid higher-revision courtesy `SetState` while the terminal status is delayed. Item 6 can stop the process before item 7b reads `unknown_machine`; the service can then stay stopped even if the connection merely broke and no authenticated refusal was received.

Make a higher-revision courtesy notice move standing to waiting and return done under the ordinary result rules. Remove the stop branch and the special two-second wait from item 6. Test a notice with no terminal status, a notice followed by an ordinary disconnect, and a notice followed by `unknown_machine` after a verified challenge. Expect continued heartbeats, a retry and a permanent stop respectively.

### logic-r5 02 — MAJOR — D7 — The removal race has conflicting acceptance criteria

By reading, the new rule permits a `Welcome` authorized by an admission commit before a removal. Check 10 holds the stream after that commit and before send, then requires that it never receive `Welcome`. Check 16 permits a frame already in transit, and check 23 requires that a held send be suppressed after a newer admission. The server handoff still marks delayed registration of an overtaken stream as open. Evidence is in `.plans/plans/go-s0-contract/draft/link.proto:121` and `:158`, `.plans/plans/go-s0-contract/plan.md:882`, `:888` and `:895`, and `.plans/plans/go-s3-server-machines/handoff-stream-and-commands.md:27` and `:46`.

Stream B commits admission, pauses before writing `Welcome`, and the owner removes its machine. An implementation that sends the authorized frame and then refuses the next frame meets the draft and fails check 10. Under replacement, a paused older handler can also register after the newer handler has come and gone unless the stream writer can identify the superseded generation.

Make checks 10, 16 and 23 use the same send boundary. Before admission, removal forbids `Welcome`. After admission, removal permits the authorized frame and forbids new writes from the machine. For a newer admission, suppress the older frame if its writer has yet to start it. Allow already-sent bytes to arrive and terminate the older stream with `aborted` if replacement was its first terminal cause. Assign the generation check and the send boundary to Server Stream, including the case where the newer stream has already disconnected.

### logic-r5 03 — MAJOR — C5 — A malformed host name has two outcomes on replay

By reading, the amended `Join` checks all value rules first, including the zero-byte and length rules for strings. The same comment says a replay never reads the host name. The Server Machines plan checks the host name only on a first use, and explicitly tests successful replay with a zero byte or 256-byte name. Client item 2 describes validation of the eight-bit Unicode Transformation Format (UTF-8) and truncation without a zero-byte check. Evidence is in `.plans/plans/go-s0-contract/draft/link.proto:21`, `:74`, `:90` and `:204`, `.plans/plans/go-s3-server-machines/plan.md:181` and `:185`, and `.plans/plans/go-s3-client-link/plan.md:101`.

A first use succeeds and the response is lost. Before the same-key retry the caller supplies a host name containing a zero byte. One rule returns the stored machine and the other returns a permanent `invalid_argument`, leaving the waiting row and no local enrollment.

Name the two validation phases. Check the token identifier and the two byte fields before looking up the token. Check a decoded host name's length and zero-byte rule only before creating a waiting machine, after the existing replay and refusal decisions. Keep rejection of undecodable requests at the transport boundary. Align the client name normalization, the server test, and the comment. Exercise the same invalid host on first use, same-key replay, a wrong secret and an expired token.

### logic-r5 04 — MAJOR — C11 — The event ceiling disagrees with the event table

By reading, the field and the Server Stream handoff require `invalid_argument` above the signed 64-bit ceiling. The earlier table takes precedence over the value rules and rejects only zero before comparing against the mark. Evidence is in `.plans/plans/go-s0-contract/draft/events.proto:13` and `:42`, `.plans/plans/go-s0-contract/draft/link.proto:9`, and `.plans/plans/go-s3-server-machines/handoff-stream-and-commands.md:33`.

At mark 42, number 2^63 gives a retryable `event_gap` by the table and `invalid_argument` by the field. At mark 2^63 − 1, the same event is mathematically next. A handler following the table can attempt to move the mark outside the database type or overflow a signed next-number calculation.

Make the first decoded-number check reject both zero and numbers above 2^63 − 1 before any comparison, conversion or arithmetic. State that at the ceiling every valid number is a repeat. Apply all report value rules, including zero-byte and numeric limits, only in the next-number branch. For the client, choose the inclusive cap explicitly. Its durable `next` is unsigned, so an append at `math.MaxInt64` can persist the value one above it as the exhausted next number. Test that the final event remains readable and acknowledgeable after reopen and that the following append changes nothing. The built counter and acknowledgement check are in `src/client/internal/link/outbox.go:27`, `:35` and `:83`.

### logic-r5 05 — MAJOR — C5 — The refusal fallback can retry an invalid join

By reading, the draft makes `Join invalid_argument` a stop, then tells the machine to dial again for any missing or unknown reason. A value-rule refusal has no reason metadata. The client plan stops on the code and uses a fallback diagnostic if the reason is unavailable. The same patch makes `event_gap` retryable, but item 6 and the public names still call it final. Evidence is in `.plans/plans/go-s0-contract/draft/link.proto:41` and `:51`, `.plans/plans/go-s0-contract/plan.md:255`, `:260`, `:312` and `:654`, and `.plans/plans/go-s3-client-link/plan.md:101`, `:146` and `:186`.

A malformed join can retry for the full 15-minute budget if a client applies the fallback to the empty reason. A client reading the older public name as a retry rule can stop after a recoverable gap.

Scope missing or unfamiliar reasons to `Open failed_precondition`. Make both `Join` stop codes unconditional and use the reason only for diagnostics. Use the action table for all retry decisions and remove the older description of the gap as final. Say that on a new welcome the client resumes above the committed mark. Have item F4 compare the verified-challenge condition and the fallback as well as the codes. Assign the fixture to the contract test, the enrollment and link retry tests, and the Server Stream tests named by the amendment. Test both join stops without metadata and a gap followed by a fresh stream.

### logic-r5 06 — MAJOR — R6 — The command send margin omits delivery time

By reading, the draft allows a non-`SetState` send up to expiry plus 23 hours. The client can prune after expiry plus 24 hours on its wall clock. The built receipt store prunes on another finish, and the combined check retries half an hour inside the send boundary. The server handoff separately leaves a claim held across finalization open. Evidence is in `.plans/plans/go-s0-contract/draft/commands.proto:6` and `:18`, `src/client/internal/link/receipts.go:137` and `:151`, `.plans/plans/go-s0-contract/plan.md:883`, and `.plans/plans/go-s3-server-machines/handoff-stream-and-commands.md:38` and `:52`.

The machine clock is 59 minutes and 59 seconds ahead. The server sends at expiry plus 22 hours, 59 minutes and 59.5 seconds. Another command finishes on the machine while the retry is in transit. After a two-second delay, the completed receipt can already have been pruned. The client returns expired despite having done the work, and the server can store that result permanently. No clock jump or offset greater than an hour is necessary.

Keep the chosen machine clock. State the replay limit in terms of the sum of fast clock offset and elapsed time from send authorization through client receipt lookup. Require the server's send window to leave room for the supported total or document the accepted false-expiry case at that boundary. Give the Server Commands plan a send-authorization check for the current, pending attempt at a strict cutoff. Continue accepting a matching result until the expiry outcome commits. Add a boundary case with delivery delay and another receipt pruned between send and lookup. Retain the independent no-late-start rule.

### logic-r5 07 — MAJOR — C7 — Server Stream is handed a different waiting policy

By reading, the amendment chooses option B, under which a valid waiting `Facts` report is accepted and shown. The Server Stream handoff says a machine can connect and do nothing else, and its cited event decision consumes every report from a waiting machine. The current client chooses to send only heartbeats while waiting. Evidence is in `.plans/plans/go-s0-contract/plan.md:145` and `:174`, `.plans/plans/go-s0-contract/draft/link.proto:150`, `.plans/plans/go-s3-server-machines/handoff-stream-and-commands.md:32`, `.plans/plans/go-s3-server-machines/decisions.md:51`, and `.plans/plans/go-s3-client-link/plan.md:198`.

A client allowed by the frozen contract sends a valid next-number `Facts` while waiting. A server built from the handoff acknowledges it and discards it. The client then forgets the event, and the approval screen never shows those facts as the selected option promises. A combined test using only the current client passes either server implementation.

Assign Server Stream to apply and acknowledge a valid next-number `Facts` while waiting, and assign Server Machines to show the data in the waiting record. Leave approval readiness clear until both reports are applied while approved on the current stream. Consume a waiting harness report without effect. Test both cases with a scripted client and read the result from the page's watch.

### logic-r5 08 — MAJOR — plan — The default freeze check can forget revision 1

Confirmed live, `git merge-base HEAD master` at the reviewed commit resolves to `f1dcbb7d` and that tree contains no `src/contracts/testdata/link-frozen.json`. By reading, item F5 succeeds when that record is absent, and its Go test compares the current files against the current manifest. The task freezes the link before either side builds, which can be earlier than the merge to `master`. Evidence is in `.plans/plans/go-s0-contract/plan.md:268`, `.plans/plans/go-s0-contract/decisions.md:78`, and `.plans/epics/go-rewrite/epic.md:128`.

Freeze revision 1 on the integration branch, then start a dependent branch before it reaches `master`. Change a rule comment and update its revision 1 hash. The Go test accepts the matching new pair, and the default base check accepts the missing record. Both programs can build against different revision 1 rules.

Have the orchestrator record the reviewed freeze commit as a baseline and pass a baseline that contains the record to every dependent branch and merge gate. Require the manifest prefix from that baseline once the freeze exists, and allow an absent record on the initial freeze gate only. Test a downstream branch with `master` still at its pre-freeze commit by changing both a file and its hash. Put the check in the existing `scripts/check-go.sh` or update the Contract scope before dispatch. Its current edit list allows that script and omits the new script path. See `.plans/scopes/contract/contracts/scope.json:7`.

### logic-r5 09 — MAJOR — C6 — Standard input enrollment has no client work item

By reading, decision 82 selects option C, whose `-` token argument reads standard input. The client plan's item 9 takes output and error writers only and forwards the literal positional token. Its decision 42 leaves the input mode unresolved. The built command is a stub and supplies no input reader to its test entry point. Evidence is in `.plans/plans/go-s0-contract/plan.md:154`, `.plans/plans/go-s0-contract/decisions.md:89`, `.plans/plans/go-s3-client-link/plan.md:263`, `.plans/plans/go-s3-client-link/decisions.md:49`, and `src/client/cmd/lgtm-client/main.go:13`.

An owner uses `lgtm-client join <address> -` and supplies a valid token on standard input. Item 9 as written passes the dash to the token decoder, so the chosen enrollment mode fails before dialing.

Hand decision 82 to the Client team before it freezes item 9's tests. Give the command a bounded input reader for the dash form, pass a single token to the existing application call, and keep the positional form for the installer and page. Test a valid input line, empty or oversized input, and the positional form, with no token in usage or diagnostics.

## Evidence and risk

All findings are confirmed in the plan, the cited sources, or the reported probe. The scores rate the consequences if the text is implemented as written. The owners listed here are the builder lanes for the fixes.

| Finding | Category | Label | Owner | Severity × fallout × probability | Score and band |
|---|---|---|---|---|---|
| logic-r5 01 | Design | MAJOR | Client Link | 6 for a stopped service × 3 for a local restart × 6 for an ordinary removal | 108, schedule |
| logic-r5 09 | Design | MAJOR | Client Link | 6 for a broken enrollment mode × 3 for a local workaround × 6 for selecting one advertised input mode | 108, schedule |
| logic-r5 02 | Design | MAJOR | Contract and Server Stream | 6 for a wrong connection result × 3 for local recovery × 4 for one timing race | 72, schedule |
| logic-r5 05 | Design | MAJOR | Contract and Client Link | 4 for a delayed refusal or interrupted retry × 3 for one client × 4 for one refusal with the wrong classification | 48, record |
| logic-r5 03 | Design | MAJOR | Contract, Server Machines and Client Link | 6 for failed enrollment × 3 for re-enrollment × 2 for a lost reply with an invalid name | 36, record |
| logic-r5 04 | Design | MAJOR | Contract and Server Stream, with Client Link for the outbox | 6 for a wrong refusal or failed write × 3 for one stream × 2 for a malformed or boundary fixture | 36, record |
| logic-r5 06 | Bug | MAJOR | Contract and Server Commands, with Client Link for the receipt check | 6 for a false permanent command outcome × 3 for one command × 2 for a skewed clock and delayed retry | 36, record |
| logic-r5 07 | Design | MAJOR | Server Stream and Server Machines | 6 for missing promised facts × 3 for one waiting machine × 2 for a waiting client that elects to report | 36, record |
| logic-r5 08 | Design | MAJOR | Contract and the orchestrator | 1 for a missing build guard × 1 for development scope × 10 for every downstream gate with a pre-freeze base | 10, record |

## Answers to the assignment

1. Each round 4 patch is checked in the table below. The amendment preserves the descriptor and changes behavior for some existing inputs.
2. The amended rules touch the unreviewed mechanisms for replay and restore, `SetState` and clocks, handshake policy, and the current-stream fence. The lens answers and decision tables cover those interactions. Restore and live copies stay outside wave 1.
3. The round 4 register had no rejection. The amendment's two rejections have evidence. Client decision 61 keeps its own receive cap and fixture and removes the contract-comment prerequisite. Client Local decision 87 supersedes count-based retention with age, as the built pruning code does. See `.plans/plans/go-s3-client-link/decisions.md:68`, `.plans/plans/go-s1-client-local/decisions.md:94`, and `src/client/internal/link/receipts.go:151`.
4. Confirmed live, no protocol name changed. By reading, the amendment assigns the refusal fixture to Contract and the readers to both endpoint test suites. The client plan omits loading that file and applying all its fields. The orchestrator who supplies the freeze baseline and the client command that reads standard input are also missing from the respective work items. Stack Client owns the renamed command and its installer invocation. Server Machines supplies the token and removal behavior. See `.plans/plans/go-s0-contract/plan.md:216`, `:248` and `:253`, `.plans/plans/go-s3-client-link/plan.md:186` and `:269`, `.plans/plans/go-s3-server-machines/plan.md:247`, and `.plans/plans/go-s3-stack-client/plan.md:73` and `:206`.
5. The affected cells are listed in the decision tables below.
6. Round 4's removal acceptance still conflicts with the admission boundary. Its gap ruling now has a correct action row and older contradictory wording. Event ordering needs the new ceiling branch. The remaining round 4 cases are covered in the patch table.
7. All eight logic questions touch the amendment. The numbered lens answers below identify the affected work items and their checks.

### Ten patch groups from the round 4 register

| Group | Result when read with the amendment |
|---|---|
| G1, removal | The rule allows a previously authorized `Welcome`. Check 10 still forbids it after admission. The courtesy notice also disagrees with the client plan. See findings 01 and 02. |
| G2, event validation | The event table fixes the original invalid-report ordering for in-range numbers. The new ceiling needs an earlier branch. See finding 04. |
| G3, refusal metadata | The carriers and keys agree with the client plan. Nothing found in the carrier change. The new action fallback is a separate conflict in finding 05. |
| G4, `SetState` | An expired approval remains deliverable. The amendment gives pending rows an end on removal. The client's courtesy stop remains contradictory. See finding 01. |
| G5, command clock | The independent start check still uses the server's clock and gives a saved result priority. The newly stated replay margin does not cover delivery time. See finding 06. |
| G6, handshake order | The six decisions remain in order and the client requires a verified challenge before a refusal of `Open` stops it. Nothing found in that order. |
| G7, projection | Waiting, approved offline, joining and online stay disjoint. The new waiting data rule disagrees with the server handoff. See finding 07. |
| G8, current stream | The admission commit preserves the old-event prefix rule. The after-admission send case conflicts with check 10. See finding 02. |
| G9, gap and finality | The new action column and the client both retry the gap. The public name and item 6 still call it final. See finding 05. |
| G10, first-use race | The loser still decides under the token lock and checks expiry and revocation again. New host-name validation leaves a contradictory replay case. See finding 03. |

### Other amended cases

Nothing found in deferring the connection token until the first stream opened after approval. The client holds the token only for the session and has no side calls in wave 1. Nothing found in giving up after 30 seconds of silence or allowing an empty load measurement. A bad heartbeat ends the stream and a heartbeat acknowledgement keeps a healthy idle stream active. See `.plans/plans/go-s0-contract/draft/link.proto:140` and `:150`, `.plans/plans/go-s0-contract/draft/events.proto:100`, and `.plans/plans/go-s3-client-link/plan.md:145`, `:173` and `:195`.

Nothing found in requiring a new token after expiry of a lost reply, retaining the client's private key until recovery, or keeping the pin tests with the client while combined acceptance uses the loopback route. Nothing found in letting the server omit `Event.happened_at` from storage and display while preserving it in the client queue. The website-facing comments in F3 match the owner calls planned by Server Machines and the command planned by Stack Client. See `.plans/plans/go-s0-contract/draft/link.proto:95`, `.plans/plans/go-s0-contract/draft/events.proto:45`, `.plans/plans/go-s3-client-link/plan.md:87`, `:158` and `:257`, `.plans/plans/go-s3-server-machines/plan.md:247`, and `.plans/plans/go-s3-stack-client/plan.md:73`.

### Answers to the logic lens

1. The event order is durable across restart, with zero reserved for an empty acknowledgement mark and a repeated event acknowledged at the existing mark. The new ceiling and its arithmetic are in finding 04. Attempts and standing revisions keep their existing order. The command send, prune and result boundaries are in finding 06.
2. A failed lookup gives `unavailable`. A lost `Join` reply can be replayed while the token remains valid, and after expiry a fresh enrollment and cleanup repair the orphan. A silent stream ends and reconnects. Findings 02, 03 and 06 cover the conflicting outcomes for removal, replay and clocks.
3. The tables below show the disagreements and the missing boundary cells.
4. A missing or zero event number gives `invalid_argument`. An empty waiting token and absent load are valid. Unknown approval waits and a missing welcome state fails validation. The missing refusal reason is ambiguous for `Join` under finding 05.
5. An admission and removal can race the first-frame writer. A command result can race the sweep. A waiting report can race approval. Findings 02, 06 and 07 identify the needed ownership and observation points. No change is needed to receipt locking inside the built client store.
6. F1 and F2's descriptor comparison catches a changed number or type. Both versions of the removal and ceiling rules have the same descriptor. F4's moved order line must fail its exact-line test. Changing only `after_verified_challenge` in the fixture can leave all listed assertions green. F5 detects its proposed mutations in its proposed fixtures. A coordinated edit escapes when the base predates the freeze.
7. F1 can preserve conflicting stop instructions, and F2 can preserve the incomplete event table and clock margin while their acceptance passes. F3 can leave the old `RemoveClient` comment and pass the specified grep and frontend type check. F4 can compare literal text without asserting the decision fallback. F5 can accept a current file and hash rewritten together, as in finding 08.
8. A single ordered event table with an explicit range guard prevents either endpoint from treating an unrepresentable number as next. A baseline independent of the candidate manifest prevents a revision from validating its own rewritten history. The other proposed fixes preserve the current wire types and identify the actor that decides each race.

### Decision tables for patched rules

| Inputs | Result or conflict |
|---|---|
| Malformed token identifier or wrong key or secret length | `invalid_argument` before token lookup. A well-shaped wrong secret instead gives `token_unknown`. |
| Valid token and same key, with a malformed decoded host name | Contract value-first gives `invalid_argument`. The replay rule and server test return the first machine. |
| Valid (decoded) request, token locks after expiry or revocation alongside a replay | Server refuses by the current state, with expiry before revocation and both before replay. |
| `Join` has a stop code but no known reason | Client stops. The unqualified draft metadata fallback also says retry. |
| `Open` has `unknown_machine` or revision mismatch after a verified challenge | Stop. Before verification, retry. |
| `Open` has a gap or no recognized reason after a verified challenge | Retry. Older plan text still lists the gap as final. |
| Removal commits before admission | No welcome, and `unknown_machine`. |
| Admission commits first and removal follows while the welcome is held before send | Draft permits the welcome. Check 10 forbids it. |
| Welcome was sent before removal | It can arrive later. A new machine frame receives no acknowledgement and changes no row. |
| Replacement commits before the older welcome is sent | Check 23 requires no welcome and `aborted`. The draft needs to distinguish a held writer from bytes already in transit. |
| Courtesy removal arrives, with or without a later verified refusal | Draft waits and keeps the stream. Client item 6 can stop in both cases. |
| Current stream, decoded event number zero or above the ceiling | Zero is invalid. The above-ceiling code conflicts with the gap and next-number branches. |
| Current stream, in-range repeat or gap | Acknowledge the repeat at the mark. End a gap and retry without considering the report. |
| Current stream, valid next-number facts while waiting | Apply and acknowledge under option B. The handed-off server rule consumes the event without applying it. |
| Current stream, bad next-number report or waiting harness report | Consume and acknowledge without setting readiness. After approval, valid current-stream facts and harnesses set readiness together. |
| Matching result arrives before or after 24 hours while the command is pending | Store it until the server commits expiry. A result arriving after that commit has no effect. `SetState` has no timer end. Removal ends all pending rows. |
| Non-`SetState` retry leaves just inside the send limit and the receipt is pruned before lookup | An already completed command can return expired with an offset under one hour. |

## Post-build checks

- Compare full built and amended Protocol Buffer descriptor images with `buf build --exclude-source-info`. Confirmed live, copying all three link drafts and applying both F3 comment edits in a temporary copy gave byte-identical images and passed `buf lint`. The Secure Hash Algorithm with a 256-bit digest (SHA-256) was `d1ef120ae5e20b1e4917f8c0a4cf02904b406e276de2615b4a656e9894b53547` for both. The images cover the declarations. Reading the comments also confirmed that the code names stay the same and some existing inputs acquire different outcomes.
- Hold a committed event's acknowledgement in transit and remove the machine. The completed event remains committed, and the server authorizes no acknowledgement for a frame processed after removal. The client stops on the later verified refusal, even if the courtesy notice arrived first.
- Apply and acknowledge event 2^63 − 1 in a seeded fixture, reopen the outbox, then try the same number and one above it. Expect a repeat acknowledgement at the ceiling, `invalid_argument` above it and no write past the ceiling.
- On a current stream, deliver a heartbeat with a present load of 101 and then run a valid idle stream. Expect a stream error and retry for the first and acknowledgements that sustain the second. A broken next-number report is consumed without effects, even when its only fault is a zero byte or an out-of-range numeric field.
- Run the refusal and replay fixtures through Connect and gRPC (gRPC Remote Procedure Calls), including missing reason, early authenticated refusal and a recoverable event gap. Expect the actions in the corrected table and the error metadata on both protocols.
- The existing contract Go tests passed. The built client-link tests did not reach the outbox and receipt assertions here because this sandbox presents the root directory as owned by an unmapped user, which the state-folder guard rejects. Evidence is in `src/client/internal/state/folder.go:154` and `:166`. The client store conclusions above are by reading.

## Grade

5 out of 10. 0 blockers, 9 majors and 0 minors.
