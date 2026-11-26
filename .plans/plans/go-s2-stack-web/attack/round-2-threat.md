REVISE

Deleting the password from the settings file can leave the old password and sessions active because Docker Compose can still take the password from the shell or another settings file.

## Findings

### threat-r2 1 — MAJOR — C4 — The removal command can leave the old sessions active

This is an access and failure-handling defect. The Stack and Server leads own the transition. Compose precedence is confirmed live with version 5.5.0. The failure paths are established by reading the plans. See `.plans/plans/go-s2-stack-web/plan.md:217`, `.plans/plans/go-s2-stack-web/plan.md:300`, `.plans/plans/go-s2-stack-web/plan.md:306`, `.plans/plans/go-s2-stack-web/plan.md:339`, `.plans/plans/go-s2-server-signin/plan.md:476` and `.plans/plans/go-s2-server-signin/plan.md:622`.

An owner signs in with the same password exported in the shell and written to the file, as item 6 itself prescribes. The owner deletes the file entry and runs the ordinary launcher. Compose still resolves that password, so it has no reason to recreate the server for the removal. The same result occurs when the password remains in an earlier file in `COMPOSE_ENV_FILES`. There is also an early failure path. With a genuinely absent password and a running server, a failed vault check or image build exits before the server phase can take the old process out of service. The earlier listener can remain open. The current test clears only one file in a fixture whose helper already cleared the shell.

Give removal an explicit launcher operation against the selected project and existing database, as the Server recovery check expects, or specify equivalent fail-closed behavior in ordinary startup. Under the state lock, stop the serving process, explicitly omit the password from the reconciliation process regardless of settings precedence, and run the approved Server entry point against retained state. Report success only for a purge committed by that invocation. Report skipped or unconfirmed reconciliation and keep serving stopped when a prerequisite fails. Do not instruct the operator to restore a password until the purge is confirmed. Freeze cases for the shell, multiple settings files, an early phase failure and an unconfirmed database commit.

### threat-r2 2 — MAJOR — R9 — Compose can print a password from a private settings file

This is a confidentiality defect. The Stack lead owns launcher diagnostics, and the check-driver owner owns its credential arguments. Confirmed live with a disposable owner-only settings file and a fabricated marker. See `.plans/plans/go-s2-stack-web/plan.md:300`, `.plans/plans/go-s2-stack-web/plan.md:301`, `.plans/plans/go-s2-stack-web/plan.md:449` and `deploy/test/checks.sh:82`.

When an owner enters a password using an opening quote and omits the closing quote, Compose fails to read the file and includes the value in its unterminated-value diagnostic. Mode `0600` and disabled shell tracing leave that output intact. The first Compose call can fail this way, before either web phase handles its errors. The driver separately puts even a correctly configured password in a `jq` argument. A generated acceptance password limits the impact of that run, while the driver still accepts the password of any disposable stack supplied by the operator.

Give every launcher Compose call a fixed diagnostic for unreadable or malformed settings, including calls in the inherited phases. Keep raw errors that can include settings out of terminal output and saved logs. Add a malformed-settings case to the secret-output check and assert that it names the file or setting without the value. Land the requested standard-input change with the driver owner and run the process-argument check with fixture values.

### threat-r2 3 — MAJOR — C1 — The vault-only route still parses the web port

This is a startup defect. The Stack lead owns both phases. Confirmed live with Compose 5.5.0 and a disposable model with the patched port interpolation. Rendering only the database service still failed with `invalid hostPort` for a malformed web port. See `.plans/plans/go-s2-stack-web/plan.md:265`, `.plans/plans/go-s2-stack-web/plan.md:301`, `.plans/plans/go-s2-stack-web/plan.md:314` and `.plans/plans/go-s1-stack-vault/plan.md:233`.

A stale web port setting prevents the operator from using the vault-only launcher while diagnosing a web configuration failure. Compose validates the model before selecting the named service. The current early-stop test omits the password but supplies a valid port through the fixture, so it can pass with the stated isolation broken.

Give the vault phases a Compose model that does not interpolate the web port, or explicitly give those invocations a safe fixed port value without letting it affect a full web run. Keep the project, state root and vault credential identity the same through both paths. Extend the early-stop test with a malformed web port and require the original vault ready line.

### threat-r2 4 — MAJOR — C1 — A port that Compose accepts can leave the printed address unusable

This is a settings contract defect. The Stack and Server leads own the published port and origin. Confirmed live with Compose 5.5.0 and the patched mapping. See `.plans/plans/go-s2-stack-web/decisions.md:39`, `.plans/plans/go-s2-stack-web/plan.md:253`, `.plans/plans/go-s2-stack-web/plan.md:265` and `deploy/test/checks.sh:32`.

Compose accepts a range such as `18787-18788` and preserves it in the web publication model. The same value forms a nonnumeric port in the Server origin, which must pass its origin parser before it can serve. Compose also accepts zero, while the guide and the rollout driver exclude it. Docker can choose a real port for zero and the launcher can print that port, while Server's configured origin still contains zero. Compose's successful parse proves a publication specification is syntactically valid, which is a wider contract than this stack uses. See `.plans/plans/go-s2-stack-web/scouting.md:225`, `.plans/plans/go-s2-stack-web/plan.md:305`, `.plans/plans/go-s2-stack-web/plan.md:337` and `.plans/plans/go-s2-server-signin/plan.md:244`.

On the full startup route, validate the effective value as one decimal port from 1 through 65535 and use the same normalized value for both Server and the publication. Read only the port from a safely handled resolved model if Compose remains the settings reader. Keep the occupied-port check at actual publication. Freeze checks for an omitted value, a nondefault port, zero and a range.

### threat-r2 5 — MAJOR — C1 — The loopback boundary depends on an unspecified Docker Engine version

This is a network isolation gap. The Stack lead owns the runtime requirements. Established by reading the plan, its recorded test platform and Docker's port-publication documentation. See `.plans/plans/go-s2-stack-web/decisions.md:37`, `.plans/plans/go-s2-stack-web/plan.md:152`, `.plans/plans/go-s2-stack-web/plan.md:265` and `.plans/plans/go-s2-stack-web/scouting.md:5`. [Docker port publication](https://docs.docker.com/engine/network/port-publishing/#publishing-ports)

On a Docker Engine release earlier than 28.0.0, a peer on the same network segment can reach ports published to loopback. Caddy's allowed host names filter application requests after a connection reaches it. They do not establish the sender's network location. The plan tests on Engine 29.7.2 and asserts an address in rendered Compose, while the operator guide has no Engine floor.

Specify Engine 28.0.0 or later with the expected bridge isolation and verify that boundary on the supported host configuration before relying on plain Hypertext Transfer Protocol (HTTP). Keep an encrypted, loopback-bound forward as the documented remote access path. Describe host-local processes as within the network reach of the listener, with browser and session checks enforced at Server.

### threat-r2 6 — MAJOR — C8 — The link timeout is extended on every request based on an incorrect Caddy limitation

This is an availability gap. The Stack lead owns the listener and the stream tests. Confirmed by reading the pinned Caddy source and release history. See `.plans/plans/go-s2-stack-web/plan.md:152`, `.plans/plans/go-s2-stack-web/decisions.md:42` and `.plans/plans/go-s2-stack-web/scouting.md:200`. [Caddy timeout handler](https://caddyserver.com/docs/caddyfile/directives/timeouts) [Caddy 2.11.8 release](https://github.com/caddyserver/caddy/releases/tag/v2.11.8) [Caddy 2.11.9 directive order](https://github.com/caddyserver/caddy/blob/v2.11.9/caddyconfig/httpcaddyfile/directives.go#L54-L60)

An ordinary browser request that stalls before finishing its body can hold proxy and upstream resources for up to ten minutes of inactivity. Sign-in is reachable before a session exists, and Server's request size limit does not bound a stalled upload. A different process on the host can reach the same listener. The plan extends every route because its decision record says a route override needs a separate listener. The pinned release supports a request-matched timeout handler and inherits the precedence fix from 2.11.8.

Keep Caddy 2.11.9 and retain the normal body timeout for ordinary requests. Apply the ten-minute allowance only to the canonical link `Open` procedure, before its reverse proxy runs. Preserve the Server handshake and heartbeat deadlines. Re-run all three silence shapes and add a check that an ordinary request still gets the shorter idle timeout. This changes the request deadline without adding sign-in rate limits.

### threat-r2 7 — MINOR — C3 — The browser check permits a false sign-in pass

This is an acceptance gap. Stack owns the browser check, and Server owns the session and optional snapshot. Established by reading `.plans/plans/go-s2-stack-web/plan.md:227`, `.plans/plans/go-s2-stack-web/plan.md:322`, `src/frontend/src/app.tsx:630`, `src/frontend/src/server/sync.ts:78` and `.plans/plans/go-s2-server-signin/plan.md:318`.

The right password can make the form disappear on the first page, then a reload can show the unreachable screen if `GetServer` fails while reading the session. Both observations meet the negative wording about the form. Separately, if request S9 lands, the revocation test's authenticated `WatchClients` response becomes a snapshot rather than `unimplemented`. The test would then fail even with sound revocation.

Observe that the browser's `GetServer` response reports `signed_in=true` after reload and that a separate anonymous visit reports false. Keep the Home observation for the first snapshot. For removal, use the same stable `GetServer` assertion with the saved cookie before and after, and an actual protected procedure to prove refusal of the old cookie. Keep the sign-out mutation out of the pre-removal assertion so it cannot invalidate the cookie under test.

### threat-r2 8 — MINOR — plan — Fixture cleanup grants root write access outside its fixture

This is a test isolation gap. The Stack lead owns the fixture helper. Established by reading `.plans/plans/go-s2-stack-web/plan.md:268` and `.plans/plans/go-s2-stack-web/plan.md:270`.

A fixture made directly under the default temporary directory puts that directory itself inside the root teardown container's writable bind. A bug in selection or cleanup can remove a sibling fixture or unrelated temporary files. The Compose project-name guard protects a different resource and cannot constrain the bind.

Give each test a private outer directory and a dedicated child state directory. Bind only that outer directory for privileged cleanup, and validate that both paths belong to the fixture before deleting anything. Run the stat helper with a read-only bind. Check that two fixtures can overlap and that cleanup of either leaves a sibling sentinel and its services intact.

### threat-r2 9 — MINOR — C9 — The downward stream test can pass before stream completion

This is a test coverage defect. The Stack lead owns the stand-in and the assertion. Established by reading `.plans/plans/go-s2-stack-web/plan.md:380`, `.plans/plans/go-s2-stack-web/plan.md:401`, `.plans/plans/go-s2-stack-web/plan.md:405` and `.plans/plans/go-s2-stack-web/plan.md:418`.

A downward-silence run can read the expected count and then suffer a proxy reset. The stand-in's downward shape does not send a trailer, and its client is only required to observe the count. The run can therefore pass without the clean closure and trailer required by criterion 3a. This also weakens the downward part of the soak.

Give the downward shape a completion trailer and require the client to read clean end of stream and that trailer after the count, in both the quick test and the soak. Make a missing trailer or a reset after the count turn the test red.

### Risk scores

The scores multiply impact on the affected person, fallout, and probability. Evidence status and responsible leads appear with each finding.

| Finding | Impact | Fallout | Probability | Score |
|---|---|---|---|---|
| 1 | 6. The expected revocation fails. | 3. The owner can take the listener down and complete removal. | 6. Shell-backed launch and subsequent removal are supported operator actions. | 108, schedule. |
| 2 | 6. A credential can reach a diagnostic. | 7. Disclosure cannot be undone after the output has been seen. | 4. It requires a malformed entry or a driver invocation visible to another user. | 168, before release. |
| 3 | 6. The recovery command fails. | 3. The owner can correct the port. | 4. A stale malformed port is an occasional deployment fault. | 72, schedule. |
| 4 | 6. The website cannot sign in. | 3. The owner can select a supported port. | 4. The accepted but unusable setting is an occasional configuration fault. | 72, schedule. |
| 5 | 6. An unintended peer can reach the service. | 3. The owner can restore isolation with an Engine upgrade. | 4. It requires an affected Engine and a peer on the same segment. | 72, schedule. |
| 6 | 4. A stalled request holds resources longer. | 3. The timeout or request closure releases them. | 4. An ordinary upload can stall. | 48, record. |
| 7 | 1. The observation can miss a failure during testing. | 1. The scoring applies to the acceptance gap. | 2. It requires a failed reload or the alternate test setup. | 2, record. |
| 8 | 8. A faulty cleanup can delete unrelated data. | 7. Affected temporary files can be unrecoverable. | 2. It requires a cleanup fault in a test environment. | 112, schedule. |
| 9 | 1. The assertion can miss an incomplete stream. | 1. The scoring applies to the test gap. | 2. It requires a reset after the final body message. | 2, record. |

## Answers to the task questions

1. **Each patch in the register.** The table records the result for all 16 groups. Relevant interactions follow the table.

| Group | Result by reading, except where a finding records a live check |
|---|---|
| G1 | The eleven server keys, named database host, port and private-stack opt-in match the Server and Vault plans. The original clean-start mismatch is fixed. The effective origin still depends on the port contract in finding 4. |
| G2 | Web now owns the fixture and early-stop option, the reset is removed, and C5, post-build check (PB) 4 and PB8 have owners. Vault-only Compose still parses the web port. The root cleanup bind widens the fixture's reach. See findings 3 and 8. |
| G3 | The fixed IPv4 publication and Server's same-port browser aliases fix the old bind and origin mismatch for a valid port. Engine requirements and invalid accepted port values still matter. See findings 4 and 5. |
| G4 | The eight-minute health startup period and 540-second launcher wait cover the Server plan's first-start budgets. Driver-specific limits and the inherited health command remain runtime checks. |
| G5 | Docker makes the occupied-port decision at publication, so the former preflight race is removed. Compose's accepted port grammar is broader than the shared origin contract. See finding 4. |
| G6 | Recreation with an effectively unset password can reach Server reconciliation. Source precedence and failure before that phase leave the original revocation failure open. See finding 1. |
| G7 | Image and test ownership have a usable order. Work item 7 can write its tests early and runs rules-only after item 2. Item 1 builds after Server item 9. Nothing further found. |
| G8 | Rules-only needs no production build and the full fixture clears `build`. The upward shape now checks its acknowledgement and trailer. The downward shape misses completion and a trailer. See finding 9. |
| G9 | The wave-one public certificate lifecycle disappears with the public mode. The later proxy's renewal and key policy stays deferred under decision 27. |
| G10 | The browser no longer receives an unverified public certificate from this stack. Remote access depends on the chosen protected forward and a correctly isolated publication. See finding 5. |
| G11 | The owner choice remains open. The signed challenge is the stated default. Sprint 3's loopback tokens omit the certificate pin. Issue 1633 remains open for a future encrypted deployment. |
| G12 | Owner-only creation fixes the original copy under a permissive mask. A malformed value can still reach a diagnostic, and the driver's argument exposure remains with its owner. See finding 2. |
| G13 | The pin removes the specific affected network dependency. The global timeout still broadens the listener even though the pinned Caddy supports a route match. See finding 6. |
| G14 | Home is deferred consistently with the absent Clients handler. Form disappearance and continued absence do not independently prove a live browser session. See finding 7. |
| G15 | The limit-sized replacement has a named owner and PB5. The current driver still appends a suffix at `deploy/test/checks.sh:415`. |
| G16 | The unreachable-screen observation, context comparison and named egress target close the three acceptance ambiguities. The egress test proves no route out. Host ingress is a different claim covered in finding 5. |

2. **Mechanisms marked unreviewed.** Decision 27's loopback-only publication and forward are addressed in finding 5. Decision 29's settings gate and Compose and Docker validation are addressed in findings 2 through 4. Decision 31 and item 5's removal mechanism are addressed in finding 1. Decision 32's listener-wide idle deadline is addressed in finding 6. Decision 34 and criterion 2d's browser observation are addressed in finding 7.
3. **Rejected findings.** Nothing found. The register has zero rejected entries and the fold's outcome counts agree. See `.plans/plans/go-s2-stack-web/attack/round-1-findings.md:15` and `.foundry/go-rewrite/results/fold-go-s2-stack-web-r1.md:23`.
4. **Writers and readers of changed names.** Stack item 4 writes the eleven settings, and Server items 1, 5 and 9 read them. Vault item 2 publishes the database secret, Vault's setup publishes `server/openbao.json`, and Server reads both mounts. Stack item 5 writes the early-stop flag and added phases, and the vault suite and Web tests read them. The Sprint 1 PB8 name `up_until_vault_stops_early` still needs to point to Web's `up_until_vault_needs_no_web_settings`. Stack item 4 writes the fixture API consumed by items 4, 5 and 7. Server item 9 owns `health` and the revocation diagnostics. Stack and the driver consume them, with Server PB3 still expecting an explicit maintenance invocation. Caddy and the Sprint 3 tests share the route and proxy names. Fold request M1 covers the two link plans but should also name the Sprint 3 Stack token command and installer as readers of the loopback-only policy. Its original hash-file request is withdrawn. See `.plans/plans/go-s2-stack-web/plan.md:263`, `.plans/plans/go-s2-stack-web/plan.md:291`, `.plans/plans/go-s2-stack-web/plan.md:326`, `.plans/plans/go-s1-stack-vault/plan.md:260` and `.foundry/go-rewrite/results/fold-go-s2-stack-web-r1.md:63`.
5. **Interactions between patches and retained requirements.** The table below traces settings and phase order together. The alternate snapshot also changes the revocation probe's expected response, as described in finding 7.

| Effective input and starting state | What the patched plan produces | What the plan promises |
|---|---|---|
| Full run, one supported free port, usable password. | Server reconciles and listens, then Web publishes. | Ready, with matching browser origin. |
| Full run, supported occupied port, usable password. | Server starts. Web publication fails and the launcher names the port setting. | A port diagnostic and the other services remain available. |
| Full run, zero or a port range. | Compose accepts the model, but Server does not receive the actual published origin. | The guide excludes zero. Server and the driver require one usable port. |
| Vault-only run, malformed web port. | Compose rejects the model before the vault phases can finish. | Vault ready regardless of web settings. |
| Live stack, file entry removed, shell or earlier file still supplies the same value. | Compose keeps the effective password. Existing authenticated sessions can remain active. | Password removal revokes sessions. |
| Live stack, effective password absent, earlier phases succeed, purge commits. | Server exits after removing sessions. Reusing the old password later cannot revive its cookies. | Revocation is reported and the server is stopped. |
| Live stack, effective password absent, earlier phase fails. | The launcher exits before replacing or stopping the old server. | The removal path takes the old listener down. |
| Live stack, effective password absent, server reaches reconciliation, purge fails. | The replacement exits without listening and reports unconfirmed revocation. Old rows can survive. | Service stays stopped and the operator retries before restoring a password. |

6. **Earlier blockers and majors still open.** G2's vault-only guarantee, G5's consistent port behavior and G6's revocation guarantee remain open. G12 closes the file-mode case and leaves a credential handling defect. G13 closes the original advisory and introduces a wider timeout. G1's eleven-key blocker is closed. G9's public certificate lifecycle and G10's trust blocker were removed with public mode.
7. **Lens coverage.** The patches touch all seven lens questions. The answers below cover the affected trust boundaries, credentials, lifecycle, transport, dependencies, integrity and runtime checks.

## Answers to the threat lens

1. **Trust boundaries.** The published socket admits clients on the host, subject to Engine isolation. Caddy checks the request host and routes the two prefixes. Server checks browser origin and session status independently of forwarded scheme and host. The machine link remains a separate family and gains proof and approval in Sprint 3. Compose reads host settings and accepts a broader port grammar. The Server readers validate the mounted database password file and OpenBao credential. See findings 2 through 6 and `.plans/plans/go-s2-server-signin/plan.md:275`.
2. **Credential inventory.** The owner supplies the password in a private file or shell. Compose passes it in the Server environment, and the browser sends it in a sign-in body through Caddy. Server mints a random cookie and PostgreSQL keeps its verifier, salt, creation time and password binding. The Vault plan creates the database application password, the limited OpenBao credential, the unseal file and the administrator response. Only the first two go to Server as read-only mounts. The Server client holds its scoped vault token in memory and keeps the link signing seed in OpenBao. C13 logs the public fingerprint. Wave 1 adds no proxy private key, certificate or pin file. Findings 1 and 2 cover end-of-life and leaks. See `.plans/plans/go-s2-stack-web/plan.md:247`, `.plans/plans/go-s2-stack-web/plan.md:264`, `.plans/plans/go-s1-stack-vault/plan.md:224` and `.plans/plans/go-s2-server-signin/plan.md:433`.
3. **Credential lifecycle.** Server revokes a cookie on committed sign-out, expiry, capacity eviction or committed password reconciliation. A restart with the same password retains healthy sessions. Removing the password must end every session through the launcher, which finding 1 shows is conditional. OpenBao's short-lived batch token expires in 15 minutes and Server obtains a fresh one near expiry. Vault owns maintenance of the underlying credential and the seal file. The signing key persists with the installation. Test fixture credentials and cookie jars should be removed with their projects and scratch data. See `docs/specs/2026-11-25-go-rewrite-foundation-design.md:202`, `.plans/plans/go-s2-server-signin/plan.md:435` and `.plans/plans/go-s2-stack-web/plan.md:270`.
4. **Encryption.** The design permits HTTP on loopback and requires encrypted HTTP (HTTPS) elsewhere. The new plan uses HTTP on host loopback and cleartext HTTP version 2 (h2c) on the private application hop. For access from a second host, the guide should require a protected, loopback-bound forward, as in finding 5. OpenBao encrypts persisted vault secrets. Session tokens are stored as verifiers, and host bootstrap files and settings rely on filesystem isolation. The later proxy's certificate fingerprint and the disposition of machine certificates from issue 1633 still need an owner decision for encrypted deployments. See `docs/specs/2026-11-25-go-rewrite-foundation-design.md:223`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:316` and `docs/issues.md:7520`.
5. **Dependencies.** The Go, Node, distroless, Caddy and inherited stack images have digest pins. The plan trusts their publishers, the version-pinned package manager from the package registry and the frozen workspace lockfile. The certificate tool is removed. The recorded Caddy dependency `golang.org/x/net` 0.60.0 is outside the specific older affected range. Runtime identification and rebuilt-image checks remain post-build evidence. See `.plans/plans/go-s2-stack-web/decisions.md:18` and `.plans/plans/go-s2-stack-web/scouting.md:73`. [Go vulnerability report](https://pkg.go.dev/vuln/GO-2026-6617)
6. **Integrity.** A pending machine has no browser-session authority and depends on Sprint 3's authenticated link restrictions. An unlisted browser origin is rejected even when its request has an allowed host name, and a forwarded scheme does not set cookie mode. A host process can reach loopback and ordinary Docker bridge networks do not automatically exclude host processes. The plan relies on application checks there. The proposed global timeout and settings failures give the local trust boundary more work. See findings 1, 2, 5 and 6, and `.plans/plans/go-s2-server-signin/plan.md:246`. [Docker bridge gateway modes](https://docs.docker.com/engine/network/port-publishing/#gateway-modes)
7. **Evidence that requires the build.** The checks below need the merged launcher, browser, Server and web image. Compose configuration and the settings parser were checked live with disposable inputs. Docker daemon access was denied, so no container, browser, vault or stream runtime was run in this review.

## Post-build checks

| Scenario | Expected result and owner |
|---|---|
| On a disposable live project, remove the password with a shell value, then with multiple settings files. In separate attempts fail an early phase and fail the reconciliation commit. | Stack and Server capture the current invocation's result. An explicit removal takes the old listener out of service. An unsuccessful purge clearly says it is unconfirmed. After a confirmed purge and restoration of the original password, the original cookie is refused. |
| Supply an owner-only malformed settings file with a marker. Sign in with a separate valid fixture credential. Capture the launcher, driver, process arguments and service logs. | Stack and the driver owner see the fixed failure message and no credential in output or process arguments. Confirm only the intended privileged Docker operators can read runtime configuration. |
| On the minimum supported Engine, verify reachability from the local browser and from a second machine on the same network segment. Try an unlisted browser origin and the documented encrypted forward. | Stack sees local and securely forwarded sign-in succeed, direct remote access refused and the unlisted origin refused before sign-in dispatch. Check the actual published address and note the Engine version. |
| Complete a browser sign-in at a nondefault port, reload and test a separate profile without a session. | Stack and Server see an owner-only session cookie with the expected attributes and a signed-in `GetServer` response after reload. The separate profile remains signed out. |
| Exercise an ordinary stalled request alongside all three silence shapes on the merged web image. Kill each peer in separate runs, including before the first response. | Stack sees the ordinary deadline take effect without delaying a concurrent normal sign-in. The exact link route passes the configured silence windows and closing trailers. A disconnected peer releases the corresponding request within the declared bound. |
| Run two fixture projects on distinct ports with a shared checkout, then clean up one. Run the frozen driver separately with fixture-only credentials. | Stack observes the other project's services and state remain intact. The rollout driver passes C1 through C4 and C13 on the supported Engine. Capture first-start timing and compare the public vault-backed signing-key fingerprint across recreation. |

## Grade

5 out of 10. There are 0 blockers, 6 majors and 3 minors.
