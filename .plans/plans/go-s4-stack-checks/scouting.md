# Scouting the sprint 4 stack checks

## Sources and limits

- Read `AGENTS.md`, the task, the complete shared planner instructions, and the foundation and running-an-agent designs. The newer design's Sprint 4 link, folder, website and proof sections and its Sprint 5 dependencies set the boundary.
- Read the Go rewrite epic's Sprint 4 row, team table, Phase 4 and test ownership. Read Phase 4 of the build loop and issues 1632 through 1635. The stack owns deployment. The orchestrator owns the paired-program test folder and the independent acceptance driver.
- Read the contract plan's freeze, revision 2 handshake, Projects routes and post-build checks. Read the round-two fold's two binding tables and the third logic and fit reviews' closing lists.
- Read the relevant rollout, verification and acceptance sections of `go-s1-stack-vault`, `go-s2-stack-web` and `go-s3-stack-client`. Read their last review registers and relevant post-build findings. The vault review calls for observations beyond fixture success text. The web review found the served browser could differ from the image history. The installer reviews require one Compose selector, real process ownership and teardown that waits for fixture children.
- Located `floorplan/plan.json` as the existing whole-rewrite map. Used the named live deployment and contract files for implementation anchors. Read the world-class-plan template and self-check, the work-area sheet and verification-before-completion. The parallel Sprint 4 plan drafts are outside this scout.

## Existing deployment anchors

| File and symbol or section | What the code gives the plan |
|---|---|
| `deploy/compose.yaml`, `services.server`, `services.web`, `volumes.pgdata` | The server and web images share `LGTM_IMAGE_TAG`. Only web publishes a port and the host binding is loopback. The database volume is project-scoped. OpenBao uses its existing database and state mounts. |
| `deploy/up.sh`, `compose`, `preflight`, `build_images`, `start_server`, `start_web`, `find_address` | The launcher respects an explicit file list, snapshots settings privately, locks stack state, builds the images, waits for server health, and then serves the website. Its normal path reuses the named volume and state. |
| `deploy/server.Dockerfile`, `build` and `runtime`. `deploy/server.Dockerfile.dockerignore`, server and generated Go allow-list | The normal binary build embeds server internals and consumes the generated Go code. Projects fits those inputs. |
| `deploy/website.Dockerfile`, `site`. `deploy/Caddyfile`, `@server` and `reverse_proxy` | The deployed build fixes the browser backend to `/`. The existing matcher passes the application service namespace and the machine link to the same upstream over cleartext Hypertext Transfer Protocol version 2. |
| `deploy/test/checks.sh`, `compose`, `client_join`, `start_client`, `check_c11`, `check_c12`, `run_check` | The wave 1 driver supplies Compose, private per-client state, an event-application observation and one result line for each selected check. It is the pattern for the orchestrator's separate Sprint 4 driver. |
| `deploy/test/with-postgres.sh`, `admin_sql` and `LGTM_TEST_POSTGRES` | The wrapper publishes a disposable PostgreSQL container on an assigned loopback port, supplies a real database to one child, propagates its status and cleans up. |
| `deploy/test/web-testlib.sh`, `web_fixture_create`, `web_fixture_override`, `web_fixture_cleanup`. `deploy/test/stack-web.sh`, `web_publishes_on_loopback_only` | The test model already has precedents for private settings, project ownership, images, loopback port selection and scoped cleanup. |
| `deploy/install-client.sh`, `main`, `join_stack`, `install_systemd_unit`, `install_launch_agent` | An existing saved identity skips join. An installed service supplies its recorded state path. The installer builds a binary from the current checkout, reinstalls the template and restarts the service. |
| `deploy/lgtm-client.service`, `Restart` and `RestartSec`. `deploy/dev.lgtmstudio.client.plist`, `KeepAlive.SuccessfulExit` and `ThrottleInterval` | A nonzero exit may restart. A zero exit stops. Both configured intervals are five seconds. |
| `deploy/test/installer.sh`, `rerun_scenario`, `agent_text_equals_template`, `plist_checks`. `deploy/test/installer-live.sh`, `live_reconnect_restarts_service`, `live_removed_machine_stays_stopped`, `unit_owned_by_fixture` | Existing stand-in tests cover installation and rerun. The live Linux script supplies the ownership checks, state mode checks and process evidence pattern. Its pending Sprint 3 run is an independent matter. |
| `deploy/README.md`, “Stop the stack and keep its data”, “Settings”, “Installing the client”. `deploy/test/README.md`, “Run C12 with disposable PostgreSQL” | The operator guide describes the persistent installation and existing installer. The test guide describes the real database wrapper and driver output. |

## Application and test harness anchors

| File and symbol or section | What the code gives the plan |
|---|---|
| `src/client/cmd/lgtm-client/main.go`, `runContext`, `main`. `src/client/internal/app/client.go`, `Run`. `src/client/internal/app/client_test.go`, `TestAppRunStoreStatus` | The executable passes the application status to the operating system. Existing clean stops exit 0 and requested restarts exit 75. Existing tests distinguish fatal marks from retryable faults. |
| `src/client/internal/state/folder.go`, `PathsIn`. `src/client/internal/config/config.go`, `StateDirVariable`, `DefaultSettings` | The installed state names are `machine.key`, `identity.json`, `client.db`, and optional `settings.json`. The default repository folder belongs to the state root. |
| `src/client/internal/link/store.go`, `OpenStore`, `metaBucket`, `eventsBucket`, `receiptsBucket`. `src/client/internal/link/receipts.go`, `readReceipt`, `Finish` | A read-only inspection of the quiesced old database can confirm the old event and finished receipt by identifier. The old binary must write both before the upgrade. |
| `src/client/internal/link/commands.go`, `settleScan`, `reply`. `src/client/internal/harness/scan.go`, `readVersion` | A forced scan writes its receipt before a result is sent. An inert version probe can count real execution. |
| `src/server/internal/link/handshake.go`, `handshake` and `openStatus`. `src/client/internal/link/conn.go`, `openSession` and `receiveError` | The old client verifies the signed Challenge, then recognizes the revision refusal. The server creates a connection only after the supported revision check. |
| `src/server/internal/link/migrations/0001_link.sql`, `link_machines.event_mark`. `src/server/internal/link/migrations/0002_link_commands.sql`, `link_commands`. `src/server/internal/machines/migrations/0001_machines.sql`, `machines`. `src/server/internal/auth/migrations/0001_web_sessions.sql`, `web_sessions` | The old records supply durable observations for the server half of the upgrade. Stable machine, approval and command fields have public or read-only test observations. |
| `src/client/internal/transport/transport.go`, `ParseOrigin`, `NewHTTPClient`, `ErrCertificateKeyMismatch`. `src/client/internal/enroll/join.go`, `decodeJoinToken`. `src/server/internal/link/join.go`, `Join` | The client pins a leaf's public key and permits plain Hypertext Transfer Protocol only on loopback. In the fixture, the server redeems the real token identifier and secret. The endpoint and pin belong to the client-side token metadata. |
| `src/server/servertest/server.go`, `Start`, `LinkSeams`, `Server.Restart`. `src/client/clienttest/clienttest.go`, `Join`, `Run`, `Options` | The named entry points already start real programs within one test process against the real disposable PostgreSQL database. |
| `src/linktest/relay_test.go`, `startRelayWithControls`, `relayControls`. `src/linktest/outbox_test.go`, `TestLinkOutboxReplayAfterServerFailure` pattern. `deploy/test/streamprobe/main.go`, `serve`, `open` | Generated messages and the current streaming handlers let a fixture hold or drop an exact event or result. A Go helper under deployment tests can use that pattern through Caddy. |
| `src/frontend/src/server/address.ts`, `fixedAddress`, `serverUrl`. `src/frontend/src/server/api.ts`, `openServer`. `src/mock-server/src/serve.ts`, `allowed` and `listen`. `src/server/internal/config/config.go`, `LGTM_ALLOWED_ORIGINS` lookup | An unpinned build can choose the in-page sample, network mock or real endpoints. The fixture can permit the exact loopback preview origin. A second deployed check exercises the fixed same-origin image. |
| `scripts/check-go.sh`, `link_record`, `link_base`. `scripts/check-private.mjs`, tracked-file loop | The history comparison uses an explicit base. The privacy command checks tracked files, so the new artifacts receive an extra direct review. |

## New file targets and adjacent patterns

| Target | Existing pattern |
|---|---|
| `deploy/test/sprint4-fixture.sh`, `deploy/test/sprint4-fixture-test.sh` | `deploy/test/web-testlib.sh` for fixture ownership and `deploy/test/checks.sh` for a private state folder per process. |
| `deploy/test/sprint4relay/main.go`, `deploy/test/sprint4relay/main_test.go` | `deploy/test/streamprobe/main.go` for a deployment helper and `src/linktest/relay_test.go` for exact frame control. Reuse generated interfaces and write the deployment relay in this plan's own files. |
| `deploy/test/sprint4-upgrade.sh`, `deploy/test/sprint4-supervisor.sh` | `deploy/test/checks.sh`, `check_c9` and `check_c11`, for before-and-after evidence. Use `deploy/test/installer-live.sh` for fixture ownership and real supervisor observations. |
| `deploy/test/sprint4-compose.sh` | `deploy/test/stack-web.sh`, `config` and `web_publishes_on_loopback_only`. |
| Proposed orchestrator target `deploy/test/checks-sprint4.sh` | `deploy/test/checks.sh`, `run_check`, `usage`. This is a handoff path. |

## Probes run

- `git show f6cbea30:src/contracts/proto/lgtm/link/v1/link.proto` showed both declarations at revision 1. The same commit is the parent of the Sprint 4 approval commit `622d1241`. The freeze baseline `fcfa96c3` serves a different purpose.
- The planning skill's `scope.py check . stack/infra` reported `ok: stack/infra`. The scope JSON document limits edits to `deploy/**`.
- A read-only Docker Engine version probe returned `29.7.2`. No deployment resources were started during planning.

## Tier M self-check

| Gate | Observation |
|---|---|
| 1. Decisions | Rows 1 through 16 have a closed decision or a documented unanswered assumption. Scope rows cite the dated assignment and approval. The plan names the preserved upgrade stores and the disposable test resources allowed to be changed. |
| 2. Acceptance | Both workstreams have observable criteria. Each item has first-written assertions, a failure mutation and an acceptance observation. G1 through G6 give commands and results. |
| 3. Design artifact | The plan covers no new screen. It links the existing approved browser text and checks the built website. |
| 4. Traceability | The named universe maps to the two workstreams. The scenario table covers R1 through R14. The foundation checks and both contract-fold requests are assigned to their gate owners. |
| 5. Risks | Both workstreams record severity and mitigation for their failure paths. |
| 6. Verification | Each workstream lists checks and the required environment. There is one rollout order. Both shipped supervisors have executable evidence requirements. |
| 7. Adversarial review | The attack entry is pending as specified by the planner assignment. The orchestrator owns the review after handoff. |
| 8. Anchors | Each task has a real anchor or a named target with an existing neighbor. Verified `start_server`, `join_stack`, `web_fixture_cleanup`, `relayControls`, `web_publishes_on_loopback_only` and `live_reconnect_restarts_service` directly in source. |
| 9. Guardrails | The plan cites the team and file boundaries, credit, issue handling, pull requests and private-detail review. The planning edits are confined to this folder and the result. |
| 10. Sizing | Five units have one work area and scope, a reason for low or medium size, and a launch count for the assigned vendor. The scope check passes. Live allowance is a dispatch check for the orchestrator. |

The frontmatter, heading order, artifact folds, decision statuses, scenario coverage, source anchors and local links were checked together. The shell example passed `bash -n`. The repository privacy command exited 0. The new files were read against the writing rules because untracked documents are outside the repository command's default input. The epic has the existing planned row. The orchestrator links it during the fold under the assignment's write boundary. The implementation and deployed tests are acceptance steps for the later build.
