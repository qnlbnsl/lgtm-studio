# SHIP

The startup test accepts a server that clears stale presence asynchronously after the listener opens, so the order the plan requires is not pinned by any test.

This is a review by Claude of code that Codex workers built and tested, so the rule that another vendor reviews is met. I reviewed branch `sprint-0` at HEAD, which includes the later repairs recorded in [post-build-d](post-build-d.md), [post-build-e](post-build-e.md) and issue 1667. I did not report again anything those records hold. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s3-server-stream-c.md) · [Plan](../plan.md#L241)

## Findings

### post-c 1 · MINOR · No test pins that stale presence is cleared before the listener opens

The plan requires that stale link connections are cleared before readiness or listening. [Plan item 10](../plan.md#L241) The code does this: `ClearConnections` runs at [start.go:230](../../../../src/server/internal/app/start.go#L230), before the follower, the last lock probe and `net.Listen`.

No test fails when that order is broken. I replaced the clearing call with an asynchronous call that runs after startup continues, and I kept the failure path out of the way. `TestStreamStartClearsStalePresence` still passed. Only `TestPartialLinkCleanupRefusesReadiness` failed, and it fails because the failure path was removed, not because of the order. [Test](../../../../src/server/internal/app/link_stream_test.go#L212)

Failure scenario: a later change starts the clearing in a goroutine. The first Machines watch after a restart can then show a machine as connected, and the first Open of that machine can be admitted beside a stale row. The test passes because it reads the rows only after the restart has settled.

Fix: add a test that holds the clearing write in PostgreSQL, tries to connect to the listen address, and expects no listener until the write completes.

### post-c 2 · MINOR · Request handlers are not joined before the lock and the Store are released

The plan says workers are joined before the Store and the lock are released. [Plan item 10](../plan.md#L241) `release` joins the workers and closes the follower first, which is correct. [start.go:345](../../../../src/server/internal/app/start.go#L345)

Handlers are not workers. When the five-second drain expires, `Shutdown` returns its error, which `serve` discards, and `Close` does not wait for handlers. [start.go:329](../../../../src/server/internal/app/start.go#L329) The same holds when the lock is lost, where `Close` alone is called. `release` then closes the instance lock at line 354 and the Store at line 358 while a handler may still be inside a Store write, which can run for up to 15 seconds.

Failure scenario: the drain expires because a handler is waiting on a slow write. The lock is released, a second server takes it and clears stale presence, and the first server's handler then commits a heartbeat or an admission after that. This is by reading, not run. The window needs a drain timeout or a lock loss, which the owner has put outside wave 1 for two live copies, so I rate it MINOR.

Fix: track handlers in a wait group inside the stream service, or wait for the Store's in-flight writes, before the lock is closed. Record the case in a test that holds a write across the drain budget.

### post-c 3 · MINOR · A panic inside the admission commit leaves the hub's admission state set

`Admit` sets `m.admitting` before calling `commit` and clears it in a deferred function that is registered only after `commit` returns. [hub.go:107](../../../../src/server/internal/link/hub.go#L107) A panic inside `commit` is recovered by the HTTP server for that request, and `admitting` stays set.

While it is set, `holds` answers false for every non-watch caller of that machine, so no action reaches the machine until the process restarts. `cancelRemoved` is not replayed either, so a removal that arrived during the panic is also not applied to the stream.

Failure scenario: a defect in the store layer panics once during an admission. That machine stays unreachable by commands, with no log line that ties the symptom to the cause. The hub is otherwise free of deadlock and lost removals, as I traced below. This is by reading, not run.

Fix: register the clearing function with `defer` before `commit` runs, and let it also run on a panic.

## What I checked and found sound

The stop order is correct. The lifetimes are cancelled first, the watch and stream contexts follow it, then `Shutdown` drains, then the workers are joined, then the follower is closed, then the lock is closed, then the Store is closed. A failed start releases in the same order. [start.go:314](../../../../src/server/internal/app/start.go#L314)

The hub's admission state cannot deadlock. The mutex is never held across `commit` or across the gate wait. A removal that arrives during an admission is replayed by `cancelRemoved` when the admission ends and again in `FinishWelcome`, so I found no ordering that loses it.

The repairs of post-a 1 to 5 hold at HEAD. `CheckCurrent` reads the registry row and the connection row in one read transaction. The handshake deadline closes the read side and reports `errHandshakeDeadline`. `SendLinkWelcome` returns the flush error. The Machines watch arms `BoundLinkEnd` as its first statement, so every early return gets the deadline. A malformed watch request returns the fixed sentence through the redacting wrapper, for Connect, gRPC and gRPC-Web.

## Post-build scenarios

| Scenario | What I ran | Result |
|---|---|---|
| P8, restart and startup cleanup | `TestStreamStartClearsStalePresence`, `TestPartialLinkCleanupRefusesReadiness` | Both passed. |
| Shutdown of live, pre-Welcome and admitted streams | `TestShutdownEndsLinkStreams` | Passed. |
| Open needs no session | `TestUnsignedOpenProvesServer` on Connect and gRPC | Passed. |
| Fixture seams | `TestLinkSeamsReachTheService` | Passed. |
| Watch shutdown and malformed watch request | The shutdown, malformed and terminal-deadline tests | Passed. |
| P2 and P3 removal, replacement and watchdog | The link package filter below | Passed. |

I did not run the combined real-client outbox test, the deployed stack or the proxy soak. They are outside these items.

## Test output

All runs used `deploy/test/with-postgres.sh -- go test -race -count=1`, with a disposable PostgreSQL container that the script starts and removes.

| Packages and filter | Output |
|---|---|
| `internal/app`, named item 10 tests, shutdown tests, `TestServerServesMachines` | `ok` in 38.086 seconds |
| `servertest`, `TestLinkSeamsReachTheService` | `ok` in 24.347 seconds |
| `internal/api`, malformed and terminal-deadline filters | `ok` in 7.496 seconds |
| `internal/link`, hub, admission, welcome, removal, watchdog, flush and lifecycle filter | `ok` in 62.686 seconds, 22 passing top-level tests |
| `internal/transport`, same filter | `ok` in 1.097 seconds |

## Mutations

I applied each change alone in a scratch copy of HEAD.

| Mutation | Result |
|---|---|
| Skip the startup clearing | `TestStreamStartClearsStalePresence` failed at `link_stream_test.go:230`. |
| Ignore a failed clearing and open the listener | `TestPartialLinkCleanupRefusesReadiness` failed at `link_stream_test.go:284`. |
| Give the stream service a lifetime that is never cancelled | `TestShutdownEndsLinkStreams` failed at `link_stream_test.go:390`, code `internal`. |
| Give the Machines watch a lifetime that is never cancelled | `TestMachinesWatchReceivesFixedShutdownSentence` and `TestShutdownDrainsMachineApproval` failed. |
| Close the instance lock before the workers and the follower | `TestShutdownDrainsMachineApproval` failed at `machines_shutdown_test.go:189`. |
| Drop the fixture seams | `TestLinkSeamsReachTheService` failed at `link_stream_test.go:154`. |
| Run the clearing in a goroutine, keeping the failure path out of the way | `TestStreamStartClearsStalePresence` passed. This is post-c 1. |

I did not run the plan's mutation that puts Open behind the session guard. The router's `linkProcedures` table exempts Open from the guard, and `TestUnsignedOpenProvesServer` covers both protocols, but I did not confirm the failure by mutation.

## Grade

**8 out of 10.** The start and stop order is right, the hub is sound, and every plan mutation I ran is caught. The remaining gaps are one unpinned ordering and two defensive gaps that need a rare failure.
