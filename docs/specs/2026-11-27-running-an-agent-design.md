# Running an agent in the Go rewrite: the sprints of wave 2, and the first two in detail

This document is a design.
It cuts the first unit of wave 2, running an agent, into seven sprints of slowly rising difficulty.
It designs the first two of them in enough detail to plan and build them: a repository on a machine, and the first agent.
Each later sprint gets a section of its own in this document before it is planned.
Nothing in it is built yet.

It follows the foundation design, `docs/specs/2026-11-25-go-rewrite-foundation-design.md`.
The rules of that document's section "Rules that every later part follows" are called rule 1 to rule 11 here.

This is the third draft.
The first two drafts were each reviewed twice on 2026-11-27, and every review asked for a revision.
The reviews are in `.plans/designs/running-an-agent/`.

## What exists before this design

Waves 0 and 1 are built on the branch `sprint-0`.
A machine joins a server, the owner approves it, and the machine stays connected over one link.
The link carries five kinds of message, and none of them concerns an agent.
The contract for the website covers sign-in, machines, the map and previews.

The website is deployed in its sample mode.
It loads sample repositories, projects, agents and conversations into the browser, and a simulation in the browser plays the agents.
Only sign-in and machines come from the server.
The deployed website therefore shows sample projects that name machines the server does not have, and its "New project" dialog simulates a lead.

The owner talks to a project lead in a column that stands beside every view of the project.
A new project opens on the view "End result", with that column at its side.
The tab "Chat" exists only in the mode of the old engine, and this design does not use it.

## Decisions of the owner

| Date | Decision |
|---|---|
| 2026-11-02 | A security problem does not stop the build. It is recorded, and the owner reviews the list later. |
| 2026-11-25 | Version one drives every harness through the Agent Client Protocol (ACP) only. |
| 2026-11-25 | A message to a running agent steers its turn, whether the owner or another agent sent it. |
| 2026-11-25 | The owner signs in to a harness account once, and the server gives each client what it needs. |
| 2026-11-25 | One user on a machine is a convenience for now. Later the client runs as a user of its own, apart from the agents. |
| 2026-11-25 | The owner tells the server when a machine is gone for good. Its agents start again on another machine, each with a report of what happened before. |
| 2026-11-26 | The Go rewrite carries the five permission levels, with automatic review as the level of a new agent. See `docs/features.md`, "The automatic-review permission mode in the Go rewrite". |
| 2026-11-27 | Wave 2 is cut into several sprints, and each sprint adds a little difficulty to the one before. |
| 2026-11-27 | The first agent is shown on the website in the same sprint that builds it. |
| 2026-11-27 | An agent starts through the "New project" dialog as it is. The project lead runs in the folder of the repository itself, and the goal is its first message. |

## The sprints

| Sprint | What it adds | What the owner can see at its end |
|---|---|---|
| 4 | A repository on a machine. With a real server, the website shows real repositories and no sample project. Link revision 2. | The owner adds a repository on a machine from the website. |
| 5 | The first agent, on Claude Code. It starts through "New project", answers the goal, takes further messages while it is idle, and can be cancelled and stopped. A new repository by name. Link revision 3. | The lead's column shows a real agent that answers. |
| 6 | A conversation in full. A message steers a running turn. "Resume" loads the saved conversation. The thread shows the details of tool calls, plans and long output, and it loads older items in pages. The owner can compact the context. | The owner interrupts a running agent with a new instruction, and the agent follows it. |
| 7 | Permissions. A request of the agent reaches the owner, and the answer goes back. The level of a living agent can change. | An agent asks before a change, and the owner answers on the website. |
| 8 | Survival. The conversation continues after an orderly restart of the client and after a crash of the client, or the thread says plainly what was lost. | The owner restarts the client in the middle of a conversation, and the conversation continues. The owner kills the client, and the thread names what was lost. |
| 9 | Every harness. Codex, Cursor and Gemini beside Claude Code, each with its own proof of steering and its own table of the five levels. A change of model and effort while the agent lives. The sign-in that the server holds in OpenBao, hands to each machine encrypted to the machine's key, and takes back when it is renewed. | The owner signs in once on the website, and an agent starts on a machine that never saw that sign-in. The owner changes its model. |
| 10 | Repositories in full. A clone and a branch for each project, several agents on one machine up to its limit, a machine chosen by the server, and a machine that the owner declares gone for good. | Two agents work in separate clones of one repository. The owner declares a machine gone, and its agent starts again on another machine with a report and the last pushed commit. |

These come after sprint 10, each with a design of its own: teams, and with them a message from one agent that steers another; tasks and the build loop; memories; the secrets of tools; terminals; files and images attached to a message; the agent of a repository; following a repository by its remote; the views Files and Git; and the investigation agent.

Sprints 4 and 5 can be built as one sprint if the owner prefers. They then freeze one revision of the link, not two.

## Rules that every sprint of this document follows

### Four identifiers keep late answers away from the wrong thing

| Identifier | Who makes it | What it is for |
|---|---|---|
| Run | The client, once at each start of its process. | Telling the agents of an older client process from those of the current one. |
| Start | The server. It is the identifier that the link gives the stored command `StartAgent`. | Tying the client's report of a start to the one start that the server asked for. |
| Turn | The server. It is the number of the thread item that began the turn. | Tying a cancellation, an end of turn and a result to the one turn they mean. |
| Operation | The server, when a call of the website first arrives with its request key. | Finding the same folder operation again after a lost reply. |

The server keeps all four in the database, as rule 7 demands.
A report or a result that names another start, another turn or another run than the stored one changes nothing.
A report is applied only when the machine that sent it is the machine of the record.

### Changes of the link's Go interface on the server

Sprint 4 gives the Link part four things. Each later rule of this document rests on them.

1. **Storing a command returns its identifier.**
2. **Every write that stores a command takes the lock of the machine first,** before it locks any other row. The Machines part does this today. Commands then get their numbers in the order in which they are committed, and the stream cannot pass one by.
3. **A part can register to be called** when one of its commands is settled, when a report of its kind is applied, when a stream of a machine is admitted, and when the run of a machine changes. Each call happens inside the database write that does the thing. This is the Go interface that rule 3 asks for.
4. **A result waits for the events before it.** The client writes on each result the number of the last event it had put into its buffer. The server settles the result only after it has applied all events up to that number. A result that arrives early is stored and settled in the write that applies that event.

Point 4 means that when a result is settled, every report that the client made before it is already applied.
The outcome tables below rest on that.

### What an outcome proves

The link settles a stored command once.

| Outcome | What it proves |
|---|---|
| Done | The client did what the command's row below calls done. |
| Failed | The client refused, and did nothing that lasts. Each command's row says what "nothing" means. |
| Expired, unsupported | The client received the command and did not run it. |
| Interrupted | The client process died while it ran the command. What it did is unknown. |
| Timed out | The server waited two days past the command's expiry for a result. What happened is unknown. |
| Removed | The owner removed the machine. |

### Nothing is sent to a client process before its past is known

A client that starts may hold events of its previous process in its buffer on disk.

- When the client opens the link, it names its run and the number of the first event that this run will send.
- The server sends that stream no command until it has applied all events below that number. It checks this when it admits the stream and each time it applies an event. Commands that wait keep their place.
- When the check passes for a run that is new to the server, the server notes the run as the machine's current run, in one write. The parts that registered are called in that write.
- A command that the older process began and did not finish is answered with `interrupted` by the new process.

A lost connection without a restart keeps the run. The check passes at once, and nothing else happens.

### Every message of the link has a size budget

One message of the link is at most 64 kibibytes.
Every command and every event of this document is at most 48 kibibytes when encoded.
The server checks a command before it stores it. The client checks an event before it numbers it, so an event that is too large never blocks the buffer.

| Field | Limit | When the value is longer |
|---|---|---|
| A goal or a message | 32 kibibytes | The call of the website is refused. |
| A path that the owner typed | 4 kibibytes | The call of the website is refused. |
| A resolved path | 8 kibibytes | The client answers with the reason "path too long". |
| The address of a remote | 2 kibibytes | The client leaves it out and marks it as left out. |
| The text in one `ThreadOutput` | 16 kibibytes | The runner sends the rest in the next event. |
| The title of a tool call, a label | 512 bytes | The runner shortens it and ends it with an ellipsis. |
| A note | 1 kibibyte | The runner shortens it. |
| The list of models | 100 entries | The runner sends the first 100 and marks the list as cut. |
| An identifier of a model, an effort or a mode | 128 bytes | The runner leaves the entry out. An identifier is never shortened. |

### What leaves a machine passes one filter

The owner decided on 2026-11-25 that the client and the harness run as one user for now.
An agent can therefore read the files of the client and of the harness, among them the client's key and the harness's sign-in.
This design does not prevent that. It keeps such a value from travelling to the server through the client.

- Every text that the client puts into its buffer or its log passes one filter first: the agent's text, the title of a tool call, a note, a label, a path, the name of a branch and the address of a remote.
- The filter knows the client's key and the token of its connection, in every form in which the client holds them.
- The filter knows the values in the file in which Claude Code keeps its sign-in, where that is a file the client can read. It reads the file when an agent starts and when the file changes.
- The filter works on the stream, not on single pieces. It holds back the end of a text until it knows that no secret continues across the cut.
- A remote address loses any user name and password it carries.
- The text of a failure never comes from the harness. The client sends a reason from a fixed list, and the server chooses the sentence.
- The standard error output of a harness stays on the machine.

On macOS, Claude Code keeps its sign-in in the keychain, where the client cannot read it without a question to the user.
The filter does not know that value. An agent that prints it sends it to the owner's own server.
This gap is recorded in `docs/issues.md` and on the owner's list of security work, and it does not stop the build, by the decision of 2026-11-02.
Sprint 9 gives the filter the sign-ins that the server hands out. A later design gives it the secrets of tools, and answers the foundation design's question about the sandbox of Claude Code.

## Sprint 4: a repository on a machine

### Link revision 2

Revision 2 adds fields and messages, and changes no existing field.

- The first message of the client on `Open` gains the run and the number of the run's first event.
- `CommandResult` gains the number of the last buffered event.
- `Command.action` gains `LookAtFolder`, and `Event.report` gains `FolderSeen`, with new field numbers. Both messages are declared in a new file of the same package, `folders.proto`.
- Both declarations of the revision in `link.proto` become 2, and so do the two constants in the programs and the sentence in the contract's readme.
- The record of frozen revisions keeps its entry for revision 1 at its place and gains an entry for revision 2. The freeze of revision 2 is checked against the base of revision 1 during the review of the contract, and the base moves to revision 2 after that review.

A server of sprint 4 speaks revision 2 only.
It refuses a client of revision 1 on the existing call with the existing code `link_revision_mismatch`, and the owner installs the newer client with the installer of sprint 3.
The newer client keeps the buffer and the receipts that a client of revision 1 left on disk.

Rule 2 holds: the contract of a sprint, for the website and for the link, is written and reviewed before any code of that sprint.

### The parts

| Part | Folder | Contract file | What it owns |
|---|---|---|---|
| Projects, on the server | `src/server/internal/projects/` | `lgtm/v1/projects.proto` | Repositories, the folder of each repository on each machine, folder operations, and from sprint 5 projects. |
| Folders, on the client | `src/client/internal/folders/` | the link's `folders.proto` | Looking at a folder, and from sprint 5 making a repository. |

The Projects part keeps its database changes in its own folder of numbered changes.

### `ProjectService` in sprint 4

| Call | What it does |
|---|---|
| `WatchRepositories` | Sends every repository with its folders, then each change. It is served through the shared `watch` folder, as rule 5 demands. |
| `InspectFolder` | Takes a machine and a path. Returns what the client found there. It stores no repository. |
| `AddRepository` | Takes a machine, a path, and optionally a repository that is already known. It stores the folder when the client found a git repository there. |

`AddRepository` without a known repository stores a new repository, named after the folder.
`AddRepository` with a known repository attaches one more folder to it. This is the dialog's action "Add to a machine".
The folder may have another name than the repository. It must be the same repository: its remote, written in one normal form, equals the repository's remote, or both hold the same first commit.
A folder that fails this test is refused with a sentence that says so.

Adding a folder that is already stored for the same repository returns the stored row.
Adding it for another repository is refused with `already_exists`.

### A folder operation outlives the call that began it

Both calls that ask a machine carry a request key, and both work the same way.

1. In one short write under the lock of the machine, the server stores an operation and a command for the machine that expires after 20 seconds. The store's receipt for the request key holds only the identifier of the operation.
2. The call then waits for the operation to finish, for at most 25 seconds. It reads the operation from the database, so a restart of the server in between loses nothing.
3. The client answers with the event `FolderSeen`, which names the operation and the command. The server finishes the operation when it applies that event. For `AddRepository` it stores the repository and its folder in the same write, whether or not the call still waits. For `InspectFolder` it stores only the result.
4. A call that waited in vain returns `deadline_exceeded` with the sentence that the machine has not answered. A second call with the same request key finds the same operation, and waits for it or returns its result.

| Outcome of the command | What the operation becomes |
|---|---|
| Done | Whatever the event made it. An operation that is still open then is failed with the reason "no answer". |
| Failed | Failed, with the reason of the event, or "no answer". |
| Expired, unsupported | Failed: the machine did not take the command. Nothing was done. |
| Interrupted, timed out | Failed with the reason "unknown". Looking at a folder changes nothing on the machine, so nothing is left behind. |
| Removed | Failed with the reason "machine removed". |

An operation of a machine that stays away stays open until the machine returns and answers `expired`, or until the link's sweep settles the command.
A call for it returns `deadline_exceeded` each time. The command is never run late, because the client refuses a command past its expiry.

### What the client does with a folder

`LookAtFolder` accepts an absolute path, or a path that begins with `~` for the home folder.
The client resolves the path to the top folder of the git repository, with every link in the path followed.
It answers with these facts, or with one reason: not a folder, not a git repository, not readable, or path too long.

| Fact | Meaning |
|---|---|
| Resolved path | The top folder of the repository, without links. |
| Name | The last part of the resolved path. |
| Main branch | The branch that the remote calls its head, when the folder knows it. Else the branch that is checked out. Else `main`, for a repository without a commit. |
| Checked-out branch | The branch that is checked out, or none when the checkout is detached. |
| Remote | The address of the remote `origin`, without a user name or a password. |
| First commit | The identifier of the repository's first commit, or none. |
| Clean | Whether the working tree has no uncommitted change. |

The server stores the resolved path. One machine cannot hold the same resolved path twice, so two spellings of one folder give one row.

### The website in sprint 4

**With a real server that has `ProjectService`, the website shows no sample record below a machine.**
`GetServer` already tells the website whether the server holds only sample data, and which revision of the contract it speaks. Sprint 4 raises that revision.
With a real server of that revision, the website loads no sample repository, project, agent, thread, task, team or request, and its simulation does not run.
With the in-page sample server and with the mock server, the website stays as it is, so the sample remains the reference for the design of the screens.

- The tree of repositories follows `WatchRepositories`, in the way the Machines page follows `WatchClients`. The website waits for both before it shows the workspace.
- "Add repository" calls `InspectFolder` while the owner types, and fills its rows "Base branch", "Remote" and "Status" from the answer. It uses an answer only when the machine and the path in the dialog are still the ones that were asked.
- Its button calls `AddRepository`, and returns to the dialog that opened it with the stored repository selected and the goal kept.
- Both dialogs start on the machine that opened them, or on the first machine that is online. They no longer name a sample machine.
- Each call carries a request key that the website keeps for every retry of the same intent.
- A sign-out, an expired session and a change of server end every watch of this sprint and clear its records, as they clear the machines.

Three things on these screens change what the owner sees. They need the owner's approval before sprint 4 is built.

1. "Add repository" says today, for every folder, that the working tree is clean. With real data, a repository with uncommitted changes needs a second sentence.
2. "New project" cannot start an agent before sprint 5. In sprint 4 its button answers that projects are not connected to this server yet, in the way the views Files and Terminal already answer in the mode of the old engine.
3. The plus sign in the sidebar opens a dialog that follows a repository by its remote, with a sample search. With a real server it opens "Add repository" directly, until following by remote is built.

### How sprint 4 is proven done

Tests of the server run against a real PostgreSQL, and tests of the link use the real client against the real server in one process, as rule 10 demands.

| Check | What is observed |
|---|---|
| R1 | `InspectFolder` on a repository with uncommitted changes, a checked-out branch that is not the main branch, and a remote. The answer holds all of them, through the real link. No repository row and no folder row exist afterwards. |
| R2 | `AddRepository` stores the repository and its folder under the chosen machine, and the stored path is the resolved one, not the typed one. A folder that is no git repository stores nothing and returns the reason. |
| R3 | A link and a subfolder that resolve to one checkout are added at the same moment. One folder row exists, and both calls return it. |
| R4 | `AddRepository` with a known repository attaches a folder of another name on a second machine. A folder of another repository is refused, and nothing is stored. |
| R5 | The server restarts while a call waits. The same request key returns the result, and the client looked at the folder once. |
| R6 | The reply of `AddRepository` is lost after the server stored the row. The same request key returns the same repository, and one row exists. |
| R7 | The machine disconnects before it takes the command. The call returns `deadline_exceeded` after 25 seconds. The machine reconnects after the expiry, answers `expired`, and the operation is failed. The client never looked at the folder. |
| R8 | Two calls for one machine store their commands at the same moment, one of them slower to commit. Both commands reach the client. |
| R9 | A client restarts with events of its old process in its buffer, while a command waits. The server applies the old events first, and then sends the command on the same stream. |
| R10 | A client of revision 1 is refused with `link_revision_mismatch`. A client of revision 2 starts on a buffer and receipts that revision 1 wrote: the server applies the buffered event once, and a command that revision 1 completed is answered from its receipt and not run again. |
| R11 | On the deployed stack, with a real machine, the owner opens "Add repository" from the Machines page, reads the three rows, adds the repository, and sees it in the tree after a reload. No sample project is shown. |

## Sprint 5: the first agent

### What the owner does and sees

The owner opens "New project", writes a goal, picks a machine and a repository, and picks Claude Code.
The website opens the new project on "End result", with the lead's column at its side.
The column shows the goal, then the answer as it is written, then the lead as idle.
The owner sends another message, and the lead answers again.
The owner can cancel a running answer and can stop the lead.
After a reload of the page, the column is as it was.

The view "End result" itself says "No statements", because a lead of sprint 5 cannot write an end result yet.

### Link revision 3

Revision 3 adds the commands and events of the tables below to the two envelopes, with new field numbers, and their messages in a new file `agents.proto`. It adds `MakeRepository` and `RepositoryMade` to `folders.proto`.
It follows the steps of revision 2. A server of sprint 5 speaks revision 3 only.

### The parts

| Part | Folder | Contract file | What it owns |
|---|---|---|---|
| Projects, on the server | as in sprint 4 | `lgtm/v1/projects.proto` | It gains projects, and the reservation of a folder. |
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

`ProjectService` gains three calls.

| Call | What it does |
|---|---|
| `WatchProjects` | Sends every project, then each change. |
| `CreateRepository` | Takes a machine and a name. The client makes the repository. The call returns the repository and its folder. It is a folder operation, as in sprint 4. |
| `CreateProject` | Takes a goal, a machine, a repository, and the lead's harness, model, effort and permission level. It stores the project and its lead, starts the lead, and returns both identifiers. |

`AgentService`, in `agents.proto`:

| Call | What it does |
|---|---|
| `WatchAgents` | Sends every agent, then each change. |
| `WatchThread` | Takes an agent. Sends the newest items of its thread, then each new or changed item. |
| `WatchHarnessChoices` | Sends, for each machine and harness, what the harness offers there, then each change. |
| `SendMessage` | Takes an agent and a text. It stores the message as an item of the thread and sends it to the agent. |
| `CancelReply` | Takes an agent and a turn. It cancels that turn. |
| `StopAgent` | Takes an agent. It ends the harness process. |

Every `Watch` call is served through the shared `watch` folder. Every call that changes something carries a request key and uses the store's receipt, so a repeated call returns the first reply.

`CreateProject` is one write under the lock of the machine.
It stores the project and the reservation of the folder, calls the Agents part's Go interface to store the lead and the first item, stores the command `StartAgent`, and gives its identifier to the Agents part as the start.
Either all of it is stored or none. The reply holds the identifiers of the project and of the lead, so a repeated call returns the same two.

The server gives a project its name, from the first words of the goal.
A project of sprint 5 is in the stage "Define". Its folder is the folder of the repository, and its branch is the branch that is checked out there.

`WatchThread` first sends at most the newest 500 items and at most 2 mebibytes of text, and says whether older items exist. Sprint 6 adds pages for them.

### A folder holds one lead until the lead is proven gone

A lead of sprint 5 works in the main checkout. Two leads in one folder would write the same files.

`CreateProject` reserves the resolved folder for its lead, and refuses a folder that is reserved.
The reservation ends only when the server has proof that no process of that lead runs and none can start:

- the event `AgentEnded` or the event `AgentStartFailed` of that lead is applied;
- the command `StartAgent` of that lead is settled as expired or unsupported;
- the run of the machine changes, because a client that starts ends every process of its previous run before it opens the link;
- the owner removes the machine, and its folders with it.

A lead that is shown as failed can still hold its folder. The state of the lead says what the owner sees. The reservation says what may run.

### An agent, as the website receives it

| Field | Meaning |
|---|---|
| Identifier, project, machine, role, name | The role of sprint 5 is always project lead. |
| Harness, model, effort | What the session reported. Before the session reports, what the owner chose, or nothing for the harness's default. |
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
| `note` | One sentence with a fixed kind, such as a cancelled reply, a refused request, no usage left, or a stopped agent. |

Each item has a number that rises within its thread, the turn it belongs to, and the time it began.
The server gives the number. An item of the runner gets its number when the server applies the first event that names it.
Each way a turn can end, other than finishing, leaves a note, so the reason stays in the thread.

### The states of an agent

| From | What happens | To |
|---|---|---|
| none | `CreateProject` stores the agent and the command `StartAgent`. | `starting` |
| `starting` | The event `AgentStarted` of this start is applied. | `working`, with turn 1 active |
| `starting` | The event `AgentStartFailed` of this start is applied. | `failed` |
| `starting` | The command `StartAgent` is settled. See its table. | `failed`, or no change |
| `starting` | Eight minutes pass since the project was created. | `failed`, with the reason "start timed out", and a stop is asked |
| `working` | The event `TurnEnded` of the active turn is applied. | `idle` |
| `idle` | `SendMessage` stores the item and the command `Prompt`. | `working`, with that turn active |
| `working` | The command `Prompt` of the active turn is settled as failed, expired or unsupported. | `idle`, with a note that the message did not reach the agent |
| `starting`, `working`, `idle` | The event `AgentEnded` is applied. | `stopped` when a stop was asked, else `failed` |
| `starting`, `working`, `idle` | The run of the machine changes, and the agent belongs to an older run or to none. | `failed`, with the reason "client restarted" |
| any but `stopped` | The owner removes the machine. | `failed`, with the reason "machine removed" |

`stopped` and `failed` are final in sprint 5. No report and no outcome moves an agent out of them.
Output for a final agent is still stored, because it was written before the end.

The server changes the state in the same write that stores the command, applies the report, or settles the command.
A sweep of the Agents part applies the rule of eight minutes every 30 seconds. It reads the agents from the database each time.
A machine that goes offline does not change the state of its agents. The website already shows that the machine is offline.

An agent belongs to the run whose stream brought its `AgentStarted`.
The run of a machine changes only after every old event is applied, so a `starting` agent whose start the old process finished is `working` by then, and is failed with the others.
A `starting` agent whose command the old process never received stays `starting`, and the new process starts it.

`SendMessage` is refused while a turn runs, while the agent starts, while a stop is asked, when the agent is final, and while the machine is not connected.
`CancelReply` is refused while the machine is not connected.
`StopAgent` is accepted at any time before the agent is final, also while the machine is away.

### Commands of sprint 5 and what each outcome does

Every command goes through the link's stored commands, as rule 6 demands.

**`StartAgent`** carries the agent, the harness, the folder, the permission level, the model, the effort, and the goal. It expires after 5 minutes.

| Outcome | What it proves and what the server does |
|---|---|
| Done | The session is open and set, the goal is written to the harness, and `AgentStarted` was applied before. Nothing more. |
| Failed | No process of this start is left. `AgentStartFailed` was applied before. An agent that is still `starting` is failed with the reason "other". The reservation ends. |
| Expired, unsupported | The start never ran. The agent is failed with "start not delivered". The reservation ends. |
| Interrupted | The client process died during the start. The run has changed by then, and the agent is failed. Nothing more. |
| Timed out | The machine stayed away. The agent was failed by the rule of eight minutes. The reservation stays. |

`StartAgent` carries the goal, so that the start and the first turn cannot arrive in the wrong order.
The reply of ACP to a prompt arrives when the turn ends. The start therefore ends when the goal is written to the harness, and the end of the turn is an event of its own.

**`Prompt`** carries the agent, the turn, and the text. It expires after 2 minutes. The server sends it only to an idle agent, so one agent never has two prompts on the way.

| Outcome | What it proves and what the server does |
|---|---|
| Done | The text is written to the harness. Nothing more. The end of the turn is an event. |
| Failed | Not one byte of the text was written. The agent is `idle` again, with a note. |
| Expired, unsupported | The same. |
| Interrupted | The client process died. The run has changed by then, and the agent is failed. Nothing more. |
| Timed out | The machine stayed away. The turn stays active, and the agent stays as it is until the machine returns or is removed. |

A write to the harness that fails half way leaves the session unusable. The runner then ends the process and reports `AgentEnded`, and answers the command as done.

**`CancelTurn`** carries the agent and the turn. It expires after 2 minutes.

| Outcome | What it proves and what the server does |
|---|---|
| Done | The harness was told to cancel that turn, or the runner withheld a prompt of that turn that it had not yet written, or that turn had ended. The end of the turn is an event. |
| Failed, expired, unsupported, timed out | The cancellation did not reach the harness. The thread gets a note that says so. The turn stays active. |
| Interrupted | The run has changed by then. Nothing more. |

**`StopAgent`** carries the agent. It expires after 2 minutes.

| Outcome | What it proves and what the server does |
|---|---|
| Done | No process of the agent is left and none can start. `AgentEnded` was applied before. |
| Any other | The stop is still asked. Nothing more. |

A stop that is asked stays asked until `AgentEnded` is applied or the run changes.
Each time a stream of the machine is admitted while a stop is asked, the server stores the command again.
The server also asks a stop itself when it fails a `starting` agent by the rule of eight minutes, so that a start that arrives late is ended.

**`DescribeHarness`** carries a harness. It expires after 5 minutes. Done means that the event `HarnessDescribed` is in the buffer. Any other outcome leaves the stored description as it was.

**`MakeRepository`** carries a name. It expires after 20 seconds, and its operation follows the table of sprint 4, with one difference: after `interrupted` or `timed out` the folder may exist.
The call then says that the outcome is unknown, and the owner adds the folder with "Add repository" if it is there.

### How the runner keeps order for one agent

The runner handles the commands of one agent one after another, in the order in which they arrive.

- A stop that arrives while a start still runs ends the start, and no process is left.
- A cancellation of a turn whose prompt is not yet written withholds the prompt, and the turn ends as cancelled.
- A client that gets `StopAgent` for an agent it does not run remembers the agent on disk, refuses a later `StartAgent` for it, and reports `AgentEnded`.
- After a cancellation is sent to the harness, the runner waits 10 seconds for the turn to end. If it does not end, the runner ends the process, and the agent is `failed` with the reason "cancel ignored".

All reports of one agent leave through one ordered sender.
The runner holds every report of the first turn until `AgentStarted` has its number in the buffer.
It sends everything it holds for a turn before it reports the end of that turn, and everything it holds for an agent before it reports the end of that agent.
It puts a report into the buffer before it answers the command that the report belongs to.

### Events of sprint 5

| Event | What it carries |
|---|---|
| `RepositoryMade` | The operation, the command, and the facts of the new folder, or a reason. |
| `HarnessDescribed` | The command it answers, the harness, the adapter's version, whether the harness is signed in, the modes, and the models, each with the efforts it offers. |
| `AgentStarted` | The start, the agent, the session's identifier, and the model, effort and mode that the session reports. |
| `AgentStartFailed` | The start, the agent, and a reason from the fixed list. |
| `ThreadOutput` | The agent, the turn, the item, its kind, the position where the new text begins, and the new text; or the revision and the status of a tool call. |
| `TurnEnded` | The agent, the turn, and how it ended: finished, cancelled, refused, no usage left, signed out, or failed. |
| `AgentEnded` | The agent, and a reason from the fixed list. |

Events are numbered and acknowledged by the link of wave 1, so each is applied once and in order.
The server stores a description of a harness only when it answers the newest `DescribeHarness` for that machine and harness.
The Agents part and the Projects part register for the reports of their kinds. A test of each sprint proves that a report changed the stored record, not only that it was acknowledged.

### How output travels

- The runner gives each item an identifier of its own.
- A position is a count of bytes of the text after the filter. An item begins at position zero.
- The runner gathers the text of an item for 300 milliseconds before it sends it. It cuts only between two characters.
- An item of text is closed before a character would take it past 32 kibibytes, and the text continues in a new item with a new identifier.
- A tool call carries a revision that rises with each change. The server keeps the highest.

The position is a check, not the mechanism. The link already applies each event once.
A piece that lies wholly inside the stored text and equals it is ignored.
Any other piece whose position is not the length of the stored text is not applied, and the server adds the note "Some output of the agent is missing here."

The website receives the whole item on each change, as the shared `Watch` shape demands.

### What sprint 5 guarantees about output, and what it does not

- The link never waits for the runner. It keeps sending, acknowledging and reconnecting while the runner waits for room in the buffer.
- When the buffer has no room, the runner stops reading the output of the harness until there is room. It keeps at most 64 kibibytes of each agent in memory meanwhile. Text that the runner has read is never dropped while the client lives.
- A harness whose output is not read stands still. So does a permission request behind that output. This lasts as long as the server is away and the buffer is full.
- A stop ends the process at once, whatever the buffer holds. Text that the runner had read is sent before `AgentEnded`. Text that the harness had written and the runner had not read is lost, and the note of the stop says that output may be missing.
- A client that dies loses the text that was not yet in the buffer. The agent is failed by the change of run, and the thread says "client restarted".

Sprint 8 looks at these limits again.

### The runner and Claude Code

**The adapter.** Claude Code speaks ACP through an adapter program.
The public registry of ACP lists the adapter as one self-contained program for Linux and for macOS.
The client's source names one version of the adapter and the SHA-256 hash of its program for each platform.
`DescribeHarness` downloads the adapter into the client's state folder when it is absent, and checks the hash. A program whose hash differs is never started.
`StartAgent` does not download. Without the adapter it fails with the reason "adapter missing".

The plan of the runner checks one fact before any code is written: whether the self-contained adapter works with the Claude Code that is installed and signed in on a machine, when the client runs as a service.
If it does not, the client runs the adapter from its package instead, and the machine then needs Node.

**Describing the harness.** For `DescribeHarness` the runner opens a session, reads its modes and models, selects each model in turn to read its efforts, and closes the session. It sends no prompt.
A harness that answers that it needs a sign-in is reported as signed out.
The server sends `DescribeHarness` after each scan that finds Claude Code. The owner's existing action "Scan harnesses" on the Machines page therefore refreshes the description, for example after a sign-in on the machine.

**One process for one agent, under a guard.** The runner starts each adapter under a small guard, which is the client's own program in another role.
The guard leads a process group of its own, starts the adapter in it, and holds one end of a pipe to the client.
When the pipe closes, because the client stopped or died, the guard ends its whole process group.
A harness process therefore does not outlive its client, on Linux and on macOS, at any moment of the start.
A process that leaves the group on purpose is not covered. This is recorded in `docs/issues.md`.

The adapter runs in the folder of the repository, with a short list of environment variables: the search path, the home folder, the user, the shell, the language settings, the terminal type, the temporary folder and the time zone.

The runner then opens one ACP session, sets the mode, the model and the effort, and writes the goal.
It allows 120 seconds from the start of the process to the written goal.
A value that the session does not offer is a failed start with the reason "choice refused". A model or an effort that the owner did not choose is left to the harness.

The agent uses the sign-in that Claude Code already has for that user on that machine. The server stores no sign-in before sprint 9.

**The five permission levels on Claude Code.**

| Level | Mode of Claude Code |
|---|---|
| Read only | `plan` |
| Ask before every change | `default` |
| Edit files without asking | `acceptEdits` |
| Automatic review | `auto` |
| Full access | `bypassPermissions` |

A new agent gets the level that the dialog sends: the owner's saved choice for a lead, or "Automatic review" without one.
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

### The website in sprint 5

The data layer of the website gains `WatchProjects`, `WatchAgents`, `WatchHarnessChoices`, and one `WatchThread` for the lead whose column is shown.
The lead's column reads the browser's store, so the data layer writes the thread there.

A project is shown only when its repository and its lead are in the store.
The website waits for machines, repositories, projects and agents before it shows the workspace.
After `CreateProject` returns, the dialog keeps its state "Creating…" until the project and its lead have arrived, and opens the project then.
A reload of a project's address waits in the same way.
A sign-out, an expired session and a change of server end every watch, and clear the records, the threads, the choices and the drafts of actions that wait.

**"New project".**

- Its lead choice wants a subscription. The server has none before sprint 9. The data layer derives one entry for each machine and harness that is described, with an identifier made of both, and the name of the harness. It stands for the sign-in that Claude Code has on that machine.
- An entry is usable when the description says "signed in". When it says "signed out", or when no description exists yet, the entry is not usable, and the dialog's existing sentence asks for a signed-in subscription.
- Its lists of models, and of efforts for the chosen model, come from `WatchHarnessChoices`, each entry with an identifier and a label. "Harness default" sends no value.
- The permission level is the owner's saved choice for a lead, or "Automatic review". The dialog has no control for it, as today.
- Its button calls `CreateRepository` for a new name, then `CreateProject`. Each call keeps its request key over retries, and the second keeps the repository that the first returned. A failed call leaves the dialog and its fields as they were.

**The lead's column.**

| Control | In sprint 5 |
|---|---|
| The message box, Send, Enter | Calls `SendMessage`. A refused message stays in the box, and the refusal is shown as the website shows other failed actions. |
| "Stop this reply" | Shown while the lead has an active turn. Calls `CancelReply` with that turn. |
| "Stop agent" | Shown where it is shown today. Calls `StopAgent`. While a stop is asked, the lead's line says "Stopping" and the control is off. |
| The context percentage | Shows a dash. No value exists before sprint 6. |
| The menus Subscription, Model and Effort | Each shows the current value as its only entry. Choosing it does nothing. A change comes in sprint 9. |
| "Context", and typing an at sign | Opens the picker, which finds nothing, because a project of sprint 5 has no tasks. |
| Attaching, dropping and pasting a file | Refused before anything is attached, with the sentence that the mode of the old engine already uses. |
| "Paste in terminal", under a shell command in an answer | Not shown. The command is shown as text. Terminals come after sprint 10. |
| "Resume", on a stopped or failed lead | Answers that it is not connected yet. It comes in sprint 6. |
| "Sign in on" a machine, on a failed lead | Answers that it is not connected yet. It comes in sprint 9. |

A lead that failed before its start is shown by the existing failed composer, with the sentence of its reason.
A lead that failed after its start is shown by the existing stopped composer, with the sentence of its reason as its line.
The counters of turns and tokens, which the sample invents, are not shown for a real lead.

**The other views of a real project.**
The tabs Questions, Status, Actions, Design, Files, Git and Settings stay.
Each shows its existing empty state where it has one that is true, such as "No questions".
A view that would show sample data, or would let the owner change a record that the server does not hold, shows the existing "not connected" page.
The plan of the website lists every view with the one or the other, and checks the lead's column on a narrow screen. The owner sees that list before the build.

Four things in sprint 5 change what the owner sees, and need the owner's approval before the build:

1. "Resume" and "Sign in on" answer that they are not connected yet.
2. The stopped composer says today that "Resume continues from its last turn". That is true from sprint 6.
3. "Paste in terminal" is not shown.
4. Some views of a real project show "not connected".

### What the removal of a machine does until sprint 10

The owner can remove a machine today. From sprint 5 a removal has more to end.

- Every agent of the machine that is not `stopped` becomes `failed`, with the reason "machine removed".
- Every open folder operation of the machine is failed with the same reason.
- The folders of the machine are removed from their repositories, and the reservations with them.
- A project of that machine stays, with its failed lead and its thread, and cannot run. Sprint 10 gives it a new start on another machine.

### How the server keeps its data

| Table | Part | What a row is |
|---|---|---|
| `repositories` | Projects | A repository: name, main branch, remote, first commit. |
| `repository_folders` | Projects | The resolved folder of one repository on one machine, and the lead that holds it. |
| `folder_operations` | Projects | One operation: its request key, its command, its state and its result. |
| `projects` | Projects | A project: repository, folder, machine, name, goal, lead. |
| `harness_choices` | Agents | What one harness offers on one machine, and the newest command that asked. |
| `agents` | Agents | An agent: its state, its start, its active turn, its run, its session, and whether a stop is asked. |
| `thread_items` | Agents | One item of one thread, with the runner's identifier of it. |

Each write names what changed, so that the change feed serves the `Watch` calls.
The Agents part ends a reservation by calling the Projects part's Go interface, in the write that holds the proof.

### Refusals and their codes

Rule 8 demands a Connect code and one sentence for each error.

| Refusal | Code |
|---|---|
| The machine is not connected. | `unavailable` |
| The folder is not a folder, not a git repository, not readable, or not the same repository. | `failed_precondition` |
| The folder belongs to another repository, or holds a lead. | `already_exists` |
| A name, a path or a text breaks a limit or a rule. | `invalid_argument` |
| A turn runs, the agent starts, a stop is asked, or the agent is final. | `failed_precondition` |
| The machine did not answer in time, or the outcome is unknown. | `deadline_exceeded` |
| The same request key arrives with another request. | The code that the store uses today. |
| An agent, a project or a repository is unknown. | `not_found` |

### How sprint 5 is proven done

Sprint 5 keeps the fixtures of sprint 4: a real PostgreSQL, and the real client against the real server in one process.
Its tests use a stand-in for the harness: a small Go program that speaks the agent's side of ACP and follows a script.
It can write slowly, answer at once, ask for a permission, start a child process, ignore a cancellation, end by itself, and write a planted secret.

| Check | What is observed |
|---|---|
| A1 | `CreateProject` with a goal that no other test uses. The stand-in receives that exact goal once, in the chosen folder, in the chosen mode, with the chosen model and effort. A watch of the thread receives the goal, then at least two growth steps of the answer, then the lead as `idle`. |
| A2 | A stand-in that answers and ends its turn the moment it reads the goal. The lead is `idle` afterwards, with the whole answer. |
| A3 | A second message to the idle lead is answered. A message during a turn is refused, the stand-in received exactly the two accepted messages, and the thread holds exactly those two. The same request key after the turn returns the first reply and starts no turn. |
| A4 | `CancelReply` ends the named turn at the stand-in, the thread holds a note, and the next message runs in the same session. A cancellation of the first turn that arrives while the second runs changes nothing. A cancellation that arrives before the prompt is written withholds the prompt. |
| A5 | A stand-in that ignores a cancellation is ended after 10 seconds with its child. The lead is `failed` with the reason, and the folder is free only after both processes are gone. |
| A6 | `StopAgent` ends the stand-in and a child that it started. The lead is `stopped` only after both are gone. A stop during the start leaves no process. A stop asked while the machine is away takes effect when it returns. |
| A7 | The server restarts after it stored a piece of an answer whose acknowledgement the client did not get. Client and stand-in stay alive. A new watch shows the same items with the exact text once. |
| A8 | The client is killed without warning while the stand-in and its child run, and once more in the middle of a start. Both times every process of the agent is gone within seconds, before the client starts again. After the restart the lead is `failed` with "client restarted", and its folder is free. |
| A9 | The machine goes away right after the project is created, and stays away. The lead is `starting`, and after eight minutes `failed` with "start timed out". The folder stays reserved, and a second project for it is refused. The machine returns, answers `expired`, and the folder is free. |
| A10 | As A9, but the start ran and its report waits in the buffer when the link drops. After the machine returns, the late report does not revive the lead, the stop that the server asked ends the process, and only then is the folder free. |
| A11 | A permission request of the stand-in gets the refusing option, and a cancellation when no such option is offered. The thread holds the note. The mode was set before the goal was written. A project created without a chosen level runs in the mode `auto`. |
| A12 | An adapter whose hash differs is not run: a marker that it would write is absent. An adapter with the right hash runs, started as the client's service starts it. |
| A13 | The stand-in writes the client's token, cut across two pieces, and once in the title of a tool call, and a value planted in a stand-in for the sign-in file of the harness. A watch of the thread, the stored items and the logs of client and server hold none of them. |
| A14 | The server is away while the stand-in writes more than a small buffer holds. After it returns, the thread holds every byte that the stand-in wrote, in order. A stop asked then ends the process, and the thread ends with the note of the stop. |
| A15 | `CreateRepository` makes a repository whose main branch is `main`. A name that leaves the folder for repositories, and a name that is a link out of it, are refused and make nothing. |
| A16 | On the deployed stack, with a real machine and the real Claude Code, the owner creates a project with "New project" for a repository that was added in the browser. The lead's column shows the goal and a growing answer of the real session. The owner sends a second message, cancels a reply, reloads the same address and sees the same content, and stops the lead. |

## Open questions

1. Does the self-contained adapter work with the installed Claude Code and its sign-in, when the client runs as a service? The plan of the runner checks it first.
2. Sprint 6 must settle how a message steers a running turn. Claude Code and Codex offer a steering call of their own beside the ordinary prompt. Cursor offers none, and a second prompt cancels its running turn.
3. The foundation design asks whether Claude Code applies its sandbox when it is driven through ACP. The design of the secrets of tools answers it.

None of them blocks the contract of sprint 4.

## What waits for the owner

1. Whether sprints 4 and 5 are built as two sprints or as one.
2. The three visible changes of sprint 4 and the four of sprint 5, each listed in its section.
