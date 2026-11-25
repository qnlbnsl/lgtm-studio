# Git tab: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

The source is `design/mock/js/screens/git.js`. The merge review dialog is `MergeReview` in `design/mock/js/dialogs.js`.

- One bar at the top holds a worktree menu, the worktree's owner and machine, the ahead and behind counts, a "Changes", "History" and "Conflicts" switch, and a unified or split diff switch.
- The worktree menu lists every worktree of the project and the base branch. Each entry shows the branch, its kind (Project, Team, Agent or Base), and "2 changed" or "Conflict".
- An integrate strip sits under the bar. A team or agent worktree gets "Update from base" and "Integrate into" its parent branch. The project worktree gets "Open merge review". The base branch gets a lock and a sentence.
- The integrate button is disabled with a reason in its hover text, such as "Commit or discard the changes first".
- Changes has two columns. The left column holds the commit form, then the "Staged" and "Changes" groups. The right column holds the selected file's diff.
- The commit form is a message box, a line that reads "2 staged files · commits as you" or the reason the commit is blocked, a "Generate" button, and "Commit". "Generate" writes the message with the cheap model.
- A file row shows an icon, the file name, the folder path and a status letter. Discard and stage buttons are on the row. Each group has "Stage all" or "Unstage all", and Changes has "Discard all".
- Discard asks first. The dialog lists the files and says that the change cannot be undone.
- The diff header shows the status letter, the name, the path, the added and removed counts, and a "Stage" or "Unstage" button. A renamed file shows the old and new paths.
- A binary file shows a note in place of a diff. A large diff, such as a lock file, is folded behind "Show anyway".
- History lists commits with a node, the subject, the author, the hash, the age and the branch or tag names. Commits that are not on the parent branch look different from shared ones.
- A selected commit shows its subject, author, time, a copy button for the hash, and each changed file as a section with its diff. Each section folds on its own.
- Conflicts appears as a third view while a merge is stopped. The left column has "Continue merge", "Abort" and the conflicted files with "2 left", "Ready" or "Resolved".
- Each conflict shows three panes, "Ours", "Base" and "Theirs", and a "Use ours", "Use theirs" and "Use both" switch. A chosen conflict shows one "Result" pane. "Mark resolved" finishes a file, and "Reopen" undoes that.
- "Open merge review" opens a dialog with a brief, a pull request title with a character count, a description, a "Generate" button for both, the evidence, the changed files, and "Approve and merge" or "Approve without evidence".
- When the review was delivered through GitHub, the strip and the dialog show the pull request number as a link and its state.
- The live website adds "Refresh Git", "Fetch", "Pull" and "Push" buttons to the top bar.
- On a phone the list and the detail are separate screens with a back arrow, and the diff is always unified.

## What is planned and not built

- From `docs/features.md`, "Pushing a finished project and opening its pull request": "Automatic push triggers and manual provider text editing below remain planned."
- From the same section: "A small menu next to the button chooses the branch the pull request merges into." No base menu was found in the dialog.
- From the same section: "The project shows the pull request's number, a link, the state of its checks, and its review state." The strip shows the number, the link and one state word. It shows no checks.
- From the same section: "The project lead asks." The lead should ask the owner, in a question with an "Open pull request" button, when every team branch is integrated.
- From `docs/features.md`, "Git actions after the website connection": "Git text generation needs an owner-configured account and model." The "Generate" buttons are refused on the live website until then.
- From the same section: hosted pull request editing "still needs an authenticated provider text update action", and "Remote-machine Git remains unavailable."
- From the same section: the engine supports aborting a merge and updating from a parent. In `git.js`, `abortMerge` and `updateFromBase` still call `refuse()` when the screen is live.
- From `docs/features.md`, "6. Source control": "Remove agent credits when merging" is agreed. It is a setting, and the Git tab does not show it.
- From `docs/features.md`, "Which component shows the files": the Pierre packages are the choice for diffs in the Git tab. The mock draws its own diff table.
- From `design/DESIGN.md`, "Git's columns": "The worktree list folds into a rail of 40 pixels." The file read today has a worktree menu and no list or rail.
- From `design/DESIGN.md`, "The commit pane": "Sync is pinned at the top" and "Integrate is pinned at the bottom." The file read today puts integrate in a strip above both columns.
- From `design/DESIGN.md`, "Conflicts": "Conflicts have their own section above Staged changes," and "Open in Terminal is the way out for hard cases." The file read today has a separate Conflicts view and no "Open in Terminal" button.
- From `design/DESIGN.md`, "Terminal and Git get the whole window": "All files stacks every changed file with a sticky name." Changes shows one file at a time. Only History stacks files.

## How other products do it

### GitHub: the Files changed tab of a pull request

[Mobbin screen](https://mobbin.com/screens/72783a50-4cc2-4e3d-83f9-048f9a2455cf?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The pull request's title, state badge and branch names are at the top, above four tabs: Conversation, Commits, Checks and Files changed. A filter row offers "Changes from all commits", "File filter" and "Conversations", and shows "0 / 2 files viewed" beside "Review changes". A narrow list on the left has a "Filter changed files" field and one row for each file with a status icon. Every file's diff is stacked on the right, and each has a fold chevron, a bar of added and removed lines, a "Viewed" checkbox and a comment icon.

Difference from LGTM Studio: all files are stacked in one scroll, and the owner ticks each one as viewed. LGTM Studio's Changes view shows one file at a time and has no viewed state.

### GitHub: a file marked as viewed folds, and a bar counts progress

[Mobbin screen](https://mobbin.com/screens/694f75af-6c29-4ec2-b58a-76ac518a3bb4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The first file has its "Viewed" box ticked, and its diff is folded to one header row. The counter reads "1 / 2 files viewed" with a progress bar under it. The second file stays open below. The button at the top right has changed to "Finish your review" with a count of pending comments.

Difference from LGTM Studio: the review has a visible measure of how much is left. LGTM Studio's merge review dialog lists the files with their counts and links to the Git tab, and it does not track what the owner has read.

### GitHub: a comment on one line of the diff

[Mobbin screen](https://mobbin.com/screens/af7b6a4f-1053-476a-b288-8fa603df8dca?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A comment sits inside the diff, directly under the line it is about. It shows the author, a "Pending" badge, the text, a reaction button and a "Reply" field. The diff continues under the comment. The pending comment is held until the review is finished.

Difference from LGTM Studio: feedback is attached to a line and is sent in one batch. The feature list says that in LGTM Studio the owner sends a review back with one reason, which goes to the project lead.

### GitHub: comparing two branches

[Mobbin screen](https://mobbin.com/screens/ef755b94-c85b-4c20-aba0-2f468203fb0f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Two selects at the top choose the base and the branch to compare. A summary row gives the number of commits, changed files and contributors. The commits are listed under it, each with its hash and a copy button. A "Split" and "Unified" switch sits above the first diff.

Difference from LGTM Studio: the base of the comparison is a select on the page. LGTM Studio fixes each worktree's parent branch and shows only the ahead and behind counts.

### Cursor: a Git tab beside the agent's conversation, with the pull request and its diff

[Mobbin screen](https://mobbin.com/screens/fa7df34d-7288-4c26-a0e9-95a84921e34e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The agent's conversation is in the centre and holds a "3 Files Changed" card that lists each file with its added count. The right panel's Git tab shows the pull request title and number, an "Open" badge, the source and target branches, and a button at the top right that reads "Merging…". Three views sit under it: Diff, Review and Commits. Each file's diff is a folding block with its added count and a "New" badge, and the lock file also has a "Generated" badge.

Difference from LGTM Studio: the diff and the merge button stay beside the conversation with the agent. LGTM Studio puts Git in its own tab and the merge in a dialog.

### Devin: a pull request tab with Changes, Bugs, Description and Discussion

[Mobbin screen](https://mobbin.com/screens/448233a9-9d70-4483-91c8-8e2930e35224?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The right panel has tabs for the work log, the changes and the pull request. The pull request tab has four views: Changes, Bugs, Description and Discussion with a count. Each file header shows the name, the folder, the added and removed counts and a "Mark as viewed" control. The diff is unified with line numbers.

Difference from LGTM Studio: the description and the discussion are views beside the diff, not fields in a dialog. LGTM Studio has no "Bugs" view of problems found in the change.

### ChatGPT: a finished coding task with its diff and a "Create PR" button

[Mobbin screen](https://mobbin.com/screens/ee06bf6f-a0da-41c4-b491-ad03fd4d00c6?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The header shows the task's title, the date, the repository, the branch, and the added and removed line counts. "Archive", "Share" and a split "Create PR" button sit at the right. The left column holds the request, a summary of what was done, a testing list and a "File (1)" row. The right column has "Diff" and "Logs" tabs, and each file is a folding block with a "New" badge.

Difference from LGTM Studio: the summary, the tests that ran and the diff are on one screen, and one button opens the pull request. LGTM Studio splits these between the Git tab and the merge review dialog.

### Graphite: a pull request page with a generated description and a checks rail

[Mobbin screen](https://mobbin.com/screens/e67c4768-e366-4dcc-bd3f-4099fd1d0060?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page shows the title, the author, the source and target branches, and "1 file +30/−0". A "Stack" section lists the pull requests that build on each other. The "Description" section has a "Generate" button and four headings: a short summary, "What changed?", "How to test?" and "Why make this change?". A rail on the right lists the draft state, the automated review result, the status checks, the approvals, labels and assignees. "Review changes" and "Publish" are at the top right.

Difference from LGTM Studio: the generated description has fixed headings, and the evidence is a rail of separate rows. LGTM Studio generates a title and a free description in a dialog, with the evidence under them.

## Options for LGTM Studio

1. Add a "Viewed" checkbox to each file and a "1 / 2 files viewed" counter to the merge review. This follows GitHub. It needs the engine to remember what the owner read for each commit, and a rule for when a later commit changes a viewed file.
2. Stack every changed file in one scroll in the Changes view, with sticky file names and folding. This follows GitHub, Cursor and DESIGN.md's "All files" rule. It makes staging one file at a time less direct, and large diffs need the "Show anyway" fold.
3. Turn the merge review dialog into a view of the Git tab with "Changes", "Description" and "Evidence" switches. This follows Devin and Graphite. It removes a wide dialog, and it changes the visible look of an existing screen, so it needs the owner's approval.
4. Show the diff of the inspected worktree in a panel beside the project lead's conversation. This follows Cursor and ChatGPT. It matches the feature list's "The project lead stays open in the side panel", and it competes for width with the end result document in the Project view.
5. Give the generated pull request description fixed headings, such as what changed, how it was tested and why. This follows Graphite. It costs one prompt template, and the owner may find the headings too rigid for small fixes.
6. Let the owner comment on a line of the diff, and send all comments to the project lead in one message. This follows GitHub's pending comments. It needs a new kind of board message that carries a file and a line.
7. Remove the hint sentences, such as "Stage the files that belong in the next commit." and the base branch sentence in the integrate strip. This follows the owner's rule of 2026-11-20. The blocked-commit reason already lives in the line under the message box.

## Ideas that are not in the feature list

- A "Viewed" checkbox for each file and a progress bar for the review ([GitHub](https://mobbin.com/screens/694f75af-6c29-4ec2-b58a-76ac518a3bb4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- Comments on one line of a diff, held as pending and sent together ([GitHub](https://mobbin.com/screens/af7b6a4f-1053-476a-b288-8fa603df8dca?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Filter changed files" field above the file list ([GitHub](https://mobbin.com/screens/72783a50-4cc2-4e3d-83f9-048f9a2455cf?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Changes from all commits" filter that narrows the diff to chosen commits ([GitHub](https://mobbin.com/screens/72783a50-4cc2-4e3d-83f9-048f9a2455cf?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A base and compare pair of selects for comparing any two branches ([GitHub](https://mobbin.com/screens/ef755b94-c85b-4c20-aba0-2f468203fb0f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Generated" badge on files such as lock files, beside the "New" badge ([Cursor](https://mobbin.com/screens/fa7df34d-7288-4c26-a0e9-95a84921e34e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Bugs" view that lists problems found in the change ([Devin](https://mobbin.com/screens/448233a9-9d70-4483-91c8-8e2930e35224?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Logs" tab beside the diff of a finished task ([ChatGPT](https://mobbin.com/screens/ee06bf6f-a0da-41c4-b491-ad03fd4d00c6?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Stack" section that shows pull requests that depend on each other ([Graphite](https://mobbin.com/screens/e67c4768-e366-4dcc-bd3f-4099fd1d0060?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "GitHub pull request files changed diff view": 8 results, all from GitHub, 8 usable. Four are cited.
- "source control panel with staged changes list and a commit message box": 8 results, 3 usable for this screen. No result showed a staged and unstaged file list with a commit box, so the staging part of the screen has no reference. The usable screens were from ChatGPT, Graphite and Devin.
- "Cursor agent changes review with a diff and a create pull request button": 8 results, 6 usable. One result was from Cursor. The others came from ChatGPT, Devin, Graphite, Cofounder, GitLab, v0 and Replit.
- No reference was found for resolving a merge conflict, or for a list of branches and worktrees. No search was left for them.
- GitLab returned a commit diff with "Inline" and "Side-by-side" buttons, which is the same pattern as LGTM Studio's diff switch, so it is not cited.
