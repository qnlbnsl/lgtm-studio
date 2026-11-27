# REVISE

An idle stream that was already replaced can receive the permanent `unknown_machine` refusal when removal commits before the replacement is published in the hub.

I reviewed commit `696858cc` on `sprint-0` against the two repairs from the previous review. The frozen regression tests entered the history at `9da17894` and were already present at the comparison base `3c054981`. The builder and reviewer both use OpenAI, so this review does not meet the separate-vendor condition. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s3-server-stream-d.md) · [Builder result](../../../../.foundry/go-rewrite/results/s3-stream-build-f.md) · [Previous review](post-build-b.md)

## Findings

### post-d 1 · MAJOR · The watchdog can give a displaced stream the removal refusal

The plan and the frozen link contract make admission commit the replacement point. The displaced stream ends with `aborted`. Removal ends the stream that was current at removal with `failed_precondition` and `unknown_machine`. [Plan](../plan.md#L64) · [Replacement contract](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L125) · [Removal contract](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L164)

By reading, admission publishes the replacement and cancels the old stream only after the database operation returns. When the watchdog finds the machine missing, it waits for that publication using the time remaining on its two-second identity check. If the wait expires, it discards the gate error and returns `ErrUnknownMachine`. That cause can win before the replacement cancels the old handle. [Admission publication](../../../../src/server/internal/link/hub.go#L108) · [Watchdog classification](../../../../src/server/internal/link/stream.go#L211) · [Public status](../../../../src/server/internal/link/handshake.go#L349)

Confirmed live on both Connect and Google Remote Procedure Call (gRPC). I paused the replacement after PostgreSQL committed it and before the hub could publish it. An independent connection saw the new generation. I committed removal while publication was held and fired the original stream's watchdog. With the barrier held across the two-second wait, the original stream received `failed_precondition` with `unknown_machine`. By reading, that reason stops retries in the pinned client. The frozen replacement-and-removal test releases publication before it checks the older stream, and the frozen watchdog test admits no replacement. [Client refusal](../../../../src/client/internal/link/conn.go#L205) · [Retry decision](../../../../src/client/internal/link/retry.go#L39) · [Frozen replacement test](../../../../src/server/internal/link/review_admission_test.go#L93) · [Frozen watchdog test](../../../../src/server/internal/link/review_watchdog_test.go#L40)

Preserve enough information from the committed admission and removal to distinguish the removed generation from any displaced one when the hub is late. A gate timeout alone cannot establish that the caller was current at removal. Add a barrier test that keeps publication held beyond the watchdog's wait and checks the older stream's wire status.

### post-d 2 · MAJOR · A rolled-back replacement can leave an open Machines watch Offline

Item 9 requires snapshots and live watch updates to project current presence. The frozen contract says an approved machine with a current stream and incomplete readiness is Joining. An admission that rolls back leaves the original stream current. [Plan](../plan.md#L195) · [Presence contract](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L148)

By reading, presence uses the same hub predicate as the live action gate. That predicate hides the old handle during every replacement attempt, even when the watch's database snapshot still selects that exact stream. The repair marks only candidates whose Welcome has yet to finish. When an admission rolls back, clearing the pending flag makes the original handle visible again without writing a Machines reference. [Presence lookup](../../../../src/server/internal/link/presence.go#L217) · [Watch bookkeeping and liveness](../../../../src/server/internal/link/hub.go#L193) · [Admission rollback](../../../../src/server/internal/link/hub.go#L108)

Confirmed live with a signed-in watch and an approved machine that had received Welcome. I held the registry row while a real replacement waited, then opened the watch. Its snapshot showed Offline. A PostgreSQL trigger rejected the replacement write. The fresh snapshot showed Joining after rollback, but the existing watch stayed Offline throughout the two-second observation window. The feed head did not advance. A later Machines write, such as the next successful heartbeat, can correct the displayed state. The frozen Joining test covers a successful first admission and leaves this sequence open. [Frozen Joining test](../../../../src/server/internal/machines/review_joining_watch_test.go#L72)

Use the watch's snapshot to select the stream and allow it to project the exact finished handle while the replacement remains uncommitted. Keep the action gate conservative. Add a watch regression covering the rolled-back replacement with no machine traffic after rollback.

### post-d 3 · MINOR · An identity lookup timeout is reported as machine silence

The plan maps a store failure in Open to `unavailable`. A heartbeat deadline uses `deadline_exceeded`. [Plan](../plan.md#L66)

By reading, `silenceCause` groups a successful identity check with every unrecognized database error. Both paths return the heartbeat timeout. The two-second context applies to the server's lookup, so its expiry says nothing about whether the server checked the current stream successfully. [Watchdog lookup](../../../../src/server/internal/link/stream.go#L211) · [Store read](../../../../src/server/internal/store/pool.go#L295)

Confirmed live over both protocols. I committed removal and held the feed consumer. An independent transaction then locked the registry table so the watchdog could not read it. Both streams received the fixed `deadline_exceeded` sentence instead of `unavailable` when the lookup timed out. No earlier cancellation had won.

Select the heartbeat timeout only after a successful identity read. Map lookup failures to the fixed database-unavailable error while preserving an earlier stream cause. Cover a blocked read in the frozen watchdog regression.

## Post-build scenarios and plan tests

The pinned plan and available register describe the unreviewed mechanisms in the item text and the register's next-review table. The checks below exercised the portions that intersect the repair. I used disposable PostgreSQL and separate Connect and gRPC clients where the frozen tests provide both. The proxy runs used the pinned Caddy image with the repository's route rules and disposable loopback ports. [Plan checks](../plan.md#L338) · [Review register](round-1-findings.md#L62)

| Scenario | Observed result |
|---|---|
| P2, admission, removal and the first Welcome write | Both protocols passed the removal orders and Welcome boundaries directly and through Caddy. A pre-entry stale Welcome was suppressed even after the replacement closed. An already-started Welcome was allowed to complete. The extended watchdog barrier exposed post-d 1. [Removal barrier](../../../../src/server/internal/link/review_admission_test.go#L30) · [Welcome barriers](../../../../src/server/internal/link/handshake_test.go#L966) |
| P3, silence, blocked writes and cleanup lanes | Through Caddy, both protocols delivered a readable silence status after the reader exited. Both delivered `unknown_machine` in the held-feed removal regression. The direct blocked-write and cleanup tests passed. An unrelated row cleared while other machine rows were locked, and retry kept the replacement's row. [Silence](../../../../src/server/internal/link/stream_test.go#L160) · [Blocked write](../../../../src/server/internal/link/stream_test.go#L693) · [Cleanup lanes](../../../../src/server/internal/link/run_test.go#L296) |
| P8, uncertain commit and restart | The lost-reply, retry, and shutdown tests passed. A replacement survived a stale cleanup. Application startup cleared stale presence while retaining machine state. Startup withheld readiness when a cleanup failed. [Admission reply](../../../../src/server/internal/link/handshake_test.go#L1118) · [Application scenarios](../../../../src/server/internal/app/link_stream_test.go#L212) |
| P11 and P12, observed address, revoked watch and saturated reader | Direct tests passed the private-stack and transport-peer address cases. A revoked watch received no later report. A valid heartbeat refreshed the deadline before entering the full queue. A report left the original deadline in effect. [Address policy](../../../../src/server/internal/link/run_test.go#L438) · [Revoked watch](../../../../src/server/internal/app/link_stream_test.go#L401) · [Reader saturation](../../../../src/server/internal/link/stream_test.go#L585) |
| Early oversized input and a failed Welcome flush | Both protocols passed the prefix refusal and failed-flush tests. The failed Welcome did not open the stream. [Prefix check](../../../../src/server/internal/transport/link_stream_test.go#L408) · [Welcome flush](../../../../src/server/internal/link/review_handshake_test.go#L46) |

The complete deployed stack, the long proxy soak, and the combined real-client outbox test were outside these runs. The `src/linktest/` directory is absent at this pin. [Deployed and combined checks](../plan.md#L342)

The race runs used `deploy/test/with-postgres.sh -- go test -race -count=1` with the plan filters or the full Machines package.

| Gate | Output |
|---|---|
| Items 5 and 8, all named tests and the selected link regressions | `ok` for `src/server/internal/link` in 69.847 seconds. |
| Item 9, the full frozen Machines package | `ok` for `src/server/internal/machines` in 150.045 seconds. |
| Frozen Joining and shutdown regressions | `ok` for `src/server/internal/machines` in 4.896 seconds and `src/server/internal/app` in 5.118 seconds. |
| Additional admission, startup, authorization and fixture checks | `ok` for `src/server/internal/link` in 5.719 seconds, `src/server/internal/transport` in 1.112 seconds, `src/server/internal/app` in 31.121 seconds, and `src/server/servertest` in 23.771 seconds. |
| P2 admission, removal and Welcome boundaries through Caddy | `ok` for `src/server/internal/link` in 49.187 seconds. |
| P3 silence and held-feed removal through Caddy | `ok` for `src/server/internal/link` in 10.339 seconds. |

## Mutation results

I applied each source substitution separately in the scratch checkout. The frozen assertions distinguished both repair reversions. A separate bad implementation dropped only the watchdog's wait for publication on a registry miss. The full frozen Link package accepted it and printed `ok` in 115.313 seconds. The extended probe for post-d 1 covers the missing ordering case and fails the pinned implementation as well.

| Mutation | Output |
|---|---|
| Skip the watchdog's identity check. | `review_watchdog_test.go:59` reported `deadline_exceeded` instead of `failed_precondition` on both protocols. |
| Skip the follow-up Machines reference after Welcome. | `review_joining_watch_test.go:115` reported that the original watch stayed Offline while a fresh snapshot showed Joining. |
| Skip the successful subscription-recovery wake. | `run_test.go:174` timed out waiting for the first live handle to wake on recovery. |
| Refresh the heartbeat deadline after enqueue. | `stream_test.go:665` reported that the watchdog expired before the writer drained the queued heartbeat. |
| Load presence in a new transaction during the watch snapshot. | `stream_projection_test.go:348` reported that the loader used a newer snapshot than the machine query. |

## Grade

**6 out of 10.** Both original test scenarios pass. The repair leaves a final status inconsistent with committed replacement order and a Machines watch that can retain an incorrect presence state.
