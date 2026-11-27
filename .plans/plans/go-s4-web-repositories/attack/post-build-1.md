# REVISE

A delayed sample Git commit message writes a sample worktree entry into the real workspace after discovery has replaced the store.

OpenAI supplied the test writer, builder and reviewer. I did not write the frozen tests or build the code. I reviewed item 1, discovery and the initial workspace, on `sprint-0` at `59aee3f4`. The task identifier is `postbuild-go-s4-web-repositories-1` and its nonce is `eb96d7`.

## Post-build scenario and acceptance

The current plan and latest register have no item 1 row marked “UNREVIEWED FIX.” The relevant post-build row still calls for one bundle to exercise the three compatibility paths, the advertising server, and a pending sample action across a switch. I ran the controlled browser portion first. The deployed R13 check requires the repository watch and the readiness gate from item 3, so this item could only exercise the machine watch. [Plan](../plan.md#post-build-checks) · [Review register](round-1-findings.md#every-reported-finding-and-its-outcome)

- With discovery held, the frozen suite observed empty record collections and zero simulation intervals. It tested product and contract revision validation and the full service membership and sample precedence matrix.
- The in-page sample, standalone mock and older server each signed in, opened the sample project, tracked a repository, ran one simulation tick, retained that repository on reconnect and signed out. All three opened only the machine watch. I also compared the rendered desktop text of Home, the sample project's Git view, Harnesses, the Track repository dialog and Login against `be121813` with the in-page sample. All five matched.
- The advertising fixture opened with only its watched machine in the counted collections after a worst-case reload. The simulation stayed off, the draft stayed in memory and the appearance preference survived. The frozen pending request, board post and install stayed isolated after the real workspace was selected. The additional probes found two paths that escape that isolation.

## Findings

### post-1 1 — MAJOR — A delayed Git message writes into the real workspace

**Statement.** The plan requires delayed sample writers to stay with the workspace that scheduled them.

**Evidence and failure scenario.** The Generate button schedules a plain 900-millisecond timeout. Its setter writes through `update` to the active store. I clicked Generate in the sample Agent console project's Git view, signed out, switched to the advertising fixture and signed in before advancing the clock. The real workspace had no worktrees and its `gitUi` was empty. After 900 milliseconds it contained `gitUi["console-shell"].message` with the generated sample text. The frozen count helper omits `gitUi`. `src/frontend/src/screens/git.tsx:711`, `src/frontend/src/screens/git.tsx:718`, `src/frontend/src/app.tsx:733`, `src/frontend/e2e/repository-discovery.spec.ts:23`.

**Fix.** Bind the write to the originating sample workspace and assert that the real Git view state stays empty when the callback runs after a switch.

### post-1 2 — MAJOR — Sample harness installations survive a switch to the real workspace

**Statement.** The plan requires the new workspace to start with watched records and browser preferences. Foundation decision D1 puts shared product facts behind the contract.

**Evidence and failure scenario.** Sample initialization and the sample installer write to the module-wide harness catalog. Workspace replacement leaves it in place, and the real Harnesses screen reads the sample installation flags and versions. I queued a simulated Gemini install, signed out and held the next server's discovery reply. I advanced the clock by 2,200 milliseconds while discovery was held, then released discovery and signed in. The watched worker reported only Codex at version `1.2.3`. The real Harnesses screen showed Gemini as installed at `0.71.1` and Codex at the sample version `0.61.0`. The frozen test advances the clock after discovery and misses mutations that finish during discovery. `src/frontend/src/screens/service.tsx:54`, `src/frontend/src/screens/harness-catalog.tsx:47`, `src/frontend/src/app.tsx:707`, `src/frontend/src/screens/service.tsx:779`, `src/frontend/e2e/repository-discovery.spec.ts:335`.

**Fix.** Keep simulated installation facts inside the compatibility workspace. Read installation facts for the real workspace from watched machine inventory. Cover both an already completed sample install and one that finishes during a held discovery.

### post-1 3 — MAJOR — A discovery reply from an unmounted view replaces the legacy engine store

**Statement.** Item 1 requires the legacy live mount to keep its existing engine path.

**Evidence and failure scenario.** The default mount registers a workspace selector that assigns the global store when discovery returns. Unmount stops the interval and leaves the pending selector able to run. In Chromium I held an older server's discovery, unmounted the default view, mounted the legacy engine with a signed-in snapshot and then released discovery. The guarded legacy session went from signed in with zero projects to signed out with seven sample projects and a new simulation interval. The same probe on the base revision kept the guarded session and zero projects. `src/frontend/src/app.tsx:707`, `src/frontend/src/app.tsx:752`, `src/frontend/src/app.tsx:794`, `src/frontend/src/server/sync.ts:81`.

**Fix.** Invalidate the default mount's pending discovery and its selector on unmount. Verify that a delayed reply cannot replace the legacy store or start a simulation interval after a remount.

### post-1 4 — MINOR — The frozen suite accepts a callback guard without workspace identity

**Statement.** The plan requires the callback's originating store object to still be current.

**Evidence and failure scenario.** I replaced the guard with a check of the current mode alone in scratch. All 32 frozen tests passed. They release the clock in the real workspace before returning to sample. In a second Chromium probe I kept a sample installation pending through sample to real to sample and then advanced the clock. The committed guard discarded it. The weakened guard installed Gemini into the second sample workspace and failed the probe. `src/frontend/src/actions.ts:21`, `src/frontend/e2e/repository-discovery.spec.ts:349`, `src/frontend/e2e/repository-discovery.spec.ts:358`.

**Fix.** Keep one callback pending until a fresh compatibility workspace has opened, then assert that its records and harness inventory stay unchanged after the clock advances.

## Verification output

I used Node.js `v26.13.0` and a single scratch copy of the reviewed frontend. The package manager's dependency verification flag was disabled for the scratch copy. The source and frozen assertions came from the reviewed commit.

| Command or independent probe | Observed output |
|---|---|
| `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-discovery` | `32 passed (16.7s)` on the desktop and phone projects, exit 0. Chromium printed the environment warning about `NO_COLOR` and `FORCE_COLOR`. |
| `pnpm --filter @lgtm/frontend typecheck` | `$ tsc --noEmit`, exit 0. |
| `pnpm --filter @lgtm/frontend build` | `✓ built in 741ms`, exit 0. Vite printed its chunk-size warning. |
| Browser probes for the Git writer, harness catalog and remount | The probes reproduced findings 1 through 3. The compatibility text comparison matched on the five tested screens. |

Each planned mutation below independently printed `1 failed` and exited 1 in Chromium. I restored the source between runs.

| Mutation | Frozen assertion that failed |
|---|---|
| Seed and start the interval before discovery. | The held workspace had records and a timer at lines 127 and 128. |
| Check only the first service. | The exact service placed last selected the wrong mode at line 133. |
| Remove precedence for the discovery sample flag. | The flagged sample selected repository mode at line 133. |
| Remove precedence for the in-page sample address. | The sample readiness assertion at line 79 timed out. |
| Run delayed callbacks without a guard. | Real records, watched inventory, the shared catalog and asynchronous errors failed at lines 350 through 354. |
| Leave the interval handle set after clearing the interval. | Returning to sample failed to start one interval at line 364. |

**Grade.** 6 out of 10.
