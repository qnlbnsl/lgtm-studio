# Round 2 review, lens logic, plan go-s2-server-signin

Task identifier: review-go-s2-server-signin-r2-logic. Nonce: 2ab61f.

## Verdict

REVISE

## Top finding

The server instance lock is a non-blocking try-lock that the plan treats as an acquisition with a ten-second budget, and `InstanceLock.Close` has no explicit unlock, so a fast restart can be refused by the previous process's still-open backend session (logic-r2 1).

## Findings

### logic-r2 1: MAJOR. The instance lock has no retry and no explicit unlock.

- Tied to: plan.
- Evidence: `.plans/plans/go-s2-server-signin/plan.md:474` says "Under a separate ten-second acquisition budget take the server instance lock". `plan.md:480` says the Store dials a dedicated session and calls `pg_try_advisory_lock`, and that `Close` "closes it". `src/server/internal/store/locks.go` declares `serverInstanceLockID` and no unlock helper.
- Failure scenario: `pg_try_advisory_lock` returns at once, so the ten-second budget bounds only the dial. When a process is restarted by the Compose restart policy, or the `servertest` harness stops and starts the server, the old backend can still exist for a short time after the client closes the socket. The new process asks for the lock, receives false and prints "Another server is already using this database." It exits with status 1. The stack then restarts it, which hides the fault, but a test that restarts the server and expects success fails at random.
- Fix: state that `Close` runs `pg_advisory_unlock` on the same connection before closing it, and ignores the unlock error only when the connection is already gone. State that a denied lock is retried every 250 milliseconds until the acquisition budget ends, and that the fixed diagnostic prints only after the budget. Add a test that closes a lock and acquires it again at once on a second connection, and a test that holds the lock for three seconds and releases it inside the budget.

### logic-r2 2: MAJOR. One slow probe kills the process.

- Tied to: plan.
- Evidence: `plan.md:480` says "If the session is lost or its probe fails, close serving immediately and fail the process", with a probe every second and a one-second deadline.
- Failure scenario: a garbage collection pause, a busy host or a slow PostgreSQL makes one probe take 1.2 seconds. The context deadline expires, the probe "fails", and the server exits and drops every in-flight request. A timeout does not show that the session is lost. A lost session is a closed connection or a server error.
- Fix: treat a closed connection or a PostgreSQL error as loss at once. Treat a deadline expiry as loss only after three consecutive expiries, or after the deadline has passed for five seconds in total. Add a test with a probe that blocks once for 1.5 seconds and assert that the process keeps serving.

### logic-r2 3: MAJOR. The loopback alias rule is undefined when the public origin is not loopback.

- Tied to: plan.
- Evidence: `plan.md:244` says "the bundled plaintext aliases `127.0.0.1`, `localhost` and `[::1]` on the configured loopback port" and "The aliases count as same-site for this policy". `plan.md:40` repeats "the same port".
- Failure scenario: the public origin is an HTTPS name behind the proxy, and the listener binds a private address in Compose. There is no "configured loopback port". An implementer picks the listener port, or any port. Then a page on `http://localhost:<listener port>` is admitted next to the HTTPS public origin, and the cookie mode for it is undefined. The truth table below has empty cells.
- Truth table (public origin kind × request origin × result):

| Public origin | Request origin | Plan says |
|---|---|---|
| Plaintext loopback, port P | Alias at port P | Admit, mode `lgtm_session`. |
| Plaintext loopback, port P | Alias at another port | Reject unless listed. |
| HTTPS non-loopback | Alias at the listener port | Empty cell. |
| HTTPS non-loopback | Alias at any other port | Empty cell. |
| HTTPS loopback | Plaintext alias at the same port | Empty cell: same site, but a plaintext page cannot receive the `__Host-` cookie. |

- Fix: admit the aliases only when the configured public origin is itself a plaintext loopback origin, and only at its port. In every other case treat an alias like any unlisted origin. Add the three empty rows to `TestOriginPolicies`.

### logic-r2 4: MAJOR. The tests do not prove the time sample comes from PostgreSQL.

- Tied to: plan.
- Evidence: `plan.md:169` says to sample `clock_timestamp()` once under the table lock, and "Fail the write if that increment would exceed the one-minute bound". `plan.md:171` repeats the cache design but names no clock.
- Failure scenario: an implementation calls Go `time.Now()` and passes it as a parameter. Every test that fixes expiry through database timestamps still passes, because the rows are written with explicit times. Two machines with a skewed clock then disagree with the database. The increment overflow case also has no named outcome: a row exactly one minute ahead is live, so the next Issue needs a time past the bound and fails. The plan does not say which error the browser receives, so the implementer may return `ErrNoSession`, which is wrong for a correct password.
- Fix: add a test that shadows `clock_timestamp()` in the test schema (a function in a schema placed first in `search_path`) and asserts that Issue, Check and Revoke use its value. State that the overflow returns the retryable store error, and that the sign-in message is the generic failure sentence, never the "Sign in to continue." sentence.

### logic-r2 5: MINOR. The "not confirmed" sentence is undefined when no purge was attempted.

- Tied to: plan.
- Evidence: `plan.md:496`: "A password purge failed or could not be confirmed."
- Failure scenario: the password is unusable and the database settings are also malformed. No purge is attempted. The table shows two applicable sentences and the order is not stated. A tester cannot say whether "Session revocation was not confirmed." must print.
- Fix: state that every exit with an unusable password prints "Session revocation was not confirmed." unless a purge committed, and that it prints after the other applicable sentence.

### logic-r2 6: MINOR. The plan registers a constant that already exists.

- Tied to: plan.
- Evidence: `plan.md:480` says item 9 "registers `serverInstanceLockID int64 = 841941100003`". `src/server/internal/store/locks.go` already declares it.
- Failure scenario: the builder of item 9 adds a second declaration and the Store package fails to compile. The list of readers is also incomplete: the Store test for the lock ids must now assert three distinct values.
- Fix: say "reuse the existing constant" and name `store/locks.go` and `store/instance.go` as the files item 9 touches. Add the lock-id uniqueness test to the item 9 list.

### logic-r2 7: MINOR. `Revoke` derives scrypt under the table lock on a cache miss.

- Tied to: plan.
- Evidence: `plan.md:171` says "Verify a changed tuple through the item 2 derivation" and `plan.md:131` takes the table lock first.
- Failure scenario: after a restart the cache is empty. Eight sign-out calls arrive together. Each holds the `SHARE ROW EXCLUSIVE` lock while it derives scrypt, so they run one after another and each holds a pool connection. Check calls keep working, but writers queue behind the lock for several seconds.
- Fix: in `Revoke`, read the row and verify the binding in a `Store.Read` first, then delete in the write by verifier without a new derivation. Otherwise state the cost as accepted.

## Answers to the task questions

1. Patch effectiveness: the Check design, the time window, the reconcile purge and the startup order each remove the failure they were written for. Two patches create new defects: the instance lock (logic-r2 1 and 2) and the alias rule (logic-r2 3).
2. UNREVIEWED MECHANISM: the instance lock is attacked in logic-r2 1 and 2. The origin aliases are attacked in logic-r2 3. The password-bound cache has no new defect, because PostgreSQL stays authoritative on every call. Its cold start is logic-r2 7.
3. "Rejected" findings: I checked that the module pins exist (x/net v0.63.0 requires x/crypto v0.59.0, so decision 24 is true) and that the frontend source has no request-key header, so refusing a request key on SignIn breaks nothing. Nothing found against the rejections.
4. Writers and readers: the lock id list is wrong (logic-r2 6). The `ErrNoSession` writer and reader list is complete.
5. Contradictions: the alias rule against the cookie modes has empty cells (logic-r2 3). The pin, OpenBao and `allowInitialize` table is consistent. The time window rules in reconcile, issue, check and revoke use one predicate and are consistent.
6. Round 1 blockers and majors: none reopened. The owner-deferred items (logic2-r1 2, 3 and 6) stay deferred.
7. Lens questions touched: Q1, Q2, Q3, Q5, Q6, Q7 and Q8 (below).

## Answers to the lens questions

1. Ordering and counters: the creation time is the sample or the latest row plus one microsecond, with ties broken by verifier. Gap and duplicate are handled. The overflow outcome is unstated (logic-r2 4).
2. Failure policy: a database failure in Check denies. A lost lock fails the process, which over-denies (logic-r2 2). Repair after a failed purge is a restart, which reruns the purge.
3. Truth tables: built in logic-r2 3 and in the answer to task question 5.
4. Zero values: an empty password and a nil password both mean purge. A zero `created_at` is excluded by the window. Nothing found.
5. Concurrent actors: two servers on one database are prevented by the lock (logic-r2 1 weakens it on restart). Two sign-outs on one row are safe because the table lock serializes them.
6. Risky-item mutations: moving cookie issue before commit fails `TestSessionContract`. Accepting a request key fails `TestSessionRequestKeys`. A wrong change that the tests miss: using Go `time.Now()` for the sample (logic-r2 4).
7. Cheapest wrong implementation: the Go clock in logic-r2 4, and a lock whose `Close` only closes the socket in logic-r2 1.
8. A different shape: one `InstanceLock` type that owns acquire, probe and unlock removes the three loose rules in logic-r2 1 and 2. Nothing else found.

## Post-build checks

- Cold-cache stampede: restart the server and sign out eight sessions at once. Expect all calls to finish inside five seconds and Check calls to stay under 200 milliseconds.
- Instance lock restart race: kill the server with signal 9 and start it again at once, 50 times. Expect no "already using this database" exit after the retry fix.
- Compressed body size: send a gzip body that expands past 32 KiB to SignIn. Expect a refusal and no read past the limit. The connect-go behaviour for compressed bodies is unverified here.
- Alias admission under an HTTPS public origin: send an `Origin` of `http://localhost:<listener port>`. Expect a refusal.

## Grade

6 out of 10. Counts: 0 blockers, 4 majors, 3 minors.
