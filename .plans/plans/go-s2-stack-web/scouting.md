# Scouting — go-s2-stack-web

This file records what the planner read and ran on 2026-11-26, with file and symbol anchors.
A file that does not exist yet is named by its target path and the nearest existing pattern.
Every probe ran on the development machine, in throwaway folders outside the checkout, with Docker 29.7.2 and Docker Compose 5.5.0.
No probe wrote a file inside the checkout.
Terms used below: HTTP is the Hypertext Transfer Protocol, HTTPS its encrypted form, and h2c is version 2 of HTTP without encryption. TAP is the Test Anything Protocol, one `ok` or `not ok` line for each test.

## The brief as it was on 2026-11-25

The brief in `plan.md` named four work items, with the sizes and needs below.
This plan replaces it, so it is kept here for the traceability fold.

| # | Work item of the brief | Files | Size | Needs |
|---|---|---|---|---|
| 1 | The image of the server: a small image that runs as a user without privileges. | `deploy/server.Dockerfile` | low | nothing |
| 2 | The image of the website and the rules of the web server: static files, and the contract's calls passed on to `server`, streams included. | `deploy/website.Dockerfile`, `deploy/Caddyfile` | medium | nothing |
| 3 | The compose file gains `server` and `web`, the settings file, and the port variable with its default of 8787. | `deploy/compose.yaml`, `deploy/example.env` | medium | 1, 2 |
| 4 | The one command that starts the stack, and the part of the user-flow script that covers sign-in turned on. | `deploy/up.sh` | low | 3 |

Its observations were: one command starts the stack and the website opens at the loopback address on port 8787; the owner signs in; the port is published on loopback only unless a public address is set, which must be HTTPS.
Its points to settle were: how a public HTTPS address is turned on and where its certificate comes from; and issue 1633.

## What was read

| File | Anchor | What it settles for this plan |
|---|---|---|
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md` | Section "The compose stack" | Four services, one address, loopback by default, the port variable with default 8787, one command. |
| Same | Section "Encryption on the link" | Loopback over plain HTTP/2; any other address over HTTPS only; the join token carries the certificate's fingerprint. |
| Same | Section "Sign-in and web sessions" | `LGTM_WEB_PASSWORD`; the cookie is `Secure` when the public address is `https`. |
| Same | Section "How wave 1 is proven done" | C1 to C4 and C13 are this sprint's checks. |
| `.plans/epics/go-rewrite/epic.md` | Table "The teams, and what each may edit" | The Stack team edits only `deploy/`. |
| `.plans/plans/go-s1-stack-vault/plan.md` | "Contract for sprint 2" | User 10001 for the server; networks `data` and `vault`; `LGTM_POSTGRES_PASSWORD_FILE` at `/run/lgtm/postgres.password`; `LGTM_OPENBAO_CREDENTIAL_FILE` at `/run/lgtm/openbao.json`; `LGTM_OPENBAO_ADDR` as `openbao` on port 8200; the launcher `deploy/up.sh`. |
| Same | Work item 2, the bullet on `internal: true` | `data` and `vault` are internal networks, and every sprint 1 service has `cap_drop: [ALL]` and `no-new-privileges:true`. |
| Same | Work item 6, the table of phases | The phases `database`, `seal`, `vault` and `ready`; the lines `phase <name> start` and `phase <name> done`; the ready line `postgres and openbao are healthy`; `--reset --yes`. |
| Same | "Post-build checks", PB5 and PB8 | Sprint 2 confirms that no server starts before the vault's setup, and runs C13 with the server. |
| `.plans/plans/go-s1-stack-vault/decisions.md` | Rows 7, 11, 16 and 25 | The images of sprint 1, the credential file, the internal networks, and the handover to sprint 2. |
| `.plans/plans/go-s2-server-signin/plan.md` | Brief, "What it must agree on with other teams" | The settings and the vault file are agreed with the Stack team. Only the brief was read, as the task asks. |
| `.plans/plans/go-s0-contract/plan.md` | Item 5, `JoinToken` field `certificate_key_sha256`; item 6, `rpc lgtm.link.v1.LinkService.Open` | The link's path prefix `/lgtm.link.v1.`, and the pinned hash of the certificate's key. |
| `.plans/plans/go-s0-contract/decisions.md` | Row 15 | The pin is the hash SHA-256 of the certificate's SubjectPublicKeyInfo. |
| `deploy/test/checks.sh` | `compose_file`, `up_file`, `configure_web`, `rpc`, `check_c1`, `check_c2`, `check_c3`, `check_c4`, `check_c13`, `vault_ready` | The names this plan keeps; C2 removes the password with `!reset null`; C4 overrides it in the shell; C13 recreates every running service. |
| `deploy/test/README.md` | Table "Names and observations the checks expect" | The names that the checks assumed. Decisions row 24 lists the one change. |
| `docs/issues.md` | Issues 1632 to 1640 | Issue 1633 is settled here. Issue 1639 explains why the key is kept. Issue 1640 does not apply, because the Stack team's tests run Docker on the host. |
| `src/frontend/vite.config.ts` | Constant `behind`, function `ownCertificate` | The development server passes `/lgtm.v1.` and `/frames/` on; the nearest pattern for making a certificate with `openssl`. |
| `src/frontend/src/server/address.ts` | `fixedAddress`, `serverUrl` | `VITE_LGTM_SERVER=/` fixes the address to the page's own. |
| `src/frontend/README.md` | Section "How it reaches a server" | The value `/` and the folders the build reads. |
| `src/frontend/package.json` | Script `build`; dependencies `@lgtm/api` and `@lgtm/mock-server` | The website build needs `src/contracts/` and `src/mock-server/`. |
| `src/frontend/tsconfig.json` | `paths` for `@mock/*` | The build reads `design/mock/`. |
| `package.json`, `pnpm-workspace.yaml` | `packageManager`, `engines`, `packages` | `pnpm@12.16.1`, Node 26.13, and the workspace folders. |
| `.gitignore` | The lines `.env`, `.env.*` and `!.env.example` | `deploy/.env` is already ignored, and `deploy/example.env` is tracked. |
| `design/openpencil/Caddyfile`, `design/openpencil/Dockerfile` | The global block, `handle`, the final stage `FROM caddy:2.10.2-alpine` | The nearest existing patterns for the web server rules and a website image. |
| `floorplan/plan.json` | Entries `deploy/server.Dockerfile`, `deploy/Caddyfile`, `deploy/website.Dockerfile`, `deploy/compose.yaml` | The planned files of the Stack team. |
| `scripts/check-private.mjs` | `privateAddress`, `homePath`, `ownerQuotation` | Loopback addresses and `0.0.0.0` are allowed; private addresses, home folders and quotations are not. |

## Files this plan creates, and their nearest pattern

| Target | Nearest existing pattern |
|---|---|
| `deploy/server.Dockerfile` | `design/openpencil/Dockerfile`, a multi-stage build that ends in a pinned runtime. |
| `deploy/server.Dockerfile.dockerignore`, `deploy/website.Dockerfile.dockerignore` | None in the repository. The mechanism was probed (see "Build context"). |
| `deploy/Caddyfile`, `deploy/caddy/site.caddy`, `deploy/caddy/site-https.caddy` | `design/openpencil/Caddyfile`. |
| `deploy/website.Dockerfile` | `design/openpencil/Dockerfile`. |
| `deploy/example.env` | None. The names come from decisions rows 11, 12 and 25. |
| `deploy/web-tls.sh` | `src/frontend/vite.config.ts`, `ownCertificate`. |
| `deploy/test/streamprobe/main.go` | The probe in "Stream probe" below. |
| `deploy/test/*.sh` | `deploy/test/checks.sh`, functions `run_check` and `fail`, and the sprint 1 suites' TAP lines. |

## Images and their digests

The command was `docker buildx imagetools inspect <image>` on 2026-11-26.
The digest is that of the multi-platform index.

| Image | Index digest | Published | Notes |
|---|---|---|---|
| `golang:1.26.11-alpine` | `sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab` | 2026-11-10 | `GOLANG_VERSION=1.26.11`, the newest 1.26 patch. |
| `gcr.io/distroless/static-debian12:nonroot` | `sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab` | not read | Read with an empty Docker settings folder, because the machine's credential helper for that registry was signed out. |
| `node:26.13.0-alpine` | `sha256:8996bb5821d62a8fd9bb27c1cb07462f3fdce6e47d640c6a881ead0b064ce216` | 2026-11-05 | The version the repository's tools use. It ships no `corepack`, so pnpm is installed with `npm`. |
| `caddy:2.11.9-alpine` | `sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0` | 2026-11-05 | `CADDY_VERSION=v2.11.9`, built with Go 1.27.3 and `golang.org/x/net` 0.60.0. The binary `/usr/bin/caddy` carries `cap_net_bind_service=p`, permitted only, and starts as user 10003 with `cap_drop ALL` and `no-new-privileges`. The plan pins this one (decisions row 32). |
| `caddy:2.10.2-alpine` | `sha256:4c6e91c6ed0e2fa03efd5b44747b625fec79bc9cd06ac5235a779726618e530d` | 2026-02-11 | `CADDY_VERSION=v2.10.2`, built with Go 1.25.0. Round 1 dropped it for 2.11.9 with `read_body_idle 10m` (decisions row 32). Its `golang.org/x/net` is version 0.42.0. The binary carries `cap_net_bind_service=ep`, so the stock image refuses to start with `cap_drop ALL` and `no-new-privileges`, with `exec /usr/bin/caddy: operation not permitted`. A copy of the binary made in a build step carries no capability and starts. |

## Probes

### Build context

A folder held `go.mod`, `src/server/main.go` and a planted file `deploy/.state/sentinel`.
A Dockerfile `deploy/x.Dockerfile` copied the whole context, and `deploy/x.Dockerfile.dockerignore` held `*`, `!go.mod` and `!src/server/**`.
The build listed only `/ctx/go.mod` and `/ctx/src/server/main.go`.
The planted file did not reach the build.

### Website build

A Dockerfile of the shape of work item 3, with the allow-list of work item 3 minus the two `deploy/` lines, built from the repository root.
`pnpm install --frozen-lockfile --filter @lgtm/frontend...` reported that the lockfile is up to date and installed 212 packages, although most workspace folders were left out.
`VITE_LGTM_SERVER=/ pnpm --filter @lgtm/frontend build` succeeded in under one second of bundling.
The final image held `/srv/site/index.html` with `id="app"` and two `<script` elements, `assets/` and `plans/`, 1.1 megabytes in all, and the image was 67.3 megabytes.

### Compose settings

A compose file in a folder `deploy/` was read with `docker compose -f deploy/compose.yaml config` from its parent folder.

| Case | Result |
|---|---|
| `deploy/.env` holds the password | The server's environment holds the file's value. |
| The shell also sets it | The shell's value wins. |
| The override of `check_c2` with `LGTM_WEB_PASSWORD: !reset null` | The key is gone, and the other keys stay. |
| Neither the file nor the shell sets it | The value is an empty string, so the server must treat empty as missing. |
| `docker compose config --environment` | It prints the resolved `LGTM_` values, with the shell winning over the file. |

### Networks

A web container on an ordinary network `edge` and an internal network `app` published `127.0.0.1:18789`, and a request through it succeeded.
It reached a second container that joined only `app`.
That second container could not open a connection out of Docker ("Network unreachable").

### Web server rules

The pinned Caddy validated a Caddyfile that defines the snippet `(app)` before an `import` of the mode's file.
A snippet defined after the import failed with "File to import not found: app".
Run as user `10003:10003` with `--cap-drop ALL`, `no-new-privileges`, a read-only root and a temporary `/tmp`, Caddy started and served.

The mode switch `import /etc/caddy/site{$LGTM_WEB_TLS}.caddy` loaded `site.caddy` when the variable was missing or empty, and `site-https.caddy` when it was `-https`.
An earlier form, `site-{$LGTM_WEB_MODE:loopback}.caddy`, failed when the variable was set but empty, which is why the plan uses the first form.

The exact rules of work item 2, with a stub server on the network alias `server`, gave these results:

| Request | Result |
|---|---|
| `GET /` | The test page with `id="app"`. |
| `POST /lgtm.v1.SessionService/GetServer` with `X-Forwarded-Proto: https` | The stub saw `HTTP/2.0`, the same path, the host `127.0.0.1`, and `X-Forwarded-Proto: http`. |
| `POST /lgtm.link.v1.LinkService/Join`, `GET /frames/x` | The stub saw `HTTP/2.0` and the same path. |
| `POST /lgtm.v2.X` | 404 from the static files. |
| `POST /lgtm.v1.SessionService/SignOut` | The stub's 401, `application/json` and body, unchanged. |
| `Host: lgtm.test:8787` on `/` and on `SignIn` | 421 for both, and the stub's log gained no line. |
| `Host: localhost:8787` | 200. |
| A missing file | 404. |
| `http://127.0.0.1:8081/healthz` inside the web container | 200. From the stub's container it was refused. |

### Stream probe

The program was a standard-library Go program with a `serve` mode and a `dial` mode, built in `golang:1.26.11-alpine` with `CGO_ENABLED=0`.
The server side ran on the network alias `upstream` with HTTP/1.1 and cleartext HTTP/2 through `http.Server.Protocols`, and echoed each line with a flush and a closing trailer.
Caddy passed `/lgtm.link.v1.*` on with `reverse_proxy h2c://upstream:9000 { flush_interval -1 }` and `protocols h1 h2 h2c`.
The client side dialled `http://127.0.0.1:18787/lgtm.link.v1.LinkService/Open` with cleartext HTTP/2 from a container on the host network.

| Run | Result |
|---|---|
| Messages every 2 seconds for 20 seconds, then 35 seconds of silence | `ok stream lasted 55s with 12 messages each way and trailer 12`. Both hops used HTTP/2. |
| Messages every 10 seconds for 6 minutes, then 6 minutes of silence | Every message of the first 6 minutes was echoed. The first message after the silence failed with `stream error: stream ID 1; INTERNAL_ERROR; received from peer`, at 12 minutes. |

The second run shows that a silence of 6 minutes cuts the stream with Caddy 2.11.9 and the default limits.
The section "Which limit cuts a silent stream" records the follow-up runs.

### Which limit cuts a silent stream

Every run below used the stand-in program of the stream probe and Caddy 2.11.9, unless the row says otherwise.
Each run of the first table sent messages both ways for 4 seconds and then held both directions silent.

| Variant | Silence | Result |
|---|---|---|
| Default rules | 35 seconds | Passed. |
| Default rules | 50 seconds | Passed: `ok stream lasted 54s`. |
| Default rules | 65 seconds | Cut at 1 minute 9 seconds. |
| Default rules | 125, 185, 270 and 330 seconds | Cut each time, a few seconds after the silence ended. |
| `timeouts { idle 15m }` on the server | 330 seconds | Cut. |
| Upstream `keepalive off` | 185 seconds | Cut. |
| Upstream `read_timeout 1h` | 70 seconds | Cut. |
| Upstream `write_timeout 1h` | 70 seconds | Cut. |
| `timeouts { read_body 1h }` on the server | 70 seconds | Cut. |
| No proxy, straight to the stand-in server | 330 seconds | Passed: `ok stream lasted 5m34s with 4 messages each way and trailer 4`. |

Every cut showed `stream error: stream ID 1; INTERNAL_ERROR; received from peer` at the client.
Caddy logged `aborting with incomplete response` with the error `reading: i/o timeout` from the upstream side.
The limit therefore lies between 50 and 65 seconds, and none of the timeouts that the Caddyfile exposes moves it.

A second table held one direction silent for 80 seconds while the other sent a line every 2 seconds.

| Shape | Through Caddy 2.11.9 | Straight to the stand-in | Through Caddy 2.10.2 |
|---|---|---|---|
| The machine sends, the server is silent until the end (`quiet-down`) | Passed. | Passed. | Not run. |
| The server sends, the machine is silent (`quiet-up`) | Cut at 60 seconds, after 30 ticks. | Passed with 40 ticks. | Passed with 40 ticks and the trailer `up-closed`. |

The cut needs only the upward direction, from the machine to the server, to be silent for about 60 seconds.
Traffic in the other direction does not keep the stream open.
Caddy 2.10.2 does not cut it.
The probes of the web rules (page, contract paths, status kept, host refusal, unknown mode, health listener) passed on a copy of Caddy 2.10.2, run as user 10003 with `cap_drop ALL`, `no-new-privileges` and a read-only root.

A soak on Caddy 2.10.2 ran both shapes of the plan's soak test at the same time.

| Run on Caddy 2.10.2 | Result |
|---|---|
| Messages both ways for 4 seconds, then both directions silent for 6 minutes and 10 seconds | Passed: `ok stream lasted 6m14s with 4 messages each way and trailer 4`. |
| The server sends a tick every 2 seconds, and the machine is silent for 6 minutes and 30 seconds | Passed: `ok quiet-up lasted 6m30s with 195 ticks down and one line up; trailer up-closed`. |

## Round 1 probes

The fold of review round 1 ran these probes on 2026-11-26, with Docker Compose 5.5.0.

### The setting that removes the cut

Caddy 2.11.9 has the timeout `read_body_idle`, which the earlier runs did not try.

| Variant on Caddy 2.11.9 | Shape | Result |
|---|---|---|
| Default rules | The server sends, the machine is silent for 80 seconds | Cut at 60 seconds, after 29 ticks. |
| `timeouts { read_body_idle 0 }` | The same | Cut at 60 seconds, so 0 keeps the default. |
| `timeouts { read_body_idle 10m }` | The same | Passed with 40 ticks and the trailer `up-closed`. |
| `timeouts { read_body_idle 10m }` | Both directions silent for 75 seconds | Passed: `ok stream lasted 1m19s with 4 messages each way and trailer 4`. |
| `timeouts { read_body_idle 10m }` | Both directions silent for 6 minutes and 10 seconds | Passed: `ok stream lasted 6m14s with 4 messages each way and trailer 4`. |
| `timeouts { read_body_idle 10m }` | The machine silent for 6 minutes and 30 seconds | Passed: `ok quiet-up lasted 6m30s with 195 ticks down and one line up; trailer up-closed`. |
| `timeouts { read_body_idle 10m }` | The server silent for 6 minutes and 30 seconds | Passed: `ok quiet-down lasted 6m30s, 196 lines up, none down until the end`. |

### The web server rules in one file

The exact rules of work item 2, in one `Caddyfile`, ran on Caddy 2.11.9 as user 10003 with `--read-only`, `--cap-drop ALL` and `no-new-privileges`, in front of a stub server.
`GET /` served the page.
`GetServer` reached the stub over HTTP/2 with `host=127.0.0.1` and `xfp=http`, although the request sent `X-Forwarded-Proto: https`.
`Join` reached the stub over HTTP/2.
`SignOut` kept the stub's 401.
`Host: lgtm.test:8787` got 421, and `Host: localhost:8787` got 200.
`/frames/x` and `/lgtm.v2.X` got 404.
The health listener answered inside the container and refused a connection from the stub's container.

### Compose and Docker behaviour

| Probe | Result |
|---|---|
| `LGTM_PUBLIC_URL: http://127.0.0.1:${LGTM_WEB_PORT:-8787}` | With no port set, the value is `http://127.0.0.1:8787`. With `LGTM_WEB_PORT=18787`, it is `http://127.0.0.1:18787`, and the published port follows. |
| `LGTM_WEB_PORT` of `web`, `65536` or `-1` | `docker compose config` fails with `invalid hostPort`. |
| `LGTM_WEB_PORT=0` | Compose accepts it, and Docker publishes a random port. The guide says that 0 is not supported. |
| A port that another process holds | `docker compose up` fails with `failed to bind host port 127.0.0.1:<port>/tcp: address already in use`. |
| `COMPOSE_ENV_FILES=<file>` | Compose reads that file in place of `deploy/.env`, and a value in the shell still wins. |
| `docker compose up -d --wait` of a service that exits 1 at once, with `restart: unless-stopped` and `start_period: 8m` | The command failed after 3 seconds with `container ... is unhealthy` and exit status 1. |

