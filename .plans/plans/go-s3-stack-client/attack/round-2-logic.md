REVISE

A killed rejoin can leave the old service stopped, its identity in an archive, and a stale lock that prevents the next run from recovering either.

## Findings

### logic-r2 01 — MAJOR, D16 — Rejoin has no recoverable commit point

Evidence by reading. The plan stops the service and moves the old state before calling the new client. Cleanup handles a returned join failure only. Installation and service activation happen in separate steps after a successful join. The crash check assumes that the next run reaches the unfinished-join refusal, although an uncatchable interruption leaves the installer lock behind. `.plans/plans/go-s3-stack-client/plan.md:229`, `.plans/plans/go-s3-stack-client/plan.md:238`, `.plans/plans/go-s3-stack-client/plan.md:379`, `.plans/plans/go-s3-stack-client/decisions.md:49`.

If the installer is killed after the archive rename, the old service stays stopped and its configured state path may be empty. Even after manual lock removal, a plain rerun can start an unrelated fresh join. If the new join returns an error after the server committed the waiting row, the rollback deletes the new key and leaves no local record for identifying the abandoned enrollment. Repeating that failure can accumulate waiting rows. A failed unit reload after a successful join also leaves no rule for which enrollment and installation to recover. The Client plan explicitly allows an error after the key or identity is placed. The exit trap leaving a directory behind after a forced kill was confirmed live in a temporary probe. `.plans/plans/go-s3-client-link/plan.md:104`, `.plans/plans/go-s3-client-link/plan.md:262`.

Stage the new enrollment in a private sibling directory while the old service is running. Record the selected state, staged state, archive, old service state and cutover phase before the first change to the working installation. Under the installer lock, define how a rerun detects an abandoned operation and either restores the old files and prior service state or finishes the same enrollment. Keep the staged key and identify a possibly committed waiting row when the join outcome is uncertain. Test stops and failures before the archive, after the archive, after the server commits and during activation. This recovery rule belongs in the plan before the build.

### logic-r2 02 — MAJOR, D19 — The lock follows the state folder while the service is shared

Evidence by reading. The installer selects the state folder before locking its parent. The binary, unit or agent, and service name are shared by every invocation for the same user. The Go client's lock covers one state folder and is independent of the installer lock. `.plans/plans/go-s3-stack-client/plan.md:84`, `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:242`, `src/client/internal/state/folder.go:94`.

One user can start two supported invocations with overrides under different parents. Both acquire the lock, both can join, and both replace the same binary and unit. The first process can report success while the second service definition wins. Separate parent directories admitting both locks was confirmed live with two directory creations. A plain invocation can also read a stale unit before locking, while a concurrent invocation changes it.

Lock a fixed location for the per-user installation before reading the unit, agent or overrides. For an installed service, give an override pointing elsewhere an explicit outcome. Refuse it for a plain rerun or removal and require an explicit migration policy for rejoin. Require successful service stop before moving state and coordinate the move with the client's folder lock, including the foreground-client case. Add a two-invocation test with different state parents and a test with the client's folder lock held.

### logic-r2 03 — MAJOR, D19 — Allowed entry names do not prove directory ownership

Evidence by reading. The sole archive precondition is a non-symlink directory whose children have allowed names. The client checks directory type, ownership and permissions before use. Its repository path defaults to a child named `repositories`. `.plans/plans/go-s3-stack-client/plan.md:233`, `.plans/plans/go-s3-stack-client/plan.md:283`, `src/client/internal/state/folder.go:73`, `src/client/internal/config/config.go:45`.

A user can accidentally point the override at a directory containing only an unrelated `repositories` directory. The installer accepts it, moves the user's projects, and enrolls a new client at their former path. A throwaway directory with that exact shape was moved by the proposed archive operation in a temporary probe. The current test uses `notes.txt`, which misses this case.

Resolve and validate the target before allocating an archive. Require positive evidence that the installer or client owns it, such as an installation record bound to the resolved directory or a locally validated existing credential for a supported legacy folder. Define the separate cases for an absent directory, an empty directory, and a recognized key-only join. Refuse unsafe ancestors and ambiguous paths before stopping the service. Test the unrelated repository directory and a path alias.

### logic-r2 04 — MAJOR, C6 — Compose defaults can select two different stacks

Evidence by reading. The installer sets `COMPOSE_FILE` at the repository root and drops the explicit file option. The launcher plan keeps `-f` when the caller supplied no file override, and its guide relies on the same option. The current rollout driver also uses `-f`. The fold requests driver changes without making their agreement with the launcher a sprint gate prerequisite. `.plans/plans/go-s3-stack-client/plan.md:83`, `.plans/plans/go-s3-stack-client/plan.md:231`, `.plans/plans/go-s3-stack-client/plan.md:370`, `.plans/plans/go-s2-stack-web/plan.md:236`, `.plans/plans/go-s2-stack-web/plan.md:275`, `deploy/test/checks.sh:22`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:41`.

A developer has different project names and ports in the root and deployment settings files. On the installed Compose command, a temporary configuration probe confirmed that exporting the default `COMPOSE_FILE` selected the root settings, while passing `-f` selected the deployment settings. The launcher can start the deployment project while the installer looks up a different project. Exporting the port alone at the sprint gate still leaves the project and file list unresolved.

Use the same project, file and environment resolution for the launcher, installer, token smoke check, live fixture and rollout driver. Keep the launcher plan's explicit file form for the default case and honor an explicitly supplied file list. Name the rollout driver's owner and make its change a prerequisite of the all-checks gate. Test competing root and deployment settings and one explicit override.

### logic-r2 05 — MAJOR, C9 — The live fixture guard checks a different state path from the installer

Evidence by reading. The default live check guards only the default state directory. The installer gives a nonempty state override precedence. The fixture records a state path, but cleanup calls the ordinary remove command after removing the recorded machine. That command acts on the current service and binary. `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:230`, `.plans/plans/go-s3-stack-client/plan.md:325`, `.plans/plans/go-s3-stack-client/plan.md:335`.

A disposable user has a previously enrolled custom directory and no installed unit or binary. With an inherited override pointing there, the guard passes and the installer starts a service for that existing identity before the test can fail for lack of a new waiting row. Cleanup can also remove another installation if the service was replaced after the fixture was recorded. A machine identifier scopes the server request and gives no authority over the current local service.

Give each run a fresh, explicit state path, reserved fixture record and isolated stack identity. Use the recorded tuple when reopening the fixture after boot. Before mutating existing resources on cleanup, verify the stack, user, unit, state path and local machine identity against that fixture. Exercise a pre-existing override and a service changed between the run and cleanup.

### logic-r2 06 — MAJOR, C6 — The browser check depends on an unassigned patch

Evidence by reading. Moving the browser edit outside the Stack team's scope is supported by the epic. The current Machines page still prints the old command. The Client plan describes a manual observation, and the fold asks the orchestrator to assign the edit. The new sprint gate lists the shell driver but carries no named dependency on the page update. The driver builds and calls the new client directly. `.plans/epics/go-rewrite/epic.md:121`, `src/frontend/src/screens/service.tsx:410`, `.plans/plans/go-s3-client-link/plan.md:74`, `.foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:46`, `.plans/plans/go-s3-stack-client/plan.md:373`, `deploy/test/checks.sh:217`.

All shell checks can pass while a copied command from the live page fails. A request to assign the fix does not make that gate pass.

Keep the edit with the team authorized to change the website. Require that team's command change before the sprint's Machines gate, and include the Client plan's paste-and-observe check on a real open page in the joint gate.

### logic-r2 07 — MAJOR, C6 — The exact address comparison rejects a valid rerun on port 80

Evidence by reading. The installer constructs the address with the port from Compose and searches for that literal address in the saved file. The server and client normalize away an explicit default port. The stack's guide permits port 80. The test stand-in saves its input address verbatim. `.plans/plans/go-s3-stack-client/plan.md:231`, `.plans/plans/go-s3-stack-client/plan.md:234`, `.plans/plans/go-s3-stack-client/plan.md:255`, `.plans/plans/go-s3-server-machines/plan.md:258`, `.plans/plans/go-s3-client-link/plan.md:90`, `.plans/plans/go-s3-client-link/plan.md:103`, `.plans/plans/go-s2-stack-web/plan.md:272`.

The first install at port 80 succeeds with an equivalent normalized address. The next run reports an address change and tells the owner to re-enroll. A textual search can also miss a valid document with line breaks around the field or accept a matching string outside the top-level address.

Parse the saved address as a top-level string in a valid identity and compare canonical origins using the agreed Client rule. Give an invalid identity its own refusal. Add a first-install and rerun case for port 80 with an independently normalized saved address, as well as a genuinely different port.

### logic-r2 08 — MAJOR, D16 — A running process can conceal a refusal from the selected stack

Evidence by reading. The patched rerun checks the saved address and whether the service remained active with the same process identifier. The Client plan explicitly keeps the process running and retrying when the server signing key differs. The readiness probe identifies the product and contract, without identifying this instance. `.plans/plans/go-s3-stack-client/plan.md:232`, `.plans/plans/go-s3-stack-client/plan.md:234`, `.plans/plans/go-s3-stack-client/plan.md:246`, `.plans/plans/go-s3-client-link/plan.md:202`.

Stop the first Compose project and start a different project on the same published port. The existing identity passes the text comparison. The new service process retries the wrong server key, stays active for five seconds, and the installer exits 0 even though the machine cannot join or reconnect. The test only models a unit becoming inactive.

State what a successful rerun guarantees. If it guarantees the enrollment reaches the selected stack, require an authenticated connection observation for the existing identity and a distinct, bounded outcome for refusal or temporary unavailability. If success means the local manager started a process, give that result its own message and report an unverified connection separately. Add the same-port, different-project case and a short, recoverable outage case.

### logic-r2 09 — MAJOR, plan — The frozen order test cannot observe installation of the binary

Evidence by reading. The new order assertion requires a trace entry for the rename. The stand-ins enumerate every writer of the call log and include no rename observer. The unit comparison uses the source template as its own oracle, so it also tolerates an incorrect restart policy in that template. `.plans/plans/go-s3-stack-client/plan.md:243`, `.plans/plans/go-s3-stack-client/plan.md:254`, `.plans/plans/go-s3-stack-client/plan.md:261`, `.plans/plans/go-s3-stack-client/plan.md:263`.

A correct installer with the listed stand-ins emits no rename line, so its frozen ordering test fails. If the test writer drops that assertion to make it pass, an implementation that places the binary before join and defers only the service call could pass the order check. An implementation that uses `Restart=always` in both the template and output also passes the Linux template comparison alone.

Make the client stand-in check the installed paths at entry, or add a pass-through rename observer that distinguishes the temporary build output from the installed path. Assert the unit's restart directives independently of the source template. Add failure injection around manager stop and activation, along with the existing failed-join mutation.

### logic-r2 10 — MINOR, C12 — The full gate omits the disposable test database

Evidence by reading. Check C12 requires the separate `LGTM_TEST_POSTGRES` setting before it runs the Go suite and the feed mutation. The disposable database wrapper supplies it. The new gate lists the stack's PostgreSQL helper for C5 but invokes the full driver without provisioning the database needed by C12. The claim that item 1 needs no Docker also conflicts with its chosen wrapper. `.plans/plans/go-s3-stack-client/plan.md:158`, `.plans/plans/go-s3-stack-client/plan.md:373`, `.plans/plans/go-s3-stack-client/plan.md:460`, `deploy/test/checks.sh:591`, `deploy/test/with-postgres.sh:71`, `deploy/test/with-postgres.sh:112`.

On a clean runner that follows the sprint gate and has no inherited test database setting, the full driver fails C12 even when the stack is healthy.

Use the existing disposable database wrapper for the full gate or require an explicitly provisioned disposable database. Name the host database dependency in item 1 and distinguish its direct Go command from the Docker-backed wrapper.

### logic-r2 11 — MINOR, C6 — The port stand-in still prints a malformed mapping

Evidence by reading. The patch attributes empty output to a parameter expansion in a line with a literal address prefix. The expected refusal test accepts either empty output or a malformed mapping, since the installer rejects both. `.plans/plans/go-s3-stack-client/plan.md:231`, `.plans/plans/go-s3-stack-client/plan.md:256`, `.plans/plans/go-s3-stack-client/plan.md:262`.

With the variable set to empty, a temporary Bash probe confirmed that the expansion removes the port and leaves the prefix. The claimed empty-output fixture still has no direct evidence.

State the two operations explicitly. Default an unset variable to 8787, then print the mapping only for a nonempty result. Assert the stand-in's empty standard output separately from the installer's refusal. Keep a malformed-output case as a separate scenario.

### logic-r2 12 — MINOR, plan — Removal depends on enrollment tools it never uses

Evidence by reading. The patch adds named prerequisite failures for Go, Docker, curl and jq before it branches to removal. Removal only uses local paths and the service manager. The removal test runs with every prerequisite present. `.plans/plans/go-s3-stack-client/plan.md:225`, `.plans/plans/go-s3-stack-client/plan.md:230`, `.plans/plans/go-s3-stack-client/plan.md:284`.

A user who uninstalled Go or Docker cannot remove the client service. The service can remain enabled even after the user removes the stack.

Check only the tools used by removal on that path. Keep the service-manager stop check before deleting the installed files. Add a removal case with the build and stack tools absent.

## Answers to the task questions

1. The sixteen numbered patches and the associated decision amendments are checked in the register below.
2. The two places marked `UNREVIEWED MECHANISM` fail separate tests. The directory lock cannot serialize all writers and survives a forced kill. The archive allowlist accepts an unrelated repository directory. Findings 01 to 03 cover the interactions.
3. The D19 rejection has a real scope basis in the approved design's one-user convenience and the recorded decision to leave another operating-system user outside wave 1. It is a scope choice, with no claim that duplicate marks are prevented. The C6 rejection correctly places the source edit outside Stack ownership. The live command remains broken by reading, and the handoff needs a required gate. See finding 06, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:44`, and `.plans/plans/go-s3-stack-client/attack/round-1-findings.md:45`.
4. The Store, vault, token command and test fixture writers and readers match their interfaces. The command dispatcher supplies the lookup, the issuer supplies the address, and Stack invokes the binary. The state setting is written by installer, unit or agent and the rollout fixture, and read by the client and installer. The recovery state has no owner. The lock misses the shared service writers. Compose and the browser gate have unresolved consumers. See findings 01, 02, 04 and 06, and `.plans/plans/go-s2-server-signin/plan.md:500`, `.plans/plans/go-s3-server-machines/plan.md:145`, and `src/client/internal/config/config.go:27`.
5. The truth tables below expose the contradiction between a preserved installation and a pre-join move, the stale lock versus the next-run refusal, and the acceptance of both an empty fresh state and an abandoned cutover.
6. The original compile blocker is removed. The rejoin recovery and serialization majors remain open. The second-run address and service assertions leave supported cases open. The live fixture guard and rollout checks still miss parts of their original failures. Findings 01 to 09 and 11 identify them.
7. The patched text touches all eight logic questions. The numbered lens answers below give the relevant behavior and test gaps.

### Patch register

| Patch | Result |
|---|---|
| 1. Overview | Fetching from the server command matches the design. The Linux and macOS descriptions state separate start points. The epic still states the broad restart requirement, and the fold leaves the platform scope for the owner. See `.plans/epics/go-rewrite/epic.md:33` and `.foundry/go-rewrite/results/fold-go-s3-stack-client-r1.md:37`. |
| 2. Key decisions | The temporary binary protects the installed binary from a returned join error. Stopping and moving the old state before join contradict the promise that nothing working changes first. The rerun rule has the gaps in findings 01, 07 and 08. |
| 3. Workstreams | The fresh-boot check and next-connection removal improve the promised observations. Recovery after interruption remains undefined, and the fixture can adopt earlier state. Findings 01 and 05 apply. |
| 4. Rollout order | Server work has a named team, and stack runs move to the joint gate. The missing browser and driver prerequisites can still let the gate omit required work or target a different stack. Findings 04 and 06 apply. |
| 5. Design citations | Nothing found in the spelling out of workstreams or the citation of the one-sentence rule. The join and rollback paths still need defined diagnostics for failures. |
| 6. Public names | The new Store and issuer declarations remove the original mismatch, including the vault client having no close method. The current `TokenOptions` has only `HostsServer`, so remove the remaining phrase about passing an empty address. The unit, archive and lock contract has the gaps in findings 01 to 03. See `src/server/internal/store/pool.go:35`, `src/server/internal/store/pool.go:290`, `.plans/plans/go-s2-server-signin/plan.md:433`, and `.plans/plans/go-s3-server-machines/plan.md:145`. |
| 7. Service, address and exit statuses | Statuses 0, 1 and 75 align with the Client command. The narrowed removal promise holds for those statuses under the intended manager policies. Cite the Client combined test as `TestLinkRestartReplaysAfterDroppedResult`. Default-port normalization, connectivity and rollback still need findings 01, 07 and 08. See `.plans/plans/go-s3-client-link/plan.md:274` and `.plans/plans/go-s3-client-link/plan.md:300`. |
| 8. Work item 1 | Corrected interfaces, Server ownership, fixed usage and failed-write exit status remove the stated faults. Test the fixed failure diagnostic itself and dispatch through `run` once, since the seven direct-body tests cannot observe a stale dispatcher. The current test runner dependency is in finding 10. |
| 9. Installer acceptance rows | The new failure and rerun checks cover the listed single-run, synchronous cases. The claim that an earlier installation stays as it was needs the recovery and fixture cases in findings 01 and 05. The ordering test still needs its observer. |
| 10. Work item 2 | The key-only refusal prevents a silent second join on the plain path. Trace suppression, private test scratch and the two state-variable replacements address their stated paths. Findings 01 to 04, 07 to 09, 11 and 12 cover remaining failure paths and proof gaps. |
| 11. Work item 3 | Reading the agent setting and requiring a graphical session fix the stated omission. The Linux recovery and per-user lock defects carry over to the agent path. The real manager gate should exercise a previously disabled agent, rollback and login start. |
| 12. Work item 4 | The new boot identifier blocks a same-boot pass and the password input removes it from the `jq` argument list for this fixture. A fixture can still adopt existing custom state. Require one actual first-login session and bind the observed exit 75 to the invocation under test. Finding 05 and the post-build checks apply. |
| 13. Installer risks and verification | The positional-token risk is recorded and the input form remains with the owner. The stated rejoin mitigation remains incomplete. The status check only proves local process liveness, and the route still depends on shared Compose selection. |
| 14. Sprint gate, post-build and exclusions | Splitting item and sprint gates is correct. The full gate needs the shared Compose contract, the browser check and its test database. The crash scenario in PB1 depends on a recovery rule that needs to be written before build. The one-user, restore and copied-state exclusions are consistent with the review scope. |
| 15. Traceability | The D19 and one-user links are accurate. The error and secret rows correctly name the direct usage and trace tests. The claimed C6 and joint coverage still needs the named dependencies in findings 04 and 06. |
| 16. Guardrails and sizing | The three Server files have Server writers. No competing write to those files is named. The Stack test-driver handoff still needs a gate. The item 1 runner claim needs to account for the Docker-backed fixture in finding 10. |
| Decision rows 8, 40 to 45 and superseded rows | Server seats and fixed usage are consistent. Rows 40 to 42 incorporate the recovery, lock and address defects above. Row 43 splits the gates but inherits their missing prerequisites. Row 44 correctly states the intended platform distinction and still needs the epic's decision. Row 45 leaves the token input change with the owner. The earlier closure references change no runtime behavior. |

For the finding groups, G1 and G16 have the stated fixes. G6 fixes the plain key-only rerun. G2 remains partial. G3 and G11 still require gate dependencies. G4, G5 and G7 remain open in the scenarios above. G8 and G20 are owner decisions. G9 and G12 have the rejection evidence in answer 3. G10 improves the boot proof but misses fixture ownership. G13 was forwarded and remains outside this patch. G14 fixes the live fixture's password input and forwards the rollout driver's input. G15 fixes the normal exit-status path. G17 adds refusal cases but leaves ordering and input gaps. G18 adds the scratch scan. G19 mostly closes the named cleanup items, while the absence and ordering observations still need the explicit test mechanisms above.

### Truth table for selecting an operation and state

| State selected at entry | Plain install or rerun | Rejoin | Remove |
|---|---|---|---|
| No installed service and no state | Enables a fresh enrollment. | Enables a fresh enrollment. A failed join has no old archive to restore. The unconditional rollback wording leaves this cell empty. | The service may already be absent. Removal still requires unused tools. |
| Active service with a matching, valid identity | Rebuilds and restarts while keeping identity. A connection refusal at the same address can go unseen. | Stops the service and exchanges state. Failure must restore the prior active state. | Stops and disables the service, preserving state. |
| Installed but inactive service with a matching identity | Starts it and observes the process. A verified removal can give a clean exit. | The written rollback starts the previously inactive service. This contradicts restoring the prior installation as it was. | Stops and disables the service, preserving state. |
| A key with no identity | Refuses the unfinished join. | Moves the recognized old state if one exists. An ambiguous outcome needs retained evidence. | Keeps the key and removes the local installation. |
| Valid enrolled state at an equivalent normalized origin | Can refuse port 80 as a different address. | Mints a new enrollment unnecessarily if the operator takes the rerun's advice. | Keeps the selected state. |
| Explicit override differs from the installed service's directory | May create a second enrollment and replace the shared service. | May move a different state directory while stopping the installed service. | Takes a different lock while removing the same installed service. |
| Foreign directory with only allowed child names | Its files determine whether the existing checks recognize an identity. | Accepts and moves the directory. | Leaves the directory while acting on shared service files. |

### Truth table for interruption and service status

| Event | Written outcome | Remaining gap |
|---|---|---|
| Readiness, build or token-fetch refusal before stopping the service | The old service keeps running. | Creating missing parent directories and the build directory can already change the filesystem. PB1's server-down setup exercises this preflight case. |
| Stop or archive fails | Only an unloaded unit is explicitly ignored. | Specify which steps may continue and what service state is restored on each failure. |
| The new join is rejected before a server write | The old state is restored on a returned failure. | The fresh case has no archive and an already inactive service must stay inactive. |
| The server writes a waiting row, then join fails locally | The rollback deletes new local state. | Preserve the new key and a way to identify or reconcile the waiting row. |
| The installer is killed after a stop or rename | The trap is supposed to release the lock. | A forced kill bypasses the trap. No stored phase supports repair after reacquisition. |
| Join succeeds, then binary, unit or activation fails | The proposed sequence returns a local failure wherever it defines one. | Which enrollment, binary, unit and service state are authoritative is unspecified. |
| Process exits 75 or 1 | The service manager restarts it. | Bind the exit 75 to the requested restart and observe the replacement invocation. |
| Process exits 0 | The service manager leaves it stopped, then boot or login may start it again. | The client also uses status 0 for terminal local store failures. Give the owner the appropriate diagnosis for the observed cause. |
| The service manager stops the process explicitly | The stop is intentional. | Distinguish a successful stop from a loaded service that the installer failed to stop. |

## Answers to the logic lens

1. The archive suffix handles a same-second tie through exclusive directory creation. Nothing found on sequence wrap for an archive name. A restart has no recorded replacement phase. Token expiry uses the database clock after taking the row lock. A boot comparison needs a positive service timestamp and at least one actual first-login timestamp from the new boot. Process identifiers are observations of invocations and can be reused. `.plans/plans/go-s3-stack-client/plan.md:282`, `.plans/plans/go-s3-stack-client/plan.md:341`, `.plans/plans/go-s3-server-machines/plan.md:270`.
2. Before service stop, database, vault, build and token acquisition failures deny enrollment. The client retries a lost join reply with the same key and token while valid. Failures after the stop, an uncertain server commit and interruptions require the repair policy in finding 01. A failed token output can leave an unused token row that expires under Server policy. `.plans/plans/go-s3-stack-client/plan.md:154`, `.plans/plans/go-s3-client-link/plan.md:105`, `.plans/plans/go-s3-server-machines/plan.md:259`.
3. See the two truth tables for the empty and contradictory cells. For exit observation, an active process can be disconnected or refused, and a clean stop can reflect removal or local store failure. `.plans/plans/go-s3-client-link/plan.md:202`.
4. No flag means an ordinary unmarked token. An empty state override means use the installed state setting or default. A missing identity with a key refuses ordinary replay. A missing key and identity can be a fresh state or an interrupted move, which presently collide. A token prefix with no payload is refused, while a syntactically shaped invalid payload can reach the real client only after the old service has stopped. A missing or empty saved address needs an explicit invalid-identity outcome. `.plans/plans/go-s3-stack-client/plan.md:153`, `.plans/plans/go-s3-stack-client/plan.md:228`, `.plans/plans/go-s3-stack-client/plan.md:233`, `.plans/plans/go-s3-stack-client/plan.md:239`.
5. Two invocations with different state parents can change one service. The running Go client and an installer can act on the same directory under separate locks. Machine removal can race service startup, so manager liveness alone cannot establish the stored machine's standing. Findings 02 and 08 apply.
6. Item 1's forced false mark and standard-error token leak fail the flag and empty-error assertions as named. A dispatcher left on its placeholder passes those direct command calls. Item 2's printed token and omitted restore fail their checks. The service-before-join mutation is observable through manager calls, while moving just the binary first needs a rename or filesystem observer. A crash between steps escapes all three mutations.
7. For item 1, leave the dispatcher on its placeholder and implement only the tested body. For item 2, apply a different restart policy to both the source template and rendered unit. For item 3, ignore a failed bootout during rejoin, which none of the four cases covers. For item 4, implement only the dry-run response and skip every live path. The joint gates catch some of these after merge, with the handoff gaps noted above.
8. A fixed installation lock and one recorded cutover phase would prevent split ownership of the binary and the state selector. Positive state-directory ownership would prevent archiving an unrelated folder with identical entry names. A fixture tuple bound to local and server identities would prevent cleanup from acting on a later installation. Findings 01 to 05 apply.

## Post-build checks

- **PBR2-1. Exercise the resolved recovery rule on real managers.** On Linux and macOS, interrupt at the recorded archive and activation boundaries after the plan defines recovery. Require the specified enrollment to resume or the original active installation to return, with no extra enrollment created by the retry. The Stack builder records redacted local state, manager status and the exact server machine identifiers.
- **PBR2-2. Check manager lifecycle by invocation.** On Linux, record the service invocation at a fresh boot before the first actual login, then ask that connected invocation to restart. Require its own journal exit status 75, one replacement invocation, and the same machine online again. Remove that machine and require a clean exit with no automatic restart during the same boot. The live-test owner captures the boot, invocation and machine identifiers.
- **PBR2-3. Check real manager operations on macOS.** Begin with the existing agent disabled, perform the planned install or upgrade, then log out and back in. Require the intended binary and state folder, a running agent after login and a clean non-restarting exit on removal. The Stack builder records agent state across the operations.
- **PBR2-4. Check the deployed command.** Run the rebuilt server command through the dispatcher against the isolated stack and issue a fresh token on the real Machines page. Paste the displayed command into a disposable client shell and require the waiting row in the page already open. The orchestrator records the command shape, the subprocess status and the waiting machine identifier without recording the token.

## Grade

5 of 10. There are 0 blockers, 9 majors and 3 minors.
