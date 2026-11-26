# Round 2 review of go-s1-stack-vault, seat logic

REVISE

Top finding: the test that is meant to guard against the round 1 blocker, a rename across mounts, cannot fail on that mutation, because the fixture `mv` shim only signals before and after the move and never observes a non-atomic copy.

- Task identifier: review-go-s1-stack-vault-r2-logic
- Nonce: 6a039c
- Reviewer model and effort: claude-sonnet-5-5, high
- Scope: the patches between commit 25d3132e and commit cd46223f of `plan.md` and `decisions.md`, and what follows from them.
- Method: I read the patched plan, the round 1 register, the design, the existing `deploy/test/checks.sh`, and the Server plan that consumes the test database. I also checked two upstream facts. The PostgreSQL image entrypoint exports `PGPASSWORD` for its init files because `--auth-local` with a password method is a supported configuration. OpenBao's PostgreSQL storage documents `max_connect_retries=0` as unlimited retries. I changed no file other than this report.

## Findings

### logic-r2 1. MAJOR. The cross-mount rename mutation does not fail any listed test

- Tied statement: D10 (OpenBao beside PostgreSQL), through the round 1 blocker `logic2-r1 1`.
- Evidence: `plan.md:275` requires the hidden file and the final file to share one mount. `plan.md:288` describes the crash tests: a fixture-only `mv` executable "delegates normal renames and sends a fixture-ready signal before or after the rename". `plan.md:296` states that using "a cross-mount rename" as a mutation "must fail the corresponding assertion". The same sentence names no assertion for it.
- Failure scenario: a builder stages the hidden file in `/state/admin` and moves it to `/state/server/openbao.json`. The two directories are separate bind mounts, as `plan.md:258` and `plan.md:288` require. `rename(2)` returns the cross-device error, and `mv` silently falls back to copy and delete. The shim delegates that call and signals only before it starts or after it ends. A kill at either signal sees an empty destination or a complete file, so every listed assertion passes. In production a crash during the copy leaves a truncated `openbao.json` that the table at `plan.md:271` and `plan.md:272` reads as a malformed credential, and the server cannot sign in. The only blocker of round 1 therefore has no working regression guard.
- Prescribed fix: make the shim reject any call whose source directory and destination directory report different device numbers, and exit with a failing status that the test reads. Alternatively, have the shim perform only `rename(2)` and fail on the cross-device error, with no copy fallback. Name the assertion that owns this mutation. `bootstrap_resumes_at_publication_boundary` is the natural owner.

### logic-r2 2. MAJOR. An interrupted reset cannot be finished with the command the plan gives

- Tied statement: D10, with R9 (secrets out of logs) for the leftover credentials.
- Evidence: `plan.md:326` orders the reset as stop the daemons, remove the verified PostgreSQL volume, then remove local state contents. The same text says "The reset verifies the binding". `plan.md:304` makes the binding live in the volume (`lgtm-installation`) and in `installation.id`. It also says a missing volume with existing claimed state returns `state directory is ahead of the database`. `plan.md:323` answers that diagnosis with "restore the matching local database and state pair" or `rotate-server-credential`. The recovery table at `plan.md:319` to `plan.md:324` offers reset only for an unusable response.
- Failure scenario: the operator runs `deploy/up.sh --reset --yes`. The volume is removed. The host loses power before `stack-prepare reset-state` runs. The next `deploy/up.sh` prints `state directory is ahead of the database`. The operator reruns the reset. The volume that carried the binding is gone, so "verifies the binding" has nothing to compare, and the plan does not say whether the reset then proceeds or refuses. The recovery table sends the operator to restore a database that was deleted on purpose. The truth table for the reset has an empty cell: volume absent, local state committed, no reset in progress recorded.
- Prescribed fix: write a `reset.pending` marker, holding the verified binding, in the state root before the volume is removed. Make `up.sh` print `reset in progress; run deploy/up.sh --reset --yes to finish` when the marker exists. Let the reset skip volume verification when the marker matches the local `installation.id`. Add a fifth row to the recovery table and a seventh assertion that kills the reset between the two removals. Also say what is locked. If the lock is a file inside the state root, then `reset-state` unlinks it while a waiting launcher holds the old inode and a third launcher creates a new one, which gives two holders. Lock the directory itself, or keep the lock file outside the state contents.

### logic-r2 3. MAJOR. A binding mismatch has no recovery path, and the path fingerprint creates false mismatches

- Tied statement: D10.
- Evidence: `plan.md:304` binds "the project, a random installation identifier and a fingerprint of the canonical state path". It returns `Compose project belongs to another state directory` to a losing claimant. `plan.md:326` states "A mismatch refuses reset against the foreign installation". The recovery table at `plan.md:319` to `plan.md:324` has no row for this diagnostic. `plan.md:326` also warns that a checkout cleanup may remove ignored state, and `plan.md:137` puts the default state directory inside the checkout.
- Failure scenario 1: the operator renames or moves the checkout, or opens the same checkout through a symlinked path. The default state directory moves with it, `installation.id` is intact, and the volume link is intact. Only the canonical path fingerprint changed. The launcher reports a foreign state directory, and the reset refuses because of the mismatch. The only exit is hand-editing a file inside a Docker volume.
- Failure scenario 2: the losing claimant keeps a provisional `installation.id`. Its project is held by another root, so the identity can never be committed or reset. The operator must delete the directory by hand, and the plan does not say so.
- Prescribed fix: let the random identifier be the proof of identity, and keep the path fingerprint for diagnostics only. When the identifier matches and only the path differs, under the lock, rewrite the volume's path fingerprint and continue. Add a recovery row that says to set a different `COMPOSE_PROJECT_NAME` or to point at the original state directory. Say that a loser deletes its own state root, and have the launcher remove a provisional identity when its volume claim is refused, because the loser never committed it. Add an assertion for the moved-checkout case.

### logic-r2 4. MAJOR. A half-finished credential rotation is invisible to `up.sh`, and the two writers of `/state/server` disagree about leftovers

- Tied statement: D9 (vault for server secrets).
- Evidence: `plan.md:344` keeps the prior identifier active when recreation fails and "resume[s] with the durable new file and journal". Resuming is an operator rerun of the same command. `plan.md:271` says that for a published valid file `up.sh` authenticates once, "remove[s] leftover staging" and prints `vault already configured`. Both flows stage hidden files in `/state/server` (`plan.md:275`, `plan.md:344`). `plan.md:344` says "Journal the candidate and superseded accessor" without saying where the journal lives or whether the candidate secret is in it. `plan.md:350` tests only the interruption after publication.
- Failure scenario 1: the operator rotates because the old secret leaked. The rotation publishes the new file, and the container recreation fails. The old accessor stays valid. A later `deploy/up.sh` validates the new file, prints `vault already configured` and exits zero. Nothing says that the rotation is unfinished, so the leaked secret stays valid for as long as the operator does not rerun the command. The failure permits access and states no repair signal.
- Failure scenario 2: the rotation registers the candidate and stages the hidden file, then crashes before the rename. `up.sh` runs the bootstrap row for a published file and deletes the "leftover staging". The candidate secret existed only in that file. The resume has the journal and no secret, so it registers a second candidate. The first becomes an orphan accessor that nobody holds. The rerun cleans that up, so the harm is bounded, but the two rules overlap and the plan names no owner of the staging files.
- Prescribed fix: give rotation its own journal file name and a fixed location in `admin/`, and say the journal holds the candidate's accessor and never its secret. Have the bootstrap cleanup delete only files it created. Have `up.sh` print `credential rotation is pending` and exit nonzero while the journal exists. Add an assertion that kills the rotation before the rename and one that checks the `up.sh` message.

### logic-r2 5. MINOR. Several validity predicates that decide the refusal rows are undefined

- Tied statement: D10, with C13 (OpenBao reopens on restart).
- Evidence: `plan.md:167` says the preparer "preserves an existing valid file", and it never defines valid. `plan.md:266` and `plan.md:267` branch on a "complete private response". `plan.md:268` requires "trusted root material". `plan.md:242` calls `bootstrap.state` "versioned" and gives no behavior for a version it does not know. The tests at `plan.md:174` to `plan.md:176` use only a 32-byte file of the right mode, and `plan.md:251` drops the response entirely.
- Failure scenario: a crash leaves a truncated response file after the server committed initialization. An implementation that tests only for the file's existence passes both listed tests, then fails on a JSON parse error with no diagnostic, and fails the same way on every rerun. The same gap exists for a zero-length or wrong-size key with evidence of an existing vault. The refusal wording `seal key missing for existing vault` is wrong for a key that is present and malformed. A file with the right bytes and the wrong mode is either repaired or refused, and the plan does not say which.
- Prescribed fix: define each predicate in one line. A valid key is a regular file of exactly 32 bytes with owner 10002 and mode 0400. A complete response parses as JSON and has nonempty `recovery_keys_base64[0]` and `root_token`. Trusted root material is a token that a lookup accepts, or a checkpoint at which a revoked token is expected. Add a short truth table with the columns key (absent, malformed, valid) and evidence (none, present), and add one test for a truncated response. State that a checkpoint with an unknown version refuses.

### logic-r2 6. MINOR. The lifetime of the recovery material contradicts itself

- Tied statement: D10.
- Evidence: `plan.md:275` says to keep "recovery material" until the `published` checkpoint is durable and to "Remove transient material afterwards". `plan.md:345` says to keep the operator file protected and speaks of a "lost operator identity and unusable matching state". `plan.md:293` compares "key and credential fingerprints" across a rerun, which only works if the recovery key remains. `plan.md:267` and `plan.md:273` refuse on "a checkpoint whose required material is missing", with no list of the required material per checkpoint.
- Failure scenario: one builder deletes `admin/recovery.key` after publication, and another keeps it. If a completed rerun then requires it, the deleting builder's installation prints `vault is initialized but bootstrap state is incomplete` on every start. If the check does not require it, the key is gone and the idempotence test has nothing to compare.
- Prescribed fix: list required material per checkpoint in the 5b table (for example `published` requires `recovery.key`, `operator.json` and the final file, and does not require `root.token`). State that `root.token` is transient and that `recovery.key` is kept.

### logic-r2 7. MINOR. The readiness test can pass an implementation that probes only one role

- Tied statement: D4 (PostgreSQL), with R10 (real PostgreSQL in tests).
- Evidence: `plan.md:142` requires an authenticated `SELECT 1` "for both role files". `plan.md:156` changes "one mounted fixture password" and requires health to fail.
- Failure scenario: the builder probes only `lgtm_app`. The fixture changes the application password, health fails, and the test passes. A broken `lgtm_bao` login then reports healthy, and OpenBao fails to start with a misleading storage error. The mutation list at `plan.md:160` does not name this case.
- Prescribed fix: run the assertion once per role file and require health to fail each time. Add "probe only one role" to the mutations.

### logic-r2 8. MINOR. The identity and capabilities of the reconciler are not stated

- Tied statement: D4.
- Evidence: `plan.md:147` sets `postgres-admin.password` to user 70, group 70 and mode 0400, and states that PostgreSQL and the reconciler read it. `plan.md:143` drops all capabilities and adds back only "file setup and user-switch capabilities". `plan.md:140` runs `reconcile-db` as the authenticated administrator but names no container user.
- Failure scenario: the reconciler runs as root with no capability to override file permissions. It cannot open a 0400 file owned by user 70, and the first clean run fails. Granting the override capability to fix it widens the privilege that `plan.md:143` limits. Running as user 70 fixes it with no capability.
- Prescribed fix: state that the reconciler runs as user 70, and add the user to the capability inspection in `stack_services_have_no_egress_and_no_net_raw`.

### logic-r2 9. MINOR. Item 2 and item 3 depend on item 6's binding protocol, and no test covers that dependence before item 6

- Tied statement: D10.
- Evidence: `plan.md:138` says `prepare-db` "claims the installation using item 6's binding protocol". `plan.md:166` has the seal step create "the empty private mount directories described in item 6". `plan.md:304` assigns the volume link to "the prepare helper". The acceptance sets at `plan.md:160` and `plan.md:181` exercise no claim.
- Failure scenario: the builder of item 2 writes `prepare-db` without any claim, because nothing in item 2 asserts one. Item 6 then has to change a merged function on a different team's schedule, and its builder discovers that item 2's fixture overrides assume an unclaimed volume.
- Prescribed fix: split the claim out as a named function in item 2 with its own two-line assertion (an unclaimed volume is claimed, and a claimed volume is refused). Item 6 then wires it into the launcher. Alternatively, move the claim text into item 2 and have item 6 reference it.

### logic-r2 10. MINOR. The rotation test covers one of four interruption points

- Tied statement: D9.
- Evidence: `plan.md:344` lists journal, register and verify, publish, recreate and destroy. `plan.md:350` interrupts only after publication.
- Failure scenario: an implementation that creates a new candidate on every run passes the listed test when the test does not count accessors. A crash between registration and publication then leaves two registered candidates for one rotation, and the superseded accessor is never recorded.
- Prescribed fix: add interruptions after the journal write, after registration and before the rename. Require one registered candidate per rotation at the end of each.

### logic-r2 11. MINOR. The setup budget has no start, no clock and a misleading timeout result

- Tied statement: D10.
- Evidence: `plan.md:246` waits on a 503 "within the same 60-second elapsed-time budget" and sets request timeouts to "the time remaining". `plan.md:312` separately gives Compose `--wait-timeout 60`. `plan.md:273` sends "Any other conflicting state" to `vault is initialized but bootstrap state is incomplete`.
- Failure scenario: a slow PostgreSQL leaves OpenBao sealed for 70 seconds. The wait expires, and the state falls into the last row. The operator is told that the bootstrap state is incomplete and is pointed toward reset, although only the wait failed. A wall-clock budget also changes when the clock is stepped.
- Prescribed fix: add a row for a timeout, `OpenBao did not become active`, which keeps all state and exits with a retryable status. Say that the budget starts when the job starts and uses a monotonic clock.

## Answers to the numbered questions

### Task questions

1. Patch by patch, each removes the failure it describes except where a finding above says otherwise. I checked the intent checkpoint and the response-file order, the seal guard with its journal and credential evidence, SCRAM enforcement at initialization, the volume claim, the reset, the operator role and rotation, the fixtures and the 60-second wait. The cross-mount blocker is fixed in text but has no working guard (finding 1). The reset and the binding add new empty cells (findings 2 and 3). I found no patch that breaks an unchanged statement, apart from the contradictions listed in findings 4 and 6.
2. `UNREVIEWED MECHANISM`, one by one:
   - `with-postgres.sh` (`plan.md:93`): nothing found. I checked the zero value (an empty command after `--`), a failing child, the loopback publication and cleanup. A kill of the wrapper itself would leave its container behind, and the plan's `cleanup when the child fails` covers only a failing child. That is minor and not filed.
   - SCRAM at initialization (`plan.md:139`): nothing found. The upstream entrypoint exports `PGPASSWORD` for its init files for the supported `--auth-local` setting, so initialization with a password method works.
   - Operator role (`plan.md:223`): the paths cover the rotation calls. Editing the server policy lets the operator read any secret by transitivity. The plan states this openly, and it is allowed by the single-owner decision, so I did not file it. The resume at `plan.md:344` needs a lookup by accessor, which the listed paths do not include, and I put that in finding 10's fix.
   - Secret identifier lookup (`plan.md:224`): nothing found beyond the injected failures that the test already lists.
   - Publication rename (`plan.md:275`): finding 1.
   - Binding protocol (`plan.md:304`): findings 3 and 9.
   - Reset (`plan.md:326`): finding 2.
   - Policy application (`plan.md:343`): nothing found. It replaces one named policy, and a completed bootstrap preserves the installed one, so the two do not conflict.
3. The register lists no finding as rejected. The only unfolded finding is `critic-r1 12`, a style point, and it rests on no failure. Nothing found.
4. Writers and readers:
   - `LGTM_TEST_POSTGRES`: writer is `with-postgres.sh`. Readers are `deploy/test/checks.sh:596`, `deploy/test/README.md:24` and the Server fixture at `go-s1-server-store/plan.md:121`. The list is complete.
   - The sprint 2 names `LGTM_POSTGRES_PASSWORD_FILE`, `LGTM_OPENBAO_CREDENTIAL_FILE` and `LGTM_OPENBAO_ADDR` (`plan.md:283` and `plan.md:284`): their readers are not yet in any plan. `go-s2-server-signin/plan.md:63` lists the credential file as open "with the Stack team". The list of readers is incomplete until that plan adopts the names, and I did not raise it as a finding because the contract table says the teams agree on it in sprint 2.
   - `bootstrap.state`: writers are 5a, the provisioner from item 4 and 5b. Readers are the preparer and the launcher. The three writers are serialized by the state lock. Its required material per checkpoint is missing (finding 6).
   - `installation.id`, `lgtm-installation` and `pg.reconciled`: writers and readers match the text. The cleanup of a losing claimant's provisional identity is missing (finding 3).
5. Truth tables built:
   - Local identity (absent, provisional, committed) against volume (absent, unclaimed empty, unclaimed with data, claimed same, claimed other): the empty cells are the path-only mismatch, the provisional identity of a loser, and committed identity with an unclaimed volume holding data (findings 3 and 9).
   - The 5b state table against the interruption points: the empty cells are an unknown time-out, a response file left after the `initialized` checkpoint, and an `intent` checkpoint with a response on an uninitialized vault (findings 5, 6 and 11).
   - Seal key (absent, malformed, valid) against evidence (none, present): the malformed row is empty (finding 5).
   - Reset against volume and state: the interrupted-reset cell is empty (finding 2).
   - Bootstrap cleanup against rotation staging: contradictory (finding 4).
6. No round 1 blocker or major is open in the text. The one blocker, `logic2-r1 1`, is fixed in the design text, but its guard test cannot fail (finding 1).
7. Lens questions touched by the patches:
   - Q1 (ordering): the checkpoint order is sound. The revoke, persist, delete order handles the crash after revocation, and the retry rule at `plan.md:269` covers it.
   - Q2 (failure policy): findings 2, 4 and 11.
   - Q3 (truth tables): the tables above.
   - Q4 (zero values): the empty `LGTM_STACK_STATE_DIR` is covered by the absolute-path rule. The unset reset arguments are covered by `--yes`. A zero-length key is finding 5.
   - Q5 (two actors): the state lock covers the launcher and the maintenance command. Finding 4 covers the staging files. Finding 2 covers the lock target.
   - Q6 (mutation strength): findings 1 and 7. The `seal` guard mutation at `plan.md:181` does fail the journal and credential controls, as claimed.
   - Q7 (cheapest wrong implementation): findings 5, 7, 9 and 10.
   - Q8 (shape): a separate journal location and one owner of staging files would make the finding 4 state impossible. A marker file for the reset would do the same for finding 2.

## Post-build checks

- PB-R2-1. In the real `openbao-init` job, replace `mv` with a copy-and-delete wrapper and kill the job midway through the copy. Require that the final path never holds a partial file, or that the run fails before it can.
- PB-R2-2. Run the reset and kill it between the volume removal and the state removal. Require that a rerun of the reset finishes, and that `deploy/up.sh` names the pending reset.
- PB-R2-3. Move the checkout directory with a committed installation. Require that `deploy/up.sh` either continues or names a documented recovery step.
- PB-R2-4. Rotate the server credential, make the recreation fail, then run `deploy/up.sh`. Require a nonzero exit and a message that names the pending rotation.
- PB-R2-5. Truncate the response file, the seal key and `bootstrap.state` in separate runs. Require a named diagnostic each time and no change to the volume.
- PB-R2-6. Run the first clean start and record the user and capability set of every one-shot job. Require that no job holds the file-permission override capability.

## Grade

6 out of 10. Counts: 0 blockers, 4 majors and 7 minors.

The patches close the forty-odd failures of round 1 that I could trace. The remaining defects sit in the three new mechanisms: the volume claim, the reset and the rotation. Each of them has a state that the plan never names. The cross-mount guard is the one that matters most, because the single blocker of round 1 can now regress without a red test.
