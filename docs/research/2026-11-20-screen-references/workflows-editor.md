# Workflows page and its editor: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `design/mock/js/screens/workflows.js`, `design/mock/css/workflows.css`, `design/mock/js/app.js` and `docs/workflows.md`.

- "Workflows" is an entry in the foot of the sidebar, beside Machines, Harnesses, Notifications and Service settings. It opens one page at `/workflows`.
- The page has a heading, one sentence saying that projects still run the default workflow, and a Repository menu. The menu chooses between "Your workflows" and the workflows of one repository.
- The body has two columns. The left column is 180 pixels wide and lists the workflows. Each entry shows a name and one small line: "Default, read-only", "Repository workflow", or "Your workflow".
- The default workflow is named "Build loop". It has 8 phases, 99 blocks and 136 moves (counted in `design/mock/workflows/build-loop.json`). It cannot be edited. It can be cloned.
- The right column starts with a toolbar: the workflow's name, then Clone, and for a copy also Save, "Discard changes" and Delete. A state line under it reads "Default, read-only", "Validating…", "Unsaved changes", "No unsaved changes", "Saved", or the validation error.
- The picture of the workflow is a row of phase columns, each 240 pixels wide, that scrolls sideways inside a box at most 60% of the window's height. A phase's heading stays at the top while its blocks scroll.
- A block is a bordered button with its name, its role, and a count of inner moves. Under each block, its moves are listed as text lines: an arrow, the name of the target block, and the condition. A forward move uses "→" and a backward move uses "↶". Clicking a move's target selects that block and scrolls to it.
- No lines are drawn between blocks. `docs/workflows.md` says the picture uses ordinary document elements and arrow text, and no diagram or dragging library was added.
- The editor is a form under the picture, not on it. For a copy, it has a field for the workflow's name and an "Add phase" button. For the selected phase, it has a name field, "← Earlier" and "Later →" buttons, "Remove phase" and "Add block". For the selected block, it has a name field, a Role menu, the same two order buttons, and "Remove block".
- "Moves from this block" lists each move with a "Remove move" button. "Add a move" has a From menu, a To menu, a "Move when" field, and an "Add move" button. Both menus list every block of every phase.
- A block's inner moves are shown in a folded list and cannot be edited on the page. Issue 731 in `docs/issues.md` records this as open.
- A Settings section at the bottom lists the workflow's nine settings as checkboxes and number fields, such as "Full review rounds" and "Stuck statement points".
- The page validates the draft after every change. Save stays disabled while validation runs or fails. Leaving with unsaved changes asks "Discard changes?".
- The Clone dialog asks for a name and where to save: "Your workflows" or the chosen repository.
- The page carries two instructions as text: the sentence under the heading, and "Phases run from left to right. → moves forward; ↶ goes back. Select a block to see its moves." above the picture.

## What is planned and not built

- "A Workflows tab with a visual editor" in `docs/features.md`. The owner asked on 2026-11-13 that a user sees the default workflow as a picture, clones it, changes the copy in a visual editor, and can also make a workflow from nothing. The section says a separate creation action and project workflow selection remain planned.
- "A better build loop" in `docs/features.md`. The owner said the current build loop is not good enough. No replacement has been designed.
- "Paths by the size of work." in `docs/features.md`. Automatic choice of a path by the workflow is not built.
- "A hotfix lane" under "8. The build loop" in `docs/features.md`. Requested and not designed. It would be a second, shorter path through a workflow.
- Issue 723 in `docs/issues.md`: workflow editing does not select or execute a project's workflow.
- Issue 728: workflow actions write no event and no receipt. The owner must decide how workflow changes appear in the audit history.
- Issue 729: two browser tabs can overwrite a workflow without a warning. There is no revision check.
- Issue 730: renaming a workflow changes only its display name. The identifier and the file name stay.
- Issue 731: the visual editor cannot change inner steps.
- "The view carries no instructions" under "The Project view, decided on 2026-11-20" in `design/DESIGN.md`. The owner's rule is stated for the Project view, and the owner has said it applies to screens in general.

## How other products do it

### n8n: a canvas of nodes joined by lines, with branches

[Mobbin screen](https://mobbin.com/screens/817531c3-11e0-4638-bdc2-831ab9adb3a4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The top bar holds the folder and workflow name, an "Add tag" link, three centred tabs (Editor, Executions and Evaluations), an Inactive switch, and Share and Save buttons. The canvas is a dotted grid. Nodes are rounded squares with an icon and a name under each. Lines join them from left to right, and each line carries a label such as "1 item". An "If" node has two outputs labelled "true" and "false" that lead to two branches. A node that ran has a green tick. Each open end has a small plus button, and an "Execute workflow" button sits at the bottom centre.

Difference from LGTM Studio: n8n draws each move as a line and each branch as a labelled output on the node. LGTM Studio lists moves as text under the block and draws no lines.

### n8n: the version history of a workflow

[Mobbin screen](https://mobbin.com/screens/6fe83d86-ff04-4c34-949a-7e873288b51c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A panel on the right has two tabs, Versions and "Publish Timeline". It lists each saved version with a short identifier, the author and the time. A menu on a version offers "Restore version", "Publish version", "Clone to new workflow", "Open version in new tab" and Download. The canvas beside it shows the workflow with a chip that reads "Current changes".

Difference from LGTM Studio: n8n keeps every save as a version and separates saving from publishing. LGTM Studio keeps one file, and the last save wins.

### n8n: a chat that builds the workflow, with logs under the canvas

[Mobbin screen](https://mobbin.com/screens/d957076d-6797-41c7-82ee-032d055e21db?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The left third of the window is a conversation with an assistant, with a message box at the bottom. The canvas on the right shows a trigger node and three grouped steps named Ingest, Generate and Schedule. A Logs pane under the canvas lists each step that ran and shows the selected step's output as a table.

Difference from LGTM Studio: the workflow is changed by talking to an agent beside the picture. LGTM Studio's editor is a form, and LGTM Studio already has a lead conversation beside most project screens, but not beside this page.

### Zapier: steps as a vertical stack, with a panel for the selected step

[Mobbin screen](https://mobbin.com/screens/cefd9051-2a3b-426f-b2a4-719233c5e479?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Steps are cards stacked from top to bottom and joined by a line. Each card has an app icon, a number and a name, and a menu. A plus button sits on the line between two steps. The second step splits into two paths that end in pills labelled "Success" and "Error". A step that is not set up has a dashed border and placeholder text. The selected step has a coloured border, and a panel on the right holds its settings under three tabs in order: Setup, Configure and Test. The panel's one button is disabled and reads "To continue, finish required fields".

Difference from LGTM Studio: Zapier adds a step at the place it belongs, with a plus on the line. LGTM Studio adds a block at the end of a phase and then moves it with Earlier and Later. Zapier edits the selected step in a side panel beside the picture, and LGTM Studio edits it in a form below the picture.

### Zapier: a published workflow, read-only, with one Edit button

[Mobbin screen](https://mobbin.com/screens/c900eacd-d44a-41d2-89c4-a81368d6fae5?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A bar under the top bar holds an On switch, the version in use ("v2 'Email Leads v2' is in use by Sam L. just now"), a Run button and an "Edit Zap" button. The two steps are plain cards with no plus buttons and no side panel.

Difference from LGTM Studio: Zapier shows the running version read-only and makes editing a deliberate step into a draft. LGTM Studio makes the default read-only and every copy always editable, with no notion of a version in use.

### Zapier: a higher-level map where a workflow is one box

[Mobbin screen](https://mobbin.com/screens/24955176-659b-42d3-b65f-30a732a96cb3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The canvas shows a box for one workflow, joined by an arrow to a box for a table. The workflow's box has a "Hide steps" button, and its two steps hang under it as smaller boxes. A "Step details" panel on the right has fields for Title, "People or team involved", "Time saved" and Notes.

Difference from LGTM Studio: a box folds its inner steps away and shows them on demand. LGTM Studio shows all 99 blocks of the build loop at once and folds only each block's inner moves.

### GitHub: the workflows of a repository and the file behind a run

[Mobbin screen](https://mobbin.com/screens/eb4a3758-ec45-4853-8bef-c1ab784d1c79?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

GitHub Actions has no visual editor on this screen. A run's left menu has an entry "Workflow file". It shows the file's path at the commit that ran, and the file's text with line numbers. A pencil at the top right opens it for editing.

Difference from LGTM Studio: GitHub shows the definition as text and ties it to a commit. LGTM Studio shows a picture and a form, and never shows the JSON file.

## Options for LGTM Studio

1. Move the editor into a panel beside the picture. Selecting a phase or a block would open its fields in a right panel, as in Zapier, and the picture would keep its place. The form under the picture would go. The cost is width on a laptop, where the list, the picture and the panel would share one row. The list of workflows could fold to a menu.
2. Add blocks where they belong. A plus button between two blocks and at the end of each phase would replace "Add block" followed by Earlier and Later, as in Zapier and n8n. The Earlier and Later buttons would stay for keyboard use. The cost is small.
3. Draw the moves as lines. Forward moves would be lines between blocks and backward moves would be curved lines, as on the n8n canvas. With 136 moves in the default workflow, the lines would need to show only for the selected block or the selected phase. This needs a drawing layer, which `docs/workflows.md` says was deliberately not added.
4. Fold each phase to one box. The picture would open as eight phase boxes with their counts of blocks, and each phase would open on its own to show its blocks, as Zapier's map folds a workflow's steps. Opening one would not close another. This makes the default workflow readable at a glance and costs one more click to reach a block.
5. Keep versions of a workflow. Each save would be listed with its time, and a version could be restored or cloned, as in n8n's Versions panel. This answers issue 729, and it gives issue 728 a place for history. It needs the engine to store versions, which it does not do today.
6. Change a workflow by asking an agent. A conversation beside the picture would propose a change, and the owner would accept it, as in n8n's assistant. This fits how the owner already works with the project lead. No agent owns workflows today, and agents cannot change workflows by rule (`docs/workflows.md`, "The API and terminal commands").
7. Show the running version read-only, with one Edit button. When project workflow selection exists, a workflow that a project runs would open read-only with a line naming the projects that use it, as Zapier names the version in use. This depends on issue 723.

## Ideas that are not in the feature list

- A list of the runs of a workflow next to its editor, as a tab. Reference: n8n's Executions tab. In LGTM Studio this would list the projects that ran the workflow.
- A switch that turns a workflow on or off without deleting it. Reference: n8n's Inactive switch and Zapier's On switch.
- A three-part order for setting up a block (Setup, Configure, Test) with a tick on each finished part. Reference: Zapier.
- A warning mark on a block that is not finished, in the picture itself. Reference: Zapier's amber mark on a step.
- A note field on a block for the owner's own remarks. Reference: Zapier's "Notes" in Step details.
- A view of the workflow's file as text. Reference: GitHub's "Workflow file".
- Download of one version as a file. Reference: n8n's version menu.

## Searches that were run

- "n8n workflow editor canvas with connected nodes". Eight results, all from n8n. Four were usable. The rest repeated the same canvas.
- "Zapier zap editor with steps and a step settings panel". Eight results, all from Zapier. Three were usable. The rest showed other tabs of the same step panel.
- "GitHub Actions workflow run with jobs list and step logs". Eight results, all from GitHub. One was used here for the workflow file.
- No search returned a list page of workflows with a state on each row. No search was run for Retool. The budget of 14 searches was spent.
