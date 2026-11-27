**REVISE.**

The Machines page offers to reconnect an offline machine even though the server can send a restart only over a live machine connection.

The builder, the frozen test writer, the second test writer, and this reviewer are OpenAI workers. This review was independent of implementation and test writing. I reviewed item 4 on `sprint-0` at `23b26fe3` using `git diff 01ccb347~1..23b26fe3 -- src/frontend`. The task identifier is `postbuild-go-s4-web-repositories-4`, and its nonce is `def241`.

### post-4 1 — MAJOR — The offline machine menu offers an action the server cannot perform

**Statement.** Plan item 4 keeps actual machine actions. Foundation check C8 brings an offline machine back by starting the client again. [Plan item 4](../plan.md#L106) · [Foundation check C8](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L385)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. An offline watch row offered an enabled “Try to reconnect” action, and selecting it sent a `ReconnectClient` request. The inherited menu shows “Calling {machine}…” before sending it. The sample handler schedules an offline worker to reappear. By reading the pinned contract and Go handler, the procedure sends a restart down a held connection and refuses a disconnected client. A machine that stays offline therefore gets a request that cannot reconnect it and a toast that says the service is calling it. [Machine action](../../../../src/frontend/src/screens/service.tsx#L380) · [Sample handler](../../../../src/mock-server/src/engine.ts#L236) · [Contract](../../../../src/contracts/proto/lgtm/v1/clients.proto#L31) · [Server handler](../../../../src/server/internal/machines/commands.go#L31)

**Fix.** Hide the offline reconnect action and its toast in repository mode. Require the watched machine to be connected before offering a real restart. Keep the compatibility action.

### post-4 2 — MAJOR — The Machines table displays unreported load and contact times as facts

**Statement.** Plan item 4 uses watched machine facts. Foundation check C7 requires the machine to show its load. The contract represents load as optional and supplies the last contact time separately from the machine state. [Plan item 4](../plan.md#L106) · [Foundation check C7](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L384) · [Contract](../../../../src/contracts/proto/lgtm/v1/clients.proto#L59)

**Evidence and failure scenario.** Confirmed live in desktop Chromium. An online worker whose watch record omitted both load values displayed `0%` for processor and memory use. A worker whose offline record omitted the last contact time displayed `20784 days ago` with the browser clock fixed to 2026-11-27. The inherited projection fills all three absent fields with zero. The table formats those zeros as measurements and a date, even though the load meter already accepts an absent value and displays a dash. The pinned server can send absent load on an online record and can leave the offline timestamp absent. [Watch projection](../../../../src/frontend/src/server/sync.ts#L18) · [Table](../../../../src/frontend/src/screens/service.tsx#L372) · [Load meter](../../../../src/frontend/src/screens/service.tsx#L117) · [Server projection](../../../../src/server/internal/machines/project.go#L19)

**Fix.** Preserve absent load and contact times in the repository projection. Use the existing dash for absent measurements and format a supplied contact timestamp. Preserve a measured zero and the compatibility displays.

### post-4 3 — MINOR — Long watched paths hide the part that distinguishes two checkouts

**Statement.** The Machines route must show every watched folder, including multiple checkouts of one repository on a machine. [Plan item 4](../plan.md#L106) · [Machines route](../plan.md#L124)

**Evidence and failure scenario.** Confirmed live in Chromium. The watched path in the independent probe occupied 1,106 pixels of text and was clipped to 280 pixels on the phone and 181 pixels on desktop. The full string was in a `title` attribute on a nonfocusable span. Two folders with the same repository name and a long shared prefix can therefore look identical on the phone when the differing suffix is hidden. The frozen test uses short paths and checks element visibility, which succeeds even when an ellipsis hides the suffix. [Folder row](../../../../src/frontend/src/screens/service.tsx#L243) · [Clipping style](../../../../src/frontend/src/css/service.css#L23) · [Frozen path assertions](../../../../src/frontend/e2e/repository-routes.spec.ts#L397)

**Fix.** Let the watched path wrap inside its row, or give it a full-path disclosure that works with touch and keyboard. Test two paths that differ only after a long shared prefix.

### post-4 4 — MINOR — The amended installation checks accept extra sample text inside a watched row

**Statement.** Plan item 4 shows reported installations, and the two sample-switch cases are meant to prove that only the watched installation is displayed. [Plan item 4](../plan.md#L106) · [Amended switch cases](../../../../src/frontend/e2e/repository-discovery-review.spec.ts#L124) · [Amended session case](../../../../src/frontend/e2e/repository-session-review.spec.ts#L91)

**Evidence and failure scenario.** The two sample-switch cases kept their timing steps and moved the assertion to the correct route. They weakened the visible-text check when they replaced the exact text list with a row count and two checks for matching descendants. The third amendment strengthened machine attribution by checking both workers and requiring an empty list when the first omits its version. It uses the same partial-text check for installed rows. Confirmed live in scratch. Appending the sample string “Gemini CLI 0.71.1” inside the existing watched Codex list item left `140 passed (1.6m)` in the full browser suite. An independent browser assertion saw the full extra string after both switch timings at both widths. The second test writer's separate-row mutation tested a different case that the row count catches. [Watched row renderer](../../../../src/frontend/src/screens/service.tsx#L229) · [Switch assertions](../../../../src/frontend/e2e/repository-discovery-review.spec.ts#L143) · [Session assertions](../../../../src/frontend/e2e/repository-session-review.spec.ts#L113) · [Second test writer's mutation](../../../../.foundry/go-rewrite/results/s4-web-4s-tests.md#the-test-detects-a-sample-installation-in-machine-detail)

**Fix.** Assert the complete normalized text of each installation row. Also assert that the harness section contains no sample installation text outside its expected rows.

**Grade.** 7 out of 10.

## Post-build scenarios and visible text

- **Unreviewed changes.** By reading, the only unreviewed mechanism in the plan is the item 6 dialog retry. The latest planning register lists advisory post-build checks for compatibility, cookies, machine removal, and later deployed work. [Plan mark](../plan.md#L190) · [Latest planning register](round-1-findings.md#L42) · [Post-build checks](../plan.md#L281)
- **Compatibility and the advertising server.** Confirmed live in Chromium. A single bundle opened the in-page sample, standalone mock, and older-server fixture. The discovery tests exercised sample work and the simulation on all three paths. The advertising fixture waited for both snapshots, including on a worst-case query reload. A pending sample action, canned post, and harness installation stayed out of the watched workspace after switching. [Compatibility and switch cases](../../../../src/frontend/e2e/repository-discovery.spec.ts#L229) · [Readiness cases](../../../../src/frontend/e2e/repository-watch.spec.ts#L188)
- **Sample screens.** Confirmed live in Chromium. Against the pre-build commit `01ccb347`, 54 normalized text and control captures matched at each width across the three compatibility paths. The captures covered Home, Machines and its expanded rows, Harnesses, Notifications, Workflows, Usage, Appearance and the seven service sections, First run, the live connection menu, and the terminal chooser. The comparison used a fixed browser clock and normalized the fixture ports. [Compatibility setup cases](../../../../src/frontend/e2e/repository-routes.spec.ts#L451)
- **Watched routes and removal.** Confirmed live in Chromium. Worker-only and empty inventories opened the approved route bodies. A signed-out or expired setup link showed Login, waited for both snapshots after sign-in, and then opened the unavailable body. Both terminal entrances showed the unavailable body without a chooser or session. After the repository watch stopped, the connection menu showed Reconnecting and offered no simulated reconnect. An unscanned worker showed its reported version and a read-only agent limit without invented installations, version comparisons, or account status. A requested scan and later watch save displayed the reported version. Both removal orders dropped the removed machine's folders and kept the other machine's folder. The watch suite also covered the orphaned repository. [Route cases](../../../../src/frontend/e2e/repository-routes.spec.ts#L199) · [Machine and removal case](../../../../src/frontend/e2e/repository-routes.spec.ts#L352) · [Orphan and surviving folders](../../../../src/frontend/e2e/repository-watch.spec.ts#L239)
- **Session boundary.** Confirmed live in Chromium. Same-host discovery waited for the cookie writer. An independent hostname proceeded while the old request was held. The selected workspace kept its current address and authenticated watches. [Cookie order cases](../../../../src/frontend/e2e/repository-session.spec.ts#L291) · [Watch session cases](../../../../src/frontend/e2e/repository-watch.spec.ts#L373)
- **Copy.** By reading the complete frontend diff, the connection readout adds the declarative fallback “Connected to the service.” The unavailable, Notifications, and terminal bodies reuse existing wording. The repository-mode offline detail drops the command instruction. The newly reachable setup Login uses the expiry copy already listed in the plan. The inherited “Calling” toast is covered by post-4 1. I found no newly authored sentence that instructs the reader. [Builder's visible-text list](../../../../.foundry/go-rewrite/results/s4-web-4-build.md#visible-text-and-its-source) · [Copy table](../plan.md#L218) · [Connection readout](../../../../src/frontend/src/app.tsx#L325) · [Offline detail](../../../../src/frontend/src/screens/service.tsx#L183)
- **Deployed scenarios.** The real worker before its first scan and the exact deployed sprint checks R12 and R13 remain unverified at this revision. The pinned server composition does not register the project service. The deployed R14 observation and history-recovery scenario require their later items. The browser probes above used controlled watch records. [Pinned server composition](../../../../src/server/internal/app/start.go#L266) · [Deployed and recovery checks](../plan.md#L281)

## Commands and mutations

I used Node.js `v26.13.0` and pnpm `12.16.1` with `export PATH=~/.nvm/versions/node/v26.13.0/bin:$PATH`. Browser projects used Chromium at 1440 by 900 and 390 by 844 pixels. The scratch workspace used pnpm's dependency-verification override. The first run used a linked pnpm task-state directory, which prevented two desktop mock-dependent cases from starting. The subsequent run used a local task-state directory.

| Check | Last output |
|---|---|
| `pnpm exec playwright test --config src/frontend/playwright.config.ts --reporter=line`. | `140 passed (1.6m)`. |
| `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-discovery repository-session repository-watch repository-routes repository-navigation`. | `124 passed (1.6m)`, exit 0. The repository navigation cases belong to item 9 and are absent at this commit. |
| `pnpm --filter @lgtm/frontend typecheck`. | `$ tsc --noEmit`, exit 0. |
| `pnpm --filter @lgtm/frontend build`. | `✓ built in 806ms`, exit 0. Vite warned of a chunk above 500 kilobytes. |
| Remove the repository-mode Login exemption guard for direct setup and run `repository-routes --grep 'direct setup'`. | `2 failed`, exit 1. Both failures missed “Sign in to the server”. |
| Mount a notification delivery form beside the empty Notifications body and run `repository-routes --grep 'service links'`. | `4 failed`, exit 1. Each case found one form where zero were expected. |
| Append the sample harness text inside the existing watched list item and run the full Playwright suite. | `140 passed (1.6m)`, exit 0. The independent visibility assertion printed `4 passed (6.6s)`. |
| `git diff --check 01ccb347~1..23b26fe3 -- src/frontend`. | Exit 0 with no output. |

The startup check of the session tooling reported that its checkout was one commit behind upstream and that four skill feedback rows were open.

Suggested first project journal entry for approval. The repository-mode route review matched the sample screens across 108 browser captures and found offline reconnect and missing telemetry still exposed in the watched Machines table.
