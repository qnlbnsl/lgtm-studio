# Top bar: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this is read from `design/mock/js/app.js`, with sizes from `design/mock/css/base.css` and `design/mock/css/tokens.css`.

- The top bar is one row of 44 pixels. From the left it holds the breadcrumbs, the "Inspecting" switcher, the bell and the connection badge. It has no search field and no account menu. Both of those live in the sidebar.
- Inside a project, the breadcrumbs read repository, project, and then the team, the agent, the sprint or the task that is open. The repository is in the monospace font. An agent crumb shows the role avatar and the role name.
- Every crumb but the last is a link. The repository opens the repository page, the project opens its first tab, and the team opens the team page.
- Outside a project there is one crumb that names the page: Home, All projects, Machines, Harnesses, Usage, Notifications, Service settings or Workflows.
- "Inspecting" appears only inside a project. It is a bordered button with a branch icon, the word "Inspecting" and the branch in the monospace font, at most 300 pixels wide. It is hidden below 860 pixels of window width.
- The "Inspecting" menu is 340 pixels wide. It lists the worktrees of the repository that belong to this project or to no project. Each row shows the branch, and "Assigned" for the project's own branch or the machine's name for any other. A tick marks the current one. The menu starts with the sentence "Changes the code you look at. It does not move any agent."
- The bell shows an amber count of the requests that wait for the owner. Its menu is 360 pixels wide and is headed "Needs you, longest wait first" or "Nothing needs you".
- The bell lists at most six requests. A row holds the kind (Question, Approval, Merge or Failed start), the title, and how long it has waited. The wait turns amber after one hour. Selecting a row opens the place of that request. The last item is "View all N" or "Open Home".
- The connection badge is a coloured dot and a word: "Live", "Reconnecting" or "Offline", or "Starting" with a progress count while the service starts (`design/mock/js/live/startup.js`). The word is hidden on a phone, and the dot stays.
- The connection menu names the machine the service runs on, or says when the page last heard from the service. Its one action is "Service settings" while live and "Reconnect now" otherwise.
- The project tab strip is a second row of 38 pixels under the top bar. Its tabs are Project (mock only), Board, Workflow, Mocks (only when the project has mocks), Chat, Messages, Agents, Terminal, Files, Git and Settings. That is up to eleven tabs.
- The Chat tab shows an amber count of the project's waiting requests. When the owner looks at an agent that is not the project lead, the Chat tab's label becomes that agent's name with an eye icon. No other tab carries a count.
- The strip scrolls sideways without a scrollbar when the tabs do not fit. A "Project lead" button is pinned at its right end. It shows or hides the docked conversation with the project lead on every tab except Project and Chat.
- A sprint page and a task page keep "Board" selected. A team page selects no tab.
- On a phone a menu button at the left of the top bar opens the sidebar as a drawer.

## What is planned and not built

- `design/DESIGN.md`, "The two switchers", says: "Inspecting lists worktrees as Git does." Main checkouts come first, then each project with its teams indented, and a search field sits at the top. The menu in the code is a flat list with no search field.
- `design/DESIGN.md`, "The bell is a short copy of Home", says a bell row is 44 pixels with a second line for the kind, the agent and the project, and that failed starts come first. The code draws one line with the kind and the title.
- `design/DESIGN.md`, "The connection lives in the Live indicator", says "Reconnect now" is a small button beside the indicator and the bell's count is muted while the data is stale. The code puts "Reconnect now" inside the badge's menu.
- `design/DESIGN.md`, "The top bar" under "The phone", says the connection becomes a dot on the bell and "Inspecting" moves under the tabs. The code hides "Inspecting" on a phone and keeps the connection dot as its own badge.
- `design/DESIGN.md`, "The tab row", records the order Board, Mocks, Chat, Terminal, Git, Project settings, and Tasks for a team. The code now has up to eleven tabs in another order.
- `docs/features.md`, "A better Define phase", under "What is undecided": "Whether the Board and Chat tabs leave the project when the board moves to the repository is undecided." The same list says the live website does not show the Project tab yet.
- `docs/features.md`, "A repository board with project prefixes on its tickets". The board would be shown at the repository level, which would take the Board tab out of the strip.
- `docs/features.md`, "A database tab with one keeper of migration numbers". The entry is agreed and is not built in the engine or the mock. It would add a twelfth tab.
- `docs/features.md`, "Keyboard shortcuts for navigation". The owner wants to move between tabs and panes without the pointer. The state is Idea.
- `docs/issues.md`, issue 1544, is open: the bell does not count the open questions of a Define record.

## How other products do it

### Supabase: breadcrumb where every level is a switcher

[Mobbin screen](https://mobbin.com/screens/aaecbeeb-5370-4ae4-9bd7-30e0c3d06448?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The top bar starts with a breadcrumb of three levels: the organisation with its plan badge, the project, and the branch "main" with a "Production" badge. Each level has its own pair of small arrows that opens a list of siblings. A "Connect" button follows the breadcrumb. The right end holds "Feedback", a search field with its key hint, and a few icon buttons before the account avatar.

Difference from LGTM Studio: Supabase puts the branch inside the breadcrumb as a third level. LGTM Studio shows the branch in a separate "Inspecting" button at the other end of the bar, and its crumbs are links without a list of siblings.

### Vercel: project switcher that opens from the top of the page

[Mobbin screen](https://mobbin.com/screens/63496168-dbae-470d-9039-6883d8d1e479?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A button at the top left of the main area reads "All Projects". It opens a panel with a "Find Project…" field, the key hint "Esc", one row per project with its icon, and "Create Project" as the last row. The name of the current page, "Overview", is centred in the bar. The right end holds a single "…" button.

Difference from LGTM Studio: Vercel switches projects from the bar itself. LGTM Studio switches from the sidebar tree or from the Control K switcher, and the bar only shows where the owner is.

### Vercel: page sections in the sidebar and a short second tab strip

[Mobbin screen](https://mobbin.com/screens/93c1e7c5-c8ab-4e73-84f2-f8be4cc3da73?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Inside a project, the sidebar lists the project's sections, such as Overview, Deployments, Logs and Settings. The top bar keeps the project switcher at the left and a centred breadcrumb of three parts for the open deployment. A tab strip of five tabs sits under the bar and belongs only to this deployment: Deployment, Logs, Resources, Source and Open Graph.

Difference from LGTM Studio: Vercel moves the project's sections out of the bar and into the sidebar. LGTM Studio keeps the tree in the sidebar and puts all eleven sections in one strip.

### GitHub: branch switcher with a find-or-create field

[Mobbin screen](https://mobbin.com/screens/c7bd6794-0b21-417d-ad9b-68db089ff993?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The top bar holds a menu button, the owner and repository as a two-part breadcrumb, a search field that names its key "/", a create menu, and a few icon buttons before the avatar. The branch button sits in the page content, not in the bar. It opens a panel titled "Switch branches/tags" with a "Find or create a branch…" field, two tabs "Branches" and "Tags", the list with a tick and a "default" label, and "View all branches".

Difference from LGTM Studio: GitHub's switcher can filter and can create a branch from the same field. The "Inspecting" menu only picks from a fixed list.

### GitHub: tab strip with an icon and a count on each tab

[Mobbin screen](https://mobbin.com/screens/e6b6c56b-6011-47cf-8ae6-ae93d5a38bcb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The repository's strip has nine tabs: Code, Issues, Pull requests, Discussions, Actions, Projects, Wiki, Security and Insights. Each tab has a small icon before its word. "Issues" and "Pull requests" end in a grey count, 25 and 89. The selected tab has an underline in an accent colour.

Difference from LGTM Studio: GitHub shows counts on any tab that holds a list. LGTM Studio shows a count only on Chat, and its tabs have no icons.

### Cursor: tools as tabs of a side pane beside the conversation

[Mobbin screen](https://mobbin.com/screens/b0623233-c114-4011-ae72-66f7530d950a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The bar over the conversation holds only the agent's title and the repository's name. The conversation fills the centre. A pane at the right has its own strip of five tabs: Setup, Secrets, Git, Desktop and Terminal. Three icon buttons sit at the right end of that strip.

Difference from LGTM Studio: Cursor keeps the conversation on screen and swaps the tool beside it. LGTM Studio swaps the whole centre for the tool and docks the conversation beside it.

### Remote: inbox panel with an action menu on each item

[Mobbin screen](https://mobbin.com/screens/5e916e4a-4c86-4c06-a455-9a8d1785603d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A panel titled "Inbox" drops from the top right over the page. Each item has an icon, a bold title, two to four lines of text and a time. A "…" button on each item opens "Mark as read" and "Dismiss notification". The panel's last line says "Showing all 7 notifications".

Difference from LGTM Studio: Remote's items are news that the person clears. The bell's rows are requests that leave only when they are answered, so the bell has no clearing action.

### Replit: notifications with "Unread" and "All"

[Mobbin screen](https://mobbin.com/screens/c507d197-deb3-4ce2-a054-eddb06af3e27?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Notifications open as a dialog in the middle of the page. Two segments at the top switch between "Unread" and "All". An item is one sentence, its age, and an arrow at the right. "Load more" follows the list.

Difference from LGTM Studio: Replit keeps past items reachable from the same panel. The bell shows only what still waits, and answered requests are found on Home.

## Options for LGTM Studio

1. Make the project crumb a switcher: a small arrow beside it opens the repository's other projects with a find field and "New project". This follows Supabase and Vercel. The cost is a third way to switch projects, beside the sidebar tree and the Control K switcher.
2. Move "Inspecting" into the breadcrumb as the last level, with the branch and an "Assigned" mark. This follows Supabase. Add the search field and the grouping that `design/DESIGN.md`, "The two switchers", already records. The risk is that the owner reads the inspected branch as the branch the agents work on.
3. Give the "Inspecting" menu a find field, as GitHub's branch panel has. The cost is small. It drops the sentence at the top of the menu, which the owner's rule against instructional sentences already asks for.
4. Shorten the strip by moving Terminal, Files and Git into a tool pane with its own tabs, beside the conversation. This follows Cursor. It contradicts `design/DESIGN.md`, "Terminal and Git get the whole window", and Git's diff needs 900 pixels.
5. Move the project's sections into the sidebar while a project is open, and keep the strip for the open record only. This follows Vercel. The cost is that the tree of repositories and projects loses its place while the owner is inside a project.
6. Put counts on more tabs: unread posts on Messages, working agents on Agents, changed files on Git. This follows GitHub. The risk is that grey counts weaken the rule that a number in the strip means something waits for the owner.
7. Add an "All" segment to the bell that shows the last answered requests, as Replit does. The cost is that the bell becomes a second copy of Home's "Show answered".

## Ideas that are not in the feature list

- A field that finds a branch or creates one, inside the branch switcher ([GitHub](https://mobbin.com/screens/c7bd6794-0b21-417d-ad9b-68db089ff993?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A badge on the branch that names its role, such as "Production" ([Supabase](https://mobbin.com/screens/aaecbeeb-5370-4ae4-9bd7-30e0c3d06448?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A search field in the top bar that shows its key, so search stays visible when the sidebar is folded ([GitHub](https://mobbin.com/screens/c7bd6794-0b21-417d-ad9b-68db089ff993?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Feedback" entry in the top bar ([Supabase](https://mobbin.com/screens/aaecbeeb-5370-4ae4-9bd7-30e0c3d06448?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A star beside the project's name that marks it as a favourite ([Linear](https://mobbin.com/screens/5bc1daea-c0a9-4e20-8f0d-77163691df63?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- An icon before each tab's word ([GitHub](https://mobbin.com/screens/e6b6c56b-6011-47cf-8ae6-ae93d5a38bcb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "Vercel project header with breadcrumb switcher and branch selector". It returned 8 screens, all from Vercel. Three were usable. None shows a branch selector in the header, so the query was weak on that part.
- "GitHub repository page with tab strip and counts". It returned 8 screens, all from GitHub, and all 8 show the tab strip. One shows the branch switcher open.
- "developer tool notifications panel opened from the bell in the top bar". It returned 8 screens from WorkOS, Ditto, Amplitude, Delphi, Remote, Replit, Unity and Cloudflare. Four were usable. None comes from a tool that runs coding agents.
- The Supabase screen came from the search recorded in `sidebar.md`. The Cursor and Linear screens came from searches recorded in `sidebar.md`.
- No query named Notion or Slack, because the search budget was spent. Nothing is said about them here.
