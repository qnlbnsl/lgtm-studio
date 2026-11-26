# Round 3 review of go-s2-server-signin, lens logic

Task: review-go-s2-server-signin-r3-logic. Nonce: 73be98. Reviewer seat: logic. Finding prefix: logic-r3.

**Verdict: REVISE**

**Top finding:** logic-r3 1. Item 7 requires `TestSessionContract` to prove that the creation-clock refusal maps to `aborted`, but the only fixture that can produce that refusal is private to the `auth` package tests, so the mapping cannot be tested where the plan says it is.

Counts: 0 blockers, 2 majors, 5 minors. Grade: 7 of 10.

## Findings

### logic-r3 1 — MAJOR — The creation-clock refusal cannot be produced by the test that must prove its mapping

- Tied statement: `plan` (items 4, 6 and 7, decision 30).
- Evidence: `plan.md:179` makes `ErrSessionClock` appear only when the latest surviving row is within one microsecond of the one-minute bound. `plan.md:185` puts the clock fixture in the `auth` package's `sessions_test.go`. `plan.md:338` asks `TestSessionContract`, in the `api` package, to "assert the creation-clock refusal returns the specified `aborted` sentence". The signature block of item 4 (the Go block of item 4) does not declare `ErrSessionClock`, and `plan.md:179` says only that auth "writes the sentinel".
- Failure scenario: with the real database clock, a row can never be placed within one microsecond of a bound that moves with every sample. The `api` test therefore either cannot arrange the case or skips it. A builder who maps `ErrSessionClock` to `unavailable`, or who wraps it with `fmt.Errorf("%w")` inside the write callback, passes every listed test. `sanitizeError` in `src/server/internal/store/pool.go` passes only a directly returned `*domainError` unchanged. A wrapped sentinel is turned into the generic sanitized error, and the owner sees "Web session storage is unavailable" instead of "retry sign-in". The mapping in the table of item 6 is then dead text.
- Prescribed fix: (a) Declare `var ErrSessionClock` next to `ErrNoSession` in the item 4 signature block, and state that auth returns both sentinels directly from the `Store.Write` callback, never wrapped. (b) Put the shadow-clock helper in a non-test internal package, for example `src/server/internal/auth/authtest` with `ShadowClock(t, schema) (dsnOption, setTime func(time.Time))`, and name it in item 4's file list. `api/session_test.go` imports it. (c) Name the exact fixture rows that produce the refusal: set fixture time to T, seed one row at T plus one minute minus zero microseconds, and require `aborted` plus the unchanged table.

### logic-r3 2 — MAJOR — The cache mutex contradicts its own test and decision 31

- Tied statement: R7 (decision 31, items 4 and 7).
- Evidence: `plan.md:181` says "Under one cache mutex, look up the complete tuple and serialize missed derivations", so a hit takes the mutex and a miss derives while holding it. `plan.md:185` says the constructor sets one `deriveKey` "for issuance and cache misses". `plan.md:194` requires a sign-in to finish while a revoke's cold derivation is held at the hook. `plan.md:193` expects exactly one derivation for the ninth, newest token after its issue. `decisions.md:38` (row 31) says "a single missed derivation at a time per instance".
- Failure scenario, three readings of the same text:
  1. Issue derives under the same mutex, as decision 31 reads. `TestRevokeVerifiesBeforeLock` then deadlocks, because the held revoke derivation owns the mutex while the sign-in waits for it.
  2. Issue derives outside the mutex, and seeds the cache afterwards. The seed needs the mutex, so the sign-in still blocks, and `TestSessionBindingCache` counts zero derivations for the newest token instead of one.
  3. Issue derives outside the mutex and does not seed the cache. Both tests pass, but decision 31's "one at a time" is false: up to five writers derive while a cache miss derives. That is acceptable for memory (about 64 MiB each), but the plan does not say so.
- A second consequence of holding the mutex through a miss: every authenticated request across all clients, including hits, waits for one cold derivation. `Check` runs inside `Store.Read`, a RepeatableRead transaction on a pooled connection (pool of 8, `idle_in_transaction_session_timeout=30s`). After a restart, eight cold sessions each pay one derivation of about 0.1 to 0.3 seconds, in sequence, while up to eight reads hold every pool connection and starve the feed follower and writers for that time.
- A third gap: "confirm the cache remains bounded" has no observation. An unbounded map, or first-in first-out eviction, passes `TestSessionBindingCache` as written.
- Prescribed fix: state reading 3 explicitly. The mutex guards only the cache map and the miss derivation. Issue derives outside it, never seeds the cache, and is bounded by the writer cap. Reword decision 31 to "one missed derivation at a time for verification". Release the mutex for hits (take it only to look up and to insert, with a per-tuple in-flight marker so identical misses share one derivation). Add a test that reads the cache length and the eviction order through a same-package accessor: warm eight entries, touch the first, insert a ninth, and require that the second entry was evicted.

### logic-r3 3 — MINOR — The order of the shadow-clock setup decides which schema holds the tables and which clock the Store defaults bind to

- Tied statement: `plan` (decision 30, item 4 test setup).
- Evidence: `plan.md:185` puts the shadow schema "explicitly before `pg_catalog` in the test Store configuration". `storetest.NewDatabase` (`src/server/internal/store/storetest/postgres.go:23,76`) sets `search_path` through `RuntimeParams`. `src/server/internal/store/migrations/0001_store.sql:7,24` use `DEFAULT clock_timestamp()`.
- Failure scenario: an unqualified `CREATE TABLE` lands in the first existing schema of the path. If migrations run with the shadow schema first in the path, `web_sessions` and the Store tables go into the shadow schema, and `Store.Migrate` and later tests disagree about where they are. If the shadow function exists at migration time, PostgreSQL binds the `written_at` defaults to the shadow function, because it resolves a default's function when the table is created. Feed and receipt rows then carry fixture timestamps, and any age-based retention in `store` sees them as ancient or future.
- Prescribed fix: in the item 4 test setup, migrate first with the normal path, then create the shadow schema and function, then open a second Store whose path is the shadow schema, `pg_catalog`, then the test schema. Assert at setup that `SELECT clock_timestamp()` on that Store returns the fixture time, so a misplaced path fails loudly.

### logic-r3 4 — MINOR — The rejection of threat-r2 6 rests on a claim that the plan itself contradicts

- Tied statement: D7 (item 8, `LoadLinkIdentity`).
- Evidence: `round-2-findings.md:27` rejects the finding because "the first-start path cannot distinguish restore from initialization". `plan.md:452` says a missing pin allows "initial creation or adoption of the canonical seed already stored". The server therefore knows which of the two it did. The cited line `attack/plan-v2.md:441` does say restoration of the database to an earlier state is outside wave 1, and I confirmed that text. It does not address the crash gap. `docs/issues.md` and `decisions.md` hold no entry for the tracking the register promises ("asks to track recovery separately").
- Failure scenario: the server creates the seed in OpenBao, crashes before the pin commits, and restarts. The pin is missing and the vault holds a seed, so it adopts and pins that seed, which is correct, and the log shows the same `link_signing_key_loaded` line as a first start. The same line appears when an older database copy meets a different vault key. The event `source`, `fingerprint` and `vault_version` cannot separate the three cases, and the proposed field `pin` with `existing`, `created` or `adopted` costs a few lines and does separate created from adopted.
- Prescribed fix: add the `pin` field and check it in PB4, or write the rejection again on the true basis (cost against value in wave 1) and file the tracking item in `docs/issues.md` with the next number. As written, the rejection cites evidence that does not answer the finding.

### logic-r3 5 — MINOR — Instance lock admission leaves the failure policy open on three cells

- Tied statement: C4 (item 9, `plan.md:482-493`).
- Evidence: `plan.md:491` says "An acquisition deadline while waiting returns the fixed busy diagnostic". It does not say what a cancellation, a driver error or a statement timeout returns, and it does not say that the dedicated connection is closed on failure. `plan.md:486` says "the independent migration context" without saying whose child it is, while `plan.md:484` says the monitor "cancels startup and serving upon loss". `configureConnection` in `src/server/internal/store/pool.go` sets `statement_timeout=15s` on every connection, including one opened "from Store's validated pool configuration".
- Failure scenario: (a) SIGTERM during the ten-second wait returns the busy diagnostic, so the owner is told that another server is running when none is. (b) A cancelled `pg_advisory_lock` leaves a connection that may still be granted the lock after the client gave up, and nothing closes it. (c) A migration context built from `context.Background()` is independent of the lifetime context, so a lost lock does not cancel a running five-minute migration, and a second instance can acquire the lock and begin migrating in parallel. (d) An admission budget set above 15 seconds is cut by `statement_timeout` and returns SQLSTATE 57014, which is not a context deadline.
- Prescribed fix: state that busy is returned only when the admission context's own deadline expired and the parent context is still live; a cancelled parent returns that cancellation. State that `AcquireServerInstance` closes its connection on every error return. Build the migration and prerequisite contexts as children of the lifetime context. State that the admission budget must stay below `statement_timeout`, or `SET statement_timeout=0` on that connection for the lock query.

### logic-r3 6 — MINOR — The monitor's stall budget has no test

- Tied statement: R10 (item 9, `plan.md:493`, `plan.md:544`).
- Evidence: `TestServerInstanceLock` delays one `SELECT 1` by 1.5 seconds and requires success. `TestStartRefusesSecondInstance` covers loss of the connection. Neither covers a probe that never returns. "A query tracer delays only the first `SELECT 1`" is also ambiguous, because the Store pool's own ping issues `SELECT 1` on its own connections.
- Failure scenario: the cheapest wrong implementation runs the probe with no timeout, or with a 60-second one. Both pass the tests, and a stalled connection that keeps the lock but never answers lets the server go on serving after it has lost the guarantee. A reading of "first" that hits the pool's ping delays the wrong connection and makes the test pass for the wrong reason.
- Prescribed fix: key the tracer to the lock connection by its application name, and add a case that delays a probe for longer than the private probe budget (a field of `phaseBudgets`), requiring the app to cancel and close the listener within the budget plus a margin.

### logic-r3 7 — MINOR — The module handoff has an empty cell and stale decision rows

- Tied statement: R9 (`plan.md:62-77`, decisions 24 and 32).
- Evidence: the table lists `golang.org/x/net` as "`v0.62.0` if item 2 lands first, then `v0.63.0`". It has no cell for item 5 landing first, and the plan lets items 2 and 5 run in parallel. `decisions.md:31` (row 24) still says "The Contract team pairs both new pins with the first importer", and row 32 (`decisions.md:39`) supersedes it, but row 24 carries no marker. Row 27 (`decisions.md:34`) says it narrows rows 18 and 20, which also carry no marker.
- Failure scenario: item 5 lands first and brings `golang.org/x/net v0.63.0`, whose own requirements select `golang.org/x/crypto v0.59.0` as an indirect entry. Item 2 then only changes an indirect line to a direct one, and the builder who follows the table looks for a crypto entry to add. Two worktrees that both add the same pins produce conflicting `go.sum` lines at merge. A later reader of rows 18, 20 and 24 finds the old rule as the live one.
- Prescribed fix: add the item-5-first row to the table (crypto `v0.59.0` becomes indirect, then direct). State that the second of the two merges takes the existing pin and runs `go mod tidy -diff`. Mark rows 11, 18, 20 and 24 "superseded by 27" or "superseded by 32" in their status column.

## Answers to the questions of the task file

1. **Does each patch remove its failure without breaking another?**
   - architect-r2 1 and threat-r2 1 (vault policy): yes. `deploy/openbao-server-policy.hcl` grants read, create and update, and `deploy/test/vault-init.sh:72,90` requires exactly that. Decision 29 and item 8 agree. That file and the `deploy/` changes were untracked or modified at review time, so the evidence holds only in the working tree.
   - architect-r2 2 and logic-r2 4 (clock): partly. The fixture exists, but see logic-r3 1 and logic-r3 3.
   - architect-r2 3, logic-r2 1, logic-r2 2 and logic-r2 6 (instance lock): the bounded wait and the monitor remove the original failures. See logic-r3 5 and logic-r3 6 for new gaps.
   - architect-r2 4, logic-r2 3 and threat-r2 2 (plaintext alias): yes. The item 5 test names HTTPS public origins on loopback and elsewhere. I did not find a cell where an HTTPS origin admits a plaintext alias.
   - architect-r2 5 and threat-r2 4 (module graph): partly, logic-r3 7.
   - architect-r2 6, logic-r2 7 and threat-r2 5 (cache and revoke): the revoke ordering removes the interval. The cache text contradicts its test, logic-r3 2.
   - logic-r2 5 (diagnostics order): yes. Details, then exactly one outcome, then the password line. See the truth table below.
   - threat-r2 3 and threat-r2 7: yes, no new conflict found.
2. **Each `UNREVIEWED MECHANISM`.**
   - Clock fixture and `ErrSessionClock`: logic-r3 1 and logic-r3 3.
   - Cache and mutex, and revoke outside the lock: logic-r3 2.
   - Revoke compares the full verified tuple under the lock, with no new derivation there. I found no way for a changed row to be deleted. A row replaced by one with the same verifier and a different salt is refused.
   - Instance lock and monitor: logic-r3 5 and logic-r3 6.
   - Module graph: logic-r3 7.
   - Vault read and create, and the pin: the create-only path is sound for the one-user, one-vault envelope. A losing create is settled by the final read. A pin written after a successful create has the crash gap described in logic-r3 4.
3. **Rejected findings.** threat-r2 6: the cited line is real and says what the register claims. The register's reason does not answer the finding, so it is logic-r3 4.
4. **Writers and readers of changed names.** `ErrSessionClock` is written by auth and read by item 7, but is missing from the signature block (logic-r3 1). `phaseBudgets` has one writer, item 9, and its tests are same-package. `InstanceLock.Check` has two callers, the monitor and the pre-listen call, and they share a connection that the plan says is serialized. The two revocation sentences have one writer and one reader (Stack Web request S13). They are the same strings in `plan.md:509-510` and in the Stack Web decision rows 38 and 43. `Sessions.deriveKey` has two writers (the constructor and tests) and two readers (miss and Issue), which is the root of logic-r3 2.
5. **Contradictions and truth tables.**
   - Cache by Issue by Revoke by mutex: three readings, logic-r3 2.
   - Password usability by database state by lock by reconcile outcome, from `plan.md:503-515`. In each row the order is detail line, reconciliation line if it applies, one outcome line, password line.

     | Password | Database | Lock | Purge | Lines on standard error | Cell sound? |
     |---|---|---|---|---|---|
     | Unusable | Settings malformed | Not tried | Not tried | invalid settings, not confirmed, password | Yes |
     | Unusable | Unreachable | Not tried | Not tried | connection failed, not confirmed, password | Yes |
     | Unusable | Reachable | Busy past 10 s | Not tried | busy, not confirmed, password | Yes, but see logic-r3 5 for a cancel |
     | Unusable | Reachable | Held | Migration failed | migration failed, not confirmed, password | Yes |
     | Unusable | Reachable | Held | Commit confirmed | committed, password | Yes |
     | Unusable | Reachable | Held | Commit outcome unknown | reconciliation did not complete, not confirmed, password | Yes |
     | Valid | any failure | any | Not applicable | detail line only, no outcome line | Yes |

   - The committed removal exits 1 and Stack Web decision 43 keys success on the printed line, so the two plans agree.
   - Row 27 against rows 11, 18 and 20: consistent in content, stale in status (logic-r3 7).
6. **Round 2 blockers and majors still open.** Round 2 had no blocker. Its majors are architect-r2 1, architect-r2 2, logic-r2 1, logic-r2 2, logic-r2 3, logic-r2 4 and threat-r2 1. All are closed in substance except the clock test path, which is logic-r3 1 and keeps logic-r2 4 half open.
7. **Lens questions for patched text.** See the next section.

## Answers to the lens questions

1. **Ordering, ties, gaps, restart.** Session order is `created_at DESC, verifier DESC`. The tie order is deterministic, a row ahead of the sample is moved by one microsecond, and a row more than a minute ahead is pruned, so a tie or a backward clock within the bound is covered. A restart keeps rows and rebuilds the cache cold. The cold cost is in logic-r3 2. The lock identifiers are distinct and tested.
2. **Failure permits or denies, and repair.** Clock overflow denies sign-in for at most one minute, until the stray row ages inside the bound, and the transaction rolls back its prune. A lost lock cancels serving. A lock acquired by a second instance during the one-second probe gap is a window of about one second plus the probe time. Session writes stay serialized by the table lock, so no data harm follows. Findings: logic-r3 5 for acquisition errors, logic-r3 6 for a stalled probe.
3. **Truth tables.** Built above for the diagnostics and in logic-r3 2 for the cache. No empty cell remains in the diagnostics table.
4. **Zero values.** The empty password is the removal route and is handled before the vault. An empty request key is refused like a present one by item 7. A zero cache entry and a zero-length token are rejected by the canonical decoder.
5. **Two actors on the same state.** Revoke against a concurrent change of the same row is closed by the tuple comparison under the lock. Two Issues are serialized by the table lock. Two instances are excluded by the advisory lock. The cache mutex is the open item (logic-r3 2).
6. **Risky items and their mutations.** Item 4: "moving revocation's derivation under the lock must fail `TestRevokeVerifiesBeforeLock`" is real. "Using the Go clock must fail `TestSessionsUseDatabaseClock`" is real. A wrong change not caught: replace least-recently-used eviction with first-in first-out, or remove eviction (logic-r3 2). Item 7: mapping the clock refusal to another code is not caught (logic-r3 1).
7. **Cheapest wrong implementation that passes.** Item 4: an unbounded cache map with a mutex around everything. Item 7: `ErrSessionClock` mapped to `unavailable`. Item 9: a probe with no timeout (logic-r3 6) and a lock that reports busy for any acquisition error (logic-r3 5).
8. **A shape that would make the wrong state impossible.** Two changes would do it. Make `ErrSessionClock` and `ErrNoSession` one exported type with a constructor, so a wrapper cannot hide them. Make `AcquireServerInstance` return a value that carries its own cancel function and closes the connection in one `Close`, so a failed acquisition cannot leak a connection that may later hold the lock.

## Post-build checks

- PB-A (new): run `go test -race` on `TestSessionBindingCache` with 100 workers and eight cold sessions. Record the longest wait of a cache hit behind a cold derivation, and require it to stay under one second.
- PB-B (new): in the sprint gate, send SIGTERM during a held lock wait and require the exit status and the line to say cancellation, not the busy sentence.
- PB4, extended: after a deliberate crash between the OpenBao create and the pin commit, restart and read the `pin` field (see logic-r3 4).
- The existing PB1 to PB3 stay as written. The sprint gate `deploy/test/checks.sh --only C1,C2,C3,C4,C13` does not cover any item above, so these checks belong to the orchestrator's list.

## Grade

7 of 10. Blockers: 0. Majors: 2. Minors: 5.
