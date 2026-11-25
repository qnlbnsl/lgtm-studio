# The row of views, with the stage at its right end

This screen is in `design/mock-vite/` only since 2026-11-23. It was built in `design/mock/`, and that folder went back to the state of `master`, because the changes reached the live website and broke one of its journeys. That is issue 1574. Where the text below says `design/mock/`, read the file of the same name under `design/mock-vite/src/`.

Changed on 2026-11-22 in `design/mock/`, on sample data. It is not wired to the engine.

surface: app screen. The row sits above every view of a project.

reference: none was searched. The change answers the blind review of the two bars, which is
issue 1569 in `docs/issues.md`.

## What the owner decided

- The stage line stops being a row of its own.
- The current stage is one word at the right end of the row of views.
- A click on it opens the list of all stages.

## Plan

First viewport: 1440 by 900.

| # | Region | Job | Container | Why this container | Rows visible in the first viewport | Empty state |
|---|---|---|---|---|---|---|
| 1 | Row of views | Choose a view of the project | none | One line of tabs | 1 | none |
| 2 | Right end of the row | Say what waits, show or hide the view's list, say the stage | none | Three controls on the row's own line, with the stage last | 1 | The count is absent when nothing waits. The list button is absent on a view with no list |
| 3 | List of stages | Show the six stages and which one is current | list | Six like items in order | 6 | none |

Main offset at 1440: the view starts at 82 pixels. It started at 118 with the stage line.

Navigation: unchanged.

Parts:

- List of stages: the mock's own `Menu` in `design/mock/js/ui.js`, with the list in the
  menu's head and no items. The mock has no part for a plain pop-up. This is a fault for a
  screen reader, and it is recorded in issue 1569.

Motion: none beyond the menu's own.

- #2 scan: edges the row's right padding; groups by none; visuals a caret after the stage;
  coloured the count of what waits, because it is what the owner must act on.
- #3 scan: edges the menu's padding; groups by none; visuals a check for a finished stage, a
  filled dot for the current one and an empty circle for a later one; coloured the check and
  the dot.

## Captured

1440 by 900 on Board, Git and Questions, with the list of stages open, in dark mode, with
worst-case data, and in three other projects (Plan, Design, and one with a single request).
Also 1920, 1280, 1101, 1100 and 390 wide.

## What was measured

- The stage button spans 1324 to 1428 pixels on Board, Git and Questions at 1440. It does
  not move between views, at 1440, 1100 or 1000.
- The row does not scroll at 1280. At 1100 it scrolls by 5 pixels, and at 1000 by 105.
- The view starts at 82 pixels at every width. No line stands under the row of views.
- At 1100, 1000, 900 and 390 the conversation button shows the conversation, the chosen tab
  shows the view again, and another tab shows its view.
- At 1000 the chosen tab is in sight on End result, Board, Git and Settings.
- Escape closes the list and returns the focus to the stage button.

## The narrow screen

The owner's window is 1100 pixels or narrower. The first version kept a line under the row
there, for the stage and for two buttons that switched between the conversation and the
view. The owner saw the stage on that third line. The line is gone. The conversation
button at the left end of the row now does the switching, and both ends of the row stay
in sight while the tabs scroll between them.

## Known costs

- Below 1280 pixels the label "Stage" and the count leave the row.
- Below about 1140 pixels some tabs of a project with ten views scroll out of sight.
- On a narrow screen the switch to the conversation is an icon with no word.
- On a phone the stage is not shown.
- The stages are no longer all visible at a glance. They are one click away.

## Not checked

- The live website. Its project page does not use this shell, so it never had the stage line.
- Firefox and Safari. Only Chromium ran.
- A real screen reader.
- No blind review was run on the changed row.
