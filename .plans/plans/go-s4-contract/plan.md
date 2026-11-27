---
topic: go-s4-contract
status: done
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# The contract for a repository on a machine

## What the sprint 4 contract enables

Sprint 4 defines how the website inspects and adds folders on a machine and follows saved repositories. Only a real server with those calls advertises them. The same website build can still open the in-page sample, the mock server, and an older real server.

A result can reach the server before its folder report. Link revision 2 gives the server the event number it needs before settling and the boundary a restarted client must reach before taking work. A refused admission behind the server's mark sets a recovery flag for the Machines page. The owner will decide what the page shows. A client that detects missing required local history stops. Recovery uses the original history or a new enrollment after removal. (architect-r1 03, logic-r2 03, architect-r2 01 and 02)

The contract is reviewed before the teams build against it. Starting an agent and creating a repository on disk belong to sprint 5.

## Key decisions

The machine returns folder facts or a fixed refusal. The server saves an early result across restarts. Recovery replays the original events in order, and removal ends an unrecoverable command.

The website contract stays at revision 1. Both folder calls use the existing request key to find their saved operation after a lost reply.

Full log → [decisions.md](decisions.md)

## Workstreams

### The new link can be frozen against revision 1

The new record line carries its own separator. The history check proves that each frozen line keeps its bytes and position.

### The machine can report a folder in order

The contract tests pin the message layouts and byte limits and decode old examples. Tests with both programs prove acknowledgement, restart and result ordering.

### The website can discover and follow repositories

A repository watch starts with a full list and then sends saved or removed records by identifier. The browser matches the add reply to that watch. The build also exercises both sample transports.

## Rollout order

1. Hand the reviewed names to the five dependent planners.
2. Build and review the contract against the revision 1 freeze.
3. Use the reviewed landed contract commit as the revision 2 baseline.
4. Build the dependents and deploy the revision 2 server and installer. Verify an in-place revision 1 client upgrade and one website build across the real server and both sample modes.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

Workstream identifiers use the prefix WS. Protocol Buffers identifiers and machine-readable error codes below are literal names. A universally unique identifier (UUID) is spelled in its canonical lowercase form. Text uses Unicode Transformation Format, 8-bit (UTF-8). A kibibyte is 1,024 bytes. A Secure Hash Algorithm 256-bit (SHA-256) digest here is lowercase hexadecimal.

**WS-1: The new link can be frozen against revision 1**

Document the append layout and prove the baseline transition with the history script.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | The append rule is documented. The real gate accepts the append against revision 1 and rejects edits to either protected revision line under the appropriate baseline. | `TestLinkFreezeAppendIsDocumented`, `TestLinkFreezeAppendAgainstPriorBaseline`, and `TestFreezeCheckFailsClosed`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Document and test how the next freeze is appended. | `src/contracts/README.md` → Rules. `src/contracts/freeze_test.go` → `TestLinkIsFrozenAtItsRevision`, `TestFreezeCheckFailsClosed`. `scripts/check-go.sh` → `link_record`, `link_base`. |

**Item 1. Append rule and history tests.** Change `src/contracts/README.md` and `src/contracts/freeze_test.go`. Items 2 and 4 wait for this item. Item 3 can run alongside it in another checkout. This implements foundation rule 2 and the revision 2 freeze.

- Write `TestLinkFreezeAppendIsDocumented` first in `freeze_test.go`. Require the readme to state that frozen link source hashes include comments, old record lines are kept byte for byte at their original line numbers, and the separator belongs on the new line. Require the note that the review uses the previous reviewed commit and that the baseline advances after review. It fails on the current readme.
- Use `TestLinkFreezeAppendAgainstPriorBaseline` to characterize the existing comparator, following the scratch-module pattern of `TestFreezeCheckFailsClosed`. Require valid JavaScript Object Notation (JSON) and exit 0 for the leading-comma append against revision 1, and for the revision 2 commit and a later unchanged commit against the fixed revision 2 base. Require exit 1 with the frozen-record diagnostic for a revision 1 line edit, a trailing comma on the old line, a moved old line, and a revision 2 hash rewrite against the revision 2 base. Keep the missing and pre-freeze base cases. (critic-r1 6)
- In `src/contracts/testdata/link-frozen.json`, preserve line 2 from `fcfa96c3`. Start line 3 with two spaces and a comma, then the new object. Close on line 4. The four frozen link sources include comments in their hashes. Update the test comment about the source count to describe the directory scan. The separate readme test expects one current declaration.
- Risky because the baseline orders revisions. The readme assertion supplies the initial red test. In a temporary copy of the script, bypass the old-line comparison and show that all four named rejection assertions fail while the positive controls pass. Restore the comparison and run the real gate against the built append in item 2. (critic-r1 6)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A formatter appends the separator to line 2 or a caller supplies a mutable baseline. | High | Prove the append layout and reject historical edits. Give dependent gates the reviewed immutable commit identifier. |

**Verify** Run `go test ./src/contracts -run 'TestLinkFreezeAppend|TestFreezeCheckFailsClosed|TestLinkIsFrozenAtItsRevision' -count=1`. No device or database is needed for the scratch-repository cases.

**Freeze pins for rollout steps 2 and 3.** The orchestrator runs `LGTM_LINK_BASE=fcfa96c3 bash scripts/check-go.sh` during build review. After landing the reviewed contract, pin that commit and rerun the gate. Later gates use the immutable landed commit.

**WS-2: The machine can report a folder in order**

Publish the additive wire changes, both revision declarations, generated files and matching freeze in one item. Keep the signed proof labels.

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

**Item 2. Revision 2 fields, folders, and comments.** Edit the Protocol Buffers paths in the task row, the readme, `src/contracts/testdata/link-frozen.json`, `src/contracts/testdata/golden/{open,commands,events,folders}.txt`, `src/contracts/{open,envelopes,rules}_test.go` and new `src/contracts/testdata/link-v2-limits.json`. Regenerate `src/contracts/gen/lgtm/link/v1/{link,commands,events,folders}_pb.{js,d.ts}`, `src/contracts/gen/go/lgtm/link/v1/{link,commands,events,folders}.pb.go` and `src/contracts/gen/go/lgtm/link/v1/linkv1connect/link.connect.go`. Braces enumerate exact paths. Item 1 comes first. Serialize generators sharing an output directory. This covers foundation rules 1, 2 and 6 to 9, C11, sprint checks R1, R2 and R6 to R11, and issue 1674 point 4.

**Tests written first.** Use reflection by string so missing symbols fail assertions. Freeze the tests and expected files before implementation.

- Extend `TestEveryLinkNameIsDescribed` and `TestEveryLinkRuleIsWritten` to cover the `folders` description and source. Update the three existing descriptions. Add `TestFoldersMatchItsDescription` in `envelopes_test.go` to pin fields, presence, union arms, enum values and counts through `matchDescription`.
- In `envelopes_test.go`, add `TestRevisionOnePayloadsDecodeUnderRevisionTwo`. Use `protowire` to decode an old Hello, result, `ScanHarnesses` and both old event arms. Expect the old values, unset new fields and a zero watermark. Stack proves the actual revision refusal.
- In `rules_test.go`, add `TestRevisionTwoLinkRulesAreWritten`. Check independent clauses for proof before history refusal, immutable latest admission and supersession through a restart, replay before approval, and one acknowledgement per repeat or committed next event with none for a gap. Check stable watermarks for standing, interrupted and legacy results, report then mark then settlement, and no fabricated advancement. Pin report filtering and omission of account-relative Secure Shell paths. Keep proof vectors, refusal codes and clock assertions. (logic-r1 02, architect-r1 04, logic-r1 05, threat-r1 02, logic-r2 01 and 05, architect-r2 04)
- Add `TestFolderWireBudgets` in `envelopes_test.go`. Match the exact values in `link-v2-limits.json`. Check each text limit and one byte over, including multibyte text. Round-trip both `FolderSeen` arms, a dirty detached checkout and an omitted versus absent origin. Include the event number, timestamp, attempt, expiry and identifier in the encoded command and event fixtures. Pad them and the outer message to their 48- and 64-kibibyte limits and one byte over. Program tests prove runtime rejection and omission.
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

Declare the new types in `folders.proto` in the existing Go package and import it in commands and events. The next available union tags start at command 14 and event 13. Leave their assignment to later revisions.

**Required comment edits in the frozen files.**

- In `link.proto`, set both revision comments to 2 using the freeze test's wording. In `Hello` and the package values, specify the run, ranges and outer budget. In `Open`, preserve proof and revision refusal precedence. Revise the after-Welcome and approval rules for the run gate, recovery, heartbeats and acknowledgements. Clarify `OpenResponse.event_ack`.
- In `commands.proto`, distinguish staged and settled results in the command lifecycle and `CommandResult`. Keep current-stream and latest-attempt matching, the 23-hour offer grace, 24-hour receipt margin and hourly sweep at 24 hours, with the `SetState` exception. Use server-only `timed_out` in the sweep comment. In `CommandOutcome`, distinguish expiry and unsupported, and retain failed for unknown or unspecified values.
- In `events.proto`, preserve number-first handling on the current stream and define `Welcome.last_event_number` as the durable replay mark. Apply or consume each eligible next event. Send one cumulative `EventAck` per repeat or committed next event. Gaps, bad numbers, undecodable frames, failed writes and obsolete streams receive none. The client may hand over up to 32 events ahead of Welcome or the latest acknowledgement.
- In the readme, name `folders.proto`, cover the link directory and advance the two revision sentences. Source comments count toward the new hash. Preserve `link-codes.json` and the proof examples.

**Run gate and result fence.** The client chooses a version 7 UUID per process and captures its first number before appending. Reconnects repeat that pair. The first number is in `[1, 2^63]` and events stop at `2^63 - 1`. Check run history after proof and revision validation. Only the latest admitted run can reconnect with its saved boundary, even while draining. Refuse superseded runs before testing the boundary, including after restart. An unseen run must start above the mark and staged watermarks and at or above any pending boundary. At a tie between unseen runs, the later successful admission takes precedence. A run becomes ready once when the mark reaches `first_event_number - 1`. (logic-r1 02, logic-r2 01)

Before readiness, heartbeats and replay use no command claim, send permit or cursor change. A client waiting for approval may send its retained pre-run events and heartbeats. The server applies valid Facts and consumes other eligible events without effects while approval waits. Fresh reports and work follow existing approval rules. Send the replay acknowledgements before the gated `SetState`. (architect-r1 04)

`CommandResult.last_event_number` is the durable high-water in `[0, 2^63 - 1]` for the first result, including `SetState`, scans, restarts and declines. Replay it on the current attempt. `SetState` keeps its revision, approval and expiry exceptions. An interrupted result uses the last durable number of the process that started the command, even across multiple restarts. Positively identified revision 1 receipts use zero. A revision 2 receipt with empty history may record zero. Missing or damaged revision 2 metadata is a local fault. (logic-r1 05, architect-r1 05, critic-r1 4, threat-r1 02, logic-r2 P1)

The server stages the first eligible result above its mark and excludes it from resends and sweeps. It settles a result already covered by the mark in its result write. In the event write that reaches the watermark, it applies or consumes the event, advances the mark, then settles. A valid folder report applied in that write wins over command settlement. Removal ends pending results as `removed`.

Done proves the command's specified completion. Failed reports it could not finish. Expired and unsupported are client declines, and a wrong clock can cause a decline after execution. Interrupted and timed out leave the effect unknown. Removal means the owner removed the machine.

**Buffer loss.** Use the existing retryable `invalid_argument` for refused announcements and `event_gap` for a received gap. Refusal leaves the incumbent and command ledger intact. A Welcome mark at or above local next or below the retained floor minus one is incompatible. The floor is the first retained number, or local next for an empty queue. Required missing or corrupt history stops replay without a fabricated advance. Restore the real prefix or remove the machine and enroll again. Server database rollback and indistinguishable client history are outside wave 1. (logic-r1 01, threat-r1 01, architect-r2 02, logic-r2 06)

**Folder facts and byte limits.** `LookAtFolder.operation_id` is the server's version 7 UUID for the durable operation. `FolderSeen` repeats it and names the specific `Command.id` as `command_id`. The authenticated stream identifies the machine. A report's result has exactly one arm. Absent arms and zero or unknown refusal values are invalid reports and are consumed without effect under the existing next-number policy. The client commits `FolderSeen` before recording its result. Done means the facts or fixed refusal report is buffered. Failed without a report covers the case where the client could not put a report in its buffer. Projects verifies the machine, operation and command before applying either arm once.

The typed path is absolute or begins with the home marker `~` on its own or followed by a slash. Inspect local facts read-only, without network access or repository-controlled program execution. Resolve the path to the absolute top of the Git working tree after following links. Its last path component is the name. The main branch is the locally known remote head, the checked-out branch, or `main`, in that order. The checked-out branch is absent in a detached checkout. The first commit is an opaque Git object identifier and is absent if unavailable. A present optional string is nonempty. Clean is false for a dirty tree. (threat-r1 03)

**Outgoing text.** Screen report text for the client key and connection token in every form the client holds. Mask display text. Omit an unsafe Facts workspace or identifier. An unsafe folder path, name, branch or commit yields `FOLDER_REFUSAL_PATH_NOT_USABLE`. Replay safe legacy payloads unchanged. A required unsafe legacy event stops replay with its bytes and number preserved. (architect-r1 08, critic-r1 3, threat-r1 04)

Strip user information from permitted origins before measuring or sending them. Ambiguous, unsupported, over-limit or unsafe origins have an absent `remote` and `remote_omitted=true`. An unconfigured origin has an absent `remote` and `remote_omitted=false`. A present remote with the flag set is invalid. (threat-r1 04, logic-r2 05, architect-r2 04)

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

**Verify** Run `go test ./src/contracts -count=1`, `pnpm --filter @lgtm/api lint` and item 4's generation gate. No device or database is needed for contract tests. Use real PostgreSQL for the paired-program checks below.

**WS-3: The website can discover and follow repositories**

Extend the website with the shared watch shape and two calls that resume a folder operation by key.

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

- In `projects_test.go`, write `TestProjectsMatchItsDescription` against the new golden file. Assert package, descriptors, tags, presence and direction. Write `TestProjectFolderCallsRequireRequestKeys` for authenticated entry, both keyed methods and fresh, repeat and conflict behavior. Expect red on the missing descriptors or source.
- In the same file, write `TestRepositoryReplyCanJoinItsWatchRecord`. Round-trip an add reply, a snapshot and a saved frame through real descriptors, using reflection before they exist. Expect matching repository identifiers and one folder match by identifier, machine and path. Include a second machine, a dirty detached checkout and absent, present and withheld origins.
- Write `TestProjectRefusalRulesAreWritten` in `projects_test.go`. Check each condition, code and sentence against independent expectations, including missing-target precedence over existing-folder conflict. Require the corresponding `projects.proto` clauses. Contract writes the clauses, and Projects and Website read them. (logic-r1 07, architect-r1 09, critic-r1 5, threat-r1 06, logic-r2 07)
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

Require a session through the life of the watch. Send a replacement snapshot first, then full saved repositories with their folders or removed identifiers. Use the shared `FeedPosition` rules, including reopening after `aborted`. The server generates version 7 repository and folder UUIDs. The add reply contains the saved identities and its folder belongs to its repository. Persist both or neither after a valid report, even after the call ends. `client_id` refers to `Client.id`. Inspect stores only the operation and result, with the link's `FolderFacts` semantics.

The unary calls require one nonempty `Lgtm-Request-Key` under the store's text rules and 256-byte limit. Request identity includes the caller, procedure, machine, exact typed path and optional repository choice. A present empty target is invalid. Store the operation and its command together. The command expires after 20 seconds and the caller waits at most 25 seconds outside the write. A fresh request to a disconnected machine is unavailable.

**Replay and settlement.** An accepted retry reads the original operation before deciding availability. A conflicting use of the caller's key is refused throughout replay. A timeout ends the wait, and the first valid report or terminal outcome ends the open operation. A valid report applied before settlement in the same write wins. An eligible late report still advances the mark and is acknowledged. (logic-r1 03 and 04, architect-r1 02, logic-r2 02, architect-r2 03)

**Folder ownership and origin identity.** At attachment, an unknown explicit target returns `not_found` for both a saved and an unsaved resolved folder. Return the existing folder for an omitted or matching target, even if its origin changed. Refuse a different known target with `already_exists`. For a new folder, an omitted target creates a named repository. An explicit target needs matching present origins, or equal present first commits when at least one origin is known absent. Withholding gives no evidence of absence. (logic-r1 06, architect-r1 07, critic-r1 2, logic-r2 07)

Client Folders writes credential-free remote text. Projects reads and normalizes it. Accept network Uniform Resource Locators (URLs) for Hypertext Transfer Protocol and its secure form (`http`, `https`), Secure Shell (`ssh`) and `git`. Accept independently absolute Secure Shell shorthand and compare it like its URL form. Send account-relative paths as absent with `remote_omitted=true`, including home markers after a URL slash. Projects never compares them as origins. Omit local paths, ambiguous forms, queries, fragments, escaped paths and dot segments. Fold scheme and host case and ignore default ports (80, 443, 22 and 9418). Keep nondefault ports, transport, path case, suffix and trailing slash significant. (logic-r1 06, architect-r1 07, critic-r1 2, threat-r1 04, logic-r2 05, architect-r2 04)

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
| Unknown explicit repository at attachment, even if the folder is already saved elsewhere | `not_found` | The repository was not found. |
| `FOLDER_REFUSAL_NOT_A_FOLDER` | `failed_precondition` | The path is not a folder. |
| `FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY` | `failed_precondition` | The folder is not a Git repository. |
| `FOLDER_REFUSAL_NOT_READABLE` | `failed_precondition` | The folder cannot be read. |
| `FOLDER_REFUSAL_PATH_NOT_USABLE` | `failed_precondition` | The folder path cannot be used. |
| Folder does not match the selected repository | `failed_precondition` | The folder does not belong to the selected repository. |
| Folder is already attached to another repository and the explicit target exists | `already_exists` | The folder is already saved for another repository. |
| Call wait ends or done or failed arrives without a report | `deadline_exceeded` | The machine did not answer the folder request. |
| Expired or unsupported without a report | `deadline_exceeded` | The machine did not take the folder command. |
| Interrupted or timed out without a report | `deadline_exceeded` | The outcome of the folder request is unknown. |
| The machine is removed before the operation finishes | `not_found` | The machine was removed. |

**Item 4. Additive website fields and generated consumer checks.** Change `src/contracts/proto/lgtm/v1/{session,clients}.proto` and `src/contracts/README.md`. Create `src/contracts/service_discovery_test.go`. Regenerate `src/contracts/gen/lgtm/v1/{session,clients}_pb.{js,d.ts}`, `src/contracts/gen/go/lgtm/v1/{session,clients}.pb.go`, and `src/contracts/gen/go/lgtm/v1/lgtmv1connect/{session,clients}.connect.go` if the generator changes those handlers. This item needs items 1 to 3 for the final whole-contract checks. It implements foundation D1, rule 2, checks C7 and C11, and newer check R13. (architect-r1 03, logic-r1 01, critic-r1 1, threat-r1 01)

- Add `repeated string services=6` to `GetServerResponse`. An entry is the case-sensitive fully qualified Protocol Buffers service name, without a slash or method suffix. Presence of `lgtm.v1.ProjectService` on a real server enables repository mode. An empty or omitted list advertises no opt-in service. Unknown entries are ignored by clients. `sample=true` still selects the sample behavior. The website contract stays at revision 1. Add the service and discovery rule to the readme.
- Write `TestGetServerServiceDiscoveryIsAdditive` first in `service_discovery_test.go`. Use registry reflection to pin the first five fields and repeated string `services=6`. Decode old binary and JSON replies with both sample flags and expect an empty service list. Round-trip the real advertisement using the service descriptor's full name and decode it with a revision 1 descriptor to check the old fields. Mutate a tag, the cardinality, and the advertised name to their wrong values and require the tests to fail. (logic-r1 07)
- Write `TestClientHistoryRecoveryIsAdditive` in the same file. Pin tags 1 to 18 and `bool event_history_needs_recovery=19`. Decode an old `Client` and expect false. Round-trip true in snapshot and saved records. Decode the new record with the old descriptor and check existing values. Mutate the tag or default expectation and require failure. Server Link writes the boolean through Machines, and Website reads `eventHistoryNeedsRecovery` from `WatchClients`. The field carries no copy or commands. (architect-r1 03, logic-r1 01, critic-r1 1, threat-r1 01, architect-r2 01, logic-r2 03)
- Open each sample transport in a fresh browser context. Observe `GetServer` revision 1, `sample=true`, and an empty service list. Sign in through that transport. Expect the Machines snapshot with the recovery flag false and the sample workspace. Capture procedure calls at the in-page router and on the network for the mock, and confirm no repository watch in either. Start the mock with `pnpm --filter @lgtm/mock-server start`. (critic-r1 7)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A watch joins to the wrong repository or keeps a removed folder. | Medium | Pin record identity, full replacement, feed position, and the existing shared shape. |
| The service list hides the real feature or enables it against sample data. | High | Pin the new field and full name. Build the existing consumers and check discovery on both sample transports. |

**Verify** Run `go test ./src/contracts -count=1` and `pnpm --filter @lgtm/api check` on the builder's committed candidate with Go 1.26 or newer. The generation gate rejects tracked or untracked output changes. Run the three consumer commands and two sample probes above without a physical device. Stack verifies real discovery with a real machine. Reuse the existing modules, dependencies and generator options.

**Requests to the dependent plans.**

| Plan | Required behavior and proof |
|---|---|
| `go-s4-server-link` | Link provides the reviewed identifier-returning enqueue and transactional report, settlement, admission, ready-run and removal boundary before Projects consumes it. Keep the error-only Machines queue and compile all three owners. **UNREVIEWED MECHANISM** (architect-r2 01, logic-r2 03). Set the durable Machines flag when admission is refused because its history is behind the committed mark. Clear it only when a later admission of that machine succeeds and becomes ready, or remove it with the machine. Traffic from a stream admitted before the flag was set never clears it. This uses later readiness alone because normal admission already checks history. In `TestSprintFourHistoryRecovery`, also force an uncertain diagnostic commit. The candidate gets the existing database error, the incumbent remains current, and the flag and feed change commit together or neither commits. Provide a real server fixture that can fail after the report callback and before the mark and result commit. A failure after commit before acknowledgement does not prove rollback. (architect-r1 06, logic-r2 06, architect-r2 06) |
| `go-s4-server-projects` | Register the three generated procedures for dispatch and browser preflight behind the existing origin and session gates. Keep fixed malformed-body replies for unary and streaming calls. With Store, protect the caller-wide key while replay is promised, including while open and through one day after settlement. Pause just after receipt lookup and race retirement before the first operation read. An admitted replay gets the original result. If retirement wins, use fresh-request rules. Race two removals of the last folders on separate machines and expect one orphan removal. Race a new attachment with last-folder removal and expect either a surviving attachment or the fixed missing-target refusal. Test unknown explicit targets with both saved and unsaved paths against the same refusal clause. (architect-r1 01, threat-r1 05 and 06, architect-r2 03, logic-r2 02, 04 and 07) |
| `go-s4-client-folders` | Coordinate with Client Link and the client application. Persist first result watermarks, including standing replies beyond ordinary expiry and an interrupted process's boundary before a new process appends. Expose a fixture interruption after durable `FolderSeen` append and before receipt completion. Use the shared filter for buffer and logs, including structured startup paths. Start the actual client with a synthetic known credential in its workspace and check the complete process log and reports. A terminal local history fault gives one recovery instruction and reaches the executable as a stop. Stack verifies supervisor behavior. (architect-r2 02, 05 and 06, logic-r2 P1) |
| `go-s4-web-repositories` | Read the flag from authenticated Machines snapshots and updates for both waiting and other machines. Show what the owner decides. A stale action reply must not replace watch state. Reopen the watch and verify both machine states, unrelated updates, a later successful ready admission, and removal. Keep the real and sample discovery rules. (architect-r2 01, logic-r2 03 and P4) |
| `go-s4-stack-checks` | Install with intact enrollment and damaged history. Observe one local recovery instruction and no automatic restart across the configured restart interval on both shipped supervisors. An ordinary requested restart still works. Run the deployed revision upgrade, certificate, discovery and repository checks. The orchestrator owns the in-process cases and captures durable state at both exact interruption points supplied by Link and Folders. (architect-r2 02 and 06) |

**Post-build checks.**

The orchestrator owns new in-process tests in `src/linktest/`. Use `servertest.Start`, `clienttest.Join` and `clienttest.Run`. Run `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^TestSprintFour' -count=1`. Use that prefix for each named case below and require an explicit pass for each, with no skips. Stack owns deployed checks in `deploy/`. (architect-r1 10)

| Case and owner | Scenario and expected result |
|---|---|
| `TestSprintFourHistoryRecovery`. Orchestrator and Link. | With mark 10, refuse a new run at 9 after proof. Let the old stream advance. Expect true on the live watch and after restart. Repeat with and without a staged result. A recovered client restarts on real history and clears the flag only when its new admission is ready. Remove it instead and expect a removed record, removed commands and failed open operation. Test incompatible Welcome marks and corrupt required payloads separately. Expect a local stop without advancement. (architect-r2 01, logic-r2 03) |
| `TestSprintFourPendingRun`. Orchestrator and Link. | Start a ready run, then admit a different run at 10 with mark 5. Restart mid-drain. Refuse the former ready run and a changed pending pair. Allow an unseen replacement at 10. Repeat with two ready runs at first number 6 and try the first again. Expect refusal and no extra ready callback. Hold three permits during replay. Expect no early claim or cursor change, one cumulative acknowledgement per eligible event, none for a gap and at most 32 outstanding. Release a permit for ordered delivery. (logic-r2 01, logic-r1 02 and P3) |
| `TestSprintFourWaitingReplay`. Orchestrator, Link and Folders. | At a waiting Welcome, drain retained events. Apply Facts and consume the blocked report. Approve on the same stream, including between final replay delivery and acknowledgement. Both acknowledgements precede `SetState`. Fresh reports wait for approval. Replaced, removed and other-machine streams cannot complete the target operation. (architect-r1 04, logic-r2 P2) |
| `TestSprintFourReceiptReplay`. Orchestrator and Folders. | Drop a `SetState` reply, append and restart. Beyond ordinary expiry expect the original watermark on a new attempt. Interrupt after report append but before receipt finish. Restart twice and append unrelated events. Expect one applied report and the first process's watermark. Repeat across startup with an empty old history. Replay explicit zero for a valid revision 2 receipt or recognized revision 1 receipt. Missing or damaged revision 2 metadata stops without a result. (logic-r1 05, threat-r1 02, architect-r2 06, logic-r2 P1) |
| `TestSprintFourFolderSettlement`. Orchestrator, Link and Projects. | At the server interruption point, expect no save, mark advance or acknowledgement. Stage a result, restart and run the sweeper past its cutoff. Expect no resend or sweep and one settlement after real replay. Test settlement before the report and removal on either side of it. A late eligible report advances and acknowledges without a Projects effect. (logic-r1 03 and P1, architect-r2 06, logic-r2 P3) |
| Projects and Store. | After ordinary pruning, retry the same operation and try a changed path and different procedure with its key. Expect one command and two conflicts. Test matching absolute origins, distinct origins with equal commits, known absence, withholding, and changed origin on an existing folder. Relative Secure Shell origins from different accounts and with different first commits are withheld and cannot match. (logic-r1 04 and 06, architect-r2 04, logic-r2 05) |
| Projects and Website. | Through the real router, run each origin preflight with a listed and an unlisted origin, including the request-key header on both unary preflights. Only the listed origin can make the signed-in calls. Run the refusal table at the handlers. Malformed bodies for both unary calls and the watch produce fixed text and create no operation. A lagged watch ends with `aborted` and starts with a replacement snapshot on reopening. Expiry stops an open watch. Superseded inspection replies and records from the old session or server are discarded. (architect-r1 01 and 09, logic-r1 07, critic-r1 5, threat-r1 05 and 06, logic-r1 P4) |
| Folders and Projects. | Inspect a checkout with an inert side-effect sentinel. Expect no sentinel or network activity. Plant synthetic credentials in a remote query, path, branch, Facts workspace, inventory text and logs. Expect omission, refusal or masking as specified. Check both logs, client buffer, server records and browser replies for absence. Safe legacy replay preserves bytes. An unsafe required legacy event faults before handoff and retains its bytes and number. (threat-r1 03 and 04, architect-r1 08, critic-r1 3) |
| Stack and Website. | Observe the revision 1 refusal and upgrade in place. Apply one old event without rerunning a receipted command. Verify certificate pinning off loopback and private state permissions. Use one signed-in website build on both samples and old and new real servers. Only the new registered server enters repository mode. Expect matching saved identities with the add reply first and with the watch first. Run deployed R14. (architect-r1 10, critic-r1 7, logic-r1 P4, threat P5) |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is assignment bullets 1 to 4 and their settlements, foundation D1, D2, D4, D5, D7, rules 1 to 11, and C3, C7, C10 to C12. It includes the newer sprint 4 design, checks R1 to R14, its open question 2, issue 1674 point 4, both round-three closing lists, and build loop Phase 4.

| Source item | Workstream | Status |
|---|---|---|
| Assignment 1 and 2. Link and freeze. | WS-1, WS-2 | Items 1 and 2 cover fields, declarations and history. |
| Assignment 3 and 4. Service, discovery and generation. | WS-2, WS-3 | Items 2 to 4 cover both generators and samples. |
| Settlements and both round-three closing lists. | WS-1, WS-2, WS-3 | Work items or dated decisions assign each point. |
| D1, D2, and rules 1 to 4. Part and store boundaries. | WS-1, WS-2, WS-3 | Review the contract first. The dependent plans own runtime and migrations. |
| D4, D5, C12 and rule 5. PostgreSQL and watch. | WS-3 | Use the shared feed and test with PostgreSQL. |
| D7 and C10. Proof. | WS-2 | Keep the proof vectors and revision refusal order. |
| Rules 6 and 7 and C11. Stored commands and replay. | WS-2, WS-3 | Verify durable admission, receipt and operation rules. |
| Rules 8 to 11. Errors, secrets, tests and credit. | WS-2, WS-3 | Pin fixed sentences and the filter. Use real fixtures. Copy no source. |
| C3 and C7. Sessions and Machines. | WS-2, WS-3 | Test protected watch entry, waiting replay and old-record decoding. |
| Newer design sprint 4 and shared link rules. | WS-1, WS-2, WS-3 | Items 1 to 4 fix layouts. Link publishes its Go boundary. |
| R1, R2, R3. Facts, resolved path and aliases. | WS-2, WS-3 | Pin facts and stored identities. |
| R4, R5. Attach and keyed replay. | WS-3 | Pin origin comparison and hold the receipt. |
| R6, R7. Report wins and late expiry. | WS-2, WS-3 | Pin settlement and run the receipt and operation cases. |
| R8, R9. Commit order and run gate. | WS-2 | Run the replay and permit case. |
| R10, R11. Result restart and upgrade. | WS-1, WS-2 | Verify staged result and in-place upgrade. |
| R12, R13, R14. Removal, discovery and deployed add. | WS-2, WS-3 | Run removal, sample and deployed cases. |
| Open question 2 and issue 1674 point 4. | WS-2, WS-3 | Items 2 and 4 pin recovery and one acknowledgement per eligible event. |
| Build loop Phase 4. | WS-1, WS-2, WS-3 | Each item names files, red tests, evidence and mutations. |

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

Load. OpenAI supplies all sessions. The original budget was 15 launches, including four test writers, four builders, four plan reviews and three focused code reviews. Round 1 used its four seats. The orchestrator checks remaining allowance before each build or review batch. Use separate sessions and one writer per unit. Review the red assertions and freeze the tests before building.

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Round 1 | 30 findings before grouping. | [The round 1 register](attack/round-1-findings.md) records the patches and the stuck statement. |
| Round 2 | 13 findings before grouping. | [The round 2 register](attack/round-2-findings.md) records the final fold. The orchestrator owns the transition to build. |

</details>
