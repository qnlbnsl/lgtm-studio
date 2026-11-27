---
topic: go-s4-web-repositories
status: attack
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# Add a repository on a real machine from the website

## What changes when the website connects to a real server

An owner signed in to a sprint 4 server can inspect the branch, remote and status of a folder, add it from the sidebar or Machines, and find it in the tree after reloading. Startup reads discovery before choosing the records to load. The real workspace opens after the machine and repository watches both send their first lists.

The owner approved five visible changes on 2026-11-27. A dirty checkout gets its own status. New project explains that projects are unavailable. The sidebar plus opens Add repository. Without a machine, both dialogs show an empty choice and disable submit. Machines shows a status and reason when saved history is behind the server.

## Key decisions

One build still opens the in-page sample, standalone mock and older server, with their sample workflows.

The repository watch supplies the tree. After a successful add, the dialog waits briefly for the saved folder on the live watch. If the watch stalls, the form keeps its inputs and can check again without saving twice. Sign-out, expiry and a server change end the watches and drop their records and unfinished actions.

[Decision log](decisions.md)

## Workstreams

### Open a workspace from the server that answered

Both lists refresh after a broken connection. The tree and search open real repositories, and an old address for a removed repository shows the existing missing-record page.

### Inspect and add a folder from either entry point

The owner can open the same folder dialog from the sidebar, Machines, or New project. It shows facts for the current input and keeps the goal when returning to New project. A retry uses the same request key within the server's retention guarantee. Both dialogs are checked on a phone-wide screen.

### Show the machine recovery state

The machine watch supplies the diagnosis. Reopening it, receiving an unrelated update or resolving a delayed approval leaves that diagnosis in place.

## Rollout order

1. Take the generated contract and build discovery and session cleanup.
2. Add the repository watch and real routes. Check the real workspace and compatibility in one build.
3. Wire the dialogs and test save ordering and goal restoration at both widths.
4. Add recovery last. Deploy after real routes, both folder entry points and recovery are ready. Verify them on the merged stack with a real machine. (architect-r1 01)

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Workstream 1 (WS-1). Open a workspace from the server that answered**

Discovery chooses the workspace and the session owns both watches. The legacy embedding entry point uses its existing path.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | One bundle opens both samples and the old server. The advertised real mode starts empty with no simulation, even with `?data=worst`. | `discovery chooses the workspace and simulation` and stack R13. |
| 2 | Sign-out, machine watch expiry and changing servers drop the machine records and dialogs. Obsolete replies cannot restore them or change the address. | `session boundaries discard late replies` and `stale machine actions cannot replace a watch`. |
| 3 | Both snapshots open the shell. Reconnect replaces the list. Sign-out, expiry and switching clear both watches and their records. | `workspace waits for both snapshots`, `repository reconnect replaces and removal converges`, `either watch ends the repository session` and stack R12. |
| 4 | Shell and service routes use the listed bodies, including Login for a signed-out setup link and read-only watched machine facts. | `machines-only routes have no sample data`. |
| 9 | The repository routes, tree and search follow watched records and the route table. | `saved repositories open from tree and search`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Defer the sample seed and simulation until discovery chooses a compatibility workspace. | `src/frontend/src/app.tsx` `mount` and `mountLive`. `data.ts` `base`, `emptyState` and `seed`. `actions.ts` `startSimulation`. |
| 2 | Fence startup, watch and action completions by the active server and session. | `src/frontend/src/server/api.ts` `openServer`. `server/sync.ts` `connect`, `watch`, `signIn` and `signOut`. `screens/service.tsx` `PairMachine` and `Machines`. |
| 3 | Add the generated repository watch and project its records into the existing tree data. | `src/frontend/src/server/sync.ts` `watch`. `types.ts` `Repo`. New `src/frontend/src/server/repositories.ts` follows `server/sync.ts` `toMachine` and its replace, save and removal cases. |
| 4 | Route the shell and service screens with only watched machine facts. | `src/frontend/src/app.tsx` `App`, `toggleTerminal` and `NotConnected`. `screens/service.tsx` `MachineDetail`, `Notifications` and `ServiceSettings`. |
| 9 | Route repository screens and search. | `app.tsx` `ProjectTree`. `screens/repo.tsx` `RepoView`. `screens/center.tsx` `RepoCenter` and `Checkouts`. `dialogs.tsx` `Switcher`. |

Short anchor paths are relative to `src/frontend/src/`. Browser tests enter through `main.ts` using one unpinned Vite bundle, generated messages and controlled Connect replies. Use explicit barriers to release responses, record request order and observe aborts. Run the in-page engine and the standalone mock with `pnpm --filter @lgtm/mock-server start`. Use the repository's pinned Node.js version. Item 5 imports its module directly. Freeze assertions before editing production files. Stubs may expose only the planned shapes. The merged stack supplies the real Go server, PostgreSQL and machine.

For item 2, map two hostnames under one test site to loopback in Chromium. Serve the same bundle from the first hostname. Use two Hypertext Transfer Protocol (HTTP) listeners on that hostname at different ports with one fixture session registry. A third listener on the other hostname has its own registry. The fixture uses host-only, HTTP-only, `SameSite=Strict` cookies at `Path=/`, the exact credentialed page origin and Connect preflight headers. Prove each listener receives its expected cookie on a protected request before staging a race. (critic-r1 10)

**Item 1. Discovery and the initial workspace.** Change `src/frontend/src/{app.tsx,data.ts,actions.ts,types.ts,define.ts}`, `src/frontend/src/server/sync.ts` and `src/frontend/src/screens/{service,mocks,terminal,workflow,messages,harness-catalog}.tsx`. Create `src/frontend/playwright.config.ts`, `src/frontend/e2e/support/repository-server.ts` and `src/frontend/e2e/repository-discovery.spec.ts`. Use the root Chromium conventions with the Website's own tests. Foundation D1, rules 2 and 7 and C1 to C4 apply. This item enables newer R13.

- Start the default mount with `emptyState` and reuse the empty slice setup beside `mountLive`. Discovery runs the existing sample initializers only on compatibility paths. Keep their existing navigation, timers, persistence and sign-out. The real path uses in-memory drafts and browser presentation preferences. It starts with empty machines, joining machines and repositories for the watches. Projects, teams, agents, requests, tasks, epics, sprints, threads, channels, posts, subscriptions, notifications, usage, workflow, design, worktrees and terminal sessions stay empty.
- After product and `CONTRACT = 1` validation, choose compatibility for the in-page address or `sample=true`. Otherwise use exact, case-sensitive membership of `lgtm.v1.ProjectService` in `services`, ignoring every other entry. A list without that name selects compatibility. Discovery writes `ServerState.repositoryMode`. The shell and sync read it. The legacy live mount keeps its own path. (architect-r1 02, critic-r1 1, logic-r1 08)
- Add `stopSimulation` beside `startSimulation` in `actions.ts`. Workspace dispatch calls it to clear the interval and reset its handle when the workspace departs. A compatibility reconnect keeps its workspace. Use the planned `sampleLater` wrapper in `actions.ts` to capture the sample store object and run a callback only while that object is current and in compatibility mode. The wrapper's readers are the delayed sample writers for project, request and Define replies, repository setup, saved notices, service restart, notifications, terminal, workflow and design, plus `sendPost`, `nudge` and the simulated harness `install`. Guard the installation before either catalog or machine mutation. Keep view-local timers with their components. (architect-r1 10, critic-r1 2, logic-r1 01)
- First write `discovery chooses the workspace and simulation` in `repository-discovery.spec.ts`. Hold discovery and assert an empty store. Try mixed lists in both orders, a duplicated exact entry, unknown-only, lookalike-only, absent and empty lists, and sample with the exact entry. Use `?data=worst` before the hash. Real mode has no sample record or 4,200-millisecond interval. On both samples and the old server, check the normal entry, sign-in, sample project, Track repository and simulation. Hold a project or request callback, a canned post and a harness install across a switch. Advance the clock and require no real-store, watched inventory or shared catalog mutation and no asynchronous error. Return to sample and observe exactly one interval. This ordering is risky. Mutate eager seeding, membership, sample precedence, the callback guard and interval reset. Require red assertions. (architect-r1 02 and 10, critic-r1 1 and 2, logic-r1 01 and 08)

**Item 2. Server and session lifetime.** Change `src/frontend/src/server/{api,sync}.ts`, `src/frontend/src/actions.ts`, `src/frontend/src/screens/service.tsx` and `src/frontend/e2e/support/repository-server.ts`. Create `src/frontend/e2e/repository-session.spec.ts`. Start after item 1. Foundation D1, D11, rules 7 and 8 and C3 to C7 apply. This item enables newer R12 to R14.

- Have `openServer` return validated discovery and candidate clients. Install them and remember the address only on the current discovery generation. Sync owns the chosen clients and abort controller. Sync's `captureSession` supplies clients, signal and `isCurrent()` to the watches, machine actions and item 6. Check scope before committing a frame, response, retry or connection error, including already queued results.
- In repository mode, invalidate the scope before awaiting sign-out, changing servers or accepting current `Code.Unauthenticated` from a protected machine call or the machine watch. Abort the watch and clear both machine collections, initial readiness, expansion, record selection and panels, dialogs and intents, toasts, drafts and derived records immediately. Items 3 and 6 extend that same path to repositories and folder calls. Keep appearance and navigation preferences and the compatibility cleanup behavior. A new sign-in opens a new scope. (architect-r1 01)
- Sync serializes credentialed discovery dispatch and cookie-writing sign-in and sign-out by server hostname, across ports. Cookie writers have a six-second network deadline. Check discovery's generation again before sending a queued probe. A different hostname has an independent queue. The browser fixture records the cookie observed on the discovery request itself. (logic-r1 05, critic-r1 10)
- Pairing uses the same captured clients for token creation and revocation. Guard delayed cleanup and machine action completions. In real mode `WatchClients` owns both machine collections. Approval may close only its own pairing dialog. Removal skips sample agent and subscription edits. An old completion cannot reopen a dialog, change expansion or show a toast.
- First write `session boundaries discard late replies` in `repository-session.spec.ts`. Hold the machine stream, a protected action and discovery across machine-watch expiry, stalled sign-out and server change. Observe that stream's abort and immediate empty records and dialogs. Release old results after abort and assert the selected address, hash, authentication and current records survive. On one hostname across ports, hold each cookie writer before its response. Require discovery to start after it and to observe the new cookie after sign-in or no cookie after sign-out. Hold sign-out and verify a queued sign-in still leaves its next machine stream authenticated. Repeat the old-result races on distinct hostnames and require independent progress. Item 2 merges with `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-session` passing against the machine watch. (architect-r1 01, critic-r1 10, logic-r1 05)
- Write `stale machine actions cannot replace a watch` in the same file. Release held approval after a newer save and removal. The row stays absent and the replacement dialog stays open. Release an old removal failure and pairing-token reply after switching. Expect no obsolete toast and no revoke at the new server. Risky mutations remove a generation guard, defer cleanup until after sign-out or let approval write a machine row. Require red assertions.

**Item 3. The repository watch.** Change `src/frontend/src/server/{api,sync}.ts` and `src/frontend/src/types.ts`. Create `src/frontend/src/server/repositories.ts` and `src/frontend/e2e/repository-watch.spec.ts`. Start after item 2 and the generated contract. It can run alongside item 5. Foundation D1 and D5, rules 2, 5, 7 and 8, and C7 to C9 apply. This item owns the browser projection of newer R12 and R13 and makes R14 observable.

- The Website's `api.ts` creates `Api.projects` with the generated service and selected transport. Sync reads it for `watchRepositories` and item 6 reads it through `captureSession` for the two folder calls. Sync writes `ServerState.synced` after both initial snapshots in repository mode, including empty snapshots. The shell reads it. Apply `all` as replacement, `saved` as full upsert and `removedId` as removal. Ignore a delta before the attempt's first snapshot and frames queued by a replaced attempt. Reopen after loss or `aborted` with the existing 500-millisecond to eight-second backoff. Retain visible rows during retry. Derive live connection status from both attempts.
- The projection in `repositories.ts` writes the facts and complete `Repo.folderRecords` for Machines and Checkouts. Preserve `Repo.folders` for existing consumers and frozen R12, choosing the first folder by identifier per machine. Preserve absent values and the omission bit. Sync extends item 2's reset to drop the repository list, both readiness marks and both attempts when either watch expires, or on sign-out or server change. (architect-r1 01)
- Sync publishes `currentRepositorySnapshot()` for item 6. It returns `{ openedAt: number; repos: readonly Repo[] }` only while the current repository attempt has accepted its first full frame. Sync stamps the opening from the same elapsed clock as the dialog's reply timestamp. The caller reads it after each store update and compares both identifiers and the folder's machine. The retained store list alone cannot settle a save. Sync also provides `refreshRepositories(): void` for item 6 to restart the repository watch once in the current scope. The result arrives through the same projection. (logic-r1 02, architect-r1 03)
- First write `workspace waits for both snapshots`, `repository reconnect replaces and removal converges` and `either watch ends the repository session` in `repository-watch.spec.ts`. Use mixed discovery lists in both orders and observe both watch requests with no sample or timer. Release the initial snapshots in both orders, with an empty list in one run. The deep link stays busy until both. Apply saved replacements and removals, then miss a removal on `aborted` and replace from an empty snapshot. Release a queued old frame after replacement. Check distinct folder and machine identifiers, stable multi-folder order and no fresh connection badge from a heartbeat while its sibling is down. In separate runs expire each watch, sign out and switch servers with both open. Require both aborts and immediate empty records and dialogs. Risky mutations accept one initial snapshot, merge `all`, append `saved`, accept an obsolete frame or mark live from one watch. Require red assertions. Item 3 merges with `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-watch` passing. (architect-r1 01 and 02, critic-r1 1, logic-r1 02 and 08)

**Item 4. Shell and service routes with only machine data.** Change `src/frontend/src/{app,ui}.tsx` and `src/frontend/src/screens/service.tsx`. Create `src/frontend/e2e/repository-routes.spec.ts`. Start after item 3. Foundation D1 and rules 2 and 7 apply. This item enables newer R12 to R14. (logic-r1 11)

- In repository mode, apply Login and initial-snapshot gates before dispatching `/setup`. Signed-in setup uses the existing unavailable body. Keep the compatibility exemption. Item 4 moves `NotConnected({ what })` to `ui.tsx` for the shell, settings and item 9. Use it in the terminal area for both shortcuts without mounting a terminal or chooser. (architect-r1 05, critic-r1 3)
- Use the route table's shell and service cases. Machine detail shows complete watched folders, reported installations and a read-only agent limit. Keep actual machine actions. Exclude synthetic version comparisons, commands to install a harness and inferring account status. Use the existing empty Notifications body without its sample controls. The connection menu keeps its readout and cannot invoke simulated reconnection.
- First write `machines-only routes have no sample data` in `repository-routes.spec.ts`. Check shell and service rows of the matrix by links and hash reload with a worker and no host, and with both lists empty. Open setup directly signed out and expired. Observe Login, then the busy gate after sign-in, then the unavailable body after both empty snapshots. Assert no setup effects. Check the compatibility setup path against the same baseline branches. Exercise both terminal entrances, phone drawer and desktop. Check unavailable views' named missing controls and no unsupported request or store mutation. Mutate the setup exemption or leave Notifications forms mounted beside the empty message. Require red assertions. (architect-r1 05, critic-r1 3, logic-r1 11)

**Item 9. Repository screens and search.** Change `src/frontend/src/{app,dialogs}.tsx` and `src/frontend/src/screens/{repo,center}.tsx`. Create `src/frontend/e2e/repository-navigation.spec.ts`. Start after item 4. Foundation D1 and rules 2 and 7 apply. This item enables newer R12 to R14. (logic-r1 11)

- Use watched identifiers for tree and Switcher links, including when a sample-only tree filter was selected. Keep repository tabs. Use the route table's repository cases before `RepoSetup` or synthetic `repoFacts` can run. Checkouts reads the full folders and the omission bit from item 3. Checkouts and Switcher use the existing dash for withheld origin and their own existing copy for known absence. Both display a present origin as text. (architect-r1 09, logic-r1 10)
- First write `saved repositories open from tree and search` in `repository-navigation.spec.ts`. Check missing, folderless, saved and removed states from links and hash reload. Find watched records in the tree and keyboard switcher. Exercise each repository route family in the table, including the unavailable bodies and absent sample actions. Observe all resolved paths and all three origin cases in Checkouts and Switcher. Test the narrow drawer and repository tabs at 390 by 844 pixels and desktop. A fixture that marks an origin withheld must show the dash in both views. (architect-r1 09, logic-r1 10 and 11)

**Routes after both snapshots**

The first column follows the hash dispatcher. An unknown page uses the existing unknown-page view. A listed unsupported route uses the existing `NotConnected` with the subject in the copy table. Requested addresses wait at the initial gate before showing a missing record.

| Address or entry | Machines and no repositories | After a repository is saved |
|---|---|---|
| Startup and connection chooser before sign-in | Use the existing Connect or fixed-address probe screen, then the existing sign-in and expiry screens. | The same gates apply. |
| `/login` | Use the existing Login screen and change-server action where offered. | The same screen applies. |
| `/`, `/home`, `/home/<type>`, `/home?type=…` | Root redirects to Home. The attention table shows its existing empty result with or without a filter. | The filters may list real repositories. The requests table stays empty. |
| `/home/projects` and `/projects` | The old address redirects to All projects, with zero projects and its existing empty table. The New project entry opens the existing dialog. | The projects table stays empty. A tree-row or repository-center project entry carries the repository identifier. |
| `/machines` | Show Machines, pairing and existing machine actions. The detail has no repositories or projects. Agent limit is read-only. No simulated harness command or account state appears. Zero machines gives the existing empty table and Add machine. | The opened row lists every watched folder for that machine. |
| `/harnesses` and `/subscriptions` | Keep the alias behavior and unavailable Harnesses body. No Add harness or account sign-in mounts. | The same view applies. |
| `/notifications` and `/settings?section=notifications` | Keep the alias behavior and No notifications body. No phone offer, delivery form, event switch, quiet-hours form or dismiss action mounts. | The same view applies. |
| `/workflows` | Use the unavailable view inside Settings. No sample run control mounts. | The same view applies. |
| `/usage` | Use the unavailable view. | The same view applies. |
| `/settings`, `/settings?section=appearance` or an unknown section | Open the existing Appearance preferences. | The same view applies. |
| `/settings?section=defaults`, `remote`, `stall-watch`, `workflow`, `git`, `connections` or `about` | Keep section navigation. Use the unavailable body without a server settings form, check or restart action. | The same view applies. |
| `/setup` | Login when signed out or expired. After signing in, wait for both snapshots and show the unavailable body. No First run steps mount. | The same view applies. (architect-r1 05, critic-r1 3) |
| `/p/<id>`, and each Project view `center`, `board`, `end-result`, `questions`, `status`, `actions`, `design`, `files`, `git`, `settings`, `conversation`, `sprint`, `task`, `team`, `chat`, `message-board`, `agents`, `map` and `terminal`, including child and older addresses | Use the existing missing-project view. No project shell or lead column mounts at any width. | The same view applies until real projects arrive in sprint 5. |
| `/r/<id>` and `/r/<id>/center` | Use the existing missing-repository view. | For a saved repository with a folder, show the existing Command center. Projects and Open requests use their empty bodies. Checkouts uses real folders and remote data. Plan, Sprints merged, Full runs, Usage by project and Base branch use unavailable bodies. |
| `/r/<id>/board` | Use the existing missing-repository view. | Keep Board and show its unavailable body without a sample task board. |
| `/r/<id>/design`, `design/screens`, `design/screen/<id>`, `design/file[/…]`, `design/browser` and `design/architecture`, including older design-file links | Use the existing missing-repository view. | Keep the Design tab and subnavigation. Use the unavailable body without a design canvas, map or preview. |
| `/r/<id>/files[/…]` and `/r/<id>/git[/…]` | Use the existing missing-repository view. | Keep Files and Git tabs and show their unavailable bodies without editors or branch history. |
| `/r/<id>/terminal`, the sidebar Terminal button and its keyboard shortcut | The address remains a missing repository. The button and shortcut open the existing unavailable view. | The direct address uses the unavailable view for the saved repository. The button and shortcut keep the same behavior. |
| Any `/r/<id>[/…]` after a folderless save or after removal | Use the existing missing-repository view when absent. | A still-saved folderless record uses the unavailable view. A removed record uses the missing-repository view. |
| Switcher, repository tree and any other address | Switcher has its existing empty search result and page matches. The tree shows the existing empty content. Unknown top-level addresses use the existing missing-page view. | Switcher and tree open only the saved identifiers or existing page destinations. No repository agent column mounts, whether collapsed or wide. |

**Design artifact** [Approved website behavior and changes](../../../docs/specs/2026-11-27-running-an-agent-design.md#the-website-in-sprint-4). [Bounded assignment and view inventory](../../../.foundry/go-rewrite/tasks/plan-go-s4-web-repositories.md).

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Old seeds, timers, or a delayed watch write cross into another server's workspace. | High | Select the data source after discovery. Invalidate scope before cleanup and test delayed callbacks across each boundary. |
| A snapshot lost while disconnected leaves stale rows or a missing-machine crash. | High | Replace snapshots, gate both initial reads, and open every supported route with a worker-only and a zero-machine fixture. |

**Verify** Run `pnpm --filter @lgtm/frontend typecheck` and `pnpm --filter @lgtm/frontend build`. Run `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-discovery repository-session repository-watch repository-routes repository-navigation` at desktop and 390 by 844 pixels. The disposable stack runs `deploy/test/checks-s4.sh --only R12,R13` and requires both passes and exit 0. R12 reads `window.lgtm.store.state.repos[].folders`. R13 wraps `createRouterTransport` from `@connectrpc/connect` in `src/frontend/src/server/api.ts`. Both samples discover, sign in and open only the machine watch. Advertising real mode uses revision 1, signs in, opens both watches and reaches synced with no sample projects. (architect-r1 04, critic-r1 8, logic-r1 09)

**Sprint 5 extension point.** Reuse discovery, the authenticated scope, readiness and the watch identifiers for project, agent and lead-thread watches.

**Rollout detail** The frozen helper `deploy/test/checks-s4-browser.mjs` runs one R13 bundle in three fresh contexts. Website's suite adds the older real server. (architect-r1 04, logic-r1 09)



**Workstream 2 (WS-2). Inspect and add a folder from either entry point**

The server decides whether the folder exists and belongs to the selected repository. The browser reads the saved record from its watch.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 5 | The captured request and key stay equal for a retry and change for a new intent. Late responses cannot reach a different input or session. | `folder procedures keep the intent key` and `inspection follows the latest input`. |
| 6 | The folder dialog shows the server's three rows and handles save, refusal, expiry and removal. The returned repository and folder join the watch in either arrival order. | `inspection follows the latest input`, `add joins the watched identity in either order`, `add refusal and expiry preserve or clear the form` and stack R14. |
| 7 | The sidebar and Machines open the folder dialog. New project keeps its goal on return, gives the unavailable response without a lead, and shares the zero-machine behavior. | `add uses the sidebar or the opening machine`, `new project keeps the goal and reports unavailable` and `both dialogs have an empty machine choice`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 5 | Give the two generated calls a browser intent with a stable request key and a current-scope check. | New `src/frontend/src/server/folders.ts` follows `server/sync.ts` `createClientToken` and `reason` and `server/api.ts` `api`. |
| 6 | Connect inspection, save and watch reconciliation in Add repository. | `src/frontend/src/screens/service.tsx` `AddRepository`. `ui.tsx` `Dialog` and `DialogHost`. `server/sync.ts` `captureSession` and the item 3 refresh. |
| 7 | Route the sidebar plus to Add repository and share machine defaults with New project. | `src/frontend/src/app.tsx` `Sidebar` and `ProjectTree`. `dialogs.tsx` `NewProject`. `screens/service.tsx` `Machines` and `MachineDetail`. `project-lead.tsx` `useProjectLead`. |

**Item 5. Folder request identity and cancellation.** Create `src/frontend/src/server/folders.ts` and `src/frontend/e2e/folder-calls.spec.ts`. Start after item 2 and the contract. This module can run alongside items 3 and 4. Foundation D1, rules 2, 7 and 8, and newer R1 to R7 and R14 apply as call boundaries.

- Item 5 exports `folderIntent(input)` from `../src/server/folders` for its test and the dialog. `input` is `{ procedure: 'inspect' | 'add'; clientId: string; path: string; repositoryId?: string }`, with the optional target accepted only for add. Its `FolderIntent` result keeps these immutable values and a fresh opaque `key: string` from browser randomness. Each dialog activation, procedure or input change creates a new intent, including editing away and back. The wrapper preserves the exact path and omitted target. (architect-r1 07, critic-r1 9)
- Item 5 exports `inspectFolder(intent, attempt)` and `addRepository(intent, attempt)` for item 6. `attempt` is `{ client: Pick<Client<typeof ProjectService>, 'inspectFolder' | 'addRepository'>; signal: AbortSignal; isCurrent: () => boolean }`. Forward `Lgtm-Request-Key` and the signal to the matching generated request. Return `Promise<InspectFolderResponse | undefined>` or `Promise<AddRepositoryResponse | undefined>`. Return undefined for an obsolete attempt, including an obsolete error. Propagate current errors to the caller. Item 6 supplies the session's `Api.projects` from item 3 and a fresh signal per attempt. It routes current `unauthenticated` through the shared cleanup. (architect-r1 07, critic-r1 9)
- First write `folder procedures keep the intent key` in `folder-calls.spec.ts`. Import the module directly and invoke both wrappers with a generated client on a controlled transport, a guard and per-attempt signals. Reject an attempt and retry with the same body and key and a fresh signal. Pin the omitted target and a path with a space. Verify separate keys on new intents and procedures. Release obsolete success and expiry and observe undefined. Mutate key creation, path forwarding and the guard. Require red assertions. Item 5 merges with `pnpm exec playwright test --config src/frontend/playwright.config.ts folder-calls` passing. (architect-r1 07, critic-r1 9)

**Item 6. Real Add repository.** Change `src/frontend/src/screens/service.tsx` and `src/frontend/src/server/folders.ts`. Create `src/frontend/e2e/add-repository.spec.ts`. Start after items 3, 4 and 5. Foundation D1 and rules 2, 7 to 9 apply. This item implements the browser piece of newer R1, R2, R4, R5, R7, R12 and R14.

- Keep the fields and styles. Item 6 writes `defaultFolderMachine` in `folders.ts` for both dialogs. Prefer a registered opener, else the first online machine, else blank. Keep a still-present manual selection, including offline, across watch updates. When the selected machine disappears, clear its facts, preserve the path and select a current default. A blank choice disables submit while leaving the path editable. Compatibility keeps its existing branch. (critic-r1 5, logic-r1 02)
- Debounce a plausible absolute or home-relative path for 700 milliseconds. Pass the exact typed path. An edit clears facts and cancels obsolete timers and attempts. Retry inspection once after an inconclusive transport failure or the fixed wait response `deadline_exceeded` with “The machine did not answer the folder request.” Use the same key and body with a fresh signal. A second refusal ends the automatic retry. Existing edit and reopen actions can start a new inspection. Populate Base branch from `mainBranch`, Remote from `remote` and `remoteOmitted`, and Status from `clean`. Render fixed refusals in the existing footer. Missing facts cannot enable Add. (logic-r1 06)
- A fresh add needs current inspection facts and an online machine. Retain the exact path and explicit target even if that repository disappears. Allow one in-flight submit. A lost reply can be retried on the same intent, even offline, for up to 23 hours from first send. Use the greater elapsed interval from the wall and monotonic clocks for the in-tab replay check. At expiry retire the intent and invalidate its facts. The existing Add button starts a fresh inspection with the current fields at that point, and only a subsequent explicit Add can write. Closing or editing also ends the intent. A server rollback to an earlier database state is outside wave 1. (logic-r1 04)
- Require both returned identifiers and the submitted machine on the reply folder. Match against `currentRepositorySnapshot()` and confirm the watched folder's machine is still registered. A retained list cannot complete the form. At two seconds without a match restart only the repository watch once. Give the join six seconds from the reply. A full snapshot from an attempt opened after the reply can prove absence. Use the missing-repository or removed-machine refusal, or generic refusal for a missing folder of a still-saved repository. On timeout, release the form with existing network feedback. On a match, expand the watched entry, show the existing success toast and close or restore `d.back` once. Unary replies never write records. (architect-r1 03, logic-r1 02 and 03, critic-r1 4)
- **UNREVIEWED MECHANISM** (critic-r1 4, logic-r1 03). Once the browser has a successful reply, keep its identifiers and let Add recheck the watch with the same time bound. The button sends no further save because the result is already known. Retain the same intent and fields, and keep ordinary watch retries running.
- If the current add loses its machine, either the watched removal or the fixed removal refusal settles that intent. Keep the exact path and show “The machine was removed.” once in the footer. A watch-driven fallback clears facts for the old machine without clearing that feedback or issuing an add to the new default. An edit, replacement dialog or session reset clears it. A late success for the removed machine cannot restore its selection. (critic-r1 5, logic-r1 02)
- First write `inspection follows the latest input` in `add-repository.spec.ts`. Hold A, edit path and machine, and apply B's clean or dirty facts, including a dirty detached checkout, absent and withheld origin cases. Release A as success and refusal. Check B's rows, exact path and distinct keys after editing away and back. Exercise one wait response followed by settlement and a repeated terminal response that stops retrying. No Inspect writes a repository. (logic-r1 06)
- First write `add joins the watched identity in either order`. Hold reply or matching folder frame and exercise both orders with a different first folder on the machine. Check two typed paths that resolve to one folder. Test lost reply replay, including offline, with the same key and body. Advance the clock past retirement with the same typed path resolving elsewhere. No old key or fresh add may send before new facts and a new explicit submit. Disconnect after seeing a save while holding its reply. Remove the machine, release the reply before the next full snapshot and order machine and repository removals both ways. No retained list completes the form. Fail one replacement and leave another silent past the deadline. The form unlocks, then the existing button rechecks the recovered watch without an extra save. Require one saved identity and one return. (architect-r1 03, critic-r1 4, logic-r1 02, 03 and 04)
- First write `add refusal and expiry preserve or clear the form`. Exercise both arrival orders of machine removal and fixed refusal, with another online machine. The path stays, the footer shows one refusal, and no add is sent to the replacement. Check mismatch and a removed explicit repository target. Expire while either call is pending and observe both aborts and a clean form after sign-in. Replace the dialog while a save is pending. At desktop and 390 by 844 pixels, require focus, wrapped long values, reachable actions and no horizontal overflow. Risky mutations let stale results commit, substitute the wrong folder, accept retained rows, skip the join deadline, ignore receipt age or overwrite store records from the reply. Require red assertions. (architect-r1 01, critic-r1 5, logic-r1 02 and 04)

**Item 7. Dialog defaults, return and sidebar entry.** Change `src/frontend/src/{app,dialogs}.tsx`. Create `src/frontend/e2e/repository-entry.spec.ts`. Start after items 6 and 9. Foundation D1 and rules 2 and 8 apply. This item implements newer R13 and the browser flow in R14.

- The sidebar's existing plus opens `add-repository` in repository mode. Keep the accessible name and tooltip. Keep both Machines entry points and their opener identifiers. Generic New project uses the first online machine. New project with a machine identifier uses it if present. Repository-specific entries keep the repository identifier. Use `defaultFolderMachine` in both dialogs. Waiting machines stay out of the choices.
- Keep the layout, empty lead controls and existing checks for online machine, goal, attached repository or a name for the New repository option. Remove the lead prerequisite only. A valid button or Enter submission shows the copy table's exact sentence in the existing unavailable treatment inside the dialog body, below the form and above the footer. Reuse the shared empty-state presentation used by Files and Terminal. Keep the form mounted and the response visible until a form edit or close. Blank machine disables submit and attach. No project or named-repository request is sent. (architect-r1 08, critic-r1 6)
- The existing Add to a machine entry carries the selected repository, machine and goal in `DialogState.back`. Back restores the previous form. A successful attachment restores it with the watched repository selected after item 6's join. Keep other existing form field values. Derive options from the current store without overriding a still-present manual machine choice.
- First write `add uses the sidebar or the opening machine` in `repository-entry.spec.ts`. Put an offline machine before two online machines. The plus picks the first online one, and both Machines actions use the second when opened there. Both samples and the old server keep Track repository from the plus. In `new project keeps the goal and reports unavailable`, check Back and successful attachment with a unique goal. Exercise valid button and Enter without a lead and expect the exact persistent response inside the dialog, unchanged form and zero project writes. Check missing goal, unnamed new repository and unattached repository in both submit paths. In `both dialogs have an empty machine choice`, check zero and offline-only lists, disabled actions, editable path and zero calls. An arriving online machine becomes the default. A later unrelated save leaves a manual choice alone. At desktop and 390 by 844 pixels use `?data=worst` and long labels. Exercise the phone drawer and nested return. Require one dialog, visible focus, reachable controls and no horizontal overflow. Risky mutations reverse machine precedence, blank the restored goal, reset manual choice on a watch update or bring back the lead gate. Require red assertions. (architect-r1 08, critic-r1 6)

**Website text shown by the changed branches**

This table records the copy used when a sentence or its display condition changes.

| Place and condition | Exact displayed copy | Authority |
|---|---|---|
| Sidebar plus, accessible name and tooltip | “Add a repository” | existing |
| Add repository, footer and row labels | “Add repository”, “Back”, “Cancel”, “Machine”, “Base branch”, “Remote” and “Status” | existing |
| Add repository, Status, clean | “A git repository with a clean working tree” | existing |
| Add repository, Status, dirty | “This Git repository has uncommitted changes.” | new. The owner approved a second sentence on 2026-11-27. |
| Withheld origin in Add repository, Checkouts and Switcher, or an unfilled folder fact | “—” | existing |
| Known absent origin in Add repository, Checkouts and Switcher | “No remote” in the dialog and Switcher. “None” in Checkouts. | existing |
| Add repository, path with a selected machine | “Folder on {machine}” with placeholder “{workspace}/my-repository” | existing |
| Add repository, path with blank machine choice | “Folder” with no placeholder | existing label reused for the approved empty choice |
| Both machine choices without a default | An empty selected option | approved on 2026-11-27 |
| Add repository, progress and success | “Detecting…”, “Looking at the folder…”, “Adding…” and “Added {name} on {machine}” | existing |
| Add repository, implausible path and offline selection | “Give the folder of a git repository.” and “{machine} is offline.” | existing |
| New project, form title and submit button | “New project” and “Create project” | existing |
| New project, goal, name and repository validation | “Write the goal.”, “Name the new repository.”, “{repo} is not on {machine}.” and “Add to {machine}” | existing |
| New project, below the form on valid submit | “Projects are not connected to this server yet.” | approved on 2026-11-27. Use the unavailable body. (architect-r1 08, critic-r1 6) |
| Home, All projects and empty widget or machine detail | “No open requests”, “No open request matches.”, “No project matches.”, “No projects” and “No repositories” | existing |
| Notifications, Switcher and missing routes | “No notifications”, “Nothing matches "{query}"”, “This project does not exist”, “This repository is not known”, “This page does not exist” and “Go to Home” | existing |
| Direct setup link before sign-in or after expiry | “Sign in to the service on {machine}” when the server host is known. “Sign in to the server” otherwise. On live expiry, “You were signed out because the session expired. Sign in again and you return to where you were.” | existing |
| Unavailable route, widget, section or Terminal | “{subject} is not connected yet” and “The service does not send this to the website yet.” Use “Files” for Files, “The terminal” for the terminal, and “This page” otherwise. | existing |
| Current folder refusal, connection or invalid input | “The machine is not connected.”, “The machine, repository, or folder path is invalid.”, “The request key is invalid.” and “The request body is invalid.” | new. The reviewed contract fixes the sentences. |
| Current folder refusal, target or key conflict | “That request key was already used for a different request.”, “The machine was not found.”, “The repository was not found.”, “The folder does not belong to the selected repository.” and “The folder is already saved for another repository.” | new. The reviewed contract fixes the sentences. |
| Current folder refusal, path facts | “The path is not a folder.”, “The path is not a Git repository.”, “The folder cannot be read.” and “The folder path cannot be used.” | new. The reviewed contract fixes the sentences. |
| Current folder refusal, operation result or watched removal | “The machine did not answer the folder request.”, “The machine did not take the folder command.”, “The outcome of the folder request is unknown.” and “The machine was removed.” | new. The reviewed contract fixes the sentences. |
| Folder footer, watch wait expires or current request fails without a fixed sentence | “The server did not answer.” on network failure or deadline. “The server refused the request.” for generic refusal or a missing folder of a still-saved repository. | existing |
| Machines, recovery flag true | “Out of sync” in Status and “This machine's saved history is older than the server's.” in the opened row | approved on 2026-11-27 |
| Machines, flagged removal confirmation and success | “Remove {machine}?”, “{machine} is disconnected and forgotten.”, “Its folders stay on its disk.”, “Remove machine” and “Removed {machine}” | existing. The flagged branch omits the pairing instruction. (architect-r1 06) |

**Design artifact** [The existing dialogs and approved changes](../../../docs/specs/2026-11-27-running-an-agent-design.md#the-website-in-sprint-4). [The contract's fixed refusals](../go-s4-contract/plan.md). [Current dialog code](../../../src/frontend/src/screens/service.tsx#L443).

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Replay starts another operation or a stale add writes the list. | High | Keep the intent key and join by both identifiers. The watch owns records. |
| Empty choice crashes a dialog or lead blocks submit. | Medium | Test the shared chooser and both entry points at both widths. |

**Verify** Run `pnpm exec playwright test --config src/frontend/playwright.config.ts folder-calls add-repository repository-entry`. Exercise both dialogs on desktop and at 390 by 844 pixels. Use `?data=worst` on sample and real paths. Supply long real-server fixture labels because that query seeds nothing on a real server.

**Rollout detail** The frozen guide's R14 saves only from Machines. Stack supplies `S4_REPO_A` and `S4_REPO_B`. The orchestrator records the owner observation for the Stack browser gate. On the candidate's same-origin web image built with `VITE_LGTM_SERVER=/`, observe same-origin discovery and no address prompt. Use the copy-table locators to save A from the sidebar and B from Machines on one approved machine. The report passes R14 when it records both saves, both watched identifier pairs and both resolved paths after reload. (architect-r1 04, critic-r1 8, logic-r1 09)

**Workstream 3 (WS-3). Show the machine recovery state**

The Machines page reads the diagnosis from the machine watch.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 8 | Snapshot and saved frames show the approved status and reason for offline and waiting machines. A later ready admission or removal updates the open page. Remove machine remains available. | `machine recovery follows snapshots and updates`, `recovery state survives stale approval and clears on watched readiness` and merged `TestSprintFourHistoryRecovery`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 8 | Project the recovery flag and display the fifth approved change. | `src/frontend/src/server/sync.ts` `toMachine`, `toJoining` and `watch`. `types.ts` `Machine` and `JoiningClient`. `screens/service.tsx` `Machines` and `MachineDetail`. |

**Item 8. Machines recovery status.** Change `src/frontend/src/server/sync.ts`, `src/frontend/src/types.ts` and `src/frontend/src/screens/service.tsx`. Create `src/frontend/e2e/machine-recovery.spec.ts`. Run last, after items 7 and 9. Foundation D1 and D5, rules 2, 5, 7 and 8, and C6 to C9 apply. Its browser part follows newer R9, R11 and R12 and the contract's recovery handoff.

- Sync maps `Client.eventHistoryNeedsRecovery` on snapshots and saves into `Machine.eventHistoryNeedsRecovery` and `JoiningClient.eventHistoryNeedsRecovery`. Machines reads both. Absence is false. In repository mode show the approved status and one reason. The reason replaces offline and pairing callouts. Keep the watched standing for counts and actions. An unflagged offline worker uses the normal Offline status and reported facts. Unflagged waiting stays in pairing. A flagged waiting row shows its actual diagnostic and Remove machine and stays out of counts and choices. The host and waiting removals use the existing confirmation and copy table. Leave out the pairing instruction for flagged removals. Only the watch changes the flag or removes a row. (architect-r1 06, critic-r1 7, logic-r1 07)
- First write `machine recovery follows snapshots and updates` in `machine-recovery.spec.ts`. Check flagged offline, waiting and host records and an older record omitting the flag. At desktop and 390 by 844 pixels require the exact status, exactly one approved reason in the opened row, no reconnect command, pairing instruction or simulated agent claim, and reachable Remove machine. Check the confirmation copy and that only watched removal deletes the row. Apply unrelated saves and reopen. The reason persists without a duplicate row. (architect-r1 06, critic-r1 7, logic-r1 07)
- First write `recovery state survives stale approval and clears on watched readiness`. Hold an old approval, watch the machine register with the flag, release approval and send an unrelated save. Only the later ready save or a replacement snapshot with false clears it. Repeat with removal and verify late approval cannot restore the row. Risky mutations read the flag only on initial snapshot, clear it on approval or unrelated update, or drop the waiting projection. Require red assertions.

**Design artifact** [Recovery decision in the approved design](../../../docs/specs/2026-11-27-running-an-agent-design.md#decisions-of-the-owner). [Recovery note](../../../.foundry/go-rewrite/tasks/note-web-recovery-flag.md).

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A waiting or offline machine loses its flag after approval. | High | Test old approval against a newer save and replacement snapshot. |
| The diagnostic hides Remove machine on a phone. | Medium | Check the action and exact copy at both widths. |

**Verify** Run `pnpm exec playwright test --config src/frontend/playwright.config.ts machine-recovery` at desktop and phone widths. The merged Go check exercises the actual later admission and removal. Link owns the durable flag.

### Post-build checks

| Owner and findings | Scenario and expected result |
|---|---|
| Stack and Website. (architect-r1 P4, critic-r1 P1, logic-r1 P1) | Run frozen R13 and the older-server Website case. Use one build and observe sign-in, sample work and no repository calls on the three compatibility paths. The advertising path waits for both snapshots and shows only watched records, including after a worst-case query reload. Repeat a pending sample action across a tab's server switch. Expect only the new workspace. |
| Stack, Projects and Website. (architect-r1 P1, critic-r1 P2, logic-r1 P2) | Record deployed R14 as above. At both widths, compare the three rows to real checkout facts. Hold a real add reply at the browser boundary and compare its request key and identity pair with the watch and tree. Reload and open both paths from the tree and Switcher. Expect one entry per saved identity and no synthetic activity or terminal. |
| Website. (architect-r1 P2, critic-r1 P4) | Run the session suite in Chromium with browser cookies. Hold a cookie writer on one port, select the other port on that hostname and observe discovery's cookie after release. A late completion on a different hostname leaves the selected address, hash and two authenticated watches alone. Use the repository-watch suite for the second stream after item 3 lands. |
| Website and Stack. (architect-r1 P3) | On a real worker before a harness scan, open Machine detail. Expect no invented version comparison, installation or account status. Run frozen R12 for live orphan and surviving folders. With a delayed watch, run both orderings and verify no removed machine action resumes. |
| Orchestrator, Link and Website. (critic-r1 P3, logic-r1 P3) | Run `TestSprintFourHistoryRecovery` and observe the watched flag on an open Machines page at both widths. An unrelated save and reconnect preserve the exact reason. Later ready admission clears the flag, or removal drops the flagged row. Check the existing Remove machine action for waiting and host records. |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe covers all six assignment deliverables, the recovery update, foundation decisions, rules and checks, sprint checks R1 to R14, both design closing lists, the Contract handoff and the assigned website checklist lines.

| Source item | Workstream | Status |
|---|---|---|
| Assignment 1 and contract Website row on discovery, watch replacement, identity, retries and stale replies | WS-1 and WS-2 | Items 1 to 3 and 5 to 6. |
| Assignment 2, fallback modes and no simulation | WS-1 | Items 1, 4 and 9. |
| Assignment 3, dialogs and machine selection | WS-2 | Items 6 and 7. |
| Assignment 4, the original four approved changes | WS-2 | Items 6 and 7 and the copy table. |
| Assignment 5 as revised by the owner note, and the contract Website row on recovery | WS-3 | Item 8 and the copy table. |
| Assignment 6, routes with only machine data | WS-1 | Items 4 and 9 and the route table. |
| Foundation D1, D5 and D11, and rules 2, 5, 7 and 8 | WS-1, WS-2 and WS-3 | Generated contract, watched state, cookie session, safe retry and fixed refusals. |
| Foundation rules 9 to 11 | WS-1 and WS-2 | Render returned facts as text. The stack checks the real server and database. No third-party code is planned. |
| Foundation C1 to C9 | WS-1 and WS-3 | The website preserves discovery, session and Machines behavior. Stack retains the real deployment checks. |
| Foundation C10 to C13 | WS-1 | Sprint regression dependency. Link and Stack retain actual proof. |
| Newer R1 and R2 | WS-2 | Browser projection plus Stack's real folder proof. |
| Newer R3 to R7 | WS-2 | Browser identity and refusal scenarios. Projects and Link own execution and restart proofs. |
| Newer R8 to R11 | WS-3 | Link and Stack own execution. Item 8 proves the related client projection. |
| Newer R12 | WS-1 and WS-3 | Open pages lose removed machines and folders through the watches. |
| Newer R13 | WS-1 and WS-2 | One website bundle exercises both sample transports and real discovery. |
| Newer R14 | WS-1 and WS-2 | Browser entry and reload plus Stack's real-machine gate. |
| Logic closing list, replace snapshots, ignore stale inspection and locate the lead controls | WS-1 and WS-2 | Items 3 and 6 cover sprint 4. Lead controls belong to sprint 5. |
| Fit closing list, identity join, late dialogs and individual views | WS-1 and WS-2 | Items 2, 4, 6 and 9. Includes terminal and no lead column at either width. |
| Checklist line 64, open from the expandable tree | WS-1 | The repository slice. |
| Checklist line 72, find and open by keyboard or search | WS-1 | The repository slice. |
| Checklist line 161, add a folder after seeing machine facts | WS-2 | Add repository and Add to a machine. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, What this project is and Where things are | Website writes stay under `src/frontend/`. The reviewed Protocol Buffers contract and generated TypeScript supply wire messages. The legacy engine and mock server are used in place. |
| `AGENTS.md`, Credit copied code | Reuse internal helpers. This work needs no external code. |
| `AGENTS.md`, Write down what you find | The bounded planner records problems and assumptions in decisions, scouting and the result for the orchestrator. |
| `AGENTS.md`, Finished work reaches the base branch through a pull request | The orchestrator handles this plan's changes and pull request. Builders use the epic's team-branch verification gates. |
| `AGENTS.md`, Writing style for documents | Cite dated decisions and quote only interface copy. Run the private-text check and inspect the new untracked files. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

The planner, test writers, builders and reviewers for this assignment are workers of one vendor, OpenAI. The assignment supplies that staffing. Use separate seats and freeze the first tests.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Discovery and initial workspace | `mobile-ui` | `website/frontend` | medium | It follows the existing initializers and changes the timing of sample startup. | Yes | Reviewed discovery contract. |
| 2. Server and session lifetime | `mobile-ui` | `website/frontend` | medium | One lifetime check protects existing transitions and machine callbacks. | Yes | 1. |
| 3. Repository watch | `mobile-ui` | `website/frontend` | medium | It follows the existing watch with a second initial snapshot and one projection. | Yes | 2 and generated contract. |
| 4. Shell and service routes | `mobile-ui` | `website/frontend` | medium | It gates setup and reuses service bodies with watched machine facts. | Yes | 3. |
| 5. Folder request identity | `mobile-ui` | `website/frontend` | medium | Two generated calls share request identity and cancellation. | Yes | 2 and generated contract. |
| 6. Add repository | `mobile-ui` | `website/frontend` | medium | One existing dialog wires a keyed read and write to the watch. | Yes | 3, 4 and 5. |
| 7. Defaults, return and sidebar | `mobile-ui` | `website/frontend` | medium | Two form states share selection and one existing transition. | Yes | 6 and 9. |
| 8. Machine recovery | `mobile-ui` | `website/frontend` | medium | One additive client field reaches existing rows and the waiting projection. | Yes | 7 and 9. |
| 9. Repository screens and search | `mobile-ui` | `website/frontend` | medium | Existing repository tabs and search read the watch projection. | Yes | 4. |

Run item 5 alongside items 3, 4 or 9. Items 6 and 9 can run together after item 4. Item 7 starts after both. (logic-r1 11, critic-r1 9)

Load is 25 OpenAI launches. Count the original planner, the fold planner, three plan reviewers, nine test writers, nine builders and two merged-review seats. The orchestrator compares the 20 remaining planned launches to the live allowance and checks `.plans/scopes/website/frontend/scope.json` before dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | 1 blocker, 21 majors and 9 minors before grouping. | Three reviewers requested revision. The [finding register](attack/round-1-findings.md) records the single round and its fold. The post-build checks cover the remaining observations. |

</details>

