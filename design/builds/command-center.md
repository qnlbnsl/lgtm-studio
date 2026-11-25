# The command center of a project and of a repository

surface:    app screen
reference:  Mobbin, searched on 2026-11-23. The screens and the five patterns are in `docs/research/2026-11-23-command-center-references.md`. Taken from them: a grid of widgets with a menu that shows and hides them (Twenty, ClickUp, Railway), a strip of figures that each say their window (Jira), and a table of projects with a health cell (ClickUp).
state:      the owner chose the grid for both on 2026-11-24, out of three layouts of each. The other four were removed.

## The plan

| # | Widget | Command center | Container | Columns | Rows in the first viewport | Empty state |
|---|---|---|---|---|---|---|
| 1 | Needs you | project | list | 2 | up to 6, then "more" | "Nothing waits for you." |
| 2 | Progress | project | panel | 1 | 8 segments and 2 pairs | "None yet", "Not written" |
| 3 | Working now | project | list | 2 | every working agent | "Nobody is working." |
| 4 | Checks | project | list | 1 | 2 | "No run yet", "No review yet" |
| 5 | Sprints under way | project | list | 2 | every sprint in progress or in review | "No sprint is under way." |
| 6 | Tasks finished | project | panel | 1 | 1 figure and 14 bars | flat bars |
| 7 | Usage | project | panel | 1 | 1 figure, 1 line, a meter for each subscription | none |
| 8 | Latest commits | project | list | 2 | 5 | "No commits yet." |
| 9 | Projects | repository | table | 2 | every project | "No projects yet." |
| 10 | Needs you | repository | list | 2 | up to 6, then "more" | "Nothing waits for you in the repository." |
| 11 | Plan | repository | list | 2 | 8 goals, then "Show all" | "No goals yet." |
| 12 | Sprints merged | repository | panel | 1 | 1 figure and 8 bars | flat bars |
| 13 | Full runs | repository | panel | 1 | 1 figure and 12 bars | none |
| 14 | Usage by project | repository | panel | 1 | 1 figure and a bar for each project | none |
| 15 | Base branch | repository | panel | 1 | 2 pairs | "None" |
| 16 | Checkouts | repository | panel | 2 | the remote and a pair for each machine | "None" |

main offset: 0. The grid starts at the top of the pane. The "Widgets" menu stands in the row of views.
navigation: none of its own. The command center is the first of the project's views.

parts:      the "Widgets" menu is the website's own `Menu`. The bars, the small line and the share bars are drawn by hand in `center.tsx`, because the website has no chart library and each is under twenty lines.
motion:     none.
feedback:   hiding a widget removes its tile at once, and the menu's count drops by one.
captured:   1440 by 900 in the dark theme with the conversation open and closed, and 1000 by 680 in the light theme, on sample data.
worst case: `?data=worst` before the `#`. It gives a title of 120 characters with a word that cannot break, a file path of 100 characters, a project name of 80 characters, and a plan of 18 goals. Text wraps and nothing is cut. The plan broke the page's length, so it now shows 8 goals and a "Show all" button. The sidebar cuts the long project name, which is point 9 of issue 1621.
gate:       the eight space checks at both sizes, on sample data and on worst-case data. No finding inside either command center. One waiver: the word "Widgets" is hidden up to 1100 pixels and kept for a screen reader, and the check reads a hidden word as cut text.
access:     the scripted checks (axe, the Tab walk, text spacing, reflow) report nothing inside either command center. The ten findings left on each page are in the row of views, the composer and the sidebar, and they are issue 1619.
walked:     flows F-28 to F-31 in `design/flows.md`.

## The blind review

A fresh reviewer read six pictures, the design contract and the reference record, without the plan and without the source.
It would not ship the first build. Its grades were: layout holds, hierarchy weak, spacing holds, parts weak, colour weak, access fails on 1.4.1.

| # | Finding | Label | Widget | What was done |
|---|---|---|---|---|
| 1 | Amber and green dots on the rows with no word | major | Working now, Checks | The rows of "Working now" have no state dot. The review's row says "6 points open". |
| 2 | Three tabs were out of sight at 1000 pixels | major | the row of views | Up to 1100 pixels the tab and the menu show their icons alone. The row fits at 1000. |
| 3 | "Now" added up to 7 and "Needs you" said 5 | major | Projects, Needs you | "Needs you" lists the open questions of Define too. Both say 7. |
| 4 | The bars were the largest block of colour | major | Tasks finished, Sprints merged | Bars are 10 pixels wide and 40 high. The head of a widget that waits is brighter. |
| 5 | "Last 7 days" stood over 14 bars, with no axis | major | Tasks finished, Sprints merged | The bars of the earlier window are paler, and the two ends of the axis are named. |
| 6 | "Checks" had no link, and "50 min" did not say what | minor | Checks | Its name leads to Actions. The age says "50 min ago". |
| 7 | Two charts stood above the sprints and the fold cut them | minor | the order | The sprints stand before the charts. A repository shows what waits second. |
| 8 | "41%" did not say of what | minor | Usage | The meters have the label "Used of the limit". The contract now says the figure is a total. |
| 9 | A link had two colours in the dark theme | minor | Plan, Progress | Every link in a widget takes the website's link colour. |
| 10 | The current phase was a faded segment, and empty tracks were faint | minor | Progress | The current phase is filled to its middle. Empty segments use the stronger line colour. The track of a meter is the website's own and was left. |
| 11 | The plan said its count twice | minor | Plan | The badge is gone. |
| 12 | "New project" and "Make a project" on one screen | minor | Projects, Plan | Both say "New project". |
| 13 | Tiles stretched to a neighbour are half empty | minor | several | Left. With every widget shown the grid has no empty cell at two or three columns. |
| 14 | Commits and the base branch are Git's content | minor | Latest commits, Base branch | Kept. The contract now says a widget may say one fact of a view and lead to it. |
| 15 | "About" was a paragraph, not a status | minor | About | It is now "Checkouts": the remote and the folder on each machine. |

not checked: a second blind review after the fixes. The worst-case pictures, which the reviewer did not get: its copies were the plain pictures, because the first worst-case data touched nothing the command centers show. I judged the new ones myself. The contrast of the phase segments and of the meter's track, which the reviewer asked to have measured. The empty, loading and error states. The open menu, a hovered row and keyboard focus in a picture. 320 pixels and a phone. A screen reader. The live website, which keeps the board as a project's first view and the older page of a repository. Tests, of which none were written.
