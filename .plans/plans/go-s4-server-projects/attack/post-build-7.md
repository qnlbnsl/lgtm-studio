# REVISE

A repository watch can send a complete saved record after session revocation has committed.

OpenAI supplied the test writers, the builder and this reviewer. I wrote neither the built code nor its frozen tests. This review covers Projects item 7 on `sprint-0` at commit `75958b90`. The task identifier is `postbuild-go-s4-server-projects-7` and the nonce is `480a47`.

## Findings

### post-7 1 · MAJOR · A saved frame can begin after revocation commits

The plan requires sign-out or expiry to end a live watch before the next change is sent. The contract requires a session throughout the watch. [Post-build check](../plan.md#L286) · [Repository watch contract](../../../../src/contracts/proto/lgtm/v1/projects.proto#L70)

By reading, the shared watch checks the session inside the change read, then loads the projection. Before send it checks the subscription's cancellation cause. The independent authorization loop polls every 30 seconds. Sign-out calls `Sessions.Revoke`, which commits the deletion and drops the verifier's cache entry. The open subscription receives no immediate revocation signal. [Per-change authorization and load](../../../../src/server/internal/watch/watch.go#L151) · [Send entry](../../../../src/server/internal/watch/watch.go#L124) · [Authorization interval](../../../../src/server/internal/watch/watch.go#L12) · [Sign-out](../../../../src/server/internal/api/session.go#L95) · [Revocation](../../../../src/server/internal/auth/sessions.go#L162)

Confirmed live, I paused a saved record at the start of its projection query, after its session check and before any frame write. I committed revocation and verified that the same verifier was refused by a fresh check. The stream then sent the complete saved record at position 2. A later change produced the fixed `unauthenticated` status. The frozen revocation test commits its first change after revocation and so misses the ordering in this probe. [Saved projection](../../../../src/server/internal/projects/repositories.go#L51) · [Frozen revocation case](../../../../src/server/internal/projects/watch_test.go#L399)

Order the shared watch's permission to **start** a frame with revocation and session expiry. A projection that completes after revocation must be refused before its first write. Keep the existing deadline for a write that already started. Add a barrier test that commits revocation after the projection starts and asserts the fixed terminal status as the next result.

### post-7 2 · MAJOR · A blocked socket loses the fixed lag status

At 2,048 outstanding references, the plan and the shared watch contract require a fixed `aborted` response and a replacement snapshot on reopen. This also concerns foundation rules 5 and 8, which require the shared watch path and a fixed display error. [Item 7 cutoff](../plan.md#L241) · [Shared contract](../../../../src/contracts/proto/lgtm/v1/common.proto#L16) · [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L358)

By reading, the follower cancels the subscription at the cutoff. While a frame is being sent, cancellation sets the real response writer's deadline to the current time. The service then maps the cancellation cause to `aborted` and sets a terminal deadline. The lag fixture substitutes a cancellation signal for the real deadline while the write is held, specifically to keep the underlying writer available for the terminal status. [Follower cutoff](../../../../src/server/internal/store/follow.go#L197) · [Write cancellation](../../../../src/server/internal/transport/deadline.go#L53) · [Terminal deadline](../../../../src/server/internal/transport/link_stream.go#L91) · [Projects status mapping](../../../../src/server/internal/projects/watch.go#L139) · [Frozen writer](../../../../src/server/internal/projects/watch_changes_test.go#L213)

Confirmed live over HTTP/1.1, with a small TCP send buffer and a reader paused during a 2,341,085-byte saved frame, lag interrupted the actual socket write before its normal ten-second deadline. When the client drained the socket, Connect returned `unknown: read enveloped message: unexpected EOF`. Reopening delivered all 400 folders at the new feed head. By reading, the website retry loop can reopen after this error. [Website retry loop](../../../../src/frontend/src/server/sync.ts#L175)

Keep the fixed `aborted` response at a writable frame boundary. Specify a transport close as a reconnect signal if cancellation interrupts a partially written frame, since a terminal envelope cannot repair a truncated data envelope. Exercise both outcomes with the real writer and require a complete replacement snapshot.

### post-7 3 · MINOR · The entry refusal test accepts the live-watch sentence

The plan's fixed-error requirement and foundation rule 8 apply to both entry and live closure. The contract specifies “Sign in to continue.” at entry and “Sign in again to reopen this watch.” after the snapshot. By reading, the implementation uses the two sentences. The missing and stale verifier assertions check only the status code. [Foundation rule 8](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L369) · [Contract sentences](../../../../src/contracts/proto/lgtm/v1/projects.proto#L49) · [Implementation](../../../../src/server/internal/projects/watch.go#L127) · [Entry assertions](../../../../src/server/internal/projects/watch_test.go#L380) · [Stale verifier assertion](../../../../src/server/internal/projects/watch_test.go#L415)

Confirmed live, substituting the live-watch sentence at entry passed all six frozen item 7 groups in 14.875 seconds. A caller opening with a stale session would receive the wrong instruction. Assert the exact entry sentence for both the missing and stale verifier cases and retain the live-closure assertions.

## Reserved refresh and the queued removal

The round-one register's logic-r1 02 and its three duplicates left the large-removal refresh unreviewed under foundation rule 4. By reading, an ordinary attachment uses a version 7 universally unique identifier (UUID). A saved reference for the reserved key `refresh` returns `store.ErrWatchLagged` before the projection query. The shared watch forwards the error and Projects returns `aborted` with “The watch fell behind and must be reopened.” [Round-one register](round-1-findings.md#L10) · [Plan mechanism](../plan.md#L239) · [Generated identifier](../../../../src/server/internal/projects/repositories.go#L35) · [Refresh handling](../../../../src/server/internal/projects/repositories.go#L47) · [Error propagation](../../../../src/server/internal/watch/watch.go#L163)

Confirmed live against real PostgreSQL, I seeded 257 affected repositories, including a shared one. A modeled removal rolled back with all 257 still visible at position 258. The original stream stayed open and delivered an independent save at 259. Retrying with one committed refresh at 260 returned the fixed status and no refresh data frame. The replacement snapshot at 260 contained the shared survivor with only the other machine's folder, and the unrelated control repository. This exercised the item 7 consumer of the reserved reference.

Confirmed live in a separate seeded removal race, I held a dequeued save before its read snapshot, then committed the removal. The queued save at position 4 resolved to the removed identifier. The watch sent the complete shared survivor at 5 and the orphan's removed identifier at 6. A reopen at 6 contained only the survivor. The earlier Add replay and final browser tree remain part of the assembled Projects and website scenario. The pinned server startup does not yet mount Projects, and the actual large machine-removal transaction belongs to items 6 and 8. [Post-build scenario](../plan.md#L286) · [Combined case](../plan.md#L252) · [Pinned startup](../../../../src/server/internal/app/start.go#L266)

## Watch behavior and test results at the pinned commit

By reading, the handler uses the process follower for `repositories` with empty scope and reads the ordered repository and folder join inside the Store transaction that reads the head. The saved loader closes its rows and checks the iterator error. Frame conversion includes a feed position and carries an explicit `all` even for an empty collection. The handler attaches shutdown cancellation, requires a verified session and uses the shared transport deadline. [Projection and handler](../../../../src/server/internal/projects/watch.go#L20) · [Feed-head transaction](../../../../src/server/internal/store/read_feed.go#L43) · [Saved row cleanup](../../../../src/server/internal/projects/repositories.go#L60) · [Frames](../../../../src/server/internal/projects/watch.go#L112)

Confirmed live, the frozen tests delivered the older snapshot through a competing commit and then the newer full saved record. The signed attachment sent a complete saved repository, other topics and scopes produced no frame, and the seeded removal passed commit and rollback controls. Entry without a verifier returned `unauthenticated`. Expiry and revocation before a subsequent commit produced the fixed live-watch closure. The held-writer fixture stayed open through reference 2,047, returned fixed `aborted` at 2,048 and reopened to a complete positioned snapshot. [Snapshot and filtering](../../../../src/server/internal/projects/watch_test.go#L277) · [Session cases](../../../../src/server/internal/projects/watch_test.go#L380) · [Attachment](../../../../src/server/internal/projects/watch_changes_test.go#L29) · [Removal](../../../../src/server/internal/projects/watch_changes_test.go#L133) · [Lag](../../../../src/server/internal/projects/watch_changes_test.go#L286)

The plan gate and the focused race run matched all six top-level watch groups with zero skips. Separate reruns reproduced both the saved frame after revocation and the truncated response under socket backpressure twice more. The known issue 1704 concerns the earlier operation wait. The deliberately failing router and stack tests, the long Link package, and the listed intermittent client and paired tests are outside this item 7 gate. [Plan gate](../plan.md#L263) · [Issue 1704](../../../../docs/issues.md#L8151) · [Shared test rules](../../../../.foundry/go-rewrite/common-build.md#L41)

```sh
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/projects -run '^TestProjectWatch' -count=1
bash deploy/test/with-postgres.sh -- go test -race -count=1 -timeout 15m -v ./src/server/internal/projects -run '^TestProjectWatch'
```

```text
--- PASS: TestProjectWatchAttachment (2.37s)
--- PASS: TestProjectWatchExplicitRefresh (0.89s)
--- PASS: TestProjectWatchSeededRemoval (1.68s)
--- PASS: TestProjectWatchLagAndReopen (2.04s)
--- PASS: TestProjectWatchSnapshotAndChanges (1.71s)
--- PASS: TestProjectWatchSessionExpiry (2.29s)
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  11.011s
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  36.680s  (race run)
```

Each mutation used its own Go overlay against the clean commit. The frozen test files match the test-writer commit. [Risky mutations](../plan.md#L241)

| Mutation | Observed result |
|---|---|
| Remove entry and live authorization. | Exit 1. The missing verifier, expiry and revocation each received a data frame at `watch_test.go:385` or `watch_test.go:414`. |
| Read repository rows in a second transaction after reading the feed head. | Exit 1. The old snapshot included the competing third folder at `watch_test.go:351`. |
| Return only the first folder in the saved frame. | Exit 1. The signed attachment returned one of two folders at `watch_changes_test.go:59`. |
| Treat the refresh key as an ordinary saved key. | Exit 1. The watch sent a data frame instead of ending at `watch_changes_test.go:128`. |
| Disable idle reauthorization while keeping the per-change check. | Exit 0. All six item 7 groups passed in 12.830 seconds. The shared watch suite has a separate idle revocation test. [Shared test](../../../../src/server/internal/watch/watch_test.go#L582) |

**Grade.** 7 out of 10.
