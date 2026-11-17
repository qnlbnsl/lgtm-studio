# The rebuilt LGTM Studio mock

This folder is a working mock of the whole LGTM Studio web page. The owner asked for it on
2026-11-10. Opened from this folder, it does not connect to the engine and it never calls an agent.
Every screen works on data held in the browser's memory.
The live website, described below, runs the same files on the engine's records.

## How to open it

Run `pnpm --filter @lgtm/studio build` from the repository root first.
This generates the shared workflow validator and the sample default from their source packages.
Then serve the `design/` folder, which is the parent of this folder.

```
cd design
python3 -m http.server 4178 --bind 127.0.0.1
```

Then open `http://127.0.0.1:4178/`, which opens the mock at `http://127.0.0.1:4178/mock/`. The generated assets are local files.
The mock loads nothing from the external network.

Changes you make stay until you reload the page. The colour mode, the density, and the
sizes of the panels are remembered in the browser. The "Mock controls" button at the foot
of the sidebar switches the connection state, the login state and the first run, and it
resets everything.

## The live website built from this mock

The same files also run as the live website, which reads the engine's real records.
The package `packages/studio` copies this folder, adds the connection to the engine, and
the engine serves the result as the website, at the root address.
The build generates the workflow validator and default in this folder.
The mock still opens on its sample data.

Build the live website from the repository root:

```
pnpm --filter @lgtm/studio build
```

Then start the engine with a web password, as the root `README.md` describes, and open
`http://127.0.0.1:8787/`.
The page asks for the web password, and it shows the engine's repositories, projects,
teams, agents and open requests after you sign in.
An old address under `/studio/` is sent to the root.

In live mode the page has no "Mock controls" button and no sample data.
Actions that change something are not connected to the engine yet.
Such an action shows the notice "This is not connected to the service yet. Nothing was
changed." and leaves the page as it was.
The Terminal and Files tabs and the first run are connected in the engine build.
`js/engine.js` describes the connection object that `mount()` accepts in live mode.

## Using it as a package

This folder is also a local npm package named `lgtm`, so another project can load the
whole website and build on it. It is the basic version of LGTM Studio. The package has no build step and no dependencies.

- Install it by path, for example `npm install ../lgtm-studio/design/mock`. npm links the
  folder rather than copying it, so the links to `design/fonts/`,
  `third_party/htm-preact/` and `third_party/codemirror/` keep working.
- Load the stylesheets that `index.html` loads, from the package's `css/` folder.
- Import `mount` from `lgtm` and call it with an empty element. It seeds the sample
  data, starts the simulation, renders the website, and returns `{ store, act }`.
- A browser page without a build step maps the package's name to its folder with an import
  map, such as `{"imports": {"lgtm": "/node_modules/lgtm/js/app.js",
  "lgtm/": "/node_modules/lgtm/"}}`.

This folder's own page starts the mock the same way, through `js/main.js`.

`mount` takes a second, optional argument, `{ connection, slots }`:

- **`connection`** receives every call the website would make to the engine. The
  website's actions report them through `js/engine.js` as `call(tag, payload)`, using
  the request names from `@lgtm/contracts`, such as `prompt`, `answer`, `stop` and
  `create`. The simulation's updates go through `push(kind, payload)`, such as
  `agent.message` and `agent.activity`. On mounting, the website calls `snapshot`, then
  `subscribe`. Without a connection, the calls go nowhere and the mock behaves as before.
- **`slots`** fills three named places with a component, by name. `sidebar.head` sits
  under the sidebar's brand row. `sidebar.account` sits in the sidebar's foot, above the
  owner's row. `topbar.connection` replaces the "Live" badge in the top bar.

`unmount(el)` removes the website from the element and returns to the default connection.

## What is in it

| Screen | Address after `#` |
|---|---|
| Home, one table of everything that waits for the owner | `/home` |
| All projects, one table of every project | `/projects` |
| Project board, as columns, a table, or a roadmap | `/p/console/board`, `/p/console/board/table`, `/p/console/board/roadmap` |
| Sprint page, with its plan and tasks side by side | `/p/console/sprint/sp7` |
| Task and team pages | opened from the board and the sidebar |
| Workflow, the build loop's phases | `/p/console/workflow` |
| Review rounds, Test runs, End result, and the workflow's settings | `/p/rules/workflow/review`, `/p/console/workflow/runs`, `/p/apiref/workflow/result`, `/p/console/workflow/settings` |
| Mocks, for a project in its Design stage | `/p/phone/mocks`, and one screen at `/p/phone/mocks/screen/requests` |
| Chat with any agent | `/p/console/chat` |
| Messages, the agents' message board | `/p/console/messages` |
| Agents, the roster by situation | `/p/console/agents` |
| Terminal, with owner shells in local worktrees in the connected website | `/p/console/terminal` |
| Files, the inspected worktree as a folder tree and a small editor that saves and commits one file | `/p/console/files`, and one file at `/p/console/files?f=README.md` |
| Git, with generated commit and pull request text | `/p/console/git` |
| Project settings, with the lead's model and the work assignment table | `/p/console/settings` |
| Repository design page, the design file on a canvas | `/r/lgtm/design`, and `/r/<repository>/design/<category>/<item>` |
| Machines, Subscriptions, Notifications, Service settings | `/machines`, `/subscriptions`, `/notifications`, `/settings` |
| Login and first run | `/login`, `/setup` |

A repository without a design file, such as `atlas-docs`, shows the Design page's empty
structure at `/r/atlas-docs/design`.

## How the files are arranged

- `index.html` loads the stylesheets and `js/main.js`, which calls `mount()` from `js/app.js`.
- `package.json` names the folder `lgtm` and lists the paths another project may import.
- `css/tokens.css` holds every colour, size and duration. No other file may hold a literal
  colour.
- `css/base.css` holds the shell and the shared components. Each screen has its own
  stylesheet beside it.
- `js/lib.js` holds the store, the router and small helpers.
- `js/data.js` holds the sample data and the functions that read it.
- `js/actions.js` holds every change to the data.
- `js/ui.js` holds the shared components.
- `js/files-data.js` holds the Files tab's listings, file text, and a small highlighter.
- `js/screens/` holds one file for each screen. `js/screens/design.js` holds the Design
  page and the canvas that the Mocks tab reuses for its large preview.
- `plans/` holds plan pages in HTML. `sp-07-phone-layout.html` is the sample plan for
  SP-07, and `plan.css` styles it.
- `js/icons.js` is generated from Lucide. `vendor/htm-preact` is a link to
  `third_party/htm-preact/`. `fonts` is a link to `design/fonts/`.

## Rules the mock keeps

Agents are named by their role, such as Project lead and Builder 1. No agent has an
invented personal name. The page's own text never says that it is a mock or a sample. The
layout and density rules are in the "Layout and density" section of
[../DESIGN.md](../DESIGN.md).

## Notifications in sample and live mode

The Notifications page configures ntfy and email independently.
Email uses the owner's mail server.
Both carriers have an enable switch and a test action.
Quiet hours have a start, an end, a time zone, and a failure override.
Merge reviews and sprint completions use the engine's existing events.
The sample mode simulates delivery without a network request.
The live website sends each action through the authenticated client application programming interface (API).
Secret fields never contain a stored secret.
See [the notification guide](../../docs/notifications.md) for the transport and API details.

## The connected project terminal

The engine build connects the existing Terminal screen to owner-only shells.
The Project terminal switch in Service settings starts off.
An enabled terminal uses the registered local worktree folder and the service's shell.
The xterm.js renderer handles colours, cursor movement, and full-screen programs.
The build adds the page's style nonce before the renderer attaches its style elements.
The content security policy is unchanged.
Output reaches only the opening owner connection unchanged, under the owner's ruling of 2026-11-14.
A printed secret appears on the owner's screen and in any scrollback the owner's tools keep.
The service stores neither output nor input.
Sessions end when the page reloads, its connection grace period expires, the owner signs out, the switch turns off, or the service stops.
Remote shells, agent terminal watching, agent terminal takeover, and session reconnection need engine support.
Those controls are disabled in the live screen, with the tooltip “Engine work needed”.
The sample screen and its pretend shell remain unchanged when opened without the engine.

## GitHub project merge reviews

The connected Git tab uses the existing review ribbon and merge dialog for GitHub pull requests.
The ribbon and dialog show the prepared mode, pull request number, link, and state.
The dialog displays the generated title and description as recorded data.
The title and description editors still refuse live edits.
An explicit Git refresh pushes newer commits for an open GitHub review and replaces that review.
Automatic background reads remain read-only.
Approval records the remote merge and reports whether the local base was updated.
The sample screens continue to work without an engine or a GitHub connection.
