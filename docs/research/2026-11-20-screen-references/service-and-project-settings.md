# Service settings and Project settings: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

Everything in this section comes from `design/mock/js/screens/service.js`, in the functions `ServiceSettings`, `ProjectSettings`, `PermissionPicker`, and the section functions that `ServiceSettings` lists. The work kinds, the permission modes and the proposed assignment come from `design/mock/js/data.js`.

- Service settings has two panes. The left pane is a list of seven sections: Appearance, Defaults, Remote access, Stall watch, Workflow, Git messages, and About. A divider and a link to the Notifications page follow the list.
- The right pane shows one section at a time. The address carries the section, as in `/settings?section=defaults`. The pane's head shows the section's name, one sentence about it, and a "Saved" mark.
- Every setting is one row: a label and a hint sentence at the left, and the control at the right. Most rows save on change.
- "Appearance" holds Theme, Density, a switch that folds the sidebar, and a preview of three sample rows.
- "Defaults" holds the Permission picker and two switches: "Need-to-know messages" and "Remove agent credits when merging".
- The Permission picker is a list of five radio rows, each with a name and one sentence. The five permission modes are Read only, Ask before every change, Edit files without asking, Automatic review, and Full access.
- "Remote access" holds the Project terminal switch, the Web page switch, the port, the check-in interval, a switch named "Behind your HTTPS proxy", and the public address. HTTPS means Hypertext Transfer Protocol Secure. This section alone has "Save" and "Discard" buttons, because saving closes open web connections.
- "Stall watch" holds a master switch, eight number fields with units, and a switch named "Wake it once first", under three small headings. It ends with a switch for standing permission rules.
- "Workflow" shows the build loop with a "Default" badge, the folder of the owner's overrides with a "Check again" button, a list of repositories with their own overrides, and three settings for tests and review.
- "Git messages" holds one model select, grouped by harness, for commit messages and pull request text. "About" holds the version, the service state, the data folder, and "Restart".
- The harness, model and effort for new project leads are not on this page. They are on the Harnesses page.
- Project settings is one scrolling page with six sections: Project, Repository and machine, Agents, Assignment, Teams, and Archive or delete. Its rows put the hint behind a small help button that opens on hover, focus or a click. The button is `SettingHelp` in `design/mock/js/ui.js`.
- "Project" holds the Name with its task prefix, two lead permissions, and the Goal. Each lead permission, "Project planning" and "Starting work", is a select with three choices: Allowed, Not allowed, and Always ask for permission.
- "Repository and machine" holds the repository, the Base branch, the project branch, the folder, "Runs on", and "Lead subscription". The lead's row holds three selects side by side: the account, the model, and the effort.
- "Agents" holds the same Permission picker, which sets the mode for every agent in the project, and three switches.
- "Assignment" is a table with four columns: Work, Harness, Model, and Effort. Its four rows are Plan, Build, Review, and Test. The section head reads "Proposed by the project lead" with an "Approve" button, or "Approved by you". A row that the owner changed is marked. A line under the table, "Every row", sets one model for all four rows with "Apply".
- On the live website the Assignment table has no rows, and most controls on Project settings are refused.

## What is planned and not built

- `docs/features.md`, "A "Where is your workspace?" setting in the Service settings": the workspace folder becomes a visible setting on Service settings. Its state is "To build". What happens when a registered repository would fall outside the new folder is undecided.
- `docs/features.md`, "Project settings controls awaiting engine support": the Assignment table needs stored choices, the owner's approval, delivery of that approval to the lead, and enforcement when agents start. The project Permission picker, Write tests first, Agent review, Remove agent credits when merging, the lead subscription picker, Need-to-know, and the task prefix also wait for the engine. The live website hides them.
- `docs/features.md`, "A website control for a harness's default permission": a harness has its own default permission, and the website has no control for it. It is recorded as an idea.
- `docs/features.md`, "A choice between the bundled harness program and the one installed on the machine": a setting for each harness. The owner has not ruled on it.
- `docs/features.md`, "A Workflows tab with a visual editor": choosing a workflow for one project remains planned.
- `AGENTS.md`, "Status": the standing permission rules remain unbuilt. Stall watch already shows a switch for them.
- `design/DESIGN.md`, "Service settings": "A section list on the left follows scrolling." The code shows one section at a time and does not scroll through all of them.
- `design/DESIGN.md`, "Service settings": "Fields save on change. There are no Save buttons." Remote access has a Save button.
- `design/DESIGN.md`, "Project settings": "Rename and Archive sit at the top right", and "The preferred shell saves on change." The code has "Save name" beside the name field and Archive in the last section. It has no shell setting.
- The owner has said that a screen gets no helper or instructional sentences. This rule was given to the researcher and was not found in `docs/features.md` or `design/DESIGN.md`. Service settings shows a hint sentence on most rows.

## How other products do it

### Cursor: defaults for agents as grouped rows on one page

[Mobbin screen](https://mobbin.com/screens/db14ab9e-0507-4a45-9483-163dec7f6cdd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is one long scroll with small muted section labels: Defaults, Pull Requests, Notifications, and Security. Each section is one rounded block of rows with hairlines between them. A row holds a label and one muted line at the left, and a compact control at the right. The controls are a select such as "GPT-5.4 High", a short text field, or a switch. No row has a save button. A second screen shows the top of the same page, where a left sidebar lists the account's pages, such as Settings, Cloud Agents, Usage, and Spending: [Mobbin screen](https://mobbin.com/screens/c4b8fd05-3028-467b-ab8a-474e1e47ce5e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp).

Difference from LGTM Studio: Cursor shows every section on one page. LGTM Studio's Service settings shows one section at a time, and its Project settings scrolls as Cursor does.

### Cursor: a model menu with the effort inside the name

[Mobbin screen](https://mobbin.com/screens/76c329b0-8d07-40d4-9f74-28293e65c173?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The "Default Model" row opens one menu of eleven choices. Most choices join the model, the effort and the speed in one name, such as "Codex 5.3 High", "GPT-5.4 Extra High Fast", and "Opus 4.6 High". Models from different makers sit in the same list. The row's closed state reads "Select model".

Difference from LGTM Studio: Cursor chooses maker, model and effort with one select. LGTM Studio's Assignment table uses three selects per row: Harness, Model, and Effort.

### Cursor: several models for one task, each with a count

[Mobbin screen](https://mobbin.com/screens/5086a06b-e76b-49ad-a5e4-c2100759b758?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The model menu under the message box has a switch at its top, "Use Multiple Models". With it on, every model has a tick box. A ticked model shows a small "1x" menu at its right, and that menu offers 1x to 4x. The closed button lists the chosen models by name, separated by commas.

Difference from LGTM Studio: Cursor lets one task run on several models at once. LGTM Studio's Assignment table gives each kind of work exactly one model.

### Linear: a grouped settings list with a way back to the app

[Mobbin screen](https://mobbin.com/screens/915c51c8-1e36-47d7-a8e9-a2c33bcc98f2?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Settings take over the whole sidebar, and "Back to app" sits at its top. The list shows at least twenty-five entries under five groups: an unnamed personal group, Issues, Projects, Features, and Administration. Each entry has an icon and a name. The content is a centred column about half the width of the window, with a page title and blocks of rows.

Difference from LGTM Studio: Linear replaces the app's sidebar with the settings list. LGTM Studio keeps its sidebar and adds a second list inside the page.

### OpenAI Platform: organization settings and project settings in one list

[Mobbin screen](https://mobbin.com/screens/8cf8d3eb-515b-4ff9-b14c-dddcfb207adf?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The settings sidebar has three headed groups: Settings, Organization, and Project. The Project group repeats some names from the Organization group, such as General, API keys, People, and Limits. API means application programming interface. A project switcher in the top bar decides which project the Project group edits. The open page, "Project limits", is a narrow centred column with a budget card and two rows that each end in a button.

Difference from LGTM Studio: OpenAI Platform puts settings for everything and settings for one project in one list. LGTM Studio puts Service settings in the sidebar and Project settings in a tab of the project.

### Langdock: one model select per kind of work

[Mobbin screen](https://mobbin.com/screens/3255e6bd-ff69-477f-9762-ef14f56b35cc?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The "Models" page starts with two rows. "Default chat model" and "Image generation model" each have a title, two lines of text, and one select at the right with the model's icon and name. A toast at the top right reads "Model saved successfully" after a change. A filtered list of models and a list of keys follow lower on the page.

Difference from LGTM Studio: Langdock assigns a model to a kind of work with one row and one select. LGTM Studio uses a table, has an approval step, and marks the rows the owner changed.

### Cofounder: a model menu with marks beside each model

[Mobbin screen](https://mobbin.com/screens/37801342-be92-4580-a24e-81e6c9f35b3a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"AI Settings" is one card named "Agent Configuration". It holds two switches, a select for how strict the review bot is, set to "Blockers only", and a text box for instructions that apply to every chat and agent. The last part, "AI Model", has one row per agent with a select. The open menu lists seven models, and each shows the maker's icon, small tags such as "NEW", and one to three diamond marks.

Difference from LGTM Studio: Cofounder's menu carries a mark beside each model, so models can be compared inside the menu. LGTM Studio's model selects list names only.

### Relevance AI: a fallback model under the main one

[Mobbin screen](https://mobbin.com/screens/c2a24da7-c90a-447c-a418-52feaad57343?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Four tabs sit above the rows: General, Language Model, Task View, and Subagents. The first row, "Language model", has a select at the right. The second row, "Fallback model", has a full-width select and a sentence that says it is used when the first model fails because of its provider. A number field for output tokens has a "Reset" button and the model's limit under it.

Difference from LGTM Studio: Relevance AI stores a second choice for when the first fails. LGTM Studio's agents wait and try again when an account reaches its limit, as the Stall watch section sets.

### GitHub: a mode picker as a menu of named choices

[Mobbin screen](https://mobbin.com/screens/afad95d1-dfe9-4158-b5bb-43bc840185b2?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

This screen is about notifications, and it is cited here for its structure only. A button shows the current mode. It opens a menu of four choices, and each choice has a bold name and one or two lines under it. A tick marks the current choice.

Difference from LGTM Studio: GitHub shows the modes only while the menu is open. LGTM Studio's Permission picker shows all five modes and their sentences at all times, on two pages.

## Options for LGTM Studio

1. Show every section of Service settings on one scrolling page, as Cursor does, with the left list following the scroll. This matches what `design/DESIGN.md` records under "Service settings". It makes a long page, because Stall watch alone has more than ten rows.
2. Remove the visible hint sentences from Service settings and use the help control that Project settings already uses. This follows the owner's rule against helper sentences. Every reference product above shows a line of description under its labels, so no reference supports this option. The risk is that settings such as "Wake it once first" are hard to read without their sentence.
3. Merge Harness, Model and Effort into one select in the Assignment table, as Cursor's "Default Model" menu does. The table would shrink to two columns, Work and Model. The cost is a long menu, because every model appears once per effort.
4. Turn the Assignment table into four setting rows, one per kind of work, as Langdock does. The "Approve" button and the "Every row" line would need a new place. The record in `design/DESIGN.md` says that a list of like things is a table, and it names the Assignment section, so this option goes against that record.
5. Add marks for price beside each model in the Assignment selects, as Cofounder's menu does. The owner would see the cost of a choice while making it. The source of prices is undecided, as `docs/features.md` records for the Usage page.
6. Add a second choice to each Assignment row, as Relevance AI's "Fallback model" does. The second choice would be used when the first account reaches its limit. This needs new engine behaviour beside the wait-and-retry settings in Stall watch.
7. Replace the Permission picker's five radio rows with one select that opens a menu of named choices with a line each, as GitHub's menu does. This saves most of the picker's height on Service settings and on Project settings. The cost is that the meaning of a mode is hidden until the menu is opened.
8. Put Service settings and the current project's settings in one list, as OpenAI Platform does with its Organization and Project groups. The owner would have one place for every setting. This removes Project settings from the project's tab row, which `design/DESIGN.md` records as its place.

## Ideas that are not in the feature list

- Several models for one kind of work, each with a count, for example two reviewers on two different models (Cursor).
- A fallback model or account for a kind of work (Relevance AI).
- A mark of price or strength beside each model in a menu (Cofounder).
- A setting for how strict a review is, such as "Blockers only" (Cofounder).
- A text box in settings for instructions that every agent receives (Cofounder). Linear's settings list also has an entry named "Agent personalization".
- A default prefix for the names of branches that agents create (Cursor).
- A "Reset" button beside a number field that returns it to its default (Relevance AI). Stall watch has eight number fields and no reset.
- A list of models that one project may use (OpenAI Platform).

## Searches that were run

- "Cursor settings models list": 8 results, all from Cursor, and 5 were usable. They show the Cloud Agents settings page and the model menus. None shows a table that assigns a model to each kind of work.
- "AI agent settings with default model and approval permissions": 8 results from Intercom, Langdock, PlayAI, Gumloop, Gorgias, Cofounder, Relevance AI and ElevenLabs. Two were usable. The search was weak on permissions: no result shows a choice of permission modes for an agent.
- "AI model provider settings with API key fields and a model list" was run for the Harnesses file. It returned the Langdock and Cofounder screens that are cited above.
- "Linear notification settings", "GitHub notification settings" and "OpenAI platform usage page with token spend by project" were run for other files. They returned the Linear, GitHub and OpenAI Platform screens that are cited above.
- Named products that returned screens: Cursor, Linear, GitHub and OpenAI Platform. Vercel, Sentry, Claude and Zed were not searched by name for this screen, because the budget of 14 searches was spent.
- The permission modes and the approval of a proposed assignment have no direct reference.
