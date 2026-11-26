REVISE

Server item 9 cannot merge until two rollout checks pass against the Stack image, while Stack item 1 cannot build that image until Server item 9 merges.

## Findings

### architect-r2 1 — BLOCKER, plan. Break the circular merge gate.

**Evidence.** By reading, Stack item 1 requires the merged server command, and Stack items 4 and 5 subsequently wire and start it. Server item 9 makes checks 2 and 13 (C2 and C13) gates on the command. The epic requires passing checks on the exact work-item commit. Confirmed live, running the two checks in this checkout reports the missing Compose file. Both checks need the running Stack services. `.plans/plans/go-s2-stack-web/plan.md:123,242,296`; `.plans/plans/go-s2-server-signin/plan.md:535`; `.plans/epics/go-rewrite/epic.md:69–76`; `deploy/test/checks.sh:369–377,619–647`.

**Failure.** With the vault sprint merged, Server item 9 has no deployed server or website for C2 or C13. Stack cannot merge the image that would supply them. Each lead is required to wait for the other.

**Fix.** Gate Server item 9 on its named package, command, startup and real-database tests. Gate Stack items 1, 4 and 5 on their own checks in that order. Make the deployed C2 and C13 runs joint sprint gates after both plans merge. Apply the same split to the deployed C3 verification named by Server's sign-in workstream.

### architect-r2 2 — MAJOR, plan. Specify how the launcher consumes test overrides.

**Evidence.** By reading, the new launcher uses explicit `-f deploy/compose.yaml` commands. Two tests describe service overrides without passing a second file to those commands. The failed-process test replaces only the command, and the production image has a binary entry point and no shell. Confirmed live with Compose 5.5.0, an explicit base `-f` ignored both a sibling override file and `COMPOSE_FILE`. Passing a second `-f` applied the override and preserved the original entry point. `.plans/plans/go-s2-stack-web/plan.md:127,300–305,317–318`.

**Failure.** Under the stated invocation the healthy production server starts in both fault tests. If a test supplies only a shell command as the additional file, the server executable still receives the shell command as its arguments and exits through its argument dispatcher. That fails to exercise the specified diagnostic and stop path. `.plans/plans/go-s2-server-signin/plan.md:482`.

**Fix.** Name a fixture-owned way to pass a Compose file set through the launcher. Specify which commands share that set, including build, startup, logs and stop. For each substitute, name the pinned image, clear the inherited build and replace the entry point and command together. Assert the rendered service and an output marker unique to the substitute before checking the elapsed time or stop behavior.

### architect-r2 3 — MAJOR, C1. Keep web port validation out of a vault-only launch.

**Evidence.** By reading, the launcher promises that `--until vault` needs no web port, and the port is interpolated in the same Compose file as PostgreSQL and OpenBao. The plan depends on Compose to validate it. Confirmed live, Compose 5.5.0 rejected a malformed web port even when asked to render only the unrelated server service from the file. It also accepted port zero and left the public origin at port zero, which the scouting notes explicitly acknowledge. `.plans/plans/go-s2-stack-web/plan.md:249–265,300–314,337`; `.plans/plans/go-s2-stack-web/decisions.md:39`; `.plans/plans/go-s2-stack-web/scouting.md:225–231`.

**Failure.** An operator leaves a malformed web port in the settings file while doing vault maintenance. The first Compose command fails while loading the website port, before starting the database. With zero on a full launch, the published port and the origin given to Server can disagree.

**Fix.** Load web-specific Compose configuration only on the full path, or render the vault-only commands with an explicit safe port override. Prove the vault-only route succeeds with a malformed web port in both the environment and settings file. Keep Docker's publication-time check for a taken port. Reject zero on the full path with a named diagnostic or specify how the chosen port reaches Server before serving.

### architect-r2 4 — MAJOR, C4. Make the documented password removal reach the revocation step.

**Evidence.** By reading, shell settings win and the removal instructions mention changing only the file. The launcher performs vault validation and image building before it recreates Server. Server deliberately puts revocation before OpenBao access and listens only after a successful usable-password start. Its maintenance check expects the old server to be stopped before an unsuccessful transition is reported. Confirmed live with Compose 5.5.0, deleting the fixture file's assignment retained a password from the shell in the rendered service. `.plans/plans/go-s2-stack-web/plan.md:215–218,300–306,335–340`; `.plans/plans/go-s2-server-signin/plan.md:473–478,486–502,622`; `docs/specs/2026-11-25-go-rewrite-foundation-design.md:202–209`.

**Failure.** After using the documented acceptance sequence, which exports the password, an operator removes it from the file. Compose keeps the same effective setting and the old cookie stays valid. Even with both sources cleared, a rejected vault credential or failed image build can halt the launcher before the installed server stops or the purge runs. Neither case produces the outcome promised by the removal guide.

**Fix.** Document removal from both setting sources. Define the installed-stack removal path against the original database and installed server image. Stop the serving instance before attempting the transition. Give the removal attempt a path that needs database readiness and can proceed independently of the vault and web image. Treat a missing committed-purge line as unconfirmed and keep the server stopped. Cover the inherited shell value and a failed vault precondition in the acceptance cases.

### architect-r2 5 — MAJOR, C3. Align the browser gate with the Server plan.

**Evidence.** By reading, only the first `WatchClients` snapshot sets `server.synced`. The planned server supplies no Clients handler, so it returns `unimplemented`. The website retries and renders a busy main element without actions. The patched Stack criterion can observe the disappearance of the form, but the neighboring Server acceptance explicitly requires sign-out using the bundled address. `src/frontend/src/server/sync.ts:38–63,78–83,101–108`; `src/frontend/src/app.tsx:626–634`; `.plans/plans/go-s2-server-signin/plan.md:275–277,340,478`; `.plans/plans/go-s2-stack-web/plan.md:227,447`.

**Failure.** The right password creates a valid cookie and the form disappears. Reloading restores the same blank busy screen. A browser tester cannot reach the sign-out action required by Server. The missing action follows from the interface choices, so PB2 alone cannot settle the sprint-2 gate.

**Fix.** Choose one verified sprint boundary across the two plans. Either assign Server a guarded empty snapshot response in sprint 2 with an explicit sprint-3 replacement, or jointly require browser evidence of the accepted cookie and a signed-in GetServer after reload in sprint 2, moving the authenticated screen and browser sign-out to the Machines integration gate.

### architect-r2 6 — MAJOR, D17. Assign the encrypted fixtures that lost their Stack supplier.

**Evidence.** By reading, the fold assignment authorizes removing public Stack mode. The patch removes the certificate writer, its mount and external URL setting. The Contract plan still requires an encrypted join through a real proxy in its post-build check 2. Server Sign-in still requires a bundled secure site for its browser cookie check. Server Machines owns a reader of `LGTM_TLS_CERT_FILE` and requires a readable certificate for an encrypted join token. `.foundry/go-rewrite/tasks/fold-go-s2-stack-web-r1.md:53–55`; `.plans/plans/go-s2-stack-web/decisions.md:37,46`; `.plans/plans/go-s0-contract/plan.md:753–758`; `.plans/plans/go-s2-server-signin/plan.md:389,621`; `.plans/plans/go-s3-server-machines/plan.md:1032,1076–1084`.

**Failure.** The integration owner reaches the encrypted cases with only a Compose listener that rejects non-loopback host names and no fixture that supplies a certificate to the token issuer. The checks cannot run as specified. The separate owner choice on machine certificates is still open before the link contract freezes.

**Fix.** Keep production publication on loopback. Amend the neighboring checks together. Where encrypted client and browser support remains in scope, name a disposable proxy and certificate fixture, its owner, the test origins and the handoff of the public certificate to Server Machines. Where an acceptance case depends on a public deployment, date and record its move to the later proxy work. Preserve the owner decision on machine certificates as a separate contract-freeze gate.

### architect-r2 7 — MAJOR, C9. Bind the stream proof to the exact source revision and fixture model.

**Evidence.** By reading, the full run uses a shared local web image tag and calls `up` without an explicit rebuild. The proof's builder can edit the Caddyfile, which the earlier image item copies at build time. The override's only rendered assertion covers `build`, although its server must discard both credential binds. Confirmed live, adding a probe bind at a different target left both base mounts in the Compose model. Tagging the list `!override` produced only the probe mount. `.plans/plans/go-s2-stack-web/plan.md:180,264–269,393–394,406–409,422–424`. [Compose build and image rules](https://docs.docker.com/reference/compose-file/build/#using-build-and-image), [Compose merge rules](https://docs.docker.com/reference/compose-file/merge/#unique-resources).

**Failure.** A runner with yesterday's `lgtm-web:local` can test yesterday's rules after a Caddyfile change. On a fresh fixture whose bootstrap never ran, an ordinary volume override retains two required file binds and Docker cannot start the stand-in even though the build assertion passed.

**Fix.** Build the website image from the candidate revision before the full run and the mutation run. Select the resulting image identifier explicitly and assert the running file matches the candidate Caddyfile. Use a volume-list replacement and assert the exact probe mount, the absent build and dependencies, the probe entry point and the absence of credential settings. Give the early and full checks their own exact-commit gates if the first merges independently.

### architect-r2 8 — MAJOR, C1. Agree on rollout deadlines for the slower permitted paths.

**Evidence.** By reading, the new launcher has a 540-second wait. Server gives instance admission ten seconds, migration five minutes and subsequent prerequisites two minutes. The existing driver still gives C2 twenty seconds and the restart probes sixty seconds. C13 also uses an untimed Compose recreation before entering its shell deadlines. PB1 assumes the database being migrated already guarantees the smaller bounds. `.plans/plans/go-s2-stack-web/plan.md:246,303,446`; `.plans/plans/go-s2-server-signin/plan.md:473–477`; `deploy/test/checks.sh:66–72,377–392,407–419,633–654`.

**Failure.** On a migrated database, a transient lock delays session reconciliation past twenty seconds while Server is still inside its allowed prerequisite budget. The driver reports that the no-password process did not exit, even though the process can still correctly commit revocation and refuse to listen. The successful 150-second launcher fixture cannot detect that mismatch.

**Fix.** Before freezing the release checks, give the migrated-stack operations their own agreed readiness and refusal limits, or make the driver wait within the supported Server budgets. Bound the Compose recreation as well as subsequent polling. Keep PB1 as the end-to-end timing proof.

### architect-r2 9 — MAJOR, C1. State the Docker Engine floor for loopback publication.

**Evidence.** By reading, the plan relies on Docker to give the loopback bind its network meaning. The local host-name check inspects the request authority, and the test checks only the rendered binding and rejected host names. Only the Compose version is named as an assumption. The scouting run used Docker Engine 29.7.2. `.plans/plans/go-s2-stack-web/plan.md:24–27,152,166,265,279,335–340`; `.plans/plans/go-s2-stack-web/decisions.md:39`; `.plans/plans/go-s2-stack-web/scouting.md:3–6`. [Docker port publication](https://docs.docker.com/engine/network/port-publishing/), [Docker health check version requirements](https://docs.docker.com/reference/dockerfile/#healthcheck).

**Failure.** An older daemon can render the expected loopback mapping and still expose it to a neighbor on the same network segment. A loopback host name is accepted by the application and is independent of the actual peer. Engines before 25 also lack the specified health start interval.

**Fix.** Set and verify a supported Linux Docker Engine version of at least 28, with the tested Compose version or a verified minimum. Have the launcher report an unsupported daemon before publishing. Check reachability on the declared platform, alongside the existing host-name test.

### architect-r2 10 — MINOR, C4. Fix the known password-size case before checking its result.

**Evidence.** By reading, Server accepts configured passwords through 4,096 bytes. The checked-in driver appends suffixes to both the C3 wrong-password probe and the C4 replacement. PB5 already identifies the overlong replacement. `.plans/plans/go-s2-server-signin/plan.md:97–106`; `deploy/test/checks.sh:400–421`; `.plans/plans/go-s2-stack-web/plan.md:450`; `.foundry/go-rewrite/results/fold-go-s2-stack-web-r1.md:69–73`.

**Failure.** At the accepted maximum size the driver creates a replacement that Server must refuse. The driver then fails while waiting for the replacement server. The fixture input and its size are already known.

**Fix.** Assign the existing request to the rollout-driver owner before its tests freeze. Use independently generated, different passwords that fit the accepted size. Exercise a valid-length wrong password in C3 and use PB5 to verify the corrected C4 run.

## Answers to the task questions

1. The register is adjudicated below by patch group. Duplicate findings inherit the result for their group.

| Group | Result on the patched text |
|---|---|
| G1 | The original failure is removed. The eleven emitted settings and two file paths agree with Server and Vault, including the private-stack opt-in. |
| G2 | Fixture ownership, reset removal, the C5 query owner and the PB4 and PB8 references are fixed. A malformed web port can still block the vault-only path, as finding 3 shows. |
| G3 | With an ordinary nondefault port, the numeric origin and publication agree, and the Server alias policy covers `localhost` on the same port. Port zero remains outside the supported flow and needs a clear outcome under finding 3. |
| G4 | The launcher's cold-start budget is fixed. The driver's independent bounds still conflict with permitted startup behavior, as finding 8 shows. |
| G5 | Docker now detects a competing bind at publication. The promised port-independent vault route still fails on an invalid port, as finding 3 shows. |
| G6 | Removing the effective value with healthy prerequisites reaches the purge. An exported value or earlier phase failure breaks the documented removal, as finding 4 shows. |
| G7 | The correct Server item supplies the command, and Vault supplies the host files. The supplier's deployed acceptance and the image merge now depend on each other, as finding 1 shows. |
| G8 | The rules-only path, deadlines, post-silence acknowledgement and trailer, and three-shape soak address the original probe gaps. The full fixture and image identity remain open in finding 7. |
| G9 | The certificate job and its renewal path are removed by the fold's recorded instruction. The encrypted checks that formerly assumed a supplier remain open in finding 6. |
| G10 | Wave 1's browser no longer meets a certificate in the default stack. The loopback-only boundary needs a compatible daemon, as finding 9 shows. |
| G11 | The machine certificate is still an explicit owner choice. The loopback token uses an empty certificate pin, and the challenge still has its own signing-key pin. |
| G12 | The mode check and generated acceptance password address the settings-file finding. The existing driver still passes its sign-in password to `jq` in an argument. D3 names its owner, and the disposable-password acceptance bounds the current test exposure. `deploy/test/checks.sh:82–94`. |
| G13 | The new Caddy release and timeout address the old release and the observed minute-long cut. The patch still needs the exact-image proof and the supported loopback boundary in findings 7 and 9. |
| G14 | The revised observation is feasible as a credential test. The alternate path still leaves Server's browser sign-out acceptance impossible, as finding 5 shows. |
| G15 | The current suffix generator deterministically exceeds the server's maximum at the stated input. Finding 10 records the known fix. |
| G16 | The context compares to the repository root, the egress test names a target and deadline, and no production shell is needed. The manual unreachable-server check corresponds to the fixed-address website's error path. `src/frontend/src/server/sync.ts:74–86`. |

2. The loopback-only mechanism leaves the encrypted fixture owners open and depends on the Engine floor in findings 6 and 9. The Compose-only preflight and the ordinary password-removal mechanism fail in findings 3 and 4. The browser-only observation conflicts with Server sign-out in finding 5. For the Caddy listener mechanism, the changed timeout bounds stalls while reading request bodies, including sign-in bodies. The silent-link proof exercises the longer limit, and the empty-body health probe bypasses it. Under the supported loopback boundary I found no independent ownership conflict from using one listener policy. Require the exact-image test in finding 7. [Caddy server timeouts](https://caddyserver.com/docs/caddyfile/options#timeouts).
3. Nothing found. The complete round-1 register and the fold outcome counts contain zero rejected findings. `.plans/plans/go-s2-stack-web/attack/round-1-findings.md:13–60`; `.foundry/go-rewrite/results/fold-go-s2-stack-web-r1.md:21–31`.
4. Item 4 writes the eleven keys for Server item 1, and Stack Vault writes the two credential files that item 4 mounts. The operator or fixture supplies the password and port. Compose reads both, the driver also reads both from the shell, the installer reads the port, and Server Machines puts the derived origin in join tokens. Server item 9 writes the `health` command and revocation diagnostics for the launcher, checks and guide. Item 4 writes the fixture functions for items 4, 5 and 7, and item 5 writes launcher flags and phase lines for the operator and vault tests. The Contract team supplies procedure names, Caddy reads their prefixes, and Server and the browser or link client consume the routes. Server Machines also reads Caddy's forwarded peer address. Item 7 writes probe modes, headers, trailer and flags for its suite and possible Sprint 3 reuse. The missing handoffs are the launcher override consumer, the image identity and the retired certificate fixture suppliers. `.plans/plans/go-s2-stack-web/plan.md:143,171,198,247–291,326,424`; `.plans/plans/go-s3-server-machines/plan.md:1119–1125`; `.plans/plans/go-s3-stack-client/plan.md:91,354–368`.
5. The table below covers the launcher rules that now interact. Findings 1, 5, 6 and 7 cover the separate merge, browser, certificate and image contradictions.

| Launcher request | Effective password | Port and prerequisites | Consequence as written |
|---|---|---|---|
| Vault only | Absent | Valid or omitted port, ready database and vault | The vault path can finish without building either image. |
| Vault only | Any | Malformed port in the full Compose file | Compose rejects the model before the requested service can run. |
| Full | Usable | Valid port and ready prerequisites | Server and web can start. |
| Full | Usable | Valid but occupied port | Server can start, and web publication fails. |
| Full | Usable | Zero port | The model accepts the port, and the configured origin differs from the eventual publication if startup reaches web. |
| Full after file deletion | Still set in the shell | Ready prerequisites | The effective password is unchanged, and the removal does not happen. |
| Full after both sources are cleared | Absent | Ready prerequisites | A recreated Server can commit the purge and exits without listening. |
| Full after both sources are cleared | Absent | Vault or image phase fails first | No server-phase stop, purge attempt or unconfirmed result is assured. |

6. Yes. The operational problems behind G2, G4, G6, G7 and G8 are only partly closed. G14 remains an integration gap after narrowing the Stack observation. Findings 1 and 3–8 name the corresponding failure states. The original G1 settings blocker is closed.
7. All eight architect questions are touched. Their answers follow.

## Answers to the architect lens

1. Confirmed live, the reviewed commit is `8f961b33`, Compose is 5.5.0 and the rollout script is executable. Its C2 and C13 runs give the missing-file result recorded in finding 1. By reading, the actual procedure constants, frontend fixed-address setting, Vite proxy and planned Link Open name resolve. The sprint Dockerfiles, application entry point, Compose services and fixture library are targets. `src/contracts/gen/go/lgtm/v1/lgtmv1connect/session.connect.go:38–46`; `src/frontend/src/server/address.ts:26–35`; `src/frontend/vite.config.ts:73`; `.plans/plans/go-s0-contract/plan.md:589`.
2. The changed writers and readers are listed in task answer 4. The planned changes keep wire field numbers intact and remove the certificate mount producer. Findings 2, 6 and 7 record the missing consumers or suppliers.
3. The patched file placements keep image, proxy, Compose and deployment test code in `deploy/`. Session authorizing and revocation belong to Server, and wire procedure names belong to Contract. The cross-team changes prescribed in findings 5 and 6 need explicit ownership. `.plans/epics/go-rewrite/epic.md:114–136`.
4. R1 does not apply because no product part or migration is added here. R2 is kept because both proxy namespaces route existing or reviewed contract names. R3, R4, R5, R6 and R7 do not apply to Stack storage or command handling, and the proxy adds no substitute writer. R8 is kept for forwarded application replies. R9 gains private settings files, and rollout-driver request D3 remains visible. R10 is kept by using real PostgreSQL for stack acceptance and reserving real client/server link tests for sprint 3. R11 does not apply to these new scripts and rules. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:358–372`; `.plans/plans/go-s2-stack-web/plan.md:151–169,359,432`; `.plans/epics/go-rewrite/epic.md:161–182`.
5. The Vault handoff agrees on runtime user, networks, database and credential names, C5 query ownership and the vault-only flag owner. The Server health command and eleven settings also agree. Server Machines' nondefault-port request is answered, and the Client plan expects ten-second heartbeats through the loopback link. Stack Client still names items 1–3 as sufficient for an override that needs the item 4 Compose services, and proposes a second echo probe even though this patch offers reuse. Update that dependency and decide which probe supplies its smoke checks. Findings 1, 5 and 6 cover the other mismatches. Issue 1634's real in-process entry point remains with Server and the combined link tests. `.plans/plans/go-s1-stack-vault/plan.md:217–227`; `.plans/plans/go-s3-server-machines/plan.md:1119–1126`; `.plans/plans/go-s3-client-link/plan.md:184–214`; `.plans/plans/go-s3-stack-client/plan.md:354–368`.
6. Item 2 can use its stub and item 3 follows item 2. Item 1 follows the Server command once finding 1 is addressed. Item 4 can merge its model after Vault and both images. Item 5 needs the item 4 model and the runnable fault fixtures in finding 2. Item 6 needs the merged launch and the agreed browser evidence. Item 7's rules-only run needs item 2, while its full acceptance needs items 3 and 4 and the exact web image in finding 7. Audit the package and deployment checks on each resulting commit. `.plans/plans/go-s2-stack-web/plan.md:74–78,514–524`.
7. The generic contract prefixes keep room for later calls. The patch removes the certificate job and defers frame routing until the map service exists. The proxy proof is needed for the restart and reconnection checks. External service calls will eventually need explicit egress from the private server. I found no new wave-2 product feature in the patch. `.plans/plans/go-s2-stack-web/decisions.md:37,45`; `docs/specs/2026-11-25-go-rewrite-foundation-design.md:417–431,486–490`.
8. The pins are Go 1.26.11 Alpine, static Debian 12 distroless nonroot, Node 26.13.0 Alpine, pnpm 12.16.1 and Caddy 2.11.9 Alpine. The Go patch meets the root module floor and the Node and pnpm versions match the repository requirements. Caddy's release source pins the Go network module at 0.60.0. Scouting records the four image-index digests and the unprivileged Caddy trial. The local Docker daemon denied access and registry name resolution was unavailable, so I could verify Compose rendering and release sources here, but could not rerun image pulls, health or streaming. Findings 7 and 9 specify the runtime checks that depend on this. `go.mod:3`; `package.json:7–10`; `.plans/plans/go-s2-stack-web/decisions.md:18,42`; `.plans/plans/go-s2-stack-web/scouting.md:68–95,198–221`. [Go release history](https://go.dev/doc/devel/release#go1.26.minor), [Node release](https://nodejs.org/en/blog/release/v26.13.0), [pnpm release](https://github.com/pnpm/pnpm/releases/tag/v12.16.1), [Caddy release dependencies](https://github.com/caddyserver/caddy/blob/v2.11.9/go.mod#L44-L46).

## Post-build checks

- **Launch and removal.** Use a disposable installation with a saved cookie. Exercise a delayed migration, a failed executable, a busy port and a failed revocation against the original database. Record phase order, elapsed times and the committed or unconfirmed result. After a successful purge, restore the original password and require that the saved cookie is refused.
- **Browser.** At a nondefault port, require that the wrong password is refused, the right password sets the Strict cookie and reload remains authenticated in the same profile. Run browser sign-out when the agreed sprint delivers it. If sprint 3 supplies the first snapshot, require Home with the displayed port after the same sign-in.
- **Image and stream.** On the supported daemon, start the pinned images with the declared users and privileges. Compare the running Caddyfile with the merged source and run all three soak shapes. Stop the upstream and kill the downstream in separate runs, including a quiet stream, and time the other peer's cleanup against PB3. Confirm actual cancellation on the pinned binary because Caddy documents a cancellation caveat for negative flush intervals. Repeat with the real link in sprint 3. `.plans/plans/go-s2-stack-web/plan.md:448`. [Caddy streaming behavior](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy#streaming).
- **Rollout driver and fixture output.** Run C1 through C4 and C13 from an empty cache after the integrated merge. Repeat C4 at the password limit. Inspect the disposable sign-in processes and service logs after D3, and require no fixture password in the argument vector, image history or logs.

## Grade

5 out of 10. There is 1 blocker, there are 8 majors, and there is 1 minor.
