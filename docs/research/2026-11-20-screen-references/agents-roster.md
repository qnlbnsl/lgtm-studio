# Agents roster: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `AgentsTab`, `LaneRow`, `AgentTable` and `situation` in `design/mock/js/screens/messages.js`, and from `agentStatus` and the sample agents in `design/mock/js/data.js`.

- The Agents tab lists every agent of one project, sorted by its situation. It is at `/p/<project>/agents`.
- A strip of five totals sits at the top: "Working" as a number of all agents, "Waiting on you", "May be stuck", "Tokens so far" with the number of turns, and "Subscriptions in use". Each account in the last total shows its weekly usage as a percentage and links to the accounts page.
- A bar under the totals holds a switch between "Lanes" and "Table", and a menu that filters by team. The choice of layout is remembered.
- The right end of that bar holds a sentence: "The stall watch flags a worker after 10 minutes without output, and a lead after 20." It is an explaining sentence, which the owner's rule of 2026-11-20 excludes from screens.
- The lanes are "Needs you", "Blocked", "Working", "Done" and "Idle". Each lane has a colour mark, its name and a count. An empty lane keeps its head and shows a sentence such as "Nobody is blocked."
- An agent with an open request, a failed start or a crash is in "Needs you". A stalled agent, or one that waits on another agent, is in "Blocked". A stopped agent is in "Idle".
- A lane row holds, from left to right: the avatar with a status dot, the role name and team, a work state word, the task label with its title, one line that says why the agent is in this lane, the age of its last output, a context meter, turns and tokens, and the harness with model, machine and effort.
- The work state words are Ready, Reviewing, Off, Assigned, Working, Waiting and Done.
- The reason line is amber or red when it needs attention. Examples are "Question: …", "No output for 14m" and "Waiting on 3 requests its teams sent you".
- The context meter is a short bar with a percentage. It turns amber at 70 percent. The message box turns its own meter amber at 80 percent (`design/mock/js/ui.js`), so the two thresholds differ.
- Each row ends with three controls: "Open chat", "Stop" or "Resume", and a menu. The menu holds "Nudge", the five permission modes, and the models of the agent's harness.
- "Nudge" posts a status check to the agent's team channel, addressed to that agent, and a notice says where it was posted.
- A click on a row opens the waiting request when there is one, and the agent's chat otherwise.
- The table layout has 13 columns: Role, Team, Work state, Task, Doing now, Last output, Context, Turns, Tokens, Harness and model, Machine, Permission, and the row's controls. Seven of them sort. A row that needs attention is tinted.
- On the live website the turns and tokens show a dash, because the page does not receive them yet.

## What is planned and not built

- "Roster work descriptions from existing tasks and threads" in `docs/features.md`. State: Built in unit s25-smallfix. A working agent shows its task title, or its latest tool summary or delivered message, on one bounded line.
- "5. The stall watch" in `docs/features.md`. "Stuck input boxes and approval dialogs" is an idea. An agent whose input box has held the same text for a long time would be reported to its lead.
- "5. The stall watch" in `docs/features.md`. "Replacement waits for the lead's team" is agreed. A lead that has not answered three nudges is marked as advised for replacement. The roster has no state for that.
- "5. The stall watch" in `docs/features.md`. "Usage-limit pauses are not stalls" is built in the engine. `agentStatus` in `data.js` has a "Paused" word, and the lanes have no lane or reason for a pause.
- "5. The stall watch" in `docs/features.md`. "Compact me when I am next idle" is an idea.
- "A Usage page for every harness account" in `docs/features.md`. The page breaks tokens down by project, by team and by agent, and shows what they would have cost. It is a draft in the mock. The roster's "Tokens so far" and "Subscriptions in use" overlap with it.
- "One Harnesses page in place of harness settings and Subscriptions" in `docs/features.md`. "Subscriptions" leaves the navigation. The roster's total still reads "Subscriptions in use" and links to `/subscriptions`.
- "A terminal view of each harness" in `docs/features.md`. State: Idea. A row's menu has no entry for it.
- "A one-off agent on a repository, without a project." in `docs/features.md`. How such an agent appears in the tree is undecided, and the roster is per project.
- "Agent names, set on 2026-11-10" in `design/DESIGN.md`. Agents are named by role, and the avatar is a glyph for the role. The roster follows this.

## How other products do it

### Cursor: agents as rows with a state chip and a count of changed lines

[Mobbin screen](https://mobbin.com/screens/05d779e2-ce20-4d8d-bcb7-dd6ca6887ee8?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Each agent is one row. A small tile at the left shows the number of files, the added and removed lines, and a chip that reads "Draft", "Branch" or "Merged". Beside it are the agent's title and one grey line with the model, the repository and the age. The sidebar repeats the agents under day headings, with the changed lines at the right. A menu at the foot of the sidebar groups the list by "Project", "Date", "Status" or "None". Its filter lists "Status", "Model", "Repo", "Pull request", "Source" and "Archived", and the statuses are Running, Finished, Error, Creating, Expired, Merged and Closed. The same menu has a "Compact" setting.

Difference from LGTM Studio: Cursor's row says what the agent produced, and LGTM Studio's row says what the agent is doing and what it has used. Cursor lets the reader choose the grouping, and LGTM Studio has fixed lanes.

### Devin: sessions with an outcome line and a dot for news

[Mobbin screen](https://mobbin.com/screens/3563ed9d-b08f-4d5d-b8a4-9204bd55ff83?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar lists recent sessions. Each row has the title on its first line and an outcome on its second line, in green, such as "PR is ready" with a small icon and a count. A blue dot sits at the right end of several rows. The last row has no outcome and shows "1 hour ago". The head of the list has buttons for search and for a new session.

Difference from LGTM Studio: Devin uses two lines and three facts a row, and LGTM Studio's lane row carries about ten facts. Devin marks rows that have something new, and LGTM Studio does not.

### Relevance AI: agent tasks in tabs by state, with a column for status

[Mobbin screen](https://mobbin.com/screens/7690aff3-4875-4930-9b99-52e52a7f0b07?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page has tabs with counts: "All tasks 17", "To review 4", "Escalated", "Errored" and "Completed 17", and a plus button that adds a tab. The table shows the task, the agent, a status chip such as "Pending approval" or "Inactive", and the dates it was created and updated. A filter panel lists nine fields, among them Agent, Status, Tool usage and Errors. The toolbar holds "Export", "Clear" and "Save view".

Difference from LGTM Studio: Relevance AI shows one state at a time behind a tab, and LGTM Studio stacks every lane on one page.

### Wrike: one list under status headings

[Mobbin screen](https://mobbin.com/screens/694618b6-b335-4b6e-a465-ae76de98465d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A single list is divided by small headings in capitals with a count: "PLANNED (1)", "IN REVIEW (1)", "NOT STARTED (3)" and "COMPLETED (1)". Each row is one line with a chevron, an avatar, the title, a tag, a date and the status word in colour at the right end. The toolbar has a filter, a "Status" grouping control and a menu.

Difference from LGTM Studio: Wrike's heading is one thin line and its rows are one line each. LGTM Studio's lane is a boxed section and its rows have two lines.

### Perplexity: sessions under day headings that fold

[Mobbin screen](https://mobbin.com/screens/24c7520d-dff6-478d-bec9-9a42001ae1fb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The list is divided by headings, "Scheduled" and then one per day, and each heading has a chevron at its right end. A row is a check, the session's title, chips for the files it produced with a count of more, and an age. Three tabs above the list read "Sessions", "Unread" and "Archived". The top right shows a number with an icon and a "New session" button. A prompt box stays at the foot of the page.

Difference from LGTM Studio: Perplexity's groups fold one by one and its rows show what a session produced. LGTM Studio's lanes do not fold.

### WRITER: an agents table with a filter in the status column's head

[Mobbin screen](https://mobbin.com/screens/c5f7835d-55be-403a-adce-15d96f057aba?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The table has five columns: Name, Status, Created, Last deployed and Type. The status is a dot and a word, "Deployed" or "Draft". Each column head has a small arrow, and the Status head opens a list of checkboxes that filters the rows. The type is a tag in capitals. A sort button sits at the right of the heads.

Difference from LGTM Studio: WRITER filters from the column head, and LGTM Studio's table sorts from the head and filters only by team.

### Lindy: an agents table with actions on several rows at once

[Mobbin screen](https://mobbin.com/screens/0f62c3f4-42cc-49a3-8c20-164e6eabd7f9?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The table has four columns: Name, Last Run, Integrations and Enabled. "Enabled" is a switch on each row. Two rows are checked, and a bar at the foot of the page reads "2 selected" with "Mute notifications", "Move to folder" and "Delete". A folder is the first row of the table.

Difference from LGTM Studio: Lindy acts on several agents at once and turns one off with a switch. LGTM Studio acts on one agent at a time through icons and a menu.

### Google AI Studio: use against a limit, as a bar in each row

[Mobbin screen](https://mobbin.com/screens/d579aa40-967c-4018-b503-19a29c933dd0?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

This is a page of rate limits, and it is cited for its row structure only. Each row is a model with its category and three meters. A meter is a short bar followed by the use and the limit, such as "0 / 10" or "0 / Unlimited". A menu above the table sets the time range.

Difference from LGTM Studio: Google AI Studio shows the limit beside the use, and LGTM Studio shows context as a percentage and tokens as a bare total.

## Options for LGTM Studio

1. Draw the lanes as one dense list under thin headings that fold one by one, and show an empty lane as its heading with "0". This follows Wrike and Perplexity. It removes the five empty-lane sentences and the boxed sections. It costs little.
2. Replace the "Lanes" and "Table" switch and the team menu with one "Group by" menu: situation, team, harness or none. This follows Cursor's group menu. It gives one layout to maintain. The risk is losing the 13-column table for a wide screen.
3. Cut the lane row to one line: role and team, the reason, the last output, the context, and the controls. The harness, model, machine, turns and tokens would open under the row. This follows the short rows in Devin and Wrike. It costs a click for the facts about use.
4. Show tokens and context as a bar with the use and the limit, such as "71% of 1M" and a share of the account's weekly limit. This follows Google AI Studio's meters. It needs the account limits that the Usage page draft already shows.
5. Add what the agent has produced to its row: changed lines, and the state of its branch. This follows Cursor's tile and Devin's "PR is ready". It answers "is this worth the tokens" on the roster. It needs Git facts for each agent's worktree.
6. Add checkboxes and a bar for several agents at once, with "Stop 3 agents" and "Nudge 3 agents". This follows Lindy. It helps when a limit is near. The risk is stopping a team by mistake, so it needs the confirm dialog that names what is lost.
7. Remove the stall watch sentence from the bar, and put the two thresholds into Service settings or into the "May be stuck" total as values, such as "10 min · 20 min for a lead". No reference is needed. It follows the owner's rule of 2026-11-20.

## Ideas that are not in the feature list

- Actions on several agents at once. Lindy has a selection bar.
- A saved view of the roster with its filters. Relevance AI has "Save view" and a plus button that adds a tab.
- An export of the roster table. Relevance AI has "Export".
- A filter on the column head. WRITER has it on "Status".
- A dot on an agent that has output the owner has not seen. Devin has it.
- A filter by model or by pull request. Cursor's filter menu lists both.
- A "Compact" setting for row height on the list. Cursor has it in the list's menu.

## Searches that were run

- "Devin sessions list with status". 8 results, none from Devin. 3 were usable: Relevance AI, Wrike and Perplexity. The named product did not come back, so this query worked as a category search.
- "AI agents dashboard table with status and token usage". 8 results. 3 were usable: WRITER, Lindy and Google AI Studio. No result showed tokens or context for each agent, so that column stays without a close reference.
- The Cursor and Devin screens came from the chat thread and message box searches.
- No search returned agents grouped by situation with a reason on each row, which is what the lanes do.
- Linear, Codex and Replit were not searched by name, because the budget of 14 searches was spent.
