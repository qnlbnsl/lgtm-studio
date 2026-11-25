---
topic: lgtm-foundation
kind: epic
status: active
parked: true
updated: 2026-11-07
---

# LGTM Studio runs the Foundry's teams on its own

## Outcome

LGTM Studio runs teams of coding agents over the Agent Client
Protocol: an orchestrator, team leads and workers, coordinated through a
strict message board, and visible and answerable from a web page, a phone,
or a pane in Herdr. The Foundry skill no longer needs Herdr commands or shell
scripts to start, watch, approve or wake its agents.

## Done when

- The Foundry skill's Steps 3 to 5 and 8 run through LGTM Studio alone, with no
  `herdr agent` command and no `collect.sh` or `board-waker.sh`.
- The owner answers a permission request and reads a thread from a phone,
  after logging in with the password, through the owner's own reverse proxy
  to the loopback web server.
- A message on the board wakes only the agent that needs to answer it.

## Plans

| Order | Plan slug | What it delivers | State |
|---|---|---|---|
| 1 | lgtm-engine | The engine: agents over ACP, the event log, commands, permissions, waits, accounts, and one authenticated API. | done |
| 2 | lgtm-client-api | Phone alerts, remote access through the owner's proxy, the web page behind one password, and a thread viewer in a Herdr pane. | done |
| 3 | lgtm-message-board | Channels, direct messages and wake rules, behind one delivery interface. | done |
| 4 | lgtm-foundry-roles | The orchestrator, team leads, workers, standing permission rules and the stall watch. | built, apart from standing rules |
| 5 | lgtm-worktrees | Git worktrees for agents under `<repo>/.worktrees/`, through an option on the agent `start` command, with safe clean-up. | absorbed by `website-replacement`, step 1 |

`done` — the plan is built and merged into `master`.

## Out of scope

- Team shapes and workflows. They live outside LGTM Studio (`AGENTS.md`, rule 6).
- Interactive mode, where an agent runs in a Herdr pane. It follows this epic.
- The desktop and phone apps themselves. Piece 2 gives them their API and a
  web page.

## Choices

- 2026-11-03: the owner accepted the three later plans: the client API at
  `b38aa6f` (contract version 17), the message board at `ea7aea5` (contract
  version 15) and the foundry roles at `57b2822`. Git worktrees become their
  own plan, `lgtm-worktrees`. The owner decided that worktrees live in a
  folder inside the repository, such as a `.worktrees` folder that Git
  ignores.
- 2026-11-03: `AGENTS.md` rule 7 changed. The owner decided that the rule
  against committing and pushing applies only to the base branch. Inside a
  team, agents have full authority to commit, push and merge. Only the pull
  request that moves finished work into the base branch needs the owner's
  approval. This repository has no
  `dev` branch, so the rule says "base branch". `engine` counts as a team
  branch, so the manager merges into it, and moving `engine` into `master`
  goes to the owner.
- 2026-10-30: the owner split LGTM Studio into four pieces and chose to build the
  engine first, then the client API, the message board and the foundry roles.
  The engine comes first because the other three are built on its
  capabilities.
- 2026-11-01: the client API needs its own short design round, because the
  first design session designed only the engine.
- 2026-11-03: the three later plans edit the engine directly wherever they
  need to. The owner decided that a phased build edits earlier code instead
  of working around it. This replaces the line of 2026-10-30 that the later pieces are built
  on the engine "without changing it". There is no extension-seam layer and
  no rule that existing behaviour stays the same. A plan may add commands,
  event kinds and a caller kind for services, and may change existing
  behaviour when it needs to. Each such change is recorded in that plan's
  decisions log, its tests are updated, and it is reviewed like an engine
  unit.
- 2026-11-03: the client API design talk changed row 2's scope. The web server
  starts with no login, and pairing with a one-time code moves to a later
  plan. The owner chose this because only the owner uses the service on the
  owner's network for now. The owner also asked that pairing can be added later
  without a major refactor. The done-when line no
  longer names a paired phone. Full decisions:
  `docs/specs/2026-11-03-client-api-design.md`.
- 2026-11-03: the owner replaced that open start, and pairing, with one
  password. The service reads it from a local environment variable, and the
  dashboard asks for it. Pairing codes and per-device credentials come out, and
  there are no rate limits. Mobile app login is later, and it starts by looking
  at what Home Assistant does. The owner decided that one password is enough,
  and that the service needs no support for several users. This supersedes the
  entry above where they differ.
- 2026-11-07: the owner froze development until the website mocks are ready,
  so this epic is parked. Its done-when lines have not been checked end to end.
  The owner asked first to reconcile everything so that `master` holds the
  current version, and then to freeze development until the mocks are ready.
