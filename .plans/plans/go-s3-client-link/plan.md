---
topic: go-s3-client-link
status: discover
tier: L
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# Joining a server and the client end of the link

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Client team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 3. A machine joins
- **Team:** Client
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- `lgtm-client join <address> <token>` enrolls a machine.
- `lgtm-client run` keeps the machine connected, and it reconnects by itself.
- The client refuses a server whose key does not match the one it kept.
- The client reports its facts, its harnesses and its load, and it runs each command once.

## What a person can observe when it is done

- After `lgtm-client join`, the machine appears as waiting for approval.
- Stopping the client shows the machine as offline within 30 seconds. Starting it again shows it online.
- Events that the client buffered while the server was down arrive once, in order.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | The `join` command: it reads the token, makes the key pair, calls `Join`, checks the server's key against the fingerprint in the token, and keeps that key. | `src/client/internal/enroll/join.go` | high | sprint 1 |
| 2 | The connection: it dials the server, checks the server's signature, and signs the server's challenge. It refuses plain HTTP to any address that is not loopback. | `src/client/internal/link/conn.go` | high | 1 |
| 3 | Reconnection: the client tries again with a growing wait, at least every 15 seconds, and sends again from the first event not acknowledged. | `src/client/internal/link/retry.go` | medium | 2 |
| 4 | Command handling: scan the harnesses, take a new state, and restart. Each runs once, through the receipts of sprint 1. | `src/client/internal/link/commands.go` | medium | 2 |
| 5 | Reporting: the facts on each connection, the harnesses found, and a heartbeat every 10 seconds with the load. | `src/client/internal/link/report.go` | medium | 2 |
| 6 | The `run` command: it ties the parts together, and it stops cleanly when the server removes the machine. | `src/client/cmd/lgtm-client/main.go` | low | 3, 4, 5 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `src/client/`
- **Reads only:** `src/contracts/`, `src/server/` to read only, the design

## What it must agree on with other teams

- Every message of the link, with `go-s3-server-machines`.
- The entry point that a test uses to start a client in its own process. See issue 1634.

## Checks of the design that it makes pass

C6 to C11, together with `go-s3-server-machines`.
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. Issue 1633: what the client pins, the server's signing key or a certificate, or both.
2. What `run` does on a machine that has not joined.
3. What the restart command does when the client is not run as a service.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
