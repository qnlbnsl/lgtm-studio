# LGTM Studio’s approved palette pair

The owner selected Field for light mode and Dusk for dark mode on
2026-11-05. The two palettes give the same app a brighter daytime surface
and a muted dark surface. Harbor is no longer an active option.

## What is approved

`tokens.css` is the color contract for future LGTM Studio app mockups.
The mock in `mock/` loads the same tokens.
Keep those blocks identical when the tokens change.

The layout, typography, sample roster, and product flows remain exploratory.
The owner's palette selection does not approve those other choices.

Cursor is the primary design reference. T3 Code is a secondary usability
reference, with the owner's discoverability concerns treated as a constraint.
Repositories contain projects. Each project has exactly one lead, a sprint
board, and teams. Selecting a project opens its board with a lead chat
panel on the right. The repository has a page listing its projects. Branch and workspace inspection
has its own context menu and cannot rebind the project repository.

The chat uses one compact header, a restrained user turn, plain agent
replies, grouped tool activity, and a growing composer. Sprint cards are
used for the Kanban board requested by the owner. Sprint and task details
open as pages. The lead panel contains the real conversation and its
composer. It does not repeat project summary cards. Field and Dusk remain unchanged. The owner approved the critique-driven refinement pass on 2026-11-06.

## The board and its lead conversation

Selecting a project opens the board. There is no separate Board row in the
navigation tree. At desktop widths, a right sidebar shows Sage's existing
conversation and composer beside the sprints. Other projects show their
own single lead. Drafts, messages, pending replies, and reading positions
stay with that conversation when it is opened full size.

The lead button at the right end of the tab row opens and closes the panel.
On a desktop it is the only control that does, so the panel has no close
button and no expand button; the Chat tab opens the full conversation. At
widths of 1100 pixels or less the panel starts closed, and the lead button
opens it as a drawer over the board. The drawer keeps its close button,
because it covers the lead button. The drawer contains keyboard focus,
Escape closes it, and focus returns to the lead button. The board retains
its own scroll position while the conversation scrolls.

## Preserve the existing page’s operational controls

The redesign retains authentication states, connection status, permission
requests, agent stop/resume, safe failed-start reasons, explicit large-item
loading, tool calls, delivery history, Notifications, and Settings. Permission
controls show the actual action input before answer buttons. Hidden input
and unconfirmed answers have explicit states. Stop cancels permissions.
Logout and expiry wipe entered secrets. Reconnecting preserves reading
context but disables mutations. These remain sample flows in the mock.

Notifications, Machines, Subscriptions, and Service settings belong to
the service, so they sit in global navigation, as a row of icons at the foot of
the sidebar. The project workspace has Board, Chat, Terminal, Git, and Project
settings tabs, and a team adds Tasks. Git combines the repository worktree hierarchy, staged and unstaged files,
commit messages, synchronization, direct file diffs, and integration controls.
History uses that same diff pane. Workspace Settings shows the assigned
repository, branch, checkout folder, and machine, plus a per-project shell
preference. Service settings remain in global navigation. Terminal replaces the main chat. Machines describes one
master and its paired workers. Project placement and branch inspection are
separate. Subscriptions uses independently named accounts; one harness can
have several accounts on a machine. Additional Codex sign-ins stay private.

Terminal execution, Git mutations, remote pairing, project movement, and provider sign-in
are not wired to live services. This boundary is documented here rather
than repeated in product copy, at the owner’s request on 2026-11-06.

## Project branch placement

The branch shows only in the two views that show code: Files and Git.
Each names the worktree at the top of its own pane, and that name is the
control that changes it. The two views share the choice.
The top bar had a control that said Inspecting until 2026-11-23. The owner
asked what it did on a view that shows no code, and it was removed.
The sidebar, the board header, and the composer do not repeat the branch.
A long branch name is cut at its start, so the end, which differs between
branches, stays visible. Hover text gives the full name. Choosing another
worktree does not move the work: the list marks the assigned worktree as
Assigned.
A new project still starts with its lead to shape the first plan.
Create a worktree is checked beneath its name by default. A safe version of
the project name becomes the branch and worktree folder. Existing names get
a numeric suffix. Unchecking uses the repository’s main checkout. The dialog
previews the exact branch and folder on two labelled lines, Branch and Folder,
before it creates the project.

## How the modes behave

Field is the default token block. Dusk overrides it under
`:root[data-theme="dark"]`. Both modes share spacing, type roles, shape,
and motion values. A first visit follows the device's color mode.
The choice lives in Service settings, under Appearance, as Light, Dark, and
Match system. The labels do not name the Field and Dusk palettes. A manual
light or dark choice takes precedence and is remembered when browser storage
is available. Match system forgets that choice, and the page follows the
device again.

The prototype migrates a previously selected Field or Dusk candidate to
the matching mode. A previous Harbor selection falls back to the device
setting.

## What the colors mean

| Role | Field light mode | Dusk dark mode |
|---|---|---|
| Workspace | Green-tinted white. | Muted plum. |
| Navigation | Soft green. | Deeper plum. |
| Surface and elevation | White and pale green. | Progressively lighter plum. |
| Primary action | Deep green with white text. | Peach with dark plum text. |
| Body and secondary text | Dark green and readable grey-green. | Pale text and readable muted pink-grey. |
| Progress | Deep green. | Soft green. |
| Needs owner attention | Dark amber. | Pale amber. |
| Error | Dark red. | Soft red-pink. |

## Type and reading

The mock uses Inter for interface text and JetBrains Mono for code,
paths, and branch names. Both are served from the repository, as the
"Fonts" section below says. Keep long replies within roughly 68 characters
per line, with enough line spacing to follow paragraphs.

The current proposal uses a shared frame for the compact conversation
header, chat, and composer. That frame stays centered within the available workspace on wide
screens. Message text has a separate 70-character reading measure. Boards
and plan details use a wider frame. Titles,
authors, replies, and metadata have distinct sizes. The composer's label and
placeholder name its recipient. While a draft is typed, the line under the tabs
or the lead panel's header still names the agent.

## Interaction rules

- Use semantic tokens for every interface color.
- Keep the same layout and state meanings across both modes.
- Pair status colors with readable labels.
- Preserve visible keyboard focus and reduced-motion support.
- Require at least 4.5:1 contrast for normal text and action labels.
- Show the owner why a decision needs their attention and what each action does.
- Use normal product copy in the mock. Keep prototype limitations in
  the design documentation rather than repeated labels or disclaimers.
- Separate chevron disclosure from repository, project, and team selection.
- Preserve each conversation's draft and reading position while navigating.
- Restore the last tab when returning to a team.
- Surface a team request in the project lead conversation and sidebar.
- Link every request entry to the requesting agent's exact question.
- Clear attention only when the request is answered.
- Keep a task's assigned workspace separate from the viewed code context.
- Keep sprint completion evidence separate from release status.
- Connect assignments to their agent, review handoff, and changed files.
- Keep mobile navigation focus contained while open and return focus to
  the menu or newly selected view when it closes.

## Visual constraints

Keep the layered, hue-tinted dark surfaces. Do not replace Dusk with flat
black and a bright accent. Do not introduce gradient text, decorative
metrics, or a marketing hero into the working app. Use actual app flows
to judge the palette.

The live visual reference is the mock in `mock/`. No separate brand brief exists
in this design directory; this contract records the owner's choice and
the UI skill's craft rules applied during the workshop.

## Scope and workspace controls

One tab row serves the selected project, team, or agent. Team Tasks remains
available in team scope, as the last tab. Git selects the team worktree when
opened from a team. The top bar names the selected repository, project, team,
and branch. Selecting a
worktree in Git or Terminal follows its owning project and machine. Opening
a project lead restores that project's assigned worktree.

The left navigation and desktop lead panel have drag handles. Arrow keys
resize a focused handle. Home or a double click restores its default size.
The browser remembers these widths. Four board columns fit beside the lead
at desktop width. A phone shows the board as one list instead, with the sprints that need
the owner first.

Git has independent state for each worktree. Staging and unstaging change the
commit selection. A commit records the selected files and message. Fetch,
Pull, and Push expose incoming and outgoing counts. Conflicts require an
explicit resolution before staging. A clean team branch integrates into its
project branch. A clean project branch requires an explicit review before
main. Uncommitted destination changes block integration. All of these are
in-memory interactions, with no repository mutation.

Agent progress and Stop sit above the composer, and Resume replaces its input.
Subscription choices are filtered by harness and machine. Files and tasks can be attached as
context, and sent turns retain those references. Grouped activity links to
its changed files. The bell lists questions, approvals, and
failed starts in one order. Only projects needing you filters projects. Quick switch searches
projects, teams, and agents by name, and opens with Command or Control + K.
Terminal sessions preserve separate input and history in each worktree.

## Home gathers work across projects

Home is the initial page and a permanent navigation destination. It offers
Needs attention, All projects, Questions, and Approvals. Needs attention
gathers unresolved questions, permissions, failed starts, and project merge
reviews. Questions and Approvals keep answered history under a collapsed
Answered section. Search combines with repository, project, agent, and machine
filters behind one Filter button, and All projects adds a status filter. Open
entries sort first, in the bell's order. Filters stay selected on return, and
Clear filters appears while one is set.

A single-line request row expands its details directly underneath. All rows
start collapsed. Selecting the same row closes it. Only one request expands
at a time. The owner can answer questions and
permissions without leaving Home. Merge approval uses the existing explicit
review dialog and records its result in place. Source links open the precise
conversation or project board. Discussion preserves any existing draft.
Request context names the requesting team's worktree where applicable.
Keyboard arrows, Home, and End operate the tabs. Filter changes retain focus.

The row retains keyboard focus when toggled. Its chevron and expanded state
expose the disclosure. The footer of the expanded content names the machine and
the worktree. Narrow screens keep the title and status on the first line and
the kind on the second. On wide
monitors Home stays in a centered frame capped at 1380 pixels.
If an answer removes its request from the queue, focus moves to the next
remaining row. Finishing a filtered queue focuses the empty result. Answered
requests that remain in history keep their inline detail available.

## Sidebar groups controls by purpose

The first section has Home and a bordered Find project or agent search
control. A divider separates Projects, New project, and the explicitly named
Only projects needing you filter from global navigation. Repository names
within that tree open their project lists. The former separate Repositories
destination is removed. Another divider separates service pages from the
project tree, and thin rules separate repository groups. The service pages
are a row of icons above the account. Search remains available with Command or
Control + K.

## Density and machine-aware creation

Use tables for comparable fleet records and compact lists for requests.
Machine rows show identity, status, role, agent slots, project count, and a
New project here action. At 1440 pixels the observed rows are 43 pixels tall.
Project placement is a second compact table. Keep comfortable paragraph
measure in conversations; compact the framing instead of shrinking body text.
The Kanban board retains cards because their grouping communicates sprint state.

Add Project asks Run on before the project name. Only an online machine
with the repository can start work. The preview resolves that host's branch,
checkout type, and folder. Name collisions are checked per repository and
machine. Opting out of a worktree uses that machine's main checkout.

Starting from a machine keeps that host selected. If no repository exists
there, Start remains disabled with a concrete reason. A future placement
change never relabels the current running workspace or implies migration.
Actual repository registration and project transport remain service work.

## Rules added by the review of 2026-11-06

Claude took over this mock from Codex on 2026-11-06. The first pass applied these rules.

### The tab row and page edges stay still

The tab row keeps the same left edge on every project view. The page title shares
that edge. Only the conversation body is centered, so its text keeps a readable
measure. Home is the exception, because it has no tabs and stays in a centered frame.

### Terminal and Git get the whole window

Terminal and Git hide the page title, because the top bar already names the project
and the tab names the tool. Both reach the bottom of the window at any height. A
terminal is a character grid, not a form. It has no input box, no Enter button, and
no placeholder, so the page can host programs such as vim and agent command lines.

Git puts the commit message above the file list, as VS Code does. Each of its three
columns scrolls on its own. Team worktrees show the end of their branch name, because
that is the part that differs. All files stacks every changed file with a sticky name.

### Names

| Name | Means |
|---|---|
| New project | The only name for starting a project, everywhere. Create project confirms it. |
| Git | The tab and the page. |
| Project settings | The per-project tab. Service settings is the global page. |
| Worktree | A checkout on disk. Workspace folder is the one folder that holds repositories. |
| Needs attention | Work waiting for the owner. The bell, Home, and the sidebar counts add up to the same number. |
| Question | A kind of attention: an agent asked something. The short form is Needs answer. |
| Approval | A kind of attention: an agent waits for permission to run a tool. The short form is Needs approval. |
| Merge | A kind of attention: a project branch waits for the owner's merge into main. It was called Approval until 2026-11-08. |
| Failed start | A kind of attention: an agent could not start. It wears the error color. |
| Uncommitted changes | Files changed in a worktree. "Working tree" is not used. |
| Thread | One agent's conversation. Chat stays the name of the tab. |
| Notifications | Notifications sent to the owner's phone. Its badge counts failed notifications only. It was named Alerts until 2026-11-08, which a first reader confused with the bell. |
| Inspecting | The top bar's control for the worktree whose code you look at, until 2026-11-23. The Files view and the Git view have the control now, with no name. |
| Ours | The worktree's own version of a conflicting change. |
| Theirs | The version being merged in: the project branch for a team, or main. |
| Base | The version both sides changed. |
| Create a worktree | The New project check box that gives the project its own branch and folder. It was named Start work tree. |
| Goal | What the owner wants from a new project. The lead turns it into sprints and teams. |
| Lead subscription | The subscription the new project's lead runs on. It was named Lead runs on. |
| Notification service | The ntfy server and topic that carry notifications. It was named Notification carrier. |
| Notification service token | The secret that lets LGTM Studio post to the notification service. It was named Carrier token. |
| Check-in interval, in seconds | How often a connected page checks in with the web server. It was named Heartbeat seconds. |
| Archive | Hides a finished project from the sidebar and Home. All projects still lists it, and Restore brings it back. |

### One density rule for records

Comparable records are tables with rows of 46 pixels or less: Machines, Project
placement, Subscriptions, Notifications, All projects, the repository page, and
tasks. Requests are single-line rows
that say how long they have waited and what they block. At 900 pixels and narrower, a request
row has a second line that starts with its kind. Expanded rows do not repeat
their own title. No text is smaller than 11 pixels, and metadata is 12 pixels.

### Threads open at the latest message

A thread with no saved position opens at its true end, in the lead panel as well as
the Chat tab. Closing and reopening a panel keeps the position. A pending approval is
the last thing in the thread, so its buttons are visible when the thread opens. Away
from the end, a Latest button appears, and it says how many requests are below.

### Fonts

The interface uses Inter, and code uses JetBrains Mono, decided by the designer's
critique of 2026-11-06 and applied on 2026-11-08. They replace `system-ui`, which drew
a wide face on Linux and a different face on every operating system.

- **Both fonts are self-hosted.** Each is a variable font, cut to Google Fonts' Latin
  subset, in `third_party/inter/` and `third_party/jetbrains-mono/`. The mock reaches
  them through symbolic links in `design/fonts/`. There is no request to a font service.
- **The measured cost is 79,688 bytes.** Inter 4.001 is 48,256 bytes, and JetBrains Mono
  2.211 is 31,432 bytes. The critic estimated about 150 kilobytes.
- **Numbers are tabular.** The body turns on Inter's tabular figures, so counts and times
  line up in rows.
- **Code has no ligatures.** Code, diffs, and the terminal turn JetBrains Mono's
  ligatures off, so `=>` stays two characters.

## Rules added by the designer's critique of 2026-11-06

The owner chose the 15 Must fix findings, the two naming findings, and Enter to send.

- **Status colors:** green means running. A failed agent has the error color and a filled
  dot. A stopped agent has a hollow gray dot. Amber means that something waits for the owner.
- **A blind approval looks different.** It has no filled button, and its button says "Allow
  without seeing input".
- **Destructive changes ask first.** Stopping an agent with waiting requests or running work
  asks, and it names what is lost. Changing a machine's role asks, and it names who depends
  on that machine.
- **A merge to main shows its evidence.** The review shows tests, the review result with a
  link to its thread, commits, and every file's diff. Without evidence, the button says
  "Approve without evidence" and is not filled.
- **A split diff lines up.** Rows pair across the sides with blank filler rows, long lines
  wrap, and both sides scroll as one. It needs 900 pixels, the threshold Visual Studio Code
  uses, and below that it reads as unified and says so. At 1440 pixels it hides the worktree
  list to make room, and it says so.
- **A failure offers its fix in place.** Sign-in opens on the right machine, and a refused
  model offers the choice beside the message.
- **Provider sign-in runs the harness's own login** in a terminal session on the account's
  machine, as the design note says. The critic recommended a device code screen, and Claude
  kept the design note's approach, because each harness has its own flow.

## The first run, built from a private app's onboarding rules

A service with no projects opens a first run after login. It has three pages, and each does one
job. The rules come from the onboarding specs and session reports of a private app, read on 2026-11-07.

- **Value comes before any ask.** The first page says what LGTM Studio does and asks for nothing. Its
  three paired promises state only what the product does. The third says "You approve what users
  would notice", not "you approve every merge", because a fix sprint users would not notice merges
  by itself after a full test run.
- **The second page shows what was found and asks nothing.** It lists harnesses, sign-in state,
  and repositories in the workspace folder. It says that LGTM Studio only looked. A harness that is
  installed but signed out offers its sign-in in place, through the usual sign-in dialog.
- **The third page is the first real ask, with its answers filled in.** It starts on the most
  recent repository, a name from that repository, and a new worktree. It hides the machine when
  there is one, and it hides the harness when one is signed in. It is the New project form moved
  into the page, not a second form.
- **Setup hands off to the lead.** Creating the project opens the lead's conversation with its
  greeting and the composer focused. There is no tour and no setup page for the conversation.
- **Look around first is a link, not a step.** It opens the sample, which offers Back to setup.
- **Back never loses the place.** Reopening resumes at the furthest step reached, and a lower step
  never lowers it. A sign-in survives a reload.
- **Every wait has a way out.** The scan offers Back while it runs. A failed scan says why, with
  Try again and Back. With nothing installed, each harness shows its install command, and Check
  again finds what was installed since. An empty folder leads to Change folder or a new repository.
- **Setup is gated by a positive record.** Finishing writes a record, so a person who later has no
  projects does not see setup again. The private app's tour replayed for finished users, because it checked
  for an absent flag.

Login still comes first, because the service has one password from its environment. The first run
does not cover a computer that joins as a worker.

- **Notifications are offered on the first waiting request, not in setup.** The owner chose
  this on 2026-11-08. The reason is plain once an agent waits, and setup stays at three pages. The
  offer sits at the foot of the oldest request that waits for the owner, in the thread and on
  Home. It shows only while no notification service is set.
- **Not now is recorded.** Declining writes a record, so the offer never returns. Service
  settings can still turn notifications on.
- **Set up notifications opens the notification service.** Setting a service ends the offer.
  A missing token is then the Notifications page's job.
- **The catch is the first request.** No phone notification goes out for it, because none is set
  up yet. The owner accepted this.

## Stages, stage teams, and mocks, decided on 2026-11-07

The owner set this direction on 2026-11-07, after looking at how a private app makes its mocks.

- **A project moves through stages.** The stages are Design, Plan, Build, Review, and Hand over.
  Each project board starts with a strip that shows them, and the repository page lists each
  project's stage. A project that draws no screens starts in Plan.
- **Teams are named for their stage.** The names are Design team, Planning team, Build team, and
  Review team. Two teams in one stage add their area after a middle dot, such as Build team ·
  Exports. The earlier names, such as App shell, were chosen one at a time.
- **The Design team makes the mocks.** When the conversation with the lead turns to screens, the
  lead starts a Design team, the Mocks tab appears after Board, and the screens arrive while the
  owner watches. A new project's board stays empty until then, and it points to the mocks.
- **Mocks are the project's own code.** Each screen is a page from the project's code, framed at
  phone or desktop size, with realistic but invented data. The private app's mocks work the same way: its
  screens import the real components, and new parts stay drafts until the owner approves them.
- **Changes go through the AI.** The owner asks the lead for a change while a screen is open. The
  screen changes in place without reloading, and the change is recorded with who made it.
- **The data is drafted with the screens.** The Data pane lists each field the screens use, and it
  marks it as new, matching the schema, or different from it.
- **Approval hands the work to planning.** Approve mocks promotes the draft components, moves the
  project to Plan, and gives a Planning team the tasks: move each draft into the real components,
  change the schema, and wire each screen to real data. Building is not part of design.
- **The design file is the source of truth, and the mocks show exactly what it says.** OpenPencil
  runs only while someone designs or edits. Looking at the mocks needs only the code. Edit design
  and the repository Design tab's Edit mode show OpenPencil's web editor in a frame, open on the
  repository's design file. It runs from `openpencil/` on port 8443 and cannot save into the
  repository yet. When it is not running, the page says so and gives the command that starts it.
- **Converting the design into code belongs to the Build team.** Decided by the owner on
  2026-11-08. Reverse Fig goes one way only, from existing code to the design file. The conversion
  from the design file to code is a separate tool, built later as part of the Build team. The mocks,
  built from that code, then show the result.
- **The repository has a Design tab.** It shows the colors and the components that every project
  in the repository draws from, with live previews. A component that a project's approval promoted
  says which project it came from.

A repository from the owner's other work can appear in the mock without being committed, because
this repository is public. Its rows go in `data/local/`, which git ignores.

## The data behind the mock

The mock draws what stub functions return, and those functions read JSON files shaped like the
engine's events table. [data/README.md](data/README.md) lists the files, the functions, and the
event types the engine does not have yet. Text in a thread comes from stored items, so the page
works out its links: a task identifier links to its task, and a Markdown-style link can name a
task or the sprint board. Open requests appear in the lead's latest turn, and a lead's reaction
to an answer is a new turn, so the earlier text stays as it was written. Team chat is board posts,
which hold only text, so it no longer shows the tool activity it showed when it was hardcoded.

## Visual foundations, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06. Claude applied them on
2026-11-08, after the rename to LGTM Studio. They are the base that later screen work builds on.

### One type scale

- **Five sizes.** Text is 11, 12, 13, 15, or 20 pixels, named `--text-xs` to `--text-xl`.
  The body is 13 pixels. Agent replies are 15 pixels. The login heading is 20 pixels.
- **One exception.** Text fields on phones are 16 pixels, named `--text-input`, because a
  smaller field makes iOS zoom the page.
- **Three weights.** Text is 400, 500, or 600, named `--weight-body`, `--weight-medium`, and
  `--weight-strong`.
- **Page titles are quiet.** A page title is 15 pixels at weight 600. The application
  shell section below says which pages keep one.

### Spacing and shape

- **Spacing sits on a 4 pixel grid.** Padding, margins, and gaps use `--space-1` (4 pixels)
  to `--space-12` (48 pixels), where the number times 4 is the size. One and two pixel
  nudges are allowed for hairlines. Values above 48 pixels are written as multiples of 4.
- **Three radii.** Controls use 4 pixels, panels and dialogs use 8 pixels, and dots and
  pills are fully round.

### Controls

- **Two heights.** Buttons, inputs, and selects are 32 pixels, named `--control`. Dense
  places use 28 pixels, named `--control-dense`: the top bar picker, the Git sync row, table
  rows, agent stop and resume, and the composer's tools. A form uses one height for its
  inputs, selects, and buttons.
- **Phones keep larger targets where they had them.** Icon buttons stay 40 to 44 pixels on
  phones, and the first run's buttons stay 44 pixels.
- **A disabled button is an outline.** It has no fill, and its text uses the muted color,
  which keeps at least 3 to 1 contrast. A disabled action that needs something says what,
  beside the button. Commit says "Add a message and stage a file."
- **Links go somewhere, and buttons change something.** Clear filters, Load full item, Try
  reconnecting (now Reconnect now), Confirm pairing, New project here, Connect subscription, Add task, Check
  again, and Change folder are buttons now. A small action in a row uses the `secondary
  dense` classes.

### Color

- **The primary color has two jobs.** It marks the one primary action in a view, and links
  in body text. The tab underline, the avatar initials, the logo, and save messages no
  longer use it.
- **Amber is for attention only.** The Modified letter in Git is muted. The failed alerts
  count uses the error color, as a failed start does. Stop agent is a neutral outline.
- **The toast is a notice, not a button.** It is a raised neutral surface at the top right,
  below the bell, so it never covers the composer.

### Icons

- **One icon set.** Every icon uses Lucide's paths, credited in `third_party/lucide/`. No text
  character stands in for an icon, so there is no ×, ←, →, ↗, ⌘, ✓, or ＋ in a button.
- **The shortcut matches the system.** The switcher shows the Command icon and K on Apple
  systems, and Ctrl K elsewhere.
- **Notifications use a phone icon,** because they are sent to the owner's phone.

### Motion, focus, tabs, and dialogs

- **Rows move in 120 milliseconds.** An expanded Home row eases in, and an answered request
  folds away instead of vanishing. Reduced motion turns both off. Nothing else animates.
- **Focus rings are not clipped.** The tree's scrolling area has 8 pixels of padding, so a
  disclosure's whole ring shows.
- **Tabs do not move.** Every tab has weight 500, selected or not, so selecting one changes
  no widths. The selected tab has the text color and an underline. Hover changes the color
  without a fill.
- **Every dialog closes the same way.** A Cancel button sits left of the action, and the
  corner close stays. A dialog with nothing to confirm, such as Design notes, has Close.
  The quick switcher is a search box rather than a form, so Escape closes it.
- **Resize handles show a grip on hover.** The target is 8 pixels wide. Hovering or
  focusing it shows a line, a grip, and the resize cursor.

### Login

- **The page names the product.** The logo mark sits above the heading "Sign in to
  LGTM Studio", which suits a first visit and a return alike. The button says Sign in.
- **A password manager can fill the field.** The field has `autocomplete="current-password"`.
- **Messages show their severity under the field.** A rejected password uses the error
  color. An expired session uses the muted color. On the unavailable page, Try again is the
  primary button, because it is the only thing to do.

## The application shell, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06. Claude applied them on
2026-11-08. They cover the sidebar, the top bar, the tab row, and the two switchers.

### The sidebar folds into a rail

- **Control + B folds the sidebar.** On Apple systems it is Command + B. The button at the
  top of the sidebar does the same, and its hover text names the keys.
- **The rail is 48 pixels wide.** It keeps Home, Find, New project and the service pages
  as icons. Each icon names itself on hover. The project tree
  hides until the sidebar opens again.
- **The browser remembers the choice.** A phone keeps its drawer, and the drawer has no rail.
- **The terminal keeps Control + B.** Programs such as tmux and the shell use it, so the
  shortcut does nothing while the terminal has focus.

### Sidebar rows

- **Every row is one line of 28 pixels on a desktop.** Phones keep their 40 to 46 pixel
  targets.
- **Only the selected project has a second line.** It names the project's lead. The branch
  is in the top bar, so the tree does not repeat it.
- **Only the current row is filled.** Its project and team above it are bold, without a
  fill. A selected repository fills its whole row too.
- **A row is one hover target.** Its disclosure and its name share one surface, so no seam
  shows between them.
- **A project with no teams has no disclosure.** It draws no tree line either.
- **The current row stays clear of the fade.** After each navigation, the tree scrolls so that
  the current row sits 8 pixels above the fade at its foot.
- **Service pages are one row of 40 pixel icons.** Notifications, Machines,
  Subscriptions, and Service settings sit in that row, with Design notes at its end. A
  red count on the phone icon says how many notifications failed.

### The top bar and the titles

- **Each place in the top bar opens.** The repository opens its projects, the project opens
  its board, and the team opens the team.
- **A page does not repeat its place.** The top bar names the place, and the selected tab
  names the view. Home, Chat, team Tasks, Terminal, Git, Project settings, and the service
  pages show no title. The board, Mocks, and the repository page keep their summary line
  without a title. Sprints and tasks keep their own titles, because nothing else names them.
  Each page still has a heading for screen readers.
- **Home keeps New project in its tab row.** The sidebar's New project is the same action.
- **Chat has one line of 32 pixels under the tabs.** It names the agent, its role, and its
  status. A team's chat says how many agents share it.

### The tab row

- **Tasks is the last tab.** The order is Board, Mocks when the project has them, Chat,
  Terminal, Git, Project settings, and then Tasks for a team. A team's Tasks tab therefore
  moves no other tab. The critic suggested Tasks after Git with Project settings pinned to the
  right end. Claude kept Project settings in the row instead, because the lead panel narrows
  the board, so a tab pinned to the right would move between the board and the other views.

### The two switchers

- **Inspecting lists worktrees as Git does.** Main checkouts come first, then each project,
  with its teams indented beneath it. Each worktree is one line of 32 pixels with its name,
  its branch, and its machine at the right. A search field sits at the top. The list has no
  footer.
- **The quick switcher matches names only.** A name that starts with the query comes first,
  then a name with a word that starts with it, then any other name that contains it. Rows are
  36 pixels and show the attention count. The switcher has no title and no close button.
  Escape closes it.
- **Design notes says what it is.** Its previews are grouped as Session, Connection, and
  Requests.

## Connection, the bell, and empty states, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06. Claude applied them on
2026-11-08.

### The connection lives in the Live indicator

- **Nothing is inserted into the layout.** The top bar's Live indicator carries the state.
  The connection banner is gone, so a lost connection moves no content and hides no button.
- **There are four states.** Live has a green dot. Connecting and Reconnecting have an amber
  dot. Offline has a red dot.
- **Reconnecting turns into Offline after 30 seconds.** Both states say when the page last
  heard from the service, such as "Last updated 2 minutes ago". The time keeps counting.
- **Reconnect now is a small button.** It sits beside the indicator, with a refresh icon and
  no arrow. On a phone the indicator and the button move into the bell, as the phone rules
  below say.
- **Data stays readable, and actions wait.** Threads, lists, and links keep working. Every
  control that changes state is disabled, and its hover text gives the reason, such as
  "LGTM Studio is offline. This works again when the connection returns." Drafts can still be
  typed. Starting a project, a new terminal, and refreshing notifications also wait.
- **The bell's count is muted while stale.** Its hover text says that the count is from the
  last update.
- **The terminal keeps its grid.** Its notice, such as "Reconnecting. Input is paused.",
  sits in the terminal's own bar, so the rows and columns do not change.

### The bell is a short copy of Home

- **The bell uses Home's kinds.** They are Question, Approval, Merge, and Failed start.
- **The bell's order is shared.** Failed starts come first, then the request that has waited
  longest. Home uses the same order.
- **Rows are 44 pixels.** Each row has the request, its age at the right, and a second line
  with the kind, the agent, and the project.
- **The bell shows at most six rows.** "View all 10" follows them and opens Needs attention
  on Home. An empty bell says "Nothing needs your attention." and has no link.

### Empty states

- **One short sentence and at most one action.** Examples are "No sprints until you approve the mocks." with
  Open the mocks, and "No matching requests." with Clear filters.
- **An empty list is not shown.** A table with no rows becomes its sentence. A sprint shows
  Completion evidence only once it has evidence. Only projects needing you says "Nothing
  needs you." when no project does.
- **Home's empty queue keeps its line about running work.** "Nothing needs you" is followed
  by how many agents are working, linked to All projects, as the Home finding asked.

## Home and its requests, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06. Claude applied them on
2026-11-08. They cover Home's list and the request block that Home shares with threads.

### Order and age

- **Home uses the bell's order.** Failed starts come first, then the request that has waited
  longest.
- **Sort offers Oldest first and Newest first.** Newest first flips only the age, so failed
  starts still lead.
- **A long wait is amber.** A request that has waited an hour or more shows its age in the
  attention color, on Home and in the bell.

### Filters

- **One search field, one Filter button, and Sort.** The Filter button opens a menu with the
  repository, project, agent, and machine filters. All projects adds Status. Escape or a
  click outside closes the menu. The band is one row on a desktop and two short rows on a
  phone, where it used to take about 200 pixels.
- **A set filter is a chip.** Each chip names its filter and value, such as "Machine:
  Studio", and removes that filter when selected. Clear filters sits after the chips and
  appears only while a filter or a search is set.
- **The count line is for filtered lists.** Without a filter, the tab counts say the same
  thing, so the line is read only by screen readers.

### Rows

- **The kinds are Question, Approval, Merge, and Failed start.** Failed start is red, and
  the others are amber while open. On narrow screens the second line starts with the kind as
  one word, and Failed start becomes Failed.
- **An approval's row shows its command.** The first 60 characters follow the title in
  JetBrains Mono, so the row shows what will run, not only what the agent called it.
- **A merge row waits like any other.** It says how long the merge has waited, and it says
  Merge rather than Approval.

### Answered requests

- **Tab counts are open requests only.** They match the bell.
- **Answered requests fold away.** Questions and Approvals end with a collapsed Answered
  section, most recent first. An answered row says Allowed, Denied, Answered, Merged, or
  Cancelled, with the time.

### The expanded row and the request block

- **The expanded row starts with the request.** It has no header and no close button,
  because the row above names the agent, the project, and the title, and its chevron closes
  it. The footer has the links, the machine, and the worktree.
- **One request block.** Questions, approvals, merges, and failed starts use
  one block. It has a 2 pixel left rule and a label that
  names the kind. Both are amber while a question, approval, or merge is open, red for a
  failed start, and the line color once closed. It has no top rule.
- **The block spans the thread.** A question sits inside its agent's turn, so it reaches
  back over the avatar column to the thread's full width, the same as an approval. On Home
  the label and title are hidden, because the row shows them.
- **An approval names its command and folder.** Command and Folder are labelled lines in
  JetBrains Mono, above the answer buttons. The request, tool, and call identifiers and the
  raw input wait under a closed Details disclosure.
- **One filled button.** An expanded row holds at most one primary button, so Home never shows
  two.

## The composer and agent threads, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06. Claude applied them on
2026-11-08. They cover the composer, Add context, and what a thread shows besides messages.

### One input over one tool row

- **The composer is 74 pixels tall.** It is one input with one tool row beneath it, inside one
  border, in Chat and in the lead panel alike. The recipient line, the worktree line, and the
  keyboard hint are gone. The input's label still names the agent, and the hint is read only by
  screen readers.
- **Both places have a top rule.** A line separates the thread from the composer in Chat, as it
  already did in the lead panel, so thread text never runs into it.
- **The tool row holds, in order:** Add context, the subscription, the model, the effort, the share
  of the context window in use, and Send at the right end. Stop response takes Send's place while
  a reply is running.
- **The subscription shows its own name,** such as "Claude Code · Team". It no longer says Host
  profile. The agent's own subscription is chosen until the owner picks another.
- **Each menu fits its label.** Subscription, model, and effort are buttons that open one shared
  menu above them, so no fixed width leaves a gap before the chevron. Arrow keys move through the
  menu, and Escape returns to its button.
- **The model and effort belong to the agent.** The data layer stores them, and the next turn
  uses them. The context share is the harness's latest usage report, such as "34% context".
- **Narrow composers drop words, not controls.** Below 600 pixels the subscription drops its
  harness name. Below 420 pixels, as in the lead panel and on phones, Add context shows only its
  icon and "context" leaves the percentage.

### The line above the composer

- **It shows only while the agent is busy or something waits.** Examples are "Waiting for
  approval · Run the web tests" and "Working on your message". An idle agent has no line.
- **Stop shows only while the agent works or waits on a tool.** Waiting for an approval or an
  answer counts, because the agent is mid-turn. A lead with no sprints in flight,
  such as Nico before the mocks are approved, offers no Stop.

### A stopped or failed agent cannot take a message

- **The state and its remedy replace the input.** A failed start says "Ash failed to start." and
  its reason, with Sign in on Buildbox and Resume. A refused model or effort offers the model and
  effort menus and Try again. A stopped agent says so and offers Resume as the primary button.
- **The draft is kept.** The panel says "Your draft is kept." when there is one, and the input
  returns with it after Resume.
- **The thread keeps the failed start as a record.** Its block says what went wrong and how to fix
  it, and the controls live in the composer, so a thread never shows two Resume buttons. Home's
  failed-start rows keep their own controls.
- **A row with two states says both.** A stopped agent with an open question says "Stopped · Needs
  answer" and wears a hollow amber dot. A sidebar narrower than 210 pixels per row drops
  "Stopped · ", and the hollow dot and the hover text still say it. At the default width of
  248 pixels an agent row is 174 pixels, so it drops it too. Claude tried shorter forms on
  2026-11-08, such as "Stopped · Question", and none fit beside a name and read well, so the
  rule stays.

### What a thread shows besides messages

- **An approval holds its tool call.** The call, its state, its input, and its output sit under
  the approval's Details, so the command shows once.
- **A request held by the lead names the lead.** It says "Held by Sage, the project lead", with a
  link to Sage's thread. The sidebar says "Waiting on Sage", as the line above the composer does.
- **An unconfirmed answer can be refreshed.** "Refresh this request" re-reads it before the owner
  answers again.
- **Delivery history is quiet unless something failed.** It is a muted Delivery details
  disclosure at 12 pixels. A message that was returned or withdrawn opens as "Message not
  delivered" in the error color.
- **A large item is named by its content and size,** such as "Review trace (192 kilobytes)".
  Loaded, it shows its first 12 lines with Open in full and Copy. A failed load offers Retry.
- **Open in full fills the main area.** The whole text is one plain block, so the browser's find
  reaches all of it. The link back and Escape return to the thread, at the item.

### Add context

- **Search, then groups.** The list has a search field and two groups, Tasks and Files, with rows
  of 32 pixels. Enter in the search field adds what is checked.
- **Typing @ opens the same list.** An @ typed at the start of the draft or after a space opens it.
  Adding context removes that @, and Cancel leaves it in the draft.

## The board, sprints, tasks, and wide screens, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06. Claude applied them on
2026-11-08. They cover the board's cards, the sprint and task pages, the project and task
tables, and what a monitor wider than 1800 pixels shows.

### Board cards

- **A card leads with what waits for the owner.** A card that needs an answer or an approval
  starts with that line, in amber, and has an amber left rule. Two or more requests read
  "2 requests need you".
- **Then the card names the sprint.** It shows the identifier and phase, the title, one line
  of goal cut with an ellipsis, the owner with the others working on it, and the progress.
- **Progress is not an empty bar.** A sprint with no finished task says how many tasks it
  has. The bar appears once a task is done, as "1 of 2 done".
- **Column headings have no dot.** The name and the count say everything.
- **Cards stop at 300 pixels.** A column with more cards is wider, so its cards sit side by
  side. A column is at least 180 pixels wide, so four fit beside the lead at 1440 pixels.

### Sprint and task pages

- **A sprint page has a middle and a rail.** The middle holds the title and one tab at a
  time. The rail on the right is 224 pixels wide and holds everything about the sprint. The
  owner chose this layout on 2026-11-22, from three prototypes.
- **The tabs are Requests, Tasks and Plan.** Requests is there only while a request waits,
  and then it opens first, so what the page waits for comes first. Otherwise Tasks opens. A
  sprint with a plan and no task opens on Plan. The left and right arrow keys move between
  the tabs. A request's buttons are outlined, as "Layout and density" says.
- **The open tab is in the address,** such as `/p/console/sprint/sp3/plan`, so a link can
  open a sprint's plan. Switching tabs does not add a step to the Back button.
- **A menu moves the focus with the arrow keys.** The item under the arrow keys holds the
  focus, so a screen reader says it. Home and End go to the first and the last item, and Tab
  closes the menu.
- **The page behind a dialog cannot be reached.** It is inert while the dialog is open.
- **The first Tab stop of every page is "Skip to content".** It shows only while it has the
  focus.
- **The right end of the tab row belongs to the open tab.** On Tasks it shows the count,
  such as "0 of 7 done", and Add task. On Plan it shows the plan's file and its state. Only
  the Requests tab carries a count of its own.
- **One view does not say a thing twice.** The same thing may show on two views. A short
  form beside its long form is allowed: the goal in the rail is the summary of the plan, so
  it stays beside the Plan tab. Decided by the owner on 2026-11-22.
- **The rail reads from the top: the next lane, the goal, the facts, the evidence.** The
  goal has the label "Goal". The facts are Status, Phase, Priority, Epic, Team and Depends
  on, the same on every sprint. A sprint that is done lists its evidence under the facts,
  with the label "Evidence".
- **A sprint with no task keeps the table's header and one muted row,** "No tasks yet", as
  "Empty states" says. A stacked table has no header, so it shows the row alone.
- **A sprint's name wraps.** It is never cut.
- **A sprint's task is one line** with Task, Title, Status and Owner, in columns of fixed
  width, so they do not move from one sprint to the next. The owner's name and the points
  show when the page is at least 900 pixels wide. A status is a plain word. Only a blocked
  status has a colour. A task that carries out a part of the plan ends with a button that
  opens the plan there.
- **A sprint's task table stacks below 480 pixels of table width,** as "The phone" says.
  This is the usual case at 1440 pixels with the conversation open. The title is whole on
  the first line. The owner leads the second line and the status follows it. Each starts at
  one edge down the list. Decided by the owner on 2026-11-22.
- **The plan's frame drops the heading that repeats the sprint's name,** and its text starts
  on the page's left edge.
- **A narrow page has no rail.** Below 600 pixels the facts become one line under the title,
  and the next lane becomes a row with its button.
- **A task blocked by its owner's approval says "Blocked by an approval".** The owner's open
  approval blocks the task that owner is working on.
- **A task page's actions sit at the top right,** beside the link back to the sprint. Talk to
  the owner is the primary button unless a request waits, and then the request's answer is.
- **A task page links its owner** and does not repeat the identifier and status from the line
  under its title. Folders and branches are cut at their start, with the full path on hover.
- **Each activity step says when it happened,** as a time today or a date and time before.
- **A section's action sits on its heading's line.** Add task lines up with Plan tasks.

### Tables

- **Tasks are one line.** Sprint and team task lists are tables with Task, Title, Status, and
  Owner columns. A row is as tall as the row token, which is 30 pixels in the Compact
  density. Team tasks use the wide frame, so they start at the tab row's left edge. A task
  table narrower than 480 pixels stacks to two lines, as "The phone" says.
- **A table row, a request card and a post have no coloured left edge.** A word or an icon
  in the row says its state. Decided by the owner on 2026-11-22.
- **All projects and the repository page share one table.** Its columns are Project,
  Repository, Lead, Stage, Machine, Status, and Branch, one line a row. The repository page
  leaves out Repository and drops its filler sentences. Status uses the same words as All
  projects: Needs attention with its count, Working, Designing, or Planning.

### The thread panel and wide screens

- **The panel docks the current thread.** It shows the agent whose view you came from: the
  lead on the board, or Iris on Git after Iris's thread. A team's views dock the team chat.
- **Git offers the panel at every width.** Below 1801 pixels it starts closed on Git, because
  Git's three columns need the width, and the lead button opens it.
- **Above 1800 pixels the panel stays open beside every project view,** including sprints,
  tasks, Terminal, Project settings, and team Tasks. Chat is the thread itself, so it shows
  the agent's task and changed files beside the thread instead. A lead shows its sprints in
  flight.
- **A lead's status counts its team's requests.** The line under Chat says "Waiting on 3
  requests", as the line above the composer does.
- **A request held by the lead does not count for the owner.** The bell and Home's Needs
  attention leave it out until the lead releases it.

## Git and the terminal, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06. Claude applied them on
2026-11-08. They cover Git's three columns, conflicts, and terminal sessions.

### Git's columns

- **Diff rows are 20 pixels.** A row has no padding of its own, so about 38 rows fit on a
  laptop screen instead of 21. File and folder rows in the commit pane are 28 pixels.
- **The worktree list folds into a rail of 40 pixels.** The rail keeps each worktree's icon,
  with a dot for changes and a red dot for a conflict, and hover text that names it. The
  button at the top of the list folds and opens it.
- **Below 1600 pixels the list starts folded.** The browser remembers the owner's choice once
  they make one. Open, the list is 240 pixels wide. Phones keep their strip of worktrees.
- **The commit pane can be resized.** Its handle works like the sidebar's. The arrow keys move
  it by 16 pixels, and Home or a double click restores 280 pixels.
- **Each worktree names its machine and labels its count,** such as "2 changes" or
  "1 conflict". A conflict is red and has a warning icon.

### The commit pane

- **Sync is pinned at the top.** The header names the team or project, and holds Fetch, Pull,
  Push, and the incoming and outgoing counts. The last Git message sits under it.
- **Integrate is pinned at the bottom.** The commit form, the file groups, and History scroll
  between the two, so a long file list hides neither.
- **Changes has Stage all and Discard.** Staged changes has Unstage all. Each appears only
  when its group has files.
- **Discard asks first.** Its dialog lists the files and says that the change cannot be undone.
  Its button names the count, such as "Discard 2 changes".
- **"Back to changes" replaces the Uncommitted changes link.** It appears only while a commit
  from History is open, so nothing that looks like a status is a link.

### Conflicts

- **Conflicts have their own section above Staged changes,** with a red count. Stage all and
  Discard leave conflicted files alone.
- **A conflict opens in the diff pane.** The pane shows the shared lines, and for each conflict
  it shows ours, the base, and theirs, one above the other. Each conflict offers Ours,
  Theirs, and Both. Both keeps ours, then theirs. A chosen conflict shows its result.
- **Mark resolved finishes the file.** It needs a choice for every conflict, and the reason
  beside it says how many are left. The file then returns to Changes for review and staging.
- **Open in Terminal is the way out for hard cases.** It opens the worktree's terminal with
  `vim` and the file already typed, and the editor shows the conflict markers.
- **The integrate line names the conflict,** such as "Resolve 2 conflicts in workspace.tsx
  first.", and the file name opens it. Conflicts come before the destination's changes,
  because the owner can only fix their own worktree here.
- **The whole-file conflict dialog is gone.**

### Terminal sessions

- **A session tab closes with its own button.** The button shows on hover and on focus, and
  always on a phone. Closing the last session leaves "No terminal session is open." with New
  terminal.
- **A double click renames a session,** and so does F2. Enter keeps the name, and Escape keeps
  the old one. The name lives in the data layer with the session.
- **A full-screen program sits on the grid.** Its text, its status line, and its command line
  start at the grid's left edge. The status line and the command line are one row each, and
  the command line is the last row of the window.
- **The prompt wraps only at the column limit,** as a terminal does, and the input takes the
  rest of the prompt's last line.
- **Phones have a row of special keys** above the keyboard: Escape, Control, Tab, and the four
  arrows. Control stays pressed for the next letter, so Control and C stops a command. The keys
  keep the focus, and with it the keyboard, in the terminal. Tab completes a word on every
  screen size.

## The service pages and Project settings, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06. They cover Phone
notifications, Machines, Subscriptions, Service settings, Project settings, and
New project.

### Shared controls on these pages

- **Actions sit at the top right.** Each page starts with a short status line and its
  actions, such as Connect machine or Add subscription. The pages have no filler subtitles.
- **Rare actions live in a row menu.** The menu is a button with three dots at the end of
  a row. It holds Rename and Remove, and a machine's menu starts with Add repository. A
  disabled item says why beneath it.
- **Every destructive action asks first.** One shared dialog names what will be lost, with
  Cancel on the left and the action on the right.
- **Rename uses one shared dialog.** It refuses an empty name and a name already in use.
- **Rows stack below 720 pixels of table width.** The name and the actions share the first
  line. The other facts go on the second line in muted text. The rule follows the table's
  own width, so a 768 pixel window with the sidebar open stacks too.

### Notifications

- **The columns are Notification, Time, About, and Delivery.** About names the agent and
  the project, and the agent's name opens its thread. The State column is gone.
- **Every failed delivery uses the error colour.** The badge counts the same failed rows.
- **A failed notification offers Retry.** Retry without a token fails again, honestly, as
  another attempt.
- **A missing token shows Fix in Service settings once, at the top.** It opens the token
  field. Repeating it on every row would add noise.

### Machines and subscriptions

- **Agent slots count running agents only.** A machine over its limit shows its count in
  amber with "Over limit" and a warning icon.
- **An offline machine says when it was last seen,** such as "Last seen 2 hours ago".
- **Remove is refused for this computer and for a machine that runs projects.** The menu
  says to move the projects first.
- **A worker shows its master's address and how long it has been connected,** with
  Disconnect. Disconnect asks first.
- **Subscriptions show usage.** Usage of 90 percent or more is amber, with the time it
  resets. A subscription that reports nothing says "Not reported".
- **Use for new work saves on change** and shows a small Saved mark.

### Service settings

- **A section list on the left follows scrolling.** Below 1100 pixels it becomes a row of
  links above the sections.
- **Fields save on change.** There are no Save buttons. Each field shows "Saved" beside
  its label for four seconds, and an invalid value shows its error under the field.
- **Turning the web server off asks first,** because it ends every open web session.
- **Numeric fields are 96 pixels wide.**
- **Send test notification sits beside the token.** It is disabled until a token is set,
  and the line under it says why.
- **First use of a permission rule** carries one sentence that explains it.

### Project settings

- **The facts are a list of labels and values,** not read-only fields. Machine links to
  the project's placement on Machines.
- **Rename and Archive sit at the top right.** Archive asks first and stops the project's
  running agents.
- **An archived project leaves the sidebar, Home, and the bell.** All projects lists it
  last, marked Archived. Its Project settings page offers Restore.
- **The preferred shell saves on change.**

### New project

- **The project lead controls share one row,** decided by the owner on 2026-11-19. Harness, subscription, model and effort use the same dropdown component and compact pick buttons as the chat composer.
- **The dialog is at most 520 pixels wide.** Machine and repository have compact placement rows. The lead choices do not become four separate form rows or two paired columns.

- **The fields are Run on, Repository, Project name, Goal, and Lead subscription.** The
  first run hides Goal, because the first project's goal comes from the onboarding
  conversation.
- **Lead subscription shows when a subscription is ready.**
- **When no machine has the repository, the dialog shows one message** with a link, such as
  "Add it to Buildbox", or "Add atlas-docs to a machine" when the chosen machine is offline. There is no separate error under the field.
- **Add repository lives on Machines.** The link opens Machines with Add repository filled
  in: the chosen machine, the repository, and a folder in the machine's workspace folder.
  Adding it returns to New project with that machine chosen and the draft kept. Cancel also
  returns to New project.

## The phone, set on 2026-11-08

These rules come from the designer's critique of 2026-11-06, findings TOP-3, PHONE-1,
PHONE-2, and PHONE-3. Claude applied them on 2026-11-08. A phone is a window of 700 pixels
or narrower.

### The top bar

- **The top bar holds three things.** They are the menu, the place, and the bell. The place
  is the project, or the team inside a team. Chat's line under the tabs names the agent, so
  the top bar does not repeat it.
- **The connection is a dot on the bell.** The dot is green for Live, amber for Connecting
  and Reconnecting, and red for Offline. The bell's name says the state, such as "LGTM Studio is
  reconnecting".
- **The bell opens the full state.** The menu starts with the indicator, the time of the
  last update, and Reconnect now, above Needs attention.
- **Inspecting moves under the tabs.** It is a line of 32 pixels at the top of the tab
  content, on every project view. A desktop keeps it in the top bar.

### The drawer

- **The drawer is the tree.** The whole drawer scrolls as one, and the tree has no scroll of
  its own. The service pages and the account follow the tree at the end.
- **Agents needing attention come first.** Under the brand, Needs attention lists each agent
  with an open request once, in the bell's order. A row names the agent, its status, and its
  project, and it opens the agent's thread.

### Lists and tables

- **The board is one list.** Sprints that need the owner come first, then In progress, In
  review, Planned, and Done. Each card starts with its status. The column jump strip is gone.
- **The status leads every phone row's second line.** All projects, the repository page,
  task tables, Machines, Subscriptions, and Notifications put the name on the first
  line and the status first on the second. A sprint's task table is the exception: its
  second line starts with the owner. All projects stacks below 620 pixels of table
  width, and a task table below 480, so a 768 pixel window stacks them too.
- **A phone diff has one line-number gutter.** It shows the new line's number, or the old
  one for a removed line.

### Tabs

- **Home's tabs have short names on a narrow row.** Below 580 pixels the tabs read
  Attention, Projects, Questions, and Approvals. Below 490 pixels New project shows only its
  icon. The rule follows the row's width, so it holds at 768 pixels beside the sidebar.
- **A tab row that still overflows fades** at the edge it can scroll toward.
- **Initials never wrap** in an avatar.

### Leftovers fixed in the same pass

- **A narrow top bar keeps the place readable.** Below 640 pixels of top bar, the repository
  and the word Inspecting drop out, and the branch shrinks before the breadcrumb does.
- **A docked request has its own identifier.** Above 1800 pixels a docked thread can show a
  request that the page beside it shows too. The docked copy's identifier starts with
  `dock-`, and answering it keeps the focus on the copy that was answered.

## Layout and density, set on 2026-11-10

These rules describe the rebuilt mock in `design/mock/`. They await the owner's approval.

LGTM Studio is an operating tool that the owner keeps open all day. The default density is
Compact. A row is 30 pixels high. A control is 28 pixels high. The inset inside a region is
10 pixels. The gap between groups is 20 pixels. Regular density is the alternate, and
Service settings switches between them. The values live in `design/mock/css/tokens.css`
under `--row`, `--control`, `--inset` and `--gap-group`.

The main screen has three regions. The sidebar on the left holds the project tree and the
service pages. The centre holds the current tab. The lead conversation docks on the right of
every project tab except Chat, and a toggle at the end of the tab row hides it.

A list of like things is a table or a list of rows with hairline dividers. A card is used
only for an item that moves on its own, such as a sprint on the board or a request that
needs an answer. A card never sits inside another card. Home, All projects, the Mocks tab,
Test runs, and the Assignment section of Project settings are tables for this reason.

A screen with no data keeps its structure. A table shows its header and one muted row,
such as "No screens" or "No fix sprints". A board keeps its columns.

A status line sits under a secondary tab row and above the main table. It holds the
counts, the badges and the screen's buttons, and it never holds a sentence that explains
the screen. Workflow, Mocks and the workflow's settings use it.

The repository Design page fills the centre with a canvas. One thin bar above it holds the
Projects and Design tabs, the category and item menus, the file's name, the zoom controls,
and "Open in OpenPencil". The Mocks tab shows one screen on the same canvas.

A question or an approval in a thread starts as a brief. The brief shows what is asked in a
few words and how long it has waited, and one line of description. A question adds its
recommended answer beside the sparkles icon, with Accept, Deny and Details. Each action has a default
icon, a check for Accept and Allow and a cross for Deny. The sparkles icon replaces the
default icon only on the action the agent recommends, so on a question only Accept carries
it. Accept, Deny, Allow and Details are outlined buttons. Deny strikes out the
recommendation and opens the details with "Your own answer" focused. An approval adds Allow, Deny
and Details, even when the harness hides the command. Allow has a menu with "Always allow",
and Deny has a menu with "Always deny". Its command is shown only in the details.

Details keeps the brief and adds a short report under it. The report starts with the agent
that asked and what the request blocks. Its sections follow: the full question or the
command and folder, "Background", "Impact", and "Rationale" for the recommended answer. The
headings carry the colour, blue for the question, amber for "Impact", and green for
"Rationale", with no coloured bars. A heading is a term of a written report, not a phrase
someone would say. The headings were "What it is about", "Why it matters" and "Why this
answer" until the owner asked for report terms on 2026-11-22. The other answers are rows, each with a short
note and a "Choose" button, and a question ends them with a "Your own answer" field. The
numbered references come last. "Open thread" sits at the right end of the card's buttons.
Paragraphs start with an indent and keep a reading width of about 72 characters.

In the mock no card has details in place. Details on a question's card or on an approval's
card opens it in the Questions view. Deny on a question's card opens it there too, with the
recommended answer declined. "The Questions view" below has the rules.

The owner's messages sit on the right of a chat, in a filled bubble. An agent's messages
sit on the left without a bubble. Only the project lead's thread has a message box. Every
other agent's thread is read-only.

Each screen has at most one filled primary button. Body text uses the full text colour.
The muted colour is for captions, paths and times.

## Agent names, set on 2026-11-10

The owner said on 2026-11-10 that agents must not carry invented personal names. An agent
is named by its role. The names are Project lead, Build lead, Planning lead, Design lead,
Review lead, Builder 1, Builder 2, Reviewer, Test runner, Test writer, Planner and Designer.
A team lead's team is shown beside its name. The avatar is a glyph for the role. A direct
message between two agents shows the first avatar, a short dash, and the second avatar.

A new project is named from its goal. It shows "Naming…" in the sidebar until the name is
set. Commit messages and pull request text are written by a small model, Haiku 5.5 by
default, chosen in Service settings under "Git messages".

## Libraries, set on 2026-11-10

The rebuilt mock has no build step. It draws its screens with Preact through the
standalone build of htm, kept in `third_party/htm-preact/`. Its icons are Lucide icons in
`design/mock/js/icons.js`. Its fonts are Inter and JetBrains Mono from `design/fonts/`.
Menus, dialogs, tabs and toasts are the mock's own small components in
`design/mock/js/ui.js`. The mock loads nothing from the network.

The Design page's canvas is the mock's own small component in `design/mock/js/screens/design.js`.
It needs zoom, Fit and focus on one frame, and no library on the approved list does only
that. Its screen frames are the mock's own addresses drawn in frames, so the Design page
always shows the code's current screens. The Palette sheet reads `css/tokens.css` at run
time to compare it with the design file.

## The Project view, decided on 2026-11-20

The owner approved this direction on 2026-11-20. It is built in the mock only.
The live website keeps opening the board until builders wire the Project view to the engine.

- **Selecting a project opens its Project view.** The Project tab is the first tab of every
  project. This replaces the rule above that selecting a project opens the board. The Board
  tab stays, and it keeps its lead panel.
- **The conversation with the project lead is in the centre.** The lead is not a side panel on
  this tab. The owner should not need to open a team unless work has gone astray.
- **The end result is a document on the right.** It lists the goal, numbered statements, what
  is left out, and what happens next. The end result is never put into a question card.
- **A question holds one decision.** It names the statement it decides and offers its answers
  as rows. Choosing a row only marks it, and "Send answer" sends it. "Something else" lets the
  owner write the answer.
- **An answer row holds only a short label.** What the marked answer leads to is shown under
  the rows, where it can be any length.
- **The decisions that wait sit above the message box.** The conversation holds no open
  question. Each decision is one line that opens and closes on its own, and opening one does
  not close another. The first one starts open, and the next one opens when it becomes first.
  The owner approved trying this on 2026-11-20, after Mobbin showed the pattern in Claude and
  Unify.
- **A team's question and an approval use the same form.** The block names the agent that
  asks. An approval shows the command and the folder, and its answers are "Allow once" and
  "Deny". One checkbox beside "Send answer" makes the answer standing in this project.
  "Impact" is a folded line, and it appears only when it has content.
- **The view carries no instructions.** A line that tells the owner how a control works, or
  what the screen already shows, is not written. Decided by the owner on 2026-11-20.
- **The conversation stays on the left of every project view.** A project is two parts. The
  left part is the conversation with the project lead. The right part is the chosen view: End
  result, Board, Questions, Workflow, Mocks, Messages, Agents, Terminal, Files, Git or Settings. Changing
  the view never moves or hides the conversation. The lead's docked panel on the right is gone.
  A button at the end of the row of views hides the conversation, so a view can take the full
  width. Decided by the owner on 2026-11-20, for the mock; the live website keeps its earlier
  layout until builders wire this.
- **There is no Chat tab.** Another agent's thread opens in the conversation's column, in place
  of the lead's, with "Back to the project lead". An address under `/chat/` still works and
  shows that agent on the left with the end result on the right. Decided by the owner on
  2026-11-20.
- **On a narrow screen one part shows at a time.** The conversation button at the left end of
  the row of views shows the conversation, and shows the view again. Choosing a tab shows
  the view, also when it is the tab already chosen. No line stands under the row of views.
  This replaced two buttons, "Conversation" and the view's name, on 2026-11-22, when the
  owner saw that the stage still stood on a third line at his window's width.
- **The conversation keeps to the left at a reading width.** Its column is 42 percent of the
  view, never narrower than 440 pixels and never wider than 560 pixels. The work product on
  the right takes all the remaining width, whether it is the end result, the mocks or the
  design canvas. Text in the end result stops at 820 pixels. Decided by the owner on
  2026-11-20.
- **The divider between the conversation and the view drags.** A dragged width replaces the
  picked one and is remembered. The conversation is never narrower than 320 pixels, and the
  view on the right always keeps 360 pixels. A double click on the divider returns to the
  picked width. Decided by the owner on 2026-11-20.
- **A view's own list sits on the far right.** In Git the list of staged and changed files is
  on the right of the diff. In Files the tree is on the right of the open file. The content is
  then next to the conversation, and the two lists of names, the sidebar and the view's list,
  are on the outside. The owner asked for this on 2026-11-21, because a list in the middle
  stood between the conversation and the content. It is in the mock only. The message board's
  list of channels is on the right of its posts in the same way, since 2026-11-21. The list
  drags from its left edge, and one remembered width serves all three views. The content
  always keeps 240 pixels.
- **Each side panel of a project has one button, above the panel.** The conversation's button
  is at the left end of the row of views, above the conversation. The list's button is at the
  right end, above the list, and it shows only in a view that has a list. Each button is one
  icon, and it is tinted while its panel shows. Hiding the list holds across Git, Files and
  the message board. The owner asked for this on 2026-11-21. One button with a crown and a
  panel icon had sat at the right end and closed the panel on the left.
- **The lead can link to an agent and to a message.** A link in the lead's message opens the
  Agents view with that agent's row marked, or opens the message board at that message with
  its thread. The owner asked on 2026-11-21 for the lead to be able to take him to a place in
  the website. In the mock the owner clicks the link. Whether the lead may also move the
  screen by itself is not decided.
- **The edges of the board fade where lanes are cut off.** A strip 36 pixels wide on the left
  or right edge blurs and fades the lanes under it, only on a side with more lanes to scroll
  to. Decided by the owner on 2026-11-21.
- **A card on the board drags to any lane, and each lane has needs.** A lane takes the card
  only when the sprint has what the lane needs. In progress needs every sprint it depends on
  to be done, and an approved plan. Review needs every task done and a passing full test run.
  Done needs a clean review and the merge. Backlog and Planned need nothing, and a move back
  needs nothing. During a drag, a lane that will not take the card dims and names what is
  missing, such as "Needs LS-A01-2, LS-A01-3, an approved plan". The order the owner drops cards
  in is kept. Decided by the owner on 2026-11-21.
- **A move into a lane starts that lane's team.** Planned starts a Planning team, In progress
  starts a Build team, and Review starts a Review team. The project lead does not create the
  team. A line in the lead's conversation says what moved and which team started. Decided by
  the owner on 2026-11-21.
- **A forced move is made from the sprint's own page.** The board never forces. The sprint
  page asks first and lists what the move skips, and the lead's line names what was forced
  past. Decided by the owner on 2026-11-21.
- **A sprint's lane is one button at the top of its rail.** The button reads the lane the
  sprint is in, such as "Build", and it opens a menu of the five lanes. Choosing a lane
  moves the sprint. This is also the way to move a sprint without a drag. A lane with unmet
  needs lists them under its name in the menu. The rail does not name a "next lane", and no
  button says "Force to". Decided by the owner on 2026-11-23. It replaces the "Next lane"
  block and the split button of 2026-11-22.
- **The prerequisites stand under the button.** The line names the lane they belong to and
  a count, such as "Before Review" and "1 of 3". One line follows for each need: a check when it is met, a clock when it
  is not. An unmet need reads what is missing, such as "LS-A01-2 not done" or "Plan not
  approved". A need names a sprint as plain text, because the links to those sprints are
  under "Depends on". A sprint with no needs shows no prerequisites.
- **A move that stops a working team asks first.** The dialog names the phase in progress
  and the team that gets the stop signal. It offers "Safe shutdown" and "Terminate" beside
  "Cancel". A move past unmet needs asks too, lists what it skips, and offers "Move
  anyway". Decided by the owner on 2026-11-23.
- **Git names its two diff layouts.** The switch in the bar of Git reads "Unified" and "Split",
  with words beside the icons. The choice is the owner's at any width above a phone, and it
  is remembered. The owner asked on 2026-11-21 for a switch, and did not see that the two
  bare icons were one.
- **The message board is a part of Agents.** There is no Messages view in the row of views. The
  board is the agents' own, so the owner reaches it under Agents, beside the roster. The two
  parts are named "Agents" and "Message board". An address under `/messages` still works.
  Decided by the owner on 2026-11-20.
- **A command an agent gives can be pasted into the terminal.** A fenced shell block in the
  lead's message has the button "Paste in terminal". The button opens the Terminal view and
  puts the command at the prompt of a session the owner can type in. Nothing runs until the
  owner presses Enter. Decided by the owner on 2026-11-20.
- **The sidebar folds and opens from its edge.** While the pointer rests on the right edge of
  the sidebar, a strip 20 pixels wide shows along the whole edge, with an arrow at the
  pointer's height. A click on the strip folds an open sidebar and opens a folded one. A drag
  on the strip resizes the open sidebar. The strip goes away when the pointer moves on. The
  button in the top left corner stays. The arrow sits 30 pixels above the pointer, so the
  pointer's own shape never covers it. Decided by the owner on 2026-11-21.
- **A drag folds and opens the sidebar.** Dragging the edge of the open sidebar to half
  its narrowest width folds it, which is 110 pixels past the point where it stops shrinking, and the width from before the drag is kept. Dragging the edge
  of the folded sidebar to the right opens it. Decided by the owner on 2026-11-21.
- **The folded sidebar peeks.** With the sidebar folded, the pointer resting on the left edge
  of the window for 1.5 seconds slides the whole sidebar over the page. A pass over the edge
  opens nothing. It slides away soon after the pointer leaves it or leaves the window, at once
  when the window loses focus, and on Escape. The page under it does not move. A click on its edge opens it for good.
  Decided by the owner on 2026-11-21. The owner is not sure the peek should stay, because the
  pinned tiles already serve a folded sidebar. The mock's controls menu has "Peek at the folded
  sidebar" to turn it off until that is decided.
- **The box for an answer in your own words grows with the text.** It grows to six lines and
  scrolls after that. Decided by the owner on
  2026-11-21.
- **A drag handle has no accent colour.** The line and the grip that show on a divider are
  grey. Decided by the owner on 2026-11-21.
- **The folded sidebar shows the pinned projects.** Each pinned project is a small tile with
  the project's prefix, and a dot when the project needs the owner. The open project shows
  too when it is not pinned, with a dashed edge. A project is pinned and unpinned from its row
  in the open sidebar. Decided by the owner on 2026-11-20.
- **The waiting decisions fold into one line.** The line reads "Waiting on 3 answers" and names
  how many are questions and how many are approvals. It starts folded. Opening it lists the
  decisions, all closed, and each one opens and closes on its own. A lone decision opens with
  the line. The count in the top line opens the line and the first decision. The dock opens
  upward, so the caret on its line points up while closed and down while open. A question or
  approval row inside it carries the double caret, which does not turn. Decided by the owner
  on 2026-11-20.
- **Nothing sits between the waiting decisions and the message box.** The Project view has no
  status line and no "Stop agent" button there. While the lead works, the send button is the
  stop button, and the lead's activity is the last line of the conversation. Decided by the
  owner on 2026-11-20.
- **The waiting decisions and the message box are one unit.** They share one edge and one
  background, with a thin line between them and no gap. The unit has no coloured bar on its
  left. The words "Waiting on 3 answers" carry the warning colour. Decided by the owner on
  2026-11-21.
- **A waiting decision has two lines.** The first line is the question, and it may wrap to
  a second line. The line under it holds the kind, the asker in full and the wait. Both
  start 12 pixels from the unit's edge, where the placeholder of the message box starts too.
  An open decision shows its whole question. Decided by the owner on 2026-11-23. It replaces
  the single line of 2026-11-22, which cut the asker after a title of 48 characters.
- **A waiting decision opens in Questions from its head.** A small arrow button stands
  beside the fold button, with the tooltip "Open in Questions". The "Details" link under the
  answers is gone. Decided by the owner on 2026-11-23.
- **Waiting decisions are parted by the strong line in dark mode.** The ordinary line was
  too faint there.
- **Questions, answers and follow-ups stay in order.** An answered question goes into the
  conversation where it was asked, with the owner's answer under it. Its title opens and closes
  the full question in the same way.
- **A merge and a failed start keep their earlier cards.** They are not recut yet.
- **Agreeing to the end result ends Define.** The action is refused while a question is open.
  The project moves to Design when screens change, and to Plan when they do not. Design work
  starts only after agreement.
- **The work stays inside the agreed end result.** After agreement, a note on a statement marks
  it "Change asked", and the project keeps its stage. A heavier change of scope redefines the
  project; that is not designed yet.
- **The stage is one word at the right end of the row of views.** It reads "Stage Build" and
  opens the list of the six stages: Define, Design, Plan, Build, Review and Hand over. In the
  list a finished stage has a check, the current stage has a filled dot and strong type, and
  a later stage has an empty circle. A project has no line of stages of its own. The owner
  decided this on 2026-11-22, after a blind review found that the line read as a second row
  of tabs and took 36 pixels on every view.
- **The stage stands still.** It is the last thing in the row on every view. The count of
  what waits and the list button of a view stand before it, so neither moves the stage when
  it comes or goes.
- **Below 1280 pixels the stage keeps its word alone.** The label "Stage" and the count leave
  the row, because the tabs need the room. The count stays on the Questions tab.
- **A row too narrow for every tab scrolls under its two ends.** The conversation button on
  the left and the stage on the right stay in sight. The tabs scroll between them, and a
  short fade before the stage says that tabs go on under it. The chosen view's tab is always
  brought into sight.
- **Reading text is larger.** The conversation and the statements use 14 pixels, a line height
  of 1.62, and a measure of 66 characters. The tokens are `--text-read`, `--leading-read` and
  `--measure-read` in `design/mock/css/tokens.css`.
- **The lead's reading and commands fold into one line.** The line opens to the list of files
  and commands.

## The Questions view, approved on 2026-11-22

The owner approved this view on 2026-11-22. It is built in the mock only. The live website
has no Questions view until builders wire it to the engine.

- **Questions is a view of a project, after Board.** Its tab counts the questions that wait
  for the owner, in the warning colour. A question a lead holds for its team is not counted
  and not listed.
- **The view is one question beside the list of questions.** The question is in the middle,
  next to the conversation. The list is on the far right, like the list of Git and of Files,
  with the same button and the same drag.
- **The list has two groups, "Waiting" and "Answered".** Waiting starts with the question
  that has waited longest. Answered starts with the latest answer. A waiting row shows the
  question, who asked, and how long it has waited. An answered row shows the answer in place
  of who asked. A wait of an hour or more carries the warning colour.
- **With no question chosen, the one that has waited longest shows.**
- **A card opens the view.** Details on a card, or the arrow button of a waiting decision
  above the message box, opens that question or approval here. No card expands in place.
- **Every link to a question or an approval opens the view.** This holds for the bell, the
  message board, the roster of agents, the lead's list of requests, the notifications and a
  row of Home. One that a lead holds for its team opens the asking agent's thread. "Open
  thread" on a card always opens the thread. A merge and a failed start keep opening in
  place on Home. Decided by the owner on 2026-11-22.
- **Approvals are in the view for now.** The owner decided this on 2026-11-22, and said that
  an approval is an agent's action and not a question, so its own design comes later. A
  waiting approval says "Approval" before the asking agent in the list. The tab keeps the
  name "Questions", and it counts the approvals that wait as well. The owner said on
  2026-11-22 that approvals will get a home of their own later, so the tab is not renamed.
- **An approval's page has the same form as a question's.** Its facts add "Tool". "Command"
  comes first, with the command and the folder in code boxes that copy. "Background" and
  "Impact" follow. "Decision" comes last, with two buttons, "Allow" and "Deny". Each has an
  arrow that offers the lasting answer, "Always allow" or "Always deny". No sentence
  explains them. An approval has no "Your own answer". Decided by the owner on 2026-11-23.
- **The question is a report to skim, not prose.** The owner asked for this on 2026-11-22.
- **The question stands alone at the top.** It is the one large text of the page. No button
  and no recommendation sit beside it.
- **A block of facts follows the question.** Each fact is a label beside its value: "Asked
  by", "Blocks" as a link to the task, and "Statements" as a link to the statement in the End result, and "Waiting" or
  "Answered". "Discuss" follows the asker's name and opens the asker's thread. It was named
  "Open thread", at the right end of the row, until 2026-11-23.
- **Ruled sections follow the facts.** They are "Background", "Impact", "Answers" and
  "References", each with a rule above it. A heading is a term of a written report. The
  headings carry no colour.
- **"Background" and "Impact" are short points.** Each point holds one claim. The text is
  the body size, with no indent. A point's mark hangs left of the text, so every text of the
  page starts on one left line. A section with one point has no mark.
- **On a pane of 560 pixels or more the headings stand in their own column.** The column is
  in line with the labels of the facts, and every value, point and answer starts on one
  second line. On a narrower pane a heading stands above its section.
- **The answers come last.** They are radio buttons when one answer is wanted, and
  checkboxes when more than one may be chosen. The agent's recommended answer is first, with
  the sparkles icon, the word "Recommended" and its reason. "Your own answer" is the last
  choice. One button, "Send answer", ends them. Decided by the owner on 2026-11-23. It
  replaces a "Choose" button on every answer.
- **A reference is a link.** It opens its file in Files.
- **The page has one layout.** The owner chose it on 2026-11-23 from three behind a
  switch, and the switch is gone. The sections are ruled and in the order of a report:
  Background, Impact, then the answers. Under the title is one line of facts: who asks,
  what it blocks, the tool, and how long it waits. The statement it is about takes a line
  of its own.
- **"Discuss" is at the right end of the title.** The owner did not want it in the middle
  of the facts. It opens the conversation with the agent that asks.
- **Deny on a card opens the question with the recommended answer declined.** That answer is
  struck out and marked "Declined", it has no button, and "Your own answer" has the focus.
- **An answered question keeps its report.** The chosen answer is marked "Your answer", and
  the other answers are muted with no buttons. An answer in the owner's own words is the
  first row.
- **A question with no report shows the agent's wording as its "Background".**

## Changes from the owner's review, decided on 2026-11-23

The owner read every screen on 2026-11-23 and said that nothing is frozen. These rules are
built in the port and nowhere else. The port was `design/mock-vite/`, and it is the website
in `src/frontend/` since the owner's decision of 2026-11-23 to split the product into three parts. The older mock in `design/mock/`
is back at the state of `master` since 2026-11-23. Every rule in this document from
2026-11-20 or later is therefore built in the port only: the Project view, the Questions
view, the new sprint page and the row of views. The rules about the sprint rail,
the waiting decisions and the question page are changed in their own sections above.

- **A project opens on the Board.** The row of views starts with Board, then End result. A
  project in Define still opens on its end result, because it has no board yet.
- **The row of views says a count once.** The Questions tab carries the count of what
  waits. The right end of the row holds only the list button of a view that has a list.
- **The button for the conversation is named.** It reads "Project lead" beside its icon.
  Below 1100 pixels it is the icon alone.
- **The End result is a document.** Each statement is one line: its number, its text, and
  its state as a word in the third column. A state is never a pill here. "Being built" is
  plain text, because it is the usual state. "Holds" is green with a check after the word,
  and "In review" has the information colour. A statement of one sentence is not bold. The head counts
  the statements in each state, and one button, "Open requests", opens Questions with its count.
  A statement that a question waits on names that question under it, as a link.
- **A tab's mark is a word.** A tab that needs attention carries a word in the state's
  colour, such as "Failed" on Test runs or "Stuck" on Review rounds. It never carries a dot
  alone.
- **A board lane is between 200 and 320 pixels wide.** The lanes share the pane's width, so
  three whole lanes show at 1000 pixels and all five show when the conversation is closed
  at 1440. This replaces the fixed width of 272 pixels.
- **A narrow window shows the view of the chosen tab.** Below 1100 pixels one half shows at
  a time. Only a project in Define opens on the conversation.
- **The view is named Status. Its tabs are Overview, Reviews, Message board and End result.**
  The first two were named Now and Review rounds until the owner asked for better names
  on 2026-11-23. "Message board" and "End result" stay, because the product uses those
  two names everywhere else. The view was named Workflow until 2026-11-23. The owner asked for a better name. Its
  address keeps the word `workflow`. The End result is the project's own view. The finished
  page of a handed-over project keeps its old address.
- **Status is a page to look at, not to work in.** The owner decided on 2026-11-23 that
  Status is a center for agents, not for tasks. The board shows what is being worked on,
  and Status shows where the project stands. Every line on it opens the place where the
  work is done.
- **The Status tab carries the stage.** It reads "Status" and then the stage in the muted
  colour, such as "Status Build". The row of views has no other stage control.
- **The Overview tab reads top to bottom in this order.** The eight phases as one strip. One
  line with the phase's number, how long ago it started, and the count of sprints. A
  statement that is stuck, when there is one. The agents, waiting first, then working, then
  idle. The two checks, Review and Actions, one line each. The fix sprints.
- **An action is a workflow file of GitHub Actions.** The owner decided on 2026-11-23 that
  these runs use the same workflow files as GitHub Actions, one to one, so that a project
  can have build and test workflows. The owner decided the same day to call them actions,
  not workflows. The Actions view lists the runs of the files in `.github/workflows/`. The filter has one
  button for each file, by its name. A run shows its file's path. A build's run shows no
  count of tests. "Run action" opens a menu of the files, each with its file name.
- **Actions is a view of its own, after Status in the row of views.** The owner chose this
  on 2026-11-23, because a run is started and read there, and Status is only looked at. Its
  tab carries a dot in the error colour when the last full run failed. Until 2026-11-23 it
  read "Actions Failed", and the owner found that it made everything look failed. Its old address under Status still opens it. Status keeps one line about the
  last run, and the line opens Actions.
- **The message board is the third tab of Status.** The owner decided on 2026-11-23 that
  the message board moves under Status, because the agents moved there. Status has the
  tabs Overview, Reviews and Message board. The tab does not read "Messages": the owner
  said that word is confused with the chat. The board keeps its own address. The old
  address of the Agents view opens Status.
- **The row of views holds what is opened often.** The owner, 2026-11-23, named the
  conversation with the project lead, the board, the questions, Status, Actions and Git.
  The row reads: Board, Questions, Status, Actions, Design, Files, Git, Settings. All eight
  fit a window of 1000 pixels.
- **Board and End result share one place.** A project in Define has no board yet, so its
  first tab is End result and it has no Board tab. From Plan on, the first tab is Board and
  the row has no End result tab. The End result is then the fourth tab of Status, after
  Message board. The owner chose this on 2026-11-23 and said the tab is redesigned later.
  The Status line also says how many statements hold, and that count opens the tab.
- **A sprint's page lists the statements it delivers.** The owner chose the page over the
  board's card on 2026-11-23. The list is in the rail, under the goal, with the label
  "Statements". Each line has the statement's number, its text and its state word. A line
  opens the End result. A sprint with no statements has no such list.
- **A task has no page of its own.** The owner proposed this on 2026-11-23, so that a
  chosen task works like the Questions tab. A task opens inside the Tasks
  tab of its sprint. The pane shows the task, and the rail lists the sprint's tasks in
  place of the sprint's facts. Each line of the list has the title, the label and the
  state. The cross at the task's title, or the Tasks tab, returns to the table.
- **A page has one address.** A task's address is `/p/<project>/sprint/<sprint>/tasks/<task>`.
  An old address moves to the page that took its place and leaves no entry in the
  browser's history. An address that names no page says "This page does not exist".
- **An address uses the word on the screen.** The owner agreed to this on 2026-11-23.
  Status is `status`, the End result is `end-result`, the Message board is
  `message-board`, and a conversation is `conversation`. The four older words,
  `workflow`, `project`, `messages` and `chat`, move to the new ones and keep the rest of
  their address. The Workflow page of Settings keeps `settings/workflow`, because that is
  its name on the screen.
- **The browser's tab names the page.** The title is the page, the project and the
  product, such as "LS-A01-3 · Agent console · LGTM Studio".
- **The terminal is a panel under the view, as in a code editor.** It is not a tab. The
  sidebar has a seventh item under Service settings, "Terminal", with its key beside it.
  Control and the backtick key open and close the panel on every keyboard. The panel takes
  the lower 45 percent of the view, and never the space under the conversation. It opens in
  the project that is open. Its old address opens the board with the panel.
- **"New session" asks where to open.** The button opens a menu of the places a session can
  open: each worktree of the project on each machine, with its folder under it. A place on an
  offline machine is off, with the reason. With one place only, the button opens it at once.
  The "Open in" select beside the button is gone. The owner asked for this on 2026-11-23.
- **The Mocks view is named Design, and it shows a product in three ways.** The owner asked
  for the name on 2026-11-23. Its row of pages has three groups, with a thin line between
  them. The first group is what was drawn: Screens, Changes, Components, Data and Design
  file. The second is what runs: Browser. The third is how the code is built: Architecture.
  The old addresses of Mocks and of Map open the new pages.
- **Design file is the repository's design file on its canvas.** It is the same canvas as
  the repository's own Design page, with the same picks and "Open in OpenPencil". Inside a
  project it has no repository tabs, and a pick takes the place of the last one in the
  browser's history.
- **Browser shows the product as it runs.** One bar names what runs, its state, and its
  machine with its port. The same bar holds the sizes Phone, Tablet and Full, Reload, "Open
  in a new tab" and Stop. The product fills the pane under it, in a sandboxed frame. A phone
  or a tablet is drawn at its own size on the canvas's dots, and made smaller to fit. A
  preview that is not running shows one button, "Run" and its command. What is shown and
  its size are in the address.
- **Architecture is the codebase map.** The owner decided on 2026-11-23 to name it
  Architecture, not Map. It was the Map view, between Files and Git, since earlier that day. It shows
  the map that an agent built with Floorplan. The map's page fills the pane, in a sandboxed
  frame. One line above it says how old the map is, and holds three controls: "Update",
  "Focus", and "Open in a new tab". The line does not repeat the counts of files and lines,
  because the map's own page shows them. "Focus" gives the page the whole window, as it does
  for the board, and hides the row of pages. A project with none shows "No architecture
  diagram yet" and the button "Draw the architecture". The page follows the website's
  light or dark theme.
- **A project's Settings has four short pages: General, Agents, Workflow and Previews.** The
  owner asked for an overhaul on 2026-11-23, after a critique that scored the old page 19 of
  40. The old Project page was 1,780 pixels tall in a pane of 818. No page needs a scroll at
  1440 by 900 now. General is the project and where its code is. Agents is the models, the
  permissions and the teams. Workflow is the rules of the build loop. Previews is what the
  Browser page can run. The row of pages ends with the word "Saved" after a change.
- **A setting is on one page, under one name.** "Write tests first" and "Agent review" are
  on Workflow only. They were also on the Project page, and the first had a second name.
- **A fact that cannot be changed is not a row.** The repository, the branch, the folder and
  the task labels are one line of names and values above the rows of the Code section.
- **A row's help mark shows when the pointer or the cursor is on the row.** At rest the old
  page showed fourteen of them. This holds for the service's Settings too. A device with no
  pointer shows every mark.
- **A choice with a sentence for each option is a select, not a list of radio buttons.**
  "Agents may" is one select. The meaning of each option is in the row's help. Choosing
  "Full access" asks first, because it lets every agent run commands unasked.
- **A text field saves when the cursor leaves it, or on Enter.** The name, the commands and
  the fields of a preview do this. Escape puts the old text back. The goal is the exception:
  it keeps "Save goal" and "Discard", because saving it tells the project lead.
- **The lead's model is the first row of the Models table.** It was a row of three selects
  under "Repository and machine". The button under the table names its outcome: "Use for
  Plan, Build, Review and Test". The header's button reads "Approve assignment", and it is
  the filled button of the page until it is pressed.
- **The website names no person.** The owner decided this on 2026-11-23: one person uses a
  server, so a name on the screen says nothing. The foot of the sidebar shows which server
  the website talks to, as a host and a port, or "Sample data". The owner's own messages
  carry a plain figure in place of initials. The first-run screen does not ask for a name.
- **The Browser page has a "Pick an element" button that does nothing yet.** It stands
  before Reload, and it turns on and off. The picker itself needs a browser extension,
  which the owner chose on 2026-11-23 and which is not built.
- **A tab that needs the owner carries a dot, not a word.** The label "Agents To approve"
  read as one name, so the owner asked for a dot on 2026-11-23. The Settings tab and its Agents page
  carry a dot in the warning colour while a model assignment waits for approval. The
  Actions tab carries a dot in the error colour after a failed full run. Each dot has a
  name for a screen reader, and the same words as its hover text.
- **A refused value is named under its row.** A preview with a taken name marks the Name
  field, and a line under the row says what was refused and what to do. The typed value
  stays. A toast far from the row is not an error message.
- **A row of settings stacks below 720 pixels.** The name sits above its control.
- **A screen drawn inside a frame does not take the focus.** The sign-in screen focused its
  password field inside the Design file's canvas, and the browser scrolled the canvas to it.
- **A row's destructive action in a table is an icon at the end of the row.** A team and a
  preview each have a bin there, with its name as hover text. Three red buttons in a column
  were the loudest thing on the page.
- **"Delete project" is always in sight.** It is off until the project is archived, and its
  hover text says so.
- **A preview is one row of a table: name, command, port, machine and state.** The last row
  is empty and adds a preview. The state is a link to the Browser page. A change of the
  command, the port or the machine stops a preview that runs.
- **The line beside Generate, in Git, says only what blocks a commit.** It is empty when
  nothing does. It does not count the staged files, because the list under it does. The
  line "Write a message first" was cut short there, and the owner asked on 2026-11-23 to
  remove it.
- **The Actions view has a narrow rail on the right with the actions themselves.** The owner
  asked for this on 2026-11-23, with GitHub's Actions tab as the yardstick. The left side is
  as it was: the filter by action, the list of runs, and the open run beside it. The rail
  lists each action in two lines: its name, and its file as a link. The link opens the file
  in the Files view. A click on an action's name shows only its runs, and a second click
  shows every run again. The chosen action and the open run are in the address, and a choice takes the place of the last one, so Back leaves the view in one step. The rail has nothing else: when an action runs is a tooltip on its
  name, and its jobs and steps are in its file. The actions are a list with lines between
  them, not cards. The runs always stand beside the open run, never above it. When space is
  tight the list of runs gets narrower and a run's title takes two lines. A first redesign of
  the same day put a rail of action names on the left and gave a run its own page. The owner
  turned it down: that rail read as labels, not as actions.
- **A view's own header has a Back button when the owner followed a link to get there.** The
  owner asked for it on 2026-11-23, with the file header of the Files view as the example. It
  is the first thing in the header: an arrow and the name of the view it returns to, such as
  "Actions". On a page with a title, such as a question or a team, it has a line of its own
  above the title. On Status, the message board and the end result it is the first thing in
  the row of sub-tabs. It returns to the address the owner came from, as the browser's own Back does.
  It never goes to a parent of the view. It shows only after a link inside a view or the
  conversation, or after an item opened on a page of its own. A move through the row of
  views, the sidebar, the path in the top bar or a row of sub-tabs is a switch, so it shows
  no Back. `design/flows.md` lists every move between the screens, its kind, and the
  screens where Back belongs. A new link between two screens is added to that document. The button is not in the row of views: the owner turned that place down
  the same day. A choice inside a view takes the place of the last one in the browser's
  history, so one Back leaves the view, and the view it returns to opens on its choice again.
- **An agent has a page beside its conversation.** A click on a build lead or a worker puts
  its conversation in the left column, as before. The right side used to keep the End
  result, which has nothing to do with that agent. It now shows, in this order: one line of
  facts (harness, model, effort, context, turns, branch), the task it works on with that
  task's sprint, what it changed, and its tasks.
- **A worker's page shows what it changed.** The owner chose this on 2026-11-23. The first
  list is the files it changed and has not committed, each with its status letter and its
  added and removed lines. A file opens its diff under its row, in the unified form. The
  heading carries the total and the branch. The second list is its commits, newest first,
  each with its hash and its lines. A commit opens the Git view.
- **A team lead's page shows its team in place of the changes.** One line for each agent,
  and the button to the team's channel. A lead's tasks are the team's tasks.
- **The agent's page does not repeat the conversation.** The conversation's head names the
  agent, its team and its state, and the conversation holds the agent's open request. The
  page shows none of the four. No tab of the row is marked while an agent's page is open.
- **A run beside the conversation puts its jobs in one row above the steps.** Below 520
  pixels of run width the column of jobs would cut the steps.
- **"Workflow" means the build loop and its replacements, and nothing else.** The build
  loop's settings page keeps the name Workflow.
- **A phase explains itself only when asked.** A step of the strip is a button. It opens
  what the phase does, who runs it, the owner's part, when it ends and what it produces,
  under the strip. Pressing it again closes it. Nothing is open at first.
- **Settings has two pages, Project and Workflow.** The workflow's settings moved here from
  the Workflow view. The pages use the secondary tab row. A setting of the workflow is one
  line: its name, a help mark that holds the explanation, and its control. No sentence
  stands under a setting's name.
- **A review round is a small bar chart.** Each reviewer has one bar with its points over
  it. The round's name and its total stand under the bars. Everything in the tile is
  centred. The tallest bar of any round sets the scale, so rounds can be compared. A clean
  round shows a check. A legend under the tiles names the reviewers.
- **A reviewer has a series colour.** The tokens are `--color-series-1`, `--color-series-2`
  and `--color-series-3`. They mark the parts of a chart and nothing else. They are no
  state's colour, and a state never borrows them.
- **A finding is a row of two lines.** The severity and its points stand first. The text of
  the finding is the first line. Its number, its statement and its reviewer are the second.
  Its state, "Not fixed" or "Fixed", is a word at the right end. The points carry their
  unit, such as "3 pts".
- **A fix sprint's label has its own column.** The columns of that table are 20 pixels
  apart. Whether a fix sprint merges on its own is a word, not a badge.
- **Agent rows in Workflow keep one width for the name.** The activity of every agent
  starts on one line down the list.
- **A picture in a table is whole.** A screen's picture in Mocks is scaled down until all of
  it fits its row, and rows keep 8 pixels above and below. A screen's note stands under its
  name. A component's preview is no taller than a button.


## The command center and a repository's view, decided on 2026-11-24

The owner chose the grid for both command centers on 2026-11-24, out of three layouts of each.

- **A project that is past Define opens on its command center.** A project in Define still
  opens on its end result. The live website still opens the board.
- **A command center is a grid of widgets.** The grid has one column in a narrow pane, two
  from 560 pixels, three from 940 and four from 1280. A widget with a list takes two columns.
  The grid follows the width of its pane, not of the window, because the conversation stands
  beside it.
- **A widget is one status.** It has a small head with its name, and a count when the count
  matters. A name that leads to a view is a link with a chevron. Files, Git and the terminal
  are views. A widget may say one fact of a view and lead to it, such as the latest commits.
  It never holds the view itself.
- **The order goes from what to do to what to know.** A project shows what waits, where it
  stands, who works and the sprints, and then the numbers. A repository shows its projects,
  what waits and the plan, and then the numbers. With every widget shown, the grid has no
  empty cell at two columns or at three.
- **The widget that waits for the owner has a brighter head.** Its name takes the text
  colour while its count is above zero. It has no coloured edge.
- **Two widgets never disagree on a count.** "Open requests" of a repository lists the open
  questions of Define as well, so it adds up to the "Now" column of "Projects" and to the
  sidebar.
- **A dot in a widget has a word beside it.** A row of "Active agents" has no state dot,
  because the list's name says the state.
- **A widget agrees with the view it leads to.** "Active agents" uses the groups of Status.
  The run in "Checks" is the run that puts the dot on the Actions tab.
- **A widget is a tile with one hairline frame.** A tile holds rows and figures. It never
  holds another tile.
- **Text in a widget wraps.** It is never cut with an ellipsis. A long name makes its row
  taller.
- **A widget has no faint text.** Ages, column heads and "None" use the muted colour, which
  passes the contrast check on a tile.
- **A chart has one series, in `--color-series-1`.** Bars are at most 10 pixels wide and 40
  high, on one baseline, and each bar says its value on hover. The two ends of the time axis
  are named under the bars. When a figure compares two windows, the bars of the earlier window
  are paler. A small line shows the last 14 days beside a total. A chart's text keeps the
  text colours.
- **A figure is a name, a number, and how it moved.** The movement is an arrow and words in
  the muted colour. It is not green or red.
- **The phases are eight segments.** A done phase is filled, the current one is filled to
  its middle, and the rest are empty in `--color-line-strong`. The name of the current phase
  stands under them.
- **The "Widgets" menu stands at the end of the row of views.** It shows and hides widgets,
  and the browser remembers the choice. Its count is the number of widgets that show.
- **The row of views fits at 1000 pixels.** Up to 1100 pixels the command center's tab and
  the "Widgets" menu show their icons alone, and the Status tab drops the name of the stage.
  The words stay for a screen reader.
- **One act has one name.** The button that starts a project says "New project", on the
  projects and on a goal of the plan.
- **A repository opens like a project.** The repository agent's conversation is on the
  left, and the row of views is Command center, Board, Design, Files and Git. The Board shows the
  sprints of the projects, and every other view shows the main checkout. There is no separate list of projects, because the command center has one.
- **The repository agent is the first row under its repository in the sidebar.**
- **Status keeps its Overview page.** The command center repeats part of it, and the owner
  decided on 2026-11-24 to keep both.

## Arranging widgets, the lead to talk to, and workflow versions, drafted on 2026-11-24

- **"Arrange the widgets" is in the "Widgets" menu.** While the owner arranges, every tile has
  a dashed frame and four tools in its head: earlier, later, one or two columns, and hide. The
  tile's content steps back and cannot be pressed. One "Done" button takes the menu's place.
  Arranging ends when the owner leaves the command center.
- **The browser remembers the arrangement.** "Reset layout" in the menu restores the
  usual order, the usual widths and every hidden widget.
- **A bar chart can show its numbers.** "Show the numbers" under a chart puts a table in its
  place, newest first, and "Show the bars" brings the chart back.
- **The conversation's button names who it is with.** A small arrow beside it opens a menu of
  the project lead and the lead of every team. The chosen lead stays beside every view until
  the owner chooses another.
- **A team lead takes a message.** Builders, reviewers and test runners stay read only.
- **A workflow shows its version beside its name.** "Version 3" opens a menu of the versions
  kept. Choosing one puts it in the editor as an unsaved change. The save button says the
  version it will make, such as "Save as version 4". The default workflow has no version.
- **A row of the sidebar says its whole name on hover.**

## Adding a repository, where a terminal opens, and stronger faint text, decided on 2026-11-24

- **The plus beside "Repositories" adds a repository.** It opens "Add a repository". A project
  is added from its repository's row, from the repository's command center, and from Home.
- **A repository is added by its link, or from GitHub.** The link is an HTTPS or SSH link.
  Under it, a connected GitHub account lists its repositories, with an account menu and a
  search. A repository that is already added says "Added". "Use a folder on a machine" opens
  the older dialog for a folder that is already cloned.
- **Adding only tracks.** The server remembers the link and clones nothing. The sidebar says
  "Not set up" on the repository's row.
- **"Set up" clones on one machine.** The page of a tracked repository shows its link, its
  state and the folder it will take, with a machine menu and one button, such as "Set up on
  Workstation". When the clone is done, the repository agent starts and writes first.
- **A terminal opens where the agent of the open place works.** A project's terminal opens in
  the project's worktree. A repository's terminal opens in its main checkout, on the machine
  that holds it. On any other page, "Terminal" asks "Open a terminal in" and lists the
  repositories and the machines. The server only passes a session on, so a machine that runs
  nothing but the server is not a place.
- **Faint text reaches 4.5 to 1 on every surface.** `--color-faint` is `#5f6a63` in the light
  theme and `#9b8e99` in the dark theme. The lowest measured pair is 4.57 to 1, on a raised
  surface in the dark theme.
- **A choice of a view is in the address.** The board's search and epic, the open phase of
  Status, the round and filter of Reviews, and the message board's search and filters are
  kept there, so Back returns to them.
- **A run in the Actions list says its state in a word,** and its title is not cut.
- **A screen in the Design table shows a state only when it is not approved.** The line above
  the table says how many are approved.
- **In a narrow window the conversation's button says a short word,** such as "Lead", and an
  agent's page names the agent.

## Who an agent's push appears as on GitHub, decided on 2026-11-24

- **Agents push and open pull requests as the app's bot.** The server reaches GitHub through
  a GitHub App, and its secrets are kept in OpenBao.
- **The owner can choose "Me" instead.** Service settings has a "GitHub" section with the
  connected account, the bot's name, and "Pushes and pull requests appear as": "The bot" or
  "Me".
- **A project can set its own.** The General page of a project's settings has "On GitHub,
  appear as", which starts on the service's setting.

## Settings holds four more pages, and the terminal moved up, drafted on 2026-11-24

- **Machines, Harnesses, Notifications and Workflows are in Settings.** They are the first four
  entries of the list on the left of Settings, above the sections of the service's own settings.
  Each keeps its address, and the top bar says "Settings" before its name.
- **Settings opens from an icon in the sidebar's foot,** between the server's name and the
  log-out button. A dot on it says that a notification failed to arrive.
- **"Terminal" sits under "Find project or agent",** above the repositories.
- **The sidebar's foot keeps "Usage" and the server's row.**

## A repository has a board, drafted on 2026-11-24

The owner asked on 2026-11-24 for the board on a repository as well.

- **"Board" is the second view of a repository,** after the command center. The row of views
  is Command center, Board, Design, Files and Git.
- **The board holds the sprints of every project of the repository.** Archived projects are
  left out. It has the same three views as a project's board: Board, Table and Roadmap.
- **A card names its project where a project's card names its epic.** The Table has a
  "Project" column in place of "Epic". The Roadmap has one row for each project, with its
  sprints under it, and no goal row.
- **The filter is by project.** The filter and the search are kept in the address.
- **A card opens its sprint in its own project.** Back returns to the repository's board.
- **Focus works as on a project's board.**

## A repository's Design view lists every drawn screen, drafted on 2026-11-24

The owner asked on 2026-11-24 for the drawn screens in a repository's Design view.

- **"Screens" is the first page of a repository's Design view.** The row of pages is Screens,
  Design file, Browser and Architecture. The Design view opens on Screens, as a project's does.
- **A repository lists every screen that is drawn in its main checkout.** A project lists only
  the screens it draws or changes.
- **The list has no status and no comments.** A screen in the main checkout is already
  approved. The column "Changing in" names each project that has an open change to the screen,
  and the name opens that project's drawing of it.
- **An open screen shows its drawing, its note and "Changing in".** It has no approval, no
  comments and no "Edit design", because a screen is changed in a project.
- **The page keeps the name "Screens",** which is the name of the same page in a project.
- **The design file moved to its own address,** `/r/<repository>/design/file`. An older
  address of a part of the design file still opens it.

## One identifier for every sprint and task, decided on 2026-11-24

The owner decided on 2026-11-24 that no two sprints or tasks of a server share an identifier.

- **An identifier has three parts,** such as "LS-A01-12". "LS" is the repository's letters, "A01"
  is the project's code, and "12" is a number.
- **A repository has two letters,** taken from its name. `lgtm-studio` is "LS".
- **A project has a code of one letter and two digits, given in order.** The first project of
  a repository is "A01", the next is "A02", and "B01" follows "A99". The owner decided this on
  2026-11-24. A project gets its code when it is created, and a new name does not change it.
  One letter and two digits name 2,574 projects in a repository.
- **A project counts its sprints and its tasks with one counter.** No sprint is "SP-01" and no
  fix sprint is "FX-01" any more. The sprint's own page still says "Fix sprint".
- **A sprint or task that a repository makes outside a project has the repository's letters
  and a number,** such as "LS-12". None is drawn yet.
- **A project's settings show its code** in the row "Identifiers", such as "LS-A01".

## The "Add a repository" dialog, Connections, and the terminal's plus, drafted on 2026-11-24

On 2026-11-24 the owner requested a better "Add a repository" dialog, drawn after screens on Mobbin, and for a place in Settings to manage what is connected.

- **The dialog has one field.** It searches the repositories of the chosen account. A pasted
  HTTPS or SSH link becomes the one row of the list, and Enter adds it. The "New project" page
  of Vercel has one field for both in the same way.
- **The connected accounts are a row of choices under the field.** A plus beside them connects
  another account. The count of repositories is at the right end of the row.
- **A row is one repository.** A lock marks a private one. A repository the server already
  has says "Open" where the others say "Add".
- **A server with no GitHub account shows "GitHub is not connected" and "Connect GitHub"** in
  the place of the list.
- **The foot holds "Connections", "Use a folder on a machine" and "Close".** "Connections"
  opens that section of Settings.
- **"Connections" is a section of Settings.** It took the place of the section "GitHub". It
  lists each connected account in a table, after the Connections pages of Linear and Lindy:
  the account, its kind, what the server may read, and how many of its repositories are on the
  server. "Remove" asks first and says what stays. The bot's name and "Pushes and pull
  requests appear as" are under the table.
- **The terminal's "New session" is a plus in the row of sessions.** Its name is a tooltip.
  Everything beside the tabs sits in the middle of the row's height. The button of an empty
  terminal still says "New session".

## A heading is a noun label, decided on 2026-11-24

The owner asked on 2026-11-24 for an audit of the wording, and for conversational headings to go. The owner's example was "Where it went" on the Usage page, which is now "Breakdown".

- **A heading, a column head, a tab, a widget title and the title of an empty view is a noun
  label.** It is not a clause, a question, or a sentence to the reader.
- **A label does not say "you" or "your".** "Needs you" is "Open requests", and "Your part" is
  "Owner input".
- **An empty view says "No" and the thing.** "Nobody is working." is "No active agents", and
  no empty view ends in "yet".
- **A sentence stays a sentence where it reports an event or a consequence.** A toast, the
  body of a confirmation, and a message from an agent are not labels.

These are the names that changed.

| Where | Was | Is |
|---|---|---|
| Usage | "Where it went", "Show where it went" | "Breakdown", "Show breakdown" |
| Usage | "At API prices", "Share of the row above", "Share of this week" | "API price", "Share of parent", "Weekly share" |
| Sidebar, bell, Home | "Nothing needs you", "3 need you" | "No open requests", "3 open requests" |
| Home | "Not now", "Answered or cancelled" | "Dismiss", "Closed" |
| Command center | "Needs you", "Working now", "Sprints under way", "Tasks finished" | "Open requests", "Active agents", "Active sprints", "Completed tasks" |
| Command center | "Arrange the widgets", "Put everything back", "Used of the limit" | "Arrange widgets", "Reset layout", "Limit used" |
| Board card | "Needs your answer", "Needs your approval", "Needs your merge" | "Awaiting answer", "Awaiting approval", "Awaiting merge" |
| Sidebar | "Needs answer", "Needs approval", "Needs merge" | "Question", "Approval", "Merge" |
| Every empty view | "No commits yet", "No channels yet", "Nothing posted here yet" and the like | "No commits", "No channels", "No posts" |
| End result | "What will exist", "Left out", "Something else", "Change asked", "Your note" | "Statements", "Out of scope", "Other", "Change requested", "Note" |
| Agents | "Needs you", "Waiting on you", "Doing now", "Tokens so far" | "Waiting for input", "Waiting for input", "Activity", "Tokens used" |
| Agent thread | "Working on", "Sprints in flight", "Runs as", "This run" | "Current task", "Active sprints", "Configuration", "Session" |
| Message board | "Nobody has picked this up.", "Talk to the project lead" | "Unclaimed", "Message the project lead" |
| Status | "What happens", "Run by", "Your part", "Ends", "Will produce", "Produced" | "Description", "Agents", "Owner input", "Exit condition", "Planned outputs", "Outputs" |
| Status | "Why it was added", "On its own", "Needs your approval", "What was built" | "Reason", "Automatic", "Approval required", "Deliverables" |
| Stuck statement | "Change it", "Keep it", "Drop it" | "Rewrite", "Keep", "Drop" |
| Screens | "What should change", "What changed", "When" | "Requested change", "Change", "Date" |
| Requests | "The question", "Your answer", "You answered", "What should change?" | "Question", "Answer", "Answer", "Requested change" |
| Git | "Level with", "See where the project is", "Waiting for your merge." | "Up to date with", "Project status", "Awaiting merge." |
| Settings | "When an agent may be stuck", "When a subscription or an agent fails", "Where it is installed" | "Stall detection", "Failure handling", "Install location" |
| Settings, notifications | "An agent asks you a question", "A merge is ready" and twelve more | "Question from an agent", "Merge ready" and twelve more |
| Settings, section lines | "How this page looks on this browser." and seven more | "Theme, density and sidebar." and seven more |
| Merge review | "Users will notice:" | "User-visible change:" |

## A screen holds no instruction, applied on 2026-11-24

The owner asked on 2026-11-24 for the instruction sentences to go, after the audit of the headings.

- **An empty view is its title and, at most, one link or button.** "Choose a file" with "Its
  diff opens here." is "No file selected". "No commits" has no sentence under it.
- **A sentence that explained how the product works is gone from the screen.** The Agents view
  no longer says when the stall watch flags an agent. The empty terminal no longer says what a
  session is. The Machines page no longer says what a master and a worker do.
- **A link beside a removed sentence stays.** "No worktrees" keeps "Project status", and "This
  sprint does not exist" keeps "Back to the board".
- **A page that does not exist no longer guesses why.** "It may have been removed." is gone.
- **The help behind a setting's "About" button stays.** It opens only when the owner asks.
- **A step of the first-run setup is its heading and its fields.** The sentence under
  "Connect a subscription" and the one under "Add a repository" are gone. The field says
  "Repository folder on Workstation", so the machine is still named. The line under the
  card says only "Step 2 of 3".
- **Three more sentences went the same day, when the owner named them.** The sign-in step no
  longer says where the harness signs in or that LGTM Studio never sees the password. The
  dialog that adds a folder no longer says that the folder must be a git repository. The
  Workflows page no longer says that projects still run the default workflow.
- **The steps of a procedure stay,** such as pairing a machine and signing in to a harness.
- **A sentence that says what a choice does stays,** such as the body of a confirmation and the
  line under "Rewrite", "Keep" and "Drop".

## The first-run list of harnesses, drafted on 2026-11-24

The owner asked on 2026-11-24 for the first step of the first-run setup to list the harnesses found on the machine, to sign in to them, and to install another. A server may be a production machine with no harness on it, so the step does not list models.

- **The step is named "Harnesses",** in the row of steps and in its heading.
- **A row is one harness found on the machine.** It shows the name, the version and "Sign in".
- **"Sign in" opens the sign-in code under its row.** "Cancel" takes the place of "Sign in" while it waits. A signed-in row says "Signed in" in green.
- **"Add harness" is under the list.** It opens the supported harnesses that are not on the machine, each as "Install" and its name. A chosen one joins the list as "Installing…" and then shows its version.
- **"Add harness" is gone when every supported harness is on the machine.**
- **A machine with no harness says "No harness detected"** in the place of the rows.

## "Add harness" asks first and lists nine harnesses, drafted on 2026-11-24

The owner asked on 2026-11-24 for the popular harnesses to be offered, and agreed that "Add harness" shows its commands and asks before it runs them. `docs/research/harnesses.md` records how the nine were chosen.

- **"Add harness" opens the supported harnesses that are not on the machine.** Each row is the harness's name.
- **A chosen harness opens the dialog "Install" and its name.** Nothing runs until the owner presses "Install on" and the machine's name.
- **The dialog states five facts:** the version, the publisher's page, the licence, the checksum and the command that starts the harness.
- **"Checksum" says "Not published" in the warning colour** when the publisher gives none. Cursor is the one such harness today.
- **The dialog shows the commands the server runs,** in a box that can be copied.
- **"Add harness" is also on the Harnesses page of Settings,** beside "Check again".
- **An installed harness joins the list with no account.** It has no models until an account is signed in.

## Where the redesign stopped, on 2026-11-24

The owner said on 2026-11-24 that the first-run setup looks good, and that this is a good place to stop the redesign. It is not a freeze. Other processes will do the rest of the work.

- **The first-run setup is in the state the owner liked:** the steps "Harnesses", "Repository" and "First project", with no helper sentences.
- **The sign-in shown on the step "Harnesses" is a demonstration.** Issue 1630 records that real harnesses sign in differently.
- **The work that is left is in `docs/issues.md`, issues 1625 to 1630,** and in the newest sections of `docs/features.md`.
- **Everything is on sample data in `src/frontend/`.** The contract and the live website are unchanged.
