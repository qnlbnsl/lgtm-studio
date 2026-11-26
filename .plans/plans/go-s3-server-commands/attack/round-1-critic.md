REVISE

The result gate can finish a command before its current attempt reaches the transport, and the send test treats that outcome as correct.

## Findings

### critic-r1 01 | MAJOR | R6

The frozen contract accepts a result for the last attempt the server sent. The plan records an attempt at claim time, accepts results using that record alone and explicitly tests finalization while dispatch is still paused. There is no durable distinction between a reserved attempt and one that entered the transport. `src/contracts/proto/lgtm/link/v1/commands.proto:61` and `.plans/plans/go-s3-server-commands/plan.md:101,153,170,172`.

If the cutoff passes after claim commit, the send fence skips the frame and keeps the stream open. The server can still accept a matching test result and stop retrying an action that the machine never received.

Specify when the result gate becomes eligible to accept an attempt. Keep the committed attempt before dispatch, require the dispatch boundary to have been reached and document how a transport failure after possible delivery is handled. Change the pre-send fixture to require a matching result for a claim that skipped transport to leave the row pending. Prove the separate post-dispatch case with a result after an observed frame.

### critic-r1 02 | MAJOR | C8

The send fence spends one shared database writer slot while it waits on the peer. The two-second deadline limits one write, but the plan leaves the number of concurrent send fences unbounded. At the minimum pool size, the store exposes five writer slots shared by the fence, heartbeats, command claims and other writes. `.plans/plans/go-s3-server-commands/plan.md:170,183` and `src/server/internal/store/pool.go:19,181,301`.

With five machines holding blocked sends, a healthy sixth machine has to wait for a slow peer's slot before it can write its heartbeat or claim a command. Under repeated batches of slow sends, the healthy machine's two-second claim can expire waiting to enter the store and terminate its stream. The per-machine tests exercise no such shared bottleneck.

Give transport-held transactions a concurrency budget that leaves capacity for ordinary database writes at the minimum pool size. Specify how a writer that yields for capacity is woken again. Add a gate with the minimum pool and blocked writes on several machine streams. A healthy stream must acknowledge a heartbeat and make command progress before those writes are released.

### critic-r1 03 | MAJOR | R10

The new deadline test requires a real Hypertext Transfer Protocol version 2 (HTTP/2) write to block, but gives no input or staging that puts a valid stored command behind flow control. The inherited test uses a fake hook and a large unknown field. Enqueue in this plan deliberately drops that field. `.plans/plans/go-s3-server-commands/plan.md:93,172` and `src/server/internal/link/stream_test.go:693,707,727`.

If the test writer sends one real pending restart after Welcome using the inherited receive window, the small frame fits and the deadline branch is never reached. Reusing the oversized fake hook can show the transport wrapper working while bypassing the real send fence and its shorter context.

Specify how the fixture leaves too little response credit for a valid production command after Welcome, using a controlled receive window or preceding valid frames. Require a barrier proving the real send entered the fenced callback before asserting deadline termination. Keep the claim pending, confirm lock release, and require the deadline mutation to fail the test.

### critic-r1 04 | MAJOR | R10

The expiry race in item 8 lacks a fixture transition that lets the real sweep win. A claim requires an age strictly below 23 hours past expiry. The sweep requires an age strictly above 24 hours. Item 8 specifies fixture aging in the send-first branch only. `.plans/plans/go-s3-server-commands/plan.md:110,152,231`.

With a freshly eligible claim paused and the ordinary database clock, an immediate sweep cannot change the row. Waiting for the database clock would take over an hour and would require continued heartbeats to keep the stream open. The test cannot observe the intended order using the stated steps.

In the sweep-first case, age the disposable command past the grace boundary in a separate locked fixture write after observing claim commit, then run the real sweep and release the barrier. Observe the committed expiry and absence of a command frame while the same stream remains live. Keep expiry immutability covered by a separate unmodified fixture.

### critic-r1 05 | MAJOR | R10

The combined clock gate requires a controlled client wall clock and a retry sent within half a second of the server cutoff. The public client fixture offers no clock injection, and the plan leaves both the fixture boundary and the placement of the near-cutoff case unspecified. The prescribed same-stream state check also names only an unspecified age past expiry. `.plans/plans/go-s3-server-commands/plan.md:261,264`, `.plans/plans/go-s3-server-commands/decisions.md:20`, `.plans/plans/go-s3-client-link/plan.md:291` and `.plans/plans/go-s0-contract/plan.md:890,898,905`.

A combined test using only the public entry can exercise the 22-hour-30-minute replay by aging data. It cannot set the machine's wall clock for the fast-clock branches through that entry. If it uses elapsed time near the cutoff, a slow test runner can miss the delivery window and pass the no-late-send assertion without exercising receipt lookup at all.

Assign the fast-clock and delivery-delay cases to named client-owned tests with controlled time and exact expected outcomes, or name an agreed fixture that exposes those controls for the combined suite. State which fixture proves each side of the cutoff. Give the same-stream state test separate ages just past expiry and more than 24 hours past expiry, and name the fixture that puts its first delivery at each age.

### critic-r1 06 | MINOR | C12

The rollout command includes deployed check C12 without setting its required PostgreSQL test setting. The earlier wrapper exports that setting to its own child only. `.plans/plans/go-s3-server-commands/plan.md:244,246`, `deploy/test/checks.sh:636` and `deploy/test/with-postgres.sh:112`.

On a fresh shell with a healthy deployed stack, check C12 fails with a missing-setting diagnostic after the other checks run. The rollout then fails even if the code is healthy.

Give the exact wrapper invocation for the deployed check batch, or run C5 through C11 in the stack and C12 separately through `deploy/test/with-postgres.sh`. State which invocation supplies the test database.

### critic-r1 07 | MINOR | R10

The per-item example invokes Go tests through a name filter but does not emit per-test results. The convention requires proof that each listed test ran without a skip. `.plans/plans/go-s3-server-commands/plan.md:60,124,185,244` and `src/server/internal/store/storetest/postgres.go:26`.

A typo in a test name or a skipped fixture can leave the package command green. The recorded command and its package result do not demonstrate that the required assertion ran.

Run each package with anchored test names and `go test -json`. Require a pass event for each mandatory test and reject a skip or missing event. Keep the names used by the mutation checks in the report.

### critic-r1 08 | MINOR | plan

The load row records 31 first-pass launches and defers checking available capacity until dispatch. It does not say what fits in the current allowance or where the next dispatch boundary falls. `.plans/plans/go-s3-server-commands/plan.md:306,318`.

If the remaining allowance is exhausted before a test writer and its builder can both launch, the plan can start work that cannot reach its per-item gate in that window.

At dispatch, record the measured remaining allowance and which complete items and review seats fit. Give the first deferred item or review an explicit place in the next window. Keep the single-vendor assignment from `.foundry/go-rewrite/tasks/plan-go-s3-server-commands.md:26`.

## Answers to the attack questions

1. Every item names tests. Item 1 needs an explicit link-1 manifest fixture because the usual presence fixture applies the complete embedded manifest (`.plans/plans/go-s3-server-commands/plan.md:90,94`, `src/server/internal/link/presence_test.go:35,52`). Item 2 needs the expected outcome for a claim that never reached dispatch (finding 01). Item 4b needs the response-credit setup and the expected result for that same unsent claim (findings 01 and 03). Item 8 needs the sweep-first age transition, plus a concrete barrier for its held-startup-sweep branch since the inherited application seams cover proof, admission and event acknowledgement (`.plans/plans/go-s3-server-commands/plan.md:231`, `.plans/plans/go-s3-server-stream/plan.md:243`). Nothing found for items 3, 4a, 5, 6 or 7 after checking their named inputs, expected observations and mutation clauses.
2. Nothing found for bare adjectives in the nine acceptance rows (`.plans/plans/go-s3-server-commands/plan.md:74,134,195`). Each row names a test or a contract check. The clock and deployed observations have execution gaps in findings 05 and 06.
3. The result must match the last attempt the server sent in `src/contracts/proto/lgtm/link/v1/commands.proto:67`, while `.plans/plans/go-s3-server-commands/plan.md:101,172` and `.plans/plans/go-s3-server-commands/decisions.md:16` permit finalization before dispatch. The single-vendor execution in `.plans/plans/go-s3-server-commands/plan.md:318` differs from the cross-vendor defaults in `.plans/epics/go-rewrite/epic.md:71,77,92`. The bounded planning assignment explicitly authorizes this exception at `.foundry/go-rewrite/tasks/plan-go-s3-server-commands.md:26`. Choice 3 uses broad wording about the machine's clock in `.foundry/go-rewrite/owner-choices.md:7`. The decision log should cite the frozen split as the authority for server-time admission and machine-time receipt pruning (`.plans/plans/go-s3-server-commands/decisions.md:10`, `src/contracts/proto/lgtm/link/v1/commands.proto:18,26`).
4. All nine items name exact implementation and test files within `src/server/`. The client fixture, combined suite and deployment checks are requests to their owners. The scope checker returned `ok: server/backend`, consistent with `.plans/epics/go-rewrite/epic.md:121`.
5. Nothing found for concurrent file collisions or items marked high. Items 6 and 7 both touch `src/server/internal/machines/records.go`, and item 7 explicitly follows item 6. Item 4b follows 3 and 4a. Item 8 follows 2, 4b, 5 and 7. For staging that cannot run as written, see findings 03 and 04.
6. A fresh checkout needs the merged Part B gate before starting this slice. Without PostgreSQL, the direct tests can skip, the wrapper must provision a database, and the application refuses to listen. A missing or invalid web password also prevents listening. A second start after a clean stop reapplies the same migration manifest safely, while a second simultaneous server is refused. A failed migration before commit leaves link revision 1. After revision 2 commits, an older manifest is refused and a forward repair can start. The plan covers the last two revision states through the upgrade test, and relies on inherited startup diagnostics and checks for the earlier states (`.plans/plans/go-s3-server-commands/plan.md:46,94,126,230,246`, `src/server/internal/app/start.go:137,148,162,182`, `src/server/internal/app/start_test.go:292,463,500` and `src/server/internal/store/storetest/postgres.go:26`).
7. Gates 2 through 5, 8 and 9 have the required artifacts for this phase. Gate 2 has named observations. Gate 3 has no screen change in scope. Gate 4 names and maps the sources. Gate 5 has a filled risk table for each workstream. Gate 8 names each target and neighboring pattern, and spot checks matched `link.Migrations`, `transport.Send` and `Service.ScanClientHarnesses` in the tree (`src/server/internal/link/migrate.go:14`, `src/server/internal/transport/deadline.go:37` and `src/server/internal/machines/records.go:125`). Gate 9 cites current repository rules. Gate 1 has only closed decisions or unanswered decisions with assumptions and dated scope sources, but the owner-confirmed scope of the blanket deadline rule for all transport sends is not recorded (`.plans/plans/go-s3-server-commands/plan.md:159,161`). Gate 6 has verification lines and one rollout order, with the incomplete invocations in findings 06 and 07. Gate 7 is in progress for this round. Gate 10 has work areas, scopes, reasons and a launch count, with the allowance check still pending (finding 08). The epic already links this plan at `.plans/epics/go-rewrite/epic.md:159`.
8. The plan names no model or harness vendor in the sizing fold. The document introduces `WS` before explaining the abbreviation. The overview leaves the actual menu change behind the reference to owner choice 6 (`.plans/plans/go-s3-server-commands/plan.md:18,68,234`, `.foundry/go-rewrite/owner-choices.md:13`). Nothing found for private paths, network addresses or quotations of the owner.

## Post-build checks

- Hold a valid command behind response flow control with a live stream. If the response flush reaches its deadline after marshaling succeeds, the server releases the registry lock, terminates the stream and retains the committed attempt. A new current stream can retry it.
- Drop a successful transport response after the real client has persisted a completed restart. A relaunch returns the receipt under the new attempt, the server stores one final outcome, and the service records one restart.
- Pause the signed-in watch before the first approved report and resume it after a committed same-stream approval. Observe joining first and online after both fresh approved reports at the same generation. Verify that a scan first becomes eligible at that point and that restart eligibility began when the approved stream was live.
- Interrupt startup after migration 2 commits and before the listener opens. The next start with the full manifest acquires the instance lock, keeps the machine records and keyed replies and starts one sweep worker.

## Grade

6 out of 10, with 0 blockers, 5 majors and 3 minors.
