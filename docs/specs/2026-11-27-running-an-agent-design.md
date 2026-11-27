# Running an agent in the Go rewrite: the sprints of wave 2, and sprint 4 in detail

This document is a design.
It cuts the first unit of wave 2, running an agent, into six sprints of slowly rising difficulty.
It designs sprint 4 in enough detail to plan and build it.
Each later sprint gets a short section of its own in this document before it is planned.
Nothing in it is built yet.

It follows the foundation design, `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, and every rule in that document's section "Rules that every later part follows".

## What exists before this design

Waves 0 and 1 are built on the branch `sprint-0`.
A machine joins a server, the owner approves it, and the machine stays connected over one link.
The link carries five kinds of message, and none of them concerns an agent.
The contract for the website covers sign-in, machines, the map and previews.
Agents, conversations, projects and repositories exist only as sample data in the website.

## Decisions of the owner

| Date | Decision |
|---|---|
| 2026-11-25 | Version one drives every harness through the Agent Client Protocol (ACP) only. |
| 2026-11-25 | A message to a running agent steers its turn, whether the owner or another agent sent it. |
| 2026-11-25 | The owner signs in to a harness account once, and the server gives each client what it needs. |
| 2026-11-26 | The Go rewrite carries the five permission levels, with automatic review as the level of a new agent. See `docs/features.md`, "The automatic-review permission mode in the Go rewrite". |
| 2026-11-27 | Wave 2 is cut into several sprints, and each sprint adds a little difficulty to the one before. |
| 2026-11-27 | Sprint 4 connects the chat screen of the website. |
| 2026-11-27 | In sprint 4 an agent starts through the "New project" dialog as it is. The project lead runs in the folder of the repository itself, and the goal is its first message. |

## The sprints

| Sprint | What it adds | What the owner can see at its end |
|---|---|---|
| 4 | One agent on one harness, Claude Code. It starts through "New project", answers the goal, takes further messages while it is idle, and can be cancelled and stopped. | The chat screen shows a real agent that answers. |
| 5 | A conversation in full. A message steers a running turn. The thread shows tool calls, plans and long output, and it loads older items in pages. | The owner interrupts a running agent with a new instruction, and the agent follows it. |
| 6 | Permissions. A request of the agent reaches the owner, and the answer goes back. The level of a running agent can change. | An agent asks before a change, and the owner answers on the website. |
| 7 | Survival. The agent outlives a lost connection, a restart of the client and a restart of the server, or the thread says plainly what was lost. | The owner restarts the client in the middle of a conversation, and the conversation continues. |
| 8 | Every harness. Codex and Cursor beside Claude Code, a change of model and effort while the agent lives, and the sign-in that the server hands to each machine. | The owner starts an agent on each harness and changes its model. |
| 9 | Repositories in full. A clone and a branch for each project, several agents on one machine up to its limit, and the choice of a machine. | Two agents work in separate clones of one repository. |

Teams, tasks and the build loop, memories, the secrets of tools, terminals and the investigation agent come after sprint 9.
Each of them gets a design of its own.

## What sprint 4 delivers

The owner opens "New project", writes a goal, picks a machine, picks or adds a repository, and picks Claude Code.
The server stores the repository, the project and its lead.
The client on that machine starts Claude Code in the folder of the repository and gives it the goal.
The chat screen shows the goal, then the answer as it is written, then the agent as idle.
The owner sends another message, and the agent answers again.
The owner can cancel a running answer, and can stop the agent.
After a reload of the page, the thread is as it was.

Sprint 4 leaves these out, and the table above says where each one comes:

- A message while a turn runs. The server refuses it with one sentence until sprint 5.
- An answer to a permission request. The client refuses each request and writes a note into the thread until sprint 6.
- A conversation that outlives a restart of the client. The thread says that the agent stopped until sprint 7.
- Any harness but Claude Code, and any sign-in but the one that Claude Code already has on the machine.
- A clone or a branch. The lead works in the main checkout.

## The parts of sprint 4

Rule 1 of the foundation design gives each part one folder, one contract file and its own database changes.
Sprint 4 adds two parts to the server and two to the client.

| Part | Folder | Contract file | What it owns |
|---|---|---|---|
| Projects, on the server | `src/server/internal/projects/` | `lgtm/v1/projects.proto` | Repositories, the folder of each repository on each machine, and projects. |
| Agents, on the server | `src/server/internal/agents/` | `lgtm/v1/agents.proto` | Agents, their states, and the items of their threads. |
| Runner, on the client | `src/client/internal/runner/` | none of its own | The harness processes, their ACP sessions, and the translation of their output. |
| Folders, on the client | `src/client/internal/folders/` | none of its own | Looking at a folder and making one. |

The link gains revision 2, in new files beside the frozen ones: `lgtm/link/v1/agents.proto` and `lgtm/link/v1/folders.proto`.
The website gains a data layer for the two new services, in `src/frontend/src/server/`.
No visible screen changes.

```text
Website                      Server                          Client                Harness
  CreateProject  ───────────▶ projects ─▶ agents
                               stores project, agent, goal
                               stores command StartAgent ───▶ runner ── starts ──▶ Claude Code
  WatchThread    ◀─────────── agents ◀── events ◀──────────── runner ◀── ACP ──── output
  SendMessage    ───────────▶ agents: stores item, command Prompt ─▶ runner ─────▶ prompt
```

## The contract for the website

Every `Watch` call uses the shared shape of `common.proto`: everything first, then one change at a time.

### `ProjectService`, in `projects.proto`

| Call | What it does |
|---|---|
| `WatchProjects` | Sends every repository and every project, then each change. |
| `AddRepository` | Takes a machine and a folder. The client looks at the folder. The call succeeds when the folder is a git repository, and it stores the name, the main branch and the remote. |
| `CreateRepository` | Takes a machine and a name. The client makes the folder under the machine's folder for repositories and starts an empty git repository in it. |
| `CreateProject` | Takes a goal, a machine, a repository and the lead's harness, model, effort and permission level. It stores the project and its lead, and it starts the lead. |
| `ListLeadChoices` | Takes a machine and a harness. It returns the models and the efforts that the harness offered on that machine. |

`AddRepository` and `CreateRepository` wait for the client's answer for at most 20 seconds.
A machine that is not connected refuses both, and `CreateProject`, with `unavailable` and one sentence.

A project of sprint 4 has no branch and no clone of its own.
Its folder is the folder of the repository on its machine.
A second project in the same repository on the same machine is refused until sprint 9, because two leads would write the same files.

### `AgentService`, in `agents.proto`

| Call | What it does |
|---|---|
| `WatchAgents` | Sends every agent, then each change. |
| `WatchThread` | Takes an agent. Sends the items of its thread, then each new or changed item. |
| `SendMessage` | Takes an agent and a text. It stores the message as an item of the thread and sends it to the agent. |
| `CancelReply` | Takes an agent. It cancels the running turn. |
| `StopAgent` | Takes an agent. It ends the harness process. |

A message is at most 32 kibibytes of text, because one message of the link is at most 64 kibibytes.
`SendMessage` is refused while a turn runs, while the agent starts, and after the agent stopped.
`WatchThread` of sprint 4 sends the newest 500 items first. Sprint 5 adds pages for older items.

### An agent, as the website receives it

| Field | Meaning |
|---|---|
| Identifier, project, role, name | The role of sprint 4 is always project lead. |
| Harness, model, effort | The model and the effort are what the harness reported for the session, not what was asked. |
| Permission level | One of the five levels. |
| State | `starting`, `working`, `idle`, `stopped` or `failed`. |
| Failure | A reason from a fixed list, and one sentence, when the state is `failed`. |
| Started at, last output at | Times of the server. |

### An item of a thread

| Kind | What it holds |
|---|---|
| `user` | A message of the owner. |
| `agent` | Text that the agent wrote. It grows while the agent writes. |
| `tools` | One tool call: its title and its status. Sprint 5 adds its details. |
| `note` | One sentence of the server or the client, such as a cancelled reply or a refused permission request. |

Each item has a number that rises within its thread, and the time it began.

## The states of an agent

| From | What happens | To |
|---|---|---|
| none | `CreateProject` stores the agent and the command `StartAgent`. | `starting` |
| `starting` | The client reports that the session is open and the first message is accepted. | `working` |
| `starting` | The client reports that the start failed. | `failed` |
| `working` | The client reports that the turn ended or was cancelled. | `idle` |
| `idle` | `SendMessage` stores the item and the command `Prompt`. | `working` |
| `starting`, `working`, `idle` | `StopAgent`, and then the client's report that the process ended. | `stopped` |
| `starting`, `working`, `idle` | The harness process ends by itself, or the client restarts. | `failed` |

The server changes the state in the same database write that stores the command or applies the event.
A machine that goes offline does not change the state of its agents.
The website already shows that the machine is offline.

## Link revision 2

Revision 2 adds messages and changes none.
A server of sprint 4 speaks revision 2 only.
It refuses a client of revision 1 with the existing code `link_revision_mismatch`, and the owner installs the newer client with the installer of sprint 3.
The frozen files of revision 1 keep their content, and the check of the freeze moves its base to revision 2 when the contract of sprint 4 is reviewed.

### Commands that go down

Every command goes through the link's stored commands, as rule 6 demands.

| Command | What it carries | When its result says `done` |
|---|---|---|
| `LookAtFolder` | A path. | The client reported what the folder is, in an event. |
| `MakeRepository` | A name. | The folder exists and holds an empty git repository. |
| `DescribeHarness` | A harness. | The client reported the harness's models, efforts and modes, in an event. |
| `StartAgent` | The agent, the harness, the folder, the permission level, the model, the effort, and the first message. | The harness process runs, its session is open, and it accepted the first message. |
| `Prompt` | The agent, the item's number, and the text. | The harness accepted the message. |
| `CancelTurn` | The agent. | The harness was told to cancel. |
| `StopAgent` | The agent. | The process group of the harness has ended. |

`StartAgent` carries the first message, so that the start and the first turn cannot arrive in the wrong order.
The server sends `Prompt` only to an idle agent, so one agent never has two prompts on the way.
A result of `failed`, `expired`, `interrupted` or `unsupported` for `Prompt` puts the agent back to `idle` and adds a note that the message did not reach the agent.

### Events that come up

| Event | What it carries |
|---|---|
| `FolderSeen` | The command it answers, and either the name, main branch and remote of the repository, or one reason from a fixed list. |
| `HarnessDescribed` | The harness, its adapter's version, and the models, efforts and modes it offers. |
| `AgentStarted` | The agent, the session's identifier, and the model, effort and mode that the session reports. |
| `AgentStartFailed` | The agent, and a reason from a fixed list. |
| `ThreadOutput` | The agent, the item, its kind, the position where the new text begins, and the new text or the new status. |
| `TurnEnded` | The agent, and how the turn ended: finished, cancelled, refused or failed. |
| `AgentEnded` | The agent, whether the owner asked for it, and the exit code. |
| `AgentsRunning` | Every agent that this client process runs. The client sends it once after each start of the process. |

Events are numbered and acknowledged by the link of wave 1, so none is lost and none is applied twice.
The server applies `ThreadOutput` only when its position equals the length of the stored text, which makes a repeated event harmless.
When the server receives `AgentsRunning`, it marks every other agent of that machine that is not `stopped` or `failed` as `failed`, with the reason that the client restarted.

### How much output travels

The runner gathers the text of an item for 300 milliseconds before it sends it.
One event carries at most 16 kibibytes of text.
An item of text is closed at 32 kibibytes, and the text continues in a new item.
When the client's buffer of unacknowledged events is full, the runner stops reading the harness's output until there is room. It drops nothing.

The website receives the whole item on each change, as the shared `Watch` shape demands.
With the limits above, one long answer costs the website at most a few hundred kibibytes.
Sprint 5 looks at this cost again when it adds long output.

## The runner on the client

### The adapter of Claude Code

Claude Code speaks ACP through an adapter program.
The public registry of ACP lists the adapter as one self-contained program for Linux and for macOS.
The client's source names one version of the adapter and the SHA-256 hash of its program for each platform.
The first time a machine needs the adapter, the client downloads it into its own state folder, checks the hash, and refuses a program whose hash differs.
A download that fails is a failed start with the reason "adapter missing".

The plan of the runner checks one fact before any code is written: whether the self-contained adapter works with the Claude Code that is installed and signed in on the machine.
If it does not, the client runs the adapter from its package instead, and the machine then needs Node.

### One process for one agent

The runner starts one adapter process for each agent, in a process group of its own, in the folder of the repository.
The process gets a short list of environment variables: the search path, the home folder, the user, the shell, the language settings, the terminal type, the temporary folder and the time zone.
It gets nothing of the client's own state.

The runner then opens one ACP session, sets the mode, the model and the effort, and sends the first message.
It allows 120 seconds for all of this.
A value that the session does not offer is a failed start with the reason "choice refused".

The agent uses the sign-in that Claude Code already has for that user on that machine.
The server stores no sign-in in sprint 4.

### The five permission levels on Claude Code

| Level | Mode of Claude Code |
|---|---|
| Read only | `plan` |
| Ask before every change | `default` |
| Edit files without asking | `acceptEdits` |
| Automatic review | `auto` |
| Full access | `bypassPermissions` |

A new agent gets the level "Automatic review" unless the owner picked another.
In sprint 4 the runner answers every permission request with a refusal, and it writes a note that names what the agent asked for.
At the levels "Automatic review" and "Full access" Claude Code rarely asks.
At the three narrower levels an agent of sprint 4 cannot change a file that it must ask for, and the note says why.

### What the runner sends on

| Output of the harness | Item |
|---|---|
| Text of the agent | An `agent` item, which grows. |
| A tool call and its updates | A `tools` item with the title and the latest status. |
| A permission request | A `note` item, and a refusal to the harness. |
| The agent's thoughts, plans and every other update | Nothing in sprint 4. |

Every text passes through one function that removes secrets before it leaves the machine.
In sprint 4 that function knows the client's own key and the token of its connection.
Sprint 8 gives it the sign-ins, and a later design gives it the secrets of tools.

### When the client stops or restarts

A client that stops ends every harness process group that it started.
A client that starts finds no agent running, says so with `AgentsRunning`, and the server marks its agents as failed.
Sprint 7 replaces this with a conversation that continues.

## Folders on the client

`LookAtFolder` accepts an absolute path, or a path that begins with `~` for the home folder.
The client answers with one of: a git repository with its name, main branch and remote; not a folder; not a git repository; or not readable.
`MakeRepository` accepts a name of letters, digits, dots, dashes and underscores.
The client makes the folder under the folder for repositories that the machine reported when it joined.
It refuses a name that already exists there as anything but a git repository.

## What the website's data layer does

The screens stay as they are. Their data comes from the server where it came from sample data.

- The lists of repositories, projects and agents follow `WatchProjects` and `WatchAgents`, in the way the Machines page follows `WatchClients`.
- The chat screen follows `WatchThread` of the agent it shows. Its composer calls `SendMessage`, its stop button calls `CancelReply`, and its menu calls `StopAgent`.
- "New project" calls `CreateRepository` for a new name, then `CreateProject`. "Add repository" calls `AddRepository`.
- The dialog wants a subscription for the lead. The server has none before sprint 8. The data layer therefore shows one entry for each machine on which Claude Code was found, which stands for the sign-in that Claude Code has there.
- The dialog's lists of models and efforts come from `ListLeadChoices`.

The server asks a machine to describe Claude Code after each scan that finds it, and it stores the answer.
`ListLeadChoices` reads that stored answer, so the dialog does not wait for a machine.

The plan of the website checks each control of the chat screen and of the two dialogs against this list.
A control that would do nothing in sprint 4 is reported to the owner before the build, because a change of a visible screen needs the owner's approval.

## How the server keeps its data

| Table | Part | What a row is |
|---|---|---|
| `repositories` | Projects | A repository: name, main branch, remote. |
| `repository_folders` | Projects | The folder of one repository on one machine. |
| `projects` | Projects | A project: repository, machine, goal, lead. |
| `harness_choices` | Agents | What one harness offers on one machine. |
| `agents` | Agents | An agent and its state. |
| `thread_items` | Agents | One item of one thread. |

The Projects part creates the lead by calling the Agents part's Go interface inside its own database write, as rule 3 demands.
The project, the agent, the first item and the command `StartAgent` are stored in one write, or none of them is.
Each write names what changed, so that the change feed serves the three `Watch` calls.

## Errors that the owner can meet

| What went wrong | What the owner sees |
|---|---|
| The machine is offline. | The dialog or the composer refuses, with a sentence that names the machine. |
| The folder is not a git repository. | "Add repository" refuses with the reason. |
| The adapter cannot be downloaded, or its hash differs. | The agent is `failed`, with the reason and one sentence. |
| Claude Code is not signed in on the machine. | The agent is `failed`, with the reason "signed out". |
| The harness refuses the model or the effort. | The agent is `failed`, with the reason "choice refused". |
| The account has no usage left. | The turn ends as failed, and a note carries the harness's sentence. |
| The harness process ends by itself. | The agent is `failed`, with the exit code. |
| The client restarts. | The agent is `failed`, with the reason that the client restarted. |

The list of failure reasons is fixed in the contract: adapter missing, signed out, choice refused, start timed out, folder missing, process ended, client restarted, and other.

## How sprint 4 is proven done

Tests use a stand-in for the harness: a small Go program that speaks the agent's side of ACP and follows a script.
It lets a test produce slow output, a permission request, a crash and a refused choice without an account.
Tests of the link use a real client against a real server in one process, as rule 10 demands.

Sprint 4 is done when these checks pass. Checks S1 to S9 run with the stand-in, and S10 runs once with the real Claude Code.

| Check | What is observed |
|---|---|
| S1 | `AddRepository` stores a git repository, and refuses a folder that is none, each with its reason. |
| S2 | `CreateProject` shows the goal as the first item, then the agent's answer growing, then the agent as `idle`. |
| S3 | A second message to the idle agent is answered. A message during a turn is refused with one sentence. |
| S4 | `CancelReply` ends the turn, the thread holds a note, and the next message is answered. |
| S5 | `StopAgent` ends the process group, and the agent is `stopped`. |
| S6 | The server restarts in the middle of an answer. No text is lost and none appears twice. |
| S7 | The client restarts. The agent is `failed` with the reason that the client restarted, and no harness process is left. |
| S8 | A permission request of the stand-in is refused, and the thread holds the note. |
| S9 | An adapter whose hash differs is not started, and the agent is `failed` with the reason. |
| S10 | On the deployed stack, with a real machine and the real Claude Code, the owner's goal is answered on the chat screen, and the thread is the same after a reload. |

## Open questions

1. Does the self-contained adapter work with the installed Claude Code and its sign-in? The plan of the runner checks it first.
2. Which controls of the chat screen and of the two dialogs would do nothing in sprint 4? The plan of the website lists them for the owner.
3. Sprint 5 must settle how a message steers a running turn. Claude Code and Codex offer a steering call of their own beside the ordinary prompt. Cursor offers none, and a second prompt cancels its running turn.

None of them blocks the contract of sprint 4.
