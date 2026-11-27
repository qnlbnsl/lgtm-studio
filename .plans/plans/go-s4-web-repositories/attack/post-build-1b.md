# REVISE

A discovery reply from an unmounted sample view can send the next sign-in and machine watch to the sample server while the website shows the real server as selected.

OpenAI supplied the frozen test writer, builder, and reviewer. I reviewed their work independently of implementation and test writing. The review covers item 1 on `sprint-0` at `16db2726`, using `git diff 82fb8fdb~1..16db2726 -- src/frontend`. The task identifier is `postbuild-go-s4-web-repositories-1b` and the nonce is `54eb39`.

## Findings

### post-1b 1 — MAJOR — A late discovery reply can bind the real workspace to the sample transport

**Statement.** Foundation decision D1 and plan item 2 require the website to read facts through the contract of the selected server. Item 2 assigns transport publication to the current discovery generation. [Foundation decision D1](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L26) · [Plan item 2](../plan.md#L87)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. I held discovery from the sample server, unmounted the website, and mounted a standalone view that completed discovery against the advertising server. I released the old reply, then submitted the fixture password through the selected server's Login screen. The selected address stayed real, but the sample endpoint received both SignIn and WatchClients. Its Gemini worker appeared in the real workspace. The selected server received only discovery. The transport and base address are published before sync checks the selector. The builder identified that publication boundary as item 2 work. [Transport publication](../../../../src/frontend/src/server/api.ts#L43) · [Mount check](../../../../src/frontend/src/server/sync.ts#L81) · [Sign-in client](../../../../src/frontend/src/server/sync.ts#L125) · [Builder handoff](../../../../.foundry/go-rewrite/results/s4-web-1r-build.md#L25)

**Fix.** Return candidate clients from discovery and publish them only for the current mount and connection attempt. Extend the remount test to assert where the next sign-in and watch go and whose worker appears.

### post-1b 2 — MAJOR — A pending sample workflow clone appears in the real workspace

**Statement.** Item 1 requires delayed sample workflow writes to stay with their originating workspace. Item 4 assigns the real Workflows route an unavailable view. Its route gate and the write lifetime address separate parts of this failure. [Callback ownership](../plan.md#L84) · [Workflows route](../plan.md#L127)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. I cloned the sample Build loop and held the clone's fetch for the bundled workflow definition. I signed out, switched to the advertising server, signed in, and waited for its machine watch. The workflow storage was empty before and after the switch. After I released the fetch, the old clone wrote to browser storage, and the real Workflows list displayed its name. The sample dispatcher selects by legacy live mode alone, and the sample clone writes to the shared browser keys after awaiting the fetch. [Route dispatch](../../../../src/frontend/src/app.tsx#L657) · [Sample workflow storage and clone](../../../../src/frontend/src/screens/workflows.tsx#L29) · [Workflow request dispatch](../../../../src/frontend/src/screens/workflows.tsx#L77)

**Fix.** Bind the sample workflow operation to its originating store and check that identity before writing storage. Use the planned unavailable body in repository mode and keep existing workflow persistence for compatibility mode.

### post-1b 3 — MAJOR — A delayed sample sign-in takes over the new workspace's navigation

**Statement.** Plan item 2 requires a server switch to discard old sign-in results and preserve the current hash and authentication. [Plan session boundary](../plan.md#L90)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. I held SignIn on a sample-compatible server. Its Login screen still offered Change server. I switched to the advertising server, left it signed out, and opened the Workflows address. Releasing the old sign-in redirected the tab to Home and dispatched a machine watch on the new server without a successful sign-in there. Sync starts the watch after the old reply and the action and screen still apply that reply to the current session and route. [Sign-in completion](../../../../src/frontend/src/server/sync.ts#L125) · [Session assignment](../../../../src/frontend/src/actions.ts#L409) · [Login navigation](../../../../src/frontend/src/screens/auth.tsx#L56) · [Change-server button](../../../../src/frontend/src/screens/auth.tsx#L130)

**Fix.** Bind the sign-in and its completion to the selected session scope. Check that scope before starting a watch, applying the result, or navigating. Keep the plan's ordering for sign-in and discovery on one hostname.

### post-1b 4 — MINOR — The frozen harness tests accept the wrong version when two machines report one harness

**Statement.** The builder documents the first available version in machine order as the choice when several machines report the same harness. The frozen fixture sends one worker and one version. [Builder handoff](../../../../.foundry/go-rewrite/results/s4-web-1r-build.md#L17) · [Frozen fixture](../../../../src/frontend/e2e/repository-discovery-review.spec.ts#L18)

**Evidence and failure scenario.** Confirmed live in scratch. I changed the existing harness version assignment to overwrite on every later machine. All 42 frozen tests still passed. By reading, with two machines reporting Codex versions, the mutated screen would display the last machine's version. [Version choice](../../../../src/frontend/src/screens/service.tsx#L749) · [Version assertion](../../../../src/frontend/e2e/repository-discovery-review.spec.ts#L142)

**Fix.** Give the harness fixture two watched machines with different versions and a case where the first report omits the version. Assert the chosen version in the rendered row.

## Prior findings and the post-build scenario

The current plan and review register contain no item 1 entry marked “UNREVIEWED FIX.” I ran the browser portion of the item 1 post-build scenario. Both sample transports and the older-server fixture signed in, opened a sample project, tracked a repository, simulated a turn, retained the repository on reconnect, and signed out. Each opened only the machine watch. The advertising fixture selected the empty workspace with the worst-case query, loaded the watched machine, kept drafts in memory, kept the appearance preference, and ran zero simulation intervals. The frozen switch case discarded the pending request reply, canned post, and harness install. I left the deployed sprint check R13 unrun because the reviewed revision lacks the repository watch and the two-snapshot readiness gate supplied by item 3. [Review register](round-1-findings.md) · [Post-build scenario](../plan.md#L281) · [Item 3](../plan.md#L96)

| Prior finding | Confirmed live at both widths |
|---|---|
| `post-1 1`, delayed Git message | Closed. Advancing 900 milliseconds after switching left the real worktree and Git view state empty. The Generate callback now uses the originating store guard. |
| `post-1 2`, sample harness installation | Closed for installation facts. Completing the sample Gemini install before switching or during held discovery left the real Harnesses page with only the watched Codex installation at version `1.2.3`. The disabled defaults still show built-in harness choices. Item 4 owns the unavailable Harnesses body. |
| `post-1 3`, unmounted discovery replacing a legacy store | Closed for the reported legacy mount. Releasing its old discovery kept the same guarded store, signed-in session, zero projects, and zero simulation intervals. Finding 1 covers the remaining transport publication. |
| `post-1 4`, callback identity coverage | Closed. A delayed Gemini install from the first sample left a second sample's machine inventory and Harnesses screen unchanged. Removing the identity check made the new frozen assertion fail. |

I also read the remaining plain timers and mutable module state. Issue 1682 still applies to the Files draft map by reading. There is no watched worktree for that map to match in the reviewed revision. The repository Design canvas can retain a reference after a zoom and unmount, as the builder observed. By reading, the reviewed revision has no watched repository to render it, and item 9 plans an unavailable Design body for real repositories. [Issue 1682](../../../../docs/issues.md#L7965) · [Design canvas channel](../../../../src/frontend/src/screens/design.tsx#L551) · [Real Design route](../plan.md#L135)

## Verification output

I used Node.js `v26.13.0` and pnpm `12.16.1` with the installed dependencies copied into one scratch checkout. The first browser run printed a read-only package-store warning during automatic dependency verification and still passed. I disabled that automatic step in subsequent scratch commands. Chromium reported the environment's color flag warning. The Vite build reported its chunk-size warning.

| Command or probe | Observed output |
|---|---|
| `sha256sum -c .foundry/go-rewrite/frozen-s4-web-1.sha256 .foundry/go-rewrite/frozen-s4-web-1r.sha256` | All five files printed `OK`, exit 0. |
| `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-discovery` | `42 passed (30.6s)` on desktop and phone, exit 0. The rerun after restoring the mutations printed `42 passed (30.8s)`, exit 0. |
| `pnpm --filter @lgtm/frontend typecheck` | `$ tsc --noEmit`, exit 0. |
| `pnpm --filter @lgtm/mock-server typecheck` | `$ tsc --noEmit`, exit 0. |
| `pnpm --filter @lgtm/frontend build` | `✓ built in 748ms`, exit 0. |
| Independent browser probes | The run with the workflow, sign-in, and sample comparison probes printed `6 passed (7.0s)`. The late-discovery transport probe printed `2 passed (4.2s)`. The probes asserted the observed defects as well as their preconditions. |
| Sample screens against `313b1bec`, the parent of the repair test commit | `12/12 identical` at each width. I compared rendered text and visible controls or values on Home, All projects, the sample project's Git and design preview, repository Design, Harnesses, Machines, Setup, Workflows, Defaults, Track repository, and Login. This was a browser text and control comparison. |

Each required mutation below ran separately in desktop Chromium and printed `1 failed`, exit 1. I restored the scratch source after each run.

| Mutation | Frozen assertion that failed |
|---|---|
| Seed data and start simulation before discovery. | The held chooser contained records and a simulation interval at discovery spec lines 127 and 128. |
| Read only the first advertised service. | The exact service placed last selected compatibility mode at discovery spec line 133. |
| Ignore the discovery sample flag. | The flagged sample selected repository mode at discovery spec line 133. |
| Ignore the in-page sample address. | The in-page sample did not reach the expected mode and sign-in state at discovery spec line 79. |
| Run delayed sample callbacks without a guard. | The pending actions changed real records, watched inventory, and the shared catalog at discovery spec lines 350 through 352. |
| Clear the simulation interval without resetting its handle. | Returning to sample failed to start one interval at discovery spec line 364. |
| Guard by mode without the originating store identity. | Gemini appeared in the second sample's inventory and screen at repair spec lines 214 and 215. |

**Grade.** 7 out of 10.
