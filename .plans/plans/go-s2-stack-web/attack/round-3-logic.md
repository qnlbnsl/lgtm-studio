REVISE

The sprint still has a circular merge gate because the Server plan requires deployed checks that need the Stack image built from its own gated work.

## Findings

### logic-r3 1 — BLOCKER — plan

The Stack fold makes checks C1 through C4 and C13 sprint gates, but the Server plan still makes deployed C13 part of acceptance for items 8 and 9 and deployed C2 part of acceptance for item 9. The fold's request S11 also omits the item 8 acceptance clause. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:92`, `.plans/plans/go-s2-stack-web/plan.md:126`, `.plans/plans/go-s2-server-signin/plan.md:455`, `.plans/plans/go-s2-server-signin/plan.md:459`, `.plans/plans/go-s2-server-signin/plan.md:535`, and `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:51`.

From an empty sprint branch, Server item 8 cannot obtain C13 evidence until Stack can build and run Server item 9. Item 9 needs item 8 and cannot clear its own gate either. An exact-commit merge rule makes the circle operative even if the implementation exists on a worker branch. Apply decision 40 to both Server acceptance clauses and to the Server execution matrix. Keep the named item tests as their merge gates. Put the deployed checks and any browser scenario that needs both teams at the sprint gate, then verify the order on the two plans together. The governing rule is at `.plans/epics/go-rewrite/epic.md:75`.

### logic-r3 2 — MAJOR — C4

The launcher performs full settings validation before the removal path can stop the live services. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:305`, `.plans/plans/go-s2-stack-web/plan.md:308`, `.plans/plans/go-s2-stack-web/plan.md:315`, `.plans/plans/go-s2-stack-web/plan.md:319`, and `.plans/plans/go-s2-stack-web/plan.md:354`.

Start the stack and retain a working session. Then leave an unterminated password line in the settings file and run the advertised removal command. The dry Compose parse fails before `stop`, and the old server and old session remain live. The fixed port only handles a bad port value. For removal, resolve and lock the installation using its recorded project and state identity independently of web settings. Confirm the serving containers have stopped before attempting the database phase. Specify an explicit failure outcome if the target cannot be identified or the stop cannot be confirmed. Once stopped, a failed database phase or an unconfirmed commit must report both facts and leave the services stopped. Add a running-stack test for each of those outcomes, including a failure in the stop itself.

### logic-r3 3 — MAJOR — R9

The permissions check and Compose can use different settings files, and each later Compose process can reload the files. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:305`, `.plans/plans/go-s2-stack-web/plan.md:307`, `.plans/plans/go-s2-stack-web/plan.md:311`, and `.plans/plans/go-s2-stack-web/attack/round-2-findings.md:38`.

A live probe used Compose 5.5.0 and disposable files. With `COMPOSE_FILE=deploy/compose.yaml`, both an absent and an empty `COMPOSE_ENV_FILES` let a mode-`0644` repository-root `.env` take precedence over a mode-`0600` `deploy/.env`. The launcher checks only the latter. A separate failure occurs when the checked file is replaced by an unfinished edit after the dry parse. The later `build` call can print a parse diagnostic with the password text that the first call suppressed. The fold's own probe records that diagnostic at `.plans/plans/go-s2-stack-web/scouting.md:244`. Use a single explicitly supplied, owner-only settings snapshot for every phase and suppress implicit lookup, with an empty private source when the default file is absent. Keep the fixed diagnostic for every invocation that can still fail to load settings. Test a second root file and a replacement between preflight and the next call.

### logic-r3 4 — MAJOR — C1

The line-oriented port extraction cannot tell an interpolation key from a continuation of another value. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:308`, `.plans/plans/go-s2-stack-web/plan.md:309`, and `.plans/plans/go-s2-stack-web/scouting.md:243`.

A live probe on Compose 5.5.0 used a fixture password. A password with a newline followed by `LGTM_WEB_PORT=19876` produced a matching line for 19876 followed by the actual setting's line for 18787. When the actual setting is absent, only the continuation matches. Choosing the first or last line, rejecting duplicates, or accepting a unique line therefore cannot implement the default and precedence rules across both states. Read the effective value from a parsed, structured Compose model that preserves the raw interpolated value, validate it before export, and avoid exposing the rest of the model. Freeze a test that supplies a multiline value for an unrelated variable, with and without a real port, and checks both the publication and the server origin.

### logic-r3 5 — MAJOR — C4

The password-removal command uses the shared image tag instead of the image of the installation it stops. The full stream fixture also builds into a shared tag. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:250`, `.plans/plans/go-s2-stack-web/plan.md:269`, `.plans/plans/go-s2-stack-web/plan.md:310`, `.plans/plans/go-s2-stack-web/plan.md:318`, `.plans/plans/go-s2-stack-web/plan.md:425`, and `.plans/plans/go-s2-stack-web/plan.md:546`.

Installation A is serving from image A. Another Compose project on the same Docker daemon builds a different revision under `lgtm-server:local`. A's lock covers its state folder. It cannot stop the other build from moving the shared tag. A then stops and runs the replacement image against its database. The replacement can fail to revoke or run its own migrations before revocation. The same tag race can displace an image between the stream fixture's build and its container creation and fail an otherwise sound gate. For removal, capture the project's serving image identifier under its lock and run exactly that image without a build or pull. Report unconfirmed removal if the image cannot be recovered. Give each test project its own image tags and check the image identifier of the one-shot process against the serving container's identifier.

### logic-r3 6 — MAJOR — C3

The new browser and removal tests require a literal false field that the default Connect JavaScript Object Notation (JSON) response omits. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:231`, `.plans/plans/go-s2-stack-web/plan.md:335`, `.plans/plans/go-s2-server-signin/plan.md:318`, `src/contracts/proto/lgtm/v1/session.proto:23`, and `src/frontend/src/server/sync.ts:82`.

A live probe used the pinned Connect codec and the generated response type. The signed-out request returned status 200 and a body with no `signedIn` field. The signed-in response included `"signedIn":true`. A correct private browsing session or a correctly revoked cookie therefore fails an assertion on the presence of literal false. Require a successful response with the expected product and contract and decode it using Protocol Buffers defaults. Require a true result before removal and after browser sign-in. Require a decoded false result in a fresh browser and after revocation, and confirm that a protected call with the old cookie returns `unauthenticated`. Keep the browser's sign-in form check in the private window.

### logic-r3 7 — MAJOR — C6

The new port lookup omits the Compose file and fixes the project to `lgtm`. The proposed driver fallback also runs before the web container exists on a clean launch. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:305`, `.plans/plans/go-s2-stack-web/plan.md:314`, `.plans/plans/go-s2-stack-web/plan.md:356`, `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:68`, `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:86`, `deploy/compose.yaml:1`, `deploy/test/checks.sh:22`, and `deploy/test/checks.sh:354`.

Confirmed live with Compose 5.5.0: from the repository root with no inherited `COMPOSE_FILE`, the guide's project and file selection fails to find a configuration for `config --services`. Adding the explicit file selects it. The launcher sets the file in its own process, so a later installer invocation does not inherit the setting. On a fresh run, the driver's C1 calls `configure_web` before calling the launcher, so a runtime lookup at that step also cannot discover the configured port. Give the launcher, guide, driver and installer one rule for selecting the deployment file and project. Read the published endpoint after startup and use it for both the join token and the client address. Run C1 at a nondefault port set only in the settings file, with no inherited Compose file or port variable. Repeat for a named project if that override is supported.

### logic-r3 8 — MAJOR — D17

The fold records a deferral for the encrypted cases on the Stack side only. The Contract plan still assigns the combined sprint 3 test a required encrypted join through the real proxy. The Server plan still describes a bundled encrypted site as a required browser scenario and ties the scenario to its sign-in criterion. The evidence is at `.plans/plans/go-s2-stack-web/decisions.md:52`, `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:55`, `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:60`, `.plans/plans/go-s0-contract/plan.md:753`, `.plans/plans/go-s0-contract/plan.md:758`, `.plans/plans/go-s2-server-signin/plan.md:209`, and `.plans/plans/go-s2-server-signin/plan.md:621`.

When the combined gate reaches the encrypted join, no service supplies its endpoint or certificate. A successful loopback test cannot fulfill that scenario. Land the dated acceptance changes in the receiving plans before treating the deferral as closed. Keep the loopback case in the combined link test. Scope any retained certificate checks to fixtures that their owners provide, and put the real encrypted deployment checks under the later proxy work. Keep the machine-certificate choice at its existing gate.

### logic-r3 9 — MAJOR — C1

The new held-lock scenario makes a known driver deadline mismatch observable, while the fix remains a request. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:230`, `.plans/plans/go-s2-stack-web/plan.md:465`, `deploy/test/checks.sh:377`, `deploy/test/checks.sh:385`, `deploy/test/checks.sh:119`, and `.plans/plans/go-s2-server-signin/plan.md:475`.

Hold the sessions-table lock from the beginning of the driver's C2 polling loop for 30 seconds. Its 20-second limit can fail the check and enter cleanup, which force-recreates the server with the original password while removal is still waiting on the lock. The recorded result then says nothing about whether the no-password process would have committed within its allowed budget. Land request D4 before the held-lock gate, or make the held-lock case a separate check owned by the sprint that waits for this process's committed marker and validates loss of the old cookie before any restore. State the allowed deadline for each phase and put a timeout around the recreation commands as well as their polls.

### logic-r3 10 — MINOR — plan

The new default-settings cases register a Git worktree in the source repository, while fixture cleanup only removes its folder. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:275` and `.plans/plans/go-s2-stack-web/plan.md:333`.

A live probe used a disposable local clone. After deleting the worktree directory, `git worktree list --porcelain` continued to list a prunable registration. The proposed suite accumulates checkout metadata after successful runs. Use a plain copy of the required checkout files for those preflight-only cases, or explicitly remove the registered worktree before cleaning the containing fixture directory.

### logic-r3 11 — MINOR — C8

The new negative timeout check covers one website path, leaving the adjacent link procedure outside the test. The evidence is at `.plans/plans/go-s2-stack-web/plan.md:155`, `.plans/plans/go-s2-stack-web/plan.md:436`, `.plans/plans/go-s2-stack-web/plan.md:440`, and `src/contracts/proto/lgtm/link/v1/link.proto:140`.

Change the matcher from exact `Open` to the whole link service. All seven stream assertions can still pass, including the default-limit test on `WatchClients`. An unfinished `Join` request then gets the longer limit contrary to row 39. Exercise a stalled body on `Join` as another default-limit case. Use a path expansion to the link service as the mutation, in addition to removing the matcher altogether.

## Answers to the assignment questions

1. Each group below records whether the patch removes its round 2 failure and where the changed rule interacts with another part of the plan.
2. The new settings and removal mechanisms in rows 37 and 38 fail the source, parsing, preflight, and image-selection cases in findings 2 through 5. The active route timeout in row 39 has a narrower scope, with the assertion gap in finding 11. The browser mechanism that replaced the marked row 34 fails the default-value case in finding 6. The marked row 27 remains in effect and the patch's port and forward rules expose findings 7 and 8. The other marked mechanisms in rows 29, 31, and 32 are superseded.
3. Nothing found. The register's outcome column has 25 fixed entries and three post-build entries. It has no rejected findings. See `.plans/plans/go-s2-stack-web/attack/round-2-findings.md:14` and `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:22`.
4. The names and their readers are traced under “Writers and readers of changed names.” Findings 1 and 5 through 8 identify handoffs that still lack a compatible reader or stable name.
5. The tables of settings, removal, gates, and stream routing below identify conflicting and empty cells, including failed removal preflight, false on the wire, and the driver's timed cleanup.
6. G1's blocker and G4, G5, G6, G7, G8, G13, and G14 still have open parts despite fixed outcomes. G11 was assigned a post-build check and still has the deterministic deadline conflict in finding 9.
7. All eight logic questions touch a patch. The next numbered list applies them to the changed items and the unchanged consumers that those items reach.

### What each patch closes

| Group | Result of this pass |
|---|---|
| G1 | The Stack text and epic now agree about joint gates. The Server acceptance still blocks the same dependency chain. See finding 1. |
| G2 | The file-list override works when exported. Confirmed live that Compose 5.5.0 accepts the reset keys and single-mount override in a disposable model. The timed restart control still needs a running engine. |
| G3 | The fixed-port override removes the specified bad-port failure from both routes. The project, state root, and volumes stay selected by the same inputs. A settings parse error still blocks an attempted live removal. See finding 2. |
| G4 | Canonical decimal validation removes the range, zero, and leading-zero cases if it receives the actual value. The extraction can select a continuation instead. See finding 4. |
| G5 | The empty selector now checks `deploy/.env`. The new invocation can also read the working directory's file. See finding 3. |
| G6 | The empty variable override and database-only route remove dependence on a lingering password or a reachable OpenBao after the serving services stop. Preflight and image selection still defeat the guarantee. See findings 2 and 5. |
| G7 | Home and sign-out have a viable later dependency. The new wire-level assertion and the Server plan's old browser acceptance still need adjustment. See finding 6 and `.plans/plans/go-s2-server-signin/plan.md:340`. |
| G8 | The Stack stops requiring an encrypted deployment. The receiving gates still name it. See finding 8. |
| G9 | On an uncontended daemon, the patch checks the correct build, file, and mount. Concurrent jobs can still move the tag between the build and container creation. See finding 5. |
| G10 | The scheduled echo count and marker before half-close repair the stated assertion gaps. The probe still needs to establish that an early transport end is a failure, even when no response-body byte arrived. |
| G11 | The timings have a named owner and a scenario. The current driver cannot run that scenario to its prescribed outcome. See finding 9. |
| G12 | The declared version floor excludes the old engine case. The supported-engine network test is still a post-build observation. The changed host-name test now covers both named aliases. |
| G13 | The parse of an already broken file is redacted. The default source and later reads remain uncovered. The argument check depends on the driver change. See finding 3. |
| G14 | Reading the running publication is the right source after startup. The concrete invocation and the driver's first-use order are incomplete. See finding 7. |
| G15 | The extended request names both generators. The current driver still appends suffixes, so the maximum-size case remains open until the driver change lands. See `deploy/test/checks.sh:400` and `deploy/test/checks.sh:415`. |
| G16 | The mount and directory guards reduce cleanup's filesystem reach. The added worktree case leaves repository metadata behind. See finding 10. |
| G17 | The scouting evidence establishes a longer idle window for the exact stream and the retained default on one ordinary route. The suite can still pass with the wider link matcher. See finding 11. |

### Writers and readers of changed names

- Stack item 4 writes the Compose services, fixture file list, settings selector, and fixture helpers. Stack items 4, 5, and 7 read them. The driver still selects its file with `-f`, and the guide and installer lookup need the same project selection as the launcher's ready line. The sprint 1 handoff request V5 also needs to land so its test name follows the rename. See `.plans/plans/go-s2-stack-web/plan.md:296`, `deploy/test/checks.sh:22`, and `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:74`.
- Stack item 5 writes the removal option and phases. The command, the guide, Stack launcher tests, and Server post-build check 3 read them. Server item 9 writes the committed and unconfirmed sentences, so the launcher must distinguish the current process's exact committed sentence from a general failure or prior output. Request S13 and the Server startup table agree on those sentences. See `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:53` and `.plans/plans/go-s2-server-signin/plan.md:486`.
- The generated contract supplies `signed_in`, Server item 7 writes its value, and the browser, launcher tests, and driver read it. The browser's display uses the decoded value. The two newly written acceptance assertions must do the same. See finding 6.
- Stack item 2 writes the exact route and timeout. Item 7 writes the probe headers, marker, trailer, modes, and override. The tests read those names, and the sprint 3 Stack installer plan still specifies its own echo program and override. Request P3 needs an explicit adoption or a decision to keep distinct fixtures. See `.plans/plans/go-s2-stack-web/plan.md:442`, `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md:70`, and `.plans/plans/go-s3-stack-client/plan.md:356`.
- The Stack no longer writes a certificate-file setting. The Machines plan still supplies its reader and the Contract and Server plans still require their encrypted scenarios. The loopback token needs the effective published endpoint wherever the Server and installer read it. See findings 7 and 8.

### Where the rules meet

| State | Full launch | Vault-only route | Password removal |
|---|---|---|---|
| Settings readable and no effective port | Use port 8787. | Use port 8787 and skip the web services. | Use port 8787 for parsing and stop the web services. |
| Settings readable and canonical port set | Use that same port for publication and origin. | The private override takes precedence. | The private override takes precedence. |
| Settings readable and bad port set | Refuse. Bad syntax can fail Compose before the launcher's port diagnostic. | The fixed port bypasses the bad value. | The fixed port bypasses the bad value. |
| Settings fail their mode check or Compose cannot parse them | Refuse before phases. | Refuse before phases. | Refuse before stopping the live server. This conflicts with a removal path that stops first. |
| An unchecked root file exists, or a file changes after preflight | Compose can use a different input. | Same empty cell in source selection. | The fixed values cover the port and one-shot password, but do not establish the target installation. |

| Removal state after resolving the installation | Commit observed | Commit absent or uncertain |
|---|---|---|
| The stop succeeded and the database phase completed. | Exit 0 after this run's exact marker. The web and server services stay stopped. | Exit 1, keep both stopped, and ask the operator to retry. |
| The stop succeeded and the database phase failed. | A new commit marker cannot come from this invocation. | Both services should stay stopped. The database failure path lacks a specified removal result. |
| Stop failed, or the process was interrupted during stop. | This run must not claim success. | The serving state and operator diagnosis are unspecified. |
| No commit was confirmed and the ordinary launcher receives the same old password. | This cell only applies after a later successful retry. | The guide calls for a retry before restore. The ordinary route does not enforce that order, so the report must distinguish a stopped service from revoked sessions. |

| Other interacting rules | Expected result | Conflict or empty cell |
|---|---|---|
| Server item acceptance and the Stack image both need deployed evidence. | Build and merge the named item checks, then run the joint checks on the merged sprint. | The receiving Server items still require the deployed check first. |
| No session, a valid session, or a revoked session. | Decode false, true, and false respectively. | On the two false cases, the valid response can omit the field. |
| Exact `Open`, another link path, or a website path. | The extended idle timeout applies to the first. The other two keep the default. | Tests observe the first and last cases. They leave the adjacent link procedure unmeasured. |
| Held database lock versus the driver's C2 poll and cleanup. | Observe committed revocation within the Server budget before restoring. | At 20 seconds the driver can restore while the 30-second hold still blocks revocation. |

### Answers to the logic lens

1. The echo schedule uses a monotonic start, starts numbering at zero, stops before the upper time boundary, and expects ten scheduled messages plus one after the gap. A wrong or missing echo should fail by message number. The live gate does not exercise a late or duplicated frame during the silence, and it should define when to fail a missed send slot instead of bunching sends to catch up. The counters in normal and soak fixtures are bounded far below wrap. Each new stream starts again at zero.
2. The route-fixed port bypasses a settings value error but leaves parse failure in front of the first stop. A database failure, a stop failure, a lost revocation marker, and a restore with the old password are different outcomes. Findings 2 and 9 identify the missing failure and retry cells. The timeout probes use elapsed time, so wall-clock disagreement is unnecessary to reproduce either bug.
3. The tables above show the conflicting gate order, settings fallback, removal failure, decoded false, and the untested adjacent route. Nothing else surfaced in the patched deadline and count rules.
4. A missing or empty port defaults to 8787. Port zero, ranges, and leading zeros are refused on the full route. Empty and unset settings selectors must select the same explicit default. The removal route sets an empty password, which Server item 9 treats as unusable and reconciles before exiting. The patched browser assertion must decode an omitted session flag as false. No new product enumeration or stored position is introduced.
5. Launch and removal serialize on the same state-folder lock. Another project can retag the same image, an editor can replace settings between calls, and the driver's direct restoration does not take the launcher's lock. The report gives each a concrete failure path. The stream probe must keep its request writer and response reader active together in the quiet-down case.
6. Item 1's user mutation and item 3's user mutation contradict direct image assertions. Their changed dependency permits the named tests once Server files are present. Item 2's catch-all mutation contradicts the 421 assertion. Item 4's port and network mutations contradict the resolved manifest. Item 5's early build, shorter wait, and dropped empty override contradict named assertions. To prove the dropped stop mutation, require an unmodified restart-policy control that demonstrates the count grows without the stop. Item 7's handler mutations reach its two assertions. Broadening the timeout to both link procedures is a wrong change that can still pass. No running mutation was executed in this review.
7. Cheap wrong implementations for the seven items are listed below. Their purpose is to identify what the patched gates can actually distinguish.
8. An explicit settings source and a structured port read eliminate the unchecked input states. Selecting the immutable image of the stopped installation eliminates a cross-project tag change. A decoded result eliminates the difference between an omitted response field and an explicit false. The timeout must attach to the exact procedure so ordinary requests cannot inherit the longer deadline.

| Work item | Wrong implementation that can pass its listed checks |
|---|---|
| 1. Server image | An image launches a stub that prints the setting name and exits on the only execution path its own gate exercises. A healthy run of the real executable is covered later by the joint gate. |
| 2. Web server rules | The longer deadline applies to both link procedures. Its suite exercises routing to `Join`, but it does not hold that request's body open. |
| 3. Website image | The image records the correct build flag in an unused step and builds the bundle with another value. The history check sees the right text. The browser observation is what would reject the result. |
| 4. Compose and fixtures | Project and state are isolated but image tags are shared. Model assertions and ordinary sequential runs pass until another build moves a tag. |
| 5. Launcher | The unconfirmed removal test can pass when the one-shot stand-in never starts. It checks the generic report and stopped state, but does not require its marker. Force the stand-in setup to succeed, require the marker, and separately test failure before the executable starts. |
| 6. Operator guide | A literal check of quoted output still passes when the port command cannot discover the Compose file. |
| 7. Stream probe | Count-based upward assertions accept a burst of the required ticks during a supposed continuous stream. Require a tick before the hold, bounded gaps during it, and a marker exchange after it. |

## Post-build checks

- With the inherited restart policy, run the failing stand-in without the launcher's stop in an isolated project as a control. It must demonstrate the restart behavior that the mutation is intended to expose. Then run the production launcher and verify the service stays stopped.
- Hold a quiet-down request open through the actual merged web image and end the peer early before sending a marker. The probe must fail promptly with a stream-end error. Repeat with intact peers. The count must arrive while the request body can still accept another line, followed by a clean end and the correct trailer.
- Run the merged image beyond ten minutes with frequent upward messages, then test each body-idle boundary independently. Regular traffic must keep `Open` alive beyond ten minutes of elapsed runtime. A stopped body on an ordinary website request or `Join` must end near the ordinary deadline. Releasing an ordinary stalled request must leave a simultaneous valid sign-in usable.
- Run the project discovery and removal checks with two different Compose projects on one Docker daemon. The installer must discover its own endpoint, and removal must start the same image identifier as its stopped server. The other project must keep its running containers and sessions.

The review follows the patch from `8f961b33` to `496cd5e2`. The local Compose client and a Connect handler were available for the reported probes. The Docker daemon was inaccessible in this review, so container starts, timed streams, and actual revocation remain post-build checks.

## Grade

4 out of 10. Findings total one blocker, eight majors, and two minors.
