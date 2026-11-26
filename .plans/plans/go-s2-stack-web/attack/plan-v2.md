---
topic: go-s2-stack-web
status: attack
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# The images, the web server, and one command that starts everything

## Overview

Sprint 2 of the Go rewrite ends when the owner runs one command, opens the website at the loopback address, and signs in against the Go server.
Sprint 1 already starts PostgreSQL and the vault, OpenBao, with one command.
This plan adds the two services that are still missing: the Go server, and a web server that serves the website.
It also extends that one command so that it starts all four services in the right order.

The website and the server share one address.
The web server, Caddy, serves the built website as static files.
It passes the website's calls, which follow the contract in `src/contracts/`, on to the server inside the stack.
Because the browser sees one address, the strict session cookie of the design works, and the website's server address is `/`.

The stack publishes its port on the loopback address only, at `http://127.0.0.1:8787` by default.
The web server also refuses any host name other than a loopback name, so a page never loads over plain HyperText Transfer Protocol (HTTP) from another computer.
A public HTTPS address, the encrypted form of HTTP, is outside wave 1: the owner reaches the stack from another computer through a port forward, and sets up a proxy for a public address later (decisions row 27).
Every check of wave 1 runs on the loopback address, so none of them needs a certificate.

The link between the server and a machine is one long two-way stream, and issue 1633 asks for proof that the web server passes such a stream on without cutting it.
The probes of 2026-11-26 found that Caddy 2.11.9, with its default settings, cuts a stream once the machine's side has been silent for about 60 seconds.
The setting `read_body_idle 10m` removes that cut, so the plan keeps the current Caddy release and sets it.
The tests hold each direction silent for longer than a minute on every run, and a six-minute soak runs before sprint 3 builds the link.

## Key decisions

**One address for the website and the server.** The web server passes two kinds of path to the server: the website's calls and the link's calls. Everything else is a file of the built website.

**Loopback only in wave 1.** The published port binds to `127.0.0.1`, and no setting changes that. A public address waits for the owner's proxy (decisions row 27).

**The server holds every rule about settings.** The launcher reads no setting. The server refuses a missing password, Compose refuses a malformed port, and Docker reports a port that is taken; the launcher only names the setting to change (decisions row 29).

**The ordinary command also removes a password.** The owner deletes the password and runs the launcher again. The server then ends every session and refuses to start, and the launcher stops it (decisions row 31).

**Images are built from an allow-list.** Each image builds from the repository, but a list next to its build file lets in only the folders it needs. The state folder, which holds the vault's key and credentials, therefore never reaches an image.

Full log → `decisions.md`

## Workstreams

### The server and the website become images

The server becomes one small image that holds the program and nothing else, not even a shell.
The website is built inside a second image and served by the web server, with rules that pass the calls on to the server.
The main risk is that a secret from the state folder could slip into an image, and each image's tests plant a decoy secret to prove it cannot.
When this is done, each image builds with one command, runs as an ordinary user without any special privilege, and passes its own tests.

### One command starts everything, and the owner signs in

The compose file gains the services `server` and `web`, and the launcher of sprint 1 gains the steps that build and start them after the vault.
The main risks are a server that gives up while its database migrates, and a password removal that leaves old sessions alive.
Tests read the resolved compose file, and the launcher's tests start disposable stacks and break them on purpose.
When this is done, `deploy/up.sh` on a clean machine starts all four services, the website opens at the printed loopback address, and the owner signs in.

### The link's long stream passes through the web server

The link between the server and a machine is one stream that stays open for days, with messages in both directions.
No link exists yet, so a small stand-in program plays both ends through the real web server rules.
The risk is real: Caddy 2.11.9 with default settings cuts a stream whose upward direction is silent for about 60 seconds.
The tests hold each direction silent for 80 seconds, and the soak holds a stream silent for 6 minutes.
When this is done, the stream test passes, and the soak has passed once on the merged image before sprint 3 starts.

## Rollout order

1. Build the web server rules, and the stream test in its rules-only form. Neither needs another plan.
2. Build the website image. Build the server image once the Server team's item 9 has merged its command.
3. Add the two services to the compose file and extend the launcher, once the sprint 1 stack is merged.
4. Run the sign-in checks of the user-flow script on a clean machine once the Server team's plan is merged, and extend the guide in `deploy/README.md`.
5. Run the stream test against the merged website image, then the six-minute soak, before sprint 3 builds the link, and record the result in the sprint's report.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

All paths are relative to the repository root.
Every test file named here is a Bash script under `deploy/test/` that prints one Test Anything Protocol (TAP) line for each named assertion, in the form `ok <n> - <test name>` or `not ok <n> - <test name>`, and exits with status 0 only when every line is `ok`.
This is the format of the sprint 1 suites.
A test that needs a disposable stack uses `web_fixture_create` and `web_fixture_cleanup` from `deploy/test/web-testlib.sh`, which work item 4 writes (architect-r1 2, critic-r1 4, logic-r1 4, threat-r1 5).
A test never reads or changes the developer's own `deploy/.env` or `deploy/.state/`.
The image digests are those of decisions rows 8 and 32.
Each workstream below is named WS-1 to WS-3, for workstream 1 to 3.

The design statements are numbered as the task assignment asks: D1 to D23 are the rows of the design's decisions table, R1 to R11 its rules, and C1 to C13 its checks, in the order of the design file on `master`.

**WS-1: The server and the website become images**

Both images build from the repository root, so that they can read the source, and both keep the rest of the repository out with an allow-list.
The build tool reads an ignore file named after the Dockerfile, next to it, in place of any ignore file at the root; a probe on 2026-11-26 confirmed this (scouting, "Build context").
The server image holds one static program on a base image that has no shell.
The web server's rules come first, because the website image copies them in, and because the stream test of WS-3 runs against them.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1a | The server image builds from pinned bases, runs as user 10001 with no shell, and holds no file from the state folder. | `bash deploy/test/server-image.sh` prints `ok` for its six tests. |
| 1b | The web server rules serve the website, pass the two path prefixes on over HTTP/2, keep status and body unchanged, and refuse a host name that is not loopback before the server sees the request. | `bash deploy/test/web-rules.sh` prints `ok` for its six tests. |
| 1c | The website image builds the website with the server address `/`, serves it as user 10003 with no capability, and holds no build tool and no file from the state folder. | `bash deploy/test/web-image.sh` prints `ok` for its eight tests. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1 | Write the server image and its allow-list. | Target `deploy/server.Dockerfile`, stages `build` and `runtime`; target `deploy/server.Dockerfile.dockerignore`. Nearest pattern: `design/openpencil/Dockerfile`, multi-stage build ending in a pinned runtime. The program is `src/server/cmd/lgtm-server/main.go`, which `.plans/plans/go-s2-server-signin/plan.md`, item 9, writes. |
| 2 | Write the web server rules. | Target `deploy/Caddyfile`, snippet `(app)`. Nearest pattern: `design/openpencil/Caddyfile`, global block and `handle`. Paths to pass on: `src/frontend/vite.config.ts`, constant `behind`. |
| 3 | Write the website image and its allow-list. | Target `deploy/website.Dockerfile`, stages `site` and `runtime`; target `deploy/website.Dockerfile.dockerignore`. The server address: `src/frontend/src/server/address.ts`, `fixedAddress` and `VITE_LGTM_SERVER`. The build: `src/frontend/package.json`, script `build`. |

**Design artifact:** none. No screen changes; the website is built as it is.

**Work item 1. The server image**

- **Files:** creates `deploy/server.Dockerfile`, `deploy/server.Dockerfile.dockerignore` and `deploy/test/server-image.sh`.
- **Needs:** nothing to write its tests. Its build and its last test need `src/server/cmd/lgtm-server/main.go`, which item 9 of `go-s2-server-signin` creates, so it merges after that item (architect-r1 6). It can run at the same time as items 2 and 7.
- **Work area and complexity:** `infra`, low. It is one Dockerfile of a known shape, and its acceptance is one script.
- **Scope:** `stack/infra`.
- **Statements:** D1, D2, C2, R9, and the design's sections "Where the code lives" (`deploy/` holds the two images) and "The compose stack".
- **What it builds:** the stage `build` starts from `golang:1.26.11-alpine@sha256:ecb67f98…`. It copies `go.mod` and `go.sum`, runs `go mod download` with a cache mount on `/go/pkg/mod`, copies the rest of the allowed context, and runs `CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/lgtm-server ./src/server/cmd/lgtm-server` with a cache mount on `/root/.cache/go-build`. The stage `runtime` starts from `gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872…`, copies `/out/lgtm-server` to `/usr/local/bin/lgtm-server`, sets `USER 10001:10001`, sets `ENTRYPOINT ["/usr/local/bin/lgtm-server"]`, and sets no `CMD`.
- **The allow-list:** `deploy/server.Dockerfile.dockerignore` holds exactly these lines, in this order: `*`, `!go.mod`, `!go.sum`, `!src/server/**`, `!src/contracts/gen/go/**`, `**/*_test.go`. A build that needs another path fails loudly, and the Server team then asks for the path.

Tests written first, in `deploy/test/server-image.sh`:

| Test | What it asserts |
|---|---|
| `server_image_bases_are_pinned` | Every `FROM` line of `deploy/server.Dockerfile` names an image followed by `@sha256:` and 64 lowercase hexadecimal characters. The two digests equal those of decisions row 8. Before the file exists, the test fails on the file check. |
| `server_image_context_is_allow_listed` | The ignore file's lines, with comments and blank lines removed, equal the list above exactly. |
| `server_image_leaves_out_state` | The test copies the tracked files of the checkout (`git ls-files -z`) into a temporary folder. It adds `deploy/.state/sentinel-<random>` and `deploy/.env`, each holding a random 32-character marker. It builds the stage `build` and the final image from that copy. In the `build` stage, `find / -xdev -name 'sentinel-*'` finds nothing, and `grep -r` finds neither marker under the build folder. `docker save` of the final image contains neither marker. |
| `server_image_runs_unprivileged` | `docker image inspect lgtm-server:local` reports `Config.User` equal to `10001:10001`, `Config.Entrypoint` equal to `["/usr/local/bin/lgtm-server"]`, and an empty `Config.Cmd`. |
| `server_image_has_no_shell` | `docker run --rm --entrypoint /bin/sh lgtm-server:local -c true` exits with a status other than 0. |
| `server_image_refuses_without_password` | `docker run --rm --read-only --cap-drop ALL lgtm-server:local`, with no environment variable set, exits within 10 seconds with a status other than 0, and its output contains `LGTM_WEB_PASSWORD`. This is C2 at the level of the image. |

- **Risky:** yes, because the state folder holds the vault's seal key and credentials. The mutation that must turn `server_image_leaves_out_state` red: remove the line `*` from the ignore file. A second mutation: remove the `USER` line, which must turn `server_image_runs_unprivileged` red.
- **Acceptance:** `bash deploy/test/server-image.sh` prints six `ok` lines and exits 0.
- **Public names:** the image tag `lgtm-server:local`, the program path `/usr/local/bin/lgtm-server`, the user `10001:10001`, the stage names `build` and `runtime`, and the rule that running the image with no arguments starts the server.

**Work item 2. The web server rules**

- **Files:** creates `deploy/Caddyfile` and `deploy/test/web-rules.sh`.
- **Needs:** nothing. It can run at the same time as items 1 and 7.
- **Work area and complexity:** `infra`, medium. The rules are short, but they decide which requests reach the server and how a stream is passed on.
- **Scope:** `stack/infra`.
- **Statements:** D1, C1, C3, C9, R8, and the design's section "The compose stack" (the website and the server share one address, and the website's server address is `/`).
- **What the rules say:** `deploy/Caddyfile` is one file. Its global block holds `admin off`, `auto_https off`, `persist_config off`, `grace_period 5s`, and `servers { protocols h1 h2 h2c timeouts { read_body_idle 10m } }` (threat-r1 7). It sets no other timeout. It defines the snippet `(app)`, in which the matcher `@server path /lgtm.v1.* /lgtm.link.v1.*` is handled by `reverse_proxy h2c://server:8080 { flush_interval -1 }`, which passes each call on as cleartext HTTP/2 (h2c), and every other path is handled by `root * /srv/site` and `file_server`. It then defines the site `http://:8081` with `bind 127.0.0.1` and `respond /healthz 200`, the site `http://127.0.0.1:8080, http://localhost:8080, http://[::1]:8080` with `import app`, and the site `http://:8080` with `respond 421`. No access log is written, because requests carry the session cookie. Scouting, "Round 1 probes", records the probe of this exact text on Caddy 2.11.9.

Tests written first, in `deploy/test/web-rules.sh`.
Each test runs the pinned Caddy image as user `10003:10003`, with `--read-only`, `--cap-drop ALL`, `--security-opt no-new-privileges:true`, a temporary file system at `/tmp`, `XDG_DATA_HOME=/tmp` and `XDG_CONFIG_HOME=/tmp`.
It mounts `deploy/Caddyfile` at `/etc/caddy/Caddyfile`, and a test folder holding `index.html` (with `id="app"` and a `<script>` element) and `assets/a.js` at `/srv/site`.
It publishes port 8080 on `127.0.0.1` at a port Docker chooses.
A stub server, a second container of the pinned Caddy image with the network alias `server`, listens on port 8080 with `protocols h1 h2c`. It answers `/lgtm.v1.SessionService/SignOut` with status 401, `Content-Type: application/json` and the body `{"code":"unauthenticated","message":"Sign in first."}`. It answers every other path with status 200 and the body `{http.request.proto} {method} {path} host={http.request.host} xfp={header.X-Forwarded-Proto}`, and it logs each request.

| Test | What it asserts |
|---|---|
| `rules_serve_the_website` | `GET /` returns 200 with a body that contains `id="app"`. `GET /assets/a.js` returns 200 with a `Content-Type` that starts with `text/javascript`. `GET /no/such/file` returns 404. |
| `rules_pass_contract_paths_on` | `POST /lgtm.v1.SessionService/GetServer` and `POST /lgtm.link.v1.LinkService/Join` each return 200 with a body that starts with `HTTP/2.0`, followed by the method and the same path. `POST /lgtm.v2.X` and `GET /frames/x` each return 404 and do not reach the stub. |
| `rules_keep_status_and_body` | `POST /lgtm.v1.SessionService/SignOut` returns 401, `Content-Type: application/json`, and the stub's body byte for byte. |
| `rules_keep_host_and_set_forwarded_proto` | The stub sees `host=127.0.0.1`. A request that sends `X-Forwarded-Proto: https` reaches the stub with `xfp=http`. |
| `rules_refuse_other_host_names` | With the header `Host: lgtm.test:8787`, `GET /` and `POST /lgtm.v1.SessionService/SignIn` each return 421, and the stub's log gains no line. With `Host: localhost:8787`, `GET /` returns 200. |
| `rules_answer_health_inside_only` | Inside the web container, `wget -qO- http://127.0.0.1:8081/healthz` exits 0. From the stub's container, a request to port 8081 of the web container fails to connect. |

- **Risky:** yes, because these rules decide which requests carry the password to the server. The mutation that must turn `rules_refuse_other_host_names` red: replace the three loopback site addresses and the 421 site with one site `http://:8080 { import app }`. A second mutation: change `h2c://server:8080` to `http://server:8080`, which must turn `rules_pass_contract_paths_on` red, because the stub then reports `HTTP/1.1`.
- **Acceptance:** `bash deploy/test/web-rules.sh` prints six `ok` lines and exits 0.
- **Public names:** the file `/etc/caddy/Caddyfile` inside the image; the container port 8080; the upstream `server:8080` over cleartext HTTP/2 (h2c); the health address `http://127.0.0.1:8081/healthz` inside the container; the status 421 for a host name that is not loopback; the static root `/srv/site`; the two prefixes `/lgtm.v1.` and `/lgtm.link.v1.`.

**Work item 3. The website image**

- **Files:** creates `deploy/website.Dockerfile`, `deploy/website.Dockerfile.dockerignore` and `deploy/test/web-image.sh`.
- **Needs:** item 2. It can run at the same time as items 1 and 7.
- **Work area and complexity:** `infra`, medium. The build reads several folders of the workspace, and its allow-list must name each one.
- **Scope:** `stack/infra`.
- **Statements:** D1, C1, R9, and the design's section "The compose stack".
- **What it builds:** the stage `site` starts from `node:26.13.0-alpine@sha256:8996bb58…` and runs `npm install --global pnpm@12.16.1`, because Node 26 ships without Corepack. It copies the allowed context to `/repo`, runs `pnpm install --frozen-lockfile --filter @lgtm/frontend...` with a cache mount on the store of pnpm, and runs `VITE_LGTM_SERVER=/ pnpm --filter @lgtm/frontend build`. The stage `runtime` starts from `caddy:2.11.9-alpine@sha256:003a9780…` (decisions row 32). It copies `/repo/src/frontend/dist` to `/srv/site` and `deploy/Caddyfile` to `/etc/caddy/Caddyfile`. It sets `USER 10003:10003`, `ENV XDG_DATA_HOME=/tmp XDG_CONFIG_HOME=/tmp`, and `CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]`.
- **The allow-list:** `deploy/website.Dockerfile.dockerignore` holds `*`, then `!package.json`, `!pnpm-lock.yaml`, `!pnpm-workspace.yaml`, `!tsconfig.base.json`, `!src/frontend/**`, `!src/contracts/**`, `!src/mock-server/**`, `!design/mock/**`, `!design/fonts/**`, `!third_party/inter/**`, `!third_party/jetbrains-mono/**`, `!third_party/codemirror/**`, `!deploy/Caddyfile`, and then `**/node_modules`, `**/dist` and `**/.cert`. A probe build with this list passed on 2026-11-26 (scouting, "Website build").

Tests written first, in `deploy/test/web-image.sh`:

| Test | What it asserts |
|---|---|
| `web_image_bases_are_pinned` | Every `FROM` line of `deploy/website.Dockerfile` names an image followed by `@sha256:` and 64 lowercase hexadecimal characters, and the digests equal those of decisions rows 8 and 32. |
| `web_image_context_is_allow_listed` | The ignore file's lines, with comments and blank lines removed, equal the list above exactly. |
| `web_image_leaves_out_state` | In a temporary copy of the tracked files, the test adds `deploy/.state/sentinel-<random>`, `deploy/.env` and `src/frontend/.cert/key.pem`, each with a random marker. In the stage `site`, `find / -xdev -name 'sentinel-*'` and `find / -xdev -path '*/.cert/*'` find nothing. `docker save` of the final image contains none of the markers. |
| `web_image_serves_the_built_site` | The image, run with no stub server, answers `GET /` with 200, `id="app"` and at least one `<script` element. Every `src` and `href` in that page that starts with `/` returns 200. |
| `web_image_fixes_the_server_address` | `docker history --no-trunc` of the stage `site` shows a build step that contains `VITE_LGTM_SERVER=/`. |
| `web_image_carries_the_rules` | `/etc/caddy/Caddyfile` in the image equals `deploy/Caddyfile` byte for byte. `Config.User` is `10003:10003`. |
| `web_image_runs_unprivileged` | The image, run as in `web_image_serves_the_built_site` with `--user 10003:10003`, `--read-only`, `--cap-drop ALL` and `--security-opt no-new-privileges:true`, answers `GET /` with 200. |
| `web_image_has_no_build_tools` | `docker run --rm --entrypoint sh lgtm-web:local -c 'command -v node || command -v pnpm || command -v npm'` exits with a status other than 0. |

- **Risky:** yes, for the same reason as item 1. The mutation that must turn `web_image_leaves_out_state` red: remove the line `*` from the ignore file. A second mutation: remove the `USER` line, which must turn `web_image_carries_the_rules` red.
- **Acceptance:** `bash deploy/test/web-image.sh` prints eight `ok` lines and exits 0. A person then runs the image as in `web_image_serves_the_built_site`, opens the page in a browser, and sees the screen that says no LGTM Studio server answers at the page's own address, with no field for an address (critic-r1 12).
- **Public names:** the image tag `lgtm-web:local`, the stage names `site` and `runtime`, the user `10003:10003`, and the build value `VITE_LGTM_SERVER=/`.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A seal key, credential or the settings file reaches an image layer. | High | Allow-lists that start with `*`, and tests that plant random markers and search every stage and the saved image for them. |
| A later Caddy image carries its binary's capability as effective, and the unprivileged start fails. | Medium | The image is pinned by digest, and `web_image_runs_unprivileged` runs on every build. Caddy 2.10.2 is such an image (scouting, "Images and their digests"). |
| The web server passes on a path that the server should not answer, or misses one it should. | Medium | One matcher of two prefixes, tested by path, and decisions row 35 records the list. |
| The website build needs a folder that the allow-list leaves out. | Low | The build fails at once and names the file, and the list is changed in item 3. |

**Verify:** `bash deploy/test/server-image.sh`, `bash deploy/test/web-rules.sh` and `bash deploy/test/web-image.sh` on Linux with Docker. A person opens the website image once in a browser. No other device is needed.

**Rollout detail:** step 1 builds item 2. Step 2 builds item 3, and item 1 after Server item 9. Each runs `docker build -f deploy/<name>.Dockerfile -t <tag> .` from the repository root.

**WS-2: One command starts everything, and the owner signs in**

The compose file of sprint 1 holds `postgres`, `openbao` and the one-shot setup jobs.
This workstream adds `server` and `web`, two networks, and the settings file, and it extends `deploy/up.sh`, which sprint 1 writes.
Compose alone reads the settings: a value in the shell wins, and the settings file supplies the rest (scouting, "Compose settings").
The settings file is `deploy/.env`, unless `COMPOSE_ENV_FILES` names another one, which is how the fixtures keep their own file (scouting, "Round 1 probes").

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2a | The resolved compose file publishes only the web port, on `127.0.0.1`, keeps the server off any network with a route out, mounts credentials read-only, and gives the server exactly the settings of the Server plan. | `bash deploy/test/stack-web.sh` prints `ok` for its nine tests. |
| 2b | The launcher stops after the vault when asked, starts no server before the vault is ready, waits through a slow first start, stops on a failed one, names a taken port, ends every session when the password is removed, and changes nothing on a second run. | `bash deploy/test/stack-up.sh` prints `ok` for its nine tests. |
| 2c | On a clean machine, one command starts the stack, the server refuses to start without a password, the owner signs in, the session survives a restart, a changed password ends it, and the vault opens again after a restart of everything. | `deploy/test/checks.sh --only C1,C2,C3,C4,C13` prints five `ok` lines and exits 0. |
| 2d | The owner signs in through the browser at a port other than 8787. | A person runs `LGTM_WEB_PORT=18787 deploy/up.sh`, opens `http://localhost:18787/`, types a wrong password and sees it refused, then types the right one and sees the sign-in form give way. After a reload the sign-in form does not return. (critic-r1 2) **UNREVIEWED MECHANISM:** Home needs the server's first `WatchClients` snapshot, which sprint 2 does not serve, so Home is check PB2. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 4 | Add `server`, `web`, the networks, the settings file and the test fixtures. | `deploy/compose.yaml`, `services.postgres` and the networks `data` and `vault`, which sprint 1 creates (`.plans/plans/go-s1-stack-vault/plan.md`, work items 2 and 3, and its "Contract for sprint 2"). The settings: `.plans/plans/go-s2-server-signin/plan.md`, item 1, the table of settings. Target `deploy/example.env`. Target `deploy/test/web-testlib.sh`. The names the check script reads: `deploy/test/checks.sh`, `compose_file`, `check_c2` and `check_c4`. |
| 5 | Extend the launcher. | `deploy/up.sh`, its phases `database`, `seal`, `vault` and `ready`, its lock and its line `postgres and openbao are healthy` (sprint 1, work item 6). The suite to keep green: `deploy/test/stack-vault.sh`. The server's startup: `.plans/plans/go-s2-server-signin/plan.md`, item 9, the startup order and the table of sentences. |
| 6 | Extend the operator guide and run the sign-in checks. | `deploy/README.md`, which sprint 1 creates. `deploy/test/checks.sh`, functions `check_c1` to `check_c4` and `check_c13`. `deploy/test/README.md`, table "Names and observations the checks expect". |

**Design artifact:** none. No screen changes.

**Work item 4. The compose services, the networks, the settings file and the fixtures**

- **Files:** changes `deploy/compose.yaml`. Creates `deploy/example.env`, `deploy/test/web-testlib.sh` and `deploy/test/stack-web.sh`.
- **Needs:** items 1 and 3, and the merged sprint 1 plan `go-s1-stack-vault`. It can run at the same time as item 7.
- **Work area and complexity:** `infra`, medium. It adds two services to a file that follows a set pattern, and it wires credentials that sprint 1 already makes.
- **Scope:** `stack/infra`.
- **Statements:** D4, D10, C1, C2, C3, C4, C13, R9, and the design's section "The compose stack" (loopback by default, the port as a variable with the default 8787, the server reachable only inside the stack).
- **The service `server`:** `build` with `context: ..`, `dockerfile: deploy/server.Dockerfile` and `target: runtime`; `image: lgtm-server:local`; `user: "10001:10001"`; `read_only: true`; `tmpfs: [/tmp]`; `cap_drop: [ALL]`; `security_opt: [no-new-privileges:true]`; `restart: unless-stopped`; `networks: [app, data, vault]`; `depends_on` on `postgres` and `openbao`, each with `condition: service_healthy`. It publishes no port. Its health check is `["CMD", "/usr/local/bin/lgtm-server", "health"]` with `start_period: 8m`, `start_interval: 2s`, `interval: 10s`, `timeout: 3s` and `retries: 3`, so a first start may migrate for the Server plan's full budget before a failed probe counts (architect-r1 5, critic-r1 5, logic-r1 3, threat-r1 6).
- **The settings of `server`:** `environment` is a mapping, so that the check script's `!reset null` can remove one key. It holds exactly these eleven keys, the Compose column of the Server plan's item 1 (architect-r1 1, critic-r1 1, logic-r1 1, threat-r1 1):

  | Key | Value |
  |---|---|
  | `LGTM_WEB_PASSWORD` | `${LGTM_WEB_PASSWORD:-}` |
  | `LGTM_LISTEN_ADDR` | `:8080` |
  | `LGTM_PUBLIC_URL` | `http://127.0.0.1:${LGTM_WEB_PORT:-8787}` (architect-r1 3, critic-r1 3, logic-r1 2) |
  | `LGTM_POSTGRES_HOST` | `postgres` |
  | `LGTM_POSTGRES_PORT` | `5432` |
  | `LGTM_POSTGRES_DATABASE` | `lgtm_app` |
  | `LGTM_POSTGRES_USER` | `lgtm_app` |
  | `LGTM_POSTGRES_PASSWORD_FILE` | `/run/lgtm/postgres.password` |
  | `LGTM_ALLOW_PRIVATE_STACK` | `true` |
  | `LGTM_OPENBAO_ADDR` | `http://openbao:8200` |
  | `LGTM_OPENBAO_CREDENTIAL_FILE` | `/run/lgtm/openbao.json` |

  The Server plan's item 1 writes this contract, this item writes the mapping, and the server reads it. `LGTM_ALLOWED_ORIGINS` and `LGTM_TLS_CERT_FILE` are absent. The server accepts `localhost` and `[::1]` at the same port as aliases of the numeric origin.
- **The mounts of `server`:** each is a bind in long syntax with `read_only: true` and `bind: { create_host_path: false }`. The sprint 1 file `server/openbao.json` of the state folder goes to `/run/lgtm/openbao.json`. The sprint 1 file `postgres-app.password` goes to `/run/lgtm/postgres.password`. The server mounts nothing else.
- **The service `web`:** `build` with `context: ..`, `dockerfile: deploy/website.Dockerfile` and `target: runtime`; `image: lgtm-web:local`; `user: "10003:10003"`; `read_only: true`; `tmpfs: [/tmp]`; `cap_drop: [ALL]`; `security_opt: [no-new-privileges:true]`; `restart: unless-stopped`; `networks: [edge, app]`; `depends_on: { server: { condition: service_healthy } }`. Its one port is `"127.0.0.1:${LGTM_WEB_PORT:-8787}:8080"`, with the address written in the file and not taken from a setting. It has no environment and no mount. Its health check is `["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8081/healthz"]`, with `start_period: 30s`, `start_interval: 1s`, `interval: 10s`, `timeout: 3s` and `retries: 3`.
- **The networks:** `edge` is an ordinary bridge network. `app` has `internal: true`. The sprint 1 networks `data` and `vault` stay as they are.
- **`deploy/example.env`:** two setting lines, each after one comment sentence: `LGTM_WEB_PASSWORD=` (empty) and `LGTM_WEB_PORT=8787`. Git already ignores `deploy/.env` through the root `.gitignore`, and it tracks `deploy/example.env`.
- **`deploy/test/web-testlib.sh`:** the fixtures of this plan's suites (architect-r1 2, critic-r1 4, logic-r1 4, threat-r1 5). It is sourced by items 4, 5 and 7. Its functions:
  - `web_fixture_create` exports `COMPOSE_PROJECT_NAME=lgtmtest-<8 random hexadecimal characters>`, `LGTM_STACK_STATE_DIR=<a new folder from mktemp -d>` and `COMPOSE_ENV_FILES=<fixture>/settings.env`. It writes that settings file with mode `0600`, holding `LGTM_WEB_PORT=<port>` and `LGTM_WEB_PASSWORD=<32 random hexadecimal characters>`. The port is a random number from 20000 to 39999 to which a Bash `/dev/tcp` connection fails. It unsets any `LGTM_WEB_PASSWORD` and `LGTM_WEB_PORT` inherited from the shell, and it refuses to run when `COMPOSE_PROJECT_NAME` would equal `lgtm`.
  - `web_fixture_cleanup` runs `docker compose -f deploy/compose.yaml down -v --remove-orphans -t 5` for the fixture's project alone, then removes the fixture's state folder through `docker run --rm` of the pinned Caddy image as user 0, with only that folder's parent mounted, because sprint 1's jobs leave files owned by other users there.
  - `web_fixture_stat <path>` prints the owner, the group and the mode of a path in the fixture's state folder through the same disposable container.
  - Every suite calls `web_fixture_cleanup` from a `trap` on exit.

Tests written first, in `deploy/test/stack-web.sh`.
Each test reads `docker compose -f deploy/compose.yaml config --format json` inside a fixture, unless it says otherwise.

| Test | What it asserts |
|---|---|
| `web_publishes_on_loopback_only` | With a settings file that sets no port, `services.web.ports` is one entry with `host_ip` `127.0.0.1`, `published` `8787`, `target` 8080 and `protocol` `tcp`. With `LGTM_WEB_PORT=18787`, `published` is `18787` and `host_ip` is still `127.0.0.1`. No other service has `ports`. |
| `server_has_no_route_out` | `services.server.networks` has exactly the keys `app`, `data` and `vault`, and each of the three networks has `internal: true`. `services.web.networks` has exactly `edge` and `app`, and `edge` is not internal. With an override file of the test, `docker compose run --rm --no-deps` of a service `target` (the pinned Caddy image running `caddy respond --listen :9000 ok` on `app`) and of a service `probe` (the pinned Caddy image on `app`, `data` and `vault`) shows that `probe` fetches `http://target:9000/` with `wget -T 3`, and that the same `wget -T 3` of `http://example.com/` exits with a status other than 0 within 10 seconds (critic-r1 12). |
| `server_settings_are_named` | With `LGTM_WEB_PORT=18787` and a fixture password, `services.server.environment` equals exactly the eleven keys and values above, with `LGTM_PUBLIC_URL` equal to `http://127.0.0.1:18787`. With a settings file that sets no port, `LGTM_PUBLIC_URL` is `http://127.0.0.1:8787`. Neither `LGTM_POSTGRES_URL` nor `LGTM_TLS_CERT_FILE` is a key. |
| `password_follows_the_checks` | Applying the override of `check_c2` in `deploy/test/checks.sh` removes `LGTM_WEB_PASSWORD` from `services.server.environment` and keeps the other keys. A value of `LGTM_WEB_PASSWORD` in the shell wins over the settings file, as `check_c4` needs. |
| `services_are_unprivileged` | For `server`, `user` is `10001:10001`; for `web`, it is `10003:10003`. Both have `read_only: true`, `cap_drop` equal to `[ALL]`, `security_opt` containing `no-new-privileges:true`, `tmpfs` containing `/tmp`, and `restart` equal to `unless-stopped`. Neither has `cap_add`, `privileged` or `pid`. |
| `server_mounts_are_read_only` | The server's binds are exactly the two above, each with `read_only: true` and `create_host_path: false`. No bind of `server` names `admin`, `bao` or a settings file. `web` has no `volumes`. |
| `server_waits_for_database_and_vault` | `services.server.depends_on` has `postgres` and `openbao`, each with `condition: service_healthy`, and `services.web.depends_on.server.condition` is `service_healthy`. The two health checks equal the commands and the timings above. |
| `images_are_built_from_the_pinned_files` | `services.server.build.context` equals the absolute path of the repository root, as `git rev-parse --show-toplevel` prints it, with the Dockerfile `deploy/server.Dockerfile` and the target `runtime`, and its `image` is `lgtm-server:local`. `services.web.build` names the same context, `deploy/website.Dockerfile` and `runtime`, and its `image` is `lgtm-web:local`. Every other service's `image` contains `@sha256:` (critic-r1 12). |
| `example_settings_list_every_setting` | `deploy/example.env` has the two setting lines above, with `LGTM_WEB_PASSWORD` empty, and every setting line follows a comment line. |

- **Risky:** yes, because it publishes the port and mounts the credentials. The mutation that must turn `web_publishes_on_loopback_only` red: change `127.0.0.1` in the port to `0.0.0.0`. A second mutation: add `edge` to the server's networks, which must turn `server_has_no_route_out` red. A third mutation: remove `LGTM_ALLOW_PRIVATE_STACK`, which must turn `server_settings_are_named` red.
- **Acceptance:** `bash deploy/test/stack-web.sh` prints nine `ok` lines and exits 0.
- **Public names:** the services `server` and `web`; the networks `edge` and `app`; the settings `LGTM_WEB_PASSWORD` and `LGTM_WEB_PORT`; the eleven server keys above; the mounts `/run/lgtm/openbao.json` and `/run/lgtm/postgres.password`; the file `deploy/example.env`; the file `deploy/test/web-testlib.sh` and its three functions.

**Work item 5. The launcher starts everything**

- **Files:** changes `deploy/up.sh` and `deploy/test/stack-vault.sh`. Creates `deploy/test/stack-up.sh`.
- **Needs:** item 4, the merged sprint 1 plan, and item 9 of `go-s2-server-signin` for the tests that run the real server. It can run at the same time as item 7.
- **Work area and complexity:** `infra`, medium. It adds three phases to a launcher that already has a fixed shape, and its failure paths decide what the owner sees.
- **Scope:** `stack/infra`.
- **Statements:** C1, C2, C4, C13, R9, the design's section "Sign-in and web sessions" (removing the password ends every session), and the design's section "The compose stack" (one command starts the stack).
- **What it reads:** the launcher reads no setting (decisions row 29; architect-r1 11, logic-r1 9, logic-r1 10, threat-r1 10). Its arguments come first: none, or `--until vault`. Any other argument exits 2 with `usage: deploy/up.sh [--until vault]`. It then looks at each file that `COMPOSE_ENV_FILES` names, comma-separated, or at `deploy/.env` when that is unset. When such a file exists and is a symbolic link, or grants any permission to its group or to others, the launcher exits 2 with `<file> must be readable only by its owner; run chmod 600 <file>` before any phase (threat-r1 2). It never prints a setting's value, and it runs with shell tracing off.
- **The phases:** every phase prints `phase <name> start` and `phase <name> done`, as in sprint 1, and the whole run holds sprint 1's lock on the state folder. The order is sprint 1's `database`, `seal` and `vault`, then `images`, `server`, `web` and `ready` (critic-r1 6). With `--until vault`, the launcher runs only `database`, `seal`, `vault` and `ready`, and `ready` prints only sprint 1's line `postgres and openbao are healthy`. That route needs no password, no port and no image of this plan.
  - `images` runs `docker compose -f deploy/compose.yaml build server web`.
  - `server` runs `docker compose -f deploy/compose.yaml up -d --wait --wait-timeout 540 --no-deps server`. A process that exits ends this wait within seconds (scouting, "Round 1 probes"). On a failure the launcher prints `docker compose logs --no-log-prefix --tail 20 server`, runs `docker compose stop server`, and exits 1.
  - `web` runs `docker compose -f deploy/compose.yaml up -d --wait --wait-timeout 90 --no-deps web`. On a failure whose output contains `address already in use`, it prints `the web port is in use; set LGTM_WEB_PORT in deploy/.env`. On any failure it exits 1 and leaves the other services running (logic-r1 9).
  - `ready` prints sprint 1's line, then `the website is at http://<address>/`, where `<address>` is what `docker compose -f deploy/compose.yaml port web 8080` prints (critic-r1 9).
- **Removing the password:** **UNREVIEWED MECHANISM** (architect-r1 4, critic-r1 7, logic-r1 5, threat-r1 4). The ordinary launcher is the removal path. When the owner deletes the password and runs it, Compose recreates `server` because its settings changed. The server then commits the end of every session, prints `Session revocation committed.` and `LGTM_WEB_PASSWORD is missing or invalid.`, and exits, as step 4 of the Server plan's item 9 promises. The phase `server` fails, prints those lines, and stops the server. A run with a password starts the server again, and no cookie from before the removal works. A failed revocation prints the Server plan's `Session revocation was not confirmed.`, and the server stays stopped.
- **`stack-vault.sh`:** its calls of `deploy/up.sh` become `deploy/up.sh --until vault`, as the sprint 1 contract assigns to this plan.

Tests written first, in `deploy/test/stack-up.sh`.
Each runs in its own fixture of `deploy/test/web-testlib.sh`.

| Test | What it asserts |
|---|---|
| `up_until_vault_needs_no_web_settings` | With no password in the shell or the settings file, `deploy/up.sh --until vault` exits 0. Its `phase` lines name exactly `database`, `seal`, `vault` and `ready`, and its last line is `postgres and openbao are healthy`. The project has no `server` and no `web` container, and no image was built during the run. (critic-r1 6, logic-r1 4, threat-r1 5) |
| `up_starts_everything_in_order` | From an empty fixture, with the password only in the shell and a settings file that holds only the port, the launcher exits 0. Its `phase` lines come in the order `database`, `seal`, `vault`, `images`, `server`, `web`, `ready`, each `start` before its `done`. Its last line is `the website is at http://127.0.0.1:<port>/`. `docker compose ps` shows the four services running and healthy. `GET /` on the port contains `id="app"`. `POST /lgtm.v1.SessionService/SignIn` with the password, with `Origin: http://127.0.0.1:<port>` and again with `Origin: http://localhost:<port>`, each returns a `Set-Cookie` header. (architect-r1 3, critic-r1 3, logic-r1 2) |
| `up_starts_no_server_before_the_vault` | In a fixture whose published vault credential is malformed, the launcher exits with a status other than 0 during the phase `vault`, prints sprint 1's line `server vault credential needs repair`, and `docker compose ps -a` lists no `server` and no `web` container. This is check PB8 of the sprint 1 plan. |
| `up_waits_for_a_slow_server` | An override file replaces `server` with the pinned Caddy image, whose command sleeps 150 seconds before it answers `/healthz` on port 8080, and whose health check test is a `wget` of that address, with the timings unchanged. The launcher exits 0, and more than 150 seconds pass between `phase server start` and `phase server done`. (architect-r1 5, critic-r1 5, logic-r1 3, threat-r1 6) |
| `up_stops_on_a_failed_server` | An override file replaces the server's command with one that prints `LGTM_WEB_PASSWORD is missing or invalid.` on standard error and exits 1. The launcher exits 1 within 60 seconds of `phase server start`, prints that sentence, leaves `server` stopped, and leaves no `web` container. |
| `up_names_a_taken_port` | While a test container publishes the fixture's port on `127.0.0.1`, the launcher exits 1 during the phase `web` and prints `the web port is in use; set LGTM_WEB_PORT in deploy/.env`. `postgres`, `openbao` and `server` stay healthy. After the test container stops, a second run reaches the last line. (architect-r1 11, logic-r1 9) |
| `up_refuses_an_open_settings_file` | With the settings file at mode `0644`, and again as a symbolic link to a file at mode `0600`, the launcher exits 2, prints the mode line above naming that file, prints no `phase` line, and leaves no container. (threat-r1 2) |
| `up_is_idempotent` | A second run exits 0. The container identifiers of `server` and `web`, and the identifiers of their images, are the same as after the first run. |
| `up_password_removal_ends_sessions` | After a full start, the test signs in and keeps the cookie, and `POST /lgtm.v1.ClientService/WatchClients` with that cookie returns the Connect code `unimplemented`. It removes the password from the settings file and runs the launcher, which exits 1 and prints `Session revocation committed.`. It restores the same password and runs the launcher, which exits 0. The same call with the old cookie now returns `unauthenticated`. |

- **Risky:** yes, because it decides the order in which the server receives its credential and what happens to sessions. The mutation that must turn `up_until_vault_needs_no_web_settings` red: run `images` before `database`. A second mutation: set `--wait-timeout 120` in the phase `server`, which must turn `up_waits_for_a_slow_server` red. A third mutation: remove `docker compose stop server` from the failure path, which must turn `up_stops_on_a_failed_server` red.
- **Acceptance:** `bash deploy/test/stack-up.sh` prints nine `ok` lines and exits 0, and `bash deploy/test/stack-vault.sh` still passes.
- **Public names:** the phase names `images`, `server` and `web`; the option `--until vault`; the usage line; the mode line; the port line; the final line `the website is at http://<address>/`.

**Work item 6. The operator guide and the sign-in checks**

- **Files:** changes `deploy/README.md`. It changes no test.
- **Needs:** items 4 and 5, and the merged plan `go-s2-server-signin`.
- **Work area and complexity:** `infra`, low. It writes a guide, and it runs checks that already exist.
- **Scope:** `stack/infra`.
- **Statements:** C1, C2, C3, C4 and C13.
- **The guide:** `deploy/README.md` gains these sections, after sprint 1's.
  - "Start everything" says to run `install -m 600 deploy/example.env deploy/.env`, set the password in it, run `deploy/up.sh`, and open the printed address.
  - "Settings" is a table of the two settings, their defaults and their effect. It says that `LGTM_WEB_PORT=0` is not supported.
  - "When port 8787 is taken" says that the old engine uses 8787 and that `LGTM_WEB_PORT` picks another.
  - "Remove the password" says to delete it from `deploy/.env` and run `deploy/up.sh`. The launcher then prints `Session revocation committed.` and stops the server. If it prints `Session revocation was not confirmed.`, the owner runs it again before setting any password.
  - "Reach the stack from another computer" says to forward the loopback port, for example with a Secure Shell (SSH) port forward, and that a public address comes with a later proxy.
- **Tests written first:** the user-flow script `deploy/test/checks.sh`, which the orchestrator's test writer wrote before any plan. Its checks C1 to C4 and C13 are red today, because `deploy/up.sh` and the services do not exist. This item changes no line of it (decisions row 24).
- **Risky:** no. It adds no code.
- **Acceptance:** on a clean machine, which means a fresh clone at the merge commit, with no image tagged `lgtm-server:local` or `lgtm-web:local`, no Compose project `lgtm` and no state folder:
  1. `install -m 600 deploy/example.env deploy/.env`.
  2. Set a password made only for the check, `export LGTM_WEB_PASSWORD=$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')`, write the same value into `deploy/.env`, and run `deploy/test/checks.sh --only C1,C2,C3,C4,C13`. It prints `ok 1`, `ok 2`, `ok 3`, `ok 4` and `ok 13` and exits 0 (threat-r1 2).
  3. A person performs criterion 2d.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The server gives up or is declared unhealthy while a first start migrates its database. | High | The health check's eight-minute start period, the launcher's 540-second wait, and `up_waits_for_a_slow_server`. |
| A password removal leaves the old sessions alive. | High | The ordinary launcher recreates the server, which ends the sessions before it exits, and `up_password_removal_ends_sessions`. |
| The server starts before the vault has published its credential, or with another installation's credential. | High | `depends_on` with `service_healthy`, the launcher's phase order, the sprint 1 binding checks, and `up_starts_no_server_before_the_vault`. |
| The Server team changes a setting name. | Medium | `server_settings_are_named` pins the eleven keys, and a change of the Server plan's table is a change of that test. |
| Another user of the same host reads the password from the settings file. | Low | The guide makes the file with mode `0600`, and the launcher refuses a file that others can read. Wave 1 has one user. |
| Port 8787 is taken by the old engine on the development machine. | Low | The launcher names the setting, and the checks honour `LGTM_WEB_PORT`. |

**Verify:** `bash deploy/test/stack-web.sh`, `bash deploy/test/stack-up.sh`, `bash deploy/test/stack-vault.sh` and `deploy/test/checks.sh --only C1,C2,C3,C4,C13` on Linux with Docker, against real PostgreSQL and OpenBao. A person signs in once in a browser (criterion 2d).

**Rollout detail:** step 3 builds items 4 and 5 after `go-s1-stack-vault` is merged, and item 5's tests of the real server pass after Server item 9. Step 4 runs item 6 after `go-s2-server-signin` is merged.

**WS-3: The link's long stream passes through the web server**

Issue 1633 asks for proof that the web server passes the link's long two-way stream on without cutting it, before sprint 3 depends on it.
The link uses the procedure `/lgtm.link.v1.LinkService/Open`, a stream in both directions, which the contract plan names.
A machine reaches a loopback address over cleartext HTTP/2, so the web server must accept HTTP/2 without encryption from the machine and pass the stream on in the same way.
A stand-in program, built from the standard library of Go alone, plays the server and the machine, so the proof needs no part of sprint 3.

The probes of 2026-11-26 found a real limit (scouting, "Which limit cuts a silent stream" and "Round 1 probes").
Caddy 2.11.9 with default settings cut a stream whose upward direction, from the machine to the server, stayed silent for 60 seconds, even while the server kept sending down.
The setting `read_body_idle 10m` of item 2 removes that cut, and a six-minute soak passed with it.
The link's client sends a heartbeat every 10 seconds, so the real link would stay inside even the default limit.
The tests still hold each direction silent for longer than a minute, so that a later image or rule that brings the limit back fails them before it ships.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 3a | A two-way stream through the real web server rules carries messages both ways, survives 35 seconds of silence in both directions, ends with its trailer, and uses HTTP/2 on both hops. Each direction alone stays silent for 80 seconds while the other keeps sending, and the stream then still carries a message both ways and ends with its trailer. A one-way stream to a browser arrives message by message for 80 seconds. | `bash deploy/test/link-stream.sh` prints `ok` for its six tests, with the soak test reported as skipped. The early form, `bash deploy/test/link-stream.sh --rules-only`, does the same against the rules of item 2. |
| 3b | On the merged website image, a stream stays open through 6 minutes of silence in both directions, and through 6 minutes and 30 seconds of silence in each direction alone, before sprint 3 starts. | `LGTM_STREAM_SOAK=1 bash deploy/test/link-stream.sh` prints `ok` for all six tests, and the sprint's report records the output. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 7 | Write the stand-in program and the stream test. | Target `deploy/test/streamprobe/main.go`, functions `serve`, `dial` and `shaped`; the nearest pattern is the probe in scouting, "Stream probe". Target `deploy/test/link-stream.sh`. The procedure name: `.plans/plans/go-s0-contract/plan.md`, item 6, `rpc lgtm.link.v1.LinkService.Open`. The rules under test: `deploy/Caddyfile`, snippet `(app)` and the global `timeouts`. |

**Design artifact:** none.

**Work item 7. The stream test**

- **Files:** creates `deploy/test/streamprobe/main.go` and `deploy/test/link-stream.sh`. Its builder may change `deploy/Caddyfile` when a test fails, once item 2 is merged, and no other file of another item.
- **Needs:** item 2 for `--rules-only`, and items 3 and 4 for the full run (architect-r1 6, critic-r1 8, logic-r1 6, threat-r1 8). Its tests can be written at once. It can run at the same time as items 1, 4 and 5.
- **Work area and complexity:** `infra`, medium. The program is small, but the test drives a real proxy through timing.
- **Scope:** `stack/infra`.
- **Statements:** the design's sections "One connection, dialled by the machine", "Knowing whether a machine is there" (a heartbeat every 10 seconds) and "Encryption on the link", C8 and C9, and issue 1633, point 4.
- **The stand-in program:** package `main`, standard library only, in the root module, so `go build ./...` and `go vet ./...` include it. The test builds it inside `golang:1.26.11-alpine@sha256:ecb67f98…` with `CGO_ENABLED=0`, so the host's Go version does not matter. Its flags:
  - `-mode serve -addr :8080` listens with HTTP/1.1 and cleartext HTTP/2 through `http.Server.Protocols`. It logs `upstream proto=<protocol> path=<path>` for each request. The header `Probe-Shape` selects what it does:
    - With no header, it enables full duplex, sends status 200 with the header `Trailer: Probe-End`, echoes each line of the request body as `echo <line>` with a flush, and sets the trailer `Probe-End` to the number of lines when the body ends.
    - With `quiet-down`, it reads every line and writes nothing until the body ends, then writes `count <n>`.
    - With `quiet-up`, it writes `tick <n>` every 2 seconds, with a flush. When a line arrives after the first, it writes `ack <line>` with a flush. When the request body ends, it sets the trailer `Probe-End` to `up-closed`.
    - With `ticks`, it reads the whole body first, then writes `tick <n>` every 2 seconds with a flush after each, as many times as the header `Probe-Ticks` says, with 5 as the default.
  - `-mode dial -url <address>` sends `m0` and reads its echo, then sends `m<n>` every `-every` until `-total` has passed, reading each echo before the next send. It then stays silent for `-gap`, sends one more line, reads its echo, closes its side, and checks that nothing more arrives and that the trailer equals the number of lines. It prints `proto <protocol>` and, at the end, `ok stream lasted <duration> with <n> messages each way`. Any missing or wrong echo prints `not ok` with the message number and exits 1.
  - `-mode shaped -shape quiet-down -total <d> -url <address>` sends a line every 2 seconds for the duration, closes its side, and expects `count <n>` with the right number. `-mode shaped -shape quiet-up -total <d> -url <address>` sends one line, stays silent and reads ticks for the duration, then sends `m1`, waits up to 5 seconds for `ack m1`, closes its side, and requires the trailer `up-closed` (logic-r1 6, threat-r1 8). Each prints one `ok` or `not ok` line and exits 0 or 1.
- **The two fixtures:**
  - `--rules-only` runs the pinned Caddy image with `deploy/Caddyfile` mounted, as item 2's tests do, and the stand-in in the pinned distroless image with the network alias `server`, on a disposable network that the test creates and removes. It needs no Compose file and no image of this plan.
  - The full run uses a fixture of `deploy/test/web-testlib.sh` and an override file of the test (architect-r1 7, logic-r1 6, threat-r1 8). The override sets on `server`: `build: !reset null`; `image:` the pinned distroless image; `entrypoint: ["/probe"]`; `command: ["-mode", "serve", "-addr", ":8080"]`; `depends_on: !reset []`; `environment: !reset {}`; `healthcheck: !reset null`; and `volumes:` replaced by one read-only bind of the built program at `/probe`. It sets `depends_on: !reset []` on `web`. The test runs `docker compose up -d --no-deps server web` and requires that `docker compose config --format json` shows no `build` on `server`. `web` is the real image of item 3.
  - Every client run uses the host's network and the published loopback port, so the path through Docker's published port is part of the test. Every run of the program has a `timeout` of its expected duration plus 60 seconds, and a run that reaches it is `not ok`.

Tests written first, in `deploy/test/link-stream.sh`:

| Test | What it asserts |
|---|---|
| `stream_passes_both_ways_loopback` | `dial` against `http://127.0.0.1:<port>/lgtm.link.v1.LinkService/Open` with `-every 2s -total 20s -gap 35s` exits 0 and prints `ok`, with at least 12 messages each way. |
| `stream_hops_speak_http2` | `dial` prints `proto HTTP/2.0`, and the stand-in's log holds `upstream proto=HTTP/2.0 path=/lgtm.link.v1.LinkService/Open`. |
| `stream_survives_upward_silence` | `shaped -shape quiet-up -total 80s` exits 0 and prints `ok` with at least 38 ticks, the `ack m1` and the trailer. This is the test that fails at 60 seconds on Caddy 2.11.9 without `read_body_idle`. |
| `stream_survives_downward_silence` | `shaped -shape quiet-down -total 80s` exits 0 and prints `ok`. |
| `server_stream_arrives_message_by_message` | `curl --http1.1 --no-buffer` posts an empty body with `Probe-Shape: ticks` and `Probe-Ticks: 40` to `/lgtm.v1.ClientService/WatchClients`. The test records the arrival time of each line. All 40 `tick` lines arrive, with at least 1.5 seconds between each pair. This is how the website receives `WatchClients`. |
| `stream_survives_soak` | Only when `LGTM_STREAM_SOAK=1`: `dial` with `-every 2s -total 4s -gap 6m`, `shaped -shape quiet-up -total 6m30s` and `shaped -shape quiet-down -total 6m30s` run at the same time, and each exits 0 and prints `ok`. Otherwise the line is `ok 6 - stream_survives_soak # SKIP soak not requested`. |

- **Risky:** no. It touches no key, token or order of records. Its strength is still checked once by the build lead: removing `read_body_idle 10m` from `deploy/Caddyfile` must turn `stream_survives_upward_silence` red within its deadline.
- **Acceptance:** `bash deploy/test/link-stream.sh --rules-only` and `bash deploy/test/link-stream.sh` each print six `ok` lines, one of them skipped, and exit 0. Before sprint 3 starts, `LGTM_STREAM_SOAK=1 bash deploy/test/link-stream.sh` prints six `ok` lines with none skipped.
- **Public names:** the program path `deploy/test/streamprobe/`, its modes and flags above, the trailer `Probe-End`, the headers `Probe-Shape` and `Probe-Ticks`, the option `--rules-only`, and the setting `LGTM_STREAM_SOAK`. Sprint 3 may reuse the program and the override to test the real link through the web server.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A limit of the web server cuts a stream whose upward direction is silent. Caddy 2.11.9 does so at 60 seconds by default. | High | `read_body_idle 10m` in the rules, `stream_survives_upward_silence` on every run, and the link's 10-second heartbeat. Request L1 asks the link plans to keep that heartbeat. |
| The web server buffers a one-way stream, and the website sees changes late. | Medium | `flush_interval -1` and `server_stream_arrives_message_by_message`. |
| The stand-in behaves unlike the real link. | Medium | It uses the same transport, path, protocol and full duplex. Sprint 3's link tests run the real client through the same rules, and check PB3 covers the loss of a peer. |

**Verify:** `bash deploy/test/link-stream.sh --rules-only` and `bash deploy/test/link-stream.sh` on Linux with Docker, and once with `LGTM_STREAM_SOAK=1` before sprint 3. No device is needed.

**Rollout detail:** step 1 writes the tests and runs `--rules-only` against item 2. Step 5 runs the full test and the soak on the merged sprint.

</details>

<details><summary><b>Post-build checks from round 1</b></summary>

Each check is named PB, for post-build, with a number. The checks run once the code exists.

| Check | Scenario | Expected observation |
|---|---|---|
| PB1. Clean launch and the rollout driver's deadlines. | Stack and Server run `deploy/test/checks.sh --only C1,C2,C3,C4,C13` from a fresh clone with an empty Docker cache, then once more at `LGTM_WEB_PORT=18787`. They record how long C2's exit and the restarts of C4 and C13 take. (architect-r1 5, critic-r1 5) | Five passing lines each time. C2 exits within the driver's 20 seconds and each restart is healthy within its 60 seconds, because the database is already migrated. A longer time becomes a request to the driver's owner. |
| PB2. Home in the browser. | Once a `WatchClients` handler serves its first snapshot, a person signs in at `http://localhost:18787/`. (critic-r1 2) | The Home screen appears, with `localhost:18787` at the foot of the sidebar. |
| PB3. A stream that loses a peer. | During a long stand-in stream through the merged web image, the test stops the stand-in server, and in another run it kills the stand-in client. (architect-r1 8) | The client sees its stream end within 10 seconds of the server's stop. The stand-in server's request ends within 10 seconds of the client's death. Sprint 3 repeats this with the real link. |
| PB4. Secrets on the host. | With a fixture password and a running fixture, read `ps` output during startup and sign-in, the image histories, the launcher's output and both services' logs. (threat-r1 2) | None contains the fixture password or the content of a credential file. |
| PB5. The C4 replacement password at the size limit. | Run C4 with an original password of 4,096 bytes. (architect-r1 12, logic-r1 11) | The driver's replacement stays within the server's limit, or the driver's owner changes its generator. Request D2 in the result file asks for that. |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe: the brief's four work items, its three observations and its two points to settle (`.plans/plans/go-s2-stack-web/plan.md` as written on 2026-11-25, preserved in `scouting.md`); the task file's three specific demands (`.foundry/go-rewrite/tasks/plan-go-s2-stack-web.md`); the design's section "The compose stack"; issue 1633, points 1 to 4; the names in `deploy/test/README.md` that touch this plan; the sprint 1 plan's "Contract for sprint 2" and its checks PB4 and PB8; the Server plan's settings table and startup order; and the statements C1, C2, C3, C4, C9, C13, D1, D17 and R9 of the design.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the server image | WS-1 | Item 1. |
| Brief item 2, the website image and the web server rules | WS-1, WS-3 | Items 2, 3 and 7. |
| Brief item 3, the compose services, the settings file and the port variable | WS-2 | Item 4. |
| Brief item 4, the one command and the sign-in part of the user-flow script | WS-2 | Items 5 and 6. |
| Observation: one command starts the stack, and the website opens on port 8787 | WS-2 | Items 5 and 6, check C1. |
| Observation: the owner signs in | WS-2 | Item 6, check C3 and criterion 2d; Home is PB2. |
| Observation: loopback only unless a public address is set, which must be HTTPS | WS-2 | Item 4 publishes on `127.0.0.1` only. A public address is outside wave 1 (decisions row 27). |
| Point 1: how a public HTTPS address is turned on, and where its certificate comes from | none | Outside wave 1, decisions row 27. |
| Point 2: where encryption ends for the link | none | Wave 1 has no encryption on loopback. The choice is for the owner, decisions row 36. |
| Task: issue 1633, a check that the web server passes the long stream on before sprint 3 | WS-3 | Item 7, criteria 3a and 3b. |
| Task: one command on a clean machine, and the website at the loopback address | WS-2 | Item 6 acceptance. |
| Task: pin every image, and bind ports to loopback | WS-1, WS-2 | Items 1, 3 and 4; decisions rows 8 and 32. |
| Design, "The compose stack": four services, one address, loopback default, port variable 8787, one command | WS-1, WS-2 | Items 2 to 6. |
| Issue 1633, points 1 to 3: the certificate's fingerprint, the machine's certificate, the challenge | none | Outside wave 1 with the public address; decisions rows 27 and 36. |
| Issue 1633, point 4: the long stream is not cut | WS-3 | Item 7. |
| `deploy/test/README.md`: the compose file, the start command, the four service names, the address, `LGTM_WEB_PORT`, `LGTM_WEB_PASSWORD`, the default server command, the website's mount point and its API | WS-1, WS-2 | Kept unchanged; items 1, 3, 4 and 5. |
| `deploy/test/README.md`: `POSTGRES_USER` and `POSTGRES_DB` for C5 | none | Sprint 1 item 2 owns the C5 query (decisions row 24). |
| Sprint 1 "Contract for sprint 2": user 10001, the networks, the database settings and files, the vault file, the launcher, the fixtures | WS-1, WS-2 | Items 1, 4 and 5. |
| Sprint 1 check PB8: the web launcher's hand-over | WS-2 | `up_until_vault_needs_no_web_settings`, `up_starts_everything_in_order`, `up_starts_no_server_before_the_vault`. |
| Sprint 1 check PB4: the restart of the whole stack | WS-2 | Item 6, check C13. |
| Server plan item 1, the settings table | WS-2 | `server_settings_are_named`. |
| Server plan item 9, the startup order and the eight-minute wait | WS-2 | Item 4's health check and item 5's phase `server`. |
| C1 | WS-2 | Items 5 and 6. |
| C2 | WS-1, WS-2 | `server_image_refuses_without_password`, `password_follows_the_checks`, `up_stops_on_a_failed_server`, item 6. |
| C3 | WS-1, WS-2 | `rules_keep_status_and_body`, `up_starts_everything_in_order`, item 6, criterion 2d. |
| C4 | WS-2 | `password_follows_the_checks`, `up_password_removal_ends_sessions`, item 6. |
| C9 | WS-3 | Item 7. |
| C13 | WS-2 | Item 6. |
| D1, three parts that meet only through the contract | WS-1 | Items 1 to 3. |
| D17, no certificate needs a public authority | none | No certificate exists in wave 1 (decisions row 27). |
| R9, secrets never reach a log or the website | WS-1, WS-2 | The allow-lists, the read-only mounts, no access log, the settings file's mode, and PB4. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "What this project is": the three parts meet only through the contract in `src/contracts/`. | The web server passes on only the contract's two packages, and no file of this plan reads the server's code. |
| `AGENTS.md`, "Where things are": new code does not go into `packages/`, `design/mock/` or `plugins/`. | Every file of this plan is under `deploy/`. The website image reads `design/mock/` only as the existing build already does. |
| `AGENTS.md`, habit 1: copied code is credited. | No code is copied. The web server rules and scripts are written for this plan. |
| `AGENTS.md`, habit 2: findings go into `docs/issues.md` and ideas into `docs/features.md`. | This plan may edit neither file, so the result file lists the items for the orchestrator to record. |
| `AGENTS.md`, habit 3: finished work reaches the base branch through a pull request. | The plan makes no commit. Each work item merges inside the team's branches after a passing check on its exact commit. |
| `AGENTS.md`, writing style: no quotation of a person, no home folder path, no private network address. | The plan names only loopback addresses, `0.0.0.0` in a mutation, and the reserved names `lgtm.test` and `example.com`. `scripts/check-private.mjs` passes. |
| The epic's team table: the Stack team edits only `deploy/`. | Every changed file is under `deploy/`. Needs of other teams are requests in the result file. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet (its harness-session skill) at launch; this section names none (owner, 2026-10-16).

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Server image | `infra` | `stack/infra` | low | One Dockerfile and its allow-list, in a known shape, with one test script. | no | Server item 9, for its build. |
| 2. Web server rules | `infra` | `stack/infra` | medium | One short file that decides which requests reach the server, tested with a stub. | yes | Nothing. |
| 3. Website image | `infra` | `stack/infra` | medium | A workspace build with an allow-list of thirteen paths. | no | Item 2. |
| 4. Compose services, settings and fixtures | `infra` | `stack/infra` | medium | Two services in a set pattern, with credential mounts that sprint 1 defines, and the shared fixtures. | yes | Items 1 and 3, and `go-s1-stack-vault`. |
| 5. Launcher | `infra` | `stack/infra` | medium | Three phases and their failure paths added to a launcher of fixed shape. | yes | Item 4, and Server item 9 for its tests of the real server. |
| 6. Guide and sign-in checks | `infra` | `stack/infra` | low | A guide, and checks that already exist. | no | Items 4 and 5, and `go-s2-server-signin`. |
| 7. Stream test | `infra` | `stack/infra` | medium | A small program and a timed test through a real proxy. | yes | Item 2 for `--rules-only`; items 3 and 4 for the full run. |

Items 2 and 7 start at once. Item 3 follows item 2, and item 1 follows Server item 9.
Item 4 follows items 1 and 3, item 5 follows item 4, and item 6 comes last.
Item 7's full run follows items 3 and 4.

Load: seven test writers and seven builders give 14 seats, and the mutation checks of items 1, 2, 3, 4, 5 and 7 add six, for 20 execution seats. Alternating the two primary vendors gives 10 seats to each. Every item is `infra` at low or medium, so no seat needs the strongest tier. The orchestrator reads each vendor's remaining weekly allowance at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | 8 / 34 / 4 from four reviewers, 16 groups once duplicates are merged | Folded; see `attack/round-1-findings.md`. Public mode is cut to outside wave 1, the launcher no longer reads settings, and the stack keeps Caddy 2.11.9 with `read_body_idle 10m`. |

</details>
