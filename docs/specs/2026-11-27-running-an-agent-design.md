# Running an agent in the Go rewrite: the sprints of wave 2, and the first two in detail

This document is a design.
It cuts the first unit of wave 2, running an agent, into seven sprints of slowly rising difficulty.
It designs the first two of them in enough detail to plan and build them: a repository on a machine, and the first agent.
Each later sprint gets a short section of its own in this document before it is planned.
Nothing in it is built yet.

It follows the foundation design, `docs/specs/2026-11-25-go-rewrite-foundation-design.md`.
The rules of that document's section "Rules that every later part follows" are called rule 1 to rule 11 here.

This is the second draft.
The first draft was reviewed twice on 2026-11-27, and both reviews asked for a revision.
The reviews are `.plans/designs/running-an-agent/round-1-logic.md` and `round-1-fit.md`.

## What exists before this design

Waves 0 and 1 are built on the branch `sprint-0`.
A machine joins a server, the owner approves it, and the machine stays connected over one link.
The link carries five kinds of message, and none of them concerns an agent.
The contract for the website covers sign-in, machines, the map and previews.

The website is deployed in its sample mode.
It loads sample repositories, projects, agents and conversations into the browser, and a simulation in the browser plays the agents.
Only sign-in and machines come from the server.
The deployed website therefore shows sample projects that name machines the server does not have, and its "New project" dialog simulates a lead.

The website has no "Chat" tab outside the mode of the old engine.
The owner talks to a project lead in a column that stands beside every view of the project.
A new project opens on the view "End result", with that column at its side.

## Decisions of the owner

| Date | Decision |
|---|---|
| 2026-11-25 | Version one drives every harness through the Agent Client Protocol (ACP) only. |
| 2026-11-25 | A message to a running agent steers its turn, whether the owner or another agent sent it. |
| 2026-11-25 | The owner signs in to a harness account once, and the server gives each client what it needs. |
| 2026-11-25 | One user on a machine is a convenience for now. Later the client runs as a user of its own, apart from the agents. |
| 2026-11-26 | The Go rewrite carries the five permission levels, with automatic review as the level of a new agent. See `docs/features.md`, "The automatic-review permission mode in the Go rewrite". |
| 2026-11-27 | Wave 2 is cut into several sprints, and each sprint adds a little difficulty to the one before. |
| 2026-11-27 | The first agent is shown on the website in the same sprint that builds it. |
| 2026-11-27 | An agent starts through the "New project" dialog as it is. The project lead runs in the folder of the repository itself, and the goal is its first message. |

## The sprints

| Sprint | What it adds | What the owner can see at its end |
|---|---|---|
| 4 | A repository on a machine. The website, with a real server, shows real repositories and no sample project. Link revision 2. | The owner adds a repository on a machine from the website, and makes a new one. |
| 5 | The first agent, on Claude Code. It starts through "New project", answers the goal, takes further messages while it is idle, and can be cancelled and stopped. | The lead's column shows a real agent that answers. |
| 6 | A conversation in full. A message steers a running turn. "Resume" loads the saved conversation. The thread shows the details of tool calls, plans and long output, and it loads older items in pages. The owner can compact the context. | The owner interrupts a running agent with a new instruction, and the agent follows it. |
| 7 | Permissions. A request of the agent reaches the owner, and the answer goes back. The level of a living agent can change. | An agent asks before a change, and the owner answers on the website. |
| 8 | Survival. The conversation continues after a restart of the client and after a crash of the client, or the thread says plainly what was lost. | The owner restarts the client in the middle of a conversation, and the conversation continues. |
| 9 | Every harness. Codex, Cursor and Gemini beside Claude Code, each with its own proof of steering. A change of model and effort while the agent lives. The sign-in that the server holds, hands to each machine and renews. | The owner starts an agent on each harness and changes its model. |
| 10 | Repositories in full. A clone and a branch for each project, several agents on one machine up to its limit, a machine chosen by the server, and a machine that the owner declares gone for good. | Two agents work in separate clones of one repository. |

Sprints 4 and 5 can be built as one sprint if the owner prefers. Nothing else in this document changes then.

These come after sprint 10, each with a design of its own: teams, and with them a message from one agent that steers another; tasks and the build loop; memories; the secrets of tools; terminals; files and images attached to a message; the views Files and Git; and the investigation agent.

## Rules that every sprint of this document follows

### Four identifiers keep late answers away from the wrong thing

| Identifier | Who makes it | What it is for |
|---|---|---|
| Run | The client, once at each start of its process. | Telling the agents of an older client process from those of the current one. |
| Start | The server. It is the identifier of the stored command `StartAgent`. | Tying the client's report of a start to the one start that the server asked for. |
| Turn | The server. It is the number of the thread item that began the turn. | Tying a cancellation, an end of turn and a result to the one turn they mean. |
| Operation | The website. It is the request key of the call. | Finding the same folder operation again after a lost reply. |

The server keeps all four in the database, as rule 7 demands.

### A command that ends, however it ends, changes what it was for

The link of wave 1 settles a stored command once, with one of these outcomes: done, failed, expired, unsupported, interrupted, or timed out by the server's sweep.
Sprint 4 adds two things to the link's Go interface on the server.
Storing a command returns its identifier.
A part can register to be called when one of its commands is settled, inside the same database write.

Every command of this document says below what each outcome does.
No outcome leaves a record waiting for ever.

### Nothing of an agent is sent to a client process before its past is known

A client that starts may hold events of its previous process in its buffer on disk.
The server must apply those before it sends that client any work for an agent.

- When the client opens the link, it names its run and the number of the first event that this run will send.
- The server holds every command that concerns an agent until it has applied all events below that number.
- At that moment, in one write, the server notes the run as the machine's current run, and the Agents part marks every agent of an older run on that machine that is `working` or `idle` as failed, with the reason "client restarted".
- A start that the older process began and did not finish is answered with `interrupted` by the new process. The outcome table of `StartAgent` covers it.

A lost connection without a restart keeps the run. Nothing is marked then.

### Every message of the link has a size budget

One message of the link is at most 64 kibibytes.
Every command and every event of this document is at most 48 kibibytes when encoded.
The server checks a command before it stores it. The client checks an event before it numbers it.

| Field | Limit | When the value is longer |
|---|---|---|
| A goal or a message | 32 kibibytes | The call of the website is refused. |
| A path | 4 kibibytes | The call of the website is refused. |
| The text in one `ThreadOutput` | 16 kibibytes | The runner sends the rest in the next event. |
| The title of a tool call | 512 bytes | The runner shortens it and ends it with an ellipsis. |
| A note | 1 kibibyte | The runner shortens it. |
| The list of models, or of efforts | 100 entries of 128 bytes | The runner sends the first 100 and marks the list as cut. |

### What leaves a machine passes one filter

The owner decided on 2026-11-25 that the client and the harness run as one user for now.
An agent can therefore read the files of the client, among them its key.
This design does not prevent that. It prevents the client from carrying such a value to the server.

- Every text that the runner sends passes one filter: the agent's text, the title of a tool call, a note, the name of a branch and the address of a remote.
- The filter knows the client's key and the token of its connection, in every form in which the client holds them.
- The filter works on the stream, not on single pieces. It holds back the end of a text until it knows that no secret continues across the cut.
- A remote address loses any user name and password it carries.
- The text of a failure never comes from the harness. The client sends a reason from a fixed list, and the server chooses the sentence.
- The standard error output of a harness stays on the machine.

Sprint 9 gives the filter the sign-ins that the server hands out. A later design gives it the secrets of tools.
Until sprint 9, a token of the harness's own sign-in is not known to the filter. This gap is recorded in `docs/issues.md` when sprint 5 is planned.

## Link revision 2

Revision 2 adds fields and messages, and changes no existing field.

- `Command.action` and `Event.report` in `commands.proto` and `events.proto` gain new members with new field numbers. The messages they carry are declared in two new files of the same package, `agents.proto` and `folders.proto`.
- The first message of the client on `Open` gains the run and the number of the run's first event.
- Both declarations of the revision in `link.proto` become 2.
- The record of frozen revisions keeps its entry for revision 1 and gains an entry for revision 2. The freeze of revision 2 is checked against the base of revision 1 during the review of the contract, and the base moves to revision 2 after that review.

A server of sprint 4 speaks revision 2 only.
It refuses a client of revision 1 on the existing call with the existing code `link_revision_mismatch`, and the owner installs the newer client with the installer of sprint 3.
The newer client reads the buffer and the receipts that a client of revision 1 left on disk. A test of sprint 4 proves it.

The files `agents.proto` and `folders.proto` of the link belong to the contract plan of each sprint.
Rule 2 holds: the contract of a sprint is written and reviewed before any code of that sprint.

## Sprint 4: a repository on a machine

### The parts

| Part | Folder | Contract file | What it owns |
|---|---|---|---|
| Projects, on the server | `src/server/internal/projects/` | `lgtm/v1/projects.proto` | Repositories, the folder of each repository on each machine, folder operations, and from sprint 5 projects. |
| Folders, on the client | `src/client/internal/folders/` | the link's `folders.proto` | Looking at a folder and making a repository. |

The Projects part keeps its database changes in its own folder of numbered changes.

### `ProjectService` in sprint 4

| Call | What it does |
|---|---|
| `WatchRepositories` | Sends every repository with its folders, then each change. It is served through the shared `watch` folder, as rule 5 demands. |
| `InspectFolder` | Takes a machine and a path. Returns what the client found there. It stores no repository. |
| `AddRepository` | Takes a machine, a path, and optionally a repository that is already known. It stores the folder when the client found a git repository there. |
| `CreateRepository` | Takes a machine and a name. The client makes the repository. The call returns the repository and its folder. |

`AddRepository` with a known repository attaches one more folder to it. This is the dialog's action "Add to a machine".
Without one, it stores a new repository, named after the folder.
When the folder's remote equals the remote of a known repository, the folder is attached to that repository instead.

### A folder operation outlives the call that began it

The three calls above need an answer from a machine. Each carries a request key, and each works the same way.

1. In one short write, the server stores an operation under the request key, and a command for the machine that expires after 20 seconds.
2. The call then waits for the operation to finish, for at most 25 seconds.
3. The client answers with an event that names the operation. The server finishes the operation when it applies that event, and stores the repository and its folder in the same write, whether or not the call still waits.
4. A call that waited in vain returns `deadline_exceeded` with one sentence. A second call with the same request key finds the same operation, and waits for it or returns its result.

| Outcome of the command | What the operation becomes |
|---|---|
| Done | Nothing by itself. The event finishes the operation. |
| Failed | Failed, with the reason of the event. |
| Expired, unsupported | Failed: the machine did not take the command. Nothing was made. |
| Interrupted, timed out | Unknown: the folder may exist. The call says so. |

### What the client does with a folder

`LookAtFolder` accepts an absolute path, or a path that begins with `~` for the home folder.
The client resolves the path to the top folder of the git repository, with every link in the path followed.
It answers with the resolved path, the name, the main branch, the remote, and whether the working tree is clean.
Or it answers with one reason: not a folder, not a git repository, or not readable.

The server stores the resolved path. One machine cannot hold the same resolved path twice, so two spellings of one folder give one row.

`MakeRepository` accepts a name of letters, digits, dots, dashes and underscores, which is not `.` and not `..`.
The client makes the folder under the folder for repositories that the machine reported when it joined, and starts a git repository in it with the main branch `main`.
It adopts a folder of that name that is empty, or that is a git repository without a commit. It refuses any other folder of that name.
A second attempt after an unknown outcome therefore completes the first.

### The website in sprint 4

**With a server that has `ProjectService`, the website shows no sample record below a machine.**
It loads no sample repository, project, agent, thread, task, team or request, and its simulation does not run.
The in-page sample server and the mock server do not have `ProjectService`. With them the website stays as it is, so the sample remains the reference for the design of the screens.

- The tree of repositories follows `WatchRepositories`, in the way the Machines page follows `WatchClients`. The website waits for both before it shows the workspace.
- "Add repository" calls `InspectFolder` while the owner types, and fills its rows "Base branch", "Remote" and "Status" from the answer. Its button calls `AddRepository` and returns to the dialog that opened it, with the stored repository selected.
- Both dialogs start on the machine that opened them, or on the first machine that is online. They no longer name a sample machine.
- A sign-out and a change of server clear the repositories as they clear the machines.

Two things on these screens change what the owner sees. They need the owner's approval before sprint 4 is built.

1. "Add repository" says today, for every folder, that the working tree is clean. With real data, a repository with uncommitted changes needs a second sentence.
2. "New project" cannot start an agent before sprint 5. In sprint 4 its button answers that projects are not connected to this server yet, in the way the views Files and Terminal already answer in the mode of the old engine.

### How sprint 4 is proven done

Tests of the server run against a real PostgreSQL, and tests of the link use a real client against a real server in one process, as rule 10 demands.

| Check | What is observed |
|---|---|
| R1 | `InspectFolder` returns the resolved path, the branch, the remote and the state of the working tree of a real repository. It stores no row. |
| R2 | `AddRepository` stores the repository and its folder under the chosen machine. A folder that is no git repository stores nothing and returns the reason. |
| R3 | Two spellings of one folder give one folder row, and the second call names the first. |
| R4 | `AddRepository` with a known repository attaches a folder on a second machine to it. |
| R5 | `CreateRepository` makes the folder, and the repository's main branch is `main`. A name that leaves the folder for repositories is refused and makes nothing. |
| R6 | The reply of `CreateRepository` is lost after the client made the folder. The same request key returns the same repository, and one row exists. |
| R7 | The machine disconnects before it takes the command. The call returns after 25 seconds, the operation is failed, and nothing is made later. |
| R8 | A client of revision 1 is refused with `link_revision_mismatch`. A client of revision 2 reads a buffer and receipts written by revision 1. |
| R9 | On the deployed stack, with a real machine, the owner adds a repository from the website and sees it in the tree after a reload. No sample project is shown. |

## Sprint 5: the first agent

### What the owner does and sees

The owner opens "New project", writes a goal, picks a machine and a repository, and picks Claude Code.
The website opens the new project on "End result", with the lead's column at its side.
The column shows the goal, then the answer as it is written, then the lead as idle.
The owner sends another message, and the lead answers again.
The owner can cancel a running answer and can stop the lead.
After a reload of the page, the column is as it was.

The view "End result" itself says "No statements", because a lead of sprint 5 cannot write an end result yet.

### The parts

| Part | Folder | Contract file | What it owns |
|---|---|---|---|
| Projects, on the server | as in sprint 4 | `lgtm/v1/projects.proto` | It gains projects. |
| Agents, on the server | `src/server/internal/agents/` | `lgtm/v1/agents.proto` | Agents, their states, the items of their threads, and what each harness offers on each machine. |
| Runner, on the client | `src/client/internal/runner/` | the link's `agents.proto` | The harness processes, their ACP sessions, and the translation of their output. |

```text
Website                      Server                          Client                Harness
  CreateProject  ───────────▶ projects ─▶ agents
                               stores project, agent, goal
                               stores command StartAgent ───▶ runner ── starts ──▶ Claude Code
  WatchThread    ◀─────────── agents ◀── events ◀──────────── runner ◀── ACP ──── output
  SendMessage    ───────────▶ agents: stores item, command Prompt ─▶ runner ─────▶ prompt
```

### The contract for the website in sprint 5

`ProjectService` gains two calls.

| Call | What it does |
|---|---|
| `WatchProjects` | Sends every project, then each change. |
| `CreateProject` | Takes a goal, a machine, a repository, and the lead's harness, model, effort and permission level. It stores the project and its lead, starts the lead, and returns both identifiers. |

`AgentService`, in `agents.proto`:

| Call | What it does |
|---|---|
| `WatchAgents` | Sends every agent, then each change. |
| `WatchThread` | Takes an agent. Sends the newest items of its thread, then each new or changed item. |
| `WatchHarnessChoices` | Sends, for each machine and harness, whether the harness is signed in and which models and efforts it offers, then each change. |
| `SendMessage` | Takes an agent and a text. It stores the message as an item of the thread and sends it to the agent. |
| `CancelReply` | Takes an agent and a turn. It cancels that turn. |
| `StopAgent` | Takes an agent. It ends the harness process. |

Every `Watch` call is served through the shared `watch` folder. Every call that changes something carries a request key and uses the store's receipt, so a repeated call returns the first reply.

`CreateProject` is one write.
It takes the lock of the machine first, as the Machines part does before it stores a command, so that commands leave in the order of their numbers.
It stores the project, calls the Agents part's Go interface to store the lead and the first item, and stores the command `StartAgent`, as rule 3 and rule 4 demand.
Either all of it is stored or none.

A folder of a repository holds one project at a time in sprint 5, because a lead works in the main checkout.
The database enforces it for the resolved folder. A project whose lead is stopped or failed does not count.

The server gives a project its name, from the first words of the goal.

`WatchThread` first sends at most the newest 500 items and at most 2 mebibytes of text. Sprint 6 adds pages for older items.

### An agent, as the website receives it

| Field | Meaning |
|---|---|
| Identifier, project, machine, role, name | The role of sprint 5 is always project lead. |
| Harness, model, effort | The model and the effort are what the session reported, not what was asked. |
| Permission level | One of the five levels. |
| State | `starting`, `working`, `idle`, `stopped` or `failed`. |
| Active turn | The turn that runs, if one runs. |
| Stop asked | Whether the owner asked to stop the agent and the machine has not confirmed it. |
| Failure | A reason from a fixed list, and whether the agent failed before or after its start. |
| Started at, last output at | Times of the server. |

The fixed reasons are: adapter missing, signed out, choice refused, start timed out, start not delivered, start interrupted, folder missing, process ended, cancel ignored, client restarted, machine removed, and other.

### An item of a thread

| Kind | What it holds |
|---|---|
| `user` | A message of the owner. |
| `agent` | Text that the agent wrote. It grows while the agent writes. |
| `tools` | One tool call: its title and its status. Sprint 6 adds its details. |
| `note` | One sentence, such as a cancelled reply or a refused permission request. |

Each item has a number that rises within its thread, the turn it belongs to, and the time it began.
The server gives the number. An item of the runner gets its number when the server applies the first event that names it.

### The states of an agent

| From | What happens | To |
|---|---|---|
| none | `CreateProject` stores the agent and the command `StartAgent`. | `starting` |
| `starting` | The event `AgentStarted` that names this start is applied. | `working`, with turn 1 active |
| `starting` | The event `AgentStartFailed` that names this start is applied. | `failed` |
| `starting` | The command `StartAgent` is settled as expired, unsupported, interrupted or timed out. | `failed` |
| `starting` | 150 seconds pass after the command expired. | `failed`, with the reason "start timed out" |
| `working` | The event `TurnEnded` that names the active turn is applied. | `idle` |
| `idle` | `SendMessage` stores the item and the command `Prompt`. | `working`, with that turn active |
| `working` | The command `Prompt` of the active turn is settled as failed, expired or unsupported. | `idle`, with a note that the message did not reach the agent |
| `starting`, `working`, `idle` | The event `AgentEnded` is applied. | `stopped` when the owner asked for it, else `failed` |
| `working`, `idle` | The machine's run changes. | `failed`, with the reason "client restarted" |
| any but `stopped` | The owner removes the machine. | `failed`, with the reason "machine removed" |

`stopped` and `failed` are final in sprint 5.
An event or an outcome that names another start or another turn than the stored one changes nothing.
An event `AgentStarted` for an agent that is final is answered with a command `StopAgent`, so that no harness runs without a record.
Output for a final agent is still stored, because it was written before the end.

The server changes the state in the same write that stores the command, applies the event, or settles the command.
A sweep of the Agents part applies the rule of 150 seconds. It reads the agents from the database each time.
An event is applied only when the machine that sent it is the machine of the agent.
A machine that goes offline does not change the state of its agents. The website already shows that the machine is offline.

`SendMessage`, `CancelReply` and `StopAgent` are refused while the machine is not connected.
`SendMessage` is also refused while a turn runs, while the agent starts, while a stop is asked, and when the agent is final.

### Commands of sprint 5

Every command goes through the link's stored commands, as rule 6 demands. All of them wait for the client's past, as described above.

| Command | What it carries | Expires after | When the client answers `done` |
|---|---|---|---|
| `DescribeHarness` | A harness. | 5 minutes | The adapter is on the machine, and the event `HarnessDescribed` is in the buffer. |
| `StartAgent` | The start, the agent, the harness, the folder, the permission level, the model, the effort, and the goal. | 5 minutes | The session is open and set, the goal is written to the harness, and the event `AgentStarted` is in the buffer. |
| `Prompt` | The agent, the turn, and the text. | 2 minutes | The text is written to the harness. |
| `CancelTurn` | The agent and the turn. | 2 minutes | The harness was told to cancel that turn, or that turn no longer runs. |
| `StopAgent` | The agent. | 23 hours | No process of the agent is left, and none can start. |

`StartAgent` carries the goal, so that the start and the first turn cannot arrive in the wrong order.
The reply of ACP to a prompt arrives when the turn ends. The start therefore ends when the goal is written to the harness, and the end of the turn is an event of its own.
The server sends `Prompt` only to an idle agent, so one agent never has two prompts on the way.

`StopAgent` has a long life, so that a stop asked while the link is down still arrives.
A client that gets `StopAgent` for an agent it does not run remembers the agent on disk, refuses a later `StartAgent` for it, and reports `AgentEnded`.
A stop that arrives while the start still runs ends the start.
When `StopAgent` is settled as timed out, the machine did not return for a day, and the agent becomes `failed`.

After `CancelTurn` the runner waits 10 seconds for the harness to end the turn. If it does not, the runner ends the process group, and the agent is `failed` with the reason "cancel ignored".

### Events of sprint 5

| Event | What it carries |
|---|---|
| `HarnessDescribed` | The command it answers, the harness, the adapter's version, whether the harness is signed in, and the models, efforts and modes it offers. |
| `AgentStarted` | The start, the agent, the session's identifier, and the model, effort and mode that the session reports. |
| `AgentStartFailed` | The start, the agent, and a reason from the fixed list. |
| `ThreadOutput` | The agent, the turn, the item, its kind, the position where the new text begins, and the new text; or the revision and the status of a tool call. |
| `TurnEnded` | The agent, the turn, and how it ended: finished, cancelled, refused, no usage left, signed out, or failed. |
| `AgentEnded` | The agent, whether the owner asked for it, and a reason from the fixed list. |

Events are numbered and acknowledged by the link of wave 1, so each is applied once and in order.
The server stores a description of a harness only when it answers the newest `DescribeHarness` for that machine and harness.

### How output travels

- The runner gives each item an identifier of its own.
- A position is a count of bytes of the text after the filter. An item begins at position zero.
- The runner gathers the text of an item for 300 milliseconds before it sends it. It cuts only between two characters.
- An item of text is closed at 32 kibibytes, and the text continues in a new item.
- A tool call carries a revision that rises with each change. The server keeps the highest.
- Before the runner reports the end of a turn, it sends everything it holds for that turn, in order.

The position is a check, not the mechanism. The link already applies each event once.
An event whose position is not the length of the stored text is not applied, and the server adds the note "Some output of the agent is missing here."

The website receives the whole item on each change, as the shared `Watch` shape demands.
The bounds are those above: 16 kibibytes for an event, 32 kibibytes for an item, and 500 items or 2 mebibytes for the first message of `WatchThread`.

### When the client's buffer is full

The buffer of events on disk is bounded. A server that is away for long fills it.

- The link never waits for the runner. It keeps sending, acknowledging and reconnecting while the runner waits for room.
- The runner keeps at most 64 kibibytes of output of each agent in memory. When the buffer has no room, the runner stops reading the output of that harness until there is room. Nothing is dropped.
- The buffer keeps a reserve that only events of a start, an end of turn and an end of agent may use, so that a stop is reported while output waits.
- A client that crashes loses the output that was not yet in the buffer, at most 300 milliseconds and 64 kibibytes for each agent. The agent is failed then in any case.

### The runner and Claude Code

**The adapter.** Claude Code speaks ACP through an adapter program.
The public registry of ACP lists the adapter as one self-contained program for Linux and for macOS.
The client's source names one version of the adapter and the SHA-256 hash of its program for each platform.
`DescribeHarness` downloads the adapter into the client's state folder when it is absent, and checks the hash. A program whose hash differs is never started.
`StartAgent` does not download. Without the adapter it fails with the reason "adapter missing".
The server sends `DescribeHarness` after each scan that finds Claude Code.

The plan of the runner checks one fact before any code is written: whether the self-contained adapter works with the Claude Code that is installed and signed in on a machine, when the client runs as a service.
If it does not, the client runs the adapter from its package instead, and the machine then needs Node.

**One process for one agent.** The runner starts one adapter process for each agent, in a process group of its own, in the folder of the repository.
The process gets a short list of environment variables: the search path, the home folder, the user, the shell, the language settings, the terminal type, the temporary folder and the time zone.

The runner then opens one ACP session, sets the mode, the model and the effort, and writes the goal.
It allows 120 seconds from the start of the process to the written goal.
A value that the session does not offer is a failed start with the reason "choice refused".

The agent uses the sign-in that Claude Code already has for that user on that machine. The server stores no sign-in before sprint 9.

**Describing the harness.** For `DescribeHarness` the runner opens a session, reads what it offers, and closes it. It sends no prompt.
A harness that answers that it needs a sign-in is reported as signed out.

**The five permission levels on Claude Code.**

| Level | Mode of Claude Code |
|---|---|
| Read only | `plan` |
| Ask before every change | `default` |
| Edit files without asking | `acceptEdits` |
| Automatic review | `auto` |
| Full access | `bypassPermissions` |

A new agent gets the level "Automatic review" unless the owner's saved choice says otherwise.
In sprint 5 the runner answers every permission request itself: with the option that refuses once, or with a cancellation when the harness offers no such option.
It writes a note that names what the agent asked for.
At the levels "Automatic review" and "Full access" Claude Code rarely asks. At the three narrower levels an agent of sprint 5 cannot do what it must ask for, and the note says so.

**What the runner sends on.**

| Output of the harness | Item |
|---|---|
| Text of the agent | An `agent` item, which grows. |
| A tool call and its updates | A `tools` item with the title and the latest status. |
| A permission request | A `note` item, and a refusal to the harness. |
| The agent's thoughts, plans and every other update | Nothing in sprint 5. |

**When the client stops, restarts or dies.** A client that stops in order ends every process group that it started, and reports each end.
The runner also writes each process group and its start time to disk when it starts one.
A client that starts ends every recorded group that still lives, before it opens the link. This covers a client that died without a chance to clean up.
Sprint 8 replaces the ending of agents at a restart with a conversation that continues.

### The website in sprint 5

The data layer of the website gains `WatchProjects`, `WatchAgents`, `WatchThread` and `WatchHarnessChoices`.
The website waits for machines, repositories, projects and agents before it shows the workspace, so that a project is never shown without its lead.
The address of a project after a reload works as soon as those lists have arrived.

**"New project".**

- Its lead choice wants a subscription. The server has none before sprint 9. The data layer shows one entry for each machine on which Claude Code is described, which stands for the sign-in that Claude Code has there. A machine whose Claude Code is signed out has no usable entry, and the dialog's existing sentence asks for a signed-in subscription.
- Its lists of models and efforts come from `WatchHarnessChoices`.
- Its button calls `CreateRepository` for a new name, then `CreateProject`, and opens the project that the server returned.

**The lead's column.**

| Control | In sprint 5 |
|---|---|
| The message box, Send, Enter | Calls `SendMessage`. A refused message stays in the box, and the refusal is shown as the website shows other failed actions. |
| "Stop this reply" | Shown while a turn runs. Calls `CancelReply` with that turn. |
| "Stop agent" | Calls `StopAgent`. |
| The context percentage | Shows a dash. No value exists before sprint 6. |
| The menus Subscription, Model and Effort | Each shows the current value as its only entry. A change comes in sprint 9. |
| "Context", and typing an at sign | Opens the picker, which finds nothing, because a project of sprint 5 has no tasks. |
| Attaching, dropping and pasting a file | Refused with the sentence that the mode of the old engine already uses. |
| "Resume", on a stopped or failed lead | Answers that it is not connected yet. It comes in sprint 6. |
| "Sign in on" a machine, on a failed lead | Answers that it is not connected yet. It comes in sprint 9. |

A lead that failed before its start is shown by the existing failed composer, with the reason.
A lead that failed after its start is shown by the existing stopped composer, with the reason as its line.

**The other views of a real project.**
The tabs Questions, Status, Actions, Design, Files, Git and Settings stay.
Each shows its existing empty state where it has one that is true, such as "No questions".
A view that would show sample data or let the owner change a record that the server does not hold shows the existing "not connected" page.
The plan of the website lists every view with the one or the other, and the owner sees that list before the build.

Three things in sprint 5 change what the owner sees, and need the owner's approval before the build:

1. "Resume" and "Sign in on" answer that they are not connected yet.
2. The stopped composer says today that "Resume continues from its last turn". That is true from sprint 6.
3. Some views of a real project show "not connected".

### How the server keeps its data

| Table | Part | What a row is |
|---|---|---|
| `repositories` | Projects | A repository: name, main branch, remote. |
| `repository_folders` | Projects | The resolved folder of one repository on one machine. |
| `folder_operations` | Projects | One operation: its request key, its command, its state and its result. |
| `projects` | Projects | A project: repository, folder, machine, name, goal, lead. |
| `harness_choices` | Agents | What one harness offers on one machine, and the newest command that asked. |
| `agents` | Agents | An agent: its state, its start, its active turn, its run, its session. |
| `thread_items` | Agents | One item of one thread, with the runner's identifier of it. |

Each write names what changed, so that the change feed serves the `Watch` calls.

### Refusals and their codes

Rule 8 demands a Connect code and one sentence for each error.

| Refusal | Code |
|---|---|
| The machine is not connected. | `unavailable` |
| The folder is not a folder, not a git repository, or not readable. | `failed_precondition` |
| The folder is already added, or already holds a project. | `already_exists` |
| A name, a path or a text breaks a limit or a rule. | `invalid_argument` |
| A turn runs, the agent starts, a stop is asked, or the agent is final. | `failed_precondition` |
| The machine did not answer in time. | `deadline_exceeded` |
| The same request key arrives with another request. | `already_exists`, as the store does today |
| An agent, a project or a repository is unknown. | `not_found` |

### How sprint 5 is proven done

Tests use a stand-in for the harness: a small Go program that speaks the agent's side of ACP and follows a script.
It can write slowly, ask for a permission, start a child process, ignore a cancellation, end by itself, and write a planted secret.

| Check | What is observed |
|---|---|
| A1 | `CreateProject` with a goal that no other test uses. The stand-in receives that exact goal once, in the chosen folder, in the chosen mode. The thread shows the goal, then at least two growth steps of the answer, then the lead as `idle`. |
| A2 | A second message to the idle lead is answered. A message during a turn is refused, and the stand-in received exactly the two accepted messages. The same request key after the turn returns the first reply and starts no turn. |
| A3 | `CancelReply` ends the named turn at the stand-in, the thread holds a note, and the next message runs in the same session. A cancellation that names an ended turn changes nothing. |
| A4 | A stand-in that ignores a cancellation is ended after 10 seconds, and the lead is `failed` with the reason. |
| A5 | `StopAgent` ends the stand-in and a child that it started. The lead is `stopped` only after both are gone. A stop during the start leaves no process. |
| A6 | The server restarts after it stored a piece of an answer whose acknowledgement the client did not get. Client and stand-in stay alive. The finished thread holds the exact text once. |
| A7 | The client is killed without warning while the stand-in and its child run. After the client starts again, both are gone, the lead is `failed` with "client restarted", and a project created just before the kill either starts once or fails with a reason. |
| A8 | The machine is offline when the project is created, and stays away. The lead is `failed` with a reason within 8 minutes. |
| A9 | A permission request of the stand-in gets the refusing option, the thread holds the note, and the mode was set before the goal was written. A project created without a chosen level runs in the mode `auto`. |
| A10 | An adapter whose hash differs is not run: a marker that it would write is absent. An adapter with the right hash runs. |
| A11 | The stand-in writes the client's token, cut across two pieces, and once in the title of a tool call. Neither the thread nor a log of the server holds it. |
| A12 | The server is away while the stand-in writes more than the buffer holds. Nothing is lost, a stop is still reported, and the thread is complete afterwards. |
| A13 | On the deployed stack, with a real machine and the real Claude Code, the owner creates a project with "New project". The lead's column shows the goal and a growing answer of the real session, and the same content after a reload of the same address. |

## Open questions

1. Does the self-contained adapter work with the installed Claude Code and its sign-in, when the client runs as a service? The plan of the runner checks it first.
2. Sprint 6 must settle how a message steers a running turn. Claude Code and Codex offer a steering call of their own beside the ordinary prompt. Cursor offers none, and a second prompt cancels its running turn.
3. The foundation design asks whether Claude Code applies its sandbox when it is driven through ACP. No secret is handed to an agent before sprint 9, so the question is answered in the design of that sprint.

None of them blocks the contract of sprint 4.

## What waits for the owner

1. Whether sprints 4 and 5 are built as two sprints or as one.
2. The two visible changes of sprint 4 and the three of sprint 5, each listed in its section.
