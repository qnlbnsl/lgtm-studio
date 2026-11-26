# Round 1 review of go-s1-server-store, seat threat

- Task identifier: review-go-s1-server-store-r1-threat
- Nonce: de16ab
- Lens: security threat model
- Plan as reviewed: `.plans/plans/go-s1-server-store/plan.md`, 579 lines, the same text as `attack/plan-v1.md`
- Model and effort: claude-sonnet-5-5, high
- Tested commit: `fb992dc9` (no Go code exists yet: `src/server/` and the root `go.mod` are absent, so every claim below is checked against the plan and the design, not against code)

## Verdict

REVISE

## Top finding

An open Watch stream outlives the session that opened it, because `Serve` authorizes once and has no way to learn that the session was signed out, evicted or expired (threat-r1 1).

## Findings

### threat-r1 1. A Watch stream keeps sending records after its session ends

- Severity: MAJOR
- Tied to: R5
- Evidence: `plan.md:464` ("The service authorizes the requested topic and scope before calling `Serve`"), `plan.md:437-462` (`Source` and `Serve` have no revocation input), `plan.md:418-422` (a stream ends only by lag or by the caller's context). The design lists sessions that end by eviction ("A ninth sign-in removes the oldest") and by a 30-day lifetime at `docs/specs/2026-11-25-go-rewrite-foundation-design.md:203`, and ends them on a password change only "before the server opens its port" at line 207.
- Failure scenario: The owner suspects a stolen session cookie and signs that session out, or a ninth sign-in evicts it. The holder of the stolen cookie already has a `WatchClients` stream open. The stream's context belongs to the transport, not to the session row. A quiet stream never lags, because it acknowledges filtered rows as it skips them. It therefore stays open until the server restarts, and each new matching feed row makes `source.Load` send the current record to the person who was just signed out.
- Fix: Add a required field to `Source`, `Authorize func(context.Context, *store.ReadTx) error`, and make `Serve` call it before each send of a saved record and at least once for every 30 seconds of silence. When it fails, `Serve` returns a new sentinel `store.ErrWatchUnauthorized`, and the later adapter maps that to `unauthenticated`. Add a test to `TestWatchFiltersAndLoadsCurrentRecords` where the check starts failing between two feed rows and the stream ends without sending the second record. Add a line to the plan's rollout detail that the sign-in plan implements `Authorize` by reading the session row.

### threat-r1 2. A receipt can hold a credential in plaintext, and a password-bearing body leaves an offline-guessable digest

- Severity: MAJOR
- Tied to: R9
- Evidence: `plan.md:163-168` (`reply bytea` with no size cap and no sensitivity marker), `plan.md:293` ("It passes only receipt-safe replies and never writes raw credentials into this table" and "The later service owns the policy"), `plan.md:293` again (the digest is a plain SHA-256 over principal, method and body), `plan.md:507` (the traceability row claims D9 and D10 are met by "Keep credentials out of receipts"). The design stores "a verifier of each token" and no secret at lines 254-255, says "Secrets never reach a log, the board or the website" at line 369, and says a copy of the database is useless without the key file at line 227.
- Failure scenario: The Machines page creates a join token with a request key, so a network retry cannot make two tokens. The reply carries the raw token, and `WriteOnce` stores it in `receipts.reply` for 24 hours, readable by anyone with a copy of the database. That breaks the verifier-only rule and the claim at design line 227. In a second case, `SignIn` carries a request key. Its body holds the password, and `request_hash` is an unsalted SHA-256 of that body. Anyone with a database copy can guess the password offline against that digest for 24 hours. The only control is a sentence that asks the adapter to behave. The store takes any `[]byte`, so the first credential-returning call of the sign-in or machines plan will use it as it stands.
- Fix: Add `Sensitive bool` to `Request`. For a sensitive request the store keeps no reply bytes (it stores a zero-length reply and a flag column `reply_withheld boolean NOT NULL DEFAULT false`), and it computes `request_hash` as HMAC-SHA-256 under a key that the server reads from the vault at startup and passes to `Open`, never a bare digest. A retry of a sensitive request that matches returns a new sentinel `store.ErrReplyWithheld`, and the adapter maps it to a sentence that tells the owner to reload and check, never to a second write. Add one cap, `octet_length(reply) <= 65536`, to the `receipts` table. Add a test that a sensitive receipt row contains neither the reply nor an unkeyed digest. Change the traceability row for D9 and D10 to name this mechanism.

### threat-r1 3. One stalled feed writer stops every writer and, through the shared pool, every reader

- Severity: MAJOR
- Tied to: R4
- Evidence: `plan.md:248-259` (the global lock is held from the flush to the commit, across three round trips and the commit's disk flush), `plan.md:103` (`Read` and `withWrite` use one pool; no pool size is set), `plan.md:385` and `plan.md:418` (the follower's listener is one dedicated connection and its reads use `Read`). A search of the plan for lock timeouts, statement timeouts, idle-in-transaction timeouts, keepalives and pool limits finds none; the only deadlines are in tests (`plan.md:387`, `plan.md:392`).
- Failure scenario: A disk stall or a half-open network connection freezes the writer that holds the feed lock. Every other writer that has feed changes blocks in `pg_advisory_xact_lock` with no timeout, each holding one pool connection. With the default pool of `max(4, CPU count)` connections, the waiting writers and the follower's listener use them all within a few requests. Sign-in checks, the follower's poll, and every `Read` then wait for a connection. The whole server is unresponsive until PostgreSQL ends the stuck session, and nothing in the plan ends it. The plan's own question "what does a slow writer block" has the answer "everything", and the plan does not say so.
- Fix: In `Open`, set on the copied configuration `RuntimeParams` the values `idle_in_transaction_session_timeout=30s` and `statement_timeout=15s` as defaults, set `lock_timeout=10s` with `SET LOCAL` immediately before the feed lock statement so a queue on that lock fails with a retryable error, enable TCP keepalives in the dialer, and refuse a pool whose `MaxConns` is below 8. Reserve connections: give `Read` and the follower their own small pool, or cap concurrent writers with a semaphore of `MaxConns - 3`. Add `TestWritersCannotStarveReaders`: hold the feed lock in one transaction, start `MaxConns` writers, and assert that `Read` still returns within one second and that the queued writers fail with the lock-timeout error.

### threat-r1 4. Nothing bounds how many changes one write may record

- Severity: MINOR
- Tied to: R5
- Evidence: `plan.md:244-246` (`Record` buffers without a limit), `plan.md:248-257` (one `INSERT` statement per buffered change, run while the lock is held), `plan.md:418-420` (a stream is cut at 2,048 rows after its cursor).
- Failure scenario: A later part rewrites 3,000 records in one transaction, for example when it renames a project or marks a machine's records as removed. The flush runs 3,000 single-row inserts under the global lock, so every other feed writer waits for the whole run. After the commit, 3,000 rows follow the cursor of every open stream, which is more than 2,048, so the follower cuts every stream at once. Every browser then reopens and loads a full snapshot at the same moment.
- Fix: Make `Tx.Record` return `ErrTooManyChanges` after 256 buffered changes in one write, state in the plan that a larger rewrite is split into several writes, and send the lock statement, the inserts and the notification in one `pgx.Batch` so they take one round trip. Keep one `INSERT` per change inside the batch, because a multi-row `INSERT ... SELECT` does not promise the order in which the identity values are drawn. Add a test that the 257th `Record` fails and that a 256-change write is a single round trip.

### threat-r1 5. The ordering guarantee of the feed rests on parts not misusing `Tx.Exec`

- Severity: MINOR
- Tied to: R4
- Evidence: `plan.md:98-100` (`Tx.Exec`, `Query` and `QueryRow` run any SQL), `plan.md:246` ("A part executes no SQL on the store's tables"), design line 148 ("A part never inserts into `feed` itself"), and `plan.md:103` ("Store code alone commits or rolls back").
- Failure scenario: A builder of a later part, working side by side with others as the design intends, writes `INSERT INTO feed ...` directly, or runs `COMMIT` through `Tx.Exec` before its last statement. The lock is then never taken, or a later `pg_advisory_xact_lock` in autocommit mode is released at the end of its own statement. Positions again follow allocation order, not commit order, and a cursor passes a late commit. No test of the part fails, because the race appears only under concurrent commits. The one guard is the store's own test, which exercises only the store's path.
- Fix: Add a `BEFORE INSERT` trigger on `feed` in `0001_store.sql` that raises unless `current_setting('lgtm.feed_flush', true) = 'on'`. `flushFeed` sets that flag with `set_config('lgtm.feed_flush', 'on', true)` straight after it takes the lock, so it holds only inside that transaction and only after the lock. Add `TestPartCannotInsertIntoFeed`, which inserts through `Tx.Exec` and expects the trigger's error.

### threat-r1 6. A request key is global, so one principal's key blocks every other principal

- Severity: MINOR
- Tied to: D3
- Evidence: `plan.md:164` (`request_key text PRIMARY KEY`), `plan.md:295` (the per-key lock hashes the key alone), decision 13 in `decisions.md` ("one global case-sensitive key"), `plan.md:300` (a different caller gives `ErrRequestKeyConflict`).
- Failure scenario: A machine that is connected but not approved, or any later caller on the link, sends a call with a key that a web session also uses. The second sender gets `already_exists`, which tells it that the key exists, and the first sender's later request with a different body is refused. Keys are random in practice, so this is mostly a nuisance, but the namespace is shared by every present and future caller, including the link, which is the party the design trusts least.
- Fix: Make the primary key `(principal_hash bytea, request_key text)` where `principal_hash` is the SHA-256 of the caller string, and include the principal hash as the second integer of the advisory lock. A key then conflicts only within one principal, and a different principal never learns that a key exists. Change decision 13 to say so.

### threat-r1 7. A digest over re-serialized bytes can turn a legitimate retry into a false conflict after an upgrade

- Severity: MINOR
- Tied to: D3
- Evidence: `plan.md:293` ("For a current Protocol Buffers caller use deterministic serialization of the same parsed request"). The Go Protocol Buffers library documents deterministic output as stable only within one build, not across library versions or generated code.
- Failure scenario: The server restarts into a new build while the website is retrying a request. The restart is the usual reason for a retry. The new build marshals the same parsed request to different bytes, the digest differs, and `WriteOnce` returns `ErrRequestKeyConflict` for a request that did succeed. The website shows an error, the owner repeats the action with a fresh key, and the change happens twice. The design asks for the opposite at its "built as if replicated" rule 4, and a second node on another build would behave the same way.
- Fix: Hash the request bytes exactly as the adapter received them on the wire, and say so in the `Request.Body` comment. State that the adapter must not re-marshal. Add a test that two requests with the same wire bytes and two different re-marshaled forms produce the same digest.

### threat-r1 8. Errors from the database carry row values, and the plan relies on callers to sanitize them

- Severity: MINOR
- Tied to: R9
- Evidence: `plan.md:103` ("Return callback or commit errors to the caller"), `plan.md:295` ("No key, digest or reply is included in diagnostics"), `plan.md:385` and `plan.md:319` (diagnostics are sanitized only where the reader and the pruner report them).
- Failure scenario: A `CHECK` violation on `receipts`, or a unique violation in a part's table, returns a PostgreSQL error whose `Detail` text reads "Failing row contains (...)" or "Key (...)=(...) already exists", with the values of the row, including `reply` bytes or a token column. The part or the adapter logs the error with `%v`. The plan sanitizes only its own diagnostics, so the first caller that logs a store error writes the values to a log.
- Fix: Have `withWrite`, `Read` and the migration runner wrap every `*pgconn.PgError` in `store.Error{Code, Constraint}` that carries the SQLSTATE code, the constraint name and a fixed sentence, and drops `Detail`, `Where` and `Hint` from `Error()`. Keep the original only behind `Unwrap` for tests. Add a test that a failing `receipts` insert produces an error string that contains none of the inserted bytes.

### threat-r1 9. The plan says nothing about encryption of the database connection

- Severity: MINOR
- Tied to: D4
- Evidence: `plan.md:91` (`Open` takes any `pgxpool.Config`), `plan.md:105` (only the test configuration is described), `plan.md:385` (the follower uses "one configured database endpoint"), design lines 209 and 318-320 (the website and the link accept only HTTPS or loopback).
- Failure scenario: The design builds the layer "as if the database were replicated", and the endpoint is a setting. An operator points the server at a PostgreSQL on another host. The pgx default is `sslmode=prefer`, which falls back to plain text without any error. Sessions' verifiers, receipts and the feed then cross the network unencrypted. The same rule that the website and the link follow is missing from this hop.
- Fix: In `Open`, refuse a configuration whose host is not a loopback address, a Unix socket, or the compose stack's service name unless its `sslmode` is `verify-full`. Add `TestOpenRefusesPlainRemote` with a table of hosts. Write the compose-internal exception into the plan so the Stack team knows it.

### threat-r1 10. Row locks taken at commit can deadlock against the feed lock

- Severity: MINOR
- Tied to: R4
- Evidence: `plan.md:248-259` (the feed lock is taken after the callback and the commit follows at once), `plan.md:295` (the key lock, then `FOR UPDATE` on the receipt), `plan.md:246` (parts may run any SQL, including deferred constraints).
- Failure scenario: Writer A holds a row lock on a parent row from its callback and waits for the feed lock. Writer B holds the feed lock, and its commit fires a deferred foreign key check that needs a key-share lock on the same parent row. Each waits for the other. PostgreSQL's deadlock detector aborts one of them after one second. Both are correct, but the caller gets a deadlock error and the one-second stall is held under the global lock. Nothing in the plan says that parts must not use deferred constraints, or that a deadlock error is retried.
- Fix: Add a rule to the plan, in the "Parts can store state" workstream: a part's tables may not use `DEFERRABLE INITIALLY DEFERRED` foreign keys or constraint triggers, and `withWrite` retries once on SQLSTATE `40P01` before it returns the error. The test-only deferred trigger of `TestFeedPositionsFollowCommitOrder` stays allowed because it touches only `feed`.

## Answers to the attack questions

Lens file:

1. Trust boundaries. Crossing in: part SQL (trusted, in-repository), request keys, request bodies and reply bytes from the adapter (checked only for length and UTF-8 at `plan.md:293`), topic and scope strings from the website (authorized once at open, see finding 1), the test database connection string, and the embedded migration files (compiled in, trusted). The checks are partial: no check ties a stream to a live session (finding 1), and no check keeps credentials out of receipts (finding 2).
2. Inventory. The `LGTM_TEST_POSTGRES` connection string holds a password and lives in an environment variable; pgx connection errors do not print it. The production connection configuration is passed to `Open` by a later plan, and its password source is not named here (the Stack plan owns it). Request keys are unguessable-by-convention and must stay out of logs (`plan.md:295`). Feed rows hold non-secret references only, but the store cannot check that (`plan.md:246`). Receipt replies and digests can hold credentials or password-derived values (finding 2). No cookie, token verifier or credential file is created by this plan.
3. How credentials end. This plan creates none. A receipt ends by the hourly sweep after 24 hours, and there is no way to purge one earlier (finding 2). A Watch stream is the one long-lived grant here, and it has no end tied to sign-out (finding 1).
4. Encryption. At rest, receipts are plaintext (finding 2). In transit to PostgreSQL, the plan states nothing (finding 9). Issue 1633 concerns the link and the web server, not this plan; nothing found here.
5. Dependencies and images. One new dependency, `github.com/jackc/pgx/v5`, named without a version at `plan.md:84`; the root `go.mod` belongs to the Contract team. The design's check used 5.13.0. The plan should say "at least the version the design checked, with `govulncheck` in the sprint's verify step" (a post-build check below). No image is added; the tests need a PostgreSQL 16 or newer that the test setup supplies. Trust added: the PostgreSQL driver and the Go standard library.
6. Integrity. A party that is not trusted can: keep a stream open after revocation (finding 1), occupy request keys of other callers (finding 6), cause a false conflict only if it can alter the bytes of a retry, which it cannot. A process of another user on the same machine can LISTEN on the empty channel, which gives it only a wake-up. A machine before approval cannot reach the store, provided the adapter authenticates first, which `plan.md:293` states.
7. Post-build checks: listed below.

Task file questions:

a. Feed order and the lock. Commit order is kept: the lock is `pg_advisory_xact_lock`, taken after the callback and after the receipt insert, released at commit, and PostgreSQL ends a transaction in the process array before it releases its locks, so a snapshot cannot see a later position without an earlier one (`plan.md:248-259`). A snapshot therefore sees a prefix of committed positions, on the node or on a physical standby. Rolled-back positions are gaps, which the reader ignores. Two caveats: the guarantee holds only for rows inserted by the store's own flush (finding 5), and a write with no pending changes never takes the lock, which is correct. A slow writer between the flush and the commit blocks every other writer that has feed changes, and through the shared pool it blocks readers too (finding 3). It does not block writers that record no change or a reader that already holds a connection.
b. The follower. A lost notification costs at most two seconds, because the poll reads `position > cursor` in order and the cursor advances only to rows actually read (`plan.md:383`). A long transaction cannot hide a lower position, because a position is drawn only under the lock, so no higher position can exist until the holder commits or aborts. A gap changes nothing: the reader skips it and the lag count counts actual rows (`plan.md:420`). A restart starts a new follower at the current head, and every old stream is gone with the process, so a repeat or a miss cannot cross a restart. A wake that arrives during a drain is kept by the one-slot channel. The cut-off is stated exactly enough: the stream is cancelled when 2,048 actual committed rows, counted across all topics and including the in-flight row, follow its acknowledged cursor, with the boundary `>=`, tested at 2,047 and 2,048 (`plan.md:420`, `plan.md:426`). One consequence is not addressed: a single write with more than 2,047 changes cuts every stream (finding 4).
c. Retries. Same key and a different body, caller or procedure: `ErrRequestKeyConflict`, no callback, no reply shown (`plan.md:295`, `plan.md:300`); across callers it is also an existence oracle (finding 6). Two identical requests at once: the second waits on the per-key advisory lock, then finds the committed receipt and returns the stored reply, so one callback runs (`plan.md:299`). A crash between the write and the receipt: impossible by design, because both are in one transaction; a crash before the commit leaves nothing and the retry runs once, and a crash after the commit but before the reply makes the retry return the stored reply. Pruning: a receipt older than 24 hours is deleted, so a retry after that runs the write again; this is stated at `plan.md:295` ("during receipt retention") and matches the design. A retry that meets the sweep at the boundary waits for the sweep's row lock, finds the row gone, and runs the write again, which is acceptable at the boundary. Two other findings bear on this question: 2 (what a receipt may hold) and 7 (a false conflict after an upgrade).

## Post-build checks

1. Gate trigger scope. In `TestFeedPositionsFollowCommitOrder`, confirm that the test-only deferred trigger blocks only writer A (for example by comparing the backend identifier it captured) and not writer B. Expected: without the production lock, B commits and a fresh observer query sees B alone. If the trigger also gates B, the mutation cannot show B visible first and the test passes vacuously.
2. Lock key decomposition. During the commit stall of the same test, read `pg_locks` for the advisory lock and confirm the feed lock appears as `classid = 0`, `objid = 841941100001`, `objsubid = 1`, and the receipt lock as `objsubid = 2`. Expected: the two spaces never meet, and `hashtext` exists on the PostgreSQL major version the compose stack pins.
3. Pool starvation. With `MaxConns = 4`, hold the feed lock in one transaction, start four writers, and call `Read`. Expected after the fix of finding 3: `Read` returns within one second and the writers fail with the lock-timeout error.
4. Large write. Record 3,000 changes in one `Write` against a live follower with three open streams. Expected after finding 4: `ErrTooManyChanges` at the 257th record, and no stream cut.
5. Notification order. In a loop of 1,000 writes, wait for the notification and then read the feed with a new snapshot. Expected: the row is visible every time, since PostgreSQL signals listeners after the commit is visible.
6. Error detail. Force a `CHECK` violation on `receipts` and a unique violation on a fixture table, and print the returned error with `%v`. Expected after finding 8: no inserted value appears.
7. Sensitive receipt. Store a sensitive receipt and dump the table. Expected after finding 2: no reply bytes, and a digest that does not equal an unkeyed SHA-256 of the body.
8. Revocation. Open a stream, delete its session row, commit a matching change. Expected after finding 1: the stream ends with the unauthorized sentinel and sends nothing.
9. Dependencies. Run `govulncheck ./...` and record the pgx version. Expected: no known vulnerability at the pinned version, and the version is at least the one the design checked.
10. Deadlock. Run two writers that lock the same parent row, one waiting for the feed lock and the other holding it, with a deferred foreign key. Expected after finding 10: the part's schema review rejects the constraint, or the single retry succeeds.

## Grade

6 out of 10. The core ordering design is sound and the tests for it are well chosen. The gaps are at the edges where the store meets sessions, credentials and failure of its own lock.

Counts: 0 blockers, 3 majors, 7 minors.
