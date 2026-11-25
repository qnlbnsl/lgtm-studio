# References for a command center

The owner asked on 2026-11-23 for two command centers: one for a project, and one for a repository.
The owner asked for a few variants, drawn from Mobbin.
Mobbin is a library of screens of shipped products.
This file holds what was found, the patterns the screens fall into, and how the patterns became the layouts in `src/frontend/src/screens/center.tsx`.
A helper agent ran ten searches for web screens and opened every picture.
A product is described only from a screen that was looked at.
Only structure is taken from a reference. No artwork and no wording is copied.

## The screens

| Product and screen | What is worth taking |
|---|---|
| [Vercel, project overview](https://mobbin.com/screens/d8e63884-5d43-4337-bf31-5667bc4ba5db?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | One wide panel on top says the current state, with its last change and its actions. A row of three small cards with small lines follows, and then a list. |
| [Linear, project overview](https://mobbin.com/screens/186d053a-bfff-4742-8333-06cd35a58e5b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | The main column holds the latest status update with what changed since the one before. A column on the right stacks the properties, the milestones with a percentage each, and one chart. |
| [incident.io, team overview](https://mobbin.com/screens/cf752001-85c3-4e42-9166-5e925845b7aa?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | Open items are grouped by stage, with counts in the head of each group. Small cards on the right say who is on duty. |
| [Deel, home](https://mobbin.com/screens/6bd4989c-7a50-413e-be7e-e955b2394a29?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | A list of what is for the person today. Each line has a count and one button. |
| [7shifts, home](https://mobbin.com/screens/cc7d3fa8-66ee-45cb-9332-e7e557975c70?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | Requests that wait, with Approve and Decline on each row, beside a log of what happened. |
| [Cursor, agents home](https://mobbin.com/screens/ef7c23a0-61cc-4d41-8988-7c3d84610f2f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | Each agent run is one row: a state, the size of its change, its model, its repository and its age. A filter lists Running, Finished, Error and Merged. |
| [ClickUp, team view](https://mobbin.com/screens/4ed5d2cd-edde-4144-915a-ba0b7f5ddf6e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | One card for each worker, with counts of done and not done, and a ring. |
| [ClickUp, space overview](https://mobbin.com/screens/bef6d943-1480-47ae-ae8c-07522286788a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | A widget of rows, each with a health cell and a progress bar. Its head has "Customize" and "Add card". |
| [Jira, space summary](https://mobbin.com/screens/bb16188e-49c8-4276-838d-609c4329f440?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | Four figures that each say their time window, over a grid of two columns with a feed and bar charts. |
| [Greptile, code review analytics](https://mobbin.com/screens/0579d5ea-23ce-449f-aa9e-70ad8d4fd23d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | One bar of filters sets the scope of everything. A strip of four numbers, then charts in two columns with a ranked list under each. |
| [Adaline, monitoring overview](https://mobbin.com/screens/7483692e-1571-40a7-829d-468a0686e2d9?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | The time window is the headline. Six small charts each carry their number in their title, with no frames. It is very dense. |
| [Vercel, usage](https://mobbin.com/screens/e828f30f-3564-432d-8a2f-33b3116dd465?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | A grouped table: each row is a name, a small line, a quantity and a cost. |
| [Railway, observability](https://mobbin.com/screens/3af70f9f-c560-4a42-a15c-cbd12db09c73?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | Blocks that the person adds, with one time range for all of them. |
| [Twenty, dashboard in edit mode](https://mobbin.com/screens/355386b9-cce5-4aae-a727-2102180e2907?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp) | In edit mode the grid shows its cells. An empty tile invites a widget, and a panel on the right chooses its kind. |

## The five patterns

| Pattern | Seen in | Strength | Weakness |
|---|---|---|---|
| A. What waits for the person comes first, with its button on the row | Deel, 7shifts, incident.io | The skim ends in a decision | The top of the page is empty when nothing waits |
| B. One wide panel says the current state, with small cards under it | Vercel, Linear | One answer at a glance | The panel takes its height even when nothing changed |
| C. A strip of numbers over a grid of charts | Jira, Greptile, Adaline, Railway | It fits analytics, and each chart is a widget | Numbers move little between visits, and nothing says what to do |
| D. One row for each worker, with its state | Cursor, ClickUp | It answers "what are the agents doing" | It grows into a list to read |
| E. A free grid of widgets, with an edit mode | Twenty, ClickUp, Railway | The person fits it to their habits | A poor default is common, so a good fixed layout must come first |

## How the patterns became layouts

Three layouts of each command center were drawn. The owner chose "Grid" for both on 2026-11-24, and the other four were removed.

| Command center | Layout | Patterns |
|---|---|---|
| Project | Queue | A and D |
| Project | State | B |
| Project | Grid | E, with the widgets of C |
| Repository | Portfolio | A list of rows with a health cell, as in ClickUp's space overview |
| Repository | Numbers | C |
| Repository | Grid | E |

## Searches that found little

- A live list of running jobs: the results were tables of past runs.
- Insights of a code repository: one result had two bare bar charts.
- The home of a product for coding agents: most open on a box to type into. None showed many agents at once.
- A wall of live status tiles: nothing close.

Mobbin showed a notice that this month's usage is high. Usage has no limit during its beta.
