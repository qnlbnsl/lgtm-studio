# Security Policy

## Supported versions

LGTM Studio has no release yet. Until the first tagged release, only the current
`main` branch receives security fixes. After that, the latest tagged release
and `main` will receive them.

## Reporting a vulnerability

Do not report a vulnerability in a public issue.

Use GitHub's private vulnerability reporting for this repository, at
`https://github.com/qnlbnsl/lgtm-studio/security/advisories/new`. If that page is
not available, email the maintainer at qnlbnsl@gmail.com with "LGTM Studio
security" in the subject line.

Include these details:

- The LGTM Studio version or commit and the platform.
- The Herdr version, if the optional plugin is involved.
- The impact: what an attacker can read, change, or run.
- A minimal reproduction.

Redact message boards, local paths, credentials, API tokens, the web password,
session cookies, and raw provider responses before you send anything.

## What LGTM Studio protects, and what it does not

LGTM Studio runs with the permissions of the local user who starts it.
It is not a sandbox. The coding agents it manages can run commands and change files as
that user, and LGTM Studio does not stop them. Run untrusted repositories inside
an external sandbox.

Agents can read LGTM Studio's own files. The agents run as the owner's user, so
any agent can read LGTM Studio's state folder, including the owner's credential,
the stored account keys and the event log. File permissions cannot stop a
program that runs as the same user. LGTM Studio's permission rules (the
permission ceiling, the folder rule and the approval routing) therefore stop
an agent's mistakes, not a deliberately hostile agent. Where a harness
supports it, LGTM Studio adds a rule that forbids its agents to read the state
folder, but that rule is a guard against accidents, not a wall. The owner
accepted this limit on 2026-11-01. The real fix is to run each agent in a
sandbox, so LGTM Studio's private files are out of its reach. The owner chose
this on 2026-11-05, in place of the earlier plan to run LGTM Studio as its own
user. It is still planned. On 2026-11-03 the owner approved building remote
access before it, so that LGTM Studio is in use sooner. Until it is built, any
agent can read LGTM Studio's state folder, including the stored account keys.

On 2026-11-17, the owner approved a narrow exception to the home-folder rule.
An agent-started child may work inside home only beneath a currently registered worktree of its starter's own project.
Only the project worktree, team worktrees and engine-allocated worker worktrees grant this exception.
A manually registered descendant alone grants nothing.
Real paths are resolved and compared by device and inode along their ancestors.
The existing parent-folder and same-repository checks still apply.
Selecting another agent's checkout still requires read-only permission.
The home folder itself and any folder containing it remain refused.
The state folder and plugin root remain refused, together with their ancestors and descendants.
The engine enumerates immediate home dotdirectories and resolves directory aliases without reading their contents.
These directories and existing configured account login folders receive the same protection.
Login paths include account homes and harness home-variable overrides, including resolved secret references.
They also include login-store fallbacks under an alternate `HOME` and Cursor's `XDG_CONFIG_HOME`.
The child's own folder must pass the protection checks, including the Linux mount-table check.
A protected sibling elsewhere in the registered root does not refuse a clean child folder.
The root of the main checkout containing the service remains refused as a child folder.
A clean subfolder such as `<repository>/packages` is admitted when the starter is already in that main checkout and the other rules pass.
A worker allocated beside the service checkout may start.
These worktrees share Git's common directory, including configuration and hooks, with the service checkout.
The folder rule does not isolate that shared metadata.
Before allocation, the source may contain a protected folder but cannot be one or be inside one.
That preliminary check returns no spawn grant.
Before creating a worker checkout, the engine also checks the allocation directory and Git common directory against protected identities.
Neither may be a protected folder or be inside one.
A missing allocation directory is checked through its nearest existing ancestor before Git can create it or change the exclusion file.
A protected sibling in the allocation directory remains allowed.
The allocated child receives ordinary admission before registration and again before spawn.
The nearest registered workspace across all projects sets the boundary, including outside home when no own-project root contains the folder.
A nested foreign or unregistered checkout cannot inherit a broader project root's grant.
Registered paths must still be canonical real directories.
Relative login paths and regular files are excluded from login-directory discovery.
An unreadable immediate home dotfolder fails closed for every agent-started child, including outside home, recovery and resume.
The reason is `a home dotfolder cannot be checked`.
An unresolved directory alias could point into the requested folder, so identity comparison cannot prove it unrelated.
The owner must restore access or remove the stale entry before retrying.
The owner caller retains its exception.
The engine reads the current project views again before every spawn, recovery reload and resume.
The allocator may validate its own pending registration at the same log head before the atomic append.
After that append, the grant relies exclusively on the committed registration.
A later registration removal makes a later resume refuse with a sentence reason.
The owner caller keeps the existing exception.
The spawn race and non-Linux mount limits in issue 49 remain unchanged.

The owner also chose the `auto` default on 2026-11-17.
The owner's decision of 2026-11-17 accepts the harness's own reviewer approving routine commands at `auto`.
Only requests the harness raises reach the engine and its owner-only routing for persistent settings.
A command approved within the harness can change `.claude/settings.json` or `.mcp.json` without an owner prompt.
Codex's automatic reviewer may approve a sandbox escalation without asking the owner.
Automatic review may also refuse an action.
The folder ceiling limits the start folder, not the paths touched by approved commands.
The declared permission-level ceiling still applies to child starts and modes the adapter reports.
The scale is `read-only`, `ask`, `edit`, `auto`, then `full`.
An omitted child permission uses the lowest of `auto`, the saved harness default and its starter's level.
Existing agents retain their levels.
The historical upgrade applies by field at read time without rewriting stored events.
A row whose four old mappings and reported modes equal the previous factory values gains the automatic mapping and mode.
Model, effort, run-mode, adapter-path and other unrelated edits do not block that upgrade.
Custom mappings and custom reported modes are kept.
A stored `defaultPermission` of `ask` reads as `auto` when the harness reports its automatic or edit mapping.
A custom harness that reports neither mapping keeps `ask`.
This upgrade includes a factory-identical row that the owner previously kept at `ask`.
Saving an `ask` default that would read as `auto` is refused with the owner's decision of 2026-11-17.
The website never offered a control for that harness field.
A stored default other than `ask` is kept.
The saved project-lead default follows the same decision.
A saved lead level of `ask` reads as the harness default when the harness reports a mode for that default, unless the owner chose `ask` in the "Defaults for new project leads" permission control.
A lead level named in a project creation request is the owner's explicit choice and is kept.
Only the owner can save lead defaults or create a project.
Factory adapter discovery runs on the first load of an empty harness list and on every `harnesses reset`.
A reset finds adapters installed since the service started.
Ordinary starts and recovery read current stored settings without repeating discovery.
An older harness mapping without `auto` falls back to its `edit` mode.
Harness status reports that fallback.

The harness installation receipts are in that state folder.
A hostile process running as the same user can rewrite a receipt and make a
changed program pass the "changed since install" check.
That check guards against accidents and changes by install scripts.
It does not stop a hostile process running as the same user.
Every input the installation verification reads is in a folder that a process running as the same user can write.
Such a process can restore the staged files and rewrite npm's hidden lockfile before the check, or rewrite the program after extraction.
It can therefore still have a different program installed, run by the engine, recorded in the receipt, and proposed as the harness.
The check stops accidents, install scripts, and a swap that is not restored.
It is not a wall against a hostile process running as the same user.
The real fix is the planned sandbox for agents.

Agents can also read the web password. Agents run as the owner's user, so an
agent can read the engine's first environment through `/proc`, even though the
engine deletes the variable and agents do not inherit it. The password protects
against the network. It is not a boundary against an agent on the same user
until the sandbox exists. The owner accepted this limit on 2026-11-03.

The part of LGTM Studio that needs the most care is its client API, because it
lets a web page, a desktop app, or a phone control agents that can run
commands on the host. Anyone who can reach that API with a valid credential
can, in effect, run code on the machine. On the web server, that credential is
one password.

## Harnesses with no deny-read or deny-write rule

The rule above, which forbids an agent to read the state folder, needs the
harness to accept it. The same is true of the rule that forbids an agent to
change its own harness settings. In LGTM Studio's default settings, no harness
gets either rule today. The owner can set a launch rule for a harness in its
settings once one is verified.

- **`claude`.** Its adapter accepts permission rules only inside the
  session's metadata, and LGTM Studio's settings contract cannot express rules
  there yet. See issue 91 in `docs/issues.md`.
- **`codex`.** No launch-argument rule for it has been verified and recorded,
  so its default is no rule. See issue 104 in `docs/issues.md`.
- **`gemini`.** No launch-argument rule for it has been verified and
  recorded, so its default is no rule. See issue 104 in `docs/issues.md`.
- **`cursor`.** No launch-argument rule for it has been verified and
  recorded, so its default is no rule. See issue 104 in `docs/issues.md`.

For these harnesses, the folder rule, the permission ceiling and the approval
routing are the only guards. They stop an agent's mistakes, not a hostile
agent.

## Security rules the design keeps

These rules apply to every change. A change that breaks one is a security bug.

1. **The API listens on the loopback address by default.** Listening on any
   other address is an explicit setting that the owner turns on.
2. **Every API request is authenticated, including on loopback.** There is no
   unauthenticated mode. The engine's Unix socket uses the owner credential and
   agent tokens. The web server uses one password, which the service reads from
   the local environment variable `LGTM_WEB_PASSWORD`, and the session that a
   correct password earns. A logged-in session is the owner. If the variable is
   not set, the web server does not start. Only the login route and the page's
   static files, which hold no data, are served with no session.

   The loopback port has a limit of its own. While the web server is down, or
   when it never started, another local user can bind `127.0.0.1` at its port,
   because that needs no rights. That program can show a fake login form and
   capture the password. In proxy mode, the proxy sends the phone's requests to
   it, and it can serve a page with a long cache lifetime under the real public
   address. `docs/remote-access.md` says that a `client.web-down` alert means the
   page must not be opened. A local program can also send any header, so the
   `Host` and `Origin` checks stop web pages and not local programs.
3. **The password is checked in constant time and is not kept by the browser.**
   A login carries it in the request body, never in a URL. One person uses
   LGTM Studio, and the owner chose no rate limits and no lockouts on 2026-11-03.
4. **Up to eight web sessions survive service restarts for at most 30 days each.**
   The owner approved survival across a service restart on 2026-11-17.
   Several signed-in browsers at once preserve the behaviour from before this unit.
   Each sign-in creates an independent random 256-bit token.
   A ninth sign-in evicts the oldest session.
   Signing out ends only the presenting browser's session.
   Changing or removing `LGTM_WEB_PASSWORD` invalidates every session before the listener starts.
   A successfully committed invalidation survives restoring an earlier password.
   Restoring an older copy of the state folder can restore signed-out sessions while their original password and 30-day lifetime still match.
   There is no revocation authority outside that folder to detect a restored backup.
   Each session expires 30 days after its own sign-in.
   The browser holds its token in an `HttpOnly`, `SameSite=Strict` cookie with that maximum age.
   The cookie is `Secure` on a public request when the public address is `https`.
   The private `web-session.json` file holds one record per session.
   Each record contains only a Secure Hash Algorithm 256 (SHA-256) token verifier, a creation time, an independent random 16-byte salt, and a keyed password binding.
   The binding key comes from scrypt with fixed parameters N=65536, r=8, and p=1.
   The binding authenticates a fixed domain label, the token verifier, and its creation time.
   Version 3 records replace the earlier development format, which is refused rather than migrated.
   These scrypt parameters deliberately use less work than the common N=131072, r=8, p=1 recommendation.
   This unit keeps N=65536 to bound synchronous startup verification of at most eight records to about two seconds on the review host, with about 64 mebibytes per derivation.
   The derivations run before the listener starts, and sign-in performs one derivation.
   This choice trades password-guessing cost for startup latency and does not claim to meet that higher recommendation.
   The reference is the [Open Worldwide Application Security Project password storage guidance](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html).
   Checking a stored binding requires about 64 mebibytes of derivation memory for each password guess.
   A file reader cannot forge a cookie from a verifier or test a password with a single fast digest.
   The service verifies every password binding at startup.
   It checkpoints the validated collection before accepting any cookie.
   If that checkpoint and invalidation both fail, this service accepts no session and refuses sign-ins until restart.
   If all writes fail, the service cannot promise durable invalidation of the unchanged file.
   A later restart with an earlier password can then accept an unchanged record if storage becomes writable.
   Repairing storage and restarting with the intended password is required to commit invalidation.
   Each update replaces the entire bounded collection through an exclusive mode-0600 temporary file, a file flush, a rename, and a directory flush.
   Memory changes only after that update succeeds.
   A failed update attempts an empty snapshot and then a mode-zero fallback before rejecting the old in-memory sessions.
   If storage cannot be invalidated, the request fails without claiming a successful revocation or clearing the pre-operation memory state.
   Invalid, expired, unreadable, wrongly permissioned, symbolic-link, hard-linked, oversized, or more than five minutes future-dated records do not authenticate a browser.
   Startup skips only an authenticated record that exceeds five minutes of clock skew.
   Other valid sessions in the same file survive.
   A backward clock step while running does not expire or delete a record.
   The 30-day boundary still uses wall-clock time, so a backward clock step can delay expiry.
5. **An encrypted channel for remote access is the owner's setup**, such as TLS, a
   Tailscale tailnet, or an SSH tunnel. LGTM Studio serves plain HTTP on loopback and
   does not offer it to a non-loopback address. A configured public address must use
   HTTPS, even inside a private network, by the owner’s decision on 2026-11-04.
   The owner supplies the external proxy's certificate. The owner's later
   clarification on 2026-11-04 accepts a self-signed certificate and continuation
   past the expected browser certificate warning. Certificate trust installation
   is not required. LGTM Studio holds no certificate and performs no trust setup.
   An SSH local forward provides interim access at `http://127.0.0.1:<port>`, with identical ports on
   the device and server. `localhost` remains refused. The owner’s earlier words,
   on 2026-11-03:
   "Let's just focus on plain HTTP. That's what the software itself is doing, right? Then, whatever proxy we add on to it, that proxies to localhost. We'll figure this out then. Because I have to set up a local certificate authority so that I can do ARPA DNS certs"
6. **Secrets never reach logs, the message board, or a client.** This covers
   API tokens, the web password, the session token, and the agent harnesses'
   own login tokens. Agents do not inherit the web password's variable.
   Explicit secrets from `secrets.json`, resolved `secret:` variables,
   harness login files, the web password, and owner and agent credentials stay
   registered at any length.
   Registered values are matched raw, JSON escaped, in base64 and base64url,
   in partial base64 runs of at least eight characters, URL encoded, and in hexadecimal.
   The bounded two-step forms are double JSON escaping, base64 of the JSON-escaped value,
   JSON escaping of base64, URL encoding of base64, and base64 of base64.
   Encodings are not composed recursively.
   Double base64 also matches bounded partial runs at all three inner and outer byte alignments.
   Each partial run still requires at least eight characters.
   Contiguous registered runs can match inside a larger encoded document.
   For a value-known secret, GNU base64 output at 76 columns with line-feed separators is also matched.
   The full forms cover single encoding and the default double-encoding pipelines from `printf` and `echo`.
   They allow the final output line feed to be removed.
   They also cover an unwrapped outer encoding of that wrapped inner output.
   These full pipeline forms add at most six entries per secret.
   The first and last inner line segments at each byte alignment add at most 36 partial entries.
   Both inner and outer partial runs still require eight characters.
   The complete bounded set attempts at most 78 insertions per secret.
   At least one insertion is always a duplicate, leaving at most 77 distinct entries.
   This is not general decoding of line-wrapped documents.
   Other wrap widths, carriage-return separators, and arbitrary line phases inside larger documents are not covered.
   Middle line segments are not independently registered.
   Mixed wrapping between encoding steps is outside this covered set.
   For example, `printf %s "$T" | base64 -w0 | base64` can publish part or all of the encoded secret.
   A secret inside a larger document encoded with default base64 wrapping is not reliably hidden.
   This includes `printf 'key=%s\n' "$T" | base64` and an encoded credentials file.
   The full GNU forms apply when the registered value is the encoding input.
   Other document content changes the line positions and can break every registered run.
   Hexadecimal output wrapped by `xxd -p` is not matched in general.
   A wrapped document can therefore escape matching when it contains none of the registered contiguous or full pipeline forms.
   The two-step list does not include hexadecimal combined with another encoding.
   It does not cover every mixture of standard and URL-safe base64 alphabets.
   Base64 of the JSON-escaped value has no additional partial-run forms.
   JSON escaping of base64 uses the spelling produced by `JSON.stringify`.
   Optional slash escapes in that text are not an additional matched spelling.
   URL encoding of base64 matches its literal and fully encoded spellings.
   An arbitrary mixture of escaped and unescaped base64 characters is not covered.
   A token known only by its stored hash is matched raw and in the supported single encodings.
   The two-step forms require its value to be registered too.
   The GNU wrapped forms also require the value to be registered.
   A streamed agent token's public prefix may be published as match context.
   Every public-prefix occurrence in the bounded candidate window remains eligible.
   A prefix inside a token cannot displace an earlier candidate.
   The private remainder stays withheld while it could still complete a registered token.
   A continuation that rules out the candidate releases it.
   A complete registered token is replaced.
   An incomplete candidate is released when its redaction item or turn ends.
   Forced withholding instead replaces its pending characters with a fixed marker.
   Its match context and completed coverage remain available for later chunks.
   Published public characters remain match context even when a value-known secret also has a matching suffix.
   Whole-text output may be `[REDACTED]` where streamed output is `shp_[REDACTED]`.
   A value inferred only from an inherited environment variable whose name
   matches `KEY`, `TOKEN`, `SECRET` or `PASSWORD` is registered only at eight
   or more characters when its trimmed text is nonempty, is neither `true`
   nor `false`, and is not a plain number.
   On 2026-11-14, the owner authorized ephemeral output for the project terminal and the harness sign-in terminal.
   The owner ruled: "what you echo on terminal is none of our business. just keep it away from log. if they use tmux or something and it hold this in the buffer then that is not on us."
   On 2026-11-17, the owner approved reattachment after a page reload by the signed-in owner.
   Sharing web project terminals among that owner's signed-in browsers is this unit's design choice for a single-owner product.
   Harness sign-in output remains restricted to its opening owner connection.
   Both terminals send the program's output byte for byte, with no redaction, hold, or idle timer.
   Project terminals retain at most 64 kibibytes of recent output in memory for reattachment.
   Neither output nor typed keys are written to disk, logged, traced, posted, sent as alerts, or included in error messages, crash reports, snapshots, or protocol logs.
   A secret printed in either terminal appears on the owner's screen and in any scrollback the owner's own tools keep.
   Rule 6 continues to apply without these exceptions to every other path.

7. **Text from an agent is data, not a command.** A message on the board
   cannot change permissions, teams, or settings unless the owner's
   credential approves it.

## The owner's harness sign-in terminal

The service runs only the shipped recipe arguments with the verified harness executable.
The sign-in request cannot supply a command, executable, argument, or shell expression.
For Gemini without a managed install, the executable comes from the owner's harness setting `adapterPath`.
The service uses the same login folder that the account gives its agents.
Resolved `secret:` values in the account environment are passed to the child.
Only the initiating owner connection may read output, type keys, resize the terminal, or stop it.
Sign-in terminals do not reattach after reload because their credential flow stays bound to that initiating connection.
Their five-minute total time limit and five-second connection grace remain unchanged.
Signing out closes only harness sign-ins initiated by that web session.
Local socket sign-ins and other web sessions' sign-ins remain unaffected.
Agent and service credentials cannot use these requests.
The service kills a live process group when the owner stops it, the connection grace period ends, the time limit passes, or engine shutdown begins.
After the child exits, the service does not signal its former process group because the identifier may be reused.
A descendant that starts its own session escapes the group kill.
Output and input stay ephemeral.
The owner's ruling of 2026-11-14 applies to this stream too.
The program's output reaches the opening connection byte for byte, with no redaction, hold, or idle timer.
Neither output nor input is stored, logged, traced, posted, sent to other subscribers or agents, or included in errors or snapshots.
A secret printed here appears on the owner's screen and in any scrollback the owner's own tools keep.
Removing an account preserves its login folder.

## The owner's project terminal

On 2026-11-14, the owner granted a narrow exception to rule 6 for project shells.
The stored project terminal switch starts off in Service settings.
While it is off, every open request is refused before a shell can start.
Only an authenticated owner may open a terminal.
Agent and service credentials are refused.
On 2026-11-17, the owner approved terminal reattachment after a page reload.
Sharing web terminals across authenticated sessions is this unit's design choice for the single-owner product.
Any such session may list the web project terminals, read output, type into them, resize them, or close them.
A new output subscription takes over from the previous reader and replays recent output.
The previous stream ends cleanly even when its connection is half-open.
The last reader wins, while both signed-in connections may still type.
The displaced tab uses the existing notice bar to explain how to take output back.
While that tab displays the shell, it checks the existing terminal list once per second and learns when the shell ends.
Opening the Terminal tab still attaches listed shells automatically.
An agent, a service credential, an unauthenticated caller, or a local socket connection cannot use web terminals.
A local socket terminal remains bound to its opening connection.
A web session cannot use a local socket terminal.
The request names a project and a registered worktree by their engine identifiers.
It cannot supply a path, command, executable, argument, environment variable, or shell expression.
Only a worktree on the local machine can open a shell.
The executable is the owner's shell from the service environment, when it is an absolute path listed in `/etc/shells`.
Otherwise, the executable is `/bin/sh`.
The shell inherits the service environment with every `LGTM_` entry removed.
An environment entry containing any registered secret is removed too.
At most eight project terminals may be live at once, including detached terminals.
A detached web terminal remains available for five minutes.
The service closes it after that grace period.
A local socket terminal keeps its five-second connection grace.
A shell has no time limit.
The service kills its live process groups on close, switch-off, connection loss past the grace period, and engine shutdown.
A web shell survives while any signed-in owner web session remains, subject to its detached grace period.
The remaining browser may be typing without reading or reloading.
Cleanup assigns a surviving shell to a remaining session without requiring a current reader.
When its reader signs out, the shell detaches with its usual grace period.
The last web session's sign-out closes all web shells.
Web sign-in, sign-out, eviction, and expiry do not close local socket shells.
On Linux, the service reads `/proc` to find and kill jobs in the shell's operating-system session.
It does this when the shell exits too.
After the shell exits, it signals only processes still found in that session.
It never signals the former shell process identifier or its former process group.
Where `/proc` is absent, descendant discovery does nothing.
On those systems, only the live shell's process group is killed.
A descendant that calls `setsid` to leave the shell's operating-system session survives this cleanup.
The scan stops when no member is found or after five passes.
A process that continues to fork beyond that bound may survive.
If the former leader's process identifier exists again after exit, discovery stops without signalling that new session.
Input and output are ephemeral.
The service retains at most 64 kibibytes of recent output per project terminal in memory.
Reattachment sends that recent output before subsequent live output.
A detached shell pauses reads when its replay ring first turns over.
Kernel backpressure then bounds processing until reattachment or the grace limit.
Reattachment resumes reads even when the retained output has no complete escape-safe prefix.
Input is not retained for replay.
Neither stream is written to disk, logged, posted, sent as events, or included in snapshots or protocol logs.
The owner's ruling of 2026-11-14 covers both this stream and the harness sign-in stream.
The program's live output reaches the attached connection byte for byte, with no redaction, hold, or idle timer.
The output goes only to the attached authenticated owner web session or the opening local socket connection.
It never goes to an agent.
Neither output nor typed keys are traced or included in an error message.
A secret printed in the terminal appears on the owner's screen and in any scrollback the owner's own tools keep.
The shell runs as the owner and can change files with that user's permissions.
Rule 6 remains unchanged for every other path.

## Repository bases and creation-time fetching

Only an owner call may infer and record an old repository's base branch.
Agent reads never append a base-setting event and never use the shared `refs/remotes/origin/HEAD` to change the effective base.
An unrecorded repository protects both `main` and `master` from ordinary integration and Files editing.
Either conventional branch remains eligible as the review destination in the repository's main checkout.
Implicit inference records only an existing current `main` or `master` branch.
It never consults the agent-writable `origin/HEAD`, including on the owner's next read after an agent changes that reference.
Only explicit owner registration may use the branch named by `origin/HEAD` when it exists locally.
Registration treats a malformed remote default as absent.
Its fallback accepts an existing current `main` or `master`, or an unborn current branch.
The registration reply reports the recorded branch.
A feature checkout alone cannot become an old row's recorded base.
The owner may explicitly select another existing local branch through `project.repository.base.set`.
That command refuses open reviews, unfinished Git operations, active merges, and unfinished project creations.
It never moves a branch or changes a checkout.
It re-reads the bounded main-checkout `HEAD` within the setting transaction.
It refreshes the main workspace binding only when that observation finds the selected branch checked out.
That read validates the metadata path and refuses symbolic links at the file.

Project creation fetches only the configured base branch into its explicit remote-tracking reference.
Remote discovery and the fetch share a 20-second deadline.
Failures fall back to the observed local commit and record a bounded explanation.
The fetch disables interactive prompts, external transport protocols, tag following, pruning, submodule recursion, automatic HTTP redirects, configured destination ref mappings, and custom remote upload programs.
Only Hypertext Transfer Protocol Secure (HTTPS) and Secure Shell (SSH) transports are enabled for that command.
A symbolic destination tracking reference is refused before fetching.
The fetch updates no local branch and writes no main-checkout files.
Only a confirmed GitHub delivery against that same remote branch may start from the fetched commit.
Local delivery starts from the local base.
The creation intent stores the selected immutable starting commit before the worktree write.
The starting remote is selected from repository configuration.
Local review hides the commit history that this remote base held when the project was created.
The comparison treats that configured remote's starting history as inherited work.
A later change to local delivery preserves this commit-history boundary.
File comparisons use one merge-base query without merging trees or writing objects.
Divergent inherited histories may add visible changes while preserving the project's own edits.

The fetch still honours repository `core.sshCommand`, credential helpers, SSH configuration, and the owner's proxy configuration.
Repository configuration can therefore run code as the service user or redirect traffic during creation as well as delivery.
These options bound the command and its reference writes; they do not isolate hostile shared Git metadata.
The owner must trust that configuration until agents run in the planned external sandbox.
No existing permission or redaction rule is relaxed by this fetch.

## Owner settings for worktree preparation

Only an authenticated owner may replace a repository's preparation settings with `project.repository.preparation.set`.
The engine stores the settings on the repository record and never reads them from repository content.
Each project creation uses the file and submodule settings recorded in its creation intent.
Each worker allocation uses the current repository record.
Agent starts and instruction restoration include the saved plain text through the existing redaction, slash escaping and composition size limit.
The heading identifies the text as the owner's repository settings.
The engine does not interpret that text as a command.
Repository scripts and session hooks are never a preparation step run by LGTM Studio.

Copying admits at most 16 relative patterns with at most one wildcard in a single segment.
Parent traversal, absolute paths, backslashes, control characters and `.git` segments are refused.
A wildcard match receives the same path validation.
Each scan visits at most 512 directory entries.
The operation accepts at most 128 regular files, one mebibyte per file and eight mebibytes in total.
It skips missing paths and nonfiles.
It refuses symbolic links at either end, including directory components.
Linux directory descriptors through `/proc/self/fd` anchor traversal.
Each component is opened without following symbolic links.
An unsupported host refuses configured copying.
The destination is created exclusively inside the new worktree.
An existing regular file is kept without a write.
A new file keeps only the source's owner permission bits.
Copied bytes and rejected paths never enter an event, reply or diagnostic.
An unsafe path or exceeded copy limit refuses preparation with a fixed sentence.
Project creation retains its incomplete worktree for inspection and retry.
Worker allocations keep their existing rollback policy.
These descriptors do not isolate a hostile process running as the same user and moving open directories.
The existing sandbox limitation still applies.

Submodule initialization runs a fixed argument vector without a shell.
It uses the existing hardened Git process wrapper with a 20-second deadline and 64 kibibytes of output.
The operation allows only Hypertext Transfer Protocol Secure (HTTPS) and Secure Shell (SSH) transports.
It disables hooks, external diff helpers, file-system monitoring, interactive prompts and automatic HTTP redirects.
It disables recursive submodule fetching.
It forces checkout mode so a configured submodule update command cannot execute.
It does not use `--recursive` or `--remote`.
A failure records only a fixed sentence and never prevents creation from completing.
Git still uses the owner's credential helpers and SSH configuration.
Shared Git configuration and checkout filters retain the trust limits described above.

Gitlink entries carry commit identifiers instead of file contents.
Git views validate the pointer path without entering the nested checkout as a regular file.
The Files interface continues to exclude nested checkouts.
Discard admits only an initialized, clean submodule whose metadata resolves into this repository's module metadata.
It restores the index pointer with a fixed detached checkout of an already-local commit.
That checkout disables transports and refuses to overwrite ignored local files.
It never fetches.
The discard fingerprint includes the index pointer and the nested head.
A dirty, changed or uninitialized submodule is refused before a destructive checkout.
No standing security rule is weakened by these settings.

## The owner's GitHub pull requests

The owner authorized GitHub pushes and pull requests on 2026-11-17.
Only an authenticated owner connection can prepare or approve the project's main review.
Agent credentials and service credentials cannot invoke those operations.
The agent tool list does not expose an owner merge tool.
The engine uses the service user's existing GitHub command-line interface (CLI) identity for authentication status, pull request reads, creation, description updates, and merging.
It never asks the CLI for a token.
It never reads or stores the CLI's configuration or credential files.
The authentication query returns only active-account state values through a CLI filter.
It never requests token values.
An empty active-account list permits the signed-out fallback.
A network error or an ambiguous authentication result refuses with a sentence.
Authentication diagnostics and unused command output are discarded.
Selected pull request metadata passes through the production redactor before entering a log, an event, or a client reply.
The project name, goal, and reported evidence are redacted before they become a pull request title or description.
The generated title uses the review screen's existing 72-character limit.
The description is bounded to 8,000 characters and at most 32 kibibytes.
A reused request receives the current redacted description through standard input.
The CLI never receives an agent-selected input filename.

Both Git and the GitHub CLI run in the registered repository folder with separate argument vectors and no shell.
Each process has a bounded timeout and bounded output.
A GitHub command has at most 64 kibibytes of standard output and 16 kibibytes of error output.
Each Git service operation that contacts GitHub has a two-minute limit.
The engine disables interactive Git and GitHub prompts.
Prepare first fetches the remote base into its explicit remote-tracking reference and computes the review against the fetched commit.
That fetch does not update the local base branch or its files.
It force-updates only the selected base tracking reference so it can mirror a rewritten remote base.
This repeatable tracking fetch does not mark a later comparison refusal as an uncertain publication.
Prepare pushes the reviewed commit only to the project's registered branch on an existing repository remote.
It never forces a push or pushes tags or submodules with that operation.
The engine checks the configured and resolved remote URLs before preparing a pull request.
It refuses the repository path `vitalysim/herdr-synapse`, including a resolved push URL with that path.
The existing owner Git push also refuses that path.
The configured fetch and push destinations must resolve to the same GitHub.com repository.
A local mirror, a different host, or a different repository is refused.
Offline tests replace the transport executable on a private search path.
Production has no setting to bypass destination validation.
Delivery commands disable the external transport protocol, push signing, and Git's configured password-prompt program.
They also disable tag following, submodule recursion, automatic HTTP redirects, and custom remote receive and upload programs.
The push also disables mirror mode and force mode.
It uses the validated push URL instead of the remote name because Git applies configured fetch mappings to local references after a named-remote push.
After a successful push, it attempts to update only the selected project tracking reference to the pushed commit.
A failed local tracking update does not block pull request creation.
That update replaces a symbolic reference without following its target.
The fetch disables pruning and configured destination ref mappings.
It writes only the selected base under the selected remote-tracking namespace.
A symbolic base tracking reference is refused before the fetch can follow its target.
A renamed repository requires an updated remote URL.
Delivery does not follow redirects or accept a different canonical repository.
A canonical-name mismatch gives a fixed remote-URL explanation.
The transport can refuse a redirect before canonical metadata is available.
The engine preserves the owner's credential helpers, Secure Shell (SSH) configuration, and URL-based or global proxy configuration because normal authentication and connectivity can depend on them.
A push to the validated URL does not apply remote-name-only proxy settings.
An owner who needs a proxy for that push must use Git's URL-based or global proxy configuration.
An agent that can write the repository's Git configuration can still configure a credential helper or SSH command to run arbitrary code as the service user when the owner prepares or approves.
Such an agent can also change proxies, credential handling, or configuration between validation and execution to redirect traffic or expose the owner's GitHub identity.
A configured checkout filter can run when a fast-forward writes local files.
These command limits do not isolate hostile repository configuration.
The owner must trust that configuration until agents run in the planned external sandbox.

Approval pins the reviewed head in GitHub's merge request and requests a merge commit.
It does not request administrative bypass, automatic merging, or a merge queue.
The preflight refuses a closed pull request or a GitHub merge status other than ready.
It reads the live base branch through GitHub’s reference endpoint at Prepare and immediately before an open pull request merge.
The pull request’s `baseRefOid` metadata is not used as a live branch head.
It checks that the live remote base still matches the fetched base recorded at Prepare.
The local base branch head does not gate hosted approval.
A moved remote base returns a fixed refusal before the merge request and preserves the open review for a fresh Prepare.
GitHub enforces the merge request against its current server state.
The remote base can move between the preflight and the merge request because GitHub offers a head condition but no base condition on that endpoint.
The local checkout is updated only after GitHub confirms the merge.
A dirty local base or a checkout on another branch is left alone.
A failed fetch or fast-forward does not undo a confirmed GitHub merge.
The reply states that the local update could not complete.
A timeout after an external effect can have an uncertain outcome.
The durable operation attempt prevents a retry from blindly repeating that effect.
The owner must refresh and inspect the recorded pull request before continuing.
A new approval request can reconcile an already merged pull request with the exact reviewed head and a valid recorded merge commit.
It records that confirmed merge and attempts the same safe local update without requesting another remote merge.
The original uncertain request keeps its uncertain receipt.

An approval failure without a verified pull request verdict leaves the stored review open.
Prepare can reconcile an already merged request with the exact project head, even after a previous verified refusal.
That path does not push or request another remote merge.
It records the confirmed provider commit and attempts the same conditional local update.
This also supports a squash merge performed elsewhere by the owner.
An explicit nonzero creation refusal is determinate after the successful branch push.
The sentence reports the CLI failure without promising that no remote request exists.
A fresh Prepare checks for an existing request before creating one.
A timeout or lost response after possible creation remains uncertain.
