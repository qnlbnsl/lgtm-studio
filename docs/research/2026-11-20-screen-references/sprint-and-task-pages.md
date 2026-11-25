# Sprint, task and team pages: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `design/mock/js/screens/project.js` and `design/mock/css/screens.css`.

- The sprint page has a header and a body. The header holds a back arrow to the board, the sprint label, the sprint name, a "Fix sprint" badge when it applies, and two toggles named Plan and Tasks.
- Under the title is one line that starts "TL;DR:" and gives the sprint's goal. A finished sprint also lists its evidence, each line with a check mark.
- A row of seven facts follows: Status, Phase, Priority, Epic, Team, Depends on, and Tasks. Team and Depends on are links.
- Open requests for the sprint sit in the header as full request blocks. The owner answers them there, or opens the agent's thread in the side panel.
- The body shows the plan and the tasks side by side. The plan takes the remaining width, and the tasks take a column between 300 and 380 pixels on the right.
- The Plan and Tasks toggles show or hide each side. One side always stays. The choice is remembered across sprints.
- The plan is an HTML page drawn in a frame. Its head line shows the file name and the plan's state. The frame takes the light or dark theme from the website.
- In the side-by-side view, a task is a compact row: label, title, and under the title its state badge, a link to its part of the plan, and its owner. Points sit at the right.
- A task's plan link scrolls the plan to that section and marks it for a moment. If the plan is hidden, the link shows it first.
- In the tasks-only view, the tasks become a full table with the columns Task, Title, State, Owner, Part of the plan, and Points.
- "Add task" opens a one-line form above the tasks. Adding a task tells the project lead. A finished sprint has no "Add task" button.
- On a narrow screen the two sides stack, and the tasks come first.
- The task page is a narrow page. It holds a back link to the sprint, the title, a "Talk to <owner>" button, a line with the label, state and points, and any open request blocks. "Talk to" is the primary button unless a request waits.
- The task page then lists Owner, Team, Worktree and "Doing now" as facts, a "Changed files" list with an "Open in Git" link, and a "Work and handoff" time line with a time and an author on each step.
- The team page holds the team name, its branch, and buttons for "Team channel" and "Git". It has two tables. Agents has the columns Role, Status, Doing now, Task, Harness and model, and Context. Tasks has the columns Task, Title, State, Owner, Sprint and Points. A context value of 70% or more is shown as a warning.

## What is planned and not built

- "Plans written as HTML pages" in `docs/features.md`. Agreed and not built. Planner agents fill in one template. The mock shows sprint SP-07 with a real plan converted to HTML. The template is not written yet.
- "A sprint is proposed on the board, with one approval" in `docs/features.md`. The owner reads a proposed sprint on the board. Nothing says how a proposed sprint's own page looks before it is accepted.
- "Teams named for their stage" under "8. The build loop" in `docs/features.md`. Requested and not designed. Teams would be a Design team, a Planning team, a Build team and a Review team.
- "Automatic worker worktrees" and "Removal of finished worker checkouts" in `docs/features.md` change what the Worktree fact on a task page can show. Neither describes the task page.
- "A Define workspace with a conversation and an evolving end-result document" in `docs/features.md`. It says small projects should be able to finish Define without inventing a sprint structure. A project could then have tasks with no sprint page.
- "Sprint and task pages" in `design/DESIGN.md` says a sprint's status is one line under its title, such as "SP-02 · Build · In progress · 2 requests need you". The current code shows a row of seven labelled facts. The document and the code differ.
- "The thread panel and wide screens" in `design/DESIGN.md` says the thread panel stays open beside sprints and tasks above 1800 pixels.

## How other products do it

### Linear: a project page with its document in the centre and facts on the right

[Mobbin screen](https://mobbin.com/screens/9d720e99-d690-47c2-a7d3-1e8eb4583727?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The project has three tabs: Overview, Updates and Issues. The centre holds the project's name, a one-line summary, a row of inline properties, a Resources row for documents and links, and a long description written as a document. A column on the right holds three foldable groups. Properties lists Status, Priority, Lead, Members, Dates, Teams and Labels, one per line. Milestones lists each milestone with a percentage and a date. Progress shows Scope, Started and Completed with a chart.

Difference from LGTM Studio: Linear puts the document and the facts on one page and moves the issue list to its own tab. LGTM Studio puts the document and the task list side by side and squeezes the facts into one row.

### Linear: milestones as rows inside the project document

[Mobbin screen](https://mobbin.com/screens/40c3f3c4-f02b-4317-aff3-822bfaf84b3b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Below the description is a Milestones heading. One milestone, "Discovery & Alignment", is open. Its row shows a twist, a target date, and "0 issues · 0%". Under the row is the milestone's own text, with Goal, Timeline, Includes and Deliverables. The same milestone appears in the right column with its percentage.

Difference from LGTM Studio: Linear's section of the plan and its progress are the same row, and the row opens in place. LGTM Studio keeps the plan section in a frame and the tasks in another column, joined by a link that scrolls.

### Linear: the list of milestones, all closed

[Mobbin screen](https://mobbin.com/screens/4609e1db-a9f7-459d-9e7f-87e52dfb2418?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Five milestones are listed as single rows. Each row has an icon, a name, a twist, a date, and a count such as "0 issues · 0%". A "+ Milestone" row ends the list. A thin outline of the document's headings sits at the left edge of the text.

Difference from LGTM Studio: every row opens and closes on its own, which matches the owner's rule for collapsible things. LGTM Studio's plan frame has no outline of its sections outside the frame.

### Linear: an issue page with sub-issues and a properties column

[Mobbin screen](https://mobbin.com/screens/7c88fd2a-866e-4db1-b3ac-38cb5cd2c043?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The centre holds the issue's title, its description, a "Sub-issues 0/4" list, and an Activity section with a comment box. Each sub-issue is one row with a status circle, a title, label chips, two small number fields and the assignee's picture. The right column holds Properties (status, priority, assignee, estimate), Labels and Project, each foldable. The top right shows "2 / 5" with up and down arrows that move to the next issue in the list.

Difference from LGTM Studio: Linear lets the reader step to the next item without going back to the list. LGTM Studio's task page has only a back link to the sprint. Linear's activity ends in a comment box, and LGTM Studio's "Work and handoff" is read-only with a "Talk to" button at the top.

### Linear: an initiative's updates with a health word

[Mobbin screen](https://mobbin.com/screens/e073e872-0477-4902-84c6-17e551e76f26?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The Activity tab starts with a box that switches between Comment and Update. Below it is one update, headed "On track" with its author and date, followed by two paragraphs of prose. The right column shows Properties, a Progress chart, and four small tabs: Health, Status, Teams and Leads. The Health tab counts "Update missing 1" and "On track 1".

Difference from LGTM Studio: Linear asks a person to write a health word and a status update. LGTM Studio has a one-line goal and machine-written history, and no dated summary of how a sprint is going.

### Jira: sprints as columns, teams as rows, with capacity in points

[Mobbin screen](https://mobbin.com/screens/3bfc03fd-9085-46ca-bacc-db3a15c9c837?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The Program tab is a grid. Each column is a sprint with its number and dates. Each row is a team, with a foldable heading. Each cell states its load, such as "0 / 30 points (1 work item)", and holds cards with a title, a key, a priority mark and an assignee. A panel on the left, "Drag work items to schedule", holds unplaced items grouped by team. A button at the top right reads "Unsaved changes 4".

Difference from LGTM Studio: Jira shows every team's work across every sprint in one grid, with points against a capacity. LGTM Studio shows points per task and never sums them against a limit.

### Jira: the teams of a plan as cards

[Mobbin screen](https://mobbin.com/screens/dbdaa2ab-88cf-47a5-8f25-b649b0ceb23c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The Teams tab lists two teams as wide cards. Each card holds the team's name, a "PLAN-ONLY" tag, three labelled facts (Board, Capacity and Sprint length), a member's picture, and a link "Filter work items by this team". An "Add team" button sits at the top right.

Difference from LGTM Studio: Jira's team page is a list of teams with settings. LGTM Studio's team page is one team with a live table of its agents and what each is doing now. LGTM Studio already uses dense rows where Jira uses cards.

## Options for LGTM Studio

1. Move the sprint's facts into a right column. Status, Phase, Priority, Epic, Team, Depends on and Tasks would be one per line, in foldable groups, as on the Linear project page. The plan would take the centre. The cost is width. The lead panel is also on the right, so the plan, the tasks and the facts would be three columns beside it, and the tasks would probably need to move under the plan or into a tab.
2. Make each plan section a row that holds its tasks. A section would show its name, its task count and its percentage, and would open in place to show its text and its tasks, as Linear's milestones do. This removes the scroll-and-mark link. It needs the plan template to declare its sections as data, and the plan is a free HTML page in a frame today.
3. Add an outline of the plan's sections beside the frame. A thin list of headings with a task count on each would sit at the plan's left edge, as in the Linear document. This is a smaller step than option 2, and it keeps the frame.
4. Add previous and next to the task page. A counter such as "2 / 5" with two arrows would step through the sprint's tasks, as on the Linear issue page. The cost is small. It saves a round trip on a phone.
5. Show points against a total on the sprint. The Tasks fact would read as points done of points planned, and the team page would sum points for each agent, as Jira's cells do. LGTM Studio has no capacity number, so this shows a sum only.
6. Add a grid of teams against sprints. Rows would be teams, columns would be sprints, and each cell would show a count and a state, as in the Jira Program tab. This would sit on the board or the Agents tab, not on a sprint page. It only earns its space when a project has several teams.
7. Keep the side-by-side layout and only tighten the header. The seven facts would become the single status line that `design/DESIGN.md` already describes. This follows no reference; it closes the gap between the document and the code.

## Ideas that are not in the feature list

- A dated health word on a sprint, such as "On track", written by the team lead. Reference: Linear initiative updates.
- A Resources row on the sprint that links documents and addresses. Reference: Linear project page.
- A progress chart of scope, started and completed. Reference: Linear project page.
- A side list of tasks not yet placed in a sprint. Reference: Jira Program tab.
- A link on a team that filters the board to that team's work. Reference: Jira Teams tab.

## Searches that were run

- "Linear issue detail page with properties sidebar and activity". Eight results, all from Linear. Three were usable for the task page. Two showed an emoji picker or a comment popover.
- "Linear project overview page with description document and milestones". Eight results, all from Linear. Four were usable for the sprint page.
- "Jira sprint backlog list with issues and story points". Three results, all from Jira. None showed a backlog list. Two showed the Program grid and one showed the Teams tab. The query was weak for a sprint's task list.
- No search found a page that shows a document and its task list side by side with links between them. No search was run for a team page of agents. The budget of 14 searches was spent.
