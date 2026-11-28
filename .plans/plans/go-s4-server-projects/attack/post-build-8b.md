# SHIP

All frozen startup tests pass when `startup.run` discards the Projects hook-registration error.

OpenAI supplied this reviewer, the startup builder, and the author of the `LagDuringSocketWrite` amendment. Anthropic supplied the author of the original frozen startup tests. The review covers the startup bullet of Projects item 8 on `sprint-0`, in `31792bc3~1..0bbd0d7f`. The task identifier is `postbuild-go-s4-server-projects-8b` and the nonce is `feecc3`.

## Findings

### post-8b 1 · MINOR · The startup tests accept a discarded registration error

By reading, item 8 requires startup to return a registration error before it starts workers or opens the listener. The built code returns the fixed duplicate-route error. The overlap test calls the registration helper on a separate Link state. Confirmed live, all twelve named startup cases passed in 77.390 seconds when the startup call discarded the helper's result. A future part that claims a Projects route first could then leave the website serving Projects with missing Link callbacks. Inject a duplicate route through the assembled startup and assert the fixed diagnostic, zero pruner, retirement, or Link worker starts, and an unopened listener. [Plan](../plan.md#L247) · [Startup error](../../../../src/server/internal/app/start.go#L253) · [Link registration](../../../../src/server/internal/link/parts.go#L62) · [Frozen overlap assertion](../../../../src/server/internal/app/projects_test.go#L299)

### post-8b 2 · MINOR · The socket case accepts cancellation after the in-flight send

By reading, the plan requires lag to interrupt a held send. The amended real socket case accepts a complete in-flight frame followed by the fixed abort. Confirmed live, the socket case passed in 9.448 seconds when I detached the send from cancellation. The Projects-level held-writer test failed on the same change at its interruption assertion. A real writer that stops observing cancellation could finish the oversized frame or consume the ordinary ten-second write budget before the peer learns it must reopen. Keep the socket's terminal and reopen checks, and have the fixture observe that lag interrupts the current write while the peer still holds the drain. [Plan](../plan.md#L241) · [Socket drain and terminal assertion](../../../../src/server/internal/app/projects_test.go#L1374) · [Watch send](../../../../src/server/internal/watch/watch.go#L129) · [Transport timeout](../../../../src/server/internal/transport/deadline.go#L11) · [Held-writer assertion](../../../../src/server/internal/projects/watch_changes_test.go#L352)

**Grade 9/10.**

## The assembled server commits and rolls back the repository refresh

By reading, the latest review register flags the reserved repository refresh mechanism used by items 5 through 7. Item 8 exercises it through the mounted service in `LargeFlaggedRemoval`. Confirmed live, that test passed with the dispatched command staged one event above the committed mark and the recovery warning set by a signed unseen run behind the mark. The initial snapshot contained exactly 257 affected repositories. One had a folder on a second machine and 256 would become orphans. [Register](round-1-findings.md#L10) · [Combined plan case](../plan.md#L252) · [Membership seed](../../../../src/server/internal/app/projects_test.go#L983) · [Stage and warning](../../../../src/server/internal/app/projects_test.go#L1015)

The fixture verified that three PostgreSQL faults were reached. The faults refused the orphan delete, the Link machine delete after the Projects callback, and the feed flush after the refresh had been recorded. After each fault, the machine, warning, command stage, operation, membership, and feed matched their earlier state. A later control save reached the original watch at its committed position. Confirmed live, committing the refresh in a separate transaction failed the frozen Link-delete rollback assertion at line 1099. This covers the refresh-atomicity gap identified in issue 1707 and exercises foundation rule 4. [Fault and rollback checks](../../../../src/server/internal/app/projects_test.go#L1062) · [Link callback order](../../../../src/server/internal/link/presence.go#L320) · [Store feed flush](../../../../src/server/internal/store/feed.go#L120) · [Foundation rule 4](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L365) · [Issue 1707](../../../../docs/issues.md#L8178)

The successful retry deleted the machine and warning, settled the command as `removed`, cleared its staged result, and made the keyed operation replay `machine_removed`. The feed held exactly one repository reference, the empty-scope `refresh` save. The original watch returned fixed `aborted` and the reopened snapshot matched the feed head and retained only the other machine's folder. The 254-, 255-, and 256-repository controls and the natural 2,047- and 2,048-reference watch boundary also passed. The combined test meets the item 8 acceptance row. [Retry and snapshot](../../../../src/server/internal/app/projects_test.go#L1132) · [Removal controls](../../../../src/server/internal/projects/removal_test.go#L26) · [Frame boundary](../../../../src/server/internal/app/projects_test.go#L1200) · [Acceptance row](../plan.md#L226)

## Startup wiring and the amended socket fixture

By reading, startup adds Projects to the migration manifest before readiness and uses the current Store, sessions, registry, Link state, follower, and lifetime for the service. The single Link registration passes the same transaction to the folder report, command settlement, and machine removal callbacks. The lock monitor and follower run as startup prerequisites, as the builder recorded. A registration error exits before the pruner, retirement worker, Link worker, or listener starts. The failure path releases the prerequisites and Store. The retirement worker belongs to the shutdown wait group and the router receives the Projects handler. The error and worker diagnostic paths use fixed messages. [Migration](../../../../src/server/internal/app/start.go#L181) · [Startup wiring](../../../../src/server/internal/app/start.go#L240) · [Link adapters](../../../../src/server/internal/app/projects_hooks.go#L12) · [Release](../../../../src/server/internal/app/start.go#L370) · [Store diagnostics](../../../../src/server/internal/store/pool.go#L47)

Confirmed live, the assembled guards rejected missing and revoked sessions and an unlisted origin for all three routes. Signing out or ageing the session before publishing the next change ended the live watch with the fixed unauthenticated response. Issue 1705 records the separate race after the watch's authorization check. The queued-save test delivered a removed identifier and the earlier keyed Add still replayed unchanged. The local upgrade probes covered the interrupted Projects migration with an existing session and ordinary receipt, Projects migration and mounted discovery at startup, and restart after an interrupted Link migration. The real-client scenarios, browser tree, and deployed upgrade are separate acceptance runs under the plan. The pinned tree contains four earlier paired test groups. [Guards](../../../../src/server/internal/app/projects_test.go#L381) · [Issue 1705](../../../../docs/issues.md#L8160) · [Queued-save race](../../../../src/server/internal/app/projects_test.go#L942) · [Projects upgrade](../../../../src/server/internal/projects/migrate_test.go#L28) · [Paired and deployed gate](../plan.md#L265)

Confirmed live, the amended socket case still proves that an oversized saved frame can be in flight when the shared follower reaches the lag threshold, that the peer observes a closed connection or the fixed `aborted` terminal response, and that reopening yields the complete positioned snapshot. Raising the live cutoff to 4,096 made the post-lag sentinel reach the peer as another data frame and failed the amended assertion. The larger receive buffer and fresh read deadlines preserve those observations. The interruption of the in-flight write itself is covered by the Projects-level held-writer test described in post-8b 2. The known contract wording issue about a closed connection remains recorded under 1706. [Socket test](../../../../src/server/internal/app/projects_test.go#L1325) · [Sentinel and reopen](../../../../src/server/internal/app/projects_test.go#L1366) · [Issue 1706](../../../../docs/issues.md#L8171)

## Frozen assertions for the four omissions

I archived `0bbd0d7f` once and used one-file Go overlays, one mutation per invocation. The archived test file matched the commit before and after the review. Each command used disposable PostgreSQL and `go test -v -count=1 -p 1 -timeout 20m -overlay <file> ./src/server/internal/app -run <selector>`. All four commands exited 1 on a test assertion.

| Omission | Failed frozen assertion |
|---|---|
| Remove Projects from the migration manifest. | `Migration`, line 295. Startup reported readiness without the Projects ledger and tables. The package printed `FAIL` at 5.648 seconds. |
| Skip the registration call in startup. | `FolderRefusalsAndRestart`, line 558. The signed folder report left the waiting inspection unfinished at the fixture's 15-second bound. The package printed `FAIL` at 18.751 seconds. |
| Omit the retirement worker. | `FolderRefusalsAndRestart`, line 643. Startup left the aged operation and its held receipt present at the fixture's 15-second bound. The package printed `FAIL` at 49.130 seconds. |
| Omit Projects from the router options. | `DiscoveryAndGuards`, line 349. Discovery advertised Projects zero times. The package printed `FAIL` at 3.214 seconds. |

## PostgreSQL verification

Each test command ran through `bash deploy/test/with-postgres.sh --` on PostgreSQL 16.16. All of the focused baseline groups printed `PASS` with zero skips.

| Command after the wrapper | Observed package result |
|---|---|
| `go test -v -count=1 -p 1 -timeout 20m ./src/server/internal/app -run '^TestProjectStartup'` | Both startup groups and all twelve named cases passed. The app package printed `ok` in 77.795 seconds. |
| The startup command with `-race`. | Both startup groups and all twelve named cases passed. The app package printed `ok` in 123.316 seconds. |
| `go test -v -count=1 -timeout 20m ./src/server/internal/api -run '^TestProjectRouter'` | All six router groups passed. The router package printed `ok` in 8.877 seconds. |
| `go test -v -count=1 -timeout 20m ./src/server/internal/projects -run '^TestProjectWatch'` | All seven watch groups passed. Projects printed `ok` in 16.489 seconds. |
| `go test -v -count=1 -p 1 -timeout 20m ./src/server/internal/app ./src/server/internal/projects ./src/server/internal/store -run '^(TestServerRestartsAfterInterruptedCommandMigration\|TestHeldReceiptUpgrade\|TestProjectMigration\|TestMigrateFailureRollsBack)$'` | All four upgrade probes passed. App, Projects, and Store printed `ok` in 10.044, 6.157, and 2.464 seconds. |
| `go test -v -count=1 -p 1 -timeout 20m ./src/server/internal/projects -run '^TestProjectMachineRemoval$/^FeedBudget(254\|255\|256)$'` | All three removal controls passed. Projects printed `ok` in 21.652 seconds. |
| `go test -v -count=1 -timeout 25m ./src/server/...` | Exit 0. All 14 tested packages printed `ok`, including app in 243.344 seconds, Link in 475.947 seconds, and Projects in 286.618 seconds. The sole skip was `TestOpenRefusesCredentialOwnedByAnotherUser`, which needs superuser access to change the fixture file's owner. |

The tracked startup files passed `gofmt -l` and the reviewed range passed `git diff --check`. The archive passed `go vet ./src/server/...` and `go build -buildvcs=false ./src/server/...`.
