---
topic: go-s3-stack-client
status: attack
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# The client installs on the machine of the stack and comes back by itself

## Overview

Sprint 3 makes a machine join the server. This plan gives the owner one command that puts the client on the same computer as the stack. The command builds the client, joins it to the server, and makes it a service of the owner's user. After the command, the machine waits for approval on the Machines page. After a restart of the computer, the client starts again without anyone logging in.

The join needs a token. The design lets the owner make a token on the Machines page or with the server's own command. This plan writes that command, `lgtm-server join-token`, because the installer cannot click a page. A token from this command can also carry one extra mark: the machine that uses it runs the server. That mark is how the Machines page learns which machine is the server's own, which no message of the link reports.

The user-flow script `deploy/test/checks.sh` already exists, and it must pass all 13 of its checks at the end of the sprint. Its machine checks run their own clients through the web server of the stack. The one thing the Stack team must add for them is a web server that carries the link: a long two-way stream over the Hypertext Transfer Protocol version 2 (HTTP/2) without encryption, on the loopback address. A trial on 2026-11-26 passed such a stream through the same web server, Caddy, for 80 seconds with a silent gap of 40 seconds. This plan turns that trial into a kept test.

Two points need the orchestrator. First, the token command lives in the Server team's folder, as the brief allows. The Server team's lead merges it. Second, check C5 cannot reach the database inside the stack as the script is written today, because the database accounts and their passwords changed in the sprint 1 plan. The result file proposes the change to the script.

On macOS, the client starts when the owner logs in, not at boot. Starting at boot would need an administrator to install a system service. The design names no tool for macOS, so the plan chose the one that stays within the owner's user.

## Key decisions

- **The installer builds the client from the checkout.** No built client is published anywhere yet, and nothing would sign one. The checkout is already on the machine, because the stack starts from it.
- **On Linux the client is a systemd user service, and the user lingers.** Lingering is the setting that lets a user's services start at boot without a login. When the installer cannot turn it on, it says so and exits with a failure, but the client is still installed.
- **The installer joins before it installs the service.** A failed join therefore leaves nothing running.
- **The token marks the server's own machine.** The machine never claims the mark itself, so a remote machine cannot pose as the server's machine.
- **The client's exit code tells the service manager what to do.** Exit 0 means stop for good, for example after the owner removed the machine. Exit 75 means that the server asked for a restart. Every other exit is a failure that is retried after 5 seconds. The Client team must agree to these codes.
- **A second run is safe.** It installs the newly built program and restarts the service, and it keeps the machine's identity. A separate option joins again from a fresh folder and keeps the old folder under another name.

Full log → `decisions.md`

## Workstreams

### The server makes a join token from its own command

The owner can make a token without the website, with one command inside the server's container. The command prints the token and nothing else, so a script can capture it. It refuses an address that a client would refuse, so no token names an address that cannot work.

The risk is that the token's secret leaks into a log. The tests read everything the command writes besides the token itself and find no part of the secret there.

When it is done, a person runs the command against the stack and gets one line that starts with `lgtm1_`. A machine that joins with it shows on the Machines page as the machine that runs the server.

### The client installs as a service that comes back by itself

`deploy/install-client.sh` takes the computer from a running stack to a joined client that runs as a service. It checks that the stack answers, builds the client, asks the server for a token, joins, and then installs and starts the service. A second run updates the program and keeps the identity. Other options join again or remove the service.

The main risk is a service that looks installed but does not start after a restart. A live test on a Linux machine kills the client and watches it come back. A person restarts the computer, and the test then proves from the clocks of the system that the client started before the first login. A second risk is the token in the installer's output, which the tests rule out the same way as for the command.

When it is done, the installer ends with the machine waiting for approval. After approval the machine shows as online. After a restart of the computer it shows as online again.

### The web server carries the link between a machine and the server

A machine on the same computer reaches the server through the web server of the stack, on the loopback address, without encryption. The link is one stream that stays open for as long as the client runs. The web server must pass it on in both directions at once, and it must end it when the server stops, so that the client notices and connects again.

The risk is a web server that buffers the stream or cuts it after a silence. A kept test runs a small echo program in place of the server and sends messages through the web server for more than two minutes, with a long silent gap. It also stops the echo program and checks that the stream ends within seconds.

When it is done, the link check passes against the website image of sprint 2. The machine checks C5 to C11 of the user-flow script then depend only on the server and the client.

## Rollout order

1. Build the link check against the merged web server of sprint 2. It can start as soon as sprint 2 is merged, and it finds a problem in the web server before any link code exists.
2. Build the installer on Linux, with its tests that need no stack. It can be written at the same time as step 1, because its tests replace every outside program with a stand-in.
3. Build the server's token command when the Server team's token and join code are merged. The Server team's lead merges it.
4. Build the macOS part of the installer after the Linux part is merged, because both change the same script.
5. Run the live test of the installer on a Linux machine once the client and the server of sprint 3 are merged. A person restarts the computer between its two parts.
6. Run the whole user-flow script. All 13 checks must pass before the sprint is handed over.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**How the statements of the design are cited**

The design is `docs/specs/2026-11-25-go-rewrite-foundation-design.md` on `master`.
D1 to D23 are the rows of its decisions table, in order. D7 is the key pair and the join token. D16 is the restart of a connected client and the retry every 15 seconds. D17 is trust without a public authority. D19 is one user on a machine.
R1 to R11 are its rules. R3 is "a part calls another part's Go interface". R9 is "secrets never reach a log". R10 is "tests run against a real PostgreSQL".
C1 to C13 are its checks of wave 1.
The design's sections are cited by their headings, such as "Joining: a token that authorizes an exchange of keys".
`scouting.md` lists each statement with its line.

**Public names that other work items and teams build against**

| Name | Kind | Meaning | Set by |
|---|---|---|---|
| `lgtm-server join-token --address <address> [--hosts-server]` | command | Makes one join token. Prints its text, one line that starts with `lgtm1_`, on standard output. Exit 0 on success, 1 when the server's settings, database or vault fail, and 2 for a wrong argument. Every message is one sentence on standard error. | item 1 |
| `joinTokenCommand(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int` | Go function in package `main` of `src/server/cmd/lgtm-server` | The body of the command. `args` excludes the program's name and the word `join-token`. | item 1 |
| `deploy/install-client.sh [--rejoin \| --remove]` | command | Installs, joins again, or removes the client on this computer. Exit 0 on success, 1 on a failure, 2 for a wrong argument. | items 3 and 4 |
| `LGTM_WEB_PORT` | variable read by the installer | The loopback port of the stack. The default is 8787. It is the same variable that `deploy/test/checks.sh` reads. | item 3 |
| `COMPOSE_PROJECT_NAME`, `LGTM_STACK_STATE_DIR` | variables passed through | The installer passes them on to `docker compose`, as `deploy/up.sh` of plan `go-s1-stack-vault` does. | item 3 |
| `$HOME/.local/bin/lgtm-client` | file | The installed program, mode `0755`. | item 3 |
| `lgtm-client.service` at `${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/` | file | The user unit on Linux. | item 3 |
| `deploy/lgtm-client.service` | template | The unit with the placeholders `@LGTM_CLIENT_BIN@`, `@LGTM_CLIENT_STATE_DIR@` and `@LGTM_CLIENT_PATH@`. | item 3 |
| `dev.lgtmstudio.client` | launchd label | The agent on macOS, at `$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist`. | item 4 |
| `deploy/dev.lgtmstudio.client.plist` | template | The agent with the placeholders of the unit and `@LGTM_CLIENT_LOG@`. | item 4 |
| `$HOME/Library/Logs/lgtm-client.log` | file | The output of the client on macOS. On Linux the output goes to the journal of the user. | item 4 |
| `LGTM_CLIENT_STATE_DIR` | variable in the service only | The state folder, fixed at install time. See decision 15. | item 3 |
| `lgtm-client.replaced-<YYYYMMDDTHHMMSSZ>` | folder beside the state folder | The old state folder after `--rejoin`. | item 3 |
| Exit codes of `lgtm-client run` | contract with plan `go-s3-client-link` | 0 stops for good and is not restarted. 75 is a restart that the server asked for. Any other code is a failure. Every code except 0 is restarted after 5 seconds. See decision 17. | items 3 and 4 |
| `deploy/test/linkecho/main.go` | test program | `linkecho serve <listen address>` and `linkecho dial <url> --messages <n> --interval <duration> --silence <duration>`. | item 2 |

**WS-1: The server makes a join token from its own command**

The design lets the owner make a token "with the server's own command", and the installer needs that path because it cannot use the website.
The command is a short program run inside the server's container.
It opens the server's settings, database and vault the same way the server does.
It asks the machines part to make a token, by rule R3, and prints the text.
It adds one fact that nothing else on the link carries: the mark that the joining machine runs the server.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1a | The command prints one token line, and a client can join with that token once. | `TestJoinTokenPrintsOneTokenLine`, `TestJoinTokenCarriesAddressAndServerKey` and `TestJoinTokenJoinsOnce` pass with `LGTM_TEST_POSTGRES` set. |
| 1b | A token made with `--hosts-server` gives a machine whose `hosts_server` is true, and a token made without it gives false. | `TestJoinTokenHostsServerFlag` passes. |
| 1c | A wrong address or argument is refused, and the secret never reaches standard error. | `TestJoinTokenRequiresAddress`, `TestJoinTokenRefusesAddressClientsRefuse`, `TestJoinTokenRefusesUnknownArgument`, `TestJoinTokenFailsWithoutDatabase` and `TestJoinTokenKeepsSecretOffStandardError` pass. |
| 1d | The command prints a token inside the running stack. | `docker compose -f deploy/compose.yaml exec -T server lgtm-server join-token --address http://127.0.0.1:8787 --hosts-server` prints exactly one line that matches `^lgtm1_[A-Za-z0-9_-]+$` and exits 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1.1 | Write the command and its argument rules. | `src/server/cmd/lgtm-server/join_token.go`, `joinTokenCommand` (target). Pattern: `src/server/cmd/lgtm-server/main.go` of plan `go-s2-server-signin`, item 1, and `src/client/cmd/lgtm-client/main.go`, `run`, of plan `go-s1-client-local`, item 1. |
| 1.2 | Write the tests against a real server and database. | `src/server/cmd/lgtm-server/join_token_test.go` (target). Pattern: `src/contracts/join_test.go`, `TestJoinTokenExampleDecodes`, of plan `go-s0-contract`, item 5. |

**Work item 1. The server's own command that makes a join token**

- **Files:** creates `src/server/cmd/lgtm-server/join_token.go` and `src/server/cmd/lgtm-server/join_token_test.go`. The test writer creates the test file.
- **Needs first:** plan `go-s2-server-signin`, items 1 and 8, for the settings, the start-up and the signing key. Plan `go-s3-server-machines`, items 2 and 4, for tokens and `Join`. The entry point for tests in `src/server/servertest`, from issue 1634. It runs at the same time as items 2 and 3.
- **Work area and complexity:** `backend`, medium. It is one small command over interfaces that other items build, but it handles a join token.
- **Scope:** `server/backend`. The file sits in the Server team's folder, and the Server team's lead merges it. See decision 8.
- **Design statements:** D7, D17, R3, R8, R9, R10, C5, C6. The section "Joining: a token that authorizes an exchange of keys", step 1, and the section "The compose stack", the bullet on `deploy/install-client.sh`.
- **Risky:** yes, because it makes a join token. Two mutations must turn the tests red. Passing `false` for the mark in every case must fail `TestJoinTokenHostsServerFlag`. Writing the token's text to standard error in a log line must fail `TestJoinTokenKeepsSecretOffStandardError`.
- **Behaviour:**
  - `lgtm-server join-token` reaches `joinTokenCommand` through the program's table of commands, which plan `go-s2-server-signin` provides (request 5 in the result file).
  - `--address <address>` is required. The address must be `https://<host>[:port]`, or `http://` with the host `127.0.0.1`, `[::1]` or `localhost`. It has no path, query or fragment. This is the same rule that a client applies to the address in a token, by the comment of `JoinToken` in plan `go-s0-contract`.
  - `--hosts-server` takes no value.
  - Any other argument, a repeated argument, or a missing address gives exit 2, nothing on standard output, and one sentence on standard error that names the argument.
  - The command opens the settings, the store and the vault through the same functions that the server's start uses (request 6 in the result file). It applies no other change and opens no port.
  - It asks the machines part for one token, with the address and the mark (request 1 in the result file). The machines part fills in both fingerprints of the token. When it cannot pin an `https` address, the command passes on its refusal as exit 1 and one sentence.
  - On success it writes the token's text and one newline to standard output, and nothing else. It writes nothing to standard error.
  - A failure of the settings, the database or the vault gives exit 1 and one sentence on standard error.
  - No path of the command writes the token's text or its secret anywhere except standard output, by rule R9.
- **Tests written first:** in `src/server/cmd/lgtm-server/join_token_test.go`, package `main`. Each test starts a server in its own process through `src/server/servertest`, against the database named by `LGTM_TEST_POSTGRES`, and calls `joinTokenCommand` with a `getenv` that returns the same settings as that server. A stub of `joinTokenCommand` that returns 0 and writes nothing makes every positive test fail on its first assertion.

| Test | What it asserts |
|---|---|
| `TestJoinTokenPrintsOneTokenLine` | With `--address http://127.0.0.1:8787`, the exit code is 0. Standard output matches `^lgtm1_[A-Za-z0-9_-]+\n$` exactly. Standard error is empty. |
| `TestJoinTokenCarriesAddressAndServerKey` | The text after `lgtm1_` decodes as unpadded base64url and unmarshals into `linkv1.JoinToken`. Its `address` equals the argument. Its `token_id` parses as a version 7 universally unique identifier (UUID). Its `secret` has 32 bytes. Its `server_key_sha256` equals the SHA-256 hash (Secure Hash Algorithm, 256 bits) of the `server_public_key` that a `Join` call with this token returns. Its `certificate_key_sha256` is empty for this loopback address. |
| `TestJoinTokenJoinsOnce` | `Join` on the started server with the token's `token_id`, its secret and a fresh Ed25519 public key succeeds and returns a machine identifier. A second `Join` with the same token and another public key returns the Connect code `permission_denied`. |
| `TestJoinTokenHostsServerFlag` | One token is made with `--hosts-server` and one without. A machine joins with each. The first message of `ClientService.WatchClients`, under a session of the started server, lists both machines as waiting for approval. The machine whose `token_id` is the first token's has `hosts_server` true. The other has `hosts_server` false. |
| `TestJoinTokenRequiresAddress` | Without `--address`, the exit code is 2, standard output is empty, and standard error holds `--address`. |
| `TestJoinTokenRefusesAddressClientsRefuse` | Each of `http://example.test:8787`, `ftp://127.0.0.1:8787`, `http://127.0.0.1:8787/path` and `127.0.0.1:8787` gives exit 2, an empty standard output, and one sentence on standard error that holds `--address`. |
| `TestJoinTokenRefusesUnknownArgument` | `--name x`, `--hosts-server=yes` and `--address` given twice each give exit 2 and an empty standard output. |
| `TestJoinTokenFailsWithoutDatabase` | With the database setting pointed at a closed loopback port, the exit code is 1, standard output is empty, and standard error is one line that ends with a full stop. |
| `TestJoinTokenKeepsSecretOffStandardError` | With the server's log level at its most detailed setting (request 6), a successful run leaves standard error without the token's text, without the secret in hexadecimal, and without the secret in standard base64 or in base64url. |

- **Acceptance:** `LGTM_TEST_POSTGRES=<url> go test -count=1 -run 'TestJoinToken' ./src/server/cmd/lgtm-server/` passes all nine tests. Criterion 1d holds against the stack.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The token's secret reaches a log of the server or of the command. | High | The command logs nothing on success. `TestJoinTokenKeepsSecretOffStandardError` searches standard error in three encodings at the most detailed log level. |
| A token marks a remote machine as the server's machine. | Medium | Only the command sets the mark, and it runs inside the server's container. `ClientService.CreateClientToken` never sets it (request 1). |
| The command and the running server disagree on the signing key. | Medium | The command loads the key through the server's own function, and `TestJoinTokenCarriesAddressAndServerKey` compares the token with the key that `Join` returns. |

**Verify:** the nine tests above against a real PostgreSQL, and criterion 1d against the stack. No device is needed.

**Rollout detail:** step 3 of the rollout order. The Server team's lead merges the item after a passing report on its exact commit.

**WS-2: The client installs as a service that comes back by itself**

The design says that the client "runs as a service of its user, outside the compose stack", and that `deploy/install-client.sh` "installs the client on the same machine and joins it".
This workstream writes that script, the service files it installs, and the tests that prove both on Linux and macOS.
The hermetic tests replace every outside program with a stand-in on `PATH`, so they need no Docker, no network and no service manager.
The live test runs the real installer against the real stack and the real service manager of a Linux user.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2a | On Linux, the installer builds the program, joins with a token of the server's command, and installs and starts the user unit, in that order. | `installs_linux_user_service`, `asks_server_for_hosts_server_token` and `joins_before_service_starts` in `bash deploy/test/installer.sh`. |
| 2b | The token never appears in the installer's output or in the files it writes. | `token_never_printed`. |
| 2c | A failed join installs no service, and each refusal of a precondition stops the installer before it changes anything. | `join_failure_installs_no_service`, `refuses_root`, `refuses_stack_not_answering`, `refuses_malformed_token`, `refuses_missing_user_manager` and `refuses_unsafe_home`. |
| 2d | A second run keeps the identity, `--rejoin` keeps the old folder under a new name, and `--remove` removes the service and keeps the state. | `rerun_keeps_joined_identity`, `rejoin_moves_old_state` and `remove_stops_and_deletes_service`. |
| 2e | On macOS, the installer loads a launchd agent that parses as a property list. | `installs_macos_launch_agent`, `macos_agent_parses` and `macos_remove_boots_out_agent` in `bash deploy/test/installer.sh`. On a Mac, a person runs the installer, then `launchctl print gui/$(id -u)/dev.lgtmstudio.client`, and reads `state = running`. |
| 2f | On a real Linux machine, the installer leaves the machine waiting, the service restarts after a crash and after the server's restart command, and a second run keeps the machine. | `live_install_leaves_machine_waiting`, `live_service_enabled_with_linger`, `live_service_restarts_after_crash`, `live_approved_machine_online`, `live_reconnect_restarts_service` and `live_rerun_keeps_machine` in `LGTM_INSTALL_LIVE=1 deploy/test/installer-live.sh`. |
| 2g | After a restart of the computer, the client started before the first login and shows as online. | A person runs `sudo systemctl reboot`, logs in, and runs `deploy/test/installer-live.sh --after-reboot`. It reports `ok` for `live_service_started_before_login` and `live_machine_online_after_reboot`. |
| 2h | The whole user-flow script passes. | `deploy/test/checks.sh` prints 13 lines that start with `ok` and exits 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 3.1 | Write the installer for Linux. | `deploy/install-client.sh`, `main` (target). Pattern: `deploy/test/checks.sh`, `compose` at line 22 and `configure_web` at line 32. |
| 3.2 | Write the user unit template. | `deploy/lgtm-client.service`, section `[Service]` (target). Pattern: the systemd unit format in `scouting.md`. |
| 3.3 | Write the hermetic tests and their stand-ins. | `deploy/test/installer.sh`, `run_test` (target). Pattern: `deploy/test/checks.sh`, `run_check` at line 658, for one Test Anything Protocol (TAP) line per test. |
| 4.1 | Add the macOS branch and the agent template. | `deploy/install-client.sh`, `install_launch_agent` (target), and `deploy/dev.lgtmstudio.client.plist` (target). Pattern: the launchd property list format in `scouting.md`. |
| 5.1 | Write the live test of the installer. | `deploy/test/installer-live.sh`, `after_reboot` (target). Pattern: `deploy/test/checks.sh`, `open_watch` at line 238 and `read_frame` at line 251. |
| 5.2 | Add the guide for installing the client. | `deploy/README.md`, section "Installing the client" (target). The file comes from plan `go-s1-stack-vault`, work item 6. |

**Work item 3. The installer on Linux**

- **Files:** creates `deploy/install-client.sh` (executable), `deploy/lgtm-client.service` and `deploy/test/installer.sh` (executable). The test writer creates the test file and its stand-ins, which the test writes into a temporary folder at each run.
- **Needs first:** nothing to write and to pass its own tests. The names it uses come from items 1 and 2 of plan `go-s3-client-link` and from item 1 of this plan. It runs at the same time as items 1 and 2.
- **Work area and complexity:** `infra`, medium. It is one script of known steps with a few failure paths, and it passes a join token.
- **Scope:** `stack/infra`.
- **Design statements:** D7, D16, D19, R9, C6. The section "The client in wave 1", on the state folder and the service of the user. The section "The compose stack", the bullet on `deploy/install-client.sh`.
- **Risky:** yes, because it passes a join token. Two mutations must turn the tests red. Printing the token with `printf '%s\n' "$token"` must fail `token_never_printed`. Moving the service step in front of the join must fail `joins_before_service_starts`.
- **Behaviour, in order:**
  1. The script changes to the repository root, as `deploy/test/checks.sh` does. It accepts no argument, `--rejoin` or `--remove`. Anything else prints the usage and exits 2.
  2. It refuses to run when `id -u` is 0, with `install-client.sh must run as the user who runs the harnesses, not as root.` and exit 1.
  3. It reads `uname -s`. `Linux` takes the steps below. `Darwin` takes the steps of item 4. Anything else gives `install-client.sh supports Linux and macOS only.` and exit 1.
  4. On Linux it runs `systemctl --user show-environment`. A failure gives `This Linux system has no systemd user manager for this user.` and exit 1.
  5. It refuses a `HOME` that holds a character outside letters, digits and `. _ / + @ -`, with `The home folder path holds a character that a service file cannot carry.` and exit 1.
  6. It works out three values. The address is `http://127.0.0.1:${LGTM_WEB_PORT:-8787}`, with the same check of the port as `configure_web` in `deploy/test/checks.sh`. The state folder follows decision 15. The service's search path follows decision 16.
  7. `--remove` stops here. It runs `systemctl --user disable --now lgtm-client.service`, ignoring a unit that is not loaded. It deletes the unit file and the program, runs `systemctl --user daemon-reload`, prints one sentence, and exits 0. It keeps the state folder.
  8. It calls `SessionService/GetServer` at the address with `curl`, as `get_server` in `deploy/test/checks.sh` does. A failure, or a body without `"product"` and `"lgtm-studio"` as a pair, gives `The stack does not answer at <address>. Start it with deploy/up.sh.` and exit 1.
  9. It builds the client with `CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=<version>" -o <temporary file> ./src/client/cmd/lgtm-client`. The temporary file sits in `$HOME/.local/bin/`. The version is `0.0.0-` and the first 12 hexadecimal digits of `git rev-parse HEAD`, with `-dirty` when `git status --porcelain` prints anything. Without Git it is `0.0.0-dev`. The script runs `<temporary file> version`, renames the file to `$HOME/.local/bin/lgtm-client` with mode `0755`, and prints `built lgtm-client <version>`.
  10. With `--rejoin`, it runs `systemctl --user stop lgtm-client.service`, ignoring a unit that is not loaded. It then renames the state folder to `lgtm-client.replaced-<time>` beside it, when the folder exists.
  11. When `<state folder>/server.json` exists and `--rejoin` was not given, it prints `lgtm-client is already joined.` and skips step 12.
  12. It runs `docker compose -f deploy/compose.yaml exec -T server lgtm-server join-token --address <address> --hosts-server`, and keeps standard output in a variable. Output that does not match `^lgtm1_[A-Za-z0-9_-]+$` gives `The server's join-token command returned no token.` and exit 1. It then runs `LGTM_CLIENT_STATE_DIR=<state folder> $HOME/.local/bin/lgtm-client join <address> <token>`. A failure passes on the client's output, prints `lgtm-client could not join the server.`, and exits 1, with no service installed. The variable that held the token is cleared after the join.
  13. It renders `deploy/lgtm-client.service` with the three placeholders into a temporary file beside the unit's path, and renames it to the unit's path. It runs `systemctl --user daemon-reload`, `systemctl --user enable lgtm-client.service` and `systemctl --user restart lgtm-client.service`.
  14. When `loginctl show-user <user> --property=Linger --value` does not print `yes`, it runs `loginctl enable-linger <user>`. A failure prints `loginctl enable-linger <user> failed, so the client starts only after this user logs in.` and sets the exit status to 1 for the end of the run.
  15. It waits up to 10 seconds for `systemctl --user is-active lgtm-client.service` to print `active`. A timeout gives `lgtm-client.service did not start.` and exit 1.
  16. It prints `lgtm-client runs as the user service lgtm-client.service.` and exits with 0, or with the status of step 14.
- **The unit template** `deploy/lgtm-client.service` holds these lines and no other setting:
  - `[Unit]`, `Description=LGTM Studio client`, `StartLimitIntervalSec=0`.
  - `[Service]`, `Type=simple`, `ExecStart=@LGTM_CLIENT_BIN@ run`, `Environment=LGTM_CLIENT_STATE_DIR=@LGTM_CLIENT_STATE_DIR@`, `Environment=PATH=@LGTM_CLIENT_PATH@`, `Restart=on-failure`, `RestartSec=5`, `TimeoutStopSec=10`.
  - `[Install]`, `WantedBy=default.target`.
  - `Restart=on-failure` restarts every exit except 0, which matches decision 17. `StartLimitIntervalSec=0` keeps systemd from giving up after five quick failures.
- **Tests written first:** `deploy/test/installer.sh` prints one TAP line per test, `ok <n> - <test id>` or `not ok <n> - <test id> <reason>`, and exits 0 only when all pass.
  - **The stand-ins.** Each test makes a temporary folder with `home/` as `HOME`, `shims/` first on `PATH`, and `calls.log`. The shims are `go`, `docker`, `systemctl`, `loginctl`, `launchctl`, `uname`, `id` and `curl`. Each appends one line with its name and arguments to `calls.log`, so the order of calls is one file. Variables of the test steer them: `SHIM_UNAME`, `SHIM_UID`, `SHIM_CURL_FAIL`, `SHIM_TOKEN`, `SHIM_JOIN_EXIT`, `SHIM_LINGER_FAIL` and `SHIM_NO_USER_MANAGER`. The `id` shim answers `-u` with `SHIM_UID` and passes other arguments to the real `id`. The `go` shim, called with `build ... -o <file>`, writes a stand-in client to `<file>`. That client answers `version` with `0.0.0-test`. It answers `join <address> <token>` by appending `client join <address> <SHA-256 of the token> <LGTM_CLIENT_STATE_DIR>` to `calls.log` and creating `server.json` and `machine.key` in that folder, or by printing `{"msg":"join_refused","code":"token_used"}` and exiting with `SHIM_JOIN_EXIT` when that is not 0. The `docker` shim prints `SHIM_TOKEN`, whose default is a fixed `lgtm1_` text of 60 characters. The `curl` shim prints `{"product": "lgtm-studio", "contract": 1}` unless `SHIM_CURL_FAIL` is set.
  - A stub `deploy/install-client.sh` that exits 0 and does nothing makes every positive test fail on its first assertion.

| Test | What it asserts |
|---|---|
| `installs_linux_user_service` | With `SHIM_UNAME=Linux`, the exit code is 0. `$HOME/.local/bin/lgtm-client` exists with mode `0755`. `$HOME/.config/systemd/user/lgtm-client.service` equals `deploy/lgtm-client.service` with `@LGTM_CLIENT_BIN@` replaced by the program's path, `@LGTM_CLIENT_STATE_DIR@` by `$HOME/.local/state/lgtm-client`, and `@LGTM_CLIENT_PATH@` by the test's search path. `calls.log` holds, in this order, `systemctl --user daemon-reload`, `systemctl --user enable lgtm-client.service` and `systemctl --user restart lgtm-client.service`. It holds `loginctl enable-linger` with the user's name. |
| `asks_server_for_hosts_server_token` | `calls.log` holds exactly one `docker` line, `docker compose -f deploy/compose.yaml exec -T server lgtm-server join-token --address http://127.0.0.1:8787 --hosts-server`. A second run in a fresh folder with `LGTM_WEB_PORT=8899` has the address `http://127.0.0.1:8899`. |
| `joins_before_service_starts` | The `client join` line comes before the first `systemctl --user enable` line in `calls.log`. Its address is `http://127.0.0.1:8787`. Its hash equals the SHA-256 of `SHIM_TOKEN`. Its folder equals the value of `LGTM_CLIENT_STATE_DIR` in the unit file. |
| `token_never_printed` | The installer's standard output and standard error do not hold `SHIM_TOKEN`. No file under the test's `HOME` holds it. |
| `join_failure_installs_no_service` | With `SHIM_JOIN_EXIT=1`, the exit code is 1. The output holds `join_refused`. No unit file exists. `calls.log` holds no `systemctl --user enable` line. |
| `refuses_root` | With `SHIM_UID=0`, the exit code is 1, the output holds `not as root`, and `calls.log` holds no `go` line. |
| `refuses_stack_not_answering` | With `SHIM_CURL_FAIL=1`, the exit code is 1, the output holds `deploy/up.sh`, and `calls.log` holds no `docker` line and no `go` line. |
| `refuses_malformed_token` | With `SHIM_TOKEN=oops`, the exit code is 1, `calls.log` holds no `client join` line, and no unit file exists. |
| `refuses_missing_user_manager` | With `SHIM_NO_USER_MANAGER=1`, the exit code is 1, and `calls.log` holds no `go` line. |
| `refuses_unsafe_home` | With `HOME` set to a folder whose name holds a space, the exit code is 1, and `calls.log` holds no `go` line. |
| `unsafe_path_entries_dropped` | With a search path that holds an entry with `%` and an entry with a space, the unit's `PATH` line holds neither entry and keeps the others in their order. |
| `linger_failure_is_reported` | With `SHIM_LINGER_FAIL=1`, the unit file exists, the `restart` line is in `calls.log`, the output names `loginctl enable-linger`, and the exit code is 1. |
| `rerun_keeps_joined_identity` | After a first run, a second run exits 0. `calls.log` of the second run holds no `docker` line and no `client join` line. It holds `daemon-reload` and `restart`. `server.json` and `machine.key` are unchanged byte for byte. |
| `rejoin_moves_old_state` | After a first run, `--rejoin` exits 0. A folder whose name matches `^lgtm-client\.replaced-[0-9]{8}T[0-9]{6}Z$` sits beside the state folder and holds the first `machine.key`. The `systemctl --user stop lgtm-client.service` line comes before the new `client join` line. A new `machine.key` exists in the state folder. |
| `remove_stops_and_deletes_service` | After a first run, `--remove` exits 0. `calls.log` holds `systemctl --user disable --now lgtm-client.service`. The unit file and the program are gone. The state folder and its files are unchanged. |
| `usage_on_unknown_argument` | `--frobnicate` gives exit 2 and an empty `calls.log`. |

- **Acceptance:** `bash deploy/test/installer.sh` prints `ok` for the 16 tests above and exits 0, with no Docker, network or service manager on the test machine.

**Work item 4. The installer on macOS**

- **Files:** changes `deploy/install-client.sh`. Creates `deploy/dev.lgtmstudio.client.plist`. Changes `deploy/test/installer.sh`, where the test writer adds the tests below.
- **Needs first:** item 3, because both change the same two files.
- **Work area and complexity:** `infra`, medium. It is a second branch of a known script, and no machine of the checks runs macOS.
- **Scope:** `stack/infra`.
- **Design statements:** D16, D19, R9. The section "The client in wave 1".
- **Risky:** no. It changes no step that touches the token, which item 3's tests keep covering on both systems.
- **Behaviour:**
  - With `uname -s` equal to `Darwin`, steps 4 and 14 of item 3 are skipped, and the service steps 7, 10, 13 and 15 use launchd.
  - The agent file is `$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist`, rendered from the template and renamed into place.
  - The template is a property list with these keys: `Label` `dev.lgtmstudio.client`; `ProgramArguments` with the program and `run`; `EnvironmentVariables` with `LGTM_CLIENT_STATE_DIR` and `PATH`; `RunAtLoad` true; `KeepAlive` as a dictionary with `SuccessfulExit` false; `ThrottleInterval` 5; `StandardOutPath` and `StandardErrorPath` both `@LGTM_CLIENT_LOG@`; `ProcessType` `Background`.
  - `KeepAlive` with `SuccessfulExit` false starts the client again after every exit except 0, which matches decision 17.
  - Installing runs `launchctl bootout gui/<uid>/dev.lgtmstudio.client`, ignoring an agent that is not loaded, then `launchctl bootstrap gui/<uid> <agent file>`, then `launchctl enable gui/<uid>/dev.lgtmstudio.client`.
  - The wait of step 15 reads `launchctl print gui/<uid>/dev.lgtmstudio.client` for `state = running`.
  - `--rejoin` stops the agent with `launchctl bootout` before it moves the folder. `--remove` runs `launchctl bootout`, then deletes the agent file and the program.
  - The state folder on macOS follows decision 15. The design names `~/.local/state/lgtm-client/` for every system.
- **Tests written first:** added to `deploy/test/installer.sh`, with `SHIM_UNAME=Darwin`.

| Test | What it asserts |
|---|---|
| `installs_macos_launch_agent` | The exit code is 0. The agent file equals the template with its four placeholders replaced, and `@LGTM_CLIENT_LOG@` becomes `$HOME/Library/Logs/lgtm-client.log`. `calls.log` holds `launchctl bootout`, `launchctl bootstrap gui/<uid> <agent file>` and `launchctl enable gui/<uid>/dev.lgtmstudio.client`, in that order, after the `client join` line. It holds no `systemctl` line and no `loginctl` line. |
| `macos_agent_parses` | The rendered agent file parses with `plutil -lint` when that program exists, and otherwise with the `plistlib` module of Python 3. The parsed `KeepAlive` is a dictionary whose `SuccessfulExit` is false, and `ProgramArguments` ends with `run`. When neither parser exists, the line is `ok <n> - macos_agent_parses # SKIP no parser`. |
| `macos_remove_boots_out_agent` | After a first run, `--remove` exits 0, `calls.log` holds `launchctl bootout gui/<uid>/dev.lgtmstudio.client`, and the agent file and the program are gone. |
| `refuses_unsupported_system` | With `SHIM_UNAME=FreeBSD`, the exit code is 1, the output holds `Linux and macOS only`, and `calls.log` holds no `go` line. |

- **Acceptance:** `bash deploy/test/installer.sh` prints `ok` for the four tests above and for the 16 tests of item 3. On a Mac with the stack running, a person runs `deploy/install-client.sh`, then `launchctl print gui/$(id -u)/dev.lgtmstudio.client`, and reads `state = running`. The person logs out, logs in, and reads `state = running` again.

**Work item 5. The live test of the installer and its guide**

- **Files:** creates `deploy/test/installer-live.sh` (executable). Changes `deploy/README.md` with a section named "Installing the client". The test writer creates the test file. The builder writes the section.
- **Needs first:** items 1, 2 and 3. Plan `go-s3-client-link`, item 6. Plan `go-s3-server-machines`, items 3 to 9.
- **Work area and complexity:** `infra`, medium. It drives the real stack and the real service manager, and one of its parts needs a restart of the computer.
- **Scope:** `stack/infra`.
- **Design statements:** D16, D19, C5, C6, C7, C9. The epic's sentence that the stack, the server and the client each come back by themselves after a restart.
- **Risky:** no. It adds no code that handles a token. The token passes through the code of items 1 and 3.
- **How the test runs:** it needs Linux, a stack started by `deploy/up.sh`, `LGTM_WEB_PASSWORD`, a systemd user manager, and Docker. Its default part also needs `LGTM_INSTALL_LIVE=1`, and it refuses to start when `lgtm-client.service` already exists, so it never replaces an installation of the owner. It signs in and reads `ClientService.WatchClients` the way `deploy/test/checks.sh` does in `signin`, `open_watch` and `read_frame`. The default part leaves the client installed for the reboot. `--after-reboot` checks the state after a restart. `--cleanup` removes the machine with `ClientService/RemoveClient` and runs `deploy/install-client.sh --remove`.
- **Tests written first:** each prints one TAP line. Before items 1 and 3 exist, the first test fails on the installer's exit code.

| Test | What it asserts |
|---|---|
| `live_install_leaves_machine_waiting` | `deploy/install-client.sh` exits 0. The first `WatchClients` message after it lists one more client than the one before it. That client is waiting for approval and has `hostsServer` true. |
| `live_service_enabled_with_linger` | `systemctl --user is-enabled lgtm-client.service` prints `enabled`. `is-active` prints `active`. `loginctl show-user <user> --property=Linger --value` prints `yes`. |
| `live_service_restarts_after_crash` | After `systemctl --user kill --signal=KILL lgtm-client.service`, the unit is active again within 15 seconds with another `MainPID`. |
| `live_approved_machine_online` | After `ClientService/ApproveClient` for that client, a `WatchClients` stream shows it online with `hostsServer` true within 35 seconds. |
| `live_reconnect_restarts_service` | After `ClientService/ReconnectClient` for that client, the property `NRestarts` of the unit grows by one within 35 seconds, and a new `WatchClients` stream shows the client online with a later `connectedAt`. |
| `live_rerun_keeps_machine` | A second `deploy/install-client.sh` exits 0. The count of clients is unchanged. The same client is online again within 35 seconds. The SHA-256 of `server.json` in the state folder is unchanged. |
| `live_service_started_before_login` | With `--after-reboot`: the property `ActiveEnterTimestampMonotonic` of the unit is smaller than the property `TimestampMonotonic` of every session of the user whose property `Class` is `user`. The test reads the sessions with `loginctl list-sessions` and `loginctl show-session`. It leaves out the sessions of class `manager`, because systemd opens one for the user's own service manager at boot when the user lingers. Both properties count microseconds from the same boot. |
| `live_machine_online_after_reboot` | With `--after-reboot`: within 60 seconds of the start of the test, a `WatchClients` stream shows the installed client online. |

- **The guide:** the section "Installing the client" of `deploy/README.md` names the command, its two options, the paths of the program, the unit or agent and the state folder, how to read the client's output on each system, and that a Mac starts the client at login. It follows the repository's writing style.
- **Acceptance:** `LGTM_INSTALL_LIVE=1 deploy/test/installer-live.sh` prints `ok` for its first six tests. After the person's restart of criterion 2g, `deploy/test/installer-live.sh --after-reboot` prints `ok` for its last two. `deploy/test/installer-live.sh --cleanup` exits 0. `deploy/test/checks.sh` prints 13 `ok` lines and exits 0.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The service runs after a login only, because lingering is off. | High | The installer turns lingering on and fails loudly when it cannot. `live_service_started_before_login` proves the start at boot from the clocks of the system. |
| The service restarts without end after the owner removed the machine. | Medium | Exit 0 stops for good, by decision 17. `Restart=on-failure` and `SuccessfulExit` false both leave an exit 0 alone. The result file asks plan `go-s3-client-link` to exit 0 on removal. |
| The service does not find a harness that the user's shell finds. | Medium | The installer copies its own search path into the service. `unsafe_path_entries_dropped` checks the copy. A harness installed later in a new folder needs a second run, which the guide says. |
| The token is visible to another user of the computer while the join runs. | Low | The join takes seconds, and the token works once. The result file asks plan `go-s3-client-link` to read the token from standard input. |
| A fixture client of the user-flow checks shares the state folder of the installed client. | Medium | The installer sets `LGTM_CLIENT_STATE_DIR` only inside the service, so the checks' `XDG_STATE_HOME` still decides for their clients. |
| A Mac starts the client only at login. | Low | Decision 13 records it, and the guide says it. |

**Verify:** `bash deploy/test/installer.sh` on any Linux machine, with no Docker. `LGTM_INSTALL_LIVE=1 deploy/test/installer-live.sh` and its `--after-reboot` part on a Linux machine with Docker, the stack and a systemd user manager. A person restarts that machine. A person runs the macOS step of item 4 on a Mac. `deploy/test/checks.sh` on the same Linux machine.

**Rollout detail:** steps 2, 4 and 5 of the rollout order. The live test runs on a disposable Linux machine or user, because it installs a real service.

**WS-3: The web server carries the link between a machine and the server**

The design has the machine dial the server and keep one two-way stream open, and it lets a machine reach a loopback address over plain HTTP/2.
In the stack, the machine's connection reaches the web server first, because the server is reachable only inside the stack.
Plan `go-s2-stack-web` writes the web server's rules for the website's calls and their streams.
This workstream proves that the same rules carry the link's route and its two-way stream, without encryption, from the loopback port to the server.
It changes the rules only where its tests fail.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 3a | A call to the link's route reaches the server through the web server, over HTTP/2 to the server. | `link_join_route_reaches_server` and `link_uses_http2_to_server` in `bash deploy/test/link-proxy.sh`. |
| 3b | A two-way stream of the link stays open through a silence of 40 seconds and echoes each message within 3 seconds. | `link_open_stream_is_full_duplex`. |
| 3c | Stopping the server ends the stream within 5 seconds. | `link_stream_ends_when_server_stops`. |
| 3d | The website's page and its calls still reach their targets after the change. | `website_still_served` and `website_calls_reach_server`. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 2.1 | Write the echo program. | `deploy/test/linkecho/main.go`, `serve` and `dial` (target). Pattern: the trial program in `scouting.md`, "A two-way stream through Caddy". |
| 2.2 | Write the override that puts the echo program in place of the server. | `deploy/test/linkecho.compose.yaml`, service `server` (target). Pattern: the override in `deploy/test/checks.sh`, `check_c2` at line 369. |
| 2.3 | Write the test. | `deploy/test/link-proxy.sh`, `run_test` (target). Pattern: `deploy/test/checks.sh`, `run_check` at line 658. |
| 2.4 | Change the web server's rules only where the tests fail. | `deploy/Caddyfile`, global `servers` option `protocols` and the route for `/lgtm.link.v1.*` (target of plan `go-s2-stack-web`). Pattern: the trial's Caddyfile in `scouting.md`. |

**Work item 2. The link through the web server**

- **Files:** creates `deploy/test/linkecho/main.go`, `deploy/test/linkecho.compose.yaml` and `deploy/test/link-proxy.sh` (executable). Changes `deploy/Caddyfile` only where the tests fail on the merged file of sprint 2. The test writer creates all three new files, because the echo program and the override are test code. The builder changes only `deploy/Caddyfile`.
- **Needs first:** plan `go-s2-stack-web`, items 1 to 3, merged. It runs at the same time as items 1 and 3.
- **Work area and complexity:** `infra`, medium. It is a rule of a web server and a test with a small program, and the approach was tried on 2026-11-26.
- **Scope:** `stack/infra`.
- **Design statements:** C6, C8, C9, C10, C11. The section "One connection, dialled by the machine". The section "Encryption on the link", on plain HTTP/2 to a loopback address. The section "The compose stack", on one shared address.
- **Risky:** no. It touches no sign-in, key or token. Its tests must still fail on two changes of the merged rules, run on a temporary branch: removing `h2c` from the `protocols` option must fail `link_open_stream_is_full_duplex`, and removing `/lgtm.link.v1.*` from the route must fail `link_join_route_reaches_server`. When the merged rules of sprint 2 already pass every test, these two changes are the proof that the tests can fail, and the item changes no line of `deploy/Caddyfile`.
- **The echo program**, with the standard library only, in the root module:
  - `linkecho serve <listen address>` serves HTTP/1.1 and HTTP/2 without encryption, using `http.Protocols` with `SetUnencryptedHTTP2(true)`.
  - A `POST` to `/lgtm.link.v1.LinkService/Open` turns on full duplex with `http.ResponseController.EnableFullDuplex`, answers 200, and writes `echo <line>` and a newline for each line it reads, flushing after each one. It writes `{"msg":"open","proto":"<protocol of the request>"}` as one line to standard error.
  - Any other path answers 200 with the body `linkecho` and a newline.
  - `linkecho dial <url> --messages <n> --interval <d> --silence <d>` opens a `POST` over HTTP/2 without encryption, sends the numbers 0 to n−1 one per line, waits `<d>` between two messages, and waits `--silence` once in the middle instead. It expects `echo <i>` for each number within 3 seconds. Success prints `echoed <n> messages over <s> seconds` and exits 0. An ended stream prints `stream ended: <error>` and exits 1. A missing echo prints `no echo for message <i> within 3 seconds` and exits 1.
- **The override** `deploy/test/linkecho.compose.yaml` keeps the service name `server`, so the web server's rules reach it unchanged. It mounts a static build of the echo program read-only at `/linkecho`, sets the entry point to `/linkecho serve` with the server's listening port (request 8 in the result file), and resets the server's volumes, environment and dependencies. The test runs `web` and `server` only, with `--no-deps`, under a Compose project of its own named `lgtm-linkproxy-<random>`, and publishes a free loopback port of its own. It never touches the owner's stack.
- **Tests written first:** `deploy/test/link-proxy.sh` builds the echo program with `CGO_ENABLED=0 go build -o <temporary file> ./deploy/test/linkecho`, starts the override, prints one TAP line per test, and removes its project at the end.

| Test | What it asserts |
|---|---|
| `link_join_route_reaches_server` | `curl --http2-prior-knowledge` with a `POST` to `/lgtm.link.v1.LinkService/Join` at the test's port returns status 200 and the body `linkecho`. |
| `link_uses_http2_to_server` | After one `linkecho dial`, the echo program's output in `docker compose logs server` holds `{"msg":"open","proto":"HTTP/2.0"}`. |
| `link_open_stream_is_full_duplex` | `linkecho dial http://127.0.0.1:<port>/lgtm.link.v1.LinkService/Open --messages 10 --interval 10s --silence 40s` exits 0 and prints `echoed 10 messages`. |
| `link_stream_ends_when_server_stops` | During `linkecho dial ... --messages 30 --interval 2s --silence 2s`, the test runs `docker compose stop server`. The dial exits 1 within 5 seconds of the end of that command and prints `stream ended`. |
| `website_still_served` | A `GET` of `/` returns a page that holds `id="app"`. |
| `website_calls_reach_server` | A `POST` to `/lgtm.v1.SessionService/GetServer` returns the body `linkecho`. |

- **Acceptance:** `bash deploy/test/link-proxy.sh` prints `ok` for all six tests and exits 0. The two changes named under "Risky" each make the named test print `not ok`.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The web server buffers the stream, so the client's messages arrive late or never. | High | `link_open_stream_is_full_duplex` requires each echo within 3 seconds. |
| The web server keeps the machine's side open after the server stops, so the client waits for a dead server. | Medium | `link_stream_ends_when_server_stops`. The trial of 2026-11-26 saw the stream reset about one second after the upstream stopped. |
| The test disturbs the owner's stack or another test's stack. | Medium | It uses a Compose project and a port of its own, and it starts only `web` and `server`. |
| The plan of sprint 2 names the server's port differently. | Low | Request 8 in the result file asks for the name, and the override reads it. |

**Verify:** `bash deploy/test/link-proxy.sh` on a Linux machine with Docker and the website image of sprint 2. No device is needed.

**Rollout detail:** step 1 of the rollout order. It needs the merged images of sprint 2, built with `docker compose -f deploy/compose.yaml build web server`.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe: the brief's work items 1 to 3, its three observations, its two agreements and its two points to settle; the four specific points of the task file `.foundry/go-rewrite/tasks/plan-go-s3-stack-client.md`; issues 1636 to 1640 in `docs/issues.md`; the names table of `deploy/test/README.md`; and the design statements D7, D16, D17, D19, R3, R9, R10, C1 to C13, and the design's sentences on the server's own command, the service of the user and `deploy/install-client.sh`.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the server's command that makes a token | WS-1 | Item 1. |
| Brief item 2, the installer and its service | WS-2 | Items 3 and 4. |
| Brief item 3, C5 to C11 turned on | WS-3, WS-2 | Item 2 is what the Stack team adds. Item 5 runs the whole script. Decision 20. |
| Brief observation, the installer ends with the machine waiting | WS-2 | `live_install_leaves_machine_waiting`. |
| Brief observation, the client starts by itself after a restart | WS-2 | `live_service_started_before_login`. |
| Brief observation, the user-flow script passes 13 checks | WS-2 | Criterion 2h. |
| Brief agreement, the Server team's lead merges item 1 | WS-1 | Item 1, scope `server/backend`. Decision 8. |
| Brief agreement, the name of the service and where the program goes | WS-2 | Public names table. Decisions 12 to 14. |
| Brief point 1, a service on macOS | WS-2 | Item 4. Decision 13. |
| Brief point 2, build or download | WS-2 | Item 3, step 9. Decision 11. |
| Task file, the script exists and is not rewritten | WS-3 | Decision 20. |
| Task file, issue 1638, where `hosts_server` comes from | WS-1 | `--hosts-server`. Decision 10. |
| Task file, the service comes back after a restart | WS-2 | Item 3, steps 13 and 14, and item 5. Decision 12. |
| Task file, the names of `deploy/test/README.md` | WS-1, WS-2, WS-3 | Kept, except the two that the result file lists. Decisions 22 and 23. |
| Issue 1636, the join command on the website | none | It concerns the website and the Client team. The installer uses the design's form. The result file records it. |
| Issue 1637, the comment of `Client.address` | none | It concerns the contract and the Server team. The result file records it. |
| Issue 1638 | WS-1 | Decision 10. |
| Issue 1639, replacing the server's keys | WS-2 | `--rejoin`. Decision 25. |
| Issue 1640, a sandbox without Docker | WS-2, WS-3 | Items 2 and 5 need Docker. Items 1, 3 and 4 do not. Execution sizing says so. |
| D7, a token that authorizes an exchange of keys | WS-1, WS-2 | Items 1 and 3. |
| D16, a restart of a connected client and a retry every 15 seconds | WS-2 | Exit 75 and `live_reconnect_restarts_service`. Decision 17. |
| D17, trust without a public authority | WS-1 | The token carries the fingerprints that the machines part fills in. |
| D19, one user on a machine | WS-2 | `refuses_root`. Decision 27. |
| R3, one part calls another part's interface | WS-1 | Item 1 calls the machines part. Request 1. |
| R9, secrets never reach a log | WS-1, WS-2 | `TestJoinTokenKeepsSecretOffStandardError` and `token_never_printed`. |
| R10, real PostgreSQL | WS-1 | Item 1's tests use `LGTM_TEST_POSTGRES`. |
| C1 to C4, C12, C13 | WS-2 | Criterion 2h runs them. Other plans make them pass. |
| C5 | WS-1, WS-2 | Item 1 and criterion 2h. The script change of decision 22. |
| C6 to C11 | WS-3, WS-2 | Item 2 and criterion 2h. |
| The design's "or with the server's own command" | WS-1 | Item 1. |
| The design's "a service of its user, outside the compose stack" | WS-2 | Items 3 and 4. |
| The design's "`deploy/install-client.sh` ... joins it with a token the server's own command makes" | WS-2 | Item 3, step 12. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "What this project is": the three parts meet only through the contract. | The installer and the command use the contract's `Join`, `GetServer` and `WatchClients`. The installer reads no file of the client except whether `server.json` exists. |
| `AGENTS.md`, "Where things are": new code does not go into `packages/`. | Every file is in `deploy/` or `src/server/cmd/lgtm-server/`. |
| The epic, "The teams, and what each may edit": the Stack team edits `deploy/`. | Items 2 to 5 edit only `deploy/`. Item 1 is the one file in the Server team's folder that the brief allows, and decision 8 records it. |
| `AGENTS.md`, habit 1: credit copied code. | Nothing is copied. The live test reuses the technique of `deploy/test/checks.sh` in the same repository. |
| `AGENTS.md`, habit 2: write down what you find. | The result file lists the problems for `docs/issues.md`, because this planner may write only its own folder and its result file. |
| `AGENTS.md`, "Writing style for documents". | The plan has full sentences and no private detail. The guide of item 5 follows the same style. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet at launch. This section names none.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on | Can run alongside | Design statements |
|---|---|---|---|---|---|---|---|---|
| 1. The server's token command | `backend` | `server/backend` | medium | One small command over other items' interfaces, but it makes a join token. | no | `go-s2-server-signin` items 1 and 8; `go-s3-server-machines` items 2 and 4; `servertest` | 2, 3 | D7, D17, R3, R8, R9, R10, C5, C6 |
| 2. The link through the web server | `infra` | `stack/infra` | medium | A web server rule and a test with a small program, on an approach tried on 2026-11-26. | yes, it is the route a machine reaches | `go-s2-stack-web` items 1 to 3 | 1, 3 | C6, C8, C9, C10, C11 |
| 3. The installer on Linux | `infra` | `stack/infra` | medium | One script of known steps with several failure paths, and it passes a join token. | no | none to build; names from item 1 and `go-s3-client-link` | 1, 2 | D7, D16, D19, R9, C6 |
| 4. The installer on macOS | `infra` | `stack/infra` | medium | A second branch of a known script, verified on Linux and by a person on a Mac. | no | 3 | 1, 2 | D16, D19, R9 |
| 5. The live test and the guide | `infra` | `stack/infra` | medium | It drives the real stack and service manager, and one part needs a restart. | no | 1, 2, 3; `go-s3-client-link` item 6; `go-s3-server-machines` items 3 to 9 | 4 | D16, D19, C5, C6, C7, C9 |

Load: five test writers, five builders, and two mutation runs for the risky items 1 and 3, which gives 12 execution seats. Item 2 adds two mutation runs as a proof that its tests can fail. Alternating the two primary vendors gives seven seats to one and seven to the other. Items 2 and 5 need a seat with access to Docker and to the loopback network, by issue 1640. Items 1, 3 and 4 do not need Docker, and item 1 needs only the disposable PostgreSQL of `deploy/test/with-postgres.sh`. A review round of this plan uses the seats that the epic names. The orchestrator checks the weekly limits at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| Round 1 | Not run yet. | The orchestrator runs the review rounds. The planner stopped after the self-check. |

</details>
