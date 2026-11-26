REVISE

An ordinary second install with a different state-folder setting can enroll another machine and replace the working service’s identity, even after the removal of `--rejoin`.

## Findings

The review covers the changes from `90d8d4bb` to `263c34b2`. The plan, decisions and scouting notes match the latter revision at checkout `e4a70245`. D numbers refer to design decisions, R numbers to design rules, C numbers to wave 1 checks, and G numbers in the round 2 register to its finding groups. I use “gate G” for the plan’s sprint gates.

### threat-r3 01 — MAJOR — D19 — A second install can change the service’s identity

**Evidence by reading.** The patch promises a second run preserves identity. Step 6 prefers a nonempty `LGTM_CLIENT_STATE_DIR` to the installed service’s folder. Steps 13 and 15 can then join and install a service against the newly selected folder. The macOS branch has the same precedence. Both update tests cover a matching or absent override. .plans/plans/go-s3-stack-client/plan.md:25, .plans/plans/go-s3-stack-client/plan.md:199, .plans/plans/go-s3-stack-client/plan.md:206, .plans/plans/go-s3-stack-client/plan.md:208, .plans/plans/go-s3-stack-client/plan.md:239, .plans/plans/go-s3-stack-client/plan.md:254.

**Failure.** A user who installed an approved client later exports a fresh state folder for a temporary client and runs the installer to update the service. The run creates a second waiting machine marked as the server host and restarts the fixed service with that identity. The original approved machine goes offline. This is the single-run part of round 2 finding `threat-r2 02`.

**Fix.** When the Linux unit or macOS agent already exists, validate its stored state-folder setting and use it for updates. Refuse a conflicting override before building, minting a token or changing the service. Apply the override to first installs and leave the documented manual removal and reinstall available. Test conflicting overrides with both an empty folder and an already enrolled folder.

### threat-r3 02 — MAJOR — C6 — The sprint still has competing Compose selection rules

**Evidence by reading.** The plan’s new rule uses `-f` by default and honors a nonempty `COMPOSE_FILE`. Gate G1 forces `-f`. Gate G4 requires the round 1 handoff, which explicitly prescribes exporting the default `COMPOSE_FILE` and dropping `-f`. The round 2 handoff corrects that rule but names the direct override as check C4. In the driver, the direct override is in check C2, while C4 already uses the helper. .plans/plans/go-s3-stack-client/plan.md:96, .plans/plans/go-s3-stack-client/plan.md:316, .plans/plans/go-s3-stack-client/plan.md:319, .foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:42, .foundry/go-rewrite/results/fold-go-s3-stack-client-r2.md:46, deploy/test/checks.sh:378, deploy/test/checks.sh:412.

**Failure confirmed live for configuration selection.** On Docker Compose v5.5.0, a disposable two-directory configuration with different settings markers selected the Compose file’s directory value with `-f` and the caller directory value with `COMPOSE_FILE`. A runner using the round 1 prerequisite can therefore check another port or project from the installer even within the same checkout. With an explicit override, the new gate G1 and the driver’s direct C2 operation can bypass the override used by the launcher and the installer.

**Fix.** Put the final selector directly in the sprint gate prerequisite. Apply it to gate G1, the driver helper and the additional override used in check C2. Specify a common working directory or absolute file selectors for the launcher, installer and gates, including a custom `COMPOSE_FILE` list. Verify the selected project and observed bound port in both default and override cases before any test mutates a stack.

### threat-r3 03 — MAJOR — D16 — The reboot proof can omit the first login

**Evidence by reading.** The patched gate compares the recorded machine’s `connectedAt` with every session whose current class is `user`. The installed `loginctl(1)` manual describes `list-sessions` as a list of current sessions. The timestamp on the wire marks the current connection and comes from PostgreSQL wall time at stream admission. The fixture records the previous boot identifier and the machine identifier. It records no login history for the new boot. .plans/plans/go-s3-stack-client/plan.md:281, .plans/plans/go-s3-stack-client/plan.md:292, src/contracts/proto/lgtm/v1/clients.proto:67, .plans/plans/go-s3-server-machines/plan.md:343.

**Failure.** The operator logs in while the client is still waiting for the stack and leaves that session after the client connects. Lingering keeps the connection. A later login can now satisfy the comparison because the first session is gone. The displayed observation passes even though the connection started after a login. A wall-clock correction between the two observations can also invert numeric ordering on one host.

**Fix.** Require a boot-scoped record of the earliest successful interactive login for the fixture user, including sessions that ended. Define its producer and the connection evidence on a common monotonic time base, or have a separate trusted observer record the authenticated test machine while the fixture user is still logged out. Mark the gate inconclusive when the required boot or ordering evidence is missing. Test a short earlier session followed by a later login.

### threat-r3 04 — MAJOR — D19 — The remaining service-custody boundary has no refusal

**Evidence by reading.** The installer validates path spelling, places a program and a service definition, and enables persistent execution. The new post-build check explicitly permits writing under an ancestor another local user can change. The client already checks the ownership and write modes of resolved state-folder ancestors, but that check never covers the installer’s service endpoints. .plans/plans/go-s3-stack-client/plan.md:198, .plans/plans/go-s3-stack-client/plan.md:205, .plans/plans/go-s3-stack-client/plan.md:208, .plans/plans/go-s3-stack-client/plan.md:257, .plans/plans/go-s3-stack-client/plan.md:328, src/client/internal/state/folder.go:131, src/client/internal/state/folder.go:166.

**Failure.** A user has an otherwise valid home path with a group-writable program or service parent. Installation reports success and enables a service whose program or definition can be changed by another operating-system user. The service will later execute outside the installing user’s control and with access to that user’s files.

**Fix.** Require trusted ownership and no untrusted write access for the resolved binary, service definition and their directory chains before installing, updating or removing the service. Apply equivalent custody to the configured macOS log destination. Refuse unsafe destinations before minting a token. Give the post-build check an expected refusal and require the original files and service to be preserved.

### threat-r3 05 — MAJOR — R9 — The gate prerequisite covers only part of the sign-in credential path

**Evidence by reading.** Gate G4 points to round 1, which specifies only changing how the password enters the JavaScript Object Notation (JSON) encoder. The round 2 handoff also calls for tracing off, private jars and sign-out on error. The patched live gate still calls its own sign-in flow without requiring tracing off or revocation on a failed path. The checked-in driver invokes helpers with password arguments at the shell level, including a changed password. The installer’s own `set +x` does not govern either test script. .plans/plans/go-s3-stack-client/plan.md:194, .plans/plans/go-s3-stack-client/plan.md:280, .plans/plans/go-s3-stack-client/plan.md:319, .foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:44, .foundry/go-rewrite/results/fold-go-s3-stack-client-r2.md:48, deploy/test/checks.sh:403, deploy/test/checks.sh:422.

**Failure.** When a failed gate is retried with shell tracing enabled, the helper call or request body can appear in retained diagnostics even if the encoder reads standard input. If the test exits after a successful sign-in, leaving the password and fixture stack in place, deleting scratch files alone leaves the server session valid.

**Fix.** Require both drivers to disable tracing before handling credentials and to pass the exact password bytes and encoded request body over standard input. Require separate cleanup of the server session and the owner-only local credential files on normal and error exits. State the current requirements in gate G4 itself. Add a redacted trace-on failure-path check with a throwaway password and a session that is proven invalid afterward.

### threat-r3 06 — MAJOR — R9 — The installer can surface a secret in a Compose diagnostic

**Evidence by reading.** The new Compose path is shared in command spelling with the launcher. The launcher plan also requires quiet handling of configuration parse failures, which the installer’s port lookup and token command do not specify. The Compose failure test changes only the exit code of the stand-in. .plans/plans/go-s3-stack-client/plan.md:96, .plans/plans/go-s3-stack-client/plan.md:202, .plans/plans/go-s3-stack-client/plan.md:206, .plans/plans/go-s3-stack-client/plan.md:217, .plans/plans/go-s2-stack-web/plan.md:236.

**Failure confirmed live for the diagnostic.** With an unterminated quote on a disposable dummy setting, Docker Compose v5.5.0 failed its `port` command and repeated the setting’s value on standard error. An installer that captures only standard output can print the owner’s malformed password before its own fixed error sentence.

**Fix.** Require the installer and gate G1 to suppress Compose diagnostics from credential-bearing configuration during configuration selection and token issuance, then print a fixed diagnostic on failure. Apply the launcher’s quiet configuration check before using the selected stack. Test a malformed dummy value and verify that the retained output contains only the fixed message.

### threat-r3 07 — MAJOR — C6 — The browser gate’s address has no assigned path through the website

**Evidence by reading.** Gate G5 assigns a command spelling to a team that may edit the website, but the sprint’s approved team table grants no team that scope. The gate omits the round 2 handoff’s source for the address. The server plan writes the token’s normalized address in `CreateClientTokenResponse.address`. The current website drops that field in the token adapter and renders a server label in the command. .plans/plans/go-s3-stack-client/plan.md:320, .plans/epics/go-rewrite/epic.md:111, .plans/epics/go-rewrite/epic.md:121, .foundry/go-rewrite/results/fold-go-s3-stack-client-r2.md:49, src/contracts/proto/lgtm/v1/clients.proto:123, .plans/plans/go-s3-server-machines/plan.md:573, src/frontend/src/server/sync.ts:117, src/frontend/src/screens/service.tsx:410.

**Failure.** The gate has no authorized builder in the sprint as written. If a builder changes only the command spelling, the adapter still discards the canonical address, and a label that differs from the token’s origin yields a command the client refuses. Opening a terminal in another folder also leaves an earlier enrollment in the client’s default state folder in place.

**Fix.** Have the orchestrator grant and assign the website work explicitly. Include the adapter, dialog and sample branch in the change. Use the token response’s address for the real command and exercise it with a real nondefault port. Supply the new client on the test terminal’s search path and use a fresh private state directory on the stack host.

### threat-r3 08 — MINOR — D16 — The interrupted-join check names the wrong process to stop

**Evidence by reading.** Step 13 starts the client as a child. The client writes the identity after it receives and verifies the server’s reply. The new post-build check kills the shell installer and assumes the client cannot complete its write. .plans/plans/go-s3-stack-client/plan.md:206, .plans/plans/go-s3-stack-client/plan.md:326, .plans/plans/go-s3-client-link/plan.md:261.

**Failure.** If the check signals only the installer process after the server commits, the child can finish enrolling and publish `identity.json`. The retry correctly follows the complete-enrollment branch and fails the proposed check.

**Fix.** Control the interruption at the client or its isolated process group and place a deterministic barrier between the server commit and the client write. Verify that the client is gone, one waiting row exists, and the saved key has no identity before starting the retry.

### threat-r3 09 — MINOR — D7 — The manual reinstall guide leaves an approved key registered

**Evidence by reading.** The new guide uses local `--remove`, a manual state-folder move and a new install after changing ports. Local removal retains the state and server record. The approved design revokes a machine’s registered key on server-side removal. .plans/plans/go-s3-stack-client/plan.md:200, .plans/plans/go-s3-stack-client/plan.md:295, .plans/plans/go-s3-stack-client/decisions.md:35, docs/specs/2026-11-25-go-rewrite-foundation-design.md:254.

**Failure.** An owner follows the changed-port procedure and approves the replacement against the same database. The previous record remains approved, and the retained private key still identifies that machine to the server.

**Fix.** In the documented recovery, identify the old machine in the correct stack and remove its record if it still exists before archiving its state. Say that the saved folder contains a private key and must retain its private permissions until the owner deletes it.

### Finding metadata

The numerical estimates multiply impact, fallout and probability. Required gate defects are recorded as gate failures. The planner owns disposition.

| Finding | Category and evidence | Responsible area | Risk estimate or gate |
|---|---|---|---|
| 01 | Integrity, confirmed by reading. | Stack installer. | 6 × 3 × 6 = 108. A routine update with an alternate setting displaces an approved client until the owner repairs it. |
| 02 | Verification and integrity, confirmed by reading and a configuration probe. | Stack launcher and sprint gate writer. | Required gate. A supported override can select different stacks. |
| 03 | Verification, confirmed by reading. | Stack live test writer. | Required gate. The proof can omit the first login. |
| 04 | Integrity and isolation, confirmed by reading. | Stack installer. | 6 × 7 × 4 = 168. A writable destination can expose the account’s private data across local users. |
| 05 | Confidentiality, confirmed by reading. | Both sign-in test writers. | 6 × 7 × 2 = 84. A traced failing test can expose a disposable stack’s still-valid credential. |
| 06 | Confidentiality, confirmed live for Compose and by reading for the plan. | Stack installer and gate G1 writer. | 6 × 7 × 4 = 168. A malformed setting can expose the real account password. |
| 07 | Design and verification, confirmed by reading. | Orchestrator and website writer. | Required gate. The current command’s origin does not come from the token response. |
| 08 | Verification, confirmed by reading. | Stack live test writer. | 1 × 1 × 2 = 2. The test may miss the intended failure window. |
| 09 | Credential lifecycle, confirmed by reading. | Stack documentation. | 2 × 3 × 6 = 36. A routine migration retains one superseded identity until the owner removes it. |

## Answers to the assignment questions

1. **Each numbered patch.** The table follows the patch list in the round 2 register.

| Patch | Result |
|---|---|
| 1. Overview. | The first installation still joins. The overview and reboot promise depend on the incomplete ordering evidence in finding 03. The state-folder update to the user-flow script remains assigned. |
| 2. Key decisions. | Removing the automatic move closes the rejoin-specific failure. The second-run promise fails in finding 01, and the claimed shared Compose rule fails in findings 02 and 06. |
| 3. Client workstream. | The narrowed result of an installer run is reflected in the detailed service rule. The identity and pre-login promises need findings 01 and 03. |
| 4. Rollout. | The image rebuild and merged Machines page are now prerequisites. The website assignment is still missing in finding 07. |
| 5. Public names. | The obsolete rejoin option is removed and the token issuer signature agrees with its writer. The body of the installer still lets a new override displace the unit’s recorded setting. Finding 01. |
| 6. Service, address and exit rules. | The accepted-start contract removes the old five-second health inference. The default-port textual comparison is gone. The shared Compose and existing-state rules still fail in findings 01 and 02. |
| 7. Token command. | Nothing found in the changed issuer call or PostgreSQL test prerequisite. The issuer owns the address and the wrapper supplies a real database. Gate G1’s changed invocation is covered under patch 13. |
| 8. Workstream criteria and anchors. | The criteria cover a fresh failure, a normal repeat and removal without build tools. The repeat criterion misses a conflicting override. Finding 01. |
| 9. Linux installer and stand-ins. | The fresh-join order and restart directives have direct observations. The port stand-in distinguishes unset, empty and explicit values. The removal branch precedes build-tool checks. Findings 01, 02, 04 and 06 cover the remaining boundaries. |
| 10. macOS branch. | The rejoin branch is removed. The state-setting override still wins over the installed agent, and the agent’s destinations share the custody question. Findings 01 and 04. |
| 11. Live test and guide. | The new private fixture closes inherited-state reuse on the default run. Later parts check the unit’s state folder. The login evidence, credential exception paths and old-key revocation need findings 03, 05 and 09. |
| 12. Risk table. | Service status and the user-visible connection outcome are distinguished. The shared-port mitigation omits the contradictory sprint commands, and the boot mitigation inherits the comparison gap. Findings 02 and 03. |
| 13. Sprint gates, post-build checks and deferrals. | The PostgreSQL wrapper supplies C12’s database, and the pasted-command check adds a needed real observation. The selected Compose project, credential paths, gate ownership, crash process and destination custody remain open in findings 02 through 08. |
| 14. Traceability, guardrails and attack log. | The new entries retain the C6 browser gate and the build-only scope of the installer. Their recorded links do not resolve the active G4 and G5 prerequisite gaps in findings 02 and 07. |

2. **New mechanisms.** First-install-only removes the automated move and rollback hazard, but ordinary updates still accept a different enrollment folder. The new boot comparison uses authenticated presence for the current stream, but it can miss earlier ended logins and cannot establish ordering across a clock step. Findings 01 and 03.
3. **Rejected findings.** Nothing found. I checked every outcome in `attack/round-2-findings.md`. No row says rejected. The retained custody issue is listed as a post-build check and is evaluated in finding 04.
4. **Writers and readers.** The removed option has no active installer or guide reader. The remaining state setting is written by the installer and read by `join`, both service managers, reruns and the live fixture commands. The token issuer’s options and address have matching producers and installer consumers. The actual browser adapter omits the token address. The new reboot reader has current presence and no complete login-history producer. Compose’s launcher, installer, gate G1, driver helper and direct C2 call do not yet share one selector. Findings 01, 02, 03 and 07.
5. **Interacting patches.** The table below traces the branches that can change enrollment, cross stacks or misclassify a boot.
6. **Previously admitted majors.** No blocker was recorded in round 2. The single-run part of G2, Compose prerequisite in G4, boot proof in G8, complete browser handoff in G9 and full credential controls in G10 remain open. G3 removed the archive risk and explicitly deferred service-path custody. G1 removed automatic archive and rollback. G5 closes the inherited fixture-state case, G6 narrows the success claim, G7 removes the text comparison, and G11 through G15 address their stated tests or interface gaps.
7. **Lens coverage.** The patches touch trust boundaries, credential inventory and termination, transport, dependencies, integrity and post-build evidence. The seven lens answers follow.

| Conditions | Result under the patched rules | Required result |
|---|---|---|
| Installed unit, override absent or pointing to its own enrolled folder. | The installer skips join and restarts with the same identity. | Preserve that identity. |
| Installed unit, override points to an empty folder. | The installer joins and changes the service identity. | Refuse the conflicting override before minting. |
| Installed unit, override points to a fully enrolled different folder. | The installer skips join and changes the service identity. | Refuse the conflicting override before changing files or restarting. |
| No installed unit, selected folder has a key without an identity. | The installer refuses before minting. | Preserve the key and give recovery instructions. |
| No installed unit, selected folder has a complete identity. | The installer skips join and starts the service from that identity. | Reuse the saved enrollment and show service and connection status separately. |
| Compose override unset or empty. | The installer uses `-f`. The referenced round 1 driver rule exports `COMPOSE_FILE`. | Use the same resolved configuration and port. |
| Compose override set. | The installer reads it. Gate G1 and the checked-in C2 override force a file. | Carry the selected base and overrides through every gate operation. |
| Current connection began before the earliest recorded login of the new boot. | The patched timestamp comparison can pass. | Pass with verified ordering and complete boot-scoped evidence. |
| Current connection began after an ended login and before the current one. | The patched timestamp comparison can pass. | Fail the before-login claim. |

## Answers to the threat lens

1. **Trust boundaries.** The installer trusts the checkout, host tools, selected Compose project, readiness response, environment and installed unit. The readiness response checks product and protocol shape. Enrollment checks the normalized token address and pinned server key. The patched selection rules and installer destination custody leave the gaps in findings 01, 02, 04 and 06. The unchanged question of a remote Docker endpoint is still with the owner. .plans/plans/go-s3-stack-client/plan.md:199, .plans/plans/go-s3-stack-client/plan.md:203, .plans/plans/go-s3-client-link/plan.md:103, .foundry/go-rewrite/results/fold-go-s3-stack-client-r2.md:37.
2. **Credential inventory.** The server creates a join secret and stores its verifier. The command writes the credential once to standard output, and the installer holds it in a shell variable until handing it to the client as an argument. Gate G1 can retain it in a private file and gate G5 handles it through the clipboard and test shell. The client creates the private `machine.key`. The file `identity.json` holds enrollment metadata. The token command reads the protected PostgreSQL and OpenBao credentials and the server signing key from the existing vault path. The database wrapper makes separate disposable database credentials. The website password enters the sign-in body, the test keeps a session cookie in its private fixture, and the server stores the verifier, salt and password binding. The authenticated link’s short-lived connection token stays in memory. Findings 04 through 06 cover the changed local destinations or diagnostic paths. .plans/plans/go-s3-server-machines/plan.md:259, .plans/plans/go-s3-server-machines/plan.md:343, .plans/plans/go-s3-stack-client/plan.md:133, .plans/plans/go-s3-stack-client/plan.md:206, .plans/plans/go-s3-stack-client/plan.md:280, .plans/plans/go-s3-stack-client/plan.md:316, .plans/plans/go-s3-stack-client/plan.md:320, deploy/test/with-postgres.sh:66, docs/specs/2026-11-25-go-rewrite-foundation-design.md:202.
3. **Credential endings.** A join token expires after 15 minutes and can be revoked. Consumption binds it to one enrollment, with same-key retries governed by the server’s redemption rules. Machine removal revokes the registered key. Local removal leaves the key file and server row intact. Sign-out, expiry and password rotation end website sessions. The wrapper removes its disposable database and credential files. The OpenBao batch token expires under its existing lease, and server signing-key rotation is outside this patch. The guide’s manual migration and failed sign-in cleanup need findings 05 and 09. .plans/plans/go-s3-server-machines/plan.md:258, .plans/plans/go-s3-server-machines/plan.md:270, .plans/plans/go-s2-server-signin/plan.md:338, .plans/plans/go-s2-server-signin/plan.md:451, deploy/test/with-postgres.sh:22, .plans/plans/go-s3-stack-client/decisions.md:35.
4. **Encryption.** The selected address is loopback, where the approved design allows cleartext Hypertext Transfer Protocol version 2 (HTTP/2). The private Compose proxy hop remains under the Stack Web plan. Public Transport Layer Security (TLS) and the certificate questions in issue 1633 are outside the patch. The issuer rejects an encrypted public address without a pin. OpenBao protects the server signing key at rest. The client’s on-disk private key relies on local file permissions and retained-state custody. .plans/plans/go-s3-stack-client/plan.md:96, .plans/plans/go-s3-stack-client/plan.md:134, .plans/plans/go-s2-stack-web/plan.md:131, docs/specs/2026-11-25-go-rewrite-foundation-design.md:215, docs/specs/2026-11-25-go-rewrite-foundation-design.md:316, docs/issues.md:7520, src/client/internal/enroll/credential.go:63.
5. **Dependencies and images.** Nothing found among added modules or images. The installer trusts the operator’s checkout and host Go, Docker, Compose and service manager. The new PostgreSQL wrapper call uses its existing digest-pinned image. .plans/plans/go-s3-stack-client/plan.md:201, .plans/plans/go-s3-stack-client/plan.md:205, .plans/plans/go-s3-stack-client/plan.md:371, deploy/test/with-postgres.sh:12.
6. **Integrity.** The server writes the host mark from a privileged issuance path, and signed link admission plus owner approval govern the machine’s access. The patched override can change which enrollment the same service runs. Another local user can influence an enabled service where its executable or definition lacks protected custody. The reboot check can misstate what this machine proved. Findings 01, 03 and 04. docs/specs/2026-11-25-go-rewrite-foundation-design.md:247, .plans/plans/go-s3-stack-client/plan.md:16, .plans/plans/go-s3-stack-client/plan.md:199, .plans/plans/go-s3-stack-client/plan.md:292, .plans/plans/go-s3-stack-client/plan.md:328.
7. **Post-build evidence.** Observe the installed service identity, process interruption, credential channels, stack selectors, the real browser command and a boot with an independent before-login observation. The plan decisions required for those checks are the findings above.

## Post-build checks

| Scenario | Expected result | Evidence and responsible area |
|---|---|---|
| Repeat an install on Linux and macOS with an exported alternate folder. | The install refuses before changing the program, service or server records. A subsequent valid update retains the recorded machine. | Keep redacted exit status, service state and before-and-after machine identifiers. Stack installer. |
| Interrupt the client after its join commits and before publication of the identity. | The retry reports an unfinished join. One waiting row and one private local key remain, with no installed service. | Keep the interruption marker, row count and file presence. Stack installer and Client Link. |
| Present a program, unit, agent or log parent writable by another local account. | The unsafe installation refuses and leaves the previous installation intact. | Keep ownership and mode observations and redacted results. Stack installer. |
| Run the installer and both deployed drivers with caller tracing and throwaway credentials. Exercise malformed configuration and sign-out after a failed assertion. | Captured diagnostics omit passwords, cookie values and join token text. The old cookie is rejected after sign-out, and private temporary files are removed. Exercise the owner’s existing positional-token policy separately. | Keep redacted assertions and session rejection only. Stack installer and sprint test writers. |
| Start a disposable stack on a nondefault port with an explicit Compose override, then run the installer, G1, G4 and the browser check. | The gates use the intended stack. The real page’s command enrolls the fresh test client and the same open page gains one waiting row. Revoke test tokens and clear disposable clipboard and shell history after the run. | Keep project selection, port and observation without a credential. Stack and website test writers. |
| Reboot with lingering. Exercise both a real pre-login connection and a delayed connection after an early session has already ended. | The first case passes, the second fails the claim, and evidence that cannot establish the ordering is inconclusive. | Keep the boot identifier, the recorded machine and the ordering evidence from both events. Stack live test writer. |

## Grade

5 of 10. There are 0 blockers, 7 majors and 2 minors.
