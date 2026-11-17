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
   Startup permits five minutes of clock skew.
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
