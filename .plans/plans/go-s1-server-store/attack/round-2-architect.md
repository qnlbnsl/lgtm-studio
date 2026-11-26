REVISE

# Round 2 review, seat architect, plan go-s1-server-store

- Task identifier: review-go-s1-server-store-r2-architect
- Nonce: 675647
- Lens: architect
- Plan text reviewed: `.plans/plans/go-s1-server-store/plan.md`, 727 lines, at commit `1712f47c`, with `decisions.md` and `attack/round-1-findings.md`.
- Model and effort requested: claude-sonnet-5-5, high.
- Scope: the patches of round 1 and what follows from them. Unchanged text is not reviewed again.

## Top finding

Two of the three prerequisites that architect-r1 1 named are still unowned. The patch moved the hook `deploy/test/mutate-feed-lock.sh` to the Stack team in the Store plan only, and no Stack work item creates it (architect-r2 1). The patch also made the driver change to `go.mod` a joint edit of the Contract and Server teams, which the epic's edit table does not allow (architect-r2 2).

## Findings

### architect-r2 1. MAJOR. The check C12 hook still has no work item that creates it

- Tied to: C12, R10, plan.
- Round-1 finding it answers: architect-r1 1, which the register marks "fixed another way".
- Evidence:
  - `plan.md:48` says Stack owns the thin executable `deploy/test/mutate-feed-lock.sh` and `plan.md:339` repeats it.
  - `.plans/plans/go-s1-stack-vault/plan.md:71` and `:91-93` give Stack item 8 three files: `vault-testlib.sh`, `with-postgres.sh` and `vault-fixtures.sh`. A search of that plan for `mutate` and `check-feed` finds nothing.
  - `deploy/test/checks.sh:11` sets `mutation_hook=deploy/test/mutate-feed-lock.sh`. `check_c12` at `:590-602` fails when the file is missing or not executable, runs `go test -count=1 ./...`, then calls the hook with no arguments and discards its output.
  - `decisions.md` row 19 records the wrapper as an assumption with status `unanswered`. The Store plan's own sentence "The orchestrator tracks these handoffs" assigns a person, not a work item.
- Failure scenario: Server item 4 merges with `check-feed-lock.sh` passing. At wave 1 verification C12 reports that the executable is missing, although every Go test passes. Nobody on the Stack team has a plan line that tells them to write it, and the Stack plan is reviewed on its own schedule.
- Fix:
  - Add the wrapper to Stack item 8 in the Stack plan. State its contract: it takes no arguments, runs `src/server/internal/store/check-feed-lock.sh` from the repository root with the caller's environment, passes the exit status through, and is committed with the executable bit.
  - Alternatively, give the Server team that one file in the epic's edit table and delete the Stack hop.
  - Either way, add the file to the epic's "What the teams must agree on" list, with its writer and its reader (`checks.sh`), and close row 19.

### architect-r2 2. MAJOR. The driver change needs two teams in one commit, and the plan disagrees with itself about the sprint

- Tied to: R10, plan (one writer per file).
- Round-1 findings it answers: architect-r1 1, critic-r1 5.
- Evidence:
  - `plan.md:46` says the Contract team owns the `pgx/v5 v5.13.0` change to `go.mod` and `go.sum` in sprint 1, and that the Server team "lands the first driver import in the same integration change" so the tidy check stays green.
  - `.plans/epics/go-rewrite/epic.md:122` lets the Contract team edit `go.mod` and `go.sum`, and every other team edit only its own folder.
  - `plan.md:85` says item 1 "needs the root module and the driver's dependency entries from the Contract team". `plan.md:685` says the dependency is "Sprint 0 module", with the driver joining "in sprint 1".
  - `.plans/plans/go-s0-contract/plan.md:161` lists only `connect` and `protobuf` in `go.mod`. `scripts/check-go.sh` runs `go mod tidy -diff` (`:485-493`).
- Failure scenario:
  - If the Contract entry lands first, `go mod tidy -diff` removes the unused requirement and sprint 0's own check is red.
  - If the Server import lands first, `go build` cannot resolve the package.
  - A commit that touches both `go.mod` and `src/server/` breaks the epic's rule, so one team must break it. The tests-first step of item 1 cannot even compile its first test.
- Fix: choose one owner and say so in both plans. The simplest way is an epic exception: the Server team may add and remove its own `require` lines and the matching `go.sum` lines, and the Contract team keeps the module path, the `go` directive and the generated-code dependencies. State that item 1 merges `go.mod`, `go.sum` and the first import together. Remove "from the Contract team" from `plan.md:85` and make `plan.md:685` say "sprint 1" the same way as `plan.md:46`. Record that `govulncheck` runs at the merge, because v5.13.0 is a late-2024 release.

### architect-r2 3. MAJOR. Once item 1 merges, the shared Go check is red on every machine without a database

- Tied to: C12, R10, plan (each item merges alone with a passing full check).
- Round-1 finding it touches: architect-r1 1 (the test role) and critic-r1 8.
- Evidence:
  - `plan.md:121` and `decisions.md` row 8 make an unconfigured integration run fail with "Set LGTM_TEST_POSTGRES to a disposable PostgreSQL maintenance database."
  - `.plans/plans/go-s0-contract/plan.md:485-493` runs `go test ./...` and only "passes `LGTM_TEST_POSTGRES` on as it finds it". Its acceptance requires `scripts/check-go.sh` to exit 0 again after item 6.
  - The client plan (`.plans/plans/go-s1-client-local/plan.md:129`) builds in the same module and runs the same command.
  - `plan.md:47` says Stack's `with-postgres.sh -- <command>` supplies the variable, but no plan wraps `scripts/check-go.sh` with it.
- Failure scenario: item 1 merges. A Client team builder runs `scripts/check-go.sh` or `go test ./...` before their own merge. The store package fails with the setup message, and the client's check is red for a reason in another team's code. Every later item of every Go team inherits the failure until someone adds a wrapper, and nobody is assigned.
- Fix: decide one rule and write it in the Store plan and the epic's agreement list.
  - Option A: Store tests skip with an explicit message when `LGTM_TEST_POSTGRES` is unset, and `check_c12` (which already requires the variable) is where the real run is forced. This keeps R10 at the check that matters.
  - Option B: `scripts/check-go.sh` starts `deploy/test/with-postgres.sh --` itself when the variable is unset. This needs a Contract edit and a Docker host everywhere.
  - Option A is the smaller change.

### architect-r2 4. MAJOR. Three tests are written in items that cannot run them

- Tied to: C12, plan (build order and merging alone).
- Round-1 findings it touches: critic-r1 10, architect-r1 7, logic2-r1 1.
- Evidence:
  - `plan.md:337` puts `TestFeedChecksDeferredDomainConstraintsBeforeLock` in item 4 and ends it with "one keyed caller retry succeeds in the merged receipt item". `WriteOnce` is item 5, which needs item 4 (`plan.md:343`).
  - `plan.md:412` (`TestPruneStartsAgeAfterCommit`) and `plan.md:413` (`TestPruneWaitsForReplayOfSameKey`) end with "in the merged build retry that key" and "the public replay path in both lock orders". Item 6 declares only items 1 and 3 as its needs and says it can run beside items 4 and 5 (`plan.md:388`).
  - `plan.md:414` uses "a gated failed sweep". The gate helpers belong to item 4 (`plan.md:268`, `:324`), and item 6 does not list item 4.
- Failure scenario: the builder of item 4 or item 6 cannot make the named test pass without code from item 5, or writes a private substitute. The reviewer cannot tell whether a green report covers the receipt half. If item 6 merges before item 5, its acceptance text cannot be observed at all.
- Fix: move the three receipt-dependent endings into `receipts_test.go` as named item 5 tests (for example `TestWriteOnceRetriesAfterDeadlock` and `TestWriteOnceSurvivesPruneOnBothSidesOfCommit`). Keep the item 4 and item 6 tests to what their own code can observe. Give item 6 its own raw advisory hold for the failed sweep, or add item 4 to its needs.

### architect-r2 5. MAJOR. Reads, the follower, the pruner and migrations share three reserved slots with no budget, and the stale-feed cutoff turns that into a loop

- Tied to: D4, D5, R5.
- Round-1 findings it answers: logic-r1 2, threat-r1 3, critic-r1 1, logic-r1 4.
- Evidence:
  - `plan.md:113` admits at most `MaxConns - 3` writers and says "`Read` and the follower can use the reserved capacity", with one slot for migrations and one maintenance worker. `Read` has no admission limit. The minimum pool is eight, so five writers can hold connections and three remain for everything else. The listener uses a separate connection.
  - `plan.md:474` gives each follower page read a two-second budget for acquisition and queries.
  - `plan.md:533` ends every stream with `ErrFeedUnavailable` when no page succeeds for ten seconds.
  - `plan.md:132` proves only that one `Read` completes while writers wait.
- Failure scenario:
  - A lag cutoff or a restart makes many clients reopen at once. Each `Serve` holds a repeatable-read snapshot connection while it loads, and each message and 30-second recheck opens another read.
  - The pruner takes one reserved slot. Page reads queue behind the snapshots, fail their two-second budget for ten seconds, and every open stream ends.
  - Every client then reopens, which repeats the load. The follower's progress is the thing the rest of the design depends on, and no test protects it.
- Fix:
  - Give the follower's page reads a connection of their own, as the listener already has, or reserve one pool slot for them with a separate admission count.
  - Cap concurrent `Read` and `Snapshot` transactions at `MaxConns - 5` (one slot for the follower, one for maintenance, three for writer headroom).
  - Add `TestFollowerProgressesUnderReadSaturation`. Hold the maximum number of snapshots open, commit a feed row, and require a page to apply within two seconds. Removing the dedicated slot must fail it.

### architect-r2 6. MINOR. The mutation script has no stable target, and only one of the three mutations runs from it

- Tied to: C12.
- Round-1 findings it answers: logic-r1 1, logic-r1 3, architect-r1 1.
- Evidence:
  - `plan.md:339` says `check-feed-lock.sh` "removes exactly the batch statement that acquires the feed lock" from an archived copy of the committed checkout, and that it "exits successfully only for the expected early-visibility assertion". `plan.md:428` describes the same mutation as a manual removal of "the production lock call" on a disposable branch.
  - The statement lives in a Go source line at `plan.md:310`. No marker, no search pattern and no count of matches is defined.
  - `plan.md:330` and `:336` require the lock-after-insert mutant and the session-lock mutant to fail, but no executable runs them. `check_c12` runs the hook once with no arguments.
  - `check_c12` runs `go test ./...` on the working tree, while the script mutates `git archive` of the commit.
- Failure scenario: a later edit renames the batch line or formats it differently. The pattern matches nothing, the mutated copy is identical to the baseline and passes, and the script's "exit successfully only on the expected failure" rule makes C12 fail for a reason that has nothing to do with the lock. The other two mutants are never run by anyone.
- Fix:
  - Put one comment marker on the lock line (for example `// lgtm:feed-lock`). The script deletes the marked line, fails if it finds the marker other than once, and matches the exact text `writer B became visible before writer A` in the test output.
  - Let the script take an optional mutation name and run all three in item 4's acceptance, with the wrapper using the default.
  - State that the script tests the committed tree and that the report names the commit.

### architect-r2 7. MINOR. The new timeout defaults do not say how `Migrate` and the pruner use them

- Tied to: D3, plan.
- Round-1 findings it answers: logic-r1 2, critic-r1 8.
- Evidence:
  - `plan.md:113` installs `statement_timeout=15s` on every connection. `plan.md:117` sets a ten-second `lock_timeout` and a 30-second operation limit in `withWrite`.
  - `plan.md:166` runs `Migrate` in one transaction with whole-file simple-query execution. It does not say whether `withWrite` and its limits apply. The tests at `plan.md:226-229` use short fixtures.
- Failure scenario: a later part ships a migration that builds an index on a table with data. It takes 20 seconds, hits the 15-second statement limit, and the server refuses to start at every restart. A second server instance that waits for the first one's migration hits the ten-second lock wait and fails the same way.
- Fix: say that `Migrate` uses its own bounds (for example a five-minute operation limit and a lock wait equal to it, with `statement_timeout` set to the same value inside the transaction), and add one test with a migration file that sleeps past 15 seconds and still succeeds.

### architect-r2 8. MINOR. The readers of the names that round 1 added are not in their own plans

- Tied to: R2, plan (what the plan gives to other plans).
- Round-1 findings it answers: architect-r1 4, critic-r1 5, threat-r1 1, threat-r1 9.
- Evidence:
  - The Store plan says the sprint 2 and 3 adapters read `Request`, `WriteOnce`, the fingerprint encoders, `Source.Authorize` with the session read function, `Retryable()`, `ErrFeedUnavailable`, `ConnectionPolicy` and `MaxWriteChanges` (`plan.md:115`, `:119`, `:368`, `:533`, `:589`).
  - A search of `.plans/plans/go-s2-server-signin/` and `.plans/plans/go-s3-server-machines/` finds none of these names. The sign-in plan's decision 7 refuses a request key for `SignIn`, which is one use, not the contract. The epic's agreement list does not name them.
- Failure scenario: sprint 3 builds the Machines `Watch` without the 30-second recheck, because its plan never says the session function must be passed as `Authorize`. The failure appears at sprint 3's review, not here.
- Fix: add one row per name to the epic's "What the teams must agree on" list with its writer and its readers: `Request` and `WriteOnce` (readers: sprint 2 and 3 adapters), `Source.Authorize` (writer: sprint 2 session read, reader: sprint 3), `ConnectionPolicy` (writer: sprint 2 bootstrap), `Retryable` and `ErrFeedUnavailable` (the adapters' status mapping). Ask the orchestrator to compare at rollout step 3.

### architect-r2 9. MINOR. Decision row 16 still names a code that the plan no longer uses

- Tied to: R8, R2.
- Round-1 finding it answers: architect-r1 2.
- Evidence: `decisions.md` row 16 says lag maps to `resource_exhausted`. Row 25 says "This refines rows 15 and 16" and names `aborted`. `plan.md:593` and `.plans/plans/go-s0-contract/draft/common.proto:17` say `aborted`. Rows 7 to 29 are all `unanswered`.
- Failure scenario: a reader who follows the decision register reaches row 16 first and builds the wrong code in the Machines adapter.
- Fix: mark row 16's code as superseded by row 25, using the same wording as row 6 for row 5. Put the rows that gate other plans (7, 8, 19, 20) in front of the owner's list, so the builder is not told to assume them silently.

### architect-r2 10. MINOR. The receipt lock number is a literal in the SQL, and the registry has a second hole

- Tied to: R10, wave 2 room.
- Round-1 finding it answers: architect-r1 9.
- Evidence: `plan.md:168` defines `receiptLockNamespace int32 = 1279743053` in `locks.go`. `plan.md:372` writes the same number as a literal inside the SQL string. The test gate id `841941100901` appears only inside the trigger body at `plan.md:326`, outside the registry.
- Failure scenario: someone changes the constant, the SQL keeps the old number, and the receipt lock and the registry disagree. The test would still pass, because it checks only the two-integer space.
- Fix: pass the namespace as a parameter (`pg_advisory_xact_lock($1::int4, hashtext(...))`), list `841941100901` in `locks.go` as a test-only id, and add one assertion in `TestWriteOnceConcurrentRetry` that the observed `classid` equals the constant.

## Answers to the questions of the task

1. **Each patch.** The register's 49 patches remove their failure as written, except where findings 1 to 5 above apply. The guard trigger, the 256-change cap, the ordered batch, the ledger-before-lock mutation, the head-regression reset and the aborted mapping each remove their failure. Findings 1 and 2 leave architect-r1 1 open. Finding 5 weakens logic-r1 4's fix under load.
2. **`UNREVIEWED MECHANISM` places.**
   - Driver ownership: finding 2.
   - `ConnectionPolicy`: no architect-level defect. Its writer is the sprint 2 bootstrap, which no plan names (finding 8).
   - Sanitized `Error`: the readers are not in the consumer plans (finding 8).
   - Fixture cleanup: sound, because a killed run discards its whole instance.
   - Migrated fixture subpackage: sound, there is no import cycle (`migrated` imports `store` and `storetest`, and neither imports `migrated`).
   - Ordered batch: sound under `SET CONSTRAINTS ALL IMMEDIATE`, with the test trigger re-deferred by the `afterConstraints` hook.
   - Lock-timeout retry boundary: sound for writers, but `Migrate` is unspecified (finding 7).
   - Semantic fingerprint versions and receipt framing: the Store side is complete. The adapter plans do not mention them (finding 8).
   - Retention first-observation clock: sound.
   - Lag at `Start`: sound.
   - Vendor counts: not an architecture question.
3. **Rejected findings.** The register has no finding with the outcome "rejected". One finding is marked "for the owner" (threat-r1 2), and its text is consistent with sign-in decision 7.
4. **Writers and readers.** Complete on the Store side, for `feed`, `feed_retention`, `receipts`, `schema_migrations`, the lock ids and the error sentinels. Incomplete on the consumer side (finding 8). Neither the lock literal nor the test gate id has a registry entry (finding 10).
5. **Contradictions.** Findings 2 (sprint 0 versus sprint 1), 4 (item order against test text), 6 (manual versus script mutation) and 9 (row 16 versus plan). Truth table for the admission rules at `MaxConns = 8`: writers 5 maximum; reserved 3 less pruner 1 less follower 1 leaves 1 for all reads. At `MaxConns = 8` five writers waiting on a gate leave one slot for every `Read`. The plan tests only one `Read` (finding 5).
6. **Open blockers or majors.** None is a blocker. Architect-r1 1 is still open (findings 1 to 3). Critic-r1 5 is still open for the driver (finding 2). Architect-r1 3 is closed. A callback can still set `lgtm.feed_flush` itself, and the plan says the guard is for accidents (`plan.md:305`). This matches the design's trusted-code assumption and is not a finding.
7. **Lens questions the patches touch.** See below.

## Answers to the lens questions

1. **Claims about existing code.** The new claims about code are the two about `checks.sh` and `check-go.sh`. Both match the files, and both show the gaps in findings 1 and 3. `go.mod`, `src/server/` and the s0 `common.proto` shape are as described. `common.proto` still says `aborted`, `uint64`, `all`, `saved` and `removed_id`, although the draft files have uncommitted edits in the working tree.
2. **Writers and readers.** See answer 4 above.
3. **Placement.** The hook and the driver change are the two misplaced items (findings 1 and 2). The store, follower, `watch` helper and fixtures are in the right folders.
4. **Rules R1 to R11.**
   - R1: the ledger moved into `Migrate`. Kept.
   - R2: the mapping to `all`, `saved` and `removed_id` is written and checked against the draft. Kept.
   - R3: kept.
   - R4: the guard is a trigger, with the accidental-use limit stated. Kept.
   - R5: kept, with finding 5 as a risk to the follower.
   - R6: does not apply.
   - R7: the head regression and stale-feed rules close the restore case.
   - R8: kept (finding 9 is a document fix).
   - R9: kept.
   - R10: weakened for developers by finding 3.
   - R11: kept.
5. **What the plan takes from and gives to other plans.** It takes the driver, the test role and the hook (findings 1 to 3). It gives the names in finding 8. The sign-in plan's `SignIn` key refusal fits the Store's open credential choice.
6. **Merging alone with a passing full check.** Items 2 and 3 merge alone once item 1 does. Item 1 fails the shared check without a database (finding 3). Items 4 and 6 contain tests that need item 5 (finding 4).
7. **Room for wave 2.** Enough. The manifest, `Record`, `Serve` and the key space serve the later parts. The new limits (256 changes, 65,536 bytes of reply, five writers at the minimum pool) are enough for the machine registry and sign-in.
8. **Dependency versions.** One new dependency, `pgx/v5 v5.13.0`, with an exact pin. The surface the plan uses (`pgxpool`, `pgx.Batch`, `pgx.Identifier.Sanitize`, `pgconn` notifications and the batch tracer) exists in the v5 line. A live build under Go 1.26 was not possible here, because the installed toolchain is 1.23.3 and neither `go.mod` nor `src/server/` exists yet. Finding 2 asks for `govulncheck` at the merge.

## Post-build checks

| # | Scenario | Expected result |
|---|---|---|
| 1 | Run `deploy/test/checks.sh --only C12` through `deploy/test/with-postgres.sh --`. | The wrapper exists and is executable, the store tests pass, the hook runs the Server script, and C12 reports pass. |
| 2 | Change the marked lock line so it no longer matches, then run the hook. | The script fails and names the missing marker. It does not report a pass. |
| 3 | In a clean checkout with the item 1 commit only, run `go build ./...` and `go mod tidy -diff`. | Both pass, and `git log` shows one commit that touches `go.mod`, `go.sum` and the first import together. |
| 4 | On a machine with no `LGTM_TEST_POSTGRES`, run `scripts/check-go.sh`. | It follows the rule chosen for finding 3, and the client and contract packages pass. |
| 5 | Merge item 6 before item 5 in a scratch branch. | Every test of item 6 still passes, and the receipt endings are absent from it. |
| 6 | Hold the maximum number of `Read` snapshots open on an eight-connection store, then commit one feed row. | The follower applies the row within two seconds and no stream ends with `ErrFeedUnavailable`. |
| 7 | Run a migration file that sleeps 20 seconds. | `Migrate` succeeds and records the version. |
| 8 | Read the epic's agreement list. | It names the writer and the readers of each name in finding 8. |

## Grade

6 out of 10. Blockers 0, majors 5, minors 5.
