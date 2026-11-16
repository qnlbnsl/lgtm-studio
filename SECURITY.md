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
4. **A session is one random token per engine start, held in memory only.**
   Logging out replaces the token, which signs out every client. Changing the
   password means restarting the engine, which also ends the session. A browser
   holds the token in an `HttpOnly`, `SameSite=Strict` cookie, which is `Secure`
   on a public request when the public address is `https`.
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
   A value inferred only from an inherited environment variable whose name
   matches `KEY`, `TOKEN`, `SECRET` or `PASSWORD` is registered only at eight
   or more characters when its trimmed text is nonempty, is neither `true`
   nor `false`, and is not a plain number.
   On 2026-11-14, the owner authorized ephemeral output for the project terminal and the harness sign-in terminal.
   The owner ruled: "what you echo on terminal is none of our business. just keep it away from log. if they use tmux or something and it hold this in the buffer then that is not on us."
   Only the authenticated owner connection that opens a session may receive its output.
   Both terminals send the program's output byte for byte, with no redaction, hold, or idle timer.
   Neither output nor typed keys are stored, logged, traced, posted, sent as alerts, or included in error messages, crash reports, snapshots, or protocol logs.
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
Only the connection that opened a session may read its output, type into it, resize it, or close it.
The request names a project and a registered worktree by their engine identifiers.
It cannot supply a path, command, executable, argument, environment variable, or shell expression.
Only a worktree on the local machine can open a shell.
The executable is the owner's shell from the service environment, when it is an absolute path listed in `/etc/shells`.
Otherwise, the executable is `/bin/sh`.
The shell inherits the service environment with every `LGTM_` entry removed.
An environment entry containing any registered secret is removed too.
At most eight project terminals may be live at once.
A shell has no time limit.
The service kills its live process groups on close, sign-out, switch-off, connection loss past the grace period, and engine shutdown.
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
They are never stored by the service, logged, posted, sent as events, or included in snapshots or protocol logs.
The owner's ruling of 2026-11-14 covers both this stream and the harness sign-in stream.
The program's output reaches the opening connection byte for byte, with no redaction, hold, or idle timer.
The output never goes to another subscriber or agent.
Neither output nor typed keys are traced or included in an error message.
A secret printed in the terminal appears on the owner's screen and in any scrollback the owner's own tools keep.
The shell runs as the owner and can change files with that user's permissions.
Rule 6 remains unchanged for every other path.
