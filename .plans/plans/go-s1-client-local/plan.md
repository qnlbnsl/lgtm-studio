---
topic: go-s1-client-local
status: discover
tier: M
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# What the client does on its own machine

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Client team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 1. Each part stands alone
- **Team:** Client
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- The client program exists and reads its settings.
- A machine makes its key pair and keeps the private key where only its user can read it.
- The client reads the facts of its machine and finds the installed harnesses.
- The client keeps two records on disk: the events that wait for the server, and a receipt of each command it ran.

## What a person can observe when it is done

- `go test ./src/client/...` passes with no server and no network.
- `lgtm-client` builds for Linux and for macOS.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | The program and its settings: the `join` and `run` commands are named and answer that they are not built yet. The state folder is `~/.local/state/lgtm-client/`. | `src/client/cmd/lgtm-client/main.go`, `src/client/internal/config/config.go` | low | sprint 0 |
| 2 | The key of the machine: an Ed25519 key pair, the private key in a file only its user can read, and a refusal to load a key that others can read. | `src/client/internal/enroll/credential.go` | high | 1 |
| 3 | The facts of the machine: operating system, cores, memory, client version and current load, on Linux and on macOS. | `src/client/internal/sysinfo/sysinfo.go` | medium | nothing |
| 4 | The harness scan: look for Claude, Codex, Gemini and Cursor, and read each version with a time limit and without a shell. | `src/client/internal/harness/scan.go` | medium | nothing |
| 5 | Events that wait for the server: numbered, kept on disk within a limit, read again from the first one not acknowledged. | `src/client/internal/link/outbox.go` | high | 1 |
| 6 | Receipts of commands: a command that arrives twice runs once, and the recorded result is returned again. A command past its expiry is not run. | `src/client/internal/link/receipts.go` | medium | 1 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `src/client/`
- **Reads only:** `src/contracts/`, the design

## What it must agree on with other teams

- The layout of the state folder. `go-s3-client-link` reads the same files.
- The event and command envelopes come from the link contract of sprint 0.

## Checks of the design that it makes pass

None alone. Its records are what C11 depends on in sprint 3.
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. What the client does when the event buffer is full: refuse new events, or drop the oldest.
2. How much the buffer may hold, and how often it is flushed to disk.
3. How the load of the processor is read on macOS without a shell command.
4. The blueprint keeps command receipts in `commands.go`. This brief gives them a file of their own.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
