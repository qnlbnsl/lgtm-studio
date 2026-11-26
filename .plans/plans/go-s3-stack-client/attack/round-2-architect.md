REVISE

A failed rejoin can leave a working client stopped because the installer moves its state before the replacement joins and defines no rollback if the service switch fails.

## Findings

### architect-r2 01 · MAJOR · D16 · Rejoin can strand the working installation

**Evidence by reading.** The overview promises to preserve the working installation until join succeeds. The steps stop the service and move its state before calling the client, roll back only for a failed join, and replace the executable and unit before reloading or starting the service. The post-build check expects the old machine online with the server held down. The client can retry a join for up to 15 minutes. `.plans/plans/go-s3-stack-client/plan.md:28`, `.plans/plans/go-s3-stack-client/plan.md:238`, `.plans/plans/go-s3-stack-client/plan.md:244`, `.plans/plans/go-s3-stack-client/plan.md:379`, `.plans/plans/go-s3-stack-client/decisions.md:49`, `.plans/plans/go-s3-client-link/plan.md:103`.

**Failure.** The token command succeeds and the server then drops the connection. The healthy old installation stays stopped for the entire retry budget. Alternatively, the join succeeds and systemd refuses the new unit's reload or start. The old state is in the archive, but the installed program and service now point to the replacement. An interruption after the move also leaves no automatic route back. The rollback on join failure starts any existing unit, including one the owner had stopped before the run.

**Fix.** Join using a fresh, private sibling directory while the old service runs. Define the commit point for switching state, executable and service. Preserve the old artifacts and the service's prior active or stopped state until that switch succeeds. Define recovery after an interrupted switch and identify any uncommitted waiting enrollment the owner needs to remove. Test a failed join, a failed switch and interruption at the switch. For the server outage check, restore the server before asserting the old machine is online.

### architect-r2 02 · MAJOR · D19 · The installer lock follows a setting that concurrent runs can change

**Evidence confirmed live.** Two `mkdir` calls for different selected state-folder parents both acquired the specified lock name. By reading, both runs still write the same executable and service. The installer reads the unit before taking the lock, and `--remove` uses the same state-scoped lock. `.plans/plans/go-s3-stack-client/plan.md:84`, `.plans/plans/go-s3-stack-client/plan.md:91`, `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:230`, `.plans/plans/go-s3-stack-client/plan.md:299`.

**Failure.** One operating-system user starts an install with a state override while another shell of that user starts a plain install or removal. Both pass the lock. Two installs can enroll twice and race to replace the unit. Removal can delete the executable after the other run has installed it.

**Fix.** Use one fixed lock per user and installation, keyed to the shared executable and service. Take it before reading installed settings or touching a service on either platform. Define how the next run handles a stale lock after a crash, including the case where a previous switch needs recovery. Exercise two different state selections and a concurrent removal.

### architect-r2 03 · MAJOR · D19 · The file-name check admits directories the client never owned

**Evidence confirmed live.** A disposable directory with only an ordinary `repositories` child passed the proposed name test and could be moved into the archive. By reading, the client itself checks the folder type, user ownership and private mode before opening its lock. Its guard runs after the installer has moved the selected folder. `.plans/plans/go-s3-stack-client/plan.md:236`, `.plans/plans/go-s3-stack-client/plan.md:240`, `src/client/internal/state/folder.go:63`.

**Failure.** The user points `LGTM_CLIENT_STATE_DIR` at a folder that holds their own projects under `repositories`. The top-level names pass. Rejoin succeeds and the projects disappear from their original path into the archive. The same predicate also accepts an empty directory or one containing only files with the temporary prefix.

**Fix.** Canonicalize the proposed path and authorize an existing folder using the installed service's binding or client-validated enrollment or pending-key evidence. Check its ownership and type before moving anything. Refuse ordinary existing directories and paths that contain the executable or service. Specify how to enroll into a new override without moving the previous installation. Test an unrelated folder containing only allowed names.

### architect-r2 04 · MAJOR · C6 · The launcher, installer and sprint commands still select different Compose projects

**Evidence confirmed live.** Docker Compose 5.5.0 resolved different project names and published ports when a disposable root and deployment folder each had a settings file. The launcher's explicit `-f` selected the deployment settings. The installer's exported default `COMPOSE_FILE` selected the root settings. By reading, the user-flow helper forces its own `-f`, and the sprint's build and token commands name neither a file nor a working folder. `.plans/plans/go-s2-stack-web/plan.md:236`, `.plans/plans/go-s2-stack-web/plan.md:275`, `.plans/plans/go-s3-stack-client/plan.md:83`, `.plans/plans/go-s3-stack-client/plan.md:366`, `.plans/plans/go-s3-stack-client/plan.md:370`, `deploy/test/checks.sh:22`, `deploy/test/checks.sh:378`.

**Failure.** The owner starts the documented stack with a project and nondefault port only in the deployment settings. A root settings file causes the installer to ask for the port from another project. Exporting the port for the sprint checks cannot repair the project mismatch. A supplied Compose file list also leaves the checks' restart and override operations on the default file.

**Fix.** Use the launcher's default absolute file and the same rules for explicit file lists, project selection and environment files in every installer and live-test call. Make the test writer's matching changes to the shared driver, including its direct override call, prerequisites of the sprint gate. Derive the probe port from that same project after startup. Verify the default and an explicit file list with conflicting root and deployment settings in a disposable fixture.

### architect-r2 05 · MAJOR · C9 · The live check can adopt an enrollment outside its fixture

**Evidence by reading.** The installer gives a nonempty state override precedence over the installed unit. Removal preserves state. The live test checks for the default folder, records a selected state folder, and later runs removal with the current environment. `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:230`, `.plans/plans/go-s3-stack-client/plan.md:327`, `.plans/plans/go-s3-stack-client/plan.md:329`.

**Failure.** A previous installation used an override and was removed. With the override still exported, the live test sees no executable, unit or default state. It reinstalls and starts the saved enrollment before the assertion for a newly waiting record fails. After a reboot, the recorded server identifier also does not prove that the currently installed unit still belongs to the test.

**Fix.** Give the fixture its own explicit private state folder and controlled service configuration. Check the actual executable, unit and selected folder before running. Persist the resolved stack selection and validate both the selected stack and the local identity against the record before the reboot and cleanup steps. Refuse cleanup when the currently installed service belongs to another enrollment.

### architect-r2 06 · MAJOR · D16 · A healthy process can conceal a failed connection

**Evidence by reading.** A rerun looks for the address as text and checks that the unit stays active with the same process identifier for five seconds. The client retries a signing-key mismatch and can delay between connection attempts for up to 15 seconds. The sprint test only reruns against the original healthy server. `.plans/plans/go-s3-stack-client/plan.md:234`, `.plans/plans/go-s3-stack-client/plan.md:246`, `.plans/plans/go-s3-stack-client/plan.md:340`, `.plans/plans/go-s3-client-link/plan.md:192`, `.plans/plans/go-s3-client-link/plan.md:202`.

**Failure.** A separate stack uses the same host port after the original stack is stopped. Its readiness response passes and the saved address matches. The client keeps retrying because the server key differs. The installer exits successfully while the enrollment cannot connect. A terminal refusal that arrives after the five-second sample can likewise leave the service stopped after the installer reports success.

**Fix.** Define a bounded startup outcome owned by the client that distinguishes a verified connection, a terminal refusal and continued retry. Have the installer consume it for the invocation it just started. Waiting and approved enrollments can both have verified connections. Use the client's address parser for normalized comparisons and keep automatic retries for ordinary outages. Test a different signing key at the same address and a refusal after the current five-second window.

### architect-r2 07 · MAJOR · D16 · The reboot probe measures the latest unit start

**Evidence by reading.** The live test compares `ActiveEnterTimestampMonotonic` with the sessions of class `user` present when it runs. The unit may restart after a failure, and the next test observes the stack only after login. `.plans/plans/go-s3-stack-client/plan.md:250`, `.plans/plans/go-s3-stack-client/plan.md:341`, `.plans/plans/go-s3-stack-client/plan.md:342`, `.plans/epics/go-rewrite/epic.md:30`, `.plans/plans/go-s2-stack-web/plan.md:200`.

**Failure.** The client starts before login and restarts after login but before the operator runs the check. The current activation timestamp then rejects a valid boot start. A run with no qualifying session can also vacuously satisfy the comparison. An automatic login hook that starts the stack can make the eventual-online check green without establishing Docker-driven recovery.

**Fix.** Record the earliest service invocation from the new boot and require a positive, identified first interactive login observation. Use boot-scoped evidence for the selected containers and the real server's readiness, then verify the same machine comes online. Define the supported Linux service-manager and Docker setup and the expected result for a failed observation. Keep the separate macOS login test and make the platform scope explicit in the epic.

### architect-r2 08 · MINOR · D17 · The token command still describes an address input that was removed

**Evidence by reading.** The connection policy, Store close and vault signature now align. The command still says it passes an empty address, and the decision log says an empty address selects the issuer's address. The Machines plan's exact `TokenOptions` has only `HostsServer`. `.plans/plans/go-s3-stack-client/plan.md:154`, `.plans/plans/go-s3-stack-client/plan.md:155`, `.plans/plans/go-s3-stack-client/decisions.md:46`, `.plans/plans/go-s3-server-machines/plan.md:145`, `src/server/internal/store/pool.go:143`, `src/server/internal/store/pool.go:290`.

**Failure.** A builder or test writer following the empty-address instruction supplies a removed field or an extra argument. That call cannot compile.

**Fix.** Spell the complete `Tokens.Create(ctx, machines.TokenOptions{HostsServer: flag})` call in item 1. State in the decision log that the issuer is the sole writer of the token's address.

### architect-r2 09 · MINOR · C6 · The merge test has no observation for the executable rename

**Evidence by reading.** The prescribed order assertion reads `calls.log` for a rename. The listed stand-ins log the Go build, client invocation and service commands, but none records the rename. `.plans/plans/go-s3-stack-client/plan.md:243`, `.plans/plans/go-s3-stack-client/plan.md:254`, `.plans/plans/go-s3-stack-client/plan.md:263`, `.plans/epics/go-rewrite/epic.md:71`.

**Failure.** A test writer freezes the specified assertion and the builder follows the prescribed rename. The expected entry never appears. Observing the final executable alone would let a premature replacement pass.

**Fix.** Make the stand-in client record the installed executable's absence on first join and its original bytes on rejoin. Keep the existing service order assertion and mutate the executable replacement to happen before join to prove the new observation fails.

### architect-r2 10 · MINOR · D19 · Removing the service requires build tools

**Evidence by reading.** The installer checks Go, Docker, curl and jq before reaching its removal branch. The removal branch only uses filesystem operations and the service manager. The same tool check precedes the macOS branch's removal. `.plans/plans/go-s3-stack-client/plan.md:225`, `.plans/plans/go-s3-stack-client/plan.md:230`, `.plans/plans/go-s3-stack-client/plan.md:299`, `.plans/plans/go-s3-stack-client/plan.md:305`.

**Failure.** The owner uninstalls Docker or Go before removing the client. The installer exits before it stops the enabled service, leaving it active or retrying at the next login.

**Fix.** Check only the service manager and path prerequisites before removal. Check build and enrollment tools on the install and rejoin paths. Exercise removal with those tools absent.

## Answers to the round 2 assignment

### Each patch in the register

1. The overview now qualifies Linux and macOS startup and treats the server-machine mark as provenance. The preservation promise and reboot evidence remain open in findings 01 and 07.
2. The key decisions preserve the executable and service files during the join itself. The old service and folder still change before success. The second-run decision handles a changed port but misses connection failure at the same address. See findings 01 and 06.
3. The workstream now names the new-boot evidence and removal on the next connection. The boot clock and preservation claims still fail in findings 01 and 07.
4. The rollout assigns the command to Server seats and moves deployed proof after integration. The Compose selector is still missing from the gate invocation. See finding 04.
5. Nothing found in adding the error rule and expanding workstream references. I checked the cited error rule against the foundation design.
6. The public command, service, archive, Store and vault names mostly resolve. The lock's scope, the state-folder writer and reader boundary, and the removed address input need the corrections in findings 02, 03 and 08.
7. The status table correctly separates the client's exit statuses from service handling and narrows removal through the next boot or login. Its readiness and rollback claims remain open in findings 01 and 06. The neighboring receipt test is named `TestLinkRestartReplaysAfterDroppedResult`.
8. Item 1 fixes the three interface errors from G1 and has a local acceptance command. The address prose remains stale, and the direct tests call the helper without asserting the dispatcher entry. See finding 08 and the post-build check for the installed command.
9. The workstream criteria move live runs to the sprint. The precondition, rerun and removal predicates still leave the cases in findings 01, 06 and 10.
10. The empty-port stand-in now distinguishes empty from unset. The state setting and unfinished-key refusal are correctly wired for ordinary first-install failures. Locking, Compose, rejoin, process readiness and the rename observation still leave findings 01 through 04, 06 and 09. The removal preflight leaves finding 10.
11. The macOS branch checks for a graphical domain, reads the installed agent's state setting and moves its device test to the sprint. It inherits the lock and rollback gaps. Its removal still checks the build tools.
12. The live-test record improves machine selection, and the status-75 journal observation improves restart evidence. The preflight and reboot checks still leave findings 05 and 07.
13. The risk table and verification separate local and deployed tests and record the positional token exposure. The mitigation for failed rejoin still omits the service switch. See finding 01.
14. The sprint gate, post-build checks and exclusions make ownership more visible. The stack selector, the scope of the live fixture, and both cases in PB1 need correction. See findings 01, 04 and 05.
15. The traceability rows point to the right general test groups. The renamed receipt test and unresolved gate input still need to agree with their writers. The traceability of the real browser observation is owned by the Server plan's criterion 16.
16. The Server seats resolve the folder assignment. The sizing line's claim that item 1 needs no Docker holds when a real PostgreSQL endpoint is supplied. The documented `deploy/test/with-postgres.sh` route uses Docker. The live fixtures need the selected stack and a reboot-capable host.

The decision log changes agree with the gate split and Server ownership. Rows 40 through 44 carry the rejoin, state and reboot gaps reported above. Row 39's statement that the token input request is dropped should point to row 45, which records that decision as pending with the owner. `.plans/plans/go-s3-stack-client/decisions.md:48`, `.plans/plans/go-s3-stack-client/decisions.md:54`.

### Unreviewed mechanisms

2. The directory lock is portable for a single path, but it is placed under a configurable parent and has no crash recovery. Both lock calls succeeded in a disposable probe. The archive reservation prevents the same-second name collision, but the entry allowlist accepts unrelated contents and the move precedes a durable switch. The installer owns the lock and archive, while the client owns the state validation and runtime lock. Findings 01 through 03 give the required boundaries.

### Rejected findings

3. G9's scope evidence is real. D19 calls one user a convenience and the fold instruction settles that scope, so the two-account case stays out of wave 1. G12's ownership evidence is also real. The committed page still emits the old command, the Client plan requires a real paste, and the Server plan's criterion 16 waits for the authorized page change. The rejection is sound as a placement decision. The browser gate remains open until that change lands. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:44`, `.foundry/go-rewrite/tasks/fold-go-s3-stack-client-r1.md:54`, `.plans/epics/go-rewrite/epic.md:121`, `src/frontend/src/screens/service.tsx:410`, `.plans/plans/go-s3-client-link/plan.md:74`, `.plans/plans/go-s3-server-machines/plan.md:540`.

### Writers and readers of changed names

4. Server sign-in writes configuration, vault identity, dispatcher and test environment. The token command and its tests read them. Server Store writes `Open`, `ConnectionPolicy` and `Close`. The token command reads them. Server Machines writes the issuer, token methods and signed-in fixture methods. Item 1 and its tests read them. Item 1 writes the command, then the installer, gate G1 and an operator read its token output.

   Client Local writes the state configuration and base paths. Client Link commits `identity.json`, writes `address` and `machineId`, reads them on run, and owns exit codes 0, 1 and 75 and the fixed diagnostics. The installer, service managers, guide, stand-ins and sprint checks consume those names. Stack creates and reads its unit, agent, archive and installer lock. The user-flow test writer is an additional reader of the state names, Compose selection and server and client diagnostics.

   Stack Web writes the service names, port, image and launcher settings contract. The installer, live driver, user-flow driver, image rebuild and token smoke check consume it. The fixture recorder writes the boot and machine record, and both later live-test modes consume it. Server Machines projects `hostsServer` and `connectedAt` into `WatchClients`, and the live driver reads them. The real Machines page must read `CreateClientTokenResponse.address` when it builds the pasted command. The committed page currently reads the displayed server label. Its assigned change and the live fixture's ownership verification remain gate dependencies. `.plans/plans/go-s3-stack-client/plan.md:76`, `.plans/plans/go-s3-stack-client/plan.md:252`, `.plans/plans/go-s3-stack-client/plan.md:329`, `.plans/plans/go-s3-server-machines/plan.md:554`, `src/contracts/proto/lgtm/v1/clients.proto:113`, `src/frontend/src/screens/service.tsx:410`.

### Interacting rules

5. The patched logic follows this table. The failing cases occur within the public installer inputs.

| Starting state and operation | Result specified by the patch | Required result |
|---|---|---|
| No installed identity or key, plain install | Join, then install the service. | Join once and start using the same folder. |
| Key without identity, plain install | Refuse before minting another token. | Preserve the pending key and name the recovery steps. |
| Identity at a different address, plain rerun | Refuse before replacing the program. | Preserve the existing installation. |
| Same address with a different peer key, plain rerun | Accept while the new process keeps retrying. | Report the failed peer check and preserve recoverable state. |
| Installed unit and a different explicit state override | Use the override and lock its parent. An empty path enrolls again even on a plain run. | Serialize work on the shared service and make any replacement deliberate. |
| Existing service, rejoin token fetch fails | Leave the service and files in place. | Preserve the old service. |
| Existing service, rejoin join fails after token fetch | Stop the old service while joining, then restore and start it. | Keep the old service available and preserve its prior run state. |
| Rejoin succeeds, service switch fails | Return failure with the old state archived and some new files installed. | Restore a coherent installation or resume a recorded switch. |
| Client exits 0 following removal | Stay stopped in the current session. An enabled service can run at boot or login. | Keep the narrowed promise and prove a clean exit. |

### Round 1 blockers and majors

6. The empty-port blocker is fixed. The G1 Store and vault interface blocker is fixed, with the address call text still stale. G4 and G5 remain major for rollback and folder ownership. G7 fixes changed ports, but a failed same-address connection can still report success. G10 and G11 remain major for the fixture, reboot evidence and Compose selection. G8 and G20 stay with the owner. G13 stays with the link test owner.

### Architect questions

7. The patch touches all eight architect questions. The affected interfaces, placement, design rules, neighboring plans, merge gates, wave boundary and tool versions follow below.

## Answers to the architect lens

1. Confirmed live on commit `6c64cde3`, the plan and decisions match `90d8d4bb`, Docker Compose reports 5.5.0, Go reports 1.26.0, and systemd reports 257. The proposed shim expansion gives 8787 for unset and an empty string for empty. The current client build printed its injected version. By reading, the state names resolve, C5's PostgreSQL helper is in place, and the Server command and test fixtures remain planned. The current anchors for the callers are `src/client/cmd/lgtm-client/main.go:10`, `src/client/internal/state/folder.go:15` and `deploy/test/checks.sh:324`.
2. The changed names are traced in the writers and readers answer above. The missing consumers are the shared Compose entry points and a client-owned readiness outcome. The installer and the client also need one explicit contract for validating and replacing an installed state folder.
3. The installer and platform service files belong in `deploy/`. The dispatcher, token command and tests belong to Server seats under `src/server/`. The contract continues to own wire messages. The orchestrator's test writer owns the shared driver's integration changes, and the orchestrator owns assigning the website change. `.foundry/go-rewrite/tasks/fold-go-s3-stack-client-r1.md:56`.
4. R1 is kept by leaving the database and contract in their owner packages. R2 is kept by consuming the reviewed contract. R3 is kept by calling the machines interface. R4 is kept by using the machines transaction for token creation. R5 is kept by reading the existing watch call. R6 is kept by testing restart through `ReconnectClient`. R7 is kept because the command persists through Store and retains no server state. R8 is kept at the Connect boundary, and the fixed shell diagnostics are separately testable. R9 is kept for the installer and token command's prescribed logs, with the positional-token decision still pending. R10 is kept by using real PostgreSQL and leaving the in-process link tests with the combined suite. R11 does not apply because no third-party logic is copied. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:358`, `.plans/plans/go-s3-server-machines/plan.md:259`, `.plans/plans/go-s3-client-link/plan.md:275`.
5. The epic's wire contract, Store interface, vault credential, client state and test entry points each have producers and consumers. The live mismatches are Compose selection, the obsolete address input, the receipt test spelling and the missing state and readiness handshake. The browser change has a separate gate in the Server plan. `.plans/epics/go-rewrite/epic.md:175`, `.plans/plans/go-s3-server-machines/plan.md:158`, `.plans/plans/go-s3-client-link/plan.md:300`.
6. Item 1 can merge after the dispatcher, fixture and Machines dependencies, using real PostgreSQL. Items 2 and 3 can merge with stand-ins once the rename observation is fixed. Item 4 can merge with its dry-run gate. The joint stack gate stays red until the dependency code, image, state setting and Compose driver agree. The epic allows that integration gap. `.plans/epics/go-rewrite/epic.md:75`, `.plans/plans/go-s3-stack-client/plan.md:146`, `.plans/plans/go-s3-stack-client/plan.md:346`.
7. The patch adds no agent execution or new wire mechanism. Rejoin supplies a recovery path for the removed machine, and removal manages a user service. Restoring older data, running two live copies and hosting a public address remain outside this slice. `.plans/plans/go-s3-stack-client/plan.md:383`, `.plans/plans/go-s3-stack-client/decisions.md:42`.
8. The plan adds no Go module or image. It relies on the existing Go 1.26 toolchain and the Stack Web Compose baseline. The local Compose probe and version checks ran. The macOS agent's native tool behavior and the rootful Linux boot path still need the sprint's platform runs. `go.mod:3`, `.plans/plans/go-s2-stack-web/plan.md:270`, `.plans/plans/go-s3-stack-client/plan.md:372`.

## Post-build checks

| Check | Scenario and expected evidence | Responsible area |
|---|---|---|
| A1 | Run the rebuilt image's token command with and without the mark, through the real dispatcher. The command writes one token line, a client redeems it, and the waiting records carry the correct marks. Capture the status, redacted output shape and watch observations. | Server token command and Stack sprint gate. |
| A2 | On both supported service managers, start a waiting client, approve it, request a restart and remove it. Observe one new process for exit 75, a verified connection from the same machine, then exit 0 without a same-session restart. Capture the service and watch records. | Stack installer and Client Link. |
| A3 | Boot a disposable Linux host with the selected stack and a lingering test user. Observe the first client invocation before interactive login, automatic startup of the selected server and web containers, and the original machine online. Capture boot-scoped service and container records. | Stack sprint gate. |

## Grade

5 out of 10. There are 0 blockers, 7 majors and 3 minors.
