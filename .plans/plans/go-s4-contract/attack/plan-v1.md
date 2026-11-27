---
topic: go-s4-contract
status: attack
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# The contract for a repository on a machine

## Overview

Sprint 4 needs a reviewed agreement for how the website asks a machine about a repository and how the machine reports what it found. The website gains calls to inspect and add a folder and to follow saved repositories. A real server advertises these calls so the same website build can still open the in-page sample, the mock server, and an older real server.

A result can reach the server while the folder report is still in the client's saved buffer. Link revision 2 gives the server the event number it must reach before settling that result. A restarted client also tells the server where the new process's events begin. This lets the server apply older events before it sends work to that process. When the buffer needed to prove an outcome is lost, the machine stops and the owner can recover its saved state or remove and enroll it again.

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
- Write `TestLinkFreezeAppendAgainstPriorBaseline` in the same file, using the scratch-module pattern of `TestFreezeCheckFailsClosed`. Commit a form with revision 1 alone, append revision 2 with a leading comma, parse the working file as valid JavaScript Object Notation (JSON), and require exit 0 of the real `scripts/check-go.sh` with revision 1 as its base. Add the revision 2 commit in the fixture. Require exit 0 at its own commit and on a later unchanged commit against that fixed revision 2 base. Require exit 1 and the existing frozen-record diagnostic for a revision 1 line edit, a trailing comma placed on the old line, a moved old line, and a revision 2 hash rewrite against the reviewed revision 2 base. Keep the cases for a missing or pre-freeze base.
- The public input is `LGTM_LINK_BASE`. The file is `src/contracts/testdata/link-frozen.json`. Line 2 remains byte-for-byte identical to commit `fcfa96c3`. Line 3 starts with two spaces and a comma before the new object. The closing bracket follows on line 4. Hash the complete bytes of the four source files for the new entry. The raw hash check ignores generated code, the website's Protocol Buffers files, and the code table outside the frozen directory. Its separate readme check expects one current freeze declaration. Expand the stale test comment about three files to describe the directory scan.
- Risky because the baseline orders revisions. Break the documented append rule to make the documentation assertion fail. In a temporary copy of the script, drop the old-line comparison to make the two history rejection assertions fail. Record the failing assertions and restore the copied script.

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
- In `rules_test.go`, add `TestRevisionTwoLinkRulesAreWritten`. Read the named source blocks and check the rule clauses in the following paragraphs, including the order of report application, watermark advancement, and settlement. Assert that result replay retains the saved watermark and the loss rule forbids fabricated advancement. Check that the repeat and consumed-event paths each send one acknowledgement and that an event rejected for a gap has no acknowledgement. Assert the temporary wait and removal behavior for a pending result. Use independently written expected clauses tied to their specific comment blocks. Preserve the revision 1 proof vectors, refusal-code test, and clock assertions. Update the event number phrase tests only if the formatting of that unchanged rule requires it.
- In `envelopes_test.go`, add `TestFolderWireBudgets`. Check exactly the limits below against `link-v2-limits.json`. Use valid UTF-8 byte-exact strings at the typed-path, resolved-path, and remote limits and at one byte over, including a multibyte character at a boundary. Exercise an absent checkout with `clean=false` and an omitted remote distinct from no origin. Round-trip the two `FolderSeen` arms and inspect their reflection presence. Size full numbered event and full command samples with the event number, timestamp, attempt, expiry, and identifier included. Use synthetic padding in encoded fixtures to show exactly 48 and 48 kibibytes plus one, and 64 and 64 kibibytes plus one for the outer message. These fixtures prove byte accounting and encoding only. Runtime rejection, omission, and unnumbered failure belong to the program tests.
- Risky because this item defines ordering and applies to the authenticated stream. Show that changing either new union tag, changing a field from optional to plain, removing the no-ack-on-gap or one-ack-per-repeat clause, lowering the window, settling before applying the awaited event, allowing the lost result to settle as done, or using 65,536 in place of 49,152 causes the relevant assertion to fail. Independently add a comment to a frozen file without refreshing the record and show `TestLinkIsFrozenAtItsRevision` fail. Run the old proof examples after undoing the mutations.

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

**Run gate and result fence.** A process chooses one version 7 UUID for `Hello.run_id` and captures `Hello.first_event_number` from the durable next counter before appending events of that run. A reconnect sends the same pair, even if the durable next counter has risen. The first number is in `[1, 2^63]`. The upper value represents an exhausted event sequence plus one and can only fence a drain. The server compares the durable mark to `first_event_number - 1` so the value it compares fits the signed event range. Event numbers themselves remain bounded by `2^63 - 1`. The server validates the pair only for the supported revision. It records a different ready run once after all lower numbers are applied or consumed, in the admission or event write that reaches the fence. A reconnect for the recorded run must repeat its original first number and has no run-change callback. Until ready the stream may replay old events and exchange heartbeats. It takes no command send permit, claims no row, and advances no command cursor. The readiness gate and command claiming remain ordered under the machine lock. Use the ordinary approval rules when the gate opens.

`CommandResult.last_event_number` carries the durable outbox high-water number when the client records the result. It is in `[0, 2^63 - 1]`. Zero means no event or a receipt written by revision 1. A revision 2 interrupted receipt uses the last event durably assigned by the process that began the command, even if more processes have since started. The client preserves that boundary for a later replay, including across a second restart. An interrupted receipt from revision 1 has watermark zero under the legacy rule. A matching result above the server mark is stored with the command and takes it out of resend and sweep selection. Its first accepted value is stable. A later copy cannot advance or lower its watermark. Settle a result already covered by the mark in its own write. When a later event reaches a stored result's watermark, apply or consume the event and advance the mark before firing the settlement in the same write. A folder report applied in that write wins over the outcome of its command. Removal ends pending results with the existing server-only outcome `removed`.

Done proves the action reached the completion its command defines. Failed reports that it could not finish the action as specified. Expired and unsupported are client declines. A wrong client clock can produce a decline for a command it previously ran, so that outcome alone gives no evidence that it never ran. Interrupted means the process died during the action and its effect is unknown. The server's timed-out outcome means it received no result in time and the effect is unknown. Removed means the owner removed the machine.

**Buffer loss.** A missing, corrupt or inconsistent local store stops the old enrollment's client with one owner-facing local recovery message. Before applying Welcome to the local outbox, compare its mark with durable history. If the mark is at or past the local next number, or is below the retained floor minus one, the histories are incompatible. The retained floor is the first retained sequence, or the next sequence if the retained queue is empty. A missing event already covered by Welcome needs no replay. A missing required event discovered later, such as a corrupt replay payload, also stops the client.

After proof, the server refuses admission with the existing `invalid_argument` if a different run's first number is at or below the committed mark or the watermark of a saved waiting result. Those numbers belong to the machine's past and the new run cannot issue them. Changing the first number of the recorded run is also invalid. A refused admission changes neither the mark nor the recorded run. Use the existing `event_gap` refusal for an event received above the next number. Use the existing retry behavior for server-side invalid arguments and the local store-fault path when the client itself detects unrecoverable loss.

During recovery, keep the original numbers, mark, saved watermark, and run gate. The server keeps its durable waiting result and its send fence. A matching saved state can replay the real prefix and release both. If no state can supply that prefix, the owner removes the machine and enrolls it under a new identity. The removal write settles its remaining commands as `removed` and fails its open folder operations. A waiting result never becomes done or timed out merely because the event cannot be found. A website call keeps its own 25-second wait and returns its ordinary timeout while waiting. A store replaced so completely that it presents indistinguishable valid history cannot be diagnosed by this wire. The operator must preserve history under an old enrollment.

**Folder facts and byte limits.** `LookAtFolder.operation_id` is the server's version 7 UUID for the durable operation. `FolderSeen` repeats it and names the specific `Command.id` as `command_id`. The authenticated stream identifies the machine. A report's result has exactly one arm. Absent arms and zero or unknown refusal values are invalid reports and are consumed without effect under the existing next-number policy. The client commits `FolderSeen` before recording its result. Done means the facts or fixed refusal report is buffered. Failed without a report covers the case where the client could not put a report in its buffer. Projects verifies the machine, operation and command before applying either arm once.

The typed path is an absolute path or starts with the home marker `~` on its own or followed by a slash. The resolved path is the absolute top of the Git working tree after following links. The name is its last path component. The main branch is the locally known remote head, the checked-out branch, or `main`, in that order. The checked-out branch is absent for a detached checkout. The first commit is an opaque Git object identifier and is absent if unavailable. A present optional string is nonempty. Clean is false for a dirty tree. Strip remote credentials before measuring, comparing or reporting `origin`. No origin is an absent `remote` and `remote_omitted=false`. A remote that exceeds the limit or cannot pass the client filter is left out with `remote_omitted=true`. A present remote with that flag is invalid. Screen the other outgoing facts before buffering. An unsafe path, name, branch, or commit produces `FOLDER_REFUSAL_PATH_NOT_USABLE`. Do not send raw failures or substitute masked key values.

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
| 3 | All three procedures and their exact request and response types exist. The watch passes the shared check with a snapshot and record-level changes. The add reply names the same repository and folder represented on the watch. | `TestProjectsMatchItsDescription`, `TestEveryWatchCallHasTheSharedShape`, and `TestRepositoryReplyCanJoinItsWatchRecord`. |
| 4 | The generated discovery message keeps old replies readable. The existing TypeScript consumers build and both sample transports return revision 1 without the service. | `TestGetServerServiceDiscoveryIsAdditive`, `pnpm --filter @lgtm/mock-server typecheck`, `pnpm --filter @lgtm/frontend typecheck`, `pnpm --filter @lgtm/frontend build`, and the two discovery probes below. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Add the repository service, records, and contract tests. | New `src/contracts/proto/lgtm/v1/projects.proto` follows `clients.proto: ClientService, WatchClientsResponse` and `common.proto: FeedPosition`. `src/contracts/watchshape_test.go` → `TestEveryWatchCallHasTheSharedShape`. |
| 4 | Add service discovery and verify old consumers and clean generation. | `src/contracts/proto/lgtm/v1/session.proto` → `GetServerResponse`. `src/contracts/package.json` → `exports`, `scripts.check`. `src/contracts/buf.gen.yaml` → `plugins`. `src/mock-server/src/engine.ts` → `createEngine`, `CONTRACT`. `src/frontend/src/server/api.ts` → `openServer`, `CONTRACT`. |

**Item 3. Repository calls and watch records.** Create `src/contracts/proto/lgtm/v1/projects.proto`, `src/contracts/projects_test.go`, and `src/contracts/testdata/golden/projects.txt`. Change `src/contracts/watchshape_test.go` so it explicitly requires `WatchRepositories` in the methods checked, without a new exception. Generate `src/contracts/gen/lgtm/v1/projects_pb.{js,d.ts}`, `src/contracts/gen/go/lgtm/v1/projects.pb.go`, and `src/contracts/gen/go/lgtm/v1/lgtmv1connect/projects.connect.go`. The source work is independent of items 1 and 2. Begin item 4 after item 3. This item implements foundation D1, D5, rules 1 to 8 as they apply to the service boundary, and newer checks R1 to R8 and R12 to R14.

- Write `TestProjectsMatchItsDescription` in `projects_test.go` against the new golden file. Require the exact package names, input and output descriptors, field presence, tags, and direction below. Add a source assertion in `TestProjectFolderCallsRequireRequestKeys` for the authenticated service, the shared header on both unary methods, and the difference between a first attempt, a repeat, and a conflicting key. Both new tests fail on the missing file or descriptors.
- Write `TestRepositoryReplyCanJoinItsWatchRecord` in `projects_test.go`. Construct one add reply with a repository and its attached folder, and two watch frames carrying the same repository in the snapshot and the saved arm. Marshal and unmarshal through the real generated descriptors, using reflective lookup for the red test. Assert that all three representations have the same repository identifier, and the returned folder matches one nested folder by identifier, machine, and resolved path. Include a second folder on a different machine, a dirty inspection with no checked-out branch, and absent, present, and withheld origins as distinct cases.
- Risky because the header governs retries and the watch has an ordering contract. Show that moving `position` into `change`, sending a non-streamed watch, removing the header rule from either unary call, or replacing the repository or folder object in the reply with a success flag fails the named assertions. Runtime proof that retries and snapshots behave as documented belongs to the other teams.

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

The two unary calls require one valid, nonempty `Lgtm-Request-Key` header using the store's current text rules and 256-byte limit. There is no request-key field in a message. Use `store.WriteOnce` with the authenticated caller and procedure. The request fingerprint includes the exact machine, typed path, and, for add, the repository selection. A present but empty repository identifier is invalid. In one short write, store the operation and the command, with command expiry of 20 seconds. The receipt holds the operation identifier. Wait for at most 25 seconds outside the write. A retry reads the same operation after a timeout, disconnect, or server restart. A first request for a disconnected machine is unavailable. A repeat of an accepted intent reads the existing operation before making a new availability decision. The operation retains its caller, key and request identity as long as it is retained, so pruning a generic receipt while the operation is still open cannot create another command. A same-key request with a changed intent receives the store's existing refusal. A timeout ends only the caller's wait. The first valid report finishes the operation, and command settlement changes only an open operation.

With an absent `repository_id`, the server names a new repository from the found folder or returns the existing one if that machine already has the resolved folder saved. With a present identifier it attaches to that repository. If both sanitized remotes are available, their normal forms must match, regardless of first commit. If at least one origin is known absent, equal present first commits are required. A withheld remote is distinct from an absent origin, so a pair with no known absence and an omitted remote cannot prove identity. A folder already stored for the same repository returns its stored identity. A known folder owned by a different requested repository is refused. An accepted add names the resolved path and never uses a typed alias as the saved identity.

**Fixed refusals for the website.** Each response uses one Connect code and one display sentence. The client never supplies that sentence. The `FolderRefusal` values ride on the link only. Reuse existing infrastructure errors and session handling.

| Condition | Connect code | Sentence |
|---|---|---|
| Session missing or expired | `unauthenticated` | Use the existing session guard's sentence. |
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

**Item 4. Additive service discovery and generated consumer checks.** Change `src/contracts/proto/lgtm/v1/session.proto` and `src/contracts/README.md`. Create `src/contracts/service_discovery_test.go`. Regenerate `src/contracts/gen/lgtm/v1/session_pb.{js,d.ts}`, `src/contracts/gen/go/lgtm/v1/session.pb.go`, and `src/contracts/gen/go/lgtm/v1/lgtmv1connect/session.connect.go` if the generator emits a difference in the latter. Keep the public package export pattern and the existing Go module. This item needs items 1 to 3 for the final whole-contract and consumer checks. It implements foundation D1 and rule 2, the additive website compatibility rule, and newer check R13.

- Add `repeated string services=6` to `GetServerResponse`. An entry is the case-sensitive fully qualified Protocol Buffers service name, without a slash or method suffix. Presence of `lgtm.v1.ProjectService` on a real server enables repository mode. An empty or omitted list advertises no opt-in service. Unknown entries are ignored by clients. `sample=true` still selects the sample behavior. The website contract stays at revision 1. Add the service and discovery rule to the readme.
- Write `TestGetServerServiceDiscoveryIsAdditive` first in `service_discovery_test.go`. Look up the service and response through the registry. Assert that old fields 1 to 5 keep their names, types, and scalar cardinality. Assert that tag 6 is the repeated string `services`. Decode old binary and JSON replies, including sample and real flags, and verify that no service is discovered. Round-trip a real reply containing the fully qualified service name. Decode the new binary with a revision 1 descriptor and confirm the five old fields. Use the existing generated discovery type through the registry. The absent field causes a red descriptor assertion on the starting checkout.
- Show that renumbering a discovery field, replacing tag 6 with one scalar, or advertising the short service name instead of the full name in the fixture fails the relevant test or contract-comment assertion. The field changes neither sign-in nor its keys or ordering. The server and website plans prove the actual advertisement and mode selection.
- Confirm the sample modes with the built website by opening the in-page sample and the existing mock server separately. For each, observe a successful `GetServer` of revision 1, `sample=true`, and no repository service advertised. Observe the first Machines snapshot and the existing sample workspace in each mode. Use the mock's existing `pnpm --filter @lgtm/mock-server start` command and the website's existing dev or built entry point. The browser probe may use a loopback server. In both modes check that no request targets `/lgtm.v1.ProjectService/WatchRepositories`. The contract change edits no consumer file. The real-server leg follows once Projects and the website are built.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A watch joins to the wrong repository or keeps a removed folder. | Medium | Pin record identity, full replacement, feed position, and the existing shared shape. |
| The service list hides the real feature or enables it against sample data. | High | Pin the new field and full name. Build the existing consumers and check discovery on both sample transports. |

**Verify** Run `go test ./src/contracts -count=1`. After committing the generated files on the builder's branch, run `pnpm --filter @lgtm/api check`. The check lints, regenerates, checks the Go contract, and fails if generation changes or adds a tracked or untracked output in `gen/`. Use the installed workspace dependencies and Go 1.26 or newer. Run the three consumer commands and the two browser probes above. No physical device is needed for these two modes. The stack plan exercises the third discovery mode with the real server and runs the check with a real machine. No new module, install dependency, or generator option is needed.

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
| Foundation C3 and C7. Sessions and Machines. | WS-3 | Existing discovery, session guard, and Machines consumers are regression checks. |
| Foundation C11 and C12. Ordered replay and feed locking. | WS-2, WS-3 | Wire compatibility and shared shape have contract checks. Behavior is delegated to real-program tests. |
| Newer design, identifiers and Link interface point 4, clocks, run gate, byte budget and outgoing filter. | WS-2 | Item 2 fixes the fields, loss rule, and comments. Points 1 to 3 of the Go interface belong to the Link plan. |
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
| 4 | contracts | `contract/contracts` | low | It adds one field and runs the existing consumers and generation checks. | Yes | 1, 2, 3. |

Load. OpenAI is the only vendor assigned to build and review this plan. Use different sessions for test writing, building, and review, and record that the check is same-vendor. Budget four test writers, four builders, one four-seat plan review round, and three focused code-review seats, for up to 15 future launches. The remaining allowance is unavailable to the planner. The orchestrator checks the current allowance before launching each batch and schedules a later batch if the vendor cannot take it. No concurrent test writer and builder edit the same unit. Tests become the builder's frozen input after the writer demonstrates red and the reviewer checks the assertions.

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| One Tier M round | Pending. | The bounded planning assignment ends at the handoff to the orchestrator. The orchestrator runs the round and records its findings before releasing the names for downstream planning. |

</details>
