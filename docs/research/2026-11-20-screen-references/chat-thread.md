# Chat thread: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

The Chat tab is in `design/mock/js/screens/chat.js`. The thread, the folded tool calls and the attachments are in `design/mock/js/ui.js` (`Thread`, `ToolGroup`, `LargeItem`, `Attachments`). The list of agents and the side panel are in `design/mock/js/app.js`.

- The Chat tab shows one agent's thread at full size. `/p/<project>/chat` shows the project lead, and `/p/<project>/chat/<agent>` shows another agent. For another agent, the tab's label becomes that agent's name with an eye icon.
- The Chat tab has no list of threads of its own. The owner picks an agent in the sidebar tree: repository, project, "Project lead", then each team and its agents. Each agent row has an avatar with a status dot and a state word.
- The head of the thread holds the avatar, the agent's name, its team or project, a state word and its activity. A stalled agent gets a badge, "No output for 14 min · nudged". A menu holds "Stop agent" or "Resume", "Open its terminal" and "Open its worktree in Git". The live website adds "Compact context".
- The owner's messages sit on the right with a time, and they show attachments and context tags. An agent's messages sit on the left with an avatar, a name and a time. A message from another agent shows that agent's name and team.
- Each tool call is its own folded line (`ToolGroup`) with a wrench icon, a summary and a chevron. Opened, it shows the raw command, its status, and boxes for input and output. A diff is coloured line by line. A box longer than 24 lines or 2,400 characters ends with "Show more".
- A large item is one line with its name and size, such as "Review trace (192 kilobytes)", and a "Load" button. Loaded, it shows 12 lines with "Copy" and "Open in full".
- A question or an approval inside a thread is a card that spans the thread's width. A merge and a failed start use the same block.
- While a reply is pending, the thread ends with the agent's avatar and three moving dots. A line above the message box shows the activity, such as "Working on your message", and a "Stop agent" button.
- A thread opens at its latest message. It follows new messages while the owner is at the end. Away from the end, a button reads "Latest · 2 requests below".
- Only the project lead takes messages. Another agent's thread ends with a bar "Read only" and a "Project lead" button. That agent gets a message box only while it has an open question for the owner.
- A sent image shows as a picture with a caption, such as "1.1 MB, resized from 33.4 MB". Another file shows as a name and a size.
- At 1,500 pixels or more, a column beside the thread shows "Working on" (the task and its files), "Sprints in flight" for a lead, "Runs as" (harness, account, model and effort, permission, machine, context, turns and tokens), and the other agents of the same team.
- The same thread also appears in a side panel beside every project tab except Chat and Project. The panel is 340 to 640 pixels wide, and it has a button that opens the thread in the Chat tab.
- The Project view draws the same conversation differently. It folds consecutive tool calls into one line, and it shows the agent's name only when the speaker changes (`design/mock/js/screens/mission.js`).

## What is planned and not built

- "3. Threads" in `docs/features.md`. "One thread per agent conversation" is partly built in the engine: each delivery's text, plan, tool calls and file changes are written as thread items.
- "3. Threads" in `docs/features.md`. "Threads where the owner already works" is an idea. A thread viewer would run in a Herdr pane and as a Visual Studio Code extension.
- "3. Threads" in `docs/features.md`. "Steer a thread from any client" is an idea. The owner would read, answer and interrupt a thread from the web page, a desktop app or a phone.
- "What a thread shows besides messages" in `design/DESIGN.md` describes a muted "Delivery details" line and "Message not delivered" in the error colour. The current `Thread` in `ui.js` shows neither. The document and the code differ.
- "A terminal view of each harness" in `docs/features.md`. State: Idea. The owner wants to see a harness's own terminal program as a view of an agent. An agent in Agent Client Protocol (ACP) mode has no terminal program to show.
- "What is undecided" under "A better Define phase" in `docs/features.md`. Whether the Chat tab leaves the project when the board moves to the repository is undecided. Whether the docked decisions replace the cards in the Chat tab is undecided.
- "Click an object in a mock to refer to it in the chat" in `docs/features.md`. How the reference looks in a sent message is undecided.
- "A one-off agent on a repository, without a project." in `docs/features.md`. How such an agent appears in the tree is undecided, so its thread has no place yet.
- "Keyboard shortcuts for navigation" in `docs/features.md`. State: Idea. Moving between agents and panes without the pointer is not designed.

## How other products do it

### Cursor: one agent thread with a tab for each model that ran it

[Mobbin screen](https://mobbin.com/screens/083767c5-2251-45ad-a5da-5a9ba404569d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Three tabs sit above the thread, one per model, and each reads "Task completed". The owner's prompt is a box across the top of the thread. The answer is plain prose with file names and line ranges in monospace. A turn ends with a folded line "Worked for 27s", and a later turn starts with "Thought for 3 seconds". A terminal step is one box that starts with a dollar sign and a few words about the step. The follow-up box at the foot shows the model's name and two icons.

Difference from LGTM Studio: Cursor folds a whole turn of work into one "Worked for" line, and LGTM Studio's Chat tab shows one folded line per tool call. Cursor has no avatar or name on each message.

### Cursor: the thread beside the agent's changes

[Mobbin screen](https://mobbin.com/screens/fa7df34d-7288-4c26-a0e9-95a84921e34e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar lists the agents, and each row ends with a count of changed lines, such as "+853". The thread is the middle column. It ends with "Worked for 37s" and a box "3 Files Changed" that lists each file with its added lines. A panel on the right has tabs for "Git", "Desktop" and "Terminal", and its Git tab shows the pull request and the diff. A bar above the follow-up box holds "Review" and "Start new agent".

Difference from LGTM Studio: Cursor shows the files a turn changed inside the thread and the diff beside it. LGTM Studio lists the task's files in the side column and sends the owner to the Git tab.

### Claude: a reply that starts with one folded line of reasoning

[Mobbin screen](https://mobbin.com/screens/98255484-f7b1-43c3-b9a1-1c6f971de3bd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The user's message is a grey bubble on the right with its image above it. The reply has no bubble, no avatar and no name. It starts with one grey line that summarises the reasoning and ends with a chevron. A round button with a down arrow floats over the foot of the thread. A strip above the reply box asks "Want to be notified when Claude responds?" with a "Notify" button.

Difference from LGTM Studio: Claude names nobody in a thread because there is one assistant, and LGTM Studio names each agent because several agents can speak in one thread.

### Claude: tool runs as grey lines that group several calls

[Mobbin screen](https://mobbin.com/screens/885e249d-6fe3-4461-9e87-7f743f6fbd36?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Between paragraphs of the agent's text sit grey lines with a chevron at the right. One reads "Writing ×4, Done, Listing files". Another reads "Reading, Running script, Done, Fork verifier agent". A repeated action is counted, as in "×4". The chat is a narrow column beside a work area.

Difference from LGTM Studio: Claude names the kinds of action in the folded line, and LGTM Studio's Chat tab folds each call by itself under its own summary.

### Lindy: the agent's steps as a checklist

[Mobbin screen](https://mobbin.com/screens/9f4affd5-f387-4149-860e-95c83f9bbba5?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The thread on the left shows a card "Start computer" with the line "Computer started". Under the agent's first sentence is a vertical list of steps. Each finished step has a green check, a title and a chevron, and the running step reads "Processing…". A card "Use Computer" closes the list. A panel on the right has two tabs, "Browser" and "Terminal". The message box holds a stop button.

Difference from LGTM Studio: Lindy shows the work as a list of named steps with the running one last, and LGTM Studio shows three dots and one activity line.

### Fireflies: steps as single lines between paragraphs

[Mobbin screen](https://mobbin.com/screens/3f11994c-6e9a-4ba4-a6cb-d80a88640fe5?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The reply is prose. Between its paragraphs are small lines with a check, a few words such as "Discovered 5 relevant meetings", and a chevron. The last line is grey and reads "Generating the final summary now…". The lines have no box and no icon column.

Difference from LGTM Studio: Fireflies keeps a step at the weight of a caption, and LGTM Studio's folded tool line has an icon and sits in its own message row.

### Devin: sessions listed with their outcome

[Mobbin screen](https://mobbin.com/screens/3563ed9d-b08f-4d5d-b8a4-9204bd55ff83?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar has a section "Recent" with one row per session. Each row has two lines: the session's title, and an outcome in green such as "PR is ready" with a small icon and a count. A blue dot sits at the right of rows with something new. A row without an outcome shows its age.

Difference from LGTM Studio: Devin's list says what each session produced, and LGTM Studio's sidebar row says the agent's state and nothing about its output.

### Microsoft Copilot: long work as one card with a cancel button

[Mobbin screen](https://mobbin.com/screens/c2cad8e4-77eb-4cd8-ae5e-26748339323c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

After one sentence from the assistant, a card holds the running work. It shows a heading, the task in grey, a progress bar, and one line "Reading" with the site being read. A "Cancel" button sits at the bottom right of the card. A day divider reads "Today" at the top of the thread.

Difference from LGTM Studio: Copilot puts the stop control inside the running work in the thread, and LGTM Studio puts it above or inside the message box.

## Options for LGTM Studio

1. Fold consecutive tool calls into one line in the Chat tab, as the Project view already does. This follows Claude's grouped lines and Cursor's "Worked for 27s". It shortens a builder's thread a great deal. The feature list requires that a tool call keeps its identity, status, input and output, so each call must stay reachable inside the fold.
2. Name the kinds of action in the folded line and count repeats, such as "Read ×4, ran 3 commands, 1 failed". This follows Claude. The Project view's line already reads this way. It costs a mapping from each harness's tool names to a few verbs.
3. End each turn with the files it changed, as a short list that links to the Git tab. This follows Cursor's "3 Files Changed" box. The engine already writes file changes as thread items. It costs height at the end of every building turn.
4. Replace the three dots with the agent's plan as a checklist, with the running step last. This follows Lindy. The engine already writes a plan as thread items. The risk is a harness that reports no plan, which would leave the list empty.
5. Give the Chat tab its own list of the project's agents, with one outcome line each. This follows Devin's session list and Cursor's sidebar counts. It makes the Chat tab usable without the sidebar tree. It repeats what the sidebar and the Agents tab show.
6. Drop the avatar and name from an agent's messages when the thread has one agent, and keep them when another agent speaks. This follows Claude and Cursor. The Project view already drops the name while the speaker stays the same. It costs the consistent left column that the cards align to.
7. Turn the side column into tabs for the agent's changes, its terminal and its facts. This follows Cursor's panel and Lindy's "Browser" and "Terminal" tabs. It needs the terminal view of a harness, which is an idea and depends on the run mode.

## Ideas that are not in the feature list

- Run one message on several models and read the replies in tabs. Cursor shows a tab per model above the thread.
- An outcome line for each thread in a list, such as "PR is ready". Devin has it.
- A dot on a thread that has output the owner has not seen. Devin has it, and the Unify screen cited in `decisions-and-approvals.md` has it.
- A button that asks for a notification when the running reply ends. Claude has "Notify" above its reply box.
- A progress bar on a long turn. Microsoft Copilot has one in its work card.

## Searches that were run

- "Devin session chat with agent work steps and progress". 8 results, none from Devin. 4 were usable: Lindy, Fireflies, Microsoft Copilot, and Unify for the decisions file. The named product did not come back, so this query worked as a category search.
- "Cursor agent conversation with tool calls and file changes". 8 results, all from Cursor. 4 were usable: two threads, the agent list and the prompt box.
- The Claude screens came from the Project view and decisions searches. The Devin screen came from the message box search "AI app builder prompt box with context mentions and model selector".
- ChatGPT was searched by name for the message box and returned one screen, which shows no thread. Codex, Replit, Lovable and v0 were not searched by name, because the budget of 14 searches was spent.
