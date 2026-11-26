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

By default the stack publishes its port on the loopback address only, at `http://127.0.0.1:8787`.
In that mode the web server also refuses any host name other than a loopback name, so a page never loads over plain HyperText Transfer Protocol (HTTP) from another computer.
The owner can turn on a public address with a setting, and that address must use HTTPS, the encrypted form of HTTP.
In that mode the stack makes one certificate of its own, and it keeps that certificate's key for good.
A browser shows a warning once for that certificate, because no public authority signed it, which the design allows.

The plan also settles issue 1633 in `docs/issues.md`, which asks where encryption ends for the link between the server and a machine.
In public mode the encryption ends at the web server, and the server reads the public half of the certificate to learn the fingerprint that a join token must carry.
The link is one long two-way stream, so the plan proves that the web server passes such a stream on without cutting it.
The probes of 2026-11-26 found that Caddy 2.11.9 cuts such a stream once the machine's side has been silent for about 60 seconds, and that Caddy 2.10.2 does not.
The plan therefore pins Caddy 2.10.2, tests silences longer than that limit in each direction, and runs a six-minute soak before sprint 3 builds the link.

Two points depend on other plans.
The server must read the settings this plan names, and it must offer a command that reports its health; both are requests to the Server team's plan.
The sprint 1 launcher must gain an option to stop after the vault, so that its own tests keep passing once the launcher also starts the server.

## Key decisions

**One address for the website and the server.** The web server passes three kinds of path to the server: the website's calls, the link's calls, and the frames of a codebase map. Everything else is a file of the built website.

**Loopback unless the owner says otherwise.** The published port binds to `127.0.0.1` by default. A wider bind needs two settings, a public `https://` address and the bind itself, and the launcher refuses the bind without the address.

**The stack makes its own certificate.** The key is made once and never replaced, because every machine that joins keeps the fingerprint of that key. A certificate of the owner's own, and certificates from a public authority, wait for a later wave.

**Encryption ends at the web server.** Inside the stack, the web server talks to the server over a private network that only those two services join. The server has no route out of the stack.

**The settings live in one file.** The owner copies `deploy/example.env` to `deploy/.env` and sets the password there, or sets it in the shell. The launcher refuses to start without a password, and it never makes one up.

**Images are built from an allow-list.** Each image builds from the repository, but a list next to its build file lets in only the folders it needs. The state folder, which holds the vault's key and credentials, therefore never reaches an image.

Full log → `decisions.md`

## Workstreams

### The server and the website become images

The server becomes one small image that holds the program and nothing else, not even a shell.
The website is built inside a second image and served by the web server, with rules that pass the calls on to the server.
The main risk is that a secret from the state folder could slip into an image, and each image's tests plant a decoy secret to prove it cannot.
When this is done, each image builds with one command, runs as an ordinary user without any special privilege, and passes its own tests.

### One command starts everything, and the owner signs in

The compose file gains the services `server` and `web`, and the launcher of sprint 1 gains the steps that build and start them.
The main risks are a port published beyond loopback by mistake, and a server that starts before the vault has handed it its credential.
Tests read the resolved compose file, and the launcher's tests start disposable stacks and break them on purpose.
When this is done, `deploy/up.sh` on a clean machine starts all four services, the website opens at `http://127.0.0.1:8787`, and the owner signs in.

### The link's long stream passes through the web server

The link between the server and a machine is one stream that stays open for days, with messages in both directions.
No link exists yet, so a small stand-in program plays both ends through the real web server rules.
The risk is real: Caddy 2.11.9 cuts a stream whose upward direction is silent for about 60 seconds, so the plan pins Caddy 2.10.2.
The tests hold each direction silent for 80 seconds, and the soak holds a stream silent for 6 minutes.
When this is done, the stream test passes, and the long run has passed once before sprint 3 starts.

### A public HTTPS address

The owner sets a public `https://` address, and the stack makes a certificate for it and serves only encrypted traffic on its port.
The risk is a lost or replaced key, which would cut off every machine that joined, so the key is made once and kept private to the web server.
When this is done, the website and the stand-in stream are reached over HTTPS with the certificate pinned by its key, and the session cookie is marked secure.

## Rollout order

1. Build the rules of the web server and the stream test first. They need no other plan, and the stream test is the proof that sprint 3 depends on.
2. Build the two images. The server image needs the start of the server program from the Server team's sprint 2 plan.
3. Add the two services to the compose file and extend the launcher. This needs the sprint 1 stack to be merged, and the option of sprint 1's launcher to stop after the vault.
4. Run the sign-in checks of the user-flow script on a clean machine once the Server team's plan is merged, and update the guide in `deploy/README.md`.
5. Add the public HTTPS mode last. It changes nothing in loopback mode.
6. Run the six-minute stream soak once on the merged work, before sprint 3 builds the link, and record its result in the sprint's report.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

All paths are relative to the repository root.
Every test file named here is a Bash script under `deploy/test/` that prints one Test Anything Protocol (TAP) line for each named assertion, in the form `ok <n> - <test name>` or `not ok <n> - <test name>`, and exits with status 0 only when every line is `ok`.
This is the format of the sprint 1 suites.
A test that needs a disposable stack uses `fixture_create` and `fixture_cleanup` from `deploy/test/vault-testlib.sh`, which sprint 1 writes, so that each run has its own Compose project, state folder and loopback port.
A test never reads or changes the developer's own `deploy/.env` or `deploy/.state/`. It passes its settings with `--env-file <fixture file>` and `LGTM_STACK_STATE_DIR`.
The image digests are those of decisions row 8.

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
| 1b | The web server rules serve the website, pass the three path prefixes on over HTTP/2, keep status and body unchanged, and refuse a host name that is not loopback before the server sees the request. | `bash deploy/test/web-rules.sh` prints `ok` for its seven tests. |
| 1c | The website image builds the website with the server address `/`, serves it as user 10003 with no capability, and holds no build tool and no file from the state folder. | `bash deploy/test/web-image.sh` prints `ok` for its eight tests. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1 | Write the server image and its allow-list. | Target `deploy/server.Dockerfile`, stages `build` and `runtime`; target `deploy/server.Dockerfile.dockerignore`. Nearest pattern: `design/openpencil/Dockerfile`, multi-stage build ending in a pinned runtime. The program is `src/server/cmd/lgtm-server/main.go`, planned in `floorplan/plan.json` entry `deploy/server.Dockerfile`. |
| 2 | Write the web server rules. | Target `deploy/Caddyfile`, snippet `(app)`; target `deploy/caddy/site.caddy`. Nearest pattern: `design/openpencil/Caddyfile`, global block and `handle`. Paths to pass on: `src/frontend/vite.config.ts`, constant `behind`. |
| 3 | Write the website image and its allow-list. | Target `deploy/website.Dockerfile`, stages `site` and `runtime`; target `deploy/website.Dockerfile.dockerignore`. The server address: `src/frontend/src/server/address.ts`, `fixedAddress` and `VITE_LGTM_SERVER`. The build: `src/frontend/package.json`, script `build`. |

**Design artifact:** none. No screen changes; the website is built as it is.

**Work item 1. The server image**

- **Files:** creates `deploy/server.Dockerfile`, `deploy/server.Dockerfile.dockerignore` and `deploy/test/server-image.sh`.
- **Needs:** nothing to write its tests. Its build and its last test need `src/server/cmd/lgtm-server/main.go`, which work item 1 of `go-s2-server-signin` creates. It can run at the same time as items 2 and 7.
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

- **Files:** creates `deploy/Caddyfile`, `deploy/caddy/site.caddy` and `deploy/test/web-rules.sh`.
- **Needs:** nothing. It can run at the same time as items 1 and 7.
- **Work area and complexity:** `infra`, medium. The rules are short, but they decide which requests reach the server and how a stream is passed on.
- **Scope:** `stack/infra`.
- **Statements:** D1, C1, C3, R8, and the design's section "The compose stack" (the website and the server share one address, and the website's server address is `/`).
- **What the rules say:** `deploy/Caddyfile` has a global block with `admin off`, `auto_https off`, `persist_config off`, `grace_period 5s`, and `servers { protocols h1 h2 h2c }`. It sets no timeout. It defines the snippet `(app)`. In it, the matcher `@server path /lgtm.v1.* /lgtm.link.v1.* /frames/*` is handled by `reverse_proxy h2c://server:8080 { flush_interval -1 }`, which passes each call on as cleartext HTTP/2 (h2c), and every other path is handled by `root * /srv/site` and `file_server`. It defines the site `http://:8081` with `bind 127.0.0.1` and `respond /healthz 200`. It ends with `import /etc/caddy/site{$LGTM_WEB_TLS}.caddy`. The file `deploy/caddy/site.caddy` defines the site `http://127.0.0.1:8080, http://localhost:8080, http://[::1]:8080` with `import app`, and the site `http://:8080` with `respond 421`. No access log is written, because requests carry the session cookie. Scouting, "Web server rules", records the probe of this exact text.

Tests written first, in `deploy/test/web-rules.sh`.
Each test runs the pinned Caddy image with its default user, a temporary file system at `/tmp`, `XDG_DATA_HOME=/tmp` and `XDG_CONFIG_HOME=/tmp`.
The unprivileged start is tested in item 3, because the stock image's binary needs the copy step of item 3 to start without capabilities (decisions row 26).
It mounts `deploy/Caddyfile` at `/etc/caddy/Caddyfile` and `deploy/caddy/site.caddy` at `/etc/caddy/site.caddy`, and a test folder holding `index.html` (with `id="app"` and a `<script>` element) and `assets/a.js` at `/srv/site`.
It publishes port 8080 on `127.0.0.1` at a port Docker chooses.
A stub server, a second container of the pinned Caddy image with the network alias `server`, listens on port 8080 with `protocols h1 h2c`. It answers `/lgtm.v1.SessionService/SignOut` with status 401, `Content-Type: application/json` and the body `{"code":"unauthenticated","message":"Sign in first."}`. It answers every other path with status 200 and the body `{http.request.proto} {method} {path} hostport={http.request.hostport} xfp={header.X-Forwarded-Proto}`, and it logs each request.

| Test | What it asserts |
|---|---|
| `rules_serve_the_website` | `GET /` returns 200 with a body that contains `id="app"`. `GET /assets/a.js` returns 200 with a `Content-Type` that starts with `text/javascript`. `GET /no/such/file` returns 404. |
| `rules_pass_contract_paths_on` | `POST /lgtm.v1.SessionService/GetServer`, `POST /lgtm.link.v1.LinkService/Join` and `GET /frames/x` each return 200 with a body that starts with `HTTP/2.0`, followed by the method and the same path. `POST /lgtm.v2.X` returns 404 and does not reach the stub. |
| `rules_keep_status_and_body` | `POST /lgtm.v1.SessionService/SignOut` returns 401, `Content-Type: application/json`, and the stub's body byte for byte. |
| `rules_keep_host_and_set_forwarded_proto` | The stub sees `hostport=127.0.0.1:<published port>`. A request that sends `X-Forwarded-Proto: https` reaches the stub with `xfp=http`. |
| `rules_refuse_other_host_names` | With the header `Host: lgtm.test:8787`, `GET /` and `POST /lgtm.v1.SessionService/SignIn` each return 421, and the stub's log gains no line. With `Host: localhost:8787`, `GET /` returns 200. |
| `rules_answer_health_inside_only` | Inside the web container, `wget -qO- http://127.0.0.1:8081/healthz` exits 0. From the stub's container, a request to port 8081 of the web container fails to connect. |
| `rules_refuse_an_unknown_mode` | With `LGTM_WEB_TLS=-nothing`, Caddy exits with a status other than 0 within 5 seconds. |

- **Risky:** yes, because these rules decide which requests carry the password to the server. The mutation that must turn `rules_refuse_other_host_names` red: replace the content of `site.caddy` with one site `http://:8080 { import app }`. A second mutation: change `h2c://server:8080` to `http://server:8080`, which must turn `rules_pass_contract_paths_on` red, because the stub then reports `HTTP/1.1`.
- **Acceptance:** `bash deploy/test/web-rules.sh` prints seven `ok` lines and exits 0.
- **Public names:** the files `/etc/caddy/Caddyfile`, `/etc/caddy/site.caddy` and `/etc/caddy/site-https.caddy` inside the image; the variable `LGTM_WEB_TLS`, which is empty or `-https`; the container port 8080; the upstream `server:8080` over cleartext HTTP/2 (h2c); the health address `http://127.0.0.1:8081/healthz` inside the container; the status 421 for a host name that is not loopback; the static root `/srv/site`; the three prefixes `/lgtm.v1.`, `/lgtm.link.v1.` and `/frames/`.

**Work item 3. The website image**

- **Files:** creates `deploy/website.Dockerfile`, `deploy/website.Dockerfile.dockerignore` and `deploy/test/web-image.sh`.
- **Needs:** item 2. It can run at the same time as items 1 and 7.
- **Work area and complexity:** `infra`, medium. The build reads several folders of the workspace, and its allow-list must name each one.
- **Scope:** `stack/infra`.
- **Statements:** D1, C1, R9, and the design's section "The compose stack".
- **What it builds:** the stage `site` starts from `node:26.13.0-alpine@sha256:8996bb58…` and runs `npm install --global pnpm@12.16.1`, because Node 26 ships without Corepack. It copies the allowed context to `/repo`, runs `pnpm install --frozen-lockfile --filter @lgtm/frontend...` with a cache mount on the store of pnpm, and runs `VITE_LGTM_SERVER=/ pnpm --filter @lgtm/frontend build`. The stage `runtime` starts from `caddy:2.10.2-alpine@sha256:4c6e91c6…`. Its first step copies `/usr/bin/caddy` to a new file and moves it back with mode `0755`, which drops the file capability that the stock binary carries as effective, so the program starts with no capability and `no-new-privileges` (decisions row 26). It then copies `/repo/src/frontend/dist` to `/srv/site`, `deploy/Caddyfile` to `/etc/caddy/Caddyfile`, and `deploy/caddy/` to `/etc/caddy/`. It sets `USER 10003:10003`, `ENV XDG_DATA_HOME=/tmp XDG_CONFIG_HOME=/tmp`, and `CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]`.
- **The allow-list:** `deploy/website.Dockerfile.dockerignore` holds `*`, then `!package.json`, `!pnpm-lock.yaml`, `!pnpm-workspace.yaml`, `!tsconfig.base.json`, `!src/frontend/**`, `!src/contracts/**`, `!src/mock-server/**`, `!design/mock/**`, `!design/fonts/**`, `!third_party/inter/**`, `!third_party/jetbrains-mono/**`, `!third_party/codemirror/**`, `!deploy/Caddyfile`, `!deploy/caddy/**`, and then `**/node_modules`, `**/dist` and `**/.cert`. A probe build with this list passed on 2026-11-26 (scouting, "Website build").

Tests written first, in `deploy/test/web-image.sh`:

| Test | What it asserts |
|---|---|
| `web_image_bases_are_pinned` | Every `FROM` line of `deploy/website.Dockerfile` names an image followed by `@sha256:` and 64 lowercase hexadecimal characters, and the digests equal those of decisions row 8. |
| `web_image_context_is_allow_listed` | The ignore file's lines, with comments and blank lines removed, equal the list above exactly. |
| `web_image_leaves_out_state` | In a temporary copy of the tracked files, the test adds `deploy/.state/sentinel-<random>`, `deploy/.env` and `src/frontend/.cert/key.pem`, each with a random marker. In the stage `site`, `find / -xdev -name 'sentinel-*'` and `find / -xdev -path '*/.cert/*'` find nothing. `docker save` of the final image contains none of the markers. |
| `web_image_serves_the_built_site` | The image, run with no stub server, answers `GET /` with 200, `id="app"` and at least one `<script` element. Every `src` and `href` in that page that starts with `/` returns 200. |
| `web_image_fixes_the_server_address` | `docker history --no-trunc` of the stage `site` shows a build step that contains `VITE_LGTM_SERVER=/`. |
| `web_image_carries_the_rules` | `/etc/caddy/Caddyfile` and `/etc/caddy/site.caddy` in the image equal `deploy/Caddyfile` and `deploy/caddy/site.caddy` byte for byte. `Config.User` is `10003:10003`. |
| `web_image_runs_unprivileged` | `getcap /usr/bin/caddy` in the image prints nothing. The image, run as in `web_image_serves_the_built_site` with `--user 10003:10003`, `--read-only`, `--cap-drop ALL` and `--security-opt no-new-privileges:true`, answers `GET /` with 200. |
| `web_image_has_no_build_tools` | `docker run --rm --entrypoint sh lgtm-web:local -c 'command -v node || command -v pnpm || command -v npm'` exits with a status other than 0. |

- **Risky:** yes, for the same reason as item 1. The mutation that must turn `web_image_leaves_out_state` red: remove the line `*` from the ignore file. A second mutation: remove the copy step of the binary, which must turn `web_image_runs_unprivileged` red.
- **Acceptance:** `bash deploy/test/web-image.sh` prints eight `ok` lines and exits 0. A person then runs the image as in `web_image_serves_the_built_site`, opens the page in a browser, and sees the sign-in screen without a field for the server's address.
- **Public names:** the image tag `lgtm-web:local`, the stage names `site` and `runtime` (item 8a adds `tls-tools`), the user `10003:10003`, and the build value `VITE_LGTM_SERVER=/`.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A seal key, credential or the settings file reaches an image layer. | High | Allow-lists that start with `*`, and tests that plant random markers and search every stage and the saved image for them. |
| The stock Caddy binary carries its capability as effective, so it refuses to start without capabilities. | Medium | The runtime stage copies the binary once, which drops the flag, and `web_image_runs_unprivileged` checks it on every build. |
| The web server passes on a path that the server should not answer, or misses one it should. | Medium | One matcher of three prefixes, tested by path, and decisions row 16 records the list. |
| The website build needs a folder that the allow-list leaves out. | Low | The build fails at once and names the file, and the list is changed in item 3. |

**Verify:** `bash deploy/test/server-image.sh`, `bash deploy/test/web-rules.sh` and `bash deploy/test/web-image.sh` on Linux with Docker. A person opens the website image once in a browser. No other device is needed.

**Rollout detail:** step 1 builds item 2. Step 2 builds items 1 and 3. Each runs `docker build -f deploy/<name>.Dockerfile -t <tag> .` from the repository root.

**WS-2: One command starts everything, and the owner signs in**

The compose file of sprint 1 holds `postgres`, `openbao` and the one-shot setup jobs.
This workstream adds `server` and `web`, two networks, and the settings file, and it extends `deploy/up.sh`, which sprint 1 writes.
The settings reach both Compose and the launcher through the same lookup: a value in the shell wins, and `deploy/.env` supplies the rest.
A probe on 2026-11-26 showed that Compose reads `deploy/.env` by itself for `docker compose -f deploy/compose.yaml`, and that the check script's override with `!reset null` removes the password from the server (scouting, "Compose settings").

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2a | The resolved compose file publishes only the web port, on `127.0.0.1` by default, keeps the server off any network with a route out, mounts credentials read-only, and names the server's settings exactly. | `bash deploy/test/stack-web.sh` prints `ok` for its eleven tests. |
| 2b | The launcher refuses missing or unsafe settings before it starts anything, starts no server before the vault is ready, starts all four services in order, and changes nothing on a second run. | `bash deploy/test/stack-up.sh` prints `ok` for its nine tests. |
| 2c | On a clean machine, one command starts the stack, the server refuses to start without a password, the owner signs in, the session survives a restart, a changed password ends it, and the vault opens again after a restart of everything. | `deploy/test/checks.sh --only C1,C2,C3,C4,C13` prints five `ok` lines and exits 0. |
| 2d | The owner signs in through the browser. | A person opens `http://127.0.0.1:8787/`, sees the sign-in screen, types the password, and sees the Home screen with `127.0.0.1:8787` at the foot of the sidebar. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 4 | Add `server`, `web`, the networks and the settings file. | `deploy/compose.yaml`, `services.postgres` and the networks `data` and `vault`, which sprint 1 creates (`.plans/plans/go-s1-stack-vault/plan.md`, work items 2 and 3, and its "Contract for sprint 2"). Target `deploy/example.env`. `deploy/stack-prepare.sh`, which sprint 1 creates, gains the command `web-dirs`. The names the check script reads: `deploy/test/checks.sh`, `compose_file`, `check_c2` and `check_c4`. |
| 5 | Extend the launcher. | `deploy/up.sh`, its phases `database`, `seal`, `vault` and `ready` (sprint 1, work item 6). The check script runs it in `deploy/test/checks.sh`, `check_c1` and `up_file`. |
| 6 | Write the operator guide and run the sign-in checks. | `deploy/README.md`, which sprint 1 creates. `deploy/test/checks.sh`, functions `check_c1` to `check_c4` and `check_c13`. `deploy/test/README.md`, table "Names and observations the checks expect". |

**Design artifact:** none. No screen changes.

**Work item 4. The compose services, the networks and the settings file**

- **Files:** changes `deploy/compose.yaml` and `deploy/stack-prepare.sh`. Creates `deploy/example.env` and `deploy/test/stack-web.sh`.
- **Needs:** items 1 and 3, and the merged sprint 1 plan `go-s1-stack-vault`. It can run at the same time as item 7.
- **Work area and complexity:** `infra`, medium. It adds two services to a file that follows a set pattern, and it wires credentials that sprint 1 already makes.
- **Scope:** `stack/infra`.
- **Statements:** D4, D10, C1, C2, C4, C13, R9, and the design's section "The compose stack" (loopback by default, the port as a variable with the default 8787, the server reachable only inside the stack).
- **The service `server`:** `build` with `context: ..`, `dockerfile: deploy/server.Dockerfile` and `target: runtime`; `image: lgtm-server:local`; `user: "10001:10001"`; `read_only: true`; `tmpfs: [/tmp]`; `cap_drop: [ALL]`; `security_opt: [no-new-privileges:true]`; `restart: unless-stopped`; `networks: [app, data, vault]`; `depends_on` on `postgres` and `openbao`, each with `condition: service_healthy`. Its health check is `["CMD", "/usr/local/bin/lgtm-server", "health"]`, with a 5-second interval, a 3-second timeout, 12 retries and a 10-second start period. It publishes no port.
- **The settings of `server`:** `environment` is a mapping, so that the check script's `!reset null` can remove one key. The keys and values are those of decisions row 25: `LGTM_WEB_PASSWORD: ${LGTM_WEB_PASSWORD:-}`, `LGTM_LISTEN_ADDR: ":8080"`, `LGTM_PUBLIC_URL: ${LGTM_PUBLIC_URL:-}`, `LGTM_POSTGRES_URL: postgres://lgtm_app@postgres:5432/lgtm_app?sslmode=disable`, `LGTM_POSTGRES_PASSWORD_FILE: /run/lgtm/postgres.password`, `LGTM_OPENBAO_ADDR: http://openbao:8200`, `LGTM_OPENBAO_CREDENTIAL_FILE: /run/lgtm/openbao.json`, and `LGTM_TLS_CERT_FILE: ${LGTM_PUBLIC_URL:+/run/lgtm-tls/tls.crt}`.
- **The mounts of `server`:** each is a bind in long syntax with `read_only: true` and `bind: { create_host_path: false }`. The sprint 1 file `server/openbao.json` of the state folder goes to `/run/lgtm/openbao.json`. The sprint 1 file `postgres-app.password` goes to `/run/lgtm/postgres.password`. The folder `web-tls/public` goes to `/run/lgtm-tls`. The server mounts nothing else from the state folder.
- **The service `web`:** `build` with `context: ..`, `dockerfile: deploy/website.Dockerfile` and `target: runtime`; `image: lgtm-web:local`; `user: "10003:10003"`; `read_only: true`; `tmpfs: [/tmp]`; `cap_drop: [ALL]`; `security_opt: [no-new-privileges:true]`; `restart: unless-stopped`; `networks: [edge, app]`; `depends_on: { server: { condition: service_started } }`. Its port is `"${LGTM_WEB_BIND:-127.0.0.1}:${LGTM_WEB_PORT:-8787}:8080"`. Its `environment` is `LGTM_WEB_TLS: ${LGTM_PUBLIC_URL:+-https}`. It mounts `web-tls/public` at `/run/lgtm-tls` and `web-tls/private` at `/run/lgtm-tls-private`, both read-only, and nothing else. Its health check is `["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8081/healthz"]`, with the same timings as the server's.
- **The networks:** `edge` is an ordinary bridge network. `app` has `internal: true`. The sprint 1 networks `data` and `vault` stay as they are.
- **`stack-prepare web-dirs`:** the privileged setup helper of sprint 1 gains this command. It creates `web-tls/public` with owner 0, group 0 and mode `0755`, and `web-tls/private` with owner and group 10003 and mode `0700`, under the state folder. It leaves existing folders and their contents unchanged. The launcher runs it in both modes, so the mounts above always find their folders.
- **`deploy/example.env`:** one line for each setting, each after one comment sentence: `LGTM_WEB_PASSWORD=` (empty), `LGTM_WEB_PORT=8787`, `LGTM_WEB_BIND=127.0.0.1` and `LGTM_PUBLIC_URL=` (empty). Git already ignores `deploy/.env` through the root `.gitignore`, and it tracks `deploy/example.env`.

Tests written first, in `deploy/test/stack-web.sh`.
Each test reads `docker compose -f deploy/compose.yaml --env-file <fixture> config --format json` with a fixture settings file and a fixture state folder, unless it says otherwise.

| Test | What it asserts |
|---|---|
| `web_publishes_on_loopback_by_default` | With an empty settings file, `services.web.ports` is one entry with `host_ip` `127.0.0.1`, `published` `8787`, `target` 8080 and `protocol` `tcp`. With `LGTM_WEB_PORT=18787`, `published` is `18787`. No other service has `ports`. |
| `web_bind_follows_its_setting` | With `LGTM_WEB_BIND=0.0.0.0` and an `https://` public address, `host_ip` is `0.0.0.0`. The launcher's guard is tested in item 5. |
| `server_has_no_route_out` | `services.server.networks` has exactly the keys `app`, `data` and `vault`, and each of the three networks has `internal: true`. `services.web.networks` has exactly `edge` and `app`, and `edge` is not internal. A running fixture of `server`, started from the pinned distroless image with the same networks, cannot open a connection to an address outside the stack. |
| `server_settings_are_named` | With `LGTM_WEB_PASSWORD=fixture-<random>` and no public address, `services.server.environment` equals exactly the eight keys and values above, and `LGTM_TLS_CERT_FILE` is empty. With `LGTM_PUBLIC_URL=https://lgtm.test`, `LGTM_TLS_CERT_FILE` is `/run/lgtm-tls/tls.crt` and `services.web.environment.LGTM_WEB_TLS` is `-https`. Without it, `LGTM_WEB_TLS` is empty. |
| `password_follows_the_checks` | Applying the override of `check_c2` in `deploy/test/checks.sh` removes `LGTM_WEB_PASSWORD` from `services.server.environment` and keeps the other keys. A value of `LGTM_WEB_PASSWORD` in the shell wins over the settings file, as `check_c4` needs. |
| `services_are_unprivileged` | For `server`, `user` is `10001:10001`; for `web`, it is `10003:10003`. Both have `read_only: true`, `cap_drop` equal to `[ALL]`, `security_opt` containing `no-new-privileges:true`, `tmpfs` containing `/tmp`, and `restart` equal to `unless-stopped`. Neither has `cap_add`, `privileged` or `pid`. |
| `server_mounts_are_read_only` | The server's binds are exactly the three above, each with `read_only: true` and `create_host_path: false`. No bind of `server` names `web-tls/private`, `admin`, `bao` or the settings file. The web service's binds are exactly its two folders. |
| `server_waits_for_database_and_vault` | `services.server.depends_on` has `postgres` and `openbao`, each with `condition: service_healthy`. `services.web.depends_on.server.condition` is `service_started`. |
| `images_are_built_from_the_pinned_files` | `services.server.build` names context `..`, the Dockerfile `deploy/server.Dockerfile` and the target `runtime`, and its `image` is `lgtm-server:local`. `services.web.build` names `deploy/website.Dockerfile` and `runtime`, and its `image` is `lgtm-web:local`. Every other service's `image` contains `@sha256:`. The two health checks equal the commands above. |
| `prepare_makes_the_web_folders` | In a fixture state folder, `stack-prepare web-dirs` makes `web-tls/public` with mode `0755` and `web-tls/private` with owner 10003 and mode `0700`, read through the sprint 1 privileged fixture helper. A file planted in `web-tls/private` survives a second run unchanged. |
| `example_settings_list_every_setting` | `deploy/example.env` has the four lines above, with `LGTM_WEB_PASSWORD` empty, and every setting line follows a comment line. |

- **Risky:** yes, because it publishes the port and mounts the credentials. The mutation that must turn `web_publishes_on_loopback_by_default` red: change the default bind to `0.0.0.0`. A second mutation: add `edge` to the server's networks, which must turn `server_has_no_route_out` red. A third mutation: mount `web-tls/private` into `server`, which must turn `server_mounts_are_read_only` red.
- **Acceptance:** `bash deploy/test/stack-web.sh` prints eleven `ok` lines and exits 0.
- **Public names:** the services `server` and `web`; the networks `edge` and `app`; the settings `LGTM_WEB_PASSWORD`, `LGTM_WEB_PORT`, `LGTM_WEB_BIND` and `LGTM_PUBLIC_URL`; the server settings of decisions row 25; the mounts `/run/lgtm/openbao.json`, `/run/lgtm/postgres.password`, `/run/lgtm-tls/` and `/run/lgtm-tls-private/`; the state folders `web-tls/public` and `web-tls/private`; the helper command `stack-prepare web-dirs`; the file `deploy/example.env`.

**Work item 5. The launcher starts everything**

- **Files:** changes `deploy/up.sh`. Creates `deploy/test/stack-up.sh`.
- **Needs:** item 4, and the option `--until vault` of the sprint 1 launcher (request V1 in the result file). If sprint 1 does not add the option, this item adds it and changes the calls of `deploy/up.sh` in the sprint 1 suites `deploy/test/stack-vault.sh` and `deploy/test/vault-admin.sh` to `deploy/up.sh --until vault`. It can run at the same time as item 7.
- **Work area and complexity:** `infra`, medium. It adds three phases to a launcher that already has a fixed shape, and its tests reuse the sprint 1 fixtures.
- **Scope:** `stack/infra`.
- **Statements:** C1, C13, R9, and the design's section "The compose stack" (one command starts the stack; loopback by default; a public address must be `https`).
- **What it adds before any phase:** the launcher reads the resolved settings with `docker compose -f deploy/compose.yaml config --environment`, so that it and Compose never disagree. It never prints the password. It then checks, in this order, and stops at the first failure with exit status 2 and the given line:
  - `LGTM_WEB_PASSWORD` is not empty, or it prints `LGTM_WEB_PASSWORD is not set; put it in deploy/.env (see deploy/example.env) or in the environment`.
  - `LGTM_WEB_PORT` is a number from 1 to 65535, or it prints `LGTM_WEB_PORT must be a number from 1 to 65535`.
  - `LGTM_PUBLIC_URL` is empty, or it starts with `https://` and holds a host and at most a port, with no path, query, user or password; otherwise it prints `LGTM_PUBLIC_URL must start with https:// and name only a host and a port`.
  - `LGTM_WEB_BIND` is an address of `127.0.0.0/8`, or `LGTM_PUBLIC_URL` is set; otherwise it prints `LGTM_WEB_BIND <address> is not a loopback address; set LGTM_PUBLIC_URL to an https address first`.
  - Nothing listens on the bind address and port, unless the running `web` of this Compose project already publishes it; otherwise it prints `port <port> on <address> is in use; set LGTM_WEB_PORT in deploy/.env`.
- **The phases:** every phase prints `phase <name> start` and `phase <name> done`, as in sprint 1. The order is `images`, then sprint 1's `database`, `seal` and `vault`, then `server`, `web` and `ready`. Item 8a adds `tls` after `seal`. The phase `images` runs `docker compose -f deploy/compose.yaml build server web`. The phase `database` gains one step at its start: `stack-prepare web-dirs`. The phase `server` runs `docker compose -f deploy/compose.yaml up -d --wait --wait-timeout 120 --no-deps server`. The phase `web` does the same for `web`. The phase `ready` prints sprint 1's line `postgres and openbao are healthy`, then `the website is at <address>`. The address is `LGTM_PUBLIC_URL` with a final `/` when that is set, and otherwise `http://<bind>:<port>/`.
- **`--until <phase>`:** the launcher stops after the named phase and runs `ready` with only sprint 1's line. Sprint 1 is asked to add it, as decisions row 18 records.
- **`--reset --yes`:** sprint 1's reset first stops and removes `web` and `server`, then goes on as sprint 1 wrote it.

Tests written first, in `deploy/test/stack-up.sh`.
Each runs in a disposable fixture, with its own Compose project, state folder, settings file and free loopback port.

| Test | What it asserts |
|---|---|
| `up_refuses_without_password` | With no `LGTM_WEB_PASSWORD` in the shell or the settings file, the launcher exits with status 2, prints the password line above, prints no `phase` line, and the project has no container. |
| `up_refuses_a_bad_port` | For each of `0`, `65536` and `web`, the launcher exits with status 2, prints the port line, and the project has no container. |
| `up_refuses_an_unsafe_public_setting` | `LGTM_WEB_BIND=0.0.0.0` with no public address prints the bind line. `LGTM_PUBLIC_URL=http://lgtm.test` and `LGTM_PUBLIC_URL=https://lgtm.test/path` each print the public address line. Each exits with status 2 and leaves no container. |
| `up_names_a_busy_port` | While a test container publishes the fixture's port on `127.0.0.1`, the launcher exits with status 2, prints `port <port> on 127.0.0.1 is in use; set LGTM_WEB_PORT in deploy/.env`, and leaves no container. |
| `up_starts_everything_in_order` | From an empty fixture, the launcher exits 0. Its `phase` lines come in the order `images`, `database`, `seal`, `vault`, `server`, `web`, `ready`, each `start` before its `done`. Its last line is `the website is at http://127.0.0.1:<port>/`. `docker compose ps` shows `postgres`, `openbao`, `server` and `web` running and healthy. `GET /` on the port contains `id="app"`. `POST /lgtm.v1.SessionService/GetServer` returns 200. |
| `up_starts_no_server_before_the_vault` | In a fixture whose published vault credential is malformed, the launcher exits with a status other than 0 during the phase `vault`, prints sprint 1's line `server vault credential needs repair`, and `docker compose ps -a` lists no `server` and no `web` container. This is check PB5 of the sprint 1 plan. |
| `up_is_idempotent` | A second run exits 0. The container identifiers of `server` and `web`, and the identifiers of their images, are the same as after the first run. |
| `up_until_vault_stops_early` | `deploy/up.sh --until vault` exits 0, its last line is `postgres and openbao are healthy`, and the project has no `server` and no `web` container. |
| `up_reset_removes_web_and_server` | After a full start, `deploy/up.sh --reset --yes` exits 0 and leaves no container and no volume of the project. A following `deploy/up.sh` starts all four services again. |

- **Risky:** yes, because it guards the password and the order in which the server receives its credential. The mutation that must turn `up_refuses_without_password` red: delete the password check. A second mutation: run the phase `server` before the phase `vault`, which must turn `up_starts_no_server_before_the_vault` red.
- **Acceptance:** `bash deploy/test/stack-up.sh` prints nine `ok` lines and exits 0.
- **Public names:** the phase names `images`, `server`, `web` and (item 8a) `tls`; the option `--until <phase>`; the five diagnostic lines above; the final line `the website is at <address>`.

**Work item 6. The operator guide and the sign-in checks**

- **Files:** changes `deploy/README.md`. It changes no test.
- **Needs:** items 4 and 5, and the merged plan `go-s2-server-signin`.
- **Work area and complexity:** `infra`, low. It writes a guide, and it runs checks that already exist.
- **Scope:** `stack/infra`.
- **Statements:** C1, C2, C3, C4 and C13.
- **The guide:** `deploy/README.md` gains these sections, after sprint 1's. "Start everything" says to copy `deploy/example.env` to `deploy/.env`, set the password, run `deploy/up.sh`, and open the printed address. "Settings" is a table of the four settings, their defaults and their effect. "When port 8787 is taken" says that the old engine uses 8787 and that `LGTM_WEB_PORT` picks another. "Reach the stack from another computer" says that a port forward of the loopback port is the default way, and that item 8b's public mode is the other. It also says that the browser warns once about the stack's own certificate and that `web-tls/public/tls.key.sha256` holds its fingerprint.
- **Tests written first:** the user-flow script `deploy/test/checks.sh`, which the orchestrator's test writer wrote on 2026-11-25 before any plan. Its checks C1 to C4 and C13 are red today, because `deploy/up.sh` and the services do not exist. This item changes no line of it (decisions row 24).
- **Risky:** no. It adds no code.
- **Acceptance:** on a clean machine, which means a fresh clone at the merge commit, with no image tagged `lgtm-server:local` or `lgtm-web:local`, no Compose project `lgtm` and no state folder:
  1. `cp deploy/example.env deploy/.env`, and set `LGTM_WEB_PASSWORD` in it.
  2. `export LGTM_WEB_PASSWORD=<the same value>`, and run `deploy/test/checks.sh --only C1,C2,C3,C4,C13`. It prints `ok 1`, `ok 2`, `ok 3`, `ok 4` and `ok 13` and exits 0. This also covers check PB8 of the sprint 1 plan.
  3. A person performs criterion 2d.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The port is published on every interface while the stack speaks plain HTTP. | High | The default bind is `127.0.0.1`, the launcher refuses another bind without an `https://` address, and the loopback rules refuse other host names. |
| The server starts before the vault has published its credential, or with another installation's credential. | High | `depends_on` with `service_healthy`, the launcher's phase order, the sprint 1 binding checks, and `up_starts_no_server_before_the_vault`. |
| The Server team picks other names for its settings or its health command. | Medium | The names are requests in the result file. Items 4 and 5 are built only after that plan is reviewed, and `server_settings_are_named` pins whatever both plans agree. |
| A sprint 1 suite breaks once the launcher also starts the server. | Medium | The option `--until vault`, and the fallback in item 5. |
| Port 8787 is taken by the old engine on the development machine. | Low | The launcher names the busy port and the setting, and the checks honour `LGTM_WEB_PORT`. |

**Verify:** `bash deploy/test/stack-web.sh`, `bash deploy/test/stack-up.sh` and `deploy/test/checks.sh --only C1,C2,C3,C4,C13` on Linux with Docker, against real PostgreSQL and OpenBao. A person signs in once in a browser (criterion 2d).

**Rollout detail:** step 3 builds items 4 and 5 after `go-s1-stack-vault` is merged. Step 4 runs item 6 after `go-s2-server-signin` is merged.

**WS-3: The link's long stream passes through the web server**

Issue 1633 asks for proof that the web server passes the link's long two-way stream on without cutting it, before sprint 3 depends on it.
The link uses the procedure `/lgtm.link.v1.LinkService/Open`, a stream in both directions, which the contract plan names.
A machine reaches a loopback address over cleartext HTTP/2, so the web server must accept HTTP/2 without encryption from the machine and pass the stream on in the same way.
A stand-in program, built from the standard library of Go alone, plays the server and the machine, so the proof needs no part of sprint 3.

The probes of 2026-11-26 found a real limit, which is why this workstream exists (scouting, "Which limit cuts a silent stream").
Caddy 2.11.9 cut a stream whose upward direction, from the machine to the server, stayed silent for 60 seconds, even while the server kept sending down.
It logged `reading: i/o timeout`, and none of its three timeout settings changed the limit.
A silent downward direction was not cut, and with no proxy in between neither direction was cut.
Caddy 2.10.2 kept the same stream open, so the plan pins Caddy 2.10.2 (decisions row 26).
The link's client sends a heartbeat every 10 seconds, so the real link would stay inside the limit even on 2.11.9.
The tests still hold each direction silent for longer than a minute, so that a later image with the limit fails them before it ships.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 3a | A two-way stream through the real web server rules carries messages both ways, survives 35 seconds of silence in both directions, ends with its trailer, and uses HTTP/2 on both hops. Each direction alone stays silent for 80 seconds while the other keeps sending, and the stream stays open. A one-way stream to a browser arrives message by message for 80 seconds. | `bash deploy/test/link-stream.sh` prints `ok` for its six tests, with the soak test reported as skipped. |
| 3b | A stream stays open through 6 minutes of silence in both directions, and through 6 minutes and 30 seconds of upward silence, before sprint 3 starts. | `LGTM_STREAM_SOAK=1 bash deploy/test/link-stream.sh` prints `ok` for all six tests, and the sprint's report records the output. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 7 | Write the stand-in program and the stream test. | Target `deploy/test/streamprobe/main.go`, functions `serve`, `dial` and `shaped`; the nearest pattern is the probe in scouting, "Stream probe". Target `deploy/test/link-stream.sh`. The procedure name: `.plans/plans/go-s0-contract/plan.md`, item 6, `rpc lgtm.link.v1.LinkService.Open`. The rules under test: `deploy/Caddyfile`, snippet `(app)`. |

**Design artifact:** none.

**Work item 7. The stream test**

- **Files:** creates `deploy/test/streamprobe/main.go` and `deploy/test/link-stream.sh`. Its builder may change `deploy/Caddyfile`, `deploy/caddy/site.caddy` and the Caddy digest in `deploy/website.Dockerfile` when a test fails, once items 2 and 3 are merged.
- **Needs:** items 2, 3 and 4 for the test to pass. Its tests can be written at once. It can run at the same time as items 1 and 5.
- **Work area and complexity:** `infra`, medium. The program is small, but the test drives a real proxy through timing.
- **Scope:** `stack/infra`.
- **Statements:** the design's sections "One connection, dialled by the machine", "Knowing whether a machine is there" (a heartbeat every 10 seconds) and "Encryption on the link", C8 and C9, and issue 1633, point 4.
- **The stand-in program:** package `main`, standard library only, in the root module, so `go build ./...` and `go vet ./...` include it. The test builds it inside `golang:1.26.11-alpine@sha256:ecb67f98…` with `CGO_ENABLED=0`, so the host's Go version does not matter. Its flags:
  - `-mode serve -addr :8080` listens with HTTP/1.1 and cleartext HTTP/2 through `http.Server.Protocols`. It logs `upstream proto=<protocol> path=<path>` for each request. The header `Probe-Shape` selects what it does:
    - With no header, it enables full duplex, sends status 200 with the header `Trailer: Probe-End`, echoes each line of the request body as `echo <line>` with a flush, and sets the trailer `Probe-End` to the number of lines when the body ends.
    - With `quiet-down`, it reads every line and writes nothing until the body ends, then writes `count <n>`.
    - With `quiet-up`, it writes `tick <n>` every 2 seconds, with a flush, until the request body ends, then sets the trailer `Probe-End` to `up-closed`.
    - With `ticks`, it reads the whole body first, then writes `tick <n>` every 2 seconds, as many times as the header `Probe-Ticks` says, with 5 as the default.
  - `-mode dial -url <address>` sends `m0` and reads its echo, then sends `m<n>` every `-every` until `-total` has passed, reading each echo before the next send. It then stays silent for `-gap`, sends one more line, reads its echo, closes its side, and checks that nothing more arrives and that the trailer equals the number of lines. It prints `proto <protocol>` and, at the end, `ok stream lasted <duration> with <n> messages each way`. Any missing or wrong echo prints `not ok` with the message number and exits 1.
  - `-mode shaped -shape quiet-down -total <d> -url <address>` sends a line every 2 seconds for the duration, closes its side, and expects `count <n>` with the right number. `-mode shaped -shape quiet-up -total <d> -url <address>` sends one line, stays silent, and reads ticks for the duration, then closes its side. Each prints one `ok` or `not ok` line and exits 0 or 1.
  - For item 8b, `-pin-sha256 <file>` and `-server-name <host>` make `dial` and `shaped` use Transport Layer Security (TLS), accept only a certificate whose key hashes to the value in the file, and exit 3 with `pin mismatch` otherwise, before sending a message.
- **The fixture:** a Compose project from `deploy/compose.yaml` plus an override file of the test. The override replaces `server` with the pinned distroless image running the stand-in program at `/probe -mode serve -addr :8080`. It resets the server's `depends_on`, `volumes`, `environment` and health check with `!reset`, and it mounts the built program read-only. The test starts only `server` and `web`, so no database and no vault are needed, and `web` is the real image of item 3. Every client run uses the host's network and the published loopback port, so the path through Docker's published port is part of the test.

Tests written first, in `deploy/test/link-stream.sh`:

| Test | What it asserts |
|---|---|
| `stream_passes_both_ways_loopback` | `dial` against `http://127.0.0.1:<port>/lgtm.link.v1.LinkService/Open` with `-every 2s -total 20s -gap 35s` exits 0 and prints `ok`, with at least 12 messages each way. |
| `stream_hops_speak_http2` | `dial` prints `proto HTTP/2.0`, and the server's log holds `upstream proto=HTTP/2.0 path=/lgtm.link.v1.LinkService/Open`. |
| `stream_survives_upward_silence` | `shaped -shape quiet-up -total 80s` exits 0 and prints `ok` with at least 38 ticks. This is the test that failed on Caddy 2.11.9 at 60 seconds. |
| `stream_survives_downward_silence` | `shaped -shape quiet-down -total 80s` exits 0 and prints `ok`. |
| `server_stream_arrives_message_by_message` | `curl --http1.1 --no-buffer` posts an empty body with `Probe-Shape: ticks` and `Probe-Ticks: 40` to `/lgtm.v1.ClientService/WatchClients`. All 40 `tick` lines arrive, with at least 1.5 seconds between each pair, measured on arrival. This is how the website receives `WatchClients`. |
| `stream_survives_soak` | Only when `LGTM_STREAM_SOAK=1`: `dial` with `-every 2s -total 4s -gap 6m` and, at the same time, `shaped -shape quiet-up -total 6m30s` each exit 0 and print `ok`. Otherwise the line is `ok 6 - stream_survives_soak # SKIP soak not requested`. |

- **Risky:** no. It touches no key, token or order of records. Its strength is still checked once by the build lead: building the website image on `caddy:2.11.9-alpine@sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0` must turn `stream_survives_upward_silence` red.
- **Acceptance:** `bash deploy/test/link-stream.sh` prints six `ok` lines, one of them skipped, and exits 0. Before sprint 3 starts, `LGTM_STREAM_SOAK=1 bash deploy/test/link-stream.sh` prints six `ok` lines with none skipped.
- **Public names:** the program path `deploy/test/streamprobe/`, its modes and flags above, the trailer `Probe-End`, the headers `Probe-Shape` and `Probe-Ticks`, and the setting `LGTM_STREAM_SOAK`. Sprint 3 may reuse the program to test the real link through the web server.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A limit of the web server cuts a stream whose upward direction is silent. Caddy 2.11.9 does so at 60 seconds. | High | Pin Caddy 2.10.2, which passed. `stream_survives_upward_silence` fails on any image with the limit, and the link's 10-second heartbeat keeps the real link inside it. Request L1 asks the link plans to keep that heartbeat. |
| Caddy 2.10.2 is older and may lack later security fixes. | Medium | It listens only on loopback by default, and it is reached from outside only in public mode. A newer Caddy replaces it when `deploy/test/link-stream.sh` passes on it, and the result file asks for an issue to track that. |
| The web server buffers a one-way stream, and the website sees changes late. | Medium | `flush_interval -1` and `server_stream_arrives_message_by_message`. |
| The stand-in behaves unlike the real link. | Medium | It uses the same transport, path, protocol and full duplex. Sprint 3's link tests run the real client through the same rules. |

**Verify:** `bash deploy/test/link-stream.sh` on Linux with Docker, and once with `LGTM_STREAM_SOAK=1` before sprint 3. No device is needed.

**Rollout detail:** step 1 writes the tests and runs them against item 2 with a fixture image. Step 6 runs the soak on the merged sprint.

**WS-4: A public HTTPS address**

The owner turns on a public address by setting `LGTM_PUBLIC_URL` to an `https://` address and, to reach it from another computer, `LGTM_WEB_BIND`.
The stack then needs a certificate, and decisions row 10 records why it makes one itself.
The key is made once, because a machine keeps the hash of the certificate's key from the moment it joins (issue 1639).
The certificate can be signed again with the same key, for example when the host name changes, and joined machines keep working.
The web server holds the key, ends the encryption, and passes calls on exactly as in loopback mode.
The server reads only the public certificate, to put the hash of its key into each join token in sprint 3.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 4a | The certificate job makes one key, keeps it on every run, signs a certificate for the public host, writes the key's hash, and refuses to make a new key when a certificate already exists. | `bash deploy/test/web-tls.sh` prints `ok` for its eight tests. |
| 4b | In public mode, the port speaks only TLS with HTTP/2. The website's page and `GetServer` return 200 through it. `SignIn` sets a cookie marked `Secure`. The stand-in stream passes with the key pinned and is refused with another pin. The server can read the certificate and cannot read the key. | `bash deploy/test/stack-https.sh` prints `ok` for its seven tests. |
| 4c | The owner opens the public address in a browser. | A person sets `LGTM_PUBLIC_URL=https://lgtm.test:8787` and maps `lgtm.test` to `127.0.0.1` in the host's name file, runs `deploy/up.sh`, opens the address, accepts the browser's warning once, and signs in. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 8a | Make and keep the certificate. | Target `deploy/web-tls.sh`, command `make`. `deploy/website.Dockerfile`, new stage `tls-tools`. `deploy/compose.yaml`, new job `web-tls` under profile `setup`, as sprint 1's `openbao-init`. `deploy/up.sh`, new phase `tls`. The nearest pattern is the certificate step in `src/frontend/vite.config.ts`, function `ownCertificate`. |
| 8b | Serve HTTPS with that certificate. | Target `deploy/caddy/site-https.caddy`. `deploy/Caddyfile`, the final `import`. The pin the client needs: `.plans/plans/go-s0-contract/plan.md`, item 5, field `certificate_key_sha256`. |

**Design artifact:** none.

**Work item 8a. The certificate of the public address**

- **Files:** creates `deploy/web-tls.sh` and `deploy/test/web-tls.sh`. Changes `deploy/website.Dockerfile`, `deploy/compose.yaml` and `deploy/up.sh`.
- **Needs:** items 3 and 5.
- **Work area and complexity:** `infra`, medium. It is one script of a known shape, but it makes a key that every machine will pin.
- **Scope:** `stack/infra`.
- **Statements:** D17, and the design's sections "Encryption on the link" and "The compose stack" (a public address must be `https`).
- **The stage `tls-tools`:** it starts from the same pinned Caddy image and adds `openssl` with `apk add --no-cache openssl=<version>`, where the version is the one the first build records (decisions row 8). Compose builds it as `lgtm-web-tls:local`.
- **The job `web-tls`:** a one-shot service under profile `setup`, from `lgtm-web-tls:local`, with no network (`network_mode: none`), `cap_drop: [ALL]`, `cap_add: [CHOWN, FOWNER, DAC_OVERRIDE]`, and `no-new-privileges:true`. It mounts the state folder's `web-tls/` at `/state/web-tls` and `deploy/web-tls.sh` read-only, and it receives `LGTM_PUBLIC_URL`. The launcher's phase `tls` runs `docker compose -f deploy/compose.yaml run --rm --no-deps web-tls` after `seal`, only when `LGTM_PUBLIC_URL` is set, and `images` builds the stage only then.
- **What `deploy/web-tls.sh make` does,** in order:
  1. It takes the host from `LGTM_PUBLIC_URL`. A name becomes the subject alternative name `DNS:<host>`, and an address becomes `IP:<address>`.
  2. When `public/tls.crt` exists and `private/tls.key` does not, it stops with status 1 and `certificate key missing; restore web-tls/private/tls.key, or remove web-tls/public/tls.crt to make a new key that every machine must join again`.
  3. When `private/tls.key` does not exist, it makes an Elliptic Curve Digital Signature Algorithm (ECDSA) P-256 key in the format PKCS #8 (Public-Key Cryptography Standards number 8), writes it to a temporary file in `private/`, sets owner and group 10003 and mode `0400`, flushes it, and renames it to `private/tls.key`.
  4. When `public/tls.crt` does not exist, or its subject alternative name differs from the host, it signs a new certificate with the existing key, with subject `CN=<host>`, that one name, and a validity of 3,650 days. It writes the certificate to a temporary file, sets mode `0444`, and renames it to `public/tls.crt`.
  5. It writes `public/tls.key.sha256`, the hash SHA-256 (Secure Hash Algorithm, 256 bits) of the certificate's SubjectPublicKeyInfo in the Distinguished Encoding Rules (DER) form, as 64 lowercase hexadecimal characters and one newline, with mode `0444`. This is the value of `certificate_key_sha256` in the contract's join token.
  6. It never prints the key.

Tests written first, in `deploy/test/web-tls.sh`.
Each runs the job in a fixture state folder and reads private files through the sprint 1 privileged fixture helper.

| Test | What it asserts |
|---|---|
| `tls_key_is_made_once` | Two runs of the job leave `private/tls.key` with the same SHA-256 of its bytes and the same modification time, and `public/tls.crt` unchanged. |
| `tls_key_stays_private` | `private/tls.key` has owner and group 10003 and mode `0400`. `public/` holds exactly `tls.crt` and `tls.key.sha256`, each with mode `0444`. Neither public file contains the text `PRIVATE KEY`. The job's output contains no line of the key. |
| `tls_certificate_names_the_host` | For `https://lgtm.test:8443`, the certificate's only subject alternative name is `DNS:lgtm.test`. For `https://127.0.0.2`, it is `IP Address:127.0.0.2`. The key is an elliptic-curve key on the curve P-256, and the certificate is valid for at least 3,649 more days. |
| `tls_hash_matches_the_key` | `public/tls.key.sha256` equals the SHA-256 of the certificate's SubjectPublicKeyInfo in DER form, computed by the test with `openssl` in the stage `tls-tools`, and ends with one newline. |
| `tls_new_host_keeps_the_key` | After a run for `lgtm.test`, a run for `other.test` changes the certificate's name to `DNS:other.test` and leaves `private/tls.key` and `public/tls.key.sha256` unchanged. |
| `tls_refuses_a_certificate_without_its_key` | With `public/tls.crt` present and `private/tls.key` removed, the job exits 1 with the line of step 2, and `private/` stays empty. |
| `tls_is_skipped_in_loopback_mode` | `deploy/up.sh` without `LGTM_PUBLIC_URL` prints no `phase tls` line, and `web-tls/public` and `web-tls/private` stay empty. |
| `tls_tools_are_pinned` | The stage `tls-tools` starts from the Caddy digest of decisions row 8, and its `apk add` line names `openssl=` followed by a version. |

- **Risky:** yes, because it makes and keeps a key. The mutation that must turn `tls_key_is_made_once` red: make a new key on every run. A second mutation: write the key with mode `0444`, which must turn `tls_key_stays_private` red.
- **Acceptance:** `bash deploy/test/web-tls.sh` prints eight `ok` lines and exits 0.
- **Public names:** the files `web-tls/private/tls.key`, `web-tls/public/tls.crt` and `web-tls/public/tls.key.sha256`; their owners and modes; the job `web-tls`; the image `lgtm-web-tls:local`; the phase `tls`; the line of step 2.

**Work item 8b. HTTPS on the public address**

- **Files:** creates `deploy/caddy/site-https.caddy` and `deploy/test/stack-https.sh`.
- **Needs:** items 7 and 8a, and, for its last two tests, the merged plan `go-s2-server-signin`.
- **Work area and complexity:** `infra`, medium. It is one short file of rules, and its tests reuse the stand-in program and the fixtures.
- **Scope:** `stack/infra`.
- **Statements:** D17, C3, the design's section "Sign-in and web sessions" (the cookie is `Secure` when the public address is `https`), and the design's section "Encryption on the link".
- **The rules:** `deploy/caddy/site-https.caddy` defines one site, `https://:8080`, with `tls /run/lgtm-tls/tls.crt /run/lgtm-tls-private/tls.key { protocols tls1.2 tls1.3 }` and `import app`. The global `protocols h1 h2 h2c` leaves out HTTP/3, so the web server advertises no other port.

Tests written first, in `deploy/test/stack-https.sh`.
Each runs a full fixture stack with `LGTM_PUBLIC_URL=https://lgtm.test:<port>` and the bind `127.0.0.1`, and reaches it with `curl --resolve lgtm.test:<port>:127.0.0.1 --cacert <fixture>/web-tls/public/tls.crt`.

| Test | What it asserts |
|---|---|
| `https_speaks_tls_only` | `GET https://lgtm.test:<port>/` returns 200 with `id="app"` over HTTP/2, as `curl -w '%{http_version}'` reports `2`. `GET http://127.0.0.1:<port>/` returns 400, and its body has no `id="app"`. |
| `https_advertises_no_other_protocol` | The response to `GET /` has no `Alt-Svc` header. |
| `https_passes_calls_on` | `POST /lgtm.v1.SessionService/GetServer` over HTTPS returns 200. |
| `https_stream_pins_the_key` | The stand-in program, run as the server as in item 7, passes `dial -pin-sha256 web-tls/public/tls.key.sha256 -server-name lgtm.test` with `-every 2s -total 20s -gap 35s`, and `shaped -shape quiet-up -total 80s` with the same pin. With a file that holds another hash, `dial` exits 3 and prints `pin mismatch`, and the stand-in's log shows no request. |
| `server_sees_only_the_certificate` | Inside `server`, `LGTM_TLS_CERT_FILE` is `/run/lgtm-tls/tls.crt`, and `/run/lgtm-tls-private` does not exist. Inside `web`, both paths exist. |
| `https_cookie_is_secure` | `SignIn` with the fixture password over HTTPS returns a `Set-Cookie` header with `Secure`, `HttpOnly` and `SameSite=Strict`. |
| `https_loopback_mode_is_unchanged` | The same fixture restarted without `LGTM_PUBLIC_URL` serves `GET http://127.0.0.1:<port>/` with 200 and refuses `Host: lgtm.test` with 421. |

- **Risky:** yes, because it ends the encryption that carries the password and the link. The mutation that must turn `https_speaks_tls_only` red: replace the site address with `http://:8080` and remove `tls`. A second mutation: mount `web-tls/private` into `server`, which must turn `server_sees_only_the_certificate` red.
- **Acceptance:** `bash deploy/test/stack-https.sh` prints seven `ok` lines and exits 0, and a person performs criterion 4c.
- **Public names:** the file `/etc/caddy/site-https.caddy`, the TLS versions 1.2 and 1.3, and HTTP/1.1 and HTTP/2 only on the public port.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The key is lost or replaced, and every joined machine is cut off. | High | The job never replaces a key, it refuses when the key is missing but a certificate exists, and the guide names the file to back up. |
| The key reaches the server or a log. | High | Separate private and public folders, the server mounts only the public one, and the tests check the mounts and the job's output. |
| A browser shows a warning that the owner does not expect. | Low | The guide says so, and decisions row 10 records it as accepted for wave 1. |
| The owner later wants a certificate of their own, or one from a public authority. | Low | The result file proposes it as a feature for a later wave. The pin of the key in the join token is the constraint any such feature must keep. |

**Verify:** `bash deploy/test/web-tls.sh` and `bash deploy/test/stack-https.sh` on Linux with Docker. A person opens the public address once in a browser (criterion 4c).

**Rollout detail:** step 5 builds items 8a and 8b after items 5 and 7 are merged. Loopback mode is the default, so nothing changes for a stack that does not set `LGTM_PUBLIC_URL`.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe: the brief's four work items, its three observations and its two points to settle (`.plans/plans/go-s2-stack-web/plan.md` as written on 2026-11-25, preserved in `scouting.md`); the task file's three specific demands (`.foundry/go-rewrite/tasks/plan-go-s2-stack-web.md`); the design's section "The compose stack"; issue 1633, points 1 to 4; the names in `deploy/test/README.md` that touch this plan; the sprint 1 plan's "Contract for sprint 2" and its checks PB5 and PB8; and the statements C1, C2, C3, C4, C13, D1, D17 and R9 of the design.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the server image | WS-1 | Item 1. |
| Brief item 2, the website image and the web server rules | WS-1, WS-3 | Items 2, 3 and 7. |
| Brief item 3, the compose services, the settings file and the port variable | WS-2 | Item 4. |
| Brief item 4, the one command and the sign-in part of the user-flow script | WS-2 | Items 5 and 6. |
| Observation: one command starts the stack, and the website opens on port 8787 | WS-2 | Items 5 and 6, check C1. |
| Observation: the owner signs in | WS-2 | Item 6, checks C3 and criterion 2d. |
| Observation: loopback only unless a public address is set, which must be HTTPS | WS-2, WS-4 | Items 4, 5, 8a and 8b. |
| Point 1: how a public HTTPS address is turned on, and where its certificate comes from | WS-4 | Items 5, 8a and 8b; decisions rows 10 and 11. |
| Point 2: where encryption ends for the link | WS-3, WS-4 | Items 7 and 8b; decisions row 9. |
| Task: issue 1633, a check that the web server passes the long stream on before sprint 3 | WS-3 | Item 7, criteria 3a and 3b. |
| Task: one command on a clean machine, and the website at the loopback address | WS-2 | Item 6 acceptance. |
| Task: pin every image, and bind ports to loopback | WS-1, WS-2, WS-4 | Items 1, 3, 4 and 8a; decisions row 8. |
| Design, "The compose stack": four services, one address, loopback default, port variable 8787, one command | WS-1, WS-2 | Items 2 to 6. |
| Issue 1633, point 1: how the server learns the certificate's fingerprint | WS-4 | Item 8a writes the hash and item 8b mounts the certificate; request S1 asks the server to read it. |
| Issue 1633, point 2: a machine's certificate stops at the web server | WS-4 | Decisions row 9. The web server asks for no client certificate. |
| Issue 1633, point 3: the challenge already proves the machine | WS-4 | Decisions row 9, after the contract plan's decisions. |
| Issue 1633, point 4: the long stream is not cut | WS-3 | Item 7. |
| `deploy/test/README.md`: the compose file, the start command, the four service names, the address, `LGTM_WEB_PORT`, `LGTM_WEB_PASSWORD`, the default server command, the website's mount point and its API | WS-1, WS-2 | Kept unchanged; items 1, 3, 4 and 5. |
| `deploy/test/README.md`: `POSTGRES_USER` and `POSTGRES_DB` for C5 | WS-2 | Decisions row 24 and the result file. Sprint 3 changes them. |
| Sprint 1 "Contract for sprint 2": user 10001, the networks, the database and vault files, the launcher | WS-1, WS-2 | Items 1, 4 and 5. |
| Sprint 1 check PB5: no server starts before the vault's setup succeeds | WS-2 | `up_starts_no_server_before_the_vault`. |
| Sprint 1 check PB8: the full restart with the server | WS-2 | Item 6, check C13. |
| C1 | WS-2 | Items 5 and 6. |
| C2 | WS-1, WS-2 | `server_image_refuses_without_password`, `password_follows_the_checks`, item 6. |
| C3 | WS-1, WS-2, WS-4 | `rules_keep_status_and_body`, item 6, `https_cookie_is_secure`. |
| C4 | WS-2 | `password_follows_the_checks`, item 6. |
| C13 | WS-2 | Item 6. |
| D1, three parts that meet only through the contract | WS-1 | Items 1 to 3. |
| D17, no certificate needs a public authority | WS-4 | Items 8a and 8b. |
| R9, secrets never reach a log or the website | WS-1, WS-2, WS-4 | The allow-lists, the read-only mounts, no access log, and a job that never prints the key. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "What this project is": the three parts meet only through the contract in `src/contracts/`. | The web server passes on only the contract's two packages and the frames path, and no file of this plan reads the server's code. |
| `AGENTS.md`, "Where things are": new code does not go into `packages/`, `design/mock/` or `plugins/`. | Every file of this plan is under `deploy/`. The website image reads `design/mock/` only as the existing build already does. |
| `AGENTS.md`, habit 1: copied code is credited. | No code is copied. The web server rules and scripts are written for this plan. |
| `AGENTS.md`, habit 2: findings go into `docs/issues.md` and ideas into `docs/features.md`. | This plan may edit neither file, so the result file lists one idea and the answers to issue 1633 for the orchestrator to record. |
| `AGENTS.md`, habit 3: finished work reaches the base branch through a pull request. | The plan makes no commit. Each work item merges inside the team's branches after a passing check on its exact commit. |
| `AGENTS.md`, writing style: no quotation of a person, no home folder path, no private network address. | The plan names only loopback addresses, `0.0.0.0` and the reserved name `lgtm.test`. `scripts/check-private.mjs` passes. |
| The epic's team table: the Stack team edits only `deploy/`. | Every changed file is under `deploy/`. Needs of other teams are requests in the result file. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet (its harness-session skill) at launch; this section names none (owner, 2026-10-16).

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Server image | `infra` | `stack/infra` | low | One Dockerfile and its allow-list, in a known shape, with one test script. | no | The server's `main.go` from `go-s2-server-signin` item 1, for its build. |
| 2. Web server rules | `infra` | `stack/infra` | medium | Two short files that decide which requests reach the server, tested with a stub. | yes | Nothing. |
| 3. Website image | `infra` | `stack/infra` | medium | A workspace build with an allow-list of fourteen paths. | no | Item 2. |
| 4. Compose services and settings | `infra` | `stack/infra` | medium | Two services in a set pattern, with credential mounts that sprint 1 defines. | yes | Items 1 and 3, and `go-s1-stack-vault`. |
| 5. Launcher | `infra` | `stack/infra` | medium | Three phases and five checks added to a launcher of fixed shape. | yes | Item 4, and request V1. |
| 6. Guide and sign-in checks | `infra` | `stack/infra` | low | A guide, and checks that already exist. | no | Items 4 and 5, and `go-s2-server-signin`. |
| 7. Stream test | `infra` | `stack/infra` | medium | A small program and a timed test through a real proxy. | yes | Items 2 and 4, to pass. |
| 8a. Certificate | `infra` | `stack/infra` | medium | One script that makes a key that every machine pins. | no | Items 3 and 5. |
| 8b. HTTPS rules | `infra` | `stack/infra` | medium | One file of rules, tested with existing fixtures. | yes | Items 7 and 8a, and `go-s2-server-signin`. |

Items 1 and 7 run alongside item 2, and items 4 and 7 run alongside each other.
Item 3 follows item 2, item 5 follows item 4, and items 8a and 8b come last.

Load: nine test writers and nine builders give 18 seats, and the mutation checks of items 1, 2, 3, 4, 5, 7, 8a and 8b add eight, for 26 execution seats. Alternating the two primary vendors gives 13 seats to each. Every item is `infra` at low or medium, so no seat needs the strongest tier. The orchestrator reads each vendor's remaining weekly allowance at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| — | Not run yet. | The orchestrator runs the review rounds. This planner stops after the self-check. |

</details>
