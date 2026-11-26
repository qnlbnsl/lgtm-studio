REVISE

Top finding: `architect-r2 1` (major). The plan asks Stack to remove the update right from the signing-key policy, but the reviewed Stack plan grants it on purpose, no handoff records Stack's agreement, and PB4 expects an outcome the Stack plan as written cannot produce.

- Task: `review-go-s2-server-signin-r2-architect`, nonce `695ff6`, lens `architect`, round 2.
- Scope: the patches of `git diff 32c6c345 f1dcbb7d -- plan.md decisions.md` and what follows from them.
- Method: every claim below was checked against `plan.md`, `decisions.md`, the round 1 register, the Store, Stack web, Stack vault, Machines and Stack client plans, the root `go.mod`, the module cache and the Store code that already exists under `src/server/internal/store/`.

## Findings

### architect-r2 1. The policy change the plan depends on is unbacked and contradicted (major, D7)

**Evidence.**
- `.plans/plans/go-s2-server-signin/plan.md:439` says "Ask Stack to restrict the key path to read and create" and says the follow-up read "finds the winner without update access".
- `plan.md:623` (PB4) expects "no update is authorized by the role".
- `.plans/plans/go-s1-stack-vault/plan.md:209` applies policy `lgtm-server` "granting read, create and update" at `lgtm/data/server/link/signing-key`. That line carries its own review marks (architect-r3 3, logic-r3 4), so it is a reviewed choice, not an oversight.
- The Stack web plan and the Stack vault plan contain no sentence that accepts the request. The sign-in plan has no list of requests to Stack besides the C5 helper (`plan.md:573`).

**Failure scenario.** The builder of item 8 and the builder of Stack's vault init each follow their own reviewed plan. The real vault then grants update. PB4 fails at its last clause. Worse, the item 8 text says the denied-create path "settles all create outcomes without adding update access". With update granted, the path is still correct, but the claimed property is false, and nobody finds out before the post-build check.

A second defect sits in the same patch. The instance lock (`plan.md:480`) leaves no competing writer, so the "denied competing creation" case in PB4 has no deterministic trigger on a real vault. A test can only produce it by writing the entry through an administrator token between the server's read and its create, which is a race. The scripted wire fixture in `vault_test.go` can produce it, but PB4 is stated against the real vault.

**Prescribed fix.**
- Write the request to Stack as a named handoff with the exact policy text: capabilities `read` and `create` on the path, nothing else. Add it to the sign-in plan's list of requests and to the Stack vault plan's "Contract for sprint 2" as accepted or refused.
- If Stack refuses, delete the sentence at `plan.md:439` that relies on missing update access and drop the clause "no update is authorized" from PB4. The create-then-read behavior is correct either way.
- Restate PB4's competing-create case as "the scripted wire fixture produces 403, 400 and a lost reply, and the real vault is checked for the absent, versioned-deletion and sealed cases only".

### architect-r2 2. Three tests need seams that no signature declares (major, R10)

**Evidence.**
- `plan.md:162-163` declares `NewSessions(s *store.Store, password Password)` and `Issue(ctx)` with no clock input and no private helper name.
- `plan.md:177` and `plan.md:181` freeze `TestSessionLifecycle` and `TestSessionIssueTieAndBackwardClock` against "a fixed clock sample from the test" and "the private authorization helper". The exact-microsecond boundary is the only evidence the patch for critic-r1 8 and logic2-r1 9 offers.
- `plan.md:464` declares `app.Start(ctx, cfg config.Config, log *slog.Logger)` with no budget input. `plan.md:530` freezes `TestStartupMigrationBudget`, which "injects shorter phase budgets" and observes "the deadlines received at the phase boundaries". `config.Config` (`plan.md:366-378`) has no such field.

**Failure scenario.** The test writer compiles against stubs, so the writer invents the seam names. The builder implements a different seam, or none. Either the first red run fails for a build reason, which the build loop does not count as red, or the test passes for an unrelated reason. The mutation named for item 9, "shorten the migration budget", cannot be shown to turn the test red. Round 1 marked logic2-r1 8 and critic-r1 8 "fixed". The fixes are in the text, but they cannot be verified as written.

**Prescribed fix.** Name each seam in the plan, as an unexported package symbol so the public surface stays as promised:
- In `auth`: an unexported `issueAt(ctx, sample time.Time)` and `checkAt(..., sample time.Time)` that the public methods call with the sample read from PostgreSQL, owned by item 4.
- In `app`: an unexported `phaseBudgets` value with the lock, migration and prerequisite durations, set by the test in `start_test.go`, owned by item 9.
- State that the test files are in the same package so they can reach the seams, and that `servertest` and `main` never see them.

### architect-r2 3. The instance lock has a ten-second budget it cannot spend (minor, C4)

**Evidence.**
- `plan.md:474` gives the lock "a separate ten-second acquisition budget".
- `plan.md:480` implements it with `pg_try_advisory_lock`, which returns at once, and "a denied lock yields a fixed already-running diagnostic". No retry is described.
- `plan.md:529` (`TestStartRefusesSecondInstance`) and `plan.md:517-520` (`servertest.Restart`) stop one instance and start another immediately. The Stack removal path does the same: `docker compose stop web server`, then `docker compose run --rm ... server` (`.plans/plans/go-s2-stack-web/plan.md:315-318`).
- `plan.md:480` also says the app "monitors the lock connection at least once per second" but not from when. The monitor is not tied to step 2 of the startup order.

**Failure scenario.**
- The old instance sends its terminate message and returns. PostgreSQL releases the session lock when the old backend exits, a moment later. The new instance opens its pool, dials the lock connection and calls the try-lock inside that moment. It prints "Another server is already using this database." and exits 1. Under `restart: unless-stopped` that loops until the race clears. In `TestStartAndRestartServer` and in every sprint 3 link test that restarts the fixture, it is a flaky failure.
- If the lock connection dies during the five-minute migration, nothing notices until the monitor starts at serving time. A second instance can take the lock and begin reconciliation while the first one is still migrating.

**Prescribed fix.** Retry the try-lock every 100 milliseconds until the ten-second budget ends, then print the busy sentence. Start the monitor immediately after the lock is held and run one probe before the listener binds. Allow a probe to fail twice in a row before the process closes. Add a test that holds the lock on another backend for three seconds and requires success, and one that holds it for twelve and requires the busy sentence at about ten seconds.

### architect-r2 4. The loopback aliases have no rule when the public origin is not plaintext loopback (minor, D11)

**Evidence.**
- `plan.md:244` admits the aliases "on the configured loopback port". `plan.md:389` says the operator's external `LGTM_PUBLIC_URL` is HTTPS only when supplied, and the Compose default is the numeric loopback spelling.
- `plan.md:246` selects the cookie from the request's origin (plaintext loopback gives `lgtm_session`, no `Secure`). `decisions.md` row 11 still says to use the configured public scheme for `Secure`. Row 20 supersedes only the name.

**Failure scenario.** The operator sets `LGTM_PUBLIC_URL=https://lgtm.example.com`. The plan does not say which port the "configured loopback port" is. An implementer may take port 443, admit `http://localhost:443` and answer a credentialed call from that plaintext origin with a non-`Secure` cookie. Another implementer admits nothing. `TestOriginPolicy` cannot freeze both. This is low risk to security, because only the operator's own browser reaches loopback, but the behavior is undefined.

**Truth table to put in the plan.**

| Public origin | Request origin | Admitted | Cookie |
|---|---|---|---|
| plaintext loopback on port P | none, or the same | yes | `lgtm_session`, Strict |
| plaintext loopback on port P | the localhost or IPv6 loopback spelling on port P | yes (alias) | `lgtm_session`, Strict |
| plaintext loopback on port P | the localhost spelling on another port | no | none |
| `https://host` | none, or the same | yes | `__Host-lgtm_session`, Strict, Secure |
| `https://host` | a localhost origin on any port | no | none |
| `https://host` | listed cross-site HTTPS origin | yes | `__Host-lgtm_cross_site_session`, None, Secure |

**Prescribed fix.** State that the aliases exist only when the public origin is plaintext loopback, and take their port from it. Add the "no" rows as cases of `TestOriginPolicy`. Mark row 11 of `decisions.md` as superseded for `Secure`.

### architect-r2 5. The module handoff leaves out the indirect bumps (minor, R9)

**Evidence.**
- `plan.md:63-71` lists five direct pins. The root `go.mod` has `golang.org/x/sys v0.29.0` and `golang.org/x/text v0.41.0` as indirect requirements and no `x/crypto`, `x/net` or `x/term`.
- The module file `golang.org/x/net@v0.63.0.mod` in the module cache requires `x/crypto v0.59.0`, `x/sys v0.50.0`, `x/term v0.48.0` and `x/text v0.44.0`.
- The epic says a team changes `go.mod` "only to add a module that its plan lists" and that the orchestrator compares the change with the plan's list (`.plans/epics/go-rewrite/epic.md`, section "The teams, and what each may edit").

**Failure scenario.** Adding `golang.org/x/net` raises `x/sys` from v0.29.0 to v0.50.0 and `x/text` from v0.41.0 to v0.44.0 and adds `x/term`. The orchestrator sees four changed lines that the plan does not list. Either the builder is sent back, or the comparison is waved through, which weakens the rule for later plans. The same bump reaches the Store's pgx build and the sprint 0 generators.

**Prescribed fix.** Add a second table to the module section with the indirect results of `go mod tidy` (`x/sys v0.50.0`, `x/term v0.48.0`, `x/text v0.44.0`) and require the Contract team's integration run to build and test `src/server/internal/store/...` after the bump.

### architect-r2 6. The session cache has no eviction rule and no sharing of work (minor, R7)

**Evidence.** `plan.md:171` bounds the cache at eight tuples and ties it to the `Sessions` instance. It says nothing about which entry goes when a ninth tuple arrives or how concurrent misses on one verifier behave. Item 2 derives a 64 MiB scrypt key per miss (`N=65536, r=8`).

**Failure scenario.** Eight sessions are cached. The oldest is evicted in the database and a ninth is issued. A cache that refuses to insert when full never caches the newest session, so every request from it derives a 64 MiB key. A browser that opens its watch streams and calls at the same moment with one cookie after a restart starts several derivations at once, each holding 64 MiB. The container has no stated memory limit in the Stack plans I read. `TestCheckRejectsForeignBinding` covers only altered rows.

**Prescribed fix.** Key the cache by verifier with one entry per verifier, replace the entry when the tuple differs, and drop the entry when a check finds no live row. Share one in-flight derivation per verifier. Add a test with nine issues and one hundred concurrent checks of the newest token that counts derivations.

## Answers to the attack questions

1. **Does each patch remove its failure?** Yes for the following, with the checks noted.
   - architect-r1 1 (health command): `plan.md:482` and `TestHealthCommand` give the exact spelling that `.plans/plans/go-s2-stack-web/plan.md` reads. Removed.
   - architect-r1 2 and critic-r1 1 (settings names): item 1's table matches the Stack environment keys and the Stack's `postgres-app.password` mode `0440` with owner 70 and group 10001. Removed.
   - architect-r1 3 and critic-r1 2 (public origin): removed for plaintext loopback. Not defined for an HTTPS public origin (finding 4).
   - architect-r1 4, logic-r1 1 and threat-r1 1 (two instances): removed in principle. The lock acquisition is racy and its monitor start is undefined (finding 3).
   - architect-r1 10, logic-r1 7 and threat-r1 8 (key creation): the read-after-create is correct. The policy request is unbacked (finding 1).
   - logic2-r1 4 (pin): the truth table in the answer to question 5 holds.
   - critic-r1 8, logic2-r1 8 and logic2-r1 9 (clock and budgets): in the text, not testable as declared (finding 2).
   - logic2-r1 5 (two HTTPS cookie names): `plan.md:246` separates them and tests cover both orders. Removed.
   - architect-r1 9 and critic-r1 3 (pins): direct pins are listed, indirect bumps are not (finding 5).
   - Others in the register (architect-r1 5 to 8 and 11, critic-r1 4 and 6 to 11, logic-r1 2 to 8, logic2-r1 1 to 3, 7, 9 to 11, threat-r1 2 to 7): no new defect found in the patched text.

2. **Attack of each `UNREVIEWED MECHANISM`.** There are seven markers at `plan.md:61, 171, 193, 244, 246, 296 and 441`, and further ones inside item 4's tests, item 8 and item 9.
   - The module table (`plan.md:61`): finding 5.
   - The session cache (`plan.md:171`): finding 6.
   - Exclusive admission (`plan.md:193`, `plan.md:474`): finding 3.
   - The aliases (`plan.md:244`): finding 4.
   - The three cookie names (`plan.md:246`): consistent, except for the alias case in finding 4.
   - The read limit of 32 KiB (`plan.md:296`): consistent with the 4096-byte password limit and the Connect library behavior the scouting file records.
   - The create-then-read path and the public pin (`plan.md:439-441`): finding 1 for the policy. The pin migration is owned by the vault part, writes through `Store.Write`, and keeps R3 because Machines and the token command call `LoadLinkIdentity` and never read the table.
   - The exact-microsecond boundary (`plan.md:177`): finding 2.

3. **Rejected findings.** The register has no finding with the outcome "rejected". Its outcomes are fixed, fixed another way, for the owner and post-build check. Nothing to check. The four "for the owner" items (logic2-r1 2, 3 and 6, critic-r1 5) are open decisions by design, and I did not argue against any owner decision.

4. **Writers and readers of changed names.** Complete for the following.
   - `Environment()`: written by item 9, read by the sprint 3 token command test (`.plans/plans/go-s3-stack-client/plan.md:146`).
   - `CheckInRead`, `ErrNoSession`, `SessionVerifier`: written by items 4 and 6, read by Machines (`.plans/plans/go-s3-server-machines/plan.md:999, 1120`). Names agree.
   - `AcquireServerInstance`, `InstanceLock` and `serverInstanceLockID`: Store reserves the identifier (`.plans/plans/go-s1-server-store/plan.md:61, 196`) and item 9 owns the file, both directions recorded. The value 841941100003 matches `src/server/internal/store/locks.go`.
   - `joinTokenCommand` and the command table: written by Stack client item 1, registered as exit 2 by item 9 (`plan.md:482`).
   - `LoadLinkIdentity(..., allowInitialize)`: listed with all consumers.
   - **Not complete:** the Stack request for the policy (finding 1), the seam names (finding 2) and the indirect module bumps (finding 5).

5. **Contradictions and truth tables.**
   - Pin and key, with `allowInitialize` as the third input: no pin, no entry, true gives create, read and pin; no pin, no entry, false fails before contacting OpenBao; pin present, entry absent fails; pin present, entry different fails; no pin, deleted entry with version metadata fails; no pin, canonical stored seed, true adopts it. All six rows follow from `plan.md:439-441`. No contradiction.
   - Startup with an unusable password: busy lock, database down and purge failure each print the reconciliation line and the unconfirmed-revocation line, and none prints a vault line, because step 4 ends before step 5. Stack's `--remove-password` greps "Session revocation committed." and the sentence at `plan.md:497` matches it including the full stop. No contradiction.
   - Startup budgets: 10 seconds plus 5 minutes plus 2 minutes is 430 seconds, below Stack's 540-second launcher wait and 8-minute start period (`.plans/plans/go-s2-stack-web/plan.md:312, 368`). No contradiction.
   - Origin and cookie modes: see the table in finding 4. Decision rows 11, 18 and 20 are an older-then-newer chain and row 11 should be marked.

6. **Round 1 blocker or major still open although marked fixed.** No blocker. Two majors are fixed in text but not provable as declared: logic2-r1 8 (budgets) and critic-r1 8 (exact boundary), both through finding 2. Threat-r1 8 is fixed only if Stack agrees (finding 1).

7. **Lens questions touched by the patches.** Answered below.

## Answers to the lens questions

1. **Claims about existing code.**
   - `store.NewDomainError` exists (`src/server/internal/store/pool.go:79`).
   - The `ConnectionPolicy` and private-stack host rule exist (`pool.go:35, 198`).
   - `storetest.NewDatabase(t, schema)` exists and handles `"public"` (`src/server/internal/store/storetest/postgres.go:23, 84`).
   - `Store.Migrate` uses `statement_timeout` of five minutes and `lock_timeout` of zero (`migrate.go:46`), as item 9 assumes.
   - Writes use a 30-second budget with a 10-second lock limit (`pool.go:23, 313`). Sign-in's table lock waits fall under that limit.
   - Not yet in code: `Outcome`, `RequestKeyHeader`, `Record`, watch, follower. They are in the Store plan (`plan.md:57, 143, 367`), and the sign-in plan depends on them at the stated names.
2. **Writers and readers of public names.** See answer 4 above.
3. **Placement.** Correct. Settings, auth, origin and cookies, the router, the Session service, the vault client and the pin table sit in the Server tree. `store/instance.go` sits in the Store folder but is written by sign-in item 9, which is allowed because Server may edit `src/server/` and Store's plan records both directions. Nothing is placed in the contract. Compose and the health spelling belong to Stack and are read by item 9.
4. **Rules R1 to R11.**
   - R1 (a part owns one folder, one contract, its migrations): kept; `auth` and `vault` own migrations. Item 9 writing into `store/` is the one crossing and is recorded.
   - R2 (contract first): kept; no contract change.
   - R3 (no foreign tables): kept; Machines calls `CheckInRead`, not the table.
   - R4 (every write through the Store and says what changed): kept; issue, revoke, reconcile and the pin record changes.
   - R5 (watch through the `watch` folder): not applicable here, with `CheckInRead` supplied as the authorize hook.
   - R6 (commands through the link): not applicable.
   - R7 (nothing in memory that cannot be read again): kept; the cache is rebuildable (finding 6 asks for a rule).
   - R8 (an error is a Connect code with one sentence): kept; Store's domain errors and the fixed sentences.
   - R9 (secrets never in logs, credit for copied code): kept; the diagnostics table and the log fields are fixed. The dependency list is incomplete (finding 5).
   - R10 (real PostgreSQL, real client and server): kept, but three tests lack declared seams (finding 2).
   - R11 (credit for third-party logic): not applicable; no third-party code is copied.
5. **Takes from and gives to other plans.** Takes from Store (`NewDomainError`, `Outcome`, five-minute migration budget, lock identifiers, `NewDatabase`) and from Stack (settings, health check, removal path, vault credential). Gives to Machines (`CheckInRead`, `ErrNoSession`, `SessionVerifier`, `servertest` additions listed at `.plans/plans/go-s3-server-machines/plan.md:1119`) and to Stack client (`Environment()`, the command table). The one take that is not agreed is the policy (finding 1).
6. **Can each item merge alone with a passing full check?** Items 2 to 5 and 8 can, with their stubs. Item 5 needs the `x/net` pin and item 2 the `x/crypto` pin, which the Contract team adds with the first importer (finding 5 for the indirect lines). Item 9 needs all others and Store's follower, pruner and fixtures. The migration order in `plan.md:476` is consistent with Store's manifests.
7. **Room for wave 2 and over-building.** Room: yes. One user, no rate limits and a single node are stated. The `servertest` surface is minimal and Machines extends it. Over-building: the session cache and the three-name cookie scheme are the two most elaborate pieces for one user. The cookie names answer a real browser conflict (logic2-r1 5) and stay. The cache is worth keeping only with the rule in finding 6, otherwise drop it and derive on each check, which at one user costs less than the cache's own risk.
8. **Dependencies.** Versions exist and are compatible with Go 1.26.0: `x/net v0.63.0` requires `x/crypto v0.59.0`. The indirect bumps are not listed (finding 5). The module proxy was unreachable in the planning sandbox, so the first importer's integration run is the real check.

## Post-build checks

- **PB-R2-A (from finding 3).** On the real stack run `deploy/up.sh --remove-password` followed by `deploy/up.sh` fifty times in a loop and stop and start the fixture in `servertest` one hundred times. Expect no "Another server is already using this database." line.
- **PB-R2-B (from finding 1).** After Stack's init, read the policy of the `lgtm-server` role and compare its capabilities on the signing-key path with the handoff. Expect exactly the agreed set.
- **PB-R2-C (from finding 4).** Start with `LGTM_PUBLIC_URL` set to an HTTPS origin and send a credentialed SignIn with a localhost origin on port 443. Expect refusal and no cookie.
- **PB-R2-D (from finding 6).** Issue nine sessions, then send one hundred concurrent calls with the newest cookie after a restart. Expect memory in the container to stay under the container's limit and one derivation for the burst.
- **Existing checks.** PB1 to PB7 stand. PB4 is restated as in finding 1.

## Grade

7 out of 10: **0 blockers, 2 majors, 4 minors.** The round 1 patches are sound in design. The remaining defects are a cross-plan assumption that nobody has accepted and test seams that nobody has named, and both can be fixed by writing a few sentences.
