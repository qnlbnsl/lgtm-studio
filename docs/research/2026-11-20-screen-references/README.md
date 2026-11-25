# Screen references: how LGTM Studio and other products do each screen

This folder holds one research file for each screen of the LGTM Studio website.
The owner asked for it on 2026-11-20, so that a design discussion about any screen starts from prepared material.
The research was done on 2026-11-20 by five researchers, from the mock in `design/mock/` and from Mobbin, a library of screens of shipped products.

## How to use this folder

Open the file of the screen under discussion.
Each file has the same six parts, in this order.

1. What LGTM Studio has today, read from the mock's code.
2. What is planned and not built, read from `docs/features.md` and `design/DESIGN.md`.
3. How other products do it, each with a link to the Mobbin screen and a description of what is on it.
4. Options for LGTM Studio, as a numbered menu. The options may contradict each other.
5. Ideas that are not in the feature list.
6. The searches that were run, and which were weak.

A product is described only from a screen that the researcher looked at.
A Mobbin link opens the full screen.
The files describe the mock as it was on 2026-11-20, so check the code before relying on a detail.

## The files, by group

### The shell and navigation

| File | Screen | The most useful reference |
|---|---|---|
| [sidebar.md](sidebar.md) | The tree of repositories, projects, teams and agents | Cursor lists agents flat and offers a "Group" choice: Project, Date, Status or None. |
| [top-bar.md](top-bar.md) | Breadcrumbs, the "Inspecting" switcher, the bell, the tab strip | Supabase makes every breadcrumb level a switcher and puts the branch in the breadcrumb. |
| [home.md](home.md) | Home and All projects | Sentry's issue table has token search, saved views with counts, and an "Undo" notice. |
| [command-switcher.md](command-switcher.md) | "Find project or agent" and keyboard shortcuts | Vercel ends its switcher with a create action. |
| [new-project.md](new-project.md) | New project and Add repository | Cursor, ChatGPT Codex, Claude and Devin start work from a prompt box that is always on screen. |
| [login-and-first-run.md](login-and-first-run.md) | Login and the first run | Devin keeps a "4 of 6" setup checklist inside the product. |

### The conversation screens

| File | Screen | The most useful reference |
|---|---|---|
| [project-view.md](project-view.md) | The lead's conversation beside the end result | Elicit gives each finished stage a "Details" button that opens what the stage produced. |
| [decisions-and-approvals.md](decisions-and-approvals.md) | Docked decisions, questions, approvals | Claude docks one question in place of the message box, with a "1 of 2" pager and "Skip". |
| [chat-thread.md](chat-thread.md) | A thread with one agent | Cursor ends a turn with one "Worked for 37s" line and a "3 Files Changed" box. |
| [message-box.md](message-box.md) | The message box and its pickers | ChatGPT, Cursor, Emergent and Gumloop each put a microphone on the box. |
| [messages-board.md](messages-board.md) | The agents' message board | Plain sorts threads by who must act next. |
| [agents-roster.md](agents-roster.md) | The roster of agents | Cursor's list says what each agent produced, such as "+17 −0" and "Merged". |

### Planning and workflow

| File | Screen | The most useful reference |
|---|---|---|
| [project-board.md](project-board.md) | The board as columns, a table and a roadmap | Linear folds unused statuses into one narrow "Hidden columns" column. |
| [sprint-and-task-pages.md](sprint-and-task-pages.md) | The sprint, task and team pages | Linear shows each milestone as a row that opens in place, with a count and a percentage. |
| [workflow-tab.md](workflow-tab.md) | The build loop's phases | Vercel shows stages as foldable rows with a duration, and lifts the failing line to the top. |
| [workflows-editor.md](workflows-editor.md) | The Workflows page and its editor | Zapier edits the selected step in a right panel and adds a step on the line where it belongs. |
| [review-and-test-results.md](review-and-test-results.md) | Review rounds and test runs | GitHub Actions gathers every error of a run into one Annotations list. |

### The developer tools

| File | Screen | The most useful reference |
|---|---|---|
| [terminal.md](terminal.md) | Agent terminals and owner shells | Devin shows an agent's shells as a record, with a scrubber and a searchable command history. |
| [files.md](files.md) | The folder tree and the small editor | GitLab puts the commit message, the target branch and a diff preview on the editor's page. |
| [git.md](git.md) | Changes, staging, the diff, commit and pull request text | GitHub gives each file a "Viewed" checkbox and shows "1 / 2 files viewed". |
| [mocks-tab.md](mocks-tab.md) | The designed screens of a project | Ditto keeps the screens as a thumbnail column beside the open one, with a comments panel. |
| [repository-design-page.md](repository-design-page.md) | The design file on a canvas | MagicPath says on each token where it is used. |

### The service and settings pages

| File | Screen | The most useful reference |
|---|---|---|
| [machines.md](machines.md) | The computers that run agents | Tailscale filters its machine list with property chips. |
| [harnesses-and-accounts.md](harnesses-and-accounts.md) | Harnesses, accounts and limits | Braintrust shows each provider as one row with "Configured" or "Not configured". |
| [usage.md](usage.md) | The draft Usage page | Claude shows each limit as its own row with a reset time and "Last updated". |
| [notifications.md](notifications.md) | Delivery kinds and devices | Linear lists delivery kinds as one-line rows and keeps each form behind its row. |
| [service-and-project-settings.md](service-and-project-settings.md) | Service settings and one project's settings | Cursor joins model and effort in one option, such as "GPT-5.4 High". |

## Which products Mobbin has

A named search returned screens of these products: Linear, GitHub, GitLab, Cursor, Claude, ChatGPT, Slack, Vercel, Sentry, Jira, n8n, Zapier, Replit, Figma, Tailscale and the OpenAI Platform.
Devin, v0 and Lovable returned nothing by name, and their screens appeared in other searches.
A named search returned nothing for Warp, Storybook, Zeroheight and CircleCI.
Notion, Zed, Datadog, Retool, Railway and Codex were not searched by name.

Cursor's screens on Mobbin show its web agents product, not the desktop editor.

## What no reference was found for

These parts of LGTM Studio matched no screen, so their options follow no reference.

- An approval of one command inside a conversation, with "Allow once" and "Always allow".
- A context meter in a message box.
- Posts between agents that carry a kind, such as Question or Decision.
- Agents grouped by situation, with a reason on each row.
- An "agree" button on a plan document.
- Graded review rounds, and a test marked flaky.
- A staged and unstaged file list with a commit box, a merge conflict, and a list of worktrees.
- A usage finding with a cause and a proposed guardrail, and a breakdown that shows a share of the row above.
- Installing a command-line program from the browser.
- A choice of permission modes for an agent.

## What the research found wrong in the mock and the documents

The researchers found places where `design/DESIGN.md`, `docs/features.md` and the mock's code disagree.
Each file lists them under "What is planned and not built".
`docs/issues.md` records them as issues 1547 to 1551.

## Limits of this research

Each researcher had a budget of 14 searches, so some screens had two searches and some had three.
Screen descriptions come from Mobbin's preview images, so small text may have been missed.
The designer read one file in full and checked eight of the code claims; the rest was not re-checked.
