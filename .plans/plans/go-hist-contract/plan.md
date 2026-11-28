---
topic: go-hist-contract
status: attack
tier: M
created: 2026-11-28
updated: 2026-11-28
epic: go-rewrite
---

# The contract for reconciling machine history

## Overview

The server and a machine need to agree on where event replay starts when either side has an older copy of its saved history. The machine will send two more numbers when it opens the link. The server can then decide which history needs to move forward and return the agreed number in its existing reply. The contract spells out when the server may advance its mark, when the machine may discard covered events, and which commands the server settles as it admits the stream.

The website contract also gains a list of recent requests that failed during a history reset. This gives the website the information it needs to show the failure and offer the same action again. Field 19 stays in place through the first builds so that the current website and server still compile.

The contract build comes first. The server and client will implement the reconciliation in their next builds. The website will then use the new list, remove the old warning, and prove the behavior with both programs running together.

## Key decisions

The two counters are enough because the server already sends its durable mark. The same reply tells the machine whether to replay its remaining events, start above the server’s mark, or reconnect after an unexpectedly low reply. The machine still stops on damaged history.

The owner decided on 2026-11-28 to amend link revision 2 in place. The historical comparison gate will use the exact reviewed amendment commit as its new base. A separate comparison against the old base will show which record was intentionally amended.

Full log → `decisions.md`

## Workstreams

### The link agrees on where replay resumes

The contract will define the three numbers in the opening request, the checks before admission, and the decisions that follow. It will also define how an older machine’s outstanding commands settle. Tests will check the text, the encoded fields, and the frozen record. The size wording will match the existing limit both before and after decompression.

### The website receives failures it can show again

The machine record will carry the identifier, kind, and failure time of a recent harness scan or restart that ended during the reset. An empty list will continue to work for the sample modes. Tests will read the list in both an initial machine snapshot and a later machine update.

## Rollout order

1. Prepare the link and website additions independently. Generate the contract from the combined sources and review the intended changes to the frozen record.
2. Prove the contract build against the reviewed amendment commit. Move the shared base when that commit lands so later builds can run the same gate.
3. Build the server and client side by side against the new names. The website build follows both. It removes its last use of the warning, then reserves field 19 and its name.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1: The link agrees on where replay resumes**

This item defines one comparison after authentication. It preserves the link’s authentication exchange and ordinary replay. It freezes the changed text and fields together so the contract build can be checked on its own.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | Hello carries the two unsigned counters at tags 6 and 7. The existing Welcome fields, proof text and command outcome values retain their exact shape. | `TestOpenMatchesItsDescription`, `TestHelloHistoryNumbersOnWire`, `TestProofExamplesHoldTogether`, and `TestCommandsMatchItsDescription`. |
| 2 | The text places the number checks after verified Proof and states the comparisons, readiness, replay, staged-result handling, reset exceptions and retry. | `TestRevisionTwoLinkRulesAreWritten`, `TestWelcomeHistoryReconciliationIsWritten`, `TestHandshakeOrderIsWritten`, and `TestLinkCodesAreWritten`. |
| 3 | The 65,536-byte limit covers the complete message wrapper both on the wire and after decompression. The separate Command and Event budget stays at 49,152 bytes. | `TestRevisionTwoLinkRulesAreWritten` and `TestFolderWireBudgets`. |
| 4 | The amended record matches the link sources at revision 2, preserves revision 1 and still catches a change against the selected base. | `TestLinkIsFrozenAtItsRevision`, `TestLinkFreezeAppendIsDocumented`, and `TestLinkFreezeAppendAgainstPriorBaseline`. The old-base check and the amendment-base check give the distinct results specified in rollout step 2. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1.1 | Write the descriptor fixture and link tests first. Use the existing test files listed below. | `src/contracts/testdata/golden/open.txt`, `message lgtm.link.v1.Hello`. `src/contracts/open_test.go`, `TestOpenMatchesItsDescription`. `src/contracts/envelopes_test.go`, `wireUint`. `src/contracts/rules_test.go`, `TestRevisionTwoLinkRulesAreWritten`. `src/contracts/review_a_test.go`, `TestWelcomeMarkIsCheckedBeforeChangingSavedHistory`. |
| 1.2 | Add the fields and replace the link comments in the tables below. | `src/contracts/proto/lgtm/link/v1/link.proto`, `LinkService.Open`, `Hello`, and `Welcome`. `src/contracts/proto/lgtm/link/v1/events.proto`, the event-order header and `Event.number`. `src/contracts/proto/lgtm/link/v1/commands.proto`, the command life header, `CommandResult.last_event_number`, and `CommandOutcome`. |
| 1.3 | Write the freeze tests first, then document the one amendment and recalculate the affected source hashes. | `src/contracts/freeze_test.go`, `TestLinkIsFrozenAtItsRevision`, `TestLinkFreezeAppendIsDocumented`, and `TestLinkFreezeAppendAgainstPriorBaseline`. `src/contracts/README.md`, Rules. `src/contracts/testdata/link-frozen.json`, revision 2. Read `scripts/check-go.sh`, the `link_base` branch. |
| 1.4 | Regenerate the link outputs in the exact file list below. | `src/contracts/buf.gen.yaml`, `plugins`. `src/contracts/package.json`, `generate` and `check`. The existing files are anchored in `scouting.md`. |

**Files and dependencies.** Change the three link sources, `src/contracts/testdata/golden/open.txt`, `src/contracts/envelopes_test.go`, `src/contracts/rules_test.go`, `src/contracts/review_a_test.go`, `src/contracts/freeze_test.go`, `src/contracts/README.md`, and `src/contracts/testdata/link-frozen.json`. Generate `src/contracts/gen/go/lgtm/link/v1/commands.pb.go`, `events.pb.go`, `link.pb.go`, and `linkv1connect/link.connect.go` in that Go directory. Generate `commands_pb.d.ts`, `commands_pb.js`, `events_pb.d.ts`, `events_pb.js`, `link_pb.d.ts`, and `link_pb.js` in `src/contracts/gen/lgtm/link/v1/`. This item has no dependency on item 2. Use separate working copies for parallel generation because the generator visits both packages. Run a clean generation on the combined candidate.

**Tests written first and existing test changes.**

| File | Test | First failing observation or change |
|---|---|---|
| `src/contracts/open_test.go` and `src/contracts/testdata/golden/open.txt` | `TestOpenMatchesItsDescription` | Add only Hello fields 6 and 7 to the fixture. The test body stays in place and fails against the old descriptor. The Welcome block in the fixture keeps the same fields. |
| `src/contracts/envelopes_test.go` | New `TestHelloHistoryNumbersOnWire` | Decode literal varints at tags 5, 6 and 7 into the three named fields. Use the boundary 2^63 with both marks at 2^63 − 1. Assert that a Hello with first number 1 and omitted marks reads both as zero. Assert singular unsigned 64-bit descriptors and that the decoded numbers survive a binary round trip. Use reflection like `TestRevisionOnePayloadsDecodeUnderRevisionTwo` so the test can fail on missing descriptors before regeneration. This fixture tests encoding. Handshake rejection belongs to H9. |
| `src/contracts/rules_test.go` | `TestRevisionTwoLinkRulesAreWritten` | Replace the old blanket unseen-run and history-refusal assertions with the shape and order checks, ordered cases in the admitting write, scoped fences, ready-at-admission exception, and no warning. Assert the bound on Welcome. In the package header and the Open comment, independently require both measurements of the outer wrapper. Replace the old replay paragraph assertions with the separate Welcome test below. Assert a staged result’s two settlement paths, the reset rules for commands and a missing folder report, the repeat-safe reset, and the server-only outcome without an enum value. Keep the unrelated readiness, event, result and size assertions. |
| `src/contracts/review_a_test.go` | Replace `TestWelcomeMarkIsCheckedBeforeChangingSavedHistory` with `TestWelcomeHistoryReconciliationIsWritten`. | Read the event-order header and assert three distinct cases, including both inclusive endpoints for ordinary replay. Assert discard and advance in one write for a high mark, redial for a low mark, empty start for a missing file with intact credentials, and a stop with `link_stopped` and exit status 0 on damaged history. Assert that the client writes fresh reports on an approved Welcome while event numbers remain available. The old disk-unchanged assertion for a high mark is retired. Item 2 edits a separate test in this file. Coordinate the two edits by function. |
| `src/contracts/freeze_test.go` | `TestLinkIsFrozenAtItsRevision` | Keep its assertions and update the mismatch diagnostic to refer to the recorded hash and the reviewed record. The existing hash-mismatch check goes red when the text changes before the new hashes are written. |
| `src/contracts/freeze_test.go` | `TestLinkFreezeAppendIsDocumented` | Add assertions that the README records the dated revision 2 exception, names the exact reviewed amendment commit as the new base, and says when to update the shared pointer. Keep the ordinary append rules. |
| `src/contracts/freeze_test.go` | `TestLinkFreezeAppendAgainstPriorBaseline` | Extend the revision 2 rewrite subcase to commit the approved rewrite in the test’s scratch repository and use that exact commit as the base for a later unchanged candidate. It passes with that base. The prior base rejects the edited revision 2 line. Keep the existing altered or moved revision 1, trailing separator, and ordinary append cases. |

`TestHandshakeOrderIsWritten`, `TestLatestRunIsReadyAtAdmissionAndCallsBackOnlyOnce`, `TestReadinessCallbackRunsInTheReadinessTransaction`, and `TestAdmissionReadinessIncludesReconnectAfterRestart` keep their assertions. Keep the six exact handshake status lines and the wording that joins the inclusive readiness threshold to its callback. `TestLinkNumbersAreWritten` still expects the exact event and command limits and timings. `TestRevisionOnePayloadsDecodeUnderRevisionTwo` keeps its existing scope. None of the test files changed in this item appears in the foundry’s frozen lists. The frozen source record is accounted for separately below.

**Hello values and check order.** Let `M = 2^63 − 1`. Events have numbers in [1, M]. `first_event_number` is field 5 in [1, M + 1]. It is the next number captured before this process first appends, and stays fixed across redials with the same version 7 universally unique identifier. `acknowledged_event_number` is field 6 in [0, M]. It equals the retained floor minus one, even when no retained rows remain. `last_event_number` is field 7 in [0, M]. It equals the same snapshot’s next number minus one. Zero in either new scalar is valid without field presence. Fresh history starts with first number 1 and both marks zero. Use the test triples (1, 0, 0), (M + 1, M, M), (1, 8, 8), (1, 2, 1), and (3, 0, 1), in first, acknowledged, last order. The first three satisfy the number rules. The last two are rejected for the separate contradictory relations. An empty retained queue after acknowledgement or adoption can have both marks above zero. Check the individual ranges before arithmetic or conversion. Require acknowledged ≤ last and first ≤ last + 1. Equality in either is legal. No ordering of first against acknowledged is required. The same process can acknowledge past its first number. The value M + 1 signals exhaustion of allocatable event numbers. It cannot be used as an Event number.

First accept a timely, decodable Hello with the right opening frame, canonical machine identifier and 32-byte nonce. Send Challenge before lookup and revision checks. Keep the order of lookup failure, unknown machine, revision mismatch, Proof deadline, bad Proof shape and bad signature. Once Proof verifies, take the machine lock and check that the machine still exists. Validate the run identifier’s shape and the three counters inside that admitting write. For a known run, reject supersession before checking its exact saved boundary. For an unseen run, calculate the comparison against its post-reconciliation mark before applying the eligible-boundary rule below. A rejected admission commits no mark, warning, or command change and leaves the incumbent current. Use the existing retryable `invalid_argument` for malformed numbers or a rejected run pair. `src/contracts/testdata/link-codes.json` needs no change. The stop reason `impossible_ack` for an out-of-range Welcome is a local diagnostic and never was a link refusal reason.

**Wire choice.** `Welcome.last_event_number` remains field 3 and carries the durable mark from admission. The client reads it against the same two counters it announced on that attempt. That supplies the reset signal and the replay starting point. The signed proof text already authenticates the machine on the stream. Keep the two nonces and signed text, and use the already verified admission path. Use the existing durable outbox quantities to form one snapshot for Hello, and keep the Welcome decision consistent through the history write. The client build owns the coordination with any worker left from an earlier stream. The Go fields on `linkv1.Hello` are `AcknowledgedEventNumber uint64` and `LastEventNumber uint64`, with the corresponding `GetAcknowledgedEventNumber() uint64` and `GetLastEventNumber() uint64` methods.

**Exact link text edits.** The line numbers refer to the pre-build tree.

| Source and lines | Replacement |
|---|---|
| `events.proto:3–4, 8–10` | Events are numbered consecutively during ordinary append and their effects are applied at most once. The client keeps them until acknowledged or covered by a reconciled Welcome. The server writes the per-machine mark with the event’s effects and result settlements in ordinary replay. In admission it may also advance the mark over events the client has already forgotten, under the rules in `link.proto`. |
| `events.proto:11–14` | Welcome carries the durable replay mark. A mark from the announced acknowledged number through the announced last number, inclusive, means to check the required retained events, then forget the covered prefix and replay the rest in order. A mark above the announced last number means to discard the retained events and set the next number to the mark plus one in one history write. A mark below the announced acknowledged number ends the stream and the client dials again without applying that mark. |
| `events.proto:21–29, 83–84` | At the largest legal mark, each valid Event number is a repeat. EventAck.through carries the durable replay mark. A gap still ends the stream with `failed_precondition` and `event_gap` and gets no acknowledgement. A later stream compares its history at admission and sends from the mark it receives. |
| `events.proto:36–42` | One machine identifier has one outbox. The retained floor is its first retained number or the next number when the outbox is empty. A missing history file is created empty when the enrolled machine still holds its identity and key. Unreadable or damaged required history stops the client with `link_stopped` and exit status 0, without a fabricated advance. Remove the old incompatible-mark classification, the remedy to restore or enroll again, and the wave 1 exclusion. |
| `events.proto:43–44, 61–62` | An approved Welcome that can be applied prompts new Facts and HarnessesFound under the next available numbers. Approval by SetState can also prompt these reports. Event numbers begin at 1 for new history and are allocated within [1, M]. Reconciliation can move the next number forward before new reports are written. Numbers above M are refused. |
| `link.proto:18–19, 298–300` | State the three Hello ranges and the two relations from the values section above. Define both new counters and their snapshot and default-zero semantics. Define the immutable process boundary and the exhausted value. Keep the existing CommandResult watermark range. |
| `link.proto:20–21, 162–163` | The complete Protocol Buffers message, including its outer wrapper, is limited to 65,536 bytes on the wire and to 65,536 bytes after decompression. Transport framing is outside both measurements. |
| `link.proto:112–114, 124–131` | A well-formed opening Hello gets Challenge before lookup or revision checks. Clarify that well-formed here covers the frame, machine identifier and nonce. Keep the six status lines at 115–120. Defer the run identifier and counters until verified Proof and the locked admitting write. Malformed run fields and rejected run pairs use the existing `invalid_argument` without changing the stream or ledger. Delete the refusal for numerical mismatch between valid histories. |
| `link.proto:134–139` | Keep only the latest admitted run eligible to reconnect with its exact saved boundary. Reject a superseded run before comparing it with its saved boundary. Compare the histories only for an eligible admission. If the mark is below acknowledged, advance it to acknowledged and settle covered results by their outcomes. Then, if the mark is above last, keep it and perform the command and folder reset in the same write. Otherwise use ordinary replay. Use the resulting mark for readiness. After a server advance, log once with the machine and the old and new marks. After a client reset, log once with the machine, the server mark and the client’s last number. An unseen boundary at or below the resulting mark is admitted ready without a warning. For an unseen boundary above that mark, require first minus one to be at least the greatest staged result watermark and at least the pending run’s replay-through number. An equal eligible boundary favors the later successful admission. Save the run and its boundary together, and replace a previous pending boundary on reset. The clauses at 140–146 continue to describe promotion and the gate on sending. |
| `link.proto:153–154, 330–331` | Welcome contains the mark from the committing admission after the incumbent’s preceding writes. This includes a history advance made in admission. Zero means the mark has never advanced. The mark must be in [0, M]. Refer to `events.proto` for how the client resumes. |
| `commands.proto:6–8, 12–18` | The server sends a pending command only on a ready current stream. Resends continue under the existing expiry and attempt rules until a result is accepted or the server settles the command. If the client is behind, the admitting write settles handed-over commands without results as `history_restored` and never sends them again. Keep SetState eligible for resend under its standing rules, because Welcome carries the standing too. Commands never handed over remain queued in order. A staged result uses its recorded outcome. A folder operation still waiting for a report ends in failure and returns an error to the website caller. A repeated admission for the same reset settles only commands and operations that are still eligible. It cannot fail the same command a second time. For a mark advance when the server is behind, settle staged results covered by the new mark and fail no command. Keep ordinary result staging, expiry, event application order and the rule that an applied folder report wins over command settlement. |
| `commands.proto:19–26, 72–74, 93–94, 97–99` | Explain that repeated attempts use the stored receipt, with the existing time margin. Ignore a late result for a command already settled as `history_restored`. Use the previously recorded receipt watermark for a repeated result. Describe a new receipt’s watermark as the durable high-water, which can include a mark adopted from Welcome. Keep the earlier process rule for interrupted receipts. Name `history_restored` with `timed_out` and `removed` as server-only outcomes in the comment. Keep the `CommandOutcome` enum and the result fields exactly as they are. |

**Design artifact.** The history design dated 2026-11-28, sections on the number comparison, the two sides’ admitting writes, and contract text.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The server acts on a counter before it verifies Proof, overflows a mark, or resets a superseded run. | High | Write the validation sequence and bounds beside Open. The server owns the signed admission proofs. |
| A client reuses covered numbers or a command handed over before a reset is sent twice. | High | Document the three Welcome cases and the transacted reset. The paired proofs observe both event numbers and command identifiers. |
| A refreshed baseline hides a different historical change. | High | Require the in-place line diff against the old base before approval. Run the unchanged gate against the reviewed amendment commit after approval. |

**Risky mutations.** In the build’s temporary test copy, document history comparison before Proof, change the second case to compare against first instead of last, remove the inclusive lower Welcome bound, or remove the SetState exception. The corresponding link text tests must each fail. Change the number of either new Hello field and regenerate. The fixture and wire test must fail. Apply the limit only after decompression in either comment. The limit assertion must fail. Change one revision 1 hash or move its line and show that review comparison rejects it even when the candidate is supplied as the new base for the later gate. Change the amended revision 2 line or a frozen source comment against the accepted base. The line gate or source hash check must fail. Restore the candidate after each probe.

**Verify:** Run `go test ./src/contracts`, including the exact descriptor, text, proof and freeze tests above. Use the scratch Git fixture in the freeze test and the real history comparison in rollout step 2. No device is required. This proves the contract and its baseline. The server, client and paired suites prove runtime behavior later.

**Rollout detail for steps 1 and 2.** Write the regression tests before the edits. Run `pnpm --filter @lgtm/api generate` using the repository’s pinned generators and Go 1.26 or newer. Calculate the Secure Hash Algorithm 256-bit digests from the final bytes of `commands.proto`, `events.proto`, and `link.proto`, including comments. Change only those three digest strings on line 3 of `src/contracts/testdata/link-frozen.json`. Keep the revision 1 line, the leading comma, the order of files, and the `folders.proto` digest. In the README, retain the ordinary append rule and record the dated exception for this one review. Follow commit `699ce488`, which changed only the existing revision 2 line and moved the reference outside the amendment commit.

Set `old_base` from `.foundry/go-rewrite/link-base.txt` before review changes its value. Confirm `git merge-base --is-ancestor "$old_base" HEAD`. Read `git diff "$old_base" -- src/contracts/testdata/link-frozen.json` and require exactly the three digest changes on the existing line. Compare line 2 byte for byte with `git show "$old_base:src/contracts/testdata/link-frozen.json" | sed -n '2p'`. After all contract assertions pass, run the same PostgreSQL-backed gate with `LGTM_LINK_BASE="$old_base"`. It rejects the new line with the script’s frozen-record message. That rejection is part of the review evidence. Keep `scripts/check-go.sh` and its fail-closed comparison. The approved exception is a reviewed record change and a base move.

Commit the changed link sources, generated link files, tests, documentation and record together. Use the full hash of the exact reviewed commit that introduces the amended revision 2 line as `amendment_commit`. Review and verify that commit locally before merge. On the clean, integrated Contract candidate, run `git merge-base --is-ancestor "$amendment_commit" HEAD`, `pnpm --filter @lgtm/api check`, `go build ./...`, and `LGTM_LINK_BASE="$amendment_commit" bash deploy/test/with-postgres.sh -- bash scripts/check-go.sh`. Require exit 0 for each. The wrapper supplies real PostgreSQL. Use `git show "$amendment_commit:src/contracts/testdata/link-frozen.json"` to confirm that the recorded base has both approved lines. If integration rewrites the amendment commit, use the exact integrated commit that introduced the same reviewed record and repeat the ancestry and gate checks on it. Use the commit that introduced the line, even when a later candidate adds more files.

When that verified Contract commit lands, and before either application team starts its gated build, the orchestrator writes `LGTM_LINK_BASE=<full amendment commit hash>` to `.foundry/go-rewrite/link-base.txt`. The orchestrator also handles the release instructions in `deploy/README.md` and the `S4_CONTRACT_COMMIT` example in `deploy/test/README.md`. For later builds the proof from the root is `source .foundry/go-rewrite/link-base.txt; LGTM_LINK_BASE="$LGTM_LINK_BASE" bash deploy/test/with-postgres.sh -- bash scripts/check-go.sh`. A zero exit status covers formatting, module, vet, tests and frozen history.

**WS-2: The website receives failures it can show again**

This item adds the failed-request data to the existing public machine record. An initial snapshot and a saved update use the same record, so they can carry the list without changing the watch envelope.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | Field 20 is repeated and contains the command identifier, kind and failure timestamp. The enum numbers and field 19 match the build boundary. | `TestClientFailedRequestsAreAdditive`. |
| 2 | Snapshot and saved updates encode and decode both request kinds with the same identifiers and timestamps. The default record has no failed requests. | `TestClientFailedRequestsAreAdditive` and `TestClientFailedRequestsKeepTheirWatchJSONNames`. |
| 3 | Both sample routes still show their machines and accept sample actions using the regenerated schema. | The two browser cases ending in `keeps sign-in, sample work and simulation`, selected together by the command below. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 2.1 | Replace old warning behavior assertions with descriptor, binary and JavaScript Object Notation (JSON) checks for the new list. Retain a temporary shape assertion on field 19. | `src/contracts/service_discovery_test.go`, `TestClientHistoryRecoveryIsAdditive`. `src/contracts/review_a_test.go`, `TestClientRecoveryFlagKeepsItsWatchJSONName`. |
| 2.2 | Add the field, message and enum. Document when an entry appears or leaves on WatchClients. | `src/contracts/proto/lgtm/v1/clients.proto`, `Client` and `ClientService.WatchClients`. |
| 2.3 | Generate the website outputs. Check the two sample routes without creating sample failures. | `src/contracts/buf.gen.yaml`, `plugins`. `src/mock-server/src/sample.ts`, `sampleClients` and `sampleJoiner`. `src/frontend/src/server/api.ts`, `openServer`. `src/mock-server/src/serve.ts`, `engine`. `src/frontend/e2e/repository-discovery.spec.ts`, the loop over sample modes. |

**Files and dependencies.** Change `src/contracts/proto/lgtm/v1/clients.proto`, `src/contracts/service_discovery_test.go`, and the specified website test in `src/contracts/review_a_test.go`. Generate `src/contracts/gen/go/lgtm/v1/clients.pb.go`, `src/contracts/gen/lgtm/v1/clients_pb.d.ts` and `src/contracts/gen/lgtm/v1/clients_pb.js`. The Connect generators are run with the same command. No ClientService method changes. This item has no dependency on item 1 and has no risky ordering or authentication change. The combined build verifies both sample modes and both sets of generated outputs.

**Tests written first and existing test changes.**

| File | Test | First failing observation or change |
|---|---|---|
| `src/contracts/service_discovery_test.go` | Replace `TestClientHistoryRecoveryIsAdditive` with `TestClientFailedRequestsAreAdditive`. | Assert the full Client descriptor and that field 19 remains the singular warning boolean for this build. Assert field 20 as the repeated `lgtm.v1.FailedClientRequest`. Assert the entry tags and types and the three enum values. Use a fixture with one scan and one restart at distinct times in `all.clients` and `saved`. Both entries survive binary round trips. A default Client has an empty list. Assert the comments describe entry arrival, ten-minute expiry, same-kind replacement on that machine, and machine removal. Drop the old assertions about when a server sets or clears the warning. |
| `src/contracts/review_a_test.go` | Replace `TestClientRecoveryFlagKeepsItsWatchJSONName` with `TestClientFailedRequestsKeepTheirWatchJSONNames`. | Using dynamic fixtures, assert both watch arms have `failedRequests`, `id`, `kind`, and `failedAt`. Assert kind labels for scan and restart and an encoded timestamp for each. Decode the resulting JSON and compare the values. The replacement removes the old warning’s JSON-name assertion. |

The missing field fails the first test before code generation. The second also fails against the old descriptor. No test file changed by this item appears in the foundry’s frozen lists. The browser cases are existing read-only acceptance evidence for this item.

**Public names.** In `lgtm.v1`, add `repeated FailedClientRequest failed_requests = 20` on `Client`. `FailedClientRequest` holds `string id = 1`, `FailedClientRequestKind kind = 2`, and `google.protobuf.Timestamp failed_at = 3`. The enumeration is `FAILED_CLIENT_REQUEST_KIND_UNSPECIFIED = 0`, `FAILED_CLIENT_REQUEST_KIND_SCAN_HARNESSES = 1`, and `FAILED_CLIENT_REQUEST_KIND_RESTART = 2`. The identifier belongs to the command. Go exposes `Client.FailedRequests []*FailedClientRequest`, `FailedClientRequest.Id string`, `Kind FailedClientRequestKind`, and `FailedAt *timestamppb.Timestamp`. TypeScript exposes `Client.failedRequests`, `FailedClientRequest.failedAt`, and `FailedClientRequestKind.SCAN_HARNESSES` and `RESTART`. WatchClients sends the record when an entry appears, reaches ten minutes after failure, or is superseded by a stored command of the same kind on that machine. It removes the machine and its entries through the existing removed arm. The list is for scan and restart. A folder look surfaces its failure to the caller, and SetState has no person’s request.

`Client.event_history_needs_recovery = 19` remains defined in this build. Replace the old refusal and warning rules in its comment with a note that the field remains during this staged change. The server stops setting it in its build. The website stops reading it and, as its last item, reserves both the number and the name and regenerates the bindings. That item also replaces the temporary assertion with the corresponding reservation assertions.

**Design artifact.** The history design dated 2026-11-28, section on what leaves and arrives on the website. This work item adds data fields.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| Early removal of field 19 breaks the intervening build. | Medium | Check the current descriptor and defer the reservation until the last website item. |
| A change of tag or enum value corrupts the list seen by an application. | Medium | Pin field and enum numbers and decode both watch arms. |
| A sample producer stops compiling or omits a needed record. | Medium | Verify both type checks and the two existing browser runs. The shared sample instantiates `ClientSchema`, whose repeated fields start empty. |

**Verify:** Run `go test ./src/contracts`, `pnpm --filter @lgtm/mock-server typecheck`, `pnpm --filter @lgtm/frontend typecheck`, and `pnpm --filter @lgtm/frontend build`. In the browser environment with Chromium installed, run `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-discovery.spec.ts --project desktop -g "(in-page sample|standalone mock) keeps sign-in, sample work and simulation"`. Expect two passing cases. No physical device is required. The new bell behavior is checked in the website plan.

**Rollout detail for steps 1 and 3.** Prepare tests with the existing dynamic protobuf helpers. Generate with `pnpm --filter @lgtm/api generate`, verify deterministic generated files on the committed candidate using `pnpm --filter @lgtm/api check`, and run the type, build and browser checks above. The browser cases exercise the schema with the shared sample engine. Hand off the names to the application builds before the website’s final removal step.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe. The history design dated 2026-11-28 supplies the six decisions in row order, the comparison and reset rules, the contract edits, and checks H1 through H12. The assignment supplies five deliverables and a staged build order. The foundation design dated 2026-11-25 supplies decisions D1 through D23, rules R1 through R11, and checks C1 through C13. The latter three identifiers number the listed decisions, later-part rules, and first-wave checks respectively.

| Source item | Workstream | Status |
|---|---|---|
| History decisions 1 and 5. Server authority and fresh reports after covered events are dropped. | WS-1 | Contract text. The server and client builds perform the writes. |
| History decision 2. Retire the machine warning. | WS-1, WS-2 | Remove the warning rule from link admission. Retain the field while it is still read. The website build owns its removal. |
| History decision 3. Fail commands handed over to an older state that have no result. | WS-1 | Contract text for the reset and its exceptions. |
| History decision 4. A short-lived bell entry can ask again. | WS-2 | Data shape and visibility rules. The website owns rendering and actions. |
| History decision 6. Amend link revision 2 in place. | WS-1 | Frozen line, tests and reviewed base. |
| History comparison and number checks, three Welcome cases, both admitting writes, fresh reports, missing or damaged file, immutable run and receipts. | WS-1 | Exact link edits and test assertions. Application writes and logs are handed off. |
| History website changes and contract text. | WS-1, WS-2 | Both packages. The final reservation is in the website build. |
| H1, H2, H3 and H4. | WS-1 | The wire describes restoration, an empty start, settlement without resend and restart without a second execution. The website build owns the paired observations. |
| H5, H6, H7 and H8. | WS-1 | The text describes staged results, a missing folder report, ordinary replay, advancing an older server and the repeat-safe reset. The website build owns the paired observations. |
| H9 and H10. | WS-1 | Contract assertions cover refusal conditions and the stop on damage. Actual signed admission and client exits are checked by the server and client builds respectively. |
| H11 and H12. | WS-2 | The field boundary and list support the later website proofs. |
| Assignment deliverables 1 and 2, including issue 1702, point 1. | WS-1 | Link rules, Hello and the two measurements of the size limit. |
| Assignment deliverable 3 and the field 19 sequence in `names-history.md`. | WS-2 | Website fields and application handoff. |
| Assignment deliverables 4 and 5. | WS-1, WS-2 | Frozen record, baseline, generation, contract tests and sample checks. |
| Foundation decisions D1, D2 and D7. | WS-1, WS-2 | The three parts meet through the contract. Generated Go carries the changes to the server and client. The existing key proof gates admission. |
| Foundation decisions D3–D6 and D8–D23. | WS-1, WS-2 | These decisions stand. The contract preserves the current machine action, key exchange, and existing service shapes. The other product choices have no implementation in this build. |
| Foundation rules R1–R2 and R6, R8–R11. | WS-1, WS-2 | The contract is reviewed before its consumers. Stored commands, existing error codes, the existing proof text and real PostgreSQL remain the basis of the implementation and its checks. The build copies no code. |
| Foundation rules R3–R5 and R7. | WS-1, WS-2 | Database writes, the change feed and watch implementation belong to the server plan. The contract keeps the shared watch shape. |
| Foundation checks C1–C9 and C13. | WS-1, WS-2 | Existing deployment, session, enrollment and presence proofs retain their owners. No stack or session change is included. |
| Foundation checks C10–C12. | WS-1 | Authentication and ordinary replay stay covered. The full gate runs against real PostgreSQL. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md` and the epic put exchanges between parts in `src/contracts/proto/`. | Edit the Protocol Buffers sources and regenerate with the repository’s generators. Scope `contract/contracts` is validated in `scouting.md`. |
| `AGENTS.md` gives the server and client one Go rewrite. The foundation design uses one root module and real PostgreSQL for integration tests. | Link bindings regenerate into the current module. The Go history gate runs with the existing PostgreSQL wrapper. |
| `AGENTS.md` requires credit for copied code. | The work uses the existing generators and locally written rules and tests. |
| `AGENTS.md` asks that findings be recorded. The task confines the planner to its four artifacts. | Carry issue 1702, point 1, into the plan and refer issue 1714 to the security list in the result. The orchestrator receives the release-document requests. |
| `AGENTS.md` routes finished work through a pull request. The bounded task gives the orchestrator the merge and forbids staging in this planning pass. | The rollout identifies the exact reviewed commit for the later pull request. This planning pass produces text files only. |
| The task keeps new edits in the Contract team’s folders. | Link and website contract work is under `src/contracts/`. The scope permits `scripts/check-go.sh`, which remains the enforcement gate. The shared pointer, deployment files, and runtime tests are owned by the named follow-on owners. |
| `AGENTS.md` requires dated decisions and repository-relative evidence without private details. | The log dates the choices. The plan cites repository-relative source files. Run the privacy check in the self-check. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| WS-1, item 1 | contracts | `contract/contracts` | medium | The wire changes are additive. The revisions, ordering and test fixtures need a coordinated review. | yes | None. Can build beside item 2 in a separate working copy. |
| WS-2, item 2 | contracts | `contract/contracts` | low | One additive message and enum follow the existing watch pattern, with focused descriptor tests. | yes | None. Can build beside item 1 in a separate working copy. |

Load. One vendor supplies the planner, test writers, builders, and reviewers. Count one planner, two test writers, two builders, one plan reviewer and two focused build reviewers, for eight launches including this planner. The remaining allowance was unavailable in the planning session. The orchestrator can compare the seven estimated launches with the vendor’s remaining allowance at dispatch and schedule them around the reset where needed. This is a sizing estimate.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| One planned round | Pending | The orchestrator runs the plan review after this self-check, per the bounded assignment. |

</details>
