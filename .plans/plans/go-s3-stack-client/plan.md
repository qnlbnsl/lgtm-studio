---
topic: go-s3-stack-client
status: discover
tier: S
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# Installing the client on a machine

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Stack team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 3. A machine joins
- **Team:** Stack
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- A second command installs the client on the machine that runs the stack, as a service of its user, and joins it.
- The server's own command makes a join token without the website.

## What a person can observe when it is done

- `deploy/install-client.sh` ends with the machine waiting for approval on the Machines page.
- After a restart of the computer, the client starts by itself.
- The user-flow script passes all 13 checks.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | The server's own command that makes a join token. | `src/server/cmd/lgtm-server/client_add.go` | low | `go-s3-server-machines`, item 2 |
| 2 | The installer: it builds the client, installs it as a service of the user outside the compose stack, and joins it with a token from the server's command. | `deploy/install-client.sh`, `deploy/lgtm-client.service` | medium | 1 |
| 3 | The rest of the user-flow script turned on: C5 to C11. | `deploy/test/` | low | 2 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `deploy/`, and one file in `src/server/cmd/lgtm-server/`
- **Reads only:** `src/server/`, `src/client/`, the design

## What it must agree on with other teams

- The Server team owns `src/server/`. Item 1 is one file there, so the Server team's lead merges it.
- The name of the service, and where the installer puts the program.

## Checks of the design that it makes pass

C5 to C11 run against the stack, and all 13 together.
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. How the client is installed as a service on macOS, where the design names no tool.
2. Whether the installer builds the client or downloads a built one.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
