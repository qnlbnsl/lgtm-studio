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

Sprint 3 makes a machine join the server. This plan gives the owner one command that puts the client on the same computer as the stack. The command builds the client, joins it to the server, and makes it a service of the owner's user. After the command, the machine waits for approval on the Machines page. On Linux, after a restart of the computer, the client starts again without anyone logging in, when the user lingers and Docker starts at boot (critic-r1 07).

The join needs a token. The design lets the owner make a token on the Machines page or with the server's own command. This plan writes that command, `lgtm-server join-token`, because the installer cannot click a page. A token from this command can also carry one extra mark: the machine that uses it runs the server. That mark is how the Machines page learns which machine is the server's own, which no message of the link reports. Whoever can run a command inside the server's container can make such a token, so the mark records how the token was made and grants nothing (architect-r1 02).

The user-flow script `deploy/test/checks.sh` already exists, and it must pass all 13 of its checks at the end of the sprint. Plan `go-s2-stack-web` already proves that its web server carries the link's two-way stream on the loopback address. This plan adds only a small change to the script: its clients get their state folder through the client's own setting.

The contract of the link now states only what the other end can observe. The exit statuses of the client, what the service manager does with them, and which address the installer fills in are rules of the programs. This plan states the rules of the service and the installer once, in its section "The service, the address and the exit statuses", each with the test that proves it.

On macOS, the client starts when the owner logs in, not at boot. Starting at boot would need an administrator to install a system service. The design names no tool for macOS, so the plan chose the one that stays within the owner's user. The unattended return after a restart is therefore proved on Linux only (critic-r1 07).

## Key decisions

- **The installer builds the client from the checkout.** No built client is published anywhere yet, and nothing would sign one. The checkout is already on the machine, because the stack starts from it.
- **On Linux the client is a systemd user service, and the user lingers.** Lingering is the setting that lets a user's services start at boot without a login. When the installer cannot turn it on, it says so and exits with a failure, but the client is still installed.
- **Nothing that works is changed before the join succeeds.** The installer joins with the newly built program from a temporary file. Only after the join does it install the program and the service. A failed `--rejoin` puts the old state back and starts the old service again.
- **The token marks the server's own machine.** The machine never claims the mark itself, so a remote machine cannot pose as the server's machine.
- **The service starts the client again after every exit except 0.** Plan `go-s3-client-link` makes `run` exit 0 for a stop for good, 75 for a restart that the server asked for, and 1 for a failure.
- **The installed client dials the stack's own loopback port.** The installer asks Compose for the published port, so a port set only in `deploy/.env` is still found.
- **A second run is safe.** It installs the newly built program and restarts the service, and it keeps the machine's identity. It refuses an identity of another address and a join that did not finish, and names `--rejoin` for both.

Full log → `decisions.md`

## Workstreams

### The server makes a join token from its own command

The owner can make a token without the website, with one command inside the server's container. The command prints the token and nothing else, so a script can capture it. The token names the server's own public address, which is the only address the machines part puts in a token.

The risk is that the token's secret leaks into a log. The command writes nothing on success except the token on standard output, and the tests check that standard error stays empty.

When it is done, a person runs the command against the stack and gets one line that starts with `lgtm1_`. A machine that joins with it shows on the Machines page as the machine that runs the server.

### The client installs as a service that comes back by itself

`deploy/install-client.sh` takes the computer from a running stack to a joined client that runs as a service. It finds the stack's port, checks that the stack answers, builds the client, asks the server for a token, joins, and then installs and starts the service. A second run updates the program and keeps the identity. Other options join again or remove the service.

The main risk is a service that looks installed but does not start after a restart. A live test on a Linux machine kills the client and watches it come back. A person restarts the computer, and the test then proves from a new boot identifier and the clocks of the system that the client started before the first login. A second risk is a failed run that leaves a working machine stopped, which the installer prevents by changing nothing before the join succeeds. A third risk is the token in the installer's output, which the tests rule out the same way as for the command.

When it is done, the installer ends with the machine waiting for approval. After approval the machine shows as online. After a restart of the computer it shows as online again. After a removal on the Machines page the client stops at its next connection.

## Rollout order

1. Build the installer on Linux, with its tests that need no stack. Its tests replace every outside program with a stand-in, so it can start before any link code exists. Its names from plans `go-s3-server-machines` and `go-s3-client-link` freeze when both of those plans have cleared review, by the epic's rule of 2026-11-26.
2. Build the server's token command when the Server team's token and join code are merged. The Server team's test writer and builder do it, and its lead merges it (critic-r1 09).
3. Build the macOS part of the installer after the Linux part is merged, because both change the same script.
4. Run the sprint gate below once the client, the server and the stack of sprint 3 are merged and the server image is rebuilt. A person restarts the computer between the parts of the live test.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**How the statements of the design are cited**

The design is `docs/specs/2026-11-25-go-rewrite-foundation-design.md` on `master`.
D1 to D23 are the rows of its decisions table, in order. D7 is the key pair and the join token. D16 is the restart of a connected client and the retry every 15 seconds. D17 is trust without a public authority. D19 is one user on a machine.
R1 to R11 are its rules. R3 is "a part calls another part's Go interface". R8 is "an error is one sentence". R9 is "secrets never reach a log". R10 is "tests run against a real PostgreSQL".
C1 to C13 are its checks of wave 1.
The design's sections are cited by their headings, such as "Joining: a token that authorizes an exchange of keys".
`scouting.md` lists each statement with its line.
A workstream (WS) groups work items. WS-1 and WS-2 are the two workstreams above.

**Public names that other work items and teams build against**

| Name | Kind | Meaning | Set by |
|---|---|---|---|
| `lgtm-server join-token [--hosts-server]` | command | Makes one join token for the server's public address. Prints its text, one line that starts with `lgtm1_`, on standard output. Exit 0 on success, 1 when the settings, the database, the vault, the token or the write of the line fail, and 2 for a wrong argument. Every message is one fixed sentence on standard error, which repeats no argument (threat-r1 2). | item 1 |
| `joinTokenCommand(ctx context.Context, args []string, lookup func(string) (string, bool), stdout, stderr io.Writer) int` | Go function in package `main` of `src/server/cmd/lgtm-server` | The body of the command, in the dispatcher's table of plan `go-s2-server-signin`, item 9. `args` excludes the program's name and the word `join-token`. The Server team writes it, the dispatcher calls it. | item 1 |
| `deploy/install-client.sh [--rejoin \| --remove]` | command | Installs, joins again, or removes the client on this computer. Exit 0 on success, 1 on a failure, 2 for a wrong argument. | items 2 and 3 |
| `COMPOSE_FILE`, `COMPOSE_PROJECT_NAME`, `COMPOSE_ENV_FILES`, `LGTM_STACK_STATE_DIR` | variables passed through | The installer sets `COMPOSE_FILE` to `deploy/compose.yaml` when it is unset or empty, calls `docker compose` without `-f`, and passes the others on, as `deploy/up.sh` of plan `go-s2-stack-web` does. | item 2 |
| `$HOME/.local/bin/lgtm-client` | file | The installed program, mode `0755`. | item 2 |
| `lgtm-client.service` at `${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/` | file | The user unit on Linux. Its line `Environment=LGTM_CLIENT_STATE_DIR=` names the installation's state folder. | item 2 |
| `deploy/lgtm-client.service` | template | The unit with the placeholders `@LGTM_CLIENT_BIN@`, `@LGTM_CLIENT_STATE_DIR@` and `@LGTM_CLIENT_PATH@`. | item 2 |
| `dev.lgtmstudio.client` | launchd label | The agent on macOS, at `$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist`. | item 3 |
| `deploy/dev.lgtmstudio.client.plist` | template | The agent with the placeholders of the unit and `@LGTM_CLIENT_LOG@`. | item 3 |
| `$HOME/Library/Logs/lgtm-client.log` | file | The output of the client on macOS. On Linux the output goes to the journal of the user. | item 3 |
| `lgtm-client.replaced-<YYYYMMDDTHHMMSSZ>.<6 random characters>/lgtm-client` | folder beside the state folder | The old state folder after `--rejoin`. The installer reserves the outer folder with `mktemp -d`, so two archives never share a name (logic-r1 05). | item 2 |
| `lgtm-client.install-lock` | folder beside the state folder | Held by one run of the installer at a time. | item 2 |

The names this plan reads from other plans:

| Name | Plan |
|---|---|
| `config.Load(lookup)` and its fields `PublicURL`, `Postgres`, `AllowPrivateStack`, `OpenBaoAddr` and `OpenBaoCredentialFile`. | `go-s2-server-signin` item 1. |
| `store.Open(ctx, config, store.ConnectionPolicy{AllowPrivateStackPostgres})` and `Store.Close()`; `vault.Open(addr, credentialFile, allowPrivateStack)`, `vault.LoadLinkIdentity(ctx, store, client, false)` and `LinkIdentity.SigningKey`. | `go-s1-server-store` and `src/server/internal/store/pool.go`; `go-s2-server-signin` items 8 and 9. |
| `machines.NewRegistry()`, `machines.NewTokens(store, registry, machines.Issuer{PublicURL, ServerKey})`, `Tokens.Create(ctx, machines.TokenOptions{HostsServer})`, `NewToken.Text` and `machines.DecodeToken`. | `go-s3-server-machines` items 1 and 3. |
| `servertest.Start`, `Server.URL`, `Server.SigningPublicKey`, `Server.Environment()`, `Server.Clients(t)` and `Server.Link()`. | `go-s2-server-signin` item 9; `go-s3-server-machines` item 14. |
| `lgtm-client join <address> <token>`, `lgtm-client run`, `lgtm-client version`, `-X main.version=<v>`, and their exit statuses. | `go-s1-client-local` item 1; `go-s3-client-link` item 9. |
| `LGTM_CLIENT_STATE_DIR`, the default `$HOME/.local/state/lgtm-client`, the files `lock`, `settings.json`, `machine.key`, `identity.json` and `client.db`, the folder `repositories`, temporary names that start with `.tmp-`, and the field `address` of `identity.json`. | `go-s1-client-local`, "The layout of the state folder", and `src/client/internal/state/folder.go:15`; `go-s3-client-link` item 2. |
| The Compose project `lgtm`, the service `server` as user 10001 with `/usr/local/bin/lgtm-server`, the service `web` and its port 8080, and `LGTM_PUBLIC_URL` equal to `http://127.0.0.1:${LGTM_WEB_PORT:-8787}`. | `go-s1-stack-vault`; `go-s2-stack-web` items 1 and 4. |

**The service, the address and the exit statuses**

These rules belong to this plan, because the contract states only facts of the wire. Each is stated here once.

| Rule | Proved by |
|---|---|
| The installer fills in the address `http://` followed by the output of `docker compose port web 8080`, which must be `127.0.0.1:<port>`. The stack's `LGTM_PUBLIC_URL` comes from the same port, so the address agrees with the address inside the token. An existing identity of another address is refused. | `asks_server_for_hosts_server_token`, `rerun_refuses_changed_address` and `live_install_leaves_machine_waiting`. |
| A failing `lgtm-client join` is a failure of the installer. It exits 1, installs neither the program nor the service, and leaves an earlier installation as it was. | `join_failure_installs_no_service` and `rejoin_failure_restores_old_state`. |
| The service starts the client again about 5 seconds after every exit except 0, without a limit on the count. On Linux this is `Restart=on-failure`, `RestartSec=5` and `StartLimitIntervalSec=0`. On macOS it is `KeepAlive` with `SuccessfulExit` false and `ThrottleInterval` 5, where launchd may wait longer (architect-r1 08). | `installs_linux_user_service`, `macos_agent_parses`, `live_service_restarts_after_crash`. |
| A restart that the server asked for ends `run` with status 75, and the service starts the client once more. | `live_reconnect_restarts_service`. On a Mac, the person step of item 3. |
| A removal ends `run` with status 0 at the client's next connection, and the service does not start it again until the next boot or login. A run of the installer after that reports the stopped client and names `--rejoin` (logic-r1 07, logic2-r1 02). | `live_removed_machine_stays_stopped` and `rerun_reports_stopped_client`. |
| The service and the installer's `join` use the same state folder, passed as `LGTM_CLIENT_STATE_DIR`. A later run without the variable takes the folder from the installed unit. | `joins_before_service_starts` and `rerun_uses_state_folder_of_installed_unit`. |

Plan `go-s3-client-link` proves the statuses of the program itself, in `TestRunCommandExitStatus`. It also proves that a restart's result is sent from its receipt after the new start, in `TestRestartReplaysAfterDroppedResult`, and that the client's own log holds no secret, in `TestAppLogRedactsSecrets` (logic2-r1 09).

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
| 1c | A wrong argument, a missing database and a failed write are refused, each with one fixed sentence. | `TestJoinTokenRefusesUnknownArgument`, `TestJoinTokenFailsWithoutDatabase` and `TestJoinTokenFailsOnWriteError` pass. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1.1 | Write the command and its argument rules. | `src/server/cmd/lgtm-server/join_token.go`, `joinTokenCommand` (target). Pattern: `src/server/cmd/lgtm-server/main.go`, `run`, of plan `go-s2-server-signin`, item 9. |
| 1.2 | Write the tests against a real server and database. | `src/server/cmd/lgtm-server/join_token_test.go` (target). Pattern: `src/server/servertest/server_test.go`, `TestStartAndRestartServer`, of plan `go-s2-server-signin`, item 9. |

**Work item 1. The server's own command that makes a join token**

- **Files:** creates `src/server/cmd/lgtm-server/join_token.go` and `src/server/cmd/lgtm-server/join_token_test.go`. Changes the entry `join-token` of the dispatcher's table in `src/server/cmd/lgtm-server/main.go`, as plan `go-s2-server-signin`, item 9, provides. A Server test writer creates the test file, and a Server builder writes the rest (critic-r1 09).
- **Needs first:** plan `go-s2-server-signin`, items 1, 8 and 9. Plan `go-s3-server-machines`, items 1, 3, 4 and 14. It runs at the same time as item 2.
- **Work area and complexity:** `backend`, medium. It is one small command over interfaces that other items build, but it handles a join token.
- **Scope:** `server/backend`. All three files sit in the Server team's folder, and the Server team's lead merges them. See decision 8.
- **Design statements:** D7, D17, R3, R8, R9, R10, C5, C6. The section "Joining: a token that authorizes an exchange of keys", step 1, and the section "The compose stack", the bullet on `deploy/install-client.sh`.
- **Risky:** yes, because it makes a join token. Two mutations must turn the tests red. Passing `false` for the mark in every case must fail `TestJoinTokenHostsServerFlag`. Writing the token's text to standard error must fail `TestJoinTokenPrintsOneTokenLine`.
- **Behaviour:**
  - The dispatcher's entry `join-token` calls `joinTokenCommand` with the dispatcher's own `lookup`.
  - The only argument is `--hosts-server`, which takes no value. Any other argument, a value, or a repeated flag gives exit 2, nothing on standard output, and the fixed sentence `usage: lgtm-server join-token [--hosts-server]` on standard error, which repeats no argument (threat-r1 2).
  - The command reads the settings with `config.Load(lookup)`. It opens the store with `store.Open(ctx, cfg.Postgres, store.ConnectionPolicy{AllowPrivateStackPostgres: cfg.AllowPrivateStack})`, opens the vault client with `vault.Open`, and reads the server's identity with `vault.LoadLinkIdentity` and `allowInitialize` false. It takes no instance lock, runs no migration and opens no port. It closes the store before it returns. The vault client has no close method, and the command calls none (architect-r1 01, critic-r1 01, logic-r1 01, threat-r1 1).
  - It calls `Tokens.Create` with `TokenOptions{HostsServer: <flag>}` and an empty address, so the token names `PublicURL`. The issuer is `machines.Issuer{PublicURL: cfg.PublicURL, ServerKey: <public half of LinkIdentity.SigningKey>}`. Wave 1 has no certificate, so the token's pin is empty, and an `https` public address gives `ErrNoCertificatePin` (architect-r1 01).
  - On success it writes `NewToken.Text` and one newline to standard output, and nothing else. It writes nothing to standard error. A failed write to standard output gives exit 1 (logic-r1 06).
  - A failure of the settings, the store, the vault or the token gives exit 1 and one sentence on standard error. The sentence names the part that failed and holds no setting's value.
- **Tests written first:** in `src/server/cmd/lgtm-server/join_token_test.go`, package `main`. Each test starts a server through `servertest.Start` and calls `joinTokenCommand` with a lookup over `Server.Environment()`. The Server team's test seat runs them with the disposable PostgreSQL of `deploy/test/with-postgres.sh` (architect-r1 08). A stub of `joinTokenCommand` that returns 0 and writes nothing makes every positive test fail on its first assertion.

| Test | What it asserts |
|---|---|
| `TestJoinTokenPrintsOneTokenLine` | With no argument, the exit code is 0. Standard output matches `^lgtm1_[A-Za-z0-9_-]+\n$` exactly. Standard error is empty. |
| `TestJoinTokenCarriesAddressAndServerKey` | `machines.DecodeToken` of the line gives a token whose `address` equals `Server.URL`, whose `token_id` is a version 7 universally unique identifier (UUID), whose `secret` has 32 bytes, whose `server_key_sha256` equals the SHA-256 hash (Secure Hash Algorithm, 256 bits) of `Server.SigningPublicKey`, and whose `certificate_key_sha256` is empty. |
| `TestJoinTokenJoinsOnce` | `Join` through `Server.Link()` with the token's `token_id`, its secret and a fresh Ed25519 public key succeeds. A second `Join` with the same token and another key returns `permission_denied` with `Lgtm-Join-Refusal` equal to `token_used` in the error's metadata. |
| `TestJoinTokenHostsServerFlag` | One token is made with `--hosts-server` and one without. A machine joins with each. The first message of `WatchClients` through `Server.Clients(t)` lists both as waiting. The machine of the first token has `hosts_server` true, and the other has false. |
| `TestJoinTokenRefusesUnknownArgument` | `--address x`, `--hosts-server=yes`, `--hosts-server` given twice, and the bare word `lgtm1_marker` each give exit 2, an empty standard output, and exactly the usage sentence on standard error, which holds neither `x`, `yes` nor `lgtm1_marker`. |
| `TestJoinTokenFailsWithoutDatabase` | With the database port of the environment changed to a closed loopback port, the exit code is 1, standard output is empty, and standard error is one line that ends with a full stop and holds no value of the environment. |
| `TestJoinTokenFailsOnWriteError` | With a standard output whose writes fail, the exit code is 1 and standard error is one line. |

- **Acceptance:** `LGTM_TEST_POSTGRES=<url> go test -count=1 -run 'TestJoinToken' ./src/server/cmd/lgtm-server/` passes all seven tests. The run inside the stack belongs to the sprint gate.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The token's secret reaches a log of the command. | High | The command logs nothing, and its usage sentence repeats no argument. `TestJoinTokenPrintsOneTokenLine` and `TestJoinTokenRefusesUnknownArgument`. |
| A token marks a remote machine as the server's machine. | Medium | Only the command sets the mark, and it runs inside the server's container. `CreateClientToken` never sets it, by plan `go-s3-server-machines`, item 12. The mark is shown on the page and grants nothing. |
| The command and the running server disagree on the signing key. | Medium | The command reads the pinned identity through the server's own function, and `TestJoinTokenCarriesAddressAndServerKey` compares the token with the server's key. |

**Verify:** the seven tests above against a real PostgreSQL. No device is needed.

**Rollout detail:** step 2 of the rollout order. The Server team's lead merges the item after a passing report on its exact commit.

**WS-2: The client installs as a service that comes back by itself**

The design says that the client "runs as a service of its user, outside the compose stack", and that `deploy/install-client.sh` "installs the client on the same machine and joins it".
This workstream writes that script, the service files it installs, and the tests that prove both on Linux and macOS.
The hermetic tests replace every outside program with a stand-in on `PATH`, so they need no Docker, no network and no service manager.
The live test runs the real installer against the real stack and the real service manager of a Linux user.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2a | On Linux, the installer finds the port, builds the program, joins with a token of the server's command, and only then installs the program and the user unit. | `installs_linux_user_service`, `asks_server_for_hosts_server_token` and `joins_before_service_starts` in `bash deploy/test/installer.sh`. |
| 2b | The token never appears in the installer's output or in any file under the test's folder. | `token_never_printed`. |
| 2c | A failed join or a refused precondition leaves an earlier installation as it was and installs nothing new. | `join_failure_installs_no_service`, `rejoin_failure_restores_old_state`, `refuses_root`, `refuses_missing_tool`, `refuses_docker_failure`, `refuses_stack_not_answering`, `refuses_malformed_token`, `refuses_missing_user_manager`, `refuses_unsafe_home`, `refuses_unfinished_join` and `build_failure_stops_installer`. |
| 2d | A second run keeps the identity, `--rejoin` keeps the old folder under a new name, and `--remove` removes the service and keeps the state. | `rerun_keeps_joined_identity`, `rerun_uses_state_folder_of_installed_unit`, `rerun_refuses_changed_address`, `rerun_reports_stopped_client`, `rejoin_moves_old_state`, `rejoin_refuses_foreign_folder` and `remove_stops_and_deletes_service`. |
| 2e | The user-flow script gives its clients their state folder through `LGTM_CLIENT_STATE_DIR`. | `! grep -q XDG_STATE_HOME deploy/test/checks.sh` exits 0. |
| 2f | On macOS, the installer loads a launchd agent that parses as a property list. | `installs_macos_launch_agent`, `macos_agent_parses` and `macos_remove_boots_out_agent`. |

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
- **Needs first:** nothing, for its own tests. It runs at the same time as item 1. Its real runs belong to the sprint gate.
- **Work area and complexity:** `infra`, medium. It is one script of known steps with several failure paths, and it passes a join token.
- **Scope:** `stack/infra`.
- **Design statements:** D7, D16, D19, R8, R9, C6. The section "The client in wave 1", on the state folder and the service of the user. The section "The compose stack", the bullet on `deploy/install-client.sh`.
- **Risky:** yes, because it passes a join token and moves a state folder. Three mutations must turn the tests red. Printing the token with `printf '%s\n' "$token"` must fail `token_never_printed`. Moving the service step in front of the join must fail `joins_before_service_starts`. Dropping the restore of step 13 must fail `rejoin_failure_restores_old_state`.
- **Behaviour, in order.** Each refusal before step 13 prints one sentence, exits 1 and changes no file except the lock.
  1. The script runs with shell tracing off (`set +x`) from its first line, and changes to the repository root, as `deploy/up.sh` does (threat-r1 2). It accepts no argument, `--rejoin` or `--remove`. Anything else prints the usage and exits 2.
  2. It refuses to run when `id -u` is 0, with `install-client.sh must run as the user who runs the harnesses, not as root.`
  3. It reads `uname -s`. `Linux` takes the steps below. `Darwin` takes the steps of item 3. Anything else gives `install-client.sh supports Linux and macOS only.` It then requires `go`, `docker`, `curl` and `jq` on `PATH`, and a missing one gives `install-client.sh needs <tool> on PATH.` (logic2-r1 07).
  4. On Linux it runs `systemctl --user show-environment`. A failure gives `This Linux system has no systemd user manager for this user.`
  5. It refuses a `HOME` that is empty, not absolute, or holds a character outside letters, digits and `. _ / + @ -`, with `The home folder path holds a character that a service file cannot carry.`
  6. It works out two values. The state folder is `LGTM_CLIENT_STATE_DIR` when that is set and not empty. Otherwise it is the folder named by the line `Environment=LGTM_CLIENT_STATE_DIR=` of the installed unit, when the unit exists, and otherwise `$HOME/.local/state/lgtm-client` (threat-r1 5, logic2-r1 05). A value that is not an absolute path, or that holds a character refused in step 5, gives `LGTM_CLIENT_STATE_DIR must be an absolute path.` The service's search path is the installer's `PATH` without the entries that hold a character outside letters, digits and `. _ / + @ : -`.
  7. It creates the parent of the state folder with mode `0700` when it is missing, and makes the folder `lgtm-client.install-lock` there with `mkdir`. When that folder exists, it prints `Another install-client.sh is running; remove <lock> if none is.` A trap removes the lock and every temporary file of this run on any exit. **UNREVIEWED MECHANISM** (logic2-r1 06, logic-r1 05): a `mkdir` lock works on Linux and macOS, while `flock` is missing on macOS.
  8. `--remove` stops here. It runs `systemctl --user disable --now lgtm-client.service`, ignoring a unit that is not loaded. It deletes the unit file and the program, runs `systemctl --user daemon-reload`, prints one sentence, and exits 0. It keeps the state folder.
  9. It sets `COMPOSE_FILE` and runs `docker compose port web 8080`. A failed command gives `docker compose could not read the stack; check that this user may run docker.` Output other than one line `127.0.0.1:<port>` gives `The stack's web service publishes no loopback port. Start it with deploy/up.sh.` (logic2-r1 07). The address is `http://127.0.0.1:<port>`.
  10. It calls `SessionService/GetServer` at the address with `curl --silent --noproxy '*' --max-time 5`, as `get_server` in `deploy/test/checks.sh` does. A status other than 200, or a body without `product` equal to `lgtm-studio`, a positive `contract` and `sample` other than true, gives `The stack does not answer at <address>. Start it with deploy/up.sh.` (critic-r1 10).
  11. It checks the state folder (architect-r1 03, architect-r1 05, logic-r1 02, logic-r1 04, logic-r1 08):
      - Without `--rejoin`, a folder with `identity.json` must hold the pair `"address"` and `"<address>"`, with only a colon and spaces between them. Otherwise it gives `lgtm-client in <folder> joined another address; run deploy/install-client.sh --rejoin.`
      - Without `--rejoin`, a folder with `machine.key` and no `identity.json` gives `A join into <folder> did not finish. Remove its waiting machine on the Machines page if one is shown, then run deploy/install-client.sh --rejoin.`
      - With `--rejoin`, an existing folder must not be a symbolic link, and each of its entries must be `lock`, `settings.json`, `machine.key`, `identity.json`, `client.db`, `repositories`, or a name that starts with `.tmp-`. Otherwise it gives `<folder> holds files that are not lgtm-client's, so install-client.sh does not move it.` **UNREVIEWED MECHANISM** (critic-r1 04, logic-r1 02, threat-r1 3): a folder that holds only the client's own names cannot be the home folder or a folder of repositories.
  12. It builds the client with `CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=<version>" -o <temporary file> ./src/client/cmd/lgtm-client`. The temporary file is made with `mktemp` in `$HOME/.local/bin/`. The version is `0.0.0-` and the first 12 hexadecimal digits of `git rev-parse HEAD`, with `-dirty` when `git status --porcelain` prints anything. Without Git it is `0.0.0-dev`. It runs `<temporary file> version`. A failure of either gives `go build of lgtm-client failed.`
  13. It joins, unless the folder has `identity.json` and `--rejoin` was not given, in which case it prints `lgtm-client is already joined.` (logic2-r1 01, critic-r1 03, architect-r1 04, logic-r1 03, threat-r1 4):
      1. It runs `docker compose exec -T server lgtm-server join-token --hosts-server`, and keeps standard output in a variable. A failed command, or output that does not match `^lgtm1_[A-Za-z0-9_-]+$`, gives `The server's join-token command returned no token.`
      2. With `--rejoin` and an existing folder, it runs `systemctl --user stop lgtm-client.service`, ignoring a unit that is not loaded. It reserves the archive with `mktemp -d <parent>/lgtm-client.replaced-<time>.XXXXXX` and moves the state folder into it as `lgtm-client`.
      3. It runs `LGTM_CLIENT_STATE_DIR=<state folder> <temporary file> join <address> <token>`, and then clears the variable that held the token.
      4. A failed join passes on the client's output and prints `lgtm-client could not join the server.` With `--rejoin`, it then removes the folder that the failed join made, moves the archived folder back, removes the empty archive, and starts the service again when its unit exists. The run exits 1, with the program and the unit as they were before it.
  14. It renames the temporary file to `$HOME/.local/bin/lgtm-client`, mode `0755`, and prints `built lgtm-client <version>`.
  15. It renders `deploy/lgtm-client.service` with the three placeholders into a temporary file beside the unit's path, and renames it to the unit's path. It runs `systemctl --user daemon-reload`, `systemctl --user enable lgtm-client.service` and `systemctl --user restart lgtm-client.service`. A failure of any gives `systemctl --user <verb> lgtm-client.service failed.` and exit 1.
  16. When `loginctl show-user <user> --property=Linger --value` does not print `yes`, it runs `loginctl enable-linger <user>`. A failure prints `loginctl enable-linger <user> failed, so the client starts only after this user logs in.` and sets the exit status to 1 for the end of the run.
  17. It waits up to 10 seconds for `systemctl --user is-active lgtm-client.service` to print `active`, and then 5 seconds more. When the unit is not active at the end, or its `ExecMainPID` changed in those 5 seconds, it prints `lgtm-client stopped after it started; read journalctl --user -u lgtm-client.service, and run deploy/install-client.sh --rejoin if the machine was removed.` and exits 1 (logic2-r1 02).
  18. After a `--rejoin`, it prints `Remove the old machine on the Machines page.` (logic2-r1 05). It prints `lgtm-client runs as the user service lgtm-client.service.` and exits with 0, or with the status of step 16.
- **The unit template** `deploy/lgtm-client.service` holds these lines and no other setting:
  - `[Unit]`, `Description=LGTM Studio client`, `StartLimitIntervalSec=0`.
  - `[Service]`, `Type=simple`, `ExecStart=@LGTM_CLIENT_BIN@ run`, `Environment=LGTM_CLIENT_STATE_DIR=@LGTM_CLIENT_STATE_DIR@`, `Environment=PATH=@LGTM_CLIENT_PATH@`, `Restart=on-failure`, `RestartSec=5`, `TimeoutStopSec=10`.
  - `[Install]`, `WantedBy=default.target`.
- **The change of the user-flow script:** line 219 runs the client's `join` with `LGTM_CLIENT_STATE_DIR=$1/lgtm-client` in place of `XDG_STATE_HOME=$1`. Line 233 runs `run` with `LGTM_CLIENT_STATE_DIR=$state_home/lgtm-client` in place of `XDG_STATE_HOME=$state_home`. The README's row "Client state folder" says that the checks set `LGTM_CLIENT_STATE_DIR` to the `lgtm-client` child of a temporary folder. C10 keeps its path `$state_home/lgtm-client/identity.json`. The client no longer reads `XDG_STATE_HOME`, by plan `go-s1-client-local`. The other names that C5 to C11 read belong to the plans that write them, and the sprint gate names them.
- **Tests written first:** `deploy/test/installer.sh` prints one TAP line per test, `ok <n> - <test id>` or `not ok <n> - <test id> <reason>`, and exits 0 only when all pass.
  - **The stand-ins.** Each test makes a temporary folder with `home/` as `HOME`, `tmp/` as `TMPDIR`, `shims/` first on `PATH`, and `calls.log`. The test unsets `LGTM_CLIENT_STATE_DIR` unless a test sets it. The shims are `go`, `docker`, `systemctl`, `loginctl`, `launchctl`, `uname`, `id`, `curl` and `jq`, where `jq` passes through to the real program. Each appends one line with its name and arguments to `calls.log`, so the order of calls is one file. Variables of the test steer them: `SHIM_UNAME`, `SHIM_UID`, `SHIM_WEB_PORT`, `SHIM_DOCKER_FAIL`, `SHIM_CURL_BODY`, `SHIM_CURL_FAIL`, `SHIM_GO_FAIL`, `SHIM_TOKEN`, `SHIM_JOIN_EXIT`, `SHIM_LINGER_FAIL`, `SHIM_NO_USER_MANAGER` and `SHIM_UNIT_STOPS` (logic2-r1 08).
  - The `id` shim answers `-u` with `SHIM_UID` and passes other arguments to the real `id`. The `go` shim, called with `build ... -o <file>`, writes a stand-in client to `<file>`, or exits 1 when `SHIM_GO_FAIL` is set. That client answers `version` with `0.0.0-test`. It answers `join <address> <token>` by appending `client join <address> <SHA-256 of the token> <LGTM_CLIENT_STATE_DIR>` to `calls.log` and creating `identity.json`, holding `{"address": "<address>"}`, and `machine.key` in that folder. When `SHIM_JOIN_EXIT` is not 0, it creates `machine.key` only, prints `{"msg":"join_refused","code":"token_used"}` and exits with that status.
  - The `docker` shim answers `compose port web 8080` with `127.0.0.1:${SHIM_WEB_PORT-8787}`, so an unset variable gives 8787 and an empty one prints nothing (architect-r1 12, critic-r1 02, logic-r1 09, threat-r1 10). With `SHIM_DOCKER_FAIL` set, it exits 1 for every call. It answers `compose exec` with `SHIM_TOKEN`, whose default is a fixed `lgtm1_` text of 60 characters. The `curl` shim prints `SHIM_CURL_BODY`, whose default is `{"product": "lgtm-studio", "contract": 1}`, followed by status 200, unless `SHIM_CURL_FAIL` is set. The `systemctl` shim answers `is-active` with `active` and `show -p ExecMainPID` with a fixed number, and with `SHIM_UNIT_STOPS` set it answers `inactive` after its first `is-active`.
  - A stub `deploy/install-client.sh` that exits 0 and does nothing makes every positive test fail on its first assertion. Each test that asserts something is absent first asserts the line that proves its run reached that step (critic-r1 11).

| Test | What it asserts |
|---|---|
| `installs_linux_user_service` | With `SHIM_UNAME=Linux`, the exit code is 0. `$HOME/.local/bin/lgtm-client` exists with mode `0755`. `$HOME/.config/systemd/user/lgtm-client.service` equals `deploy/lgtm-client.service` with `@LGTM_CLIENT_BIN@` replaced by the program's path, `@LGTM_CLIENT_STATE_DIR@` by `$HOME/.local/state/lgtm-client`, and `@LGTM_CLIENT_PATH@` by the test's search path. `calls.log` holds, in this order, `systemctl --user daemon-reload`, `systemctl --user enable lgtm-client.service` and `systemctl --user restart lgtm-client.service`. It holds `loginctl enable-linger` with the user's name. The `go` line holds `-X main.version=0.0.0-` and `./src/client/cmd/lgtm-client`. The lock folder is gone. |
| `asks_server_for_hosts_server_token` | `calls.log` holds `docker compose port web 8080` before exactly one `docker compose exec -T server lgtm-server join-token --hosts-server`. The `client join` line has the address `http://127.0.0.1:8787`. A run in a fresh folder with `SHIM_WEB_PORT=8899` has the address `http://127.0.0.1:8899`. A run with `SHIM_WEB_PORT=` empty exits 1, prints `publishes no loopback port`, and holds no `go` line. |
| `joins_before_service_starts` | The `client join` line comes before the line that renames the program into place and before the first `systemctl --user` line that names the unit. Its hash equals the SHA-256 of `SHIM_TOKEN`. Its folder equals the value of `LGTM_CLIENT_STATE_DIR` in the unit file. With `LGTM_CLIENT_STATE_DIR` set to an absolute folder of the test, the join line and the unit both name that folder. With it set to `relative/dir`, the exit code is 1 and `calls.log` holds no `go` line. |
| `token_never_printed` | `calls.log` holds the `client join` line with the token's hash. The installer's standard output and standard error do not hold `SHIM_TOKEN`. No file under the test's folder holds it, which covers `HOME` and `TMPDIR` (logic2-r1 09). |
| `join_failure_installs_no_service` | With `SHIM_JOIN_EXIT=1`, the exit code is 1. The output holds `join_refused`. Neither the program nor the unit file exists. `calls.log` holds no `systemctl --user enable` line. A second run without `--rejoin` exits 1 and prints `did not finish`. |
| `rejoin_failure_restores_old_state` | After a first run, `--rejoin` with `SHIM_JOIN_EXIT=1` exits 1. The state folder holds the first `machine.key` and `identity.json` byte for byte. No archive folder remains. The unit file and the program are unchanged. `calls.log` holds `systemctl --user stop` before the `client join` line and a `systemctl --user start` or `restart` line after it. A second case with `SHIM_TOKEN=oops` exits 1 and holds no `systemctl --user stop` line. |
| `refuses_root` | With `SHIM_UID=0`, the exit code is 1, the output holds `not as root`, and `calls.log` holds no `go` line. |
| `refuses_missing_tool` | With a search path that has no `go`, the exit code is 1 and the output holds `needs go`. |
| `refuses_docker_failure` | With `SHIM_DOCKER_FAIL=1`, the exit code is 1, the output holds `may run docker`, and `calls.log` holds no `go` line. |
| `refuses_stack_not_answering` | With `SHIM_CURL_FAIL=1`, and again with `SHIM_CURL_BODY` set to `{"product": "lgtm-studio", "contract": 1, "sample": true}`, the exit code is 1, the output holds `deploy/up.sh`, and `calls.log` holds no `docker compose exec` line and no `go` line (critic-r1 10). |
| `refuses_malformed_token` | With `SHIM_TOKEN=oops`, the exit code is 1, `calls.log` holds no `client join` line, and neither the program nor the unit file exists. |
| `refuses_missing_user_manager` | With `SHIM_NO_USER_MANAGER=1`, the exit code is 1, and `calls.log` holds no `go` line. |
| `refuses_unsafe_home` | With `HOME` set to a folder whose name holds a space, and again with `HOME` empty, the exit code is 1, and `calls.log` holds no `go` line. |
| `refuses_unfinished_join` | With a state folder that holds `machine.key` and no `identity.json`, the exit code is 1, the output holds `did not finish` and `--rejoin`, and `calls.log` holds no `docker compose exec` line. |
| `build_failure_stops_installer` | With `SHIM_GO_FAIL=1`, the exit code is 1, the output holds `go build`, `calls.log` holds no `docker compose exec` line, and `$HOME/.local/bin/` holds no file. |
| `unsafe_path_entries_dropped` | With a search path that holds an entry with `%` and an entry with a space, the unit's `PATH` line holds neither entry and keeps the others in their order. |
| `linger_failure_is_reported` | With `SHIM_LINGER_FAIL=1`, the unit file exists, the `restart` line is in `calls.log`, the output names `loginctl enable-linger`, and the exit code is 1. |
| `rerun_keeps_joined_identity` | After a first run, a second run exits 0. `calls.log` of the second run holds `docker compose port`, no `docker compose exec` line and no `client join` line. It holds `daemon-reload` and `restart`. `identity.json` and `machine.key` are unchanged byte for byte. |
| `rerun_uses_state_folder_of_installed_unit` | After a first run with `LGTM_CLIENT_STATE_DIR` set to an absolute folder of the test, a second run without it exits 0, joins nothing, and its unit still names that folder. |
| `rerun_refuses_changed_address` | After a first run, a second run with `SHIM_WEB_PORT=8899` exits 1, prints `joined another address` and `--rejoin`, and holds no `go` line and no `systemctl` line that names the unit. |
| `rerun_reports_stopped_client` | After a first run, a second run with `SHIM_UNIT_STOPS=1` exits 1 and prints `stopped after it started` and `--rejoin`. |
| `rejoin_moves_old_state` | After a first run, `--rejoin` exits 0. A folder whose name matches `^lgtm-client\.replaced-[0-9]{8}T[0-9]{6}Z\.[A-Za-z0-9]{6}$` sits beside the state folder and holds `lgtm-client/machine.key` equal to the first one. The `systemctl --user stop lgtm-client.service` line comes after the `docker compose exec` line and before the new `client join` line. A new `machine.key` exists in the state folder. The output holds `Remove the old machine`. A second `--rejoin` in the same second makes a second archive and changes the first one in nothing. |
| `rejoin_refuses_foreign_folder` | With `LGTM_CLIENT_STATE_DIR` naming a folder of the test that holds `notes.txt`, and again naming a symbolic link to a client folder, `--rejoin` exits 1, prints `not lgtm-client's`, and leaves the folder and `notes.txt` in place. `calls.log` holds no `systemctl --user stop` line. |
| `remove_stops_and_deletes_service` | After a first run, `--remove` exits 0. `calls.log` holds `systemctl --user disable --now lgtm-client.service`. The unit file and the program are gone. The state folder and its files are unchanged. |
| `second_run_waits_for_lock` | With the lock folder present, the exit code is 1, the output holds `Another install-client.sh`, `calls.log` holds no `go` line, and the lock folder is still there. |
| `usage_on_unknown_argument` | `--frobnicate` gives exit 2 and an empty `calls.log`. |

- **Acceptance:** `bash deploy/test/installer.sh` prints `ok` for the 26 tests above and exits 0, with no Docker, network or service manager on the test machine. Criterion 2e holds. The runs of `deploy/test/checks.sh` belong to the sprint gate (architect-r1 08, logic-r1 09).

**Work item 3. The installer on macOS**

- **Files:** changes `deploy/install-client.sh`. Creates `deploy/dev.lgtmstudio.client.plist`. Changes `deploy/test/installer.sh`, where the test writer adds the tests below.
- **Needs first:** item 2, because both change the same two files.
- **Work area and complexity:** `infra`, medium. It is a second branch of a known script, and no machine of the checks runs macOS.
- **Scope:** `stack/infra`.
- **Design statements:** D16, D19, R9. The section "The client in wave 1".
- **Risky:** no. It changes no step that touches the token, which item 2's tests keep covering on both systems.
- **Behaviour:**
  - With `uname -s` equal to `Darwin`, steps 4 and 16 of item 2 are skipped, and the service steps 6, 8, 13, 15 and 17 use launchd. Step 6 reads the folder from the agent's `EnvironmentVariables` when the agent file exists.
  - Before any change, it requires `launchctl print gui/<uid>` to succeed, and otherwise gives `macOS starts lgtm-client at login; run install-client.sh from a logged-in session.` (architect-r1 09).
  - The agent file is `$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist`, rendered from the template and renamed into place.
  - The template is a property list with these keys: `Label` `dev.lgtmstudio.client`; `ProgramArguments` with the program and `run`; `EnvironmentVariables` with `LGTM_CLIENT_STATE_DIR` and `PATH`; `RunAtLoad` true; `KeepAlive` as a dictionary with `SuccessfulExit` false; `ThrottleInterval` 5; `StandardOutPath` and `StandardErrorPath` both `@LGTM_CLIENT_LOG@`; `ProcessType` `Background`.
  - Installing runs `launchctl bootout gui/<uid>/dev.lgtmstudio.client`, ignoring an agent that is not loaded, then `launchctl bootstrap gui/<uid> <agent file>`, then `launchctl enable gui/<uid>/dev.lgtmstudio.client`.
  - The wait of step 17 reads `launchctl print gui/<uid>/dev.lgtmstudio.client` for `state = running` and an unchanged `pid`.
  - `--rejoin` stops the agent with `launchctl bootout` before it moves the folder, and a failed join bootstraps it again. `--remove` runs `launchctl bootout`, then deletes the agent file and the program. A failed `bootout` of a loaded agent gives exit 1 (logic-r1 answer 7).
- **Tests written first:** added to `deploy/test/installer.sh`, with `SHIM_UNAME=Darwin`.

| Test | What it asserts |
|---|---|
| `installs_macos_launch_agent` | The exit code is 0. The agent file equals the template with its four placeholders replaced, and `@LGTM_CLIENT_LOG@` becomes `$HOME/Library/Logs/lgtm-client.log`. `calls.log` holds `launchctl print gui/<uid>` first, then `launchctl bootout`, `launchctl bootstrap gui/<uid> <agent file>` and `launchctl enable gui/<uid>/dev.lgtmstudio.client`, in that order, after the `client join` line. It holds no `systemctl` line and no `loginctl` line. |
| `macos_agent_parses` | The rendered agent file parses with `plutil -lint` when that program exists, and otherwise with the `plistlib` module of Python 3. The parsed `KeepAlive` is a dictionary whose `SuccessfulExit` is false, `ThrottleInterval` is 5, and `ProgramArguments` ends with `run`. When neither parser exists, the line is `ok <n> - macos_agent_parses # SKIP no parser`. |
| `macos_remove_boots_out_agent` | After a first run, `--remove` exits 0, `calls.log` holds `launchctl bootout gui/<uid>/dev.lgtmstudio.client`, and the agent file and the program are gone. |
| `refuses_unsupported_system` | With `SHIM_UNAME=FreeBSD`, the exit code is 1, the output holds `Linux and macOS only`, and `calls.log` holds no `go` line. |

- **Acceptance:** `bash deploy/test/installer.sh` prints `ok` for the four tests above and for the 26 tests of item 2. The steps on a Mac belong to the sprint gate.

**Work item 4. The live test of the installer and its guide**

- **Files:** creates `deploy/test/installer-live.sh` (executable). Changes `deploy/README.md` with a section named "Installing the client". The test writer creates the test file. The builder writes the section.
- **Needs first:** items 1 and 2. Plan `go-s3-client-link`, item 9. Plan `go-s3-server-machines`, items 12 to 14.
- **Work area and complexity:** `infra`, medium. It drives the real stack and the real service manager, and one of its parts needs a restart of the computer.
- **Scope:** `stack/infra`.
- **Design statements:** D16, D19, C5, C6, C7, C9. The epic's sentence that the stack, the server and the client each come back by themselves after a restart.
- **Risky:** no. It adds no code that handles a token. The token passes through the code of items 1 and 2.
- **How the test runs** (architect-r1 11, critic-r1 08, logic-r1 10, threat-r1 7, threat-r1 9):
  - It runs on a disposable Linux user, with a stack started by `deploy/up.sh` with a password made for the test. It needs `LGTM_WEB_PASSWORD`, a systemd user manager, Docker, and `systemctl is-enabled docker.service` printing `enabled`, so the stack can come back at boot.
  - The default part needs `LGTM_INSTALL_LIVE=1`. It refuses to start when the unit, the program or the default state folder already exists, so it never adopts an earlier installation.
  - It reads the address the way the installer does. It signs in and reads `ClientService.WatchClients` the way `deploy/test/checks.sh` does in `signin`, `open_watch` and `read_frame`, except that the password reaches `jq` on standard input, and every request body and cookie jar sits in a private folder that the test removes. It signs out at the end of each part.
  - The default part writes the fixture record `$HOME/.local/state/lgtm-installer-live/fixture`, mode `0600`, with the Compose project, the state folder, the boot identifier from `/proc/sys/kernel/random/boot_id` and the new machine's identifier. `--after-reboot` and `--cleanup` refuse to run without it, act only on the machine it names, and `--cleanup` removes it last.
  - `--after-reboot` checks the state after a restart. `--cleanup` removes the machine with `ClientService/RemoveClient`, runs its one test, and then runs `deploy/install-client.sh --remove`.
- **Tests written first:** each prints one TAP line. Before items 1 and 2 exist, the first test fails on the installer's exit code.

| Test | What it asserts |
|---|---|
| `live_install_leaves_machine_waiting` | `deploy/install-client.sh` exits 0. The first `WatchClients` message after it lists exactly one client that was absent before, and that client is waiting for approval and has `hostsServer` true. Its identifier goes into the fixture record. |
| `live_service_enabled_with_linger` | `systemctl --user is-enabled lgtm-client.service` prints `enabled`. `is-active` prints `active`. `loginctl show-user <user> --property=Linger --value` prints `yes`. |
| `live_service_restarts_after_crash` | After `systemctl --user kill --signal=KILL lgtm-client.service`, the unit is active again within 15 seconds with another main process identifier (`MainPID`). |
| `live_approved_machine_online` | After `ClientService/ApproveClient` for the recorded machine, a `WatchClients` stream shows it online with `hostsServer` true within 35 seconds. |
| `live_reconnect_restarts_service` | After `ClientService/ReconnectClient` for the recorded machine, `NRestarts` grows by exactly one within 35 seconds, the user journal of the unit since the call holds `status=75` for the ended run, the unit has another `MainPID`, and a new `WatchClients` stream shows the machine online with a later `connectedAt` (logic-r1 10, threat-r1 9). |
| `live_rerun_keeps_machine` | A second `deploy/install-client.sh` exits 0. The count of clients is unchanged. The recorded machine is online again within 35 seconds. The SHA-256 of `identity.json` is unchanged. |
| `live_service_started_before_login` | With `--after-reboot`: the boot identifier differs from the recorded one. The unit's `ActiveEnterTimestampMonotonic` is positive and smaller than the `TimestampMonotonic` of every session of the user whose `Class` is `user`. The test reads the sessions with `loginctl list-sessions` and `loginctl show-session`. It leaves out the sessions of class `manager`, because systemd opens one for the user's own service manager at boot when the user lingers. Both properties count microseconds from the same boot. The guide tells the person to run this part from the first login after the restart. |
| `live_machine_online_after_reboot` | With `--after-reboot`: within 60 seconds of the start of the test, a `WatchClients` stream shows the recorded machine online, with no start of the stack or the service by hand. |
| `live_removed_machine_stays_stopped` | With `--cleanup`, after `RemoveClient` of the recorded machine: within 35 seconds the unit is `inactive`, and the journal holds a clean exit of its last run. Fifteen seconds later `NRestarts` is unchanged and the unit is still `inactive`. |

- **The guide:** the section "Installing the client" of `deploy/README.md` names the command, its two options, the paths of the program, the unit or agent and the state folder, and how to read the client's output on each system. It says that one user of the computer runs the installer, that a Mac starts the client at login, that a Linux restart brings the client back only when the user lingers and Docker starts at boot, that a removed machine, a changed port or an unfinished join needs `--rejoin` (logic2-r1 05), and that a client started by hand must be stopped before `--rejoin` (logic-r1 05). It says that whoever can run a command in the server's container can make a join token, and that the token is visible in the list of processes while the join runs. It follows the repository's writing style.
- **Acceptance:** the item's own gate is a dry run of `deploy/test/installer-live.sh` without `LGTM_INSTALL_LIVE`, which prints `1..9`, exits 2, and changes nothing. The live parts belong to the sprint gate.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The service runs after a login only, because lingering is off. | High | The installer turns lingering on and fails loudly when it cannot. `live_service_started_before_login` proves the start at boot from a new boot identifier and the clocks of the system. |
| A failed `--rejoin` leaves a working machine stopped. | High | The token is fetched before anything stops, and a failed join puts the old folder back and starts the service. `rejoin_failure_restores_old_state`. |
| The service restarts without end after the owner removed the machine. | Medium | A removal ends `run` with status 0, and the service leaves an exit 0 alone. `live_removed_machine_stays_stopped`. |
| The installed client dials an address that disagrees with its token or its stack. | Medium | The address and `LGTM_PUBLIC_URL` both come from the published port, and a rerun refuses an identity of another address. `asks_server_for_hosts_server_token` and `rerun_refuses_changed_address`. |
| The service does not find a harness that the user's shell finds. | Medium | The installer copies its own search path into the service. `unsafe_path_entries_dropped` checks the copy. A harness installed later in a new folder needs a second run, which the guide says. |
| The token is readable by every local user in the list of processes for the length of the join (logic2-r1 04). | Medium | The token works once and expires in 15 minutes, and the owner's approval still gates the machine. Whether the client also reads the token from standard input is for the owner, in the result file. |
| A Mac starts the client only at login. | Low | Decision 13 records it, and the guide says it. |

**Verify:** `bash deploy/test/installer.sh` on any Linux machine, with no Docker. The sprint gate below for every run against the stack.

**Rollout detail:** steps 1, 3 and 4 of the rollout order. The live test runs on a disposable Linux user, because it installs a real service.

**The sprint gate**

These runs need the merged programs of several teams, so they are joint gates of the sprint, not gates of one item (architect-r1 08, critic-r1 09, logic2-r1 03). Before them, the Stack team rebuilds the server image with `docker compose build server` and recreates the stack with `deploy/up.sh`. They run on a Linux runner with Docker and the loopback network, by issue 1640.

| # | Run | Expected result |
|---|---|---|
| G1 | `docker compose exec -T server lgtm-server join-token --hosts-server`, with the output captured into a private file and only its shape reported. | Exactly one line matching `^lgtm1_[A-Za-z0-9_-]+$`, empty standard error, exit 0. |
| G2 | `LGTM_INSTALL_LIVE=1 deploy/test/installer-live.sh`, then a restart of the computer by a person, then `--after-reboot` from the first login, then `--cleanup`. | Six, two and one `ok` lines. |
| G3 | On a Mac with the stack running: `deploy/install-client.sh`, then `launchctl print gui/$(id -u)/dev.lgtmstudio.client`. Approve the machine and press its restart on the Machines page. Log out and log in. Remove the machine. | `state = running` after the install. `last exit code = 75` and `state = running` after the restart. `state = running` after the login. After the removal, the agent is not running and its last exit code is 0. |
| G4 | `deploy/test/checks.sh`, with `LGTM_WEB_PORT` exported in the shell when the stack uses another port than 8787, as the script's guide says. It needs `psql_in_stack` of plan `go-s1-stack-vault`, item 2, the table of plan `go-s3-server-machines`, item 3, and the log names of plans `go-s3-client-link` and `go-s3-server-machines` that C5, C10 and C11 read (logic-r1 11, threat-r1 8). | 13 lines that start with `ok`, and exit 0. |

**Post-build checks**

| # | Scenario | Expected result |
|---|---|---|
| PB1 | Run `deploy/install-client.sh --rejoin` while `docker compose stop server` holds the server down, and kill a second `--rejoin` after its join committed on the server and before the client wrote `identity.json`. | The first exits 1 and the old machine is online again. The second leaves a folder that the next run refuses with `did not finish`, and the Machines page shows at most one waiting machine for the owner to remove. |
| PB2 | Run the installer with shell tracing turned on in the caller, `bash -x deploy/install-client.sh`, and read `ps -eo args` as another user during the join. | The trace holds no token. The process list shows the token only as the last argument of `lgtm-client join`, as the guide states. |
| PB3 | Run the installer as a user outside the `docker` group. | It exits 1 with `may run docker` and changes no file. |

**Outside wave 1**

A second user of the same computer who runs the installer gets a second machine with the mark, because D19 makes one user per machine a convenience and the installer cannot see another user's service (architect-r1 06). A restore of a state folder and two live copies of one state stay outside wave 1, by issues 1641 and 1642.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe: the brief's work items 1 to 3, its three observations, its two agreements and its two points to settle; the specific points of the task files `plan-go-s3-stack-client.md`, `reconcile-go-s3-stack-client.md` and `fold-go-s3-stack-client-r1.md`; the requests to this plan in `requests-from-contract-r3.md`, `requests-from-contract-r4.md`, `requests-from-client-r4.md` and the fold of round 2 of `go-s2-stack-web`; the findings of round 1 in `attack/`; issues 1636 to 1642 in `docs/issues.md`; the names table of `deploy/test/README.md`; and the design statements D7, D16, D17, D19, R3, R8, R9, R10, C1 to C13, and the design's sentences on the server's own command, the service of the user and `deploy/install-client.sh`.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the server's command that makes a token | WS-1 | Item 1. |
| Brief item 2, the installer and its service | WS-2 | Items 2 and 3. |
| Brief item 3, C5 to C11 turned on | WS-2 | Plan `go-s2-stack-web`, item 7, proves the link through the web server. Item 2 changes the script's state folder. Gate G4 runs the whole script. Decisions 20 and 31. |
| Brief observation, the installer ends with the machine waiting | WS-2 | `live_install_leaves_machine_waiting`. |
| Brief observation, the client starts by itself after a restart | WS-2 | `live_service_started_before_login`, on Linux. |
| Brief observation, the user-flow script passes 13 checks | WS-2 | Gate G4. |
| Brief agreement, the Server team's lead merges item 1 | WS-1 | Item 1, scope `server/backend`. Decision 8. |
| Brief agreement, the name of the service and where the program goes | WS-2 | Public names table. Decisions 12 to 14. |
| Brief point 1, a service on macOS | WS-2 | Item 3. Decision 13. |
| Brief point 2, build or download | WS-2 | Item 2, step 12. Decision 11. |
| Task file, the script exists and is not rewritten | WS-2 | Only lines 219 and 233 change. Decision 35. |
| Task file, issue 1638, where `hosts_server` comes from | WS-1 | `--hosts-server`. Decision 10. |
| Task file, the service comes back after a restart | WS-2 | Item 2, steps 15 and 16, and item 4. Decision 12. |
| Contract round 3, the address the installer fills in | WS-2 | The section "The service, the address and the exit statuses". Decision 33. |
| Contract round 3, a failing join is a failure, and the service restarts `run` by its statuses | WS-2 | The same section. Decision 34. |
| Contract round 3, its post-build checks of exit statuses and of the installer | WS-2 | `live_reconnect_restarts_service`, `live_removed_machine_stays_stopped`, `live_install_leaves_machine_waiting` and gate G3. Decision 34. |
| Contract round 4, refusals in the error's metadata | WS-1 | `TestJoinTokenJoinsOnce`. |
| Client round 4, ruling 5: `identity.json`, one state setting, and the script's lines | WS-2 | Item 2, steps 6 and 11, and the change of the script. Decisions 32 and 35. |
| Stack web round 2, request P1: the port set only in `deploy/.env` | WS-2 | Item 2, step 9. Decision 33. |
| Round 1 findings | WS-1, WS-2 | `attack/round-1-findings.md`. Decisions 40 to 45. |
| Issues 1636 and 1637 | none | They concern the website, the contract and the Server team. |
| Issue 1639, replacing the server's keys | none | Outside wave 1, by plan `go-s3-server-machines`. `--rejoin` serves a removed machine. Decision 36. |
| Issue 1640, a sandbox without Docker | WS-2 | The sprint gate needs Docker. Items 1, 2 and 3 do not. |
| Issues 1641 and 1642, a restore and two live copies | none | Outside wave 1. The plan adds no rule for them. |
| D7, a token that authorizes an exchange of keys | WS-1, WS-2 | Items 1 and 2. |
| D16, a restart of a connected client and a retry every 15 seconds | WS-2 | `live_reconnect_restarts_service`. |
| D17, trust without a public authority | WS-1 | The token carries the fingerprint that the machines part fills in. |
| D19, one user on a machine | WS-2 | `refuses_root`, decision 27, and the section "Outside wave 1". |
| R3, one part calls another part's interface | WS-1 | Item 1 calls `Tokens.Create`. |
| R8, an error is one sentence | WS-1, WS-2 | Every refusal of items 1 and 2 is one fixed sentence. |
| R9, secrets never reach a log | WS-1, WS-2 | `TestJoinTokenPrintsOneTokenLine`, `TestJoinTokenRefusesUnknownArgument` and `token_never_printed`. |
| R10, real PostgreSQL | WS-1 | Item 1's tests use `LGTM_TEST_POSTGRES`. |
| C1 to C4, C7 to C9, C11 to C13 | WS-2 | Gate G4 runs them. Other plans make them pass. |
| C5, C6, C10 | WS-1, WS-2 | Item 1, criterion 2e and gate G4. |
| The design's "or with the server's own command" | WS-1 | Item 1. |
| The design's "a service of its user, outside the compose stack" | WS-2 | Items 2 and 3. |
| The design's "`deploy/install-client.sh` ... joins it with a token the server's own command makes" | WS-2 | Item 2, step 13. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "What this project is": the three parts meet only through the contract. | The installer and the command use the contract's `Join`, `GetServer` and `WatchClients`. The installer reads of the client's files only which names exist and the `address` of `identity.json`. |
| `AGENTS.md`, "Where things are": new code does not go into `packages/`. | Every file is in `deploy/` or `src/server/cmd/lgtm-server/`. |
| The epic, "The teams, and what each may edit": the Stack team edits `deploy/`. | Items 2 to 4 edit only `deploy/`. Item 1's three files are the Server team's work, done by its own seats. Decision 8. |
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
| 2. The installer on Linux | `infra` | `stack/infra` | medium | One script of known steps with several failure paths and a restore, and it passes a join token. | no | none to build; names from item 1 and `go-s3-client-link` | 1 | D7, D16, D19, R8, R9, C6 |
| 3. The installer on macOS | `infra` | `stack/infra` | medium | A second branch of a known script, verified on Linux and by a person on a Mac. | no | 2 | 1 | D16, D19, R9 |
| 4. The live test and the guide | `infra` | `stack/infra` | medium | It drives the real stack and service manager, and one part needs a restart. | no | 1, 2; `go-s3-client-link` item 9; `go-s3-server-machines` items 12 to 14 | 3 | D16, D19, C5, C6, C7, C9 |

Load: four test writers, four builders, and two mutation runs for the risky items 1 and 2, which gives 10 execution seats. Item 1's seats are Server seats. Alternating the two primary vendors gives five seats to each. The sprint gate needs a seat with access to Docker and to the loopback network, by issue 1640, and a person for the restart and the Mac. Items 1, 2 and 3 do not need Docker, and item 1 needs only the disposable PostgreSQL of `deploy/test/with-postgres.sh`. A review round of this plan uses the seats that the epic names. The orchestrator checks the weekly limits at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| Round 1 | 5, 37, 12 as reported in 54 findings; 2, 13, 5 in 20 groups after duplicates. | Folded on 2026-11-26. The register is `attack/round-1-findings.md`. |

</details>
