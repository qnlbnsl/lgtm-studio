# Machines: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

Everything in this section comes from `design/mock/js/screens/service.js`, in the functions `Machines`, `MachineDetail`, `PairMachine` and `AddRepository`.

- The page head shows the title "Machines", a count such as "2 of 3 online", and one primary button, "Add machine".
- The page is one table with ten columns: Machine, Role, Status, Address, Harnesses, Agents, CPU, Memory, Last seen, and a menu of actions. CPU means central processing unit.
- The Machine cell holds a chevron, a server icon, the name, and a "This computer" badge on the computer that the owner is using.
- Role is "Master" or "Worker". Status is a dot with "Online", "Offline" or "Pairing". An offline row takes the warning colour.
- The Harnesses cell shows one tag for each installed harness. It shows "Codex missing" in the warning colour, or "Scanning…" while a new machine is read.
- The Agents cell shows running agents against the limit, such as "3 of 16". The number takes the warning colour when the machine is full.
- CPU and Memory are each a short bar with a percentage. The bar turns to the warning colour at 80 percent and to the error colour at 100 percent. An offline or pairing machine shows a dash.
- Clicking a row opens a detail row under it. Each row opens and closes on its own.
- The detail row has three columns. "This machine" lists Workspace, System, the LGTM Studio version, Connected, and an editable Agent limit. "Harnesses" lists each harness with its version and how many accounts are signed in, followed by "Repositories" with each repository's folder and an "Add repository" button. "Worktrees and agents" lists each project with its worktree count and running agents, and then every running agent with its state.
- A harness that is not installed shows its install command with a copy button in the detail row.
- An offline machine shows a callout in its detail row. The callout gives the time it stopped answering and the command `lgtm connect`.
- The row menu holds Rename, Scan harnesses again, Add repository, Try to reconnect (offline machines only), and Remove machine. Remove is disabled for the master, and the menu says why.
- "Add machine" opens a dialog with three numbered steps: an install command, a connect command with a one-time code that expires in ten minutes, and a confirmation of the machine that used the code. The confirm button carries the machine's name.
- "Remove machine" asks first. The dialog lists what is lost: running agents, subscriptions on the machine, and the placement of its projects.
- On the live website the table shows only the local machine. The Role, Status and Last seen cells are empty, "Add machine" is refused, and the foot note reads "The service accepts only the local machine."

## What is planned and not built

- `docs/features.md`, "Terminal, Git, machines, and subscriptions": one computer runs the master process, and other computers pair through a connect mode and act as workers. Pairing, availability, host identity and project placement need authenticated coordination. The section says "The machine page is a preview and does not move running work."
- `docs/features.md`, "Global Home, machine choice, and screen density": Machines uses compact tables for machine status, role, agent slots, project counts, start actions, and future project placement.
- `design/DESIGN.md`, "Density and machine-aware creation": a machine row shows a project count and a "New project here" action, and project placement is a second compact table. The `Machines` function in `service.js` has neither the action nor the second table.
- `design/DESIGN.md`, "Machines and subscriptions": a machine over its limit shows "Over limit" with a warning icon, and a worker shows its master's address with a Disconnect action. The `Machines` function colours the count and has no "Over limit" words and no Disconnect action.
- `docs/features.md`, "Project settings controls awaiting engine support": changing a project's placement stays unavailable until machine pairing exists.
- `docs/issues.md`, issue 1546, "The mock's Harnesses and Machines pages tie each account to one machine": the owner stated on 2026-11-20 that accounts are signed in through the server. The Machines detail row still counts signed-in accounts under each machine. The issue is open and needs the owner's decision.
- `docs/features.md`, "A "Where is your workspace?" setting in the Service settings": the workspace folder that the detail row shows as a read-only fact becomes an editable setting on Service settings.

## How other products do it

### Tailscale: the list of machines on a private network

[Mobbin screen](https://mobbin.com/screens/bedfacb1-958b-40df-999d-308e76d8401f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page has a search field, a Filters button, an export button, and a count chip that reads "2 machines". The table has four columns: Machine, Addresses, Version, and Last seen. Each cell holds two lines: the machine name over its owner, and the version over the operating system. Last seen shows a green dot with "Connected", and a row menu of three dots sits at the end. "Add device" is the one primary button, at the top right, and it opens a menu.

Difference from LGTM Studio: Tailscale shows four columns with two lines each, and LGTM Studio shows ten columns with one line each. Tailscale shows no load figures.

### Tailscale: a machine row with tags and a filter chip

[Mobbin screen](https://mobbin.com/screens/6b2a0a2c-f7fd-48a1-a586-6649c28c29b7?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The search field holds the text "property:subnet", and a chip under it reads "Property: Subnet" with a remove button. The one matching row shows a tag chip and three small capability badges under the machine name: SSH, Subnets, and Connector. SSH means Secure Shell. A machine that is not connected shows a grey dot and the date and time it was last seen. The count chip changes to "1 machine".

Difference from LGTM Studio: Tailscale puts a machine's capabilities as badges under its name. LGTM Studio puts the harnesses in their own column and has no search or filter.

### Tailscale: one machine's own page

[Mobbin screen](https://mobbin.com/screens/bbf821a4-a169-4eec-a6ec-003238b63003?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A breadcrumb reads "All Machines / 100.70.253.121". The head shows the machine name with a green dot, a "Share…" button, and a "Machine settings" button. A facts line shows "Managed by" and "Status" with badges. Below it, each concern is a titled section with its own action button, such as "Subnets" with "Review". Inside a section, values sit in labelled columns with a small "Edit" link under each.

Difference from LGTM Studio: Tailscale gives each machine a page of its own. LGTM Studio opens the details in a row under the machine, so two machines can be compared without leaving the table.

### Twingate: a list of connectors with a connection state

[Mobbin screen](https://mobbin.com/screens/7edfad93-adb2-40bd-a455-b1e252fcadbc?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The title is the count itself, "2 Connectors". A toolbar row holds a search field, two filter menus named "Remote Network" and "Connection Status", and the count again. The columns are Connector, Remote Network, Version, and Connection Status, and the last one is sortable. A connector that never connected shows "Not yet connected" and a dash for its version. A column-chooser icon sits at the end of the header row.

Difference from LGTM Studio: Twingate sorts and filters by connection state. LGTM Studio lists machines in a fixed order and has a third state, "Pairing", that Twingate words as "Not yet connected".

### Render: services grouped under the environment that runs them

[Mobbin screen](https://mobbin.com/screens/3e106d4a-c875-45d5-be51-387a519355f8?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page stacks one block for each environment, "Production" and "Staging". Each block has small tabs with counts, a search field, and its own table. The columns are Service name, Status, Runtime, Region, and Deployed. Status is a badge such as "Available" or "Canceled deploy". Each block has its own add button and menu at the right of its heading.

Difference from LGTM Studio: Render answers "what runs where" with one table per place. LGTM Studio answers it inside each machine's detail row, so it is hidden until a row is opened.

### Vanta: a computer list with one check column per requirement

[Mobbin screen](https://mobbin.com/screens/3a576fae-5de1-4241-97bd-420225181c90?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Two tabs split the list into "Monitored" and "Unmonitored". A filter row holds a search field and four filter menus. The columns are Device, Owner, OS version, Monitoring, four narrow columns with an icon and two letters as a header, and Last check. OS means operating system. Each narrow column holds a tick, a cross, or a clock for every computer. "Export" and a bulk action sit at the top right.

Difference from LGTM Studio: Vanta shows each requirement as its own narrow column of icons, which makes a missing item easy to see down a column. LGTM Studio shows installed harnesses as tags in one cell and names a missing one in words.

### GitHub: the runners page with no self-hosted runner

[Mobbin screen](https://mobbin.com/screens/a5d8aa9f-b63c-40f8-9fe2-ccedf8ad2447?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is titled "Runners" with a "New runner" button at the top right. Two tabs split "GitHub-hosted runners" from "Self-hosted runners". The self-hosted tab is empty and shows one bordered box with a heading and one sentence that says what a self-hosted runner is. The first tab lists the hosted runners as rows with a name, badges, and a line of sizes, as a second screen shows: [Mobbin screen](https://mobbin.com/screens/0076e553-07bc-495e-9033-4194f828e2ec?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp).

Difference from LGTM Studio: GitHub separates the machines it provides from the machines the owner brings. LGTM Studio shows the master and the workers in one table with a Role column.

### Cursor: self-hosted workers inside the Cloud Agents settings

[Mobbin screen](https://mobbin.com/screens/0ea8d2f9-a3a0-4090-b7c0-eac2b5f69f83?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Self-hosted Agents" is one section of a long settings page. It has a switch named "Allow self-hosted agents", two small tabs named "Team worker pool" and "My workers", and a "Refresh" button. The empty list reads "No personal self-hosted agents" with a line that says to connect one from the owner's machine. The section has no table header until a worker exists.

Difference from LGTM Studio: Cursor treats the owner's own machines as an optional part of a settings page, behind a switch. LGTM Studio gives machines a page of their own in the sidebar.

## Options for LGTM Studio

1. Cut the table to five columns and use two-line cells, as the Tailscale list does. The machine name would sit over its address and role, and the harness tags would move under the name as badges. This costs row height, which would pass the 46 pixel limit that `design/DESIGN.md` sets for tables.
2. Give each machine its own page, as Tailscale does, and remove the expanding detail row. This gives room for the "what runs where" lists and for a second table of project placement. It costs one more route, and two machines can no longer be compared on one screen.
3. Show "what runs where" as one block per machine, as Render does with environments. Each block would hold a table of the projects and agents on that machine. This makes the running work visible without opening a row. It makes the page long, and it repeats what the project sidebar already shows.
4. Replace the Harnesses cell with one narrow icon column per harness, as Vanta does with its requirement columns. A missing harness becomes a cross in a column. This costs width as harnesses are added, and "Custom harnesses" in `docs/features.md` could add many.
5. Remove the signed-in counts from the detail row, following issue 1546 and the owner's statement that accounts are signed in through the server. The Harnesses list in the detail row would then show only what is installed and at which version. This needs the owner's decision, because it changes an existing screen.
6. On the live website, show the local machine and an empty "Workers" block, as the GitHub runners page shows an empty self-hosted tab. The block would hold the "Add machine" button. The risk is that the block advertises pairing, which the engine does not have yet.
7. Add a count in the title and a sort on Status, as Twingate does. This is cheap. It pays off only when the owner has more than a few machines.

## Ideas that are not in the feature list

- A search field with property filters shown as removable chips, for example "Harness: Codex" or "Status: Offline" (Tailscale).
- An export of the machine list to a comma-separated values (CSV) file (Tailscale).
- Tags on a machine, which could later decide what kind of work a machine takes (Tailscale).
- A column chooser at the end of the header row (Twingate).
- A "Refresh" button on the worker list that reads every machine again in one action (Cursor). LGTM Studio has "Scan harnesses again" for one machine at a time.

## Searches that were run

- "Tailscale machines list": 8 results, all from Tailscale, and 6 were usable. This was the strongest search for this screen.
- "server list table with online status and CPU and memory usage": 8 results from Tailscale, Twingate, Render, Vanta, Customer.io, Cloudflare, Vapi and Sentry. Four were usable. The search was weak on its main point: no result shows CPU or memory figures in a row.
- "GitHub self-hosted runners settings": 3 results, all from GitHub, and 2 were usable.
- "Cursor settings models list" was run for the Harnesses and settings files. It returned the Cursor screen with self-hosted workers that is cited above.
- Datadog, Railway and Sentry were not searched by name for this screen, because the budget of 14 searches was spent. Nothing is said about their host pages here.
