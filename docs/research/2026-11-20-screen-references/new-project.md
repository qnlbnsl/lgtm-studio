# New project and Add repository: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

New project is read from `design/mock/js/dialogs.js` and `design/mock/js/project-lead.js`. Add repository is read from `design/mock/js/screens/service.js`.

- "New project" is a dialog. Five places open it: the plus button beside "Repositories" in the sidebar, the plus button on a repository row, the plus icon in the folded rail, the button on All projects, and the button on a repository's page. The two repository places fill in that repository.
- The first field is "Goal", a text box of three lines. Enter creates the project, and Shift with Enter starts a new line.
- There is no name field. The project lead names the project in its first turn, and the sidebar shows the project in faint italics until then.
- "Run on" and "Repository" follow as two compact select rows. An offline machine cannot be chosen and reads "(offline)". A repository that is not on the chosen machine reads "(not on Studio)", with the machine's name.
- The last choice in "Repository" is "New repository…". Choosing it adds a "New repository name" field.
- "Project lead" is one row of four pick buttons: Harness, Subscription, Model and Effort. Model and Effort start on "Harness default". Changing the harness or the subscription clears the model and the effort.
- On the live website the model and effort choices are read from the chosen account, and the row says "Reading the project lead choices…" while that happens. The live website offers only the local machine.
- The foot holds "Cancel" and the filled button "Create project". While the button is disabled, a short reason sits beside it, such as "Write the goal." or "Studio is offline."
- When the repository is not on the chosen machine, a warning line names the fact and offers one button, such as "Add to Studio". The button opens Add repository, and finishing or leaving it returns to New project with the goal kept.
- The dialog shows no preview of the branch or the folder that the project will get.
- "Add repository" is a dialog with two fields: "Machine" and "Folder on Studio", with the machine's name. The folder field starts with the machine's workspace folder.
- Under the fields, three lines show what was found: "Base branch", "Remote" and "Status". In the mock they fill in about a second after the path is typed. Status then reads "A git repository with a clean working tree".
- The folder must already be a Git repository. The dialog has no folder browser, no list of found repositories, and no way to clone from an address.
- Its foot holds "Cancel" or "Back", and "Add repository". The reasons beside a disabled button include "Give the folder of a git repository." and "atlas-docs is already on Studio.", with the real names.
- Add repository opens from the Machines page, under a machine and from a machine's menu, and from the warning line in New project.

## What is planned and not built

- `docs/features.md`, "7. Repository, project, and team navigation", entry "Dedicated project worktrees", says: "The dialog always previews Branch and Folder before submission." The dialog in the code has no such preview.
- `design/DESIGN.md`, "New project", says: "The fields are Run on, Repository, Project name, Goal, and Lead subscription." The code has no project name and puts Goal first. The same section says the "Add it to…" link opens the Machines page with Add repository filled in. The code opens the dialog directly.
- `docs/features.md`, "Choosing the lead when a project is created". The state is Implemented on 2026-11-19. It is listed here because the dialog's lead row comes from it.
- `docs/features.md`, "A one-off agent on a repository, without a project." The owner starts one agent from a repository by choosing a harness, with no project, sprint or team. The worktree, the merge and the place in the tree are undecided.
- `docs/features.md`, "A repository agent that works in the main checkout and prepares a repository for projects". Its first job starts by itself when a repository is added. It fills in the base branch, the files to copy into new worktrees, the setup instructions and the test commands.
- `docs/features.md`, "Paths by the size of work." The lead may propose skipping planning when the work is small. Automatic choice of a path is not built.
- `docs/features.md`, "A "Where is your workspace?" setting in the Service settings". The workspace folder limits which folders Add repository can register. Today it can be set only in the first run.
- `docs/features.md`, "Environment variables that LGTM Studio gives to agents". The state is Idea. Variables would be stored for a repository.
- `docs/features.md`, "The next piece: installation and configuration", entry "Project configuration in `.lgtm`". The command `lgtm init` would create the folder and start a setup conversation.

## How other products do it

### Cursor: a prompt box with the repository, the branch and the model around it

[Mobbin screen](https://mobbin.com/screens/ef7c23a0-61cc-4d41-8988-7c3d84610f2f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Starting work is not a dialog. A prompt box sits at the top of the dashboard. Two small menus above the box choose the repository and the branch. A row inside the box, at the bottom, chooses the model and the connected tools, with image and voice buttons at the right. Three suggestion buttons under the box offer ready tasks.

Difference from LGTM Studio: Cursor's start is always on screen, and everything but the prompt is a small menu on its edge. LGTM Studio opens a dialog with labelled rows.

### Cursor: model menu that runs several models on one prompt

[Mobbin screen](https://mobbin.com/screens/5cfd4022-fd5e-4372-b240-2e37754159fc?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The model menu opens under the prompt box. A switch at its top reads "Use Multiple Models". Each model is a row with a checkbox, and a ticked row shows a small count such as "2x" or "1x" at its right. The button that opens the menu then lists the chosen models and their counts.

Difference from LGTM Studio: Cursor can start the same prompt on several models at once. New project chooses exactly one harness, subscription, model and effort for the lead.

### ChatGPT Codex: one field with the repository and the branch in its bottom row

[Mobbin screen](https://mobbin.com/screens/ce38ca22-4439-40a6-8777-0eb2d6318530?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is headed "What should we code next?". One field reads "Describe a task". Its bottom row holds an add button, the repository, the branch and a count menu, with voice and send at the right. Under it are three tabs: Tasks, Code reviews and Archive. An empty list offers three starter tasks as cards, each with a "Start" button.

Difference from LGTM Studio: Codex puts the list of work directly under the start field on one page. LGTM Studio separates the start, which is a dialog, from the list, which is the sidebar and All projects.

### Claude: a prompt box with "Select a repository" under it

[Mobbin screen](https://mobbin.com/screens/4ba51de5-d1f7-4102-af6a-2da18e34cc0d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The main area is headed "Let Claude handle it". The prompt box holds an add button at the left and the model menu and send button at the right. A strip joined to the bottom of the box holds a "Select a repository" button at the left and an environment menu at the right. Four cards under it offer connections, such as "Import issues" and "Monitor PRs". The sidebar lists sessions under "Today", with an "All projects" filter above them.

Difference from LGTM Studio: Claude lets the person type before choosing a repository, and the repository is one button. New project shows the repository and the machine as form rows with every choice already filled.

### Devin: the repository is chosen by a mention inside the prompt

[Mobbin screen](https://mobbin.com/screens/cb506f20-a571-438c-9897-b3cc43b20345?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The person types the task and then "@repos:". A list titled "Repositories" opens at the cursor, with one row per repository and its owner under the name. A line under the box shows "12 repositories" and the machine image. A "Get started with Devin" checklist under that shows "4 of 6" with a progress bar and can be closed.

Difference from LGTM Studio: Devin takes the repository from the text of the task. New project takes it from a select row.

### Vercel: a list of found repositories with one button on each row

[Mobbin screen](https://mobbin.com/screens/96989b3e-1d55-4eb6-948f-6f700ff9393f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"New Project" is a full page. The left half is "Import Git Repository". It has an account menu, a search field, and five rows. A row is the repository's name, the date of its last change, and an "Import" button. A field above both halves accepts a prompt or a repository's address. "Create Empty Project" is a separate row at the bottom.

Difference from LGTM Studio: Vercel shows what it found and asks for one click. Add repository asks the owner to type a folder path and then reports what it found.

### Vercel: the settings step with the uncommon parts folded

[Mobbin screen](https://mobbin.com/screens/8234b183-6909-4963-bf81-b63b743beaee?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

After "Import", one card asks for the rest. A grey line at its top repeats what is being imported: the repository and its branch. Team and project name share a row, and the name is filled in from the repository. "Application Preset" and "Root Directory" follow. "Build and Output Settings" and "Environment Variables" are two folded rows. One wide button reads "Deploy".

Difference from LGTM Studio: Vercel fills in every field and folds the settings most people leave alone. New project has no folded part, and its settings beyond the lead live in the project's Settings tab after creation.

### Cursor: a first agent that prepares the repository, with a checklist beside it

[Mobbin screen](https://mobbin.com/screens/44239877-86ce-46c8-a615-273ec82046f5?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

After a repository is connected, one agent sets up its environment. The conversation on the left shows what the agent reads and runs. A pane on the right is headed "Agent is setting things up" and lists five steps: "Get notified when finished", "Connect to Slack", "Setting up environment", "Save environment" and "Start your first agent". The first step holds a "Turn On Notifications" button. A later capture of the same pane ([Mobbin screen](https://mobbin.com/screens/4b4f4f61-1c30-496c-abec-fde4d35d61bd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)) reads "Setup is done", with ticks on four steps and a "Start new agent" button.

Difference from LGTM Studio: Cursor runs a setup agent when a repository is added and shows its progress as a checklist. LGTM Studio registers the folder and stops there, and the repository agent that would do this is an idea in `docs/features.md`.

## Options for LGTM Studio

1. Make the start a goal field that is always on screen, on Home or on the repository page, with the machine, the repository and the lead as small menus on its edge. This follows Cursor and ChatGPT Codex. The cost is that Home's table starts lower, and the dialog that opens from five places goes away.
2. Keep the dialog and move "Run on" and "Repository" into the same row of pick buttons as the lead. The dialog becomes a goal box with one row under it. This follows ChatGPT Codex's bottom row. The risk is six buttons that wrap in a dialog of 520 pixels.
3. Let the owner name the repository inside the goal with a mention. This follows Devin. The cost is a control that nothing on screen announces, and the owner has ruled out helper sentences.
4. Turn Add repository into a list of the repositories found in the workspace folder, with an "Add" button on each row and a search field, and keep the path field for anything else. This follows Vercel. The first run already shows such a list under "Found in", so the reading exists. The cost is a scan of the workspace folder each time the dialog opens.
5. Start the repository agent when a repository is added, and show its steps as a checklist beside its conversation. This follows Cursor. It depends on the undecided permission of that agent in the main checkout.
6. Add folded rows to New project for the settings that are set after creation today, such as the base branch and the permission. This follows Vercel's folded rows. The cost is a longer dialog and two places for the same setting.
7. Offer ready goals as buttons under the goal field for a repository with no projects. This follows Cursor's suggestion buttons and Codex's starter cards. The risk is goals that do not fit the repository.

## Ideas that are not in the feature list

- Starting the same goal on several models at once ([Cursor](https://mobbin.com/screens/5cfd4022-fd5e-4372-b240-2e37754159fc?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A branch menu beside the repository, to start from a branch other than the base branch ([ChatGPT Codex](https://mobbin.com/screens/ce38ca22-4439-40a6-8777-0eb2d6318530?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- Ready goals as buttons under the goal field ([Cursor](https://mobbin.com/screens/ef7c23a0-61cc-4d41-8988-7c3d84610f2f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A voice button on the goal field ([ChatGPT Codex](https://mobbin.com/screens/ce38ca22-4439-40a6-8777-0eb2d6318530?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A mention that picks a repository from inside the text ([Devin](https://mobbin.com/screens/cb506f20-a571-438c-9897-b3cc43b20345?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A list of found repositories with one button per row ([Vercel](https://mobbin.com/screens/96989b3e-1d55-4eb6-948f-6f700ff9393f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A field that accepts a repository's address and clones it ([Vercel](https://mobbin.com/screens/96989b3e-1d55-4eb6-948f-6f700ff9393f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A checklist that shows the steps of a repository's first setup ([Cursor](https://mobbin.com/screens/44239877-86ce-46c8-a615-273ec82046f5?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "Vercel new project import git repository". It returned 8 screens, all from Vercel, and all 8 show a step of creating a project.
- "coding agent new task prompt with repository and model selectors". It returned 8 screens from Cursor, ChatGPT, Claude, Devin, Gumloop, OpenAI Platform, Cofounder and Ferndesk. The first four were usable. The other four are not tools that work on a repository.
- The first and last Cursor screens came from the search recorded in `sidebar.md`.
- No query named GitHub, Linear, Notion or Slack for this screen, because the search budget was spent. Nothing is said about how they create a project or a repository.
