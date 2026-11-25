# Project view: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `design/mock/js/screens/mission.js`, `design/mock/js/define.js`, `design/mock/css/mission.css` and the section "The Project view, decided on 2026-11-20" in `design/DESIGN.md`.

- The Project tab is the first tab of every project in the mock, at `/p/<project>/project`. `homeTab()` in `define.js` opens it in the mock and opens the board on the live website, because the Project view is not wired to the engine.
- A bar at the top holds the stage strip. It lists Define, Design, Plan, Build, Review and Hand over. A finished stage gets a check, and the current stage is marked.
- The right end of that bar holds one count button, such as "2 questions open" or "3 requests". The button opens the first waiting decision.
- Below the bar are two columns. The conversation with the project lead takes the centre. The end result takes the right column, which is between 400 and 640 pixels wide, at 44 percent of the width.
- At 1,100 pixels or less the view is one column. Two tabs named "Conversation" and "End result" switch between the halves.
- The owner's messages sit on the right in a tinted bubble with a time. The project lead's messages sit on the left with an avatar, the name "Project lead" and a time. The name line shows only when the speaker changes. The reading column is at most 700 pixels wide, and reading text is 14 pixels with a line height of 1.62 and a measure of 66 characters.
- The project lead's reading and commands fold into one line, such as "Read 4 files, searched 3 times and ran 3 commands · 30 s · 1 failed". The line opens to a list of verbs and arguments, and a failed line is marked.
- A change to the end result leaves one note line in the conversation, such as "Rewrote the end result as draft 2 · 3 statements changed".
- While the project lead works, the last line of the conversation is a pulsing dot and the lead's activity. No status line sits above the message box, and the send button becomes the stop button.
- The end result has a header with a badge: "Not written yet", "Draft 2" or "Agreed". Beside it is "changed 3 min ago", or "agreed at 14:02" with the number of changes asked since.
- The end result has these sections in order: "Needs you" (only after Define), "Goal", "What will exist", "Left out", "Next" and "Teams". "Next" names what each later stage will do and which harness and permission the agents use. "Teams" is one row per team with the team lead, the team name, and a request count or an activity.
- A statement is a number, one bold sentence, and the rest of its text. Under it is a row with a state badge, a link to the open question that decides it, a "Previous wording" line that opens, and an "Ask for a change" button.
- The state badge reads "Changed" or "Agreed" in Define. After Define it reads "Holds", "Being built", "In review", "Stuck", "Dropped" or "Not started". A statement with a note after agreement reads "Change asked".
- "Ask for a change" puts a line "About statement 2" above the message box, and the next message carries that reference. A question links to its statement, and a statement links to its question. Either link scrolls to the other half and flashes it.
- The foot of the end result holds "2 questions open" and one primary button, "Agree and start Design". The button reads "Agree and start Plan" when no screens change. It is disabled while a question is open. After agreement the foot reads "Agreed" with "Open the mocks" or "Open the board".

## What is planned and not built

- "The Project view, decided on 2026-11-20" in `design/DESIGN.md`. The view is built in the mock only. The live website keeps opening the board until builders wire the view to the engine.
- "The Project view, decided on 2026-11-20" in `design/DESIGN.md`. A heavier change of scope redefines the project, and that is not designed.
- "What is undecided" under "A better Define phase" in `docs/features.md`. Whether the owner can edit a statement directly, and what happens to conflicting edits, is undecided.
- "What is undecided" under "A better Define phase" in `docs/features.md`. How many changes after agreement are allowed, and who judges that a change is too heavy, is undecided.
- "What is undecided" under "A better Define phase" in `docs/features.md`. Whether the Board and Chat tabs leave the project when the board moves to the repository is undecided.
- "What is undecided" under "A better Define phase" in `docs/features.md`. The open questions of a Define record do not count in the sidebar, the bell or Home.
- "What is undecided" under "A better Define phase" in `docs/features.md`. A newly created project shows an empty end result, because the sample lead is scripted for one project only.
- "What is undecided" under "A better Define phase" in `docs/features.md`. The engine has no record for a draft end result, its statements or its questions. The client application programming interface (API) needs one.
- "A Define workspace with a conversation and an evolving end-result document" in `docs/features.md`. Agreement on the end result would be separate from permission to plan or start work. Small projects should finish Define without a sprint structure.
- "A repository board with project prefixes on its tickets" in `docs/features.md`. State: Idea. The work board would move to the repository level, which leaves the Project view as the main view of one project.
- "Click an object in a mock to refer to it in the chat" in `docs/features.md`. A click on an object in a mock would put a reference into the message box. The Project view already does this for a statement.

## How other products do it

### Claude: a chat column beside a tabbed work area

[Mobbin screen](https://mobbin.com/screens/885e249d-6fe3-4461-9e87-7f743f6fbd36?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat is a narrow column on the left, about a quarter of the width. The rest of the window is a work area with its own tabs: "Design Files", a sketch, and "Pricing.html". The work area has a tool row with a "Tweaks" switch and "Comment", "Edit" and "Draw" buttons. In the chat, the agent's tool runs are single grey lines such as "Writing ×4, Done, Listing files", each with a chevron. The chat column itself has two tabs, "Chat" and "Comments".

Difference from LGTM Studio: Claude gives the produced thing most of the width and the chat the smaller share, and LGTM Studio does the opposite. Claude's right side holds several documents as tabs, and LGTM Studio's right side holds one document.

### Claude: the result as a list of lines that open

[Mobbin screen](https://mobbin.com/screens/f4221e3c-92ce-4e02-bf02-033ffe51aba4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The right side is a document named "Design System". It is a list of groups: Type, Colors, Spacing, Components and Brand. Each item under a group is one line with a chevron on the left and a small dot on the right. One line shows a check in place of the dot. The chat on the left ends with a numbered list titled "Big asks to make this real".

Difference from LGTM Studio: Claude shows each part of the result as a closed line with a state mark, and LGTM Studio shows every statement in full with a badge row under it.

### Spellbook: a chat beside a document with three editing modes

[Mobbin screen](https://mobbin.com/screens/7d6cb2f0-92cf-4134-bedd-131819e9cce3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat takes the left half and a contract takes the right half. A menu above the document offers three modes: "Editing: Make any changes", "Reviewing: Suggest changes" and "Viewing: View the file, but make no changes". The answer in the chat starts with a folded line "Thinking complete". The message box holds a chip "Focus on" with the file's name and a remove button. Two tabs under the chat read "Chat" and "Review table".

Difference from LGTM Studio: Spellbook lets the reader edit or suggest in the document itself, and LGTM Studio's owner changes a statement only by sending a message about it.

### Elicit: a report beside a list of stages with their results

[Mobbin screen](https://mobbin.com/screens/6b7e1d2e-50e7-4414-b2b6-b542449be72f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The report fills the centre. A panel on the right lists four steps under "Status": "Gather papers", "Screen papers", "Extract data" and "Generate report". Each step has a check, one line with its result, such as "50 papers found", and a "Details" button. The last step holds the primary button, "Save PDF". A chat box sits under the steps. A menu at the top of the page lists the output of every stage, and the current one is marked.

Difference from LGTM Studio: Elicit's stage list says what each stage produced and links to it, and LGTM Studio's stage strip shows only names and checks.

### Langdock: a chat beside a document that the agent writes

[Mobbin screen](https://mobbin.com/screens/579fb137-50fa-42d0-9222-4970f35648d5?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat is a narrow column, and a document opens on the right with its title and "Last saved 0 s ago". The document has a formatting row and an "Export" button. In the chat, the document appears as a small card with its title and an expand icon. The message box has a "Canvas" switch, a model picker and a stop button while the agent writes.

Difference from LGTM Studio: Langdock's document is a free text that the reader can format and export, and LGTM Studio's end result is a fixed set of sections with numbered statements.

### Contractbook: comments tied to a phrase of the document

[Mobbin screen](https://mobbin.com/screens/73d09b75-ba7c-4a46-b069-36b1e1af019b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The document is in the centre and a panel named "Comments and suggestions" is on the right. A comment in the panel quotes the phrase it is about, and then shows the comment and its age. The same comment appears as a small box beside the phrase in the document, with a "Resolve" link. A count badge sits in the margin of the commented line.

Difference from LGTM Studio: Contractbook keeps a note beside its text until someone resolves it, and LGTM Studio sends the note into the conversation and shows it under the statement as "Your note" with no resolved state.

### Coda: a document with a chat panel whose answers can be inserted

[Mobbin screen](https://mobbin.com/screens/1a6fa6a9-a16e-4358-8f64-3234b4e84122?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The document takes the centre, and a chat panel takes the right side. Each answer in the chat ends with two buttons, "Insert" and "Copy". The message box has a "Context" picker set to "This page". The panel has a collapse control on its edge.

Difference from LGTM Studio: Coda's reader decides whether an answer goes into the document, and LGTM Studio's project lead rewrites the end result on its own.

### Cursor: an agent's conversation beside its Git changes

[Mobbin screen](https://mobbin.com/screens/fa7df34d-7288-4c26-a0e9-95a84921e34e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The conversation is the left column, and a panel on the right has five tabs: "Setup", "Secrets", "Git", "Desktop" and "Terminal". The Git tab shows the pull request's title, its state, its branch, and the diff of each file. The conversation ends with "Worked for 37s" and a box "3 Files Changed" that lists each file with its added lines. A bar above the message box holds two buttons, "Review" and "Start new agent".

Difference from LGTM Studio: Cursor puts the agent's work products in tabs beside the conversation, and LGTM Studio puts them in separate project tabs that replace the conversation.

## Options for LGTM Studio

1. Give the right column its own tabs, with "End result" first and then the mocks, the board and the changes. This follows Claude's tabbed work area and Cursor's panel. It costs a second tab row under the project's tab row, and the "Agree and start Design" button is hidden while another tab is open.
2. Show each statement as one line that opens, with its state mark at the right end. This follows Claude's design system list. It fits more statements on a screen. The risk is that the owner agrees to statements whose second half was never opened.
3. Add a "Suggest" mode to the end result, in which the owner rewrites a statement in place and the project lead accepts or answers. This follows Spellbook's three modes. It needs the engine record for statements, and it needs a rule for an edit that conflicts with the lead's own rewrite.
4. Keep a note beside its statement until it is closed, with the outcome in a button such as "Mark handled". This follows Contractbook. It gives "Change asked" an end. It costs one more state per statement and one more thing to count.
5. Let each stage in the strip carry its result and open its output, such as "Define · 3 statements agreed" and "Design · 2 mocks approved". This follows Elicit's status panel. It costs width in the bar, or it moves the stages into the "Next" section of the end result.
6. Let the owner fold the end result away and bring it back with one control on its edge. This follows the collapse control on Coda's chat panel. It gives the conversation the whole width while the owner reads a long answer. The risk is that the "Agree and start Design" button is out of sight when the last question closes.
7. End each of the project lead's turns with one line that names what changed, such as "Worked for 37 s · draft 3 · 2 statements changed". This follows Cursor's "Worked for" line and "Files Changed" box. It replaces the separate note line, and it needs the turn's duration from the engine.

## Ideas that are not in the feature list

- Export or share the agreed end result as a file or a link. Elicit has "Save PDF", Langdock has "Export", and Claude has "Share".
- A "Comments" tab beside the conversation that lists every note on the end result. Claude has "Chat" and "Comments" tabs.
- A menu of every stage's output, reachable from the stage strip. Elicit has this menu at the top of its report.
- A "Details" button on each finished stage that opens what the stage produced. Elicit has one per step.

## Searches that were run

- "Claude conversation with a document panel open on the right". 8 results, all from Claude. 3 were usable, and the rest were Claude's project list and its empty states.
- "AI coding agent chat beside a plan document with an approve button". 8 results, none from a coding product. 5 were usable for the layout of a chat beside a document: Spellbook, Elicit, Langdock, Coda and Contractbook. No result showed an approve button on a plan, so the query was weak for "Agree and start Design".
- The Cursor screen came from the chat thread search, "Cursor agent conversation with tool calls and file changes".
- v0, Lovable and Replit were not searched by name, because the budget of 14 searches was spent. No screen from them appeared in any result.
