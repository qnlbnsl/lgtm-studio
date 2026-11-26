# Round 1, architect lens: plan `go-s2-server-signin`

Task identifier: review-go-s2-server-signin-r1-architect. Nonce: d61805. Model and effort: claude-sonnet-5-5, high.

Verdict: REVISE

Top finding: the plan builds a `lgtm-server` program with no `health` command and no table of commands, so the Compose health check of plan `go-s2-stack-web` can never pass and `deploy/up.sh` cannot reach a healthy server (architect-r1 1).

All paths below are relative to the repository root. `plan.md` means `.plans/plans/go-s2-server-signin/plan.md` as it is now, and I give line numbers of that file. I did not read any other file in `attack/`. I did not read `packages/engine/src/client/web/session.ts`, so I make no claim about the old engine.

## Findings

### architect-r1 1. BLOCKER (the stack never becomes healthy, so C1 to C4 and C13 cannot pass on Compose)

- **Statement:** C1, with C2, C3, C4 and C13 behind it.
- **Evidence:**
  - `.plans/plans/go-s2-stack-web/plan.md:259` gives the `server` service the health check `["CMD", "/usr/local/bin/lgtm-server", "health"]`.
  - `.plans/plans/go-s2-stack-web/decisions.md:28` and `:36` say the command is a request to the Server team's plan.
  - `plan.md:427` defines only two shapes of the program: no arguments serves, and `join-token` is "reserved".
  - `plan.md:370` mentions a health probe that does `GET /healthz`, but no item says which program makes that request.
  - `.plans/plans/go-s3-stack-client/plan.md:137` says that `lgtm-server join-token` is reached "through the program's table of commands, which plan `go-s2-server-signin` provides". Its line 125 names item 1 of this plan as the owner of `main.go`. In this plan `main.go` belongs to item 9, and no table of commands exists.
- **Failure scenario:** The image has no shell and no download tool, so the only way to answer the health check is a subcommand of the program. A builder who follows this plan writes `main` with no subcommand `health`. The program then treats the argument as an unknown command or starts serving, so the check never exits zero, and the container stays `unhealthy`. The launcher runs `docker compose up -d --wait --wait-timeout 120 --no-deps server`, so it fails after 120 seconds. C1 fails, and every check behind it is unreachable. Sprint 3 then has to guess how to add `join-token`.
- **Fix:**
  - In item 9, define `main` as a table of commands with three entries: no argument (serve), `health`, and `join-token` (a function that sprint 3 fills; until then it returns exit status 2 with one sentence).
  - Define `health` exactly. It reads `LGTM_LISTEN_ADDR` (see finding 2), replaces an empty or wildcard host with the loopback address, sends `GET /healthz` with a 2-second timeout, and exits 0 only on status 204. It must not read the password, the database or the vault.
  - Add a test `TestHealthCommand` in `src/server/cmd/lgtm-server` that runs against a started server and against a closed port.
  - Fix the item number that `go-s3-stack-client` cites by telling the orchestrator, because `main.go` is item 9 here.

### architect-r1 2. MAJOR (the server would ignore or misread the settings that Stack writes)

- **Statement:** D10 and C2.
- **Evidence:**
  - `plan.md:355-368` names `LGTM_SERVER_LISTEN_ADDR`, the four settings `LGTM_POSTGRES_HOST`, `_PORT`, `_DATABASE` and `_USER`, and `LGTM_ALLOW_PRIVATE_STACK`.
  - `.plans/plans/go-s2-stack-web/plan.md:260` and `decisions.md:34` set `LGTM_LISTEN_ADDR`, a single `LGTM_POSTGRES_URL` that has no password, and `LGTM_TLS_CERT_FILE`. They never set `LGTM_ALLOW_PRIVATE_STACK`.
  - `.plans/plans/go-s1-stack-vault/plan.md:344` fixes the five names `LGTM_POSTGRES_PASSWORD_FILE`, `_HOST`, `_PORT`, `_DATABASE` and `_USER`. So the server plan matches sprint 1, and the sprint 2 stack plan departs from it.
- **Failure scenario:** The stack starts the server with `LGTM_LISTEN_ADDR=:8080` and a URL. The server reads `LGTM_SERVER_LISTEN_ADDR` (unset), so it binds the loopback address on port 8080 inside the container, and the proxy and the health check cannot reach it. Separately, with `LGTM_ALLOW_PRIVATE_STACK` unset, plaintext to the host `postgres` is refused at `Open`, and the server exits at startup. Unknown settings such as `LGTM_TLS_CERT_FILE` are ignored without any diagnostic.
- **Fix:** Settle one table of names in this plan's item 1 and send it to the orchestrator as a request to Stack. My recommendation is to keep this plan's five PostgreSQL names (they are the sprint 1 contract), rename the listener setting to `LGTM_LISTEN_ADDR` (the shorter name that Stack already used), have Stack add `LGTM_ALLOW_PRIVATE_STACK: "true"`, and drop `LGTM_POSTGRES_URL` and `LGTM_TLS_CERT_FILE` from Stack, because no sprint 2 code reads them. Add to `TestReadServerSettings` a case that sets an unknown `LGTM_*` name which the plan did not list, and require a one-line warning on standard error so that a renamed setting is never silent.

### architect-r1 3. MAJOR (the owner's browser is refused at sign-in unless the address matches `LGTM_PUBLIC_URL` exactly)

- **Statement:** D11 and C3.
- **Evidence:**
  - `plan.md:359` defaults `LGTM_PUBLIC_URL` to the loopback address with port 8787. `plan.md:370` says that Stack translates `LGTM_WEB_PORT` into it.
  - `.plans/plans/go-s2-stack-web/plan.md:260` passes `LGTM_PUBLIC_URL: ${LGTM_PUBLIC_URL:-}`, which is empty in loopback mode. Nothing in that plan derives it from `LGTM_WEB_PORT`.
  - `.plans/plans/go-s2-stack-web/plan.md:298` allows `LGTM_PUBLIC_URL` only if it starts with `https://`, so Stack can never put the loopback origin there. The test at `.plans/plans/go-s2-stack-web/plan.md:178` shows that the proxy also serves the host name `localhost`.
  - `plan.md:219-221` accepts an origin only if it is the public origin or is listed. The same rules treat `localhost` and a literal loopback address as different sites.
  - `deploy/test/checks.sh:49-55` (function `rpc`) sends no `Origin` header, so C3 and C4 take the "absent Origin" path and cannot see this.
- **Failure scenario:** The owner sets `LGTM_WEB_PORT=18787` (the stack plan's own advice when port 8787 is taken by the old engine) and opens the website. The page loads, and a browser sends `Origin` with port 18787 on the sign-in call. The server computes the public origin from the empty setting, which gives port 8787, so it answers `permission_denied` ("This website origin is not allowed."). The same happens when the owner types `localhost` instead of the loopback address. Every check in the driver stays green.
- **Fix:** In loopback mode (the setting `LGTM_PUBLIC_URL` is unset), define the public origin as the origin of the request that Stack's proxy sends, not a constant. The simplest rule uses the setting that this plan already has: in loopback mode Stack sets `LGTM_ALLOWED_ORIGINS` to the two loopback spellings (the numeric loopback address and `localhost`) at the port of `LGTM_WEB_PORT`. Both are the same site as the default public origin, so the cookie stays `SameSite=Strict`. Delete the sentence at `plan.md:370` that says Stack translates `LGTM_WEB_PORT` into `LGTM_PUBLIC_URL`, because Stack's own validation forbids it. Record that rule in item 1 as a request to Stack and add to `TestBrowserOriginGate` the case "port other than 8787, with `Origin` set, through the public origin". Add a post-build check (below) that sends `Origin` on the driver's sign-in.

### architect-r1 4. MAJOR (a session issued under the old password stays valid under the new password if two servers ever share the database)

- **Statement:** C4 and D5.
- **Evidence:** `plan.md:61` and `:155` say that a request never recomputes the binding and that only `Reconcile` checks it. `plan.md:427` says "The server runs as a single instance. Deployment stops the old process before starting one with a different password." Nothing in the code enforces this.
- **Failure scenario:** An operator, or a failed recreate, leaves server A (password P1) running and starts server B (password P2) against the same PostgreSQL. B's `Reconcile` purges every row, then A signs the owner in again and writes a row bound to P1. B's `Check` looks up only the verifier and the expiry, so it accepts that cookie as valid under P2. The design's rule "changing the password ends every session before the server opens its port" is broken, and no test sees it, because the tests start one server per database.
- **Fix:** Make the single instance real. After `Migrate` and before `Reconcile`, `Start` takes a session-level PostgreSQL advisory lock on a dedicated connection with `pg_try_advisory_lock` and holds it until `Close`. If the lock is held, it exits nonzero with one sentence. Choose the lock number from the Store plan's list of lock numbers (it already holds 841941100001 for the feed and 841941100002 for migration), and ask the Store team to add the new number to that list so that the numbers cannot collide. Add `TestStartRefusesSecondInstance` to `start_test.go`.

### architect-r1 5. MAJOR (the server could refuse the PostgreSQL password file that Stack mounts)

- **Statement:** R9 and C1.
- **Evidence:** `plan.md:365` calls the file "a protected file" and `plan.md:372` gives no ownership or mode rule for it. `plan.md:395` states a rule only for the OpenBao credential file: owned by the process user, mode 0400 or 0600. `.plans/plans/go-s1-stack-vault/plan.md:344` mounts the PostgreSQL password file with owner 70, group 10001 and mode 0440.
- **Failure scenario:** The builder of item 1 reuses the only stated rule (owner equals process user). The file is owned by user 70, so `Load` rejects it, the server exits at startup, and the whole stack stays unhealthy. If the builder instead skips the check, a world-readable file is accepted.
- **Fix:** State the rule for this file in item 1: a regular file, not a symbolic link, with no permission bit for "other", no write bit for group, and group or owner equal to the process identity. Name the Stack mount (owner 70, group 10001, mode 0440) in `TestReadServerSettings` as the accepted case, and name mode 0644 as the refused case.

### architect-r1 6. MAJOR (sprint 3 cannot build its tests on the public test package as written)

- **Statement:** R10 and issue 1634.
- **Evidence:** `plan.md:431-449` fixes the exported surface of `servertest` as "exactly" `Options{Password}`, `Server{URL, HTTPClient, SigningPublicKey}`, `Start`, `Stop` and `Restart`. `.plans/plans/go-s3-stack-client/plan.md:146` needs the settings of the started server (a `getenv` "that returns the same settings as that server") to call `joinTokenCommand`. `plan.md:449` says the machine work "plugs its handlers and migrations into the same application", but `app.Start(ctx, cfg, log)` at `plan.md:419` has no parameter for either.
- **Failure scenario:** The test writer of `go-s3-stack-client` item 1 cannot learn the database name, the OpenBao fixture address or the credential file of the test server, so the test either cannot be written or reads the private fixture through an extra import that issue 1634 forbids.
- **Fix:** Add one method to `servertest.Server`, `Environment() map[string]string`, which returns a fresh copy of the settings that the application received (including the database password file path and the OpenBao fixture path). State in item 9 that sprint 3 adds handlers and migrations by editing `app.Start` directly (the repository's rule is to edit earlier code, not to add seams), and delete the sentence "plugs into", or turn it into an `app.Options` struct with fields `Clients`, `Link` and `Migrations` now, which costs three lines.

### architect-r1 7. MINOR (a leaked cookie can come back to life if the database is down when the password is removed)

- **Statement:** C4.
- **Evidence:** `plan.md:427` and `decisions.md:16` (row 9) accept that when the purge cannot commit, the operator must repeat the removal before restoring the old password. `plan.md:467` calls this "fail closed".
- **Failure scenario:** The owner removes the password because a cookie leaked, and PostgreSQL is stopped at that moment. The server cannot purge, exits, and never opens a port. The owner then restores the same old password and starts the stack. `Reconcile` finds bindings that match, so the leaked cookie is valid again. Nothing in the plan makes this visible.
- **Fix:** Keep the design (the state must live in the database), but make the gap loud. Print a fixed diagnostic that names the exact step ("Sessions were not revoked; start the server once with PostgreSQL available and no password before restoring the old one."), and say the same in the stack guide through the orchestrator. Add the diagnostic to `TestStartRevokesBeforeListen`.

### architect-r1 8. MINOR (the startup order is spread over three places and leaves two cases undecided)

- **Statement:** D3 and R4.
- **Evidence:** `plan.md:372` ("validate database settings first"), `plan.md:360` (`Load` rejects empty entries in `LGTM_ALLOWED_ORIGINS`), `plan.md:427-429` (open, migrate, reconcile, origin policy, vault, followers, listener) and `decisions.md:19` (row 12). `plan.md:121` says that with a good password "remove only expired rows" but, unlike the other branches, does not say that each removed row gets a `Tx.Record` call.
- **Failure scenario:** A bad `LGTM_ALLOWED_ORIGINS` makes `Load` fail before the purge, so a password change plus a typo leaves old sessions in the table (harmless while the server is down, but a builder may implement either order). The expired-row removal without a feed record breaks R4 ("every write says what changed") for a later Watch reader of topic `web_sessions`.
- **Fix:** Add one ordered list to item 9: load and validate database and password settings only; open the store; migrate; take the instance lock (finding 4); reconcile; validate origins, listener and OpenBao settings; read the key; start background work; listen. Say that `Load` returns the later settings unvalidated, and name where each is validated. State that every deleted row, expired or not, is recorded as `removed`.

### architect-r1 9. MINOR (the two modules are named without a version, and the epic requires one)

- **Statement:** plan (the epic's rule at `.plans/epics/go-rewrite/epic.md:129-131`).
- **Evidence:** `plan.md:81` names `golang.org/x/crypto/scrypt` and `plan.md:201` names `golang.org/x/net/publicsuffix`, with no version for either. I looked both up in the Go module proxy: `golang.org/x/crypto` has v0.59.0 and `golang.org/x/net` has v0.63.0 as their latest tagged versions, both published in November 2026. I did not check their minimum Go version against the module's `go 1.26.0` line, though no release of either needs more than the Go version of the same month.
- **Failure scenario:** The orchestrator has no pinned version to add before items 2 and 5, so the version is chosen at build time and is not reviewed.
- **Fix:** Add a "Modules" line to the plan: `golang.org/x/crypto v0.59.0` before item 2, and `golang.org/x/net v0.63.0` before item 5. The Store plan's pgx pin (`github.com/jackc/pgx/v5 v5.13.0`) already covers `pgxpool`, so item 1 needs nothing more. Connect v1.21.1 and protobuf v1.36.12 come from sprint 0.

### architect-r1 10. MINOR (the loser of the first-start race may get 403, not the conflict the plan expects)

- **Statement:** C13.
- **Evidence:** `plan.md:399-401` classifies only an HTTP 400 as the compare-and-set conflict and fails startup on an authorization error. `.plans/plans/go-s1-stack-vault/plan.md:261` grants read, create and update on the key path. OpenBao decides between "create" and "update" by whether the key exists when the write arrives.
- **Failure scenario:** The Stack team later trims the server policy to read and create, as the plan's "create-only" wording suggests. The second of two servers that start together then gets 403 on its create, not 400, so its startup fails and the stack restarts it. The fixture of `TestLinkSigningKeyCreateRace` is written to return 400, so the test stays green.
- **Fix:** After a 403 on a create that followed a 404 read, make one fresh read, and use the stored seed if it is present. Add a fixture case for it. Tell the Stack team that the `update` grant is required for the race and should stay.

### architect-r1 11. MINOR (the key event could lose its `source` field)

- **Statement:** C13.
- **Evidence:** `plan.md:429` logs "one JSON event" with `msg`, `source` and `fingerprint`. The driver at `deploy/test/checks.sh:610-613` selects on `.source == "openbao"` and a string `fingerprint`. The standard `log/slog` package reserves the key `source` for the call site when the handler option `AddSource` is on.
- **Failure scenario:** A builder turns on `AddSource` for the production handler. The event then holds a `source` object, `jq` finds no `openbao` match, and C13 reports that the server never loaded its key.
- **Fix:** Name the handler in item 9 (a `slog.NewJSONHandler` with `AddSource` false) and have `TestStartWithUnavailableVault` decode the captured line and assert the three exact keys and string values.

## Answers to the eight questions of the architect lens

1. **Claims about existing code and tools:** One miss. Every anchor that points at a written plan or at code resolves: the Store names (`ConnectionPolicy{AllowPrivateStackPostgres}`, `Open`, `Store.Read`, `Store.Write`, `Tx.Record`, `ChangeRef`, `Migrations`, `Migrate(ctx, parts...)`, `StartFollower(ctx, s, report)`, `RunPruner`, `RequestKeyHeader`, `storetest.NewDatabase`), the Stack contract for sprint 2 (`.plans/plans/go-s1-stack-vault/plan.md:338-352`), `session.proto` and the driver functions `get_server`, `signin`, `signin_rejected`, `unauthenticated`, `server_key_fingerprint`, `check_c2`, `check_c4` and `check_c13`. The one claim that does not resolve is the health probe of finding 1. The files that the plan targets do not exist yet (`src/server/` and `go.mod` are absent), which is expected for sprint 2.
2. **Writers and readers of public names:** Forgotten readers are the Stack plan (`health`, listener and PostgreSQL names; findings 1 and 2), `go-s3-stack-client` (`join-token` table, `servertest` settings; findings 1 and 6) and the future Watch authorizer in sprint 3 (`CheckInRead`, named in the plan). `Verifier`, `SessionVerifier` and `CookieName` have one writer and the website as the only reader.
3. **Placement:** Right. The contract is untouched, the server holds sessions in its own `auth` part, the vault client is the only OpenBao code, and Stack owns the images and the compose file. `servertest` sits in `src/server/`, as issue 1634 needs.
4. **R1 to R11:** R1 kept (`auth`, `api`, `vault`, each with one folder and one migration part). R2 kept (the contract file `session.proto` exists and is frozen). R3 kept (`api` calls `auth.Sessions`; the table belongs to `auth`). R4 kept, except that finding 8 shows one unrecorded path. R5 does not apply (no `Watch` here; the hook is named for sprint 3). R6 does not apply. R7 kept (every request reads the table; the key is re-readable from OpenBao). R8 kept, with one exception: a refused browser request with no recognised media type gets a bare 403 (`plan.md:256`), which has no Connect code and sentence. I accept it because the browser cannot read the body of such a refusal anyway. R9 kept (fixed error sentences, no values in logs; the fingerprint is public). R10 kept (real PostgreSQL; the vault tests use a wire fixture and C13 covers the real one). R11 does not apply (nothing is copied), although the traceability row at `plan.md:486` says it is "retained".
5. **What it takes and gives:** It takes the Store interface and the Stack handoff correctly by name, but not the Stack plan of sprint 2 (findings 1, 2 and 3). It gives `servertest`, `join-token` and the session hook to sprint 3, and findings 1 and 6 name the gaps. The epic's agreement 3 (vault credential) matches both sides. Agreement 5 (test entry points) is only half met (finding 6).
6. **Merging alone:** Items 1, 2, 5 and 8 can merge alone. Items 3, 4, 6 and 7 can merge once their listed prerequisites are in, and each has its own tests, so a passing full check at each commit is possible. Item 9 is the only item that produces a runnable program, so until it merges no image can be built (the Stack plan's item 1 and the C1 to C4 checks wait for it). Items 6 and 7 need the sprint 0 generated code, which is present by then. At the commit between items 7 and 9 nothing is broken, because nothing calls the new packages.
7. **Room for wave 2 without building it:** Yes. `Options.Clients` and `Options.Link` stay nil, the Link procedures answer `unimplemented`, and the feed references and `CheckInRead` are in place for the sprint 3 Watch hook. Built beyond the wave 1 checks: the cross-site cookie mode and the public suffix list (the design's D11 asks for them, and no check exercises them), gRPC and streaming tests, and the feed references of `web_sessions`. I accept them because the design or R4 requires each, but they carry the most review cost, and finding 3 shows the common case (loopback) needs more care than the cross-site case.
8. **Dependencies:** Two modules are added with no version (finding 9). Both exist in the Go module proxy at the versions above. `http.Server.Protocols` and unencrypted HTTP/2 need Go 1.24 or later, and the module sets Go 1.26.0, so they are available. No image is added by this plan.

## Answers to the plan-specific questions

### a. Sessions and the password (C2 to C4)

| State | Rows before | What the plan does | Result |
|---|---|---|---|
| First start, password set | none | Migrate, reconcile (nothing to do), serve | Correct. |
| Restart, same password | up to 8 live | Every binding is recomputed with scrypt and matches; only expired rows go | Sessions survive (C4). Correct. |
| Password changed, then restart or recreate | live rows | A binding fails, so the whole set is deleted in one transaction before the listener opens | Sessions end (C4). Correct. |
| Password removed, empty or invalid; PostgreSQL up | live rows | Rows deleted and committed, then exit nonzero naming `LGTM_WEB_PASSWORD` | Correct (C2). |
| Password removed; PostgreSQL down | live rows | No purge, exit nonzero | Sessions stay, and return if the old password is restored (finding 7). |
| Password restored after a committed purge | none | Nothing to revive | Old cookies stay invalid (C4). Correct. |
| Wrong password at sign-in | unchanged | `REJECTED`, no cookie, no row, no database read | Correct (C3). |
| Missing cookie on `SignOut` | unchanged | `unauthenticated`, HTTP 401 | Correct (C3). |
| A second server with a different password on the same database | rows written by the first | Purge at start, but the first keeps issuing and the second accepts its rows | Wrong (finding 4). |
| One row tampered by a database writer | live rows | The next start purges the whole set | Fails closed. Between starts the row is accepted, because requests do not check the binding. |
| Crash during the purge | live rows | One transaction, so all or nothing | Correct. |
| 720 hours pass while running | old rows | The expiry predicate uses database time | Correct. |
| Sign-in whose commit result is lost | one extra row | The cookie is not returned; the row expires or is evicted | Correct and stated. |

### b. Cookie, origins and the stored verifier

- **Same-site website:** `lgtm_session`, host-only, `Path=/`, `HttpOnly`, `Max-Age=2592000`, `SameSite=Strict`, and `Secure` exactly when the configured public address is HTTPS.
- **Listed cross-site website:** the same, with `SameSite=None` and `Secure`; both origins must be HTTPS or the server refuses to start.
- **An unlisted site:** The origin check runs before the body or the cookie is read. A cross-site form post, a plain-text post or a preflight carries `Origin`, so each gets a 403 with no permission headers and no dispatch. A request with no `Origin` and `Sec-Fetch-Site: cross-site` is refused. The `Strict` cookie is not sent in any of these cases. The plan never consults `Host` or `X-Forwarded-*`, so a forged forwarded scheme cannot change the mode. An unlisted same-site sibling (a different subdomain or port) is refused too. I found no hole in the unlisted-site path. Finding 3 is about the opposite problem, that the right site can be refused.
- **Reading the database:** The table holds the SHA-256 of a 256-bit random token. A reader of the table cannot produce a token that hashes to a stored value, so they cannot sign in. The binding does let a reader guess the password offline, at the cost of one scrypt run (N=65536) per guess; this is the design's own choice. A writer to the table can insert a row with an arbitrary binding and use it until the next start (see the table above), but that writer already controls the whole database.

### c. The vault client and the signing key

- **First start:** read returns 404 without version metadata, the client posts a random seed with `cas=0`, reads again, and uses the stored seed. Correct.
- **Two starts at once:** both see 404, one create succeeds, and the other gets the conflict (HTTP 400, provided the policy keeps `update`; finding 10), reads the winner and uses it. Both load the same key. The `Migrate` step is serialised by the Store's own lock, so migrations do not race either.
- **Vault sealed or unreachable:** login or read fails with a fixed category, the server exits nonzero before it listens, and Compose restarts it. A transport or gateway failure on the first read never authorises a create. A 404 caused by a missing mount cannot be told from a missing key, but the create then fails and startup fails, so no key is invented.
- **Stored key differs from the one in memory:** The server always uses the value it read back after a create or a conflict, never its own candidate. A key that is replaced later in OpenBao is picked up only at the next start. That is correct for this sprint, because rotation is a later decision (issue 1639).
- **Exact names:** The plan matches the Store names listed above and the Stack contract: the path `lgtm/data/server/link/signing-key`, the field `private_seed_b64`, the file `openbao.json` with `role_id` and `secret_id`, the five PostgreSQL names, the setting names `LGTM_OPENBAO_ADDR` and `LGTM_OPENBAO_CREDENTIAL_FILE`, the 15-minute batch token, and the event `link_signing_key_loaded`. Both plans were still in review at the time I read them, and `.plans/plans/go-s1-stack-vault/plan.md` has uncommitted changes in the working tree, so the Store and Stack contracts should be compared again when they are frozen.

## Post-build checks

1. **Health command.** Run `deploy/up.sh` on a clean machine. Expected: the `server` service reaches `healthy` within 120 seconds, and `lgtm-server health` exits 0 inside the container and exits 1 when the server is stopped.
2. **Port other than 8787.** Start the stack with `LGTM_WEB_PORT=18787` and send `SignIn` with the right password and the header `Origin` set to the website's own origin at that port. Expected: `SIGN_IN_RESULT_SIGNED_IN` and a `Set-Cookie` with `SameSite=Strict`. Repeat with the `localhost` spelling. Expected: the same result.
3. **Unlisted origin.** Send `SignIn` with the right password and an unlisted `Origin`. Expected: HTTP 403, `permission_denied`, no `Set-Cookie`, and no new row in `web_sessions`.
4. **Second instance.** Start a second `lgtm-server` with a different password on the same database while the first runs. Expected: the second exits nonzero with one sentence and purges nothing.
5. **Password file.** Mount the PostgreSQL password file with owner 70, group 10001 and mode 0440. Expected: the server starts. Change the mode to 0644. Expected: the server exits with a fixed diagnostic that names `LGTM_POSTGRES_PASSWORD_FILE`.
6. **Restart and change (C4).** Sign in, run `compose restart server`, then `GetServer` with the cookie. Expected: `signedIn` is true. Recreate with a changed password. Expected: the old cookie gets 401 on `SignOut`, and `SELECT count(*) FROM web_sessions` is 0 before the new sign-in. Restore the old password. Expected: the old cookie is still refused.
7. **Stored verifier.** After a sign-in, copy the `verifier` column in base64 and send it as the cookie. Expected: 401. The table holds no 43-character token.
8. **Nine sign-ins.** Sign in nine times. Expected: eight rows, and the first cookie is refused.
9. **Race on first start.** Start two servers at once on an empty OpenBao. Expected: both log the same `fingerprint`, and the secret has exactly one version.
10. **C13 log shape.** After `compose up -d --force-recreate` of all services, run `jq` on the server log. Expected: exactly one line with `msg` equal to `link_signing_key_loaded`, `source` equal to the string `openbao`, and the same `fingerprint` as before the recreate.
11. **Public test package.** A test in package `servertest_test` starts, stops and restarts the server, reads `Environment()`, and calls `joinTokenCommand`. Expected: the same URL and public key after restart, and no import of an `internal` package.

## Grade

Grade: 6 out of 10. The plan is careful about sessions, bindings and the vault race, and it matches the Store and Stack names of sprint 1. It loses points because its contact with the sprint 2 Stack plan is not closed. Counts: 1 blocker, 5 majors, 5 minors.
