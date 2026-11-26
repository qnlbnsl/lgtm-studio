REVISE

An interrupted enrollment can still strand the live fixture, because cleanup requires the unit that the installer writes only after enrollment.

## Findings

The review covers the patch from `263c34b2` to `94dfe2c7`. Each finding identifies evidence from the plan or checkout. The shell and Compose probes used dummy values and started no services.

### logic-r4 01 — MAJOR — C9 — Cleanup refuses reachable installer states

**Evidence by reading.** Both `--after-reboot` and `--cleanup` require an installed unit with the fixture state path. The installer publishes the unit after the client finishes enrollment. The client saves `identity.json` after the server replies, and the server can have committed a waiting machine before then. Cleanup accepts `not_found`, then unconditionally runs the service-exit assertion. See `.plans/plans/go-s3-stack-client/plan.md:205`, `.plans/plans/go-s3-stack-client/plan.md:208`, `.plans/plans/go-s3-stack-client/plan.md:280`, `.plans/plans/go-s3-stack-client/plan.md:292`, `.plans/plans/go-s3-client-link/plan.md:104` and `.plans/plans/go-s3-server-machines/plan.md:575`.

**Failure.** Kill the installer after the client saves its identity and before the unit is written. The occupied fixture blocks the default run, and the missing unit blocks cleanup despite the known machine identifier. Kill it before the identity write and a committed waiting row has no cleanup identifier, even after the unit prerequisite is relaxed. A later retry can also reach the exit assertion with both the server row and local unit already gone. The record stores `COMPOSE_FILE` but neither resumed part is required to use it, so a new shell can fail the address check before getting to any of these cases.

**Fix.** Keep the unit and new-boot prerequisites for `--after-reboot`. Give cleanup its own state rules. Record fixture intent before any install, bind it to the effective Compose selection and require that selection on resume. Accept an absent unit after a partial install or partial teardown, and refuse a present unit that points outside the fixture. Use the machine identifier when available. When the key exists without an identity, prescribe a safe fixture-token correlation or an explicit manual reconciliation that keeps the fixture and reports the unresolved row. Separate the clean-exit assertion for a running service from repeatable teardown, and delete the fixture only after its remote and local cleanup are accounted for. Cover the stop before the first record, the two enrollment boundaries, and a retry after each removal.

### logic-r4 02 — MAJOR — C6 — Gate G4 can use two settings sources and two ports

**Evidence by reading.** Gate G4 adds the password-removal overlay after the same list of Compose files and moves port discovery in C1 after launch. The checked-in driver resolves the address independently for later checks, and each check runs inside a command substitution. See `.plans/plans/go-s3-stack-client/plan.md:96`, `.plans/plans/go-s3-stack-client/plan.md:318`, `deploy/test/checks.sh:32`, `deploy/test/checks.sh:187`, `deploy/test/checks.sh:355`, `deploy/test/checks.sh:370`, `deploy/test/checks.sh:654` and `deploy/test/checks.sh:659`.

**Failure confirmed live for selection and shell scope.** With Compose v5.5.0, a two-file `COMPOSE_FILE` fixture resolved a dummy setting from the working directory. Supplying those same files with `-f` and appending `-f -` kept the fixture image and removed the intended field, but selected the dummy setting from the deployment directory. A file-list match alone can therefore recreate the server with different settings. Separately, with a nondefault port set only in the deployment settings file, C1 may pass while C2 and later checks still dial the default port. A shell probe confirmed that the value discovered inside C1 is unavailable to C2.

**Fix.** Require identical file selection, settings-file resolution and project identity on both sides of the C2 overlay. When `COMPOSE_FILE` is in use, write the overlay to a private temporary file and append it using the effective path separator while preserving the environment-based invocation. Keep the explicit base-file invocation when the selector is unset or empty. Make the common address helper resolve the published port for every check that uses the website, and call it after launch in C1 and after recreation in C13. Verify both a cold launch with a settings-only port and a two-file fixture with different dummy settings in the two directories.

### logic-r4 03 — MAJOR — D19 — An installed service can enroll again when its saved folder is empty

**Evidence by reading.** Step 6 now treats an installed unit or agent as the authority for the state folder. Step 13 still uses the absence of `identity.json` as the sole entry condition for enrollment after the key-only refusal. Decision 52 gives the installed definition precedence, while active wording in decision 32 gives the override precedence. See `.plans/plans/go-s3-stack-client/plan.md:25`, `.plans/plans/go-s3-stack-client/plan.md:199`, `.plans/plans/go-s3-stack-client/plan.md:204`, `.plans/plans/go-s3-stack-client/plan.md:206`, `.plans/plans/go-s3-stack-client/plan.md:254`, `.plans/plans/go-s3-stack-client/decisions.md:41` and `.plans/plans/go-s3-stack-client/decisions.md:61`.

**Failure.** A user leaves the unit installed but accidentally moves its state folder aside, or the filesystem that holds the override is temporarily absent. The unit still identifies the installation, and the caller passes no conflicting override. With neither state marker visible, the rerun enrolls a fresh machine and restarts the existing unit against it. The original approved machine remains registered and the replacement waits for approval. The new override tests leave this case open on both platforms.

**Fix.** When an installed definition exists, require a readable state-folder setting and an existing identity before any build or token request. Treat missing or unreadable state as a refusal and direct the user to restore the saved folder or follow the documented remove-and-install path. Keep `--remove` independent of both the override and state-folder parsing. Allow a fresh join only when no installed definition exists and both enrollment markers are absent. Mark the precedence in decision 32 as superseded by decision 52, and freeze the missing-state case for both service managers.

### logic-r4 04 — MAJOR — plan — The frozen rerun test forbids a command that the installer must run

**Evidence by reading.** The Linux steps require `systemctl --user show-environment` at step 4 and check for the conflicting override at step 6. The patched fourth run of `rerun_uses_state_folder_of_installed_unit` requires an empty set of `systemctl` calls. Every shim call is logged. See `.plans/plans/go-s3-stack-client/plan.md:197`, `.plans/plans/go-s3-stack-client/plan.md:199`, `.plans/plans/go-s3-stack-client/plan.md:215` and `.plans/plans/go-s3-stack-client/plan.md:239`.

**Failure.** A working Linux user manager answers the step 4 probe on a rerun with a conflicting override. Step 6 then correctly refuses it without changing state, but the required read-only probe makes the acceptance test fail. An implementation that skips step 4 in this case can pass the test while departing from the stated order.

**Fix.** Keep the preflight order and allow the read-only manager probe in the test. Assert that no build, token request, join or mutating service-manager call occurs, and assert that the program, unit and both state folders are unchanged. Give the macOS agent an equivalent rerun test, allowing its read-only domain probe.

### logic-r4 05 — MAJOR — R9 — Gate G1 discards the stream it claims to check

**Evidence by reading.** Gate G1 requires standard error to be discarded and empty. The installer test induces a failure on every Compose call, so it stops at the first port lookup and never tests the token-issuance call's redirection. The server-command unit test already owns the command-level empty-stream assertion. See `.plans/plans/go-s3-stack-client/plan.md:141`, `.plans/plans/go-s3-stack-client/plan.md:202`, `.plans/plans/go-s3-stack-client/plan.md:206`, `.plans/plans/go-s3-stack-client/plan.md:217`, `.plans/plans/go-s3-stack-client/plan.md:229` and `.plans/plans/go-s3-stack-client/plan.md:315`.

**Failure confirmed live for redirection.** A shell command that printed correctly shaped standard output, emitted a marker on standard error and exited 0 produced the same retained evidence as a quiet command after redirecting standard error to the null device. G1 can therefore pass on a noisy deployed command. An installer that redirects only the port lookup also passes the new Compose failure test and can then surface a diagnostic on token issuance.

**Fix.** Keep raw Compose diagnostics out of gate output. If G1 must prove the deployed stream is empty, capture it in a private temporary file, test whether the file contains any bytes, report only the result and remove the file. Otherwise define G1 to observe status and token shape, and leave the empty-stream assertion explicitly with the server-command test. Add a shim case that makes the port lookup succeed and token issuance fail with a dummy diagnostic, and require the fixed refusal without the marker.

## Answers to the attack questions

### The seven assignment questions

1. The patch table covers every patch in the register, including its effect on the original failure and on interacting rules.
2. The marked mechanism at `.plans/plans/go-s3-stack-client/plan.md:25` removes the installer-owned archive, lock, collision and rollback operations. The service/state table shows the remaining sequential failure and the intended partial-write repair. Concurrent runs are expressly outside wave 1. Findings 01 and 03 cover the remaining reachable interruption and missing-state cases.
3. Nothing found. The register contains no rejected finding. Group G6 was carried forward as a post-build check under the recorded owner ruling.
4. The name table traces the writers and readers. The important gaps are the recorded Compose value with no prescribed resume reader, the old precedence in decision 32, and incomplete handling of the website token's empty sample address.
5. The tables show contradictions at cleanup with no unit, installation with an existing definition and no identity, a conflicting override after a manager probe, C2's settings selection, and G1's discarded diagnostic.
6. The round 3 major about fixture recovery remains open. The Compose major remains open in G4, and the installed-folder major is addressed for a differing override but still fails when the installed folder is missing its state. The original observer and website prerequisite changes address their central round 3 cases.
7. The patches touch all eight logic questions. The answers below limit the review to the changed behavior and its direct interactions.

### Each patch in the round 3 register

| Patch | Result |
|---|---|
| 1. The installation workstream summary. | The observer replaces a retrospective timestamp comparison. No ordering defect was found in a controlled boot with an observer outside the fixture account. |
| 2. Linux state selection, Compose diagnostics, the unit and stand-ins. | Installed-folder precedence fixes a valid enrollment with a conflicting override. Findings 03 and 04 cover the remaining state and test contradictions. `Type=exec` closes the successful-fork window, as confirmed by the installed systemd manual. Diagnostic suppression is specified for both calls, with a test gap in finding 05. |
| 3. The macOS state reader and installation-order test. | Reading the agent environment gives the same intended precedence. Including the agent file in the join-time snapshot closes the early-file hole. The missing-state case also affects this branch, and the only rerun test exercises Linux. |
| 4. The live fixture and guide. | Early intent and the client-written identifier improve the happy path. Finding 01 covers earlier stops and retries. Tracing is disabled and the sign-in input path is specified. The guide now accounts for the old server row and moved local key during a manual reinstall. |
| 5. The reboot risk row. | The risk row points to the observer that actually proves a pre-login connection. The later automated check proves only a new boot and present connectivity. |
| 6. Gates G1, G2, G4 and G5. | G1 and G4 have findings 05 and 02. G2 closes the timestamp failure but retains the fixture failure in finding 01. G4 names the credential prerequisites directly. G5 assigns both website files through an explicit grant prerequisite and supplies a fresh state folder. The terminal for the real probe must be on the stack host, because the wave 1 token address is loopback. |
| 7. The three post-build checks. | PB1 distinguishes an interrupted client from a completed client. A page update alone cannot stop the first case deterministically, so the post-build check needs a barrier at the identity write. PB2 adds the traced live entry point. PB3 records the delegated host-custody issue under the rule in the register. |
| 8. Traceability, decisions and attack log. | The new reboot test is named and decisions 24 and 49 explicitly defer to 51. Decision 32's active wording remains inconsistent with 52. No major was found in the attack-log change or the renaming of the traceability references. |

### Changed names and ownership

| Name or value | Writers and readers |
|---|---|
| Installed `LGTM_CLIENT_STATE_DIR`. | The Stack installer writes the unit or agent and its later run reads it. Systemd or launchd passes it to the client. The live fixture and Linux stand-in read it. Decision 32 also needs to defer to 52. |
| `fixture`, boot identifier, address, `COMPOSE_FILE` and `machineId`. | The live driver writes intent and the client writes `identity.json`. The two resumed parts read intent and the identity. The plan gives those readers no rule for applying the recorded Compose value and no cleanup branch for a committed join without the identifier. |
| `live_online_after_reboot` and the manual pre-login observation. | The live driver emits the renamed test result. Gate G2 and traceability consume it. The observer reads the Machines page using a different account from the fixture user. The renamed result proves current connectivity and a changed boot. |
| `CreateClientTokenResponse.address` and website `ClientToken`. | The Go issuer and service write the normalized address. The assigned website writer must update the adapter's type and returned object in `src/frontend/src/server/sync.ts:117` and the dialog in `src/frontend/src/screens/service.tsx:369` and `src/frontend/src/screens/service.tsx:410`. The sample writer at `src/mock-server/src/engine.ts:182` intentionally omits the address, as `src/contracts/proto/lgtm/v1/clients.proto:121` allows. The dialog needs to keep its separate sample example. |
| Compose selection, discovered address and password jars. | The launcher and installer select the project, and G1 and both test drivers consume that selection. In G4, C1's port value dies with its subshell. The sign-in helper and changed-password check write separate jars. The cleanup trap must account for both, including `deploy/test/checks.sh:422`, whose jar is outside the primary variable read at `deploy/test/checks.sh:138`. |

### Installer and live-cleanup truth tables

| Installed definition | State and invocation | Prescribed path and gap |
|---|---|---|
| Absent | No key or identity, fresh install. | The installer may join. |
| Absent or present | Key only, install. | The installer refuses an unfinished join and points to manual repair. |
| Absent | Identity present after interrupted installation. | The installer can finish installation using the saved identity. |
| Present | Identity present and override absent, empty or equal. | The installer updates the program and restarts with the installed identity. |
| Present | Override differs. | The installer refuses. The Linux test forbids the read-only manager call required earlier in the plan. |
| Present | Both state markers absent with no conflicting override. | The installer joins again, contradicting the one-installation rule. |
| Present or absent | Removal with a conflicting override. | The override is ignored. Removal still needs an explicit path around parsing a missing or malformed state setting. |

| Fixture state | Intended cleanup | As written |
|---|---|---|
| Folder reserved, intent missing, no install started. | Release the reserved folder safely. | The record prerequisite forbids cleanup. |
| Intent exists, no installed unit, no identity. | Clean locally, and reconcile any waiting row that the interrupted join created. | The unit prerequisite forbids cleanup, and there is no server-row correlation. |
| Intent and identity exist, no installed unit. | Remove the known row and clean locally. | The unit prerequisite forbids cleanup. |
| Intent, identity and matching running unit exist. | Remove the row and prove the service exits cleanly. | This path is defined. |
| Row already removed, matching unit remains. | Continue local teardown. | `not_found` is accepted. The clean-exit assertion still assumes evidence from a running service. |
| Unit already removed, fixture remains. | Finish repeatable teardown. | The unit prerequisite forbids cleanup. |
| Present unit names another folder. | Refuse to act on that installation. | This refusal is defined and should remain. |

### Boot evidence, Compose and gate diagnostics

| Condition | Required result | Review |
|---|---|---|
| Same boot, machine online. | Automated reboot check fails. | Defined by boot-identifier inequality. |
| New boot, fixture user has yet to log in, observer sees recorded machine online. | Manual pre-login evidence passes. | Defined without session timestamps. |
| New boot, machine first goes online after fixture login. | The automated online check may pass and the manual gate fails. | Both observations are required. |
| New boot, observer sees another online machine. | Manual gate fails for the recorded machine. | The operator must match the fixture identity. |
| `COMPOSE_FILE` empty or unset. | Use the explicit deployment base file and add the C2 overlay after it. | File selection is defined. |
| `COMPOSE_FILE` nonempty. | Keep its files, effective settings and project when applying the C2 overlay. | Merely repeating its files with explicit flags can change settings, as demonstrated in finding 02. |
| G1 returns 0 with one valid line, with or without bytes on standard error. | The quiet case should pass the empty-stream assertion. | Both cases look the same after discarding the stream. |

### The eight logic questions

1. The new reboot rule compares boot identifiers for equality and relies on observation order. It removes wall-clock ties and repeated-connection ambiguity. An unchanged boot identifier fails. No new counter, sequence or expiry is introduced by these patches.
2. A mismatched override refuses and a Compose error returns a fixed diagnostic. Fixture recovery remains undefined at partial enrollment, loss of the saved selector and a retry after local removal. See findings 01 and 03.
3. The tables name the empty and contradictory cases. The strongest contradictions are cleanup with no unit, C2's environment selection and the quiet G1 assertion.
4. An empty override falls back to the installed setting or default. A missing identity with no key is safe for a first install and unsafe for an existing unit. An empty token address is valid for the sample contract and needs its existing sample branch. No enumeration changed.
5. The client writes the state while its parent controls service installation, and the server commits independently before the client publishes the identity. The fixture observes both. Client-side locking does not make the shell's service installation atomic. The plan explicitly leaves concurrent installer runs outside wave 1.
6. Item 1's unchanged mark and error-stream mutations target direct assertions. Item 2's token print and early Linux installation mutations target the hash/output and join-time snapshot assertions. Redirecting only the first Compose call still passes the new failure test, and changing both the unit template and its rendering back to `Type=simple` can pass the shim comparisons.
7. For item 1, the patch leaves its component tests unchanged, but G1 ignores a deployed diagnostic. Item 2 can re-enroll from a missing installed state folder. Item 3 can give a differing override precedence on rerun because no Mac rerun test exercises it. Item 4 can reject cleanup when the unit is missing and still pass its own no-flag acceptance and an uninterrupted live run.
8. Give the installed definition an explicit update-only branch, and give fixture teardown a retryable path with independent local and server outcomes. Each change removes a wrong state reachable under the patched rules.

## Post-build checks

- **Startup after execution.** Run a disposable Linux install whose service executable cannot run. The real service manager must refuse the start, and the installer must return failure. Retain the returned status and unit state.
- **Interrupted enrollment.** Use a controlled barrier immediately before the identity write, after the test has established that the fixture's waiting row exists. Interrupt the client and wait for it to exit. Verify the key-only refusal and the fixture cleanup outcome. Repeat with a published identity and no unit, and verify that a rerun reuses the identity if the fixture is kept.
- **Trace and session cleanup.** Run the two drivers with tracing enabled and synthetic credentials. An induced failure after a sign-in, including the changed-password sign-in, must leave no credential marker in the retained output and no usable session from the run. Verify both session jars and the server response.
- **Website command and observer.** On the stack host, use a nondefault port and the installed binary in a fresh private state folder. The pasted command must yield the waiting row for that dialog's token on the already-open page. The sample still shows its example. Before the fixture user's first login on a fresh boot, observe the fixture machine from a different local account or another account-independent route to the loopback site.

## Grade

6 out of 10. The review found 0 blockers, 5 majors and 0 minors.
