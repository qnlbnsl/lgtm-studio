# Rewriting LGTM Studio in Go: the three parts and the first two build waves

This document is a design.
It records the decisions of the rewrite, and it designs the first two build waves in enough detail to build them.
Nothing in it is built yet.
The later waves get designs of their own, and each of them follows the rules in this one.

## What is being rewritten, and what stays

The engine in `packages/` is written in TypeScript on the Effect library.
It is one process on one machine.
It is rewritten from scratch in Go, as two programs: a server and a client.

The website in `src/frontend/` stays as it is.
It is a static build in Preact, Vite and TypeScript, and it reaches a server at an address.

The three parts meet only through the contract in `src/contracts/`, which is written in Protocol Buffers.

The code in `packages/` is not edited during the rewrite.
It stays as a reference until the Go programs replace it, and then it is removed.

## Decisions of the owner

| Date | Decision |
|---|---|
| 2026-11-23 | The product is three parts: a website, a server and a client. They meet only through the contract. |
| 2026-11-24 | The server and the client are written in Go. The engine is rewritten, not ported line by line. |
| 2026-11-24 | Version one runs one server node. The database layer is built as if the database were replicated. |
| 2026-11-24 | The database is PostgreSQL from the start. A compose stack starts it. |
| 2026-11-24 | Tables hold the current state. One ordered change feed sits beside them. The server does not replay history. |
| 2026-11-25 | A message to a running agent steers its turn, whether the owner or another agent sent it. |
| 2026-11-25 | A machine proves itself with a key pair. The join token only authorizes the exchange of keys. |
| 2026-11-25 | Version one drives every harness through the Agent Client Protocol (ACP) only. The software kits of single harnesses wait for version two. |
| 2026-11-25 | Secrets live in a vault that the server holds. An agent uses a tool that needs a key without ever reading the key. See "Three rulings that later waves carry". |
| 2026-11-25 | The vault is OpenBao. It is a service of the compose stack, beside PostgreSQL, and it needs no machine of its own. |
| 2026-11-25 | All agents have one source for their memory files, held by the server, so that accounts and machines share it. |
| 2026-11-25 | Retries of a refused duty back off. After three refusals an investigation agent finds the cause and reports to the project lead. |

## The three parts and how they connect

```text
Website  --contract calls-->  Web server of the stack  -->  Server  <-->  PostgreSQL
(static)                      (same address)                  ^
                                                              |  one open two-way connection,
                                                              |  dialled by the machine
                                                           Client, one on each machine
                                                              |
                                                             ACP
                                                              |
                                                  Harnesses, and a clone for each agent
```

- **The website** shows everything and sends the owner's actions to the server.
- **The server** holds all state in PostgreSQL, answers the website, and tells machines what to do. It runs no agent and holds no repository.
- **The client** is the program installed on each machine. It runs the harnesses, holds a clone for each agent, and removes secrets from everything before it leaves the machine.

In this document, a machine is a computer that runs the client.
The contract calls it a client, and the website calls it a machine.

## Where the code lives

One Go module sits at the repository root, with the module path `github.com/qnlbnsl/lgtm-studio`.
It needs Go 1.26 or newer.

| Folder | What it holds |
|---|---|
| `src/contracts/proto/lgtm/v1/` | The calls between the website and the server. |
| `src/contracts/proto/lgtm/link/v1/` | What the server and a machine say to each other. |
| `src/contracts/gen/` | Generated TypeScript, as today, and generated Go beside it. |
| `src/server/cmd/lgtm-server/` | The start of the server program. |
| `src/server/internal/<part>/` | One folder for each part of the server. |
| `src/client/cmd/lgtm-client/` | The start of the client program. |
| `src/client/internal/<part>/` | One folder for each part of the client. |
| `deploy/` | The compose stack, the two images and the client installer. |

Each program keeps its code under an `internal` folder.
Go then refuses an import from the server into the client, and the other way round.
The two programs share only the generated contract code.

## The build waves

| Wave | What is built | What it needs |
|---|---|---|
| 0 | The shared `Watch` shape, the link contract, and Go generation. | Nothing. |
| 1 | The server's foundation, the machines and the link on both ends, and the compose stack. | Wave 0. |
| 2 | The other parts of the server and of the client, side by side. | Wave 1. |
| 3 | The website's records move from sample data to the server, one record at a time. | The service of each record. |

This document designs waves 0 and 1.
Wave 1 is three units that can be built side by side once wave 0 is merged:

1. The database layer, sign-in and request handling.
2. The machine registry and the link, on the server and on the client.
3. The compose stack.

## The database layer

The folder is `src/server/internal/store/`, with `src/server/internal/watch/` beside it.
Every other part of the server stands on it.

### What it means to build it as if the database were replicated

1. Everything durable lives in the database. The server writes no state file.
2. The database decides the order of changes. No lock in the server's memory does.
3. The server keeps no state in memory that it cannot read again from the database.
4. Every request that changes something can be sent twice and happens once.
5. Identifiers are unique on every node. They are version 7 universally unique identifiers (UUIDs), made in Go.
6. Nothing stored names the server's own disk or processes. Paths and process identifiers belong to a machine.

### Tables and the change feed

Each part owns its tables.
A table holds the current state of its records.
History that matters, such as the items of a conversation, is a table that only grows.

One table named `feed` holds the change feed.

| Column | Meaning |
|---|---|
| `position` | A number that only grows. It orders every change on the server. |
| `topic` | The kind of record, such as `machines`. |
| `scope` | The project the record belongs to, or empty for a record of the whole server. |
| `key` | The identifier of the record. |
| `change` | Saved or removed. |
| `written_at` | When the change was committed. |

A feed row names a record. It does not carry the record.
A reader that follows the feed reads the record again when it needs it.

### Positions must follow commit order

A plain database sequence does not give commit order.
A check against PostgreSQL 16 on 2026-11-25 showed it: one write took position 2, another took position 3 and committed first, and a reader that had reached 3 would never have seen 2.

The store closes that gap in one place.
A part never inserts into `feed` itself.
It tells the store's transaction what changed, and the store writes the feed rows as the last step before the commit, under one transaction-level advisory lock.
The lock is held only from that step to the commit.
Positions therefore follow commit order, on the node that writes and on any replica.

### Following the feed

One follower runs in each server process.
It listens for a notification that the store sends with each commit, and it also reads the feed every two seconds.
The second path matters because a notification can be missed, and because a replica does not pass notifications on.

Rows older than one day are removed from the feed.
No reader needs them, because every `Watch` call starts with everything.

### How every `Watch` call works

The folder `watch` serves every `Watch` call of the contract the same way.

1. It subscribes to the follower.
2. It reads everything the caller asked for, and the highest feed position, in one transaction.
3. It sends everything as the first message.
4. It then sends one message for each later feed row of that topic and scope: the record as it is now, or the identifier of a record that is gone.

A caller that falls 2,048 changes behind is cut off, and the website opens the call again.

Every `Watch` call uses one message shape, in `common.proto`: everything first, then one change at a time.
`WatchMap` and `WatchPreviews` send the whole record or the whole list on each change today.
They change to the shared shape when their services are built, and that raises the contract revision.

### Safe retries

A call that changes something may carry a request key in the header `Lgtm-Request-Key`.
The store records the key, a hash of the request and the reply, in the same transaction as the change.
The same key with the same request returns the recorded reply.
The same key with a different request is refused.
Receipts older than one day are removed.

### Database changes, numbered by part

Each part keeps its numbered database changes in its own folder, `src/server/internal/<part>/migrations/`.
The store applies them at start-up and records, for each part, the last number applied.
Two builders working side by side therefore never share a number.
A database change is never edited after it is merged. A correction is a new numbered change.

### How it is tested

Tests run against a real PostgreSQL, named by the variable `LGTM_TEST_POSTGRES`.
Each test package works in a schema of its own, which it creates and drops.
No test replaces the database with a stand-in.

## Sign-in and web sessions

The folder is `src/server/internal/auth/`.
It carries over how the old engine handles sign-in, with one change.

- The server reads the password from the variable `LGTM_WEB_PASSWORD`. It does not start without it.
- The password is checked in constant time. It travels in the request body only.
- Up to eight sessions exist at once, and each lasts 30 days. A ninth sign-in removes the oldest.
- A session is a random 256-bit token in a cookie with `HttpOnly` and `SameSite=Strict`. The cookie is `Secure` when the public address is `https`.
- The server stores only a verifier of each token, a creation time, a salt and a binding to the password. The binding uses scrypt with N=65536, r=8 and p=1, as the old engine does.
- Changing or removing the password ends every session before the server opens its port.
- The server allows a website on another origin only when that origin is listed. It refuses a page that is neither HTTPS nor loopback.

**The change:** the old engine keeps sessions in the file `web-session.json`, and much of its care goes into writing that file safely.
Here the sessions are rows of a table, because nothing durable may live outside the database.
The transaction of the database replaces the temporary file, the rename and the flush.

### Secrets live in OpenBao

The server holds a few secrets of its own: its signing key for the link, and later the tokens of hosting accounts and notification carriers.
The owner decided on 2026-11-25 that the vault is OpenBao, as a service of the compose stack beside PostgreSQL.
The server's own tables hold no secret.

- The folder `src/server/internal/vault/` is the only code that speaks to OpenBao. Every other part asks it for a secret by name.
- OpenBao keeps its data in the same PostgreSQL, in a database of its own. Nothing durable is then added outside the database.
- OpenBao unseals itself at start-up from a key file that the installer makes once. No one types a key after a restart.
- A set-up step of the stack starts OpenBao the first time. It gives the server a credential that reaches only the server's own paths.
- OpenBao is reachable only inside the stack.

A copy of the database alone is useless for posing as the server, because every secret in it is encrypted under that key file.
A person who wants OpenBao on a machine of its own points the server at its address. Nothing requires that.

## Serving the contract

The folder is `src/server/internal/api/`.

- The server answers the Connect protocol, so a browser reaches it with `fetch`, and the same handlers answer gRPC.
- Every call needs a session, except `SessionService.GetServer` and `SessionService.SignIn`. A call without one gets the code `unauthenticated`.
- An error is a Connect code with one sentence the website can show.
- `GetServer` returns the contract revision the server speaks. The website already refuses a server of another revision.
- A service that is not built yet answers `unimplemented`.

Wave 1 answers two services for the website: `SessionService` and `ClientService`.
Both exist in the contract today, so the website's sign-in and its Machines page work against the Go server without a change to a screen.

## Joining: a token that authorizes an exchange of keys

The folders are `src/server/internal/machines/` and `src/client/internal/enroll/`.

1. The owner makes a join token on the Machines page, or with the server's own command. The token is used once and expires after 15 minutes.
2. The token carries the fingerprint of the server's public key. The machine therefore knows which server it is joining.
3. On the new machine, `lgtm-client join <address> <token>` makes an Ed25519 key pair. The private key stays in a file that only its user can read, and it never leaves the machine.
4. The client calls `Join` with the token and its public key. The server answers with its own public key, which the client checks against the fingerprint and keeps.
5. The machine now waits for approval. It can connect, and it can do nothing else.
6. The owner approves the machine under a name. From then on its public key is its identity.

The server stores a verifier of each token and the public key of each machine.
It stores no secret of any machine.
Removing a machine forgets its key.

## The link between the server and a machine

The folders are `src/server/internal/link/` and `src/client/internal/link/`.
The contract is `src/contracts/proto/lgtm/link/v1/`.

### One connection, dialled by the machine

A machine dials the server and keeps one two-way stream open.
The server never dials a machine, so a machine needs no address the server can reach.
A new connection from a machine replaces its older one.

A check on 2026-11-25 confirmed that a two-way Connect stream works in Go over HTTP/2.

### Both ends prove themselves on every connection

1. The client sends its identifier and a random value.
2. The server signs that value with its key and sends a random value of its own.
3. The client checks the signature against the server key it kept, and signs the server's value with its private key.
4. The server checks that signature against the public key the machine registered.

The server then sends a short-lived token for this connection.
Later wave-2 calls beside the stream, such as a byte stream for a terminal, present that token.

### Commands go down, and none is lost or repeated

- The server stores a command before it sends it, with an identifier, an attempt number and an expiry.
- The client keeps a receipt of each command it ran. A command that arrives twice runs once, and the client sends the recorded result again.
- A result names the command and the attempt. A result for an older attempt is ignored.
- A command past its expiry is not run.

### Events come up, and none is lost or applied twice

- The client numbers its events and keeps them on disk, within a limit, until the server acknowledges them.
- The server remembers the last event number it applied for each machine and drops any it already has.
- After a reconnection, the client sends again from the first event that was not acknowledged.

### Knowing whether a machine is there

The client sends a heartbeat every 10 seconds, with the load of its processor and its memory.
The server marks a machine offline when its stream closes, or after 30 seconds without a heartbeat.
A lost connection does not prove that a harness stopped. Wave 2 settles what runs after a reconnection.

### What travels in wave 1

| Direction | Message | Meaning |
|---|---|---|
| Up | Facts | The operating system, cores, memory, client version, folder for repositories and agent limit. |
| Up | Harnesses found | Each harness the machine looked for, and its version when installed. |
| Up | Heartbeat | The current load. |
| Down | Scan harnesses | Look for installed harnesses again. |
| Down | State | Approved, renamed or removed. |

`ClientService.ReconnectClient` cannot work as the contract describes it, because the server cannot dial a machine.
It is an open question below.

### Encryption on the link

A machine reaches a loopback address over plain HTTP/2.
It reaches any other address over HTTPS only, and the client refuses anything else.
This is the same rule the website follows.

## The client in wave 1

The client is one program, `lgtm-client`, with two commands in wave 1.

- `lgtm-client join <address> <token>` enrolls the machine.
- `lgtm-client run` keeps the machine connected. It reports the machine's facts, looks for the four harnesses and their versions, and sends heartbeats.

The client keeps its key, its event buffer and its command receipts in `~/.local/state/lgtm-client/`.
It runs as a service of its user, outside the compose stack, because it needs that user's harnesses, repositories and sign-ins.

The four harnesses are Claude, Codex, Gemini and Cursor.
A check on 2026-11-25 drove the installed Claude, Codex and Cursor from a Go program through the library `github.com/coder/acp-go-sdk`.
Gemini was not installed on the machine that ran the check.

## The compose stack

The folder is `deploy/`.

| Service | What it does |
|---|---|
| `postgres` | The database, with its data in a named volume. |
| `openbao` | The vault. It keeps its data in `postgres`, and it is reachable only inside the stack. |
| `server` | The Go server. It is reachable only inside the stack. |
| `web` | A web server that serves the built website and passes the contract's calls on to `server`. |

- The website and the server share one address. The strict session cookie therefore works, and the website's server address is `/`.
- The stack publishes its port on the loopback address only, by default.
- A public address is a setting the owner turns on, and it must be `https`.
- The port is a variable. Its default is 8787, which the old engine also uses, so a machine that still runs the old engine sets another.
- One command starts the stack. A second command, `deploy/install-client.sh`, installs the client on the same machine and joins it with a token the server's own command makes.

## Rules that every later part follows

These rules are what lets the parts of wave 2 be built side by side.

1. A part owns one folder under `internal`, one contract file, and its own folder of database changes.
2. The contract file of a part is written and reviewed before its code.
3. A part does not read or write the tables of another part. It calls that part's Go interface.
4. Every write goes through the store's transaction and says what changed.
5. Every `Watch` call is served through the `watch` folder.
6. Every command to a machine goes through the link's stored commands.
7. A part keeps nothing in memory that it cannot read again from the database.
8. An error is a Connect code with one sentence the website can show.
9. Text from an agent is data, never a command. Secrets never reach a log, the board or the website.
10. Tests run against a real PostgreSQL. Tests of the link use a real client against a real server in one process.
11. Logic carried over from a file under `third_party/` keeps that file's credit.

## How wave 1 is proven done

Each line is a check that someone can run.

1. One command starts the stack, and the website opens at the loopback address.
2. The server refuses to start without `LGTM_WEB_PASSWORD`.
3. The owner signs in with the password. A wrong password is refused. A call without a session gets `unauthenticated`.
4. The session survives a restart of the server. Changing the password ends it.
5. The owner makes a join token on the Machines page. A used or expired token is refused.
6. `lgtm-client join` on a machine makes the machine appear as waiting for approval, without a reload of the page.
7. After approval the machine shows as online, with its facts, its load and the harnesses it found.
8. Stopping the client shows the machine as offline within 30 seconds. Starting it again shows it online.
9. Restarting the server while a machine is connected brings the machine back online by itself.
10. A machine whose key the server does not know is refused. A client refuses a server whose key does not match the one it kept.
11. Events that a machine buffered while the server was down arrive once, in order.
12. `go test ./...` passes against a real PostgreSQL, and the test of the change feed fails when the lock is removed.
13. Restarting the whole stack brings OpenBao back unsealed with no one typing a key, and the server reads its signing key from it.

## Facts checked on 2026-11-25

A throwaway Go program ran each of these on the development machine.

| Building block | Go module | Result |
|---|---|---|
| ACP | `github.com/coder/acp-go-sdk` 0.20.0 | Drove Claude, Codex and Cursor. It speaks protocol version 1. |
| The registry of ACP | plain HTTPS and JSON | Listed 46 agents, among them every harness the first-run screen offers. |
| The contract | `connectrpc.com/connect` 1.21.1 with `buf` | Generated Go from the four existing contract files and served them. |
| PostgreSQL | `github.com/jackc/pgx/v5` 5.13.0 | A write and its feed row in one transaction, and a second connection woke on it. |
| Terminals | `github.com/creack/pty` 1.1.24 | Ran a shell in a terminal. Linux, macOS and the BSD systems only. |
| Tools for agents | `github.com/modelcontextprotocol/go-sdk` 1.8.0 | A typed tool answered a call. |

Read in the documentation of OpenBao on the same day, and not run: OpenBao can keep its data in PostgreSQL, and it can unseal itself from a key in a file.

The same day's checks of how each harness takes a message during a turn:

- Claude advertises steering and a queue of its own.
- Codex advertises steering.
- Cursor steers and queues only in its own terminal program. Over ACP, a second prompt cancels the running turn and starts a new one in the same conversation. The client uses that as steering for Cursor, and it holds a message itself when the message must wait.

## Three rulings that later waves carry

The owner ruled on 45 open issues on 2026-11-25. The rulings are in [docs/issues.md](../issues.md).
Three of them shape the server and the client, so they are stated here.
None of them is built in waves 0 or 1. Each names what wave 1 must not make harder.

### A vault on the server, and tools that use a key the agent never reads

This comes from the ruling on issue 326.

- The server holds the secrets of tools, such as the key of a search service.
- An agent's call to such a tool goes through its client to the server.
- The server adds the secret to the call. The agent receives the result, never the key.
- An agent sees the names of the variables of its environment. It does not see the values of the secret ones.

What this asks of wave 1:

- The vault is OpenBao, and it is part of the compose stack from wave 1. See "Secrets live in OpenBao". The server's own secrets go into it first. Each secret sits under a path that names its kind and its owner, so that the secrets of tools fit beside them later.
- The link already leaves out "the calls an agent makes through its machine". Those calls are how a tool call reaches the vault, so the link contract keeps room for a call that travels up and an answer that travels down.

Still open: how a tool that runs on the machine, and not on the server, gets its key without the agent being able to read it.

### One source of memory files for all agents

This comes from the ruling on issue 946.

- The memory files of agents are held in one place by the server, possibly as rows in the database.
- An agent on any machine, under any account of a harness, reads the same memories.
- The aim is that a change of account, and a fleet of several machines, lose no memory.
- The owner noted that this may be hard to build.

What this asks of wave 1: nothing is built. The client must not treat the memory folder of a harness on one machine as the record. The record is the server's.

Still open: how the server's memories reach a harness that reads memory from files in a fixed folder, how a memory that an agent writes on a machine travels back, and what happens when two agents write the same memory.

### An investigation agent after three refusals

This comes from the ruling on issue 336.

- A duty that keeps being refused is retried with a growing wait between tries.
- After three refusals the server starts an investigation agent, which is a new kind of agent.
- It is a short job in the background. It runs on the account that has the lowest usage.
- It finds out why the duty is refused and delivers the result to the project lead.
- The project lead's own context is not filled with the search.

What this asks of wave 1: nothing is built. The count of refusals and the time of the next try are stored in the database, by rule 7 of "Rules that every later part follows".

Still open: which model the investigation agent uses, what it may read, and what happens when the investigation itself fails.

## What these two waves leave out

- Running an agent. That is the first unit of wave 2.
- Every service of the contract except `SessionService` and `ClientService`.
- Byte streams on the link, and the calls an agent makes through its machine.
- Choosing a machine for work.
- The secrets of tools in the vault, the one source of memory files, and the investigation agent. See "Three rulings that later waves carry".
- Several server nodes.
- A client on Windows.
- Any import of the old engine's data. The server starts with an empty database.

## Open questions

1. How does a website hosted on another site than its server sign in? The session cookie is strict, so the browser does not send it across sites. The compose stack avoids the question by serving both from one address.
2. Does `ReconnectClient` stay in the contract? A machine dials the server, so the server cannot call an offline machine.
3. How does the client trust a certificate that no public authority signed? The join token could carry the certificate's fingerprint as it carries the server key's.
4. What happens to an agent's work when its machine is gone for good?
5. When one account is used on several machines, does the owner sign in on each one?
6. Which hostnames do running products get, so that a product's frame may have an origin of its own?

Questions 1, 2 and 3 touch wave 1. None of them stops it from being built on one machine with a loopback address.

## What must change outside this document before builders start

- The owner removed `SECURITY.md` on 2026-11-25, because its rules described the old engine. The security rules of the new design are the ones this document states. A new security document is written when the design has settled.
- `README.md` still describes the old engine and how to start it.
- `src/contracts/README.md` needs the shared `Watch` shape and the link contract added to its tables once wave 0 is merged.
- `docs/specs/2026-11-20-server-client-website-direction.md` has open questions that the decisions above now answer.
