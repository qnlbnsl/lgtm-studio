# Round 3 review of the PostgreSQL and OpenBao plan, seat architect

Task identifier: review-go-s1-stack-vault-r3-architect. Nonce: 81d73b. Finding prefix: architect-r3.

The review reads the working-tree `plan.md` of 536 lines. It differs from commit `040b6ce0` by four small uncommitted edits, at lines 165, 259, 282 and 368, and none of them changes a finding below.

## Verdict

REVISE

## Top finding

The plan keeps five mechanisms that no check of wave 1 and no design statement needs, namely the operator role with its credential and rotation journal, the installation binding with the confirmed reset, the five-step bootstrap checkpoint, the rename-only publication with its exact-version helper image, and the fixture proxy and move shim that test them, and its own text concedes that the operator credential removes no authority that the revoked root token held.

## Findings

### architect-r3 1

- Severity: MAJOR.
- Tied statement: D9 (the vault is held by the server, and the server's credential reaches only its own paths).
- Evidence: `plan.md:18`, `plan.md:24`, `plan.md:263`, `plan.md:264`, `plan.md:314`, `plan.md:420` to `plan.md:437`, and the design at `docs/specs/2026-11-25-go-rewrite-foundation-design.md:224`.
- What it is: The plan revokes the initial root token, then creates an operator role, `lgtm-admin`, and a durable `admin/operator.json` so that the operator can still change the server policy and issue server identifiers. Line 264 says that the file is "standing vault administrator material", that its holder can widen the server policy and obtain a server identity with the resulting policy, and that this gives "effective read or modification access anywhere the server can be authorized, including every stored secret".
- Failure scenario: A person who reads `admin/operator.json` holds the same authority that the revoked root token held, so revocation reduced nothing. The plan paid for the swap with the `provisioned`, `revoked` and `published` checkpoints, the pending operator file, the operator policy, the `lgtm-admin` role, the whole of item 7 (`deploy/vault-admin.sh`, `deploy/openbao-admin.sh`, the rotation journal `admin/rotation.state`, the rotation staging file, the policy diff and `--yes`), the test `admin_can_rotate_and_widen_policy_after_root_revocation`, the two other item 7 tests, `operator_token_has_no_direct_data_access`, and PB12 and PB14. The design asks only for a credential for the server that "reaches only the server's own paths", and no check of C1 to C13 touches the operator.
- What the plan loses when this is cut: Only the supported way to widen the server policy or replace the server credential after bootstrap without a reset. The signing-key path is the only path wave 1 needs. A later sprint that needs a second path can ask for one maintenance command at that time, when its real need is known.
- Fix: Cut the operator role, `admin/operator.json`, item 7, the rotation journal and the `revoked` checkpoint. Choose one of two honest ends. Either keep `admin/root.token` as a root-owned file of mode `0600` and document it as the vault administrator file, or keep the single recovery share and document `bao operator generate-root` for the day a policy changes. In both cases the server policy file is applied by hand with the administrator file, and `deploy/README.md` says so. State the decision in `decisions.md` with its date.

### architect-r3 2

- Severity: MAJOR.
- Tied statement: D10 (OpenBao is a service of the stack beside PostgreSQL).
- Evidence: `plan.md:143`, `plan.md:160` to `plan.md:166`, `plan.md:366` to `plan.md:416`, `plan.md:455`, and `plan.md:195` to `plan.md:205`.
- What it is: The installation binding (`installation.id`, the volume file `lgtm-installation`, the atomic hard link, the provisional and committed phases, the second-root refusal, the moved-root and symlink rules), the confirmed reset (`--reset --yes`, `reset.pending`, the manifest allow-list, the orphan-volume mode with `--orphan-volume-id`, the offline `reset-state` job) and the launcher lock together fill most of item 6 and part of item 2. Item 6 has eleven assertions, and seven of them (`stack_refuses_foreign_state_root`, `stack_refuses_state_ahead_of_database`, `stack_requires_explicit_orphan_volume_reset`, `stack_preserves_unclaimed_data_volume`, `stack_accepts_moved_state`, `stack_reset_requires_confirmation`, `stack_resumes_interrupted_reset`) and `stack_reset_refuses_foreign_entries` test this machinery alone.
- Failure scenario: The only loss that C13 and D10 care about is a stack that restarts with a different key than the one that encrypted the data. Item 3 already stops that case, because `stack-prepare seal` refuses to create a key when the vault database has rows (`plan.md:195`), and a wrong key stays sealed (`wrong_key_preserves_existing_vault`). The binding adds a second guard on top of this for a case that nobody asked about, which is two state folders for one Compose project. Meanwhile the reset can delete two databases and a state folder, and every round has found a new way for it to fail (round 1 logic-r1 3, round 2 logic-r2 2, logic-r2 3, threat-r2 2).
- What the plan loses when this is cut: The refusal to start from a second state folder, and the single-command wipe. Both are replaced by a two-line section in `deploy/README.md`: stop the stack, run `docker compose -p lgtm down -v`, delete the state folder.
- Fix: Cut `installation.id`, `lgtm-installation`, the orphan-volume mode, `reset.pending`, the manifest and the `reset-state` job. Keep the `flock` on the state folder, because two launchers racing is a real fault of C1. Keep the seal guard of item 3, `pg.reconciled` only if the launcher uses it, and a documented manual reset. Item 6 then keeps `stack_reopens_existing_vault`, `stack_refuses_missing_key` and `stack_bootstrap_is_serialized`. Remove PB11.

### architect-r3 3

- Severity: MAJOR.
- Tied statement: D10.
- Evidence: `plan.md:284`, `plan.md:305` to `plan.md:319`, `plan.md:323` to `plan.md:334`, `plan.md:336`, `plan.md:352`, `plan.md:356`.
- What it is: The bootstrap checkpoint has five steps (`intent`, `initialized`, `provisioned`, `revoked`, `published`), an eleven-row recovery table, eight protected files with their own lifetimes, pending and staged copies of both credentials, a rename-only publication, and a test harness that holds a recording proxy on the `vault` network and a `mv` shim that kills the container before or after the rename.
- Failure scenario: Two of the five steps carry real information. `PUT /v1/sys/init` returns the root token once, so the response must reach a file before anything else happens, and that is the `initialized` fact. The other fact is whether `server/openbao.json` exists. Every other step exists to resume between steps of root revocation (cut by finding 1) and of identifier staging. AppRole role, policy and mount creation are upserts, so a rerun of the provisioner creates the same objects. The rename-only publication guards against a copy across two mounts, but the plan's own command moves `/state/server/.openbao.bootstrap.pending.json` to `/state/server/openbao.json` (`plan.md:336`), which is one directory on one mount, so `rename(2)` cannot fail with a cross-device error there. The shim and the cross-mount mutation test a mistake that the production layout cannot make.
- What the plan loses when this is cut: Resumption at the exact crash boundaries. A crash at the unrecoverable point, after the vault is initialized and before the response file is durable, still ends in `vault is initialized but bootstrap state is incomplete` and the manual reset, as it does in the plan today (`plan.md:311`).
- Fix: Reduce the checkpoint to the two facts above, kept as two file names and not as a versioned state file. Make the provisioner idempotent by design. Cut the pending copies, the recording proxy, the `mv` shim and `bootstrap_resumes_staged_registration`. Keep the init-response write, one active-health wait with the 60-second deadline, one login check of the published file, and the test that kills the job after the response write. The rename-only fixture may stay as one assertion if the owner wants it, but the plan must then say which check needs it.

### architect-r3 4

- Severity: MAJOR.
- Tied statement: D10.
- Evidence: `plan.md:259`, `plan.md:275`, `plan.md:376`, `plan.md:301`.
- What it is: Patch threat-r2 7 and logic-r2 1 require the tools image to install `curl`, `jq` and GNU `coreutils` at exact Alpine package versions, and it stops the build when a pin is unavailable.
- Failure scenario: On 2026-11-26 in a throwaway container I observed that the Alpine package repository carries one current revision of each package, and that `coreutils` 9.11-r0 supports `mv --no-copy -T`, which fails across two mounts and succeeds inside one. The OpenBao image is Alpine 3.24.4, and its own `mv` is the busybox one. So the first Alpine revision bump after this plan merges makes every build on a new machine, or after `docker image prune`, stop with the message that the plan prescribes ("stop and name the missing pin"). A machine that already has the image keeps working, so the fault shows only on a clean clone, which is the first-start case of C1 and the case that sprint 3 installs on.
- What the plan loses when this is cut: The `--no-copy` option, which finding 3 shows is not needed, and `jq` and `curl` in a separate image. The `bao` command in the OpenBao image can create the mount, the role and the policy, and `bao ... -format=json -field=...` returns single values, so the setup jobs can run from the pinned OpenBao image and need no second image, no `deploy/openbao-tools/`, no `.dockerignore` pair and no `helper_packages_are_pinned`, `helper_context_excludes_state` or `images_are_pinned_by_digest` tests of the helper.
- Fix: Run the setup and maintenance jobs from the OpenBao image already pinned by digest, and use plain `mv -f` inside one directory. If a tools image stays, pin its base by digest, install packages without an exact version, and record in the plan that reproducibility ends at the base digest.

### architect-r3 5

- Severity: MAJOR.
- Tied statement: C12 (with R10).
- Evidence: `plan.md:95`, `plan.md:97`, `plan.md:454`, and the Store plan at `.plans/plans/go-s1-server-store/plan.md:47` and `.plans/plans/go-s1-server-store/plan.md:132`.
- What it is: The Store plan says that `storetest.NewDatabase` fails with a fixed diagnostic when the database name in `LGTM_TEST_POSTGRES` is not `lgtm_store_test` and when the role lacks `CREATEDB`, and it says that Stack supplies "the disposable `lgtm_store_test` maintenance database with `CREATEDB`". The Stack plan says only that the wrapper gives "the test login `CREATEDB` and a disposable password" and sets `LGTM_TEST_POSTGRES`. It never names the database, and the string `lgtm_store_test` appears nowhere in the Stack plan folder.
- Failure scenario: A Stack builder follows item 8 exactly and creates the wrapper's default database, `postgres`, or a name of their own. Every store test then stops at the name check, and C12, which runs `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12`, fails at `go test ./...` before it reaches the mutation hook. The writer of the name (Stack) and its reader (Server) disagree on a name that only one of them states.
- Fix: In item 8 state that the wrapper creates the database `lgtm_store_test` owned by the test login, that the connection string in `LGTM_TEST_POSTGRES` names that database, and that `test_database_is_disposable` asserts the name and the `CREATEDB` attribute.

### architect-r3 6

- Severity: MAJOR.
- Tied statement: plan.
- Evidence: `plan.md:368`, `plan.md:373` to `plan.md:377`, `plan.md:424`, `plan.md:428`, `plan.md:434`.
- What it is: Patch logic-r2 4 makes the launcher refuse to start while `admin/rotation.state` exists, and says the check happens "before bootstrap cleanup or server recreation". Item 7 requires "active OpenBao" for `rotate-server-credential`, and `vault-admin.sh` starts no service. The phase table does not say in which phase the rotation check runs.
- Truth table, stack state against launcher placement of the check:

| Stack state when rotation is pending | Check before the `database` phase | Check after the `vault` phase starts both daemons |
|---|---|---|
| Both daemons running | Launcher refuses. Rotation can run. | Launcher refuses. Rotation can run. |
| Stack down after a reboot or `docker compose down` | Launcher refuses and starts nothing. Rotation needs an active OpenBao and cannot run. The operator has no supported command left. | Launcher starts both daemons, then refuses. Rotation can run. |

- Failure scenario: A rotation crashes, or the host reboots, with the journal on disk. The operator runs `deploy/up.sh` and gets `credential rotation is pending; run deploy/vault-admin.sh rotate-server-credential`. The operator runs that command and it reports that OpenBao is not active. The only exit is the reset, which discards the vault. The test `admin_resumes_credential_rotation` runs with the fixture stack already up, so it cannot see this.
- Fix: If finding 1 is not accepted, add a row to the phase table. The rotation check runs inside the `vault` phase after `--wait` shows both daemons healthy and before `openbao-init`, the launcher exits nonzero and leaves both daemons running, and the test `admin_resumes_credential_rotation` starts from a stopped stack in one of its four cases. If finding 1 is accepted, this finding is moot.

### architect-r3 7

- Severity: MINOR.
- Tied statement: plan.
- Evidence: `plan.md:259`, `plan.md:274`, `plan.md:519`.
- What it is: Item 4 pins the helper's base "to item 3's digest" and its test `images_are_pinned_by_digest` needs the merged Compose file. The sizing table lists item 4 as depending on items 1 and 8 only and as able to run beside items 2 and 3.
- Failure scenario: A builder starts item 4 beside item 3, as the table allows, and finds the image digest and the Compose file missing, or merges item 4 with a failing or skipped test.
- Fix: Add item 3 to the dependencies of item 4 and remove the "can run alongside" entry for item 3. If finding 4 is accepted, the image pin moves to item 3 and this finding is moot.

### architect-r3 8

- Severity: MINOR.
- Tied statement: plan.
- Evidence: `plan.md:344`, `plan.md:345`, `plan.md:350`, and the Stack-web plan at `.plans/plans/go-s2-stack-web/decisions.md:34` to `.plans/plans/go-s2-stack-web/decisions.md:36`, row 18, row 24 and row 25.
- What it is: Three names that the neighbouring Stack-web plan reads do not agree with this plan. Row 25 of its decisions sets `LGTM_POSTGRES_URL` with the credentials split out, where this plan freezes five names (`LGTM_POSTGRES_HOST`, `_PORT`, `_DATABASE`, `_USER` and `_PASSWORD_FILE`) and the Server sign-in plan adopted them. Row 18 asks this plan for a launcher option `--until vault` (request V1), and this plan neither adds nor rejects it. Row 24 says that C5 must change in sprint 3 by moving the defaults of `POSTGRES_USER` and `POSTGRES_DB` to `lgtm_app`, while item 2 here already rewrites `psql_in_stack` with the account and the database written in as `lgtm_app` (`plan.md:345`) and does not touch the two environment defaults at `deploy/test/checks.sh:326`.
- Failure scenario: The web plan builds a Compose file that sets `LGTM_POSTGRES_URL`, the Go server reads the five names, finds `LGTM_POSTGRES_HOST` empty and stops at start. Separately, both plans edit `psql_in_stack` and the second merge conflicts with the first.
- Fix: In the sprint 2 contract table add one row that answers V1 (accept `--until vault` in item 6, or refuse it and say that the Stack-web plan edits the suites). State in the C5 row that item 2 owns the whole `psql_in_stack` change and that the default names in `deploy/test/README.md:63` are updated there. Ask the orchestrator to relay the five names to the Stack-web plan once more, because the register's relay has not reached its row 25.

## Q8 mapping of the mechanisms of rounds 1 and 2 to the checks and statements that need them

| Mechanism | Needed by | Verdict | What the plan loses when it is cut |
|---|---|---|---|
| Operator role `lgtm-admin`, `admin/operator.json`, operator policy | None. D9 asks only for the server's restricted credential. | Cut (finding 1). Major. | A supported route to widen the policy after bootstrap. A kept root or recovery file gives the same authority with less machinery. |
| Root revocation | D9 intent only, and it reduces nothing once the operator file exists (`plan.md:264`). | Cut with the operator (finding 1). | A root token file on disk, which the owner can accept for one machine. |
| Rotation journal `admin/rotation.state` and the rotation staging file | None. | Cut (finding 1). Major. | An automatic credential replacement. The manual route remains. |
| `apply-server-policy` and its diff | None in wave 1. | Cut (finding 1). | A reviewed policy change command. |
| Installation binding (`installation.id`, `lgtm-installation`) | None. C13 is served by the seal guard of item 3. | Cut (finding 2). Major. | Refusal of a second state folder for the same project. |
| Confirmed reset, `reset.pending`, manifest, orphan-volume mode | None. | Cut (finding 2). Major. | A single-command wipe. A README recipe replaces it. |
| Five-step bootstrap checkpoint and pending copies | Reduced form needed by D10 ("starts OpenBao the first time") and C13 (no second init on restart). | Reduce to two facts (finding 3). Major. | Resumption at boundaries after init. |
| Rename-only publication and `--no-copy -T` | None. The path stays inside one directory. | Cut (finding 3 and finding 4). | A guard for a cross-mount copy that cannot occur. |
| Tools image with exact apk pins | None. The pinned OpenBao image carries `bao`. | Cut (finding 4). Major. | `curl` and `jq` in a second image. |
| `deploy/test/mutate-feed-lock.sh` | C12 (`deploy/test/checks.sh:11`, `deploy/test/checks.sh:600`; epic `.plans/epics/go-rewrite/epic.md:133`). | Keep. | Not applicable. |
| `deploy/test/with-postgres.sh` | R10 and C12 (`LGTM_TEST_POSTGRES`). | Keep, with finding 5. | Not applicable. |
| `deploy/test/vault-testlib.sh`, `vault-fixtures.sh`, `fixture_key_action` | C13 (seal and restart tests need isolated projects and a key stash). | Keep. | Not applicable. |
| Recording proxy, `mv` shim, kill-at-rename harness | Only the cut mechanisms above. | Cut with finding 3. | Tests of mechanisms that are gone. |
| Container user and capability tables, internal networks | D10 ("reachable only inside the stack") and R9. | Keep. | Not applicable. |
| Authenticated `psql_in_stack` | C5 (`deploy/test/checks.sh:324`). | Keep. | Not applicable. |

## Answers to the attack questions of the lens file

1. Claims about existing code: nothing found beyond the names above. `deploy/test/checks.sh:22` (`compose`), `:324` to `:326` (`psql_in_stack`, defaults `lgtm`), `:594` to `:602` (`check_c12`), `:604` (`vault_ready`) and `:11` (`mutation_hook`) match what the plan says. `deploy/` holds only `test/`, as the plan assumes. The two image digests come from the proof in scouting. I confirmed in a throwaway container that the pinned PostgreSQL image starts with password-only authentication as user 70 under `cap_drop: ALL` and `no-new-privileges`.
2. Writers and readers: the name of the store test database is missing (finding 5). The five database names, the two OpenBao names and the signing-key log fields in the contract table have a named writer and a named reader. The neighbouring Stack-web plan holds a diverging name (finding 8).
3. Placement: nothing found. Everything is under `deploy/`, and the Server script `src/server/internal/store/check-feed-lock.sh` stays with the Server team, as the epic says.
4. Rules R1 to R11: R9 is kept (password files, no secrets in arguments or logs). R10 is kept, subject to finding 5. R1 to R8 and R11 govern the Go server and do not apply here.
5. Neighbours: finding 8. The ownership of the mutation hook agrees across this plan, the Store plan and the epic. The Sign-in plan adopted the five database names and the two OpenBao names.
6. Merge order: finding 7. Items 1, 8, 2 and 3 merge in a consistent order. The item 5a to 5b split is consistent. Item 7 depends on item 6 for the lock, which is stated.
7. Room for wave 2 and what no check needs: the Q8 table and findings 1 to 4. The plan keeps room for wave 2 through the exact-path policy and the sprint 2 contract table.
8. Dependencies and images: PostgreSQL 16.16-alpine and OpenBao 2.7.3 are pinned by digest, which is sound. The exact apk versions of the helper image are not sound (finding 4).

## Answers to the questions of the task file

1. Patches: the readiness test that substitutes each role password (logic-r2 7), the helper for `psql_in_stack` (architect-r2 1), the reconciler's user and group 70 (architect-r2 8, logic-r2 8), the named capability tables (threat-r2 8), the host-bridge note and PB9 (threat-r2 4), the sprint 2 names and the log fields (architect-r2 5, threat-r2 10), the `pg_hba_file_rules` check (threat-r2 3) and the 60-second monotonic deadline (logic-r2 11) remove their failures as written and break nothing else. The mutation wrapper (architect-r2 3) removes its failure but leaves the database name open (finding 5). The rotation-pending guard (logic-r2 4) leaves an ordering gap (finding 6). Every patch that adds state (architect-r2 2, architect-r2 4, logic-r2 2, logic-r2 3, logic-r2 5, logic-r2 6, threat-r2 1, threat-r2 2, threat-r2 6, threat-r2 9) fixes its own failure and adds the mechanisms that findings 1 to 3 cut. The exact-package pin (threat-r2 7) creates a new failure (finding 4).
2. Unreviewed mechanisms: the disposable database wrapper (`plan.md:95`) is sound, subject to finding 5. The password-only PostgreSQL start (`plan.md:146`) works on the pinned image, as observed. The binding (`plan.md:162`, `plan.md:165`), the operator file (`plan.md:263`), the absent-identifier lookup (`plan.md:265`), the recovery-share lifetime (`plan.md:321`), the rename-only publication (`plan.md:336`), the orphan reset (`plan.md:396`), the reset marker (`plan.md:400`) and the policy apply (`plan.md:425`) are attacked in findings 1 to 4.
3. Rejected outcomes: the register has no finding with the outcome "rejected". Five carry "fixed another way" (architect-r2 2, architect-r2 4, logic-r2 1, logic-r2 3, logic-r2 6). I checked their evidence and found it real, but over-built for the failure each describes (findings 2 and 3).
4. Writers and readers of changed names: complete for the names in the contract table. Incomplete for the store test database (finding 5) and for the Stack-web plan's readers (finding 8).
5. Contradictions: item 4 against the sizing table (finding 7), and the rotation guard against the maintenance command (finding 6, with its truth table). The `reset.pending` check comes before the binding check, so the reset marker and a removed local identity do not conflict. The manifest covers the rotation files and every `.tmp` sibling.
6. Open majors of round 2: no blocker is open. Two majors are not fully closed, namely architect-r2 3 (database name) and logic-r2 4 (ordering).
7. Lens questions touched by patches: answered above.
8. Mandate: answered in the table above. Five mechanisms have no check or statement that needs them, and each is a MAJOR finding (1, 2, 3 and 4, with the fixture machinery inside 3).

## Post-build checks

1. Build the helper image, or its replacement, on a machine with no cached image. Expected result: the build succeeds. If exact package pins stay, repeat the build after the next Alpine revision lands and require the same result.
2. Run `deploy/test/with-postgres.sh -- go test ./src/server/internal/store/...` once the Server store merges. Expected result: the name check of `storetest.NewDatabase` passes and the tests run.
3. Interrupt a rotation (if it survives), stop the stack, run `deploy/up.sh`, and then the maintenance command. Expected result: the operator reaches a completed rotation without a reset.
4. Place the state folder under a symbolic link, and start the stack twice in parallel. Expected result: one winner, one clear refusal, and the lock released when the winner exits.
5. Run `deploy/up.sh` on a clean clone, restart the host, and run C13. Expected result: the vault opens with no typed key and the signing-key fingerprint is unchanged. This is the one check that all of the machinery above exists to serve.

## Grade

5 out of 10. Blockers: 0. Majors: 6. Minors: 2.
