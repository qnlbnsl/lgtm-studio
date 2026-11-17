# Automatic permission modes in the pinned adapters

This check was made on 2026-11-17 for the owner's decision that new leads should use automatic review.
The engine pins `claude-agent-acp` at 0.99.1 and `codex-acp` at 2.6.0.
The installed adapter source and the corresponding upstream tags were read without starting a harness or reading credentials.

## Claude exposes the auto mode

The [pinned session-mode manager](https://github.com/agentclientprotocol/claude-agent-acp/blob/v0.99.1/src/session-mode.ts) accepts `auto` through the Agent Client Protocol (ACP).
It synchronizes the selected mode with the Claude Agent Software Development Kit.
It falls back to `acceptEdits` when the selected model does not support automatic review.
The adapter publishes the fallback mode and a warning.
The [Claude permission documentation](https://code.claude.com/docs/en/permission-modes) explains that a classifier evaluates actions in this mode.
Some actions can still require human input or be refused.
Automatic review is not a promise that every risky action produces a prompt.

## Codex exposes the agent mode

The [pinned AgentMode source](https://github.com/agentclientprotocol/codex-acp/blob/v2.6.0/src/AgentMode.ts) defines `agent` as the automatic review preset.
It sets `approvalsReviewer` to `auto_review` and `approvalPolicy` to `on-request`.
It retains the workspace-write sandbox with network access disabled by default.
The distinct `workspace-write` preset uses the user reviewer.
The unrestricted preset is `agent-full-access`.
The [official OpenAI automatic review documentation](https://learn.chatgpt.com/docs/sandboxing/auto-review) confirms that an eligible approval request goes to a separate reviewer.
The reviewer can approve or deny the action.
That feature is separate from the sandbox and the approval policy.

## LGTM Studio's mapping and compatibility

The scale is `read-only`, `ask`, `edit`, `auto`, then `full`.
The new defaults map Claude's `auto` level to `auto` and Codex's to `agent`.
Gemini maps it to `autoEdit`, the existing edit mapping.
Cursor maps it to `agent`, also its existing edit mapping.
These last two are explicit fallbacks, not claims that those harnesses expose automatic review.
The harness status describes the fallback.
An old mapping without an `auto` field falls back to its existing edit mapping.
New harness defaults use `auto`.
A child start without a level uses `auto`, capped at the starter's level.
Existing agents and saved owner defaults retain their levels.
No existing event needs migration.
The engine's persistent-permission routing and permission ceiling still apply.
