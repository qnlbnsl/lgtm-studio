# REVISE

The builder and reviewer were workers from the same vendor, so the rule requiring another vendor to review the work was unmet.

A requested scan can record and report done after its context is canceled before the inventory transaction starts.

This review covers items 4 through 6 at commit `658630e4` on 2026-11-26. The frozen tests entered at `8a2a6960` and were unchanged in the reviewed range. The test runs used a snapshot of the reviewed commit with normal filesystem ownership. The sandbox's remapped root owner trips the state-folder fixture.

## Findings

### post-c 1 | MAJOR | Plan items 5 and 6

The cancellation check is outside the durable append boundary. The reporter checks the context before invoking a callback that takes only bytes. The store then takes its lock and writes without seeing that context. The command handler records done when the reporter returns a successful append. The frozen cancellation test cancels during the scan probe, before the reporter's last check. [report.go:158](../../../../src/client/internal/link/report.go#L158), [report.go:203](../../../../src/client/internal/link/report.go#L203), [outbox.go:18](../../../../src/client/internal/link/outbox.go#L18), [commands.go:230](../../../../src/client/internal/link/commands.go#L230), [commands_test.go:793](../../../../src/client/internal/link/commands_test.go#L793), [plan.md:190](../plan.md#L190), [plan.md:262](../plan.md#L262)

A scratch probe canceled a requested scan just before the store opened its report transaction. The handler sent `COMMAND_OUTCOME_DONE`, one report was queued, and the next event number advanced to 2. A scan blocked waiting for the store can cross the same boundary during restart or shutdown.

Pass the scan context into the append and check it under the store lock, at the point where the store admits the write. Treat cancellation that wins before write admission as a refusal without an event number. Preserve the committed report and its successful outcome when cancellation follows a successful commit. Add a regression test that holds the store immediately before the write.

### post-c 2 | MAJOR | Plan item 5, rule 9 and check 7

Machine facts can carry an absolute repository path. The configuration accepts an absolute path, and the reporter preserves the original value when it cannot express the path relative to the user's home. A scratch probe confirmed that a short synthetic repository path outside the configured home was present in the encoded facts. The frozen test covers a workspace under home and an overlong workspace. [config.go:72](../../../../src/client/internal/config/config.go#L72), [report.go:94](../../../../src/client/internal/link/report.go#L94), [report.go:232](../../../../src/client/internal/link/report.go#L232), [report_test.go:153](../../../../src/client/internal/link/report_test.go#L153), [report_test.go:216](../../../../src/client/internal/link/report_test.go#L216)

Item 5 calls for the workspace, abbreviated when possible, and the frozen contract permits the field. The review's privacy requirement is stricter. A machine with a repository on another volume sends the full local directory to the server as a fact. Apply an explicit workspace disclosure rule. For example, omit absolute paths that cannot be abbreviated if abbreviated workspaces are permitted. The existing wire field can represent an omitted value. Add a test for an outside-home workspace. [plan.md:187](../plan.md#L187), [events.proto:71](../../../../src/contracts/proto/lgtm/link/v1/events.proto#L71), [design rule 9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370)

### post-c 3 | MINOR | Rule 9 and plan items 5 and 6

A failed report append returns a database error that can expose the local file path in its text. The store wraps the original error and the reporter passes it through. The command handler passes a scan error to its caller. A scratch probe injected a file-write error with a synthetic database path. The return value matched `ErrStoreFailed` and its error text contained the synthetic path. The probe observed no buffered-event log. [store.go:386](../../../../src/client/internal/link/store.go#L386), [report.go:203](../../../../src/client/internal/link/report.go#L203), [commands.go:252](../../../../src/client/internal/link/commands.go#L252), [design rule 9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370)

A low-level file-write failure can carry a state directory into diagnostic text if the caller displays the returned error. Preserve the typed store failure for teardown and return a fixed, path-free message across the reporting and command boundary. Test both the error text and the emitted log fields.

### post-c 4 | MINOR | Plan item 5 and check 7

The frozen tests accept a reporter that drops every installed version except Claude's. The inventory fixture gives only Claude a known version. A scratch mutation suppressed the versions of Codex, Gemini and Cursor, and the entire frozen link package still passed under the race detector. [report.go:133](../../../../src/client/internal/link/report.go#L133), [report_test.go:122](../../../../src/client/internal/link/report_test.go#L122), [report_test.go:195](../../../../src/client/internal/link/report_test.go#L195), [plan.md:187](../plan.md#L187)

The website could list an installed Codex, Gemini or Cursor with no version even when its probe had measured one. Give each installed harness a distinct version in the fixture, and retain a case that expects an absent version when the probe cannot read it.

## Scenarios marked unreviewed before the build

- **Busy scan expiry.** Confirmed live with `TestCommandBusySlotWritesNoReceipt`. A saved expiry produced a reply with zero writes. A new expired scan wrote one receipt and left the handler on the current stream. A different runnable scan requested a retry without saving a receipt. [round-4 register](round-4-findings.md#L3), [commands_test.go:268](../../../../src/client/internal/link/commands_test.go#L268), [plan.md:316](../plan.md#L316)
- **Handoff ceiling and a fresh stream after a gap.** Confirmed live with `TestReplayRejectsImpossibleMarks` and `TestReplayResumesAboveFreshWelcomeAfterGap`. The cursor accepted the handed-off number immediately, refused the next queued number, and kept that queued event on disk. Both Connect and gRPC Remote Procedure Calls resumed from the fresh welcome mark plus one. The pinned snapshot had no `TestSessionAcceptsImmediateAck` to repeat 200 times. The sole sender, retired reply handling, blocked send, installed journal and process stop diagnostics belong to items 7b and 8. [replay_test.go:118](../../../../src/client/internal/link/replay_test.go#L118), [replay_test.go:184](../../../../src/client/internal/link/replay_test.go#L184), [plan.md:312](../plan.md#L312), [plan.md:321](../plan.md#L321)
- **Removal notice and verified refusal.** Confirmed live at the component boundary with `TestCommandStateAndUnknownAction` and `TestCommandRemovalKeepsHeartbeatsUntilVerifiedRefusal`. A higher-revision notice applied waiting standing, used the normal done reply, and stayed active while that reply was held. On both protocols the fixture received a bare waiting heartbeat, observed an ordinary break and classified a refusal before a challenge as retryable. It then classified `unknown_machine` after a verified challenge as terminal. The fixture explicitly sent the heartbeat and opened the replacement streams. Automatic redial, heartbeat cadence and process exit remain part of item 7b and the installed check. [commands_test.go:695](../../../../src/client/internal/link/commands_test.go#L695), [commands_test.go:877](../../../../src/client/internal/link/commands_test.go#L877), [contract check 30](../../go-s0-contract/plan.md#L909)
- **Report strings.** Confirmed live with `TestReportInventoryAndLoad`. The encoded facts and inventory had valid Unicode Transformation Format, 8-bit (UTF-8), with no zero bytes and with the expected wire lengths. The inventory excluded the executable path and raw diagnostic field. [report_test.go:226](../../../../src/client/internal/link/report_test.go#L226), [plan.md:193](../plan.md#L193)

## Durability and disclosure boundaries

- **Command kills.** Confirmed live in 13 separate scratch child-process scenarios against the command handler. The scan cases covered both sides of the start, report and finish commits, work already underway, and the result callback. The restart cases covered both sides of the start and finish commits, and the result callback. Each kill was followed by reopening the same store and delivering the next attempt. Work remained eligible after a kill before the first durable start. A durable start without a finish returned interrupted. A durable finish replayed done. None of the retries repeated an observed scan or accepted a second restart. The frozen decline-kill and randomized receipt-kill tests also passed. The clock assumption and the accepted early-pruning exception remain as written in the frozen contract. [commands.go:179](../../../../src/client/internal/link/commands.go#L179), [commands.go:282](../../../../src/client/internal/link/commands.go#L282), [receipts.go:64](../../../../src/client/internal/link/receipts.go#L64), [commands.proto:18](../../../../src/contracts/proto/lgtm/link/v1/commands.proto#L18)
- **Server commit and report deletion.** By reading, the client only checks that a welcome mark is below the next local number and a live mark is at or below the current handoff ceiling. Both marks assert a server commit. The client cannot establish the commit independently, and a premature in-range mark from the authenticated server would delete the claimed prefix. The frozen contract requires the server to commit before acknowledging. The reviewed snapshot contained neither the real combined suite nor the session wiring. The real PostgreSQL check still needs to observe a lost connection before commit, a committed report with its acknowledgement lost, and survival of any uncommitted suffix. [replay.go:23](../../../../src/client/internal/link/replay.go#L23), [replay.go:49](../../../../src/client/internal/link/replay.go#L49), [outbox.go:76](../../../../src/client/internal/link/outbox.go#L76), [events.proto:7](../../../../src/contracts/proto/lgtm/link/v1/events.proto#L7), [plan.md:314](../plan.md#L314)
- **Reports and logs.** By reading, the reporter builds facts and inventory without accessing enrollment tokens or keys. The scanner extracts a full-line version match and passes a limited child environment. Command replies contain the identifier, attempt and outcome. The two direct log calls emit a fixed state-conflict sentence or a sequence number. Findings 2 and 3 cover the path exceptions in report and error text. The reviewed receipt code uses the existing private database and opens no new file handles. [report.go:94](../../../../src/client/internal/link/report.go#L94), [report.go:217](../../../../src/client/internal/link/report.go#L217), [scan.go:59](../../../../src/client/internal/harness/scan.go#L59), [scan.go:121](../../../../src/client/internal/harness/scan.go#L121), [commands.go:113](../../../../src/client/internal/link/commands.go#L113), [commands.go:306](../../../../src/client/internal/link/commands.go#L306), [store.go:236](../../../../src/client/internal/link/store.go#L236)

## Acceptance commands and mutation results

All three prescribed commands exited 0. The package was `github.com/qnlbnsl/lgtm-studio/src/client/internal/link` in each result.

| Command | Package output |
|---|---|
| `go test -race ./src/client/internal/link -run 'TestReplay'` | `ok`, 1.462 seconds. |
| `go test ./src/client/internal/link -run 'TestReport'` | `ok`, 0.089 seconds. |
| `go test -race ./src/client/internal/link -run 'TestCommand\|TestDecline\|TestFinishPrunesByAge'` | `ok`, 2.050 seconds. |

Each planned mutation below exited 1 on an assertion in the frozen tests. The runs used disposable overlays in the scratch snapshot and one uncached run per mutation.

| Mutation | Assertion |
|---|---|
| Let a live mark above the handoff ceiling reach the store. | `replay_test.go:136` accepted a mark for the unsent report. |
| Advance the cursor while leaving the ceiling at its pre-send value. | `replay_test.go:130` refused the immediate acknowledgement. |
| Resume from the prior handoff. | `replay_test.go:239` could not fetch the first report expected from the fresh welcome mark. |
| Write the sequence log before the append returns. | `report_test.go:299` saw an early log. |
| Put the harness executable path in its version field. | `report_test.go:212` saw local path data. |
| Start a scan before its start receipt commits. | `commands_test.go:530` saw work before commit returned. |
| Let a second scan reserve a receipt while the slot is busy. | `commands_test.go:325` saw no error requesting a retry. |
| Check expiry before replay. | `commands_test.go:223` saw expired instead of the saved done. |
| Omit the decline write. | `commands_test.go:1053` saw zero writes. |
| Mark a discarded scan done. | `commands_test.go:859` saw done instead of failed. |

## Grade

7 out of 10.
