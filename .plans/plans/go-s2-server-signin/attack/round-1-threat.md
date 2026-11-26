# Round 1, threat lens: the plan go-s2-server-signin

Task: review-go-s2-server-signin-r1-threat. Nonce: 248bfc. Reviewer seat: claude-sonnet-5-5, high effort.
Reviewed text: `.plans/plans/go-s2-server-signin/plan.md` as it stands, which equals `attack/plan-v1.md`.

**Verdict: REVISE**

**Top finding:** The password binding is checked only when the server starts, so a session that must end can stay valid when two server processes overlap during a password change (threat-r1 1).

## Findings

### threat-r1 1. MAJOR (a session that must end stays valid). Tied statement: C4

- **Evidence:** `.plans/plans/go-s2-server-signin/plan.md:61` says the server checks a cookie without re-deriving the binding on each request. Line 155 shows that `Check` reads only the row and the 720-hour expiry. Line 427 says "the server runs as a single instance" and that deployment stops the old process first. The only enforcement is that sentence of procedure.
- **Failure scenario:** The owner changes the password from P1 to P2 because P1 leaked. A second process with P2 starts while the first process with P1 still serves, for example a slow shutdown, a manual `docker compose run`, or a second stack on the same database. The new process purges every row at startup. The old process still accepts P1, issues a session, and stores a row bound to P1. The new process's `Check` finds the row by its verifier and accepts it, because it never recomputes the binding. The holder of the leaked P1 is signed in after the change. The check driver's C4 uses `--force-recreate`, which stops the old container first, so no test exercises this overlap.
- **Prescribed fix:** Enforce the single instance in the database. Hold a PostgreSQL session-level advisory lock on a dedicated connection for the whole process lifetime, taken before `Reconcile`. A second process then waits or exits instead of serving. Add a test that starts a second application on the same database and requires it to fail or block before reconciling, and that the first process's sessions stay intact. As a stronger option, also store a cheap per-row tag, computed with a key derived once at startup from the password, and verify it in `Check`.

### threat-r1 2. MAJOR (the stack cannot start the server, so C1 to C4 and C13 cannot pass). Tied statement: `plan` (C2, D10)

- **Evidence:** `.plans/plans/go-s2-server-signin/plan.md:355-368` names `LGTM_SERVER_LISTEN_ADDR`, the five `LGTM_POSTGRES_*` settings and `LGTM_ALLOW_PRIVATE_STACK`. Line 370 says the container health probe is `GET /healthz`. Line 427 reserves only `join-token` as a subcommand. `.plans/plans/go-s2-stack-web/plan.md:259-260` and row 25 of `.plans/plans/go-s2-stack-web/decisions.md` set `LGTM_LISTEN_ADDR`, `LGTM_POSTGRES_URL` with `sslmode=disable`, `LGTM_TLS_CERT_FILE`, no `LGTM_ALLOW_PRIVATE_STACK`, and a health check that runs `lgtm-server health`.
- **Failure scenario:** The Stack plan's environment reaches the server built from this plan. The server reads no `LGTM_POSTGRES_HOST`, uses its loopback default inside the container, and cannot reach PostgreSQL. Even with the host fixed, the missing private-stack opt-in makes the plaintext connection to `postgres` and `openbao` refused (lines 372 and 395). The health command `lgtm-server health` does not exist, so the container never becomes healthy. Both plans mark the names as unanswered (decision 12 here, row 25 there), so the gap is open on both sides.
- **Prescribed fix:** Decide the names once, in this plan's item 1, and answer row 25 of the Stack plan with them. Either adopt the Stack's names, or hand the Stack the exact list above including `LGTM_ALLOW_PRIVATE_STACK=true`. Define the `health` subcommand in item 9, or require the Stack to probe `GET /healthz` without a program in the image. Add one test in `config_test.go` that loads exactly the environment the Stack plan writes and requires success.

### threat-r1 3. MINOR (a forged session is accepted until the next restart). Tied statement: C4

- **Evidence:** `.plans/plans/go-s2-server-signin/plan.md:155` and line 61 (no binding check in `Check`); the binding exists to defeat forged rows (line 92, design section "Sign-in and web sessions").
- **Failure scenario:** A person who can write to the `web_sessions` table, without knowing the password, inserts a row whose verifier is the SHA-256 of a token of their choosing, with any salt and binding. `Check` accepts the cookie at once. The next restart purges the row, but until then the binding protects nothing. Reading the table gives no sign-in, because only one-way hashes of 256-bit random tokens are stored, but writing does.
- **Prescribed fix:** Verify a per-row tag in `Check` as in finding 1, or state in the plan that write access to this table is equivalent to a session and keep the application role the only writer. A test inserts a row with a random binding while the server runs and requires `unauthenticated`.

### threat-r1 4. MINOR (a removed password and a restored one revive old sessions after an uncommitted purge). Tied statement: C4

- **Evidence:** `.plans/plans/go-s2-server-signin/plan.md:36` ("even if the old password is later restored") and line 427 (when PostgreSQL is unreachable, the operator must repeat the removal before restoring the old password). Decision 9 is marked unanswered.
- **Failure scenario:** The owner removes the password, but PostgreSQL is down, so the purge cannot commit and the server exits with the port closed. The owner then restores the old password. Reconciliation finds bindings that match and keeps every old session. The promise on line 36 is broken in exactly this path.
- **Prescribed fix:** Reword line 36 and the done-when row 2 so that the promise reads "after a committed purge". Name the exception as a known limit in the plan and in the Stack's README. No database-free fix exists, so do not add a mechanism for it.

### threat-r1 5. MINOR (a sibling site can lock the owner out). Tied statement: D11

- **Evidence:** `.plans/plans/go-s2-server-signin/plan.md:223` treats a duplicate `lgtm_session` occurrence as absent. The cookie is host-only with no prefix (lines 204 and 223).
- **Failure scenario:** A page on another subdomain of the same registrable domain sets a cookie `lgtm_session` with `Domain` set to the parent. The browser then sends two such cookies on every call. The server treats the cookie as absent, so `GetServer` reports signed out and every protected call returns `unauthenticated`, even after a successful sign-in.
- **Prescribed fix:** Use the `__Host-` prefix, `__Host-lgtm_session`, whenever the cookie is `Secure`, since that prefix forbids a `Domain` attribute and cannot be set from a subdomain. Keep the plain name only for plaintext loopback. Extend `TestCookieAttributes` and `TestReadsOneCanonicalCookie` to cover both names.

### threat-r1 6. MINOR (cross-site mode leaves the origin check as the only defence). Tied statement: D11

- **Evidence:** `.plans/plans/go-s2-server-signin/plan.md:221` (an absent Origin uses the public-origin policy; only `Sec-Fetch-Site: cross-site` without Origin is refused).
- **Failure scenario:** Once any listed cross-site origin exists, the cookie is `SameSite=None`, so the browser sends it from any site. A request with neither an Origin nor a Sec-Fetch-Site header, for example from a browser without fetch metadata, is admitted as same-origin and carries the cookie. Modern browsers always send an Origin on a cross-origin POST, so the exposure is small, but the fallback is the weak point of the model.
- **Prescribed fix:** When the allowlist holds a cross-site entry, refuse a request that carries the session cookie and has neither header. The check driver, which owns its curl calls, sends an Origin header on cookie calls. Add a case to `TestOriginPolicies`.

### threat-r1 7. MINOR (dependencies are named without versions). Tied statement: R9

- **Evidence:** `.plans/plans/go-s2-server-signin/plan.md:81` and `:201` name `golang.org/x/crypto/scrypt` and `golang.org/x/net/publicsuffix` as "the root dependency handoff". `.plans/epics/go-rewrite/epic.md:129-130` requires a plan to list each module and version it needs.
- **Failure scenario:** The orchestrator adds the modules before the first item that imports them. With no version in the plan, the choice falls to whoever runs `go get`, so the password-hashing and origin code build on an unreviewed version. The suffix list is frozen into the module version and goes stale. A stale list fails closed here, because the site class selects only the cookie mode, while admission depends on the exact list of origins.
- **Prescribed fix:** Add a table of every module this plan imports (`x/crypto`, `x/net`, the pgx pool and the Connect modules the other plans name) with an exact version. Add a one-line note that the suffix list is refreshed when `x/net` is bumped.

### threat-r1 8. MINOR (the server can overwrite its signing key, and a replaced key goes unnoticed). Tied statement: D7

- **Evidence:** `.plans/plans/go-s1-stack-vault/plan.md:261` grants the server role read, create and update on the signing-key path. `.plans/plans/go-s2-server-signin/plan.md:399` says the client never overwrites an existing entry, and line 429 logs only the public fingerprint.
- **Failure scenario:** A bug or a compromised server process sends a plain write to the signing-key path. The policy allows it, and every machine that kept the old key refuses the server (issue 1639). On a later restart the server loads whatever is stored and logs a new fingerprint, and nothing compares it with the last one.
- **Prescribed fix:** Ask the Stack to remove `update` from the server policy, since a create-only write with `cas=0` needs `create` only, and add the check to `app_role_access_is_scoped`. Add the stored version and creation time from the KV metadata to the `link_signing_key_loaded` event, so a replaced key shows up in the log.

## Answers to the numbered lens questions

1. **Trust boundaries.** The browser reaches the proxy; the proxy reaches the server over private cleartext HTTP/2 on the `app` network that only those two services join; the server reaches PostgreSQL on `data` and OpenBao on `vault`. Findings 5 and 6 are the gaps at the browser edge; the plan states the origin gate runs before body decoding and before cookie reading (`plan.md:254`).
2. **Inventory of secrets.** The password (environment variable, process memory as text plus a digest), the session token (cookie only; the database holds a SHA-256 verifier, a salt and a binding), the scrypt-derived binding key (cleared after use, `plan.md:92`), the AppRole `role_id` and `secret_id` (file, mode 0400, owner 10001), the 15-minute batch token (memory), the Ed25519 seed (memory; only the public fingerprint is logged), and the PostgreSQL password (file). None is in a response body, a log line or a receipt, per items 7 and 8 and the redaction tests.
3. **How each credential ends.** A session ends by sign-out, eviction at the ninth sign-in, 30-day expiry, or a committed purge on a password change or removal. Findings 1, 3 and 4 are the paths where it does not end. There is no sign-out-everywhere call; the designed way to end a stolen cookie is a password change. The batch token lapses after 15 minutes with a fresh login. The AppRole identifier is replaced by the Stack's maintenance command. A Watch stream is re-checked by the Store every 30 seconds and on each message, so revocation ends it within that bound (`.plans/plans/go-s1-server-store/plan.md:611`).
4. **Encryption in transit and at rest (issue 1633).** Encryption ends at the proxy. The password and the cookie cross one private cleartext hop inside the `app` network; the signing seed and the AppRole credential cross cleartext hops on `vault`. The Stack plan makes these networks internal, drops all capabilities, and publishes no port (`.plans/plans/go-s1-stack-vault/plan.md:150`, `.plans/plans/go-s2-stack-web/plan.md:259-263`). I found no contradiction with the owner's decisions. At rest, the database holds hashes only, and the seed lives in OpenBao.
5. **Dependencies and images pinned.** Not pinned in this plan: finding 7. The images are the Stack plans' concern, and their digests are pinned there.
6. **Integrity against untrusted parties.** An unlisted origin is refused before the body or the cookie is read, and receives no cross-origin permission. A database reader cannot sign in. A database writer can until restart (finding 3). Sibling-site cookies can cause a lockout (finding 5).
7. **Post-build checks.** Listed below.

## Answers to the task questions

**(a) Table of states.**

| State | What happens | Matches C2 to C4? |
|---|---|---|
| Wrong password at SignIn | Result rejected, no cookie, no row. | Yes (C3). |
| No, malformed, duplicate, expired or revoked cookie | `unauthenticated`, fixed sentence. | Yes (C3). |
| Password setting missing, empty, invalid or over 4096 bytes, PostgreSQL reachable | All rows purged and committed, exit nonzero, no listener, message names the setting. | Yes (C2). |
| Same, PostgreSQL unreachable, then the old password restored | No purge, no listener; old rows revive on restoration. | No, finding 4. |
| Restart with the same password | Rows whose bindings match are kept; expired rows are removed. | Yes (C4). |
| Password changed, one process | All rows purged before the port opens; old cookie gets `unauthenticated`. | Yes (C4). |
| Password changed, old process still serving | Old process issues rows bound to the old password; the new process accepts them. | No, finding 1. |
| Old password restored after a committed purge | Purged rows stay gone. | Yes. |
| Row inserted by a database writer | Accepted until the next restart. | No, finding 3. |
| Sign-out | Only that row is revoked and the cookie cleared after commit; other sessions survive. | Yes. |
| Ninth sign-in | Oldest row evicted under the table lock. | Yes. |
| 720 hours on the database clock | Row refused. | Yes. |

**(b) Cookie, origin and verifier.**

- **Same-site website:** `lgtm_session`, `Path=/`, `HttpOnly`, `Max-Age=2592000`, `SameSite=Strict`, host-only, and `Secure` when the configured public address is `https`.
- **Listed cross-site website:** the same attributes with `SameSite=None` and `Secure`. Startup refuses a cross-site entry unless both origins are `https`.
- **Page on a site that is not listed:** its non-simple request needs a preflight, which gets 403 without permission, so the browser sends nothing. A simple cross-site POST carries an Origin and is refused before the body or the cookie is read (`plan.md:254`). A Strict cookie is not sent cross-site at all. The remaining gaps are findings 5 and 6.
- **Stored verifier:** a reader of the database cannot sign in, because the row holds only a SHA-256 of a 256-bit random token, and the feed key is a separate domain-separated hash.

**(c) Vault client and signing key.**

- **First start:** the entry is absent (404 with no version metadata), so the client creates it with `options.cas=0`, then reads it again and uses the stored seed.
- **Two starts at once:** one create wins; the other gets the conflict response, reads, and returns the winner's seed (`TestLinkSigningKeyCreateRace`).
- **Sealed or unreachable vault:** startup fails with a fixed message and no listener. A failed initial read never authorizes a create.
- **Stored key differs from memory:** the server reads the key once and never again. A replaced entry is noticed only on restart, and only if someone compares fingerprints (finding 8).
- **Exact names:** the plan matches `go-s1-server-store` (`Store.Migrate`, `Migrations`, `Tx.Record`, `ChangeRef`, `RequestKeyHeader`, `StartFollower`, `RunPruner`, `storetest.NewDatabase`, `migrated.NewStore`, `ErrWatchUnauthorized`) and `go-s1-stack-vault` (the two files, the mounts, the path `lgtm/data/server/link/signing-key`, the field `private_seed_b64`, the log event `link_signing_key_loaded`). It does not match `go-s2-stack-web` (finding 2).

## Post-build checks

1. Start a second server on the same database with a changed password while the first runs. Expected: the second waits or exits, and the first process's sessions are intact; after the first stops and the second serves, the old cookie gets `unauthenticated`.
2. With the server running, insert a `web_sessions` row with a random binding. Expected: a cookie for it gets `unauthenticated`.
3. Stop PostgreSQL, start the server with no password, observe the closed port, restore PostgreSQL and the old password. Record whether old sessions revive, and confirm the documentation says so.
4. Run `deploy/test/checks.sh --only C2,C3,C4,C13` on the stack as the Stack plan writes it, with no extra environment. Expected: four passing lines.
5. In a real browser, set a duplicate `lgtm_session` cookie from a sibling subdomain. Expected: sign-in still works (finding 5).
6. Preflight and POST from an unlisted origin in a real browser. Expected: no cookie sent, 403, no dispatch. Repeat from a listed cross-site HTTPS origin and inspect `SameSite=None` and `Secure`.
7. Seal and unseal OpenBao during server start. Expected: the server exits or waits with no listener and no seed in any log; then it loads the same fingerprint.
8. Search the server and proxy logs and every error body for the password, a token, the seed and the AppRole fields. Expected: none.
9. Inspect the running stack. Expected: port 8080 is not published, the server joins only `app`, `data` and `vault`, `openbao.json` has mode 0400 and owner 10001, and no container has raw-network capability.
10. Compare the signing-key metadata version before and after the recreate in C13. Expected: unchanged.

## Grade

**6 of 10.** Counts: 0 blockers, 2 majors, 6 minors.
The plan's core is sound: hashed tokens, constant-time compare, serialized issue, an origin gate before decoding, and a create-only vault write with a read-back. The two majors are the unenforced single-instance rule and the setting names that do not meet the Stack plan.
