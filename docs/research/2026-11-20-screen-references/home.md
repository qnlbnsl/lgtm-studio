# Home and All projects: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this is read from `design/mock/js/screens/home.js`.

- Home is at `/home`. It is one table of the requests that wait for the owner, across every project. Its heading is "Needs attention" with a count, which is amber when the count is above zero.
- The toolbar holds a search field, a "Type" control with All, Question, Approval, Merge and Failed start, a "Repository" menu, a "Project" menu, a "Show answered" checkbox, and a "Clear" button while any filter is set. Every choice shows a count. Each count is what the other filters leave, so the numbers add up to the table.
- The table has seven columns: Title, Type, Repository, Project, Stage, Agent and Waiting. A click on a heading sorts by that column, and a second click reverses it. The table opens sorted by Waiting, longest first.
- A row is one line. It holds a fold arrow and the title, the kind as a small label, the repository in the monospace font, the project, the stage, the agent's role avatar and name with its team's area, and the wait. A wait of one hour or more is marked.
- An open row has an amber mark at its left edge. A failed start has a red mark.
- A click on a row opens the request under it, inside the table. The owner answers there, and a link opens the agent's thread. Only one row is open at a time, because opening a second row closes the first.
- With "Show answered" on, a group row "Answered" or "Answered or cancelled" follows the open rows with its count. An answered row shows the answer and the time in place of the wait.
- An empty table says "Nothing needs you." A filtered empty table says "No open request matches." with a "Clear the filters" link.
- The Type filter is kept in the address, as `/home?type=question`. The search, the two menus and the sort are not kept when the owner leaves the page.
- While notifications are not set up, the head shows "Notifications are off" with "Set up" and "Not now".
- All projects is at `/projects`. Its head has the heading with a count and a "New project" button. Its toolbar has a search field and a "Status" control with All, Needs you and Archived, each with a count.
- The All projects table has nine columns: Project, Repository, Stage, Status, Project lead, Agents, Machine, Branch, and a last column for "Restore". Stage is a row of six dots with the stage word. Status is a word such as "Needs you · 2", "Building" or "Idle". Project lead shows what the lead is doing now. Agents reads as "2 of 5", which is working agents of all agents.
- A click on a project row opens the project. Archived projects sort last, and "Restore" is the only button in the table.
- Home does not list the open questions of a Define record (`docs/issues.md`, issue 1544).

## What is planned and not built

- `design/DESIGN.md`, "Home gathers work across projects", says: "Filters stay selected on return". The code forgets the search, the menus and the sort. The same section names agent and machine filters, and the code has only repository and project.
- `design/DESIGN.md`, "Filters", records one "Filter" button whose choices become chips, and a "Sort" with "Oldest first" and "Newest first". The code has separate menus in the toolbar and sortable headings.
- `design/DESIGN.md`, "Order and age", says failed starts come first, then the longest wait. The code sorts by wait alone.
- `design/DESIGN.md`, "Rows", says: "An approval's row shows its command." The row in the code shows the title only. The search does look inside the command.
- `design/DESIGN.md`, "Empty states", says Home's empty queue keeps a line about running work, linked to All projects. The code says only "Nothing needs you."
- `docs/features.md`, "Global Home, machine choice, and screen density", lists search combined with repository, project, agent, machine and status filters.
- `docs/features.md`, "A better Define phase", under "What is undecided": "Whether the new question form replaces the earlier cards in the Chat tab, a team's thread, the side panel and Home is undecided." The same list says: "The open questions of a Define record do not yet count in the sidebar, the bell or Home."
- `docs/features.md`, "A one-sentence reason on every approval request". An approval would carry one sentence that says what is being attempted, which would change what a Home row can show.
- `docs/features.md`, "A repository board with project prefixes on its tickets". Work across projects would be shown together at the repository level, which overlaps with what All projects is for.
- The owner's rule in the research brief says that opening one collapsible thing never closes another. Home's rows do close each other today, and `design/DESIGN.md` still records "Only one request expands at a time."

## How other products do it

### Sentry: issue table with a filter bar and saved views

[Mobbin screen](https://mobbin.com/screens/92525937-e703-4e62-bd8a-2b6bcb7ca627?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is one table of issues under a filter bar. The bar holds three menus (projects, environments and a time range), a search field that shows each condition as a removable token such as "is unresolved", a sort menu and "Save". A row is two or three lines: the error's name, its message, and a line of small labels. The columns to the right are Last Seen, Age, a trend line, Events, Users, a Priority menu and an Assignee menu. The side panel lists "Starred Views", each with its count.

Difference from LGTM Studio: Sentry lets a row be changed in the table, through the Priority and Assignee menus, and keeps a named filter as a view. Home's row must be opened before anything can be done, and its filters are not kept.

### Sentry: filter menu with a find field and checkboxes

[Mobbin screen](https://mobbin.com/screens/f46d41ec-c8f5-47e4-8338-18c0194be5f6?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The projects menu opens under its button. It has a title, a "Reset" link, a find field, and one row per project with a checkbox at the right. One project is ticked. "Cancel" and "Apply" close the menu.

Difference from LGTM Studio: Sentry's menu picks several values and has a find field. Home's Repository and Project menus pick one value each.

### Sentry: empty result with an "Undo" notice

[Mobbin screen](https://mobbin.com/screens/90011886-5504-453c-9383-2a20335a4707?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

After the last issue is resolved, the table is replaced by "No issues match your search" with a drawing and three tips. A dark notice at the bottom right names the issue that was resolved and offers "Undo". The filter bar stays as it was.

Difference from LGTM Studio: Sentry lets the person take back the last action from a notice. An answer on Home is final once it is sent.

### Lemni: inbox with the list, the conversation and the details side by side

[Mobbin screen](https://mobbin.com/screens/d06db11d-fdee-4810-84c4-94817e47dc8e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page has three columns beside the sidebar. The first is the list, headed by a "Todo" count with "Filter" and "Sort". A list item is two lines: who, and the first words. The second column is the open conversation. The third column is "Details", with Status, Priority, Assigned and Dates as label and value pairs. The sidebar shows "Inbox" and "Assigned" with their counts.

Difference from LGTM Studio: Lemni opens an item beside the list, so the list never moves. Home opens the request inside the table and pushes the later rows down.

### Linear: project table with health, lead, date and progress

[Mobbin screen](https://mobbin.com/screens/3d3a4fae-d824-440a-961b-001c6b715a4a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The Projects page is a table with six columns: Name, Health, Priority, Lead, Target date and Status. Name holds an icon, the project's name and its current milestone. Health is a word with an age, such as "On track · 6d", or "No updates". Status is a small ring with a percentage, and one row adds a small trend line. A checkbox appears at the left of the row under the pointer.

Difference from LGTM Studio: Linear's row says how the project is doing and when it is due. The All projects row says where the project runs: its machine, its branch and its agents.

### Linear: project table with a summary panel beside it

[Mobbin screen](https://mobbin.com/screens/bbbfd972-0581-4b69-a920-38cc771ec428?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A panel at the right of the same table has two tabs, "Health" and "Leads". The "Leads" tab lists each lead with the number of projects that lead has. The table narrows to make room, and its right-hand columns are cut off.

Difference from LGTM Studio: Linear adds totals beside the table. All projects shows totals only as the counts on its Status control.

### Vercel: project grid with the last commit on each card

[Mobbin screen](https://mobbin.com/screens/63496168-dbae-470d-9039-6883d8d1e479?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Projects are cards in two columns. A card holds the project's icon and name, its address, the repository as a small label, the last commit's message, and its age with the branch. A round status icon and a "…" button sit at the card's right. Two buttons above the grid switch between grid and list, and "Add New…" is the filled button.

Difference from LGTM Studio: Vercel shows what last happened in each project. All projects shows what the lead is doing now, and has no card form.

### Cursor: prompt box first, then the list of agents

[Mobbin screen](https://mobbin.com/screens/ef7c23a0-61cc-4d41-8988-7c3d84610f2f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The dashboard starts with a prompt box. The repository and the branch are chosen above the box, and the model below it. Under the box is a list of agents. Each item has a small tile with the number of files and the lines added and removed, a state label (Draft, Branch or Merged), the title, and a line with the model, the repository and the age.

Difference from LGTM Studio: Cursor's first page is for starting work and then for looking at results. Home is only for answering what waits, and starting work is a dialog.

## Options for LGTM Studio

1. Open a request beside the table, not inside it: the list on the left, the request on the right. This follows Lemni. The cost is that seven columns do not fit beside a detail pane, so the list would shrink to title, kind and wait.
2. Keep the table and let several rows stay open at once. This follows the owner's rule for collapsible things. The cost is a long page after three or four open requests, and the rule in `design/DESIGN.md` must change.
3. Replace the Type control and the two menus with one search field that holds conditions as removable tokens, and let the owner save a set as a named view in the sidebar. This follows Sentry. The cost is a small query language for an owner who usually has fewer than twenty requests.
4. Let the Repository and Project menus pick several values, with a find field. This follows Sentry's filter menu. The cost is an "Apply" step on every change.
5. Show an "Undo" notice for a few seconds after an answer. This follows Sentry. It needs the engine to hold an answer before it reaches the agent, and `docs/features.md` says that whether a sent answer can still be changed is undecided.
6. Recut the All projects row around how the project is doing: stage, what waits, last activity and its age, in place of machine and branch. This follows Linear's project table. The cost is that machine and branch move to the project's Settings tab.
7. Put a goal field at the top of Home that starts a project, with the table below it. This follows Cursor. The cost is that the table starts lower, on the one page whose job is answering.

## Ideas that are not in the feature list

- Named views of the request list, kept in the sidebar with their counts ([Sentry](https://mobbin.com/screens/92525937-e703-4e62-bd8a-2b6bcb7ca627?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A checkbox on each row and in the heading, to answer several requests at once ([Sentry](https://mobbin.com/screens/92525937-e703-4e62-bd8a-2b6bcb7ca627?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- An "Undo" notice after an action removes a row ([Sentry](https://mobbin.com/screens/90011886-5504-453c-9383-2a20335a4707?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A time range on the list, such as the last 14 days, for the answered history ([Sentry](https://mobbin.com/screens/92525937-e703-4e62-bd8a-2b6bcb7ca627?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A priority on each request ([Lemni](https://mobbin.com/screens/d06db11d-fdee-4810-84c4-94817e47dc8e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A health word with its age on a project row, such as "On track · 6d" ([Linear](https://mobbin.com/screens/3d3a4fae-d824-440a-961b-001c6b715a4a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A target date on a project ([Linear](https://mobbin.com/screens/3d3a4fae-d824-440a-961b-001c6b715a4a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A summary panel beside the project table that counts projects by one field ([Linear](https://mobbin.com/screens/bbbfd972-0581-4b69-a920-38cc771ec428?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- The last commit's message and age on a project row ([Vercel](https://mobbin.com/screens/63496168-dbae-470d-9039-6883d8d1e479?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- The number of files and lines changed on a row ([Cursor](https://mobbin.com/screens/ef7c23a0-61cc-4d41-8988-7c3d84610f2f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "Linear inbox with a list of notifications and a detail pane". It returned 8 screens, and none is from Linear. They came from Lindy, Salesforce, Lemni, Cloaked, Substack, Intercom, folk and Quo. Two were usable, Lemni and Intercom. The query was weak, and Linear's inbox was not found by it.
- "Sentry issues list with filters and sortable columns". It returned 8 screens, all from Sentry. Seven show the issue table or its filters.
- The Linear project tables came from the search recorded in `sidebar.md`. The Vercel grid came from the search recorded in `top-bar.md`. The Cursor dashboard came from the search recorded in `sidebar.md`.
- No query named GitHub, Notion or Slack for this screen, because the search budget was spent. Nothing is said about their inboxes here.
