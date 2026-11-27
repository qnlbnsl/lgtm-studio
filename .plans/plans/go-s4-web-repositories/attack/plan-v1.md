---
topic: go-s4-web-repositories
status: attack
tier: M
created: 2026-11-27
updated: 2026-11-27
epic: go-rewrite
---

# Add a repository on a real machine from the website

## Overview

An owner signed in to a sprint 4 server can inspect a folder, add it from the sidebar or Machines, and find the saved repository in the tree after reloading. The website waits for the machine and repository lists before opening the workspace. Its current startup fills the workspace with sample work before it knows which server answered. Startup will use that answer to choose the source of the workspace records.

The folder check shows the server's base branch, remote and working-tree status as the owner edits the path. When a reply is delayed, the website keeps it tied to the original input and sign-in. The server's repository list settles what appears in the tree, even if a save reply and a list update arrive in the opposite order.

The owner approved five visible changes on 2026-11-27. A dirty checkout gets its own status sentence. The New project button explains that projects are unavailable on this server. The sidebar plus opens the folder dialog. Both dialogs show an empty machine choice and disable their submit buttons when no machine is available. Machines shows a distinct status and one line of reason when its saved history is behind the server. Screens without real data use their existing empty or unavailable views.

## Key decisions

The website reads the server's list of available services before loading workspace data. One build can still open the in-page sample, the standalone mock, and an older server. Their sample workflow is the reference for the screens.

The repository list comes from the server watch. A successful add gives the browser an identifier for the saved repository and its folder. Waiting for those identifiers keeps a delayed reply from bringing back a folder that another action has already removed. A sign-out, an expired session, or a server change ends the watches and drops the old records and unfinished dialog actions.

The recovery display uses the existing status column, the opened row and the Remove machine action. The watch supplies the diagnosis and its eventual clearing.

Full log → [decisions.md](decisions.md)

## Workstreams

### Open a workspace from the server that answered

The workspace shows the machine and repository lists from the signed-in server after both are available. Each list refreshes after a broken connection. The sample modes still load their records and simulation. The owner can use the tree and search to open a real repository, and a saved address with no matching record shows the existing missing-record page.

### Inspect and add a folder from either entry point

The owner can open the same folder dialog from the sidebar, Machines, or New project. It shows the facts for the current input and keeps the goal when returning to New project. A retry uses the same request key. An edited path or a replaced dialog has its own identity, so an old reply cannot select the wrong folder. Both dialogs are checked on a phone-wide screen.

### Show the machine recovery state

A machine's recovery state comes from the same watch that supplies its other facts. The final work item proves that reopening the watch, receiving an unrelated update, or accepting a delayed approval reply does not erase that state.

## Rollout order

1. Take the reviewed, generated contract from the Contract team. Build the workspace selection and cleanup first so a repository can arrive only in the current session.
2. Add the repository watch and connect the existing screens to its records. Exercise the machines-only workspace and the compatibility modes in the same website build.
3. Wire both dialogs to the current machine and the folder calls. Prove the two arrival orders and the return to the goal. Run the browser checks at desktop and phone widths.
4. Build the approved recovery display as the last item. Verify both folder entry points and the recovery state on the merged stack once the server and a real machine are ready.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1: Open a workspace from the server that answered**

Discovery, session ownership and the two watches define when records can enter the browser. The existing screen entry points use that same state. The application keeps its TypeScript website and the existing legacy embedding entry point.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1 | The same bundle opens the two sample transports and a revision 1 server without the advertised service. A real server with the exact service name starts empty and has no simulation timer even with the worst-case sample query. | `discovery chooses the workspace and simulation` and stack R13. |
| 2 | Sign-out, either watch expiring the session, and a server change drop the old records, polls, action effects and dialog state. An older discovery or sign-in reply cannot install a transport or restore sign-in. | `session boundaries discard late replies` and `stale machine actions cannot replace a watch`. |
| 3 | The shell opens after the first full frame from both watches. A reopened repository watch replaces the whole list. Removing a machine and its final folder changes both open views through their respective watches. | `workspace waits for both snapshots`, `repository reconnect replaces and removal converges` and stack R12. |
| 4 | Each current route has the outcome in the route table. Repository tree, machine detail and search use the watched records and the existing links. | `machines-only routes have no sample data` and `saved repositories open from tree and search`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Defer the sample seed and simulation until discovery chooses a compatibility workspace. | `src/frontend/src/app.tsx` `mount` and `mountLive`. `data.ts` `base`, `emptyState` and `seed`. `actions.ts` `startSimulation`. |
| 2 | Fence startup, watch and action completions by the active server and session. | `src/frontend/src/server/api.ts` `openServer`. `server/sync.ts` `connect`, `watch`, `signIn` and `signOut`. `screens/service.tsx` `PairMachine` and `Machines`. |
| 3 | Add the generated repository watch and project its records into the existing tree data. | `src/frontend/src/server/sync.ts` `watch`. `types.ts` `Repo`. New `src/frontend/src/server/repositories.ts` follows `server/sync.ts` `toMachine` and its replace, save and removal cases. |
| 4 | Route the machines-only workspace and keep repository navigation usable. | `src/frontend/src/app.tsx` `App`, `ProjectTree`, `toggleTerminal` and `NotConnected`. `screens/repo.tsx` `RepoView`. `screens/center.tsx` `RepoCenter`. `screens/service.tsx` `MachineDetail` and `ServiceSettings`. `dialogs.tsx` `Switcher`. |

Paths shortened in anchor cells are relative to `src/frontend/src/`. The tests below run through the existing website entry point in `src/frontend/src/main.ts`. The fixture builds the same Vite bundle once with address choice enabled. One loopback web listener serves that bundle and the typed test responses, with separate path prefixes or listeners for server identity tests. Browser fixtures use generated schemas for messages and controllable Connect stream envelopes over Node's existing Hypertext Transfer Protocol (HTTP) listener. They hold and release a reply or frame using explicit barriers, record requests and observe aborts. Use the real in-page engine and start the real standalone mock with `pnpm --filter @lgtm/mock-server start` for compatibility. Website fixtures exercise the browser boundary. The stack tests exercise the real Go server, real PostgreSQL and a real machine. Test writers freeze assertions before builders change production files. Stubs use only the planned names and shapes if needed to reach a failing assertion.

**Item 1. Discovery and the initial workspace.** Change `src/frontend/src/{app.tsx,data.ts,actions.ts,types.ts,define.ts}`, `src/frontend/src/server/sync.ts` and `src/frontend/src/screens/{service,mocks,terminal,workflow}.tsx`. Create `src/frontend/playwright.config.ts`, `src/frontend/e2e/support/repository-server.ts` and `src/frontend/e2e/repository-discovery.spec.ts`. The browser configuration points only to this team's `e2e/` folder. The existing root configuration supplies the Chromium and Playwright conventions. Foundation D1, rules 2 and 7 and C1 to C4 apply. This item enables newer R13.

- Start the default mount with an empty workspace using `emptyState` and the empty slice setup beside `mountLive`. Factor that setup so the Go mode uses empty collections without enabling the legacy engine path. Let discovery call the existing sample initializers only when choosing the compatibility workspace. Within that workspace keep its existing initialization, navigation, timers, persistence and sign-out behavior. The real path uses in-memory workspace drafts and the same browser presentation preferences. Its record slices include machines, joining machines and repositories from the two watches. Projects, teams, agents, requests, tasks, epics, sprints, threads, channels, posts, subscriptions, notifications, usage history, workflow, design, worktrees and terminal sessions start empty.
- Select repository mode only after the product and `CONTRACT = 1` checks, when the address is outside the in-page sample, `sample` is false and `services` contains precisely `lgtm.v1.ProjectService`. An absent or empty list, an unknown entry or `sample = true` chooses compatibility behavior. Preserve `mount(..., { mode: 'live' })`, which uses its own store and engine connection. Add `ServerState.repositoryMode` as the internal mode bit. Do not make capability decisions from link revision.
- Stop the interval through `stopSimulation` and use the existing timer sites to invalidate callbacks tied to the departed sample workspace. A small `sampleLater` wrapper in `actions.ts` captures the state object and checks that it is still current and in the compatibility workspace before running. Use it for delayed sample writes that can escape the view, including project naming, request replies, Define replies, repository setup, service restart and notifications, terminal output and sample workflow and design completions. A same-mode compatibility reconnect keeps its current sample workspace. Entering or leaving repository mode replaces the workspace and clears this interval. Legacy mounted behavior uses its existing path.
- First test `discovery chooses the workspace and simulation` in `repository-discovery.spec.ts`. Hold discovery and inspect `window.lgtm.store` for empty sections. After discovery, assert the candidate modes separately, including `sample=true` with the service present and a lookalike service name. Use `?data=worst` before the hash. Assert the real store has no sample sentinel and its 4,200 millisecond agent timer is never scheduled. On both actual samples, demonstrate the sample project and changing activity. On capability-absent revision 1, demonstrate existing sample project and track-by-remote dialog. From a sample, leave a delayed sample project or request reply in flight, switch to a repository server and advance the browser clock. The real store and connection freshness stay free of sample writes. Mutate eager seeding, the sample precedence or the timer guard and require that test to fail. This item is risky because discovery orders startup and timer writes.

**Item 2. Server and session lifetime.** Change `src/frontend/src/server/{api,sync}.ts`, `src/frontend/src/actions.ts` and `src/frontend/src/screens/service.tsx`. Create `src/frontend/e2e/repository-session.spec.ts`. Start after item 1. Foundation D1, D11, rules 7 and 8 and C3 to C7 apply. This item enables newer R12 to R14.

- Have `openServer` return the validated discovery and its candidate typed clients. Install the candidate and its address only if the discovery generation is still current. Keep the selected service clients and an abort controller in one authenticated scope. `captureSession` in `server/sync.ts` returns that scope's clients and signal with an `isCurrent()` check. It is also the boundary for item 5's folder procedures. Watches capture their clients once. Check the scope before applying a frame, scheduling a retry or handling any response, including sign-in and sign-out completions. The controller cancels network waits. The identity check also covers a reply already queued to run. A failed obsolete probe cannot change connection, address or storage.
- In repository mode, invalidate the scope before awaiting a best-effort sign-out, changing the selected server, or accepting current `Code.Unauthenticated` from either watch or a protected machine or folder call. Close both watches and clear the actual store immediately. Clear the two machine collections and repository list, both initial readiness marks, expanded record keys, record-specific selection and panels, dialog state and pending intents, toasts, drafts and derived records. Preserve appearance and navigation preferences. Keep the sign-in outcome, address choice and connection error handling on the current generation. A late sign-out result must not sign out a newer session. Give cookie-writing sign-in and sign-out calls a six-second deadline. Serialize them by server hostname, since browser cookies can share a hostname across ports. Discovery of an address on that hostname waits for earlier cookie-writing calls before treating its signed-in result as current. A different hostname can still connect immediately. Preserve the fallback mode's existing cleanup and storage behavior. A fresh sign-in creates a new scope and fresh watches. Reopening a watch in the same authenticated scope is a transport retry.
- Capture the clients used by pairing, including token creation and revocation. A delayed dialog cleanup must never revoke a token at the newly selected server. Guard machine action UI completions in the same scope. In real mode, removal and approval leave both machine collections to `WatchClients`. Approval may close only its own active pairing dialog and must not restore a removed row or replace a newer machine update. Removing a machine in real mode performs no simulated agent or subscription changes. Late successes and refusals may not reopen a dialog, set expansion, or show a toast for a previous session.
- First write `session boundaries discard late replies` in `repository-session.spec.ts`. Hold either watch, an authenticated action and a discovery probe across expiry, explicit sign-out with a stalled request, and a change to a second server. Assert both stream aborts, empty browser collections and dialogs before the old reply resolves, and only the new server's records after it resolves. Run expiry once from each watch and again through the folder call in item 6. Hold old sign-in and sign-out against a newer server on another hostname. For a same-host sign-in after stalled sign-out, use the fixture's session cookie and verify the next sign-in reaches the server after the previous cookie writer settles or aborts. Check the new watch remains authenticated. Check that an old probe neither selects its clients nor writes its address back. Use a controlled fetch in the browser fixture to release a queued result after abort and verify the commit check independently of fetch cancellation.
- In the same file, write `stale machine actions cannot replace a watch`. Hold approval, deliver a newer machine save and removal from the watch, then resolve the approval. Assert that the row is absent and the approval cannot close a replacement dialog. Repeat with a late remove failure across a server switch and assert no obsolete toast. Release an old pairing-token reply and assert zero revoke requests at the new server. This item is risky because it touches session transitions and action ordering. Remove one generation guard, delay cleanup until after sign-out, or let approval write a machine row. Show the corresponding assertions go red.

**Item 3. The repository watch.** Change `src/frontend/src/server/{api,sync}.ts` and `src/frontend/src/types.ts`. Create `src/frontend/src/server/repositories.ts` and `src/frontend/e2e/repository-watch.spec.ts`. Start after item 2 and the generated contract. It can run alongside item 5. Foundation D1 and D5, rules 2, 5, 7 and 8, and C7 to C9 apply. This item owns the browser projection of newer R12 and R13 and makes R14 observable.

- Add `Api.projects` using `createClient(ProjectService, transport)` from `@lgtm/api/projects` on the selected transport. Consumers use generated `watchRepositories`, `inspectFolder` and `addRepository`. Define `ServerState.synced` as the current scope's initial machine snapshot and, in repository mode, its initial repository snapshot. Gate the workspace on both in either order, including an empty message. Each stream opens with `all`. Apply `WatchRepositoriesResponse.change.all` as full replacement, `saved` as full upsert by repository identifier and `removedId` as removal. Ignore a delta before that stream's first snapshot. Preserve the existing retry bounds of 500 milliseconds to 8 seconds. A normal disconnect or `aborted` reopens that stream from a replacement snapshot, with no cursor resume. Give an individual stream attempt its own cancellation and identity. A queued frame from a replaced stream cannot commit inside the same session. Retain the current visible snapshot for a transient reconnect. Derive live connection status from both streams in repository mode so a machine heartbeat cannot mark a disconnected repository watch live.
- In `repositories.ts`, project `Repository.id`, `name`, `mainBranch`, `remote`, `remoteOmitted` and `firstCommit` to the existing `Repo` shape. Retain every `RepositoryFolder.id`, `clientId` and `resolvedPath` as `Repo.folderRecords`. Keep the existing `Repo.folders` path lookup for clients of the sample shape. Use the first folder by identifier for each machine in that convenience lookup. Machine and checkout lists can read the complete folder list. Preserve optional facts as absent and carry the omission bit through the projection. `hasRepositoryFolder(repoId, folderId)` reads the current projection by both identities. That gives item 6 a wait condition without applying unary reply data to the store. Expose a scope-bound repository refresh in `sync.ts` that can replace just this stream and report its first snapshot. Item 6 uses it only when a returned identity still has not appeared.
- First write `workspace waits for both snapshots` in `repository-watch.spec.ts`. Release the streams in both orders, with an empty repository snapshot in one pass. Before both `all` frames the shell is busy and a deep link is unresolved. Afterwards assert Machines and the requested view. In `repository reconnect replaces and removal converges`, begin with two folders on different machines, including a folder identifier distinct from its machine identifier. Save a replacement with one removed folder and assert the old path disappears. Remove the last machine and repository through the two watches. Then disconnect with `aborted`, miss a removal, reopen with an empty snapshot and assert the tree has no stale rows. While one stream is down, a heartbeat from the other leaves the connection reconnecting. Include multiple folders on one machine and reordered records to verify stable projection. This item is risky because frames can arrive in either order. Mutate the initial gate to accept one snapshot, merge instead of replacing `all`, append instead of replacing `saved`, and let either watch alone mark the connection live. Show the named tests fail.

**Item 4. Existing views with only machine data.** Change `src/frontend/src/{app,ui,dialogs}.tsx` and `src/frontend/src/screens/{repo,center,service}.tsx`. Create `src/frontend/e2e/repository-routes.spec.ts`. Start after item 3. It can run alongside item 5 when the fixture exists. Foundation D1 and rules 2 and 7 apply. This item enables newer R12 to R14.

- Keep the current address and navigation shell. Reuse the existing `NotConnected` and `Empty` views. Move `NotConnected` to the shared view module if both the shell and the repository page need it. In a signed-in repository workspace, the terminal shortcut and button open that existing unavailable view in the existing terminal area. Do not mount a terminal, chooser or simulated session. Direct terminal links with a missing project or repository use the same missing-record rule as the rest of that address. Keep compatibility route dispatch and rendering. The sample-only controls do not mount in repository mode. The existing connection menu keeps its readout and cannot mark a reconnecting watch live by invoking a simulated action.
- Render repository links from the watched list and make them searchable through `Switcher`. Use complete folders for the Machines detail and checkout list. A saved repository keeps its existing view tabs. Use an existing repository link keyed by its server identifier even if an old sample-only tree filter was selected. The sample-only filter cannot hide all real repositories by default. Its Command center uses the existing Projects, Open requests and Checkouts widgets with their empty or real bodies. The other widgets of that center use the existing unavailable view in their bodies when their data is absent. Separate the connected-center path before `repoFacts` draws synthetic run statistics. An unknown repository uses the existing missing-repository outcome. A saved repository with zero folders uses the unavailable view before `RepoSetup` can mount. No sample Git, Files, map, preview, repository agent, branch history or terminal should be created by visiting a real repository in sprint 4.
- Display machine facts as reported. Machine detail may show the watched installed versions and read-only agent limit. It cannot infer account sign-in from an empty subscription list or run an unsupported settings write. Keep the existing actual machine actions. The view table also covers the cases where service settings or account controls would only offer simulated writes.
- First write `machines-only routes have no sample data` in `repository-routes.spec.ts` as one test over the route table. Navigate by links and by hash reload with one worker and no host, with no machine for the zero-machine edges, and with `?data=worst`. Assert the corresponding existing headings and empty states, no unsafe dereference, and no network or store writes from opening unavailable views. Exercise the keyboard terminal shortcut and sidebar button. The real connection badge follows the watches and sample-only controls cannot seed or mutate the workspace. Write `saved repositories open from tree and search` in the same file. Add a watched record, find it in the tree and keyboard switcher, open each repository view, verify resolved paths, then remove it. Confirm no synthetic activity or terminal was initialized. Desktop and phone both cover the sidebar drawer and the repository tab row.

**Routes after both snapshots**

The first column follows the hash dispatcher. An unknown page uses the existing unknown-page view. A listed unsupported route uses the existing `NotConnected` with the subject in the copy table. Requested addresses wait at the initial gate before showing a missing record.

| Address or entry | Machines and no repositories | After a repository is saved |
|---|---|---|
| Startup and connection chooser before sign-in | Use the existing Connect or fixed-address probe screen, then the existing sign-in and expiry screens. | The same gates apply. |
| `/login` | Use the existing Login screen and change-server action where offered. | The same screen applies. |
| `/`, `/home`, `/home/<type>`, `/home?type=…` | Root redirects to Home. The attention table shows its existing empty result with or without a filter. | The filters may list real repositories. The requests table stays empty. |
| `/home/projects` and `/projects` | The old address redirects to All projects, with zero projects and its existing empty table. The New project entry opens the existing dialog. | The projects table stays empty. A tree-row or repository-center project entry carries the repository identifier. |
| `/machines` | Show the machine table, pairing and machine actions. The opened row has no repositories or projects. Zero machines gives the existing empty table and Add machine. | The opened row lists only the folders from the repository watch for that machine. |
| `/harnesses` and `/subscriptions` | The alias redirects as today. The Harnesses body uses the unavailable view because account and harness settings are absent. | The same view applies. |
| `/notifications` and `/settings?section=notifications` | The alias redirects as today. Use the existing empty notification result, with no sample write actions. | The same view applies. |
| `/workflows` | Use the unavailable view inside Settings. | The same view applies. |
| `/usage` | Use the unavailable view. | The same view applies. |
| `/settings`, `/settings?section=appearance` or an unknown section | Open the existing Appearance preferences. | The same view applies. |
| `/settings?section=defaults`, `remote`, `stall-watch`, `workflow`, `git`, `connections` or `about` | Keep the section navigation and use the unavailable body for each. The current About body could dereference a missing host and simulate service administration. | The same view applies. |
| `/setup` | Use the unavailable view in the signed-in path. The existing authentication gate still takes precedence. | The same view applies. |
| `/p/<id>`, and each Project view `center`, `board`, `end-result`, `questions`, `status`, `actions`, `design`, `files`, `git`, `settings`, `conversation`, `sprint`, `task`, `team`, `chat`, `message-board`, `agents`, `map` and `terminal`, including child and older addresses | Use the existing missing-project view. No project shell or lead column mounts at any width. | The same view applies until real projects arrive in sprint 5. |
| `/r/<id>` and `/r/<id>/center` | Use the existing missing-repository view. | For a saved repository with a folder, show the existing Command center. Projects and Open requests use their empty bodies. Checkouts uses real folders and remote data. Plan, Sprints merged, Full runs, Usage by project and Base branch use unavailable bodies. |
| `/r/<id>/board` | Use the existing missing-repository view. | Keep the Board tab and show its unavailable body. |
| `/r/<id>/design`, `design/screens`, `design/screen/<id>`, `design/file[/…]`, `design/browser` and `design/architecture`, including older design-file links | Use the existing missing-repository view. | Keep the Design tab and subnavigation. Each uses the unavailable body. |
| `/r/<id>/files[/…]` and `/r/<id>/git[/…]` | Use the existing missing-repository view. | Keep Files and Git tabs and show their unavailable bodies. |
| `/r/<id>/terminal`, the sidebar Terminal button and its keyboard shortcut | The address remains a missing repository. The button and shortcut open the existing unavailable view. | The direct address uses the unavailable view for the saved repository. The button and shortcut keep the same behavior. |
| Any `/r/<id>[/…]` after a folderless save or after removal | Use the existing missing-repository view when absent. | A still-saved folderless record uses the unavailable view. A removed record uses the missing-repository view. |
| Switcher, repository tree and any other address | Switcher has its existing empty search result and page matches. The tree shows the existing empty content. Unknown top-level addresses use the existing missing-page view. | Switcher and tree open only the saved identifiers or existing page destinations. No repository agent column mounts, whether collapsed or wide. |

**Design artifact** [Approved website behavior and changes](../../../docs/specs/2026-11-27-running-an-agent-design.md#the-website-in-sprint-4). [Bounded assignment and view inventory](../../../.foundry/go-rewrite/tasks/plan-go-s4-web-repositories.md).

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Old seeds, timers, or a delayed watch write cross into another server's workspace. | High | Select the data source after discovery. Invalidate scope before cleanup and test delayed callbacks across each boundary. |
| A snapshot lost while disconnected leaves stale rows or a missing-machine crash. | High | Replace snapshots, gate both initial reads, and open every supported route with a worker-only and a zero-machine fixture. |

**Verify** Run `pnpm --filter @lgtm/frontend typecheck` and `pnpm --filter @lgtm/frontend build`. Run `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-discovery repository-session repository-watch repository-routes`. Check desktop and a 390 by 844 pixel Chromium viewport. On the deployed website, Stack checks compatibility and removals with the actual server and machine.

**Sprint 5 extension point.** Keep the discovery service list, the shared authenticated scope, the readiness check and the repository and folder identifiers available to later watches. Sprint 5 can require initial project and agent snapshots, wait for a returned project and lead before navigation, and add a per-lead thread watch under the same reset. This plan adds no sprint 5 service, synthetic lead, or project writer.

**Rollout detail** Step 1 requires the published generation under `@lgtm/api/projects` and the additive discovery fields. Step 2 runs the four browser specs above. The phase 5 merged stack verifies R12 and R13 with one website bundle and the real transport.



**WS-2: Inspect and add a folder from either entry point**

Inspection confirms what the machine found. Saving still goes through the server, which decides whether the folder exists and belongs to the selected repository. The browser can join the saved record without making a second copy.

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

- Define internal `FolderIntent` and `folderIntent`. An intent captures the current session, procedure, machine identifier, exact typed path and optional target repository identifier. Give it one fresh opaque key from the browser's cryptographic random source. Give `inspectFolder(intent)` and `addRepository(intent)` the captured generated client. Forward `Lgtm-Request-Key` and the intent's abort signal. Use `InspectFolderRequest.client_id` and `path`, and `AddRepositoryRequest.client_id`, `path` and optional `repository_id`. Keep an omitted target absent. The same intent sends the same body and key after a lost reply or a wait deadline. Edits, a machine or target change, a new dialog and a different procedure create new intents. An edit away and back creates a new intent.
- Let the wrapper take the current-scope guard as an argument. This permits testing the generated calls without starting the workspace. The caller aborts the replaced attempt and checks session, dialog and intent identity before consuming success or refusal. Current `unauthenticated` uses the common session-ending path. An obsolete result has no effect. Display fixed contract refusals and reuse existing generic network feedback. Render the sentences and facts as text.
- First write `folder procedures keep the intent key` in `folder-calls.spec.ts`. Use a typed transport with a held response and capture the method, body, header and abort signal. Reject an attempt and retry the intent with the same key and exact path, including a path with a space. Assert separate keys across operations and changes to machine, path, target or intent. Test that an omitted target stays absent. Release an obsolete success and an obsolete session error and assert neither reaches the current scope. Risky because this item uses keys and ordering. Regenerate the key on every attempt, share a key between procedures, trim the path and remove the scope check in separate mutations. Require red assertions.

**Item 6. Real Add repository.** Change `src/frontend/src/screens/service.tsx` and `src/frontend/src/server/folders.ts`. Create `src/frontend/e2e/add-repository.spec.ts`. Start after items 3, 4 and 5. Foundation D1 and rules 2, 7 to 9 apply. This item implements the browser piece of newer R1, R2, R4, R5, R7, R12 and R14.

- Reuse the current fields, rows and styles. The shared `defaultFolderMachine` in `folders.ts` selects `d.machine` if it identifies a registered machine, otherwise the first online machine. If neither exists, keep the choice empty. If the selected machine disappears, clear its facts and choose from the new list by the same rule. A selected machine that goes offline keeps its selection and cannot start fresh calls. With no machine, the machine option is blank and the path input and submit are off. Leave the two sample transports and compatibility dialog on their current branch.
- Debounce inspection for 700 milliseconds, using the current sample timing. A plausible absolute or home-relative path starts work. Pass the exact input to the server to decide validity. Clear previous facts and errors synchronously when input changes. Cancel the timer and request when machine, path, target, dialog or session changes. On an inconclusive transport failure, one brief automatic inspection retry uses the same intent. `InspectFolderResponse.facts` supplies `mainBranch` for Base branch, `remote` and `remoteOmitted` for Remote, and `clean` for Status. A detached `checkedOutBranch` never replaces `mainBranch`. Withheld remote uses a dash. Known absence uses the existing absence label. Put current fixed refusals in the existing footer feedback.
- Call `AddRepository` after successful inspection of the current input. Pass the selected machine, exact path and optional known target. Keep an explicit target identifier even if its watched row disappears during editing or an in-flight request. Never turn a missing explicit target into an omitted target. Show the contract refusal if the server rejects it. Suppress concurrent submits from the same intent. On failure keep the fields and add key while that intent is current. A retry of a previously sent add may proceed while the machine is offline, since the server can return its stored result. A fresh add requires an online machine. The server resolves and compares folders. The browser does not derive identity from the name, typed path or machine. Ignore obsolete success and refusal.
- On success, use `AddRepositoryResponse.repository.id` and `AddRepositoryResponse.folder.id` to join. Verify the reply folder identifies the submitted machine. Finish if the matching folder is already projected under that repository. Otherwise wait for the watch. If the join is missing after two seconds, reopen only the repository watch once and wait for a full snapshot from a stream opened after the reply. That snapshot can establish absence after a missed save or removal. Release the form if the record is absent and use the existing missing-repository or removed-machine sentence when applicable. Use the existing generic refusal for a still-present repository with a missing folder. The wait ends on the match, authoritative absence, close, supersession or session end. A unary reply never writes repository records. On a match expand the watched row, reuse the existing success toast with the saved name, and close or restore `d.back` with the saved identifier and chosen machine.
- First write `inspection follows the latest input` in `add-repository.spec.ts`. Hold inspection A, change path and machine, and release B with a dirty detached checkout, distinct main branch and remote. Release A by success and by refusal in separate passes. Assert B's three rows and enabled state survive. Check clean, absent and omitted remotes, exact request path and editing away and back. Inspection alone leaves the tree empty and sends no add call.
- First write `add joins the watched identity in either order`. Hold the save or watch and release them in both orders. Use an existing repository with one watched folder and a second folder with another identifier. A stale reply name or folder list cannot overwrite the watch. Retry after a lost add response, including once while offline. Check one row, same body and same key. Deliver a save reply after a watched removal and let the post-reply refresh prove absence. It cannot resurrect or select the record. Check two typed paths that resolve to one saved identity. Machine details show the watched resolved path.
- First write `add refusal and expiry preserve or clear the form`. Check the fixed mismatch refusal and an explicit target removed during the edit. Assert that the request keeps the explicit target and the input stays in the dialog. Expire the session while inspection or add is pending. Observe one cleanup, two aborted watches and a clean form after sign-in. Replace the dialog while a save is pending and show that the replacement survives. At 390 by 844 pixels with `?data=worst` and long fixture branches, remotes and paths, verify focus, remote wrapping, no horizontal viewport overflow, and reachable submit and Back or Cancel. Repeat at desktop width. Risky because the watch, call and dialog can race. Remove the stale check, substitute the first folder on the machine for the returned folder identifier, copy the reply to the store, and strand the join after a replacement snapshot. Require red assertions for each mutation.

**Item 7. Dialog defaults, return and sidebar entry.** Change `src/frontend/src/{app,dialogs}.tsx`. Create `src/frontend/e2e/repository-entry.spec.ts`. Start after item 6. Foundation D1 and rules 2 and 8 apply. This item implements newer R13 and the browser flow in R14.

- The sidebar's existing plus opens `add-repository` in repository mode. Keep the accessible name and tooltip. Keep both Machines entry points and their opener identifiers. Generic New project uses the first online machine. New project with a machine identifier uses it if present. Repository-specific entries keep the repository identifier. Use `defaultFolderMachine` in both dialogs. Waiting machines stay out of the choices.
- Keep the New project layout and its empty lead controls. Enable submit when the selected machine is online, regardless of goal, repository name or lead. Button and Enter use the copy table's unavailable response and keep the form. A blank selection disables submit and attach. Existing name-field presentation remains inert. This item makes no project or named-repository request.
- The existing Add to a machine entry carries the selected repository, machine and goal in `DialogState.back`. Preserve other current field values as needed to restore the form. Back restores the previous form. A save restores it after item 6's identity join, with the watched repository selected on the chosen machine. Derive the options from the current store.
- First write `add uses the sidebar or the opening machine` in `repository-entry.spec.ts`. Put an offline machine ahead of two online machines. Check that the plus selects the first online one and that the menu and details on the second select the second. Verify that the old server and both sample transports still open Track repository from the plus. Write `new project keeps the goal and reports unavailable`. Enter a unique goal, choose a repository absent on the chosen machine, and test Back and successful attachment. Test button and Enter in separate passes. Expect unchanged goal, selected saved identifier, unavailable feedback and zero project writes or simulated records. Write `both dialogs have an empty machine choice`. Check both with zero and offline-only machines. Assert empty selection, disabled actions and zero requests. After a watched online machine arrives, verify it becomes the default. Check desktop and 390 by 844 pixels with `?data=worst` and long fixture labels. Exercise the phone drawer plus and the nested return. Verify one active dialog, visible focus, reachable goal and footer, and no horizontal viewport overflow. Risky because nested dialogs and inputs change in order. Reverse machine precedence, blank the restored goal and reintroduce the lead-choice gate in separate mutations. Require red assertions.

**Website text introduced or changed in repository mode**

The copy below records new sentences and changed feedback. Existing text is included where the real condition or placement changes. Session and watch failures use the existing sign-in and connection views.

| Place and condition | Exact displayed copy | Authority |
|---|---|---|
| Add repository, Status, clean | “A git repository with a clean working tree” | Existing clean text. |
| Add repository, Status, dirty | “This Git repository has uncommitted changes.” | Approved dirty state. The planner supplies the sentence. |
| Add repository, Remote, absent or withheld | “No remote” for known absence. “—” for withheld. | Existing labels. The contract distinguishes the two states. |
| Add repository, path field or either machine choice with no selectable machine | “Folder” for the path label. The machine choice has an empty label. | Existing field vocabulary and approved empty choice. |
| Add repository, progress and success | “Detecting…”, “Looking at the folder…”, “Adding…”, and “Added {name} on {machine}” | Existing progress and success feedback used with watched data. |
| New project, submit | “Projects are not connected to this server yet.” | Approved response. Use the existing action-refusal toast. |
| Missing records and empty routes | “No open requests”, “No open request matches.”, “No project matches.”, “No projects”, “No repositories”, “No notifications”, “This project does not exist”, “This repository is not known”, and “This page does not exist” | Existing views and empty text. |
| Unavailable route, widget, section or Terminal | “{subject} is not connected yet” and “The service does not send this to the website yet.” | Existing unavailable view. Use “This page” for generic pages and sections, “Files” for Files, and “The terminal” for the terminal entry. |
| Current folder refusal, connection or invalid input | “The machine is not connected.”, “The machine, repository, or folder path is invalid.”, “The request key is invalid.”, and “The request body is invalid.” | Contract fixed refusals. |
| Current folder refusal, target or key conflict | “That request key was already used for a different request.”, “The machine was not found.”, “The repository was not found.”, “The folder does not belong to the selected repository.”, and “The folder is already saved for another repository.” | Contract fixed refusals. |
| Current folder refusal, path facts | “The path is not a folder.”, “The path is not a Git repository.”, “The folder cannot be read.”, and “The folder path cannot be used.” | Contract fixed refusals. |
| Current folder refusal, operation result | “The machine did not answer the folder request.”, “The machine did not take the folder command.”, “The outcome of the folder request is unknown.”, and “The machine was removed.” | Contract fixed refusals. |
| Transport failure or a missing folder after a refreshed snapshot | “The server did not answer.” or “The server refused the request.” | Existing generic feedback for the corresponding condition. |
| Machines Status column and opened row, recovery flag true | “Out of sync” and “This machine's saved history is older than the server's.” | Owner decision of 2026-11-27. |

**Design artifact** [The existing dialogs and approved changes](../../../docs/specs/2026-11-27-running-an-agent-design.md#the-website-in-sprint-4). [The contract's fixed refusals](../go-s4-contract/plan.md). [Current dialog code](../../../src/frontend/src/screens/service.tsx#L443).

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A replay starts a second intent or a stale add overwrites the repository list. | High | Keep the key with the input and procedure. Join by both identifiers. Leave collection writes to the watch. |
| A missing default crashes a zero-machine dialog or a lead choice blocks the approved message. | Medium | Share the real chooser and prove both entry points at both widths with blank, offline and watched machine lists. |

**Verify** Run `pnpm exec playwright test --config src/frontend/playwright.config.ts folder-calls add-repository repository-entry`. Exercise both dialogs on desktop and at 390 by 844 pixels. Use `?data=worst` on sample and real paths. Supply long real-server fixture labels because that query seeds nothing on a real server.

**Rollout detail** Step 3 runs the browser specs. Step 4 adds two distinct checkouts, one dirty, on the deployed stack. Open Add repository from the sidebar for one and the machine's menu or details for the other. Compare the three rows to the checkout facts. Reload and check both saved identities and resolved paths.

**WS-3: Show the machine recovery state**

The Machines page reads the server's diagnosis through the regular watch. A flagged machine has its own status in the table and one reason in its opened row.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 8 | Snapshot and saved frames show the approved status and reason for offline and waiting machines. A later ready admission or removal updates the open page. Remove machine remains available. | `machine recovery follows snapshots and updates`, `recovery state survives stale approval and clears on watched readiness` and merged `TestSprintFourHistoryRecovery`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 8 | Project the recovery flag and display the fifth approved change. | `src/frontend/src/server/sync.ts` `toMachine`, `toJoining` and `watch`. `types.ts` `Machine` and `JoiningClient`. `screens/service.tsx` `Machines` and `MachineDetail`. |

**Item 8. Machines recovery status.** Change `src/frontend/src/server/sync.ts`, `src/frontend/src/types.ts` and `src/frontend/src/screens/service.tsx`. Create `src/frontend/e2e/machine-recovery.spec.ts`. Start after items 2, 3, 4 and 7. This is the final ordinary item. Foundation D1 and D5, rules 2, 5, 7 and 8, and C6 to C9 apply. Its browser part follows newer R9, R11 and R12 and the contract's recovery handoff.

- Read `Client.eventHistoryNeedsRecovery` from `WatchClients` snapshots and saves. Project it into `Machine.eventHistoryNeedsRecovery` and `JoiningClient.eventHistoryNeedsRecovery`. Absence decodes as false. Give the approved status word priority and the same emphasis as Offline. Show the exact reason in the opened row only. Keep the underlying online, offline or waiting state for actions and counting. A flagged waiting machine appears in the Machines table for the approved diagnostic and existing removal action. Keep a minimal waiting row from its actual watch data and reuse the expanded-row container without fabricating a capacity or connection measurement. It stays in the waiting collection for pairing and outside the usable machine list. Unflagged waiting machines use the existing pairing surface. Continue to offer the existing Remove machine action for flagged machines, including a flagged host. The server's `RemoveClient` accepts any standing. Only a watch update changes or clears the rendered flag, and only watched removal deletes the row. Apply the display in repository mode.
- First write `machine recovery follows snapshots and updates` in `machine-recovery.spec.ts`. Start with flagged offline and waiting machines and an older record without the field. Assert the exact table word and expanded reason. Assert registered online counts and reachable Remove machine actions. Apply unrelated saves, then reopen for a replacement snapshot. The flag survives and there is no duplicate diagnostic row. At desktop and 390 by 844 pixels, open the row and verify the reason stays in the detail. Remove a flagged waiting or host machine and assert its row disappears only on watched removal.
- First write `recovery state survives stale approval and clears on watched readiness`. Hold approval with an older flag and deliver a watched transition to a flagged registered machine. Release approval, send an unrelated save, then send a false save representing the later ready admission. Assert the newest flag at each step, normal status and removal of the reason. Reopen with a false snapshot. In a second pass remove instead of clearing and show that late approval and refresh cannot resurrect the row. Risky because approval, snapshots and updates are ordered independently. Read the flag only on initial snapshot, clear it on approval or an unrelated update, and drop the waiting projection in separate mutations. Require red assertions.

**Design artifact** [Recovery decision in the approved design](../../../docs/specs/2026-11-27-running-an-agent-design.md#decisions-of-the-owner). [Recovery note](../../../.foundry/go-rewrite/tasks/note-web-recovery-flag.md).

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A waiting or offline machine loses its flag, or an approval reply replaces the diagnosis. | High | Map full watch records in both collections. Test an older response against a newer save and a replacement snapshot. |
| The diagnostic hides the removal action or overflows the table on a phone. | Medium | Reuse the row and action. Check access and exact copy at both widths. |

**Verify** Run `pnpm exec playwright test --config src/frontend/playwright.config.ts machine-recovery` at desktop and phone widths. The merged Go check exercises the actual later admission and removal. Link owns the durable flag.

**Rollout detail** Step 4 includes item 8 and the merged history recovery fixture.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe is the assignment's six deliverables, its recovery update, the foundation identifiers used in the plan, the newer design's sprint 4 checks R1 to R14, the website entries in both round-three closing lists, the contract handoff's Website row, and the three checklist lines assigned in whole or part to sprint 4. Foundation rule identifiers use the older design's order. Newer R identifiers refer to its sprint proof table.

| Source item | Workstream | Status |
|---|---|---|
| Assignment 1 and contract Website row on discovery, watch replacement, identity, retries and stale replies | WS-1 and WS-2 | Items 1 to 3 and 5 to 6. |
| Assignment 2, fallback modes and no simulation | WS-1 | Items 1 and 4. |
| Assignment 3, dialogs and machine selection | WS-2 | Items 6 and 7. |
| Assignment 4, the original four approved changes | WS-2 | Items 6 and 7 and the copy table. |
| Assignment 5 as revised by the owner note, and the contract Website row on recovery | WS-3 | Item 8 and the copy table. |
| Assignment 6, routes with only machine data | WS-1 | Item 4 and the route table. |
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
| Fit closing list, identity join, late dialogs and individual views | WS-1 and WS-2 | Items 2, 4 and 6. Includes terminal and no lead column at either width. |
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
| 4. Existing views | `mobile-ui` | `website/frontend` | medium | Each listed address reuses an available view inside the shell. | Yes | 3. |
| 5. Folder request identity | `mobile-ui` | `website/frontend` | medium | Two generated calls share request identity and cancellation. | Yes | 2 and generated contract. |
| 6. Add repository | `mobile-ui` | `website/frontend` | medium | One existing dialog wires a keyed read and write to the watch. | Yes | 3, 4 and 5. |
| 7. Defaults, return and sidebar | `mobile-ui` | `website/frontend` | medium | Two form states share selection and one existing transition. | Yes | 6. |
| 8. Machine recovery | `mobile-ui` | `website/frontend` | medium | One additive client field reaches existing rows and the waiting projection. | Yes | 2, 3, 4 and 7. |

Items 3 and 5 can build together. Item 4 can also run alongside 5 after 3 joins. Other shared-file writers follow the dependencies.

Load: budget 24 OpenAI launches. Count one planner, eight test writers, eight builders, five plan-review seats, and two merged-review seats for the focused slice and complete flow. The remaining weekly allowance is unavailable in this planning session. The orchestrator checks it before dispatch and can schedule across the reset if needed. The orchestrator creates and checks `.plans/scopes/website/frontend/scope.json` before dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | Pending | The orchestrator owns the review after the handoff at `attack`. |

</details>

