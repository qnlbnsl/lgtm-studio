# Splitting LGTM Studio into a server, a client and a website: the owner's direction

This document records what the owner said on 2026-11-20.
It is a direction, not a design.
Nothing in it is built, and every open question is listed at the end.

## The three pieces

The owner wants to break the software into three pieces.

- **The server** holds the state and answers every request.
- **The client** is a process on a machine that does the work: it holds the checkouts and runs the agents. In this document "client" always means this process, never the website.
- **The website** is the page the owner uses.

The website already talks to the engine through remote procedure calls defined with the Effect library.
The owner wants those calls written down as contract definitions, as protocol buffers or in another form.
How a client connects to the server also needs a definition.

## How the website is reached

Installing the server also starts the website.
The website can also be hosted by itself.
When it is hosted by itself, it asks the user for the address of the server.

## Several server nodes

The owner wants to allow a load balancer in front of several server nodes.
The nodes keep themselves in step and maintain a quorum.

The owner's first thought is a dedicated database.
The server creates read replicas of it on other machines.
If a machine goes down, every server instance switches to another database, and the switch is immediate.

The owner's own open question: with 20 machines that run clients, of which 5 also run a server node, are there 5 databases, or one database with 2 read replicas on other nodes that all 5 server nodes connect to?

## Clients and speed

A client connects to the main server process.
How that works behind a load balancer still has to be thought out.
Clients reach out to a server node and relay data to the website.
The transport can be remote procedure calls, WebSockets or another choice.
It must be efficient and very fast.

## Adding a repository from a hosting service

The owner clicks "Add a repository" and sees the repositories that the linked account can reach.
The entry "Linking a hosting account to add a repository directly from GitHub, GitLab or Bitbucket" in `docs/features.md` records the account side.
The repository is cloned on whichever machine needs it.
The first clone goes to an idle client; with one client, it goes there.

After the clone, the repository agent sets the repository up.
It creates the configuration files and runs the whole onboarding of the repository.
The entry "A repository agent that works in the main checkout and prepares a repository for projects" records that agent.
When it is done, the result is committed, and the owner opens a project.

## Projects in the database, and a clone per agent

The owner calls this the tricky part of the idea.

A project and all its information are stored in the database.
When an agent is launched, it does not get a Git worktree beside the main checkout.
Instead, any client is chosen, the repository is cloned there, and the agent works in that clone.
The reason is complete isolation, and the freedom to spread work over machines.

The project has its own branch.
All of its teams push to branches under it.

## Why the owner wants this

With three separate pieces, the same software serves a personal laptop, a home laboratory and a large company's fleet.

## Open questions

- The contract format: protocol buffers, or the existing Effect schemas published as the contract.
- The client's connection: who dials whom, how a client proves its identity, and how it finds a server node behind a load balancer.
- The database: which one, how many, and who promotes a replica. Today the engine keeps one SQLite event store in one process with one writer.
- Whether several server nodes are needed for the first version, or one server node with a replicated database is enough.
- What a clone per agent costs for a large repository, and whether a client keeps one shared object store to make clones cheap.
- How ignored files, submodules and installed packages reach a fresh clone; issue 1469 records the same need for worktrees.
- How agents on different machines exchange branches: through the hosting service, or through the server.
- Where secrets live when agents run on many machines; the entry "A built-in vault in the server process" is related.
- What happens to work in progress when a client goes away.
- `SECURITY.md` assumes one machine, one user and loopback or HTTPS; each of its binding rules needs a reading for several machines.
