# Default role instructions

LGTM Studio ships instructions for the twelve agent roles in its default build loop.
The files live in `packages/engine/src/roles/instructions/`.
They were adapted from the software-foundry agent files of the owner's private toolkit.
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

A bare default role name selects the same instructions as its name after the matching kind and a colon.
For example, `builder` and `worker:builder` select the builder instructions.
Both `lead:project-lead` and the older `project-lead` select the project lead instructions.
An unknown name receives the base of its parsed kind.
A known name with the wrong kind receives only that kind's base.
For example, `lead:builder` receives only `lead-base.md`, and `worker:build-lead` receives only `worker-base.md`.
An unknown kind such as `chef:builder` receives only the generic worker base and no owner advice named for builder.
Workers receive `worker-base.md`.
Leads and orchestrators receive `lead-base.md`.
A display name does not change the authority granted to an agent.
Use the explicit kind when launching an agent.

## How instructions reach the agent

The engine prepends the composed instructions to the caller's first ordinary prompt in a new session.
It delivers both in the same turn through the adapter.
The caller's stored prompt and its size remain unchanged.
The thread and chat retain their existing prompt text.
Live user-message echoes are currently ignored by the adapter translator.
The runner also filters an instruction echo for the whole carrying turn, across message identifiers.
The filter removes any opening decoration and all text through the random closing marker.
Ordinary text without an instruction marker is retained until the turn ends.
The search buffer is bounded by the instruction prefix size plus 4 KiB.
A recognized block retains only enough text to find its closing marker.
A malformed closing marker withholds the remaining echo rather than storing instructions.

For a project lead, the order is the lead base, the project lead role, the engine's project opening instruction, and the goal.
The opening instruction describes Define and the owner's approval mechanics.
A role override cannot remove that instruction.
A fresh resume repeats the role rules without repeating that original project opening prompt.

A fresh resume receives instructions again.
Startup recovery and account return also supply them when they open a new session.
Loading an existing session does not repeat them or read the instruction files.
A size or read failure cannot prevent that load.
A successful compaction keeps the same session and restores the instructions on its next ordinary prompt.
An agent can call `roles.orient` to read the current composed text in `advice`.
That read resolves current overrides again.
A fresh session keeps its initial composition for compaction restoration.
A loaded session resolves the current composition only when restoration is needed after compaction.
If that restoration fails, the engine sends the caller's text alone and keeps the instructions owed.
It logs one fixed warning for consecutive restoration failures and retries on the next ordinary prompt.
The engine owes the instructions until a carrying turn succeeds without error or cancellation.
Capacity, usage pauses, prompt errors and cancellation leave them owed for the retry.
A failed send also leaves them owed.
The runner keeps this obligation in memory, and the engine rebuilds it from the log when it loads a session after a restart.
A loaded session owes the instructions when no turn has ended normally since its session opened or since its last compaction.
A requeued first prompt after a restart therefore carries them again, even when the saved history already shows the instruction block.
The runner and the engine use one rule for this, in `packages/engine/src/roles/instructionObligation.ts`.
Acceptance alone does not discharge the instructions.
A restart part way through a turn the adapter had accepted therefore sends them again with the carry-on.
Each successful engine-requested compaction re-adds the full text on the next ordinary prompt.
That costs roughly 10 KiB for a default lead, or up to the 48 KiB composition bound for overrides.
A compaction started by the harness itself is not detected.

Engine prose is redacted and escaped on every line before delivery.
The adapter receives it in its own `Engine role instructions` block with a fresh random marker.
The block names the sending agent as the brief's author only for the original first prompt from the agent's parent.
The engine checks the start event, delivery identifier and recorded parent rather than the prompt's text.
Retries of that same delivery retain the brief label.
Other messages, including routed questions after compaction, receive no brief sentence.
That brief keeps its separate agent-data envelope.
The engine block contains no delivery identifier and grants no replay acceptance proof.
An owner's unaccepted first prompt is requeued after restart even when the saved history contains the instruction block.
A prompt already accepted before the crash receives the existing carry-on behavior.
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
Saving named advice is refused if any applicable composition would exceed the instruction bound.

Repository folders are not searched for role overrides.
There is no role instruction editor or new file-management command in this change.
The existing role-file commands remain available through the authenticated application programming interface (API).

## Limits and unsafe files

An override file may contain at most 24 kibibytes (KiB) of UTF-8 text.
The home folder and each override directory must be real directories without symbolic links.
An override must be a regular file with one hard link.
Directory descriptors remain open during the bounded read to prevent a pathname replacement from redirecting it.
These file checks use the same descriptor filesystem approach as workflow storage.
Overrides do not apply without `/proc/self/fd` or when the configured home path is a symbolic link.
The engine uses the defaults in those cases.
A missing descriptor filesystem has a distinct fixed diagnostic.

A missing, unsafe, unreadable or oversized override falls back to its built-in file.
A merely missing override is silent.
An unsafe, unreadable or oversized override emits one redacted diagnostic with a fixed instruction name.
The diagnostic includes no supplied path or file content.
An unreadable built-in file refuses a new start.
Orientation still returns identity and roster with a fixed unavailable-advice note.
Existing-session loads remain independent of these reads.

The composed instruction text may contain at most 48 KiB of UTF-8 text.
The engine checks the bound before starting an agent and after redaction and escaping.
An excessive composition returns a typed command refusal.
The caller's prompt is outside this instruction budget.

The encoded `roles.orient` reply may contain at most 256 KiB, including its roster and data envelope.
If the advice makes that reply too large, orientation replaces it with the fixed unavailable-advice note.
If the roster and metadata alone exceed the bound, the reply is refused.
The engine does not silently truncate the roster.
The existing JSON role file remains limited to 64 KiB.
Its encoded stored event remains limited to 200 KiB.

## Same-user access to overrides

An agent with shell access or full permission runs as the same operating-system user as the engine.
It can write an override that the next agent of that role receives as engine instructions.
This is the same file-access exposure as `~/.lgtm/workflows`.
File checks guard against unsafe paths and accidents.
They do not separate programs running as the same user.
The agent sandbox described in `SECURITY.md` remains planned.

## What the prose does not enforce

The instructions ask agents to finish checks before reporting done.
They also prohibit committing run notes.
The engine does not mechanically validate those claims or reject such commits.
Frozen tests, independent review and isolated builder folders remain responsibilities of the agreed brief and its leads.
The engine uses the parsed role kind to allocate worktrees for project workers.
Leads and orchestrators retain their supplied folders.
A worker starts from the starter's last commit.
Uncommitted changes are not copied.
The required `folder` field does not choose a project worker's checkout.
A lead calls `client.snapshot` and reads `project.workspaces` to find a builder's worktree.
Its `owner` matches the builder's agent identifier returned by `start`.
The lead supplies that row's `worktreeId` and `read-only` permission when starting a reviewer or evaluator there.
Before starting a test runner, the lead merges the builder's branch into its own worktree.
The test runner starts without `worktreeId` and receives its own worktree at the lead's last commit.
Its brief names an authorized writable permission and a log folder inside its own worktree.
Test requests name that runner's folder and exact commit.
A lead can merge the worker's branch with its own Git commands.
The owner can use Integrate to merge into the recorded parent worktree.
An instruction file does not create a blind review.

## Defaults for starting agents

The owner's decision of 2026-11-17 makes automatic review the default for new leads and workers.
The ordered permission levels are `read-only`, `ask`, `edit`, `auto` and `full`.
An omitted start permission uses `auto`, capped at the saved harness default and the starter's level.
A lead still supplies `read-only` explicitly for another agent's checkout.
An explicit level remains valid when it does not exceed the starter's ceiling.
Existing agents keep their stored levels.
A registered worktree of the starter's project may be inside the owner's home.
The home itself, state folder, plugin root and home dotfolders remain protected.
A removed registration prevents a later resume.
