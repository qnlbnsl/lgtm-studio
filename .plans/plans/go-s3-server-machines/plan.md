---
topic: go-s3-server-machines
status: discover
tier: L
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# Machines and the server end of the link

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Server team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 3. A machine joins
- **Team:** Server
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- The server keeps a record of each machine and of the public key it registered.
- The owner makes a join token, approves a machine by name, renames it and removes it.
- A machine joins with a token, proves itself on every connection, and is refused when its key is unknown.
- A command to a machine is stored before it is sent. An event from a machine is applied once.
- The server knows within 30 seconds that a machine is gone.

## What a person can observe when it is done

- The Machines page of the website works against the Go server with no change to a screen.
- A used or expired token is refused.
- A machine whose key the server does not know is refused.
- Restarting the server brings a connected machine back online by itself.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | Machine records: the table, its database change, and the functions that read and write it through the store. | `src/server/internal/machines/store.go`, `src/server/internal/machines/migrations/` | medium | sprint 2 |
| 2 | Join tokens: used once, expired after 15 minutes, stored as a verifier only, and able to be revoked. | `src/server/internal/machines/tokens.go` | high | 1 |
| 3 | The Machines page's calls that only touch records: `WatchClients`, `CreateClientToken`, `RevokeClientToken`, `ApproveClient`, `RenameClient` and `RemoveClient`. | `src/server/internal/machines/service.go` | medium | 1, 2 |
| 4 | The `Join` call: a token and a public key make a machine that waits for approval. | `src/server/internal/link/join.go` | high | 2 |
| 5 | The proof on every connection: both ends sign a challenge, and a new connection of a machine replaces its older one. | `src/server/internal/link/handshake.go`, `src/server/internal/link/hub.go` | high | 4 |
| 6 | Stored commands: stored before sending, with an attempt number and an expiry. A result for an older attempt is ignored. | `src/server/internal/link/outbox.go` | high | 5 |
| 7 | Incoming events: applied in order, dropped when already applied, and acknowledged. Facts, harnesses found and heartbeats update the machine record. | `src/server/internal/link/inbox.go` | high | 5 |
| 8 | Presence: a machine is offline when its stream closes or after 30 seconds without a heartbeat. A server that starts marks no machine online by memory. | `src/server/internal/link/presence.go` | medium | 7 |
| 9 | The two calls that send a command: `ScanClientHarnesses`, and `ReconnectClient`, which restarts a client that is connected. | `src/server/internal/machines/commands.go` | medium | 3, 6 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `src/server/`
- **Reads only:** `src/contracts/`, `src/client/` to read only, the design

## What it must agree on with other teams

- Every message of the link, with `go-s3-client-link`. Both plans are compared line by line before phase 4.
- The entry point that a test uses to start a server in its own process. See issue 1634.

## Checks of the design that it makes pass

C5 to C11, together with `go-s3-client-link`.
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. Issue 1633: which certificate's fingerprint the join token carries, and how the server learns it.
2. What a machine that waits for approval may do on the link.
3. How the signing key of the server is replaced, and what that does to machines that kept the old one.
4. The blueprint names `link/service.go`. This brief cuts it into `join.go`, `handshake.go` and `presence.go`.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
