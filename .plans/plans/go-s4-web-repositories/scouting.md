# Scouting for the sprint 4 website repository plan

## Sources read for this assignment

| Source | Evidence used |
|---|---|
| [Assignment](../../../.foundry/go-rewrite/tasks/plan-go-s4-web-repositories.md), [common planner rules](../../../.foundry/go-rewrite/common-planner.md), and `AGENTS.md` | These files establish the Website boundary, private-text rules and the separate orchestrator handoff. The assignment permits three to nine items. |
| [Recovery note](../../../.foundry/go-rewrite/tasks/note-web-recovery-flag.md) and [running-an-agent design](../../../docs/specs/2026-11-27-running-an-agent-design.md) | Read the entire design. Its sprint 4 section supplies the folder operation, removal, workspace and proof checks. Its sprint 5 website section supplies the extension points. The added decision row and note approve the fifth visible change. |
| [Foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md) | Read the entire design. The plan uses the website and machine boundaries, rule 5 for watch shape, rule 7 for durable ownership and rule 8 for errors. Foundation check identifiers are distinct from sprint 4 check identifiers. |
| [Go rewrite epic](../../epics/go-rewrite/epic.md), [build loop](../../../docs/specs/2026-11-05-build-loop-design.md#4-build) and [issues](../../../docs/issues.md#1632-what-the-design-of-the-go-rewrite-leaves-open) | The team edits `src/frontend/`. Phase 4 supplies tests first, frozen checks and risky mutations. Issues 1632 to 1634 inform authentication, the link and fixture handoffs. Issue 1635 supplies the small-item sizing rule. |
| [Contract plan](../go-s4-contract/plan.md) and [round 2 handoff](../../../.foundry/go-rewrite/results/fold-go-s4-contract-r2.md) | The Website workstream, additive discovery, fixed refusals, generated names and recovery handoff ground the site plan. Website uses the generated ProjectService and no link messages. |
| [Round-three logic](../../designs/running-an-agent/round-3-logic.md#for-the-plans) and [round-three fit](../../designs/running-an-agent/round-3-fit.md#for-the-plans) | The closing lists feed the decision log. The individual-route and dialog findings were checked against current source. |
| [Earlier website scout](../../../.foundry/go-rewrite/results/w2-scout-website.md) | Its folder, lead-choice and machine watch findings were checked against the source. Its live sample observations are earlier evidence, not new browser runs for this assignment. |
| [Website checklist](../../../docs/website-checklist.md#sidebar-and-navigation) and `floorplan/plan.json` | Checklist lines 64, 72 and 161 cover the repository parts of the tree and search and the folder workflow. The floor plan identifies the existing website and watch consumers. |
| Planning skill, template, self-check and work-area sheet | Read the Tier M probes, canonical structure, work-area names, review seat counts and allowance rules. The bounded assignment leaves epic registration, scope construction and review to the orchestrator. |

## Existing source anchors

The application observations in this table are by reading at the checked-out head. Paths are relative to the repository. The website build and browser specs in the plan are future implementation gates.

| Source and symbols | What the source showed |
|---|---|
| `src/frontend/src/main.ts` `mount` call and `window.lgtm`. `src/frontend/src/app.tsx` `mount` and `mountLive` | Main starts the default mode and exposes the store. The default bootstrap runs each seed and `startSimulation` before `server.start`. The legacy live mount starts with empty collections and uses engine snapshots. `src/frontend/src/engine.ts` `setConnection`, `call` and `push` identify that older boundary. |
| `src/frontend/src/data.ts` `seed`, `base`, `emptyState`, `projectOf` and `threadOf`. `src/frontend/src/drafts.ts` `composerDrafts` and `clearComposerDrafts` | The worst-case sample branch reads the query before the hash. The empty constructor already provides empty record collections. The base has browser preferences and mock composer storage. Helpers can insert local thread arrays from reads. |
| `src/frontend/src/actions.ts` `startSimulation`, `createProject`, `answerRequest`, `signIn` and `signOut` | The interval writes activity and connection freshness. Project and request callbacks can run later. Auth actions currently mutate local session after their awaited result. |
| `src/frontend/src/define.ts` `leadSays`. `src/frontend/src/screens/mocks.tsx` `comment`. `src/frontend/src/screens/workflow.tsx` `startRun`. `src/frontend/src/screens/terminal.tsx` `seedTerminal`, `termPlace` and `TerminalPanel`. `src/frontend/src/screens/service.tsx` `RepoSetup`, `Notifications` and `About` | These include delayed compatibility writes, empty terminal initialization, and service bodies with simulated confirmations or data. The real branch must keep their sample writers off. |
| `src/frontend/src/server/api.ts` `Api`, `api`, `onServer`, `openServer` and `CONTRACT`. `src/frontend/src/server/address.ts` `SAMPLE`, `serverUrl` and `rememberAddress` | The client factory checks product and revision, uses credentialed network requests and an in-page router, and installs the selected clients and address inside the async function today. |
| `src/frontend/src/server/sync.ts` `toMachine`, `toJoining`, `watch`, `reason`, `connect`, `signIn`, `signOut`, `changeServer` and `createClientToken` | The current watch partitions waiting clients and replaces its lists on snapshots. It handles backoff and sets the workspace's one readiness bit. A stale frame can reach the update because no check runs immediately before that update. `captureSession` and the repository-only refresh in the plan are new functions in this file. |
| `src/frontend/src/types.ts` `Machine`, `JoiningClient`, `ServerState`, `Repo`, `DialogState` and `State` | The repository's existing `folders` shape is a machine-to-path lookup. Waiting records and machine choices are represented separately. `repositoryMode`, `folderRecords` and the two recovery fields are planned additions. |
| `src/frontend/src/app.tsx` `App`, `Sidebar`, `ProjectTree`, `NotConnected`, `toggleTerminal` and `ProjectView`. `src/frontend/src/lib.tsx` `parseRoute` and `update` | The hash dispatcher and its aliases supply the route inventory. The plus currently opens remote tracking. The sidebar includes `MockControls` and `Live` can simulate a connection state. The old engine branch already uses the unavailable view. The normal workspace gate uses `server.synced`. |
| `src/frontend/src/screens/home.tsx` `Home` and `AllProjects`. `src/frontend/src/screens/auth.tsx` `Login` and `FirstRun`. `src/frontend/src/screens/connect.tsx` `Connect` | Home and All projects have true empty states. First run expects a machine and fabricates setup data in compatibility mode. Login can choose another server. |
| `src/frontend/src/screens/repo.tsx` `RepoView`. `src/frontend/src/screens/center.tsx` `RepoCenter`, `repoFacts`, `Projects` and `Checkouts` | Missing repositories already have an empty page. Folderless ones go to simulated setup today. The normal center fabricates run history. The existing projects and checkout widgets can display real or empty data. |
| `src/frontend/src/screens/service.tsx` `Machines`, `MachineDetail`, `PairMachine`, `AddRepository`, `TrackRepository`, `RepoSetup`, `ServiceSettings` and `SettingsShell` | Machines offers adding from both its menu and detail. Pairing sees waiting records. The folder dialog prefixes a sample path, fakes inspection and writes into the repository array. Remote tracking simulates search and creation. Some settings screens offer simulated writes. |
| `src/frontend/src/dialogs.tsx` `Switcher` and `NewProject`. `src/frontend/src/project-lead.tsx` `useProjectLead`. `src/frontend/src/ui.tsx` `Dialog`, `DialogHost` and `Empty` | Search already reads `s.repos`. The inner dialog return state carries repository, machine and goal. New project currently gates on a lead and dereferences its default machine. The dialog host picks the mounted component by type. |
| `src/mock-server/src/engine.ts` `createEngine`. `src/mock-server/src/serve.ts` `handler`. `src/server/internal/api/{session,server}.go` `ContractRevision`, `GetServer` and `webProcedures` | The mock has in-page and network transports using existing generated services. The server registers allowed procedures separately from discovery. Its sign-in and sign-out handlers set or expire the same cookie. The plan needs additive registration by the Projects team. |
| `src/server/internal/machines/watch.go` `WatchClients`. `src/server/internal/machines/project.go` `ProjectWithPresence`. `src/server/internal/machines/records.go` `RemoveClient` and `ApproveClient`. `records_test.go` `TestRemoveClient` | The authenticated watch projects registry and presence. The approval reply can carry an older projection. RemoveClient accepts any standing, including waiting and server-host records. Watch removal supplies the deletion observation. |

## New file targets and reference patterns

| New target | Nearest existing pattern |
|---|---|
| `src/frontend/src/server/repositories.ts` | Use `server/sync.ts` `toMachine` and `watch` for full projection, replacement, update and removal. |
| `src/frontend/src/server/folders.ts` | Use `server/api.ts` `api` and `server/sync.ts` `createClientToken` and `reason` for generated calls and failure feedback. The generated module comes from the reviewed contract. |
| `src/frontend/playwright.config.ts` | Use root `playwright.config.ts` for Chromium, test matching and bounded workers. Keep the test directory in the Website team. |
| `src/frontend/e2e/support/repository-server.ts` | `src/mock-server/src/serve.ts` `handler` hosts Connect on a local listener. `packages/web/e2e/new-project.spec.ts` `replyGate` holds a result while watching another side. Reuse the pattern with generated messages and no application test hooks. |
| `src/frontend/e2e/{repository-discovery,repository-session,repository-watch,repository-routes,folder-calls,add-repository,repository-entry,machine-recovery}.spec.ts` | `packages/web/e2e/new-project.spec.ts` `fixture` uses a built site and held server replies. `packages/web/e2e/phone-flow.spec.ts` shows the existing narrow-flow browser pattern. The new config and support file belong to item 1. |

## Probes run during planning

- `git rev-parse --short HEAD` reported `44c931d9`. Independent source checks used `rg -n` and `sed -n`. The available Playwright executable reported version 1.64.1.
- The epic's Website row is still planned, and `.plans/scopes/website/frontend/scope.json` is absent. The bounded planner cannot change either. The handoff requests both checks from the orchestrator.


## Tier M planner self-check

| Gate | Observation |
|---|---|
| 1. Decisions | Read the 15 dated rows and checked each disposition and assumption. The approved scope and the included and excluded records are explicit. |
| 2. Acceptance | Checked eight criteria against their named first tests or stack observations. The implementation items specify results and risky mutations where applicable. |
| 3. Design | Checked that the local source paths in the three plan documents resolve. The existing dialog sections and the recovery note supply the design evidence. |
| 4. Traceability | Checked all six deliverables, the recovery update, the contract handoff, the applicable design checks, the checklist and the review closing lines against the three workstreams. |
| 5. Risks | Checked a filled risk and mitigation table for each workstream. |
| 6. Verification | Checked one rollout order and three verification commands with desktop or phone observations. |
| 7. Review | Verified the empty round's handoff entry. The assignment gives review execution and findings to the orchestrator. |
| 8. Anchors | Checked existing task anchors by searching each named file and checked new paths against neighboring patterns. |
| 9. Guardrails | Checked the cited rules against the current repository instructions. |
| 10. Sizing | Checked eight medium units, the scope identifier, one-vendor launch count and current allowance caveat. Scope materialization and the allowance reading belong to dispatch. |
| Template and linkage | Checked the frontmatter, document order, five folds and the existing epic row. The orchestrator changes the epic row to linked. |

The mandated private-text script exited 0 on tracked files. The same script's predicates also exited 0 with the three untracked plan documents as explicit input. The planner reviewed those documents for relative paths, dated decision sources and interface copy. A whitespace check of the tracked diff also exited 0. These are checks of the plan and its handoff. The browser, database and deployed checks in the plan are future build acceptance.
