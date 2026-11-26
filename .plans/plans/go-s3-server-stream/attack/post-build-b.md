# REVISE

When the feed is behind, a committed removal can end an idle machine stream with a heartbeat timeout, violating the frozen `unknown_machine` rule.

The builder and reviewer were both OpenAI workers, so the requirement for review by another vendor was unmet. I reviewed items 4, 5, 8 and 9 and the Machines shutdown repair against `fcfa95c2..afed172f` on `sprint-0`. The code, frozen tests and disposable PostgreSQL runs came from `afed172f`. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s3-server-stream-b.md).

## Findings

### post-b 1 · MAJOR · The watchdog can report silence after removal commits

**Statement.** The frozen removal rule and the plan require the removed machine's current stream to end with `failed_precondition` and `unknown_machine`. [Frozen removal rule](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L164), [plan](../plan.md#L65).

**Evidence.** By reading, the feed calls `EndRemoved` when it consumes the removal reference. A frame or a wake can independently check durable identity in the steady loop. The watchdog cancels with the timeout directly, without checking durable identity. Once that cancellation sets the cause, a later removal signal cannot replace it. [Feed removal](../../../../src/server/internal/link/run.go#L239), [steady identity check](../../../../src/server/internal/link/stream.go#L31), [watchdog cancellation](../../../../src/server/internal/link/stream.go#L190), [terminal cause](../../../../src/server/internal/link/handshake.go#L96).

**Failure scenario.** Confirmed live over Connect. I waited for a heartbeat acknowledgement, held the feed consumer, and committed deletion of the machine and its link rows. An independent PostgreSQL read confirmed the machine was gone. I advanced the monotonic test clock to 31 seconds and fired the timer while the feed was held. The stream returned `deadline_exceeded` and “The machine did not answer in time.”

**Fix.** Recheck the exact durable stream and machine with a bounded read before publishing a silence timeout. Use the frozen refusal when the machine is already gone, and preserve any cause that was already selected. Add the held-feed case to the removal test.

### post-b 2 · MAJOR · An open Machines watch can miss the transition to Joining

**Statement.** The plan requires a watch and a fresh snapshot to use the same presence projection. The frozen website rule puts an approved machine with a current stream and incomplete readiness in Joining. [Plan item 9](../plan.md#L195), [frozen website states](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L148).

**Evidence.** By reading, admission records its Machines feed reference in the same transaction as the connection row. The page requires `Hub.Holds`, which first becomes true after `FinishWelcome`. Finishing Welcome changes the handle in memory and records no feed reference. The frozen watch test finishes its synthetic handle before committing the admission, which reverses the production order and misses the interval. [Admission feed write](../../../../src/server/internal/link/presence.go#L67), [presence read](../../../../src/server/internal/link/presence.go#L194), [Welcome completion](../../../../src/server/internal/link/handshake.go#L223), [live hold](../../../../src/server/internal/link/hub.go#L150), [fixture order](../../../../src/server/internal/machines/stream_projection_test.go#L141).

**Failure scenario.** Confirmed live with a signed-in Machines watch and a real machine stream. I held the stream after admission committed and let the watch consume its saved reference. It showed Offline. I released Welcome and waited for the exact handle to become live. A fresh snapshot showed Joining, and the feed head was still 4. The original watch needed a later write to learn the new presence.

**Fix.** When the stream becomes projectable, record a Machines saved reference under the machine lock with an exact-stream check. Add a watch test that consumes the admission reference before releasing the real Welcome and requires a Joining update without sending a report or heartbeat.

### post-b 3 · MINOR · The shutdown test accepts the wrong public sentence

**Statement.** The repair requires an ending Machines watch to receive `unavailable` with “The server is stopping.” Rule 8 requires a public error with a sentence the website can show. [Original shutdown finding](../../go-s3-server-machines/attack/post-build-d.md#L18), [foundation rule 8](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L369).

**Evidence.** By reading, production uses the fixed sentence. The frozen test checks that the reply came from the server and has code `unavailable`. Its sentence assertion only excludes the database outage text. The other named tests that assert the stopping sentence exercise the machine link, which uses a separate error. [Machines shutdown mapping](../../../../src/server/internal/machines/watch.go#L115), [frozen watch assertion](../../../../src/server/internal/app/machines_shutdown_test.go#L176), [link shutdown assertion](../../../../src/server/internal/link/stream_test.go#L641).

**Failure scenario.** Confirmed live with an overlay that changed the Machines watch message to “The server stopped this watch.” The frozen shutdown test exited zero. A scratch assertion for the exact sentence passed on the committed code and failed at line 180 with the altered message.

**Fix.** Compare the watch message with the exact stopping sentence while retaining the wire-origin, status-code and drain-order assertions.

## Post-build checks

- **Cleanup lanes and silence, P3.** Confirmed live. I kept six older machine rows locked and held the first retry. The unrelated first attempt ran within the five-second budget, and the signed-in Machines watch received Offline after about 97 milliseconds while all six locks were still held. The steady tests observed reader exit before terminal status in both protocols and cleanup after a blocked write. [Cleanup test](../../../../src/server/internal/link/run_test.go#L296), [silence test](../../../../src/server/internal/link/stream_test.go#L160), [blocked-write test](../../../../src/server/internal/link/stream_test.go#L693).
- **Peer addresses and saturated input, P11 and P12.** Confirmed live in process. Direct mode retained the transport peer and private-stack mode stored an empty address across the forwarding-header cases. With 16 queued frames, a valid heartbeat decoded before the pending enqueue extended the deadline and later got its acknowledgement. A report at that boundary did not extend the deadline. [Peer cases](../../../../src/server/internal/link/run_test.go#L438), [saturation cases](../../../../src/server/internal/link/stream_test.go#L621).
- **Event transactions and removal, P2 and P4.** Confirmed live. The commit barrier left no acknowledgement, effect or mark advance when PostgreSQL rolled back. Repeats acknowledged the stored mark without a second effect. In a separate scratch probe, the acknowledgement hook paused after an independent connection could see mark 1. Removal then canceled the held stream, which ended with `unknown_machine` and no event acknowledgement. The eight removal and waiting-token revocation cases passed across Connect and Google Remote Procedure Calls (gRPC). By reading, admission, heartbeat and event writes take the registry lock before writing link state. Disconnect takes the nonwaiting form of that lock. Approval locks the machine before projecting its receipt. Removal locks through the registry delete before deleting link rows, and revocation takes the token row first. [Event commit test](../../../../src/server/internal/link/stream_test.go#L241), [report lock](../../../../src/server/internal/link/inbox.go#L27), [presence writes](../../../../src/server/internal/link/presence.go#L42), [record writes](../../../../src/server/internal/machines/records.go#L40), [removal ordering test](../../../../src/server/internal/link/handshake_test.go#L851).
- **Uncertain admission and shutdown, P8 and the Machines repair.** Confirmed live. Losing the successful admission reply left a candidate for exact-stream cleanup, and a replacement survived retry of the old identifier. The lifecycle test retained a prior watchdog cause and canceled other stream contexts while cleanup was held. During the application drain, the pending approval committed with the follower still open. The tightened scratch assertion observed “The server is stopping.” on the watch. The follower closed before instance unlock, which preceded Store close. [Admission reply cases](../../../../src/server/internal/link/handshake_test.go#L1118), [lifecycle test](../../../../src/server/internal/link/run_test.go#L371), [shutdown order assertions](../../../../src/server/internal/app/machines_shutdown_test.go#L146).

The deployed proxy, real client outbox replay and startup-presence checks remain unverified at this pin. The application still wires the legacy Join service at `afed172f`. Item 10 owns the streaming startup and fixture wiring that those checks require. [Application wiring](../../../../src/server/internal/app/start.go#L230), [legacy Open](../../../../src/server/internal/link/stream.go#L18), [item 10](../plan.md#L241).

## Race test and mutation output

I ran `go test -race -count=1` through `deploy/test/with-postgres.sh` with the plan's named filters. The runs used a writable scratch cache and bounded Go build parallelism.

| Gate | Output |
|---|---|
| Item 4, six inbox tests | `ok` for `src/server/internal/link` in 27.828 seconds. |
| Item 5, eight runner and address tests | `ok` for `src/server/internal/link` in 19.415 seconds. |
| Item 8, ten steady-stream tests | `ok` for `src/server/internal/link` in 39.739 seconds. |
| Item 9, full Machines package including the frozen Part A tests | `ok` for `src/server/internal/machines` in 200.675 seconds. |
| Machines application shutdown regression | The frozen test printed `PASS` and the application package printed `ok` in 5.599 seconds. The exact-sentence scratch assertion also passed in 5.408 seconds. |
| Additional removal and uncertain-admission cases | The eight ordering cases and three uncertain-admission cases passed. The combined probe run reported the expected failure for post-b 1. |

| Planned mutation | Observed assertion |
|---|---|
| Replace the inbox's registry lock with a read. | Both orders failed at `oneline_test.go:57` because the second operation returned before the first released the lock. |
| Skip the successful subscription recovery wake. | `run_test.go:174` failed while waiting for the first live handle to wake on recovery. |
| Send the event-six acknowledgement before committing its transaction. | `stream_test.go:276` failed because event six had a wire response before commit was released. |
| Refresh the heartbeat deadline only after enqueue. | `stream_test.go:665` failed because the watchdog expired before the queued heartbeat could be drained. |
| Forget the link rows when revocation loses to approval. | `stream_records_test.go:111` failed because the approved machine's link record disappeared. |
| Bind the running follower to the application stop context. | `machines_shutdown_test.go:186` failed because the follower closed before PostgreSQL acknowledged the approval commit. |

## Grade

**7/10.**
