# Switcher and keyboard shortcuts: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

The switcher is read from `design/mock/js/dialogs.js`. The shortcuts are read from `design/mock/js/app.js`, `design/mock/js/ui.js` and the screen files named below.

- The switcher is named "Find project or agent". Control K opens it from anywhere, and Command K does on Apple systems. The sidebar row with the same name and the search icon in the folded rail also open it.
- It is a dialog 620 pixels wide, placed in the upper part of the window. It has one field with the placeholder "Search repositories, projects and agents" and an "Esc" hint. It has no title and no close button.
- Before anything is typed, it lists every repository, every project that is not archived, and each agent whose status is amber.
- It shows at most 14 results in one list without group headings. A row holds an icon or the role avatar, the name, a muted second text, a status note, and the kind at the right: Repository, Project, Agent or Page.
- The second text is the remote for a repository, the repository for a project, and the project and team for an agent. The note is the stage of a project, or its status word in amber when something waits.
- Six pages can be found: Home, All projects, Machines, Harnesses, Notifications and Service settings. Usage and Workflows are not in that list. Teams are not results of their own.
- Matching looks at names and at wider text: a project's goal and sprints, a repository's description, an agent's role, team and activity. A word in the name counts three times a word in the text. A word such as "agent" or "project" favours that kind. A comment in the file says the real product ranks with a small embedding model, which the mock imitates.
- Up and Down move the selection, Enter opens it, and Escape or a click outside closes the dialog. The pointer also moves the selection. A foot line shows the keys for "move" and "open".
- The switcher only goes to places. It runs no action, it lists no open request, and it keeps no list of recent places.
- Control B, or Command B, folds and opens the sidebar (`app.js`). The fold button's hover text names the keys.
- Enter sends in the goal field, the chat message box and the Messages box, and Shift with Enter starts a new line. Control or Command with Enter commits in Git, submits the message field in the Files tab, and sends a decision's own-answer field in the Project view.
- Escape closes a dialog, a menu and the board's focus mode. Arrow keys move inside a menu. On a resize handle, the arrow keys resize and Home restores the default width.
- No screen lists the shortcuts. The only hints are the "Ctrl K" mark on the sidebar row, the fold button's hover text and the switcher's foot line.
- The terminal's key handler does not take Control K or Control B (`design/mock/js/screens/terminal.js`). The page's handler in `app.js` does not check where the focus is.

## What is planned and not built

- `docs/features.md`, "Keyboard shortcuts for navigation". The state is Idea. The entry says: "The owner wants keyboard shortcuts for moving around the website, in the manner of Vim and Visual Studio Code." Its examples are moving between projects, tabs, agents and panes, and opening the quick switcher. It also says: "The shortcut sets, whether they can be chosen or remapped, and how they coexist with typing in the chat composer and the terminal are not decided."
- `design/DESIGN.md`, "The two switchers", says: "The quick switcher matches names only." It also says rows are 36 pixels and show the attention count. The code matches wider text and shows a status word.
- `design/DESIGN.md`, "Scope and workspace controls", says the quick switch searches projects, teams and agents by name. Teams are not results in the code.
- `design/DESIGN.md`, "The sidebar folds into a rail", says: "The terminal keeps Control + B." It says the shortcut does nothing while the terminal has focus. The code read here does not show that guard.
- `docs/features.md`, "Simple editing in the Files tab", names Control S as the shortcut for the "Save" button.

## How other products do it

### Linear: search results in one list with the kind in the first column

[Mobbin screen](https://mobbin.com/screens/a2f96ae5-202c-4747-a959-d33d7e5c0f72?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The search fills the main area, not a dialog. The field is at the top with a close button at the right. Four tabs under it narrow the list: All, Issues, Projects and Documents. Each result is one line. The first column says the kind, or the issue's identifier. Then come a state icon, the title with the matched word in bold, and the age at the right.

Difference from LGTM Studio: Linear puts the kind first and offers tabs for the kinds. The switcher puts the kind last and has no way to narrow to one kind except typing the kind's word.

### Linear: a setup step that has the person press the keys

[Mobbin screen](https://mobbin.com/screens/1e783aca-8b8e-4ae5-994d-284a8ddae491?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

One step of the setup is titled "Meet the command menu". A line under it says the menu completes any action. A box shows the two keys as large key caps under "Try opening the command menu with:". "Continue" is under the box, and seven dots mark the step.

Difference from LGTM Studio: Linear teaches the shortcut once, during setup. LGTM Studio shows it only as a small mark on a sidebar row.

### Linear: key hints at the right of menu items

[Mobbin screen](https://mobbin.com/screens/f1e8745b-81b7-4a78-85e1-a325f4843f42?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A document's "…" menu lists Move to, Duplicate, Rename, Favorite, Copy, Remind me, Version history and Delete. Several items show their keys at the right edge in a muted colour. "Move to" opens a second list with a "Filter…" field and two groups, "Projects" and "Initiatives".

Difference from LGTM Studio: Linear shows a shortcut on the menu item that it replaces. LGTM Studio's menus show no keys.

### Linear: number keys that pick an option in an open list

[Mobbin screen](https://mobbin.com/screens/5bc1daea-c0a9-4e20-8f0d-77163691df63?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The status control on a project opens a short list. A "Change status…" field heads it, with a key hint at its right. The five options are Backlog, Planned, In Progress, Completed and Canceled. Each option shows a digit from 1 to 5 at its right, and the current one has a tick.

Difference from LGTM Studio: Linear lets a digit pick an option. LGTM Studio's answer rows and menus are picked with the pointer, or with the arrow keys and Enter.

### Vercel: a find field in the sidebar that opens with one letter

[Mobbin screen](https://mobbin.com/screens/63496168-dbae-470d-9039-6883d8d1e479?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar's second row is a "Find…" field with the letter "F" as its key hint. The project switcher at the top of the page is open. It has its own "Find Project…" field with an "Esc" hint, a list of projects, and "Create Project" as the last row.

Difference from LGTM Studio: Vercel's switcher ends in a create action, and a single letter focuses the find field. The switcher has no action, and it opens only with a two-key shortcut.

### Obvious: shortcut list with a search field, in groups by area

[Mobbin screen](https://mobbin.com/screens/d1f418bb-9d1a-40b6-a7b5-7b294ef64703?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A dialog titled "Keyboard Shortcuts" has a "Search shortcuts…" field and four columns. The groups are Navigation, Actions, Threads, Chat input, Agent mode, Layout, Sidebar panels and System. A line is the action's name and its keys as key caps. Navigation holds "Open search", "Navigate to projects" and "Switch recent project". Threads holds "Previous thread" and "Next thread" on the bracket keys. System holds the shortcut that opens this dialog.

Difference from LGTM Studio: Obvious gives every area a shortcut and one place that lists them all. LGTM Studio has two global shortcuts and no list.

### Gorgias: two-key sequences for going to a place

[Mobbin screen](https://mobbin.com/screens/f3bcf37a-7ed0-45e8-b6bb-2f7649d4044a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The "Keyboard shortcuts" dialog has two columns of groups, such as "Global navigation", "Spotlight search", "Views (Global)" and "Ticket page". The keys come first on each line, as small dark labels, and the action follows. The first two lines pair the letter G with a second letter for "Go to the home view" and "Go to the current view". Arrow keys move through pages and items.

Difference from LGTM Studio: Gorgias uses plain letters in sequence, with no modifier key. LGTM Studio uses only modifier shortcuts, which is safe while the owner types.

### Zendesk: shortcut list with a switch that turns shortcuts off

[Mobbin screen](https://mobbin.com/screens/f679b815-490f-4349-a70b-36c23bb89a18?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The dialog groups its lines by when they apply: "Used anywhere", "Used in lists", "Used when viewing a ticket" and "Used when composing a comment". The foot has two checkboxes, "Enable keyboard shortcuts" and one for shortcuts in the composer. "Close" and "Download as a PDF" are the buttons.

Difference from LGTM Studio: Zendesk states where each shortcut works and lets the person turn them off, with a separate choice for the composer. LGTM Studio has not decided how shortcuts behave in the message box and the terminal.

## Options for LGTM Studio

1. Add actions to the switcher as a "Commands" group: "New project", "Fold sidebar", "Open Git", "Log out". This follows Linear, whose setup step says its menu completes any action. The cost is that ranking must mix places and actions, and an action that changes something needs a confirm step.
2. Add tabs under the field that narrow the list: All, Projects, Agents and Pages. This follows Linear's search. The cost is one more row in a small dialog.
3. List open requests in the switcher, so that typing "merge" finds the merge that waits. This follows Linear's one list of several kinds. The cost is overlap with the bell and with Home's search.
4. Add a shortcuts dialog that a key opens, grouped by area, with a search field. This follows Obvious. It is a reference that the owner opens on demand, so it adds no sentence to any screen. The cost is one more thing to keep current as shortcuts are added.
5. Add two-key "go to" sequences for the project tabs, such as G then B for Board. This follows Gorgias. The risk is a clash with typing, so the sequences must be off while a field or the terminal has focus.
6. Show a shortcut at the right of the menu item and in the hover text of the button it replaces. This follows Linear. The cost is small once the shortcut set exists.
7. Let digits pick an answer in an open decision and an option in an open menu. This follows Linear's status list. The risk is a wrong answer from a stray key, so "Send answer" must stay a separate step.

## Ideas that are not in the feature list

- A switch that turns keyboard shortcuts off, with a separate switch for the message box ([Zendesk](https://mobbin.com/screens/f679b815-490f-4349-a70b-36c23bb89a18?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A search field inside the list of shortcuts ([Obvious](https://mobbin.com/screens/d1f418bb-9d1a-40b6-a7b5-7b294ef64703?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A shortcut that switches to the most recent project ([Obvious](https://mobbin.com/screens/d1f418bb-9d1a-40b6-a7b5-7b294ef64703?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A step in the first run that has the owner press the switcher's keys ([Linear](https://mobbin.com/screens/1e783aca-8b8e-4ae5-994d-284a8ddae491?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- Digits that pick an option in an open list ([Linear](https://mobbin.com/screens/5bc1daea-c0a9-4e20-8f0d-77163691df63?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A create action as the last row of the switcher ([Vercel](https://mobbin.com/screens/63496168-dbae-470d-9039-6883d8d1e479?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A filter field inside a long submenu, with group headings ([Linear](https://mobbin.com/screens/f1e8745b-81b7-4a78-85e1-a325f4843f42?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A printable copy of the shortcut list ([Zendesk](https://mobbin.com/screens/f679b815-490f-4349-a70b-36c23bb89a18?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "Linear command menu with search results grouped by type". It returned 6 screens, all from Linear. Four were usable. None shows the command menu itself open. The closest are the search page and the setup step about the menu, so the query was weak for the menu itself.
- "keyboard shortcuts reference dialog listing keys by section". It returned 8 screens from Relume, Frame.io, Obvious, Luma, Gorgias, Webflow, Asana and Zendesk. All 8 show a shortcut list.
- The Vercel screen came from the search recorded in `top-bar.md`. The Linear status list came from the search recorded in `sidebar.md`.
- No query named GitHub, Cursor, Notion or Slack for this screen, because the search budget was spent. Nothing is said about their switchers here.
