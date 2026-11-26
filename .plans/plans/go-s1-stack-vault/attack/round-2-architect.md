# Round 2 review of PostgreSQL and OpenBao, seat architect

- Task identifier: review-go-s1-stack-vault-r2-architect
- Nonce: 1f066f
- Lens: architect. Identifier prefix: architect-r2.
- Reviewed text: the patches of `plan.md` and `decisions.md` between commits `25d3132e` and `cd46223f`, and what follows from them.
- Method limit: the planner's Docker socket is denied, so no container ran. Image claims rest on the round 1 transcripts. An attempt to read the registry's tag metadata returned no usable answer from this host, so the two image digests in the plan are not independently confirmed here.

**Verdict: REVISE**

**Top finding.** The patch that makes every PostgreSQL login require a password (SCRAM, the Salted Challenge Response Authentication Mechanism) breaks the existing check driver. `psql_in_stack` in `deploy/test/checks.sh` runs `psql` inside the container, without a password, as a role and database that the plan never creates. (architect-r2 1)

## Findings

### architect-r2 1. The password-only PostgreSQL patch breaks the check driver's database access

- Severity: MAJOR
- Tied statement: D4, with C5 as the check it breaks.
- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:139` sets `--auth-local=scram-sha-256` and `POSTGRES_HOST_AUTH_METHOD=scram-sha-256`. Lines 141 and 283 create the logins `lgtm_app` and `lgtm_bao` and name the database `lgtm_app`. `deploy/test/checks.sh:324-326` defines `psql_in_stack` as `compose exec -T postgres psql -U "${POSTGRES_USER:-lgtm}" -d "${POSTGRES_DB:-lgtm}"`, with no password and no password file. `deploy/test/checks.sh:434` uses it in C5 to set a join token's expiry in the past. The words `psql_in_stack`, `POSTGRES_USER` and `POSTGRES_DB` appear nowhere in the plan.
- Failure scenario: Sprint 3 runs C5. The role `lgtm` and the database `lgtm` do not exist. Even if the environment names `lgtm_app`, a Unix-socket login now demands a password, so `psql` fails and C5 reports "PostgreSQL could not set the fixture token's expiry in the past". Before the patch, local trust made this call work, so the patch created the break. The failure surfaces two sprints later, in another team's work.
- Prescribed fix: add one row to the sprint 2 contract table, named "Check driver access to the database". Choose how `psql_in_stack` authenticates, for example `compose exec` with a `PGPASSFILE` that the stack mounts into the `postgres` container for the application login. Name the role and database the driver uses. State that Stack edits `deploy/test/checks.sh` for this, since the file lives in `deploy/`. Add one frozen assertion to `deploy/test/postgres.sh`: the exact `psql_in_stack` command, run against the fixture, returns a row from `lgtm_app`'s database.

### architect-r2 2. Items 2 and 3 cannot merge alone, because they depend on text in item 6

- Severity: MAJOR
- Tied statement: plan (lens question 6, "can each work item merge alone").
- Evidence:
  - `plan.md:136` says `prepare-db` "claims the installation using item 6's binding protocol". Item 2 needs only items 1 and 8 (`plan.md:425`).
  - `plan.md:166` has item 3's preparer create "the empty private mount directories described in item 6".
  - `plan.md:174-176` freezes `seal_key_is_created_once`, `missing_key_preserves_existing_vault` and `wrong_key_preserves_existing_vault` on "the privileged fixture helper" that stashes the key and reads fingerprints. Only item 6 specifies that helper (`plan.md:302`). Item 8's function list (`plan.md:91`) has `fixture_create`, `fixture_postgres`, `fixture_openbao`, `fixture_exec_as` and `fixture_cleanup`, and none of them stashes, restores or fingerprints.
  - The table in `plan.md:262-273` names a test, `stack_refuses_state_ahead_of_database`, that lives in item 6's file.
  - Item 6 depends on items 2, 3 and 5b and merges last (`plan.md:430`).
- Failure scenario: A builder takes item 2. The binding protocol, its file names and the root-owned directory layout exist only as prose in item 6, so the builder invents them. The builder of item 6 then reads a different protocol into the same `stack-prepare.sh`. Item 3's three seal tests cannot go red or green until a helper exists that no earlier item owns. The "each item merges alone with a passing full check" rule fails for two of nine items.
- Prescribed fix: move the binding protocol (file names, content fields, the atomic link, the provisional identity), the directory creation and ownership, and the stash, restore and fingerprint helper into item 8, which already owns fixtures and merges first. Or add a small item between 8 and 2. Item 6 then keeps only the launcher, the lock, the phases and the reset. Fix the table at `plan.md:262-273` so that each row names a test in the file of the item that owns that row.

### architect-r2 3. The C12 patch covers the database half only, and the mutation hook has no writer

- Severity: MAJOR
- Tied statement: C12.
- Evidence: `deploy/test/checks.sh:11` sets `mutation_hook=deploy/test/mutate-feed-lock.sh`. `check_c12` (`deploy/test/checks.sh:590-602`) fails unless the hook exists and is executable, and also needs `LGTM_TEST_POSTGRES` in its environment. `deploy/test/README.md:76` lists the hook with status "Assignment". `.plans/plans/go-s1-server-store/plan.md:48` says Stack owns the thin executable `deploy/test/mutate-feed-lock.sh` and that it invokes Server's `check-feed-lock.sh`. This plan does not mention the hook anywhere. Its work item 8 (`plan.md:91`) creates three files, and its traceability row for C12 (`plan.md:398`) says item 8 "gives the Server team's tests a disposable host database".
- Failure scenario: Both plans are built as written. The Server team ships `check-feed-lock.sh` and waits for Stack's wrapper. Stack ships none, because its plan never listed it. `deploy/test/checks.sh --only C12` fails with "The executable ... is missing", and the sprint 1 exit check for the C12 half cannot pass. The patch for architect-r1 2 (round 1, fixed another way) closed only the database half of that failure.
- Prescribed fix: add `deploy/test/mutate-feed-lock.sh` to item 8's file list. It is a thin executable that calls `src/server/internal/store/check-feed-lock.sh` with the inherited `LGTM_TEST_POSTGRES` and exits non-zero when that script is missing. Add one frozen assertion for it: absent script gives a failure, present script has its exit code passed through. Also fix two wordings in `plan.md:93`. The wrapper "writes the variable", but it only sets it for the child process. And the plan never says that C12 is run as `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12`, because `deploy/test/README.md:14` still tells the reader to export the variable by hand. State that invocation, or update the README.

### architect-r2 4. The recovery and reset patch leaves the most ordinary broken state without a defined action

- Severity: MAJOR
- Tied statement: D10, which is the stuck statement after round 1, and C13.
- Evidence: the reset rules are at `plan.md:324-326`. The reset "verifies the binding and the selected project" and "a mismatch refuses reset". The volume binding contains "a fingerprint of the canonical state path" (`plan.md:304`). Recovery row 3 (`plan.md:323`) offers only "restore the matching local database and state pair if available". The tests `stack_refuses_state_ahead_of_database` and `stack_reset_requires_confirmation` (`plan.md:334-335`) check the diagnostic and one incomplete-bootstrap fixture only.
- Truth table of the states that the plan has to answer:

| Volume | Local state | Binding check | What the plan says | Gap |
|---|---|---|---|---|
| Absent | Absent | Not applicable | Fresh installation. | None. |
| Absent (for example after `docker compose down -v`) | Present and claimed | Volume binding cannot be read | `state directory is ahead of the database`. Restore the pair if available. | Reset must "verify the binding", and the volume binding does not exist. Whether reset is allowed is not stated. With no backup, the owner has to delete root-owned directories by hand. |
| Present and claimed | Absent (for example a checkout cleanup removed `.state`) | Local identity unreadable | `restore the matching installation state`. | Reset needs the local identity to verify against and it is gone. Not stated. |
| Present and claimed | Present, but the state path changed (the checkout moved, or a second worktree) | Path fingerprint differs | `Compose project belongs to another state directory`. | Reset refuses a mismatch, so the one action that would clear the stale pairing is refused. The only way out is moving the directory back. |
| Present with PostgreSQL data, no claim | Any | Unclaimed | Rejected. | No operator action named. |

- Failure scenario: The owner runs `docker compose down -v` to clear containers, which is a routine command. The next `deploy/up.sh` refuses with "ahead of the database". The README offers a restore that cannot exist, and the reset path is undefined for this exact state. The owner is stuck on a fresh local installation, which round 1 tried to prevent (logic-r1 3, threat-r1 2).
- Prescribed fix: add to the recovery table, and to the reset description, the four missing rows above. For a missing volume, `--reset --yes` verifies against the local `installation.id` and removes the local state only. For missing local state, it verifies against the volume binding and removes the volume. For a mismatched path fingerprint, it accepts an explicit `--adopt-path` or equivalent argument, or the plan states that moving the checkout needs the old path. Add one frozen assertion per added row to `deploy/test/stack-vault.sh`, and count them in its acceptance line.

### architect-r2 5. The sprint 2 contract is still half open, and its readers have not adopted it

- Severity: MINOR
- Tied statement: plan, "What the teams must agree on" in the epic, point 3.
- Evidence: `plan.md:283` fixes three names, `LGTM_POSTGRES_PASSWORD_FILE`, `LGTM_OPENBAO_CREDENTIAL_FILE` and `LGTM_OPENBAO_ADDR`. It leaves the setting names for the database host, port, name and user to "sprint 2". The `go-s2-server-signin` brief (its "must agree on" section) and the `go-s2-stack-web` brief name no variable. No file other than this plan contains any of the three names. `plan.md:343` makes `deploy/vault-admin.sh apply-server-policy` the route for "adding approved server paths", yet Server cannot edit `deploy/`, and the contract table does not say that a new vault path is a Stack change followed by an operator command.
- Failure scenario: Server names its host setting `LGTM_DB_HOST` while Stack, writing the compose entry, names it `LGTM_POSTGRES_HOST`. The sprint 2 integration finds the mismatch late. Separately, a later Server plan that needs a second vault path has no stated procedure.
- Prescribed fix: name the four remaining settings in the table now (`LGTM_POSTGRES_HOST`, `LGTM_POSTGRES_PORT`, `LGTM_POSTGRES_DATABASE`, `LGTM_POSTGRES_USER`). Add a row "New vault path": Stack edits `deploy/openbao-server-policy.hcl`, the operator runs `apply-server-policy`. Ask the orchestrator to record all of it in the epic's agreement list, so that both sprint 2 planners see it.

### architect-r2 6. Credential rotation has no precondition on the bootstrap checkpoint

- Severity: MINOR
- Tied statement: D9 and D10.
- Evidence: `plan.md:262-273` defines the checkpoints `intent`, `initialized`, `provisioned`, `revoked` and `published`. `plan.md:341-345` lets `rotate-server-credential` run through the same state lock and publish with the same rename, but states no required checkpoint. The operator credential is written during provisioning, before root revocation.
- Failure scenario: The first start is interrupted at `revoked`. The operator, seeing "server vault credential needs repair" from an older run, runs rotation. It publishes a credential and retires an identifier while the bootstrap job still holds pending identities. The next `deploy/up.sh` resumes at `revoked` and publishes the pending file over the rotated one, or reports a conflicting state.
- Prescribed fix: state that rotation and policy application run only when the checkpoint is `published`, and otherwise print `vault is initialized but bootstrap state is incomplete`. Add that refusal to `admin_resumes_credential_rotation`'s file as a third assertion, or fold it into `deploy/test/vault-admin.sh`.

### architect-r2 7. One operator-role assertion claims a boundary that does not exist

- Severity: MINOR
- Tied statement: D9 and R9.
- Evidence: `plan.md:223` lets `lgtm-admin` update `sys/policies/acl/lgtm-server` and register custom secret identifiers for the server role. `plan.md:231` has `operator_role_is_separate` verify "the operator token cannot read a stored secret".
- Failure scenario: The operator token itself cannot read stored data. But the holder of `admin/operator.json` can register a secret identifier for the server role, log in as it, widen the server's policy to every path, and read all secrets. The assertion passes while the real boundary, root access on the host, is the only protection. A reader of the test trusts a property that does not hold.
- Prescribed fix: keep the test, but rename or reword it as "the operator token has no direct data access", and add one sentence to the plan and to `deploy/README.md`: holding the operator credential equals read access to every stored secret. Nothing else changes, since the owner decided on one user and one machine.

### architect-r2 8. Reduced capabilities meet the new file modes without an expected list

- Severity: MINOR
- Tied statement: D4 and R9.
- Evidence: `plan.md:143` sets `cap_drop: [ALL]` and "add back only file setup and user-switch capabilities needed by the pinned entries", to be verified on the first clean run. `plan.md:147` sets `postgres-admin.password` to owner 70, mode `0400`. The official PostgreSQL entrypoint reads `POSTGRES_PASSWORD_FILE` as root before it drops to user 70. A root process without the override capability for file permissions cannot read a file owned by 70 with mode `0400`. The `openbao-init` job must also change file ownership to 10001 (`plan.md:275`). No capability list is written for it.
- Failure scenario: The first clean run of item 2 fails at start with a permission error. The builder adds capabilities until it passes. The test `stack_services_have_no_egress_and_no_net_raw` (`plan.md:179`) checks only that the raw-network capability is absent, so any wider list also passes.
- Prescribed fix: write the expected capability list per service in a table (for example `postgres`, `openbao`, `openbao-init`, the preparer), and make the test assert equality with that list, not only the absence of one capability. If the admin password file needs the override capability, either change its owner to root or name the capability in the table with the reason.

### architect-r2 9. The criteria in "Done when" do not account for every frozen test

- Severity: MINOR
- Tied statement: plan.
- Evidence: criterion 3b (`plan.md:204`) names three of the five tests in item 5b and omits `bootstrap_resumes_staged_registration` and `bootstrap_refuses_invalid_published_credential`. Criterion 3d (`plan.md:206`) names one of the two item 7 tests and omits `admin_resumes_credential_rotation`. Criterion 3a names two of the six item 4 tests. Criterion 2c names three of the eight item 3 tests, and 3c names two of the six item 6 tests.
- Failure scenario: A reviewer who walks the criteria believes the work is done when the named tests pass, while the omitted tests, including the ones that guard the interruption points the patches added, have no criterion that depends on them.
- Prescribed fix: each criterion lists every test of its item, or the plan adds a line in each item saying that its acceptance line (the count in the last sentence of the item) covers the remainder.

## Answers to the attack questions

1. **Does each patch remove its finding and break nothing else?** Mostly yes for the blocker. The hidden-file rename inside the destination mount removes the cross-mount failure of logic2-r1 1, and the checkpoint table of item 5b (`plan.md:262-273`) is complete over its rows. The patches that break something else are: the SCRAM patch (logic-r1 4, threat-r1 4) breaks `psql_in_stack` (architect-r2 1); the C12 patch (architect-r1 2) leaves the hook (architect-r2 3); the reset patch (logic-r1 3, threat-r1 2) leaves undefined states (architect-r2 4); the item 6 text that item 2 and 3 now lean on (critic-r1 4, logic2-r1 6) breaks independent merging (architect-r2 2).
2. **`UNREVIEWED MECHANISM`, each attacked as a new design.**
   - `with-postgres.sh` (`plan.md:93`): placement is right (Stack writes it, Server reads it). The invocation form and the missing hook are findings 3. A loopback-only published port and a temporary filesystem are sound. The pinned image is the same as production.
   - SCRAM at initialization (`plan.md:139`): the official entrypoint exports the password for its own socket calls, so initialization itself works. The consequence is the driver break in finding 1, and the capability interaction in finding 8.
   - Operator AppRole (`plan.md:223`): the scope is as narrow as it can be for a role that edits a policy, but see finding 7 for what it implies.
   - Absent-identifier classification (`plan.md:224`): measuring the pinned response before using it as a condition is the right design. Nothing found beyond the dependence on the digest, which is not yet confirmed.
   - Retained pending state and one-login validation (`plan.md:275`): sound. A crash after the rename leaves `revoked` with a valid final file, and row 7 of the table covers it.
   - Atomic volume claim (`plan.md:304`): a hard link on one volume is atomic. The placement of the binding outside `PGDATA` is right, since the PostgreSQL image requires an empty data directory only in `PGDATA`. The open issue is finding 4 (the path fingerprint) and finding 2 (where the protocol is specified).
   - Explicit reset (`plan.md:326`): placement is right (Stack, behind a flag and a confirmation). See finding 4.
   - `apply-server-policy` (`plan.md:343`): sound as an explicit operator step. See finding 5 for the missing procedure, and finding 6 for the missing precondition.
3. **Rejected findings.** The register has none. Every outcome is fixed, fixed another way, or not folded (critic-r1 12, a style finding). Nothing found. The style finding's acronyms are in fact expanded at first use in the patched text that I read.
4. **Names that others build against, with writers and readers.**
   - `LGTM_TEST_POSTGRES`: writer is the Stack wrapper, readers are Server's store tests and `deploy/test/checks.sh`. The hook is missing from the list (finding 3).
   - The three sprint 2 setting names: writers and readers are listed in `plan.md:283-284`, but the readers' own plans do not use the names (finding 5).
   - `pg.reconciled`: writer `reconcile-db`, reader the launcher. Complete.
   - `admin/bootstrap.state`: writers items 5a and 5b, readers the preparer and the launcher. Item 7 also needs to read it (finding 6).
   - `installation.id` and `lgtm-installation`: the plan names the launcher and the preparer, but the item that owns the protocol is wrong (finding 2).
   - Phase lines `phase <name> start/done`: writer the launcher, reader the fixture. Complete.
5. **Contradictions and truth tables.** The recovery truth table is in finding 4. A second interaction: SCRAM-only local access, role names and the existing driver, in finding 1. A third: `check_c13` (`deploy/test/checks.sh:604-619`) recreates services with `compose up -d --force-recreate`, so it never runs `deploy/up.sh`. The plan states that Compose can recreate initialized daemons without setup (`plan.md:315`), the file binds exist, and `openbao-init` is under profile `setup`, so the C13 path still reopens the vault. Nothing found there.
6. **Round 1 blockers and majors reported fixed but still open.** The blocker logic2-r1 1 is closed. Partially open: architect-r1 2 (the hook, finding 3), logic-r1 3 with threat-r1 2 (reset states, finding 4) and architect-r1 9 (finding 5, minor in round 1). D10 stays the stuck statement until finding 4 is folded.
7. **Lens questions touched by the patches.**
   - Question 1 (anchors): `deploy/openbao.hcl`, `deploy/openbao-init.sh` and `deploy/compose.yaml` resolve in `floorplan/plan.json`. `up.sh`, `vault-admin.sh` and `openbao-bootstrap.sh` are not in the blueprint, which the brief allows as suggestions. `psql_in_stack` and `mutation_hook` resolve in the driver and are not in the plan (findings 1 and 3).
   - Question 2: findings 3, 5 and 6.
   - Question 3 (placement): nothing found. The hook, the compose entries and the scripts belong to Stack. The vault client stays with Server. The policy edit procedure crosses teams (finding 5).
   - Question 4 (R1–R11): R9 is touched by finding 7 and 8; R10 by findings 1 and 3. No other rule is affected.
   - Question 5 (other plans): findings 3 and 5.
   - Question 6 (merge alone): finding 2.
   - Question 7 (room for wave 2): the policy file and `apply-server-policy` give a route. See finding 5 for the missing statement.
   - Question 8 (dependencies): PostgreSQL `16.16-alpine` and OpenBao `2.7.3` are pinned by digest and rest on the round 1 transcripts. The helper image's `curl` and `jq` packages are pinned by item 4. I could not confirm the digests from this host, so the replay in item 1 stays the gate, as row 18 of `decisions.md` says.

## Post-build checks

- PB-A. Run the exact `psql_in_stack` command against the built stack, with the setting chosen for finding 1. Expect one row from the application database. This is the earliest point at which the sprint 3 C5 step can be proven.
- PB-B. Run `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12` once Server's store plan has merged. Expect the Go tests to pass and the hook to confirm the failure with the lock removed.
- PB-C. On a disposable installation, run `docker compose down -v`, then `deploy/up.sh`, then the reset described for finding 4. Expect a named diagnostic and a clean restart, with no manual deletion of root-owned directories.
- PB-D. Print the effective capability sets of each sprint 1 service from `docker inspect` and compare them with the table asked for in finding 8.
- PB-E. Confirm both image digests with the registry's tag metadata on a host that has network access, and record the result in `scouting.md`.

## Grade

6 out of 10. Blockers: 0. Majors: 4 (architect-r2 1, 2, 3, 4). Minors: 5 (architect-r2 5, 6, 7, 8, 9).
