REVISE

Top finding: the runtime session check never verifies the password binding, so a second server process or a row written by hand keeps a session valid that check C4 says must end.

# Round 1 review, lens logic, plan go-s2-server-signin

- Task identifier: review-go-s2-server-signin-r1-logic
- Nonce: 7d2316
- Reviewed text: `.plans/plans/go-s2-server-signin/plan.md` as it stood at the start of the review, with `decisions.md` and `scouting.md` beside it.
- Also read, to check exact names: `.plans/plans/go-s1-server-store/plan.md`, `.plans/plans/go-s1-stack-vault/plan.md`, the foundation design, the epic, and the old engine's session file `packages/engine/src/client/web/session.ts`.
- Not re-opened: one user with one password from a setting, OpenBao in the Compose stack on one machine, a single server node with PostgreSQL and no SQLite, tests against a real PostgreSQL, and the sizes low, medium and high.

## Findings

### logic-r1 1. MAJOR. Tied to C4 (the plan also cites D4 and R4)

The binding is checked only when the server starts, so a row that does not belong to the running password stays valid until the next restart.

- Evidence: `plan.md:121` checks the bindings only inside `Reconcile`. `plan.md:155` makes `Check` and `CheckInRead` look up "only the provided verifier" and its expiry. `plan.md:427` says "the server runs as a single instance" and "deployment stops the old process", which is prose with no mechanism and no test.
- Failure scenario one, two processes. The owner keeps the Compose server running with password P1. The owner starts a second server on the same database, for example a development copy from a worktree, with password P2. The second process purges every session at startup and then serves. The first process is still running. A sign-in on the first process with P1 inserts a row bound to P1. The second process looks that row up by verifier and expiry, finds it and accepts the cookie. A session bound to the former password is now valid on the server that holds the new password. The reverse also holds: a sign-in with P2 gives a row that the first process accepts.
- Failure scenario two, a row written by hand. Anyone who can write to `web_sessions` can insert a row whose verifier is the SHA-256 of a token they chose, with any salt and any binding of the right length. `CheckInRead` accepts it at once. The binding column exists to stop exactly this forgery. The old engine checked it only at load as well (`packages/engine/src/client/web/session.ts:86`), but the old engine had one writer, a file with mode 0600. A shared database has more writers. The next restart then purges the whole set, so the forged row also deletes every real session.
- Fix, two parts. First, hold a session-level advisory lock on a dedicated connection from before `Reconcile` until `Close`, with an identifier outside the reserved spaces 841941100001, 841941100002 and namespace 1279743053. A second process then fails startup with a fixed diagnostic. Second, let `CheckInRead` read the salt, the binding and the creation time, recompute the binding once for each distinct row against the in-memory password, and cache only the success keyed by verifier and binding. That costs one scrypt run for each row for each process and keeps the token out of memory. Add two tests: `TestCheckRejectsForeignBinding` inserts a row bound to another password and requires `ErrNoSession`, and `TestSecondInstanceRefused` starts a second app on the same database and requires a startup failure.

### logic-r1 2. MINOR. Tied to D5 (eight sessions at most), plan item 4

A sign-in whose commit outcome is unknown leaves a live session that nobody holds, and that session can push out a session the owner does hold.

- Evidence: `plan.md:151` says "an ambiguous commit may leave a session that expires or is evicted. There is no automatic second issue." `plan.md:153` evicts by newest first, so the orphan counts as one of the eight.
- Failure scenario: the owner has eight signed-in browsers. A sign-in reply is lost after the commit. The server committed a ninth row, and the oldest real session is evicted. The owner holds no cookie for the new row and signs in again, which evicts a second real session. One lost reply costs two devices.
- Fix: when the store reports `OutcomeUnknown`, `Issue` knows the verifier. It should make one best-effort `Revoke` of that verifier in a fresh write before returning the error, and it should ignore the result of that revoke. Add a case to `TestSessionWriteRollsBack` that terminates the backend during commit and then counts the live rows.

### logic-r1 3. MINOR. Tied to D4 (a session lasts 30 days), plan item 4

One forward step of the database clock stretches the lifetime of every later session.

- Evidence: `plan.md:153` selects `created_at = max(sample, latest + 1 microsecond)` and `plan.md:155` expires on `created_at > clock_timestamp() - 720 hours`. The old engine skipped a record dated in the future (`packages/engine/src/client/web/session.ts:99`); this plan has no such bound.
- Failure scenario: the database host clock jumps forward by a year, one sign-in happens, and the clock is corrected. The new row carries a creation time a year ahead. Every later sign-in takes `latest + 1 microsecond`, so it also carries a time a year ahead. Each of those sessions stays valid for a year and 30 days, long after its cookie has expired in the browser.
- Fix: treat a row whose creation time is more than a small skew, for example one minute, ahead of `clock_timestamp()` as expired in the read predicate. Let `Issue` and `Reconcile` delete such rows. Take `latest + 1 microsecond` only when `latest` is within that skew. Add a test with a fixture row dated ahead.

### logic-r1 4. MINOR. Tied to plan (decisions 7 to 15 are unanswered)

Item 7 builds a refusal and a mutation test on a decision that the log still lists as unanswered.

- Evidence: `decisions.md:14` marks decision 7 as unanswered, and `plan.md:290` and `plan.md:297` build `invalid_argument` and `TestSessionRequestKeys` on it. The store plan says the retry rule for credential calls still needs the owner (`.plans/plans/go-s1-server-store/plan.md:24`). The website sends no request key today, and `src/frontend/src` has no use of `Lgtm-Request-Key`.
- Failure scenario: a later retry layer in the website attaches the key to every mutation, because the store plan makes the key the retry mechanism. Every sign-in then fails with `invalid_argument`, and the owner cannot sign in.
- Fix: close decision 7 with the owner before the build starts. Add one test that sends SignIn through the website's real transport settings, so a future interceptor shows up as a failure and not as a silent lockout.

### logic-r1 5. MINOR. Tied to D4, R3 and R7 (ordering and tie rules), plan item 4

Three ordering rules in the plan have no test, so the cheapest wrong implementation passes every named test.

- Evidence: `plan.md:153` names `latest + 1 microsecond` and the eviction order `created_at DESC, verifier DESC`. The four tests at `plan.md:159` to `plan.md:162` assert only that A comes before B in an ordinary race.
- Failure scenario: a builder uses plain `clock_timestamp()` and `ORDER BY created_at DESC`. Both tests pass. Two sign-ins in the same microsecond, or a clock step backward, then give equal or reversed creation times. Eviction then removes the newer row, or the check at binding time disagrees with the stored time.
- Fix: add `TestSessionIssueTieAndBackwardClock`. Seed a row with a creation time in the future within the skew and another equal to the database sample. Require the new row to sort first and to survive the eviction. Name the mutation "drop the `latest + 1 microsecond` rule" in item 4's risk paragraph.

### logic-r1 6. MINOR. Tied to C2 and C4, plan items 3 and 9

When PostgreSQL is unreachable at startup, a removed or changed password cannot end sessions, and restoring the old password revives them.

- Evidence: `plan.md:427` says the operator must "repeat the removal while PostgreSQL is available before restoring the old password". `decisions.md:16` records this as an assumption. `plan.md:372` lets a malformed database setting block the purge. No test covers the unreachable-database case, only a trigger that aborts the purge (`plan.md:453`).
- Failure scenario: the owner suspects a leaked cookie and sets a new password, but the database container is down. The server exits without purging. The owner gives up and puts the old password back. The suspect session is valid again, and no diagnostic says so.
- Fix: the design cannot record a purge while the database is down, so print a fixed diagnostic that states the sessions were not ended, and add a test that checks that line. Name the operator duty in `deploy/README.md` through the Stack team. This is a documented limit and not a defect that code can remove.

### logic-r1 7. MINOR. Tied to C13, plan item 8

The create-conflict path is classified by one HTTP status code that only a wire fixture confirms.

- Evidence: `plan.md:401` says "classify only that validation response as a conflict" and the code is checked only in a wire fixture. `.plans/plans/go-s1-stack-vault/plan.md:270` checks that a second create conflicts, but not which status or body the real OpenBao returns.
- Failure scenario: the pinned OpenBao answers the losing create with a different status or body than the fixture. The losing process treats a harmless race as a creation failure and exits. A single-instance start never meets this, but a restart during a slow first create can. Compose restarts the process and the second start reads the winner, so the harm is one failed start.
- Fix: classify by outcome and not by code. After any non-success answer to the create, make one follow-up read. If it returns a canonical 32-byte seed, use it. Otherwise fail. Never send a second create. Keep the fixture, and add the status and body to the real-vault check so a drift shows up.

### logic-r1 8. MINOR. Tied to C13, plan item 8

Nothing notices when the stored signing key changes between two starts.

- Evidence: `plan.md:399` reads the latest version of the seed each start. The server's policy grants read, create and update on that one path (`.plans/plans/go-s1-stack-vault/plan.md:261`), and the maintenance operator can widen it (`plan.md:264` of the same file). `plan.md:429` logs the fingerprint only.
- Failure scenario: someone writes a second version of the seed to the path. The next start loads it, logs a new fingerprint, and serves under a new identity. In sprint 3 every joined client then sees a server key it does not know, with no earlier record to compare against.
- Fix: also log the secret's version number from the read response, and let sprint 3 compare the public fingerprint with the one the clients pinned. If the owner wants detection now, keep the public fingerprint in a small `server_identity` row and refuse a mismatch unless an explicit flag allows it. This needs an owner decision, because the design says no key material lives in the application database. The fingerprint is public, so it is outside that rule.

## Answers to the lens questions

1. Sequences, counters and timestamps: the creation time is monotonic by `latest + 1 microsecond` and survives a tie, but a forward clock step inflates later lifetimes (finding 3) and no test covers the tie or the backward step (finding 5). The eight-row counter and the eviction order are enforced under one table lock, so nothing is lost or duplicated. The binding uses the same microsecond value that the database stores.
2. Failures: a failed write returns an empty token and no cookie, a failed purge refuses to listen, and a vault failure exits before the listener opens. Each fails closed. The unknown-outcome sign-in has no repair (finding 2). An unreachable database leaves old sessions untouched, and the plan hands the repair to the operator (finding 6).
3. Truth tables for shared state: see the table of states in question a below. The rules that touch `web_sessions` are issue, revoke, reconcile, expiry and the eight-row cap. All four write rules take the same table lock, so only the readers see an in-between state, and a read-only repeatable-read snapshot cannot see a half-applied change.
4. Zero values: the zero `Verifier` can never come from a decoded token, because the digest of any token is not zero, and the router passes only decoded cookies. The zero `Password` matches no candidate. A zero `Password` given to `NewSessions` would still bind a row to the empty password. The app never does this, so I found no defect, only an easy guard to add in `NewSessions`. An empty request key header is covered, an empty origin list is covered, and a zero-length cookie is treated as absent.
5. Two actors on one row: issue, revoke and reconcile serialize on the `SHARE ROW EXCLUSIVE` lock, and a reader never blocks. Two server processes are the open case (finding 1).
6. Named mutations: each named mutation fails its named test. The skipped-recomputation mutation fails `TestReconcilePassword`. Removing the lock fails the `pg_locks` observation. The request-key mutation fails `TestSessionRequestKeys`. Two mutations are missing (findings 1 and 5).
7. Cheapest wrong implementation that passes all tests: a `CheckInRead` that never reads the binding (finding 1), and a plain `clock_timestamp()` with a plain descending sort (finding 5).
8. A different shape that makes the wrong state impossible: the process-lifetime advisory lock together with a once-per-row binding check at read time (finding 1), and a bounded-skew read predicate (finding 3).

## Answers to the task questions

a. States of sign-in, checks C2 to C4. The table follows. Rows marked "no" are the defects.

| State | Must happen | What the plan does | Matches |
|---|---|---|---|
| Restart, same password, database up | Live sessions survive, expired rows go | `Reconcile` recomputes each binding and keeps the live rows | Yes |
| Password changed, database up | Every session ends before the port opens | The whole set is deleted in one committed write, then the listener opens | Yes |
| Password removed, empty or invalid, database up | Every session ends, the server exits nonzero | Purge with a nil password, then exit | Yes |
| Password changed or removed, database down | Sessions end | Nothing can be written. The server exits, the sessions stay, and restoring the old password revives them | No, a documented limit (finding 6) |
| Restore of a former password after a committed purge | The purged sessions stay gone | The rows are deleted, so none can return | Yes |
| Wrong password at sign-in | No session, no cookie | `REJECTED` in a successful reply and nothing written | Yes |
| Right password, commit fails | No cookie | An empty token and an error | Yes |
| Right password, commit outcome unknown | No cookie, no harm | No cookie, but a live row remains and counts toward the eight | No (finding 2) |
| Tenth, ninth and later sign-ins | The oldest session goes | Eviction by creation time and verifier under the lock | Yes |
| Second process with another password on the same database | Old-password sessions end everywhere | The second process purges once, then accepts rows from the first | No (finding 1) |
| A row inserted or edited by hand | The row is rejected | Accepted until the next restart, then the whole set is purged | No (finding 1) |
| Clock stepped forward then corrected | A session lasts at most 30 days | Later sessions inherit the future time | No (finding 3) |
| Revoke, then check | The session fails | `ErrNoSession` | Yes |
| Session ends during a watch | The stream closes within 30 seconds | The authorizer maps `ErrNoSession` to `ErrWatchUnauthorized` | Yes |

b. Cookie and origin rules. A website on the same site as the server gets `lgtm_session` as host-only, `Path=/`, `HttpOnly`, `Max-Age=2592000`, `SameSite=Strict`, and `Secure` exactly when the public address is `https` (`plan.md:221`, `plan.md:223`). A listed website on another site gets the same attributes with `SameSite=None; Secure`, and the plan refuses to start unless both origins are `https`. The browser may still drop a third-party cookie by its own rule, and then sign-in answers `SIGNED_IN` while the cookie never arrives. That is a browser policy limit that the plan names. A page on a site that is not listed can make the browser send a request to the server, and it carries the `None` cookie if the server uses that mode. The server answers such a request with `permission_denied` before it decodes the body or reads the cookie, because the browser always sends an `Origin` header on a cross-site POST and the origin is not listed. A cross-site request with no `Origin` and `Sec-Fetch-Site: cross-site` is refused, and `Origin: null` is refused. Preflights from unlisted sites get 403 with no permission headers. I found no gap here. The stored verifier is the SHA-256 of a 256-bit random token, so a reader of the database cannot sign in with it, and the feed reference is a second one-way hash. A reader of the database can still guess the password offline against the binding, because the salt, the verifier and the creation time are all stored and only the password is missing. Scrypt with N=65536 makes each guess cost about one tenth of a second. The design sets that cost, so it is not a finding. A writer to the database can sign in with a forged row (finding 1).

c. Vault client and signing key.
- First start: the server reads the path, gets 404 without version metadata, creates with `cas=0`, reads again, and uses the stored seed. This is correct.
- Two starts at once: the loser of the create gets a conflict, reads the winner's seed and uses it. The plan guards the classification only by status code (finding 7). The single-instance rule means this seldom happens.
- Vault sealed or unreachable: login or read fails, startup exits before the listener opens, and no create is sent after a failed read. A lost create answer allows exactly one follow-up read. All of this fails closed.
- Stored key differs from the key in memory: the server never compares them after startup, because it loads the key once. A changed stored key shows only as a new fingerprint at the next start (finding 8).
- Names: the plan builds against the exact names of both dependency plans as they stand today. I checked `Open`, `ConnectionPolicy.AllowPrivateStackPostgres`, `Store.Migrate(ctx, parts ...Migrations)`, `Migrations{Part, Files}`, `migrated.NewStore(t, schema, parts...)`, `storetest.NewDatabase`, `Store.Write`, `Tx.Record`, `Store.Read`, `ReadTx`, `StartFollower`, `RunPruner`, `RequestKeyHeader`, `Source.Authorize` and `ErrWatchUnauthorized` in the store plan. In the stack plan I checked the file layout, the mount at `/run/lgtm/openbao.json`, the two environment names `LGTM_OPENBAO_ADDR` and `LGTM_OPENBAO_CREDENTIAL_FILE`, the path `lgtm/data/server/link/signing-key`, the field `private_seed_b64`, the 15-minute batch token, and the log event `link_signing_key_loaded`. All match. The server policy has no metadata or list path, and the plan reads only the data path, so the two agree.

## Post-build checks

| Scenario | Expected result |
|---|---|
| Start a server with password P1, sign in, start a second process on the same database | The second process exits nonzero with a fixed diagnostic and the first session still works. |
| Insert a row with a random verifier and a wrong binding while the server runs, then send its token as a cookie | `unauthenticated`, and the real sessions stay valid. |
| Sign in with P1, stop the server, start it with P2, then restart it with P1 | The session fails after the P2 start and stays failed after the P1 start. |
| Start with an unset password while PostgreSQL is stopped | Nonzero exit, a diagnostic that says the sessions were not ended, and no listener. |
| Sign in nine times | Exactly the eight newest cookies work, and the first fails. |
| Terminate the database backend during the commit of a sign-in | No cookie is issued, and the live row count does not exceed the number of cookies the owner holds plus zero. |
| Set the database clock a year ahead, sign in, and set it back | The new session expires at most 30 days after the real sign-in. |
| Start with the vault sealed, then unseal it | The first start exits with no listener. The second start logs one `link_signing_key_loaded` event whose fingerprint equals the one from the first successful start. |
| Send a cross-site POST from an unlisted origin with the `None` cookie attached | `permission_denied`, no session lookup and no cookie change. |

## Grade

6 out of 10. The database design, the lock, the ordering and the vault flow are careful, and the plan fails closed on every storage and vault error. One major gap keeps the grade down: the binding is never checked while the server runs. Counts: 0 blockers, 1 major, 7 minors.
