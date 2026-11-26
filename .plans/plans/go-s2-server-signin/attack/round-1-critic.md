REVISE

# Round 1 review of go-s2-server-signin, lens critic

- Task identifier: review-go-s2-server-signin-r1-critic
- Nonce: 05be32
- Reviewed text: `.plans/plans/go-s2-server-signin/plan.md` as of 2026-11-26, the same text as `attack/plan-v1.md`.
- Finding prefix: critic-r1.

## Top finding

The compose environment that `go-s2-stack-web` writes uses setting names that the server settings of this plan do not read, so the server container would listen on the wrong address and look for PostgreSQL in the wrong place (critic-r1 1).

## Findings

### critic-r1 1 — MAJOR — plan

The sprint 2 stack plan and this plan name the server's settings differently, so the stack cannot start the server as written.

- **Evidence.** `.plans/plans/go-s2-server-signin/plan.md:355-368` reads `LGTM_SERVER_LISTEN_ADDR`, the five `LGTM_POSTGRES_*` settings and `LGTM_PUBLIC_URL`. `.plans/plans/go-s1-stack-vault/plan.md:343` (Contract for sprint 2, row Database) promises the same five `LGTM_POSTGRES_*` names. `.plans/plans/go-s2-stack-web/plan.md:260` instead sets `LGTM_LISTEN_ADDR`, `LGTM_POSTGRES_URL`, and `LGTM_TLS_CERT_FILE`, and sets none of `LGTM_POSTGRES_HOST`, `LGTM_POSTGRES_PORT`, `LGTM_POSTGRES_DATABASE`, `LGTM_POSTGRES_USER` or `LGTM_SERVER_LISTEN_ADDR`.
- **Failure scenario.** The compose file passes `LGTM_LISTEN_ADDR=:8080` and a PostgreSQL URL. `config.Load` ignores both and falls back to the standalone defaults. The server then listens on the container's loopback only and dials PostgreSQL on loopback. The proxy cannot reach the server, and C1 fails. The sign-in tests all pass, because they use the sign-in plan's own names.
- **Fix.** Add a table to the plan, "Names the stack must set in compose", that lists exactly the settings of item 1 with their compose values. Record in the plan that the orchestrator compares that table with `go-s2-stack-web` at the end of phase 3. Also state that this plan does not read `LGTM_POSTGRES_URL`, `LGTM_LISTEN_ADDR` or `LGTM_TLS_CERT_FILE`, so the stack plan must drop them or the orchestrator must agree a rename. The vault plan's names win, because that plan is the one the epic names as the source (epic.md:178).

### critic-r1 2 — MAJOR — C3

The only origin the server accepts by default is a fixed public address, and nothing in the plan or in the stack makes it match the address the owner types.

- **Evidence.** `plan.md:359` defaults `LGTM_PUBLIC_URL` to the loopback address on port 8787. `plan.md:370` says the stack translates `LGTM_WEB_PORT` into `LGTM_PUBLIC_URL`, but `.plans/plans/go-s2-stack-web/plan.md:260` passes `${LGTM_PUBLIC_URL:-}`, which is the empty string, and its line 298 rejects any public address that is not `https://`. The web server accepts the Host names `localhost`, the loopback IPv4 address and the loopback IPv6 address (`.plans/plans/go-s2-stack-web/plan.md:164`). `plan.md:219` and `plan.md:221` accept the configured origin only and never read `Host`. `plan.md:358` makes the standalone listen default port 8080, while the public default is port 8787. The check driver sets no `Origin` header (`deploy/test/checks.sh:36` and the helpers at lines 60 to 100), so no scripted check sees the browser path.
- **Failure scenario.** One. The owner starts the stack with `LGTM_WEB_PORT=18787` and opens the website in a browser. The browser sends `Origin` with port 18787. The server's public origin is still port 8787. Every call answers 403 and the owner cannot sign in, while `checks.sh --only C3` passes. Two. The owner opens `localhost` on port 8787 instead of the numeric loopback address. The web server serves the page, and every call is refused by origin. Three. A developer runs the standalone server on its default port 8080 and opens it in a browser, and gets the same refusal. In all three cases the person sees only "This website origin is not allowed.", which names no remedy.
- **Fix.** In item 1, say that an empty `LGTM_PUBLIC_URL` counts as unset (the compose file passes an empty value), and that the standalone default is built from the listen address (`http://` plus `ListenAddr`) rather than a fixed port. In item 5, say that when the public origin is plaintext loopback, the same scheme and port on the other two loopback spellings is accepted and gets the same `Strict` cookie, because the website and the server are then served by one proxy. Add one `TestOriginPolicies` row for each rule. Add a scripted check, in the plan's Verify line of WS-2, that signs in with `curl` and an explicit `Origin` header for a non-default `LGTM_WEB_PORT`. The stack must also pass the actual published port in `LGTM_PUBLIC_URL` (see critic-r1 1).

### critic-r1 3 — MAJOR — plan

The plan names no module versions, which the epic requires of every plan from sprint 1 on.

- **Evidence.** `.plans/epics/go-rewrite/epic.md:129-131` says a plan lists each module and version it needs, and the orchestrator adds them before the first item that imports them. `plan.md:81` and `plan.md:201` only say "the root dependency handoff" for the scrypt and public suffix packages. The sprint 0 plan pins only `connectrpc.com/connect v1.21.1` and `google.golang.org/protobuf v1.36.12` (`.plans/plans/go-s0-contract/plan.md:220`). No document lists `golang.org/x/crypto`, `golang.org/x/net`, or the PostgreSQL driver for this plan.
- **Failure scenario.** The builder of item 2 cannot compile `scrypt`, and the builder of item 5 cannot compile `publicsuffix`. The team may not edit `go.mod`, so both items stop until the orchestrator guesses versions. A test writer's "red for the right reason" run cannot even build the stub.
- **Fix.** Add a section "Modules this plan needs" with a table of module path, exact version, the first item that imports it and the package that is used. Cover `golang.org/x/crypto` (`scrypt`), `golang.org/x/net` (`publicsuffix`), the OpenBao client needs none because it uses the standard library, and name the Connect, protobuf and PostgreSQL driver versions already pinned by the earlier plans. Choose released versions that are at least two weeks old, and confirm each with `go list -m -versions`.

### critic-r1 4 — MAJOR — C2

The startup diagnostics are called "fixed" but no sentence is given, and the shape of `config.Load` on several errors cannot be tested.

- **Evidence.** `plan.md:427` says to "report `LGTM_WEB_PASSWORD`" and `plan.md:429` says "return fixed diagnostics". `plan.md:372` says a malformed database setting "is reported together with" the missing password diagnostic, but `plan.md:350` gives `Load` the signature `(Config, error)`, and `plan.md:357` says a missing password does not make `Load` fail. Item 8's missing or unreadable credential file is covered only inside `TestOpenBaoLeaseAndFailures` (`plan.md:409`). `deploy/test/checks.sh:396` requires only that the log names `LGTM_WEB_PASSWORD`.
- **Failure scenario.** A test writer cannot assert the C2 sentence, the stream it goes to, or the exit status. With the password missing and the database port malformed, `Load` returns one error and no `Config`, so the app has no way to "purge" or to report both. A builder can choose either behaviour and pass the frozen tests. A missing credential file with a valid database gives no tested diagnostic at all.
- **Fix.** Add a table of startup diagnostics with the exact sentence, the stream, and the exit status for these cases: password missing, empty, invalid or overlong; database setting malformed; database unreachable; purge failed; credential file missing, wrong owner or wrong mode; vault unreachable or sealed; key unreadable. For example: "LGTM_WEB_PASSWORD is not set or is not a usable password. The server did not start." Change `Load` to return a `Config` plus a list of setting errors, or state that it returns the error only after filling in everything it could. Add one named test, `TestStartupDiagnostics`, that asserts each row.

### critic-r1 5 — MINOR — plan

The owner has not answered the request-key rule, and the plan builds on the assumption.

- **Evidence.** `decisions.md` row 7 is `unanswered`. `plan.md:28` and `plan.md:290` refuse a request key on SignIn and SignOut. The design's "Safe retries" section (`docs/specs/2026-11-25-go-rewrite-foundation-design.md:176-182`) says a call that changes something may carry a key, and the store plan leaves authentication mutations to a receipt policy decision.
- **Failure scenario.** A lost SignIn reply leaves one orphan session, bounded by the eight-session cap and the 720-hour expiry. Nothing breaks today, because the website never sends `Lgtm-Request-Key` (no match in `src/frontend/src`). If the owner chooses the other policy, items 6, 7 and the receipts test change.
- **Fix.** Keep the assumption, and list row 7 in the plan's attack log as a decision that needs the owner before build, not only an assumption. State the cost of each answer in one sentence each.

### critic-r1 6 — MINOR — C4

Sessions are valid or ended correctly in every single-process state, but three edges are left to the operator or to chance, and the plan states only some of them.

- **Evidence.** `plan.md:61` and `plan.md:155` check a session by its verifier alone. `plan.md:121` recomputes bindings only at startup. `plan.md:427` says the operator must repeat a removal when the database was unreachable, and says the server runs as a single instance.
- **Failure scenario.** One. A person with write access to the database inserts a row with a known token hash and any binding, and gets a valid session until the next start, when reconciliation deletes every session. Two. The operator removes the password while the database is down, restores it, and the old sessions revive, because no start ever reached the database. Three. A second process started by mistake with a different password purges the sessions of the first, which then issues new ones bound to the old password and purged again at the next start.
- **Fix.** State the three edges in the Risks table of WS-1 as accepted limits with one sentence each. Add one test, `TestReconcileTamperedRowIsRejectedAtStart`, to the item 3 list, which inserts a forged row and expects a purge at the next start. Say in item 9 that a held PostgreSQL advisory lock for the process lifetime is not part of this sprint.

### critic-r1 7 — MINOR — D11

The cross-site cookie can be accepted and then dropped by the browser, and the plan gives the person no sign of it.

- **Evidence.** `plan.md:221` gives a listed cross-site website `SameSite=None` and `Secure` with no `Partitioned` attribute. `plan.md:310` says "Browser policy controls cross-site cookie availability" and tests only a profile that allows such cookies.
- **Failure scenario.** In a browser that blocks third-party cookies, SignIn returns the signed-in result and the next call is unauthenticated. The person sees the sign-in screen again with no explanation.
- **Fix.** Add a row to `decisions.md` that records whether the cookie is partitioned, and add a post-build check for a blocking browser that records what the person sees. If the owner wants no change, say so in the plan so the limit is a decision and not a surprise.

### critic-r1 8 — MINOR — plan

Several acceptance rows omit named tests, and one observation is not a command.

- **Evidence.** The Done-when row 2 (`plan.md:68`) omits `TestSessionMigration` and `TestReconcileRollsBack`, although `plan.md:129` says the three tests gate acceptance. Row 4 (`plan.md:187`) ends with "the browser checks below". Row 7 (`plan.md:322`) omits `TestSettingsErrorsRedactValues`. Row 8 (`plan.md:323`) omits `TestOpenBaoLeaseAndFailures`. The cookie row omits `TestReadsOneCanonicalCookie`. `plan.md:159` tests a session "exactly 720 hours old", but the database clock moves between setup and check, so that test cannot tell `>` from `>=`.
- **Failure scenario.** A test writer reading only the table skips five named tests. The mutation of the expiry comparison survives.
- **Fix.** Add each omitted test to its row. Replace "the browser checks below" with the step list of the WS-2 Verify line. Change the boundary test to 720 hours minus one minute (live) and 720 hours plus one minute (expired).

### critic-r1 9 — MINOR — R8

The error table is incomplete, so a refusal can reach the website without its one-sentence message.

- **Evidence.** `plan.md:258-266` lists seven rows. `plan.md:290` returns `invalid_argument` with its own sentence and `resource_exhausted` with none. `plan.md:250` and `plan.md:252` return bare HTTP 400 and 405. `plan.md:254` returns a bare 403 for a refused preflight.
- **Failure scenario.** An over-limit body returns `resource_exhausted` with whatever message the builder writes. The test writer has no expected text.
- **Fix.** Add rows for `invalid_argument` ("This call does not accept a request key.") and `resource_exhausted` (give one sentence), and say that the 400, 403 and 405 answers are bare by design because a browser cannot read them across origins.

### critic-r1 10 — MINOR — R9

The split of the binding derivation is required but unnamed, so the items that share it cannot be tested together.

- **Evidence.** `plan.md:92` asks for "a private derivation function" so that issuance derives before the lock and computes the tag after it, but the item 2 code block shows only `bindPassword`. Item 4 (`plan.md:151`) relies on the split.
- **Failure scenario.** The test writer of item 4 cannot call the pieces. A builder can derive the key inside the table lock, and holding the lock across roughly 64 MiB and 100 ms of scrypt work per sign-in passes every named test.
- **Fix.** Add `deriveBindingKey(p Password, salt [16]byte) ([32]byte, error)` and `bindingTag(key [32]byte, verifier [32]byte, createdAt time.Time) [32]byte` to the item 2 block, with `bindPassword` composing them. Add one assertion to `TestSessionIssueSerializes` that the writer waiting on the table lock has already finished its derivation.

### critic-r1 11 — MINOR — plan

Items 6 and 9 each bundle several concerns under one medium rating.

- **Evidence.** Item 6 (`plan.md:233-274`) holds routing, the guard, cross-origin preflights, error mapping and three transports, with three large tests. Item 9 (`plan.md:413-457`) holds the application start order, shutdown, the command and the public test package, and depends on seven other items.
- **Failure scenario.** One builder holds two files' worth of unrelated failures. A fault in the preflight rules blocks the server command.
- **Fix.** Split item 6 into the router and guard, and the preflight and error mapping. Split item 9 into the application and command, and the `servertest` package. Rate the router and application items high if they stay whole.

## Answers to the attack questions

1. **Can a test writer write the tests from the item alone?** Items 2, 4, 5 and 8 name tests, inputs and expected results well. Item 1 lacks expected diagnostics (critic-r1 4). Item 4 lacks the derivation function name (critic-r1 10). Item 3's expiry boundary is untestable as written (critic-r1 8). Item 9 lacks diagnostic sentences (critic-r1 4).
2. **Does each acceptance observation name a command, test or step, and a result?** All rows name a test or a command. Bare adjectives: "stable" in the overview and "supported" transports in row 6 are left unmeasured. "The browser checks below" in row 4 is not a step (critic-r1 8).
3. **Where does the plan contradict itself, the log, the epic or the design?**
   - `plan.md:355-368` against `.plans/plans/go-s2-stack-web/plan.md:260` (critic-r1 1).
   - `plan.md:370` against `.plans/plans/go-s2-stack-web/plan.md:260` and `:298` (critic-r1 2).
   - `plan.md:358` against `plan.md:359`, the listen and public defaults (critic-r1 2).
   - `plan.md:81` against `.plans/epics/go-rewrite/epic.md:129-131` (critic-r1 3).
   - `plan.md:290` against the design's "Safe retries" lines 176 to 182 (critic-r1 5).
   - `plan.md:129` against `plan.md:68` on which tests gate acceptance (critic-r1 8).
   - Nothing found against `decisions.md` row 9, which matches `plan.md:427`.
4. **Exact files, and edit rights.** Every item names exact files under `src/server/`, which the Server team may edit. No file lies outside. The plan asks for `go.mod` changes only as a handoff, which the epic allows once versions are listed (critic-r1 3). `deploy/` edits are requested from the Stack team only.
5. **Splits, order and shared files.** Items 6 and 9 could be split (critic-r1 11). The order is right: 2, then 3, then 4, then 7, then 9. Items 3 and 4 share the private helpers `lockSessions` and `recordSessionChange` in `reconcile.go`, which is safe only because item 4 needs item 3. No two parallel items write the same file.
6. **What a person sees in each gap state.**
   - Fresh checkout: standalone, the website at the server's own address gets "This website origin is not allowed." (critic-r1 2).
   - No PostgreSQL: the server exits with a fixed message and no listener, but the sentence is unspecified (critic-r1 4).
   - Missing password: the server refuses to start and names the setting.
   - Missing credential file: refused with an unspecified sentence.
   - Second run with the same password: sessions survive, which the plan tests.
   - Half-finished earlier run: a run that died after the migration and before the purge repeats cleanly, because the purge is one transaction (`plan.md:121`). A run that died during a key creation reads the winner on the next start.
7. **Do the 10 gates pass?** Gate 1 passes, with rows 7 to 15 and 17 `unanswered` and each carrying an assumption. Gate 2 passes with the notes in critic-r1 8. Gate 3 passes because no screen changes. Gate 4 passes. Gate 5 passes with a risk row in each workstream. Gate 6 passes with one rollout section and a Verify line in each workstream. Gate 7 is pending by design, since the attack log reads "Pending". Gate 8 passes for the anchors that I spot-checked: `deploy/test/checks.sh` `check_c2`, the session proto, and the store names. Gate 9 passes. Gate 10 passes: no model name appears in the fold.
8. **Writing rules and harness names.** No model name appears. The sizing paragraph at `plan.md:526` names the vendors OpenAI, Anthropic and xAI. Several paragraphs, for example `plan.md:153` and `plan.md:401`, hold four or more claims in one paragraph and some sentences hold two claims. Acronyms are expanded at first use in the items that I read. No home path, network address or quotation of a person appears in the plan. The only addresses are loopback defaults.

### Task question a — sign-in, restart and password states (C2 to C4)

| State | Rows afterwards | What the browser sees |
|---|---|---|
| Wrong password | Unchanged. | `REJECTED`, no cookie. |
| Right password | One new row, and the oldest is removed when there are nine. | `SIGNED_IN` and one cookie. |
| Password missing, empty, invalid or overlong, database reachable | All rows deleted, including expired ones. | No listener. Exit nonzero. |
| The same, database unreachable | Unchanged. | No listener. Restoring the old password later revives every live session (critic-r1 6). |
| Password changed, database reachable | Every live row fails its binding, so the whole set is deleted. | Old cookies answer `unauthenticated`. Restoring the old password revives nothing. |
| Password changed, database unreachable | Unchanged until the next start that reaches the database. | No listener. The next start purges unless the old password was restored first. |
| Restart with the same password | Live rows kept, expired rows deleted. | Old cookie still works. |
| A row tampered with by someone who writes to the database | Valid until the next start, then the whole set is deleted. | Accepted without a binding check (critic-r1 6). |
| A second process with a different password | The first process's sessions are purged, and its later sessions are purged again at the next start. | Out of scope by the single-instance rule (critic-r1 6). |
| A row exactly 720 hours old | Expired, because the comparison is strict. | The boundary cannot be tested (critic-r1 8). |

The password is read once, so it cannot change while the server runs.

### Task question b — cookie, origin and verifier

- **Same site as the public origin.** The cookie has `HttpOnly`, `Path=/`, `Max-Age=2592000`, no `Domain`, `SameSite=Strict`, and `Secure` when the public address is `https` (`plan.md:221-223`).
- **A listed website on another site.** The cookie has the same attributes except `SameSite=None` and `Secure`. A cross-site entry must be HTTPS on both sides or the server refuses to start. There is no `Partitioned` attribute (critic-r1 7).
- **A site that is not listed.** The origin check runs before any body or cookie is read. A refused preflight gets 403 with no cross-origin permission headers, and a refused actual request gets `permission_denied` or a bare 403. No session is written. A page on an unlisted site cannot even attach the `Strict` cookie. A sibling subdomain on the same site can attach it, but the origin check refuses it, and a test covers that row. A GET is refused with 405. I found nothing that lets an unlisted page make the browser send a credentialed request that the server acts on. One narrow gap: a request with no `Origin` and `Sec-Fetch-Site: same-site` is treated as a native client (`plan.md:221` refuses only `cross-site`). A browser always sends `Origin` on a cross-origin POST, so I found no exploit, but refusing `same-site` as well costs nothing.
- **Does reading the database allow sign-in?** No. The token is 256 random bits and only its SHA-256 is stored, so the stored verifier cannot be turned into a cookie. The feed reference is a second hash of the verifier. The salt, the binding and the creation time allow offline password guessing, slowed by scrypt at about 64 MiB and 100 ms per guess. A weak password would fall. Someone who learns the password can sign in, as with any password.

### Task question c — the vault client and the signing key

- **First start.** Login, then a read of the key path. A 404 without version metadata permits a create with `options.cas=0`, then a read of the stored value, which is used (`plan.md:399-401`).
- **Two starts at once.** One create succeeds. The other gets the HTTP 400 version conflict and reads the winner. Only a hand-written wire fixture shows the conflict code (`TestLinkSigningKeyCreateRace`). The plan relies on C13 to confirm it, but C13 does not race two starts (see the post-build checks).
- **Vault sealed or unreachable.** Startup fails inside the two-minute deadline with a fixed category, no listener, and no success log. A transport failure on the first read never permits a create.
- **Stored key differs from the one in memory.** The code always returns the persisted seed and never the local candidate, which `TestLinkSigningKeyCreateRace` checks.
- **Names.** The plan builds on the names of `go-s1-stack-vault` as they are now: the two settings, the credential file layout, `/v1/auth/approle/login`, the 15-minute batch token, `lgtm/data/server/link/signing-key`, the field `private_seed_b64`, `cas=0`, and the `link_signing_key_loaded` log event. The role grants read, create and update, and the secret identifier is reusable (`.plans/plans/go-s1-stack-vault/plan.md:261`), so a later restart can log in again. The store names in `go-s1-server-store` also match: `store.Open`, `ConnectionPolicy`, `Store.Migrate`, `store.Migrations`, `Store.Write`, `Store.Read`, `Tx.Record`, `StartFollower`, `RunPruner`, `storetest.NewDatabase` and `migrated.NewStore`. The only name mismatch is with the stack-web plan (critic-r1 1).

## Post-build checks

1. **Origin with a changed port.** Start the stack with `LGTM_WEB_PORT=18787`. Send SignIn with `curl` and `Origin` set to the published address. Expect 200, the signed-in result, and a `Strict` cookie.
2. **The other loopback spelling.** Repeat check 1 with `Origin` set to `localhost` and the same port. Expect the behaviour chosen in the fix for critic-r1 2, and never an unexplained 403.
3. **Unlisted origins.** Send a preflight and a SignIn from an unlisted cross-site origin and from an unlisted sibling subdomain. Expect 403, no permission headers, no cookie, and no new row in `web_sessions`.
4. **Password revocation with restoration.** Sign in, change the password and restart, confirm `unauthenticated`, restore the old password, restart again, and confirm the old cookie is still `unauthenticated`.
5. **Compose names.** Run `docker compose config` and compare the server's environment with the table of item 1. Expect every setting that the table names and none that the server ignores.
6. **Sealed vault.** Stop OpenBao, restart the server, and expect a nonzero exit, a closed port and a log that holds no secret. Start and unseal OpenBao, and expect the server to come back by itself with the same fingerprint.
7. **Two first starts at once.** On an empty real OpenBao, start two server containers together. Expect one stored key and the same fingerprint in both logs. This is the only check that shows the real conflict response.
8. **Key replaced while running.** Replace the key in OpenBao and expect no change until restart, as issue 1639 records.
9. **Third-party cookie blocking.** In a browser that blocks third-party cookies, sign in from a listed cross-site website, and record what the person sees.
10. **Forged row.** Insert a row with a known token hash and a random binding. Expect a valid session until restart and a full purge at the next start.

## Grade

Grade: 6 out of 10. Blockers: 0. Majors: 4. Minors: 7.
