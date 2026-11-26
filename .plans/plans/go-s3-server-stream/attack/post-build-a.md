# REVISE

A removal can give the already displaced stream the final `unknown_machine` refusal when it commits after the replacement and before the hub publishes that replacement.

The builder and reviewer were both OpenAI workers, so the requirement for review by another vendor was not met. I independently reviewed items 2, 3, 6 and 7 at `fcfa95c2` against `0d900115`. The runtime evidence uses archived copies of `fcfa95c2`. One fixture rerun also includes the builder's three-line revocation fixture repair.

## Findings

### post-a 1 · MAJOR · Plan and D7

The hub uses publication to decide which stream to end for removal, while the frozen contract decides which stream is current at the database commit. `Hub.Admit` runs the database operation before it updates `m.current`. During that gap, `Hub.Holds` still accepts the older handle, and `EndRemoved` can record removal as its first cancellation cause. The pending removal is also carried forward to the replacement. [Hub admission](../../../../src/server/internal/link/hub.go#L96), [live hold](../../../../src/server/internal/link/hub.go#L176), [removal signal](../../../../src/server/internal/link/hub.go#L209), [frozen replacement order](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L125), [frozen removal order](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L164).

In a PostgreSQL probe, stream A finished its Welcome. I paused B after PostgreSQL committed its admission and before the database call returned to the hub. An independent read saw B's generation. I committed removal, signalled the hub and resumed B. A received `failed_precondition` with `unknown_machine` in both Connect and Google Remote Procedure Call (gRPC). B could finish its Welcome and then received `unknown_machine`. A separate probe at the same boundary showed that `Hub.Holds` accepted A while `CheckCurrent` rejected it. The durable checks still prevented A from writing.

Record that admission is in progress before entering the database and queue removal against that state without blocking the removal notifier. Resolve the displacement and removal causes against the durable winner. Make any gate that authorizes actions verify the exact durable stream identifier. Add a barrier after PostgreSQL commits and before publication, with A live and a removal between those two points.

### post-a 2 · MAJOR · Plan item 6

A failed Welcome flush is discarded. The protocol handler calls the writer's `Flush` method, which drops the error reported by `FlushError`. If the buffered writes succeeded, `SendLinkWelcome` can return success and the handshake can call `FinishWelcome` and log the stream as opened. [Link writer flush](../../../../src/server/internal/transport/link_stream.go#L120), [Welcome send result](../../../../src/server/internal/transport/link_stream.go#L161), [handshake completion](../../../../src/server/internal/link/handshake.go#L220).

I exercised the generated handler with a writer that accepted the Welcome bytes and failed only the Welcome flush. `SendLinkWelcome` returned nil in both protocols. A buffered write followed by a failed flush can therefore leave the server treating the handshake as complete even though the flush reported a transport failure.

Capture the first flush error while the Welcome guard is armed. Return it if the send otherwise succeeded, preserving the gate refusal and first cancellation cause. Add a test that fails the flush after a successful write and observes that Welcome never becomes live.

### post-a 3 · MINOR · Plan item 7 and R8

The phase timer can turn an on-time request closure into `deadline_exceeded`. After the timer closes the body, `receivePhase` converts any non-malformed receive error it collects to the timeout before it checks the recorded receive time. [Receive timestamp](../../../../src/server/internal/link/handshake.go#L231), [timer result selection](../../../../src/server/internal/link/handshake.go#L267), [terminal code mapping](../../../../src/server/internal/link/handshake.go#L327).

I half-closed the request after Challenge and held the reader after it timestamped the end of file. I fired the phase timer and then released the captured result. Both protocols returned `deadline_exceeded` for the on-time closure, which maps to `canceled` when handed back before the timer runs.

Preserve a receive that completed by the cutoff, including an ordinary closure. Identify a receive interrupted by the timer as a separate outcome. Extend the exact-cutoff tests with a held closure result and a timer-interrupted receive.

### post-a 4 · MINOR · R3

`State.CheckCurrent` queries the registry's `machines` table directly. The design gives a part access to another part's records through its Go interface. A registry schema change can therefore break Open's live identity check even after the registry implementation has been updated. The existing store read transaction provides a repeatable snapshot. [Identity query](../../../../src/server/internal/link/presence.go#L86), [registry lookup](../../../../src/server/internal/machines/store.go#L107), [store read](../../../../src/server/internal/store/pool.go#L295), [foundation rule R3](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L364).

Read approval through the registry and query the exact link row inside the same store read transaction. Preserve the missing-machine decision ahead of the displaced-stream decision.

### post-a 5 · MINOR · Plan item 3 and R10

The frozen tests of these items accept an implementation that skips the central processing unit (CPU) limit on a current heartbeat. The current-stream test exercises memory at 101, and the stale-stream test exercises CPU at 101 only after displacement. [Current heartbeat assertion](../../../../src/server/internal/link/presence_test.go#L240), [stale heartbeat assertion](../../../../src/server/internal/link/oneline_test.go#L155), [both load limits](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L18).

I removed only the CPU comparison in scratch. The item 2, 3 and 7 link tests produced only the four pre-existing revocation fixture failures. A separate current-stream probe passed on the original and failed on the mutant. PostgreSQL rejected the write and the error mapped to `unavailable`, where the link requires `invalid_argument`. Add a current-stream CPU-at-101 case alongside the existing memory case and assert that the connection row and feed stay unchanged.

## Post-build checks

- **Item 3 registry and presence.** The merged registry and state tests exercised both operation orders. Admission and heartbeat took `Registry.Lock`. Disconnect used `TryLock` and returned a retryable PostgreSQL `55P03` while a competing write still held the row. Retrying the displaced disconnect left the replacement intact. Startup clear processed one machine per transaction and kept its marks and generations. Removal first deleted the registry row, then forgot link rows in the same transaction. The approval probe held the machine row while used-token revocation waited. Once approval committed, revocation kept the machine and its subsequent Welcome carried approved revision 2 and a connection token. [Registry locks](../../../../src/server/internal/machines/store.go#L226), [presence writes](../../../../src/server/internal/link/presence.go#L42), [interleaving assertions](../../../../src/server/internal/link/oneline_test.go#L72), [startup clear assertions](../../../../src/server/internal/link/presence_test.go#L271).
- **Items 2 and 6 first-write fence.** Both protocols suppressed an older unsent Welcome after replacement, including after the replacement closed. A newer stream also finished admission while the older stream's already-started write was held. The transport test observed readable `aborted` after pre-entry refusal, with zero delegated writes, flushes or resets. The reader-close and terminal-write deadline tests passed. Finding 1 covers the separate interval before publication. Finding 2 covers flush failure after entry. [Unsent Welcome test](../../../../src/server/internal/link/handshake_test.go#L963), [started Welcome test](../../../../src/server/internal/link/handshake_test.go#L1038), [transport fence assertions](../../../../src/server/internal/transport/link_stream_test.go#L194).
- **Item 6 frame size.** Complete and payload-stalled oversized envelopes stopped at the five-byte prefix in both protocols. The result held on the first envelope and after a valid frame. Exact-limit and repeated valid frames passed. A compressed frame above the decoded limit was refused by the generated protocol reader. The application interface tests also passed, including the existing Join decoding test. [Envelope test](../../../../src/server/internal/transport/link_stream_test.go#L408), [route wiring](../../../../src/server/internal/api/server.go#L155).
- **Item 7 handshake.** Both proof texts and both signatures matched the frozen examples byte for byte. The generated route sent a valid Challenge before refusal, rejected replayed Proof and unusable keys, and kept lookup, revision and Proof precedence in the named cases. It accepted valid results at the inclusive cutoff. The lost-commit-reply test observed cleanup of the exact candidate and preservation of a later replacement. Two approved Welcomes carried different tokens and the waiting Welcome carried none. The disposable tokens were absent from the tested database rows, server logs and owner refusal body. [Proof specification](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L277), [example assertions](../../../../src/server/internal/link/handshake_test.go#L310), [token assertions](../../../../src/server/internal/link/handshake_test.go#L407), [uncertain commit assertions](../../../../src/server/internal/link/handshake_test.go#L1173).
- **Peer address and deployed checks.** In the generated Hypertext Transfer Protocol version 2 (HTTP/2) fixture, both protocols recorded the transport peer in direct mode and an empty address in private-stack mode with missing, supplied, repeated and malformed forwarding fields. The frozen tests saw `Cache-Control: no-store` at Challenge. This evidence covers the assigned portions of P1, P2, P3, P7, P8, P9, P10 and P11 in process. The deployed proxy, watchdog, cleanup retry lanes, command behavior and startup wiring require the later items and deployed stack. [Peer address policy](../../../../src/server/internal/link/handshake.go#L361), [Challenge header assertion](../../../../src/server/internal/link/handshake_test.go#L265), [post-build checks](../plan.md#L342).

## Test and mutation output

I ran the plan's named filters through `deploy/test/with-postgres.sh` with `go test -race -count=1 -p 1`. No run reported a race.

| Item | Output |
|---|---|
| 2 | The five hub tests passed. The link package printed `ok` in 1.289 seconds. |
| 3 | The eight presence and registry tests passed. The link and machines packages printed `ok` in 13.311 and 2.907 seconds. |
| 6 | The four transport tests passed. Transport printed `ok` in 1.259 seconds and the application interface package printed `ok` in 75.380 seconds. |
| 7, frozen revision | The link package printed `FAIL` in 51.229 seconds. The failures were the four known revocation cases in `TestRemovalOrdersAdmission`. Both before-admission cases saw a Welcome. Both after-admission cases reached `deadline_exceeded` on the later dial. The fixture revoked the token and cleared link rows while leaving the waiting registry row. The other named tests and all four direct-removal cases passed. [Frozen fixture](../../../../src/server/internal/link/handshake_test.go#L923). |
| 7, builder's fixture repair | All eight removal and revocation cases passed after the fixture called `DeleteWaiting` before `Forget`. The full item 7 filter then passed, and the link package printed `ok` in 41.231 seconds. |

| Planned mutation | Observed assertion |
|---|---|
| Clear the newest generation when its handle unregisters. | `hub_test.go:95` reported that the hub lost its high-water generation while an older candidate was still tracked. |
| Remove the registry row lock used by admission and heartbeat. | `oneline_test.go:57` reported a competing operation returning before the first gave up the row lock. The nonwaiting disconnect assertion also failed. |
| Pass the oversized envelope prefix to the protocol reader. | `link_stream_test.go:480` failed for complete and stalled cases in both protocols. |
| Swap the nonces in the proof text. | `handshake_test.go:330` and `handshake_test.go:334` reported mismatched bytes and signatures for both roles. |

## Grade

**6/10.**
