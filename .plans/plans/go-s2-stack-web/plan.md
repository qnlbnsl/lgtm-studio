---
topic: go-s2-stack-web
status: discover
tier: S
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# The images and one command to start everything

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Stack team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 2. The owner signs in
- **Team:** Stack
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- The server is built into one small image.
- The website is built into static files behind a web server, which passes the contract's calls to the server on the same address.
- One command starts the whole stack.

## What a person can observe when it is done

- One command starts the stack, and the website opens at the loopback address on port 8787.
- The owner signs in on that page.
- The stack publishes its port on the loopback address only, unless a public address is set, and that address must be HTTPS.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | The image of the server: a small image that runs as a user without privileges. | `deploy/server.Dockerfile` | low | nothing |
| 2 | The image of the website and the rules of the web server: static files, and the contract's calls passed on to `server`, streams included. | `deploy/website.Dockerfile`, `deploy/Caddyfile` | medium | nothing |
| 3 | The compose file gains `server` and `web`, the settings file, and the port variable with its default of 8787. | `deploy/compose.yaml`, `deploy/example.env` | medium | 1, 2 |
| 4 | The one command that starts the stack, and the part of the user-flow script that covers sign-in turned on. | `deploy/up.sh` | low | 3 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `deploy/`
- **Reads only:** `src/server/cmd/`, `src/frontend/`, the design

## What it must agree on with other teams

- The environment variables of the server, with the Server team.
- How the web server passes a long stream on without cutting it. The link of sprint 3 depends on the same rule.

## Checks of the design that it makes pass

C1. It also runs C2, C3 and C4 against the stack.
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. How a public HTTPS address is turned on, and where its certificate comes from.
2. Issue 1633: where encryption ends for the link, since the web server stands in front of the server.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
