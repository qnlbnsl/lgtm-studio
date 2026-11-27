REVISE

**Top finding.** A superseded ready run can match the reconnect rule while a newer run is still draining, leaving command admission dependent on which rule the server checks first.

Plan and decision references use revision `1cf3deb0`. The candidate in `attack/plan-v2.md` matches that revision's plan byte for byte. Findings describe the proposed behavior. Source references identify the existing boundaries that each patch must change or preserve.

## Findings

### logic-r2 01 — MAJOR — R7

The pending-pair patch does not say whether the previously ready run remains eligible to reconnect. Run A starts at 1 and becomes ready. After the mark reaches 5, run B is admitted at 10 and disconnects mid-drain. A delayed authenticated admission for A repeats its saved ready pair. The ready-run reconnect rule recognizes A, while the new pending-boundary rule rejects A as a different run below 10. A builder who checks the ready match first can give A commands before the pending prefix drains. There is a second case at a tied boundary. If A, then B, both become ready at first number 6 while the mark stays 5, another admission for A passes the numeric checks and can run the ready callback again.

Make the latest admitted pair the only pair eligible for a same-run reconnect. A pending admission supersedes the old ready pair for stream and command admission. Reject a previously superseded run before considering numeric replacement, including at a tie. Define the winner for two as-yet-unseen runs with the same boundary and preserve the permitted replacement when the previous process appended nothing. Keep the data needed to enforce the rule through a server restart. Test old-ready admission during pending replay and an A, B, A sequence at equal boundaries.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:143–145`, `.plans/plans/go-s4-contract/plan.md:159`, `.plans/plans/go-s4-contract/plan.md:299`, `docs/specs/2026-11-27-running-an-agent-design.md:115–124`, `src/server/internal/link/handshake.go:224–249`, `src/server/internal/link/hub.go:99–141`.

### logic-r2 02 — MAJOR — R4

The held receipt keeps the key reserved, but the existing Store replay hands back only the operation identifier after its transaction. At the retirement boundary, a matching replay can acquire the key, read the identifier and commit. Retirement then acquires the same key and removes both rows before Projects performs its first operation read. The first replay won the serialization and still loses the saved result.

Give Store a replay hook that Projects uses to read and copy a terminal result while the key is held. For an operation observed open under that lock, tie the bounded wait to the existing promise to hold the result for at least a day after settlement. Keep retirement behind the same key and define what a request sees when retirement wins. Add a barrier after the receipt lookup and before the operation read. Force retirement at that barrier and assert the response to the replay.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:236–238`, `.plans/plans/go-s4-contract/plan.md:303`, `.plans/plans/go-s4-contract/decisions.md:40`, `docs/specs/2026-11-27-running-an-agent-design.md:215–222`, `src/server/internal/store/receipts.go:69–125`.

### logic-r2 03 — MAJOR — C11

The recovery flag can clear without evidence that the refused client recovered. At mark 10, allow an existing ready stream to remain current. A different run with boundary 9 is refused and sets the flag. With no staged watermark, a next event from the existing ready stream satisfies the plan's clear condition. The new run can still only retry its refused pair. The Machines page can lose the instruction without any repair. The shared feed reads the current record, so a set and clear before a watcher reads the set row can hide the instruction entirely.

Record the admission generation and required history boundary when the contradiction is committed. Specify what subsequent admission and what proof of recovery may clear it. An event or reconnect from the pre-refusal ready run must not clear it merely because it already satisfies its own fence. Keep the website field boolean. Make the recovery procedure explicit about closing the retrying process before restoring its state, then starting it with the restored counter and a fresh run identifier. Exercise a live old stream, no staged result, a rejected new stream and a newly opened Machines watch before and after actual recovery.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:143`, `.plans/plans/go-s4-contract/plan.md:155–161`, `.plans/plans/go-s4-contract/plan.md:298`, `.plans/plans/go-s4-contract/decisions.md:37`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:138–139`, `src/server/internal/machines/watch.go:90–100`.

### logic-r2 04 — MAJOR — D5

The patched transition and ownership rules serialize on one machine, while a repository can span machines. Suppose one repository has a folder on machine A and another on machine B. Two removals take different machine locks. Each removes its own folder and sees the other's uncommitted deletion as an existing folder. Both can commit without removing the now-empty repository. An attachment on B can also race the last-folder cleanup on A and base its decision on a disappearing target.

Have Projects serialize all membership changes and orphan cleanup on the repository row after taking the initiating machine lock. For a removal that touches several repositories, lock them in stable identifier order. Re-read membership and target existence after obtaining the repository lock. Keep the per-machine folder uniqueness constraint. Prove two concurrent removals produce one repository removal and an attachment race produces either a live repository containing the new folder or a fixed missing-repository reply.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:234`, `.plans/plans/go-s4-contract/plan.md:240–242`, `.plans/plans/go-s4-contract/plan.md:289`, `docs/specs/2026-11-27-running-an-agent-design.md:250–259`, `src/server/internal/machines/records.go:59–70`.

### logic-r2 05 — MAJOR — D5

Removing the Secure Shell user from a relative origin can discard part of the repository's identity. Two users on the same host can each have a different repository at the same path relative to their account. The patched normal form removes the two user names and treats the remaining host and relative path as equal. An explicit attachment then succeeds even if the first commits differ, because the present-origin branch ignores them.

Keep the prohibition on sending user information. Treat a relative Secure Shell origin as withheld for identity when the client cannot establish an account-independent identity from local facts. The simplest supported rule omits all relative Secure Shell origins and retains comparison for the supported absolute forms. Test unequal account-relative repositories with the same host and relative path. They must never pass the present-origin equality check.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:167–169`, `.plans/plans/go-s4-contract/plan.md:242–244`, `docs/specs/2026-11-27-running-an-agent-design.md:155–157`, `docs/specs/2026-11-27-running-an-agent-design.md:204–211`.

### logic-r2 06 — MINOR — R7

An uncertain diagnostic commit can disconnect the existing stream if Link reuses its current admission result path. Suppose the connection to PostgreSQL drops while acknowledging a commit that could only write the flag and feed reference. The handshake currently sends an unknown write outcome through the admission hub. That branch cancels the old stream and clears it even though the candidate's write could never replace it. The recovery patch promises to leave the existing stream intact.

Distinguish a diagnostic-only write from a write that could replace the current stream, including on an uncertain commit. Return the existing database-error response to the candidate when a diagnostic commit cannot be confirmed. Keep the current stream intact on that path. Confirm the durable flag and feed are either both present or both absent after retry.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:159`, `src/server/internal/link/handshake.go:224–237`, `src/server/internal/link/hub.go:113–134`.

### logic-r2 07 — MINOR — R8

The pinned refusal table gives no precedence when an add names an unknown repository and the resolved folder already belongs to a different saved repository. The new ownership-first wording gives `already_exists`, while a handler that checks target existence before inspecting gives `not_found`. Both can pass the proposed single-condition assertions and the browser can get different advice about the same request.

Choose and state one precedence for this overlap. Apply it at the same operation stage in both the source comments and the handler. Test an explicit, well-formed but unknown target for an already saved path and for a previously unsaved path. Give each case an exact code and sentence.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:213`, `.plans/plans/go-s4-contract/plan.md:242`, `.plans/plans/go-s4-contract/plan.md:259–265`, `.plans/plans/go-s4-contract/plan.md:304`.

## Answers to the attack questions

### Assignment questions

1. Each registered patch is checked in the table below. Duplicate register entries share a row when they check the same behavior.
2. The Machines diagnostic is the one marked unreviewed mechanism. Its zero value, first write and removal are defined. Its clear predicate and uncertain-commit branch have the failures in findings 03 and 06.
3. Nothing found to recheck as a rejected finding. Every register entry is marked fixed or fixed another way. The alternate fix for the pre-Welcome refusal was checked directly.
4. The new or changed cross-part names and their readers and writers are listed below. The named gaps concern the Store replay hook, Link's effective admitted pair and the diagnostic projection.
5. The admission, diagnostic, settlement, retry and origin truth tables below identify the conflicting and empty cells.
6. The recovery, admission and retry promises behind logic-r1 01, 02 and 04 still have failing cases. The instruction can disappear before repair, old-ready admission can conflict with the pending fence, and an admitted replay can lose its saved result. The original same-pending-run boundary change, late-report overwrite and missing standing receipt have direct fixes. The normalization patch closes the origin-precedence ambiguity and introduces a false match.
7. The patches touch all eight lens questions. The answers below distinguish plan guarantees from checks that need the built programs.

### Each registered patch

| Register entries | Result at the reviewed revision |
|---|---|
| architect-r1 01 and threat-r1 05 | The patch names both dispatch and preflight for all three generated procedure constants. The router has separate registration and preflight paths. The listed- and unlisted-origin probes cover the split. Nothing found in this patch. `src/server/internal/api/server.go:173–179`, `src/server/internal/api/server.go:245–267`, `.plans/plans/go-s4-contract/plan.md:290`, `.plans/plans/go-s4-contract/plan.md:304`. |
| architect-r1 02 and logic-r1 04 | Holding the existing receipt prevents another procedure from taking the key while the hold stands. Retirement can still overtake the operation read. See finding 02. |
| architect-r1 03, critic-r1 1, logic-r1 01 and threat-r1 01 | The authenticated Machines route can surface the error before Welcome and preserve the local fault path. Its clear and uncertain-commit rules need findings 03 and 06. |
| architect-r1 04 | The waiting client can replay the retained prefix before receiving gated `SetState`. The existing approval check still allows Facts and consumes the other waiting reports. The pairing of stale ready and pending admission affects the gate as in finding 01. `src/server/internal/link/inbox.go:75–91`. |
| architect-r1 05, critic-r1 4 and logic-r1 05 | The first `SetState` outcome and watermark now have a durable receipt, including past ordinary receipt expiry. The patch closes the standing fast-path gap in the design. The declared latest-attempt and standing-revision checks still apply. `.plans/plans/go-s4-contract/plan.md:147`, `src/client/internal/link/commands.go:132–149`. |
| architect-r1 06 | The reviewed boundary keeps a caller-owned transaction and adds a returned command identifier and callbacks. The existing enqueue accepts that transaction. Keep machine-before-repository lock order in the new Projects callbacks as in finding 04. `src/server/internal/link/outbox.go:21–46`, `.plans/plans/go-s4-contract/plan.md:289`. |
| architect-r1 07, critic-r1 2 and logic-r1 06 | Existing ownership now precedes identity comparison and the supported origins have named equality rules. The relative Secure Shell case falsely equates two origins. See finding 05. |
| architect-r1 08, critic-r1 3 and threat-r1 04 | The common filter covers old reports, new reports and client logs. It checks the required legacy suffix without renumbering. An empty workspace already passes the receiver's current validation. Nothing found in the specified masking and local-fault behavior. `.plans/plans/go-s4-contract/plan.md:167–169`, `src/server/internal/link/inbox.go:100–124`. |
| architect-r1 09, critic-r1 5, logic-r1 07 and threat-r1 06 | The source table, handler checks and replacement snapshot close the named assertion gaps. The reused malformed-body path covers both unary and streaming handling when Projects installs both the codec and wrapper. The new pinned table still leaves overlapping refusals open. See finding 07. `src/server/internal/api/session_decode.go:25–52`. |
| architect-r1 10 | The orchestrator owns the named in-process tests and must run real PostgreSQL with explicit passes. Stack owns the installed upgrade. The present fixture skips without the database setting, so the no-skip rule is substantive. Nothing found in the assignment. `src/linktest/fixture_test.go:29–30`, `.plans/plans/go-s4-contract/plan.md:294`, `.plans/plans/go-s4-contract/plan.md:306`. |
| critic-r1 6 | The new readme assertion supplies red and the comparator supplies characterization. Confirmed live in a scratch probe using the comparator extracted from the script. The three positive cases passed and the four named changes failed. Bypassing the compare made all seven pass. `scripts/check-go.sh:31–49`. |
| critic-r1 7 | The separate fresh sessions, sign-in and procedure observation at each transport address the original vacuous negative. The real and old-real mode checks remain assigned to the deployed gate. Nothing found. `.plans/plans/go-s4-contract/plan.md:276`, `.plans/plans/go-s4-contract/plan.md:306`. |
| logic-r1 02 | The durable pending pair closes the original same-pending-run change from 10 to 6 after restart. Its interaction with the older ready pair and tied replacement boundaries remains open. See finding 01. |
| logic-r1 03 | The open-only guards close the original late-report overwrite. Late eligible reports still advance and acknowledge. The per-machine lock does not serialize separate machines' repository writes. See finding 04. |
| threat-r1 02 | Sealing the old process before a new append closes the interrupted-result boundary gap. Explicit legacy recognition separates zero from damaged metadata. The post-build check below exercises missing version 2 watermark presence and unreferenced seals. `.plans/plans/go-s4-contract/plan.md:147–149`, `src/client/internal/link/receipts.go:34–60`. |
| threat-r1 03 | The patch limits inspection to read-only local facts and names an inert sentinel. No write or network exception was found in the patched text. Runtime proof still needs the built client. `.plans/plans/go-s4-contract/plan.md:165`, `.plans/plans/go-s4-contract/plan.md:305`. |

### Names that cross a part boundary

| Name or persisted value | Writers and readers |
|---|---|
| `Hello.run_id`, `Hello.first_event_number`, admitted pair and ready callback | Client Link writes the announcement and repeats it. Server Link validates, writes and reloads the pair, and uses it in admission, events and command scheduling. Link owns callback invocation and Projects registers its hooks. The superseded state is missing as in finding 01. |
| `CommandResult.last_event_number`, sealed run and standing receipt | Client Link and its local store write and read the outcome and watermark. Server Link reads the field in result matching and owns staging, settlement, sweep exclusions and removal. Projects receives the terminal callback. |
| `Client.event_history_needs_recovery=19` | Contract generates the field. Server Link writes and clears the durable diagnostic and adds the Machines feed reference. Machines reads it for snapshot and saved projections, including offline, and Website reads it on `WatchClients`. Old records and sample records default to false. Wire the read through the Link interface for both projection paths. Ensure any shared full `Client` projection, including Approve and Rename replies if they can replace a website machine record, preserves the value. `src/server/internal/machines/watch.go:70–99`, `src/server/internal/machines/service.go:58–71`, `src/server/internal/machines/records.go:94–101`. |
| Enqueue identifier, operation identifier, held receipt and retirement | Link writes the command identifier through the caller's transaction. Projects stores the operation and registers its callbacks. Client Folders echoes the operation and command identifiers in `FolderSeen`. Store owns the caller-and-key reservation. Both folder calls and other keyed calls read it, and pruning skips it. The replay must also read a protected terminal result as in finding 02. |
| `remote`, `remote_omitted` and normal form | Client Folders writes the link facts. Projects validates, stores, compares and projects browser facts and repository records. Website reads the browser fields without having to repeat the identity decision. See finding 05. |
| The three generated procedures, discovery name and refusal pairs | Contract publishes the names and expected pairs. The API router reads the procedures for dispatch and preflight. Server session discovery publishes the generated full service name. Website reads the advertisement and handler replies. The Contract, handler and browser checks cover the new readers, subject to finding 07. |

### Where rules interact

| State and input | Required or unresolved outcome |
|---|---|
| Malformed announcement or changed boundary of the latest admitted run | Generic `invalid_argument` with no diagnostic and no change to admission. |
| Latest admitted run repeats its boundary while pending | Resume replay with the gate closed. When the mark reaches the predecessor of the boundary, run the ready transition once. |
| Latest admitted run repeats its boundary when ready | Resume with the gate open. The mark or a staged watermark may already cover the run's original boundary. The new-run rejection applies only to a different run. |
| Former ready run repeats its own pair while a newer pair drains | The old-ready rule and the pending-boundary rule overlap. Reject the superseded run as in finding 01. |
| Never-admitted run has a valid boundary above the mark and staged watermarks, at or above the pending boundary | Admit. It is ready immediately when the mark equals the boundary less one and pending when there is a prefix left. Define tie precedence as in finding 01. |
| Different run falls at or below the mark or a staged watermark, or below a pending boundary | After proof, commit the diagnostic only and refuse. A repeat changes neither the ledger nor the flag. |
| Diagnostic set, earlier ready stream advances, and no staged watermark exists | The current clear predicate permits a false clear. The previous stream's progress cannot establish recovery of the rejected run. |
| Diagnostic set, recovery admission drains through all required marks, or machine is removed | Clear with the recovery change, or delete with the machine. If the diagnostic commit is uncertain, preserve the old stream as in finding 06. |
| Current valid result for the latest attempt and watermark at or below the mark | Settle once. A terminal Projects operation keeps its original result. |
| Current valid result above the mark, repeat result, or sweep while staged | Stage the first result. Repeats and sweeps leave it staged. At the awaited event, apply or consume, advance and settle. Removal ends the pending command as removed. |
| Eligible folder report while Projects operation is open or terminal | Apply once while open, ahead of any same-write settlement. When terminal, consume and acknowledge without changing Projects. A replaced or removed stream cannot advance the mark. |
| Held key and same digest, or held key and changed digest or procedure | Replay the saved operation on a match. Reject the conflict. At retirement, the lookup and promised operation read need the same serialization as in finding 02. |
| New folder, both origins present | Compare normal forms and ignore first commits. Different transport, path case, suffix or trailing slash fails. A different account can give a false match as in finding 05. |
| New folder, at least one origin known absent | Equal present first commits are required. This includes an absent and a withheld origin under the stated rule. If neither origin is known absent, a withheld origin cannot prove identity. |
| Existing folder, target omitted or matches, or explicit target differs | Return the stored row for the first two cases even if origins changed. Refuse a different target. Unknown target overlap needs finding 07. |

### Logic lens questions

1. Events and watermarks use the durable sequence, not timestamps. Repeats get a cumulative acknowledgement, gaps receive none, and the exhausted next-counter sentinel can be a boundary only. Attempts still increase without wrapping and only the latest dispatched attempt can settle. The server clock controls the 20-second command expiry, 23-hour offer grace and 24-hour sweep. The client keeps the 24-hour receipt margin and the standing exception. Run ties and supersession need finding 01. The request waiter and retention clock interact as in finding 02.
2. A failed event write gives no acknowledgement. A lost acknowledgement is covered by Welcome. A caller timeout ends the wait, while a duplicate reuses the saved result. A damaged required replay stops locally. The diagnostic repair and uncertain-commit cells need findings 03 and 06. The sealed-run write supplies the old process's boundary across a crash. Session expiry closes the watch and requires sign-in.
3. The interaction table supplies the open cells for pending versus ready admission, diagnostic clearing, receipt lookup versus retirement, repository identity and overlapping refusals. Ordinary staged-result and open-operation transitions have a single winner under the machine lock.
4. An empty run identifier or zero first number is invalid. Zero watermark is valid for empty history or a recognized legacy receipt. The plan must distinguish it from missing metadata in a version 2 receipt. Empty service lists and absent diagnostic fields decode as no opt-in service and false. An unset report arm or zero or unknown refusal is consumed without effect. An absent target selects create-or-reuse and a present empty target is invalid.
5. Admission, event, result, sweep and removal contend on the machine lock. Report and result only transition an open operation. Store's key lock spans a receipt decision, while Projects can read its reply later. Two different machine locks still permit simultaneous writes to one repository. See findings 01–04.
6. The scratch comparator mutation was confirmed live. Item 2's descriptor, text, digest and byte assertions can catch the listed mutations when the test inputs are independent. A runtime sender that claims a command early still passes those contract tests. Item 3's refusal mutation fails an independent code assertion, but two true conditions can still return the wrong one. Item 4's field mutations reach reflection assertions, and the full-name mutation needs an independent expected name to fail. A live Machines projection that always sends false can still pass synthetic compatibility checks.
7. For item 1, matching documentation phrases and the named comparison cases can coexist with contradictory prose. For item 2, correct schema comments and fixtures can coexist with a client that accepts a missing revision 2 watermark as zero. For item 3, all the enumerated single-condition cases can pass while retirement overtakes the last operation read. For item 4, correctly generated fields can coexist with a diagnostic that clears on unrelated traffic. The dependent gates need to observe the real persisted state and frames.
8. Keep an explicit latest admission and durable supersession state. Protect the receipt lookup and result lifetime together. Give repository membership one serialization point shared by machine removals and additions. Encode version 2 receipt watermark presence explicitly and validate it before using zero. The report union and conditional open-to-terminal operation write already prevent two common invalid shapes.

## Post-build checks

- **logic-r2 P1.** In the paired Link fixture, use a valid version 2 finished receipt with explicit zero and a separate receipt with the watermark missing. Replay the first unchanged. The second must stop through the local store-fault path, with no fabricated result. Crash before and after sealing an empty old run, then restart twice and append later events. The old seal stays zero. The client and orchestrator compare preserved receipt bytes and wire results.
- **logic-r2 P2.** In the paired Link fixture, approve a waiting machine between delivery of the old prefix's final event and its acknowledgement. Hold the send permits and observe the durable cursor, event change and outgoing frames. The gate stays closed until the final event commits. That acknowledgement precedes `SetState` and fresh reports follow approval. The orchestrator and Link compare frame order and database state.
- **logic-r2 P3.** Disconnect a client after the server stages a result above its mark. Advance the server's test clock beyond the ordinary sweep and offer cutoffs and run the actual sweeper. The result remains staged, has no new attempt and settles once when the real prefix returns. The orchestrator and Link compare command rows and report callbacks before and after replay.
- **logic-r2 P4.** Use the real website and server with one machine flagged offline and another flagged while waiting for approval. Reopen the Machines watch and check both snapshot and saved updates, including across a restart and a Machines mutation. The instruction persists through unrelated changes, clears after the repaired admission drains, and disappears when the machine is removed. Website and Stack compare the visible page with the observed watch records.

## Grade

6 out of 10. 0 blockers, 5 majors and 2 minors.
