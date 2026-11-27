---
topic: go-s4-stack-checks
status: attack
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# Upgrade an installed stack and prove Sprint 4 on real machines

## Overview

Sprint 4 is ready for deployment when an installed stack can move to the new link revision and the website can add a real repository. The proof starts with the previous server and an enrolled client that has saved work. The new server starts on the existing database. The installer then updates the client in its existing state folder. The same signed-in session and approved machine must survive, and the server must apply the old buffered event once and accept the saved command answer.

The machine will be offline between the two updates. A client with damaged history stops and logs one instruction to recover it. We will measure whether Linux and macOS leave that process stopped and still start a new process when the owner requests an ordinary restart.

The release checks use a private loopback stack, two independently enrolled clients and disposable Git repositories. The deployed browser must show the repository in the tree after a reload. A separate browser run verifies that the in-page sample, the mock server and the old real server still show sample projects.

## Key decisions

The server and website go first. Each server accepts one link revision, so the owner should expect an offline interval until each installed client has been updated. During the interval the owner can still use the website. The existing installer keeps the client's enrollment and restarts its service.

The stack uses the same four running services. The server starts its own database changes for the repository service. The release gate builds the server and website from the same commit and gives their images a private tag. No owner decision is needed to begin plan review.

Full log → [decisions.md](decisions.md)

## Workstreams

### The joint gate uses a private stack and real repositories

The checks get two machines, real Git data and an independent database test environment. The private stack settings select the same installation for the launcher, installer, browser and database observations. The gate also verifies that the website reaches the repository service through the installed web server.

### An installed client keeps its history and stops when it needs recovery

The proof uses the old binaries to create a real event buffer and a real command receipt. After the cutover it checks the existing identity, local state and server records. A lost or damaged history test starts the actual service and observes its exit and continued absence. The requested restart test gives a positive control for each supervisor.

## Rollout order

1. Review the additive link contract against its previous freeze and record the reviewed revision 2 commit. Run the private gate on matching server, client and website builds before handing the update to an installation.
2. In the installation, record the project name and published address. Keep the prior images available for diagnosis and take a consistent backup of PostgreSQL and the stack state. Stop each client long enough to copy its state consistently, then let it reconnect if it must keep running until cutover.
3. Update the server and website in the same Compose project and wait for the website to return. The database and vault keep their existing storage. A revision 1 client can no longer connect, so the Machines page shows it offline once the prior connection is gone.
4. Run the new installer once for each client as its service user. Wait for that same machine to show online. Check that the old session still works. If a client stops with the history recovery instruction, keep its state for recovery, restore the actual saved history when available, or remove the machine and enroll it again.
5. Use the deployed website to add a repository and verify it after a reload. Record the joint gate results before releasing the sprint.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1. The joint gate uses a private stack and real repositories**

Supply the fixture that the independent design checks can run without reaching the installed stack. Use actual client binaries and an actual database. Use test settings to control the traffic and browser origin only where the assertions need them.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | A run selects private projects, images, state and nondefault loopback ports. It leaves the surrounding installation alone. | `bash deploy/test/sprint4-fixture-test.sh` prints `ok` for `s4_private_stack_selection`, `s4_owned_cleanup` and `s4_two_git_workspaces`. |
| 2 | The test relay controls exact frames through the existing link. A bad leaf key prevents a client request at the private encrypted edge. | `go test -v ./deploy/test/sprint4relay -count=1` explicitly passes `TestSprintFourRelayPreservesProofAndSelectsFrames` and `TestSprintFourRelayUsesPinnedLeaf`. Gate G3 proves the deployed upstream. |
| 3 | The normal Compose model needs only its existing server and web builds. Projects is discoverable through the deployed website. | `bash deploy/test/sprint4-compose.sh` prints `ok 1 - s4_projects_uses_existing_stack`. Gates G4 and G5 cover discovery and browser behavior. |
| 4 | The release gate proves both the older and new design checks. | The orchestrator records the expected results of G1, G2, G4 and G5. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Prepare a private Compose and checkout fixture. | New `deploy/test/sprint4-fixture.sh` and `deploy/test/sprint4-fixture-test.sh`. Patterns are `deploy/test/web-testlib.sh` with `web_fixture_create` and `web_fixture_cleanup`, and `deploy/test/checks.sh` with `client_join`. |
| 2 | Add a test-only streaming relay and encrypted edge. | New `deploy/test/sprint4relay/main.go` and `deploy/test/sprint4relay/main_test.go`. Patterns are `deploy/test/streamprobe/main.go` with `serve`, and `src/linktest/relay_test.go` with `relayControls`. |
| 5 | Prove the existing Compose model and document the release steps. | New `deploy/test/sprint4-compose.sh` follows `deploy/test/stack-web.sh` and `web_publishes_on_loopback_only`. Edit `deploy/README.md` in “Installing the client” and `deploy/test/README.md` after “Run C12 with disposable PostgreSQL”. Use `deploy/up.sh` and `start_server`. |

**Item 1. Private Compose and checkout fixtures.** Create only the two files in task 1. No predecessor. Items 2 and 4 can start alongside it. Items 3 and 5 need its fixture contract. Applies foundation decisions D4, D10, D17 and rules 9 and 10, foundation checks C1, C12 and C13, and sprint checks R1 through R5, R7, R11 through R14.

- Supply shell functions `sprint4_fixture_open <candidate-root> <candidate-commit>`, `sprint4_select <main|compat> <3|4>` and `sprint4_fixture_close`. Open records a canonical candidate root, requires its source to match the recorded commit for build evidence, and creates a private scratch directory. The caller sets a cleanup trap and calls close after fixture children stop. Select exports the effective Compose settings. It rejects `compat 4` because that project serves the old comparison server. Export `S4_FIXTURE_DIR`, `S4_LEGACY_ROOT`, `S4_BROWSER_PORT`, `S4_BROWSER_ORIGIN`, `S4_MOCK_PORT`, `S4_CLIENT_A_STATE`, `S4_CLIENT_B_STATE`, `S4_REPO_A`, `S4_REPO_B`, `S4_BAD_ORIGIN_REPO`, `S4_NON_REPO`, `S4_REPO_A_ALIAS` and `S4_REPO_A_SUBDIR`. State and checkout paths are absolute on each client fixture. Keep the manifest and environment files private.
- Extract the pinned Sprint 3 tree with `git archive f6cbea30` into `S4_LEGACY_ROOT`. Build from that tree, including its installer. Build the candidate from the recorded candidate tree. Use projects `lgtm-s4-<run>` and `lgtm-s4-<run>-compat`. The main project selects tag `s4-<run>-s3-f6cbea30` or `s4-<run>-s4-<candidate-short-sha>`. The comparison project uses the old tag and has its own volume, stack state and port. The main project's volume, `LGTM_STACK_STATE_DIR`, private password and `LGTM_WEB_PORT` stay constant on selection. A scratch caller uses an explicit `COMPOSE_FILE` with the selected revision's base file first, its fixture-only origin override second, and private `COMPOSE_ENV_FILES`. Clear inherited overrides which would change what those files select. Export the chosen web port for the old wave driver. Supply that driver's password from fixture settings in its subprocess environment with tracing off. Keep it out of command arguments and reports.
- Have Compose resolve the model privately before starting containers. Check its project, image tags, published web port and host binding, and state mounts. Require unused loopback ports with no choice of 8787 for either stack or any fixture listener. On a bind collision stop the attempted private run and choose a new fixture before proceeding. Confirm the actual web address through `docker compose port web 8080` for the selected project. Use the existing `up.sh` normal path. Cleanup uses the recorded fixture owner and labels to target the private resources. It releases the browser, mock, relay and client processes before deleting scratch and leaves the old proof images until both projects have stopped.
- Create a deterministic first commit with fixture-only Git settings and no network fetch. In A, store the remote head at `main`, check out `work`, and leave a dirty file. Use a qualified absolute, credential-free fixture origin. B is another checkout with a different name and the same first commit and origin. The negative checkout shares that commit and uses a different qualified origin. Keep a non-repository path, a nonempty subfolder of A, and an alias resolving to A. For two-machine cases use two separately issued tokens, approvals, machine identifiers and state folders. Tests may launch A and B on the same host. Each client process sees its own configured repository workspace. The clean positive and negative fixtures are reusable across test-specific enrollments.
- **Tests written first.** `sprint4-fixture-test.sh` writes `s4_private_stack_selection`, `s4_owned_cleanup` and `s4_two_git_workspaces`. Start from a stub and require the tests to fail on an effective-settings, cleanup, and Git-fact assertion respectively. Feed conflicting inherited settings and an unrelated Docker label through the existing fixture test pattern. Assert the selected volume identity survives tag change, scope limits hold under cleanup, and read the Git commit, branch, dirty state, origin and alias independently of the fixture.
- **Risky.** Reuse the default project name or 8787, remove the cleanup ownership check, and give B the same state directory as A, one mutation at a time. The corresponding fixture assertions must fail. **Acceptance.** The fixture suite prints the three named `ok` lines with exit 0.

**Item 2. Selective frame relay and private certificate fixture.** Create only the two Go files in task 2. The relay can be built and tested before item 1. Gate G3, item 3 and the independent check writer use it after the fixture contract is ready. Applies foundation D7, D17 and rules 6, 8, 9 and 10, C10 and C11, and sprint checks R5 through R11 where frame control is needed.

- Use the generated `LinkService.Open` interface to forward both directions to the actual Caddy endpoint. Forward `Join` to Caddy as well. Verify transparent forwarding of the signed Challenge, terminal error code and refusal metadata. Live fixture controls allow the driver to hold or release one direction at an exact numbered event or result, or close a stream and record the last observation. Keep controls in owner-only scratch on the fixture host. Observe only correlated public identifiers, event numbers, outcomes and payload digests. End blocked reads and writes on cancellation. This helper supplies network boundaries. The paired Go programs use the actual server and client seams for transaction and durable-append boundaries.
- Offer the test executable interface `sprint4relay --listen <addr> --upstream <url> --control-dir <dir>`, with optional `--tls-cert <file> --tls-key <file>` together. Print `relay ready` after binding. Put observations in `observations.jsonl` under the private control directory. Use an atomic replacement of `control.json` to arm or release the current hold. Agree the proposed fields `action`, `machine_id`, `direction`, `frame`, `event_number`, `command_id` and `attempt` with the independent writer before freezing its consumers. The upgrade may arm an event and a result together and discard the old held copies when their stream ends. The candidate must replay from disk.
- For certificate evidence, run a relay with Transport Layer Security (TLS) on a fixture-only Docker network and dial its network alias from a disposable client container. Publish no edge port to the host. The private Compose override attaches the edge and deployed Caddy service to the fixture network. Dial the Caddy container by its service name, using an allowed loopback Host header on that private upstream hop. Use a valid self-signed leaf and pin the Secure Hash Algorithm 256-bit digest of its encoded Subject Public Key Info. The fixture takes a disposable real token and changes only the client-side address and pin in the private copy. The server still redeems that token using its real verifier. Confirm a matching leaf reaches the real Join and Open. Substitute a second valid leaf with a different key under the same network name, then expect `certificate_key_mismatch` and zero forwarded application requests on the bad connection. Restore the matching key and confirm a real link again. Use the generated token type and feed the token on standard input. Keep the fixture independent of public certificate issuance.
- **Tests written first.** In `main_test.go`, `TestSprintFourRelayPreservesProofAndSelectsFrames` asserts that a matched frame is held across ordinary traffic, unrelated frames pass, terminal metadata survives, and cancellation releases blocked forwarding. `TestSprintFourRelayUsesPinnedLeaf` asserts a matching pinned client reaches the helper with Hypertext Transfer Protocol version 2 (HTTP/2) and a second key reaches no handler. Require red on the forwarding or listener assertions with only a compiling stub.
- **Risky.** Deliver a held result, strip the `Lgtm-Link-Refusal` header, or make the mismatched leaf control use the trusted key. Show the corresponding assertion failing after each change. **Acceptance.** Both named tests explicitly pass and the certificate part of G3 prints its success line.

**Item 5. Compose and release guide.** Create `deploy/test/sprint4-compose.sh`. Change only `deploy/README.md` and `deploy/test/README.md`. Item 1 comes first. Its static test and draft guide can follow item 1 alongside items 3 and 4. The final guide uses their validated commands. Applies foundation D1, D4, D10, D17 and rules 1, 2, 9 and 10, C1, C12 and C13, the Sprint 4 Projects registration, and R13 and R14.

- Keep `deploy/compose.yaml`, both Dockerfiles and `deploy/Caddyfile` as the existing production inputs. The existing build context includes `src/server/internal/projects/` and its embedded migrations, and the generated Go package. The current path matcher passes all three new calls to the server. Use the same PostgreSQL and OpenBao digest references. Keep the paired release tags explicit through `LGTM_IMAGE_TAG`, and leave the production default `local` for an ordinary development build. One immutable candidate tag identifies the pair of built Sprint 4 images. Store the old tag, candidate tag and image identifiers in the gate report without configuration secrets.
- **Tests written first.** `s4_projects_uses_existing_stack` in `sprint4-compose.sh` runs `docker compose config --format json` privately. It asserts the exact services and profiles of the checked-in baseline, the two selected image tags, one loopback web binding at the fixture port, no other published port, and the existing `pgdata` storage and private network topology. Make it first fail on an assertion with a stub fixture selecting the wrong tag and binding. Separately the independent checks require actual, authenticated calls through the built website, following the guarded router in the accepted contract.
- **Risky.** Mutate the fixture image tag selection or publish a second service. The static assertion must fail. Change the served build to prompt for a server address or route Projects outside the existing proxy in a scratch override. The deployed browser acceptance must fail. **Acceptance.** The static check prints `ok 1 - s4_projects_uses_existing_stack` and exits 0. The guide supplies operator commands and the exact evidence rows in the joint gate.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| The harness starts, probes or cleans an installed stack. | High | Validate the resolved model and actual loopback binding. Use project labels and private paths for cleanup. |
| A model check passes while the served bundle calls the wrong backend. | Medium | Run signed-in browser checks against the real served image and the comparison build. Record the same-origin request on the deployed path. |
| Holding whole connections obscures which event or result the test exercised. | Medium | Select frames by identity and number and retain sanitized observations. Use the designated in-process seams at commit and receipt boundaries. |

**Verify.** Use the item suites and G1, G2, G4 and G5. Use Linux with the existing Docker and Go prerequisites, a real PostgreSQL container, a browser, and two client processes. Use the private Docker network for the certificate case. Website UI changes belong to the Website plan. These tasks exercise the approved screens.

**Design artifact.** The browser gate uses the approved text in [The website in Sprint 4](../../../docs/specs/2026-11-27-running-an-agent-design.md#the-website-in-sprint-4) and the unchanged deployed website image build.

**WS-2. An installed client keeps its history and stops when it needs recovery**

Exercise the installer as the client user, with a real Sprint 3 service on its recorded state folder. The private fixture lets the installer, PostgreSQL reads, browser and link observations refer to the same installation.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 5 | The old binary is refused by the new server, then the same machine reconnects through an installer rerun with the saved state. | G3 prints `ok 1 - verified_revision_one_refusal`, `ok 2 - installed_state_retained` and `ok 3 - retained_event_and_receipt_replayed`. |
| 6 | The actual service handles both kinds of exit on each supported operating system. | G6 prints `ok 1 - requested_restart_restarts_service` and `ok 2 - damaged_history_stops_service` on both systems. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Prove the forward installer and database upgrade. | New `deploy/test/sprint4-upgrade.sh`. Patterns are `deploy/test/checks.sh` with `check_c9` and `check_c11`, `deploy/test/installer-live.sh` with `live_rerun_keeps_machine`, and `deploy/install-client.sh` with `main` and `join_stack`. |
| 4 | Prove the installed supervisor's exit behavior. | New `deploy/test/sprint4-supervisor.sh`. Patterns are `deploy/test/installer-live.sh` with `live_reconnect_restarts_service`, `live_removed_machine_stays_stopped` and `unit_owned_by_fixture`, and the two shipped templates' restart policies. |

**Item 3. In-place database and client upgrade.** Create `deploy/test/sprint4-upgrade.sh`. This is Stack's separate deployment proof for installed state. The orchestrator still writes the independent R11 assertion. Items 1 and 2 come first. Item 4 can proceed on another disposable user and fixture. Applies foundation D4, D5, D7, D10, D16 and rules 2, 4, 6 through 10, C4 and C9 through C13, and sprint R11.

- Start the main project on Sprint 3 using `sprint4_select main 3` and the legacy tree's `deploy/up.sh`. Build both candidate images before arming any replay control. Leave the old containers running during that build. Enroll the fixture through the relay with a real one-use token and install it using the Sprint 3 installer in its own disposable Linux user. Use pre-enrollment for this one fixture so the installer points to the controlled link while still checking the selected Compose endpoint. Rewrap the loopback token's address as the existing paired fixture does. Record the old source commit and installed executable digest, since an exported tree can produce the installer's development version label. Wait for approval, a real online projection and an authenticated browser session. Record the published address, PostgreSQL volume, OpenBao signing-key fingerprint, machine identifier and approval, stable identity and key digests, state path and modes, and a completed prior command as a server-side sentinel. Keep the session cookie private. Count the existing machine rows for the fixture's identity. A normal first-time install is covered in the separate supervisor case.
- On the old live stream, run `ClientService.ScanClientHarnesses` for the fixture machine after startup reports are drained. Hold the corresponding durable report at the relay and withhold its first `CommandResult` from the server. Read the finished receipt and numbered event from the quiesced `client.db` with a read-only Bolt database view, using its revision 1 layout. Record only the number, digest, command identifier, saved outcome and attempt. Require the server's event mark below the held number and the command unsettled. An inert version probe in the fixture path measures the scan independently. Control for the old and new startup inventory by marking their runs separately and expecting a new scan to increment the probe only in a positive control. The old binary must create the saved event and receipt itself.
- Stop or pause the fixture client while the server and website are updated. Use `sprint4_select main 4` on the same project and state, with the same password and port. In the fixture, start the prebuilt server first with `docker compose up -d --wait --wait-timeout 540 --no-build --no-deps server`, then the prebuilt web with `docker compose up -d --wait --wait-timeout 90 --no-build --no-deps web`. This keeps image builds outside the command replay window. Record the old command's expiry and show that replay precedes the 23-hour offer cutoff. Bound the test waits by the readiness and delivery budgets. Wait for server health, an authenticated repository snapshot, and `GetServerResponse.services` including `lgtm.v1.ProjectService` while contract revision stays 1. The old cookie must still authorize the website. Resume or invoke the original binary on that enrollment. Require a valid signed Challenge followed by `failed_precondition` with `Lgtm-Link-Refusal: link_revision_mismatch`, with no Welcome or new admission and no local event or receipt lost. For the deployed trace, the control records a Challenge and refusal and the actual old binary reports the verified refusal. Check that Machines shows the existing identifier offline. Quiesce the old service before rerunning the installer.
- Rerun `deploy/install-client.sh` as the same user with the original recorded directory and no new token. Expect the installer's `lgtm-client is already joined.` line and a new executable built from the candidate. Compare the fixture token issuance count across the rerun. Require the same machine online with its approval, the same enrollment key and server identity, the same session and volume, and the previously settled sentinel command intact. Require owner-only directory and state files. Observe the original held event applied once at its exact number and digest. The server advances and acknowledges it. The current attempt of the same stored command receives the saved outcome with `CommandResult.last_event_number` zero and settles once. The enrolled machine count stays one, and the measured scan adds no execution from replay. Confirm a subsequent fresh command runs. Use the relay and application observations together. A successful HTTP response alone cannot prove that the old records survived.
- **Tests written first.** In `sprint4-upgrade.sh`, write the three named criteria plus `off_loopback_certificate_pin` using item 2's helper. Capture preconditions before reporting an assertion failure, so a missing candidate or missing old artifacts cannot yield passing results. Freeze the checks with a stub where the assertion sees the old server after the requested switch, or sees no recovered old report or receipt. The certificate case needs both the working and wrong-leaf controls.
- **Risky.** Skip the verified refusal, switch to a fresh volume or identity, clear the old receipt in the disposable fixture, or change the event identity before replay. Show failure at the affected criterion. Do the equivalent of clearing the receipt before startup and use the scan count or changed result to catch the repeat. **Acceptance.** G3 emits its four success lines and exits 0.

**Item 4. Terminal history and requested restart under systemd and launchd.** Create only `deploy/test/sprint4-supervisor.sh`. It can start against a fixture stub in parallel with item 1. Its positive and terminal tests need the candidate stack and the Client team's status convention. Neither template needs a new restart rule with exit 0 on terminal history and 75 on requested restart. Applies foundation D7, D16 and D19, rules 7 through 10, C8 through C10 and the contract fold's local terminal history proof.

- On Linux use the actual shipped `lgtm-client.service`. On macOS use `dev.lgtmstudio.client.plist` in a logged-in disposable user. Use the same private local Compose selector as the installer's port discovery. When the Docker engine runs in a Linux virtual machine for macOS, start the private candidate stack on the Linux side and make its selected port available on the Mac's loopback. Run the supervisor script with `--prepared` in that case. It must verify the passed private selector, fixture owner, image and published port before invoking the installer on macOS. Validate the address on the Mac before installation. A fixture enrollment, process and supervisor label must be owned by the disposable user and scope. Run the candidate installer unchanged.
- First, approve the machine and see it online. Call `ClientService.ReconnectClient`. Record 75 from the manager, a different process identifier, and the same machine online. Then stop the service through the manager and copy its approved state directory. Alter only the disposable `client.db` with a corrupt required history case. Keep `machine.key` and `identity.json` valid. Start the installed service once, capture the scoped log interval and manager's exit status, and observe the new process leave. Require one local recovery instruction and exit 0. Observe at least 12 seconds after the exit, covering two configured five-second intervals. Check no new start or process via manager counters and scoped logs, and no new successful admission or event mark in the server fixture. Check that no enrollment or history is replaced. Restore the saved fixture state and prove the original identity connects following one manual start. Run the missing-history and incompatible Welcome cases in the contract's paired tests as well.
- On Linux use `systemctl --user show lgtm-client.service` for process, status and restart observations and `journalctl --user -u lgtm-client.service` for the new log interval. On macOS use `launchctl print gui/$(id -u)/dev.lgtmstudio.client` for the process and last exit, and the installed launch agent's log path for the diagnostic. Record the manager's event evidence at the transition, so a successful new process cannot hide exit 75. Verify the fixture service against the recorded path before any lifecycle action, and wait for child processes to leave before cleanup. This sprint check uses its own test entry point and run, separate from the existing installer test involving a reboot.
- **Tests written first.** `requested_restart_restarts_service` asserts 75, one replacement process and the same online machine. `damaged_history_stops_service` asserts status 0, one local instruction, unchanged enrollment, no replacement after the interval, and successful manual recovery from the original copy. Write against the existing supervisor interfaces and prove each red with a test mode that returns the wrong status or produces two starts. Validate against the real supervisors in G6.
- **Risky.** Return 1 for terminal history, return 0 for a requested restart, or change either service to restart on successful exit. Show each affected assertion fail in a disposable run or the independent client test followed by the service reproduction. **Acceptance.** G6 records both named passes with the measured exit and process observations on Linux and macOS.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A fresh installation appears to be a successful upgrade. | High | Compare the existing volume, session, enrollment, approved identity, old event and command ledger across cutover. |
| A retried command runs again after the installer updates the binary. | High | Hold the result after its durable receipt. Correlate the new attempt and saved outcome with the version-probe count. |
| A service crashes repeatedly or normal restart silently stops working. | High | Observe both statuses on each shipped supervisor. Check process and log evidence past two intervals and exercise a positive restart. |

**Verify.** Run G3 and G6 plus `bash deploy/test/installer.sh` as regression coverage. The deployed work needs a disposable Linux user with a real systemd user manager. G6 also needs a logged-in disposable macOS user and a private stack reachable through the installer. Stand-in tests supply initial red evidence. Both real device results are required for the Sprint 4 supervisor case.

**Exact interfaces for the joint gate**

| Boundary | Public names and observations |
|---|---|
| Client and launchers | `deploy/up.sh`, `deploy/install-client.sh`, `lgtm-client join <address> -`, `lgtm-client run`, `LGTM_CLIENT_STATE_DIR`, `lgtm-client.service`, `dev.lgtmstudio.client`. Expected client exit values are 0 for terminal history and 75 for requested restart. |
| Compose | `COMPOSE_FILE`, `COMPOSE_ENV_FILES`, `COMPOSE_PROJECT_NAME`, `LGTM_STACK_STATE_DIR`, `LGTM_IMAGE_TAG`, `LGTM_WEB_PORT`, `LGTM_WEB_PASSWORD`. Running services are `postgres`, `openbao`, `server` and `web`. |
| Machine link | `LinkService.Open`, `Hello.link_revision`, `Hello.run_id`, `Hello.first_event_number`, `Welcome.link_revision`, `EventAck.through`, `CommandResult.last_event_number`, `Command.action.look_at_folder`, `Event.report.folder_seen`, `LookAtFolder.operation_id`, `FolderSeen.operation_id`, `FolderSeen.command_id`. The verified legacy refusal is `failed_precondition` plus `Lgtm-Link-Refusal: link_revision_mismatch`. |
| Website and discovery | `GetServerResponse.services`, `lgtm.v1.ProjectService`, `ProjectService.WatchRepositories`, `ProjectService.InspectFolder`, `ProjectService.AddRepository`, `Lgtm-Request-Key`, `WatchRepositoriesResponse.change.all`, `WatchRepositoriesResponse.change.saved`, `WatchRepositoriesResponse.change.removed_id`, `Repository.id`, `RepositoryFolder.id`, `RepositoryFolder.client_id`, `RepositoryFolder.resolved_path`, `FolderFacts.main_branch`, `FolderFacts.checked_out_branch`, `FolderFacts.remote`, `FolderFacts.clean`. Website revision stays 1. Projects and Link own the fixed refusal codes and sentences. `lgtm.v1.Client.event_history_needs_recovery` is part of the paired history case. |
| In-process harness | `servertest.Start`, `clienttest.Join`, `clienttest.Run`, `LGTM_TEST_POSTGRES`, `src/contracts/testdata/link-v2-limits.json`. Server Link and Client Folders supply the respective transaction and durable-append interruption points. Read the contract fixture values for the 65,536-byte link message, 49,152-byte command and event, 4,096-byte typed path, 8,192-byte resolved path, 2,048-byte remote and 32 unacknowledged events. |

**Rollout detail for steps 1 and 5.** The orchestrator writes the canonical R1 through R14 checks independently in the proposed `deploy/test/checks-sprint4.sh`, plus the five named in-process tests under `src/linktest/`. The driver reads fixture settings and `--only R1,R2,...` and reports one Test Anything Protocol (TAP) result per selected check. Its gate also requires Stack's separate deployment evidence. Any failed or skipped required scenario leaves the gate red. Store the tested commit, image identifiers, result lines and sanitized frame or supervisor observations. The fixture logout and cleanup happen after evidence is captured.

For G1's running checks, G4 and G5, set `S4_CANDIDATE_COMMIT` to the integrated commit and `S4_CONTRACT_COMMIT` to the reviewed landed contract commit. Confirm `git merge-base --is-ancestor "$S4_CONTRACT_COMMIT" "$S4_CANDIDATE_COMMIT"` exits 0. From the candidate root, open the fixture in a dedicated shell. Set the script's cleanup trap before creating Docker resources. After G1's private C checks, start a fresh main fixture for G4 and G5. Use the same fixture for these two gates, and start the comparison server before G5. Save both published addresses in the private manifest.

```bash
set +x
set -e
source deploy/test/sprint4-fixture.sh
sprint4_fixture_open "$PWD" "$S4_CANDIDATE_COMMIT"
trap sprint4_fixture_close EXIT
(cd "$S4_LEGACY_ROOT" && go build -o "$S4_FIXTURE_DIR/lgtm-client-r1" ./src/client/cmd/lgtm-client)
sprint4_select compat 3
bash "$S4_LEGACY_ROOT/deploy/up.sh"
sprint4_select main 4
bash deploy/up.sh
```

Both starts must print `phase server done`, `phase web done`, and `the website is at` with the respective private port. The prerequisite for G1's C checks is the main selection only. C1 starts its stack. G3 and G6 open and clean their own fixtures.

| Gate | Command or operator step | Required observation |
|---|---|---|
| G1. Contract, source and wave 1 | During contract build review, run `bash deploy/test/with-postgres.sh -- env LGTM_LINK_BASE=fcfa96c3 bash scripts/check-go.sh`. Once the reviewed contract lands, run it on the candidate with `LGTM_LINK_BASE="$S4_CONTRACT_COMMIT"` through the same wrapper. Run `pnpm --filter @lgtm/api check`, `pnpm --filter @lgtm/frontend typecheck`, `pnpm --filter @lgtm/mock-server typecheck`, and `node scripts/check-private.mjs` on that candidate. In the selected private Sprint 4 project, run `bash deploy/test/with-postgres.sh -- bash deploy/test/checks.sh`. | Each command exits 0. The Go gate prints passing package summaries and no frozen-record refusal. The privacy check has empty output. The wave driver prints exactly one `ok` line each for C1 through C13. C12 reports that the feed-lock mutation turned its named assertion red. |
| G2. Real client and server together | `bash deploy/test/with-postgres.sh -- go test -v ./src/linktest -run '^TestSprintFour' -count=1`. | Explicit `--- PASS:` for `TestSprintFourHistoryRecovery`, `TestSprintFourPendingRun`, `TestSprintFourWaitingReplay`, `TestSprintFourReceiptReplay`, and `TestSprintFourFolderSettlement`, and zero skips. The report captures durable state at both interruption points. It captures the staged result across restart and sweep, the three occupied permits and unchanged claim cursor, cumulative acknowledgements, and the exact byte and 32-event limits. |
| G3. Deployed upgrade and certificate | `LGTM_SPRINT4_LIVE=1 bash deploy/test/sprint4-upgrade.sh "$S4_CANDIDATE_COMMIT"` as a disposable Linux service user. The script owns its private projects and invokes the paired old and new installers. | `ok 1 - verified_revision_one_refusal`, `ok 2 - installed_state_retained`, `ok 3 - retained_event_and_receipt_replayed`, and `ok 4 - off_loopback_certificate_pin`. Require exit 0. |
| G4. Independent repository and link checks | In the private candidate fixture run `bash deploy/test/with-postgres.sh -- bash deploy/test/checks-sprint4.sh --only R1,R2,R3,R4,R5,R6,R7,R8,R9,R10,R11,R12`. Provide the legacy binary, fixture workspaces and frame controls. | One line starting with `ok <n> - R<n>` for each of R1 through R12, with zero skips and exit 0. Run in design order once all inputs are ready. |
| G5. Browser and real stack | Run `bash deploy/test/checks-sprint4.sh --only R13,R14` using the private comparison project, the selected candidate project, and a mock process. Keep the measured browser observations below. | One `ok 13 - R13` line and one `ok 14 - R14` line, with no skip, and exit 0. Keep the candidate's actual same-origin request and the real stored identifiers from the R14 browser run. |
| G6. Installed restart and stop | Run `LGTM_SPRINT4_LIVE=1 bash deploy/test/sprint4-supervisor.sh "$S4_CANDIDATE_COMMIT"` on Linux and on macOS. Use `--prepared` before the commit argument with a private stack already started for the Mac. | Each run exits 0 and prints `ok 1 - requested_restart_restarts_service` and `ok 2 - damaged_history_stops_service`. Record the distinct process identifiers for 75, and the single recovery line, 0, and no replacement over the 12-second observation for terminal history. |

**On-ramp for the independent checks.** The checks stay red until the owning components and their fixtures are available. This ordering groups dependencies. The final run still checks the published design numbers.

| When a group can pass | Checks | Inputs and evidence that the gate needs |
|---|---|---|
| The revision 2 link, deployed predecessor and client replay are built. | R11 | Use the real old binary, buffered event and completed receipt. Link can also prove its ordering interfaces at this point with existing command and report types. |
| The real client Folders part and the server Projects part are wired. | R1, R2, R3 and R4 | One checkout with unequal checked-out and main branches, a dirty change and remote for R1. Give R2 a valid resolved folder and an actual non-repository. Offer the alias and subfolder of the same checkout concurrently for R3. Offer B with the shared first commit and matching origin and the wrong-origin checkout for R4. |
| The real folder operations work and the independent writer has the interruption seams. | R5, R6, R7, R8, R9, R10 and R12 | For R5 retain the request key and database through server restart, and prove one inspection. For R6 interrupt after durable `FolderSeen` append and before receipt finish, then prove the success survives `interrupted`. For R7 enqueue while connected, disconnect before command delivery, and observe `deadline_exceeded` after the 25-second caller wait. Reconnect after the 20-second command expiry. Require the decline, failed operation and zero inspections. For R8 use two commands that reach the client in committed order, including a delayed commit. For R8 and R9 occupy all three shared send permits while old events replay. Observe application before command claim and an unchanged cursor until the same stream becomes ready. Include the no-old-event case. For R10 observe a staged result across server restart and sweep, then a single settlement after the awaited event. For R12 keep a watch open, and check both a repository whose last folder goes and a repository with a surviving folder, plus the failed open operation. |
| The website and the deployed images are ready, with R1 through R12 green. | R13 | R13 uses one build with `VITE_LGTM_SERVER` unset. Run `env -u VITE_LGTM_SERVER pnpm --filter @lgtm/frontend build --outDir "$S4_FIXTURE_DIR/browser-dist"` and serve that output on `S4_BROWSER_PORT` at `S4_BROWSER_ORIGIN`. Start the real mock with `LGTM_MOCK_HOST=127.0.0.1 LGTM_MOCK_PORT="$S4_MOCK_PORT" pnpm --filter @lgtm/mock-server start`. Use fresh browser profiles for `sample`, mock, old real and candidate real, and compare asset digests. Both real fixture servers list the exact preview origin in `LGTM_ALLOWED_ORIGINS`. Sign in and wait for the watches to initialize. The first three profiles show sample projects. The fourth advertises Projects and shows real machines and repositories without sample projects. |
| R13 confirmed the real data mode. | R14 | Use the candidate's actual `web` image with `VITE_LGTM_SERVER=/`. Confirm a same-origin `GetServer` request and the absence of a server-address prompt. Open Add repository from the sidebar and from Machines. Verify each dialog selects the intended real machine and reflects the fixture branch, remote and dirty status. Add the fixture with both entry points using distinct checkout names or attach the second folder, compare saved repository and folder identities, and verify the tree after reload. Exercise reply-before-watch and watch-before-reply, replace a reconnect snapshot, and discard superseded inspection replies after editing the intent or switching session or server. Use the approved Sprint 4 design as the browser text brief. |

**Rollout detail for steps 2 through 4.** Record the existing Compose selector, credential source, port, image tags and volume. Preserve the PostgreSQL volume and the stack state together. On the selected project run the candidate's `deploy/up.sh`. Expect the `phase server done` and `phase web done` lines and `the website is at` with the original loopback port. Confirm the existing session and the Project service on revision 1 of the website contract. Once healthy, run the candidate's `deploy/install-client.sh` as the installed user and confirm `lgtm-client is already joined.` and the same machine online. Use the service logs for the authenticated mismatch and local recovery instructions. Preserve the failed installation for diagnosis if migration or history recovery blocks progress. The deployment check covers forward migration on the existing stores. The contract's separate database-rollback limitation still applies.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is the five assignment deliverables and the shared assignment rules, foundation decisions D1, D4, D5, D7, D10, D16, D17 and D19, foundation rules 1 through 11 and checks C1 through C13, the Sprint 4 revision, Projects, folders, removal, website and R1 through R14, both third-round closing lists as they apply to Stack, the contract fold's Stack row and names, issues 1632 through 1635, the epic team boundary and the build loop Phase 4.

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
| Sprint 4 link revision and R6 through R11 | WS-1, WS-2 | Gates G2 through G4 cover result order, run gates, expiry, exact interruption boundaries and upgrade. |
| Sprint 4 Projects, folders, removal and R1 through R5 and R12 | WS-1 | Item 1 supplies the checkout and machines. G4 uses the real Projects calls and durable results. |
| Sprint 4 website and R13 and R14 | WS-1 | G5 covers the same-build modes, the actual website image and both entry points for adding a real repository. |
| Logic and fit third-round closing lists | WS-1, WS-2 | Decision rows 12 and 13 name the applied parts and the Sprint 5 disposition. G2 and G5 retain their observations. |
| Contract fold Stack request and exact names | WS-1, WS-2 | G1 through G6 use its names and freeze transition, exact in-process passes and required deployment evidence. |
| Issues 1632 through 1635 | WS-1, WS-2 | Use Go and observe ordered database behavior and connected restart, verify the private TLS edge, use public paired-program test entry points and rate the five work items. |
| Build loop Phase 4 | WS-1, WS-2 | The test writer proves the initial failure, the builder preserves the frozen assertions, and the lead requires a passing report and the specified mutation evidence on the built commit. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, “Where things are” | Deployment work targets `deploy/`. Old engine folders stay reference material. Product interface requirements use the reviewed Protocol Buffers contract. |
| `AGENTS.md`, “Credit copied code” | The plan uses existing repository patterns. If a builder copies an external implementation, use the required license and `NOTICE` process through the owning team. |
| `AGENTS.md`, “Write down what you find” | Planning findings are recorded in this bounded plan and its receipt. Route any new product issue to the orchestrator for the repository issue log. |
| `AGENTS.md`, “Finished work reaches the base branch through a pull request” | The orchestrator owns integration. The gate report names the exact candidate commit. |
| `AGENTS.md`, “This repository is not a fork of Herdr Synapse” | The fixture uses only the LGTM Studio checkout and local images. |
| `AGENTS.md`, “Writing style for documents” | Use dated sources and repository-relative paths. Run the repository privacy check and inspect the new planning files directly. |
| Epic team table and Phase 4 | One writer per build file, test-first and frozen assertions, isolated mutation evidence, and orchestrator ownership of joint tests. |
| Assignment write boundary | Only this plan folder and its result file are written in planning. The scope check for `stack/infra` passes. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Compose and checkout fixture | infra | `stack/infra` | medium | It combines existing Docker ownership and client state patterns with two Git workspaces. | Yes | None. |
| 2. Frame relay and certificate edge | infra | `stack/infra` | medium | One test helper reuses generated streaming interfaces and adds private, selected frame control. | Yes | None for unit tests. Item 1 for deployed use. |
| 3. Installed upgrade proof | infra | `stack/infra` | medium | One runner checks the old and new installations across ordered lifecycle steps. | Yes | Items 1 and 2, candidate Link and Client, and mounted Projects for final acceptance. |
| 4. Supervisor exit proof | infra | `stack/infra` | medium | One runner checks two existing platform service managers and their recorded exit states. | Yes | Item 1 for live use and the Client exit convention. |
| 5. Compose proof and guide | infra | `stack/infra` | low | One declarative configuration assertion and edits to two existing guides use established launch paths. | Yes | Item 1 for static checks and items 3 and 4 for final guide verification. |

Load. OpenAI supplies the planner, test writers, builders and reviewers. Budget one planner, five test writers, five builders, three plan review seats and two focused build review seats, for 16 launches including this planner. The current remaining vendor allowance was unavailable in this bounded session. The orchestrator checks that the allowance covers the remaining 15 planned seats before dispatch and schedules later batches if needed. The orchestrator's independent design-check writer and paired-test capacity are tracked at the sprint level.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| Pending | Not yet scored. | The bounded planner hands the self-checked plan to the orchestrator for the assigned review round. |

</details>
