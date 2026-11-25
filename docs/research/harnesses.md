# Which harnesses people use, read on 2026-11-24

The owner asked on 2026-11-24 which coding harnesses are popular, so that LGTM Studio can offer the common ones.
This document records what three sources say and which harnesses the website now offers.

## The three sources

- **The registry of the Agent Client Protocol (ACP).** The file `https://cdn.agentclientprotocol.com/registry/v1/latest/registry.json` was read in full on 2026-11-24. It lists 46 agents, each with its version and where its program comes from.
- **T3 Code.** Its README on GitHub says it works with Claude Code, Codex, Cursor, OpenCode, Grok Build, Antigravity, Kiro, Pi, Muse Code, fx, and any ACP agent. This was read from a search result, not from the repository.
- **Herdr.** A setup guide quotes 22 agent kinds from its command reference: pi, claude, codex, gemini, cursor, devin, agy, cline, omp, mastracode, opencode, copilot, kimi, kiro, droid, amp, grok, hermes, kilo, qodercli, qwen and maki. This was read from a search result, not from Herdr's own reference.

The owner also named a fourth source that was heard as "OMRG". It could not be identified, so it was not read.

## The harnesses that at least two sources name

A harness is in this table when it is in the ACP registry and at least one of T3 Code and Herdr names it too.

| Harness | T3 Code | Herdr | How the ACP registry gets it |
|---|---|---|---|
| Claude Code | yes | yes | npm package `@agentclientprotocol/claude-agent-acp`, an adapter |
| Codex | yes | yes | npm package `@agentclientprotocol/codex-acp`, an adapter |
| Cursor | yes | yes | archive from `downloads.cursor.com`, with no checksum |
| OpenCode | yes | yes | archive from its GitHub releases |
| Pi | yes | yes | npm package `pi-acp`, an adapter by another author |
| Kiro CLI | yes | yes | archive from `prod.download.cli.kiro.dev` |
| Grok Build | yes | yes | npm package `@xai-official/grok` |
| Google Antigravity | yes | yes | archive from `dl.google.com` |
| Gemini CLI | no | yes | npm package `@google/gemini-cli` |
| GitHub Copilot | no | yes | npm package `@github/copilot` |
| Amp | no | yes | archive of `amp-acp`, an adapter by another author |

## The harnesses one source names

Herdr also names Cline, Factory Droid, Qwen Code, Kimi, Kilo, Devin, Qoder and Hermes, and T3 Code also names Muse Code. Each is in the ACP registry except Hermes, which is on the protocol's list of agents. They are not offered yet. Goose, Junie, Mistral Vibe and Auggie are in the registry and in neither tool.

## What the website offers now

The engine ships Claude Code and Codex. "Add harness" offers the other nine of the table: Gemini CLI, Cursor, OpenCode, GitHub Copilot, Pi, Kiro CLI, Grok Build, Google Antigravity and Amp. The list is in `src/frontend/src/screens/harness-catalog.tsx`, with the version, the licence, the source and the checksum that the registry gave on 2026-11-24.

## What is not known

- **How many people use each harness.** No source here counts users. "Popular" means that tools which manage several harnesses chose to support it.
- **Whether each harness works with the engine.** The engine has run only Claude Code, Codex, Gemini CLI and Cursor. The other seven have not been started by it.
- **Whether an adapter by another author is safe to install.** Pi and Amp reach ACP through adapters that their makers did not write.
