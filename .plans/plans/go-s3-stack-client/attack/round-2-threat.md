REVISE

A rejoin still stops the working client before the replacement is ready, and an archive or activation failure can leave the service down without a defined recovery path.

## Findings

This is a plan review of the changes from `6e5abd4b` to `90d8d4bb`. The plan and decisions match those revisions at checkout `6c64cde3`. The findings come from reading the plans and source at that checkout. The proposed installer, service and live checks are build targets. D numbers identify design decisions, R numbers identify design rules, and C numbers identify design checks.

### threat-r2 01 — MAJOR — D16 — Rejoin recovery ends at a failed join command

**Evidence.** The plan promises preservation until join succeeds, but stops the old service before reserving or moving the archive. It restores state only after `join` returns failure. Program replacement and service activation are separate later steps with no rollback. Post-build check PB1 expects recovery after an interruption but requires a message that step 11 can only produce after the stale lock has been removed. `.plans/plans/go-s3-stack-client/plan.md:28`, `.plans/plans/go-s3-stack-client/plan.md:229`, `.plans/plans/go-s3-stack-client/plan.md:240`, `.plans/plans/go-s3-stack-client/plan.md:244`, `.plans/plans/go-s3-stack-client/plan.md:379`, `.plans/plans/go-s3-stack-client/decisions.md:49`.

**Failure.** The working service stops and archive creation fails because the state filesystem is full. The restore branch is never reached. The same outage can follow a successful join when reloading or starting the new service fails. If the service was already inactive, the failure handler also starts it simply because its unit exists. A failed join can leave a server-side waiting row even if the installer deletes the new local key and restores the old one. The Client plan explicitly describes an uncertain join and a local file that may have been placed before a failed sync. `.plans/plans/go-s3-client-link/plan.md:106`, `.plans/plans/go-s3-client-link/plan.md:262`.

**Fix.** Stage the new enrollment in a private sibling directory while the old service runs. Record the old service's enabled and active states, and preserve its binary and definition until activation is verified. Specify the recovery steps for archive, swap, reload and activation failures, including interruption and any failed restore. Use a private recovery record to distinguish each completed move and the attempted enrollment. Retain pending enrollment state privately long enough to correlate it with the waiting row and give the owner exact cleanup steps. Restore only the original service state. Change PB1 to cover failure after the stop or swap as well as failure before any change.

### threat-r2 02 — MAJOR — D19 — The lock guards the state path and leaves the service shared

**Evidence.** Step 6 reads the service before acquiring the lock. Step 7 locks a directory beside the selected state path. The program path and Linux unit or macOS agent are shared by every installer invocation for that user. The existing test only places a lock beside one state path. `.plans/plans/go-s3-stack-client/plan.md:84`, `.plans/plans/go-s3-stack-client/plan.md:91`, `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:229`, `.plans/plans/go-s3-stack-client/plan.md:285`, `.plans/plans/go-s3-stack-client/plan.md:299`.

**Failure.** One run uses the installed state under one parent and a concurrent run has an override under another. Both acquire a lock. Both can enroll a machine and replace the same executable and service definition. A concurrent removal can delete the newly installed program. A single run with an override that disagrees with the installed unit can also replace its identity through an ordinary update.

**Fix.** Acquire a per-user lock in a verified private directory before reading the installed definition. Hold it through every install, rejoin, removal and recovery. Release only the lock that the current invocation owns. Define how an abandoned lock is recovered. Bind an update or removal to the installed state directory and refuse a conflicting override unless a separately defined migration permits it. Cover conflicting parents and install versus remove in the installer tests.

### threat-r2 03 — MAJOR — D19 — Allowed filenames fail to establish ownership of state or service paths

**Evidence.** The installer allows any absolute path of the allowed character set and permits archival when the final component passes a symbolic-link check and its entries match an allowlist. It creates the lock beforehand. The client's real state open checks ancestors, ownership and private mode, but it runs after the installer has selected and moved the old directory. The installer has separate unchecked destinations for the program, unit, agent and agent log. `.plans/plans/go-s3-stack-client/plan.md:227`, `.plans/plans/go-s3-stack-client/plan.md:229`, `.plans/plans/go-s3-stack-client/plan.md:236`, `.plans/plans/go-s3-stack-client/plan.md:237`, `.plans/plans/go-s3-stack-client/plan.md:244`, `.plans/plans/go-s3-stack-client/plan.md:301`, `.plans/plans/go-s3-stack-client/plan.md:302`, `src/client/internal/state/folder.go:63`, `src/client/internal/state/folder.go:131`.

**Failure.** A private directory for another application that happens to contain only `settings.json` passes the check and is moved during rejoin. A state path with an ancestor other local users can change is unsafe to check and then move. A group-writable service parent also leaves the executable or service configuration replaceable after installation. Archive name uniqueness alone does not establish custody of archived private keys.

**Fix.** Require positive evidence of a dedicated client directory before moving it, such as a validated installation record and a safe path for verified unfinished state. Treat an empty unrelated directory as a refusal. Normalize paths, check type and ownership before any move, and use stable parent directories protected against replacement. Apply the client's ancestor and private-mode policy to the state and archive. Apply ownership and safe-parent checks to the binary, service and log destinations. Document that archives retain live private keys until the old machine is revoked, and when the owner can delete the archive.

### threat-r2 04 — MAJOR — R9 — The new sprint gate still runs a password-bearing argument path

**Evidence.** The new live test moves password encoding to standard input and adds private scratch and sign-out. It has no trace rule for the password or cookie operations. Gate G4 invokes a different script whose `signin` and `signin_rejected` still pass the password as a `jq` argument at the current commit. The fold requests the change from the script's test writer, but G4 does not require it. `.plans/plans/go-s3-stack-client/plan.md:328`, `.plans/plans/go-s3-stack-client/plan.md:373`, `deploy/test/checks.sh:46`, `deploy/test/checks.sh:82`, `deploy/test/checks.sh:90`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:41`.

**Failure.** The new live test can handle its password correctly while the sprint driver still exposes its configured password while signing in. With shell tracing enabled, a shell can also print an expanded password or request body even when the encoder receives its input on standard input. A cookie left after failed sign-out stays usable until server expiry or revocation.

**Fix.** Make the driver owner's credential-handling change a named gate dependency. Require both scripts to disable tracing before handling credentials, encode exact password bytes through standard input, send the encoded body through standard input, and use owner-only scratch files. Specify sign-out on ordinary completion and error paths that have an active session, with independent local file cleanup. Use a throwaway password throughout the sprint gate and check that the trace, process arguments and reported output exclude it.

### threat-r2 05 — MAJOR — C6 — The sprint gate can pass while the Machines page offers a broken command

**Evidence.** The epic's team table confirms that this plan cannot edit the website. The Client Link plan requires pasting the displayed command, and the fold requests an assignment, but the patched sprint gate omits the browser observation and traceability lists issue 1636 with no workstream. The page still renders the old command and derives the address from the displayed server label. G4's shell check invokes the new client itself. `.plans/epics/go-rewrite/epic.md:121`, `.plans/plans/go-s3-client-link/plan.md:74`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:46`, `.plans/plans/go-s3-stack-client/plan.md:368`, `.plans/plans/go-s3-stack-client/plan.md:415`, `src/frontend/src/screens/service.tsx:410`, `deploy/test/checks.sh:219`, `deploy/test/checks.sh:447`.

**Failure.** The stack rolls out with 13 passing driver checks. A person copies the real page's command, which uses the wrong executable spelling and flag for the new client. The open Machines page never shows the expected waiting row.

**Fix.** Name the website change as a sprint prerequisite assigned by the orchestrator to a writer with website scope. Require the displayed command to use the address returned with the token and the agreed executable and arguments. Add the Client Link browser observation to the sprint gate or require its recorded pass there.

### threat-r2 06 — MAJOR — D16 — The boot test proves a process start and a later connection at different times

**Evidence.** The revised test checks a new boot identifier and compares unit and login timestamps. Another test checks that the row is online within 60 seconds of starting the test, after login. The unit timestamp has no link to the server's authenticated connection for that machine. `.plans/plans/go-s3-stack-client/plan.md:326`, `.plans/plans/go-s3-stack-client/plan.md:341`, `.plans/plans/go-s3-stack-client/plan.md:342`, `.plans/epics/go-rewrite/epic.md:33`.

**Failure.** The lingering client process starts before login while the web service is unavailable. The web service first answers after the operator logs in. Both assertions pass even though the client had no authenticated link before login, which is the remaining part of the round 1 finding.

**Fix.** Require proof that the recorded approved machine authenticated in the new boot before the first interactive login. For example, capture a boot-scoped timestamp for an authenticated ready event and compare it with the first login on the same time base. Require a real first login record and scope the restart journal record to the ended service invocation. Preserve the separate macOS login gate.

### threat-r2 07 — MAJOR — C6 — The live fixture does not identify the installation it is allowed to remove

**Evidence.** The guard checks the program, unit and default state folder. The real installer can select another existing state folder through `LGTM_CLIENT_STATE_DIR`. The fixture records the selected project and a machine identifier obtained by comparing snapshots, but the plan gives no stable stack identity check on resumed phases. Cleanup invokes `--remove` against the installed service without checking that it still belongs to that fixture. `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:230`, `.plans/plans/go-s3-stack-client/plan.md:327`, `.plans/plans/go-s3-stack-client/plan.md:329`, `.plans/plans/go-s3-stack-client/plan.md:330`, `.plans/plans/go-s3-stack-client/plan.md:335`, `.plans/plans/go-s3-stack-client/plan.md:379`.

**Failure.** An exported state override names an existing enrollment even though the test's three default paths are absent. The installer can start using that enrollment before the test discovers that no new waiting row exists. After a valid first phase, a later unrelated installation can occupy the shared service path. A stale fixture still authorizes cleanup of that service.

**Fix.** Create a private fixture directory and pass its absolute path explicitly. Verify dedicated stack selection and unused service paths before launching. Correlate the new row with the machine identifier in the enrollment just written. Persist the stable stack selector, origin, server identity, state directory and machine identifier. On both resumed phases, verify those bindings and verify the service is still the fixture's before any removal or stop.

### threat-r2 08 — MAJOR — C6 — The rerun compares serialized text and accepts service liveness as enrollment health

**Evidence.** The patched installer looks for a text pair in `identity.json` and looks for the service to stay active for five seconds. The Client Link plan normalizes origins by removing default ports and validates the saved record at runtime. The Server Machines plan drops the default plain Hypertext Transfer Protocol (HTTP) port from the token origin. Stack Web's documented port range includes that port. `.plans/plans/go-s3-stack-client/plan.md:234`, `.plans/plans/go-s3-stack-client/plan.md:246`, `.plans/plans/go-s3-client-link/plan.md:90`, `.plans/plans/go-s3-client-link/plan.md:106`, `.plans/plans/go-s3-server-machines/plan.md:258`, `.plans/plans/go-s2-stack-web/plan.md:272`.

**Failure.** On a stack published on the default HTTP port, a saved canonical origin without an explicit port describes the same stack, but the installer can refuse the rerun because its Compose-derived string includes the port. An identity for a different server at the same origin clears the text check. When the client is still waiting on a connection or has not received a refusal in five seconds, the installer can announce success without a validated enrollment.

**Fix.** Agree with the Client team on a read-only local enrollment check that reuses its private-file reader and origin normalization, and a bounded readiness result that distinguishes waiting, authenticated online and still connecting. Check the saved identity before replacing the program and service. Use the readiness result to report the current installation honestly. Cover a canonical default-port origin, a corrupted enrollment and a same-origin changed server in the respective tests.

### threat-r2 09 — MAJOR — C5 — The sprint driver can inspect one stack and mutate another

**Evidence.** The installer honors `COMPOSE_FILE`, while the checked-in driver `compose()` always supplies `-f`. Gate G4 names the database helper and tells the operator how to set the web port, but it omits the pending Compose selector change in the fold. The driver recreates the server during C4 and modifies the join-token table during C5. `.plans/plans/go-s3-stack-client/plan.md:83`, `.plans/plans/go-s3-stack-client/plan.md:373`, `deploy/test/checks.sh:22`, `deploy/test/checks.sh:32`, `deploy/test/checks.sh:416`, `deploy/test/checks.sh:435`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:41`.

**Failure.** A gate run uses a Compose override and exports its web port. Direct calls reach the intended fixture port, while the fixed `-f` drops the override for a lifecycle or database call. The run can fail against a different database or recreate the server with the base definition instead of the verified fixture definition.

**Fix.** Make the script writer's Compose selector change a prerequisite for G4. Use the launcher and installer's effective project and file list for rebuild, port discovery, lifecycle and database calls. Test G4 with a disposable project and an override, and require the selected web endpoint and database to belong to that project before any mutation.

### threat-r2 10 — MINOR — D17 — The token command still refers to a deleted address option

**Evidence.** The changed token behaviour says to call `Tokens.Create` with options and an empty address. The neighboring signature has two inputs, a context and options, and the only option is `HostsServer`. Decision 37 repeats the outdated rationale. The patched public name table and issuer declaration otherwise match. `.plans/plans/go-s3-stack-client/plan.md:99`, `.plans/plans/go-s3-stack-client/plan.md:155`, `.plans/plans/go-s3-stack-client/decisions.md:46`, `.plans/plans/go-s3-server-machines/plan.md:145`.

**Failure.** A builder following the prose literally can add a third argument and fail compilation. The correct public signature gives the builder enough information to resolve the local inconsistency.

**Fix.** Write the exact call `Tokens.Create(ctx, machines.TokenOptions{HostsServer: hostsServer})`. Say that it always uses `Issuer.PublicURL`. Update decision 37 and the stale scouting note to match.

### Finding metadata

These are reviewer estimates. Each numeric score multiplies impact, fallout and probability. A score records consequence and leaves the build policy and disposition with the owner.

| Finding | Category and evidence | Responsible area | Risk estimate and basis |
|---|---|---|---|
| 01 | Integrity, confirmed by reading. | Stack installer | 6 × 5 × 4 = 120. A filesystem or service failure strands a working client until recovery. |
| 02 | Integrity, confirmed by reading. | Stack installer | 6 × 3 × 4 = 72. Overlapping local invocations can create conflicting enrollments and a broken service. |
| 03 | Integrity and isolation, confirmed by reading. | Stack installer | 6 × 5 × 4 = 120. A misplaced override or shared path can move unrelated data or break service custody. |
| 04 | Confidentiality, confirmed by reading. | Driver test writer and Stack live tests | 6 × 7 × 4 = 168. A disclosed website password can grant access until it is changed. |
| 05 | Design, confirmed by reading. | Sprint gate and website | Required gate. The current page emits an unsupported command. |
| 06 | Verification, confirmed by reading. | Stack live tests | Required gate. The two passing observations admit the reported failure. |
| 07 | Integrity, confirmed by reading. | Stack live tests | 6 × 3 × 2 = 36. A reused tester account or stale fixture can affect an installation outside the fixture. |
| 08 | Functional failure, confirmed by reading. | Stack installer and Client Link | 6 × 3 × 4 = 72. A supported port or stale enrollment can prevent or misreport a routine update. |
| 09 | Integrity and verification, confirmed by reading. | Driver test writer and sprint gate | Required gate. Divergent Compose calls can affect the wrong stack configuration. |
| 10 | Design, confirmed by reading. | Server command | 2 × 1 × 2 = 4. Correcting the ambiguity is local to the command's specification. |

## Answers to the task questions

1. **Every patch.** The following rows follow the register's patch numbering. The decision changes were checked against the same groups.

| Patch | Result |
|---|---|
| 1. Overview. | The revised boot and mark wording is narrower. The Linux boot proof leaves the gap in finding 06. The issuance mark describes an operator action, and the remote endpoint choice is still with the owner. |
| 2. Key decisions. | Fetching the token first fixes an early failure. The preservation and safe rerun promises still fail in findings 01, 02 and 08. |
| 3. Workstreams. | The next-connection wording closes the old immediate-removal promise in G15. The claim that rejoin changes nothing before success conflicts with the stop and rename. Findings 01 and 06 cover the other claims. |
| 4. Rollout order. | The Server seats and image rebuild close the named ownership and ordering gaps. Browser evidence and driver prerequisites are incomplete in findings 05 and 09. |
| 5. Design citations. | Nothing found. Checked rule 8 and the definition of a workstream against the approved design and the plan's two workstreams. |
| 6. Public names. | The issuer, store policy and close method line up with their writers. Findings 02, 03, 08 and 10 cover the new filesystem scope, saved address reader and obsolete address input. |
| 7. Service rules. | Removal timing is now accurately narrow. Normalization, rollback and stopped-service recovery still fail in findings 01 and 08. |
| 8. Token command. | G1's missing fields and vault close are removed. Fixed usage and the output-writer failure now have tests. The stale address prose remains in finding 10. |
| 9. Workstream 2 criteria. | Per-item tests no longer require a real stack. The first failure path and simple repeat are covered. The failure criteria miss cross-path locking, archive failures and state validation in findings 01 through 03 and 08. |
| 10. Linux item. | The empty port, malformed token and unfinished fresh join reach their refusal branches. G16's distinct missing-tool and Docker failures are specified. The lock and directory changes create findings 01 through 03. G17 and G18 add a bounded probe, observable mutations and a whole-fixture secret scan. The product probe establishes readiness only. |
| 11. macOS item. | Graphical-session checks and launch agent observation are explicit. The per-user lock, protected paths and rollback gaps also apply to macOS. The unsupported-system and parser cases do not exercise those states. |
| 12. Live test and guide. | The boot identifier, ended run's status, standard-input password handling and documented process-argument exposure address parts of G8, G10 and G14. Findings 04, 06 and 07 identify remaining verification and cleanup gaps. |
| 13. Workstream 2 risks and verification. | The split between hermetic and deployed checks is sound. The risk rows overstate recovery and origin agreement. Findings 01 and 08 apply. The positional token still goes to the owner, as G8 records. |
| 14. Sprint gate, post-build checks and scope. | The authenticated C5 helper is named and present in the checked-in script. The browser, credentials and Compose dependency gaps appear in findings 04, 05 and 09. In PB1 a stale lock blocks the expected result until someone removes it. PB3 also needs to account for the parent created before Docker is tested. The second operating-system user remains out of scope. |
| 15. Traceability. | The state-folder producers and C10's retained file path agree. R9 evidence omits the real sign-in channels and C6 evidence omits the browser. Findings 04 and 05 apply. |
| 16. Guardrail and execution sizing. | Nothing found in the revised ownership and load. The Server seats own the three server files and items 1 through 3 have no deployed stack gate. The planned cross-team verification remains at sprint scope. |

The decision log's revised status labels preserve the dated history. Decision 45 supersedes decision 39's old reference to row 19 and leaves standard input with the owner. Decision 40's recovery claim conflicts with the detailed behaviour. G13, the C11 replay proof, was forwarded and has no patch in this plan. G19's closed rows, expanded terms and positive assertions for negative tests are present. `.plans/plans/go-s3-stack-client/decisions.md:48`, `.plans/plans/go-s3-stack-client/decisions.md:49`, `.plans/plans/go-s3-stack-client/decisions.md:54`, `.plans/plans/go-s3-stack-client/plan.md:257`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:47`.

2. **New mechanisms.** The lock does not span the resources it serializes and its cleanup cannot recover after forcible termination. The rejoin allowlist checks names and can move unrelated state. `mktemp -d` fixes the same-second archive collision under a trusted parent. Findings 01 through 03 cover the new mechanisms and their interactions.

3. **Rejections.** G9 has real evidence that D19 treats one operating-system user as a wave 1 convenience. The evidence supports leaving a second user's independent install outside scope. G12 has real evidence for team ownership and for a Client Link browser observation. The checked-in page and G4 show that the observation is still a necessary sprint dependency. Finding 05 carries that part forward. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:44`, `.plans/epics/go-rewrite/epic.md:121`, `.plans/plans/go-s3-client-link/plan.md:74`, `src/frontend/src/screens/service.tsx:410`.

4. **Writers and readers.** The dispatcher and token writer agree with the server settings, Store and Vault. Server Machines writes the issuer fields, token table and refusal codes. The installer reads those and the service path, and the Client plan owns `identity.json` and the state names. Both service templates, the operating-system service managers and the driver consume the selected state directory. The live fixture writer also has two later readers that need the bindings in finding 07. The Compose selector, sign-in helper and browser command still need named writers tied to their gates. Findings 04, 05, 07, 09 and 10 cover the gaps. `.plans/plans/go-s3-stack-client/plan.md:80`, `.plans/plans/go-s3-stack-client/plan.md:93`, `.plans/plans/go-s3-stack-client/plan.md:252`, `.plans/plans/go-s3-stack-client/plan.md:299`, `.plans/plans/go-s3-stack-client/plan.md:329`, `src/client/internal/state/folder.go:15`, `.plans/plans/go-s2-server-signin/plan.md:500`.

5. **Interacting rules.** The state and failure branches below show where the claimed preservation, exclusivity and post-crash evidence diverge. `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:233`, `.plans/plans/go-s3-stack-client/plan.md:238`, `.plans/plans/go-s3-stack-client/plan.md:244`, `.plans/plans/go-s3-stack-client/plan.md:379`.

| Starting state and event | Behaviour as written | Result |
|---|---|---|
| Existing service, override absent, valid same-origin identity, ordinary repeat. | Reads the installed state and skips join. | The simple repeat is covered. The health claim exceeds the process check. |
| Existing service, override points to another parent, ordinary repeat. | Selects the override and can join under a different lock. | One service can be overwritten with another enrollment. |
| Existing active service, rejoin, token fetch fails. | The refusal precedes stop and archive. | The old service and state stay in place. |
| Existing active service, rejoin, archive preparation or move fails after stop. | The join-failure restore branch is never reached. | The prior installation can stay stopped. |
| Existing inactive service, rejoin, move succeeds and join fails. | Restores the directory and starts any unit that exists. | A stopped or revoked machine is started again. |
| Existing active service, rejoin, join succeeds and activation fails. | The new directory and possibly the binary or unit are installed. | No recovery restores the old working set. |
| Rejoin is killed after the remote commit and before writing the identity file. | A forcible kill can leave the lock directory. The next run checks it first. | The unfinished-join message promised by PB1 is unreachable until lock recovery. |

6. **Round 1 blocker and major status.** G1 and G2's original blockers close. G4 and G5 remain open on recovery and filesystem custody. G7 remains open on rerun validation. G10 remains open on pre-login authentication and fixture custody. G11 and G14's driver-side changes are still requests, and G12 still needs its sprint dependency. G8's positional token and G20's remote Docker endpoint remain owner decisions.

7. **Lens coverage.** The patched command, filesystem changes, live credential handoff and sprint gates touch all seven lens questions. The answers below follow the lens numbering.

## Answers to the threat lens

1. **Trust boundaries.** The container command trusts its configured database password file and vault credential, delegates policy checks and uses the established signing identity. The installer accepts Compose selectors, a published address, a product response, environment paths and existing service files. The client verifies the token's normalized origin and server signing-key fingerprint. The product response proves readiness only. The unresolved remote Docker choice does not establish that token issuance occurred on the installer's host. Findings 02, 03, 08 and 09 identify the filesystem and stack limits. `.plans/plans/go-s3-stack-client/plan.md:154`, `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:231`, `.plans/plans/go-s3-client-link/plan.md:103`, `.plans/plans/go-s3-client-link/plan.md:106`.

2. **Credential inventory.** Server Machines draws the join secret and persists only its verifier. The command prints the token on standard output, the installer keeps it in a shell variable until use, and the client sends it in `Join`. The positional argument remains visible under the pending owner choice. The private client key is created locally in `machine.key` and retained in old state and archives. `identity.json` holds enrollment metadata and public keys. The command reads the protected database password and OpenBao AppRole files. OpenBao issues a leased token in memory and returns the signing key to the command, with only a public-key pin on the database side. The live test uses a configured test password and stores the session cookie in a private scratch jar. PostgreSQL holds the website cookie verifier, salt and password binding. The approved stream's connection token remains in memory. Findings 03 and 04 cover archive and test handling. `.plans/plans/go-s3-server-machines/plan.md:259`, `.plans/plans/go-s3-server-machines/plan.md:374`, `.plans/plans/go-s2-server-signin/plan.md:141`, `.plans/plans/go-s2-server-signin/plan.md:418`, `.plans/plans/go-s2-server-signin/plan.md:447`, `.plans/plans/go-s3-stack-client/plan.md:241`, `.plans/plans/go-s3-stack-client/plan.md:328`, `.plans/plans/go-s1-client-local/plan.md:130`.

3. **Credential endings.** Join tokens expire after 15 minutes and can be revoked. Same-key replay within the permitted window is an enrollment recovery path. Removing the machine revokes use of the registered key, while local removal and rejoin retain the file. Session sign-out, expiry and password replacement end the website credential. Scratch cleanup removes its local copy. The OpenBao batch token expires after 15 minutes. The database and AppRole files have the existing stack lifecycle, and server signing-key rotation remains outside wave 1. The connection token ends with the stream. Finding 03 covers disposal of retained archives. `.plans/plans/go-s3-server-machines/plan.md:258`, `.plans/plans/go-s3-server-machines/plan.md:263`, `.plans/plans/go-s2-server-signin/plan.md:181`, `.plans/plans/go-s2-server-signin/plan.md:338`, `.plans/plans/go-s2-server-signin/plan.md:451`, `.plans/plans/go-s3-stack-client/plan.md:416`.

4. **Encryption.** The design permits cleartext Hypertext Transfer Protocol version 2 (HTTP/2) on loopback and requires pinned Transport Layer Security (TLS) elsewhere. Wave 1 uses loopback and the private Compose proxy hop. The changed issuer rejects an HTTPS address without a pin. Issue 1633's public proxy termination and client certificate rules remain deferred. OpenBao stores the server signing key. Client state and archives contain an unencrypted private key whose protection depends on file permissions, directory custody and host disk protection. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:316`, `docs/issues.md:7520`, `.plans/plans/go-s3-server-machines/plan.md:258`, `.plans/plans/go-s2-stack-web/plan.md:203`, `src/client/internal/enroll/credential.go:63`.

5. **Dependencies and images.** Nothing found among new modules or images. The checkout pins the Go module dependencies and the PostgreSQL and OpenBao images. The Stack Web plan pins its base images. The local build trusts the operator's checkout and host tooling, including Go, Docker and Compose, and the service uses the installer-supplied search path. The plan adds no distribution artifact. `go.mod:3`, `deploy/compose.yaml:3`, `.plans/plans/go-s2-stack-web/decisions.md:21`, `.plans/plans/go-s3-stack-client/plan.md:225`, `.plans/plans/go-s3-stack-client/plan.md:237`, `.plans/plans/go-s3-stack-client/plan.md:442`.

6. **Integrity.** A valid join can register a waiting identity, and signed proof and owner approval govern the link's useful work. The owner session gates Machines mutations. The `hosts_server` field records issuance metadata and grants no additional access. The remaining local-account boundaries are writable paths and process-argument visibility. A plain product response does not prove stack identity. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:247`, `.plans/plans/go-s3-server-machines/plan.md:555`, `.plans/plans/go-s3-server-machines/plan.md:573`, `.plans/plans/go-s3-stack-client/plan.md:16`, `.plans/plans/go-s3-stack-client/plan.md:357`.

7. **Post-build checks.** The checks below require the built installer, service managers, browser and live stack. The missing decisions about lock scope, recovery and target identity are plan findings above.

## Post-build checks

| Check | Scenario and expected result | Evidence and responsible area |
|---|---|---|
| PB1. Rejoin recovery. | On a disposable enrolled machine, simulate supported failure and interruption points from archive through activation. A failed rejoin keeps or restores the original service state and identity. The next invocation handles any interrupted work and identifies a pending row if the server committed one. | Record redacted diagnostics, old and new state fingerprints, and service state before and after. Stack installer. |
| PB2. Lock and destination custody. | Exercise concurrent update and removal with different state overrides, then check refusal for fixtures with unrelated state or writable destination parents. Exactly one invocation owns the service at a time, and the refusal keeps the unrelated files in place. | Capture both exit codes, state identifiers, and ownership and mode checks. Stack installer on Linux and macOS. |
| PB3. Credentials in test and install output. | Use throwaway credentials during normal and failed operations with caller tracing enabled. Passwords and session cookies stay out of process arguments, traces and reported output. Verify the decided token-input policy and check temporary files and sign-out. | Retain pass or fail markers without the credentials. Stack and the driver test writer. |
| PB4. Boot and cleanup binding. | Restart a dedicated Linux fixture with a recorded approved machine. Prove that the same machine authenticated before first login. A resumed run with a mismatched fixture or service refuses any removal. | Keep the new boot identifier, the two boot-scoped times and the machine identifier. Stack live tests. |
| PB5. Repeat installation. | Exercise a matching canonical default-port origin, an interrupted enrollment and a changed server identity at the same origin. A valid repeat keeps the machine identifier. Unfinished or unverified enrollment receives the planned refusal or waiting result and retains the original installation. | Keep redacted status and before-and-after public fingerprints. Stack installer and Client Link. |
| PB6. Sprint target and browser. | Run the driver with a disposable Compose override, then copy the real Machines page command for that stack and observe the existing page show the new waiting row. Compose lifecycle and database checks stay within the same fixture. | Keep the project selection, 13 passing check lines and browser observation without the token. Sprint gate and website. |

## Grade

5 of 10. 0 blockers, 9 majors and 1 minor.
