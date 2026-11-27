---
topic: go-s4-contract
status: attack
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# The contract for a repository on a machine

## What the sprint 4 contract enables

Sprint 4 needs a reviewed agreement for how the website asks a machine about a repository and how the machine reports what it found. The website gains calls to inspect and add a folder and to follow saved repositories. A real server advertises these calls so the same website build can still open the in-page sample, the mock server, and an older real server.

A result can reach the server while the folder report is still in the client's saved buffer. Link revision 2 gives the server the event number it must reach before settling that result. A restarted client tells the server where the new process's events begin. If that announcement contradicts the server's saved history, the Machines page tells the owner how to recover while the client retries. A client that detects missing required events locally stops with the same instruction. The owner can restore the original event history or remove and enroll the machine again. (architect-r1 03, logic-r1 01, critic-r1 1, threat-r1 01)

This plan fixes the message names, numbers, and promises before the server, client, website, and stack teams build against them. The approved sprint split puts starting an agent and making a new repository on disk in sprint 5.

## Key decisions

The server offers a folder inspection through its existing link. The machine returns either the facts it found or one fixed refusal reason. A result that reaches the server before those facts stays with the command across a restart. Recovery uses the saved events in their original order. Removal ends an unrecoverable command as removed.

The contract for the website stays at revision 1. A list in the server's description advertises the repository service by its full name. A missing list gives the website the same information as an empty list. Both folder calls use the existing request-key header, so the same action can find its saved operation after a lost reply.

Full log → [decisions.md](decisions.md)

## Workstreams

### The new link can be frozen against revision 1

The history check already compares each old revision line byte for byte. The new line carries its own separator, so the old line survives as it was. The build proves that the record passes against revision 1 and that a rewrite of either frozen line fails once that revision is the baseline.

### The machine can report a folder in order

The contract names the data needed to inspect a folder and the refusals it can return. The tests compare the generated message layout and show that the old proof and event examples still decode. They also pin the byte budgets and the words that define acknowledgement, restart, and result ordering. The sprint's running-stack checks prove those rules against both programs.

### The website can discover and follow repositories

The website can tell when the server implements the repository calls. A repository watch gives it the whole list first, and then replaces or removes records by their identifiers. The add call returns the saved repository and folder, so the browser can match the reply to the watch. The existing sample and mock code goes through its normal build and discovery paths with the new generated types.

## Rollout order

1. Review this plan and hand its names to the five dependent planners after it clears review.
2. Build and review the contract against the existing revision 1 freeze. Keep the old link programs and the two sample modes usable through this contract change.
3. Set the revision 2 baseline to the reviewed landed contract commit. Use that commit for the history check of subsequent work.
4. Build the dependent sprint 4 plans. Deploy the revision 2 server with the matching client installer. An installed revision 1 client is refused until it is upgraded. Use the same website build to verify discovery on the real server, in-page sample, and mock server.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

Workstream identifiers use the prefix WS. Protocol Buffers identifiers and machine-readable error codes below are literal names. A universally unique identifier (UUID) is spelled in its canonical lowercase form. Text uses Unicode Transformation Format, 8-bit (UTF-8). A kibibyte is 1,024 bytes. A Secure Hash Algorithm 256-bit (SHA-256) digest here is lowercase hexadecimal.

**WS-1: The new link can be frozen against revision 1**

The existing history gate checks line identity and line number. Document the append layout and prove the two-baseline transition with the real script in a scratch repository.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | The append rule is documented. The real gate accepts the append against revision 1 and rejects edits to either protected revision line under the appropriate baseline. | `TestLinkFreezeAppendIsDocumented`, `TestLinkFreezeAppendAgainstPriorBaseline`, and `TestFreezeCheckFailsClosed`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Document and test how the next freeze is appended. | `src/contracts/README.md` → Rules. `src/contracts/freeze_test.go` → `TestLinkIsFrozenAtItsRevision`, `TestFreezeCheckFailsClosed`. `scripts/check-go.sh` → `link_record`, `link_base`. |

**Item 1. Append rule and history tests.** Change `src/contracts/README.md` and `src/contracts/freeze_test.go`. This item starts first. Items 2 and 4 wait for it because they also edit the readme. Item 3 is logically independent if the build lead gives its code generation a separate checkout. This item implements foundation rule 2 and the newer design's revision 2 freeze.

- Write `TestLinkFreezeAppendIsDocumented` first in `freeze_test.go`. Require the readme to state that frozen link source hashes include comments, old record lines are kept byte for byte at their original line numbers, and the separator belongs on the new line. Require the note that the review uses the previous reviewed commit and that the baseline advances after review. It fails on the current readme.
- Use `TestLinkFreezeAppendAgainstPriorBaseline` to characterize the existing comparator, following the scratch-module pattern of `TestFreezeCheckFailsClosed`. Require valid JavaScript Object Notation (JSON) and exit 0 for the leading-comma append against revision 1, and for the revision 2 commit and a later unchanged commit against the fixed revision 2 base. Require exit 1 with the frozen-record diagnostic for a revision 1 line edit, a trailing comma on the old line, a moved old line, and a revision 2 hash rewrite against the revision 2 base. Keep the missing and pre-freeze base cases. (critic-r1 6)
- The public input is `LGTM_LINK_BASE`. The file is `src/contracts/testdata/link-frozen.json`. Line 2 remains byte-for-byte identical to commit `fcfa96c3`. Line 3 starts with two spaces and a comma before the new object. The closing bracket follows on line 4. Hash the complete bytes of the four source files for the new entry. The raw hash check ignores generated code, the website's Protocol Buffers files, and the code table outside the frozen directory. Its separate readme check expects one current freeze declaration. Expand the stale test comment about three files to describe the directory scan.
- Risky because the baseline orders revisions. The readme assertion supplies the initial red test. In a temporary copy of the script, bypass the old-line comparison and show that all four named rejection assertions fail while the positive controls pass. Restore the comparison and run the real gate against the built append in item 2. (critic-r1 6)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A formatter appends the separator to line 2 or a caller supplies a mutable baseline. | High | Prove the append layout and reject historical edits. Give dependent gates the reviewed immutable commit identifier. |

**Verify** Run `go test ./src/contracts -run 'TestLinkFreezeAppend|TestFreezeCheckFailsClosed|TestLinkIsFrozenAtItsRevision' -count=1`. No device or database is needed for the scratch-repository cases.

**Rollout detail** For step 2, run the full contract build and `LGTM_LINK_BASE=fcfa96c3 bash scripts/check-go.sh` through the review of the built contract. Use a revision 1 baseline from this repository's history, with its record blob present. The script trusts the caller's baseline, so the orchestrator pins `fcfa96c3` for this review. At step 3, after the contract is reviewed and lands, record that exact landed commit and use it as `LGTM_LINK_BASE` for the dependent branches and later gates. Re-run the gate with that commit. Never use a candidate branch's moving head for a review or change the historical object to make an append pass.

**WS-2: The machine can report a folder in order**

The wire changes are additive. The two existing declarations of the link revision advance together and the proof labels stay stable. Publish the link, its descriptions, its generated files, and its matching new freeze in one work item so the contract always has a matching current record.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 2a | The additive wire layout, revision declarations, and revision 1 examples are checked on generated messages. | `TestOpenMatchesItsDescription`, `TestCommandsMatchItsDescription`, `TestEventsMatchItsDescription`, `TestFoldersMatchItsDescription`, `TestEveryLinkNameIsDescribed`, `TestRevisionOnePayloadsDecodeUnderRevisionTwo`, and `TestProofExamplesHoldTogether`. |
| 2b | The contract pins one acknowledgement per eligible receipt, the run and result fences, loss recovery, the folder fact and refusal tradeoffs, and the byte budgets. | `TestRevisionTwoLinkRulesAreWritten`, `TestFolderWireBudgets`, `TestEveryLinkRuleIsWritten`, and `TestLinkIsFrozenAtItsRevision`. |
| 2c | The revision 2 sources and artifacts are reproducible and pass against the prior freeze. | `pnpm --filter @lgtm/api check` and `LGTM_LINK_BASE=fcfa96c3 bash scripts/check-go.sh` exit 0. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 2 | Write and freeze link revision 2 in one change. | `src/contracts/proto/lgtm/link/v1/{link,commands,events}.proto` → `Hello`, `LinkService.Open`, `Command.action`, `CommandResult`, `Event.report`, `EventAck`. New `folders.proto` follows `commands.proto: SetState` and `events.proto: Facts`. `src/contracts/open_test.go` → `TestEveryLinkNameIsDescribed`. `src/contracts/envelopes_test.go` → `newMessage`. `src/contracts/rules_test.go` → `TestLinkNumbersAreWritten`. |

**Item 2. Revision 2 fields, folders, and comments.** Change the three existing files named in its task row. Create `src/contracts/proto/lgtm/link/v1/folders.proto`. Change `src/contracts/README.md`, `src/contracts/testdata/link-frozen.json`, `src/contracts/testdata/golden/{open,commands,events}.txt`, `src/contracts/open_test.go`, `src/contracts/envelopes_test.go`, and `src/contracts/rules_test.go`. Create `src/contracts/testdata/golden/folders.txt` and `src/contracts/testdata/link-v2-limits.json`. Regenerate `src/contracts/gen/lgtm/link/v1/{link,commands,events,folders}_pb.{js,d.ts}`, `src/contracts/gen/go/lgtm/link/v1/{link,commands,events,folders}.pb.go`, and `src/contracts/gen/go/lgtm/link/v1/linkv1connect/link.connect.go` with the existing configuration. In the grouped paths, braces enumerate exact files. Verify the diff and retain only generator-produced changes. This item needs item 1. Serialize the code-generation runs with items 3 and 4 in a checkout, because generation cleans the output directory. This item implements foundation rules 1, 2, 6, 7, 8 and 9, foundation check C11, newer checks R1, R2 and R6 to R11, and issue 1674 point 4.

**Tests written first.** Use reflection by string and existing golden-description helpers so absent symbols yield an assertion failure. Freeze the expected files and tests before implementation.

- In `open_test.go`, expand `TestEveryLinkNameIsDescribed` and `TestEveryLinkRuleIsWritten` to include the new `folders` description and source. Update the three existing descriptions. In `envelopes_test.go`, add `TestFoldersMatchItsDescription` for every field, union, enum value, and presence in the link layout below. Use `matchDescription` and explicitly count the new declarations through the existing coverage test.
- In `envelopes_test.go`, add `TestRevisionOnePayloadsDecodeUnderRevisionTwo`. Decode revision 1 binary examples built with the existing field tags using `protowire`. Include a `Hello` without the new fields, a result that decodes with watermark zero, an old `ScanHarnesses` command, and both old event arms. Check the old values and unset new fields. These are binary and receipt compatibility assertions. A real revision 1 handshake must be refused in the stack check.
- In `rules_test.go`, add `TestRevisionTwoLinkRulesAreWritten`. Use independent expected clauses under the corresponding source blocks for proof before history refusal, a durable immutable pending pair, replay while waiting for approval, and one acknowledgement per repeat or committed next event, with none for a gap. Check receipt persistence including `SetState`, interrupted and legacy receipt rules, report then mark then settlement, the shared filter on old and new report arms and logs, and loss recovery without fabricated advancement. Keep the old proof vectors, refusal-code test, and clock assertions. (logic-r1 02, architect-r1 04, logic-r1 05, threat-r1 02, critic-r1 3)
- In `envelopes_test.go`, add `TestFolderWireBudgets`. Check exactly the limits below against `link-v2-limits.json`. Use valid UTF-8 byte-exact strings at the typed-path, resolved-path, and remote limits and at one byte over, including a multibyte character at a boundary. Exercise an absent checkout with `clean=false` and an omitted remote distinct from no origin. Round-trip the two `FolderSeen` arms and inspect their reflection presence. Size full numbered event and full command samples with the event number, timestamp, attempt, expiry, and identifier included. Use synthetic padding in encoded fixtures to show exactly 48 and 48 kibibytes plus one, and 64 and 64 kibibytes plus one for the outer message. These fixtures prove byte accounting and encoding only. Runtime rejection, omission, and unnumbered failure belong to the program tests.
- Risky because this item defines ordering on the authenticated stream. Mutate either union tag, optional presence, one acknowledgement per repeat, no acknowledgement on a gap, the replay gate, the shared filter's coverage of old report arms, report before settlement, or the 49,152-byte budget. Show the corresponding assertion fail. Add a frozen comment without refreshing the digest and show `TestLinkIsFrozenAtItsRevision` fail. Restore the text and run the old proof examples.

**Link revision 2 fields.** These declarations use package `lgtm.link.v1`. In each existing message, retain all old fields and reservations.

| Message or union | New member | Number and type |
|---|---|---|
| `Hello` | `run_id` | 4, `string` |
| `Hello` | `first_event_number` | 5, `uint64` |
| `CommandResult` | `last_event_number` | 5, `uint64`. Keep number 4 and the name `message` reserved. |
| `Command.action` | `look_at_folder` | 13, `LookAtFolder` |
| `Event.report` | `folder_seen` | 12, `FolderSeen` |
| `LookAtFolder` | `operation_id`, `path` | 1 and 2, both `string` |
| `FolderSeen` | `operation_id`, `command_id` | 1 and 2, both `string` |
| `FolderSeen.result` | `facts`, `refusal` | 3, `FolderFacts`. 4, `FolderRefusal`. |
| `FolderFacts` | `resolved_path`, `name`, `main_branch` | 1, 2, and 3, all `string` |
| `FolderFacts` | `checked_out_branch`, `remote`, `first_commit` | 4, 5, and 6, all `optional string` |
| `FolderFacts` | `clean`, `remote_omitted` | 7 and 8, both `bool` |
| `FolderRefusal` | `FOLDER_REFUSAL_UNSPECIFIED`, `FOLDER_REFUSAL_NOT_A_FOLDER`, `FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY`, `FOLDER_REFUSAL_NOT_READABLE`, `FOLDER_REFUSAL_PATH_NOT_USABLE` | 0, 1, 2, 3, and 4, respectively. |

Declare the four new types in `folders.proto` using the existing Go package and import that source from `commands.proto` and `events.proto`. Tags 14 and above in `Command.action` and 13 and above in `Event.report` stay unallocated where available. The short tags leave room for more union arms and the Protocol Buffers tag encoding can use higher numbers. No reservation declares a later sprint's member.

**Required comment edits in the frozen files.**

- In `link.proto`, change the revision sentences on `Hello.link_revision` and `Welcome.link_revision` from 1 to 2, retaining the wording recognized by the freeze test. Keep the proof labels and signed bytes as in revision 1. Update the `Hello` introduction and the package value rules for the run, number ranges, and 64-kibibyte budget. In `Open`, add validation of the new fields after the existing lookup and revision check so a well-formed revision 1 hello still receives its verified Challenge and established refusal. Qualify the after-Welcome any-order sentence and the approval-waiting exception to reflect the run gate, including for `SetState`. Keep heartbeats and acknowledgements active while the command gate is closed. Clarify the `OpenResponse.event_ack` comment to say what it answers. Add the recovery rule beside the run gate.
- In `commands.proto`, replace the command life sentences about resends, when a final result is stored, and the first final outcome. Distinguish a saved early result from a settled outcome. Replace the sweep's reference to the wire expiry enum with the server-only outcome `timed_out`. Preserve the 23-hour offer limit, the 24-hour receipt margin, the hourly sweep at 24 hours, and the pre-existing `SetState` exception to time-based expiry. Clarify that removed, timed out, and expired mean different things. In `CommandResult` retain the matching current-stream and latest-dispatched-attempt rule, and add watermark receipt and settlement rules. Clarify in `CommandOutcome` that expiry is a client report and that unsupported denotes an unknown action. Normalize unknown and unspecified result outcomes to failed, as before.
- In `events.proto`, revise the ordering bullets so consuming a valid next number advances the same mark as applying a report. `Welcome.last_event_number` is the durable mark. Spell out recovery when retained events and the mark cannot be reconciled. Keep the current-stream, number-first, repeat, and gap precedence. Replace the `EventAck` introduction and following comment with one acknowledgement for each received repeat or committed next event on the current stream. Its `through` is cumulative even though the acknowledgements are sent individually. There is no acknowledgement for a gap, a bad number, an undecodable frame, a failed write, or an event from a stream that lost its right to write. A lost acknowledgement is covered by the durable mark in Welcome. The client hands over at most 32 events ahead of that mark or the latest acknowledgement on the stream.
- In `src/contracts/README.md`, name `folders.proto` and update both the frozen-revision and the spoken-link-revision sentences. Widen the freeze wording to the entire link directory. All changes to these source comments count toward the frozen hashes because `TestLinkIsFrozenAtItsRevision` hashes raw file bytes. The revision declarations themselves are comments. The schema additions in item 2 are the wire changes. The new revision is the only line with new hashes. `link-codes.json` and the proof examples keep their existing contents.

**Run gate and result fence.** A process chooses one version 7 UUID for `Hello.run_id` and captures `Hello.first_event_number` from the durable next counter before it appends. Its reconnects repeat that pair. The first number is in `[1, 2^63]`. The upper value fences an exhausted sequence without allowing an event above `2^63 - 1`. After proof and revision validation, Server Link saves the admitted pair under the machine lock, including while draining. It reads that pair after a server restart. The same pending or ready run must repeat its boundary. A different run can replace a pending pair at the same or a greater boundary, subject to the history checks below. The admission or event write that first reaches `first_event_number - 1` records the ready run and fires its callback once. (logic-r1 02)

Before readiness, heartbeats and retained replay continue without a command claim, send permit, or cursor change. A client waiting for approval may send only its retained pre-run events and heartbeats. Under the machine lock, the server applies valid Facts and consumes other eligible events without applying them while approval still waits. The client sends fresh reports and takes fresh work under the existing approval rules. `SetState` follows the run gate as well. (architect-r1 04)

`CommandResult.last_event_number` is the durable high-water number in `[0, 2^63 - 1]` when the client records the first result. Persist that value with the outcome before sending each result, including `SetState`, scans, restarts, declines, and interrupted results. Replay the same value on the current attempt. `SetState` keeps its standing-revision, approval, and expiry exceptions. Apply current standing by its revision even when replaying its result. Retain these standing receipts for the enrollment lifetime because the server can offer the command after the ordinary receipt cutoff. (logic-r1 05, architect-r1 05, critic-r1 4)

Bind started revision 2 receipts to the starting process. Durably seal the old process's final outbox high-water and bind its started receipts to that value before a new process may append. Seal and open the new run in one local write so a repeat startup preserves the old seal. A started or finished receipt positively identified as revision 1 uses zero. A valid revision 2 receipt can have zero with empty history. Invalid revision 2 metadata takes the local store-fault path. (threat-r1 02, logic-r1 05)

The server stages the first eligible result above its mark and excludes it from resends and sweeps. It settles a result already covered by the mark in its result write. In the event write that reaches the watermark, it applies or consumes the event, advances the mark, then settles. A valid folder report applied in that write wins over command settlement. Removal ends pending results as `removed`.

Done proves the action reached the completion its command defines. Failed reports that it could not finish the action as specified. Expired and unsupported are client declines. A wrong client clock can produce a decline for a command it previously ran, so that outcome alone gives no evidence that it never ran. Interrupted means the process died during the action and its effect is unknown. The server's timed-out outcome means it received no result in time and the effect is unknown. Removed means the owner removed the machine.

**Buffer loss.** A missing, corrupt or inconsistent local store stops the old enrollment's client with one owner-facing local recovery message. Before applying Welcome to the local outbox, compare its mark with durable history. If the mark is at or past the local next number, or is below the retained floor minus one, the histories are incompatible. The retained floor is the first retained sequence, or the next sequence if the retained queue is empty. A missing event already covered by Welcome needs no replay. A missing required event discovered later, such as a corrupt replay payload, also stops the client.

**Recovery on the Machines page.** UNREVIEWED MECHANISM (logic-r1 01, critic-r1 1, threat-r1 01). Keep the existing retryable `invalid_argument` and publish one fixed instruction through the authenticated Machines watch (architect-r1 03). Reuse its client record because the client can be rejected before Welcome and the owner can recover a retrying client without a new link message.

After proof, refuse a well-formed different run whose first number is at or below the mark or a staged watermark, or below the admitted pending boundary. Commit only the diagnostic under the machine lock and return `invalid_argument` after that commit. Server Link writes a durable per-machine flag and records a Machines change on the first transition. Machines projects it in `lgtm.v1.Client.event_history_needs_recovery`, including when offline. The website reads it on `WatchClients` and shows the fixed instruction on the affected machine, including a machine waiting for approval. At successful admission and event commits, clear it with a Machines change once the admitted run is ready and the mark covers the saved watermarks. Removal deletes the flag with the machine. Repeated refusals leave the mark, admission, existing stream, and command ledger intact. A changed boundary for the same run or malformed announcement is a generic `invalid_argument` without a diagnostic. Use the existing `event_gap` refusal for a numbered gap.

The fixed instruction for server-detected loss and the local store fault is “Restore this machine’s saved event history, or remove the machine and enroll it again.” Restoring the real prefix lets the gated run replay and settle. Removal ends commands as `removed` and fails open folder operations. The website call keeps its ordinary 25-second wait. Restoration of the server database to an earlier state and detection of an indistinguishable replacement history are outside wave 1.

**Folder facts and byte limits.** `LookAtFolder.operation_id` is the server's version 7 UUID for the durable operation. `FolderSeen` repeats it and names the specific `Command.id` as `command_id`. The authenticated stream identifies the machine. A report's result has exactly one arm. Absent arms and zero or unknown refusal values are invalid reports and are consumed without effect under the existing next-number policy. The client commits `FolderSeen` before recording its result. Done means the facts or fixed refusal report is buffered. Failed without a report covers the case where the client could not put a report in its buffer. Projects verifies the machine, operation and command before applying either arm once.

The typed path is absolute or begins with the home marker `~` on its own or followed by a slash. Inspect local facts read-only, without network access or repository-controlled program execution. Resolve the path to the absolute top of the Git working tree after following links. Its last path component is the name. The main branch is the locally known remote head, the checked-out branch, or `main`, in that order. The checked-out branch is absent in a detached checkout. The first commit is an opaque Git object identifier and is absent if unavailable. A present optional string is nonempty. Clean is false for a dirty tree. (threat-r1 03)

**Outgoing text.** Client Folders uses one filter before text reaches the buffer or client logs. It knows the client key and connection token in every form the client holds. Mask display text, including versions and log text. Omit an unsafe existing Facts workspace or identifier. An unsafe folder path, name, branch, or commit yields `FOLDER_REFUSAL_PATH_NOT_USABLE`. Leave an unsafe remote out and set `remote_omitted=true`. Use fixed failure text. For retained events, check only the required unacknowledged suffix before replay. Replay safe legacy payloads unchanged. If an unsafe legacy event is required, stop through the local fault path and preserve its bytes and number. Leave agent output and agent sign-in filtering to sprint 5. (architect-r1 08, critic-r1 3, threat-r1 04)

Client Folders strips the user information from a parsed origin before filtering and measuring it. Use the permitted formats below. Omit ambiguous, unsupported, over-limit or unsafe origins with `remote_omitted=true`. No origin gives an absent `remote` and `remote_omitted=false`. A present remote with the flag set is invalid. (threat-r1 04)

| Key in `link-v2-limits.json` | Exact value | Wire or runtime rule |
|---|---|---|
| `link_message_bytes` | 65536 | Maximum encoded message bytes including the outer Protocol Buffers request or response wrapper, excluding transport framing. The receive limit is applied after decompression. |
| `command_event_bytes` | 49152 | Maximum bytes of a complete encoded `Command` or `Event`. Budget the command as it will be sent, including the widest attempt, before storing. Budget the event at the number it will get, before allocating that number. An invalid inbound next event between the two budgets is consumed without applying its report. |
| `typed_path_bytes` | 4096 | A larger typed path is refused by the website call with `invalid_argument`. It never becomes a command. |
| `resolved_path_bytes` | 8192 | A longer path is refused with `FOLDER_REFUSAL_PATH_NOT_USABLE`. The derived name has no larger independent budget. |
| `remote_bytes` | 2048 | A longer sanitized remote is omitted and the flag is set. |
| `unacked_events` | 32 | The client hands over at most this many events while it waits for acknowledgement. |

Branch and commit text is subject to the total event budget plus the existing UTF-8 and zero-byte rules. If otherwise valid facts still do not fit the aggregate budget, emit the fixed path-not-usable refusal before allocating a number. If the refusal cannot be buffered, answer with a failed result using the unchanged actual watermark. A failed append consumes no event number.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A missed report is treated as proof of a command outcome or the restarted client receives work before its history is applied. | High | Document both fences and the loss rule. Hand the server, client, and stack teams exact runtime checks and mutations. |
| A field number collides, the receipt for an older client is lost, or a payload exceeds the number that the receiver accepts. | High | Compare descriptors, decode the old fixtures, and pin the fully encoded budgets before numbering. |

**Verify** Run `go test ./src/contracts -count=1`, `pnpm --filter @lgtm/api lint`, and the generation gate in item 4. The contract test package needs no device or database. The sprint's real PostgreSQL and paired-program suite exercises run, result, receipt, and folder behavior from this contract.

**WS-3: The website can discover and follow repositories**

The new service is a separate website contract. Its list and records follow the shared watch shape. Its two one-shot calls start or resume the operation identified by a request key in the existing header.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3 | The procedure shapes, the add reply's matching watch identities, and each condition, status, and display sentence are pinned. | `TestProjectsMatchItsDescription`, `TestEveryWatchCallHasTheSharedShape`, `TestRepositoryReplyCanJoinItsWatchRecord`, and `TestProjectRefusalRulesAreWritten`. |
| 4 | The additive discovery and Machines diagnostics decode old records. Both existing consumers build and both sample transports report revision 1 without the service. | `TestGetServerServiceDiscoveryIsAdditive`, `TestClientHistoryRecoveryIsAdditive`, `pnpm --filter @lgtm/mock-server typecheck`, `pnpm --filter @lgtm/frontend typecheck`, `pnpm --filter @lgtm/frontend build`, and the two discovery probes below. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Add the repository service, records, and contract tests. | New `src/contracts/proto/lgtm/v1/projects.proto` follows `clients.proto: ClientService, WatchClientsResponse` and `common.proto: FeedPosition`. `src/contracts/watchshape_test.go` → `TestEveryWatchCallHasTheSharedShape`. |
| 4 | Add service discovery and the Machines diagnostic, and verify old consumers and clean generation. | `src/contracts/proto/lgtm/v1/session.proto` → `GetServerResponse`. `src/contracts/proto/lgtm/v1/clients.proto` → `Client`, `WatchClientsResponse`. `src/contracts/package.json` → `exports`, `scripts.check`. `src/contracts/buf.gen.yaml` → `plugins`. `src/mock-server/src/engine.ts` → `createEngine`, `CONTRACT`. `src/frontend/src/server/api.ts` → `openServer`, `CONTRACT`. |

**Item 3. Repository calls and watch records.** Create `src/contracts/proto/lgtm/v1/projects.proto`, `src/contracts/projects_test.go`, and `src/contracts/testdata/golden/projects.txt`. Change `src/contracts/watchshape_test.go` so it explicitly requires `WatchRepositories` in the methods checked, without a new exception. Generate `src/contracts/gen/lgtm/v1/projects_pb.{js,d.ts}`, `src/contracts/gen/go/lgtm/v1/projects.pb.go`, and `src/contracts/gen/go/lgtm/v1/lgtmv1connect/projects.connect.go`. The source work is independent of items 1 and 2. Begin item 4 after item 3. This item implements foundation D1, D5, rules 1 to 8 as they apply to the service boundary, and newer checks R1 to R8 and R12 to R14.

- Write `TestProjectsMatchItsDescription` in `projects_test.go` against the new golden file. Require the exact package names, input and output descriptors, field presence, tags, and direction below. Add a source assertion in `TestProjectFolderCallsRequireRequestKeys` for the authenticated service, the shared header on both unary methods, and the difference between a first attempt, a repeat, and a conflicting key. Both new tests fail on the missing file or descriptors.
- Write `TestRepositoryReplyCanJoinItsWatchRecord` in `projects_test.go`. Construct one add reply with a repository and its attached folder, and two watch frames carrying the same repository in the snapshot and the saved arm. Marshal and unmarshal through the real generated descriptors, using reflective lookup for the red test. Assert that all three representations have the same repository identifier, and the returned folder matches one nested folder by identifier, machine, and resolved path. Include a second folder on a different machine, a dirty inspection with no checked-out branch, and absent, present, and withheld origins as distinct cases.
- Write `TestProjectRefusalRulesAreWritten` in `projects_test.go`. Use an independent expected table for every condition, code, and sentence below. Require each row in the corresponding `projects.proto` source comment. The table is written by Contract and read by Projects for its handler assertions and by Website for display behavior. (logic-r1 07, architect-r1 09, critic-r1 5, threat-r1 06)
- Risky because the header governs retries and the watch has an ordering contract. Mutate the position placement, streaming direction, either header rule, the reply identity, and one refusal code. Show the relevant assertions fail.

**Project service layout.** Use package `lgtm.v1`, the website Go package, and `common.proto`. Keep the website independent of link types.

| Procedure or message | Fields and numbers, or input and output |
|---|---|
| `ProjectService.WatchRepositories` | `WatchRepositoriesRequest` → stream `WatchRepositoriesResponse` |
| `ProjectService.InspectFolder` | `InspectFolderRequest` → `InspectFolderResponse` |
| `ProjectService.AddRepository` | `AddRepositoryRequest` → `AddRepositoryResponse` |
| `WatchRepositoriesRequest` | Empty. |
| `WatchRepositoriesResponse` | `RepositoryList all=1`, `Repository saved=2`, `string removed_id=3` in `oneof change`. `FeedPosition position=4` outside the union. |
| `RepositoryList` | `repeated Repository repositories=1` |
| `Repository` | `string id=1`, `string name=2`, `string main_branch=3`, `optional string remote=4`, `optional string first_commit=5`, `repeated RepositoryFolder folders=6`, `bool remote_omitted=7` |
| `RepositoryFolder` | `string id=1`, `string client_id=2`, `string resolved_path=3` |
| `InspectFolderRequest` | `string client_id=1`, `string path=2` |
| `InspectFolderResponse` | `FolderFacts facts=1` |
| `lgtm.v1.FolderFacts` | `string resolved_path=1`, `string name=2`, `string main_branch=3`, `optional string checked_out_branch=4`, `optional string remote=5`, `optional string first_commit=6`, `bool clean=7`, `bool remote_omitted=8` |
| `AddRepositoryRequest` | `string client_id=1`, `string path=2`, `optional string repository_id=3` |
| `AddRepositoryResponse` | `Repository repository=1`, `RepositoryFolder folder=2` |

The service is authenticated, including the entire lifetime of the watch. The watch sends one full initial snapshot, then a complete saved repository with all its current folders, or the removed repository's identifier. A folder change on a retained repository is a saved repository. The snapshot replaces the previous collection on reconnect. Positions follow `FeedPosition`, which requires a first position, a rising position on each change, and reconnection after `aborted`. Repository and folder identifiers are server-generated version 7 UUIDs. `client_id` refers to `Client.id`. The repo and folder identifiers in a successful add are the stored identities also used on the watch. The reply's folder is in that reply's repository. The server persists both records or neither, even if the call has stopped waiting. `InspectFolderResponse.facts` holds the client facts for that inspection with the same semantics as link `FolderFacts`. Inspect stores the operation and result but creates no repository or repository folder.

The unary calls require one nonempty `Lgtm-Request-Key` under the store's text rules and 256-byte limit. `store.WriteOnce` identifies the caller and key. Its digest includes the procedure, machine, exact typed path, and add's optional repository choice. A present empty repository identifier is invalid. One write stores the operation and its command under the machine lock. The command expires after 20 seconds and the receipt reply holds only the operation identifier. Wait at most 25 seconds outside the write. A fresh request to a disconnected machine is unavailable. An accepted replay finds the operation before checking availability. A timeout ends only the caller's wait.

**Replay and settlement.** Store keeps the original caller-and-key receipt as the single reservation while the operation is retrievable. Projects asks Store to hold it in the initial write. The pruner skips held receipts. Projects retires the terminal operation through Store under the same caller-and-key serialization, so a replay admitted before retirement reads the saved operation or result and a conflicting request cannot seize its key. Leave the reservation held for open operations and through at least the ordinary one-day receipt interval after settlement. Remove the separate fallback identity. The Server Projects plan sets the Store interface with the Store owner. (logic-r1 04, architect-r1 02)

The report and settlement callbacks may each transition only an open operation under the machine lock. A late report for a terminal operation is consumed with an acknowledgement and no Projects effect. A valid report applied before settlement in the same write wins. Removal uses the same lock. (logic-r1 03)

**Folder ownership and origin identity.** Resolve the folder before checking ownership under the machine lock. Return the existing row for an omitted or matching repository choice, even if its origin changed. Refuse a different explicit target. For a new folder association, omit the target to create a named repository, or require that the folder match the explicit target. If both remotes are available, Projects compares their normalized forms. When at least one origin is known absent, require equal present first commits. A withheld origin gives no evidence of absence. Distinct present remotes fail even with the same first commit. (logic-r1 06, architect-r1 07, critic-r1 2)

Client Folders writes credential-free remote text. Projects reads it and normalizes for comparison. Accept a parsed network URL (Uniform Resource Locator) with scheme `http` or `https` (Hypertext Transfer Protocol), `ssh` (Secure Shell), or `git`, and Git's `[user@]host:path` spelling for Secure Shell. Omit local paths, unparseable or ambiguous forms, query strings, fragments, and paths with escape sequences or dot segments. Normalize scheme and host case and drop the scheme's default port (80, 443, 22, or 9418). Preserve nondefault port, transport, path case, the `.git` suffix and trailing slash. Preserve the distinction between a relative Secure Shell path and one starting with a slash. An absolute Secure Shell shorthand and its URL form compare equal. A host case or explicit default-port difference compares equal. A path case change, an added `.git` suffix, or a different transport compares unequal. (logic-r1 06, architect-r1 07, critic-r1 2, threat-r1 04)

**Fixed refusals for the website.** Each response uses one Connect code and one display sentence. The client never supplies that sentence. The `FolderRefusal` values ride on the link only. Reuse existing infrastructure errors and session handling.

| Condition | Connect code | Sentence |
|---|---|---|
| Session missing or expired at entry | `unauthenticated` | Sign in to continue. |
| Session ends during a watch | `unauthenticated` | Sign in again to reopen this watch. |
| Watch falls behind | `aborted` | The watch fell behind and must be reopened. |
| Malformed JSON body on a protected call | `invalid_argument` | The request body is invalid. |
| Machine is disconnected when starting a new operation | `unavailable` | The machine is not connected. |
| Invalid machine or repository identifier, or invalid typed path | `invalid_argument` | The machine, repository, or folder path is invalid. |
| Invalid or missing request key | `invalid_argument` | The request key is invalid. |
| Same key for a different request | `already_exists` | That request key was already used for a different request. |
| Unknown machine | `not_found` | The machine was not found. |
| Unknown repository | `not_found` | The repository was not found. |
| `FOLDER_REFUSAL_NOT_A_FOLDER` | `failed_precondition` | The path is not a folder. |
| `FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY` | `failed_precondition` | The folder is not a Git repository. |
| `FOLDER_REFUSAL_NOT_READABLE` | `failed_precondition` | The folder cannot be read. |
| `FOLDER_REFUSAL_PATH_NOT_USABLE` | `failed_precondition` | The folder path cannot be used. |
| Folder does not match the selected repository | `failed_precondition` | The folder does not belong to the selected repository. |
| Folder is already attached to a different repository | `already_exists` | The folder is already saved for another repository. |
| Call wait ends or done or failed arrives without a report | `deadline_exceeded` | The machine did not answer the folder request. |
| Expired or unsupported without a report | `deadline_exceeded` | The machine did not take the folder command. |
| Interrupted or timed out without a report | `deadline_exceeded` | The outcome of the folder request is unknown. |
| The machine is removed before the operation finishes | `not_found` | The machine was removed. |

**Item 4. Additive website fields and generated consumer checks.** Change `src/contracts/proto/lgtm/v1/{session,clients}.proto` and `src/contracts/README.md`. Create `src/contracts/service_discovery_test.go`. Regenerate `src/contracts/gen/lgtm/v1/{session,clients}_pb.{js,d.ts}`, `src/contracts/gen/go/lgtm/v1/{session,clients}.pb.go`, and `src/contracts/gen/go/lgtm/v1/lgtmv1connect/{session,clients}.connect.go` if the generator changes those handlers. This item needs items 1 to 3 for the final whole-contract checks. It implements foundation D1, rule 2, checks C7 and C11, and newer check R13. (architect-r1 03, logic-r1 01, critic-r1 1, threat-r1 01)

- Add `repeated string services=6` to `GetServerResponse`. An entry is the case-sensitive fully qualified Protocol Buffers service name, without a slash or method suffix. Presence of `lgtm.v1.ProjectService` on a real server enables repository mode. An empty or omitted list advertises no opt-in service. Unknown entries are ignored by clients. `sample=true` still selects the sample behavior. The website contract stays at revision 1. Add the service and discovery rule to the readme.
- Write `TestGetServerServiceDiscoveryIsAdditive` first in `service_discovery_test.go`. Use registry reflection to pin the first five fields and repeated string `services=6`. Decode old binary and JSON replies with both sample flags and expect an empty service list. Round-trip the real advertisement using the service descriptor's full name and decode it with a revision 1 descriptor to check the old fields. Mutate a tag, the cardinality, and the advertised name to their wrong values and require the tests to fail. (logic-r1 07)
- Write `TestClientHistoryRecoveryIsAdditive` in the same file. Pin existing tags 1 to 18 and `bool event_history_needs_recovery=19`. Decode an old `Client` with the new descriptor and require false. Round-trip true in both Machines snapshot and saved records. Decode the new record with the old descriptor and check its existing values. Mutate the tag or default expectation and require a failure. Server Link writes the flag through the Machines projection, and Website reads the generated `eventHistoryNeedsRecovery` field only for the Machines instruction above. The field has no sign-in or command authority. (architect-r1 03, logic-r1 01, critic-r1 1, threat-r1 01)
- Open each sample transport in a fresh browser context. Observe `GetServer` revision 1, `sample=true`, and an empty service list. Sign in through that transport. Expect the Machines snapshot with the recovery flag false and the sample workspace. Capture procedure calls at the in-page router and on the network for the mock, and confirm no repository watch in either. Start the mock with `pnpm --filter @lgtm/mock-server start`. (critic-r1 7)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A watch joins to the wrong repository or keeps a removed folder. | Medium | Pin record identity, full replacement, feed position, and the existing shared shape. |
| The service list hides the real feature or enables it against sample data. | High | Pin the new field and full name. Build the existing consumers and check discovery on both sample transports. |

**Verify** Run `go test ./src/contracts -count=1`. After committing the generated files on the builder's branch, run `pnpm --filter @lgtm/api check`. The check lints, regenerates, checks the Go contract, and fails if generation changes or adds a tracked or untracked output in `gen/`. Use the installed workspace dependencies and Go 1.26 or newer. Run the three consumer commands and the two browser probes above. No physical device is needed for these two modes. The stack plan exercises the third discovery mode with the real server and runs the check with a real machine. No new module, install dependency, or generator option is needed.

**Handoffs across the server and browser.**

- Server Link publishes and lands the reviewed Go boundary before Projects builds against it. It returns the identifier from enqueue after the machine lock. It invokes registered callbacks in the writes that settle a command, apply a report, admit a stream, mark a run ready, or remove a machine. Removal invokes its own callback even when there is no pending command. Link writes the interface. Projects registers and reads it. Keep the Machines `CommandQueue.Enqueue` error-only surface with a wrapper, and compile the Link, Projects and Machines owners and fixtures at the handoff. (architect-r1 06)
- Server Projects registers the generated procedures in dispatch and in the Cross-Origin Resource Sharing (CORS) table `webProcedures`. Use `ProjectServiceWatchRepositoriesProcedure`, `ProjectServiceInspectFolderProcedure`, and `ProjectServiceAddRepositoryProcedure` from `lgtmv1connect`. Contract generates these names and the server router reads them. Keep the origin and session gates. Reuse `redactMalformedJSON` for the two unary calls and the `isMalformedJSON` check on the watch's first request, following `redactingClientService.WatchClients`. (architect-r1 01, threat-r1 05, threat-r1 06)

**Post-build checks.**

The orchestrator owns new in-process tests in `src/linktest/`. Use `servertest.Start`, `clienttest.Join` and `clienttest.Run`. Run `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^TestSprintFour' -count=1`. Use that prefix for each named case below and require an explicit pass for each, with no skips. Stack owns deployed checks in `deploy/`. (architect-r1 10)

| Case and owner | Scenario and expected result |
|---|---|
| `TestSprintFourHistoryRecovery`. Orchestrator, Link and Website. | Stage a result at 12 with mark 10 and restart the server. A valid-looking new run starting at 11 is refused after proof. The diagnostic appears once in the Machines feed and persists through another server restart. Repeated attempts leave the admission, mark, staged result and gate intact. Restore the real 11 and 12, and expect one settlement and a cleared diagnostic. In a separate attempt remove the machine and expect a removed command, failed open operation and removed machine record. For a Welcome below the retained floor or at or above local next, or a damaged required payload, expect one local recovery instruction and no fabricated advancement. Confirm the same instruction in the signed-in Machines page for server-detected loss. (architect-r1 03, logic-r1 01, critic-r1 1, threat-r1 01) |
| `TestSprintFourPendingRun`. Orchestrator and Link. | At mark 5, admit a run starting at 10 and restart the server mid-drain. Refuse the same run starting at 6 and a different run below 10. Allow replacement at 10. Hold the three send permits during replay. Expect one cumulative acknowledgement per repeat or committed next event, none for a gap, and no more than 32 events outstanding. Expect one readiness callback and no claim, permit or cursor change before readiness. Release a permit and observe ordered commands. A caught-up run becomes ready on admission. (logic-r1 02, logic-r1 P3) |
| `TestSprintFourWaitingReplay`. Orchestrator, Link and Folders. | Start a waiting stream at first event 3 with two older events, one valid Facts and one report that waiting status blocks. Require application of Facts and consumption without effect of the other event. Commit approval on the same stream. Both acknowledgements precede `SetState` and fresh reports wait for approval. A report from a replaced, removed or other machine creates no Projects records. Only an eligible current stream can consume a number and receive its acknowledgement. (architect-r1 04, threat P2) |
| `TestSprintFourReceiptReplay`. Orchestrator and Folders. | Drop a `SetState` reply, buffer another event and restart. Replay the original watermark with the latest attempt, including past ordinary receipt expiry. Crash on either side of sealing an old run after it buffers `FolderSeen` and before it finishes its receipt. Restart twice and append an unrelated event in a later run. Expect the original watermark and one applied report. A damaged revision 2 receipt faults. Started and finished revision 1 receipts use zero. (logic-r1 05, architect-r1 05, critic-r1 4, threat-r1 02, logic-r1 P2) |
| `TestSprintFourFolderSettlement`. Orchestrator, Link and Projects. | Interrupt the database write after applying a folder report and before committing the mark and result. Expect no save or acknowledgement. Replay and expect one save, one mark advance, and one settlement in the same commit. Run settlement before the first report, and removal before and after the report. A late eligible report after settlement advances and acknowledges only. A removed stream cannot write. The watch emits the saved or removed repository as appropriate. (logic-r1 03, logic-r1 P1) |
| Projects and Store. | After the ordinary receipt cutoff run the pruner with an open operation. Retry the same request, then change its path and try the key from another keyed procedure. Expect the original operation, two key conflicts, and one command. Race terminal replay against retirement under the key lock. Add a second checkout with equivalent origins, then try distinct origins with an equal first commit. Repeat with known absence, withholding, and a saved folder whose origin changed. Verify the identity and refusal rules above. (logic-r1 04 and 06, architect-r1 02 and 07, critic-r1 2) |
| Projects and Website. | Through the real router, run each origin preflight with a listed and an unlisted origin, including the request-key header on both unary preflights. Only the listed origin can make the signed-in calls. Run the refusal table at the handlers. Malformed bodies for both unary calls and the watch produce fixed text and create no operation. A lagged watch ends with `aborted` and starts with a replacement snapshot on reopening. Expiry stops an open watch. Superseded inspection replies and records from the old session or server are discarded. (architect-r1 01 and 09, logic-r1 07, critic-r1 5, threat-r1 05 and 06, logic-r1 P4) |
| Folders and Projects. | Inspect a checkout with an inert side-effect sentinel. Expect the sentinel untouched and no network activity. Plant synthetic credentials in a remote query, path, branch, old Facts workspace, inventory display text and logs. Expect remote and workspace omission, a fixed folder refusal, and display masking. Check both logs, the client buffer, server records and browser replies for absence. Safe legacy events replay unchanged. A recognized secret in a required legacy payload stops before handoff with the original bytes and counter preserved. (threat-r1 03 and 04, architect-r1 08, critic-r1 3) |
| Stack and Website. | Run the deployed revision 1 refusal and upgrade in place, including an unacknowledged old event and a completed receipt. Observe one replay and no rerun. Verify the pinned server certificate path off loopback and private client state permissions. Use one signed-in website build on in-page sample, network mock, older real and revision 2 real servers. Only the new registered server enters repository mode. Return an add reply before its watch update and then reverse the order. Both must join the same saved identities. Run R14 on the deployed stack. (architect-r1 10, critic-r1 7, logic-r1 P4, threat P5) |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is assignment bullets 1 to 4 and its settlements, the foundation design's applicable decisions D1, D2, D4, D5 and D7, rules 1 to 11, and checks C3, C7, C10 to C12. It also includes the 2026-11-27 design's sprint 4 sections and checks R1 to R14, its open question 2, issue 1674 point 4, the contract handoffs in the two round-three review closing lists, and Phase 4 of the build loop. In this table, R1 to R14 are the newer sprint checks. Foundation rules are called rules 1 to 11. The remaining foundation decisions and checks concern existing runtime or later sprints.

| Source item | Workstream | Status |
|---|---|---|
| Assignment 1. Link additions and both revision declarations. | WS-2 | Item 2 fixes the additive layout and declarations. |
| Assignment 2. Freeze and old baseline. | WS-1, WS-2 | Items 1 and 2 prove and apply the append. |
| Assignment 3. Repository service and advertisement. | WS-3 | Items 3 and 4 specify the service and the additive field. |
| Assignment 4. Both generators and sample compatibility. | WS-2, WS-3 | Item 2, item 3, and item 4 generate the contract and verify its consumers. |
| Settlements. Lost buffer, field numbers, fixed facts and refusals, comments, both closing review lists. | WS-1, WS-2, WS-3 | Specified in the work items and the decision log. |
| Foundation D1 and D2. Separate parts and existing Go module. | WS-2, WS-3 | The contract is the shared boundary. |
| Foundation D4 and D5. PostgreSQL and one ordered change feed. | WS-3 | The watch uses the shared feed. The real-database test is a sprint gate. |
| Foundation D7 and C10. Machine key proof. | WS-2 | The proof labels, bytes and refusal precedence remain covered by existing tests. |
| Foundation rules 1, 2, 3 and 4. Named part ownership, contract first, and writes through the store. | WS-1, WS-2, WS-3 | Contract review precedes runtime work. Server implementation and migrations go to the dependent plans. |
| Foundation rules 5, 6 and 7. Shared watch, stored commands and durable state. | WS-2, WS-3 | Both durable fences and the operation and watch contracts are specified. |
| Foundation rules 8 and 9. One error sentence and no secret-bearing text sent. | WS-2, WS-3 | Fixed refusal values, safe display sentences, and comments on filtering. |
| Foundation rules 10 and 11. Real fixtures and carried credit. | WS-2, WS-3 | Runtime fixtures are a sprint gate. The plan calls for no copied logic. |
| Foundation C3 and C7. Sessions and Machines. | WS-2, WS-3 | The waiting replay proof exercises approval and the additive Machines projection preserves old records. |
| Foundation C11 and C12. Ordered replay and feed locking. | WS-2, WS-3 | The wire rules and Machines diagnostic have contract checks. The paired suite verifies ordered recovery without fabricated advancement. |
| Newer design, identifiers and Link interface, clocks, run gate, byte budget and outgoing filter. | WS-2, WS-3 | Item 2 fixes the wire rules. Link publishes its reviewed Go boundary before Projects builds against it. |
| Newer design, Link revision 2, parts, folder operation and mapping, machine removal and website in sprint 4. | WS-1, WS-2, WS-3 | The layouts and refusal semantics are pinned. The behavior is assigned in the result. |
| R1, R2 and R3. Inspection facts, persisted resolved path and alias convergence. | WS-2, WS-3 | Facts and identifiers are pinned. Projects, Folders, and stack own behavioral checks. |
| R4 and R5. Attach by identity and replay a keyed operation. | WS-3 | The request, saved identities, and retry promise are pinned. Projects and stack own behavioral checks. |
| R6 and R7. Report wins across restart, and late commands expire. | WS-2, WS-3 | Settlement, expiry, and outcomes are pinned. Server, client, and stack own the runtime checks. |
| R8 and R9. Commit order and admission gate. | WS-2 | The run gate and absence of a claim or permit while waiting are pinned. Link and stack own the pool and order probes. |
| R10 and R11. Durable early result and revision 1 refusal with disk upgrade. | WS-1, WS-2 | Wire layout and freeze are pinned. Link, Folders, and stack own the restart and installation proofs. |
| R12. Removal drops folders and ends operations. | WS-2, WS-3 | Removed outcome and watch fields are pinned. Projects and stack own runtime tests. |
| R13 and R14. Three discovery modes and the real add flow. | WS-3 | The two existing sample modes are contract regression checks. The website and stack own the real flow. |
| Open question 2. Lost buffer. | WS-2 | Item 2 and the decision log specify the preservation and removal rule. |
| Issue 1674 point 4. Acknowledgements. | WS-2 | One acknowledgement per eligible event receipt is stated and tested in comments. |
| Round-three logic and fit closing lists. | WS-2, WS-3 | Decisions give each line a contract action or an explicit owner. |
| Build loop Phase 4. Assertions first, frozen tests, isolated mutations, one writer. | WS-1, WS-2, WS-3 | Every item names its files, tests, observation, and mutations. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md` → The product parts meet at `src/contracts/`. | The build edits only the contract. The result gives requests to the other teams. |
| `AGENTS.md` → A change to what the parts exchange starts in `src/contracts/proto/`. | The plan defines the new sources before generated artifacts or dependent code. |
| `AGENTS.md` → Old code lives under `packages/`, `design/mock/`, and `plugins/`. | The contract uses the current generation and shared watch patterns. |
| `AGENTS.md` → Credit copied code and record issues and features. | No copied code is proposed. Issue 1674 point 4 is addressed in the plan and any later issue tracking is handed to the orchestrator within the bounded assignment. |
| `AGENTS.md` → Finished work reaches the base through a pull request. | The orchestrator owns integration and the freeze pin for the reviewed commit. |
| `AGENTS.md` → Nothing is pushed to Herdr Synapse. | The planning assignment makes no remote change. |
| `AGENTS.md` → Writing style and privacy. | The plan uses relative paths, dated decisions, and a separate check of its new files. |
| Epic team boundary and the bounded task. | Build writes stay under `src/contracts/`. Plan writes stay in this folder and the named result. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | contracts | `contract/contracts` | low | It adds tests and documentation to an existing history gate. | No | None. |
| 2 | contracts | `contract/contracts` | medium | The link schema, descriptions, bytes, and matched freeze need one atomic update using the existing generator. | Yes | 1. |
| 3 | contracts | `contract/contracts` | medium | It adds a service with three calls through an existing pattern and shared shape. | Yes | None. Serialize generators that share a checkout. |
| 4 | contracts | `contract/contracts` | low | It adds two scalar field declarations and tests their compatibility through existing watches and discovery. | Yes | 1, 2, 3. |

Load. OpenAI is the only vendor assigned to build and review this plan. Use different sessions for test writing, building, and review, and record that the check is same-vendor. Budget four test writers, four builders, one four-seat plan review round, and three focused code-review seats, for up to 15 future launches. The remaining allowance is unavailable to the planner. The orchestrator checks the current allowance before launching each batch and schedules a later batch if the vendor cannot take it. No concurrent test writer and builder edit the same unit. Tests become the builder's frozen input after the writer demonstrates red and the reviewer checks the assertions.

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Round 1 | Four reviewers filed 30 findings before grouping. | [The round 1 register](attack/round-1-findings.md) records all outcomes, the stuck statement, and the sections for the next review. The orchestrator owns that review. |

</details>
