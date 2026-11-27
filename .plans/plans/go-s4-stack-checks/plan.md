---
topic: go-s4-stack-checks
status: done
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# Upgrade an installed stack and prove Sprint 4 on real machines

## Overview

Sprint 4 is ready when an installed stack can upgrade and the deployed website can add real repositories. The server starts on the existing database and the installer keeps the client’s state. The same session and approved machine must survive. One old buffered event must apply once, and the client must answer a retried command from its saved receipt.

The machine will be offline between the two updates. A client with damaged history stops and logs one instruction to recover it. We will measure whether Linux and macOS leave that process stopped and still start a new process when the owner requests an ordinary restart.

The independent checks use a private stack, two enrollments and disposable Git repositories. The deployed browser must show both saved repositories after reload. The browser runner covers sample, mock and real candidate. Website tests cover older-server compatibility. (seam 2)

## Key decisions

The server and website go first. Each server accepts one link revision, so the owner should expect an offline interval until each installed client has been updated. The website can be unavailable during cutover. After it returns, the owner can use the same session while the old client is offline. The installer keeps each client’s enrollment and restarts its service. Once the database migration commits, recovery uses the new server image. (critic-r1 04, logic-r1 02)

The stack uses the same four running services. The server starts its own database changes for the repository service. The release gate builds the server and website from the same commit and gives their images a private tag. No owner decision is needed to begin plan review.

Full log → [decisions.md](decisions.md)

## Workstreams

### The joint gate uses a private stack and real repositories

Verify fixture ownership before testing the real repository service through the deployed website.

### An installed client keeps its history and stops when it needs recovery

The old binaries create the buffer and receipt. After cutover, check the identity, client state and server records. Observe the installed process stop for damaged history and restart on request.

## Rollout order

1. Review the additive link contract against its previous freeze and record the reviewed revision 2 commit. Run the private gate on matching server, client and website builds before handing the update to an installation.
2. Record the project and address, verify the existing password source, and build the candidate images while the old stack serves. Take a consistent backup of PostgreSQL and stack state. Stop each client to copy its state consistently and keep it stopped through the database cutover.
3. Update the server and website in the same Compose project and wait for the website to return. The database and vault keep their existing storage. A revision 1 client can no longer connect, so the Machines page shows it offline once the prior connection is gone.
4. After both services return, rerun the installer as each installed service user. Check the original session and machine. For damaged history, restore the required saved history or remove the machine and enroll again. For a revision mismatch, install a compatible client and keep its state. (logic-r1 09)
5. Follow the Stack test guide to save a distinct checkout from each deployed Add repository entry point. Check both saved folders after reload. (seam 3)

---
<!-- ⛔ Implementation details and gate evidence follow. -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Workstream 1 (WS-1). The joint gate uses a private stack and real repositories** (critic-r1 11)

Keep the independent checks, real binaries and database inside owned fixture resources.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | A run selects private projects, images, state and nondefault loopback ports. The surrounding installation is untouched. | `bash deploy/test/sprint4-fixture-test.sh` prints `ok` for `s4_private_stack_selection`, `s4_owned_cleanup` and `s4_two_git_workspaces`. |
| 2 | The relay preserves proof and retires both holds with the old stream. A mismatched leaf key prevents a request at the private edge. | `go test -v ./deploy/test/sprint4relay -count=1` passes `TestSprintFourRelayPreservesProofAndSelectsFrames`. Gate G7 proves the edge against deployed Caddy. |
| 3 | The existing Compose model serves Projects through the deployed website. | `bash deploy/test/sprint4-compose.sh` prints `ok 1 - s4_projects_uses_existing_stack`. Gates G4 and G5 cover discovery and browser behavior. |
| 4 | The release gate proves both the older and new design checks. | The orchestrator records the expected results of G1, G2, G4 and G5. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Prepare a private Compose and checkout fixture. | New `deploy/test/sprint4-fixture.sh` and `deploy/test/sprint4-fixture-test.sh`. Patterns are `deploy/test/web-testlib.sh` with `web_fixture_create` and `web_fixture_cleanup`, and `deploy/test/checks.sh` with `client_join`. |
| 2 | Add a test-only streaming relay. | New `deploy/test/sprint4relay/main.go` and `deploy/test/sprint4relay/main_test.go`. Patterns are `deploy/test/streamprobe/main.go` with `serve`, and `src/linktest/relay_test.go` with `relayControls`. |
| 6 | Prove the off-loopback certificate pin. | New `deploy/test/sprint4-certificate.sh` follows `deploy/test/checks.sh` and `check_c10`. Use item 2’s relay. |
| 5 | Prove the existing Compose model and document the release steps. | New `deploy/test/sprint4-compose.sh` follows `deploy/test/stack-web.sh` and `web_publishes_on_loopback_only`. Edit `deploy/README.md` in “Installing the client” and `deploy/test/README.md` after “Run C12 with disposable PostgreSQL”. Use `deploy/up.sh` and `start_server`. |

**Item 1. Private Compose and checkout fixtures.** Create only the two files in task 1. Items 2 and 4 can start alongside it. Items 3, 5 and 6 consume its fixture. Applies foundation D4, D10 and D17, rules 9 and 10, C1, C12 and C13, and sprint R11, R13 and R14. The frozen runner creates its own enrollment and checkout fixtures. (critic-r1 01, logic-r1 03)

- `sprint4_fixture_open <repository-root> <candidate-commit>` takes a read-only source location and checks out the exact commit into a private disposable Git checkout, exported as `S4_CANDIDATE_ROOT`. Retain Git history and both reviewed freeze objects there. Extract `f6cbea30` into `S4_LEGACY_ROOT` with `git archive`. Use those trees for builds, generation, and the feed-lock mutation. Compare tracked and untracked source before and after generation inside scratch and fail on drift. Keep caller source and any local certificates unchanged. (critic-r1 02, logic-r1 04)
- `sprint4_fixture_open` exclusively creates `S4_FIXTURE_DIR` and its run identity. Before the first launcher or request, reserve `lgtm-s4-<run>`, unused candidate and legacy image tags, a dedicated state directory and a free loopback port other than 8787. (seam 2) Refuse pre-existing project containers, networks, the expected named PostgreSQL volume and tags, even with matching Compose labels. Reject missing selectors and occupied ports before writing stack resources. Resolve the Compose model, check effective mounts and binding, and record the Docker daemon, resource identities and canonical paths as this invocation creates them. Attach a run-specific ownership label as resources are created. Use owner-only credential files and tracing off. (logic-r1 01, critic-r1 03)
- `sprint4_select <3|4>` exports explicit `COMPOSE_FILE`, `COMPOSE_ENV_FILES`, `COMPOSE_PROJECT_NAME`, `LGTM_STACK_STATE_DIR`, `LGTM_IMAGE_TAG`, `LGTM_WEB_PORT` and `LGTM_WEB_PASSWORD`. Clear competing inherited overrides. The first Compose file is the selected tree’s base. Its volume, state, address and password stay constant from 3 to 4. (seam 2) Recheck ownership and the actual container and binding before reuse, requests, lifecycle actions and teardown. Guard driver replacements against the same project, configuration and recorded stores. On ambiguity stop, and run the preinstalled `sprint4_fixture_close` trap for owned resources only.
- Stack’s fixture writes `S4_CANDIDATE_ROOT`, `S4_LEGACY_ROOT`, `S4_FIXTURE_DIR`, `S4_REPO_A` and `S4_REPO_B` for the release and proof runners. Create two distinct local checkouts for R14. Give one a fixture-only origin, divergent branch and dirty change. The independent runner owns its fixtures and scratch Vite configuration. Its writer owns both listener prerequisites below. (logic-r1 04) (seam 4)

- **Tests written first.** In `sprint4-fixture-test.sh`, `s4_private_stack_selection` rejects a missing selector, stale project-matching volume, foreign stack on the selected port, occupied port and port 8787 before requests or lifecycle calls. `s4_owned_cleanup` checks before-and-after identities, preserves a planted caller file and local certificate, and preserves the main volume through the tag change. `s4_two_git_workspaces` verifies distinct paths and the stated branch, origin and dirty state. Expect each test to fail on its named assertion against a stub.
- **Risky.** Remove the ownership or port check, or give both checkouts the same path. Require the corresponding assertion to fail. **Acceptance.** All three fixture cases pass.

**Item 2. Selective frame relay.** Create `deploy/test/sprint4relay/main.go` and `deploy/test/sprint4relay/main_test.go`. Items 3 and 6 use its interface after item 1. Applies foundation D7, D17, rules 6 and 8 through 10, C10 and C11, and sprint R11. (logic-r1 05, critic-r1 10)

- `sprint4relay --listen <addr> --upstream <url> --control-dir <dir>` accepts the paired optional flags `--tls-cert <file> --tls-key <file>` and prints `relay ready` after binding. It forwards generated `LinkService.Join` and `LinkService.Open`, including Challenge and refusal metadata. The relay assigns and records an identifier per connection. Stack’s upgrade runner atomically replaces `control.json` with `generation`, `stream_id`, `action` and `rules`. For `action: arm`, each rule gives `machine_id`, `direction` and `frame`. An event rule adds a nonzero `event_number`. A result rule adds `command_id` and a nonzero `attempt`. The relay rejects malformed or stale generations and makes an identical repeat idempotent. Disarmed new streams pass through.
- The relay writes `observations.jsonl`. The upgrade and certificate runners read it. Records identify the stream, generation and rule as above. `action` reports `armed`, `held`, `forwarded`, `discarded` or `closed`, with `outcome` or `digest` where applicable. Wait for both `armed` and both `held` records. Forward unrelated traffic, except that a later numbered event waits behind a held event. Closing the old stream discards its holds and permanently retires that generation. Confirm both discards and closure before candidate startup. No relay control is used to infer server permits or transaction state.
- **Tests written first.** `TestSprintFourRelayPreservesProofAndSelectsFrames` holds both frames while unrelated traffic passes. It checks arming, identical repeat, invalid selectors, ordered numbered frames, cancellation, discard and failed stale control on a second stream for the same machine. Only the second stream’s fresh replay may cross after cutover. Check signed proof and refusal metadata. **Risky.** Leak an old hold into the candidate stream or strip refusal metadata. Require the corresponding assertion to fail. **Acceptance.** The named Go test passes and G3 includes the two discarded old holds.

**Item 6. Off-loopback certificate pin.** Create only `deploy/test/sprint4-certificate.sh` after items 1 and 2. Applies foundation D7, D17, rules 8 and 9 and C10. Stack owns the runner and consumes the relay’s existing flags and observations. (critic-r1 10)

Use a private Docker network with no added published port. The disposable client uses the relay’s network alias with Transport Layer Security (TLS). The relay forwards to the actual deployed Caddy. Rewrap a real one-use token for the test-only address and pin. Verify Join and authenticated Open with the pinned leaf. Use fresh tokens. Wait for the same listener to serve a valid leaf with a different key. Require `certificate_key_mismatch` and zero forwarded application calls for that connection. Restore the matching key and prove a new real Join and Open. **Tests written first.** Freeze `off_loopback_certificate_pin` with a wrong-key negative and both matching-leaf controls. **Risky.** Make the bad-leaf control use the trusted key and require red. **Acceptance.** G7 prints the named pass and exits 0.

**Item 5. Compose and release guide.** Create `deploy/test/sprint4-compose.sh`. Change only `deploy/README.md` and `deploy/test/README.md`. Item 1 comes first. The draft can run alongside items 3, 4 and 6. Finalize the guides after live verification in the joint release wave. Item 5 owns “Run the deployed Sprint 4 repository check” in `deploy/test/README.md`, the authoritative R14 checklist. (seam 3) Applies foundation D1, D4, D10, D17 and rules 1, 2, 9 and 10, C1, C12 and C13, the Sprint 4 Projects registration, and R13 and R14.

- Keep `deploy/compose.yaml`, both Dockerfiles and `deploy/Caddyfile` as the existing production inputs. The build context includes Projects migrations and generated Go. Caddy’s existing matcher passes all three procedures. Use the existing PostgreSQL and OpenBao digests and keep the `local` development default. Gate evidence records both private tags and the image identifiers.
- **Tests written first.** `s4_projects_uses_existing_stack` in `sprint4-compose.sh` runs `docker compose config --format json` privately. It asserts the exact services and profiles of the checked-in baseline, the two selected image tags, one loopback web binding at the fixture port, no other published port, and the existing `pgdata` storage and private network topology. Make it first fail on an assertion with a stub fixture selecting the wrong tag and binding. Separately the independent checks require actual, authenticated calls through the built website, following the guarded router in the accepted contract.
- **Risky.** Mutate the fixture image tag selection or publish a second service. The static assertion must fail. Change the served build to prompt for a server address or route Projects outside the existing proxy in a scratch override. The deployed browser acceptance must fail. **Acceptance.** The static check prints `ok 1 - s4_projects_uses_existing_stack` and exits 0. The guide names the commands and evidence.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| The harness starts, probes or cleans an installed stack. | High | Refuse an occupied selector up front. Recheck recorded resource and listener identities before each phase and remove only fixture-owned resources. |
| A model check passes while the served bundle calls the wrong backend. | Medium | Run signed-in browser checks against the real served image and the comparison build. Record the same-origin request on the deployed path. |
| Holding whole connections obscures which event or result the test exercised. | Medium | Fence relay holds by stream and sequence. Use only the in-process seams for permit, commit and receipt boundaries. |

**Verify.** Use the item suites and G1, G2, G4, G5 and G7. Use Linux, Docker, Go, real PostgreSQL, two clients and a browser. G7 needs a private Docker network. Website user interface changes belong to Website.

**Design artifact.** The browser gate uses the approved text in [The website in Sprint 4](../../../docs/specs/2026-11-27-running-an-agent-design.md#the-website-in-sprint-4) and the unchanged deployed website image build.

**Workstream 2 (WS-2). An installed client keeps its history and stops when it needs recovery**

Run the installer as the old service user. Correlate state, session and replay on that installation.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 5 | The old binary is refused by the new server, then the same machine reconnects through an installer rerun with the saved state. | G3 prints `ok 1 - verified_revision_one_refusal`, `ok 2 - installed_state_retained`, `ok 3 - retained_event_and_receipt_replayed` and `ok 4 - upgrade_recovers_forward`. |
| 6 | The actual service handles both kinds of exit on each supported operating system. | G6 prints `ok 1 - requested_restart_restarts_service`, `ok 2 - invalid_history_counter_stops_before_dial`, `ok 3 - invalid_history_payload_stops_after_welcome` and `ok 4 - trusted_git_inspects_as_service_user` on both systems. (seam 11) |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Prove the forward installer and database upgrade. | New `deploy/test/sprint4-upgrade.sh`, `deploy/test/sprint4store/main.go` and `deploy/test/sprint4store/main_test.go`. Patterns are `deploy/test/installer-live.sh` with `live_rerun_keeps_machine` and the pinned `replay.go` and `receipts.go` client readers. |
| 4 | Prove the installed supervisor's exit behavior. | New `deploy/test/sprint4-supervisor.sh`. Patterns are `deploy/test/installer-live.sh` with `live_reconnect_restarts_service`, `live_removed_machine_stays_stopped` and `unit_owned_by_fixture`, and the two shipped templates' restart policies. |

**Item 3. In-place database and client upgrade.** Create only task 3’s files. Run after items 1 and 2, separately from item 4. Applies foundation D4, D5, D7, D10, D16, rules 2, 4 and 6 through 10, C4 and C9 through C13, and sprint R11. (critic-r1 04, critic-r1 05, logic-r1 02, logic-r1 05)

- Launch Sprint 3 through its normal `deploy/up.sh`. Apply item 4’s account and path checks. Enroll through the relay, then run the old installer and approve the machine. Build the candidate pair while the old stack serves. Record the original password source, authenticated browser session, executable digest, machine and approval, client state and key digests, OpenBao key fingerprint, PostgreSQL volume and a settled sentinel command. The normal first install is exercised in item 4.
- Drain and acknowledge the old startup prefix. Briefly suspend the fixture process, request `ClientService.ScanClientHarnesses`, and take the command and attempt from the relay’s forward observation. Arm both holds for that attempt and the next event number before resuming within the link and command deadlines. After both holds are observed, stop the old service, wait for its database lock to release and confirm both relay copies were discarded. Stack owns the private reader `sprint4store --db <closed-client.db> --event-number <n> --command-id <id>`. It uses the pinned revision 1 buckets and receipt format, opens read-only with a bounded lock wait, and emits one record with `event_number`, `digest`, `command_id` and `outcome` for the upgrade runner. It requires the version byte and a finished receipt. The reader decodes the stored report without its version byte. The relay clears the number on a copy of the wire event. Both hash the same deterministic Protocol Buffers encoding using the 256-bit Secure Hash Algorithm. Compare the sequence separately. Require the server mark below the event and the command still unsettled. Distinguish startup inventory probes from a commanded scan.
- Select `4` with the original stores and settings and run the candidate’s normal `deploy/up.sh` on the prebuilt pair. Check migration and health, Projects discovery at website revision 1 and the original browser session. Confirm a signed Challenge followed by the revision 1 refusal and no Welcome, admission or lost local records. Run the old client alone for the refusal before starting the candidate client. Quiesce it again before using the candidate installer. Require `lgtm-client is already joined.`, no token issued, the same approved machine online, owner-only state, and the original volume, session, identity and settled sentinel.
- The cutover must finish before the old command’s expiry plus 23 hours according to the server clock. Record its original and reoffered attempt. The fresh candidate stream must deliver the same event sequence and digest from disk, with exactly one committed `link_event_applied` record for `machine_id` and `sequence`, a matching cumulative acknowledgement and a mark advance. The same command settles once on its new attempt with the saved outcome and `CommandResult.last_event_number` equal to zero. The scan probe shows zero replay executions and increments for a fresh positive-control command. Keep old and new stream evidence separate.
- **Tests written first.** Freeze the three G3 criteria in `sprint4-upgrade.sh`. They must fail for a fresh volume, missing old finished receipt, altered digest or stale-stream forwarding. In `main_test.go`, `TestSprintFourStoreReadsRevisionOne` checks a closed revision 1 fixture and refuses missing, damaged or writer-locked input without mutation. `upgrade_recovers_forward` interrupts before migration commits, after commit before web readiness and after binary replacement before service restart. Use the operator procedure below and compare the original volume, session and machine after repair.
- **Risky.** Skip the verified refusal, replace a store, clear the receipt or change the payload. Require the matching assertion to fail. **Acceptance.** G3 reports the four named passes and exits 0.

**Item 4. Terminal history and requested restart under systemd and launchd.** Create only `deploy/test/sprint4-supervisor.sh`. Stub assertions can start with item 1. Native proof needs items 1 and 2 and the Client exit convention. (seam 11) Applies foundation D7, D16, D19, rules 7 through 10 and C8 through C10.

- Run separately as a logged-in disposable user on Linux and macOS. On each host the runner launches and verifies its own stack and loopback listener through the same Docker context and Compose selector the installer uses. macOS needs compatible Bash and `flock`, plus a local Docker context with loopback publication and private bind modes. Before installing, verify the manager and account, and require the executable, service, log, state and manager entry to be absent. Later actions use the recorded identities and paths. **UNREVIEWED MECHANISM** (critic-r1 09, logic-r1 01). Reject prepared selectors.
- Install and approve the machine. Call `ClientService.ReconnectClient` and require exit 75 and a replacement process on the same enrollment. As the installed service user, record the canonical trusted Git executable and its `git --version` under the service search path. Run a real `InspectFolder` on a disposable checkout through that installed client. (seam 11)
- On each manager, run two isolated faults with intact enrollment. Set closed-store `meta.next` to 2^63 + 1 and require zero dials and admissions. Separately retain an unacknowledged report using item 2, close the writer, discard its relay copy, and corrupt the required payload while keeping valid counters. Require a verified Welcome, unchanged server mark and Projects records, and no handoff of the corrupt event. For each fault require intact disputed bytes and enrollment, exit 0, one history remedy and no replacement for 12 seconds. Restore the matching fixture state and prove a manual recovery. (seam 11)
- Capture the old process and its exit before replacement can hide it. On Linux use `systemctl --user show lgtm-client.service` and `journalctl --user -u lgtm-client.service`. On macOS use `launchctl print gui/$(id -u)/dev.lgtmstudio.client` and the recorded log interval. Incompatible Welcome also exits 0. Its single remedy is to install a compatible client and keep its state, while damaged history calls for original saved history or a fresh enrollment. The paired assertions check both diagnostics, preserved state and no second dial. (logic-r1 09)
- **Tests written first.** Freeze G6’s four named cases on both managers. Refuse a foreign prepared selector or a pre-existing binary before mutation. **Risky.** Allow a dial on bad counters, advance the mark after the bad payload, restart on status 0 or resolve a checkout-local Git. Require the matching assertion to fail. **Acceptance.** G6 reports all four passes on both hosts. (seam 11)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A fresh installation appears to be a successful upgrade. | High | Compare the existing volume, session, enrollment, approved identity, old event and command ledger across cutover. |
| A retried command runs again after the installer updates the binary. | High | Hold the result after its durable receipt. Correlate the new attempt and saved outcome with the version-probe count. |
| A service crashes repeatedly or normal restart silently stops working. | High | Observe both statuses on each shipped supervisor. Check process and log evidence past two intervals and exercise a positive restart. |

**Verify.** Run G3 and G6 plus `bash deploy/test/installer.sh` as regression coverage. Use disposable Linux and logged-in macOS users with their native managers. Record both G6 runs.

**Exact interfaces for the joint gate**

| Boundary | Public names and observations |
|---|---|
| Client and launchers | `deploy/up.sh`, `deploy/install-client.sh`, `lgtm-client join <address> -`, `lgtm-client run`, `LGTM_CLIENT_STATE_DIR`, `lgtm-client.service`, `dev.lgtmstudio.client`. Expected client exit values are 0 for terminal history and 75 for requested restart. |
| Compose | `COMPOSE_FILE`, `COMPOSE_ENV_FILES`, `COMPOSE_PROJECT_NAME`, `LGTM_STACK_STATE_DIR`, `LGTM_IMAGE_TAG`, `LGTM_WEB_PORT`, `LGTM_WEB_PASSWORD`. Running services are `postgres`, `openbao`, `server` and `web`. |
| Wire and discovery | Use the [contract fold’s published names](../../../.foundry/go-rewrite/results/fold-go-s4-contract-r2.md#names-the-other-plans-of-sprint-4-build-against). Website revision stays 1. Link revision 2 preserves the Challenge and the refusal `failed_precondition` with `Lgtm-Link-Refusal: link_revision_mismatch`. |
| In-process harness | `servertest.Start`, `clienttest.Join`, `clienttest.Run`, `LGTM_TEST_POSTGRES`, `src/contracts/testdata/link-v2-limits.json`. Server Link supplies `servertest.Options.LinkSeams.BeforeEventCommit func(context.Context, string, uint64) error`, `BeforeEventAck func(context.Context, string, uint64)` and `servertest.Options.ConfigurePostgres func(*pgxpool.Config)`. Client Folders writes `clienttest.Options.AfterFolderReport func(context.Context, uint64) error`. The orchestrator’s paired tests read these options. Use the known fixture operation and command records to correlate the client callback’s event number. Compile the paired suite against both fixture packages before freezing. (critic-r1 06, logic-r1 06) Read the exact byte and event limits from the fixture. |

**Rollout detail for steps 1 and 5.** Keep `deploy/test/checks-s4.sh` and `README-s4.md` at `22ca76c6`. Preserve the Test Anything Protocol (TAP) results. (seam 2) (seam 3) Record the commit, images and sanitized evidence. Missing or skipped evidence fails the gate. Verify ownership and actual binding before requests. G1’s password changes and each live proof use separate fixtures. G4 and G5 share one. (critic-r1 01 and 02, logic-r1 01, 03 and 04)

Set `S4_CONTRACT_COMMIT=89506b7c` and verify it is an ancestor of `S4_CANDIDATE_COMMIT`. Run history and paired gates before landing the single revision 2 runtime cutover with its matching fixture edits. Run G1 through G7 on the integrated candidate. Build, generate and mutate in scratch. For G4 and G5 select `4` and run the normal launcher. Require `phase server done`, `phase web done` and the recorded private address. The driver reads `LGTM_WEB_PASSWORD`, `LGTM_WEB_PORT`, `COMPOSE_FILE` and `LGTM_REV1_COMMIT=f6cbea30`. (seam 2)

In build wave 1 the orchestrator’s independent test writer is the sole writer of `deploy/test/checks-s4-browser.mjs` and `deploy/test/with-postgres.sh`. Freeze planted-listener and reserved-port controls. Before requests, require the browser child’s readiness and the owned database container’s explicit loopback binding. Refuse port 8787 and require zero requests to the planted listener. G1 and G2 require the wrapper freeze. G5 requires the browser-helper freeze. (seam 4)

| Gate | Command or operator step | Required observation |
|---|---|---|
| G1. Contract, source and wave 1 | For contract review, run `bash deploy/test/with-postgres.sh -- env LGTM_LINK_BASE=fcfa96c3 bash scripts/check-go.sh`. After it lands, repeat with `LGTM_LINK_BASE="$S4_CONTRACT_COMMIT"`. Run `pnpm --filter @lgtm/api check`, `pnpm --filter @lgtm/frontend typecheck`, `pnpm --filter @lgtm/mock-server typecheck`, and `node scripts/check-private.mjs` in the disposable candidate checkout. Verify its tracked and untracked source status across generation. On its private main selection, run `bash deploy/test/with-postgres.sh -- bash deploy/test/checks.sh`, which starts its own stack at C1. | Zero exits, passing Go summaries, no freeze refusal, empty privacy output, and unchanged source. One `ok` line each for C1 through C13. C12 confirms the feed-lock mutation fails its assertion. |
| G2. Real client and server together | Compile with `go test ./src/linktest ./src/server/servertest ./src/client/clienttest -run '^$'`. Run `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^Test(SprintFour|LinkOutboxReplayAfterServerFailure$|LinkReservedCommandResult$)' -count=1`. | Explicit `--- PASS:` on all five named tests and both retained cases, zero skips and the listed Link, Folders and Projects evidence. |
| G3. Installed upgrade | As the disposable Linux service user, run `LGTM_SPRINT4_LIVE=1 bash deploy/test/sprint4-upgrade.sh "$S4_CANDIDATE_COMMIT"`. | Exit 0 and the four results under criterion 5. Save the old and new migration ledger, session, replay, receipt and attempt observations. |
| G4. Independent repository, refusal and removal checks | Run `bash deploy/test/checks-s4.sh --only R1,R2,R4,R11,R12` in the selected candidate fixture. | Exit 0 and lines beginning `ok 1 - R1`, `ok 2 - R2`, `ok 4 - R4`, `ok 11 - R11` and `ok 12 - R12`. The R11 line covers refusal. |
| G5. Browser and real stack | Run `bash deploy/test/checks-s4.sh --only R13`. Collect the Website cases and perform the Stack guide’s R14 checklist on the deployed image. (seam 2) (seam 3) | Exit 0 and `ok 13 - R13`. Keep the signed-in comparison results, the deployed same-origin request and both saved identities after reload. Record R14 as an owner observation. |
| G6. Installed restart and stop | Run `LGTM_SPRINT4_LIVE=1 bash deploy/test/sprint4-supervisor.sh "$S4_CANDIDATE_COMMIT"` once per native manager. | Both runs exit 0 and print the four passes in criterion 6. Save manager exits, both terminal observations and the trusted Git version and inspection. (seam 11) |
| G7. Off-loopback certificate | Run `LGTM_SPRINT4_LIVE=1 bash deploy/test/sprint4-certificate.sh "$S4_CANDIDATE_COMMIT"` on an owned fixture. | Exit 0 and `ok 1 - off_loopback_certificate_pin`. Save listener identity and per-connection forwarded request counts. |

**Design check assignment and bring-up.** Run refusal and paired history when Link and Folders are ready, folder operations and removal after Projects, and G5 with the built website. The orchestrator owns `src/linktest/`. (critic-r1 01 and 07, logic-r1 03 and 07)

| Design check | Authoritative evidence and prerequisite |
|---|---|
| R1, R2 and R4 | G4’s respective results on real Link, Folders and Projects. Inspect a dirty, divergent branch without storing it. Resolve the added folder and refuse the non-repository. Attach on a second machine and refuse a distinct origin despite a shared first commit. |
| R3 and R7 | G2 `TestSprintFourFolderSettlement` runs `R3/SimultaneousAliases` and `R7/DisconnectBeforeDelivery`. Require one folder from both paths. Disconnect after enqueue. Require the 25-second deadline with the operation open. Reconnect after the 20-second expiry. Require decline, failed operation and zero inspections. |
| R5 and R6 | G2 `TestSprintFourReceiptReplay` runs `R5/ServerRestart` and `R6/InterruptedAfterAppend`. On the same key after server restart, return the original operation, command and reply with one inspection. For R6 capture the durable report and started receipt. The successful operation survives the later `interrupted` result. |
| R8 and R9 | G2 `TestSprintFourPendingRun` runs `R8/CommitOrder` and `R9/RunGate`. Hold the first write under the machine lock and observe the second blocked before release. Occupy three actual send permits, then apply and acknowledge old events before any claim, attempt or cursor change. Free a permit and require both commands on the same stream in commit order. Repeat with an empty prefix. Keep `TestProjectOperationAtomicStart`, `TestPartEnqueueIsAtomicAndOrdered` and `TestRunBarrierDoesNotUseCommandPermit` as direct evidence. |
| R10 | G2 `TestSprintFourFolderSettlement` runs `R10/StagedAcrossRestart`. Sweep past cutoff without resend or settlement. At the pre-commit fault require no save, mark change, settlement or acknowledgement. On replay expect one application and settlement. After outcome-first settlement, a late eligible report advances and acknowledges without a Projects effect. Exercise removal on both sides. |
| R11 | G4 proves actual predecessor refusal. G3 proves installed replay. G2 `TestSprintFourReceiptReplay` runs `R11/LegacyReceiptAndBuffer` with the legacy zero and no reexecution. |
| R12 | G4 proves removal in a page already open on the deployed website. A repository with another folder keeps that folder, a now-orphaned repository disappears, and the pending operation fails. |
| R13 | G5’s `--only R13` uses one scratch build in the in-page sample, mock and advertising real-candidate contexts. Require sample projects in both sample modes, real sign-in and both watch responses with no sample projects. Website’s discovery and snapshot cases prove older-server compatibility, snapshot gating, no sample collections and no simulation. (seam 2) |
| R14 | G5 records the owner’s two saves and reload observations using the Stack guide. (seam 3) |

**Additional required case evidence for G2.** (critic-r1 06, critic-r1 07, logic-r1 06, logic-r1 07)

| Named owner case | Required observation |
|---|---|
| `TestSprintFourHistoryRecovery` | Refuse authenticated history behind the mark. See the warning on the live watch and after restart, with and without a staged result. Incumbent traffic leaves it set. A later ready admission clears it. Removal clears the record and fails open work. Use `ConfigurePostgres` to cut commit before delivery and to lose a committed reply. Require the existing database error, same current incumbent and heartbeat, and flag and feed both present or both absent. Exercise corrupt or missing local history and incompatible Welcome with the separate Client remedies and unchanged surviving state. |
| `TestSprintFourPendingRun` | In addition to R9, restart mid-drain and refuse the former ready run and changed pending pair. Allow an unseen replacement. Refuse a superseded ready run at the same first event number without a second ready callback. Require one cumulative acknowledgement for a committed next event or repeat. A gap leaves the mark untouched and receives no acknowledgement. |
| `TestSprintFourWaitingReplay` | At waiting Welcome apply retained Facts and consume the blocked folder report. Pause after final application and before its acknowledgement, approve on that stream, and require both replay acknowledgements before `SetState`. Fresh reports wait for approval. Replaced, removed and other-machine streams cannot finish the operation. |
| `TestSprintFourReceiptReplay` | Drop the original `SetState` reply. Show the same watermark beyond ordinary expiry, after two restarts and additional events, including a restart with empty old history. Observe the started receipt and exact durable event before the client interruption, then one committed report and an unchanged success after interruption settlement. Require zero for valid revision 2 metadata and recognized revision 1 receipts. Missing or damaged revision 2 metadata stops before a result. |
| `TestSprintFourFolderSettlement` | Retain the observations for R3, R7 and R10 above. Capture database state on both sides of the Link rollback seam and tie the acknowledgement and settlement to the retried event. |
| Link, Folders and Projects limits | Record `TestRevisionTwoEventBudgetsAndAcks`, `TestLookAtFolderCommandBudgets`, `TestLookAtFolderEventBudget` and `TestReviewGUnacknowledgedEventWindow`. Use `link-v2-limits.json` for exact and one-over encoded message, command and event, typed path, resolved path and remote edges. Require the specified rejection or omission before storage or numbering, and withhold event 33 until cumulative credit returns. Keep Projects’ path boundary assertion in `TestProjectOperationAtomicStart`. |

**Deployed browser proof and authoritative R14 checklist.** Under “Run the deployed Sprint 4 repository check” in `deploy/test/README.md`, use the actual candidate `web` image with `VITE_LGTM_SERVER=/`. Confirm same-origin `GetServer` with no server-address prompt. Use two distinct checkouts on one approved machine, including one dirty checkout. Compare Base branch, Remote and Status. Save from the sidebar and from Machines. Capture each save’s repository, folder and machine identifiers and resolved path. After reload verify both against the authenticated watch. (seam 3) Keep the Website tests `discovery chooses the workspace and simulation`, `workspace waits for both snapshots`, `repository reconnect replaces and removal converges`, `both dialogs have an empty machine choice`, `new project keeps the goal and reports unavailable`, `add uses the sidebar or the opening machine`, `add joins the watched identity in either order` and `inspection follows the latest input` as release evidence. `machine recovery follows snapshots and updates` and `recovery state survives stale approval and clears on watched readiness` prove “Out of sync”. G2 proves flag persistence and clearing. (critic-r1 08, logic-r1 08)

**Rollout detail for steps 2 through 4.** Record the project, volume, address, credential source and both image tags. Privately verify that the candidate resolves the unchanged valid website password and original stores. Prepare the paired images without stopping the old stack. Abort cutover if credential verification, backup or prebuild fails. Quiesce client writers and preserve their state, then run the candidate’s `deploy/up.sh` using the same settings. On ambiguous startup, stop the candidate server and read `SELECT part, version FROM schema_migrations ORDER BY part` on the selected PostgreSQL database. Store writes this ledger. The guide and upgrade runner read it. Compare against the recorded Sprint 3 ledger and the reviewed candidate manifest. Before any new migration commits, the prior pair may start on the verified old ledger with the original settings. After any migration commits, preserve both stores and recover forward with the same candidate launcher, including a web-only startup failure. If the ledger cannot be verified, keep clients stopped until the migration outcome is known. Check health, Projects discovery and the original session before updating clients. If an installer stops after replacing the binary, check the recorded unit, installed binary and running process, then rerun the same candidate installer. After a candidate client writes revision 2 receipt metadata, keep using the candidate with that state. The saved copy serves deliberate history recovery. Database rewind and indistinguishable replacement history are outside wave 1 and tracked in issue 1676. (critic-r1 04, logic-r1 02)

**Post-build checks.**

| Scenario | Expected evidence |
|---|---|
| Retry after a failed private run with a stale selector, a pre-existing matching-label volume, an occupied port and an existing caller certificate. Then run twice on fresh identities. | Refuse before mutation or application requests on the stale or occupied selection. Preserve unrelated resource identities, caller source and certificate hashes across the two fresh runs. |
| Interrupt around migration commit and after installer binary replacement. Close the old stream with both holds armed. | Use the verified old-ledger or forward-repair branch and recover the session and machine on the original volume. Require both old discards, fresh replay at the original sequence and digest, one application and settlement on a new attempt, and zero replay scans. |
| Rotate the certificate leaf and run both actual supervisors. | Before the bad-key request, observe the new leaf on the owned listener. Require zero forwards, then successful matching-leaf Join and Open. Capture exit 75 and replacement, both status 0 stops without replacement, recovery and trusted Git on each manager. (seam 11) |
| Save through both deployed entries, reload, then reopen the watch after fixture removal. | Both saved identities appear before removal. The replacement snapshot retains the surviving identity and excludes the removed one. Preserve authenticated watch and browser evidence from the same fixture. |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is the five assignment deliverables and shared rules, foundation D1, D4, D5, D7, D10, D16, D17, D19, rules 1 through 11 and C1 through C13, Sprint 4 Link, Projects, folders, removal, website and R1 through R14, both third-round closing lists, the contract fold’s Stack row and names, issues 1632 through 1635, the epic team boundary, build loop Phase 4 and sprint seams 2, 3, 4 and 11.

| Source item | Workstream | Status |
|---|---|---|
| Assignment 1 and 5, checks and gate | WS-1, WS-2 | The fixture, on-ramp and joint gate keep the independent checks with their author and add deployment proof. |
| Assignment 2 and 3, upgrade and terminal stop | WS-2 | Items 3 and 4 cover the actual installed state and process outcomes. |
| Assignment 4, Compose and tags | WS-1 | Items 1 and 5 keep paired versions and observe the existing topology and real route. |
| Shared assignment rules and epic team boundary | WS-1, WS-2 | Decision rows 1 through 3 and 14 assign scope, independent tests, separate roles and review. |
| D1, D4, D5, D7 and D10 | WS-1, WS-2 | Keep the three existing boundaries, real PostgreSQL and persistent stack state. Measure sessions, proof and ordering in the gate. |
| D16, D17 and D19 | WS-1, WS-2 | Observe a requested restart, pinned transport and existing user service. |
| Foundation rules 1 through 5 | WS-1, WS-2 | Contract-first build, Projects-owned migrations and authenticated watches. Test observations use the public service or read-only fixture access. |
| Foundation rules 6 through 10 | WS-1, WS-2 | Keep durable command and event checks, fixed refusals, private credentials, real PostgreSQL and the named in-process harness. |
| Foundation rule 11 | WS-1, WS-2 | Reuse repository patterns and generated interfaces. The guardrail for future copied code still applies. |
| C1 through C4, C5 through C11, C12 and C13 | WS-1, WS-2 | G1 retains the complete wave 1 gate, with C4, C9 through C11 and C13 also grounding the upgrade proof. |
| Sprint 4 link revision and R6 through R11 | WS-1, WS-2 | G2 owns result order, run gates, expiry and exact interruption boundaries. G3 and G4 prove the deployed upgrade and refusal. |
| Sprint 4 Projects, folders, removal and R1 through R5 and R12 | WS-1 | G4 uses its own checkout and machine fixtures. G2 proves convergence and retry at the durable boundaries. |
| Sprint 4 website and R13 and R14 | WS-1 | G5 covers the three runner contexts, Website cases and the two deployed saves. (seam 2) (seam 3) |
| Logic and fit third-round closing lists | WS-1, WS-2 | Decision rows 12 and 13 name the applied parts and the Sprint 5 disposition. G2 and G5 retain their observations. |
| Contract fold Stack request and exact names | WS-1, WS-2 | G1 through G7 use its names and freeze transition, exact in-process passes and required deployment evidence. |
| Issues 1632 through 1635 | WS-1, WS-2 | Use Go and observe ordered database behavior and connected restart, verify the private TLS edge, use public paired-program test entry points and rate the six work items. |
| Sprint seams 2, 3, 4 and 11 | WS-1, WS-2 | Adopt the agreed evidence, listener prerequisites and separate native faults. |
| Build loop Phase 4 | WS-1, WS-2 | The test writer proves the initial failure, the builder preserves the frozen assertions, and the lead requires a passing report and the specified mutation evidence on the built commit. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, “Where things are” | Deployment work targets `deploy/`. Old engine folders stay reference material. Product interface requirements use the reviewed Protocol Buffers contract. |
| `AGENTS.md`, “Credit copied code” | Reuse repository patterns. Credit any copied code through `NOTICE` and its required license. |
| `AGENTS.md`, “Write down what you find” | Planning findings are recorded in this bounded plan and its receipt. Route any new product issue to the orchestrator for the repository issue log. |
| `AGENTS.md`, “Finished work reaches the base branch through a pull request” | The orchestrator owns integration. The gate report names the exact candidate commit. |
| `AGENTS.md`, “This repository is not a fork of Herdr Synapse” | The fixture uses only the LGTM Studio checkout and local images. |
| `AGENTS.md`, “Writing style for documents” | Use dated sources and repository-relative paths. Run the repository privacy check and inspect the new planning files directly. |
| Epic team table and Phase 4 | One writer per build file, test-first and frozen assertions, isolated mutation evidence, and orchestrator ownership of joint tests. |
| Assignment write boundary | Only this plan folder and its result file are written in planning. Stack uses `stack/infra`. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Compose and checkout fixture | infra | `stack/infra` | medium | It combines existing Docker ownership and client state patterns with two Git workspaces. | Yes | None. |
| 2. Frame relay | infra | `stack/infra` | medium | One helper reuses generated streaming interfaces and owns two stream-bound holds. | Yes | None for unit tests. Item 1 for deployed use. |
| 3. Installed upgrade proof | infra | `stack/infra` | medium | One runner checks the old and new installations across ordered lifecycle steps. | Yes | Items 1 and 2, candidate Link and Client, and mounted Projects for final acceptance. |
| 4. Supervisor exit proof | infra | `stack/infra` | medium | One runner checks two existing platform service managers and their recorded exit states. | Yes | Items 1 and 2 for live use, plus the Client exit convention. (seam 11) |
| 5. Compose proof and guide | infra | `stack/infra` | low | A configuration assertion and two guides use established launch paths. | Yes | Item 1 for static checks and items 3, 4 and 6 for final guide verification. |
| 6. Certificate edge | infra | `stack/infra` | medium | One fixture runner checks key changes with the relay and existing client transport. | Yes | Items 1 and 2 and the built candidate. |

Load. OpenAI supplies the planner, test writers, builders and reviewers. The plan author, two reviewers, fold planner and alignment planner have run. Six test writers, six builders and two build reviewers remain, for 19 launches total. The orchestrator checks the vendor allowance before dispatch and counts independent and paired-test seats at sprint level.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | Findings by severity | Outcome |
|---|---|---|
| 1 | 2 blockers, 10 majors and 3 minors after deduplication. | [Round 1 findings](attack/round-1-findings.md) records the disposition of each finding. The [cross-plan report](../../epics/go-rewrite/sprint-4-seams.md) assigns seams 2, 3, 4 and 11, now resolved in the plan. |

</details>
