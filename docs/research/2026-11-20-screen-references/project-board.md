# Project board: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `design/mock/js/screens/project.js` and `design/mock/css/screens.css`.

- The board has three views of the same sprints: Board, Table and Roadmap. A row of three view tabs switches between them, and each view has its own address (`/p/<project>/board`, `/board/table`, `/board/roadmap`).
- One bar sits above every view. It holds the view tabs, a text filter, an Epic menu with a count beside each epic, the project's status line, and a Focus button.
- The status line is a link to the Workflow tab. In the mock it reads the phase name and "Phase 4 of 8". On the live website it reads only the project's stage.
- The Focus button hides the sidebar and the top bar so the board fills the window. Escape leaves focus.
- The Board view has five lanes: Backlog, Planned, In progress, Review and Done. Each lane is 272 pixels wide, has a name and a count, and scrolls on its own. The lanes scroll sideways together.
- The cards in a lane are sprints, not tasks. A lane sorts its cards by priority first, and then by the number of open requests.
- A sprint card holds the sprint label (SP-04), an epic chip, a priority icon, the sprint name, a stage word, the team's area or "No team", and a task count such as "3/7".
- A card that waits for the owner shows one line with a bell, such as "Needs your approval" or "2 requests need you". The card gets a warning edge, or an error edge when a start failed.
- Hovering a card for 350 milliseconds shows the sprint's goal under it, with the label "TL;DR:". The note hides when the lanes scroll. It does not appear on touch screens.
- On a phone, a strip of lane tabs with counts sits above the lanes, and a tap scrolls to that lane.
- The Table view has seven sortable columns: Title, Epic, Priority, Team, Stage, Status and Tasks. A click on a heading sorts, and a second click reverses. A row that waits for the owner has a bell beside its name. A click on a row opens the sprint page.
- The Roadmap view has two columns. The left column holds names and stays in place. The right column is a time line in days, 96 pixels a day, and only it scrolls sideways.
- The Roadmap's first row is the project's goal. Each epic then has a row with a thin bar from its first sprint to its last, and each sprint has a row under its epic with a bar coloured by its status. A vertical line marks now, and the axis says "Today".
- Nothing on the board can be dragged. A card does not move by hand, and a roadmap bar does not resize. The feature list says moving a card is not offered as a substitute for verification or merging.
- An archived project shows one line and a Restore button in place of the board.

## What is planned and not built

- "A sprint is proposed on the board, with one approval" in `docs/features.md`. An agent puts proposed sprint cards on the board and asks one short question. Proposed cards must look different from accepted cards until the owner accepts them. "Deny" removes them.
- "A repository board with project prefixes on its tickets" in `docs/features.md`. State: Idea. The work board would move to the repository level, and each ticket would carry a prefix for its project. The ticket unit, the prefix format and the filter to one project are undecided.
- "Project settings controls awaiting engine support" in `docs/features.md`. The task label prefix needs a stored project prefix before the live website can show it.
- "A hotfix lane" under "8. The build loop" in `docs/features.md`. Requested and not designed. The owner asks the project lead for a hotfix, and a very limited team fixes and pushes. No board treatment is described.
- "Paths by the size of work." in `docs/features.md`. The lead may propose skipping planning for small work. A small project may then have no sprint structure to show on a board.
- "The Project view, decided on 2026-11-20" in `design/DESIGN.md`. Selecting a project now opens the Project view, not the board. The Board tab stays and keeps its lead panel. This is built in the mock only.
- "Board cards" in `design/DESIGN.md` says a card shows one line of goal and a progress bar once a task is done. The current card in `project.js` shows the goal only on hover and shows progress as a count. The document and the code differ.
- "Lists and tables" under "The phone" in `design/DESIGN.md` says the phone board is one list and the column jump strip is gone. The current code still renders the strip on a phone. The document and the code differ.

## How other products do it

### Linear: a project's issues as a board

[Mobbin screen](https://mobbin.com/screens/29621c0a-9946-46aa-9d39-6d5f4f578b21?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The board sits under a project's own tabs, which are Overview, Updates and Issues. Three columns show: Todo, In Progress and Done, each with a status icon, a count, a menu and a plus button. A fourth narrow column on the right is titled "Hidden columns" and lists Backlog, Canceled and Duplicate as single rows. Each card holds the issue identifier, a status circle with the title, a priority icon, label chips, a due date, the creation date, and the assignee's picture at the top right. A bar at the foot of the board says "1 issue hidden by filters" with a "Clear Filters" button.

Difference from LGTM Studio: Linear folds unused statuses into one narrow column of rows, while LGTM Studio always draws all five lanes at 272 pixels each. Linear's cards are single issues, and LGTM Studio's cards are whole sprints.

### Linear: a team's active issues as a board

[Mobbin screen](https://mobbin.com/screens/720724d3-f686-457f-8c00-fa7efa409b12?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

This board belongs to a team, not to a project. Three tabs above it, All issues, Active and Backlog, cut the same issues by state. Only the Todo and In Progress columns show. A card that belongs to a project carries a row with a document icon and the project's name. Label chips and the creation date follow on their own rows.

Difference from LGTM Studio: the project name on each card is the pattern the planned repository board needs. LGTM Studio's board today is always one project.

### GitHub Projects: a table grouped by a field

[Mobbin screen](https://mobbin.com/screens/231c5ed2-c43c-47e3-82bf-5acdb0f0af82?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The view tabs are named by the user: Backlog, By priority, By size, and New View. A filter field runs the full width under the tabs, with Discard and Save buttons for the view's changes. The table is cut into groups, here "Urgent 1" and "No Priority 2". Each group has a twist to fold it, a count, and its own "Add item" row. The columns are Title, Assignees, Status, Priority and Size, and each value is a small pill with a menu.

Difference from LGTM Studio: GitHub groups table rows under foldable headings, and LGTM Studio's table is one flat list that only sorts. GitHub lets the user save several named views, and LGTM Studio has exactly three fixed views.

### GitHub Projects: the menu that configures a view

[Mobbin screen](https://mobbin.com/screens/fe6b3b59-fe3f-4a3d-9e2e-e5a5bbaafeb9?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A menu opens from the current view's tab. Its first row is a three-part switch: Table, Board and Roadmap. Under it are Fields, Group by, Sort by, Field sum and Slice by, each showing its current value on the right. The menu ends with Generate chart, Rename view, Duplicate view, Delete view and Export view data.

Difference from LGTM Studio: GitHub treats Board, Table and Roadmap as a layout setting of a saved view. LGTM Studio treats them as three sibling tabs with no settings, and its code comment says the three views follow GitHub Projects.

### Jira: a plan's time line grouped by team

[Mobbin screen](https://mobbin.com/screens/dcd924fe-b574-4e91-99f1-b76a7e2ce4cb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The plan has tabs for Summary, Timeline, Program, Calendar, Teams, Releases and Dependencies. The Timeline's left side is a table with the columns Work item, Assignee and Team. Rows are grouped under foldable team headings, such as Mobile App, Platform and Unassigned. The right side is the time axis, with grey bars marked "Past sprint". A floating control at the bottom holds arrows, Today and a Months zoom menu. A "Group: Team" menu and a Timeline or List switch sit in the toolbar.

Difference from LGTM Studio: Jira's left side carries extra columns beside the name, and its grouping can be changed. LGTM Studio's left side holds only the name, and it always groups by epic.

### Productboard: a time line cut into status bands

[Mobbin screen](https://mobbin.com/screens/c554bbed-6b24-4375-bd41-e336f301b8aa?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The time line is cut into four foldable bands: Upcoming, In progress, Evaluating and Completed. Each item is a bar with its name inside the band of its status. An open settings panel offers "Color initiatives by", and a switch between "Exact dates" and "Weekly horizons". A zoom control with a percentage sits at the bottom right.

Difference from LGTM Studio: Productboard groups the time line by status, and LGTM Studio groups it by epic and shows status only as bar colour. The "Weekly horizons" switch lets a bar be approximate, and LGTM Studio's bars are exact to a fraction of a day.

### Wrike: a table and a Gantt chart side by side

[Mobbin screen](https://mobbin.com/screens/309ce63c-f7a7-4f54-93a0-16a72d3181c2?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The view switch offers Table, Board and Gantt Chart. The Gantt view keeps a full table on the left with the columns Name, Status, Start date, Due date and Impact. Rows nest three levels deep and fold. Each bar on the right is followed by the item's name and its owner, and a parent bar shows a percentage. Thin arrows join a bar to the bar that depends on it. A red line marks today.

Difference from LGTM Studio: Wrike draws the dependency between two bars as an arrow. LGTM Studio records what a sprint depends on, and shows it only as text on the sprint page.

### ClickUp: a time line with a side list of what is not scheduled

[Mobbin screen](https://mobbin.com/screens/9fe6fe05-2823-4250-896c-982b2ac61724?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Bars sit on a day axis with a red marker on today. A panel on the right is titled "Tasks" and has two tabs, Unscheduled and Overdue. It lists each task's name and age, with a sort menu. The toolbar has a "Group: None" menu, a Filter button and a Closed toggle.

Difference from LGTM Studio: ClickUp gives items without dates their own list beside the time line. LGTM Studio draws a sprint with no dates at day zero with a width of one day, so it looks scheduled.

## Options for LGTM Studio

1. Fold empty lanes into one narrow column. Backlog and Done would become single rows with a count when they are empty or when the owner hides them, as in the Linear project board. This saves two lanes of width beside the lead panel. The risk is that a lane the owner wants is one click away.
2. Group the Table view under foldable headings. The grouping field would be a menu with Status, Epic, Team and Stage, as in the GitHub Projects grouped table. Each group opens and closes on its own. The cost is one more control in the bar and a stored choice for each project.
3. Put the project name on each card for a repository board. The card would carry one row with the project, as the Linear team board does. This is the smallest change that serves the planned repository board. The ticket unit is still undecided, so this option depends on that decision.
4. Draw dependencies on the Roadmap. An arrow would join a sprint's bar to the bar it depends on, as in Wrike. The data already exists as `dependsOn` on the sprint. The risk is clutter when many sprints depend on one.
5. Add zoom to the Roadmap. A zoom menu with days, weeks and months would replace the fixed scale, as in the Jira time line's Months menu. The cost is small. A phone gains the most, because 96 pixels a day shows about three days on a narrow screen.
6. Let the Roadmap group by status as well as by epic. The bands would be Planned, In progress, Review and Done, as in Productboard. This contradicts option 4 in spirit, because dependency arrows read best in epic order.
7. Mark a proposed sprint on the card. A proposed card would have a dashed border and the word "Proposed", and one Accept button would sit in the bar for all proposed cards. The Roadmap already uses a dashed bar for Backlog, so the two would need different marks. No reference above shows a proposed state; this follows the feature list only.

## Ideas that are not in the feature list

- A foot bar that says how many items the filter hides, with a button that clears the filter. Reference: Linear project board.
- Named views that the owner saves, each with its own layout, grouping and sort. Reference: GitHub Projects view menu.
- A count or a sum under each group in the table. Reference: GitHub Projects "Field sum".
- A side list of sprints that have no dates yet. Reference: ClickUp.
- Start date and due date columns beside each name on the Roadmap. Reference: Wrike.
- Approximate dates shown as weekly horizons. Reference: Productboard.

## Searches that were run

- "Linear board view with issue cards in status columns". Two usable results, both from Linear.
- "project roadmap timeline with grouped rows and bars along dates". Seven usable results, from Jira, Airtable, Productboard, Asana, ClickUp, Wrike and Plane. Four are written up above. An eighth result was a Canva spreadsheet template and was not used.
- "GitHub Projects table view of issues with sortable columns". Eight results, all from GitHub. Four were usable. The rest showed dialogs for adding items.
- No search was run for Notion. The budget of 14 searches was spent on the other screens.
