# Evidence for the sprint 4 contract plan

The checkout read on 2026-11-27 was at `622d1241`. A universally unique identifier (UUID) is a 36-character identifier. The protocol comments also refer to Secure Hash Algorithm 256-bit (SHA-256) digests and Unicode Transformation Format, 8-bit (UTF-8).

## Design and scope read

| Source | Anchor and finding |
|---|---|
| `.foundry/go-rewrite/tasks/plan-go-s4-contract.md` and `.foundry/go-rewrite/common-planner.md` | Both were read in full. There was no separate brief. The assignment and approved design fix the work and its four writable files. |
| `AGENTS.md` and `.plans/epics/go-rewrite/epic.md` | The team table and plan entries 12 to 17 give the Contract team its edit paths and five dependents. The existing epic table keeps the new row at `planned` for the orchestrator to link. |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md` | Read in full. The decisions, shared watch, safe retries, link proof, rules 1 to 11, and wave checks C1 to C13 supply the foundation. |
| `docs/specs/2026-11-27-running-an-agent-design.md` | Read in full. The approved sprint 4 sections, checks R1 to R14, open question 2, and the revision 3 boundary set this plan's scope. |
| `docs/specs/2026-11-05-build-loop-design.md` | Phase 4 and Build subitems specify red assertions before code, frozen tests, mutation proof, one writer per file, and merge checks on the exact builder commit. |
| `docs/issues.md` | Issues 1632 to 1635 give the watch, transport, test-fixture and sizing background. Issue 1674 point 4 asks for one acknowledgement per event. |
| `.plans/designs/running-an-agent/round-3-logic.md` and `round-3-fit.md` | The closing lists were read line by line. Decisions 17 to 28 give each item a contract action or handoff. Their later-check labels refer to the preceding draft. |
| `floorplan/plan.json` | The paths for `common.proto`, `session.proto`, `clients.proto`, `projects.proto`, and `src/server/internal/watch/watch.go` were inspected. Its older split-out `repositories.proto` sketch yields to the approved `ProjectService` design. |
| World-class-plan skill and the model sheet | The planning skill and its template and self-check references supply the gates. The model sheet supplies the `contracts` work area and low and medium sizing. |

## Contract files and nearby patterns

| Path | Symbol or finding |
|---|---|
| `src/contracts/proto/lgtm/link/v1/link.proto` | `LinkService.Open`, `Hello`, `Welcome`, `Proof`, and `OpenResponse.event_ack`. The revision text is in the two field comments. The signed proof labels still name `v1`. |
| `src/contracts/proto/lgtm/link/v1/commands.proto` | `Command.action` uses 10 to 12. `CommandResult` has 1 to 3 and reserves number 4 and name `message`. `CommandOutcome` and the life-of-a-command prose show where the old expiry shorthand needs clarification. |
| `src/contracts/proto/lgtm/link/v1/events.proto` | `Event.report` uses 10 and 11. The event order decides by number before content. `EventAck.through` is cumulative. |
| `src/contracts/proto/lgtm/link/v1/folders.proto` | New target. The nearest patterns are `commands.proto: SetState` for a named action and `events.proto: Facts, Harness.version` for a report and optional text. |
| `src/contracts/README.md` and `src/contracts/testdata/{link-frozen,link-codes,link-proof}.json` | The readme's Rules name the current link and the generation command. The freeze record's original revision 1 object occupies line 2 without a comma. The code table records the existing refusal and retry behavior. The proof fixture records the signed bytes. |
| `src/contracts/freeze_test.go` and `scripts/check-go.sh` | `TestLinkIsFrozenAtItsRevision` hashes raw file bytes and discovers the file set. Its declaration assertions read `link.proto` and the readme. `TestFreezeCheckFailsClosed` uses the real script in a scratch repository. At `link_record` and `link_base`, the script compares frozen record lines by content and number. |
| `src/contracts/{open,envelopes,describe,rules}_test.go` | `TestEveryLinkNameIsDescribed` has a named set of description files. `TestEveryLinkRuleIsWritten` enumerates the three existing sources. `matchDescription`, `newMessage`, `TestLinkNumbersAreWritten`, and the proof assertions provide existing paths for reflection, rule and compatibility tests. |
| `src/contracts/testdata/golden/{open,commands,events,folders}.txt` and `src/contracts/testdata/link-v2-limits.json` | The first three are existing descriptions consumed by `matchDescription`. New `folders.txt` follows `events.txt`. The new numeric fixture follows the data-driven role of `link-codes.json`. Boundary numbers come from the approved size table. |
| `src/contracts/proto/lgtm/v1/{session,clients,common}.proto` | `GetServerResponse` has five fields. `ClientService`, `WatchClientsResponse`, `ClientList`, and `FeedPosition` give the package, machine naming and watch shapes. |
| `src/contracts/proto/lgtm/v1/projects.proto`, `src/contracts/projects_test.go`, `src/contracts/testdata/golden/projects.txt`, and `src/contracts/service_discovery_test.go` | New targets. The service follows `clients.proto: ClientService, WatchClientsResponse` and `common.proto: FeedPosition`. The tests follow descriptor and fixture patterns in `envelopes_test.go`, `describe_test.go`, and `testdata/golden/common.txt`. Discovery uses the existing `session.proto: GetServerResponse` descriptor. |
| `src/contracts/watchshape_test.go` | `watchMethods` enumerates methods from registered descriptors. `TestEveryWatchCallHasTheSharedShape` fixes the tag and type shape and asserts that it found the existing Machines watch. Only Map and Previews are exempt. |
| `src/contracts/{buf.yaml,buf.gen.yaml,package.json}` | `lint`, `plugins`, `exports`, and `scripts.check` select checked-in JavaScript and TypeScript declarations, Go messages, and generated Connect handlers. New generated paths follow the folder and project sources in the same pattern as `events.pb.go`, `clients_pb.js`, and `clients.connect.go`. Generation cleans the output directory and the gate checks tracked and untracked differences. |

## Existing program boundaries

| Path | Symbol or finding |
|---|---|
| `src/server/internal/link/handshake.go` | `LinkRevision`, `errLinkRevision`, and `Service.handshake` send Challenge before lookup and revision refusal. The code maps the mismatch through `openStatus` and `linkRefusal` to the existing metadata reason. The proof text takes the old labels. |
| `src/server/internal/link/{service,presence,stream,inbox}.go` | `Service`, `State.BecomeCurrent`, `Service.serve`, `Service.dispatch`, `State.ApplyEvent`, and `State.Forget` anchor admission, send scheduling, current-stream validation, durable event marks, acknowledgement, and removal. `ApplyEvent` already distinguishes an applied or consumed next number from a repeat. |
| `src/server/internal/link/{outbox,outbox_result,outbox_claim,outbox_send,outbox_sweep}.go` | `Enqueue`, `commandAction`, `AcceptResult`, `claimNext`, `SendNext`, and `SweepCommands` have the current ordering and receipt behavior. The result currently completes the row directly. Its match includes the machine, last sent stream, dispatched attempt, and open outcome. `SendNext` uses the shared slots and a cursor. The normal sweep records `timed_out`. |
| `src/server/internal/api/{session,server}.go` and `src/server/internal/transport/link_stream.go` | `ContractRevision` is 1. `sessionService.GetServer` returns no service inventory today. The API router explicitly registers procedures, and `linkReadLimit` and `WithLinkStream` impose existing transport bounds. |
| `src/server/internal/{watch/watch.go,machines/watch.go,machines/records.go,store/receipts.go,store/prune.go}` | `watch.Serve` and `machines.Service.WatchClients` provide authenticated snapshots and saved or removed records. `recordRequest` and `recordError` use `store.RequestKeyHeader` and map `ErrRequestKeyConflict` to `already_exists`. `WriteOnce` returns the receipt ahead of the domain callback. `stampReceipts` and `deleteReceipts` show that the generic receipt can be pruned independently of a folder operation. |
| `src/client/internal/link/{conn,session}.go` | `linkRevision`, `openSession`, and `validateWelcome` name the revision on both sides of the handshake. `Run`, `liveSession.serve`, and `stopForStore` own the process, listen for acknowledgements, and stop on local faults. `maxUnackedEvents` is 32. |
| `src/client/internal/link/{store,outbox,replay,receipts,commands}.go` | `OpenStore` refuses missing state. `inspectBuckets` checks a contiguous retained suffix ending at next minus one. `Store.Append` advances next with the event, and `Store.Ack` rejects a mark at or ahead of next. `newReplay` and `replay.next` handle Welcome and the persisted payloads. `readReceipt`, `findReceipt`, `savedOutcome`, and `finishCommand` show that revision 1 persisted the started or finished state but no event watermark. |
| `src/frontend/src/server/{api,sync}.ts` and `src/mock-server/src/{engine,serve}.ts` | `openServer` checks website revision `CONTRACT=1` and chooses the in-page router or the Connect transport. `watch` replaces the Machines snapshot and `changeServer` stops the old watch. The in-page sample and mock share `createEngine`, which returns `sample=true` with its own `CONTRACT=1`. |

## Probes run while planning

- `git rev-parse --short HEAD` returned `622d1241`. Git confirmed `fcfa96c3` is an ancestor and that the current revision 1 record matches that commit. The repository is not shallow.
- The actual comparison program was extracted from `scripts/check-go.sh` and run against a valid synthetic two-entry record. Leading-comma append against revision 1 returned 0. Adding the comma to line 2 or moving line 2 returned 1. The intact record against revision 2 returned 0, and rewriting its revision 2 hash returned 1.
- `go test ./src/contracts -count=1` passed with a writable temporary Go build cache. The first run used the default read-only cache and failed in the scratch-module test. This retry verified the revision 1 baseline suite, including the real history script.
- `pnpm --filter @lgtm/frontend typecheck` and `pnpm --filter @lgtm/mock-server typecheck` each exited 0 on the starting checkout. These are pre-change compatibility probes.
- `scope.py check . contract/contracts` returned `ok: contract/contracts`. The scope source is `CLAUDE.md`, which imports `AGENTS.md`.
- The privacy script exited 0 on the existing tracked set. The completion check below covers the new plan files and ignored handoff as well. No builder changes or runtime integration tests were run during planning.

## Tier M self-check

| Gate | Observation |
|---|---|
| 1. Decisions | All 30 rows have a dated rationale and a closed or unanswered status. Each unanswered row states its assumption. The scope rows reference the owner-approved sources and dates. |
| 2. Acceptance | All three workstreams name observed tests, command exits, or actions in a browser. All four items identify red tests and a mutation. |
| 3. Design artifact | The approved design supplies the text brief. The contract plan adds no screen or hierarchy. |
| 4. Traceability | The mapping names the source universe. All 14 new checks, applicable foundation statements, and both closing lists map to a workstream, a decision, or a downstream gate. |
| 5. Risks | Each workstream has risks, severities, and a mitigation tied to an assertion or a rollout action. |
| 6. Verification | Each workstream names its test and environment. A single rollout order covers the changing baseline and deployment. |
| 7. Adversarial review | Pending at the authorized handoff. The plan has an attack log. The orchestrator owns the review and its findings files. |
| 8. Anchors | The task anchors match the file and symbol inventory above. New files give existing neighbors. The revision check, history test, shared watch test and server admission were checked against source. |
| 9. Guardrails | The table covers the repository boundary, contract source, notices, issue tracking, integration, and privacy. The assignment limits this write to the named artifacts. |
| 10. Sizing | The live scope check passed. Every unit has one work area, one scope, and one low or medium size. The launch count is given for one vendor. The remaining vendor allowance is unverified and is checked at launch. |

The plan has the required frontmatter, section order, divider and five folds. The owner sections explain the feature and the recovery risk before the execution details. The epic path and plan row exist. The orchestrator owns the change from `planned` to `linked` under the assignment's file limit.
