# Default instructions for the default workflow

These are the default instructions for the roles of LGTM Studio's default workflow.
They were adapted from the software-foundry agent files of the owner's private toolkit.
The prose describes conduct and does not grant authority.

## The base for each role

| Base | Roles that read it |
|---|---|
| `lead-base.md` | `project-lead`, `orchestrator`, `planning-lead`, `design-lead` and `build-lead`. |
| `worker-base.md` | `planner`, `designer`, `build-init` and `builder`. |
| `reviewer-base.md` | `reviewer`. |
| No base | `test-runner` and `evaluator`. |

The base comes first and the role file follows it.
The reviewer does not read the worker base.
Unknown role names receive the base of their parsed kind.

## Replacing a file

The owner can replace one file at `~/.lgtm/roles/<name>.md` under the engine's configured home.
Each file is resolved independently.
Named advice from `roles.role-file.set` replaces the role text and preserves the bases.
Repository role overrides are not loaded.
See [the role instructions document](../../../../../docs/roles.md) for delivery, safety checks and limits.

The engine reads the Markdown resources beside its module through `import.meta.url`.
A packaged install must keep this directory beside `instructions.ts`, as it keeps the shipped workflow beside its store.
