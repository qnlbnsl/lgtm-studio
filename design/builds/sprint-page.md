# Sprint page: tabs in the middle, a rail about the sprint

This screen is in `design/mock-vite/` only since 2026-11-23. It was built in `design/mock/`, and that folder went back to the state of `master`, because the changes reached the live website and broke one of its journeys. That is issue 1574. Where the text below says `design/mock/`, read the file of the same name under `design/mock-vite/src/`.

The rail of this page was changed in the port on 2026-11-23. The lane is one button with a menu, the needs are "Prerequisites", and a move that stops a team asks first. "Next lane" and "Force to" below describe the page of 2026-11-22. The rules are in `design/DESIGN.md`, and the change is item 12 of issue 1572.

A task has no page of its own since 2026-11-23. It opens inside the Tasks tab, and the rail then lists the sprint's tasks. The rule is in `design/DESIGN.md`.

Built on 2026-11-22 in `design/mock/`, on sample data. It is not wired to the engine.
It replaces the page of 2026-11-21, whose blind review said it would not ship.

surface: app screen. The owner opens one sprint from the board several times a day.

reference: three prototypes of this page, built on 2026-11-21 and shown to the owner behind
a picker. The owner chose the tabs of one and the rail of another on 2026-11-22. He asked
for no search of other products. The gold screen `table-detail.html` of the screen-build
skill set the row and control sizes.

## What the owner decided

- Requests, Tasks and Plan are tabs, one at a time.
- The facts and the move sit in a rail on the right.
- The goal sits in the rail, because it was out of place under the title.
- The head holds only the title, because a taller head wastes space.
- The phone is not the focus for now. It must not break.

## Plan

First viewport: 1440 by 900. The conversation is open beside the page, so the page is
about 680 pixels wide.

| # | Region | Job | Container | Why this container | Rows visible in the first viewport | Empty state |
|---|---|---|---|---|---|---|
| 1 | Title row | Name the sprint and go back to the board | none | One line | 1, or more when a long name wraps | none |
| 2 | Tab row | Choose Requests, Tasks or Plan. Its right end holds the open tab's count and action | none | One line | 1 | Requests is absent when nothing waits |
| 3 | Requests tab | Answer what the agents ask about this sprint | object | Each request stands alone and is answered alone | 2 to 4 | The tab is absent |
| 4 | Tasks tab | The main list | table | Like rows | 12 stacked rows of 54 pixels while the conversation is open, 22 one-line rows of 30 pixels when the page is wide | "No tasks yet" |
| 5 | Plan tab | Read the plan | panel | A document in a frame | not a list | "No plan yet" |
| 6 | Rail: next lane | Name the next lane, mark each need, move the sprint | list | Needs are like items | 1 to 8 needs, then the button | Absent when the sprint is done |
| 7 | Rail: goal | Say the goal in one sentence | none | One sentence under its label | 1 to 4 lines | none |
| 8 | Rail: facts | Status, Phase, Priority, Epic, Team, Depends on | list | Label beside value, the same six on every sprint | 6 | "None" |
| 9 | Rail: evidence | Tests, review and merge of a finished sprint | list | Like items | 0 or 3 | Absent until the sprint has evidence |

Main offset at 1440: 0.22. The top bar, the row of views and the stage line of the project
take 118 pixels. The title row and the tab row take about 80. On the page of 2026-11-21 the
offset was 0.33, and 0.69 with two requests.

Navigation: three tabs, switched often. Three destinations of one object are a tab row.
The page is reached from the board, and the back arrow returns.

Parts:

- Tabs: built by hand as a `tablist` with roving focus and the left and right arrow keys.
  The mock has no tabs part that switches in place, and it has no build step for a library.
- Menu of lanes beside the move button: the mock's own `Menu` in `design/mock/js/ui.js`, in the split-button shape a request's Allow already has.
- Question before a forced move: the mock's own confirm dialog in the same file.

Motion: none. A tab switches at once.

- #1 scan: edges the page's left padding; groups by none; visuals the sprint's label as a
  tag; coloured nothing.
- #2 scan: edges the tabs' words on the page's left padding, the action on its right
  padding; groups by none; visuals a count beside Requests and Tasks; coloured the count of
  Requests, because it is what waits for the owner.
- #4 scan: edges the task label on the left padding, the plan button on the right; groups
  by none; visuals a check before "Done", a document button for a task with a part of the
  plan; coloured only a blocked status.
- #6 scan: edges the rail's padding; groups by none; visuals a green check for a met need
  and a clock for an unmet one; coloured the check, and the button when every need is met.
- #8 scan: edges labels on the rail's left padding and values on one line 76 pixels in;
  groups by none; visuals none; coloured the links only.
- feedback: adding a task shows the toast "Task added. The project lead was told." and the
  count on the Tasks tab goes up. A move changes Status, Team and the next lane in place,
  and a line appears in the lead's conversation beside the page.

## Captured

1440 by 900 in nine states: Requests (`/p/console/sprint/sp3`), Tasks of the same sprint,
Tasks with a plan (`sp7`), Plan (`sp7`), a move that is allowed (`sp8`), a move that is not
(`sp5`), the menu of lanes open (`sp5`), a finished sprint with evidence (`sp1`), and the
worst case. 390 by 844 in two states, to see that nothing breaks. Dark mode at 1440 for the
Tasks tab.

## Worst case

The switch is `?data=worst` before the `#`. SP-05 gets a name of 104 characters, seven
sprints it depends on, and a long epic name. SP-08 gets a name of one letter.

- The name wraps to three lines. Nothing is cut.
- The eight needs are eight lines in the rail, with "1 of 8" beside the lane's name.
- The long epic name and the seven sprints wrap inside the rail.

## Gate

The `pnpm ui-test` command is not installed in this repository. The eight space checks
were run from the skill's own file `space-checks.mjs` through this repository's Playwright.

- 1440, demo data and worst-case data: no finding, on the Requests, Tasks and Plan tabs.
- The Requests tab had a finding of uneven padding on each request block. The block's
  padding is now 12 pixels on each side, and the finding is gone.
- A first run flagged the hidden label of the last task column as cut text. The label
  moved into the column's name for screen readers, and the finding is gone.
- Two waivers of `empty-pane`, both in `design/mock/js/screens/project.js`. The open tab
  fills the page, so a short list leaves the rest empty. The rail is as long as its facts.
  This is the cost of one part at a time, which the owner chose. It is issue 1564.
- 390: the task list starts at 0.40 to 0.53, and at 0.73 with eight needs. The plan says
  0.22 for the desktop. The phone is not the focus for now.

The impeccable detector found nothing in the changed lines. It flagged one older rule,
`design/mock/css/screens.css:38`, a coloured left edge.

All 795 tests of `packages/studio` pass, and the build of the live website passes.

## Known costs

- The tasks and the plan are never on the screen together. A task's plan button opens the
  Plan tab at that part.

## Not checked

- The live website. This is the mock. In the live website the page shows no next lane and
  no move button, because the engine has no gated move yet.
- The browser journeys of the live website. They need a test engine. The names of the
  request buttons they look for are unchanged.
- Loading and error states. The mock has none for this page.
- Firefox and Safari. Only Chromium ran.
- A real screen reader. Names and keyboard order were read from the page, not heard.
- Right-to-left text and 200 percent zoom.
- The agent check of the ui-test skill. The mock has no UI SDK bridge.

## Blind review

A fresh reviewer was sent eleven pictures on 2026-11-22: eight states at 1440, dark mode,
and two at 390. It read the design document. It did not see the plan, the source or the
gate's output. Its verdict was "no", with no blocker, three findings labelled major and
sixteen labelled minor. All three majors are fixed. Of the minors, three stay open: 6, 12
and 15.

| # | Label | Plan row | What the reviewer saw | Answer |
|---|---|---|---|---|
| 1 | major | 4 | Every task title is cut at about 20 characters, while the pane below is empty | Fixed. The table stacks below 480 pixels of table width, as "The phone" in the design document says. The title is whole |
| 2 | major | 4 | The owner is an icon with no name | Fixed by the same change. The owner's name follows the status on the second line |
| 3 | major | 6, 8 | Two controls move the sprint, and the words do not line up. "Force move…" does not say where | Fixed. One place moves the sprint: the button names the lane, such as "Force to Review…", and an arrow beside it opens the menu of all five lanes. Status is a plain value. The label reads "Next lane" |
| 4 | minor | 3 | No request button is filled, and the design document says two things | Fixed in the design document. The buttons are outlined. Issue 1565, item 2 |
| 5 | minor | 4 | An empty Tasks tab drops the table's header, and the design document says two things | Fixed. An empty table keeps its header and one muted row. A stacked table has no header. The sprint in the picture has no plan, so it opens on Tasks by the rule |
| 6 | minor | 4, 8 | In dark mode the attention colour and the link colour look alike | Not fixed. It is the approved palette. Issue 1566, item 2 |
| 7 | minor | 6 | The sprint labels in unmet needs are green, like the check. "1 of 8" has no label | Fixed in part. A need names its sprint as plain text. The links stay under "Depends on". The count is unchanged |
| 8 | minor | 4 | Nothing shows that a row opens a task | Fixed in part. The task's label has the link colour. The plan button is unchanged |
| 9 | minor | 4 | The columns move from one sprint to the next | Fixed. The columns have fixed widths |
| 10 | minor | 2 | Add task is 24 pixels high, and every other button is 28 | Fixed. It is 28 |
| 11 | minor | 5 | A plan's headings are off the type scale | Fixed. The sample plans use 20, 15 and 13 pixels |
| 12 | minor | 5, 7 | The plan opens with a Goal that repeats the rail | Not fixed. It is the plan's content. Issue 1566, item 4 |
| 13 | minor | 2, 6 | The task count is stated three times | Fixed in part. The Tasks tab has no count of its own now |
| 14 | minor | 3 | The two request cards have different shapes | Fixed. On a narrow card "Open thread" is its icon, so both keep one line of buttons |
| 15 | minor | 3 | A request card does not name the task it blocks | Not fixed. It adds to the brief, so it waits for the owner. Issue 1561, item 19 |
| 16 | minor | 9 | The evidence has no label | Fixed. It has the label "Evidence" |
| 17 | minor | 1 | On a phone the facts line wraps with a hanging dot | Fixed in part. The dot follows its word. Issue 1566, item 6 |
| 18 | minor | 3 | On a phone one row of buttons has two heights | Fixed. Every button of a request is 36 pixels tall on a phone |
| 19 | minor | 4 | On a phone the table is the desktop table squeezed | Fixed by the stacking. A row is 54 pixels and opens the task |

The reviewer also noted that the design document gives two heights for a table row. That is
issue 1565, item 1.

After the fixes the space checks, the build and the 795 tests were run again. At 1440 the
space checks find nothing, with demo data and with worst-case data. No second blind review
was run.

## Second round of fixes

The owner asked on 2026-11-22 for all the issues to be addressed. This round fixed the
items of issues 1561, 1565 and 1566 that have one right answer.

- One place moves the sprint. The Status fact is a plain value.
- The open tab is in the address.
- A menu moves the focus with the arrow keys. The page behind a dialog is inert. Each
  request button is described by its request. The first Tab stop is "Skip to content".
- The four sprints that are past Planned have an approved sample plan.
- The sample plans use the type scale.

The owner then had the coloured left edge removed (1561, item 4), and had the owner put
before the status on a stacked task row, so both start at one edge down the list.

Four items wait for the owner: the edge of a button
and the dark-mode colours (1561, item 9, and 1566, item 2), the locale of dates (1561, item
12), the task a request blocks (1561, item 19), and the phone's first screen (1561, item 2).
The owner closed the plan's Goal section (1566, item 4) on 2026-11-22: the goal in the rail
is the summary of the plan.

## The question tab was taken out

The owner proposed on 2026-11-22 a tab that holds one question with all its details. A first
prototype put the tab on this page. The owner answered the same day that the tab belongs
beside Board, in the project's row of views. The tab is gone from this page. The Questions
view replaces it, and its hand-over file is `design/builds/questions-view.md`.

A question's card on the Requests tab has Accept, Deny and Details. Details opens the
question in the Questions view. An approval's card is unchanged.
