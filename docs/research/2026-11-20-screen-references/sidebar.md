# Sidebar: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this is read from `design/mock/js/app.js` and `design/mock/css/base.css`.

- The sidebar is 264 pixels wide by default. A drag handle on its right edge resizes it between 220 and 420 pixels, and the browser remembers the width.
- The head holds the brand, which opens Home, and a fold button. The fold button's hover text names Control B, or Command B on Apple systems.
- The top section has two rows. "Home" shows an amber count of the requests that wait for the owner. "Find project or agent" shows the Control K hint and opens the switcher.
- A "Repositories" label follows, with three icon buttons: "All projects", the filter "Only projects needing you", and "New project".
- The tree has four levels: repository, project, team, agent. The project lead is a row directly under its project, beside the team rows. Each level is indented 14 pixels, and a thin vertical line joins the children. A row is one line of 30 pixels in Compact density and 36 pixels in Regular density (`--row` in `design/mock/css/tokens.css`).
- A single click on a row opens its page. The arrow, or a double click, folds the row. Each repository, project and team folds on its own, and opening one never closes another.
- A repository row shows a folder icon and the name in the monospace font. It shows an amber count of waiting requests only while it is folded. On hover, a plus button named "New project here" takes the count's place.
- A project row shows the name and, at the right, an amber count when something waits. When nothing waits, it shows the stage word in faint text: Define, Design, Plan, Build, Review or Hand over. A project that the lead has not named yet is shown in faint italics.
- The "Project lead" row shows the role avatar and a status word only when the status needs attention. Examples are "Needs answer", "2 requests", "Failed start" and "May be stuck".
- A team row shows a people icon, the team's name, and an amber count while folded. An agent row shows the role avatar, the role name such as "Builder 1", and a status word only when it is amber or red.
- A repository with no projects shows "No projects yet". With the filter on and nothing waiting, the tree is replaced by "Nothing needs you" and "Every project is running or idle."
- The foot navigation is six text rows: Machines (with "2 of 3 online"), Harnesses, Usage (mock only), Notifications (with a red count of failed deliveries), Workflows and Service settings.
- The owner row is last. It holds the owner's avatar, the owner's name, the "Mock controls" button (mock only) and a "Log out" icon button.
- Folded, the sidebar is a rail of 48 pixels. The rail holds icons for Home (with an amber dot), All projects, Find, New project, and then Workflows, Machines, Harnesses, Usage, Notifications and Service settings at the bottom. The tree, the owner row and "Log out" are not in the rail.
- Below 860 pixels of window width the sidebar is a drawer that slides in from the left, at most 320 pixels wide. The drawer has no rail, and following any link closes it.

## What is planned and not built

- `docs/features.md`, "A one-off agent on a repository, without a project." The owner starts one agent from a repository by choosing a harness. The entry says: "How the agent appears in the tree is undecided."
- `docs/features.md`, "A repository agent that works in the main checkout and prepares a repository for projects". One agent sits at the repository level, above the projects. The entry says how the agent appears in the navigation is undecided.
- `docs/features.md`, "A repository board with project prefixes on its tickets". The work board would move to the repository level. That changes what a repository row opens.
- `docs/features.md`, "Keyboard shortcuts for navigation". The owner wants to move between projects and agents without the pointer. The state is Idea.
- `docs/features.md`, "A Usage page for every harness account", under "The first draft in the mock". The Usage row exists in the mock only.
- `design/DESIGN.md`, "Sidebar rows", records rules that the mock code does not show. "Only the selected project has a second line. It names the project's lead." The code draws every row as one line. "Service pages are one row of 40 pixel icons." The code draws six text rows. "A project with no teams has no disclosure." The code gives every project an arrow.
- `design/DESIGN.md`, "The sidebar folds into a rail", says the rail keeps the owner's initials. The rail in the code has no owner and no "Log out".
- `design/DESIGN.md`, "The drawer", says the phone drawer starts with a "Needs attention" list of agents with an open request. The code has no such list.
- `docs/issues.md`, issue 517, is open: the sidebar does not scroll to the selected agent in a long project list.
- `docs/issues.md`, issue 1162, is open for the owner's decision: a folded team's count can differ from the status words on its agent rows.
- `docs/issues.md`, issue 1544, is open: the sidebar counts open Define questions on a project row, and Home does not, so the sidebar's counts add up to more than Home's count.

## How other products do it

### Linear: sidebar with workspace rows and a fixed set of rows under each team

[Mobbin screen](https://mobbin.com/screens/02e40d29-f036-4543-9ae4-eb93f297b588?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar starts with the workspace switcher, a search icon and a compose icon. "Inbox" and "My issues" come first. Three labelled sections follow, and each label folds its section: "Workspace" (Projects, Views, Teams, More), "Your teams", and "Try". Under "Your teams" each team is a row with an arrow, and every team has the same three children: Issues, Projects and Views. No row in this capture carries a count or a status word.

Difference from LGTM Studio: Linear's tree stops at two levels and its children are views, not records. LGTM Studio's tree lists the records themselves, down to each agent, with a status on the row.

### Linear: sidebar with a Favorites section and a third level

[Mobbin screen](https://mobbin.com/screens/f1e8745b-81b7-4a78-85e1-a325f4843f42?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A "Favorites" section sits between "Workspace" and "Your teams". It holds one document that the person marked. Two teams are listed under "Your teams". One team shows a third level: "Cycles" opens to "Upcoming". The top bar of the page is a breadcrumb of the project and the document.

Difference from LGTM Studio: LGTM Studio has no place for things the owner picked. Its only shortcut to a deep row is the "Only projects needing you" filter.

### Cursor: flat list of agents under date labels

[Mobbin screen](https://mobbin.com/screens/4b4f4f61-1c30-496c-abec-fde4d35d61bd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar has three fixed rows: "New Agent", "Automations" and "Dashboard". A label "Today" follows, and then one row per agent. A row holds a small state mark, the agent's title, and the size of its change at the right, such as "+853". The selected row is filled. The repository is not in the sidebar. It is named in the top bar of the main area, beside the agent's title. The foot holds the account with its plan, a "…" button and a filter icon.

Difference from LGTM Studio: Cursor shows no hierarchy at all, and one row is one piece of work. LGTM Studio puts three levels above the agent.

### Cursor: Group and Filter menu at the foot of the sidebar

[Mobbin screen](https://mobbin.com/screens/ef7c23a0-61cc-4d41-8988-7c3d84610f2f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The filter icon at the foot opens a menu. Its "Group" part offers Project, Date, Status and None, with a tick on the current one. Its "Filter" row opens a second menu: "My agents", "Team agents", "Collaborating on", and then Status, Model, Repo, Pull request, Source and Archived, each with its own values. The Status values in the capture are Running, Finished, Error, Creating, Expired, Merged and Closed. A "Compact" toggle is the last item.

Difference from LGTM Studio: Cursor lets the person choose how the same list is grouped. LGTM Studio has one fixed grouping, by repository, and one filter.

### Devin: session rows with a status line

[Mobbin screen](https://mobbin.com/screens/cb506f20-a571-438c-9897-b3cc43b20345?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar has five fixed rows: Sessions, Ask, Automations, Review and Wiki. A "Recent" label follows with search, add and "…" icons. Each session is a row of two lines. The first line is the title. The second line is the state in colour, such as "PR is ready" with a count, or an age such as "1 hour ago". A blue dot at the right marks some rows.

Difference from LGTM Studio: Devin spends two lines on every row to show the state in words. LGTM Studio keeps one line and shows a status word only when the row needs the owner.

### Sentry: icon rail that stays, with a second panel beside it

[Mobbin screen](https://mobbin.com/screens/92525937-e703-4e62-bd8a-2b6bcb7ca627?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A narrow rail at the far left holds one icon with a short label under it for each area: Issues, Explore, Dashboards, Insights, Prevent and Settings. Help icons and the account sit at the bottom of the rail. A second panel beside the rail belongs to the selected area. It lists Feed and four other views, then a foldable "Starred Views" group whose rows carry counts. The second panel has its own fold button, and the rail stays when it folds.

Difference from LGTM Studio: Sentry's rail is always there and the panel changes with the area. LGTM Studio's rail appears only when the whole sidebar is folded, and then the tree is gone.

### Productboard: sections with counts on the right of rows

[Mobbin screen](https://mobbin.com/screens/12795266-e92c-441b-aa04-371b6c617d6d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar starts with the workspace switcher, a fold icon and a "Find board" field. Fixed rows follow, and "Getting started" carries a blue badge with a count. Capital-letter labels divide the rest: "Favorites", "Teamspaces", "Personal" and "Data". The Favorites rows end in plain counts, such as "Notes assigned to me 7" and "Followed notes 0". Under "Teamspaces" a space opens to its boards, which are folder rows.

Difference from LGTM Studio: Productboard shows a count on a row even when the count is zero. LGTM Studio shows a count only when something waits, and hides a parent's count once the parent is open.

### Vercel: flat navigation with no projects in it

[Mobbin screen](https://mobbin.com/screens/63496168-dbae-470d-9039-6883d8d1e479?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar starts with the team switcher and a "Find…" field with the key "F". About seventeen rows follow in three groups divided by thin rules, and some rows end in an arrow that opens a sub-list. Projects are not in the sidebar. A switcher at the top of the main area lists them, with a find field and "Create Project". The foot holds an "Action Required" notice and then the owner's name with a "…" button and a bell.

Difference from LGTM Studio: Vercel keeps records out of the sidebar and uses it only for kinds of pages. LGTM Studio's sidebar is mostly records, and its page links are pushed to the foot.

## Options for LGTM Studio

1. Stop the tree at the project. Teams and agents would be reached from the project's own tabs. This follows Linear, whose tree is two levels deep. The cost is that an agent's thread is no longer one click away, and the status words on agent rows have no place in the sidebar.
2. Replace the fixed repository grouping with a "Group" choice: Repository, Stage, Status or None. This follows Cursor's Group and Filter menu. The cost is a new control to learn, and the repository page and its "New project here" button need another home when the grouping is not by repository.
3. Give each project row a second line that states its state in words, such as the stage and what waits. This follows Devin. `design/DESIGN.md` already allows a second line on the selected project. The cost is half as many projects on screen, which works against dense rows.
4. Add a "Favorites" section above "Repositories" for projects and agents the owner marks. This follows Linear. The cost is one more section, which stays empty for an owner with two or three projects.
5. Keep the rail on screen at all times and show the tree as a second panel beside it. The foot navigation moves into the rail. This follows Sentry. The cost is 48 pixels of width that the board and the lead panel lose on a laptop.
6. Fold the six foot rows into one row of icons, or into one "More" row that opens a list. `design/DESIGN.md`, "Sidebar rows", already records the one-row form, and Linear shows the "More" form. The gain is about 150 pixels of height for the tree. The cost is that "2 of 3 online" and the failed-delivery count need a new place or become dots.
7. Show a parent's count while it is open too, at the right of the row. This follows Productboard. The cost is that the same request is counted on three rows at once, and issue 1162 must be settled first so the numbers agree.

## Ideas that are not in the feature list

- A choice of how the list is grouped: by project, date, status or not at all ([Cursor](https://mobbin.com/screens/ef7c23a0-61cc-4d41-8988-7c3d84610f2f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- Filters on the list by status, model, repository and pull request, beyond "Only projects needing you" ([Cursor](https://mobbin.com/screens/ef7c23a0-61cc-4d41-8988-7c3d84610f2f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Favorites" section for records the owner marks ([Linear](https://mobbin.com/screens/f1e8745b-81b7-4a78-85e1-a325f4843f42?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- An "Archive" button that appears on a row on hover ([Cursor](https://mobbin.com/screens/e3a25999-ee07-4cef-8016-42821d1b92ec?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- The size of the change, such as "+853", as the value at the right of a row ([Cursor](https://mobbin.com/screens/4b4f4f61-1c30-496c-abec-fde4d35d61bd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A dot on a row that has news the owner has not seen yet ([Devin](https://mobbin.com/screens/cb506f20-a571-438c-9897-b3cc43b20345?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Customize sidebar" entry that lets the owner choose which fixed rows show ([Linear](https://mobbin.com/screens/37054da5-16ca-474e-88a7-7d49e339d1bc?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "Linear sidebar with teams and nested projects". It returned 8 screens, all from Linear, and all 8 show the sidebar.
- "Cursor sidebar listing agents grouped by repository". It returned 8 screens, all from Cursor. None groups agents by repository. The lists are grouped by date, and one screen shows the menu that offers grouping by project.
- "developer tool sidebar with nested collapsible tree and count badges". It returned 8 screens from Snowflake, hilos, Fibery, Productboard, Airtable, GitBook, Relume and Supabase. Five were usable. Only Productboard shows counts on rows, so the query was weak on count badges.
- The Devin, Sentry and Vercel screens above came from searches recorded in `new-project.md`, `home.md` and `top-bar.md`. The second Linear screen came from the search recorded in `command-switcher.md`.
- No query named Notion or Slack, because the search budget was spent. Nothing is said about them here.
