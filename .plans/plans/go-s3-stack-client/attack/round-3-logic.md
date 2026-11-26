REVISE

The reboot gate can certify a connection made after the first login, because it compares the current connection with sessions that are still open when the operator runs the check.

## Findings

The review covers the patches from `90d8d4bb` to `263c34b2`. The plan and decision log matched the latter revision in the checkout at `e4a70245`. Unless a finding names a live probe, its evidence is from reading the cited files.

### logic-r3 01 · MAJOR · D16 · The reboot test can miss the first login and the first connection

The test compares the current machine's `connectedAt` with every current session of class `user`. The server overwrites `connected_at` on each admission with PostgreSQL's wall-clock time and projects it only while there is presence. The earlier decision still describes a monotonic comparison. See `.plans/plans/go-s3-stack-client/plan.md:292`, `.plans/plans/go-s3-stack-client/decisions.md:33`, `.plans/plans/go-s3-server-machines/plan.md:343` and `.plans/plans/go-s3-server-machines/plan.md:554`.

If a first login ends at 09:02, the client connects at 09:03, and a second login opens at 09:04, the test passes with only the second session visible. A step backward in the wall clock can also put a post-login connection before the login timestamp. Conversely, a valid pre-login connection followed by a reconnect after login fails, because the earlier connection is overwritten. The installed `loginctl` manual confirms that `list-sessions` lists current sessions. Checking for one session closes the vacuous-pass case from round 2.

Give the reboot fixture boot-scoped evidence that survives a logout and reconnect. Capture the recorded machine's first authenticated admission and the fixture user's first interactive login on the same monotonic clock, for example through structured host journal evidence tied to the new boot. Require a positive admission and login record for the same boot, treat a tie or a missing record as inconclusive, and check current online status separately. Mark the earlier monotonic rule as superseded where it specifies the unit-start event. Freeze cases for a closed earlier session, a post-login reconnect, and a wall-clock adjustment.

### logic-r3 02 · MAJOR · C6 · The sprint gates still use different Compose file lists

The installer now uses the launcher's selector by default. Gate G1 still forces the base file. Gate G4 names the round 1 change, which exports a default `COMPOSE_FILE` and drops `-f`. The round 2 handoff corrects that default, but assigns the direct override call to C4. The direct call is in C2. It explicitly passes only the base file and its inline override. See `.plans/plans/go-s3-stack-client/plan.md:96`, `.plans/plans/go-s3-stack-client/plan.md:316`, `.plans/plans/go-s3-stack-client/plan.md:319`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:42`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r2.md:46` and `deploy/test/checks.sh:378`.

With a two-file fixture configuration, G1 can run the command outside the intended fixture. C2 can recreate the fixture's server without the fixture's service definition, or fail against the base project. A disposable configuration probe with Docker Compose v5.5.0 confirmed that the file-list form used the fixture image while the explicit C2 form used the base image. This probe resolved configuration and started no containers. Port discovery has an ordering problem too. C1 calls `configure_web` before `up.sh`, so discovering the published port there fails on a cold stack with no exported port. See `deploy/test/checks.sh:355`.

Replace the stale handoff reference in G4 with one selector for G1, G2 and every driver call. Name C2 explicitly and append its inline file after the full effective file list, preserving the selector's settings-file and project resolution. Discover the actual published port in C1 after the launcher returns. Freeze a cold C1 case with the port only in the deployment settings file and an override case that verifies C2 mutates the same fixture G1 and G2 use.

### logic-r3 03 · MAJOR · D19 · A sequential rerun can still replace the installation's identity

The new rule says the installer joins once and a subsequent run keeps the identity. Step 6 still prefers an explicit override over the installed unit or agent. Step 13 joins into a selected folder that has neither marker, and step 15 rewrites the shared unit. The pre-existing round 2 report explicitly included the sequential case. See `.plans/plans/go-s3-stack-client/plan.md:25`, `.plans/plans/go-s3-stack-client/plan.md:199`, `.plans/plans/go-s3-stack-client/plan.md:206`, `.plans/plans/go-s3-stack-client/plan.md:208`, `.plans/plans/go-s3-stack-client/plan.md:254` and `.plans/plans/go-s3-stack-client/attack/round-2-threat.md:21`.

Install with one state folder and let the approved client go online. Run the installer again from a shell with a different, empty state override. The second command can join another machine and restart the shared service with that identity. The original record becomes offline and the new record waits for approval. The two runs are sequential and within the guide's supported operating procedure.

When an installed definition exists, require an explicit override to resolve to its existing state directory. Refuse a mismatch before obtaining a token, building or touching the service on either platform. Keep `--remove` independent of any state override, because that operation keeps the state and acts on the installed service. Freeze matching, missing and conflicting override cases.

### logic-r3 04 · MAJOR · C9 · A failed fixture phase can leave no way to resume cleanup

The fixture folder is reserved before installation, but the recovery record is written only after the waiting machine is observed. Both later phases require that record and the unit's state path. Cleanup removes the server record before it verifies the service exit or removes the unit. The server returns `not_found` for a new call to remove an absent machine. See `.plans/plans/go-s3-stack-client/plan.md:279`, `.plans/plans/go-s3-stack-client/plan.md:281`, `.plans/plans/go-s3-stack-client/plan.md:293` and `.plans/plans/go-s3-server-machines/plan.md:575`.

If enrollment succeeds and the first snapshot times out, the next default run refuses the occupied fixture folder and cleanup refuses the missing record. If a later cleanup removes the server row and then stops before local teardown, a retry has no defined response to the missing row. The record contains no stack selector or local identity binding, although later phases resolve Compose again from the current shell. The fresh state path closes the inherited-override failure from round 2, but verifies only a local path when resuming.

Write fixture intent before installation. Persist and verify the effective stack selector, the server identity, the user and the fixture state path. Once available, bind the machine identifier from the saved enrollment to the observed waiting row and the unit before either resumed phase acts. Separate the removal assertion from retryable teardown. Define cleanup both before and after record or unit creation, treat an already absent machine as a completed removal for a verified fixture, and preserve the fixture record until local teardown is confirmed. Freeze a first-snapshot timeout and a cleanup interruption after the server commits the removal.

### logic-r3 05 · MAJOR · C6 · The browser gate needs a runnable client and fresh state

Gate G5 says to paste in another folder. The installer puts its binary under the user's local bin folder, and the existing driver uses an absolute path to a separate binary with temporary state. The real page currently discards the token response address and prints a display label without a scheme. The round 2 handoff correctly requires the token's address, but G5's prerequisite does not say how the paste terminal finds the built binary or receives separate state. See `.plans/plans/go-s3-stack-client/plan.md:76`, `.plans/plans/go-s3-stack-client/plan.md:320`, `deploy/test/checks.sh:196`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r2.md:49`, `src/frontend/src/server/sync.ts:117`, `src/frontend/src/server/address.ts:45` and `.plans/plans/go-s3-client-link/plan.md:104`.

On a clean runner with the local bin folder absent from `PATH`, the pasted command cannot start. With that folder on `PATH`, a shell using the installed client's default identity can fail as already joined. Changing the shell's working directory has no effect on either condition. A website edit that only changes the spelling of the command also leaves it using a display label from the wrong source.

Have the orchestrator record a named website writer with permission for the token adapter and the command renderer. Make the response address a required input to the live command. Give G5 a disposable terminal with the tested client on `PATH` and a fresh absolute `LGTM_CLIENT_STATE_DIR`. Run the copy and paste against the already open page, correlate the waiting row with that token and clean up the gate's row and state.

### logic-r3 06 · MINOR · D16 · Linux can report a start before it invokes the client

Success is now defined as the service manager starting the client. The Linux template uses `Type=simple`, and the final check accepts the first `active` result. The installed systemd manual says this type treats the fork as startup, before the service process executes the binary. See `.plans/plans/go-s3-stack-client/plan.md:98`, `.plans/plans/go-s3-stack-client/plan.md:210` and `.plans/plans/go-s3-stack-client/plan.md:212`.

If the manager forks and then fails to execute the file, a status read during the window can report active and the installer can exit 0. This is a local startup gap even under the narrowed promise. Use `Type=exec` for Linux so a successful start confirms execution, or define success as the manager accepting the job and change the reported message accordingly. Keep the Machines page as the connection check. Add a service-manager test for a binary that cannot be executed.

### logic-r3 07 · MINOR · plan · Killing the installer does not isolate the pending join

PB1 kills the installer after the server has committed and before the client has written `identity.json`. The installer waits on a separate temporary client process, which owns the identity write. A local process probe confirmed that killing a shell while it waits does not stop the child from finishing. See `.plans/plans/go-s3-stack-client/plan.md:206`, `.plans/plans/go-s3-stack-client/plan.md:326` and `.plans/plans/go-s3-client-link/plan.md:106`.

If the child completes after the shell dies, a correctly implemented rerun can install that existing identity and PB1 incorrectly fails. Kill the child at an observed barrier after the server commit and before the identity write, then stop the installer and wait for both processes before the rerun. Add the separate case in which the child completes and the rerun reuses its identity with no second machine.

### logic-r3 08 · MINOR · R9 · The live fixture can still trace its sign-in material

The G4 handoff now requires tracing off. The separate G2 live script adopts the sign-in helper and feeds the password to `jq` on standard input, but specifies neither tracing off nor a check of that path. Its installer child turning tracing off protects only that child. See `.plans/plans/go-s3-stack-client/plan.md:194`, `.plans/plans/go-s3-stack-client/plan.md:280`, `.plans/plans/go-s3-stack-client/plan.md:319`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r2.md:48` and `deploy/test/checks.sh:193`.

A diagnostic run with Bash tracing prints an expanded helper argument or an assignment containing the password body. A synthetic-marker probe confirmed both trace lines even with `jq` reading standard input. The test password is disposable, which limits the impact. Turn tracing off in the live entry point before it reads sign-in material. Freeze a traced run with a synthetic password and require that its output contains neither the password nor the cookie.

### logic-r3 09 · MINOR · C6 · The order probe does not observe the macOS agent file

The changed stand-in snapshots the installed program and Linux unit at the moment of join. The macOS test checks final file contents and the order of manager calls after join. See `.plans/plans/go-s3-stack-client/plan.md:216`, `.plans/plans/go-s3-stack-client/plan.md:224` and `.plans/plans/go-s3-stack-client/plan.md:263`.

A macOS branch can rename the agent file before joining and wait until afterwards to bootstrap it. The prescribed tests pass, although a failed join can now leave an agent file to be loaded at the next login. Record whether the macOS agent exists when the client stand-in is called. Assert its absence during a successful first join and after a failed join.

### Finding metadata

The estimates multiply impact, fallout and probability. The evidence status applies to the plan defects. The implementation remains to be built.

| Finding | Category | Severity | Evidence status | Responsible area | Risk score and basis |
|---|---|---|---|---|---|
| 03 | Integrity and design | MAJOR | Confirmed by reading. | Stack installer. | 6 × 3 × 6 = 108, schedule. A specific shell setting can switch a working client's identity until the user repairs it. |
| 06 | Bug | MINOR | Confirmed by reading the plan and systemd manual. | Stack Linux installer. | 2 × 3 × 4 = 24, record. A rare execution fault can yield a misleading status. |
| 04 | Bug and design | MAJOR | Confirmed by reading. | Stack live fixture. | 2 × 1 × 4 = 8, record. A failed test phase needs manual fixture recovery. |
| 08 | Security | MINOR | Confirmed live with a synthetic marker and by reading the live fixture requirements. | Stack live fixture. | 2 × 1 × 2 = 4, record. A diagnostic run exposes disposable test material. |
| 07 | Bug | MINOR | Confirmed live for child process behavior and by reading PB1. | Stack installer test. | 1 × 1 × 2 = 2, record. A test-only interruption can reject a valid outcome. |
| 09 | Design | MINOR | Confirmed by reading. | Stack macOS installer test. | 1 × 1 × 2 = 2, record. The frozen test misses a platform-specific ordering mutation. |
| 01 | Design | MAJOR | Confirmed by reading the plan, server writer and local manual. | Stack reboot gate and server admission evidence. | Required gate. |
| 02 | Design | MAJOR | Confirmed live for selector precedence and by reading the gate. | Stack launcher and sprint-driver test writer. | Required gate. |
| 05 | Design | MAJOR | Confirmed by reading. | Website writer and sprint-gate owner. | Required gate. |

## Answers to the attack questions

### The seven questions of the task

1. The patch table below checks all fourteen register entries. Its rows also state whether the affected finding groups close.
2. The two unreviewed mechanisms are the join-once installer and the pre-login test. Findings 01, 03, 04 and 07 cover their ordering and recovery. The truth tables show the exceptional states.
3. The round 2 register contains no outcome of `rejected`. The folder move was removed and the remaining writable-parent issue was deferred to PB3. The serialization issue was deferred to the documented single-run scope. Those dispositions leave their named risks open. See `.plans/plans/go-s3-stack-client/attack/round-2-findings.md:13`, `.plans/plans/go-s3-stack-client/attack/round-2-findings.md:21` and `.plans/plans/go-s3-stack-client/plan.md:328`.
4. The changed writers and readers appear in the names table below. The open handoffs are the Compose selector, historical boot evidence, fixture identity, installed state on rerun, and the website token address.
5. The tables below show the interactions between the installer override and its join-once rule, between current presence and pre-login timing, and between Compose's default and explicit selectors. The conflicting cells are identified there.
6. Round 2 reported zero blockers. The Compose selector, reboot proof and fixture ownership majors remain partly open. The sequential form of the conflicting override remains open even under the one-run rule. The G10 driver prerequisite closes its named omission, while the live script still needs the trace guard. G9 now prevents an omitted browser check, while its runnable fixture is underspecified. See findings 01 to 05 and 08.
7. The patches touch all eight logic questions. The numbered lens answers below give concrete examples for ordering, failure, zero values, concurrent actors, mutations, missed implementations and state shape.

### Each patch in the round 2 register

| Patch | Result |
|---|---|
| 1. Overview | The first-install description and platform distinction are clear. The separate macOS login gate agrees with the installer plan. The epic's broad restart wording remains an open owner choice identified by the fold. See `.plans/epics/go-rewrite/epic.md:33` and `.foundry/go-rewrite/results/fold-go-s3-stack-client-r2.md:37`. |
| 2. Key decisions | Removing automatic rejoin, the lock and the folder move removes the original G1 and G3 failure mechanism and makes the G2 concurrency exclusion explicit. It leaves the sequential override case. The explicit file option fixes the default installer selector in G4. |
| 3. Workstreams | The update and remove promises match the ordinary single-state path. The reboot proof is still incomplete. Findings 01 and 03 apply. |
| 4. Rollout order | Requiring the merged website and rebuilt image makes the browser and deployed checks visible dependencies. G5 still needs the fresh paste fixture in finding 05. |
| 5. Public names | The installer, unit and token command signatures align with the patched ordinary path. The removed archive and lock have no remaining operative readers in this plan. The listed state readers lack the mismatch rule in finding 03. |
| 6. Service, address and exits | G7 closes the port-80 string comparison. G6 removes the connection promise and retains the local startup promise, whose Linux evidence has finding 06. The actual token normalizes the default port away, so agreement is by origin. The client compares normalized origins. See `.plans/plans/go-s3-server-machines/plan.md:258` and `.plans/plans/go-s3-client-link/plan.md:103`. |
| 7. Work item 1 | G12 closes. The sole option is `HostsServer` and the issuer owns the address. G14 closes the missing database prerequisite for this item's test command. A fresh disposable database is supplied by the checked-in wrapper. See `.plans/plans/go-s3-server-machines/plan.md:150` and `deploy/test/with-postgres.sh:112`. |
| 8. Workstream criteria and tasks | The failed-join criterion is observable for its listed tests. The second-run criterion is only demonstrated with an unchanged or omitted override. Finding 03 covers the conflicting input. |
| 9. Linux item and stand-ins | G11 closes the missing Linux rename observation. Its installation order assertions and independent restart fields are observable. G13 closes the build-tool prerequisite for removal. G15 now specifies three distinct outputs. Its refusal test alone still accepts a malformed nonempty line, so verify the stand-in's empty output separately when freezing it. |
| 10. macOS behavior | The launch agent reads the installed state, and the new startup wait delegates to launchd as intended. The common override problem and the agent's early-write gap remain. See findings 03 and 09. |
| 11. Live item and guide | The explicit new state path closes G5's inherited-state adoption case, and the later unit check covers a simple replacement with a different path. The fixture still needs a recoverable record. G8 now observes authenticated presence but loses the history needed for the reboot claim. See findings 01 and 04. |
| 12. Workstream risk table | The lingering and routing mitigations depend on the reboot proof and common selector. The positional-token exposure remains a recorded owner choice. The manual rejoin path is now distinct from a plain update, subject to finding 03. |
| 13. Sprint gate, post-build checks and exclusions | G14 supplies C12's test database. G10 makes the driver password change a prerequisite, with a separate live-script gap. G9 requires a real paste. G4, G5 and G8 retain findings 01, 02, 04 and 05. PB1 has finding 07. PB3 records the still-deferred writable-parent part of G3. |
| 14. Traceability, guardrails and attack log | The amended traceability points to the right decisions and gates. The decision log supersedes the archive and health-watch rows. Its earlier boot rule still needs a supersession marker as part of finding 01. No stale installer caller requires the former `--rejoin` interface in the patched plan. |

### Names with writers and readers

| Changed name or value | Writer and readers |
|---|---|
| The removed `--rejoin` mode, archive and installer lock. | The installer no longer writes the archive or lock. The guide, live fixture and usage tests use the reduced command. The old decision rows are marked superseded. The independent client state lock remains client-owned. |
| `LGTM_CLIENT_STATE_DIR`, `machine.key` and `identity.json`. | The installer, Linux unit, macOS agent, sprint driver and live test set the state directory for their clients. The Client team writes the files. The installer reads their presence and its service's state setting on rerun. The driver also edits the fixture identity for C10. The fixture still needs a validated machine-identity read and the installer needs the conflicting-override outcome. See `.plans/plans/go-s3-stack-client/plan.md:213` and `.plans/plans/go-s3-client-link/plan.md:106`. |
| The Compose selector and published port. | The launcher defines the default selector. The installed Compose command resolves it, and the installer, live fixture, token smoke check and sprint driver consume it. Include the C2 inline override path and the cold C1 caller in the handoff. |
| `Tokens.Create` and the token address. | The Server team supplies the issuer from settings. Its token logic writes the normalized address and the optional mark. The command and website token endpoint call it, the installer gets the emitted token, and the client compares its normalized origin. G5 also needs the website adapter and renderer as explicit readers of the response address. |
| `connectedAt`, session timestamps and the fixture record. | The server's current-stream admission writes presence, the machine projection returns it, and the live gate reads it. The login manager supplies current session properties. The default phase writes the fixture record, and reboot and cleanup read it. No writer is named for historical admission or first-login evidence. Record and check the stack and local identity before authorizing later use. |

### Installer state and action

| State at entry | Plain install or rerun in the patch | Removal in the patch |
|---|---|---|
| No unit, no key and no identity. | The installer builds and joins into the selected state. | The installer keeps state and attempts local removal. |
| No unit, key only. | The installer refuses and tells the user how to recover. The server may or may not have a waiting row. | The installer keeps the key. |
| No unit, saved identity and key. | The installer skips join and can finish a prior installation interrupted after enrollment. | The installer keeps the identity. |
| Installed unit and no override, or an override matching its valid state. | The installer updates the program and restarts the service with the same identity. | The installer keeps the enrolled state and removes the service. |
| Installed unit and a different empty or enrolled state override. | The installer can mint another identity or switch to an existing one. This contradicts the plain-update promise. | The installer stops the same shared service but still validates an unrelated override before entering the removal branch. |
| Identity file with missing key, or an invalid identity. | Presence skips join. The client validates the record and key on run and can exit 1. The installer promises local startup, so its success can coexist with a later runtime failure. | The installer keeps the state. |

### Reboot evidence by time and session state

| Boot and login evidence | Current presence | Result under the patch |
|---|---|---|
| Same boot, or no session of class `user` visible. | Any. | The check fails, which closes the previous vacuous pass. |
| New boot and genuine first session still open. | Current connection is earlier than the session on a stable clock. | The check passes with evidence for the stated ordering. |
| New boot and the first session has closed. | Current connection fell between the earlier closed login and the visible login. | The check passes incorrectly. |
| New boot with a first session open. | A pre-login connection was replaced by a post-login reconnect. | The check fails despite the earlier authenticated connection. |
| New boot with a first session open. | The wall clock stepped backward before a post-login admission. | The check can pass incorrectly. |
| New boot with a session timestamp equal to the admission timestamp, or a missing timestamp. | Any. | Equality fails the strict comparison. The missing-value conversion and diagnostic are undefined. |

### Compose selection by caller

| File selection at entry | Installer and intended launcher | G1 and the C2 direct call |
|---|---|---|
| `COMPOSE_FILE` unset or empty. | They supply the deployment file explicitly. | They use the base file. G4's reference to the older default-export behavior leaves an inconsistent instruction for its test writer. |
| `COMPOSE_FILE` contains a fixture file list. | They honor the complete list. | G1 overrides the list. C2 overrides it and appends only its own inline file. |

### The eight questions of the logic lens

1. The boot identifier is used only for equality. The proposed ordering uses wall-clock times for the current admission and visible logins, so ties fail and a reconnect or clock step invalidates the intended inference. The gate introduces no wrapping counter. Its existing restart delta is meaningful only within one service-manager lifetime.
2. A refused join keeps a pending key if one was written, and a retry after a completed identity reuses it. The manual repair for the pending case is explicit. Fixture cleanup after a committed removal or an interrupted first observation has no complete recovery rule. Failed observation also needs a distinct outcome in the boot test.
3. The three truth tables identify the conflicting override, missing boot history and inconsistent Compose selector. Their failure cases follow from changed rules and readers.
4. An empty override falls back to the installed setting or default. An empty `COMPOSE_FILE` selects explicit `-f`. An empty port mapping refuses installation. An absent `connectedAt` has no stated failure rule for the comparison. No new enumeration is introduced.
5. Two installers can still write the same service paths and are explicitly outside the one-run scope. The client child and its installer parent can proceed independently during an interruption. The session manager can open or close a login while the gate samples the machine. The server can replace the sampled stream with a new one.
6. Item 1's forced false mark and extra standard-error token each contradict explicit assertions. Item 2's printed token and early Linux binary or unit installation each contradict its explicit assertions. Writing a macOS agent early still passes the listed checks. A token command could also skip emitting its promised diagnostic text on output-write failure while writing some other one-line diagnostic and passing that assertion.
7. Item 1 can leave a usable undisclosed token in the database after its output fails and still pass its tests. Item 2 can honor a different override on update. Item 3 can write its agent early. Item 4 can implement only the default, after-reboot and cleanup happy paths, with no resumed cleanup after an interruption, and still satisfy its listed acceptance and live assertions.
8. Binding one installed service to its state directory prevents the sequential second identity. A fixture record that binds the stack and local enrollment before cleanup prevents a directory-name match from authorizing an unrelated mutation. Boot-scoped historical evidence prevents a current snapshot from standing in for a past event.

## Post-build checks

| Scenario | Expected result |
|---|---|
| A normal installer rerun follows an initial Linux or macOS installation with a custom absolute state directory. The shell has no override on rerun. | The same machine identifier, key and identity survive. The manager loads the same state directory and no new join token is issued. |
| The server commits a join and the join child is stopped at the pre-write barrier. In a separate run, the child finishes while the installer is stopped. | The first case leaves a key, no identity and one waiting machine, and the next run refuses with its repair message. The second run completes installation of the existing identity with one machine total. |
| The selected stack returns after a host reboot with no interactive session. The client later reconnects after the operator logs in. | The new boot's preserved evidence shows the recorded machine's first authenticated admission before the first login, and the post-login stream shows the same machine online. |
| A portable service test places a valid unit and then makes its executable unavailable before asking the manager to start it. | The Linux local start check reports the execution failure. The Mac gate separately observes the real launch agent through restart, login and removal. |

## Grade

5 of 10. There are 0 blockers, 5 majors and 4 minors.
