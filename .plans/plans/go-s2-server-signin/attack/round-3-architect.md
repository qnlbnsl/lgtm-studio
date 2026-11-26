SHIP

Task: review-go-s2-server-signin-r3-architect. Nonce: 46d6fe. Lens: architect. Plan commit under review: 9d87abd0.

Top finding: the only test of the new five-second probe delays the first query by 1.5 seconds on the client, so it proves the bound is above 1.5 seconds and never proves that a stalled probe ends the server (architect-r3 1).

No blocker and no major survived. The patches remove the failures that their findings described. Nine minor defects remain in the patch text, and each is a small edit or a test line.

## Findings

### architect-r3 1 — MINOR — The probe bound is never exercised from above

- Tied statement: R10 and C4, plan item 9, `TestServerInstanceLock`.
- Evidence: `plan.md:493` says an exhausted probe budget cancels the app. `plan.md:544` tests only a tracer that delays the first `SELECT 1` by 1.5 seconds and a terminated backend. `attack/round-2-findings.md:16` describes the test as a real delayed query.
- Failure scenario: a builder writes the probe with no timeout, or with a timeout of one hour. The tracer test still passes, because 1.5 seconds is below any bound. The terminated-backend test passes too, because it takes the error path and not the timeout path. A server whose database connection hangs without closing keeps serving while the lock may already be gone, and no test fails.
- Prescribed fix: add a loopback TCP relay between the test and PostgreSQL that withholds the probe's reply. Hold it for 1.5 seconds and require the same backend to survive. Hold it for longer than the budget and require that the app cancels, serving closes and a second instance acquires the lock. Name the mutation "remove the probe bound" as a failure of that test. Say in the register that the tracer delay is client-side and covers only the lower bound.

### architect-r3 2 — MINOR — Nothing proves that the Server never issues an update, and the Stack follow-up has no record

- Tied statement: D7 and C13, plan item 8, decisions row 29.
- Evidence: `plan.md:450` says Server uses read and initial compare-and-set creation only, and that a Stack-owned permission change is requested separately. `deploy/openbao-server-policy.hcl` grants read, create and update. `deploy/test/vault-init.sh:72` and `:90` require update to succeed. `docs/issues.md` entries 1632 to 1643 hold no entry about narrowing this policy. Decisions row 29 is `unanswered`. The named wire tests check `options.cas=0` on creation and "no create" from a second client, and none asserts that every write is a creation with `cas` zero.
- Failure scenario: a builder handles the version conflict by re-reading the version and writing again with `cas` set to that version. The real vault, under the current policy, accepts it. The signing key changes silently. The pin makes the next start fail, so the owner loses the server instead of seeing the bug in a test.
- Prescribed fix: add to `TestLinkSigningKeyCreateRace` an assertion that the fixture saw no write whose body lacks `cas` zero, and no `PUT`, `PATCH` or `DELETE`. Record the policy narrowing (drop `update` from the server policy and flip `vault-init.sh:90`) as the next issue number in `docs/issues.md`, as the writing habit requires, and name that number in the plan.

### architect-r3 3 — MINOR — The expected Connect overrun text has a second variant

- Tied statement: R8, plan item 6, error table row at `plan.md:300`.
- Evidence: `plan.md:300` and `:308` pin the text `message size <bytes> is larger than configured max 32768`. In the pinned Connect source, `envelope.go:350` and `compression.go:105` emit `message is larger than configured max 32768 - unable to determine message size: <cause>` when the remaining bytes cannot be counted. Neither text is one capitalized sentence, which R8 asks for.
- Failure scenario: the Session wire test sends a body that the client cuts off after the limit. The server cannot count the rest, answers with the second text, and the exact-text assertion fails on a correct build. Or the test passes and the owner sees a library sentence that breaks R8.
- Prescribed fix: assert the code `resource_exhausted` and a prefix common to both texts, or state that the Session handler rewrites the message to one fixed sentence in an interceptor and test that sentence.

### architect-r3 4 — MINOR — The clock fixture needs an interface that the plan does not name

- Tied statement: R10, plan item 4.
- Evidence: `plan.md:185` says to put the fixture schema explicitly before `pg_catalog` in the test Store configuration. `src/server/internal/store/storetest/postgres.go:23` returns a `*pgxpool.Config` and sets `search_path` to the one quoted schema. `storetest/migrated/store.go:12` takes only a schema name and builds the Store itself. Item 4's file list has no helper.
- Failure scenario: the builder of item 4 uses `migrated.NewStore` as the other items do. The schema is searched after `pg_catalog`, the built-in `clock_timestamp()` wins, and `TestSessionsUseDatabaseClock` passes or fails for the wrong reason. Or the builder edits `storetest`, which the Store plan owns, inside an item that does not list it.
- Prescribed fix: say that the test calls `storetest.NewDatabase(t, "clockfix")`, sets `RuntimeParams["search_path"]` to `clockfix, pg_catalog` on the returned config, calls `store.Open` and `Store.Migrate`, and does not use `migrated.NewStore`. Say that the shadow also applies to any unqualified clock call in Store code that runs in the same test.

### architect-r3 5 — MINOR — `ErrSessionClock` has no declared surface and no test at the API layer

- Tied statement: R8, plan items 4 and 7.
- Evidence: `plan.md:179` names the sentinel and says item 7 maps it. The exported block of item 4 (`plan.md:157` to `:171`) lists `ErrNoSession` and not `ErrSessionClock`. The mapping row is at `plan.md:303`. Item 7's tests (`plan.md:336` on) never provoke it.
- Failure scenario: the sentinel is unexported or wrapped by the Store write path, the router's error mapping tests `Outcome()` first, finds a domain error and answers `internal`. The auth test passes and a user sees the wrong code.
- Prescribed fix: add `var ErrSessionClock` to the item 4 block. Add one API test that seeds a row one minute ahead and requires the code `aborted` with the sentence at `plan.md:303` and no cookie.

### architect-r3 6 — MINOR — The rejection of threat-r2 6 rests partly on opinion

- Tied statement: D7, `attack/round-2-findings.md:27`.
- Evidence: the first half is real. `attack/plan-v2.md:441` places restoring an earlier database state outside wave 1, and `docs/issues.md` has issue 1641 for it. The second half says the first-start path cannot distinguish restore from initialization. `plan.md:450` and `:452` show that the loader knows whether it created the entry, adopted a stored one, or read an existing pin.
- Failure scenario: a database restored from before the pin existed, beside a vault that holds a different key, adopts the vault key and logs the same `link_signing_key_loaded` line as a first start. The owner cannot tell.
- Prescribed fix: either add the `pin` field with the values `existing`, `created` and `adopted` to the log event and to PB4, or answer the finding as declined for cost, pointing to issue 1641, and drop the sentence about distinction.

### architect-r3 7 — MINOR — A pin mismatch has no owner action

- Tied statement: D7 and C13, plan item 8 and the diagnostics table.
- Evidence: `plan.md:452` makes absence or mismatch fail startup. The table row near `plan.md:513` says only that OpenBao did not provide the established key. The Stack Web plan sets `restart: unless-stopped` on the server. Issue 1639 leaves rotation outside the wave.
- Failure scenario: the owner removes the vault's data and keeps the database. The server fails at step 5 and the container restarts forever. The only way out, deleting the one pin row, is written nowhere.
- Prescribed fix: add to the diagnostics sentence or to the Stack guide the fact that an owner who accepts a new key deletes the `server_link_identity` row, and that the next start creates a new key. State that clients must then re-pair (or that wave 2 will).

### architect-r3 8 — MINOR — The cache test and the cache mutex are loosely specified

- Tied statement: R7, plan item 4.
- Evidence: `plan.md:181` says the lookup and the missed derivations share one mutex and that the context is rechecked after the lock is taken. `plan.md:193` warms eight entries, issues a ninth, then counts "one derivation for that tuple".
- Failure scenario: the text does not say whether `Issue` inserts its own tuple. If it does, the count is zero and the test fails on a correct build. If it does not, the count includes the issuing derivation and the hook cannot tell the two apart. A plain mutex cannot honour a cancelled context, so a request with a dead context still waits for the running derivation, and cache hits wait behind a miss.
- Prescribed fix: say that `Issue` seeds the cache and that the test expects zero check-time derivations, or the opposite. Say that a cache hit takes the lock for the lookup only and that derivation runs behind a channel that a cancelled context can leave.

### architect-r3 9 — MINOR — Two decisions rows disagree about who pins the new modules

- Tied statement: `plan`, decisions rows 24 and 32.
- Evidence: `decisions.md` row 24 says the Contract team pairs both new pins with the first importer. Row 32 says each first importer adds its pin and supersedes row 24. Both are `unanswered`.
- Failure scenario: a builder reads row 24, waits for the Contract team, and item 2 stalls. Or the orchestrator checks `go.mod` against row 24's owner and rejects a correct change.
- Prescribed fix: mark row 24 as superseded by row 32 and keep one `unanswered` row. The versions themselves are real: `go.mod` already has connect, pgx and protobuf at the stated versions, and the cached `go.mod` of `golang.org/x/net` v0.63.0 requires `golang.org/x/crypto` v0.59.0.

## Answers to the lens questions

1. Claims about code and tools: checked. The server lock identifier, `StartFollower`, `Follower.Close`, `RunPruner`, `Store.Read`, `ReadTx`, `NewDomainError` and `Store.Migrate` resolve as the Store plan or the tree states. `StartFollower` and the write path are still stubs in the tree, and the plan names them as prerequisites. The Stack policy file matches the plan's description. One anchor needed a note: `AcquireServerInstance` takes its configuration from `Store.pool.Config()`, because `Store` keeps no separate configuration, and that is workable inside the store package.
2. Writers and readers: complete for `Sessions`, `ErrNoSession`, `CheckInRead` (the Machines plan calls the same signature), the three cookie names, the eleven settings (the Stack Web plan lists the same eleven), the `health` command, the removal route's five database settings and the two revocation lines. Missing: the exported surface of `ErrSessionClock` (architect-r3 5).
3. Placement: right. The vault pin lives in the server part, the lock in the Store package, the policy in Stack. The pin's feed topic has no reader yet and costs one feed row, which is harmless.
4. Rules R1 to R11: R1 kept (the vault part owns its migration), R2 kept, R3 kept, R4 kept (the pin goes through `Store.Write`), R5 and R6 do not apply, R7 kept (the cache can be rebuilt from rows), R8 weak at the Connect overrun text (architect-r3 3), R9 kept, R10 kept except the clock fixture interface (architect-r3 4), R11 does not apply.
5. Other plans: the Stack Web plan's settings, health command, removal route and revocation lines match the Server plan. The Machines plan's `CheckInRead` call matches. Decisions rows 24 and 32 disagree (architect-r3 9).
6. Each item merges alone: items 8 and 9 need the Store plan's write, follower and pruner to be real, and the plan lists them as prerequisites. Nothing else breaks between items.
7. Room for wave 2: kept. The reserved `join-token` command, `allowInitialize=false` and the pin leave room. The plan builds nothing that wave 1 checks do not need, apart from the feed record of the pin.
8. Dependencies: `connectrpc.com/connect v1.21.1`, `github.com/jackc/pgx/v5 v5.13.0` and `google.golang.org/protobuf v1.36.12` are already in `go.mod`. `golang.org/x/net v0.63.0` is in the local module cache and requires `golang.org/x/crypto v0.59.0`, so both pins agree. The `v0.59.0` archive itself is not in the local cache, so the first builder needs the network. `WithReadMaxBytes` exists in connect v1.21.1.

## Answers to the task questions

1. Patches against their findings: the clock sample, the cache bound, the verify-then-recheck `Revoke`, the plaintext aliases, the three cookie names, the pin and the phase budgets each remove the failure as written. The probe patch removes the pause failure and leaves its upper bound untested (architect-r3 1). The vault patch removes the replacement failure only through a detective control (architect-r3 2). No patch breaks another statement of the design.
2. Each `UNREVIEWED MECHANISM`: the clock sample and `ErrSessionClock` hold (architect-r3 5 for the surface). The shadow schema works with a stated interface (architect-r3 4). The `deriveKey` hook is private and same-package, which is fine. The cache and mutex are loose (architect-r3 8). `Revoke` is correct. The aliases and three cookie names hold in the truth table below. `WithReadMaxBytes` has two texts (architect-r3 3). The pin and policy are in architect-r3 2, 6 and 7. The phase budgets fit under the connection's inherited 15-second statement limit, and the plan does not say so. The advisory lock with a probe is sound and its test is weak (architect-r3 1).
3. Rejected findings: threat-r2 6 is half real (architect-r3 6). The other rejections in the register rest on text I checked in the plan and were not contradicted.
4. Added or changed names: the writer and reader lists are complete, apart from the surface of `ErrSessionClock`.
5. Contradictions: none inside the patches. The truth table for modes, below, has no gap. Rows 24 and 32 of the decisions disagree.
6. Round 2 majors: none is open. architect-r2 1 is fixed another way, with the follow-up unrecorded (architect-r3 2). logic-r2 2 is fixed with a weaker test than the register says (architect-r3 1).
7. Lens questions that patches touch: answered above, questions 1, 2, 4, 5, 6 and 8.

Truth table for cookie modes (public origin, request origin, result): HTTPS public with an absent origin gives Strict and the secure name. HTTPS public with a plaintext alias is refused. Plaintext loopback public with an alias at the same port gives Strict and the plain name. The same alias at another port is refused unless listed. A listed cross-site HTTPS origin under an HTTPS public origin gives None, Secure and the cross-site name. A listed cross-site entry under a plaintext public origin is rejected at startup. Cross-site fetch metadata with no origin is refused. I found no combination without a result.

## Post-build checks

1. Hold a probe reply for 1.5 seconds through the relay: the server keeps its backend and serves. Hold it for longer than the bound: the app cancels and a second instance acquires the lock within its budget.
2. Make the vault fixture answer the create with a version conflict: the client reads the stored key and sends no second write.
3. Seed a session row one minute ahead and sign in over HTTP: the code is `aborted` with the fixed sentence and no cookie.
4. Send a Session body above 32 KiB with and without a length header: the code is `resource_exhausted` in both cases.
5. Remove the vault's data and keep the database: startup fails before listening, no create request is sent, and the log names the pin.
6. Restart with eight live sessions and send eight first requests at once: every one succeeds and the derivation count is eight.

## Grade

Grade: 8 of 10. Blockers: 0. Majors: 0. Minors: 9.
