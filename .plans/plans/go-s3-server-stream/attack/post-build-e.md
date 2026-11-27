# REVISE

The server can end a displaced stream with `unavailable` even after a committed removal identifies the newer stream that replaced it.

I reviewed commit `6aeb25fe` against `522845ae` on `sprint-0`. The three frozen stream regressions entered at `fff5d8dd` and are present at both ends of the comparison. The plan calls its unreviewed changes “UNREVIEWED MECHANISM,” and the review register lists their locations under “Place for the next review.” I used those locations and the three repair scenarios from the previous review. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s3-server-stream-e.md) · [Review register](round-1-findings.md#L70) · [Previous review](post-build-d.md#L9) · [Builder result](../../../../.foundry/go-rewrite/results/s3-review-build-g.md#L13)

## Findings

### post-e 1 · MAJOR · A lost commit reply overrides the confirmed replacement order

The frozen contract makes admission commit the point at which the older stream is displaced and requires that stream to end with `aborted`. Plan item 2 uses `unavailable` to fence the older handle while the replacement commit outcome is unknown. The database can settle the ordering when the following removal commits and records which stream it deleted. [Replacement contract](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L125) · [Removal contract](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L164) · [Plan item 2](../plan.md#L99)

By reading, `Forget` records the exact stream that the removal transaction deleted. The follower marks the removal after commit. During a pending admission, the hub defers removal cancellation. When admission subsequently reports an unknown outcome, `Hub.Admit` cancels the older handle with `errDatabaseUnavailable` and clears it before the deferred removal check can act. That path never consults the recorded identity, which the new watchdog and frame paths already use. [Recording the removed stream](../../../../src/server/internal/link/presence.go#L252) · [Committed removal notification](../../../../src/server/internal/link/run.go#L247) · [Unknown-outcome cancellation](../../../../src/server/internal/link/hub.go#L113) · [Identity classification](../../../../src/server/internal/link/hub.go#L265)

Confirmed live over Connect and Google Remote Procedure Call (gRPC). I opened stream A and received its heartbeat acknowledgement. I used the existing commit-reply wrapper to hold stream B's successful PostgreSQL reply and verified B's stream identifier and higher generation on an independent connection. I committed removal, delivered the hub's removal notification, then dropped B's held reply. B returned `unavailable` without a Welcome. A also returned `unavailable` on both protocols, where the frozen replacement contract requires `aborted`. The frozen lost-reply test has no older live wire stream, and the frozen removal test releases a successful reply. [Commit-reply wrapper](../../../../src/server/internal/link/handshake_test.go#L1100) · [Frozen lost-reply test](../../../../src/server/internal/link/handshake_test.go#L1118) · [Frozen successful-reply test](../../../../src/server/internal/link/review_admission_test.go#L93)

When an observed removal commit supplies the selected stream identifier, use it to resolve the older handle's terminal cause before the unknown-admission fallback cancels it. Preserve `unavailable` while evidence of the outcome is missing. Keep a barrier test that combines the lost reply with removal and reads both terminal statuses.

### post-e 2 · MINOR · An approval can save Offline for a machine whose original stream remains live

Plan item 9 requires the approval reply to project presence inside the keyed write. An approved machine with a current stream and incomplete readiness is Joining. [Plan item 9](../plan.md#L191) · [Presence contract](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L148)

By reading, `ApproveClient` saves its projected record in the receipt, but presence only uses the database-selected handle during an admission when called from Watch. The approval therefore uses the action gate, which hides the original handle even while that approval holds the registry row and prevents the replacement from committing. The projection turns the hidden handle into Offline. [Approval receipt](../../../../src/server/internal/machines/records.go#L86) · [Presence lookup](../../../../src/server/internal/link/presence.go#L231) · [Action gate](../../../../src/server/internal/link/hub.go#L194) · [Projection](../../../../src/server/internal/machines/project.go#L18)

Confirmed live against the Machines service with a signed-in request. I held a keyed approval immediately before its presence read, started a replacement that waited for the machine row, and released approval. A database trigger then forced the replacement to roll back. The original handle was still live. The approval and the replay both showed `CLIENT_STATE_OFFLINE`, while Watch showed `CLIENT_STATE_JOINING`. The current website discards the approval reply, so this affects consumers of the returned record and its receipt. The frozen approval projection test has no simultaneous admission. [Website approval call](../../../../src/frontend/src/server/sync.ts#L124) · [Frozen approval and replay assertions](../../../../src/server/internal/machines/stream_projection_test.go#L415)

Use the same snapshot-selected presence rule for the approval projection. Keep the conservative action gate for dispatch eligibility. Add the rollback ordering to the keyed approval test and compare the first reply and replay with the continuing stream's projection.

### post-e 3 · MINOR · The tests miss a displaced frame blocked by replacement publication

The plan makes the database the durable stream fence and requires identity to win over frame rules. Its removal test checks removal and displacement separately. The new held-publication regression uses only the idle watchdog. [Plan fence](../plan.md#L64) · [Plan item 8](../plan.md#L182) · [Frozen watchdog regression](../../../../src/server/internal/link/review_outbox_test.go#L121)

Confirmed live over both protocols. With B's committed publication held and removal committed, I sent A a heartbeat and observed its identity lookup. The pinned code sent `aborted` while publication was still held. A one-file mutation restored the previous gate wait in `open` and left the watchdog repair in place. Both mutated requests remained pending until I released publication after the three-second observation window. [Frame identity check](../../../../src/server/internal/link/presence.go#L135) · [Open terminal classification](../../../../src/server/internal/link/handshake.go#L112)

The Link package accepted this mutation under the race detector. It printed `ok` in 225.978 seconds with the four unrelated command-send tests listed in the builder result excluded. A run limited to the six watchdog and removal tests also printed `ok` in 38.217 seconds. The frozen combined-order test releases publication before reading A's status, so it cannot observe the wait. [Frozen combined-order test](../../../../src/server/internal/link/review_admission_test.go#L101) · [Builder's command exclusions](../../../../.foundry/go-rewrite/results/s3-review-build-g.md#L84)

Keep the same publication barrier and drive a frame in a second test case. Assert wire `aborted` before releasing the barrier, with no acknowledgement.

## Post-build checks on the pinned code

Confirmed live with disposable PostgreSQL. The direct plan runs used `deploy/test/with-postgres.sh -- go test -race -count=1 -v -run` with filters for the named tests. The proxy runs placed the pinned Caddy image and the repository's routing rules in front of the test handler, with disposable loopback ports. [Plan test command](../plan.md#L56) · [Scenario register](../plan.md#L338)

| Scenario | Observed result |
| --- | --- |
| P2, admission, removal and the first Welcome write | Both protocols passed the removal orders and Welcome boundaries directly and through Caddy. An older Welcome held before the first write was suppressed. A Welcome whose write had entered the transport could finish. The original watchdog regression returned `aborted` before publication, and the independent heartbeat probe did the same. The waiting-token revocation, concurrent approval and atomic removal tests passed. [Admission order](../../../../src/server/internal/link/review_admission_test.go#L30) · [Welcome boundary](../../../../src/server/internal/link/handshake_test.go#L966) · [Machines removal tests](../../../../src/server/internal/machines/stream_records_test.go#L31) |
| P3, watchdog, cleanup and Machines watch | Both protocols passed the silence, idle removal and blocked registry read checks through Caddy. An ordinary silence returned `deadline_exceeded`, a removed current stream returned `unknown_machine`, and the blocked lookup returned the fixed `unavailable` response. The direct blocked-write test ended the reader. A fresh cleanup deleted its row and advanced the feed within the five-second bound while six other rows stayed locked. The signed-in rollback watch retained Joining without an extra feed row, and the live action gate stayed closed during the attempt. [Watchdog lookup test](../../../../src/server/internal/link/review_outbox_test.go#L184) · [Cleanup observation](../../../../src/server/internal/link/run_test.go#L296) · [Rollback watch](../../../../src/server/internal/machines/review_rollback_watch_test.go#L21) |
| P4, event replay across cuts | The combined real client and server regression completed its two cuts and durable mark and effect checks with the Go race detector enabled. It printed `PASS` in 46.81 seconds after I moved the test binary to a disposable container. The first attempt stopped at enrollment because the filesystem sandbox presents the root and temporary directory as owned by an untrusted user. The client refused that layout before it could connect. [Combined regression](../../../../src/linktest/outbox_test.go#L45) · [Client parent check](../../../../src/client/internal/state/folder.go#L166) |
| P7 and P8, startup and uncertain admission | Startup preserved the earlier keyed receipt and cleared stale presence. Forced partial cleanup withheld readiness. Shutdown, lost commit reply, exact cleanup and later replacement passed in their separate plan tests. Combining lost reply with committed removal exposed post-e 1. [Startup tests](../../../../src/server/internal/app/link_stream_test.go#L212) · [Lost reply](../../../../src/server/internal/link/handshake_test.go#L1118) |
| P10 and P11, proof, peer address and watch authorization | Caddy tests passed replayed and unusable registered-key refusals. The address checks passed directly and through Caddy for ordinary and forged forwarding headers. Private-stack mode returned an empty address. The direct application check confirmed a revoked watch received no later report. [Proof test](../../../../src/server/internal/link/handshake_test.go#L506) · [Address cases](../../../../src/server/internal/link/run_test.go#L438) · [Revoked watch](../../../../src/server/internal/app/link_stream_test.go#L401) |
| P12 and early oversized input | The full-queue test observed that a valid heartbeat extended the watchdog and a report left the original deadline in force. The first terminal cause won the staged race. Both protocols rejected an oversized prefix with its payload withheld. The first-write and terminal-write bounds passed. [Full-queue test](../../../../src/server/internal/link/stream_test.go#L585) · [Oversized prefix](../../../../src/server/internal/transport/link_stream_test.go#L408) |

The deployed stack, ten-minute proxy soak, prior-binary ledger refusal and installer redaction were outside these runs. The Caddy checks used the repository's route behavior with a test upstream. The direct cleanup backlog assertion observed the connection row and feed, and the signed-in Watch assertions used the rollback, liveness and authorization fixtures.

| Race test run | Output |
| --- | --- |
| Frozen repairs and direct removal, watch, transport and startup checks | `ok` for `src/server/internal/link` in 105.401 seconds, `src/server/internal/machines` in 54.232 seconds, `src/server/internal/transport` in 1.224 seconds, and `src/server/internal/app` in 48.913 seconds. |
| Additional named plan tests for the hub, registry, stream writer, feed, proof and application wiring | `ok` for `src/server/internal/link` in 88.409 seconds, `src/server/internal/machines` in 6.903 seconds, `src/server/internal/app` in 6.970 seconds, and `src/server/servertest` in 24.275 seconds. |
| Frozen notice and restart validation, waiting-machine readiness and machine key validation | `ok` for `src/server/internal/link` in 8.904 seconds. |
| Caddy admission, removal, Welcome and watchdog scenarios | `ok` for `src/server/internal/link` in 91.497 seconds. |
| Caddy proof and peer-address scenarios | `ok` for `src/server/internal/link` in 27.376 seconds. |

## Mutation checks

I applied each change separately through a Go source overlay in the scratch checkout. [Plan item 8 mutation](../plan.md#L189)

| Mutation | Observed result |
| --- | --- |
| Return `unknown_machine` for a displaced handle after a confirmed registry miss. | `review_outbox_test.go:176` reported `failed_precondition`, wanted `aborted`, on both protocols. |
| Hide the older live handle from Watch during a pending replacement. | `review_rollback_watch_test.go:115` reported Offline, wanted Joining. |
| Use the heartbeat deadline for an identity read error. | `review_outbox_test.go:223` reported `deadline_exceeded`, wanted `unavailable`, on both protocols. |
| Skip the identity fence for an ignored frame, as proposed in item 8. | `TestRemovalRefusesNextFrame` received `deadline_exceeded` where it expected `failed_precondition` for removal and `aborted` for replacement. [Removal and replacement assertions](../../../../src/server/internal/link/stream_test.go#L503) |

## Grade

**7 out of 10.** The three frozen regressions passed directly, and the two watchdog regressions passed through Caddy. The terminal decision still misses a committed replacement when the database reply is lost, and the approval receipt can preserve the wrong presence.
