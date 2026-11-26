---
topic: go-s0-contract
status: discover
tier: M
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# The contract and the Go module

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Contract team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 0. The contract
- **Team:** Contract
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- The repository gets one Go module at its root.
- The contract files generate Go code beside the TypeScript they generate today.
- The shape that every `Watch` call shares is written down once.
- What a server and a machine say to each other is written down as a contract, before either end is built.

## What a person can observe when it is done

- `go build ./...` passes at the repository root.
- `pnpm --filter ./src/contracts check` passes, and it now fails when the generated Go code is out of date.
- The website still builds and still signs in against the sample server.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | The Go module at the repository root, with the module path of the design and Go 1.26. | `go.mod`, `go.sum` | low | nothing |
| 2 | Go generation: the contract generates Go messages and Connect handlers into `src/contracts/gen/go/`. The plugins are pinned in `go.mod`. | `src/contracts/buf.gen.yaml`, `src/contracts/package.json` | medium | 1 |
| 3 | The shared `Watch` shape: everything first, then one change at a time, each with its position in the feed. | `src/contracts/proto/lgtm/v1/common.proto` | medium | 2 |
| 4 | Joining and the proof of both ends: the `Join` call, the two signed challenges, and the short-lived token of a connection. | `src/contracts/proto/lgtm/link/v1/link.proto` | high | 2 |
| 5 | Commands to a machine: the envelope with an identifier, an attempt number and an expiry, and the two commands of wave 1. | `src/contracts/proto/lgtm/link/v1/commands.proto` | medium | 4 |
| 6 | Events from a machine: the numbered envelope, the acknowledgement, and the three reports of wave 1. | `src/contracts/proto/lgtm/link/v1/events.proto` | medium | 4 |
| 7 | One command that checks Go code: formatting, `go vet` and `go test ./...`. The contract's README lists the new files. | `scripts/check-go.sh`, `src/contracts/README.md` | low | 1 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `src/contracts/`, `go.mod`, `go.sum`, `scripts/check-go.sh`
- **Reads only:** `src/frontend/`, `src/mock-server/`, the design

## What it must agree on with other teams

- The Server team and the Client team both read the link contract. Their planners review it in phase 3, before it is frozen.
- The link contract keeps room for what wave 2 adds: byte streams, and the calls an agent makes through its machine.

## Checks of the design that it makes pass

None of the 13 checks. This plan has its own: the three lines under "What a person can observe".
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. Protocol Buffers has no generic message. The planner decides what `common.proto` holds, and how each `Watch` reply follows the shared shape.
2. Whether `WatchClientsResponse` gains the feed position now. Adding a field breaks no website, so the contract revision can stay at 1.
3. Where the generated Go code lives, so that `clean: true` in `buf.gen.yaml` does not delete the generated TypeScript.
4. Which fields the join token carries. Issue 1633 bears on this.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
