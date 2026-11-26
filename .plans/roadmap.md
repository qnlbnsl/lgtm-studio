---
kind: roadmap
updated: 2026-11-25
---

# Roadmap

LGTM Studio is meant to let anyone install it, and set up and steer teams of coding agents from a phone, with every agent's conversation available as a thread.

The owner decided on 2026-11-24 that the engine is rewritten from scratch in Go, as a server and a client.
The rewrite is the only work in progress.
The work that was planned for the old engine is not continued there. What the product still needs from it returns in the later waves of the rewrite.

## Now

| Kind | Name | Overview |
|---|---|---|
| epic | go-rewrite | A Go server on PostgreSQL and a Go client on each machine replace the TypeScript engine. The website stays. The first four sprints end with a machine that joins a server and stays connected. |

## Later

| Kind | Name | Overview |
|---|---|---|
| other | Desktop and phone apps | Native apps on the same contract as the website. |
| other | A Visual Studio Code extension | Threads and approvals in a side panel of Cursor or Visual Studio Code, on the same contract. |

## Done

| Kind | Name | Overview |
|---|---|---|
| epic | lgtm-foundation | The TypeScript engine, its client API, the message board and the foundry roles. The rewrite replaces it, and it stays in `packages/` as a reference. |
