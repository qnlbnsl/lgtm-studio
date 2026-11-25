# Mocks tab: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

The source is `design/mock/js/screens/mocks.js`. The large preview reuses the canvas from `design/mock/js/screens/design.js`.

- The tab has a secondary tab row with four views: Screens, Changes, Data and Components. Each shows a count, and Changes shows the count of changes the owner has not seen.
- A status line under the row shows "2 of 3 screens approved", or an "Approved" badge with its age. It holds "Request changes" and "Approve mocks".
- Screens is a table with one row for each screen. The columns are a small thumbnail, the name, the status, a note, the revision, the comment count and the age.
- A screen's status is "Draft", "Changed" or "Approved". A changed row carries a warning mark.
- A row opens the screen's own view. The screen is drawn at its real size on a canvas, with zoom, "Fit", a phone or desktop switch, and previous and next arrows.
- A panel beside the canvas holds the screen's note, an "Approve" button for that one screen, and "Edit design", which opens the screen in OpenPencil.
- The same panel holds the screen's comments as a thread. The field under it reads "Ask the designer for a change". In the mock, the designer agent replies and makes a new revision.
- Changes is a table with the screen, the revisions, what changed, who changed it and when. A row with "Compare" opens the two revisions side by side under it.
- Data is a table of every field the screens use, with a sample value and the screens it appears on. Each field is marked "Matches", "Differs", "New" or "Unavailable" against the schema, and a switch filters by that mark.
- Components is a table of the components the screens use. Each is marked "Real" or "Draft", with a small preview, a use count and the screens it appears on.
- "Request changes" opens a dialog with a screen select and one text box. It sends the text to the designer agent as a message.
- "Approve mocks" asks first. It marks every screen approved, moves the project to the Plan stage, and has the project lead start a Planning team.
- After approval, a comment is passed to the project lead as a change request, and the screen keeps its revision.
- On the live website, comments and the single-screen "Approve" are refused, and thumbnails are not drawn.

## What is planned and not built

- From `docs/features.md`, "10. Design and mock modes": "Design mode and mock mode" was requested on 2026-11-07 and is "not designed". Each project lists all of its mocks.
- From the same section: "Recorded mock previews and OpenPencil" is decided, with "implementation pending". The engine renders a project's recorded mocks itself, in a sandboxed frame on another hostname.
- From `docs/features.md`, "A full-size OpenPencil editor beside the chat": a maximize button lets the editor fill the page while the chat stays open at the side. It needs the owner's approval of a mock first.
- From `docs/features.md`, "Click an object in a mock to refer to it in the chat": a click on one object puts a reference to it into the chat composer. What the reference carries, and how it looks, are still to decide.
- From `design/DESIGN.md`, "Stages, stage teams, and mocks, decided on 2026-11-07": "Changes go through the AI." The screen should change in place without reloading while the owner has it open.
- From the same section: OpenPencil "cannot save into the repository yet".
- From the same section: "Approve mocks promotes the draft components". The mock marks screens approved and does not change a component from "Draft" to "Real".
- From the same section: "Converting the design into code belongs to the Build team." The tool that does it is to be built later.

## How other products do it

### Ditto: a list of frames, one frame, and a comments panel

[Mobbin screen](https://mobbin.com/screens/a6f1658c-22d3-4513-8ae4-5b34b3f925e1?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A "Frames" column on the left shows each screen as a thumbnail with its name, and one thumbnail carries a comment count. The selected frame is drawn in the centre, and one piece of text in it is outlined. A panel on the right is titled "Selected text" and has four tabs: Edit, Activity, Comments with a count, and Variants. The Comments tab has a field at the top, then a list of comments with a reply link each, and a menu to sort by date or unread and to show resolved comments.

Difference from LGTM Studio: the list of screens stays visible beside the open screen, and a comment belongs to one selected element. LGTM Studio leaves the list to open a screen, and a comment belongs to the whole screen.

### Claude: a comment pinned to one element, with "Send to Claude"

[Mobbin screen](https://mobbin.com/screens/1dc4800e-1824-4ab6-a46e-1a582a6221e7?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat is on the left, with "Chat" and "Comments" tabs above it. The generated page is on the right under a toolbar with "Comment", "Edit", "Draw" and a zoom value. One element of the page is outlined, and a small "Comment" box is attached to it with the buttons "Comment" and "Send to Claude". The chat composer shows the comment as a chip above the text field, under the line "Submit your changes when ready".

Difference from LGTM Studio: the owner points at an element and the reference travels to the agent with the message. This is the planned "Click an object in a mock to refer to it in the chat", which LGTM Studio has not built.

### Figma: a comment thread on the canvas and a list of all comments

[Mobbin screen](https://mobbin.com/screens/2047d593-6cd4-48bc-8529-bfebc5207483?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The frames sit on a canvas, and a round pin marks the spot a comment is about. The thread opens beside the pin as a small window with the author, the time, the text, an attached image and a "Reply" field. A check icon in the window's header resolves the thread. A "Comments" panel on the right lists every thread with a search field, and each entry names its number and its page.

Difference from LGTM Studio: comments sit at a point on the design and can be resolved one by one. LGTM Studio's comments are one running thread for each screen with no resolved state.

### Canva: a table of designs waiting for review

[Mobbin screen](https://mobbin.com/screens/476563dd-d2bd-418e-8ddf-8ba75b6130c8?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is titled "Designs to review". A table has six columns: the design with a thumbnail, the submitter, the reviewers, the status, the submitted date and the due date. The status is a badge, here "Approved". A "Status" filter is at the top left and a sort menu at the top right. A second list, "Your submissions", is in the side menu.

Difference from LGTM Studio: the table says who submitted each design and who must review it. LGTM Studio's Screens table shows the revision and the comment count, and the reviewer is always the owner.

### Fiverr: reviewing a delivery, with draft comments and one "Done" button

[Mobbin screen](https://mobbin.com/screens/709a7cc6-9f8f-48c2-81db-bd356495d4c2?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The delivered image fills the centre, with previous and next arrows at its sides and a strip of thumbnails under it. A pin on the image marks where a comment points. A "Comments" panel on the right shows the comment with a "Not yet submitted" badge and a field to edit it. A "Done" button at the bottom of the panel finishes the review.

Difference from LGTM Studio: comments are drafts until the owner finishes, so the designer gets them together. In LGTM Studio each comment goes to the designer agent at once and can start a new revision.

### Base44: a version history beside the preview

[Mobbin screen](https://mobbin.com/screens/6a1aaaf1-2630-4cf0-84f7-04967ddfaa3c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A "Version History" column on the left lists each version with a name and its age. One version has a "Current" badge and another has a "Live" badge. The selected version is drawn in the preview on the right. The top bar reads "Viewing Version History" and holds a page select, "View Code", "Restore This Version", "Publish This Version" and "Exit".

Difference from LGTM Studio: any earlier version can be opened at full size and restored. LGTM Studio's Changes view compares two revisions as small thumbnails and has no restore.

### Manus: a version history with a thumbnail and actions on each row

[Mobbin screen](https://mobbin.com/screens/4142fbf7-14e2-4841-aad7-70e51ddf2dfe?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A "Version history" window lists versions from the newest down. Each row shows the age, one line about what changed, a version identifier and a thumbnail at the right. The newest row has "Publish", a published row has a "Published" label, and each older row has "Preview" and "Rollback".

Difference from LGTM Studio: each change carries its own thumbnail and its own actions in the row. LGTM Studio's Changes rows hold text, and the thumbnails appear only after "Compare".

### Lovable: each change is a card in the chat that sets what the preview shows

[Mobbin screen](https://mobbin.com/screens/b6847b07-4933-471b-9153-0f1838ada822?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The chat is on the left and the preview fills the right. The agent's reply lists what it did in short rows, such as "Generated image" and "Edited" with a file name. Under the reply, a card names the change and reads "Previewing latest version", with a bookmark icon and a "Code" link beside it. Suggested next requests sit as chips above the message field.

Difference from LGTM Studio: the revision is chosen from the conversation, and the preview follows. LGTM Studio keeps revisions in the Changes table, apart from the comments.

## Options for LGTM Studio

1. Keep the list of screens visible as a narrow column of thumbnails beside the open screen. This follows Ditto. It removes the back arrow and the previous and next arrows, and it takes about 200 pixels from the canvas on a laptop.
2. Attach a comment to one element of the screen, and show the reference as a chip in the message to the designer. This follows Claude and Ditto, and it is the planned "Click an object in a mock to refer to it in the chat". It needs the mock frame to report which element was clicked, across the sandbox boundary.
3. Hold comments as drafts and send them together with one button, such as "Send 3 comments". This follows Fiverr and GitHub's pending comments. It stops a new revision from starting after each sentence, and it adds a state the owner can forget to submit.
4. Give each comment a resolved state, and hide resolved ones by default. This follows Figma and Ditto. It needs a rule for who resolves a comment: the owner, or the designer agent when it makes the revision.
5. Put a thumbnail of each revision in the Changes rows, and let a row open that revision at full size on the canvas. This follows Manus and Base44. It needs the engine to keep a rendering of each revision.
6. Add "Restore revision 2" to a revision. This follows Base44 and Manus. It conflicts with the rule that changes go through the designer agent, unless restore is sent to the agent as a request.
7. Show the designer's change as a card in the comment thread that switches the canvas to that revision. This follows Lovable. It joins Changes and Comments for one screen, and the Changes view then only serves the cross-screen list.

## Ideas that are not in the feature list

- A comment that can be resolved, with a filter for resolved comments ([Figma](https://mobbin.com/screens/2047d593-6cd4-48bc-8529-bfebc5207483?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- One list of every comment across all screens, with search ([Figma](https://mobbin.com/screens/2047d593-6cd4-48bc-8529-bfebc5207483?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- Draft comments that are sent together when the review is done ([Fiverr](https://mobbin.com/screens/709a7cc6-9f8f-48c2-81db-bd356495d4c2?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- Opening an earlier revision at full size and restoring it ([Base44](https://mobbin.com/screens/6a1aaaf1-2630-4cf0-84f7-04967ddfaa3c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A thumbnail on each row of the change history ([Manus](https://mobbin.com/screens/4142fbf7-14e2-4841-aad7-70e51ddf2dfe?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Draw" tool for marking up the screen before sending it to the agent ([Claude](https://mobbin.com/screens/1dc4800e-1824-4ab6-a46e-1a582a6221e7?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A "Variants" tab that holds other versions of one selected element ([Ditto](https://mobbin.com/screens/a6f1658c-22d3-4513-8ae4-5b34b3f925e1?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A due date on a design that waits for review ([Canva](https://mobbin.com/screens/476563dd-d2bd-418e-8ddf-8ba75b6130c8?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "Figma design file with a comment thread on a frame": 8 results, all from Figma, 8 usable. They are eight steps of one comment flow, so one is cited.
- "v0 generated screen preview with a version history list": 8 results, 3 usable. No result was from v0. The usable screens came from Base44, Manus and Lovable.
- "design review screen with a list of screens, a status for each, and an approve button": 8 results, 1 usable. Most were approval tables for timesheets and expenses. The Canva screen is cited.
- "Vercel preview deployment with a comment thread on the page": 8 results, 3 usable. No result was from Vercel. The usable screens came from Ditto, Claude and Fiverr.
- Vercel: not found on Mobbin.
- v0 was not found for this screen. It was found for the Files tab and the repository Design page.
- No reference was found for the Data view, which checks the fields of a mock against the schema.
