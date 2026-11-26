---
topic: go-s1-server-store
status: discover
tier: M
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# The server's database layer

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Server team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 1. Each part stands alone
- **Team:** Server
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- Every later part of the server writes through one transaction helper.
- Each part keeps its own numbered database changes, so two builders never share a number.
- One ordered change feed sits beside the tables, and its positions follow commit order.
- A request sent twice happens once.
- Every `Watch` call is served the same way: everything first, then one change at a time.

## What a person can observe when it is done

- `go test ./src/server/...` passes against a real PostgreSQL named by `LGTM_TEST_POSTGRES`.
- The test of the change feed fails when the lock is removed from the code.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | The connection pool and the transaction helper. A test package gets a schema of its own, which it creates and drops. | `src/server/internal/store/pool.go` | medium | sprint 0 |
| 2 | Identifiers: version 7 universally unique identifiers, made in Go. | `src/server/internal/store/ids.go` | low | nothing |
| 3 | Numbered database changes for each part, applied at start-up, with the last number of each part recorded. | `src/server/internal/store/migrate.go` | medium | 1 |
| 4 | Writing the change feed: a part says what changed, and the store writes the feed rows last, under one transaction-level advisory lock. | `src/server/internal/store/feed.go` | high | 1, 3 |
| 5 | Following the change feed: one follower in each process, woken by a notification and by a read every two seconds. | `src/server/internal/store/follow.go` | high | 4 |
| 6 | Removing feed rows and receipts that are older than one day. | `src/server/internal/store/prune.go` | low | 4, 7 |
| 7 | Safe retries: the request key, a hash of the request and the reply are stored in the transaction of the change. | `src/server/internal/store/receipts.go` | medium | 1, 3 |
| 8 | `Watch` streams: subscribe, read everything and the highest position in one transaction, then send each later change. A caller that falls 2,048 changes behind is cut off. | `src/server/internal/watch/watch.go` | high | 5 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `src/server/internal/store/`, `src/server/internal/watch/`
- **Reads only:** `src/contracts/`, the design

## What it must agree on with other teams

- The Go interface of this layer. `go-s2-server-signin` and `go-s3-server-machines` both call it, so their planners review it.
- How a part registers its folder of database changes.

## Checks of the design that it makes pass

C12.
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. The exact Go interface: how a part opens a transaction, says what changed, and registers its database changes.
2. How the `watch` helper stays one piece of code for records of different types.
3. What a slow reader of the feed costs the others, and whether the cut-off at 2,048 changes is enough.
4. The blueprint in `floorplan/` names five files for this folder. This brief adds `follow.go` and `prune.go`.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
