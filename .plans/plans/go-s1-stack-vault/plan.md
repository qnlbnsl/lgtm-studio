---
topic: go-s1-stack-vault
status: discover
tier: M
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# The stack with PostgreSQL and the vault

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Stack team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 1. Each part stands alone
- **Team:** Stack
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- A compose stack starts PostgreSQL and OpenBao.
- OpenBao keeps its data in the same PostgreSQL, in a database of its own.
- OpenBao opens by itself after a restart, from a key file that is made once.
- The server will find a credential that reaches only its own paths in the vault.

## What a person can observe when it is done

- One command starts the two services, and both report that they are healthy.
- After the stack is stopped and started again, OpenBao is unsealed and no one typed a key.
- A second run of the first-start script changes nothing.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | A proof of the two facts read in the documentation of OpenBao: it stores in PostgreSQL, and it unseals from a key file. It is throwaway work, and it runs before the other items. | none kept | medium | nothing |
| 2 | The compose file with `postgres`: a named volume, a health check, and one database each for the server and for OpenBao. | `deploy/compose.yaml`, `deploy/postgres-init.sql` | low | nothing |
| 3 | The settings of OpenBao: storage in PostgreSQL, unsealing from the key file, and a listener that only the stack reaches. | `deploy/openbao.hcl` | medium | 1, 2 |
| 4 | The first-start script: it starts the vault once, makes the key file that only its user can read, and writes the server's credential to a file. | `deploy/openbao-init.sh` | high | 3 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `deploy/`
- **Reads only:** the design

## What it must agree on with other teams

- Where the server's credential for the vault is written, and in what form. `go-s2-server-signin` reads it.
- The path layout of secrets in the vault: each secret sits under a path that names its kind and its owner.

## Checks of the design that it makes pass

The first half of C13.
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. Which form the server's credential takes, and how long it lasts.
2. Where the key file lives on the host, and which user owns it.
3. What the stack does when the key file is missing and the vault already holds data.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
