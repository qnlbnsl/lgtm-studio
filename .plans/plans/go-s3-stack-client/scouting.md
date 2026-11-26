# Sources and checks for the client installer

## What exists at the time of planning

The scan ran on 2026-11-26 at commit `040b6ce0` of `master`.
No Go code, no `go.mod`, and no file of `deploy/` besides `deploy/test/` existed in the checkout.
Every file that this plan creates is therefore a target, named with its nearest pattern below.

## The statements of the design, with their lines

The design is `docs/specs/2026-11-25-go-rewrite-foundation-design.md`.
Its decisions table starts at line 26, so D1 is line 26 and D23 is line 48.

| Statement | Line | What this plan takes from it |
|---|---|---|
| D7 | 32 | A machine proves itself with a key pair, and the join token only authorizes the exchange of keys. |
| D16 | 41 | `ReconnectClient` restarts a connected client. A client retries at least every 15 seconds and starts no web server. |
| D17 | 42 | No certificate needs a public authority. Trust comes from the join token and from the owner's approval. |
| D19 | 44 | One user on a machine is a convenience for now. |
| The server's own command | 247 | "The owner makes a join token on the Machines page, or with the server's own command." |
| Encryption on the link | 318 | A machine reaches a loopback address over plain HTTP/2 (Hypertext Transfer Protocol version 2). |
| The state folder | 334 | The client keeps its state in `~/.local/state/lgtm-client/`. |
| The service of the user | 335 | The client runs as a service of its user, outside the compose stack. |
| One address | 352 | The website and the server share one address. |
| The loopback port | 353 | The stack publishes its port on the loopback address only, by default. |
| The installer | 356 | `deploy/install-client.sh` installs the client on the same machine and joins it with a token the server's own command makes. |
| R3, R8, R9, R10 | 364, 369, 370, 371 | Another part's interface, one sentence for an error, no secret in a log, real PostgreSQL. |
| C1 to C13 | 378 to 390 | The checks of wave 1. |

## Files read, with anchors

| File and anchor | What it establishes |
|---|---|
| `.foundry/go-rewrite/tasks/plan-go-s3-stack-client.md` | The assignment, including the three specific points: the script exists, issue 1638, and the service after a restart. |
| `.foundry/go-rewrite/common-planner.md` | The shared rules: only this folder and the result file are written, and no model or harness is named. |
| `.plans/epics/go-rewrite/epic.md`, "The teams, and what each may edit" | The Stack team edits `deploy/` and reads `src/server/cmd/` and `src/client/cmd/`. |
| `.plans/plans/go-s3-stack-client/plan.md` as a brief, in Git at commit `040b6ce0` | Three work items, two points to settle, and the grant of one file in `src/server/cmd/lgtm-server/`. |
| `.plans/plans/go-s3-server-machines/plan.md`, a brief | Items 2 and 4 make tokens and `Join`. Its first point to settle is issue 1633. |
| `.plans/plans/go-s3-client-link/plan.md`, a brief | Items 1 and 6 make `join` and `run`. Its point 3 asks what the restart command does when the client is not a service. |
| `.plans/plans/go-s2-stack-web/plan.md`, a brief | Item 2 names `deploy/Caddyfile` and says that streams pass through it. Its agreement on long streams covers the link of sprint 3. |
| `.plans/plans/go-s2-server-signin/plan.md`, a brief | Item 1 names `src/server/cmd/lgtm-server/main.go` and `src/server/internal/config/config.go`. Item 8 is the vault client and the signing key. |
| `.plans/plans/go-s0-contract/plan.md`, "Public names that other teams build against" and item 5 | The token's text is `lgtm1_` and the unpadded base64url of `JoinToken`. `JoinToken` has `token_id`, `secret`, `address`, `server_key_sha256` and `certificate_key_sha256`. `JoinRequest` has `token_id`, `token_secret`, `machine_public_key` and `host_name`. A refused token gets `permission_denied`. A client refuses an address that is neither `https` nor loopback over `http`. |
| `.plans/plans/go-s0-contract/attack/round-1-architect.md`, the finding on `Client.address` and `hosts_server` | A reviewer proposed that the server's own token marks the machine of the server. Decision 10 adopts that. |
| `.plans/plans/go-s1-client-local/plan.md`, "The layout of the state folder" and item 1, at the time of planning | The folder was `LGTM_CLIENT_STATE_DIR`, then `$XDG_STATE_HOME/lgtm-client`, then `$HOME/.local/state/lgtm-client`. It holds `machine.key` and `server.json`. One client holds a lock on the folder. `main.version` is set with `-ldflags "-X main.version=<v>"`. |
| `.plans/plans/go-s1-client-local/plan.md`, item 4, `DefaultScanOptions` | The harness scan reads `PATH` from the environment, which is why decision 16 copies it into the service. |
| `.plans/plans/go-s1-stack-vault/plan.md`, items 2, 6 and "Contract for sprint 2" | The Compose project is `lgtm`, the state folder is `deploy/.state` or `LGTM_STACK_STATE_DIR`, and the launcher is `deploy/up.sh`. The databases and their owners are `lgtm_app` and `lgtm_bao`, the administrator is `postgres`, and every connection needs a password, also on the socket. The server container runs as user 10001. |
| `.plans/plans/go-s1-server-store/plan.md`, rollout step 3 | The Stack team owns `deploy/test/mutate-feed-lock.sh` for C12 in sprint 1. It is the only hook file of the user-flow script. |
| `deploy/test/checks.sh`, `compose` at line 22, `configure_web` at line 32, `get_server` at line 60, `signin` at line 82, `machines` at line 196, `client_join` at line 217, `start_client` at line 232, `open_watch` at line 238, `read_frame` at line 251, `psql_in_stack` at line 324, `check_c2` at line 369, `check_c5` at line 425, `check_c10` at line 505, `run_check` at line 658 | The checks build their own client, pass `XDG_STATE_HOME`, join through the web address, and read `WatchClients`. None of them runs the installer. `psql_in_stack` runs `psql` as `lgtm` on the socket without a password. C10 edits `identity.json`. |
| `deploy/test/README.md`, the table "Names and observations the checks expect", lines 63 to 76 | The provisional names. The server's token command is called `lgtm-server join-token` there. |
| `src/contracts/proto/lgtm/v1/clients.proto`, lines 16 to 17 and 38 to 39 | `CreateClientToken`'s comment names a command `lgtm client add`. `Client.hosts_server` is field 4. |
| `src/frontend/src/screens/service.tsx`, line 410 | The page shows the join command as `lgtm client join <address> --token <code>`, which is issue 1636. |
| `src/frontend/src/screens/service.tsx`, line 353, and `src/frontend/src/server/sync.ts`, line 18 | The page maps `hostsServer` to the role of the server's machine. It blocks removal of that machine only with sample data. |
| `docs/issues.md`, issues 1632 to 1640 | 1633 is where encryption ends. 1634 is the entry points for tests. 1638 is the source of `hosts_server`. 1639 is a replaced key. 1640 is a sandbox without Docker. |
| `docs/specs/2026-11-05-build-loop-design.md`, "4 Build" and "Block: Build subitems" | Tests first, frozen, failing on their assertion. A risky item gets a mutation on a temporary branch. |
| `floorplan/plan.json`, entries `deploy/install-client.sh`, `deploy/Caddyfile` and `src/server/cmd/lgtm-server/main.go` | The planned installer and web server rules. The blueprint names no service file and no token command file. |
| `scripts/check-private.mjs`, `check` | It reads tracked files only, and it flags home paths, scratch paths and private addresses. |

## Targets and their nearest patterns

| Target path | Nearest pattern |
|---|---|
| `src/server/cmd/lgtm-server/join_token.go`, `joinTokenCommand` | `src/server/cmd/lgtm-server/main.go` of plan `go-s2-server-signin`, item 1, for the start of the server. `src/client/cmd/lgtm-client/main.go`, `run(args, stdout, stderr) int`, of plan `go-s1-client-local`, item 1, for a command that returns its exit code. |
| `src/server/cmd/lgtm-server/join_token_test.go` | `src/contracts/join_test.go`, `TestJoinTokenExampleDecodes`, of plan `go-s0-contract`, item 5, for decoding a token's text. |
| `deploy/install-client.sh`, `main` and `install_launch_agent` | `deploy/test/checks.sh`, `compose`, `configure_web` and `get_server`, for the stack's address and calls. `deploy/up.sh` of plan `go-s1-stack-vault`, item 6, for a launcher in `deploy/`. |
| `deploy/lgtm-client.service`, section `[Service]` | The systemd manuals `systemd.unit(5)` and `systemd.service(5)`, read on the development machine with systemd 257. `Restart=on-failure` restarts after an exit code that is not zero and after a signal. `StartLimitIntervalSec=` belongs in `[Unit]`, and 0 turns the limit off. `loginctl enable-linger` starts a user's manager at boot. |
| `deploy/dev.lgtmstudio.client.plist` | The launchd manual `launchd.plist(5)`: `KeepAlive` as a dictionary with `SuccessfulExit` false restarts a job after an exit that is not zero. `launchctl bootstrap gui/<uid> <file>` and `launchctl bootout gui/<uid>/<label>` load and unload an agent of the user's session. Read from documentation, not run, because no Mac was available. |
| `deploy/test/installer.sh`, `run_test` | `deploy/test/checks.sh`, `run_check`, for one TAP (Test Anything Protocol) line per test. |
| `deploy/test/installer-live.sh`, `after_reboot` | `deploy/test/checks.sh`, `signin`, `open_watch` and `read_frame`, for a session and the first `WatchClients` message. |
| `deploy/README.md`, section "Installing the client" | `deploy/README.md` of plan `go-s1-stack-vault`, item 6. |

## A two-way stream through Caddy on 2026-11-26

This trial backed the link check of the first version. Decision 31 removed that check, because plan `go-s2-stack-web`, item 7, now tests the same route. The record stays as evidence.

This trial answers point 4 of issue 1633 for the loopback case.
It ran in a scratch folder outside the checkout, with Go 1.26.0 and the image `caddy:2.11.9-alpine`.
Nothing of it was kept.

The program served HTTP/1.1 and HTTP/2 without encryption with `http.Protocols` and `SetUnencryptedHTTP2(true)`.
Its handler for `POST /lgtm.link.v1.LinkService/Open` called `http.ResponseController.EnableFullDuplex`, answered 200, and echoed each line at once with a flush.
Its client sent one numbered line at a time over HTTP/2 without encryption and required each echo within 3 seconds.

The Caddyfile was this:

```text
{
	admin off
	servers {
		protocols h1 h2 h2c
	}
}
:28787 {
	bind 127.0.0.1
	handle /lgtm.link.v1.* {
		reverse_proxy h2c://127.0.0.1:29000 {
			flush_interval -1
		}
	}
	handle {
		respond "site" 200
	}
}
```

Caddy ran with `docker run --network host` and the file mounted read-only.

| Run | Result |
|---|---|
| The client straight to the program | `echoed 3 messages over 2 seconds`, over HTTP/2. |
| The client through Caddy, 10 messages 5 seconds apart, with one silence of 40 seconds | `echoed 10 messages over 80 seconds`. The program saw the request as `HTTP/2.0`. |
| The program stopped 5 seconds into a stream through Caddy | The client read `stream error: stream ID 1; INTERNAL_ERROR; received from peer` about one second after the stop, and exited 1. |
| A `GET /` with `curl --http2-prior-knowledge` through Caddy | HTTP/2 with status 200. |

A first attempt used port 18787 and gave a result that could not be trusted.
Another planner's probe already listened on that port, so the first Caddy container could not bind and stopped, and the client reached the other probe.
The runs above used ports 28787 and 29000, which nothing else held, and the container was checked as running before each run.
The kept test of work item 2 therefore runs under a Compose project of its own and picks a free port.

## The machine of the planning session on 2026-11-26

| Fact | Value |
|---|---|
| Docker | Engine 29.7.2, reachable from the session. |
| Go | 1.23.3 installed, with the toolchain 1.26.0 in the module cache. |
| systemd | 257. `systemctl --user` answered. Lingering was on for the session's user. |
| Sessions | `loginctl list-sessions` listed a session of class `manager` for the user's service manager, and sessions of class `user`. The session of class `manager` started before the user's services. Decision 24 leaves it out. |
| Unit properties | `systemctl --user show` gave `ActiveEnterTimestampMonotonic` for a unit, and `loginctl show-session` gave `TimestampMonotonic`. Both count microseconds from boot. |
| Tools | `bash` 5.2, `curl` 8.14 with HTTP/2 support, and `jq`. No `caddy` program on the host. |

## The database probe of check C5 on 2026-11-26

Plan `go-s1-stack-vault`, item 2, now owns `psql_in_stack`, so this probe is evidence for that plan (decision 35).

A disposable container of `postgres:16.16-alpine` ran with the account and database `lgtm_app` and with password authentication on both the network and the socket, as plan `go-s1-stack-vault` sets it.
The password sat in a mounted file, as that plan mounts it.

| Form | Result |
|---|---|
| `psql -U lgtm_app -d lgtm_app` on the socket, as `psql_in_stack` in `deploy/test/checks.sh` runs it | `fe_sendauth: no password supplied`. |
| The body proposed in the result file, which reads the mounted file and connects over loopback | The `UPDATE ... RETURNING id` of C5 printed the identifier and exited 0. |

## Why exit code 75

Code 75 is `EX_TEMPFAIL` in the list of exit codes of `sysexits.h`, which means "try again later".
It is not a code that the Go runtime uses for a crash, which is 2.

## Sources read for the reconcile of 2026-11-26

| File and anchor | What it establishes |
|---|---|
| `.foundry/go-rewrite/tasks/reconcile-go-s3-stack-client.md` | The requests to carry out, the limit of 700 lines, and the rule that a program's own rules sit in its plan. |
| `requests-from-contract-r3.md`, "Requests to other plans" | This plan fills in the address, treats a failing `join` as a failure, restarts `run` by its statuses, and takes over the contract's checks of exit statuses and of the installer. |
| `requests-from-contract-r4.md` | No name changed. `Lgtm-Join-Refusal` travels in the error's metadata. Nothing is addressed to this plan. |
| `requests-from-client-r4.md`, ruling 5 | `identity.json` in place of `server.json`, one setting `LGTM_CLIENT_STATE_DIR`, and the lines of `deploy/test/checks.sh`. |
| `.foundry/go-rewrite/results/fold-go-s2-stack-web-r2.md`, request P1 | The installer reads the port from Compose, as the Stack guide's "The port in other tools" says. |
| `.plans/epics/go-rewrite/epic.md`, lines 132 and 133 | A module is added in the same change as its first import. |
| `docs/issues.md`, issues 1641 and 1642 | A restore and two live copies of one state are outside wave 1. After a restore, the owner removes the machine and joins it again. |
| `.plans/plans/go-s0-contract/plan.md`, lines 66, 194, 350, 476 and 753 | The contract states facts of the wire only. Exit statuses, the installer's address and the checks of a service belong to the programs' plans. |
| `.plans/plans/go-s0-contract/attack/plan-v3.md`, post-build checks 12 and 14 | The text of the two checks that move to this plan. |
| `.plans/plans/go-s1-client-local/plan.md`, "The layout of the state folder" | `LGTM_CLIENT_STATE_DIR` when set and absolute, otherwise `$HOME/.local/state/lgtm-client`. The folder holds `identity.json`, written last in a join, and `machine.key`. |
| `.plans/plans/go-s3-client-link/plan.md`, item 9 | `run` exits 0 on cancellation or a stop for good, 75 for an accepted restart and 1 for a failure. `join` exits 0 or 1. A usage error exits 2. `TestRunCommandExitStatus` proves them. |
| `.plans/plans/go-s2-server-signin/plan.md`, items 1, 8 and 9 | `config.Load(lookup)`, `vault.Open`, `vault.LoadLinkIdentity` with `allowInitialize` false, the dispatcher `run(ctx, args, lookup, stdout, stderr) int` with a placeholder entry `join-token`, and `servertest.Server.Environment()`. |
| `.plans/plans/go-s1-server-store/plan.md`, line 126 | `store.Open(ctx, config, policy)`. |
| `.plans/plans/go-s3-server-machines/plan.md`, items 3, 12 and 14, and its request 12 | `machines.NewTokens`, `Tokens.Create` with `TokenOptions{HostsServer}` and the address always `PublicURL`, `ErrNoCertificatePin`, `client_join_tokens`, `CreateClientToken` without the mark, and the helpers `Clients(t)`, `Link()` and `Environment()` of `servertest`. This row was corrected in the fold of round 2. |
| `.plans/plans/go-s2-stack-web/plan.md`, items 1, 4, 5 and 7 | The image's `/usr/local/bin/lgtm-server` as user 10001, `LGTM_PUBLIC_URL` equal to `http://127.0.0.1:${LGTM_WEB_PORT:-8787}`, the launcher's use of `COMPOSE_FILE`, and the stream test of the link's route. |
| `deploy/compose.yaml`, line 1 | The project is named `lgtm`, so `docker compose port web 8080` finds the stack without `-p`. |
| `deploy/test/checks.sh`, lines 219 and 233, and `deploy/test/README.md`, the row "Client state folder" at line 81 | The two lines and the row that still name `XDG_STATE_HOME`. |

## Sources checked for the fold of round 1 on 2026-11-26

| File and anchor | What it establishes |
|---|---|
| `.plans/plans/go-s3-server-machines/plan.md`, lines 139 to 153 | `Issuer` has only `PublicURL` and `ServerKey`. `TokenOptions` has `Address` and `HostsServer`. `NewToken` has `ID`, `Text`, `Address` and `ExpiresAt`. |
| `.plans/plans/go-s3-server-machines/plan.md`, line 251 | An `https` address returns `ErrNoCertificatePin` in wave 1. |
| `.plans/plans/go-s3-server-machines/plan.md`, line 585 | `servertest.Server` gains `Clients(t)` and `Link()`, and `Environment()` gives a second program the settings. |
| `.plans/plans/go-s2-server-signin/plan.md`, lines 418 to 428 | `vault.Client` declares no close method. |
| `src/server/internal/store/pool.go`, line 35 | `ConnectionPolicy` has one field, `AllowPrivateStackPostgres`. |
| `src/client/internal/state/folder.go`, lines 15 to 22 | The client's lock file is `lock`, and its temporary names start with `.tmp-`. |
| `deploy/test/checks.sh`, lines 45 to 63 | `get_server` uses `curl --silent --noproxy '*' --max-time 5` and requires status 200, `product` equal to `lgtm-studio`, a positive `contract` and `sample` other than true. |
| `deploy/test/checks.sh`, lines 82 to 86 | `signin` passes the password to `jq` as an argument, which threat-r1 7 names. |
| `.plans/plans/go-s3-client-link/plan.md`, line 24 | The owner settles whether the client also reads a token from standard input. |
| `.plans/plans/go-s3-client-link/plan.md`, line 108 | `identity.json` holds `address`, which the installer compares with the selected stack. |
| A Bash probe by three reviewers | `${SHIM_WEB_PORT:-8787}` gives 8787 for an empty value, while `${SHIM_WEB_PORT-8787}` gives an empty string. |

## Sources checked for the fold of round 2 on 2026-11-26

| File and anchor | What it establishes |
|---|---|
| `deploy/up.sh`, lines 19 and 20 | The launcher calls `docker compose -f "$here/compose.yaml"`. Plan `go-s2-stack-web`, item 5, keeps that call when `COMPOSE_FILE` is unset and drops `-f` when it is set. |
| Round 2 reports architect-r2 04 and logic-r2 04 | Two reviewers found alone, with Docker Compose, that an exported default `COMPOSE_FILE` reads the settings file at the repository root, while `-f` reads the one in `deploy/`. |
| `deploy/test/with-postgres.sh`, line 7 | The wrapper runs a command given after `--` with `LGTM_TEST_POSTGRES` set to a disposable database in Docker. |
| `src/contracts/proto/lgtm/v1/clients.proto`, line 67 | `Client.connected_at` is a field of the contract, set by the server for a machine with presence, by plan `go-s3-server-machines`. |
| `.plans/plans/go-s3-server-machines/plan.md`, lines 150 to 152 | `TokenOptions` has only `HostsServer`. |
| `src/frontend/src/screens/service.tsx`, line 410 | The Machines page still shows the old join command, which gate G5 depends on. |

## Sources checked for the fold of round 3 on 2026-11-26

| File and anchor | What it establishes |
|---|---|
| `deploy/test/checks.sh`, line 378 | Check C2 calls `docker compose -f "$compose_file" -f -` with an inline override, outside the helper `compose`. |
| `deploy/test/checks.sh`, line 355 | Check C1 starts before `deploy/up.sh` has run, so a port read from Compose must come after the launcher returns. |
| `.plans/plans/go-s3-client-link/plan.md`, line 106 | `identity.json` holds `machineId`, which the live test reads to bind the waiting machine to its fixture. |
| `.plans/plans/go-s3-server-machines/plan.md`, line 343 | The server sets `connected_at` from the database's wall clock at each admission, so it shows only the latest connection. |
| `src/frontend/src/server/sync.ts`, line 117 | The website's token adapter drops the `address` of `CreateClientTokenResponse`. |
| Round 3 report threat-r3 06 | Docker Compose repeats a value of a malformed settings file on standard error when `port` fails. |
