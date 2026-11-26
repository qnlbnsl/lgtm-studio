# LGTM Studio: the feature list

Status: **draft, not approved.** The foundation is merged into `master`, so
clients reach the built features through the web page, the command-line tool
and the client API. The owner lifted the development freeze on 2026-11-08
for the website build.

Each feature has a short name, one paragraph saying what it does, and a state:
**idea** (written down, not agreed), **agreed** (the owner wants it), or
**built** (merged and working).

The state **built in the engine** dates from when the engine had its own
branch, and that code is now on `master`. A feature is **partly built** when
only some of what its paragraph describes is merged; the note after it says
which part. The states were last compared with the code on 2026-11-07 by
searching for the matching source, not by running each feature.

An earlier, much longer list of message-board features was written for the
Synapse fork. It is at `docs/message-board/features.md` in the
`herdr-synapse` checkout, and it is the main input for section 2 below.

## The next piece: installation and configuration

- **Standalone installer and startup at login** (agreed, 2026-11-05).
  Install the `lgtm` command and let the owner start the service at login.
  The engine, board, and website run without Herdr.
  The optional plugin lives in `plugins/herdr` and calls the installed command.
  The installer and login integration are not built by the rename and split.
- **Project configuration in `.lgtm`** (agreed, 2026-11-05).
  Define the folder's contents and loading rules in the next piece.
  The owner decided on 2026-11-11 that `lgtm init`, run in a repository,
  creates the folder. It writes the template files, puts the project in setup
  mode, copies the global settings from `~/.lgtm` when LGTM Studio was set up
  globally before, and starts the setup conversation with the owner's
  preferred harness.
  Agents can read the folder, and changes require the owner through the
  existing settings protection.
- **An overridable default workflow** (agreed, 2026-11-05).
  LGTM Studio ships a default workflow.
  People can override its team definitions and instructions.

## 1. The foundry: roles and their lifecycle

- **Default role instructions** (built in s22). The engine ships twelve role files and three bases.
  It composes them for the first turn of a fresh session and for `roles.orient`.
  The owner can replace individual files under `~/.lgtm/roles`.
  See [the role instructions document](roles.md).
- **Repository role overrides** (idea). Decide whether a repository may supply agent instructions before adding repository override loading.
- **Role instruction editor and file commands** (idea). Add authenticated API actions to list, read and save role instruction files, and an editor that uses them.
- **Evidence before done** (idea). Check that a reported completion names an existing branch head and that required checks have finished.
- **No run notes in commits** (idea). Reject a team merge that adds run notes, reports or scratch files according to an owner-approved rule.

- **Repeat the roles stall check without the cache wait** (agreed,
  2026-11-05). The owner can run `pnpm live-check:roles --skip-timed-flush`
  to repeat the team collection and first stall without waiting for the
  cache timer. The report marks the timed flush as skipped. The run remains
  incomplete and exits with code 1, even when every remaining step passes.
- **LGTM Studio tools need no harness approval** (agreed, 2026-11-05). An agent
  can call LGTM Studio's own Model Context Protocol server in every permission
  mode on Claude, Codex, Gemini, and Cursor. The engine authenticates every
  call and checks the caller's existing rights. Other servers and native
  file and command tools continue following the selected mode. The settings
  belong to the managed session or process, not the owner's harness settings.
- **Orchestrator** (built). The top-level agent. It receives the owner's goal,
  decides which teams to staff, and talks to team leads, not to workers.
- **Team leads** (built). One per team. A lead plans its team's work, hands it
  to workers, and reports to the orchestrator.
- **Worker kinds** (idea). Workers have a kind, such as implementer,
  reviewer, or researcher, and a harness, such as Claude Code or Codex. The
  kind decides what the worker is told; the harness decides how it is run.
- **Start, stop, and reuse** (idea). The foundry starts agents, retires them,
  and reuses an idle worker instead of starting a new one when it can.
- **Two run modes for every harness** (agreed). Each harness has a run-mode
  setting, which the client API exposes, so it can be changed from a phone.
  - **ACP mode** (built first; partly built in the engine: unit 6a starts
    the agent, opens its session, sends prompts and turns its updates and
    permission requests into events; unit 6b pauses and returns agents for
    their accounts; unit 6c records crashes, sweeps leftover process groups
    and tells a signed-out account from one out of usage; unit 6d recovers
    agents after an engine restart: it reloads each one with a new token,
    scans the replayed history for delivery ids, then sends a carry-on or
    re-sends, returns paused agents, and supports `resume --fresh`; unit
    7a serves the website and password login during recovery, while authenticated data and socket requests report typed startup progress until recovery ends). LGTM Studio starts the agent as a child process
    and drives it through the Agent Client Protocol (ACP): it opens a
    session, sends prompts, and receives structured events (text, plan, tool
    calls, permission requests). Adding a harness means adding its ACP
    command, so Claude Code, Codex, Gemini CLI, OpenCode and the rest come
    almost for free. Claude Code and Codex need their adapters,
    `claude-agent-acp` and `codex-acp`.
  - **Interactive mode** (built second). LGTM Studio opens the harness's normal
    terminal program in a Herdr pane through the optional plugin, and the owner
    can watch it and type into it. Use this for Claude when the 5-hour limit
    matters: Claude in ACP mode runs through the Agent SDK, which T3 Code
    reports drains the 5-hour limit faster than interactive Claude Code.
  - Both kinds of agent share one board, roster and thread view. Only the way
    a post is delivered differs: an ACP agent receives it as a prompt, and an
    interactive agent receives it through its hooks or a typed nudge when it
    is idle. See [research/acp.md](research/acp.md).
- **Several accounts for every harness** (agreed, 2026-11-01; partly built
  in the engine: unit 6b picks an account, reads its secrets, gives a second
  Codex account its own shadow home, pauses agents whose account signs out or
  runs out of usage, and returns them when the account is available again;
  unit 7a carries the accounts command over the API). The owner can
  set up more than one account for a harness, such as two ChatGPT accounts
  for Codex or three Cursor accounts, and LGTM Studio spreads agents across
  them. Each account is its own sign-in, kept apart by its own home
  directory (`CODEX_HOME` for Codex, `CLAUDE_CONFIG_DIR` for Claude Code) or
  by its own API key, plus any extra environment variables. Starting an agent
  names an account, or asks for any account of that harness. Secrets are
  stored as references and never written to the event log. The design is T3
  Code's "provider instances", including Codex's "shadow home", which lets
  two Codex accounts continue the same threads. See T3 Code's
  `docs/user/providers-codex.md`, `providers-claude.md` and `cursor.md`, and
  `packages/contracts/src/providerInstance.ts`, at commit `6fc9144`.
- **A token for every agent** (built in the engine: units 5a and 5b). When LGTM Studio starts an agent, it mints a
  private token for it and passes it in an environment variable. The agent
  proves who it is with that token, not with the pane's environment or the
  process tree, so Codex's sandbox and a backgrounded Claude stop breaking
  identity. Borrowed from T3 Code; see
  [research/t3-code.md](research/t3-code.md), item 1.
- **LGTM Studio's tools over MCP** (built). The board's verbs (post, read, finish
  work, ask the owner) are offered to agents through a Model Context Protocol
  (MCP) server on a loopback address, as well as through the command line.
  Every tool declares which roles may call it. See research item 1.
- **A permission ceiling** (built in the engine: units 5a, 5b and 6a; unit
  6a sets back a mode change made inside the agent). An agent that a lead starts, swaps or
  messages never runs with a broader permission mode than the lead. See
  research item 6.
- **Hand-offs built from records** (built). A replacement worker starts from a
  hand-off that LGTM Studio builds from the work brief, the worker's posts, the
  files it touched and the commands it ran, with no model turn spent. See
  research item 5.
- **Role rules enforced** (idea). The command handler refuses an action that a role's rules forbid, such as a worker starting an agent or asking the owner. It reads a role table that the team's definition supplies. Version 1 only advises and reports (roles decision row 10).
- **Pin Claude Code's shell folder at launch** (idea). A launch setting, such as `CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR`, would keep Claude Code's shell in the session folder. Standing rules could then match Claude agents, whose shell input names no folder (roles round 2, N5).

## 2. The message board

- **Channels and direct messages** (built). Each project starts with General and
  Leads channels for the owner and the project lead.
  General admits each active agent in the project once.
  Leads admits active agents with a lead role and a team group from that project.
  The engine preserves the owner's member removals.
  Each channel holds at most 64 members.
  A service post records each admission skipped because the channel is full.
  The owner can add a skipped agent manually after making room.
  The engine appends missing defaults for active projects on startup.
  It skips archived and archiving projects.
  Project creation requires the board to be enabled.
  Each team also has a channel.
  Leads can message the orchestrator directly.
  Workers can message each other.
- **Wake rules** (built). Each post says whether it needs an answer. Only a
  post that needs an answer wakes its recipient, because every wake costs a
  full agent turn.
- **System events in a log** (idea). Joins, exits, and status changes go to
  an event log, not onto the boards that agents read.
- **A mailbox per agent, with receipts** (partly built in the engine:
  units 4a and 5d deliver to ACP agents, with queued, sent and accepted
  steps and request receipts; the hook and typed-nudge paths for
  interactive agents are not built). Every post is stored before
  it is delivered, and something is always responsible for delivering it. A
  post to an idle agent wakes it, a post to a busy Claude is steered into its
  running turn through a hook, and anything else waits for the turn to end.
  Each delivery is recorded with when and how it landed, so a wake can never
  be silently lost. See research item 3.
- **Wake Claude without typing** (idea). Each Claude agent keeps one
  long-lived `Monitor` running an LGTM Studio command that prints a line only when
  a post needs it. Claude Code starts the turn by itself, and nothing is typed
  into the pane. Other agents keep a typed nudge that only fires when the
  agent is idle. See research item 2.
- **Wait results that say why** (built in the engine: unit 5d's
  waits end with a reason per member, and unit 7b's `lgtm wait` exits
  with the codes below). A wait or a collection ends early,
  with a reason, when the agent it waits on is stuck on a question or an
  approval. The command line exits with 0 for done, 1 for failed, and 2 for
  waiting on an answer. See research item 7.
- **Post kinds** (idea). A label on each post, such as question, blocked, hand-off, done or decision, for filtering and for the dashboard. It was left out of the board's first plan, because the wake rule needs only "needs a reply" (board decisions row 14).
- **No acknowledgement-only posts** (idea). Refuse a reply that needs no reply to a post that needed none, to stop "thanks" posts. This is item 10 of the Synapse list, and it is not in the board's first plan.

## 3. Threads

- **One thread per agent conversation** (partly built in the engine: unit 6a
  writes each delivery's text, plan, tool calls and file changes as thread
  items; no client shows them yet). Every agent's conversation is
  visible as a thread, with its messages, approvals, and changed files, in the
  spirit of T3 Code.
- **Threads where the owner already works** (idea, 2026-11-01). The owner
  works in the terminal, with Herdr open inside Cursor and everything else
  hidden, and found T3 Code's separate chat window awkward to work beside.
  So threads should also be readable and answerable in two places besides
  the web page: a thread viewer that runs in a Herdr pane, and a Visual
  Studio Code extension, which Cursor can also run. Forking Visual Studio
  Code itself was considered and set aside, because keeping a fork of an
  editor current is a large, permanent job. Both views are clients of the
  same client API.
- **Steer a thread from any client** (idea). The owner can read, answer, and
  interrupt a thread from the web page, a desktop app, or a phone.

## 4. The client API

- **One authenticated API for every client** (idea; partly built in the
  engine: unit 7a serves an authenticated WebSocket API on a Unix socket in
  the owner-only state folder, for the owner, paired devices and agents; unit
  7b adds the client library). The web page, a future
  desktop app, and a future mobile app all use the same API.
- **Device pairing** (dropped for phones and browsers by the owner on
  2026-11-03, in favour of one password; a pairing code remains allowed for
  worker machines). A new device is added with a one-time pairing
  code, as described in [SECURITY.md](../SECURITY.md).
- **Live updates** (idea; partly built in the engine: unit 7a's `subscribe`
  streams events, and an agent sees only its own and its children's). Clients receive board posts, thread events, and
  agent states as a stream, not by polling.
- **Updates that survive a sleeping phone** (partly built in the engine:
  units 4a to 4c give request receipts, gapless subscriptions and
  snapshots, unit 7a serves them over the socket, and unit 7b's client library
  reconnects and resumes from its cursor). A client gets a snapshot
  and then only newer events, picks up where it left off after a reconnect,
  and every action carries a request id so a double tap acts once. See
  research item 9.
- **A browser terminal that cannot type into the agent** (idea). The page
  opens an agent's terminal read-only by default, and strips old terminal
  queries before replaying output, so the browser never answers them into the
  agent's input box. See research item 10.
- **Four phone alerts** (idea). The phone is alerted only for an owner
  question, an agent stuck on an approval, a stall that reached the owner, and
  a failed run. See research item 11.
- **Agents run in a sandbox** (agreed, 2026-11-05). Each agent runs inside
  bubblewrap, wrapped around the whole harness. Only its worktree is writable
  and `.lgtm` is mounted read-only. The owner's home folder, LGTM Studio's state
  folder and other processes are not visible. This replaces the earlier plan
  for LGTM Studio to run as its own Unix user, because bubblewrap needs no root
  access at install and leaves worktree files owned by the owner. Real Codex
  and Claude command-line runs worked this way on 2026-11-05, in a manual test.
  It is not built.
  A harness's login stays visible inside its sandbox, and how to hide it from
  the commands an agent runs is still open.
- **Agents use API keys without holding them** (idea, 2026-11-05). The
  engine runs a tool server such as Firecrawl's outside the sandbox with the
  key, and agents call the tool. A proxy that swaps a placeholder for the real
  key may follow for scripts. Trusted steps such as test commands run outside
  the agent's sandbox. It is not built.
- **macOS support for the ancestry check, engine and client** (agreed). Version
  1 runs on Linux only, so the client refuses to send the owner credential on
  any other system, and the engine refuses it too. The owner agreed on
  2026-11-01 ("yeah linux only for now. shoud be easy enought to port to
  macos") that macOS comes later, as a port after version 1. The port adds
  the `ps -o ppid=,lstart=` ancestry reader to the client and the engine, and
  a macOS reader for the leftover-process sweep. See issue 183 and issue 120
  in [issues.md](issues.md).
- **Filtered live updates** (idea). A subscription for phones that is filtered by agent and by event kind, with checkpoints that keep the cursor moving.
- **One password login** (built). The service reads the password from the environment variable `LGTM_WEB_PASSWORD`, and the dashboard asks for it. One person uses LGTM Studio, and a logged-in session is the owner. This replaces the device pairing entry above (client API rows 184 to 188).
- **Owner phone notification actions** (built, 2026-11-08). The owner can
  retry a failed notification through the configured service and send a real test
  notification. Retry validates the displayed generation and attempt, and records
  the next result and its time. A test creates no alert and stores no carrier
  response. Declining the first waiting request's phone notification offer is
  remembered across service restarts.
- **Phone alerts through one sender** (built). Phone alerts go through one sender interface, `AlertSender`, with ntfy as the first carrier. A later carrier, such as Expo's push service for the phone app, slots in behind it (client API row 83).
- **Mobile login** (later). Starts by looking at what Home Assistant does.
- **VS Code extension** (later). One of the last things, along with the mobile app, by the owner's answer of 2026-11-03.

## 5. The stall watch

The orchestrator and the owner lead the work. The stall watch is the part of
LGTM Studio's background service that keeps every agent moving without
spending their turns. It never decides what the work is; it only notices when
an agent has stopped and nudges it back, or tells its lead.

- **Stall detection** (built). The stall watch knows each agent's work state (off,
  assigned, working, waiting, done) and notices an agent that has stopped
  without finishing. It wakes the agent once, then tells its lead. A worker
  that stalls three times in a row on the same work is replaced. The rules
  come from Synapse-list items 23 and 24 (see
  [research/herdr-and-synapse.md](research/herdr-and-synapse.md)).
- **Usage-limit pauses are not stalls** (built). An agent stopped by a usage
  limit is resumed when the limit resets, and the pause never counts toward
  replacing it. See research item 8.
- **Stuck input boxes and approval dialogs** (idea). An agent whose input box
  has held the same text for a long time, or that is waiting on an approval
  dialog, is reported to its lead.
- **Compaction timed to the prompt cache** (built). An idle Claude agent with a
  large context is compacted two minutes before its prompt cache expires, not
  after, because a wake after the cache expires costs about 20 times more for
  the cached part. Posts that need an answer and can wait are delivered
  before the cache expires. See research item 4.
- **Replacement waits for the lead's team** (agreed, 2026-11-07). When a
  lead has not answered three nudges, the stall watch marks the lead as advised
  for replacement and tells the orchestrator. When the lead's workers have all
  finished, the stall watch tells the orchestrator that the lead is ready to
  replace. The new lead starts from the original order and the old lead's
  transcript, and it reaches the team's workers directly. Issue 520 holds the
  details.
- **Runaway work** (built). Work that runs far past its expected length is
  reported to the lead, and can continue in a fresh agent with a built
  hand-off.
- **Cache lifetime setting** (built). A per-harness `cacheLifetime` setting, five minutes or one hour, for timing compaction when the adapter does not report which cache lifetime it wrote (roles round 1).
- **"Compact me when I am next idle"** (idea). An agent asks the stall watch to compact it after its current turn. This replaces the Foundry's `want-<name>` file for the context-keeper.

## 6. Source control

- **Remove agent credits when merging** (agreed). A setting, off by default,
  that each project can override. When LGTM Studio merges a worker's or a team's
  work with a merge commit or a squash commit, it removes the lines that
  credit a coding agent from that commit's message: `Co-authored-by:`
  trailers for known agents, and footers such as "🤖 Generated with Claude
  Code". Human co-authors stay credited. The original commits are not
  rewritten. The client API exposes the setting, so it can be changed from a
  phone. The code is T3 Code's own, copied with its MIT licence into
  [third_party/t3code/](../third_party/t3code/README.md).

## 7. Repository, project, and team navigation

- **A workspace folder chosen at installation** (agreed, 2026-11-05).
  Installation asks which folder contains the owner's repositories.
  Repository setup stays on the host. The mock uses known sample paths.
- **Repositories contain projects** (agreed, 2026-11-05; mocked in
  `design/mock/`). Repositories are the top level of navigation.
  Each repository has a page listing its projects and an Add Project
  action. Multiple projects can work within the same repository.
- **Exactly one lead per project** (agreed, 2026-11-05; mocked). Selecting
  a project opens its sprint board, updated on 2026-11-06. A right chat
  panel opens the project lead alongside that board. Each project has its own
  goal, sprint board, teams, and messages. The owner's clarification
  replaces the earlier ambiguous request for multiple leads within a
  project. Multiple projects have separate leads.
- **Start a project through its lead** (agreed, 2026-11-05; mocked).
  Add Project creates a named project under a known repository and opens
  its lead conversation. A new project has no sprints or teams until its
  plan is shaped. Existing projects never reopen repository setup.
- **Dedicated project worktrees** (updated by the owner, 2026-11-09).
  Every project has its own worktree. Add Project has no worktree checkbox.
  A safe version of the name determines its branch and worktree folder.
  Numeric suffixes resolve existing names. The dialog always previews Branch
  and Folder before submission. The repository main checkout remains recorded
  for the owner’s review and approval.
- **Workspace settings** (agreed, 2026-11-06; mocked). A Settings tab beside
  Board, Chat, Terminal, and Git shows the assigned repository, branch,
  checkout type, folder, and machine. It stores a project-specific terminal
  shell preference in the preview. Service settings remain a global page.
  On 2026-11-14 the real page gained the lead's model and effort, a Teams
  table, and its own "Archive or restore" section. The mock's base branch,
  permission, three switches, assignment table, need-to-know, delete and task
  prefix still need engine support.
- **A project board of sprints** (agreed, 2026-11-05; mocked). The Kanban
  board shows Planned, In progress, Review, and Done. A sprint contains
  the plan's tasks and shows ownership, current phase, dependencies,
  blockers, and completion evidence. Team task lists and sprint details
  read the same assignments. The owner referenced the build-order roadmap of
  a private repository. No implementation code was copied.
- **Talk to the lead beside the board** (agreed, 2026-11-06; mocked).
  The navigation tree has no separate Board row. The board has a right
  chat panel with its single lead and a working sample composer. It
  shares messages, drafts, and reading position with the full conversation.
  The panel can be collapsed. On smaller screens it opens as a drawer
  with contained keyboard focus and Escape to close.
- **The project states its assigned branch** (agreed, 2026-11-06; mocked).
  The sidebar shows it under the project name beside the lead. The board
  repeats it beside the title. The top-right picker says Viewing and only
  changes inspected code context. It leaves the project assignment intact.
- **Completion reflects evidence** (mock behavior, 2026-11-05). A completed
  sprint records verification and merge evidence. Release status is
  separate. Completed sprints do not accept new tasks. Moving a card is
  not offered as a substitute for verification or merging.
- **Teams and individual conversations** (agreed, 2026-11-05; mocked).
  Teams appear under their project. Each team has Chat, Tasks, and Changes.
  Each agent also has a direct conversation. Disclosure controls expand
  the tree independently of navigation. Senders identify their coding tool.
- **Conversation continuity** (agreed, 2026-11-05; mocked). Each conversation
  keeps its draft and reading position while navigating. A team remembers
  its last view. Messages and these preferences remain in memory until
  refresh. Sample replies can be stopped and are explicitly labelled.
- **Branches and workspaces in the context menu** (agreed, 2026-11-05;
  mocked). The menu lists the selected repository's branches, workspace
  paths, ownership, and activity. The mock treats selection as inspection:
  it changes the viewed code context and Changes view. It never changes
  the repository or redirects an agent's assigned workspace.
- **Project leads surface team requests** (agreed, 2026-11-05; mocked).
  Sage can point to Rowan's design question. The project and requesting
  agent are highlighted, and the affected sprint shows its dependency.
  Selecting the request opens Rowan's conversation at the exact question.
  Answering clears every indicator. Viewing alone does not resolve it.
- **Compact chat composition** (agreed, 2026-11-05; mocked). The header
  combines identity and navigation. User messages have a restrained
  treatment. Agent replies use readable text with grouped tool details.
  The composer grows when needed and retains its recipient label.

### Existing page capabilities the redesign must retain

Agreed on 2026-11-06 and represented in the browser mock. These are existing
product capabilities, not permission to remove them during implementation.
The source-to-mock mapping was `design/FEATURE-PARITY.md`. It was removed with the older
workbench on 2026-11-12, and it remains in the git history.

- Login, logout, and session expiry stay available. Rejected login and an
  unavailable session have explicit recovery states. Entered secrets clear
  when the session ends.
- Connection status stays visible. Reconnection retains the last received
  state and drafts. Stale views do not offer functioning mutation controls.
- Permission requests show their identity and action input before approval.
  Hidden inputs, held requests, denial, cancellation, and unconfirmed
  answers remain distinct from an agent's ordinary text.
- Agent stop and resume stay available. Stopping cancels permissions but
  preserves questions. A fresh resume explains its loss of prior context.
- Failed starts keep the current safe reason categories and prevent sends.
- Large thread items keep an explicit full loader with failure and retry.
- Tool calls keep identity, status, input, and output. Deliveries retain
  their sender, source, identity, and history.
- Alerts and Settings remain full pages. Alerts retain pagination, subject
  links, and notification delivery failures. Settings retain all current
  notification and web-server controls.

### Workspace scope and navigation refinement

Approved on 2026-11-06 after comparison with T3 Code and Visual Studio Code.
These interactions are represented in the browser mock.

- One workspace tab row follows the selected project, team, or agent.
  Git in team scope selects that team's worktree. The header names the
  worktree's branch, team or project, and machine.
- Team worktrees integrate into their project worktree. Project branches
  reach main through explicit owner review. Dirty source or destination
  worktrees and unresolved conflicts prevent integration.
- Each worktree has its own staging selection, commit message, history,
  incoming/outgoing state, and file diff. Git includes Commit, Fetch, Pull,
  Push, and conflict resolution controls.
- The navigation and project lead panel resize with pointer or keyboard.
  Focused chat stays centered and readable on wide monitors. The board fits
  four columns beside its lead at desktop width. Phone users have a status
  strip for jumping between columns.
- Composer controls show current agent progress, stop/resume, subscription
  choice, and attached file/task context. Activity details link to Git.
- An attention summary distinguishes decisions, permissions, and failed
  starts. A navigation filter hides projects with no unresolved attention.
  A keyboard quick switcher searches projects, teams, and agents.
- Named terminal sessions preserve independent input and command history
  within each machine and worktree. Context changes save the old session
  before selecting the destination.

### Terminal, Git, machines, and subscriptions

Requested on 2026-11-06. The project Terminal tab is connected as of 2026-11-14.
Machine coordination remains planned.

- The main conversation can switch to a terminal. The intended product
  provides actual direct commands on the selected machine and workspace.
  The connected website provides owner-only shells in registered local worktrees.
  Service settings has a stored switch that starts off.
  The renderer supports colours, cursor movement, full-screen programs, and resizing.
  Live output reaches the attached owner connection unchanged.
  On 2026-11-17, the owner approved reattachment after a reload.
  Sharing shells across the owner's signed-in sessions is this unit's design choice for a single-owner product.
  Up to eight independent web sessions survive service restarts for 30 days each.
  Signing out ends only that browser's session.
  The last output reader wins, while other signed-in tabs can still type.
  The displaced tab explains how to take output back and learns when the shell ends.
  Web shells survive while any signed-in owner session remains, subject to the detached grace limit.
  A reload restores the shells and at most 64 kibibytes of recent output from memory.
  The service never writes terminal input or output to disk or logs.
  At most eight sessions may be open, including detached shells.
  A detached web shell stays available for five minutes.
  Taking over an agent terminal remains planned.
  The standalone mock still runs only its pretend shell. The owner requested normal product copy without repeated mock
  disclaimers on 2026-11-06. Prototype boundaries remain in design documentation.
- Git combines a changed-file tree with unified and split diffs. Clicking a
  file directly opens its diff. Branch-aware commit history uses the same
  pane. There is no separate Diff tab or intermediate file preview.
  Live repository reads, Git mutations, and merge validation remain integration work.
- One computer runs the master process. Other computers pair through a
  connect mode and act as workers. The master assigns projects across
  machines. Pairing, availability, host identity, and project placement
  need authenticated coordination and recovery without duplicate work.
  The machine page is a preview and does not move running work.
- A Subscriptions page supports several named accounts for the same
  harness. Each entry has its own machine, enabled state, and sign-in.
  Additional subscriptions do not reuse the default Codex authentication.
  The engine's existing account/profile separation must be preserved.

### Global Home, machine choice, and screen density

Requested on 2026-11-06 and represented in the browser mock.

- Home is the initial global page. Needs attention combines unresolved
  questions, permissions, failed starts, and project merge reviews across
  all projects. All projects opens each project's board.
- Questions and Approvals have their own global lists with answered history.
  Search combines with repository, project, agent, machine, and status filters.
  Requests are single-line disclosures, collapsed initially. Their details
  expand directly underneath rather than in a split pane. The same toggle
  closes them and retains keyboard focus.
  Questions and permissions can be answered without leaving Home. Merge
  requests retain the explicit review step. Links open the source conversation
  or board, preserving drafts and the requesting team's branch context.
- Add Project chooses a machine through Run on before the project name.
  The preview shows that host's branch and folder. Offline hosts and hosts
  without the repository cannot start work. The owner approved a dedicated
  worktree for every project. New project has no worktree checkbox and always
  previews Branch and Folder. No host is silently substituted.
- Machines uses compact tables for machine status, role, agent slots,
  project counts, start actions, and future project placement. Future
  placement does not relabel running work or imply that a project moved.
- Cursor is the primary design reference. T3 Code remains secondary, with
  explicit discoverability and space usage requirements from the owner.
  These flows remain an isolated prototype; durable queues and machine
  execution require service integration.
- Sidebar boundaries separate Home/search, Projects, and service pages.
  New project belongs to Projects. Only projects needing you names the tree
  filter's effect. Repository names open their project lists; a second
  Repositories destination is unnecessary and has been removed.

## 8. The build loop

The full design is in
[specs/2026-11-05-build-loop-design.md](specs/2026-11-05-build-loop-design.md).

- **A default workflow as a parsable list** (agreed, 2026-11-05). The eight
  phases become a list of steps that LGTM Studio reads. LGTM Studio looks in the
  repository's `.lgtm` first, then `~/.lgtm`, then its built-in copy. The
  default install places a copy in `~/.lgtm`.
- **`.lgtm` is read-only to agents** (agreed, 2026-11-05). Agents can read
  it and cannot edit it.
- **Skill files for any agent** (agreed, 2026-11-05). LGTM Studio ships skill files
  that let any agent work with it.
- **A project setup agent** (agreed, 2026-11-05). It tells the owner what the
  project is made of, asks for the general direction, talks through testing,
  and produces a local `.lgtm` folder with a config file. How that reaches a
  folder agents cannot edit is open.
- **Optional testing** (agreed, 2026-11-05). The owner can turn tests off, and
  can also turn agent review off. They can turn tests on later through their
  main agent.
- **Scored stops on stuck statements** (agreed, 2026-11-05). A script scores
  review findings by the statement they come from. A stuck statement pings the
  owner, who keeps, changes or drops it.
- **A limit on review rounds** (agreed, 2026-11-05). Past round 3, another
  review runs only for blockers and majors. Rounds 5 and 6 need the owner's
  approval, and 6 is the limit.
- **Autonomous fix sprints** (agreed, 2026-11-05). A fix that users would not
  notice merges and moves on without the owner.
- **A hotfix lane** (requested by the owner, 2026-11-06; not designed). The
  owner asks the project lead for a hotfix, and it starts a very limited team
  focused on fixing and pushing.
- **A way to bisect** (requested by the owner, 2026-11-06; not designed). A
  command finds the commit that broke something by halving the range of commits
  at each test.
- **A way to freeze interfaces** (requested by the owner, 2026-11-06; not
  designed). A frozen interface cannot change while teams build against it.
- **The design file is the source of truth** (decided by the owner, 2026-11-07;
  not designed). A project's design lives in a Figma-format `.fig` file. LGTM Studio
  runs OpenPencil, the open-source editor from openpencil.dev, on the same
  computer and shows it inside its page. OpenPencil opens only while someone is
  designing, looking at components, or weighing an edit. Mocks that are only
  looked at are built from the project's real code, so what the owner sees is
  what users get, and any gap between the mock and the design gets fixed.
  Since 2026-11-08 the web mock shows OpenPencil's web editor in a frame, built
  from source in `design/openpencil/`. It opens a
  repository's design file. The live Mocks tab now accepts the editor's exported file
  through Save changes from editor. The owner selects the export, and the engine
  replaces only the registered design file. Exports are limited to 512 kibibytes.
  Saving records a new design revision and mock revision. It returns the project
  to Design and clears approval and seen marks for the new version.
- **Teams named for their stage** (requested by the owner, 2026-11-07; not
  designed). A project moves through a Design team, a Planning team, a Build
  team and a Review team, in place of hand-picked names. The project page shows
  which stage the project is in.
- **Mocks made by the Design team** (requested by the owner, 2026-11-07; not
  designed). A project with user interface work gets a Mocks tab. The mocks use
  the project's real components and realistic, invented data, as the mocks of
  the owner's private app did. While designing, the team records the fields the screens need and supplies
  reviewed comparisons with the real schema, as approved by the owner on 2026-11-09.
  The engine does not infer an application schema from mock data files.
  Fields without a supplied review remain unavailable. When the mocks are approved, new
  components move into the real component folder, and wiring them to real data
  becomes the Build team's tasks.
  The owner approved supported scenario routes on 2026-11-09, in item 2 of the
  website simplification shortlist. Each screen keeps its base route and only
  the concrete routes supplied for its supported declared scenario identities.
  The supported list may be empty. The engine does not invent missing routes
  or infer bindings from scenario names. Existing revisions remain unchanged.
- **A Reverse Fig skill** (requested by the owner, 2026-11-07; not designed).
  When someone connects a project that already has an interface, the skill
  turns its existing components and screens into a `.fig` file, so the design
  becomes the source of truth. Reverse Fig goes only from code to the design.
  The owner decided on 2026-11-08 that the conversion from the design file to
  code is a separate tool, built later as part of the Build team. A test
  on 2026-11-07 did this for three of the mock's components with OpenPencil's
  command-line tool. The result had real component sets, variants and color
  variables, and it exported back to React components. Some styling was lost
  on the way.
  A first version is in `skills/reverse-fig/` since 2026-11-08. It covers color tokens and
  components, not screens. Everything specific to a project lives in a profile
  file, so each project tunes its own profile.

## 9. Client mode

- **LGTM Studio in client mode** (requested by the owner, 2026-11-06; not designed).
  One main LGTM Studio holds the event log, the board and the website. Other machines
  run LGTM Studio in client mode and take their commands from the main one, so a
  project's load spreads across machines. How a client machine proves who it is
  needs the owner's word, because the login rule is one user, one password and no
  pairing.
- **The server does no machine work itself** (the owner's direction, 2026-11-14;
  not designed). The owner described the split in more detail.
  - The server holds the event log, the board and the website. It runs no agent
    and no command on a machine.
  - A client is the program that runs agents, shells and Git commands on one
    machine.
  - A client opens the connection to the server and takes its commands over it.
    The client knows the server's address. The server does not need to know
    where a client is, so it can run in the cloud.
  - On one machine, starting the server in client mode also starts a client, so
    a single-machine setup still needs one command.
  - More machines are added by starting more clients.
  - Still to decide: how a client proves who it is to the server, what a client
    may refuse, and what happens to its agents when the connection drops.
  - The owner sees this as a project to run inside LGTM Studio.

## 10. Design and mock modes

- **Design mode and mock mode** (requested by the owner, 2026-11-07; not
  designed). The website gains an integrated design mode and a mock mode, so
  the owner can use it to design and update the mocks of projects and apps.
  Each project lists all of its mocks. For example, the owner could open the
  website, go to the mocks of a private project, and see every one of them there. The owner is
  adding this to the website mock first.

- **Recorded mock previews and OpenPencil** (decided by the owner,
  2026-11-09; implementation pending). The engine renders a project's recorded
  mocks itself instead of starting a separate mock web server. The screen preview
  and OpenPencil stay in sandboxed frames. Both frames use one hostname other than
  LGTM Studio's, so the owner's cookie never reaches them. The page's content security
  policy permits frames from that hostname only. The engine starts and manages
  OpenPencil with access to only the project's design folder. No separate one-file
  capability is built. This decision does not add an editor save pipeline.
  On 2026-11-10, the owner approved `allow-same-origin` for the editor frame
  in addition to `allow-scripts`, so OpenPencil can use its display hostname
  storage. Recorded mocks retain an opaque origin. The editor receives no
  owner cookie or owner origin authority.

## Evaluation at project teardown

The owner raised these ideas on 2026-11-11. They are not decided. The decided
timing is a round after each phase step, in the build loop design's
"Evaluation rounds and the model ledger".

- When a project is torn down, LGTM Studio offers: "Would you like to evaluate
  how the models did from the session reports and help improve the model
  ledger?" The round runs only if the user says yes.
- The round reads the project's session reports and logs. The user reads the
  evaluation before anything else happens with it.
- The results are kept locally, in LGTM Studio's config files.
- LGTM Studio could send the results to a public endpoint, so that many users
  improve the shared model ledger. This sends data off the user's machine, so it
  needs the user's consent each time, and the payload must leave out code, file
  paths and anything else private. SECURITY.md applies to it.

## Open questions

- Should the name "LGTM Studio" be registered as a trademark, so that a changed version must use another name?
- Should outside contributors sign an agreement, so that the owner can offer other licence terms later?

## Decided

- **The licence is the GNU Affero General Public License, version 3 only**
  (2026-11-14). The owner wants every changed version to stay open, including
  one that is run as a network service, so that the project's history and
  credit stay visible.
- **The product is named LGTM Studio** (2026-11-11). The owner chose the name
  on 2026-11-11 and confirmed the spelling, two words with a space. On
  2026-11-12 the owner asked for the full rename. The command is `lgtm`, the
  packages are `@lgtm/...`, the environment variables start with `LGTM_`, and
  the state folder is `~/.local/state/lgtm`. The configuration folders are
  `.lgtm` and `~/.lgtm`. The GitHub repository is `qnlbnsl/lgtm-studio`. The
  owner chose to start fresh, so nothing reads earlier names, and every
  record in the repository uses the new name.
- **The public site is `lgtmstudio.dev`** (2026-11-11). It is the product's
  public website. It is not the address where the owner's own instance runs,
  which stays out of this repository.
- **LGTM Studio product website** (2026-11-11). The owner selected LGTM Studio
  as the new public name and requested a separate product website.
  On 2026-11-12 the owner moved the website out of this repository.
  It is no longer kept here.
  This change does not rename the engine packages, commands, or state folders.

- **Language: TypeScript** (2026-10-30). One language for the backend, the
  web page and the future apps, so the client API's types are written once.
  See [AGENTS.md](../AGENTS.md), "Language".
- **How agents are run: both ACP mode and interactive mode, ACP first**
  (2026-10-30). This answers the earlier question "panes, a structured
  protocol, or both?". ACP mode is built first, because it is the quickest
  start and gives every harness at once. Interactive mode follows, chosen per
  harness in its settings. See section 1, "Two run modes for every harness".
  The T3 Code research's rule that every agent stays a visible session now
  applies only to agents in interactive mode.
- **The engine is built first, and it is finished when the Foundry needs
  nothing else from it** (2026-10-31). The software-foundry skill's Steps 3
  to 5 and 8 give eleven capabilities, all offered through one API that
  agents and the owner's phone both use:
  1. Start an agent, with its harness, model, effort, permission mode,
     folder, name and role.
  2. Let agents start, prompt and stop their own agents, never with wider
     permissions than their own.
  3. Prompt an agent, with the sender recorded.
  4. Report each agent's state, and say when it changes.
  5. Wake a waiting agent once, when one agent or a whole set finishes,
     without polling.
  6. Stream any agent's thread live to any client.
  7. Route each permission request to the agent that started the asker,
     which answers it or passes it up. The owner can answer any request.
  8. Alert the owner's phone for a request passed up, or a question.
  9. Let the owner open and prompt any agent's thread remotely.
  10. Stop, resume and compact an agent.
  11. Group agents, as the Foundry's tabs do.
  12. Run each agent under one of several accounts for its harness, added by
      the owner on 2026-11-01.

  The board's channels and wake rules, the role rules, git worktrees, usage
  meters and interactive mode stay out of the engine.
- **The engine is one service with an event log** (2026-11-01). One
  background service owns every agent. Everything that happens is written to
  an append-only, numbered log in SQLite before anything acts on it, and
  threads, states, waits and the live stream are read from that log. T3 Code's
  server has the same shape, and since Effect was chosen (the next entry),
  LGTM Studio can copy T3's event store with credit. The rejected options were
  saved tables updated in place, and one supervisor process per agent
  sharing files.
- **LGTM Studio is written with the Effect library** (2026-11-01). Effect is a
  TypeScript library for whole programs: failures are part of each
  function's type, every running task has an owner that cleans it up, and
  one schema both checks data and defines its type. The reason for choosing
  it is reuse: T3 Code is written with Effect 4, so its ACP client
  (`packages/effect-acp`), event store, accounts and device pairing can be
  copied with credit instead of rebuilt. LGTM Studio pins the same Effect
  version as the T3 Code commit it copies from, so the copied code compiles
  unchanged.
- **LGTM Studio runs on Node.js 26** (2026-11-01). Node.js 26 is the current
  long-term support line, and the repository's `.nvmrc` says `26`. A single
  file compiled with Bun was considered and not chosen: T3 Code's SQLite
  code uses Node.js's own `node:sqlite`, and the Claude Code ACP adapter
  needs Node.js anyway.


## The real browser check

`pnpm live-check:web` exercises a private engine with a real Codex agent and
Chromium. It verifies streamed replies, prompts, command permission answers,
reload, stop, resume and safe startup failures. It also creates a real project
and worktree through first-run setup, has its actual lead create a linked
sprint, task and question with supported tools, answers from the board, and
checks the durable result and exact receipt again after service restart. It remains separate from
`pnpm check` because it consumes the signed-in account's allowance.

## Evaluation rounds that update the model ledger

The owner asked for this on 2026-11-10. It is not built. At the end of each key
task and at the end of a project, the build loop runs an evaluation round. The
round grades each task from its reviews and later corrections, and it writes the
grades to the model ledger. The ledger feeds the list that says which model to
try for each kind of task. The design is in "Evaluation rounds and the model
ledger" in
[docs/specs/2026-11-05-build-loop-design.md](specs/2026-11-05-build-loop-design.md).

The owner decided on 2026-11-11 that model evaluation leaves the owner's private toolkit and becomes
part of LGTM Studio, as research. LGTM Studio ships one default model selection ledger, and each
user can update it.

## Questions that carry their own brief

The owner asked for this on 2026-11-11, and it is agreed for the mock. It is not
built in the engine. When an agent asks the owner a question, it writes a short
brief of the question, one line of description, a short recommendation, what the
question is about, why it matters, why it recommends its answer, and the files it
cites. A thread shows the brief first, with Accept, Deny and Details, and the
full card opens from Details. The mock in `design/mock/` shows the layout. The
engine's question records and the agents' instructions would need these fields.

## A file explorer in the Git view

The owner asked for this on 2026-11-11. It is agreed. It was built on the branch
`area/files-settings` on 2026-11-14, for worktrees on this computer only. A
project has a "Files" tab, beside its Git tab, that works like the file explorer of an
editor such as Visual Studio Code. It shows a folder tree of the inspected worktree, and
the open file beside it. References in questions, approvals and plans link to a file and
open it here. The mock in `design/mock/` shows the tab at `/p/console/files`.

### Where the files come from

The files live on whichever machine holds the worktree, such as Studio or Buildbox. The
web page and the phone app never reach that machine themselves. They ask the LGTM Studio
service through the client API, and the service asks the machine. The rules are these:

- The service lists one folder at a time, so a large repository does not load at once.
  Each entry carries its name, its size, whether it is a folder, and its Git status.
- The service returns one file at a time, as text. A file over 1 megabyte, or a binary
  file, is not returned as text. The client offers it as a download, which the service
  streams from the machine.
  As built, the limit is 512 kilobytes, because a reply must fit in one WebSocket message
  of 1 mebibyte. Downloads are not built yet. Both are recorded in
  [docs/issues.md](issues.md).
- Images, such as SVG and PNG files, are returned as images and shown as pictures.
- Every path is checked on the machine against the worktree's folder, after symbolic
  links are resolved, so a request cannot read a file outside the worktree.
- The owner can edit a text file, save it, and commit it. "Simple editing in the Files
  tab" below describes how.
- When the machine is offline, the tab keeps its structure and says so. It shows the
  files again when the machine reconnects.
- The tab follows the "Inspecting" choice in the top bar. A branch with no worktree yet
  shows the base branch, with a note that says so.

### Which component shows the files

The owner asked on 2026-11-11 for the option that works best across platforms. The
choice is the stack that T3 Code uses, so that its code can be copied under the MIT
License:

- `@pierre/diffs` renders files and diffs. T3 Code uses it on the web and in its server,
  and its import style works in React Native as well.
- `@pierre/highlights` colours the code. T3 Code moved its web app from Shiki to it,
  because Shiki took most of the render time on large files and could freeze the window.
  T3 Code's phone app already uses the native form of `@pierre/highlights`.

Highlighting runs in each client, so the service sends plain text and stays simple. The
licences of the two Pierre packages have not been read yet, and they must be read before
any code is copied. Monaco, the editor inside Visual Studio Code, was not chosen, because
it does not support phones.

The owner chose CodeMirror 6 for the Files tab on 2026-11-12, because the tab now edits
files and the Pierre packages only display them. CodeMirror 6 works on the web and in a
phone's web view. The mock uses it, from one bundle in
[third_party/codemirror/](../third_party/codemirror/README.md), because the mock has no
build step. The Pierre packages remain the choice for the diffs in the Git tab.

## Plans written as HTML pages

The owner decided this on 2026-11-11. It is agreed and not built. From now on,
every plan in LGTM Studio is an HTML page. Planner agents fill in one template, and the
page is written to get the point across to the owner in a human-friendly way. The
plans the agents work from keep their full detail. Each sprint has its own plan, shown on
the sprint page beside the sprint's tasks. The tasks are the short version of the plan,
and each task links to the part of the plan it carries out. The mock shows sprint LS-A01-7
with a real LGTM Studio plan converted to HTML. The template is not written yet.

## An IDE button that starts a code server on demand

The owner raised this on 2026-11-14. It is an idea, not yet agreed. The owner is first
finding out what they use an IDE for, after the Files tab is in use.

- LGTM Studio does not embed Visual Studio Code in its own page. The Files tab stays the
  small editor, because it works on phones and keeps the Git review rules.
- An "IDE" button starts a dedicated code server session for the inspected worktree, on
  the machine that holds it, and opens it in its own browser tab.
- A monitor watches that session's connection. When no tab has been connected for over
  60 seconds, LGTM Studio stops the code server.
- Opening it again starts a fresh session the same way, so it acts like an IDE that
  launches on demand.
- Open questions: which code server to use and its licence, how it is authenticated
  behind LGTM Studio's single login, how it reaches a remote machine, and how its Git
  panel relates to the owner's review of merges into the base branch.

## Files and images sent with a message

The owner asked for this on 2026-11-11. It is agreed and not built in the engine. The
owner can send files and images to an agent with a message. They come from a paperclip
button in the message box, from a drop on the message box, or from a paste. Images show
as small pictures before sending and as larger pictures in the thread. Other files show
as a name and a size. The mock limits each file to 100 megabytes, and that limit is not
decided. The engine would need to store the files, pass them to the agent's harness, and
serve them back to every client through the client API.

Large photos are resized before they are sent, decided by the owner on 2026-11-11. A photo
from a phone can be 50 megabytes or more, and the models scale large images down anyway.
The rules the mock follows are these:

- A photo whose long edge is over 2,048 pixels, or whose file is over 2 megabytes, is
  redrawn on the owner's device with a long edge of at most 2,048 pixels.
- The redrawn photo is saved as a JPEG at 85 percent quality. A 33-megabyte test photo
  became 1.1 megabytes.
- Redrawing drops the photo's camera details, including the place where it was taken.
- The chip shows "Resized from 33.4 MB to 1.1 MB", with "Send original" to send the file
  as it was. A sent photo is captioned with both sizes.
- Animated images and drawings in the Scalable Vector Graphics format are sent unchanged.
- A photo that the browser cannot read is sent as a file. A HEIC photo, the iPhone's own
  format, is marked "Converted by the service", because Chrome cannot read it. Raw camera
  files, such as ProRAW, are sent as files.

The service must apply the same rules to images that other clients send at full size. The
common server image library, sharp, cannot read HEIC in its standard build, so the service
needs a separate converter for it.

### Safety of uploaded files, required by the owner

The owner set this requirement on 2026-11-14.
An uploaded file is untrusted data, even from a signed-in owner page, because a page or a pasted file can carry a malicious payload.
The path of a file is: the page, then the server, then the client machine that runs the agent, then the agent's harness.
Every step on that path must be safe on its own.
The design must meet these points before any upload is built.

- The library that parses uploads on the server must be a well-maintained one with a record of security fixes. The choice and its reasons are written down before it is used.
- The server enforces a size limit and a count limit while the bytes arrive, not after.
- The server never trusts the file name, the folder path or the declared type that the page sends. It gives each file its own stored name. It finds the real type from the bytes.
- The server stores uploads outside every project folder and outside every folder it serves. It never runs an upload, and never unpacks an archive.
- The server and the client machine treat image decoding as dangerous. Decoding and converting run with tight limits on memory, time and image size, apart from the main service process if possible.
- A drawing in the Scalable Vector Graphics format can carry scripts. It is never shown as a live page element. It is shown as a flat picture or as a file.
- A file served back to a page is sent with headers that stop the browser from running it or guessing its type.
- The client machine checks a file again when the server forwards it: size, stored name and type. It does not rely on the server's checks alone.
- The agent's harness receives a file as data in a folder made for that message. The file never lands in the worktree unless the owner asks.
- Agents cannot upload through this path. Only the owner's signed-in page can.
- A security review with real malicious samples is part of the work, and SECURITY.md gains a section for it.

## Pushing a finished project and opening its pull request

The owner review flow was connected to GitHub.com on 2026-11-17.
Prepare pushes the registered project branch with the service user's signed-in GitHub CLI and creates or reuses an open pull request.
Approval requests a merge commit and conditionally fast-forwards the local base.
The existing review area shows the mode, provider link, state, and local update result.
Missing GitHub support keeps the explained local merge fallback.
Automatic push triggers and manual provider text editing below remain planned.
The pull request flow reads the live remote base independently of stored pull request metadata.
A fresh owner approval reconciles a confirmed remote merge after a lost receipt.
Refresh always reads Git state and pushes only a changed project head.
Hosted comparisons use the fetched live remote base even when the local base is dirty or behind.
Prepare also recognizes an exact-head request merged elsewhere after a previous refusal.
Reusing an open request refreshes its redacted description.

The owner raised this on 2026-11-12 and decided its shape the same day. It is
**agreed**, and it is not built in the engine or the mock. Agents merge their work
locally, from an agent's worktree into its team branch and from the team branch into the
project branch. Nothing reaches the remote during that work. When the project branch
holds everything, LGTM Studio pushes that one branch and opens a pull request.

LGTM Studio is meant to be the one place where the work happens, so the merge button
lives in LGTM Studio, not only on the code host. The steps are these:

- **The project lead asks.** When every team branch is integrated, every worktree of
  the project is clean, and no merge is in progress, the project lead asks the owner a
  question. It says that everything is ready, summarises where the work stands, and asks
  whether to start a pull request. The question carries an "Open pull request" button.
- **The base can be changed.** A small menu next to the button chooses the branch the
  pull request merges into. By default it is the branch that the project's worktree was
  made from.
- **Push and open.** LGTM Studio pushes only the project branch. Team and agent
  branches stay local. It then opens the pull request through the code host, such as
  GitHub, with a title and description that the owner can edit.
- **Review in the Git tab.** To read the pull request's changes, the owner opens the
  project's Git tab. The project lead stays open in the side panel meanwhile.
- **Watch.** The project shows the pull request's number, a link, the state of its
  checks, and its review state. Later commits on the project branch, such as fixes
  after review, are pushed to the same pull request.
- **The final merge goes through the project lead.** The owner approves the merge in
  LGTM Studio, and the project lead carries it out. A merge made on the code host is
  noticed, and the project is marked merged.
- **No remote.** A repository without a remote keeps the local merge that the mock
  shows today.

The fix-sprint exception in rule 7 of AGENTS.md could use the same path, with the pull
request merged automatically after a passing full test run.

## Simple editing in the Files tab

The owner raised this on 2026-11-12 and decided its shape the same day. It is
**agreed**, and the mock shows it. It was built on the branch `area/files-settings` on
2026-11-14, without the generated commit message. The Files tab is a
small editor for minor changes, without autocomplete or other language features. The
owner edits only one file at a time. Anything bigger is work for the project lead, and the
owner asks the lead to do it.

- The editor is CodeMirror 6, as "Which component shows the files" above says.
- An edited file shows a dot in the folder tree until it is saved.
- A "Save" button at the top of the file saves it. The keyboard shortcut is Control+S,
  or Command+S on a Mac.
- A saved file is an uncommitted change in the inspected worktree. The tree marks it as
  changed, and the Git tab shows its diff.
- After a save, a "Commit" button appears at the top of the file. It opens a small dialog
  for the commit message, and it commits that one file as the owner. The message can be
  written by the cheap model, as in the Git tab.
- The base branch is read-only, because agents never edit it directly.
- When the worktree's agent is working, the tab says so above the file. After a save, that
  agent is told which file the owner changed.
- A save carries the version of the file that the owner opened. If an agent changed the
  file since then, the save is refused and the owner sees both versions. The mock does not
  show this case.
- Writes follow the same path check as reads, and nothing under `.git` can be written.
- Every action is a client API call, as rule 5 of AGENTS.md requires.

## A database tab with one keeper of migration numbers

The owner raised this on 2026-11-12 and decided its shape the same day. It is
**agreed**, and it is not built in the engine or the mock.

### Where the databases are listed

A repository lists its databases in its LGTM Studio config file, in the repository's
`.lgtm` folder. Each entry names the database, its kind, such as PostgreSQL or Firestore,
and where its schema and migrations live. When an agent sets up a new database, the entry
is written into the config file, so the list stays current. How the agent's change reaches
the config file is not decided yet, because agents cannot write to `.lgtm` today.

A project shows a "Database" tab only when its config file lists at least one database.

### What the tab shows

When the config file lists more than one database, a small bar at the top of the tab
switches between them, with a menu. With one database, the bar is hidden. The view
depends on the kind of database, and it stays simple:

- **Relational databases**, such as PostgreSQL and SQLite, show their tables, columns,
  keys and indexes, and the list of migrations.
- **Document databases**, such as Firestore and MongoDB, show their collections, the
  fields their documents usually carry, their indexes, and their security rules. Their
  migrations are usually scripts that rewrite data, and those are listed too.

Other kinds, such as key-value stores, wait until a project needs them.

### One keeper hands out migration numbers

Parallel agents that each write a migration pick the same next number, and the merges
then conflict. LGTM Studio's service keeps one counter for each migrations folder. An
agent asks the service for a number before it writes a migration. No two branches hold
the same number.

The config file only lists the databases. The service tracks the migrations in its own
local SQLite database, where it records these things:

- which number was given to which agent, on which branch;
- which numbers are already in the base branch;
- which numbers are still being worked on;
- whether each migration's tests have passed, and on which commit.

A number is a reservation until its migration reaches the base branch. Branches often
merge in a different order from the order they reserved in. For example, branch 1
reserves 123 and branch 2 reserves 124, while the base branch ends at 122. If branch 2
is ready first, it merges as 123, the next number after the base branch. The service
then tells branch 1 to renumber to 124 and to run its tests again on the new base. So
the final numbers follow the order of the merges.

Renumbering depends on the migration tool:

- Tools that order migrations by a number in the file name, such as Flyway and
  golang-migrate, need a rename. LGTM Studio can do that rename itself.
- Tools that chain each migration to its parent, such as Alembic and Django, need the
  parent reference rewritten. The agent that owns the branch does this.
- Tools that name migrations by timestamp, such as Rails, Prisma and Drizzle, rarely
  collide on names. They can still apply migrations in an order that nobody tested, so
  the rerun of the tests on the new base still applies.

## A one-sentence reason on every approval request

The owner asked on 2026-11-13 for an approval card to say what is being attempted.
The live website can name only the program, because a harness reports the command and not the reason.
The engine could ask the requesting agent for one sentence of intent, or take it from the agent's last message before the tool call.
The card would then show that sentence as its summary.

## One Harnesses page in place of harness settings and Subscriptions

The owner asked for this on 2026-11-13.
One page lists every harness.
Each harness shows its accounts and subscriptions.
A button on a harness adds another account or subscription.
Adding an account checks whether the harness is installed.
A harness that is not installed shows its install script and is installed from the page.
"Subscriptions" then leaves the left navigation.
The mock and the live website now provide this page.
The old address opens Harnesses.
Harness defaults now live beside the accounts.

## A harness installer

The owner asked for this on 2026-11-13.
LGTM Studio installs a missing harness by itself.
The page shows the install script before it runs.
The engine now provides owner-only status, recipe, install, and local sign-in reads.
Gemini CLI has a pinned npm recipe with a dependency lockfile.
Automatic installs disable package scripts.
Claude Code and Cursor show their official commands for the owner to run manually.
The bundled Claude and Codex runtimes count as installed without separate command-line installations.
Codex has nothing to install separately.
The installer checks the staged manifests and installed package metadata before running a new executable.
The engine checks managed file hashes before running an installed harness.
Provider sign-in stays in the owner's terminal.

## Notifications with more than one kind of delivery

The owner asked for this on 2026-11-13.
The page and its navigation entry are named "Notifications".
Delivery kinds are added, starting with email and ntfy, and others may follow.
Email is sent through the user's own mail account, with the user's own credentials or key.
LGTM Studio's maker never sends the email.

The engine and website support independent email and ntfy delivery as of 2026-11-13.
Email uses the owner's Simple Mail Transfer Protocol (SMTP) server through Nodemailer.
The owner can remove its settings and store its password from the terminal.
Quiet hours expose a held state and allow an override for agent failures, account problems, and a web server outage.
Each configured carrier has an enable switch and a test action.
Tests report whether that carrier accepted the message.
Quiet hours use a start, an end, and a named time zone.
The owner can let failure alerts through during quiet hours.
Sprint completion events raise sprint alerts, which are off by default.
Opening an owner merge review raises a merge alert.
Passwords and access tokens stay in the owner's secret store.
Clients receive only whether a secret is set.


## A Workflows tab with a visual editor

The owner asked for this on 2026-11-13.
The build loop is shown in a "Workflows" tab.
A user sees the default workflow as a picture.
A user clones it and changes the copy in a visual editor.
A user can also make a workflow from nothing.
The definition store, typed API, terminal commands, and visual clone editor are implemented.
The format and storage rules are in [the workflow document](workflows.md).
The visual editor can add and remove every phase and block in a copy.
A separate creation action and project workflow selection remain planned.

## Model choices before an agent starts

The client application programming interface can discover a harness account's model choices.
The engine opens a temporary Agent Client Protocol session without sending a prompt.
The engine closes the harness before the request finishes.
The result keeps the exact model identifiers and the reported labels.
A separate reason explains an unavailable model list or effort list.
The engine remembers successful results for five minutes in memory.
A client can explicitly refresh a result.
A client can request a model's effort choices.
Reports remain separate for each account and selected model.
The live website uses these reports for its model choices.

Failures and empty reports are remembered for thirty seconds.
Concurrent requests for one report share a discovery session.
At most two discovery sessions run in the engine process at once.
Service settings shows each harness's report as it arrives.

## Discovery without persistent harness history

A future harness-specific discovery mode could disable session persistence through supported harness interfaces.
It must preserve the account's login and the owner's existing history.
The investigation and its limits are recorded in issue 719.


## Git actions after the website connection

The owner requested the remaining Git actions on 2026-11-13.
The local engine, client application programming interface, command-line program, and live website now support aborting a merge, reopening a resolved file, sending a main review back with a reason, updating from a registered parent, and integrating a registered builder worktree into its team.
The owner approves every merge into the base branch.
Sending a review back records a distinct outcome and addresses its reason to the project lead on the board.
Git text generation needs an owner-configured account and model with a bounded short-task execution boundary.
Hosted pull request editing now has a durable GitHub delivery record.
It still needs an authenticated provider text update action.
Those generation and editing controls remain refused until their prerequisites exist.
Remote-machine Git remains unavailable.

## Sign in to a harness from the page, with a login folder for each account

The owner asked for this on 2026-11-14.
"Add account" on the "Harnesses" page should run the harness's sign-in for the owner, in a terminal shown beside the dialog.
Each added account should get its own login folder under `~/.lgtm`, made by the service, so one harness can hold two sign-ins.
The builder `s7-signin` is building both.

## A choice between the bundled harness program and the one installed on the machine

The owner asked on 2026-11-14 whether agents should run the vendor's installed program and not the copy that arrives with the adapter package.
Today the engine runs the copy that the Agent Client Protocol adapter depends on, at the version the lockfile pins.
A setting for each harness could offer "Bundled with LGTM Studio" and "Installed on this machine".
The owner has not ruled on it.

## Offer a sign-in that is already on the machine

The owner asked for this on 2026-11-14.
The "Harnesses" page lists an account only after someone adds it.
A harness can already be signed in on the service's machine, in its own default folder.
The page should notice that and offer it in one step, for example "A Claude Code sign-in was found on this machine. Use it?".
The page must not add the account by itself, because an account lets agents run as the owner.
The check must read only whether a sign-in exists, as "Check again" does today.

## A simpler way to set up harnesses and subscriptions

The owner said on 2026-11-14 that adding a harness account feels weird and should be simpler.
The "Harnesses" page and its "Add account" dialog need an overhaul.
The owner plans to run that overhaul as the first project inside LGTM Studio itself.
The section "Offer a sign-in that is already on the machine" above is one part of it.

## One agreed layout for the `~/.lgtm` folder

The owner asked for this on 2026-11-14.
The folder `~/.lgtm` in the owner's home should hold a separate home for each subscription, the configuration, and the owner's overrides.
Today it holds login folders under `~/.lgtm/accounts/<harness>/<account id>` and workflow overrides.
The service's data lives elsewhere, in `~/.local/state/lgtm`.
The layout has not been designed as a whole.
The design should say what each folder holds, which files are secret, and what a backup must include.

## A better build loop

The owner said on 2026-11-14 that the current build loop is not good enough and needs a better way.
No replacement has been designed.
The owner wants the Terminal finished first.

## A full-size OpenPencil editor beside the chat

The owner asked for this on 2026-11-14.
In edit mode on the Mocks screen, the OpenPencil editor sits in a small area.
A maximize button should let the editor fill the page, with all of OpenPencil's own tools.
The chat stays open at the side while the editor is maximized.
The button returns the screen to its normal layout when it is pressed again.
This changes the look of an existing screen, so it needs the owner's approval of a mock first.

## Click an object in a mock to refer to it in the chat

The owner asked for this on 2026-11-14.
In view mode on the Mocks screen, the owner should be able to click one object in a mock.
The click puts a reference to that object into the chat composer.
The agent that receives the message then knows exactly which object the owner means.
Still to decide: what the reference carries, for example the object's identifier, its name, its screen and the mock revision.
Still to decide: how the reference looks in the composer and in the sent message.
The section "Files and images sent with a message" above covers a related need.

## A sprint is proposed on the board, with one approval

The owner asked for this on 2026-11-14, after a long acceptance request for a first sprint.
When an agent proposes a sprint, it should put the proposed cards on the Kanban board.
It then asks one short question, such as "I made the following changes. Do they look fine?".
The owner reads the proposal on the board, because a board is easier to read than a long card.
"Accept" keeps the cards.
"Deny" makes the agent remove the proposed cards and try again.
Proposed cards must look different from accepted cards until the owner accepts them.
This is for sprint and board changes only. Other approvals keep the short card with "Details".

## Paths by the size of work.

Requested by the owner on 2026-11-14.
The lead may propose skipping planning when the work is small.
The lead agrees that path with the owner before building starts.
The permission settings added on 2026-11-15 determine whether a separate permission card is needed.
A build team's lead may do the work itself without starting workers.
The project lead may propose doing a one-off small correction itself.
Unit s10-lead builds the opening instruction and the engine's approval gates for these paths.
Automatic choice of a path by the workflow is not built.

## A one-off agent on a repository, without a project.

Requested by the owner on 2026-11-14.
The owner asked for a way to make small, one-off changes without a project.
From a repository, the owner starts one agent by choosing a harness.
The agent takes the default model and effort.
The owner can change the model and effort in the composer bar at the bottom of the chat.
No project, sprint or team is needed.
The worktree and branch that the agent uses are undecided.
How the change is merged is undecided.
How the agent appears in the tree is undecided.
This feature is not built in unit s10-lead.

## The lead has two independent permissions, allowed by default.

Requested by the owner on 2026-11-15.
The owner decided that these settings are lead permissions, not approvals, and that each one says what it allows the lead to do.
The permission for project planning is allowed by default, because creating sprints, teams and tasks is the lead's main job.
The owner's concern was a lead that jumped straight to building before it understood the request.
The lead must first work with the owner to understand the request.
Both permissions are allowed by default.
The owner noted that the building permission need not depend on the end result, and accepted it as it is for now.
Each permission gets a dropdown with the options "Allowed", "Not allowed" and "Always ask for permission".

Unit s11-permissions builds this change.
"Lead permissions" replaces "Approvals" in Project settings.
"Project planning" says, "The lead can create sprints, teams and tasks."
"Starting work" says, "The lead can start workers and move tasks out of the queue."
Each dropdown offers "Allowed", "Not allowed" and "Always ask for permission", in that order.
Both permissions default to "Allowed" on new projects and existing projects without a recorded setting.
Only the owner can change a setting through the website, the command-line interface or the client application programming interface (API).
A permission set to "Not allowed" refuses its gated actions and any request to grant it.
The refusal tells the lead that the owner turned it off and that it cannot be requested.
The permissions are independent.
Withdrawing Project planning leaves Starting work intact.
Starting work can be granted before Project planning.
The gated actions are unchanged from unit s10-lead.
An owner action remains ungated.
A worker can still finish or annotate its own started task.

The orchestrator interprets "Always ask for permission" as requiring one explicit owner grant that lasts until the owner changes that setting or withdraws the grant.
It does not mean asking again before each action.
The engine continues to require the owner's exact "Accepted" when granting through a card.
An ask permission displays "Granted" or "Not granted" beside its dropdown.
Only a granted ask permission shows "Withdraw".
Changing a setting clears that permission's grant and closes its open cards with a fixed note.
Choosing "Always ask for permission" starts from "Not granted", including on a project approved under pull request 58.
The lead must ask for a new grant after that change.
The other permission is unchanged.
Old approval events still replay and retain their recorded grants.
Historical grants do not override an explicit denied setting.
No permission change moves the project stage backwards.
The engine writes fixed card sentences using the new permission names and the same words as the help lines.
A combined request becomes a card for only the permissions that still need a grant.
An answer reports "Approval recorded." only when that card grants a permission.
A declined card does not report a grant made elsewhere.
The two outcome sentences remain "Approval recorded." and "Approval not recorded. Ask again if you still need it."
The old scope-withdrawn outcome is no longer produced.

A new project still starts in Define.
The lead first works with the owner to understand the goal and agree the end result.
It asks an ordinary confirmation question before creating sprints, teams or tasks.
A permission allowed by default does not remove that conversation.
The opening instruction keeps the small-work and nitpick paths requested on 2026-11-14.
If an action is refused because a permission requires a request, the lead uses `ask` with `projectApproval`.
If an action is refused because the permission is off, the lead tells the owner and stops attempting that action.

## Project stages follow newly recorded planning and work.

Fixed after review of unit s11-permissions on 2026-11-15.
Creating a sprint, team or queued task moves a project from Define to Plan in the same command batch.
Starting or resuming a project worker moves Define or Plan to Build in the same batch.
Creating a task outside the queue or moving a task out of the queue also moves Define or Plan to Build.
These automatic moves do not move Design or later stages backwards.
They never advance beyond Build.
The owner's explicit stage changes remain available.
The lead may use `project.stage.set` when an automatic move does not fit.
These moves are explicit events in new command batches.
Replaying older stored events does not infer new stage moves.
New projects still start in Define.
The lead still confirms the end result with the owner before planning.

Changing a permission setting closes affected permission cards in the same batch.
The engine writes the fixed note, "Permission request closed because the owner changed its setting."
Withdrawing a grant also closes its affected cards with a fixed withdrawal note.
The closed card cannot be answered again.
An unrelated permission card and an ordinary question stay open.
The lead receives the closure through the ordinary question-answer delivery.
A combined card closes when either of its requested permissions changes.
The lead can request the other permission in a new card.


## Project settings controls awaiting engine support

Unit s14 hides unsupported settings on the website through its build-time replacement.
This is the orchestrator's choice for the owner's question about hiding controls or labelling them “Not built yet”.
The owner can overrule that choice.
The standalone mock keeps the complete sample design.

- The task label prefix needs a stored project prefix and task label allocation that uses it.
- The base branch is now a stored repository setting, implemented by unit s30.
  The owner command `project.repository.base.set` validates an existing local branch.
  It refuses an open review, an unfinished Git operation, an active merge, or an unfinished project creation.
  The website shows the recorded base through its existing branch display.
  A website control for changing the base branch remains unbuilt.
  The command line and client application programming interface can change it now.
  The main checkout must still have that base checked out for project creation and local review approval.
- The project permission picker needs an owner setting and a defined way to apply it across existing agents without exceeding their permission ceilings.
- Write tests first needs a stored project policy and enforcement by the build workflow.
- Agent review needs a stored project policy and enforcement when assigning reviewers and handing off work.
- Remove agent credits when merging needs a stored project policy and a merge path that applies it without rewriting unrelated commits.
- Assignment needs stored work-kind choices for the harness, model and effort.
  It needs owner approval, delivery of that approval to the lead, and enforcement when agents start.
  Its Approve action, Every row model picker and Apply action need that same engine support.
- The lead subscription picker needs a supported account reassignment command and defined handling for the running lead's session.
  The website keeps the current subscription as a read-only fact beside its working model and effort controls.
- Need-to-know needs a stored team policy and enforcement in the board's read and delivery rules.

The machine row already shows the registered machine as a read-only fact.
Changing project placement remains unavailable until machine pairing exists.

## Optional removal of a deleted project's worktree folder

Project deletion currently keeps all folders and branches on disk.
The engine has no public worktree removal operation.
The internal rollback for a refused worker allocation only removes its unchanged new checkout.
The website therefore has no removal checkbox.
A future removal path must prove that the engine created the exact worktree for that project.
It must verify the worktree's identity and refuse uncommitted changes.
It must never remove a branch or an externally registered folder.

## Automatic worker worktrees

State: Implemented in unit s23-worktrees, pending merge.

Starting a project worker allocates a registered child worktree and a safe worker branch.
The starter's current committed head supplies its starting point.
The owner can select a project through the client application programming interface.
The start command can select an existing registered worktree for a reviewer or another intentional reuse.
Leads keep their current checkout.
The existing website shows the worker in its Git, Files, and Terminal choices.
Resume and recovery reuse the stored folder.
Removal preserves worktrees and branches.
Agent callers use read-only permission when selecting another checkout.
The same rule applies when a lead's supplied folder resolves into another registered project checkout.
An owner can select a registered project checkout at any permission level.
Implicit owner project discovery applies only to worker roles.
Test runners receive their own writable worktree after the lead merges the builder's branch.
An agent that is not a lead must own the checkout it allocates from.
Resume refuses a checkout that now belongs to another agent.
Allocation accepts an uncommitted source and copies only its last commit.
An unchanged sessionless allocation can be reused after a harness start fails.
That retry keeps its worktree identifier and updates its owner.
Remote allocation waits for machine pairing.

The owner's decision of 2026-11-17 permits lead starts in registered worktrees inside home.
Only the starter's current project worktree, team worktrees and engine allocations grant that exception.
The home itself, state folder, plugin root, home dotdirectories and configured login folders remain protected.
Recovery and resume refresh the registration before spawning.

## Automatic permission defaults

State: Implemented in unit s26, under the owner's decision of 2026-11-17.

New harness defaults use `auto` for project leads.
A worker start without a permission level uses the lowest of `auto`, the saved harness default and its starter's level.
The ordered levels are `read-only`, `ask`, `edit`, `auto` and `full`.
Claude uses its `auto` mode.
Codex uses its `agent` automatic review mode.
An unsupported or historical mapping falls back to edit and reports that fact in harness status.
The website offers Automatic review alongside the existing choices.
Existing agents keep their levels.
Unrelated owner edits, custom permission mappings and non-ask harness defaults are kept.
Factory-identical permission mappings and reported modes receive automatic mode fields at read time, even when other fields were edited.
Every stored ask default reads as auto under the owner's decision of 2026-11-17.
The website never offered a control for that harness field.

## Reconciliation of interrupted worker allocations

State: Idea.

A durable allocation intent could reconcile a Git checkout created just before a hard service crash.
The recovery path would verify the original folder and branch identities before registering or removing anything.
See issue 1254 in `docs/issues.md`.

## Removal of finished worker checkouts

State: Idea.

An owner-directed command could remove a worker checkout after proving that its branch is merged into its parent and that its folder is clean.
See issue 1253 in `docs/issues.md`.

## Roster work descriptions from existing tasks and threads

State: Built in unit s25-smallfix.

The roster shows an active task title for a working agent when one is available.
Without an active task, it uses the latest mapped tool summary or delivered message from the thread it already reads.
The description preserves redaction and occupies one bounded line.
An idle or stopped agent retains its lifecycle description.
See issues s25-3 and s25-5 in `docs/issues.md`.

## A website control for a harness's default permission

State: Planned as an idea recorded in round 3 of unit s26.

The website does not offer a control for a harness's `defaultPermission` field.
The owner can currently choose a project lead permission in the existing Defaults editor.
A future harness control should let the owner choose that separate harness default.
It needs a stored indication of an explicit choice so a newly chosen ask default can survive the historical upgrade.
This unit adds no website control.

## A terminal view of each harness

State: Idea. Requested by the owner on 2026-11-17, to be added later.

The owner wants to see a harness's own terminal program from the website, as a view of an agent.
The "Terminal, Git, machines, and subscriptions" section above already plans a main conversation that can switch to a terminal, and taking over an agent's terminal.
An agent run in ACP (Agent Client Protocol) mode has no terminal program to show, so this view depends on the interactive run mode or on a rendering of the agent's session.
`SECURITY.md` governs who may read and type into such a terminal.

## Keyboard shortcuts for navigation

State: Idea. Requested by the owner on 2026-11-17, to be added later.

The owner wants keyboard shortcuts for moving around the website, in the manner of Vim and Visual Studio Code.
Examples are moving between projects, tabs, agents and panes, and opening the quick switcher, without the pointer.
The shortcut sets, whether they can be chosen or remapped, and how they coexist with typing in the chat composer and the terminal are not decided.

## An escalation tool and an escalation agent

State: Idea. Requested by the owner on 2026-11-17, to be built later.

An agent whose command is blocked by its permission level, for example by the harness's automatic review, calls a tool named "request escalation".
The call carries the exact command and the agent's explanation of what the command is for.
The engine sends the call to an escalation agent that runs with full permissions.
The escalation agent authorises the command, refuses it, or asks the owner.
Open points are recorded here and are not decided.
The explanation is text from an agent, so rule 7 of `SECURITY.md` treats it as data, and the escalation agent must judge the command itself.
A request that touches a secret, an account, a persistent setting or a protected folder should always reach the owner.
An authorisation should apply to one exact command, once, and should be stored as an event that the owner can read.
The idea is related to the standing permission rules, which remain unbuilt.

## Tool scripts that keep intermediate results out of the agent's context

State: Idea. Requested by the owner on 2026-11-17, to be built later.

An agent that needs several tool calls in a row writes one short script.
The script runs the calls and passes each result to the next call.
Only the script's final output enters the agent's context.
The technique is known as programmatic tool calling, or code execution with the Model Context Protocol (MCP).
The harness owns each agent's tool loop, so LGTM Studio could offer this only for its own tools, as one tool on its MCP server that runs a script.
Every call inside a script would need the same caller checks, limits and redaction as a direct call.
Running agent-written code inside the engine is a new security surface and needs a sandbox decision.

## Choosing the lead when a project is created

State: Implemented on 2026-11-19, following the owner's request on 2026-11-18 and his subscription-selection clarification on 2026-11-19.

The New project dialog starts with the saved lead and lets the owner choose the harness, subscription, model and effort for that project.
Model and effort choices come from the selected harness account.
Changing the account clears the previous model and effort.
Changing the model reads that model's effort choices.
The saved defaults do not change.
The client application programming interface (API) accepts the same optional lead selection for placement preview and project creation.
The engine validates the selection before allocating a project, and a retried creation keeps its recorded lead.

## Custom harnesses

State: Idea. Requested by the owner on 2026-11-18. The owner will build it through LGTM Studio.

LGTM Studio supports four harnesses today: Claude Code, Codex, Gemini and Cursor.
The owner wants a survey of the major harnesses first.
The owner then wants a simple way for a person to declare a custom harness.
The engine already stores a settings row for each harness, and issues 1399 and 1457 describe gaps for custom rows.

## LGTM Studio as a skill for other agents

State: Idea. Requested by the owner on 2026-11-18. The owner will build it through LGTM Studio.

The owner wants everything about LGTM Studio available as a skill.
A person's own agent could then build a workflow, or work with LGTM Studio in another way, by reading that skill.
`AGENTS.md` rule 6 already says that other workflows override the default in `~/.lgtm` or in a repository's `.lgtm`.

## A testing skill with fixed rules

State: Idea. Requested by the owner on 2026-11-18. The owner will build it through LGTM Studio.

The owner finds testing in this project disorganised and expensive today.
He wants a skill that sets specific rules for how testing is conducted.
The aim is that agents do not spend compute on redundant test runs, for the owner and for other users of the software.
The skill applies both to LGTM Studio's own development and to any project built with it.
Issue 1458 and the earlier load-sensitive journey issues record what the current practice costs.

## Test tools for each language, added by pull request

State: Idea. Requested by the owner on 2026-11-18. The owner will build it through LGTM Studio.

The owner uses JavaScript, TypeScript, Go and Python, and sometimes C++.
LGTM Studio would ship test tools with scripts for some languages, starting with TypeScript and Go.
Other people extend it by opening a pull request that adds a test tool for another language, such as Rust.
The testing skill tells an agent adding a tool to read how the existing tools work and what the overall rules are, before it writes the new scripts.
The same extension point covers simulator testing and testing on a physical phone.

## A Define workspace with a conversation and an evolving end-result document

State: Idea. Requested by the owner on 2026-11-19, after using both issue 940 projects.
The layout and the interaction rules still need the owner's agreement.

The owner wants precise control while agreeing what a project will build.
The current question card contains the lead's whole proposal and can bundle several decisions into one answer.
The owner suggested a chat-centric Define view, with suggestions beside the conversation.
The end-result document should be a persistent draft that the lead's harness edits as the conversation develops.
The owner should be able to see the current proposed result without reconstructing it from older messages or cards.

The proposed direction is a main conversation beside the current end-result draft and its unresolved questions.
Each question would have its own answer, so accepting one suggestion would not accept unrelated choices.
Suggestions would identify the statement they change.
The draft would distinguish proposed wording from agreed wording and show subsequent changes.
Agreement on the end result would be separate from permission to plan or start work.
Small projects should be able to finish Define without inventing a sprint structure.

Whether this is a new tab or a different opening view of the existing project is undecided.
Direct owner editing, the treatment of conflicting edits, and the exact approval controls are also undecided.
The existing website remains the only website.
Any implementation must expose the same document and decision actions through the client application programming interface (API).
Issue 1464 records the observed question-card defect separately from this broader idea.

## A repository board with project prefixes on its tickets

State: Idea. Requested by the owner on 2026-11-19, to be changed later.

The owner wants the work board at the repository level.
Each ticket would show a prefix identifying its project.
The reason is that a separate board is mostly wasted space for a small project.
The repository board would make work across those projects visible together.

The ticket unit, the relationship to existing sprint and task records, and the prefix format are undecided.
Filtering the shared board to one project and any separate treatment for very large projects still need design.
This request concerns the work board shown on the website.
It does not decide changes to the agents' message-board channels, delivery rules or access boundaries.

## A "Where is your workspace?" setting in the Service settings

State: Built on 2026-11-20 in unit s32-workspacesetting.
The owner requested it on 2026-11-20.

The workspace folder is the one folder that must contain every repository that LGTM Studio may work in.
The owner can choose it in the first-run steps.
The Service settings screen now asks "Where is your workspace?" in Defaults.
The command `project.settings.set` remains available too.

It shows the current folder and lets the owner change it.
The engine validates the folder as an existing absolute directory.
The helper says, "LGTM Studio's Git commands work in every repository under this folder."
A change that excludes a registered repository is refused with the first excluded repository's name and path.
An unchanged folder skips containment validation so legacy installations can still save other settings.
A changed folder must contain every registered repository, location, workspace and common Git directory.
Repositories without projects still count.
Saving the folder preserves the other settings, including the lead defaults and the explicit permission marker.
The standalone mock uses Maya Okafor's sample path.

Issue 1466 records the fault that led to the request: a repository in a sibling folder could not be registered.
On 2026-11-20 the owner's own workspace folder was widened by command from one folder of projects to its parent.

## Website controls for repository worktree preparation

State: To build.
Recorded during unit s31 on 2026-11-20.

The owner can set copied path patterns, submodule initialization and agent instructions through the client application programming interface (API) command `project.repository.preparation.set`.
Repository reads include those settings.
The website needs controls to read and change them.
The owner's approval of their appearance remains required.
Unit s31 adds no website controls or screen changes.

## Environment variables that LGTM Studio gives to agents

State: Idea. Requested by the owner on 2026-11-20.

The owner wants to store environment variables in LGTM Studio for a repository.
LGTM Studio would provide them in the shell of every agent that works in that repository.
Any script that an agent runs would then see those variables, as it would in the owner's own shell.

This would remove the need to copy ignored environment files into each worktree for values that are plain variables.
The request came while preparing a repository whose builds need ignored environment files; issue 1469 records that fault.

Undecided: whether variables are set per repository, per project, or both.
Undecided: how secret values are stored and shown, and whether they follow the existing rules for account secrets.
Undecided: which agents receive them, and whether the terminal tab receives them too.
`SECURITY.md` governs this; a stored secret must never reach a log, an event or a trace.

### A built-in vault in the server process

State: Idea. Stated by the owner on 2026-11-20, a little after the entry above.

The owner is thinking of a vault that is built into LGTM Studio.
The server process holds the vault, and the vault is encrypted where it is stored.
The vault holds the environment variables of each repository and similar values.
When a project is launched, it is given access to the vault.

The owner also said that LGTM Studio will move from a single program to a server with a client website.
How the vault and the other parts are split between server and client is not decided; the owner will discuss it.

Undecided: what unlocks the vault when the server starts, and where its key lives.

The owner said on 2026-11-24 that OpenBao is launched and is where the secrets are held.
OpenBao is an open-source vault, forked from HashiCorp Vault.
This repository has no code or setting for OpenBao yet, so how the server reaches it is not written down.
The owner prefers a GitHub App for the server's access to GitHub, so that clients share one delegated access and the owner does not log in on each client.
The app's private key would be kept in OpenBao.
Decided by the owner on 2026-11-24: pushes and pull requests appear as the app's bot by default. The owner can choose to appear as themselves, for the whole service or for one project. Both settings are drawn on sample data in `src/frontend/`. The engine has neither.
Undecided: whether a project receives the whole repository scope or named values only.
Undecided: how this relates to the existing stored account secrets.

## A repository agent that works in the main checkout and prepares a repository for projects

State: Idea. Requested by the owner on 2026-11-20.

The owner wants one agent per repository, at the repository level and above its projects.
It always works in the main checkout only.
The owner talks to it about one-off matters of that repository.

Its first job starts by itself when a repository is added.
Its automatic first prompt asks the agent to verify that the repository is ready to work.
The prompt points the agent to the relevant documents of LGTM Studio.
It asks the agent first to go through the repository and work out what it is.
The agent then fills in what LGTM Studio needs as configuration for that repository.
Examples are the base branch, the files to copy into new worktrees, submodules, setup instructions, environment variables and the test commands.
If the repository has tests, running them is part of that first run.
The result is saved to the main checkout.
After that, the repository is ready: the owner starts a new project, states the goal, and the work gets done.

This extends the earlier entry "A project setup agent", which would produce a `.lgtm` folder.
It depends on worktree preparation (issue 1469) and on the environment variables entry above.

Undecided: the agent's permission level in the main checkout, since agents never edit the base branch today.
Undecided: whether the saved configuration lives in the repository's `.lgtm` folder, in engine state, or both.
Undecided: how the agent appears in the navigation, and whether it stays running or starts on demand.

## A better Define phase

State: In design by the owner from 2026-11-20, with a separate design agent.

The owner finds the present Define phase poor and wants a much better user experience for it.
When a project starts with a goal, the Define agent must read the repository, run scripts where needed, ask questions, and then produce the end-result document.
The entry "A Define workspace with a conversation and an evolving end-result document" and issue 1464 record earlier observations.
The design is worked out in the mock first; nothing here is decided yet.

### What the owner said on 2026-11-20

The owner wants a Project view that works as the command centre of one project.
Everything the project needs is there, and everything the owner discusses with the project lead is there.
The owner opens a team or an agent directly only when something has gone wrong, for example when a problem returns after three reviews.
The owner connected this to moving the work board to the repository level, which the entry "A repository board with project prefixes on its tickets" records.

The project lead in the right-hand panel felt too muted.
The owner's attention went to the centre, where the board was.
The Chat tab was easier to read than the panel.
A long message inside a card was hard to follow, so the typography of long text needs work.

Questions, answers and follow-up messages must appear in order.
A question card must not carry the whole proposed end result.
The owner wants the design to work from the transcript of the project "We're trying issue 940".

### The first draft in the mock

The draft is a new Project tab in `design/mock/`, built in `js/screens/mission.js`, `js/define.js` and `css/mission.css`.
The owner has not approved it.
The sample project "We're trying issue 940" opens on it, at `#/p/naming/project`.
Its conversation is the recorded Define conversation of that project, with the two proposal cards recut.

The conversation with the project lead fills the centre, and the end result sits beside it on the right.
A phase line above both shows where the project is in the build loop.
The lead's reading and commands fold into one line that opens to the list.
The proposed end result lives in the document, as numbered statements, and no longer in a card.
A statement that the lead rewrote is marked Changed and keeps its previous wording.
A question holds one decision, names the statement it decides, and offers its answers as rows.
Choosing a row only marks it, and "Send answer" sends it, so a wrong click costs nothing.
The owner can choose "Something else" and write the answer.
An answered question stays where it was asked, with the owner's answer under it.
A typed message can speak about one statement, and the message box says so.
Agreeing to the end result is one action, and it is refused while a question is open.
Agreeing ends Define: the project moves to Design when screens change, and to Plan when they do not.
After agreement, a note on a statement marks it "Change asked" and the project stays in its stage.
The top line shows the project's stages: Define, Design, Plan, Build, Review and Hand over.
A project past Define shows its statements with their build state, its open requests and its teams in the same place.
Reading text in this view is 14 pixels on a measure of about 66 characters.

### What the owner decided on 2026-11-20

The owner approved the direction: the end result is a document that the conversation works on, and it is not posed as a question.
The owner's agreement to the end result is what promotes the project out of Define.
The order is agree first, then design, so that the lead knows what it orchestrates before any design work starts.
After agreement, the work stays within the bounds of the end-result document.
A certain number of changes within those bounds are allowed, because some are unavoidable.
A heavier change of scope would redefine the project; the owner left its design for later.
An answer must not be sent by one click, because the owner cannot change a sent answer; the owner asked for a submit step.
The Project view is the view every project opens on, in place of the board.
A team's question and an approval use the new question form in the Project view; the owner asked for this on 2026-11-20.
The owner then rejected three things in that form, on the same day.
Three open blocks stacked in one conversation were annoying and used too much space, so only one decision is open at a time.
A consequence written inside an answer row could grow to a paragraph, so a row now holds only a short label.
The line "Choose an answer. Nothing is sent until you send it." stated the obvious, so the view carries no instructions.
The owner then asked to try the docked form found on Mobbin, where the waiting decisions sit above the message box.
The owner also asked that each decision opens and closes on its own, that opening one does not close another, and that an answered one keeps the same behaviour.
The mock now does both.
The mock now does this for every sample project, and `design/DESIGN.md` records the Project view.

### What is undecided

Whether the new question form replaces the earlier cards in the Chat tab, a team's thread, the side panel and Home is undecided.
A merge request and a failed start still use their earlier cards in the Project view.
An approval cannot yet carry a reason when the owner denies it.
How many changes after agreement are allowed, and who judges that a change is too heavy, is undecided.
What redefining a project looks like is undecided.
Whether a sent answer can still be changed before the lead acts on it is undecided.
Whether the Board and Chat tabs leave the project when the board moves to the repository is undecided.
Whether the owner can edit a statement directly, and what happens to conflicting edits, is undecided.
The open questions of a Define record do not yet count in the sidebar, the bell or Home.
A newly created project shows an empty end result, because the sample lead is scripted only for the issue 940 project.
The live website does not show the Project tab; wiring it to the engine is later work for builders.
The engine has no record for a draft end result, its statements or its questions, and the client application programming interface (API) needs one.
The lead's opening instruction in `packages/engine/src/project/lifecycle.ts` still tells the lead to put the full proposal in a card.

## Bringing a repository's existing mocks into LGTM Studio, and support for other languages and tools

State: Idea. Stated by the owner on 2026-11-20, after registering a second repository.

### The problem the owner saw

The second repository is registered, but LGTM Studio shows no mocks for it.
That repository already has its own mock application, a separate package that runs the product's real components on sample data.
It also already has a design file in the Figma format.
LGTM Studio has no way to take either of them in.

### The process the owner described

1. Produce the OpenPencil design file, in the Figma format, holding all design components and mock screens.
2. Convert that file into the mock application.
3. Build the mock application.

The owner prefers converting an existing mock project over merely importing it.
This follows the earlier entries "The design file is the source of truth" and "Mocks made by the Design team".

### The open question: languages other than React

The owner expects a problem when a repository does not use React.
The conversion from a design file to a mock application, and the build of that application, differ for each language and user-interface toolkit.

### What the owner hopes for

The first agent that prepares a repository checks four things.
See the entry "A repository agent that works in the main checkout and prepares a repository for projects".

- Which language and toolkit does the repository use?
- Does it already have a testing framework?
- Does it already have a design framework?
- Does it already have a mocks framework?

Where LGTM Studio lacks support for what the agent finds, the agent opens a pull request against LGTM Studio that adds support for that language or tool.
This matches the earlier entry about per-language test tools arriving by pull request.

### Undecided

- What "support for a language" consists of: most likely a small adapter that states how to install, test, build and serve the mock application, and how design components map to code.
- Whether an existing mock application is used as it is, with LGTM Studio only told how to build and serve it, before any conversion from the design file is attempted.
- Who reviews a pull request that an agent opens against LGTM Studio from another repository's work, and where it is opened when the installation is not a checkout of LGTM Studio.
- How a mock application that needs a device or a simulator is shown in the Mocks tab.

## An extensible mockups module with one framework per language and tool

State: Idea. Decided in direction by the owner on 2026-11-20. This corrects the entry "Bringing a repository's existing mocks into LGTM Studio, and support for other languages and tools".

### The decision

The owner does not want LGTM Studio to use a repository's own mock application as it is.
That application was built for one product.
Instead, LGTM Studio gets its own mockups module, and it is extensible.
The second repository's existing mocks are then converted to it.
The earlier recommendation, to point LGTM Studio at an existing mock application first, is withdrawn.

### What the module does

The module takes what a repository is building and renders it so that it runs in LGTM Studio's website.
Each mockup is rendered in the repository's own language and toolkit, and the result shown is HTML.
A phone project is shown inside a phone image, at a stated screen size and pixel ratio.
A website or another kind of project gets its own presentation, which is not designed yet.
The owner does not yet know how this works for native applications.

The owner recalled a tool that loaded Angular, Vue, Svelte and React from a content delivery network at run time and rendered components from each.
That is the model for how one page can render several toolkits.

### What you see is what you get

The Mocks tab must show exactly what users will get.
The condition is the same screen size, pixel ratio and similar settings.
This is the main requirement, and it is why the mockups render the real code in its own toolkit and are not redrawn.

### One framework per language and tool

A framework guide says how to add a language or a tool.
Every language and every tool then gets its own framework.
For mockups, a framework says: here is a new language, and here is how to render it. The language then shows up in the Mocks tab.

The same idea applies to testing.
Each test tool, such as Vitest or the TypeScript compiler written in Go, gets its own framework for how it runs tests and scripts.
Those frameworks carry the lessons from testing the owner's second repository, and improve over time.
This joins the earlier entries about a testing skill with fixed rules and per-language test tools.

### Undecided

- What a framework is in code: its files, its interface, and where it lives (inside LGTM Studio, in `~/.lgtm`, or in a repository's `.lgtm`).
- How the module builds a repository's components for the browser: bundling on the server, loading libraries at run time, or both.
- How sample data, fonts, images and the repository's own styling reach a mockup.
- Native applications: a web build where the toolkit has one (such as React Native for Web), a simulator stream, or screenshots; each weakens the "what you see is what you get" promise in a different way.
- How a mockup is isolated from LGTM Studio's own page, since it runs a repository's code inside the website.
- How the existing conversion from the design file fits: the design file stays the source of truth, and mockups come from real code.

## Linking a hosting account to add a repository directly from GitHub, GitLab or Bitbucket

State: Idea. Requested by the owner on 2026-11-20.

The owner wants to link an account at a repository hosting service to LGTM Studio.
With a linked account, the owner adds a repository directly from that service, without first cloning it by hand.
The owner named the largest services: GitHub, GitLab and Bitbucket.

Today a repository is added only by the path of a folder that already exists on the machine.
The pull request flow uses the `gh` command with whatever account it is signed in to, and supports GitHub only.

What this would add:

- A linked account per hosting service, shown in the settings beside the harness accounts.
- A list of the repositories that the account can reach, to choose from when adding a repository.
- A clone into the workspace folder, followed by the same registration as today, including the recorded base branch.
- The same linked account then serves pull request delivery for that repository.

Undecided: how each account is linked, for example a sign-in in the browser, a stored token, or the service's own command-line tool.
Undecided: where the credential is kept; the entry "A built-in vault in the server process" is the likely place.
Undecided: pull request delivery for GitLab and Bitbucket, which the present GitHub flow does not cover.
Undecided: self-hosted instances of these services.
`SECURITY.md` governs this, because a hosting credential can push code; accounts and secrets stay with the owner.

## A server, a client and a website as three separate pieces

State: Direction stated by the owner on 2026-11-20. Not designed.

The owner wants LGTM Studio split into a server, a client process on each working machine, and a website that can also be hosted by itself.
Several server nodes behind a load balancer, a replicated database, and one clone per agent on any machine are part of the idea.
The full record, with the open questions, is in [the direction document](specs/2026-11-20-server-client-website-direction.md).

## Session reports kept by the server, and an agent that learns from them and cleans them up

State: Idea. Stated by the owner on 2026-11-20. Narrowed by the owner on 2026-11-25.

### The decision of 2026-11-25

Session reports go to the data store of LGTM Studio, not to the repository.
This settles the choice between the two ways named under "The idea".
The reason is privacy as well as litter.
A report records what a person said and the names and paths of that person's machine.
A repository can be public, so a report in it publishes those details.
The owner also decided on 2026-11-25 against a commit hook that scans for such details.
A pattern check cannot find every private detail, so the reports must not reach the repository at all.

### The problem

Session reports are stored in the repository today.
The owner finds that they litter it.
Whether to keep session reports at all is each user's choice; LGTM Studio must not require them.

### The idea

Session reports go to a central place.
The owner named two ways to think about it: skills that manage them, or a central area in the data store that the server holds.
The owner's direction for a server with its own data store is in [the direction document](specs/2026-11-20-server-client-website-direction.md).

A separate agent reads the stored reports.
It is launched only after the user approves.
It looks for what can be improved in the software, what went well and what went badly.
Cleaning the reports up afterwards is part of the evaluation phase.

Where a finding is an improvement or a fault of LGTM Studio itself, the agent opens an issue on GitHub.
The data in that issue is sanitised first.
The agent cleans up the session reports as it goes.

### Related entries

The model evaluation records read the same reports to grade models.
The entry "A repository agent that works in the main checkout and prepares a repository for projects" could set this choice up for a repository.

### Undecided

- Whether reports leave the repository entirely, or a short summary stays in it.
- What "sanitised" requires before anything is sent to a public issue tracker: no names, no paths, no repository content, no secrets. A report from a private repository must never leak into a public issue.
- Whether the user sees and approves each issue before it is opened, in addition to approving the agent's launch.
- Which repository receives the issues, and under whose account.
- How long reports are kept, and what "cleaned up" means: deleted, or condensed into findings.
- How this relates to the existing lists `docs/issues.md` and `docs/features.md` in this repository.

## A usage page for every connected harness and account

State: Idea. The owner will design it; stated on 2026-11-20.

The owner wants a usage monitor in LGTM Studio.
It covers every connected harness and every account.
Where it sits in the website is not decided.

The model is the status line the owner already has in the terminal.
For each account the page shows the model, the weekly limit and how much of it is left, and when it resets.
For each running agent it shows how full the context is.
It shows the cost in dollars of the tokens used.
The page breaks these numbers down, for example by harness, account, project and agent.

What exists today: the engine reports context use for an agent, and an account can be marked available again after a limit.
The engine does not read any provider's usage or limit report.

Undecided: where each number comes from, since every harness reports limits and cost differently, and some report none.
Undecided: whether cost is taken from the harness's own figure or computed from token counts and a price table that must be kept current.
Undecided: whether the page only shows usage, or also warns before a limit and moves work to another account.
Reading a provider's usage may need the account's own credentials; accounts and secrets stay with the owner under `SECURITY.md`.

## How an agent receives its purpose, and which settings, hooks and memory it gets

State: Idea. The owner will design it; agreed on 2026-11-20.

### What happens today

LGTM Studio composes an agent's purpose from the built-in role text, the owner's overrides and the per-repository note.
It sends that text inside the first message of the agent's session.
The owner's first message is therefore shown with the whole role text in front of it; issue 1542 records this.

An agent in a project also runs with the owner's ordinary harness settings.
It loads the repository's `CLAUDE.md` or `AGENTS.md`, the owner's memory notes and the owner's session-start hooks.
Issue 946 records this and waits for the owner's decision.

### What the owner wants decided

- How the purpose reaches the agent without appearing in the chat as the owner's message. The owner asked whether a session-start hook can inject it.
- Which settings, hooks and memory an agent in a project gets, and which of the owner's own it must not get.

### Notes for the design

- A session-start hook exists in Claude Code and can inject text. What other harnesses offer in the mode LGTM Studio drives them in has not been checked.
- The board, the roster and the thread view must not depend on the harness, so a hook cannot be the only way.
- Other ways to consider: a system prompt passed when the session is created, where the harness allows it; or keeping the first message and hiding the role text in the thread.
- A general rule belongs in the built-in role text; a repository's own facts belong in its `AGENTS.md`, `CLAUDE.md` or `.lgtm` folder. The owner said this on 2026-11-20 about the submodule note set for one repository.
- The entry "Environment variables that LGTM Studio gives to agents" is related: a hook can read such a variable and stay quiet inside a project.

## Watchers that LGTM Studio runs and manages, for stale agents and for waiting on work

State: Idea. Requested by the owner on 2026-11-22.

### The problem

An agent that waits for something starts its own watcher today.
A lead that waits for a builder, a test run or a pull request writes a shell loop and runs it in the background.
These scripts fail silently, differ per agent and per shell, and die with the agent's session.
Nothing shows the owner which watchers exist or what they wait for.

### The idea

LGTM Studio runs and manages watchers itself.
An agent does not launch a watcher; it asks LGTM Studio for one.
The owner wants two things watched:

- **Stale agents.** An agent that has stopped making progress is noticed and reported.
- **Watcher scripts.** A watcher that has itself stopped, hung or lost its target is noticed too.

### What exists today

The stall watch notices an agent that has gone quiet and tells its lead; a usage-limit pause is not counted as a stall.
The board wakes an agent when a message arrives for it.
Neither lets an agent say "wake me when this condition holds".

### What this would add

- A command for an agent to register a watch: on another agent finishing, a branch moving, a check or a pull request changing state, or a time passing.
- The engine evaluates the watch and wakes the agent through the board, with the reason.
- Every watch is a record: who asked, what it waits for, when it was last checked, and when it ends.
- The owner sees the active watches and can cancel one, in the website and through the client API.
- A watch has a time limit, and a watch whose target is gone ends and says so.
- Watches survive a restart of the service.

### What the owner settled on 2026-11-23

A timer is certain to be included.
Pull request monitoring is certain to be included.
Another agent finishing is included; part of it exists already.
The fixed list of conditions is accepted as the starting point.
The owner's main case is a test run: a run is started, and the agent waits for that run to finish.
The agent asks LGTM Studio to notify it when a named test run is done.
The entry "Executions: builds and tests as workflow files that LGTM Studio runs" makes a build the same kind of thing.

### What agents waited for in a private repository's history

The owner asked for a look at everything the sessions of a private repository have run as monitors and waits.
A count on 2026-11-23 covered 922 Claude Code transcript files of that repository and its worktrees.
305 of them started at least one background command, poll loop, monitor or scheduled wake-up.
There were 3,274 background shell commands, 1,792 poll loops in the foreground, 691 monitors and 116 scheduled wake-ups.
The sorting below is by keyword, so the numbers are rough, and a background command is not always a wait.

- About 1,560 waited on a test run or a test stack.
- About 1,130 waited on a file or a log line.
- About 940 waited on another agent or its terminal pane.
- About 430 were a plain timer or a wait for a process to exit.
- About 250 waited on a pull request or its checks.
- About 160 waited for a local server or port to answer.
- About 12 waited on a build, and about 11 on a branch or commit.
- About 1,350 did not match any keyword.

That repository has no cron job for this work; its scheduled work is seven GitHub Actions workflows and one systemd timer.
The count supports the owner's point: a test run is the commonest thing an agent waits for.
A wait on a file or a log line is nearly always a stand-in for "this run finished" or "this server is up".

### Undecided

- Whether an agent may supply a command of its own to be polled. A command chosen by an agent and run by the engine is a `SECURITY.md` question.
- What a stale agent is, beyond the present stall watch: no output, no tool call, no commit, or a context that stopped growing.
- What happens after a stale agent is noticed: a message to its lead, a nudge to the agent, a restart, or only a notice to the owner.
- How watches fit the direction for a server and clients on several machines.

## Executions: builds and tests as workflow files that LGTM Studio runs

State: Idea. Stated by the owner on 2026-11-23. The owner said the details wait until this is built.

Builds and tests follow the example of GitHub Actions.
An agent writes the build script in a YAML file.
It writes the test scripts in further YAML files, like GitHub Actions workflows.
The owner suggests renaming the "Testing" tab to "Executions" or a similar name.

Agents run these workflows themselves.
The owner can choose to run one on a different machine, with the changes synced to it.
That is for the model with several machines in [the direction document](specs/2026-11-20-server-client-website-direction.md).
The owner's reason is a lower security risk.

Every run has an identity.
An agent asks LGTM Studio to notify it when a named run is done; the watchers entry above records this.

LGTM Studio could support GitHub Actions itself, so that an existing workflow file drops in and works.
That would require support for GitHub Actions plugins as well.

Decided by the owner on 2026-11-23: a workflow is a GitHub Actions workflow file, used one to one.
The same workflow files give build workflows and test workflows.
The owner wants them as the managed way to test the software and to make builds.
The name of the screen is "Actions", not "Executions", as the owner decided on 2026-11-23.
The word "workflow" stays with the build loop and its replacements.
Undecided: which parts of GitHub Actions a first version runs, such as actions from the marketplace, runners and secrets.
Undecided: what "synced" means for uncommitted changes, and how secrets reach a run.
Undecided: how this joins the testing work that is being built now and the mockups module's per-language frameworks.
A rename of an existing tab is a change to an existing screen and needs the owner's approval when it is built.

## Automatic compaction, clearing and handover of long-running agent sessions

State: Idea. Stated by the owner on 2026-11-23.

### The problem

A project can stay open for months, so an agent's session can run for months.
The owner says that is not right.
LGTM Studio has no rule today for when a conversation is compacted or cleared; each harness compacts by its own rule when its context fills.

### The owner's thoughts

A project that has not been closed in a month may need other handling, especially for its project lead.
One option is a handover: a new agent comes up and summarises the previous one.
A compaction may be the same thing, because the task is the same and not new, so a clear is not needed.
Switching a lead from one harness to another, for example from Claude to Codex, needs a handoff file; the model cannot simply be changed.

### What this would add

- A rule, set by the owner, for when a session is compacted: by context size, by age, or at a boundary such as a closed unit.
- A handover that LGTM Studio runs: the leaving agent writes a handoff file, and the arriving agent starts from it.
- The same handover for a change of harness or model.
- A record of each compaction and handover, visible to the owner.

### Undecided

- When to compact, when to hand over to a fresh session, and when to clear.
- What the handoff file must hold, and whether the engine writes part of it from its own records, such as the board, the units and the open requests.
- Whether a handover needs the owner's approval, or only a notice.
- What a project open for a month should trigger.
- How this relates to the entry "Session reports kept by the server, and an agent that learns from them and cleans them up".

## A Usage page for every harness account

The owner asked for this on 2026-11-20.

### What the owner said on 2026-11-20

The page shows, for every harness and every account, how much it has used and what its limits are.
Different harnesses report limits differently, so the page must not assume one shape.
The American harnesses usually report a five-hour limit and a weekly limit.
The page also shows tokens used, and what those tokens would have cost in dollars.
It breaks the use down by project, by team within the project, and by agent within the team.
That breakdown should support a report such as "In this build, planning took 30% of your tokens, and 25% of that 30% came from Planner 1 in Agent console."
An agent could read the use, name the cause of a large share, and propose a guardrail that prevents it next time.

### The first draft in the mock

The draft is the Usage page in `design/mock/`, in `js/screens/usage.js`, and it awaits the owner's approval.
It has a period switch for today, this week, and 30 days.
The Accounts table groups accounts by harness, and each account lists the limits that it reports, with the reset time of each.
Each account shows its tokens, its share of all tokens, and the cost at API prices.
"Breakdown" is one table that opens level by level, grouped by project, by stage, or by account.
Each row shows its share of the row above it, which is how "25% of that 30%" is read.
"Findings" lists what a usage review found: the cause, a proposed guardrail, and the tokens involved.
A finding has "Add guardrail" and "Dismiss".
The sidebar gains a Usage row under Harnesses, in the mock only.

### What the owner said about the draft on 2026-11-20

The owner accepted the page as a good start that can be made better.
The harness rows in the Accounts table needed a clearer separation, because they had the colour of the column titles above them.
The owner asked for the machine name to be removed from each account.
Every machine signs in to the same accounts through the server, so every machine has the same accounts to choose from.
The owner found that the Findings had the right details and a poor layout.

The draft changed in answer.
Each harness now opens its own darker band in the Accounts table.
An account row shows its name and its account address, and no machine.
A finding is now one row with its tokens, its share of the week, and its cost.
A row opens and closes on its own, and the first waiting finding starts open.
An open finding shows the cause on the left and the proposed guardrail in a box on the right, with "Add guardrail" and "Dismiss" inside the box.
The box also names where the guardrail would go, for example "Build loop · Plan review".
"Show where it went" opens the breakdown at the project and stage the finding is about.
A row in the breakdown carries a "1 finding" marker when a waiting finding is about it, and the marker opens that finding.
A finding that was added or dismissed moves under a folded "Handled" row, and "Reopen" puts it back.

### What is undecided

Where the dollar prices come from is undecided; the mock's prices are invented.
Which limits each harness can report is undecided, and so is the form for a harness that reports neither a five-hour nor a weekly limit.
Which agent writes the findings, and when it runs, is undecided.
Where a guardrail is stored is not built or settled in detail; the section "Where a guardrail goes" below holds the owner's direction.
Whether a guardrail needs the owner's approval before it takes effect is undecided.
The engine's stored turn usage holds input and cache token counts; it holds no price and no stage, and the record read on 2026-11-20 had no output token count.
The engine and the client API have no usage read for the website yet, so the live website does not show this page.

### Where a guardrail goes

The owner said on 2026-11-20 that the agent that writes a finding could decide whether its guardrail is generic or specific to one repository.
The draft follows that: each guardrail arrives with a scope the agent chose, either the repository's name or "Every repository".
The owner can switch the scope in the guardrail box before adding it.
A guardrail for one repository would go into that repository's `.lgtm` folder.
A guardrail for every repository would go into `~/.lgtm`.
The designer's recommendation is that a guardrail never goes into `CLAUDE.md` or `AGENTS.md`.
It should attach to one step or one role of the workflow, so that only the agent doing that step reads it.
Where the engine can enforce a guardrail itself, such as a limit on review rounds, it should be a workflow rule and cost no context at all.
This recommendation awaits the owner's decision.

## Short instruction files for agents

The owner asked for this on 2026-11-20.

The owner wants the `CLAUDE.md` files of projects run on LGTM Studio to be very short: simple instructions and the gotchas, and nothing else.
The reason is that every agent loads the file at the start of every session, so a long file spends context in every agent.
The owner named the file of a private repository as the example to avoid.
On 2026-11-20 that file was 32,938 bytes, and this repository's `AGENTS.md` was 7,221 bytes.
Guardrails added from the Usage page must not grow these files; the section "Where a guardrail goes" above says where they go instead.
A size figure for instruction files on the Usage page, and a finding when one grows, are possible follow-ups and are not designed.

## Ideas from the screen reference research of 2026-11-20

The owner asked on 2026-11-20 for a scan of every screen against similar products, and said that an idea that seems logical should be added.
The research is in `docs/research/2026-11-20-screen-references/`, one file for each screen.
Each file has its own longer list under "Ideas that are not in the feature list".
The designer chose the ideas below as the ones that fit how the owner works.
State: Suggested. The owner has decided none of them.

### Knowing what changed without opening it

- A dot on a sidebar row, a thread and a roster row that has output the owner has not seen. Devin has it. See `sidebar.md`.
- An outcome on a project or thread row, such as "PR is ready" or "+853". Devin and Cursor have it. See `chat-thread.md` and `agents-roster.md`.
- One closing line for a finished turn, such as "Worked for 37s", with a box of the files it changed. Cursor has it. See `chat-thread.md`.
- A duration on every finished phase and step of the workflow. Vercel has it. See `workflow-tab.md`.
- The failing line of a failed run lifted to the top of the run. Vercel and GitHub Actions have it. See `review-and-test-results.md`.
- A state for a flaky test, apart from passed and failed. No reference has it. See `review-and-test-results.md`.

### Answering and undoing

- An "Undo" notice after an action removes a row or changes a state. Sentry has it. See `home.md`.
- A "Skip" on a question, which tells the agent to go on without that answer. Claude and Unify have it. See `decisions-and-approvals.md`.
- Keys that move between answers and send one. Claude has them. See `decisions-and-approvals.md`.
- A marker in the conversation at the place a waiting decision was asked. Emergent and Unify have it. See `decisions-and-approvals.md`.
- A "Viewed" checkbox on each file of a change under review, with a count of files viewed. GitHub has it. See `git.md`.

### Starting work

- A prompt box that is always on screen for starting work, with the repository, branch and model as small menus on its edge. Cursor, ChatGPT Codex, Claude and Devin have it. See `new-project.md`.
- Dictation into the message box. ChatGPT and Cursor have it. See `message-box.md`.
- A setup checklist that stays in the product after the first run, with a count such as "4 of 6". Devin has it. See `login-and-first-run.md`.
- A choice of how the sidebar is grouped: by repository, stage or status. Cursor has it. See `sidebar.md`.

### Limits, accounts and notifications

- A second account that takes over when the first reaches its limit. Relevance AI has a fallback model. See `harnesses-and-accounts.md`.
- An alert when an account's limit passes a chosen percentage. The OpenAI Platform has it. See `usage.md`.
- Each limit of an account as its own row, with the time the figure was last read. Claude has it. See `usage.md`.
- A "notify me when this reply ends" button on a running reply. Claude has it. See `chat-thread.md`.
- A pause of notifications that ends at a chosen time, and a notification setting for one project. Slack and GitHub have them. See `notifications.md`.
- Model and effort joined in one menu option. Cursor has it. See `service-and-project-settings.md`.

### Looking back at what an agent did

- A searchable history of the commands an agent ran, with status, duration and start time. Devin has it. See `terminal.md`.
- A comment on a mock that can be resolved, with a filter for resolved comments. Figma has it. See `mocks-tab.md`.

## Project view changes the owner asked for on 2026-11-20

Status: designed in the mock, `design/mock/`. None of it is wired to the engine.

The owner asked for these after the conversation moved to the left of every project view.
`design/DESIGN.md` records each decision under "The Project view".

- A divider between the conversation and the view that drags, with the width remembered.
- A fold button on the edge of the sidebar, so the owner does not travel to the top left corner.
- Pinned projects, shown as tiles in the folded sidebar. The owner pins and unpins a project from its row.
- The agents' message board as a part of Agents, with no Messages view of its own.
- A "Paste in terminal" button on a shell command that the project lead gives.

Not decided yet:

- Whether pinned projects also get their own group in the open sidebar.
- Where the pins are stored. The mock keeps them in the browser. A real pin belongs to the owner, so the engine should keep it.
- Whether a pasted command may open a new terminal session when none is free. The mock does.
- How narrow Git and Files may get beside the conversation. See issue 1555 in `docs/issues.md`.
- A setting that flips the whole project: the conversation on the right, the view's list on the left, and the content in the middle. The owner named this on 2026-11-21. Cursor is laid out that way.
- Whether the lead may move the owner's screen by itself, or only offer a link. The mock offers a link. The engine needs a way for an agent to name a place in the website: a view, an agent, a message, a sprint or a file.
- Whether the two panel buttons stay in the row of views. Two other layouts were named: a close button in each panel's own header with a strip on the edge to reopen it, or one group of layout buttons in the top bar.

### A move on the board is gated, and it starts a team

The owner decided this on 2026-11-21, and the mock shows it. The engine does not have it.

- Each lane has needs. A sprint enters the lane only when they are met. The mock's needs are a first guess: In progress needs the sprints it depends on and an approved plan, Review needs every task done and a passing full run, and Done needs a clean review and the merge.
- The needs belong to the workflow, not to the website. A workflow override in `~/.lgtm` or in a repository's `.lgtm` should be able to change them.
- A move into Planned, In progress or Review starts that lane's team. The project lead is told and does not create the team.
- A forced move skips the needs. It is made from the sprint's page, after a question that lists what is skipped. The lead is told what was forced past.
- Not decided: what a move back does to a team that is working. The mock stops a team that was started for that sprint alone.
- Not decided: whether a forced move needs anything more than one question, such as a typed reason.

### Vercel's UI SDK and the lead's control of the website

The designer raised this on 2026-11-21, after the evaluation of the sprint page. Nothing is decided.

- UI SDK is a library of headless parts: menus, dialogs, tabs, selects and drag and drop, with their keyboard, focus and screen-reader behaviour. It is a 0.x beta under the Apache License 2.0.
- The mock's menu and dialog are its own small components. The evaluation found gaps in both that UI SDK's parts do not have. See issue 1561 in `docs/issues.md`.
- UI SDK also has a bridge that lets an agent read a page's controls by name and use them, with a rule for what needs a person. This is one way to build the owner's wish of 2026-11-21 that the project lead can take him to a place in the website.
- The mock has no build step and loads nothing from the network, and UI SDK has no Preact package. Its web components would be the way in. That is a change of foundation for the live website, so it needs the owner's decision and a small trial first.

### What "a customizable board" can mean

The owner asked on 2026-11-21 how the board could be customizable, since its lanes are the workflow.
The designer's answer has two layers.

- **The view is the owner's to change, and it changes nothing for the agents.** This covers the order of cards, grouping by epic or team, which lanes are folded, which fields a card shows, and saved filters.
- **The lanes are the workflow's stages.** Adding, removing or renaming a lane changes what the agents do. That belongs in a workflow override in `~/.lgtm` or in the repository's `.lgtm`, and the board then draws whatever stages the workflow names.

### What else the Questions view could hold

The Questions view of 2026-11-22 lists the questions that agents ask in a project. It is a prototype in the mock, and issue 1567 in `docs/issues.md` tracks it.

- **Approvals.** The owner said on 2026-11-22 to think about approvals later. One way is a second group in the same list.
- **The questions of the Define phase.** The lead asks them about the end result. They have answers to pick and no report, so their full view would be shorter.
- **A way to the next question.** After an answer, a button could open the question that has waited longest.

### Moving the website to Preact 11 with a build step

The owner raised a move to another framework on 2026-11-22, because editing a screen in the mock had become slow. He decided the same day: the website stays on Preact, on version 11, with Vite as the bundler and TypeScript as the language. The port is builder work with a plan, and it is not started.

The mock is written in Preact with htm, and it has no build step. A screen is a template string in a plain JavaScript file, with no types. The package `packages/studio` makes the live website by copying the mock and replacing pieces of its text at build time, with 17 replacement files. The styles are global, in a few large files.

- **The build step is what helps.** Vite, TypeScript and JSX give types, components in their own files, and errors at build time. Vite 8.3.4 is already pinned, and `@preact/preset-vite` 2.10.6 accepts Vite 8.
- **The mock and the website could share one source.** Each screen would take its data from a provider, sample data in the mock and the engine in the website. That would remove the replacement of text at build time.
- **The cost is a port.** The mock is about 12,700 lines of JavaScript and styles, and the 795 tests of `packages/studio` read the mock's modules. "Where the website lives" in `AGENTS.md` would change.
- **The rules for writing Preact belong in the lint settings and the types.** Imports come from `preact` and `preact/hooks`. Typing in a field is `onInput`. A style value carries its unit, because Preact 11 no longer adds `px`. `defaultProps` is not used.

These were found on 2026-11-22 and are not decided:

- **The compiler stays the Rust port.** The owner named tsgo. That was the preview of Microsoft's compiler in Go, and it became the `tsc` of TypeScript 7.0 on 2026-07-08. This repository pins `@tsc-rs/typescript` 0.8.0, a Rust port of that compiler, because T3 Code builds with it. The owner decided on 2026-11-22 to keep the Rust port, so one compiler checks the whole repository.
- **The tests of a component.** Vitest can run tests in a real browser, with Playwright driving it. The package `vitest-browser-preact` 2.0.0 renders one Preact 11 component there. The assertion `toMatchScreenshot` compares the picture with a stored master and writes a picture of the difference.
- **The tests of a whole journey.** Playwright is already here as `pnpm test:browser`, and it can compare a page with a master through `toHaveScreenshot`.
- **Masters before the port.** Pictures of today's mock on sample data can be the masters. A ported screen is done when its picture matches.

The port of the mock was built on 2026-11-22, in `design/mock-vite/`. It runs with `pnpm --filter @lgtm/mock-vite dev` on port 4179. It reads the styles, the fonts and CodeMirror from `design/mock/`, so the two cannot drift in look. Twelve routes were compared with the mock at 1440 and at 1000 pixels wide, and every element sits in the same place. It has no tests. Three steps remain. The build of the live website must be moved from `design/mock/` to the port. The old mock must then be removed, and `AGENTS.md` must name the new folder. The master pictures for the component tests can be taken from either mock until then.

## Ideas from the owner's review of the website on 2026-11-23

The owner read every screen of the mock on 2026-11-23. The faults are issue 1572. These are the ideas.

### Test workflows written like GitHub Actions

The owner raised this while reading the Test runs tab. The idea is recorded in full under "Executions: builds and tests as workflow files that LGTM Studio runs".
The owner added on 2026-11-23 that the workflow should be reused elsewhere and replace the tests.
The owner then chose the name "Actions" for it instead.
In the port the Test runs tab of the Status view became Actions that day, with a Build action beside the test ones.
The owner then made it a view of its own in a project's row of views.
Whether the files are also managed at the repository is not decided.

- **The Test runs tab would show these runs.** The owner asked for no change to that tab until executions are thought through.

### A conversation with the repository

A repository has no conversation of its own today. Only a project has one, with its lead.

- **A repository chatbot.** The owner wants to talk to an agent about the whole repository, outside any project.

### The repository as the place for project management

The owner said the repository level is meant for project management.

- **The board moves up.** A repository shows the board of all its projects.
- **A project shows a filtered board.** The project's view is the same board with only its own sprints.
- **A roadmap.** The repository also shows a roadmap.

### The Design view inside a project

- **A project may change the design.** The owner asked whether the Design view also belongs in a project. It may clash with the Mocks view, which already shows a project's screens.

### A short and a detailed End result

- **Two forms of one document.** The owner asked whether the End result should have a concise view and a detailed view. The port now shows the concise one: each statement on one line.

## Environment groups that a project lead assigns to build teams

State: Idea. Stated by the owner on 2026-11-23.

A repository's environment values are grouped.
When a project lead assigns work to a build team, it also assigns the groups that team needs.
A team receives only its assigned groups.
A repository-level agent has all of the repository's environment values.

This answers which environment files an agent receives: none by default, and a named group by assignment.
Until it is built, the per-repository `copyPaths` setting is the only way, and it gives a copied file to every agent of the repository.

Related entries: "A built-in vault in the server process" would hold the values, and "Environment variables that LGTM Studio gives to agents" would deliver them.
"A repository agent that works in the main checkout and prepares a repository for projects" is the repository-level agent.

Undecided: who defines the groups, the owner or the repository agent with the owner's approval.
Undecided: whether a lead may assign any group by itself, or some groups need the owner's approval, for example production credentials.
Undecided: whether a group is delivered as files in the worktree, as variables in the agent's shell, or only to a run under "Executions".
Undecided: how a group is taken back when a team closes.
`SECURITY.md` governs this; accounts and secrets stay with the owner.

## A map of the codebase, so that an owner who does not read the code can follow changes

State: Partly built. The owner chose Floorplan on 2026-11-23. Unit s33-codemap adds the engine, client application programming interface (API) and command-line surface.

### What the owner wants

A way to see how a repository fits together.
When changes are made, the map shows what is where and what was touched.
A developer who is not reading the code stays in the loop.

### What Floorplan is

[Floorplan](https://github.com/haydenschmitty/floorplan) is a plugin for Claude Code and Codex under the MIT licence.
This description comes from its README at version 0.11.0, read on 2026-11-23; it was not installed or run.

- Scripts scan a repository: files, imports, routes and web requests. They are plain Node.js with no dependencies.
- The agent names the parts and describes every file in plain English.
- The result is a `floorplan/` folder with one HTML map, an `architecture.json` file and descriptions. The folder ignores itself in Git.
- The map zooms from the whole application to a part, a file and its endpoints.
- "Recent work" marks what the agent just changed, with a note on why.
- "Health check" lists common faults, such as a key in the code or an endpoint without sign-in.
- An update reads only what changed.
- It can be run without an agent session, from scripts or continuous integration.
- Its README says it has no hooks, no Model Context Protocol servers and no downloads, and that its small server listens only on the local machine.

### What unit s33-codemap built

The owner can read map status with `project.map`.
The reply includes file size, modification time and bounded optional graph facts.
The owner can open a ten-minute address with `project.map.open`.
The owner can revoke it early with `project.map.release`.
The commands accept a registered repository, project or workspace.
A project alone selects its own worktree.
The map remains uncommitted and belongs to that checkout.

The existing isolated preview listener serves only `floorplan/index.html`.
It refuses symbolic links, nonregular files and pages over 16 mebibytes.
It checks the opened descriptor against the checkout on Linux.
Four map frames have their own limit, separate from eight mock and editor frames.
Prepared responses are cached per token with current file and secret checks.
The response sandbox separates agent-written content from the owner's session.
The Content Security Policy blocks fetch-type background requests, peer connections and form submissions.
The page cannot read the owner's session.
An opened page can still navigate itself or open a window to another site, carrying its content and its own address.
The engine strips automatic meta refresh and speculative fetch links from the served markup.
The popup permission keeps the credit link functional.
LGTM Studio injects fonts and theme-specific accents while preserving the map's theme switch.
It adds or keeps a Floorplan credit link, which the page could hide.
A map cannot be opened on the default loopback-only setup.
Opening requires a separate HTTPS display hostname and its reverse proxy.
The engine runs no Floorplan scripts and starts no "Ask" server.
No Floorplan code is copied into this repository.

### What remains

The Map view is drawn in the website, `src/frontend/`, since 2026-11-23, with its "Build map" and "Update map" buttons.
It reads the map through the new contract, `src/contracts/proto/lgtm/v1/map.proto`, from the mock server.
The live website and the engine's `project.map` commands are not joined to it. That is issue 1614.
Joining "Recent work" to project units remains unbuilt.

### How it could fit LGTM Studio

- A "Map" view per repository or per project, showing the map for that worktree.
- "Recent work" joined to a project's units and its review, so the owner sees which parts a unit touched.
- The repository agent builds the first map during onboarding; a build team's lead updates it when a unit closes.
- The blueprint mode, a map drawn from a specification, relates to the Define phase.

### Undecided

- Where the map is stored. Floorplan keeps it uncommitted in the checkout. A project has many worktrees, and the direction for a server keeps project data in the server.
- Who pays for the map: the first map of a large repository describes every file and uses the account's allowance.
- Whether its screen list fits repositories that are not web applications.

## The terminal as a panel, opened from the sidebar

State: Drawn in the port on 2026-11-23, by the owner's description.

The owner asked for the terminal to leave the row of views.
It opens under the view, as it does in VS Code, from a seventh item in the sidebar and with Control and the backtick key.
It opens in the project that is open.
What happens when the owner switches projects with the terminal open is not decided. That is issue 1594, item 2.

## A command center for a project and for a repository

State: drawn in `src/frontend/` on sample data. The owner chose the grid for both on 2026-11-24, out of three layouts of each. Nothing is wired.

The owner decided on 2026-11-23 that a command center is the landing page, because the board is not the right landing page.
A command center is made of widgets.
A widget is like one of the views, except Files and Terminal, and shows a status, such as what agents are doing and the throughput.
On a repository, the widgets are closer to analytics.
A command center can later become a custom dashboard that a person builds.

- **A project's command center.** It is the first view of a project that is past Define. Its widgets are what waits for the owner, who works now, the progress through the phases, the sprints under way, the tasks finished, the checks, the usage and the latest commits.
- **A repository's command center.** It is the first view of a repository. Its widgets are the projects, the plan, what waits for the owner in any project, the sprints merged, the full runs, the usage by project, the base branch and the checkouts.
- **A grid of widgets.** Three layouts of each were drawn, and the owner chose the grid. The two that lost, a list of what waits and a wide panel of the current state, are described in [docs/research/2026-11-23-command-center-references.md](research/2026-11-23-command-center-references.md).
- **Custom dashboards, a first step.** The "Widgets" menu shows and hides widgets. "Arrange the widgets" moves a widget earlier or later and makes it one or two columns wide. The browser remembers it. Dragging a widget, more than one dashboard, and keeping the arrangement on the server are not built.
- **Add a repository by its link, or from GitHub.** Decided by the owner on 2026-11-24. The server tracks a repository by its link and clones nothing. "Set up" clones it on one client machine and starts its repository agent. A connected GitHub account lists its repositories. Drawn on sample data in `src/frontend/`. The engine, the contract and the GitHub connection are not built. Other hosts than GitHub are an idea for later.
- **A terminal opens where the agent of the open place works.** Decided by the owner on 2026-11-24. A project's terminal opens in its worktree, a repository's in its main checkout, and any other page asks for the place. The server only passes the session on, so a machine that runs only the server gets no terminal. Drawn on sample data.

## A repository agent, and the repository's main checkout

State: drawn in `src/frontend/` on sample data. The engine has no repository agent.

The owner decided on 2026-11-23 to add a repository agent, so that a repository is equal to a project.
The repository has its main checkout, and the Browser page and the other views work there.
The owner works with the repository agent on an overall list of what the owner wants.
The repository agent makes projects from that list.
It fills in epics and some tasks on the board, and the projects then fill in more.

- **What is drawn.** A repository opens like a project. The repository agent's conversation is on the left. The views are the command center, Board, Design, Files and Git. The Board holds the sprints of every project of the repository. Design holds the drawn screens of the main checkout, the design file, the Browser page and the architecture. Each view shows the main checkout on the base branch.
- **The plan.** The repository agent keeps a list of what the owner wants from the repository. A line is done, or it has a project, or it has none. A line with no project has a "New project" button, which opens the new project dialog with the line as the goal.
- **The contract.** A preview and a map can now belong to a repository's main checkout. `preview.proto` and `map.proto` have a `repository_id` for that.
- **What is not drawn.** The repository agent filling epics and tasks on a project's board. A board of the repository itself. Settings of a repository.

Questions that must be answered before it is built:

- Whether the repository agent may edit the main checkout. The Git view says today that agents never edit the base branch directly.
- Whether the repository agent makes a project by itself, or asks first. The owner said on 2026-11-14 that a lead creates no team unasked.
- What the repository agent may read of a project's conversation.

## Three parts: a website, a server and a client

State: Started on 2026-11-23, by the owner's decision. The website's side is being built. The server and the client are not started.

The owner wants the product split into three parts, so that each part can change without the others.
The owner will rewrite the server and the client in Go. The website stays in TypeScript.

- **The website** is `src/frontend/`. When no address is set in its environment, it asks for the server's address: a name or an IP address, with a port unless the port is 443 or 80.
- **The server** holds the projects and answers the website. It lets the website make client tokens. Its own command `lgtm client add` makes one too.
- **The client** is a machine that joins a server with a token and runs agents.
- **The contract** between them is `src/contracts/`, in Protocol Buffers. The owner chose Protocol Buffers and gRPC on 2026-11-12.
- **The mock server** is `src/mock-server/`. It holds sample data and answers the contract, so the website is fully usable before the real server exists. A mock server that answers over remote procedure calls lets the owner rewrite the server and the client in Go.
- **The user interface parts** are to be a package of their own, so the owner can make other variants of the website. This is not started.

Built on 2026-11-23: the contract's first two services, the mock server, the screen that asks for the address, signing in, and the Machines page with client tokens.
The codebase map followed the same day, as a third service and the Map view.
What is left is issue 1613.

## The Design view: the drawn design, the running product and the architecture

The owner asked for this on 2026-11-23.
The Mocks view is renamed Design, and it shows a product in three ways.

- **What was drawn.** The screens, their changes, their components and their data, as before. The repository's design file is a page here too, on the same canvas as the repository's own Design page.
- **What runs.** A page named Browser shows the product as it runs. The owner wants to see any product as it runs, not only a web product.
- **How the code is built.** The codebase map is a page here, named Architecture. The owner chose the name Architecture over Map.

Built on 2026-11-23, in the website and the mock server only: the three groups, the contract `preview.proto`, and the Browser page on a sample product.

What the owner named, and where each stands:

- **A web product.** The machine runs the product's own command, such as `pnpm dev`, and the server passes its pages on. The contract covers this. The mock server shows one sample page for every preview.
- **A phone app built with Expo.** Its web build is a web product, so it is a second preview with its own command. The sample has one, named "Phone app".
- **A game built for the web,** for example with Unity. It is a web product too. A game needs WebGL, a pointer and a keyboard inside the frame, and that is not tried.
- **A product with no web build,** such as a desktop game. This needs the screen share below.

What is not decided:

- Whether an agent may also set a preview, or whether the command could be read from the repository. Since 2026-11-23 the owner sets the previews on the Previews page of the project's Settings.
- How a product that needs its own origin is shown. Issue 1618 records it.

## The Browser page: an address, an element picker, and a message to any lead

The owner's ideas of 2026-11-23, after trying the Browser page. Nothing of this is built, except a "Pick an element" button on the Browser page that does nothing yet.

The owner decided the same day that the first picker is a browser extension, and that a desktop client comes later. The research and the decision are in [docs/research/2026-11-23-element-picker-and-browser.md](research/2026-11-23-element-picker-and-browser.md).

- **The Browser page loads an address.** The owner decided not to run the product's development command for now. The page loads a given IP address and port as an embedded web page. This replaces the list of previews with a command. The Previews page of the project's Settings would hold addresses, or go away.
- **An element picker.** The owner wants a tool that selects elements of the page. The selection is added by itself to the project lead's message box, or to any other open message box. The owner types the request after it, such as "I want to modify this row", so that an agent knows which element is meant. The owner named Cursor's browser as the model to look at.
- **The picker also works on the Design file.** The picker works while the owner talks to a build lead with the design file or the Browser page open.
- **A message to a build lead from any view.** Drawn on sample data on 2026-11-24: a menu beside the conversation's button chooses the project lead or a team lead, and a team lead takes a message. The picker needs a way to message a build lead from any view. The chat column beside a view holds only the project lead today.
- **A command center.** The owner suggested ideas for a command center of a project, a command center of the repository, and more on the Design view. This is not specified further.

What must be decided before the picker is built:

- A page in a sandboxed frame from another origin cannot be read by the website. This is the rule that keeps a product's page away from the session, in [SECURITY.md](../SECURITY.md). A picker therefore needs a helper inside the product's page. The server can add the helper when it passes the page on, or the product can load a small script in development.
- The helper sends the website only a description of the chosen element: its selector, its text, its size, and the source file when the product's build records one. The website treats that description as untrusted text.
- What an agent receives: a line of text in the message, a picture of the element, or both.

## A screen share of a machine, passed on by the server

The owner's idea of 2026-11-23, recorded as an idea. Nothing is built.

A client of LGTM Studio would start a local screen share, by Virtual Network Computing (VNC) or another way that is not chosen.
The LGTM Studio server passes the share on.
The owner could then watch the project as it runs without sitting at any of the machines.

- **What it is for.** A product that has no web build, such as a game that runs in its own window. Also a look at what an agent sees on a machine.
- **Where it would show.** As one more kind of preview on the Browser page, so the owner chooses "Website", "Phone app" or "Screen of Workstation" in one place.
- **How it could work.** The client captures one window or one screen and sends it to the server, and the server passes it to the website. VNC and Web Real-Time Communication (WebRTC) are the two usual ways. The owner also named a direct connection between the two machines, which WebRTC allows, with the server only setting it up.

The owner added on 2026-11-23, as an idea for later:

- **One program's window, not only a screen.** A person who builds a Unity game or a Windows program launches it, and the client captures that window and sends it on. The client captures the program's window when it is launched and forwards it to the remote session.
- **The codec follows the machine.** The client relays a stream in AV1, VP9, H.264 or H.265, whichever the machine can encode in hardware. Without hardware encoding, the page warns that the picture will be poor. So the client picks a codec that the machine can encode in hardware, and the page says so when there is none.
- **What it opens.** People who build games or desktop programs could build them in LGTM Studio and watch them run.

Questions that must be answered before it is built, because a screen share shows more than a product does:

- Whether the share is one window, or the whole screen. A whole screen shows passwords, mail and other projects.
- Whether the owner can also type and click, or only watch. Typing into a machine from a website is the same power as a terminal.
- Whether the share is off until the owner turns it on for one machine, and how the machine shows that it is being watched.
- How the stream is kept from anyone without a session. `SECURITY.md` requires a session for every request, and a frame's address with its own key is an exception made for a page with no data.

## Versions of a workflow, kept beside its file

State: Decided by the owner on 2026-11-23. Drawn on sample data in `src/frontend/` on 2026-11-24: the version number, the menu of kept versions, restoring by an ordinary save, and the refusal of a save that edited an older version. The engine keeps no versions yet, and the `.gitignore` file is not written.

This settles issues 728 and 729.

### What the owner decided

A workflow change writes no event and no receipt, so the event log is not filled with them.
The owner said that a person who makes their own workflows keeps their own backups.
The default workflow is not modified; a person clones it and changes the clone. The engine already refuses to save the default.
The owner wants versions: each save is a version, kept with the workflow, so that a person can go back.
The history of a workflow inside a repository's `.lgtm` folder is ignored by Git.
LGTM Studio provides a default `.gitignore` file inside the `.lgtm` folder for that.

### How it could work

- Each workflow file carries a version number, and every save adds one.
- On a save, the version being replaced is copied to a history folder beside the file, such as `my-loop.history/6.json`.
- Whole copies are kept, not differences, because a workflow file is small and a whole copy can be opened and read.
- The engine keeps the last 20 versions of a workflow and removes the oldest.
- Restoring an older version is an ordinary save: version 4 becomes the new version 8.
- A save names the version it edited. If the file has a newer version, the save is refused and the editor says that the workflow changed. This is the revision check of issue 729.
- When the engine first writes into a repository's `.lgtm` folder, it writes `.lgtm/.gitignore` with a line that ignores the history folders, unless that file exists.

### Undecided

- The number of versions kept. Twenty is a proposal.
- Whether the engine adds its lines to an existing `.lgtm/.gitignore`, or leaves a file that a person wrote alone.
- What else the default `.gitignore` of the `.lgtm` folder ignores.
- The history does not say who made a change. That matters only if LGTM Studio gets more than one user.

## A new machine gets the harnesses and the sign-ins the server already knows

The owner described this on 2026-11-24, after the first-run step "Harnesses" was drawn. Nothing here is built.

- A harness that the owner installs is installed on the first client.
- The server keeps a note of every installed harness and of the credentials for each sign-in.
- When the owner installs a client on another machine, that client reads the server's note.
- The new client installs every harness the first client had, or that the server has a note of and credentials for.
- The new client signs in to each of them, so the owner does not sign in again on each machine.

### Undecided

- Where the credentials are kept. The owner decided earlier that secrets live in OpenBao.
- Whether a harness allows one sign-in on several machines. A subscription may limit it, and a sign-in may be tied to one machine.
- What the owner sees while a new machine installs and signs in, and what happens when one harness fails.
- Whether the owner may leave a harness out on one machine.
- `SECURITY.md` has not been read against copying a sign-in from one machine to another.

## A regular pass that reconciles the memories of agents

The owner raised this on 2026-11-25, while the design of the Go rewrite was reviewed.

- The server holds one pool of memory files for all agents.
- When two agents write the same memory, both writes stand.
- A pass, perhaps once a week, goes over the memories.
- It merges memories that say the same thing, settles those that disagree, and removes those that have gone stale.

### Undecided

- Who runs the pass: an agent of a kind of its own, or the server.
- Whether the owner sees and approves what the pass changes.
- How a memory is judged stale.
- The owner named Hermes as a project to look at for this. What it does has not been read.

## A check that the frozen contract did not change by accident

Found on 2026-11-26, while the contract of the Go rewrite was planned.

After sprint 0 the contract is frozen, because two teams build against it.
The contract check could run `buf breaking` against the commit that merged sprint 0, and fail on a change that breaks a reader.
It is not part of sprint 0, because adding `go_package` to the existing files itself counts as a breaking change under the strictest rule set.
