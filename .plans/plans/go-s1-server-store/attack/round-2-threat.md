REVISE

# Round 2 threat review of the server store plan

Task identifier: review-go-s1-server-store-r2-threat. Nonce: 039c15. Lens: security threat model. Identifier prefix of findings: threat-r2.

## Top finding

Major, threat-r2 1. The sanitized `*Error` is defined only for errors that carry a PostgreSQL error code (SQLSTATE), so driver-side errors without one, such as parameter encoding failures and connection failures, can still carry parameter values or network addresses to the log.

## Findings

### threat-r2 1. Errors without a SQLSTATE are not covered by the sanitizer

- Severity: major.
- Tied statement: R9, "Secrets never reach a log, the board or the website."
- Evidence: `plan.md:119` defines `*Error` as "only the SQL state code (SQLSTATE), the declared constraint identifier ... and a fixed public sentence". It replaces "PostgreSQL errors". It says that "context cancellation and domain errors keep their identity". `plan.md:130` shows that `TestDatabaseErrorsHideValues` causes only a check violation and a unique conflict. Both of those carry a SQLSTATE.
- Failure scenario: A callback inserts a row with a parameter that the driver cannot encode, for example a string with a zero byte for a `text` column. In the pinned pgx v5 line, an encoding failure is raised on the client and formats the argument into its message, and it has no SQLSTATE. The sanitizer's input is "PostgreSQL errors", so this error is not replaced. It reaches the adapter and the log with the value in its text. A password hash or a request key is such a value. A refused or reset connection has the same shape. It has no SQLSTATE, and its text holds the host and port of the database. The test only proves the two server-side cases, so the build passes while the leak stays open. I mark the pgx wording as something to verify by the test below, not as established.
- A second path is a callback that wraps a driver error with `%w` and adds a domain sentence. The rule "domain errors keep their identity" leaves two readings. Replace it, and the domain identity is lost. Keep it, and the raw message inside the chain is kept as well.
- Fix: Make the mapping total. Any error whose chain contains a type from the driver or the network layer becomes `*Error` with an empty SQLSTATE, a class (`connection`, `encode`, `other`) and a fixed sentence. A domain error is recognized only by a store-declared marker, for example a `store.Domain(err)` wrapper or a registered sentinel. An unmarked error from a callback that contains a driver type in its chain is replaced. Add three cases to `TestDatabaseErrorsHideValues`: an unencodable parameter, a connection closed by the test mid-transaction, and a callback that wraps a SQLSTATE error with `%w`. Each case must show no inserted value and no network address in `%v`, `%+v` and a chain walk.

### threat-r2 2. The sanitized error drops the commit outcome that the retry rule needs

- Severity: major.
- Tied statement: R4 (every write goes through the store's transaction) and R8 (an error is a Connect code with one sentence).
- Evidence: `plan.md:119` says "An ambiguous keyed commit is retried with the same key. Adapters must avoid restarting an ambiguous unkeyed mutation." The same paragraph removes raw causes and keeps only SQLSTATE, constraint and sentence. `Retryable()` is the only classification offered.
- Failure scenario: The connection drops after the client sends `COMMIT` and before the reply arrives. The error has no SQLSTATE, and the raw cause is gone. The adapter cannot tell this case from a rolled-back deadlock. It cannot follow the plan's own rule for an unkeyed mutation, such as issuing a join token. If it treats the error as retryable, the token is issued twice. If it treats it as final, a committed change is reported as failed. The sanitizing patch removed the only signal that told these cases apart.
- Fix: Add a store-computed `Outcome()` to `*Error` with the values `rolled_back` and `unknown`. The store sets `unknown` only when the failure happens after it sent the commit. Test it with a connection killed on another backend during commit. `Retryable()` stays true only for `rolled_back` errors with the three listed SQLSTATE values. Name the adapter as the reader of `Outcome()` in item 1's list of writers and readers.

### threat-r2 3. The store cannot enforce the credential policy, and threat-r1 2 is still open

- Severity: major.
- Tied statement: R9 and D3. The design prohibits credentials in server tables.
- Evidence: `plan.md:361` to `plan.md:371` shows `WriteOnce` accepting any `[]byte` reply. `plan.md:374` says the policy for credential-bearing calls is "awaiting a receipt policy decision". `decisions.md:34` (row 27) is `unanswered`. The sign-in plan already assumes that `SignIn` refuses a request key, at `go-s2-server-signin/decisions.md` row 7, and that row is also `unanswered`.
- Failure scenario: A later adapter author wires a keyed `WriteOnce` around a call that returns a new session token or a join token. The store stores the reply bytes in `receipts.reply` with no complaint. The request digest is an unkeyed SHA-256. If the fingerprint includes a password, anyone with a database dump can guess it offline. The only guard is a sentence in the plan that a reviewer must remember.
- Fix: Two steps, neither of which decides the owner's question. First, the store fails closed. Add `Request.ReceiptSafe bool` and make `WriteOnce` return `ErrInvalidRequest` unless the adapter sets it. Each adapter then has to say the word, which a reviewer can grep. Second, ask the owner to take option 1 of the register now for sprint 2 and 3, because the sign-in plan already assumes it. Options 2 and 3 can come later without changing the store. Add a post-build grep check that no keyed call in the sign-in, session or join-token packages passes `ReceiptSafe`.

### threat-r2 4. The Watch authorization result has no complete table, and one sentinel has no name

- Severity: minor.
- Tied statement: R5 (every Watch call is served through the `watch` folder) and D5.
- Evidence: `plan.md:557` declares `Authorize`. `plan.md:567` declares only `ErrWatchUnauthorized`. `plan.md:589` says "An unavailable authorization check terminates with a retryable unavailable error." `plan.md:593` maps `ErrFeedUnavailable` to the sentence "The change feed is unavailable, try again."
- Truth table of what `Authorize` can do, with the plan's answer:

| Result of `Authorize` | Plan says |
|---|---|
| nil | continue |
| `ErrWatchUnauthorized` | end, map to `unauthenticated` |
| database error or two-second timeout | end, "retryable unavailable error", unnamed |
| any other error | not stated |
| panic | not stated |

- Failure scenario: An authorizer returns an error of its own, such as "session table missing". The helper's behavior is unspecified. An implementation that continues on unknown errors fails open on a revoked session. The unnamed "unavailable" error has no sentinel, so the Machines adapter has nothing to map. It may use `ErrFeedUnavailable`, which tells the browser that the feed is down when the session store is the problem.
- Fix: State that any non-nil result of `Authorize` ends the stream. Recover a panic and treat it the same way. Name `watch.ErrAuthorizationUnavailable`, list the sprint 3 adapter as its reader, and give it its own sentence. Add the "any other error" case and a panic case to `TestWatchRechecksAuthorization`.

### threat-r2 5. The connection policy does not say how loopback and the stack host are recognized

- Severity: minor.
- Tied statement: D4 (the database stays inside the stack and connections are protected).
- Evidence: `plan.md:115` allows plaintext for "Local Unix sockets and loopback connections" and for "the exact private Compose service `postgres`". It validates "every configured endpoint including fallbacks before dialing". `plan.md:131` shows that `TestOpenRefusesPlainRemote` has five cases and no name that resolves to loopback.
- Failure scenario: The check classifies by host name, and the driver resolves the name later, at dial time. A name that resolves to loopback during the check and elsewhere during the dial, or an entry in a hosts file, sends plaintext traffic to a non-loopback address. The same gap applies to the exception for `postgres`. Another container on the Compose network can claim that name as an alias. The Stack plan keeps the network internal and publishes no port, which limits the exposure to one machine, so I rate this minor.
- Fix: Classify only by literal address (`127.0.0.0/8`, `::1`) and by a Unix socket path. For the exact service name, install a `DialFunc` that checks the resolved address at dial time against a private range. Add two cases to the test: a name that resolves to a public address, and `sslmode=prefer` with a plaintext fallback to a loopback entry followed by a remote one.

### threat-r2 6. A receipt that the sweeper never observes lives forever, and a forward clock jump has no check

- Severity: minor.
- Tied statement: R4 and D5.
- Evidence: `plan.md:199` makes `retention_started_at` nullable, and `plan.md:222` says "Item 6 alone fills the retention clock". `plan.md:402` says "Unobserved rows cannot be deleted." `plan.md:404` reports a stall only for feed rows. `plan.md:402` also puts a forward clock jump "in the deployment checks", but the post-build table at `plan.md:707` to `plan.md:727` has no such row.
- Failure scenario: If `RunPruner` is not started, or it fails every hour, every reply stays in `receipts.reply` for as long as the stall lasts, and nothing reports it. This lengthens the time that stored reply bytes sit at rest. In the opposite case, an administrator or a time service steps the database clock forward by a day. The next sweep sees all observed receipts as expired, deletes them, and a retry of an already committed mutation runs a second time. The plan documents this and then leaves it without a check.
- Fix: Make the sweep result report the oldest unobserved receipt by `written_at`, so a stall appears for receipts as it does for the feed. Add a post-build row that steps the database clock forward by 25 hours between a keyed write and its retry and records the expected behavior. State in item 6 whether the sweeper may delete more than one day of receipts in a single sweep after a jump.

### threat-r2 7. The feed guard covers inserts only

- Severity: minor.
- Tied statement: R4.
- Evidence: `plan.md:209` to `plan.md:221` defines `BEFORE INSERT FOR EACH ROW`. `plan.md:316` sets the flag with `set_config(..., true)`. `plan.md:222` says the guard "catches accidental direct writes by repository parts".
- Failure scenario: A part's cleanup code runs `DELETE FROM feed WHERE key = $1` or `TRUNCATE feed` by mistake. The guard does not fire. The deleted position was already committed. A follower reading past the gap never sees the change, and a watcher misses an update. The retention row's `pruned_through` records only the pruner's deletions, so the follower cannot tell the difference.
- Fix: Add a statement-level trigger that refuses `TRUNCATE`, and a row trigger that refuses `UPDATE` and `DELETE` unless a second transaction-local flag, set by the sweeper only, is on. Add one direct-delete case to the direct-insert test.

### threat-r2 8. The test fixture does not check that the database is disposable

- Severity: minor.
- Tied statement: R10 (tests run against a real PostgreSQL) and the plan's own fixture rule.
- Evidence: `plan.md:121` reads `LGTM_TEST_POSTGRES`, requires the creation privilege, and runs `CREATE DATABASE` and `DROP DATABASE ... WITH (FORCE)`. It says only "a disposable PostgreSQL maintenance database". `plan.md:339` runs `check-feed-lock.sh` against the same variable.
- Failure scenario: A developer exports the variable from a shell that also holds the production login, which has a creation privilege in some setups. The test run then creates, fills and force-drops databases on the production server, and holds advisory locks there. Only random names are dropped, so data loss is unlikely, but a force drop on a live server disconnects other sessions of the dropped database only. The larger exposure is a test fixture that writes to the production instance.
- Fix: The Stack wrapper creates the maintenance database with a fixed sentinel name. The fixture refuses any other name before it creates anything and says so in a fixed sentence. Add the refusal to `pool_test.go`.

### threat-r2 9. Two decision rows still state the rules that the patches replaced

- Severity: minor.
- Tied statement: plan.
- Evidence: `decisions.md:20` (row 13) still says "one global case-sensitive key". `decisions.md:23` (row 16) still maps lag to `resource_exhausted`. `plan.md:593` and `decisions.md:32` (row 25) say `aborted`. Row 23 (`decisions.md:30`) says it replaces row 13, but row 13 keeps its status.
- Failure scenario: A builder reads the rows as the record of what is assumed and implements the global key. That is the failure of threat-r1 6, where one principal's key could collide with another's. Row 16 sends a lagging watcher a status that the contract draft does not name.
- Fix: Mark rows 13 and 16 as superseded by rows 23 and 25, with the same note that row 5 got in round 1. Keep their original text and date.

## Answers to the seven task questions

1. Patches against their findings.
   - threat-r1 1 (Watch authorization): the per-message and 30-second recheck removes the stale-session stream. The failure table in threat-r2 4 is what remains.
   - threat-r1 2 (credentials in receipts): not fixed by design, left to the owner. The store accepts anything until threat-r2 3 is done.
   - threat-r1 3 (writer exhaustion): `MaxConns - 3` admission with a 10-second lock wait removes the failure as written. No patch breaks another statement.
   - threat-r1 4 (batch size): the cap of 256 and the ordered batch remove the unbounded loop.
   - threat-r1 5 (feed insert guard): the trigger removes the accidental direct insert. It does not cover deletes (threat-r2 7).
   - threat-r1 6 (receipt namespace): the principal hash in the key and lock removes cross-principal reuse. Decision row 13 is stale (threat-r2 9).
   - threat-r1 7 (fingerprint stability): the semantic fingerprints and versions remove it.
   - threat-r1 8 (error text): partly. See threat-r2 1 and 2.
   - threat-r1 9 (plaintext connection): the policy removes it except for the classification gap in threat-r2 5.
   - threat-r1 10 (deadlock under the lock): `SET CONSTRAINTS ALL IMMEDIATE` before the lock removes the cycle I checked. I found no new lock acquired after it in the production flow.
2. Each `UNREVIEWED MECHANISM`, attacked with the lens questions.
   - Connection policy: threat-r2 5.
   - Sanitized error: threat-r2 1 and 2.
   - Flush batch: the transaction-local flag resets at commit or rollback, so it does not leak through the pool. The guard is accidental-only, as the plan says, and covers only inserts (threat-r2 7).
   - Receipt framing: the length framing is unambiguous, and the principal hash is a fixed 64 hex characters, so the `:` join in the lock key cannot be confused. A 32-bit lock hash collision only delays a request. Nothing found beyond threat-r2 3.
   - Retention first observation: threat-r2 6.
   - Fixture cleanup by instance teardown: threat-r2 8.
3. No finding has the outcome "rejected" in the register. Threat-r1 2 is "for the owner", which is a decision and not a rejection. Nothing to check.
4. Names and their lists of writers and readers.
   - `ErrWatchUnauthorized`: written by Watch, read by the sprint 3 adapter. Complete.
   - `Source.Authorize`: written by Watch and bound by sprint 3. Sprint 2 supplies the session read. Complete, but the "unavailable" error has no name (threat-r2 4).
   - `*Error` and `Retryable()`: written by Store, read by the sweep, follower and later services. Incomplete, because `Outcome()` is missing (threat-r2 2).
   - `ConnectionPolicy` and `AllowPrivateStackPostgres`: written by Store, supplied by the sprint 2 bootstrap, bound by Stack. Complete.
   - `Request`, `ErrReceiptReplyTooLarge`, `ErrRequestKeyConflict`: written by Store, read by sprint 2 and 3 adapters. Complete. A `ReceiptSafe` field would add the sign-in team as a reader (threat-r2 3).
   - Lock constants in `locks.go`: written by Store. No other part reads them. Complete.
5. Contradictions.
   - Decision rows 13 and 16 contradict the plan (threat-r2 9).
   - The retention text says deployment checks cover a forward clock jump, and the post-build table has no such row (threat-r2 6).
   - Truth table for the keyed retry after a failure: rolled-back error with the same key retries safely. Commit-failure error with a key retries and replays the receipt if it committed, which is safe. The same failure without a key is the case that the plan cannot decide today (threat-r2 2).
   - No two patches contradict each other otherwise.
6. No blocker is open. Of the round-1 majors, threat-r1 1 and 3 are fixed as written. Threat-r1 2 is open by decision. Architect-r1 3 and critic-r1 1, which touch the same mechanism, are fixed as written.
7. The lens questions the patches touch, answered for the patched text.
   - Trust boundaries: the connection policy (threat-r2 5) and the Watch authorization (threat-r2 4).
   - Secret inventory: receipts and errors (threat-r2 1 and 3). The password file path and the test connection string are not printed. Nothing else found.
   - Credential end of life: a revoked session ends a stream within about 32 seconds (30-second recheck plus the two-second bound). Bytes already sent are not recalled, and the plan says so. The window is a stated choice, so I do not raise it.
   - Encryption in transit and at rest: threat-r2 5 for transit and threat-r2 6 for reply bytes at rest.
   - Dependencies: the driver pin belongs to the Contract team. The post-build table has a `govulncheck` row. Re-check the pinned version against the scan at the time it is pinned. Nothing else found.
   - Integrity: threat-r2 2, 6 and 7.
   - Post-build checks: the additions are below.

## Post-build checks to add

1. Cause an encoding failure, a closed connection and a wrapped SQLSTATE error in the built Store, then format each error and inspect the log. Expect no inserted value and no network address.
2. Kill the connection during commit on a keyed and an unkeyed write. Expect `Outcome()` to be `unknown` and a keyed retry to replay or to run once.
3. Grep the sign-in, session and join-token packages for keyed `WriteOnce` calls. Expect none unless the owner has chosen a replay option.
4. Return an arbitrary error and a panic from `Authorize` during an idle stream and during a blocked send. Expect the stream to end both times.
5. Configure a host name that resolves to a public address, with and without `sslmode=prefer`. Expect `Open` to refuse.
6. Stop `RunPruner` for 25 hours of test time. Expect the sweep result to report the oldest unobserved receipt. Then step the database clock forward by 25 hours between a keyed write and its retry, and record the result.
7. Run `DELETE FROM feed` and `TRUNCATE feed` from a test callback. Expect refusal.
8. Point `LGTM_TEST_POSTGRES` at a database with a name other than the wrapper's sentinel. Expect the fixture to refuse before it creates anything.

## Grade

Grade: 7 out of 10. Counts: 0 blockers, 3 majors, 6 minors.
