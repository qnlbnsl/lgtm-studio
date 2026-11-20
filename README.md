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
A browser stays signed in across service restarts for at most 30 days from sign-in.
Up to eight browsers or devices may stay signed in together.
A ninth sign-in ends the oldest session.
Signing out ends only that browser's session.
Changing or removing the password ends every session.
The Terminal tab restores the owner's web shells and recent output after a reload within five minutes.
Another signed-in device can pick up the same shell.
Web shells stay alive while any signed-in owner session remains, subject to the five-minute detached grace limit.
The last session ending closes every web shell.
A displaced tab explains how to take output back and learns when its shell ends.
A service restart closes all shells.
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

### Run a development service beside the stable one

The stable service runs from the main checkout on port 8787.
A change to LGTM Studio itself is tried in a second service, started from the worktree that holds the change.
Run `LGTM_WEB_PASSWORD=<password> ./bin/lgtm-dev <port>` in that worktree.
The command builds the website from that worktree and serves it on `127.0.0.1:<port>`.
It refuses port 8787.
The development service keeps its own state folder, `~/.local/state/lgtm-dev/<name>`, where the name defaults to the worktree folder's name.
It therefore shares no projects, accounts, sessions or web settings with the stable service.
A harness must be signed in again inside a development service before it can run agents there.
Stop it with `./bin/lgtm-dev stop`.
Restarting a development service never interrupts the stable service or its agents.

The website is built from the mock in `design/mock/` by the package `packages/studio`.
An old address under `/studio/` is sent to the root.
See [design/mock/README.md](design/mock/README.md) for what the website does and does not do.

The workspace folder contains the repositories that LGTM Studio's Git commands may use.
Choose it during the first run, or change "Where is your workspace?" under Service settings, in Defaults.
The folder must be an existing absolute directory.
A change that excludes a registered repository is refused with that repository's name and path.
Choose a common parent folder to include repositories in sibling folders.
The client API and command line expose the same setting through `project.settings.set`.

The service command also works without a web password.
In that case, the authenticated Unix socket is available and the web server
stays off.
See [remote access](docs/remote-access.md) before using a proxy or a phone.
An installer, startup at login, and loading project configuration from
`.lgtm` are the next piece of work.

## Repository base branches

Each registered repository records its own base branch.
Registration uses the branch named by `refs/remotes/origin/HEAD` when that branch exists locally.
A malformed remote default is treated as absent.
Otherwise, registration uses an existing current `main` or `master`, or the current unborn branch in a repository without a commit.
The registration reply includes the recorded base branch.
A refusal explains when no usable local default exists.
Older repository records gain only an existing current `main` or `master` on an owner call without changing old events.
This implicit inference never reads `origin/HEAD`.
Selecting another base for an old row requires the explicit setting command.
Both `main` and `master` remain guarded until a base is recorded.
A failed inference is retried after the checkout branch changes or the owner calls a command for that repository.
New repositories created by LGTM Studio use `main`.
The main checkout must have the recorded base checked out before creating a project.

The owner can change the base to an existing local branch through the client API or the command line:

```sh
./bin/lgtm project.repository.base.set --input '{"repoId":"<repository-id>","baseBranch":"dev"}'
```

The command refuses an open review, an unfinished Git operation, an active merge, or an unfinished project creation in that repository.
Changing the setting does not switch the owner's checkout.
The registered workspace binding changes only when a fresh checkout observation in the setting transaction finds the selected branch checked out.
Switch the checkout before changing the setting, or repeat the setting command after switching it.
The website displays the recorded base in its existing repository details.
It does not yet have a control to change that setting.

Before creating a project worktree, the engine fetches the base branch from its configured remote.
It uses the fetched commit when the local base is its ancestor and delivery is confirmed to use a GitHub pull request against that same remote branch.
Local delivery starts from the local base.
The creation intent records its immutable starting commit before creating the worktree.
The project retains that commit after creation and across service restarts.
Review files use the later merge base of the starting commit and the current review target with the project head.
One `git merge-base <head> <target> <start>` call selects the comparison base without merging trees or writing objects.
When the inherited histories diverge, the review may show additional inherited changes while retaining the project's own edits.
Review commits and pending counts exclude history already reachable from that starting commit.
A later loss of hosted delivery still excludes the starting history from the project's commit list.
A missing remote, a failed fetch, or a local base that is ahead or diverged uses the local base instead.
Remote discovery, fetching, and the GitHub authentication probe share a 20-second limit and fall back to the local base on failure.
The authentication probe stays inside the Git gate to keep the fetch and delivery selection in one uninterrupted engine operation.
This bounded serialization can delay another Git operation by up to 20 seconds during the optional start decision.
The completed creation event records the selected source or the fallback reason.
This operation never moves the local base branch or changes the main checkout's files.

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
The optional permission level defaults to `auto`, limited by the saved harness default and an agent starter's own level.
Selecting another agent's checkout still requires an explicit `read-only` level.
Roles parsed as leads or orchestrators keep their supplied folder.
For an agent caller, selecting a checkout other than its own requires `read-only` permission.
The owner can select a registered project checkout at any permission level.
Worker allocation currently requires the local machine.
A clean registered allocation whose harness never opened a session can be reused on a retry by the same starter with the same agent name.
Reuse requires an unchanged source commit and no session or work from any previous attempt.
An agent outside every project keeps the existing folder-based start behaviour.
Manual worktree registration remains restricted to the owner.
On 2026-11-17, the owner allowed lead starts inside home for registered worktrees of the starter's project.
The engine compares real directory identities for the project worktree, team worktrees and engine allocations.
The home folder itself and its ancestors remain refused.
The state folder, plugin root, home dotfolders and configured harness login folders remain protected.
Protection applies to the child's folder.
A clean worker may start beside a protected service checkout under the same registered repository.
The nearest registered workspace sets the project boundary.
Foreign and unregistered nested checkouts cannot inherit a broader grant.
Registered paths must remain canonical directories.
The engine refreshes these records before each spawn, recovery and resume.
Removing a folder's registration makes a later resume refuse with a reason.

The worker's stored folder is reused on recovery and either kind of resume.
A missing folder is reported as unavailable instead of being recreated.
Stopping a worker preserves its folder and branch.
Deleting a team preserves its registered worktrees and branches.
The engine does not automatically remove even a clean, merged worker checkout.
Registered worker worktrees appear in the existing Git, Files, and Terminal choices.


## Finish project work through a pull request

The Git tab's existing merge review prepares a GitHub pull request when the repository has a GitHub.com remote and the service user has a signed-in GitHub command-line interface (CLI), `gh`.
Prepare fetches the live remote base without changing the owner's checkout.
Its comparison, commit list, and file totals use that fetched commit.
A dirty local base can remain behind GitHub while hosted reviews continue.
Prepare pushes only the project's registered branch without force.
Its title uses the project name.
Its description uses the goal and the reported review evidence.
An existing open pull request for that branch and base is reused.
Its description is refreshed with the current reviewed head, base, and evidence summary.
The review area shows its number, link, and state.
Preparing again pushes the reviewed project commits.
Refresh always reads Git state.
It prepares a new review only when the local project head differs from the recorded reviewed head.
A failed Prepare is reported separately from that refresh.
Approval is required for the refreshed review.
Automatic background reads never push.
A binary or oversized comparison refuses before publication without leaving an uncertain operation.
After a successful push, Prepare attempts to update the project branch's remote-tracking reference.
A failed local tracking update does not prevent pull request creation.
Prepare refuses a symbolic base tracking reference before fetching.

Approve merges the reviewed pull request on GitHub with a merge commit.
The engine refuses a changed reviewed head, a remote base that differs from the reviewed base, a closed request, conflicts, or checks and reviews that GitHub has not marked ready.
If the GitHub base moved, the refusal asks the owner to open the merge review again.
That refusal leaves the review open until Prepare replaces it.
A clean local base checkout on its registered base branch is fetched and fast-forwarded.
A dirty checkout or a checkout on another branch is preserved.
The Git tab reports whether that local update completed.

When no suitable GitHub remote is configured, `gh` is missing, or the service user is signed out, Prepare records the existing local merge mode and explains why.
That mode merges locally without pushing the base branch.
A network failure or ambiguous authentication result refuses the request instead of offering a local merge.
After a lost merge receipt, a new owner approval can reconcile GitHub’s confirmed merge without merging twice.
An unverified approval failure leaves the review open.
Prepare can also record an exact-head request already merged elsewhere, including after a prior refusal.
The local update also refreshes the explicit remote-tracking base reference.
Update the remote URL after a GitHub repository rename because hosted delivery does not follow repository redirects.
An explicit pull request creation refusal reports that the branch was pushed and explains how to retry.
There is no setting to force local mode.
Only the authenticated owner can prepare, push, or approve these changes.
See [the project Git guide](packages/engine/src/project/README.md) for the typed client API.

## Agent permission levels

On 2026-11-17, the owner chose `auto` as the default for new project leads and their workers.
The ordered levels are `read-only`, `ask`, `edit`, `auto` and `full`.
New harness defaults use `auto`.
The historical upgrade is applied by field when settings are read.
If the four old permission mappings and reported modes match the previous factory values, the row gains the automatic mapping and mode.
This applies even when the owner edited the model, effort, run mode or adapter path.
A stored `defaultPermission` of `ask` reads as `auto` when the harness reports its automatic or edit mapping.
A custom harness that reports neither mapping keeps `ask`.
Saving an `ask` default that would read as `auto` is refused with the owner's decision of 2026-11-17.
The website never offered a control for that harness field.
A stored default other than `ask` is kept.
A saved project-lead default of `ask` follows the harness default too, unless the owner chose it in the "Defaults for new project leads" permission control.
Custom mappings, custom reported modes and unrelated owner edits are kept.
Existing agents keep their levels.
The stored event log is not rewritten.
Factory adapter discovery runs on the first load of an empty harness list and on every `harnesses reset`.
A reset finds adapters installed since the service started.
Ordinary starts and recovery read current stored settings without repeating discovery.
Claude's automatic mode is `auto`.
Codex's automatic review mode is `agent`.
A harness without a configured automatic mode uses its edit mapping and reports the fallback in its status.
The owner can choose any level.
A child cannot exceed its starter's level.
See [the adapter findings](docs/research/2026-11-17-auto-permissions.md) for the pinned versions and their limits.

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
