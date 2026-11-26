# Scouting the client link and its acceptance checks

This is the planning evidence from 2026-11-26. The Sprint 0 and Sprint 1 plans were still in review. Their planned interfaces are identified below. The second-round instruction of 2026-11-26 supersedes the old client-supplied event mark.

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
| `.plans/plans/go-s0-contract/draft/events.proto` and `commands.proto` | `Event`, `EventAck`, `Facts`, `HarnessesFound`, `Heartbeat`, `Command`, `CommandResult` and `MachineState`. The producer timestamp is display data. The command draft specifies the elapsed time calculation from the welcome sample. |
| `.plans/plans/go-s1-client-local/plan.md` | "The layout of the state folder", items 1 through 7 and "What plan go-s3-client-link must do". The later plan has `state.Folder.Paths()`, `IdentityFile`, `StoreFile` and `link.OpenStore`. Its `MarkSent`, `Reconcile`, `Begin` and `Finish` methods govern replay. The caller prose still mentions the client-supplied mark removed by the later instruction. |
| `.plans/plans/go-s3-server-machines/plan.md` and `.plans/plans/go-s3-stack-client/plan.md` | The Server brief assigns the matching proof, event and command handling. The Stack plan's exit-code agreement and `live_reconnect_restarts_service` supply the service observation. |
| `deploy/test/README.md` and `deploy/test/checks.sh` | Read the script and assumed names. `client_join` and `start_client` set `XDG_STATE_HOME`. `refused_join` and `wait_refusal` read fixed messages. `check_c10` changes `identity.json`. `client_sequences`, `server_sequences` and `check_c11` compare at least three outage sequences in order. |
| `src/frontend/src/screens/service.tsx` | The `Add a machine` dialog near line 410 shows the previous installer and command. The machine action near line 351 offers `ReconnectClient` only if offline. |
| `src/contracts/proto/lgtm/v1/clients.proto` | `ClientService.ReconnectClient`, `Client.address`, `Client.hosts_server` and `CreateClientTokenResponse` anchor the website handoff. The current response has `token_id`, `token` and `expires_at`. |
| `floorplan/plan.json` | The neighboring entries "Joining a server", "Connection to the server", "Events waiting to be sent", "Machine settings" and "Machine facts and load" place the work. The connection entry uses the older `Connect` procedure spelling. |
| Planning skill and model sheet | Read the planning skill, template, self-check probes and the work area and complexity rows. `backend` and `client/backend` are the applicable values. |

## Greenfield anchors for the work items

There is no `src/client/` or `src/server/` implementation directory in this checkout yet. The targets below are future paths. Patterns identify a current file and symbol or section, or a planned dependency when the pattern is the direct build prerequisite.

| Item | File and planned anchor | Nearest pattern or dependency |
|---|---|---|
| 1 | `src/client/internal/transport/transport.go`, `ParseOrigin`, `NewHTTPClient` | Sprint 0 `draft/link.proto`, `JoinToken.address` and `certificate_key_sha256`. The cached Go 1.26 source has `http.Protocols.SetHTTP2` and `SetUnencryptedHTTP2`. |
| 2 | `src/client/internal/enroll/join.go`, `Join`, `LoadServer` | `floorplan/plan.json`, "Joining a server". Sprint 1 `Credential`, `Folder.Paths()`, `WriteAtomic` and `SyncDir`. `deploy/test/checks.sh`, `check_c10`, fixes the identity names. |
| 3 | `src/client/internal/link/conn.go`, `openSession`, `proofText` | Sprint 0 `draft/link.proto`, `LinkService.Open` and `Proof`. The proof tests will use the contract fixture `src/contracts/testdata/link-proof.json` once generated. |
| 4 | `src/client/internal/link/replay.go`, `replay` | Sprint 1 `src/client/internal/link/outbox.go` target with `Store.Read`, `Store.MarkSent`, `Store.Ack` and `Store.Reconcile`. `deploy/test/checks.sh`, `client_sequences`, reads the durable observation. |
| 5 | `src/client/internal/link/report.go`, `reporter`, `captureFacts`, `scanHarnesses` | Sprint 1 `sysinfo.ReadFacts`, `sysinfo.ReadLoad` and `harness.Scan`. Sprint 0 `draft/events.proto`, `Facts` and `HarnessesFound`. |
| 6 | `src/client/internal/link/commands.go`, `commandHandler`, `serverClock` | Sprint 0 `draft/commands.proto`, `Command`, `CommandResult` and `MachineState`. Sprint 1 `Store.Begin` and `Store.Finish` in the receipt item. |
| 7 | `src/client/internal/link/retry.go` and `session.go`, `Run), `RunOptions` | `floorplan/plan.json`, "Connection to the server". Sprint 0 `draft/link.proto`, `LinkService` code table and `Open` timeout comments. |
| 8 | `src/client/internal/app/client.go`, `Join`, `Run` | Sprint 1 `config.StateDir`, `state.Open` and the package main `run` stub. Stack Client's plan names the service outcomes. |
| 9 | `src/client/cmd/lgtm-client/main.go`, `runContext`. `src/client/clienttest/clienttest.go`, `Options`, `Join` and `Run`. | Sprint 1 package main's `run` and `version`. Issue 1634 and the epic's in-process test rule supply the package boundary. |

## Checks run during planning

- Read the files and searched the named declarations above. The three spot checks were `LinkService.Open` in the draft contract, `check_c10` in the stack script and the old join command in the Machines dialog. Each file and anchor was present.
- The active `go version` reported Go 1.23.3. The local Go 1.26.0 toolchain source exposes the two HTTP/2 protocol setters used by the plan. Sprint 0 owns the module toolchain selection.
- `bash -n deploy/test/checks.sh` passed. The full script needs the built programs and disposable stack. It is a build acceptance command.
- `scope.py check . client/backend` reported `ok: client/backend`. The orchestrator maintains that scope.

## Tier L self-check

| Gate | Evidence and result |
|---|---|
| 1. Decisions | The 24 rows have status `closed` or `unanswered`. Every unanswered row names its assumption. Row 6 records the owner's sprint confirmation and supersedes row 5. |
| 2. Acceptance | The nine done criteria each name a test, command or observed stack step. Every item names a first failing assertion and a verification command. |
| 3. Design artifacts | The three workstreams point to the approved design. The client work changes no screen hierarchy. The result carries the website copy request. |
| 4. Traceability | The declared inputs and all 23 decisions, 11 rules and 13 checks map to the workstreams, another team or a later wave. |
| 5. Risks | The three workstreams each have risk, severity and mitigation rows. All nine risky items name a mutation. |
| 6. Verification | Each workstream lists its gating tests and environment checks. There is one plan-level rollout sequence. |
| 7. Adversarial review | The plan is ready for round 1. The bounded assignment delegates findings and their resolution to the orchestrator. |
| 8. Anchors | Nine task rows have matching scouting anchors. The three existing spot checks passed. |
| 9. Guardrails | The table cites the repository's ownership, credits, issue, integration and writing rules. |
| 10. Sizing | The scope check passed. All nine rows are low or medium, with a reason, prerequisites and attack flag. The estimate gives per-vendor launches. Live allowance verification belongs to the orchestrator at launch, as the epic specifies. |
| Format and linkage | The frontmatter, four top-level sections, divider, five folds and linked epic row passed. The epic remains active. |

The stock Node invocation hit `spawnSync git EPERM` while creating an unused standard-input pipe for Git in this runner. A probe showed that Git ran successfully with standard input ignored. The same privacy checker ran with only that subprocess option adjusted at runtime. A second scan applied the same rules to the four deliverables, including the untracked files. The deliverables were also read for full sentences, expanded terms, relative paths and dated decisions.
