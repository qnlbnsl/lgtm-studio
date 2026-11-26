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

The user-flow script `deploy/test/checks.sh` already exists, and it must pass all 13 of its checks at the end of the sprint. Plan `go-s2-stack-web` already proves that its web server carries the link's two-way stream on the loopback address. This plan adds only a small change to the script: its clients get their state folder through the client's own setting.

The contract of the link now states only what the other end can observe. The exit statuses of the client, what the service manager does with them, and which address the installer fills in are rules of the programs. This plan states the rules of the service and the installer once, in its section "The service, the address and the exit statuses", each with the test that proves it.

On macOS, the client starts when the owner logs in, not at boot. Starting at boot would need an administrator to install a system service. The design names no tool for macOS, so the plan chose the one that stays within the owner's user.

## Key decisions

- **The installer builds the client from the checkout.** No built client is published anywhere yet, and nothing would sign one. The checkout is already on the machine, because the stack starts from it.
- **On Linux the client is a systemd user service, and the user lingers.** Lingering is the setting that lets a user's services start at boot without a login. When the installer cannot turn it on, it says so and exits with a failure, but the client is still installed.
- **The installer joins before it installs the service.** A failed join is a failure of the installer, and it leaves nothing running.
- **The token marks the server's own machine.** The machine never claims the mark itself, so a remote machine cannot pose as the server's machine.
- **The service starts the client again after every exit except 0.** Plan `go-s3-client-link` makes `run` exit 0 for a stop for good, 75 for a restart that the server asked for, and 1 for a failure.
- **The installed client dials the stack's own loopback port.** The installer asks Compose for the published port, so a port set only in `deploy/.env` is still found.
- **A second run is safe.** It installs the newly built program and restarts the service, and it keeps the machine's identity. A separate option joins again from a fresh folder and keeps the old folder under another name.

Full log → `decisions.md`

## Workstreams

### The server makes a join token from its own command

The owner can make a token without the website, with one command inside the server's container. The command prints the token and nothing else, so a script can capture it. The token names the server's own public address, which is the only address the machines part puts in a token.

The risk is that the token's secret leaks into a log. The command writes nothing on success except the token on standard output, and the tests check that standard error stays empty.

When it is done, a person runs the command against the stack and gets one line that starts with `lgtm1_`. A machine that joins with it shows on the Machines page as the machine that runs the server.

### The client installs as a service that comes back by itself

`deploy/install-client.sh` takes the computer from a running stack to a joined client that runs as a service. It finds the stack's port, checks that the stack answers, builds the client, asks the server for a token, joins, and then installs and starts the service. A second run updates the program and keeps the identity. Other options join again or remove the service.

The main risk is a service that looks installed but does not start after a restart. A live test on a Linux machine kills the client and watches it come back. A person restarts the computer, and the test then proves from the clocks of the system that the client started before the first login. A second risk is a removed machine that the service starts again and again, which the live test rules out. A third risk is the token in the installer's output, which the tests rule out the same way as for the command.

When it is done, the installer ends with the machine waiting for approval. After approval the machine shows as online. After a restart of the computer it shows as online again. After a removal on the Machines page the client stays stopped.

## Rollout order

1. Build the installer on Linux, with its tests that need no stack. Its tests replace every outside program with a stand-in, so it can start before any link code exists. Its names from plans `go-s3-server-machines` and `go-s3-client-link` freeze when both of those plans have cleared review, by the epic's rule of 2026-11-26.
2. Build the server's token command when the Server team's token and join code are merged. The Server team's lead merges it.
3. Build the macOS part of the installer after the Linux part is merged, because both change the same script.
4. Run the live test of the installer on a Linux machine once the client and the server of sprint 3 are merged. A person restarts the computer between its parts.
5. Run the whole user-flow script. All 13 checks must pass before the sprint is handed over.

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
| `lgtm-server join-token [--hosts-server]` | command | Makes one join token for the server's public address. Prints its text, one line that starts with `lgtm1_`, on standard output. Exit 0 on success, 1 when the settings, the database or the vault fail, and 2 for a wrong argument. Every message is one sentence on standard error. | item 1 |
| `joinTokenCommand(ctx context.Context, args []string, lookup func(string) (string, bool), stdout, stderr io.Writer) int` | Go function in package `main` of `src/server/cmd/lgtm-server` | The body of the command, in the dispatcher's table of plan `go-s2-server-signin`, item 9. `args` excludes the program's name and the word `join-token`. | item 1 |
| `deploy/install-client.sh [--rejoin \| --remove]` | command | Installs, joins again, or removes the client on this computer. Exit 0 on success, 1 on a failure, 2 for a wrong argument. | items 2 and 3 |
| `COMPOSE_FILE`, `COMPOSE_PROJECT_NAME`, `COMPOSE_ENV_FILES`, `LGTM_STACK_STATE_DIR` | variables passed through | The installer sets `COMPOSE_FILE` to `deploy/compose.yaml` when it is unset or empty, calls `docker compose` without `-f`, and passes the others on, as `deploy/up.sh` of plan `go-s2-stack-web` does. | item 2 |
| `$HOME/.local/bin/lgtm-client` | file | The installed program, mode `0755`. | item 2 |
| `lgtm-client.service` at `${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/` | file | The user unit on Linux. | item 2 |
| `deploy/lgtm-client.service` | template | The unit with the placeholders `@LGTM_CLIENT_BIN@`, `@LGTM_CLIENT_STATE_DIR@` and `@LGTM_CLIENT_PATH@`. | item 2 |
| `dev.lgtmstudio.client` | launchd label | The agent on macOS, at `$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist`. | item 3 |
| `deploy/dev.lgtmstudio.client.plist` | template | The agent with the placeholders of the unit and `@LGTM_CLIENT_LOG@`. | item 3 |
| `$HOME/Library/Logs/lgtm-client.log` | file | The output of the client on macOS. On Linux the output goes to the journal of the user. | item 3 |
| `lgtm-client.replaced-<YYYYMMDDTHHMMSSZ>` | folder beside the state folder | The old state folder after `--rejoin`. | item 2 |

The names this plan reads from other plans:

| Name | Plan |
|---|---|
| `config.Load(lookup)` and its fields `PublicURL`, `Postgres`, `AllowPrivateStack`, `OpenBaoAddr`, `OpenBaoCredentialFile` and `TLSCertFile`. | `go-s2-server-signin` item 1, and `go-s3-server-machines` item 14 for `TLSCertFile`. |
| `store.Open(ctx, config, policy)`, `vault.Open(addr, credentialFile, allowPrivateStack)`, `vault.LoadLinkIdentity(ctx, store, client, false)` and `LinkIdentity.SigningKey`. | `go-s1-server-store`; `go-s2-server-signin` items 8 and 9. |
| `machines.NewRegistry()`, `machines.NewTokens(store, registry, machines.Issuer{PublicURL, ServerKey, CertificateFile})`, `Tokens.Create(ctx, machines.TokenOptions{HostsServer})`, `NewToken.Text` and `machines.DecodeToken`. | `go-s3-server-machines` items 1 and 3. |
| `servertest.Start`, `Server.URL`, `Server.SigningPublicKey`, `Server.Environment()`, `Server.Clients(t)` and `Server.Link()`. | `go-s2-server-signin` item 9; `go-s3-server-machines` item 14. |
| `lgtm-client join <address> <token>`, `lgtm-client run`, `lgtm-client version`, `-X main.version=<v>`, and their exit statuses. | `go-s1-client-local` item 1; `go-s3-client-link` item 9. |
| `LGTM_CLIENT_STATE_DIR`, the default `$HOME/.local/state/lgtm-client`, and the files `identity.json` and `machine.key`. | `go-s1-client-local`, "The layout of the state folder". |
| The Compose project `lgtm`, the service `server` as user 10001 with `/usr/local/bin/lgtm-server`, the service `web` and its port 8080, and `LGTM_PUBLIC_URL` equal to `http://127.0.0.1:${LGTM_WEB_PORT:-8787}`. | `go-s1-stack-vault`; `go-s2-stack-web` items 1 and 4. |

**The service, the address and the exit statuses**

These rules belong to this plan, because the contract states only facts of the wire. Each is stated here once.

| Rule | Proved by |
|---|---|
| The installer fills in the address `http://` followed by the output of `docker compose port web 8080`, which must be `127.0.0.1:<port>`. The stack's `LGTM_PUBLIC_URL` comes from the same port, so the address agrees with the address inside the token. | `asks_server_for_hosts_server_token` and `live_install_leaves_machine_waiting`. |
| A failing `lgtm-client join` is a failure of the installer. It exits 1 and installs no service. | `join_failure_installs_no_service`. |
| The service starts the client again 5 seconds after every exit except 0, without a limit on the count. On Linux this is `Restart=on-failure`, `RestartSec=5` and `StartLimitIntervalSec=0`. On macOS it is `KeepAlive` with `SuccessfulExit` false and `ThrottleInterval` 5. | `installs_linux_user_service`, `macos_agent_parses`, `live_service_restarts_after_crash`. |
| A restart that the server asked for ends `run` with status 75, and the service starts the client once more. | `live_reconnect_restarts_service`. On a Mac, the person step of item 3. |
| A removal ends `run` with status 0, and the service leaves the client stopped. | `live_removed_machine_stays_stopped`. |
| The service and the installer's `join` use the same state folder, passed as `LGTM_CLIENT_STATE_DIR`. | `joins_before_service_starts`. |

Plan `go-s3-client-link` proves the statuses of the program itself, in `TestRunCommandExitStatus`. It also proves that a restart's result is sent from its receipt after the new start, in `TestRestartReplaysAfterDroppedResult`.

**WS-1: The server makes a join token from its own command**

The design lets the owner make a token "with the server's own command", and the installer needs that path because it cannot use the website.
The command is a short program run inside the server's container.
It opens the server's settings, database and vault the same way the server does, without the server's lock and without migrating.
It asks the machines part to make a token, by rule R3, and prints the text.
It adds one fact that nothing else on the link carries: the mark that the joining machine runs the server.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1a | The command prints one token line, and a client can join with that token once. | `TestJoinTokenPrintsOneTokenLine`, `TestJoinTokenCarriesAddressAndServerKey` and `TestJoinTokenJoinsOnce` pass with `LGTM_TEST_POSTGRES` set. |
| 1b | A token made with `--hosts-server` gives a machine whose `hosts_server` is true, and a token made without it gives false. | `TestJoinTokenHostsServerFlag` passes. |
| 1c | A wrong argument and a missing database are refused, each with one sentence. | `TestJoinTokenRefusesUnknownArgument` and `TestJoinTokenFailsWithoutDatabase` pass. |
| 1d | The command prints a token inside the running stack. | `docker compose exec -T server lgtm-server join-token --hosts-server` prints exactly one line that matches `^lgtm1_[A-Za-z0-9_-]+$` and exits 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1.1 | Write the command and its argument rules. | `src/server/cmd/lgtm-server/join_token.go`, `joinTokenCommand` (target). Pattern: `src/server/cmd/lgtm-server/main.go`, `run`, of plan `go-s2-server-signin`, item 9. |
| 1.2 | Write the tests against a real server and database. | `src/server/cmd/lgtm-server/join_token_test.go` (target). Pattern: `src/server/servertest/server_test.go`, `TestStartAndRestartServer`, of plan `go-s2-server-signin`, item 9. |

**Work item 1. The server's own command that makes a join token**

- **Files:** creates `src/server/cmd/lgtm-server/join_token.go` and `src/server/cmd/lgtm-server/join_token_test.go`. Changes the entry `join-token` of the dispatcher's table in `src/server/cmd/lgtm-server/main.go`, as plan `go-s2-server-signin`, item 9, provides. The test writer creates the test file.
- **Needs first:** plan `go-s2-server-signin`, items 1, 8 and 9. Plan `go-s3-server-machines`, items 1, 3, 4 and 14. It runs at the same time as item 2.
- **Work area and complexity:** `backend`, medium. It is one small command over interfaces that other items build, but it handles a join token.
- **Scope:** `server/backend`. The files sit in the Server team's folder, and the Server team's lead merges them. See decision 8.
- **Design statements:** D7, D17, R3, R8, R9, R10, C5, C6. The section "Joining: a token that authorizes an exchange of keys", step 1, and the section "The compose stack", the bullet on `deploy/install-client.sh`.
- **Risky:** yes, because it makes a join token. Two mutations must turn the tests red. Passing `false` for the mark in every case must fail `TestJoinTokenHostsServerFlag`. Writing the token's text to standard error must fail `TestJoinTokenPrintsOneTokenLine`.
- **Behaviour:**
  - The dispatcher's entry `join-token` calls `joinTokenCommand` with the dispatcher's own `lookup`.
  - The only argument is `--hosts-server`, which takes no value. Any other argument, a value, or a repeated flag gives exit 2, nothing on standard output, and one sentence on standard error that names the argument.
  - The command reads the settings with `config.Load(lookup)`. It opens the store with the settings' database and connection policy, opens the vault client, and reads the server's identity with `vault.LoadLinkIdentity` and `allowInitialize` false. It takes no instance lock, runs no migration and opens no port.
  - It calls `Tokens.Create` with an empty address, so the token names `PublicURL`, and with `HostsServer` equal to the flag. The issuer's `ServerKey` is the public half of `LinkIdentity.SigningKey`, and its `CertificateFile` is `TLSCertFile`.
  - On success it writes `NewToken.Text` and one newline to standard output, and nothing else. It writes nothing to standard error.
  - A failure of the settings, the store, the vault or the token gives exit 1 and one sentence on standard error. The sentence names the part that failed and holds no setting's value.
  - It closes the vault client and the store before it returns.
- **Tests written first:** in `src/server/cmd/lgtm-server/join_token_test.go`, package `main`. Each test starts a server through `servertest.Start` and calls `joinTokenCommand` with a lookup over `Server.Environment()`. A stub of `joinTokenCommand` that returns 0 and writes nothing makes every positive test fail on its first assertion.

| Test | What it asserts |
|---|---|
| `TestJoinTokenPrintsOneTokenLine` | With no argument, the exit code is 0. Standard output matches `^lgtm1_[A-Za-z0-9_-]+\n$` exactly. Standard error is empty. |
| `TestJoinTokenCarriesAddressAndServerKey` | `machines.DecodeToken` of the line gives a token whose `address` equals `Server.URL`, whose `token_id` is a version 7 universally unique identifier (UUID), whose `secret` has 32 bytes, whose `server_key_sha256` equals the SHA-256 hash (Secure Hash Algorithm, 256 bits) of `Server.SigningPublicKey`, and whose `certificate_key_sha256` is empty. |
| `TestJoinTokenJoinsOnce` | `Join` through `Server.Link()` with the token's `token_id`, its secret and a fresh Ed25519 public key succeeds. A second `Join` with the same token and another key returns `permission_denied` with `Lgtm-Join-Refusal` equal to `token_used` in the error's metadata. |
| `TestJoinTokenHostsServerFlag` | One token is made with `--hosts-server` and one without. A machine joins with each. The first message of `WatchClients` through `Server.Clients(t)` lists both as waiting. The machine of the first token has `hosts_server` true, and the other has false. |
| `TestJoinTokenRefusesUnknownArgument` | `--address x`, `--hosts-server=yes`, `--hosts-server` given twice, and a bare word each give exit 2, an empty standard output, and one line on standard error that ends with a full stop. |
| `TestJoinTokenFailsWithoutDatabase` | With the database port of the environment changed to a closed loopback port, the exit code is 1, standard output is empty, and standard error is one line that ends with a full stop and holds no value of the environment. |

- **Acceptance:** `LGTM_TEST_POSTGRES=<url> go test -count=1 -run 'TestJoinToken' ./src/server/cmd/lgtm-server/` passes all six tests. Criterion 1d holds against the stack.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The token's secret reaches a log of the command. | High | The command logs nothing. `TestJoinTokenPrintsOneTokenLine` requires an empty standard error on success. |
| A token marks a remote machine as the server's machine. | Medium | Only the command sets the mark, and it runs inside the server's container. `CreateClientToken` never sets it, by plan `go-s3-server-machines`, item 12. |
| The command and the running server disagree on the signing key. | Medium | The command reads the pinned identity through the server's own function, and `TestJoinTokenCarriesAddressAndServerKey` compares the token with the server's key. |

**Verify:** the six tests above against a real PostgreSQL, and criterion 1d against the stack. No device is needed.

**Rollout detail:** step 2 of the rollout order. The Server team's lead merges the item after a passing report on its exact commit.

**WS-2: The client installs as a service that comes back by itself**

The design says that the client "runs as a service of its user, outside the compose stack", and that `deploy/install-client.sh` "installs the client on the same machine and joins it".
This workstream writes that script, the service files it installs, and the tests that prove both on Linux and macOS.
The hermetic tests replace every outside program with a stand-in on `PATH`, so they need no Docker, no network and no service manager.
The live test runs the real installer against the real stack and the real service manager of a Linux user.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2a | On Linux, the installer finds the port, builds the program, joins with a token of the server's command, and installs and starts the user unit, in that order. | `installs_linux_user_service`, `asks_server_for_hosts_server_token` and `joins_before_service_starts` in `bash deploy/test/installer.sh`. |
| 2b | The token never appears in the installer's output or in the files it writes. | `token_never_printed`. |
| 2c | A failed join installs no service, and each refusal of a precondition stops the installer before it changes anything. | `join_failure_installs_no_service`, `refuses_root`, `refuses_stack_not_answering`, `refuses_malformed_token`, `refuses_missing_user_manager` and `refuses_unsafe_home`. |
| 2d | A second run keeps the identity, `--rejoin` keeps the old folder under a new name, and `--remove` removes the service and keeps the state. | `rerun_keeps_joined_identity`, `rejoin_moves_old_state` and `remove_stops_and_deletes_service`. |
| 2e | The user-flow script gives its clients their state folder through `LGTM_CLIENT_STATE_DIR`. | `grep -c XDG_STATE_HOME deploy/test/checks.sh` prints 0, and `deploy/test/checks.sh --only C5,C6,C10` prints three `ok` lines. |
| 2f | On macOS, the installer loads a launchd agent that parses as a property list. | `installs_macos_launch_agent`, `macos_agent_parses` and `macos_remove_boots_out_agent`. The person steps of item 3 on a Mac. |
| 2g | On a real Linux machine, the installer leaves the machine waiting, the service restarts after a crash and after the server's restart command, and a second run keeps the machine. | The first six tests of `LGTM_INSTALL_LIVE=1 deploy/test/installer-live.sh`. |
| 2h | After a restart of the computer, the client started before the first login and shows as online. | A person runs `sudo systemctl reboot`, logs in, and runs `deploy/test/installer-live.sh --after-reboot`. It reports `ok` for `live_service_started_before_login` and `live_machine_online_after_reboot`. |
| 2i | A machine removed on the Machines page stays stopped. | `deploy/test/installer-live.sh --cleanup` reports `ok` for `live_removed_machine_stays_stopped`. |
| 2j | The whole user-flow script passes. | `deploy/test/checks.sh` prints 13 lines that start with `ok` and exits 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 2.1 | Write the installer for Linux. | `deploy/install-client.sh`, `main` (target). Pattern: `deploy/up.sh` of plan `go-s2-stack-web`, item 5, and `deploy/test/checks.sh`, `get_server` at line 60. |
| 2.2 | Write the user unit template. | `deploy/lgtm-client.service`, section `[Service]` (target). Pattern: the systemd unit format in `scouting.md`. |
| 2.3 | Write the hermetic tests and their stand-ins. | `deploy/test/installer.sh`, `run_test` (target). Pattern: `deploy/test/checks.sh`, `run_check` at line 658, for one Test Anything Protocol (TAP) line per test. |
| 2.4 | Give the user-flow clients their state folder. | `deploy/test/checks.sh`, `client_join` at line 219 and `start_client` at line 233, and `deploy/test/README.md`, the row "Client state folder". |
| 3.1 | Add the macOS branch and the agent template. | `deploy/install-client.sh`, `install_launch_agent` (target), and `deploy/dev.lgtmstudio.client.plist` (target). Pattern: the launchd property list format in `scouting.md`. |
| 4.1 | Write the live test of the installer. | `deploy/test/installer-live.sh`, `after_reboot` (target). Pattern: `deploy/test/checks.sh`, `open_watch` at line 238 and `read_frame` at line 251. |
| 4.2 | Add the guide for installing the client. | `deploy/README.md`, section "Installing the client" (target). |

**Work item 2. The installer on Linux**

- **Files:** creates `deploy/install-client.sh` (executable), `deploy/lgtm-client.service` and `deploy/test/installer.sh` (executable). Changes lines 219 and 233 of `deploy/test/checks.sh` and one row of `deploy/test/README.md`. The test writer creates the test file and its stand-ins, which the test writes into a temporary folder at each run.
- **Needs first:** nothing to write and to pass its own tests. It runs at the same time as item 1.
- **Work area and complexity:** `infra`, medium. It is one script of known steps with a few failure paths, and it passes a join token.
- **Scope:** `stack/infra`.
- **Design statements:** D7, D16, D19, R9, C6. The section "The client in wave 1", on the state folder and the service of the user. The section "The compose stack", the bullet on `deploy/install-client.sh`.
- **Risky:** yes, because it passes a join token. Two mutations must turn the tests red. Printing the token with `printf '%s\n' "$token"` must fail `token_never_printed`. Moving the service step in front of the join must fail `joins_before_service_starts`.
- **Behaviour, in order:**
  1. The script changes to the repository root, as `deploy/up.sh` does. It accepts no argument, `--rejoin` or `--remove`. Anything else prints the usage and exits 2.
  2. It refuses to run when `id -u` is 0, with `install-client.sh must run as the user who runs the harnesses, not as root.` and exit 1.
  3. It reads `uname -s`. `Linux` takes the steps below. `Darwin` takes the steps of item 3. Anything else gives `install-client.sh supports Linux and macOS only.` and exit 1.
  4. On Linux it runs `systemctl --user show-environment`. A failure gives `This Linux system has no systemd user manager for this user.` and exit 1.
  5. It refuses a `HOME` that holds a character outside letters, digits and `. _ / + @ -`, with `The home folder path holds a character that a service file cannot carry.` and exit 1.
  6. It works out two values. The state folder is `LGTM_CLIENT_STATE_DIR` when that is set, and `$HOME/.local/state/lgtm-client` otherwise. A value that is not an absolute path, or that holds a character refused in step 5, gives `LGTM_CLIENT_STATE_DIR must be an absolute path.` and exit 1. The service's search path is the installer's `PATH` without the entries that hold a character outside letters, digits and `. _ / + @ : -`.
  7. `--remove` stops here. It runs `systemctl --user disable --now lgtm-client.service`, ignoring a unit that is not loaded. It deletes the unit file and the program, runs `systemctl --user daemon-reload`, prints one sentence, and exits 0. It keeps the state folder.
  8. It sets `COMPOSE_FILE` and runs `docker compose port web 8080`. Output other than one line `127.0.0.1:<port>` gives `The stack's web service publishes no loopback port. Start it with deploy/up.sh.` and exit 1. The address is `http://127.0.0.1:<port>`.
  9. It calls `SessionService/GetServer` at the address with `curl`, as `get_server` in `deploy/test/checks.sh` does. A failure, or a body without `"product"` and `"lgtm-studio"` as a pair, gives `The stack does not answer at <address>. Start it with deploy/up.sh.` and exit 1.
  10. It builds the client with `CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=<version>" -o <temporary file> ./src/client/cmd/lgtm-client`. The temporary file sits in `$HOME/.local/bin/`. The version is `0.0.0-` and the first 12 hexadecimal digits of `git rev-parse HEAD`, with `-dirty` when `git status --porcelain` prints anything. Without Git it is `0.0.0-dev`. The script runs `<temporary file> version`, renames the file to `$HOME/.local/bin/lgtm-client` with mode `0755`, and prints `built lgtm-client <version>`.
  11. With `--rejoin`, it runs `systemctl --user stop lgtm-client.service`, ignoring a unit that is not loaded. It then renames the state folder to `lgtm-client.replaced-<time>` beside it, when the folder exists.
  12. When `<state folder>/identity.json` exists and `--rejoin` was not given, it prints `lgtm-client is already joined.` and skips step 13.
  13. It runs `docker compose exec -T server lgtm-server join-token --hosts-server`, and keeps standard output in a variable. Output that does not match `^lgtm1_[A-Za-z0-9_-]+$` gives `The server's join-token command returned no token.` and exit 1. It then runs `LGTM_CLIENT_STATE_DIR=<state folder> $HOME/.local/bin/lgtm-client join <address> <token>`. A failure passes on the client's output, prints `lgtm-client could not join the server.`, and exits 1, with no service installed. The variable that held the token is cleared after the join.
  14. It renders `deploy/lgtm-client.service` with the three placeholders into a temporary file beside the unit's path, and renames it to the unit's path. It runs `systemctl --user daemon-reload`, `systemctl --user enable lgtm-client.service` and `systemctl --user restart lgtm-client.service`.
  15. When `loginctl show-user <user> --property=Linger --value` does not print `yes`, it runs `loginctl enable-linger <user>`. A failure prints `loginctl enable-linger <user> failed, so the client starts only after this user logs in.` and sets the exit status to 1 for the end of the run.
  16. It waits up to 10 seconds for `systemctl --user is-active lgtm-client.service` to print `active`. A timeout gives `lgtm-client.service did not start.` and exit 1.
  17. It prints `lgtm-client runs as the user service lgtm-client.service.` and exits with 0, or with the status of step 15.
- **The unit template** `deploy/lgtm-client.service` holds these lines and no other setting:
  - `[Unit]`, `Description=LGTM Studio client`, `StartLimitIntervalSec=0`.
  - `[Service]`, `Type=simple`, `ExecStart=@LGTM_CLIENT_BIN@ run`, `Environment=LGTM_CLIENT_STATE_DIR=@LGTM_CLIENT_STATE_DIR@`, `Environment=PATH=@LGTM_CLIENT_PATH@`, `Restart=on-failure`, `RestartSec=5`, `TimeoutStopSec=10`.
  - `[Install]`, `WantedBy=default.target`.
- **The change of the user-flow script:** line 219 runs the client's `join` with `LGTM_CLIENT_STATE_DIR=$1/lgtm-client` in place of `XDG_STATE_HOME=$1`. Line 233 runs `run` with `LGTM_CLIENT_STATE_DIR=$state_home/lgtm-client` in place of `XDG_STATE_HOME=$state_home`. The README's row "Client state folder" says that the checks set `LGTM_CLIENT_STATE_DIR` to the `lgtm-client` child of a temporary folder. The client no longer reads `XDG_STATE_HOME`, by plan `go-s1-client-local`. No other line of the script changes.
- **Tests written first:** `deploy/test/installer.sh` prints one TAP line per test, `ok <n> - <test id>` or `not ok <n> - <test id> <reason>`, and exits 0 only when all pass.
  - **The stand-ins.** Each test makes a temporary folder with `home/` as `HOME`, `shims/` first on `PATH`, and `calls.log`. The test unsets `LGTM_CLIENT_STATE_DIR` unless a test sets it. The shims are `go`, `docker`, `systemctl`, `loginctl`, `launchctl`, `uname`, `id` and `curl`. Each appends one line with its name and arguments to `calls.log`, so the order of calls is one file. Variables of the test steer them: `SHIM_UNAME`, `SHIM_UID`, `SHIM_WEB_PORT`, `SHIM_CURL_FAIL`, `SHIM_TOKEN`, `SHIM_JOIN_EXIT`, `SHIM_LINGER_FAIL` and `SHIM_NO_USER_MANAGER`. The `id` shim answers `-u` with `SHIM_UID` and passes other arguments to the real `id`. The `go` shim, called with `build ... -o <file>`, writes a stand-in client to `<file>`. That client answers `version` with `0.0.0-test`. It answers `join <address> <token>` by appending `client join <address> <SHA-256 of the token> <LGTM_CLIENT_STATE_DIR>` to `calls.log` and creating `identity.json` and `machine.key` in that folder, or by printing `{"msg":"join_refused","code":"token_used"}` and exiting with `SHIM_JOIN_EXIT` when that is not 0. The `docker` shim answers `compose port web 8080` with `127.0.0.1:${SHIM_WEB_PORT:-8787}`, and `compose exec` with `SHIM_TOKEN`, whose default is a fixed `lgtm1_` text of 60 characters. The `curl` shim prints `{"product": "lgtm-studio", "contract": 1}` unless `SHIM_CURL_FAIL` is set.
  - A stub `deploy/install-client.sh` that exits 0 and does nothing makes every positive test fail on its first assertion.

| Test | What it asserts |
|---|---|
| `installs_linux_user_service` | With `SHIM_UNAME=Linux`, the exit code is 0. `$HOME/.local/bin/lgtm-client` exists with mode `0755`. `$HOME/.config/systemd/user/lgtm-client.service` equals `deploy/lgtm-client.service` with `@LGTM_CLIENT_BIN@` replaced by the program's path, `@LGTM_CLIENT_STATE_DIR@` by `$HOME/.local/state/lgtm-client`, and `@LGTM_CLIENT_PATH@` by the test's search path. The unit holds `Restart=on-failure`, `RestartSec=5` and `StartLimitIntervalSec=0`. `calls.log` holds, in this order, `systemctl --user daemon-reload`, `systemctl --user enable lgtm-client.service` and `systemctl --user restart lgtm-client.service`. It holds `loginctl enable-linger` with the user's name. |
| `asks_server_for_hosts_server_token` | `calls.log` holds `docker compose port web 8080` before exactly one `docker compose exec -T server lgtm-server join-token --hosts-server`. The `client join` line has the address `http://127.0.0.1:8787`. A run in a fresh folder with `SHIM_WEB_PORT=8899` has the address `http://127.0.0.1:8899`. A run with `SHIM_WEB_PORT=` empty exits 1 and holds no `go` line. |
| `joins_before_service_starts` | The `client join` line comes before the first `systemctl --user enable` line in `calls.log`. Its hash equals the SHA-256 of `SHIM_TOKEN`. Its folder equals the value of `LGTM_CLIENT_STATE_DIR` in the unit file. With `LGTM_CLIENT_STATE_DIR` set to an absolute folder of the test, the join line and the unit both name that folder. With it set to `relative/dir`, the exit code is 1 and `calls.log` holds no `go` line. |
| `token_never_printed` | The installer's standard output and standard error do not hold `SHIM_TOKEN`. No file under the test's `HOME` holds it. |
| `join_failure_installs_no_service` | With `SHIM_JOIN_EXIT=1`, the exit code is 1. The output holds `join_refused`. No unit file exists. `calls.log` holds no `systemctl --user enable` line. |
| `refuses_root` | With `SHIM_UID=0`, the exit code is 1, the output holds `not as root`, and `calls.log` holds no `go` line. |
| `refuses_stack_not_answering` | With `SHIM_CURL_FAIL=1`, the exit code is 1, the output holds `deploy/up.sh`, and `calls.log` holds no `docker compose exec` line and no `go` line. |
| `refuses_malformed_token` | With `SHIM_TOKEN=oops`, the exit code is 1, `calls.log` holds no `client join` line, and no unit file exists. |
| `refuses_missing_user_manager` | With `SHIM_NO_USER_MANAGER=1`, the exit code is 1, and `calls.log` holds no `go` line. |
| `refuses_unsafe_home` | With `HOME` set to a folder whose name holds a space, the exit code is 1, and `calls.log` holds no `go` line. |
| `unsafe_path_entries_dropped` | With a search path that holds an entry with `%` and an entry with a space, the unit's `PATH` line holds neither entry and keeps the others in their order. |
| `linger_failure_is_reported` | With `SHIM_LINGER_FAIL=1`, the unit file exists, the `restart` line is in `calls.log`, the output names `loginctl enable-linger`, and the exit code is 1. |
| `rerun_keeps_joined_identity` | After a first run, a second run exits 0. `calls.log` of the second run holds no `docker compose exec` line and no `client join` line. It holds `daemon-reload` and `restart`. `identity.json` and `machine.key` are unchanged byte for byte. |
| `rejoin_moves_old_state` | After a first run, `--rejoin` exits 0. A folder whose name matches `^lgtm-client\.replaced-[0-9]{8}T[0-9]{6}Z$` sits beside the state folder and holds the first `machine.key`. The `systemctl --user stop lgtm-client.service` line comes before the new `client join` line. A new `machine.key` exists in the state folder. |
| `remove_stops_and_deletes_service` | After a first run, `--remove` exits 0. `calls.log` holds `systemctl --user disable --now lgtm-client.service`. The unit file and the program are gone. The state folder and its files are unchanged. |
| `usage_on_unknown_argument` | `--frobnicate` gives exit 2 and an empty `calls.log`. |

- **Acceptance:** `bash deploy/test/installer.sh` prints `ok` for the 16 tests above and exits 0, with no Docker, network or service manager on the test machine. Criterion 2e holds.

**Work item 3. The installer on macOS**

- **Files:** changes `deploy/install-client.sh`. Creates `deploy/dev.lgtmstudio.client.plist`. Changes `deploy/test/installer.sh`, where the test writer adds the tests below.
- **Needs first:** item 2, because both change the same two files.
- **Work area and complexity:** `infra`, medium. It is a second branch of a known script, and no machine of the checks runs macOS.
- **Scope:** `stack/infra`.
- **Design statements:** D16, D19, R9. The section "The client in wave 1".
- **Risky:** no. It changes no step that touches the token, which item 2's tests keep covering on both systems.
- **Behaviour:**
  - With `uname -s` equal to `Darwin`, steps 4 and 15 of item 2 are skipped, and the service steps 7, 11, 14 and 16 use launchd. The state folder follows step 6, as on Linux.
  - The agent file is `$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist`, rendered from the template and renamed into place.
  - The template is a property list with these keys: `Label` `dev.lgtmstudio.client`; `ProgramArguments` with the program and `run`; `EnvironmentVariables` with `LGTM_CLIENT_STATE_DIR` and `PATH`; `RunAtLoad` true; `KeepAlive` as a dictionary with `SuccessfulExit` false; `ThrottleInterval` 5; `StandardOutPath` and `StandardErrorPath` both `@LGTM_CLIENT_LOG@`; `ProcessType` `Background`.
  - Installing runs `launchctl bootout gui/<uid>/dev.lgtmstudio.client`, ignoring an agent that is not loaded, then `launchctl bootstrap gui/<uid> <agent file>`, then `launchctl enable gui/<uid>/dev.lgtmstudio.client`.
  - The wait of step 16 reads `launchctl print gui/<uid>/dev.lgtmstudio.client` for `state = running`.
  - `--rejoin` stops the agent with `launchctl bootout` before it moves the folder. `--remove` runs `launchctl bootout`, then deletes the agent file and the program.
- **Tests written first:** added to `deploy/test/installer.sh`, with `SHIM_UNAME=Darwin`.

| Test | What it asserts |
|---|---|
| `installs_macos_launch_agent` | The exit code is 0. The agent file equals the template with its four placeholders replaced, and `@LGTM_CLIENT_LOG@` becomes `$HOME/Library/Logs/lgtm-client.log`. `calls.log` holds `launchctl bootout`, `launchctl bootstrap gui/<uid> <agent file>` and `launchctl enable gui/<uid>/dev.lgtmstudio.client`, in that order, after the `client join` line. It holds no `systemctl` line and no `loginctl` line. |
| `macos_agent_parses` | The rendered agent file parses with `plutil -lint` when that program exists, and otherwise with the `plistlib` module of Python 3. The parsed `KeepAlive` is a dictionary whose `SuccessfulExit` is false, `ThrottleInterval` is 5, and `ProgramArguments` ends with `run`. When neither parser exists, the line is `ok <n> - macos_agent_parses # SKIP no parser`. |
| `macos_remove_boots_out_agent` | After a first run, `--remove` exits 0, `calls.log` holds `launchctl bootout gui/<uid>/dev.lgtmstudio.client`, and the agent file and the program are gone. |
| `refuses_unsupported_system` | With `SHIM_UNAME=FreeBSD`, the exit code is 1, the output holds `Linux and macOS only`, and `calls.log` holds no `go` line. |

- **Acceptance:** `bash deploy/test/installer.sh` prints `ok` for the four tests above and for the 16 tests of item 2. On a Mac with the stack running, a person runs `deploy/install-client.sh`, then `launchctl print gui/$(id -u)/dev.lgtmstudio.client`, and reads `state = running`. The person approves the machine and presses its restart on the Machines page, then reads `last exit code = 75` and `state = running` again. The person logs out, logs in, and reads `state = running` once more.

**Work item 4. The live test of the installer and its guide**

- **Files:** creates `deploy/test/installer-live.sh` (executable). Changes `deploy/README.md` with a section named "Installing the client". The test writer creates the test file. The builder writes the section.
- **Needs first:** items 1 and 2. Plan `go-s3-client-link`, item 9. Plan `go-s3-server-machines`, items 12 to 14.
- **Work area and complexity:** `infra`, medium. It drives the real stack and the real service manager, and one of its parts needs a restart of the computer.
- **Scope:** `stack/infra`.
- **Design statements:** D16, D19, C5, C6, C7, C9. The epic's sentence that the stack, the server and the client each come back by themselves after a restart.
- **Risky:** no. It adds no code that handles a token. The token passes through the code of items 1 and 2.
- **How the test runs:** it needs Linux, a stack started by `deploy/up.sh`, `LGTM_WEB_PASSWORD`, a systemd user manager, and Docker. Its default part also needs `LGTM_INSTALL_LIVE=1`, and it refuses to start when `lgtm-client.service` already exists, so it never replaces an installation of the owner. It reads the address the way the installer does. It signs in and reads `ClientService.WatchClients` the way `deploy/test/checks.sh` does in `signin`, `open_watch` and `read_frame`. The default part leaves the client installed for the reboot. `--after-reboot` checks the state after a restart. `--cleanup` removes the machine with `ClientService/RemoveClient`, runs its one test, and then runs `deploy/install-client.sh --remove`.
- **Tests written first:** each prints one TAP line. Before items 1 and 2 exist, the first test fails on the installer's exit code.

| Test | What it asserts |
|---|---|
| `live_install_leaves_machine_waiting` | `deploy/install-client.sh` exits 0. The first `WatchClients` message after it lists one more client than the one before it. That client is waiting for approval and has `hostsServer` true. |
| `live_service_enabled_with_linger` | `systemctl --user is-enabled lgtm-client.service` prints `enabled`. `is-active` prints `active`. `loginctl show-user <user> --property=Linger --value` prints `yes`. |
| `live_service_restarts_after_crash` | After `systemctl --user kill --signal=KILL lgtm-client.service`, the unit is active again within 15 seconds with another `MainPID`. |
| `live_approved_machine_online` | After `ClientService/ApproveClient` for that client, a `WatchClients` stream shows it online with `hostsServer` true within 35 seconds. |
| `live_reconnect_restarts_service` | After `ClientService/ReconnectClient` for that client, the unit's `NRestarts` grows by exactly one within 35 seconds, its `ExecMainStatus` of the ended run is 75, and a new `WatchClients` stream shows the client online with a later `connectedAt`. |
| `live_rerun_keeps_machine` | A second `deploy/install-client.sh` exits 0. The count of clients is unchanged. The same client is online again within 35 seconds. The SHA-256 of `identity.json` in the state folder is unchanged. |
| `live_service_started_before_login` | With `--after-reboot`: the unit's `ActiveEnterTimestampMonotonic` is smaller than the `TimestampMonotonic` of every session of the user whose `Class` is `user`. The test reads the sessions with `loginctl list-sessions` and `loginctl show-session`. It leaves out the sessions of class `manager`, because systemd opens one for the user's own service manager at boot when the user lingers. Both properties count microseconds from the same boot. |
| `live_machine_online_after_reboot` | With `--after-reboot`: within 60 seconds of the start of the test, a `WatchClients` stream shows the installed client online. |
| `live_removed_machine_stays_stopped` | With `--cleanup`, after `RemoveClient`: within 35 seconds the unit is `inactive`, and its `ExecMainStatus` is 0. Fifteen seconds later `NRestarts` is unchanged and the unit is still `inactive`. |

- **The guide:** the section "Installing the client" of `deploy/README.md` names the command, its two options, the paths of the program, the unit or agent and the state folder, how to read the client's output on each system, that a Mac starts the client at login, and that a removed machine joins again with `--rejoin`. It follows the repository's writing style.
- **Acceptance:** `LGTM_INSTALL_LIVE=1 deploy/test/installer-live.sh` prints `ok` for its first six tests. After the person's restart of criterion 2h, `deploy/test/installer-live.sh --after-reboot` prints `ok` for its two tests. `deploy/test/installer-live.sh --cleanup` prints `ok` for its one test and exits 0. `deploy/test/checks.sh` prints 13 `ok` lines and exits 0.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The service runs after a login only, because lingering is off. | High | The installer turns lingering on and fails loudly when it cannot. `live_service_started_before_login` proves the start at boot from the clocks of the system. |
| The service restarts without end after the owner removed the machine. | Medium | A removal ends `run` with status 0, and the service leaves an exit 0 alone. `live_removed_machine_stays_stopped`. |
| The installed client dials an address that disagrees with its token. | Medium | The address and `LGTM_PUBLIC_URL` both come from the published port. `asks_server_for_hosts_server_token` and `live_install_leaves_machine_waiting`. |
| The service does not find a harness that the user's shell finds. | Medium | The installer copies its own search path into the service. `unsafe_path_entries_dropped` checks the copy. A harness installed later in a new folder needs a second run, which the guide says. |
| The token is visible to another user of the computer while the join runs. | Low | The join takes seconds, and the token works once. |
| A Mac starts the client only at login. | Low | Decision 13 records it, and the guide says it. |

**Verify:** `bash deploy/test/installer.sh` on any Linux machine, with no Docker. `LGTM_INSTALL_LIVE=1 deploy/test/installer-live.sh`, its `--after-reboot` part and its `--cleanup` part on a Linux machine with Docker, the stack and a systemd user manager. A person restarts that machine. A person runs the macOS steps of item 3 on a Mac. `deploy/test/checks.sh` on the same Linux machine.

**Rollout detail:** steps 1, 3 and 4 of the rollout order. The live test runs on a disposable Linux machine or user, because it installs a real service.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe: the brief's work items 1 to 3, its three observations, its two agreements and its two points to settle; the specific points of the task files `plan-go-s3-stack-client.md` and `reconcile-go-s3-stack-client.md`; the requests to this plan in `requests-from-contract-r3.md`, `requests-from-contract-r4.md`, `requests-from-client-r4.md` and the fold of round 2 of `go-s2-stack-web`; issues 1636 to 1642 in `docs/issues.md`; the names table of `deploy/test/README.md`; and the design statements D7, D16, D17, D19, R3, R9, R10, C1 to C13, and the design's sentences on the server's own command, the service of the user and `deploy/install-client.sh`.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the server's command that makes a token | WS-1 | Item 1. |
| Brief item 2, the installer and its service | WS-2 | Items 2 and 3. |
| Brief item 3, C5 to C11 turned on | WS-2 | Plan `go-s2-stack-web`, item 7, proves the link through the web server. Item 2 changes the script's state folder. Item 4 runs the whole script. Decisions 20 and 31. |
| Brief observation, the installer ends with the machine waiting | WS-2 | `live_install_leaves_machine_waiting`. |
| Brief observation, the client starts by itself after a restart | WS-2 | `live_service_started_before_login`. |
| Brief observation, the user-flow script passes 13 checks | WS-2 | Criterion 2j. |
| Brief agreement, the Server team's lead merges item 1 | WS-1 | Item 1, scope `server/backend`. Decision 8. |
| Brief agreement, the name of the service and where the program goes | WS-2 | Public names table. Decisions 12 to 14. |
| Brief point 1, a service on macOS | WS-2 | Item 3. Decision 13. |
| Brief point 2, build or download | WS-2 | Item 2, step 10. Decision 11. |
| Task file, the script exists and is not rewritten | WS-2 | Only lines 219 and 233 change. Decision 35. |
| Task file, issue 1638, where `hosts_server` comes from | WS-1 | `--hosts-server`. Decision 10. |
| Task file, the service comes back after a restart | WS-2 | Item 2, steps 14 and 15, and item 4. Decision 12. |
| Contract round 3, the address the installer fills in | WS-2 | The section "The service, the address and the exit statuses". Decision 33. |
| Contract round 3, a failing join is a failure, and the service restarts `run` by its statuses | WS-2 | The same section. Decision 34. |
| Contract round 3, its post-build checks of exit statuses and of the installer | WS-2 | `live_reconnect_restarts_service`, `live_removed_machine_stays_stopped`, `live_install_leaves_machine_waiting` and the Mac step. Decision 34. |
| Contract round 4, refusals in the error's metadata | WS-1 | `TestJoinTokenJoinsOnce`. |
| Client round 4, ruling 5: `identity.json`, one state setting, and the script's lines | WS-2 | Item 2, steps 6 and 12, and the change of the script. Decisions 32 and 35. |
| Stack web round 2, request P1: the port set only in `deploy/.env` | WS-2 | Item 2, step 8. Decision 33. |
| Issues 1636 and 1637 | none | They concern the website, the contract and the Server team. |
| Issue 1639, replacing the server's keys | none | Outside wave 1, by plan `go-s3-server-machines`. `--rejoin` serves a removed machine. Decision 36. |
| Issue 1640, a sandbox without Docker | WS-2 | Item 4 needs Docker. Items 1, 2 and 3 do not. |
| Issues 1641 and 1642, a restore and two live copies | none | Outside wave 1. The plan adds no rule for them. |
| D7, a token that authorizes an exchange of keys | WS-1, WS-2 | Items 1 and 2. |
| D16, a restart of a connected client and a retry every 15 seconds | WS-2 | `live_reconnect_restarts_service`. |
| D17, trust without a public authority | WS-1 | The token carries the fingerprints that the machines part fills in. |
| D19, one user on a machine | WS-2 | `refuses_root`. Decision 27. |
| R3, one part calls another part's interface | WS-1 | Item 1 calls `Tokens.Create`. |
| R9, secrets never reach a log | WS-1, WS-2 | `TestJoinTokenPrintsOneTokenLine` and `token_never_printed`. |
| R10, real PostgreSQL | WS-1 | Item 1's tests use `LGTM_TEST_POSTGRES`. |
| C1 to C4, C7 to C9, C11 to C13 | WS-2 | Criterion 2j runs them. Other plans make them pass. |
| C5, C6, C10 | WS-1, WS-2 | Item 1 and criterion 2e. |
| The design's "or with the server's own command" | WS-1 | Item 1. |
| The design's "a service of its user, outside the compose stack" | WS-2 | Items 2 and 3. |
| The design's "`deploy/install-client.sh` ... joins it with a token the server's own command makes" | WS-2 | Item 2, step 13. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "What this project is": the three parts meet only through the contract. | The installer and the command use the contract's `Join`, `GetServer` and `WatchClients`. The installer reads no file of the client except whether `identity.json` exists. |
| `AGENTS.md`, "Where things are": new code does not go into `packages/`. | Every file is in `deploy/` or `src/server/cmd/lgtm-server/`. |
| The epic, "The teams, and what each may edit": the Stack team edits `deploy/`. | Items 2 to 4 edit only `deploy/`. Item 1 is the one command in the Server team's folder that the brief allows, and decision 8 records it. |
| The epic, a module is added with its first import. | The plan adds no module. Item 1 imports only packages of the repository and its existing modules. |
| `AGENTS.md`, habit 1: credit copied code. | Nothing is copied. The live test reuses the technique of `deploy/test/checks.sh` in the same repository. |
| `AGENTS.md`, habit 2: write down what you find. | The result file lists the problems for `docs/issues.md`, because this planner may write only its own folder and its result file. |
| `AGENTS.md`, "Writing style for documents". | The plan has full sentences and no private detail. The guide of item 4 follows the same style. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet at launch. This section names none.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on | Can run alongside | Design statements |
|---|---|---|---|---|---|---|---|---|
| 1. The server's token command | `backend` | `server/backend` | medium | One small command over other items' interfaces, but it makes a join token. | no | `go-s2-server-signin` items 1, 8 and 9; `go-s3-server-machines` items 1, 3, 4 and 14 | 2 | D7, D17, R3, R8, R9, R10, C5, C6 |
| 2. The installer on Linux | `infra` | `stack/infra` | medium | One script of known steps with several failure paths, and it passes a join token. | no | none to build; names from item 1 and `go-s3-client-link` | 1 | D7, D16, D19, R9, C6 |
| 3. The installer on macOS | `infra` | `stack/infra` | medium | A second branch of a known script, verified on Linux and by a person on a Mac. | no | 2 | 1 | D16, D19, R9 |
| 4. The live test and the guide | `infra` | `stack/infra` | medium | It drives the real stack and service manager, and one part needs a restart. | no | 1, 2; `go-s3-client-link` item 9; `go-s3-server-machines` items 12 to 14 | 3 | D16, D19, C5, C6, C7, C9 |

Load: four test writers, four builders, and two mutation runs for the risky items 1 and 2, which gives 10 execution seats. Alternating the two primary vendors gives five seats to each. Item 4 needs a seat with access to Docker and to the loopback network, by issue 1640. Items 1, 2 and 3 do not need Docker, and item 1 needs only the disposable PostgreSQL of `deploy/test/with-postgres.sh`. A review round of this plan uses the seats that the epic names. The orchestrator checks the weekly limits at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| Round 1 | Not run yet. | The orchestrator runs the review rounds. The planner stopped after the self-check. |

</details>
