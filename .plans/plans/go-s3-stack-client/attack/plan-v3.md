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

Sprint 3 makes a machine join the server. This plan gives the owner one command that puts the client on the same computer as the stack. The command builds the client, joins it to the server once, writes one service file and starts the service of the owner's user. After the command, the machine waits for approval on the Machines page. On Linux, after a restart of the computer, the client starts again without anyone logging in, when the user lingers and Docker starts at boot. On macOS, the client starts when the owner logs in, because a start at boot would need an administrator, so the unattended return is proved on Linux only (critic-r1 07).

The join needs a token. The design lets the owner make a token on the Machines page or with the server's own command. This plan writes that command, `lgtm-server join-token`, because the installer cannot click a page. A token from this command can also carry one extra mark: the machine that uses it runs the server. That mark is how the Machines page learns which machine is the server's own, which no message of the link reports. Whoever can run a command inside the server's container can make such a token, so the mark records how the token was made and grants nothing (architect-r1 02).

The user-flow script `deploy/test/checks.sh` already exists, and it must pass all 13 of its checks at the end of the sprint. Plan `go-s2-stack-web` already proves that its web server carries the link's two-way stream on the loopback address. This plan changes two lines of the script, so its clients get their state folder through the client's own setting. The exit statuses of the client, what the service manager does with them, and which address the installer fills in are rules of the programs, not of the contract. This plan states them once, in its section "The service, the address and the exit statuses", each with the test that proves it.

## Key decisions

- **The installer builds the client from the checkout.** No built client is published anywhere yet, and nothing would sign one. The checkout is already on the machine, because the stack starts from it.
- **On Linux the client is a systemd user service, and the user lingers.** Lingering is the setting that lets a user's services start at boot without a login. When the installer cannot turn it on, it says so and exits with a failure, but the client is still installed.
- **Nothing is installed before the join succeeds.** The installer joins with the newly built program from a temporary file. Only after the join does it install the program and the service.
- **The installer joins once and never moves a state folder.** A second run updates the program and restarts the service. To join again, after a removal on the Machines page or a changed port, the owner removes the client, moves the state folder aside by hand, and installs again, as the guide says. **UNREVIEWED MECHANISM** (architect-r2 01 to 03, logic-r2 01 to 03, threat-r2 01 to 03): removing `--rejoin`, its lock and its folder check makes a stranded or wrongly moved folder impossible, where three rounds of rules could not (decision 46).
- **The token marks the server's own machine.** The machine never claims the mark itself, so a remote machine cannot pose as the server's machine.
- **The service starts the client again after every exit except 0.** Plan `go-s3-client-link` makes `run` exit 0 for a stop for good, 75 for a restart that the server asked for, and 1 for a failure.
- **The installed client dials the stack's own loopback port.** The installer asks Compose for the published port the same way the launcher calls Compose, so a port set only in `deploy/.env` is still found.

Full log → `decisions.md`

## Workstreams

### The server makes a join token from its own command

The owner can make a token without the website, with one command inside the server's container. The command prints the token and nothing else, so a script can capture it. The token names the server's own public address, which is the only address the machines part puts in a token.

The risk is that the token's secret leaks into a log. The command writes nothing on success except the token on standard output, and the tests check that standard error stays empty.

When it is done, a person runs the command against the stack and gets one line that starts with `lgtm1_`. A machine that joins with it shows on the Machines page as the machine that runs the server.

### The client installs as a service that comes back by itself

`deploy/install-client.sh` takes the computer from a running stack to a joined client that runs as a service. It finds the stack's port, checks that the stack answers, builds the client, asks the server for a token, joins, and then installs and starts the service. A second run updates the program and keeps the identity. `--remove` removes the service and keeps the state.

The main risk is a service that looks installed but does not start after a restart. A live test on a Linux machine kills the client and watches it come back. A person restarts the computer, and the test then proves that the machine connected to the server before the first login. A second risk is the token in the installer's output, which the tests rule out the same way as for the command.

When it is done, the installer ends with the machine waiting for approval. After approval the machine shows as online. After a restart of the computer it shows as online again. After a removal on the Machines page the client stops at its next connection.

## Rollout order

1. Build the installer on Linux, with its tests that need no stack. Its tests replace every outside program with a stand-in, so it can start before any link code exists. Its names from plans `go-s3-server-machines` and `go-s3-client-link` freeze when both of those plans have cleared review, by the epic's rule of 2026-11-26.
2. Build the server's token command when the Server team's token and join code are merged. The Server team's test writer and builder do it, and its lead merges it (critic-r1 09).
3. Build the macOS part of the installer after the Linux part is merged, because both change the same script.
4. Run the sprint gate below once the client, the server, the stack and the Machines page of sprint 3 are merged and the server image is rebuilt. A person restarts the computer between the parts of the live test.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**How the statements of the design are cited**

The design is `docs/specs/2026-11-25-go-rewrite-foundation-design.md` on `master`.
D1 to D23 are the rows of its decisions table, in order. D7 is the key pair and the join token. D16 is the restart of a connected client and the retry every 15 seconds. D17 is trust without a public authority. D19 is one user on a machine.
R3 is "a part calls another part's Go interface". R8 is "an error is one sentence". R9 is "secrets never reach a log". R10 is "tests run against a real PostgreSQL". C1 to C13 are its checks of wave 1.
`scouting.md` lists each statement with its line. A workstream (WS) groups work items, and WS-1 and WS-2 are the two above.

**Public names that other work items and teams build against**

| Name | Kind | Meaning | Set by |
|---|---|---|---|
| `lgtm-server join-token [--hosts-server]` | command | Makes one join token for the server's public address. Prints its text, one line that starts with `lgtm1_`, on standard output. Exit 0 on success, 1 when the settings, the database, the vault, the token or the write of the line fail, and 2 for a wrong argument. Every message is one fixed sentence on standard error, which repeats no argument (threat-r1 2). | item 1 |
| `joinTokenCommand(ctx context.Context, args []string, lookup func(string) (string, bool), stdout, stderr io.Writer) int` | Go function in package `main` of `src/server/cmd/lgtm-server` | The body of the command. The Server team writes it, and the dispatcher of plan `go-s2-server-signin`, item 9, calls it with `args` after the word `join-token`. | item 1 |
| `deploy/install-client.sh [--remove]` | command | Installs or removes the client on this computer. Exit 0 on success, 1 on a failure, 2 for a wrong argument. | items 2 and 3 |
| `$HOME/.local/bin/lgtm-client` | file | The installed program, mode `0755`. | item 2 |
| `lgtm-client.service` at `${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/`, from `deploy/lgtm-client.service` | file and template | The user unit on Linux, with the placeholders `@LGTM_CLIENT_BIN@`, `@LGTM_CLIENT_STATE_DIR@` and `@LGTM_CLIENT_PATH@`. Its line `Environment=LGTM_CLIENT_STATE_DIR=` names the installation's state folder, which a later run reads. | item 2 |
| `dev.lgtmstudio.client` and `deploy/dev.lgtmstudio.client.plist` | launchd label and template | The agent on macOS, at `$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist`, with the placeholders of the unit and `@LGTM_CLIENT_LOG@`, which becomes `$HOME/Library/Logs/lgtm-client.log`. | item 3 |

The names this plan reads from other plans:

| Name | Plan |
|---|---|
| `config.Load(lookup)` and its fields `PublicURL`, `Postgres`, `AllowPrivateStack`, `OpenBaoAddr` and `OpenBaoCredentialFile`; `store.Open(ctx, config, store.ConnectionPolicy{AllowPrivateStackPostgres})` and `Store.Close()`; `vault.Open(addr, credentialFile, allowPrivateStack)`, `vault.LoadLinkIdentity(ctx, store, client, false)` and `LinkIdentity.SigningKey`. | `go-s2-server-signin` items 1, 8 and 9; `src/server/internal/store/pool.go`. |
| `machines.NewRegistry()`, `machines.NewTokens(store, registry, machines.Issuer{PublicURL, ServerKey})`, `Tokens.Create(ctx, machines.TokenOptions{HostsServer})`, `NewToken.Text` and `machines.DecodeToken`; `servertest.Start`, `Server.URL`, `Server.SigningPublicKey`, `Server.Environment()`, `Server.Clients(t)` and `Server.Link()`. | `go-s3-server-machines` items 1, 3 and 14; `go-s2-server-signin` item 9. |
| `lgtm-client join <address> <token>`, `lgtm-client run`, `lgtm-client version`, `-X main.version=<v>`, and their exit statuses. | `go-s1-client-local` item 1; `go-s3-client-link` item 9. |
| `LGTM_CLIENT_STATE_DIR`, the default `$HOME/.local/state/lgtm-client`, and the files `machine.key` and `identity.json`. | `go-s1-client-local`, "The layout of the state folder". |
| The Compose project `lgtm`, the launcher's rule for calling Compose, the service `server` as user 10001 with `/usr/local/bin/lgtm-server`, the service `web` and its port 8080, and `LGTM_PUBLIC_URL` equal to `http://127.0.0.1:${LGTM_WEB_PORT:-8787}`. | `go-s1-stack-vault`; `go-s2-stack-web` items 1, 4 and 5. |

**The service, the address and the exit statuses**

These rules belong to this plan, because the contract states only facts of the wire. Each is stated here once.

| Rule | Proved by |
|---|---|
| The installer calls Compose as the launcher does: `docker compose -f <repository>/deploy/compose.yaml` when `COMPOSE_FILE` is unset or empty, and `docker compose` without `-f` when it is set (architect-r2 04, logic-r2 04). It fills in the address `http://` followed by the output of `port web 8080`, which must be `127.0.0.1:<port>`. The stack's `LGTM_PUBLIC_URL` comes from the same port, so the address agrees with the address inside the token. | `asks_server_for_hosts_server_token` and `live_install_leaves_machine_waiting`. |
| A failing `lgtm-client join` is a failure of the installer. It exits 1 and installs neither the program nor the service. | `join_failure_installs_no_service`. |
| A successful run means that the service manager started the client. Whether the machine connects is shown on the Machines page and proved by the live test, not by the installer (logic-r2 08). | `installs_linux_user_service` and `live_approved_machine_online`. |
| The service starts the client again about 5 seconds after every exit except 0, without a limit on the count. On Linux this is `Restart=on-failure`, `RestartSec=5` and `StartLimitIntervalSec=0`. On macOS it is `KeepAlive` with `SuccessfulExit` false and `ThrottleInterval` 5, where launchd may wait longer. | `installs_linux_user_service`, `macos_agent_parses`, `live_service_restarts_after_crash`. |
| A restart that the server asked for ends `run` with status 75, and the service starts the client once more. | `live_reconnect_restarts_service`, and gate G3 on a Mac. |
| A removal ends `run` with status 0 at the client's next connection, and the service does not start it again until the next boot or login (logic-r1 07). | `live_removed_machine_stays_stopped`. |
| The service and the installer's `join` use the same state folder, passed as `LGTM_CLIENT_STATE_DIR`. A later run without the variable takes the folder from the installed unit. | `joins_before_service_starts` and `rerun_uses_state_folder_of_installed_unit`. |

Plan `go-s3-client-link` proves the statuses of the program itself in `TestRunCommandExitStatus`, the result of a restart sent from its receipt in `TestRestartReplaysAfterDroppedResult`, and a client log without secrets in `TestAppLogRedactsSecrets`.

**WS-1: The server makes a join token from its own command**

The command is a short program run inside the server's container. It opens the server's settings, database and vault the same way the server does, without the server's lock and without migrating. It asks the machines part to make a token, by rule R3, and prints the text. It adds one fact that nothing else on the link carries: the mark that the joining machine runs the server.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1a | The command prints one token line, and a client can join with that token once. | `TestJoinTokenPrintsOneTokenLine`, `TestJoinTokenCarriesAddressAndServerKey` and `TestJoinTokenJoinsOnce` pass with `LGTM_TEST_POSTGRES` set. |
| 1b | A token made with `--hosts-server` gives a machine whose `hosts_server` is true, and a token made without it gives false. | `TestJoinTokenHostsServerFlag` passes. |
| 1c | A wrong argument, a missing database and a failed write are refused, each with one fixed sentence. | `TestJoinTokenRefusesUnknownArgument`, `TestJoinTokenFailsWithoutDatabase` and `TestJoinTokenFailsOnWriteError` pass. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1.1 | Write the tests against a real server and database, then the command. | `src/server/cmd/lgtm-server/join_token_test.go` and `join_token.go`, `joinTokenCommand` (targets). Pattern: `src/server/cmd/lgtm-server/main.go`, `run`, and `src/server/servertest/server_test.go`, `TestStartAndRestartServer`, of plan `go-s2-server-signin`, item 9. |

**Work item 1. The server's own command that makes a join token**

- **Files:** creates `src/server/cmd/lgtm-server/join_token.go` and `src/server/cmd/lgtm-server/join_token_test.go`. Changes the entry `join-token` of the dispatcher's table in `src/server/cmd/lgtm-server/main.go`. A Server test writer creates the test file, and a Server builder writes the rest (critic-r1 09).
- **Needs first:** plan `go-s2-server-signin`, items 1, 8 and 9. Plan `go-s3-server-machines`, items 1, 3, 4 and 14. It runs at the same time as item 2.
- **Work area, complexity and scope:** `backend`, medium, `server/backend`. It is one small command over interfaces that other items build, but it handles a join token. The Server team's lead merges it. See decision 8.
- **Design statements:** D7, D17, R3, R8, R9, R10, C5, C6. The section "Joining: a token that authorizes an exchange of keys", step 1.
- **Risky:** yes, because it makes a join token. Passing `false` for the mark in every case must fail `TestJoinTokenHostsServerFlag`. Writing the token's text to standard error must fail `TestJoinTokenPrintsOneTokenLine`.
- **Behaviour:**
  - The only argument is `--hosts-server`, which takes no value. Any other argument, a value, or a repeated flag gives exit 2, nothing on standard output, and the fixed sentence `usage: lgtm-server join-token [--hosts-server]` on standard error.
  - The command reads the settings with `config.Load(lookup)`. It opens the store with `store.Open(ctx, cfg.Postgres, store.ConnectionPolicy{AllowPrivateStackPostgres: cfg.AllowPrivateStack})`, opens the vault client with `vault.Open`, and reads the server's identity with `vault.LoadLinkIdentity` and `allowInitialize` false. It takes no instance lock, runs no migration and opens no port. It closes the store before it returns. The vault client has no close method.
  - It calls `machines.NewTokens(s, machines.NewRegistry(), machines.Issuer{PublicURL: cfg.PublicURL, ServerKey: <public half of LinkIdentity.SigningKey>})` and then `Tokens.Create(ctx, machines.TokenOptions{HostsServer: hostsServer})`. The issuer is the only writer of the token's address, which is always `PublicURL` (architect-r2 08, threat-r2 10). Wave 1 has no certificate, so the pin is empty, and an `https` public address gives `ErrNoCertificatePin`.
  - On success it writes `NewToken.Text` and one newline to standard output, and nothing else. A failed write to standard output gives exit 1.
  - A failure of the settings, the store, the vault or the token gives exit 1 and one sentence on standard error that names the part that failed and holds no setting's value.
- **Tests written first,** in `join_token_test.go`, package `main`. Each test starts a server through `servertest.Start` and calls `joinTokenCommand` with a lookup over `Server.Environment()`. The tests need the disposable PostgreSQL of `deploy/test/with-postgres.sh`, which runs in Docker, or another database named by `LGTM_TEST_POSTGRES` (logic-r2 10). A stub that returns 0 and writes nothing fails every positive test on its first assertion.

| Test | What it asserts |
|---|---|
| `TestJoinTokenPrintsOneTokenLine` | With no argument, the exit code is 0. Standard output matches `^lgtm1_[A-Za-z0-9_-]+\n$` exactly. Standard error is empty. |
| `TestJoinTokenCarriesAddressAndServerKey` | `machines.DecodeToken` of the line gives a token whose `address` equals `Server.URL`, whose `token_id` is a version 7 universally unique identifier (UUID), whose `secret` has 32 bytes, whose `server_key_sha256` equals the SHA-256 hash (Secure Hash Algorithm, 256 bits) of `Server.SigningPublicKey`, and whose `certificate_key_sha256` is empty. |
| `TestJoinTokenJoinsOnce` | `Join` through `Server.Link()` with the token's `token_id`, its secret and a fresh Ed25519 public key succeeds. A second `Join` with the same token and another key returns `permission_denied` with `Lgtm-Join-Refusal` equal to `token_used` in the error's metadata. |
| `TestJoinTokenHostsServerFlag` | One token is made with `--hosts-server` and one without. A machine joins with each. The first message of `WatchClients` through `Server.Clients(t)` lists both as waiting. The machine of the first token has `hosts_server` true, and the other has false. |
| `TestJoinTokenRefusesUnknownArgument` | `--address x`, `--hosts-server=yes`, `--hosts-server` given twice, and the bare word `lgtm1_marker` each give exit 2, an empty standard output, and exactly the usage sentence on standard error. |
| `TestJoinTokenFailsWithoutDatabase` | With the database port of the environment changed to a closed loopback port, the exit code is 1, standard output is empty, and standard error is one line that ends with a full stop and holds no value of the environment. |
| `TestJoinTokenFailsOnWriteError` | With a standard output whose writes fail, the exit code is 1 and standard error is one line. |

- **Acceptance:** `deploy/test/with-postgres.sh -- go test -count=1 -run 'TestJoinToken' ./src/server/cmd/lgtm-server/` passes all seven tests. The run inside the stack belongs to the sprint gate.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The token's secret reaches a log of the command. | High | The command logs nothing, and its usage sentence repeats no argument. `TestJoinTokenPrintsOneTokenLine` and `TestJoinTokenRefusesUnknownArgument`. |
| A token marks a remote machine as the server's machine. | Medium | Only the command sets the mark, and it runs inside the server's container. `CreateClientToken` never sets it, by plan `go-s3-server-machines`, item 12. The mark grants nothing. |
| The command and the running server disagree on the signing key. | Medium | The command reads the pinned identity through the server's own function, and `TestJoinTokenCarriesAddressAndServerKey` compares the token with the server's key. |

**Verify:** the seven tests above. **Rollout detail:** step 2 of the rollout order.

**WS-2: The client installs as a service that comes back by itself**

The design says that the client "runs as a service of its user, outside the compose stack", and that `deploy/install-client.sh` "installs the client on the same machine and joins it". The hermetic tests replace every outside program with a stand-in on `PATH`, so they need no Docker, no network and no service manager. The live test runs the real installer against the real stack and the real service manager of a Linux user.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2a | On Linux, the installer finds the port, builds the program, joins with a token of the server's command, and only then installs the program and the user unit. | `installs_linux_user_service`, `asks_server_for_hosts_server_token` and `joins_before_service_starts` in `bash deploy/test/installer.sh`. |
| 2b | The token never appears in the installer's output or in any file under the test's folder. | `token_never_printed`. |
| 2c | A failed join or a refused precondition installs nothing. | `join_failure_installs_no_service`, `refuses_root`, `refuses_missing_tool`, `refuses_docker_failure`, `refuses_stack_not_answering`, `refuses_malformed_token`, `refuses_missing_user_manager`, `refuses_unsafe_home`, `refuses_unfinished_join` and `build_failure_stops_installer`. |
| 2d | A second run keeps the identity and its state folder, and `--remove` removes the service and keeps the state. | `rerun_keeps_joined_identity`, `rerun_uses_state_folder_of_installed_unit`, `remove_stops_and_deletes_service` and `remove_needs_no_build_tools`. |
| 2e | The user-flow script gives its clients their state folder through `LGTM_CLIENT_STATE_DIR`. | `! grep -q XDG_STATE_HOME deploy/test/checks.sh` exits 0. |
| 2f | On macOS, the installer loads a launchd agent that parses as a property list. | `installs_macos_launch_agent`, `macos_agent_parses` and `macos_remove_boots_out_agent`. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 2.1 | Write the installer for Linux and the user unit template. | `deploy/install-client.sh`, `main`, and `deploy/lgtm-client.service`, section `[Service]` (targets). Pattern: `deploy/up.sh`, `compose` at line 19, `deploy/test/checks.sh`, `get_server` at line 60, and the systemd unit format in `scouting.md`. |
| 2.2 | Write the hermetic tests and their stand-ins. | `deploy/test/installer.sh`, `run_test` (target). Pattern: `deploy/test/checks.sh`, `run_check` at line 658, for one Test Anything Protocol (TAP) line per test. |
| 2.3 | Give the user-flow clients their state folder. | `deploy/test/checks.sh`, `client_join` at line 219 and `start_client` at line 233, and `deploy/test/README.md`, the row "Client state folder". |
| 3.1 | Add the macOS branch and the agent template. | `deploy/install-client.sh`, `install_launch_agent` (target), and `deploy/dev.lgtmstudio.client.plist` (target). |
| 4.1 | Write the live test of the installer and the guide. | `deploy/test/installer-live.sh`, `after_reboot`, and `deploy/README.md`, section "Installing the client" (targets). Pattern: `deploy/test/checks.sh`, `open_watch` at line 238 and `read_frame` at line 251. |

**Work item 2. The installer on Linux**

- **Files:** creates `deploy/install-client.sh` (executable), `deploy/lgtm-client.service` and `deploy/test/installer.sh` (executable). Changes lines 219 and 233 of `deploy/test/checks.sh` and one row of `deploy/test/README.md`. The test writer creates the test file and its stand-ins.
- **Needs first:** nothing, for its own tests. It runs at the same time as item 1. Its real runs belong to the sprint gate.
- **Work area, complexity and scope:** `infra`, medium, `stack/infra`. It is one script of known steps with a few failure paths, and it passes a join token.
- **Design statements:** D7, D16, D19, R8, R9, C6. The sections "The client in wave 1" and "The compose stack".
- **Risky:** yes, because it passes a join token. Printing the token with `printf '%s\n' "$token"` must fail `token_never_printed`. Installing the program or the unit before the join must fail `joins_before_service_starts`.
- **Behaviour, in order.** Each refusal prints one sentence, exits 1 and changes no file.
  1. The script runs with shell tracing off (`set +x`) from its first line, and changes to the repository root. It accepts no argument or `--remove`. Anything else prints the usage and exits 2.
  2. It refuses to run when `id -u` is 0, with `install-client.sh must run as the user who runs the harnesses, not as root.`
  3. It reads `uname -s`. `Linux` takes the steps below, `Darwin` the steps of item 3, and anything else gives `install-client.sh supports Linux and macOS only.`
  4. On Linux it runs `systemctl --user show-environment`. A failure gives `This Linux system has no systemd user manager for this user.`
  5. It refuses a `HOME` that is empty, not absolute, or holds a character outside letters, digits and `. _ / + @ -`, with `The home folder path holds a character that a service file cannot carry.`
  6. The state folder is `LGTM_CLIENT_STATE_DIR` when that is set and not empty, otherwise the folder of the line `Environment=LGTM_CLIENT_STATE_DIR=` of the installed unit, and otherwise `$HOME/.local/state/lgtm-client`. A value that is not absolute, or holds a character refused in step 5, gives `LGTM_CLIENT_STATE_DIR must be an absolute path.` The service's search path is the installer's `PATH` without the entries that hold a character outside letters, digits and `. _ / + @ : -`.
  7. `--remove` stops here. It runs `systemctl --user disable --now lgtm-client.service`, ignoring a unit that is not loaded, deletes the unit file and the program, runs `systemctl --user daemon-reload`, prints one sentence and exits 0. It keeps the state folder.
  8. It requires `go`, `docker`, `curl` and `jq` on `PATH`, and a missing one gives `install-client.sh needs <tool> on PATH.` This check comes after step 7, so a removal needs none of them (architect-r2 10, logic-r2 12).
  9. It runs `port web 8080` through Compose as the section above says. A failed command gives `docker compose could not read the stack; check that this user may run docker.` Output other than one line `127.0.0.1:<port>` gives `The stack's web service publishes no loopback port. Start it with deploy/up.sh.`
  10. It calls `SessionService/GetServer` at the address with `curl --silent --noproxy '*' --max-time 5`, as `get_server` in `deploy/test/checks.sh` does. A status other than 200, or a body without `product` equal to `lgtm-studio`, a positive `contract` and `sample` other than true, gives `The stack does not answer at <address>. Start it with deploy/up.sh.`
  11. A state folder with `machine.key` and no `identity.json` gives `A join into <folder> did not finish. Remove its waiting machine on the Machines page if one is shown, move <folder> aside, and run install-client.sh again.` (architect-r1 03).
  12. It builds the client with `CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=<version>" -o <temporary file> ./src/client/cmd/lgtm-client`, with the temporary file made by `mktemp` in `$HOME/.local/bin/`, and runs `<temporary file> version`. The version is `0.0.0-` and the first 12 hexadecimal digits of `git rev-parse HEAD`, with `-dirty` when `git status --porcelain` prints anything, and `0.0.0-dev` without Git. A failure gives `go build of lgtm-client failed.` A trap removes the temporary file on every exit.
  13. When the folder holds `identity.json`, it prints `lgtm-client is already joined.` Otherwise it runs `docker compose exec -T server lgtm-server join-token --hosts-server` through Compose as above, and keeps standard output in a variable. A failed command, or output that does not match `^lgtm1_[A-Za-z0-9_-]+$`, gives `The server's join-token command returned no token.` It then runs `LGTM_CLIENT_STATE_DIR=<state folder> <temporary file> join <address> <token>`, and clears the variable. A failed join passes on the client's output and prints `lgtm-client could not join the server.`
  14. It renames the temporary file to `$HOME/.local/bin/lgtm-client`, mode `0755`, and prints `built lgtm-client <version>`.
  15. It renders `deploy/lgtm-client.service` into a temporary file beside the unit's path and renames it into place. It runs `systemctl --user daemon-reload`, `enable lgtm-client.service` and `restart lgtm-client.service`. A failure of any gives `systemctl --user <verb> lgtm-client.service failed.`
  16. When `loginctl show-user <user> --property=Linger --value` does not print `yes`, it runs `loginctl enable-linger <user>`. A failure prints `loginctl enable-linger <user> failed, so the client starts only after this user logs in.` and sets the exit status to 1 for the end of the run.
  17. It waits up to 10 seconds for `systemctl --user is-active lgtm-client.service` to print `active`. A timeout gives `lgtm-client.service did not start; read journalctl --user -u lgtm-client.service.`
  18. It prints `lgtm-client runs as the user service lgtm-client.service; the Machines page shows whether it connects.` and exits with 0, or with the status of step 16.
- **The unit template** `deploy/lgtm-client.service` holds these lines and no other setting: `[Unit]`, `Description=LGTM Studio client`, `StartLimitIntervalSec=0`; `[Service]`, `Type=simple`, `ExecStart=@LGTM_CLIENT_BIN@ run`, `Environment=LGTM_CLIENT_STATE_DIR=@LGTM_CLIENT_STATE_DIR@`, `Environment=PATH=@LGTM_CLIENT_PATH@`, `Restart=on-failure`, `RestartSec=5`, `TimeoutStopSec=10`; `[Install]`, `WantedBy=default.target`.
- **The change of the user-flow script:** line 219 runs `join` with `LGTM_CLIENT_STATE_DIR=$1/lgtm-client` in place of `XDG_STATE_HOME=$1`. Line 233 runs `run` with `LGTM_CLIENT_STATE_DIR=$state_home/lgtm-client` in place of `XDG_STATE_HOME=$state_home`. The README's row "Client state folder" says the same. C10 keeps its path `$state_home/lgtm-client/identity.json`. The other names that C5 to C11 read belong to the plans that write them, and the sprint gate names them.
- **Tests written first:** `deploy/test/installer.sh` prints one TAP line per test, `ok <n> - <test id>` or `not ok <n> - <test id> <reason>`, and exits 0 only when all pass.
  - **The stand-ins.** Each test makes a temporary folder with `home/` as `HOME`, `tmp/` as `TMPDIR`, `shims/` first on `PATH`, and `calls.log`, and unsets `LGTM_CLIENT_STATE_DIR` and `COMPOSE_FILE` unless a test sets them. The shims are `go`, `docker`, `systemctl`, `loginctl`, `launchctl`, `uname`, `id`, `curl` and `jq`, where `jq` passes through to the real program. Each appends its name and arguments to `calls.log`. Variables of the test steer them: `SHIM_UNAME`, `SHIM_UID`, `SHIM_WEB_PORT`, `SHIM_DOCKER_FAIL`, `SHIM_CURL_BODY`, `SHIM_CURL_FAIL`, `SHIM_GO_FAIL`, `SHIM_TOKEN`, `SHIM_JOIN_EXIT`, `SHIM_LINGER_FAIL` and `SHIM_NO_USER_MANAGER`.
  - The `id` shim answers `-u` with `SHIM_UID` and passes other arguments to the real `id`. The `go` shim, called with `build ... -o <file>`, writes a stand-in client to `<file>`, or exits 1 when `SHIM_GO_FAIL` is set. The stand-in client answers `version` with `0.0.0-test`. It answers `join <address> <token>` by appending `client join <address> <SHA-256 of the token> <LGTM_CLIENT_STATE_DIR> installed=<present or absent>` to `calls.log`, where the last field says whether `$HOME/.local/bin/lgtm-client` or the unit file exists at that moment (architect-r2 09, logic-r2 09). It then creates `identity.json` and `machine.key` in that folder. When `SHIM_JOIN_EXIT` is not 0, it creates `machine.key` only, prints `{"msg":"join_refused","code":"token_used"}` and exits with that status.
  - The `docker` shim answers `compose ... port web 8080` with `127.0.0.1:8787` when `SHIM_WEB_PORT` is unset, with `127.0.0.1:<value>` when it is not empty, and with no output when it is empty (logic-r2 11). With `SHIM_DOCKER_FAIL` set, it exits 1 for every call. It answers `compose ... exec` with `SHIM_TOKEN`, whose default is a fixed `lgtm1_` text of 60 characters. The `curl` shim prints `SHIM_CURL_BODY`, whose default is `{"product": "lgtm-studio", "contract": 1}`, followed by status 200, unless `SHIM_CURL_FAIL` is set.
  - A stub `deploy/install-client.sh` that exits 0 and does nothing fails every positive test on its first assertion. Each test that asserts something is absent first asserts the line that proves its run reached that step.

| Test | What it asserts |
|---|---|
| `installs_linux_user_service` | With `SHIM_UNAME=Linux`, the exit code is 0. `$HOME/.local/bin/lgtm-client` exists with mode `0755`. The unit file equals the template with its three placeholders replaced by the program's path, `$HOME/.local/state/lgtm-client` and the test's search path. Read on its own, it holds `Restart=on-failure`, `RestartSec=5` and `StartLimitIntervalSec=0` (logic-r2 09). `calls.log` holds `systemctl --user daemon-reload`, `enable lgtm-client.service` and `restart lgtm-client.service`, in that order, and `loginctl enable-linger` with the user's name. The `go` line holds `-X main.version=0.0.0-` and `./src/client/cmd/lgtm-client`. |
| `asks_server_for_hosts_server_token` | `calls.log` holds `docker compose -f <repository>/deploy/compose.yaml port web 8080` before exactly one `docker compose -f <repository>/deploy/compose.yaml exec -T server lgtm-server join-token --hosts-server`. With `COMPOSE_FILE` set, both lines have no `-f`. The `client join` line has the address `http://127.0.0.1:8787`, and with `SHIM_WEB_PORT=8899` the address `http://127.0.0.1:8899`. With `SHIM_WEB_PORT` empty, the run exits 1, prints `publishes no loopback port`, and holds no `go` line. |
| `joins_before_service_starts` | The `client join` line ends with `installed=absent` and comes before the first `systemctl --user` line that names the unit. Its hash equals the SHA-256 of `SHIM_TOKEN`. Its folder equals the value of `LGTM_CLIENT_STATE_DIR` in the unit file. With `LGTM_CLIENT_STATE_DIR` set to an absolute folder of the test, the join line and the unit both name it. With it set to `relative/dir`, the exit code is 1 and `calls.log` holds no `go` line. |
| `token_never_printed` | `calls.log` holds the `client join` line with the token's hash. The installer's standard output and standard error do not hold `SHIM_TOKEN`, and no file under the test's folder holds it. |
| `join_failure_installs_no_service` | With `SHIM_JOIN_EXIT=1`, the exit code is 1 and the output holds `join_refused`. Neither the program nor the unit file exists, and `calls.log` holds no `systemctl --user enable` line. A second run exits 1 and prints `did not finish`. |
| `refuses_root` | With `SHIM_UID=0`, the exit code is 1, the output holds `not as root`, and `calls.log` holds no `go` line. |
| `refuses_missing_tool` | With a search path that has no `go`, the exit code is 1 and the output holds `needs go`. |
| `refuses_docker_failure` | With `SHIM_DOCKER_FAIL=1`, the exit code is 1, the output holds `may run docker`, and `calls.log` holds no `go` line. |
| `refuses_stack_not_answering` | With `SHIM_CURL_FAIL=1`, and again with `SHIM_CURL_BODY` holding `"sample": true`, the exit code is 1, the output holds `deploy/up.sh`, and `calls.log` holds no `exec` line and no `go` line. |
| `refuses_malformed_token` | With `SHIM_TOKEN=oops`, the exit code is 1, `calls.log` holds no `client join` line, and neither the program nor the unit file exists. |
| `refuses_missing_user_manager` | With `SHIM_NO_USER_MANAGER=1`, the exit code is 1, and `calls.log` holds no `go` line. |
| `refuses_unsafe_home` | With `HOME` set to a folder whose name holds a space, and again with `HOME` empty, the exit code is 1, and `calls.log` holds no `go` line. |
| `refuses_unfinished_join` | With a state folder that holds `machine.key` and no `identity.json`, the exit code is 1, the output holds `did not finish`, and `calls.log` holds no `exec` line. |
| `build_failure_stops_installer` | With `SHIM_GO_FAIL=1`, the exit code is 1, the output holds `go build`, `calls.log` holds no `exec` line, and `$HOME/.local/bin/` holds no file. |
| `unsafe_path_entries_dropped` | With a search path that holds an entry with `%` and an entry with a space, the unit's `PATH` line holds neither entry and keeps the others in their order. |
| `linger_failure_is_reported` | With `SHIM_LINGER_FAIL=1`, the unit file exists, the `restart` line is in `calls.log`, the output names `loginctl enable-linger`, and the exit code is 1. |
| `rerun_keeps_joined_identity` | After a first run, a second run exits 0. Its `calls.log` holds a `port` line, no `exec` line and no `client join` line, and it holds `daemon-reload` and `restart`. `identity.json` and `machine.key` are unchanged byte for byte. |
| `rerun_uses_state_folder_of_installed_unit` | After a first run with `LGTM_CLIENT_STATE_DIR` set to an absolute folder of the test, a second run without it exits 0, joins nothing, and its unit still names that folder. |
| `remove_stops_and_deletes_service` | After a first run, `--remove` exits 0. `calls.log` holds `systemctl --user disable --now lgtm-client.service`. The unit file and the program are gone. The state folder and its files are unchanged. |
| `remove_needs_no_build_tools` | After a first run, `--remove` with a search path that has no `go`, `docker`, `curl` or `jq` exits 0 and removes the unit file and the program. |
| `usage_on_unknown_argument` | `--frobnicate` and `--rejoin` each give exit 2 and an empty `calls.log`. |

- **Acceptance:** `bash deploy/test/installer.sh` prints `ok` for the 21 tests above and exits 0, with no Docker, network or service manager on the test machine. Criterion 2e holds.

**Work item 3. The installer on macOS**

- **Files:** changes `deploy/install-client.sh` and `deploy/test/installer.sh`. Creates `deploy/dev.lgtmstudio.client.plist`.
- **Needs first:** item 2, because both change the same two files.
- **Work area, complexity and scope:** `infra`, medium, `stack/infra`. It is a second branch of a known script, and no machine of the checks runs macOS.
- **Design statements:** D16, D19, R9. The section "The client in wave 1".
- **Risky:** no. It changes no step that touches the token, which item 2's tests keep covering on both systems.
- **Behaviour:**
  - With `uname -s` equal to `Darwin`, steps 4 and 16 of item 2 are skipped, and steps 6, 7, 15 and 17 use launchd. Step 6 reads the folder from the agent's `EnvironmentVariables` when the agent file exists.
  - Before any change, it requires `launchctl print gui/<uid>` to succeed, and otherwise gives `macOS starts lgtm-client at login; run install-client.sh from a logged-in session.`
  - The template is a property list with these keys: `Label` `dev.lgtmstudio.client`; `ProgramArguments` with the program and `run`; `EnvironmentVariables` with `LGTM_CLIENT_STATE_DIR` and `PATH`; `RunAtLoad` true; `KeepAlive` as a dictionary with `SuccessfulExit` false; `ThrottleInterval` 5; `StandardOutPath` and `StandardErrorPath` both `@LGTM_CLIENT_LOG@`; `ProcessType` `Background`.
  - Installing renders the agent file and renames it into place, then runs `launchctl bootout gui/<uid>/dev.lgtmstudio.client`, ignoring an agent that is not loaded, `launchctl bootstrap gui/<uid> <agent file>` and `launchctl enable gui/<uid>/dev.lgtmstudio.client`. The wait of step 17 reads `launchctl print gui/<uid>/dev.lgtmstudio.client` for `state = running`.
  - `--remove` runs `launchctl bootout`, then deletes the agent file and the program. A failed `bootout` of a loaded agent gives exit 1.
- **Tests written first,** added to `deploy/test/installer.sh` with `SHIM_UNAME=Darwin`:

| Test | What it asserts |
|---|---|
| `installs_macos_launch_agent` | The exit code is 0. The agent file equals the template with its four placeholders replaced. `calls.log` holds `launchctl print gui/<uid>` first, then `launchctl bootout`, `launchctl bootstrap gui/<uid> <agent file>` and `launchctl enable gui/<uid>/dev.lgtmstudio.client`, in that order, after the `client join` line. It holds no `systemctl` and no `loginctl` line. |
| `macos_agent_parses` | The rendered agent file parses with `plutil -lint` when that program exists, and otherwise with the `plistlib` module of Python 3. The parsed `KeepAlive` is a dictionary whose `SuccessfulExit` is false, `ThrottleInterval` is 5, and `ProgramArguments` ends with `run`. When neither parser exists, the line is `ok <n> - macos_agent_parses # SKIP no parser`. |
| `macos_remove_boots_out_agent` | After a first run, `--remove` exits 0, `calls.log` holds `launchctl bootout gui/<uid>/dev.lgtmstudio.client`, and the agent file and the program are gone. |
| `refuses_unsupported_system` | With `SHIM_UNAME=FreeBSD`, the exit code is 1, the output holds `Linux and macOS only`, and `calls.log` holds no `go` line. |

- **Acceptance:** `bash deploy/test/installer.sh` prints `ok` for these four tests and the 21 of item 2. The steps on a Mac belong to the sprint gate.

**Work item 4. The live test of the installer and its guide**

- **Files:** creates `deploy/test/installer-live.sh` (executable). Changes `deploy/README.md` with a section named "Installing the client". The test writer creates the test file, and the builder writes the section.
- **Needs first:** items 1 and 2. Plan `go-s3-client-link`, item 9. Plan `go-s3-server-machines`, items 12 to 14.
- **Work area, complexity and scope:** `infra`, medium, `stack/infra`. It drives the real stack and the real service manager, and one of its parts needs a restart of the computer.
- **Design statements:** D16, D19, C5, C6, C7, C9. The epic's sentence that the stack, the server and the client each come back by themselves after a restart.
- **Risky:** no. It adds no code that handles a token.
- **How the test runs** (architect-r2 05, logic-r2 05, threat-r2 07):
  - It runs on a disposable Linux user, against a stack started by `deploy/up.sh` with a password made for the test. It needs `LGTM_WEB_PASSWORD`, a systemd user manager, Docker, and `systemctl is-enabled docker.service` printing `enabled`.
  - The default part needs `LGTM_INSTALL_LIVE=1`. It refuses to start when the unit or the program exists, or when its fixture folder `$HOME/.local/state/lgtm-installer-live` exists. It creates that folder with mode `0700` and runs the installer with `LGTM_CLIENT_STATE_DIR=<fixture folder>/state`, which did not exist before, so the installer can only join afresh.
  - It reads the address the way the installer does. It signs in and reads `ClientService.WatchClients` the way `deploy/test/checks.sh` does in `signin`, `open_watch` and `read_frame`, except that the password reaches `jq` on standard input and every request body and cookie jar sits in the fixture folder. It signs out at the end of each part.
  - The default part writes the record `<fixture folder>/fixture`, mode `0600`, with the boot identifier from `/proc/sys/kernel/random/boot_id` and the machine identifier that `WatchClients` shows for the new waiting machine. `--after-reboot` and `--cleanup` refuse to run unless the record exists and the installed unit names `<fixture folder>/state`. `--cleanup` removes the recorded machine with `ClientService/RemoveClient`, runs its one test, runs `deploy/install-client.sh --remove`, and removes the fixture folder last.
- **Tests written first:** each prints one TAP line. Before items 1 and 2 exist, the first test fails on the installer's exit code.

| Test | What it asserts |
|---|---|
| `live_install_leaves_machine_waiting` | `deploy/install-client.sh` exits 0. The first `WatchClients` message after it lists exactly one client that was absent before, waiting for approval with `hostsServer` true. |
| `live_service_enabled_with_linger` | `systemctl --user is-enabled lgtm-client.service` prints `enabled`, `is-active` prints `active`, and `loginctl show-user <user> --property=Linger --value` prints `yes`. |
| `live_service_restarts_after_crash` | After `systemctl --user kill --signal=KILL lgtm-client.service`, the unit is active again within 15 seconds with another main process identifier (`MainPID`). |
| `live_approved_machine_online` | After `ClientService/ApproveClient` for the recorded machine, a `WatchClients` stream shows it online with `hostsServer` true within 35 seconds. |
| `live_reconnect_restarts_service` | After `ClientService/ReconnectClient` for the recorded machine, `NRestarts` grows by exactly one within 35 seconds, the user journal of the unit since the call holds `status=75` for the ended run, the unit has another `MainPID`, and a new stream shows the machine online with a later `connectedAt`. |
| `live_rerun_keeps_machine` | A second `deploy/install-client.sh`, run without `LGTM_CLIENT_STATE_DIR`, exits 0. The count of clients is unchanged, the recorded machine is online again within 35 seconds, and the SHA-256 of `identity.json` is unchanged. |
| `live_online_before_login` | With `--after-reboot`: the boot identifier differs from the recorded one, and at least one session of the user has `Class` equal to `user`. A `WatchClients` stream shows the recorded machine online, and its `connectedAt` is earlier than the `Timestamp` of every such session, read with `loginctl show-session`. Both are times of the same host's clock, so the machine had an authenticated connection to the server before the first login (architect-r2 07, threat-r2 06). **UNREVIEWED MECHANISM:** the server's `connectedAt` replaces the unit's start time, because only the server can see an authenticated connection. A restart of the client after the login fails the test, and the person runs it again after the next restart. |
| `live_removed_machine_stays_stopped` | With `--cleanup`, after `RemoveClient`: within 35 seconds the unit is `inactive` and the journal holds a clean exit of its last run. Fifteen seconds later `NRestarts` is unchanged and the unit is still `inactive`. |

- **The guide:** the section "Installing the client" of `deploy/README.md` names the command and `--remove`, the paths of the program, the unit or agent and the state folder, and how to read the client's output on each system. It says that one user of the computer runs the installer, once at a time. It says that a Mac starts the client at login, and that a Linux restart brings the client back only when the user lingers and Docker starts at boot. It says that a removed machine, a changed port or an unfinished join needs `--remove`, the state folder moved aside by hand, and a new install. It says that whoever can run a command in the server's container can make a join token, and that the token is visible in the list of processes while the join runs.
- **Acceptance:** the item's own gate is a run of `deploy/test/installer-live.sh` without `LGTM_INSTALL_LIVE`, which prints `1..8`, exits 2, and changes nothing. The live parts belong to the sprint gate.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The service runs after a login only, because lingering is off. | High | The installer turns lingering on and fails loudly when it cannot. `live_online_before_login` proves a connection before the first login of a new boot. |
| The service restarts without end after the owner removed the machine. | Medium | A removal ends `run` with status 0, and the service leaves an exit 0 alone. `live_removed_machine_stays_stopped`. |
| The installed client dials an address that disagrees with its token. | Medium | The address and `LGTM_PUBLIC_URL` both come from the published port, read through the launcher's Compose rule. `asks_server_for_hosts_server_token`. |
| The service does not find a harness that the user's shell finds. | Medium | The installer copies its own search path into the service. `unsafe_path_entries_dropped`. A harness installed later in a new folder needs a second run, which the guide says. |
| The token is readable by every local user in the list of processes for the length of the join (logic2-r1 04). | Medium | The token works once and expires in 15 minutes, and the owner's approval still gates the machine. Whether the client also reads the token from standard input is for the owner. |

**Verify:** `bash deploy/test/installer.sh` on any Linux machine, with no Docker, and the sprint gate below. **Rollout detail:** steps 1, 3 and 4 of the rollout order.

**The sprint gate**

These runs need the merged programs of several teams, so they are joint gates of the sprint. Before them, the Stack team rebuilds the server image and recreates the stack with `deploy/up.sh`, with a password made for the gate. They run on a Linux runner with Docker and the loopback network, by issue 1640.

| # | Run | Expected result |
|---|---|---|
| G1 | `docker compose -f deploy/compose.yaml exec -T server lgtm-server join-token --hosts-server`, with the output captured into a private file and only its shape reported. | Exactly one line matching `^lgtm1_[A-Za-z0-9_-]+$`, empty standard error, exit 0. |
| G2 | `LGTM_INSTALL_LIVE=1 deploy/test/installer-live.sh`, then a restart of the computer by a person, then `--after-reboot` from the first login, then `--cleanup`. | Six, one and one `ok` lines. |
| G3 | On a Mac with the stack running: `deploy/install-client.sh`, then `launchctl print gui/$(id -u)/dev.lgtmstudio.client`. Approve the machine and press its restart on the Machines page. Log out and log in. Remove the machine. | `state = running` after the install. `last exit code = 75` and `state = running` after the restart, and again `state = running` after the login. After the removal, the agent is not running and its last exit code is 0. |
| G4 | `deploy/test/with-postgres.sh -- deploy/test/checks.sh`, so C12 has its disposable database (logic-r2 10). Its prerequisites: `psql_in_stack` of plan `go-s1-stack-vault`, item 2; the table of plan `go-s3-server-machines`, item 3; the log names of plans `go-s3-client-link` and `go-s3-server-machines` that C5, C10 and C11 read; and the change of the script's `compose`, `configure_web`, `signin` and `signin_rejected` that the result file of round 1 requests, which the orchestrator assigns to the script's test writer (threat-r2 04, threat-r2 09). | 13 lines that start with `ok`, and exit 0. |
| G5 | On an open Machines page, make a token, copy the command the page shows, and paste it in a terminal of another folder. Its prerequisite is the page's change to `lgtm-client join <address> <token>`, which the orchestrator assigns to a team that may edit `src/frontend/` (logic-r2 06, threat-r2 05). | The command joins, and the page shows the new waiting machine without a reload. |

**Post-build checks**

| # | Scenario | Expected result |
|---|---|---|
| PB1 | Kill the installer after the server committed its join and before the client wrote `identity.json`, then run it again. | The second run refuses with `did not finish`, and the Machines page shows one waiting machine for the owner to remove. |
| PB2 | Run the installer with tracing turned on in the caller, `bash -x deploy/install-client.sh`, and read `ps -eo args` as another user during the join. | The trace holds no token. The process list shows the token only as the last argument of `lgtm-client join`, as the guide states. |
| PB3 | Make the parent of `$HOME/.local/bin` or of the unit's folder writable by another local user, and run the installer (threat-r2 03). | The result is recorded for `docs/issues.md`. Wave 1 installs into the user's own folders and checks no ancestor. |

**Outside wave 1**

These cases are named here, and the result file asks for issues (decision 46):

- Joining again from the installer, after a removal, a changed port, a restore or another stack on the same port. The owner removes the client, moves the state folder aside and installs again.
- Two runs of the installer at the same time by one user, which can join twice and race on the service file.
- A second user of the same computer who runs the installer and gets a second machine with the mark, because D19 makes one user per machine a convenience (architect-r1 06).
- A check of the owners and modes of the folders the installer writes into.
- A restore of a state folder and two live copies of one state, by issues 1641 and 1642.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe: the brief's work items, observations, agreements and points to settle; the task files of the plan, the reconcile and the folds of rounds 1 and 2; the requests to this plan from the contract's rounds 3 and 4, the client's round 4 and the fold of round 2 of `go-s2-stack-web`; the findings in `attack/`; issues 1636 to 1642; the names table of `deploy/test/README.md`; and the design statements D7, D16, D17, D19, R3, R8, R9, R10 and C1 to C13.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the server's command that makes a token | WS-1 | Item 1. |
| Brief item 2, the installer and its service, and point 1 on macOS | WS-2 | Items 2 and 3. Decisions 12 and 13. |
| Brief item 3, C5 to C11 turned on | WS-2 | Plan `go-s2-stack-web`, item 7, proves the link. Item 2 changes the script's state folder, and gate G4 runs the script. Decisions 31 and 35. |
| Brief observations, agreements and point 2; issue 1638 | WS-1, WS-2 | `live_install_leaves_machine_waiting`, `live_online_before_login`, gate G4, `--hosts-server`, and decisions 8, 10, 11, 12 and 14. |
| Contract round 3: the address, a failing join, the restart by status, and its moved checks | WS-2 | The section "The service, the address and the exit statuses", and gates G2 and G3. Decisions 33 and 34. |
| Contract round 4: refusals in the error's metadata | WS-1 | `TestJoinTokenJoinsOnce`. |
| Client round 4: `identity.json`, one state setting, the script's lines | WS-2 | Item 2, steps 6, 11 and 13, and the change of the script. Decisions 32 and 35. |
| Stack web round 2, request P1, and round 2 of this plan: the launcher's Compose rule | WS-2 | The section above. Decision 48. |
| Findings of rounds 1 and 2 | WS-1, WS-2 | `attack/round-1-findings.md` and `attack/round-2-findings.md`. Decisions 40 to 50. |
| Issues 1636 to 1642 | WS-2 | Gate G5 depends on issue 1636. The sprint gate and item 1's database need Docker, by issue 1640. Issues 1637, 1639, 1641 and 1642 are outside this plan or wave 1. |
| D7, D17, R3, R10 | WS-1 | Item 1 calls `Tokens.Create` against a real PostgreSQL, and the token carries the server key's fingerprint. |
| D16, D19 | WS-2 | `live_reconnect_restarts_service`, `live_online_before_login`, `refuses_root`, decision 27 and the section "Outside wave 1". |
| R8, R9 | WS-1, WS-2 | One fixed sentence per refusal. `TestJoinTokenPrintsOneTokenLine`, `TestJoinTokenRefusesUnknownArgument` and `token_never_printed`. |
| C1 to C13 | WS-1, WS-2 | Gate G4 runs them. C5, C6 and C10 also rest on item 1 and criterion 2e. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "What this project is": the three parts meet only through the contract. | The installer and the command use the contract's `Join`, `GetServer` and `WatchClients`. The installer reads of the client's files only whether `machine.key` and `identity.json` exist. |
| `AGENTS.md`, "Where things are", and the epic's table of teams. | Items 2 to 4 edit only `deploy/`. Item 1's three files are the Server team's work, done by its own seats. Decision 8. |
| The epic, a module is added with its first import. | The plan adds no module. |
| `AGENTS.md`, habits 1 and 2, and "Writing style for documents". | Nothing is copied. The result file lists the problems for `docs/issues.md`. Full sentences and no private detail, in the plan and the guide. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet at launch. This section names none.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on | Can run alongside | Design statements |
|---|---|---|---|---|---|---|---|---|
| 1. The server's token command | `backend` | `server/backend` | medium | One small command over other items' interfaces, but it makes a join token. | no | `go-s2-server-signin` items 1, 8 and 9; `go-s3-server-machines` items 1, 3, 4 and 14 | 2 | D7, D17, R3, R8, R9, R10, C5, C6 |
| 2. The installer on Linux | `infra` | `stack/infra` | medium | One script of known steps with a few failure paths, and it passes a join token. | no | none to build; names from item 1 and `go-s3-client-link` | 1 | D7, D16, D19, R8, R9, C6 |
| 3. The installer on macOS | `infra` | `stack/infra` | medium | A second branch of a known script, verified on Linux and by a person on a Mac. | no | 2 | 1 | D16, D19, R9 |
| 4. The live test and the guide | `infra` | `stack/infra` | medium | It drives the real stack and service manager, and one part needs a restart. | no | 1, 2; `go-s3-client-link` item 9; `go-s3-server-machines` items 12 to 14 | 3 | D16, D19, C5, C6, C7, C9 |

Load: four test writers, four builders, and two mutation runs for the risky items 1 and 2, which gives 10 execution seats, five for each primary vendor. Item 1's seats are Server seats and need Docker for the disposable database. The sprint gate needs a seat with Docker and the loopback network, by issue 1640, and a person for the restart and the Mac. The orchestrator checks the weekly limits at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| Round 1 | 5, 37, 12 as reported in 54 findings; 2, 13, 5 in 20 groups after duplicates. | Folded on 2026-11-26. The register is `attack/round-1-findings.md`. |
| Round 2 | 0, 25, 7 as reported in 32 findings; 0, 11, 4 in 15 groups after duplicates. | Folded on 2026-11-26 by removing `--rejoin`, the lock, the folder check and the rerun's health check. The register is `attack/round-2-findings.md`. |

</details>
