REVISE

The planned Compose service supplies different database and listener settings and calls a health command that the sign-in plan never defines, so a clean stack cannot reach readiness as written.

## Findings

### logic2-r1 1 — BLOCKER — C1 — Compose cannot start the planned server

**Evidence.** The sign-in plan requires `LGTM_SERVER_LISTEN_ADDR`, the five PostgreSQL settings from sprint 1, and `LGTM_ALLOW_PRIVATE_STACK=true`. It exposes `GET /healthz` with status 204. The sprint 2 Stack plan passes `LGTM_LISTEN_ADDR` and `LGTM_POSTGRES_URL`, omits the private-stack opt-in, supplies an empty public origin in default mode, and probes `lgtm-server health`. Its launcher waits for that probe. The sprint 1 database handoff is already precise. `.plans/plans/go-s2-server-signin/plan.md:355`, `.plans/plans/go-s2-server-signin/plan.md:427`, `.plans/plans/go-s2-stack-web/plan.md:259`, `.plans/plans/go-s2-stack-web/plan.md:260`, `.plans/plans/go-s2-stack-web/plan.md:301`, `.plans/plans/go-s1-stack-vault/plan.md:344`.

**Failure.** The default Compose environment leaves the server pointing to its own container for PostgreSQL. Fixing that alone leaves private network access and the bind unresolved. Even a running server only satisfies an HTTP (Hypertext Transfer Protocol) probe that the planned image never sends.

**Fix.** Close the two Stack handoff decisions before either team freezes its tests. Make Compose pass the Server names and the sprint 1 database settings, including the private-stack opt-in. Compute the server's public origin from the actual published port in loopback mode. Give the distroless image a bounded `lgtm-server health` subcommand that only checks the private readiness endpoint and exits successfully on 204. Test the merged container at a nondefault loopback port.

### logic2-r1 2 — MAJOR — C4 — A failed password transition leaves a route back to old sessions

**Evidence.** The design requires changing or removing the password to end every session before the server opens its port. The plan makes the purge transactional and tells the operator to repeat a failed removal before restoring the old password. The Stack launcher rejects an empty password before it reaches the server phase. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:202`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:208`, `.plans/plans/go-s2-server-signin/plan.md:121`, `.plans/plans/go-s2-server-signin/plan.md:427`, `.plans/plans/go-s2-stack-web/plan.md:296`.

**Failure.** A server with live sessions stops. Starting with a changed or missing setting fails before the purge commits. When the database comes back, restoring the old setting makes its bindings match, and those sessions work again. If the operator uses the empty-password launcher path while the old server is running, the launcher can exit while that process still serves its original sessions.

**Fix.** Define a password transition as a completed store operation and give the deployment a supported transition path that stops the old process, commits the purge and verifies completion. Keep a failed transition pending and prevent a serving rollback through that path until the purge commits. If the design requires a change attempted during a database outage to survive an arbitrary direct restart, settle where the durable transition intent can live. The existing settings and session rows alone cannot distinguish that restart from an ordinary restart. Add a transition-failure, recovery and restoration scenario to the application and Stack gates.

### logic2-r1 3 — MAJOR — C4 — An expired session can become valid again

**Evidence.** Authorization and revocation use current database wall time. Authorization leaves an expired row in place. Issue and reconciliation prune it only when they independently observe it as expired. Issue also borrows the former high timestamp to order a new session after a backward clock step. `.plans/plans/go-s2-server-signin/plan.md:121`, `.plans/plans/go-s2-server-signin/plan.md:153`, `.plans/plans/go-s2-server-signin/plan.md:155`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:204`.

**Failure.** At the exact 720-hour cutoff the server refuses a stored token. With no intervening prune, stepping the PostgreSQL clock back across the cutoff makes the same token pass. Evaluating the planned comparison at one microsecond before expiry, at expiry and then one microsecond before expiry returned true, false and true in this review. A cookie's browser age cannot enforce the server-side expiry of the underlying credential.

**Fix.** Specify a durable rule that makes expiry final, and use it consistently in startup, issue, authorization and revoke. Separate insertion order from the time used for expiry and password binding. Define the accepted clock behavior and the failure policy for a database clock discontinuity. Freeze a boundary test that refuses a token at the cutoff and still refuses it after clock regression and restart.

### logic2-r1 4 — MAJOR — C13 — A different stored key can silently become the server's identity

**Evidence.** The application reads the persisted seed once and hands the private key to the link in memory. A future startup accepts any well-formed seed from the path. Stack grants the server both create and update on that path. The restart driver detects a changed fingerprint only while it runs its paired comparison. `.plans/plans/go-s2-server-signin/plan.md:393`, `.plans/plans/go-s2-server-signin/plan.md:399`, `.plans/plans/go-s2-server-signin/plan.md:429`, `.plans/plans/go-s1-stack-vault/plan.md:261`, `deploy/test/checks.sh:630`, `deploy/test/checks.sh:646`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:271`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:387`.

**Failure.** A running server holds key A. OpenBao later contains well-formed key B. The running process still uses A. Its next start reports B as successfully loaded and opens the listener, even though clients enrolled under A will refuse it. Removing the entire key entry yields the same outcome through automatic creation.

**Fix.** Persist the expected public fingerprint with the installation before first readiness. Permit creation only for an installation without an established identity. Compare every later startup read to the expected fingerprint and fail before listening on mismatch or loss. Document that the running key stays fixed and that an intentional identity replacement requires a separately defined operation. Keep the seed in OpenBao. Specify a test with a valid different seed and a test with an unexpectedly absent entry after successful first startup.

### logic2-r1 5 — MAJOR — D11 — Sign-ins through the two website modes overwrite one browser cookie

**Evidence.** Both modes set `lgtm_session` for the same host and path. The plan checks them separately and signs out separate jars. The cookie storage model uses the name, domain, host-only flag and path for replacement. `.plans/plans/go-s2-server-signin/plan.md:221`, `.plans/plans/go-s2-server-signin/plan.md:223`, `.plans/plans/go-s2-server-signin/plan.md:294`, `.plans/plans/go-s2-server-signin/plan.md:310`, `Cookies: HTTP State Management Mechanism`, section 5.7, step 23.

**Failure.** In a browser profile that permits unpartitioned cross-site cookies, the listed cross-site website signs in and receives `SameSite=None`. Signing in on the bundled website then replaces that cookie with `SameSite=Strict`. The cross-site website loses its session although its row is still live. In the reverse order the bundled website presents the newer cross-site token, so its SignOut revokes the credential the other website is using.

**Fix.** Decide whether the two modes are independent within one browser profile. To support both at once while preserving their specified attributes, give the modes separate cookie names and consistently select the correct name for reading, issuing and clearing. Test both sign-in orders and sign-out from either mode in one browser profile. If one browser cookie is the intended contract, document which actions replace it and revise acceptance to match.

### logic2-r1 6 — MAJOR — plan — The retry exception contradicts the general write rule

**Evidence.** The foundation says a mutating request can be sent twice and happens once. Item 7 refuses a request key on SignIn, and its test requires an unkeyed retry to create two live sessions. The choice log marks the exception provisional. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:117`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:176`, `.plans/plans/go-s2-server-signin/plan.md:290`, `.plans/plans/go-s2-server-signin/plan.md:297`, `.plans/plans/go-s2-server-signin/decisions.md:14`.

**Failure.** With seven existing sessions, a successful SignIn whose reply is lost fills the eighth slot. Retrying creates a second session and evicts one of the original seven. The current test deliberately accepts this outcome. SignOut already changes the row at most once, but a repeat after a successful commit reports `unauthenticated` because its credential is gone.

**Fix.** Reconcile and record the approved exception before freezing item 7. If the proposed SignIn policy stands, make the duplicate write and possible extra eviction explicit in the foundation retry rule and in the acceptance case. Keep credentials out of receipts. State the repeat response for SignOut separately from its one-time state change.

### logic2-r1 7 — MAJOR — C3 — The auth adapter needs the Store's safe error identities and commit outcomes

**Evidence.** Sign-in declares `ErrNoSession` and expects its identity to survive Store reads and writes. Its error table groups store failures under a generic unavailable message. The current Store plan preserves only direct sentinels registered by `NewDomainError` and requires adapters to map an unconfirmed commit before retryability. `.plans/plans/go-s2-server-signin/plan.md:141`, `.plans/plans/go-s2-server-signin/plan.md:155`, `.plans/plans/go-s2-server-signin/plan.md:258`, `.plans/plans/go-s1-server-store/plan.md:119`, `.plans/plans/go-s1-server-store/plan.md:137`, `.plans/plans/go-s1-server-store/plan.md:139`.

**Failure.** An ordinary Go sentinel returned from the read callback becomes a generic Store error. A well-formed cookie for a deleted row then gives `unavailable` on both public and protected calls. The design calls for a signed-out `GetServer` reply and `unauthenticated` on a protected call. The current public mapping also gives callers the same message for a lost commit acknowledgement and an ordinary storage outage.

**Fix.** Define `ErrNoSession` using `store.NewDomainError` and return it directly from Store callbacks. Map `OutcomeUnknown` before `Retryable`, with the Store's fixed unconfirmed-write message. Keep the cookie unchanged and forbid automatic reissue. Add ordinary-path cases for an expired or revoked canonical cookie and a deliberately unconfirmed commit.

### logic2-r1 8 — MAJOR — R1 — The startup timeout cuts off the Store's migration budget

**Evidence.** Store gives `Migrate` a five-minute budget and explicitly requires sprint 2 bootstrap to separate that budget from other startup work. The sign-in plan applies one two-minute startup context to prerequisites, and the Stack launcher waits only 120 seconds for the server. `.plans/plans/go-s1-server-store/plan.md:188`, `.plans/plans/go-s2-server-signin/plan.md:427`, `.plans/plans/go-s2-server-signin/plan.md:429`, `.plans/plans/go-s2-stack-web/plan.md:301`.

**Failure.** A migration or migration-lock wait completes after two minutes but within the Store budget. The application cancels or the launcher declares a failed start before readiness. Restart retries can repeatedly cancel a valid database upgrade at the same point.

**Fix.** Give migrations their separate bounded context and keep a distinct budget for the other prerequisites. Align Compose health retries and launcher wait with their combined budget. Test with short injected durations that model a migration exceeding the other-prerequisite budget while remaining within its own.

### logic2-r1 9 — MINOR — C4 — The lock and expiry checks can miss their intended failure

**Evidence.** Item 4 samples the wall clock and then checks an expiry fixture established in an earlier statement. Its trigger setup makes the second writer wait, and the Store has its own lock timeout. Item 9 probes the port before releasing a database gate without stating how the observer proves startup reached that gate. `.plans/plans/go-s2-server-signin/plan.md:159`, `.plans/plans/go-s2-server-signin/plan.md:160`, `.plans/plans/go-s2-server-signin/plan.md:453`, `.plans/plans/go-s1-server-store/plan.md:135`.

**Failure.** Rejecting only sessions strictly older than 720 hours can pass the proposed boundary fixture because time elapses between setup and authorization. A single early port probe can also pass an early-listen mutation when startup has yet to be scheduled. A missing lock can turn a mutation into a fixture timeout if the test waits exclusively for the expected blocker.

**Fix.** Freeze the same cutoff for fixture and authorization. Establish startup arrival at the reconciliation gate before probing the port. In the two-writer test, assert the second writer's state after it reaches the protected section and report premature trigger entry as the mutation failure. Keep fixture waits inside the Store lock deadline.

### logic2-r1 10 — MINOR — plan — The root dependency handoff has no versions

**Evidence.** Items 2 and 5 name `golang.org/x/crypto/scrypt` and `golang.org/x/net/publicsuffix`, but leave their module versions unspecified. The epic requires each module and version before its first importer. `.plans/plans/go-s2-server-signin/plan.md:81`, `.plans/plans/go-s2-server-signin/plan.md:201`, `.plans/epics/go-rewrite/epic.md:129`.

**Failure.** The orchestrator cannot perform the exact handoff the epic calls for. The public suffix list version also affects cookie policy for some origins.

**Fix.** List the exact module versions adopted by the root module, including a transitive version if it already supplies the chosen package. Put each handoff in the dependency for its first importer and verify the root tidy check with that importer present.

### logic2-r1 11 — MINOR — C4 — The release driver can turn a valid password into an invalid fixture

**Evidence.** The plan accepts configured passwords up to 4096 bytes. The release driver appends a suffix to construct the replacement setting. `.plans/plans/go-s2-server-signin/plan.md:84`, `.plans/plans/go-s2-server-signin/plan.md:90`, `deploy/test/checks.sh:415`.

**Failure.** C4 starts with a valid 4096-byte password. The replacement setting is over the limit, so the correctly implemented server refuses startup and C4 cannot test rotation to a valid password.

**Fix.** Have the Stack owner generate a distinct, valid-length replacement independently of the original length. Add a release-driver fixture at the configured password limit.

## Answers to the attack questions

1. **Order and time.** The session lock serializes writes, ties advance one microsecond and the verifier breaks a stored tie. Duplicate verifiers fail the insert, and transaction gaps have no auth meaning. The Store owns feed positions, restart reads stored timestamps, and PostgreSQL wall time can revive expiry. The timestamp range near overflow has no specified boundary beyond a finite SQL (Structured Query Language) check. The vault lease uses local elapsed time and a one-minute refresh margin. A process restart logs in again. Findings 3, 8 and 9 cover the timing gaps.
2. **Failure and repair.** The failure cases are in the tables below. A failed commit issues no cookie, and an ambiguous commit may leave an unused row. Authentication denies on storage failure for a presented credential. Anonymous `GetServer` can still answer. The plan distinguishes canceled, retryable and unavailable requests. Findings 2, 3, 7 and 8 cover the remaining failure-policy gaps.
3. **Overlapping rules.** The session, origin and vault tables below name the affected cells. A rejected password preserves an existing valid cookie. A successful password transition purges it. A failed transition followed by restoration, a clock regression, successive sign-ins from both website modes, and a valid replacement seed are the unclosed cases.
4. **Zero values.** A missing or empty configured password enters purge and exit. An absent or empty candidate is rejected. A missing, duplicated or malformed cookie is unauthenticated on protected calls. An absent Origin uses public-origin policy unless the request reports cross-site fetch metadata. Empty allowlist admits only the public origin for requests with Origin. An empty seed or nonpositive lease fails. The contract's zero `SIGN_IN_RESULT_UNSPECIFIED` has no successful return path. The implementation must initialize `ErrNoSession` as a nonnil sentinel. `src/contracts/proto/lgtm/v1/session.proto:43`, `.plans/plans/go-s2-server-signin/plan.md:141`.
5. **Concurrent actors.** SignIn, SignOut and reconciliation share the table lock. The database read can see the prior committed row and cannot undo already admitted work. Two sign-outs race to one delete and the other gets `unauthenticated`. A pending request admitted before revoke has only the router's admission-time check unless its service rechecks. Watch specifies a fresh authorization for later messages. Two creators of the first vault seed converge through compare-and-set. Deployment assumes one serving process and stops the old process before a different password takes effect.
6. **Named mutations.** The table below audits the mutations on all nine risky items and names an additional wrong change for each. I recomputed the item 2 binding vector and confirmed that halving its scrypt cost changes the output.
7. **Weakest passing implementation.** The final column of the item table gives one wrong implementation the written tests can miss for each item. Those are gaps in the planned assertions, since the test files do not yet exist.
8. **State shape.** The primary key and width checks make duplicate or malformed verifiers impossible on the normal write path. Encode cookie mode in cookie identity if both website modes must persist. Separate creation order from expiry and bind the vault seed to the installation's durable public identity. Record a transition before allowing rollback to a former password on the supported deployment path.
a. **Sign-in, change and restart.** The session table covers C2 through C4. Findings 2 and 3 identify two paths where an old session can work after it was expected to end.
b. **The cookie and origin boundary.** The browser table gives the attributes and delivery rules. A database reader cannot present the verifier as a cookie because the server hashes the supplied token before lookup. The salt and binding can check password guesses offline, so a weak owner password still matters. `.plans/plans/go-s2-server-signin/plan.md:92`, `.plans/plans/go-s2-server-signin/plan.md:151`.
c. **Vault and exact interfaces.** The vault table covers first start, competing creation, outages and mismatch. The sprint 1 symbol names match by reading. Findings 7 and 8 cover Store behavior the sign-in handoff misses, and finding 1 covers the sprint 2 Stack wiring.

### Session states for checks C2 through C4

| State or transition | Result under the plan | Design check or gap |
|---|---|---|
| No cookie and a wrong, missing or oversized candidate in a body within the protocol limit | The call rejects the candidate and writes no session. Protected calls still refuse. A body above the protocol limit gets `resource_exhausted`. | C3 holds. |
| Live cookie and a wrong candidate | The wrong attempt writes nothing and leaves the existing session live. | C3 refuses the attempt. No sign-out is requested. |
| Correct password and a committed issue | The new cookie authorizes. At the cap, one oldest live token ends. | C3 holds. |
| Issue fails before commit, or the reply or commit acknowledgement is lost | The request gets no new cookie. A committed issue whose reply is lost can leave a row. An explicit new sign-in can consume a second slot. | The session cap holds. The general retry rule needs the exception in finding 6. |
| Same password, a live token and an ordinary restart | Reconciliation keeps the row. The original cookie remains usable when origin, host and browser policy still permit it. | C4 holds. |
| Changed password and committed reconciliation | The mismatched live binding purges every row before requests are accepted. Reusing the former setting cannot bring deleted tokens back. | C4 holds. |
| Missing, empty or invalid configured password and a reachable store | Reconciliation purges and the process exits. A later start with the former password starts without the old rows. | C2 and the removal rule hold when the server is invoked and the purge commits. |
| Missing or changed password and a database outage, rollback or pre-start launcher refusal | The attempted process stays closed. Old rows can survive and match again after restoration. A refused launcher invocation can leave the preceding process intact. | C4 has the transition gap in finding 2. |
| Expired token and continuous, advancing database time | Authorization refuses at the cutoff. Issue or startup subsequently deletes the row. | The 30-day rule holds under that clock. |
| Expired but unpruned token and a backward database clock step | Authorization can pass again before a write prunes the row. | The 30-day rule has the revival gap in finding 3. |
| Protected call with an expired, absent, revoked or malformed cookie, or with a storage error | Missing authority returns `unauthenticated`. Storage errors produce a mapped failure and prevent dispatch. | C3 holds. Anonymous `GetServer` still reports server metadata. |
| Concurrent SignOut and authorization | A check made after the committed deletion denies. Earlier admitted work may finish. A second revoke finds no live row. | The planned Watch rechecks bound stream lifetime after revocation. |

### Cookie delivery and origin admission

| Request or browser state | Cookie and admission result |
|---|---|
| Public origin or a listed origin on the same site | The issued cookie is host-only, `Path=/`, `HttpOnly`, `Max-Age=2592000` and `SameSite=Strict`. It is `Secure` when the public origin uses HTTPS (Hypertext Transfer Protocol Secure). The cookie belongs to the server host. |
| Listed origin on another site and both use HTTPS | The same host, path, script and age restrictions apply. The cookie uses `SameSite=None; Secure`. The browser must allow cross-site cookies. The browser's preflight needs the exact allowed origin and credentials permission as well as the method and allowed headers. |
| Unlisted same-site origin | A browser may send a same-site cookie to the server on a request that can be sent without preflight. The exact origin gate denies dispatch and omits cross-origin response permission. Cookies also have no port boundary, so hosting a mutually untrusted service directly on another port of the cookie host needs a separate hosting decision. |
| Unlisted cross-site origin, with a Strict cookie | The cookie is withheld on a cross-site request. A supported browser call that needs preflight is stopped there. An actual request that reaches the server with a refused Origin is denied before dispatch. |
| Unlisted cross-site origin, with a None cookie | The browser may attach the cookie to a request that can be sent without preflight, subject to its cookie policy. An unapproved preflight still bars the website's credentialed contract call. The actual origin gate is what stops a request bearing that Origin before it can act. |
| Absent Origin | The public-origin mode is used unless the request declares `Sec-Fetch-Site: cross-site`. This supports native callers. Missing both headers gives the server no origin evidence. The exact method, recognized protocol and canonical credential still apply. |
| SignOut | The selected cookie is cleared only after revoke commits. Clearing uses an empty value, wire `Max-Age=0` and the epoch. A storage failure retains the cookie and produces an error. |

The website already uses `fetch` with `credentials: 'include'`. Cross-origin resource sharing (CORS) governs permission to send a preflighted call and share the response. Cookie delivery on simpler requests follows the cookie rules. See `src/frontend/src/server/api.ts:39`, the Fetch Standard sections 3.3.1 to 3.3.5, and `Cookies: HTTP State Management Mechanism` sections 5.7, 5.8 and 8.5.

### Vault startup and dependency names

| State | Result under the plan |
|---|---|
| First start with an initialized, reachable vault and a genuinely absent key | AppRole login gives a lease. The server makes one create-only compare-and-set write with `options.cas=0`. It reads and returns the persisted seed, then the application can listen. |
| Two first starts observe absence | The losing write must receive the recognized compare-and-set conflict. Both start attempts then read the same persisted winner. This resolves key creation. Production still has one serving process by deployment policy. |
| Create response lost | There is one confirming read inside the live startup deadline. A valid stored seed is usable. An absent, invalid or unreadable result fails startup without a second write. |
| Sealed, unreachable, unauthorized, malformed or versioned deletion | Startup fails before listening or emitting a success event. An initial transport failure, denied read or versioned deletion cannot trigger creation. A retry may succeed when the vault is ready or the original entry is restored. |
| Valid stored key differs from the running key | The live process holds the originally loaded seed. A new process accepts the changed seed and reports a different fingerprint. There is no startup identity check against the original. See finding 4. |
| Credential rotates or a lease expires | The client refreshes before the lease runs out. Stack recreates the container for an atomic file replacement, which gives the new process the new file. A rejected login fails its operation. `.plans/plans/go-s1-stack-vault/plan.md:428`. |

- **Store.** The planned imports match `store.Open(ctx, cfg, ConnectionPolicy{AllowPrivateStackPostgres: ...})`, `store.Migrations{Part, Files}`, `Store.Migrate`, `Store.Read`, `Store.Write`, `Tx.Record(ChangeRef)`, `store.RequestKeyHeader`, `storetest.NewDatabase` and `storetest/migrated.NewStore`. Startup must pass the error reporter to both `store.StartFollower(ctx, s, report)` and `s.RunPruner(ctx, report)`. The follower returns a handle and error. The `watch.Source.Authorize` callback accepts `*store.ReadTx`, and its unauthorized sentinel matches item 4. Later keyed mutations must also provide `ReceiptSafe`, `Caller` and versioned `Fingerprints`. The disposable wrapper must target the `lgtm_store_test` maintenance database, which the Store now requires by name and the Stack wrapper leaves unspecified. `.plans/plans/go-s1-server-store/plan.md:102`, `.plans/plans/go-s1-server-store/plan.md:141`, `.plans/plans/go-s1-server-store/plan.md:178`, `.plans/plans/go-s1-server-store/plan.md:192`, `.plans/plans/go-s1-server-store/plan.md:295`, `.plans/plans/go-s1-server-store/plan.md:361`, `.plans/plans/go-s1-server-store/plan.md:385`, `.plans/plans/go-s1-server-store/plan.md:416`, `.plans/plans/go-s1-server-store/plan.md:534`, `.plans/plans/go-s1-server-store/plan.md:578`, `.plans/plans/go-s1-stack-vault/plan.md:95`.
- **Stack Vault.** The names agree on the `LGTM_POSTGRES_DATABASE` spelling and four companion settings, the mounted `LGTM_OPENBAO_CREDENTIAL_FILE` and `LGTM_OPENBAO_ADDR`, the owner and mode, `role_id` and `secret_id`, AppRole login, `auth.client_token`, the 15-minute batch lease, the `lgtm/data/server/link/signing-key` path, `private_seed_b64`, compare-and-set on creation, and the three signing-key log fields read by C13. The tests also agree on `deploy/test/with-postgres.sh -- <command>`. The HTTP conflict and absent-entry wire details still need a captured response from the pinned server. `.plans/plans/go-s1-stack-vault/plan.md:95`, `.plans/plans/go-s1-stack-vault/plan.md:261`, `.plans/plans/go-s1-stack-vault/plan.md:338`, `.plans/plans/go-s2-server-signin/plan.md:355`, `.plans/plans/go-s2-server-signin/plan.md:395`, `deploy/test/checks.sh:610`.

### What the nine sets of tests prove

| Item | Would the named mutation fail the stated test? | Wrong implementation that can still pass the listed tests |
|---|---|---|
| 1. Settings | Only if `TestReadServerSettings` passes a service name other than `postgres` with private-stack opt-in enabled. The specified remote verified-TLS and opt-in-off cases alone do not exercise that branch. `.plans/plans/go-s2-server-signin/plan.md:376`. | Apply only the correct endpoint and TLS options for the fixture and accept a conflicting PostgreSQL service configuration otherwise. Add a service configuration or fallback fixture that would change the effective connection. |
| 2. Password | The source assertion catches replacing the constant-time helper with equality. The independent binding vector catches the named scrypt mutation. `.plans/plans/go-s2-server-signin/plan.md:96`. | Admit an invalid UTF-8 (8-bit Unicode Transformation Format) configured password. The tests cover valid non-ASCII input and lengths, but do not require rejection of malformed UTF-8. |
| 3. Reconciliation | Yes for skipping binding verification. The delete-trigger case catches returning success in the face of the delete error. It does not establish failure after a successful callback and failed commit. `.plans/plans/go-s2-server-signin/plan.md:125`. | Treat a later Store commit error as success. Add a store failure after the domain callback and assert the application remains closed. |
| 4. Sessions | The table-lock observation, rollback assertion and expired fixture target their respective mutations. The lock assertion needs the arrival and failure distinctions in finding 9. `.plans/plans/go-s2-server-signin/plan.md:159`. | Skip expiry pruning during issue, or use a strict-older-than comparison for expiry. The existing fixture can pass without checking physical pruning during issue or equality at one frozen time. |
| 5. Origins and cookies | The unlisted same-site case and emitted cross-site attributes both fail on the named mutations. `.plans/plans/go-s2-server-signin/plan.md:227`. | In one profile, overwrite the cross-site cookie with the Strict cookie and leave the existing cross-site row stranded. Individual attribute and parser scenarios still pass. |
| 6. Router | The protected SignOut procedure and denied-origin handler probe catch the named changes if the probe records entry before any method-specific validation. `.plans/plans/go-s2-server-signin/plan.md:270`. | Refuse a listed origin for one actual protected streaming call while still emitting correct credentialed headers on the unary cases. The protocol fixtures do not expressly combine an allowed browser origin and a protected stream. |
| 7. Session calls | Returning Set-Cookie on the failed issue or accepting a key on SignIn fails. The trigger exercises a domain-write error. The planned issuance interface has no pre-commit token return. `.plans/plans/go-s2-server-signin/plan.md:296`. | Revoke an existing session when its owner sends a wrong candidate, while still returning `REJECTED` without a cookie. The wrong-password test has no live credential attached. |
| 8. Vault | Returning the losing candidate fails if the fixture proves the candidates differ. Omitting compare-and-set fails the race only if the fixture checks or honors the option. The persistence test directly checks it. `.plans/plans/go-s2-server-signin/plan.md:407`. | Accept any well-formed different seed on a later start. C13 compares an unchanged vault across restart and the fixture never replaces the seed with a different valid one. |
| 9. Startup | Recreating the fixture database or using a transient key on error fails the named observations. The early-listener mutation requires the synchronization in finding 9. `.plans/plans/go-s2-server-signin/plan.md:453`. | Omit the follower or the pruner, or bypass the Compose command environment entry point in integration testing. The application and fixture tests observe session calls, the listener and the key, without asserting those lifecycles. The Compose integration will expose the command handoff. |

## Post-build checks

| Check | Scenario and expected result | Evidence and owner |
|---|---|---|
| PB1. Production handoff | Start the merged stack on a nondefault loopback port. Require server health, successful browser sign-in and C2 through C4. The health check must work in the actual server image. | Compose configuration, readiness status and the check output. Stack and Server. |
| PB2. Password transition recovery | Use a disposable live session. Interrupt or fail reconciliation, verify the port stays closed, complete the supported transition and restore the former password. The prior token must stay refused. Verify that the launcher does not leave the preceding server serving after an acknowledged removal. | Application probe and committed database state after the transition. Server and Stack. |
| PB3. Browser delivery | Test one browser profile with the bundled site and an allowed cross-site site. Then remove the allowance. An origin without permission never reaches a handler. Cookie delivery and sign-out follow the adopted two-mode policy. | Browser cookie attributes and service entry observations. Server and Stack. |
| PB4. Real vault creation | Use the pinned OpenBao on a disposable installation. Capture the genuine absent, competing create, versioned deletion and sealed responses with secrets removed. Require both racing clients to use the same winner and require no second create after an uncertain result. Recreate the stack and compare fingerprints through C13. | Redacted response classes, write count and public fingerprint. Server and Stack. |
| PB5. Key continuity | After successful startup, supply a different valid seed or remove the established entry in the fixture. The restarted server must follow the revised identity policy before listening. | Closed-port or explicitly authorized replacement observation and public fingerprints. Server. |
| PB6. Watch revocation | Once Machines supplies a stream, revoke its session while the stream is quiet or blocked. The Store's authorization recheck must end it with `unauthenticated` within its specified bound. A fresh stream with the old cookie must also fail. | Generated client status and timed Store authorization observation. Server. |

## Grade

5/10. The report records 1 blocker, 7 majors and 3 minors. I reviewed the working plans at base commit `32c6c345`, including the current uncommitted dependency plans. The binding-vector and expiry comparisons were checked independently. The Go server, root Go module and Compose application do not exist in this checkout, so the integration observations remain post-build checks.
