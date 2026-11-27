---
topic: go-s4-client-folders
status: done
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# Inspect a machine's checkout and upgrade its link

## Overview

Sprint 4 lets the website inspect and save an existing Git checkout. The client reads local checkout facts without changing the repository or contacting its remote.

The upgrade keeps enrollment, buffered events and command receipts. A restarted client can send a saved report even if it died before answering. Unusable history stops the service with a recovery instruction.

One filter masks known secrets in outgoing display text and logs. The client refuses a folder with a secret in its identity. Both operating systems are tested with synthetic credentials. The macOS keychain gap stays recorded.

## Key decisions

Reconnects preserve the run boundary and saved result numbers. Unusable origins are withheld.

Full log → [decisions.md](decisions.md).

## Workstreams

### The client can reconnect with its saved history

The upgrade keeps receipts and buffered events. A waiting machine can replay them before approval.

### The client can inspect a checkout without changing it

The machine reports the resolved checkout, its branches, usable origin, first known commit and whether the working tree is clean. A real-link test interrupts the client between recording the report and finishing the command, then verifies that the server saves the folder once.

### Reports and logs pass the same filter

The executable tests capture masked display text, exact safe identifiers and a required unsafe old event that stays on disk and stops the client.

## Rollout order

1. Land the reviewed revision 2 contract. After the item gates and PostgreSQL paired suite pass, land both revision switches and matching fixtures in one commit through the integration pull request. (architect-r1 01)
2. Run the five sprint cases on that candidate. Retain the untouched sprint 3 fixture.
3. Reinstall against the upgraded server and verify that enrollment, buffer and receipts survive.
4. Check terminal recovery and requested restart on both operating systems. Add a real repository from each deployed website entry point.

---
<!-- ⛔ Implementation details and gate evidence follow. -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

Workstream identifiers use the prefix WS.

**WS-1 — The client can reconnect with its saved history**

The durable counter supplies both the process boundary and result watermark.

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

**Item 1. Send the revision and the process boundary.** Create `src/client/internal/link/run_identity.go` and `revision_two_session_test.go` in that folder. Change `conn.go` and `session.go` there. Update only the revision-dependent fixtures listed below. Start after Contract beside Folders. Items 2 and 3 follow on the integration branch. (architect-r1 01) This covers foundation decisions D7, D16 and D17, foundation rules 2, 6 and 7, foundation checks C6 and C8 to C11, and sprint checks R8, R9 and R11.

- Set `linkRevision = 2`. In `link.Run`, generate a version 7 universally unique identifier (UUID) with Go's cryptographic random source. Before any append or dial, capture `runIdentity{id string, firstEventNumber uint64}` from that identifier and `Store.Next()`. Link passes the pair to `openSession`, which writes `Hello.run_id` and `Hello.first_event_number`. Use a fresh nonce per attempt. Accept revision 2 after structural and proof checks and preserve both proof labels.
- Use `Welcome.last_event_number` as the replay cursor. A waiting client sends bare heartbeats and retained events below its run boundary. Fresh reports wait for approval. Preserve the single sender, cumulative acknowledgement and receive-time ceiling. Keep at most 32 event handoffs beyond the greater of the Welcome mark and last acknowledgement. Heartbeats and results can still send. Server Link sends the final replay acknowledgement before gated `SetState`.
- Retry `invalid_argument` and `failed_precondition` with `Lgtm-Link-Refusal: event_gap`. Preserve the final revision refusal after a verified Challenge. Item 3 handles incompatible Welcome and preserves malformed Welcome and unverified refusal behavior.
- **Tests written first.** In `revision_two_session_test.go`, write `TestRevisionTwoRunAnnouncement`. Seed events 1 to 3 and require first number 4. Reconnect after an acknowledgement or append and require the same pair with a fresh nonce. A restart uses a new version 7 identifier and actual next. Two no-event runs have different identifiers and equal next. A verified `invalid_argument` retries with the same pair. Keep the proof byte fixture.
- Write `TestRevisionTwoWaitingPrefix` there. Seed 33 events. Await 32 handoffs and a bare heartbeat while waiting. Acknowledge one and require the 33rd. Acknowledge the prefix, approve and require fresh reports. Repeat on empty history and a same-run reconnect mid-replay. Use the shared limits. The existing window test covers command and heartbeat traffic at capacity.
- **Risky mutations.** Generate the pair inside `attempt`, capture the floor instead of the next number, change either proof label, suppress waiting replay or change the window to 31 or 33. The matching test must fail on its assertion.
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/internal/link -run 'TestRevisionTwoRunAnnouncement|TestRevisionTwoWaitingPrefix|TestReviewGUnacknowledgedEventWindow|TestOpenProofFixture' -count=1` natively on each, then run `TestSprintFourWaitingReplay` and `TestSprintFourPendingRun`.

**Item 2. Save the first result watermark and accept old receipts.** Change `src/client/internal/link/store.go`, `receipts.go` and `commands.go`. Create `revision_two_receipts_test.go` and `testdata/client-v1.db` in that package. Update `receipts_test.go`, `conn.go` and `session.go`, plus the frozen standing and clock fixtures. This follows item 1 and precedes item 3. This covers foundation decisions D7 and D16, rules 6, 7 and 10, checks C9 to C11, and sprint checks R5 to R11.

- Keep the existing buckets and event bytes. Link writes version 2 receipt JavaScript Object Notation (JSON) with `version`, `expires_at`, `state`, `standing`, `last_event_number` and base64 `result` when finished. Link and the disjoint paired fixture read this format. A current started receipt has no result and a present null watermark. A recovered `interrupted` receipt has no result and an integer watermark. A finished receipt requires a recognized result and a present integer watermark. Integers run from zero to the durable greatest event number. Reject missing or invalid metadata with `ErrStoreDamaged`. Identify version 1 by its original format and field presence. Valid version 1 results and started receipts replay with zero. Preserve their bytes. Version 1 had no standing receipt.
- At startup, `Store.prepareRun() error` validates required receipt envelopes and seals prior version 2 starts to next minus one in one bbolt transaction. Skip the write when no start needs sealing. Preserve already sealed values, event bytes and valid version 1 records. Repeat the preparation before handler tests reuse a reopened store. Keep `OpenStore` read-only on an existing file and keep store helpers used by frozen tests. The Store returns `savedResult{outcome linkv1.CommandOutcome, lastEventNumber uint64}` to the command handler. Keep the running map and the existing uncertain-commit fence.
- Record next minus one with the first outcome under the store lock, even on an empty queue or failed append. Replay it on the current attempt. Write a `SetState` receipt before replying, including when waiting or expired. Keep its revision comparison, idempotent state application and standing retention. Keep recovered interruptions under started-receipt retention and preserve the restart drain budget.
- Keep the 23-hour offer and 24-hour receipt margin. Use verified Welcome server time plus monotonic time elapsed since receipt as the pruning lower bound. The command owner supplies it to receipt writes. Defer pruning without it, including before the first Welcome. Admission keeps the existing upper bound. (threat-r1 01)
- **Tests written first.** In `revision_two_receipts_test.go`, write `TestRevisionTwoReceiptWatermarks`. Save after an append, acknowledge the queue, append again and replay on new attempts over two restarts. Cover scan, restart, failed, expired, unsupported and expired standing outcomes. Trigger ordinary pruning and require the standing watermark and newest applied state revision. In both `TestDeclineClockOffsets` cases, reopen and write another receipt before retrying. Require the original outcome and watermark. The age test proves eventual ordinary pruning.
- Write `TestRevisionTwoReceiptMetadata` there. Kill after a durable start and a later append. Across two restarts with an intervening append, require the original final watermark and no repeated work. Exercise a torn seal, empty history, present zero, omitted number, null on a finished or interrupted receipt, number above the counter, unknown version and malformed legacy receipt. Require no result or rewrite for corrupt metadata.
- Write `TestRevisionTwoLegacyStore` there. Before freezing, use the actual sprint 3 client at `44c931d9` and a signed revision 1 peer to produce `testdata/client-v1.db` with synthetic enrollment, unacknowledged Facts, a finished receipt and an interrupted start. Record the producer and expected commands and numbers. Copy it before testing the executable path. Require original safe bytes, zero on later attempts and no rerun. Keep the old producer for installer proof.
- **Risky mutations.** Sample the watermark on resend, replace it with queue length or acknowledgement mark, default a missing revision 2 watermark to zero, reseal a started receipt after new appends, prune a standing receipt, or prune by local wall time. Each matching test must go red.
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/internal/link -run 'TestRevisionTwoReceipt|TestRevisionTwoLegacyStore|TestReviewFRestartHalfClosesBeforeCancel' -count=1` natively on each. Run the real receipt replay in rollout step 2.

**Item 3. Stop locally on an unusable replay.** Change `src/client/internal/link/{replay,session,retry,store}.go` and `src/client/internal/app/client.go`. Use the command entry point as a read-only anchor. Create `src/client/internal/link/revision_two_recovery_test.go` and `src/client/internal/app/revision_two_process_test.go`. Update the listed frozen assertions after items 1 and 2. This covers foundation decisions D16 and D17, rules 7 to 10, checks C8 to C11, sprint checks R9 to R11 and the contract's history recovery supplement.

- At store inspection, require `next` in the inclusive range 1 through 2^63 before receipt sealing or dial. Preserve an out-of-range store. Let the final legal event, 2^63 minus one, drain and keep sequence exhaustion for any later append. Link's store validator reads the existing counter written by `Store.appendContext`. (architect-r1 03, logic-r1 02)
- Before acknowledging a verified Welcome, reject a mark at or above next or below the retained floor minus one. An empty buffer's floor is next. In batches, check that the full required suffix is contiguous, decodable, within the event limit and safe under item 7. Preserve the outbox on fault. Skip validation of the confirmed prefix. Keep per-handoff validation and the receive-time acknowledgement ceiling.
- Link returns `RunRecoveryStop` without a retry error for required local history faults, an incompatible valid Welcome or verified revision refusal. App maps it through the Stack handoff and preserves it on cleanup failure. At fault detection, fence writes under the store lock before tearing down transport. Cancel workers and leave unfinished receipts untouched. Keep prior commits and the separate orderly restart drain. (logic-r1 03)
- Emit one fixed diagnostic. Name `client.db` and suggest restoring original history or removing and enrolling again with fresh state. For wrong revision, suggest installing a compatible client with saved state. Keep existing uncertain-commit, contention and cancellation paths.
- **Tests written first.** In `revision_two_recovery_test.go`, write `TestRevisionTwoTerminalHistory`. With intact enrollment, test a missing database, corrupt event or receipt, a suffix hole or empty suffix, and Welcome at next or below the retained floor. Require one instruction, no second dial or invented events, no results and preserved bytes. Exercise next at 2^63, a queued last legal event and next one above the limit with zero dials. Hold a scan across reconnect and release it after the write fence. Require identical rows, next and started receipt. A structurally valid incompatible Welcome stops once after proof. Invalid Challenge, malformed Welcome and remote `invalid_argument` keep their distinct behavior.
- Write `TestRevisionTwoExecutableStop` in `revision_two_process_test.go`. Run the actual command with intact enrollment and, separately, lost history or an invalid counter. Require the Stack handoff status, one instruction, no replacement database and no raw error text. A signed `Restart` yields the agreed restart exit and a durable result.
- **Risky mutations.** Acknowledge before validating the range, skip a missing number, retry an incompatible Welcome, accept the out-of-range counter, settle after the recovery fence, return a retryable status for terminal history, or emit two terminal instructions. The corresponding test must fail.
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/internal/link ./src/client/internal/app -run 'TestRevisionTwoTerminalHistory|TestRevisionTwoExecutableStop|TestRetryFailureExitsAndKeepsStore|TestAppRunStoreStatus' -count=1` on each. Stack must exercise the installed service independently.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A restarted client takes work ahead of its saved reports. | High | Compare the run pair across reconnects and apply replay before command delivery. |
| A result covers the wrong events after acknowledgement or a restart. | High | Check interrupted and standing watermarks across two restarts and another append. |
| A supervisor loops while durable history is missing. | High | Observe one instruction, the exit status and both supervisors past their restart interval. |

**Verify.** Use native Linux and macOS, disposable enrolled state and a signed peer. Run the paired tests with PostgreSQL. Keep the existing pinning, timing, removal, window and restart regressions.

**Freeze pin.** Review with `LGTM_LINK_BASE=fcfa96c3 bash scripts/check-go.sh`. Advance the pin after the reviewed contract lands.

**WS-2 — The client can inspect a checkout without changing it**

The Folders part reads checkout facts. Link owns expiry, duplicate delivery, append order and results. It runs the filesystem work off the stream owner so an inspection cannot hold up heartbeats or acknowledgements.

**Done when**

| Item | Criterion | Observed by |
|---|---|---|
| 4 | Discovery resolves aliases, reads branch, origin and first commit, and captures coherent inputs for status. | `TestInspectFolderMetadata`, `TestInspectFolderRefusals`, `TestInspectFolderOrigin`, `TestCheckoutSnapshot`. |
| 5 | Inspection reports clean or dirty without changing the checkout, executing checkout-controlled programs or using the network. | `TestInspectFolderFacts`, `TestInspectFolderReadOnly`, `TestInspectFolderNoSideEffects` and real-link checks R1 to R4. |
| 6 | A durable report precedes the first result, timeout frees the slot and oversized events consume no number. | `TestLookAtFolderLifecycle`, `TestLookAtFolderEventBudget`. |
| 9 | The fixture counts real starts and distinguishes the durable report number from the final process watermark. (seam 8) | `TestClienttestSharesApp`, `TestAfterFolderReport`, `TestSprintFourFolderSettlement`, `TestSprintFourReceiptReplay`. |

**Tasks**

| Item | Task | Anchor |
|---|---|---|
| 4 | Resolve the checkout and capture its metadata. | New `src/client/internal/folders/{checkout,origin}.go`. Use the bounded process pattern at `src/client/internal/harness/scan.go` (`Scan`) and the home input at `src/client/internal/config/config.go` (`LoadSettings`). |
| 5 | Read isolated status and assemble facts. | New `src/client/internal/folders/{inspect,status}.go` beside the metadata reader. |
| 6 | Wire the folder command and durable report into the link owner. | `src/client/internal/link/commands.go` (`startScan`, `finishCommand`), `outbox.go` (`Store.appendContext`), `session.go` (`liveSession.serve`) and new `folder_command.go`. |
| 9 | Expose real inspection starts and durable appends. (seam 8) | `src/client/clienttest/clienttest.go` (`Options`, `Run`), `src/client/internal/app/client.go` (`Options`, `Run`) and item 6's `folder_command.go`. |

**Item 4. Resolve the checkout and capture local metadata.** Create `src/client/internal/folders/{checkout,origin,checkout_test}.go`. Start with the reviewed Folders wire names. Gate item 5 on metadata. Independent Link work can proceed. This covers foundation rules 1, 2, 8 and 9 and sprint checks R1 to R4, R7 and R14. (architect-r1 08, critic-r1 07)

- Accept an absolute path, bare `~` or `~/...`. Reject other expansions, relative paths, invalid Unicode Transformation Format, 8-bit (UTF-8), zero bytes and typed paths over 4,096 bytes. Follow filesystem links before interpreting a following parent component. Resolve the physical working-tree root and use its basename. A resolved path over 8,192 bytes or a symlink loop gives `FOLDER_REFUSAL_PATH_NOT_USABLE`. An absent path, dangling link or non-directory gives `FOLDER_REFUSAL_NOT_A_FOLDER`. A readable directory outside a working tree, including a bare repository, gives `FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY`. A denied or unusable read gives `FOLDER_REFUSAL_NOT_READABLE`. Discard raw errors.
- Use one Git policy for discovery, refs, objects, status and children. Select a canonical absolute installed executable from the service search path. Skip a checkout-local executable and any executable or ancestor writable by the service or other unprivileged users. Use fixed arguments and a clean environment. Require inspection phases that run Git with original metadata to use only plumbing that cannot execute repository configuration. Suppress inherited configuration, optional locks, lazy fetch and all transports. Read original configuration as data without executing includes, hooks, conversion, filesystem monitors or credential helpers. The installed Git needs local ref and object plumbing, isolated machine-readable status with submodules suppressed, and support for the index and object formats in use. An unavailable program or capability gives the fixed not-readable refusal. (architect-r1 07, architect-r1 11, threat-r1 03)
- Name the main branch from the known local symbolic `refs/remotes/origin/HEAD`, else the checked-out branch, else `main`. Walk the exact remote-tracking target when remote head supplied the name and that target is available. Otherwise walk the selected local branch or the checked-out commit. Omit an unprovable root in shallow or unborn history. Choose the lexicographically first full object identifier when there are several roots. A detached checkout has no checked-out branch. (critic-r1 06)
- Read origin locally. Strip user information from permitted network Uniform Resource Locators (URLs) and Secure Shell (SSH) shorthand. Withhold account-relative, local, ambiguous or unsupported forms, queries, fragments, escaped paths, dot segments and results over 2,048 bytes after stripping. A configured but withheld origin sets `remote_omitted=true` with no `remote`. An unconfigured origin leaves `remote` absent and the flag false. Item 7 screens the remaining exact address.
- **UNREVIEWED MECHANISM (logic-r1 09, threat-r1 08).** Item 4 owns `checkoutPaths{root, gitDir, commonDir, indexPath string}` and writes `readCheckout(ctx context.Context, path, home string) (checkoutPaths, *linkv1.FolderFacts, linkv1.FolderRefusal)` and `captureStatus(ctx context.Context, checkout checkoutPaths) (statusSnapshot, error)`. Item 5 reads both. It fills `FolderFacts.clean` only after status succeeds. It gives item 5 `statusSnapshot{root, gitDir string, cleanup func()}`, where `gitDir` is the disposable view outside the checkout. Capture HEAD, the per-worktree index and its referenced shared index, local object access, ignore data and sparse selection when present. If the source index or configuration changes during capture, require a coherent view or refuse. Item 5 reads the snapshot and calls cleanup. This puts capture beside discovery and uses the ninth item for the paired fixture.
- Preserve inert status settings `core.filemode`, `core.ignorecase` and `core.symlinks` from local and worktree configuration. Use checkout `.gitignore` files, local `info/exclude` and the effective excludes file when safely readable. Keep defaults where the settings are absent. Refuse a required unreadable ignore source. Use the fixed not-readable refusal if required local objects or status-affecting settings cannot be read or reproduced safely. (critic-r1 05)
- **Tests written first.** In `checkout_test.go`, write `TestInspectFolderMetadata`, `TestInspectFolderRefusals`, `TestInspectFolderOrigin` and `TestCheckoutSnapshot`. Check the physical root from a link and a subdirectory, including a link followed by a parent component. Give the remote-tracking branch and checked-out branch divergent roots with no local main branch. Assert exact roots for remote head, local branch and detached fallback, plus shallow, unborn and multiple-root cases. Observe all four refusals, both edges of each path and origin limit and user-information removal. Exercise the private snapshot with a linked worktree, split index, missing required objects, cancellation and a changed index or configuration during capture. Expect a coherent view or fixed refusal, unchanged source bytes and private-view cleanup.
- **Risky mutations.** Prefer the checked-out branch to a known remote head, use the common index in a linked worktree, load executable configuration or send origin user information. Each must fail its fixture.
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/internal/folders -run 'TestInspectFolderMetadata|TestInspectFolderRefusals|TestInspectFolderOrigin|TestCheckoutSnapshot' -count=1` as an ordinary service user.

**Item 5. Read isolated status and return full folder facts.** Create `src/client/internal/folders/{inspect,status,status_test}.go` after item 4. It can run beside independent Link work. Item 6 consumes its reader. This covers foundation rules 8, 9 and 11 and sprint checks R1 to R4 and R7.

- Implement `workingTreeClean(ctx context.Context, checkout checkoutPaths) (bool, error)` in item 5. Use item 4's private view to check staged, unstaged and untracked changes. Verify tracked entries with flags that suppress comparisons in the private view or refuse. Treat a sparse entry as intentionally absent only if it is absent and confirmed outside the sparse selection. Check present entries, including their mode where relevant. Preserve the original index. (logic-r1 01)
- Suppress implicit child inspection in parent status. Compare tracked submodule targets explicitly and inspect populated children through their own private views under the same deadline. Count changed targets and dirty children. Keep index writes in the private view. Bound child output and clean up each view. (architect-r1 07, threat-r1 03)
- Export `func Inspect(ctx context.Context, path, home string) (*linkv1.FolderFacts, linkv1.FolderRefusal)`. Folders writes it here and Link item 6 calls it. Assemble local metadata and isolated status. Return not readable if status cannot be established. (architect-r1 08, critic-r1 07)
- **Tests written first.** In `status_test.go`, write `TestInspectFolderFacts`, `TestInspectFolderReadOnly` and `TestInspectFolderNoSideEffects`. Compare pristine, staged, unstaged and untracked states. Contrast executable-bit changes with file-mode checks enabled and disabled. Compare ignored and unignored files using each ignore source. Exercise a changed assume-unchanged file, an intentionally absent sparse entry and a present modified sparse entry. Require accurate facts or the fixed refusal where verification is unsupported. Test a changed submodule target and an unchanged target with a dirty child. Check unchanged index, refs, configuration and worktree. Use inert markers during discovery, history and status, including a populated child and a checkout-local executable on the service path. Require no marker or request. Exercise conversion-required refusal and an inert configuration control. (critic-r1 05, logic-r1 01)
- **Risky mutations.** Load original configuration for status, assume a suppressed tracked entry is clean, allow implicit submodule traversal or ignore untracked files. Each must fail a matching assertion.
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/internal/folders -run 'TestInspectFolderFacts|TestInspectFolderReadOnly|TestInspectFolderNoSideEffects' -count=1` with native Git as the ordinary service user. Record both Git versions and run R1 to R4 on the integrated candidate.

**Item 6. Append a folder report before answering its command.** Create `src/client/internal/link/folder_command.go` and `folder_command_test.go`. Change `src/client/internal/link/{commands,outbox,replay,session}.go` and `src/client/internal/app/client.go`. Follow items 2 to 5, 7 and item 8's app edits. (seam 8) This covers foundation rules 1, 2, 6 to 10, checks C9 and C11, and sprint checks R1 to R11 and R14.

- Decode the exact reviewed `Command_LookAtFolder` arm, validate `LookAtFolder.operation_id` and use the existing approval, clock and command receipt ordering. Check the complete inbound command's encoded size before reserving a receipt. A stale command is declined before any filesystem call. Confirm expiry again immediately before the worker starts. Use one folder worker slot alongside the existing scan slot. A duplicate uses the same work and newest attempt on the current stream. If the slot is busy for a different command, use the existing busy/retry pattern with no new receipt or read. The owner can still receive heartbeats, replays and restart. Give the whole read a five-second process-scoped budget from accepted start. At timeout, release the command slot even if the original stream dropped. Offer a fixed not-readable report, or record failed if the buffer definitely refuses the append. Cancel the read on process stop or accepted restart. (critic-r1 04, threat-r1 07)
- **UNREVIEWED MECHANISM (threat-r1 07).** The timer wrapper returns one completion and the read-only task has no store or stream access. Cancel and reap Git independently of the command slot. If a filesystem call cannot return on cancellation, discard its late result. Allow at most two unresolved reads, with only one active command. When both reads remain blocked, refuse new reads as not readable until one exits. Restart waits for the wrapper, so a stuck call cannot hold the exit open.
- Return checkout facts to the owner of the link. Send one `Event_FolderSeen` with the exact operation and command identifiers, and a `FolderSeen_Facts` or `FolderSeen_Refusal` arm. Use the common outgoing filter and an event timestamp from the client. Commit the report through the outbox and record done afterwards, for either arm. If the buffer cannot admit a report, record failed with the actual current watermark. Distinguish full capacity from uncertain commit. On uncertain commit, stop without a result or a second append. Keep command reports separate from coalescing Facts or inventory.
- While holding the existing store lock, size the complete encoded `Event` at its proposed number using `proto.Size`. Include its timestamp, oneof and nested fields. Use the 49,152-byte event limit and the outer 65,536-byte limits with `connect.WithReadMaxBytes` and `connect.WithSendMaxBytes`. Try the fixed path-not-usable refusal at the same number if facts exceed the event limit. If it also fails admission, save failed at the unchanged watermark. Keep the stored envelope unnumbered. Use the shared limit fixture in tests.
- **Tests written first.** In `folder_command_test.go`, write `TestLookAtFolderLifecycle`. Cover waiting and approved streams, facts, fixed refusal, a newer attempt, expiry and full buffer. Count reads and observe heartbeats and acknowledgements while a read is held. Require the report durable before the result. Stall the read past its budget and disconnect its stream. Require one fixed refusal or failed if the buffer is full, then another read while the retired one is still held. Hold that new read across a requested restart and require the normal durable restart result and exit. A retired read returning late cannot append. (critic-r1 04, threat-r1 07)
- Write `TestLookAtFolderEventBudget` there using production Protocol Buffers encoders. Exercise a numbered event at exactly 49,152 bytes and one byte over, including a number requiring more than one variable-length byte. Prove an oversized facts event leaves the original candidate number for the fixed refusal. Fill the outbox and check no number is consumed and the failed result has the last durable mark. Test inbound command size at the contract boundary and exercise the complete outer message cap with a real encoded message. Include a full 32-event window and a cumulative acknowledgement that allows exactly one further handoff.
- **Risky mutations.** Write the receipt first, record done on a full buffer, run an expired command, size an unnumbered event, or keep the inspection slot occupied after its deadline. Require the exact assertion in the lifecycle or budget fixture to fail.
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/internal/link -run 'TestLookAtFolderLifecycle|TestLookAtFolderEventBudget|TestReviewGUnacknowledgedEventWindow' -count=1` natively on each. Run paired acceptance after item 9.

**Item 9. Provide the in-process inspection and interruption fixture. (seam 8)** Change `src/client/clienttest/{clienttest,clienttest_test}.go`, `src/client/internal/app/client.go` and `src/client/internal/link/{session,folder_command,folder_command_test}.go` after items 6 and 8. This covers foundation C9 and C11 and sprint R3, R5 to R7 and R9 to R11.

- Add `clienttest.Options.OnFolderInspectionStart func(context.Context, string, string)` and `clienttest.Options.AfterFolderReport func(context.Context, uint64) error`. Forward both through identical fields on `app.Options` and `link.RunOptions`. The real worker calls the start observer with its read context, operation identifier and command identifier immediately before `folders.Inspect`, after the final expiry and cancellation checks. Call it once per actual worker start, including reads that cannot publish. Replays and pre-read declines never call it. Invoke the append callback outside the store and reporter locks after durable append and before receipt completion or report handoff. An error stops the run with the receipt still started and no result. Both fields default to nil. (seam 8)
- **Tests written first.** Extend `TestClienttestSharesApp` in `clienttest_test.go` to prove both callbacks pass through the app. In `folder_command_test.go`, extend `TestLookAtFolderLifecycle` to count a start with a full buffer and zero for expiry and replay. Write `TestAfterFolderReport` to observe the durable report and started receipt, interrupt, then reopen with only the append hook unset and check `interrupted` on the newest attempt. (seam 8) (architect-r1 10, logic-r1 06)
- **Paired inspection proof.** Hold one read across server restart and retry the same key in `TestSprintFourReceiptReplay/R5/ServerRestart`. Use the same observer for `TestSprintFourFolderSettlement/R3/SimultaneousAliases` and zero starts in `TestSprintFourFolderSettlement/R7/DisconnectBeforeDelivery`. (seam 8)
- **Paired interruption proof.** In `TestSprintFourReceiptReplay/R6/InterruptedAfterAppend`, release the armed startup inventory probe while `AfterFolderReport` is held. Wait for `link_event_buffered` with `sequence` above the folder number before returning the error. Read the started receipt and eight-byte big-endian `meta/next` from the closed store. Verify the numbered inventory and take `next - 1` as the watermark. (seam 8)
- Let fresh inventory with a distinct version append beyond that watermark after restart. The relay records the first recovered result's `last_event_number` before dropping it by command and `interrupted` outcome. Stop and join that client after both observations, then forward the second restart's result. Require the original watermark twice, one read, one applied report and unchanged success. (seam 8)
- **Risky mutations.** Count a replay as a read, call the append hook after receipt completion, or use the report number or new process counter as the recovered watermark. Require the affected case to fail. (seam 8)
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/clienttest ./src/client/internal/link -run 'TestClienttestSharesApp|TestLookAtFolderLifecycle|TestAfterFolderReport' -count=1`. Require each pass and the paired cases above. (seam 8)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An inspection mutates the checkout or executes checkout configuration. | High | Check inert sentinels and source bytes after private status on both hosts. |
| A reply overtakes a report or loses it during a crash. | High | Interrupt after durable append. Compare the watermark and saved folder after restart and settlement. |
| Oversized metadata consumes a number that will never be sent. | High | Test the exact numbered limit, the next byte and a full buffer. |

**Verify.** Use native Git on Linux and macOS with a dirty checkout, linked worktree and synthetic origin. Run the paired PostgreSQL folder tests. Stack and Website own R14.

**WS-3 — Reports and logs pass the same filter**

One filter protects logs, buffered reports and immutable replay.

**Done when**

| Item | Criterion | Observed by |
|---|---|---|
| 7 | Display text is masked before append, exact fields are omitted or refused, and unsafe retained events preserve their number and bytes when replay stops. | `TestOutgoingFilterText`, `TestOutgoingFilterReports`, `TestOutgoingFilterLegacyReplay`. |
| 8 | The actual startup log, process output, buffer and wire reports contain none of the planted secrets. | `TestOutgoingFilterExecutable`, `TestOutgoingFilterStructuredLog`, `TestOpenFrameDebugDoesNotLogConnectionToken`, and the paired capture in rollout step 2. |

**Tasks**

| Item | Task | Anchor |
|---|---|---|
| 7 | Filter new and replayed reports with one exact-value and display-text policy. | New `src/client/internal/link/filter.go` beside `report.go` (`reporter.publish`, `reportString`), `replay.go` (`encodeReport`, `decodeReport`, `replay.next`), and `conn.go` (`validateWelcome`). |
| 8 | Seed the filter and apply it to structured logs from process startup. | New `src/client/internal/app/outgoing.go` beside `client.go` (`Run`, `withDefaults`). Reuse the credential safeguards at `src/client/internal/enroll/credential.go` (`Load`, `decodeCredential`, `Credential.Format`). |

**Item 7. Filter outgoing reports, including their replay.** Create `src/client/internal/link/filter.go` and `filter_test.go`. Change `src/client/internal/link/{commands,outbox,report,replay,session}.go`. Land its common dispatcher and report gates before item 6 uses them. This follows item 3 for replay validation. Item 8 uses the filter. This covers foundation rules 7 to 10, checks C7, C10 and C11, sprint checks R1, R2 and R11, and the shared outgoing-filter rule.

- Provide `type OutgoingFilter`, `NewOutgoingFilter() *OutgoingFilter`, `AddSecrets(values ...string)`, `MaskText(text string) string` and `ContainsSecret(text string) bool`. Keep source strings out of diagnostics. Register the verified token in all forms the client holds before session output. Retain known values for the process. Provide `NewTextStream()` with `Push([]byte) string` and `Flush() string` for callers with pieces of one text. Hold the possible matching suffix across chunks, including overlapping matches and partial characters. Mask matched text with `[redacted]`.
- For new reports, normalize display text, mask it before truncation, and screen final field values at durable append. Use the same append check for direct publishes and pending flushes. Under the filter's registration lock, reclassify an unnumbered pending candidate with current secrets before allocating a number. Omit unsafe exact inventory identifiers and workspaces. Screen the final workspace after abbreviation and normalization. Keep safe exact strings and numeric load unchanged. A new folder report refuses unsafe paths, names, branches or commits. Check typed and expanded paths before inspection and resolved values at append. Omit unsafe sanitized origins with `remote_omitted=true`. (architect-r1 04, architect-r1 09, logic-r1 04, threat-r1 04)
- Screen `Command.id` at authenticated command admission, before receipt lookup or mutation for every action. Item 6 checks `LookAtFolder.operation_id` before folder work. Linearize the exact identifier check with admission and recheck at the shared result handoff. Item 6 rechecks both folder identifiers at append if filter knowledge changed. Use the existing invalid-frame path with a fixed diagnostic. Preserve required authentication fields in their existing protocol roles. The server writes the inbound identifiers, Link reads and guards them, and the server reads admitted report and result identifiers. (architect-r1 05, critic-r1 02, logic-r1 08, threat-r1 05)
- Check decoded queued fields during replay preflight and handoff, including text escaped in storage. Preserve safe payloads byte for byte. Stop on a required unsafe or unclassifiable report and preserve its number and bytes. Link writes the filter API. App uses it for logs and registers local credentials.
- **Tests written first.** In `filter_test.go`, write `TestOutgoingFilterText`. Register synthetic values with overlapping prefixes and characters outside the basic Latin alphabet. Cut a value across multiple pieces and a multibyte character across writes. Match the exact masked final text, confirm safe surrounding text and exercise a replacement while a suffix is pending.
- Write `TestOutgoingFilterReports` there for a synthetic private key form and token in Facts, inventory, and every folder exact field. Check unchanged safe exact values, masked text, omitted workspace, withheld origin and the fixed folder refusal. Include a credential embedded in a long value across its truncation boundary. Exercise a workspace with a zero byte separating the parts of a synthetic secret and a safe short workspace. On a full outbox, hold a pending inventory snapshot, learn its planted token in a new verified Welcome, and release one slot. Require a masked or omitted replacement in durable bytes and on the wire, and continued replay. Use a signed peer to send unsafe identifiers in standing, ordinary, unsupported and replayed commands. In a separate control, register the value after a result is queued. Assert fixed diagnostics and no unsafe identifier written after registration or sent on the wire.
- Write `TestOutgoingFilterLegacyReplay` there. Seed a known-safe and a known-unsafe version 1 event with synthetic values in display and exact fields. Across approved and waiting replay, require the safe legacy payload and matching number. Require the unsafe one to remain on disk with its original bytes and number while the client emits one recovery instruction and terminates.
- **Risky mutations.** Filter after truncation, mask a path in place, match each chunk independently, register the connection token after first use, bypass final screening on pending flush or queued results, or bypass screening for legacy data. The corresponding tests must fail.
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/internal/link -run 'TestOutgoingFilterText|TestOutgoingFilterReports|TestOutgoingFilterLegacyReplay' -count=1` natively on both. Confirm the actual executable path with item 8 and retain the frame-debug guard.

**Item 8. Filter logs from process startup.** Create `src/client/internal/app/outgoing.go` and `outgoing_test.go`. Change `src/client/internal/app/client.go` after items 3 and 7. Gate App on startup, Facts and inventory. Run the folder case after integration with item 6. (logic-r1 07) This covers foundation decision D19, rules 8 to 10, checks C6, C7 and C10, and the shared outgoing-filter rule.

- Create the shared filter before the `slog.Handler`. Use fixed diagnostics until secrets are known. Screen message, structured keys and values, groups and attached attributes. Keep numbers numeric. Before `client_starting`, read the enrollment key from an owned private regular-file descriptor with a size bound and no symlink follow. Recheck its decoded public half against enrollment. Stop if this fails. Register the file, block body, decoded bytes and other held forms. Keep the signer and its public formatter.
- Load readable local Claude Code sign-in values before dynamic startup output. Use `.credentials.json` in `CLAUDE_CONFIG_DIR` or `.claude/.credentials.json` under the service home. Recheck the bounded regular-file contents before dynamic log encoding, report admission, replay and result handoff. Read replacements too. Serialize refresh and classification with secret registration. Retain prior values for the process lifetime. Keep issue 1670's macOS keychain exception and leave the agent-start hook to sprint 5. App owns file reads and registers the credential strings in the shared Link filter. (threat-r1 02)
- Screen the enrollment argument before diagnostics. Discard raw peer and Git errors.
- **Tests written first.** In `outgoing_test.go`, write `TestOutgoingFilterExecutable`. Start the real client with a signed peer and a sign-in secret in a disposable workspace. Use peer token `907.806.705` and probe output `907.806.705 (Claude Code)`. An unfiltered control must reach the parser. Capture both output streams, durable reports and frames. Require startup, Facts and inventory, masked paths, omitted workspace, safe surrounding display text and no known raw or encoded secret. Rotate the file after approved Welcome and check fresh reports and logs while the process stays up. Required history with the rotated secret stops with bytes preserved. Item 6 adds the integrated folder check. (logic-r1 07, threat-r1 02)
- Write `TestOutgoingFilterStructuredLog` there. Require decoded JSON with masked nested, attached and lazy attributes, a safe message and an unmodified numeric attribute. Require no planted raw value.
- **Risky mutations.** Emit startup before loading sign-in values, filter only the message, bypass attached attributes, omit credential refresh, or give the logger and Link different instances. Require observable leakage of a synthetic value or a failed omission assertion.
- **Acceptance on Linux and macOS.** Run `go test -v -race ./src/client/internal/app ./src/client/internal/link -run 'TestOutgoingFilterExecutable|TestOutgoingFilterStructuredLog|TestAppLogRedactsSecrets|TestOpenFrameDebugDoesNotLogConnectionToken' -count=1` natively on each. Use an explicit disposable sign-in file on both hosts.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Filtering silently changes an identifier used to locate a checkout. | High | Observe exact safe keys, refusal and omission on the wire. |
| Startup or replay leaks a previously known credential. | High | Capture executable output and reports. Require a stop before unsafe replay. |
| The client cannot read the Claude Code sign-in from the macOS keychain. | High | Use readable synthetic credentials on both hosts. Keep issue 1670 open. |

**Verify.** On both native platforms, use synthetic secrets and capture process output, buffer and frames. Include stored facts and replies in the paired check.

**Sprint 3 frozen test changes**

Freeze the listed changes before implementation. Keep the proof, trailer, expiry and retry controls. Require explicit passes for the named new tests and zero skips on both hosts. Close the client gate with `go test -v -race ./src/client/...` on both. (critic-r1 08)

| Written test or shared fixture | Revision 2 change |
|---|---|
| `src/client/internal/link/conn_test.go` (`goodWelcome`, `open`, `TestOpenProofFixture`, `TestOpenExpiryAndRefusal`). | Use revision 2 for successful Welcome and Hello. Supply a fixed valid run pair when driving `openSession` directly. Keep signed bytes. Test revision 1 as incompatible alongside zero and an unsupported high revision. |
| `src/client/internal/link/conn_validation_test.go` (`TestOpenRejectsMalformedWelcomeBeforeRevisionMismatch`). | Give the malformed Welcome the now-incompatible revision 1 to keep the precedence assertion. |
| `src/client/internal/link/retry_test.go` (`wireLeg.hello` and direct `openSession` fixtures), and the direct `openSession` calls in `commands_test.go` and `replay_test.go`. | Switch peer acceptance to revision 2 and supply the fixture run pair. Keep the existing retry, gap, proof and refusal assertions. |
| `src/client/internal/link/session_test.go` (`runWelcome`, `TestRetryReportsOnApprovalAndOutage`, `TestRetryFailureExitsAndKeepsStore`). | Update the Welcome fixture. Require the saved event while waiting and acknowledge it before approval. Check the new terminal outcome and actionable diagnostic on required-history cases. For the live impossible acknowledgement case, send a mark above the recorded handoff ceiling without competing with newly allowed waiting replay. Keep the retryable commit and clean cancellation assertions. |
| `src/client/internal/link/replay_test.go` (`TestReplayRejectsDamagedPayload`). | Require rejection at initial replay validation before acknowledgement or event handoff. Keep the byte-preservation assertion. |
| `src/client/internal/link/receipts_test.go` (`TestBeginWritesStartedBeforeReturn`, `TestStartedFromEarlierRunIsInterrupted`). | Require the version 2 started format. Use startup preparation to observe the stable interrupted watermark. Require a hard fault for damaged metadata rather than the former synthetic interruption. Keep original valid legacy cases and byte preservation for damaged records. |
| `src/client/internal/link/commands_test.go` (`TestCommandStateAndUnknownAction`, `TestCommandRestartAndRecoveredReceipt`, `TestDeclineDecisions`). | Expect durable standing receipts for expired state and removal notices. Prepare earlier-run starts on reopen before testing replay. Use the malformed-receipt case to expect a store fault and no extra commit. Keep the existing outcome, expiry and write-count assertions for the other cases. |
| `src/client/internal/app/client_test.go` (`appPeer`, `TestAppRunStoreStatus`). | Speak revision 2 in the shared peer. Keep the baseline terminal status and assert the new recovery instruction. Keep the retryable error status and redaction assertion. |
| `src/client/cmd/lgtm-client/link_test.go` (`commandPeer.challenge`, `commandPeer.welcome`, `TestRunCommandExitStatus`, `TestRunCommandReportsAndReconnects`), and `src/client/clienttest/clienttest_test.go` (`TestClienttestSharesApp` peer). | Change the scripted peers' expected Hello and Welcome revision. Keep the existing exit, proof, delivery, redaction and app delegation assertions. Read the new process fixture for the added terminal-status assertion. |
| `src/client/internal/link/commands_test.go` (`TestCommandBusySlotWritesNoReceipt`). | Prepare the earlier start before counting writes. Require zero writes for the three ordinary replays, one for the first expired standing notice and zero for its newer-attempt replay. Require two cumulative writes after the new expiry and the same two after the distinct busy command. Keep the retry, outcome and scan-count controls. (critic-r1 01) |
| `src/client/internal/link/commands_test.go` (`TestDeclineClockOffsets`) and `receipts_test.go` (`TestFinishPrunesByAge`). | After reopen, both fast-clock cases keep the first outcome and watermark under authenticated time. Drive the age-boundary unit test from the trusted sample and prove no prune without it. Preserve equality at the 24-hour boundary and the old started receipt. (threat-r1 01) |
| `src/linktest/command_fixture_test.go` (`savedCommandReceipt`, `requireReceipt`) and `src/linktest/commands_test.go` (`testCompletedScanReplay`). | Read all six version 2 fields and distinguish present zero from missing. Edit only `expires_at` in the age fixture. Keep version 1 and zero replay on the genuine legacy fixture. (seam 8) (architect-r1 02) |
| `src/linktest/outbox_test.go` (`authenticateScriptedLink`, `TestLinkOutboxReplayAfterServerFailure`) and `reserved_result_test.go` (`TestLinkReservedCommandResult`). | The orchestrator supplies revision 2 and a stable canonical version 7 run identifier per scripted process. Start at `current+1` for replay verification and 1 for the fresh reserved-result machine. Preserve proof, gap, repeat, unknown-field and reserved-field controls. Add the separate signed revision 1 refusal from Server Link's handoff. (seam 6) |

</details>

## Post-build checks

- **Paired cutover.** Run `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -count=1`. Require the inherited and five Sprint 4 cases with zero skips. Stack owns the independent runner freeze and R14 checklist. (seam 2) (seam 3) R11 runs the old binary for refusal. Stack separately proves retained-state upgrade. The runner uses `lgtm-client join <address> <token>` and `lgtm-client run` with `LGTM_CLIENT_STATE_DIR` ending in `lgtm-client`. The installer uses `lgtm-client join <address> -` and standard input.
- **Folder settlement and retries.** Hold one read across disconnect, then send a newer attempt in the same run. Observe one start, one applied report and the current-attempt result. Use item 9 for interruption. Separately fault `servertest.Options.LinkSeams.BeforeEventCommit func(context.Context, string, uint64) error` by machine and number. Require unchanged folder, terminal result and mark, no acknowledgement and one save on replay. Use `BeforeEventAck` for separate loss after commit. (seam 8) (architect-r1 06, critic-r1 03, logic-r1 05, threat-r1 06)
- **Native read-only inspection.** Client and Stack run as the installed service user and record the Git version on Linux and macOS. Compare linked-worktree, split-index, populated-child and missing-local-object cases. Capture accurate facts or the fixed refusal, zero markers or remote requests, unchanged source metadata and cleanup after success, refusal and cancellation.
- **Executable filter and authorization.** After items 6 to 9, Client, Projects and the orchestrator inspect a branch and origin with rotated synthetic secrets. Capture logs, queue, frames and server and website records. Use separate branch and origin inputs. Require a fixed branch refusal, origin omission, one matching safe operation and no effects from waiting, obsolete or wrong-machine reports. Run both `join` and `run` through an early state error. Require fixed diagnostics without planted path text. Website checks an unauthenticated and unlisted-origin request at its own gate.
- **History, upgrade and supervisor.** Stack uses the signed legacy refusal and in-place upgrade. Before dial, inject an out-of-range local counter and require zero admissions. After Welcome, inject an invalid required payload and require an unchanged mark. Require no Projects effect, preserved disputed bytes, one instruction and a stopped service across the restart interval. With a valid upper boundary, require the final legal event to drain, no wrap and the existing exhaustion outcome. On a valid restart require a real replacement process.

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe. The dated foundation decisions D1 to D23, its rules 1 to 11 and its checks C1 to C13. The approved running-an-agent design dated 2026-11-27 supplies sprint 4 checks R1 to R14, the common link and filter rules, and the sprint 5 boundaries. The task file supplies assignment points 1 to 7. The reviewed contract plan and the round 2 fold supply the Client row, exact wire names and post-build proofs. The two design round 3 closing lists supply handoffs. The open-issue inputs are 1632 to 1635, 1670 and 1676. Decision numbers below follow the foundation table order.

| Source item | Workstream | Status |
|---|---|---|
| Assignment 1 and 2. Wire revision and process run. | WS-1, item 1. | Planned. |
| Assignment 3 and 4. Result watermark, interruption and sprint 3 data. | WS-1, item 2. | Planned. |
| Assignment 5. Local recovery stop. | WS-1, item 3. | Planned with Stack's executable and supervisor check. |
| Assignment 6. Folder facts and report order. | WS-2, items 4 to 6 and 9. | Planned. |
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
| Shared newer rules. Run, operation, clocks, budgets, results, replay, outgoing text and sprint 5. | WS-1 to WS-3. | Preserve ordered publishing and exact correlation. Leave room for reports carrying a producing run and for future output separate from stream ownership. Restoring the server database to an earlier state and indistinguishable history replacement stay outside wave 1 under issue 1676. |
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
| 3 | backend | `client/backend` | medium | It validates the retained range and threads a typed stop through the existing entry point. | Yes | 1 and 2. |
| 4 | backend | `client/backend` | medium | It captures checkout metadata and a coherent private status input. | Yes | Reviewed Contract. |
| 5 | backend | `client/backend` | medium | It uses the private view to read ordinary and populated-child status. | Yes | 4. |
| 6 | backend | `client/backend` | medium | It reuses the command worker and sender for bounded inspection and durable report. | Yes | 2, 3, 4, 5, 7 and the app edits from 8. (seam 8) |
| 7 | backend | `client/backend` | medium | It applies one matcher to known field classes and the ordered replay path. | Yes | 1, 3. |
| 8 | backend | `client/backend` | medium | It wraps the logger and refreshes readable secrets for output. | Yes | 3, 7. |
| 9 | backend | `client/backend` | low | It forwards two test callbacks at the real worker and append boundaries. (seam 8) | Yes | 6 and 8. |

Load. Budget 28 OpenAI launches for one author, one fold, nine test writers, nine builders and eight reviewers. Six went to completed roles. The orchestrator checks the remaining 22 at dispatch. Keep one writer per shared file.

</details>

<details><summary><b>Attack log (gate 7)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Plan review round 1 | 36 findings from four reviewers. | The [folded register](attack/round-1-findings.md) records outcomes and patches. |
| Sprint 4 alignment | Client fixture disagreement. | Item 9 publishes both observations. (seam 8) |

</details>
