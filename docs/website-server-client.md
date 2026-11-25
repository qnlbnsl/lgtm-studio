# Splitting LGTM Studio into website, server, and client

This is a proposed refactor, not a description of implemented behavior.
The goal is to make three parts independently runnable while retaining a
simple local setup. The code can stay in one repository with shared types.

## What each part owns

| Part | Responsibilities |
|---|---|
| **Website** | Displays projects, teams, conversations, approvals, machines, and usage. Sends user actions to the server and subscribes to updates. Its server address is configurable, so it can be hosted independently or served locally. |
| **Server** | Exposes the application programming interface (API). Owns the board, project and team state, authorization, scheduling, client health, notifications, and usage aggregation. Persists accepted work and results in a database. Delegates machine operations to clients. |
| **Client** | Runs on each computer that performs work. Starts and supervises harnesses through the Agent Client Protocol (ACP). Owns local repositories, worktrees, Git operations, files, terminals, installed adapters, and local credential access. Reports capabilities, capacity, progress, and usage to the server. |

Here, **client** means the installed execution program. The existing
`@lgtm/client` package is the API library; its name does not define the new
program's package boundary.

## How the parts connect

```text
Website <-- Effect remote procedure calls (RPC) --> Server <--> Database
                                                    ^
                                                    |
                                       Outbound client connections
                                                    |
                                          Clients on computers
                                                    |
                                                   ACP
                                                    |
                                        Harnesses and local worktrees
```

Keep Effect RPC for the website API. Define a separate client connection
contract for registration, assignments, acknowledgements, events, and
health reports. That contract can also use Effect RPC.

Each client opens an authenticated connection to the server and receives
commands over it. The server does not need an inbound address for each
computer. The website connects to the server rather than directly to a
client. Agent calls to the board also reach the server through the client,
preserving the agent's identity and permission limits.

Clients retain local path and permission checks. Credentials stay out of
website responses, board messages, and logs. Client enrollment and authentication
need a design that preserves the existing security rules.

## How scheduling and reporting work

The server first finds eligible clients: connected computers with permission,
the required harness and account, the right repository state, and free capacity.
It can then use round-robin among those clients, reserving capacity atomically.
An active session remains assigned to its client.

Start by distributing independent projects across computers. Spreading agents
from one project across computers also requires a defined way to transfer
commits and checkpoints. Local paths and unfinished files are not shared
automatically. A provider account used on several computers still has one
shared usage limit.

Clients report usage with the account, model, project, execution attempt, and
measurement source. The server deduplicates reports. Unknown usage stays unknown.
Calculated token costs remain distinct from provider-reported charges.

## What survives a disconnection

The server stores a command before sending it. Clients retain command receipts
and buffer bounded progress events until acknowledged. Stable identifiers make
retries safe against duplicate launches and duplicate usage reports.

Heartbeats detect lost contact. The server stops assigning new work to an
unavailable client and records an alert. Reconnection reconciles active work
before retrying it. A lost connection does not prove a harness stopped.

Recovery on the same computer can use supported harness session restoration.
Recovery elsewhere additionally needs repository checkpoints, credentials,
and portable session state. Assignments need an expiry and an attempt identifier
so old results cannot overwrite replacement work. Commands with uncertain
external effects must not be retried blindly.

## How to introduce the split

1. Separate machine operations from coordination behind internal interfaces.
   Keep existing local behavior working during extraction.
2. Run one server and one client as separate processes on the same computer.
   Verify prompts, cancellation, approvals, Git, files, terminals, and board calls.
3. Connect a second computer and verify explicit placement, capacity limits,
   and round-robin scheduling among eligible clients.
4. Introduce central database persistence and verify command receipts, event
   replay, usage aggregation, server restarts, and client reconnection.
5. Run the website independently and verify sign-in, API access, and live updates.

A local launcher should still start all three parts with one command.
Start with one always-on server. PostgreSQL is a proposed central database;
SQLite can remain useful locally. Database migration needs its own design.
Multiple server instances are a later change: a shared database alone does not
coordinate their locks, scheduling decisions, or event subscriptions.

## Where extraction starts in the existing code

Confirmed live through source inspection on 2026-11-20 at revision `69497a40`:

- [`service.ts`](../packages/engine/src/service.ts) composes persistence,
  harness execution, recovery, and website serving in one service.
- [`Runner.ts`](../packages/engine/src/runner/Runner.ts) supplies an execution
  interface to build on. Its local callbacks and services need explicit wire types.
- [`gitApi.ts`](../packages/engine/src/project/gitApi.ts) still restricts machine
  operations to `local`; those operations belong on the installed client.
- [`connection.ts`](../packages/studio/src/connection.ts) derives the API address
  from the website host. Independent hosting needs endpoint configuration and
  deliberate session and origin handling.
- [`EventLog.ts`](../packages/engine/src/log/EventLog.ts) uses a process-local
  writer lock and event publication. These need redesign before server replication.

The earlier client-mode direction is recorded in
[`features.md`, section 9](features.md#9-client-mode).
