# Default role instructions

LGTM Studio ships instructions for the twelve agent roles in its default build loop.
The files live in `packages/engine/src/roles/instructions/`.
They were adapted from the owner's Founder OS software-foundry agent files.
They describe conduct and do not grant permissions.
The authority parser, parent chain and holder checks remain independent of these files.

## Which files an agent receives

| Role | Files, in order |
|---|---|
| `project-lead` | `lead-base.md`, then `project-lead.md`. |
| `orchestrator` | `lead-base.md`, then `orchestrator.md`. |
| `planning-lead` | `lead-base.md`, then `planning-lead.md`. |
| `design-lead` | `lead-base.md`, then `design-lead.md`. |
| `build-lead` | `lead-base.md`, then `build-lead.md`. |
| `planner` | `worker-base.md`, then `planner.md`. |
| `designer` | `worker-base.md`, then `designer.md`. |
| `build-init` | `worker-base.md`, then `build-init.md`. |
| `builder` | `worker-base.md`, then `builder.md`. |
| `reviewer` | `reviewer-base.md`, then `reviewer.md`. |
| `test-runner` | `test-runner.md`. |
| `evaluator` | `evaluator.md`. |

A bare default role name selects the same instructions as its name after a colon.
For example, `builder` and `worker:builder` select the builder instructions.
Both `lead:project-lead` and the older `project-lead` select the project lead instructions.
An unknown name receives the base of its parsed kind.
Workers receive `worker-base.md`.
Leads and orchestrators receive `lead-base.md`.
A display name does not change the authority granted to an agent.
Use the explicit kind when launching an agent.

## How instructions reach the agent

The engine prepends the composed instructions to the caller's first ordinary prompt in a new session.
It delivers both in the same turn through the adapter.
The caller's stored prompt and its size remain unchanged.
The thread and chat retain their existing prompt text.
Exact user-message echoes of the injected prefix are removed before they reach the thread.

For a project lead, the order is the lead base, the project lead role, the engine's project opening instruction, and the goal.
The opening instruction describes Define and the owner's approval mechanics.
A role override cannot remove that instruction.

A fresh resume receives instructions again.
Startup recovery and account return also supply them when they open a new session.
Loading an existing session does not repeat them.
A successful compaction keeps the same session and restores the instructions on its next ordinary prompt.
An agent can call `roles.orient` to read the current composed text in `advice`.
That read resolves current overrides again.
A session keeps the instruction text resolved at its start for compaction restoration.

Engine prose is redacted and escaped on every line before delivery.
The adapter receives it inside a fresh data envelope under an instruction header.
No instruction line can become a leading harness slash command.

## How the owner overrides a file

Put a Markdown file at `~/.lgtm/roles/<name>.md`.
The engine resolves this folder under its configured home directory.
It resolves each file independently.
For example, replacing `builder.md` keeps the shipped worker base.
Replacing `worker-base.md` affects every role that reads that base.

The existing owner-only `roles.role-file.set` command can supply named advice and supervision limits as a JavaScript Object Notation (JSON) role file.
Its named advice takes precedence over the disk role file.
It replaces the role text and keeps the bases.
Advice for a custom named role is appended after that kind's base.
An empty advice string intentionally removes the role text.
The role file's limits retain their existing meaning.

Repository folders are not searched for role overrides.
There is no role instruction editor or new file-management command in this change.
The existing role-file commands remain available through the authenticated application programming interface (API).

## Limits and unsafe files

An override file may contain at most 24 kibibytes (KiB) of UTF-8 text.
The home folder and each override directory must be real directories without symbolic links.
An override must be a regular file with one hard link.
Directory descriptors remain open during the bounded read to prevent a pathname replacement from redirecting it.
These file checks use the same descriptor filesystem approach as workflow storage.

A missing, unsafe, unreadable or oversized override falls back to its built-in file.
Each fallback emits one redacted diagnostic with a fixed instruction name.
The diagnostic includes no supplied path or file content.
An unreadable built-in file refuses the operation.

The composed instruction text may contain at most 48 KiB of UTF-8 text.
The engine checks the bound before starting an agent and after redaction and escaping.
An excessive composition returns a typed command refusal.
The caller's prompt is outside this instruction budget.

The encoded `roles.orient` reply may contain at most 256 KiB, including its roster and data envelope.
An oversized reply is refused without truncating the advice or roster.
The existing JSON role file remains limited to 64 KiB.
Its encoded stored event remains limited to 200 KiB.

## What the prose does not enforce

The instructions ask agents to finish checks before reporting done.
They also prohibit committing run notes.
The engine does not mechanically validate those claims or reject such commits.
Frozen tests, independent review and isolated builder folders remain responsibilities of the agreed brief and its leads.
A role name does not create a worktree or a blind review.
