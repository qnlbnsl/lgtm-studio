# Terminal tab: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

The source is `design/mock/js/screens/terminal.js`, with the launch control in `design/mock/js/terminal-launch.js`.

- The tab holds a row of session tabs for one project. A session is one shell in one worktree on one machine.
- A session tab shows a status dot, a label and a close button. The label is the machine and the folder, such as "Studio · agent-console-shell", until the owner renames it.
- A session that watches an agent shows an eye icon in place of the dot.
- A double click on a session tab opens a "Rename session" dialog.
- An "Open in" select beside the tabs lists each worktree with its machine and folder. An offline machine is listed and disabled, with the reason in the option. A "New session" button opens a shell in the selected worktree.
- The right side of the row holds the text size control, from 11 to 18 pixels, and a session menu. The menu has "Rename session", "Copy all output", "Clear scrollback" and "Close session".
- A watched session shows a bar above the grid. The bar names the agent and the command, and it says that the owner's keys are not sent. It holds a "Take over" button.
- "Take over" asks first while the command runs. The dialog lists what is lost: the command stops, and the agent is told that the owner took the session.
- Three more bars cover connection trouble. A dropped connection has "Reconnect". A reconnecting session shows a spinner. An offline machine has "Open Machines".
- The screen is a character grid with no input box. A hidden text area takes the keys. A full-screen editor takes over the whole grid when the owner runs `vim`.
- The output sticks to the last line until the owner scrolls up. A session keeps at most 1,500 lines.
- On a phone, a row of extra keys sits under the grid: Escape, Tab, Control, the four arrows, and five symbols. Control stays pressed for the next key.
- A status line at the bottom shows a state word, the current folder, the branch, the shell name, and the grid size in columns and rows. The state words are "Ready", "Running", "Watching", "vim", "Disconnected", "Reconnecting" and "Machine offline".
- Closing a session that runs a command, or that has unwritten edits, asks first.
- Sessions are tabs only. The file has no split panes.
- With no session open, the tab shows "No sessions are open", one sentence that explains a session, and the launch control.
- The mock's shell is a pretend one with a small set of commands. `design/mock/README.md`, under "The connected project terminal", says that the live website connects the same screen to owner-only shells in registered local worktrees.

## What is planned and not built

- From `docs/features.md`, "Terminal, Git, machines, and subscriptions": "Taking over an agent terminal remains planned." The mock shows the take-over bar, and the engine does not support it.
- From the same section: "The main conversation can switch to a terminal." The section says that the intended product gives direct commands on the selected machine and workspace. The connected website gives owner shells in local worktrees only.
- From the same section: machine coordination remains planned. A shell on another machine depends on it.
- From `design/mock/README.md`, "The connected project terminal": "Remote shells, agent terminal watching, and agent terminal takeover need engine support."
- From `docs/features.md`, "A terminal view of each harness": an idea, requested on 2026-11-17. The owner wants to see a harness's own terminal program as a view of an agent. An agent in Agent Client Protocol (ACP) mode has no terminal program to show.
- From `docs/features.md`, "Keyboard shortcuts for navigation": an idea. How shortcuts coexist with typing in the terminal is not decided.
- From `design/DESIGN.md`, "Terminal sessions": "A double click renames a session, and so does F2. Enter keeps the name, and Escape keeps the old one." The file read today opens a dialog on a double click and has no F2 handler.
- From `design/DESIGN.md`, "Conflicts": "Open in Terminal is the way out for hard cases." It should open the worktree's terminal with `vim` and the conflicted file already typed. No such button was found in `design/mock/js/screens/git.js`.
- From `docs/features.md`, "An IDE button that starts a code server on demand": an idea, not agreed. It would open a code server for the inspected worktree in its own browser tab.

## How other products do it

### Amazon Web Services (AWS): CloudShell, a browser terminal with session tabs

[Mobbin screen](https://mobbin.com/screens/a2f7a934-d1bb-40c9-8098-e70620142d18?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The terminal fills the page under a thin header that reads "CloudShell". Two session tabs sit above the grid, and both are labelled with a region name, "us-east-1". Each tab has a close cross, and a plus after the last tab opens another. An "Actions" menu button and a gear sit at the right of the header. The grid shows only a prompt.

Difference from LGTM Studio: the tabs carry no state and two tabs can have the same label. LGTM Studio shows a dot or an eye on each tab and adds a status line under the grid.

### Devin: the agent's shells, with a command history under the output

[Mobbin screen](https://mobbin.com/screens/f7e9f607-dc8f-49ab-b42e-26f9270fd31b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The conversation is on the left and a tool panel is on the right. The panel's tabs include Changes, a pull request, Desktop, a report file and Shell. Under Shell, a second row names each of the agent's shells, such as "backend6", "frontend2" and "setup". Below the output there is a progress bar with back and forward arrows and a "Live" marker. Below that, a "Search Command History" list shows every command with a status dot, a duration and a start time.

Difference from LGTM Studio: Devin shows the agent's shells as a record that can be searched and stepped through. LGTM Studio shows one live grid for each session and has no list of the commands that ran.

### Cursor: a Terminal tab in the panel beside an agent's conversation

[Mobbin screen](https://mobbin.com/screens/9c12aefb-daa1-482a-bbe7-9eaff84f2967?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A list of agents is on the left, and the selected agent's conversation is in the centre. A right panel has five tabs: Setup, Secrets, Git, Desktop and Terminal. The Terminal tab shows one prompt, "workspace $", with no session tabs and no status line. The conversation stays visible while the terminal is open.

Difference from LGTM Studio: the terminal belongs to one agent's environment and sits beside its conversation. In LGTM Studio the terminal is a whole project tab, and the owner picks the worktree from a select.

### Bolt.new: one bottom panel with an agent tab and a user tab

[Mobbin screen](https://mobbin.com/screens/38707e43-1f98-49b6-9d21-7c607db610d3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat is on the left. The right side has a file tree, an editor, and a panel under the editor. The panel has two tabs, "Bolt" and "Terminal", and a plus. The "Bolt" tab shows the output of the process that the agent runs. The "Terminal" tab is the user's own shell.

Difference from LGTM Studio: the agent's output and the user's shell are two named tabs of one panel. LGTM Studio marks an agent's session with an eye icon in the same tab row as the owner's sessions.

### Replit: a Shell pane beside the preview and the files

[Mobbin screen](https://mobbin.com/screens/8327a792-fe66-45fd-b10d-6647ce812d71?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The workspace has four columns: the agent chat, a Shell pane, the app preview, and a file tree. The Shell pane has its own tab with a close cross and a plus. A thin line above the grid shows the folder and the shell name, with search, clear and close icons at its right. The grid itself holds only the prompt.

Difference from LGTM Studio: the shell is one pane among several that are visible together. LGTM Studio gives the terminal the whole window and shows the folder in the status line at the bottom.

### Replit: a Console that stacks the output of every running process

[Mobbin screen](https://mobbin.com/screens/e4e121eb-19f5-4702-a67f-c25c6dc2daf4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A "Console" tab lists the running processes one above the other. Each block has a header with the command, an "Ask Agent" link, a port number and the time since it started. A "Show Only Latest" switch and a "Clear Past Runs" button sit at the top right. Each block folds with a chevron.

Difference from LGTM Studio: every running command is visible at once, and each one has a link that hands it to the agent. LGTM Studio shows one session at a time and has no link from output to an agent.

### GitHub: Codespaces, a terminal panel under the editor with a split button

[Mobbin screen](https://mobbin.com/screens/37875e94-9cf7-4164-932f-fdc1ad32d6b3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The file explorer is on the left and the open file is in the centre. A panel under the file has six tabs: Problems, Output, Debug Console, Terminal, Ports and Comments. The Terminal tab's header names the shell, "bash", and holds a plus with a menu, a split icon, a bin, a maximise chevron and a close cross. The prompt shows the user, the folder and the branch.

Difference from LGTM Studio: the terminal can be split into panes and can be maximised from its own header. LGTM Studio has tabs only, and the terminal is always the full tab.

## Options for LGTM Studio

1. Put the agent's sessions and the owner's sessions in two named groups in the tab row. This follows Bolt.new's "Bolt" and "Terminal" tabs. It costs width in the row on a laptop, and it needs a rule for a session after the owner takes it over.
2. Add a command history list under a watched session, with a status, a duration and a start time for each command. This follows Devin. It needs the engine to record each command an agent runs, which the feature list does not plan.
3. Show an agent's terminal in a panel beside that agent's conversation, and keep the Terminal tab for owner shells. This follows Cursor, and it matches the feature list's "The main conversation can switch to a terminal." It makes two places that show a terminal, and the owner must learn which one holds what.
4. Add a split button so two sessions sit side by side. This follows GitHub Codespaces. It adds pane focus, pane resizing and a phone fallback, and DESIGN.md's rule that the terminal is one character grid would need a new wording.
5. Move the folder and the branch from the bottom status line to a thin line above the grid, with clear and search icons. This follows Replit's Shell pane. It changes the visible look of an existing screen, so it needs the owner's approval, and it gives up one row of the grid at the top in exchange for one at the bottom.
6. Replace the sentence in the "No sessions are open" state and the sentences in the watch bar with labels and button names. This follows the owner's rule of 2026-11-20 about instructions on screens. The watch bar then needs another way to show that keys are not sent, such as a "Watching" badge beside "Take over".
7. Rename a session in place on the tab, with F2 as well as a double click. This follows DESIGN.md's "Terminal sessions" rule, which the mock does not follow today. The cost is small.

## Ideas that are not in the feature list

- A searchable history of the commands an agent ran, with status, duration and start time ([Devin](https://mobbin.com/screens/f7e9f607-dc8f-49ab-b42e-26f9270fd31b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A scrubber that steps back through an agent's shell output, with a "Live" marker ([Devin](https://mobbin.com/screens/f7e9f607-dc8f-49ab-b42e-26f9270fd31b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- An "Ask Agent" link on a block of process output, which hands that output to the agent ([Replit](https://mobbin.com/screens/e4e121eb-19f5-4702-a67f-c25c6dc2daf4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A view that stacks the output of every running process, with "Show Only Latest" ([Replit](https://mobbin.com/screens/e4e121eb-19f5-4702-a67f-c25c6dc2daf4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A split button that puts two shells side by side ([GitHub](https://mobbin.com/screens/37875e94-9cf7-4164-932f-fdc1ad32d6b3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A search icon in the terminal's header for finding text in the output ([Replit](https://mobbin.com/screens/8327a792-fe66-45fd-b10d-6647ce812d71?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "Warp terminal window with session tabs": 8 results, 5 usable. No result was from Warp. The usable screens came from Amazon Web Services, Devin, Bolt.new, Cursor and Replit.
- "Replit workspace with a shell terminal pane beside the code editor": 8 results, all from Replit, 2 usable. The other six showed logs or the preview.
- "GitHub repository file browser with a folder tree and the open file": run for the Files tab. It returned three GitHub Codespaces screens, and one is used here for its terminal panel.
- Warp: not found on Mobbin.
- Zed was not searched, because the search budget ran out.
