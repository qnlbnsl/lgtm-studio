# Website feature checklist

This file lists everything a person can do on the website, so that no feature is forgotten during the Go rewrite.
The owner asked for it on 2026-11-27.
A line is ticked when the feature works against the Go server, or when it needs no server at all.

## How this list is kept

- The list was made on 2026-11-27 by reading the source of the website in `src/frontend/src/`. It has 247 lines.
- "When" names the sprint that makes a line real. The sprints are in the design [Running an agent](specs/2026-11-27-running-an-agent-design.md).
- A line with two sprints is ticked when its last part is real.
- "After sprint 10" names a later design that the design for running an agent already lists.
- "Not assigned yet" means that no design names the feature. Such a line needs a decision of the owner before it can be planned.
- "Old engine only" means that only the mode of the old TypeScript engine reaches the feature.
- When a sprint passes its checks, its lines are ticked in the commit that closes the sprint.
- When a feature is added to the website, a line is added here.

## Counts on 2026-11-27

| State | Lines |
|---|---:|
| Real today | 13 |
| Works in the browser, needs no server | 24 |
| Sample mode only, by design | 3 |
| Sprint 4 | 1 |
| Sprint 4, first part of the line | 3 |
| Sprint 5 | 7 |
| Sprint 5, first part of the line | 6 |
| Sprint 6 | 1 |
| Sprint 6, first part of the line | 2 |
| Sprint 7 | 5 |
| Sprint 7, first part of the line | 5 |
| Sprint 9 | 13 |
| Sprint 9, first part of the line | 1 |
| Sprint 10 | 1 |
| After sprint 10, in a later design that is already named | 81 |
| Not assigned yet | 73 |
| Old engine only | 8 |
| Total | 247 |

## Signing in and connecting to the server

- [x] Connect to a server, see validation and connection errors, and retry the probe.
      Where: Connect to a server. When: real today.
- [x] Use sample data through the in-page server.
      Where: Connect to a server. When: sample mode only, by design.
- [x] Change the remembered server on a build that allows an address choice.
      Where: Sign in to the server. When: works in the browser, needs no server.
- [x] Sign in with the server password and see rejection, expiration, or service availability.
      Where: Sign in to the server. When: real today.
- [x] Show or hide the password and see caps-lock and insecure-connection warnings.
      Where: Sign in to the server. When: works in the browser, needs no server.
- [x] Log out and return to the sign-in screen.
      Where: Navigation. When: real today.
- [x] See the connected server and the connection state as the machine watch runs or retries.
      Where: Navigation, Connection. When: real today.
- [ ] Mark the connection as restored from the connection menu.
      Where: Connection. When: not assigned yet.

## Sidebar and navigation

- [x] Open Home, All projects, Usage, and Settings from the sidebar.
      Where: Navigation. When: works in the browser, needs no server.
- [ ] Open a repository, project, team, or agent from the expandable tree and see its stage, status, or request count.
      Where: Navigation → Repositories. When: sprint 4 for repositories, sprint 5 for projects and leads, later for teams and request counts.
- [ ] Limit the tree to projects with requests you can answer.
      Where: Navigation → Repositories. When: sprint 7.
- [x] Pin or unpin projects for quick access in the folded sidebar.
      Where: Navigation → Repositories, Pinned projects. When: works in the browser, needs no server.
- [x] Fold, resize, or peek at the sidebar, or open navigation as a drawer on a phone.
      Where: Navigation. When: works in the browser, needs no server.
- [ ] Find and open a repository, project, agent, or page by keyboard or search.
      Where: Find project or agent. When: sprint 4 for repositories, sprint 5 for projects and leads.
- [x] Navigate through breadcrumbs and use Skip to content to move keyboard focus.
      Where: Where you are, Skip to content. When: works in the browser, needs no server.
- [x] Open recent destinations through the browser’s back and forward controls, including older addresses that redirect.
      Where: Project views, Home, Harnesses. When: works in the browser, needs no server.
- [ ] See the oldest answerable requests and jump to one from the bell.
      Where: Open requests. When: sprint 7.
- [x] Hide, show, or resize the right-hand list in supported views.
      Where: Questions, Message board, Files, Git. When: works in the browser, needs no server.
- [x] Recover from an unknown page or a missing project, repository, team, sprint, task, or agent by following its return link.
      Where: This page does not exist, other missing-item views. When: works in the browser, needs no server.

## Machines

- [x] See registered machines, their role, online count, connection state, address, and last contact.
      Where: Settings → Machines. When: real today.
- [x] See central processing unit load, memory use, hardware, workspace, and client version.
      Where: Settings → Machines → This machine. When: real today.
- [x] See scanned harness availability and versions.
      Where: Settings → Machines → Harnesses. When: real today.
- [ ] See how many sample accounts are signed in on a machine.
      Where: Settings → Machines → Harnesses. When: sprint 9.
- [ ] See sample repository checkouts, project worktrees, and running agents for a machine.
      Where: Settings → Machines → Repositories, Worktrees and agents. When: sprint 4 for folders, sprint 5 for running agents, sprint 10 for clones.
- [ ] See how many sample agents run on a machine and whether it is at capacity.
      Where: Settings → Machines. When: sprint 5.
- [ ] Change the displayed machine’s agent limit.
      Where: Settings → Machines → This machine. When: not assigned yet.
- [x] Pair a machine using an expiring one-use code and confirm its identity and display name.
      Where: Add a machine. When: real today.
- [x] Cancel pairing and revoke its unused code.
      Where: Add a machine. When: real today.
- [x] Rename a registered machine.
      Where: Settings → Machines → Rename. When: real today.
- [x] Request another harness scan.
      Where: Settings → Machines → Scan harnesses again. When: real today.
- [x] Request a reconnect for an offline machine and see any refusal.
      Where: Settings → Machines → Try to reconnect. When: real today.
- [x] Remove a worker machine after reviewing the affected projects, agents, and accounts.
      Where: Remove machine. When: real today.

## Harnesses, subscriptions, and usage

- [ ] See supported harnesses, installation status, version, executable path, and available models.
      Where: Settings → Harnesses → Program. When: sprint 9.
- [ ] Check the harness inventory again or add a supported harness with its installation instructions.
      Where: Settings → Harnesses, Add harness. When: sprint 9 for the inventory; installing a harness is not assigned yet.
- [ ] Set the starting harness, model, and effort for new project leads.
      Where: Settings → Harnesses → Defaults for new project leads. When: sprint 9.
- [ ] Set the run mode and default model and effort for a harness.
      Where: Settings → Harnesses → Defaults for this harness. When: sprint 9.
- [ ] Add an account with a name and login folder and see installation and sign-in instructions.
      Where: Add account. When: sprint 9.
- [ ] Open the account sign-in terminal, see its output, and send input.
      Where: Add account, Sign in. When: after sprint 10, with terminals.
- [ ] Sign an account in or out, or check whether it is still signed in.
      Where: Settings → Harnesses → Accounts, Sign in. When: sprint 9.
- [ ] Enable or disable an account for starting agents.
      Where: Settings → Harnesses → Accounts. When: sprint 9.
- [ ] Choose the account preferred for new agents on its machine.
      Where: Settings → Harnesses → Accounts. When: sprint 9.
- [ ] Choose an account’s model for new agents.
      Where: Settings → Harnesses → Accounts. When: sprint 9.
- [ ] Rename or remove an account after seeing its affected agents.
      Where: Settings → Harnesses → Accounts. When: sprint 9.
- [ ] Clear an account’s reported usage restriction.
      Where: Settings → Harnesses → Accounts. When: old engine only, no place in the rewrite yet.
- [ ] See account identity, machine, sign-in state, limits, and agents using the account.
      Where: Settings → Harnesses → Accounts. When: sprint 9.
- [ ] See usage totals and estimated application programming interface prices by time period.
      Where: Usage. When: not assigned yet.
- [ ] See account usage shares and limit reset times.
      Where: Usage → Accounts. When: not assigned yet.
- [ ] Drill into usage by project, stage, account, or agent.
      Where: Usage → Breakdown. When: not assigned yet.
- [ ] Inspect a usage finding, its cause, and the breakdown behind it.
      Where: Usage → Findings. When: not assigned yet.
- [ ] Apply a proposed usage guardrail to one repository or all repositories.
      Where: Usage → Findings → Guardrail. When: not assigned yet.
- [ ] Dismiss, list, or reopen handled usage findings.
      Where: Usage → Findings. When: not assigned yet.

## Repositories

- [ ] Browse and search repositories from connected GitHub accounts, including public and private entries.
      Where: Add a repository. When: after sprint 10, with following a repository by its remote.
- [ ] Track a repository by selecting it or pasting a Hypertext Transfer Protocol Secure (HTTPS) or Secure Shell (SSH) link.
      Where: Add a repository. When: after sprint 10, with following a repository by its remote.
- [ ] Add a repository folder on an online machine after seeing what the machine found in it.
      Where: Add repository, Add to a machine. When: sprint 4.
- [ ] Set up a tracked repository on a machine and get a repository agent.
      Where: Repository setup. When: after sprint 10, with the agent of a repository.
- [ ] Stop tracking a repository that has yet to be set up.
      Where: Repository setup. When: after sprint 10, with following a repository by its remote.
- [ ] Talk to the repository agent beside the main checkout’s views.
      Where: Repository agent. When: after sprint 10, with the agent of a repository.
- [ ] See projects, open requests, checkouts, and base-branch activity for a repository.
      Where: Repository → Command center. When: sprint 5 for the list of projects; the rest is not assigned yet.
- [ ] See the repository plan and start a project from an open goal.
      Where: Repository → Command center → Plan. When: after sprint 10, with the agent of a repository.
- [ ] See repository trends for merged sprints, full runs, and usage by project.
      Where: Repository → Command center. When: not assigned yet.

## Projects and the Define stage

- [ ] Search, filter, and open projects, including projects with open requests and archived projects.
      Where: All projects. When: sprint 5.
- [ ] See each project’s stage, activity, and open-request status.
      Where: All projects. When: sprint 5.
- [ ] Create a project from a goal on a chosen machine and use an existing checkout or name a new repository.
      Where: New project. When: sprint 5, with an existing folder or a new repository by name.
- [ ] Choose the lead’s account, model, and effort before starting the project.
      Where: New project → Project lead. When: sprint 5 for model and effort, sprint 9 for the account.
- [ ] See the project get a name, branch, worktree, and initial reply.
      Where: End result, Project lead. When: sprint 5 for the name and the first reply, sprint 10 for the branch and the clone.
- [ ] Discuss the goal and see the lead turn replies into a draft with questions, statements, and next steps.
      Where: End result, Project lead. When: after sprint 10, with tasks and the build loop.
- [ ] Agree to the draft and continue to design or planning.
      Where: End result. When: after sprint 10, with tasks and the build loop.
- [ ] See open requests and active agents with links to their work.
      Where: Project → Command center. When: sprint 5 for the lead, sprint 7 for requests.
- [ ] See phase progress, active sprints, and completed-task throughput.
      Where: Project → Command center. When: after sprint 10, with tasks and the build loop.
- [ ] See checks, usage, and recent commits.
      Where: Project → Command center. When: after sprint 10, with tasks and the build loop.
- [x] Hide, reorder, resize, and restore the project or repository command-center widgets.
      Where: Command center → Widgets. When: works in the browser, needs no server.

## Conversation with an agent

- [x] Show, hide, or resize the conversation, and switch between it and the selected view on narrow screens.
      Where: Project lead, Repository agent. When: works in the browser, needs no server.
- [ ] Switch between the project lead and another team lead, or return from another agent’s conversation.
      Where: Project lead, Agent thread. When: after sprint 10, with teams.
- [ ] See messages, attributed replies, collapsed activity and tool output, request cards, and work in progress.
      Where: Project lead, Agent thread. When: sprint 5.
- [ ] Open oversized tool output and jump to the latest conversation activity.
      Where: Agent thread. When: sprint 6.
- [ ] Send a message and receive a reply.
      Where: Project lead, Repository agent, Agent thread. When: sprint 5.
- [x] Keep a text draft while switching conversations or reloading.
      Where: Message composer. When: works in the browser, needs no server.
- [ ] Add or remove task and file references for a message.
      Where: Message composer → Add a task or a file. When: after sprint 10, with tasks and the build loop.
- [ ] Attach files or images, choose an original or smaller image, and open a sent image.
      Where: Message composer, Agent thread. When: after sprint 10, with files and images attached to a message.
- [ ] Change the conversation’s subscription, model, and effort and see context usage.
      Where: Message composer. When: sprint 6 for context usage, sprint 9 for a change of account, model and effort.
- [ ] Stop a reply in progress.
      Where: Message composer. When: sprint 5.
- [ ] Stop an agent or resume a stopped agent.
      Where: Message composer, Agent actions. When: sprint 5 to stop, sprint 6 to resume.
- [ ] Read a worker’s conversation and go to its lead when direct replies are disabled.
      Where: Agent thread. When: after sprint 10, with teams.
- [ ] Open the standalone Chat view and see the agent’s task, configuration, or peers.
      Where: Chat. When: old engine only, no place in the rewrite yet.
- [ ] Compact an agent’s conversation context.
      Where: Chat → Agent actions. When: sprint 6 for compaction; the old view itself has no place in the rewrite yet.

## Questions and requests

- [ ] Search, filter, and sort requests by type, project, repository, and waiting or answered state.
      Where: Home → Needs attention. When: sprint 7 for approvals; questions of agents are not assigned yet.
- [ ] See waiting and answered questions and approvals for a project.
      Where: Questions. When: sprint 7 for approvals; questions of agents are not assigned yet.
- [ ] Accept a recommendation from its request card or open the full question to choose a different answer.
      Where: Needs attention, Agent thread. When: sprint 7 for approvals; questions of agents are not assigned yet.
- [ ] See the recommendation, evidence, source, and work a question or approval blocks.
      Where: Questions, Needs attention. When: sprint 7 for approvals; questions of agents are not assigned yet.
- [ ] Answer a question with suggested choices, your own text, or a supporting note where offered.
      Where: Questions, Waiting, Needs attention. When: not assigned yet.
- [ ] Allow or deny an approval once and inspect or copy the command and folder.
      Where: Questions, Waiting, Needs attention. When: sprint 7.
- [ ] Choose an always-allow or always-deny decision for a project approval.
      Where: Questions, Waiting, Needs attention. When: not assigned yet.
- [ ] Open the asking agent’s thread to discuss a question or approval.
      Where: Questions, Needs attention. When: sprint 7 for approvals; questions of agents are not assigned yet.
- [ ] See open items in the lead’s Waiting dock and see held or answered items in the transcript.
      Where: Project lead → Waiting. When: sprint 7.
- [ ] See evidence for a merge request and approve it or send it back with a note.
      Where: Review merge, Needs attention, Git. When: not assigned yet.
- [ ] See a failed start, run the sign-in flow, or resume once the account is ready.
      Where: Needs attention, Agent thread, Sign in. When: sprint 5 to see the failed start, sprint 6 to resume, sprint 9 to sign in.

## Teams and other agents

- [ ] See teams, their branch and lead, and their agents and assignments.
      Where: Team. When: after sprint 10, with teams.
- [ ] See an agent’s current work, team, tasks, uncommitted changes, and commits.
      Where: Agent conversation → Working on, Team, Tasks. When: after sprint 10, with teams.
- [ ] See working, waiting, and idle agents and open their conversations from the activity list.
      Where: Status → Overview → Agents. When: after sprint 10, with teams.
- [x] Open Git or the terminal from another agent’s actions menu.
      Where: Agent thread → Agent actions. When: works in the browser, needs no server.
- [ ] Compare agents by situation or table and filter by team.
      Where: Agents. When: old engine only, no place in the rewrite yet.
- [ ] Nudge an agent to ask for a status update.
      Where: Agents. When: old engine only, no place in the rewrite yet.
- [ ] Change an agent’s permission or model from the agent roster.
      Where: Agents. When: old engine only, no place in the rewrite yet.

## Tasks and the board

- [ ] See sprints as a board, sortable table, or dependency roadmap.
      Where: Project or repository → Board. When: after sprint 10, with tasks and the build loop.
- [ ] Filter sprints by text and epic or by repository project.
      Where: Board. When: after sprint 10, with tasks and the build loop.
- [ ] See sprint priority, phase, team, task progress, and lane requirements.
      Where: Board. When: after sprint 10, with tasks and the build loop.
- [ ] Reorder sprint cards or move them between lanes when requirements allow.
      Where: Board. When: after sprint 10, with tasks and the build loop.
- [x] Put the board in focus mode to use the full window.
      Where: Board. When: works in the browser, needs no server.
- [ ] See a sprint’s goal, team, dependencies, linked statements, and evidence.
      Where: Sprint → About this sprint. When: after sprint 10, with tasks and the build loop.
- [ ] See and act on outstanding requests attached to a sprint.
      Where: Sprint → Requests. When: after sprint 10, with tasks and the build loop.
- [ ] Move a sprint from its detail page and confirm any override of unmet requirements or team shutdown.
      Where: Sprint. When: after sprint 10, with tasks and the build loop.
- [ ] Read the sprint plan and jump from a task to its part of the plan.
      Where: Sprint → Plan, Tasks. When: after sprint 10, with tasks and the build loop.
- [ ] See task status, assignment, and points.
      Where: Sprint → Tasks. When: after sprint 10, with tasks and the build loop.
- [ ] Add a task to an open sprint.
      Where: Sprint → Tasks. When: after sprint 10, with tasks and the build loop.
- [ ] Read a task’s work and handoff history and open its changed files or related commits.
      Where: Sprint → Tasks → Task. When: after sprint 10, with tasks and the build loop.

## End result and statements

- [ ] See the goal, draft or agreement state, numbered statements, exclusions, and next steps.
      Where: End result. When: after sprint 10, with tasks and the build loop.
- [ ] Inspect a statement’s previous wording, delivery status, evidence, and related questions when available.
      Where: End result → Statements. When: after sprint 10, with tasks and the build loop.
- [ ] Reply to the lead about a specific statement.
      Where: End result → Statements, Project lead. When: after sprint 10, with tasks and the build loop.
- [ ] See active teams for the end result and follow their work.
      Where: End result → Teams. When: after sprint 10, with teams.
- [ ] Open the finished-page or pull-request shortcut from the Hand over phase.
      Where: Status → Overview → Hand over. When: after sprint 10, with tasks and the build loop.
- [ ] See the finished-page deliverables, evidence, open items, and pull request.
      Where: Status → Hand over → Finished page. When: old engine only, no place in the rewrite yet.

## Status, Actions, and workflows

- [ ] See build-loop phases, timing, and outputs for each phase.
      Where: Status → Overview. When: after sprint 10, with tasks and the build loop.
- [ ] See current checks, workflow gates, and fix sprints with their reasons and progress.
      Where: Status → Overview. When: after sprint 10, with tasks and the build loop.
- [ ] See a stuck statement and decide whether to keep, change, or drop it.
      Where: Status → Overview, Reviews. When: after sprint 10, with tasks and the build loop.
- [ ] Choose a plan or product review and see its verdict and reviewer scores across rounds.
      Where: Status → Reviews. When: after sprint 10, with tasks and the build loop.
- [ ] See the findings in a review round, their points and outcomes, and references to statements or evidence.
      Where: Status → Reviews. When: after sprint 10, with tasks and the build loop.
- [ ] See the failed-full-run indicator and filter or select past runs by action.
      Where: Actions. When: not assigned yet.
- [ ] Run a check, build, or full test action and see it progress.
      Where: Actions → Run action. When: not assigned yet.
- [ ] Inspect run outcomes, commit, timing, test counts, jobs, steps, and logs.
      Where: Actions → Run. When: not assigned yet.
- [ ] Open an action definition in Files.
      Where: Actions → Actions. When: not assigned yet.
- [ ] Browse workflow definitions, their scope, and their phase and block diagrams.
      Where: Settings → Workflows. When: not assigned yet.
- [ ] Clone a workflow as a repository or service-wide override.
      Where: Workflows → Clone workflow. When: not assigned yet.
- [ ] Add, rename, reorder, or remove phases and blocks, and set each block’s role and steps.
      Where: Workflows → Phase, Block. When: not assigned yet.
- [ ] Add or remove moves and their conditions between workflow blocks.
      Where: Workflows → Moves from this block, Add a move. When: not assigned yet.
- [ ] Edit a workflow’s settings and see validation errors before saving.
      Where: Workflows → Settings. When: not assigned yet.
- [ ] Save a workflow version, discard edits, or restore a past version.
      Where: Workflows → History. When: not assigned yet.
- [ ] Delete a workflow override.
      Where: Workflows. When: not assigned yet.

## Design, mockups, and previews

- [ ] See mockup thumbnails, revisions, approval status, and unseen changes.
      Where: Design → Screens. When: not assigned yet.
- [ ] See which projects are changing a screen in the repository’s main design.
      Where: Repository → Design → Screens. When: not assigned yet.
- [ ] Open a mockup, step through screens, and compare phone and desktop views.
      Where: Design → Screens → Screen. When: not assigned yet.
- [x] Pan, zoom, or fit a mockup, flow, or design sheet on the canvas.
      Where: Design → Screens, Design file. When: works in the browser, needs no server.
- [ ] Read comments and send feedback on a mockup or the whole design.
      Where: Design → Comments, Request changes. When: not assigned yet.
- [ ] Approve a screen or approve the whole design and advance to planning.
      Where: Design → Screens. When: not assigned yet.
- [ ] Open a mockup or the repository design file in OpenPencil.
      Where: Design → Screen, Design file. When: not assigned yet.
- [ ] Compare previous and revised mockups and mark changes as seen.
      Where: Design → Changes. When: not assigned yet.
- [ ] See mockup components, their visual examples, and the screens that use them.
      Where: Design → Components. When: not assigned yet.
- [ ] Filter mockup fields by schema match and see sample values and usage.
      Where: Design → Data. When: not assigned yet.
- [ ] Create a repository design file and see its path and last change.
      Where: Design → Design file. When: not assigned yet.
- [ ] Browse screen flows and inspect frames for different product states.
      Where: Design → Design file → Screens. When: not assigned yet.
- [ ] Browse component variants and compare design values with the website’s palette, type, space, shape, motion, and icons.
      Where: Design → Design file → Components, Palette. When: not assigned yet.
- [ ] Choose a preview and see its state, command, branch, machine, port, or error.
      Where: Design → Browser. When: not assigned yet.
- [ ] Start or stop a product preview.
      Where: Design → Browser. When: not assigned yet.
- [ ] View a running preview in a frame, reload it, or open it in a new tab when available.
      Where: Design → Browser. When: not assigned yet.
- [x] Switch the preview between Phone, Tablet, and Full sizes.
      Where: Design → Browser. When: works in the browser, needs no server.
- [x] Toggle the element picker.
      Where: Design → Browser → Pick an element. When: works in the browser, needs no server.
- [ ] Refresh or open a recorded design and its editor.
      Where: Design → Refresh the design, Edit design. When: old engine only, no place in the rewrite yet.

## Files

- [ ] Choose a worktree and browse or search its files and folders.
      Where: Project or repository → Files. When: after sprint 10, with the views Files and Git.
- [ ] See changed, unsaved, read-only, and offline states for files.
      Where: Files. When: after sprint 10, with the views Files and Git.
- [ ] Open text or images and see file metadata or the binary-file notice.
      Where: Files → Open file. When: after sprint 10, with the views Files and Git.
- [ ] Edit and save a text file in a writable checkout.
      Where: Files → Open file. When: after sprint 10, with the views Files and Git.
- [ ] Commit a saved changed file with a commit message.
      Where: Files → Commit. When: after sprint 10, with the views Files and Git.
- [x] Wrap long lines or copy a file’s path.
      Where: Files → More. When: works in the browser, needs no server.
- [x] Go from a changed file to Git.
      Where: Files → Open in Git. When: works in the browser, needs no server.
- [ ] Request a file download.
      Where: Files → More → Download. When: after sprint 10, with the views Files and Git.

## Git

- [ ] Choose a checkout and see its branch, machine, owner, and ahead or behind counts.
      Where: Project or repository → Git. When: after sprint 10, with the views Files and Git.
- [ ] See whether a project branch is merged and how the merge was delivered.
      Where: Git. When: after sprint 10, with the views Files and Git.
- [ ] See staged and unstaged changes, file status, and line counts.
      Where: Git → Changes. When: after sprint 10, with the views Files and Git.
- [ ] Read changed lines in unified or split view.
      Where: Git → Changes, History. When: after sprint 10, with the views Files and Git.
- [ ] Stage or unstage one file or all files.
      Where: Git → Changes. When: after sprint 10, with the views Files and Git.
- [ ] Discard one file’s unstaged changes or all unstaged changes.
      Where: Git → Changes. When: after sprint 10, with the views Files and Git.
- [ ] Generate a commit message from staged changes or enter one yourself, then commit.
      Where: Git → Changes. When: after sprint 10, with the views Files and Git.
- [ ] Inspect commit history, authors, refs, changed files, and diffs.
      Where: Git → History. When: after sprint 10, with the views Files and Git.
- [x] Copy a commit hash.
      Where: Git → History. When: works in the browser, needs no server.
- [ ] Update a worktree from its base branch.
      Where: Git → Update from base. When: after sprint 10, with the views Files and Git.
- [ ] Integrate a team branch into its parent and see any blockers.
      Where: Git → Integrate. When: after sprint 10, with the views Files and Git.
- [ ] See conflicted files and compare your version, the base, their version, and the result.
      Where: Git → Conflicts. When: after sprint 10, with the views Files and Git.
- [ ] Choose a resolution for a conflict, mark a file resolved, then continue or abort the merge.
      Where: Git → Conflicts. When: after sprint 10, with the views Files and Git.
- [ ] Prepare a project merge review with test, review, and change evidence.
      Where: Git → Review merge. When: after sprint 10, with the views Files and Git.
- [ ] Refresh Git data or fetch, pull, and push a worktree.
      Where: Git. When: old engine only, no place in the rewrite yet.

## Terminals

- [ ] Open or close a terminal beside the project or repository, or choose a repository or machine elsewhere.
      Where: Navigation → Terminal, Open a terminal in. When: after sprint 10, with terminals.
- [ ] Create, switch, rename, or close terminal sessions in available worktrees.
      Where: Terminal → Terminal sessions. When: after sprint 10, with terminals.
- [ ] See the shell prompt, directory, branch, running command, and output.
      Where: Terminal. When: after sprint 10, with terminals.
- [ ] Run sample shell, file, Git, harness, and test commands and interrupt a running command.
      Where: Terminal. When: after sprint 10, with terminals.
- [ ] Use command history, completion, clipboard input, and phone keys.
      Where: Terminal → Extra keys. When: after sprint 10, with terminals.
- [ ] Open and edit a sample file in the simulated terminal editor.
      Where: Terminal. When: after sprint 10, with terminals.
- [ ] Paste a command from an agent’s message into the project terminal.
      Where: Project lead, Agent thread, Terminal. When: after sprint 10, with terminals.
- [ ] Change terminal text size, copy output, or clear scrollback.
      Where: Terminal. When: after sprint 10, with terminals.
- [ ] Watch a read-only session, take it over, or reconnect a lost session.
      Where: Terminal. When: after sprint 10, with terminals.

## Messages and posts

- [ ] Browse lead and team channels and direct messages, including unread, private, and waiting indicators.
      Where: Status → Message board → Channels. When: after sprint 10, with teams.
- [ ] See posts across channels that are still waiting for a reply.
      Where: Message board → Waiting for a reply. When: after sprint 10, with teams.
- [ ] See channel membership and open a member’s conversation.
      Where: Message board → Channel. When: after sprint 10, with teams.
- [ ] Search posts and filter by kind or author.
      Where: Message board. When: after sprint 10, with teams.
- [ ] Read a post’s recipients, status, and linked task or request, or open its author’s conversation.
      Where: Message board → Post. When: after sprint 10, with teams.
- [ ] Write a post, select a kind and recipients, and say whether it needs a reply.
      Where: Message board → Post. When: after sprint 10, with teams.
- [ ] Open a post’s thread, write a reply, and follow direct links to posts.
      Where: Message board → Thread. When: after sprint 10, with teams.
- [ ] Mark a post answered or reopen a manually closed post.
      Where: Message board → Post. When: after sprint 10, with teams.

## The architecture map

- [ ] See the current codebase diagram, build time, and any loading or failure state.
      Where: Project or repository → Design → Architecture. When: not assigned yet.
- [ ] Draw the initial diagram or update the existing one.
      Where: Design → Architecture. When: not assigned yet.
- [x] Focus on the diagram and leave focus mode with Escape.
      Where: Design → Architecture. When: works in the browser, needs no server.

## Memories

- [ ] See a sample memory-file edit in the lead’s tool output.
      Where: Project lead → Tool output. When: after sprint 10, with memories.

## Project and service settings

- [x] Change theme, match the operating system theme, or change density and see the preview.
      Where: Settings → Appearance. When: works in the browser, needs no server.
- [ ] Set the default workspace folder, permissions, and message visibility for new agents and teams.
      Where: Settings → Defaults. When: not assigned yet.
- [ ] Set the default policy for removing agent credits from merges.
      Where: Settings → Defaults. When: not assigned yet.
- [ ] Configure the web page, listening port, and connection check-in interval.
      Where: Settings → Remote access. When: not assigned yet.
- [ ] Set a public address behind a secure proxy or see port-forwarding instructions.
      Where: Settings → Remote access. When: not assigned yet.
- [ ] Set the project terminal preference for the service.
      Where: Settings → Remote access. When: after sprint 10, with terminals.
- [ ] Enable stall detection and set when assigned work, workers, or leads count as stalled.
      Where: Settings → Stall watch. When: not assigned yet.
- [ ] Set automatic nudges and the number of stalls before replacing a worker.
      Where: Settings → Stall watch. When: not assigned yet.
- [ ] Set capacity wait and retry limits, crash retries, and idle-team shutdown.
      Where: Settings → Stall watch. When: not assigned yet.
- [ ] Set whether standing permission rules can answer future approvals.
      Where: Settings → Stall watch. When: not assigned yet.
- [ ] Choose the workflow override folder and see repository override precedence.
      Where: Settings → Workflow. When: not assigned yet.
- [ ] Set default test-first work, agent review, and review rounds.
      Where: Settings → Workflow. When: not assigned yet.
- [ ] Connect or remove GitHub accounts and see their access and repositories.
      Where: Settings → Connections, Add a repository. When: after sprint 10, with following a repository by its remote.
- [ ] Choose whose identity appears on pushes and pull requests, with a project override.
      Where: Settings → Connections, Project Settings → General. When: not assigned yet.
- [ ] Choose the model for Git messages and see examples and title constraints.
      Where: Settings → Git messages. When: not assigned yet.
- [ ] See service version, status, data folder, number of machines, and licence.
      Where: Settings → About. When: not assigned yet.
- [ ] Check for an update or request a service restart.
      Where: Settings → About. When: not assigned yet.
- [ ] Rename a project or change its goal.
      Where: Project Settings → General → Project. When: not assigned yet.
- [ ] See the project branch and folder, then choose its merge target or machine for new runs.
      Where: Project Settings → General → Code. When: sprint 10.
- [ ] Set the lead’s subscription, model, and effort.
      Where: Project Settings → Agents. When: sprint 9.
- [ ] Change or approve the proposed harness, model, and effort for each kind of work.
      Where: Project Settings → Agents → Models. When: not assigned yet.
- [ ] Set the project’s agent permission mode.
      Where: Project Settings → Agents → Permissions. When: sprint 7.
- [ ] Set when the lead may plan or start work, and see or withdraw an existing grant.
      Where: Project Settings → Agents → Permissions. When: after sprint 10, with tasks and the build loop.
- [ ] Delete a team after its agents stop and its tasks move away.
      Where: Project Settings → Agents → Teams. When: after sprint 10, with teams.
- [ ] Set team-specific need-to-know message visibility.
      Where: Project Settings → Agents → Teams. When: after sprint 10, with teams.
- [ ] Archive a project or restore it to the active list.
      Where: Project Settings → General → Archive and delete, All projects. When: not assigned yet.
- [ ] Delete an archived project and its associated work from the visible lists.
      Where: Project Settings → General → Archive and delete. When: not assigned yet.
- [ ] Configure project tests and the commands used for a check and a full run.
      Where: Project Settings → Workflow → Tests. When: not assigned yet.
- [ ] Configure project agent review and its round and point limits.
      Where: Project Settings → Workflow → Review. When: not assigned yet.
- [ ] Set whether eligible fix sprints merge automatically and whether merges remove agent credits.
      Where: Project Settings → Workflow → Merging. When: not assigned yet.
- [ ] See configured previews and open one in Browser.
      Where: Project Settings → Previews. When: not assigned yet.
- [ ] Add or edit a preview’s name, command, port, and machine.
      Where: Project Settings → Previews. When: not assigned yet.
- [ ] Remove a preview definition.
      Where: Project Settings → Previews. When: not assigned yet.

## Notifications

- [ ] Set up or enable phone notifications through ntfy, manage its token, and send a test.
      Where: Settings → Notifications → ntfy. When: not assigned yet.
- [ ] Set up or enable email notifications, manage mail credentials, and send a test.
      Where: Settings → Notifications → Email. When: not assigned yet.
- [ ] Choose the events that trigger notifications.
      Where: Settings → Notifications → Events. When: not assigned yet.
- [ ] Set quiet hours and choose whether urgent alerts bypass them.
      Where: Settings → Notifications → Quiet hours. When: not assigned yet.
- [ ] Retry delivery of a failed notification.
      Where: Settings → Notifications → Recent notifications. When: not assigned yet.
- [ ] See notification delivery history and open related work or mark notifications read.
      Where: Settings → Notifications → Recent notifications. When: not assigned yet.
- [ ] Accept the invitation to set up phone alerts or dismiss it.
      Where: Home, Settings → Notifications. When: not assigned yet.

## First-run setup and sample controls

- [ ] See detected harnesses and sign in to an account or skip this setup step.
      Where: First run → Harnesses. When: sprint 9.
- [ ] Choose or enter the first repository folder and see its branch and remote.
      Where: First run → Repository. When: not assigned yet.
- [ ] Enter the first project goal and choose a model for Git messages.
      Where: First run → First project. When: not assigned yet.
- [x] Skip setup and continue to Home.
      Where: First run → Skip setup. When: works in the browser, needs no server.
- [x] Switch between sample connection and login states and open the first-run wizard.
      Where: Mock controls. When: sample mode only, by design.
- [x] Toggle whether the folded sidebar peeks on hover.
      Where: Mock controls. When: works in the browser, needs no server.
- [x] Reset the sample and reload the website.
      Where: Mock controls → Reset the sample. When: sample mode only, by design.

## Controls that are real today and promise more than they do

These were found while the list was made. They are recorded as issue 1675 in [issues.md](issues.md).

- "Reconnect now" in the connection menu changes the badge in the browser. The watch of the machines retries by itself.
- "Try to reconnect" is offered for an offline machine. The server refuses a restart of a machine that is not connected.
- "Scan harnesses again" reports that nothing changed. It does not compare with the scan that follows.
- The sign-in screen says that the owner returns to where they were after an expired session. The owner lands on Home.
- Logging out clears the browser's view also when the server did not confirm the logout.
