# Scouting the client link and its acceptance checks

This records planning, prerequisite reconciliation and three review folds on 2026-11-26. The anchors distinguish built code from planned work.

## Requirements and existing repository anchors

| Read | Anchor and observation |
|---|---|
| `AGENTS.md` | "What this project is" defines the three parts. "Where things are" keeps new code out of the old engine. "Habits" and "Writing style for documents" govern the plan and the result. |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md` | Read in full. Decisions are the rows at lines 29 through 51. "Joining" at lines 243 through 256 supplies the command and trust exchange. "The link between the server and a machine" at lines 258 through 339 supplies proof, command, replay, presence and reporting behavior. Rules 1 through 11 are at lines 362 through 372. Checks 1 through 13 are at lines 378 through 390. |
| `.plans/epics/go-rewrite/epic.md` | "The sprints", "The teams, and what each may edit", "Plans" and "Tests that are written before any plan". The owner confirmed the sprint cut. The linked row assigns the plan and the epic assigns the combined tests to Sprint 3. |
| `.foundry/go-rewrite/common-planner.md` and `.foundry/go-rewrite/tasks/plan-go-s3-client-link.md` | Read in full. They limit writes to this plan's folder and one result file. The planner hands off at the adversarial review phase. Read the original brief and its five decision rows before rewriting it. |
| `docs/specs/2026-11-05-build-loop-design.md` | "4 Build" and "Build subitems". The first test run fails on an assertion. The lead checks the immutable commit and the mutation for each risky subitem. |
| `docs/issues.md` | Issues 1632 through 1640 at lines 7501 through 7589. They record encryption termination, in-process testing, size, command copy, address, host role, key rotation and runner access. |
| `.plans/plans/go-s0-contract/plan.md` | Public names and work items 4 to 6, including the two golden fixtures and both proof strings. Read the contract result's Client team requests and the first-round fold. |
| `.plans/plans/go-s0-contract/draft/link.proto` | `LinkService`, `JoinToken`, `JoinRequest`, `JoinResponse`, `Hello`, `Challenge`, `Proof` and `Welcome`. The later draft removes the client acknowledgement, leaves token expiry to the server, changes the terminal refusal and has `Welcome.server_time` at field 4. |
| `.plans/plans/go-s0-contract/draft/events.proto` and `commands.proto` | `EventAck.through` is the server's mark and `Event.happened_at` is display data. `CommandResult` has three fields. `SetState` uses revisions and applies through expiry. The contract supplies the welcome time sample. |
| `.plans/plans/go-s1-client-local/plan.md` | Items 1 to 6 fix the state setting, private folder, credential and store interfaces. The caller fold in round 4 names `CreateStore`, `OpenStore`, `Read`, `Ack`, `Begin` and `Finish`. Finished receipts age by the machine's wall clock. |
| `.plans/plans/go-s3-server-machines/plan.md` and `.plans/plans/go-s3-stack-client/plan.md` | The Server brief assigns the matching proof, event and command handling. The Stack plan's exit-code agreement and `live_reconnect_restarts_service` supply the service observation. |
| `deploy/test/README.md` and `deploy/test/checks.sh` | Read the script and assumed names. `client_join` and `start_client` set `XDG_STATE_HOME`. `refused_join` and `wait_refusal` read fixed messages. `check_c10` changes `identity.json`. `client_sequences`, `server_sequences` and `check_c11` compare at least three outage sequences in order. |
| `src/frontend/src/screens/service.tsx` | The `Add a machine` dialog near line 410 shows the previous installer and command. The machine action near line 351 offers `ReconnectClient` only if offline. |
| `src/contracts/proto/lgtm/v1/clients.proto` | `ClientService.ReconnectClient`, `Client.address`, `Client.hosts_server` and `CreateClientTokenResponse` anchor the website handoff. The current response has `token_id`, `token` and `expires_at`. |
| `floorplan/plan.json` | The neighboring entries "Joining a server", "Connection to the server", "Events waiting to be sent", "Machine settings" and "Machine facts and load" place the work. The connection entry uses the older `Connect` procedure spelling. |
| Planning skill and model sheet | Read the planning skill, template, self-check probes and the work area and complexity rows. `backend` and `client/backend` are the applicable values. |

## Existing dependencies and client-link work item anchors

The repository has the Sprint 1 client program, state, key, settings, facts, harness scan and store sources. The link and application targets below name planned work. A dependency on another plan remains a build gate until its interface is agreed and available.

| Item | File and planned anchor | Nearest pattern or dependency |
|---|---|---|
| 1 | `src/client/internal/transport/transport.go`, `ParseOrigin`, `NewHTTPClient` | Sprint 0 `draft/link.proto`, `JoinToken.address` and `certificate_key_sha256`. The cached Go 1.26 source has `http.Protocols.SetHTTP2` and `SetUnencryptedHTTP2`. |
| 2 | `src/client/internal/enroll/join.go`, `Join`, `SaveServer`, `LoadServer` | `floorplan/plan.json`, "Joining a server". Sprint 1 `Credential`, `Folder.Paths()`, `WriteAtomic` and `SyncDir`. `deploy/test/checks.sh`, `check_c10`, fixes the identity names. |
| 3 | `src/client/internal/link/conn.go`, `openSession`, `proofText` | Sprint 0 `draft/link.proto`, `LinkService.Open` and `Proof`. The proof tests will use the contract fixture `src/contracts/testdata/link-proof.json` once generated. |
| 4 | `src/client/internal/link/replay.go`, `newReplay`, `replay.next` and `replay.ack` | Built `src/client/internal/link/outbox.go` exposes `Store.Read`, `Store.Ack`, `Store.Next` and `Store.Changed`. `deploy/test/checks.sh`, `client_sequences`, reads the durable observation. |
| 5 | `src/client/internal/link/report.go`, `reporter`, `captureFacts`, `scanHarnesses` | Sprint 1 `sysinfo.ReadFacts`, `sysinfo.ReadLoad` and `harness.Scan`. Sprint 0 `draft/events.proto`, `Facts` and `HarnessesFound`. |
| 6 | `src/client/internal/link/commands.go`, `commandHandler`, `serverClock`. `src/client/internal/link/receipts.go`, `Store.Decline` | `commands.proto`, `Command` and `CommandResult`. Item 6 extends the built `Store.Begin` and `Store.Finish` receipt pattern. Its tests extend `receipts_test.go`. |
| 7a | `src/client/internal/link/retry.go`, `supervise` | Sprint 0 `link.proto`, the `LinkService` code table and `Open` handshake deadlines. Item 3 owns the error classification. |
| 7b | `src/client/internal/link/session.go`, `Run`, `RunOptions` | `floorplan/plan.json`, "Connection to the server". `deploy/test/checks.sh`, `wait_refusal` and `check_c10`, fixes the diagnostics. |
| 8 | `src/client/internal/app/client.go`, `Join`, `Run` | Sprint 1 `config.StateDir`, `state.Open` and the package main `run` stub. Stack Client's plan names the service outcomes. |
| 9 | `src/client/cmd/lgtm-client/main.go`, `runContext`. `src/client/clienttest/clienttest.go`, `Options`, `Join` and `Run`. | Sprint 1 package main's `run` and `version`. Issue 1634 and the epic's in-process test rule supply the package boundary. |

## Checks run during initial planning

- Read the files and searched the named declarations above. The three spot checks were `LinkService.Open` in the draft contract, `check_c10` in the stack script and the old join command in the Machines dialog. Each file and anchor was present.
- The active `go version` reported Go 1.23.3. The local Go 1.26.0 toolchain source exposes the two HTTP/2 protocol setters used by the plan. Sprint 0 owns the module toolchain selection.
- `bash -n deploy/test/checks.sh` passed. The full script needs the built programs and disposable stack. It is a build acceptance command.
- `scope.py check . client/backend` reported `ok: client/backend`. The orchestrator maintains that scope.

## Tier L self-check

| Gate | Evidence and result |
|---|---|
| 1. Decisions | The 62 rows have status `closed` or `unanswered`. Every unanswered row names its assumption. Rows 41 and 42 name the owner gates. |
| 2. Acceptance | The nine done criteria each name a test, command or observed stack step. All ten build items name a first failing assertion and a verification command. |
| 3. Design artifacts | The three workstreams point to the approved design. The client work changes no screen hierarchy. The result carries the website copy request. |
| 4. Traceability | The declared inputs and all 23 decisions, 11 rules and 13 checks map to the workstreams, another team or a later wave. |
| 5. Risks | The three workstreams each have risk, severity and mitigation rows. All ten risky items name a mutation. |
| 6. Verification | Each workstream lists its gating tests and environment checks. There is one plan-level rollout sequence. |
| 7. Adversarial review | The registers account for every finding. Round 3 has 10 findings in nine groups worth 17 points. The plan remains in `attack`. |
| 8. Anchors | Ten task rows have matching scouting anchors. The contract, receipt store and stack script checks below anchor the changed work. |
| 9. Guardrails | The table cites the repository's ownership, credits, issue, integration and writing rules. |
| 10. Sizing | The scope check passed. The ten low or medium rows have reasons, prerequisites and attack flags. The orchestrator owns launch allowances. |
| Format and linkage | The frontmatter, four top-level sections, divider, five folds and linked epic row passed. The epic remains active. |

The stock Node invocation hit `spawnSync git EPERM` during the first fold. The scan uses the same rules with Git's unused standard input ignored. The deliverable scan also covers the untracked register and ignored result file.

## Prerequisite reconciliation

| Read | Anchor |
|---|---|
| The Contract rounds 3 and 4 and Client Local round 4 requests | The Client Local ruling accepts machine clock skew in pruning. `commands.proto` still states the full retention window. The fold result carries the wording request to Contract. |
| Sprint 1 server store and vault | The server store classifies an indeterminate transaction as unavailable. The vault gives the signing key to the server. The client uses the generated link for both proofs. |
| Sprint 2 server sign-in and Stack Web | Sign-in item 8 supplies `servertest.Start`, its bound endpoint and restart fixture. The stack uses cleartext HTTP/2 on loopback and reserves a public encrypted endpoint for later. |
| Sprint 3 Stack Client and the wave 1 script | The service policy restarts nonzero run exits. The current Stack Client plan names `identity.json`. The script still uses the earlier state setting and derives the path. The result carries the implementation dependency. |
| The epic and issues 1641 and 1642 | The Sprint 3 link plans share the freeze gate. The first importer owns a module change. Restored state and live copies are outside wave 1. |

The elapsed-clock targets are `src/client/internal/link/clock_linux.go` and `clock_darwin.go`. The dependency provides `unix.ClockGettime` on both platforms. Linux `CLOCK_BOOTTIME` counts suspend time, and Apple uses its continuous clock for `CLOCK_MONOTONIC_RAW`. The planned sign-in dependency selects the same module version used here. [Linux clock documentation](https://www.man7.org/linux/man-pages/man3/clock_gettime.3.html), [Apple clock implementation](https://github.com/apple-oss-distributions/Libc/blob/main/gen/clock_gettime.c), [Go cryptography module requirements](https://github.com/golang/crypto/blob/v0.59.0/go.mod).

The Contract round 4 result, drafts and post-build checks 16, 18 to 20 and 24 anchor the follow-up. The pinned Connect library's `error.go` documents `(*Error).Meta()` as exposing the metadata decoded from protocol headers, trailers or the stream body. The local-state plan's `Begin` prefers existing receipts, returns `DecisionExpired` without a write for a new expired command and lets `Finish` store the final result of a reserved command.

## Code and contract checked for the first review fold

| Anchor | Observation |
|---|---|
| `src/client/internal/config/config.go`, `src/client/internal/state/` and `src/client/internal/enroll/` | The configuration and state code provides `config.StateDir`, `LoadSettings`, `state.Open`, `WriteAtomic` and `SyncDir`. The credential package provides `Load` and `Credential.Sign`. |
| `src/client/internal/sysinfo/` and `src/client/internal/harness/` | The facts and load readers are `ReadFacts` and `ReadLoad`. The scanner exposes `Definitions`, `Scan` and `DefaultScanOptions`. |
| `src/client/internal/link/receipts.go`, `store.go` and `outbox.go` | `Begin` checks existing receipts before expiry. An absent expired command writes nothing. `Finish` requires a running receipt and prunes by `wallClock`. The current store has no atomic decline call. `Read`, `Ack`, `Next` and `Changed` are the outbox interfaces. |
| `src/contracts/proto/lgtm/link/v1/{link,events,commands}.proto` and the Contract drafts | The verified challenge precedes a trusted refusal. A welcome carries time and the committed event number after proof. An event gap ends one stream. An approved welcome requires fresh facts and inventory. A standing update is answered with done irrespective of expiry. |
| `deploy/test/checks.sh`, `client_join`, `start_client`, `check_c10` and `check_c11` | The state exports still use `XDG_STATE_HOME`. C10 reads `msg=link_refused` and the two refusal codes. C11 keeps the same process, observes three durable events and checks that each number was applied once in order. |
| `src/server/internal/store/storetest/postgres.go`, `scripts/check-go.sh` and the Stack Web plan | The fixture skips explicitly if `LGTM_TEST_POSTGRES` is unset. The root check runs `go test ./...`. Stack Web owns the existing long-stream soak command. |
| Connect 1.21.1 and the `golang.org/x/net` v0.63.0 module metadata | The client option `WithReadMaxBytes` caps each received message. The Server Machines cap applies to requests. The network module selects `golang.org/x/sys` v0.50.0. |

## Evidence for the second review fold

| Anchor | Observation |
|---|---|
| `src/client/internal/link/receipts.go:65`, `:98`, `:111` and `:137` | `Begin` checks the receipt first. An absent expiry refusal writes nothing, while a runnable command commits and enters `s.running`. `Finish` prunes by `wallClock`. Client Local decision 84 already supersedes the proposed clock field in row 69. |
| `src/client/internal/link/store.go:74`, `:117` and `:386` | Tests can fail the commit. A missing store exits `OpenStore` without creating one. A failed commit fences subsequent writes. `receipts_test.go`, `TestFinishPrunesByAge`, already uses the machine clock seam. |
| `src/contracts/proto/lgtm/link/v1/commands.proto:88` | Scan done requires the command's `HarnessesFound` on disk. A newer inventory can make an old scan stale without satisfying that condition. |
| `src/contracts/proto/lgtm/link/v1/link.proto`, `OpenResponse` and `Welcome`, and the Server Machines plan | The current field comments leave the size assumptions implicit. The Server plan's `MaxNameBytes`, `NormalizeName` and `BecomeCurrent` give a 64-byte name and a 43-byte approved token. Their largest planned known-field reply is a 161-byte wrapped welcome. |
| `deploy/test/checks.sh:219`, `:233`, `:344` and `:521`, and Stack Client plan items 2 and 3 | The script still exports the earlier state setting and expects a private existing identity under the fixture. The Stack plan already owns the setting fix. Both service templates retry nonzero run statuses. The stopped-unit installer output reads the journal first and mentions rejoin for removal. |

## Evidence for the third review fold

| Anchor | Observation |
|---|---|
| `src/client/internal/link/store.go:110`, `:247` and `:353` | Open normalizes access and format errors to `ErrStoreDamaged` unless they match a specific exception. The embedded database can also report invalid metadata when its read fails. The fold uses a retry exit for all open failures. |
| `src/client/internal/link/outbox.go:55` and `:77`, and `src/contracts/proto/lgtm/link/v1/events.proto:9` | Reads return database errors. Acknowledgement rejects a mark at or above `Next()`. The server may answer as soon as it commits, before the client send returns. The new session test holds the send there. |
| `src/client/internal/link/receipts.go:65` and `:98`, and `src/contracts/proto/lgtm/link/v1/commands.proto:51` and `:88` | Existing receipts take precedence. `Begin` at expiry leaves an absent receipt absent. A requested scan earns done when its own event commits, and later attempts can replay the stored result. |
| Stack Client plan, service outcomes, installer step 17 and `live_removed_machine_stays_stopped` | The service manager retries nonzero exits. The installer directs the operator to the journal. Check the existing local-fault and removal diagnostics in the installed service. |
| Server Machines plan, `MaxNameBytes` and `BecomeCurrent` | The planned server has a 64-byte name and 43-byte approved token. These supply the welcome fixture. No new dependency on server internals or a contract annotation is required. |
