# Files tab: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

The sources are `design/mock/js/screens/files.js` and `design/mock/js/editor.js`.

- The tab has two columns: a folder tree on the left and the open file on the right. On a phone it shows one of the two, with a back arrow on the file.
- The tab reads the inspected worktree. A branch with no worktree falls back to the project's worktree or the base branch, and a note above the tree says so.
- The top of the left column shows the branch, a status dot, the machine name and the worktree's folder.
- A "Go to file" field filters every path as the owner types. It lists at most 40 matches as full paths.
- Folders come first in the tree, then files, both by name. Each folder opens and closes on its own.
- A file row shows a letter for its Git status: A for added, M for changed, R for renamed. A folder shows a dot when anything inside it has changed.
- A file with unsaved edits shows a second dot in the tree until it is saved.
- The file's header shows the path as breadcrumbs, the status word, the line count and the size. It holds "Open in Git" for a changed file, "Save", "Commit", and a menu with "Wrap long lines", "Copy path" and "Download".
- The editor is CodeMirror 6. It has line numbers, bracket matching, search, undo history, and colouring for TypeScript, JavaScript, Markdown, Cascading Style Sheets (CSS), JavaScript Object Notation (JSON) and YAML. It has no autocomplete.
- "Save" turns the file into an uncommitted change in the worktree. Control+S, or Command+S on a Mac, does the same.
- "Commit" appears after a save. It opens a dialog for one commit message, with a "Generate" button that writes the message with the cheap model. The dialog commits that one file as the owner.
- The base branch is read-only, and the header shows a lock with "Read-only".
- When the worktree's agent is working and the file has unsaved edits, a line above the editor says that the agent is told which file the owner changed.
- A Scalable Vector Graphics (SVG) file shows as a picture. A binary file, or a text file over the size limit, shows its size and a "Download" button.
- An offline machine keeps the tab's structure and shows "is offline" in place of the file.
- Only one file is open at a time. Unsaved text for other files is kept in the page and comes back when the file is opened again.

## What is planned and not built

- From `docs/features.md`, "A file explorer in the Git view": the tab was built on 2026-11-14 "for worktrees on this computer only". Files on another machine are not built.
- From the same section, "Where the files come from": "Downloads are not built yet." The built size limit is 512 kilobytes, not the 1 megabyte first agreed.
- From the same section: "References in questions, approvals and plans link to a file and open it here." No such link was found in the two source files, which only read the `f` address parameter.
- From `docs/features.md`, "Simple editing in the Files tab": the tab was built "without the generated commit message". The mock shows the "Generate" button, and the engine does not write the message.
- From the same section: "If an agent changed the file since then, the save is refused and the owner sees both versions. The mock does not show this case."
- From `docs/features.md`, "Which component shows the files": the Pierre packages stay the choice for diffs in the Git tab. Their licences "have not been read yet".
- From `docs/features.md`, "An IDE button that starts a code server on demand": an idea, not agreed. The Files tab stays the small editor, and an "IDE" button would open a code server for the inspected worktree in its own browser tab.
- From `docs/features.md`, "Keyboard shortcuts for navigation": an idea. Moving between files and panes without the pointer is not decided.

## How other products do it

### GitHub: the repository code view with a file tree panel

[Mobbin screen](https://mobbin.com/screens/ee3e030f-ef84-4ad6-b838-9ac06d7607e3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A "Files" panel on the left holds a branch select, a plus button, a search button, a "Go to file" field with its shortcut key shown, and the tree. The right side starts with one row for the latest commit: the author, the message, the short hash, the time and a "History" link. Under it, a table lists each entry with its last commit message and last commit date. The README file is drawn below the table with a pencil icon for editing.

Difference from LGTM Studio: every file row says which commit last touched it and when. LGTM Studio's tree shows only the uncommitted status letter.

### GitLab: one file, with its actions in the file's header

[Mobbin screen](https://mobbin.com/screens/4a17ba69-9e41-4c0f-b754-ebf1ce84ee97?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A branch select and path breadcrumbs sit at the top left, with "Find file", "Blame", "History" and "Permalink" at the right. A row under them shows the last commit, its author and its hash with a copy button. The file's own header shows the name and the size, then "Blame", an "Edit" button with a menu, "Lock", "Replace", "Delete", and icons for copy, raw view and download. The code is shown with line numbers and no tree beside it.

Difference from LGTM Studio: the file has history, blame and lock actions, and no folder tree on the same page. LGTM Studio keeps the tree visible and offers save, commit, copy path and download.

### GitLab: editing one file and committing it on the same page

[Mobbin screen](https://mobbin.com/screens/ab04390a-ff56-4d30-a115-4c7d493103bb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is titled "Edit file" and has two tabs, "Write" and "Preview changes". The editor's header shows the branch, the file path in a field, and a "No wrap" button. Under the editor, the page has a "Commit message" box that is already filled with "Update server.test.js", a "Target Branch" field, and the buttons "Commit changes" and "Cancel". There is no separate save step.

Difference from LGTM Studio: the commit message and the target branch are on the page under the editor, and one button commits. LGTM Studio saves first and then opens a dialog for the message.

### GitLab: a preview of the change before the commit

[Mobbin screen](https://mobbin.com/screens/57c7a1ac-fa0f-442f-836a-da4bbeca9c0c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The "Preview changes" tab replaces the editor with a diff of the edit. Removed lines are red and added lines are green, with a hunk header between distant changes. The commit message, the target branch and "Commit changes" stay under the diff.

Difference from LGTM Studio: the owner reads the diff of the edit without leaving the file. LGTM Studio sends the owner to the Git tab through "Open in Git".

### GitHub: Codespaces, with a tab for each open file and an image shown in place

[Mobbin screen](https://mobbin.com/screens/81856131-3eff-4e1a-81ca-c01a45d9daeb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The explorer on the left lists the folder tree, with icons in its header for a new file, a new folder, refresh and collapse. Two files are open, and each has its own tab above the editor. Breadcrumbs under the tabs give the folder and the file. The selected file is an image, and it is drawn in the centre. The status bar at the bottom gives the image's dimensions and its size.

Difference from LGTM Studio: several files stay open as tabs, and the tree's header can create files and folders. LGTM Studio opens one file at a time and cannot create a file.

### v0: a file tree, file tabs, and a "Changes" tab that shows a file's diff

[Mobbin screen](https://mobbin.com/screens/3dd95168-b620-4c77-a80f-1d3e26dd8140?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat is on the left, the file tree is in the middle, and the file is on the right. Two tabs sit above the file: the file itself and the same file with "(Changes)" after its name. The Changes tab shows the whole file with removed lines in red and added lines in green. Unchanged stretches fold into rows such as "6 hidden lines" and "21 hidden lines". Icons at the top right step to the previous and next change.

Difference from LGTM Studio: a file's changes open as a second tab of the same file, in the file view. LGTM Studio keeps diffs in the Git tab.

### Replit: a Files pane with a search field and grouped root files

[Mobbin screen](https://mobbin.com/screens/8327a792-fe66-45fd-b10d-6647ce812d71?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The Files pane is the right column, with a "Library" and "Files" switch above it and a "Search files" field under the switch. Folders and source files come first. Two labelled groups follow, "Packager files" and "Config files". A changed folder shows a green count, and a new file in it shows the letter A.

Difference from LGTM Studio: package and configuration files are moved out of the main list into their own groups, and a folder shows how many files changed. LGTM Studio lists every file by name and marks a changed folder with a dot.

## Options for LGTM Studio

1. Put the commit message under the editor, filled with a default such as "Update Sidebar.tsx", and make one button save and commit. This follows GitLab's "Edit file" page. It removes the dialog and the separate "Save" step, and it loses the saved-but-uncommitted state that the Git tab uses today.
2. Add a "Changes" view of the open file, as a second tab or a switch in the file's header. This follows v0 and GitLab's "Preview changes". It duplicates part of the Git tab, and it needs the diff component to work inside the Files tab.
3. Let several files stay open as tabs above the editor. This follows GitHub Codespaces and v0. It takes one more row, and it works against the feature list's rule that the owner edits only one file at a time.
4. Show the last commit for the open file in its header: the author, the message, the hash and the time, with a "History" link into the Git tab. This follows GitHub and GitLab. It needs one more read from the machine for each file opened.
5. Replace the dot on a changed folder with the count of changed files inside it. This follows Replit. The cost is small, and the count must stay correct as agents change files.
6. Group lock files and configuration files at the end of the tree under their own labels. This follows Replit. It hides the true order of the folder, which can confuse someone who knows the repository from an editor.
7. Remove the sentence in the commit dialog and the sentence above the editor, and put their facts into labels. For example, the commit button can read "Commit to feat/agent-console/shell". This follows the owner's rule of 2026-11-20 about instructions on screens.

## Ideas that are not in the feature list

- A last commit line for the open file, with author, message, hash and time ([GitHub](https://mobbin.com/screens/ee3e030f-ef84-4ad6-b838-9ac06d7607e3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- "Blame" and "History" buttons on one file ([GitLab](https://mobbin.com/screens/4a17ba69-9e41-4c0f-b754-ebf1ce84ee97?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Lock" button that stops others from changing a file ([GitLab](https://mobbin.com/screens/4a17ba69-9e41-4c0f-b754-ebf1ce84ee97?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A preview of the edit as a diff before the commit, on the same page ([GitLab](https://mobbin.com/screens/57c7a1ac-fa0f-442f-836a-da4bbeca9c0c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A target branch field beside the commit message ([GitLab](https://mobbin.com/screens/ab04390a-ff56-4d30-a115-4c7d493103bb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- New file and new folder buttons in the tree's header ([GitHub](https://mobbin.com/screens/81856131-3eff-4e1a-81ca-c01a45d9daeb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- Buttons that step to the previous and next change inside one file ([v0](https://mobbin.com/screens/3dd95168-b620-4c77-a80f-1d3e26dd8140?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A count of changed files on each folder ([Replit](https://mobbin.com/screens/8327a792-fe66-45fd-b10d-6647ce812d71?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "GitHub repository file browser with a folder tree and the open file": 8 results, all from GitHub, 3 usable. Three were Codespaces screens and five were the repository's front page.
- "GitLab web editor with a file tree and a commit button": 8 results, all from GitLab, 4 usable. None showed a file tree beside the editor, so the query was weak for the tree and strong for edit and commit.
- "Cursor agent changes review with a diff and a create pull request button": run for the Git tab. Its v0 result is used here for the file tree with a "Changes" tab.
- "Replit workspace with a shell terminal pane beside the code editor": run for the Terminal tab. Its first result is used here for the Files pane.
- No Cursor or Zed screen of a file tree was found. Cursor's results showed its cloud agent view only, and Zed was not searched.
