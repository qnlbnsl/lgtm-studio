# Which leftover points of the website's open issues still hold

This is a read-only review of issues 1572, 1593 to 1595 and 1613 to 1620 against the code on 2026-11-24. Nothing was run in a browser.

Paths are relative to `src/frontend/src/` unless they start with `src/`, `design/` or `docs/`. Nine files had uncommitted edits by someone else during the review (`app.tsx`, `mission.tsx`, `center.tsx`, `ui.tsx`, `data.ts`, `types.ts`, `workflows.tsx`, two style files), and `mission.tsx` moved by about ten lines while it was read. Line numbers in those files may be a few lines off. The one calculation that was run was the contrast ratio.

## 1. Table

The verdict column uses four words.
The word **holds** means that the point is still true in today's code.
The word **fixed** means that the code no longer has the fault.
The word **part** means that the point is partly fixed.
The word **run** means that reading the code cannot settle the point, and something has to be run.

The size column uses three letters.
The letter S means a small fix: under an hour, in one or two files, with no decision needed.
The letter D means that the point needs the owner's decision.
The letter L means a large piece of work.

### 1572, the owner's open questions

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| O1 | Home: what "Needs attention" is for | holds | `screens/home.tsx:123` | D: what Home is for |
| O2 | Three controls at top right | part | The top bar now holds the bell and "Live" only, `app.tsx:330-331`. No Home control is there. | D: a job for the two left |
| O3 | Two top bars, branch menu out of place | part | No branch menu is in the row of views; `StageMark` returns only the list button, `screens/mission.tsx:491`. Two bars remain. | D |
| O4 | Stage too far right | fixed | Status tab carries the stage, `app.tsx:432`. Waits for the owner's look. | — |
| O5 | Sidebar plus adds a project, repositories have no place | holds | `app.tsx:241`; "Add repository" exists only on Machines, `screens/service.tsx:226` | D: where a repository is added |
| O6 | End result short and detailed form | holds | One form only, `screens/mission.tsx:392` | D |
| O7 | Phases tab; command center layout "not drawn" | fixed | Tab is named "Overview", not "Now", `screens/workflow.tsx:931`; command center drawn, `screens/center.tsx` | — |
| O8 | Test runs becomes Actions | fixed | `screens/workflow.tsx:923`. Waits for the owner's look. | — |
| O9 | Agents view becomes message board | fixed | `app.tsx:481`, `screens/project.tsx:607` | — |
| O10 | Design view may belong in a project, may clash with Mocks | fixed | Design is one project view that holds the Mocks pages, `app.tsx:436`, `screens/mocks.tsx:559` | — |
| O11 | Approvals live in Questions | holds | `screens/questions.tsx:15` | D |

### 1572, blind review findings and code points

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| B1 | Run title cut and run state an icon in the list | holds | `screens/workflow.tsx:738-740`; clamp at two lines `css/workflow.css:139` | S |
| B2 | Deciding control below the fold | holds by order, measure needs a run | Decision comes after Command, Background and Impact, `screens/questions.tsx` (the `return` of `QuestionDoc`) | D: move the decision up |
| B3 | State is a pill here and a word there | holds | Pill `screens/workflow.tsx:540`, `screens/mocks.tsx:331`; word `screens/workflow.tsx:589` | D: one rule |
| B4 | Sprint tabs underlined, other tabs filled | holds | `css/base.css:386` against `css/workflow.css:7` | D: pick one |
| B5 | Mocks says "Approved" twice | holds | `screens/mocks.tsx:583` and `:356` | S |
| B6 | Long answer cut after two lines in the list | holds | `css/screens.css:404` | S |
| B7 | Two facts look like settings | fixed | They are a list of facts above the rows, `screens/workflow.tsx:873-877` | — |
| B8 | "Stage" of six and "Phase" of eight | holds | `screens/workflow.tsx:436`, `app.tsx:432` | D: one list or two |
| C2 | Series colour tokens not in the design document | fixed | `design/DESIGN.md:1666` | — |
| C3 | Sample gives findings to reviewers in turn | holds | `screens/workflow.tsx:47-48` | L (engine) |

### 1593

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1 | Project's tokens and turns | part | Tokens are on the command center, `screens/center.tsx:202`. The count of turns is nowhere. | S |
| 2 | Subscriptions with share used | fixed | `screens/center.tsx:205-211` | — |
| 3 | "May be stuck" count and row warning | part | The word shows on an agent, `data.ts:584`. The count is only in the live Agents tab, `screens/messages.tsx:655`. | S |
| 4 | Tokens of one agent | holds | `screens/project.tsx:619-620` | S |
| 5 | Message and stop buttons on an agent's row | holds | Rows are links only, `screens/workflow.tsx:372`; Stop is in the thread menu, `app.tsx:358` | D |
| 6 | Team filter, lanes or table | holds | Only in the live tab, `screens/messages.tsx:640` | D |
| a | Agent's page does not name the agent under 1100 | holds | `screens/project.tsx:616-622` | S |
| b | Task table wider than the page at 1440 | holds by reading, width needs a run | Plain table with no stacking rule, `screens/project.tsx:653`, `css/screens.css:416` | S |
| c | A commit opens Git, not that commit | holds | `screens/git.tsx:681`; same on the command center, `screens/center.tsx:220` | S |
| d | Shared worktree shows only the task's files | holds | `screens/git.tsx:652` | D |

### 1594

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1 | Only four sprints name their statements | holds | `data.ts:142-145` | S |
| 2 | Terminal stays open on a change of project | holds | A preference, `app.tsx:456` | D |
| 3 | Terminal outside a project shows nothing | holds | Toggle `app.tsx:259`; panel drawn only in a project, `app.tsx:514` | D |
| 4 | Status layouts not drawn side by side | holds | Strip only, `screens/workflow.tsx:425` | D |
| 5 | Mocks and Design, one view or two | fixed | One view, `app.tsx:392` and `:436` | — |
| 6 | Files stays | nothing to do | `app.tsx:445` | — |
| 7 | Under 1100 the conversation button is an icon | holds | `css/mission.css:55` | S |

### 1595 (open faults)

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1 | Live website uses the older words | holds | `design/mock/js/app.js:259-264` | L (builders) |
| 2 | End result has one address for two places | holds | `app.tsx:428`, `:454`, `:466` | D |
| 3 | Wrong sprint tab opens the first tab, address stays | holds | `screens/project.tsx:352` | S |
| 4 | Conversation with an unknown agent says nothing | holds | `app.tsx:417`, `:470`. It now shows the command center, not the board. | S |
| 5 | Team page marks no view | holds | `app.tsx:457` | S |
| 6 | Pages outside a project are titled "LGTM Studio" | part | A repository's pages name themselves, `screens/repo.tsx:25`. Home, Machines and the rest do not, `app.tsx:594`. | S |
| 7 | Filter, open phase and terminal not in the address | part | Actions and Browser keep theirs, `screens/workflow.tsx:710`, `screens/browser.tsx:29`. Board, Status and terminal do not, `screens/project.tsx:72`, `screens/workflow.tsx:416`. | S each |

### 1613

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| R1-R10 | Ten groups of records still in the page | holds | The contract has four files only: session, clients, map, preview, `src/contracts/proto/lgtm/v1/` | L |
| — | 395 places ask `live.on` | holds, now 411 | count by grep | L |
| P1 | Address scheme rule; `:4180` fails from HTTPS | holds | `server/address.ts` (`serverUrl`); mock server is `node:http`, `src/mock-server/src/serve.ts:9` | D |
| P2 | Cookie is `SameSite=Strict` | holds | `src/mock-server/src/engine.ts:73` | D |
| P3 | `ReconnectClient` | holds | `src/contracts/proto/lgtm/v1/clients.proto:28` | D |
| P4 | "Master" and "Worker" with no roles in the contract | holds | `server/sync.ts:18`, `screens/service.tsx:320` | D |
| P5 | A name may repeat a waiting client's name | holds | `src/mock-server/src/engine.ts:138` | D |
| K1 | Reads fonts, CodeMirror, plans, workflow from `design/mock/` | holds | `css/tokens.css:4`, `screens/workflows.tsx:39`, `src/frontend/vite.config.ts:15` | S to L |
| K2 | `ui.tsx` imports the store and actions | holds | `ui.tsx:8` | L |
| K3 | Page removes a machine's subscriptions itself | holds | `screens/service.tsx:273-276` | L (follows R1) |
| K4 | Preview-state menu sets the session in the page only | holds | `app.tsx:137-140`, `actions.ts:383` | S |
| K5 | No tests | holds | No test file under `src/` | D |
| K6 | Documents still say `design/mock-vite/` | holds | Four files in `design/builds/`, and issues 1572 and 1595 themselves | S |

### 1614

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1 | Sample map comes from `LGTM_MOCK_MAP` | holds | `src/mock-server/src/serve.ts:24` | D |
| 2 | Live website hides the map | holds | `app.tsx:396`, `screens/mocks.tsx` (`!live.on && view === 'architecture'`) | L |
| 3 | Not served from a separate hostname | holds | `src/mock-server/src/serve.ts:84-94` | L |
| 4 | Floorplan's "Ask" bar | run | The page is not in this repository | D |
| 5 | Build leaves no line in the conversation | holds | `screens/map.tsx:31` | S |
| 6 | Cannot say whether the map matches the code | holds | `screens/map.tsx:50` | L |
| 7 | Nine entries in the row | holds, for another reason | Map is now a page of Design; the command center made the ninth, `app.tsx:426-447` | D (issue 1592) |
| 8 | No map icon | holds | `screens/map.tsx:37` | S |
| 9 | No blind review or space checks | run | — | — |

### 1615

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| N1 | Filters by status, branch, event, starter | holds | Action only, `screens/workflow.tsx:723` | D |
| N2 | Pages of runs | holds | `screens/workflow.tsx:736` | D |
| N3 | Stop a run, rerun failed jobs | holds | Only "Run action", `screens/workflow.tsx:730` | D |
| N4 | Turn an action off | holds | `screens/workflow.tsx:756-760` | D |
| N5 | Summary of a run | holds | `RunDetail`, `screens/workflow.tsx:641` | D |
| N6 | Caches, runners, usage | holds | none in the view | D |
| O1 | List 208 pixels wide, title cut after two lines | holds, width needs a run | `css/workflow.css:139` | S |
| O2 | Same four actions for every repository | holds | `actions-data.ts:27` | S |
| O3 | Sample data in the page | holds | `actions-data.ts` | L |
| O4 | No blind review | run | — | — |

### 1616

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1 | Status forgets phase, round and filter | holds | `screens/workflow.tsx:416`, `:504` | S |
| 2 | Message board forgets search and filters | holds | `screens/messages.tsx:351` | S |
| 3 | Board forgets search and epic | holds | `screens/project.tsx:72` | S |
| 4 | Mocks forgets the open change and the data filter | holds | `screens/mocks.tsx:448`, `:487` | S |
| 5 | Git forgets the open file and commit | does not hold by reading | Both are kept in the store, `screens/git.tsx:706-708`, `:788-789`. A walk in a browser would confirm. | — |
| 6 | No view returns to its scroll | holds | No scroll is stored anywhere | L |
| B1 | Task from its own sprint shows the arrow to the board | holds | `screens/project.tsx:408` | D |
| B2 | Back cuts the owner's name in the Git bar at 1000 | run | `screens/git.tsx:1065`, `:1079` | S |
| B3 | Git with no worktrees and End result in Define have no Back | holds | `screens/git.tsx:1041-1053`; no `Back` in `screens/mission.tsx` | S |
| B4 | Back on Review rounds cannot show | fixed for Review rounds | Links now open it from other views, `screens/center.tsx:190`. The mock's one screen still has no link from another view. | — |
| B5 | Phone has no Back in Files and Questions | holds | `screens/files.tsx:268-269`, `screens/questions.tsx:149-150` | D |
| B6 | A file chosen in the tree replaces the history entry | holds | `screens/files.tsx:210` | D |
| B7 | A link counts only within one second of a click | holds | `lib.tsx:146` | S |
| B8 | Keyboard move from the search shows no Back | holds | Only clicks are watched, `lib.tsx:140` | S |
| B9-B10 | Live layout unchecked, no tests | run | — | — |

### 1617

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1 | Waiting assignment is only a dot | holds | `app.tsx:447`, `:497`; Questions lists questions and approvals only, `screens/questions.tsx:15` | D |
| 2 | "Delete team" is an icon in Settings | holds | `screens/service.tsx:1689` | D |
| 3 | Cannot choose another workflow | holds | `screens/workflow.tsx:874` | D |
| 4 | No project notifications or usage limit | holds | none in `ProjectSettings`, `screens/service.tsx:1470` | D |
| 5 | Only previews come through the contract | holds | `screens/browser.tsx:27` | L |
| 6 | Preview command set from the website | holds | `screens/browser.tsx` (`savePreview`) | D (issue 1618) |
| 7 | Preview has no worktree of its own | holds | No worktree field in the draft, `screens/browser.tsx` (`Draft`) | D |
| 8 | Live layout shows General and Agents as one page | holds | `app.tsx:493` | L |
| 9 | Three pages scroll at 1000 by 680 | run | — | S |
| 10 | Worst-case data and space checks not run | part | The same entry says later that the space checks ran. Worst-case data is not recorded as tried. | — |
| 11 | A row's help covers the row under it | holds | `css/base.css:674` (absolute, `top:100%`) | S |
| 12 | Select beside "Use for Plan…" has no visible name | holds | `screens/service.tsx:1637` | S |
| 13 | Product's own words explained only in help | holds | `screens/service.tsx:1196`, `:1680`, `screens/workflow.tsx:898`, `:908` | D |
| 14 | "Delete project" gives its reason on hover only | holds | `screens/service.tsx:1711` | S |
| 15 | Previews and General are mostly empty | run | — | D |

### 1618

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1-4 | Own hostname, every path, WebSocket, end addresses on stop | holds | Sandbox without an origin, `screens/browser.tsx` (the `iframe`), `src/mock-server/src/serve.ts:94`; no `upgrade` handler in the mock server | L |
| 5 | Rule for a preview's command | holds | none | D |
| 6 | Sizes cannot turn or be typed | holds | `screens/browser.tsx:15-19` | S |
| 7 | Not tried with a real product | run | — | — |
| 8 | Reload loads the first page | holds | `setTurn` opens a new address, `screens/browser.tsx` (`Reload` button) | L |
| 9 | Row of Design pages has seven entries | holds | `screens/mocks.tsx:559-560` | D |
| 10 | Flow F-25 not walked | run | — | — |
| 11 | "Pick an element" picks nothing | holds | `screens/browser.tsx` (`picking` is only toggled) | S to hide, L to build |
| 12 | A plain address with no command | holds | Command is required, `screens/browser.tsx` (`save`) | D, then contract |

### 1619

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1 | Row of views is a `tablist` that holds a button | holds | `app.tsx:531`, `ui.tsx:1136`; same in `screens/repo.tsx:59` | S |
| 2 | Faint text below 4.5 to 1 | holds | `css/tokens.css:87` and `:94` give 4.20 to 1; on the raised colour, `:88`, it is 3.77 | D: a new faint colour |
| 3 | Three composer menus named without their words | holds | `ui.tsx:1110-1112` | S |
| 4 | Editor holds Tab, and its focus ring is removed | holds | `editor.tsx:88`, `:29` | S |
| 5 | Seven labels cut under text spacing | run | — | S |
| 6 | Worktree folder cut in Files | holds | `screens/files.tsx:241` | S |

### 1620

| # | Point | Verdict | Evidence | Size |
|---|---|---|---|---|
| 1 | "Open in a new tab" hidden on sample data | holds | `screens/browser.tsx` and `screens/map.tsx:58` (`!frame.url.startsWith('data:')`) | D |
| 2 | One picker, three looks | holds | `css/git.css:4`, `css/browser.css:5-8`, `css/files.css:58-60` | S |
| 3 | "Approved" pill on every row | holds | same as 1572 B5 | S |
| 4 | "Request changes" does not say on what | holds | `screens/mocks.tsx:594-596` | S |
| 5 | Design file opens at 20 percent | holds | `screens/design.tsx:561` | S |
| 6 | Design file's name is cut | holds | `css/design.css:9` | S |
| 7 | Line beside Generate is empty | holds, by the owner's word | `screens/git.tsx:755` | — |
| 8 | Architecture's icon button has no word | holds | `screens/map.tsx:55` | S |
| 9 | Colour pairs not measured | run | — | — |

## 2. The ten points worth fixing first

1. **Terminal in the sidebar does nothing outside a project** (1594.3). It needs a decision: hide the item there, or open a terminal for the server.
2. **The plus beside "Repositories" adds a project** (1572 O5). It needs a decision on where a repository is added.
3. **"Pick an element" turns on and picks nothing** (1618). Hiding it is a small fix; building it is large.
4. **A commit row opens Git, not that commit** (1593c), on the agent's page and on the command center. Small: set the commit in `gitUi` before the move.
5. **"Approved" shows on every row and again above the table** (1572 B5, 1620). Small.
6. **A run in the Actions list has an icon for its state and a cut title** (1572 B1, 1615 O1). Small.
7. **In a narrow window the agent's page names no agent, and the conversation button is an icon** (1593a, 1594.7). Small.
8. **Status, the board and the message board forget their choice on Back** (1616.1 to 1616.3). Each is small; put the choice in the address as Actions does.
9. **Wrong addresses fail silently, and the team page marks no view** (1595.3, 1595.4, 1595.5). Small.
10. **Faint text is under the contrast bar** (1619). One token, but it needs the owner's approval of the new colour because it changes every faint text.

Next after these: the two looks of tabs (1572 B4, a decision), the `tablist` role and the composer menu names (1619, small and invisible to the eye).

## 3. Issue text that should be updated

- **1572**: O4, O7, O8, O9 and O10 are built. O7 says the first tab is "Now"; it is "Overview", and the command center it calls "not drawn" exists. O3's branch menu is gone. O2 has two controls left, not three. B7 is fixed. The series tokens are in `design/DESIGN.md`. The entry still says `design/mock-vite/`.
- **1593**: point 2 is fixed, and point 1 is fixed for tokens only. Point 3's warning shows; only the count is missing.
- **1594**: point 5 is settled, Mocks and Design are one view. Point 6 needs nothing.
- **1595**: the "Where" line names `design/mock-vite/`. Fault 4 now shows the command center, not the board. Fault 6 no longer covers a repository's pages. Fault 7 no longer covers the Actions filter.
- **1613**: "reads only the session and the machines" should add the map and the previews. The count of `live.on` is 411, not 395.
- **1614**: the Map view is now the Architecture page of Design, so the title and point 7 are out of date.
- **1616**: point 5 (Git) does not hold by reading. Button point 4 no longer holds for Review rounds.
- **1617**: "the space checks were not run" contradicts the review paragraph in the same entry.
