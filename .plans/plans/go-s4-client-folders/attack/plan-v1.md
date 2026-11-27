---
topic: go-s4-client-folders
status: attack
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# Inspect a machine's checkout and upgrade its link

## Overview

Sprint 4 lets the website ask a machine what is in an existing Git checkout. The client reads the resolved checkout path, branch and working-tree status without changing the repository or contacting its remote. The server can use that report to save the folder under the machine.

The upgrade keeps the machine's existing enrollment, event buffer and command receipts. When the client restarts, it announces where its new events begin. A retained report can still reach the server if the old process died before answering the command. A client that detects unusable history stops and tells the operator how to recover it.

One filter screens outgoing reports and startup logs for secrets the client knows. Display text can be masked. The client refuses a folder when a secret would otherwise be sent as part of its path or identity. The macOS keychain gap remains recorded, and the plan tests the filter with synthetic credentials on both operating systems.

## Key decisions

A restarted client announces where its new events begin. It keeps that boundary while it reconnects. The server can then apply the old events before it sends more work.

A command result keeps the event number it had when it was first recorded. A later attempt can return that number even after another restart. The client stops with a recovery instruction if it cannot supply the history the server needs.

The folder reader uses local Git data. It can return an exact path or branch without disclosing a known secret by refusing an unsafe folder. It can still report the rest of the facts when it has to withhold an origin.

Full log → [decisions.md](decisions.md).

## Workstreams

### The client can reconnect with its saved history

An installed client keeps its buffer and receipts when it upgrades. A waiting machine can deliver retained events before approval. If enrolled history is missing or unusable, the operator sees one instruction and the installed service stays stopped.

### The client can inspect a checkout without changing it

The machine resolves links and subdirectories to the top of the checkout. It reports the branches, origin when usable, first commit when known, and whether the working tree is clean. A real-link test kills the client between recording the report and finishing the command, then verifies that the server saves the result once.

### Reports and logs pass the same filter

The client masks display text and leaves usable identifiers exact. A planted-value test checks the actual executable's startup log and outgoing reports. A replay test verifies that a required unsafe old event stays on disk and stops the client.

## Rollout order

1. Land the reviewed revision 2 contract and give the client and server teams the same generated code. The two installed revisions require a coordinated upgrade.
2. Run the client tests and the real client and server tests against disposable data. Keep an untouched copy of the state written by the sprint 3 client for the installer check.
3. Upgrade the deployed server and reinstall the client through the existing installer. Use the existing enrollment and verify that the buffer and receipts survive before trying the repository dialog.
4. On disposable installations, exercise the local recovery instruction and the normal requested restart on both operating systems. Add a real repository from each website entry point to complete the deployed check.

---
<!-- ⛔ Implementation details and gate evidence follow. -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1 — The client can reconnect with its saved history**

The durable event counter already records the greatest number appended, even after acknowledgements clear the queue. Use that counter to identify the process boundary and to capture the first result watermark. The client keeps one process identity in memory while the server handles admission and command ordering.

**Done when**

| Item | Criterion | Observed by |
|---|---|---|
| 1 | A new process captures the next number before appending. Reconnects repeat the pair, and waiting streams deliver the retained prefix under the existing window. | `TestRevisionTwoRunAnnouncement`, `TestRevisionTwoWaitingPrefix`, `TestReviewGUnacknowledgedEventWindow`. |
| 2 | Finished, interrupted and standing receipts return their first watermarks. A sprint 3 store opens in place and replays its old result with zero. | `TestRevisionTwoReceiptWatermarks`, `TestRevisionTwoReceiptMetadata`, `TestRevisionTwoLegacyStore`, `TestSprintFourReceiptReplay`. |
| 3 | A required history fault or incompatible Welcome yields one diagnostic and no redial or automatic service restart. A normal requested restart still happens. | `TestRevisionTwoTerminalHistory`, `TestRevisionTwoExecutableStop`, `TestSprintFourHistoryRecovery` and the two supervisor checks. |

**Tasks**

| Item | Task | Anchor |
|---|---|---|
| 1 | Introduce the run pair and make the old prefix eligible before approval. | `src/client/internal/link/conn.go` (`openSession`, `proofText`), `src/client/internal/link/session.go` (`Run`, `liveSession.serve`), and new `src/client/internal/link/run_identity.go` beside `conn.go`. |
| 2 | Save and replay the first result and seal interrupted starts before the next process can append. | `src/client/internal/link/store.go` (`inspectBuckets`, `Store.update`), `outbox.go` (`Store.appendContext`), `receipts.go` (`readReceipt`, `findReceipt`, `writeFinishedReceipt`), `commands.go` (`commandHandler.handle`, `settleScan`, `restart`). |
| 3 | Stop on incompatible history and carry that outcome to the executable. | `src/client/internal/link/replay.go` (`newReplay`, `replay.next`), `session.go` (`clientRun.attempt`, `stopForStore`), `retry.go` (`supervise`), `src/client/internal/app/client.go` (`Run`) and `src/client/cmd/lgtm-client/main.go` (`runContext`, `main`). |

**Item 1. Send the revision and the process boundary.** Create `src/client/internal/link/run_identity.go` and `revision_two_session_test.go` in that folder. Change `conn.go` and `session.go` there. Update only the revision-dependent fixtures listed below. Start this with the reviewed Contract outputs. Items 4 and 5 can run alongside it. Items 2 and 3 use its process startup and stream code and merge after it. This covers foundation decisions D7, D16 and D17, foundation rules 2, 6 and 7, foundation checks C6 and C8 to C11, and sprint checks R8, R9 and R11.

- Set `linkRevision = 2`. Generate one random version 7 universally unique identifier (UUID) in `link.Run`. Use Go's existing cryptographic random source and system clock, with no new module. Capture `runIdentity{id string, firstEventNumber uint64}` once from `Store.Next()` before any report, dial or outage worker. Pass it to `openSession` on every attempt. Feed `Hello.run_id` and `Hello.first_event_number` from that pair and use a fresh nonce on each attempt. Accept `Welcome.link_revision=2` after the existing structural and signature checks. Keep both `lgtm-link-v1` proof labels byte for byte.
- Keep `Welcome.last_event_number` as the replay cursor. With approval pending, allow only retained events with numbers below the captured boundary and bare heartbeats. Start fresh reports under the existing approval rule. Keep a single sender, the cumulative acknowledgement cursor and the ceiling recorded when an acknowledgement is received. Reuse the window of exactly 32 event handoffs after the greater of the Welcome mark and the last accepted acknowledgement. A heartbeat or a command result can use the sender while that window is full. The server owns the command gate and sends the last required acknowledgement before the gated `SetState`.
- Keep the ordinary reconnect behavior for `invalid_argument` and `failed_precondition` with `Lgtm-Link-Refusal: event_gap`. Keep a verified post-Challenge `failed_precondition` with `Lgtm-Link-Refusal: link_revision_mismatch` final. Item 3 owns the stop after an incompatible Welcome and keeps malformed Welcome and unverified refusal behavior.
- **Tests written first.** In `revision_two_session_test.go`, write `TestRevisionTwoRunAnnouncement`. Seed events 1 through 3 and require first number 4 on the announced UUID. Reconnect after acknowledging or appending an event and require the original pair and new nonce. Restart and require a different version 7 UUID and the actual next number. On two no-event starts, require different identifiers with equal first numbers. Check a verified `invalid_argument` retries with the same pair. Use the existing proof fixture for unchanged signed bytes.
- In the same file, write `TestRevisionTwoWaitingPrefix`. Seed 33 events before startup. On a waiting Welcome, observe 32 consecutive handoffs and a bare heartbeat with no freshly produced report. Acknowledge one and require the 33rd, then acknowledge the prefix before sending approval. Require new reports after approval. Repeat with an empty retained prefix and with a same-run reconnect partway through replay. Use the shared limit fixture, and let the existing window test keep exercising its full-window command and heartbeat path.
- **Risky mutations.** Generate the pair inside `attempt`, capture the floor instead of the next number, change either proof label, suppress waiting replay or change the window to 31 or 33. The matching test must fail on its assertion.
- **Acceptance on Linux and macOS.** Run `go test -race ./src/client/internal/link -run 'TestRevisionTwoRunAnnouncement|TestRevisionTwoWaitingPrefix|TestReviewGUnacknowledgedEventWindow|TestOpenProofFixture' -count=1` natively on each, then run `TestSprintFourWaitingReplay` and `TestSprintFourPendingRun`.

**Item 2. Save the first result watermark and accept old receipts.** Change `src/client/internal/link/store.go`, `receipts.go` and `commands.go`. Create `revision_two_receipts_test.go` and `testdata/client-v1.db` in that package. Change the recovery assertion in `receipts_test.go` and the startup call in `session.go`. This follows item 1. Item 3 follows the receipt change. Items 4 and 5 can run alongside it. This covers foundation decisions D7 and D16, rules 6, 7 and 10, checks C9 to C11, and sprint checks R5 to R11.

- Keep the existing bucket layout and numbered payload bytes. Accept the old store metadata as it stands. Define version 2 receipt JSON (JavaScript Object Notation) with `version`, `expires_at`, `state`, the existing base64 `result` when finished, `last_event_number` and `standing`. A finished receipt requires an explicitly present unsigned number between zero and the durable greatest number. A current `started` receipt has no result and an explicit null watermark. A recovered receipt has state `interrupted`, no result and the sealed integer watermark. The state and watermark must agree. A production version 2 result must contain a recognized client outcome. A missing field, null on a finished or interrupted receipt, an invalid state or damaged metadata yields `ErrStoreDamaged`. Recognize version 1 by its actual format and field presence. A positively identified version 1 result or interrupted start uses zero. Preserve its record and reject malformed version 1 envelopes in production startup or on lookup. Version 1 never wrote a standing receipt.
- In `Store.prepareRun() error`, validate stored receipt envelopes and seal prior version 2 receipts in state `started` to `next - 1` in one bbolt transaction before the new process may append or accept work. Validate before writing. Do no write when there is nothing to seal. On recovery, an `interrupted` receipt returns interrupted with its saved value and is never resealed. The store is exclusive to one process. The existing in-memory running map distinguishes active starts from recovered ones. An uncertain commit poisons the store before more work. Read through the original store, with no import, copy or destructive rewrite of the event buffer. Keep the existing public store helper signatures where frozen tests use them. Run process startup preparation in tests that exercise the command handler across reopen. Keep `OpenStore` itself read-only on an existing file. The link's receipt operations return `savedResult{outcome linkv1.CommandOutcome, lastEventNumber uint64}` and avoid a second lookup after committing.
- Compute a new result under the store lock in the same write that stores the receipt. Use `next - 1` even when the outbox was acknowledged and is now empty. Use the saved outcome and watermark with the current `Command.id` and `attempt` in `CommandResult.last_event_number`. Use this path for done, failed, expired and unsupported outcomes, including a failed report append that consumes no number. Persist `SetState` before replying, including when waiting or after expiry. Keep its existing revision comparison and idempotent state application when replaying. Exempt its finished receipts from ordinary pruning. Keep the 23-hour offer and the 24-hour ordinary receipt margin. Keep recovered interrupted receipts under the existing started-receipt retention rule. Keep the existing restart half-close budget and worker drain behavior.
- **Tests written first.** In `revision_two_receipts_test.go`, write `TestRevisionTwoReceiptWatermarks`. Capture a result after an event becomes durable. Acknowledge and empty the queue, append a newer event, and replay on a new attempt twice across restarts. Assert the first number for requested scan, failed and expired outcomes, restart, unsupported and expired standing state. Force a different receipt to trigger pruning after ordinary expiry and then replay `SetState`. Confirm the newest applied machine state follows its revision.
- In the same file, write `TestRevisionTwoReceiptMetadata`. Kill after a durable start and a further append, before finish. Restart, append unrelated events and restart again, then request the old command. Require interrupted with the original process's last number and no repeated work. Exercise zero history, a torn seal transaction, an explicitly present zero, an omitted field, null on a finished or interrupted receipt, a mark above the durable counter, an unknown receipt version and a malformed version 1 record. Corrupt required metadata must give no result and no rewrite of that record.
- Write `TestRevisionTwoLegacyStore` there. Generate `testdata/client-v1.db` with the actual sprint 3 client built from baseline `44c931d9`, a generated synthetic identity and a signed revision 1 peer. Have the peer leave an unacknowledged Facts event, a finished command and an interrupted start. Record the producing commit, commands and expected numbers in the test. Copy the fixture before use. Decode it through the new executable path and require unchanged safe payload bytes and replay with zero on new attempts, with no rerun. Keep the producer available for the separate live upgrade check. A database assembled by the revision 2 writer does not count.
- **Risky mutations.** Sample the watermark on resend, replace it with queue length or acknowledgement mark, default a missing revision 2 watermark to zero, reseal a started receipt after new appends, or prune a standing receipt. Each matching test must go red.
- **Acceptance on Linux and macOS.** Run `go test -race ./src/client/internal/link -run 'TestRevisionTwoReceipt|TestRevisionTwoLegacyStore|TestReviewFRestartHalfClosesBeforeCancel' -count=1` natively on each. Run the real receipt replay in rollout step 2.

**Item 3. Stop locally on an unusable replay.** Change `src/client/internal/link/replay.go`, `session.go`, `retry.go` and `src/client/internal/app/client.go`. Create `src/client/internal/link/revision_two_recovery_test.go` and `src/client/internal/app/revision_two_process_test.go`. Update the explicitly listed frozen assertions. This follows items 1 and 2 and uses the Stack stop-status agreement. Items 4 and 5 can run alongside it. This covers foundation decisions D16 and D17, rules 7 to 10, checks C8 to C11, sprint checks R9 to R11 and the contract's history recovery supplement.

- Before calling `Store.Ack` on a verified Welcome, reject a mark at or above local next or below the retained floor minus one. Treat local next as the floor of an empty buffer. Read the required range in small batches. Check that numbers are contiguous through the captured high-water, each payload decodes, and each required event fits the revision 2 limit. Item 7 adds the same filter to this check. A missing row within the required range, including a premature end, is a fault. Preserve the outbox when this check fails. A confirmed prefix at or below the server mark needs no replay. Keep validation at handoff for events added after the initial check. Retain the acknowledgement guard at frame receipt.
- Use explicit `link.RunRecoveryStop` for a required local history fault, a structurally valid incompatible Welcome, or a verified final revision refusal. `RunRecoveryStop` carries no retry error and maps through `app.Run` to the Stack-agreed no-restart process status. The proposed status is 0, which matches both installed supervisor definitions. Keep the accepted restart status 75 and retryable failures at 1. Keep the recovery status if cleanup also fails. Consolidate the diagnostic into one record for each terminal cause. For history, identify `client.db` and tell the operator to restore the original saved history or remove the machine and enroll it again in a fresh state folder. For a wrong revision, tell the operator to install a compatible client and keep the state folder. Drop raw paths and underlying error strings. A commit failure with unknown durability retains the existing stop-and-retry-by-supervisor path unless the next startup diagnoses invalid history. Keep ordinary contention and cancellation on their current paths.
- **Tests written first.** Write `TestRevisionTwoTerminalHistory` in `revision_two_recovery_test.go`. Use intact enrollment and inject a missing database, corrupted event, a hole including an empty retained suffix, a mark below the floor, a mark at next and damaged required receipt metadata. Require one instruction, no second dial, no result or fabricated event, and unchanged surviving event bytes and numbers. For invalid Welcome revision, complete the proof, send a structurally valid revision 1 or unsupported Welcome, and require one incompatible-revision instruction and no second dial. Keep separate negative cases for invalid Challenge, malformed Welcome and retryable remote `invalid_argument`.
- Write `TestRevisionTwoExecutableStop` in `revision_two_process_test.go`. Build and start the actual command with a disposable enrolled folder, inject local history loss with valid identity, and assert the exact agreed exit status, one instruction, no fresh database and no travel of an injected error string. In another run, accept a `Restart` with a valid proof and assert exit 75 and a durable result. This distinguishes ordinary termination from a restart request.
- **Risky mutations.** Acknowledge before validating the range, skip a missing number, retry an incompatible Welcome, map terminal history to status 1, or emit two terminal instructions. The corresponding test must fail.
- **Acceptance on Linux and macOS.** Run `go test -race ./src/client/internal/link ./src/client/internal/app -run 'TestRevisionTwoTerminalHistory|TestRevisionTwoExecutableStop|TestRetryFailureExitsAndKeepsStore|TestAppRunStoreStatus' -count=1` on each. Stack must exercise the installed service independently.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A restarted client takes work ahead of its saved reports. | High | Compare the two process fields across reconnects and assert replay and application before command delivery in the real gate test. |
| A result covers the wrong events after acknowledgement or a restart. | High | Verify the persisted number after two restarts and an unrelated append, including an interrupted command and a standing reply. |
| A supervisor loops while durable history is missing. | High | Check the actual process status and one instruction, then observe both supervisors beyond their restart interval. |

**Verify.** Run items 1 to 3 on native Linux and macOS. Use disposable enrolled state, a signed protocol peer for boundary tests, and the paired real server with PostgreSQL for the sprint acceptance. Keep the existing certificate pinning, timing, removal, window and restart tests.

**Rollout detail for steps 1 to 4.** Review with `LGTM_LINK_BASE=fcfa96c3 bash scripts/check-go.sh` and the reviewed generated files. Advance the pin only after the contract lands. Run `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^TestSprintFour' -count=1` and require five explicit passes with zero skips. Use a disposable installation for stop and re-enrollment proof. Stack runs the old-client refusal after a verified Challenge, installer upgrade with intact enrollment, public certificate-key pinning and private-state permission checks.

**WS-2 — The client can inspect a checkout without changing it**

The Folders part reads checkout facts. Link owns expiry, duplicate delivery, append order and results. It runs the filesystem work off the stream owner so an inspection cannot hold up heartbeats or acknowledgements.

**Done when**

| Item | Criterion | Observed by |
|---|---|---|
| 4 | A dirty checkout with a distinct remote head, link aliases and an exact origin returns its local facts. Unsupported or unreadable inputs receive a fixed refusal. | `TestInspectFolderFacts`, `TestInspectFolderRefusals`, `TestInspectFolderOrigin`, and R1 to R4 through the real link. |
| 5 | Inspection leaves the work tree, index and Git metadata unchanged. No repository-controlled process or network call occurs. | `TestInspectFolderReadOnly`, `TestInspectFolderNoSideEffects`. |
| 6 | A durable report precedes the first result or survives interruption. Retry does not inspect again. Limits reject oversized events without skipping a number. | `TestLookAtFolderLifecycle`, `TestLookAtFolderEventBudget`, `TestSprintFourFolderSettlement`, and `TestSprintFourReceiptReplay`. |

**Tasks**

| Item | Task | Anchor |
|---|---|---|
| 4 | Resolve the checkout and read its metadata. | New `src/client/internal/folders/{inspect,origin}.go`. Reuse the process and context pattern at `src/client/internal/harness/scan.go` (`Scan`) and the home input at `src/client/internal/config/config.go` (`LoadSettings`). |
| 5 | Read status with program execution and writes fenced out. | New `src/client/internal/folders/status.go` beside the metadata reader. The process pattern is `src/client/internal/harness/scan.go` (`Scan`). |
| 6 | Wire the folder command and durable report into the link owner. | `src/client/internal/link/commands.go` (`startScan`, `finishCommand`), `outbox.go` (`Store.appendContext`), `session.go` (`liveSession.serve`), and new `folder_command.go` beside `commands.go`. The fixture bridge follows `src/client/clienttest/clienttest.go` (`Options`, `Run`) and `src/client/internal/app/client.go` (`Run`). |

**Item 4. Resolve the checkout and read local facts.** Create `src/client/internal/folders/inspect.go`, `origin.go` and `inspect_test.go`. Items 1 to 3 and 5 can run beside it. Use the reviewed Folders wire names and agree on item 5's reader signature first. Item 6 integrates the reader after items 2 and 5. This covers foundation rules 1, 2, 8 and 9, and sprint checks R1 to R4, R7 and R14.

- Export `func Inspect(ctx context.Context, path, home string) (*linkv1.FolderFacts, linkv1.FolderRefusal)` from `src/client/internal/folders`. Accept an absolute path, bare `~`, or `~/...`. Reject other user expansions and relative paths. Enforce the 4,096-byte input limit and check valid Unicode Transformation Format, 8-bit (UTF-8), with no zero byte. Walk symlinks using the host filesystem, check that the input is a directory, and resolve the physical top level Git working tree. Use its basename as the name. Inspect linked worktrees and supported subdirectories using local Git discovery. A resolved path longer than 8,192 bytes gives path not usable.
- Return `FOLDER_REFUSAL_NOT_A_FOLDER` for an absent path, a dangling link or a non-directory. Return `FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY` for a readable directory outside a Git working tree, including a bare repository. Return `FOLDER_REFUSAL_NOT_READABLE` for denied reads, an unusable Git reader or a checkout whose status cannot be established safely. Use `FOLDER_REFUSAL_PATH_NOT_USABLE` for path syntax, invalid exact text or a path that passes the filesystem limit but exceeds the wire limit. Use path not usable for a symlink loop. Keep raw path, Git diagnostics and operating-system error text out of both the reply and logs.
- Read the local symbolic `refs/remotes/origin/HEAD` first for the main branch, then the locally checked-out branch, then `main`. A detached checkout has no checked-out branch. An unborn symbolic branch is a checked-out branch with no first commit. Find the root of the locally available main ref or the checked-out commit if that ref is unavailable. If a complete walk reaches several roots, use the lexicographically first full object identifier. Leave the first commit absent when a full root cannot be established, including an unborn checkout or a walk reaching a shallow boundary. Preserve Git's full object identifier regardless of hash format.
- Read the configured origin without asking a remote or invoking a credential helper. Remove user information from allowed network Uniform Resource Locators (URLs) and Secure Shell (SSH) shorthand. Preserve the qualified address as local evidence and leave transport and comparison normalization to Projects. Allow the schemes and absolute SSH shorthand in the contract. Omit account-relative forms, including a home marker after the scheme's slash, local paths, ambiguous forms, query strings, fragments, escaped paths and dot segments. Withhold an otherwise safe address over 2,048 bytes after removing user information. A configured but withheld origin has absent `remote` and `remote_omitted=true`. An absent origin has absent `remote` and `remote_omitted=false`. Item 7 screens the remaining exact value for known secrets.
- **Tests written first.** In `inspect_test.go`, write `TestInspectFolderFacts` for a repository with remote head `main`, local branch `topic`, changed tracked and untracked content, and an exact root commit. Require identical canonical paths from a symlink and a subdirectory. Also test a linked worktree, a detached checkout with and without a known remote head, an unborn branch, a locally shallow checkout and multiple roots. Assert optional presence explicitly.
- Write `TestInspectFolderRefusals` there for a regular file, missing directory, denied read under an ordinary service user, non-repository, bare repository, broken link, loop, invalid path bytes and absent home. Cover exact typed and resolved path boundaries. Use a unit helper with synthetic long resolved paths where the platform rejects them before Git can inspect them. Verify all four enum arms with no exception text.
- Write `TestInspectFolderOrigin` there for absent, allowed and withheld origins. Include absolute and account-relative SSH forms, a URL with user information, and a query, fragment, escaped segment, dot segment and local path. Include a 2,048-byte sanitized origin and a 2,049-byte one. Compare an expected complete safe address and absence of credential fragments.
- **Risky mutations.** Echo the typed path, prefer the current branch over the remote head, send configured user information, or treat a withheld origin as absent. Expect an assertion failure for each.
- **Acceptance on Linux and macOS.** Run `go test -race ./src/client/internal/folders -run 'TestInspectFolderFacts|TestInspectFolderRefusals|TestInspectFolderOrigin' -count=1` with native Git as an ordinary user. Run R1 and the alias inputs of R2 and R3 through the real link.

**Item 5. Prove Git reads status without side effects.** Create `src/client/internal/folders/status.go` and `status_test.go`. This can run beside items 1 to 4 once the local input is agreed. Item 6 follows it. This covers foundation rules 8, 9 and 11, and sprint checks R1 to R3 and R7.

- Keep `workingTreeClean(ctx context.Context, checkout checkoutPaths) (bool, error)` private to Folders. Read staged, unstaged and untracked change, and include populated submodule changes. Use explicit, fixed-argument Git plumbing for discovery and a read-only porcelain status for clean. Select Git from the service's search path before changing its working directory. Clear inherited Git overrides, pin parser locale, disable network protocols and lazy fetch, and discard standard error. Bound command output and honor context cancellation. The status reader needs only the first complete dirty record or a clean exit with empty output. Do not invoke a shell, remote helpers, credential helpers, hooks, external diff, text conversion or filesystem monitoring.
- Give status a private, disposable metadata directory with a fixed, allowlisted Git configuration, the captured HEAD, and access to the original objects and a read-only index snapshot. Use the original work tree. Keep the metadata directory outside the inspected checkout and remove it on every exit. Disable optional locks, filesystem monitoring and external commands in the private view. Carry only inert settings needed for Git's status semantics, plus local ignore data and shared-index data when present. Never load original configuration into the execution-capable status command. Read declared external filters as data. If correct clean status requires executing a filter, or inspection lacks required local objects or metadata, use the fixed not-readable refusal. Preserve tracked submodule target changes and inspect populated submodules by the same isolated read, with the same cancellation and without running arbitrary submodule configuration. Treat a configuration or index change during capture as an unsuccessful read if it would mix incompatible snapshots.
- **Tests written first.** In `status_test.go`, write `TestInspectFolderReadOnly` against clean, staged, unstaged, untracked, ignored and submodule states including a linked worktree. Compare the index, reference, configuration and file contents and relevant metadata before and after. Require that no lock or new file remains in the checkout. Test canceling a blocked local probe and require the worker to exit.
- In the same file, write `TestInspectFolderNoSideEffects`. Use disposable checkout configuration with inert marker programs for a hook, filesystem monitor and external conversion, and a local transport sentinel. Require no marker and no request. A configuration that makes conversion necessary yields not readable. Check that unrelated safe configuration still yields facts. Change the source configuration after capture to prove that the status invocation cannot use the new program.
- **Risky mutations.** Use the original config for status, allow optional index updates, ignore untracked changes, or ask the remote for its head. Require at least one frozen assertion to fail for each.
- **Acceptance on Linux and macOS.** Run `go test -race ./src/client/internal/folders -run 'TestInspectFolderReadOnly|TestInspectFolderNoSideEffects' -count=1` natively on each with Git and an ordinary account. Run the source tree diff and marker checks in the tests.

**Item 6. Append a folder report before answering its command.** Create `src/client/internal/link/folder_command.go` and `folder_command_test.go`. Change `src/client/internal/link/{commands,outbox,replay,session}.go`, `src/client/internal/app/client.go` and the test adapter `src/client/clienttest/clienttest.go`. This follows items 2 to 5 and the filter interface from item 7. The approved command dispatcher and fixture paths are shared with items 3 and 7. Land them in that order. This covers foundation rules 1, 2, 6 to 10, checks C9 and C11, and sprint checks R1 to R11 and R14.

- Decode the exact reviewed `Command_LookAtFolder` arm, validate `LookAtFolder.operation_id` and use the existing approval, clock and command receipt ordering. Check the complete inbound command's encoded size before reserving a receipt. A stale command is declined before any filesystem call. Confirm expiry again immediately before the worker starts. Use one folder worker slot alongside the existing scan slot. A duplicate uses the same work and newest attempt on the current stream. If the slot is busy for a different command, use the existing busy/retry pattern with no new receipt or read. The owner can still receive heartbeats, replays and restart. Cancel and join the worker at process stop or an accepted restart using the existing worker completion pattern.
- Return checkout facts to the owner of the link. Send one `Event_FolderSeen` with the exact operation and command identifiers, and a `FolderSeen_Facts` or `FolderSeen_Refusal` arm. Use the common outgoing filter and an event timestamp from the client. Commit the report through the outbox and record done afterwards, for either arm. If the buffer cannot admit a report, record failed with the actual current watermark. Distinguish full capacity from uncertain commit. On uncertain commit, stop without a result or a second append. Keep command reports separate from coalescing Facts or inventory.
- While holding the existing store lock, size the complete encoded `Event` at its proposed number using `proto.Size`. Include its timestamp, oneof and nested fields. The event limit is 49,152 bytes, separate from the 65,536-byte outer request or response limit. Refuse before advancing the durable counter, and replace oversized folder facts with `FOLDER_REFUSAL_PATH_NOT_USABLE` at the same available number. If that refusal also fails a definite admission check, save failed with the unchanged watermark. Set both transport caps with `connect.WithReadMaxBytes` and `connect.WithSendMaxBytes`. Check the budgets from `src/contracts/testdata/link-v2-limits.json` in the tests. Preserve the stored unnumbered envelope representation and give the next event its number only on the outgoing copy.
- Call `AfterFolderReport` immediately after the durable append and before finishing the receipt or handing off the report. Expose it as `AfterFolderReport func(context.Context, uint64) error` on `clienttest.Options`, threaded through `app.Options` and `link.RunOptions` with nil in the executable. An error at this fixture boundary ends the fixture run without writing a result or settling the still-started receipt. Never implement it as an environment or command-line switch. Capture the supplied event number and state when the fixture ends. Reopen with the ordinary code path and the callback unset.
- **Tests written first.** In `folder_command_test.go`, write `TestLookAtFolderLifecycle`. Drive a waiting and approved stream, including a valid checkout, fixed refusal, duplicate on a new attempt, late delivery after expiry, full buffer and a context cancellation while inspection is blocked. Count actual inspections. Hold the read and observe heartbeats and receipt of acknowledgements. Require one durable `FolderSeen` with both identifiers before its done receipt. Inject interruption through the fixture and require the same event number after reopen, the previously captured started receipt, and interrupted on the newest attempt without another inspection. Reopen a second time and check the same watermark.
- Write `TestLookAtFolderEventBudget` there using production Protocol Buffers encoders. Exercise a numbered event at exactly 49,152 bytes and one byte over, including a number requiring more than one variable-length byte. Prove an oversized facts event leaves the original candidate number for the fixed refusal. Fill the outbox and check no number is consumed and the failed result has the last durable mark. Test inbound command size at the contract boundary and exercise the complete outer message cap with a real encoded message. Include a full 32-event window and a cumulative acknowledgement that allows exactly one further handoff.
- **Risky mutations.** Write the receipt first, record done on a full buffer, run an expired command, size an unnumbered event, or check the callback after finishing. Require the exact assertion in the lifecycle or budget fixture to fail.
- **Acceptance on Linux and macOS.** Run `go test -race ./src/client/internal/link -run 'TestLookAtFolderLifecycle|TestLookAtFolderEventBudget|TestReviewGUnacknowledgedEventWindow' -count=1` natively on each. Run the real folder settlement and receipt replay cases with the test adapter.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An inspection mutates the checkout or executes checkout configuration. | High | Use restricted Git reads and a private configuration for status. Verify the inert sentinels and byte comparisons on both hosts. |
| A reply overtakes a report or loses it during a crash. | High | Seal the started receipt after a real durable append. On restart compare the number and applied folder row before and after settlement. |
| Oversized metadata consumes a number that will never be sent. | High | Check the complete numbered message before commit. Try the exact limit, the next byte and a full buffer. |

**Verify.** Run items 4 to 6 with native Git on Linux and macOS and with a disposable dirty repository, a linked worktree and a synthetic origin. Run the real PostgreSQL-backed folder tests in rollout step 2. Stack and Website own R14.

**WS-3 — Reports and logs pass the same filter**

One process-level filter serves the application logger and the buffered reports. It uses the same knowledge to decide whether an immutable old report can be replayed. Link never assigns a number to an unsafe new folder report.

**Done when**

| Item | Criterion | Observed by |
|---|---|---|
| 7 | Display text is masked before append, exact fields are omitted or refused, and unsafe retained events preserve their number and bytes when replay stops. | `TestOutgoingFilterText`, `TestOutgoingFilterReports`, `TestOutgoingFilterLegacyReplay`. |
| 8 | The actual startup log, process output, buffer and wire reports contain none of the planted secrets. | `TestOutgoingFilterExecutable`, `TestOpenFrameDebugDoesNotLogConnectionToken`, and the paired capture in rollout step 2. |

**Tasks**

| Item | Task | Anchor |
|---|---|---|
| 7 | Filter new and replayed reports with one exact-value and display-text policy. | New `src/client/internal/link/filter.go` beside `report.go` (`reporter.publish`, `reportString`), `replay.go` (`encodeReport`, `decodeReport`, `replay.next`), and `conn.go` (`validateWelcome`). |
| 8 | Seed the filter and apply it to structured logs from process startup. | New `src/client/internal/app/outgoing.go` beside `client.go` (`Run`, `withDefaults`). Reuse the credential safeguards at `src/client/internal/enroll/credential.go` (`Load`, `decodeCredential`, `Credential.Format`). |

**Item 7. Filter outgoing reports, including their replay.** Create `src/client/internal/link/filter.go` and `filter_test.go`. Change `src/client/internal/link/{report,replay,session}.go`. Give item 6 the filter interface early and land the main report changes before its dispatcher integration. This follows item 3 for replay validation. Item 8 uses the filter. This covers foundation rules 7 to 10, checks C7, C10 and C11, sprint checks R1, R2 and R11, and the shared outgoing-filter rule.

- Provide `type OutgoingFilter`, `NewOutgoingFilter() *OutgoingFilter`, `AddSecrets(values ...string)`, `MaskText(text string) string` and `ContainsSecret(text string) bool`. Keep the source strings private and safe from diagnostic formatting. Register the verified connection token with the shared filter before any report or log using that session. Register its representations actually held by the client. Retain registered values for the life of the process. Do not attempt to reconstruct expired tokens that the client no longer holds. Provide `NewTextStream()` with `Push([]byte) string` and `Flush() string` for callers with pieces of one text. Hold an undecided suffix across consecutive inputs and preserve whole characters. Mask matched text with the fixed marker `[redacted]`. Match overlapping secrets before releasing any bytes from a possible longer match.
- For new Facts and inventory, mask display fields before truncation. Keep fixed harness identifiers exact. Omit an unsafe Facts workspace. Screen a workspace before and after home abbreviation. Keep heartbeat load numeric. For `FolderSeen`, check the operation and command identifiers for exact safety before buffering. If a path, name, branch or commit is unsafe, emit the fixed path-not-usable refusal. Check typed and expanded paths before inspection. Check the resolved path and output facts before append. Omit an unsafe origin after stripping user information, with `remote_omitted=true`. If the correlation identifiers themselves contain a known secret, no safe report can name them. Treat the frame as invalid and send neither a report nor a result containing those identifiers. Use a fixed diagnostic.
- Pass every pre-existing queued payload through checks at replay and before handoff. A safe payload retains its stored bytes and its assigned number. If replay would require altering even a display field, use the local recovery stop. Do not send, renumber or discard an unsafe required event. Match on field values before serialization, including strings that would be escaped in the encoded payload. Read old report arms and keep their wire shape and capture times. Reject unknown fields in a required stored report because their text class cannot be established. Carry one filter through both report paths and the logger.
- **Tests written first.** In `filter_test.go`, write `TestOutgoingFilterText`. Register synthetic values with overlapping prefixes and characters outside the basic Latin alphabet. Cut a value across multiple pieces and a multibyte character across writes. Match the exact masked final text, confirm safe surrounding text and exercise a replacement while a suffix is pending.
- Write `TestOutgoingFilterReports` there for a synthetic private key form and token in Facts, inventory, and every folder exact field. Check unchanged safe exact values, masked text, omitted workspace, withheld origin and the fixed folder refusal. Include a credential embedded in a long value across its truncation boundary. Check correlation fields stay safe and capture raw persisted and transmitted messages.
- Write `TestOutgoingFilterLegacyReplay` there. Seed a known-safe and a known-unsafe version 1 event with synthetic values in display and exact fields. Across approved and waiting replay, require the safe legacy payload and matching number. Require the unsafe one to remain on disk with its original bytes and number while the client emits one recovery instruction and terminates.
- **Risky mutations.** Filter after truncation, mask a path in place, match each chunk independently, register the connection token after first use, or bypass screening for legacy data. The corresponding tests must fail.
- **Acceptance on Linux and macOS.** Run `go test -race ./src/client/internal/link -run 'TestOutgoingFilterText|TestOutgoingFilterReports|TestOutgoingFilterLegacyReplay' -count=1` natively on both. Confirm the actual executable path with item 8 and retain the frame-debug guard.

**Item 8. Filter logs from process startup.** Create `src/client/internal/app/outgoing.go` and `outgoing_test.go`. Change `src/client/internal/app/client.go` and use `revision_two_process_test.go` from item 3 if its executable-launch helper is needed. This follows items 3 and 7. It can run beside item 6 after the shared app options change is assigned to one writer. This covers foundation decision D19, rules 8 to 10, checks C6, C7 and C10, and the shared outgoing-filter rule.

- Create one filter before constructing the process logger. Use a wrapped `slog.Handler` that screens messages, structured values and keys, including grouped and attached attributes, before encoding. Keep numeric attributes numeric. Use fixed diagnostics before saved secrets can be loaded. A verified key read that cannot be reproduced for filtering halts startup before sending reports. Populate the filter before the `client_starting` record logs the state or workspace path. Read the enrollment key representation with no symlink follow, an owned private regular file descriptor and a small bound. Verify the decoded public half against the credential already loaded. Include the private key file form, block body, decoded bytes and forms the process actually holds. Keep the existing signer and avoid exposing a raw key through the test adapter, credentials formatter or a public API.
- At current process startup load readable local Claude Code sign-in values, from `.credentials.json` in `CLAUDE_CONFIG_DIR`, or from `.claude/.credentials.json` relative to the process home. Use bounded regular-file reads with no prompt and no environment dump. Collect credential string values and omit structural JSON keys and noncredential counters. Missing or unreadable sign-in stays outside the filter as recorded in issue 1670. The shared filter accepts added values so sprint 5 can refresh on agent start and file change. Sprint 4 starts no agent and needs no watcher.
- Keep the existing protections for enrollment and transport errors. Screen the enrollment argument when available to the logging path. Continue to discard peer error strings and Git diagnostic output. Start the shared link with the filtered logger and the same filter instance.
- **Tests written first.** In `outgoing_test.go`, write `TestOutgoingFilterExecutable`. Launch the real built client against a signed test peer with a temporary sign-in file and a synthetic credential embedded in the configured workspace. Add a synthetic connection token in a report-probe input such as a disposable inventory output. Capture complete standard output, standard error, durable queued payloads and decoded outgoing frames. Require a real startup record, a Facts report and an inventory report. Require the safe surrounding display text, masked structured paths, omitted workspace and no synthetic raw value, including encoded variants actually held by the client. Repeat with a planted value in an inspected branch and origin. Keep a separate controlled test of nested, attached and lazy attributes in the same file to catch bypass of filtering.
- **Risky mutations.** Emit startup before loading sign-in values, filter only the message, bypass attached attributes, or give the logger and link different instances. Require observable leakage of a synthetic value or a failed omission assertion.
- **Acceptance on Linux and macOS.** Run `go test -race ./src/client/internal/app ./src/client/internal/link -run 'TestOutgoingFilterExecutable|TestAppLogRedactsSecrets|TestOpenFrameDebugDoesNotLogConnectionToken' -count=1` natively on each. Use an explicit disposable sign-in file on both hosts.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Filtering silently changes an identifier used to locate a checkout. | High | Test exact preservation, refusal and omission separately. Confirm the server never received an altered key. |
| Startup or replay leaks a previously known credential. | High | Capture the full executable output and watch the real report path. Stop on an unsafe retained event before replay. |
| The client cannot read the Claude Code sign-in from the macOS keychain. | High | Keep issue 1670 open. Use synthetic readable credentials for cross-platform proof and preserve the recorded build decision. |

**Verify.** Run items 7 and 8 with synthetic secrets on each native platform. Inspect process logs, stored bytes and decoded frames. Use the paired real server capture for the server's stored facts and replies. Verify that the fixture leaves no real credentials or diagnostic data in the repository.

**Sprint 3 frozen test changes**

The test writer makes the assertion changes below before implementation. The existing proof vectors, trailer names, clock bounds and retry cases remain the baseline. Shared fixture edits provide revision 2 peers for tests whose original assertion still holds.

| Written test or shared fixture | Revision 2 change |
|---|---|
| `src/client/internal/link/conn_test.go` (`goodWelcome`, `open`, `TestOpenProofFixture`, `TestOpenExpiryAndRefusal`). | Use revision 2 for successful Welcome and Hello. Supply a fixed valid run pair when driving `openSession` directly. Keep signed bytes. Test revision 1 as incompatible alongside zero and an unsupported high revision. |
| `src/client/internal/link/conn_validation_test.go` (`TestOpenRejectsMalformedWelcomeBeforeRevisionMismatch`). | Give the malformed Welcome the now-incompatible revision 1 to keep the precedence assertion. |
| `src/client/internal/link/retry_test.go` (`wireLeg.hello` and direct `openSession` fixtures), and the direct `openSession` calls in `commands_test.go` and `replay_test.go`. | Switch peer acceptance to revision 2 and supply the fixture run pair. Keep the existing retry, gap, proof and refusal assertions. |
| `src/client/internal/link/session_test.go` (`runWelcome`, `TestRetryReportsOnApprovalAndOutage`, `TestRetryFailureExitsAndKeepsStore`). | Update the Welcome fixture. Require the saved event while waiting and acknowledge it before approval. Check the new terminal outcome and actionable diagnostic on required-history cases. For the live impossible acknowledgement case, send a mark above the recorded handoff ceiling without competing with newly allowed waiting replay. Keep the retryable commit and clean cancellation assertions. |
| `src/client/internal/link/replay_test.go` (`TestReplayRejectsDamagedPayload`). | Require rejection at initial replay validation before acknowledgement or event handoff. Keep the byte-preservation assertion. |
| `src/client/internal/link/receipts_test.go` (`TestBeginWritesStartedBeforeReturn`, `TestStartedFromEarlierRunIsInterrupted`). | Require the version 2 started format. Use startup preparation to observe the stable interrupted watermark. Require a hard fault for damaged metadata rather than the former synthetic interruption. Keep original valid legacy cases and byte preservation for damaged records. |
| `src/client/internal/link/commands_test.go` (`TestCommandStateAndUnknownAction`, `TestCommandRestartAndRecoveredReceipt`, `TestDeclineDecisions`). | Expect durable standing receipts for expired state and removal notices. Prepare earlier-run starts on reopen before testing replay. Use the malformed-receipt case to expect a store fault and no extra commit. Keep the existing outcome, expiry and write-count assertions for the other cases. |
| `src/client/internal/app/client_test.go` (`appPeer`, `TestAppRunStoreStatus`). | Speak revision 2 in the shared peer. Keep status 0 for the existing terminal conditions and assert the new recovery instruction. Keep the retryable error status and redaction assertion. |
| `src/client/cmd/lgtm-client/link_test.go` (`commandPeer.challenge`, `commandPeer.welcome`, `TestRunCommandExitStatus`, `TestRunCommandReportsAndReconnects`), and `src/client/clienttest/clienttest_test.go` (`TestClienttestSharesApp` peer). | Change the scripted peers' expected Hello and Welcome revision. Keep the existing exit, proof, delivery, redaction and app delegation assertions. Read the new process fixture for the added terminal-status assertion. |
| `src/linktest/outbox_test.go` (`authenticateScriptedLink`) and consumers of the old manual stream. | The orchestrator updates the manual Hello to revision 2 and supplies an appropriate stable pair wherever the test drives the current server. Keep an explicit revision 1 case for upgrade refusal. |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe. The dated foundation decisions D1 to D23, its rules 1 to 11 and its checks C1 to C13. The approved running-an-agent design dated 2026-11-27 supplies sprint 4 checks R1 to R14, the common link and filter rules, and the sprint 5 boundaries. The task file supplies assignment points 1 to 7. The reviewed contract plan and the round 2 fold supply the Client row, exact wire names and post-build proofs. The two design round 3 closing lists supply handoffs. The open-issue inputs are 1632 to 1635, 1670 and 1676. Decision numbers below follow the foundation table order.

| Source item | Workstream | Status |
|---|---|---|
| Assignment 1 and 2. Wire revision and process run. | WS-1, item 1. | Planned. |
| Assignment 3 and 4. Result watermark, interruption and sprint 3 data. | WS-1, item 2. | Planned. |
| Assignment 5. Local recovery stop. | WS-1, item 3. | Planned with Stack's executable and supervisor check. |
| Assignment 6. Folder facts and report order. | WS-2, items 4 to 6. | Planned. |
| Assignment 7. Shared filter. | WS-3, items 7 and 8. | Planned. |
| Foundation D1, D2, D4, D7, D16, D17 and D19. | WS-1 to WS-3. | Reuse generated Go contracts, real PostgreSQL in paired tests, key proof, outbound retry, pins and the existing process user. |
| Foundation D3, D5, D6, D8 to D15, D18 and D20 to D23. | WS-1 to WS-3. | Remain the baseline with server, website or later-wave owners. No client change here. |
| Foundation rules 1, 2 and 6 to 11. | WS-1 to WS-3. | Assign parts and receipts, use reviewed messages, keep durable ordering, sanitize diagnostics, verify both ends and preserve provenance. |
| Foundation rules 3 to 5. | WS-1, WS-2. | Server boundary. Paired checks observe the applied effects and leave ownership with the server. |
| Foundation C6 to C11. | WS-1 to WS-3. | Preserve enrollment, approval, reports, loaded reconnect, identity proof and ordered application. |
| Foundation C1 to C5, C12 and C13. | WS-1. | Stack and Server own these environment and database observations. Existing gates continue. |
| Sprint R1 to R4. | WS-2, WS-3. | Verify actual facts and unchanged safe identifiers. Projects owns operation effects and origin comparison. |
| Sprint R5 to R8. | WS-1, WS-2. | Prove once-only reads, durable report before receipt, expiry before read and arrival in committed order. Server owns command order and operation retention. |
| Sprint R9 to R11. | WS-1, WS-3. | Preserve the real old prefix and receipt, and join the paired run, result fence and upgrade checks. |
| Sprint R12 and R13. | WS-1. | Projects, Machines and Website own removal effects and discovery. Preserve client removal and no change to website revision. |
| Sprint R14. | WS-2. | Supply actual folder facts. Stack and Website prove both entry points and reload. |
| Shared newer rules. Run, operation, clocks, budgets, results, replay, outgoing text and sprint 5. | WS-1 to WS-3. | Preserve ordered publishing and exact correlation. Leave room for reports carrying a producing run and for future output separate from stream ownership. |
| Reviewed contract Client row, wire table and paired proof. | WS-1 to WS-3. | Plan the full client work and hand off the paired cases in the result. |
| Logic round 3 closing list. | WS-1, WS-2. | Send-permit proof is requested for R8 and R9. The remaining dispositions are in decisions 17 to 21. |
| Fit round 3 closing list. | WS-1 to WS-3. | Add the client byte and window edges. The remaining dispositions are in decisions 22 to 26. |
| Issues 1632 to 1635. | WS-1. | Keep outbound reconnect, reviewed certificate behavior and the combined test entry points. Size items by complexity. |
| Issues 1670 and 1676. | WS-1, WS-3. | Preserve the recorded recovery limits and the open sign-in gap. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, What this project is. | New client code is Go. Only generated Protocol Buffers values cross the program boundary. |
| `AGENTS.md`, Where things are. | Use the current client folders. Link and Folders build against the reviewed Contract files. The legacy tree remains a reference. |
| `AGENTS.md`, Credit copied code. | Use existing repository patterns, standard-library code and installed Git. Third-party source is unnecessary. |
| `AGENTS.md`, Write down what you find. | Cite the recorded issues and send new cross-team dependencies through the result under the assignment's write boundary. |
| `AGENTS.md`, Pull requests and the unrelated upstream. | Integration belongs to the orchestrator. The planner makes no remote change. |
| `AGENTS.md`, Writing style for documents. | Keep dated sources, relative paths and generic fixtures. Review the new files for privacy in addition to the tracked-file check. |
| Epic team paths and phase 4 of the build loop. | Keep production edits in Link, Folders and App with the narrow existing client fixture bridge. The test writer runs each assertion red before implementation and freezes it. Review risky mutations on a temporary build branch and discard them after proof. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1 | backend | `client/backend` | medium | It changes the handshake pair and the existing sender's waiting eligibility. | Yes | Reviewed Contract. |
| 2 | backend | `client/backend` | medium | It extends receipt validation and uses one startup transaction and the existing store lock. | Yes | 1. |
| 3 | backend | `client/backend` | medium | It validates the retained range and threads a typed stop through the existing entry point. | Yes | 1, 2 and the Stack status agreement. |
| 4 | backend | `client/backend` | medium | It reads local Git metadata and handles four fixed refusal outcomes. | Yes | Reviewed Contract and item 5's signature. |
| 5 | backend | `client/backend` | medium | It isolates one Git status call and its submodule reads with the same platform-neutral mechanism. | Yes | Item 4's checkout inputs. |
| 6 | backend | `client/backend` | medium | It reuses the command worker and sender patterns for a bounded report and a fixture interruption. | Yes | 2, 3, 4, 5, 7. |
| 7 | backend | `client/backend` | medium | It applies one matcher to known field classes and the ordered replay path. | Yes | 1, 3. |
| 8 | backend | `client/backend` | medium | It wraps the existing logger and loads bounded secret sources before startup output. | Yes | 3, 7. |

Load. OpenAI supplies the planner, test writers, builders and reviewers. Budget one planner, eight test writers, eight builders, four plan-review seats and four focused build reviewers, for 25 launches including this planner. The live remaining allowance is not available in this bounded assignment. The orchestrator compares the remaining 24 planned launches with the live allowance and can schedule a later batch with the same vendor. Keep each shared file with one active writer. Items 4 and 5 agree on types up front and can then build in parallel. Items 6 and 8 serialize their edits to `app/client.go`.

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Plan review | Pending. | The assignment ends at `attack` after the planner's self-check. The orchestrator owns the review rounds and their finding registers. |

</details>
