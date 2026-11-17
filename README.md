# LGTM Studio

LGTM Studio is a standalone program that runs teams of coding agents as a small
company. An orchestrator directs team leads, and each lead directs workers.
The agents coordinate through a strict message board.
The engine, board, authenticated application programming interface (API), and
web page are the core program.
The product's public site is [lgtmstudio.dev](https://lgtmstudio.dev).

An optional [Herdr plugin](plugins/herdr/README.md) connects the core program
to [Herdr](https://github.com/herdrdev/herdr) for terminal panes and future
interactive sessions.
The core runs without Herdr installed.
LGTM Studio ships a default workflow that people can override.

> [!WARNING]
> LGTM Studio is under development. The engine, authenticated client API, message
> board and foundry supervision code are merged. The owner lifted the development
> freeze on 2026-11-08 for the website build. The findings of two of the
> three foundation reviews are not yet checked against this code. Standing
> permission rules remain unbuilt, pending the owner's harness recordings.
> Real-account and physical phone checks remain separate owner-run work. This
> is not a released or cleared build, and the capabilities below still include
> planned work.

## Where LGTM Studio came from

LGTM Studio is inspired by [Herdr Synapse](https://github.com/vitalysim/herdr-synapse),
created by [Vitaly Simonovich](https://github.com/vitalysim). Synapse showed
that agents inside one Herdr session can work as a team: a shared board, a
human-owned charter, and a notifier that only speaks to an agent when it can
listen. LGTM Studio started as a fork of that idea and then took a different
direction, so it is a separate project with its own codebase.

Thank you, Vitaly, for building Synapse and for releasing it under the
Apache License 2.0.

The optional plugin uses Herdr to host terminal panes.

LGTM Studio borrows ideas, and some code, from
[T3 Code](https://github.com/pingdotgg/t3code) by T3 Tools Inc., released
under the MIT License. The code LGTM Studio uses from it is in
`third_party/t3code/`, and the [NOTICE](NOTICE) file lists every piece of
code taken from another project.

## What LGTM Studio is meant to provide

These are the intended pieces. Each one is tracked in more detail in
[docs/features.md](docs/features.md).

- **The foundry.** The backend that knows about orchestrators, team leads, and
  the different kinds of worker, and that starts, stops, and reassigns them.
  The [default role instructions](docs/roles.md) reach each agent in its first turn.
  The owner can override their files under `~/.lgtm/roles`.
- **A strict message board.** Channels per team, direct messages, and clear
  rules for who gets woken by which post. The board's first job is to save
  turns and tokens.
- **Threads.** Every agent's conversation is visible and steerable as a
  thread, in the spirit of [T3 Code](https://github.com/pingdotgg/t3code).
- **A client API.** One authenticated API that the web page, a future desktop
  app, and a future mobile app all use. Nothing a client can do is reserved
  for the terminal.

LGTM Studio comes with one default workflow, the
[build loop](docs/specs/2026-11-05-build-loop-design.md). Anyone who wants a
different workflow overrides it in `~/.lgtm`, or for one repository in that
repository's `.lgtm`. Particular team shapes (which teams exist, what each
role is told to do) are not part of LGTM Studio. They live elsewhere and are loaded
into it.

## Start LGTM Studio without Herdr

This checkout needs Node.js 26 and pnpm.
From the repository root, select Node.js and install the dependencies:

```sh
nvm use
pnpm install --frozen-lockfile
pnpm --filter @lgtm/studio build
```

Start the service with a web password in its environment:

```sh
read -rs LGTM_WEB_PASSWORD
export LGTM_WEB_PASSWORD
./bin/lgtm serve --detach
unset LGTM_WEB_PASSWORD
```

The `read` command above works in Bash and Zsh.
Enter a long, random password at its prompt, then press Enter.
Open `http://127.0.0.1:8787` and log in with that password.
The web password must contain at least eight characters.
The engine's default state folder is `~/.local/state/lgtm`.
Set `LGTM_HOME` on the service and command processes to use another folder.
With `serve --detach`, the service continues running after the launching terminal closes.
Use `./bin/lgtm serve` to keep it in the foreground and read its redacted logs in the terminal.
Press Ctrl-C or send the termination signal, SIGTERM, to shut it down cleanly.
The hangup signal, SIGHUP, also shuts it down.
The command exits zero only after a clean shutdown.
The attached service also stops if its launcher dies.
The foreground command refuses to start if a service already uses the same state folder.
The terminal thread viewer is available through `./bin/lgtm view`.

The website is built from the mock in `design/mock/` by the package `packages/studio`.
An old address under `/studio/` is sent to the root.
See [design/mock/README.md](design/mock/README.md) for what the website does and does not do.

The service command also works without a web password.
In that case, the authenticated Unix socket is available and the web server
stays off.
See [remote access](docs/remote-access.md) before using a proxy or a phone.
An installer, startup at login, and loading project configuration from
`.lgtm` are the next piece of work.

## Worker worktrees

A worker started inside a project receives its own registered Git worktree.
The engine creates its folder under the repository's `.worktrees/` folder.
The branch name has the form `worker/<project>/<agent>`.
The engine makes safe slugs from the names and adds a number when a name is taken.
The branch starts at the head of the starter's registered worktree.
Uncommitted changes in the starter's checkout are not copied into the worker's folder.
The new worktree records that starter's worktree as its parent.
The existing Git integration action merges the worker into that parent.
The project's final merge into the base branch still requires the owner's review.

The `start` client API accepts an optional `projectId` for owner starts.
An agent's start inherits its project membership.
The optional `worktreeId` selects an existing registered worktree in that project.
A reviewer or evaluator can use that field with the existing
`read-only` permission to inspect another worker's checkout.
Before starting a test runner, the lead merges the builder's branch into its own worktree.
The test runner starts without `worktreeId` and receives its own worktree at the lead's last commit.
Its brief names an authorized writable permission and a log folder inside its own worktree.
The permission level is still an explicit start option.
Roles parsed as leads or orchestrators keep their supplied folder.
For an agent caller, selecting a checkout other than its own requires `read-only` permission.
The owner can select a registered project checkout at any permission level.
Worker allocation currently requires the local machine.
A clean registered allocation whose harness never opened a session can be reused on a retry by the same starter with the same agent name.
Reuse requires an unchanged source commit and no session or work from any previous attempt.
An agent outside every project keeps the existing folder-based start behaviour.
Manual worktree registration remains restricted to the owner.
The existing folder ceiling refuses agent-started children inside the owner's home folder.
Lead-started workers therefore need a repository outside that protected folder.

The worker's stored folder is reused on recovery and either kind of resume.
A missing folder is reported as unavailable instead of being recreated.
Stopping a worker preserves its folder and branch.
Deleting a team preserves its registered worktrees and branches.
The engine does not automatically remove even a clean, merged worker checkout.
Registered worker worktrees appear in the existing Git, Files, and Terminal choices.


## Finish project work through a pull request

The Git tab's existing merge review prepares a GitHub pull request when the repository has a GitHub.com remote and the service user has a signed-in GitHub command-line interface (CLI), `gh`.
Prepare pushes only the project's registered branch without force.
Its title uses the project name.
Its description uses the goal and the reported review evidence.
An existing open pull request for that branch and base is reused.
The review area shows its number, link, and state.
Preparing again pushes the reviewed project commits.
Refresh always reads Git state.
It prepares a new review only when the local project head differs from the recorded reviewed head.
A failed Prepare is reported separately from that refresh.
Approval is required for the refreshed review.
Automatic background reads never push.

Approve merges the reviewed pull request on GitHub with a merge commit.
The engine refuses a changed reviewed head, a remote base that differs from the reviewed base, a closed request, conflicts, or checks and reviews that GitHub has not marked ready.
A clean local base checkout on its registered base branch is fetched and fast-forwarded.
A dirty checkout or a checkout on another branch is preserved.
The Git tab reports whether that local update completed.

When no suitable GitHub remote is configured, `gh` is missing, or the service user is signed out, Prepare records the existing local merge mode and explains why.
That mode merges locally without pushing the base branch.
A network failure or ambiguous authentication result refuses the request instead of offering a local merge.
After a lost merge receipt, a new owner approval can reconcile GitHub’s confirmed merge without merging twice.
There is no setting to force local mode.
Only the authenticated owner can prepare, push, or approve these changes.
See [the project Git guide](packages/engine/src/project/README.md) for the typed client API.

## Documents

- [AGENTS.md](AGENTS.md): rules for coding agents working in this repository.
- [SECURITY.md](SECURITY.md): how to report a vulnerability, and the security
  rules the design must keep.
- [docs/features.md](docs/features.md): the feature list.
- [docs/notifications.md](docs/notifications.md): email, ntfy, quiet hours, and notification API actions.
- [docs/issues.md](docs/issues.md): the running list of problems found.
- [docs/specs/2026-11-01-engine-design.md](docs/specs/2026-11-01-engine-design.md):
  the design of the engine, the first of LGTM Studio's four pieces.
- [docs/research/acp.md](docs/research/acp.md): what the Agent Client
  Protocol is and how it relates to Herdr and to LGTM Studio's client API.
- [docs/research/t3-code.md](docs/research/t3-code.md): how T3 Code compares
  with Herdr and Synapse, and what LGTM Studio should borrow from it.
- [docs/research/herdr-and-synapse.md](docs/research/herdr-and-synapse.md):
  what LGTM Studio keeps from Herdr and Herdr Synapse, and the failures it must
  prevent.

## Licence

LGTM Studio is free software.
Its licence is the GNU Affero General Public License, version 3 only.
The full text is in [LICENSE](LICENSE).
Copyright (C) 2026 Kunal Bansal.

The licence lets anyone use, change and share LGTM Studio.
Anyone who shares a changed version must share its source code under the same licence.
Anyone who runs a changed version as a network service must offer its source code to the people who use that service.
Every copy must keep the copyright lines and the [NOTICE](NOTICE) file.

Code copied from other projects keeps its own licence.
The [NOTICE](NOTICE) file lists each such project, and the licence texts are under `third_party/`.

The licence does not grant the use of the name "LGTM Studio" for a changed version.
