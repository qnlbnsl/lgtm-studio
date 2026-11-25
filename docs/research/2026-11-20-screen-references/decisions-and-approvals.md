# Decisions and approvals: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

LGTM Studio has two forms for the same thing. The docked form is in `design/mock/js/screens/mission.js` (`Dock`, `Ask`, `Question`, `TeamAsk`). The older request cards are in `design/mock/js/ui.js` (`RequestBlock`, `BriefCard`, `MergeBrief`).

- In the Project view, every decision that waits sits in a dock above the message box. The dock has an amber left edge, takes at most 58 percent of the window's height, and scrolls inside.
- Each waiting decision is one line: a label "Question" or "Approval", the title, the agent that asks when it is not the project lead, the age, and a chevron. The first line starts open. Each line opens and closes on its own.
- An open question shows one sentence, a link "Statement 1", and its answers as radio rows. A row holds a short label, an optional example in monospace such as "P-42", and a "Recommended" tag.
- A last row, "Something else", opens a text box "Your answer". Control and Enter, or Command and Enter, sends it.
- Marking a row only marks it. What the marked answer leads to shows under the rows. The button "Send answer" stays disabled until a row is marked.
- An approval in the dock shows the command in a code box, or "Input hidden by the harness". Under it is the tool and the folder. Its two rows are "Allow once" and "Deny", side by side.
- After a row is marked on an approval, a checkbox appears beside "Send answer". It reads "Always allow in this project" or "Always deny in this project".
- "Why it matters" is a folded line in the foot, and it shows only when the request has that text. "Open thread" shows when a team's agent asked.
- A request that the project lead holds is one note line, such as "Run the web tests · waiting on Project lead".
- An answered decision leaves the dock and stays in the conversation where it was asked. It shows a check, the title, "You", the answer and the time. Its title opens the full question again with the rows disabled.
- The older card (`BriefCard`) is used in the Chat tab and the side panel. It shows the brief, the age, one line of summary, and the recommendation with a sparkles icon. Its buttons are "Accept", "Deny", "Details" and "Open thread". One click on "Accept" sends the answer.
- "Details" on the older card opens a report: who asked, what it blocks, the question, the command and folder with copy buttons, "What it is about", "Why it matters", "Why this answer", "Other answers" with a "Choose" button each, a field "Your own answer", and numbered references.
- An approval on the older card has two split buttons. "Allow" has a menu with "Always allow", and "Deny" has a menu with "Always deny". On the live website the menu shows the harness's own wording.
- A merge request still uses the older card. It shows a brief, "Users will notice:", four facts (Tests, Review, Changes, and conflicts into the target branch), "Review and merge", and "Send back" with a field "What should change?". A failed start also uses the older card, with the reason and one button, "Sign in on Buildbox" or "Resume".
- Home shows each request as a row, and the expanded row holds the same block in its "detail" shape (`design/mock/js/screens/home.js`).

## What is planned and not built

- "What is undecided" under "A better Define phase" in `docs/features.md`. Whether the docked form replaces the older cards in the Chat tab, a team's thread, the side panel and Home is undecided.
- "What is undecided" under "A better Define phase" in `docs/features.md`. A merge request and a failed start still use their earlier cards in the Project view.
- "What is undecided" under "A better Define phase" in `docs/features.md`. An approval cannot carry a reason when the owner denies it.
- "What is undecided" under "A better Define phase" in `docs/features.md`. Whether a sent answer can still be changed before the project lead acts on it is undecided.
- "What is undecided" under "A better Define phase" in `docs/features.md`. The open questions of a Define record do not count in the sidebar, the bell or Home.
- "A one-sentence reason on every approval request" in `docs/features.md`. The card would say what the agent is attempting. A harness reports the command and not the reason, so the engine would have to ask the agent or read its last message.
- "Questions that carry their own brief" in `docs/features.md`. Agreed for the mock and not built in the engine. The engine's question records and the agents' instructions need the brief, the recommendation, the reasons and the cited files.
- "A sprint is proposed on the board, with one approval" in `docs/features.md`. The agent puts proposed cards on the board and asks one short question. "Deny" makes the agent remove the cards.
- "The lead has two independent permissions, allowed by default." in `docs/features.md`. The owner suggested a control with "Allowed", "Not allowed" and "Always ask for permission". The section says unit s11-permissions builds the change.
- "Status" in `AGENTS.md`. The standing permission rules remain unbuilt, pending the owner's harness recordings. "Always allow in this project" depends on them.
- "What a thread shows besides messages" in `design/DESIGN.md` says a held request reads "Held by Sage, the project lead" and an unconfirmed answer offers "Refresh this request". The current code shows "waiting on Project lead" and has no refresh control. The document and the code differ.

## How other products do it

### Claude: one question in a panel that takes the place of the message box

[Mobbin screen](https://mobbin.com/screens/34aa9592-2138-4be6-95f6-4aa7410e9bb9?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A panel sits at the foot of the conversation, where the message box normally is. Its head holds the question, a pager "1 of 2" with arrows, and a close button. The answers are four numbered rows, and the first row is highlighted with an arrow at its right end. A fifth row holds a field "Something else" and a "Skip" button. One line under the panel names the keys: arrows to move, Enter to select, and Escape to skip.

Difference from LGTM Studio: Claude shows one question at a time and pages through the rest, and LGTM Studio lists every waiting decision as its own line. Claude sends on Enter, and LGTM Studio needs "Send answer".

### Unify: a question docked above the message box, with a reason under each answer

[Mobbin screen](https://mobbin.com/screens/ca1f0060-e1ab-4ba2-98f9-6a9789f2afc4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The panel sits directly above the message box, and the message box stays visible. The head reads "1 / 4" and the question, with a collapse icon. Each answer is a bold label with one line of description under it, and a small key badge with its number at the right. The first label ends with "(Recommended)". The last row is a field "Other". The foot holds "Previous", "Skip" and "Next". Above the panel, the conversation says "Waiting for your answer 8s".

Difference from LGTM Studio: Unify writes the consequence inside each row, which the owner rejected on 2026-11-20. Unify offers "Skip", and LGTM Studio does not.

### Emergent: a docked question with checkboxes and an automatic answer

[Mobbin screen](https://mobbin.com/screens/99a1a63e-01dc-4310-b836-f110b0fc30a3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The panel is docked at the foot and titled "Agent has questions for you", with a close button. The question has four rows with checkboxes, and the last row reads "Something else". The foot holds a pager "Question 1 of 4", a button "Auto-answer" and a primary button "Next". In the conversation above, one line repeats "Agent has questions for you" with the words "Waiting for answers" and a chevron.

Difference from LGTM Studio: Emergent allows several answers to one question and can answer for the user. LGTM Studio allows one answer and always waits for the owner. Emergent leaves a marker in the conversation where the question was asked, and LGTM Studio leaves none until the answer is sent.

### Claude: all questions as one form in a tab

[Mobbin screen](https://mobbin.com/screens/607a00db-cf9a-40c2-a9b6-c1cb0e8eca94?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat column shows a link line "Claude has some questions". The work area opens a tab named "Questions" with a heading and four questions on one page. Each question is a bold title, one grey line, and its answers as chips. Marked chips are filled. Each question also has chips "Decide for me" and "Other", and a field "Other…". The first question is a free text box. One button, "Continue", sits at the bottom right.

Difference from LGTM Studio: Claude collects every answer and sends them together with one button, and LGTM Studio sends each decision by itself.

### Notion: a permission choice as two radio rows

[Mobbin screen](https://mobbin.com/screens/e91c0e04-d5b9-474b-90f7-ecab1caf8200?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A small dialog titled "Configure access" opens over the agent's chat. Under "Select permission" are two rows with a radio button each. The first row reads "Read and post messages" with a "Recommended" tag and one line of description. The second row reads "Read messages only". One wide button, "Connect", sends the choice.

Difference from LGTM Studio: Notion puts the width of the permission into the rows themselves, and LGTM Studio puts "Allow once" and "Deny" in the rows and the standing answer in a checkbox that appears afterwards.

### Trello: a consent page that lists what will and will not be allowed

[Mobbin screen](https://mobbin.com/screens/14dc3849-7cdf-489c-bde7-c9b807ba8f9c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

This is a sign-in consent page, and it is cited for its structure only. One list is headed "will be able to" and a second list is headed "will not be able to". A third block names the boards and workspaces the grant covers. Two buttons at the bottom right read "Deny" and "Allow".

Difference from LGTM Studio: Trello states the full reach of the grant before the answer, and LGTM Studio's "Always allow in this project" does not show which later commands it will cover.

### Relevance AI: agent tasks sorted by what they wait for

[Mobbin screen](https://mobbin.com/screens/7690aff3-4875-4930-9b99-52e52a7f0b07?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A table of agent tasks has tabs with counts: "All tasks 17", "To review 4", "Escalated", "Errored" and "Completed 17". Each row shows the task, the agent, a status chip such as "Pending approval" or "Inactive", and two dates. A filter panel lists Agent, Status, Tool usage, Errors and other fields. The toolbar has "Export" and "Save view".

Difference from LGTM Studio: Relevance AI separates what waits for a person into tabs by reason, and LGTM Studio's Home is one table with filters.

### LangChain: an inbox of agent threads that need attention

[Mobbin screen](https://mobbin.com/screens/23d60ab4-1d3a-4c08-b4eb-3abad2ce6095?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is named "Inbox" and has three tabs: "Needs Attention", "Completed" and "All". Each row is one line with the agent's name, a red "Error" tag, the thread's title and a date. The sidebar shows a count badge on "Inbox" and on "Chat".

Difference from LGTM Studio: LangChain's row is one dense line with the reason as a tag, and LGTM Studio's older card takes several lines before any button.

## Options for LGTM Studio

1. Show one decision at a time with a pager "1 of 3". This follows Claude, Unify and Emergent. It uses the least height. It contradicts the owner's rule that opening one line never closes another, and it hides the titles of the other decisions.
2. Add number keys that mark a row, and keep "Send answer" as the only way to send. This follows the key badges in Unify and the numbered rows in Claude. It costs a rule for when the keys apply, because the message box sits directly under the dock.
3. Add a row that hands the decision back, such as "Use the recommendation". This follows Claude's "Decide for me" chip and Emergent's "Auto-answer". It saves a read when the owner trusts the project lead. The risk is a habit of delegating decisions that the Define stage exists to settle.
4. Make the standing answer a row and not a checkbox. The rows become "Allow once", "Always allow in this project", "Deny" and "Always deny in this project". This follows Notion's two permission rows. It removes a control that appears only after a click. It costs two more rows on every approval.
5. Under "Always allow in this project", show the rule that would be stored, such as the command pattern and the folder. This follows Trello's lists of what is and is not allowed. It depends on the standing permission rules, which are unbuilt.
6. Offer every open Define question as one form in the end-result column, with one button "Send 3 answers". This follows Claude's "Questions" tab. It suits a first draft with many questions. The risk is that the answer to one question changes the wording of another before it is read.
7. Recut Home and the Chat tab to one-line rows with a reason tag, and open the docked form from the row. This follows LangChain's inbox and the tabs in Relevance AI. It settles the undecided item about the older cards. It costs the one-click "Accept" that the older card has today.

## Ideas that are not in the feature list

- A "Skip" on a question, which tells the agent to go on without that answer. Claude and Unify both have it.
- A question that takes several answers at once, as checkboxes. Emergent has it.
- A waiting timer in the conversation, such as "Waiting for your answer 8s". Unify has it.
- A marker line in the conversation at the place a docked decision was asked. Emergent has it.
- A saved filter for requests, such as "Approvals older than one hour". Relevance AI has "Save view".
- Keys that move between answers and send. Claude names them under its panel. The entry "Keyboard shortcuts for navigation" in `docs/features.md` covers moving around the website and does not cover answering.

## Searches that were run

- "Claude question with selectable answer options above the message box". 8 results, all from Claude. 2 were usable: the docked question and the "Questions" tab.
- "Cursor agent asking for approval to run a terminal command". 3 results, all from Cursor. None was usable. They showed an automation's settings and a cloud environment dialog.
- "AI agent permission request with allow once and always allow buttons". 8 results. 1 was usable, the Notion dialog. The rest were consent dialogs for a whole product.
- "Claude chat asking permission to use a tool with allow buttons". 8 results, none from Claude. 1 was usable for structure only, the Trello consent page.
- No search returned an approval of one command inside a conversation with "Allow once" and "Always allow". That pattern stays without a reference.
- The Unify and Emergent screens came from the chat thread search "Devin session chat with agent work steps and progress". The Relevance AI and LangChain screens came from the roster and message board searches.
- Replit, Lovable, v0 and Codex were not searched by name, because the budget of 14 searches was spent. No screen from them appeared in any result.
