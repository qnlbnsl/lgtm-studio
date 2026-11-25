# Message box: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `Composer`, `ContextPicker` and `Attachments` in `design/mock/js/ui.js`, and from "The composer and agent threads, set on 2026-11-08" in `design/DESIGN.md`.

- The message box is one text box over one tool row, inside one border. The text box grows with the draft up to 220 pixels. Its placeholder names the agent, such as "Message the project lead…".
- Enter sends, and Shift and Enter adds a line. That hint is read only by screen readers.
- The tool row holds, in order: "Context", a paperclip, the account, the model, the effort, the context meter, and the send button at the right end. The code and `design/DESIGN.md` call the account "the subscription".
- "Context" opens a list above the box with a search field and two groups, "Tasks" and "Files", of at most six rows each. Typing @ at the start of the draft or after a space opens the same list. A chosen item becomes a chip above the text box with a remove button.
- Files and images come from the paperclip, from a drop on the box, or from a paste. A drop shows "Drop to attach" over the box. A file over 100 megabytes is refused with a notice.
- A large photo is redrawn on the owner's device to a long edge of 2,048 pixels. Its chip shows a small picture, the name, and "Resized from 33.4 MB to 1.1 MB", with a link "Send original". The send button is disabled while a photo is "Resizing…".
- The account menu lists the accounts of the agent's harness with the machine each is on. An account that cannot be used is disabled and says "Signed out" or "Not reported".
- The model menu is headed "Model, from the next turn", and the effort menu is headed "Effort, from the next turn". On the live website the choices come from the harness's own report.
- The context meter is a number, such as "34%". It turns amber at 80 percent. It shows a dash when the harness has not reported.
- The send button is an arrow. It is disabled when the draft is empty, when the connection is not live, or while a photo is resizing. While a reply is pending it becomes a square that stops the reply.
- In the Chat tab and the side panel, a line above the box shows the agent's activity and a "Stop agent" button. The Project view leaves that line out, and its send button stops the agent while the agent works and the draft is empty.
- A failed agent replaces the box with its reason, "Sign in on Buildbox" and "Resume". A stopped agent replaces it with "Resume". Both say "Your draft is kept." when there is a draft.
- A draft is kept for each thread. In the Project view, a line "About statement 2" above the box says what the next message is about (`design/mock/js/screens/mission.js`).
- When the box itself is 470 pixels wide or less, "Context" shows only its icon (`design/mock/css/base.css`). The message board has its own variant with a kind, the recipients and a "Needs reply" switch (`design/mock/js/screens/messages.js`).

## What is planned and not built

- "Files and images sent with a message" in `docs/features.md`. Agreed and not built in the engine. The engine must store the files, pass them to the harness and serve them back. The limit of 100 megabytes is not decided. On the live website, attaching a file is refused today.
- "Safety of uploaded files, required by the owner" in `docs/features.md`. The server must enforce size and count limits as bytes arrive, find the real type from the bytes, and never show a Scalable Vector Graphics drawing as a live element. A security review with malicious samples is part of the work.
- "Click an object in a mock to refer to it in the chat" in `docs/features.md`. A click on an object in a mock puts a reference into the message box. How the reference looks in the box is undecided.
- "A one-off agent on a repository, without a project." in `docs/features.md`. The owner starts one agent by choosing a harness and changes the model and effort from the tool row. The section says it is not built in unit s10-lead.
- "2. The message board" in `docs/features.md`. "A mailbox per agent, with receipts" is partly built. A message to a busy Claude agent is steered into its running turn, and anything else waits for the turn to end. The message box does not show which of the two will happen.
- "3. Threads" in `docs/features.md`. "Steer a thread from any client" is an idea. It covers interrupting a thread from a phone.
- "Keyboard shortcuts for navigation" in `docs/features.md`. State: Idea. How shortcuts coexist with typing in the message box is not decided.
- "One input over one tool row" in `design/DESIGN.md` says the meter reads "34% context" and loses the word "context" below 420 pixels. The current code shows the meter as a bare number at every width. The document and the code differ.

## How other products do it

### Cursor: a prompt box with the repository and branch above it

[Mobbin screen](https://mobbin.com/screens/05d779e2-ce20-4d8d-bcb7-dd6ca6887ee8?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Two pickers sit above the box, outside its border: the repository and the branch. Inside the box, the foot row holds the model with its effort as one label, "Codex 5.3 High", and a second picker, "MCPs". An image button and a round button sit at the right end. Under the box are three small buttons with ready prompts, such as "Run security audit". The recent agents are listed under that.

Difference from LGTM Studio: Cursor shows where the agent will work above the box and joins model and effort into one label. LGTM Studio shows no branch at the box and has separate pickers for the account, the model and the effort.

### Cursor: one message sent to several models at once

[Mobbin screen](https://mobbin.com/screens/5086a06b-e76b-49ad-a5e4-c2100759b758?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The model menu opens under the box. A switch at its top reads "Use Multiple Models". Each model is a row with a checkbox, and a checked row gets a count picker from "1x" to "4x". The label in the box then names every chosen model. A second switch, "Long-running", is marked "Preview".

Difference from LGTM Studio: Cursor lets one prompt start several runs, and LGTM Studio's box always speaks to one agent with one model.

### Claude: a reply box with the model and its mode at the right

[Mobbin screen](https://mobbin.com/screens/98255484-f7b1-43c3-b9a1-1c6f971de3bd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The box has a placeholder "Reply…" and one foot row. A plus button sits at the left end. The right end reads "Haiku 4.5 Extended" with a chevron, and then a round stop button while the reply runs. Nothing else is in the row.

Difference from LGTM Studio: Claude has three controls in the row and hides attaching behind the plus button. LGTM Studio has seven controls in the row.

### ChatGPT: a one-line box with the model in the page's head

[Mobbin screen](https://mobbin.com/screens/83ee2b5a-1692-4041-896b-67d85ac1696d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The box is a single line with rounded ends and the placeholder "Ask anything". A plus button is at its left end, and a microphone and a round voice button are at its right end. The model is not in the box. It is a menu at the top left of the page, and each entry has a name and one line of description.

Difference from LGTM Studio: ChatGPT keeps the model out of the box and offers voice input. LGTM Studio keeps every setting in the box and has no voice input.

### Devin: a mode picker whose rows say what each mode is for

[Mobbin screen](https://mobbin.com/screens/3563ed9d-b08f-4d5d-b8a4-9204bd55ff83?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The box has a foot row with a picker "Agent", a plus button and a menu of three dots at the left, and the send button at the right. The picker opens a list of five rows. Each row has an icon, a name, and one grey line, such as "Fast and good at long-horizon planning" or "2.5x faster, 2x more expensive, same intelligence".

Difference from LGTM Studio: Devin's rows state the trade of each choice, and LGTM Studio's model and effort rows are names only.

### Gumloop: a model menu with shortcuts and a facts card

[Mobbin screen](https://mobbin.com/screens/a3f8b130-cfc5-4ddc-8fd9-b95a4a6927bf?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

An attached file sits as a chip at the top of the box, above the text. The foot row holds a plus button, the model's icon and "17 connectors". The model menu has a search field, then three shortcut rows named "Recommended", "Smartest" and "Fastest", then one row per provider that opens a further list. A card beside the highlighted row shows bars for speed and intelligence, the provider, the context size as "1M tokens", and checks for tool calling and vision.

Difference from LGTM Studio: Gumloop shows a model's context size and abilities before it is chosen, and LGTM Studio shows the context in use only after the agent runs.

### Emergent: every setting behind one plus button

[Mobbin screen](https://mobbin.com/screens/1aa8ca69-26e4-4e50-a3a4-3222eb2fc967?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The box has only three controls: a plus button at the left, and a microphone and the send button at the right. The plus button opens a menu of four rows: "Upload file", "Import from Github", the current model with a further list, and "Advanced controls".

Difference from LGTM Studio: Emergent keeps the row nearly empty and costs two clicks to change the model. LGTM Studio shows every setting and costs one click.

### Spellbook: a labelled chip that says what the message is about

[Mobbin screen](https://mobbin.com/screens/7d6cb2f0-92cf-4134-bedd-131819e9cce3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A chip at the top of the box reads "Focus on", then a file icon and the file's name, then a remove button. The placeholder is under it. The foot row holds a paperclip, "Library" and "Legal sources" at the left, and a microphone and the send button at the right.

Difference from LGTM Studio: Spellbook puts the subject of the message inside the box as a chip. LGTM Studio's Project view puts "About statement 2" on a separate line above the box, and its context chips carry no label.

## Options for LGTM Studio

1. Join the account, the model and the effort into one picker that reads "Opus 5.5 · High". This follows Claude's "Haiku 4.5 Extended" and Cursor's "Codex 5.3 High". The menu would hold three groups. It frees width on a phone. The cost is that a signed-out account is one level deeper.
2. Move "Context", the paperclip and the account behind one plus button. This follows Claude, ChatGPT and Emergent. The row keeps the model, the meter and send. The cost is a second click for an attachment, and the @ key and the drop still work.
3. Add a value to each model and effort row, such as the context size and which account limit it draws on. This follows Devin's rows and Gumloop's card. It uses values and not sentences. It needs the model report to carry those values.
4. Show the agent's branch and machine as one quiet value above the box. This follows Cursor's repository and branch pickers. `design/DESIGN.md` removed the worktree line on 2026-11-08, so this reverses a decision.
5. Put the statement reference inside the box as a chip, "Statement 2", beside the context chips. This follows Spellbook's "Focus on" chip. It removes the separate line above the box. The cost is small.
6. Add a microphone button for dictation. This follows ChatGPT, Cursor and Emergent. The owner works from a phone. It is not in the feature list, and it needs a choice between the browser's speech service and one that the service runs.
7. Show whether a message sent now is steered into the running turn or waits for the turn to end, as a word on the send button's tip or beside the meter. No reference showed this. It follows from the mailbox rules in the feature list. The cost is one more state to keep true for each harness.

## Ideas that are not in the feature list

- Dictation into the message box. ChatGPT, Cursor, Emergent and Gumloop each show a microphone.
- One message sent to several models, with a count for each. Cursor has "Use Multiple Models".
- A switch for each tool server on the message box. Cursor has "MCPs" with a search and a switch per server.
- Ready prompts as small buttons under an empty box. Cursor has "Run security audit" and "Improve AGENTS.md".
- Shortcut rows in the model menu, such as "Fastest" and "Smartest". Gumloop has them.
- A model's context size and abilities shown before it is chosen. Gumloop has a card for the highlighted model.

## Searches that were run

- "ChatGPT message composer with model picker and attached file". 1 result, from ChatGPT. It was usable, and it shows an empty box with no attachment. The query was weak, and it was not repeated because of the budget.
- "AI app builder prompt box with context mentions and model selector". 8 results. 5 were usable: Cursor, Devin, Gumloop, Emergent and Base44. No result showed an @ mention list or a context meter.
- The Claude and Spellbook screens came from the Project view searches. The first Cursor screen came from the chat thread search.
- No search returned a context meter in a message box, so the meter stays without a reference.
- v0, Lovable, Replit and Codex were not searched by name, because the budget of 14 searches was spent.
