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
| `.plans/plans/go-s1-client-local/plan.md`, "The layout of the state folder" and item 1 | The folder is `LGTM_CLIENT_STATE_DIR`, then `$XDG_STATE_HOME/lgtm-client`, then `$HOME/.local/state/lgtm-client`. It holds `machine.key` and `server.json`. One client holds a lock on the folder. `main.version` is set with `-ldflags "-X main.version=<v>"`. |
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
| `deploy/test/linkecho/main.go`, `serve` and `dial` | The trial program below. |
| `deploy/test/linkecho.compose.yaml`, service `server` | `deploy/test/checks.sh`, `check_c2`, which feeds an override to `docker compose -f - up`. |
| `deploy/test/link-proxy.sh`, `run_test` | `deploy/test/checks.sh`, `run_check`. |
| `deploy/Caddyfile`, option `protocols` and the route `/lgtm.link.v1.*` | The trial's Caddyfile below. |
| `deploy/README.md`, section "Installing the client" | `deploy/README.md` of plan `go-s1-stack-vault`, item 6. |

## A two-way stream through Caddy on 2026-11-26

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

A disposable container of `postgres:16.16-alpine` ran with the account and database `lgtm_app` and with password authentication on both the network and the socket, as plan `go-s1-stack-vault` sets it.
The password sat in a mounted file, as that plan mounts it.

| Form | Result |
|---|---|
| `psql -U lgtm_app -d lgtm_app` on the socket, as `psql_in_stack` in `deploy/test/checks.sh` runs it | `fe_sendauth: no password supplied`. |
| The body proposed in the result file, which reads the mounted file and connects over loopback | The `UPDATE ... RETURNING id` of C5 printed the identifier and exited 0. |

## Why exit code 75

Code 75 is `EX_TEMPFAIL` in the list of exit codes of `sysexits.h`, which means "try again later".
It is not a code that the Go runtime uses for a crash, which is 2.
