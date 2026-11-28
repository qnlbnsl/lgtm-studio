# Post-build review 7c of the Server Link plan: item 7 and the repair of item 6

Task `postbuild-go-s4-server-link-7c`, nonce `424feb`, branch `sprint-0`, commits `eb2088b6~1..b61b9131`, reviewed in a scratch copy of the head commit.

The cross-vendor rule is met for this review. The test writers and the builders were Codex workers, and this review was done by a Claude model.

## Verdict

**SHIP.** No finding blocks the plan, and the repair of item 6 closes post-6 1 and post-6 2.

The top finding is post-7c 1: a valid 65,536-byte wrapper is refused when it is sent compressed, because the receive limit is checked against the compressed size on the wire, although the contract and the plan say it is measured after decompression.

## What I ran, and what it printed

All runs were on a disposable PostgreSQL database, with the 25-minute limit for the link package.

- `TestFolderReportPrecedesSettlement`, `TestRevisionTwoEventBudgetsAndAcks` and `TestLookAtFolderCommandBudgets` passed.
- The repair tests `TestSweepContinuesAfterTimeoutCallbackFailure`, `TestStagedResultsSettleInReplayOrder`, `TestStagedResultSurvivesRestartAndSweep`, `TestResultFenceKeepsAttemptAndCommandRules` and `TestLinkRemovalSettlesAndCallsParts` passed.
- The whole link package passed in 439 seconds.
- The packages `api` (with the four red-on-purpose `TestProjectRouter*` tests skipped), `machines`, `app` and `src/server/servertest` passed.
- The five item tests that matter most for races passed again with the race detector on.
- The plan's Verify list names `src/server/internal/servertest`, which does not exist. The package is `src/server/servertest`. This is a typing error in the plan, not in the code.

## Reproduction of the other reviewer's claim about gzip

The claim is **reproduced**.

- I built a valid Open request whose wrapper was exactly 65,536 bytes, with a small valid event and incompressible unknown-field padding.
- Sent without compression, the server accepted it and acknowledged through event 1.
- Sent with gzip, the same wrapper grew to 65,579 bytes on the wire. The server refused it with `invalid_argument` on both the Connect streaming protocol and the gRPC (remote procedure call) protocol.
- The cause is the check of the length prefix in `src/server/internal/transport/link_stream.go:60`, which compares the on-wire size, and the Connect read limit set at `src/server/internal/api/server.go:163`. Both see the compressed size. The comment at `server.go:31` and the contract text in `link.proto` say the limit applies after decompression.
- The impact is small. The copied client does not compress its requests, and real events stay near 49,152 bytes. See post-7c 1.

## Places that no reviewer read before the build

| Place | What I ran | Result |
|---|---|---|
| Folder report before settlement, with a fault before the mark update | `TestFolderReportPrecedesSettlement` | Passed. Mutation "skip the approval check" fails it. |
| Replay settles only the sender's stage | `TestStagedResultsSettleInReplayOrder` with a second machine | Passed. Removing the machine filter in `settleCoveredResults` now fails it. In review 6 the same mutation exited 0, so post-6 2 is closed. |
| Sweep goes on after one machine's callback fails | `TestSweepContinuesAfterTimeoutCallbackFailure` | Passed. Returning at the first failure, and skipping retention after a timeout error, each fail it. So post-6 1 is closed. |
| Folder queue claimed in order | `TestLookAtFolderCommandBudgets` | Passed. Removing the folder deferral and removing the `min` reset in `revisitCommandHead` each fail it. |

## Mutations of the plan, and what failed

| Mutation | Test result |
|---|---|
| Skip the approval check for a folder report | Failed in `TestFolderReportPrecedesSettlement`. |
| Remove the decoded size check in `applyReport` | Failed in `TestRevisionTwoEventBudgetsAndAcks`. |
| Remove the derived-name check in `validFolderFacts` | Failed in `TestRevisionTwoEventBudgetsAndAcks`. |
| Remove the contradiction check of a present remote with `remote_omitted` | Failed in `TestRevisionTwoEventBudgetsAndAcks`. |
| Allow a busy folder successor | Failed in `TestLookAtFolderCommandBudgets`. |
| Remove the `min` in `revisitCommandHead` | Failed in `TestLookAtFolderCommandBudgets`, after 50 seconds. |
| Settle another machine's stages during replay | Failed in `TestStagedResultsSettleInReplayOrder`. |
| Return at the first timeout callback error | Failed in the sweep test. |
| Skip retention after a timeout error | Failed in the sweep test. |
| Stop skipping a machine whose timeout batch already failed | **Exit 0. No test fails.** See post-7c 3. |

The mutation "use the command instead of the authenticated stream as the machine source" has no place to land in the link package, because an event carries no machine field and `stream.go:145` passes the machine of the authenticated stream. The mutation "acknowledge before commit" is covered by the pre-commit fault in `TestFolderReportPrecedesSettlement`, which expects no acknowledgement.

## Findings

### post-7c 1 — MINOR — The receive limit is measured on the compressed size, not after decompression

- **Statement.** A wrapper of exactly 65,536 bytes after decompression is refused when the sender uses gzip and the compressed form is larger than 65,536 bytes. The contract and the plan row of item 7 say the limit is measured after decompression.
- **Evidence.** `src/server/internal/transport/link_stream.go:60` rejects a frame by its length prefix. `src/server/internal/api/server.go:163` sets `connect.WithReadMaxBytes(linkReadLimit)`, and the comment at line 31 says "after decompression". My reproduction is in the section above. The frozen test only gzips the over-limit case: `src/server/internal/link/folders_v2_test.go:535-538` sends `outerOver` compressed, and its padding is a zeroed slice that compresses well, so the case cannot tell the two measurements apart.
- **Failure scenario.** A client or a proxy that compresses requests sends a legal edge-sized wrapper. The server answers `invalid_argument`, and the client treats this as a malformed stream and reconnects with the same event.
- **Fix.** Either measure the decoded wrapper and the envelope separately, or write into the contract and the plan that the limit also applies to the on-wire envelope length. Add a gzip case at 65,536 bytes with incompressible padding, which accepts or refuses according to the chosen rule. Foundation rule R3 (bounded input) is met either way.

### post-7c 2 — MINOR — The retention pass still stops at the first failed batch

- **Statement.** The timeout pass now continues to other machines after a failed batch, but the retention pass returns at the first error. One machine whose delete fails stops the pruning for every machine that sorts after it in that sweep.
- **Evidence.** `src/server/internal/link/outbox_sweep.go:108-110` returns `err` when `prune` is true. The repair test does not make a retention batch fail.
- **Failure scenario.** A locked or damaged row in one machine's old terminal commands makes each hourly sweep stop at that machine, and the older rows of the later machines are never pruned.
- **Fix.** Apply the same continue-and-report rule to the retention pass, with a test that fails one machine's delete. This is not required by the plan row, so I judge it MINOR.

### post-7c 3 — MINOR — A test accepts a sweep that retries a failed machine on every later page

- **Statement.** Removing the `failedTimeouts[ref.machineID]` skip leaves every test green, although the skip is the stated design of the repair.
- **Evidence.** `src/server/internal/link/outbox_sweep.go:90`. With the skip removed, `TestSweepContinuesAfterTimeoutCallbackFailure` exited 0. The test has three machines and fits in one page of 64 rows, so a later page never meets the failed machine again.
- **Failure scenario.** A machine with more than 64 due commands, whose callback always fails, is retried once per page in one sweep. The part sees the failing callback many times in one hour instead of once.
- **Fix.** Add a case with more than 64 due commands for the failing machine and one healthy machine after it. Require exactly one callback attempt for the failing machine in one sweep.

### post-7c 4 — MINOR — A folder report does not refresh the connection's last-heard time

- **Statement.** The facts and harness reports update `link_connections.last_heard_at`, and an applied folder report returns before that update.
- **Evidence.** `src/server/internal/link/inbox.go:132-136` returns `dispatchReport` directly, and the update is at lines 140-143. The registry touch at `inbox.go:86` still runs.
- **Failure scenario.** A machine that only sends folder reports and no heartbeat shows an older last-heard time on the Machines screen. Heartbeats normally keep it fresh, so the effect is cosmetic.
- **Fix.** Decide whether any applied event counts as "heard", and if so move the update before the switch. Add a one-line assertion to the folder tests either way.

## Plan rows I checked against the code and found done

- Fence and number are checked before the report (`inbox.go:47-66`), and a repeat ignores its payload.
- An over-budget, invalid or unsupported next report is consumed without a part call (`inbox.go:109-139`).
- A folder report reaches the part only for an approved machine, inside the event write and before the mark update.
- The binary Open codec in `src/server/internal/api/link_decode.go` measures received payload bytes and clears the report when over budget, so the number is still consumed. The test with redundant field encoding passes on both protocols.
- The command is sized with the widest attempt and the sampled database clock before insert (`outbox.go`), and `LookAtFolder` is rebuilt from declared fields only.
- The folder head and deferred sequence follow the scan pattern in `outbox_claim.go`.

## Post-build checks that still wait for the joint step with Client and Projects

Item 7 is the last item of the plan. These rows of the plan's table cannot be settled in the Link package alone:

1. Hold the first worker while a second alias queues, release before expiry: needs Projects aliases and the real client.
2. Hold the final replay acknowledgement, replace or remove the stream, send event 33: needs the real client's credit rule.
3. Race an applied report against timeout and removal in both lock orders, with waiting and wrong-machine reports, an unlisted origin and a denied browser call: needs Projects.
4. Lose diagnostic commit replies on both sides of commit, and fault Projects after its callback: needs Projects.
5. Synthetic credential markers, private state modes and both certificate pins during upgrade: needs Client.
6. Remove a flagged machine with a stage and 257 repositories, fault cleanup and retry: needs Projects.
7. The paired cases `TestSprintFourHistoryRecovery/RevisionOneRefusal`, `TestSprintFourPendingRun`, `TestSprintFourWaitingReplay`, `TestSprintFourReceiptReplay` and `TestSprintFourFolderSettlement` in `src/linktest/`. Files for them were still unfinished in the working tree, so I did not run them.
8. The red-on-purpose `TestProjectRouter*` tests, the stack compose check and the two frontend end-to-end cases stay red until Projects lands.

The row "stage three results, restart, repeat, replay" and the row "register two parts in startup order" are covered inside Link by tests that passed.

## Grade

**8 out of 10.** The code does what the plan rows say, every mutation of the plan except one gap in the sweep test is killed, and the repair of item 6 is real. Points are lost for the compressed-size mismatch, the test that cannot distinguish the retry of a failed machine, and the retention pass.
