REVISE

The password-removal command can select a different Compose project from the launcher and leave the intended server and its old sessions running.

## Findings

### logic-r4 1 — MAJOR — C4

Startup and password removal derive the installation identity from different inputs. The launcher lets Compose load the project from its settings, while removal matches `${COMPOSE_PROJECT_NAME:-lgtm}` in the shell. The relevant rules are in `.plans/plans/go-s2-stack-web/plan.md:173`, `.plans/plans/go-s2-stack-web/plan.md:236`, `.plans/plans/go-s2-stack-web/plan.md:243` and `.plans/plans/go-s2-stack-web/decisions.md:55`. The expected removal behavior is in `docs/specs/2026-11-25-go-rewrite-foundation-design.md:208`.

Confirmed live with Docker Compose 5.5.0 and a disposable model whose declared name was `lgtm`. From `deploy/`, I set `COMPOSE_FILE=compose.yaml`, left the project name unset in the shell, and put `COMPOSE_PROJECT_NAME=studio` in `deploy/.env`. `docker compose config --format json` returned `studio` as the project name. The planned removal expression returned `lgtm` in that same shell. If `studio` has the live server, removal either finds no server or acts on a separate `lgtm` project. A committed purge from that second project would produce a success message while the `studio` cookie remains valid. The fixture always exports its project name, so the current removal test misses this case.

Give startup, removal and port lookup one project selector that works with an unreadable settings file. A small option is to define the selector as a nonempty shell `COMPOSE_PROJECT_NAME` or the default `lgtm`, and force it into the environment of every Compose invocation in each caller. Document the source of the selector and test either a conflicting file value or a recorded installation identity, depending on which rule is selected. Start a named project beside a separate default project, break the named project's web settings file, and require that removal stops and revokes the named project alone.

Category is design. Stack owns the fix. The selector mismatch is confirmed live. The risk score is 72, from severity 6 for a failed password removal, fallout 3 for an installation the operator can recover, and probability 4 for an alternate project selector.

### logic-r4 2 — MAJOR — C4

The failure branch promises that the server stays stopped in the case where the server is still running. It also omits a failed or unverified web stop. The patch is at `.plans/plans/go-s2-stack-web/plan.md:242`, `.plans/plans/go-s2-stack-web/plan.md:245` and `.plans/plans/go-s2-stack-web/plan.md:260`.

By reading, a failed `docker stop` can leave the live server and web server up. An implementation that exits on the Docker error never prints the promised revocation result. An implementation that reaches the proposed unconfirmed line instead tells the operator that the server stays stopped, although the observation was that it is still running. A failed web stop with a successful server stop can also take the success branch despite the promise that both stop. Neither of the prescribed unconfirmed tests attempts a failed stop against a live fixture.

Require positive confirmation that both selected services are stopped or absent before running the purge. Report an unverified stop separately from an unconfirmed purge after a verified stop. The stop failure should name the service whose state is still running or unknown and instruct the operator to retry. Exercise a failed stop with a previously valid cookie and require a nonzero exit, no maintenance run and an accurate diagnostic. Treat failed inspection as unknown rather than absence.

Category is failure handling. Stack owns the fix. The contradictory branch is confirmed by reading. The risk score is 72, from severity 6 for misleading the operator during revocation, fallout 3 for a recoverable installation, and probability 4 for a Docker stop or inspection fault.

### logic-r4 3 — MAJOR — R9

The ordinary launcher emits a password in a Compose parse diagnostic, contrary to the design's logging rule and the expected observation of post-build check 4. The rules are at `docs/specs/2026-11-25-go-rewrite-foundation-design.md:370`, `.plans/plans/go-s2-stack-web/plan.md:236`, `.plans/plans/go-s2-stack-web/plan.md:376` and `.plans/plans/go-s2-stack-web/scouting.md:244`.

Confirmed live with a disposable settings file and Compose 5.5.0. A line beginning `LGTM_WEB_PASSWORD="fixturemarker` with no closing quote made `docker compose config --quiet` exit nonzero and print the fixture marker in its diagnostic. A single operator can make this typo in the normal private file. The launcher can then write the password into saved terminal output or a captured test log. Changing the working directory and disabling shell tracing leave that failure in place.

Keep validation in Compose and handle its settings-load failure at the output boundary. Emit a safe settings-load diagnostic wherever a launcher Compose invocation can print the raw parser error, including calls made by a helper. Preserve a safe taken-port diagnostic. Require post-build check 4 to look for the marker after the malformed-file call as well. If the raw message is an intended exception, have the orchestrator record the exception to rule 9 before changing the expected result.

Category is secret handling. Stack owns the fix. The emitted value is confirmed live. The risk score is 168, from severity 6 for a logged password, fallout 7 because it cannot be made unseen after disclosure, and probability 4 for a malformed settings edit.

### logic-r4 4 — MAJOR — C1

The new published-port handoff still lacks the changes required in the rollout driver and the installer. The promise is at `.plans/plans/go-s2-stack-web/plan.md:278` and `.plans/plans/go-s2-stack-web/plan.md:373`. The rollout driver still defaults to the shell value or 8787 and calls `configure_web` before the launcher. Each check also runs in its own subshell. See `deploy/test/checks.sh:32`, `deploy/test/checks.sh:354` in the committed driver, and `deploy/test/checks.sh:658`. The installer looks up the running port, but its planned working directory remains the repository root. See `.plans/plans/go-s3-stack-client/plan.md:83`, `.plans/plans/go-s3-stack-client/plan.md:227` and `.plans/plans/go-s3-stack-client/plan.md:234`.

With 18787 set only in `deploy/.env`, the launcher can start the correct stack while the driver probes 8787. Moving lookup after launch in C1 alone leaves C2, C3, C4 and C13 with the same problem. In a disposable Compose 5.5.0 probe with `COMPOSE_FILE` set, a conflicting repository-root `.env` selected a different project and port for the installer's working directory than for the launcher's. The installer can therefore look up no endpoint or the endpoint of a different project. Requests D5 and P1 are documented at `.foundry/go-rewrite/results/fold-go-s2-stack-web-r3.md:70` and `.foundry/go-rewrite/results/fold-go-s2-stack-web-r3.md:86`.

Make those two handoffs prerequisites for the first gates that need them. Use the same working directory, Compose files and project selection for all driver and installer Compose calls. Have the driver's C1 start before lookup and arrange for each standalone check and subshell to obtain the same running binding. Gate the two readers on a file-only nondefault port, then repeat with a named project and a conflicting root `.env`. Require the installer to use that binding for both the token command's project and the address it dials.

Category is design. The rollout-driver owner and the Stack Client planner own the receiving changes. The Compose divergence is confirmed live and the driver and installer paths are confirmed by reading. This is a required gate mismatch.

### logic-r4 5 — MAJOR — C4

One of the mandatory removal mutations has no effect on the session result, and the image mutation can leave the test's only image observation unchanged. The specified image defaults, command, assertions and mutations are at `.plans/plans/go-s2-stack-web/plan.md:92`, `.plans/plans/go-s2-stack-web/plan.md:244`, `.plans/plans/go-s2-stack-web/plan.md:259` and `.plans/plans/go-s2-stack-web/plan.md:262`. The Server plan treats missing and empty passwords the same at `.plans/plans/go-s2-server-signin/plan.md:487`.

By reading, deleting `-e LGTM_WEB_PASSWORD=` leaves the setting absent in the planned image, so the exported host password still stays out of the container and the purge commits. The test cannot detect this equivalent change. The launcher can also print the inspected identifier and pass a tag to Docker. If that tag happens to run a compatible image, the planned comparison against text printed before the run still passes. [Docker environment flag semantics](https://docs.docker.com/reference/cli/docker/container/run/#env)

Mutate the empty assignment to `-e LGTM_WEB_PASSWORD`, which imports the valid exported fixture password and prevents the committed purge. Observe the actual image argument in the Docker call, or inspect the one-shot container while it runs. For the tag mutation, use a controlled argument observation that stays independent of whether the daemon already has `lgtm-server:local`.

Category is a test gate defect. Stack owns the fix. The consequence follows from the documented runtime semantics and the stated test assertions. The required mutation gate is unscored.

### logic-r4 6 — MAJOR — D17

The encrypted-check handoff still leaves the combined link check with no endpoint. The conditional move is at `.plans/plans/go-s2-stack-web/decisions.md:53` and `.plans/plans/go-s2-stack-web/plan.md:392`. The Contract plan still assigns the real web server's encrypted join and mismatched-pin cases to sprint 3 at `.plans/plans/go-s0-contract/plan.md:753` and `.plans/plans/go-s0-contract/plan.md:758`. The Server plan moved its own browser check at `.plans/plans/go-s2-server-signin/plan.md:634`. The Machines plan now issues loopback-only tokens at `.plans/plans/go-s3-server-machines/plan.md:50` and `.plans/plans/go-s3-server-machines/plan.md:252`.

By reading, at the combined sprint 3 link check, a successful cleartext join cannot satisfy the required encrypted join or exercise the stored pin after restart. There is no encrypted endpoint to run the other half, and the condition that moves it has not been met. Record the split in Contract post-build check 2 before the combined gate is frozen. Keep the loopback join, restart and heartbeat cases there. Name an owner and a gate in the later proxy work for the deployed encrypted join and mismatched-pin cases. Keep component tests with local certificates where they already have a fixture and retain the separate owner decision about a machine certificate.

Category is design. The orchestrator and the Contract planner own the handoff. The outstanding requirement is confirmed by reading. This is a required gate mismatch.

### logic-r4 7 — MAJOR — C4

The post-build check measures a held lock without tying those seconds to the wait it claims to exercise. The new cases are at `.plans/plans/go-s2-stack-web/plan.md:373`. Removal stops the services before starting the maintenance process at `.plans/plans/go-s2-stack-web/plan.md:242`. The process opens the database, takes a serving lock and migrates before taking the session-table lock at `.plans/plans/go-s2-server-signin/plan.md:484` and `.plans/plans/go-s2-server-signin/plan.md:145`. The ten-second lock timeout is set at `src/server/internal/store/pool.go:311`.

By reading, a test can start its 15-second hold just before invoking the launcher. If shutdown and startup take six seconds before the session transaction begins waiting, only nine seconds of contention remain. A correct revocation can commit in a case that requires an unconfirmed result. The five-second case can similarly pass without any contention at all.

Acquire the conflicting table lock first, then observe the maintenance transaction waiting on that specific blocker before timing the release. Release after five seconds of observed contention in one case. Keep holding in the other until the maintenance call reports unconfirmed at the ten-second limit, with an outer timeout. Verify the original rows before releasing the lock and retry against the same database to prove a durable purge.

Category is a test gate defect. Stack and Server own the post-build check. The timing gap is confirmed by reading. The risk score is 4, from severity 1 for faulty gate evidence, fallout 1 for test-only impact and probability 4 for ordinary variation in stop and startup time.

## Answers to the assignment questions

1. **Round 3 patches.** G1 closes the circular gate by moving both Server acceptances to named tests and placing deployment at the sprint gate (`.plans/plans/go-s2-server-signin/plan.md:466`, `.plans/plans/go-s2-server-signin/plan.md:549`). G2 removes the Compose parse and vault bind from revocation, but findings 1, 2 and 5 cover the new selector, partial stop and proof. G3 removes root `.env` precedence for the launcher's default call and gives the vault suite a private empty file, while finding 3 covers the reintroduced raw diagnostic. G4 removes the ambiguous line parser. Live Compose rendering rejected `bad` and accepted `0` and a range. The guide excludes those latter forms and the server origin needs a usable port. The new port reader's consumers are covered by finding 4. G5 fixes the guide and request order, while the driver and installer handoffs remain open. G6 isolates the full stream fixture's image tags and verifies its running image and exact rule bytes with `exec -T`. G7 records the encrypted move condition, with one required receiver still open in finding 6. G8 fixes the Server browser split and names the Machines provider of the snapshot in item 11, with the joint Home and browser sign-out scenario in the Stack plan's post-build check 2. G9 fixes omitted false values and adds protected-call refusal after revocation. G10 correctly isolates the lock test from the rollout driver, with timing still open in finding 7. G11 removes the checkout-registering test. G12 exercises `Join` and names the widened matcher mutation. For the key decision summaries, shortened traceability, combined guardrail rows, execution sizing, attack-log row and paragraph joins, nothing found beyond the interactions reported above.
2. **Unreviewed mechanisms.** Live Compose 5.5.0 rendering confirmed that the `deploy/` working directory gives its default settings precedence, and that the shell and the settings file can still resolve project identity differently. The mechanism that defers settings checks removes port guessing, while raw output and the rollout-driver handoff remain in findings 3 and 4. The encrypted deferral has one unresolved consumer in finding 6. The older public-mode and superseded decision markers carry no new operational rule in this patch, and the public-mode interaction was checked against current consumers.
3. **Rejections.** Nothing found. The register contains zero rejected findings, and the fold result independently records zero at `.foundry/go-rewrite/results/fold-go-s2-stack-web-r3.md:23`.
4. **Writers and readers.** The fixture sets `LGTM_IMAGE_TAG`, the Compose model uses it for both images, and fixture consumers and cleanup share the selected tag. The Server plan emits the committed outcome and item 5 consumes it. Sprint 1 supplies the database keys and bind names and Server reads them. The missing agreements are the common project and file selector and the driver's and installer's port readers, plus the outstanding Contract encrypted-check handoff. The Server browser now uses request S12 as specified, and the Machines plan supplies the snapshot through items 11 and 14.
5. **Interactions.** The tables below cover service state, settings state and encrypted-check ownership. The contradictory cell is a running or unverified service reported as stopped. The missing ownership cell is the Contract check that still requires the encrypted endpoint.
6. **Round 3 findings still open.** G1 is closed by reading both Server acceptances. G2 still has a wrong-project and failed-stop case. G3 still exposes a password in the parser output on the supported private-file path. G5 and G7 depend on specific receiving edits that are absent. G10 is reduced to the separate lock-check timing and the outstanding driver work recorded in D4.
7. **Lens coverage.** The patch touches all eight questions through fixed time windows, settings and revocation failures, phase precedence, empty values, concurrent project identity and fixture tags, required mutations, weak observations and the shape of state selection.

| Removal state | Required result or gap |
|---|---|
| Correct project, services verified stopped, usable database, purge commits. | The command reports success. The old cookie fails after a restart with the original password. |
| Correct project, services verified stopped, database unavailable or purge unconfirmed. | The command exits nonzero. Serving stays stopped and retry is possible after database recovery. |
| Correct project, either service remains running or cannot be inspected. | The patch promises stopped services even though the stop is unverified. A purge attempt must wait for verified containment. |
| Normal launch resolves a file-selected project and removal falls back to the default. | The patch can check or purge the wrong installation, including a false success if the default project exists. |

| Input or receiving state | Full launch or joint gate | Early vault route or scoped check |
|---|---|---|
| The port setting is malformed but the settings file parses. | Compose refuses the model. | The fixed port takes precedence. |
| The settings file itself is malformed. | Compose refuses the model and may emit the value. | The fixed port cannot repair dotenv syntax. Removal can bypass the file once the correct target is known. |
| The port is nondefault and appears only in `deploy/.env`. | Compose uses the port. The current rollout driver still probes its default. | A published-port lookup works when it selects the same files and project. |
| A receiving plan has dated the deferral. | The real encrypted case belongs to the later proxy gate. | Its retained loopback or component fixture can run in wave 1. |
| A receiving plan still requires the encrypted deployment. | The combined gate has no supplier for that case. | Loopback success supplies only the loopback observation. |

## Answers to the logic lens

1. The patched phases have a clear order and a repeated successful removal converges on an empty session set. The session-lock scenarios need a clock anchored to the observed wait. Nothing found for counters, wrap or gaps because these patches add no persistent counter. The separate stream cut uses elapsed time since the last body read, and the new negative test checks both paths. [Caddy route timeout semantics](https://caddyserver.com/docs/caddyfile/directives/timeouts)
2. A taken port leaves the launched database, vault and server healthy for retry. A database or purge failure after a verified stop is unconfirmed. Wrong-target selection, a failed stop, and malformed settings output remain in findings 1 to 3. The inherited database-recovery suggestion needs a parseable settings file on the vault route.
3. The assignment tables cover the interacting removal and settings rules. For the proxy, `Open` gets the longer body-read idle window, adjacent `Join` and `WatchClients` keep the default, and a completed request body should allow a one-way response to continue.
4. An omitted `signed_in` boolean decodes to false at `src/contracts/proto/lgtm/v1/session.proto:33`. Empty and missing passwords both purge. An empty image tag selects `local`. Port `0` is accepted by Compose but is outside the guide's supported range. An empty shell project selector is unsafe where startup can take a project from the settings file.
5. Two launcher calls to the same canonical state folder share the sprint 1 lock, and the Server plan serializes database admission. The state lock alone does not resolve different interpretations of the Compose project. The full stream fixture now avoids competing for the operator's image tag.
6. For risky touched item 4, adding an external network to `server` or dropping the private-stack environment key conflicts directly with the rendered-model assertions. Its tests would miss an implementation that returns the same generated tag for two concurrent fixtures and cleans up the other fixture's tag. For risky item 5, the 120-second wait mutation conflicts with a 150-second server, and the bad-port mutation conflicts with Compose rendering. The two removal mutations need the changes in finding 5. For item 7, the exact timeout removal and widened matcher mutations target separate positive and negative stream scenarios.
7. Items 1 to 3 have no substantive test change in this fold. For item 4, rendering two tags sequentially misses a collision between concurrent fixtures. For item 5, a shell-only selector passes the exported-project removal cases, and printing an inspected image can conceal use of a tag. Item 6 has no direct test for the guide's port lookup, and the first real receiver checks belong at the joint gates. For item 7, an unlimited read on `Open` passes the current survival tests while violating the ten-minute cutoff. The exact timeout boundary needs the deployed check below.
8. One authoritative project selector removes the wrong-target state. Separate verified-stop and committed-purge outcomes remove the misleading failure message. Using the inspected immutable image identifier closes tag replacement once it is verified at the actual Docker invocation.

## Post-build checks

- **PB-R4-1. Removal against the installed stack.** Stack runs the real command with a live session, a missing vault credential file, a broken settings file and a healthy database. Verify the stopped service identifiers, actual maintenance image, one database bind, committed outcome and old-cookie rejection after restoring the original password. With a stopped PostgreSQL service, verify the command reports an unconfirmed purge and both listeners stay down. After database recovery, retry against the same rows and require a committed purge.
- **PB-R4-2. Lock contention during removal.** Stack and Server record the actual database waiter and its blocker for both timed cases. Require a commit after release inside the limit, an unconfirmed result with intact rows beyond it, and a successful retry after release.
- **PB-R4-3. The merged proxy's read window.** Stack runs the pinned image with a silent `Open` request body through the ten-minute boundary and with a second `Open` that sends the real client's regular heartbeats for longer than ten minutes. Require a cut near the configured idle limit in the silent case and a live stream in the heartbeat case. On the same image, verify the stalled ordinary calls end near the default while a separate normal sign-in succeeds.
- **Evidence limit.** The reviewed Stack Web plan, decisions and scouting match the requested patch tip. I ran Compose rendering and settings-failure probes without a daemon. Docker daemon access was denied in this session, so the deployed checks above remain for the build.

## Grade

5 out of 10. There are 0 blockers, 7 majors and 0 minors.
